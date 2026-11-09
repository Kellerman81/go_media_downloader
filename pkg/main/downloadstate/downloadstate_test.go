package downloadstate

import (
	"context"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

func useTestDB(t *testing.T) {
	t.Helper()

	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)
}

type fakeClient struct {
	states  map[string]apiexternal_v2.DownloadState
	removed []string
}

func (f *fakeClient) DownloadStates(context.Context) (map[string]apiexternal_v2.DownloadState, error) {
	return f.states, nil
}

func (f *fakeClient) RemoveDownload(_ context.Context, id string, _ bool) error {
	f.removed = append(f.removed, id)
	return nil
}

func exec(t *testing.T, q string, args ...any) {
	t.Helper()

	if err := database.ExecNErr(q, args...); err != nil {
		t.Fatal(err)
	}
}

type histRow struct {
	State    string  `db:"download_state"`
	Progress float64 `db:"download_progress"`
	Message  string  `db:"download_message"`
	DlID     string  `db:"download_id"`
}

func history(t *testing.T, id uint) histRow {
	t.Helper()

	rows := database.StructscanT[histRow](false, 1,
		"select download_state, download_progress, download_message, download_id from movie_histories where id = ?", id)
	if len(rows) != 1 {
		t.Fatalf("history row %d not found", id)
	}

	return rows[0]
}

func lastscanSet(t *testing.T, movieID uint) bool {
	t.Helper()

	return database.Getdatarow[uint](false, "select count() from movies where id = ? and lastscan is not null", movieID) == 1
}

// seed creates movie 1 with a set lastscan and one history row per title.
func seed(t *testing.T, titles ...string) []uint {
	t.Helper()

	exec(t, "insert into dbmovies (id, title, imdb_id) values (1, 'Heat', 'tt0113277')")
	exec(t, "insert into movies (id, dbmovie_id, listname, missing, lastscan) values (1, 1, 'L', 1, datetime('now'))")

	ids := make([]uint, 0, len(titles))
	for _, title := range titles {
		id, err := database.ExecNid(
			"insert into movie_histories (title, url, downloaded_at, movie_id, dbmovie_id) values (?, 'u', datetime('now','localtime'), 1, 1)",
			title)
		if err != nil {
			t.Fatal(err)
		}

		ids = append(ids, uint(id)) //nolint:gosec // test ids
	}

	return ids
}

func useClient(t *testing.T, c *fakeClient) {
	t.Helper()

	providers.ReplaceDownloadProviders(map[string]any{"fake": c})
	t.Cleanup(func() { providers.ReplaceDownloadProviders(nil) })
}

func TestCheck(t *testing.T) {
	useTestDB(t)

	ids := seed(t, "Heat.1995.1080p", "Heat.1995.720p", "Heat.1995.2160p", "Heat.1995.Gone", "Heat.1995.NoID")
	Started(config.MediaTypeMovie, ids[0], "fake", "aaaa")
	Started(config.MediaTypeMovie, ids[1], "fake", "BBBB")
	Started(config.MediaTypeMovie, ids[2], "fake", "cccc")
	Started(config.MediaTypeMovie, ids[3], "fake", "dddd")
	Started(config.MediaTypeMovie, ids[4], "fake", "")
	// The vanished download has been missing for long enough.
	exec(t, "update movie_histories set state_changed_at = datetime('now','localtime','-1 hour') where id = ?", ids[3])

	c := &fakeClient{states: map[string]apiexternal_v2.DownloadState{
		"AAAA": {State: apiexternal_v2.DownloadDownloading, Progress: 42.4},
		"BBBB": {State: apiexternal_v2.DownloadCompleted, Progress: 100},
		"CCCC": {State: apiexternal_v2.DownloadFailed, Message: "par repair failed"},
		"EEEE": {Name: "Heat 1995 NoID", State: apiexternal_v2.DownloadQueued},
	}}
	useClient(t, c)

	if err := Check(context.Background()); err != nil {
		t.Fatal(err)
	}

	for i, want := range []histRow{
		{State: Downloading, Progress: 42.4, DlID: "aaaa"},
		{State: Completed, Progress: 100, DlID: "BBBB"},
		{State: Failed, Message: "par repair failed", DlID: "cccc"},
		{State: Removed, Message: "no longer in fake", DlID: "dddd"},
		{State: Added, DlID: "EEEE"},
	} {
		if got := history(t, ids[i]); got != want {
			t.Errorf("row %d: %+v, want %+v", i, got, want)
		}
	}

	if lastscanSet(t, 1) {
		t.Error("a failed download must reset the movie's lastscan")
	}

	if len(c.removed) != 0 {
		t.Errorf("removed %v without remove_failed", c.removed)
	}
}

func TestOrganized(t *testing.T) {
	useTestDB(t)

	ids := seed(t, "Heat.1995.1080p.BluRay-GRP", "Heat.1995.720p.WEB-OTHER")
	Started(config.MediaTypeMovie, ids[0], "drone", "")
	Started(config.MediaTypeMovie, ids[1], "drone", "")

	// The folder name picks the older row over the newest one.
	Organized(config.MediaTypeMovie, []uint{1}, "/dl/Heat.1995.1080p.BluRay-GRP", "/dl/Heat.1995.1080p.BluRay-GRP/heat.mkv", "", "")

	if got := history(t, ids[0]); got.State != Processed || got.Progress != 100 {
		t.Errorf("matching row: %+v", got)
	}

	if got := history(t, ids[1]); got.State != Added {
		t.Errorf("other row changed: %+v", got)
	}

	Organized(config.MediaTypeMovie, []uint{1}, "/dl/x", "", "lower_quality", "lower quality")

	if got := history(t, ids[1]); got.State != Rejected || got.Message != "lower_quality: lower quality" || !lastscanSet(t, 1) {
		t.Errorf("lower quality: %+v, lastscan kept %t", got, lastscanSet(t, 1))
	}

	exec(t, "update movie_histories set download_state = ? where id = ?", Completed, ids[1])
	Organized(config.MediaTypeMovie, []uint{1}, "/dl/x", "", "wrong_runtime", "wrong runtime")

	if got := history(t, ids[1]); got.State != Rejected || lastscanSet(t, 1) {
		t.Errorf("wrong runtime: %+v, lastscan kept %t", got, lastscanSet(t, 1))
	}
}

func TestCheckRowRemovesFailed(t *testing.T) {
	useTestDB(t)

	ids := seed(t, "Heat.1995.1080p")
	Started(config.MediaTypeMovie, ids[0], "fake", "abc")

	c := &fakeClient{}
	useClient(t, c)

	k := kindOf(config.MediaTypeMovie)
	r := &k.rows("id = ?", ids[0])[0]
	states := map[string]apiexternal_v2.DownloadState{"ABC": {State: apiexternal_v2.DownloadFailed, Message: "x"}}

	checkRow(context.Background(), k, r, states, "fake", &config.DownloaderConfig{RemoveFailed: true})

	if len(c.removed) != 1 || c.removed[0] != "abc" {
		t.Errorf("removed %v, want the stored id abc", c.removed)
	}
}

// TestOrganizedImportFolderSuffix covers the folder names the downloader
// really uses: the release name plus " (<external id>)". Without stripping
// it the newest row was marked instead of the organized one.
func TestOrganizedImportFolderSuffix(t *testing.T) {
	useTestDB(t)

	ids := seed(t, "a.season.of.promises.2026.720p.web.h264-norbit", "a.season.of.promises.2026.1080p.web.h264-norbit")
	Started(config.MediaTypeMovie, ids[0], "en", "")
	Started(config.MediaTypeMovie, ids[1], "en", "")

	folder := "/downloads/completed/Movies/a.season.of.promises.2026.720p.web.h264-norbit (tt46075510)"
	Organized(config.MediaTypeMovie, []uint{1}, folder, folder+"/a.season.of.promises.2026.720p.web.h264-norbit (tt46075510).mkv", "", "")

	if got := history(t, ids[0]); got.State != Processed {
		t.Errorf("organized row: %+v", got)
	}

	if got := history(t, ids[1]); got.State != Added {
		t.Errorf("newest row changed: %+v", got)
	}
}

func TestImportNames(t *testing.T) {
	tests := map[string][]string{
		"Heat.1995.1080p-GRP (tt0113277)": {"heat 1995 1080p grp (tt0113277)", "heat 1995 1080p grp"},
		"Heat.1995.1080p-GRP":             {"heat 1995 1080p grp"},
		"Show.S01E01 (76568)":             {"show s01e01 (76568)", "show s01e01"},
	}
	for in, want := range tests {
		if got := importNames(in); !slices.Equal(got, want) {
			t.Errorf("importNames(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestPendingFor(t *testing.T) {
	useTestDB(t)

	ids := seed(t, "added", "downloading.old", "added.old", "completed", "processed", "failed")
	exec(t, "update movie_histories set resolution_id = 5, quality_id = 7 where id = ?", ids[0])

	states := []string{Added, Downloading, Added, Completed, Processed, Failed}
	for i, id := range ids {
		exec(t, "update movie_histories set download_state = ? where id = ?", states[i], id)
	}

	// Two days old: a client-reported download still counts, a plain "added" no longer does.
	exec(t, "update movie_histories set downloaded_at = datetime('now','localtime','-2 days') where id in (?, ?)", ids[1], ids[2])

	got := PendingFor(config.MediaTypeMovie, 1)
	if len(got) != 3 {
		t.Fatalf("PendingFor = %+v, want the added, old downloading and completed rows", got)
	}

	if !slices.Contains(got, Pending{ResolutionID: 5, QualityID: 7}) {
		t.Errorf("quality ids not read: %+v", got)
	}

	if got := PendingFor(config.MediaTypeMovie, 2); len(got) != 0 {
		t.Errorf("other movie: %+v", got)
	}
}

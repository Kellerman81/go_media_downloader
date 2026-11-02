package database

import (
	"errors"
	"strconv"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

func TestAlbumFileAndRootpathHelpers(t *testing.T) {
	seedMusic(t)

	root := "/data/music/Pink Floyd/The Wall"
	if AlbumExistsByRootpath(&root) {
		t.Error("album without files reported as existing")
	}

	file := root + "/01 - In the Flesh.flac"
	track := TrackFileInfo{Filepath: file, Filename: "01 - In the Flesh.flac", Extension: ".flac", Format: "flac", TrackNumber: 1, DiscNumber: 1}

	if err := InsertAlbumFile(0, &track, "", 10, 0); err != nil {
		t.Fatal(err)
	}

	if AlbumFileExists(&file) {
		t.Error("InsertAlbumFile with album id 0 must not insert")
	}

	if err := InsertAlbumFile(100, &track, "acoustid-1", 10, 0); err != nil {
		t.Fatal(err)
	}

	if !AlbumFileExists(&file) || !AlbumExistsByRootpath(&root) {
		t.Error("inserted file/rootpath not found")
	}

	ab, err := GetAlbumByRootpath(root)
	if err != nil || ab.ID != 100 || ab.DbalbumID != 10 || ab.Listname != "Rock" || ab.QualityProfile != "lossless" {
		t.Errorf("GetAlbumByRootpath = %+v, %v", ab, err)
	}

	moved := "/data/music/Pink Floyd/The Wall (1979)"
	if err := UpdateAlbumRootpath(100, moved); err != nil {
		t.Fatal(err)
	}

	if _, err := GetAlbumByRootpath(root); !errors.Is(err, logger.ErrNotFoundAlbum) {
		t.Errorf("old rootpath still found: %v", err)
	}

	if _, err := GetAlbumByRootpath(""); !errors.Is(err, logger.ErrNotFoundAlbum) {
		t.Errorf("empty rootpath: %v", err)
	}

	dbid, list := uint(10), "ROCK"
	if got := GetAlbumListEntryID(&dbid, &list); got != 100 {
		t.Errorf("GetAlbumListEntryID = %d, want 100", got)
	}

	other := "Pop"
	if got := GetAlbumListEntryID(&dbid, &other); got != 0 {
		t.Errorf("GetAlbumListEntryID(other list) = %d", got)
	}

	empty := ""
	if AlbumFileExists(&empty) || AlbumExistsByRootpath(&empty) || AlbumFileExists(nil) {
		t.Error("empty path reported as existing")
	}
}

func TestFindAlbumsByArtistTitleBoundedAndOrdered(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbalbums (id, title, slug, series_name) values (1, 'Live', 'live', 'Tour')")
	for i := 2; i <= 40; i++ {
		mustExec(t, db, "insert into dbalbums (id, title, slug) values (?, ?, ?)",
			i, "Live "+strconv.Itoa(i), "live-"+strconv.Itoa(i))
	}

	title := "Live"

	res, err := FindAlbumsByArtistTitle(nil, &title, 5)
	if err != nil {
		t.Fatal(err)
	}

	if len(res) != 5 {
		t.Fatalf("got %d results, want 5 (maxResults)", len(res))
	}

	if res[0].ID != 1 {
		t.Fatalf("first result ID %d, want the exact title match 1", res[0].ID)
	}

	seen := map[uint]bool{}
	for _, r := range res {
		if seen[r.ID] {
			t.Fatalf("duplicate result ID %d", r.ID)
		}

		seen[r.ID] = true
	}

	// Shortest prefix matches come first after the exact match.
	if res[1].Title != "Live 2" {
		t.Errorf("second result %q, want %q", res[1].Title, "Live 2")
	}

	var ab Dbalbum

	id := uint(1)
	if err := ab.GetDbalbumByIDP(&id); err != nil || ab.SeriesName != "Tour" {
		t.Errorf("GetDbalbumByIDP series_name = %q (err %v), want %q", ab.SeriesName, err, "Tour")
	}
}

func TestFindAlbumEmptySlugDoesNotMatchUnpopulatedSlug(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbalbums (id, title, slug) values (1, 'Unrelated', '')")
	mustExec(t, db, "insert into dbartists (id, name, slug) values (1, 'Someone', '')")
	mustExec(t, db, "insert into dbalbum_artists (dbalbum_id, dbartist_id) values (1, 1)")

	title, artist := "...", "!!!"

	if res, err := FindAlbumByArtistTitle(&artist, &title); !errors.Is(err, logger.ErrNotFoundDbalbum) {
		t.Fatalf("FindAlbumByArtistTitle matched %+v via empty slug", res)
	}

	if res, err := FindAlbumsByArtistTitle(&artist, &title, 5); !errors.Is(err, logger.ErrNotFoundDbalbum) {
		t.Fatalf("FindAlbumsByArtistTitle matched %d albums via empty slug", len(res))
	}
}

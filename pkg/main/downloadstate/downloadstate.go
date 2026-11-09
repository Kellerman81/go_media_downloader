// Package downloadstate tracks what became of the releases sent to download
// clients: the client's progress, failures (which reset the item's last scan
// so another release is searched), and whether the structure job organized
// or rejected the result. The state lives in the *_histories rows.
package downloadstate

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// States of a history row. Rows from before tracking existed have "".
const (
	Added       = "added"       // sent to the client
	Downloading = "downloading" // the client is transferring it
	Completed   = "completed"   // the client has all data
	Failed      = "failed"      // the client gave up; the item is searched again
	Removed     = "removed"     // gone from the client before completing
	Processed   = "processed"   // the structure job organized it
	Rejected    = "rejected"    // the structure job refused it (runtime, language, ...)
)

const (
	// removedAfter is how long a download may be missing from its client
	// before it counts as removed (some clients list a new torrent late).
	removedAfter = 15 * time.Minute
	// trackDays bounds which rows are checked and matched at all.
	trackDays = 30
)

// pendingHours is how long a download that its client does not report as
// downloading still counts as pending. Clients without state reporting (drone)
// leave rows "added" forever, and a completed download the structure job never
// picks up stays "completed", so without a bound they would block searches
// for the item for the whole trackDays window.
const pendingHours = 24

// kind describes the history table of one media type.
type kind struct {
	isType     uint
	history    string // e.g. movie_histories
	mediaCol   string // its list-entry column, e.g. movie_id
	mediaTable string // the list table holding lastscan, e.g. movies
	hasQuality bool   // the history rows carry resolution/quality/codec/audio ids
}

var kinds = []kind{
	{config.MediaTypeMovie, "movie_histories", "movie_id", "movies", true},
	{config.MediaTypeSeries, "serie_episode_histories", "serie_episode_id", "serie_episodes", true},
	{config.MediaTypeBook, "book_histories", "book_id", "books", false},
	{config.MediaTypeAudiobook, "audiobook_histories", "audiobook_id", "audiobooks", false},
	{config.MediaTypeMusic, "album_histories", "album_id", "albums", false},
}

// Pending is a download of a list entry that was sent to a client and has
// not been organized or rejected yet. The ids are 0 for media types whose
// history has no quality columns.
type Pending struct {
	ResolutionID uint `db:"resolution_id"`
	QualityID    uint `db:"quality_id"`
	CodecID      uint `db:"codec_id"`
	AudioID      uint `db:"audio_id"`
}

// PendingFor returns the pending downloads of the list entry mediaID (movie,
// episode, book, audiobook or album id): rows the client reports as
// downloading, and added or completed rows from the last pendingHours.
func PendingFor(isType uint, mediaID uint) []Pending {
	k := kindOf(isType)
	if k == nil || mediaID == 0 {
		return nil
	}

	cols := "0 as resolution_id, 0 as quality_id, 0 as codec_id, 0 as audio_id"
	if k.hasQuality {
		cols = "resolution_id, quality_id, codec_id, audio_id"
	}

	return database.StructscanT[Pending](false, 0,
		"select "+cols+" from "+k.history+
			" where download_state in (?, ?, ?) and "+k.mediaCol+" = ?"+
			" and downloaded_at > datetime('now','localtime','-"+fmt.Sprint(trackDays)+" days')"+
			" and (download_state = ? or downloaded_at > datetime('now','localtime','-"+fmt.Sprint(pendingHours)+" hours'))",
		Added, Downloading, Completed, mediaID, Downloading)
}

func kindOf(isType uint) *kind {
	for i := range kinds {
		if kinds[i].isType == isType {
			return &kinds[i]
		}
	}

	return nil
}

// Started marks the history row historyID of media type isType as sent to
// the downloader config client, under the client's downloadID ("" when the
// client returned none).
func Started(isType uint, historyID uint, client, downloadID string) {
	k := kindOf(isType)
	if k == nil || historyID == 0 {
		return
	}

	err := database.ExecNErr(
		"update "+k.history+" set download_client = ?, download_id = ?, download_state = ?, download_progress = 0, "+
			"download_message = '', state_changed_at = datetime('now','localtime') where id = ?",
		&client, &downloadID, Added, &historyID)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Uint("history_id", historyID).Msg("Recording download state failed")
	}
}

// row is a tracked history row.
type row struct {
	ID       uint    `db:"id"`
	Client   string  `db:"download_client"`
	DlID     string  `db:"download_id"`
	State    string  `db:"download_state"`
	Progress float64 `db:"download_progress"`
	Message  string  `db:"download_message"`
	Title    string  `db:"title"`
	MediaID  uint    `db:"media_id"`
	// Age is the seconds since the state last changed.
	Age float64 `db:"age"`
}

func (k *kind) rows(where string, args ...any) []row {
	return database.StructscanT[row](false, 0,
		"select id, download_client, download_id, download_state, download_progress, download_message, title, "+
			k.mediaCol+" as media_id, (julianday('now','localtime') - julianday(coalesce(state_changed_at, downloaded_at))) * 86400 as age from "+
			k.history+" where downloaded_at > datetime('now','localtime','-"+fmt.Sprint(trackDays)+" days') and "+where+
			" order by id desc", args...)
}

func (k *kind) set(r *row, state string, progress float64, message string) {
	if state == r.State && message == r.Message && int(progress) == int(r.Progress) {
		return
	}

	query := "update " + k.history + " set download_state = ?, download_progress = ?, download_message = ?"
	if state != r.State {
		query += ", state_changed_at = datetime('now','localtime')"
	}

	if err := database.ExecNErr(query+" where id = ?", state, progress, message, r.ID); err != nil {
		logger.Logtype("error", 0).Err(err).Uint("history_id", r.ID).Msg("Updating download state failed")
		return
	}

	if state != r.State {
		logger.Logtype("info", 1).
			Str("title", r.Title).
			Str("client", r.Client).
			Str("from", r.State).
			Str("to", state).
			Str("message", message).
			Msg("Download state changed")
	}

	r.State, r.Progress, r.Message = state, progress, message
}

// resetLastscan makes the item due for the next missing/upgrade search.
func (k *kind) resetLastscan(mediaID uint) {
	if mediaID == 0 {
		return
	}

	database.ExecN("update "+k.mediaTable+" set lastscan = null where id = ?", &mediaID)
}

// normName compares release names regardless of separators and case.
func normName(s string) string {
	return strings.ToLower(strings.NewReplacer(".", " ", "_", " ", "-", " ").Replace(strings.TrimSpace(s)))
}

// Check asks every client with open downloads for their state and updates
// the history rows: progress, completion, failures and removals. A failure
// resets the item's last scan and, with the downloader's remove_failed,
// removes the download from the client.
func Check(ctx context.Context) error {
	type tracked struct {
		k *kind
		r *row
	}

	byClient := make(map[string][]tracked)

	for i := range kinds {
		k := &kinds[i]

		rows := k.rows("download_state in (?, ?) and download_client != ''", Added, Downloading)
		for j := range rows {
			byClient[rows[j].Client] = append(byClient[rows[j].Client], tracked{k, &rows[j]})
		}
	}

	var errs []error

	for name, list := range byClient {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}

		lister, ok := providers.GetDownloadClient(name).(apiexternal_v2.DownloadStateLister)
		if !ok {
			continue // drone, an unregistered client, or one without states
		}

		states, err := lister.DownloadStates(ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("download client %s: %w", name, err))
			continue
		}

		dlCfg := config.GetSettingsDownloader(name)

		for _, t := range list {
			checkRow(ctx, t.k, t.r, states, name, dlCfg)
		}
	}

	return errors.Join(errs...)
}

func checkRow(
	ctx context.Context,
	k *kind,
	r *row,
	states map[string]apiexternal_v2.DownloadState,
	client string,
	dlCfg *config.DownloaderConfig,
) {
	if r.DlID == "" {
		r.DlID = matchByName(states, r.Title)
		if r.DlID == "" {
			return
		}

		database.ExecN("update "+k.history+" set download_id = ? where id = ?", &r.DlID, &r.ID)
	}

	st, ok := states[apiexternal_v2.NormalizeDownloadID(r.DlID)]
	if !ok {
		if r.Age > removedAfter.Seconds() {
			k.set(r, Removed, r.Progress, "no longer in "+client)
		}

		return
	}

	switch st.State {
	case apiexternal_v2.DownloadFailed:
		k.set(r, Failed, st.Progress, st.Message)
		k.resetLastscan(r.MediaID)

		if dlCfg != nil && dlCfg.RemoveFailed {
			remover, ok := providers.GetDownloadClient(client).(apiexternal_v2.DownloadRemover)
			if ok {
				if err := remover.RemoveDownload(ctx, r.DlID, true); err != nil {
					logger.Logtype("warn", 0).Err(err).Str("title", r.Title).Msg("Removing failed download failed")
				}
			}
		}

	case apiexternal_v2.DownloadCompleted:
		k.set(r, Completed, 100, st.Message)

	case apiexternal_v2.DownloadDownloading:
		k.set(r, Downloading, st.Progress, st.Message)

	default:
		// Queued or paused: a download that was transferring stays downloading.
		state := r.State
		if state == "" {
			state = Added
		}

		k.set(r, state, st.Progress, st.Message)
	}
}

// matchByName finds the id of the one download named like the release, for
// rows whose add call returned no id.
func matchByName(states map[string]apiexternal_v2.DownloadState, title string) string {
	want := normName(title)
	if want == "" {
		return ""
	}

	var found string

	for id, st := range states {
		if normName(st.Name) == want {
			if found != "" {
				return "" // ambiguous
			}

			found = id
		}
	}

	return found
}

// importNames returns the normalized forms a release name can have in the
// import folder: as is, and without the " (<external id>)" the downloader
// appends to the folder (and so to the file) name, e.g. " (tt46075510)".
func importNames(name string) []string {
	n := normName(name)
	if i := strings.LastIndex(n, " ("); i > 0 && strings.HasSuffix(n, ")") {
		return []string{n, strings.TrimSpace(n[:i])}
	}

	return []string{n}
}

// Organized records what the structure job did with the downloaded files of
// the list entries mediaIDs (movie, episode, book, audiobook or album ids):
// reason "" means organized, anything else is the move_unprocessed reason it
// was rejected for, with detail as the error text. Of each entry's open rows
// the one named like the import folder or file wins, else the newest. A
// rejection resets the entry's last scan unless the library already had
// equal or better quality.
func Organized(isType uint, mediaIDs []uint, folder, file, reason, detail string) {
	k := kindOf(isType)
	if k == nil {
		return
	}

	names := importNames(filepath.Base(folder))
	if file != "" {
		names = append(names, importNames(strings.TrimSuffix(filepath.Base(file), filepath.Ext(file)))...)
	}

	for _, id := range mediaIDs {
		if id == 0 {
			continue
		}

		rows := k.rows(k.mediaCol+" = ? and download_state in (?, ?, ?)", id, Added, Downloading, Completed)
		if len(rows) == 0 {
			continue
		}

		r := &rows[0]

		for i := range rows {
			if n := normName(rows[i].Title); n != "" && slices.Contains(names, n) {
				r = &rows[i]
				break
			}
		}

		if reason == "" {
			k.set(r, Processed, 100, "")
			continue
		}

		message := reason
		if detail != "" {
			message += ": " + detail
		}

		k.set(r, Rejected, r.Progress, message)

		if reason != "lower_quality" {
			k.resetLastscan(id)
		}
	}
}

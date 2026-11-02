package structure

import (
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// RootpathChange is one album/audiobook/book whose stored rootpath differs
// from the folder its files are in.
type RootpathChange struct {
	Table string
	ID    uint
	Old   string
	New   string
}

// RootpathRepairResult summarises RepairAudioRootpaths per table.
type RootpathRepairResult struct {
	Changes   []RootpathChange
	Ambiguous map[string]int // entries whose files are spread over several album folders
}

// audioFileFolder is the album folder a track file belongs to: its directory,
// or the one above a disc subfolder.
func audioFileFolder(location string) string {
	dir := filepath.Dir(location)
	if parent := filepath.Dir(dir); importfeed.IsDiscSubfolder(filepath.Base(dir), filepath.Base(parent)) {
		return parent
	}

	return dir
}

// bookFileFolder returns the folder holding a book file, or "" when that is
// one of the book library roots (bookRootpath stores nothing there either).
func bookFileFolder(libroots []string) func(string) string {
	return func(location string) string {
		dir := filepath.Dir(location)
		for _, root := range libroots {
			if strings.EqualFold(dir, root) {
				return ""
			}
		}

		return dir
	}
}

// bookLibraryRoots returns the configured paths of all book media configs.
func bookLibraryRoots() []string {
	var roots []string

	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		if cfgp.IsType != config.MediaTypeBook {
			return nil
		}

		for _, data := range cfgp.DataMap {
			if data.CfgPath != nil {
				if p := strings.TrimRight(data.CfgPath.Path, "/\\"); p != "" {
					roots = append(roots, filepath.Clean(p))
				}
			}
		}

		return nil
	})

	return roots
}

// planRootpathRepairs returns the changes for one table's rows (rootpath,
// location, id), which must be ordered by id. folderOf maps a file location
// to the rootpath it belongs to ("" for none). Entries whose files sit in
// more than one folder are left alone and counted as ambiguous.
func planRootpathRepairs(
	table string,
	rows []database.DbstaticTwoStringOneRInt,
	folderOf func(string) string,
) ([]RootpathChange, int) {
	var (
		changes   []RootpathChange
		ambiguous int
	)

	for start := 0; start < len(rows); {
		end := start + 1
		for end < len(rows) && rows[end].Num == rows[start].Num {
			end++
		}

		folder := folderOf(rows[start].Str2)
		single := true

		for i := start + 1; i < end; i++ {
			if folderOf(rows[i].Str2) != folder {
				single = false
				break
			}
		}

		switch {
		case !single:
			ambiguous++
		case folder != "" && folder != rows[start].Str1 && rows[start].Num > 0:
			changes = append(changes, RootpathChange{
				Table: table,
				ID:    uint(rows[start].Num), //nolint:gosec // ids are positive
				Old:   rows[start].Str1,
				New:   folder,
			})
		}

		start = end
	}

	return changes, ambiguous
}

// RepairAudioRootpaths recomputes albums.rootpath / audiobooks.rootpath /
// books.rootpath from the locations of their files (the album or book folder,
// as the organizer and library scan store it). Older scans stored the first
// folder below the library root (the artist or author folder) for some
// entries, which makes the next upgrade move the album/book into that folder
// and makes later scans skip the artist's other albums. With apply false
// nothing is written.
func RepairAudioRootpaths(apply bool) RootpathRepairResult {
	res := RootpathRepairResult{Ambiguous: make(map[string]int, 3)}

	for _, t := range []struct {
		table, query string
		folderOf     func(string) string
	}{
		{"albums", "SELECT a.rootpath, f.location, a.id FROM albums a JOIN album_files f ON f.album_id = a.id ORDER BY a.id", audioFileFolder},
		{"audiobooks", "SELECT a.rootpath, f.location, a.id FROM audiobooks a JOIN audiobook_files f ON f.audiobook_id = a.id ORDER BY a.id", audioFileFolder},
		{"books", "SELECT b.rootpath, f.location, b.id FROM books b JOIN book_files f ON f.book_id = b.id ORDER BY b.id", bookFileFolder(bookLibraryRoots())},
	} {
		rows := database.GetrowsN[database.DbstaticTwoStringOneRInt](false, 0, t.query)

		changes, ambiguous := planRootpathRepairs(t.table, rows, t.folderOf)
		res.Changes = append(res.Changes, changes...)
		res.Ambiguous[t.table] = ambiguous

		if !apply {
			continue
		}

		for i := range changes {
			var err error

			switch t.table {
			case "albums":
				err = database.UpdateAlbumRootpath(changes[i].ID, changes[i].New)
			case "audiobooks":
				err = database.UpdateAudiobookRootpath(changes[i].ID, changes[i].New)
			default:
				database.ExecN("UPDATE books SET rootpath = ? WHERE id = ?", &changes[i].New, &changes[i].ID)
			}

			if err != nil {
				logger.Logtype("error", 0).
					Str("table", t.table).
					Uint("id", changes[i].ID).
					Err(err).
					Msg("Failed to repair rootpath")

				continue
			}

			logger.Logtype("info", 0).
				Str("table", t.table).
				Uint("id", changes[i].ID).
				Str("old", changes[i].Old).
				Str("new", changes[i].New).
				Msg("Repaired rootpath")
		}
	}

	return res
}

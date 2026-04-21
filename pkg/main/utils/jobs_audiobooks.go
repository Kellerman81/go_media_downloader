package utils

import (
	"context"
	"errors"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/audiobooks"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
)

func init() {
	audiobooks.RegisterRefresh(refreshAudiobooksWrapper)
}

func refreshAudiobooksWrapper(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if arr, ok := data.([]string); ok {
		return refreshaudiobooks(ctx, cfgp, arr)
	}

	return nil
}

// refreshaudiobooks refreshes audiobook metadata by re-importing from Audnex for each ASIN.
func refreshaudiobooks(ctx context.Context, cfgp *config.MediaTypeConfig, arr []string) error {
	if len(arr) == 0 {
		return nil
	}

	var err error
	for idx := range arr {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}

		logger.Logtype("info", 1).
			Str("asin", arr[idx]).
			Msg("Refresh Audiobook")

		listid := getrefreshaudiobooklistid(&arr[idx], cfgp)
		if listid == -1 {
			continue
		}

		_, errsub := importfeed.JobImportAudiobooks(
			ctx, arr[idx],
			cfgp,
			listid,
			false,
		)
		if errsub != nil {
			err = errsub
		}
	}

	return err
}

// getrefreshaudiobooklistid looks up the list ID for the given ASIN and media config.
func getrefreshaudiobooklistid(asin *string, cfgp *config.MediaTypeConfig) int {
	listname := database.Getdatarow[string](
		false,
		"SELECT a.listname FROM audiobooks a JOIN dbaudiobooks db ON a.dbaudiobook_id = db.id WHERE db.asin = ?",
		asin,
	)
	if listname == "" {
		return -1
	}

	k, ok := cfgp.ListsMapIdx[listname]
	if !ok {
		return -1
	}

	return k
}

// RefreshAudiobook refreshes the data for a single audiobook by its database ID.
func RefreshAudiobook(cfgp *config.MediaTypeConfig, id *string) error {
	return refreshaudiobooks(
		context.Background(),
		cfgp,
		database.GetrowsN[string](
			false,
			1,
			"select distinct dbaudiobooks.asin from dbaudiobooks inner join audiobooks on audiobooks.dbaudiobook_id = dbaudiobooks.id where dbaudiobooks.id = ?",
			id,
		),
	)
}

// RetagAudiobook re-writes audio tags for a single audiobook by its dbaudiobook ID.
// It uses existing DB metadata (from the original Audnex import) rather than re-matching.
func RetagAudiobook(cfgp *config.MediaTypeConfig, dbID uint) error {
	// Get audiobook metadata from DB
	var dbAudiobook database.Dbaudiobook
	if err := dbAudiobook.GetDbaudiobookByIDP(&dbID); err != nil {
		return err
	}

	// Get file locations from audiobook_files
	type fileRow struct {
		Location    string `db:"location"`
		TrackNumber uint16 `db:"track_number"`
		DiscNumber  uint16 `db:"disc_number"`
	}

	files := database.StructscanT[fileRow](
		false,
		database.Getdatarow[uint](false, "SELECT count() FROM audiobook_files WHERE dbaudiobook_id = ?", &dbID),
		"SELECT location, track_number, disc_number FROM audiobook_files WHERE dbaudiobook_id = ? ORDER BY disc_number, track_number",
		&dbID,
	)
	if len(files) == 0 {
		return errors.New("no files found for audiobook")
	}

	// Get narrator from DB
	var narrator string
	database.Scanrowsdyn(false,
		"SELECT n.name FROM dbnarrators n "+
			"JOIN dbaudiobook_narrators an ON n.id = an.dbnarrator_id "+
			"WHERE an.dbaudiobook_id = ? LIMIT 1",
		&narrator, &dbID)

	// Get chapter titles from DB (keyed by chapter number)
	type chapterRow struct {
		Title         string `db:"title"`
		ChapterNumber uint16 `db:"chapter_number"`
	}

	chapters := database.StructscanT[chapterRow](
		false,
		database.Getdatarow[uint](false, "SELECT count() FROM dbaudiobook_chapters WHERE dbaudiobook_id = ?", &dbID),
		"SELECT title, chapter_number FROM dbaudiobook_chapters WHERE dbaudiobook_id = ? ORDER BY chapter_number",
		&dbID,
	)

	chapterMap := make(map[int]string, len(chapters))
	for i := range chapters {
		chapterMap[int(chapters[i].ChapterNumber)] = chapters[i].Title
	}

	// Build AlbumInfo from DB metadata
	album := &parser_v2.AlbumInfo{
		Title:      dbAudiobook.Title,
		Year:       int(dbAudiobook.Year),
		DatabaseID: dbID,
		TrackCount: len(files),
		ASIN:       dbAudiobook.ASIN,
		Narrator:   narrator,
	}

	tracks := make([]parser_v2.TrackInfo, 0, len(files))
	for i := range files {
		track := parser_v2.TrackInfo{
			Filepath:    files[i].Location,
			TrackNumber: int(files[i].TrackNumber),
			DiscNumber:  int(files[i].DiscNumber),
			ASIN:        dbAudiobook.ASIN,
			Narrator:    narrator,
		}
		// Use chapter title from DB if available, otherwise use track title from file
		if title, ok := chapterMap[int(files[i].TrackNumber)]; ok && title != "" {
			track.Title = title
		} else if ti, err := parser_v2.ReadAudioTags(files[i].Location); err == nil && ti != nil {
			track.Title = ti.Title
			parser_v2.PutTrackInfo(ti)
		}

		tracks = append(tracks, track)
	}

	album.Tracks = tracks

	// Determine embedArt and embedLyrics from config
	embedArt := false
	embedLyrics := false
	for idx := range cfgp.Data {
		if cfgp.Data[idx].EmbedArt {
			embedArt = true
		}
		if cfgp.Data[idx].EmbedLyrics {
			embedLyrics = true
		}
	}

	return structure.TagAlbumFiles(config.MediaTypeAudiobook, embedArt, embedLyrics, album)
}

// RetagAuthorAudiobooks re-writes audio tags for all audiobooks by a given dbauthor ID.
func RetagAuthorAudiobooks(cfgp *config.MediaTypeConfig, authorID uint) error {
	ids := database.Getrowssize[uint](false,
		"SELECT count(DISTINCT ab.dbaudiobook_id) FROM audiobooks ab JOIN dbaudiobook_authors aba ON ab.dbaudiobook_id = aba.dbaudiobook_id WHERE aba.dbauthor_id = ?",
		"SELECT DISTINCT ab.dbaudiobook_id FROM audiobooks ab JOIN dbaudiobook_authors aba ON ab.dbaudiobook_id = aba.dbaudiobook_id WHERE aba.dbauthor_id = ?",
		&authorID)

	var lastErr error
	for _, id := range ids {
		if err := RetagAudiobook(cfgp, id); err != nil {
			lastErr = err
			logger.Logtype("error", 1).
				Uint("dbaudiobook_id", id).
				Err(err).
				Msg("Failed to retag audiobook")
		}
	}

	return lastErr
}

// RetagAllAudiobooks re-writes audio tags for all audiobooks that have files.
func RetagAllAudiobooks(cfgp *config.MediaTypeConfig) error {
	ids := database.Getrowssize[uint](false,
		"SELECT count(DISTINCT dbaudiobook_id) FROM audiobook_files",
		"SELECT DISTINCT dbaudiobook_id FROM audiobook_files")

	var lastErr error
	for _, id := range ids {
		if err := RetagAudiobook(cfgp, id); err != nil {
			lastErr = err
			logger.Logtype("error", 1).
				Uint("dbaudiobook_id", id).
				Err(err).
				Msg("Failed to retag audiobook")
		}
	}

	return lastErr
}

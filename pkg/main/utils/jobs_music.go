package utils

import (
	"context"
	"errors"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/music"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
)

func init() {
	music.RegisterRefresh(refreshMusicWrapper)
}

func refreshMusicWrapper(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if arr, ok := data.([]string); ok {
		return refreshmusic(ctx, cfgp, arr)
	}

	return nil
}

// refreshmusic refreshes album metadata by re-importing from MusicBrainz for each release ID.
func refreshmusic(ctx context.Context, cfgp *config.MediaTypeConfig, arr []string) error {
	if len(arr) == 0 {
		return nil
	}

	var err error
	for idx := range arr {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}

		logger.Logtype("info", 1).
			Str("musicbrainz_id", arr[idx]).
			Msg("Refresh Album")

		listid := getrefreshalbumlistid(&arr[idx], cfgp)
		if listid == -1 {
			continue
		}

		_, errsub := importfeed.JobImportAlbums(
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

// getrefreshalbumlistid looks up the list ID for the given MusicBrainz release ID and media config.
func getrefreshalbumlistid(mbid *string, cfgp *config.MediaTypeConfig) int {
	listname := database.Getdatarow[string](
		false,
		"SELECT a.listname FROM albums a JOIN dbalbums db ON a.dbalbum_id = db.id WHERE db.musicbrainz_release_id = ? OR db.upc = ?",
		mbid,
		mbid,
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

// RefreshAlbum refreshes the data for a single album by its database ID.
func RefreshAlbum(cfgp *config.MediaTypeConfig, id *string) error {
	return refreshmusic(
		context.Background(),
		cfgp,
		database.GetrowsN[string](
			false,
			1,
			"select distinct dbalbums.musicbrainz_release_id from dbalbums inner join albums on albums.dbalbum_id = dbalbums.id where dbalbums.id = ?",
			id,
		),
	)
}

// RetagAlbum re-writes audio tags for a single album by its dbalbum ID.
// It uses existing DB metadata (from the original MusicBrainz import) rather than re-matching.
func RetagAlbum(ctx context.Context, cfgp *config.MediaTypeConfig, dbID uint) error {
	// Get album metadata from DB
	var dbAlbum database.Dbalbum
	if err := dbAlbum.GetDbalbumByIDP(&dbID); err != nil {
		return err
	}

	// Get file locations from album_files (joined with dbtracks for track metadata)
	type fileRow struct {
		Location    string `db:"location"`
		TrackNumber uint16 `db:"track_number"`
		DiscNumber  uint16 `db:"disc_number"`
		AcoustID    string `db:"acoustid"`
		DbtrackID   uint   `db:"dbtrack_id"`
	}

	files := database.StructscanT[fileRow](
		false,
		database.Getdatarow[uint](
			false,
			"SELECT count() FROM album_files WHERE dbalbum_id = ?",
			&dbID,
		),
		"SELECT location, track_number, disc_number, acoustid, dbtrack_id FROM album_files WHERE dbalbum_id = ? ORDER BY disc_number, track_number",
		&dbID,
	)
	if len(files) == 0 {
		return errors.New("no files found for album")
	}

	// Get track metadata from dbtracks (keyed by ID)
	type trackMeta struct {
		ID                     uint   `db:"id"`
		Title                  string `db:"title"`
		MusicbrainzRecordingID string `db:"musicbrainz_recording_id"`
		ISRC                   string `db:"isrc"`
		AcoustID               string `db:"acoustid"`
		TrackNumber            uint16 `db:"track_number"`
		DiscNumber             uint16 `db:"disc_number"`
	}

	dbTracks := database.StructscanT[trackMeta](
		false,
		database.Getdatarow[uint](
			false,
			"SELECT count() FROM dbtracks WHERE dbalbum_id = ?",
			&dbID,
		),
		"SELECT id, title, musicbrainz_recording_id, isrc, acoustid, track_number, disc_number FROM dbtracks WHERE dbalbum_id = ?",
		&dbID,
	)

	trackMap := make(map[uint]*trackMeta, len(dbTracks))
	for idx := range dbTracks {
		trackMap[dbTracks[idx].ID] = &dbTracks[idx]
	}

	// Build AlbumInfo from DB metadata
	album := &parser_v2.AlbumInfo{
		Title:       dbAlbum.Title,
		Year:        int(dbAlbum.Year),
		DatabaseID:  dbID,
		TrackCount:  dbAlbum.TotalTracks,
		Label:       dbAlbum.Label,
		Genre:       dbAlbum.Genres,
		ReleaseType: dbAlbum.ReleaseType,
		Country:     dbAlbum.Country,
	}
	if album.TrackCount == 0 {
		album.TrackCount = len(files)
	}

	tracks := make([]parser_v2.TrackInfo, 0, len(files))
	for i := range files {
		track := parser_v2.TrackInfo{
			Filepath:    files[i].Location,
			TrackNumber: int(files[i].TrackNumber),
			DiscNumber:  int(files[i].DiscNumber),
			AcoustID:    files[i].AcoustID,
		}

		// Use track metadata from DB (from the original MusicBrainz import)
		if tm, ok := trackMap[files[i].DbtrackID]; ok {
			track.Title = tm.Title
			track.MusicBrainzID = tm.MusicbrainzRecordingID

			track.ISRC = tm.ISRC
			if tm.AcoustID != "" {
				track.AcoustID = tm.AcoustID
			}
		} else if ti, err := parser_v2.ReadAudioTags(files[i].Location); err == nil && ti != nil {
			// Fallback to file tags if no DB track found
			track.Title = ti.Title
			track.Artist = ti.Artist
			track.MusicBrainzID = ti.MusicBrainzID
			track.ISRC = ti.ISRC
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

	return structure.TagAlbumFiles(ctx, config.MediaTypeMusic, embedArt, embedLyrics, album)
}

// RetagArtistAlbums re-writes audio tags for all albums by a given dbartist ID.
func RetagArtistAlbums(ctx context.Context, cfgp *config.MediaTypeConfig, artistID uint) error {
	ids := database.Getrowssize[uint](
		false,
		"SELECT count(DISTINCT a.dbalbum_id) FROM albums a JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id WHERE aa.dbartist_id = ?",
		"SELECT DISTINCT a.dbalbum_id FROM albums a JOIN dbalbum_artists aa ON a.dbalbum_id = aa.dbalbum_id WHERE aa.dbartist_id = ?",
		&artistID,
	)

	var lastErr error
	for i := range ids {
		if err := RetagAlbum(ctx, cfgp, ids[i]); err != nil {
			lastErr = err
			logger.Logtype("error", 1).
				Uint("dbalbum_id", ids[i]).
				Err(err).
				Msg("Failed to retag album")
		}
	}

	return lastErr
}

// RetagAllAlbums re-writes audio tags for all albums that have files.
func RetagAllAlbums(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	ids := database.Getrowssize[uint](false,
		"SELECT count(DISTINCT dbalbum_id) FROM album_files",
		"SELECT DISTINCT dbalbum_id FROM album_files")

	var lastErr error
	for i := range ids {
		if err := RetagAlbum(ctx, cfgp, ids[i]); err != nil {
			lastErr = err
			logger.Logtype("error", 1).
				Uint("dbalbum_id", ids[i]).
				Err(err).
				Msg("Failed to retag album")
		}
	}

	return lastErr
}

package importfeed

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// JobImportDBAlbum imports an album into the database and media lists from a ManualConfig.
// It supports three import modes based on which fields are set in the config:
//   - Full artist: ArtistName/ArtistID set - imports all albums by this artist
//   - Album series: AlbumSeriesName/AlbumSeriesID set - imports all albums in the series
//   - Single album: Only Name set - imports a single album
func JobImportDBAlbum(
	ctx context.Context, album *config.ManualConfig,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	// Determine import mode based on which fields are set
	var (
		importMode string
		importName string
	)

	if album.Name == "" && (album.ArtistName != "" || album.ArtistID != "") {
		// Artist-catalog mode: no specific title — import all releases by this artist.
		importMode = "artist"

		importName = album.ArtistName
		if importName == "" {
			importName = album.ArtistID
		}
	} else if album.AlbumSeriesName != "" || album.AlbumSeriesID != "" {
		importMode = "album_series"

		importName = album.AlbumSeriesName
		if importName == "" {
			importName = album.AlbumSeriesID
		}
	} else {
		importMode = "single"
		importName = album.Name
	}

	logger.Logtype("info", 0).
		Str("album", importName).
		Str("mode", importMode).
		Int(logger.StrRow, idx).
		Msg("Import/Update Album")

	return jobImportDBAlbum(ctx, album, cfgp, listid, importMode)
}

// AddAlbumByMusicBrainzID looks up the release on MusicBrainz by its release ID and
// adds it to dbalbums + the given list. Suitable for direct API calls from the metadata
// search UI where the caller already has the exact MBID from a prior search.
func AddAlbumByMusicBrainzID(
	ctx context.Context,
	mbid string,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if mbid == "" {
		return logger.ErrNotFound
	}

	release := apiexternal_v2.ReleaseSearchResult{
		ID:            mbid,
		MusicBrainzID: mbid,
	}

	var str string

	return addAlbumToDatabase(ctx, &release, cfgp, listid, &str)
}

// jobImportDBAlbum performs the actual album import based on the import mode.
func jobImportDBAlbum(
	ctx context.Context,
	album *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	importMode string,
) error {
	// Use MusicBrainz provider from registry
	mbProvider := providers.GetMusicBrainz()

	switch importMode {
	case "artist":
		return importAlbumsByArtist(ctx, album, cfgp, listid, mbProvider)
	case "album_series":
		return importAlbumsBySeries(ctx, album, cfgp, listid, mbProvider)
	case "single":
		return importSingleAlbum(ctx, album, cfgp, listid, mbProvider)
	}

	return nil
}

// releaseMatchesMBFormats reports whether every disc in r has a format that appears in formats.
// If formats is empty, all releases pass.
// Comparison is case-insensitive.
// A release with no media information is accepted when formats is empty, rejected otherwise.
func releaseMatchesMBFormats(r *apiexternal_v2.ReleaseSearchResult, formats []string) bool {
	if len(formats) == 0 {
		return true
	}

	if len(r.MediaFormats) == 0 {
		// No disc format info from MB — cannot verify; skip it
		return false
	}

	for i := range r.MediaFormats {
		matched := false
		for j := range formats {
			if strings.EqualFold(r.MediaFormats[i], formats[j]) {
				matched = true
				break
			}
		}

		if !matched {
			return false
		}
	}

	return true
}

// importAlbumsByArtist imports all albums by an artist.
func importAlbumsByArtist(
	ctx context.Context,
	album *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	mbProvider *musicbrainz.Provider,
) error {
	artistName := album.ArtistName
	artistID := album.ArtistID

	mediaFormats := album.MBMediaFormats

	// importAllReleasesByMBID fetches all releases for a given MusicBrainz artist ID, paginating as needed.
	importAllReleasesByMBID := func(mbid string) error {
		const pageSize = 100
		for offset := 0; ; offset += pageSize {
			if err := logger.CheckContextEnded(ctx); err != nil {
				return err
			}

			releases, err := mbProvider.GetArtistReleases(ctx, mbid, pageSize, offset)
			if err != nil {
				return err
			}

			for i := range releases {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if !releaseMatchesMBFormats(&releases[i], mediaFormats) {
					continue
				}

				if err := addAlbumToDatabase(
					ctx,
					&releases[i],
					cfgp,
					listid,
					&artistName,
				); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("album", releases[i].Title).
						Msg("Failed to add album")
				}
			}

			if len(releases) < pageSize {
				break
			}
		}

		return nil
	}

	// If we have an artist ID (MusicBrainz MBID), use it directly
	if artistID != "" {
		if err := importAllReleasesByMBID(artistID); err == nil {
			return nil
		}
	}

	// Search for artist by name first
	if artistName != "" {
		artists, err := mbProvider.SearchArtists(ctx, artistName, 5)
		if err == nil && len(artists) > 0 {
			// Use the first (best) match
			if err := importAllReleasesByMBID(artists[0].MusicBrainzID); err == nil {
				return nil
			}
		}
	}

	return logger.ErrNotFound
}

// importAlbumsBySeries imports albums by series name (e.g., "Bravo Hits", "Now That's What I Call Music").
func importAlbumsBySeries(
	ctx context.Context,
	album *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	mbProvider *musicbrainz.Provider,
) error {
	seriesName := album.AlbumSeriesName
	if seriesName == "" {
		seriesName = album.AlbumSeriesID
	}

	if seriesName != "" {
		mediaFormats := album.MBMediaFormats

		// Build Lucene query: optionally append format filter terms for server-side pre-filtering.
		// Example: "Bravo Hits AND (format:CD)"
		query := seriesName
		if len(mediaFormats) > 0 {
			var fmtParts []string
			for i := range mediaFormats {
				fmtParts = append(fmtParts, "format:"+mediaFormats[i])
			}

			query = logger.JoinStrings(
				seriesName,
				" AND (",
				logger.JoinStringsSep(fmtParts, " OR "),
				")",
			)
		}

		const pageSize = 100

		var str string
		for offset := 0; ; offset += pageSize {
			if err := logger.CheckContextEnded(ctx); err != nil {
				return err
			}

			releases, total, err := mbProvider.SearchReleases(ctx, query, pageSize, offset)
			if err != nil {
				return err
			}

			for i := range releases {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				// Client-side filter as a safety net (server-side Lucene filter is best-effort)
				if !releaseMatchesMBFormats(&releases[i], mediaFormats) {
					continue
				}

				if err := addAlbumToDatabase(ctx, &releases[i], cfgp, listid, &str); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("album", releases[i].Title).
						Msg("Failed to add album")
				}
			}

			if offset+len(releases) >= total || len(releases) < pageSize {
				break
			}
		}

		return nil
	}

	return logger.ErrNotFound
}

// importSingleAlbum imports a single album by name.
func importSingleAlbum(
	ctx context.Context,
	album *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	mbProvider *musicbrainz.Provider,
) error {
	if album.Name == "" {
		return logger.ErrNotFound
	}

	query := string(BuildArtistAlbumSearch(album.Name, album.ArtistName))

	releases, _, err := mbProvider.SearchReleases(ctx, query, 10, 0)
	if err == nil && len(releases) > 0 {
		isVA := IsVariousArtists(album.ArtistName)
		for i := range releases {
			if !isVA && album.ArtistName != "" {
				artistMatch := false
				for j := range releases[i].Artists {
					if strings.EqualFold(releases[i].Artists[j].Name, album.ArtistName) {
						artistMatch = true
						break
					}
				}

				if !artistMatch {
					continue
				}
			}

			return addAlbumToDatabase(ctx, &releases[i], cfgp, listid, &album.ArtistName)
		}
	}

	return logger.ErrNotFound
}

// addAlbumToDatabase adds an album to the database.
func addAlbumToDatabase(
	ctx context.Context,
	release *apiexternal_v2.ReleaseSearchResult,
	cfgp *config.MediaTypeConfig,
	listid int,
	artistName *string,
) error {
	if release == nil || release.Title == "" {
		return logger.ErrNotFound
	}

	// Fetch full release details to get tracks and calculate runtime
	var (
		releaseDetails *apiexternal_v2.ReleaseDetails
		totalRuntimeMs int64
	)

	mbProvider := getMusicBrainzProvider()
	if release.MusicBrainzID != "" || release.ReleaseGroupID != "" {
		mbid := release.MusicBrainzID
		if mbid == "" {
			mbid = release.ReleaseGroupID
		}

		var err error

		releaseDetails, err = retryOnRateLimit(ctx, func() (*apiexternal_v2.ReleaseDetails, error) {
			return mbProvider.GetReleaseByID(ctx, mbid)
		})
		if err == nil && releaseDetails != nil {
			// Calculate total runtime from tracks
			for i := range releaseDetails.Tracks {
				totalRuntimeMs += releaseDetails.Tracks[i].Duration.Milliseconds()
			}
		}
	}

	// Check if album already exists by various IDs
	var existingID uint
	if release.MusicBrainzID != "" {
		database.Scanrowsdyn(false,
			"SELECT id FROM dbalbums WHERE musicbrainz_release_id = ?",
			&existingID, &release.MusicBrainzID,
		)
	}

	if existingID == 0 && release.ReleaseGroupID != "" {
		database.Scanrowsdyn(false,
			"SELECT id FROM dbalbums WHERE musicbrainz_release_group_id = ?",
			&existingID, &release.ReleaseGroupID,
		)
	}

	if existingID == 0 && release.DiscogsID > 0 {
		discogsStr := strconv.Itoa(release.DiscogsID)
		database.Scanrowsdyn(false,
			"SELECT id FROM dbalbums WHERE discogs_release_id = ?",
			&existingID, &discogsStr,
		)
	}

	if existingID == 0 && release.Barcode != "" {
		database.Scanrowsdyn(false,
			"SELECT id FROM dbalbums WHERE upc = ?",
			&existingID, &release.Barcode,
		)
	}

	// Last-resort dedup: same title + same release year prevents duplicate rows when
	// different MB search runs return different release objects for the same album.
	if existingID == 0 && release.Title != "" && release.ReleaseYear > 0 {
		releaseYear := uint16(
			release.ReleaseYear,
		)
		database.Scanrowsdyn(false,
			"SELECT id FROM dbalbums WHERE LOWER(title) = LOWER(?) AND year = ?",
			&existingID,
			&release.Title, &releaseYear,
		)
	}

	var dbalbumID uint

	slug := logger.StringToSlug(release.Title)
	year := uint16(release.ReleaseYear) //nolint:gosec // safe: value within target type range

	// Convert genres slice to comma-separated string
	genres := ""
	if len(release.Genres) > 0 {
		genres = logger.JoinStringsSep(release.Genres, ", ")
	}

	if existingID > 0 {
		dbalbumID = existingID
		// Update existing album with any new metadata
		_, _ = database.ExecNid(
			`UPDATE dbalbums SET
				cover_url = CASE WHEN cover_url = '' OR cover_url IS NULL THEN ? ELSE cover_url END,
				label = CASE WHEN label = '' OR label IS NULL THEN ? ELSE label END,
				country = CASE WHEN country = '' OR country IS NULL THEN ? ELSE country END,
				release_type = CASE WHEN release_type = '' OR release_type IS NULL THEN ? ELSE release_type END,
				total_tracks = CASE WHEN total_tracks = 0 THEN ? ELSE total_tracks END,
				total_runtime_ms = CASE WHEN total_runtime_ms = 0 THEN ? ELSE total_runtime_ms END,
				genres = CASE WHEN genres = '' OR genres IS NULL THEN ? ELSE genres END,
				updated_at = current_timestamp
			 WHERE id = ?`,
			&release.CoverURL, &release.Label, &release.Country, &release.Type,
			&release.TrackCount, &totalRuntimeMs, &genres, &dbalbumID,
		)
		logger.Logtype("debug", 1).
			Str("album", release.Title).
			Uint("id", dbalbumID).
			Msg("Album already exists in database")
	} else {
		// Insert new dbalbum with all available metadata
		discogsStr := ""
		if release.DiscogsID > 0 {
			discogsStr = strconv.Itoa(release.DiscogsID)
		}

		masterStr := ""
		if release.MasterID > 0 {
			masterStr = strconv.Itoa(release.MasterID)
		}

		result, err := database.ExecNid(
			`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id, discogs_release_id, discogs_master_id,
			 upc, slug, year, release_type, format, label, country, total_tracks, total_runtime_ms, genres, cover_url)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			&release.Title,
			&release.ReleaseGroupID,
			&release.MusicBrainzID,
			&discogsStr,
			&masterStr,
			&release.Barcode,
			&slug,
			&year,
			&release.Type,
			&release.Format,
			&release.Label,
			&release.Country,
			&release.TrackCount,
			&totalRuntimeMs,
			&genres,
			&release.CoverURL,
		)
		if err != nil {
			return err
		}

		dbalbumID = logger.Int64ToUint(result)
		logger.Logtype("info", 0).
			Str("album", release.Title).
			Uint("id", dbalbumID).
			Msg("Added album to database")

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheThreeString(
				logger.CacheDBAlbum,
				syncops.DbstaticThreeStringTwoInt{
					Str1: release.Title,
					Str2: slug,
					Str3: release.MusicBrainzID,
					Num1: int(year),
					Num2: dbalbumID,
				},
			)
		}
	}

	// Fetch and store series name from MusicBrainz if available
	releaseGroupID := release.MusicBrainzID
	if releaseGroupID == "" {
		releaseGroupID = release.ReleaseGroupID
	}

	if releaseGroupID != "" && dbalbumID > 0 {
		// Skip the HTTP call if series_name is already stored.
		existing := database.Getdatarow[string](
			false,
			"SELECT series_name FROM dbalbums WHERE id = ? AND (series_name IS NOT NULL AND series_name != '')",
			&dbalbumID,
		)
		if existing == "" {
			_, seriesName, seriesErr := getSeriesIDFromReleaseGroup(ctx, releaseGroupID)
			if seriesErr == nil && seriesName != "" {
				database.ExecN(
					"UPDATE dbalbums SET series_name = ? WHERE id = ?",
					&seriesName,
					&dbalbumID,
				)
				logger.Logtype("info", 1).
					Str("album", release.Title).
					Str("series", seriesName).
					Msg("Stored album series name")
			}
		}
	}

	// Add tracks to the database if we have release details
	if releaseDetails != nil && len(releaseDetails.Tracks) > 0 {
		var existingTrack, dbtrackID uint
		var runtimeMs int64
		for i := range releaseDetails.Tracks {
			existingTrack = 0
			dbtrackID = 0
			// Check if track already exists
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbtracks WHERE dbalbum_id = ? AND disc_number = ? AND track_number = ?",
				&existingTrack,
				&dbalbumID,
				&releaseDetails.Tracks[i].DiscNumber,
				&releaseDetails.Tracks[i].Position,
			)

			if existingTrack == 0 {
				runtimeMs = releaseDetails.Tracks[i].Duration.Milliseconds()

				result, err := database.ExecNid(
					`INSERT INTO dbtracks (dbalbum_id, title, track_number, disc_number, runtime_ms, acoustid)
					 VALUES (?, ?, ?, ?, ?, ?)`,
					&dbalbumID,
					&releaseDetails.Tracks[i].Title,
					&releaseDetails.Tracks[i].Position,
					&releaseDetails.Tracks[i].DiscNumber,
					&runtimeMs,
					&releaseDetails.Tracks[i].AcoustID,
				)
				if err == nil {
					dbtrackID = logger.Int64ToUint(result)
				}
			} else {
				dbtrackID = existingTrack
			}

			// Add track artists if available
			if dbtrackID > 0 && len(releaseDetails.Tracks[i].Artists) > 0 {
				var artistID uint
				for idx := range releaseDetails.Tracks[i].Artists {
					if releaseDetails.Tracks[i].Artists[idx].Name == "" {
						continue
					}

					artistID = addOrGetArtist(
						&releaseDetails.Tracks[i].Artists[idx].Name,
						&releaseDetails.Tracks[i].Artists[idx].ID,
					)
					if artistID > 0 {
						var existingRelation uint
						database.Scanrowsdyn(
							false,
							"SELECT id FROM dbtrack_artists WHERE dbtrack_id = ? AND dbartist_id = ?",
							&existingRelation,
							&dbtrackID,
							&artistID,
						)

						if existingRelation == 0 {
							_, _ = database.ExecNid(
								`INSERT INTO dbtrack_artists (dbtrack_id, dbartist_id, position)
								 VALUES (?, ?, ?)`,
								&dbtrackID, &artistID, &idx,
							)
						}
					}
				}
			}
		}
	}

	// Add artists to the database and create relationships
	var artistID uint
	for idx := range release.Artists {
		if release.Artists[idx].Name == "" {
			continue
		}

		artistID = addOrGetArtist(&release.Artists[idx].Name, &release.Artists[idx].ID)
		if artistID > 0 {
			// Check if relationship already exists
			var existingRelation uint
			database.Scanrowsdyn(false,
				"SELECT id FROM dbalbum_artists WHERE dbalbum_id = ? AND dbartist_id = ?",
				&existingRelation, &dbalbumID, &artistID)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position)
					 VALUES (?, ?, ?)`,
					&dbalbumID, &artistID, &idx,
				)
			}
		}
	}

	// Also add the artistName parameter if provided and not already in Artists list
	if artistName != nil && *artistName != "" {
		idx := slices.IndexFunc(
			release.Artists,
			func(a apiexternal_v2.ArtistRef) bool { return strings.EqualFold(a.Name, *artistName) },
		)

		if idx == -1 {
			var artistNameMBID string
			if releaseDetails != nil {
				if di := slices.IndexFunc(
					releaseDetails.Artists,
					func(a apiexternal_v2.ArtistRef) bool { return strings.EqualFold(a.Name, *artistName) },
				); di != -1 {
					artistNameMBID = releaseDetails.Artists[di].ID
				}
			}

			artistID := addOrGetArtist(artistName, &artistNameMBID)
			if artistID > 0 {
				var existingRelation uint
				database.Scanrowsdyn(false,
					"SELECT id FROM dbalbum_artists WHERE dbalbum_id = ? AND dbartist_id = ?",
					&existingRelation, &dbalbumID, &artistID)

				if existingRelation == 0 {
					_, _ = database.ExecNid(
						`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position)
						 VALUES (?, ?, ?)`,
						&dbalbumID, &artistID, len(release.Artists),
					)
				}
			}
		}
	}

	// Get or create tracked artists for artists on the release.
	// For single-artist releases: track the artist with track_mode='albums'.
	// For 2-3 artist collaborations: primary gets 'albums', secondary get 'none'.
	// For compilations (4+ artists like "Bravo Hits"): skip creating new tracking entries entirely.
	var trackedArtistID uint

	isMultiArtist := len(release.Artists) > 1
	isCompilation := len(release.Artists) > 3
	listName2 := cfgp.Lists[listid].Name

	if !isCompilation {
		var (
			dbartistID, existingTrackedID uint
			artistSlugSearch              string
		)

		for i := range release.Artists {
			relArtistRef := release.Artists[i]
			if relArtistRef.Name == "" {
				continue
			}

			dbartistID = 0
			existingTrackedID = 0

			artistSlugSearch = logger.StringToSlug(relArtistRef.Name)
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR slug = ?",
				&dbartistID,
				&relArtistRef.Name,
				&artistSlugSearch,
			)

			if dbartistID == 0 {
				continue
			}

			// Check if tracked artist already exists
			database.Scanrowsdyn(
				false,
				"SELECT id FROM artists WHERE dbartist_id = ? AND listname = ?",
				&existingTrackedID,
				&dbartistID,
				&listName2,
			)

			// Determine if this is the primary artist we want to track
			isPrimary := !isMultiArtist ||
				(artistName != nil && *artistName != "" && strings.EqualFold(relArtistRef.Name, *artistName))

			trackMode := "albums"
			if isMultiArtist && !isPrimary {
				trackMode = "none"
			}

			if existingTrackedID == 0 {
				result, err := database.ExecNid(
					`INSERT INTO artists (dbartist_id, listname, track_mode, dont_search)
					 VALUES (?, ?, ?, 0)`,
					&dbartistID, &listName2, &trackMode,
				)
				if err == nil {
					newID := logger.Int64ToUint(result)
					if isPrimary {
						trackedArtistID = newID
					}

					logger.Logtype("debug", 1).
						Str("artist", relArtistRef.Name).
						Str("list", listName2).
						Str("track_mode", trackMode).
						Msg("Added artist to tracking list")
				}
			} else if isPrimary {
				trackedArtistID = existingTrackedID
			}
		}
	}

	// For compilations or if artistName wasn't found above, find/create tracking entry for artistName only.
	if artistName != nil && *artistName != "" && trackedArtistID == 0 {
		var dbartistID uint

		artistSlugSearch := logger.StringToSlug(*artistName)
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR slug = ?",
			&dbartistID,
			artistName,
			&artistSlugSearch,
		)

		if dbartistID > 0 {
			database.Scanrowsdyn(
				false,
				"SELECT id FROM artists WHERE dbartist_id = ? AND listname = ?",
				&trackedArtistID,
				&dbartistID,
				&listName2,
			)

			if trackedArtistID == 0 {
				result, err := database.ExecNid(
					`INSERT INTO artists (dbartist_id, listname, track_mode, dont_search)
					 VALUES (?, ?, 'albums', 0)`,
					&dbartistID, &listName2,
				)
				if err == nil {
					trackedArtistID = logger.Int64ToUint(result)
				}
			}
		}
	}

	// Check if tracked album already exists
	listName := cfgp.Lists[listid].Name

	var trackedID uint
	database.Scanrowsdyn(
		false,
		"SELECT id FROM albums WHERE dbalbum_id = ? AND listname = ?",
		&trackedID,
		&dbalbumID,
		&listName,
	)

	if trackedID == 0 {
		albumID, err := database.ExecNid(
			`INSERT INTO albums (dbalbum_id, listname, missing, quality_profile, artist_id)
			 VALUES (?, ?, 1, ?, ?)`,
			&dbalbumID, &listName, &cfgp.Lists[listid].CfgQuality.Name, &trackedArtistID,
		)
		if err != nil {
			return err
		}

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				logger.CacheAlbum,
				syncops.DbstaticOneStringTwoInt{
					Str:  listName,
					Num1: dbalbumID,
					Num2: logger.Int64ToUint(albumID),
				},
			)
		}

		logger.Logtype("info", 0).
			Str("album", release.Title).
			Str("list", listName).
			Msg("Added album to tracking list")
	} else if trackedArtistID > 0 {
		// Update existing album to link to artist if not already linked
		_, _ = database.ExecNid(
			`UPDATE albums SET artist_id = ? WHERE id = ? AND artist_id = 0`,
			&trackedArtistID, &trackedID,
		)
	}

	return nil
}

// addOrGetArtist finds an existing artist by name or creates a new one.
// mbid is the MusicBrainz artist ID (may be empty). It is stored on INSERT and
// back-filled on existing rows that currently have no musicbrainz_id.
// Returns the artist ID.
func addOrGetArtist(artistName, mbid *string) uint {
	if artistName == nil || *artistName == "" {
		return 0
	}

	// Check if artist already exists
	artistSlug := logger.StringToSlug(*artistName)
	artistID := database.Getdatarow[uint](false,
		"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR slug = ?",
		artistName, &artistSlug,
	)

	if artistID > 0 {
		// Back-fill MBID if we now have one and the row has none.
		if mbid != nil && *mbid != "" {
			_, _ = database.ExecNid(
				`UPDATE dbartists SET musicbrainz_id = ? WHERE id = ? AND (musicbrainz_id IS NULL OR musicbrainz_id = "")`,
				&mbid,
				&artistID,
			)
		}

		return artistID
	}

	// Insert new artist
	result, err := database.ExecNid(
		`INSERT INTO dbartists (name, slug, musicbrainz_id) VALUES (?, ?, ?)`,
		&artistName,
		&artistSlug,
		&mbid,
	)
	if err != nil {
		logger.Logtype("error", 1).
			Err(err).
			Str("artist", *artistName).
			Msg("Failed to insert artist")

		return 0
	}

	return logger.Int64ToUint(result)
}

// DiscoverAndAddArtistAlbums discovers other albums by the same artist and adds them to the database.
// This is called after successfully importing an album to automatically add the artist's discography.
func DiscoverAndAddArtistAlbums(
	ctx context.Context,
	artistName *string,
	cfgp *config.MediaTypeConfig,
	listid int,
	maxAlbums int,
	allowedReleaseTypes []string,
	mediaFormats []string,
) int {
	if artistName == nil || *artistName == "" || listid == -1 {
		return 0
	}

	logger.Logtype("info", 1).
		Str("artist", *artistName).
		Str("list", cfgp.Lists[listid].Name).
		Msg("Discovering other albums by artist")

	// Get MusicBrainz provider
	mbProvider := getMusicBrainzProvider()

	// Search for the artist
	artists, err := mbProvider.SearchArtists(ctx, *artistName, 5)
	if err != nil || len(artists) == 0 {
		logger.Logtype("debug", 1).
			Str("artist", *artistName).
			Err(err).
			Msg("Failed to find artist on MusicBrainz")

		return 0
	}

	// Get the best match (first result)
	artist := artists[0]

	logger.Logtype("debug", 1).
		Str("artist", *artistName).
		Str("mbid", artist.MusicBrainzID).
		Str("matched_name", artist.Name).
		Msg("Found artist on MusicBrainz")

	// Get artist's releases
	releases, err := mbProvider.GetArtistReleases(ctx, artist.MusicBrainzID, maxAlbums, 0)
	if err != nil || len(releases) == 0 {
		logger.Logtype("debug", 1).
			Str("artist", *artistName).
			Str("mbid", artist.MusicBrainzID).
			Err(err).
			Msg("Failed to get artist releases from MusicBrainz")

		return 0
	}

	logger.Logtype("info", 1).
		Str("artist", *artistName).
		Int("releases_found", len(releases)).
		Msg("Found releases by artist")

	// Add each release to the database
	albumsAdded := 0
	for i := range releases {
		if err := ctx.Err(); err != nil {
			break
		}

		// Skip if we already have this album
		var existingID uint
		if releases[i].MusicBrainzID != "" {
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbalbums WHERE musicbrainz_release_id = ?",
				&existingID,
				&releases[i].MusicBrainzID,
			)
		}

		if existingID > 0 {
			logger.Logtype("debug", 2).
				Str("album", releases[i].Title).
				Str("artist", *artistName).
				Msg("Album already in database, skipping")

			continue
		}

		// Filter by allowed release types
		if len(allowedReleaseTypes) > 0 && releases[i].Type != "" {
			if !logger.SlicesContainsI(allowedReleaseTypes, releases[i].Type) {
				logger.Logtype("debug", 2).
					Str("album", releases[i].Title).
					Str("type", releases[i].Type).
					Msg("Release type not allowed, skipping")

				continue
			}
		}

		// Filter by media format (e.g. only CD releases)
		if !releaseMatchesMBFormats(&releases[i], mediaFormats) {
			logger.Logtype("debug", 2).
				Str("album", releases[i].Title).
				Strs("disc_formats", releases[i].MediaFormats).
				Msg("Release format not allowed, skipping")

			continue
		}

		// Add the album
		err := addAlbumToDatabase(ctx, &releases[i], cfgp, listid, artistName)
		if err == nil {
			albumsAdded++

			logger.Logtype("info", 1).
				Str("album", releases[i].Title).
				Str("artist", *artistName).
				Int("year", releases[i].ReleaseYear).
				Msg("Added album from artist discovery")
		} else {
			logger.Logtype("debug", 2).
				Str("album", releases[i].Title).
				Str("artist", *artistName).
				Err(err).
				Msg("Failed to add album from artist discovery")
		}
	}

	if albumsAdded > 0 {
		logger.Logtype("info", 0).
			Str("artist", *artistName).
			Int("albums_added", albumsAdded).
			Int("total_releases", len(releases)).
			Msg("Artist discovery completed")
	}

	return albumsAdded
}

// mbReleaseGroupSeriesResponse is the JSON response shape for a release-group?inc=series-rels request.
type mbReleaseGroupSeriesResponse struct {
	Relations []struct {
		Type       string `json:"type"`
		TypeID     string `json:"type-id"`
		TargetType string `json:"target-type"`
		Series     struct {
			ID   string `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"series"`
	} `json:"relations"`
}

// mbSeriesReleaseGroupsResponse is the JSON response shape for a series?inc=release-group-rels request.
type mbSeriesReleaseGroupsResponse struct {
	Relations []struct {
		Type         string `json:"type"`
		TargetType   string `json:"target-type"`
		ReleaseGroup struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"release-group"`
	} `json:"relations"`
}

// getSeriesIDFromReleaseGroup retrieves the series ID and name if the release group is part of a series.
func getSeriesIDFromReleaseGroup(
	ctx context.Context,
	releaseGroupID string,
) (string, string, error) {
	mbProvider := getMusicBrainzProvider()

	endpoint := logger.JoinStrings("/release-group/", releaseGroupID, "?inc=series-rels&fmt=json")

	var response mbReleaseGroupSeriesResponse
	if err := mbProvider.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return "", "", fmt.Errorf("failed to get release group series: %w", err)
	}

	// Look for "part of" series relationship
	for i := range response.Relations {
		if response.Relations[i].TargetType == "series" && response.Relations[i].Type == "part of" {
			logger.Logtype("debug", 2).
				Str("release_group_id", releaseGroupID).
				Str("series_id", response.Relations[i].Series.ID).
				Str("series_name", response.Relations[i].Series.Name).
				Msg("Found series relationship")

			return response.Relations[i].Series.ID, response.Relations[i].Series.Name, nil
		}
	}

	return "", "", fmt.Errorf("release group is not part of a series")
}

// getReleaseGroupsInSeries retrieves all release groups that are part of a series.
func getReleaseGroupsInSeries(ctx context.Context, seriesID string) ([]string, error) {
	mbProvider := getMusicBrainzProvider()

	endpoint := logger.JoinStrings("/series/", seriesID, "?inc=release-group-rels&fmt=json")

	var response mbSeriesReleaseGroupsResponse
	if err := mbProvider.MakeRequest(ctx, "GET", endpoint, nil, &response, nil); err != nil {
		return nil, fmt.Errorf("failed to get series release groups: %w", err)
	}

	// Extract all release group IDs
	var releaseGroupIDs []string
	for i := range response.Relations {
		if response.Relations[i].TargetType == "release-group" &&
			response.Relations[i].Type == "part of" {
			releaseGroupIDs = append(releaseGroupIDs, response.Relations[i].ReleaseGroup.ID)
		}
	}

	if len(releaseGroupIDs) == 0 {
		return nil, fmt.Errorf("no release groups found in series")
	}

	logger.Logtype("debug", 2).
		Str("series_id", seriesID).
		Int("count", len(releaseGroupIDs)).
		Msg("Found release groups in series")

	return releaseGroupIDs, nil
}

// DiscoverAndAddSeriesAlbums discovers other albums in the same series and adds them to the database.
// This is used for "Various Artists" compilations where albums are part of a MusicBrainz series (like "Bravo Hits").
func DiscoverAndAddSeriesAlbums(
	ctx context.Context,
	releaseGroupID string,
	albumTitle string,
	cfgp *config.MediaTypeConfig,
	listid int,
	allowedReleaseTypes []string,
	mediaFormats []string,
) int {
	if releaseGroupID == "" || listid == -1 {
		return 0
	}

	logger.Logtype("info", 1).
		Str("album", albumTitle).
		Str("release_group_id", releaseGroupID).
		Str("list", cfgp.Lists[listid].Name).
		Msg("Checking if album is part of a series")

	// Check if this release group is part of a series
	seriesID, _, err := getSeriesIDFromReleaseGroup(ctx, releaseGroupID)
	if err != nil {
		logger.Logtype("debug", 1).
			Str("release_group_id", releaseGroupID).
			Err(err).
			Msg("Release group is not part of a series")

		return 0
	}

	// Get all release groups in the series
	releaseGroupIDs, err := getReleaseGroupsInSeries(ctx, seriesID)
	if err != nil {
		logger.Logtype("debug", 1).
			Str("series_id", seriesID).
			Err(err).
			Msg("Failed to get release groups from series")

		return 0
	}

	logger.Logtype("info", 1).
		Str("album", albumTitle).
		Str("series_id", seriesID).
		Int("release_groups_found", len(releaseGroupIDs)).
		Msg("Found release groups in series")

	mbProvider := getMusicBrainzProvider()
	albumsAdded := 0

	// For each release group in the series, get one release and add it
	for j := range releaseGroupIDs {
		if err := ctx.Err(); err != nil {
			break
		}

		// Skip the original release group we already have
		if releaseGroupIDs[j] == releaseGroupID {
			continue
		}

		// Get the release group details
		releaseGroupDetails, err := mbProvider.GetReleaseGroupByID(ctx, releaseGroupIDs[j])
		if err != nil {
			logger.Logtype("debug", 2).
				Str("release_group_id", releaseGroupIDs[j]).
				Err(err).
				Msg("Failed to get release group details")

			continue
		}

		// Skip if we already have this album
		var existingID uint
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbalbums WHERE musicbrainz_release_group_id = ?",
			&existingID,
			&releaseGroupIDs[j],
		)

		if existingID > 0 {
			logger.Logtype("debug", 2).
				Str("album", releaseGroupDetails.Title).
				Msg("Album already in database, skipping")
			continue
		}

		// Filter by allowed release types
		if len(allowedReleaseTypes) > 0 && releaseGroupDetails.Type != "" {
			if !logger.SlicesContainsI(allowedReleaseTypes, releaseGroupDetails.Type) {
				logger.Logtype("debug", 2).
					Str("album", releaseGroupDetails.Title).
					Str("type", releaseGroupDetails.Type).
					Msg("Release type not allowed, skipping")

				continue
			}
		}

		var releaseToAdd *apiexternal_v2.ReleaseSearchResult

		if len(mediaFormats) > 0 {
			// Release-group details have no per-disc format info.
			// Search for individual releases in this group and pick the first one matching the format filter.
			rgReleases, _, rgErr := mbProvider.SearchReleases(
				ctx,
				fmt.Sprintf("rgid:%s", releaseGroupIDs[j]),
				50,
				0,
			)
			if rgErr != nil || len(rgReleases) == 0 {
				logger.Logtype("debug", 2).
					Str("release_group_id", releaseGroupIDs[j]).
					Msg("No individual releases found for format check, skipping")
				continue
			}

			for i := range rgReleases {
				if releaseMatchesMBFormats(&rgReleases[i], mediaFormats) {
					r := rgReleases[i]

					releaseToAdd = &r
					break
				}
			}

			if releaseToAdd == nil {
				logger.Logtype("debug", 2).
					Str("album", releaseGroupDetails.Title).
					Strs("wanted_formats", mediaFormats).
					Msg("No release in group matches format filter, skipping")

				continue
			}
		} else {
			// No format filter — use release group details directly (existing behaviour).
			r := apiexternal_v2.ReleaseSearchResult{
				ID:             releaseGroupDetails.ID,
				Title:          releaseGroupDetails.Title,
				MusicBrainzID:  releaseGroupDetails.MusicBrainzID,
				ReleaseGroupID: releaseGroupIDs[j],
				Artists:        releaseGroupDetails.Artists,
				ReleaseYear:    releaseGroupDetails.ReleaseYear,
				ProviderType:   apiexternal_v2.ProviderMusicBrainz,
			}

			releaseToAdd = &r
		}

		// Add the album - use "Various Artists" as the artist name
		strArtistName := VariousArtistsName

		err = addAlbumToDatabase(ctx, releaseToAdd, cfgp, listid, &strArtistName)
		if err == nil {
			albumsAdded++

			logger.Logtype("info", 1).
				Str("album", releaseToAdd.Title).
				Int("year", releaseToAdd.ReleaseYear).
				Msg("Added album from series discovery")
		} else {
			logger.Logtype("debug", 2).
				Str("album", releaseToAdd.Title).
				Err(err).
				Msg("Failed to add album from series discovery")
		}
	}

	if albumsAdded > 0 {
		logger.Logtype("info", 0).
			Str("series_id", seriesID).
			Int("albums_added", albumsAdded).
			Int("total_release_groups", len(releaseGroupIDs)).
			Msg("Series discovery completed")
	}

	return albumsAdded
}

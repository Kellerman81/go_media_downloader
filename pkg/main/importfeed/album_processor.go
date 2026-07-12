package importfeed

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// ProcessAudioFolderAsAlbum processes a folder of audio files as an album.
// It collects files, matches to API, verifies track count, and organizes the album.
// This is exported so both utils and structure packages can call it.
func ProcessAudioFolderAsAlbum(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) bool {
	_, reason, _ := MatchAudioFolderAsAlbum(ctx, folder, cfgp, data, true)
	return reason == ""
}

const (
	isrcMaxFiles      = 5   // maximum files to read tags from for ISRC collection
	acoustidMaxFiles  = 5   // maximum files to fingerprint per album
	acoustidRelThresh = 0.6 // fraction of files that must share a release (beets COMMON_REL_THRESH)
)

// MatchAudioFolderAsAlbumForced is like MatchAudioFolderAsAlbum but bypasses normal
// candidate search and matches against a specific MusicBrainz release ID (music) or
// ASIN (audiobook) supplied by the caller.
func MatchAudioFolderAsAlbumForced(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
	forcedID *string,
) (*parser_v2.AlbumInfo, string, *MatchReport) {
	if forcedID == nil || *forcedID == "" {
		return MatchAudioFolderAsAlbum(ctx, folder, cfgp, data, addToDatabase)
	}

	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil || len(files) == 0 {
		return nil, "no_files", nil
	}

	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		return matchAudiobookFolderForced(ctx, folder, files, cfgp, data, addToDatabase, forcedID)
	case config.MediaTypeMusic:
		return matchMusicFolderForced(ctx, folder, files, cfgp, data, addToDatabase, forcedID)
	default:
		return nil, "no_match", nil
	}
}

func MatchAudioFolderAsAlbum(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
) (*parser_v2.AlbumInfo, string, *MatchReport) {
	// Step 1: Collect files only (no tag reading yet)
	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Err(err).
			Msg("CollectFilesOnly returned error")

		return nil, "files_error", nil
	}

	// If this is a disc subfolder (e.g. "CD3"), redirect to the parent album folder
	// and collect all sibling disc files so the full album is matched together.
	if len(files) > 0 && looksLikeDiscFolder(filepath.Base(folder)) {
		parent := filepath.Dir(folder)
		parentEntries, _ := os.ReadDir(parent)

		var (
			combined []string
			allDisc  = true
		)
		for _, e := range parentEntries {
			if !e.IsDir() {
				continue
			}

			sub := filepath.Join(parent, e.Name())

			sf, _ := parser_v2.CollectFilesOnly(sub, parser_v2.AudioExtensions)
			if len(sf) == 0 {
				continue
			}

			if !looksLikeDiscFolder(e.Name()) {
				allDisc = false
				break
			}

			combined = append(combined, sf...)
		}

		if allDisc && len(combined) > len(files) {
			folder = parent
			files = combined
		}
	}

	if len(files) == 0 {
		// Pattern 4: no audio files at root — check subfolders.
		// If there is exactly one audio subfolder, use it.
		// If all audio subfolders are disc-type (CD1, Disc2, …), combine their files
		// and keep the parent folder so the full multi-disc album is matched together.
		entries, err2 := os.ReadDir(folder)
		if err2 != nil {
			return nil, "no_files", nil
		}

		var (
			firstAudioSub string
			allFiles      []string
			allDisc       = true
			subCount      int
		)
		for i := range entries {
			if !entries[i].IsDir() {
				continue
			}

			subPath := filepath.Join(folder, entries[i].Name())

			sf, err3 := parser_v2.CollectFilesOnly(subPath, parser_v2.AudioExtensions)
			if err3 != nil || len(sf) == 0 {
				continue
			}

			subCount++
			if subCount == 1 {
				firstAudioSub = subPath
			}

			if !looksLikeDiscFolder(entries[i].Name()) {
				allDisc = false
			}

			allFiles = append(allFiles, sf...)
		}

		switch subCount {
		case 0:
			return nil, "no_files", nil
		case 1:
			folder = firstAudioSub
			files = allFiles

		default:
			// Multiple audio subfolders: only valid when all are disc-type.
			if !allDisc {
				return nil, "no_files", nil
			}

			// Keep folder = parent album folder; combine all disc files.
			files = allFiles
		}
	}

	// Route to appropriate handler based on media type
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		return matchAudiobookFolder(ctx, folder, files, cfgp, data, addToDatabase)
	case config.MediaTypeMusic:
		result, status, report := matchMusicFolder(ctx, folder, files, cfgp, data, addToDatabase)
		if status != "no_match" {
			return result, status, report
		}

		// Fingerprint/LastFM fallback: only runs when normal matching found nothing.
		folderArtist, folderAlbum, _ := parser_v2.ParseAudioFolder(folder)
		if mbid := resolveAlbumMBID(ctx, folder, files, folderArtist, folderAlbum); mbid != "" {
			return matchMusicFolder(ctx, folder, files, cfgp, data, addToDatabase, mbid)
		}

		return result, status, report

	default:
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Uint("mediaType", cfgp.IsType).
			Msg("DEBUG: Unsupported media type for audio folder processing")

		return nil, "no_match", nil
	}
}

// searchPair holds an author+title combination to try against a search API.
type searchPair struct {
	author string
	title  string
	source string
}

// verifyASINChapterCount checks if the Audnex chapter count for the given ASIN
// matches fileCount. Returns the ASIN unchanged when Audnex is unavailable or
// returns no chapters (fail-open). Returns "" when the chapter count doesn't satisfy
// the configured criteria.
//
// When data.AllowMissingTracks is true the strict count equality is replaced by a
// two-tier runtime check using the durations of the "extra" Audnex chapters
// (those beyond fileCount) that we don't have locally:
//
//   - fileCount > len(chapters): always reject (more local files than expected).
//   - fileCount < len(chapters): run the two-tier check using actual track runtimes
//     when available (tracks[i].RuntimeMS > 0), otherwise fall back to chapter durations.
//     Tier 1 — per-track: |tracks[i].RuntimeMS - chapters[i].LengthMs| ≤ PerTrackToleranceSeconds
//     for every i in 0..fileCount-1. If all pass, accept.
//     Tier 2 — total (ExceedToleranceIfTotalMatch=true): per-track exceeded but
//     |localTotalMs - audnexSubsetMs| ≤ fileCount×PerTrackToleranceSeconds
//     (overridden by MaxTotalDifferenceSeconds). Accept if within tolerance.
//     If ExceedToleranceIfTotalMatch=false, reject as soon as any per-track check fails.
func verifyASINChapterCount(
	ctx context.Context,
	asin, region string,
	tracks []parser_v2.TrackInfo,
	data *config.MediaDataConfig,
) string {
	fileCount := len(tracks)
	if asin == "" || fileCount <= 0 {
		return asin
	}

	audnexProvider := providers.GetAudnex()
	if audnexProvider == nil {
		return asin
	}

	chapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, region)
	if err != nil || len(chapters) == 0 {
		return asin // fail-open: Audnex unavailable or ASIN not in DB
	}

	if len(chapters) == fileCount {
		return asin // exact match
	}

	// Split-files case: more local files than Audnex chapters (e.g. 458 files / 39 chapters).
	// The book is almost certainly correct — the download is just split more finely than
	// chapter boundaries. Keep the ASIN and let runtime verification decide.
	if fileCount > len(chapters) {
		logger.Logtype("debug", 0).
			Str("asin", asin).
			Int("audnexChapters", len(chapters)).
			Int("fileCount", fileCount).
			Msg("More local files than Audnex chapters (split files) — keeping ASIN")

		return asin
	}

	allowMissing := data != nil && data.AllowMissingTracks
	if allowMissing && fileCount < len(chapters) {
		perTrackMs := int64(3000)
		if data.PerTrackToleranceSeconds > 0 {
			perTrackMs = int64(data.PerTrackToleranceSeconds) * 1000
		}

		// Determine whether we have real local runtimes to work with.
		var localTotalMs int64

		hasRuntimes := false
		for i := range tracks {
			localTotalMs += tracks[i].RuntimeMS
			if tracks[i].RuntimeMS > 0 {
				hasRuntimes = true
			}
		}

		// Tier 1: per-track comparison.
		// With real runtimes: compare each local track against the corresponding Audnex chapter.
		// Without runtimes: compare each missing (extra) chapter duration against perTrackMs.
		perTrackOK := true
		if hasRuntimes {
			for i := range fileCount {
				diff := tracks[i].RuntimeMS - chapters[i].LengthMs
				if diff < 0 {
					diff = -diff
				}

				if diff > perTrackMs {
					perTrackOK = false
					break
				}
			}
		} else {
			for i := fileCount; i < len(chapters); i++ {
				if chapters[i].LengthMs > perTrackMs {
					perTrackOK = false
					break
				}
			}
		}

		if perTrackOK {
			return asin
		}

		// Tier 2 (ExceedToleranceIfTotalMatch): per-track exceeded, but accept if total
		// runtime difference is within the configured total tolerance.
		if data.ExceedToleranceIfTotalMatch {
			totalToleranceMs := int64(fileCount) * perTrackMs
			if data.MaxTotalDifferenceSeconds > 0 {
				totalToleranceMs = int64(data.MaxTotalDifferenceSeconds) * 1000
			}

			var audnexSubsetMs int64
			for i := range fileCount {
				audnexSubsetMs += chapters[i].LengthMs
			}

			var totalDiff int64
			if hasRuntimes {
				totalDiff = localTotalMs - audnexSubsetMs
			} else {
				// No local runtimes: use total duration of missing chapters as diff.
				for i := fileCount; i < len(chapters); i++ {
					totalDiff += chapters[i].LengthMs
				}
			}

			if totalDiff < 0 {
				totalDiff = -totalDiff
			}

			if totalDiff <= totalToleranceMs {
				return asin
			}
		}

		logger.Logtype("debug", 0).
			Str("asin", asin).
			Int("audnexChapters", len(chapters)).
			Int("fileCount", fileCount).
			Int64("localTotalMs", localTotalMs).
			Int64("perTrackMs", perTrackMs).
			Bool("hasRuntimes", hasRuntimes).
			Bool("exceedToleranceIfTotalMatch", data.ExceedToleranceIfTotalMatch).
			Msg("ASIN chapter count/runtime mismatch — discarding ASIN")

		return ""
	}

	logger.Logtype("debug", 0).
		Str("asin", asin).
		Int("audnexChapters", len(chapters)).
		Int("fileCount", fileCount).
		Bool("allowMissingTracks", allowMissing).
		Msg("ASIN chapter count mismatch — discarding ASIN")

	return ""
}

// matchAudiobookFolder matches an audiobook folder to the database.
// Uses ASIN for identification and Audible/Audnex for matching.
// If addToDatabase is true, adds files to database after matching.
// Returns the matched AlbumInfo and success status.
func matchAudiobookFolder(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
	asinOverride ...string,
) (*parser_v2.AlbumInfo, string, *MatchReport) {
	// Step 1b: Read all tracks with real tag data and runtimes upfront so every
	// downstream decision (ASIN verification, candidate selection, runtime matching)
	// works with actual durations rather than counts alone.
	tracks := parser_v2.CollectTracksFromFiles(files)

	tracks = parser_v2.EnrichTracksWithTags(tracks)

	// Step 2: Parse folder path, first filename, parent dir, and file tags.
	meta := parseFolderMetadata(folder, files, data)

	// ASIN resolution (requires ctx, tracks, cfgp — kept here, not in parseFolderMetadata).
	var asin string
	if len(asinOverride) > 0 && asinOverride[0] != "" {
		asin = asinOverride[0]
	} else {
		asin = verifyASINChapterCount(
			ctx,
			parser_v2.ParseASINFromPath(folder),
			cfgp.AudibleRegion,
			tracks,
			data,
		)
	}

	if asin == "" && meta.FilenameASIN != "" {
		asin = verifyASINChapterCount(ctx, meta.FilenameASIN, cfgp.AudibleRegion, tracks, data)
	}

	if asin == "" && meta.TagASIN != "" {
		asin = verifyASINChapterCount(ctx, meta.TagASIN, cfgp.AudibleRegion, tracks, data)
	}

	if meta.IsDiscFolder && meta.ParentAlbum != "" && asin == "" {
		asin = verifyASINChapterCount(
			ctx,
			parser_v2.ParseASINFromPath(meta.ParentDir),
			cfgp.AudibleRegion,
			tracks,
			data,
		)
	}

	var artist, albumTitle string
	if meta.IsDiscFolder {
		artist = coalesceStr(
			meta.TagAlbumArtist,
			meta.TagArtist,
			meta.GrandparentArtist,
			meta.ParentArtist,
		)
		albumTitle = coalesceStr(meta.TagAlbum, meta.ParentAlbum, meta.FilenameAlbum)
	} else {
		artist = coalesceStr(
			meta.TagAlbumArtist,
			meta.TagArtist,
			meta.FolderArtist,
			meta.FilenameArtist,
			meta.ParentArtist,
		)
		albumTitle = coalesceStr(
			meta.TagAlbum,
			meta.FolderAlbum,
			meta.FilenameAlbum,
			meta.ParentAlbum,
		)
	}

	logParsedMetadata(folder, meta, "asin", asin)

	// Step 3: Try to find audiobook match in database
	var dbMatch *database.AudiobookSearchResult

	fileCount := len(files)

	// Try by ASIN first (most reliable)
	if asin != "" {
		dbMatch, _ = database.FindAudiobookByASIN(asin)
	}

	// Try by tag ASIN if different from folder ASIN
	if dbMatch == nil && meta.TagASIN != "" && meta.TagASIN != asin {
		dbMatch, _ = database.FindAudiobookByASIN(meta.TagASIN)
	}

	// Build search pairs from parsed metadata.
	searchPairs := buildAudiobookSearchPairs(meta)

	// Collect ALL potential matches from all search pairs
	// Then select the best one based on chapter count (preferring exact matches)
	var allMatches []*database.AudiobookSearchResult

	seenIDs := make(map[uint]bool)

	for i := range searchPairs {
		matches, searchErr := database.FindAudiobooksByTitleAuthor(
			&searchPairs[i].title,
			&searchPairs[i].author,
			10,
		)
		if searchErr != nil || len(matches) == 0 {
			continue
		}

		// Add unique matches to our collection
		for _, m := range matches {
			if !seenIDs[m.ID] {
				seenIDs[m.ID] = true
				allMatches = append(allMatches, m)
			}
		}
	}

	// Select all candidates with matching chapter count for runtime verification
	bestCandidates, dbMatch := selectAudiobookCandidatesWithRefresh(
		ctx, allMatches, fileCount, data, albumTitle, artist, cfgp,
	)

	// If no chapter count match but we have series candidates, skip the track count
	// requirement when SkipSeriesTrackMatch is enabled and runtime is under 1 hour.
	seriesTrackSkipped := false
	if dbMatch == nil && len(allMatches) > 0 {
		if m, skipped := pickSeriesAudiobookFallback(allMatches, data, fileCount, folder); skipped {
			dbMatch = m
			seriesTrackSkipped = true
		}
	}

	if dbMatch == nil && albumTitle != "" && len(allMatches) == 0 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("author", artist).
			Str("title", albumTitle).
			Int("searchPairsTried", len(searchPairs)).
			Msg("No audiobook match found after trying all combinations")
	}

	// Build album info structure
	album := &parser_v2.AlbumInfo{
		Title:        albumTitle,
		Artist:       artist,
		Year:         meta.Year,
		Genre:        meta.TagGenre,
		ASIN:         asin,
		SourceFolder: folder,
		TrackCount:   len(files),
	}

	if dbMatch != nil {
		album.DatabaseID = dbMatch.ID
		album.ExpectedTracks = dbMatch.ChapterCount
	}

	// Step 4b: If ASIN still unknown, search Audible using all the same search pairs
	// already built above (including stripped episode/series prefix variants).
	// This lets addFound work for folders with no ASIN in path or tags.
	if album.ASIN == "" && data.AddFound && len(searchPairs) > 0 {
		if found := findASINByTitleAuthor(
			ctx,
			searchPairs,
			cfgp.AudibleRegion,
			album.TrackCount,
		); found != "" {
			album.ASIN = found
			logger.Logtype("debug", 0).
				Str("folder", folder).
				Str("asin", found).
				Msg("Discovered ASIN via Audible search pairs")
		}
	}

	// Step 5: If still no match and addFound is enabled, try to import
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	var addFoundAudiobookEntry *database.AudiobookSearchResult
	if album.DatabaseID == 0 && data.AddFound && album.ASIN != "" && listid != -1 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("asin", album.ASIN).
			Str("title", album.Title).
			Int("tracks", album.TrackCount).
			Msg("Audiobook not in database - importing via addFound")

		if audnexAddFoundPreFlight(ctx, album.ASIN, album.TrackCount, tracks, cfgp, data, folder) {
			dbID, expTracks, newCands, addFoundEntry := importAudiobookWithAddFound(
				ctx, album.ASIN, allMatches, bestCandidates, cfgp, listid, data, folder,
			)
			if dbID > 0 {
				album.DatabaseID = dbID
				album.ExpectedTracks = expTracks
				bestCandidates = newCands
				addFoundAudiobookEntry = addFoundEntry
			}
		}
	}

	// If still no match, can't proceed
	if album.DatabaseID == 0 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("title", album.Title).
			Str("artist", album.Artist).
			Str("asin", album.ASIN).
			Msg("Audiobook not found in database - skipping")

		return nil, "no_match", nil
	}

	// Step 6: Verify track count matches expected chapter count
	if !seriesTrackSkipped && album.ExpectedTracks > 0 && album.TrackCount != album.ExpectedTracks {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Int("localTracks", album.TrackCount).
			Int("expectedChapters", album.ExpectedTracks).
			Msg("Track count mismatch - skipping")

		return nil, "wrong_track_count", nil
	}

	// Step 7: Assign the tracks (already read and enriched at the top of this function).
	album.Tracks = tracks
	album.TotalRuntime = parser_v2.CalculateTotalRuntime(tracks)

	// Step 8: Sort all candidates by album distance, then match tracks using beets-style
	// distance scoring. The first candidate where all tracks match (or AllowMissingTracks
	// permits partial matches) wins.

	// When series track skip was used, ensure the selected candidate is included.
	if seriesTrackSkipped && dbMatch != nil {
		found := false
		for _, c := range bestCandidates {
			if c.ID == dbMatch.ID {
				found = true
				break
			}
		}

		if !found {
			bestCandidates = append(bestCandidates, dbMatch)
		}
	}

	// Ensure the ASIN-matched candidate (if any) is also included.
	if dbMatch != nil {
		found := false
		for _, c := range bestCandidates {
			if c.ID == dbMatch.ID {
				found = true
				break
			}
		}

		if !found {
			bestCandidates = append([]*database.AudiobookSearchResult{dbMatch}, bestCandidates...)
		}
	}

	// Sort by album distance, best first (stable so equal-distance order is preserved).
	sort.SliceStable(bestCandidates, func(i, j int) bool {
		di := audiobookMatchDistance(bestCandidates[i], albumTitle, artist, fileCount)
		dj := audiobookMatchDistance(bestCandidates[j], albumTitle, artist, fileCount)
		return di < dj
	})

	isVA := IsVariousArtists(artist)

	validABMatches := runAudiobookCandidatePass(
		ctx,
		bestCandidates,
		album.Tracks,
		isVA,
		albumTitle,
		artist,
		fileCount,
		data,
		cfgp,
	)

	sortSuccess := false
	if dbID, expTracks, matched, ok := applyAudiobookRecommendation(validABMatches); ok {
		album.DatabaseID = dbID
		album.ExpectedTracks = expTracks
		album.Tracks = matched
		sortSuccess = true
	}

	if !sortSuccess && len(bestCandidates) > 0 {
		return nil, "wrong_runtime", buildAudiobookFailureReport(
			ctx, bestCandidates, validABMatches, album.Tracks, albumTitle, artist,
			int64(album.TotalRuntime/time.Millisecond), isVA, data, cfgp,
		)
	}

	// Deferred author discovery from addFound - only runs after track/runtime verification succeeds
	if addFoundAudiobookEntry != nil && artist != "" && data.AddFound {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("author", artist).
			Msg("DEBUG: Discovering other audiobooks by author")

		audiobooksAdded := DiscoverAndAddAuthorAudiobooks(ctx, artist, cfgp, listid, 50)
		if audiobooksAdded > 0 {
			logger.Logtype("info", 1).
				Str("author", artist).
				Int("audiobooks_added", audiobooksAdded).
				Msg("Added other audiobooks by author")
		}
	}

	// Ensure ExpectedTracks is always set — use the actual matched track count as fallback.
	if album.ExpectedTracks == 0 {
		album.ExpectedTracks = len(album.Tracks)
	}

	// Step 9: Add files to database if requested
	if addToDatabase {
		if !addAudiobookFilesToDatabase(ctx, folder, album, cfgp, listid) {
			return album, "no_match", nil
		}

		return album, "", nil
	}

	return album, "", nil
}

// matchAudiobookFolderForced matches an audiobook folder against a single,
// caller-supplied ASIN, bypassing all text-search and scoring logic.
//
// Rules for the forced path:
//   - Look up (or import) the audiobook by ASIN only — no addPair / text search.
//   - Skip verifyASINChapterCount, selectBestAudiobookMatches, and recommendation() entirely.
//   - Skip the track-count equality check (AllowMissingTracks is always true).
//   - For DB tracks that matchTracksByDistance left unmatched, assign the closest
//     remaining local file by track distance ("next-best" fallback).
func matchAudiobookFolderForced(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
	forcedASIN *string,
) (*parser_v2.AlbumInfo, string, *MatchReport) {
	// Step 1: find audiobook in DB, importing it if necessary.
	dbMatch, _ := database.FindAudiobookByASIN(*forcedASIN)
	if dbMatch == nil {
		listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
		if listid != -1 {
			if dbID, err := JobImportAudiobooks(
				ctx,
				*forcedASIN,
				cfgp,
				listid,
				true,
			); err == nil &&
				dbID != 0 {
				dbMatch, _ = database.FindAudiobookByASIN(*forcedASIN)
			}
		}

		if dbMatch == nil {
			return nil, "no_match", nil
		}
	}

	// Step 2: parse folder metadata and build album struct.
	tracks := parser_v2.CollectTracksFromFiles(files)

	tracks = parser_v2.EnrichTracksWithTags(tracks)

	tagData := parser_v2.ReadTagsForFirstFile(files)

	var tagArtist, tagAlbumArtist, tagAlbum, tagGenre string
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbumArtist = tagData.AlbumArtist
		tagAlbum = tagData.Album
		tagGenre = tagData.Genre
	}

	folderArtist, folderAlbum, year := parser_v2.ParseAudioFolder(folder)
	firstTrack := parser_v2.ParseAudioFilename(files[0])
	artist := coalesceStr(tagAlbumArtist, tagArtist, folderArtist, firstTrack.Artist)
	albumTitle := coalesceStr(tagAlbum, folderAlbum, firstTrack.Album, dbMatch.Title)

	album := &parser_v2.AlbumInfo{
		Title:          albumTitle,
		Artist:         artist,
		Year:           year,
		Genre:          tagGenre,
		ASIN:           *forcedASIN,
		SourceFolder:   folder,
		TrackCount:     len(files),
		DatabaseID:     dbMatch.ID,
		ExpectedTracks: dbMatch.ChapterCount,
	}

	album.Tracks = tracks
	album.TotalRuntime = parser_v2.CalculateTotalRuntime(tracks)

	// Step 3: get DB tracks and run the standard LAP matcher.
	isVA := IsVariousArtists(artist)

	dbTracks := resolveTracksForMatching(
		ctx,
		dbMatch.ID,
		"",
		*forcedASIN,
		cfgp.AudibleRegion,
		albumTitle,
		artist,
	)
	if len(dbTracks) == 0 {
		return nil, "no_tracks", nil
	}

	forcedData := *data

	forcedData.AllowMissingTracks = true

	result, matched, used := matchTracksByDistance(album.Tracks, dbTracks, isVA, true, &forcedData)

	// Step 4: build the matched-tracks slice with next-best fallback for unmatched DB tracks.
	matchedTracks := buildMatchedTracksWithFallback(
		album.Tracks, dbTracks, result, matched, used, isVA, true, &forcedData,
	)

	album.Tracks = matchedTracks
	if album.ExpectedTracks == 0 {
		album.ExpectedTracks = len(matchedTracks)
	}

	// Step 5: persist to database if requested.
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
	if addToDatabase {
		if !addAudiobookFilesToDatabase(ctx, folder, album, cfgp, listid) {
			return album, "no_match", nil
		}

		return album, "", nil
	}

	return album, "", nil
}

// matchMusicFolderForced matches a music folder against a single, caller-supplied
// MusicBrainz release ID, bypassing all text-search and scoring logic.
//
// Rules for the forced path:
//   - Look up (or import) the album by MBID only — no addPair / text search.
//   - Skip selectBestMatches and the recommendation() threshold entirely.
//   - AllowMissingTracks is always treated as true.
//   - For DB tracks that matchTracksByDistance left unmatched, assign the closest
//     remaining local file by track distance so the rename plan is as complete as
//     possible ("next-best" fallback).
func matchMusicFolderForced(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
	forcedMBID *string,
) (*parser_v2.AlbumInfo, string, *MatchReport) {
	// Step 1: find album in DB, importing it if necessary.
	dbMatch, _ := database.FindAlbumByMusicBrainzID(forcedMBID)
	if dbMatch == nil {
		listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
		if listid != -1 {
			if dbID, err := JobImportAlbums(
				ctx,
				*forcedMBID,
				cfgp,
				listid,
				true,
			); err == nil &&
				dbID != 0 {
				dbMatch, _ = database.FindAlbumByMusicBrainzID(forcedMBID)
			}
		}

		if dbMatch == nil {
			return nil, "no_match", nil
		}
	}

	// Step 2: parse folder metadata and build album struct.
	tagData := parser_v2.ReadTagsForFirstFile(files)

	var tagArtist, tagAlbumArtist, tagAlbum, tagGenre string
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbumArtist = tagData.AlbumArtist
		tagAlbum = tagData.Album
		tagGenre = tagData.Genre
	}

	folderArtist, folderAlbum, year := parser_v2.ParseAudioFolder(folder)
	artist := coalesceStr(tagAlbumArtist, tagArtist, folderArtist)
	albumTitle := coalesceStr(tagAlbum, folderAlbum, dbMatch.Title)

	album := &parser_v2.AlbumInfo{
		Title:          albumTitle,
		Artist:         artist,
		Year:           year,
		Genre:          tagGenre,
		SourceFolder:   folder,
		TrackCount:     len(files),
		DatabaseID:     dbMatch.ID,
		ExpectedTracks: dbMatch.TotalTracks,
	}

	tracks := parser_v2.CollectTracksFromFiles(files)

	tracks = parser_v2.EnrichTracksWithTags(tracks)
	album.Tracks = tracks
	album.TotalRuntime = parser_v2.CalculateTotalRuntime(tracks)

	// Step 3: get DB tracks and run the standard LAP matcher.
	isVA := DetectVA(artist, tracks)

	dbTracks := resolveTracksForMatching(ctx, dbMatch.ID, *forcedMBID, "", "", albumTitle, artist)
	if len(dbTracks) == 0 {
		return nil, "no_tracks", nil
	}

	// Use a copy of data with AllowMissingTracks forced true so the matcher
	// doesn't reject unmatched pairs before we can apply the fallback.
	forcedData := *data

	forcedData.AllowMissingTracks = true

	result, matched, used := matchTracksByDistance(album.Tracks, dbTracks, isVA, false, &forcedData)

	// Step 4: build the matched-tracks slice.
	// For DB tracks left unmatched by the LAP, find the closest unused local file.
	matchedTracks := buildMatchedTracksWithFallback(
		album.Tracks, dbTracks, result, matched, used, isVA, false, &forcedData,
	)

	album.Tracks = matchedTracks
	if album.ExpectedTracks == 0 {
		album.ExpectedTracks = len(matchedTracks)
	}

	// Step 5: persist to database if requested.
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
	if addToDatabase {
		if !addAlbumFilesToDatabase(ctx, folder, album, cfgp, listid) {
			return album, "no_match", nil
		}

		return album, "", nil
	}

	return album, "", nil
}

// matchMusicFolder matches a music folder to the database.
// Uses MusicBrainzID/UPC for identification and MusicBrainz/Discogs for matching.
// Does NOT perform language validation (unlike audiobooks).
// If addToDatabase is true, adds files to database after matching.
// Returns the matched AlbumInfo and success status.
func matchMusicFolder(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
	mbidOverride ...string,
) (*parser_v2.AlbumInfo, string, *MatchReport) {
	// Step 2: Parse all metadata from folder path, filename, parent dir, and tags.
	meta := parseFolderMetadata(folder, files, data)

	musicBrainzID := meta.TagMusicBrainzID
	if musicBrainzID == "" && len(mbidOverride) > 0 && mbidOverride[0] != "" {
		musicBrainzID = mbidOverride[0]
	}

	// Tags take priority because they are authoritative; folder-name parsing is a
	// fallback for files with no tags.
	var artist, albumTitle string
	if meta.IsDiscFolder {
		artist = coalesceStr(
			meta.TagAlbumArtist,
			meta.TagArtist,
			meta.GrandparentArtist,
			meta.ParentArtist,
		)
		albumTitle = coalesceStr(meta.TagAlbum, meta.ParentAlbum, meta.FilenameAlbum)
	} else {
		artist = coalesceStr(
			meta.TagAlbumArtist,
			meta.TagArtist,
			meta.FolderArtist,
			meta.FilenameArtist,
			meta.ParentArtist,
		)
		albumTitle = coalesceStr(
			meta.TagAlbum,
			meta.FolderAlbum,
			meta.FilenameAlbum,
			meta.ParentAlbum,
		)
	}

	logParsedMetadata(folder, meta, "musicBrainzID", musicBrainzID)

	// Step 3: Try to find album match in database.
	var dbMatch *database.AlbumSearchResult

	fileCount := len(files)
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	var addFoundMusicEntry *database.AlbumSearchResult
	// dbMatchedByMBID is true only when FindAlbumByMusicBrainzID found the exact release.
	var dbMatchedByMBID bool

	if musicBrainzID != "" {
		dbMatch, _ = database.FindAlbumByMusicBrainzID(&musicBrainzID)
		if dbMatch != nil {
			dbMatchedByMBID = true
		}
	}

	// Build and search all unique artist+album combinations.
	searchPairs := buildMusicSearchPairs(meta)

	// Collect ALL potential matches from all search pairs
	// Then select the best one based on track count (preferring exact matches)
	var allMatches []*database.AlbumSearchResult

	seenIDs := make(map[uint]bool)

	for i := range searchPairs {
		matches, searchErr := database.FindAlbumsByArtistTitle(
			&searchPairs[i].artist,
			&searchPairs[i].album,
			10,
		)
		if searchErr != nil || len(matches) == 0 {
			continue
		}

		// Add unique matches to our collection
		for _, m := range matches {
			if !seenIDs[m.ID] {
				seenIDs[m.ID] = true
				allMatches = append(allMatches, m)
			}
		}
	}

	// Select all candidates with matching track count for runtime verification
	var bestCandidates []*database.AlbumSearchResult
	if len(allMatches) > 0 {
		bestCandidates = selectBestAlbumMatches(
			allMatches,
			fileCount,
			artist,
			albumTitle,
			musicBrainzID,
			"",
			"",
			meta.Year,
			data,
		)
		if len(bestCandidates) > 0 {
			// Use first candidate initially, will verify runtime later
			dbMatch = bestCandidates[0]
		} else {
			// No candidates passed the distance threshold.  Sort allMatches by
			// album distance so the best-matching title is used as the fallback
			// for searchAndImportAlternativeRelease (instead of an arbitrary first result).
			sort.SliceStable(allMatches, func(i, j int) bool {
				di := albumMatchDistance(
					allMatches[i],
					artist,
					albumTitle,
					musicBrainzID,
					"",
					"",
					meta.Year,
					fileCount,
					data,
				)
				dj := albumMatchDistance(
					allMatches[j],
					artist,
					albumTitle,
					musicBrainzID,
					"",
					"",
					meta.Year,
					fileCount,
					data,
				)

				return di < dj
			})

			dbMatch = allMatches[0]
			bestCandidates = allMatches
		}
	}

	if dbMatch == nil && albumTitle != "" {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("artist", artist).
			Str("album", albumTitle).
			Int("searchPairsTried", len(searchPairs)).
			Int("totalCandidates", len(allMatches)).
			Msg("No album match found after trying all combinations")
	}

	// Build album info structure
	album := &parser_v2.AlbumInfo{
		Title:        albumTitle,
		Artist:       artist,
		Year:         meta.Year,
		Genre:        meta.TagGenre,
		SourceFolder: folder,
		TrackCount:   len(files),
	}

	if dbMatch != nil {
		album.DatabaseID = dbMatch.ID
		album.ExpectedTracks = dbMatch.TotalTracks
	}

	// Step 4: Read tags for all files and build track list.
	// This must happen before the addFound pre-flight so that track-length
	// verification can compare local runtimes against MusicBrainz data.
	{
		tracks := parser_v2.CollectTracksFromFiles(files)

		tracks = parser_v2.EnrichTracksWithTags(tracks)
		album.Tracks = tracks
		album.TotalRuntime = parser_v2.CalculateTotalRuntime(tracks)
	}

	// Compute isVA and localTotalMs here — needed by fallback calls below as well
	// as by the candidate scoring in Step 7.
	isVA := DetectVA(artist, album.Tracks)
	localTotalMs := int64(album.TotalRuntime / time.Millisecond)

	// Step 5: If MBID was not matched directly in the DB and AddFound is enabled,
	// import the correct release from MusicBrainz now — before any fuzzy DB match can
	// set album.DatabaseID and prevent addfound from running.
	// Pre-flight verifies both track count AND track lengths against local files so
	// we never import a release with a different edition or track list.
	if !dbMatchedByMBID && data.AddFound && musicBrainzID != "" && listid != -1 {
		dbID, expTracks, newCands, addFoundEntry := verifyAndImportMusicBrainzAddFound(
			ctx, &musicBrainzID, album.Tracks, bestCandidates,
			artist, fileCount, cfgp, listid, data, folder,
		)
		if dbID > 0 {
			album.DatabaseID = dbID
			album.ExpectedTracks = expTracks
			bestCandidates = newCands
			addFoundMusicEntry = addFoundEntry
		}
	}

	// If still no match, try AddFound text search before giving up.
	fallbacksRan := false
	if album.DatabaseID == 0 {
		if data.AddFound && listid != -1 && artist != "" && albumTitle != "" {
			dbID, expTracks, newCands, addFoundEntry := importMusicAddFoundTextSearch(
				ctx, albumTitle, meta.TagAlbum, &artist, meta.TagArtist, bestCandidates,
				int64(album.TotalRuntime/time.Millisecond), &fileCount,
				cfgp, listid, data, folder,
			)
			if dbID > 0 {
				album.DatabaseID = dbID
				album.ExpectedTracks = expTracks
				bestCandidates = newCands
				addFoundMusicEntry = addFoundEntry
			}
		}

		if album.DatabaseID == 0 {
			fallbackCands := tryMusicFallbacks(
				ctx,
				&albumTitle,
				&artist,
				localTotalMs,
				&fileCount,
				data,
				folder,
			)

			fallbacksRan = true

			if len(fallbackCands) == 0 {
				logger.Logtype("debug", 0).
					Str("folder", folder).
					Str("title", album.Title).
					Str("artist", album.Artist).
					Str("musicBrainzID", musicBrainzID).
					Msg("Album not found in database - fallback failed - skipping")

				return nil, "no_match", nil
			}

			bestCandidates = append(bestCandidates, fallbackCands...)
		}
	}

	// Step 7: Score, rank, and match candidates using beets-style full distance.
	//
	// Pass 1: pre-compute album-only distances (O(1) each) and sort candidates so we
	// visit the most-promising ones first and can stop as soon as we exceed the threshold.
	//
	// Pass 2: for each candidate within threshold, fetch its track listing, run the
	// Hungarian assignment, verify the match quality, then compute the full distance
	// (album metadata + per-track distances).  Collect every valid match so we can
	// apply the beets recommendation logic before committing to the best one.
	//
	// isVA and localTotalMs already computed above (after Step 4).

	// Build sorted pre-scored list for the report (used even when runMusicCandidatePasses
	// returns matches, so the failure-report path can enumerate the top candidates).
	preScored := make([]preScoredC, 0, len(bestCandidates))
	for _, c := range bestCandidates {
		d := albumMatchDistance(
			c,
			artist,
			albumTitle,
			musicBrainzID,
			"",
			"",
			meta.Year,
			fileCount,
			data,
		)

		preScored = append(preScored, preScoredC{c, d})
	}

	sort.SliceStable(
		preScored,
		func(i, j int) bool { return preScored[i].dist < preScored[j].dist },
	)

	validMatches := runMusicCandidatePasses(
		ctx, preScored, album.Tracks, localTotalMs, isVA,
		albumTitle, artist, musicBrainzID, meta.Year, fileCount, data,
	)

	sortSuccess := false
	if dbID, expTracks, matched, ok := applyMusicRecommendation(
		validMatches,
		localTotalMs,
		fileCount,
		data,
	); ok {
		album.DatabaseID = dbID
		album.ExpectedTracks = expTracks
		album.Tracks = matched
		sortSuccess = true
	}

	if !sortSuccess && len(bestCandidates) > 0 {
		// No candidates passed distance matching. Try MusicBrainz for an alternative release.
		if altID, altTracks, altMatched, altOK := tryMusicAlternativeRelease(
			ctx, album.Tracks, bestCandidates, &albumTitle, &artist,
			localTotalMs, &fileCount, isVA, cfgp, listid, data, folder,
		); altOK {
			album.DatabaseID = altID
			album.ExpectedTracks = altTracks
			album.Tracks = altMatched
			sortSuccess = true
		}

		if !sortSuccess && !fallbacksRan {
			fallbackCands := tryMusicFallbacks(
				ctx,
				&albumTitle,
				&artist,
				localTotalMs,
				&fileCount,
				data,
				folder,
			)
			if len(fallbackCands) > 0 {
				// Update query values from API-provided data before scoring so that
				// albumMatchDistance compares correct metadata, not stale folder-parsed values.
				fbPreScored := make([]preScoredC, 0, len(fallbackCands))
				for _, c := range fallbackCands {
					d := albumMatchDistance(
						c,
						artist,
						albumTitle,
						musicBrainzID,
						"",
						"",
						meta.Year,
						fileCount,
						data,
					)

					fbPreScored = append(fbPreScored, preScoredC{c, d})
				}

				sort.SliceStable(
					fbPreScored,
					func(i, j int) bool { return fbPreScored[i].dist < fbPreScored[j].dist },
				)

				fbMatches := runMusicCandidatePasses(
					ctx,
					fbPreScored,
					album.Tracks,
					localTotalMs,
					isVA,
					albumTitle,
					artist,
					musicBrainzID,
					meta.Year,
					fileCount,
					data,
				)
				if dbID2, expTracks2, matched2, ok2 := applyMusicRecommendation(
					fbMatches,
					localTotalMs,
					fileCount,
					data,
				); ok2 {
					album.DatabaseID = dbID2
					album.ExpectedTracks = expTracks2
					album.Tracks = matched2
					sortSuccess = true
				}
			}

			if !sortSuccess {
				return nil, "wrong_runtime", buildMusicFailureReport(
					ctx, preScored, bestCandidates, album.Tracks,
					int64(album.TotalRuntime/time.Millisecond), isVA, data, folder,
				)
			}
		}
	}

	// Deferred artist/series discovery from addFound - only runs after track/runtime verification succeeds
	if addFoundMusicEntry != nil && data.AddFound {
		discoverAddFoundArtistAlbums(
			ctx, album.Title, &artist, &musicBrainzID, cfgp, listid, data, folder,
		)
	}

	// Ensure ExpectedTracks is always set — use the actual matched track count as fallback.
	if album.ExpectedTracks == 0 {
		album.ExpectedTracks = len(album.Tracks)
	}

	// Step 8: Add files to database if requested (no language validation for music)
	if addToDatabase {
		if !addAlbumFilesToDatabase(ctx, folder, album, cfgp, listid) {
			return album, "no_match", nil
		}

		return album, "", nil
	}

	return album, "", nil
}

// searchAndImportAlternativeRelease searches MusicBrainz for an alternative release
// of the same album with matching track count and runtime.
// This is called when we find albums in the database but none match the local files' runtime.
// Returns the database ID of the imported release and true if successful.
func searchAndImportAlternativeRelease(
	ctx context.Context,
	artist *string,
	albumTitle *string,
	fileCount *int,
	localTotalRuntimeMs int64,
	cfgp *config.MediaTypeConfig,
	listid int,
	existingCandidates []*database.AlbumSearchResult,
	data *config.MediaDataConfig,
) (uint, bool) {
	if artist == nil || albumTitle == nil || fileCount == nil {
		return 0, false
	}

	// Map common abbreviations for Various Artists
	searchArtist := *artist
	if strings.EqualFold(*artist, "VA") || strings.EqualFold(*artist, "V.A.") {
		searchArtist = VariousArtistsName
	}

	// Calculate runtime tolerance (configurable, default 3 seconds per track).
	// Shared by all provider paths below.
	perTrackMs := int64(3000)
	if data != nil && data.PerTrackToleranceSeconds > 0 {
		perTrackMs = int64(data.PerTrackToleranceSeconds) * 1000
	}

	toleranceMs := int64(*fileCount) * perTrackMs
	if data != nil && data.MaxTotalDifferenceSeconds > 0 {
		toleranceMs = int64(data.MaxTotalDifferenceSeconds) * 1000
	}

	// ── MusicBrainz path ─────────────────────────────────────────────────────────
	mbProvider := getMusicBrainzProvider()
	if mbProvider != nil {
		// Build search query using unquoted release title tokens.
		// Quoted Lucene phrases require exact title matches; unquoted tokens allow fuzzy keyword
		// matching so translated/variant titles (e.g. "Al Capolinea" → "At Capolinea") are found.
		// For non-VA artists we also add a server-side artist: clause so common album names like
		// "Live" or "Greatest Hits" don't exhaust the 50-result window with wrong artists.
		// Artist filtering is also applied client-side as a final guard.
		// NOTE: MB tracks: field is per-medium (per disc), not the release total — unreliable
		// for multi-disc releases. Track count filtering is done client-side instead.
		query := string(BuildArtistAlbumSearch(*albumTitle, searchArtist))

		releases, err := retryOnRateLimit(
			ctx,
			func() ([]apiexternal_v2.ReleaseSearchResult, error) {
				r, _, e := mbProvider.SearchReleases(ctx, query, 50, 0)
				return r, e
			},
		)
		if (err != nil || len(releases) == 0) && len(existingCandidates) == 0 {
			// No DB candidates and no MB results — retry with a quoted phrase+slop
			// query (release:"Title"~2) to handle minor title variations such as
			// stripped diacritics or single-character typos.
			buf := logger.PlAddBuffer.Get()
			buf.WriteString(`release:"`)
			buf.WriteString(LuceneEscape(*albumTitle))
			buf.WriteString(`"~2`)

			if searchArtist != "" && !IsVariousArtists(searchArtist) {
				buf.WriteString(` AND artist:"`)
				buf.WriteString(LuceneEscape(searchArtist))
				buf.WriteString(`"~2`)
			}

			fuzzyQuery := buf.String()
			logger.PlAddBuffer.Put(buf)
			logger.Logtype("debug", 0).
				Str("original_query", query).
				Str("fuzzy_query", fuzzyQuery).
				Msg("MB text search returned no results — retrying with fuzzy release query")

			releases, err = retryOnRateLimit(
				ctx,
				func() ([]apiexternal_v2.ReleaseSearchResult, error) {
					r, _, e := mbProvider.SearchReleases(ctx, fuzzyQuery, 50, 0)
					return r, e
				},
			)
		}

		if err == nil && len(releases) > 0 {
			// Check if any existing DB candidate has files attached.
			existingHasFiles := false
			for _, candidate := range existingCandidates {
				candidateID := candidate.ID

				fc := database.Getdatarow[int](
					false,
					"SELECT COUNT(*) FROM album_files WHERE dbalbum_id = ?",
					&candidateID,
				)
				if fc > 0 {
					existingHasFiles = true
					break
				}
			}

			for i := range releases {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return 0, false
				}

				if releases[i].TrackCount != *fileCount {
					continue
				}

				if !strings.EqualFold(searchArtist, VariousArtistsName) {
					artistMatch := false
					for j := range releases[i].Artists {
						if strings.EqualFold(releases[i].Artists[j].Name, searchArtist) {
							artistMatch = true
							break
						}
					}

					if !artistMatch {
						continue
					}
				}

				existingID := database.Getdatarow[uint](
					false,
					"SELECT id FROM dbalbums WHERE musicbrainz_release_id = ?",
					&releases[i].MusicBrainzID,
				)
				if existingID > 0 {
					continue
				}

				if data != nil && !data.AllowAllFormatsWhenStructuring &&
					len(releases[i].MediaFormats) > 0 {
					if !releaseMatchesMBFormats(&releases[i], data.MBMediaFormats) {
						logger.Logtype("debug", 2).
							Str("album", releases[i].Title).
							Strs("disc_formats", releases[i].MediaFormats).
							Msg("Alternative release format not allowed, skipping")

						continue
					}
				}

				releaseDetails, detailsErr := retryOnRateLimit(
					ctx,
					func() (*apiexternal_v2.ReleaseDetails, error) {
						return mbProvider.GetReleaseByID(ctx, releases[i].MusicBrainzID)
					},
				)
				if detailsErr != nil || releaseDetails == nil {
					continue
				}

				var releaseTotalRuntimeMs int64
				for j := range releaseDetails.Tracks {
					releaseTotalRuntimeMs += releaseDetails.Tracks[j].Duration.Milliseconds()
				}

				if releaseTotalRuntimeMs == 0 {
					logger.Logtype("debug", 0).
						Str("releaseID", releases[i].MusicBrainzID).
						Msg("MB release has no track durations — accepting on track count match")
				}

				runtimeDiff := localTotalRuntimeMs - releaseTotalRuntimeMs
				if runtimeDiff < 0 {
					runtimeDiff = -runtimeDiff
				}

				if releaseTotalRuntimeMs == 0 || runtimeDiff <= toleranceMs {
					if !existingHasFiles && len(existingCandidates) > 0 {
						dbID, replaceOk := replaceExistingRelease(
							ctx,
							existingCandidates[0].ID,
							releases[i].MusicBrainzID,
							cfgp,
							listid,
						)
						if replaceOk {
							logger.Logtype("info", 1).
								Uint("databaseID", dbID).
								Str("releaseID", releases[i].MusicBrainzID).
								Msg("Replaced existing release with alternative")

							return dbID, true
						}
					}

					dbID, importErr := JobImportAlbums(
						ctx,
						releases[i].MusicBrainzID,
						cfgp,
						listid,
						true,
					)
					if importErr == nil && dbID != 0 {
						logger.Logtype("debug", 0).
							Uint("databaseID", dbID).
							Str("releaseID", releases[i].MusicBrainzID).
							Msg("Successfully imported alternative release as new entry")

						return dbID, true
					}

					logger.Logtype("debug", 0).
						Str("releaseID", releases[i].MusicBrainzID).
						Err(importErr).
						Msg("Failed to import alternative release")
				}
			}
		}
	}

	// ── Last.fm fallback ──────────────────────────────────────────────────────────
	if lfm := providers.GetLastFM(); lfm != nil {
		if release, err := lfm.GetAlbumInfo(ctx, *artist, *albumTitle, ""); err == nil &&
			release != nil && len(release.Tracks) == *fileCount {
			var lfmTotalMs int64
			for i := range release.Tracks {
				lfmTotalMs += release.Tracks[i].Duration.Milliseconds()
			}

			runtimeOK := lfmTotalMs == 0
			if !runtimeOK {
				diff := lfmTotalMs - localTotalRuntimeMs
				if diff < 0 {
					diff = -diff
				}

				runtimeOK = diff <= toleranceMs
			}

			if runtimeOK {
				lfmMBID := release.MusicBrainzID
				slug := logger.StringToSlugCachedP(albumTitle)

				var existingID uint
				if lfmMBID != "" {
					// Last.fm returns a release-group MBID, not a specific release ID.
					database.Scanrowsdyn(
						false,
						"SELECT id FROM dbalbums WHERE musicbrainz_release_group_id = ? LIMIT 1",
						&existingID,
						&lfmMBID,
					)
				} else {
					database.Scanrowsdyn(false,
						`SELECT a.id FROM dbalbums a
						  JOIN dbalbum_artists aa ON aa.dbalbum_id = a.id
						  JOIN dbartists ar ON ar.id = aa.dbartist_id
						 WHERE (a.title = ? COLLATE NOCASE OR a.slug = ?)
						   AND ar.name = ? COLLATE NOCASE
						   AND (a.musicbrainz_release_id = '' OR a.musicbrainz_release_id IS NULL)
						 LIMIT 1`,
						&existingID, albumTitle, &slug, artist,
					)
				}

				if existingID == 0 && data != nil &&
					(data.AddFound || data.AllowAlternativeReleases) {
					lfmAltTitle := releaseTitleName(release.Title, albumTitle)
					lfmAltSlug := logger.StringToSlugCached(lfmAltTitle)

					result, insertErr := database.ExecNid(
						`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
						  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
						  label, country, total_tracks, total_runtime_ms, genres, cover_url)
						 VALUES (?, ?, '', '', '', '', ?, 0, '', '', '', '', ?, ?, '', '')`,
						&lfmAltTitle,
						&lfmMBID,
						&lfmAltSlug,
						fileCount,
						&lfmTotalMs,
					)
					if insertErr == nil {
						existingID = logger.Int64ToUint(result)

						lfmAltArtistName, lfmAltArtistMBID := releaseArtistName(
							release.Artists,
							artist,
						)

						lfmAltArtistNamePtr := &lfmAltArtistName
						if artistID := addOrGetArtist(
							lfmAltArtistNamePtr,
							&lfmAltArtistMBID,
						); artistID > 0 {
							pos := 0

							_, _ = database.ExecNid(
								`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position) VALUES (?, ?, ?)`,
								&existingID,
								&artistID,
								&pos,
							)
						}

						for i := range release.Tracks {
							t := &release.Tracks[i]
							rms := t.Duration.Milliseconds()
							// Use sequential index (i+1) as track_number rather than
							// t.Position (Last.fm rank): Last.fm sometimes numbers
							// multi-disc albums per-disc (1-5, 1-5) instead of globally
							// (1-10), which would create duplicate (disc=1, track=N) rows.
							tn := i + 1

							_, _ = database.ExecNid(
								`INSERT INTO dbtracks (dbalbum_id, title, track_number, disc_number, runtime_ms, acoustid)
								 VALUES (?, ?, ?, 1, ?, '')`,
								&existingID,
								&t.Title,
								&tn,
								&rms,
							)
						}

						logger.Logtype("info", 1).
							Str("artist", *artist).
							Str("album", *albumTitle).
							Uint("dbalbumID", existingID).
							Msg("Last.fm alternative: inserted synthetic album into database")
					}
				}

				if existingID > 0 {
					return existingID, true
				}
			}
		}
	}

	// ── Discogs fallback ──────────────────────────────────────────────────────────
	if dg := providers.GetDiscogs(); dg != nil {
		results, err := dg.SearchReleases(
			ctx,
			logger.JoinStrings(searchArtist, " ", *albumTitle),
			5,
		)
		if err == nil {
			for i := range results {
				release, fetchErr := dg.GetReleaseByID(ctx, results[i].DiscogsID)
				if fetchErr != nil || release == nil || len(release.Tracks) != *fileCount {
					continue
				}

				var dgTotalMs int64
				for j := range release.Tracks {
					dgTotalMs += release.Tracks[j].Duration.Milliseconds()
				}

				runtimeOK := dgTotalMs == 0
				if !runtimeOK {
					diff := dgTotalMs - localTotalRuntimeMs
					if diff < 0 {
						diff = -diff
					}

					runtimeOK = diff <= toleranceMs
				}

				if !runtimeOK {
					continue
				}

				var existingID uint
				if release.DiscogsID != "" {
					database.Scanrowsdyn(
						false,
						`SELECT id FROM dbalbums WHERE discogs_release_id = ? OR discogs_master_id = ? LIMIT 1`,
						&existingID,
						&release.DiscogsID,
						&release.DiscogsID,
					)
				}

				if existingID == 0 && release.MasterID > 0 {
					masterIDStr := strconv.Itoa(release.MasterID)
					database.Scanrowsdyn(false,
						`SELECT id FROM dbalbums WHERE discogs_master_id = ? LIMIT 1`,
						&existingID, &masterIDStr,
					)
				}

				if existingID == 0 && data != nil &&
					(data.AddFound || data.AllowAlternativeReleases) {
					dgAltTitle := releaseTitleName(release.Title, albumTitle)
					slug := logger.StringToSlugCached(dgAltTitle)

					masterIDStr := ""
					if release.MasterID > 0 {
						masterIDStr = strconv.Itoa(release.MasterID)
					}

					result, insertErr := database.ExecNid(
						`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
						  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
						  label, country, total_tracks, total_runtime_ms, genres, cover_url)
						 VALUES (?, '', '', ?, ?, '', ?, ?, '', ?, ?, ?, ?, ?, '', ?)`,
						&dgAltTitle,
						&release.DiscogsID,
						&masterIDStr,
						&slug,
						&release.ReleaseYear,
						&release.Format,
						&release.Label,
						&release.Country,
						fileCount,
						&dgTotalMs,
						&release.CoverURL,
					)
					if insertErr == nil {
						existingID = logger.Int64ToUint(result)

						dgAltArtistName, dgAltArtistMBID := releaseArtistName(
							release.Artists,
							artist,
						)

						dgAltArtistNamePtr := &dgAltArtistName
						if aid := addOrGetArtist(dgAltArtistNamePtr, &dgAltArtistMBID); aid > 0 {
							pos := 0

							_, _ = database.ExecNid(
								`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position) VALUES (?, ?, ?)`,
								&existingID,
								&aid,
								&pos,
							)
						}

						insertSyntheticTracks(&existingID, release.Tracks)
						logger.Logtype("info", 1).
							Str("artist", *artist).
							Str("album", *albumTitle).
							Uint("dbalbumID", existingID).
							Msg("Discogs alternative: inserted synthetic album into database")
					}
				}

				if existingID > 0 {
					return existingID, true
				}
			}
		}
	}

	// ── Deezer fallback ──────────────────────────────────────────────────────────
	if dz := providers.GetDeezer(); dz != nil {
		results, err := dz.SearchAlbums(ctx, searchArtist+" "+*albumTitle, 5)
		if err == nil {
			for i := range results {
				if results[i].TrackCount != *fileCount {
					continue
				}

				release, fetchErr := dz.GetAlbumByID(ctx, results[i].DeezerID)
				if fetchErr != nil || release == nil || len(release.Tracks) != *fileCount {
					continue
				}

				var dzTotalMs int64
				for j := range release.Tracks {
					dzTotalMs += release.Tracks[j].Duration.Milliseconds()
				}

				runtimeOK := dzTotalMs == 0
				if !runtimeOK {
					diff := dzTotalMs - localTotalRuntimeMs
					if diff < 0 {
						diff = -diff
					}

					runtimeOK = diff <= toleranceMs
				}

				if !runtimeOK {
					continue
				}

				deezerIDStr := release.DeezerID

				var existingID uint
				if deezerIDStr != "" {
					database.Scanrowsdyn(false,
						`SELECT id FROM dbalbums WHERE deezer_id = ? LIMIT 1`,
						&existingID, &deezerIDStr,
					)
				}

				if existingID == 0 && data != nil &&
					(data.AddFound || data.AllowAlternativeReleases) {
					dzAltTitle := releaseTitleName(release.Title, albumTitle)
					slug := logger.StringToSlugCached(dzAltTitle)

					result, insertErr := database.ExecNid(
						`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
						  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
						  label, country, total_tracks, total_runtime_ms, genres, cover_url, deezer_id)
						 VALUES (?, '', '', '', '', ?, ?, ?, '', '', ?, '', ?, ?, '', ?, ?)`,
						&dzAltTitle,
						&release.Barcode,
						&slug,
						&release.ReleaseYear,
						&release.Label,
						&fileCount,
						&dzTotalMs,
						&release.CoverURL,
						&deezerIDStr,
					)
					if insertErr == nil {
						existingID = logger.Int64ToUint(result)

						dzAltArtistName, dzAltArtistMBID := releaseArtistName(
							release.Artists,
							artist,
						)

						dzAltArtistNamePtr := &dzAltArtistName
						if aid := addOrGetArtist(dzAltArtistNamePtr, &dzAltArtistMBID); aid > 0 {
							pos := 0

							_, _ = database.ExecNid(
								`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position) VALUES (?, ?, ?)`,
								&existingID,
								&aid,
								&pos,
							)
						}

						insertSyntheticTracks(&existingID, release.Tracks)
						logger.Logtype("info", 1).
							Str("artist", *artist).
							Str("album", *albumTitle).
							Uint("dbalbumID", existingID).
							Msg("Deezer alternative: inserted synthetic album into database")
					}
				}

				if existingID > 0 {
					return existingID, true
				}
			}
		}
	}

	logger.Logtype("debug", 0).
		Str("artist", searchArtist).
		Str("album", *albumTitle).
		Msg("No alternative release found with matching track count and runtime")

	return 0, false
}

// replaceExistingRelease replaces an existing DB release with a new MusicBrainz release.
// It updates the musicbrainz_release_id on the existing dbalbum, deletes old tracks,
// and re-imports metadata. Returns the database ID and success status.
func replaceExistingRelease(
	ctx context.Context,
	existingDBID uint,
	newMusicBrainzID string,
	cfgp *config.MediaTypeConfig,
	listid int,
) (uint, bool) {
	// logger.Logtype("debug", 0).
	// 	Uint("existingDBID", existingDBID).
	// 	Str("newMusicBrainzID", newMusicBrainzID).
	// 	Msg("DEBUG: Replacing existing release with alternative")

	// Update the existing dbalbum's musicbrainz_release_id
	database.ExecN(
		"UPDATE dbalbums SET musicbrainz_release_id = ? WHERE id = ?",
		&newMusicBrainzID,
		&existingDBID,
	)

	// Delete old tracks so they can be re-imported with correct data
	database.ExecN(
		"DELETE FROM dbtracks WHERE dbalbum_id = ?",
		&existingDBID,
	)

	// Re-import metadata via the standard import flow.
	// addnew=true so tracks are always re-fetched regardless of UpdatedAt — the
	// existing row has just had its tracks deleted and MBID changed.
	dbID, importErr := JobImportAlbums(ctx, newMusicBrainzID, cfgp, listid, true)
	if importErr != nil {
		logger.Logtype("debug", 0).
			Uint("existingDBID", existingDBID).
			Err(importErr).
			Msg("DEBUG: Failed to re-import metadata after replace")
		// Return the existing ID since we already updated the release ID
		return existingDBID, true
	}

	if dbID == 0 {
		dbID = existingDBID
	}

	return dbID, true
}

// replaceExistingAudiobookRelease replaces an existing dbaudiobook entry with a new Audible edition.
// It updates the ASIN, deletes all child rows (chapters, authors, narrators, titles), then
// re-imports metadata via JobImportAudiobooks with addnew=false so the existing row is reused.
func replaceExistingAudiobookRelease(
	ctx context.Context,
	existingDBID uint,
	newASIN string,
	cfgp *config.MediaTypeConfig,
	listid int,
) (uint, bool) {
	// Update the ASIN on the existing row
	database.ExecN("UPDATE dbaudiobooks SET asin = ? WHERE id = ?", &newASIN, &existingDBID)

	// Clear child tables so they can be re-populated by the import
	database.ExecN("DELETE FROM dbaudiobook_chapters WHERE dbaudiobook_id = ?", &existingDBID)
	database.ExecN("DELETE FROM dbaudiobook_authors WHERE dbaudiobook_id = ?", &existingDBID)
	database.ExecN("DELETE FROM dbaudiobook_narrators WHERE dbaudiobook_id = ?", &existingDBID)
	database.ExecN("DELETE FROM dbaudiobook_titles WHERE dbaudiobook_id = ?", &existingDBID)

	// Re-import metadata with the new ASIN (addnew=false reuses the existing row)
	dbID, importErr := JobImportAudiobooks(ctx, newASIN, cfgp, listid, false)
	if importErr != nil {
		logger.Logtype("debug", 0).
			Uint("existingDBID", existingDBID).
			Err(importErr).
			Msg("Failed to re-import metadata after audiobook release replace")

		return existingDBID, true
	}

	if dbID == 0 {
		dbID = existingDBID
	}

	return dbID, true
}

// MatchSingleAudiobookFile matches a single audio file as an individual audiobook.
// Used for multi-episode folders where each file is a separate audiobook/episode.
// Returns the matched AlbumInfo (with 1 track) and a rejection reason (empty on success).
func MatchSingleAudiobookFile(
	ctx context.Context,
	filePath string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	addToDatabase bool,
) (*parser_v2.AlbumInfo, string) {
	// logger.Logtype("debug", 0).
	// 	Str("file", filePath).
	// 	Msg("DEBUG: MatchSingleAudiobookFile called")

	// Parse filename for artist/title/track info
	parsed := parser_v2.ParseAudioFilename(filePath)

	// Read tags from the file
	tagData, _ := parser_v2.ReadAudioTags(filePath)

	var tagArtist, tagAlbumArtist, tagAlbum, tagASIN, tagGenre string
	if tagData != nil {
		tagArtist = tagData.Artist
		tagAlbumArtist = tagData.AlbumArtist
		tagAlbum = tagData.Album
		tagASIN = tagData.ASIN
		tagGenre = tagData.Genre
		parser_v2.PutTrackInfo(tagData)
	}

	// Get parent folder for additional context
	folder := filepath.Dir(filePath)
	folderArtist, _, _ := parser_v2.ParseAudioFolder(folder)

	// Build best artist/title
	artist := coalesceStr(tagAlbumArtist, tagArtist, parsed.Artist, folderArtist)
	albumTitle := coalesceStr(tagAlbum, parsed.Album, parsed.Title)
	asin := coalesceStr(tagASIN, parsed.ASIN)

	logger.Logtype("debug", 0).
		Str("file", filePath).
		Str("artist", artist).
		Str("title", albumTitle).
		Str("asin", asin).
		Msg("DEBUG: Parsed single audiobook file metadata")

	// Try to find match in database
	var dbMatch *database.AudiobookSearchResult

	// Try by ASIN first
	if asin != "" {
		dbMatch, _ = database.FindAudiobookByASIN(asin)
		// if dbMatch != nil {
		// 	logger.Logtype("debug", 0).
		// 		Str("file", filePath).
		// 		Str("asin", asin).
		// 		Uint("dbID", dbMatch.ID).
		// 		Msg("DEBUG: Found single audiobook by ASIN")
		// }
	}

	// Try by title/author combinations
	if dbMatch == nil && albumTitle != "" {
		type searchPair struct {
			author, title, source string
		}

		var pairs []searchPair

		seen := make(map[string]bool)

		pairbuf := logger.PlAddBuffer.Get()
		defer logger.PlAddBuffer.Put(pairbuf)

		addPair := func(a, t, src string) {
			if a == "" || t == "" {
				return
			}

			pairbuf.Reset()

			for i := range len(a) {
				c := a[i]
				if c >= 'A' && c <= 'Z' {
					c += 32
				}

				pairbuf.WriteByte(c)
			}

			pairbuf.WriteByte('|')

			for i := range len(t) {
				c := t[i]
				if c >= 'A' && c <= 'Z' {
					c += 32
				}

				pairbuf.WriteByte(c)
			}

			key := pairbuf.String()
			if !seen[key] {
				seen[key] = true

				pairs = append(pairs, searchPair{a, t, src})
			}
		}

		addPair(tagAlbumArtist, tagAlbum, "tag-albumartist")
		addPair(tagArtist, tagAlbum, "tag-artist")
		addPair(parsed.Artist, parsed.Album, "filename")
		addPair(parsed.Artist, parsed.Title, "filename-title")
		addPair(folderArtist, tagAlbum, "folder+tag")
		addPair(folderArtist, parsed.Title, "folder+filename")
		addPair(tagAlbumArtist, parsed.Title, "tag+filename")
		addPair(tagArtist, parsed.Title, "tag+filename")

		for i := range pairs {
			matches, err := database.FindAudiobooksByTitleAuthor(
				&pairs[i].title,
				&pairs[i].author,
				5,
			)
			if err != nil || len(matches) == 0 {
				continue
			}

			// For single-file episodes, prefer the first match (we can't verify chapter count)
			dbMatch = matches[0]
			// logger.Logtype("debug", 0).
			// 	Str("file", filePath).
			// 	Str("source", pair.source).
			// 	Str("searchTitle", pair.title).
			// 	Str("searchAuthor", pair.author).
			// 	Uint("dbID", dbMatch.ID).
			// 	Msg("DEBUG: Found single audiobook by title/author")
			break
		}
	}

	// Build album info with single track
	album := &parser_v2.AlbumInfo{
		Title:        albumTitle,
		Artist:       artist,
		Genre:        tagGenre,
		ASIN:         asin,
		SourceFolder: folder,
		TrackCount:   1,
	}

	if dbMatch != nil {
		album.DatabaseID = dbMatch.ID
		album.ExpectedTracks = 1 // Single-file episode
	}

	// If no match and addFound is enabled, try to import
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
	if album.DatabaseID == 0 && data.AddFound && album.ASIN != "" && listid != -1 {
		logger.Logtype("info", 1).
			Str("file", filePath).
			Str("asin", album.ASIN).
			Str("title", album.Title).
			Msg("Single audiobook not in database - importing via addFound")

		dbID, importErr := JobImportAudiobooks(ctx, album.ASIN, cfgp, listid, true)
		if importErr == nil && dbID != 0 {
			album.DatabaseID = dbID
		}
	}

	if album.DatabaseID == 0 {
		logger.Logtype("debug", 0).
			Str("file", filePath).
			Str("title", album.Title).
			Str("artist", album.Artist).
			Msg("Single audiobook not found in database - skipping")

		return nil, "no_match"
	}

	// Build single track
	track := parser_v2.TrackInfo{
		Filepath:    filePath,
		Filename:    filepath.Base(filePath),
		Extension:   strings.ToLower(filepath.Ext(filePath)),
		Format:      logger.ExtToFormat(filepath.Ext(filePath)),
		TrackNumber: 1,
		DiscNumber:  1,
		Title:       albumTitle,
		Artist:      artist,
		Album:       albumTitle,
	}
	if tagData != nil {
		track.RuntimeMS = tagData.RuntimeMS
		track.Runtime = tagData.Runtime
		track.Bitrate = tagData.Bitrate
		track.SampleRate = tagData.SampleRate
		track.FileSize = tagData.FileSize
		track.QualityProfile = tagData.QualityProfile
	}

	album.Tracks = []parser_v2.TrackInfo{track}
	album.TotalRuntime = track.Runtime

	// Add to database if requested
	if addToDatabase {
		if !addAudiobookFilesToDatabase(ctx, folder, album, cfgp, listid) {
			return album, "no_match"
		}
	}

	return album, ""
}

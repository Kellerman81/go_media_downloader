package importfeed

import (
	"context"
	"math"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
)

// mbTracksToDbTracks converts MusicBrainz Track slice to DbtrackWithArtist,
// preserving position, disc/track number, runtime, recording ID, and artist credit.
func mbTracksToDbTracks(tracks []apiexternal_v2.Track) []database.DbtrackWithArtist {
	result := make([]database.DbtrackWithArtist, len(tracks))
	for i, t := range tracks {
		trackNum := t.TrackNumber
		if trackNum == 0 {
			trackNum = t.Position
		}

		artist := t.ArtistCredit
		if artist == "" && len(t.Artists) > 0 {
			artist = t.Artists[0].Name
		}

		runtimeMs := t.Duration.Milliseconds()
		if runtimeMs == 0 {
			runtimeMs = int64(t.DurationMs)
		}

		result[i] = database.DbtrackWithArtist{
			Dbtrack: database.Dbtrack{
				Title:                  t.Title,
				MusicbrainzRecordingID: t.MusicBrainzID,
				ISRC:                   t.ISRC,
				RuntimeMs:              runtimeMs,
				TrackNumber:            uint16(trackNum),
				DiscNumber:             uint16(t.DiscNumber),
			},
			Artist: artist,
		}
	}

	return result
}

// audnexChaptersToDbTracks converts Audnex AudiobookChapter slice to DbtrackWithArtist.
// Chapters have no recording ID or per-chapter artist.
func audnexChaptersToDbTracks(
	chapters []apiexternal_v2.AudiobookChapter,
) []database.DbtrackWithArtist {
	result := make([]database.DbtrackWithArtist, len(chapters))
	for i, ch := range chapters {
		num := ch.ChapterNumber
		if num == 0 {
			num = ch.Number
		}

		runtimeMs := ch.LengthMs
		if runtimeMs == 0 {
			runtimeMs = ch.Duration.Milliseconds()
		}

		result[i] = database.DbtrackWithArtist{
			Dbtrack: database.Dbtrack{
				Title:       ch.Title,
				RuntimeMs:   runtimeMs,
				TrackNumber: uint16(num),
				DiscNumber:  1,
			},
		}
	}

	return result
}

// resolveTracksForMatching returns DbtrackWithArtist records for the given album,
// falling back to the appropriate external API when the database has no track rows:
//
//   - music albums:  falls back to MusicBrainz GetReleaseByID (mbReleaseID required)
//   - audiobooks:    falls back to Audnex GetChaptersByASIN (asin required)
//
// Returns nil when no data is available from any source.

const (
	rateLimitSleep      = 20 * time.Second
	rateLimitMaxRetries = 3
)

// retryOnRateLimit calls fn() and retries up to rateLimitMaxRetries times when
// the provider returns any rate-limit error:
//
//	"server rate limit active, retry after …"
//	"rate limit exceeded, …"
//	"daily rate limit exceeded, …"
//	"total rate limit exceeded …"
//
// Circuit-breaker errors ("circuit breaker is open …") are NOT retried.
// Any other error is returned immediately.
func retryOnRateLimit[T any](ctx context.Context, fn func() (T, error)) (T, error) {
	for attempt := 0; ; attempt++ {
		result, err := fn()
		if err == nil || attempt >= rateLimitMaxRetries {
			return result, err
		}
		if !strings.Contains(err.Error(), "rate limit") {
			return result, err
		}
		logger.Logtype("warning", 1).
			Int("attempt", attempt+1).
			Int("max", rateLimitMaxRetries).
			Dur("sleep", rateLimitSleep).
			Err(err).
			Msg("Provider rate limit hit, sleeping before retry")
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(rateLimitSleep):
		}
	}
}

func resolveTracksForMatching(
	ctx context.Context,
	albumID uint,
	mbReleaseID string, // music: MusicBrainz release ID (empty for audiobooks)
	asin, region string, // audiobook: Audnex ASIN + region (empty for music)
	albumTitle, artist string, // music: used for Last.fm/Discogs fallback when MB is absent
) []database.DbtrackWithArtist {
	// Always try the database first — it's free and already populated for known releases.
	if albumID > 0 {
		if dbTracks := database.GetDbtracksByAlbumIDWithArtist(albumID); len(dbTracks) > 0 {
			// For music: always prefer MusicBrainz runtime (more accurate/up-to-date).
			// Fall back to the DB value only when MB doesn't provide one.
			if mbReleaseID != "" {
				if mbProvider := providers.GetMusicBrainz(); mbProvider != nil {
					if release, err := retryOnRateLimit(
						ctx,
						func() (*apiexternal_v2.ReleaseDetails, error) {
							return mbProvider.GetReleaseByID(ctx, mbReleaseID)
						},
					); err == nil &&
						len(release.Tracks) > 0 {
						mbTracks := mbTracksToDbTracks(release.Tracks)

						mbByNum := make(map[uint32]int64, len(mbTracks))
						for _, t := range mbTracks {
							disc := uint32(t.DiscNumber)
							if disc == 0 {
								disc = 1
							}

							mbByNum[disc*10000+uint32(t.TrackNumber)] = t.RuntimeMs
						}

						for i := range dbTracks {
							disc := uint32(dbTracks[i].DiscNumber)
							if disc == 0 {
								disc = 1
							}

							if ms, ok := mbByNum[disc*10000+uint32(dbTracks[i].TrackNumber)]; ok &&
								ms > 0 {
								dbTracks[i].RuntimeMs = ms
							}

							// else keep existing DB value as fallback
						}
					}
				}
			}

			return dbTracks
		}
	}

	// Music fallback: fetch track listing from MusicBrainz.
	if mbReleaseID != "" {
		mbProvider := providers.GetMusicBrainz()
		if mbProvider != nil {
			if release, err := retryOnRateLimit(
				ctx,
				func() (*apiexternal_v2.ReleaseDetails, error) {
					return mbProvider.GetReleaseByID(ctx, mbReleaseID)
				},
			); err == nil && release != nil &&
				len(release.Tracks) > 0 {
				return mbTracksToDbTracks(release.Tracks)
			}
		}
	}

	// Music fallback: Last.fm and Discogs when MusicBrainz is absent or returned nothing.
	// Only attempted when we have artist+title to search with.
	if albumTitle != "" && artist != "" {
		if lfm := providers.GetLastFM(); lfm != nil {
			if release, err := lfm.GetAlbumInfo(ctx, artist, albumTitle, ""); err == nil &&
				release != nil && len(release.Tracks) > 0 {
				return mbTracksToDbTracks(release.Tracks)
			}
		}

		if dg := providers.GetDiscogs(); dg != nil {
			if results, err := dg.SearchReleases(ctx, artist+" "+albumTitle, 3); err == nil {
				for i := range results {
					if release, err := dg.GetReleaseByID(ctx, results[i].DiscogsID); err == nil &&
						release != nil && len(release.Tracks) > 0 {
						return mbTracksToDbTracks(release.Tracks)
					}
				}
			}
		}

		if dz := providers.GetDeezer(); dz != nil {
			if results, err := dz.SearchAlbums(ctx, artist+" "+albumTitle, 3); err == nil {
				for i := range results {
					if release, err := dz.GetAlbumByID(ctx, results[i].DeezerID); err == nil &&
						release != nil && len(release.Tracks) > 0 {
						return mbTracksToDbTracks(release.Tracks)
					}
				}
			}
		}
	}

	// Audiobook fallback: fetch chapter listing from Audnex.
	if asin != "" {
		audnexProvider := providers.GetAudnex()
		if audnexProvider != nil {
			if chapters, err := audnexProvider.GetChaptersByASIN(
				ctx,
				asin,
				region,
			); err == nil &&
				len(chapters) > 0 {
				return audnexChaptersToDbTracks(chapters)
			}
		}
	}

	return nil
}

// GetExpectedTrackRuntimes retrieves expected track runtimes from the database.
// For music albums, queries dbtracks. For audiobooks, queries dbaudiobook_chapters.
func GetExpectedTrackRuntimes(databaseID uint, isAudiobook bool) []int64 {
	if databaseID == 0 {
		return nil
	}

	var query, countQuery string
	if isAudiobook {
		countQuery = "SELECT count() FROM dbaudiobook_chapters WHERE dbaudiobook_id = ?"
		query = "SELECT runtime_ms FROM dbaudiobook_chapters WHERE dbaudiobook_id = ? ORDER BY chapter_number"
	} else {
		countQuery = "SELECT count() FROM dbtracks WHERE dbalbum_id = ?"
		query = "SELECT runtime_ms FROM dbtracks WHERE dbalbum_id = ? ORDER BY disc_number, track_number"
	}

	return database.Getrowssize[int64](false, countQuery, query, &databaseID)
}

// allRuntimesZero returns true if the slice is non-empty and every element is 0.
func allRuntimesZero(runtimes []int64) bool {
	if len(runtimes) == 0 {
		return false
	}

	for _, r := range runtimes {
		if r != 0 {
			return false
		}
	}

	return true
}

// findASINByTitleAuthor searches Audible using the provided search pairs (same pairs
// used for DB lookup in matchAudiobookFolder, including stripped episode/series variants).
// Audible is queried once per unique title (results cached), then every pair's author
// is checked against the cached results — so no combination is skipped.
// region must be a valid Audible region (e.g. "de", "us") from cfgp.AudibleRegion.
// fileCount is the number of local audio files; when >0 and multiple ASINs match,
// Audnex chapter count is used to pick the correct edition. Fail-open to the first
// candidate if Audnex is unavailable or no exact chapter match is found.
// Returns "" when no confident match is found so the caller can skip addFound.
func findASINByTitleAuthor(
	ctx context.Context,
	pairs []searchPair,
	region string,
	fileCount int,
) string {
	prov := getOrCreateAudibleProvider(region)

	// Cache Audible results keyed by lowercase title to avoid duplicate API calls.
	type cacheEntry struct {
		results []apiexternal_v2.AudiobookSearchResult
		fetched bool
	}

	cache := make(map[string]*cacheEntry)

	// Collect all candidate ASINs (deduplicated) instead of returning on first match.
	// A different edition with the wrong chapter count may appear before the correct one.
	var candidates []string

	seenASINs := make(map[string]bool)

	for i := range pairs {
		if pairs[i].title == "" {
			continue
		}

		titleKey := strings.ToLower(pairs[i].title)

		// Fetch from Audible once per unique title.
		entry, exists := cache[titleKey]
		if !exists {
			entry = &cacheEntry{fetched: true}

			results, err := prov.SearchByTitle(ctx, pairs[i].title, 10)
			if err == nil {
				entry.results = results
			}

			cache[titleKey] = entry
		}

		if len(entry.results) == 0 {
			continue
		}

		// Check every Audible result against this pair's title+author.
		authorLower := strings.ToLower(pairs[i].author)
		for j := range entry.results {
			if entry.results[j].ASIN == "" || seenASINs[entry.results[j].ASIN] {
				continue
			}

			rTitleLower := strings.ToLower(entry.results[j].Title)
			if !strings.Contains(rTitleLower, titleKey) &&
				!strings.Contains(titleKey, rTitleLower) {
				continue
			}

			if authorLower != "" {
				authorMatched := false
				// Exact substring check against Authors.
				for _, a := range entry.results[j].Authors {
					aLower := strings.ToLower(a)
					if strings.Contains(aLower, authorLower) ||
						strings.Contains(authorLower, aLower) {
						authorMatched = true
						break
					}
				}

				// Fallback: word-level partial match against Authors + SeriesName/Series.
				// Handles cases where the local AlbumArtist is a series name stored differently
				// (e.g. "Die drei Fragezeichen" in tags vs "Die drei ???" in Audible series).
				// A word ≥4 chars from the local author appearing anywhere in the Audible
				// author or series fields is treated as a partial match.
				if !authorMatched {
					for word := range strings.FieldsSeq(authorLower) {
						if len(word) < 4 {
							continue
						}

						for _, a := range entry.results[j].Authors {
							if strings.Contains(strings.ToLower(a), word) {
								authorMatched = true
								break
							}
						}

						if !authorMatched {
							for _, sn := range []string{entry.results[j].SeriesName, entry.results[j].Series} {
								if strings.Contains(strings.ToLower(sn), word) {
									authorMatched = true
									break
								}
							}
						}

						if authorMatched {
							break
						}
					}
				}

				if !authorMatched {
					continue
				}
			}

			logger.Logtype("debug", 0).
				Str("searchTitle", pairs[i].title).
				Str("searchAuthor", pairs[i].author).
				Str("source", pairs[i].source).
				Str("matchedTitle", entry.results[j].Title).
				Str("matchedASIN", entry.results[j].ASIN).
				Msg("Audible search pair matched audiobook candidate")

			seenASINs[entry.results[j].ASIN] = true
			candidates = append(candidates, entry.results[j].ASIN)
		}
	}

	if len(candidates) == 0 {
		return ""
	}

	// Single candidate or no file count to verify — return immediately.
	if fileCount <= 0 || len(candidates) == 1 {
		return candidates[0]
	}

	// Multiple candidates: verify chapter count via Audnex to pick the right edition.
	audnexProvider := providers.GetAudnex()
	if audnexProvider == nil {
		// Audnex unavailable — fail-open with first candidate.
		return candidates[0]
	}

	for _, asin := range candidates {
		chapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, region)
		if err != nil || len(chapters) == 0 {
			continue
		}

		if len(chapters) == fileCount {
			logger.Logtype("debug", 0).
				Str("asin", asin).
				Int("audnexChapters", len(chapters)).
				Int("fileCount", fileCount).
				Msg("Audible ASIN verified by chapter count via Audnex")

			return asin
		}

		logger.Logtype("debug", 0).
			Str("asin", asin).
			Int("audnexChapters", len(chapters)).
			Int("fileCount", fileCount).
			Msg("Audible ASIN chapter count mismatch — skipping candidate")
	}

	// No exact chapter count match — fail-open with first candidate.
	logger.Logtype("debug", 0).
		Strs("candidates", candidates).
		Int("fileCount", fileCount).
		Msg("No Audible ASIN chapter count matched fileCount — using first candidate")

	return candidates[0]
}

// luceneEscapeTo writes the Lucene-escaped form of s into buf.
// Special characters: + - && || ! ( ) { } [ ] ^ " ~ * ? : \ /
func luceneEscapeTo(buf *logger.AddBuffer, s string) {
	for _, c := range s {
		switch c {
		case '+', '-', '!', '(', ')', '{', '}', '[', ']', '^', '"', '~', '*', '?', ':', '\\', '/':
			buf.WriteByte('\\')
		case '&', '|':
			// && and || are two-character operators; escape each rune individually.
			buf.WriteByte('\\')
		}
		buf.WriteRune(c)
	}
}

// LuceneEscape escapes all Lucene special characters in s so the value is safe
// to embed inside a quoted phrase or unquoted term in a MusicBrainz search query.
func LuceneEscape(s string) string {
	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)
	luceneEscapeTo(buf, s)
	return buf.String()
}

// BuildArtistAlbumSearch builds a MusicBrainz Lucene query for a release title
// and optional artist. For non-VA artists it appends a slop-2 phrase filter
// (artist:"Name"~2) which outperforms exact-phrase and per-word-AND in live API
// tests (including edge cases like Deep Purple "Machine Head" and AC/DC).
func BuildArtistAlbumSearch(album, artist string) []byte {
	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)
	buf.WriteString("release:")
	buf.WriteString(album)
	if artist != "" && !IsVariousArtists(artist) {
		buf.WriteString(` AND artist:"`)
		luceneEscapeTo(buf, artist)
		buf.WriteString(`"~2`)
	}
	return append([]byte(nil), buf.Bytes()...)
}

// coalesceStr returns the first non-empty string.
func coalesceStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

// addAudiobookFilesToDatabase adds audiobook files to the database.
func addAudiobookFilesToDatabase(
	ctx context.Context,
	folder string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	listid int,
) bool {
	// logger.Logtype("debug", 0).
	// 	Str("folder", folder).
	// 	Uint("databaseID", album.DatabaseID).
	// 	Int("listid", listid).
	// 	Int("trackCount", len(album.Tracks)).
	// 	Msg("DEBUG: addAudiobookFilesToDatabase called")
	if album.DatabaseID == 0 || listid == -1 {
		// logger.Logtype("debug", 0).
		// 	Str("folder", folder).
		// 	Uint("databaseID", album.DatabaseID).
		// 	Int("listid", listid).
		// 	Msg("DEBUG: addAudiobookFilesToDatabase returning false - invalid IDs")
		return false
	}

	// Resolve the correct audiobooks.id (list entry) from dbaudiobook_id + listname
	var listname string
	if listid >= 0 && listid < len(cfgp.Lists) {
		listname = cfgp.Lists[listid].Name
	}

	audiobookListID := database.GetAudiobookListEntryID(album.DatabaseID, listname)
	if audiobookListID == 0 {
		// logger.Logtype("error", 1).
		// 	Str("folder", folder).
		// 	Uint("dbaudiobookID", album.DatabaseID).
		// 	Str("listname", listname).
		// 	Msg("Could not find audiobook list entry for dbaudiobook_id")
		return false
	}

	// Step 6a: Update rootpath
	if err := database.UpdateAudiobookRootpath(audiobookListID, folder); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to update audiobook rootpath")
	}

	// Add each file to the database
	filesAdded := 0

	filesSkipped := 0
	for i := range album.Tracks {
		if err := ctx.Err(); err != nil {
			return false
		}

		// Skip files already in database
		if database.AudiobookFileExists(&album.Tracks[i].Filepath) {
			filesSkipped++
			continue
		}

		// Insert file record (audiobookListID = audiobooks.id, album.DatabaseID = dbaudiobook_id)
		if err := database.InsertAudiobookFile(
			audiobookListID,
			album.Tracks[i].Filepath,
			album.Tracks[i].Filename,
			album.Tracks[i].Extension,
			album.Tracks[i].Format,
			album.Tracks[i].QualityProfile,
			album.Tracks[i].FileSize,
			album.Tracks[i].Bitrate,
			album.Tracks[i].RuntimeMS,
			album.Tracks[i].TrackNumber,
			album.Tracks[i].DiscNumber,
			album.DatabaseID,
		); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to insert audiobook file")

			continue
		}

		filesAdded++

		// Step 6b: Tag file if added (add to cache)
		if config.GetSettingsGeneral().UseFileCache {
			database.AppendCacheMap(cfgp.IsType, logger.CacheFiles, album.Tracks[i].Filepath)
		}
	}

	// logger.Logtype("debug", 0).
	// 	Str("folder", folder).
	// 	Int("filesAdded", filesAdded).
	// 	Int("filesSkipped", filesSkipped).
	// 	Int("totalTracks", len(album.Tracks)).
	// 	Msg("DEBUG: addAudiobookFilesToDatabase complete")

	if filesAdded > 0 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("title", album.Title).
			Uint("audiobook_id", album.DatabaseID).
			Int("files_added", filesAdded).
			Msg("Added audiobook files to database")
	}

	return true
}

// addAlbumFilesToDatabase adds music album files to the database.
func addAlbumFilesToDatabase(
	ctx context.Context,
	folder string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	listid int,
) bool {
	if album.DatabaseID == 0 || listid == -1 {
		return false
	}

	// Resolve the correct albums.id from dbalbum_id + listname
	// album.DatabaseID is dbalbum_id, NOT albums.id
	var listname string
	if listid >= 0 && listid < len(cfgp.Lists) {
		listname = cfgp.Lists[listid].Name
	}

	albumListID := database.GetAlbumListEntryID(album.DatabaseID, listname)
	if albumListID == 0 {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Uint("dbalbumID", album.DatabaseID).
			Str("listname", listname).
			Msg("Could not find album list entry for dbalbum_id")

		return false
	}

	// Step 6a: Update rootpath using the correct albums.id
	if err := database.UpdateAlbumRootpath(albumListID, folder); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to update album rootpath")
	}

	// Add each file to the database
	filesAdded := 0
	for i := range album.Tracks {
		if err := ctx.Err(); err != nil {
			return false
		}

		// Skip files already in database
		if database.AlbumFileExists(&album.Tracks[i].Filepath) {
			continue
		}

		// Look up the dbtrack_id from dbtracks table
		var dbTrackID uint
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbtracks WHERE dbalbum_id = ? AND disc_number = ? AND track_number = ?",
			&dbTrackID,
			&album.DatabaseID,
			&album.Tracks[i].DiscNumber,
			&album.Tracks[i].TrackNumber,
		)

		// If AcoustID is empty, try to generate it via fingerprinting
		acoustID := album.Tracks[i].AcoustID
		if acoustID == "" && providers.GetAcoustID() != nil {
			// Try fingerprinting with a timeout to avoid blocking
			result, err := tags.FingerprintAndIdentifyWithTimeout(
				album.Tracks[i].Filepath,
				15*time.Second,
			)
			if err == nil && result.AcoustID != "" {
				acoustID = result.AcoustID
				logger.Logtype("debug", 1).
					Str("file", album.Tracks[i].Filepath).
					Str("acoustid", acoustID).
					Msg("Generated AcoustID via fingerprinting")

				// Also update the dbtrack if we have one
				if dbTrackID > 0 {
					_, _ = database.ExecNid(
						"UPDATE dbtracks SET acoustid = ? WHERE id = ?",
						&acoustID, &dbTrackID,
					)
				}
			} else if err != nil {
				logger.Logtype("debug", 2).
					Str("file", album.Tracks[i].Filepath).
					Err(err).
					Msg("Failed to generate AcoustID via fingerprinting")
			}
		}

		// Insert file record — albumListID is albums.id, album.DatabaseID is dbalbum_id
		trackFile := &database.TrackFileInfo{
			Filepath:       album.Tracks[i].Filepath,
			Filename:       album.Tracks[i].Filename,
			Extension:      album.Tracks[i].Extension,
			Format:         album.Tracks[i].Format,
			QualityProfile: album.Tracks[i].QualityProfile,
			FileSize:       album.Tracks[i].FileSize,
			Bitrate:        album.Tracks[i].Bitrate,
			SampleRate:     album.Tracks[i].SampleRate,
			BitDepth:       album.Tracks[i].BitDepth,
			RuntimeMS:      album.Tracks[i].RuntimeMS,
			TrackNumber:    album.Tracks[i].TrackNumber,
			DiscNumber:     album.Tracks[i].DiscNumber,
		}
		if err := database.InsertAlbumFile(
			albumListID,
			trackFile,
			acoustID,
			album.DatabaseID,
			dbTrackID,
		); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to insert album file")

			continue
		}

		filesAdded++

		// Step 6b: Tag file if added (add to cache)
		if config.GetSettingsGeneral().UseFileCache {
			database.AppendCacheMap(cfgp.IsType, logger.CacheFiles, album.Tracks[i].Filepath)
		}
	}

	if filesAdded > 0 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("title", album.Title).
			Uint("album_id", album.DatabaseID).
			Int("files_added", filesAdded).
			Msg("Added album files to database")
	}

	return true
}

// majorityRelease returns the release ID that appears in at least threshold entries of
// releaseCounts, preferring the highest count. Returns "" if nothing meets the threshold.
func majorityRelease(releaseCounts map[string]int, total int) string {
	threshold := int(math.Ceil(float64(total) * acoustidRelThresh))
	best, bestCount := "", 0

	for id, count := range releaseCounts {
		if count >= threshold && count > bestCount {
			best, bestCount = id, count
		}
	}

	return best
}

// resolveAlbumMBID attempts to find a MusicBrainz release MBID for a folder when normal
// tag-based matching has failed. Four stages, cheapest first:
//  1. LastFM album lookup by artist+title (one HTTP call, no local tools)
//  2. ISRC lookup via MusicBrainz — reads file tags for ISRCs, queries MB; majority vote
//  3. DiscID lookup — SHA1 of track offsets; uses tag-embedded ID or computes from durations
//  4. AcoustID fingerprint majority vote — runs fpcalc, requires AcoustID API key
func resolveAlbumMBID(
	ctx context.Context,
	folder string,
	files []string,
	artist, albumTitle string,
) string {
	// 1. LastFM — cheapest, just needs artist+album text
	if artist != "" && albumTitle != "" {
		if lfm := providers.GetLastFM(); lfm != nil {
			if details, err := lfm.GetAlbumInfo(
				ctx,
				artist,
				albumTitle,
				"",
			); err == nil && details != nil &&
				details.MusicBrainzID != "" {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Str("mbid", details.MusicBrainzID).
					Msg("MusicBrainzID resolved via LastFM fallback")

				return details.MusicBrainzID
			}
		}
	}

	// 2. ISRC → MusicBrainz release lookup
	// Read tags for up to isrcMaxFiles files and collect unique ISRCs.
	if mb := providers.GetMusicBrainz(); mb != nil {
		n := min(len(files), isrcMaxFiles)
		isrcCounts := make(map[string]int, n)

		for i := range n {
			if track := parser_v2.ReadTagsForFirstFile(
				files[i : i+1],
			); track != nil &&
				track.ISRC != "" {
				isrcCounts[track.ISRC]++
			}
		}

		if len(isrcCounts) > 0 {
			releaseCounts := make(map[string]int, n*2)
			lookedUp := 0

			for isrc := range isrcCounts {
				ids, err := mb.GetReleaseIDsByISRC(ctx, isrc)
				if err != nil {
					continue
				}

				lookedUp++

				for _, id := range ids {
					releaseCounts[id]++
				}
			}

			if lookedUp > 0 {
				if best := majorityRelease(releaseCounts, lookedUp); best != "" {
					logger.Logtype("debug", 1).
						Str("folder", folder).
						Str("mbid", best).
						Int("isrcs", lookedUp).
						Msg("MusicBrainzID resolved via ISRC fallback")

					return best
				}
			}
		}
	}

	// 3. DiscID lookup — no external tools, just file durations + SHA1
	// First check if any file already has MUSICBRAINZ_DISCID embedded by the ripper.
	// Fall back to computing it from file durations (works well for lossless rips).
	if mb := providers.GetMusicBrainz(); mb != nil {
		discID := ""

		// Check tag-embedded DiscID first (exact, reliable).
		checkN := min(len(files), isrcMaxFiles)
		for i := range checkN {
			if track := parser_v2.ReadTagsForFirstFile(
				files[i : i+1],
			); track != nil &&
				track.DiscID != "" {
				discID = track.DiscID
				break
			}
		}

		// If not in tags, try computing it from file durations.
		// Require at least 2 tracks — a single-file DiscID is far too ambiguous
		// (leadout ≈ 150 + duration×75 would collide with many unrelated releases).
		if discID == "" && len(files) >= 2 {
			if computed, err := parser_v2.CalculateDiscID(files); err == nil {
				discID = computed
			}
		}

		if discID != "" {
			if ids, err := mb.GetReleaseIDsByDiscID(ctx, discID); err == nil && len(ids) > 0 {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Str("discid", discID).
					Str("mbid", ids[0]).
					Msg("MusicBrainzID resolved via DiscID fallback")

				return ids[0]
			}
		}
	}

	// 4. AcoustID majority vote — fingerprint up to acoustidMaxFiles tracks.
	// Require at least 2 files: with 1 file the threshold is ceil(1×0.6)=1, meaning
	// any single result passes and the release association is unreliable.
	// Skip entirely when AcoustID provider is not registered (disabled in config).
	if providers.GetAcoustID() == nil || len(files) < 2 {
		return ""
	}

	n := min(len(files), acoustidMaxFiles)

	releaseCounts := make(map[string]int, n*2)
	fingerprintedFiles := 0

	for i := range n {
		releaseIDs, err := tags.FingerprintReleaseIDs(ctx, files[i])
		if err != nil {
			logger.Logtype("debug", 2).
				Str("file", files[i]).
				Err(err).
				Msg("AcoustID fingerprint failed")

			continue
		}

		fingerprintedFiles++

		for _, id := range releaseIDs {
			releaseCounts[id]++
		}
	}

	if fingerprintedFiles == 0 {
		return ""
	}

	if best := majorityRelease(releaseCounts, fingerprintedFiles); best != "" {
		logger.Logtype("debug", 1).
			Str("folder", folder).
			Str("mbid", best).
			Int("files", fingerprintedFiles).
			Msg("MusicBrainzID resolved via AcoustID majority vote")

		return best
	}

	return ""
}

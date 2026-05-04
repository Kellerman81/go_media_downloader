package importfeed

import (
	"context"
	"math"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

var (
	seriesPrefixKeywords = []string{
		"folge", "episode", "chapter", "band", "teil",
		"volume", "vol.", "book", "buch", "nr.", "no.",
	}
	seriesSuffixKeywords = []string{
		"folge", "reihe", "chapter", "episode", "band", "teil",
		"volume", "vol.", "book", "buch", "nr.", "nr ", "no.", "no ",
	}
)

// ── Shared types ─────────────────────────────────────────────────────────────

// folderMetadata holds everything that can be derived from a folder path, the
// first filename, the parent directory, and audio-file tags.  It is built once
// by parseFolderMetadata and passed as a value to sub-functions of both
// matchMusicFolder and matchAudiobookFolder.
type folderMetadata struct {
	FolderBase        string
	FolderArtist      string
	FolderAlbum       string
	Year              int
	RawArtist         string // from a raw hyphen-split when the parser found no artist
	RawAlbum          string
	FilenameArtist    string
	FilenameAlbum     string
	ParentDir         string
	ParentArtist      string
	ParentAlbum       string
	GrandparentArtist string
	IsDiscFolder      bool
	TagArtist         string
	TagAlbumArtist    string
	TagAlbum          string
	TagGenre          string
	TagMusicBrainzID  string // populated for music only
	TagASIN           string // populated for audiobooks only (from file tags)
	FilenameASIN      string // populated for audiobooks only (from first filename)
}

// musicSearchPair is a unique artist+album combination tried against the DB.
type musicSearchPair struct {
	artist string
	album  string
	source string
}

// preScoredC couples a music album candidate with its pre-computed album-only
// distance so the candidates can be sorted before expensive track resolution.
type preScoredC struct {
	c    *database.AlbumSearchResult
	dist float64
}

// musicValidMatch is a music candidate that passed the full track-matching pipeline.
type musicValidMatch struct {
	c             *database.AlbumSearchResult
	fullDist      float64
	matchedTracks []parser_v2.TrackInfo
}

// audiobookValidMatch is an audiobook candidate that passed the full track-matching pipeline.
type audiobookValidMatch struct {
	c             *database.AudiobookSearchResult
	fullDist      float64
	matchedTracks []parser_v2.TrackInfo
}

// ── Shared helpers ────────────────────────────────────────────────────────────

// parseFolderMetadata extracts all available metadata from the folder path, the
// first audio filename, the parent directory, and file tags.  The result is used
// by both matchMusicFolder and matchAudiobookFolder.
func parseFolderMetadata(
	folder string,
	files []string,
	data *config.MediaDataConfig,
) folderMetadata {
	var m folderMetadata

	m.FolderBase = filepath.Base(folder)
	m.FolderArtist, m.FolderAlbum, m.Year = parser_v2.ParseAudioFolder(folder)

	// Raw hyphen-split for "Artist - Album" patterns the structured parser missed.
	if idx := strings.Index(m.FolderBase, "-"); idx > 0 &&
		idx < len(m.FolderBase)-1 &&
		m.FolderArtist == "" {
		r := strings.NewReplacer("_", " ", ".", " ")

		m.RawArtist = r.Replace(strings.TrimSpace(m.FolderBase[:idx]))
		m.RawAlbum = r.Replace(strings.TrimSpace(m.FolderBase[idx+1:]))
	}

	// First filename.
	if len(files) > 0 {
		first := parser_v2.ParseAudioFilename(files[0])

		m.FilenameArtist = first.Artist
		m.FilenameAlbum = first.Album
		m.FilenameASIN = first.ASIN
	}

	// Parent directory — suppressed when it equals the configured walk path.
	m.ParentDir = filepath.Dir(folder)

	m.ParentArtist, m.ParentAlbum, _ = parser_v2.ParseAudioFolder(m.ParentDir)
	if data != nil && data.CfgPath != nil && data.CfgPath.Path != "" {
		walkBase := filepath.Base(data.CfgPath.Path)

		parentBase := filepath.Base(m.ParentDir)
		if strings.EqualFold(parentBase, walkBase) ||
			strings.EqualFold(m.ParentDir, data.CfgPath.Path) {
			m.ParentArtist = ""
			m.ParentAlbum = ""
		}
	}

	// Disc/volume folder detection (CD1, Disc 2, Vol 3, …).
	m.IsDiscFolder = logger.HasPrefixI(m.FolderBase, "cd") ||
		logger.HasPrefixI(m.FolderBase, "disc") ||
		logger.HasPrefixI(m.FolderBase, "disk") ||
		logger.HasPrefixI(m.FolderBase, "vol") ||
		logger.HasPrefixI(m.FolderBase, "volume") ||
		strings.EqualFold(m.FolderBase, "disc") ||
		strings.EqualFold(m.FolderBase, "disk")

	if m.IsDiscFolder && m.ParentAlbum != "" {
		m.GrandparentArtist, _, _ = parser_v2.ParseAudioFolder(filepath.Dir(m.ParentDir))
	}

	// File tags.
	if tagData := parser_v2.ReadTagsForFirstFile(files); tagData != nil {
		m.TagArtist = tagData.Artist
		m.TagAlbumArtist = tagData.AlbumArtist
		m.TagAlbum = tagData.Album
		m.TagGenre = tagData.Genre
		m.TagMusicBrainzID = tagData.MusicBrainzID
		m.TagASIN = tagData.ASIN
	}

	return m
}

// stripLeadingEpisodeNumber removes a leading run of digits + separator from
// title, e.g. "001 - Der Super-Papagei" → "Der Super-Papagei".
// Returns "" when no leading number is found.
func stripLeadingEpisodeNumber(title string) string {
	for i, c := range title {
		if c >= '0' && c <= '9' {
			continue
		}

		if i > 0 {
			rest := strings.TrimLeft(title[i:], " .-_")
			if rest != "" && len(rest) < len(title) {
				return rest
			}
		}

		break
	}

	return ""
}

// stripSeriesPrefix removes a known series keyword + number + separator from the
// start of title, e.g. "Folge 09: Das leere Grab" → "Das leere Grab".
// Returns "" when no match is found.  Audiobook-specific.
func stripSeriesPrefix(title string) string {
	for i := range seriesPrefixKeywords {
		if !logger.HasPrefixI(title, seriesPrefixKeywords[i]) {
			continue
		}

		rest := title[len(seriesPrefixKeywords[i]):]

		i := 0
		for i < len(rest) && (rest[i] == ' ' || (rest[i] >= '0' && rest[i] <= '9')) {
			i++
		}

		if i > 0 && i < len(rest) {
			if trimmed := strings.TrimLeft(rest[i:], ":.- "); trimmed != "" {
				return trimmed
			}
		}
	}

	return ""
}

// stripSeriesSuffix removes a trailing parenthetical that contains a series
// keyword or a digit, e.g. "Der Fall Jane Eyre (Thursday Next 1)" → "Der Fall Jane Eyre".
// Returns "" when no match is found.  Audiobook-specific.
func stripSeriesSuffix(title string) string {
	idx := strings.LastIndex(title, "(")
	if idx <= 0 {
		return ""
	}

	suffix := title[idx:]
	for i := range seriesSuffixKeywords {
		if logger.ContainsI(suffix, seriesSuffixKeywords[i]) {
			return strings.TrimSpace(title[:idx])
		}
	}

	for _, c := range suffix {
		if c >= '0' && c <= '9' {
			return strings.TrimSpace(title[:idx])
		}
	}

	return ""
}

// applyTrackDBOverrides copies track-number, disc-number, title, and expected
// runtime from dbTracks[i] into result[i] for every matched entry, and computes
// TrackDist.  isAudiobook controls which matching rules trackDistance applies.
func applyTrackDBOverrides(
	result []parser_v2.TrackInfo,
	matched []bool,
	dbTracks []database.DbtrackWithArtist,
	isVA, isAudiobook bool,
	data *config.MediaDataConfig,
) []parser_v2.TrackInfo {
	out := make([]parser_v2.TrackInfo, 0, len(result))
	for i, tr := range result {
		if !matched[i] {
			continue
		}

		if dbTracks[i].TrackNumber > 0 {
			tr.TrackNumber = int(dbTracks[i].TrackNumber)
		}

		if dbTracks[i].DiscNumber > 0 {
			tr.DiscNumber = int(dbTracks[i].DiscNumber)
		}

		if dbTracks[i].Title != "" {
			tr.Title = dbTracks[i].Title
		}

		if dbTracks[i].RuntimeMs > 0 {
			tr.ExpectedRuntimeMS = dbTracks[i].RuntimeMs
		}

		tr.TrackDist = trackDistance(&result[i], &dbTracks[i], isVA, isAudiobook, data)
		out = append(out, tr)
	}

	return out
}

// releaseArtistName returns the best artist name and MBID to use when linking a
// synthetic dbalbum. It prefers the name from the API response (more reliable
// than folder/tag parsing which can mis-identify years as artist names) and
// falls back to the locally-parsed artist only when the API provides nothing.
func releaseArtistName(
	apiArtists []apiexternal_v2.ArtistRef,
	localArtist *string,
) (name, mbid string) {
	for i := range apiArtists {
		if apiArtists[i].Name != "" {
			return apiArtists[i].Name, apiArtists[i].ID
		}
	}

	if localArtist != nil {
		return *localArtist, ""
	}

	return "", ""
}

// releaseTitleName returns the best album title to use when inserting a
// synthetic dbalbum. It prefers the title from the API response (which is
// clean and correctly formatted) over the locally-parsed folder title (which
// may include the artist name as a prefix, e.g. "Stef Bos - In Een Ander Licht").
func releaseTitleName(apiTitle string, localTitle *string) string {
	if apiTitle != "" {
		return apiTitle
	}

	if localTitle != nil {
		return *localTitle
	}

	return ""
}

// insertSyntheticTracks inserts a slice of tracks into dbtracks for a newly
// created synthetic dbalbum. It detects (disc, track_number) collisions caused
// by providers that number tracks per-disc (1-5, 1-5 for a 2-disc album) but
// return no disc indicator, and auto-increments the disc number on collision so
// every row gets a unique (disc, track_number) pair.
func insertSyntheticTracks(dbalbumID *uint, tracks []apiexternal_v2.Track) {
	seen := make(map[uint32]bool, len(tracks))

	var (
		tn, dn    int
		key       uint32
		runtimeMs int64
	)

	for j := range tracks {
		runtimeMs = tracks[j].Duration.Milliseconds()

		tn = tracks[j].TrackNumber
		if tn == 0 {
			tn = tracks[j].Position
		}

		if tn == 0 {
			tn = j + 1
		}

		dn = tracks[j].DiscNumber
		if dn == 0 {
			dn = 1
		}

		// Resolve collision: if (dn, tn) already used, increment disc until free.
		key = uint32(dn)*10000 + uint32(tn) //nolint:gosec // safe: value within target type range
		for seen[key] {
			dn++

			key = uint32(
				dn,
			)*10000 + uint32(
				tn,
			)
		}

		seen[key] = true
		_, _ = database.ExecNid(
			`INSERT INTO dbtracks (dbalbum_id, title, track_number, disc_number, runtime_ms, acoustid)
			 VALUES (?, ?, ?, ?, ?, '')`,
			dbalbumID,
			&tracks[j].Title,
			&tn,
			&dn,
			&runtimeMs,
		)
	}
}

// countUnmatched returns the number of unmatched DB entries and unused local
// tracks from the boolean slices produced by matchTracksByDistance.
func countUnmatched(matched, used []bool) (unmatchedDB, unusedLocal int) {
	for i := range matched {
		if !matched[i] {
			unmatchedDB++
		}
	}

	for i := range used {
		if !used[i] {
			unusedLocal++
		}
	}

	return
}

// runtimeDiffWithinTolerance returns true when |localMs - expectedMs| is within
// the configured tolerance (per-track or max-total).
func runtimeDiffWithinTolerance(
	localMs, expectedMs int64,
	fileCount int,
	data *config.MediaDataConfig,
) bool {
	if data == nil || expectedMs <= 0 {
		return false
	}

	perTrackMs := int64(3000)
	if data.PerTrackToleranceSeconds > 0 {
		perTrackMs = int64(data.PerTrackToleranceSeconds) * 1000
	}

	toleranceMs := int64(fileCount) * perTrackMs
	if data.MaxTotalDifferenceSeconds > 0 {
		toleranceMs = int64(data.MaxTotalDifferenceSeconds) * 1000
	}

	diff := localMs - expectedMs
	if diff < 0 {
		diff = -diff
	}

	return diff <= toleranceMs
}

// ── Music search-pair builder ─────────────────────────────────────────────────

// buildMusicSearchPairs returns all unique artist+album combinations to try
// against the database, derived from the parsed folder metadata.
func buildMusicSearchPairs(meta folderMetadata) []musicSearchPair {
	pairs := make([]musicSearchPair, 0, 32)
	seen := make(map[string]bool, 32)

	add := func(artist, album, source string) {
		if artist == "" || album == "" {
			return
		}

		key := strings.ToLower(artist) + "|" + strings.ToLower(album)
		if seen[key] {
			return
		}

		seen[key] = true

		pairs = append(pairs, musicSearchPair{artist, album, source})
	}

	add(meta.FolderArtist, meta.FolderAlbum, "folder")
	add(meta.FolderArtist, meta.FilenameAlbum, "folder+filename")
	add(meta.FilenameArtist, meta.FilenameAlbum, "filename")
	add(meta.FilenameArtist, meta.FolderAlbum, "filename+folder")
	add(meta.RawArtist, meta.RawAlbum, "raw-folder")
	add(meta.RawAlbum, meta.RawArtist, "raw-folder-swapped")
	add(meta.ParentArtist, meta.FolderAlbum, "parent+folder")
	add(meta.ParentArtist, meta.FilenameAlbum, "parent+filename")
	add(meta.ParentArtist, meta.TagAlbum, "parent+tag")
	add(meta.ParentArtist, meta.ParentAlbum, "parent")

	if meta.IsDiscFolder {
		add(meta.GrandparentArtist, meta.ParentAlbum, "grandparent+parent")
	}

	add(meta.FolderArtist, meta.ParentAlbum, "folder+parent")
	add(meta.FilenameArtist, meta.ParentAlbum, "filename+parent")
	add(meta.TagAlbumArtist, meta.TagAlbum, "tag-albumartist")
	add(meta.TagArtist, meta.TagAlbum, "tag-artist")
	add(meta.TagAlbumArtist, meta.FolderAlbum, "tag-albumartist+folder")
	add(meta.TagAlbumArtist, meta.FilenameAlbum, "tag-albumartist+filename")
	add(meta.TagAlbumArtist, meta.ParentAlbum, "tag-albumartist+parent")
	add(meta.TagArtist, meta.FolderAlbum, "tag-artist+folder")
	add(meta.TagArtist, meta.FilenameAlbum, "tag-artist+filename")
	add(meta.TagArtist, meta.ParentAlbum, "tag-artist+parent")

	// Expand common VA abbreviations.
	if strings.EqualFold(meta.FolderArtist, "VA") ||
		strings.EqualFold(meta.FolderArtist, "V.A.") {
		add(VariousArtistsName, meta.FolderAlbum, "va-expanded+folder")
		add(VariousArtistsName, meta.FilenameAlbum, "va-expanded+filename")
		add(VariousArtistsName, meta.TagAlbum, "va-expanded+tag")
		add(VariousArtistsName, meta.ParentAlbum, "va-expanded+parent")
	}

	// Stripped episode-number variants.
	for _, title := range []string{
		meta.FolderAlbum, meta.FilenameAlbum, meta.TagAlbum, meta.ParentAlbum,
	} {
		stripped := stripLeadingEpisodeNumber(title)
		if stripped == "" {
			continue
		}

		add(meta.FolderArtist, stripped, "stripped-ep+folder")
		add(meta.FilenameArtist, stripped, "stripped-ep+filename")
		add(meta.TagAlbumArtist, stripped, "stripped-ep+tag")
		add(meta.TagArtist, stripped, "stripped-ep+tag-artist")
		add(meta.ParentArtist, stripped, "stripped-ep+parent")

		if idx := strings.Index(stripped, " - "); idx > 0 {
			potArtist := strings.TrimSpace(stripped[:idx])
			potAlbum := strings.TrimSpace(stripped[idx+3:])
			add(potArtist, potAlbum, "stripped-ep-reparsed")
			add(meta.TagArtist, potAlbum, "tag-artist+stripped-ep-reparsed")
			add(meta.TagAlbumArtist, potAlbum, "tag-albumartist+stripped-ep-reparsed")
		}
	}

	return pairs
}

// ── Audiobook search-pair builder ─────────────────────────────────────────────

// buildAudiobookSearchPairs returns all unique author+title combinations to try
// against the database, derived from the parsed folder metadata.
// In addition to the base combinations used by music, it also generates
// series-prefix and series-suffix stripped variants.
func buildAudiobookSearchPairs(meta folderMetadata) []searchPair {
	pairs := make([]searchPair, 0, 48)
	seen := make(map[string]bool, 48)

	add := func(author, title, source string) {
		if author == "" || title == "" {
			return
		}

		key := strings.ToLower(author) + "|" + strings.ToLower(title)
		if seen[key] {
			return
		}

		seen[key] = true

		pairs = append(pairs, searchPair{author: author, title: title, source: source})
	}

	add(meta.FolderArtist, meta.FolderAlbum, "folder")
	add(meta.FolderArtist, meta.FilenameAlbum, "folder+filename")
	add(meta.FilenameArtist, meta.FilenameAlbum, "filename")
	add(meta.FilenameArtist, meta.FolderAlbum, "filename+folder")
	add(meta.RawArtist, meta.RawAlbum, "raw-folder")
	add(meta.RawAlbum, meta.RawArtist, "raw-folder-swapped")
	add(meta.ParentArtist, meta.FolderAlbum, "parent+folder")
	add(meta.ParentArtist, meta.FilenameAlbum, "parent+filename")
	add(meta.ParentArtist, meta.TagAlbum, "parent+tag")
	add(meta.ParentArtist, meta.ParentAlbum, "parent")

	if meta.IsDiscFolder {
		add(meta.GrandparentArtist, meta.ParentAlbum, "grandparent+parent")
	}

	add(meta.FolderArtist, meta.ParentAlbum, "folder+parent")
	add(meta.FilenameArtist, meta.ParentAlbum, "filename+parent")
	add(meta.TagAlbumArtist, meta.TagAlbum, "tag-albumartist")
	add(meta.TagArtist, meta.TagAlbum, "tag-artist")
	add(meta.TagAlbumArtist, meta.FolderAlbum, "tag-albumartist+folder")
	add(meta.TagAlbumArtist, meta.FilenameAlbum, "tag-albumartist+filename")
	add(meta.TagAlbumArtist, meta.ParentAlbum, "tag-albumartist+parent")
	add(meta.TagArtist, meta.FolderAlbum, "tag-artist+folder")
	add(meta.TagArtist, meta.FilenameAlbum, "tag-artist+filename")
	add(meta.TagArtist, meta.ParentAlbum, "tag-artist+parent")

	// Episode-number prefix variants.
	for _, title := range []string{
		meta.FolderAlbum, meta.FilenameAlbum, meta.TagAlbum, meta.ParentAlbum,
	} {
		stripped := stripLeadingEpisodeNumber(title)
		if stripped != "" {
			add(meta.FolderArtist, stripped, "stripped-ep+folder")
			add(meta.FilenameArtist, stripped, "stripped-ep+filename")
			add(meta.TagAlbumArtist, stripped, "stripped-ep+tag")
			add(meta.TagArtist, stripped, "stripped-ep+tag-artist")
			add(meta.ParentArtist, stripped, "stripped-ep+parent")

			if idx := strings.Index(stripped, " - "); idx > 0 {
				potAuthor := strings.TrimSpace(stripped[:idx])
				potTitle := strings.TrimSpace(stripped[idx+3:])
				add(potAuthor, potTitle, "stripped-ep-reparsed")
				add(meta.TagArtist, potTitle, "tag-artist+stripped-ep-reparsed")
				add(meta.TagAlbumArtist, potTitle, "tag-albumartist+stripped-ep-reparsed")
			}
		}
	}

	// Series-keyword prefix and suffix variants (audiobook-specific).
	for _, title := range []string{
		meta.FolderAlbum, meta.FilenameAlbum, meta.TagAlbum, meta.ParentAlbum,
	} {
		if stripped := stripSeriesPrefix(title); stripped != "" {
			add(meta.FolderArtist, stripped, "stripped-prefix+folder")
			add(meta.FilenameArtist, stripped, "stripped-prefix+filename")
			add(meta.TagAlbumArtist, stripped, "stripped-prefix+tag")
			add(meta.TagArtist, stripped, "stripped-prefix+tag-artist")
			add(meta.ParentArtist, stripped, "stripped-prefix+parent")
		}

		if stripped := stripSeriesSuffix(title); stripped != "" {
			add(meta.FolderArtist, stripped, "stripped-series+folder")
			add(meta.FilenameArtist, stripped, "stripped-series+filename")
			add(meta.TagAlbumArtist, stripped, "stripped-series+tag")
			add(meta.TagArtist, stripped, "stripped-series+tag-artist")
			add(meta.ParentArtist, stripped, "stripped-series+parent")
		}
	}

	return pairs
}

// ── Music candidate matching ──────────────────────────────────────────────────

// relaxedAlbumThresh is the wider album-distance cap used in the second
// candidate pass when the strict pass found no valid matches.
const relaxedAlbumThresh = 0.6

// runMusicCandidateMatchingLoop runs a single pass over preScored candidates
// within (skipBelow, maxDist].  Candidates with dist ≤ skipBelow are skipped
// (already handled by a prior pass); candidates with dist > maxDist stop the
// loop (list is sorted ascending).
//
// withTierTwo enables the ExceedToleranceIfTotalMatch total-runtime fallback
// when per-track matching fails (only for the strict first pass).
func runMusicCandidateMatchingLoop(
	ctx context.Context,
	preScored []preScoredC,
	albumTracks []parser_v2.TrackInfo,
	localTotalMs int64,
	isVA bool,
	albumTitle, artist, musicBrainzID string,
	year, fileCount int,
	data *config.MediaDataConfig,
	skipBelow, maxDist float64,
	withTierTwo bool,
) []musicValidMatch {
	var validMatches []musicValidMatch
	for i := range preScored {
		if preScored[i].dist <= skipBelow {
			continue
		}

		if preScored[i].dist > maxDist {
			break
		}

		dbTracks := resolveTracksForMatching(
			ctx,
			preScored[i].c.ID,
			preScored[i].c.MusicBrainzReleaseID,
			"",
			"",
			preScored[i].c.Title,
			preScored[i].c.Artist,
		)
		if len(dbTracks) == 0 {
			continue
		}

		result, matched, used := matchTracksByDistance(albumTracks, dbTracks, isVA, false, data)
		unmatchedDB, unusedLocal := countUnmatched(matched, used)

		if (unmatchedDB > 0 || unusedLocal > 0) && (data == nil || !data.AllowMissingTracks) {
			if !withTierTwo ||
				!tierTwoRuntimeAccepted(preScored[i].c, dbTracks, localTotalMs, fileCount, data) {
				continue
			}
		}

		matchedTracks := applyTrackDBOverrides(result, matched, dbTracks, isVA, false, data)
		fullDist := albumDistanceWithTracks(
			preScored[i].c, artist, albumTitle, musicBrainzID, year,
			albumTracks, dbTracks, isVA, data,
		)

		validMatches = append(
			validMatches,
			musicValidMatch{preScored[i].c, fullDist, matchedTracks},
		)
	}

	return validMatches
}

// tierTwoRuntimeAccepted returns true when ExceedToleranceIfTotalMatch is set
// and the total album runtime difference is within tolerance.
func tierTwoRuntimeAccepted(
	c *database.AlbumSearchResult,
	dbTracks []database.DbtrackWithArtist,
	localTotalMs int64,
	fileCount int,
	data *config.MediaDataConfig,
) bool {
	if data == nil || !data.ExceedToleranceIfTotalMatch {
		return false
	}

	expectedMs := int64(c.TotalRuntime)
	if expectedMs == 0 {
		for i := range dbTracks {
			expectedMs += dbTracks[i].RuntimeMs
		}
	}

	return runtimeDiffWithinTolerance(localTotalMs, expectedMs, fileCount, data)
}

// runMusicCandidatePasses runs the two-pass candidate matching strategy:
//
//   - Pass 1 (strict):  albumDist ≤ mediumRecThresh with tier-2 fallback.
//   - Pass 2 (relaxed): only when pass 1 yields nothing; wider dist cap (0.6).
//
// Returns valid matches sorted by fullDist ascending.
func runMusicCandidatePasses(
	ctx context.Context,
	preScored []preScoredC,
	albumTracks []parser_v2.TrackInfo,
	localTotalMs int64,
	isVA bool,
	albumTitle, artist, musicBrainzID string,
	year, fileCount int,
	data *config.MediaDataConfig,
) []musicValidMatch {
	validMatches := runMusicCandidateMatchingLoop(
		ctx, preScored, albumTracks, localTotalMs, isVA,
		albumTitle, artist, musicBrainzID, year, fileCount, data,
		-1, mediumRecThresh, true,
	)
	if len(validMatches) > 0 {
		sort.SliceStable(validMatches, func(i, j int) bool {
			return validMatches[i].fullDist < validMatches[j].fullDist
		})
		return validMatches
	}

	validMatches = runMusicCandidateMatchingLoop(
		ctx, preScored, albumTracks, localTotalMs, isVA,
		albumTitle, artist, musicBrainzID, year, fileCount, data,
		mediumRecThresh, relaxedAlbumThresh, false,
	)
	sort.SliceStable(validMatches, func(i, j int) bool {
		return validMatches[i].fullDist < validMatches[j].fullDist
	})

	return validMatches
}

// ── Audiobook candidate matching ──────────────────────────────────────────────

// runAudiobookCandidatePass runs the candidate matching loop for audiobooks.
// Candidates with albumDist > mediumRecThresh are skipped.
// Returns valid matches sorted by fullDist ascending.
func runAudiobookCandidatePass(
	ctx context.Context,
	bestCandidates []*database.AudiobookSearchResult,
	albumTracks []parser_v2.TrackInfo,
	isVA bool,
	albumTitle, artist string,
	fileCount int,
	data *config.MediaDataConfig,
	cfgp *config.MediaTypeConfig,
) []audiobookValidMatch {
	var validMatches []audiobookValidMatch
	for i := range bestCandidates {
		if audiobookMatchDistance(
			bestCandidates[i],
			albumTitle,
			artist,
			fileCount,
		) > mediumRecThresh {
			continue
		}

		dbTracks := resolveTracksForMatching(
			ctx,
			bestCandidates[i].ID,
			"",
			bestCandidates[i].ASIN,
			cfgp.AudibleRegion,
			bestCandidates[i].Title,
			bestCandidates[i].Author,
		)
		if len(dbTracks) == 0 {
			continue
		}

		result, matched, used := matchTracksByDistance(albumTracks, dbTracks, isVA, true, data)

		unmatchedDB, unusedLocal := countUnmatched(matched, used)
		if (unmatchedDB > 0 || unusedLocal > 0) && (data == nil || !data.AllowMissingTracks) {
			continue
		}

		matchedTracks := applyTrackDBOverrides(result, matched, dbTracks, isVA, true, data)
		fullDist := audiobookDistanceWithTracks(
			bestCandidates[i], albumTitle, artist, albumTracks, dbTracks, data,
		)

		validMatches = append(
			validMatches,
			audiobookValidMatch{bestCandidates[i], fullDist, matchedTracks},
		)
	}

	sort.SliceStable(validMatches, func(i, j int) bool {
		return validMatches[i].fullDist < validMatches[j].fullDist
	})

	return validMatches
}

// selectAudiobookCandidatesWithRefresh calls selectBestAudiobookMatches and, if no
// chapter-count match is found, refreshes missing chapter counts from Audnex and retries.
// Returns (bestCandidates, initial dbMatch).
func selectAudiobookCandidatesWithRefresh(
	ctx context.Context,
	allMatches []*database.AudiobookSearchResult,
	fileCount int,
	data *config.MediaDataConfig,
	albumTitle, artist string,
	cfgp *config.MediaTypeConfig,
) ([]*database.AudiobookSearchResult, *database.AudiobookSearchResult) {
	if len(allMatches) == 0 {
		return nil, nil
	}

	bestCandidates := selectBestAudiobookMatches(allMatches, fileCount, data, albumTitle, artist)
	if len(bestCandidates) > 0 {
		return bestCandidates, bestCandidates[0]
	}

	// No exact chapter count match. Check if any candidate has ChapterCount=0
	// (missing data) and try to refresh from Audnex.
	audnexProvider := providers.GetAudnex()

	refreshed := false

	var chaptercount int
	for i := range allMatches {
		if allMatches[i].ChapterCount != 0 || allMatches[i].ASIN == "" || audnexProvider == nil {
			continue
		}

		audnexChapters, err := audnexProvider.GetChaptersByASIN(
			ctx,
			allMatches[i].ASIN,
			cfgp.AudibleRegion,
		)
		if err != nil || len(audnexChapters) == 0 {
			continue
		}

		allMatches[i].ChapterCount = len(audnexChapters)
		refreshed = true
		chaptercount = len(audnexChapters)
		database.ExecN(
			"UPDATE dbaudiobooks SET chapter_count = ? WHERE id = ?",
			&chaptercount, &allMatches[i].ID,
		)
	}

	if refreshed {
		bestCandidates = selectBestAudiobookMatches(allMatches, fileCount, data, albumTitle, artist)
		if len(bestCandidates) > 0 {
			return bestCandidates, bestCandidates[0]
		}
	}

	return nil, nil
}

// pickSeriesAudiobookFallback picks the first series audiobook from allMatches whose
// runtime is ≤ 60 minutes when SkipSeriesTrackMatch is enabled.
func pickSeriesAudiobookFallback(
	allMatches []*database.AudiobookSearchResult,
	data *config.MediaDataConfig,
	fileCount int,
	folder string,
) (*database.AudiobookSearchResult, bool) {
	if data == nil || !data.SkipSeriesTrackMatch {
		return nil, false
	}

	for i := range allMatches {
		if allMatches[i].Series != "" && allMatches[i].Runtime > 0 && allMatches[i].Runtime <= 60 {
			logger.Logtype("info", 1).
				Str("folder", folder).
				Uint("id", allMatches[i].ID).
				Str("title", allMatches[i].Title).
				Str("series", allMatches[i].Series).
				Str("seriesNum", allMatches[i].SeriesNum).
				Int("dbChapters", allMatches[i].ChapterCount).
				Int("localFiles", fileCount).
				Int("runtimeMin", allMatches[i].Runtime).
				Msg("Skipped chapter count match for series audiobook")

			return allMatches[i], true
		}
	}

	return nil, false
}

// audnexAddFoundPreFlight verifies chapter count via Audnex before an addFound import.
// Returns true when import should proceed.
func audnexAddFoundPreFlight(
	ctx context.Context,
	asin string,
	trackCount int,
	tracks []parser_v2.TrackInfo,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	folder string,
) bool {
	audnexProvider := providers.GetAudnex()
	if audnexProvider == nil {
		return true
	}

	chapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, cfgp.AudibleRegion)
	if err != nil || len(chapters) == 0 {
		return true
	}

	if trackCount < len(chapters) {
		if data.AllowMissingTracks && data.ExceedToleranceIfTotalMatch {
			var audnexTotalMs int64
			for i := range chapters {
				audnexTotalMs += chapters[i].LengthMs
			}

			var localTotalMs int64
			for i := range tracks {
				localTotalMs += tracks[i].RuntimeMS
			}

			totalToleranceMs := int64(trackCount) * 3000
			if data.PerTrackToleranceSeconds > 0 {
				totalToleranceMs = int64(trackCount) * int64(data.PerTrackToleranceSeconds) * 1000
			}

			if data.MaxTotalDifferenceSeconds > 0 {
				totalToleranceMs = int64(data.MaxTotalDifferenceSeconds) * 1000
			}

			diff := localTotalMs - audnexTotalMs
			if diff < 0 {
				diff = -diff
			}

			proceed := diff <= totalToleranceMs
			logger.Logtype("debug", 0).
				Str("folder", folder).
				Int("localTracks", trackCount).
				Int("audnexChapters", len(chapters)).
				Int64("localTotalMs", localTotalMs).
				Int64("audnexTotalMs", audnexTotalMs).
				Int64("diffMs", diff).
				Int64("toleranceMs", totalToleranceMs).
				Bool("proceed", proceed).
				Msg("Fewer local tracks than Audnex chapters - total runtime check")

			if !proceed {
				return false
			}
		} else {
			logger.Logtype("debug", 0).
				Str("folder", folder).
				Int("localTracks", trackCount).
				Int("audnexChapters", len(chapters)).
				Msg("Fewer local tracks than Audnex chapters - skipping addFound import")

			return false
		}
	} else if trackCount > len(chapters) {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Int("localTracks", trackCount).
			Int("audnexChapters", len(chapters)).
			Msg("More local tracks than Audnex chapters - split files, proceeding with import")
	}

	return true
}

// importAudiobookWithAddFound tries to import the audiobook via addFound (either by
// replacing an existing alternative release or importing fresh from Audible).
// Returns the new database ID, expected chapter count, updated candidates slice, and
// the imported entry (nil when nothing was imported).
func importAudiobookWithAddFound(
	ctx context.Context,
	asin string,
	allMatches []*database.AudiobookSearchResult,
	bestCandidates []*database.AudiobookSearchResult,
	cfgp *config.MediaTypeConfig,
	listid int,
	data *config.MediaDataConfig,
	folder string,
) (dbID uint, expectedTracks int, newBestCandidates []*database.AudiobookSearchResult, addFoundEntry *database.AudiobookSearchResult) {
	newBestCandidates = bestCandidates

	if data.AllowAlternativeReleases && len(allMatches) > 0 {
		for i := range len(allMatches) {
			if allMatches[i].ASIN == asin {
				continue
			}

			candidateID := allMatches[i].ID

			fc := database.Getdatarow[int](
				false,
				"SELECT COUNT(*) FROM audiobook_files WHERE dbaudiobook_id = ?",
				&candidateID,
			)
			if fc > 0 {
				continue
			}

			replacedID, ok := replaceExistingAudiobookRelease(
				ctx,
				allMatches[i].ID,
				asin,
				cfgp,
				listid,
			)
			if ok {
				if imported, _ := database.FindAudiobookByASIN(asin); imported != nil {
					newBestCandidates = append(newBestCandidates, imported)
					addFoundEntry = imported
					expectedTracks = imported.ChapterCount
					dbID = replacedID

					logger.Logtype("info", 1).
						Str("folder", folder).
						Uint("replacedID", allMatches[i].ID).
						Str("oldASIN", allMatches[i].ASIN).
						Str("newASIN", asin).
						Uint("databaseID", replacedID).
						Msg("Replaced existing audiobook with alternative edition")
				}

				return dbID, expectedTracks, newBestCandidates, addFoundEntry
			}
		}
	}

	importedID, importErr := JobImportAudiobooks(ctx, asin, cfgp, listid, true)
	if importErr == nil && importedID != 0 {
		if imported, _ := database.FindAudiobookByASIN(asin); imported != nil {
			newBestCandidates = append(newBestCandidates, imported)
			addFoundEntry = imported
			expectedTracks = imported.ChapterCount
			dbID = importedID

			logger.Logtype("debug", 0).
				Str("folder", folder).
				Uint("databaseID", importedID).
				Msg("DEBUG: Successfully imported audiobook")
		}
	}

	return dbID, expectedTracks, newBestCandidates, addFoundEntry
}

// applyAudiobookRecommendation applies the beets recommendation logic to the sorted
// valid matches.  Returns the winning database ID, chapter count, matched tracks, and
// whether a match was accepted.
func applyAudiobookRecommendation(
	validABMatches []audiobookValidMatch,
) (dbID uint, expectedTracks int, matchedTracks []parser_v2.TrackInfo, sortSuccess bool) {
	if len(validABMatches) == 0 {
		return
	}

	fullDists := make([]float64, len(validABMatches))
	for i, vm := range validABMatches {
		fullDists[i] = vm.fullDist
	}

	if recommendation(fullDists) >= recMedium {
		best := validABMatches[0]

		dbID = best.c.ID
		expectedTracks = best.c.ChapterCount
		matchedTracks = best.matchedTracks
		sortSuccess = true
	}

	return
}

// buildAudiobookFailureReport constructs a MatchReport for the "wrong_runtime" denial
// path, including per-candidate track details for the top bestCandidates.
func buildAudiobookFailureReport(
	ctx context.Context,
	bestCandidates []*database.AudiobookSearchResult,
	validABMatches []audiobookValidMatch,
	albumTracks []parser_v2.TrackInfo,
	albumTitle, artist string,
	actualRuntimeMS int64,
	isVA bool,
	data *config.MediaDataConfig,
	cfgp *config.MediaTypeConfig,
) *MatchReport {
	report := &MatchReport{
		DenialReason:    "wrong_runtime",
		ActualTracks:    len(albumTracks),
		ActualRuntimeMS: actualRuntimeMS,
	}

	vmByID := make(map[uint]audiobookValidMatch, len(validABMatches))
	for i := range validABMatches {
		vmByID[validABMatches[i].c.ID] = validABMatches[i]
	}

	limit := min(len(bestCandidates), 3)
	for i := range limit {
		c := bestCandidates[i]
		cr := CandidateReport{
			Title:             c.Title,
			Artist:            c.Author,
			MBID:              c.ASIN,
			ExpectedTracks:    c.ChapterCount,
			ExpectedRuntimeMS: c.Runtime * 60000,
			AlbumDist:         audiobookMatchDistance(c, albumTitle, artist, len(albumTracks)),
		}

		if vm, ok := vmByID[c.ID]; ok {
			cr.FullDist = vm.fullDist
			for j := range vm.matchedTracks {
				cr.Tracks = append(cr.Tracks, CandidateTrackReport{
					Title:          vm.matchedTracks[j].Title,
					TrackNumber:    vm.matchedTracks[j].TrackNumber,
					DiscNumber:     vm.matchedTracks[j].DiscNumber,
					DBRuntimeMS:    vm.matchedTracks[j].ExpectedRuntimeMS,
					LocalRuntimeMS: vm.matchedTracks[j].RuntimeMS,
					TrackDist:      vm.matchedTracks[j].TrackDist,
				})
			}
		} else {
			rptTracks := resolveTracksForMatching(
				ctx,
				c.ID,
				"",
				c.ASIN,
				cfgp.AudibleRegion,
				c.Title,
				c.Author,
			)
			if len(rptTracks) > 0 {
				rptResult, rptMatched, _ := matchTracksByDistance(
					albumTracks,
					rptTracks,
					isVA,
					true,
					data,
				)
				for j := range rptResult {
					if !rptMatched[j] {
						continue
					}

					if rptTracks[j].TrackNumber > 0 {
						rptResult[j].TrackNumber = int(rptTracks[j].TrackNumber)
					}

					if rptTracks[j].DiscNumber > 0 {
						rptResult[j].DiscNumber = int(rptTracks[j].DiscNumber)
					}

					if rptResult[j].Title == "" && rptTracks[j].Title != "" {
						rptResult[j].Title = rptTracks[j].Title
					}

					if rptTracks[j].RuntimeMs > 0 {
						rptResult[j].ExpectedRuntimeMS = rptTracks[j].RuntimeMs
					}

					cr.Tracks = append(cr.Tracks, CandidateTrackReport{
						Title:          rptResult[j].Title,
						TrackNumber:    rptResult[j].TrackNumber,
						DiscNumber:     rptResult[j].DiscNumber,
						DBRuntimeMS:    rptResult[j].ExpectedRuntimeMS,
						LocalRuntimeMS: rptResult[j].RuntimeMS,
						TrackDist: trackDistance(
							&rptResult[j],
							&rptTracks[j],
							isVA,
							true,
							data,
						),
					})
				}
			}
		}

		if cr.ExpectedRuntimeMS == 0 {
			for j := range cr.Tracks {
				cr.ExpectedRuntimeMS += int(cr.Tracks[j].DBRuntimeMS)
			}
		}

		report.Candidates = append(report.Candidates, cr)
	}

	return report
}

// verifyAndImportMusicBrainzAddFound fetches the MusicBrainz release, verifies track
// count and lengths against local files, then imports it via AddFound.
// Returns the new database ID, expected track count, updated candidates, and imported
// entry; dbID == 0 when nothing was imported.
func verifyAndImportMusicBrainzAddFound(
	ctx context.Context,
	musicBrainzID *string,
	albumTracks []parser_v2.TrackInfo,
	bestCandidates []*database.AlbumSearchResult,
	artist string,
	fileCount int,
	cfgp *config.MediaTypeConfig,
	listid int,
	data *config.MediaDataConfig,
	folder string,
) (dbID uint, expectedTracks int, newBestCandidates []*database.AlbumSearchResult, addFoundEntry *database.AlbumSearchResult) {
	newBestCandidates = bestCandidates

	isVAEarly := IsVariousArtists(artist)

	shouldImport := false
	if mbProvider := providers.GetMusicBrainz(); mbProvider != nil {
		release, err := retryOnRateLimit(
			ctx,
			func() (*apiexternal_v2.ReleaseDetails, error) {
				return mbProvider.GetReleaseByID(ctx, *musicBrainzID)
			},
		)
		if err == nil && release != nil && release.TrackCount > 0 {
			if release.TrackCount != fileCount {
				logger.Logtype("debug", 0).
					Str("folder", folder).
					Int("localTracks", fileCount).
					Int("mbTracks", release.TrackCount).
					Msg("Track count mismatch - skipping addFound import")
			} else {
				mbDbTracks := mbTracksToDbTracks(release.Tracks)
				_, matched, used := matchTracksByDistance(
					albumTracks,
					mbDbTracks,
					isVAEarly,
					false,
					data,
				)

				unmatchedDB, unusedLocal := countUnmatched(matched, used)
				if unmatchedDB == 0 && unusedLocal == 0 {
					shouldImport = true
				} else {
					logger.Logtype("debug", 0).
						Str("folder", folder).
						Int("unmatchedDB", unmatchedDB).
						Int("unusedLocal", unusedLocal).
						Msg("Track length mismatch against MusicBrainz - skipping addFound import")
				}
			}
		}
	}

	if !shouldImport {
		return dbID, expectedTracks, newBestCandidates, addFoundEntry
	}

	importedID, importErr := JobImportAlbums(ctx, *musicBrainzID, cfgp, listid, true)
	if importErr != nil || importedID == 0 {
		return dbID, expectedTracks, newBestCandidates, addFoundEntry
	}

	imported, _ := database.FindAlbumByMusicBrainzID(musicBrainzID)
	if imported == nil {
		return dbID, expectedTracks, newBestCandidates, addFoundEntry
	}

	dbID = importedID
	expectedTracks = imported.TotalTracks
	newBestCandidates = append([]*database.AlbumSearchResult{imported}, bestCandidates...)
	addFoundEntry = imported

	logger.Logtype("info", 1).
		Str("folder", folder).
		Str("musicBrainzID", *musicBrainzID).
		Uint("databaseID", importedID).
		Msg("AddFound: imported and verified album via MusicBrainzID")

	return dbID, expectedTracks, newBestCandidates, addFoundEntry
}

// importMusicAddFoundTextSearch searches MusicBrainz by artist+title text and imports
// the best matching release via AddFound.
// Returns the new database ID, expected track count, updated candidates, and imported
// entry; dbID == 0 when nothing was found/imported.
func importMusicAddFoundTextSearch(
	ctx context.Context,
	albumTitle, tagAlbum string,
	artist *string,
	bestCandidates []*database.AlbumSearchResult,
	totalRuntimeMs int64,
	fileCount *int,
	cfgp *config.MediaTypeConfig,
	listid int,
	data *config.MediaDataConfig,
	folder string,
) (dbID uint, expectedTracks int, newBestCandidates []*database.AlbumSearchResult, addFoundEntry *database.AlbumSearchResult) {
	newBestCandidates = bestCandidates

	searchTitle := albumTitle
	if tagAlbum != "" {
		searchTitle = tagAlbum
	} else if stripped := stripLeadingEpisodeNumber(albumTitle); stripped != "" {
		searchTitle = stripped
	}

	importedID, found := searchAndImportAlternativeRelease(
		ctx, artist, &searchTitle, fileCount, totalRuntimeMs, cfgp, listid, nil, data,
	)
	if !found || importedID == 0 {
		return dbID, expectedTracks, newBestCandidates, addFoundEntry
	}

	dbID = importedID

	var newTrackCount int
	database.Scanrowsdyn(
		false,
		"SELECT COUNT(*) FROM dbtracks WHERE dbalbum_id = ?",
		&newTrackCount,
		&importedID,
	)

	expectedTracks = newTrackCount

	var ab database.Dbalbum
	if err := ab.GetDbalbumByIDP(&importedID); err == nil {
		imported := &database.AlbumSearchResult{
			ID:                   importedID,
			Title:                ab.Title,
			TotalTracks:          ab.TotalTracks,
			TotalRuntime:         int(ab.TotalRuntimeMs),
			Year:                 int(ab.Year),
			MusicBrainzReleaseID: ab.MusicbrainzReleaseID,
			Label:                ab.Label,
			Country:              ab.Country,
		}

		newBestCandidates = append([]*database.AlbumSearchResult{imported}, bestCandidates...)
		addFoundEntry = imported
	}

	logger.Logtype("info", 1).
		Str("folder", folder).
		Str("artist", *artist).
		Str("album", albumTitle).
		Uint("databaseID", importedID).
		Msg("AddFound: imported album via text search (no MBID)")

	return dbID, expectedTracks, newBestCandidates, addFoundEntry
}

// applyMusicRecommendation applies the beets recommendation logic (and tier-2 runtime
// tolerance) to the sorted valid matches.  Returns the winning ID, track count, matched
// tracks, and whether a match was accepted.
func applyMusicRecommendation(
	validMatches []musicValidMatch,
	localTotalMs int64,
	fileCount int,
	data *config.MediaDataConfig,
) (dbID uint, expectedTracks int, matchedTracks []parser_v2.TrackInfo, sortSuccess bool) {
	if len(validMatches) == 0 {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	fullDists := make([]float64, len(validMatches))
	for i, vm := range validMatches {
		fullDists[i] = vm.fullDist
	}

	if recommendation(fullDists) >= recMedium {
		best := validMatches[0]

		dbID = best.c.ID
		expectedTracks = best.c.TotalTracks
		matchedTracks = best.matchedTracks
		sortSuccess = true

		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	if data == nil || !data.ExceedToleranceIfTotalMatch {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	best := validMatches[0]

	expectedBestRuntime := int64(best.c.TotalRuntime)
	if expectedBestRuntime == 0 {
		for i := range best.matchedTracks {
			expectedBestRuntime += best.matchedTracks[i].ExpectedRuntimeMS
		}
	}

	if expectedBestRuntime == 0 {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	perTrackMs := int64(3000)
	if data.PerTrackToleranceSeconds > 0 {
		perTrackMs = int64(data.PerTrackToleranceSeconds) * 1000
	}

	totalToleranceMs := int64(fileCount) * perTrackMs
	if data.MaxTotalDifferenceSeconds > 0 {
		totalToleranceMs = int64(data.MaxTotalDifferenceSeconds) * 1000
	}

	diff := localTotalMs - expectedBestRuntime
	if diff < 0 {
		diff = -diff
	}

	if diff <= totalToleranceMs {
		dbID = best.c.ID
		expectedTracks = best.c.TotalTracks
		matchedTracks = best.matchedTracks
		sortSuccess = true
	}

	return dbID, expectedTracks, matchedTracks, sortSuccess
}

// tryMusicAlternativeRelease searches MusicBrainz for an alternative release when no
// candidate passed distance matching.  Returns the winning database ID, expected track
// count, matched tracks, and whether a match was accepted.
func tryMusicAlternativeRelease(
	ctx context.Context,
	albumTracks []parser_v2.TrackInfo,
	bestCandidates []*database.AlbumSearchResult,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	isVA bool,
	cfgp *config.MediaTypeConfig,
	listid int,
	data *config.MediaDataConfig,
	folder string,
) (dbID uint, expectedTracks int, matchedTracks []parser_v2.TrackInfo, sortSuccess bool) {
	if data == nil || (!data.AddFound && !data.AllowAlternativeReleases) {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	// AddFound requires a valid list; AllowAlternativeReleases can proceed with
	// listid=-1 because JobImportAlbums resolves the list from the existing DB entry.
	if data.AddFound && !data.AllowAlternativeReleases && listid == -1 {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	importedID, found := searchAndImportAlternativeRelease(
		ctx, artist, albumTitle, fileCount, localTotalMs, cfgp, listid, bestCandidates, data,
	)
	if !found || importedID == 0 {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	var newTrackCount int
	database.Scanrowsdyn(
		false,
		"SELECT COUNT(*) FROM dbtracks WHERE dbalbum_id = ?",
		&newTrackCount,
		&importedID,
	)

	var altMBID string
	database.Scanrowsdyn(
		false,
		"SELECT musicbrainz_release_id FROM dbalbums WHERE id = ?",
		&altMBID,
		&importedID,
	)

	altDbTracks := resolveTracksForMatching(ctx, importedID, altMBID, "", "", *albumTitle, *artist)
	if len(altDbTracks) == 0 {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	altResult, altMatched, altUsed := matchTracksByDistance(
		albumTracks,
		altDbTracks,
		isVA,
		false,
		data,
	)

	altUnmatchedDB, altUnusedLocal := countUnmatched(altMatched, altUsed)
	if (altUnmatchedDB > 0 || altUnusedLocal > 0) && !data.AllowMissingTracks {
		// Mirror runMusicCandidateMatchingLoop: try tier-2 runtime fallback before giving up.
		altCandidate := &database.AlbumSearchResult{ID: importedID}
		if !tierTwoRuntimeAccepted(altCandidate, altDbTracks, localTotalMs, *fileCount, data) {
			return dbID, expectedTracks, matchedTracks, sortSuccess
		}
	}

	// Mirror runMusicCandidateMatchingLoop: gate on fullDist recommendation.
	altCandidate := &database.AlbumSearchResult{
		ID:          importedID,
		TotalTracks: newTrackCount,
	}

	fullDist := albumDistanceWithTracks(
		altCandidate, *artist, *albumTitle, altMBID, 0,
		albumTracks, altDbTracks, isVA, data,
	)
	if recommendation([]float64{fullDist}) < recMedium {
		return dbID, expectedTracks, matchedTracks, sortSuccess
	}

	altMatchedTracks := applyTrackDBOverrides(altResult, altMatched, altDbTracks, isVA, false, data)

	logger.Logtype("info", 1).
		Str("folder", folder).
		Uint("databaseID", importedID).
		Float64("fullDist", fullDist).
		Msg("Successfully matched to alternative MusicBrainz release")

	dbID = importedID
	expectedTracks = newTrackCount
	matchedTracks = altMatchedTracks
	sortSuccess = true

	return dbID, expectedTracks, matchedTracks, sortSuccess
}

// buildMusicFailureReport constructs a MatchReport for the "wrong_runtime" denial path,
// including per-candidate track details for the top pre-scored candidates.
func buildMusicFailureReport(
	ctx context.Context,
	preScored []preScoredC,
	bestCandidates []*database.AlbumSearchResult,
	albumTracks []parser_v2.TrackInfo,
	actualRuntimeMS int64,
	isVA bool,
	data *config.MediaDataConfig,
	folder string,
) *MatchReport {
	logger.Logtype("debug", 0).
		Str("folder", folder).
		Int("candidatesTried", len(bestCandidates)).
		Msg("Track distance matching failed for all candidates - skipping")

	report := &MatchReport{
		DenialReason:    "wrong_runtime",
		ActualTracks:    len(albumTracks),
		ActualRuntimeMS: actualRuntimeMS,
	}

	limit := min(len(preScored), 3)
	for i := range limit {
		ps := preScored[i]
		cr := CandidateReport{
			Title:          ps.c.Title,
			Artist:         ps.c.Artist,
			MBID:           ps.c.MusicBrainzReleaseID,
			Year:           ps.c.Year,
			ExpectedTracks: ps.c.TotalTracks,
			AlbumDist:      ps.dist,
		}

		rptTracks := resolveTracksForMatching(
			ctx,
			ps.c.ID,
			ps.c.MusicBrainzReleaseID,
			"",
			"",
			ps.c.Title,
			ps.c.Artist,
		)
		if len(rptTracks) > 0 {
			var dbTotalMs int64
			for j := range rptTracks {
				dbTotalMs += rptTracks[j].RuntimeMs
			}

			if ps.c.TotalRuntime > 0 {
				cr.ExpectedRuntimeMS = ps.c.TotalRuntime
			} else {
				cr.ExpectedRuntimeMS = int(dbTotalMs)
			}

			rptResult, rptMatched, rptUsed := matchTracksByDistance(
				albumTracks,
				rptTracks,
				isVA,
				false,
				data,
			)
			for j := range rptTracks {
				if rptMatched[j] {
					tr := rptResult[j]
					if rptTracks[j].TrackNumber > 0 {
						tr.TrackNumber = int(rptTracks[j].TrackNumber)
					}

					if rptTracks[j].DiscNumber > 0 {
						tr.DiscNumber = int(rptTracks[j].DiscNumber)
					}

					if tr.Title == "" && rptTracks[j].Title != "" {
						tr.Title = rptTracks[j].Title
					}

					if rptTracks[j].RuntimeMs > 0 {
						tr.ExpectedRuntimeMS = rptTracks[j].RuntimeMs
					}

					cr.Tracks = append(cr.Tracks, CandidateTrackReport{
						Title:          tr.Title,
						TrackNumber:    tr.TrackNumber,
						DiscNumber:     tr.DiscNumber,
						DBRuntimeMS:    tr.ExpectedRuntimeMS,
						LocalRuntimeMS: tr.RuntimeMS,
						TrackDist: trackDistance(
							&rptResult[j],
							&rptTracks[j],
							isVA,
							false,
							data,
						),
					})
				} else {
					bestDist := math.MaxFloat64

					var bestLocalMs int64
					for k := range albumTracks {
						d := trackDistance(&albumTracks[k], &rptTracks[j], isVA, false, data)
						if d < bestDist {
							bestDist = d
							bestLocalMs = albumTracks[k].RuntimeMS
						}
					}

					if bestDist == math.MaxFloat64 {
						bestDist = 0
					}

					cr.Tracks = append(cr.Tracks, CandidateTrackReport{
						Title:          rptTracks[j].Title,
						TrackNumber:    int(rptTracks[j].TrackNumber),
						DiscNumber:     int(rptTracks[j].DiscNumber),
						DBRuntimeMS:    rptTracks[j].RuntimeMs,
						LocalRuntimeMS: bestLocalMs,
						TrackDist:      bestDist,
						Unmatched:      true,
					})
				}
			}

			for k, localTrack := range albumTracks {
				if rptUsed[k] {
					continue
				}

				cr.Tracks = append(cr.Tracks, CandidateTrackReport{
					Title:          localTrack.Title,
					TrackNumber:    localTrack.TrackNumber,
					DiscNumber:     localTrack.DiscNumber,
					LocalRuntimeMS: localTrack.RuntimeMS,
					LocalOnly:      true,
				})
			}
		}

		report.Candidates = append(report.Candidates, cr)
	}

	return report
}

// discoverAddFoundArtistAlbums triggers deferred artist or series discovery after a
// successful addFound import and track/runtime verification.
func discoverAddFoundArtistAlbums(
	ctx context.Context,
	albumTitle string, artist, musicBrainzID *string,
	cfgp *config.MediaTypeConfig,
	listid int,
	data *config.MediaDataConfig,
	folder string,
) {
	isVariousArtists := IsVariousArtists(*artist)

	if isVariousArtists && data.DiscoverSeriesAlbums {
		releaseGroupID := database.Getdatarow[string](false,
			"SELECT musicbrainz_release_group_id FROM dbalbums WHERE musicbrainz_release_id = ?",
			musicBrainzID,
		)

		if releaseGroupID == "" {
			return
		}

		// If a series name is known and other albums in the series are already in the DB,
		// discovery was already triggered for this series — skip to avoid redundant API calls.
		seriesName := database.Getdatarow[string](false,
			"SELECT series_name FROM dbalbums WHERE musicbrainz_release_id = ?",
			musicBrainzID,
		)
		if seriesName != "" {
			otherCount := database.Getdatarow[int](
				false,
				"SELECT COUNT(*) FROM dbalbums WHERE series_name = ? AND musicbrainz_release_id != ?",
				&seriesName,
				musicBrainzID,
			)
			if otherCount > 0 {
				logger.Logtype("debug", 1).
					Str("series", seriesName).
					Int("existing", otherCount).
					Msg("Series already discovered, skipping")

				return
			}
		}

		albumsAdded := DiscoverAndAddSeriesAlbums(
			ctx,
			releaseGroupID,
			albumTitle,
			cfgp,
			listid,
			data.AllowedReleaseTypes,
			data.MBMediaFormats,
		)
		if albumsAdded > 0 {
			logger.Logtype("info", 1).
				Str("series", albumTitle).
				Int("albums_added", albumsAdded).
				Msg("Added other albums in series")
		}
	} else if artist != nil && *artist != "" {
		// Only discover if this is the first album we have for this artist.
		// If the artist already has more than one album in the DB, discovery was already run.
		artistAlbumCount := database.Getdatarow[int](false,
			`SELECT COUNT(DISTINCT aa.dbalbum_id) FROM dbalbum_artists aa
			 JOIN dbartists ar ON ar.id = aa.dbartist_id
			 WHERE ar.name = ? COLLATE NOCASE`,
			artist,
		)
		if artistAlbumCount > 1 {
			logger.Logtype("debug", 1).
				Str("artist", *artist).
				Int("existing", artistAlbumCount).
				Msg("Artist already has albums, skipping discovery")

			return
		}

		albumsAdded := DiscoverAndAddArtistAlbums(
			ctx,
			artist,
			cfgp,
			listid,
			50,
			data.AllowedReleaseTypes,
			data.MBMediaFormats,
		)
		if albumsAdded > 0 {
			logger.Logtype("info", 1).
				Str("artist", *artist).
				Int("albums_added", albumsAdded).
				Msg("Added other albums by artist")
		}
	}
}

// logParsedMetadata emits a debug log of all fields from a parsed folderMetadata struct.
// idKey/idValue carry the media-type-specific identifier (e.g. "asin"/"B001..." or
// "musicBrainzID"/"...").
func logParsedMetadata(folder string, meta folderMetadata, idKey, idValue string) {
	logger.Logtype("debug", 0).
		Str("folder", folder).
		Str("folderArtist", meta.FolderArtist).
		Str("folderAlbum", meta.FolderAlbum).
		Str("rawArtist", meta.RawArtist).
		Str("rawAlbum", meta.RawAlbum).
		Str("filenameArtist", meta.FilenameArtist).
		Str("filenameAlbum", meta.FilenameAlbum).
		Str("tagArtist", meta.TagArtist).
		Str("tagAlbumArtist", meta.TagAlbumArtist).
		Str("tagAlbum", meta.TagAlbum).
		Str("parentArtist", meta.ParentArtist).
		Str("parentAlbum", meta.ParentAlbum).
		Str("grandparentArtist", meta.GrandparentArtist).
		Bool("isDiscFolder", meta.IsDiscFolder).
		Str(idKey, idValue).
		Int("year", meta.Year).
		Msg("DEBUG: Parsed metadata from all sources")
}

// tryMusicFallbacks queries all configured fallback providers in priority order,
// collects every DB row they find or insert, and returns them as candidates for
// the normal scoring pipeline (albumMatchDistance → runMusicCandidatePasses →
// applyMusicRecommendation). Unlike the old approach, it does NOT stop at the
// first result and does NOT perform track matching itself.
func tryMusicFallbacks(
	ctx context.Context,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	data *config.MediaDataConfig,
	folder string,
) []*database.AlbumSearchResult {
	seenIDs := make(map[uint]bool)

	var candidates []*database.AlbumSearchResult

	addIDs := func(ids []uint) {
		for i := range ids {
			if ids[i] == 0 || seenIDs[ids[i]] {
				continue
			}

			seenIDs[ids[i]] = true
			if c := database.FillAlbumResult(
				&database.AlbumSearchResult{ID: ids[i]},
			); c != nil &&
				c.ID > 0 {
				candidates = append(candidates, c)
			}
		}
	}

	for _, source := range config.GetMusicMetaSourcePriority() {
		switch source {
		case "lastfm":
			addIDs(
				tryMusicLastFMFallback(
					ctx,
					albumTitle,
					artist,
					localTotalMs,
					fileCount,
					data,
					folder,
				),
			)

		case "discogs":
			addIDs(
				tryMusicDiscogsFallback(
					ctx,
					albumTitle,
					artist,
					localTotalMs,
					fileCount,
					data,
					folder,
				),
			)

		case "deezer":
			addIDs(
				tryMusicDeezerFallback(
					ctx,
					albumTitle,
					artist,
					localTotalMs,
					fileCount,
					data,
					folder,
				),
			)

		case "theaudiodb":
			addIDs(
				tryMusicTheAudioDBFallback(
					ctx,
					albumTitle,
					artist,
					localTotalMs,
					fileCount,
					data,
					folder,
				),
			)

		case "itunes":
			addIDs(
				tryMusicItunesFallback(
					ctx,
					albumTitle,
					artist,
					localTotalMs,
					fileCount,
					data,
					folder,
				),
			)
		}
	}

	return candidates
}

// tryMusicLastFMFallback queries Last.fm for album info when the primary MusicBrainz
// lookup resulted in no_match or wrong_runtime. If Last.fm returns track data with a
// runtime close enough to the local files, it inserts a synthetic dbalbum+dbtracks row
// (no MusicBrainz ID) and performs track matching so the folder can be organised.
//
// Returns (dbID, expectedTracks, matchedTracks, true) on success; zero values on failure.
// tryMusicLastFMFallback queries Last.fm for album info.
// It finds/inserts all matching DB rows and returns their IDs.
// Track matching is handled by the caller via the normal scoring pipeline.
func tryMusicLastFMFallback(
	ctx context.Context,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	data *config.MediaDataConfig,
	folder string,
) []uint {
	lfm := providers.GetLastFM()
	if lfm == nil {
		return nil
	}

	release, err := lfm.GetAlbumInfo(ctx, *artist, *albumTitle, "")
	if err != nil || release == nil || len(release.Tracks) == 0 {
		return nil
	}

	// Track count must match file count exactly.
	if len(release.Tracks) != *fileCount {
		logger.Logtype("debug", 1).
			Str("folder", folder).
			Int("lfmTracks", len(release.Tracks)).
			Int("fileCount", *fileCount).
			Msg("Last.fm fallback: track count mismatch")

		return nil
	}

	// Compute total runtime from Last.fm tracks.
	var lfmTotalMs int64
	for i := range release.Tracks {
		lfmTotalMs += release.Tracks[i].Duration.Milliseconds()
	}

	// Runtime check when Last.fm returned durations.
	if lfmTotalMs > 0 {
		toleranceSec := 10
		if data != nil && data.PerTrackToleranceSecondsMax > 0 {
			toleranceSec = data.PerTrackToleranceSecondsMax
		} else if data != nil && data.PerTrackToleranceSeconds > 0 {
			toleranceSec = data.PerTrackToleranceSeconds
		}

		toleranceMs := int64(toleranceSec) * int64(*fileCount) * 1000

		diff := lfmTotalMs - localTotalMs
		if diff < 0 {
			diff = -diff
		}

		if diff > toleranceMs {
			logger.Logtype("debug", 1).
				Str("folder", folder).
				Int64("lfmTotalMs", lfmTotalMs).
				Int64("localTotalMs", localTotalMs).
				Int64("toleranceMs", toleranceMs).
				Msg("Last.fm fallback: runtime mismatch")

			return nil
		}
	}

	// Prefer the MusicBrainz ID returned by Last.fm for DB lookups.
	lfmMBID := release.MusicBrainzID
	slug := logger.StringToSlug(*albumTitle)

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

	if existingID == 0 {
		if data == nil || (!data.AddFound && !data.AllowAlternativeReleases) {
			return nil
		}

		lfmTitle := releaseTitleName(release.Title, albumTitle)
		lfmSlug := logger.StringToSlug(lfmTitle)

		result, insertErr := database.ExecNid(
			`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
			  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
			  label, country, total_tracks, total_runtime_ms, genres, cover_url)
			 VALUES (?, ?, '', '', '', '', ?, 0, '', '', '', '', ?, ?, '', '')`,
			&lfmTitle,
			&lfmMBID,
			&lfmSlug,
			fileCount,
			&lfmTotalMs,
		)
		if insertErr != nil {
			logger.Logtype("debug", 1).
				Str("folder", folder).
				Err(insertErr).
				Msg("Last.fm fallback: failed to insert synthetic dbalbum")

			return nil
		}

		existingID = logger.Int64ToUint(result)

		lfmArtistName, lfmArtistMBID := releaseArtistName(release.Artists, artist)

		lfmArtistNamePtr := &lfmArtistName
		if artistID := addOrGetArtist(lfmArtistNamePtr, &lfmArtistMBID); artistID > 0 {
			position := 0

			_, _ = database.ExecNid(
				`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position) VALUES (?, ?, ?)`,
				&existingID,
				&artistID,
				&position,
			)
		}

		// Use sequential index as track_number — Last.fm sometimes numbers per-disc.
		var (
			runtimeMs int64
			trackNum  int
		)

		for i := range release.Tracks {
			runtimeMs = release.Tracks[i].Duration.Milliseconds()
			trackNum = i + 1

			_, _ = database.ExecNid(
				`INSERT INTO dbtracks (dbalbum_id, title, track_number, disc_number, runtime_ms, acoustid)
				 VALUES (?, ?, ?, 1, ?, '')`,
				&existingID,
				&release.Tracks[i].Title,
				&trackNum,
				&runtimeMs,
			)
		}

		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("artist", *artist).
			Str("album", *albumTitle).
			Uint("dbalbumID", existingID).
			Msg("Last.fm fallback: inserted synthetic album into database")
	}

	return []uint{existingID}
}

// tryMusicDiscogsFallback queries Discogs for album info.
// It finds/inserts all matching DB rows and returns their IDs.
// Track matching is handled by the caller via the normal scoring pipeline.
func tryMusicDiscogsFallback(
	ctx context.Context,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	data *config.MediaDataConfig,
	folder string,
) []uint {
	dg := providers.GetDiscogs()
	if dg == nil {
		return nil
	}

	query := *artist + " " + *albumTitle

	results, err := dg.SearchReleases(ctx, query, 5)
	if err != nil || len(results) == 0 {
		return nil
	}

	var (
		ids                          []uint
		dgTitle, slug, masterIDStr   string
		dgTotalMs, toleranceMs, diff int64
		existingID                   uint
	)

	// Try each candidate until one passes validation.

	for i := range results {
		release, fetchErr := dg.GetReleaseByID(ctx, results[i].DiscogsID)
		if fetchErr != nil || release == nil || len(release.Tracks) == 0 {
			continue
		}

		// Track count must match file count exactly.
		if len(release.Tracks) != *fileCount {
			logger.Logtype("debug", 1).
				Str("folder", folder).
				Int("dgTracks", len(release.Tracks)).
				Int("fileCount", *fileCount).
				Msg("Discogs fallback: track count mismatch")

			continue
		}

		// Runtime check — only when Discogs returned actual durations.
		dgTotalMs = 0
		for j := range release.Tracks {
			dgTotalMs += release.Tracks[j].Duration.Milliseconds()
		}

		if dgTotalMs > 0 {
			toleranceSec := 10
			if data != nil && data.PerTrackToleranceSecondsMax > 0 {
				toleranceSec = data.PerTrackToleranceSecondsMax
			} else if data != nil && data.PerTrackToleranceSeconds > 0 {
				toleranceSec = data.PerTrackToleranceSeconds
			}

			toleranceMs = int64(toleranceSec) * int64(*fileCount) * 1000

			diff = dgTotalMs - localTotalMs
			if diff < 0 {
				diff = -diff
			}

			if diff > toleranceMs {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Int64("dgTotalMs", dgTotalMs).
					Int64("localTotalMs", localTotalMs).
					Int64("toleranceMs", toleranceMs).
					Msg("Discogs fallback: runtime mismatch")

				continue
			}
		}

		// Check if a DB entry already exists for this Discogs release / master.
		existingID = 0
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
			masterIDStr = strconv.Itoa(release.MasterID)
			database.Scanrowsdyn(false,
				`SELECT id FROM dbalbums WHERE discogs_master_id = ? LIMIT 1`,
				&existingID,
				&masterIDStr,
			)
		}

		if existingID == 0 {
			// Only insert new content when the user has opted in via AddFound or AllowAlternativeReleases.
			if data == nil || (!data.AddFound && !data.AllowAlternativeReleases) {
				continue
			}

			dgTitle = releaseTitleName(release.Title, albumTitle)
			slug = logger.StringToSlug(dgTitle)

			masterIDStr = ""
			if release.MasterID > 0 {
				masterIDStr = strconv.Itoa(release.MasterID)
			}

			result, insertErr := database.ExecNid(
				`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
				  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
				  label, country, total_tracks, total_runtime_ms, genres, cover_url)
				 VALUES (?, '', '', ?, ?, '', ?, ?, '', ?, ?, ?, ?, ?, '', ?)`,
				&dgTitle,
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
			if insertErr != nil {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Err(insertErr).
					Msg("Discogs fallback: failed to insert synthetic dbalbum")

				continue
			}

			existingID = logger.Int64ToUint(result)

			// Link artist — prefer name from Discogs response.
			dgArtistName, dgArtistMBID := releaseArtistName(release.Artists, artist)

			dgArtistNamePtr := &dgArtistName
			if artistID := addOrGetArtist(dgArtistNamePtr, &dgArtistMBID); artistID > 0 {
				position := 0

				_, _ = database.ExecNid(
					`INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position) VALUES (?, ?, ?)`,
					&existingID,
					&artistID,
					&position,
				)
			}

			insertSyntheticTracks(&existingID, release.Tracks)

			logger.Logtype("info", 1).
				Str("folder", folder).
				Str("artist", dgArtistName).
				Str("album", *albumTitle).
				Uint("dbalbumID", existingID).
				Msg("Discogs fallback: inserted synthetic album into database")
		}

		ids = append(ids, existingID)
	}

	return ids
}

// tryMusicDeezerFallback queries Deezer for album info.
// It finds/inserts all matching DB rows and returns their IDs.
// Track matching is handled by the caller via the normal scoring pipeline.
func tryMusicDeezerFallback(
	ctx context.Context,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	data *config.MediaDataConfig,
	folder string,
) []uint {
	dz := providers.GetDeezer()
	if dz == nil {
		return nil
	}

	query := *artist + " " + *albumTitle

	results, err := dz.SearchAlbums(ctx, query, 5)
	if err != nil || len(results) == 0 {
		return nil
	}

	var ids []uint

	for i := range results {
		if results[i].TrackCount != *fileCount {
			logger.Logtype("debug", 1).
				Str("folder", folder).
				Int("dzTracks", results[i].TrackCount).
				Int("fileCount", *fileCount).
				Msg("Deezer fallback: track count mismatch")

			continue
		}

		release, fetchErr := dz.GetAlbumByID(ctx, results[i].DeezerID)
		if fetchErr != nil || release == nil || len(release.Tracks) == 0 {
			continue
		}

		if len(release.Tracks) != *fileCount {
			continue
		}

		var dzTotalMs int64
		for j := range release.Tracks {
			dzTotalMs += release.Tracks[j].Duration.Milliseconds()
		}

		if dzTotalMs > 0 {
			toleranceSec := 10
			if data != nil && data.PerTrackToleranceSecondsMax > 0 {
				toleranceSec = data.PerTrackToleranceSecondsMax
			} else if data != nil && data.PerTrackToleranceSeconds > 0 {
				toleranceSec = data.PerTrackToleranceSeconds
			}

			toleranceMs := int64(toleranceSec) * int64(*fileCount) * 1000

			diff := dzTotalMs - localTotalMs
			if diff < 0 {
				diff = -diff
			}

			if diff > toleranceMs {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Int64("dzTotalMs", dzTotalMs).
					Int64("localTotalMs", localTotalMs).
					Int64("toleranceMs", toleranceMs).
					Msg("Deezer fallback: runtime mismatch")

				continue
			}
		}

		// Check for existing DB entry by Deezer ID.
		deezerIDStr := release.DeezerID

		var existingID uint
		if deezerIDStr != "" {
			database.Scanrowsdyn(false,
				`SELECT id FROM dbalbums WHERE deezer_id = ? LIMIT 1`,
				&existingID, &deezerIDStr,
			)
		}

		if existingID == 0 {
			if data == nil || (!data.AddFound && !data.AllowAlternativeReleases) {
				continue
			}

			dzTitle := releaseTitleName(release.Title, albumTitle)
			slug := logger.StringToSlug(dzTitle)

			result, insertErr := database.ExecNid(
				`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
				  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
				  label, country, total_tracks, total_runtime_ms, genres, cover_url, deezer_id)
				 VALUES (?, '', '', '', '', ?, ?, ?, '', '', ?, '', ?, ?, '', ?, ?)`,
				&dzTitle,
				&release.Barcode,
				&slug,
				&release.ReleaseYear,
				&release.Label,
				fileCount,
				&dzTotalMs,
				&release.CoverURL,
				&deezerIDStr,
			)
			if insertErr != nil {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Err(insertErr).
					Msg("Deezer fallback: failed to insert synthetic dbalbum")

				continue
			}

			existingID = logger.Int64ToUint(result)

			dzArtistName, dzArtistMBID := releaseArtistName(release.Artists, artist)

			dzArtistNamePtr := &dzArtistName
			if aid := addOrGetArtist(dzArtistNamePtr, &dzArtistMBID); aid > 0 {
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
				Str("folder", folder).
				Str("artist", dzArtistName).
				Str("album", *albumTitle).
				Uint("dbalbumID", existingID).
				Msg("Deezer fallback: inserted synthetic album into database")
		}

		ids = append(ids, existingID)
	}

	return ids
}

// tryMusicTheAudioDBFallback queries TheAudioDB for album info.
// It finds/inserts all matching DB rows and returns their IDs.
// Track matching is handled by the caller via the normal scoring pipeline.
func tryMusicTheAudioDBFallback(
	ctx context.Context,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	data *config.MediaDataConfig,
	folder string,
) []uint {
	tadb := providers.GetTheAudioDB()
	if tadb == nil {
		return nil
	}

	results, err := tadb.SearchAlbums(ctx, *artist, *albumTitle)
	if err != nil || len(results) == 0 {
		return nil
	}

	var ids []uint

	for i := range results {
		tadID := results[i].TheAudioDBID
		if tadID == "" {
			continue
		}

		release, fetchErr := tadb.GetTracksByAlbumID(ctx, tadID)
		if fetchErr != nil || release == nil || len(release.Tracks) == 0 {
			continue
		}

		if len(release.Tracks) != *fileCount {
			logger.Logtype("debug", 1).
				Str("folder", folder).
				Int("tadbTracks", len(release.Tracks)).
				Int("fileCount", *fileCount).
				Msg("TheAudioDB fallback: track count mismatch")

			continue
		}

		var tadbTotalMs int64
		for j := range release.Tracks {
			tadbTotalMs += release.Tracks[j].Duration.Milliseconds()
		}

		if tadbTotalMs > 0 {
			toleranceSec := 10
			if data != nil && data.PerTrackToleranceSecondsMax > 0 {
				toleranceSec = data.PerTrackToleranceSecondsMax
			} else if data != nil && data.PerTrackToleranceSeconds > 0 {
				toleranceSec = data.PerTrackToleranceSeconds
			}

			toleranceMs := int64(toleranceSec) * int64(*fileCount) * 1000

			diff := tadbTotalMs - localTotalMs
			if diff < 0 {
				diff = -diff
			}

			if diff > toleranceMs {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Int64("tadbTotalMs", tadbTotalMs).
					Int64("localTotalMs", localTotalMs).
					Int64("toleranceMs", toleranceMs).
					Msg("TheAudioDB fallback: runtime mismatch")

				continue
			}
		}

		// Check for existing DB entry by TheAudioDB ID.
		var existingID uint
		database.Scanrowsdyn(false,
			`SELECT id FROM dbalbums WHERE theaudiodb_id = ? LIMIT 1`,
			&existingID, &tadID,
		)

		if existingID == 0 {
			if data == nil || (!data.AddFound && !data.AllowAlternativeReleases) {
				continue
			}

			tadbTitle := releaseTitleName(results[i].Title, albumTitle)
			slug := logger.StringToSlug(tadbTitle)

			result, insertErr := database.ExecNid(
				`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
				  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
				  label, country, total_tracks, total_runtime_ms, genres, cover_url, theaudiodb_id)
				 VALUES (?, '', '', '', '', '', ?, ?, '', '', ?, '', ?, ?, '', ?, ?)`,
				&tadbTitle,
				&slug,
				&results[i].ReleaseYear,
				&results[i].Label,
				fileCount,
				&tadbTotalMs,
				&results[i].CoverURL,
				&tadID,
			)
			if insertErr != nil {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Err(insertErr).
					Msg("TheAudioDB fallback: failed to insert synthetic dbalbum")

				continue
			}

			existingID = logger.Int64ToUint(result)

			tadbArtistName, tadbArtistMBID := releaseArtistName(release.Artists, artist)

			tadbArtistNamePtr := &tadbArtistName
			if aid := addOrGetArtist(tadbArtistNamePtr, &tadbArtistMBID); aid > 0 {
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
				Str("folder", folder).
				Str("artist", tadbArtistName).
				Str("album", *albumTitle).
				Uint("dbalbumID", existingID).
				Msg("TheAudioDB fallback: inserted synthetic album into database")
		}

		ids = append(ids, existingID)
	}

	return ids
}

// tryMusicItunesFallback queries the iTunes Search API for album info.
// It finds/inserts all matching DB rows and returns their IDs.
// Track matching is handled by the caller via the normal scoring pipeline.
func tryMusicItunesFallback(
	ctx context.Context,
	albumTitle, artist *string,
	localTotalMs int64,
	fileCount *int,
	data *config.MediaDataConfig,
	folder string,
) []uint {
	it := providers.GetITunes()
	if it == nil {
		return nil
	}

	results, err := it.SearchAlbums(ctx, *artist, *albumTitle, 5)
	if err != nil || len(results) == 0 {
		return nil
	}

	var ids []uint

	for i := range results {
		itunesID := results[i].ITunesID
		if itunesID == 0 {
			continue
		}

		release, fetchErr := it.GetAlbumTracks(ctx, itunesID)
		if fetchErr != nil || release == nil || len(release.Tracks) == 0 {
			continue
		}

		// Use the actual track count from the lookup, not the search result metadata —
		// iTunes' trackCount in search can differ from the purchasable song count
		// (e.g. music videos counted separately, catalog inconsistencies).
		if len(release.Tracks) != *fileCount {
			logger.Logtype("debug", 1).
				Str("folder", folder).
				Int("itunesTracks", len(release.Tracks)).
				Int("fileCount", *fileCount).
				Msg("iTunes fallback: track count mismatch")

			continue
		}

		var itunessTotalMs int64
		for j := range release.Tracks {
			itunessTotalMs += release.Tracks[j].Duration.Milliseconds()
		}

		if itunessTotalMs > 0 {
			toleranceSec := 10
			if data != nil && data.PerTrackToleranceSecondsMax > 0 {
				toleranceSec = data.PerTrackToleranceSecondsMax
			} else if data != nil && data.PerTrackToleranceSeconds > 0 {
				toleranceSec = data.PerTrackToleranceSeconds
			}

			toleranceMs := int64(toleranceSec) * int64(*fileCount) * 1000

			diff := itunessTotalMs - localTotalMs
			if diff < 0 {
				diff = -diff
			}

			if diff > toleranceMs {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Int64("itunessTotalMs", itunessTotalMs).
					Int64("localTotalMs", localTotalMs).
					Int64("toleranceMs", toleranceMs).
					Msg("iTunes fallback: runtime mismatch")

				continue
			}
		}

		// Check for existing DB entry by iTunes collection ID.
		itunesIDStr := release.ITunesID

		var existingID uint
		if itunesIDStr != "" {
			database.Scanrowsdyn(false,
				`SELECT id FROM dbalbums WHERE itunes_id = ? LIMIT 1`,
				&existingID, &itunesIDStr,
			)
		}

		if existingID == 0 {
			if data == nil || (!data.AddFound && !data.AllowAlternativeReleases) {
				continue
			}

			itunesTitle := releaseTitleName(release.Title, albumTitle)
			slug := logger.StringToSlug(itunesTitle)

			result, insertErr := database.ExecNid(
				`INSERT INTO dbalbums (title, musicbrainz_release_group_id, musicbrainz_release_id,
				  discogs_release_id, discogs_master_id, upc, slug, year, release_type, format,
				  label, country, total_tracks, total_runtime_ms, genres, cover_url, itunes_id)
				 VALUES (?, '', '', '', '', '', ?, ?, '', '', '', '', ?, ?, '', ?, ?)`,
				&itunesTitle,
				&slug,
				&release.ReleaseYear,
				fileCount,
				&itunessTotalMs,
				&release.CoverURL,
				&itunesIDStr,
			)
			if insertErr != nil {
				logger.Logtype("debug", 1).
					Str("folder", folder).
					Err(insertErr).
					Msg("iTunes fallback: failed to insert synthetic dbalbum")

				continue
			}

			existingID = logger.Int64ToUint(result)

			itunesArtistName, itunesArtistMBID := releaseArtistName(release.Artists, artist)

			itunesArtistNamePtr := &itunesArtistName
			if aid := addOrGetArtist(itunesArtistNamePtr, &itunesArtistMBID); aid > 0 {
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
				Str("folder", folder).
				Str("artist", *artist).
				Str("album", *albumTitle).
				Uint("dbalbumID", existingID).
				Msg("iTunes fallback: inserted synthetic album into database")
		}

		ids = append(ids, existingID)
	}

	return ids
}

// buildMatchedTracksWithFallback builds a matched-tracks slice from LAP results.
// For each DB track left unmatched, it assigns the closest unused local file by
// track distance ("next-best" fallback).  isAudiobook controls whether
// trackDistance treats the content as an audiobook.
func buildMatchedTracksWithFallback(
	albumTracks []parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	result []parser_v2.TrackInfo,
	matched, used []bool,
	isVA, isAudiobook bool,
	data *config.MediaDataConfig,
) []parser_v2.TrackInfo {
	var matchedTracks []parser_v2.TrackInfo
	for i := range dbTracks {
		tr := result[i]
		if !matched[i] {
			bestDist := math.MaxFloat64

			bestK := -1
			for k := range albumTracks {
				if used[k] {
					continue
				}

				d := trackDistance(&albumTracks[k], &dbTracks[i], isVA, isAudiobook, data)
				if d < bestDist {
					bestDist = d
					bestK = k
				}
			}

			if bestK < 0 {
				continue
			}

			tr = albumTracks[bestK]
			used[bestK] = true
		}

		if dbTracks[i].TrackNumber > 0 {
			tr.TrackNumber = int(dbTracks[i].TrackNumber)
		}

		if dbTracks[i].DiscNumber > 0 {
			tr.DiscNumber = int(dbTracks[i].DiscNumber)
		}

		if dbTracks[i].Title != "" {
			tr.Title = dbTracks[i].Title
		}

		if dbTracks[i].RuntimeMs > 0 {
			tr.ExpectedRuntimeMS = dbTracks[i].RuntimeMs
		}

		tr.TrackDist = trackDistance(&tr, &dbTracks[i], isVA, isAudiobook, data)
		matchedTracks = append(matchedTracks, tr)
	}

	return matchedTracks
}

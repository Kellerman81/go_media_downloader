package importfeed

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
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

// Candidate sources reported by the match preview.
const (
	PreviewSourceDatabase    = "database"
	PreviewSourceMusicBrainz = "musicbrainz"
	PreviewSourceAudible     = "audible"
)

const (
	// previewDBLimit caps the database candidates whose track lists are
	// fetched; each one may cost a MusicBrainz call.
	previewDBLimit = 6
	// previewProviderLimit caps the releases fetched per external provider.
	previewProviderLimit = 3
	// previewMaxCandidates caps the candidates kept per entry.
	previewMaxCandidates = 12
)

var errPreviewNotImportable = errors.New("candidate cannot be imported")

// PreviewTrack is one row of a candidate's track comparison: a local file
// and the track it was assigned to. Missing rows have no local file, Extra
// rows have no wanted track.
type PreviewTrack struct {
	LocalFile       string
	LocalTitle      string
	LocalArtist     string
	LocalTrack      int
	LocalDisc       int
	LocalRuntimeMS  int64
	WantedTitle     string
	WantedArtist    string
	WantedTrack     int
	WantedDisc      int
	WantedRuntimeMS int64
	Dist            float64
	Missing         bool
	Extra           bool
}

// PreviewCandidate is one release a folder could be matched to. DBID is 0 for
// a release that only exists at an external provider; ImportPreviewCandidate
// creates its database row.
type PreviewCandidate struct {
	Key        string
	Source     string
	DBID       uint
	ExternalID string
	Title      string
	Artist     string
	Year       int
	Tracks     int
	RuntimeMS  int64
	AlbumDist  float64
	Dist       float64
	Missing    int
	Extra      int
	Rows       []PreviewTrack

	// What ImportPreviewCandidate needs to create a non-MusicBrainz music
	// release as a synthetic album.
	synth      *syntheticAlbum
	release    *apiexternal_v2.ReleaseDetails
	sequential bool
}

// PreviewEntry is the match preview of one album folder, or of one file of a
// multi-episode audiobook folder (File set).
type PreviewEntry struct {
	Folder     string
	File       string
	Artist     string
	Album      string
	Year       int
	MBID       string
	ASIN       string
	Files      int
	RuntimeMS  int64
	Candidates []*PreviewCandidate
	Err        string
}

// Best returns the closest candidate, or nil when there is none.
func (e *PreviewEntry) Best() *PreviewCandidate {
	if len(e.Candidates) == 0 {
		return nil
	}

	return e.Candidates[0]
}

// Candidate returns the candidate with key, or nil.
func (e *PreviewEntry) Candidate(key string) *PreviewCandidate {
	for _, c := range e.Candidates {
		if c.Key == key {
			return c
		}
	}

	return nil
}

// RecThresholds returns the strong and medium recommendation distance
// thresholds in effect for data.
func RecThresholds(data *config.MediaDataConfig) (strong, medium float64) {
	return recThresholds(data)
}

// PreviewAudioFolder runs the candidate search of the album matcher for folder
// without writing anything: database rows and external releases alike are
// only scored. A multi-episode audiobook folder yields one entry per file.
func PreviewAudioFolder(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) []*PreviewEntry {
	folder, files, reason := ResolveAlbumFiles(folder)
	if reason != "" {
		return []*PreviewEntry{{Folder: folder, Err: reason}}
	}

	switch cfgp.IsType {
	case config.MediaTypeMusic:
		return []*PreviewEntry{previewMusicFolder(ctx, folder, files, data)}
	case config.MediaTypeAudiobook:
		if len(files) > 1 && parser_v2.IsMultiEpisodeAudiobookFolder(folder, files) {
			out := make([]*PreviewEntry, 0, len(files))
			for i := range files {
				if ctx.Err() != nil {
					break
				}

				out = append(out, previewEpisodeFile(ctx, files[i], cfgp, data))
			}

			return out
		}

		return []*PreviewEntry{previewAudiobookFolder(ctx, folder, files, cfgp, data)}
	default:
		return []*PreviewEntry{{Folder: folder, Err: "unsupported media type"}}
	}
}

// ── Music ────────────────────────────────────────────────────────────────────

// musicPreviewCand is a music candidate before scoring.
type musicPreviewCand struct {
	pc       *PreviewCandidate
	c        *database.AlbumSearchResult
	dbTracks []database.DbtrackWithArtist
}

func previewMusicFolder(
	ctx context.Context,
	folder string,
	files []string,
	data *config.MediaDataConfig,
) *PreviewEntry {
	meta := parseFolderMetadata(folder, files, data)
	artist, albumTitle := meta.musicQuery()

	tracks := parser_v2.EnrichTracksWithTags(parser_v2.CollectTracksFromFiles(files))
	localTotalMs := int64(parser_v2.CalculateTotalRuntime(tracks) / time.Millisecond)
	fileCount := len(files)
	mbid := meta.TagMusicBrainzID

	entry := &PreviewEntry{
		Folder:    folder,
		Artist:    artist,
		Album:     albumTitle,
		Year:      meta.Year,
		MBID:      mbid,
		Files:     fileCount,
		RuntimeMS: localTotalMs,
	}

	var (
		cands []*musicPreviewCand
		seen  = make(map[string]bool)
	)

	addDB := func(c *database.AlbumSearchResult) {
		if c == nil || c.ID == 0 {
			return
		}

		key := "db:" + strconv.FormatUint(uint64(c.ID), 10)
		if seen[key] {
			return
		}

		seen[key] = true
		if c.MusicBrainzReleaseID != "" {
			seen["mb:"+c.MusicBrainzReleaseID] = true
		}

		cands = append(cands, &musicPreviewCand{
			pc: &PreviewCandidate{
				Key:        key,
				Source:     PreviewSourceDatabase,
				DBID:       c.ID,
				ExternalID: c.MusicBrainzReleaseID,
			},
			c: c,
		})
	}

	// Database: the tagged release first, then every artist+album combination.
	if mbid != "" {
		if c, _ := database.FindAlbumByMusicBrainzID(&mbid); c != nil {
			addDB(c)
		}
	}

	var dbMatches []*database.AlbumSearchResult

	pairs := buildMusicSearchPairs(meta)
	for i := range pairs {
		matches, err := database.FindAlbumsByArtistTitle(&pairs[i].artist, &pairs[i].album, 10)
		if err != nil {
			continue
		}

		dbMatches = append(dbMatches, matches...)
	}

	sort.SliceStable(dbMatches, func(i, j int) bool {
		return albumMatchDistance(dbMatches[i], artist, albumTitle, mbid, "", "", meta.Year, fileCount, data) <
			albumMatchDistance(dbMatches[j], artist, albumTitle, mbid, "", "", meta.Year, fileCount, data)
	})

	for i := range dbMatches {
		if len(cands) >= previewDBLimit {
			break
		}

		addDB(dbMatches[i])
	}

	addRelease := func(source, externalID string, release *apiexternal_v2.ReleaseDetails, existing func() uint, synth *syntheticAlbum, sequential bool) {
		key := source + ":" + externalID
		if release == nil || len(release.Tracks) == 0 || seen[key] {
			return
		}

		seen[key] = true

		if id := existing(); id != 0 {
			if c := database.FindAlbumByID(id); c != nil {
				addDB(c)
				return
			}
		}

		var totalMs int64
		for i := range release.Tracks {
			totalMs += releaseTrackMs(&release.Tracks[i])
		}

		releaseArtist, _ := releaseArtistName(release.Artists, &artist)
		c := &database.AlbumSearchResult{
			Title:        release.Title,
			Artist:       releaseArtist,
			Year:         release.ReleaseYear,
			TotalTracks:  len(release.Tracks),
			TotalRuntime: int(totalMs),
			Source:       source,
		}
		if source == PreviewSourceMusicBrainz {
			c.MusicBrainzReleaseID = externalID
		}

		if synth != nil {
			synth.totalRuntimeMs = totalMs
		}

		cands = append(cands, &musicPreviewCand{
			pc: &PreviewCandidate{
				Key:        key,
				Source:     source,
				ExternalID: externalID,
				synth:      synth,
				release:    release,
				sequential: sequential,
			},
			c:        c,
			dbTracks: mbTracksToDbTracks(release.Tracks),
		})
	}

	previewMusicBrainz(ctx, mbid, artist, albumTitle, fileCount, seen, addRelease)

	if artist != "" && albumTitle != "" {
		for _, source := range config.GetMusicMetaSourcePriority() {
			if ctx.Err() != nil {
				break
			}

			previewMusicProvider(ctx, source, artist, albumTitle, fileCount, addRelease)
		}
	}

	isVA := DetectVA(artist, tracks)

	for _, mc := range cands {
		if ctx.Err() != nil {
			break
		}

		if mc.dbTracks == nil {
			mc.dbTracks = resolveTracksForMatching(ctx, mc.c.ID, mc.c.MusicBrainzReleaseID, mc.c.Title, mc.c.Artist)
		}

		// Like the matcher: a release without a track list can't be verified,
		// and a forced organize against it fails.
		if len(mc.dbTracks) == 0 {
			continue
		}

		pc := mc.pc
		pc.Title = mc.c.Title
		pc.Artist = mc.c.Artist
		pc.Year = mc.c.Year
		pc.Tracks = len(mc.dbTracks)

		for i := range mc.dbTracks {
			pc.RuntimeMS += mc.dbTracks[i].RuntimeMs
		}

		pc.AlbumDist = albumMatchDistance(mc.c, artist, albumTitle, mbid, "", "", meta.Year, fileCount, data)
		pc.Dist = albumDistanceWithTracks(mc.c, artist, albumTitle, mbid, meta.Year, tracks, mc.dbTracks, isVA, data)
		pc.Rows, pc.Missing, pc.Extra = previewTrackRows(tracks, mc.dbTracks, isVA, false, data)

		entry.Candidates = append(entry.Candidates, pc)
	}

	finishPreviewEntry(entry)

	return entry
}

// previewMusicBrainz adds the tagged MusicBrainz release and the releases of
// a MusicBrainz text search for artist+album.
func previewMusicBrainz(
	ctx context.Context,
	mbid, artist, albumTitle string,
	fileCount int,
	seen map[string]bool,
	addRelease func(string, string, *apiexternal_v2.ReleaseDetails, func() uint, *syntheticAlbum, bool),
) {
	mb := getMusicBrainzProvider()
	if mb == nil {
		return
	}

	getRelease := func(id string) *apiexternal_v2.ReleaseDetails {
		release, err := retryOnRateLimit(ctx, func() (*apiexternal_v2.ReleaseDetails, error) {
			return mb.GetReleaseByID(ctx, id)
		})
		if err != nil {
			return nil
		}

		return release
	}

	existing := func(id string) func() uint {
		return func() uint {
			return database.Getdatarow[uint](false, "SELECT id FROM dbalbums WHERE musicbrainz_release_id = ?", &id)
		}
	}

	if mbid != "" && !seen["mb:"+mbid] {
		addRelease(PreviewSourceMusicBrainz, mbid, getRelease(mbid), existing(mbid), nil, false)
	}

	if artist == "" || albumTitle == "" {
		return
	}

	searchArtist := artist
	if IsVariousArtists(artist) {
		searchArtist = VariousArtistsName
	}

	query := string(BuildArtistAlbumSearch(albumTitle, searchArtist))

	releases, err := retryOnRateLimit(ctx, func() ([]apiexternal_v2.ReleaseSearchResult, error) {
		r, _, e := mb.SearchReleases(ctx, query, 25, 0)
		return r, e
	})
	if err != nil {
		return
	}

	releases = slices.DeleteFunc(releases, func(r apiexternal_v2.ReleaseSearchResult) bool {
		return r.MusicBrainzID == "" || seen["mb:"+r.MusicBrainzID] || !namesMatch(r.Title, albumTitle)
	})

	// Releases with as many tracks as there are files first: those are the
	// editions the folder most likely holds.
	sort.SliceStable(releases, func(i, j int) bool {
		return (releases[i].TrackCount == fileCount) && (releases[j].TrackCount != fileCount)
	})

	for i := range releases {
		if i >= previewProviderLimit+2 || ctx.Err() != nil {
			break
		}

		id := releases[i].MusicBrainzID
		addRelease(PreviewSourceMusicBrainz, id, getRelease(id), existing(id), nil, false)
	}
}

// previewMusicProvider adds the releases one fallback provider finds for
// artist+album, each with the synthetic album ImportPreviewCandidate would
// insert for it.
func previewMusicProvider(
	ctx context.Context,
	source, artist, albumTitle string,
	fileCount int,
	addRelease func(string, string, *apiexternal_v2.ReleaseDetails, func() uint, *syntheticAlbum, bool),
) {
	byColumn := func(column, value string) func() uint {
		return func() uint {
			if value == "" {
				return 0
			}

			return database.Getdatarow[uint](false, "SELECT id FROM dbalbums WHERE "+column+" = ? LIMIT 1", &value)
		}
	}

	switch source {
	case "lastfm":
		lfm := providers.GetLastFM()
		if lfm == nil {
			return
		}

		release, err := lfm.GetAlbumInfo(ctx, artist, albumTitle, "")
		if err != nil || release == nil {
			return
		}

		id := coalesceStr(release.MusicBrainzID, release.ID, artist+" - "+albumTitle)
		addRelease(source, id, release, byColumn("musicbrainz_release_id", release.MusicBrainzID),
			&syntheticAlbum{
				title:       releaseTitleName(release.Title, &albumTitle),
				mbReleaseID: release.MusicBrainzID,
			}, true)

	case "discogs":
		dg := providers.GetDiscogs()
		if dg == nil {
			return
		}

		results, err := dg.SearchReleases(ctx, artist+" "+albumTitle, 5)
		if err != nil {
			return
		}

		for i := range results {
			if i >= previewProviderLimit || ctx.Err() != nil {
				break
			}

			release, err := dg.GetReleaseByID(ctx, results[i].DiscogsID)
			if err != nil || release == nil {
				continue
			}

			var masterID string
			if release.MasterID > 0 {
				masterID = strconv.Itoa(release.MasterID)
			}

			addRelease(source, release.DiscogsID, release, byColumn("discogs_release_id", release.DiscogsID),
				&syntheticAlbum{
					title:            releaseTitleName(release.Title, &albumTitle),
					discogsReleaseID: release.DiscogsID,
					discogsMasterID:  masterID,
					year:             release.ReleaseYear,
					format:           release.Format,
					label:            release.Label,
					country:          release.Country,
					coverURL:         release.CoverURL,
				}, false)
		}

	case "deezer":
		dz := providers.GetDeezer()
		if dz == nil {
			return
		}

		results, err := dz.SearchAlbums(ctx, artist+" "+albumTitle, 5)
		if err != nil {
			return
		}

		sort.SliceStable(results, func(i, j int) bool {
			return results[i].TrackCount == fileCount && results[j].TrackCount != fileCount
		})

		for i := range results {
			if i >= previewProviderLimit || ctx.Err() != nil {
				break
			}

			release, err := dz.GetAlbumByID(ctx, results[i].DeezerID)
			if err != nil || release == nil {
				continue
			}

			addRelease(source, release.DeezerID, release, byColumn("deezer_id", release.DeezerID),
				&syntheticAlbum{
					title:    releaseTitleName(release.Title, &albumTitle),
					upc:      release.Barcode,
					year:     release.ReleaseYear,
					label:    release.Label,
					coverURL: release.CoverURL,
					deezerID: release.DeezerID,
				}, false)
		}

	case "theaudiodb":
		tadb := providers.GetTheAudioDB()
		if tadb == nil {
			return
		}

		results, err := tadb.SearchAlbums(ctx, artist, albumTitle)
		if err != nil {
			return
		}

		for i := range results {
			if i >= previewProviderLimit || ctx.Err() != nil {
				break
			}

			tadID := results[i].TheAudioDBID
			if tadID == "" {
				continue
			}

			release, err := tadb.GetTracksByAlbumID(ctx, tadID)
			if err != nil || release == nil {
				continue
			}

			if release.Title == "" {
				release.Title = results[i].Title
			}

			if release.ReleaseYear == 0 {
				release.ReleaseYear = results[i].ReleaseYear
			}

			addRelease(source, tadID, release, byColumn("theaudiodb_id", tadID),
				&syntheticAlbum{
					title:        releaseTitleName(results[i].Title, &albumTitle),
					year:         results[i].ReleaseYear,
					label:        results[i].Label,
					coverURL:     results[i].CoverURL,
					theaudiodbID: tadID,
				}, false)
		}

	case "itunes":
		it := providers.GetITunes()
		if it == nil {
			return
		}

		results, err := it.SearchAlbums(ctx, artist, albumTitle, 5)
		if err != nil {
			return
		}

		for i := range results {
			if i >= previewProviderLimit || ctx.Err() != nil {
				break
			}

			if results[i].ITunesID == 0 {
				continue
			}

			release, err := it.GetAlbumTracks(ctx, results[i].ITunesID)
			if err != nil || release == nil {
				continue
			}

			addRelease(source, release.ITunesID, release, byColumn("itunes_id", release.ITunesID),
				&syntheticAlbum{
					title:    releaseTitleName(release.Title, &albumTitle),
					year:     release.ReleaseYear,
					coverURL: release.CoverURL,
					itunesID: release.ITunesID,
				}, false)
		}
	}
}

// hasRuntimes reports whether any chapter carries a length; audiobooks are
// matched by chapter lengths, so chapters without any can't verify a folder.
func hasRuntimes(tracks []database.DbtrackWithArtist) bool {
	for i := range tracks {
		if tracks[i].RuntimeMs > 0 {
			return true
		}
	}

	return false
}

// releaseTrackMs returns a provider track's length in milliseconds.
func releaseTrackMs(t *apiexternal_v2.Track) int64 {
	if t.DurationMs > 0 {
		return int64(t.DurationMs)
	}

	return t.Duration.Milliseconds()
}

// ── Audiobooks ───────────────────────────────────────────────────────────────

// audiobookPreviewCand is an audiobook candidate before scoring.
type audiobookPreviewCand struct {
	pc *PreviewCandidate
	c  *database.AudiobookSearchResult
}

// audiobookCandidates collects database and Audible candidates for the given
// ASINs and author+title pairs.
type audiobookCandidates struct {
	cands []*audiobookPreviewCand
	seen  map[string]bool
}

func (a *audiobookCandidates) addDB(c *database.AudiobookSearchResult) {
	if c == nil || c.ID == 0 {
		return
	}

	key := "db:" + strconv.FormatUint(uint64(c.ID), 10)
	if a.seen[key] {
		return
	}

	a.seen[key] = true
	if c.ASIN != "" {
		a.seen["audible:"+c.ASIN] = true
	}

	a.cands = append(a.cands, &audiobookPreviewCand{
		pc: &PreviewCandidate{Key: key, Source: PreviewSourceDatabase, DBID: c.ID, ExternalID: c.ASIN},
		c:  c,
	})
}

func (a *audiobookCandidates) addAudible(r *apiexternal_v2.AudiobookSearchResult) {
	if r.ASIN == "" || a.seen["audible:"+r.ASIN] {
		return
	}

	if c, _ := database.FindAudiobookByASIN(r.ASIN); c != nil {
		a.addDB(c)
		return
	}

	a.seen["audible:"+r.ASIN] = true

	title := r.Title
	if r.Subtitle != "" && !logger.ContainsI(title, r.Subtitle) {
		title += ": " + r.Subtitle
	}

	a.cands = append(a.cands, &audiobookPreviewCand{
		pc: &PreviewCandidate{
			Key:        "audible:" + r.ASIN,
			Source:     PreviewSourceAudible,
			ExternalID: r.ASIN,
			Year:       r.ReleaseYear,
		},
		c: &database.AudiobookSearchResult{
			Title:     title,
			ASIN:      r.ASIN,
			Author:    strings.Join(r.Authors, ", "),
			Runtime:   r.RuntimeMinutes,
			Series:    coalesceStr(r.SeriesName, r.Series),
			SeriesNum: r.SeriesPosition,
		},
	})
}

// collect gathers the candidates: the given ASINs, the database by each
// author+title pair, and Audible by each distinct title.
func (a *audiobookCandidates) collect(
	ctx context.Context,
	asins []string,
	pairs []searchPair,
	title, author string,
	fileCount int,
	region string,
) {
	prov := providers.SharedAudible(region)

	for _, asin := range asins {
		if asin == "" {
			continue
		}

		if c, _ := database.FindAudiobookByASIN(asin); c != nil {
			a.addDB(c)
			continue
		}

		if prov == nil || a.seen["audible:"+asin] {
			continue
		}

		if d, err := prov.SearchByASIN(ctx, asin); err == nil && d != nil {
			a.addAudible(&apiexternal_v2.AudiobookSearchResult{
				ASIN:           d.ASIN,
				Title:          d.Title,
				Subtitle:       d.Subtitle,
				Authors:        d.Authors,
				ReleaseYear:    d.ReleaseYear,
				RuntimeMinutes: d.RuntimeMinutes,
				SeriesName:     d.SeriesName,
				SeriesPosition: d.SeriesPosition,
			})
		}
	}

	var dbMatches []*database.AudiobookSearchResult

	for i := range pairs {
		matches, err := database.FindAudiobooksByTitleAuthor(&pairs[i].title, &pairs[i].author, 10)
		if err == nil {
			dbMatches = append(dbMatches, matches...)
		}
	}

	sort.SliceStable(dbMatches, func(i, j int) bool {
		return audiobookMatchDistance(dbMatches[i], title, author, fileCount) <
			audiobookMatchDistance(dbMatches[j], title, author, fileCount)
	})

	for i := range dbMatches {
		if len(a.cands) >= previewDBLimit {
			break
		}

		a.addDB(dbMatches[i])
	}

	if prov == nil {
		return
	}

	var found []apiexternal_v2.AudiobookSearchResult

	searched := make(map[string]bool)
	for i := range pairs {
		key := strings.ToLower(pairs[i].title)
		if pairs[i].title == "" || searched[key] || len(searched) >= 3 || ctx.Err() != nil {
			continue
		}

		searched[key] = true

		results, err := prov.SearchByTitle(ctx, pairs[i].title, 10)
		if err == nil {
			found = append(found, results...)
		}
	}

	// Only the closest Audible results get their chapters fetched.
	distOf := func(r *apiexternal_v2.AudiobookSearchResult) float64 {
		return audiobookMatchDistance(&database.AudiobookSearchResult{
			Title:  r.Title,
			Author: strings.Join(r.Authors, ", "),
		}, title, author, 0)
	}

	sort.SliceStable(found, func(i, j int) bool { return distOf(&found[i]) < distOf(&found[j]) })

	added := 0
	for i := range found {
		if added >= previewProviderLimit+2 {
			break
		}

		before := len(a.cands)
		a.addAudible(&found[i])

		if len(a.cands) > before {
			added++
		}
	}
}

// score fills each candidate's distances and track rows against tracks.
func (a *audiobookCandidates) score(
	ctx context.Context,
	entry *PreviewEntry,
	tracks []parser_v2.TrackInfo,
	title, author string,
	region string,
	data *config.MediaDataConfig,
) {
	isVA := IsVariousArtists(author)

	for _, ac := range a.cands {
		if ctx.Err() != nil {
			break
		}

		dbTracks := resolveChaptersForMatching(ctx, ac.c.ID, ac.c.ASIN, region)
		if !hasRuntimes(dbTracks) {
			continue
		}

		pc := ac.pc
		pc.Title = ac.c.Title
		pc.Artist = ac.c.Author
		pc.Tracks = len(dbTracks)

		for i := range dbTracks {
			pc.RuntimeMS += dbTracks[i].RuntimeMs
		}

		if ac.c.ID != 0 {
			pc.Year = database.Getdatarow[int](false, "SELECT year FROM dbaudiobooks WHERE id = ?", &ac.c.ID)
			if pc.Artist == "" {
				// Only for display: the distance above scores what the matcher sees.
				pc.Artist = strings.Join(database.Getrowssize[string](false,
					"SELECT count() FROM dbaudiobook_authors WHERE dbaudiobook_id = ?",
					`SELECT a.name FROM dbauthors a JOIN dbaudiobook_authors aa ON aa.dbauthor_id = a.id
					  WHERE aa.dbaudiobook_id = ?`, &ac.c.ID), ", ")
			}
		}

		pc.AlbumDist = audiobookMatchDistance(ac.c, title, author, len(tracks))
		pc.Dist = audiobookDistanceWithTracks(ac.c, title, author, tracks, dbTracks, data)

		if len(tracks) == 1 && len(dbTracks) > 1 {
			pc.Rows = []PreviewTrack{singleFilePreviewRow(ac.c, &tracks[0], dbTracks, data)}
		} else {
			pc.Rows, pc.Missing, pc.Extra = previewTrackRows(tracks, dbTracks, isVA, true, data)
		}

		entry.Candidates = append(entry.Candidates, pc)
	}
}

func previewAudiobookFolder(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) *PreviewEntry {
	tracks := parser_v2.EnrichTracksWithTags(parser_v2.CollectTracksFromFiles(files))
	meta := parseFolderMetadata(folder, files, data)
	author, title := meta.audiobookQuery()

	asins := []string{parser_v2.ParseASINFromPath(folder), meta.FilenameASIN, meta.TagASIN}
	if meta.IsDiscFolder {
		asins = append(asins, parser_v2.ParseASINFromPath(meta.ParentDir))
	}

	entry := &PreviewEntry{
		Folder:    folder,
		Artist:    author,
		Album:     title,
		Year:      meta.Year,
		ASIN:      coalesceStr(asins...),
		Files:     len(files),
		RuntimeMS: int64(parser_v2.CalculateTotalRuntime(tracks) / time.Millisecond),
	}

	ac := &audiobookCandidates{seen: make(map[string]bool)}
	ac.collect(ctx, asins, buildAudiobookSearchPairs(meta), title, author, len(files), cfgp.AudibleRegion)
	ac.score(ctx, entry, tracks, title, author, cfgp.AudibleRegion, data)

	finishPreviewEntry(entry)

	return entry
}

// previewEpisodeFile previews one file of a multi-episode audiobook folder as
// its own audiobook.
func previewEpisodeFile(
	ctx context.Context,
	file string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) *PreviewEntry {
	f := readSingleAudiobookFile(file)

	entry := &PreviewEntry{
		Folder:    f.folder,
		File:      file,
		Artist:    f.artist,
		Album:     f.albumTitle,
		ASIN:      f.asin,
		Files:     1,
		RuntimeMS: f.track.RuntimeMS,
	}

	ac := &audiobookCandidates{seen: make(map[string]bool)}
	ac.collect(ctx, []string{f.asin}, singleAudiobookSearchPairs(&f), f.albumTitle, f.artist, 1, cfgp.AudibleRegion)
	ac.score(ctx, entry, []parser_v2.TrackInfo{f.track}, f.albumTitle, f.artist, cfgp.AudibleRegion, data)

	finishPreviewEntry(entry)

	return entry
}

// singleFilePreviewRow compares one file holding a whole book with the sum of
// the candidate's chapters.
func singleFilePreviewRow(
	c *database.AudiobookSearchResult,
	local *parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	data *config.MediaDataConfig,
) PreviewTrack {
	var chaptersMs int64
	for i := range dbTracks {
		chaptersMs += dbTracks[i].RuntimeMs
	}

	_, diffMs, toleranceMs := singleFileTotalMatch(local.RuntimeMS, chaptersMs, len(dbTracks), data)

	dist := 1.0
	if toleranceMs > 0 {
		dist = min(float64(diffMs)/float64(toleranceMs), 1.0)
	}

	return PreviewTrack{
		LocalFile:       local.Filename,
		LocalTitle:      local.Title,
		LocalArtist:     local.Artist,
		LocalTrack:      local.TrackNumber,
		LocalDisc:       local.DiscNumber,
		LocalRuntimeMS:  local.RuntimeMS,
		WantedTitle:     c.Title + " (" + strconv.Itoa(len(dbTracks)) + " chapters)",
		WantedArtist:    c.Author,
		WantedTrack:     1,
		WantedDisc:      1,
		WantedRuntimeMS: chaptersMs,
		Dist:            dist,
	}
}

// ── Shared ───────────────────────────────────────────────────────────────────

// previewTrackRows pairs local tracks with a candidate's tracks the way the
// matcher does and returns one row per pair, then the wanted tracks without
// a file, then the files without a wanted track.
func previewTrackRows(
	tracks []parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	isVA, isAudiobook bool,
	data *config.MediaDataConfig,
) (rows []PreviewTrack, missing, extra int) {
	result, matched, used := matchTracksByDistance(tracks, dbTracks, isVA, isAudiobook, data)
	scored := globaliseDiscTrackNumbers(dbTracks, tracks)

	rows = make([]PreviewTrack, 0, max(len(tracks), len(dbTracks)))

	wanted := func(row *PreviewTrack, i int) {
		row.WantedTitle = dbTracks[i].Title
		row.WantedArtist = dbTracks[i].Artist
		row.WantedTrack = int(dbTracks[i].TrackNumber)
		row.WantedDisc = int(dbTracks[i].DiscNumber)
		row.WantedRuntimeMS = dbTracks[i].RuntimeMs
	}

	local := func(row *PreviewTrack, t *parser_v2.TrackInfo) {
		row.LocalFile = t.Filename
		if row.LocalFile == "" {
			row.LocalFile = filepath.Base(t.Filepath)
		}

		row.LocalTitle = t.Title
		row.LocalArtist = t.Artist
		row.LocalTrack = t.TrackNumber
		row.LocalDisc = t.DiscNumber
		row.LocalRuntimeMS = t.RuntimeMS
	}

	for i := range dbTracks {
		if !matched[i] {
			continue
		}

		var row PreviewTrack
		wanted(&row, i)
		local(&row, &result[i])
		row.Dist = trackDistance(&result[i], &scored[i], isVA, isAudiobook, data)
		rows = append(rows, row)
	}

	for i := range dbTracks {
		if matched[i] {
			continue
		}

		row := PreviewTrack{Missing: true, Dist: 1}
		wanted(&row, i)
		rows = append(rows, row)
		missing++
	}

	for j := range tracks {
		if used[j] {
			continue
		}

		row := PreviewTrack{Extra: true, Dist: 1}
		local(&row, &tracks[j])
		rows = append(rows, row)
		extra++
	}

	return rows, missing, extra
}

// finishPreviewEntry sorts the candidates closest first and drops the tail.
func finishPreviewEntry(entry *PreviewEntry) {
	sort.SliceStable(entry.Candidates, func(i, j int) bool {
		return entry.Candidates[i].Dist < entry.Candidates[j].Dist
	})

	if len(entry.Candidates) > previewMaxCandidates {
		entry.Candidates = entry.Candidates[:previewMaxCandidates]
	}

	if len(entry.Candidates) == 0 && entry.Err == "" {
		entry.Err = "no_match"
	}
}

// ImportPreviewCandidate returns the database row of c, creating it first
// when c only exists at an external provider: MusicBrainz releases and
// Audible books are imported like AddFound does, releases of the other music
// providers become synthetic albums. listid is the list imports go to.
func ImportPreviewCandidate(
	ctx context.Context,
	c *PreviewCandidate,
	entry *PreviewEntry,
	cfgp *config.MediaTypeConfig,
	listid int,
) (uint, error) {
	if c.DBID != 0 {
		return c.DBID, nil
	}

	var (
		id  uint
		err error
	)

	switch {
	case c.Source == PreviewSourceAudible:
		id, err = JobImportAudiobooks(ctx, c.ExternalID, cfgp, listid, true)
	case c.Source == PreviewSourceMusicBrainz:
		id, err = JobImportAlbums(ctx, c.ExternalID, cfgp, listid, true)
	case c.synth != nil && c.release != nil:
		trackCount := len(c.release.Tracks)
		id = insertSyntheticAlbum(ctx, c.synth, c.release.Artists, &entry.Artist,
			c.release.Tracks, c.sequential, &trackCount, entry.Folder, c.Source)
	default:
		return 0, errPreviewNotImportable
	}

	if err != nil {
		return 0, err
	}

	if id == 0 {
		return 0, errPreviewNotImportable
	}

	return id, nil
}

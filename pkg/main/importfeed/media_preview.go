package importfeed

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// Candidate sources of the movie, series and book match preview, besides
// PreviewSourceDatabase.
const (
	MediaSourceImdb        = "imdb"
	MediaSourceTmdb        = "tmdb"
	MediaSourceOmdb        = "omdb"
	MediaSourceTvdb        = "tvdb"
	MediaSourceTrakt       = "trakt"
	MediaSourceOpenLibrary = "openlibrary"
)

const (
	// mediaDBLimit caps the database rows taken from one title lookup.
	mediaDBLimit = 10
	// mediaProviderLimit caps the results taken from one provider search;
	// TMDB results cost one more call each for their IMDb id.
	mediaProviderLimit = 5
	// mediaAltTitleLimit caps the alternate titles a row is scored against.
	mediaAltTitleLimit = 30
)

// mediaWeights weighs the components of a media candidate's distance. An
// external id the file names (IMDb, TVDB, ISBN) outweighs the title.
var mediaWeights = map[string]float64{
	"title":   3,
	"year":    1,
	"id":      5,
	"episode": 2,
	"author":  2,
}

var errMediaNoID = errors.New("candidate has no IMDb id, TVDB id or ISBN to import it by")

// MediaQuery is what was parsed from a movie, episode or book file.
type MediaQuery struct {
	IsType     uint
	Title      string
	Year       int
	Imdb       string
	Tvdb       int
	ISBN       string
	Author     string
	Identifier string
	Season     string
	Episodes   []int
}

// MediaField is one row of a candidate's field comparison.
type MediaField struct {
	Name   string
	Have   string
	Wanted string
	Dist   float64
	Scored bool
}

// MediaEpisode is a parsed episode of a file and the database episode of the
// candidate it maps to. DBEpisodeID is 0 when the candidate has no such
// episode (or, for an external candidate, it is unknown before import).
type MediaEpisode struct {
	Identifier  string
	Title       string
	DBEpisodeID uint
	EpisodeID   uint
	Files       int
}

// MediaCandidate is one movie, series or book a file could be matched to.
// DBID is 0 for an entry that only exists at a provider; MediaID is the
// list entry of the media config (movies.id, series.id, books.id), 0 when
// the entry is in none of its lists. ImportMediaCandidate creates both.
type MediaCandidate struct {
	Key        string
	Source     string
	DBID       uint
	MediaID    uint
	ListName   string
	ExternalID string
	Title      string
	MatchedAs  string
	Year       int
	Author     string
	Dist       float64
	Matcher    bool
	Fields     []MediaField
	Episodes   []MediaEpisode

	altTitles []string
}

// MediaCandidateSet is the scored candidates of one file.
type MediaCandidateSet struct {
	Candidates []*MediaCandidate
}

// Best returns the closest candidate, or nil.
func (s *MediaCandidateSet) Best() *MediaCandidate {
	if s == nil || len(s.Candidates) == 0 {
		return nil
	}

	return s.Candidates[0]
}

// Candidate returns the candidate with key, or nil.
func (s *MediaCandidateSet) Candidate(key string) *MediaCandidate {
	if s == nil {
		return nil
	}

	for _, c := range s.Candidates {
		if c.Key == key {
			return c
		}
	}

	return nil
}

// Matcher returns the candidate the structure job would pick, or nil.
func (s *MediaCandidateSet) Matcher() *MediaCandidate {
	if s == nil {
		return nil
	}

	for _, c := range s.Candidates {
		if c.Matcher {
			return c
		}
	}

	return nil
}

// mediaExt is a provider search result: the id it is imported by and what
// it is scored on.
type mediaExt struct {
	id, title, author string
	year              int
}

// MediaSearchCache keeps provider search results between the files of one
// folder; all episodes of a season pack search for the same title. It is not
// safe for concurrent use.
type MediaSearchCache struct {
	results map[string][]mediaExt
}

// NewMediaSearchCache returns an empty search cache.
func NewMediaSearchCache() *MediaSearchCache {
	return &MediaSearchCache{results: make(map[string][]mediaExt)}
}

type mediaCollector struct {
	cache *MediaSearchCache
	ctx   context.Context
	q     *MediaQuery
	cfgp  *config.MediaTypeConfig
	byDB  map[uint]*MediaCandidate
	byExt map[string]*MediaCandidate
	out   []*MediaCandidate
}

// FindMediaCandidates collects and scores the candidates of a movie, episode
// or book file without writing anything: rows of the database found by id
// or title, and results of the metadata providers (through cache, which may
// be nil). matcherDBID is the database row the structure job's own lookup
// found (0 = none); it is always a candidate and flagged as the matcher's
// pick.
func FindMediaCandidates(
	ctx context.Context,
	q *MediaQuery,
	cfgp *config.MediaTypeConfig,
	matcherDBID uint,
	cache *MediaSearchCache,
) *MediaCandidateSet {
	if cache == nil {
		cache = NewMediaSearchCache()
	}

	col := &mediaCollector{
		cache: cache,
		ctx:   ctx,
		q:     q,
		cfgp:  cfgp,
		byDB:  make(map[uint]*MediaCandidate),
		byExt: make(map[string]*MediaCandidate),
	}

	if matcherDBID != 0 {
		col.addDB(matcherDBID, PreviewSourceDatabase).Matcher = true
	}

	switch q.IsType {
	case config.MediaTypeMovie:
		col.movieCandidates()
	case config.MediaTypeSeries:
		col.serieCandidates()
	case config.MediaTypeBook:
		col.bookCandidates()
	}

	for _, c := range col.out {
		scoreMediaCandidate(q, c)
	}

	sort.SliceStable(col.out, func(i, j int) bool {
		return col.out[i].Dist < col.out[j].Dist
	})

	// The matcher's pick stays even when it scores below the cap.
	if len(col.out) > previewMaxCandidates {
		kept := col.out[:previewMaxCandidates]
		for _, c := range col.out[previewMaxCandidates:] {
			if c.Matcher {
				kept[len(kept)-1] = c
			}
		}

		col.out = kept
	}

	return &MediaCandidateSet{Candidates: col.out}
}

// addDB adds the database row dbid as a candidate (once) and returns it.
func (col *mediaCollector) addDB(dbid uint, source string) *MediaCandidate {
	if c, ok := col.byDB[dbid]; ok {
		return c
	}

	c := &MediaCandidate{
		Key:    "db:" + strconv.FormatUint(uint64(dbid), 10),
		Source: source,
		DBID:   dbid,
	}

	switch col.q.IsType {
	case config.MediaTypeMovie:
		var orig string
		database.GetdatarowArgs(
			"select coalesce(title, ''), coalesce(original_title, ''), coalesce(year, 0), coalesce(imdb_id, '') from dbmovies where id = ?",
			&dbid, &c.Title, &orig, &c.Year, &c.ExternalID,
		)
		c.altTitles = append(database.GetrowsN[string](false, mediaAltTitleLimit,
			"select title from dbmovie_titles where dbmovie_id = ? and title != '' limit 30", &dbid), orig)

	case config.MediaTypeSeries:
		var (
			firstaired string
			tvdb       int
		)
		database.GetdatarowArgs(
			"select coalesce(seriename, ''), coalesce(firstaired, ''), coalesce(thetvdb_id, 0) from dbseries where id = ?",
			&dbid, &c.Title, &firstaired, &tvdb,
		)

		if len(firstaired) >= 4 {
			c.Year = logger.StringToInt(firstaired[:4])
		}

		if tvdb != 0 {
			c.ExternalID = strconv.Itoa(tvdb)
		}

		c.altTitles = database.GetrowsN[string](false, mediaAltTitleLimit,
			"select title from dbserie_alternates where dbserie_id = ? and title != '' limit 30", &dbid)

	case config.MediaTypeBook:
		var isbn10 string
		database.GetdatarowArgs(
			"select coalesce(b.title, ''), coalesce(b.year, 0), coalesce(b.isbn_13, ''), coalesce(b.isbn_10, ''), coalesce(a.name, '') from dbbooks b left join dbauthors a on a.id = b.dbauthor_id where b.id = ?",
			&dbid, &c.Title, &c.Year, &c.ExternalID, &isbn10, &c.Author,
		)

		if c.ExternalID == "" {
			c.ExternalID = isbn10
		}

		c.altTitles = database.GetrowsN[string](false, mediaAltTitleLimit,
			"select title from dbbook_titles where dbbook_id = ? and title != '' limit 30", &dbid)
	}

	c.MediaID, c.ListName = mediaListEntry(col.cfgp, dbid)

	if col.q.IsType == config.MediaTypeSeries {
		c.Episodes = serieEpisodes(col.q, dbid, c.MediaID)
	}

	col.byDB[dbid] = c
	if c.ExternalID != "" {
		col.byExt[c.ExternalID] = c
	}

	col.out = append(col.out, c)

	return c
}

// addExternal adds a provider result identified by extID (IMDb id, TVDB id
// or ISBN). A result whose id is already in the database becomes that row's
// candidate; dbLookup finds it.
func (col *mediaCollector) addExternal(
	source, extID, title string,
	year int,
	author string,
	dbLookup func(string) uint,
) {
	if extID == "" {
		return
	}

	if _, ok := col.byExt[extID]; ok {
		return
	}

	if dbid := dbLookup(extID); dbid != 0 {
		col.addDB(dbid, PreviewSourceDatabase)
		return
	}

	c := &MediaCandidate{
		Key:        source + ":" + extID,
		Source:     source,
		ExternalID: extID,
		Title:      title,
		Year:       year,
		Author:     author,
	}

	if col.q.IsType == config.MediaTypeSeries {
		c.Episodes = serieEpisodes(col.q, 0, 0)
	}

	col.byExt[extID] = c
	col.out = append(col.out, c)
}

// addSearch adds the results of a provider search, running it only when the
// cache has no results for source and key yet.
func (col *mediaCollector) addSearch(source, key string, search func() []mediaExt, dbLookup func(string) uint) {
	if col.ctx.Err() != nil {
		return
	}

	key = source + "|" + key

	results, ok := col.cache.results[key]
	if !ok {
		results = search()
		col.cache.results[key] = results
	}

	for _, r := range results {
		col.addExternal(source, r.id, r.title, r.year, r.author, dbLookup)
	}
}

func (col *mediaCollector) addDBRows(query string, args ...any) {
	for _, id := range database.GetrowsN[uint](false, mediaDBLimit, query, args...) {
		if id != 0 {
			col.addDB(id, PreviewSourceDatabase)
		}
	}
}

// ── Movies ───────────────────────────────────────────────────────────────────

func movieDBIDByImdb(imdb string) uint {
	return MovieFindDBIDByImdb(&imdb)
}

func (col *mediaCollector) movieCandidates() {
	q := col.q
	ctx := col.ctx

	if q.Imdb != "" {
		imdb := logger.AddImdbPrefix(q.Imdb)
		if dbid := movieDBIDByImdb(imdb); dbid != 0 {
			col.addDB(dbid, PreviewSourceDatabase)
		} else {
			var (
				title string
				year  int
			)
			database.GetdatarowArgsImdb(
				"select coalesce(primary_title, ''), coalesce(start_year, 0) from imdb_titles where tconst = ?",
				&imdb, &title, &year,
			)
			col.addExternal(MediaSourceImdb, imdb, title, year, "", movieDBIDByImdb)
		}
	}

	if q.Title == "" {
		return
	}

	slug := logger.StringToSlugCached(q.Title)
	col.addDBRows(
		"select id from dbmovies where title = ?1 COLLATE NOCASE or original_title = ?1 COLLATE NOCASE or slug = ?2 union select dbmovie_id from dbmovie_titles where title = ?1 COLLATE NOCASE or slug = ?2 limit 10",
		&q.Title, &slug,
	)

	general := config.GetSettingsGeneral()

	for _, provider := range getsearchprovider(false) {
		var search func() []mediaExt

		switch provider {
		case logger.StrImdb:
			if !general.MovieMetaSourceImdb {
				continue
			}

			search = func() []mediaExt {
				rows := database.GetrowsN[database.DbstaticTwoStringOneRInt](
					true,
					mediaProviderLimit,
					"select t.tconst, t.primary_title, coalesce(t.start_year, 0) from imdb_titles t left join imdb_ratings r on r.tconst = t.tconst where (t.primary_title = ?1 COLLATE NOCASE or t.original_title = ?1 COLLATE NOCASE or t.slug = ?2) and t.title_type in "+imdbMovieTitleTypes+" order by coalesce(r.num_votes, 0) desc limit 5",
					&q.Title, &slug,
				)

				out := make([]mediaExt, 0, len(rows))
				for _, r := range rows {
					out = append(out, mediaExt{id: r.Str1, title: r.Str2, year: r.Num})
				}

				return out
			}

		case MediaSourceTmdb:
			tmdbp := providers.GetTMDB()
			if !general.MovieMetaSourceTmdb || tmdbp == nil {
				continue
			}

			search = func() []mediaExt {
				results, err := tmdbp.SearchMovies(ctx, q.Title, 0)
				if err != nil {
					logger.Logtype("debug", 1).Err(err).Str(logger.StrTitle, q.Title).Msg("match preview: tmdb search failed")
					return nil
				}

				var out []mediaExt

				for _, r := range results[:min(len(results), mediaProviderLimit)] {
					imdb := database.Getdatarow[string](false, "select imdb_id from dbmovies where moviedb_id = ?", &r.ID)
					if imdb == "" {
						ext, err := tmdbp.GetMovieExternalIDs(ctx, r.ID)
						if err != nil || ext == nil {
							continue
						}

						imdb = ext.IMDbID
					}

					year := r.Year
					if year == 0 && !r.ReleaseDate.IsZero() {
						year = r.ReleaseDate.Year()
					}

					out = append(out, mediaExt{id: imdb, title: r.Title, year: year})
				}

				return out
			}

		case MediaSourceOmdb:
			omdbp := providers.GetOMDB()
			if !general.MovieMetaSourceOmdb || omdbp == nil {
				continue
			}

			search = func() []mediaExt {
				results, err := omdbp.SearchByTitle(ctx, q.Title, 0, "movie")
				if err != nil {
					logger.Logtype("debug", 1).Err(err).Str(logger.StrTitle, q.Title).Msg("match preview: omdb search failed")
					return nil
				}

				out := make([]mediaExt, 0, mediaProviderLimit)
				for _, r := range results[:min(len(results), mediaProviderLimit)] {
					out = append(out, mediaExt{id: r.ImdbID, title: r.Title, year: logger.StringToInt(r.Year)})
				}

				return out
			}

		default:
			continue
		}

		col.addSearch(provider, q.Title, search, movieDBIDByImdb)
	}
}

// ── Series ───────────────────────────────────────────────────────────────────

func serieDBIDByTvdb(tvdb string) uint {
	return database.Getdatarow[uint](false, "select id from dbseries where thetvdb_id = ?", &tvdb)
}

func seriesExts(results []apiexternal_v2.SeriesSearchResult, err error, source, title string) []mediaExt {
	if err != nil {
		logger.Logtype("debug", 1).Err(err).Str(logger.StrTitle, title).Msg("match preview: " + source + " search failed")
		return nil
	}

	var out []mediaExt

	for _, r := range results[:min(len(results), mediaProviderLimit)] {
		if r.TVDbID == 0 {
			continue
		}

		year := 0
		if !r.FirstAirDate.IsZero() {
			year = r.FirstAirDate.Year()
		}

		out = append(out, mediaExt{id: strconv.Itoa(r.TVDbID), title: r.Name, year: year})
	}

	return out
}

func (col *mediaCollector) serieCandidates() {
	q := col.q
	ctx := col.ctx

	if q.Tvdb != 0 {
		tvdb := strconv.Itoa(q.Tvdb)
		if dbid := serieDBIDByTvdb(tvdb); dbid != 0 {
			col.addDB(dbid, PreviewSourceDatabase)
		} else if tvdbp := providers.GetTVDB(); tvdbp != nil {
			col.addSearch(MediaSourceTvdb, "id:"+tvdb, func() []mediaExt {
				d, err := tvdbp.GetSeriesByID(ctx, q.Tvdb, "")
				if err != nil || d == nil {
					return nil
				}

				return []mediaExt{{id: tvdb, title: d.Name, year: d.FirstAirDate.Year()}}
			}, serieDBIDByTvdb)
		}
	}

	if q.Title == "" {
		return
	}

	titles := []string{q.Title}
	if q.Year != 0 {
		titles = append(titles, q.Title+" ("+strconv.Itoa(q.Year)+")")
	}

	for _, t := range titles {
		slug := logger.StringToSlugCached(t)
		col.addDBRows(
			"select id from dbseries where seriename = ?1 COLLATE NOCASE or slug = ?2 union select dbserie_id from dbserie_alternates where title = ?1 COLLATE NOCASE or slug = ?2 limit 10",
			&t, &slug,
		)
	}

	if tvdbp := providers.GetTVDB(); tvdbp != nil {
		col.addSearch(MediaSourceTvdb, q.Title, func() []mediaExt {
			res, err := tvdbp.SearchSeries(ctx, q.Title, 0)
			return seriesExts(res, err, MediaSourceTvdb, q.Title)
		}, serieDBIDByTvdb)
	}

	if traktp := providers.GetTrakt(); traktp != nil {
		col.addSearch(MediaSourceTrakt, q.Title, func() []mediaExt {
			res, err := traktp.SearchSeries(ctx, q.Title, 0)
			return seriesExts(res, err, MediaSourceTrakt, q.Title)
		}, serieDBIDByTvdb)
	}
}

// serieEpisodes maps the parsed episodes of q to the episodes of the series
// dbid, and to their list entries of serieID. With dbid 0 (a series not yet
// in the database) only the parsed identifiers are returned.
func serieEpisodes(q *MediaQuery, dbid, serieID uint) []MediaEpisode {
	var out []MediaEpisode

	lookup := func(ident, query string, args ...any) {
		ep := MediaEpisode{Identifier: ident}

		if dbid != 0 {
			rows := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 1, query, args...)
			if len(rows) == 1 {
				ep.Title, ep.DBEpisodeID = rows[0].Str, rows[0].Num
			}
		}

		if ep.DBEpisodeID != 0 && serieID != 0 {
			database.Scanrowsdyn(false,
				"select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?",
				&ep.EpisodeID, &ep.DBEpisodeID, &serieID)

			if ep.EpisodeID != 0 {
				ep.Files = database.Getdatarow[int](false,
					"select count() from serie_episode_files where serie_episode_id = ?", &ep.EpisodeID)
			}
		}

		out = append(out, ep)
	}

	if len(q.Episodes) > 0 && q.Season != "" {
		season := logger.StringToInt(q.Season)
		for _, e := range q.Episodes {
			ident := fmt.Sprintf("S%02dE%02d", season, e)
			episode := strconv.Itoa(e)
			lookup(ident,
				"select coalesce(title, ''), id from dbserie_episodes where dbserie_id = ? and ((season = ? and episode = ?) or identifier = ? COLLATE NOCASE) limit 1",
				&dbid, &q.Season, &episode, &ident)
		}

		return out
	}

	if q.Identifier != "" {
		lookup(q.Identifier,
			"select coalesce(title, ''), id from dbserie_episodes where dbserie_id = ?1 and (identifier = ?2 COLLATE NOCASE or identifier = replace(?2, '.', ' ') COLLATE NOCASE or identifier = replace(?2, '.', '-') COLLATE NOCASE or identifier = replace(?2, ' ', '-') COLLATE NOCASE) limit 1",
			&dbid, &q.Identifier)
	}

	return out
}

// mediaListTable returns the list table of a media type and its column
// referencing the database row.
func mediaListTable(isType uint) (string, string) {
	switch isType {
	case config.MediaTypeMovie:
		return "movies", "dbmovie_id"
	case config.MediaTypeSeries:
		return "series", "dbserie_id"
	case config.MediaTypeBook:
		return "books", "dbbook_id"
	}

	return "", ""
}

// mediaListEntry returns the list entry of database row dbid in the first
// list of cfgp that has one, and that list's name.
func mediaListEntry(cfgp *config.MediaTypeConfig, dbid uint) (uint, string) {
	table, col := mediaListTable(cfgp.IsType)
	if table == "" {
		return 0, ""
	}

	query := "select id from " + table + " where " + col + " = ? and listname = ? COLLATE NOCASE"
	for i := range cfgp.Lists {
		var id uint
		database.Scanrowsdyn(false, query, &id, &dbid, &cfgp.Lists[i].Name)

		if id != 0 {
			return id, cfgp.Lists[i].Name
		}
	}

	return 0, ""
}

// ── Books ────────────────────────────────────────────────────────────────────

func bookDBIDByISBN(isbn string) uint {
	return BookFindDBIDByISBN(&isbn)
}

func firstOf(values []string) string {
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (col *mediaCollector) bookCandidates() {
	q := col.q
	ctx := col.ctx
	ol := providers.GetOpenLibrary()

	if q.ISBN != "" {
		if dbid := bookDBIDByISBN(q.ISBN); dbid != 0 {
			col.addDB(dbid, PreviewSourceDatabase)
		} else if ol != nil {
			col.addSearch(MediaSourceOpenLibrary, "isbn:"+q.ISBN, func() []mediaExt {
				d, err := ol.SearchByISBN(ctx, q.ISBN)
				if err != nil || d == nil {
					return nil
				}

				return []mediaExt{{
					id: coalesceStr(d.ISBN13, d.ISBN10, q.ISBN), title: d.Title,
					year: d.PublishYear, author: firstOf(d.Authors),
				}}
			}, bookDBIDByISBN)
		}
	}

	if q.Title == "" {
		return
	}

	slug := logger.StringToSlugCached(q.Title)
	col.addDBRows(
		"select id from dbbooks where title = ?1 COLLATE NOCASE or slug = ?2 union select dbbook_id from dbbook_titles where title = ?1 COLLATE NOCASE or slug = ?2 limit 10",
		&q.Title, &slug,
	)

	if ol == nil {
		return
	}

	col.addSearch(MediaSourceOpenLibrary, q.Author+"|"+q.Title, func() []mediaExt {
		results, err := ol.SearchBooks(ctx, q.Title, q.Author, mediaProviderLimit)
		if err != nil {
			logger.Logtype("debug", 1).Err(err).Str(logger.StrTitle, q.Title).Msg("match preview: openlibrary search failed")
			return nil
		}

		out := make([]mediaExt, 0, len(results))
		for _, r := range results[:min(len(results), mediaProviderLimit)] {
			out = append(out, mediaExt{
				id: coalesceStr(r.ISBN13, r.ISBN10), title: r.Title,
				year: r.PublishYear, author: firstOf(r.Authors),
			})
		}

		return out
	}, bookDBIDByISBN)
}

// ── Scoring ──────────────────────────────────────────────────────────────────

func yearString(y int) string {
	if y == 0 {
		return ""
	}

	return strconv.Itoa(y)
}

// mediaIDs returns the external id parsed from the file and the candidate's,
// normalized for comparison, and the id's display name.
func mediaIDs(q *MediaQuery, c *MediaCandidate) (string, string, string) {
	switch q.IsType {
	case config.MediaTypeMovie:
		have := q.Imdb
		if have != "" {
			have = logger.AddImdbPrefix(have)
		}

		return "IMDb", have, c.ExternalID
	case config.MediaTypeSeries:
		if q.Tvdb == 0 {
			return "TVDB", "", c.ExternalID
		}

		return "TVDB", strconv.Itoa(q.Tvdb), c.ExternalID
	default:
		norm := strings.NewReplacer("-", "", " ", "")

		return "ISBN", norm.Replace(q.ISBN), norm.Replace(c.ExternalID)
	}
}

// scoreMediaCandidate sets c.Dist and c.Fields. A matching external id is
// decisive: title and author no longer count, as a file named after a
// translated title still is the right movie.
func scoreMediaCandidate(q *MediaQuery, c *MediaCandidate) {
	d := newBeetsDistance()
	d.weightOverride = mediaWeights

	c.Fields = c.Fields[:0]

	idName, haveID, wantID := mediaIDs(q, c)
	idMatch := haveID != "" && strings.EqualFold(haveID, wantID)

	titleDist, matched := 1.0, c.Title
	for i, t := range append([]string{c.Title}, c.altTitles...) {
		if t == "" {
			continue
		}

		if dist := stringDist(q.Title, t); dist < titleDist || i == 0 && dist == titleDist {
			titleDist, matched = dist, t
		}
	}

	c.MatchedAs = ""
	if matched != c.Title {
		c.MatchedAs = matched
	}

	titleScored := q.Title != "" && !idMatch
	if titleScored {
		d.add("title", titleDist)
	}

	c.Fields = append(c.Fields, MediaField{
		Name: "Title", Have: q.Title, Wanted: c.Title, Dist: titleDist, Scored: titleScored,
	})

	if q.IsType == config.MediaTypeBook && (q.Author != "" || c.Author != "") {
		f := MediaField{Name: "Author", Have: q.Author, Wanted: c.Author}
		if q.Author != "" && c.Author != "" && !idMatch {
			f.Dist, f.Scored = stringDist(q.Author, c.Author), true
			d.add("author", f.Dist)
		}

		c.Fields = append(c.Fields, f)
	}

	yf := MediaField{Name: "Year", Have: yearString(q.Year), Wanted: yearString(c.Year)}
	if q.Year != 0 && c.Year != 0 {
		switch diff := q.Year - c.Year; {
		case diff == 0:
			yf.Dist = 0
		case diff == 1 || diff == -1:
			yf.Dist = 0.5
		default:
			yf.Dist = 1
		}

		yf.Scored = true
		d.add("year", yf.Dist)
	}

	c.Fields = append(c.Fields, yf)

	idf := MediaField{Name: idName, Have: haveID, Wanted: wantID}
	if haveID != "" {
		idf.Scored = true
		if !idMatch {
			idf.Dist = 1
		}

		d.add("id", idf.Dist)
	}

	c.Fields = append(c.Fields, idf)

	if q.IsType == config.MediaTypeSeries && len(c.Episodes) > 0 {
		ef := MediaField{Name: "Episodes", Scored: true}

		missing := 0
		for i, ep := range c.Episodes {
			if i > 0 {
				ef.Have += ", "
				ef.Wanted += ", "
			}

			ef.Have += ep.Identifier

			switch {
			case c.DBID == 0:
				ef.Wanted += "?"
			case ep.DBEpisodeID == 0:
				missing++

				ef.Wanted += "missing"
			default:
				ef.Wanted += ep.Identifier
				if ep.Title != "" {
					ef.Wanted += " " + ep.Title
				}
			}
		}

		// Episodes of a series not in the database are only known after
		// importing it.
		ef.Dist = float64(missing) / float64(len(c.Episodes))
		if c.DBID == 0 {
			ef.Dist = 0.5
		}

		d.add("episode", ef.Dist)
		c.Fields = append(c.Fields, ef)
	}

	c.Dist = d.distance()
}

// ── Import ───────────────────────────────────────────────────────────────────

// MediaImport is where ImportMediaCandidate put a candidate.
type MediaImport struct {
	MediaID  uint
	DBID     uint
	ListName string
}

// ImportMediaCandidate returns the list entry (movies.id, series.id,
// books.id) of c in cfgp, importing the movie, series or book into list
// listid first when c is not in any of cfgp's lists: by IMDb id, TVDB id or
// ISBN, as a list import would.
func ImportMediaCandidate(
	ctx context.Context,
	c *MediaCandidate,
	cfgp *config.MediaTypeConfig,
	listid int,
) (MediaImport, error) {
	if c.MediaID != 0 {
		return MediaImport{MediaID: c.MediaID, DBID: c.DBID, ListName: c.ListName}, nil
	}

	if c.ExternalID == "" {
		return MediaImport{}, errMediaNoID
	}

	if listid < 0 || listid >= len(cfgp.Lists) {
		return MediaImport{}, logger.ErrListnameEmpty
	}

	var (
		dbid uint
		err  error
	)

	switch cfgp.IsType {
	case config.MediaTypeMovie:
		dbid, err = JobImportMovies(c.ExternalID, cfgp, listid, true, false)
	case config.MediaTypeSeries:
		err = JobImportDBSeries(ctx, &config.ManualConfig{
			Name:   c.Title,
			TvdbID: logger.StringToInt(c.ExternalID),
		}, 0, cfgp, listid)
		dbid = serieDBIDByTvdb(c.ExternalID)
	case config.MediaTypeBook:
		dbid, err = JobImportBooks(ctx, c.ExternalID, cfgp, listid, true)
	default:
		return MediaImport{}, errPreviewNotImportable
	}

	if err != nil {
		return MediaImport{}, err
	}

	if dbid == 0 {
		return MediaImport{}, logger.ErrNotFound
	}

	mediaID, listName := mediaListEntry(cfgp, dbid)
	if mediaID == 0 {
		return MediaImport{}, fmt.Errorf("%s was imported but is in none of the lists of %s: %w",
			c.Title, cfgp.NamePrefix, logger.ErrNotFound)
	}

	return MediaImport{MediaID: mediaID, DBID: dbid, ListName: listName}, nil
}

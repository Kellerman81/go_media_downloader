package api

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// calendarEvent represents a calendar event for movies/series/albums/audiobooks.
type calendarEvent struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	Show       string    `json:"show,omitempty"`         // series name, Title without the episode title
	EpTitle    string    `json:"episodeTitle,omitempty"` // episode title, when known
	Type       string    `json:"type"`                   // "movie", "series", "album", or "audiobook"
	Date       time.Time `json:"date"`
	Year       int       `json:"year,omitempty"`
	Season     string    `json:"season,omitempty"`
	Episode    int       `json:"episode,omitempty"`
	Status     string    `json:"status"`
	Monitored  bool      `json:"monitored"`
	Downloaded bool      `json:"downloaded"`
	AirTime    string    `json:"airTime,omitempty"`
	Network    string    `json:"network,omitempty"`
	Overview   string    `json:"overview,omitempty"`
	PosterURL  string    `json:"posterUrl,omitempty"`
	IMDBRating float64   `json:"imdbRating,omitempty"`
	Runtime    int       `json:"runtime,omitempty"`
	IMDBID     string    `json:"imdbId,omitempty"`
	TheTVDBID  int       `json:"thetvdbId,omitempty"`
	MovieDBID  int       `json:"moviedbId,omitempty"`
	TraktID    int       `json:"traktId,omitempty"`
	Listname   string    `json:"listname,omitempty"`
	// Links are the external pages, built from the IDs the metadata
	// providers stored (shared with the Library: movieLinks, seriesLinks, ...).
	Links []calendarExtLink `json:"links,omitempty"`
	// ThumbURL is a frame of a downloaded episode's file.
	ThumbURL string `json:"thumbUrl,omitempty"`
	// LibraryURL opens the entry in the Library.
	LibraryURL string `json:"libraryUrl,omitempty"`
	// Overdue: released, monitored and still missing.
	Overdue bool `json:"overdue,omitempty"`
	// New: a file of it arrived in the last 24 hours.
	New bool `json:"new,omitempty"`
	// Entries are the list entries the event merges, one per media group
	// (list) the item is in, each with its own files and search.
	Entries []calendarEntry `json:"entries,omitempty"`
	// Partial: some entries have their file, others still want one.
	Partial bool `json:"partial,omitempty"`

	dbKey    string // the movie/episode/album/audiobook the row belongs to
	genre    string // for the genre filter
	entryID  uint   // the list entry the Library opens (series: the series)
	coverKey string // the db* id of the cover (series: the dbseries)
}

// calendarEntry is one list entry of an event (a movies, series episode, ...
// row): ID is what Play and Search take.
type calendarEntry struct {
	ID         uint   `json:"id"`
	Group      string `json:"group,omitempty"`
	List       string `json:"list,omitempty"`
	Downloaded bool   `json:"downloaded"`
	Monitored  bool   `json:"monitored"`
	LibraryURL string `json:"libraryUrl,omitempty"`
}

// calendarExtLink is an external page of an event.
type calendarExtLink struct {
	Label string `json:"label"`
	URL   string `json:"url"`
	Icon  string `json:"icon"`
}

// CalendarPageHandler renders the calendar page. The calendar area (toolbar
// and view) is one htmx fragment, CalendarContentHandler.
func CalendarPageHandler(c *gin.Context) {
	content := html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader(
			"fas fa-calendar-alt",
			"Calendar",
			"Release dates of the movies, episodes, albums, audiobooks and books in your lists",
		),
		html.Div(
			html.Class("card border-0 shadow-sm"),
			html.Div(
				html.Class("card-body"),
				html.Div(
					html.ID("calendar-content"),
					hx.Get("/api/admin/calendar/content?"+c.Request.URL.RawQuery),
					hx.Trigger("load"),
					html.Div(
						html.Class("text-center py-5"),
						html.Div(html.Class("spinner-border text-secondary"), gomponents.Attr("role", "status")),
						html.P(html.Class("mt-2 text-muted"), gomponents.Text("Loading calendar events...")),
					),
				),
			),
		),
		calendarModalsAndScripts(),
		html.Script(gomponents.Raw(fmt.Sprintf("window.calendarCanSearch = %t;", accessOf(c).CanSearch()))),
		// The Library's bottom-bar player, for "Play" in the event dialog.
		playerDock(),
		html.StyleEl(gomponents.Raw(playerCSS)),
		html.Script(gomponents.Raw(playerJS)),
	)

	pageNode := pageAs(accessOf(c),
		"Calendar",
		false, // activeConfig
		false, // activeDatabase
		false, // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// CalendarEventsHandler returns calendar events as JSON.
func CalendarEventsHandler(c *gin.Context) {
	events := getCalendarEventsFromQuery(c)
	c.JSON(http.StatusOK, events)
}

func getCalendarEventsFromQuery(c *gin.Context) []calendarEvent {
	start := c.Query("start")
	end := c.Query("end")
	eventType := c.DefaultQuery("type", "all")

	startDate, err := time.Parse("2006-01-02", start)
	if err != nil {
		startDate = time.Now().AddDate(0, 0, -30)
	}

	endDate, err := time.Parse("2006-01-02", end)
	if err != nil {
		endDate = time.Now().AddDate(0, 0, 60)
	}

	if endDate.Before(startDate) {
		endDate = startDate
	}

	if endDate.Sub(startDate) > maxCalendarRange {
		endDate = startDate.Add(maxCalendarRange)
	}

	events := accessOf(c).calendarEvents(getCalendarEvents(startDate, endDate, eventType))

	if events == nil {
		events = []calendarEvent{}
	}

	return events
}

// maxCalendarRange bounds one calendar events request.
const maxCalendarRange = 366 * 24 * time.Hour

// calendarState is what the calendar shows: the view, the media type filter,
// the date the view is anchored at and whether downloaded items are hidden.
// The page and the iCal download read it from the same query parameters, so
// the download contains exactly what is displayed.
type calendarState struct {
	View, Filter string
	Date         time.Time
	OnlyMissing  bool
	Group, Genre string // a media group (e.g. movie_EN) and a genre, "" for all
	access       *userAccess
}

var (
	calendarViews   = []string{"month", "week", "agenda"}
	calendarFilters = []string{"all", "movies", "series", "albums", "audiobooks", "books"}
)

// agendaDays is the length of the agenda view; it starts a week before its date.
const agendaDays = 60

func calendarStateFromQuery(c *gin.Context) calendarState {
	st := calendarState{View: c.Query("view"), Filter: c.Query("filter"), OnlyMissing: c.Query("missing") == "1",
		Group: c.Query("group"), Genre: strings.TrimSpace(c.Query("genre")), access: accessOf(c)}
	if st.Group != "" && (libGroupByName(st.Group) == nil || !st.access.AllowsGroup(st.Group)) {
		st.Group = ""
	}

	if !slices.Contains(calendarViews, st.View) {
		st.View = "month"
	}

	if !slices.Contains(calendarFilters, st.Filter) {
		st.Filter = "all"
	}

	now := time.Now()
	st.Date = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	if d, err := time.Parse("2006-01-02", c.Query("date")); err == nil {
		st.Date = d
	}

	return st
}

// Range returns the first and last day (inclusive) the view shows.
func (st calendarState) Range() (time.Time, time.Time) {
	switch st.View {
	case "month":
		first := time.Date(st.Date.Year(), st.Date.Month(), 1, 0, 0, 0, 0, time.UTC)
		return first, first.AddDate(0, 1, -1)
	case "week":
		first := st.Date.AddDate(0, 0, -int(st.Date.Weekday()))
		return first, first.AddDate(0, 0, 6)
	default:
		first := st.Date.AddDate(0, 0, -7)
		return first, first.AddDate(0, 0, agendaDays-1)
	}
}

// Step returns the anchor date of the previous (-1) or next (+1) period.
func (st calendarState) Step(dir int) time.Time {
	switch st.View {
	case "month":
		return time.Date(st.Date.Year(), st.Date.Month()+time.Month(dir), 1, 0, 0, 0, 0, time.UTC)
	case "week":
		return st.Date.AddDate(0, 0, 7*dir)
	default:
		return st.Date.AddDate(0, 0, agendaDays*dir)
	}
}

// Query returns the state as URL query, with the given changes applied.
func (st calendarState) Query(change func(*calendarState)) string {
	if change != nil {
		change(&st)
	}

	q := url.Values{"view": {st.View}, "filter": {st.Filter}, "date": {st.Date.Format("2006-01-02")}}
	if st.OnlyMissing {
		q.Set("missing", "1")
	}

	if st.Group != "" {
		q.Set("group", st.Group)
	}

	if st.Genre != "" {
		q.Set("genre", st.Genre)
	}

	return q.Encode()
}

// Events returns the events the state shows.
func (st calendarState) Events() []calendarEvent {
	start, end := st.Range()
	return st.filter(getCalendarEvents(start, end, st.Filter), true)
}

// filter applies the account's media groups, "only missing", the media
// group and (withGenre) the genre.
func (st calendarState) filter(events []calendarEvent, withGenre bool) []calendarEvent {
	if st.access != nil {
		events = st.access.calendarEvents(events)
	}

	g := libGroupByName(st.Group)

	if g != nil {
		// Within one media group an event shows only that group's entries.
		today := calendarToday()
		for i := range events {
			e := &events[i]
			if e.Type != g.Kind.Type {
				e.Entries = nil
				continue
			}

			e.Entries = slices.DeleteFunc(slices.Clone(e.Entries), func(en calendarEntry) bool {
				return !slices.ContainsFunc(g.Lists, func(gl string) bool { return strings.EqualFold(gl, en.List) })
			})
			if len(e.Entries) > 0 {
				e.settle(today)
			}
		}
	}

	return slices.DeleteFunc(events, func(e calendarEvent) bool {
		if g != nil && len(e.Entries) == 0 {
			return true
		}

		if st.OnlyMissing && e.Downloaded {
			return true
		}

		return withGenre && st.Genre != "" && !calendarHasGenre(e.genre, st.Genre)
	})
}

// calendarHasGenre reports whether a comma-separated genre list holds genre.
func calendarHasGenre(list, genre string) bool {
	for _, g := range strings.Split(list, ",") {
		if strings.EqualFold(strings.TrimSpace(g), genre) {
			return true
		}
	}

	return false
}

// calendarGenres lists the most common genres of events, by name.
func calendarGenres(events []calendarEvent) []string {
	lists := make([]string, 0, len(events))
	for _, e := range events {
		lists = append(lists, e.genre)
	}

	return commonGenres(lists, 60)
}

// CalendarICalHandler downloads the displayed events (same query as the page)
// as an iCalendar file.
func CalendarICalHandler(c *gin.Context) {
	st := calendarStateFromQuery(c)
	start, end := st.Range()

	name := fmt.Sprintf("media-calendar-%s-%s_%s.ics", st.Filter, start.Format("2006-01-02"), end.Format("2006-01-02"))

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename="+name)
	c.String(http.StatusOK, generateICalContent(st.Events(), calendarTitle(st)))
}

// calendarTitle names the exported calendar after its filter.
func calendarTitle(st calendarState) string {
	title := "Media Calendar"
	if st.Filter != "all" {
		title += " - " + cases.Title(language.English).String(st.Filter)
	}

	if st.OnlyMissing {
		title += " (missing)"
	}

	return title
}

// getCalendarEvents retrieves calendar events from the database.
func getCalendarEvents(startDate, endDate time.Time, eventType string) []calendarEvent {
	events := make([]calendarEvent, 0)

	// Get movie events
	if eventType == "all" || eventType == "movies" {
		movieEvents := getMovieCalendarEvents(startDate, endDate)

		events = append(events, movieEvents...)
	}

	// Get series events
	if eventType == "all" || eventType == "series" {
		seriesEvents := getSeriesCalendarEvents(startDate, endDate)

		events = append(events, seriesEvents...)
	}

	// Get album events
	if eventType == "all" || eventType == "albums" {
		albumEvents := getAlbumCalendarEvents(startDate, endDate)

		events = append(events, albumEvents...)
	}

	// Get audiobook events
	if eventType == "all" || eventType == "audiobooks" {
		audiobookEvents := getAudiobookCalendarEvents(startDate, endDate)

		events = append(events, audiobookEvents...)
	}

	if eventType == "all" || eventType == "books" {
		events = append(events, getBookCalendarEvents(startDate, endDate)...)
	}

	// Covers come from the Library's cover cache (with its fallbacks), and
	// each entry opens in the Library of its media group.
	groups := calendarGroupIndex()

	for i := range events {
		e := &events[i]
		if e.coverKey != "" {
			e.PosterURL = "/api/admin/library/cover/" + e.Type + "/" + e.coverKey
		}

		g := groups[e.Type+"|"+strings.ToLower(e.Listname)]
		if g != "" && e.entryID > 0 {
			e.LibraryURL = "/api/admin/library?group=" + url.QueryEscape(g) + "#" + e.Type + "/" + strconv.FormatUint(uint64(e.entryID), 10)
		}

		e.Entries = []calendarEntry{{
			ID: e.ID, Group: g, List: e.Listname, Downloaded: e.Downloaded,
			Monitored: e.Monitored, LibraryURL: e.LibraryURL,
		}}
	}

	return mergeCalendarEvents(events)
}

// calendarGroupIndex maps "type|list name" (lower case) to the media group
// the list belongs to.
func calendarGroupIndex() map[string]string {
	idx := map[string]string{}

	for _, g := range libraryGroups() {
		for _, l := range g.Lists {
			idx[g.Kind.Type+"|"+strings.ToLower(l)] = g.Name
		}
	}

	return idx
}

// mergeCalendarEvents turns the rows of one item that is in several lists
// (one movies/series/... row per list) into one event that keeps the rows as
// its entries; settle derives its state from them. Events are sorted by date
// and title.
func mergeCalendarEvents(events []calendarEvent) []calendarEvent {
	merged := make([]calendarEvent, 0, len(events))
	index := make(map[string]int, len(events))

	for _, e := range events {
		key := e.Type + ":" + e.dbKey
		if e.dbKey == "" {
			key = fmt.Sprintf("%s#%d", e.Type, e.ID)
		}

		if len(e.Entries) == 0 {
			e.Entries = []calendarEntry{{ID: e.ID, List: e.Listname, Downloaded: e.Downloaded, Monitored: e.Monitored, LibraryURL: e.LibraryURL}}
		}

		i, ok := index[key]
		if !ok {
			index[key] = len(merged)
			merged = append(merged, e)

			continue
		}

		merged[i].Entries = append(merged[i].Entries, e.Entries...)
	}

	today := calendarToday()
	for i := range merged {
		merged[i].settle(today)
	}

	slices.SortStableFunc(merged, func(a, b calendarEvent) int {
		if c := a.Date.Compare(b.Date); c != 0 {
			return c
		}

		return strings.Compare(a.Title, b.Title)
	})

	return merged
}

// settle derives the event's state from its entries. Downloaded means no
// monitored entry still misses its file; Partial means some entry has a file
// while another monitored one doesn't. Play, the Library link and the ID
// prefer an entry with a file.
func (e *calendarEvent) settle(today time.Time) {
	var anyFile, wanted bool

	e.Monitored = false
	lists := make([]string, 0, len(e.Entries))
	first := -1

	for i, en := range e.Entries {
		anyFile = anyFile || en.Downloaded
		wanted = wanted || (en.Monitored && !en.Downloaded)
		e.Monitored = e.Monitored || en.Monitored

		if en.Downloaded && first < 0 {
			first = i
		}

		if en.List != "" && !slices.Contains(lists, en.List) {
			lists = append(lists, en.List)
		}
	}

	e.Downloaded = anyFile && !wanted
	e.Partial = anyFile && wanted
	e.Listname = strings.Join(lists, ", ")

	e.Status = getMovieStatus(e.Downloaded, e.Monitored)
	if e.Partial {
		e.Status = "Partly downloaded"
	}

	if first < 0 {
		first = 0
	}

	if len(e.Entries) > 0 {
		e.ID = e.Entries[first].ID
		e.LibraryURL = e.Entries[first].LibraryURL
	}

	e.Overdue = wanted && e.Date.Before(today)
}

// MovieCalendarQuery represents the structure returned by movie calendar query.
type MovieCalendarQuery struct {
	ID          uint     `db:"id"`
	Title       string   `db:"title"`
	ReleaseDate string   `db:"release_date"`
	Year        int      `db:"year"`
	Overview    *string  `db:"overview"`
	IMDBRating  *float64 `db:"imdb_rating"`
	Runtime     *float64 `db:"runtime"`
	Downloaded  bool     `db:"downloaded"`
	Monitored   bool     `db:"monitored"`
	IMDBID      *string  `db:"imdb_id"`
	MovieDBID   *int     `db:"moviedb_id"`
	TraktID     *int     `db:"trakt_id"`
	Listname    string   `db:"listname"`
	DbID        uint     `db:"db_id"`
	Poster      *string  `db:"poster"`
	URL         string   `db:"url"`
	Genres      string   `db:"genres"`
	IsNew       bool     `db:"is_new"`
}

// getMovieCalendarEvents retrieves movie calendar events.
func getMovieCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	movieData := database.StructscanT[MovieCalendarQuery](false, 0, `
		SELECT m.id, m.dbmovie_id as db_id, dm.poster, dm.title, dm.release_date, dm.year, dm.overview, dm.vote_average as imdb_rating, dm.runtime, dm.imdb_id, dm.moviedb_id, dm.trakt_id,
			   COALESCE(dm.url, '') as url, COALESCE(dm.genres, '') as genres,
			   EXISTS (SELECT 1 FROM movie_files mf WHERE mf.movie_id = m.id) as downloaded,
			   EXISTS (SELECT 1 FROM movie_files mf WHERE mf.movie_id = m.id AND mf.created_at >= datetime('now', '-1 day')) as is_new,
			   CASE WHEN m.dont_search = 0 THEN 1 ELSE 0 END as monitored,
			   m.listname
		FROM movies m
		INNER JOIN dbmovies dm ON dm.id = m.dbmovie_id
		WHERE dm.release_date >= ? AND dm.release_date < ?
		ORDER BY dm.release_date ASC
	`, startDate.Format("2006-01-02"), endDate.AddDate(0, 0, 1).Format("2006-01-02"))

	for _, movie := range movieData {
		var (
			releaseDate time.Time
			err         error
		)

		releaseDate, err = time.Parse("2006-01-02", movie.ReleaseDate)
		if err != nil {
			releaseDate, err = time.Parse("2006-01-02T15:04:05Z", movie.ReleaseDate)
			if err != nil {
				releaseDate, err = time.Parse("2006-01-02 15:04:05", movie.ReleaseDate)
				if err != nil {
					continue
				}
			}
		}

		event := calendarEvent{
			ID:         movie.ID,
			Title:      movie.Title,
			Type:       "movie",
			Date:       releaseDate,
			Year:       movie.Year,
			Downloaded: movie.Downloaded,
			Monitored:  movie.Monitored,
			Status:     getMovieStatus(movie.Downloaded, movie.Monitored),
			Listname:   movie.Listname,
			dbKey:      strconv.FormatUint(uint64(movie.DbID), 10),
			New:        movie.IsNew,
			genre:      movie.Genres,
			entryID:    movie.ID,
			coverKey:   strconv.FormatUint(uint64(movie.DbID), 10),
		}

		if movie.Overview != nil {
			event.Overview = *movie.Overview
		}

		if movie.IMDBRating != nil {
			event.IMDBRating = *movie.IMDBRating
		}

		if movie.Runtime != nil {
			event.Runtime = int(*movie.Runtime)
		}

		if movie.IMDBID != nil {
			event.IMDBID = *movie.IMDBID
		}

		if movie.MovieDBID != nil {
			event.MovieDBID = *movie.MovieDBID
		}

		if movie.TraktID != nil {
			event.TraktID = *movie.TraktID
		}

		event.Links = movieLinks(event.IMDBID, event.MovieDBID, event.TraktID, movie.URL)

		events = append(events, event)
	}

	return events
}

// SeriesCalendarQuery represents the structure returned by series calendar query.
type SeriesCalendarQuery struct {
	ID           uint    `db:"id"`
	SeriesName   string  `db:"seriename"`
	FirstAired   *string `db:"first_aired"`
	Season       string  `db:"season"`
	Episode      int     `db:"episode"`
	EpisodeTitle *string `db:"episode_title"`
	Overview     *string `db:"overview"`
	Network      *string `db:"network"`
	Downloaded   bool    `db:"downloaded"`
	Monitored    bool    `db:"monitored"`
	TheTVDBID    *int    `db:"thetvdb_id"`
	TraktID      *int    `db:"trakt_id"`
	Listname     string  `db:"listname"`
	DbID         uint    `db:"db_id"`
	Poster       *string `db:"poster"`
	ScraperURL   string  `db:"scraper_url"`
	IMDBID       string  `db:"imdb_id"`
	Genre        string  `db:"genre"`
	SerieID      uint    `db:"serie_id"`
	DbSerieID    uint    `db:"dbserie_id"`
	FileID       uint    `db:"file_id"`
	IsNew        bool    `db:"is_new"`
}

// getSeriesCalendarEvents retrieves series calendar events.
func getSeriesCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	seriesData := database.StructscanT[SeriesCalendarQuery](false, 0, `
		SELECT se.id, se.dbserie_episode_id as db_id, ds.poster, ds.seriename, dse.first_aired,
			   CAST(dse.season AS INTEGER) as season, CAST(dse.episode AS INTEGER) as episode,
			   dse.title as episode_title, dse.overview, ds.network, ds.thetvdb_id, ds.trakt_id,
			   COALESCE(dse.scraper_url, '') as scraper_url, COALESCE(ds.imdb_id, '') as imdb_id, COALESCE(ds.genre, '') as genre,
			   s.id as serie_id, ds.id as dbserie_id,
			   COALESCE((SELECT sef.id FROM serie_episode_files sef WHERE sef.serie_episode_id = se.id ORDER BY sef.id LIMIT 1), 0) as file_id,
			   EXISTS (SELECT 1 FROM serie_episode_files sef WHERE sef.serie_episode_id = se.id AND sef.created_at >= datetime('now', '-1 day')) as is_new,
			   EXISTS (SELECT 1 FROM serie_episode_files sef WHERE sef.serie_episode_id = se.id) as downloaded,
			   CASE WHEN se.dont_search = 0 AND s.dont_search = 0 THEN 1 ELSE 0 END as monitored,
			   s.listname
		FROM serie_episodes se
		INNER JOIN series s ON s.id = se.serie_id
		INNER JOIN dbseries ds ON ds.id = s.dbserie_id
		INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id
		WHERE dse.first_aired >= ? AND dse.first_aired < ?
		ORDER BY dse.first_aired ASC
	`, startDate.Format("2006-01-02"), endDate.AddDate(0, 0, 1).Format("2006-01-02"))

	for _, series := range seriesData {
		if series.FirstAired == nil {
			continue
		}

		var (
			airDate time.Time
			err     error
		)

		airDate, err = time.Parse("2006-01-02", *series.FirstAired)
		if err != nil {
			airDate, err = time.Parse("2006-01-02T15:04:05Z", *series.FirstAired)
			if err != nil {
				airDate, err = time.Parse("2006-01-02 15:04:05", *series.FirstAired)
				if err != nil {
					continue
				}
			}
		}

		event := calendarEvent{
			ID:         series.ID,
			Title:      series.SeriesName,
			Type:       "series",
			Date:       airDate,
			Season:     series.Season,
			Episode:    series.Episode,
			Downloaded: series.Downloaded,
			Monitored:  series.Monitored,
			Status:     getSeriesStatus(series.Downloaded, series.Monitored),
			Listname:   series.Listname,
			dbKey:      strconv.FormatUint(uint64(series.DbID), 10),
			IMDBID:     series.IMDBID,
			New:        series.IsNew,
			genre:      series.Genre,
			entryID:    series.SerieID,
			coverKey:   strconv.FormatUint(uint64(series.DbSerieID), 10),
		}

		if series.FileID > 0 {
			event.ThumbURL = "/api/admin/library/thumb/episode/" + strconv.FormatUint(uint64(series.FileID), 10)
		}

		if series.EpisodeTitle != nil && strings.TrimSpace(*series.EpisodeTitle) != "" {
			event.Show = event.Title
			event.EpTitle = *series.EpisodeTitle
			event.Title = fmt.Sprintf("%s - %s", event.Title, *series.EpisodeTitle)
		}

		if series.Overview != nil {
			event.Overview = *series.Overview
		}

		if series.Network != nil {
			event.Network = *series.Network
		}

		if series.TheTVDBID != nil {
			event.TheTVDBID = *series.TheTVDBID
		}

		if series.TraktID != nil {
			event.TraktID = *series.TraktID
		}

		event.Links = seriesLinks(event.IMDBID, event.TheTVDBID, event.TraktID)

		// Episodes of scraper lists link the page they were scraped from.
		if libHTTPURL(series.ScraperURL) {
			event.Links = append(event.Links, calendarExtLink{Label: "Source", URL: series.ScraperURL, Icon: "fa-solid fa-globe"})
		}

		events = append(events, event)
	}

	return events
}

// AlbumCalendarQuery represents the structure returned by album calendar query.
type AlbumCalendarQuery struct {
	ID          uint    `db:"id"`
	Title       string  `db:"title"`
	ReleaseDate *string `db:"release_date"`
	Year        int     `db:"year"`
	Downloaded  bool    `db:"downloaded"`
	Monitored   bool    `db:"monitored"`
	Listname    string  `db:"listname"`
	DbID        uint    `db:"db_id"`
	Cover       *string `db:"cover_url"`
	Artist      string  `db:"artist"`
	MBGroup     string  `db:"mb_group"`
	MBRelease   string  `db:"mb_release"`
	DiscogsMstr string  `db:"discogs_master"`
	DiscogsRel  string  `db:"discogs_release"`
	Deezer      string  `db:"deezer"`
	Itunes      string  `db:"itunes"`
	Genres      string  `db:"genres"`
	IsNew       bool    `db:"is_new"`
}

// getAlbumCalendarEvents retrieves album calendar events.
func getAlbumCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	albumData := database.StructscanT[AlbumCalendarQuery](false, 0, `
		SELECT a.id, a.dbalbum_id as db_id, da.cover_url, da.title, da.release_date, da.year, a.listname,
			   COALESCE((SELECT ar.name FROM dbalbum_artists daa INNER JOIN dbartists ar ON ar.id = daa.dbartist_id
			             WHERE daa.dbalbum_id = da.id ORDER BY daa.position LIMIT 1), '') as artist,
			   COALESCE(da.musicbrainz_release_group_id, '') as mb_group, COALESCE(da.musicbrainz_release_id, '') as mb_release,
			   COALESCE(da.discogs_master_id, '') as discogs_master, COALESCE(da.discogs_release_id, '') as discogs_release,
			   COALESCE(da.deezer_id, '') as deezer, COALESCE(da.itunes_id, '') as itunes, COALESCE(da.genres, '') as genres,
			   EXISTS (SELECT 1 FROM album_files af WHERE af.album_id = a.id AND af.created_at >= datetime('now', '-1 day')) as is_new,
			   EXISTS (SELECT 1 FROM album_files af WHERE af.album_id = a.id) as downloaded,
			   CASE WHEN a.dont_search = 0 THEN 1 ELSE 0 END as monitored
		FROM albums a
		INNER JOIN dbalbums da ON da.id = a.dbalbum_id
		WHERE da.release_date >= ? AND da.release_date < ?
		ORDER BY da.release_date ASC
	`, startDate.Format("2006-01-02"), endDate.AddDate(0, 0, 1).Format("2006-01-02"))

	for _, album := range albumData {
		if album.ReleaseDate == nil {
			continue
		}

		var (
			releaseDate time.Time
			err         error
		)

		releaseDate, err = time.Parse("2006-01-02", *album.ReleaseDate)
		if err != nil {
			releaseDate, err = time.Parse("2006-01-02T15:04:05Z", *album.ReleaseDate)
			if err != nil {
				releaseDate, err = time.Parse("2006-01-02 15:04:05", *album.ReleaseDate)
				if err != nil {
					continue
				}
			}
		}

		events = append(events, calendarEvent{
			ID:         album.ID,
			Title:      album.Title,
			Type:       "album",
			Date:       releaseDate,
			Year:       album.Year,
			Downloaded: album.Downloaded,
			Monitored:  album.Monitored,
			Status:     getMovieStatus(album.Downloaded, album.Monitored),
			Listname:   album.Listname,
			dbKey:      strconv.FormatUint(uint64(album.DbID), 10),
			Network:    album.Artist,
			Links:      albumLinks(&album),
			New:        album.IsNew,
			genre:      album.Genres,
			entryID:    album.ID,
			coverKey:   strconv.FormatUint(uint64(album.DbID), 10),
		})
	}

	return events
}

// AudiobookCalendarQuery represents the structure returned by audiobook calendar query.
type AudiobookCalendarQuery struct {
	ID             uint    `db:"id"`
	Title          string  `db:"title"`
	ReleaseDate    *string `db:"release_date"`
	Year           int     `db:"year"`
	Description    *string `db:"description"`
	RuntimeMinutes *int    `db:"runtime_minutes"`
	Downloaded     bool    `db:"downloaded"`
	Monitored      bool    `db:"monitored"`
	Listname       string  `db:"listname"`
	DbID           uint    `db:"db_id"`
	Cover          *string `db:"cover_url"`
	Author         string  `db:"author"`
	ASIN           string  `db:"asin"`
	IsNew          bool    `db:"is_new"`
}

// getAudiobookCalendarEvents retrieves audiobook calendar events.
func getAudiobookCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	audiobookData := database.StructscanT[AudiobookCalendarQuery](false, 0, `
		SELECT ab.id, ab.dbaudiobook_id as db_id, dab.cover_url, dab.title, dab.release_date, dab.year, dab.description, dab.runtime_minutes, ab.listname,
			   COALESCE((SELECT au.name FROM dbaudiobook_authors daa INNER JOIN dbauthors au ON au.id = daa.dbauthor_id
			             WHERE daa.dbaudiobook_id = dab.id ORDER BY daa.position LIMIT 1), '') as author,
			   COALESCE(dab.asin, '') as asin,
			   EXISTS (SELECT 1 FROM audiobook_files abf WHERE abf.audiobook_id = ab.id AND abf.created_at >= datetime('now', '-1 day')) as is_new,
			   EXISTS (SELECT 1 FROM audiobook_files abf WHERE abf.audiobook_id = ab.id) as downloaded,
			   CASE WHEN ab.dont_search = 0 THEN 1 ELSE 0 END as monitored
		FROM audiobooks ab
		INNER JOIN dbaudiobooks dab ON dab.id = ab.dbaudiobook_id
		WHERE dab.release_date >= ? AND dab.release_date < ?
		ORDER BY dab.release_date ASC
	`, startDate.Format("2006-01-02"), endDate.AddDate(0, 0, 1).Format("2006-01-02"))

	for _, audiobook := range audiobookData {
		if audiobook.ReleaseDate == nil {
			continue
		}

		var (
			releaseDate time.Time
			err         error
		)

		releaseDate, err = time.Parse("2006-01-02", *audiobook.ReleaseDate)
		if err != nil {
			releaseDate, err = time.Parse("2006-01-02T15:04:05Z", *audiobook.ReleaseDate)
			if err != nil {
				releaseDate, err = time.Parse("2006-01-02 15:04:05", *audiobook.ReleaseDate)
				if err != nil {
					continue
				}
			}
		}

		event := calendarEvent{
			ID:         audiobook.ID,
			Title:      audiobook.Title,
			Type:       "audiobook",
			Date:       releaseDate,
			Year:       audiobook.Year,
			Downloaded: audiobook.Downloaded,
			Monitored:  audiobook.Monitored,
			Status:     getMovieStatus(audiobook.Downloaded, audiobook.Monitored),
			Listname:   audiobook.Listname,
			dbKey:      strconv.FormatUint(uint64(audiobook.DbID), 10),
			New:        audiobook.IsNew,
			entryID:    audiobook.ID,
			coverKey:   strconv.FormatUint(uint64(audiobook.DbID), 10),
			Network:    audiobook.Author,
			Links:      audiobookLinks(&audiobook, audibleRegionOf(audiobook.Listname)),
		}

		if audiobook.Description != nil {
			event.Overview = *audiobook.Description
		}

		if audiobook.RuntimeMinutes != nil {
			event.Runtime = *audiobook.RuntimeMinutes
		}

		events = append(events, event)
	}

	return events
}

// BookCalendarQuery represents the structure returned by the book calendar query.
type BookCalendarQuery struct {
	ID          uint    `db:"id"`
	DbID        uint    `db:"db_id"`
	Title       string  `db:"title"`
	PublishDate *string `db:"publish_date"`
	Year        int     `db:"year"`
	Description *string `db:"description"`
	Cover       *string `db:"cover_url"`
	Author      *string `db:"author"`
	Downloaded  bool    `db:"downloaded"`
	Monitored   bool    `db:"monitored"`
	Listname    string  `db:"listname"`
	ISBN        string  `db:"isbn"`
	OpenLibrary string  `db:"openlibrary"`
	Goodreads   string  `db:"goodreads"`
	Genres      string  `db:"genres"`
	IsNew       bool    `db:"is_new"`
}

// getBookCalendarEvents retrieves book calendar events by publish date.
func getBookCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	bookData := database.StructscanT[BookCalendarQuery](false, 0, `
		SELECT b.id, b.dbbook_id as db_id, db.title, db.publish_date, db.year, db.description, db.cover_url, b.listname,
			   (SELECT name FROM dbauthors WHERE id = db.dbauthor_id) as author,
			   COALESCE(db.isbn_13, '') as isbn, COALESCE(db.openlibrary_id, '') as openlibrary, COALESCE(db.goodreads_id, '') as goodreads,
			   COALESCE(db.genres, '') as genres,
			   EXISTS (SELECT 1 FROM book_files bf WHERE bf.book_id = b.id AND bf.created_at >= datetime('now', '-1 day')) as is_new,
			   EXISTS (SELECT 1 FROM book_files bf WHERE bf.book_id = b.id) as downloaded,
			   CASE WHEN b.dont_search = 0 THEN 1 ELSE 0 END as monitored
		FROM books b
		INNER JOIN dbbooks db ON db.id = b.dbbook_id
		WHERE db.publish_date >= ? AND db.publish_date < ?
		ORDER BY db.publish_date ASC
	`, startDate.Format("2006-01-02"), endDate.AddDate(0, 0, 1).Format("2006-01-02"))

	for _, book := range bookData {
		if book.PublishDate == nil {
			continue
		}

		date, ok := parseCalendarDate(*book.PublishDate)
		if !ok {
			continue
		}

		event := calendarEvent{
			ID:         book.ID,
			Title:      book.Title,
			Type:       "book",
			Date:       date,
			Year:       book.Year,
			Downloaded: book.Downloaded,
			Monitored:  book.Monitored,
			Status:     getMovieStatus(book.Downloaded, book.Monitored),
			Listname:   book.Listname,
			dbKey:      strconv.FormatUint(uint64(book.DbID), 10),
			New:        book.IsNew,
			genre:      book.Genres,
			entryID:    book.ID,
			coverKey:   strconv.FormatUint(uint64(book.DbID), 10),
		}

		if book.Author != nil {
			event.Network = *book.Author
		}

		event.Links = bookLinks(&book, event.Network)

		if book.Description != nil {
			event.Overview = *book.Description
		}

		events = append(events, event)
	}

	return events
}

// calendarID reports whether a stored provider ID is set ("0" is unset).
func calendarID(id string) bool {
	id = strings.TrimSpace(id)
	return id != "" && id != "0"
}

// movieLinks are a movie's pages on IMDb, TMDB and Trakt and its website
// (calendar and Library).
func movieLinks(imdb string, tmdb, trakt int, website string) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, URL: u, Icon: icon}) }

	if calendarID(imdb) {
		add("IMDb", "fa-brands fa-imdb", "https://www.imdb.com/title/"+url.PathEscape(imdb))
	}

	if tmdb > 0 {
		add("TMDB", "fa-solid fa-film", "https://www.themoviedb.org/movie/"+strconv.Itoa(tmdb))
	}

	if trakt > 0 {
		add("Trakt", "fa-solid fa-circle-play", "https://trakt.tv/movies/"+strconv.Itoa(trakt))
	}

	if libHTTPURL(website) {
		add("Website", "fa-solid fa-globe", website)
	}

	return links
}

// seriesLinks are a series' pages on IMDb, TheTVDB and Trakt.
func seriesLinks(imdb string, tvdb, trakt int) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, URL: u, Icon: icon}) }

	if calendarID(imdb) {
		add("IMDb", "fa-brands fa-imdb", "https://www.imdb.com/title/"+url.PathEscape(imdb))
	}

	if tvdb > 0 {
		add("TheTVDB", "fa-solid fa-tv", "https://thetvdb.com/dereferrer/series/"+strconv.Itoa(tvdb))
	}

	if trakt > 0 {
		add("Trakt", "fa-solid fa-circle-play", "https://trakt.tv/shows/"+strconv.Itoa(trakt))
	}

	return links
}

func albumLinks(a *AlbumCalendarQuery) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, URL: u, Icon: icon}) }

	switch {
	case calendarID(a.MBGroup):
		add("MusicBrainz", "fa-solid fa-compact-disc", "https://musicbrainz.org/release-group/"+url.PathEscape(a.MBGroup))
	case calendarID(a.MBRelease):
		add("MusicBrainz", "fa-solid fa-compact-disc", "https://musicbrainz.org/release/"+url.PathEscape(a.MBRelease))
	}

	switch {
	case calendarID(a.DiscogsMstr):
		add("Discogs", "fa-solid fa-record-vinyl", "https://www.discogs.com/master/"+url.PathEscape(a.DiscogsMstr))
	case calendarID(a.DiscogsRel):
		add("Discogs", "fa-solid fa-record-vinyl", "https://www.discogs.com/release/"+url.PathEscape(a.DiscogsRel))
	}

	if calendarID(a.Deezer) {
		add("Deezer", "fa-solid fa-headphones", "https://www.deezer.com/album/"+url.PathEscape(a.Deezer))
	}

	if calendarID(a.Itunes) {
		add("Apple Music", "fa-brands fa-apple", "https://music.apple.com/album/"+url.PathEscape(a.Itunes))
	}

	// Last.fm pages are addressed by name, so they need no stored ID.
	if a.Artist != "" {
		add("Last.fm", "fa-brands fa-lastfm", "https://www.last.fm/music/"+url.QueryEscape(a.Artist)+"/"+url.QueryEscape(a.Title))
	}

	return links
}

// audibleSites maps the audible_region setting to the store's site.
var audibleSites = map[string]string{
	"us": "www.audible.com", "uk": "www.audible.co.uk", "ca": "www.audible.ca", "au": "www.audible.com.au",
	"de": "www.audible.de", "fr": "www.audible.fr", "it": "www.audible.it", "es": "www.audible.es",
	"in": "www.audible.in", "jp": "www.audible.co.jp",
}

// audibleRegionOf returns the audible_region of the media group a list
// belongs to.
func audibleRegionOf(listname string) string {
	if media, _ := findMediaConfigByList(logger.StrAudiobook, listname); media != nil {
		return media.AudibleRegion
	}

	return ""
}

func audiobookLinks(a *AudiobookCalendarQuery, region string) []calendarExtLink {
	var links []calendarExtLink

	if calendarID(a.ASIN) {
		site := audibleSites[strings.ToLower(strings.TrimSpace(region))]
		if site == "" {
			site = audibleSites["us"]
		}

		links = append(links, calendarExtLink{Label: "Audible", Icon: "fa-brands fa-audible", URL: "https://" + site + "/pd/" + url.PathEscape(a.ASIN)})
	}

	links = append(links, calendarExtLink{Label: "Goodreads", Icon: "fa-brands fa-goodreads",
		URL: "https://www.goodreads.com/search?q=" + url.QueryEscape(strings.TrimSpace(a.Title+" "+a.Author))})

	return links
}

func bookLinks(b *BookCalendarQuery, author string) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, URL: u, Icon: icon}) }

	switch {
	case calendarID(b.OpenLibrary):
		add("Open Library", "fa-solid fa-book-open", "https://openlibrary.org/works/"+url.PathEscape(b.OpenLibrary))
	case calendarID(b.ISBN):
		add("Open Library", "fa-solid fa-book-open", "https://openlibrary.org/isbn/"+url.PathEscape(b.ISBN))
	}

	switch {
	case calendarID(b.Goodreads):
		add("Goodreads", "fa-brands fa-goodreads", "https://www.goodreads.com/book/show/"+url.PathEscape(b.Goodreads))
	case calendarID(b.ISBN):
		add("Goodreads", "fa-brands fa-goodreads", "https://www.goodreads.com/search?q="+url.QueryEscape(b.ISBN))
	default:
		add("Goodreads", "fa-brands fa-goodreads", "https://www.goodreads.com/search?q="+url.QueryEscape(strings.TrimSpace(b.Title+" "+author)))
	}

	return links
}

// parseCalendarDate parses the date formats the release date columns hold.
func parseCalendarDate(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", "2006-01-02 15:04:05", time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}

	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

// calendarImageURL turns a stored poster or cover into a URL: full URLs as
// they are, relative TMDB/TVDB paths behind base.
func calendarImageURL(path *string, base string) string {
	if path == nil || *path == "" {
		return ""
	}

	p := *path
	if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") || base == "" {
		if strings.HasPrefix(p, "http") {
			return p
		}

		return ""
	}

	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(p, "/")
}

// getMovieStatus returns the status string for a movie.
func getMovieStatus(downloaded, monitored bool) string {
	if downloaded {
		return "Downloaded"
	}

	if monitored {
		return "Monitored"
	}

	return "Unmonitored"
}

// getSeriesStatus returns the status string for a series episode.
func getSeriesStatus(downloaded, monitored bool) string {
	if downloaded {
		return "Downloaded"
	}

	if monitored {
		return "Monitored"
	}

	return "Unmonitored"
}

// generateICalContent renders events as an iCalendar (RFC 5545) file: one
// all-day event per item.
func generateICalContent(events []calendarEvent, calName string) string {
	var ical strings.Builder

	line := func(format string, args ...any) {
		ical.WriteString(foldICalLine(fmt.Sprintf(format, args...)))
		ical.WriteString("\r\n")
	}

	caser := cases.Title(language.English)
	stamp := time.Now().UTC().Format("20060102T150405Z")

	line("BEGIN:VCALENDAR")
	line("VERSION:2.0")
	line("PRODID:-//Go Media Downloader//Calendar//EN")
	line("CALSCALE:GREGORIAN")
	line("METHOD:PUBLISH")
	line("X-WR-CALNAME:%s", escapeICalText(calName))
	line("X-WR-CALDESC:Release dates from Go Media Downloader")

	for _, event := range events {
		title := event.Title
		if ep := episodeCode(event); ep != "" {
			title += " " + ep
		}

		desc := []string{"Type: " + caser.String(event.Type), "Status: " + event.Status}
		if event.Listname != "" {
			desc = append(desc, "Lists: "+event.Listname)
		}

		if event.Network != "" {
			desc = append(desc, "Network: "+event.Network)
		}

		if event.IMDBRating > 0 {
			desc = append(desc, fmt.Sprintf("Rating: %.1f", event.IMDBRating))
		}

		if event.Runtime > 0 {
			desc = append(desc, fmt.Sprintf("Runtime: %d minutes", event.Runtime))
		}

		if event.Overview != "" {
			desc = append(desc, "", event.Overview)
		}

		category := caser.String(event.Type)
		if event.Downloaded {
			category += ",Downloaded"
		} else if event.Partial {
			category += ",Partly downloaded"
		} else if event.Monitored {
			category += ",Monitored"
		}

		uid := event.dbKey
		if uid == "" {
			uid = strconv.FormatUint(uint64(event.ID), 10)
		}

		line("BEGIN:VEVENT")
		line("UID:%s-%s@media-downloader", event.Type, uid)
		line("DTSTAMP:%s", stamp)
		line("DTSTART;VALUE=DATE:%s", event.Date.Format("20060102"))
		line("DTEND;VALUE=DATE:%s", event.Date.AddDate(0, 0, 1).Format("20060102"))
		line("SUMMARY:%s", escapeICalText(title))
		line("DESCRIPTION:%s", escapeICalText(strings.Join(desc, "\n")))
		line("CATEGORIES:%s", category)
		line("TRANSP:TRANSPARENT")
		line("END:VEVENT")
	}

	line("END:VCALENDAR")

	return ical.String()
}

// episodeCode returns "S01E02" for a series event with season and episode.
func episodeCode(e calendarEvent) string {
	if e.Type != "series" || e.Season == "" || e.Season == "0" || e.Episode <= 0 {
		return ""
	}

	return fmt.Sprintf("S%02dE%02d", logger.StringToInt(e.Season), e.Episode)
}

// escapeICalText escapes a TEXT value: backslash, comma and semicolon, and
// line breaks as the two characters \n.
func escapeICalText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, "\r\n", "\\n")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, "\r", "\\n")

	return text
}

// foldICalLine folds a content line into lines of at most 75 octets, each
// continuation starting with a space, without splitting a UTF-8 character.
func foldICalLine(s string) string {
	const limit = 75
	if len(s) <= limit {
		return s
	}

	var b strings.Builder

	width := limit
	for len(s) > width {
		cut := width
		for cut > 0 && !utf8.RuneStart(s[cut]) {
			cut--
		}

		b.WriteString(s[:cut])
		b.WriteString("\r\n ")

		s = s[cut:]
		width = limit - 1 // the leading space counts
	}

	b.WriteString(s)

	return b.String()
}

// truncateText shortens text to at most maxLen characters (not bytes, so a
// multi-byte character is never cut in half).
func truncateText(text string, maxLen int) string {
	if utf8.RuneCountInString(text) <= maxLen {
		return text
	}

	return string([]rune(text)[:maxLen]) + "…"
}

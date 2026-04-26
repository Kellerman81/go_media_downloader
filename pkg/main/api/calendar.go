package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// calendarEvent represents a calendar event for movies/series/albums/audiobooks.
type calendarEvent struct {
	ID         uint      `json:"id"`
	Title      string    `json:"title"`
	Type       string    `json:"type"` // "movie", "series", "album", or "audiobook"
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
}

// CalendarPageHandler renders the calendar page.
func CalendarPageHandler(c *gin.Context) {
	content := html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-calendar-alt header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Calendar")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Manage your movies, series, albums and audiobooks release schedule",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Calendar controls using HTMX
			html.Div(
				html.ID("calendar-controls"),
				gomponents.Attr("hx-get", "/api/admin/calendar/controls"),
				gomponents.Attr("hx-trigger", "load"),
				gomponents.Attr("hx-swap", "innerHTML"),
			),

			// Calendar container
			html.Div(
				html.Class("card shadow-sm mt-3"),
				html.Div(
					html.Class("card-body"),
					html.Div(
						html.ID("calendar-content"),
						gomponents.Attr(
							"hx-get",
							"/api/admin/calendar/content?view=agenda&filter=all",
						),
						gomponents.Attr("hx-trigger", "load"),
						gomponents.Attr("hx-swap", "innerHTML"),
						// Loading placeholder
						html.Div(
							html.Class("text-center py-5"),
							html.Div(
								html.Class("spinner-border text-secondary"),
								gomponents.Attr("role", "status"),
							),
							html.P(
								html.Class("mt-2 text-muted"),
								gomponents.Text("Loading calendar events..."),
							),
						),
					),
				),
			),

			// iCal export link
			html.Div(
				html.Class("mt-4"),
				html.A(
					html.Href("/api/admin/calendar/ical"),
					html.Target("_blank"),
					html.Class("btn btn-outline-secondary"),
					html.I(html.Class("fas fa-calendar-alt me-2")),
					gomponents.Text("Export to Calendar (iCal)"),
				),
			),
		),

		// API key for search functionality
		html.Script(
			gomponents.Raw(`window.calendarApiKey = "`+config.GetSettingsGeneral().WebAPIKey+`";`),
		),

		// Calendar modals and scripts
		calendarModalsAndScripts(),
	)

	// Wrap in full page layout
	pageNode := page(
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

	events := getCalendarEvents(startDate, endDate, eventType)

	if events == nil {
		events = []calendarEvent{}
	}

	return events
}

// CalendarICalHandler exports calendar events as iCal format.
func CalendarICalHandler(c *gin.Context) {
	startDate := time.Now().AddDate(0, 0, -30)
	endDate := time.Now().AddDate(0, 0, 180) // 6 months ahead

	events := getCalendarEvents(startDate, endDate, "all")

	// Generate iCal content
	ical := generateICalContent(events)

	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=media_calendar.ics")
	c.String(http.StatusOK, ical)
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

	return events
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
}

// getMovieCalendarEvents retrieves movie calendar events.
func getMovieCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	movieData := database.StructscanT[MovieCalendarQuery](false, 0, `
		SELECT m.id, dm.title, dm.release_date, dm.year, dm.overview, dm.vote_average as imdb_rating, dm.runtime, dm.imdb_id, dm.moviedb_id, dm.trakt_id,
			   CASE WHEN mf.id IS NOT NULL THEN 1 ELSE 0 END as downloaded,
			   CASE WHEN m.dont_search = 0 THEN 1 ELSE 0 END as monitored,
			   m.listname
		FROM movies m
		INNER JOIN dbmovies dm ON dm.id = m.dbmovie_id
		LEFT JOIN movie_files mf ON mf.movie_id = m.id
		WHERE dm.release_date BETWEEN ? AND ?
		ORDER BY dm.release_date ASC
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

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
}

// getSeriesCalendarEvents retrieves series calendar events.
func getSeriesCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	seriesData := database.StructscanT[SeriesCalendarQuery](false, 0, `
		SELECT se.id, ds.seriename, dse.first_aired, dse.season, dse.episode,
			   dse.title as episode_title, dse.overview, ds.network, ds.thetvdb_id, ds.trakt_id,
			   CASE WHEN sef.id IS NOT NULL THEN 1 ELSE 0 END as downloaded,
			   CASE WHEN se.dont_search = 0 THEN 1 ELSE 0 END as monitored,
			   s.listname
		FROM serie_episodes se
		INNER JOIN series s ON s.id = se.serie_id
		INNER JOIN dbseries ds ON ds.id = s.dbserie_id
		INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id
		LEFT JOIN serie_episode_files sef ON sef.serie_episode_id = se.id
		WHERE dse.first_aired BETWEEN ? AND ?
		ORDER BY dse.first_aired ASC
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

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
		}

		if series.EpisodeTitle != nil {
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
}

// getAlbumCalendarEvents retrieves album calendar events.
func getAlbumCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	albumData := database.StructscanT[AlbumCalendarQuery](false, 0, `
		SELECT a.id, da.title, da.release_date, da.year, a.listname,
			   CASE WHEN af.id IS NOT NULL THEN 1 ELSE 0 END as downloaded,
			   CASE WHEN a.dont_search = 0 THEN 1 ELSE 0 END as monitored
		FROM albums a
		INNER JOIN dbalbums da ON da.id = a.dbalbum_id
		LEFT JOIN album_files af ON af.album_id = a.id
		WHERE da.release_date BETWEEN ? AND ?
		ORDER BY da.release_date ASC
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

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
}

// getAudiobookCalendarEvents retrieves audiobook calendar events.
func getAudiobookCalendarEvents(startDate, endDate time.Time) []calendarEvent {
	var events []calendarEvent

	audiobookData := database.StructscanT[AudiobookCalendarQuery](false, 0, `
		SELECT ab.id, dab.title, dab.release_date, dab.year, dab.description, dab.runtime_minutes, ab.listname,
			   CASE WHEN abf.id IS NOT NULL THEN 1 ELSE 0 END as downloaded,
			   CASE WHEN ab.dont_search = 0 THEN 1 ELSE 0 END as monitored
		FROM audiobooks ab
		INNER JOIN dbaudiobooks dab ON dab.id = ab.dbaudiobook_id
		LEFT JOIN audiobook_files abf ON abf.audiobook_id = ab.id
		WHERE dab.release_date BETWEEN ? AND ?
		ORDER BY dab.release_date ASC
	`, startDate.Format("2006-01-02"), endDate.Format("2006-01-02"))

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

// generateICalContent generates iCal format content for calendar events.
func generateICalContent(events []calendarEvent) string {
	var ical strings.Builder

	ical.WriteString("BEGIN:VCALENDAR\r\n")
	ical.WriteString("VERSION:2.0\r\n")
	ical.WriteString("PRODID:-//Go Media Downloader//Calendar//EN\r\n")
	ical.WriteString("CALSCALE:GREGORIAN\r\n")
	ical.WriteString("METHOD:PUBLISH\r\n")
	ical.WriteString("X-WR-CALNAME:Media Calendar\r\n")
	ical.WriteString("X-WR-CALDESC:Movies and TV Series Release Calendar\r\n")
	ical.WriteString("X-WR-TIMEZONE:UTC\r\n")

	for _, event := range events {
		ical.WriteString("BEGIN:VEVENT\r\n")
		fmt.Fprintf(&ical, "UID:%s-%d@media-downloader\r\n", event.Type, event.ID)
		fmt.Fprintf(&ical, "DTSTAMP:%s\r\n", time.Now().UTC().Format("20060102T150405Z"))
		fmt.Fprintf(&ical, "DTSTART;VALUE=DATE:%s\r\n", event.Date.Format("20060102"))

		title := event.Title
		if event.Type == "series" && event.Season != "0" && event.Season != "" &&
			event.Episode > 0 {
			title = fmt.Sprintf(
				"%s S%02dE%02d",
				title,
				logger.StringToInt(event.Season),
				event.Episode,
			)
		}

		fmt.Fprintf(&ical, "SUMMARY:%s\r\n", escapeICalText(title))

		caser := cases.Title(language.English)

		desc := fmt.Sprintf("Type: %s\\nStatus: %s", caser.String(event.Type), event.Status)
		if event.Overview != "" {
			desc += "\\n\\n" + event.Overview
		}

		if event.IMDBRating > 0 {
			desc += fmt.Sprintf("\\nIMDB Rating: %.1f", event.IMDBRating)
		}

		if event.Runtime > 0 {
			desc += fmt.Sprintf("\\nRuntime: %d minutes", event.Runtime)
		}

		if event.Network != "" {
			desc += "\\nNetwork: " + event.Network
		}

		fmt.Fprintf(&ical, "DESCRIPTION:%s\r\n", escapeICalText(desc))

		category := caser.String(event.Type)
		if event.Downloaded {
			category += ",Downloaded"
		} else if event.Monitored {
			category += ",Monitored"
		}

		fmt.Fprintf(&ical, "CATEGORIES:%s\r\n", category)
		ical.WriteString("END:VEVENT\r\n")
	}

	ical.WriteString("END:VCALENDAR\r\n")

	return ical.String()
}

// escapeICalText escapes special characters for iCal format.
func escapeICalText(text string) string {
	text = strings.ReplaceAll(text, "\\", "\\\\")
	text = strings.ReplaceAll(text, ",", "\\,")
	text = strings.ReplaceAll(text, ";", "\\;")
	text = strings.ReplaceAll(text, "\n", "\\n")
	text = strings.ReplaceAll(text, "\r", "\\r")

	return text
}

// truncateText truncates text to the specified length.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	return text[:maxLen] + "..."
}

package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderMovieMetadataPage renders a page for testing movie metadata lookup.
func renderMovieMetadataPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-info-circle header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Media Metadata Lookup")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Lookup metadata from various providers (IMDB, TMDB, OMDB, Trakt, TVDB). Support for movies, TV series, and episodes with comprehensive information retrieval.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("metadataTestForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Media Type & Identification"),
					),

					renderFormGroup("metadata", map[string]string{
						"MediaType": "Select the type of media to lookup",
					}, map[string]string{
						"MediaType": "Media Type",
					}, "MediaType", "select", "movie", map[string][]string{
						"options": {"movie", "series", "episode"},
					}),

					renderFormGroup("metadata", map[string]string{
						"ImdbID": "Enter an IMDB ID (e.g., 'tt0133093' for The Matrix)",
					}, map[string]string{
						"ImdbID": "IMDB ID",
					}, "ImdbID", "text", "", nil),

					html.Div(
						html.ID("tvdbFields"),
						html.Style("display: none;"),
						renderFormGroup("metadata", map[string]string{
							"TvdbID": "Enter a TVDB ID for series/episodes (e.g., '81189' for Breaking Bad)",
						}, map[string]string{
							"TvdbID": "TVDB ID",
						}, "TvdbID", "text", "", nil),
					),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Additional Parameters"),
					),

					html.Div(
						html.ID("episodeFields"),
						html.Style("display: none;"),

						renderFormGroup("metadata", map[string]string{
							"Season": "Season number for episode lookup",
						}, map[string]string{
							"Season": "Season Number",
						}, "Season", "number", "", nil),

						renderFormGroup("metadata", map[string]string{
							"Episode": "Episode number for episode lookup",
						}, map[string]string{
							"Episode": "Episode Number",
						}, "Episode", "number", "", nil),
					),

					renderFormGroup("metadata", map[string]string{
						"Provider": "Select metadata provider (leave empty for default behavior)",
					}, map[string]string{
						"Provider": "Metadata Provider",
					}, "Provider", "select", "", map[string][]string{
						"options": {"", "imdb", "tmdb", "omdb", "trakt", "tvdb"},
					}),

					renderFormGroup("metadata", map[string]string{
						"UpdateDB": "Update the database with retrieved metadata",
					}, map[string]string{
						"UpdateDB": "Update Database",
					}, "UpdateDB", "checkbox", false, nil),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class(ClassBtnPrimary),
					gomponents.Text("Get Metadata"),
					html.Type("button"),
					hx.Target("#metadataResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/moviemetadata"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Include("#metadataTestForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('metadataTestForm').reset(); document.getElementById('metadataResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("metadataResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-search me-1")),
						gomponents.Text("Instructions"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Metadata Lookup Instructions"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Follow these guidelines to lookup metadata for your media"),
				),
				html.Ul(
					html.Class("list-unstyled mb-3"),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-success me-2"),
							html.I(html.Class("fas fa-film me-1")),
							gomponents.Text("Movies"),
						),
						gomponents.Text("Use IMDB ID (e.g., 'tt0133093' for The Matrix)"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-info me-2"),
							html.I(html.Class("fas fa-tv me-1")),
							gomponents.Text("Series"),
						),
						gomponents.Text(
							"Use IMDB ID or TVDB ID (e.g., TVDB '81189' for Breaking Bad)",
						),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-warning me-2"),
							html.I(html.Class("fas fa-play-circle me-1")),
							gomponents.Text("Episodes"),
						),
						gomponents.Text("Use IMDB ID or TVDB ID plus season and episode numbers"),
					),
				),

				html.Div(
					html.Class("alert alert-light border-0 mt-3 mb-3"),
					html.Style(
						"background-color: rgba(13, 110, 253, 0.1); border-radius: 8px; padding: 0.75rem 1rem;",
					),
					html.Div(
						html.Class("d-flex align-items-start"),
						html.I(
							html.Class("fas fa-database me-2 mt-1"),
							html.Style("color: #0d6efd; font-size: 0.9rem;"),
						),
						html.Div(
							html.Strong(
								html.Style("color: #0d6efd;"),
								gomponents.Text("Providers: "),
							),
							gomponents.Text(
								"Different providers offer different information. IMDB and TMDB are best for movies, TVDB is essential for TV series.",
							),
						),
					),
				),

				html.P(
					html.Class("mb-0"),
					html.Strong(gomponents.Text("Update Database: ")),
					gomponents.Text(
						"Check this option to save retrieved metadata to your local database.",
					),
				),
			),
		),

		// JavaScript for dynamic field visibility
		// Simplified JavaScript for Metadata - CSS classes handle field visibility via HTMX
		html.Script(gomponents.Raw(`
			// No JavaScript needed - HTMX and CSS handle field visibility
		`)),
	)
}

// HandleMovieMetadata handles media metadata lookup requests (movies, series, episodes).
func HandleMovieMetadata(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	mediaType := c.PostForm("metadata_MediaType")
	imdbID := c.PostForm("metadata_ImdbID")
	tvdbID := c.PostForm("metadata_TvdbID")
	provider := c.PostForm("metadata_Provider")
	updateDB, _ := strconv.ParseBool(c.PostForm("metadata_UpdateDB"))

	// Validate required fields based on media type
	switch mediaType {
	case "movie":
		if imdbID == "" {
			c.String(
				http.StatusOK,
				renderAlert("Please enter an IMDB ID for movie lookup", "warning"),
			)

			return
		}

		if !strings.HasPrefix(imdbID, "tt") || len(imdbID) < 9 {
			c.String(
				http.StatusOK,
				renderAlert("Invalid IMDB ID format. Expected format: tt0133093", "warning"),
			)

			return
		}

	case "series":
		if imdbID == "" && tvdbID == "" {
			c.String(
				http.StatusOK,
				renderAlert(
					"Please enter either an IMDB ID or TVDB ID for series lookup",
					"warning",
				),
			)

			return
		}

	case "episode":
		seasonStr := c.PostForm("metadata_Season")

		episodeStr := c.PostForm("metadata_Episode")
		if imdbID == "" && tvdbID == "" {
			c.String(
				http.StatusOK,
				renderAlert(
					"Please enter either an IMDB ID or TVDB ID for episode lookup",
					"warning",
				),
			)

			return
		}

		if seasonStr == "" || episodeStr == "" {
			c.String(
				http.StatusOK,
				renderAlert(
					"Please enter both season and episode numbers for episode lookup",
					"warning",
				),
			)

			return
		}

	default:
		c.String(http.StatusOK, renderAlert("Invalid media type", "danger"))
		return
	}

	// Process based on media type
	switch mediaType {
	case "movie":
		c.String(http.StatusOK, handleMovieMetadataLookup(imdbID, provider, updateDB))
	case "series":
		c.String(http.StatusOK, handleSeriesMetadataLookup(imdbID, tvdbID, provider, updateDB))
	case "episode":
		seasonNum, err := strconv.Atoi(c.PostForm("metadata_Season"))
		if err != nil {
			logger.Logtype("error", 1).
				Str("value", c.PostForm("metadata_Season")).
				Err(err).
				Msg("Failed to parse season number")
		}

		episodeNum, err := strconv.Atoi(c.PostForm("metadata_Episode"))
		if err != nil {
			logger.Logtype("error", 1).
				Str("value", c.PostForm("metadata_Episode")).
				Err(err).
				Msg("Failed to parse episode number")
		}

		c.String(
			http.StatusOK,
			handleEpisodeMetadataLookup(imdbID, tvdbID, seasonNum, episodeNum, provider, updateDB),
		)
	}
}

// handleMovieMetadataLookup handles movie metadata lookup.
func handleMovieMetadataLookup(imdbID, provider string, updateDB bool) string {
	result := map[string]any{
		"media_type": "movie",
		"imdb_id":    imdbID,
		"provider":   provider,
		"update_db":  updateDB,
	}

	var imdb, omdb, tmdb, trakt bool

	var dbmovie database.Dbmovie

	dbmovie.ImdbID = imdbID

	switch provider {
	case logger.StrImdb:
		imdb = true
	case "omdb":
		omdb = true
	case "tmdb":
		tmdb = true
	case "trakt":
		trakt = true
	default:
		imdb = true
		omdb = true
		tmdb = true
		trakt = true
	}

	metadata.MovieGetMetadata(&dbmovie, imdb, tmdb, omdb, trakt)

	if updateDB {
		dbmovie.MovieFindDBIDByImdbParser()

		if dbmovie.ID == 0 {
			dbresult, err := database.ExecNid(
				"insert into dbmovies (Imdb_ID) VALUES (?)",
				&dbmovie.ImdbID,
			)
			if err == nil {
				dbmovie.ID = uint(dbresult)
			}
		}

		database.ExecN(
			"update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?",
			&dbmovie.Title,
			&dbmovie.ReleaseDate,
			&dbmovie.Year,
			&dbmovie.Adult,
			&dbmovie.Budget,
			&dbmovie.Genres,
			&dbmovie.OriginalLanguage,
			&dbmovie.OriginalTitle,
			&dbmovie.Overview,
			&dbmovie.Popularity,
			&dbmovie.Revenue,
			&dbmovie.Runtime,
			&dbmovie.SpokenLanguages,
			&dbmovie.Status,
			&dbmovie.Tagline,
			&dbmovie.VoteAverage,
			&dbmovie.VoteCount,
			&dbmovie.TraktID,
			&dbmovie.MoviedbID,
			&dbmovie.ImdbID,
			&dbmovie.FreebaseMID,
			&dbmovie.FreebaseID,
			&dbmovie.FacebookID,
			&dbmovie.InstagramID,
			&dbmovie.TwitterID,
			&dbmovie.URL,
			&dbmovie.Backdrop,
			&dbmovie.Poster,
			&dbmovie.Slug,
			&dbmovie.ID,
		)
	}

	// Convert dbmovie to map[string]any matching renderMovieMetadataDisplay expectations
	movieData := map[string]any{
		"title":            dbmovie.Title,
		"original_title":   dbmovie.OriginalTitle,
		"plot":             dbmovie.Overview, // renderMovieMetadataDisplay expects "plot"
		"genres":           dbmovie.Genres,
		"language":         dbmovie.OriginalLanguage, // renderMovieMetadataDisplay expects "language"
		"spoken_languages": dbmovie.SpokenLanguages,
		"status":           dbmovie.Status,
		"tagline":          dbmovie.Tagline,
		"imdb_id":          dbmovie.ImdbID,
		"trakt_id":         fmt.Sprintf("%d", dbmovie.TraktID), // Convert to string as expected
		"tmdb_id": fmt.Sprintf(
			"%d",
			dbmovie.MoviedbID,
		), // renderMovieMetadataDisplay expects "tmdb_id"
		"freebase_m_id": dbmovie.FreebaseMID,
		"freebase_id":   dbmovie.FreebaseID,
		"facebook_id":   dbmovie.FacebookID,
		"instagram_id":  dbmovie.InstagramID,
		"twitter_id":    dbmovie.TwitterID,
		"website":       dbmovie.URL, // renderMovieMetadataDisplay expects "website"
		"backdrop":      dbmovie.Backdrop,
		"poster":        dbmovie.Poster,
		"slug":          dbmovie.Slug,
		"release_date":  dbmovie.ReleaseDate,
		"released": dbmovie.ReleaseDate.Time.Format(
			"2006-01-02",
		), // renderMovieMetadataDisplay expects "released" as string
		"created_at": dbmovie.CreatedAt,
		"updated_at": dbmovie.UpdatedAt,
		"popularity": dbmovie.Popularity,
		"rating":     dbmovie.VoteAverage, // renderMovieMetadataDisplay expects "rating"
		"votes":      dbmovie.VoteCount,   // renderMovieMetadataDisplay expects "votes"
		"budget":     dbmovie.Budget,
		"revenue":    dbmovie.Revenue,
		"runtime":    dbmovie.Runtime,
		"year":       int(dbmovie.Year), // Convert uint16 to int as expected
		"adult":      dbmovie.Adult,
		"id":         dbmovie.ID,
	}

	// Merge movie data into result
	for key, value := range movieData {
		result[key] = value
	}

	var lastError string

	// If no data was found, return detailed error information
	if result["title"] == nil {
		result["error"] = "No movie data found for the provided IMDB ID"

		result["placeholder"] = true
		if lastError != "" {
			result["note"] = fmt.Sprintf("API errors encountered: %s", lastError)
		} else {
			result["note"] = "Unable to retrieve movie metadata from OMDB or Trakt APIs. Please check that the IMDB ID is correct and that API services are configured properly."
		}

		result["debug_info"] = map[string]any{
			"imdb_id_provided": imdbID != "",
			"last_error":       lastError,
		}
	}

	return renderMetadataResults(result)
}

// handleSeriesMetadataLookup handles series metadata lookup.
func handleSeriesMetadataLookup(imdbID, tvdbID, provider string, updateDB bool) string {
	result := map[string]any{
		"media_type": "series",
		"imdb_id":    imdbID,
		"tvdb_id":    tvdbID,
		"provider":   provider,
		"update_db":  updateDB,
	}

	var lastError string

	// Try TVDB lookup first if TVDB ID is provided
	if tvdbID != "" {
		if tvdbIDInt, err := strconv.Atoi(tvdbID); err == nil {
			if tvdbSeries, err := apiexternal.GetTvdbSeries(tvdbIDInt, "en"); err == nil &&
				tvdbSeries != nil {
				result["title"] = tvdbSeries.Data.SeriesName
				result["plot"] = tvdbSeries.Data.Overview
				result["first_aired"] = tvdbSeries.Data.FirstAired
				result["network"] = tvdbSeries.Data.Network
				result["status"] = tvdbSeries.Data.Status
				result["rating"] = tvdbSeries.Data.Rating
				result["runtime"] = tvdbSeries.Data.Runtime
				result["genres"] = strings.Join(tvdbSeries.Data.Genre, ", ")

				result["banner"] = tvdbSeries.Data.Banner
				if tvdbSeries.Data.ImdbID != "" && imdbID == "" {
					result["imdb_id"] = tvdbSeries.Data.ImdbID
				}

				result["data_source"] = "TVDB"
			} else if err != nil {
				lastError = fmt.Sprintf("TVDB API error: %v", err)
			}
		} else {
			lastError = fmt.Sprintf("Invalid TVDB ID format: %s", tvdbID)
		}
	}

	// Try Trakt lookup as fallback or if IMDB ID is provided
	if result["title"] == nil && imdbID != "" {
		if traktSeries, err := apiexternal.GetTraktSerie(imdbID); err == nil && traktSeries != nil {
			result["title"] = traktSeries.Title
			result["plot"] = traktSeries.Overview
			result["network"] = traktSeries.Network
			result["status"] = traktSeries.Status
			result["language"] = traktSeries.Language
			result["country"] = traktSeries.Country
			result["genres"] = strings.Join(traktSeries.Genres, ", ")
			result["imdb_id"] = traktSeries.IDs.Imdb
			result["tvdb_id"] = strconv.Itoa(traktSeries.IDs.Tvdb)
			result["tmdb_id"] = strconv.Itoa(traktSeries.IDs.Tmdb)
			result["trakt_id"] = strconv.Itoa(traktSeries.IDs.Trakt)
			result["data_source"] = "Trakt"

			// Get seasons information from Trakt
			if seasons, err := apiexternal.GetTraktSerieSeasons(imdbID); err == nil &&
				len(seasons) > 0 {
				result["seasons"] = len(seasons)
				result["season_details"] = seasons
			}
		} else if err != nil {
			if lastError != "" {
				lastError += "; "
			}

			lastError += fmt.Sprintf("Trakt API error: %v", err)
		}
	}

	// If no data was found, return detailed error information
	if result["title"] == nil {
		result["error"] = "No series data found for the provided IDs"

		result["placeholder"] = true
		if lastError != "" {
			result["note"] = fmt.Sprintf("API errors encountered: %s", lastError)
		} else {
			result["note"] = "Unable to retrieve series metadata from TVDB or Trakt APIs. Please check that the IDs are correct and that API services are configured properly."
		}

		result["debug_info"] = map[string]any{
			"tvdb_id_provided": tvdbID != "",
			"imdb_id_provided": imdbID != "",
			"last_error":       lastError,
		}
	}

	return renderMetadataResults(result)
}

// handleEpisodeMetadataLookup handles episode metadata lookup.
func handleEpisodeMetadataLookup(
	imdbID, tvdbID string,
	season, episode int,
	provider string,
	updateDB bool,
) string {
	result := map[string]any{
		"media_type": "episode",
		"imdb_id":    imdbID,
		"tvdb_id":    tvdbID,
		"season":     season,
		"episode":    episode,
		"provider":   provider,
		"update_db":  updateDB,
	}

	var lastError string

	// Try to get series data first to get IMDB ID for Trakt episode lookup
	var seriesImdbID string
	if tvdbID != "" && imdbID == "" {
		if tvdbIDInt, err := strconv.Atoi(tvdbID); err == nil {
			if tvdbSeries, err := apiexternal.GetTvdbSeries(tvdbIDInt, "en"); err == nil &&
				tvdbSeries != nil {
				seriesImdbID = tvdbSeries.Data.ImdbID
			} else if err != nil {
				lastError = fmt.Sprintf("TVDB series lookup error: %v", err)
			}
		} else {
			lastError = fmt.Sprintf("Invalid TVDB ID format: %s", tvdbID)
		}
	} else {
		seriesImdbID = imdbID
	}

	// Try Trakt episode lookup with the series IMDB ID
	if result["title"] == nil && seriesImdbID != "" {
		seasonStr := strconv.Itoa(season)
		if episodes, err := apiexternal.GetTraktSerieSeasonEpisodes(seriesImdbID, seasonStr); err == nil &&
			len(episodes) > 0 {
			// Find the specific episode
			for _, ep := range episodes {
				if ep.Episode == episode {
					result["title"] = ep.Title
					result["plot"] = ep.Overview
					result["first_aired"] = ep.FirstAired.Format("2006-01-02")
					result["season"] = ep.Season
					result["episode"] = ep.Episode
					result["runtime"] = ep.Runtime
					result["data_source"] = "Trakt"

					break
				}
			}
		} else if err != nil {
			if lastError != "" {
				lastError += "; "
			}

			lastError += fmt.Sprintf("Trakt episodes API error: %v", err)
		}
	}

	// If no data was found, return detailed error information
	if result["title"] == nil {
		result["error"] = fmt.Sprintf(
			"No episode data found for Season %d Episode %d",
			season,
			episode,
		)

		result["placeholder"] = true
		if lastError != "" {
			result["note"] = fmt.Sprintf("API errors encountered: %s", lastError)
		} else {
			result["note"] = "Unable to retrieve episode metadata from TVDB or Trakt APIs. Please check that the IDs and episode numbers are correct and that API services are configured properly."
		}

		result["debug_info"] = map[string]any{
			"tvdb_id_provided": tvdbID != "",
			"imdb_id_provided": imdbID != "",
			"series_imdb_id":   seriesImdbID,
			"season":           season,
			"episode":          episode,
			"last_error":       lastError,
		}
	}

	return renderMetadataResults(result)
}

// renderMetadataResults renders metadata lookup results for movies, series, and episodes.
func renderMetadataResults(result map[string]any) string {
	mediaType, _ := result["media_type"].(string)

	// For real metadata results, format based on media type
	switch mediaType {
	case "movie":
		return renderMovieMetadataDisplay(result)
	case "series":
		return renderSeriesMetadataDisplay(result)
	case "episode":
		return renderEpisodeMetadataDisplay(result)
	default:
		return renderComponentToString(
			html.Div(
				html.Class("card border-0 shadow-sm border-danger mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-danger me-3"),
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Error"),
						),
						html.H5(
							html.Class("card-title mb-0 text-danger fw-bold"),
							gomponents.Text("Unknown Media Type"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-0"),
						gomponents.Text(
							"Unable to render metadata for unknown media type: "+mediaType,
						),
					),
				),
			),
		)
	}
}

// renderMovieMetadataDisplay renders movie metadata in a formatted table.
func renderMovieMetadataDisplay(result map[string]any) string {
	resultRows := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Movie Metadata:"))),
			html.Td(gomponents.Text("")),
		),
	}

	// Add metadata fields based on what's available in the result
	if title, ok := result["title"].(string); ok {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Title:")), html.Td(gomponents.Text(title))),
		)
	}

	if dataSource, ok := result["data_source"].(string); ok {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Data Source:")), html.Td(gomponents.Text(dataSource))),
		)
	}
	// Handle year as both int and string (depending on source)
	if year, ok := result["year"].(int); ok {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Year:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", year))),
			),
		)
	} else if yearStr, ok := result["year"].(string); ok && yearStr != "" {
		resultRows = append(resultRows, html.Tr(html.Td(gomponents.Text("Year:")), html.Td(gomponents.Text(yearStr))))
	}

	if imdbID, ok := result["imdb_id"].(string); ok && imdbID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("IMDB ID:")), html.Td(gomponents.Text(imdbID))),
		)
	}

	if tmdbID, ok := result["tmdb_id"].(string); ok && tmdbID != "" && tmdbID != "0" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("TMDB ID:")), html.Td(gomponents.Text(tmdbID))),
		)
	}

	if traktID, ok := result["trakt_id"].(string); ok && traktID != "" && traktID != "0" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Trakt ID:")), html.Td(gomponents.Text(traktID))),
		)
	}

	if plot, ok := result["plot"].(string); ok && plot != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Plot:")), html.Td(gomponents.Text(plot))),
		)
	}

	if tagline, ok := result["tagline"].(string); ok && tagline != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Tagline:")), html.Td(gomponents.Text(tagline))),
		)
	}

	if genre, ok := result["genre"].(string); ok && genre != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Genre:")), html.Td(gomponents.Text(genre))),
		)
	}

	if genres, ok := result["genres"].(string); ok && genres != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Genres:")), html.Td(gomponents.Text(genres))),
		)
	}
	// Handle runtime as both int and string
	if runtime, ok := result["runtime"].(int); ok && runtime > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Runtime:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d minutes", runtime))),
			),
		)
	} else if runtimeStr, ok := result["runtime"].(string); ok && runtimeStr != "" {
		resultRows = append(resultRows, html.Tr(html.Td(gomponents.Text("Runtime:")), html.Td(gomponents.Text(runtimeStr))))
	}
	// Handle rating as both float32 and string
	if rating, ok := result["rating"].(float32); ok && rating > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Rating:")),
				html.Td(gomponents.Text(fmt.Sprintf("%.1f", rating))),
			),
		)
	} else if ratingStr, ok := result["rating"].(string); ok && ratingStr != "" {
		resultRows = append(resultRows, html.Tr(html.Td(gomponents.Text("IMDB Rating:")), html.Td(gomponents.Text(ratingStr))))
	}

	if votes, ok := result["votes"].(string); ok && votes != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("IMDB Votes:")), html.Td(gomponents.Text(votes))),
		)
	} else if votesInt, ok := result["votes"].(int32); ok && votesInt > 0 {
		resultRows = append(resultRows, html.Tr(html.Td(gomponents.Text("Votes:")), html.Td(gomponents.Text(fmt.Sprintf("%d", votesInt)))))
	}

	if language, ok := result["language"].(string); ok && language != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Language:")), html.Td(gomponents.Text(language))),
		)
	}

	if country, ok := result["country"].(string); ok && country != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Country:")), html.Td(gomponents.Text(country))),
		)
	}

	if released, ok := result["released"].(string); ok && released != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Released:")), html.Td(gomponents.Text(released))),
		)
	}

	if status, ok := result["status"].(string); ok && status != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Status:")), html.Td(gomponents.Text(status))),
		)
	}

	if website, ok := result["website"].(string); ok && website != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Website:")), html.Td(gomponents.Text(website))),
		)
	}

	if originalTitle, ok := result["original_title"].(string); ok && originalTitle != "" {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Original Title:")),
				html.Td(gomponents.Text(originalTitle)),
			),
		)
	}

	if spokenLanguages, ok := result["spoken_languages"].(string); ok && spokenLanguages != "" {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Spoken Languages:")),
				html.Td(gomponents.Text(spokenLanguages)),
			),
		)
	}

	if popularity, ok := result["popularity"].(float32); ok && popularity > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Popularity:")),
				html.Td(gomponents.Text(fmt.Sprintf("%.1f", popularity))),
			),
		)
	}

	if budget, ok := result["budget"].(int); ok && budget > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Budget:")),
				html.Td(gomponents.Text(fmt.Sprintf("$%d", budget))),
			),
		)
	}

	if revenue, ok := result["revenue"].(int); ok && revenue > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Revenue:")),
				html.Td(gomponents.Text(fmt.Sprintf("$%d", revenue))),
			),
		)
	}

	if adult, ok := result["adult"].(bool); ok {
		adultStr := "No"
		if adult {
			adultStr = "Yes"
		}

		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Adult Content:")), html.Td(gomponents.Text(adultStr))),
		)
	}

	if backdrop, ok := result["backdrop"].(string); ok && backdrop != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Backdrop:")), html.Td(gomponents.Text(backdrop))),
		)
	}

	if poster, ok := result["poster"].(string); ok && poster != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Poster:")), html.Td(gomponents.Text(poster))),
		)
	}

	if slug, ok := result["slug"].(string); ok && slug != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Slug:")), html.Td(gomponents.Text(slug))),
		)
	}

	if freebaseMID, ok := result["freebase_m_id"].(string); ok && freebaseMID != "" {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Freebase Machine ID:")),
				html.Td(gomponents.Text(freebaseMID)),
			),
		)
	}

	if freebaseID, ok := result["freebase_id"].(string); ok && freebaseID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Freebase ID:")), html.Td(gomponents.Text(freebaseID))),
		)
	}

	if facebookID, ok := result["facebook_id"].(string); ok && facebookID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Facebook ID:")), html.Td(gomponents.Text(facebookID))),
		)
	}

	if instagramID, ok := result["instagram_id"].(string); ok && instagramID != "" {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Instagram ID:")),
				html.Td(gomponents.Text(instagramID)),
			),
		)
	}

	if twitterID, ok := result["twitter_id"].(string); ok && twitterID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Twitter ID:")), html.Td(gomponents.Text(twitterID))),
		)
	}

	if movieID, ok := result["id"].(uint); ok && movieID > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Database ID:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", movieID))),
			),
		)
	}

	return renderComponentToString(
		html.Div(
			html.Class("card border-0 shadow-sm border-success mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center justify-content-between"),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-success me-3"),
							html.I(html.Class("fas fa-check-circle me-1")),
							gomponents.Text("Retrieved"),
						),
						html.H5(
							html.Class("card-title mb-0 text-success fw-bold"),
							gomponents.Text("Movie Metadata Retrieved"),
						),
					),
					html.Span(
						html.Class("badge bg-success"),
						html.I(html.Class("fas fa-film me-1")),
						gomponents.Text("Movie"),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(resultRows)),
				),
			),
		),
	)
}

// renderSeriesMetadataDisplay renders series metadata in a formatted table.
func renderSeriesMetadataDisplay(result map[string]any) string {
	resultRows := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Series Metadata:"))),
			html.Td(gomponents.Text("")),
		),
	}

	// Add metadata fields based on what's available in the result
	if title, ok := result["title"].(string); ok {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Title:")), html.Td(gomponents.Text(title))),
		)
	}

	if dataSource, ok := result["data_source"].(string); ok {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Data Source:")), html.Td(gomponents.Text(dataSource))),
		)
	}

	if imdbID, ok := result["imdb_id"].(string); ok && imdbID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("IMDB ID:")), html.Td(gomponents.Text(imdbID))),
		)
	}

	if tvdbID, ok := result["tvdb_id"].(string); ok && tvdbID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("TVDB ID:")), html.Td(gomponents.Text(tvdbID))),
		)
	}

	if tmdbID, ok := result["tmdb_id"].(string); ok && tmdbID != "" && tmdbID != "0" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("TMDB ID:")), html.Td(gomponents.Text(tmdbID))),
		)
	}

	if traktID, ok := result["trakt_id"].(string); ok && traktID != "" && traktID != "0" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Trakt ID:")), html.Td(gomponents.Text(traktID))),
		)
	}

	if plot, ok := result["plot"].(string); ok && plot != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Plot:")), html.Td(gomponents.Text(plot))),
		)
	}

	if firstAired, ok := result["first_aired"].(string); ok && firstAired != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("First Aired:")), html.Td(gomponents.Text(firstAired))),
		)
	}

	if network, ok := result["network"].(string); ok && network != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Network:")), html.Td(gomponents.Text(network))),
		)
	}

	if status, ok := result["status"].(string); ok && status != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Status:")), html.Td(gomponents.Text(status))),
		)
	}

	if rating, ok := result["rating"].(string); ok && rating != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Rating:")), html.Td(gomponents.Text(rating))),
		)
	}

	if runtime, ok := result["runtime"].(string); ok && runtime != "" {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Runtime:")),
				html.Td(gomponents.Text(runtime+" minutes")),
			),
		)
	}

	if genres, ok := result["genres"].(string); ok && genres != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Genres:")), html.Td(gomponents.Text(genres))),
		)
	}

	if language, ok := result["language"].(string); ok && language != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Language:")), html.Td(gomponents.Text(language))),
		)
	}

	if country, ok := result["country"].(string); ok && country != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Country:")), html.Td(gomponents.Text(country))),
		)
	}

	if seasons, ok := result["seasons"].(int); ok {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Seasons:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", seasons))),
			),
		)
	}

	return renderComponentToString(
		html.Div(
			html.Class("card border-0 shadow-sm border-success mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center justify-content-between"),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-success me-3"),
							html.I(html.Class("fas fa-check-circle me-1")),
							gomponents.Text("Retrieved"),
						),
						html.H5(
							html.Class("card-title mb-0 text-success fw-bold"),
							gomponents.Text("Series Metadata Retrieved"),
						),
					),
					html.Span(
						html.Class("badge bg-success"),
						html.I(html.Class("fas fa-tv me-1")),
						gomponents.Text("Series"),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(resultRows)),
				),
			),
		),
	)
}

// renderEpisodeMetadataDisplay renders episode metadata in a formatted table.
func renderEpisodeMetadataDisplay(result map[string]any) string {
	resultRows := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Episode Metadata:"))),
			html.Td(gomponents.Text("")),
		),
	}

	// Add metadata fields based on what's available in the result
	if title, ok := result["title"].(string); ok {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Episode Title:")), html.Td(gomponents.Text(title))),
		)
	}

	if dataSource, ok := result["data_source"].(string); ok {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Data Source:")), html.Td(gomponents.Text(dataSource))),
		)
	}

	if season, ok := result["season"].(int); ok {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Season:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", season))),
			),
		)
	}

	if episode, ok := result["episode"].(int); ok {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Episode:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", episode))),
			),
		)
	}

	if imdbID, ok := result["imdb_id"].(string); ok && imdbID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Series IMDB ID:")), html.Td(gomponents.Text(imdbID))),
		)
	}

	if tvdbID, ok := result["tvdb_id"].(string); ok && tvdbID != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Series TVDB ID:")), html.Td(gomponents.Text(tvdbID))),
		)
	}

	if plot, ok := result["plot"].(string); ok && plot != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Plot:")), html.Td(gomponents.Text(plot))),
		)
	}

	if firstAired, ok := result["first_aired"].(string); ok && firstAired != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("First Aired:")), html.Td(gomponents.Text(firstAired))),
		)
	}

	if runtime, ok := result["runtime"]; ok {
		if runtimeInt, ok := runtime.(int); ok && runtimeInt > 0 {
			resultRows = append(
				resultRows,
				html.Tr(
					html.Td(gomponents.Text("Runtime:")),
					html.Td(gomponents.Text(fmt.Sprintf("%d minutes", runtimeInt))),
				),
			)
		}
	}

	if poster, ok := result["poster"].(string); ok && poster != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Poster:")), html.Td(gomponents.Text(poster))),
		)
	}

	return renderComponentToString(
		html.Div(
			html.Class("card border-0 shadow-sm border-success mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center justify-content-between"),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-success me-3"),
							html.I(html.Class("fas fa-check-circle me-1")),
							gomponents.Text("Retrieved"),
						),
						html.H5(
							html.Class("card-title mb-0 text-success fw-bold"),
							gomponents.Text("Episode Metadata Retrieved"),
						),
					),
					html.Span(
						html.Class("badge bg-success"),
						html.I(html.Class("fas fa-play-circle me-1")),
						gomponents.Text("Episode"),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(resultRows)),
				),
			),
		),
	)
}

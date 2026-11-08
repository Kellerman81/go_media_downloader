package api

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
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
							"Look up metadata for movies, series, episodes, books, audiobooks and music, with the app's combined lookup or with any single provider. Every provider has a test value that is known to return a result.",
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
					html.H5(html.Class("form-section-title"), gomponents.Text("What to look up")),

					renderFormGroup("metadata", map[string]string{
						"MediaType": "Select the type of media to look up",
					}, map[string]string{
						"MediaType": "Media Type",
					}, "MediaType", "select", "movie", map[string][]string{
						"options": mdKinds,
					}),

					renderFormGroup("metadata", map[string]string{
						"Provider": "The app lookup combines the configured sources and can update the database; " +
							"a single provider shows exactly what that provider returns",
					}, map[string]string{
						"Provider": "Metadata Provider",
					}, "Provider", "select", mdAppProvider, map[string][]string{
						"options": {mdAppProvider},
					}),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Identifier")),

					html.Div(
						html.Class("mb-3"),
						html.Label(html.For("metadata_Identifier"), html.Class("form-label"), html.ID("metadataIDLabel"),
							gomponents.Text("ID")),
						html.Div(
							html.Class("input-group"),
							html.Input(html.Type("text"), html.Class("form-control"), html.ID("metadata_Identifier"),
								html.Name("metadata_Identifier")),
							html.Button(html.Type("button"), html.Class("btn btn-outline-secondary"), html.ID("metadataTestValue"),
								html.I(html.Class("fas fa-vial me-1")), gomponents.Text("Use test value")),
						),
						html.Small(html.Class("form-text text-muted"), html.ID("metadataTestHint")),
					),

					html.Div(
						html.ID("episodeFields"),
						html.Style("display: none;"),
						html.Class("row"),
						html.Div(html.Class("col-6"),
							renderFormGroup("metadata", map[string]string{"Season": "Season number"},
								map[string]string{"Season": "Season"}, "Season", "number", "1", nil)),
						html.Div(html.Class("col-6"),
							renderFormGroup("metadata", map[string]string{"Episode": "Episode number"},
								map[string]string{"Episode": "Episode"}, "Episode", "number", "1", nil)),
					),

					html.Div(
						html.ID("updateDBField"),
						renderFormGroup("metadata", map[string]string{
							"UpdateDB": "Save the retrieved metadata to the database (app lookup only)",
						}, map[string]string{
							"UpdateDB": "Update Database",
						}, "UpdateDB", "checkbox", false, nil),
					),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class(ClassBtnPrimary),
					html.I(html.Class("fas fa-search")),
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
					html.Class("btn btn-secondary"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('metadataTestForm').reset(); document.getElementById('metadataResults').innerHTML = '';",
					),
					html.I(html.Class("fas fa-rotate-left")),
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
						gomponents.Text("App lookup: IMDb ID. TMDB: TMDB ID. OMDb and Trakt: IMDb ID"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-info me-2"),
							html.I(html.Class("fas fa-tv me-1")),
							gomponents.Text("Series"),
						),
						gomponents.Text(
							"App lookup: IMDb or TVDB ID. TVDB, TMDB and TVmaze: their own IDs. Trakt: IMDb ID",
						),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-warning me-2"),
							html.I(html.Class("fas fa-play-circle me-1")),
							gomponents.Text("Episodes"),
						),
						gomponents.Text("The series ID of the chosen provider plus season and episode numbers"),
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
								"Each provider takes its own kind of ID; the ID label and 'Use test value' change with the provider. A provider without an API key in the config reports that it is not configured.",
							),
						),
					),
				),

				html.P(
					html.Class("mb-0"),
					html.Strong(gomponents.Text("Update Database: ")),
					gomponents.Text(
						"Only the app lookup can save the retrieved metadata to your local database; single providers are read-only.",
					),
				),
			),
		),

		// Fills the provider dropdown for the media type and the ID label and
		// test value for the provider.
		html.Script(gomponents.Raw(`
			(function(){
				var providers = `+mdProvidersJSON()+`;
				function el(id){ return document.getElementById(id); }
				function current(){
					var t = el('metadata_MediaType'), p = el('metadata_Provider');
					var list = providers[t ? t.value : ''] || [];
					for (var i = 0; i < list.length; i++) { if (p && list[i].key === p.value) return list[i]; }
					return list[0];
				}
				function updProvider(){
					var c = current(); if (!c) return;
					el('metadataIDLabel').textContent = c.idLabel;
					el('metadata_Identifier').placeholder = 'e.g. ' + c.test;
					el('metadataTestHint').textContent = 'Test value: ' + c.test;
					el('updateDBField').style.display = c.key === 'app' ? '' : 'none';
				}
				function updType(){
					var t = el('metadata_MediaType'), p = el('metadata_Provider');
					if (!t || !p) return;
					var keep = p.value;
					p.innerHTML = '';
					(providers[t.value] || []).forEach(function(e){
						var o = document.createElement('option');
						o.value = e.key; o.textContent = e.label;
						p.appendChild(o);
					});
					p.value = keep; if (p.value !== keep) p.value = 'app';
					el('episodeFields').style.display = t.value === 'episode' ? '' : 'none';
					updProvider();
				}
				function init(){
					var t = el('metadata_MediaType'); if (!t) return;
					t.addEventListener('change', updType);
					el('metadata_Provider').addEventListener('change', updProvider);
					el('metadataTestValue').addEventListener('click', function(){
						var c = current(); if (c) el('metadata_Identifier').value = c.test;
					});
					el('metadataTestForm').addEventListener('reset', function(){ setTimeout(updType, 0); });
					updType();
				}
				if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init); else init();
			})();
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
	identifier := strings.TrimSpace(c.PostForm("metadata_Identifier"))
	updateDB := formBool(c.PostForm("metadata_UpdateDB"))

	prov, ok := findMDProvider(mediaType, c.PostForm("metadata_Provider"))
	if !ok {
		c.String(http.StatusOK, renderAlert("Unknown media type or provider", "danger"))
		return
	}

	if identifier == "" {
		c.String(http.StatusOK, renderAlert("Please enter the "+prov.IDLabel, "warning"))
		return
	}

	if prov.lookup != nil {
		season, _ := strconv.Atoi(c.PostForm("metadata_Season"))
		episode, _ := strconv.Atoi(c.PostForm("metadata_Episode"))
		c.String(http.StatusOK, runMDLookup(prov, identifier, season, episode))

		return
	}

	// The app lookup takes the identifier as the field its media type uses;
	// series and episodes accept an IMDb ID or a TVDB ID.
	var imdbID, tvdbID, isbn, asin, mbReleaseID string

	provider := ""

	switch mediaType {
	case "movie":
		imdbID = identifier
	case "series", "episode":
		if strings.HasPrefix(identifier, "tt") {
			imdbID = identifier
		} else {
			tvdbID = identifier
		}
	case "book":
		isbn = identifier
	case "audiobook":
		asin = identifier
	case "music":
		mbReleaseID = identifier
	}

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

	case "book":
		if isbn == "" {
			c.String(http.StatusOK, renderAlert("Please enter an ISBN for book lookup", "warning"))
			return
		}

	case "audiobook":
		if asin == "" {
			c.String(
				http.StatusOK,
				renderAlert("Please enter an ASIN for audiobook lookup", "warning"),
			)
			return
		}

	case "music":
		if mbReleaseID == "" {
			c.String(
				http.StatusOK,
				renderAlert("Please enter a MusicBrainz release ID for music lookup", "warning"),
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
			logger.Logtype("error", 0).
				Str("value", c.PostForm("metadata_Season")).
				Err(err).
				Msg("Failed to parse season number")
		}

		episodeNum, err := strconv.Atoi(c.PostForm("metadata_Episode"))
		if err != nil {
			logger.Logtype("error", 0).
				Str("value", c.PostForm("metadata_Episode")).
				Err(err).
				Msg("Failed to parse episode number")
		}

		c.String(
			http.StatusOK,
			handleEpisodeMetadataLookup(imdbID, tvdbID, seasonNum, episodeNum, provider, updateDB),
		)

	case "book":
		c.String(http.StatusOK, handleBookMetadataLookup(isbn, updateDB))
	case "audiobook":
		c.String(http.StatusOK, handleAudiobookMetadataLookup(asin, updateDB))
	case "music":
		c.String(http.StatusOK, handleAlbumMetadataLookup(mbReleaseID, updateDB))
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

	// Without any provider data the update below would blank the existing row.
	if updateDB && dbmovie.Title != "" {
		dbmovie.MovieFindDBIDByImdbParser()

		if dbmovie.ID == 0 {
			dbresult, err := database.ExecNid(
				"insert into dbmovies (Imdb_ID) VALUES (?)",
				&dbmovie.ImdbID,
			)
			if err == nil {
				dbmovie.ID = uint(dbresult) //nolint:gosec // safe: value within target type range
			}
		}

		if dbmovie.ID == 0 {
			result["db_error"] = "movie row could not be created"
		} else if err := database.ExecNErr(
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
		); err != nil {
			result["db_error"] = err.Error()
		}
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
	maps.Copy(result, movieData)

	var lastError string

	// If no data was found, return detailed error information
	if dbmovie.Title == "" {
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

// tvdbSeriesDetails fetches a series from TVDB.
func tvdbSeriesDetails(id int, language string) (*apiexternal_v2.SeriesDetails, error) {
	provider := providers.GetTVDB()
	if provider == nil {
		return nil, errProviderNotInitialized
	}

	return provider.GetSeriesByID(context.Background(), id, language)
}

// traktSeasonEpisodes fetches the episodes of one season of the show with the
// given IMDB ID from Trakt.
func traktSeasonEpisodes(imdbID string, season int) ([]*apiexternal_v2.Episode, error) {
	show, err := providers.TraktSerieByIMDb(context.Background(), imdbID)
	if err != nil {
		return nil, err
	}

	return providers.GetTrakt().GetSeasonEpisodes(context.Background(), show.ID, season)
}

// joinGenres lists genre names comma-separated.
func joinGenres(genres []apiexternal_v2.Genre) string {
	names := make([]string, len(genres))
	for i := range genres {
		names[i] = genres[i].Name
	}

	return strings.Join(names, ", ")
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
			if tvdbSeries, err := tvdbSeriesDetails(tvdbIDInt, "en"); err == nil &&
				tvdbSeries != nil {
				result["title"] = tvdbSeries.Name
				result["plot"] = tvdbSeries.Overview
				result["status"] = tvdbSeries.Status
				result["genres"] = joinGenres(tvdbSeries.Genres)

				if !tvdbSeries.FirstAirDate.IsZero() {
					result["first_aired"] = tvdbSeries.FirstAirDate.Format("2006-01-02")
				}

				if len(tvdbSeries.Networks) > 0 {
					result["network"] = tvdbSeries.Networks[0].Name
				}

				if len(tvdbSeries.EpisodeRunTime) > 0 {
					result["runtime"] = strconv.Itoa(tvdbSeries.EpisodeRunTime[0])
				}

				if tvdbSeries.IMDbID != "" && imdbID == "" {
					result["imdb_id"] = tvdbSeries.IMDbID
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
		if traktSeries, err := providers.TraktSerieByIMDb(context.Background(), imdbID); err == nil && traktSeries != nil {
			result["title"] = traktSeries.Name
			result["plot"] = traktSeries.Overview
			result["status"] = traktSeries.Status
			result["language"] = traktSeries.OriginalLanguage
			result["genres"] = joinGenres(traktSeries.Genres)
			result["imdb_id"] = traktSeries.IMDbID
			result["tvdb_id"] = strconv.Itoa(traktSeries.TVDbID)
			result["trakt_id"] = strconv.Itoa(traktSeries.ID)
			result["data_source"] = "Trakt"

			if len(traktSeries.Networks) > 0 {
				result["network"] = traktSeries.Networks[0].Name
			}

			// Get seasons information from Trakt
			if seasons, err := providers.GetTrakt().GetAllSeasons(context.Background(), traktSeries.ID); err == nil &&
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

	if updateDB && result["title"] != nil {
		if err := updateSeriesMetadataDB(imdbID, tvdbID, provider); err != nil {
			result["db_error"] = err.Error()
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
			if tvdbSeries, err := tvdbSeriesDetails(tvdbIDInt, "en"); err == nil &&
				tvdbSeries != nil {
				seriesImdbID = tvdbSeries.IMDbID
			} else if err != nil {
				lastError = fmt.Sprintf("TVDB series lookup error: %v", err)
			}
		} else {
			lastError = fmt.Sprintf("Invalid TVDB ID format: %s", tvdbID)
		}
	} else {
		seriesImdbID = imdbID
	}

	var (
		epTitle, epOverview string
		epAired             time.Time
		epRuntime           int
	)

	// Try Trakt episode lookup with the series IMDB ID
	if result["title"] == nil && seriesImdbID != "" {
		if episodes, err := traktSeasonEpisodes(seriesImdbID, season); err == nil &&
			len(episodes) > 0 {
			// Find the specific episode
			for _, ep := range episodes {
				if ep.EpisodeNumber == episode {
					result["title"] = ep.Name
					result["plot"] = ep.Overview
					result["first_aired"] = ep.AirDate.Format("2006-01-02")
					result["season"] = ep.SeasonNumber
					result["episode"] = ep.EpisodeNumber
					result["runtime"] = ep.Runtime
					result["data_source"] = "Trakt"
					epTitle, epOverview, epAired, epRuntime = ep.Name, ep.Overview, ep.AirDate, ep.Runtime

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

	if updateDB && result["title"] != nil {
		if err := updateEpisodeMetadataDB(
			seriesImdbID, tvdbID, season, episode,
			epTitle, epOverview, epAired, epRuntime,
		); err != nil {
			result["db_error"] = err.Error()
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

// findDbserieID resolves an existing dbseries row by TVDB ID, falling back to IMDB ID.
func findDbserieID(imdbID string, tvdbID int) uint {
	var id uint
	if tvdbID != 0 {
		database.Scanrowsdyn(false, database.QueryDbseriesGetIDByTvdb, &id, &tvdbID)
	}

	if id == 0 && imdbID != "" {
		database.Scanrowsdyn(false, "select id from dbseries where imdb_id = ?", &id, &imdbID)
	}

	return id
}

// updateSeriesMetadataDB refreshes (or creates) the dbseries row through the
// metadata package. Existing ids and links are kept because the row is loaded
// first and SerieGetMetadata only replaces fields a provider actually returned.
func updateSeriesMetadataDB(imdbID, tvdbID, provider string) error {
	tvdbIDInt, _ := strconv.Atoi(tvdbID)

	var dbserie database.Dbserie
	if id := findDbserieID(imdbID, tvdbIDInt); id != 0 {
		if err := dbserie.GetDbserieByIDP(&id); err != nil {
			return fmt.Errorf("load series %d: %w", id, err)
		}
	}

	if dbserie.ThetvdbID == 0 {
		dbserie.ThetvdbID = tvdbIDInt
	}

	if dbserie.ImdbID == "" {
		dbserie.ImdbID = imdbID
	}

	if dbserie.ThetvdbID == 0 {
		return errors.New("series metadata refresh needs a TVDB ID (or an existing series row with one)")
	}

	var querytmdb, querytrakt bool
	switch provider {
	case "tmdb":
		querytmdb = true
	case "trakt":
		querytrakt = true
	default:
		querytmdb = config.GetSettingsGeneral().SerieMetaSourceTmdb
		querytrakt = config.GetSettingsGeneral().SerieMetaSourceTrakt
	}

	metadata.SerieGetMetadata(&dbserie, "", querytmdb, querytrakt, true, nil)

	if dbserie.Seriename == "" {
		return errors.New("no provider returned series data; database left unchanged")
	}

	if dbserie.ID == 0 {
		inres, err := database.ExecNid(
			"insert into dbseries (seriename, thetvdb_id, identifiedby) values (?, ?, ?)",
			&dbserie.Seriename,
			&dbserie.ThetvdbID,
			&dbserie.Identifiedby,
		)
		if err != nil {
			return fmt.Errorf("insert series: %w", err)
		}

		dbserie.ID = logger.Int64ToUint(inres)
	}

	return database.ExecNErr(
		"update dbseries SET Seriename = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?",
		&dbserie.Seriename,
		&dbserie.Season,
		&dbserie.Status,
		&dbserie.Firstaired,
		&dbserie.Network,
		&dbserie.Runtime,
		&dbserie.Language,
		&dbserie.Genre,
		&dbserie.Overview,
		&dbserie.Rating,
		&dbserie.Siterating,
		&dbserie.SiteratingCount,
		&dbserie.Slug,
		&dbserie.TraktID,
		&dbserie.ImdbID,
		&dbserie.ThetvdbID,
		&dbserie.FreebaseMID,
		&dbserie.FreebaseID,
		&dbserie.TvrageID,
		&dbserie.Facebook,
		&dbserie.Instagram,
		&dbserie.Twitter,
		&dbserie.Banner,
		&dbserie.Poster,
		&dbserie.Fanart,
		&dbserie.Identifiedby,
		&dbserie.ID,
	)
}

// updateEpisodeMetadataDB writes the looked-up episode fields into the
// existing dbserie_episodes row. Episodes are only created by the series
// refresh jobs, so a missing row is reported rather than inserted; empty
// provider values never overwrite stored ones.
func updateEpisodeMetadataDB(
	imdbID, tvdbID string,
	season, episode int,
	title, overview string,
	aired time.Time,
	runtime int,
) error {
	tvdbIDInt, _ := strconv.Atoi(tvdbID)

	dbserieID := findDbserieID(imdbID, tvdbIDInt)
	if dbserieID == 0 {
		return errors.New("series not found in database")
	}

	seasonStr, episodeStr := strconv.Itoa(season), strconv.Itoa(episode)

	var episodeID uint
	database.Scanrowsdyn(
		false,
		"select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?",
		&episodeID,
		&dbserieID,
		&seasonStr,
		&episodeStr,
	)

	if episodeID == 0 {
		return fmt.Errorf("episode S%dE%d not found in database", season, episode)
	}

	firstAired := sql.NullTime{Time: aired, Valid: !aired.IsZero()}

	return database.ExecNErr(
		"update dbserie_episodes SET title = CASE WHEN ? != '' THEN ? ELSE title END, overview = CASE WHEN ? != '' THEN ? ELSE overview END, first_aired = COALESCE(?, first_aired), runtime = CASE WHEN ? > 0 THEN ? ELSE runtime END where id = ?",
		&title, &title,
		&overview, &overview,
		&firstAired,
		&runtime, &runtime,
		&episodeID,
	)
}

// renderMetadataResults renders metadata lookup results for movies, series, and episodes.
func renderMetadataResults(result map[string]any) string {
	mediaType, _ := result["media_type"].(string)

	// The per-type renderers always draw a "retrieved" card, so a failed lookup
	// has to be reported before reaching them.
	if errMsg, ok := result["error"].(string); ok && errMsg != "" {
		if note, ok := result["note"].(string); ok && note != "" {
			errMsg += " - " + note
		}

		return renderAlert(errMsg, "warning")
	}

	if dbErr, ok := result["db_error"].(string); ok && dbErr != "" {
		delete(result, "db_error")
		return renderAlert("Database update failed: "+dbErr, "danger") + renderMetadataResults(result)
	}

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

// mdNum formats a numeric field for display, returning "" for zero so the
// renderer can skip empty rows.
func mdNum[T int | int32 | int64 | uint | uint16](n T) string {
	if n == 0 {
		return ""
	}

	return fmt.Sprint(n)
}

// renderMediaMetadataResult renders a simple key/value card for book, audiobook
// and music metadata lookups. Empty values are skipped.
func renderMediaMetadataResult(heading string, rows [][2]string) string {
	trs := make([]gomponents.Node, 0, len(rows))

	for _, r := range rows {
		if r[1] == "" {
			continue
		}

		trs = append(trs, html.Tr(
			html.Th(
				gomponents.Attr("scope", "row"),
				html.Style("width: 12rem;"),
				gomponents.Text(r[0]),
			),
			html.Td(gomponents.Text(r[1])),
		))
	}

	node := html.Div(
		html.Class("card border-0 shadow-sm mt-3"),
		html.Div(
			html.Class("card-header bg-success text-white"),
			html.H5(
				html.Class("mb-0"),
				html.I(html.Class("fas fa-circle-check me-2")),
				gomponents.Text(heading),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Div(
				html.Class("table-responsive"),
				html.Table(
					html.Class("table table-sm table-striped mb-0 align-middle"),
					html.TBody(trs...),
				),
			),
		),
	)

	var buf strings.Builder
	node.Render(&buf)

	return buf.String()
}

// audiobookMetadataConfig returns an audiobook media config (for the Audible
// region) - the first configured one, or a default if none exist.
func audiobookMetadataConfig() *config.MediaTypeConfig {
	if all := config.GetSettingsMediaAll(); len(all.AudioBooks) > 0 {
		return &all.AudioBooks[0]
	}

	return &config.MediaTypeConfig{AudibleRegion: "us"}
}

// handleBookMetadataLookup fetches book metadata by ISBN and optionally updates
// (or inserts) the matching dbbooks row.
func handleBookMetadataLookup(isbn string, updateDB bool) string {
	var book database.Dbbook

	var isbn13, isbn10 string
	if len(isbn) == 10 {
		isbn10 = isbn
	} else {
		isbn13 = isbn
	}

	book.ISBN13 = isbn13
	book.ISBN10 = isbn10

	if err := metadata.BookGetMetadata(context.Background(), &book, true); err != nil {
		return renderAlert("Book metadata lookup failed: "+err.Error(), "danger")
	}

	if book.Title == "" {
		return renderAlert("No book metadata found for ISBN "+isbn, "warning")
	}

	heading := "Book"
	if updateDB {
		var id uint
		if isbn13 != "" {
			id = database.Getdatarow[uint](
				false,
				"SELECT id FROM dbbooks WHERE isbn_13 = ? LIMIT 1",
				&isbn13,
			)
		} else {
			id = database.Getdatarow[uint](
				false,
				"SELECT id FROM dbbooks WHERE isbn_10 = ? LIMIT 1",
				&isbn10,
			)
		}

		if id == 0 {
			if newid, err := database.ExecNid(
				"INSERT INTO dbbooks (isbn_13, isbn_10) VALUES (?, ?)", &isbn13, &isbn10,
			); err == nil {
				id = uint(newid) //nolint:gosec // safe: value within target type range
			}
		}

		if id != 0 {
			book.ID = id
			if book.Slug == "" {
				book.Slug = logger.StringToSlugCached(book.Title)
			}

			// The metadata lookup does not resolve author/series links; keep the
			// row's existing ones instead of resetting them to 0.
			if book.DbauthorID == 0 {
				book.DbauthorID = database.Getdatarow[uint](false, "SELECT dbauthor_id FROM dbbooks WHERE id = ?", &id)
			}

			if book.DbbookSeriesID == 0 {
				book.DbbookSeriesID = database.Getdatarow[uint](false, "SELECT dbbook_series_id FROM dbbooks WHERE id = ?", &id)
			}

			err := database.ExecNErr(
				"update dbbooks SET title = ?, original_title = ?, isbn_13 = ?, isbn_10 = ?, asin = ?, openlibrary_id = ?, goodreads_id = ?, description = ?, publisher = ?, publish_date = ?, page_count = ?, language = ?, genres = ?, cover_url = ?, dbauthor_id = ?, dbbook_series_id = ?, series_position = ?, average_rating = ?, ratings_count = ?, year = ?, slug = ? where id = ?",
				&book.Title,
				&book.OriginalTitle,
				&book.ISBN13,
				&book.ISBN10,
				&book.ASIN,
				&book.OpenlibraryID,
				&book.GoodreadsID,
				&book.Description,
				&book.Publisher,
				&book.PublishDate,
				&book.PageCount,
				&book.Language,
				&book.Genres,
				&book.CoverURL,
				&book.DbauthorID,
				&book.DbbookSeriesID,
				&book.SeriesPosition,
				&book.AverageRating,
				&book.RatingsCount,
				&book.Year,
				&book.Slug,
				&book.ID,
			)
			if err != nil {
				heading = "Book (database update failed: " + err.Error() + ")"
			} else {
				heading = fmt.Sprintf("Book (saved to database, id %d)", id)
			}
		} else {
			heading = "Book (database update failed)"
		}
	}

	return renderMediaMetadataResult(heading, [][2]string{
		{"Title", book.Title},
		{"Original Title", book.OriginalTitle},
		{"ISBN-13", book.ISBN13},
		{"ISBN-10", book.ISBN10},
		{"OpenLibrary ID", book.OpenlibraryID},
		{"Goodreads ID", book.GoodreadsID},
		{"Publisher", book.Publisher},
		{"Language", book.Language},
		{"Year", mdNum(book.Year)},
		{"Pages", mdNum(book.PageCount)},
		{"Genres", book.Genres},
		{"Description", book.Description},
		{"Cover URL", book.CoverURL},
	})
}

// handleAudiobookMetadataLookup fetches audiobook metadata by ASIN and optionally
// updates (or inserts) the matching dbaudiobooks row.
func handleAudiobookMetadataLookup(asin string, updateDB bool) string {
	var ab database.Dbaudiobook

	ab.ASIN = asin

	if err := metadata.AudiobookGetMetadata(
		context.Background(), audiobookMetadataConfig(), &ab, true,
	); err != nil {
		return renderAlert("Audiobook metadata lookup failed: "+err.Error(), "danger")
	}

	if ab.Title == "" {
		return renderAlert("No audiobook metadata found for ASIN "+asin, "warning")
	}

	heading := "Audiobook"
	if updateDB {
		id := database.Getdatarow[uint](
			false,
			"SELECT id FROM dbaudiobooks WHERE asin = ? LIMIT 1",
			&asin,
		)
		if id == 0 {
			if newid, err := database.ExecNid(
				"INSERT INTO dbaudiobooks (asin) VALUES (?)",
				&asin,
			); err == nil {
				id = uint(newid) //nolint:gosec // safe: value within target type range
			}
		}

		if id != 0 {
			ab.ID = id
			if ab.Slug == "" {
				ab.Slug = logger.StringToSlugCached(ab.Title)
			}

			// Keep an existing book link when the lookup found none.
			if ab.DbbookID == 0 {
				ab.DbbookID = database.Getdatarow[uint](false, "SELECT dbbook_id FROM dbaudiobooks WHERE id = ?", &id)
			}

			err := database.ExecNErr(
				"update dbaudiobooks SET title = ?, asin = ?, audible_id = ?, runtime_minutes = ?, chapter_count = ?, release_date = ?, publisher = ?, language = ?, abridged = ?, cover_url = ?, sample_url = ?, average_rating = ?, ratings_count = ?, year = ?, slug = ?, dbbook_id = ?, description = ?, series_name = ?, series_position = ? where id = ?",
				&ab.Title,
				&ab.ASIN,
				&ab.AudibleID,
				&ab.RuntimeMinutes,
				&ab.ChapterCount,
				&ab.ReleaseDate,
				&ab.Publisher,
				&ab.Language,
				&ab.Abridged,
				&ab.CoverURL,
				&ab.SampleURL,
				&ab.AverageRating,
				&ab.RatingsCount,
				&ab.Year,
				&ab.Slug,
				&ab.DbbookID,
				&ab.Description,
				&ab.SeriesName,
				&ab.SeriesPosition,
				&ab.ID,
			)
			if err != nil {
				heading = "Audiobook (database update failed: " + err.Error() + ")"
			} else {
				heading = fmt.Sprintf("Audiobook (saved to database, id %d)", id)
			}
		} else {
			heading = "Audiobook (database update failed)"
		}
	}

	return renderMediaMetadataResult(heading, [][2]string{
		{"Title", ab.Title},
		{"ASIN", ab.ASIN},
		{"Audible ID", ab.AudibleID},
		{"Publisher", ab.Publisher},
		{"Language", ab.Language},
		{"Year", mdNum(ab.Year)},
		{"Runtime (min)", mdNum(ab.RuntimeMinutes)},
		{"Chapters", mdNum(ab.ChapterCount)},
		{"Description", ab.Description},
		{"Cover URL", ab.CoverURL},
	})
}

// handleAlbumMetadataLookup fetches music album metadata by MusicBrainz release
// ID and optionally updates (or inserts) the matching dbalbums row.
func handleAlbumMetadataLookup(mbReleaseID string, updateDB bool) string {
	var album database.Dbalbum

	album.MusicbrainzReleaseID = mbReleaseID

	if err := metadata.AlbumGetMetadata(context.Background(), &album, true); err != nil {
		return renderAlert("Album metadata lookup failed: "+err.Error(), "danger")
	}

	if album.Title == "" {
		return renderAlert(
			"No album metadata found for MusicBrainz release "+mbReleaseID,
			"warning",
		)
	}

	heading := "Album"
	if updateDB {
		id := database.Getdatarow[uint](
			false, "SELECT id FROM dbalbums WHERE musicbrainz_release_id = ? LIMIT 1", &mbReleaseID,
		)
		if id == 0 {
			if newid, err := database.ExecNid(
				"INSERT INTO dbalbums (musicbrainz_release_id) VALUES (?)", &mbReleaseID,
			); err == nil {
				id = uint(newid) //nolint:gosec // safe: value within target type range
			}
		}

		if id != 0 {
			album.ID = id
			if album.Slug == "" {
				album.Slug = logger.StringToSlugCached(album.Title)
			}

			err := database.ExecNErr(
				"update dbalbums SET title = ?, musicbrainz_release_group_id = ?, musicbrainz_release_id = ?, discogs_master_id = ?, discogs_release_id = ?, spotify_id = ?, upc = ?, release_date = ?, release_type = ?, format = ?, label = ?, country = ?, total_tracks = ?, total_runtime_ms = ?, genres = ?, styles = ?, cover_url = ?, year = ?, slug = ? where id = ?",
				&album.Title,
				&album.MusicbrainzReleaseGroupID,
				&album.MusicbrainzReleaseID,
				&album.DiscogsMasterID,
				&album.DiscogsReleaseID,
				&album.SpotifyID,
				&album.UPC,
				&album.ReleaseDate,
				&album.ReleaseType,
				&album.Format,
				&album.Label,
				&album.Country,
				&album.TotalTracks,
				&album.TotalRuntimeMs,
				&album.Genres,
				&album.Styles,
				&album.CoverURL,
				&album.Year,
				&album.Slug,
				&album.ID,
			)
			if err != nil {
				heading = "Album (database update failed: " + err.Error() + ")"
			} else {
				heading = fmt.Sprintf("Album (saved to database, id %d)", id)
			}
		} else {
			heading = "Album (database update failed)"
		}
	}

	return renderMediaMetadataResult(heading, [][2]string{
		{"Title", album.Title},
		{"MusicBrainz Release", album.MusicbrainzReleaseID},
		{"Release Group", album.MusicbrainzReleaseGroupID},
		{"Label", album.Label},
		{"Country", album.Country},
		{"Format", album.Format},
		{"Release Type", album.ReleaseType},
		{"Year", mdNum(album.Year)},
		{"Tracks", mdNum(album.TotalTracks)},
		{"Genres", album.Genres},
		{"Cover URL", album.CoverURL},
	})
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
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Year:")), html.Td(gomponents.Text(yearStr))),
		)
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
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("Runtime:")), html.Td(gomponents.Text(runtimeStr))),
		)
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
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("IMDB Rating:")), html.Td(gomponents.Text(ratingStr))),
		)
	}

	if votes, ok := result["votes"].(string); ok && votes != "" {
		resultRows = append(
			resultRows,
			html.Tr(html.Td(gomponents.Text("IMDB Votes:")), html.Td(gomponents.Text(votes))),
		)
	} else if votesInt, ok := result["votes"].(int32); ok && votesInt > 0 {
		resultRows = append(
			resultRows,
			html.Tr(
				html.Td(gomponents.Text("Votes:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", votesInt))),
			),
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

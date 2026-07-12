package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// MovieMetadataSearchPage renders the movie metadata search page.
func MovieMetadataSearchPage(c *gin.Context) {
	movieLists := getMediaListsByType("movie")
	qualityProfiles := getQualityProfileNames()
	csrfToken := getCSRFToken(c)

	content := movieMetadataSearchContent(movieLists, qualityProfiles, csrfToken)

	pageNode := page(
		"Movie Metadata Search",
		false, // activeConfig
		false, // activeDatabase
		true,  // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// SeriesMetadataSearchPage renders the series metadata search page.
func SeriesMetadataSearchPage(c *gin.Context) {
	seriesLists := getMediaListsByType("series")
	qualityProfiles := getQualityProfileNames()
	csrfToken := getCSRFToken(c)

	content := seriesMetadataSearchContent(seriesLists, qualityProfiles, csrfToken)

	pageNode := page(
		"Series Metadata Search",
		false, // activeConfig
		false, // activeDatabase
		true,  // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// getMediaListsByType returns media list names filtered by type (movie, series, book, audiobook, music).
func getMediaListsByType(mediaType string) []string {
	allMedia := config.GetSettingsMediaAll()
	if allMedia == nil {
		return []string{}
	}

	lists := make([]string, 0)

	switch mediaType {
	case "movie":
		for _, media := range allMedia.Movies {
			for _, list := range media.Lists {
				lists = append(lists, list.Name)
			}
		}

	case "series":
		for _, media := range allMedia.Series {
			for _, list := range media.Lists {
				lists = append(lists, list.Name)
			}
		}

	case "book":
		for _, media := range allMedia.Books {
			for _, list := range media.Lists {
				lists = append(lists, list.Name)
			}
		}

	case "audiobook":
		for _, media := range allMedia.AudioBooks {
			for _, list := range media.Lists {
				lists = append(lists, list.Name)
			}
		}

	case "music":
		for _, media := range allMedia.Music {
			for _, list := range media.Lists {
				lists = append(lists, list.Name)
			}
		}
	}

	return lists
}

// getQualityProfileNames returns all quality profile names.
func getQualityProfileNames() []string {
	qualityConfigs := config.GetSettingsQualityAll()
	names := make([]string, 0, len(qualityConfigs))

	for _, qc := range qualityConfigs {
		names = append(names, qc.Name)
	}

	return names
}

func movieMetadataSearchContent(
	mediaConfigs, qualityProfiles []string,
	csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-search header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Movie Metadata Search")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search for movies across metadata sources and add them to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Search Form Card
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text("Search Movies"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("movieSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("movie_title"),
										html.Class("form-label"),
										gomponents.Text("Movie Title"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("movie_title"),
										html.Name("movie_title"),
										html.Placeholder("Enter movie title to search..."),
										gomponents.Attr("required", "true"),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("movie_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("movie_list"),
											html.Name("movie_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("movie_quality_profile"),
											html.Class("form-label"),
											gomponents.Text("Quality Profile"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("movie_quality_profile"),
											html.Name("movie_quality_profile"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(qualityProfiles, ""),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text("Search Movies"),
									),
								),
							),
						),
					),
				),

				// Manual Entry Card
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-edit me-2")),
								gomponents.Text("Manual Entry"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("movieManualForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-8 mb-3"),
										html.Label(
											html.For("manualMovie_title"),
											html.Class("form-label"),
											gomponents.Text("Movie Title *"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualMovie_title"),
											html.Name("manualMovie_title"),
											html.Placeholder("Enter movie title"),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualMovie_year"),
											html.Class("form-label"),
											gomponents.Text("Year"),
										),
										html.Input(
											html.Type("number"),
											html.Class("form-control"),
											html.ID("manualMovie_year"),
											html.Name("manualMovie_year"),
											html.Placeholder("YYYY"),
											gomponents.Attr("min", "1900"),
											gomponents.Attr("max", "2030"),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List *"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualMovie_list"),
											html.Name("manualMovie_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_quality_profile"),
											html.Class("form-label"),
											gomponents.Text("Quality Profile *"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualMovie_quality_profile"),
											html.Name("manualMovie_quality_profile"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(qualityProfiles, ""),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_imdb_id"),
											html.Class("form-label"),
											gomponents.Text("IMDB ID"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualMovie_imdb_id"),
											html.Name("manualMovie_imdb_id"),
											html.Placeholder("tt1234567"),
											gomponents.Attr("pattern", "tt[0-9]+"),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_overview"),
											html.Class("form-label"),
											gomponents.Text("Overview"),
										),
										html.Textarea(
											html.Class("form-control"),
											html.ID("manualMovie_overview"),
											html.Name("manualMovie_overview"),
											html.Placeholder("Movie description (optional)"),
											html.Rows("2"),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-secondary"),
										html.I(html.Class("fas fa-plus me-2")),
										gomponents.Text("Add Movie Manually"),
									),
								),
							),
						),
					),
				),
			),

			// Results area
			html.Div(
				html.ID("movieSearchResults"),
				html.Class("mt-4"),
			),

			movieSearchScript(csrfToken),
		),
	)
}

func seriesMetadataSearchContent(
	mediaConfigs, qualityProfiles []string,
	csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-tv header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Series Metadata Search")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search for TV series across metadata sources and add them to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Search Form Card
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text("Search Series"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("seriesSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("series_title"),
										html.Class("form-label"),
										gomponents.Text("Series Title"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("series_title"),
										html.Name("series_title"),
										html.Placeholder("Enter series title to search..."),
										gomponents.Attr("required", "true"),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("series_list"),
										html.Class("form-label"),
										gomponents.Text("Add to List"),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("series_list"),
										html.Name("series_list"),
										gomponents.Attr("required", "true"),
										renderSelectOptions(mediaConfigs, ""),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("series_quality_profile"),
										html.Class("form-label"),
										gomponents.Text("Quality Profile"),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("series_quality_profile"),
										html.Name("series_quality_profile"),
										renderSelectOptions(qualityProfiles, ""),
									),
									html.Small(
										html.Class("form-text text-muted"),
										gomponents.Text(
											"Leave to use the list's default quality.",
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text("Search Series"),
									),
								),
							),
						),
					),
				),

				// Manual Entry Card
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-edit me-2")),
								gomponents.Text("Manual Entry"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("seriesManualForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-8 mb-3"),
										html.Label(
											html.For("manualSeries_seriename"),
											html.Class("form-label"),
											gomponents.Text("Series Name *"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualSeries_seriename"),
											html.Name("manualSeries_seriename"),
											html.Placeholder("Enter series name"),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List *"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualSeries_list"),
											html.Name("manualSeries_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_thetvdb_id"),
											html.Class("form-label"),
											gomponents.Text("TVDB ID"),
										),
										html.Input(
											html.Type("number"),
											html.Class("form-control"),
											html.ID("manualSeries_thetvdb_id"),
											html.Name("manualSeries_thetvdb_id"),
											html.Placeholder("123456"),
											gomponents.Attr("min", "1"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_firstaired"),
											html.Class("form-label"),
											gomponents.Text("First Aired"),
										),
										html.Input(
											html.Type("date"),
											html.Class("form-control"),
											html.ID("manualSeries_firstaired"),
											html.Name("manualSeries_firstaired"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_network"),
											html.Class("form-label"),
											gomponents.Text("Network"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualSeries_network"),
											html.Name("manualSeries_network"),
											html.Placeholder("ABC, NBC, etc."),
										),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("manualSeries_overview"),
										html.Class("form-label"),
										gomponents.Text("Overview"),
									),
									html.Textarea(
										html.Class("form-control"),
										html.ID("manualSeries_overview"),
										html.Name("manualSeries_overview"),
										html.Placeholder("Series description (optional)"),
										html.Rows("3"),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-secondary"),
										html.I(html.Class("fas fa-plus me-2")),
										gomponents.Text("Add Series Manually"),
									),
								),
							),
						),
					),
				),
			),

			// Results area
			html.Div(
				html.ID("seriesSearchResults"),
				html.Class("mt-4"),
			),

			seriesSearchScript(csrfToken),
		),
	)
}

// renderSelectOptions renders options for a select element.
func renderSelectOptions(options []string, selectedValue string) gomponents.Node {
	nodes := make([]gomponents.Node, 0, len(options)+1)

	nodes = append(nodes, html.Option(html.Value(""), gomponents.Text("Select...")))

	for _, opt := range options {
		if opt == selectedValue {
			nodes = append(
				nodes,
				html.Option(
					html.Value(opt),
					gomponents.Attr("selected", "selected"),
					gomponents.Text(opt),
				),
			)
		} else {
			nodes = append(nodes, html.Option(html.Value(opt), gomponents.Text(opt)))
		}
	}

	return gomponents.Group(nodes)
}

// SearchMovieMetadata handles AJAX movie search requests.
func SearchMovieMetadata(c *gin.Context) {
	title := c.PostForm("movie_title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TMDB provider not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.SearchMovies(ctx, title, 0)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search movies")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to search movies: " + err.Error()},
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No movies found for: "+title),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(fmt.Sprintf("Search Results (%d movies found)", len(results))),
	))

	for _, movie := range results {
		resultNodes = append(resultNodes, createMovieResultCard(movie))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// SearchSeriesMetadata handles AJAX series search requests.
func SearchSeriesMetadata(c *gin.Context) {
	title := c.PostForm("series_title")
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title is required"})
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TMDB provider not available"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.SearchSeries(ctx, title, 0)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search series")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to search series: " + err.Error()},
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No series found for: "+title),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(fmt.Sprintf("Search Results (%d series found)", len(results))),
	))

	for _, series := range results {
		resultNodes = append(resultNodes, createSeriesResultCard(series))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

func createMovieResultCard(movie apiexternal_v2.MovieSearchResult) gomponents.Node {
	year := ""
	if movie.Year > 0 {
		year = fmt.Sprintf(" (%d)", movie.Year)
	}

	overview := movie.Overview
	if len(overview) > 200 {
		overview = overview[:200] + "..."
	}

	rating := ""
	if movie.VoteAverage > 0 {
		rating = fmt.Sprintf("%.1f", movie.VoteAverage)
	}

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				html.Div(
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(movie.Title+year),
					),
					func() gomponents.Node {
						if movie.OriginalTitle != "" && movie.OriginalTitle != movie.Title {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text("Original: "+movie.OriginalTitle),
							)
						}

						return nil
					}(),
					html.P(
						html.Class("card-text"),
						gomponents.Text(overview),
					),
					func() gomponents.Node {
						if rating != "" {
							return html.Span(
								html.Class("badge bg-warning text-dark me-2"),
								html.I(html.Class("fas fa-star me-1")),
								gomponents.Text(rating),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Class("btn btn-success add-movie-btn"),
					gomponents.Attr("data-tmdb-id", strconv.Itoa(movie.ID)),
					gomponents.Attr("data-title", movie.Title),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text("Add Movie"),
				),
			),
		),
	)
}

func createSeriesResultCard(series apiexternal_v2.SeriesSearchResult) gomponents.Node {
	year := ""
	if !series.FirstAirDate.IsZero() {
		year = fmt.Sprintf(" (%d)", series.FirstAirDate.Year())
	}

	overview := series.Overview
	if len(overview) > 200 {
		overview = overview[:200] + "..."
	}

	rating := ""
	if series.VoteAverage > 0 {
		rating = fmt.Sprintf("%.1f", series.VoteAverage)
	}

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				html.Div(
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(series.Name+year),
					),
					func() gomponents.Node {
						if series.OriginalName != "" && series.OriginalName != series.Name {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text("Original: "+series.OriginalName),
							)
						}

						return nil
					}(),
					html.P(
						html.Class("card-text"),
						gomponents.Text(overview),
					),
					func() gomponents.Node {
						if rating != "" {
							return html.Span(
								html.Class("badge bg-warning text-dark me-2"),
								html.I(html.Class("fas fa-star me-1")),
								gomponents.Text(rating),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Class("btn btn-success add-series-btn"),
					gomponents.Attr("data-tmdb-id", strconv.Itoa(series.ID)),
					gomponents.Attr("data-title", series.Name),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text("Add Series"),
				),
			),
		),
	)
}

// AddMovieToDatabase handles adding a movie from metadata sources to the database.
func AddMovieToDatabase(c *gin.Context) {
	tmdbIDStr := c.PostForm("tmdb_id")
	listName := c.PostForm("movie_list")
	qualityProfile := c.PostForm("movie_quality_profile")

	if tmdbIDStr == "" || listName == "" || qualityProfile == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "TMDB ID, list name, and quality profile are required"},
		)

		return
	}

	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid TMDB ID"})
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TMDB provider not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	movieDetails, err := provider.GetMovieByID(ctx, tmdbID)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to get movie details")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to get movie details: " + err.Error()},
		)

		return
	}

	// Check if movie already exists in dbmovies
	movieExists := database.GetrowsN[int](
		false,
		1,
		"SELECT id FROM dbmovies WHERE moviedb_id = ?",
		tmdbID,
	)

	var dbMovieID int

	nowTime := time.Now()

	if len(movieExists) == 0 {
		newID, err := database.ExecNid(
			"INSERT INTO dbmovies (title, release_date, year, adult, budget, genres, original_language, original_title, overview, popularity, revenue, runtime, spoken_languages, status, tagline, vote_average, vote_count, moviedb_id, imdb_id, freebase_m_id, freebase_id, facebook_id, instagram_id, twitter_id, url, backdrop, poster, slug, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			movieDetails.Title,
			movieDetails.ReleaseDate,
			getYearFromDate(movieDetails.ReleaseDate),
			movieDetails.Adult,
			movieDetails.Budget,
			genresToString(movieDetails.Genres),
			movieDetails.OriginalLanguage,
			movieDetails.OriginalTitle,
			movieDetails.Overview,
			movieDetails.Popularity,
			movieDetails.Revenue,
			movieDetails.Runtime,
			spokenLanguagesToString(movieDetails.SpokenLanguages),
			movieDetails.Status,
			movieDetails.Tagline,
			movieDetails.VoteAverage,
			movieDetails.VoteCount,
			tmdbID,
			movieDetails.IMDbID,
			"",
			"",
			"",
			"",
			"",
			"",
			movieDetails.BackdropPath,
			movieDetails.PosterPath,
			"",
			0,
			nowTime,
			nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert movie")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Failed to add movie to database: " + err.Error()},
			)

			return
		}

		dbMovieID = int(newID)
	} else {
		dbMovieID = movieExists[0]
	}

	// Check if movie is already in the specified list
	listExists := database.GetrowsN[int](
		false,
		1,
		"SELECT count() FROM movies WHERE dbmovie_id = ? AND listname = ?",
		dbMovieID,
		listName,
	)
	if len(listExists) > 0 && listExists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "Movie already exists in list '" + listName + "'"},
		)

		return
	}

	// Add to movies table
	database.ExecN(
		"INSERT INTO movies (dbmovie_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, ?, 0, 0, 0, ?, ?)",
		dbMovieID,
		listName,
		qualityProfile,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Movie added successfully to " + listName})
}

// AddSeriesToDatabase handles adding a series to the database.
func AddSeriesToDatabase(c *gin.Context) {
	tmdbIDStr := c.PostForm("tmdb_id")
	listName := c.PostForm("series_list")

	if tmdbIDStr == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "TMDB ID and list name are required"})
		return
	}

	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid TMDB ID"})
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "TMDB provider not initialized"})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	series, err := provider.GetSeriesByID(ctx, tmdbID)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to get series details")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to get series details: " + err.Error()},
		)

		return
	}

	tvdbID := series.TVDbID
	if tvdbID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "TVDB ID not found for this series"})
		return
	}

	// Check if series already exists in dbseries
	seriesExists := database.GetrowsN[int](
		false,
		1,
		"SELECT id FROM dbseries WHERE thetvdb_id = ?",
		tvdbID,
	)

	var dbSeriesID int

	nowTime := time.Now()

	if len(seriesExists) == 0 {
		newID, err := database.ExecNid(
			"INSERT INTO dbseries (seriename, season, status, firstaired, network, runtime, language, genre, overview, rating, siterating, siterating_count, slug, imdb_id, thetvdb_id, freebase_m_id, freebase_id, tvrage_id, facebook, instagram, twitter, banner, fanart, poster, identifiedby, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			series.Name,
			"",
			series.Status,
			series.FirstAirDate,
			getNetworkString(series.Networks),
			getRuntimeFromEpisodeRuntime(series.EpisodeRunTime),
			series.OriginalLanguage,
			genresToString(series.Genres),
			series.Overview,
			"",
			fmt.Sprintf("%.1f", series.VoteAverage),
			fmt.Sprintf("%d", series.VoteCount),
			"",
			series.IMDbID,
			tvdbID,
			"",
			"",
			0,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			0,
			nowTime,
			nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert series")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Failed to add series to database: " + err.Error()},
			)

			return
		}

		dbSeriesID = int(newID)
	} else {
		dbSeriesID = seriesExists[0]
	}

	// Check if series is already in the specified list
	listExists := database.GetrowsN[int](
		false,
		1,
		"SELECT count() FROM series WHERE dbserie_id = ? AND listname = ?",
		dbSeriesID,
		listName,
	)
	if len(listExists) > 0 && listExists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "Series already exists in list '" + listName + "'"},
		)

		return
	}

	// Resolve the wanted quality for this list so the series row (and the episodes
	// imported from it) carry the correct profile instead of an arbitrary one. An
	// explicitly selected profile wins; otherwise fall back to the list's quality
	// template, then the media group default.
	cfgp, listid := findSeriesCfgpAndListID(listName)

	qualityProfile := c.PostForm("series_quality_profile")
	if qualityProfile == "" && cfgp != nil && listid >= 0 {
		qualityProfile = cfgp.Lists[listid].TemplateQuality
		if qualityProfile == "" {
			qualityProfile = cfgp.TemplateQuality
		}
	}

	// Add to series table
	database.ExecN(
		"INSERT INTO series (dbserie_id, listname, rootpath, quality_profile, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', ?, 0, 0, ?, ?)",
		dbSeriesID,
		listName,
		qualityProfile,
		nowTime,
		nowTime,
	)

	// Kick off episode import in the background so the series is actually populated
	// (the previous flow added the series row but never imported any episodes).
	if cfgp != nil && listid >= 0 {
		serieName := series.Name

		worker.Dispatch(
			"add_series_"+strconv.Itoa(tvdbID)+"_"+listName,
			func(_ uint32, ctx context.Context) error {
				err := importfeed.JobImportDBSeries(
					ctx,
					&config.ManualConfig{Name: serieName, TvdbID: tvdbID},
					0,
					cfgp,
					listid,
				)
				logger.Logtype("info", 0).
					Str("series", serieName).
					Str("list", listName).
					Err(err).
					Msg("AddSeriesToDatabase: episode import completed")

				return nil
			},
			"Data",
		)
	}

	c.JSON(http.StatusOK, gin.H{"success": "Series added successfully to " + listName})
}

// findSeriesCfgpAndListID resolves a series list name to its media config and list index.
func findSeriesCfgpAndListID(listName string) (*config.MediaTypeConfig, int) {
	allMedia := config.GetSettingsMediaAll()
	if allMedia == nil {
		return nil, -1
	}

	for i := range allMedia.Series {
		cfgp := config.GetSettingsMedia("serie_" + allMedia.Series[i].Name)
		if cfgp == nil {
			continue
		}

		if listid, ok := cfgp.ListsMapIdx[listName]; ok {
			return cfgp, listid
		}
	}

	return nil, -1
}

// AddMovieManual handles manual movie entry.
func AddMovieManual(c *gin.Context) {
	title := c.PostForm("manualMovie_title")
	yearStr := c.PostForm("manualMovie_year")
	listName := c.PostForm("manualMovie_list")
	qualityProfile := c.PostForm("manualMovie_quality_profile")
	imdbID := c.PostForm("manualMovie_imdb_id")
	overview := c.PostForm("manualMovie_overview")

	if title == "" || listName == "" || qualityProfile == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "Title, list name, and quality profile are required"},
		)

		return
	}

	year := 0
	if yearStr != "" {
		if parsedYear, err := strconv.Atoi(yearStr); err == nil {
			year = parsedYear
		}
	}

	// Check if movie with same title and year already exists
	var exists []int
	if year > 0 {
		exists = database.GetrowsN[int](
			false,
			1,
			"SELECT count() FROM dbmovies WHERE title = ? AND year = ?",
			title,
			year,
		)
	} else {
		exists = database.GetrowsN[int](
			false,
			1,
			"SELECT count() FROM dbmovies WHERE title = ?",
			title,
		)
	}

	if len(exists) > 0 && exists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "Movie with same title and year already exists in database"},
		)

		return
	}

	nowTime := time.Now()

	newID, err := database.ExecNid(
		"INSERT INTO dbmovies (title, release_date, year, adult, budget, genres, original_language, original_title, overview, popularity, revenue, runtime, spoken_languages, status, tagline, vote_average, vote_count, moviedb_id, imdb_id, freebase_m_id, freebase_id, facebook_id, instagram_id, twitter_id, url, backdrop, poster, slug, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		title,
		"",
		year,
		false,
		0,
		"",
		"",
		title,
		overview,
		0.0,
		0,
		0,
		"",
		"",
		"",
		0.0,
		0,
		0,
		imdbID,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		0,
		nowTime,
		nowTime,
	)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to insert manual movie")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add movie to database"})
		return
	}

	database.ExecN(
		"INSERT INTO movies (dbmovie_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, ?, 0, 0, 0, ?, ?)",
		int(newID),
		listName,
		qualityProfile,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Movie added manually to " + listName})
}

// AddSeriesManual handles manual series entry.
func AddSeriesManual(c *gin.Context) {
	seriename := c.PostForm("manualSeries_seriename")
	tvdbIDStr := c.PostForm("manualSeries_thetvdb_id")
	listName := c.PostForm("manualSeries_list")
	firstaired := c.PostForm("manualSeries_firstaired")
	network := c.PostForm("manualSeries_network")
	overview := c.PostForm("manualSeries_overview")

	if seriename == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Series name and list name are required"})
		return
	}

	tvdbID := 0
	if tvdbIDStr != "" {
		if parsedID, err := strconv.Atoi(tvdbIDStr); err == nil {
			tvdbID = parsedID
		}
	}

	// Check if series already exists
	var exists []int
	if tvdbID > 0 {
		exists = database.GetrowsN[int](
			false,
			1,
			"SELECT count() FROM dbseries WHERE thetvdb_id = ?",
			tvdbID,
		)
	} else {
		exists = database.GetrowsN[int](
			false,
			1,
			"SELECT count() FROM dbseries WHERE seriename = ?",
			seriename,
		)
	}

	if len(exists) > 0 && exists[0] > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Series already exists in database"})
		return
	}

	nowTime := time.Now()

	newID, err := database.ExecNid(
		"INSERT INTO dbseries (seriename, season, status, firstaired, network, runtime, language, genre, overview, rating, siterating, siterating_count, slug, imdb_id, thetvdb_id, freebase_m_id, freebase_id, tvrage_id, facebook, instagram, twitter, banner, fanart, poster, identifiedby, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		seriename,
		"",
		"",
		"",
		firstaired,
		network,
		"",
		"",
		"",
		overview,
		"",
		"",
		"",
		"",
		"",
		tvdbID,
		"",
		"",
		0,
		"",
		"",
		"",
		"",
		"",
		"",
		"",
		0,
		nowTime,
		nowTime,
	)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to insert manual series")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add series to database"})
		return
	}

	qualityProfile := ""
	if cfgp, listid := findSeriesCfgpAndListID(listName); cfgp != nil && listid >= 0 {
		qualityProfile = cfgp.Lists[listid].TemplateQuality
		if qualityProfile == "" {
			qualityProfile = cfgp.TemplateQuality
		}
	}

	database.ExecN(
		"INSERT INTO series (dbserie_id, listname, rootpath, quality_profile, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', ?, 0, 0, ?, ?)",
		int(newID),
		listName,
		qualityProfile,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Series added manually to " + listName})
}

// Helper functions

func getYearFromDate(t time.Time) int {
	if t.IsZero() {
		return 0
	}

	return t.Year()
}

func genresToString(genres []apiexternal_v2.Genre) string {
	if len(genres) == 0 {
		return ""
	}

	var result strings.Builder
	for i, genre := range genres {
		if i > 0 {
			result.WriteString(", ")
		}

		result.WriteString(genre.Name)
	}

	return result.String()
}

func spokenLanguagesToString(languages []apiexternal_v2.SpokenLanguage) string {
	if len(languages) == 0 {
		return ""
	}

	var result strings.Builder
	for i, lang := range languages {
		if i > 0 {
			result.WriteString(", ")
		}

		result.WriteString(lang.Name)
	}

	return result.String()
}

func getNetworkString(networks []apiexternal_v2.Network) string {
	if len(networks) == 0 {
		return ""
	}

	return networks[0].Name
}

func getRuntimeFromEpisodeRuntime(episodeRuntime []int) int {
	if len(episodeRuntime) == 0 {
		return 0
	}

	return episodeRuntime[0]
}

func movieSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
document.getElementById('movieSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('movie_title').value;
	const list = document.getElementById('movie_list').value;
	const qualityProfile = document.getElementById('movie_quality_profile').value;

	if (!title || !list || !qualityProfile) {
		alert('Please fill in all fields including quality profile');
		return;
	}

	const resultsDiv = document.getElementById('movieSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">Loading...</span></div><p class="mt-2">Searching movies...</p></div>';

	fetch('/api/admin/search/movies', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'movie_title=' + encodeURIComponent(title)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;

		document.querySelectorAll('.add-movie-btn').forEach(btn => {
			btn.addEventListener('click', function() {
				const tmdbId = this.getAttribute('data-tmdb-id');
				const movieTitle = this.getAttribute('data-title');
				const selectedList = document.getElementById('movie_list').value;
				const selectedQualityProfile = document.getElementById('movie_quality_profile').value;

				if (!selectedList || !selectedQualityProfile) {
					alert('Please select both a list and quality profile');
					return;
				}

				confirmAction('Please confirm', 'Add "' + movieTitle + '" to list "' + selectedList + '" with quality profile "' + selectedQualityProfile + '"?', function() {
					addMovieToDatabase(tmdbId, selectedList, selectedQualityProfile, this);
				})
			});
		});
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for movies. Please try again.</div>';
	});
});

function addMovieToDatabase(tmdbId, listName, qualityProfile, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
	button.disabled = true;

	fetch('/api/admin/add/movie', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'tmdb_id=' + tmdbId + '&movie_list=' + encodeURIComponent(listName) + '&movie_quality_profile=' + encodeURIComponent(qualityProfile)
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>Added!';
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add movie'));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding movie to database');
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('movieManualForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const formData = new FormData(this);
	const data = new URLSearchParams(formData);

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Adding...';
	submitBtn.disabled = true;

	fetch('/api/admin/add/movie/manual', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: data
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert('Success: ' + data.success);
			this.reset();
			submitBtn.innerHTML = '<i class="fas fa-check me-2"></i>Added!';
			setTimeout(() => {
				submitBtn.innerHTML = originalText;
				submitBtn.disabled = false;
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add movie'));
			submitBtn.innerHTML = originalText;
			submitBtn.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding movie manually');
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
`))
}

func seriesSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
document.getElementById('seriesSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('series_title').value;
	const list = document.getElementById('series_list').value;

	if (!title || !list) {
		alert('Please fill in all fields');
		return;
	}

	const resultsDiv = document.getElementById('seriesSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">Loading...</span></div><p class="mt-2">Searching series...</p></div>';

	fetch('/api/admin/search/series', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'series_title=' + encodeURIComponent(title)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;

		document.querySelectorAll('.add-series-btn').forEach(btn => {
			btn.addEventListener('click', function() {
				const tmdbId = this.getAttribute('data-tmdb-id');
				const seriesTitle = this.getAttribute('data-title');
				const selectedList = document.getElementById('series_list').value;
				const selectedQualityProfile = document.getElementById('series_quality_profile').value;
				const addBtn = this;

				confirmAction('Please confirm', 'Add "' + seriesTitle + '" to list "' + selectedList + '"?', function() {
					addSeriesToDatabase(tmdbId, selectedList, selectedQualityProfile, addBtn);
				})
			});
		});
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for series. Please try again.</div>';
	});
});

function addSeriesToDatabase(tmdbId, listName, qualityProfile, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
	button.disabled = true;

	fetch('/api/admin/add/series', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'tmdb_id=' + tmdbId + '&series_list=' + encodeURIComponent(listName) + '&series_quality_profile=' + encodeURIComponent(qualityProfile || '')
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>Added!';
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add series'));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding series to database');
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('seriesManualForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const formData = new FormData(this);
	const data = new URLSearchParams(formData);

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Adding...';
	submitBtn.disabled = true;

	fetch('/api/admin/add/series/manual', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: data
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert('Success: ' + data.success);
			this.reset();
			submitBtn.innerHTML = '<i class="fas fa-check me-2"></i>Added!';
			setTimeout(() => {
				submitBtn.innerHTML = originalText;
				submitBtn.disabled = false;
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add series'));
			submitBtn.innerHTML = originalText;
			submitBtn.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding series manually');
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
`))
}

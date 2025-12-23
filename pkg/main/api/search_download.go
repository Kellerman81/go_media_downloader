package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// ================================================================================
// SEARCH AND DOWNLOAD PAGE
// ================================================================================

// renderSearchDownloadPage renders a page for searching and downloading content.
func renderSearchDownloadPage(csrfToken string) gomponents.Node {
	// Get available media configurations
	media := config.GetSettingsMediaAll()

	var mediaConfigs []string
	for i := range media.Movies {
		mediaConfigs = append(mediaConfigs, media.Movies[i].NamePrefix)
	}

	for i := range media.Series {
		mediaConfigs = append(mediaConfigs, media.Series[i].NamePrefix)
	}

	// Movie, series, and episode options will be populated dynamically via apiAdminDropdownData

	searchTypes := []string{
		"movies_rss", "movies_search", "series_rss", "series_search", "series_episode_search",
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-magnifying-glass-arrow-right header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Search & Download")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search for movies, TV series, and episodes across your configured indexers. View results and optionally download selected items.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("searchForm"),
			html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),

			html.Div(
				html.Class("form-cards-grid"),

				// Search Configuration Card
				html.Div(
					html.Class("form-card"),
					html.Div(
						html.Class("card-header"),
						html.I(html.Class("fas fa-cog card-icon")),
						html.H5(html.Class("card-title"), gomponents.Text("Search Configuration")),
						html.P(
							html.Class("card-subtitle"),
							gomponents.Text("Configure search type and settings"),
						),
					),
					html.Div(
						html.Class("card-body"),
						renderFormGroup("search", map[string]string{
							"SearchType": "Type of search to perform",
						}, map[string]string{
							"SearchType": "Search Type",
						}, "SearchType", "select", "movies_search", map[string][]string{
							"options": searchTypes,
						}),

						renderFormGroup("search", map[string]string{
							"MediaConfig": "Media configuration to use for search",
						}, map[string]string{
							"MediaConfig": "Media Configuration",
						}, "MediaConfig", "select", "", map[string][]string{
							"options": mediaConfigs,
						}),

						renderFormGroup("search", map[string]string{
							"Limit": "Maximum number of results to return",
						}, map[string]string{
							"Limit": "Result Limit",
						}, "Limit", "number", "50", nil),

						renderFormGroup("search", map[string]string{
							"TitleSearch": "Use title-based search instead of ID-based search for better results",
						}, map[string]string{
							"TitleSearch": "Enable Title Search",
						}, "TitleSearch", "checkbox", false, nil),
					),
				),

				// Search Parameters Card
				html.Div(
					html.Class("form-card"),
					html.Div(
						html.Class("card-header"),
						html.I(html.Class("fas fa-target card-icon")),
						html.H5(html.Class("card-title"), gomponents.Text("Search Parameters")),
						html.P(
							html.Class("card-subtitle"),
							gomponents.Text("Select specific content to search for"),
						),
					),
					html.Div(
						html.Class("card-body"),

						html.Div(
							html.Class("form-group-enhanced mb-4"),
							html.Div(
								html.Class("form-field-card p-3 border rounded-3"),
								html.Style(
									"background: #ffffff; border: 1px solid #dee2e6 !important; transition: all 0.3s ease; box-shadow: 0 1px 2px rgba(0,0,0,0.03);",
								),
								html.Div(
									html.Class("d-flex align-items-center mb-2"),
									html.I(html.Class("fa-solid fa-list text-primary me-2")),
									createFormLabel("MovieID", "Movie", false),
								),

								html.Select(
									html.ID("MovieID"),
									html.Name("MovieID"),
									html.Class("form-select choices-ajax"),
									html.Data("ajax-url", "/api/admin/dropdown/movies/dbmovie_id"),
									html.Data("placeholder", "-- Select Movie --"),
									html.Data("allow-clear", "true"),
									html.Option(
										gomponents.Attr("value", ""),
										gomponents.Text("-- Select Movie --"),
									),
								),
							),
						),

						html.Div(
							html.Class("form-group-enhanced mb-4"),
							html.Div(
								html.Class("form-field-card p-3 border rounded-3"),
								html.Style(
									"background: #ffffff; border: 1px solid #dee2e6 !important; transition: all 0.3s ease; box-shadow: 0 1px 2px rgba(0,0,0,0.03);",
								),
								html.Div(
									html.Class("d-flex align-items-center mb-2"),
									html.I(html.Class("fa-solid fa-list text-primary me-2")),
									createFormLabel("SerieID", "Series", false),
								),
								html.Select(
									html.ID("SerieID"),
									html.Name("SerieID"),
									html.Class("form-select choices-ajax"),
									html.Data("ajax-url", "/api/admin/dropdown/series/dbserie_id"),
									html.Data("placeholder", "-- Select Series --"),
									html.Data("allow-clear", "true"),
									html.Option(
										gomponents.Attr("value", ""),
										gomponents.Text("-- Select Series --"),
									),
								),
							),
						),

						renderFormGroup("search", map[string]string{
							"SeasonNum": "Season number (for episode searches and series RSS with specific season)",
						}, map[string]string{
							"SeasonNum": "Season Number",
						}, "SeasonNum", "number", "", nil),

						html.Div(
							html.Class("form-group-enhanced mb-4"),
							html.Div(
								html.Class("form-field-card p-3 border rounded-3"),
								html.Style(
									"background: #ffffff; border: 1px solid #dee2e6 !important; transition: all 0.3s ease; box-shadow: 0 1px 2px rgba(0,0,0,0.03);",
								),
								html.Div(
									html.Class("d-flex align-items-center mb-2"),
									html.I(html.Class("fa-solid fa-list text-primary me-2")),
									createFormLabel("EpisodeNum", "Episode", false),
								),
								html.Select(
									html.ID("EpisodeNum"),
									html.Name("EpisodeNum"),
									html.Class("form-select"),
									html.Data("placeholder", "-- Select Series First --"),
									html.Data("allow-clear", "true"),
									html.Data("depends-on", "SerieID"),
									html.Option(
										gomponents.Attr("value", ""),
										gomponents.Text("-- Select Series First --"),
									),
								),
							),
						),
					),
				),
			),

			// Enhanced action buttons
			html.Div(
				html.Class("form-actions-enhanced"),
				html.Button(
					html.Class("btn-action-primary"),
					html.ID("searchButton"),
					html.Type("button"),
					hx.Target("#searchResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/searchdownload"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#searchForm"),
					html.I(html.Class("fas fa-search action-icon")),
					html.Span(
						html.Class("action-text"),
						html.ID("searchButtonText"),
						gomponents.Text("Search"),
					),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn-action-secondary"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('searchForm').reset(); document.getElementById('searchResults').innerHTML = '';",
					),
					html.I(html.Class("fas fa-undo action-icon")),
					html.Span(html.Class("action-text"), gomponents.Text("Reset Form")),
				),
			),
		),

		// Enhanced results container
		html.Div(
			html.Class("results-container-enhanced"),
			html.ID("searchResults"),
			// Search results will be injected here
		),

		// Enhanced help section with modern styling
		html.Div(
			html.Class("help-section-enhanced"),
			html.Div(
				html.Class("help-header"),
				html.I(html.Class("fas fa-info-circle help-icon")),
				html.H5(html.Class("help-title"), gomponents.Text("Search Types & Options")),
			),
			html.Div(
				html.Class("help-content"),
				html.Div(
					html.Class("help-grid"),
					html.Div(
						html.Class("help-card"),
						html.Div(html.Class("help-card-icon"), html.I(html.Class("fas fa-rss"))),
						html.Div(
							html.Class("help-card-content"),
							html.Strong(gomponents.Text("RSS Searches")),
							html.P(
								gomponents.Text(
									"Search RSS feeds for movies and TV series from your configured indexers",
								),
							),
						),
					),
					html.Div(
						html.Class("help-card"),
						html.Div(html.Class("help-card-icon"), html.I(html.Class("fas fa-search"))),
						html.Div(
							html.Class("help-card-content"),
							html.Strong(gomponents.Text("Targeted Search")),
							html.P(
								gomponents.Text(
									"Search indexers for specific movies, series, or episodes with ID-based matching",
								),
							),
						),
					),
					html.Div(
						html.Class("help-card"),
						html.Div(html.Class("help-card-icon"), html.I(html.Class("fas fa-filter"))),
						html.Div(
							html.Class("help-card-content"),
							html.Strong(gomponents.Text("Advanced Filtering")),
							html.P(
								gomponents.Text(
									"Season filtering for series and episode-specific searches with quality matching",
								),
							),
						),
					),
					html.Div(
						html.Class("help-card"),
						html.Div(
							html.Class("help-card-icon"),
							html.I(html.Class("fas fa-download")),
						),
						html.Div(
							html.Class("help-card-content"),
							html.Strong(gomponents.Text("Direct Download")),
							html.P(
								gomponents.Text(
									"View results with download links and quality information for immediate action",
								),
							),
						),
					),
				),
				html.Div(
					html.Class("help-tips"),
					html.Div(
						html.Class("tip-item"),
						html.I(html.Class("fas fa-lightbulb tip-icon")),
						html.Strong(gomponents.Text("Title Search: ")),
						gomponents.Text(
							"Enable for better search results using titles instead of just database IDs. Recommended for most searches.",
						),
					),
					html.Div(
						html.Class("tip-item"),
						html.I(html.Class("fas fa-calendar-alt tip-icon")),
						html.Strong(gomponents.Text("Season Filter: ")),
						gomponents.Text(
							"For series RSS searches, specify a season number to search only for that season's episodes.",
						),
					),
					html.Div(
						html.Class("tip-item"),
						html.I(html.Class("fas fa-shield-alt tip-icon")),
						html.Strong(gomponents.Text("Legal Notice: ")),
						gomponents.Text(
							"Search results will include download links. Use caution when downloading content and ensure compliance with your local laws.",
						),
					),
				),
			),
		),

		// CSS for loading indicator
		html.StyleEl(gomponents.Raw(`
			#searchButton {
				position: relative;
			}
			.htmx-indicator {
				display: none !important;
				position: absolute;
				top: 50%;
				left: 50%;
				transform: translate(-50%, -50%);
				white-space: nowrap;
			}
			.htmx-request .htmx-indicator {
				display: inline !important;
			}
			.htmx-request #searchButtonText {
				visibility: hidden; /* Use visibility instead of display to maintain button size */
			}
			/* Full page overlay during search */
			.search-overlay {
				display: none;
				position: fixed;
				top: 0;
				left: 0;
				width: 100%;
				height: 100%;
				background-color: rgba(0, 0, 0, 0.05);
				z-index: 9999;
				cursor: wait;
			}
			/* Show overlay when search button is making request */
			#searchButton.htmx-request ~ .search-overlay,
			.htmx-request .search-overlay {
				display: block;
			}
		`)),

		// JavaScript for dynamic field visibility and episode loading
		// Simplified JavaScript for Search Download - HTMX handles field visibility and episode loading
		html.Script(gomponents.Raw(`
			document.addEventListener('DOMContentLoaded', function() {
				// Initialize Choices.js for enhanced selects
				if (window.initChoicesGlobal) {
					window.initChoicesGlobal();
				}
				
				// Wait for Choices.js to initialize before setting up field visibility
				setTimeout(function() {
					setupFieldVisibility();
				}, 1000);
			});
			
			function setupFieldVisibility() {
				
				const searchTypeSelect = document.getElementById('search_SearchType');
				const movieSelect = document.getElementById('MovieID');
				const serieSelect = document.getElementById('SerieID');
				const seasonInput = document.querySelector('input[name="search_SeasonNum"]');
				const episodeSelect = document.getElementById('EpisodeNum');
				
				const movieFields = movieSelect ? movieSelect.closest('.form-group-enhanced') : null;
				const serieFields = serieSelect ? serieSelect.closest('.form-group-enhanced') : null;
				const seasonFields = seasonInput ? seasonInput.closest('.form-group') : null;
				const episodeFields = episodeSelect ? episodeSelect.closest('.form-group-enhanced') : null;
				
				function toggleFields() {
					if (!searchTypeSelect) {
						return;
					}
					
					const searchType = searchTypeSelect.value;
					
					// Hide all fields initially
					if (movieFields) movieFields.style.display = 'none';
					if (serieFields) serieFields.style.display = 'none';
					if (seasonFields) seasonFields.style.display = 'none';
					if (episodeFields) episodeFields.style.display = 'none';
					
					// Show relevant fields based on search type
					switch(searchType) {
						case 'movies_search':
						case 'movies_rss':
							if (movieFields) movieFields.style.display = 'block';
							break;
						case 'series_search':
						case 'series_rss':
							if (serieFields) serieFields.style.display = 'block';
							if (seasonFields) seasonFields.style.display = 'block';
							break;
						case 'series_episode_search':
							if (serieFields) serieFields.style.display = 'block';
							if (episodeFields) episodeFields.style.display = 'block';
							break;
					}
				}
				
				function loadEpisodes() {
					if (!serieSelect || !episodeSelect) return;
					
					const seriesValue = serieSelect.value;
					if (!seriesValue || seriesValue === '') {
						if (episodeSelect.choicesInstance) {
							episodeSelect.choicesInstance.clearChoices();
							episodeSelect.choicesInstance.setChoices([{
								value: '',
								label: '-- Select Series First --'
							}], 'value', 'label', true);
						}
						return;
					}
					
					// Series ID should be the direct value from Choices.js (just the ID)
					const seriesID = seriesValue;
					
					// Get CSRF token
					const csrfToken = document.querySelector('input[name="csrf_token"]').value;
					
					// Make AJAX request to load episodes  
					const formData = new URLSearchParams();
					formData.append('search', seriesID);
					formData.append('page', '1');
					
					fetch('/api/admin/dropdown/serie_episodes/serie_id', {
						method: 'POST',
						headers: {
							'Content-Type': 'application/x-www-form-urlencoded',
							'X-CSRF-Token': csrfToken
						},
						body: formData.toString()
					})
					.then(response => response.json())
					.then(data => {
						// Clear existing choices
						if (episodeSelect.choicesInstance) {
							episodeSelect.choicesInstance.clearChoices();
						}
						
						if (data.results && data.results.length > 0) {
							const choicesList = data.results.map(function(episode) {
								return {
									value: episode.id,
									label: episode.text
								};
							});
							
							if (episodeSelect.choicesInstance) {
								episodeSelect.choicesInstance.setChoices(choicesList, 'value', 'label', true);
							}
						}
					})
					.catch(error => {
						console.error('Error loading episodes:', error);
						if (episodeSelect.choicesInstance) {
							episodeSelect.choicesInstance.clearChoices();
							episodeSelect.choicesInstance.setChoices([{
								value: '',
								label: '-- Error Loading Episodes --'
							}], 'value', 'label', true);
						}
					});
				}
				
				if (searchTypeSelect) {
					searchTypeSelect.addEventListener('change', toggleFields);
				}
				if (serieSelect) {
					// Listen for Choices.js change events
					serieSelect.addEventListener('change', loadEpisodes);
					serieSelect.addEventListener('choice', loadEpisodes);
				}
				toggleFields(); // Initial setup
				
				// Initialize basic Choices.js for episode dropdown (no AJAX)
				setTimeout(function() {
					if (episodeSelect && !episodeSelect.classList.contains('choices-initialized')) {
						episodeSelect.choicesInstance = new Choices(episodeSelect, {
							placeholder: true,
							placeholderValue: '-- Select Series First --',
							removeItemButton: true,
							searchEnabled: false,
							allowHTML: true
						});
						episodeSelect.classList.add('choices-initialized');
					}
				}, 500);
			}
		`)),

		// Full page overlay to prevent clicks during search
		html.Div(
			html.Class("search-overlay"),
			html.ID("searchOverlay"),
		),
	)
}

// HandleSearchDownload handles search and download requests.
func HandleSearchDownload(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	searchType := c.PostForm("search_SearchType")
	mediaConfig := c.PostForm("search_MediaConfig")
	limitStr := c.PostForm("search_Limit")

	if searchType == "" || mediaConfig == "" {
		c.String(
			http.StatusOK,
			renderAlert("Please select search type and media configuration", "warning"),
		)

		return
	}

	limit := 50
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Get TitleSearch checkbox
	titleSearch := c.PostForm("search_TitleSearch") == "on"

	// Get search parameters based on type
	var (
		movieID, serieID, seasonNum, episodeID int
		err                                    error
	)

	if strings.Contains(searchType, "movies") {
		movieIDStr := c.PostForm("MovieID")
		if movieIDStr != "" {
			movieID, err = strconv.Atoi(movieIDStr)
			if err != nil {
				c.String(http.StatusOK, renderAlert("Invalid Movie ID", "danger"))
				return
			}
		}
	} else if strings.Contains(searchType, "series") {
		serieIDStr := c.PostForm("SerieID")
		if serieIDStr != "" {
			serieID, err = strconv.Atoi(serieIDStr)
			if err != nil {
				c.String(http.StatusOK, renderAlert("Invalid Series ID", "danger"))
				return
			}
		}

		// Get season number for all series searches (RSS and episode)
		seasonStr := c.PostForm("search_SeasonNum")
		if seasonStr != "" {
			seasonNum, _ = strconv.Atoi(seasonStr)
		}

		if strings.Contains(searchType, "episode") {
			episodeStr := c.PostForm("EpisodeNum")
			if episodeStr != "" {
				episodeID, _ = strconv.Atoi(episodeStr)
			}
		}
	}

	var searchResults *searcher.ConfigSearcher
	defer searchResults.Close()

	// Perform the search based on type
	results, err := performSearch(
		searchResults,
		searchType,
		mediaConfig,
		movieID,
		serieID,
		seasonNum,
		episodeID,
		limit,
		titleSearch,
	)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Search failed: "+err.Error(), "danger"))
		return
	}

	// Render search results
	c.String(http.StatusOK, renderSearchResults(results, searchType, mediaConfig))
}

// SearchResults represents the response from search API functions.
type SearchResults struct {
	Accepted []SearchResult `json:"accepted"`
	Denied   []SearchResult `json:"denied"`
}

// convertNzbwithprioToSearchResult converts apiexternal_v2.Nzbwithprio to SearchResult.
func convertNzbwithprioToSearchResult(nzb apiexternal_v2.Nzbwithprio) SearchResult {
	// Extract category from indexer if available
	category := "Unknown"
	if nzb.NZB.Indexer != nil {
		category = nzb.NZB.Indexer.Name
	}

	// Use current date since specific published date isn't available in struct
	date := "N/A"

	return SearchResult{
		Title:    nzb.NZB.Title,
		Size:     formatFileSize(nzb.NZB.Size),
		Indexer:  nzb.NZB.SourceEndpoint,
		Category: category,
		Link:     nzb.NZB.DownloadURL,
		Date:     date,
		Quality:  nzb.Quality,
		Reason:   nzb.Reason,
	}
}

// formatFileSize converts bytes to human readable format.
func formatFileSize(bytes int64) string {
	if bytes == 0 {
		return "Unknown"
	}

	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// performSearch executes real search calls based on the specified type and parameters.
func performSearch(
	searchResults *searcher.ConfigSearcher,
	searchType, mediaConfig string,
	movieID, serieID, seasonNum, episodeID, limit int,
	titleSearch bool,
) (*SearchResults, error) {
	// Get the media configuration
	var mediaTypeConfig *config.MediaTypeConfig

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if strings.EqualFold(media.NamePrefix, mediaConfig) {
			mediaTypeConfig = media
			return nil
		}

		return nil
	})

	if mediaTypeConfig == nil {
		return nil, fmt.Errorf("media configuration '%s' not found", mediaConfig)
	}

	ctx := context.Background()

	var err error

	switch searchType {
	case "movies_rss":
		// Call the actual movies RSS search
		searchResults = searcher.NewSearcher(
			mediaTypeConfig,
			mediaTypeConfig.CfgQuality,
			logger.StrRss,
			nil,
		)
		err = searchResults.SearchRSS(
			ctx,
			mediaTypeConfig,
			mediaTypeConfig.CfgQuality,
			false,
			false,
		)

	case "movies_search":
		// logger.Logtype("info", 1).Any("id", movieID).Msg("movie search")
		if movieID <= 0 {
			return &SearchResults{Accepted: []SearchResult{}, Denied: []SearchResult{}}, nil
		}

		// Get movie from database
		movie, _ := database.GetMovies(
			database.Querywithargs{Select: "id, listname", Where: "id = ?"},
			movieID,
		)
		if movie.ID == 0 {
			// logger.Logtype("error", 1).Any("id", movieID).Msg("movie db search")
			return nil, fmt.Errorf("movie with ID %d not found", movieID)
		}

		// Find the appropriate list config
		var listConfig *config.MediaListsConfig
		for _, list := range mediaTypeConfig.Lists {
			if strings.EqualFold(list.Name, movie.Listname) {
				listConfig = &list
				break
			}
		}

		if listConfig == nil {
			// logger.Logtype("info", 1).Any("id", movieID).Msg("movie list search")
			return nil, fmt.Errorf("list configuration for movie not found")
		}

		searchResults = searcher.NewSearcher(
			mediaTypeConfig,
			listConfig.CfgQuality,
			"search",
			&movie.ID,
		)
		err = searchResults.MediaSearch(ctx, mediaTypeConfig, movie.ID, titleSearch, false, false)

	case "series_rss":
		if serieID <= 0 {
			return &SearchResults{Accepted: []SearchResult{}, Denied: []SearchResult{}}, nil
		}

		// Get series from database
		serie, _ := database.GetSeries(
			database.Querywithargs{Select: "id, listname", Where: "id = ?"},
			serieID,
		)
		if serie.ID == 0 {
			return nil, fmt.Errorf("series with ID %d not found", serieID)
		}

		// Use season if provided, otherwise search all seasons
		useseason := seasonNum > 0

		seasonStr := ""
		if useseason {
			seasonStr = fmt.Sprintf("%d", seasonNum)
		}

		searchResults, err = searcher.SearchSerieRSSSeasonSingle(
			&serie.ID,
			seasonStr,
			useseason,
			mediaTypeConfig,
		)

	case "series_search":
		if serieID <= 0 {
			return &SearchResults{Accepted: []SearchResult{}, Denied: []SearchResult{}}, nil
		}

		// Get series from database
		serie, _ := database.GetSeries(
			database.Querywithargs{Select: "id, listname", Where: "id = ?"},
			serieID,
		)
		if serie.ID == 0 {
			return nil, fmt.Errorf("series with ID %d not found", serieID)
		}

		// Find the appropriate list config
		var listConfig *config.MediaListsConfig
		for _, list := range mediaTypeConfig.Lists {
			if strings.EqualFold(list.Name, serie.Listname) {
				listConfig = &list
				break
			}
		}

		if listConfig == nil {
			return nil, fmt.Errorf("list configuration for series not found")
		}

		searchResults = searcher.NewSearcher(
			mediaTypeConfig,
			listConfig.CfgQuality,
			"search",
			&serie.ID,
		)
		err = searchResults.MediaSearch(ctx, mediaTypeConfig, serie.ID, titleSearch, false, false)

	case "series_episode_search":
		if episodeID <= 0 {
			return &SearchResults{Accepted: []SearchResult{}, Denied: []SearchResult{}}, nil
		}

		// Get episode from database to get series info
		episode, _ := database.GetSerieEpisodes(
			database.Querywithargs{
				Select: "id, serie_id",
				Where:  "id = ?",
			},
			episodeID,
		)
		if episode.ID == 0 {
			return nil, fmt.Errorf("episode with ID %d not found", episodeID)
		}

		// Get series from database to get list config
		serie, _ := database.GetSeries(
			database.Querywithargs{Select: "id, listname", Where: "id = ?"},
			episode.SerieID,
		)
		if serie.ID == 0 {
			return nil, fmt.Errorf("series with ID %d not found", episode.SerieID)
		}

		// Find the appropriate list config
		var listConfig *config.MediaListsConfig
		for _, list := range mediaTypeConfig.Lists {
			if strings.EqualFold(list.Name, serie.Listname) {
				listConfig = &list
				break
			}
		}

		if listConfig == nil {
			return nil, fmt.Errorf("list configuration for series not found")
		}

		searchResults = searcher.NewSearcher(
			mediaTypeConfig,
			listConfig.CfgQuality,
			"search",
			&episode.ID,
		)
		err = searchResults.MediaSearch(ctx, mediaTypeConfig, episode.ID, titleSearch, false, false)

	default:
		return nil, fmt.Errorf("unsupported search type: %s", searchType)
	}

	if err != nil {
		if searchResults != nil {
			searchResults.Close()
		}
		return nil, fmt.Errorf("search failed: %w", err)
	}

	if searchResults == nil {
		return &SearchResults{Accepted: []SearchResult{}, Denied: []SearchResult{}}, nil
	}

	defer searchResults.Close()

	// Convert Nzbwithprio results to SearchResult
	var accepted, denied []SearchResult

	for _, nzb := range searchResults.Accepted {
		accepted = append(accepted, convertNzbwithprioToSearchResult(nzb))
	}

	for _, nzb := range searchResults.Denied {
		denied = append(denied, convertNzbwithprioToSearchResult(nzb))
	}

	// Apply limit to both accepted and denied
	if limit > 0 {
		if len(accepted) > limit {
			accepted = accepted[:limit]
		}

		if len(denied) > limit {
			denied = denied[:limit]
		}
	}

	return &SearchResults{
		Accepted: accepted,
		Denied:   denied,
	}, nil
}

// SearchResult represents a search result item.
type SearchResult struct {
	Title    string
	Size     string
	Indexer  string
	Category string
	Link     string
	Date     string
	Quality  string
	Reason   string
}

// renderSearchResults renders search results with separate accepted and denied datatables.
func renderSearchResults(results *SearchResults, searchType, mediaConfig string) string {
	if results == nil || (len(results.Accepted) == 0 && len(results.Denied) == 0) {
		return renderComponentToString(
			html.Div(
				html.Class("card border-0 shadow-sm border-warning mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-warning me-3"),
							html.I(html.Class("fas fa-search me-1")),
							gomponents.Text("Search"),
						),
						html.H5(
							html.Class("card-title mb-0 text-warning fw-bold"),
							gomponents.Text("No Results Found"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-0"),
						gomponents.Text(
							"No search results were found matching your criteria. Try adjusting your search parameters or check your indexer configurations.",
						),
					),
				),
			),
		)
	}

	totalResults := len(results.Accepted) + len(results.Denied)

	result := html.Div(
		html.Class("w-100"),

		// Summary Header
		html.Div(
			html.Class("card border-0 shadow-sm border-success mb-4"),
			html.Div(
				html.Class("card-body"),
				html.H5(html.Class("card-title fw-bold mb-3"), gomponents.Text("Search Results")),
				html.Div(html.Class("row g-3 mb-3"),
					html.Div(html.Class("col-md-4"),
						html.Div(
							html.Class("d-flex align-items-center"),
							html.Span(
								html.Class("badge bg-primary me-2"),
								html.I(html.Class("fas fa-search me-1")),
								gomponents.Text("Total"),
							),
							html.Span(
								html.Class("fw-bold text-primary"),
								gomponents.Text(fmt.Sprintf("%d", totalResults)),
							),
						),
					),
					html.Div(html.Class("col-md-4"),
						html.Div(
							html.Class("d-flex align-items-center"),
							html.Span(
								html.Class("badge bg-success me-2"),
								html.I(html.Class("fas fa-check me-1")),
								gomponents.Text("Accepted"),
							),
							html.Span(
								html.Class("fw-bold text-success"),
								gomponents.Text(fmt.Sprintf("%d", len(results.Accepted))),
							),
						),
					),
					html.Div(html.Class("col-md-4"),
						html.Div(
							html.Class("d-flex align-items-center"),
							html.Span(
								html.Class("badge bg-danger me-2"),
								html.I(html.Class("fas fa-times me-1")),
								gomponents.Text("Denied"),
							),
							html.Span(
								html.Class("fw-bold text-danger"),
								gomponents.Text(fmt.Sprintf("%d", len(results.Denied))),
							),
						),
					),
				),
				html.Div(html.Class("row g-2"),
					html.Div(html.Class("col-md-6"),
						html.Small(html.Class("text-muted"),
							html.I(html.Class("fas fa-tag me-1")),
							gomponents.Text("Search Type: "),
							html.Span(html.Class("fw-bold"), gomponents.Text(searchType)),
						),
					),
					html.Div(html.Class("col-md-6"),
						html.Small(html.Class("text-muted"),
							html.I(html.Class("fas fa-cog me-1")),
							gomponents.Text("Media Config: "),
							html.Span(html.Class("fw-bold"), gomponents.Text(mediaConfig)),
						),
					),
				),
			),
		),

		// Accepted Results Section - Full Width
		html.Div(
			html.Class("card border-0 shadow-sm border-success mb-5"),
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
							html.I(html.Class("fas fa-check me-1")),
							gomponents.Text("Accepted"),
						),
						html.H4(
							html.Class("card-title mb-0 text-success fw-bold"),
							gomponents.Text("Accepted Results"),
						),
					),
					html.Span(
						html.Class("badge bg-success"),
						gomponents.Text(fmt.Sprintf("%d", len(results.Accepted))),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				func() gomponents.Node {
					if len(results.Accepted) == 0 {
						return html.Div(
							html.Class("text-center p-5"),
							html.I(
								html.Class("fas fa-search mb-3"),
								html.Style("font-size: 3rem; color: #28a745; opacity: 0.3;"),
							),
							html.H5(
								html.Class("text-muted mb-2"),
								gomponents.Text("No Accepted Results"),
							),
							html.P(
								html.Class("text-muted small mb-0"),
								gomponents.Text(
									"No results met the acceptance criteria for this search.",
								),
							),
						)
					}

					return renderResultsTable(results.Accepted, "accepted", true)
				}(),
			),
		),

		// Denied Results Section - Full Width
		html.Div(
			html.Class("card border-0 shadow-sm border-danger mb-5"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #f8d7da 0%, #f1aeb5 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center justify-content-between"),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-danger me-3"),
							html.I(html.Class("fas fa-times me-1")),
							gomponents.Text("Denied"),
						),
						html.H4(
							html.Class("card-title mb-0 text-danger fw-bold"),
							gomponents.Text("Denied Results"),
						),
					),
					html.Span(
						html.Class("badge bg-danger"),
						gomponents.Text(fmt.Sprintf("%d", len(results.Denied))),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				func() gomponents.Node {
					if len(results.Denied) == 0 {
						return html.Div(
							html.Class("text-center p-5"),
							html.I(
								html.Class("fas fa-filter mb-3"),
								html.Style("font-size: 3rem; color: #dc3545; opacity: 0.3;"),
							),
							html.H5(
								html.Class("text-muted mb-2"),
								gomponents.Text("No Denied Results"),
							),
							html.P(
								html.Class("text-muted small mb-0"),
								gomponents.Text("No results were filtered out during this search."),
							),
						)
					}

					return renderResultsTable(results.Denied, "denied", true)
				}(),
			),
		),

		// Information Footer
		html.Div(
			html.Class("card border-0 shadow-sm border-info"),
			html.Div(
				html.Class("card-body"),
				html.H6(
					html.Class("card-title fw-bold mb-3"),
					gomponents.Text("Search Results Information"),
				),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Real-time search results from your configured indexers"),
				),
				html.Ul(html.Class("list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-success me-2"),
							html.I(html.Class("fas fa-check me-1")),
							gomponents.Text("Accepted"),
						),
						gomponents.Text("Results that passed quality and criteria filters"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-danger me-2"),
							html.I(html.Class("fas fa-times me-1")),
							gomponents.Text("Denied"),
						),
						gomponents.Text(
							"Results that were filtered out with reasons (quality, size, etc.)",
						),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-primary me-2"),
							html.I(html.Class("fas fa-search me-1")),
							gomponents.Text("Live Search"),
						),
						gomponents.Text("Queries actual indexers using your media configuration"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-info me-2"),
							html.I(html.Class("fas fa-chart-bar me-1")),
							gomponents.Text("Quality Analysis"),
						),
						gomponents.Text("Shows detailed quality matching and rejection reasons"),
					),
					html.Li(
						html.Class("mb-2"),
						html.Span(
							html.Class("badge bg-secondary me-2"),
							html.I(html.Class("fas fa-download me-1")),
							gomponents.Text("Download Available"),
						),
						gomponents.Text("Both accepted and denied results can be downloaded"),
					),
					html.Li(
						html.Class("mb-0"),
						html.Span(
							html.Class("badge bg-warning me-2"),
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Manual Override"),
						),
						gomponents.Text("Downloading denied results bypasses quality filters"),
					),
				),
			),
		),
	)

	return renderComponentToString(result)
}

// renderResultsTable creates a datatable for either accepted or denied results.
func renderResultsTable(
	results []SearchResult,
	tableType string,
	showDownload bool,
) gomponents.Node {
	// Create table headers
	headers := []gomponents.Node{
		html.Th(html.Class("sorting"), gomponents.Text("Title")),
		html.Th(html.Class("sorting"), gomponents.Text("Size")),
		html.Th(html.Class("sorting"), gomponents.Text("Quality")),
		html.Th(html.Class("sorting"), gomponents.Text("Indexer")),
		html.Th(html.Class("sorting"), gomponents.Text("Category")),
		html.Th(html.Class("sorting"), gomponents.Text("Date")),
		html.Th(html.Class("sorting"), gomponents.Text("Reason")),
	}

	if showDownload {
		headers = append(
			headers,
			html.Th(gomponents.Attr("data-orderable", "false"), gomponents.Text("Actions")),
		)
	}

	// Create table rows
	var rows []gomponents.Node
	for i, result := range results {
		rowCells := []gomponents.Node{
			html.Td(
				html.Class("font-monospace small"),
				gomponents.Text(result.Title),
			),
			html.Td(gomponents.Text(result.Size)),
			html.Td(
				html.Span(
					html.Class(func() string {
						if showDownload {
							return "badge bg-success"
						}
						return "badge bg-secondary"
					}()),
					gomponents.Text(result.Quality),
				),
			),
			html.Td(gomponents.Text(result.Indexer)),
			html.Td(
				html.Span(
					html.Class("badge bg-secondary"),
					gomponents.Text(result.Category),
				),
			),
			html.Td(gomponents.Text(result.Date)),
			html.Td(
				html.Small(
					html.Class(func() string {
						if showDownload {
							return "text-success"
						}
						return "text-danger"
					}()),
					gomponents.Text(result.Reason),
				),
			),
		}

		if showDownload {
			// Determine button style based on table type
			downloadBtnClass := "btn btn-success btn-sm"

			downloadBtnText := "Download"
			if tableType == "denied" {
				downloadBtnClass = "btn btn-warning btn-sm"
				downloadBtnText = "Force Download"
			}

			rowCells = append(rowCells,
				html.Td(
					html.Div(
						html.Class("btn-group-sm"),
						html.Button(
							html.Class(downloadBtnClass),
							gomponents.Text(downloadBtnText),
							html.Type("button"),
							hx.Post("/api/admin/searchdownload"),
							hx.Target(fmt.Sprintf("#download-result-%s-%d", tableType, i)),
							hx.Swap("innerHTML"),
							hx.Vals(
								fmt.Sprintf(
									`{"action": "download", "link": "%s", "title": "%s"}`,
									result.Link,
									result.Title,
								),
							),
						),
						html.A(
							html.Class("btn btn-info btn-sm ml-1"),
							html.Href(result.Link),
							html.Target("_blank"),
							gomponents.Text("Link"),
						),
					),
					html.Div(
						html.ID(fmt.Sprintf("download-result-%s-%d", tableType, i)),
						html.Style("min-height: 20px; margin-top: 5px;"),
					),
				),
			)
		}

		rows = append(rows, html.Tr(gomponents.Group(rowCells)))
	}

	tableID := fmt.Sprintf("%s-results-table", tableType)

	return html.Div(
		html.Class("table-responsive w-100"),
		html.Table(
			html.Class("table table-striped w-100"),
			html.ID(tableID),
			html.THead(
				html.Tr(gomponents.Group(headers)),
			),
			html.TBody(gomponents.Group(rows)),
		),
		html.Script(gomponents.Rawf(`
			$(document).ready(function() {
				if (window.initDataTable) {
					window.initDataTable('#%s', {
						lengthMenu: [[10, 25, 50, 100, -1], [10, 25, 50, 100, "All"]],
						order: [[ 0, "asc" ]],
						columnDefs: [
							{
								targets: "no-sort",
								orderable: false,
								searchable: false
							}
						],
						language: {
							search: "Filter %s results:",
							lengthMenu: "Show _MENU_ %s results per page",
							info: "Showing _START_ to _END_ of _TOTAL_ %s results",
							infoEmpty: "No %s results to show",
							infoFiltered: "(filtered from _MAX_ total results)",
							zeroRecords: "No matching %s results found"
						}
					});
				}
			});
		`, tableID, tableType, tableType, tableType, tableType, tableType)),
	)
}

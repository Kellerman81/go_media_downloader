package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/gin-gonic/gin"
	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// ================================================================================
// SEARCH AND DOWNLOAD PAGE
// ================================================================================

// renderSearchDownloadPage renders a page for searching and downloading content
func renderSearchDownloadPage(csrfToken string) Node {
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

	return Div(
		Class("config-section"),
		H3(Text("Search & Download")),
		P(Text("Search for movies, TV series, and episodes across your configured indexers. View results and optionally download selected items.")),

		Form(
			Class("config-form"),
			ID("searchForm"),
			Input(Type("hidden"), Name("csrf_token"), Value(csrfToken)),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Text("Search Configuration")),

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

				Div(
					Class("col-md-6"),
					H5(Text("Search Parameters")),

					Div(
						Class("form-group"),
						Label(
							For("MovieID"),
							Text("Movie"),
							Small(Class("form-text text-muted"), Text("Select movie to search for")),
						),
						Select(
							ID("MovieID"),
							Name("MovieID"),
							Class("form-control select2-ajax"),
							Data("ajax-url", "/api/admin/dropdown/movies/dbmovie_id"),
							Data("placeholder", "-- Select Movie --"),
							Data("allow-clear", "true"),
							Option(Attr("value", ""), Text("-- Select Movie --")),
						),
					),

					Div(
						Class("form-group"),
						Label(
							For("SerieID"),
							Text("Series"),
							Small(Class("form-text text-muted"), Text("Select series to search for")),
						),
						Select(
							ID("SerieID"),
							Name("SerieID"),
							Class("form-control select2-ajax"),
							Data("ajax-url", "/api/admin/dropdown/series/dbserie_id"),
							Data("placeholder", "-- Select Series --"),
							Data("allow-clear", "true"),
							Option(Attr("value", ""), Text("-- Select Series --")),
						),
					),

					renderFormGroup("search", map[string]string{
						"SeasonNum": "Season number (for episode searches and series RSS with specific season)",
					}, map[string]string{
						"SeasonNum": "Season Number",
					}, "SeasonNum", "number", "", nil),

					Div(
						Class("form-group"),
						Label(
							For("EpisodeNum"),
							Text("Episode"),
							Small(Class("form-text text-muted"), Text("Select episode to search for (select series first)")),
						),
						Select(
							ID("EpisodeNum"),
							Name("EpisodeNum"),
							Class("form-control"),
							Data("placeholder", "-- Select Series First --"),
							Data("allow-clear", "true"),
							Data("depends-on", "SerieID"),
							Option(Attr("value", ""), Text("-- Select Series First --")),
						),
					),
				),
			),

			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Search"),
					Type("button"),
					hx.Target("#searchResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/searchdownload"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#searchForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('searchForm').reset(); document.getElementById('searchResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("searchResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// Instructions
		Div(
			Class("mt-4 alert alert-info"),
			H5(Text("Search Type Descriptions:")),
			Ul(
				Li(Strong(Text("movies_rss: ")), Text("Search RSS feeds for movies")),
				Li(Strong(Text("movies_search: ")), Text("Search indexers for specific movies (supports title search)")),
				Li(Strong(Text("series_rss: ")), Text("Search RSS feeds for TV series (supports season filtering)")),
				Li(Strong(Text("series_search: ")), Text("Search indexers for specific series (supports title search)")),
				Li(Strong(Text("series_episode_search: ")), Text("Search for specific episodes (supports title search)")),
			),
			P(
				Class("mt-2"),
				Strong(Text("Title Search: ")),
				Text("Enable for better search results using titles instead of just database IDs. Recommended for most searches."),
			),
			P(
				Class("mt-2"),
				Strong(Text("Season Filter: ")),
				Text("For series RSS searches, specify a season number to search only for that season's episodes."),
			),
			P(
				Class("mt-2"),
				Strong(Text("Note: ")),
				Text("Search results will include download links. Use caution when downloading content and ensure compliance with your local laws."),
			),
		),

		// JavaScript for dynamic field visibility and episode loading
		Script(Raw(`
			document.addEventListener('DOMContentLoaded', function() {
				const searchTypeSelect = document.querySelector('select[name="search_SearchType"]');
				const movieSelect = document.querySelector('select[name="MovieID"]');
				const serieSelect = document.querySelector('select[name="SerieID"]');
				const seasonInput = document.querySelector('input[name="search_SeasonNum"]');
				const episodeSelect = document.querySelector('select[name="EpisodeNum"]');
				
				const movieFields = movieSelect ? movieSelect.closest('.form-group') : null;
				const serieFields = serieSelect ? serieSelect.closest('.form-group') : null;
				const seasonFields = seasonInput ? seasonInput.closest('.form-group') : null;
				const episodeFields = episodeSelect ? episodeSelect.closest('.form-group') : null;
				
				function toggleFields() {
					if (!searchTypeSelect) return;
					
					const searchType = searchTypeSelect.value;
					
					// Hide all fields initially
					if (movieFields) movieFields.style.display = 'none';
					if (serieFields) serieFields.style.display = 'none';
					if (seasonFields) seasonFields.style.display = 'none';
					if (episodeFields) episodeFields.style.display = 'none';
					
					// Show relevant fields based on search type
					if (searchType.includes('movies')) {
						if (movieFields) movieFields.style.display = 'block';
					} else if (searchType.includes('series')) {
						if (serieFields) serieFields.style.display = 'block';
						if (searchType.includes('episode')) {
							if (seasonFields) seasonFields.style.display = 'block';
							if (episodeFields) episodeFields.style.display = 'block';
						} else if (searchType === 'series_rss') {
							// Show season field for RSS searches (optional)
							if (seasonFields) seasonFields.style.display = 'block';
						}
					}
				}
				
				function loadEpisodes() {
					if (!serieSelect || !episodeSelect) return;
					
					const seriesValue = serieSelect.value;
					if (!seriesValue || seriesValue === '') {
						episodeSelect.innerHTML = '<option value="">-- Select Series First --</option>';
						return;
					}
					
					// Series ID should be the direct value from Select2 (just the ID)
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
						episodeSelect.innerHTML = '<option value="">-- Select Episode --</option>';
						if (data.results && data.results.length > 0) {
							data.results.forEach(function(episode) {
								const option = document.createElement('option');
								option.value = episode.id;
								option.textContent = episode.text;
								episodeSelect.appendChild(option);
							});
						}
					})
					.catch(error => {
						console.error('Error loading episodes:', error);
						episodeSelect.innerHTML = '<option value="">-- Error Loading Episodes --</option>';
					});
				}
				
				if (searchTypeSelect) {
					searchTypeSelect.addEventListener('change', toggleFields);
				}
				if (serieSelect) {
					// Listen for both regular change and Select2 change events
					serieSelect.addEventListener('change', loadEpisodes);
					$(serieSelect).on('change', loadEpisodes);
				}
				toggleFields(); // Initial setup
				
				// Initialize Select2 dropdowns
				if (window.initSelect2Global) {
					window.initSelect2Global();
				}
				
				// Initialize basic Select2 for episode dropdown (no AJAX)
				if (episodeSelect) {
					$(episodeSelect).select2({
						placeholder: '-- Select Series First --',
						allowClear: true,
						width: '100%'
					});
				}
			});
		`)),
	)
}

// HandleSearchDownload handles search and download requests
func HandleSearchDownload(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	searchType := c.PostForm("search_SearchType")
	mediaConfig := c.PostForm("search_MediaConfig")
	limitStr := c.PostForm("search_Limit")

	if searchType == "" || mediaConfig == "" {
		c.String(http.StatusOK, renderAlert("Please select search type and media configuration", "warning"))
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
	var movieID, serieID, seasonNum, episodeID int
	var err error

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
	results, err := performSearch(searchResults, searchType, mediaConfig, movieID, serieID, seasonNum, episodeID, limit, titleSearch)
	if err != nil {
		c.String(http.StatusOK, renderAlert("Search failed: "+err.Error(), "danger"))
		return
	}

	// Render search results
	c.String(http.StatusOK, renderSearchResults(results, searchType, mediaConfig))
}

// SearchResults represents the response from search API functions
type SearchResults struct {
	Accepted []SearchResult `json:"accepted"`
	Denied   []SearchResult `json:"denied"`
}

// convertNzbwithprioToSearchResult converts apiexternal.Nzbwithprio to SearchResult
func convertNzbwithprioToSearchResult(nzb apiexternal.Nzbwithprio) SearchResult {
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

// formatFileSize converts bytes to human readable format
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

// performSearch executes real search calls based on the specified type and parameters
func performSearch(searchResults *searcher.ConfigSearcher, searchType, mediaConfig string, movieID, serieID, seasonNum, episodeID, limit int, titleSearch bool) (*SearchResults, error) {
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
		searchResults = searcher.NewSearcher(mediaTypeConfig, mediaTypeConfig.CfgQuality, logger.StrRss, nil)
		err = searchResults.SearchRSS(ctx, mediaTypeConfig, mediaTypeConfig.CfgQuality, false, false)

	case "movies_search":
		logger.LogDynamicany1Any("info", "movie search", "id", movieID)
		if movieID <= 0 {
			return &SearchResults{Accepted: []SearchResult{}, Denied: []SearchResult{}}, nil
		}

		// Get movie from database
		movie, _ := database.GetMovies(
			database.Querywithargs{Select: "id, listname", Where: "id = ?"},
			movieID,
		)
		if movie.ID == 0 {
			logger.LogDynamicany1Any("error", "movie db search", "id", movieID)
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
			logger.LogDynamicany1Any("info", "movie list search", "id", movieID)
			return nil, fmt.Errorf("list configuration for movie not found")
		}

		searchResults = searcher.NewSearcher(mediaTypeConfig, listConfig.CfgQuality, "search", &movie.ID)
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
		searchResults, err = searcher.SearchSerieRSSSeasonSingle(&serie.ID, seasonStr, useseason, mediaTypeConfig)

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

		searchResults = searcher.NewSearcher(mediaTypeConfig, listConfig.CfgQuality, "search", &serie.ID)
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

		searchResults = searcher.NewSearcher(mediaTypeConfig, listConfig.CfgQuality, "search", &episode.ID)
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

// SearchResult represents a search result item
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

// renderSearchResults renders search results with separate accepted and denied datatables
func renderSearchResults(results *SearchResults, searchType, mediaConfig string) string {
	if results == nil || (len(results.Accepted) == 0 && len(results.Denied) == 0) {
		return renderComponentToString(
			Div(
				Class("alert-warning"),
				H5(Text("No Results Found")),
				P(Text("No search results were found matching your criteria. Try adjusting your search parameters or check your indexer configurations.")),
			),
		)
	}

	totalResults := len(results.Accepted) + len(results.Denied)

	result := Div(
		Class("w-100"),

		// Summary Header
		Div(
			Class("alert-success mb-4"),
			H5(Text(fmt.Sprintf("Search Results (%d total: %d accepted, %d denied)", totalResults, len(results.Accepted), len(results.Denied)))),
			P(Text(fmt.Sprintf("Search Type: %s | Media Config: %s", searchType, mediaConfig))),
		),

		// Accepted Results Section - Full Width
		Div(
			Class("mb-5"),
			H4(Text("✅ Accepted Results"), Class("text-success mb-3")),
			func() Node {
				if len(results.Accepted) == 0 {
					return Div(
						Class("alert-info"),
						Text("No results met the acceptance criteria."),
					)
				}
				return renderResultsTable(results.Accepted, "accepted", true)
			}(),
		),

		// Denied Results Section - Full Width
		Div(
			Class("mb-5"),
			H4(Text("❌ Denied Results"), Class("text-danger mb-3")),
			func() Node {
				if len(results.Denied) == 0 {
					return Div(
						Class("alert-info"),
						Text("No results were denied."),
					)
				}
				return renderResultsTable(results.Denied, "denied", true)
			}(),
		),

		// Information Footer
		Div(
			Class("alert-info"),
			H6(Text("Search Results Information")),
			P(Text("Real-time search results from your configured indexers:")),
			Ul(
				Li(Text("✅ Accepted: Results that passed quality and criteria filters")),
				Li(Text("❌ Denied: Results that were filtered out with reasons (quality, size, etc.)")),
				Li(Text("🔍 Live Search: Queries actual indexers using your media configuration")),
				Li(Text("📊 Quality Analysis: Shows detailed quality matching and rejection reasons")),
				Li(Text("💾 Download Available: Both accepted and denied results can be downloaded")),
				Li(Text("⚠️ Manual Override: Downloading denied results bypasses quality filters")),
			),
		),
	)

	return renderComponentToString(result)
}

// renderResultsTable creates a datatable for either accepted or denied results
func renderResultsTable(results []SearchResult, tableType string, showDownload bool) Node {
	// Create table headers
	headers := []Node{
		Th(Class("sorting"), Text("Title")),
		Th(Class("sorting"), Text("Size")),
		Th(Class("sorting"), Text("Quality")),
		Th(Class("sorting"), Text("Indexer")),
		Th(Class("sorting"), Text("Category")),
		Th(Class("sorting"), Text("Date")),
		Th(Class("sorting"), Text("Reason")),
	}

	if showDownload {
		headers = append(headers, Th(Attr("data-orderable", "false"), Text("Actions")))
	}

	// Create table rows
	var rows []Node
	for i, result := range results {
		rowCells := []Node{
			Td(
				Class("font-monospace small"),
				Text(result.Title),
			),
			Td(Text(result.Size)),
			Td(
				Span(
					Class(func() string {
						if showDownload {
							return "badge bg-success"
						}
						return "badge bg-secondary"
					}()),
					Text(result.Quality),
				),
			),
			Td(Text(result.Indexer)),
			Td(
				Span(
					Class("badge bg-secondary"),
					Text(result.Category),
				),
			),
			Td(Text(result.Date)),
			Td(
				Small(
					Class(func() string {
						if showDownload {
							return "text-success"
						}
						return "text-danger"
					}()),
					Text(result.Reason),
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
				Td(
					Div(
						Class("btn-group-sm"),
						Button(
							Class(downloadBtnClass),
							Text(downloadBtnText),
							Type("button"),
							hx.Post("/api/admin/searchdownload"),
							hx.Target(fmt.Sprintf("#download-result-%s-%d", tableType, i)),
							hx.Swap("innerHTML"),
							hx.Vals(fmt.Sprintf(`{"action": "download", "link": "%s", "title": "%s"}`, result.Link, result.Title)),
						),
						A(
							Class("btn btn-info btn-sm ml-1"),
							Href(result.Link),
							Target("_blank"),
							Text("Link"),
						),
					),
					Div(
						ID(fmt.Sprintf("download-result-%s-%d", tableType, i)),
						Style("min-height: 20px; margin-top: 5px;"),
					),
				),
			)
		}

		rows = append(rows, Tr(Group(rowCells)))
	}

	tableID := fmt.Sprintf("%s-results-table", tableType)

	return Div(
		Class("table-responsive w-100"),
		Table(
			Class("table table-striped w-100"),
			ID(tableID),
			THead(
				Tr(Group(headers)),
			),
			TBody(Group(rows)),
		),
		Script(Rawf(`
			$(document).ready(function() {
				if ($.fn.DataTable.isDataTable('#%s')) {
					$('#%s').DataTable().destroy();
				}
				$('#%s').DataTable({
					"bDestroy": true,
					"bFilter": true,
					"bSort": true,
					"bPaginate": true,
					"pageLength": 25,
					"lengthMenu": [[10, 25, 50, 100, -1], [10, 25, 50, 100, "All"]],
					responsive: true,
					"aaSorting": [[ 0, "asc" ]],
					"columnDefs": [
						{
							"targets": "no-sort",
							"orderable": false,
							"searchable": false
						}
					],
					"language": {
						"search": "Filter %s results:",
						"lengthMenu": "Show _MENU_ %s results per page",
						"info": "Showing _START_ to _END_ of _TOTAL_ %s results",
						"infoEmpty": "No %s results to show",
						"infoFiltered": "(filtered from _MAX_ total results)",
						"zeroRecords": "No matching %s results found"
					}
				});
			});
		`, tableID, tableID, tableID, tableType, tableType, tableType, tableType, tableType)),
	)
}


package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	gin "github.com/gin-gonic/gin"
	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// ================================================================================
// FEED PARSING PAGE
// ================================================================================

// renderFeedParsingPage renders a page for parsing and displaying feed results
// createJSONString creates a JSON string from a map for use in JavaScript
func createJSONString(data map[string][]string) string {
	if jsonBytes, err := json.Marshal(data); err == nil {
		jsonStr := string(jsonBytes)
		logger.Logtype("debug", 1).Str("json", jsonStr).Msg("JSON created for mediaToLists")
		return jsonStr
	}
	logger.Logtype("error", 0).Msg("Failed to marshal mediaToLists to JSON")
	return "{}"
}

func renderFeedParsingPage(csrfToken string) Node {
	// Get available media configurations
	media := config.GetSettingsMediaAll()
	var mediaConfigs []string
	for i := range media.Movies {
		mediaConfigs = append(mediaConfigs, media.Movies[i].NamePrefix)
	}
	for i := range media.Series {
		mediaConfigs = append(mediaConfigs, media.Series[i].NamePrefix)
	}

	// Create a mapping of media configs to their lists
	mediaToLists := make(map[string][]string)
	for i := range media.Movies {
		config := &media.Movies[i]
		var lists []string
		for j := range config.Lists {
			lists = append(lists, config.Lists[j].Name)
		}
		if len(lists) > 0 {
			mediaToLists[config.NamePrefix] = lists
		}
	}
	for i := range media.Series {
		config := &media.Series[i]
		var lists []string
		for j := range config.Lists {
			lists = append(lists, config.Lists[j].Name)
		}
		if len(lists) > 0 {
			mediaToLists[config.NamePrefix] = lists
		}
	}

	// Debug: log the mapping
	logger.Logtype("debug", 1).Any("mediaToLists", mediaToLists).Msg("Media to lists mapping created")

	// If no media configurations have lists, add some test data for debugging
	if len(mediaToLists) == 0 {
		logger.Logtype("debug", 0).Msg("No media configurations with lists found, creating test data")
		mediaToLists["test_movies"] = []string{"test_list_1", "test_list_2"}
		mediaToLists["test_series"] = []string{"test_series_list_1", "test_series_list_2"}
		if len(mediaConfigs) == 0 {
			mediaConfigs = append(mediaConfigs, "test_movies", "test_series")
		}
	}

	// Create better feed type descriptions
	feedTypeOptions := map[string]string{
		"imdbcsv":               "IMDB CSV - Import from IMDB CSV list",
		"imdbfile":              "IMDB File - Import from local IMDB file",
		"traktpublicmovielist":  "Trakt Public Movie List - User's public movie list",
		"traktmoviepopular":     "Trakt Popular Movies - Currently popular movies",
		"traktmovieanticipated": "Trakt Anticipated Movies - Most anticipated movies",
		"traktmovietrending":    "Trakt Trending Movies - Currently trending movies",
		"tmdbmoviediscover":     "TMDB Movie Discovery - Discover movies by criteria",
		"tmdblist":              "TMDB List - Import from TMDB list",
		"seriesconfig":          "Series Config File - Import from series configuration",
		"traktpublicshowlist":   "Trakt Public Series List - User's public series list",
		"traktseriepopular":     "Trakt Popular Series - Currently popular series",
		"traktserieanticipated": "Trakt Anticipated Series - Most anticipated series",
		"traktserietrending":    "Trakt Trending Series - Currently trending series",
		"tmdbshowdiscover":      "TMDB Series Discovery - Discover series by criteria",
		"tmdbshowlist":          "TMDB Series List - Import from TMDB series list",
		"newznabrss":            "Newznab RSS - Import from RSS feed",
	}

	var feedTypes []string
	for feedType := range feedTypeOptions {
		feedTypes = append(feedTypes, feedType)
	}

	return Div(
		Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		Div(
			Class("page-header-enhanced"),
			Div(
				Class("header-content"),
				Div(
					Class("header-icon-wrapper"),
					I(Class("fa-solid fa-rss header-icon")),
				),
				Div(
					Class("header-text"),
					H2(Class("header-title"), Text("Feed Parser & Results")),
					P(Class("header-subtitle"), Text("Parse and display feed results from various sources (IMDB lists, Trakt lists, CSV files, etc.). View the parsed movies and series that would be added to your configuration.")),
				),
			),
		),

		Form(
			Class("config-form"),
			ID("feedParsingForm"),

			Div(
				Class("form-cards-grid"),

				// Feed Configuration Card
				Div(
					Class("form-card"),
					Div(
						Class("card-header"),
						I(Class("fas fa-cog card-icon")),
						H5(Class("card-title"), Text("Feed Configuration")),
						P(Class("card-subtitle"), Text("Select source and media settings")),
					),
					Div(
						Class("card-body"),
						renderFormGroup("feed", map[string]string{
							"MediaConfig": "Select the media configuration to use for feed parsing",
						}, map[string]string{
							"MediaConfig": "Media Configuration",
						}, "MediaConfig", "select", "", map[string][]string{
							"options": mediaConfigs,
						}),

						renderFormGroup("feed", map[string]string{
							"FeedType": "Type of feed to parse",
						}, map[string]string{
							"FeedType": "Feed Type",
						}, "FeedType", "select", "imdb", map[string][]string{
							"options": feedTypes,
						}),
					),
				),

				// Parsing Options Card
				Div(
					Class("form-card"),
					Div(
						Class("card-header"),
						I(Class("fas fa-sliders-h card-icon")),
						H5(Class("card-title"), Text("Parsing Options")),
						P(Class("card-subtitle"), Text("Configure parsing behavior")),
					),
					Div(
						Class("card-body"),
						renderFormGroup("feed", map[string]string{
							"ListName": "Target list name for parsed items (select media configuration first)",
						}, map[string]string{
							"ListName": "List Name",
						}, "ListName", "select", "", map[string][]string{
							"options": {"-- Select Media Configuration First --"},
						}),

						renderFormGroup("feed", map[string]string{
							"DryRun": "Parse feeds without adding to database (preview only)",
						}, map[string]string{
							"DryRun": "Dry Run (Preview Only)",
						}, "DryRun", "checkbox", true, nil),

						renderFormGroup("feed", map[string]string{
							"Limit": "Maximum number of items to parse (0 = no limit)",
						}, map[string]string{
							"Limit": "Parse Limit",
						}, "Limit", "number", "100", nil),
					),
				),
			),

			// Enhanced action buttons
			Div(
				Class("form-actions-enhanced"),
				Button(
					Class("btn-action-primary"),
					Type("button"),
					hx.Target("#feedResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/feedparse"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Include("#feedParsingForm"),
					I(Class("fas fa-play action-icon")),
					Span(Class("action-text"), Text("Parse Feed")),
				),
				Button(
					Type("button"),
					Class("btn-action-secondary"),
					Attr("onclick", "document.getElementById('feedParsingForm').reset(); document.getElementById('feedResults').innerHTML = '';"),
					I(Class("fas fa-undo action-icon")),
					Span(Class("action-text"), Text("Reset Form")),
				),
			),
		),

		// Enhanced results container
		Div(
			Class("results-container-enhanced"),
			ID("feedResults"),
			// Loading state will be injected here
		),

		// Enhanced HTMX approach - add dynamic behavior to the MediaConfig select
		Script(Raw(`
			document.addEventListener('DOMContentLoaded', function() {
				// Initialize Choices.js first
				if (window.initChoicesGlobal) {
					window.initChoicesGlobal();
				}
				
				// Wait for Choices.js to initialize before setting up HTMX
				setTimeout(function() {
					setupFeedListsHTMX();
				}, 1500);
			});
			
			function setupFeedListsHTMX() {
				const mediaSelect = document.querySelector('select[name="feed_MediaConfig"]');
				const listSelect = document.querySelector('select[name="feed_ListName"]');
				
				if (mediaSelect && listSelect) {
					
					// Function to update list options
					function updateListOptions(mediaConfig) {
						if (!mediaConfig || mediaConfig === '') {
							// Clear list options
							updateListChoices([]);
							return;
						}
						
						// Make AJAX request to get lists
						fetch('/api/admin/feed-lists', {
							method: 'POST',
							headers: {
								'Content-Type': 'application/x-www-form-urlencoded',
								'X-CSRF-Token': '`+csrfToken+`'
							},
							body: 'feed_MediaConfig=' + encodeURIComponent(mediaConfig)
						})
						.then(response => {
							return response.text();
						})
						.then(html => {
							// Parse the HTML to get options
							var tempDiv = document.createElement('div');
							tempDiv.innerHTML = html;
							var options = tempDiv.querySelectorAll('option');
							
							var choicesOptions = [];
							options.forEach(function(option) {
								choicesOptions.push({
									value: option.value,
									label: option.textContent,
									selected: option.selected
								});
							});
							
							updateListChoices(choicesOptions);
						})
						.catch(error => {
							console.error('Error loading feed lists:', error);
							updateListChoices([{
								value: '',
								label: 'Error loading lists',
								disabled: true
							}]);
						});
					}
					
					// Function to update Choices.js with new options
					function updateListChoices(options) {
						// Find the Choices.js instance for the list select
						var listChoicesInstance = null;
						if (window.Choices && listSelect.choicesInstance) {
							listChoicesInstance = listSelect.choicesInstance;
						}						
						if (listChoicesInstance) {
							listChoicesInstance.clearStore();
							listChoicesInstance.setChoices(options, 'value', 'label', true);
						} else {
							// Fallback: update the select element directly
							listSelect.innerHTML = '';
							options.forEach(function(option) {
								var optionElement = document.createElement('option');
								optionElement.value = option.value;
								optionElement.textContent = option.label;
								if (option.selected) optionElement.selected = true;
								if (option.disabled) optionElement.disabled = true;
								listSelect.appendChild(optionElement);
							});
						}
					}
					
					// Listen for changes to media select
					mediaSelect.addEventListener('change', function() {
						updateListOptions(this.value);
					});
					
					// Trigger initial load if there's a selected value
					if (mediaSelect.value && mediaSelect.value !== '') {
						setTimeout(function() {
							updateListOptions(mediaSelect.value);
						}, 100);
					}
				}
			}
		`)),

		// Enhanced help section with modern styling
		Div(
			Class("help-section-enhanced"),
			Div(
				Class("help-header"),
				I(Class("fas fa-info-circle help-icon")),
				H5(Class("help-title"), Text("Feed Types & Sources")),
			),
			Div(
				Class("help-content"),
				Div(
					Class("help-grid"),
					Div(
						Class("help-card"),
						Div(Class("help-card-icon"), I(Class("fas fa-film"))),
						Div(Class("help-card-content"),
							Strong(Text("IMDB Sources")),
							P(Text("Import from IMDB CSV lists or local files")),
						),
					),
					Div(
						Class("help-card"),
						Div(Class("help-card-icon"), I(Class("fas fa-fire"))),
						Div(Class("help-card-content"),
							Strong(Text("Trakt Sources")),
							P(Text("Popular, trending, anticipated, or user's public lists")),
						),
					),
					Div(
						Class("help-card"),
						Div(Class("help-card-icon"), I(Class("fas fa-database"))),
						Div(Class("help-card-content"),
							Strong(Text("TMDB Sources")),
							P(Text("Import from TMDB lists or discovery queries")),
						),
					),
					Div(
						Class("help-card"),
						Div(Class("help-card-icon"), I(Class("fas fa-rss"))),
						Div(Class("help-card-content"),
							Strong(Text("RSS Feeds")),
							P(Text("Import from Newznab RSS feeds")),
						),
					),
				),
				Div(
					Class("help-tips"),
					Div(
						Class("tip-item"),
						I(Class("fas fa-lightbulb tip-icon")),
						Strong(Text("Media Configuration: ")),
						Text("Select a media configuration first to see its available lists. Only lists from the selected configuration will be shown."),
					),
					Div(
						Class("tip-item"),
						I(Class("fas fa-eye tip-icon")),
						Strong(Text("Dry Run: ")),
						Text("When enabled, shows what would be parsed without adding to your database. Useful for testing feeds before import."),
					),
				),
			),
		),
	)
}

// HandleFeedLists returns list options for selected media config via HTMX
func HandleFeedLists(c *gin.Context) {
	selectedConfig := c.PostForm("feed_MediaConfig")
	if selectedConfig == "" {
		c.String(http.StatusOK, `<option value="">-- Select a media configuration first --</option>`)
		return
	}

	// Get media configurations
	media := config.GetSettingsMediaAll()
	mediaToLists := make(map[string][]string)
	for i := range media.Movies {
		config := &media.Movies[i]
		var lists []string
		for j := range config.Lists {
			lists = append(lists, config.Lists[j].Name)
		}
		if len(lists) > 0 {
			mediaToLists[config.NamePrefix] = lists
		}
	}
	for i := range media.Series {
		config := &media.Series[i]
		var lists []string
		for j := range config.Lists {
			lists = append(lists, config.Lists[j].Name)
		}
		if len(lists) > 0 {
			mediaToLists[config.NamePrefix] = lists
		}
	}

	lists, exists := mediaToLists[selectedConfig]
	if !exists || len(lists) == 0 {
		c.String(http.StatusOK, `<option value="">-- No lists available for this configuration --</option>`)
		return
	}

	// Build HTML options
	var result strings.Builder
	result.WriteString(`<option value="">-- Select a list --</option>`)
	for _, list := range lists {
		result.WriteString(fmt.Sprintf(`<option value="%s">%s</option>`, list, list))
	}

	c.String(http.StatusOK, result.String())
}

// FeedParsingRequest represents the parsed form data for feed parsing
type FeedParsingRequest struct {
	MediaConfig string
	FeedType    string
	ListName    string
	DryRun      bool
	Limit       int
}

// parseFeedParsingRequest extracts and validates feed parsing request data
func parseFeedParsingRequest(c *gin.Context) (*FeedParsingRequest, error) {
	if err := c.Request.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %v", err)
	}

	req := &FeedParsingRequest{
		MediaConfig: c.PostForm("feed_MediaConfig"),
		FeedType:    c.PostForm("feed_FeedType"),
		ListName:    c.PostForm("feed_ListName"),
		DryRun:      c.PostForm("feed_DryRun") == "on",
		Limit:       parseIntOrDefault(c.PostForm("feed_Limit"), DefaultLimit),
	}

	if req.MediaConfig == "" || req.FeedType == "" {
		return nil, fmt.Errorf("please fill in all required fields")
	}

	return req, nil
}

// findMediaTypeConfig finds a media type configuration by name prefix
func findMediaTypeConfig(namePrefix string) *config.MediaTypeConfig {
	var mediaTypeConfig *config.MediaTypeConfig
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if strings.EqualFold(media.NamePrefix, namePrefix) {
			mediaTypeConfig = media
			return nil
		}
		return nil
	})
	return mediaTypeConfig
}

// HandleFeedParsing handles feed parsing requests
func HandleFeedParsing(c *gin.Context) {
	req, err := parseFeedParsingRequest(c)
	if err != nil {
		c.String(http.StatusOK, renderAlert(err.Error(), "danger"))
		return
	}

	// Get the media configuration
	mediaTypeConfig := findMediaTypeConfig(req.MediaConfig)
	if mediaTypeConfig == nil {
		c.String(http.StatusOK, renderAlert("Media configuration not found", "danger"))
		return
	}

	// Get available lists for each media config
	var tempListConfig *config.MediaListsConfig
	for i := range mediaTypeConfig.Lists {
		if mediaTypeConfig.Lists[i].Name == req.ListName {
			tempListConfig = &mediaTypeConfig.Lists[i]
		}
	}

	// Create a temporary list configuration based on the feed type and source
	if tempListConfig == nil {
		c.String(http.StatusOK, renderAlert("Unsupported feed type: "+req.FeedType, "warning"))
		return
	}

	feedResults, err := utils.Feeds(mediaTypeConfig, tempListConfig, req.DryRun)
	if err != nil {
		utils.ReturnFeeds(feedResults)
		c.String(http.StatusOK, renderAlert("Feed parsing failed: "+err.Error(), "danger"))
		return
	}
	defer utils.ReturnFeeds(feedResults)

	// Prepare result with actual parsed data
	result := map[string]any{
		"media_config": req.MediaConfig,
		"feed_type":    req.FeedType,
		"list_name":    req.ListName,
		"dry_run":      req.DryRun,
		"limit":        req.Limit,
		"movies_found": len(feedResults.Movies),
		"series_found": len(feedResults.Series),
		"movies":       feedResults.Movies,
		"series":       feedResults.Series,
		"success":      true,
	}

	c.String(http.StatusOK, renderFeedParsingResults(result))
}

// renderFeedParsingResults renders the feed parsing results
func renderFeedParsingResults(result map[string]any) string {
	feedType, _ := result["feed_type"].(string)
	mediaConfig, _ := result["media_config"].(string)
	listName, _ := result["list_name"].(string)
	dryRun, _ := result["dry_run"].(bool)
	limit, _ := result["limit"].(int)

	// Create components for movies and series
	var components []Node
	var alertcomponents []Node

	// Add basic information table
	components = append(components,
		Table(
			Class("table table-sm"),
			TBody(
				Tr(Td(Text("Media Configuration:")), Td(Text(mediaConfig))),
				Tr(Td(Text("Feed Type:")), Td(Text(strings.ToTitle(feedType)))),
				Tr(Td(Text("Target List:")), Td(Text(listName))),
				Tr(Td(Text("Dry Run:")), Td(Text(fmt.Sprintf("%t", dryRun)))),
				Tr(Td(Text("Parse Limit:")), Td(Text(func() string {
					if limit > 0 {
						return fmt.Sprintf("%d items", limit)
					}
					return "No limit"
				}()))),
			),
		),
	)

	var count int
	var feedComponents []Node
	if strings.HasPrefix(mediaConfig, "movie_") {
		movies, _ := result["movies"].([]string)
		count = len(movies)

		// Display movies if found
		if count > 0 {
			feedComponents = append(feedComponents,

				H6(Text(fmt.Sprintf("Movies (%d)", count))),
			)

			// Show first 20 movies, with option to show more
			maxDisplay := count
			if maxDisplay > 20 {
				maxDisplay = 20
			}

			var movieItems []Node
			for i := 0; i < maxDisplay; i++ {
				movieItems = append(movieItems, Li(
					Code(Text(movies[i])),
				))
			}

			if count > 20 {
				movieItems = append(movieItems, Li(
					Em(Text(fmt.Sprintf("... and %d more movies", count-20))),
				))
			}

			feedComponents = append(feedComponents, Ul(movieItems...))
		}
	} else {
		series, _ := result["series"].([]config.SerieConfig)
		count = len(series)
		// Display series if found
		if count > 0 {
			feedComponents = append(feedComponents,

				H6(Text(fmt.Sprintf("Series (%d)", count))),
			)

			// Show first 20 series, with option to show more
			maxDisplay := count
			if maxDisplay > 20 {
				maxDisplay = 20
			}

			var seriesItems []Node
			for i := 0; i < maxDisplay; i++ {
				serie := series[i]
				seriesItems = append(seriesItems, Li(
					Text(fmt.Sprintf("%s (TVDB ID: %d)", serie.Name, serie.TvdbID)),
				))
			}

			if count > 20 {
				seriesItems = append(seriesItems, Li(
					Em(Text(fmt.Sprintf("... and %d more series", count-20))),
				))
			}

			feedComponents = append(feedComponents, Ul(seriesItems...))
		}
	}
	AllNodes := append([]Node{Class("mt-3")}, feedComponents...)
	components = append(components, Div(AllNodes...))

	if count == 0 {
		return renderComponentToString(
			Div(
				Class("card border-0 shadow-sm border-danger mb-4"),
				Div(
					Class("card-header border-0"),
					Style("background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%); border-radius: 15px 15px 0 0;"),
					Div(
						Class("d-flex align-items-center"),
						Span(Class("badge bg-danger me-3"), I(Class("fas fa-exclamation-triangle me-1")), Text("Error")),
						H5(Class("card-title mb-0 text-danger fw-bold"), Text("Feed Parsing Error")),
					),
				),
				Div(
					Class("card-body"),
					P(Class("card-text text-muted mb-0"), Text("No results were returned from the feed parsing operation.")),
				),
			),
		)
	}

	alertcomponents = append(alertcomponents,

		Div(
			Class("mt-3 card border-0 shadow-sm"),
			Div(
				Class("card-body"),
				H6(Class("card-title fw-bold mb-3"), Text("Parsing Results")),
				Div(Class("d-flex align-items-center mb-2"),
					Span(Class("badge bg-info me-2"), I(Class("fas fa-rss me-1")), Text("Feed Entries")),
					Span(Class("fw-bold text-info"), Text(fmt.Sprintf("%d", count))),
					Span(Class("text-muted ms-2"), Text("entries found")),
				),
				P(Class("card-text text-muted small mb-0"), Text("Successfully parsed feed source and extracted available entries")),
			),
		),
	)

	if count == 0 {
		alertcomponents = append(alertcomponents,
			Div(
				Class("mt-3 card border-0 shadow-sm border-warning mb-4"),
				Div(
					Class("card-header border-0"),
					Style("background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;"),
					Div(
						Class("d-flex align-items-center"),
						Span(Class("badge bg-warning me-3"), I(Class("fas fa-exclamation-circle me-1")), Text("Warning")),
						H5(Class("card-title mb-0 text-warning fw-bold"), Text("No Results")),
					),
				),
				Div(
					Class("card-body"),
					P(Class("card-text text-muted mb-3"), Text("No movies or series were found from the specified feed source. This could be due to:")),
					Ul(
						Class("mb-0 list-unstyled"),
						Li(Class("mb-2"), Text("• Empty or invalid feed source")),
						Li(Class("mb-2"), Text("• Network connectivity issues")),
						Li(Class("mb-2"), Text("• Invalid credentials for private lists")),
						Li(Class("mb-2"), Text("• Feed source temporarily unavailable")),
					),
				),
			),
		)
	}

	resultnodes := Div(append([]Node{
		Class("card border-0 shadow-sm border-success"),
		Div(
			Class("card-body"),
			H5(Class("card-title fw-bold mb-3"), Text("Feed Parsing Complete")),
			Div(Class("d-flex align-items-center mb-3"),
				Span(Class("badge bg-success me-2"), I(Class("fas fa-check me-1")), Text("Success")),
				Span(Class("text-success"), Text("Feed has been parsed successfully")),
			),
		),
	}, components...)...)

	feedAllNodes := append([]Node{resultnodes}, alertcomponents...)
	return renderComponentToString(
		Div(feedAllNodes...),
	)
}

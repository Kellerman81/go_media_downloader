package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// ================================================================================
// FEED PARSING PAGE
// ================================================================================

func renderFeedParsingPage(csrfToken string) gomponents.Node {
	// Get available media configurations
	media := config.GetSettingsMediaAll()

	var mediaConfigs []SelectOption
	for i := range media.Movies {
		mediaConfigs = append(mediaConfigs, SelectOption{Label: media.Movies[i].NamePrefix, Value: media.Movies[i].NamePrefix})
	}

	for i := range media.Series {
		mediaConfigs = append(mediaConfigs, SelectOption{Label: media.Series[i].NamePrefix, Value: media.Series[i].NamePrefix})
	}

	// Create a mapping of media configs to their lists
	var mediaToLists []SelectOption
	for i := range media.Movies {
		config := &media.Movies[i]

		var lists []string
		for j := range config.Lists {
			lists = append(lists, config.Lists[j].Name)
		}

		if len(lists) > 0 {
			mediaToLists = append(mediaToLists, SelectOption{Label: config.NamePrefix, Value: lists})
		}
	}

	for i := range media.Series {
		config := &media.Series[i]

		var lists []string
		for j := range config.Lists {
			lists = append(lists, config.Lists[j].Name)
		}

		if len(lists) > 0 {
			mediaToLists = append(mediaToLists, SelectOption{Label: config.NamePrefix, Value: lists})
		}
	}

	// Debug: log the mapping
	logger.Logtype("debug", 1).
		Any("mediaToLists", mediaToLists).
		Msg("Media to lists mapping created")

	// If no media configurations have lists, add some test data for debugging
	if len(mediaToLists) == 0 {
		logger.Logtype("debug", 0).
			Msg("No media configurations with lists found, creating test data")

		mediaToLists = append(mediaToLists, SelectOption{Label: "test_movies", Value: []string{"test_list_1", "test_list_2"}})
		mediaToLists = append(mediaToLists, SelectOption{Label: "test_series", Value: []string{"test_series_list_1", "test_series_list_2"}})
	}

	// Create better feed type descriptions
	feedTypes := []SelectOption{
		{Value: "imdbcsv", Label: "IMDB CSV - Import from IMDB CSV list"},
		{Value: "imdbfile", Label: "IMDB File - Import from local IMDB file"},
		{Value: "traktpublicmovielist", Label: "Trakt Public Movie List - User's public movie list"},
		{Value: "traktmoviepopular", Label: "Trakt Popular Movies - Currently popular movies"},
		{Value: "traktmovieanticipated", Label: "Trakt Anticipated Movies - Most anticipated movies"},
		{Value: "traktmovietrending", Label: "Trakt Trending Movies - Currently trending movies"},
		{Value: "tmdbmoviediscover", Label: "TMDB Movie Discovery - Discover movies by criteria"},
		{Value: "tmdblist", Label: "TMDB List - Import from TMDB list"},
		{Value: "seriesconfig", Label: "Series Config File - Import from series configuration"},
		{Value: "traktpublicshowlist", Label: "Trakt Public Series List - User's public series list"},
		{Value: "traktseriepopular", Label: "Trakt Popular Series - Currently popular series"},
		{Value: "traktserieanticipated", Label: "Trakt Anticipated Series - Most anticipated series"},
		{Value: "traktserietrending", Label: "Trakt Trending Series - Currently trending series"},
		{Value: "tmdbshowdiscover", Label: "TMDB Series Discovery - Discover series by criteria"},
		{Value: "tmdbshowlist", Label: "TMDB Series List - Import from TMDB series list"},
		{Value: "newznabrss", Label: "Newznab RSS - Import from RSS feed"},
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
					html.I(html.Class("fa-solid fa-rss header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Feed Parser & Results")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Parse and display feed results from various sources (IMDB lists, Trakt lists, CSV files, etc.). View the parsed movies and series that would be added to your configuration.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("feedParsingForm"),

			html.Div(
				html.Class("form-cards-grid"),

				// Feed Configuration Card
				html.Div(
					html.Class("form-card"),
					html.Div(
						html.Class("card-header"),
						html.I(html.Class("fas fa-cog card-icon")),
						html.H5(html.Class("card-title"), gomponents.Text("Feed Configuration")),
						html.P(
							html.Class("card-subtitle"),
							gomponents.Text("Select source and media settings"),
						),
					),
					html.Div(
						html.Class("card-body"),
						renderFormGroup("feed", map[string]string{
							"MediaConfig": "Select the media configuration to use for feed parsing",
						}, map[string]string{
							"MediaConfig": "Media Configuration",
						}, "MediaConfig", "select", "", convertSelectOptionsToMap(mediaConfigs),
						),

						renderFormGroup("feed", map[string]string{
							"FeedType": "Type of feed to parse",
						}, map[string]string{
							"FeedType": "Feed Type",
						}, "FeedType", "select", "imdb", convertSelectOptionsToMap(feedTypes),
						),
					),
				),

				// Parsing Options Card
				html.Div(
					html.Class("form-card"),
					html.Div(
						html.Class("card-header"),
						html.I(html.Class("fas fa-sliders-h card-icon")),
						html.H5(html.Class("card-title"), gomponents.Text("Parsing Options")),
						html.P(
							html.Class("card-subtitle"),
							gomponents.Text("Configure parsing behavior"),
						),
					),
					html.Div(
						html.Class("card-body"),
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
			html.Div(
				html.Class("form-actions-enhanced"),
				html.Button(
					html.Class("btn-action-primary"),
					html.Type("button"),
					hx.Target("#feedResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/feedparse"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Include("#feedParsingForm"),
					html.I(html.Class("fas fa-play action-icon")),
					html.Span(html.Class("action-text"), gomponents.Text("Parse Feed")),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn-action-secondary"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('feedParsingForm').reset(); document.getElementById('feedResults').innerHTML = '';",
					),
					html.I(html.Class("fas fa-undo action-icon")),
					html.Span(html.Class("action-text"), gomponents.Text("Reset Form")),
				),
			),
		),

		// Enhanced results container
		html.Div(
			html.Class("results-container-enhanced"),
			html.ID("feedResults"),
			// Loading state will be injected here
		),

		// Enhanced HTMX approach - add dynamic behavior to the MediaConfig select
		html.Script(gomponents.Raw(`
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
		html.Div(
			html.Class("help-section-enhanced"),
			html.Div(
				html.Class("help-header"),
				html.I(html.Class("fas fa-info-circle help-icon")),
				html.H5(html.Class("help-title"), gomponents.Text("Feed Types & Sources")),
			),
			html.Div(
				html.Class("help-content"),
				html.Div(
					html.Class("help-grid"),
					html.Div(
						html.Class("help-card"),
						html.Div(html.Class("help-card-icon"), html.I(html.Class("fas fa-film"))),
						html.Div(html.Class("help-card-content"),
							html.Strong(gomponents.Text("IMDB Sources")),
							html.P(gomponents.Text("Import from IMDB CSV lists or local files")),
						),
					),
					html.Div(
						html.Class("help-card"),
						html.Div(html.Class("help-card-icon"), html.I(html.Class("fas fa-fire"))),
						html.Div(
							html.Class("help-card-content"),
							html.Strong(gomponents.Text("Trakt Sources")),
							html.P(
								gomponents.Text(
									"Popular, trending, anticipated, or user's public lists",
								),
							),
						),
					),
					html.Div(
						html.Class("help-card"),
						html.Div(
							html.Class("help-card-icon"),
							html.I(html.Class("fas fa-database")),
						),
						html.Div(html.Class("help-card-content"),
							html.Strong(gomponents.Text("TMDB Sources")),
							html.P(gomponents.Text("Import from TMDB lists or discovery queries")),
						),
					),
					html.Div(
						html.Class("help-card"),
						html.Div(html.Class("help-card-icon"), html.I(html.Class("fas fa-rss"))),
						html.Div(html.Class("help-card-content"),
							html.Strong(gomponents.Text("RSS Feeds")),
							html.P(gomponents.Text("Import from Newznab RSS feeds")),
						),
					),
				),
				html.Div(
					html.Class("help-tips"),
					html.Div(
						html.Class("tip-item"),
						html.I(html.Class("fas fa-lightbulb tip-icon")),
						html.Strong(gomponents.Text("Media Configuration: ")),
						gomponents.Text(
							"Select a media configuration first to see its available lists. Only lists from the selected configuration will be shown.",
						),
					),
					html.Div(
						html.Class("tip-item"),
						html.I(html.Class("fas fa-eye tip-icon")),
						html.Strong(gomponents.Text("Dry Run: ")),
						gomponents.Text(
							"When enabled, shows what would be parsed without adding to your database. Useful for testing feeds before import.",
						),
					),
				),
			),
		),
	)
}

// HandleFeedLists returns list options for selected media config via HTMX.
func HandleFeedLists(c *gin.Context) {
	selectedConfig := c.PostForm("feed_MediaConfig")
	if selectedConfig == "" {
		c.String(
			http.StatusOK,
			`<option value="">-- Select a media configuration first --</option>`,
		)

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
		c.String(
			http.StatusOK,
			`<option value="">-- No lists available for this configuration --</option>`,
		)

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

// FeedParsingRequest represents the parsed form data for feed parsing.
type FeedParsingRequest struct {
	MediaConfig string
	FeedType    string
	ListName    string
	DryRun      bool
	Limit       int
}

// parseFeedParsingRequest extracts and validates feed parsing request data.
func parseFeedParsingRequest(c *gin.Context) (*FeedParsingRequest, error) {
	if err := c.Request.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form data: %w", err)
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

// findMediaTypeConfig finds a media type configuration by name prefix.
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

// HandleFeedParsing handles feed parsing requests.
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

// renderFeedParsingResults renders the feed parsing results.
func renderFeedParsingResults(result map[string]any) string {
	feedType, _ := result["feed_type"].(string)
	mediaConfig, _ := result["media_config"].(string)
	listName, _ := result["list_name"].(string)
	dryRun, _ := result["dry_run"].(bool)
	limit, _ := result["limit"].(int)

	// Create components for movies and series
	var (
		components      []gomponents.Node
		alertcomponents []gomponents.Node
	)

	// Add basic information table
	components = append(components,
		html.Table(
			html.Class("table table-sm"),
			html.TBody(
				html.Tr(
					html.Td(gomponents.Text("Media Configuration:")),
					html.Td(gomponents.Text(mediaConfig)),
				),
				html.Tr(
					html.Td(gomponents.Text("Feed Type:")),
					html.Td(gomponents.Text(strings.ToTitle(feedType))),
				),
				html.Tr(
					html.Td(gomponents.Text("Target List:")),
					html.Td(gomponents.Text(listName)),
				),
				html.Tr(
					html.Td(gomponents.Text("Dry Run:")),
					html.Td(gomponents.Text(fmt.Sprintf("%t", dryRun))),
				),
				html.Tr(
					html.Td(gomponents.Text("Parse Limit:")),
					html.Td(gomponents.Text(func() string {
						if limit > 0 {
							return fmt.Sprintf("%d items", limit)
						}
						return "No limit"
					}())),
				),
			),
		),
	)

	var (
		count          int
		feedComponents []gomponents.Node
	)

	if strings.HasPrefix(mediaConfig, "movie_") {
		movies, _ := result["movies"].([]string)

		count = len(movies)

		// Display movies if found
		if count > 0 {
			feedComponents = append(feedComponents,

				html.H6(gomponents.Text(fmt.Sprintf("Movies (%d)", count))),
			)

			// Show first 20 movies, with option to show more
			maxDisplay := count
			if maxDisplay > 20 {
				maxDisplay = 20
			}

			var movieItems []gomponents.Node
			for i := range maxDisplay {
				movieItems = append(movieItems, html.Li(
					html.Code(gomponents.Text(movies[i])),
				))
			}

			if count > 20 {
				movieItems = append(movieItems, html.Li(
					html.Em(gomponents.Text(fmt.Sprintf("... and %d more movies", count-20))),
				))
			}

			feedComponents = append(feedComponents, html.Ul(movieItems...))
		}
	} else {
		series, _ := result["series"].([]config.SerieConfig)

		count = len(series)
		// Display series if found
		if count > 0 {
			feedComponents = append(feedComponents,

				html.H6(gomponents.Text(fmt.Sprintf("Series (%d)", count))),
			)

			// Show first 20 series, with option to show more
			maxDisplay := count
			if maxDisplay > 20 {
				maxDisplay = 20
			}

			var seriesItems []gomponents.Node
			for i := range maxDisplay {
				serie := series[i]

				seriesItems = append(seriesItems, html.Li(
					gomponents.Text(fmt.Sprintf("%s (TVDB ID: %d)", serie.Name, serie.TvdbID)),
				))
			}

			if count > 20 {
				seriesItems = append(seriesItems, html.Li(
					html.Em(gomponents.Text(fmt.Sprintf("... and %d more series", count-20))),
				))
			}

			feedComponents = append(feedComponents, html.Ul(seriesItems...))
		}
	}

	AllNodes := append([]gomponents.Node{html.Class("mt-3")}, feedComponents...)

	components = append(components, html.Div(AllNodes...))

	if count == 0 {
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
							gomponents.Text("Feed Parsing Error"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-0"),
						gomponents.Text(
							"No results were returned from the feed parsing operation.",
						),
					),
				),
			),
		)
	}

	alertcomponents = append(alertcomponents,

		html.Div(
			html.Class("mt-3 card border-0 shadow-sm"),
			html.Div(
				html.Class("card-body"),
				html.H6(html.Class("card-title fw-bold mb-3"), gomponents.Text("Parsing Results")),
				html.Div(
					html.Class("d-flex align-items-center mb-2"),
					html.Span(
						html.Class("badge bg-info me-2"),
						html.I(html.Class("fas fa-rss me-1")),
						gomponents.Text("Feed Entries"),
					),
					html.Span(
						html.Class("fw-bold text-info"),
						gomponents.Text(fmt.Sprintf("%d", count)),
					),
					html.Span(html.Class("text-muted ms-2"), gomponents.Text("entries found")),
				),
				html.P(
					html.Class("card-text text-muted small mb-0"),
					gomponents.Text(
						"Successfully parsed feed source and extracted available entries",
					),
				),
			),
		),
	)

	if count == 0 {
		alertcomponents = append(alertcomponents,
			html.Div(
				html.Class("mt-3 card border-0 shadow-sm border-warning mb-4"),
				html.Div(
					html.Class("card-header border-0"),
					html.Style(
						"background: linear-gradient(135deg, #fff3cd 0%, #ffeaa7 100%); border-radius: 15px 15px 0 0;",
					),
					html.Div(
						html.Class("d-flex align-items-center"),
						html.Span(
							html.Class("badge bg-warning me-3"),
							html.I(html.Class("fas fa-exclamation-circle me-1")),
							gomponents.Text("Warning"),
						),
						html.H5(
							html.Class("card-title mb-0 text-warning fw-bold"),
							gomponents.Text("No Results"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-3"),
						gomponents.Text(
							"No movies or series were found from the specified feed source. This could be due to:",
						),
					),
					html.Ul(
						html.Class("mb-0 list-unstyled"),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• Empty or invalid feed source"),
						),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• Network connectivity issues"),
						),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• Invalid credentials for private lists"),
						),
						html.Li(
							html.Class("mb-2"),
							gomponents.Text("• Feed source temporarily unavailable"),
						),
					),
				),
			),
		)
	}

	resultnodes := html.Div(append([]gomponents.Node{
		html.Class("card border-0 shadow-sm border-success"),
		html.Div(
			html.Class("card-body"),
			html.H5(
				html.Class("card-title fw-bold mb-3"),
				gomponents.Text("Feed Parsing Complete"),
			),
			html.Div(
				html.Class("d-flex align-items-center mb-3"),
				html.Span(
					html.Class("badge bg-success me-2"),
					html.I(html.Class("fas fa-check me-1")),
					gomponents.Text("Success"),
				),
				html.Span(
					html.Class("text-success"),
					gomponents.Text("Feed has been parsed successfully"),
				),
			),
		),
	}, components...)...)

	feedAllNodes := append([]gomponents.Node{resultnodes}, alertcomponents...)

	return renderComponentToString(
		html.Div(feedAllNodes...),
	)
}

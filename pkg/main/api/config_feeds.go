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
		logger.LogDynamicany1String("debug", "JSON created for mediaToLists", "json", jsonStr)
		return jsonStr
	}
	logger.LogDynamicany0("error", "Failed to marshal mediaToLists to JSON")
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
	logger.LogDynamicany1Any("debug", "Media to lists mapping created", "mediaToLists", mediaToLists)

	// If no media configurations have lists, add some test data for debugging
	if len(mediaToLists) == 0 {
		logger.LogDynamicany0("debug", "No media configurations with lists found, creating test data")
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
		Class("config-section"),
		H3(Text("Feed Parser & Results")),
		P(Text("Parse and display feed results from various sources (IMDB lists, Trakt lists, CSV files, etc.). View the parsed movies and series that would be added to your configuration.")),

		Form(
			Class("config-form"),
			ID("feedParsingForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Text("Feed Configuration")),

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

				Div(
					Class("col-md-6"),
					H5(Text("Parsing Options")),

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

			Div(
				Class("form-group submit-group"),
				Button(
					Class(ClassBtnPrimary),
					Text("Parse Feed"),
					Type("button"),
					hx.Target("#feedResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/feedparse"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Include("#feedParsingForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('feedParsingForm').reset(); document.getElementById('feedResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("feedResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// JavaScript for dynamic list updates
		Script(
			Rawf(`
				const mediaToLists = %s;
				
				function updateListOptions() {
					const mediaSelect = document.querySelector('select[name="feed_MediaConfig"]');
					const listSelect = document.querySelector('select[name="feed_ListName"]');
					
					console.log('updateListOptions called');
					console.log('mediaSelect found:', !!mediaSelect);
					console.log('listSelect found:', !!listSelect);
					
					if (!mediaSelect || !listSelect) {
						console.error('Could not find form elements');
						return;
					}
					
					const selectedMedia = mediaSelect.value;
					const lists = mediaToLists[selectedMedia] || [];
					
					console.log('Selected media:', selectedMedia);
					console.log('Available lists:', lists);
					
					// Clear existing options
					listSelect.innerHTML = '';
					
					if (lists.length === 0) {
						const option = document.createElement('option');
						option.value = '';
						option.textContent = '-- No lists available for this configuration --';
						listSelect.appendChild(option);
					} else {
						// Add default option
						const defaultOption = document.createElement('option');
						defaultOption.value = '';
						defaultOption.textContent = '-- Select a list --';
						listSelect.appendChild(defaultOption);
						
						// Add list options
						lists.forEach(function(list) {
							const option = document.createElement('option');
							option.value = list;
							option.textContent = list;
							listSelect.appendChild(option);
						});
					}
				}
				
				// Set up event listener
				document.addEventListener('DOMContentLoaded', function() {
					console.log('DOM loaded, setting up feed parser event listeners');
					console.log('mediaToLists:', mediaToLists);
					
					const mediaSelect = document.querySelector('select[name="feed_MediaConfig"]');
					console.log('Media select element found:', !!mediaSelect);
					
					if (mediaSelect) {
						mediaSelect.addEventListener('change', updateListOptions);
						console.log('Event listener added to media select');
						
						// Call updateListOptions initially to handle any pre-selected values
						updateListOptions();
						console.log('Initial updateListOptions called');
					} else {
						// Try alternative selectors
						const altMediaSelect = document.querySelector('#feedParsingForm select[name="feed_MediaConfig"]');
						console.log('Alternative media select found:', !!altMediaSelect);
						if (altMediaSelect) {
							altMediaSelect.addEventListener('change', updateListOptions);
							console.log('Event listener added to alternative media select');
							
							// Call updateListOptions initially for alternative selector too
							updateListOptions();
							console.log('Initial updateListOptions called for alternative selector');
						}
					}
				});
			`, createJSONString(mediaToLists)),
		),

		// Instructions
		Div(
			Class("mt-4 alert alert-info"),
			H5(Text("Feed Types & Sources:")),
			Ul(
				Li(Strong(Text("IMDB Sources: ")), Text("Import from IMDB CSV lists or local files")),
				Li(Strong(Text("Trakt Sources: ")), Text("Popular, trending, anticipated, or user's public lists")),
				Li(Strong(Text("TMDB Sources: ")), Text("Import from TMDB lists or discovery queries")),
				Li(Strong(Text("Series Config: ")), Text("Import from series configuration files")),
				Li(Strong(Text("RSS Feeds: ")), Text("Import from Newznab RSS feeds")),
			),
			P(
				Class("mt-2"),
				Strong(Text("Media Configuration: ")),
				Text("Select a media configuration first to see its available lists. Only lists from the selected configuration will be shown."),
			),
			P(
				Class("mt-2"),
				Strong(Text("Dry Run: ")),
				Text("When enabled, shows what would be parsed without adding to your database. Useful for testing feeds before import."),
			),
		),
	)
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
				Class("alert alert-danger"),
				H5(Text("Feed Parsing Error")),
				P(Text("No results were returned from the feed parsing operation.")),
			),
		)
	}

	components = append(components,
		Div(
			Class("mt-3 alert alert-info"),
			H6(Text("Parsing Results")),
			P(Text(fmt.Sprintf("Found %d entries from the feed source.", count))),
		),
	)

	if count == 0 {
		components = append(components,
			Div(
				Class("mt-3 alert alert-warning"),
				H6(Text("No Results")),
				P(Text("No movies or series were found from the specified feed source. This could be due to:")),
				Ul(
					Li(Text("Empty or invalid feed source")),
					Li(Text("Network connectivity issues")),
					Li(Text("Invalid credentials for private lists")),
					Li(Text("Feed source temporarily unavailable")),
				),
			),
		)
	}

	feedAllNodes := append([]Node{Class("alert alert-success"), H5(Text("Feed Parsing Complete"))}, components...)
	return renderComponentToString(
		Div(feedAllNodes...),
	)
}

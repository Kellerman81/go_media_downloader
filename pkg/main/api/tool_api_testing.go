package api

import (
	"fmt"
	"net/http"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// apiEndpointDoc describes one row in the API Testing Suite's "API Endpoints
// Reference" accordion.
type apiEndpointDoc struct {
	Method string
	Path   string
	Desc   string
}

// renderAPIEndpointsSection renders one collapsible accordion item listing
// endpoints. The header's count is derived from len(endpoints) instead of a
// hand-typed number, so it can never drift out of sync with the list again
// (the previous hardcoded counts were wrong for Movies/Series/General).
func renderAPIEndpointsSection(id, title string, endpoints []apiEndpointDoc) gomponents.Node {
	items := make([]gomponents.Node, 0, len(endpoints))
	for _, e := range endpoints {
		items = append(
			items,
			html.Li(gomponents.Text(e.Method+" "+e.Path+" - "+e.Desc)),
		)
	}

	return html.Div(
		html.Class("accordion-item"),
		html.H2(
			html.Class("accordion-header"),
			html.ID("heading"+id),
			html.Button(
				html.Class("accordion-button collapsed"),
				html.Type("button"),
				gomponents.Attr("data-bs-toggle", "collapse"),
				gomponents.Attr("data-bs-target", "#collapse"+id),
				gomponents.Attr("aria-expanded", "false"),
				gomponents.Attr("aria-controls", "collapse"+id),
				gomponents.Text(fmt.Sprintf("%s (%d)", title, len(endpoints))),
			),
		),
		html.Div(
			html.ID("collapse"+id),
			html.Class("accordion-collapse collapse"),
			gomponents.Attr("aria-labelledby", "heading"+id),
			gomponents.Attr("data-bs-parent", "#endpointsAccordion"),
			html.Div(
				html.Class("accordion-body"),
				html.Ul(html.Class("list-unstyled mb-0"), gomponents.Group(items)),
			),
		),
	)
}

var apiEndpointsAll = []apiEndpointDoc{
	{"GET", "/api/all/feeds", "Search all feeds"},
	{"GET", "/api/all/data", "Search all folders"},
	{"GET", "/api/all/search/rss", "Search all rss feeds"},
	{"GET", "/api/all/search/missing/full", "Search all Missing"},
	{"GET", "/api/all/search/missing/inc", "Search all Missing Incremental"},
	{"GET", "/api/all/search/upgrade/full", "Search all Upgrades"},
	{"GET", "/api/all/search/upgrade/inc", "Search all Upgrades Incremental"},
}

var apiEndpointsMovies = []apiEndpointDoc{
	{"GET", "/api/movies", "Get all movies"},
	{"GET", "/api/movies/unmatched", "Get unmatched movies"},
	{"DELETE", "/api/movies/{id}", "Delete movie"},
	{"GET", "/api/movies/list/{name}", "Get movies by list name"},
	{"GET", "/api/movies/metadata/{imdb}", "Get movie metadata"},
	{"DELETE", "/api/movies/list/{id}", "Delete movie from list"},
	{"GET", "/api/movies/job/{job}", "Execute movie job"},
	{"GET", "/api/movies/job/{job}/{name}", "Execute movie job for config"},
	{"POST", "/api/movies", "Create/update movie"},
	{"POST", "/api/movies/list", "Create/update movie list"},
	{"GET", "/api/movies/search/id/{id}", "Search movie by ID"},
	{"GET", "/api/movies/search/list/{id}", "Search movie list"},
	{"GET", "/api/movies/rss/search/list/{group}", "RSS search movie list"},
	{"POST", "/api/movies/search/download/{id}", "Download movie"},
	{"GET", "/api/movies/all/refreshall", "Refresh all movies metadata"},
	{"GET", "/api/movies/refresh/{id}", "Refresh movie metadata"},
	{"GET", "/api/movies/all/refresh", "Refresh movies"},
	{"GET", "/api/movies/search/history/clear/{name}", "Clear movie search history"},
	{"GET", "/api/movies/search/history/clearid/{id}", "Clear movie search history by ID"},
}

var apiEndpointsSeries = []apiEndpointDoc{
	{"GET", "/api/series", "Get all series"},
	{"DELETE", "/api/series/{id}", "Delete series"},
	{"GET", "/api/series/list/{name}", "Get series by list name"},
	{"DELETE", "/api/series/list/{id}", "Delete series from list"},
	{"GET", "/api/series/metadata/{tvdb}", "Get series metadata"},
	{"GET", "/api/series/unmatched", "Get unmatched series"},
	{"GET", "/api/series/episodes", "Get all episodes"},
	{"GET", "/api/series/episodes/{id}", "Get episode by ID"},
	{"DELETE", "/api/series/episodes/{id}", "Delete episode"},
	{"GET", "/api/series/episodes/list/{id}", "Get episodes by series ID"},
	{"DELETE", "/api/series/episodes/list/{id}", "Delete episodes by series ID"},
	{"GET", "/api/series/job/{job}", "Execute series job"},
	{"GET", "/api/series/job/{job}/{name}", "Execute series job for config"},
	{"POST", "/api/series", "Create/update series"},
	{"POST", "/api/series/episodes", "Create/update episode"},
	{"POST", "/api/series/list", "Create/update series list"},
	{"POST", "/api/series/episodes/list", "Create/update episode list"},
	{"GET", "/api/series/refresh/{id}", "Refresh series metadata"},
	{"GET", "/api/series/all/refreshall", "Refresh all series metadata"},
	{"GET", "/api/series/all/refresh", "Refresh series"},
	{"GET", "/api/series/search/id/{id}", "Search series by ID"},
	{"GET", "/api/series/search/id/{id}/{season}", "Search series season"},
	{"GET", "/api/series/searchrss/id/{id}", "RSS search series"},
	{"GET", "/api/series/searchrss/list/id/{id}", "RSS search series list"},
	{"GET", "/api/series/searchrss/id/{id}/{season}", "RSS search series season"},
	{"GET", "/api/series/episodes/search/id/{id}", "Search episodes"},
	{"GET", "/api/series/episodes/search/list/{id}", "Search episode list"},
	{"GET", "/api/series/rss/search/list/{group}", "RSS search series list"},
	{"POST", "/api/series/episodes/search/download/{id}", "Download episode"},
	{"GET", "/api/series/search/history/clear/{name}", "Clear series search history"},
	{"GET", "/api/series/search/history/clearid/{id}", "Clear series search history by ID"},
}

var apiEndpointsBooks = []apiEndpointDoc{
	{"GET", "/api/books", "Get all books"},
	{"GET", "/api/books/list/{name}", "Get books by list name"},
	{"DELETE", "/api/books/{id}", "Delete book"},
	{"GET", "/api/books/job/{job}", "Execute book job"},
	{"GET", "/api/books/job/{job}/{name}", "Execute book job for config"},
	{"GET", "/api/books/feeds/date/{name}/{listname}", "Get book feed by date"},
	{"GET", "/api/books/rss/search/list/{group}", "RSS search book list"},
	{"GET", "/api/books/search/list/{id}", "Search one book (results)"},
	{"GET", "/api/books/search/missing/{name}", "Search missing books of a group"},
	{"GET", "/api/books/search/history/clear/{name}", "Clear book search history"},
	{"GET", "/api/books/search/history/clearid/{id}", "Clear book search history by ID"},
	{"GET", "/api/books/search/authors/missing/{name}", "Search missing books by author"},
	{"GET", "/api/books/search/authors/upgrade/{name}", "Search upgrade books by author"},
}

var apiEndpointsAudiobooks = []apiEndpointDoc{
	{"GET", "/api/audiobooks/all/refresh", "Refresh audiobooks"},
	{"GET", "/api/audiobooks/all/refreshall", "Refresh all audiobooks metadata"},
	{"GET", "/api/audiobooks/refresh/{id}", "Refresh audiobook metadata"},
	{"GET", "/api/audiobooks/tag/all", "Retag all audiobooks"},
	{"GET", "/api/audiobooks/tag/author/{id}", "Retag all audiobooks for author"},
	{"GET", "/api/audiobooks/tag/{id}", "Retag audiobook"},
	{"GET", "/api/audiobooks", "Get all audiobooks"},
	{"GET", "/api/audiobooks/list/{name}", "Get audiobooks by list name"},
	{"DELETE", "/api/audiobooks/{id}", "Delete audiobook"},
	{"GET", "/api/audiobooks/job/{job}", "Execute audiobook job"},
	{"GET", "/api/audiobooks/job/{job}/{name}", "Execute audiobook job for config"},
	{"GET", "/api/audiobooks/author/add/{name}/{listname}", "Add author to audiobook list"},
	{"GET", "/api/audiobooks/rss/search/list/{group}", "RSS search audiobook list"},
	{"GET", "/api/audiobooks/search/list/{id}", "Search one audiobook (results)"},
	{"GET", "/api/audiobooks/search/missing/{name}", "Search missing audiobooks of a group"},
	{"GET", "/api/audiobooks/search/history/clear/{name}", "Clear audiobook search history"},
	{
		"GET",
		"/api/audiobooks/search/history/clearid/{id}",
		"Clear audiobook search history by ID",
	},
	{"GET", "/api/audiobooks/search/authors/missing/{name}", "Search missing audiobooks by author"},
	{"GET", "/api/audiobooks/search/authors/upgrade/{name}", "Search upgrade audiobooks by author"},
}

var apiEndpointsMusic = []apiEndpointDoc{
	{"GET", "/api/music/all/refresh", "Refresh music"},
	{"GET", "/api/music/all/refreshall", "Refresh all music metadata"},
	{"GET", "/api/music/refresh/{id}", "Refresh album metadata"},
	{"GET", "/api/music/tag/all", "Retag all albums"},
	{"GET", "/api/music/tag/artist/{id}", "Retag all albums for artist"},
	{"GET", "/api/music/tag/{id}", "Retag album"},
	{"GET", "/api/music", "Get all music"},
	{"GET", "/api/music/list/{name}", "Get music by list name"},
	{"DELETE", "/api/music/{id}", "Delete album"},
	{"GET", "/api/music/artists", "Get all artists"},
	{"GET", "/api/music/artist/add/{name}/{listname}", "Add artist to music list"},
	{"GET", "/api/music/job/{job}", "Execute music job"},
	{"GET", "/api/music/job/{job}/{name}", "Execute music job for config"},
	{"GET", "/api/music/feeds/date/{name}/{listname}", "Get music feed by date"},
	{"GET", "/api/music/rss/search/list/{group}", "RSS search music list"},
	{"GET", "/api/music/discover/series/artist/{id}", "Discover series for artist"},
	{"GET", "/api/music/search/list/{id}", "Search one album (results)"},
	{"GET", "/api/music/search/missing/{name}", "Search missing albums of a group"},
	{"GET", "/api/music/search/history/clear/{name}", "Clear music search history"},
	{"GET", "/api/music/search/history/clearid/{id}", "Clear music search history by ID"},
	{"GET", "/api/music/search/artists/missing/{name}", "Search missing albums by artist"},
	{"GET", "/api/music/search/artists/upgrade/{name}", "Search upgrade albums by artist"},
}

var apiEndpointsGeneral = []apiEndpointDoc{
	{"GET", "/api/debugstats", "Get debug statistics"},
	{"GET", "/api/queue", "Get job queue"},
	{"GET", "/api/queue/history", "Get job history"},
	{"DELETE", "/api/queue/cancel/{id}", "Cancel job"},
	{"GET", "/api/trakt/authorize", "Authorize Trakt"},
	{"GET", "/api/trakt/token/{code}", "Get Trakt token"},
	{"GET", "/api/trakt/user/{user}/{list}", "Get Trakt user list"},
	{"GET", "/api/slug", "Get slug information"},
	{"POST", "/api/parse/string", "Parse string"},
	{"POST", "/api/parse/file", "Parse file"},
	{"GET", "/api/fillimdb", "Fill IMDB data"},
	{"GET", "/api/scheduler/stop", "Stop scheduler"},
	{"GET", "/api/scheduler/start", "Start scheduler"},
	{"GET", "/api/scheduler/list", "List scheduled jobs"},
	{"GET", "/api/db/backup", "Backup database"},
	{"GET", "/api/db/integrity", "Check database integrity"},
	{"GET", "/api/db/cleanuporphans", "Clean up orphaned database entries"},
	{"DELETE", "/api/db/clear/{name}", "Clear database table"},
	{"DELETE", "/api/db/delete/{name}/{id}", "Delete database entry"},
	{"DELETE", "/api/db/clearcache", "Clear cache"},
	{"GET", "/api/db/vacuum", "Vacuum database"},
	{"DELETE", "/api/db/oldjobs", "Delete old jobs"},
	{"GET", "/api/quality", "Get quality profiles"},
	{"DELETE", "/api/quality/{id}", "Delete quality profile"},
	{"POST", "/api/quality", "Create/update quality profile"},
	{"GET", "/api/quality/get/{name}", "Get quality by name"},
	{"GET", "/api/quality/all", "Get all qualities"},
	{"GET", "/api/quality/complete", "Get complete qualities"},
	{"GET", "/api/config/all", "Get all configurations"},
	{"DELETE", "/api/config/clear", "Clear configuration"},
	{"GET", "/api/config/get/{name}", "Get config by name"},
	{"DELETE", "/api/config/delete/{name}", "Delete configuration"},
	{"GET", "/api/config/refresh", "Refresh configuration"},
	{"POST", "/api/config/update/{name}", "Update configuration"},
	{"GET", "/api/config/type/{type}", "Get config by type"},
	{"POST", "/api/naming", "Test naming patterns"},
	{"POST", "/api/structure", "Test file structure"},
	{"POST", "/api/album/force-match", "Force-match an album"},
	{"GET", "/api/cache/refresh", "Refresh in-memory cache"},
	{"GET", "/api/cache/cleanupfiles", "Clean up cover and subtitle cache"},
	{"GET", "/api/cache/list", "List cache entries"},
	{"POST", "/api/cache/add", "Add cache entry"},
	{"DELETE", "/api/cache/remove/{key}", "Remove cache entry"},
	{"DELETE", "/api/cache/clear/{type}", "Clear cache by type"},
	{"GET", "/api/statistics", "Get overall statistics"},
	{"GET", "/api/statistics/movies", "Get movie statistics"},
	{"GET", "/api/statistics/series", "Get series statistics"},
	{"GET", "/api/statistics/workers", "Get worker statistics"},
}

// apiEndpointsAdmin covers the session-cookie-authenticated browser admin
// table routes (/api/admin/table/...), not the apikey-authenticated external
// API the rest of this page tests - kept separate and out of the total count
// below since it's a different auth mechanism entirely.
var apiEndpointsAdmin = []apiEndpointDoc{
	{"GET", "/api/admin", "Admin dashboard"},
	{"POST", "/api/admin/table/{name}/insert", "Insert database record"},
	{"POST", "/api/admin/table/{name}/update/{index}", "Update database record"},
	{"POST", "/api/admin/table/{name}/delete/{index}", "Delete database record"},
}

// renderAPITestingPage renders a comprehensive API testing interface.
func renderAPITestingPage() gomponents.Node {
	// Get the actual API key from configuration
	apiKey := config.GetSettingsGeneral().WebAPIKey

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header
		renderEnhancedPageHeader(
			"fa-solid fa-plug",
			"API Testing Suite",
			"Comprehensive testing interface for all API endpoints. Test jobs, searches, configurations, and database operations with real-time feedback.",
		),

		// Main content layout with resizable splitter
		html.Div(
			html.ID("resizable-container"),
			html.Style("display: flex; min-height: 800px;"),
			// Main content area
			html.Div(
				html.ID("main-content"),
				html.Style("flex: 1; padding-right: 15px; min-width: 300px;"),

				// API Configuration Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-secondary text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-key me-2")),
							gomponents.Text("API Configuration"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("row g-3 align-items-end"),
							html.Div(
								html.Class("col-md-8"),
								html.Label(
									html.For("apiKeyInput"),
									html.Class("form-label"),
									gomponents.Text("API Key"),
								),
								html.Div(
									html.Class("input-group"),
									html.Input(
										html.ID("apiKeyInput"),
										html.Type("text"),
										html.Class("form-control"),
										html.Value(apiKey),
										gomponents.Attr("placeholder", "Enter your API key"),
									),
									html.Span(
										html.ID("apiKeyStatus"),
										html.Class("input-group-text bg-light"),
										html.I(html.Class("fas fa-question-circle text-muted")),
									),
								),
								html.Small(
									html.Class("form-text text-muted"),
									gomponents.Text(
										"API key is automatically loaded from configuration. You can override it here if needed.",
									),
								),
							),
							html.Div(
								html.Class("col-md-2"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-primary w-100"),
									gomponents.Attr("onclick", "testAPIKey()"),
									html.I(
										html.Class("fas fa-vial me-2"),
									),
									gomponents.Text("Test Key"),
								),
							),
							html.Div(
								html.Class("col-md-2"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-secondary w-100"),
									gomponents.Attr("onclick", "resetAPIKey()"),
									html.I(
										html.Class("fas fa-undo me-2"),
									),
									gomponents.Text("Reset"),
								),
							),
						),
					),
				),

				// Quick Actions Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-primary text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-bolt me-2")),
							gomponents.Text("Quick Actions"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("row g-3"),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-success w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/debugstats', '', 'Debug Stats')",
									),
									html.I(
										html.Class("fas fa-chart-line me-2"),
									),
									gomponents.Text("Debug Stats"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-info w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/queue', '', 'Job Queue')",
									),
									html.I(
										html.Class("fas fa-tasks me-2"),
									),
									gomponents.Text("Job Queue"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-warning w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/config/all', '', 'All Configs')",
									),
									html.I(
										html.Class("fas fa-cog me-2"),
									),
									gomponents.Text("All Configs"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-secondary w-100"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/db/backup', '', 'DB Backup')",
									),
									html.I(
										html.Class("fas fa-database me-2"),
									),
									gomponents.Text("DB Backup"),
								),
							),
							html.Div(
								html.Class("col-md-3"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-secondary w-100"),
									html.Title("Remove cached covers and subtitles nothing refers to any more"),
									gomponents.Attr(
										"onclick",
										"executeAPICall('GET', '/api/cache/cleanupfiles', '', 'File Cache Cleanup')",
									),
									html.I(
										html.Class("fas fa-broom me-2"),
									),
									gomponents.Text("Clean File Cache"),
								),
							),
						),
					),
				),

				// Job Management Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-info text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-play me-2")),
							gomponents.Text("Job Management"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Form(
							gomponents.Attr("onsubmit", "return executeJobTest(event)"),
							html.Div(
								html.Class("row g-3"),
								html.Div(
									html.Class("col-md-4"),
									html.Label(
										html.For("jobType"),
										html.Class("form-label"),
										gomponents.Text("Job Type"),
									),
									html.Select(
										html.ID("jobType"),
										html.Name("jobType"),
										html.Class("form-select"),
										gomponents.Attr("onchange", "updateJobOptions()"),
										html.Option(
											html.Value(""),
											gomponents.Text("Select Job Type"),
										),
										html.Option(
											html.Value("movies"),
											gomponents.Text("Movies"),
										),
										html.Option(
											html.Value("series"),
											gomponents.Text("Series"),
										),
										html.Option(
											html.Value("books"),
											gomponents.Text("Books"),
										),
										html.Option(
											html.Value("audiobooks"),
											gomponents.Text("Audiobooks"),
										),
										html.Option(
											html.Value("music"),
											gomponents.Text("Music"),
										),
										html.Option(html.Value("db"), gomponents.Text("Database")),
									),
								),
								html.Div(
									html.Class("col-md-4"),
									html.Label(
										html.For("jobAction"),
										html.Class("form-label"),
										gomponents.Text("Action"),
									),
									html.Select(
										html.ID("jobAction"),
										html.Name("jobAction"),
										html.Class("form-select"),
										html.Option(
											html.Value(""),
											gomponents.Text("Select Action"),
										),
									),
								),
								html.Div(
									html.Class("col-md-4"),
									html.Label(
										html.For("jobConfig"),
										html.Class("form-label"),
										gomponents.Text("Config (Optional)"),
									),
									html.Input(
										html.ID("jobConfig"),
										html.Name("jobConfig"),
										html.Type("text"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "EN, DE, X, etc."),
									),
								),
							),
							html.Div(
								html.Class("mt-3"),
								html.Button(
									html.Type("submit"),
									html.Class("btn btn-primary me-2"),
									html.I(
										html.Class("fas fa-rocket me-2"),
									),
									gomponents.Text("Execute Job"),
								),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-secondary"),
									gomponents.Attr("onclick", "clearResults()"),
									html.I(
										html.Class("fas fa-trash me-2"),
									),
									gomponents.Text("Clear Results"),
								),
							),
						),
					),
				),

				// Custom API Testing Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-dark text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-terminal me-2")),
							gomponents.Text("Custom API Testing"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Form(
							gomponents.Attr("onsubmit", "return executeCustomAPITest(event)"),
							html.Div(
								html.Class("row g-3"),
								html.Div(
									html.Class("col-md-2"),
									html.Label(
										html.For("httpMethod"),
										html.Class("form-label"),
										gomponents.Text("Method"),
									),
									html.Select(
										html.ID("httpMethod"),
										html.Name("httpMethod"),
										html.Class("form-select"),
										html.Option(
											html.Value("GET"),
											gomponents.Attr("selected"),
											gomponents.Text("GET"),
										),
										html.Option(html.Value("POST"), gomponents.Text("POST")),
										html.Option(html.Value("PUT"), gomponents.Text("PUT")),
										html.Option(
											html.Value("DELETE"),
											gomponents.Text("DELETE"),
										),
									),
								),
								html.Div(
									html.Class("col-md-10"),
									html.Label(
										html.For("apiEndpoint"),
										html.Class("form-label"),
										gomponents.Text("API Endpoint"),
									),
									html.Input(
										html.ID("apiEndpoint"),
										html.Name("apiEndpoint"),
										html.Type("text"),
										html.Class("form-control"),
										gomponents.Attr(
											"placeholder",
											"/api/movies or /api/series/search/id/123",
										),
									),
								),
							),
							html.Div(
								html.Class("mt-3"),
								html.Label(
									html.For("requestBody"),
									html.Class("form-label"),
									gomponents.Text("Request Body (JSON for POST/PUT)"),
								),
								html.Textarea(
									html.ID("requestBody"),
									html.Name("requestBody"),
									html.Class("form-control"),
									gomponents.Attr("rows", "4"),
									gomponents.Attr("placeholder", `{"key": "value"}`),
								),
							),
							html.Div(
								html.Class("mt-3"),
								html.Button(
									html.Type("submit"),
									html.Class("btn btn-primary me-2"),
									html.I(
										html.Class("fas fa-paper-plane me-2"),
									),
									gomponents.Text("Send Request"),
								),
							),
						),
					),
				),

				// Search & Download Testing
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-success text-white"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-search me-2")),
							gomponents.Text("Search & Download Testing"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("row g-3"),
							html.Div(
								html.Class("col-md-6"),
								html.H6(gomponents.Text("Movie Search")),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("movieSearchId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Movie ID"),
									),
									html.Button(
										html.Class("btn btn-outline-primary"),
										html.Type("button"),
										gomponents.Attr("onclick", "testMovieSearch()"),
										gomponents.Text("Search Movie"),
									),
								),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("movieListId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Movie List ID"),
									),
									html.Button(
										html.Class("btn btn-outline-success"),
										html.Type("button"),
										gomponents.Attr("onclick", "testMovieListSearch()"),
										gomponents.Text("Search List"),
									),
								),
							),
							html.Div(
								html.Class("col-md-6"),
								html.H6(gomponents.Text("Series Search")),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("seriesSearchId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Series ID"),
									),
									html.Button(
										html.Class("btn btn-outline-primary"),
										html.Type("button"),
										gomponents.Attr("onclick", "testSeriesSearch()"),
										gomponents.Text("Search Series"),
									),
								),
								html.Div(
									html.Class("input-group mb-2"),
									html.Input(
										html.ID("episodeId"),
										html.Type("number"),
										html.Class("form-control"),
										gomponents.Attr("placeholder", "Episode ID"),
									),
									html.Button(
										html.Class("btn btn-outline-success"),
										html.Type("button"),
										gomponents.Attr("onclick", "testEpisodeSearch()"),
										gomponents.Text("Search Episode"),
									),
								),
							),
						),
					),
				),

				// API Endpoints Reference Section
				html.Div(
					html.Class("card border-0 shadow-sm mb-4"),
					html.Div(
						html.Class("card-header bg-gradient-warning text-dark"),
						html.H5(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-book me-2")),
							gomponents.Text("API Endpoints Reference"),
						),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("accordion"),
							html.ID("endpointsAccordion"),

							renderAPIEndpointsSection("All", "All Endpoints", apiEndpointsAll),
							renderAPIEndpointsSection("Movies", "Movies Endpoints", apiEndpointsMovies),
							renderAPIEndpointsSection("Series", "Series Endpoints", apiEndpointsSeries),
							renderAPIEndpointsSection("Books", "Books Endpoints", apiEndpointsBooks),
							renderAPIEndpointsSection(
								"Audiobooks",
								"Audiobooks Endpoints",
								apiEndpointsAudiobooks,
							),
							renderAPIEndpointsSection("Music", "Music Endpoints", apiEndpointsMusic),
							renderAPIEndpointsSection(
								"General",
								"General & System Endpoints",
								apiEndpointsGeneral,
							),
							renderAPIEndpointsSection(
								"Admin",
								"Admin Endpoints (browser session auth, not apikey)",
								apiEndpointsAdmin,
							),
						),

						html.Div(
							html.Class("mt-3 p-3 bg-light rounded"),
							html.H6(html.Class("fw-bold mb-2"), gomponents.Text("Quick Reference")),
							html.P(
								html.Class("mb-1 small"),
								html.Strong(gomponents.Text("Total Endpoints:")),
								gomponents.Text(fmt.Sprintf(
									" %d external API (apikey auth) + %d admin table routes (session auth)",
									len(apiEndpointsAll)+len(apiEndpointsMovies)+len(apiEndpointsSeries)+
										len(apiEndpointsBooks)+len(apiEndpointsAudiobooks)+len(apiEndpointsMusic)+
										len(apiEndpointsGeneral),
									len(apiEndpointsAdmin),
								)),
							),
							html.P(
								html.Class("mb-1 small"),
								html.Strong(gomponents.Text("Authentication:")),
								gomponents.Text(" All endpoints require 'apikey' query parameter"),
							),
							html.P(
								html.Class("mb-1 small"),
								html.Strong(gomponents.Text("Base URL:")),
								gomponents.Text(" /api/{category}/{action}"),
							),
							html.P(
								html.Class("mb-0 small"),
								html.Strong(gomponents.Text("Response Format:")),
								gomponents.Text(" JSON (success/error messages or data)"),
							),
						),
					),
				),
			), // End main content

			// Resizable divider
			html.Div(
				html.ID("resizer"),
				html.Style(
					"width: 5px; cursor: col-resize; background: #dee2e6; border-left: 1px solid #adb5bd; border-right: 1px solid #adb5bd; flex-shrink: 0;",
				),
				gomponents.Attr("title", "Drag to resize"),
			),

			// Sidebar for results
			html.Div(
				html.ID("sidebar-content"),
				html.Style("width: 400px; min-width: 300px; max-width: 800px; padding-left: 15px;"),
				// Results Section
				html.Div(
					html.Class("card border-0 shadow-sm sticky-top"),
					html.Style("top: 20px;"), // Sticky positioning
					html.Div(
						html.Class(
							"card-header bg-primary text-white d-flex justify-content-between align-items-center",
						),
						html.H6(
							html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-list-alt me-2")),
							gomponents.Text("Test Results"),
						),
						html.Div(
							html.Class("btn-group btn-group-sm"),
							html.Button(
								html.Type("button"),
								html.Class("btn btn-light btn-sm"),
								gomponents.Attr("onclick", "copyResults()"),
								gomponents.Attr("title", "Copy Results"),
								html.I(html.Class("fas fa-copy"), html.Style("color: black;")),
							),
							html.Button(
								html.Type("button"),
								html.Class("btn btn-light btn-sm"),
								gomponents.Attr("onclick", "clearResults()"),
								gomponents.Attr("title", "Clear Results"),
								html.I(html.Class("fas fa-trash"), html.Style("color: black;")),
							),
						),
					),
					html.Div(
						html.Class("card-body p-2"),
						html.Div(
							html.ID("testResults"),
							html.Class("bg-light p-2 rounded"),
							html.Style(
								"min-height: 400px; font-family: 'Courier New', monospace; white-space: pre-wrap; overflow: auto; font-size: 0.8rem; line-height: 1.3; resize: both; height: 600px;",
							),
							gomponents.Text("Test results will appear here..."),
						),
					),
				),
			), // End sidebar content
		), // End resizable container

		// JavaScript for functionality
		html.Script(gomponents.Raw(`
			// Store the original API key for reset functionality
			const originalAPIKey = '`+apiKey+`';
			
			// Job options mapping - kept in sync with each type's allowedjobs*str
			// constant in routes.go.
			const jobOptions = {
				movies: ['rss', 'data', 'datafull', 'checkmissing', 'checkmissingflag', 'checkreachedflag', 'structure', 'searchmissingfull', 'searchmissinginc', 'searchupgradefull', 'searchupgradeinc', 'searchmissingfulltitle', 'searchmissinginctitle', 'searchupgradefulltitle', 'searchupgradeinctitle', 'clearhistory', 'feeds', 'refresh', 'refreshinc'],
				series: ['rss', 'rssseasons', 'rssseasonsall', 'data', 'datafull', 'checkmissing', 'checkmissingflag', 'checkreachedflag', 'structure', 'searchmissingfull', 'searchmissinginc', 'searchupgradefull', 'searchupgradeinc', 'searchmissingfulltitle', 'searchmissinginctitle', 'searchupgradefulltitle', 'searchupgradeinctitle', 'clearhistory', 'feeds', 'refresh', 'refreshinc'],
				books: ['rss', 'rssauthors', 'rssauthorsupgrade', 'data', 'datafull', 'checkmissing', 'checkmissingflag', 'checkreachedflag', 'structure', 'searchmissingfull', 'searchmissinginc', 'searchupgradefull', 'searchupgradeinc', 'searchmissingfulltitle', 'searchmissinginctitle', 'searchupgradefulltitle', 'searchupgradeinctitle', 'clearhistory', 'feeds', 'refresh', 'refreshinc'],
				audiobooks: ['rss', 'rssauthors', 'rssauthorsupgrade', 'data', 'datafull', 'checkmissing', 'checkmissingflag', 'checkreachedflag', 'structure', 'searchmissingfull', 'searchmissinginc', 'searchupgradefull', 'searchupgradeinc', 'searchmissingfulltitle', 'searchmissinginctitle', 'searchupgradefulltitle', 'searchupgradeinctitle', 'clearhistory', 'feeds', 'refresh', 'refreshinc'],
				music: ['rss', 'rssartists', 'rssartistsupgrade', 'data', 'datafull', 'checkmissing', 'checkmissingflag', 'checkreachedflag', 'structure', 'searchmissingfull', 'searchmissinginc', 'searchupgradefull', 'searchupgradeinc', 'searchmissingfulltitle', 'searchmissinginctitle', 'searchupgradefulltitle', 'searchupgradeinctitle', 'clearhistory', 'feeds', 'refresh', 'refreshinc'],
				db: ['backup', 'integrity', 'clearcache', 'clear/serie_episode_histories', 'clear/movie_histories', 'clear/r_sshistories']
			};

			// DELETE db actions (matches the DELETE routes in routes.go) - every
			// other db action, and every non-db job, is a GET.
			const dbDeleteActions = ['clearcache', 'clear/serie_episode_histories', 'clear/movie_histories', 'clear/r_sshistories'];
			
			function updateJobOptions() {
				const jobType = document.getElementById('jobType').value;
				const jobAction = document.getElementById('jobAction');
				
				jobAction.innerHTML = '<option value="">Select Action</option>';
				
				if (jobType && jobOptions[jobType]) {
					jobOptions[jobType].forEach(action => {
						const option = document.createElement('option');
						option.value = action;
						option.textContent = action;
						jobAction.appendChild(option);
					});
				}
			}
			
			function executeJobTest(event) {
				event.preventDefault();
				
				const jobType = document.getElementById('jobType').value;
				const jobAction = document.getElementById('jobAction').value;
				const jobConfig = document.getElementById('jobConfig').value;
				
				if (!jobType || !jobAction) {
					showResult('Error: Please select job type and action', 'error');
					return false;
				}
				
				let endpoint, method = 'GET';
				if (jobType === 'db') {
					endpoint = '/api/db/' + jobAction;
					if (dbDeleteActions.includes(jobAction)) {
						method = 'DELETE';
					}
				} else {
					endpoint = '/api/' + jobType + '/job/' + jobAction;
					if (jobConfig) {
						endpoint += '/' + jobConfig;
					}
				}

				executeAPICall(method, endpoint, '', 'Job: ' + jobType + '/' + jobAction);
				return false;
			}
			
			function executeCustomAPITest(event) {
				event.preventDefault();
				
				const method = document.getElementById('httpMethod').value;
				const endpoint = document.getElementById('apiEndpoint').value;
				const body = document.getElementById('requestBody').value;
				
				if (!endpoint) {
					showResult('Error: Please enter an API endpoint', 'error');
					return false;
				}
				
				executeAPICall(method, endpoint, body, 'Custom: ' + method + ' ' + endpoint);
				return false;
			}
			
			function testMovieSearch() {
				const movieId = document.getElementById('movieSearchId').value;
				if (movieId) {
					executeAPICall('GET', '/api/movies/search/id/' + movieId, '', 'Movie Search ID: ' + movieId);
				}
			}
			
			function testMovieListSearch() {
				const listId = document.getElementById('movieListId').value;
				if (listId) {
					executeAPICall('GET', '/api/movies/search/list/' + listId + '?searchByTitle=true&download=true', '', 'Movie List Search: ' + listId);
				}
			}
			
			function testSeriesSearch() {
				const seriesId = document.getElementById('seriesSearchId').value;
				if (seriesId) {
					executeAPICall('GET', '/api/series/search/id/' + seriesId, '', 'Series Search ID: ' + seriesId);
				}
			}
			
			function testEpisodeSearch() {
				const episodeId = document.getElementById('episodeId').value;
				if (episodeId) {
					executeAPICall('GET', '/api/series/episodes/search/list/' + episodeId + '?searchByTitle=true&download=true', '', 'Episode Search: ' + episodeId);
				}
			}
			
			function executeAPICall(method, endpoint, body, title) {
				const resultsDiv = document.getElementById('testResults');
				const timestamp = new Date().toLocaleTimeString();
				
				showResult('[' + timestamp + '] Executing: ' + title + '\nEndpoint: ' + method + ' ' + endpoint + '\n' + '-'.repeat(50), 'info');
				
				const options = {
					method: method,
					headers: {
						'Content-Type': 'application/json'
					}
				};
				
				if (body && (method === 'POST' || method === 'PUT')) {
					options.body = body;
				}
				
				// Add API key if endpoint starts with /api/
				let fullUrl = endpoint;
				if (endpoint.startsWith('/api/')) {
					const apiKey = document.getElementById('apiKeyInput').value || originalAPIKey;
					fullUrl += (endpoint.includes('?') ? '&' : '?') + 'apikey=' + encodeURIComponent(apiKey);
				}
				
				fetch(fullUrl, options)
					.then(response => {
						const statusText = response.status + ' ' + response.statusText;
						return response.text().then(text => {
							try {
								const json = JSON.parse(text);
								return { status: statusText, data: JSON.stringify(json, null, 2), isJson: true };
							} catch {
								return { status: statusText, data: text, isJson: false };
							}
						});
					})
					.then(result => {
						showResult('Response: ' + result.status + '\n\n' + result.data + '\n' + '='.repeat(70) + '\n', result.status.startsWith('2') ? 'success' : 'error');
					})
					.catch(error => {
						showResult('Error: ' + error.message + '\n' + '='.repeat(70) + '\n', 'error');
					});
			}
			
			function showResult(message, type) {
				const resultsDiv = document.getElementById('testResults');
				const colorClass = type === 'error' ? 'text-danger' : (type === 'success' ? 'text-success' : 'text-info');
				
				if (resultsDiv.textContent === 'Test results will appear here...') {
					resultsDiv.innerHTML = '<div class="' + colorClass + '">' + escapeHtml(message) + '</div>';
				} else {
					resultsDiv.innerHTML += '<div class="' + colorClass + '">' + escapeHtml(message) + '</div>';
				}
				
				resultsDiv.scrollTop = resultsDiv.scrollHeight;
			}
			
			function clearResults() {
				document.getElementById('testResults').textContent = 'Test results will appear here...';
			}
			
			function copyResults() {
				const results = document.getElementById('testResults').textContent;
				navigator.clipboard.writegomponents.Text(results).then(() => {
					alert('Results copied to clipboard!');
				});
			}
			
			function escapeHtml(text) {
				const div = document.createElement('div');
				div.textContent = text;
				return div.innerHTML;
			}
			
			function testAPIKey() {
				const apiKey = document.getElementById('apiKeyInput').value;
				if (!apiKey) {
					showResult('Error: Please enter an API key to test', 'error');
					updateAPIKeyStatus('error', 'Missing API key');
					return;
				}
				
				updateAPIKeyStatus('testing', 'Testing...');
				showResult('Testing API key: ' + apiKey.substring(0, 8) + '...', 'info');
				
				// Test with a simple endpoint
				const options = {
					method: 'GET',
					headers: { 'Content-Type': 'application/json' }
				};
				
				const testUrl = '/api/debugstats?apikey=' + encodeURIComponent(apiKey);
				
				fetch(testUrl, options)
					.then(response => {
						if (response.ok) {
							updateAPIKeyStatus('success', 'Valid API key');
							showResult('✅ API Key Test: Valid', 'success');
						} else if (response.status === 401) {
							updateAPIKeyStatus('error', 'Invalid API key');
							showResult('❌ API Key Test: Invalid or unauthorized', 'error');
						} else {
							updateAPIKeyStatus('warning', 'Unexpected response');
							showResult('⚠️ API Key Test: Unexpected response (' + response.status + ')', 'error');
						}
					})
					.catch(error => {
						updateAPIKeyStatus('error', 'Connection failed');
						showResult('❌ API Key Test: Connection failed - ' + error.message, 'error');
					});
			}
			
			function resetAPIKey() {
				document.getElementById('apiKeyInput').value = originalAPIKey;
				updateAPIKeyStatus('reset', 'Reset to original');
				showResult('API key reset to original value', 'info');
			}
			
			function updateAPIKeyStatus(type, message) {
				const statusElement = document.getElementById('apiKeyStatus');
				let iconClass, textColor;
				
				switch(type) {
					case 'success':
						iconClass = 'fas fa-check-circle';
						textColor = 'text-success';
						break;
					case 'error':
						iconClass = 'fas fa-times-circle';
						textColor = 'text-danger';
						break;
					case 'warning':
						iconClass = 'fas fa-exclamation-triangle';
						textColor = 'text-warning';
						break;
					case 'testing':
						iconClass = 'fas fa-spinner fa-spin';
						textColor = 'text-info';
						break;
					default:
						iconClass = 'fas fa-question-circle';
						textColor = 'text-muted';
				}
				
				statusElement.innerHTML = '<i class="' + iconClass + ' ' + textColor + '"></i>';
				statusElement.title = message;
			}
			
			// Resizer functionality
			let isResizing = false;
			let startX = 0;
			let startSidebarWidth = 0;
			
			const resizer = document.getElementById('resizer');
			const sidebar = document.getElementById('sidebar-content');
			const container = document.getElementById('resizable-container');
			
			if (resizer && sidebar) {
				resizer.addEventListener('mousedown', function(e) {
					isResizing = true;
					startX = e.clientX;
					startSidebarWidth = parseInt(window.getComputedhtml.Style(sidebar).width, 10);
					
					document.addEventListener('mousemove', doResize);
					document.addEventListener('mouseup', stopResize);
					
					// Prevent text selection during resize
					document.body.style.userSelect = 'none';
					document.body.style.webkitUserSelect = 'none';
					document.body.style.msUserSelect = 'none';
					
					e.preventDefault();
				});
				
				function doResize(e) {
					if (!isResizing) return;
					
					const deltaX = startX - e.clientX;
					const newWidth = startSidebarWidth + deltaX;
					
					// Set min and max width constraints
					const minWidth = 300;
					const maxWidth = 800;
					const containerWidth = container.offsetWidth;
					const maxAllowedWidth = containerWidth - 400; // Leave space for main content
					
					const constrainedWidth = Math.max(minWidth, Math.min(newWidth, Math.min(maxWidth, maxAllowedWidth)));
					
					sidebar.style.width = constrainedWidth + 'px';
				}
				
				function stopResize() {
					isResizing = false;
					document.removeEventListener('mousemove', doResize);
					document.removeEventListener('mouseup', stopResize);
					
					// Restore text selection
					document.body.style.userSelect = '';
					document.body.style.webkitUserSelect = '';
					document.body.style.msUserSelect = '';
				}
			}
		`)),
	)
}

// HandleAPITesting handles API testing requests.
func HandleAPITesting(c *gin.Context) {
	// This handler could be extended to provide additional server-side functionality
	// For now, the frontend JavaScript handles most of the API testing functionality
	c.String(http.StatusOK, renderAlert("API Testing functionality is handled client-side", "info"))
}

package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderMissingEpisodesFinderPage renders a page for finding missing episodes.
func renderMissingEpisodesFinderPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-exclamation-triangle",
			"Missing Episodes Finder",
			"Search for gaps in your series collections and optionally trigger automatic downloads for missing episodes.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("missingEpisodesForm"),
			// The shared Choices.js dropdown init reads its CSRF token from here.
			html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Search Configuration"),
					),

					html.Div(
						html.Class("form-group mb-3"),
						html.Label(html.Class("form-label"), html.For("missing_SerieID"), gomponents.Text("Series")),
						html.Select(
							html.ID("missing_SerieID"),
							html.Name("missing_SerieID"),
							html.Class("form-select choices-ajax"),
							html.Data("ajax-url", "/api/admin/dropdown/series/id"),
							html.Data("placeholder", "All series"),
							html.Data("allow-clear", "true"),
							html.Option(html.Value(""), gomponents.Text("All series")),
						),
						html.Small(html.Class("form-text text-muted"),
							gomponents.Text("Type to search your tracked series (name - list); leave empty to scan all series")),
					),

					renderFormGroup("missing", map[string]string{
						"SeasonNumber": "Specific season to check (0 = all seasons)",
					}, map[string]string{
						"SeasonNumber": "Season Number",
					}, "SeasonNumber", "number", "0", nil),

					renderFormGroup("missing", map[string]string{
						"Status": "Filter by series status",
					}, map[string]string{
						"Status": "Series Status",
					}, "Status", "select", "all", map[string][]string{
						"options": {"all", "continuing", "ended", "upcoming"},
					}),

					renderFormGroup("missing",
						map[string]string{
							"IncludeSpecials": "Search for missing special episodes (Season 0)",
						},
						map[string]string{
							"IncludeSpecials": "Include Special Episodes",
						},
						"IncludeSpecials", "checkbox", false, nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Action Configuration"),
					),

					renderFormGroup("missing", map[string]string{
						"DateRange": "How far back to look for missing episodes",
					}, map[string]string{
						"DateRange": "Date Range",
					}, "DateRange", "select", "30", map[string][]string{
						"options": {"7", "30", "90", "180", "365", "all"},
					}),

					renderFormGroup("missing",
						map[string]string{
							"AutoDownload": "Automatically search and download missing episodes",
						},
						map[string]string{
							"AutoDownload": "Auto-Download Missing",
						},
						"AutoDownload", "checkbox", false, nil),

					renderFormGroup("missing",
						map[string]string{
							"OnlyAired": "Only look for episodes that have already aired",
						},
						map[string]string{
							"OnlyAired": "Only Already Aired",
						},
						"OnlyAired", "checkbox", true, nil),

					renderFormGroup("missing",
						map[string]string{
							"ShowAllInTable": "Display all missing episodes in a sortable, searchable table instead of showing only first 5",
						},
						map[string]string{
							"ShowAllInTable": "Show All in DataTable",
						},
						"ShowAllInTable", "checkbox", false, nil),

					renderFormGroup("missing", map[string]string{
						"QualityProfile": "Quality profile to use for downloads",
					}, map[string]string{
						"QualityProfile": "Quality Profile",
					}, "QualityProfile", "select", "default", map[string][]string{
						"options": missingEpisodesQualityOptions(),
					}),
				),
			),

			renderHTMXSubmitButton(
				"Find Missing Episodes",
				"missingResults",
				"/api/admin/missing-episodes",
				"missingEpisodesForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('missingEpisodesForm').reset(); document.getElementById('missingResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("missingResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// missingEpisodesQualityOptions lists "default" (the series' own media
// config quality) plus every configured quality profile name.
func missingEpisodesQualityOptions() []string {
	opts := []string{"default"}
	for _, qc := range config.GetSettingsQualityAll() {
		opts = append(opts, qc.Name)
	}

	return opts
}

// maxMissingEpisodeSearches caps how many episodes one "auto-download" run
// of the missing-episodes finder queues; each is a full indexer search.
const maxMissingEpisodeSearches = 100

// HandleMissingEpisodes handles missing episodes finder requests.
func HandleMissingEpisodes(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	serieID, _ := strconv.Atoi(c.PostForm("missing_SerieID"))
	seasonNumberStr := c.PostForm("missing_SeasonNumber")
	status := c.PostForm("missing_Status")
	includeSpecials := c.PostForm("missing_IncludeSpecials") == "on" ||
		c.PostForm("missing_IncludeSpecials") == "true"
	autoDownload := c.PostForm("missing_AutoDownload") == "on" ||
		c.PostForm("missing_AutoDownload") == "true"
	onlyAired := c.PostForm("missing_OnlyAired") == "on" ||
		c.PostForm("missing_OnlyAired") == "true"
	dateRangeStr := c.PostForm("missing_DateRange")
	qualityProfile := c.PostForm("missing_QualityProfile")
	showAllInTable := c.PostForm("missing_ShowAllInTable") == "on" ||
		c.PostForm("missing_ShowAllInTable") == "true"

	// Parse numeric values
	seasonNumber, err := strconv.Atoi(seasonNumberStr)
	if err != nil {
		seasonNumber = 0
	}

	// "all" (or anything non-numeric) means no date range limit.
	dateRangeDays, err := strconv.Atoi(dateRangeStr)
	if err != nil || dateRangeDays < 0 {
		dateRangeDays = 0
	}

	startTime := time.Now()

	var (
		queryWhere string
		queryArgs  []any
	)

	queryWhere = "serie_episodes.missing = 1"

	seriesLabel := "All series"
	if serieID > 0 {
		queryWhere += " AND series.id = ?"

		queryArgs = append(queryArgs, serieID)

		seriesLabel = database.Getdatarow[string](false,
			"SELECT COALESCE(dbseries.seriename, '(missing #' || series.dbserie_id || ')') || ' - ' || series.listname "+
				"FROM series LEFT JOIN dbseries ON dbseries.id = series.dbserie_id WHERE series.id = ?", &serieID)
		if seriesLabel == "" {
			c.String(http.StatusOK, renderAlert(fmt.Sprintf("Series %d not found", serieID), "warning"))
			return
		}
	}

	if seasonNumber > 0 {
		queryWhere += " AND dbserie_episodes.season = ?"

		queryArgs = append(queryArgs, strconv.Itoa(seasonNumber))
	}

	if !includeSpecials {
		queryWhere += " AND dbserie_episodes.season != '0'"
	}

	// first_aired holds Go's time text ("2025-03-14 00:00:00 +0000 UTC"),
	// which SQLite's DATE() can't parse; its first 10 characters are the date.
	// Episodes without a known air date (NULL, '' or the zero time) are kept by
	// both date filters.
	const unknownAired = "(dbserie_episodes.first_aired IS NULL OR dbserie_episodes.first_aired = '' OR " +
		"dbserie_episodes.first_aired LIKE '0001-01-01%')"

	if onlyAired {
		queryWhere += " AND (" + unknownAired + " OR substr(dbserie_episodes.first_aired, 1, 10) <= ?)"

		queryArgs = append(queryArgs, time.Now().Format("2006-01-02"))
	}

	if dateRangeDays > 0 {
		queryWhere += " AND (" + unknownAired + " OR substr(dbserie_episodes.first_aired, 1, 10) >= ?)"

		queryArgs = append(queryArgs, time.Now().AddDate(0, 0, -dateRangeDays).Format("2006-01-02"))
	}

	// The form's status select is the series status (continuing/ended/upcoming).
	if status != "" && status != "all" {
		queryWhere += " AND dbseries.status LIKE ?"

		queryArgs = append(queryArgs, status)
	}

	missingEpisodes := database.StructscanT[database.SerieEpisode](false, 0,
		"SELECT serie_episodes.* FROM serie_episodes "+
			"INNER JOIN series ON series.id = serie_episodes.serie_id "+
			"INNER JOIN dbseries ON dbseries.id = series.dbserie_id "+
			"INNER JOIN dbserie_episodes ON dbserie_episodes.id = serie_episodes.dbserie_episode_id "+
			"WHERE "+queryWhere+
			" ORDER BY dbseries.seriename, series.listname, CAST(dbserie_episodes.season AS INTEGER), CAST(dbserie_episodes.episode AS INTEGER)",
		queryArgs...)

	totalMissing := len(missingEpisodes)

	seriesMap := make(map[uint]bool)
	for i := range missingEpisodes {
		seriesMap[missingEpisodes[i].SerieID] = true
	}

	seriesScanned := len(seriesMap)

	downloadNote := ""

	if autoDownload && totalMissing > 0 {
		toSearch := make([]database.SerieEpisode, 0, min(totalMissing, maxMissingEpisodeSearches))
		ignored := 0

		for i := range missingEpisodes {
			if missingEpisodes[i].DontSearch {
				ignored++
				continue
			}

			if len(toSearch) < maxMissingEpisodeSearches {
				toSearch = append(toSearch, missingEpisodes[i])
			}
		}

		eligible := totalMissing - ignored

		if len(toSearch) == 0 {
			downloadNote = "No episodes queued (all matching episodes have searching disabled)"
		} else {
			profile := qualityProfile
			jobEpisodes := toSearch

			err := worker.Dispatch(
				"missing_episodes_search_"+strconv.FormatInt(time.Now().UnixNano(), 10),
				func(_ uint32, ctx context.Context) error {
					res := triggerEpisodeDownloads(ctx, jobEpisodes, profile)
					logger.Logtype("info", 0).
						Int("total", res.TotalEpisodes).
						Int("searched", res.TriggeredDownloads).
						Int("failed", res.FailedDownloads).
						Int("skipped", res.SkippedEpisodes).
						Msg("missing episodes search finished")

					return nil
				},
				"Search",
			)
			if err != nil {
				c.String(
					http.StatusOK,
					renderAlert("Failed to queue episode searches: "+err.Error(), "danger"),
				)

				return
			}

			downloadNote = fmt.Sprintf("%d episodes queued for search (background job)", len(toSearch))
			if eligible > len(toSearch) {
				downloadNote += fmt.Sprintf(" - capped at %d of %d eligible, run again for the rest", maxMissingEpisodeSearches, eligible)
			}
		}

		if ignored > 0 {
			downloadNote += fmt.Sprintf("; %d episodes with searching disabled were not searched", ignored)
		}
	}

	scanDuration := time.Since(startTime)

	results := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Search Configuration:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Series:")), html.Td(gomponents.Text(seriesLabel))),
		html.Tr(html.Td(gomponents.Text("Season:")), html.Td(gomponents.Text(func() string {
			if seasonNumber == 0 {
				return "All seasons"
			}

			return fmt.Sprintf("Season %d", seasonNumber)
		}()))),
		html.Tr(html.Td(gomponents.Text("Status Filter:")), html.Td(gomponents.Text(status))),
		html.Tr(
			html.Td(gomponents.Text("Include Specials:")),
			html.Td(gomponents.Text(func() string {
				if includeSpecials {
					return "Yes"
				}

				return "No"
			}())),
		),
		html.Tr(html.Td(gomponents.Text("Only Aired:")), html.Td(gomponents.Text(func() string {
			if onlyAired {
				return "Yes"
			}

			return "No"
		}()))),
		html.Tr(
			html.Td(gomponents.Text("Scan Duration:")),
			html.Td(gomponents.Text(scanDuration.String())),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Search Results:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(
			html.Td(gomponents.Text("Series Scanned:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", seriesScanned))),
		),
		html.Tr(
			html.Td(gomponents.Text("Missing Episodes:")),
			html.Td(gomponents.Text(fmt.Sprintf("%d", totalMissing))),
		),
	}

	if totalMissing > 0 {
		if showAllInTable {
			// Show all missing episodes in a detailed datatable
			results = append(
				results,
				html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
				html.Tr(
					html.Td(html.Strong(gomponents.Text("All Missing Episodes:"))),
					html.Td(gomponents.Text("")),
				),
			)
		} else {
			// Show actual missing episode details (first few episodes)
			var sampleEpisodes []string

			limit := min(
				// Show first 5 missing episodes
				len(missingEpisodes), 5)

			for i := range limit {
				// Get episode details from database
				episodeDetails := database.StructscanT[database.DbserieEpisode](false, 1,
					"SELECT season, episode, title FROM dbserie_episodes WHERE id = ?",
					&missingEpisodes[i].DbserieEpisodeID)

				if len(episodeDetails) > 0 {
					ep := episodeDetails[0]

					episodeTitle := fmt.Sprintf("S%02sE%02s", ep.Season, ep.Episode)
					if ep.Title != "" {
						episodeTitle += " - " + ep.Title
					}

					sampleEpisodes = append(sampleEpisodes, episodeTitle)
				}
			}

			if len(sampleEpisodes) > 0 {
				results = append(
					results,
					html.Tr(
						html.Td(gomponents.Text("Sample Missing:")),
						html.Td(gomponents.Text(strings.Join(sampleEpisodes, ", "))),
					),
				)
			}
		}

		if autoDownload {
			results = append(
				results,
				html.Tr(
					html.Td(gomponents.Text("Downloads Triggered:")),
					html.Td(gomponents.Text(downloadNote)),
				),
				html.Tr(
					html.Td(gomponents.Text("Quality Profile:")),
					html.Td(gomponents.Text(qualityProfile)),
				),
			)
		}
	}

	alertClass := "card border-0 shadow-sm border-info mb-4"

	message := "Missing Episodes Search Complete"
	if totalMissing == 0 {
		alertClass = "card border-0 shadow-sm border-success mb-4"
		message = "No Missing Episodes Found"
	} else if totalMissing > 20 {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = fmt.Sprintf("%d Missing Episodes Found - Review Required", totalMissing)
	}

	var resultContent gomponents.Node

	if showAllInTable && totalMissing > 0 {
		// Create a comprehensive datatable for all missing episodes
		tableRows := make([]gomponents.Node, 0)

		for _, episode := range missingEpisodes {
			// Get episode details and series name
			episodeDetails := database.StructscanT[database.DbserieEpisode](false, 1,
				"SELECT season, episode, title FROM dbserie_episodes WHERE id = ?",
				&episode.DbserieEpisodeID)

			var seriesLabel string
			database.Scanrowsdyn(false,
				"SELECT dbseries.seriename || ' (' || series.listname || ')' FROM series INNER JOIN dbseries ON dbseries.id = series.dbserie_id WHERE series.id = ?",
				&seriesLabel, &episode.SerieID)

			if len(episodeDetails) > 0 && seriesLabel != "" {
				ep := episodeDetails[0]

				tableRows = append(tableRows, html.Tr(
					html.Td(gomponents.Text(seriesLabel)),
					html.Td(gomponents.Text(ep.Season)),
					html.Td(gomponents.Text(ep.Episode)),
					html.Td(gomponents.Text(ep.Title)),
					html.Td(gomponents.Text(func() string {
						if episode.Missing {
							return "Missing"
						}

						return "Available"
					}())),
					html.Td(gomponents.Text(episode.QualityProfile)),
				))
			}
		}

		resultContent = html.Div(
			html.Class(alertClass),
			html.Div(
				html.Class("card-header border-0"),
				html.Style("background: linear-gradient(135deg, "+func() string {
					if totalMissing == 0 {
						return "#d4edda 0%, #c3e6cb 100%"
					} else if totalMissing > 20 {
						return "#fff3cd 0%, #ffeaa7 100%"
					}

					return "#d1ecf1 0%, #bee5eb 100%"
				}()+"); border-radius: 15px 15px 0 0;"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(html.Class("badge "+func() string {
						if totalMissing == 0 {
							return "bg-success"
						} else if totalMissing > 20 {
							return "bg-warning"
						}

						return "bg-info"
					}()+" me-3"), html.I(html.Class("fas fa-search me-1")), gomponents.Text("Search")),
					html.H5(html.Class("card-title mb-0 "+func() string {
						if totalMissing == 0 {
							return "text-success"
						} else if totalMissing > 20 {
							return "text-warning"
						}

						return "text-info"
					}()+" fw-bold"), gomponents.Text(message)),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(results)),
				),
			),
			// Add detailed datatable section
			html.Div(
				html.Class("card-body"),
				html.H6(html.Class("text-primary mb-3"),
					html.I(html.Class("fas fa-table me-2")),
					gomponents.Text(fmt.Sprintf("All %d Missing Episodes", totalMissing))),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-striped table-hover"),
						html.ID("missing-episodes-table"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Series")),
								html.Th(gomponents.Text("Season")),
								html.Th(gomponents.Text("Episode")),
								html.Th(gomponents.Text("Title")),
								html.Th(gomponents.Text("Status")),
								html.Th(gomponents.Text("Quality Profile")),
							),
						),
						html.TBody(gomponents.Group(tableRows)),
					),
				),
				// Add DataTables initialization using centralized function
				html.Script(gomponents.Text(`
					$(document).ready(function() {
						if (window.initDataTable) {
							window.initDataTable('#missing-episodes-table', {
								order: [[0, 'asc'], [1, 'asc'], [2, 'asc']],
								columnDefs: [
									{ orderable: true, targets: [0, 1, 2, 3, 4, 5] }
								],
								language: {
									search: "Filter episodes:",
									lengthMenu: "Show _MENU_ episodes per page",
									info: "Showing _START_ to _END_ of _TOTAL_ missing episodes"
								}
							});
						}
					});
				`)),
			),
		)
	} else {
		// Standard result display
		resultContent = html.Div(
			html.Class(alertClass),
			html.Div(
				html.Class("card-header border-0"),
				html.Style("background: linear-gradient(135deg, "+func() string {
					if totalMissing == 0 {
						return "#d4edda 0%, #c3e6cb 100%"
					} else if totalMissing > 20 {
						return "#fff3cd 0%, #ffeaa7 100%"
					}

					return "#d1ecf1 0%, #bee5eb 100%"
				}()+"); border-radius: 15px 15px 0 0;"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(html.Class("badge "+func() string {
						if totalMissing == 0 {
							return "bg-success"
						} else if totalMissing > 20 {
							return "bg-warning"
						}

						return "bg-info"
					}()+" me-3"), html.I(html.Class("fas fa-search me-1")), gomponents.Text("Search")),
					html.H5(html.Class("card-title mb-0 "+func() string {
						if totalMissing == 0 {
							return "text-success"
						} else if totalMissing > 20 {
							return "text-warning"
						}

						return "text-info"
					}()+" fw-bold"), gomponents.Text(message)),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(results)),
				),
			),
		)
	}

	c.String(http.StatusOK, renderComponentToString(resultContent))
}

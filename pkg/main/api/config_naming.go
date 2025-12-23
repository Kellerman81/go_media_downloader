package api

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderNamingTestPage renders a page for testing naming conventions.
func renderNamingTestPage(csrfToken string) gomponents.Node {
	media := config.GetSettingsMediaAll()

	lists := make([]string, 0, len(media.Movies)+len(media.Series))
	for i := range media.Movies {
		lists = append(lists, media.Movies[i].NamePrefix)
	}

	for i := range media.Series {
		lists = append(lists, media.Series[i].NamePrefix)
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
					html.I(html.Class("fa-solid fa-edit header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Naming Convention Test")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Test how your naming templates will format movie and episode filenames. This tool helps you preview the generated folder and file names before applying them to your media library.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("namingTestForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Media Configuration"),
					),

					renderFormGroup("naming", map[string]string{
						"MediaType": "Select whether you're testing movie or TV series naming",
					}, map[string]string{
						"MediaType": "Media Type",
					}, "MediaType", "select", "movie", map[string][]string{
						"options": {"movie", "serie"},
					}),

					renderFormGroup("naming", map[string]string{
						"ConfigKey": "Select the media configuration to use for naming",
					}, map[string]string{
						"ConfigKey": "Media Config",
					}, "ConfigKey", "select", "", map[string][]string{
						"options": lists,
					}),

					renderFormGroup("naming", map[string]string{
						"FilePath": fmt.Sprintf(
							"Example file path to test (e.g., '/downloads/Movie.%d.1080p.BluRay.mkv')",
							time.Now().Year(),
						),
					}, map[string]string{
						"FilePath": "File Path",
					}, "FilePath", "text", "/downloads/The.Matrix.1999.1080p.BluRay.x264-RARBG.mkv", nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Media Selection")),
					html.P(
						html.Class("text-muted"),
						gomponents.Text(
							"Select existing media from your database to test naming conventions:",
						),
					),

					html.Div(
						html.ID("movieFields"),
						html.Style("display: block;"),
						renderFormGroup("naming", map[string]string{
							"MovieID": "Enter the database ID of an existing movie",
						}, map[string]string{
							"MovieID": "Movie ID",
						}, "MovieID", "number", "1", nil),
					),

					html.Div(
						html.ID("serieFields"),
						html.Style("display: none;"),
						renderFormGroup("naming", map[string]string{
							"SerieID": "Enter the database ID of an existing TV series",
						}, map[string]string{
							"SerieID": "Series ID",
						}, "SerieID", "number", "1", nil),
					),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Test Naming"),
					html.Type("button"),
					hx.Target("#namingResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/namingtest"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#namingTestForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('namingTestForm').reset(); document.getElementById('namingResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("namingResults"),
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
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Usage"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Usage Instructions"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Follow these steps to test your naming templates"),
				),
				html.Ol(
					html.Class("mb-3 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("1. Select the media type (Movie or TV Series)"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"2. Choose the media configuration that contains your naming templates",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("3. Enter a sample file path to test"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"4. Provide the database ID of an existing movie or series",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"5. Click 'Test Naming' to see how your templates will format the names",
						),
					),
				),

				html.Div(
					html.Class("alert alert-light border-0 mt-3 mb-0"),
					html.Style(
						"background-color: rgba(13, 110, 253, 0.1); border-radius: 8px; padding: 0.75rem 1rem;",
					),
					html.Div(
						html.Class("d-flex align-items-start"),
						html.I(
							html.Class("fas fa-lightbulb me-2 mt-1"),
							html.Style("color: #0d6efd; font-size: 0.9rem;"),
						),
						html.Div(
							html.Strong(html.Style("color: #0d6efd;"), gomponents.Text("Note: ")),
							gomponents.Text(
								"The movie or series ID must exist in your database. You can find these IDs in the database management interface.",
							),
						),
					),
				),
			),
		),

		// JavaScript for toggling fields
		// Simplified JavaScript for Naming - CSS classes handle field visibility
		html.Script(gomponents.Raw(`
			// No JavaScript needed - CSS classes handle movie/series field visibility
		`)),
	)
}

// HandleNamingTest handles naming convention test requests.
func HandleNamingTest(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(200, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	mediaType := c.PostForm("naming_MediaType")
	configKey := c.PostForm("naming_ConfigKey")
	filePath := c.PostForm("naming_FilePath")

	if mediaType == "" || configKey == "" || filePath == "" {
		c.String(200, renderAlert("Please fill in all required fields", "warning"))
		return
	}

	// Get the appropriate ID based on media type
	var (
		movieID, serieID int
		err              error
	)

	if mediaType == "movie" {
		movieIDStr := c.PostForm("naming_MovieID")
		if movieIDStr == "" {
			c.String(200, renderAlert("Please enter a Movie ID", "warning"))
			return
		}

		movieID, err = strconv.Atoi(movieIDStr)
		if err != nil {
			c.String(200, renderAlert("Invalid Movie ID: "+err.Error(), "danger"))
			return
		}
	} else {
		serieIDStr := c.PostForm("naming_SerieID")
		if serieIDStr == "" {
			c.String(200, renderAlert("Please enter a Series ID", "warning"))
			return
		}

		serieID, err = strconv.Atoi(serieIDStr)
		if err != nil {
			c.String(200, renderAlert("Invalid Series ID: "+err.Error(), "danger"))
			return
		}
	}

	cfg := apiNameInput{
		CfgMedia:  configKey,
		GroupType: mediaType,
		FilePath:  filePath,
		MovieID:   movieID,
		SerieID:   serieID,
	}
	if cfg.GroupType == "movie" {
		movie, _ := database.GetMovies(
			database.Querywithargs{Where: logger.FilterByID},
			cfg.MovieID,
		)
		if movie == nil {
			c.String(200, renderAlert("Movie not found", "danger"))
			return
		}

		cfgp := config.GetSettingsMedia(cfg.CfgMedia)
		if cfgp == nil {
			c.String(200, renderAlert("Movie Config not found", "danger"))
			return
		}

		s := structure.NewStructure(
			cfgp,
			config.GetSettingsMedia(cfg.CfgMedia).DataImport[0].TemplatePath,
			config.GetSettingsMedia(cfg.CfgMedia).Data[0].TemplatePath, false, false, 0)
		// defer s.Close()
		if s == nil {
			c.String(200, renderAlert("Movie Structure failed", "danger"))
			return
		}

		to := filepath.Dir(cfg.FilePath)

		var orgadata2 structure.Organizerdata

		orgadata2.Videofile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = movie.Rootpath

		m := parser.ParseFile(
			cfg.FilePath,
			true,
			true,
			cfgp,
			cfgp.GetMediaListsEntryListID(movie.Listname),
		)
		if m == nil {
			c.String(200, renderAlert("Movie Parse failed", "danger"))
			return
		}

		if m.ListID == -1 {
			c.String(200, renderAlert("Movie List not found", "danger"))
			return
		}

		orgadata2.Listid = m.ListID
		s.ParseFileAdditional(&orgadata2, m, 0, false, false, s.Cfgp.Lists[m.ListID].CfgQuality)

		s.GenerateNamingTemplate(&orgadata2, m, &movie.DbmovieID)
		c.String(
			200,
			renderNamingResults(
				map[string]any{
					"foldername": orgadata2.Foldername,
					"filename":   orgadata2.Filename,
					"m":          m,
				},
				cfg.GroupType,
				cfg.CfgMedia,
				cfg.FilePath,
				cfg.MovieID,
				cfg.SerieID,
			),
		)
	} else {
		series, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, cfg.SerieID)
		if series == nil {
			c.String(200, renderAlert("Series not found", "danger"))
			return
		}
		// defer logger.ClearVar(&series)
		cfgp := config.GetSettingsMedia(cfg.CfgMedia)
		if cfgp == nil {
			c.String(200, renderAlert("Series Config not found", "danger"))
			return
		}

		s := structure.NewStructure(
			cfgp,
			config.GetSettingsMedia(cfg.CfgMedia).DataImport[0].TemplatePath,
			config.GetSettingsMedia(cfg.CfgMedia).Data[0].TemplatePath,
			false, false, 0,
		)
		if s == nil {
			c.String(200, renderAlert("Series Structure failed", "danger"))
			return
		}
		// defer s.Close()
		to := filepath.Dir(cfg.FilePath)

		var orgadata2 structure.Organizerdata

		orgadata2.Videofile = cfg.FilePath
		orgadata2.Folder = to
		orgadata2.Rootpath = series.Rootpath

		m := parser.ParseFile(cfg.FilePath, true, true, cfgp, cfgp.GetMediaListsEntryListID(series.Listname))
		if m == nil {
			c.String(200, renderAlert("Series Parse failed", "danger"))
			return
		}

		if m.ListID == -1 {
			c.String(200, renderAlert("Series List not found", "danger"))
			return
		}

		orgadata2.Listid = m.ListID
		s.ParseFileAdditional(&orgadata2, m, 0, false, false, s.Cfgp.Lists[m.ListID].CfgQuality)

		m.SerieID = series.ID
		m.DbserieID = series.DbserieID
		s.GetSeriesEpisodes(&orgadata2, m, true, s.Cfgp.Lists[orgadata2.Listid].CfgQuality)

		var firstepiid uint
		for _, entry := range m.Episodes {
			firstepiid = entry.Num1
			break
		}

		s.GenerateNamingTemplate(&orgadata2, m, &firstepiid)
		c.String(200, renderNamingResults(map[string]any{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m}, cfg.GroupType, cfg.CfgMedia, cfg.FilePath, cfg.MovieID, cfg.SerieID))
	}
}

// renderNamingResults renders the naming test results.
func renderNamingResults(
	result map[string]any,
	mediaType, configKey, filePath string,
	movieID, serieID int,
) string {
	// Check if there's an error in the result
	if errMsg, ok := result["error"]; ok {
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
							html.I(html.Class("fas fa-exclamation-triangle me-1")),
							gomponents.Text("Error"),
						),
						html.H5(
							html.Class("card-title mb-0 text-warning fw-bold"),
							gomponents.Text("API Integration Limitation"),
						),
					),
				),
				html.Div(
					html.Class("card-body"),
					html.P(
						html.Class("card-text text-muted mb-3"),
						gomponents.Text(fmt.Sprintf("%v", errMsg)),
					),
					html.P(
						html.Class("card-text text-muted mb-3"),
						gomponents.Text(fmt.Sprintf("Note: %v", result["note"])),
					),
					html.Details(
						html.Summary(gomponents.Text("Technical Details")),
						html.P(gomponents.Text("Payload that would be sent to /api/naming:")),
						html.Pre(
							html.Class("bg-light p-3 mt-2"),
							html.Style("border-radius: 6px;"),
							html.Code(gomponents.Text(fmt.Sprintf("%v", result["payload"]))),
						),
					),
				),
			),
		)
	}

	// Extract results (when properly implemented)
	foldername := ""
	filename := ""

	if fn, ok := result["foldername"]; ok {
		foldername = fmt.Sprintf("%v", fn)
	}

	if fn, ok := result["filename"]; ok {
		filename = fmt.Sprintf("%v", fn)
	}

	resultRows := []gomponents.Node{
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Input Parameters:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Media Type:")), html.Td(gomponents.Text(mediaType))),
		html.Tr(html.Td(gomponents.Text("Config Used:")), html.Td(gomponents.Text(configKey))),
		html.Tr(html.Td(gomponents.Text("File Path:")), html.Td(gomponents.Text(filePath))),
		func() gomponents.Node {
			if mediaType == "movie" {
				return html.Tr(
					html.Td(gomponents.Text("Movie ID:")),
					html.Td(gomponents.Text(fmt.Sprintf("%d", movieID))),
				)
			}

			return html.Tr(
				html.Td(gomponents.Text("Series ID:")),
				html.Td(gomponents.Text(fmt.Sprintf("%d", serieID))),
			)
		}(),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),

		html.Tr(
			html.Td(html.Strong(gomponents.Text("Generated Names:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Folder Name:")), html.Td(gomponents.Text(func() string {
			if foldername != "" {
				return foldername
			}
			return "No folder name generated"
		}()))),
		html.Tr(html.Td(gomponents.Text("File Name:")), html.Td(gomponents.Text(func() string {
			if filename != "" {
				return filename
			}
			return "No filename generated"
		}()))),

		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Full Path Preview:"))),
			html.Td(gomponents.Text("")),
		),
		html.Tr(html.Td(gomponents.Text("Complete Path:")), html.Td(gomponents.Text(func() string {
			if foldername != "" && filename != "" {
				return foldername + "/" + filename
			}

			if foldername != "" {
				return foldername + "/[filename not generated]"
			}

			if filename != "" {
				return "[folder not generated]/" + filename
			}

			return "No path generated"
		}()))),
	}

	// Add parsed information if available
	if m, ok := result["m"]; ok {
		if parseInfo, ok := m.(*database.ParseInfo); ok {
			resultRows = append(
				resultRows,
				html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
				html.Tr(
					html.Td(html.Strong(gomponents.Text("Parsed Information:"))),
					html.Td(gomponents.Text("")),
				),
				html.Tr(
					html.Td(gomponents.Text("Parsed Title:")),
					html.Td(gomponents.Text(parseInfo.Title)),
				),
				html.Tr(
					html.Td(gomponents.Text("Parsed Year:")),
					html.Td(gomponents.Text(fmt.Sprintf("%d", parseInfo.Year))),
				),
				html.Tr(
					html.Td(gomponents.Text("Parsed Quality:")),
					html.Td(gomponents.Text(parseInfo.Quality)),
				),
				html.Tr(
					html.Td(gomponents.Text("Parsed Resolution:")),
					html.Td(gomponents.Text(parseInfo.Resolution)),
				),
				html.Tr(
					html.Td(gomponents.Text("Parsed Codec:")),
					html.Td(gomponents.Text(parseInfo.Codec)),
				),
				html.Tr(
					html.Td(gomponents.Text("Parsed Audio:")),
					html.Td(gomponents.Text(parseInfo.Audio)),
				),
			)

			// Add episode/season info if applicable
			if parseInfo.Season > 0 || parseInfo.Episode > 0 {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Season:")),
						html.Td(gomponents.Text(fmt.Sprintf("%d", parseInfo.Season))),
					),
					html.Tr(
						html.Td(gomponents.Text("Episode:")),
						html.Td(gomponents.Text(fmt.Sprintf("%d", parseInfo.Episode))),
					),
				)
			}

			// Add additional useful fields
			if parseInfo.Date != "" {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Release Date:")),
						html.Td(gomponents.Text(parseInfo.Date)),
					),
				)
			}

			if len(parseInfo.Languages) > 0 {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Languages:")),
						html.Td(gomponents.Text(strings.Join(parseInfo.Languages, ", "))),
					),
				)
			}

			if parseInfo.Extended {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Extended Cut:")),
						html.Td(gomponents.Text("Yes")),
					),
				)
			}

			if parseInfo.Proper {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Proper Release:")),
						html.Td(gomponents.Text("Yes")),
					),
				)
			}

			if parseInfo.Repack {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Repack Release:")),
						html.Td(gomponents.Text("Yes")),
					),
				)
			}

			if parseInfo.Runtime > 0 {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Runtime:")),
						html.Td(gomponents.Text(fmt.Sprintf("%d min", parseInfo.Runtime))),
					),
				)
			}

			if parseInfo.Width > 0 && parseInfo.Height > 0 {
				resultRows = append(
					resultRows,
					html.Tr(
						html.Td(gomponents.Text("Dimensions:")),
						html.Td(
							gomponents.Text(
								fmt.Sprintf("%dx%d", parseInfo.Width, parseInfo.Height),
							),
						),
					),
				)
			}
		}
	}

	var alertClass, message, icon, color string
	if foldername != "" && filename != "" {
		alertClass = "card border-0 shadow-sm border-success mb-4"
		message = "Naming Test Completed Successfully"
		icon = "fas fa-check-circle"
		color = "#28a745"
	} else if foldername != "" || filename != "" {
		alertClass = "card border-0 shadow-sm border-warning mb-4"
		message = "Naming Test Partially Successful"
		icon = "fas fa-exclamation-circle"
		color = "#ffc107"
	} else {
		alertClass = "card border-0 shadow-sm border-danger mb-4"
		message = "Naming Test Failed"
		icon = "fas fa-times-circle"
		color = "#dc3545"
	}

	_ = color

	results := html.Div(
		html.Class(alertClass),
		html.Div(
			html.Class("card-header border-0"),
			html.Style("background: linear-gradient(135deg, "+func() string {
				if foldername != "" && filename != "" {
					return "#d4edda 0%, #c3e6cb 100%"
				} else if foldername != "" || filename != "" {
					return "#fff3cd 0%, #ffeaa7 100%"
				}

				return "#f8d7da 0%, #f5c6cb 100%"
			}()+"); border-radius: 15px 15px 0 0;"),
			html.Div(
				html.Class("d-flex align-items-center"),
				html.Span(html.Class("badge "+func() string {
					if foldername != "" && filename != "" {
						return "bg-success"
					} else if foldername != "" || filename != "" {
						return "bg-warning"
					}

					return "bg-danger"
				}()+" me-3"), html.I(html.Class(icon+" me-1")), gomponents.Text(func() string {
					if foldername != "" && filename != "" {
						return "Success"
					} else if foldername != "" || filename != "" {
						return "Partial"
					}

					return "Failed"
				}())),
				html.H5(html.Class("card-title mb-0 "+func() string {
					if foldername != "" && filename != "" {
						return "text-success"
					} else if foldername != "" || filename != "" {
						return "text-warning"
					}

					return "text-danger"
				}()+" fw-bold"), gomponents.Text(message)),
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

		// Add helpful information about naming results
		html.Div(
			html.Class("mt-3 card border-0 shadow-sm border-info mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d1ecf1 0%, #bee5eb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-info me-3"),
						html.I(html.Class("fas fa-info-circle me-1")),
						gomponents.Text("Information"),
					),
					html.H5(
						html.Class("card-title mb-0 text-info fw-bold"),
						gomponents.Text("Naming Test Information"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					gomponents.Text(
						"This page tests your naming conventions using real file parsing and naming generation:",
					),
				),
				html.Ul(
					html.Li(
						gomponents.Text(
							"✅ File Parsing: Real parsing using your quality and regex configurations",
						),
					),
					html.Li(
						gomponents.Text(
							"📁 Folder Naming: Generated using your folder naming templates",
						),
					),
					html.Li(
						gomponents.Text(
							"📄 File Naming: Generated using your file naming templates",
						),
					),
					html.Li(
						gomponents.Text(
							"🔍 Quality Analysis: Shows parsed quality, resolution, codec, and audio information",
						),
					),
					html.Li(
						gomponents.Text(
							"📊 Media Details: Displays season/episode info for TV series",
						),
					),
				),
				func() gomponents.Node {
					if foldername == "" && filename == "" {
						return html.P(
							html.Class("mt-2 text-warning"),
							html.Strong(gomponents.Text("Note: ")),
							gomponents.Text(
								"No names were generated. Check your naming templates and ensure the media exists in your database.",
							),
						)
					} else if foldername == "" {
						return html.P(html.Class("mt-2 text-warning"), html.Strong(gomponents.Text("Note: ")), gomponents.Text("Folder name not generated. Check your folder naming template configuration."))
					} else if filename == "" {
						return html.P(html.Class("mt-2 text-warning"), html.Strong(gomponents.Text("Note: ")), gomponents.Text("File name not generated. Check your file naming template configuration."))
					}

					return html.P(
						html.Class("mt-2 text-success"),
						html.Strong(gomponents.Text("Success: ")),
						gomponents.Text("Both folder and file names generated successfully!"),
					)
				}(),
			),
		),
	)

	return renderComponentToString(results)
}

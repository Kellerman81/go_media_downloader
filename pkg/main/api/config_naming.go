package api

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/gin-gonic/gin"
	. "maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	. "maragu.dev/gomponents/html"
)

// renderNamingTestPage renders a page for testing naming conventions
func renderNamingTestPage(csrfToken string) Node {
	media := config.GetSettingsMediaAll()
	lists := make([]string, 0, len(media.Movies)+len(media.Series))
	for i := range media.Movies {
		lists = append(lists, media.Movies[i].NamePrefix)
	}
	for i := range media.Series {
		lists = append(lists, media.Series[i].NamePrefix)
	}
	return Div(
		Class("config-section"),
		H3(Text("Naming Convention Test")),
		P(Text("Test how your naming templates will format movie and episode filenames. This tool helps you preview the generated folder and file names before applying them to your media library.")),

		Form(
			Class("config-form"),
			ID("namingTestForm"),

			Div(
				Class("row"),
				Div(
					Class("col-md-6"),
					H5(Text("Media Configuration")),

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
						"FilePath": "Example file path to test (e.g., '/downloads/Movie.2023.1080p.BluRay.mkv')",
					}, map[string]string{
						"FilePath": "File Path",
					}, "FilePath", "text", "/downloads/The.Matrix.1999.1080p.BluRay.x264-RARBG.mkv", nil),
				),

				Div(
					Class("col-md-6"),
					H5(Text("Media Selection")),
					P(Class("text-muted"), Text("Select existing media from your database to test naming conventions:")),

					Div(
						ID("movieFields"),
						Style("display: block;"),
						renderFormGroup("naming", map[string]string{
							"MovieID": "Enter the database ID of an existing movie",
						}, map[string]string{
							"MovieID": "Movie ID",
						}, "MovieID", "number", "1", nil),
					),

					Div(
						ID("serieFields"),
						Style("display: none;"),
						renderFormGroup("naming", map[string]string{
							"SerieID": "Enter the database ID of an existing TV series",
						}, map[string]string{
							"SerieID": "Series ID",
						}, "SerieID", "number", "1", nil),
					),
				),
			),

			Div(
				Class("form-group submit-group"),
				Button(
					Class("btn btn-primary"),
					Text("Test Naming"),
					Type("button"),
					hx.Target("#namingResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/namingtest"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#namingTestForm"),
				),
				Button(
					Type("button"),
					Class("btn btn-secondary ml-2"),
					Attr("onclick", "document.getElementById('namingTestForm').reset(); document.getElementById('namingResults').innerHTML = '';"),
					Text("Reset"),
				),
			),
		),

		Div(
			ID("namingResults"),
			Class("mt-4"),
			Style("min-height: 50px;"),
		),

		// Instructions
		Div(
			Class("mt-4 alert alert-info"),
			H5(Text("Usage Instructions:")),
			Ol(
				Li(Text("Select the media type (Movie or TV Series)")),
				Li(Text("Choose the media configuration that contains your naming templates")),
				Li(Text("Enter a sample file path to test")),
				Li(Text("Provide the database ID of an existing movie or series")),
				Li(Text("Click 'Test Naming' to see how your templates will format the names")),
			),
			P(
				Class("mt-2"),
				Strong(Text("Note: ")),
				Text("The movie or series ID must exist in your database. You can find these IDs in the database management interface."),
			),
		),

		// JavaScript for toggling fields
		Script(Raw(`
			document.addEventListener('DOMContentLoaded', function() {
				const mediaTypeSelect = document.querySelector('select[name="naming_MediaType"]');
				const movieFields = document.getElementById('movieFields');
				const serieFields = document.getElementById('serieFields');
				
				function toggleFields() {
					const mediaType = mediaTypeSelect.value;
					if (mediaType === 'movie') {
						movieFields.style.display = 'block';
						serieFields.style.display = 'none';
					} else {
						movieFields.style.display = 'none';
						serieFields.style.display = 'block';
					}
				}
				
				mediaTypeSelect.addEventListener('change', toggleFields);
				toggleFields(); // Initial setup
			});
		`)),
	)
}

// HandleNamingTest handles naming convention test requests
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
	var movieID, serieID int
	var err error

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
		c.String(200, renderNamingResults(map[string]any{"foldername": orgadata2.Foldername, "filename": orgadata2.Filename, "m": m}, cfg.GroupType, cfg.CfgMedia, cfg.FilePath, cfg.MovieID, cfg.SerieID))
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

// renderNamingResults renders the naming test results
func renderNamingResults(result map[string]any, mediaType, configKey, filePath string, movieID, serieID int) string {
	// Check if there's an error in the result
	if errMsg, ok := result["error"]; ok {
		return renderComponentToString(
			Div(
				Class("alert alert-warning"),
				H5(Text("API Integration Limitation")),
				P(Text(fmt.Sprintf("%v", errMsg))),
				P(Text(fmt.Sprintf("Note: %v", result["note"]))),
				Details(
					Summary(Text("Technical Details")),
					P(Text("Payload that would be sent to /api/naming:")),
					Pre(
						Class("bg-light p-3"),
						Code(Text(fmt.Sprintf("%v", result["payload"]))),
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

	resultRows := []Node{
		Tr(Td(Strong(Text("Input Parameters:"))), Td(Text(""))),
		Tr(Td(Text("Media Type:")), Td(Text(mediaType))),
		Tr(Td(Text("Config Used:")), Td(Text(configKey))),
		Tr(Td(Text("File Path:")), Td(Text(filePath))),
		func() Node {
			if mediaType == "movie" {
				return Tr(Td(Text("Movie ID:")), Td(Text(fmt.Sprintf("%d", movieID))))
			}
			return Tr(Td(Text("Series ID:")), Td(Text(fmt.Sprintf("%d", serieID))))
		}(),
		Tr(Td(Attr("colspan", "2"), Hr())),

		Tr(Td(Strong(Text("Generated Names:"))), Td(Text(""))),
		Tr(Td(Text("Folder Name:")), Td(Text(func() string {
			if foldername != "" {
				return foldername
			}
			return "No folder name generated"
		}()))),
		Tr(Td(Text("File Name:")), Td(Text(func() string {
			if filename != "" {
				return filename
			}
			return "No filename generated"
		}()))),

		Tr(Td(Attr("colspan", "2"), Hr())),
		Tr(Td(Strong(Text("Full Path Preview:"))), Td(Text(""))),
		Tr(Td(Text("Complete Path:")), Td(Text(func() string {
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
			resultRows = append(resultRows,
				Tr(Td(Attr("colspan", "2"), Hr())),
				Tr(Td(Strong(Text("Parsed Information:"))), Td(Text(""))),
				Tr(Td(Text("Parsed Title:")), Td(Text(parseInfo.Title))),
				Tr(Td(Text("Parsed Year:")), Td(Text(fmt.Sprintf("%d", parseInfo.Year)))),
				Tr(Td(Text("Parsed Quality:")), Td(Text(parseInfo.Quality))),
				Tr(Td(Text("Parsed Resolution:")), Td(Text(parseInfo.Resolution))),
				Tr(Td(Text("Parsed Codec:")), Td(Text(parseInfo.Codec))),
				Tr(Td(Text("Parsed Audio:")), Td(Text(parseInfo.Audio))),
			)

			// Add episode/season info if applicable
			if parseInfo.Season > 0 || parseInfo.Episode > 0 {
				resultRows = append(resultRows,
					Tr(Td(Text("Season:")), Td(Text(fmt.Sprintf("%d", parseInfo.Season)))),
					Tr(Td(Text("Episode:")), Td(Text(fmt.Sprintf("%d", parseInfo.Episode)))),
				)
			}

			// Add additional useful fields
			if parseInfo.Date != "" {
				resultRows = append(resultRows, Tr(Td(Text("Release Date:")), Td(Text(parseInfo.Date))))
			}
			if len(parseInfo.Languages) > 0 {
				resultRows = append(resultRows, Tr(Td(Text("Languages:")), Td(Text(strings.Join(parseInfo.Languages, ", ")))))
			}
			if parseInfo.Extended {
				resultRows = append(resultRows, Tr(Td(Text("Extended Cut:")), Td(Text("Yes"))))
			}
			if parseInfo.Proper {
				resultRows = append(resultRows, Tr(Td(Text("Proper Release:")), Td(Text("Yes"))))
			}
			if parseInfo.Repack {
				resultRows = append(resultRows, Tr(Td(Text("Repack Release:")), Td(Text("Yes"))))
			}
			if parseInfo.Runtime > 0 {
				resultRows = append(resultRows, Tr(Td(Text("Runtime:")), Td(Text(fmt.Sprintf("%d min", parseInfo.Runtime)))))
			}
			if parseInfo.Width > 0 && parseInfo.Height > 0 {
				resultRows = append(resultRows, Tr(Td(Text("Dimensions:")), Td(Text(fmt.Sprintf("%dx%d", parseInfo.Width, parseInfo.Height)))))
			}
		}
	}

	var alertClass, message string
	if foldername != "" && filename != "" {
		alertClass = "alert-success"
		message = "Naming Test Completed Successfully"
	} else if foldername != "" || filename != "" {
		alertClass = "alert-warning"
		message = "Naming Test Partially Successful"
	} else {
		alertClass = "alert-danger"
		message = "Naming Test Failed"
	}

	results := Div(
		Class(alertClass),
		H5(Text(message)),
		Table(
			Class("table table-striped table-sm"),
			TBody(Group(resultRows)),
		),

		// Add helpful information about naming results
		Div(
			Class("mt-3 alert-info"),
			H6(Text("Naming Test Information")),
			P(Text("This page tests your naming conventions using real file parsing and naming generation:")),
			Ul(
				Li(Text("✅ File Parsing: Real parsing using your quality and regex configurations")),
				Li(Text("📁 Folder Naming: Generated using your folder naming templates")),
				Li(Text("📄 File Naming: Generated using your file naming templates")),
				Li(Text("🔍 Quality Analysis: Shows parsed quality, resolution, codec, and audio information")),
				Li(Text("📊 Media Details: Displays season/episode info for TV series")),
			),
			func() Node {
				if foldername == "" && filename == "" {
					return P(Class("mt-2 text-warning"), Strong(Text("Note: ")), Text("No names were generated. Check your naming templates and ensure the media exists in your database."))
				} else if foldername == "" {
					return P(Class("mt-2 text-warning"), Strong(Text("Note: ")), Text("Folder name not generated. Check your folder naming template configuration."))
				} else if filename == "" {
					return P(Class("mt-2 text-warning"), Strong(Text("Note: ")), Text("File name not generated. Check your file naming template configuration."))
				}
				return P(Class("mt-2 text-success"), Strong(Text("Success: ")), Text("Both folder and file names generated successfully!"))
			}(),
		),
	)

	return renderComponentToString(results)
}

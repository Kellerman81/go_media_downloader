package api

import (
	"fmt"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderTestParsePage renders a page for testing string parsing.
func renderTestParsePage(csrfToken string) gomponents.Node {
	media := config.GetSettingsMediaAll()

	lists := make(
		[]string,
		0,
		len(media.Movies)+len(media.Series)+len(media.Music)+len(media.Books)+len(media.AudioBooks),
	)
	for i := range media.Movies {
		lists = append(lists, media.Movies[i].NamePrefix)
	}

	for i := range media.Series {
		lists = append(lists, media.Series[i].NamePrefix)
	}

	for i := range media.Music {
		lists = append(lists, media.Music[i].NamePrefix)
	}

	for i := range media.Books {
		lists = append(lists, media.Books[i].NamePrefix)
	}

	for i := range media.AudioBooks {
		lists = append(lists, media.AudioBooks[i].NamePrefix)
	}

	qualitycfg := config.GetSettingsQualityAll()

	qualities := make([]string, 0, len(qualitycfg))
	for i := range qualitycfg {
		qualities = append(qualities, qualitycfg[i].Name)
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-flask",
			"String Parse Test",
			"Test the file parser with different filename patterns. This tool helps you understand how the parser extracts metadata from filenames.",
		),

		html.Form(
			html.Class("config-form"),
			html.ID("parseTestForm"),

			renderFormGroup("testparse", map[string]string{
				"Filename": "Enter a filename to parse (e.g., 'The.Matrix.1999.1080p.BluRay.x264-RARBG')",
			}, map[string]string{
				"Filename": "Filename",
			}, "Filename", "text", "", nil),

			renderFormGroup("testparse", map[string]string{
				"ConfigKey": "Select the media configuration to use for parsing",
			}, map[string]string{
				"ConfigKey": "Media Config",
			}, "ConfigKey", "select", "", map[string][]string{
				"options": lists,
			}),

			renderFormGroup("testparse", map[string]string{
				"QualityKey": "Select the quality configuration to use",
			}, map[string]string{
				"QualityKey": "Quality Config",
			}, "QualityKey", "select", "", map[string][]string{
				"options": qualities,
			}),

			mediaConfigQualityScript("testparse_ConfigKey", "testparse_QualityKey"),

			renderFormGroup("testparse", map[string]string{
				"UsePath": "Parse as full path instead of just filename",
			}, map[string]string{
				"UsePath": "Use Path",
			}, "UsePath", "checkbox", false, nil),

			renderFormGroup("testparse", map[string]string{
				"UseFolder": "Use folder information in parsing",
			}, map[string]string{
				"UseFolder": "Use Folder",
			}, "UseFolder", "checkbox", false, nil),

			renderHTMXSubmitButton(
				"Parse",
				"parseResults",
				"/api/admin/testparse",
				"parseTestForm",
				csrfToken,
			),
			html.Div(
				html.Class("form-group submit-group"),
				createButton(
					"Reset",
					"button",
					"btn btn-secondary ml-2",
					gomponents.Attr(
						"onclick",
						"document.getElementById('parseTestForm').reset(); document.getElementById('testparse_ConfigKey').dispatchEvent(new Event('change')); document.getElementById('parseResults').innerHTML = '';",
					),
				),
			),
		),

		html.Div(
			html.ID("parseResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// renderParseResults renders the parsing results in a formatted table.
func renderParseResults(
	m *database.ParseInfo,
	originalFilename, configKey, qualityKey string,
) string {
	resultRows := []gomponents.Node{
		// Header information
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Original Filename:"))),
			html.Td(gomponents.Text(originalFilename)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Config Used:"))),
			html.Td(gomponents.Text(configKey)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Quality Used:"))),
			html.Td(gomponents.Text(qualityKey)),
		),
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),

		// Basic media information
		html.Tr(html.Td(html.Strong(gomponents.Text("Title:"))), html.Td(gomponents.Text(m.Title))),
		html.Tr(html.Td(html.Strong(gomponents.Text("File:"))), html.Td(gomponents.Text(m.File))),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Year:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Year))),
		),
		html.Tr(html.Td(html.Strong(gomponents.Text("Date:"))), html.Td(gomponents.Text(m.Date))),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Identifier:"))),
			html.Td(gomponents.Text(m.Identifier)),
		),

		// Episode/Season information
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Season:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Season))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Season String:"))),
			html.Td(gomponents.Text(m.SeasonStr)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Episode:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Episode))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Episode String:"))),
			html.Td(gomponents.Text(m.EpisodeStr)),
		),

		// Quality and technical information
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Quality:"))),
			html.Td(gomponents.Text(m.Quality)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Resolution:"))),
			html.Td(gomponents.Text(m.Resolution)),
		),
		html.Tr(html.Td(html.Strong(gomponents.Text("Codec:"))), html.Td(gomponents.Text(m.Codec))),
		html.Tr(html.Td(html.Strong(gomponents.Text("Audio:"))), html.Td(gomponents.Text(m.Audio))),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Width x Height:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d x %d", m.Width, m.Height))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Runtime:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d min", m.Runtime))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Languages:"))),
			html.Td(gomponents.Text(strings.Join(m.Languages, ", "))),
		),

		// Flags and properties
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Extended:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%t", m.Extended))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Proper:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%t", m.Proper))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Repack:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%t", m.Repack))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Priority:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.Priority))),
		),

		// External IDs
		html.Tr(
			html.Td(html.Strong(gomponents.Text("IMDB ID:"))),
			html.Td(gomponents.Text(m.Imdb)),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("TVDB ID:"))),
			html.Td(gomponents.Text(m.Tvdb)),
		),

		// Internal information
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("List ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.ListID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("First IDX:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.FirstIDX))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("First Year IDX:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.FirstYearIDX))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Temp ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.TempID))),
		),

		// Database IDs
		html.Tr(html.Td(gomponents.Attr("colspan", "2"), html.Hr())),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Quality ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.QualityID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Resolution ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.ResolutionID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Codec ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.CodecID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Audio ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.AudioID))),
		),

		// Movie IDs
		html.Tr(
			html.Td(html.Strong(gomponents.Text("DB Movie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.DbmovieID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Movie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.MovieID))),
		),

		// Series IDs
		html.Tr(
			html.Td(html.Strong(gomponents.Text("DB Serie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.DbserieID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("DB Serie Episode ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.DbserieEpisodeID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Serie ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.SerieID))),
		),
		html.Tr(
			html.Td(html.Strong(gomponents.Text("Serie Episode ID:"))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", m.SerieEpisodeID))),
		),

		// Internal strings (if not empty)
		func() gomponents.Node {
			if m.Str != "" {
				return html.Tr(
					html.Td(html.Strong(gomponents.Text("Internal String:"))),
					html.Td(gomponents.Text(m.Str)),
				)
			}

			return gomponents.Text("")
		}(),
		func() gomponents.Node {
			if m.RuntimeStr != "" {
				return html.Tr(
					html.Td(html.Strong(gomponents.Text("Runtime String:"))),
					html.Td(gomponents.Text(m.RuntimeStr)),
				)
			}

			return gomponents.Text("")
		}(),
		func() gomponents.Node {
			if m.TempTitle != "" {
				return html.Tr(
					html.Td(html.Strong(gomponents.Text("Temp Title:"))),
					html.Td(gomponents.Text(m.TempTitle)),
				)
			}

			return gomponents.Text("")
		}(),
	}

	results := html.Div(
		html.Class("card border-0 shadow-sm border-success mb-4"),
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
						html.I(html.Class("fas fa-code me-1")),
						gomponents.Text("Parse"),
					),
					html.H5(
						html.Class("card-title mb-0 text-success fw-bold"),
						gomponents.Text("Parse Results"),
					),
				),
				html.Span(
					html.Class("badge bg-success"),
					gomponents.Text(fmt.Sprintf("%d", len(resultRows))),
				),
			),
		),
		html.Div(
			html.Class("card-body p-0"),
			func() gomponents.Node {
				if len(resultRows) == 0 {
					return html.Div(
						html.Class("text-center p-5"),
						html.I(
							html.Class("fas fa-code mb-3"),
							html.Style("font-size: 3rem; color: #28a745; opacity: 0.3;"),
						),
						html.H5(html.Class("text-muted mb-2"), gomponents.Text("No Parse Results")),
						html.P(
							html.Class("text-muted small mb-0"),
							gomponents.Text("No parsing operations have been performed yet."),
						),
					)
				}

				return html.Table(
					html.Class("table table-hover mb-0"),
					html.Style("background: transparent;"),
					html.TBody(gomponents.Group(resultRows)),
				)
			}(),
		),
	)

	return renderComponentToString(results)
}

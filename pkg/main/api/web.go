package api

import (
	"fmt"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// getFilterableFieldsForTable extracts filterable fields from goadmin models and creates filter inputs.
func getFilterableFieldsForTable(tableName string) []gomponents.Node {
	var filterFields []gomponents.Node

	// Define filterable fields based on goadmin model definitions
	filterDefinitions := map[string][]FilterFieldDef{
		"dbmovies": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "year", Label: "Year", Type: "number", Placeholder: "Year..."},
			{Field: "imdb_id", Label: "IMDB ID", Type: "text", Placeholder: "tt1234567..."},
			{
				Field:       "vote_average",
				Label:       "Vote Average",
				Type:        "number",
				Placeholder: "Rating...",
			},
			{Field: "runtime", Label: "Runtime", Type: "number", Placeholder: "Minutes..."},
			{
				Field:       "original_language",
				Label:       "Language",
				Type:        "text",
				Placeholder: "en, de, fr...",
			},
			{
				Field:        "adult",
				Label:        "Adult Content",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
			{Field: "status", Label: "Status", Type: "text", Placeholder: "Status..."},
		},
		"movies": {
			{Field: "title", Label: "Movie Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "imdb_id", Label: "IMDB ID", Type: "text", Placeholder: "Filter by IMDB ID..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality profile...",
			},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
			{Field: "rootpath", Label: "Root Path", Type: "text", Placeholder: "Path..."},
			{
				Field:        "quality_reached",
				Label:        "Quality Reached",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
			{
				Field:        "missing",
				Label:        "Missing",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
		},
		"dbseries": {
			{
				Field:       "seriename",
				Label:       "Series Name",
				Type:        "text",
				Placeholder: "Filter by series name...",
			},
			{Field: "status", Label: "Status", Type: "text", Placeholder: "Status..."},
			{Field: "genre", Label: "Genre", Type: "text", Placeholder: "Genre..."},
			{Field: "imdb_id", Label: "IMDB ID", Type: "text", Placeholder: "tt1234567..."},
			{Field: "thetvdb_id", Label: "TVDB ID", Type: "number", Placeholder: "TVDB ID..."},
		},
		"qualities": {
			{
				Field:        "type",
				Label:        "Type",
				Type:         "select",
				Options:      []string{"", "1", "2", "3", "4"},
				OptionLabels: []string{"All", "Resolution", "Quality", "Codec", "Audio"},
			},
			{Field: "name", Label: "Name", Type: "text", Placeholder: "Quality name..."},
			{Field: "regex", Label: "Regex", Type: "text", Placeholder: "Regular expression..."},
			{Field: "strings", Label: "Strings", Type: "text", Placeholder: "String patterns..."},
			{Field: "priority", Label: "Priority", Type: "number", Placeholder: "Priority..."},
			{
				Field:        "use_regex",
				Label:        "Use Regex",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
		},
		"series": {
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
			{Field: "rootpath", Label: "Root Path", Type: "text", Placeholder: "Path..."},
		},
		"movie_files": {
			{Field: "movie_id", Label: "Movie ID", Type: "text", Placeholder: "Movie ID..."},
			{Field: "location", Label: "Location", Type: "text", Placeholder: "File path..."},
			{Field: "filename", Label: "Filename", Type: "text", Placeholder: "Filename..."},
			{Field: "extension", Label: "Extension", Type: "text", Placeholder: "Extension..."},
			{Field: "quality_profile", Label: "Quality", Type: "text", Placeholder: "Quality..."},
		},
		"serie_episode_files": {
			{
				Field:       "serie_episode_id",
				Label:       "Episode ID",
				Type:        "text",
				Placeholder: "Episode ID...",
			},
			{Field: "location", Label: "Location", Type: "text", Placeholder: "File path..."},
			{Field: "filename", Label: "Filename", Type: "text", Placeholder: "Filename..."},
			{Field: "extension", Label: "Extension", Type: "text", Placeholder: "Extension..."},
		},
		"dbmovie_titles": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{
				Field:       "movie_title",
				Label:       "Movie Name",
				Type:        "text",
				Placeholder: "Filter by movie name...",
			},
			{Field: "region", Label: "Region", Type: "text", Placeholder: "Region..."},
		},
		"job_histories": {
			{Field: "job_type", Label: "Job Type", Type: "text", Placeholder: "Job type..."},
			{Field: "job_group", Label: "Job Group", Type: "text", Placeholder: "Job group..."},
			{
				Field:       "job_category",
				Label:       "Job Category",
				Type:        "text",
				Placeholder: "Job category...",
			},
			{
				Field:       "started",
				Label:       "Started After",
				Type:        "datetime-local",
				Placeholder: "Start date...",
			},
			{
				Field:       "ended",
				Label:       "Ended After",
				Type:        "datetime-local",
				Placeholder: "End date...",
			},
		},
		// Book tables
		"dbbooks": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "isbn", Label: "ISBN", Type: "text", Placeholder: "ISBN-10 or ISBN-13..."},
			{Field: "author", Label: "Author", Type: "text", Placeholder: "Author name..."},
			{Field: "publisher", Label: "Publisher", Type: "text", Placeholder: "Publisher..."},
			{Field: "language", Label: "Language", Type: "text", Placeholder: "Language..."},
			{Field: "year", Label: "Year", Type: "number", Placeholder: "Year..."},
		},
		"dbauthors": {
			{Field: "name", Label: "Name", Type: "text", Placeholder: "Author name..."},
			{
				Field:       "goodreads_id",
				Label:       "Goodreads ID",
				Type:        "text",
				Placeholder: "Goodreads ID...",
			},
		},
		"dbbook_titles": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "book_title", Label: "Book Title", Type: "text", Placeholder: "Book title..."},
			{Field: "region", Label: "Region", Type: "text", Placeholder: "Region..."},
		},
		"dbbook_series": {
			{Field: "name", Label: "Series Name", Type: "text", Placeholder: "Series name..."},
			{
				Field:       "goodreads_id",
				Label:       "Goodreads ID",
				Type:        "text",
				Placeholder: "Goodreads ID...",
			},
		},
		"books": {
			{Field: "title", Label: "Book Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "author", Label: "Author", Type: "text", Placeholder: "Author name..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality...",
			},
			{
				Field:        "missing",
				Label:        "Missing",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
			{
				Field:        "quality_reached",
				Label:        "Quality Reached",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
		},
		"book_files": {
			{Field: "book_id", Label: "Book ID", Type: "text", Placeholder: "Book ID..."},
			{Field: "title", Label: "Book Title", Type: "text", Placeholder: "Book title..."},
			{Field: "filename", Label: "Filename", Type: "text", Placeholder: "Filename..."},
			{Field: "location", Label: "Location", Type: "text", Placeholder: "File path..."},
		},
		"authors": {
			{Field: "name", Label: "Author Name", Type: "text", Placeholder: "Author name..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
		},
		"book_series": {
			{Field: "name", Label: "Series Name", Type: "text", Placeholder: "Series name..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
		},
		"book_file_unmatcheds": {
			{Field: "filepath", Label: "File Path", Type: "text", Placeholder: "File path..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
		},
		"book_histories": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Release title..."},
			{Field: "indexer", Label: "Indexer", Type: "text", Placeholder: "Indexer..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality...",
			},
		},
		// Audiobook tables
		"dbaudiobooks": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "asin", Label: "ASIN", Type: "text", Placeholder: "Amazon ASIN..."},
			{Field: "narrator", Label: "Narrator", Type: "text", Placeholder: "Narrator name..."},
			{Field: "publisher", Label: "Publisher", Type: "text", Placeholder: "Publisher..."},
			{Field: "language", Label: "Language", Type: "text", Placeholder: "Language..."},
			{Field: "year", Label: "Year", Type: "number", Placeholder: "Year..."},
		},
		"dbnarrators": {
			{Field: "name", Label: "Name", Type: "text", Placeholder: "Narrator name..."},
			{Field: "audible_id", Label: "Audible ID", Type: "text", Placeholder: "Audible ID..."},
		},
		"dbaudiobook_titles": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{
				Field:       "audiobook_title",
				Label:       "Audiobook Title",
				Type:        "text",
				Placeholder: "Audiobook title...",
			},
			{Field: "region", Label: "Region", Type: "text", Placeholder: "Region..."},
		},
		"audiobooks": {
			{
				Field:       "title",
				Label:       "Audiobook Title",
				Type:        "text",
				Placeholder: "Filter by title...",
			},
			{Field: "narrator", Label: "Narrator", Type: "text", Placeholder: "Narrator name..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality...",
			},
			{
				Field:        "missing",
				Label:        "Missing",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
			{
				Field:        "quality_reached",
				Label:        "Quality Reached",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
		},
		"audiobook_files": {
			{
				Field:       "audiobook_id",
				Label:       "Audiobook ID",
				Type:        "text",
				Placeholder: "Audiobook ID...",
			},
			{
				Field:       "title",
				Label:       "Audiobook Title",
				Type:        "text",
				Placeholder: "Audiobook title...",
			},
			{Field: "filename", Label: "Filename", Type: "text", Placeholder: "Filename..."},
			{Field: "location", Label: "Location", Type: "text", Placeholder: "File path..."},
		},
		"audiobook_file_unmatcheds": {
			{Field: "filepath", Label: "File Path", Type: "text", Placeholder: "File path..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
		},
		"audiobook_histories": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Release title..."},
			{Field: "indexer", Label: "Indexer", Type: "text", Placeholder: "Indexer..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality...",
			},
		},
		// Music tables
		"dbalbums": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "artist", Label: "Artist", Type: "text", Placeholder: "Artist name..."},
			{Field: "label", Label: "Label", Type: "text", Placeholder: "Record label..."},
			{Field: "year", Label: "Year", Type: "number", Placeholder: "Year..."},
		},
		"dbartists": {
			{Field: "name", Label: "Name", Type: "text", Placeholder: "Artist name..."},
			{Field: "country", Label: "Country", Type: "text", Placeholder: "Country..."},
		},
		"dbalbum_titles": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Filter by title..."},
			{
				Field:       "album_title",
				Label:       "Album Title",
				Type:        "text",
				Placeholder: "Album title...",
			},
			{Field: "region", Label: "Region", Type: "text", Placeholder: "Region..."},
		},
		"dbtracks": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Track title..."},
			{
				Field:       "album_title",
				Label:       "Album Title",
				Type:        "text",
				Placeholder: "Album title...",
			},
			{
				Field:       "track_number",
				Label:       "Track Number",
				Type:        "number",
				Placeholder: "Track #...",
			},
		},
		"artists": {
			{Field: "name", Label: "Artist Name", Type: "text", Placeholder: "Artist name..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
		},
		"albums": {
			{Field: "title", Label: "Album Title", Type: "text", Placeholder: "Filter by title..."},
			{Field: "artist", Label: "Artist", Type: "text", Placeholder: "Artist name..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality...",
			},
			{
				Field:        "missing",
				Label:        "Missing",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
			{
				Field:        "quality_reached",
				Label:        "Quality Reached",
				Type:         "select",
				Options:      []string{"", "0", "1"},
				OptionLabels: []string{"All", "No", "Yes"},
			},
		},
		"album_files": {
			{Field: "album_id", Label: "Album ID", Type: "text", Placeholder: "Album ID..."},
			{Field: "title", Label: "Album Title", Type: "text", Placeholder: "Album title..."},
			{Field: "filename", Label: "Filename", Type: "text", Placeholder: "Filename..."},
			{Field: "location", Label: "Location", Type: "text", Placeholder: "File path..."},
		},
		"album_file_unmatcheds": {
			{Field: "filepath", Label: "File Path", Type: "text", Placeholder: "File path..."},
			{Field: "listname", Label: "List Name", Type: "text", Placeholder: "List name..."},
		},
		"album_histories": {
			{Field: "title", Label: "Title", Type: "text", Placeholder: "Release title..."},
			{Field: "indexer", Label: "Indexer", Type: "text", Placeholder: "Indexer..."},
			{
				Field:       "quality_profile",
				Label:       "Quality Profile",
				Type:        "text",
				Placeholder: "Quality...",
			},
		},
	}

	if definitions, exists := filterDefinitions[tableName]; exists {
		for _, def := range definitions {
			filterFields = append(filterFields, createFilterField(def))
		}
	}

	return filterFields
}

// createFilterField creates a filter input field based on field definition.
func createFilterField(def FilterFieldDef) gomponents.Node {
	switch def.Type {
	case "select":
		options := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("All"))}
		for i, opt := range def.Options[1:] { // Skip first empty option as we added "All"
			label := opt
			if i+1 < len(def.OptionLabels) {
				label = def.OptionLabels[i+1]
			}

			options = append(options, html.Option(html.Value(opt), gomponents.Text(label)))
		}

		return html.Div(
			html.Label(html.Class("form-label"), gomponents.Text(def.Label)),
			html.Select(
				append([]gomponents.Node{
					html.Class("form-control custom-filter"),
					html.ID("filter-" + def.Field),
				}, options...)...),
		)

	case "datetime-local":
		return html.Div(
			html.Label(html.Class("form-label"), gomponents.Text(def.Label)),
			html.Input(html.Class("form-control custom-filter"), html.Type("datetime-local"),
				html.ID("filter-"+def.Field), html.Placeholder(def.Placeholder)),
		)

	default:
		return html.Div(
			html.Label(html.Class("form-label"), gomponents.Text(def.Label)),
			html.Input(html.Class("form-control custom-filter"), html.Type(def.Type),
				html.ID("filter-"+def.Field), html.Placeholder(def.Placeholder)),
		)
	}
}

// Helper functions for admin functionality

// FieldMapping holds both struct field name and display name for a database field.
type FieldMapping struct {
	StructField string
	DisplayName string
}

func buildDatabaseTableSelect() gomponents.Node {
	tables := database.GetAllTableNames()
	options := make([]gomponents.Node, 0, len(tables)+1)

	options = append(
		options,
		gomponents.El("option", gomponents.Attr("value", ""), gomponents.Text("— select table —")),
	)
	for _, t := range tables {
		options = append(
			options,
			gomponents.El("option", gomponents.Attr("value", t), gomponents.Text(t)),
		)
	}

	return gomponents.Group([]gomponents.Node{
		gomponents.El(
			"select",
			html.ID("dbTableSelect"),
			html.Class("form-select form-select-sm"),
			gomponents.Attr(
				"onchange",
				"if(this.value)window.location='/api/admin/database/'+this.value",
			),
			gomponents.Group(options),
		),
		html.Script(gomponents.Raw(
			`(function(){` +
				`var m=window.location.pathname.match(/\/database\/([^/?#]+)/);` +
				`if(m)document.getElementById('dbTableSelect').value=m[1];` +
				`})();`,
		)),
	})
}

func createNavbar(activeConfig bool, activeDatabase bool, activeManagement bool) gomponents.Node {
	collapsed := "sidebar-dropdown list-unstyled listunstyle collapse "
	uncollapsed := "sidebar-dropdown list-unstyled listunstyle collapse show "

	cssRootConfig := collapsed
	cssRootDatabase := collapsed
	cssRootManagement := collapsed

	if activeConfig {
		cssRootConfig = uncollapsed
	}

	if activeDatabase {
		cssRootDatabase = uncollapsed
	}

	if activeManagement {
		cssRootManagement = uncollapsed
	}

	return html.Nav(
		html.ID("sidebar"),
		html.Class("sidebar js-sidebar"),
		html.Div(
			html.Class("sidebar-content js-simplebar"),
			html.A(
				html.Class("sidebar-brand"),
				html.Href("/api/admin"),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.I(html.Class("fa-solid fa-download me-3 fs-4")),
					html.Div(
						html.Span(
							html.Class("sidebar-brand-text align-middle d-block"),
							gomponents.Text("Go Media"),
						),
						html.Small(
							html.Class("text-white-50 d-block"),
							html.Style("font-size: 0.7rem; margin-top: -2px;"),
							gomponents.Text("Downloader"),
						),
					),
				),
			),
			html.Ul(html.Class("sidebar-nav "),
				html.Li(html.Class("sidebar-header"), gomponents.Text("Pages")),
				html.Li(
					html.Class("sidebar-item"),
					html.A(
						html.Data("bs-target", "#Configuration"),
						html.Data("bs-toggle", "collapse"),
						html.Class("sidebar-link collapsed"),
						html.I(html.Class("align-middle fa-solid fa-gear")),
						html.Span(html.Class("align-middle"), gomponents.Text("Configuration")),
					),
					html.Ul(
						html.Class(cssRootConfig),
						html.ID("Configuration"),
						html.Data("bs-parent", "#sidebar"),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/config/general"),
								html.I(html.Class("align-middle fa-solid fa-sliders")),
								html.Span(html.Class("align-middle"), gomponents.Text("General")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/config/imdb"),
								html.I(html.Class("align-middle fa-solid fa-film")),
								html.Span(html.Class("align-middle"), gomponents.Text("Imdb")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/config/media"),
								html.I(html.Class("align-middle fa-solid fa-video")),
								html.Span(html.Class("align-middle"), gomponents.Text("Media")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/config/downloader"),
								html.I(html.Class("align-middle fa-solid fa-download")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Downloader"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/config/lists"),
								html.I(html.Class("align-middle fa-solid fa-list")),
								html.Span(html.Class("align-middle"), gomponents.Text("Lists")),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/config/indexers"),
								html.I(html.Class("align-middle fa-solid fa-search")),
								html.Span(html.Class("align-middle"), gomponents.Text("Indexers")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/config/paths"),
								html.I(html.Class("align-middle fa-solid fa-folder")),
								html.Span(html.Class("align-middle"), gomponents.Text("Paths")),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/config/notifications"),
								html.I(html.Class("align-middle fa-solid fa-bell")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Notifications"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/config/regex"),
								html.I(html.Class("align-middle fa-solid fa-code")),
								html.Span(html.Class("align-middle"), gomponents.Text("Regex")),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/config/quality"),
								html.I(html.Class("align-middle fa-solid fa-star")),
								html.Span(html.Class("align-middle"), gomponents.Text("Quality")),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/config/scheduler"),
								html.I(html.Class("align-middle fa-solid fa-clock")),
								html.Span(html.Class("align-middle"), gomponents.Text("Scheduler")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/seriesconfig"),
								html.I(html.Class("align-middle fa-solid fa-tv")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Series Config"),
								),
							),
						),
					),
				),
				html.Li(
					html.Class("sidebar-item inactive"),
					html.A(
						html.Data("bs-target", "#Database"),
						html.Data("bs-toggle", "collapse"),
						html.Class("sidebar-link collapsed"),
						html.I(html.Class("align-middle fa-solid fa-database")),
						html.Span(html.Class("align-middle"), gomponents.Text("Database")),
					),
					html.Ul(
						html.Class(cssRootDatabase),
						html.ID("Database"),
						html.Data("bs-parent", "#sidebar"),
						html.Li(
							html.Class("sidebar-item px-2 py-1"),
							buildDatabaseTableSelect(),
						),
					),
				),
				html.Li(
					html.Class("sidebar-item inactive"),
					html.A(
						html.Data("bs-target", "#Management"),
						html.Data("bs-toggle", "collapse"),
						html.Class("sidebar-link collapsed"),
						html.I(html.Class("align-middle fa-solid fa-cogs")),
						html.Span(html.Class("align-middle"), gomponents.Text("Management")),
					),
					html.Ul(
						html.Class(cssRootManagement),
						html.ID("Management"),
						html.Data("bs-parent", "#sidebar"),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/grid/queue"),
								html.I(html.Class("align-middle fa-solid fa-list-ol")),
								html.Span(html.Class("align-middle"), gomponents.Text("Queue")),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/grid/scheduler"),
								html.I(html.Class("align-middle fa-solid fa-calendar")),
								html.Span(html.Class("align-middle"), gomponents.Text("Scheduler")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/grid/stats"),
								html.I(html.Class("align-middle fa-solid fa-chart-bar")),
								html.Span(html.Class("align-middle"), gomponents.Text("Stats")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/statistics"),
								html.I(html.Class("align-middle fa-solid fa-chart-line")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Statistics Dashboard"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/calendar"),
								html.I(html.Class("align-middle fa-solid fa-calendar-alt")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Calendar"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/metadata-search/movies"),
								html.I(html.Class("align-middle fa-solid fa-film")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Add Movies"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/metadata-search/series"),
								html.I(html.Class("align-middle fa-solid fa-tv")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Add Series"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/metadata-search/audiobooks"),
								html.I(html.Class("align-middle fa-solid fa-headphones")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Add Audiobooks"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/metadata-search/music"),
								html.I(html.Class("align-middle fa-solid fa-music")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Add Music"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/jobmanagement"),
								html.I(html.Class("align-middle fa-solid fa-tasks")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Job Management"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/debugstats"),
								html.I(html.Class("align-middle fa-solid fa-bug")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Debug Statistics"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/dbmaintenance"),
								html.I(html.Class("align-middle fa-solid fa-tools")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Database Maintenance"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/searchdownload"),
								html.I(
									html.Class(
										"align-middle fa-solid fa-magnifying-glass-arrow-right",
									),
								),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Search & Download"),
								),
								html.Span(
									html.Class("badge bg-success ms-1"),
									html.Style("font-size: 0.6rem;"),
									gomponents.Text("NEW"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/pushovertest"),
								html.I(html.Class("align-middle fa-solid fa-paper-plane")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Pushover Test"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/logviewer"),
								html.I(html.Class("align-middle fa-solid fa-file-text")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Log Viewer"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/feedparse"),
								html.I(html.Class("align-middle fa-solid fa-rss")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Feed Parser"),
								),
								html.Span(
									html.Class("badge bg-info ms-1"),
									html.Style("font-size: 0.6rem;"),
									gomponents.Text("BETA"),
								),
							),
						),
						// html.Li(html.Class("sidebar-item"),
						// 	html.A(html.Class("sidebar-link"), html.Href("/api/admin/folderstructure"),
						// 		html.Span(html.Class("align-middle"), gomponents.Text("Folder Organizer")),
						// 	),
						// ),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/testparse"),
								html.I(html.Class("align-middle fa-solid fa-flask")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Test Parsing"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/moviemetadata"),
								html.I(html.Class("align-middle fa-solid fa-info-circle")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Test Metadata"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/traktauth"),
								html.I(html.Class("align-middle fa-solid fa-key")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Trakt Authenticate"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/namingtest"),
								html.I(html.Class("align-middle fa-solid fa-edit")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Test Naming"),
								),
							),
						),
					),
				),

				html.Li(
					html.Class("sidebar-item inactive"),
					html.A(
						html.Data("bs-target", "#Tools"),
						html.Data("bs-toggle", "collapse"),
						html.Class("sidebar-link collapsed"),
						html.I(html.Class("align-middle fa-solid fa-toolbox")),
						html.Span(html.Class("align-middle"), gomponents.Text("Tools")),
					),
					html.Ul(
						html.Class(cssRootManagement),
						html.ID("Tools"),
						html.Data("bs-parent", "#sidebar"),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/media-cleanup"),
								html.I(html.Class("align-middle fas fa-broom")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Media Cleanup"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/missing-episodes"),
								html.I(html.Class("align-middle fa-solid fa-exclamation-triangle")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Missing Episodes"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/log-analysis"),
								html.I(html.Class("align-middle fas fa-chart-line")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Log Analysis"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/storage-health"),
								html.I(html.Class("align-middle fas fa-hdd")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Storage Health"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/service-health"),
								html.I(html.Class("align-middle fas fa-heartbeat")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Service Health"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/api-testing"),
								html.I(html.Class("align-middle fas fa-cogs")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("API Testing"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/quality-reorder"),
								html.I(html.Class("align-middle fas fa-sort-amount-down")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Quality Reorder"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/regex-tester"),
								html.I(html.Class("align-middle fas fa-search-plus")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Regex Tester"),
								),
								html.Span(
									html.Class("badge bg-info ms-1"),
									html.Style("font-size: 0.6rem;"),
									gomponents.Text("NEW"),
								),
							),
						),
						html.Li(
							html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/naming-generator"),
								html.I(html.Class("align-middle fas fa-code")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Naming Generator"),
								),
								html.Span(
									html.Class("badge bg-success ms-1"),
									html.Style("font-size: 0.6rem;"),
									gomponents.Text("NEW"),
								),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(
								html.Class("sidebar-link"),
								html.Href("/api/admin/crongen"),
								html.I(html.Class("align-middle fas fa-clock")),
								html.Span(
									html.Class("align-middle"),
									gomponents.Text("Cron Generator"),
								),
							),
						),
					),
				),
			),
			html.Div(
				html.Class("simplebar-track simplebar-horizontal"),
				html.Style("visibility: hidden;"),
				html.Div(
					html.Class("simplebar-scrollbar"),
					html.Style("width: 0px; display: none;"),
				),
			),
			html.Div(
				html.Class("simplebar-track simplebar-vertical"),
				html.Style("visibility: visible;"),
				html.Div(
					html.Class("simplebar-scrollbar"),
					html.Style(
						"height: 933px; transform: translate3d(0px, 0px, 0px); display: block;",
					),
				),
			),
		),
	)
}

// adminDatabaseTab component.
func adminDatabaseContent(tableName string, csrfToken string) gomponents.Node {
	tableColumns := getAdminTableColumns(tableName)
	tableDefault := database.GetTableDefaults(tableName)

	tableInfo := TableInfo{
		Name:    tableName,
		Columns: tableColumns,
		Rows: func() []map[string]any {
			if tableDefault.DefaultColumns == "" {
				// logger.Logtype("warning", 2).Str("table", tableName).Str("defaultColumns", tableDefault.DefaultColumns).Msg("Empty DefaultColumns")
				return []map[string]any{}
			}

			query := fmt.Sprintf(
				"SELECT %s FROM %s LIMIT 10",
				tableDefault.DefaultColumns,
				tableDefault.Table,
			)
			// logger.Logtype("debug", 1).Str("query", query).Msg("Executing preview query")
			return database.GetrowsType(tableDefault.Object, false, 10, query)
		}(),
		DeleteURL: fmt.Sprintf("/api/admin/table/%s/delete", tableName),
	}

	// Enhanced database content with modern design
	return html.Div(
		html.Input(html.Name("table-name"), html.Type("hidden"), html.ID("table-name")),

		adminModal(),
		adminAddModal(tableName, csrfToken),

		// Enhanced main container with modern styling
		html.Div(
			html.Class("config-section-enhanced"),

			// Enhanced status messages with modern alerts
			html.Div(html.Class("alert-success-enhanced d-none"), html.ID("db-success")),
			html.Div(html.Class("alert-danger-enhanced d-none"), html.ID("db-error")),

			// Enhanced page header with AdminKit design
			html.Div(
				html.Class("page-header-enhanced"),
				html.Div(
					html.Class("header-content"),
					html.Div(
						html.Class("header-icon-wrapper"),
						html.I(html.Class("fas fa-database header-icon")),
					),
					html.Div(
						html.Class("header-text"),
						html.H2(
							html.Class("header-title"),
							gomponents.Text(strings.ReplaceAll(tableName, "_", " ")),
						),
						html.P(
							html.Class("header-subtitle"),
							gomponents.Text(
								"Manage database records with advanced filtering and real-time operations",
							),
						),
					),
					html.Div(
						html.Class("ms-auto d-flex gap-2"),
						html.Button(
							html.Class("btn btn-outline-secondary btn-sm"),
							html.Type("button"),
							html.Data("bs-toggle", "modal"),
							html.Data("bs-target", "#addRecordModal"),
							html.I(html.Class("fas fa-plus me-1")),
							gomponents.Text("Add Record"),
						),
						html.Button(
							html.Class("btn btn-outline-secondary btn-sm"),
							html.Type("button"),
							html.ID("refresh-table"),
							html.I(html.Class("fas fa-sync-alt me-1")),
							gomponents.Text("Refresh"),
						),
					),
				),
			),

			// Button description region
			adminButtonDescription(tableName),

			// Enhanced custom filters
			renderCustomFilters(tableName),

			// Enhanced table container
			html.Div(
				html.Class("database-table-container"),
				html.ID("table-content"),
				renderTable(&tableInfo, csrfToken),
			),
		),
	)
}

// adminButtonDescription returns a description panel explaining the available action buttons for the given table.
func adminButtonDescription(tableName string) gomponents.Node {
	var desc string
	switch tableName {
	case "dbmovies":
		desc = "Edit: Edit record | Delete: Remove record | Refresh: Refresh movie metadata from external sources"
	case "dbseries":
		desc = "Edit: Edit record | Delete: Remove record | Refresh: Refresh series metadata from external sources"
	case "dbalbums":
		desc = "Edit: Edit record | Delete: Remove record | Refresh: Start metadata refresh job for all albums"
	case "dbaudiobooks":
		desc = "Edit: Edit record | Delete: Remove record | Refresh: Start metadata refresh job for all audiobooks"
	case "dbbooks":
		desc = "Edit: Edit record | Delete: Remove record | Refresh: Start metadata refresh job for all books"
	case "movies":
		desc = "Edit: Edit record | Delete: Remove record | Files: View associated files | Search: Search by IMDB ID | Search+: Search by title"
	case "serie_episodes":
		desc = "Edit: Edit record | Delete: Remove record | Files: View associated files | Search: Search by TVDB ID | Search+: Search by title"
	case "albums":
		desc = "Edit: Edit record | Delete: Remove record | Files: View album files | Search+: Search for missing albums in this list"
	case "audiobooks":
		desc = "Edit: Edit record | Delete: Remove record | Files: View audiobook files | Search+: Search for missing audiobooks in this list"
	case "books":
		desc = "Edit: Edit record | Delete: Remove record | Files: View book files | Search+: Search for missing books in this list"
	case "dbartists":
		desc = "Edit: Edit record | Delete: Remove record | List: View albums by this artist"
	case "artists":
		desc = "Edit: Edit record | Delete: Remove record | List: View albums by this artist | Search: Search for missing albums by artist | Discover: Discover and import all series albums"
	case "dbauthors":
		desc = "Edit: Edit record | Delete: Remove record | List: View books or audiobooks by this author"
	case "authors":
		desc = "Edit: Edit record | Delete: Remove record | List: View books or audiobooks by this author | Search: Search for missing books/audiobooks by author"
	default:
		desc = "Edit: Edit record | Delete: Remove record"
	}

	return html.Div(
		html.Class("alert alert-info py-2 px-3 mb-3"),
		html.Style("font-size: 0.85rem;"),
		html.I(html.Class("fas fa-info-circle me-2")),
		gomponents.Text(desc),
	)
}

// adminModal component.
func adminModal() gomponents.Node {
	return html.Div(
		html.Class("modal fade"),
		html.ID("editFormModal"),

		html.Div(
			html.Class("modal-dialog modal-xl"),

			html.Div(
				html.Class("modal-content"),

				html.Div(
					html.Class("modal-header"),
					html.H5(html.Class("modal-title"), gomponents.Text("Edit Record")),
					html.Button(
						html.Class("btn-close"),
						gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"),
					),
				),

				html.Div(
					html.Class("modal-body"),
					html.ID("modal-body-content"),
					// The form content will be loaded here by the DataTables edit handler
				),
			),
		),
	)
}

// adminAddModal component.
func adminAddModal(tableName string, csrfToken string) gomponents.Node {
	// Get table columns to create empty data map - use form-specific columns to exclude joined columns
	emptyData := make(map[string]any)
	tableColumns := getAdminFormColumns(tableName)

	// Initialize empty data for all columns except auto-generated ones
	for _, col := range tableColumns {
		columnName := col.Name
		if strings.Contains(col.Name, " as ") {
			columnName = strings.Split(col.Name, " as ")[1]
		}

		// Skip auto-generated fields
		if columnName != "id" && columnName != "created_at" && columnName != "updated_at" {
			// Initialize boolean-like fields as integers (0 = false) so they render as switches
			if columnName == "missing" || columnName == "blacklisted" ||
				columnName == "dont_search" ||
				columnName == "dont_upgrade" ||
				columnName == "use_regex" ||
				columnName == "proper" ||
				columnName == "extended" ||
				columnName == "repack" ||
				columnName == "ignore_runtime" ||
				columnName == "adult" ||
				columnName == "search_specials" ||
				columnName == "quality_reached" {
				emptyData[columnName] = 0
			} else {
				emptyData[columnName] = ""
			}
		}
	}

	return html.Div(
		html.Class("modal fade"),
		html.ID("addRecordModal"),

		html.Div(
			html.Class("modal-dialog modal-lg"),

			html.Div(
				html.Class("modal-content"),

				html.Div(
					html.Class("modal-header"),
					html.H5(html.Class("modal-title"), gomponents.Text("Add New Record")),
					html.Button(
						html.Class("btn-close"),
						gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"),
					),
				),
				html.Div(
					html.Class("modal-body"),
					renderTableEditForm(tableName, emptyData, "new", csrfToken),
				),
			),
		),
	)
}

// adminJavaScript component.
func adminJavaScript() gomponents.Node {
	jsContent := `
			function showToaster(type, message) {								
						// Fallback for missing Bootstrap or when toasts don't work
						if (typeof bootstrap === 'undefined' || !bootstrap.Toast) {
										alert((type === 'success' ? '✓ ' : '✗ ') + message);
							return;
						}
						
						const toastContainer = document.getElementById('toastContainer');
						if (!toastContainer) {
										alert((type === 'success' ? '✓ ' : '✗ ') + message);
							return;
						}
						
						const toastId = 'toast-' + Date.now();
						
						// Determine toast styling based on type
						let bgClass, iconClass, headerText;
						switch(type) {
							case 'success':
								bgClass = 'bg-success text-white';
								iconClass = 'fas fa-check-circle';
								headerText = 'Success';
								break;
							case 'error':
								bgClass = 'bg-danger text-white';
								iconClass = 'fas fa-exclamation-circle';
								headerText = 'Error';
								break;
							case 'warning':
								bgClass = 'bg-warning text-dark';
								iconClass = 'fas fa-exclamation-triangle';
								headerText = 'Warning';
								break;
							case 'info':
							default:
								bgClass = 'bg-info text-white';
								iconClass = 'fas fa-info-circle';
								headerText = 'Info';
								break;
						}
						
						// Create toast HTML
						const toastHTML = ` + "`" + `
							<div id="${toastId}" class="toast align-items-center ${bgClass} border-0" role="alert" aria-live="assertive" aria-atomic="true">
								<div class="d-flex">
									<div class="toast-body d-flex align-items-center">
										<i class="${iconClass} me-2"></i>
										<span>${message}</span>
									</div>
									<button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
								</div>
							</div>
						` + "`" + `;
						
						// Add toast to container
						toastContainer.insertAdjacentHTML('beforeend', toastHTML);
						
						try {
							// Initialize and show the toast
							const toastElement = document.getElementById(toastId);
							const toast = new bootstrap.Toast(toastElement, {
								autohide: true,
								delay: type === 'error' ? 8000 : 4000  // Error messages stay longer
							});
							
							// Remove toast from DOM after it's hidden
							toastElement.addEventListener('hidden.bs.toast', function() {
								toastElement.remove();
							});
							
							toast.show();
									} catch (error) {
										alert((type === 'success' ? '✓ ' : '✗ ') + message);
						}
					}
					
					// Global error handler for AJAX requests
					window.showToaster = showToaster;
			// Add CSS for Choices.js z-index fix
			if (!document.querySelector('#choices-css-fix')) {
				var style = document.createElement('style');
				style.id = 'choices-css-fix';
				style.textContent = '.choices { position: relative; } ' +
					'.choices[data-type*="select-one"] .choices__inner { cursor: pointer; } ' +
					'.choices__list--dropdown.is-active { transform: none !important; } ' +
					'.form-card, .form-cards-grid, .modal-body, .card-body, .edit-form-container, .edit-form-fields, .config-section-enhanced { overflow: visible !important; } ' +
					'.choices.is-open .choices__list--dropdown { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; } ' +
					'.form-group-enhanced .form-check-input-modern:checked + label::after { content: "✓"; position: absolute; right: 10px; color: #28a745; font-weight: bold; } ' +
					'.form-group-enhanced { position: relative; } ' +
					'.form-group-enhanced .form-check-input-modern { position: relative; }';
				document.head.appendChild(style);
			}

			// Enhanced Choices.js initialization with AJAX support
			window.initChoicesGlobal = function() {
				// Check if Choices is available
				if (typeof Choices === 'undefined') {
					return;
				}
				
				// Check if Choices elements exist
				if (document.querySelectorAll('.choices-ajax').length === 0) {
					return;
				}
				
				// Remove duplicate elements with same ID to prevent conflicts
				var seenIds = {};
				document.querySelectorAll('.choices-ajax').forEach(function(element) {
					var id = element.id;
					if (seenIds[id]) {
						element.remove();
					} else {
						seenIds[id] = true;
					}
				});
				
				// Initialize AJAX-enabled Choices.js dropdowns
				document.querySelectorAll('.choices-ajax').forEach(function(element) {
					// Skip if already initialized
					if (element.classList.contains('choices__input')) {
						return;
					}
					var ajaxUrl = element.dataset.ajaxUrl;
					var selectedValue = element.dataset.selectedValue;
					var placeholder = element.dataset.placeholder || 'Search...';
					var csrfToken = document.querySelector('input[name="csrf_token"]').value || '';
					
					// Clear existing options
					element.innerHTML = '<option value="">' + placeholder + '</option>';
					
					var allowCustom = element.dataset.allowCustom === 'true';
					
					var choices = new Choices(element, {
						placeholder: true,
						placeholderValue: placeholder,
						searchPlaceholderValue: allowCustom ? 'Type to search or enter custom value...' : 'Type to search...',
						removeItemButton: true,
						searchEnabled: true,
						searchResultLimit: 50,
						renderChoiceLimit: 50,
						shouldSort: false,
						allowHTML: true,
						addItems: allowCustom,
						editItems: false,  // Never allow editing of selected items
						addItemText: allowCustom ? 'Press Enter to add custom value' : '',
						duplicateItemsAllowed: false,
						position: 'auto'
					});
					
					// Store choices instance for later use
					element.choicesInstance = choices;
					
					// Ensure dropdown is never disabled
					choices.enable();
					
					// Fix dropdown positioning when opened
					element.addEventListener('choice', function() {
						setTimeout(function() {
							var dropdown = element.parentElement.querySelector('.choices__list--dropdown');
							if (dropdown) {
								var rect = element.getBoundingClientRect();
								var viewportHeight = window.innerHeight;
								var spaceBelow = viewportHeight - rect.bottom;
								var dropdownHeight = dropdown.offsetHeight || 200;
								
								if (spaceBelow < dropdownHeight && rect.top > dropdownHeight) {
									// Not enough space below, position above if there's room
									dropdown.style.top = 'auto';
									dropdown.style.bottom = '100%';
								} else {
									// Default position below
									dropdown.style.top = '100%';
									dropdown.style.bottom = 'auto';
								}
								dropdown.style.left = '0';
								dropdown.style.right = 'auto';
								dropdown.style.width = element.offsetWidth + 'px';
							}
						}, 10);
					});
					
					// Function to load choices from server
					function loadChoices(searchTerm) {
						fetch(ajaxUrl, {
							method: 'POST',
							headers: {
								'Content-Type': 'application/x-www-form-urlencoded',
								'X-CSRF-Token': csrfToken
							},
							body: 'search=' + encodeURIComponent(searchTerm || '') + '&page=1'
						})
						.then(response => response.json())
						.then(data => {
							if (data.results && Array.isArray(data.results)) {
								choices.clearStore();
								if (data.results.length > 0) {
									choices.setChoices(data.results.map(function(item) {
										return {
											value: item.id,
											label: item.text,
											selected: false
										};
									}), 'value', 'label', true);
								} else {
									// Don't set disabled choices - just clear the store
									// This prevents the dropdown from becoming disabled
									choices.clearStore();
									choices.enable(); // Ensure it stays enabled
								}
							}
						})
						.catch(error => {
							console.error('Error loading choices:', error);
						});
					}

					// Set up search event listener
					element.addEventListener('search', function(event) {
						var searchTerm = event.detail.value;
						if (searchTerm && searchTerm.length >= 2) {
							loadChoices(searchTerm);
						} else if (searchTerm.length === 0) {
							// Load initial options when search is cleared
							loadChoices('');
						}
					});

					// Load initial options on first open
					var initialLoadDone = false;
					element.addEventListener('showDropdown', function() {
						if (!initialLoadDone) {
							loadChoices('');
							initialLoadDone = true;
						}
					});

					// Load selected value if it exists
					if (selectedValue && selectedValue !== '') {
						fetch(ajaxUrl, {
							method: 'POST',
							headers: {
								'Content-Type': 'application/x-www-form-urlencoded',
								'X-CSRF-Token': csrfToken
							},
							body: 'id=' + encodeURIComponent(selectedValue)
						})
						.then(response => response.json())
						.then(data => {
							if (data.results && data.results.length > 0) {
								var selectedItem = data.results[0];
								
								// First add the selected item as a choice if it's not already there
								var existingChoice = choices._store.choices.find(function(choice) {
									return choice.value == selectedItem.id;
								});
								
								if (!existingChoice) {
									choices.setChoices([{
										value: selectedItem.id,
										label: selectedItem.text,
										selected: true
									}], 'value', 'label', false);
								} else {
									// If choice exists, just select it
									choices.setChoiceByValue(selectedItem.id.toString());
								}
								// Ensure dropdown stays enabled after loading selected value
								choices.enable();
							}
						})
						.catch(error => {
							console.error('Error loading selected value:', error);
							choices.enable(); // Ensure it stays enabled even on error
						});
					}
				});
			};
			
			// Centralized DataTable initialization function
			window.initDataTable = function(selector, options) {
				console.log('initDataTable called with:', selector, options);
				// Check if DataTables is available
				if (!$.fn.DataTable) {
					console.warn('DataTables not available for selector:', selector);
					return null;
				}
				
				// Default configuration
				var defaultConfig = {
					pageLength: 25,
					responsive: true,
					destroy: true,
					searching: true,
					ordering: true,
					paging: true,
					language: {
						search: "Filter:",
						lengthMenu: "Show _MENU_ entries per page",
						info: "Showing _START_ to _END_ of _TOTAL_ entries",
						zeroRecords: "No matching records found",
						emptyTable: "No data available in table"
					}
				};
				
				// Server-side specific defaults
				if (options && options.serverSide) {
					defaultConfig.processing = true;
					defaultConfig.serverSide = true;
					defaultConfig.order = [[ 0, "desc" ]];
				}
				
				// Merge provided options with defaults
				var config = $.extend(true, {}, defaultConfig, options || {});
				
				// Initialize DataTable
				var table = $(selector).DataTable(config);
				
				return table;
			};
			
			// Initialize Choices.js when Add Record Modal is shown
			$(document).on('shown.bs.modal', '#addRecordModal', function() {
				setTimeout(function() {
					if (window.initChoicesGlobal) {
						window.initChoicesGlobal();
					}
				}, 100);
			});
			
			// Initialize Choices.js when Edit Form Modal is shown  
			$(document).on('shown.bs.modal', '#editFormModal', function() {
				setTimeout(function() {
					if (window.initChoicesGlobal) {
						window.initChoicesGlobal();
					}
				}, 100);
			});
			
			// Enhanced form submission handling for tools sidebar
			document.addEventListener('DOMContentLoaded', function() {
				// Ensure form submissions work with both form-group and form-group-enhanced
				const forms = document.querySelectorAll('form');
				forms.forEach(function(form) {
					form.addEventListener('submit', function(e) {
						// Find all checkboxes in the form (both regular and modern)
						const checkboxes = form.querySelectorAll('input[type="checkbox"]');
						checkboxes.forEach(function(checkbox) {
							// Ensure checkbox values are properly serialized
							if (checkbox.checked && !checkbox.value) {
								checkbox.value = 'on';
							}
						});
					});
				});
				
				// Initialize Choices.js on page load
				if (window.initChoicesGlobal) {
					window.initChoicesGlobal();
				}
				
				// Add current page highlighting
				const currentPath = window.location.pathname;
				document.querySelectorAll('.sidebar-link[href]').forEach(function(link) {
					if (link.getAttribute('href') === currentPath) {
						const parentItem = link.closest('.sidebar-item');
						if (parentItem) {
							parentItem.classList.add('current');
							// Expand parent dropdown if this is a sub-item
							const parentDropdown = parentItem.closest('.sidebar-dropdown');
							if (parentDropdown) {
								parentDropdown.classList.add('show');
								const parentToggle = document.querySelector('[data-bs-target="#' + parentDropdown.id + '"]');
								if (parentToggle) parentToggle.classList.remove('collapsed');
							}
						}
					}
				});
			});
		`

	return html.Script(gomponents.Raw(jsContent))
}

// getDropdownOptionByID retrieves a single dropdown option by ID for preselection.
func getDropdownOptionByID(tableName, fieldName string, id int) *map[string]any {
	switch tableName {
	case "dbmovies":
		if movie, err := database.Structscan[database.Dbmovie](
			"SELECT id, title FROM dbmovies WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(movie.ID, movie.Title)
		}

	case "dbseries":
		if serie, err := database.Structscan[database.Dbserie](
			"SELECT id, seriename FROM dbseries WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(serie.ID, serie.Seriename)
		}

	case "dbserie_episodes":
		if episode, err := database.Structscan[database.DbserieEpisode](
			"SELECT id, identifier, title FROM dbserie_episodes WHERE id = ?",
			false,
			id,
		); err == nil {
			label := fmt.Sprintf("%s - %s", episode.Identifier, episode.Title)
			return createSelect2OptionPtr(episode.ID, label)
		}

	case "movies":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbmovies.title || ' - ' || movies.listname, movies.id FROM movies LEFT JOIN dbmovies ON movies.dbmovie_id = dbmovies.id WHERE movies.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "series":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbseries.seriename || ' - ' || series.listname, series.id FROM series LEFT JOIN dbseries ON series.dbserie_id = dbseries.id WHERE series.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "serie_episodes":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT COALESCE(dbseries.seriename, 'Unknown Series') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.identifier IS NOT NULL AND dbserie_episodes.identifier != 'S00E00' THEN dbserie_episodes.identifier ELSE 'ID:' || serie_episodes.id END, 'Unknown') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.title IS NOT NULL AND TRIM(dbserie_episodes.title) != '' THEN dbserie_episodes.title ELSE 'Episode ' || COALESCE(dbserie_episodes.episode, 'Unknown') END, 'Unknown Episode') || ' (' || series.listname || ')', serie_episodes.id FROM serie_episodes LEFT JOIN dbserie_episodes ON serie_episodes.dbserie_episode_id = dbserie_episodes.id LEFT JOIN series ON serie_episodes.serie_id = series.id LEFT JOIN dbseries ON series.dbserie_id = dbseries.id WHERE serie_episodes.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "qualities":
		// Determine quality type based on field name
		var typeFilter string
		switch fieldName {
		case "resolution_id":
			typeFilter = " AND type = 1"
		case "quality_id":
			typeFilter = " AND type = 2"
		case "codec_id":
			typeFilter = " AND type = 3"
		case "audio_id":
			typeFilter = " AND type = 4"
		default:
			typeFilter = ""
		}

		query := fmt.Sprintf("SELECT id, name FROM qualities WHERE id = ?%s", typeFilter)
		if quality, err := database.Structscan[database.Qualities](query, false, id); err == nil {
			return createSelect2OptionPtr(quality.ID, quality.Name)
		}

	// Book tables
	case "dbbooks":
		if book, err := database.Structscan[database.Dbbook](
			"SELECT id, title FROM dbbooks WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(book.ID, book.Title)
		}

	case "dbauthors":
		if author, err := database.Structscan[database.Dbauthor](
			"SELECT id, name FROM dbauthors WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(author.ID, author.Name)
		}

	case "books":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbbooks.title || ' - ' || books.listname, books.id FROM books LEFT JOIN dbbooks ON books.dbbook_id = dbbooks.id WHERE books.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	// Audiobook tables
	case "dbaudiobooks":
		if audiobook, err := database.Structscan[database.Dbaudiobook](
			"SELECT id, title FROM dbaudiobooks WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(audiobook.ID, audiobook.Title)
		}

	case "dbnarrators":
		if narrator, err := database.Structscan[database.Dbnarrator](
			"SELECT id, name FROM dbnarrators WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(narrator.ID, narrator.Name)
		}

	case "audiobooks":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbaudiobooks.title || ' - ' || audiobooks.listname, audiobooks.id FROM audiobooks LEFT JOIN dbaudiobooks ON audiobooks.dbaudiobook_id = dbaudiobooks.id WHERE audiobooks.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	// Music tables
	case "dbalbums":
		if album, err := database.Structscan[database.Dbalbum](
			"SELECT id, title FROM dbalbums WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(album.ID, album.Title)
		}

	case "dbartists":
		if artist, err := database.Structscan[database.Dbartist](
			"SELECT id, name FROM dbartists WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(artist.ID, artist.Name)
		}

	case "albums":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbalbums.title || ' - ' || albums.listname, albums.id FROM albums LEFT JOIN dbalbums ON albums.dbalbum_id = dbalbums.id WHERE albums.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	// Alternative title tables
	case "dbbook_titles":
		if title, err := database.Structscan[database.DbbookTitle](
			"SELECT id, title FROM dbbook_titles WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(title.ID, title.Title)
		}

	case "dbaudiobook_titles":
		if title, err := database.Structscan[database.DbaudiobookTitle](
			"SELECT id, title FROM dbaudiobook_titles WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(title.ID, title.Title)
		}

	case "dbalbum_titles":
		if title, err := database.Structscan[database.DbalbumTitle](
			"SELECT id, title FROM dbalbum_titles WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(title.ID, title.Title)
		}

	// Series and track tables
	case "dbbook_series":
		if series, err := database.Structscan[database.DbbookSeries](
			"SELECT id, name FROM dbbook_series WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(series.ID, series.Name)
		}

	case "book_series":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbbook_series.name || ' - ' || book_series.listname, book_series.id FROM book_series LEFT JOIN dbbook_series ON book_series.dbbook_series_id = dbbook_series.id WHERE book_series.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "dbtracks":
		if track, err := database.Structscan[database.Dbtrack](
			"SELECT id, title FROM dbtracks WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(track.ID, track.Title)
		}

	// Tracked authors and artists
	case "authors":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbauthors.name || ' - ' || authors.listname, authors.id FROM authors LEFT JOIN dbauthors ON authors.dbauthor_id = dbauthors.id WHERE authors.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "artists":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbartists.name || ' - ' || artists.listname, artists.id FROM artists LEFT JOIN dbartists ON artists.dbartist_id = dbartists.id WHERE artists.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}
	}

	return nil
}

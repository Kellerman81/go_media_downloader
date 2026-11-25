package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderListsForm renders one row; idx is the row index used in the form field names.
func renderListsForm(idx int, configv *config.ListsConfig, csrfToken string) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "lists_" + strconv.Itoa(idx)

	return renderOptimizedArrayItemForm("lists", configv.Name, "List", configv,
		renderListsConfigSections(configv, group, comments, displayNames, csrfToken))
}

// listsTestFields are the fields HandleTestListConnection reads, by name
// suffix within the current row - a superset covering every list type with
// a lightweight connectivity check (see HandleTestListConnection).
var listsTestFields = []connTestField{
	{Key: "listtype", Suffix: "ListType"},
	{Key: "traktusername", Suffix: "TraktUsername"},
	{Key: "traktlistname", Suffix: "TraktListName"},
	{Key: "traktlisttype", Suffix: "TraktListType"},
	{Key: "plexserverurl", Suffix: "PlexServerURL"},
	{Key: "plextoken", Suffix: "PlexToken"},
	{Key: "plexusername", Suffix: "PlexUsername"},
	{Key: "jellyfinserverurl", Suffix: "JellyfinServerURL"},
	{Key: "jellyfintoken", Suffix: "JellyfinToken"},
	{Key: "jellyfinusername", Suffix: "JellyfinUsername"},
	{Key: "imdbcsvfile", Suffix: "IMDBCSVFile"},
}

// renderListsConfigSections organizes lists fields into logical groups.
func renderListsConfigSections(
	configv *config.ListsConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "listsConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-lists-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{
					Name:    "",
					Type:    "custom",
					Value:   renderTemplateSelect("Template", listTemplates),
					Options: nil,
				},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "ListType",
					Type:  "select",
					Value: configv.ListType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {
							"seriesconfig",
							"traktpublicshowlist",
							"imdbcsv",
							"imdbfile",
							"traktpublicmovielist",
							"traktmoviepopular",
							"traktmovieanticipated",
							"tmdbmovieupcoming",
							"traktmovietrending",
							"traktseriepopular",
							"traktserieanticipated",
							"traktserietrending",
							"tmdbmoviepopular",
							"tmdbmovietrending",
							"tmdbmoviediscover",
							"tmdblist",
							"tmdbseriepopular",
							"tmdbserietrending",
							"tmdbshowdiscover",
							"tmdbshowlist",
							"newznabrss",
							"plexwatchlist",
							"jellyfinwatchlist",
							"moviescraper",
							"rottentomatoes",
							"letterboxd",
							"audiobookconfig",
							"bookconfig",
							"musicconfig",
							"musiccharts",
							"bookbestsellers",
							"lastfmtopartists",
							"irc",
						},
					}),
				},
				{
					Name:  "",
					Type:  "custom",
					Value: renderConnectionTestControl(csrfToken, "/api/admin/lists/test", listsTestFields),
				},
				{Name: "Enabled", Type: "checkbox", Value: configv.Enabled, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Source Configuration
		renderConfigGroupWithParent(
			"Source Configuration",
			"source-lists-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "URL", Type: "text", Value: configv.URL, Options: nil},
				{Name: "IMDBCSVFile", Type: "text", Value: configv.IMDBCSVFile, Options: nil},
				{
					Name:    "ManualConfigFile",
					Type:    "text",
					Value:   configv.ManualConfigFile,
					Options: convertMapToSelectOptions(map[string][]string{"options": manualConfigFileSuggestions()}),
				},
				{
					Name:    "SkipEmptySize",
					Type:    "checkbox",
					Value:   configv.SkipEmptySize,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Trakt Settings
		renderConfigGroupWithParent(
			"Trakt Settings",
			"trakt-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "TraktUsername", Type: "text", Value: configv.TraktUsername, Options: nil},
				{Name: "TraktListName", Type: "text", Value: configv.TraktListName, Options: nil},
				{
					Name:  "TraktListType",
					Type:  "select",
					Value: configv.TraktListType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"", "movie", "show"},
					}),
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Filter Settings
		renderConfigGroupWithParent(
			"Filter Settings",
			"filter-lists-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Limit", Type: "text", Value: configv.Limit, Options: nil},
				{Name: "MinVotes", Type: "number", Value: configv.MinVotes, Options: nil},
				{Name: "MinRating", Type: "number", Value: configv.MinRating, Options: nil},
				{Name: "Excludegenre", Type: "array", Value: configv.Excludegenre, Options: nil},
				{Name: "Includegenre", Type: "array", Value: configv.Includegenre, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// TMDB Settings
		renderConfigGroupWithParent(
			"TMDB Settings",
			"tmdb-lists-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "TmdbDiscover", Type: "array", Value: configv.TmdbDiscover, Options: nil},
				{Name: "TmdbList", Type: "arrayint", Value: configv.TmdbList, Options: nil},
				{Name: "TmdbPages", Type: "number", Value: configv.TmdbPages, Options: nil},
				{
					Name:    "RemoveFromList",
					Type:    "checkbox",
					Value:   configv.RemoveFromList,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Media Server Settings
		renderConfigGroupWithParent(
			"Media Server Settings",
			"mediaserver-lists-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "PlexServerURL", Type: "text", Value: configv.PlexServerURL, Options: nil},
				{Name: "PlexToken", Type: "password", Value: configv.PlexToken, Options: nil},
				{Name: "PlexUsername", Type: "text", Value: configv.PlexUsername, Options: nil},
				{
					Name:    "JellyfinServerURL",
					Type:    "text",
					Value:   configv.JellyfinServerURL,
					Options: nil,
				},
				{Name: "JellyfinToken", Type: "password", Value: configv.JellyfinToken, Options: nil},
				{
					Name:    "JellyfinUsername",
					Type:    "text",
					Value:   configv.JellyfinUsername,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - Basic Configuration
		renderConfigGroupWithParent(
			"Movie Scraper - Basic Configuration",
			"moviescraper-basic-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:  "MovieScraperType",
					Type:  "select",
					Value: configv.MovieScraperType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"", "htmlxpath", "csrfapi", "project1service", "algolia"},
					}),
				},
				{
					Name:    "MovieScraperStartURL",
					Type:    "text",
					Value:   configv.MovieScraperStartURL,
					Options: nil,
				},
				{
					Name:    "MovieScraperSiteURL",
					Type:    "text",
					Value:   configv.MovieScraperSiteURL,
					Options: nil,
				},
				{
					Name:    "MovieScraperSiteID",
					Type:    "number",
					Value:   configv.MovieScraperSiteID,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - HTML/XPath Settings
		renderConfigGroupWithParent(
			"Movie Scraper - HTML/XPath Settings",
			"moviescraper-xpath-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MovieSceneNodeXPath",
					Type:    "text",
					Value:   configv.MovieSceneNodeXPath,
					Options: nil,
				},
				{
					Name:    "MovieTitleXPath",
					Type:    "text",
					Value:   configv.MovieTitleXPath,
					Options: nil,
				},
				{Name: "MovieYearXPath", Type: "text", Value: configv.MovieYearXPath, Options: nil},
				{
					Name:    "MovieImdbIDXPath",
					Type:    "text",
					Value:   configv.MovieImdbIDXPath,
					Options: nil,
				},
				{Name: "MovieURLXPath", Type: "text", Value: configv.MovieURLXPath, Options: nil},
				{
					Name:    "MovieReleaseDateXPath",
					Type:    "text",
					Value:   configv.MovieReleaseDateXPath,
					Options: nil,
				},
				{
					Name:    "MovieGenreXPath",
					Type:    "text",
					Value:   configv.MovieGenreXPath,
					Options: nil,
				},
				{
					Name:    "MovieRatingXPath",
					Type:    "text",
					Value:   configv.MovieRatingXPath,
					Options: nil,
				},
				{
					Name:    "MovieTitleAttribute",
					Type:    "text",
					Value:   configv.MovieTitleAttribute,
					Options: nil,
				},
				{
					Name:    "MovieURLAttribute",
					Type:    "text",
					Value:   configv.MovieURLAttribute,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - Pagination Settings
		renderConfigGroupWithParent(
			"Movie Scraper - Pagination Settings",
			"moviescraper-pagination-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:  "MoviePaginationType",
					Type:  "select",
					Value: configv.MoviePaginationType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"", "sequential", "offset"},
					}),
				},
				{
					Name:    "MoviePageIncrement",
					Type:    "number",
					Value:   configv.MoviePageIncrement,
					Options: nil,
				},
				{
					Name:    "MoviePageURLPattern",
					Type:    "text",
					Value:   configv.MoviePageURLPattern,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - CSRF API Settings
		renderConfigGroupWithParent(
			"Movie Scraper - CSRF API Settings",
			"moviescraper-csrfapi-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MovieCSRFCookieName",
					Type:    "text",
					Value:   configv.MovieCSRFCookieName,
					Options: nil,
				},
				{
					Name:    "MovieCSRFHeaderName",
					Type:    "text",
					Value:   configv.MovieCSRFHeaderName,
					Options: nil,
				},
				{
					Name:    "MovieAPIURLPattern",
					Type:    "text",
					Value:   configv.MovieAPIURLPattern,
					Options: nil,
				},
				{
					Name:    "MoviePageStartIndex",
					Type:    "number",
					Value:   configv.MoviePageStartIndex,
					Options: nil,
				},
				{
					Name:    "MovieResultsArrayPath",
					Type:    "text",
					Value:   configv.MovieResultsArrayPath,
					Options: nil,
				},
				{
					Name:    "MovieTitleField",
					Type:    "text",
					Value:   configv.MovieTitleField,
					Options: nil,
				},
				{Name: "MovieYearField", Type: "text", Value: configv.MovieYearField, Options: nil},
				{
					Name:    "MovieImdbIDField",
					Type:    "text",
					Value:   configv.MovieImdbIDField,
					Options: nil,
				},
				{Name: "MovieURLField", Type: "text", Value: configv.MovieURLField, Options: nil},
				{
					Name:    "MovieRatingField",
					Type:    "text",
					Value:   configv.MovieRatingField,
					Options: nil,
				},
				{
					Name:    "MovieGenreField",
					Type:    "text",
					Value:   configv.MovieGenreField,
					Options: nil,
				},
				{
					Name:    "MovieReleaseDateField",
					Type:    "text",
					Value:   configv.MovieReleaseDateField,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Movie Scraper - Common Settings
		renderConfigGroupWithParent(
			"Movie Scraper - Common Settings",
			"moviescraper-common-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "MovieDateFormat",
					Type:    "text",
					Value:   configv.MovieDateFormat,
					Options: nil,
				},
				{
					Name:    "MovieWaitSeconds",
					Type:    "number",
					Value:   configv.MovieWaitSeconds,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Chart / Bestseller Scraper Settings
		renderConfigGroupWithParent(
			"Chart / Bestseller Scraper Settings",
			"chartscraper-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{
					Name:    "ChartEntryNodeXPath",
					Type:    "text",
					Value:   configv.ChartEntryNodeXPath,
					Options: nil,
				},
				{
					Name:    "ChartTitleXPath",
					Type:    "text",
					Value:   configv.ChartTitleXPath,
					Options: nil,
				},
				{
					Name:    "ChartArtistXPath",
					Type:    "text",
					Value:   configv.ChartArtistXPath,
					Options: nil,
				},
				{
					Name:    "ChartTitleAttribute",
					Type:    "text",
					Value:   configv.ChartTitleAttribute,
					Options: nil,
				},
				{
					Name:    "ChartArtistAttribute",
					Type:    "text",
					Value:   configv.ChartArtistAttribute,
					Options: nil,
				},
				{
					Name:    "ChartDefaultArtist",
					Type:    "text",
					Value:   configv.ChartDefaultArtist,
					Options: nil,
				},
				{
					Name:    "ChartDateURLPattern",
					Type:    "text",
					Value:   configv.ChartDateURLPattern,
					Options: nil,
				},
				{
					Name:    "ChartDateFormat",
					Type:    "text",
					Value:   configv.ChartDateFormat,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// IRC Announce Settings
		renderConfigGroupWithParent(
			"IRC Announce Settings",
			"irc-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "IRCServer", Type: "text", Value: configv.IRCServer, Options: nil},
				{Name: "IRCUseTLS", Type: "checkbox", Value: configv.IRCUseTLS, Options: nil},
				{Name: "IRCNick", Type: "text", Value: configv.IRCNick, Options: nil},
				{
					Name:    "IRCNickServPassword",
					Type:    "password",
					Value:   configv.IRCNickServPassword,
					Options: nil,
				},
				{
					Name:    "IRCInviteCommand",
					Type:    "text",
					Value:   configv.IRCInviteCommand,
					Options: nil,
				},
				{Name: "IRCChannels", Type: "array", Value: configv.IRCChannels, Options: nil},
				{
					Name:    "IRCAnnounceNicks",
					Type:    "array",
					Value:   configv.IRCAnnounceNicks,
					Options: nil,
				},
				{
					Name:    "IRCAnnounceRegex",
					Type:    "text",
					Value:   configv.IRCAnnounceRegex,
					Options: nil,
				},
				{
					Name:    "IRCReadSeconds",
					Type:    "number",
					Value:   configv.IRCReadSeconds,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderListsConfig renders the lists configuration section.
func renderListsConfig(configv []config.ListsConfig, csrfToken string) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Lists Configuration",
		Subtitle:       "Manage media lists and collections for organizing movies and TV series into custom groups with specific rules and criteria.",
		Icon:           "list",
		AddButtonText:  "Add List",
		AddEndpoint:    "/api/manage/lists/form",
		FormContainer:  "listsContainer",
		SubmitEndpoint: "/api/admin/config/list/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(i int, config config.ListsConfig, _ string) gomponents.Node {
			return renderListsForm(i, &config, csrfToken)
		},
	)
}

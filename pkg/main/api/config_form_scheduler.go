package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderSchedulerForm renders one row; idx is the row index used in the form field names.
func renderSchedulerForm(idx int, configv *config.SchedulerConfig) gomponents.Node {
	group := "scheduler_" + strconv.Itoa(idx)
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)

	collapseId := group + "_main_collapse"

	return html.Div(
		html.Class(ClassArrayItem),
		html.Style("margin: 10px;"),
		html.Button(
			html.Class("card-header accordion-button w-100 border-0"),
			html.Style(
				"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 0.75rem 1rem; font-weight: 600; text-align: left; cursor: pointer;",
			),
			html.Type("button"),
			gomponents.Attr("data-bs-toggle", "collapse"),
			gomponents.Attr("data-bs-target", "#"+collapseId),
			gomponents.Attr("aria-expanded", "true"),
			gomponents.Attr("aria-controls", collapseId),
			html.I(html.Class("fas fa-clock me-2 text-primary")),
			gomponents.Text("Scheduler "+configv.Name),
		),
		html.Div(
			html.ID(collapseId),
			html.Class(ClassCollapse),
			html.Div(
				html.Class(ClassCardBody),
				// Organized scheduler sections
				renderSchedulerConfigSections(configv, group, comments, displayNames),
			),
		),
	)
}

// renderSchedulerConfigSections organizes scheduler fields into logical groups.
func renderSchedulerConfigSections(
	configv *config.SchedulerConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	return html.Div(
		html.Class("accordion"),
		html.ID(
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Basic Settings
		renderConfigGroupWithParent(
			"Basic Settings",
			"basic-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{Name: "Name", Type: "text", Value: configv.Name},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Feed Intervals
		renderConfigGroupWithParent(
			"Feed Scheduling",
			"feeds-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "IntervalFeeds", Type: "text", Value: configv.IntervalFeeds},
				{Name: "CronFeeds", Type: "text", Value: configv.CronFeeds},
				{
					Name:  "IntervalFeedsRefreshSeries",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshSeries,
				},
				{
					Name:  "CronFeedsRefreshSeries",
					Type:  "text",
					Value: configv.CronFeedsRefreshSeries,
				},
				{
					Name:  "IntervalFeedsRefreshMovies",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshMovies,
				},
				{
					Name:  "CronFeedsRefreshMovies",
					Type:  "text",
					Value: configv.CronFeedsRefreshMovies,
				},
				{
					Name:  "IntervalFeedsRefreshSeriesFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshSeriesFull,
				},
				{
					Name:  "CronFeedsRefreshSeriesFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshSeriesFull,
				},
				{
					Name:  "IntervalFeedsRefreshMoviesFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshMoviesFull,
				},
				{
					Name:  "CronFeedsRefreshMoviesFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshMoviesFull,
				},
				{
					Name:  "IntervalFeedsRefreshBooks",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshBooks,
				},
				{
					Name:  "CronFeedsRefreshBooks",
					Type:  "text",
					Value: configv.CronFeedsRefreshBooks,
				},
				{
					Name:  "IntervalFeedsRefreshBooksFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshBooksFull,
				},
				{
					Name:  "CronFeedsRefreshBooksFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshBooksFull,
				},
				{
					Name:  "IntervalFeedsRefreshAudiobooks",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshAudiobooks,
				},
				{
					Name:  "CronFeedsRefreshAudiobooks",
					Type:  "text",
					Value: configv.CronFeedsRefreshAudiobooks,
				},
				{
					Name:  "IntervalFeedsRefreshAudiobooksFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshAudiobooksFull,
				},
				{
					Name:  "CronFeedsRefreshAudiobooksFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshAudiobooksFull,
				},
				{
					Name:  "IntervalFeedsRefreshMusic",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshMusic,
				},
				{
					Name:  "CronFeedsRefreshMusic",
					Type:  "text",
					Value: configv.CronFeedsRefreshMusic,
				},
				{
					Name:  "IntervalFeedsRefreshMusicFull",
					Type:  "text",
					Value: configv.IntervalFeedsRefreshMusicFull,
				},
				{
					Name:  "CronFeedsRefreshMusicFull",
					Type:  "text",
					Value: configv.CronFeedsRefreshMusicFull,
				},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Indexer Intervals
		renderConfigGroupWithParent(
			"Indexer Scheduling",
			"indexer-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{
					Name:  "IntervalIndexerMissing",
					Type:  "text",
					Value: configv.IntervalIndexerMissing,
				},
				{Name: "CronIndexerMissing", Type: "text", Value: configv.CronIndexerMissing},
				{
					Name:  "IntervalIndexerUpgrade",
					Type:  "text",
					Value: configv.IntervalIndexerUpgrade,
				},
				{Name: "CronIndexerUpgrade", Type: "text", Value: configv.CronIndexerUpgrade},
				{
					Name:  "IntervalIndexerMissingFull",
					Type:  "text",
					Value: configv.IntervalIndexerMissingFull,
				},
				{
					Name:  "CronIndexerMissingFull",
					Type:  "text",
					Value: configv.CronIndexerMissingFull,
				},
				{
					Name:  "IntervalIndexerUpgradeFull",
					Type:  "text",
					Value: configv.IntervalIndexerUpgradeFull,
				},
				{
					Name:  "CronIndexerUpgradeFull",
					Type:  "text",
					Value: configv.CronIndexerUpgradeFull,
				},
				{Name: "IntervalIndexerRss", Type: "text", Value: configv.IntervalIndexerRss},
				{Name: "CronIndexerRss", Type: "text", Value: configv.CronIndexerRss},
				{
					Name:  "IntervalIndexerRssSeasons",
					Type:  "text",
					Value: configv.IntervalIndexerRssSeasons,
				},
				{Name: "CronIndexerRssSeasons", Type: "text", Value: configv.CronIndexerRssSeasons},
				{
					Name:  "IntervalIndexerRssSeasonsAll",
					Type:  "text",
					Value: configv.IntervalIndexerRssSeasonsAll,
				},
				{
					Name:  "CronIndexerRssSeasonsAll",
					Type:  "text",
					Value: configv.CronIndexerRssSeasonsAll,
				},
				{
					Name:  "IntervalIndexerRssArtists",
					Type:  "text",
					Value: configv.IntervalIndexerRssArtists,
				},
				{
					Name:  "CronIndexerRssArtists",
					Type:  "text",
					Value: configv.CronIndexerRssArtists,
				},
				{
					Name:  "IntervalIndexerRssArtistsUpgrade",
					Type:  "text",
					Value: configv.IntervalIndexerRssArtistsUpgrade,
				},
				{
					Name:  "CronIndexerRssArtistsUpgrade",
					Type:  "text",
					Value: configv.CronIndexerRssArtistsUpgrade,
				},
				{
					Name:  "IntervalIndexerRssAuthors",
					Type:  "text",
					Value: configv.IntervalIndexerRssAuthors,
				},
				{
					Name:  "CronIndexerRssAuthors",
					Type:  "text",
					Value: configv.CronIndexerRssAuthors,
				},
				{
					Name:  "IntervalIndexerRssAuthorsUpgrade",
					Type:  "text",
					Value: configv.IntervalIndexerRssAuthorsUpgrade,
				},
				{
					Name:  "CronIndexerRssAuthorsUpgrade",
					Type:  "text",
					Value: configv.CronIndexerRssAuthorsUpgrade,
				},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Title Search Scheduling
		renderConfigGroupWithParent(
			"Title Search Scheduling",
			"titles-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{
					Name:  "IntervalIndexerMissingTitle",
					Type:  "text",
					Value: configv.IntervalIndexerMissingTitle,
				},
				{
					Name:  "CronIndexerMissingTitle",
					Type:  "text",
					Value: configv.CronIndexerMissingTitle,
				},
				{
					Name:  "IntervalIndexerUpgradeTitle",
					Type:  "text",
					Value: configv.IntervalIndexerUpgradeTitle,
				},
				{
					Name:  "CronIndexerUpgradeTitle",
					Type:  "text",
					Value: configv.CronIndexerUpgradeTitle,
				},
				{
					Name:  "IntervalIndexerMissingFullTitle",
					Type:  "text",
					Value: configv.IntervalIndexerMissingFullTitle,
				},
				{
					Name:  "CronIndexerMissingFullTitle",
					Type:  "text",
					Value: configv.CronIndexerMissingFullTitle,
				},
				{
					Name:  "IntervalIndexerUpgradeFullTitle",
					Type:  "text",
					Value: configv.IntervalIndexerUpgradeFullTitle,
				},
				{
					Name:  "CronIndexerUpgradeFullTitle",
					Type:  "text",
					Value: configv.CronIndexerUpgradeFullTitle,
				},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),

		// Data Scanning & Maintenance
		renderConfigGroupWithParent(
			"Data Scanning & Maintenance",
			"maintenance-"+strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-"),
			false,
			[]FormFieldDefinition{
				{Name: "IntervalImdb", Type: "text", Value: configv.IntervalImdb},
				{Name: "CronImdb", Type: "text", Value: configv.CronImdb},
				{Name: "IntervalScanData", Type: "text", Value: configv.IntervalScanData},
				{Name: "CronScanData", Type: "text", Value: configv.CronScanData},
				{
					Name:  "IntervalScanDataMissing",
					Type:  "text",
					Value: configv.IntervalScanDataMissing,
				},
				{Name: "CronScanDataMissing", Type: "text", Value: configv.CronScanDataMissing},
				{Name: "IntervalScanDataFlags", Type: "text", Value: configv.IntervalScanDataFlags},
				{Name: "CronScanDataFlags", Type: "text", Value: configv.CronScanDataFlags},
				{
					Name:  "IntervalScanDataimport",
					Type:  "text",
					Value: configv.IntervalScanDataimport,
				},
				{Name: "CronScanDataimport", Type: "text", Value: configv.CronScanDataimport},
				{
					Name:  "IntervalDatabaseBackup",
					Type:  "text",
					Value: configv.IntervalDatabaseBackup,
				},
				{Name: "CronDatabaseBackup", Type: "text", Value: configv.CronDatabaseBackup},
				{Name: "IntervalDatabaseCheck", Type: "text", Value: configv.IntervalDatabaseCheck},
				{Name: "CronDatabaseCheck", Type: "text", Value: configv.CronDatabaseCheck},
				{Name: "IntervalCacheRefresh", Type: "text", Value: configv.IntervalCacheRefresh},
				{Name: "CronCacheRefresh", Type: "text", Value: configv.CronCacheRefresh},
				{Name: "IntervalDownloadCheck", Type: "text", Value: configv.IntervalDownloadCheck},
				{Name: "CronDownloadCheck", Type: "text", Value: configv.CronDownloadCheck},
				{Name: "IntervalMovieCollections", Type: "text", Value: configv.IntervalMovieCollections},
				{Name: "CronMovieCollections", Type: "text", Value: configv.CronMovieCollections},
				{Name: "IntervalFileCacheCleanup", Type: "text", Value: configv.IntervalFileCacheCleanup},
				{Name: "CronFileCacheCleanup", Type: "text", Value: configv.CronFileCacheCleanup},
			},
			group,
			comments,
			displayNames,
			"schedulerConfigAccordion-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
		),
	)
}

// renderSchedulerConfig renders the scheduler configuration section
// renderSchedulerConfig renders the scheduler configuration section.
func renderSchedulerConfig(configv []config.SchedulerConfig, csrfToken string) gomponents.Node {
	options := ConfigSectionOptions{
		SectionTitle:    "Scheduler Configuration",
		SectionSubtitle: "Configure automated scheduling rules and timing for various system tasks, jobs, and maintenance operations.",
		SectionIcon:     "clock",
		ContainerID:     "schedulerContainer",
		AddButtonText:   "Add Scheduler",
		AddFormPath:     "/api/manage/scheduler/form",
		UpdatePath:      "/api/admin/config/scheduler/update",
	}

	return renderConfigSection(configv, csrfToken, options, renderSchedulerForm)
}

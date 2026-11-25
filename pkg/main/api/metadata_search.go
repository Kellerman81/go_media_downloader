package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// metadataSearchTab is one media type of the Add Media page.
type metadataSearchTab struct {
	Key, Label, Icon string
	configured       func(*config.MediaConfig) bool
	content          func(csrfToken string, acc *userAccess) gomponents.Node
	kind             string // getMediaListsByType's media type
}

var metadataSearchTabs = []metadataSearchTab{
	{"movies", t("Movies"), "fa-film", func(m *config.MediaConfig) bool { return len(m.Movies) > 0 },
		func(csrf string, acc *userAccess) gomponents.Node {
			return movieMetadataSearchContent(getMediaListsByType(acc, "movie"), getQualityProfileNames(), csrf)
		}, "movie"},
	{"series", t("Series"), "fa-tv", func(m *config.MediaConfig) bool { return len(m.Series) > 0 },
		func(csrf string, acc *userAccess) gomponents.Node {
			return seriesMetadataSearchContent(getMediaListsByType(acc, "series"), getQualityProfileNames(), csrf)
		}, "series"},
	{"books", t("Books"), "fa-book", func(m *config.MediaConfig) bool { return len(m.Books) > 0 },
		func(csrf string, acc *userAccess) gomponents.Node {
			return bookMetadataSearchContent(getMediaListsByType(acc, "book"), getQualityProfileNames(), csrf)
		}, "book"},
	{"audiobooks", t("Audiobooks"), "fa-headphones", func(m *config.MediaConfig) bool { return len(m.AudioBooks) > 0 },
		func(csrf string, acc *userAccess) gomponents.Node {
			return audiobookMetadataSearchContent(getMediaListsByType(acc, "audiobook"), getQualityProfileNames(), csrf)
		}, "audiobook"},
	{"music", t("Music"), "fa-music", func(m *config.MediaConfig) bool { return len(m.Music) > 0 },
		func(csrf string, acc *userAccess) gomponents.Node {
			return musicMetadataSearchContent(getMediaListsByType(acc, "music"), csrf)
		}, "music"},
}

// MetadataSearchPage renders the Add Media page: a tab per configured media
// type and the search form of the selected one (?type=movies|series|books|
// audiobooks|music). Only one form is on the page at a time, as the forms
// share element ids and scripts.
func MetadataSearchPage(c *gin.Context) {
	var tabs []metadataSearchTab

	acc := accessOf(c)

	if media := config.GetSettingsMediaAll(); media != nil {
		for _, t := range metadataSearchTabs {
			if t.configured(media) && len(getMediaListsByType(acc, t.kind)) > 0 {
				tabs = append(tabs, t)
			}
		}
	}

	var content gomponents.Node = gomponents.Raw(renderAlert(t("No media configuration yet - add one under Configuration → Media first."), "info"))

	if len(tabs) > 0 {
		active := tabs[0]
		for _, t := range tabs {
			if t.Key == c.Query("type") {
				active = t
			}
		}

		items := make([]gomponents.Node, 0, len(tabs))
		for _, tab := range tabs {
			class := "nav-link"
			if tab.Key == active.Key {
				class += " active"
			}

			items = append(items, html.Li(html.Class("nav-item"),
				html.A(html.Class(class), html.Href("/api/admin/metadata-search?type="+tab.Key),
					html.I(html.Class("fa-solid "+tab.Icon+" me-1")), gomponents.Text(t(tab.Label))),
			))
		}

		content = html.Div(
			html.Ul(html.Class("nav nav-tabs mb-4"), gomponents.Group(items)),
			active.content(getCSRFToken(c), acc),
			// Manual entries (movies, series cards; book and audiobook tabs)
			// are for admins: a request needs a search result.
			gomponents.If(!acc.Admin(), html.StyleEl(gomponents.Raw(
				".meta-manual, #bk4-tab, #ab4-tab, #bk4-pane, #ab4-pane { display: none !important; }"))),
			// Non-admins get their groups' default lists; one is chosen for them.
			gomponents.If(!acc.Admin(), html.Script(gomponents.Raw(`document.querySelectorAll('select[name$="_list"]').forEach(function (s) {
	var opts = [].filter.call(s.options, function (o) { return o.value !== ''; });
	if (opts.length === 1 && !s.value) { s.value = opts[0].value; s.dispatchEvent(new Event('change', {bubbles: true})); }
});`))),
		)
	}

	pageNode := pageAs(acc,
		t("Add Media"),
		false, // activeConfig
		false, // activeDatabase
		true,  // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// metadataSearchPrefixes are the media config name prefixes of the Add
// Media types.
var metadataSearchPrefixes = map[string]string{
	"movie": "movie_", "series": "serie_", "book": "book_", "audiobook": "audiobook_", "music": "music_",
}

// defaultRequestList is the list media requested for a group goes to: its
// add_found_list, else its first list ("" without lists).
func defaultRequestList(media *config.MediaTypeConfig) string {
	for _, d := range media.Data {
		if d.AddFoundList == "" {
			continue
		}

		for i := range media.Lists {
			if strings.EqualFold(media.Lists[i].Name, d.AddFoundList) {
				return media.Lists[i].Name
			}
		}
	}

	if len(media.Lists) > 0 {
		return media.Lists[0].Name
	}

	return ""
}

// getMediaListsByType returns the list names of one media type (movie,
// series, book, audiobook, music) the account may add to (acc nil: all).
// Non-admins get only the default list of each of their groups: they don't
// know the lists.
func getMediaListsByType(acc *userAccess, mediaType string) []string {
	kind := map[string]string{"series": "serie"}[mediaType]
	if kind == "" {
		kind = mediaType
	}

	lists := make([]string, 0)

	for _, media := range mediaEntriesForKind(kind) {
		if acc != nil && !acc.AllowsGroup(metadataSearchPrefixes[mediaType]+media.Name) {
			continue
		}

		if acc != nil && !acc.Admin() {
			if l := defaultRequestList(&media); l != "" {
				lists = append(lists, l)
			}

			continue
		}

		for _, list := range media.Lists {
			lists = append(lists, list.Name)
		}
	}

	return lists
}

// getQualityProfileNames returns all quality profile names.
func getQualityProfileNames() []string {
	qualityConfigs := config.GetSettingsQualityAll()
	names := make([]string, 0, len(qualityConfigs))

	for _, qc := range qualityConfigs {
		names = append(names, qc.Name)
	}

	return names
}

// listQualityProfiles returns the quality profiles a list can be added with -
// the ones its media group is configured with (the group template plus every
// list's template), in quality-config order - and the list's own default.
func listQualityProfiles(cfgp *config.MediaTypeConfig, listid int) ([]string, string) {
	def := cfgp.Lists[listid].TemplateQuality
	if def == "" {
		def = cfgp.TemplateQuality
	}

	return configQualityProfiles(cfgp), def
}

// configQualityProfiles returns the quality profiles a media config is set up
// with - its group template plus every list's template - in config order.
func configQualityProfiles(cfgp *config.MediaTypeConfig) []string {
	used := map[string]struct{}{cfgp.TemplateQuality: {}}
	for i := range cfgp.Lists {
		used[cfgp.Lists[i].TemplateQuality] = struct{}{}
	}

	names := getQualityProfileNames()
	allowed := make([]string, 0, len(used))

	for _, name := range names {
		if _, ok := used[name]; ok {
			allowed = append(allowed, name)
		}
	}

	return allowed
}

// configuredListQuality rejects a quality profile the list's media group is
// not configured with; an empty one is left for the caller to default. On
// failure it writes a 400 and returns false.
func configuredListQuality(c *gin.Context, listName, kind, quality string) bool {
	if quality == "" {
		return true
	}

	if cfgp, listid := findCfgpAndListID(listName, kind+"_", mediaEntriesForKind(kind)); cfgp != nil && listid >= 0 {
		if allowed, _ := listQualityProfiles(cfgp, listid); slices.Contains(allowed, quality) {
			return true
		}
	}

	c.JSON(http.StatusBadRequest, gin.H{"error": tf("Quality profile '%s' is not configured for list '%s'", quality, listName)})

	return false
}

// listQualityScript narrows each quality select to the profiles configured for
// the list chosen in its paired list select and preselects the list's default.
// pairs maps list select id to quality select id; only the offered lists
// (the account's) are included.
func listQualityScript(kind string, lists []string, pairs map[string]string) gomponents.Node {
	byList := make(map[string]selectQuality)

	for _, media := range mediaEntriesForKind(kind) {
		cfgp := config.GetSettingsMedia(kind + "_" + media.Name)
		if cfgp == nil {
			continue
		}

		for i := range cfgp.Lists {
			if _, ok := byList[cfgp.Lists[i].Name]; ok || !slices.Contains(lists, cfgp.Lists[i].Name) {
				continue
			}

			profiles, def := listQualityProfiles(cfgp, i)
			byList[cfgp.Lists[i].Name] = selectQuality{Profiles: profiles, Default: def}
		}
	}

	return qualitySelectScript(byList, pairs, t("Select a list first"))
}

// selectQuality is the quality profiles offered for one value of a source
// select (a list or a media config) and the one preselected.
type selectQuality struct {
	Profiles []string `json:"q"`
	Default  string   `json:"d"`
}

// qualitySelectScript narrows each quality select to the profiles of the value
// chosen in its paired source select. pairs maps source select id to quality
// select id; placeholder is shown while the source has no known value.
func qualitySelectScript(byValue map[string]selectQuality, pairs map[string]string, placeholder string) gomponents.Node {
	// json.Marshal escapes <, > and &, so the data is safe inside <script>.
	valueJSON, _ := json.Marshal(byValue)
	pairJSON, _ := json.Marshal(pairs)
	placeholderJSON, _ := json.Marshal(placeholder)

	return html.Script(gomponents.Raw(`
(function() {
	var byList = ` + string(valueJSON) + `, pairs = ` + string(pairJSON) + `, placeholder = ` + string(placeholderJSON) + `;
	function fill(listSel, qSel) {
		// A leading empty option (e.g. "use the list's default") is kept.
		if (qSel.dataset.blank === undefined) {
			var first = qSel.options[0];
			qSel.dataset.blank = (first && first.value === '') ? first.text : '';
			qSel.dataset.hasBlank = (first && first.value === '') ? '1' : '';
		}
		var entry = byList[listSel.value];
		qSel.innerHTML = '';
		if (!entry) {
			qSel.add(new Option(placeholder, ''));
			qSel.disabled = true;
			return;
		}
		qSel.disabled = false;
		if (qSel.dataset.hasBlank) {
			qSel.add(new Option(qSel.dataset.blank, ''));
		}
		entry.q.forEach(function(name) { qSel.add(new Option(name, name)); });
		qSel.value = entry.q.indexOf(entry.d) >= 0 ? entry.d : (entry.q[0] || '');
	}
	Object.keys(pairs).forEach(function(listID) {
		var listSel = document.getElementById(listID), qSel = document.getElementById(pairs[listID]);
		if (!listSel || !qSel) {
			return;
		}
		listSel.addEventListener('change', function() { fill(listSel, qSel); });
		fill(listSel, qSel);
	});
})();
`))
}

func movieMetadataSearchContent(
	mediaConfigs, qualityProfiles []string,
	csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-search header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(t("Movie Metadata Search"))),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							t("Search for movies across metadata sources and add them to your library"),
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Search Form Card
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text(t("Search Movies")),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("movieSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("movie_title"),
										html.Class("form-label"),
										gomponents.Text(t("Movie Title")),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("movie_title"),
										html.Name("movie_title"),
										html.Placeholder(t("Enter movie title to search...")),
										gomponents.Attr("required", "true"),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("movie_list"),
											html.Class("form-label"),
											gomponents.Text(t("Add to List")),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("movie_list"),
											html.Name("movie_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("movie_quality_profile"),
											html.Class("form-label"),
											gomponents.Text(t("Quality Profile")),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("movie_quality_profile"),
											html.Name("movie_quality_profile"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(qualityProfiles, ""),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text(t("Search Movies")),
									),
								),
							),
						),
					),
				),

				// Manual Entry Card (admins only: meta-manual)
				html.Div(
					html.Class("col-lg-6 meta-manual"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-edit me-2")),
								gomponents.Text(t("Manual Entry")),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("movieManualForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-8 mb-3"),
										html.Label(
											html.For("manualMovie_title"),
											html.Class("form-label"),
											gomponents.Text(t("Movie Title *")),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualMovie_title"),
											html.Name("manualMovie_title"),
											html.Placeholder(t("Enter movie title")),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualMovie_year"),
											html.Class("form-label"),
											gomponents.Text(t("Year")),
										),
										html.Input(
											html.Type("number"),
											html.Class("form-control"),
											html.ID("manualMovie_year"),
											html.Name("manualMovie_year"),
											html.Placeholder(t("YYYY")),
											gomponents.Attr("min", "1900"),
											gomponents.Attr("max", "2030"),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_list"),
											html.Class("form-label"),
											gomponents.Text(t("Add to List *")),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualMovie_list"),
											html.Name("manualMovie_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_quality_profile"),
											html.Class("form-label"),
											gomponents.Text(t("Quality Profile *")),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualMovie_quality_profile"),
											html.Name("manualMovie_quality_profile"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(qualityProfiles, ""),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_imdb_id"),
											html.Class("form-label"),
											gomponents.Text(t("IMDB ID")),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualMovie_imdb_id"),
											html.Name("manualMovie_imdb_id"),
											html.Placeholder(t("tt1234567")),
											gomponents.Attr("pattern", "tt[0-9]+"),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualMovie_overview"),
											html.Class("form-label"),
											gomponents.Text(t("Overview")),
										),
										html.Textarea(
											html.Class("form-control"),
											html.ID("manualMovie_overview"),
											html.Name("manualMovie_overview"),
											html.Placeholder(t("Movie description (optional)")),
											html.Rows("2"),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-secondary"),
										html.I(html.Class("fas fa-plus me-2")),
										gomponents.Text(t("Add Movie Manually")),
									),
								),
							),
						),
					),
				),
			),

			// Results area
			html.Div(
				html.ID("movieSearchResults"),
				html.Class("mt-4"),
			),

			movieSearchScript(csrfToken),
			listQualityScript("movie", mediaConfigs, map[string]string{
				"movie_list":       "movie_quality_profile",
				"manualMovie_list": "manualMovie_quality_profile",
			}),
		),
	)
}

func seriesMetadataSearchContent(
	mediaConfigs, qualityProfiles []string,
	csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-tv header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(t("Series Metadata Search"))),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							t("Search for TV series across metadata sources and add them to your library"),
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Search Form Card
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text(t("Search Series")),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("seriesSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("series_title"),
										html.Class("form-label"),
										gomponents.Text(t("Series Title")),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("series_title"),
										html.Name("series_title"),
										html.Placeholder(t("Enter series title to search...")),
										gomponents.Attr("required", "true"),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("series_list"),
										html.Class("form-label"),
										gomponents.Text(t("Add to List")),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("series_list"),
										html.Name("series_list"),
										gomponents.Attr("required", "true"),
										renderSelectOptions(mediaConfigs, ""),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("series_quality_profile"),
										html.Class("form-label"),
										gomponents.Text(t("Quality Profile")),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("series_quality_profile"),
										html.Name("series_quality_profile"),
										renderSelectOptions(qualityProfiles, ""),
									),
									html.Small(
										html.Class("form-text text-muted"),
										gomponents.Text(
											t("Leave to use the list's default quality."),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text(t("Search Series")),
									),
								),
							),
						),
					),
				),

				// Manual Entry Card (admins only: meta-manual)
				html.Div(
					html.Class("col-lg-6 meta-manual"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-edit me-2")),
								gomponents.Text(t("Manual Entry")),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("seriesManualForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-8 mb-3"),
										html.Label(
											html.For("manualSeries_seriename"),
											html.Class("form-label"),
											gomponents.Text(t("Series Name *")),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualSeries_seriename"),
											html.Name("manualSeries_seriename"),
											html.Placeholder(t("Enter series name")),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_list"),
											html.Class("form-label"),
											gomponents.Text(t("Add to List *")),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualSeries_list"),
											html.Name("manualSeries_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_thetvdb_id"),
											html.Class("form-label"),
											gomponents.Text(t("TVDB ID")),
										),
										html.Input(
											html.Type("number"),
											html.Class("form-control"),
											html.ID("manualSeries_thetvdb_id"),
											html.Name("manualSeries_thetvdb_id"),
											html.Placeholder("123456"),
											gomponents.Attr("min", "1"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_firstaired"),
											html.Class("form-label"),
											gomponents.Text(t("First Aired")),
										),
										html.Input(
											html.Type("date"),
											html.Class("form-control"),
											html.ID("manualSeries_firstaired"),
											html.Name("manualSeries_firstaired"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualSeries_network"),
											html.Class("form-label"),
											gomponents.Text(t("Network")),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualSeries_network"),
											html.Name("manualSeries_network"),
											html.Placeholder(t("ABC, NBC, etc.")),
										),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("manualSeries_overview"),
										html.Class("form-label"),
										gomponents.Text(t("Overview")),
									),
									html.Textarea(
										html.Class("form-control"),
										html.ID("manualSeries_overview"),
										html.Name("manualSeries_overview"),
										html.Placeholder(t("Series description (optional)")),
										html.Rows("3"),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-secondary"),
										html.I(html.Class("fas fa-plus me-2")),
										gomponents.Text(t("Add Series Manually")),
									),
								),
							),
						),
					),
				),
			),

			// Results area
			html.Div(
				html.ID("seriesSearchResults"),
				html.Class("mt-4"),
			),

			seriesSearchScript(csrfToken),
			listQualityScript("serie", mediaConfigs, map[string]string{"series_list": "series_quality_profile"}),
		),
	)
}

// renderSelectOptions renders options for a select element.
func renderSelectOptions(options []string, selectedValue string) gomponents.Node {
	nodes := make([]gomponents.Node, 0, len(options)+1)

	nodes = append(nodes, html.Option(html.Value(""), gomponents.Text(t("Select..."))))

	for _, opt := range options {
		if opt == selectedValue {
			nodes = append(
				nodes,
				html.Option(
					html.Value(opt),
					gomponents.Attr("selected", "selected"),
					gomponents.Text(opt),
				),
			)
		} else {
			nodes = append(nodes, html.Option(html.Value(opt), gomponents.Text(opt)))
		}
	}

	return gomponents.Group(nodes)
}

// SearchMovieMetadata handles AJAX movie search requests.
func SearchMovieMetadata(c *gin.Context) {
	title := c.PostForm("movie_title")
	if title == "" {
		c.String(http.StatusBadRequest, renderAlert(t("Title is required"), "warning"))
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.String(http.StatusInternalServerError, renderAlert("TMDB provider not available", "danger"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.SearchMovies(ctx, title, 0)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search movies")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(
			http.StatusInternalServerError,
			renderAlert(t("Failed to search movies: ")+err.Error(), "danger"),
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text(tf("No movies found for: %s", title)),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(tf("Search Results (%d movies found)", len(results))),
	))

	lists := haveLists(c, "movie")
	for i := range results {
		resultNodes = append(resultNodes, createMovieResultCard(results[i], haveMovie(lists, &results[i])))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// SearchSeriesMetadata handles AJAX series search requests.
func SearchSeriesMetadata(c *gin.Context) {
	title := c.PostForm("series_title")
	if title == "" {
		c.String(http.StatusBadRequest, renderAlert(t("Title is required"), "warning"))
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.String(http.StatusInternalServerError, renderAlert("TMDB provider not available", "danger"))
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.SearchSeries(ctx, title, 0)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search series")
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(
			http.StatusInternalServerError,
			renderAlert(t("Failed to search series: ")+err.Error(), "danger"),
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text(tf("No series found for: %s", title)),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(tf("Search Results (%d series found)", len(results))),
	))

	lists := haveLists(c, "serie")
	for i := range results {
		resultNodes = append(resultNodes, createSeriesResultCard(results[i], haveSeries(lists, &results[i])))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

func createMovieResultCard(movie apiexternal_v2.MovieSearchResult, have bool) gomponents.Node {
	year := ""
	if movie.Year > 0 {
		year = fmt.Sprintf(" (%d)", movie.Year)
	}

	overview := movie.Overview
	if runes := []rune(overview); len(runes) > 200 {
		overview = string(runes[:200]) + "..."
	}

	rating := ""
	if movie.VoteAverage > 0 {
		rating = fmt.Sprintf("%.1f", movie.VoteAverage)
	}

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start gap-3"),
				metaCover(metaPosterURL(movie.PosterPath, movie.ProviderName), false),
				html.Div(
					html.Class("flex-grow-1"),
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(movie.Title+year),
					),
					func() gomponents.Node {
						if movie.OriginalTitle != "" && movie.OriginalTitle != movie.Title {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text(t("Original: ")+movie.OriginalTitle),
							)
						}

						return nil
					}(),
					html.P(
						html.Class("card-text"),
						gomponents.Text(overview),
					),
					func() gomponents.Node {
						if rating != "" {
							return html.Span(
								html.Class("badge bg-warning text-dark me-2"),
								html.I(html.Class("fas fa-star me-1")),
								gomponents.Text(rating),
							)
						}

						return nil
					}(),
					metaLinks(movieResultLinks(&movie)),
				),
				metaAddButton(have, "btn btn-success add-movie-btn",
					gomponents.Attr("data-tmdb-id", strconv.Itoa(movie.ID)),
					gomponents.Attr("data-title", movie.Title),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text(t("Add Movie")),
				),
			),
		),
	)
}

func createSeriesResultCard(series apiexternal_v2.SeriesSearchResult, have bool) gomponents.Node {
	year := ""
	if !series.FirstAirDate.IsZero() {
		year = fmt.Sprintf(" (%d)", series.FirstAirDate.Year())
	}

	overview := series.Overview
	if runes := []rune(overview); len(runes) > 200 {
		overview = string(runes[:200]) + "..."
	}

	rating := ""
	if series.VoteAverage > 0 {
		rating = fmt.Sprintf("%.1f", series.VoteAverage)
	}

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start gap-3"),
				metaCover(metaPosterURL(series.PosterPath, series.ProviderName), false),
				html.Div(
					html.Class("flex-grow-1"),
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(series.Name+year),
					),
					func() gomponents.Node {
						if series.OriginalName != "" && series.OriginalName != series.Name {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text(t("Original: ")+series.OriginalName),
							)
						}

						return nil
					}(),
					html.P(
						html.Class("card-text"),
						gomponents.Text(overview),
					),
					func() gomponents.Node {
						if rating != "" {
							return html.Span(
								html.Class("badge bg-warning text-dark me-2"),
								html.I(html.Class("fas fa-star me-1")),
								gomponents.Text(rating),
							)
						}

						return nil
					}(),
					metaLinks(seriesResultLinks(&series)),
				),
				metaAddButton(have, "btn btn-success add-series-btn",
					gomponents.Attr("data-tmdb-id", strconv.Itoa(series.ID)),
					gomponents.Attr("data-title", series.Name),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text(t("Add Series")),
				),
			),
		),
	)
}

// AddMovieToDatabase handles adding a movie from metadata sources to the database.
func AddMovieToDatabase(c *gin.Context) {
	tmdbIDStr := c.PostForm("tmdb_id")
	listName := c.PostForm("movie_list")
	qualityProfile := c.PostForm("movie_quality_profile")

	if tmdbIDStr == "" || listName == "" || qualityProfile == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": t("TMDB ID, list name, and quality profile are required")},
		)

		return
	}

	listName, ok := configuredListName(c, listName, "movie")
	if !ok {
		return
	}

	if !configuredListQuality(c, listName, "movie", c.PostForm("movie_quality_profile")) {
		return
	}

	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": t("Invalid TMDB ID")})
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": t("TMDB provider not initialized")})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lang string
	if cfgp := mediaConfigForList("movie", listName); cfgp != nil {
		lang = cfgp.MetadataLanguage
	}

	movieDetails, err := metadata.TmdbMovieDetails(ctx, tmdbID, lang)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to get movie details")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": t("Failed to get movie details: ") + err.Error()},
		)

		return
	}

	// Check if movie already exists in dbmovies: by TMDB id, or by its IMDb
	// id (a row imported from a list may not have its TMDB id yet).
	movieExists := database.GetrowsN[int](
		false,
		1,
		"SELECT id FROM dbmovies WHERE moviedb_id = ? OR (? != '' AND imdb_id = ?) ORDER BY id LIMIT 1",
		&tmdbID, &movieDetails.IMDbID, &movieDetails.IMDbID,
	)

	var dbMovieID int

	nowTime := time.Now()

	if len(movieExists) == 0 {
		newID, err := database.ExecNid(
			"INSERT INTO dbmovies (title, release_date, year, adult, budget, genres, original_language, original_title, overview, popularity, revenue, runtime, spoken_languages, status, tagline, vote_average, vote_count, moviedb_id, imdb_id, freebase_m_id, freebase_id, facebook_id, instagram_id, twitter_id, url, backdrop, poster, slug, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&movieDetails.Title,
			&movieDetails.ReleaseDate,
			getYearFromDate(movieDetails.ReleaseDate),
			&movieDetails.Adult,
			&movieDetails.Budget,
			genresToString(movieDetails.Genres),
			&movieDetails.OriginalLanguage,
			&movieDetails.OriginalTitle,
			&movieDetails.Overview,
			&movieDetails.Popularity,
			&movieDetails.Revenue,
			&movieDetails.Runtime,
			spokenLanguagesToString(movieDetails.SpokenLanguages),
			&movieDetails.Status,
			&movieDetails.Tagline,
			&movieDetails.VoteAverage,
			&movieDetails.VoteCount,
			&tmdbID,
			&movieDetails.IMDbID,
			"",
			"",
			"",
			"",
			"",
			"",
			&movieDetails.BackdropPath,
			&movieDetails.PosterPath,
			"",
			0,
			&nowTime,
			&nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert movie")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": t("Failed to add movie to database: ") + err.Error()},
			)

			return
		}

		dbMovieID = int(newID)
		newMovie := uint(newID) //nolint:gosec // a rowid
		metadata.SaveMovieCollection(newMovie, movieDetails.Collection)
		metadata.SaveMovieExtras(newMovie, movieDetails)
	} else {
		dbMovieID = movieExists[0]
	}

	// Check if movie is already in the specified list
	listExists := database.GetrowsN[int](
		false,
		1,
		"SELECT count() FROM movies WHERE dbmovie_id = ? AND listname = ? COLLATE NOCASE",
		&dbMovieID,
		&listName,
	)
	if len(listExists) > 0 && listExists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": tf("Movie already exists in list '%s'", listName)},
		)

		return
	}

	// Add to movies table
	if err := database.ExecNErr(
		"INSERT INTO movies (dbmovie_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, ?, 0, 0, 0, ?, ?)",
		&dbMovieID,
		&listName,
		&qualityProfile,
		&nowTime,
		&nowTime,
	); err != nil {
		// ExecN (the fire-and-forget variant previously used here) has no
		// return value at all - a write failure here (lock contention,
		// constraint violation, disk full) was reported to the user as
		// success even though the movie was never actually linked into the
		// requested list.
		c.JSON(http.StatusInternalServerError, gin.H{"error": t("Failed to add movie: ") + err.Error()})
		return
	}

	notifyManualAdd("movie", listName, uint(dbMovieID), len(movieExists) == 0) //nolint:gosec // row ids are positive

	c.JSON(http.StatusOK, gin.H{"success": t("Movie added successfully to ") + listName})
}

// AddSeriesToDatabase handles adding a series to the database.
func AddSeriesToDatabase(c *gin.Context) {
	tmdbIDStr := c.PostForm("tmdb_id")
	listName := c.PostForm("series_list")

	if tmdbIDStr == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": t("TMDB ID and list name are required")})
		return
	}

	listName, ok := configuredListName(c, listName, "serie")
	if !ok {
		return
	}

	if !configuredListQuality(c, listName, "serie", c.PostForm("series_quality_profile")) {
		return
	}

	tmdbID, err := strconv.Atoi(tmdbIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": t("Invalid TMDB ID")})
		return
	}

	provider := providers.GetTMDB()
	if provider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": t("TMDB provider not initialized")})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var lang string
	if cfgp, _ := findSeriesCfgpAndListID(listName); cfgp != nil {
		lang = cfgp.MetadataLanguage
	}

	series, err := provider.WithFallbackLanguage(lang).GetSeriesByID(ctx, tmdbID)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to get series details")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": t("Failed to get series details: ") + err.Error()},
		)

		return
	}

	tvdbID := series.TVDbID
	if tvdbID == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": t("TVDB ID not found for this series")})
		return
	}

	// Check if series already exists in dbseries
	seriesExists := database.GetrowsN[int](
		false,
		1,
		"SELECT id FROM dbseries WHERE thetvdb_id = ?",
		&tvdbID,
	)

	var dbSeriesID int

	nowTime := time.Now()

	if len(seriesExists) == 0 {
		newID, err := database.ExecNid(
			"INSERT INTO dbseries (seriename, season, status, firstaired, network, runtime, language, genre, overview, rating, siterating, siterating_count, slug, imdb_id, thetvdb_id, freebase_m_id, freebase_id, tvrage_id, facebook, instagram, twitter, banner, fanart, poster, identifiedby, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&series.Name,
			"",
			&series.Status,
			&series.FirstAirDate,
			getNetworkString(series.Networks),
			getRuntimeFromEpisodeRuntime(series.EpisodeRunTime),
			&series.OriginalLanguage,
			genresToString(series.Genres),
			&series.Overview,
			"",
			fmt.Sprintf("%.1f", series.VoteAverage),
			fmt.Sprintf("%d", series.VoteCount),
			"",
			&series.IMDbID,
			&tvdbID,
			"",
			"",
			0,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			0,
			&nowTime,
			&nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert series")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": t("Failed to add series to database: ") + err.Error()},
			)

			return
		}

		dbSeriesID = int(newID)
	} else {
		dbSeriesID = seriesExists[0]
	}

	// Check if series is already in the specified list
	listExists := database.GetrowsN[int](
		false,
		1,
		"SELECT count() FROM series WHERE dbserie_id = ? AND listname = ? COLLATE NOCASE",
		&dbSeriesID,
		&listName,
	)
	if len(listExists) > 0 && listExists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": tf("Series already exists in list '%s'", listName)},
		)

		return
	}

	// Resolve the wanted quality for this list so the series row (and the episodes
	// imported from it) carry the correct profile instead of an arbitrary one. An
	// explicitly selected profile wins; otherwise fall back to the list's quality
	// template, then the media group default.
	cfgp, listid := findSeriesCfgpAndListID(listName)

	qualityProfile := c.PostForm("series_quality_profile")
	if qualityProfile == "" && cfgp != nil && listid >= 0 {
		qualityProfile = cfgp.Lists[listid].TemplateQuality
		if qualityProfile == "" {
			qualityProfile = cfgp.TemplateQuality
		}
	}

	// Add to series table
	if err := database.ExecNErr(
		"INSERT INTO series (dbserie_id, listname, rootpath, quality_profile, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', ?, 0, 0, ?, ?)",
		&dbSeriesID,
		&listName,
		&qualityProfile,
		&nowTime,
		&nowTime,
	); err != nil {
		// ExecN (the fire-and-forget variant previously used here) has no
		// return value - a write failure here was reported to the user as
		// success even though the series was never linked into the list.
		c.JSON(http.StatusInternalServerError, gin.H{"error": t("Failed to add series: ") + err.Error()})
		return
	}

	// Kick off episode import in the background so the series is actually populated
	// (the previous flow added the series row but never imported any episodes).
	if cfgp != nil && listid >= 0 {
		serieName := series.Name

		worker.Dispatch(
			"add_series_"+strconv.Itoa(tvdbID)+"_"+listName,
			func(_ uint32, ctx context.Context) error {
				err := importfeed.JobImportDBSeries(
					ctx,
					&config.ManualConfig{Name: serieName, TvdbID: tvdbID},
					0,
					cfgp,
					listid,
				)
				logger.Logtype("info", 0).
					Str("series", serieName).
					Str("list", listName).
					Err(err).
					Msg("AddSeriesToDatabase: episode import completed")

				return nil
			},
			"Data",
		)
	}

	notifyManualAdd("serie", listName, uint(dbSeriesID), len(seriesExists) == 0) //nolint:gosec // row ids are positive

	c.JSON(http.StatusOK, gin.H{"success": t("Series added successfully to ") + listName})
}

// findSeriesCfgpAndListID resolves a series list name to its media config and list index.
func findSeriesCfgpAndListID(listName string) (*config.MediaTypeConfig, int) {
	allMedia := config.GetSettingsMediaAll()
	if allMedia == nil {
		return nil, -1
	}

	return findCfgpAndListID(listName, "serie_", allMedia.Series)
}

// AddMovieManual handles manual movie entry.
func AddMovieManual(c *gin.Context) {
	title := c.PostForm("manualMovie_title")
	yearStr := c.PostForm("manualMovie_year")
	listName := c.PostForm("manualMovie_list")
	qualityProfile := c.PostForm("manualMovie_quality_profile")
	imdbID := c.PostForm("manualMovie_imdb_id")
	overview := c.PostForm("manualMovie_overview")

	if title == "" || listName == "" || qualityProfile == "" {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": t("Title, list name, and quality profile are required")},
		)

		return
	}

	listName, ok := configuredListName(c, listName, "movie")
	if !ok {
		return
	}

	if !configuredListQuality(c, listName, "movie", c.PostForm("manualMovie_quality_profile")) {
		return
	}

	year := 0
	if yearStr != "" {
		if parsedYear, err := strconv.Atoi(yearStr); err == nil {
			year = parsedYear
		}
	}

	// An existing dbmovies row (same IMDb id, or same title and year) is
	// linked into the list instead of creating a duplicate metadata row.
	var existing []int

	switch {
	case imdbID != "":
		existing = database.GetrowsN[int](false, 1, "SELECT id FROM dbmovies WHERE imdb_id = ? ORDER BY id LIMIT 1", &imdbID)
	case year > 0:
		existing = database.GetrowsN[int](false, 1, "SELECT id FROM dbmovies WHERE title = ? COLLATE NOCASE AND year = ? ORDER BY id LIMIT 1", &title, &year)
	default:
		existing = database.GetrowsN[int](false, 1, "SELECT id FROM dbmovies WHERE title = ? COLLATE NOCASE ORDER BY id LIMIT 1", &title)
	}

	nowTime := time.Now()

	var dbMovieID int

	if len(existing) > 0 {
		dbMovieID = existing[0]

		inList := database.GetrowsN[int](false, 1,
			"SELECT count() FROM movies WHERE dbmovie_id = ? AND listname = ? COLLATE NOCASE", &dbMovieID, &listName)
		if len(inList) > 0 && inList[0] > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": tf("Movie already exists in list '%s'", listName)})
			return
		}
	} else {
		newID, err := database.ExecNid(
			"INSERT INTO dbmovies (title, release_date, year, adult, budget, genres, original_language, original_title, overview, popularity, revenue, runtime, spoken_languages, status, tagline, vote_average, vote_count, moviedb_id, imdb_id, freebase_m_id, freebase_id, facebook_id, instagram_id, twitter_id, url, backdrop, poster, slug, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&title,
			"",
			&year,
			false,
			0,
			"",
			"",
			&title,
			&overview,
			0.0,
			0,
			0,
			"",
			"",
			"",
			0.0,
			0,
			0,
			&imdbID,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			0,
			&nowTime,
			&nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert manual movie")
			c.JSON(http.StatusInternalServerError, gin.H{"error": t("Failed to add movie to database")})
			return
		}

		dbMovieID = int(newID)
	}

	if err := database.ExecNErr(
		"INSERT INTO movies (dbmovie_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, ?, 0, 0, 0, ?, ?)",
		&dbMovieID,
		&listName,
		&qualityProfile,
		&nowTime,
		&nowTime,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": t("Failed to add movie: ") + err.Error()})
		return
	}

	notifyManualAdd("movie", listName, uint(dbMovieID), len(existing) == 0) //nolint:gosec // row ids are positive

	c.JSON(http.StatusOK, gin.H{"success": t("Movie added manually to ") + listName})
}

// Lookups of AddSeriesManual for an existing dbseries row.
const (
	manualSerieByTvdbQuery = "SELECT id FROM dbseries WHERE thetvdb_id = ? ORDER BY id LIMIT 1"
	// Args: name, TVDB id (0 = none given).
	manualSerieByNameQuery = "SELECT id FROM dbseries WHERE seriename = ? COLLATE NOCASE AND (? = 0 OR thetvdb_id = 0) ORDER BY id LIMIT 1"
)

// AddSeriesManual handles manual series entry.
func AddSeriesManual(c *gin.Context) {
	seriename := c.PostForm("manualSeries_seriename")
	tvdbIDStr := c.PostForm("manualSeries_thetvdb_id")
	listName := c.PostForm("manualSeries_list")
	firstaired := c.PostForm("manualSeries_firstaired")
	network := c.PostForm("manualSeries_network")
	overview := c.PostForm("manualSeries_overview")

	if seriename == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": t("Series name and list name are required")})
		return
	}

	listName, ok := configuredListName(c, listName, "serie")
	if !ok {
		return
	}

	tvdbID := 0
	if tvdbIDStr != "" {
		if parsedID, err := strconv.Atoi(tvdbIDStr); err == nil {
			tvdbID = parsedID
		}
	}

	// An existing dbseries row (same TVDB id, else same name) is linked into
	// the list instead of creating a duplicate metadata row. A name match is
	// only taken when it can't be a different show: no TVDB id was given, or
	// the row has none.
	var existing []int
	if tvdbID > 0 {
		existing = database.GetrowsN[int](false, 1, manualSerieByTvdbQuery, &tvdbID)
	}

	if len(existing) == 0 {
		existing = database.GetrowsN[int](false, 1,
			manualSerieByNameQuery, &seriename, &tvdbID)
	}

	nowTime := time.Now()

	var dbSerieID int

	if len(existing) > 0 {
		dbSerieID = existing[0]

		inList := database.GetrowsN[int](false, 1,
			"SELECT count() FROM series WHERE dbserie_id = ? AND listname = ? COLLATE NOCASE", &dbSerieID, &listName)
		if len(inList) > 0 && inList[0] > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": tf("Series already exists in list '%s'", listName)})
			return
		}
	} else {
		newID, err := database.ExecNid(
			"INSERT INTO dbseries (seriename, season, status, firstaired, network, runtime, language, genre, overview, rating, siterating, siterating_count, slug, imdb_id, thetvdb_id, freebase_m_id, freebase_id, tvrage_id, facebook, instagram, twitter, banner, fanart, poster, identifiedby, trakt_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&seriename,
			"",
			"",
			&firstaired,
			&network,
			"",
			"",
			"",
			&overview,
			"",
			"",
			"",
			"",
			"",
			&tvdbID,
			"",
			"",
			0,
			"",
			"",
			"",
			"",
			"",
			"",
			"",
			0,
			&nowTime,
			&nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert manual series")
			c.JSON(http.StatusInternalServerError, gin.H{"error": t("Failed to add series to database")})
			return
		}

		dbSerieID = int(newID)
	}

	qualityProfile := ""
	if cfgp, listid := findSeriesCfgpAndListID(listName); cfgp != nil && listid >= 0 {
		qualityProfile = cfgp.Lists[listid].TemplateQuality
		if qualityProfile == "" {
			qualityProfile = cfgp.TemplateQuality
		}
	}

	if err := database.ExecNErr(
		"INSERT INTO series (dbserie_id, listname, rootpath, quality_profile, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', ?, 0, 0, ?, ?)",
		&dbSerieID,
		&listName,
		&qualityProfile,
		&nowTime,
		&nowTime,
	); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": t("Failed to add series: ") + err.Error()})
		return
	}

	notifyManualAdd("serie", listName, uint(dbSerieID), len(existing) == 0) //nolint:gosec // row ids are positive

	c.JSON(http.StatusOK, gin.H{"success": t("Series added manually to ") + listName})
}

// Helper functions

func getYearFromDate(t time.Time) int {
	if t.IsZero() {
		return 0
	}

	return t.Year()
}

func genresToString(genres []apiexternal_v2.Genre) string {
	if len(genres) == 0 {
		return ""
	}

	var result strings.Builder
	for i, genre := range genres {
		if i > 0 {
			result.WriteString(", ")
		}

		result.WriteString(genre.Name)
	}

	return result.String()
}

func spokenLanguagesToString(languages []apiexternal_v2.SpokenLanguage) string {
	if len(languages) == 0 {
		return ""
	}

	var result strings.Builder
	for i, lang := range languages {
		if i > 0 {
			result.WriteString(", ")
		}

		result.WriteString(lang.Name)
	}

	return result.String()
}

func getNetworkString(networks []apiexternal_v2.Network) string {
	if len(networks) == 0 {
		return ""
	}

	return networks[0].Name
}

func getRuntimeFromEpisodeRuntime(episodeRuntime []int) int {
	if len(episodeRuntime) == 0 {
		return 0
	}

	return episodeRuntime[0]
}

func movieSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
// confirmAction puts its message into innerHTML; titles come from TMDB.
function metaSearchEscape(v) {
	return String(v == null ? '' : v).replace(/[&<>"']/g, function(ch) {
		return {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[ch];
	});
}

document.getElementById('movieSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('movie_title').value;
	const list = document.getElementById('movie_list').value;
	const qualityProfile = document.getElementById('movie_quality_profile').value;

	if (!title || !list || !qualityProfile) {
		alert(_t('Please fill in all fields including quality profile'));
		return;
	}

	const resultsDiv = document.getElementById('movieSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">' + _t('Loading...') + '</span></div><p class="mt-2">' + _t('Searching movies...') + '</p></div>';

	fetch('/api/admin/search/movies', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'movie_title=' + encodeURIComponent(title) + '&have_list=' + encodeURIComponent(list)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;

		document.querySelectorAll('.add-movie-btn').forEach(btn => {
			btn.addEventListener('click', function() {
				const tmdbId = this.getAttribute('data-tmdb-id');
				const movieTitle = this.getAttribute('data-title');
				const selectedList = document.getElementById('movie_list').value;
				const selectedQualityProfile = document.getElementById('movie_quality_profile').value;

				if (!selectedList || !selectedQualityProfile) {
					alert(_t('Please select both a list and quality profile'));
					return;
				}

				// confirmAction calls the callback without a receiver, so "this"
				// is not the button there.
				const addBtn = this;
				confirmAction(_t('Please confirm'), _t('Add "') + metaSearchEscape(movieTitle) + '" to list "' + metaSearchEscape(selectedList) + '" with quality profile "' + metaSearchEscape(selectedQualityProfile) + '"?', function() {
					addMovieToDatabase(tmdbId, selectedList, selectedQualityProfile, addBtn);
				})
			});
		});
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">' + _t('Error searching for movies. Please try again.') + '</div>';
	});
});

function addMovieToDatabase(tmdbId, listName, qualityProfile, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>' + _t('Adding...');
	button.disabled = true;

	fetch('/api/admin/add/movie', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'tmdb_id=' + tmdbId + '&movie_list=' + encodeURIComponent(listName) + '&movie_quality_profile=' + encodeURIComponent(qualityProfile)
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>' + _t('Added!');
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert(_t('Error: ') + (data.error || _t('Failed to add movie')));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert(_t('Error adding movie to database'));
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('movieManualForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const formData = new FormData(this);
	const data = new URLSearchParams(formData);

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>' + _t('Adding...');
	submitBtn.disabled = true;

	fetch('/api/admin/add/movie/manual', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: data
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert(_t('Success: ') + data.success);
			this.reset();
			submitBtn.innerHTML = '<i class="fas fa-check me-2"></i>' + _t('Added!');
			setTimeout(() => {
				submitBtn.innerHTML = originalText;
				submitBtn.disabled = false;
			}, 2000);
		} else {
			alert(_t('Error: ') + (data.error || _t('Failed to add movie')));
			submitBtn.innerHTML = originalText;
			submitBtn.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert(_t('Error adding movie manually'));
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
`))
}

func seriesSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
// confirmAction puts its message into innerHTML; titles come from TMDB.
function metaSearchEscape(v) {
	return String(v == null ? '' : v).replace(/[&<>"']/g, function(ch) {
		return {'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[ch];
	});
}

document.getElementById('seriesSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('series_title').value;
	const list = document.getElementById('series_list').value;

	if (!title || !list) {
		alert(_t('Please fill in all fields'));
		return;
	}

	const resultsDiv = document.getElementById('seriesSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">' + _t('Loading...') + '</span></div><p class="mt-2">' + _t('Searching series...') + '</p></div>';

	fetch('/api/admin/search/series', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'series_title=' + encodeURIComponent(title) + '&have_list=' + encodeURIComponent(list)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;

		document.querySelectorAll('.add-series-btn').forEach(btn => {
			btn.addEventListener('click', function() {
				const tmdbId = this.getAttribute('data-tmdb-id');
				const seriesTitle = this.getAttribute('data-title');
				const selectedList = document.getElementById('series_list').value;
				const selectedQualityProfile = document.getElementById('series_quality_profile').value;
				const addBtn = this;

				confirmAction(_t('Please confirm'), _t('Add "') + metaSearchEscape(seriesTitle) + '" to list "' + metaSearchEscape(selectedList) + '"?', function() {
					addSeriesToDatabase(tmdbId, selectedList, selectedQualityProfile, addBtn);
				})
			});
		});
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">' + _t('Error searching for series. Please try again.') + '</div>';
	});
});

function addSeriesToDatabase(tmdbId, listName, qualityProfile, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>' + _t('Adding...');
	button.disabled = true;

	fetch('/api/admin/add/series', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'tmdb_id=' + tmdbId + '&series_list=' + encodeURIComponent(listName) + '&series_quality_profile=' + encodeURIComponent(qualityProfile || '')
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>' + _t('Added!');
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert(_t('Error: ') + (data.error || _t('Failed to add series')));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert(_t('Error adding series to database'));
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('seriesManualForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const formData = new FormData(this);
	const data = new URLSearchParams(formData);

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>' + _t('Adding...');
	submitBtn.disabled = true;

	fetch('/api/admin/add/series/manual', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: data
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert(_t('Success: ') + data.success);
			this.reset();
			submitBtn.innerHTML = '<i class="fas fa-check me-2"></i>' + _t('Added!');
			setTimeout(() => {
				submitBtn.innerHTML = originalText;
				submitBtn.disabled = false;
			}, 2000);
		} else {
			alert(_t('Error: ') + (data.error || _t('Failed to add series')));
			submitBtn.innerHTML = originalText;
			submitBtn.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert(_t('Error adding series manually'));
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
`))
}

// mediaConfigQualityScript narrows the quality select qualityID to the
// profiles of the media config chosen in configID (option values are the
// configs' NamePrefix, e.g. "movie_EN") and preselects the group template.
func mediaConfigQualityScript(configID, qualityID string) gomponents.Node {
	byConfig := make(map[string]selectQuality)

	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		byConfig[cfgp.NamePrefix] = selectQuality{
			Profiles: configQualityProfiles(cfgp),
			Default:  cfgp.TemplateQuality,
		}

		return nil
	})

	return qualitySelectScript(byConfig, map[string]string{configID: qualityID}, t("Select a media config first"))
}

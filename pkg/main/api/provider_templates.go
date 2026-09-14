// Package api - provider_templates.go adds Radarr/Sonarr-style "pick a known
// service, get the fields pre-filled" template pickers for Downloader,
// Indexer, Notification and List forms, in both the Setup Wizard and the
// full Settings screens.
//
// The mechanism is intentionally generic: each <option> carries its prefill
// values as a single JSON-encoded data-fields attribute (field name suffix ->
// value) plus a data-required attribute (JSON array of field name suffixes
// still needing the user's own value, even if the template already filled in
// a placeholder). One small shared JS function (applyGmdTemplate) copies
// values into, and highlights, whichever sibling inputs share that suffix -
// [name$="_FieldSuffix"] - regardless of whether the surrounding form uses
// flat wizard field names (wizard_indexer_URL) or array-indexed Settings
// field names (indexers_MyIndexer_URL). No per-category JS needed, and the
// same template lists work in both the wizard and Settings.
//
// Field names go in the JSON *values*, not per-field attribute *names*
// (data-f-<Field>): HTML lowercases attribute names during parsing, which
// would silently break the suffix match for any mixed-case field like
// NotificationType or AppriseURLs. Attribute *values* aren't case-normalized,
// so JSON blobs sidestep that entirely.
package api

import (
	"encoding/json"

	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// providerTemplate is one preset entry in a template dropdown.
type providerTemplate struct {
	Label    string
	Fields   map[string]string // form field name suffix -> value to prefill
	Required []string          // field name suffixes still needing the user's own value
}

// gmdTemplateScript is the shared prefill handler. Safe to emit multiple
// times on one page (e.g. several rows each rendering their own template
// select) - re-declaring a function is a no-op in JS.
//
// Scope is the whole row card (.array-item-enhanced, see ClassArrayItem in
// config.go), not the nearest .accordion-item: each Settings row splits its
// fields across several accordion-item sub-groups (Basic Settings,
// Connection Settings, ...), so scoping to the template select's own
// (narrow) sub-group would miss fields like URL/Hostname/Port that live in a
// sibling group. The wizard's flat forms have no .array-item-enhanced
// ancestor, so they fall through to the whole <form> instead.
var gmdTemplateScript = html.Script(gomponents.Raw(`
function applyGmdTemplate(selectEl) {
  var opt = selectEl.options[selectEl.selectedIndex];
  var scope = selectEl.closest('.array-item-enhanced') || selectEl.closest('form') || document;
  var hint = selectEl.parentElement.querySelector('.gmd-required-hint');

  scope.querySelectorAll('.gmd-needs-input').forEach(function(el) {
    el.classList.remove('gmd-needs-input', 'border-warning');
    el.style.borderWidth = '';
    el.required = false;
  });
  if (hint) hint.innerHTML = '';

  if (!opt) return;

  var fields = {};
  var raw = opt.getAttribute('data-fields');
  if (raw) { try { fields = JSON.parse(raw); } catch (e) {} }

  Object.keys(fields).forEach(function(field) {
    var el = scope.querySelector('[name$="_' + field + '"]');
    if (!el) return;
    if (el.type === 'checkbox') { el.checked = (fields[field] === true || fields[field] === 'true'); }
    else { el.value = fields[field]; }
  });

  var required = [];
  var requiredRaw = opt.getAttribute('data-required');
  if (requiredRaw) { try { required = JSON.parse(requiredRaw); } catch (e) {} }

  var labels = [];
  required.forEach(function(field) {
    var el = scope.querySelector('[name$="_' + field + '"]');
    if (!el) return;
    el.classList.add('gmd-needs-input', 'border-warning');
    el.style.borderWidth = '2px';
    el.required = true;
    var wrap = el.closest('.form-field-compact');
    var lbl = wrap ? wrap.querySelector('label') : null;
    labels.push(lbl ? lbl.textContent : field);
  });

  if (hint && labels.length) {
    hint.innerHTML = '<i class="fa-solid fa-triangle-exclamation text-warning me-1"></i>' +
      'Still needs your own value for: <strong>' + labels.join(', ') + '</strong>';
  }
}
`))

// renderTemplateSelect renders a "Template" dropdown followed by the shared
// prefill script. Place it as the first field in a form/section so users see
// it before the fields it fills in.
func renderTemplateSelect(label string, templates []providerTemplate) gomponents.Node {
	options := make([]gomponents.Node, 0, len(templates)+1)
	options = append(
		options,
		html.Option(html.Value(""), gomponents.Text("-- Choose a template (optional) --")),
	)

	for _, t := range templates {
		fieldsJSON, _ := json.Marshal(t.Fields)

		required := t.Required
		if required == nil {
			required = []string{}
		}

		requiredJSON, _ := json.Marshal(required)

		attrs := []gomponents.Node{
			html.Value(t.Label),
			gomponents.Text(t.Label),
			gomponents.Attr("data-fields", string(fieldsJSON)),
			gomponents.Attr("data-required", string(requiredJSON)),
		}

		options = append(options, html.Option(attrs...))
	}

	return html.Div(
		html.Class("form-group mb-2"),
		html.Div(
			html.Class("form-field-compact p-2 border rounded"),
			html.Style("background: #fffdf5; border: 1px solid #f0e0a0 !important;"),
			html.Div(
				html.Class("d-flex align-items-center mb-1"),
				html.I(
					html.Class("fa-solid fa-wand-magic-sparkles text-primary me-2"),
					html.Style("font-size: 0.85em;"),
				),
				html.Label(gomponents.Text(label)),
			),
			html.Select(
				html.Class("form-select"),
				gomponents.Attr("onchange", "applyGmdTemplate(this)"),
				gomponents.Group(options),
			),
			html.Small(
				html.Class("text-muted"),
				gomponents.Text("Pre-fills the fields below for a known service - fields outlined in orange still need your own value."),
			),
			html.Div(html.Class("gmd-required-hint small mt-2")),
		),
		gmdTemplateScript,
	)
}

// ---- Downloader templates ----
// One per supported DlType, with that client's typical default port.

// downloaderPreset builds a real-client downloader template - DlType +
// typical default host/port prefilled, but Hostname/Port are still flagged
// required since "localhost:<default>" is only a guess and needs verifying
// against the user's actual setup, on top of whatever credentials that
// client needs.
func downloaderPreset(label, dlType, port string, credentialFields ...string) providerTemplate {
	return providerTemplate{
		Label:    label,
		Fields:   map[string]string{"DlType": dlType, "Hostname": "localhost", "Port": port},
		Required: append([]string{"Hostname", "Port"}, credentialFields...),
	}
}

var downloaderTemplates = []providerTemplate{
	downloaderPreset("qBittorrent", "qbittorrent", "8080", "Username", "Password"),
	downloaderPreset("Deluge", "deluge", "8112", "Password"),
	downloaderPreset("Transmission", "transmission", "9091", "Username", "Password"),
	downloaderPreset("rTorrent", "rtorrent", "5000"),
	// SABnzbd auths via API key - entered in the Password field.
	downloaderPreset("SABnzbd", "sabnzbd", "8080", "Password"),
	downloaderPreset("NZBGet", "nzbget", "6789", "Username", "Password"),
	{
		Label:  "Drone (file only, no client)",
		Fields: map[string]string{"DlType": "drone"},
		// Drone has no Hostname/Port/path of its own - the destination folder
		// comes from the Quality profile's NZB path (Settings -> Quality ->
		// Indexer entry -> Path Template), not from the downloader itself.
	},
}

// ---- Indexer templates ----
// Sourced from NZBHydra2's own preset list (core/ui-src/js/config/formly-indexers.js,
// $scope.newznabPresets / $scope.torznabPresets), cross-checked against a real
// production config.toml where the two disagreed.

var indexerTemplates = []providerTemplate{
	{
		Label:    "Newznab (Generic)",
		Fields:   map[string]string{"IndexerType": "newznab"},
		Required: []string{"URL", "Apikey"},
	},
	{
		Label:    "Torznab (Generic)",
		Fields:   map[string]string{"IndexerType": "torznab"},
		Required: []string{"URL", "Apikey"},
	},

	indexerPreset("abNZB", "https://abnzb.com/"),
	indexerPreset("altHUB", "https://api.althub.co.za"),
	indexerPreset("ameNZB", "https://amenzb.moe"),
	indexerPreset("BlurayNZB", "https://www.bluraynzb.org"),
	indexerPreset("Digital Carnage", "https://digitalcarnage.info"),
	{
		Label: "DogNZB",
		Fields: map[string]string{
			"IndexerType":  "newznab",
			"URL":          "https://api.dognzb.cr",
			"Customrssurl": "https://dognzb.cr/rss?r={your_api_token}&i=0",
		},
		Required: []string{"URL", "Apikey", "Customrssurl"},
	},
	indexerPreset("Drunken Slug", "https://api.drunkenslug.com"),
	indexerPreset("FastNZB", "https://fastnzb.com"),
	indexerPreset("LuluNZB", "https://lulunzb.com"),
	indexerPreset("miatrix", "https://www.miatrix.com"),
	indexerPreset("NZB Finder", "https://nzbfinder.ws"),
	indexerPreset("NZBCat", "https://nzb.cat"),
	indexerPreset("nzb.life", "https://api.nzb.life"),
	indexerPreset("NZBGeek", "https://api.nzbgeek.info"),
	{
		Label: "NZBFinder",
		Fields: map[string]string{
			"IndexerType":       "newznab",
			"URL":               "https://nzbfinder.ws/api/v1",
			"Customrssurl":      "https://nzbfinder.ws/rss/category?api_token={your_api_token}",
			"Customrsscategory": "id",
		},
		Required: []string{"URL", "Apikey", "Customrssurl"},
	},
	indexerPreset("NzbNdx", "https://www.nzbndx.com"),
	indexerPreset("NzBNooB", "https://www.nzbnoob.com"),
	indexerPreset("NzbNation", "http://www.nzbnation.com/"),
	indexerPreset("nzbplanet", "https://api.nzbplanet.net"),
	indexerPreset("omgwtfnzbs", "https://api.omgwtfnzbs.org"),
	indexerPreset("OZnzb", "https://legendapi.oznzb.com"),
	indexerPreset("Treasure Maps", "https://treasure-maps.com"),
	indexerPreset("spotweb.com", "https://spotweb.me"),
	indexerPreset("Tabula-Rasa", "https://www.tabula-rasa.pw/api/v1/"),
	indexerPreset("Torbox (Newznab)", "https://search-api.torbox.app/newznab"),
	indexerPreset("Usenet Crawler", "https://www.usenet-crawler.com"),

	{
		Label: "Jackett/Cardigann (self-hosted)",
		Fields: map[string]string{
			"IndexerType": "torznab",
			"URL":         "http://localhost:9117/api/v2.0/indexers/YOURTRACKER/results/torznab/",
		},
		Required: []string{"URL", "Apikey"}, // URL still needs YOURTRACKER replaced
	},
	{
		Label:    "Torbox (Torrents)",
		Fields:   map[string]string{"IndexerType": "torznab", "URL": "https://search-api.torbox.app/torznab"},
		Required: []string{"URL", "Apikey"},
	},
	{
		Label:    "NZBHydra2 (self-hosted)",
		Fields:   map[string]string{"IndexerType": "newznab", "URL": "http://localhost:5076"},
		Required: []string{"URL", "Apikey"},
	},
	{
		Label:    "Prowlarr (self-hosted)",
		Fields:   map[string]string{"IndexerType": "torznab", "URL": "http://localhost:9696"},
		Required: []string{"URL", "Apikey"},
	},
}

// indexerPreset builds a standard Newznab indexer template - name + base URL
// prefilled, but URL and Apikey are both still flagged: the URL is a
// best-known default that can change, and Apikey always needs the user's own
// value.
func indexerPreset(label, url string) providerTemplate {
	return providerTemplate{
		Label:    label,
		Fields:   map[string]string{"IndexerType": "newznab", "URL": url},
		Required: []string{"URL", "Apikey"},
	}
}

// ---- Notification templates ----

var notificationTemplates = []providerTemplate{
	{
		Label:    "Pushover",
		Fields:   map[string]string{"NotificationType": "pushover"},
		Required: []string{"Apikey", "Recipient"},
	},
	{
		Label: "Gotify (self-hosted)",
		Fields: map[string]string{
			"NotificationType": "gotify",
			"ServerURL":        "http://localhost:8080",
		},
		Required: []string{"ServerURL", "Apikey"},
	},
	{
		Label:    "Pushbullet",
		Fields:   map[string]string{"NotificationType": "pushbullet"},
		Required: []string{"Apikey"},
	},
	{
		Label: "Discord (via Apprise)",
		Fields: map[string]string{
			"NotificationType": "apprise",
			"AppriseURLs":      "discord://{webhook_id}/{webhook_token}",
		},
		Required: []string{"AppriseURLs"},
	},
	{
		Label: "Slack (via Apprise)",
		Fields: map[string]string{
			"NotificationType": "apprise",
			"AppriseURLs":      "slack://{TokenA}/{TokenB}/{TokenC}/{Channel}",
		},
		Required: []string{"AppriseURLs"},
	},
	{
		Label: "Telegram (via Apprise)",
		Fields: map[string]string{
			"NotificationType": "apprise",
			"AppriseURLs":      "tgram://{bot_token}/{chat_id}",
		},
		Required: []string{"AppriseURLs"},
	},
	{
		Label: "Email (via Apprise)",
		Fields: map[string]string{
			"NotificationType": "apprise",
			"AppriseURLs":      "mailto://{user}:{password}@{domain}",
		},
		Required: []string{"AppriseURLs"},
	},
	{
		Label: "CSV (write to file)",
		Fields: map[string]string{
			"NotificationType": "csv",
			"Outputto":         "./logs/notifications.csv",
		},
		Required: []string{"Outputto"},
	},
	{
		Label: "Email (direct SMTP)",
		Fields: map[string]string{
			"NotificationType": "sendmail",
			"SMTPPort":         "587",
		},
		Required: []string{"SMTPServer", "SMTPFromEmail", "SMTPToEmail"},
	},
}

// ---- List templates ----
// A small curated set of ready-to-use ListType combos - the full ListType
// dropdown already covers everything else.

var listTemplates = []providerTemplate{
	{
		Label:  "Trakt Popular Movies",
		Fields: map[string]string{"ListType": "traktmoviepopular", "Limit": "50"},
	},
	{
		Label:  "Trakt Trending Series",
		Fields: map[string]string{"ListType": "traktserietrending", "Limit": "50"},
	},
	{Label: "Plex Watchlist", Fields: map[string]string{"ListType": "plexwatchlist"}},
	{Label: "Jellyfin Watchlist", Fields: map[string]string{"ListType": "jellyfinwatchlist"}},
	{
		Label:    "IMDB CSV Export",
		Fields:   map[string]string{"ListType": "imdbcsv"},
		Required: []string{"IMDBCSVFile"},
	},
}

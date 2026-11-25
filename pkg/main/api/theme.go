package api

import (
	"net/http"
	"slices"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Light and dark mode. The choice is kept in the account's settings
// ("theme": light, dark, auto) and mirrored in the gmd_theme cookie, which
// the page head reads before anything is drawn: every page (also the admin
// pages built without the account) starts in the right colors.

var themes = []string{"light", "dark", "auto"}

// accountSettingAllowed reports whether a page may set the setting key to
// value (/admin/account/setting).
func accountSettingAllowed(key, value string) bool {
	switch key {
	case "theme":
		return slices.Contains(themes, value)
	case "lang":
		return value == "auto" || languageNames[value] != ""
	}

	return false
}

// themeHeadScript picks the stylesheet from the cookie; it runs right after
// the stylesheet link in the head. gmdSetTheme switches and stores a theme.
const themeHeadScript = `(function () {
	var mq = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
	var cur = function () { var m = /(?:^|; )gmd_theme=(\w+)/.exec(document.cookie); return m ? m[1] : 'light'; };
	window.gmdThemeApply = function (t) {
		var dark = t === 'dark' || (t === 'auto' && mq && mq.matches);
		var link = document.getElementById('gmdThemeCss'), href = '/static/css/' + (dark ? 'dark' : 'light') + '.css';
		if (link && link.getAttribute('href') !== href) { link.setAttribute('href', href); }
		// Not data-bs-theme: AdminKit's dark.css is dark under :root and has
		// a different (plain Bootstrap) palette for data-bs-theme=dark.
		var root = document.documentElement;
		root.setAttribute('data-gmd-dark', dark ? '1' : '0');
		root.setAttribute('data-gmd-theme', t);
		document.querySelectorAll('[data-gmd-theme-pick]').forEach(function (b) {
			b.classList.toggle('active', b.getAttribute('data-gmd-theme-pick') === t);
		});
	};
	window.gmdThemeCookie = function (t) {
		document.cookie = 'gmd_theme=' + t + '; max-age=31536000; path=/; SameSite=Lax';
		gmdThemeApply(t);
	};
	window.gmdSetTheme = function (t) {
		gmdThemeCookie(t);
		if (window.gmdSettings) { window.gmdSettings.theme = t; }
		var tok = /(?:^|; )csrf_token=([^;]+)/.exec(document.cookie);
		fetch('/api/admin/account/setting', { method: 'POST', credentials: 'same-origin',
			body: new URLSearchParams({ key: 'theme', value: t }),
			headers: { 'X-CSRF-Token': tok ? decodeURIComponent(tok[1]) : '' } }).catch(function () {});
	};
	window.gmdSetLang = function (l) {
		if (window.gmdSettings) { window.gmdSettings.lang = l; }
		var tok = /(?:^|; )csrf_token=([^;]+)/.exec(document.cookie);
		fetch('/api/admin/account/setting', { method: 'POST', credentials: 'same-origin',
			body: new URLSearchParams({ key: 'lang', value: l }),
			headers: { 'X-CSRF-Token': tok ? decodeURIComponent(tok[1]) : '' } })
			.then(function () { location.reload(); }).catch(function () {});
	};
	gmdThemeApply(cur());
	if (mq && mq.addEventListener) { mq.addEventListener('change', function () { if (cur() === 'auto') { gmdThemeApply('auto'); } }); }
	document.addEventListener('DOMContentLoaded', function () {
		gmdThemeApply(cur());
		var lang = (window.gmdSettings && window.gmdSettings.lang) || 'auto';
		document.querySelectorAll('[data-gmd-lang-pick]').forEach(function (b) {
			b.classList.toggle('active', b.getAttribute('data-gmd-lang-pick') === lang);
		});
	});
})();`

// themeSyncScript takes the account's stored theme over into this browser
// (another browser may have changed it).
const themeSyncScript = `if (window.gmdSettings && /^(light|dark|auto)$/.test(window.gmdSettings.theme || '') &&
	document.documentElement.getAttribute('data-gmd-theme') !== window.gmdSettings.theme) { gmdThemeCookie(window.gmdSettings.theme); }`

// themeHead is the stylesheet link and the script that picks it.
func themeHead() gomponents.Node {
	return gomponents.Group([]gomponents.Node{
		html.Link(html.Rel("stylesheet"), html.ID("gmdThemeCss"), html.Href("/static/css/light.css")),
		html.Script(gomponents.Raw(themeHeadScript + "\n" + trJS)),
	})
}

// themeDarkCSS turns the app's own light surfaces (addCSS and the pages'
// styles) dark; it follows addCSS in the head. The colors are AdminKit
// dark's: page #19222c, card #222e3c, raised #2d3846, border #4e5863.
const themeDarkCSS = `
[data-gmd-dark="1"] { color-scheme: dark; }
[data-gmd-dark="1"] .page-header-enhanced, [data-gmd-dark="1"] .config-section, [data-gmd-dark="1"] .array-item,
[data-gmd-dark="1"] .array-item-enhanced, [data-gmd-dark="1"] .array-item-enhanced .card-body,
[data-gmd-dark="1"] .database-content-container, [data-gmd-dark="1"] .filters-card-enhanced,
[data-gmd-dark="1"] .database-table-container, [data-gmd-dark="1"] .table-modern, [data-gmd-dark="1"] .form-card,
[data-gmd-dark="1"] .form-actions-enhanced, [data-gmd-dark="1"] .results-container-enhanced, [data-gmd-dark="1"] .help-content,
[data-gmd-dark="1"] .edit-form-container, [data-gmd-dark="1"] .form-field-enhanced, [data-gmd-dark="1"] .modal-footer-enhanced,
[data-gmd-dark="1"] #editFormModal .modal-footer, [data-gmd-dark="1"] .choices__list--dropdown, [data-gmd-dark="1"] .help-card-icon,
[data-gmd-dark="1"] .dataTables_wrapper table th:last-child, [data-gmd-dark="1"] .dataTables_wrapper table tr:not(.child) > td:last-child {
	background: #222e3c !important; color: #bdc0c5;
}
[data-gmd-dark="1"] .field-help, [data-gmd-dark="1"] .header-icon-wrapper, [data-gmd-dark="1"] .array-item-enhanced .card-header,
[data-gmd-dark="1"] .filters-header, [data-gmd-dark="1"] .table-header-modern, [data-gmd-dark="1"] .card-header,
[data-gmd-dark="1"] .btn-action-secondary, [data-gmd-dark="1"] .help-card, [data-gmd-dark="1"] .tip-item,
[data-gmd-dark="1"] .form-control-modern, [data-gmd-dark="1"] .form-switch-enhanced, [data-gmd-dark="1"] .modal-body-enhanced,
[data-gmd-dark="1"] .submit-group .btn-secondary, [data-gmd-dark="1"] .help-text-content,
[data-gmd-dark="1"] .dataTables_wrapper .dataTables_length select, [data-gmd-dark="1"] .dataTables_wrapper .dataTables_filter input,
[data-gmd-dark="1"] .dataTables_wrapper .dataTables_paginate .paginate_button,
[data-gmd-dark="1"] .choices[data-type*="select-one"] .choices__inner, [data-gmd-dark="1"] .choices[data-type*="select-multiple"] .choices__inner,
[data-gmd-dark="1"] .dataTables_wrapper table th:last-child {
	background: #2d3846 !important; color: #d3d5d8 !important; border-color: #4e5863 !important;
}
[data-gmd-dark="1"] .array-item-enhanced .card-header:hover, [data-gmd-dark="1"] .card-header[data-bs-toggle="collapse"]:hover,
[data-gmd-dark="1"] .table-modern tbody tr:hover, [data-gmd-dark="1"] .btn-action-secondary:hover,
[data-gmd-dark="1"] .submit-group .btn-secondary:hover, [data-gmd-dark="1"] .choices__item--choice:hover,
[data-gmd-dark="1"] .dataTables_wrapper table tbody tr:not(.child):hover > td:last-child,
[data-gmd-dark="1"] .dataTables_wrapper .dataTables_paginate .paginate_button.disabled {
	background: #384350 !important;
}
[data-gmd-dark="1"] .header-title, [data-gmd-dark="1"] .card-title, [data-gmd-dark="1"] .help-title,
[data-gmd-dark="1"] .help-card-content strong, [data-gmd-dark="1"] .tip-item strong, [data-gmd-dark="1"] .field-help-heading {
	color: #e9eaec !important;
}
[data-gmd-dark="1"] .table-loading-overlay { background: rgba(25, 34, 44, .9); }
[data-gmd-dark="1"] .alert-success-enhanced { background: #173f33; color: #a4e4d1; }
[data-gmd-dark="1"] .alert-danger-enhanced { background: #4a1f25; color: #f1aeb5; }
[data-gmd-dark="1"] .content-footer, [data-gmd-dark="1"] .content-footer > div { background: #111820 !important; color: #bdc0c5 !important; }
[data-gmd-dark="1"] .form-control, [data-gmd-dark="1"] .form-select { color-scheme: dark; }
/* The config forms set their colors inline. */
[data-gmd-dark="1"] .accordion-button, [data-gmd-dark="1"] .form-check-wrapper, [data-gmd-dark="1"] .submit-group-enhanced {
	background: #2d3846 !important; color: #e9eaec !important; border-color: #4e5863 !important;
}
[data-gmd-dark="1"] .accordion-body, [data-gmd-dark="1"] .form-field-compact, [data-gmd-dark="1"] .form-field-card {
	background: #222e3c !important; border-color: #4e5863 !important;
}
[data-gmd-dark="1"] .lib-cover, [data-gmd-dark="1"] .lib-thumb, [data-gmd-dark="1"] .lib-detail-cover,
[data-gmd-dark="1"] .lib-ep-thumb, [data-gmd-dark="1"] .calendar-thumb { background-color: #2d3846; }
[data-gmd-dark="1"] .form-check-input-modern:not(:checked) { background: #4e5863 !important; }
[data-gmd-dark="1"] .content-footer .btn-outline-light, [data-gmd-dark="1"] .content-footer .btn-outline-dark { color: #d3d5d8 !important; border-color: #7a828a !important; }
/* The theme's dark backdrop is white. */
[data-gmd-dark="1"] .modal-backdrop { background-color: #000 !important; }
[data-gmd-dark="1"] .modal-backdrop.show { opacity: .7 !important; }
[data-gmd-dark="1"] .sidebar-link i, [data-gmd-dark="1"] .sidebar-link svg { color: rgba(233, 236, 239, .8); }
[data-gmd-dark="1"] .sidebar-link:hover i, [data-gmd-dark="1"] .sidebar-item.active > .sidebar-link i { color: #ffd700; }
/* The icon sidebar's flyout submenus (iconSidebarStyle). */
[data-gmd-dark="1"] body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-dropdown {
	background: #2d3846 !important; box-shadow: 0 .5rem 1.5rem rgba(0, 0, 0, .5);
}
[data-gmd-dark="1"] body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link { color: #d3d5d8 !important; }
[data-gmd-dark="1"] body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link i { color: #9aa3ad; }
[data-gmd-dark="1"] body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link:hover,
[data-gmd-dark="1"] body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-item.active .sidebar-link {
	color: #fff !important; background: #384350 !important; border-left-color: #6ea8fe;
}
[data-gmd-dark="1"] body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link:hover i { color: #ffd700; }
/* The library player dock (playerCSS) falls back to a white background. */
[data-gmd-dark="1"] .lp-dock { background: #19222c; border-top-color: #4e5863; box-shadow: 0 -4px 16px rgba(0, 0, 0, .5); }
/* !important: the video has Bootstrap's bg-black, which AdminKit dark turns white (--bs-black-rgb). */
[data-gmd-dark="1"] .lp-media video { background-color: #000 !important; }
[data-gmd-dark="1"] .lp-tl-knob { box-shadow: 0 0 0 2px #19222c; }
`

// themeStyle is themeDarkCSS for the head (after addCSS).
func themeStyle() gomponents.Node {
	return html.StyleEl(gomponents.Raw(themeDarkCSS))
}

// themeMenuItems are the theme choices of the account menu.
func themeMenuItems() gomponents.Node {
	item := func(t, icon, label string) gomponents.Node {
		return html.Button(html.Type("button"), html.Class("dropdown-item"), html.Data("gmd-theme-pick", t),
			gomponents.Attr("onclick", "gmdSetTheme('"+t+"')"),
			html.I(html.Class("fa-solid "+icon+" me-2")), gomponents.Text(label))
	}

	// The pages are translated on the server: a new language reloads.
	lang := func(l, label string) gomponents.Node {
		return html.Button(html.Type("button"), html.Class("dropdown-item"), html.Data("gmd-lang-pick", l),
			gomponents.Attr("onclick", "gmdSetLang('"+l+"')"),
			html.I(html.Class("fa-solid fa-language me-2")), gomponents.Text(label))
	}

	langs := []gomponents.Node{lang("auto", t("Like the browser"))}
	for _, l := range languages {
		langs = append(langs, lang(l, languageNames[l]))
	}

	return gomponents.Group([]gomponents.Node{
		html.Div(html.Class("dropdown-divider")),
		html.H6(html.Class("dropdown-header"), gomponents.Text(t("Appearance"))),
		item("light", "fa-sun", t("Light")),
		item("dark", "fa-moon", t("Dark")),
		item("auto", "fa-circle-half-stroke", t("Like the system")),
		html.Div(html.Class("dropdown-divider")),
		html.H6(html.Class("dropdown-header"), gomponents.Text(t("Language"))),
		gomponents.Group(langs),
		html.Div(html.Class("dropdown-divider")),
	})
}

// HandleAccountSetting stores one setting of the account
// (/admin/account/setting: key, value).
func HandleAccountSetting(c *gin.Context) {
	key, value := c.PostForm("key"), c.PostForm("value")
	if !accountSettingAllowed(key, value) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown setting or value"})
		return
	}

	if err := database.SetUserSetting(accName(accessOf(c)), key, value); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

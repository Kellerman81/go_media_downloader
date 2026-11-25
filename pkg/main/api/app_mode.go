package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// App mode: the desktop player (pkg/player) opens the portal with ?app=1.
// From then on that browser profile gets only the end-user pages (Library,
// Calendar, Add Media, own account) with a narrow icon sidebar and no
// footer; ?app=0 switches back.

const (
	appCookie = "gmd_app"
	appHome   = "/api/admin/library"
)

// appMode reports whether the request comes from the desktop player or a
// browser in TV mode.
func appMode(c *gin.Context) bool {
	if q := c.Query("app"); q != "" {
		return q == "1"
	}

	if tvMode(c) {
		return true
	}

	v, _ := c.Cookie(appCookie)

	return v == "1"
}

// setAppMode stores ?app=1/0 in the cookie and sends the player from the
// dashboard to its start page; true when it answered the request.
func setAppMode(c *gin.Context) bool {
	setTVMode(c)

	switch c.Query("app") {
	case "1":
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(appCookie, "1", 10*365*24*3600, "/", "", requestIsHTTPS(c), true)
	case "0":
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(appCookie, "", -1, "/", "", requestIsHTTPS(c), true)
	}

	if p := c.Request.URL.Path; appMode(c) && c.Request.Method == http.MethodGet && (p == "/api/admin" || p == "/api/admin/") {
		c.Redirect(http.StatusFound, appHome)
		c.Abort()

		return true
	}

	return false
}

// appNavbar is the player's sidebar: icons only, the end-user pages.
func appNavbar(acc *userAccess) gomponents.Node {
	link := func(href, icon, label string) gomponents.Node {
		return html.Li(html.Class("sidebar-item"),
			html.A(html.Class("sidebar-link"), html.Href(href), html.Title(label), gomponents.Attr("aria-label", label),
				html.I(html.Class("align-middle fa-solid "+icon)),
				html.Span(html.Class("align-middle"), gomponents.Text(label))))
	}

	return html.Nav(html.ID("sidebar"), html.Class("sidebar js-sidebar"), gomponents.Attr("aria-label", "Main navigation"),
		html.Div(html.Class("sidebar-content"),
			html.A(html.Class("sidebar-brand"), html.Href(appHome), html.Title("Go Media"),
				html.Img(html.Src("/static/img/icons/icon-48x48.png"), html.Alt("Go Media"), html.Width("32"), html.Height("32"))),
			html.Ul(html.Class("sidebar-nav"),
				link("/api/admin/library", "fa-photo-film", t("Library")),
				link("/api/admin/calendar", "fa-calendar-alt", t("Calendar")),
				gomponents.If(acc.CanRequest(), link("/api/admin/metadata-search", "fa-plus", t("Add Media"))),
				link("/api/admin/account", "fa-user", t("My Account")),
				link("/api/admin/help", "fa-circle-question", t("Help")),
				link("/api/logout", "fa-right-from-bracket", t("Log out")),
			)))
}

// iconSidebarStyle is the narrow icon sidebar: the desktop player's own
// (gmd-app), and additions to the theme's compact layout
// (data-sidebar-layout=compact, flyout submenus), which the hamburger's
// "icons" state uses on wide screens.
const iconSidebarStyle = `
body.gmd-app .sidebar{min-width:64px;max-width:64px}
body.gmd-app .sidebar.collapsed{margin-left:-64px}
body.gmd-app .sidebar-brand{padding:.9rem 0;text-align:center}
body.gmd-app .sidebar-link{padding:.7rem 0;text-align:center}
body.gmd-app .sidebar-link i{margin:0;font-size:1.15rem}
body.gmd-app .sidebar-link span{display:none}
body[data-sidebar-layout=compact] .sidebar-brand small{display:none !important}
body[data-sidebar-layout=compact] .sidebar-brand .d-flex{justify-content:center}
body[data-sidebar-layout=compact] .sidebar-brand img{margin:0 !important}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-link{text-align:center;padding:.75rem 0}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-link i{margin:0;font-size:1.15rem}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-link .badge{display:inline-block !important;position:absolute;
 top:.25rem;right:.6rem;font-size:.6rem;padding:.15rem .3rem;margin:0 !important}
body[data-sidebar-layout=compact] .sidebar-brand .sidebar-brand-text{display:none !important}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-dropdown{background:#fff !important;padding:.4rem 0 !important}
body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link{background:transparent !important;color:#495057 !important;
 text-align:left;padding:.45rem 1.25rem !important;border-left:3px solid transparent}
body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link i{color:#6c757d;margin-right:.6rem}
body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link:hover,body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-item.active .sidebar-link{
 color:#3b7ddd !important;background:#eef4fd !important;border-left-color:#3b7ddd}
body[data-sidebar-layout=compact] .sidebar-dropdown .sidebar-link span{display:inline !important}
body[data-sidebar-layout=compact] .sidebar:not(.collapsed) ~ .main .content-footer{left:74px}
body[data-sidebar-layout=compact] .sidebar-item:hover{transform:none}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-dropdown{left:calc(100% + 12px) !important;top:-.25rem}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item::after{content:"";position:absolute;top:0;left:100%;width:16px;height:100%}
body[data-sidebar-layout=compact] .sidebar-nav>.sidebar-item>.sidebar-dropdown{max-height:calc(100vh - 1rem);overflow-y:auto}
body[data-sidebar-layout=compact] .sidebar{background:#222e3c;z-index:1010}
body[data-sidebar-layout=compact] .sidebar-content{position:sticky;top:0;height:100vh}
`

// sidebarModeScript gives the hamburger three states on wide screens: full
// sidebar, icons only, hidden (remembered in the gmd_nav cookie); narrow
// screens and app mode keep the template's show/hide.
const sidebarModeScript = `(function () {
	var body = document.body, sb = document.getElementById('sidebar');
	if (!sb || body.classList.contains('gmd-app')) { return; }
	var get = function () { var m = /(?:^|; )gmd_nav=(\w+)/.exec(document.cookie); return m ? m[1] : 'full'; };
	var wide = function () { return window.innerWidth >= 992; };
	var apply = function (s) {
		if (!wide() || s !== 'icons') { body.removeAttribute('data-sidebar-layout'); } else { body.setAttribute('data-sidebar-layout', 'compact'); }
		if (wide()) { sb.classList.toggle('collapsed', s === 'hidden'); }
	};
	sb.querySelectorAll('.sidebar-link').forEach(function (a) { if (!a.title) { a.title = a.textContent.trim(); } });
	// Icons only: a flyout that would run off the bottom moves up into view.
	sb.querySelectorAll('.sidebar-nav > .sidebar-item').forEach(function (it) {
		var dd = it.querySelector(':scope > .sidebar-dropdown');
		if (!dd) { return; }
		it.addEventListener('mouseenter', function () {
			dd.style.top = '';
			if (body.getAttribute('data-sidebar-layout') !== 'compact') { return; }
			var over = dd.getBoundingClientRect().bottom - (window.innerHeight - 8);
			if (over > 0) { dd.style.top = (parseFloat(getComputedStyle(dd).top) - over) + 'px'; }
		});
	});
	apply(get());
	document.addEventListener('click', function (e) {
		if (!e.target.closest('.js-sidebar-toggle') || !wide()) { return; }
		e.preventDefault();
		e.stopImmediatePropagation();
		var s = { full: 'icons', icons: 'hidden', hidden: 'full' }[get()] || 'icons';
		document.cookie = 'gmd_nav=' + s + '; max-age=31536000; path=/; SameSite=Lax';
		apply(s);
		window.dispatchEvent(new Event('resize'));
	}, true);
	window.addEventListener('resize', function () { apply(get()); });
})();`

// appStyle hides what only admins manage (the player is the end-user
// view, for admins too) and tightens the page.
const appStyle = `
body.gmd-app .navbar-bg{min-height:48px;padding-top:.25rem;padding-bottom:.25rem}
body.gmd-app .content{padding:1rem 1.25rem}
body.gmd-app .lib-admin{display:none !important}
`

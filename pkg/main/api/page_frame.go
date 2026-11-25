package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// faviconLinks returns the favicon / app-icon <link> tags shared by every page.
// Sized PNGs (generated from app-icon.png) let the browser pick a small file for
// the tab and a larger one for the home-screen icon; the full-size app-icon.png
// is listed last as a fallback so an icon always shows even before the sized
// files have been generated.
func faviconLinks() gomponents.Node {
	const dir = "/static/img/icons/"

	sized := func(rel, size string) gomponents.Node {
		return html.Link(
			html.Rel(rel),
			html.Type("image/png"),
			gomponents.Attr("sizes", size),
			html.Href(dir+"icon-"+size+".png"),
		)
	}

	return gomponents.Group([]gomponents.Node{
		sized("icon", "16x16"),
		sized("icon", "32x32"),
		sized("icon", "192x192"),
		html.Link(
			html.Rel("apple-touch-icon"),
			gomponents.Attr("sizes", "180x180"),
			html.Href(dir+"icon-180x180.png"),
		),
		// Fallback used when none of the sized files exist yet.
		html.Link(html.Rel("icon"), html.Type("image/png"), html.Href(dir+"app-icon.png")),
		// Installable as an app (home screen, own window).
		html.Link(html.Rel("manifest"), html.Href("/static/manifest.webmanifest")),
		html.Meta(html.Name("theme-color"), html.Content("#222e3c")),
		html.Meta(html.Name("mobile-web-app-capable"), html.Content("yes")),
		html.Meta(html.Name("apple-mobile-web-app-capable"), html.Content("yes")),
		html.Meta(html.Name("apple-mobile-web-app-title"), html.Content("Go Media")),
		html.Script(gomponents.Raw("if ('serviceWorker' in navigator) { navigator.serviceWorker.register('/sw.js').catch(function () {}); }")),
	})
}

// csrfAutoAttachScript adds the session's CSRF token (from the JS-readable
// csrf_token cookie set by requireAuth) to every same-origin state-changing
// htmx, fetch and jQuery request that doesn't already carry one, so no
// individual form or button can forget it.
func csrfAutoAttachScript() gomponents.Node {
	return html.Script(gomponents.Raw(`
(function () {
	function gmdCsrfToken() {
		var m = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/);
		if (m) { return decodeURIComponent(m[1]); }
		if (window.csrfToken) { return window.csrfToken; }
		var el = document.querySelector('input[name="csrf_token"]');
		return el ? el.value : '';
	}
	window.gmdCsrfToken = gmdCsrfToken;
	function unsafeMethod(m) { return !/^(GET|HEAD|OPTIONS)$/i.test(m || 'GET'); }
	function sameOrigin(u) {
		try { return new URL(u, window.location.href).origin === window.location.origin; } catch (e) { return false; }
	}
	document.addEventListener('htmx:configRequest', function (evt) {
		var d = evt.detail;
		if (unsafeMethod(d.verb) && !d.headers['X-CSRF-Token']) { d.headers['X-CSRF-Token'] = gmdCsrfToken(); }
	});
	if (window.fetch) {
		var origFetch = window.fetch;
		window.fetch = function (input, init) {
			var url = (typeof input === 'string' || input instanceof URL) ? String(input) : input.url;
			var method = (init && init.method) || (input && input.method) || 'GET';
			if (unsafeMethod(method) && sameOrigin(url)) {
				init = init || {};
				var h = new Headers(init.headers || (input instanceof Request ? input.headers : undefined));
				if (!h.get('X-CSRF-Token')) { h.set('X-CSRF-Token', gmdCsrfToken()); }
				init.headers = h;
			}
			return origFetch.call(this, input, init);
		};
	}
	if (window.jQuery) {
		jQuery.ajaxPrefilter(function (options) {
			if (unsafeMethod(options.type || options.method) && sameOrigin(options.url)) {
				options.headers = options.headers || {};
				if (!options.headers['X-CSRF-Token']) { options.headers['X-CSRF-Token'] = gmdCsrfToken(); }
			}
		});
	}
})();
`))
}

// pageHead is the document head shared by full pages and embedded tool pages.
func pageHead(pageTitle string) gomponents.Node {
	return html.Head(
		html.Meta(html.Charset("utf-8")),
		html.Meta(
			html.Name("viewport"),
			html.Content("width=device-width, initial-scale=1"),
		),
		html.TitleEl(gomponents.Text(pageTitle)),
		faviconLinks(),

		// Load jQuery first
		html.Script(html.Src("https://code.jquery.com/jquery-3.7.1.min.js")),
		themeHead(), // AdminKit light.css / dark.css
		// Choices.js CSS and JS for searchable dropdowns
		// html.Link(html.Rel("stylesheet"), html.Href("https://cdn.jsdelivr.net/npm/choices.js/public/assets/styles/choices.min.css")),
		html.Script(html.Src("/static/js/app.js")),
		html.Script(html.Src("/static/js/datatables.js")),
		// html.Script(html.Src("https://cdn.jsdelivr.net/npm/choices.js/public/assets/scripts/choices.min.js")),

		html.Script(html.Src("https://unpkg.com/htmx.org@2.0.11/dist/htmx.min.js")),
		csrfAutoAttachScript(),
		addCSS(),
		themeStyle(),
		helperToolScript(),
		// adminStyles(),
	)
}

// renderToolPage writes a helper tool page. With ?embed=1 it renders only the
// tool, without sidebar and footer, for the overlay that config fields open
// (helperToolScript).
func renderToolPage(ctx *gin.Context, title string, content gomponents.Node) {
	renderPageIn(ctx, title, true, content)
}

// renderPageIn is renderToolPage for a page that is not in the Management
// menu (management false: no submenu is opened for it).
func renderPageIn(ctx *gin.Context, title string, management bool, content gomponents.Node) {
	var node gomponents.Node
	if ctx.Query("embed") == "1" {
		node = html.Doctype(html.HTML(
			html.Lang("en"),
			pageHead(title+" · Media Downloader"),
			html.Body(
				html.Class("helper-embed"),
				html.Main(html.Class("content p-3"),
					html.Div(html.Class("container-fluid p-0"), html.Div(html.Class("row"), content))),
			),
		))
	} else {
		node = pageAs(accessOf(ctx), title, false, false, management, content)
	}

	var buf strings.Builder
	node.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func page(
	title string,
	activeConfig bool,
	activeDatabase bool,
	activeManagement bool,
	addcontent ...gomponents.Node,
) gomponents.Node {
	return pageAs(nil, title, activeConfig, activeDatabase, activeManagement, addcontent...)
}

// pageAs is page for an account: non-admins get their reduced sidebar, and
// the top bar names the account (nil: an admin page, no name).
func pageAs(
	acc *userAccess,
	title string,
	activeConfig bool,
	activeDatabase bool,
	activeManagement bool,
	addcontent ...gomponents.Node,
) gomponents.Node {
	pageTitle := "Media Downloader Management"
	if strings.TrimSpace(title) != "" {
		pageTitle = title + " · Media Downloader"
	}

	app := acc != nil && acc.App
	tv := acc != nil && acc.TV
	// Not inside gomponents.If below: its arguments are evaluated even when
	// acc is nil (the admin pages built by page()).
	canReport := acc != nil && acc.CanReport()

	return html.Doctype(
		html.HTML(
			html.Lang("en"),
			pageHead(pageTitle),
			html.Body(
				gomponents.If(app && !tv, html.Class("gmd-app")),
				gomponents.If(tv, html.Class("gmd-app gmd-tv")),
				gomponents.If(tv, html.StyleEl(gomponents.Raw(tvStyle))),
				html.StyleEl(gomponents.Raw(iconSidebarStyle)),
				gomponents.If(app, html.StyleEl(gomponents.Raw(appStyle))),
				html.A(
					html.Href("#main-content"),
					html.Class("visually-hidden-focusable"),
					html.Style(
						"position:absolute; left:8px; top:8px; z-index:2000; background:#fff; padding:0.5rem 1rem; border-radius:6px;",
					),
					gomponents.Text("Skip to main content"),
				),
				html.Div(html.Class("wrapper"),
					createNavbarFor(acc, activeDatabase),
					gomponents.If(acc != nil, html.Script(gomponents.Raw("window.gmdUser = "+jsString(accName(acc))+";"+
						"window.gmdSettings = "+accountSettingsJS(acc)+";"+
						"window.gmdNetMinutes = "+strconv.Itoa(config.GetSettingsGeneral().PlayerSpeedTestMinutes)+";"+
						"window.gmdCanReport = "+strconv.FormatBool(canReport)+";"+themeSyncScript))),
					html.Script(gomponents.Raw(sidebarModeScript)),
					html.Div(
						html.Class("main"),
						html.Nav(html.Class("navbar navbar-expand navbar-light navbar-bg"),
							html.A(html.Class("sidebar-toggle js-sidebar-toggle"),
								html.Href("#"),
								html.Role("button"),
								gomponents.Attr("aria-label", "Toggle navigation sidebar"),
								html.I(
									html.Class("hamburger align-self-center"),
									gomponents.Attr("aria-hidden", "true"),
								),
							),
							accountMenu(acc),
						),
						html.Main(
							html.Class("content"),
							html.ID("main-content"),
							html.TabIndex("-1"),
							html.Div(html.Class("container-fluid p-0"),
								// html.H1(html.Class("h3 mb-3"), gomponents.Text(headertext)),
								html.Div(
									append([]gomponents.Node{html.Class("row")}, addcontent...)...),
							),
						),
						gomponents.If(!app, html.Footer(
							html.Class("content-footer"),
							// System Status Footer
							html.Div(
								html.Class("bg-dark text-white py-4"),
								html.Div(
									html.Class("container"),
									html.Div(
										html.Class("row align-items-center"),
										html.Div(
											html.Class("col-md-6"),
											html.P(
												html.Class("mb-0"),
												gomponents.Text(copyrightLine(time.Now().Year())),
											),
										),
										html.Div(
											html.Class("col-md-6 text-md-end"),
											html.A(
												html.Href(
													"https://github.com/Kellerman81/go_media_downloader",
												),
												html.Target("_blank"),
												html.Class("btn btn-outline-light btn-sm me-3"),
												html.Rel("noopener noreferrer"),
												html.I(html.Class("fab fa-github me-1")),
												gomponents.Text("GitHub"),
											),
											html.Span(
												html.Class("badge bg-success me-2"),
												html.I(html.Class("fas fa-circle me-1")),
												gomponents.Text("System Online"),
											),
											renderBuildFooter(acc == nil || acc.Admin()),
										),
									),
								),
							),
						)),
					),
				),

				// Toast notification container
				html.Div(
					html.Class("toast-container position-fixed top-0 end-0 p-3"),
					html.ID("toastContainer"),
					html.Style("z-index: 1055;"),
				),

				// Config Preview Modal
				html.Div(
					html.ID("configPreviewModal"),
					html.Class("modal fade"),
					html.TabIndex("-1"),
					gomponents.Attr("aria-labelledby", "configPreviewModalLabel"),
					gomponents.Attr("aria-hidden", "true"),
					html.Div(
						html.Class("modal-dialog modal-lg modal-dialog-scrollable"),
						html.Div(
							html.Class("modal-content"),
							html.Div(
								html.Class("modal-header bg-light"),
								html.H5(
									html.ID("configPreviewModalLabel"),
									html.Class("modal-title"),
									html.I(html.Class("fas fa-eye me-2 text-primary")),
									gomponents.Text("Configuration Preview"),
								),
								html.Button(
									html.Type("button"),
									html.Class("btn-close"),
									html.Data("bs-dismiss", "modal"),
									gomponents.Attr("aria-label", "Close"),
								),
							),
							html.Div(
								html.ID("configPreviewBody"),
								html.Class("modal-body"),
								html.Div(
									html.Class("d-flex justify-content-center"),
									html.Div(
										html.Class("spinner-border text-primary"),
										html.Role("status"),
										html.Span(
											html.Class("visually-hidden"),
											gomponents.Text("Loading..."),
										),
									),
								),
							),
							html.Div(
								html.Class("modal-footer"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-secondary"),
									html.Data("bs-dismiss", "modal"),
									gomponents.Text("Close"),
								),
							),
						),
					),
				),

				// Config Preview JavaScript
				html.Script(gomponents.Raw(`
function showConfigPreview(configType, configName, fieldName) {
    if (!configName) {
        return;
    }

    const modal = new bootstrap.Modal(document.getElementById('configPreviewModal'));
    const modalBody = document.getElementById('configPreviewBody');
    const modalLabel = document.getElementById('configPreviewModalLabel');

    // Show loading spinner
    modalBody.innerHTML = '<div class="d-flex justify-content-center p-5"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">Loading...</span></div></div>';

    // Update modal title
    modalLabel.innerHTML = '<i class="fas fa-eye me-2 text-primary"></i>Preview: ' + fieldName + ' - ' + configName;

    // Show modal
    modal.show();

    // Fetch preview content
    fetch('/api/manage/config/preview/' + configType + '/' + encodeURIComponent(configName))
        .then(response => {
            if (!response.ok) { throw new Error('the server answered ' + response.status); }
            return response.text();
        })
        .then(html => {
            modalBody.innerHTML = html;
            // After inserting HTML, expand all collapsed sections
            expandAllCollapsedSections(modalBody);
        })
        .catch(error => {
            modalBody.innerHTML = '<div class="alert alert-danger">Error loading preview: ' + error.message + '</div>';
        });
}

// Helper function to expand all collapsed sections in a container
function expandAllCollapsedSections(container) {
    if (!container) return;

    // Add 'show' class to the collapsed sections; field help boxes stay closed.
    container.querySelectorAll('.collapse:not([id^="help-"])').forEach(function(el) {
        el.classList.add('show');
    });

    // Update accordion buttons to show expanded state
    container.querySelectorAll('.accordion-button').forEach(function(el) {
        el.classList.remove('collapsed');
        el.setAttribute('aria-expanded', 'true');
    });

    // Also handle any buttons with data-bs-toggle="collapse"
    container.querySelectorAll('[data-bs-toggle="collapse"]:not(.field-help-toggle)').forEach(function(el) {
        el.classList.remove('collapsed');
        el.setAttribute('aria-expanded', 'true');
    });
}

// Open config page with hash anchor for auto-expand
function openConfigPage(baseUrl, configName) {
    if (!configName) {
        window.open(baseUrl, '_blank');
        return;
    }
    // Create URL with hash anchor for the config name
    // The config pages will use this to auto-expand the correct item
    window.open(baseUrl + '#' + encodeURIComponent(configName), '_blank');
}

// On page load, check for hash anchor and expand the matching config item
document.addEventListener('DOMContentLoaded', function() {
    // The Library keeps its open entry in the hash (#album/12) itself.
    if (document.getElementById('libModal')) { return; }
    if (window.location.hash) {
        var configName = decodeURIComponent(window.location.hash.substring(1));
        if (configName) {
            // Expand the item whose header names exactly this config
            // ("List SeriesX"), not every header that contains the text.
            setTimeout(function() {
                var btn = Array.prototype.find.call(
                    document.querySelectorAll('#main-content .accordion-button, #main-content [data-bs-toggle="collapse"]'),
                    function(b) {
                        var text = (b.textContent || '').replace(/\s+/g, ' ').trim();
                        return text === configName || text.endsWith(' ' + configName);
                    });
                var target = null;
                try {
                    target = btn && document.querySelector(btn.getAttribute('data-bs-target') || btn.getAttribute('href'));
                } catch (e) { /* the header points to no valid selector */ }
                if (!target) {
                    if (typeof showToaster === 'function') { showToaster('warning', configName + ' was not found on this page'); }
                    return;
                }
                bootstrap.Collapse.getOrCreateInstance(target, {toggle: false}).show();
                btn.classList.remove('collapsed');
                btn.setAttribute('aria-expanded', 'true');
                btn.scrollIntoView({behavior: 'smooth', block: 'center'});
                btn.style.backgroundColor = '#fff3cd';
                setTimeout(function() { btn.style.backgroundColor = ''; }, 2000);
            }, 300); // Small delay to ensure DOM is ready
        }
    }
});
`)),

				adminJavaScript(),
				gomponents.If(tv, html.Script(gomponents.Raw(tvScript))),
			),
		),
	)
}

// copyrightLine is the footer's notice, from the first year to now.
func copyrightLine(year int) string {
	years := "2025"
	if year > 2025 {
		years += "–" + strconv.Itoa(year)
	}

	return "© " + years + " Go Media Downloader - Advanced Media Automation"
}

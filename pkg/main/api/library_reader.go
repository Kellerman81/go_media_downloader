package api

import (
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The Library's reader: EPUB (epub.js) and PDF (pdf.js) books open in a
// dialog, a page at a time. Where the reader stands is the account's play history
// of the book file (kind "book", position = percent): the book counts as
// read from 95% on, like a film as watched.

// readerKinds are the book formats the reader opens.
var readerKinds = map[string]string{".epub": "epub", ".pdf": "pdf"}

// readerKind is how the reader opens a book file ("" not at all).
func readerKind(loc string) string {
	return readerKinds[strings.ToLower(path.Ext(strings.ReplaceAll(loc, "\\", "/")))]
}

// HandleLibraryRead sends a book file to read in the browser
// (/admin/library/read/:id/*name, a book_files ID).
func HandleLibraryRead(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || !accessOf(c).allowsFile(libKindByType("book"), &id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	loc := database.Getdatarow[string](false, "SELECT location FROM book_files WHERE id = ?", &id)
	if loc == "" || readerKind(loc) == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	c.Header("Content-Type", mediaContentType(loc))
	c.Header("Cache-Control", "private, max-age=3600")
	http.ServeFile(c.Writer, c.Request, loc)
}

// HandleLibraryReadProgress keeps where the account is in a book
// (/admin/library/read/progress: id, pct = 0..1).
func HandleLibraryReadProgress(c *gin.Context) {
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	pct, perr := strconv.ParseFloat(c.PostForm("pct"), 64)
	acc := accessOf(c)

	if err != nil || perr != nil || pct < 0 || pct > 1 || !acc.allowsFile(libKindByType("book"), &id) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid progress"})
		return
	}

	title := database.Getdatarow[string](false,
		"SELECT COALESCE(d.title, '') FROM book_files f INNER JOIN dbbooks d ON d.id = f.dbbook_id WHERE f.id = ?", &id)

	fid := uint(id)
	h := database.PlayHistory{Username: accName(acc), Kind: "book", FileID: fid, Title: title, Position: pct * 100, Duration: 100}

	if err := database.PlayProgress(&h, false); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

// libReadButton opens a book file in the reader, where the account
// stopped (a share of 0..1).
func libReadButton(fileID uint, loc, title string, at float64, small bool) gomponents.Node {
	kind := readerKind(loc)
	if kind == "" {
		return nil
	}

	class := "btn btn-sm btn-success"
	if small {
		class += " py-0 me-1"
	}

	return html.Button(html.Type("button"), html.Class(class), html.Title(t("Read book")),
		gomponents.Attr("onclick", "libRead("+strconv.FormatUint(uint64(fileID), 10)+", "+jsString(kind)+", "+jsString(title)+", "+
			strconv.FormatFloat(at, 'f', 4, 64)+", "+jsString(path.Base(strings.ReplaceAll(loc, "\\", "/")))+")"),
		html.I(html.Class("fa-solid fa-book-open"+map[bool]string{false: " me-1"}[small])), gomponents.If(!small, gomponents.Text(t("Read book"))))
}

// renderReaderModal is the reader's dialog: a page at a time like an
// e-reader, turned by tapping or swiping the page's sides, the arrow, page
// and space keys or the side buttons; the bar below jumps.
func renderReaderModal() gomponents.Node {
	tool := func(id, icon, title, onclick string) gomponents.Node {
		return html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary"), html.ID(id), html.Title(title),
			gomponents.Attr("aria-label", title), gomponents.Attr("onclick", onclick), html.I(html.Class("fa-solid "+icon)))
	}

	return html.Div(html.Class("modal fade"), html.ID("libReader"), html.TabIndex("-1"), gomponents.Attr("aria-labelledby", "libReaderTitle"),
		html.Div(html.Class("modal-dialog modal-fullscreen"),
			html.Div(html.Class("modal-content"),
				html.Div(html.Class("modal-header py-2 gap-2"),
					html.H5(html.Class("modal-title text-truncate me-auto"), html.ID("libReaderTitle")),
					html.Span(html.Class("small text-muted text-nowrap"), html.ID("libReaderPos")),
					html.Div(html.Class("dropdown"),
						html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary dropdown-toggle"), html.ID("libReaderToc"),
							gomponents.Attr("data-bs-toggle", "dropdown"), html.Title(t("Contents")), gomponents.Attr("aria-label", t("Contents")),
							html.I(html.Class("fa-solid fa-list"))),
						html.Ul(html.Class("dropdown-menu dropdown-menu-end"), html.ID("libReaderTocMenu"), html.Style("max-height:70vh;overflow-y:auto"))),
					tool("libReaderSmaller", "fa-minus", t("Smaller text"), "libReaderFont(-10)"),
					tool("libReaderLarger", "fa-plus", t("Larger text"), "libReaderFont(10)"),
					tool("libReaderFit", "fa-arrows-left-right", t("Fit width / whole page"), "libReaderToggleFit()"),
					tool("libReaderSpread", "fa-book-open", t("One page / two pages"), "libReaderToggleSpread()"),
					tool("libReaderFull", "fa-expand", t("Full screen"), "libReaderFullscreen()"),
					html.Button(html.Type("button"), html.Class("btn-close ms-1"), gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", t("Close")))),
				// overflow hidden: a scrollbar showing for a moment while a
				// chapter is laid out narrows the page, and epub.js lays the
				// chapter out again at the new width between two pages.
				// The page between the turn buttons; its width is the reader's
				// choice (the grips beside it), the buttons take the rest.
				html.Div(html.Class("modal-body p-0 d-flex"), html.ID("libReaderBody"), html.Style("min-height:0;overflow:hidden"),
					html.Button(html.Type("button"), html.Class("btn btn-link fs-3 px-2 d-none d-md-block lib-reader-side"), html.ID("libReaderPrev"),
						html.Title(t("Previous page (←)")), gomponents.Attr("onclick", "libReaderTurn(-1)"), html.I(html.Class("fa-solid fa-chevron-left"))),
					libReaderGrip(),
					html.Div(html.ID("libReaderArea"), html.Class("position-relative"), html.Style("flex:1 1 auto;margin:0 auto;min-width:0;overflow:hidden;touch-action:pan-y pinch-zoom")),
					libReaderGrip(),
					html.Button(html.Type("button"), html.Class("btn btn-link fs-3 px-2 d-none d-md-block lib-reader-side"), html.ID("libReaderNext"),
						html.Title(t("Next page (→)")), gomponents.Attr("onclick", "libReaderTurn(1)"), html.I(html.Class("fa-solid fa-chevron-right")))),
				html.Div(html.Class("modal-footer py-1"),
					html.Input(html.Type("range"), html.Class("form-range"), html.ID("libReaderSeek"), html.Min("0"), html.Max("1000"), html.Value("0"),
						gomponents.Attr("aria-label", t("Position in the book")), gomponents.Attr("onchange", "libReaderSeekTo(this.value / 1000)"))))))
}

// libReaderGrip is a handle beside the page: dragged, it makes the page
// narrower or wider on both sides; a double click gives it the full width.
func libReaderGrip() gomponents.Node {
	title := t("Drag to change the width of the page, double-click for the full width")

	return html.Div(html.Class("lib-reader-grip d-none d-md-block"), html.Title(title), html.Role("separator"),
		gomponents.Attr("aria-orientation", "vertical"), gomponents.Attr("aria-label", title),
		gomponents.Attr("onpointerdown", "libReaderGripStart(event)"), gomponents.Attr("ondblclick", "libReaderSetWidth(0)"))
}

// readerCSS styles the grips: a thin bar, wider to grab.
const readerCSS = `.lib-reader-grip{flex:0 0 10px;cursor:col-resize;position:relative;touch-action:none}
.lib-reader-grip::after{content:"";position:absolute;top:50%;left:3px;width:4px;height:48px;margin-top:-24px;border-radius:2px;background:var(--bs-secondary-color);opacity:.35}
.lib-reader-grip:hover::after,.lib-reader-grip.active::after{opacity:.8}
.lib-reader-side{flex:0 0 auto}
#libReaderBody.lib-reader-narrow .lib-reader-side{flex:1 1 0}`

// readerJS reads books: EPUB with epub.js, PDF with pdf.js (both loaded on
// first use). Where the reader stands is saved as the read share: EPUB the
// position in the book, PDF page / pages, so the same page opens again.
const readerJS = `
var libReaderState = null;
// libReaderAt is where each book was left in this page: newer than the
// position the read buttons were rendered with.
var libReaderAt = {};

function libReaderLoad(src) {
	return new Promise(function (ok, fail) {
		var s = document.createElement('script');
		s.src = src; s.onload = ok; s.onerror = function () { fail(new Error(src)); };
		document.head.appendChild(s);
	});
}

function libReaderEpubScripts() {
	if (window.ePub) { return Promise.resolve(); }
	return libReaderLoad('/static/js/jszip.min.js').then(function () { return libReaderLoad('/static/js/epub.min.js'); });
}

function libReaderPdfLib() {
	if (window.libPdfjs) { return Promise.resolve(window.libPdfjs); }
	return import('/static/js/pdfjs/pdf.min.mjs').then(function (m) {
		m.GlobalWorkerOptions.workerSrc = '/static/js/pdfjs/pdf.worker.min.mjs';
		window.libPdfjs = m;
		return m;
	});
}

function libReaderPref(key, def) {
	try { var v = localStorage.getItem('gmd-reader-' + key); return v === null ? def : v; } catch (e) { return def; }
}

function libReaderSetPref(key, v) {
	try { localStorage.setItem('gmd-reader-' + key, String(v)); } catch (e) {}
}

function libReaderSave(force, leaving) {
	var st = libReaderState;
	if (!st || st.pct === null) { return; }
	libReaderAt[st.id] = st.pct;
	var now = Date.now();
	if (!force && now - st.saved < 5000) { return; }
	st.saved = now;
	var form = new URLSearchParams({ id: String(st.id), pct: String(Math.min(Math.max(st.pct, 0), 1)), csrf_token: libCsrf });
	// keepalive: the last save on closing survives leaving the page right after.
	fetch('/api/admin/library/read/progress', { method: 'POST', credentials: 'same-origin', body: form, keepalive: !!(force || leaving),
		headers: { 'X-CSRF-Token': libCsrf } }).catch(function () {});
}

// libReaderShow puts where the reader stands into the header and the bar.
function libReaderShow(text, share) {
	document.getElementById('libReaderPos').textContent = text;
	var seek = document.getElementById('libReaderSeek');
	if (document.activeElement !== seek) { seek.value = String(Math.round(share * 1000)); }
}

function libReaderToc(entries) {
	var menu = document.getElementById('libReaderTocMenu');
	menu.innerHTML = '';
	document.getElementById('libReaderToc').classList.toggle('d-none', !entries.length);
	entries.forEach(function (e) {
		var li = document.createElement('li'), a = document.createElement('button');
		a.type = 'button'; a.className = 'dropdown-item text-truncate'; a.style.paddingLeft = (1 + e.level) + 'rem';
		a.textContent = e.label; a.onclick = e.go;
		li.appendChild(a); menu.appendChild(li);
	});
}

function libRead(id, kind, title, at, name) {
	var modal = document.getElementById('libReader'), area = document.getElementById('libReaderArea');
	var url = '/api/admin/library/read/' + id + '/' + encodeURIComponent(name);
	if (libReaderAt[id] !== undefined) { at = libReaderAt[id] >= 0.95 ? 0 : libReaderAt[id]; }
	document.getElementById('libReaderTitle').textContent = title;
	libReaderShow('', 0);
	libReaderToc([]);
	area.innerHTML = '<div class="text-center text-muted p-5">' + _t('Loading…') + '</div>';
	area.onclick = null;
	['libReaderSmaller', 'libReaderLarger'].forEach(function (b) { document.getElementById(b).classList.toggle('d-none', kind !== 'epub'); });
	document.getElementById('libReaderFit').classList.toggle('d-none', kind !== 'pdf');
	libReaderState = { id: id, kind: kind, pct: null, saved: 0 };
	libReaderSpreadIcon();
	libReaderApplyWidth();
	// Pages are measured and drawn once the dialog has its size.
	libReaderState.shown = modal.classList.contains('show') ? Promise.resolve() :
		new Promise(function (ok) { modal.addEventListener('shown.bs.modal', ok, { once: true }); });
	bootstrap.Modal.getOrCreateInstance(modal).show();

	var open = kind === 'pdf' ? libReaderOpenPdf(url, at) : libReaderOpenEpub(url, at);
	open.catch(function (e) { area.innerHTML = ''; libNotify('error', _t('Can\'t open the book: ') + e.message); });
}

// ---- EPUB ----

function libReaderOpenEpub(url, at) {
	var st = libReaderState, area = document.getElementById('libReaderArea');
	return libReaderEpubScripts().then(function () { return st.shown; }).then(function () {
		if (libReaderState !== st) { return; }
		area.innerHTML = '';
		var book = ePub(url, { openAs: 'epub' });
		var rendition = book.renderTo(area, { width: '100%', height: '100%', flow: 'paginated', spread: libReaderSpread('epub') ? 'auto' : 'none' });
		st.book = book; st.rendition = rendition;
		if (document.documentElement.getAttribute('data-gmd-dark') === '1') {
			rendition.themes.register('dark', { body: { color: '#e9ecef !important', background: '#19222c !important' }, a: { color: '#6ea8fe !important' } });
			rendition.themes.select('dark');
		}
		rendition.themes.fontSize(libReaderPref('font', '100') + '%');
		rendition.on('keyup', libReaderKey);
		// A tap on the page's outer thirds turns it; a swipe as well. The
		// chapters are documents in iframes: their clicks are measured from
		// the iframe's place in the page (epub.js scrolls a wide iframe).
		rendition.hooks.content.register(function (contents) {
			var doc = contents.document;
			doc.addEventListener('click', function (e) {
				var frame = e.view && e.view.frameElement, r = area.getBoundingClientRect();
				if (!frame) { return; }
				libReaderTap(frame.getBoundingClientRect().left + e.clientX - r.left, r.width, e);
			});
			doc.addEventListener('touchstart', function (e) { st.touch = e.changedTouches[0].screenX; }, { passive: true });
			doc.addEventListener('touchend', function (e) { libReaderSwipe(e.changedTouches[0].screenX); });
		});
		// Taps beside the chapter's iframe (its margins) reach the page.
		area.onclick = function (e) {
			var r = area.getBoundingClientRect();
			libReaderTap(e.clientX - r.left, r.width, e);
		};
		rendition.on('relocated', function (loc) {
			if (!book.locations.length()) { return; }
			st.pct = loc.start.percentage;
			libReaderShow(Math.round(st.pct * 100) + '%', st.pct);
			libReaderSave(false);
		});
		book.loaded.navigation.then(function (nav) {
			var list = [];
			var walk = function (items, level) {
				items.forEach(function (it) {
					list.push({ label: it.label.trim(), level: level, go: function () { libReaderNav(function () { return rendition.display(it.href); }); } });
					if (it.subitems && level < 2) { walk(it.subitems, level + 1); }
				});
			};
			walk(nav.toc || [], 0);
			libReaderToc(list);
		}).catch(function () {});
		return book.ready.then(function () { return book.locations.generate(1600); }).then(function () {
			var start = at > 0 && at < 0.95 ? book.locations.cfiFromPercentage(at) : undefined;
			return rendition.display(start);
		});
	});
}

function libReaderFont(step) {
	var st = libReaderState;
	if (!st || !st.rendition) { return; }
	var size = Math.min(Math.max(parseInt(libReaderPref('font', '100'), 10) + step, 60), 250);
	libReaderSetPref('font', size);
	st.rendition.themes.fontSize(size + '%');
}

// libReaderSpread: two pages side by side (EPUB: on wide screens, its
// default; PDF: like a printed book, page 1 alone) or one.
function libReaderSpread(kind) {
	return libReaderPref('spread-' + kind, kind === 'epub' ? '2' : '1') === '2';
}

function libReaderSpreadIcon() {
	var st = libReaderState, two = st && libReaderSpread(st.kind);
	document.getElementById('libReaderSpread').querySelector('i').className = 'fa-solid ' + (two ? 'fa-book-open' : 'fa-file');
}

function libReaderToggleSpread() {
	var st = libReaderState;
	if (!st) { return; }
	libReaderSetPref('spread-' + st.kind, libReaderSpread(st.kind) ? '1' : '2');
	libReaderSpreadIcon();
	if (st.rendition) {
		var loc = st.rendition.currentLocation();
		st.rendition.spread(libReaderSpread('epub') ? 'auto' : 'none');
		if (loc && loc.start) { libReaderNav(function () { return st.rendition.display(loc.start.cfi); }); }
	}
	if (st.doc) { libReaderPdfPage(st.page); }
}

// libReaderNav runs EPUB page turns one after the other: a turn started
// while the last one still lays out its chapter lands between two pages.
function libReaderNav(fn) {
	var st = libReaderState;
	st.nav = (st.nav || Promise.resolve()).then(function () { return libReaderState === st ? fn() : null; }).catch(function () {});
	return st.nav;
}

// ---- PDF ----

function libReaderOpenPdf(url, at) {
	var st = libReaderState;
	return libReaderPdfLib().then(function (pdfjs) {
		return pdfjs.getDocument({ url: url, withCredentials: true, isEvalSupported: false,
			standardFontDataUrl: '/static/js/pdfjs/standard_fonts/' }).promise;
	}).then(function (doc) {
		if (libReaderState !== st) { doc.destroy(); return; }
		st.doc = doc; st.pages = doc.numPages;
		st.fit = libReaderPref('fit', 'page');
		var area = document.getElementById('libReaderArea');
		area.innerHTML = '<div class="lib-reader-page h-100 w-100 overflow-auto d-flex"><div class="m-auto d-flex">' +
			'<canvas class="shadow-sm" style="background:#fff"></canvas><canvas class="shadow-sm d-none" style="background:#fff"></canvas></div></div>';
		var box = area.firstChild;
		box.addEventListener('click', function (e) {
			var r = box.getBoundingClientRect();
			libReaderTap(e.clientX - r.left, r.width, e);
		});
		box.addEventListener('touchstart', function (e) { st.touch = e.changedTouches[0].screenX; }, { passive: true });
		box.addEventListener('touchend', function (e) { libReaderSwipe(e.changedTouches[0].screenX); });
		doc.getOutline().then(function (outline) { libReaderToc(libReaderPdfToc(doc, outline || [], 0)); }).catch(function () {});
		return st.shown.then(function () {
			return libReaderPdfPage(at > 0 && at < 0.95 ? Math.min(Math.max(Math.round(at * st.pages), 1), st.pages) : 1);
		});
	});
}

// libReaderPdfToc flattens the PDF's outline (three levels) into entries
// that jump to their page.
function libReaderPdfToc(doc, items, level) {
	var out = [];
	items.forEach(function (it) {
		out.push({ label: it.title, level: level, go: function () {
			var dest = typeof it.dest === 'string' ? doc.getDestination(it.dest) : Promise.resolve(it.dest);
			dest.then(function (d) { return d ? doc.getPageIndex(d[0]) : Promise.reject(); })
				.then(function (i) { libReaderPdfPage(i + 1); }).catch(function () {});
		} });
		if (it.items && it.items.length && level < 2) { out = out.concat(libReaderPdfToc(doc, it.items, level + 1)); }
	});
	return out;
}

// libReaderPdfPage shows page n (in two-page view the pair it is in: 1,
// 2-3, 4-5, ...), fitted to the whole page or the width, sharp on
// high-density screens.
function libReaderPdfPage(n) {
	var st = libReaderState;
	if (!st || !st.doc) { return Promise.resolve(); }
	n = Math.min(Math.max(n, 1), st.pages);
	var two = libReaderSpread('pdf');
	if (two && n > 1 && n % 2 === 1) { n--; }
	var last = two && n > 1 ? Math.min(n + 1, st.pages) : n;
	st.page = n; st.last = last;
	st.pct = n / st.pages;
	libReaderShow(last > n ? _tf('Pages %d–%d of %d', n, last, st.pages) : _tf('Page %d of %d', n, st.pages), st.pct);
	libReaderSave(false);
	var nums = last > n ? [n, last] : [n];
	return Promise.all(nums.map(function (i) { return st.doc.getPage(i); })).then(function (pages) {
		if (libReaderState !== st || st.page !== n) { return; }
		var box = document.querySelector('#libReaderArea .lib-reader-page'), canvases = box.querySelectorAll('canvas');
		var bases = pages.map(function (p) { return p.getViewport({ scale: 1 }); }), pad = 16;
		if (box.clientWidth <= pad || box.clientHeight <= pad) { return; } // not laid out (yet): the resize draws it
		var width = bases.reduce(function (w, b) { return w + b.width; }, 0), height = Math.max.apply(null, bases.map(function (b) { return b.height; }));
		var scale = (box.clientWidth - pad) / width;
		if (st.fit !== 'width') { scale = Math.min(scale, (box.clientHeight - pad) / height); }
		var ratio = window.devicePixelRatio || 1;
		(st.tasks || []).forEach(function (t) { t.cancel(); });
		canvases[1].classList.toggle('d-none', pages.length < 2);
		st.tasks = pages.map(function (page, i) {
			var vp = page.getViewport({ scale: scale }), canvas = canvases[i];
			canvas.width = Math.floor(vp.width * ratio); canvas.height = Math.floor(vp.height * ratio);
			canvas.style.width = Math.floor(vp.width) + 'px'; canvas.style.height = Math.floor(vp.height) + 'px';
			return page.render({ canvasContext: canvas.getContext('2d'), viewport: vp, transform: ratio !== 1 ? [ratio, 0, 0, ratio, 0, 0] : null });
		});
		box.scrollTop = 0;
		return Promise.all(st.tasks.map(function (t) { return t.promise; })).catch(function () {});
	});
}

function libReaderToggleFit() {
	var st = libReaderState;
	if (!st || !st.doc) { return; }
	st.fit = st.fit === 'width' ? 'page' : 'width';
	libReaderSetPref('fit', st.fit);
	libReaderPdfPage(st.page);
}

// ---- Turning pages ----

function libReaderTurn(dir) {
	var st = libReaderState;
	if (!st) { return; }
	if (st.rendition) { libReaderNav(function () { return dir < 0 ? st.rendition.prev() : st.rendition.next(); }); }
	if (st.doc) { libReaderPdfPage(dir < 0 ? st.page - 1 : st.last + 1); }
}

function libReaderSeekTo(share) {
	var st = libReaderState;
	if (!st) { return; }
	if (st.rendition && st.book && st.book.locations.length()) { libReaderNav(function () { return st.rendition.display(st.book.locations.cfiFromPercentage(share)); }); }
	if (st.doc) { libReaderPdfPage(Math.max(1, Math.round(share * st.pages))); }
}

// libReaderTap turns the page on a tap on its left or right third; links
// and selected text are left alone.
function libReaderTap(x, width, e) {
	if (!width || (e.target && e.target.closest && e.target.closest('a'))) { return; }
	var sel = e.view && e.view.getSelection ? e.view.getSelection() : window.getSelection();
	if (sel && String(sel).length) { return; }
	if (x < width * 0.3) { libReaderTurn(-1); } else if (x > width * 0.7) { libReaderTurn(1); }
}

function libReaderSwipe(endX) {
	var st = libReaderState;
	if (!st || st.touch === undefined) { return; }
	var dx = endX - st.touch;
	st.touch = undefined;
	if (Math.abs(dx) > 50) { libReaderTurn(dx < 0 ? 1 : -1); }
}

function libReaderKey(e) {
	if (!libReaderState || !document.getElementById('libReader').classList.contains('show')) { return; }
	if (e.target && /^(INPUT|SELECT|TEXTAREA)$/.test(e.target.tagName)) { return; }
	if (e.key === 'ArrowLeft' || e.key === 'PageUp' || (e.key === ' ' && e.shiftKey)) { libReaderTurn(-1); }
	if (e.key === 'ArrowRight' || e.key === 'PageDown' || (e.key === ' ' && !e.shiftKey)) { libReaderTurn(1); }
}

function libReaderFullscreen() {
	var modal = document.getElementById('libReader');
	if (document.fullscreenElement) { document.exitFullscreen(); } else if (modal.requestFullscreen) { modal.requestFullscreen(); }
}

document.addEventListener('keyup', libReaderKey);
// ---- Page width ----

// libReaderApplyWidth gives the page the chosen width (0: all there is);
// the turn buttons beside it take the rest.
function libReaderApplyWidth() {
	var w = parseInt(libReaderPref('width', '0'), 10) || 0, area = document.getElementById('libReaderArea');
	area.style.flex = w ? '0 1 ' + w + 'px' : '1 1 auto';
	document.getElementById('libReaderBody').classList.toggle('lib-reader-narrow', w > 0);
}

// libReaderRelayout lays the book out again at the page's new width.
function libReaderRelayout() {
	var st = libReaderState;
	if (!st) { return; }
	if (st.rendition) { libReaderNav(function () { return st.rendition.resize(); }); }
	if (st.doc) { libReaderPdfPage(st.page); }
}

function libReaderSetWidth(w) {
	libReaderSetPref('width', Math.round(w));
	libReaderApplyWidth();
	libReaderRelayout();
}

// libReaderGripStart drags a grip: the page keeps its middle and gets
// twice the distance from it to the pointer, at least 320 pixels; the full
// width when dragged to the window's edge.
function libReaderGripStart(e) {
	if (e.button !== 0) { return; }
	e.preventDefault();
	var grip = e.currentTarget, body = document.getElementById('libReaderBody'), area = document.getElementById('libReaderArea');
	var r = body.getBoundingClientRect(), mid = r.left + r.width / 2, w = 0;
	grip.setPointerCapture(e.pointerId);
	grip.classList.add('active');
	area.style.pointerEvents = 'none'; // the book's iframes would take the pointer
	var move = function (ev) {
		w = Math.max(320, Math.abs(ev.clientX - mid) * 2);
		if (w >= r.width - 80) { w = 0; }
		area.style.flex = w ? '0 1 ' + w + 'px' : '1 1 auto';
		body.classList.toggle('lib-reader-narrow', w > 0);
	};
	var end = function () {
		grip.removeEventListener('pointermove', move);
		grip.removeEventListener('pointerup', end);
		grip.removeEventListener('pointercancel', end);
		grip.classList.remove('active');
		area.style.pointerEvents = '';
		libReaderSetWidth(w);
	};
	grip.addEventListener('pointermove', move);
	grip.addEventListener('pointerup', end);
	grip.addEventListener('pointercancel', end);
}

window.addEventListener('resize', function () {
	var st = libReaderState;
	if (!st || !st.doc) { return; }
	clearTimeout(st.resize);
	st.resize = setTimeout(function () { libReaderPdfPage(st.page); }, 200);
});
window.addEventListener('pagehide', function () { libReaderSave(true, true); });
document.getElementById('libReader').addEventListener('hidden.bs.modal', function () {
	var st = libReaderState;
	libReaderSave(true);
	if (document.fullscreenElement) { document.exitFullscreen(); }
	if (st && st.book) { st.book.destroy(); }
	if (st && st.doc) { st.doc.destroy(); }
	libReaderState = null;
	document.getElementById('libReaderArea').innerHTML = '';
});
`

// readFrom is where to open a book the account read the share c of (from
// the start when it finished it).
func readFrom(c float64) float64 {
	if c >= database.Watched {
		return 0
	}

	return c
}

// libFirstReadButton opens the first file of a book the reader can show.
func libFirstReadButton(files []libFile, it *libItem) gomponents.Node {
	for _, f := range files {
		if readerKind(f.Location) != "" {
			return libReadButton(f.ID, f.Location, it.Title, readFrom(it.Watched[f.ID]), false)
		}
	}

	return nil
}

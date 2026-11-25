package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// TV mode: the end-user pages (app mode) for a TV's browser, driven by its
// remote control. ?tv=1 switches it on in that browser (a cookie, like app
// mode, which it switches on too), ?tv=0 off. The arrow keys move between
// the tiles and buttons by where they are on the screen, OK opens, Back
// closes the dialog, leaves the full-screen player or goes back; playing
// opens the player full screen, where the arrows seek and the media keys
// pause, play and wind.

const tvCookie = "gmd_tv"

// tvMode reports whether the request comes from a browser in TV mode.
func tvMode(c *gin.Context) bool {
	if q := c.Query("tv"); q != "" {
		return q == "1"
	}

	v, _ := c.Cookie(tvCookie)

	return v == "1"
}

// setTVMode stores ?tv=1/0 (appMode follows it).
func setTVMode(c *gin.Context) {
	switch c.Query("tv") {
	case "1":
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(tvCookie, "1", 10*365*24*3600, "/", "", requestIsHTTPS(c), true)
	case "0":
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(tvCookie, "", -1, "/", "", requestIsHTTPS(c), true)
	}
}

// tvStyle is the 10-foot layout: larger text and covers, a focus frame
// that shows across the room.
const tvStyle = `
html{font-size:125%}
body.gmd-tv :focus{outline:none}
body.gmd-tv .tv-focus,body.gmd-tv :focus-visible{outline:4px solid #ffc107!important;outline-offset:3px;border-radius:.35rem;
  box-shadow:0 0 0 8px rgba(255,193,7,.25)!important}
body.gmd-tv .lib-card.tv-focus,body.gmd-tv .lib-continue-card.tv-focus{transform:scale(1.06);transition:transform .12s}
body.gmd-tv .btn{min-height:2.4rem}
`

// tvScript moves the focus with the remote control.
const tvScript = `
(function () {
	var tiles = '.lib-card, .lib-continue-card, .lib-list-item, .list-group-item-action';
	var focusSel = 'a[href], button:not([disabled]), input:not([type=hidden]):not([disabled]), select:not([disabled]), textarea, [tabindex]:not([tabindex="-1"]), ' + tiles;
	var BACK = { 8: 1, 27: 1, 461: 1, 10009: 1 };

	function markTiles(root) {
		(root || document).querySelectorAll(tiles).forEach(function (el) { if (!el.hasAttribute('tabindex')) { el.tabIndex = 0; } });
	}
	markTiles();
	new MutationObserver(function () { markTiles(); }).observe(document.body, { childList: true, subtree: true });

	document.addEventListener('focusin', function (e) {
		document.querySelectorAll('.tv-focus').forEach(function (x) { x.classList.remove('tv-focus'); });
		if (e.target && e.target.classList) { e.target.classList.add('tv-focus'); }
	});

	function visible(el) {
		if (el.offsetParent === null && getComputedStyle(el).position !== 'fixed') { return false; }
		var r = el.getBoundingClientRect();
		return r.width > 0 && r.height > 0;
	}

	function scope() {
		var m = document.querySelector('.modal.show');
		return m || document;
	}

	function candidates() {
		return [].slice.call(scope().querySelectorAll(focusSel)).filter(function (el) {
			return visible(el) && !el.closest('.d-none, [aria-hidden=true]');
		});
	}

	function move(dir) {
		var all = candidates();
		if (!all.length) { return; }
		var cur = document.activeElement;
		if (!cur || all.indexOf(cur) < 0) { all[0].focus(); return; }
		var a = cur.getBoundingClientRect(), ax = a.left + a.width / 2, ay = a.top + a.height / 2;
		var best = null, bestScore = Infinity;
		all.forEach(function (el) {
			if (el === cur || el.contains(cur) || cur.contains(el)) { return; }
			var b = el.getBoundingClientRect(), bx = b.left + b.width / 2, by = b.top + b.height / 2;
			var dx = bx - ax, dy = by - ay, main, side;
			switch (dir) {
				case 'left': if (b.right > a.left + 2) { return; } main = -dx; side = Math.abs(dy); break;
				case 'right': if (b.left < a.right - 2) { return; } main = dx; side = Math.abs(dy); break;
				case 'up': if (b.bottom > a.top + 2) { return; } main = -dy; side = Math.abs(dx); break;
				default: if (b.top < a.bottom - 2) { return; } main = dy; side = Math.abs(dx);
			}
			var score = main + side * 2;
			if (score < bestScore) { bestScore = score; best = el; }
		});
		if (best) { best.focus(); best.scrollIntoView({ block: 'nearest', inline: 'nearest' }); }
	}

	function playerFull() {
		var p = document.getElementById('libPlayer');
		return p && p.classList.contains('lp-fullwin');
	}

	function media(code) {
		if (typeof lp === 'undefined' || !lp || !lp.el) { return false; }
		switch (code) {
			case 415: case 19: case 10252: case 179: lpToggle(); return true;
			case 417: case 228: lpJump(30); return true;
			case 412: case 227: lpJump(-30); return true;
			case 413: case 178: lpClose(); return true;
		}
		return false;
	}

	document.addEventListener('keydown', function (e) {
		if (e.ctrlKey || e.metaKey || e.altKey) { return; }
		if (media(e.keyCode)) { e.preventDefault(); e.stopPropagation(); return; }

		var t = e.target, tag = t && t.tagName, typing = tag === 'INPUT' || tag === 'TEXTAREA' || (t && t.isContentEditable);

		if (BACK[e.keyCode] && !(typing && e.keyCode === 8)) {
			e.preventDefault(); e.stopPropagation();
			var m = document.querySelector('.modal.show');
			if (m && window.bootstrap) { bootstrap.Modal.getOrCreateInstance(m).hide(); return; }
			if (playerFull()) { lpFullWindow(false); return; }
			history.back();
			return;
		}

		// The full-screen player keeps its keys (arrows seek, OK pauses).
		if (playerFull()) {
			if (e.key === 'Enter') { e.preventDefault(); e.stopPropagation(); lpToggle(); }
			return;
		}

		var dir = { ArrowLeft: 'left', ArrowRight: 'right', ArrowUp: 'up', ArrowDown: 'down' }[e.key];
		if (dir) {
			// Left and right move the cursor in a text field.
			if (typing && (dir === 'left' || dir === 'right')) { return; }
			e.preventDefault(); e.stopPropagation();
			move(dir);
			return;
		}

		// OK on a list opens it (TV browsers show selects as dialogs).
		if (e.key === 'Enter' && tag === 'SELECT' && t.showPicker) {
			e.preventDefault(); e.stopPropagation();
			try { t.showPicker(); } catch (err) { /* not offered */ }
			return;
		}

		if (e.key === 'Enter' && t && t.matches && t.matches(tiles)) {
			e.preventDefault(); e.stopPropagation();
			t.click();
		}
	}, true);

	// An opened dialog takes the focus: its first green (play) button, else
	// its first control.
	document.addEventListener('shown.bs.modal', function (e) {
		var m = e.target;
		setTimeout(function () {
			var first = m.querySelector('.modal-body .btn-success') || [].slice.call(m.querySelectorAll(focusSel)).filter(visible)[0];
			if (first) { first.focus(); }
		}, 300);
	});

	// A video starting opens the player full screen (media events don't
	// bubble: caught on the way down).
	document.addEventListener('play', function (e) {
		var p = document.getElementById('libPlayer');
		if (p && e.target && e.target.tagName === 'VIDEO' && p.contains(e.target) && !p.classList.contains('lp-fullwin') &&
			typeof lpFullWindow === 'function') { lpFullWindow(true); }
	}, true);

	// Bigger covers once, unless the account chose a size.
	if (typeof libSetCoverSize === 'function' && typeof lpStore === 'function' && !lpStore('lib-cover-size')) { libSetCoverSize(220); }

	setTimeout(function () { var c = candidates(); var first = document.querySelector('#libGrid ' + tiles) || c[0]; if (first && document.activeElement === document.body) { first.focus(); } }, 600);
})();
`

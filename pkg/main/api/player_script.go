package api

import (
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// playerDock is the element the bottom-bar player (playerJS) lives in.
func playerDock() gomponents.Node {
	return html.Div(html.ID("libPlayer"), html.Class("lp-dock d-none"), html.Role("region"), gomponents.Attr("aria-label", t("Player")))
}

// playerCSS and playerJS are the bottom-bar player, shared by the Library
// and the calendar. The page provides an element #libPlayer (playerDock).
const playerCSS = `
#libPlayer .lp-shuffle.active,#libPlayer .lp-repeat.active{color:#fff;background:var(--bs-primary,#3b7ddd);border-color:var(--bs-primary,#3b7ddd)}
.lp-dock{position:fixed;left:0;right:0;bottom:0;z-index:1065;background:var(--bs-body-bg,#fff);border-top:1px solid #dee2e6;box-shadow:0 -4px 16px rgba(0,0,0,.15);padding:.5rem 1rem}
.lp-bar{display:flex;align-items:center;gap:.75rem}
.lp-tl{position:relative;height:24px;cursor:pointer;touch-action:none;outline:none}
.lp-tl-track{position:absolute;left:0;right:0;top:50%;height:4px;margin-top:-2px;border-radius:2px;background:rgba(128,128,128,.35);overflow:hidden;transition:height .1s,margin .1s}
.lp-tl:hover .lp-tl-track,.lp-tl-drag .lp-tl-track,.lp-tl:focus-visible .lp-tl-track{height:6px;margin-top:-3px}
.lp-tl-buf{position:absolute;top:0;bottom:0;background:rgba(59,125,221,.35)}
.lp-tl-fill{position:absolute;left:0;top:0;bottom:0;background:var(--bs-primary,#3b7ddd)}
.lp-tl-knob{position:absolute;top:50%;width:14px;height:14px;margin:-7px 0 0 -7px;border-radius:50%;background:var(--bs-primary,#3b7ddd);box-shadow:0 0 0 2px var(--bs-body-bg,#fff)}
.lp-tl:focus-visible .lp-tl-knob{box-shadow:0 0 0 2px var(--bs-body-bg,#fff),0 0 0 4px rgba(59,125,221,.5)}
.lp-tl-hover{position:absolute;top:4px;bottom:4px;width:2px;margin-left:-1px;background:rgba(128,128,128,.8);pointer-events:none}
.lp-tl-tip{position:absolute;bottom:100%;transform:translateX(-50%);background:#212529;color:#fff;font-size:.75rem;line-height:1.2;padding:.15rem .4rem;border-radius:.25rem;white-space:nowrap;pointer-events:none}
.lp-pos,.lp-dur{min-width:3.2em;font-variant-numeric:tabular-nums}
.lp-main{flex:1 1 auto;min-width:0;display:flex;flex-direction:column;gap:.2rem}
.lp-head{min-width:0}
.lp-media{flex:0 0 auto;cursor:pointer}
.lp-media video{width:160px;height:90px;object-fit:contain;display:block}
.lp-cover{width:64px;height:64px;object-fit:cover;border-radius:.375rem;display:block}
.lp-panel{display:none}
.lp-expanded .lp-panel{display:block}
.lp-expanded .lp-bar{flex-wrap:wrap}
.lp-expanded .lp-media{flex-basis:100%}
.lp-expanded .lp-media video{width:100%;height:auto;max-height:55vh}
.lp-expanded .lp-cover{width:160px;height:160px;margin:0 auto}
body.lp-docked{padding-bottom:var(--lp-h,0)}
body.lp-docked .modal-dialog{margin-bottom:calc(var(--lp-h,0px) + var(--bs-modal-margin,1.75rem))}
body.lp-docked .modal-dialog-scrollable{height:calc(100% - var(--bs-modal-margin,1.75rem)*2 - var(--lp-h,0px))}
body.lp-docked .modal-dialog-centered{min-height:calc(100% - var(--bs-modal-margin,1.75rem)*2 - var(--lp-h,0px))}
@media (max-width:575px){.lp-media video{width:96px;height:54px}.lp-cover{width:48px;height:48px}.lp-vol{display:none}}
#libPlayer:fullscreen,#libPlayer.lp-fullwin{background:#000;color:#fff;padding:1rem;display:flex;flex-direction:column;justify-content:center;overflow:auto}
#libPlayer.lp-fullwin{top:0;height:100vh;height:100dvh;z-index:1080;border-top:0;box-shadow:none;padding-bottom:max(1rem,env(safe-area-inset-bottom))}
body.lp-fullwin-on{overflow:hidden}
#libPlayer:fullscreen .lp-bar,#libPlayer.lp-fullwin .lp-bar{flex-wrap:wrap}
#libPlayer:fullscreen .lp-media,#libPlayer.lp-fullwin .lp-media{flex-basis:100%}
#libPlayer:fullscreen video{width:100%;height:auto;max-height:calc(100vh - 8rem)}
#libPlayer.lp-fullwin video{width:100%;height:auto;max-height:calc(100vh - 9rem);max-height:calc(100dvh - 9rem)}
#libPlayer:fullscreen .lp-panel,#libPlayer.lp-fullwin .lp-panel{display:none!important}
#libPlayer:fullscreen .btn-outline-secondary,#libPlayer.lp-fullwin .btn-outline-secondary{color:#fff;border-color:#adb5bd}
#libPlayer.lp-fullwin .text-muted{color:#ced4da!important}
#libPlayer.lp-overlay{padding:0}
#libPlayer.lp-overlay .lp-bar{position:absolute;inset:0;display:block}
#libPlayer.lp-overlay .lp-media{position:absolute;inset:0;display:flex;align-items:center;justify-content:center}
#libPlayer.lp-overlay .lp-media video{width:100%;height:100%;max-height:none;object-fit:contain}
#libPlayer.lp-overlay .lp-main{position:absolute;left:0;right:0;bottom:0;z-index:2;padding:3rem 1.25rem max(1rem,env(safe-area-inset-bottom));
 background:linear-gradient(to bottom,rgba(0,0,0,0),rgba(0,0,0,.55) 35%,rgba(0,0,0,.85));transition:opacity .35s}
#libPlayer.lp-overlay.lp-idle .lp-main{opacity:0;pointer-events:none}
#libPlayer.lp-overlay.lp-idle{cursor:none}
#libPlayer .lp-about-box{position:absolute;right:1rem;bottom:calc(100% + .5rem);z-index:5;width:min(30rem,calc(100vw - 2rem));max-height:65vh;overflow:auto;
 background:rgba(25,32,42,.96);color:#e9ecef;border-radius:.6rem;box-shadow:0 .5rem 2rem rgba(0,0,0,.45);padding:1rem}
#libPlayer.lp-fullwin .lp-about-box,#libPlayer:fullscreen .lp-about-box{top:1rem;bottom:auto;max-height:calc(100% - 9rem)}
#libPlayer .lp-about-box img{width:96px;border-radius:.4rem;float:left;margin:0 .9rem .5rem 0}
#libPlayer .lp-about-box .lp-about-ep{clear:both;border-top:1px solid rgba(255,255,255,.15);margin-top:.75rem;padding-top:.75rem}
#libPlayer .lp-about-box p{font-size:.875rem;white-space:pre-line;margin:.35rem 0 0}
.lp-vol,.lib-size{-webkit-appearance:none;appearance:none;height:4px;border-radius:2px;cursor:pointer;background:linear-gradient(to right,var(--bs-primary,#3b7ddd) var(--v,100%),rgba(128,128,128,.35) var(--v,100%))}
.lp-vol::-webkit-slider-runnable-track,.lib-size::-webkit-slider-runnable-track{height:4px;background:transparent}
.lp-vol::-webkit-slider-thumb,.lib-size::-webkit-slider-thumb{-webkit-appearance:none;width:14px;height:14px;margin-top:-5px;border:0;border-radius:50%;background:var(--bs-primary,#3b7ddd);box-shadow:0 0 0 2px var(--bs-body-bg,#fff)}
.lp-vol::-moz-range-track,.lib-size::-moz-range-track{height:4px;background:transparent}
.lp-vol::-moz-range-thumb,.lib-size::-moz-range-thumb{width:14px;height:14px;border:0;border-radius:50%;background:var(--bs-primary,#3b7ddd);box-shadow:0 0 0 2px var(--bs-body-bg,#fff)}
`

const playerJS = `
// ---- Player ----
// The player is a bar docked at the bottom of the page with its own
// controls. Direct files can be seeked by the browser; repackaged and
// converted streams restart at the wanted position (start=seconds) unless it
// is already buffered, so the player keeps its own position (lp.offset).
// Multi-part audiobooks get one timeline across their parts (lp.book), and
// the next entry is loaded ahead so it starts without a gap (lp.pre).
var lp = null;

// lpStore keeps the Library's and the player's settings (saved views, sizes,
// volume, languages) in the account (window.gmdSettings, saved to the
// server shortly after a change), so they follow it to every browser.
// Settings this browser saved before that are taken over on first use.
var lpSettingsTimer = null;

// libReport asks why a file is wrong and reports it to the admins.
function libReport(type, id, label) {
	var old = document.getElementById('libReportBox');
	if (old) { old.lpClose(); }
	// A dimmed backdrop under a bordered dialog, so it reads as a window
	// over the detail view; a click beside it or Esc cancels.
	var wrap = document.createElement('div');
	wrap.id = 'libReportBox';
	wrap.className = 'position-fixed top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center';
	wrap.style.zIndex = 2100; wrap.style.background = 'rgba(0,0,0,.55)';
	var box = document.createElement('div');
	// Dark on white also over a full-screen player (white text there).
	box.className = 'card shadow-lg bg-white text-dark border border-2 border-warning';
	box.setAttribute('role', 'dialog'); box.setAttribute('aria-modal', 'true'); box.setAttribute('aria-labelledby', 'libReportTitle');
	box.style.width = 'min(26rem, 92vw)';
	box.innerHTML = '<div class="card-header bg-warning-subtle d-flex align-items-center border-bottom border-warning">' +
		'<h6 class="mb-0 flex-grow-1 text-dark" id="libReportTitle"><i class="fas fa-flag me-2 text-warning"></i>' + _t('Report a wrong file') + '</h6>' +
		'<button type="button" class="btn-close lib-report-cancel" aria-label="' + _t('Close') + '"></button></div>' +
		'<div class="card-body"><div class="small text-muted mb-2 text-break lib-report-label"></div>' +
		'<label class="form-label small" for="libReportReason">' + _t('What is wrong?') + '</label>' +
		'<select class="form-select form-select-sm mb-2" id="libReportReason"><option value="quality">' + _t('Bad quality') + '</option>' +
		'<option value="language">' + _t('Wrong language') + '</option><option value="content">' + _t('Wrong content') + '</option><option value="other">' + _t('Something else') + '</option></select>' +
		'<label class="form-label small" for="libReportNote">' + _t('Note (optional)') + '</label>' +
		'<textarea class="form-control form-control-sm mb-3" id="libReportNote" rows="2" maxlength="500"></textarea>' +
		'<div class="text-end"><button type="button" class="btn btn-sm btn-secondary me-2 lib-report-cancel">' + _t('Cancel') + '</button>' +
		'<button type="button" class="btn btn-sm btn-warning lib-report-send">' + _t('Report') + '</button></div></div>';
	wrap.appendChild(box);
	var close = function () { wrap.remove(); document.removeEventListener('keydown', onKey, true); };
	var onKey = function (e) { if (e.key === 'Escape') { e.stopPropagation(); close(); } };
	wrap.lpClose = close;
	document.addEventListener('keydown', onKey, true);
	wrap.addEventListener('click', function (e) { if (e.target === wrap) { close(); } });
	box.querySelectorAll('.lib-report-cancel').forEach(function (b) { b.addEventListener('click', close); });
	box.querySelector('.lib-report-label').textContent = label;
	box.querySelector('.lib-report-send').addEventListener('click', function () {
		var form = new URLSearchParams({ type: type, id: String(id), reason: box.querySelector('#libReportReason').value,
			note: box.querySelector('#libReportNote').value, csrf_token: lpCsrf() });
		fetch('/api/admin/library/report', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': lpCsrf() } })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function () { close(); libNotify('success', _t('Reported - an admin will look at it.')); })
			.catch(function (e) { libNotify('error', _t('Not reported: ') + e.message); });
	});
	// In full screen only the full-screen element is shown.
	(document.fullscreenElement || document.body).appendChild(wrap);
	box.querySelector('#libReportReason').focus();
}

// lpCsrf is the page's CSRF token (the Library's, else the cookie's: the
// Calendar has the player without the Library script).
function lpCsrf() {
	if (typeof libCsrf !== 'undefined') { return libCsrf; }
	var m = /(?:^|; )csrf_token=([^;]+)/.exec(document.cookie);
	return m ? decodeURIComponent(m[1]) : '';
}
function lpStore(key, value) {
	var s = window.gmdSettings || (window.gmdSettings = {});
	if (value === undefined) {
		if (Object.prototype.hasOwnProperty.call(s, key)) { return s[key]; }
		var old = lpLocal(key);
		if (old !== null) { s[key] = old; lpSaveSettings(); }
		return old;
	}
	s[key] = String(value);
	lpSaveSettings();
}

// lpLocal reads what this browser stored per account before (the built-in
// admin's from before accounts existed: no prefix).
function lpLocal(key) {
	var user = window.gmdUser || 'admin';
	try {
		var v = localStorage.getItem('u:' + user + ':' + key);
		return v === null && user === 'admin' ? localStorage.getItem(key) : v;
	} catch (e) { return null; }
}

function lpSaveSettings() {
	clearTimeout(lpSettingsTimer);
	lpSettingsTimer = setTimeout(function () {
		fetch('/api/admin/account/settings', { method: 'POST', credentials: 'same-origin', keepalive: true,
			headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': lpCsrf() }, body: JSON.stringify(window.gmdSettings) }).catch(function () {});
	}, 1500);
}

// The measured connection is kept per browser (it is the device's network)
// in a cookie that expires after player_speedtest_minutes: a laptop that
// moves to another network measures again.
function lpNetCookie(kbps) {
	if (kbps === undefined) {
		var m = /(?:^|; )gmd_net=(\d+)/.exec(document.cookie);
		return m ? +m[1] : 0;
	}
	document.cookie = 'gmd_net=' + kbps + '; max-age=' + ((window.gmdNetMinutes || 30) * 60) + '; path=/; SameSite=Lax';
}

// lpNet is what a stream can get: the connection, at most the server's
// share of player_max_bandwidth.
function lpNet() {
	var server = (lp && lp.info && lp.info.serverKbps) || 0;
	if (!lp || !lp.net) { return server; }
	return server ? Math.min(lp.net, server) : lp.net;
}

// lpReport sends the position to the server (play history, resume, the
// admins' Now Playing); its answer can stop the player with a message.
function lpReport(state, pos) {
	if (!lp || !lp.info || !lp.sid) { return; }
	var it = lpItem(), m = lp.info.meta || {}, mine = lp, q = lpQ('.lp-quality');
	var body = new URLSearchParams({ sid: lp.sid, kind: it.kind, id: it.id, state: state,
		pos: String(Math.floor(pos === undefined ? lpPosition() : pos)), dur: String(Math.floor(lp.info.duration || 0)),
		title: [m.title || it.label, m.sub || lp.entry].filter(Boolean).join(' · '), quality: q ? q.value : '',
		mode: lp.hlsSid ? 'hls' : (q && q.value !== 'original') ? 'transcode' : (lp.info.mode || '') });
	fetch('/api/admin/library/play/progress', { method: 'POST', credentials: 'same-origin', body: body, keepalive: state === 'stop',
		headers: { 'X-CSRF-Token': lpCsrf() } })
		.then(function (r) { return r.ok ? r.json() : null; })
		.then(function (d) { if (d && d.stop && lp === mine) { lp.sid = ''; lpClose(); lpStoppedNote(d.message); } })
		.catch(function () {});
}

// lpStoppedNote shows an admin's message until it is closed.
function lpStoppedNote(msg) {
	var n = document.createElement('div');
	n.className = 'card shadow border-warning position-fixed end-0 m-3';
	n.style.zIndex = 2100; n.style.width = 'min(28rem, 92vw)'; n.style.bottom = '5rem'; n.setAttribute('role', 'alert');
	n.innerHTML = '<div class="card-body d-flex gap-3 align-items-start"><i class="fa-solid fa-circle-stop text-warning fs-4"></i>' +
		'<div class="flex-grow-1"><div class="fw-semibold mb-1">' + _t('Playback stopped by an admin') + '</div><div class="lp-stop-msg small"></div></div>' +
		'<button type="button" class="btn-close" aria-label="' + _t('Close') + '"></button></div>';
	n.querySelector('.lp-stop-msg').textContent = msg || '';
	n.querySelector('.btn-close').addEventListener('click', function () { n.remove(); });
	document.body.appendChild(n);
}

function lpFmt(t) {
	if (!isFinite(t) || t < 0) { t = 0; }
	var h = Math.floor(t / 3600), m = Math.floor(t % 3600 / 60), sec = Math.floor(t % 60);
	return (h ? h + ':' + String(m).padStart(2, '0') : m) + ':' + String(sec).padStart(2, '0');
}

// libPlayFirst plays a list where the account left it: the file played
// last (the next one when that was finished), else the first one not
// watched yet (series: the next episode), else from the start. Albums
// always start at the first track.
function libPlayFirst(list) {
	var btns = libPlayButtons(list);
	if (!btns.length) { return; }
	if (btns[0].dataset.playKind === 'track') { libPlay(btns[0]); return; }
	var start = -1, last = btns.findIndex(function (b) { return b.dataset.last === '1'; });
	if (last >= 0) { start = btns[last].dataset.watched === '1' ? last + 1 : last; }
	if (start < 0 || start >= btns.length) { start = btns.findIndex(function (b) { return b.dataset.watched !== '1'; }); }
	libPlay(btns[Math.max(start, 0)]);
}

// libContinue plays on from the Continue row: an audiobook with all its
// parts (one timeline), else the file itself.
function libContinue(kind, fileID, entryID, label, title, cover) {
	if (kind !== 'audiobook') { lpPlayList([{ kind: kind, id: String(fileID), label: label }], 0, title, cover); return; }
	fetch('/api/admin/library/playlist?type=audiobook&id=' + entryID, { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			var list = (d.list || []).map(function (x) { return { kind: x.kind, id: String(x.id), label: x.label }; });
			var idx = list.findIndex(function (x) { return x.id === String(fileID); });
			lpPlayList(list.length ? list : [{ kind: kind, id: String(fileID), label: label }], Math.max(idx, 0), d.entry || title, d.cover || cover);
		})
		.catch(function (e) { libNotify('error', _t('Can\'t play: ') + e.message); });
}

// libContinueDrop removes a card from the Continue or Next up row (and the
// row once it's empty).
function libContinueDrop(btn, row, type, id) {
	var form = new URLSearchParams({ row: row, type: type, id: String(id), csrf_token: libCsrf });
	fetch('/api/admin/library/continue-drop', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
		.then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } })
		.then(function () {
			var card = btn.closest('.lib-continue-card'), list = card && card.parentElement;
			if (card) { card.remove(); }
			if (list && !list.querySelector('.lib-continue-card')) { list.parentElement.remove(); }
		})
		.catch(function (e) { libNotify('error', e.message); });
}

// libWatchedButton is the HTML of a row's watched toggle.
function libWatchedButton(kind, id, on) {
	var book = kind === 'book';
	return on
		? '<button type="button" class="btn btn-sm btn-success py-0 me-1" title="' + (book ? _t('Read - click to mark as not read') : _t('Watched - click to mark as not watched')) + '" onclick="libWatched(\'' + kind + '\', ' + id + ', false, this)"><i class="fa-solid fa-check"></i></button>'
		: '<button type="button" class="btn btn-sm btn-outline-secondary py-0 me-1" title="' + (book ? _t('Mark as read') : _t('Mark as watched')) + '" onclick="libWatched(\'' + kind + '\', ' + id + ', true, this)"><i class="fa-regular fa-eye"></i></button>';
}

// libWatchedRow shows a list row as watched or not.
function libWatchedRow(row, kind, id, on) {
	var play = row.querySelector('[data-play-id]'), btn = row.querySelector('button[onclick^="libWatched("]');
	if (play) { if (on) { play.dataset.watched = '1'; } else { delete play.dataset.watched; } }
	if (row.dataset.ws) { row.dataset.ws = on ? 'watched' : 'unwatched'; }
	if (btn) { btn.outerHTML = libWatchedButton(kind, id, on); }
}

// libWatchedPost marks files watched or not.
function libWatchedPost(kind, ids, on) {
	var form = new URLSearchParams({ kind: kind, watched: on ? '1' : '0', csrf_token: libCsrf });
	ids.forEach(function (id) { form.append('id', String(id)); });
	return fetch('/api/admin/library/watched', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
		.then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
}

// libWatched marks a file watched or not watched.
function libWatched(kind, id, on, btn) {
	libWatchedPost(kind, [id], on)
		.then(function () {
			var row = btn.closest('tr');
			if (row) { libWatchedRow(row, kind, id, on); } else { btn.outerHTML = libWatchedButton(kind, id, on); }
		})
		.catch(function (e) { libNotify('error', e.message); });
}

// libMarkShown marks the rows a list shows (all, or those its filters
// leave) watched or not watched.
function libMarkShown(el, kind, on) {
	var pane = el.closest('.tab-pane');
	var rows = [].filter.call(pane.querySelectorAll('tbody tr[data-wid]'), function (r) { return !r.classList.contains('d-none'); });
	if (!rows.length) { return; }
	libWatchedPost(kind, rows.map(function (r) { return r.dataset.wid; }), on)
		.then(function () {
			rows.forEach(function (r) { libWatchedRow(r, kind, r.dataset.wid, on); });
			libFilterList(el);
		})
		.catch(function (e) { libNotify('error', e.message); });
}

// libPlayGrid plays an episode of the episode view, then the ones after it
// on the page.
function libPlayGrid(btn) {
	var btns = [].slice.call(document.querySelectorAll('#libGrid [data-play-list="grid"]'))
		.sort(function (a, b) { return a.dataset.playOrder - b.dataset.playOrder; });
	var list = btns.map(function (b) { return { kind: b.dataset.playKind, id: b.dataset.playId, label: b.dataset.playLabel }; });
	lpPlayList(list, btns.indexOf(btn), '', '');
}

function libPlayButtons(list) {
	// Rows the list's search box hides don't play.
	return [].slice.call(document.querySelectorAll('#libModalBody [data-play-list="' + list + '"]'))
		.filter(function (b) { return !b.closest('tr.d-none'); })
		.sort(function (a, b) { return a.dataset.playOrder - b.dataset.playOrder; });
}

function lpBtn(cls, icon, title) {
	return '<button type="button" class="btn btn-sm btn-outline-secondary ' + cls + '" title="' + title + '" aria-label="' + title + '"><i class="fa-solid ' + icon + '"></i></button>';
}

function lpQ(sel) { return document.querySelector('#libPlayer ' + sel); }

function libPlay(btn) {
	var list = libPlayButtons(btn.dataset.playList).map(function (b) { return { kind: b.dataset.playKind, id: b.dataset.playId, label: b.dataset.playLabel }; });
	var idx = list.findIndex(function (x) { return x.id === btn.dataset.playId; });
	var cover = document.querySelector('#libModalBody .lib-detail-cover');
	lpPlayList(list, idx, document.getElementById('libModalTitle').textContent, cover ? cover.getAttribute('src') : '');
}

// lpPlayList plays a list of files ({kind, id, label}) from index idx in the
// bottom bar; entry and cover describe what the list belongs to.
function lpPlayList(list, idx, entry, cover) {
	lpStop();
	// Shuffle and repeat are for music; speed and sleep timer for audiobooks.
	var music = list.length > 0 && list[0].kind === 'track';
	var book = list.length > 0 && list[0].kind === 'audiobook';
	lp = { list: list, idx: idx < 0 ? 0 : idx, offset: 0, music: music, isBook: book, hist: [], played: {},
		shuffle: music && lpStore('lp-shuffle') === '1', repeat: music ? (lpStore('lp-repeat') || 'off') : 'off' };
	// The entry the playlist belongs to, for the bar and the media keys.
	lp.entry = entry || '';
	lp.cover = cover || '';
	lp.sid = (Date.now().toString(36) + Math.random().toString(36).slice(2, 10)).replace(/[^a-z0-9]/g, '').slice(0, 24);
	var box = document.getElementById('libPlayer');
	box.classList.remove('d-none', 'lp-expanded');
	var rates = [0.75, 1, 1.25, 1.5, 1.75, 2];
	box.innerHTML =
		'<div class="lp-panel">' +
		'<div class="lp-chapters d-flex flex-wrap gap-1 mb-2 d-none"></div>' +
		'<div class="lp-list list-group mb-2 d-none" style="max-height:30vh;overflow-y:auto;position:relative"></div></div>' +
		'<div class="lp-bar">' +
		'<div class="lp-media"></div>' +
		'<div class="lp-main">' +
		'<div class="d-flex align-items-center gap-2 lp-head">' +
		'<div class="text-truncate"><span class="lp-title fw-semibold"></span> <span class="small text-muted lp-entry"></span></div>' +
		'<span class="small text-muted text-truncate lp-info"></span><span class="small text-muted text-nowrap lp-buf"></span></div>' +
		'<div class="lp-seek d-flex align-items-center gap-2">' +
		'<button type="button" class="btn btn-link btn-sm p-0 text-reset text-decoration-none small lp-pos" title="' + _t('Jump to a time: click and type it, e.g. 1:23:45') + '">0:00</button>' +
		'<input type="text" class="form-control form-control-sm d-none lp-goto" inputmode="numeric" placeholder="' + _t('h:mm:ss') + '" aria-label="' + _t('Jump to time') + '" style="width:6rem">' +
		'<div class="flex-grow-1 lp-tl" role="slider" tabindex="0" aria-label="' + _t('Position') + '" aria-valuemin="0" title="' + _t('Click or drag to jump') + '">' +
		'<div class="lp-tl-track"><div class="lp-tl-buf"></div><div class="lp-tl-fill"></div></div>' +
		'<div class="lp-tl-hover d-none"></div><div class="lp-tl-knob"></div><div class="lp-tl-tip d-none"></div></div>' +
		'<span class="small lp-dur">0:00</span></div>' +
		'<div class="d-flex flex-wrap align-items-center gap-1 lp-controls">' +
		lpBtn('lp-prev', 'fa-backward-step', _t('Previous (P)')) + lpBtn('lp-back', 'fa-rotate-left', _t('Back 10 seconds (←)')) +
		'<button type="button" class="btn btn-sm btn-primary lp-play" title="' + _t('Play or pause (Space)') + '" aria-label="' + _t('Play or pause') + '"><i class="fa-solid fa-play"></i></button>' +
		lpBtn('lp-fwd', 'fa-rotate-right', _t('Forward 10 seconds (→)')) + lpBtn('lp-next', 'fa-forward-step', _t('Next (N)')) +
		(music ? lpBtn('lp-shuffle', 'fa-shuffle', _t('Shuffle')) + lpBtn('lp-repeat', 'fa-repeat', _t('Repeat')) : '') +
		(book ? '<select class="form-select form-select-sm w-auto lp-rate" title="' + _t('Speed') + '" aria-label="' + _t('Speed') + '">' +
			rates.map(function (r) { return '<option value="' + r + '">' + r + '×</option>'; }).join('') + '</select>' +
			'<select class="form-select form-select-sm w-auto lp-sleep" title="' + _t('Sleep timer') + '" aria-label="' + _t('Sleep timer') + '">' +
			'<option value="">' + _t('Sleep: off') + '</option><option value="15">' + _t('15 min') + '</option><option value="30">' + _t('30 min') + '</option>' +
			'<option value="45">' + _t('45 min') + '</option><option value="60">' + _t('60 min') + '</option>' +
			'<option value="chapter">' + _t('End of chapter') + '</option><option value="file">' + _t('End of part') + '</option></select>' : '') +
		'<span class="flex-grow-1"></span>' +
		lpBtn('lp-mute', 'fa-volume-high', _t('Mute (M)')) +
		'<input type="range" class="lp-vol" style="width:5rem" min="0" max="1" step="0.05" aria-label="' + _t('Volume') + '">' +
		'<select class="form-select form-select-sm w-auto lp-audio d-none" style="max-width:14rem" title="' + _t('Audio track') + '" aria-label="' + _t('Audio track') + '"></select>' +
		'<select class="form-select form-select-sm w-auto lp-subs d-none" style="max-width:14rem" title="' + _t('Subtitles') + '" aria-label="' + _t('Subtitles') + '"></select>' +
		'<select class="form-select form-select-sm w-auto lp-quality" style="max-width:22rem" aria-label="' + _t('Quality') + '"></select>' +
		lpBtn('lp-about', 'fa-circle-info', _t('Details (I)')) +
		(window.gmdCanReport ? lpBtn('lp-report', 'fa-flag', _t('Report this file as wrong (quality, language, content)')) : '') +
		lpBtn('lp-show-chapters d-none', 'fa-bookmark', _t('Show the chapters')) +
		lpBtn('lp-show-list d-none', 'fa-list', _t('Show the playlist')) +
		lpBtn('lp-expand', 'fa-up-right-and-down-left-from-center', _t('Bigger or smaller')) +
		lpBtn('lp-full', 'fa-expand', _t('Full screen (F)')) +
		'<button type="button" class="btn btn-sm btn-outline-danger lp-close" title="' + _t('Close the player') + '" aria-label="' + _t('Close the player') + '"><i class="fa-solid fa-xmark"></i></button>' +
		'</div></div></div>';
	lpQ('.lp-entry').textContent = lp.entry && lp.entry !== list[lp.idx].label ? '· ' + lp.entry : '';
	lpQ('.lp-about').addEventListener('click', function () { lpAbout(); });
	var rep = lpQ('.lp-report');
	if (rep) {
		rep.addEventListener('click', function () {
			var it = lpItem(), m = (lp.info && lp.info.meta) || {};
			libReport({ movie: 'movie', episode: 'series', track: 'album', audiobook: 'audiobook' }[it.kind], it.id,
				[m.title || it.label, m.sub].filter(Boolean).join(' · '));
		});
	}
	lpQ('.lp-expand').addEventListener('click', function () {
		lp.sized = true;
		var cur = lpSize();
		lpSetSize(cur === 'mini' ? 'expanded' : cur === 'expanded' && !lp.audio ? 'fullwin' : 'mini');
	});
	// Chapters and playlist are hidden until asked for; showing one makes
	// the player big.
	[['.lp-show-chapters', '.lp-chapters'], ['.lp-show-list', '.lp-list']].forEach(function (p) {
		lpQ(p[0]).addEventListener('click', function () {
			var part = lpQ(p[1]), show = part.classList.contains('d-none');
			part.classList.toggle('d-none', !show);
			this.classList.toggle('active', show);
			if (show && lpSize() !== 'expanded') { lp.sized = true; lpSetSize('expanded'); }
			if (show && p[1] === '.lp-list') {
				var cur = part.querySelector('.active');
				if (cur) { part.scrollTop = cur.offsetTop - part.clientHeight / 2; }
			}
			lpDockSpace();
		});
	});
	lpQ('.lp-quality').addEventListener('change', function () {
		lpStore(lp.audio ? 'lp-q-audio' : 'lp-q-video', this.value);
		lpDiscardPre();
		lpStart(lpPosition());
	});
	lpQ('.lp-audio').addEventListener('change', function () {
		var t = (lp.info.audioTracks || [])[+this.value];
		lpStore('lp-audio-lang', t && t.lang ? t.lang : '');
		lp.sel.audio = +this.value;
		lpDiscardPre();
		lpStart(lpPosition());
	});
	lpQ('.lp-subs').addEventListener('change', function () { lpSetSub(this.value, true); });
	lpTimeline();
	lpQ('.lp-play').addEventListener('click', lpToggle);
	lpQ('.lp-mute').addEventListener('click', function () { if (lp.el) { lp.el.muted = !lp.el.muted; } });
	lpQ('.lp-vol').addEventListener('input', function () { lpVolShow(+this.value); if (lp.el) { lp.el.muted = false; lp.el.volume = +this.value; } });
	lpQ('.lp-full').addEventListener('click', lpFullscreen);
	lpQ('.lp-prev').addEventListener('click', function () { lpStep(-1, false); });
	lpQ('.lp-back').addEventListener('click', function () { lpJump(-10); });
	lpQ('.lp-fwd').addEventListener('click', function () { lpJump(10); });
	lpQ('.lp-next').addEventListener('click', function () { lpStep(1, false); });
	lpQ('.lp-close').addEventListener('click', lpClose);
	if (music) {
		lpQ('.lp-shuffle').addEventListener('click', function () {
			lp.shuffle = !lp.shuffle; lp.played = {}; lp.played[lp.idx] = true;
			lpStore('lp-shuffle', lp.shuffle ? '1' : '0'); lpDiscardPre(); lpShowModes();
		});
		lpQ('.lp-repeat').addEventListener('click', function () {
			lp.repeat = { off: 'all', all: 'one', one: 'off' }[lp.repeat] || 'off';
			lpStore('lp-repeat', lp.repeat); lpDiscardPre(); lpShowModes();
		});
		lpShowModes();
	}
	if (book) {
		var rate = lpQ('.lp-rate');
		rate.value = String(parseFloat(lpStore('lp-rate')) || 1);
		if (!rate.value) { rate.value = '1'; }
		rate.addEventListener('change', function () { lpStore('lp-rate', this.value); lpApplyRate(lp.el); });
		lpQ('.lp-sleep').addEventListener('change', function () { lpSetSleep(this.value); });
	}
	if (list.length > 1) {
		lpQ('.lp-list').innerHTML = list.map(function (x, i) {
			return '<button type="button" class="list-group-item list-group-item-action py-1 small" data-i="' + i + '"></button>';
		}).join('');
		box.querySelectorAll('.lp-list [data-i]').forEach(function (b) {
			b.textContent = list[+b.dataset.i].label;
			b.addEventListener('click', function () { lpLoad(+b.dataset.i); });
		});
		lpQ('.lp-show-list').classList.remove('d-none');
	}
	lpDockSpace();
	lpSessionHandlers();
	// iOS starts media only from a tap, or on elements a tap started once:
	// both element types are made and started here, still in the tap. Only
	// there: other browsers mark an element started without a source as
	// playing.
	lp.ready = lpUseHLS() ? { audio: lpUnlock(lpMakeEl(true)), video: lpUnlock(lpMakeEl(false)) } : {};
	// The connection is measured before the stream starts, so the two don't
	// share the line (a result is kept for 30 minutes). A multi-part
	// audiobook's timeline loads meanwhile.
	var mine = lp;
	var bookInfo = book && list.length > 1
		? fetch('/api/admin/library/play/book?ids=' + list.map(function (x) { return x.id; }).join(','), { credentials: 'same-origin' })
			.then(function (r) { return r.ok ? r.json() : null; }).catch(function () { return null; })
		: Promise.resolve(null);
	Promise.all([lpMeasure(false), bookInfo]).then(function (res) {
		if (lp !== mine) { return; }
		if (res[1] && res[1].duration > 0) { lp.book = res[1]; }
		lpLoad(lp.idx);
	});
}

// ---- Connection speed and quality marks ----
function lpMeasure(force) {
	var cached = lpNetCookie();
	if (!force && cached) { lp.net = cached; return Promise.resolve(); }
	var info = lpQ('.lp-info');
	if (info) { info.textContent = _t('Measuring the connection…'); }
	var ctrl = new AbortController(), bytes = 0, t0 = 0, t1 = 0;
	var stop = setTimeout(function () { ctrl.abort(); }, 2500);
	return fetch('/api/admin/library/play/speedtest?kb=16384', { credentials: 'same-origin', cache: 'no-store', signal: ctrl.signal })
		.then(function (r) {
			var reader = r.body.getReader();
			function pump() {
				return reader.read().then(function (res) {
					if (res.done) { return; }
					// Timing starts with the first data, so the request's round trip doesn't count.
					if (!t0) { t0 = performance.now(); } else { bytes += res.value.length; }
					t1 = performance.now();
					return pump();
				});
			}
			return pump();
		})
		.catch(function () {})
		.then(function () {
			clearTimeout(stop);
			var secs = (t1 - t0) / 1000;
			if (bytes > 65536 && secs > 0.005) {
				var kbps = Math.round(bytes * 8 / 1000 / secs);
				lpNetCookie(kbps);
				if (lp) { lp.net = kbps; }
			}
			if (lp) { lpMarkQualities(); lpShowInfo(); }
		});
}

function lpFmtKbps(k) { return k >= 1000 ? (k / 1000).toFixed(1) + _t(' Mbit/s') : k + ' kbit/s'; }

// lpMarkQualities marks each quality: ★ the best one that should play
// without stalls, ✓ the others that fit, ⚠ the ones that will stall because
// the connection is too slow or the server converts slower than real time.
// A stream needs some headroom over its average bitrate.
function lpMarkQualities() {
	if (!lp || !lp.info) { return; }
	var sel = lpQ('.lp-quality');
	if (!sel) { return; }
	var best = -1, bestKbps = -1, notes = [];
	lp.info.qualities.forEach(function (q, i) {
		var note = '';
		var net = lpNet();
		var netOK = net ? (q.kbps || 0) <= net * 0.75 : null;
		var convOK = q.converted && q.speed ? q.speed >= 1.1 : null;
		if (netOK === false) { note = _t(' ⚠ needs a faster connection'); }
		else if (convOK === false) { note = _tf(' ⚠ server converts at %s×', q.speed.toFixed(1)); }
		else if (netOK) { note = ' ✓'; if ((q.kbps || 0) > bestKbps) { bestKbps = q.kbps || 0; best = i; } }
		notes.push(note);
	});
	if (best >= 0) { notes[best] = _t(' ★ best without stalls'); }
	[].forEach.call(sel.options, function (o, i) { o.textContent = lp.info.qualities[i].label + notes[i]; });
}

function lpShowModes() {
	var sh = lpQ('.lp-shuffle'), rp = lpQ('.lp-repeat');
	if (!sh || !rp) { return; }
	sh.classList.toggle('active', lp.shuffle);
	sh.setAttribute('aria-pressed', lp.shuffle ? 'true' : 'false');
	var label = { off: _t('Repeat: off'), all: _t('Repeat: all'), one: _t('Repeat: this track') }[lp.repeat];
	rp.classList.toggle('active', lp.repeat !== 'off');
	rp.title = label; rp.setAttribute('aria-label', label);
	rp.innerHTML = '<i class="fa-solid fa-repeat"></i>' + (lp.repeat === 'one' ? '<sup class="ms-1">1</sup>' : '');
}

function lpItem() { return lp.list[lp.idx]; }
function lpBaseOf(it) { return '/api/admin/library/play/' + it.kind + '/' + it.id; }
function lpBase() { return lpBaseOf(lpItem()); }
function lpPosKey() { var it = lpItem(); return 'lp-pos:' + it.kind + ':' + it.id; }
// lpPosition is the position in the current file.
function lpPosition() { return lp.el ? lp.offset + (lp.el.currentTime || 0) : 0; }
// On a book's timeline, the current part starts at lpPartStart.
function lpPartStart() { return lp.book && lp.book.parts[lp.idx] ? lp.book.parts[lp.idx].start : 0; }
function lpAbs() { return lpPartStart() + lpPosition(); }
function lpTotal() { return lp.book ? lp.book.duration : (lp.info && lp.info.duration) || 0; }
function lpChapters() { return lp.book ? lp.book.chapters : (lp.info && lp.info.chapters) || []; }

// lpUnlock starts an element without a source during a tap (it fails, but
// iOS allows the element to play from then on).
function lpUnlock(el) {
	try { var p = el.play(); if (p && p.catch) { p.catch(function () {}); } } catch (e) { /* nothing to play yet */ }
	return el;
}

// lpTake returns the element prepared in the tap for audio or video, or a new one.
function lpTake(audio) {
	var key = audio ? 'audio' : 'video';
	if (lp.ready && lp.ready[key]) { var el = lp.ready[key]; lp.ready[key] = null; return el; }
	return lpMakeEl(audio);
}

// lpMakeEl creates a media element; its events only count while it is the
// playing one (a preloaded next element stays quiet).
function lpMakeEl(audio) {
	var el = document.createElement(audio ? 'audio' : 'video');
	if (!audio) { el.className = 'rounded bg-black'; el.setAttribute('playsinline', ''); }
	el.preload = 'auto';
	var vol = parseFloat(lpStore('lp-vol'));
	if (vol >= 0 && vol <= 1) { el.volume = vol; }
	function mine(fn) { return function (e) { if (lp && lp.el === el) { fn(e); } }; }
	el.addEventListener('timeupdate', mine(lpTick));
	el.addEventListener('ended', mine(lpEnded));
	el.addEventListener('play', mine(function () { lp.userPaused = false; lp.buffering = null; lpShowInfo(); lpShowState(); }));
	el.addEventListener('pause', mine(function () { if (!lp.buffering) { lp.userPaused = true; lp.pausedAt = Date.now(); lpReport('pause'); } lpShowState(); }));
	el.addEventListener('waiting', mine(function () { if (!lp.userPaused && !lp.buffering && !el.ended) { lpBuffer(true); } }));
	el.addEventListener('progress', mine(lpShowBuffered));
	el.addEventListener('volumechange', mine(lpShowState));
	el.addEventListener('loadedmetadata', mine(function () { lpApplyRate(el); lpShowInfo(); }));
	el.addEventListener('click', mine(lpToggle));
	// A broken stream (e.g. an HLS session removed during a long pause)
	// restarts at the position, twice at most per file.
	el.addEventListener('error', mine(function () {
		if (!el.getAttribute('src') || lpDirect() || (lp.retries || 0) >= 2) { return; }
		lp.retries = (lp.retries || 0) + 1;
		lpStart(lpPosition());
	}));
	return el;
}

function lpApplyRate(el) {
	if (!el || !lp || !lp.isBook) { return; }
	var r = parseFloat(lpStore('lp-rate')) || 1;
	el.defaultPlaybackRate = r;
	el.playbackRate = r;
}

// lpPlaceEl puts el into the bar (audio next to the cover).
function lpPlaceEl(el) {
	var media = lpQ('.lp-media');
	var old = lp.el;
	if (old && old !== el) { lpUnfeed(old); old.pause(); old.removeAttribute('src'); old.load(); old.remove(); }
	media.appendChild(el);
	lp.el = el;
}

// lpCoverSrc is the playing file's cover (info.meta.cover), else the one of
// the entry the playlist was started from.
function lpCoverSrc() {
	return (lp.info && lp.info.meta && lp.info.meta.cover) || lp.cover || '';
}

// lpShowCover shows the cover next to audio (tracks of a mixed playlist
// each get their own album's).
function lpShowCover() {
	var media = lpQ('.lp-media'), img = media.querySelector('img'), src = lpCoverSrc();
	if (!lp.el || lp.el.tagName !== 'AUDIO' || !src) { if (img) { img.remove(); } return; }
	if (!img) {
		img = document.createElement('img'); img.className = 'lp-cover'; img.alt = '';
		media.insertBefore(img, media.firstChild);
	}
	if (img.getAttribute('src') !== src) { img.src = src; }
}

// lpLoad plays entry i, at startAt seconds into it when given (else where
// it was left). A preloaded entry starts right away.
function lpLoad(i, startAt) {
	lp.idx = i;
	lp.retries = 0;
	lp.played[i] = true;
	if (lp.hist[lp.hist.length - 1] !== i) { lp.hist.push(i); }
	var box = document.getElementById('libPlayer');
	lpQ('.lp-title').textContent = lpItem().label;
	var listBox = box.querySelector('.lp-list');
	box.querySelectorAll('.lp-list [data-i]').forEach(function (b) {
		b.classList.toggle('active', +b.dataset.i === i);
		// Scroll the playlist only, not the page.
		if (+b.dataset.i === i && (b.offsetTop < listBox.scrollTop || b.offsetTop + b.offsetHeight > listBox.scrollTop + listBox.clientHeight)) {
			listBox.scrollTop = b.offsetTop - listBox.clientHeight / 2;
		}
	});
	var pre = lp.pre;
	lp.pre = null;
	if (pre && pre.idx === i && pre.info && pre.el && startAt === undefined) {
		lpApply(pre.info, pre.el, 0);
		return;
	}
	if (pre) { lpDiscard(pre); }
	var mine = lp;
	fetch(lpBase() + '/info', { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (info) { if (lp === mine && lp.idx === i) { lpApply(info, null, startAt); } })
		.catch(function (e) { libNotify('error', _t('Can\'t play: ') + e.message); });
}

// lpShowMeta shows what is playing: the track, part, movie or episode, and
// next to it artist and album, author and book, year and genres or series
// and episode (info.meta); before that, the playlist's label.
function lpShowMeta() {
	var m = (lp.info && lp.info.meta) || {}, label = lpItem().label;
	lpQ('.lp-title').textContent = m.title || label;
	lpQ('.lp-title').title = label;
	var sub = m.sub || (lp.entry && lp.entry !== label ? lp.entry : '');
	lpQ('.lp-entry').textContent = sub ? '· ' + sub : '';
}

function lpApply(info, preEl, startAt) {
	lp.info = info;
	lpShowMeta();
	lp.audio = info.audio;
	lp.sel = lpPickTracks(info);
	lp.cues = null;
	lp.subNote = '';
	// The next file was loaded ahead as it is; another track restarts it.
	if (preEl && lpSelParams()) { lpDiscard({ el: preEl }); preEl = null; startAt = 0; }
	if (preEl) {
		lpPlaceEl(preEl);
	} else if (!lp.el || (lp.el.tagName === 'AUDIO') !== info.audio) {
		lpPlaceEl(lpTake(info.audio));
	}
	lpShowCover();
	lpOverlaySync();
	lpAbout(true);
	var sel = lpQ('.lp-quality');
	var wanted = lpWantedQuality(info);
	sel.innerHTML = '';
	info.qualities.forEach(function (q) {
		var o = document.createElement('option'); o.value = q.value; o.textContent = q.label;
		if (q.value === wanted) { o.selected = true; }
		sel.appendChild(o);
	});
	lpMarkQualities();
	lpShowTracks();
	if (lpSubById(lp.sel.sub)) { lpSetSub(lp.sel.sub, false); } else { lpApplySubs(); }
	lpShowChapters();
	lpQ('.lp-dur').textContent = lpFmt(lpTotal());
	lpTlShow(lpAbs());
	lpQ('.lp-full').classList.toggle('d-none', info.audio);
	// Video opens in the bigger player, unless the size was chosen already.
	if (!info.audio && !lp.sized) {
		lp.sized = true;
		lpSetSize('expanded');
	}
	lpSizeButton();
	lpSessionMeta();
	if (preEl) {
		// Preloaded from the start of the file with the wanted quality.
		lp.offset = 0;
		lp.reported = Date.now();
		lpReport('start', 0);
		lpApplyRate(lp.el);
		lpShowInfo();
		lp.userPaused = false;
		lpBuffer();
		return;
	}
	var t = startAt;
	if (t === undefined) {
		// Movies, episodes and audiobooks resume where they stopped; tracks start over.
		var resume = lp.music ? 0 : +(info.resume || lpLocal(lpPosKey()) || 0);
		t = resume > 10 && resume < (info.duration || 0) - 30 ? resume : 0;
	}
	lp.reported = Date.now();
	lpReport('start', t);
	lpStart(t);
}

function lpWantedQuality(info) {
	var wanted = lpStore(info.audio ? 'lp-q-audio' : 'lp-q-video');
	return info.qualities.some(function (q) { return q.value === wanted; }) ? wanted : 'original';
}

function lpShowChapters() {
	var ch = lpQ('.lp-chapters');
	ch.innerHTML = '';
	lpChapters().forEach(function (c) {
		var b = document.createElement('button');
		b.type = 'button'; b.className = 'btn btn-sm btn-outline-secondary py-0';
		b.textContent = c.title || lpFmt(c.start);
		b.title = lpFmt(c.start);
		b.dataset.start = c.start;
		b.addEventListener('click', function () { lpSeekAbs(c.start); });
		ch.appendChild(b);
	});
	lpQ('.lp-show-chapters').classList.toggle('d-none', !ch.childElementCount);
}

function lpDirect() { return lp.info.mode === 'direct' && lpQ('.lp-quality').value === 'original' && !lpSelParams(); }

// ---- Audio tracks and subtitles ----
// lp.sel is the audio track (index) and subtitle (id) of the current file.
// Another audio track or a burned-in picture subtitle makes the server
// repackage or convert the file (lpSelParams). Text subtitles come as cues
// (seconds in the file) and go into the video's text track, shifted by
// lp.offset. The languages last chosen are preferred for the next file.
function lpFind(arr, fn) { for (var i = 0; i < arr.length; i++) { if (fn(arr[i])) { return arr[i]; } } return null; }

function lpPickTracks(info) {
	var sel = { audio: 0, sub: '' };
	var at = info.audioTracks || [], subs = info.subtitles || [];
	var al = lpStore('lp-audio-lang') || '';
	var a = (al && lpFind(at, function (t) { return t.lang === al; })) || lpFind(at, function (t) { return t.default; });
	if (a) { sel.audio = a.index; }
	var audioLang = at[sel.audio] ? at[sel.audio].lang : '';
	// Only text subtitles are chosen by themselves; burning in converts.
	var sl = lpStore('lp-sub-lang') || '';
	var s = sl ? lpFind(subs, function (x) { return x.lang === sl && !x.image && !x.forced; }) || lpFind(subs, function (x) { return x.lang === sl && !x.image; }) : null;
	if (!s && audioLang) { s = lpFind(subs, function (x) { return x.forced && !x.image && x.lang === audioLang; }); }
	if (s) { sel.sub = s.id; }
	return sel;
}

function lpSubById(id) { return id && lp.info ? lpFind(lp.info.subtitles || [], function (x) { return x.id === id; }) : null; }

function lpSelParams() {
	if (!lp || !lp.sel || lp.audio) { return ''; }
	var p = lp.sel.audio ? '&a=' + lp.sel.audio : '';
	var s = lpSubById(lp.sel.sub);
	if (s && s.image) { p += '&burn=' + s.id.slice(1); }
	return p;
}

function lpShowTracks() {
	var at = (!lp.audio && lp.info.audioTracks) || [], subs = (!lp.audio && lp.info.subtitles) || [];
	var a = lpQ('.lp-audio'), su = lpQ('.lp-subs');
	a.innerHTML = '';
	at.forEach(function (t) {
		var o = document.createElement('option'); o.value = t.index; o.textContent = t.label;
		if (t.index === lp.sel.audio) { o.selected = true; }
		a.appendChild(o);
	});
	a.classList.toggle('d-none', at.length < 2);
	su.innerHTML = '<option value="">' + _t('Subtitles: off') + '</option>';
	subs.forEach(function (x) {
		var o = document.createElement('option'); o.value = x.id; o.textContent = x.label;
		if (x.id === lp.sel.sub) { o.selected = true; }
		su.appendChild(o);
	});
	su.classList.toggle('d-none', !subs.length);
}

// lpSetSub switches the subtitle; user: chosen in the menu (remembered, and
// a burned-in one restarts the stream).
function lpSetSub(id, user) {
	var old = lpSubById(lp.sel.sub), s = lpSubById(id);
	lp.sel.sub = id;
	lp.cues = null;
	if (user) { lpStore('lp-sub-lang', s ? (s.lang || '') : ''); }
	if (user && ((old && old.image) || (s && s.image))) { lpDiscardPre(); lpStart(lpPosition()); }
	if (s && !s.image) { lpLoadCues(s); } else { lpApplySubs(); }
}

function lpLoadCues(s) {
	var mine = lp, idx = lp.idx;
	lp.subNote = _t('Loading subtitles…');
	lpShowInfo();
	fetch(lpBase() + '/sub/' + encodeURIComponent(s.id), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			if (lp !== mine || lp.idx !== idx || lp.sel.sub !== s.id) { return; }
			lp.subNote = '';
			lp.cues = d.cues || [];
			lpApplySubs();
			lpShowInfo();
		})
		.catch(function (e) {
			if (lp !== mine) { return; }
			lp.subNote = '';
			lpShowInfo();
			libNotify('error', _t('Can\'t load the subtitles: ') + e.message);
		});
}

// lpApplySubs puts the cues into the element's text track, in the times of
// the stream (which starts at lp.offset of the file).
function lpApplySubs() {
	var el = lp && lp.el;
	if (!el || el.tagName !== 'VIDEO') { return; }
	var cues = lp.cues || [];
	var tr = el.lpTrack;
	if (!tr) {
		if (!cues.length) { return; }
		tr = el.lpTrack = el.addTextTrack('subtitles', _t('Subtitles'));
	}
	tr.mode = 'hidden';
	while (tr.cues && tr.cues.length) { tr.removeCue(tr.cues[0]); }
	if (!cues.length) { tr.mode = 'disabled'; return; }
	var off = lp.offset || 0, Cue = window.VTTCue || window.TextTrackCue;
	cues.forEach(function (c) {
		if (c.e - off <= 0) { return; }
		try { tr.addCue(new Cue(Math.max(0, c.s - off), c.e - off, c.t)); } catch (e) { /* a cue the browser refuses */ }
	});
	tr.mode = 'showing';
}

// A browser without Media Source Extensions (the iPhone: every browser there
// is Apple's WebKit) only plays video it can fetch in byte ranges; converted
// video goes there as HLS, which it plays natively. Every other browser takes
// the converted stream through MSE (lpFeed). Decided by what the browser can
// do, not what it calls itself: recent Chrome also plays HLS natively but
// treats the growing playlist as live, fetching one segment in four, and a
// Chrome with a Safari user agent (DevTools override) would land there.
var lpHLSSupport = null;
function lpUseHLS() {
	if (lpHLSSupport === null) {
		var v = document.createElement('video');
		var mse = !!(window.MediaSource && MediaSource.isTypeSupported('video/mp4; codecs="avc1.640028,mp4a.40.2"'));
		lpHLSSupport = !mse && !!(v.canPlayType && v.canPlayType('application/vnd.apple.mpegurl'));
	}
	return lpHLSSupport;
}

// ---- Converted video through Media Source Extensions ----
// A plain <video src> reads an endless converted stream only a few seconds
// ahead, so a jump of 10 s already restarts the conversion. lpFeed fetches
// the stream itself and appends it to a SourceBuffer: everything received is
// buffered (up to lpFeedAhead seconds; then the server and its conversion
// wait), and jumps within it are instant. 60 s behind the position are kept
// for jumps back. On any MSE error the player falls back to the plain stream.
var lpFeedAhead = 120;

// lpPid names this page's player to the server: a new stream (a jump past
// what is buffered) replaces the player's previous one there at once, so
// quick jumps don't fill player_max_transcodes with streams already left.
var lpPid = Math.random().toString(36).slice(2, 10);

function lpMSEType() {
	var audio = !lp.info.audioTracks || lp.info.audioTracks.length > 0;
	var type = 'video/mp4; codecs="avc1.640028' + (audio ? ',mp4a.40.2' : '') + '"';
	return window.MediaSource && MediaSource.isTypeSupported(type) ? type : '';
}

function lpUnfeed(el) { if (el && el.lpFeed) { el.lpFeed.abort(); el.lpFeed = null; } }

// t is the wanted position: the stream may begin a little earlier (at a
// keyframe, X-Stream-Start), the player starts at t anyway. A stream that
// breaks off (the connection dropped, e.g. a proxy closing it during a long
// pause) marks the feed broken; the player restarts it where it is
// (lpRestartBroken) instead of treating that as the end.
function lpFeed(el, url, t) {
	var type = lpMSEType();
	if (!type || lp.noMSE) { return false; }
	var ms = new MediaSource(), ctrl = new AbortController(), blobURL = URL.createObjectURL(ms);
	var queue = [], done = false, reading = false, reader = null, sb = null, alive = true, first = true;
	var feed = el.lpFeed = { broken: false, abort: function () { alive = false; ctrl.abort(); URL.revokeObjectURL(blobURL); } };
	function broke(e) {
		if (!alive) { return; }
		feed.broken = true;
		// What is buffered still plays; lpTick restarts when it runs out.
		if (window.console) { console.warn('player: stream broke off', e); }
	}
	el.src = blobURL;
	function failed(e) {
		if (!alive) { return; }
		lpUnfeed(el);
		if (window.console) { console.warn('player: MSE failed, plain stream instead', e); }
		if (lp && lp.el === el) { lp.noMSE = true; lpStart(lpPosition()); }
	}
	function ahead() { var b = sb.buffered; return b.length ? b.end(b.length - 1) - el.currentTime : 0; }
	function read() {
		if (!alive || reading || done) { return; }
		if (ahead() > lpFeedAhead || queue.length > 16) { setTimeout(read, 1000); return; }
		reading = true;
		reader.read().then(function (res) {
			reading = false;
			if (!alive) { return; }
			if (res.done) {
				// Ended before the end of the file: the connection was cut.
				var b = sb.buffered, end = lp.offset + (b.length ? b.end(b.length - 1) : 0);
				if (lp.info && lp.info.duration && end < lp.info.duration - 3) { broke(new Error(_tf('stream ended at %d s', Math.floor(end)))); return; }
				done = true;
			} else { queue.push(res.value); }
			pump();
			read();
		}).catch(function (e) { reading = false; broke(e); });
	}
	// trim removes what lies far behind the position; force: the browser's
	// buffer is full, keep only 10 s.
	function trim(force) {
		var keep = force ? 10 : 60, b = sb.buffered;
		if (b.length && el.currentTime - b.start(0) > keep + 5) {
			try { sb.remove(0, el.currentTime - keep); } catch (e) { /* busy: next time */ }
		} else if (force) { setTimeout(pump, 1000); }
	}
	function pump() {
		if (!alive || !sb || sb.updating) { return; }
		if (first && sb.buffered.length) {
			first = false;
			// Start at the wanted position, not at the keyframe before it.
			var want = Math.max(sb.buffered.start(0), (t || 0) - lp.offset);
			if (lp.el === el && want > el.currentTime + 0.05) { el.currentTime = want; }
		}
		if (queue.length) {
			var n = 0, i;
			for (i = 0; i < queue.length; i++) { n += queue[i].length; }
			var chunk = new Uint8Array(n);
			for (n = 0, i = 0; i < queue.length; i++) { chunk.set(queue[i], n); n += queue[i].length; }
			queue = [];
			try { sb.appendBuffer(chunk); } catch (e) {
				if (e.name === 'QuotaExceededError') { queue.unshift(chunk); trim(true); return; }
				failed(e);
			}
			return;
		}
		if (done) {
			if (ms.readyState === 'open') { try { ms.endOfStream(); } catch (e) { /* closed meanwhile */ } }
			return;
		}
		trim(false);
	}
	ms.addEventListener('sourceopen', function () {
		if (!alive) { return; }
		try { sb = ms.addSourceBuffer(type); } catch (e) { failed(e); return; }
		sb.addEventListener('updateend', pump);
		sb.addEventListener('error', function (e) { failed(e); });
		fetch(url, { credentials: 'same-origin', signal: ctrl.signal })
			.then(function (r) {
				if (!r.ok || !r.body) {
					return r.text().then(function (msg) { throw new Error(msg || ('HTTP ' + r.status)); });
				}
				var st = parseFloat(r.headers.get('X-Stream-Start'));
				if (alive && lp && lp.el === el && !isNaN(st)) { lp.offset = st; lpApplySubs(); }
				reader = r.body.getReader();
				read();
			})
			.catch(function (e) {
				if (!alive) { return; }
				feed.broken = true;
				if (lp && lp.el === el) { libNotify('error', _t('Can\'t play: ') + e.message); lp.buffering = null; lp.userPaused = true; lpShowInfo(); lpShowState(); }
			});
	}, { once: true });
	return true;
}

// lpRestartBroken restarts a stream that broke off, where it is.
function lpRestartBroken() {
	if (!lp || !lp.el) { return; }
	lp.buffering = null;
	lpStart(lpPosition());
}

// lpStale: the stream can't go on after a pause: its MSE feed broke off, a
// plain converted stream was paused so long that the connection is likely
// gone, or the HLS session was (nearly) removed for being idle.
function lpStale() {
	var el = lp.el;
	if (!el || lpDirect()) { return false; }
	if (el.lpFeed) { return el.lpFeed.broken; }
	var paused = lp.pausedAt ? Date.now() - lp.pausedAt : 0;
	return paused > (lp.hlsSid ? 150000 : 30000);
}

// lpReachable: t (seconds of the stream) can be played without a new
// request: buffered by lpFeed, or seekable for the browser.
function lpReachable(el, t) { return (el.lpFeed && lpInRanges(el.buffered, t)) || lpInRanges(el.seekable, t); }

function lpStopHLS() {
	if (lp && lp.hlsSid) {
		fetch('/api/admin/library/play/hls/' + lp.hlsSid + '/stop', { credentials: 'same-origin', keepalive: true }).catch(function () {});
		lp.hlsSid = '';
	}
}

function lpStart(t) {
	var q = lpQ('.lp-quality').value;
	lpUnfeed(lp.el);
	lp.el.controls = false;
	if (lpDirect()) {
		lpStopHLS();
		lp.offset = 0;
		lp.el.src = lpBase() + '/stream?q=original';
		if (t > 0) { lp.el.addEventListener('loadedmetadata', function f() { lp.el.removeEventListener('loadedmetadata', f); lp.el.currentTime = t; }); }
	} else if (!lp.audio && lpUseHLS()) {
		lp.offset = t;
		var el = lp.el, mine = lp, prev = lp.hlsSid || '';
		lp.hlsSid = '';
		el.removeAttribute('src');
		fetch(lpBase() + '/hls?q=' + encodeURIComponent(q) + '&start=' + Math.floor(t) + lpSelParams() + '&prev=' + encodeURIComponent(prev), { credentials: 'same-origin' })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				if (lp !== mine || lp.el !== el) {
					fetch('/api/admin/library/play/hls/' + d.session + '/stop', { credentials: 'same-origin' }).catch(function () {});
					return;
				}
				lp.hlsSid = d.session;
				// The stream may begin at the keyframe before t.
				if (d.start >= 0 && d.start <= t) {
					lp.offset = d.start;
					lpApplySubs();
					if (t - d.start > 0.05) { el.addEventListener('loadedmetadata', function f() { el.removeEventListener('loadedmetadata', f); el.currentTime = t - d.start; }); }
				}
				el.src = d.url;
				lpShowInfo();
				lpBuffer();
			})
			.catch(function (e) { libNotify('error', _t('Can\'t play: ') + e.message); });
	} else {
		lpStopHLS();
		lp.offset = t;
		var url = lpBase() + '/stream?q=' + encodeURIComponent(q) + '&start=' + Math.floor(t) + lpSelParams() + '&pid=' + lpPid;
		if (lp.audio || !lpFeed(lp.el, url, t)) { lp.el.src = url; }
	}
	lpApplySubs();
	lpApplyRate(lp.el);
	lpShowInfo();
	lpShowState();
	lp.userPaused = false;
	lpBuffer();
}

// ---- Loading the next entry ahead ----
// lpNextIndex is the entry after the current one, or -1 at the end of the
// list (auto: the step at the end of a file, which only wraps when repeating).
function lpNextIndex(auto) {
	var n = lp.list.length;
	if (lp.shuffle && n > 1) {
		var left = [];
		for (var j = 0; j < n; j++) { if (!lp.played[j]) { left.push(j); } }
		if (!left.length) {
			if (auto && lp.repeat !== 'all') { return -1; }
			lp.played = {};
			lp.played[lp.idx] = true;
			for (var k = 0; k < n; k++) { if (k !== lp.idx) { left.push(k); } }
		}
		return left[Math.floor(Math.random() * left.length)];
	}
	var i = lp.idx + 1;
	if (i >= n) { return lp.repeat === 'all' ? 0 : -1; }
	return i;
}

// lpPreload starts loading the next entry some seconds before the current
// one ends, so it plays without a gap.
function lpPreload() {
	if (lp.pre || lp.repeat === 'one' || !lp.info || lp.list.length < 2 || (lp.sleep && lp.sleep.mode === 'file')) { return; }
	// Video over HLS starts its own session when it plays.
	if (!lp.audio && lpUseHLS()) { return; }
	var left = (lp.info.duration || 0) - lpPosition();
	if (!(lp.info.duration > 0) || left > 15) { return; }
	var i = lpNextIndex(true);
	if (i < 0) { lp.pre = { idx: -1 }; return; }
	var pre = { idx: i };
	lp.pre = pre;
	var it = lp.list[i];
	fetch(lpBaseOf(it) + '/info', { credentials: 'same-origin' })
		.then(function (r) { return r.ok ? r.json() : null; })
		.then(function (info) {
			if (!info || !lp || lp.pre !== pre) { return; }
			var el = lpMakeEl(info.audio);
			var q = lpWantedQuality(info);
			el.src = lpBaseOf(it) + '/stream?q=' + encodeURIComponent(q) + (info.mode === 'direct' && q === 'original' ? '' : '&start=0');
			el.load();
			pre.info = info;
			pre.el = el;
		})
		.catch(function () {});
}

function lpDiscard(pre) { if (pre && pre.el) { lpUnfeed(pre.el); pre.el.removeAttribute('src'); pre.el.load(); } }
function lpDiscardPre() { if (lp && lp.pre) { lpDiscard(lp.pre); lp.pre = null; } }

// ---- Buffering ----
// When the stream runs dry, the player waits until lpPrebuffer seconds lie
// ahead, so a varying speed doesn't stutter.
function lpPrebuffer() { return lp.audio ? 5 : 15; }

// lpAhead is how many seconds are buffered after the current position.
function lpAhead() {
	var el = lp.el, t = el.currentTime, b = el.buffered;
	for (var i = 0; i < b.length; i++) { if (b.start(i) <= t + 0.5 && b.end(i) >= t) { return b.end(i) - t; } }
	return 0;
}

function lpInRanges(ranges, t) {
	for (var i = 0; i < ranges.length; i++) { if (ranges.start(i) <= t && ranges.end(i) >= t) { return true; } }
	return false;
}

// lpBuffer pauses until playback can go on: a start (open, resume, seek,
// quality) only until the browser can play, which keeps buffering ahead
// while playing; a stall in playback until lpPrebuffer() seconds are there,
// so it doesn't stutter.
function lpBuffer(stall) {
	if (!lp || !lp.el) { return; }
	var el = lp.el, token = {}, since = Date.now();
	lp.buffering = token;
	if (!el.paused) { el.pause(); }
	lpShowInfo();
	lpShowState();
	(function check() {
		if (!lp || lp.buffering !== token || lp.el !== el) { return; }
		var need = stall ? lpPrebuffer() : 0;
		if (lp.info && lp.info.duration) { need = Math.min(need, Math.max(lp.info.duration - lpPosition() - 0.5, 0)); }
		// Nothing more is coming.
		if (el.lpFeed && el.lpFeed.broken && lpAhead() < need + 1) { lpRestartBroken(); return; }
		// A browser that stops loading while paused never reaches the
		// target: after 8 s, whatever it has must do.
		if (el.readyState >= 3 && (lpAhead() >= need || Date.now() - since > 8000)) {
			lp.buffering = null;
			lpShowInfo();
			el.play().catch(lpPlayRefused);
			return;
		}
		lpShowBuffered();
		setTimeout(check, 250);
	})();
}

// lpPlayRefused: the browser (iOS) only starts playback from a tap.
function lpPlayRefused(err) {
	if (!lp || !err || err.name !== 'NotAllowedError') { return; }
	lp.buffering = null;
	lp.userPaused = true;
	lpShowState();
	lpQ('.lp-info').textContent = _t('Tap ▶ to start');
}

function lpShowBuffered() {
	if (!lp || !lp.el) { return; }
	var ahead = lpAhead();
	lpQ('.lp-buf').textContent = ahead >= 1 ? _tf('· %s s buffered', Math.floor(ahead)) : '';
}

// lpShowInfo says how the file is played, with the picture size actually
// received, so a chosen quality can be checked.
function lpShowInfo() {
	if (!lp || !lp.info) { return; }
	var sel = lpQ('.lp-quality');
	// Another audio track repackages a direct file; burning in converts.
	var extra = lpSelParams(), burn = extra.indexOf('burn=') >= 0;
	var mode = burn ? 'transcode' : lp.info.mode === 'direct' && extra ? 'remux' : lp.info.mode;
	var text = lpDirect() ? _t('Original file')
		: sel.value === 'original' ? (mode === 'remux' ? _t('Original, repackaged') : _t('Original, converted'))
		: _t('Converted to ') + lp.info.qualities[sel.selectedIndex].label;
	if (burn) { text += _t(' · subtitles burned in'); }
	if (!lp.audio && lp.el && lp.el.videoHeight) { text += ' · ' + lp.el.videoWidth + '×' + lp.el.videoHeight; }
	if (lp.hlsSid) { text += ' · HLS'; }
	if (lp.subNote) { text += ' · ' + lp.subNote; }
	if (lp.buffering) { text += _t(' · Buffering…'); }
	if (lp.net) { text += _t(' · connection ') + lpFmtKbps(lp.net); }
	if (lp.info && lp.info.serverKbps && lp.info.serverKbps < (lp.net || Infinity)) { text += _t(' · server share ') + lpFmtKbps(lp.info.serverKbps); }
	if (lp.sleep) { text += ' · ' + lpSleepText(); }
	lpQ('.lp-info').textContent = text;
}

// lpVolShow fills the volume line up to v (0..1).
function lpVolShow(v) { var r = lpQ('.lp-vol'); if (r) { r.style.setProperty('--v', Math.round(v * 100) + '%'); } }

function lpShowState() {
	if (!lp || !lp.el) { return; }
	// While buffering, playback is on its way: the button pauses.
	lpQ('.lp-play').innerHTML = '<i class="fa-solid ' + (lp.el.paused && !lp.buffering ? 'fa-play' : 'fa-pause') + '"></i>';
	lpQ('.lp-mute').innerHTML = '<i class="fa-solid ' + (lp.el.muted || lp.el.volume === 0 ? 'fa-volume-xmark' : 'fa-volume-high') + '"></i>';
	lpQ('.lp-vol').value = lp.el.muted ? 0 : lp.el.volume;
	lpVolShow(lp.el.muted ? 0 : lp.el.volume);
	lpStore('lp-vol', String(lp.el.volume));
	if ('mediaSession' in navigator) { navigator.mediaSession.playbackState = lp.el.paused && !lp.buffering ? 'paused' : 'playing'; }
}

function lpToggle() {
	if (!lp || !lp.el) { return; }
	if (lp.buffering) { lp.buffering = null; lp.userPaused = true; lpShowInfo(); lpShowState(); return; }
	if (lp.el.paused) {
		if (lpStale()) { lp.pausedAt = 0; lpRestartBroken(); return; }
		// play() inside the tap, so iOS allows it; buffering may pause again.
		lp.el.play().catch(lpPlayRefused);
		lp.userPaused = false;
		lpBuffer();
	} else { lp.el.pause(); }
}

// lpFullscreen shows the player (video with its controls) full screen.
// Where page elements can't go full screen (the iPhone), the player fills
// the browser window instead (lpFullWindow), keeping its own controls; older
// Safari has the prefixed API.
function lpFullscreen() {
	if (!lp || lp.audio || !lp.el) { return; }
	var box = document.getElementById('libPlayer');
	var fsEl = document.fullscreenElement || document.webkitFullscreenElement;
	if (box.classList.contains('lp-fullwin')) {
		lpFullWindow(false);
	} else if (fsEl) {
		(document.exitFullscreen || document.webkitExitFullscreen).call(document);
	} else if (box.requestFullscreen && document.fullscreenEnabled !== false) {
		box.requestFullscreen().catch(function () { lpFullWindow(true); });
	} else if (box.webkitRequestFullscreen && document.webkitFullscreenEnabled) {
		box.webkitRequestFullscreen();
	} else {
		lpFullWindow(true);
	}
}

// The player has three sizes: 'mini' (a bar), 'expanded' (bigger at the
// bottom, with chapters and playlist) and 'fullwin' (covering the window,
// video only). The size button cycles through them.
function lpSize() {
	var box = document.getElementById('libPlayer');
	return box.classList.contains('lp-fullwin') ? 'fullwin' : box.classList.contains('lp-expanded') ? 'expanded' : 'mini';
}

function lpSetSize(size) {
	var box = document.getElementById('libPlayer');
	// Full window keeps 'expanded' underneath: leaving it returns there.
	box.classList.toggle('lp-expanded', size !== 'mini');
	lpFullWindow(size === 'fullwin');
}

// lpSizeButton shows what the size button does next.
function lpSizeButton() {
	var b = document.querySelector('#libPlayer .lp-expand');
	if (!b || !lp) { return; }
	var size = lpSize(), next = size === 'mini' ? 'expanded' : size === 'expanded' && !lp.audio ? 'fullwin' : 'mini';
	var look = { expanded: ['fa-up-right-and-down-left-from-center', _t('Bigger player')], fullwin: ['fa-maximize', _t('Full window')], mini: ['fa-down-left-and-up-right-to-center', _t('Small player')] }[next];
	b.innerHTML = '<i class="fa-solid ' + look[0] + '"></i>';
	b.title = look[1];
	b.setAttribute('aria-label', look[1]);
	b.classList.toggle('active', size !== 'mini');
}

// lpFullWindow lets the player cover the browser window (on) or go back.
function lpFullWindow(on) {
	var box = document.getElementById('libPlayer');
	if (!box) { return; }
	box.classList.toggle('lp-fullwin', on);
	document.body.classList.toggle('lp-fullwin-on', on);
	lpOverlaySync();
	var b = box.querySelector('.lp-full');
	if (b) {
		b.classList.toggle('active', on);
		b.title = on ? _t('Leave full window (F, Esc)') : _t('Full screen (F)');
		b.innerHTML = '<i class="fa-solid ' + (on ? 'fa-compress' : 'fa-expand') + '"></i>';
	}
	lpSizeButton();
	lpDockSpace();
}

// lpSeek jumps to t seconds of the current file. A converted stream
// restarts there, unless t lies in the part the browser already has.
function lpSeek(t) {
	if (!lp || !lp.el) { return; }
	t = Math.max(0, lp.info && lp.info.duration ? Math.min(t, lp.info.duration - 1) : t);
	if (lpDirect()) {
		lp.el.currentTime = t;
	} else if (t >= lp.offset && lpReachable(lp.el, t - lp.offset)) {
		lp.el.currentTime = t - lp.offset;
	} else {
		if (!lpSeekAhead(t)) { lpStart(t); }
		return;
	}
	lp.userPaused = false;
	lpBuffer();
}

// lpSeekAhead jumps forward in a repackaged or converted stream to a point
// it hasn't received yet but soon will (at most 30 s past what it has): it
// waits up to 8 s for it instead of restarting the conversion there, which
// would start with an empty buffer. False: restart instead.
function lpSeekAhead(t) {
	var el = lp.el, rel = t - lp.offset, b = el.buffered;
	if (rel < 0 || !b.length || rel > b.end(b.length - 1) + 30) { return false; }
	var token = { target: t }, since = Date.now();
	lp.buffering = token;
	lp.userPaused = false;
	if (!el.paused) { el.pause(); }
	lpShowInfo();
	lpShowState();
	(function check() {
		if (!lp || lp.buffering !== token || lp.el !== el) { return; }
		if (lpInRanges(el.buffered, rel) && lpReachable(el, rel)) {
			lp.buffering = null;
			el.currentTime = rel;
			lpBuffer();
			return;
		}
		if (Date.now() - since > 8000) { lp.buffering = null; lpStart(t); return; }
		lpShowBuffered();
		setTimeout(check, 250);
	})();
	return true;
}

// lpSeekAbs jumps on the timeline: across the parts of a book, else within
// the file.
function lpSeekAbs(t) {
	if (!lp) { return; }
	if (!lp.book) { lpSeek(t); return; }
	t = Math.max(0, Math.min(t, lp.book.duration - 1));
	var parts = lp.book.parts, i = 0;
	while (i < parts.length - 1 && t >= parts[i + 1].start) { i++; }
	if (i === lp.idx) { lpSeek(t - parts[i].start); } else { lpDiscardPre(); lpLoad(i, t - parts[i].start); }
}

// Several presses while a jump waits for the stream add up.
function lpJump(d) {
	if (!lp || !lp.el) { return; }
	var waiting = lp.buffering && lp.buffering.target !== undefined;
	lpSeekAbs((waiting ? lpPartStart() + lp.buffering.target : lpAbs()) + d);
}

// ---- Timeline ----
// The position bar: a line with the played part, what is loaded and a
// marker. Hovering shows the time under the pointer, a click or drag jumps
// there; the position label takes a typed time (1:23:45, 83:45, 90).

// lpTlShow draws the marker at abs seconds (or the dragged time).
function lpTlShow(abs) {
	var tl = lpQ('.lp-tl'), total = lpTotal();
	if (!tl) { return; }
	var pct = function (t) { return (total > 0 ? Math.max(0, Math.min(1, t / total)) * 100 : 0) + '%'; };
	lpQ('.lp-tl-fill').style.width = pct(abs);
	lpQ('.lp-tl-knob').style.left = pct(abs);
	// The loaded part around the position.
	var buf = lpQ('.lp-tl-buf'), el = lp.el, from = 0, to = 0;
	if (el && el.buffered) {
		for (var i = 0; i < el.buffered.length; i++) {
			if (el.buffered.start(i) <= el.currentTime + 0.5 && el.buffered.end(i) >= el.currentTime) {
				var base = lpPartStart() + (lp.offset || 0);
				from = base + el.buffered.start(i);
				to = base + el.buffered.end(i);
			}
		}
	}
	buf.style.left = pct(from);
	buf.style.width = (total > 0 ? Math.max(0, Math.min(to, total) - from) / total * 100 : 0) + '%';
	tl.setAttribute('aria-valuemax', String(Math.floor(total)));
	tl.setAttribute('aria-valuenow', String(Math.floor(abs)));
	tl.setAttribute('aria-valuetext', lpFmt(abs) + ' of ' + lpFmt(total));
}

// lpParseTime reads "1:23:45", "83:45" or "90" (seconds); NaN if invalid.
function lpParseTime(s) {
	var parts = String(s).trim().split(':');
	if (parts.length > 3 || parts.some(function (p) { return !/^\d+$/.test(p); })) { return NaN; }
	return parts.reduce(function (acc, p) { return acc * 60 + +p; }, 0);
}

function lpTimeline() {
	var tl = lpQ('.lp-tl'), tip = lpQ('.lp-tl-tip'), hover = lpQ('.lp-tl-hover');
	function at(e) {
		var r = tl.getBoundingClientRect(), f = r.width ? Math.max(0, Math.min(1, (e.clientX - r.left) / r.width)) : 0;
		return { t: f * lpTotal(), x: f * 100 + '%' };
	}
	function showTip(p) {
		tip.textContent = lpFmt(p.t);
		tip.style.left = p.x;
		hover.style.left = p.x;
		tip.classList.remove('d-none');
		hover.classList.remove('d-none');
	}
	function hideTip() { tip.classList.add('d-none'); hover.classList.add('d-none'); }
	tl.addEventListener('pointermove', function (e) {
		if (!lp || !lpTotal()) { return; }
		var p = at(e);
		showTip(p);
		if (lp.tlDrag) { lp.tlDrag = p.t; lpTlShow(p.t); }
	});
	tl.addEventListener('pointerleave', function () { if (!lp || !lp.tlDrag) { hideTip(); } });
	tl.addEventListener('pointerdown', function (e) {
		if (!lp || !lpTotal() || e.button > 0) { return; }
		e.preventDefault();
		tl.focus();
		tl.setPointerCapture(e.pointerId);
		tl.classList.add('lp-tl-drag');
		var p = at(e);
		lp.tlDrag = p.t;
		showTip(p);
		lpTlShow(p.t);
	});
	function end(e, jump) {
		if (!lp || !lp.tlDrag) { return; }
		var t = lp.tlDrag;
		lp.tlDrag = null;
		tl.classList.remove('lp-tl-drag');
		if (e.pointerType !== 'mouse') { hideTip(); }
		if (jump) { lpSeekAbs(t); } else { lpTlShow(lpAbs()); }
	}
	tl.addEventListener('pointerup', function (e) { end(e, true); });
	tl.addEventListener('pointercancel', function (e) { end(e, false); });
	// Arrows jump 10 s (the player's keys); Page Up/Down a minute.
	tl.addEventListener('keydown', function (e) {
		var d = { PageUp: 60, PageDown: -60 }[e.key];
		if (d) { e.preventDefault(); e.stopPropagation(); lpJump(d); }
		if (e.key === 'Home') { e.preventDefault(); e.stopPropagation(); lpSeekAbs(0); }
	});
	// The position label turns into a field for a typed time.
	var pos = lpQ('.lp-pos'), go = lpQ('.lp-goto');
	function closeGo() { go.classList.add('d-none'); go.classList.remove('is-invalid'); pos.classList.remove('d-none'); }
	pos.addEventListener('click', function () {
		if (!lp || !lpTotal()) { return; }
		go.value = lpFmt(lpAbs());
		pos.classList.add('d-none');
		go.classList.remove('d-none');
		go.focus();
		go.select();
	});
	go.addEventListener('keydown', function (e) {
		if (e.key === 'Escape') { e.preventDefault(); closeGo(); pos.focus(); return; }
		if (e.key !== 'Enter') { return; }
		e.preventDefault();
		var t = lpParseTime(go.value);
		if (isNaN(t) || t > lpTotal()) { go.classList.add('is-invalid'); return; }
		closeGo();
		lpSeekAbs(t);
	});
	go.addEventListener('blur', closeGo);
}

function lpTick() {
	var abs = lpAbs();
	if (!lp.tlDrag) { lpTlShow(abs); }
	lpQ('.lp-pos').textContent = lpFmt(abs);
	lpShowBuffered();
	var now = Date.now();
	if (!lp.el.paused && (!lp.reported || now - lp.reported > 10000)) { lp.reported = now; lpReport('play'); }
	// The chapter being played is highlighted.
	var cur = null;
	lpQ('.lp-chapters').querySelectorAll('[data-start]').forEach(function (b) { if (+b.dataset.start <= abs + 0.5) { cur = b; } });
	lpQ('.lp-chapters').querySelectorAll('[data-start]').forEach(function (b) { b.classList.toggle('active', b === cur); });
	if (lp.el.lpFeed && lp.el.lpFeed.broken && !lp.el.paused && lpAhead() < 5) { lpRestartBroken(); return; }
	lpSleepTick(abs);
	lpPreload();
	lpSessionPosition();
}

function lpEnded() {
	// A converted stream that ends well before the file does was cut off.
	if (!lpDirect() && lp.info && lp.info.duration && lpPosition() < lp.info.duration - 10) { lpRestartBroken(); return; }
	lpReport('end');
	if (lp.repeat === 'one') { lpSeek(0); return; }
	if (lp.sleep && lp.sleep.mode === 'file') { lpSleepNow(); return; }
	lpStep(1, true);
}

// lpStep goes to the next (d=1) or previous (d=-1) entry; auto is the step
// at the end of a file, which stops at the end of the list unless repeating.
function lpStep(d, auto) {
	if (!lp) { return; }
	if (d > 0) {
		// The preloaded entry is the one lpNextIndex picked.
		if (lp.pre && lp.pre.idx >= 0) { lpLoad(lp.pre.idx); return; }
		var next = lpNextIndex(auto);
		if (next < 0 && !auto && lp.shuffle) { next = lpNextIndex(false); }
		if (next >= 0) { lpLoad(next); }
		return;
	}
	lpDiscardPre();
	var n = lp.list.length, i;
	if (lp.shuffle && n > 1) {
		if (lp.hist.length < 2) { return; }
		lp.hist.pop();
		lpLoad(lp.hist.pop());
		return;
	}
	i = lp.idx - 1;
	if (i < 0) { if (lp.repeat !== 'all') { return; } i = n - 1; }
	lpLoad(i);
}

// ---- Sleep timer (audiobooks) ----
function lpSetSleep(v) {
	if (!v) { lp.sleep = null; lpShowInfo(); return; }
	if (v === 'chapter') {
		var abs = lpAbs(), next = null;
		lpChapters().forEach(function (c) { if (next === null && c.start > abs + 1) { next = c.start; } });
		lp.sleep = { mode: 'chapter', at: next === null ? lpTotal() : next };
	} else if (v === 'file') {
		lp.sleep = { mode: 'file' };
		lpDiscardPre();
	} else {
		lp.sleep = { mode: 'time', at: Date.now() + (+v) * 60000 };
	}
	lpShowInfo();
}

function lpSleepText() {
	if (lp.sleep.mode === 'time') { return _tf('sleep in %d min', Math.max(1, Math.ceil((lp.sleep.at - Date.now()) / 60000))); }
	return lp.sleep.mode === 'chapter' ? _t('sleep at the end of the chapter') : _t('sleep at the end of the part');
}

function lpSleepTick(abs) {
	if (!lp.sleep) { return; }
	if ((lp.sleep.mode === 'time' && Date.now() >= lp.sleep.at) || (lp.sleep.mode === 'chapter' && abs >= lp.sleep.at)) { lpSleepNow(); }
}

function lpSleepNow() {
	lp.sleep = null;
	lp.buffering = null;
	if (lp.el) { lp.el.pause(); }
	var s = lpQ('.lp-sleep');
	if (s) { s.value = ''; }
	lpShowInfo();
	libNotify('info', _t('Sleep timer: paused'));
}

// ---- Media keys and lock screen (Media Session API) ----
function lpSessionHandlers() {
	if (!('mediaSession' in navigator)) { return; }
	var ms = navigator.mediaSession;
	var set = function (action, fn) { try { ms.setActionHandler(action, fn); } catch (e) { /* not supported */ } };
	set('play', function () { if (lp && lp.el && lp.el.paused) { lpToggle(); } });
	set('pause', function () { if (lp && lp.el && !lp.el.paused) { lpToggle(); } });
	set('previoustrack', function () { lpStep(-1, false); });
	set('nexttrack', function () { lpStep(1, false); });
	set('seekbackward', function (d) { lpJump(-((d && d.seekOffset) || 10)); });
	set('seekforward', function (d) { lpJump((d && d.seekOffset) || 10); });
	set('seekto', function (d) { if (d && d.seekTime !== undefined) { lpSeekAbs(d.seekTime); } });
}

function lpSessionMeta() {
	if (!('mediaSession' in navigator) || !window.MediaMetadata) { return; }
	var src = lpCoverSrc();
	var art = src ? [{ src: new URL(src, location.href).href, sizes: '512x512' }] : [];
	var m = (lp.info && lp.info.meta) || {};
	navigator.mediaSession.metadata = new MediaMetadata({ title: m.title || lpItem().label, artist: m.artist || lp.entry || '',
		album: m.album || lp.entry || '', artwork: art });
}

function lpSessionPosition() {
	if (!('mediaSession' in navigator) || !navigator.mediaSession.setPositionState) { return; }
	var total = lpTotal();
	if (!(total > 0)) { return; }
	try { navigator.mediaSession.setPositionState({ duration: total, position: Math.min(lpAbs(), total), playbackRate: (lp.el && lp.el.playbackRate) || 1 }); } catch (e) { /* ignore */ }
}

function lpSessionClear() {
	if (!('mediaSession' in navigator)) { return; }
	navigator.mediaSession.metadata = null;
	navigator.mediaSession.playbackState = 'none';
	['play', 'pause', 'previoustrack', 'nexttrack', 'seekbackward', 'seekforward', 'seekto'].forEach(function (a) {
		try { navigator.mediaSession.setActionHandler(a, null); } catch (e) { /* ignore */ }
	});
}

// ---- Keyboard ----
// Space play/pause, ←/→ 10 seconds, N/P next/previous, M mute, F full
// screen; not while typing in a field.
document.addEventListener('keydown', function (e) {
	if (!lp || !lp.el || e.ctrlKey || e.metaKey || e.altKey) { return; }
	var t = e.target, tag = t && t.tagName;
	if (tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || (t && t.isContentEditable)) { return; }
	switch (e.key) {
		case ' ': case 'k': case 'K':
			if (tag === 'BUTTON' && e.key === ' ') { return; }
			e.preventDefault(); lpToggle(); break;
		case 'ArrowLeft': e.preventDefault(); lpJump(-10); break;
		case 'ArrowRight': e.preventDefault(); lpJump(10); break;
		case 'n': case 'N': lpStep(1, false); break;
		case 'p': case 'P': lpStep(-1, false); break;
		case 'm': case 'M': lp.el.muted = !lp.el.muted; break;
		case 'f': case 'F': lpFullscreen(); break;
		case 'i': case 'I': lpAbout(); break;
		case 'Escape': if (document.getElementById('libPlayer').classList.contains('lp-fullwin')) { lpFullWindow(false); } break;
	}
});

// Stopping the media element ends the stream, which stops ffmpeg.
function lpStop() {
	if (lp) { lpReport('stop'); lpDiscardPre(); lpStopHLS(); lp.ready = null; }
	if (lp && lp.el) { lpUnfeed(lp.el); lp.el.pause(); lp.el.removeAttribute('src'); lp.el.load(); }
	if (document.fullscreenElement) { document.exitFullscreen().catch(function () {}); }
	lpFullWindow(false);
	lp = null;
}

function lpClose() {
	lpStop();
	lpSessionClear();
	var box = document.getElementById('libPlayer');
	if (box) { box.classList.add('d-none'); box.innerHTML = ''; }
	lpDockSpace();
}

// The page and the dialog leave room for the docked player.
function lpDockSpace() {
	var box = document.getElementById('libPlayer');
	var on = box && !box.classList.contains('d-none');
	// Covering the window, the player needs no room below the page.
	if (on && box.classList.contains('lp-fullwin')) { return; }
	document.body.classList.toggle('lp-docked', !!on);
	document.documentElement.style.setProperty('--lp-h', on ? box.offsetHeight + 'px' : '0px');
}
if (window.ResizeObserver) { new ResizeObserver(lpDockSpace).observe(document.getElementById('libPlayer')); }


// lpAbout shows (or hides) the Info panel: what is playing, with cover,
// year, rating and overview, for an episode also the episode. It stays
// open (and follows the next file) until closed.
function lpAbout(refresh) {
	var box = document.getElementById('libPlayer'), el = box && box.querySelector('.lp-about-box');
	if (refresh && !el) { return; }
	if (el && !refresh) { el.remove(); lpQ('.lp-about').classList.remove('active'); return; }
	if (!el) {
		el = document.createElement('div');
		el.className = 'lp-about-box';
		box.appendChild(el);
		lpQ('.lp-about').classList.add('active');
	}
	var m = (lp && lp.info && lp.info.meta) || {}, cover = m.cover || lp.cover;
	var esc = function (t) { var d = document.createElement('div'); d.textContent = t || ''; return d.innerHTML; };
	// An episode's heading is the series already; music shows its artist.
	var line = [m.year, m.rating ? '★ ' + m.rating : '', m.episodeTitle ? '' : m.artist].filter(Boolean).join(' · ');
	var head = m.episodeTitle ? (m.artist || lp.entry) : (m.title || lpItem().label);
	el.innerHTML = '<button type="button" class="btn-close btn-close-white float-end" aria-label="' + _t('Close') + '"></button>' +
		(cover ? '<img src="' + esc(cover) + '" alt="">' : '') +
		'<div class="fw-semibold fs-5">' + esc(head) + '</div>' +
		(line ? '<div class="small text-white-50">' + esc(line) + '</div>' : '') +
		(m.overview ? '<p>' + esc(m.overview) + '</p>' : '') +
		(m.episodeTitle ? '<div class="lp-about-ep"><div class="fw-semibold">' + esc([m.album, m.episodeTitle].filter(Boolean).join(' · ')) + '</div>' +
			(m.aired ? '<div class="small text-white-50">' + _tf('Aired %s', esc(m.aired)) + '</div>' : '') +
			(m.episodeOverview ? '<p>' + esc(m.episodeOverview) + '</p>' : '') + '</div>' : '') +
		(m.entryType && m.entryId ? '<div class="clearfix"></div><button type="button" class="btn btn-sm btn-outline-light mt-2 lp-about-details">' +
			'<i class="fa-solid fa-circle-info me-1"></i>' + _t('Show details') + '</button>' : '');
	el.querySelector('.btn-close').addEventListener('click', function () { lpAbout(); });
	var det = el.querySelector('.lp-about-details');
	if (det) { det.addEventListener('click', function () { lpOpenDetails(m.entryType, m.entryId); }); }
}

// lpOpenDetails shows the Library details of the playing entry: the player
// leaves full screen / full window for it and keeps playing. Pages without
// the Library (Calendar) go to it.
function lpOpenDetails(type, id) {
	lpAbout();
	if (typeof openLibraryItem !== 'function') { location.href = '/api/admin/library#' + type + '/' + id; return; }
	if (document.fullscreenElement && document.exitFullscreen) { document.exitFullscreen(); }
	var box = document.getElementById('libPlayer');
	if (box && box.classList.contains('lp-fullwin')) { lpFullWindow(false); }
	openLibraryItem(type, id);
}

// lpOverlaySync lays the controls over a video in full window and full
// screen; they hide after a few seconds without mouse or keys while it
// plays.
function lpOverlaySync() {
	var box = document.getElementById('libPlayer');
	if (!box) { return; }
	var fs = (document.fullscreenElement || document.webkitFullscreenElement) === box;
	var on = !!(lp && !lp.audio && (fs || box.classList.contains('lp-fullwin')));
	box.classList.toggle('lp-overlay', on);
	lpWake();
}

var lpIdleTimer = null;
function lpWake() {
	var box = document.getElementById('libPlayer');
	if (!box) { return; }
	box.classList.remove('lp-idle');
	clearTimeout(lpIdleTimer);
	if (!box.classList.contains('lp-overlay')) { return; }
	lpIdleTimer = setTimeout(function () {
		var main = box.querySelector('.lp-main');
		// Not while paused, or while the mouse is on the controls or a list is open.
		if (!lp || !lp.el || lp.el.paused || (main && main.matches(':hover')) ||
			(document.activeElement && document.activeElement.tagName === 'SELECT' && box.contains(document.activeElement))) {
			lpWake();
			return;
		}
		box.classList.add('lp-idle');
	}, 3000);
}

document.addEventListener('fullscreenchange', lpOverlaySync);
document.addEventListener('webkitfullscreenchange', lpOverlaySync);
['mousemove', 'mousedown', 'touchstart', 'keydown'].forEach(function (ev) {
	document.addEventListener(ev, function (e) {
		var box = document.getElementById('libPlayer');
		if (box && box.classList.contains('lp-overlay') && (ev === 'keydown' || box.contains(e.target))) { lpWake(); }
	}, { passive: true });
});
function libNotify(kind, msg) { if (typeof showToaster === 'function') { showToaster(kind, msg); } else { alert(msg); } }
`

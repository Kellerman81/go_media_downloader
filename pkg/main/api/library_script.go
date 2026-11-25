package api

import ()

const libraryCSS = `
.lib-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(var(--lib-size,150px),1fr));gap:1rem}
.lib-card{cursor:pointer;min-width:0}
.lib-card:focus-visible{outline:2px solid var(--bs-primary,#3b7ddd);outline-offset:3px;border-radius:.5rem}
.lib-cover{position:relative;aspect-ratio:2/3;border-radius:.5rem;overflow:hidden;background:#e9ecef;box-shadow:0 1px 3px rgba(0,0,0,.15);transition:transform .15s,box-shadow .15s}
.lib-square .lib-cover{aspect-ratio:1/1}
.lib-cover img{width:100%;height:100%;object-fit:cover;display:block}
.lib-card:hover .lib-cover{transform:translateY(-2px);box-shadow:0 6px 16px rgba(0,0,0,.2)}
.lib-badge{position:absolute;top:.4rem;right:.4rem}
.lib-cover .lib-progress{position:absolute;left:.4rem;bottom:.4rem;opacity:.9}
.lib-title{font-weight:600;font-size:.9rem;margin-top:.4rem}
.lib-sub{font-size:.8rem;color:#6c757d}
.lib-thumb{width:calc(var(--lib-size,150px) * .375);height:calc(var(--lib-size,150px) * .5625);object-fit:cover;background:#e9ecef;flex-shrink:0}
.lib-thumb-square{height:calc(var(--lib-size,150px) * .375)}
.lib-detail-cover{width:200px;aspect-ratio:2/3;object-fit:cover;align-self:flex-start;background:#e9ecef}
.lib-detail-square{aspect-ratio:1/1}
#libModal .edit-form-header{display:none}
#libModal .lib-edit-section{border:1px solid #dee2e6;border-radius:.5rem;padding:0 1rem 1rem;margin-bottom:1.5rem;scroll-margin-top:1rem}
#libModal .lib-edit-head{font-weight:600;font-size:1.05rem;padding:.75rem 0;margin-bottom:.5rem;border-bottom:1px solid #dee2e6}
.lib-ep-thumb{width:96px;height:54px;object-fit:cover;background:#e9ecef;display:block}
.d-contents{display:contents}
.lib-selbar{position:sticky;top:0;z-index:20;display:flex;flex-wrap:wrap;align-items:center;gap:.4rem;padding:.6rem .75rem;margin-bottom:.75rem;background:var(--bs-body-bg,#fff);border:1px solid var(--bs-primary,#3b7ddd);border-radius:.5rem;box-shadow:0 2px 8px rgba(0,0,0,.1)}
.lib-selecting [data-lib-id]{position:relative}
.lib-selecting [data-lib-id]::before{content:"";position:absolute;top:.45rem;left:.45rem;width:1.4rem;height:1.4rem;border-radius:.3rem;border:2px solid #fff;background:rgba(0,0,0,.35);z-index:3;box-shadow:0 0 0 1px rgba(0,0,0,.3)}
.lib-selecting .list-group-item[data-lib-id]::before{top:50%;transform:translateY(-50%);left:.25rem}
.lib-selecting [data-lib-id].lib-selected::before{content:"\2713";background:var(--bs-primary,#3b7ddd);color:#fff;font-weight:700;text-align:center;line-height:1.1rem}
.lib-selecting [data-lib-id].lib-selected .lib-cover,.lib-selecting .list-group-item.lib-selected{outline:3px solid var(--bs-primary,#3b7ddd);outline-offset:2px}
.lib-desc{grid-template-columns:repeat(auto-fill,minmax(calc(var(--lib-size,150px) + 50px),1fr));gap:1.25rem}
.lib-desc-card .lib-title{display:-webkit-box;-webkit-line-clamp:2;-webkit-box-orient:vertical;overflow:hidden}
.lib-overview{font-size:.8rem;margin:.3rem 0 0;display:-webkit-box;-webkit-line-clamp:5;-webkit-box-orient:vertical;overflow:hidden}
@media (max-width:575px){.lib-grid{grid-template-columns:repeat(auto-fill,minmax(min(var(--lib-size,110px),42vw),1fr))}.lib-desc{grid-template-columns:repeat(auto-fill,minmax(min(calc(var(--lib-size,110px) + 50px),42vw),1fr))}.lib-detail-cover{width:140px}}
`

const libraryJS = `
var libCsrf = {{csrf}};

var libCurrent = null;

function libReloadGrid() { htmx.trigger('#libForm', 'change'); }

// The episode list of a series follows the grid's filter unless filter is given.
// The open entry is kept in the address (#movie/12), so reloading, the back
// button and bookmarks bring the dialog back.
function openLibraryItem(type, id, filter, fromHistory) {
	if (filter === undefined) { var f = document.querySelector('#libForm [name="filter"]'); filter = f ? f.value : 'all'; }
	libCurrent = { type: type, id: id, filter: filter };
	var hash = '#' + type + '/' + id;
	if (!fromHistory && location.hash !== hash) { history.pushState(null, '', hash); }
	var body = document.getElementById('libModalBody');
	document.getElementById('libModalTitle').textContent = '';
	body.innerHTML = '<div class="text-center p-5"><div class="spinner-border text-primary" role="status"></div></div>';
	bootstrap.Modal.getOrCreateInstance(document.getElementById('libModal')).show();
	fetch('/api/admin/library/item?type=' + encodeURIComponent(type) + '&id=' + id + '&filter=' + encodeURIComponent(filter), { credentials: 'same-origin' })
		.then(function (r) { return r.text(); })
		.then(function (html) {
			body.innerHTML = html;
			var root = body.firstElementChild;
			document.getElementById('libModalTitle').textContent = root && root.dataset.title ? root.dataset.title : '';
			libLoadLinks();
			libLoadCollection();
			var more = body.querySelector('[data-bs-target="#libTabMore"]');
			if (more) { more.addEventListener('shown.bs.tab', libLoadExtras, { once: true }); }
		})
		.catch(function (e) { body.innerHTML = '<div class="alert alert-danger">' + e.message + '</div>'; });
}

// libLoadLinks adds "Open in Plex/Jellyfin" for a movie or series.
function libLoadLinks() {
	var box = document.getElementById('libLinks');
	if (!box) { return; }
	fetch('/api/admin/library/links?type=' + encodeURIComponent(box.dataset.type) + '&id=' + encodeURIComponent(box.dataset.id), { credentials: 'same-origin' })
		.then(function (r) { return r.ok ? r.json() : []; })
		.then(function (links) {
			(links || []).forEach(function (l) {
				var a = document.createElement('a');
				a.className = 'btn btn-sm btn-outline-secondary';
				a.href = l.url; a.target = '_blank'; a.rel = 'noopener';
				a.title = _t('Open in ') + l.server;
				a.innerHTML = '<i class="fa-solid fa-arrow-up-right-from-square me-1"></i>';
				a.appendChild(document.createTextNode(l.server));
				box.appendChild(a);
			});
		})
		.catch(function () {});
	fetch('/api/admin/library/trailer?type=' + encodeURIComponent(box.dataset.type) + '&id=' + encodeURIComponent(box.dataset.id) +
		'&list=' + encodeURIComponent(box.dataset.list || ''), { credentials: 'same-origin' })
		.then(function (r) { return r.status === 200 ? r.json() : null; })
		.then(function (t) {
			if (!t || !t.key) { return; }
			var b = document.createElement('button');
			b.type = 'button'; b.className = 'btn btn-sm btn-outline-danger';
			b.title = _t('Trailer') + (t.language ? ' (' + t.language.toUpperCase() + ')' : '') + (t.name ? ': ' + t.name : '');
			b.setAttribute('aria-label', b.title);
			b.innerHTML = '<i class="fa-brands fa-youtube"></i>';
			b.addEventListener('click', function () { libShowTrailer(t); });
			box.insertBefore(b, box.firstChild);
		})
		.catch(function () {});
}

// libLoadExtras fills the More tab: cast, videos, recommendations, similar
// titles and reviews.
function libLoadExtras() {
	var box = document.getElementById('libExtras');
	if (!box) { return; }
	fetch('/api/admin/library/extras?type=' + encodeURIComponent(box.dataset.type) + '&id=' + encodeURIComponent(box.dataset.id) +
		'&list=' + encodeURIComponent(box.dataset.list || ''), { credentials: 'same-origin' })
		.then(function (r) { return r.text(); })
		.then(function (h) { box.innerHTML = h; })
		.catch(function (e) { box.innerHTML = '<div class="alert alert-warning">' + e.message + '</div>'; });
}

// libLoadCollection shows the TMDB collection of a movie (admins): its
// movies, adding the missing ones and adding new ones automatically.
function libLoadCollection() {
	var box = document.getElementById('libCollection');
	if (!box) { return; }
	var q = 'id=' + encodeURIComponent(box.dataset.id) + '&list=' + encodeURIComponent(box.dataset.list || '');
	fetch('/api/admin/library/collection?' + q, { credentials: 'same-origin' })
		.then(function (r) { return r.status === 200 ? r.json() : null; })
		.then(function (d) {
			if (!d || !d.parts || d.parts.length < 2) { return; }
			var missing = d.parts.filter(function (p) { return !p.have && !p.excluded; }).length;
			var have = d.parts.filter(function (p) { return p.have; }).length;
			box.innerHTML = '<div class="border rounded p-2"><div class="d-flex flex-wrap align-items-center gap-2 mb-2">' +
				'<strong class="lib-coll-name"></strong><span class="badge bg-secondary"></span><span class="flex-grow-1"></span>' +
				'<div class="form-check form-switch mb-0"><input class="form-check-input" type="checkbox" id="libCollMonitor">' +
				'<label class="form-check-label small" for="libCollMonitor">' + _t('Add new movies automatically') + '</label></div>' +
				'<button type="button" class="btn btn-sm btn-primary lib-coll-add"></button></div><div class="d-flex flex-wrap gap-1 lib-coll-parts"></div></div>';
			box.querySelector('.lib-coll-name').textContent = _t('Part of ') + d.name;
			// On the Library page the name shows the collection's movies in the grid.
			if (document.getElementById('libColl')) {
				var nm = box.querySelector('.lib-coll-name');
				nm.setAttribute('role', 'button');
				nm.title = _t('Show the movies of this collection');
				nm.classList.add('text-primary');
				nm.style.cursor = 'pointer';
				nm.onclick = function () { libShowCollection(d.id); };
			}
			box.querySelector('.badge').textContent = _tf('%s / %s in this media group', have, d.parts.length);
			var add = box.querySelector('.lib-coll-add');
			add.textContent = missing ? _tf('Add %d missing', missing) : _t('Complete');
			add.disabled = !missing;
			var mon = box.querySelector('#libCollMonitor');
			mon.checked = !!d.monitored;
			d.parts.forEach(function (p) {
				var s = document.createElement('span');
				s.className = 'badge ' + (p.have ? 'text-bg-success' : p.excluded ? 'text-bg-dark' : 'text-bg-light border');
				s.title = p.have ? _t('In a list of this media group') : p.excluded ? _t('Excluded (Import Exclusions)') : _t('Not in this media group');
				s.textContent = p.title + (p.year ? ' (' + p.year + ')' : '');
				box.querySelector('.lib-coll-parts').appendChild(s);
			});
			var post = function (vals, done) {
				vals.id = box.dataset.id; vals.list = box.dataset.list || ''; vals.csrf_token = libCsrf;
				fetch('/api/admin/library/collection/add', { method: 'POST', credentials: 'same-origin', body: new URLSearchParams(vals), headers: { 'X-CSRF-Token': libCsrf } })
					.then(function (r) { return r.json().then(function (j) { if (!r.ok) { throw new Error(j.error || ('HTTP ' + r.status)); } return j; }); })
					.then(done)
					.catch(function (e) { libNotify('error', e.message); });
			};
			mon.addEventListener('change', function () {
				post({ monitor: mon.checked ? '1' : '0' }, function () {
					libNotify('success', mon.checked ? _tf('New movies of %s are added to %s', d.name, box.dataset.list) : _t('No longer adding new movies of ') + d.name);
				});
			});
			add.addEventListener('click', function () {
				add.disabled = true;
				post({ add: '1' }, function (j) { libNotify('success', _t('Adding ') + j.adding + ' movies in the background - they show up in the Library shortly'); });
			});
		})
		.catch(function () {});
}

// libShowTrailer plays a YouTube trailer over the page (privacy-enhanced
// embed: no cookies before playing).
function libShowTrailer(t) {
	// Brave strips the Referer of embeds, which YouTube answers with error 153:
	// play the trailer on YouTube's own page in a window of its own.
	if (navigator.brave) {
		window.open('https://www.youtube.com/watch?v=' + encodeURIComponent(t.key), 'gmdTrailer', 'popup,width=1100,height=700');
		return;
	}
	var o = document.createElement('div');
	o.className = 'position-fixed top-0 start-0 w-100 h-100 d-flex align-items-center justify-content-center';
	o.style.zIndex = 2100; o.style.background = 'rgba(0,0,0,.85)';
	o.innerHTML = '<div style="width:min(960px,94vw)"><div class="d-flex justify-content-between align-items-center mb-2 text-white">' +
		'<span class="lib-trailer-name text-truncate"></span><span class="d-flex align-items-center gap-3">' +
		'<a class="link-light small text-nowrap lib-trailer-yt" target="_blank" rel="noopener"><i class="fa-brands fa-youtube me-1"></i>' + _t('Open on YouTube') + '</a>' +
		'<button type="button" class="btn-close btn-close-white" aria-label="' + _t('Close') + '"></button></span></div>' +
		// YouTube refuses embeds without a Referer (error 153); a reverse proxy's
		// Referrer-Policy: no-referrer would strip it, the frame's own policy wins.
		'<div class="ratio ratio-16x9"><iframe referrerpolicy="strict-origin-when-cross-origin" ' +
		'allow="autoplay; encrypted-media; picture-in-picture; fullscreen" allowfullscreen></iframe></div></div>';
	o.querySelector('.lib-trailer-name').textContent = t.name || '';
	o.querySelector('.lib-trailer-yt').href = 'https://www.youtube.com/watch?v=' + encodeURIComponent(t.key);
	o.querySelector('iframe').src = 'https://www.youtube-nocookie.com/embed/' + encodeURIComponent(t.key) +
		'?autoplay=1&rel=0&origin=' + encodeURIComponent(location.origin);
	var close = function () { o.remove(); document.removeEventListener('keydown', esc, true); };
	var esc = function (e) { if (e.key === 'Escape') { e.stopPropagation(); close(); } };
	o.querySelector('.btn-close').addEventListener('click', close);
	o.addEventListener('click', function (e) { if (e.target === o) { close(); } });
	document.addEventListener('keydown', esc, true);
	document.body.appendChild(o);
}

var libHashRe = /^#(movie|series|album|audiobook|book)\/(\d+)$/;

document.getElementById('libModal').addEventListener('hidden.bs.modal', function () {
	if (libHashRe.test(location.hash)) { history.replaceState(null, '', location.pathname + location.search); }
});

window.addEventListener('popstate', function () {
	var m = libHashRe.exec(location.hash);
	if (m) { openLibraryItem(m[1], +m[2], undefined, true); return; }
	var modal = bootstrap.Modal.getInstance(document.getElementById('libModal'));
	if (modal) { modal.hide(); }
});

(function () {
	var m = libHashRe.exec(location.hash);
	if (m) { openLibraryItem(m[1], +m[2], undefined, true); }
})();

// ---- Selecting several entries ----
var libSel = { on: false, items: new Map() };

function libCardClick(el, type, id) {
	if (libSel.on) { libToggleSel(el); return; }
	openLibraryItem(type, id);
}

function libSelectMode(on) {
	libSel.on = on === undefined ? !libSel.on : !!on;
	if (!libSel.on) { libSel.items.clear(); }
	document.getElementById('libGrid').classList.toggle('lib-selecting', libSel.on);
	document.getElementById('libSelBar').classList.toggle('d-none', !libSel.on);
	document.getElementById('libSelectBtn').classList.toggle('active', libSel.on);
	libSelSync();
}

function libSelKey(el) { return el.dataset.libType + ':' + el.dataset.libId; }

function libToggleSel(el) {
	var k = libSelKey(el);
	if (libSel.items.has(k)) { libSel.items.delete(k); } else {
		libSel.items.set(k, { type: el.dataset.libType, id: el.dataset.libId, title: el.dataset.libTitle, search: el.dataset.libSearch, refresh: el.dataset.libRefresh });
	}
	libSelSync();
}

function libSelSync(text) {
	document.querySelectorAll('#libGrid [data-lib-id]').forEach(function (el) { el.classList.toggle('lib-selected', libSel.items.has(libSelKey(el))); });
	document.getElementById('libSelCount').textContent = text || (libSel.items.size + ' selected');
}

function libSelectPage() {
	document.querySelectorAll('#libGrid [data-lib-id]').forEach(function (el) {
		if (!libSel.items.has(libSelKey(el))) { libToggleSel(el); }
	});
}

function libSelectClear() { libSel.items.clear(); libSelSync(); }

// The selection survives paging and filtering.
document.body.addEventListener('htmx:afterSwap', function (e) { if (e.target && e.target.id === 'libGrid') { libSelSync(); } });
// A listener, not hx-on: htmx compiles hx-on with eval, which a CSP may block.
document.body.addEventListener('htmx:afterRequest', function (e) {
	if (e.detail && e.detail.elt && e.detail.elt.classList.contains('lib-pager')) { window.scrollTo({ top: 0, behavior: 'smooth' }); }
});

// libBulk runs an action on every selected entry, one after the other.
function libBulk(action) {
	var items = Array.from(libSel.items.values());
	var mark = action === 'watched' || action === 'unwatched';
	if (!items.length) { libNotify('error', mark ? _t('Nothing selected that can be watched') : _t('Nothing selected')); return; }
	// delete, deletefiles, deleteexcl, deletefilesexcl: with the files on
	// disk and/or "don't add again from lists".
	var del = action.indexOf('delete') === 0, withFiles = action.indexOf('files') > 0, exclude = /excl$/.test(action);
	var what = del ? (withFiles ? _t('Delete WITH their files on disk') : _t('Delete')) :
		{ search: _t('Search by ID and download'), searchtitle: _t('Search by ID and title and download'), refresh: _t('Refresh the metadata of'), watched: _t('Marked as watched'), unwatched: _t('Marked as not watched') }[action];
	var msg = what + ' ' + (items.length === 1 ? _t('1 entry?') : _tf('%d entries?', items.length)) +
		(del ? (withFiles ? _t(' The files cannot be restored.') : _t(' Files on disk are kept.')) : '') +
		(exclude ? _t(' The lists of their media groups won\'t add them again.') : '');
	// Marking only changes the account's own history: no confirmation.
	(mark ? function (f) { f(); } : function (f) { confirmAction(_t('Please confirm'), msg, f); })(function () {
		var ok = 0, failed = 0, i = 0;
		function one(it) {
			if (action === 'search' || action === 'searchtitle') {
				if (!it.search) { return Promise.reject(new Error(_t('not searched'))); }
				// Series: only the missing episodes the scheduled search would take.
				return gmdApiProxyFetch(it.search + '?download=true' + (it.type === 'series' ? '&missing=true' : '') + '&searchByTitle=' + (action === 'searchtitle')).then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
			}
			if (action === 'refresh') {
				return gmdApiProxyFetch(it.refresh).then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
			}
			if (mark) {
				var wf = new URLSearchParams({ type: it.type, id: it.id, watched: action === 'watched' ? '1' : '0', csrf_token: libCsrf });
				return fetch('/api/admin/library/watched-entry', { method: 'POST', credentials: 'same-origin', body: wf, headers: { 'X-CSRF-Token': libCsrf } })
					.then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
			}
			var form = new URLSearchParams({ type: it.type, id: it.id, files: withFiles ? '1' : '0', exclude: exclude ? '1' : '0', csrf_token: libCsrf });
			return fetch('/api/admin/library/delete', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
				.then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } });
		}
		(function next() {
			if (i >= items.length) {
				libNotify(failed ? 'error' : 'success', what + ': ' + _tf('%d done', ok) + (failed ? _tf(', %d failed', failed) : ''));
				if (del) { libSel.items.clear(); libReloadGrid(); }
				if (mark) { libReloadGrid(); }
				libSelSync();
				return;
			}
			var it = items[i++];
			libSelSync(_tf('Working %d of %d…', i, items.length));
			one(it).then(function () { ok++; }, function () { failed++; }).then(next);
		})();
	});
}

// libEdit loads the edit forms of a list entry row and its metadata row
// (the database browser's forms) into the Edit tab.
function libEdit(entryTable, entryID, dbTable, dbID, label) {
	var box = document.getElementById('libEditForms');
	if (!box) { return; }
	// Mark the tab as loaded first: showing it below must not replace these
	// forms with the entry's own (an episode's pencil opens the tab too).
	box.dataset.loaded = '1';
	box.innerHTML = '';
	var sections = [[dbTable, dbID, _t('Metadata'), 'fa-database'], [entryTable, entryID, _t('List entry'), 'fa-list']];
	var nav = document.createElement('div');
	nav.className = 'd-flex flex-wrap gap-2 align-items-center mb-3 small';
	nav.innerHTML = '<span class="text-muted">' + (label ? _t('Editing ') : _t('Jump to')) + '</span>';
	if (label) { var b = document.createElement('strong'); b.textContent = label; nav.appendChild(b); nav.appendChild(document.createTextNode(' ·')); }
	box.appendChild(nav);
	if (!label && box.dataset.episodes === '1') {
		var hint = document.createElement('p');
		hint.className = 'small text-muted';
		hint.innerHTML = 'Episodes: use the <i class="fa-solid fa-pen"></i> button of an episode in the Episodes tab.';
		box.appendChild(hint);
	}
	sections.forEach(function (p, n) {
		var id = 'libEditSec' + n;
		var a = document.createElement('a');
		a.href = '#' + id; a.className = 'link-primary'; a.textContent = p[2] + ' (' + p[0] + ')';
		a.addEventListener('click', function (e) { e.preventDefault(); document.getElementById(id).scrollIntoView({ behavior: 'smooth', block: 'start' }); });
		nav.appendChild(a);
		var sec = document.createElement('div');
		sec.className = 'lib-edit-section';
		sec.id = id;
		sec.innerHTML = '<div class="lib-edit-head"><i class="fa-solid ' + p[3] + ' me-2 text-primary"></i><span></span></div><div class="text-muted small">' + _t('Loading…') + '</div>';
		sec.querySelector('.lib-edit-head span').textContent = p[2] + ' · ' + p[0];
		box.appendChild(sec);
		fetch('/api/admin/tableedit/' + encodeURIComponent(p[0]) + '/' + p[1], { credentials: 'same-origin' })
			.then(function (r) { return r.text(); })
			.then(function (html) {
				sec.querySelector('div.text-muted').outerHTML = html;
				// The lookup selects (e.g. the metadata row an entry points to)
				// load their options like on the database page.
				if (typeof window.initChoicesGlobal === 'function') { window.initChoicesGlobal(); }
			});
	});
	var tab = document.querySelector('[data-bs-target="#libTabEdit"]');
	if (tab) { bootstrap.Tab.getOrCreateInstance(tab).show(); }
}

// Opening the Edit tab loads the entry's forms once.
document.addEventListener('shown.bs.tab', function (e) {
	if (e.target.getAttribute('data-bs-target') !== '#libTabEdit') { return; }
	var box = document.getElementById('libEditForms');
	if (box && !box.dataset.loaded) {
		box.dataset.loaded = '1';
		libEdit(box.dataset.entryTable, box.dataset.entryId, box.dataset.dbTable, box.dataset.dbId, '');
	}
});

// Edit forms inside the dialog save without leaving the page.
document.addEventListener('submit', function (e) {
	var form = e.target;
	if (!form.closest || !form.closest('#libModal')) { return; }
	e.preventDefault();
	var data = new FormData(form);
	// A lookup select still waiting for (or failing) its option list must not
	// blank the reference it holds.
	form.querySelectorAll('select.choices-ajax[data-selected-value]').forEach(function (sel) {
		if (sel.name && !sel.value && sel.dataset.selectedValue) { data.set(sel.name, sel.dataset.selectedValue); }
	});
	// URL-encoded: the update endpoint doesn't parse multipart bodies.
	fetch(form.action, { method: 'POST', credentials: 'same-origin', body: new URLSearchParams(data), headers: { 'X-CSRF-Token': libCsrf } })
		.then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { if (!r.ok || d.success === false) { throw new Error(d.error || ('HTTP ' + r.status)); } }); })
		.then(function () { libNotify('success', _t('Saved')); libReloadGrid(); })
		.catch(function (err) { libNotify('error', _t('Saving failed: ') + err.message); });
});

// libRandom opens a random entry of the current group, search and filter.
function libRandom() {
	var params = new URLSearchParams(new FormData(document.getElementById('libForm')));
	fetch('/api/admin/library/random?' + params.toString(), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) { openLibraryItem(d.type, d.id); })
		.catch(function (e) { libNotify('error', e.message); });
}

// libSetCoverSize sets the width of the covers (px) in the cover views: more
// or fewer per row. Remembered in the browser.
function libSetCoverSize(px, keep) {
	var grid = document.getElementById('libGrid'), r = document.getElementById('libCoverSize');
	if (!grid || !r) { return; }
	grid.style.setProperty('--lib-size', px + 'px');
	r.value = px;
	r.style.setProperty('--v', Math.round((px - r.min) / (r.max - r.min) * 100) + '%');
	if (!keep) { lpStore('lib-cover-size', String(px)); }
}
(function () {
	var px = parseInt(lpStore('lib-cover-size'), 10);
	if (px >= 90 && px <= 320) { libSetCoverSize(px, true); } else if (document.getElementById('libCoverSize')) {
		var r = document.getElementById('libCoverSize');
		r.style.setProperty('--v', Math.round((150 - r.min) / (r.max - r.min) * 100) + '%');
	}
})();

// libPlayAll plays everything the search and filters find, all pages
// (shuffled: in random order across all files).
function libPlayAll(shuffle) {
	var params = new URLSearchParams(new FormData(document.getElementById('libForm')));
	if (shuffle) { params.set('shuffle', '1'); }
	fetch('/api/admin/library/playall?' + params.toString(), { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			if (!d.list || !d.list.length) { throw new Error(_t('nothing to play')); }
			if (d.capped) { libNotify('info', _tf('Playing the first %d files.', d.list.length)); }
			lpPlayList(d.list.map(function (x) { return { kind: x.kind, id: String(x.id), label: x.label }; }), 0, d.entry, '');
		})
		.catch(function (e) { libNotify('error', _t('Can\'t play: ') + e.message); });
}

// ---- Saved views ----
// A saved view is the form (group, search, filters, sort, view, page size)
// under a name, kept in this browser. Opening one loads the page with it.
function libSavedViews() {
	try { return JSON.parse(lpStore('lib-saved-views') || '[]') || []; } catch (e) { return []; }
}

function libSavedRender() {
	var menu = document.getElementById('libSavedMenu');
	if (!menu) { return; }
	var views = libSavedViews(), esc = function (t) { var d = document.createElement('div'); d.textContent = t; return d.innerHTML; };
	var html = views.map(function (v, i) {
		return '<li class="d-flex align-items-center"><a class="dropdown-item text-truncate" href="/api/admin/library?' + esc(v.params) + '" title="' + esc(v.name) + '">' + esc(v.name) + '</a>' +
			'<button type="button" class="btn btn-sm btn-link text-danger px-2" title="' + _t('Delete this saved view') + '" onclick="libDeleteSaved(' + i + ', event)"><i class="fa-solid fa-xmark"></i></button></li>';
	}).join('');
	if (!views.length) { html = '<li><span class="dropdown-item-text small text-muted">' + _t('No saved views yet.') + '</span></li>'; }
	menu.innerHTML = html + '<li><hr class="dropdown-divider"></li>' +
		'<li><a class="dropdown-item" href="#" onclick="libSaveView(); return false;"><i class="fa-solid fa-plus me-2"></i>' + _t('Save current view…') + '</a></li>';
}

function libSaveView() {
	var form = document.getElementById('libForm');
	var params = new URLSearchParams(new FormData(form));
	params.delete('seed');
	var name = prompt(_t('Name for this view (search and filters):'), params.get('q') || params.get('group') || '');
	if (!name || !(name = name.trim())) { return; }
	var views = libSavedViews().filter(function (v) { return v.name !== name; });
	views.push({ name: name, params: params.toString() });
	views.sort(function (a, b) { return a.name.localeCompare(b.name); });
	lpStore('lib-saved-views', JSON.stringify(views));
	libSavedRender();
	libNotify('success', _t('Saved "') + name + '".');
}

function libDeleteSaved(i, e) {
	if (e) { e.preventDefault(); e.stopPropagation(); }
	var views = libSavedViews();
	views.splice(i, 1);
	lpStore('lib-saved-views', JSON.stringify(views));
	libSavedRender();
}
libSavedRender();

// libSetPageSize shows size entries per page ("all" for all), from page 1.
// Remembered in the browser; a page opened without a size uses it.
function libSetPageSize(size, keep) {
	var f = document.getElementById('libSize');
	if (!f) { return; }
	f.value = size;
	if (!keep) { lpStore('lib-page-size', size); }
	htmx.trigger(document.getElementById('libForm'), 'change');
}
(function () {
	var size = lpStore('lib-page-size'), f = document.getElementById('libSize');
	if (!f || !size || size === f.value || /[?&]size=/.test(location.search)) { return; }
	// htmx sets the form up once the page has loaded.
	var apply = function () { libSetPageSize(size, true); };
	if (document.readyState === 'loading') { document.addEventListener('DOMContentLoaded', apply); } else { apply(); }
})();

// The grid's search, filters, order, view and page go into the address
// after every grid request (the form, the pager, a reload), so reloading the
// page or a bookmark shows the same; empty and zero values are left out.
document.body.addEventListener('htmx:afterRequest', function (e) {
	var p = e.detail && e.detail.pathInfo && e.detail.pathInfo.finalRequestPath;
	if (!e.detail.successful || !p || p.indexOf('/api/admin/library/grid') !== 0) { return; }
	var params = new URLSearchParams(p.split('?')[1] || ''), keep = new URLSearchParams();
	params.forEach(function (v, k) { if (v !== '' && v !== '0') { keep.append(k, v); } });
	var url = location.pathname + (keep.toString() ? '?' + keep.toString() : '') + location.hash;
	if (url !== location.pathname + location.search + location.hash) { history.replaceState(history.state, '', url); }
});

// libToggleSortDir reverses the grid's sort; random has no direction.
function libToggleSortDir() {
	var rev = document.getElementById('libRev');
	rev.value = rev.value === '1' ? '' : '1';
	libShowSortDir();
	htmx.trigger(document.getElementById('libForm'), 'change');
}

function libShowSortDir() {
	var form = document.getElementById('libForm'), btn = document.getElementById('libSortDir');
	var rev = document.getElementById('libRev').value === '1';
	btn.disabled = form.querySelector('[name="sort"]').value === 'random';
	btn.innerHTML = '<i class="fa-solid ' + (rev ? 'fa-arrow-up-short-wide' : 'fa-arrow-down-wide-short') + '"></i>';
}

// ---- Lists of the dialog (episodes, tracks, files) ----
// The rows come in their natural order (data-i; episodes newest first). They
// sort by it, by title or at random in the browser, and the search box hides
// the rows whose texts (data-q) lack one of its words, the watched filter
// those in another state (data-ws). The playlist (Play)
// takes the shown rows in the shown order, except that the natural order
// always plays forwards (episodes oldest first).
function libShuffle(a) {
	for (var i = a.length - 1; i > 0; i--) { var j = Math.floor(Math.random() * (i + 1)), t = a[i]; a[i] = a[j]; a[j] = t; }
	return a;
}

function libSortList(el) {
	var pane = el.closest('.tab-pane'), tb = pane.querySelector('tbody'), tools = pane.querySelector('.lib-list-tools');
	var by = tools && tools.querySelector('.lib-list-sort');
	if (!tb || !by) { return; }
	var dirBtn = tools.querySelector('.lib-list-dir'), rev = dirBtn.dataset.rev === '1', natural = by.selectedIndex === 0;
	var rows = [].slice.call(tb.rows), n = rows.length;
	dirBtn.disabled = by.value === 'random';
	dirBtn.innerHTML = '<i class="fa-solid ' + (rev ? 'fa-arrow-up-short-wide' : 'fa-arrow-down-wide-short') + '"></i>';
	if (by.value === 'random') {
		libShuffle(rows);
	} else {
		rows.sort(function (a, b) {
			var c = natural ? a.dataset.i - b.dataset.i
				: (a.dataset.title || '').localeCompare(b.dataset.title || '', undefined, { numeric: true, sensitivity: 'base' }) || a.dataset.i - b.dataset.i;
			return rev ? -c : c;
		});
	}
	rows.forEach(function (r, i) {
		tb.appendChild(r);
		var b = r.querySelector('[data-play-list]');
		if (b) { b.dataset.playOrder = !natural ? i : tools.dataset.playRev === '1' ? n - r.dataset.i : r.dataset.i; }
	});
}

function libToggleListDir(btn) {
	btn.dataset.rev = btn.dataset.rev === '1' ? '' : '1';
	libSortList(btn);
}

function libFilterList(el) {
	var pane = el.closest('.tab-pane'), q = pane.querySelector('.lib-list-q'), ws = pane.querySelector('.lib-list-ws');
	var words = (q ? q.value : '').toLowerCase().split(/\s+/).filter(Boolean), state = ws ? ws.value : '', shown = 0;
	pane.querySelectorAll('tbody tr').forEach(function (r) {
		var text = r.dataset.q || r.textContent.toLowerCase();
		var ok = words.every(function (w) { return text.indexOf(w) >= 0; }) && (!state || r.dataset.ws === state);
		r.classList.toggle('d-none', !ok);
		if (ok) { shown++; }
	});
	var none = pane.querySelector('.lib-list-none');
	if (none) { none.classList.toggle('d-none', shown > 0); }
}

// libPlayRandom plays a list (the downloaded episodes of a series, the tracks
// of an album) in shuffled order.
function libPlayRandom(list) {
	var btns = libShuffle(libPlayButtons(list));
	if (!btns.length) { return; }
	var cover = document.querySelector('#libModalBody .lib-detail-cover');
	lpPlayList(btns.map(function (b) { return { kind: b.dataset.playKind, id: b.dataset.playId, label: b.dataset.playLabel }; }), 0,
		document.getElementById('libModalTitle').textContent, cover ? cover.getAttribute('src') : '');
}

// libSwitchGroup shows another media group without leaving the page.
function libSwitchGroup(e, a) {
	if (e.ctrlKey || e.metaKey || e.shiftKey || e.button === 1) { return true; }
	var form = document.getElementById('libForm');
	form.querySelector('[name="group"]').value = a.dataset.group;
	form.querySelector('[name="q"]').value = '';
	document.querySelectorAll('[data-group]').forEach(function (x) { x.classList.toggle('active', x === a); });
	// The episode view belongs to series groups.
	var series = a.dataset.kind === 'series', epView = document.getElementById('libViewepisodes');
	document.querySelector('label[for="libViewepisodes"]').classList.toggle('d-none', !series);
	if (!series && epView.checked) { document.getElementById('libViewgrid').checked = true; }
	var music = a.dataset.kind === 'album', arView = document.getElementById('libViewartists');
	document.querySelector('label[for="libViewartists"]').classList.toggle('d-none', !music);
	if (!music && arView.checked) { document.getElementById('libViewgrid').checked = true; }
	var movies = a.dataset.kind === 'movie', coView = document.getElementById('libViewcollections');
	document.querySelector('label[for="libViewcollections"]').classList.toggle('d-none', !movies);
	if (!movies && coView.checked) { document.getElementById('libViewgrid').checked = true; }
	document.getElementById('libArtist').value = '';
	document.getElementById('libColl').value = '0';
	document.getElementById('libCast').value = '';
	history.replaceState(null, '', a.href);
	libSelectMode(false);
	// The list, genre and resolution filters belong to the group.
	fetch('/api/admin/library/filters?group=' + encodeURIComponent(a.dataset.group), { credentials: 'same-origin' })
		.then(function (r) { return r.text(); })
		.catch(function () { return ''; })
		.then(function (html) {
			var row = document.getElementById('libFilters2');
			if (row && html) { row.outerHTML = html; }
			htmx.trigger(form, 'change');
		});
	return false;
}


// libShowArtist shows the albums of an artist ('' : all albums again).
function libShowArtist(id) {
	document.getElementById('libArtist').value = id;
	if (id !== '') { document.getElementById('libViewgrid').checked = true; }
	htmx.trigger(document.getElementById('libForm'), 'change');
}

// libShowCollection shows the movies of a TMDB collection, oldest first
// (sort year reversed; 0: all movies again); from the dialog it closes it.
function libShowCollection(id) {
	var form = document.getElementById('libForm');
	document.getElementById('libColl').value = id;
	if (id) {
		document.getElementById('libViewgrid').checked = true;
		var sort = form.querySelector('[name="sort"]'), rev = document.getElementById('libRev');
		if (sort) { sort.value = 'year'; }
		if (rev) { rev.value = '1'; }
	}
	var m = document.querySelector('.modal.show');
	if (m) { bootstrap.Modal.getOrCreateInstance(m).hide(); }
	htmx.trigger(form, 'change');
}

// The Collections view lists all collections again.
document.addEventListener('change', function (e) {
	if (e.target && e.target.id === 'libViewcollections') { document.getElementById('libColl').value = '0'; }
}, true);

// libShowCast filters the grid by a cast member of the open item (kind:
// its kind, the grid must show that kind; '' name: all again).
function libShowCast(kind, name) {
	var form = document.getElementById('libForm'), input = document.getElementById('libCast');
	if (!form || !input) {
		location.href = '/api/admin/library?cast=' + encodeURIComponent(name);
		return;
	}
	var group = document.querySelector('[data-group].active');
	if (name && group && group.dataset.kind && group.dataset.kind !== kind) {
		libNotify('info', _t('Pick a library of this kind to filter by cast.'));
		return;
	}
	input.value = name;
	if (name) { document.getElementById('libViewgrid').checked = true; }
	var m = document.querySelector('.modal.show');
	if (m) { bootstrap.Modal.getOrCreateInstance(m).hide(); }
	htmx.trigger(form, 'change');
}

// libShowGenre filters the grid by a genre of the open item (when the grid
// shows that kind of media).
function libShowGenre(kind, genre) {
	var form = document.getElementById('libForm'), sel = form && form.querySelector('select[name="genre"]');
	var group = document.querySelector('[data-group].active');
	if (!sel || (group && group.dataset.kind && group.dataset.kind !== kind)) {
		libNotify('info', _t('Pick a library of this kind to filter by genre.'));
		return;
	}
	var opt = [].slice.call(sel.options).filter(function (o) { return o.value.toLowerCase() === genre.toLowerCase(); })[0];
	if (!opt) { opt = new Option(genre, genre); sel.add(opt); }
	sel.value = opt.value;
	document.getElementById('libViewgrid').checked = true;
	var m = document.querySelector('.modal.show');
	if (m) { bootstrap.Modal.getOrCreateInstance(m).hide(); }
	htmx.trigger(form, 'change');
}

function libAction(kind, url) {
	var label = kind === 'refresh' ? _t('Refresh the metadata?') : _t('Search and download the best accepted release?');
	confirmAction(_t('Please confirm'), label, function () {
		gmdApiProxyFetch(url)
			.then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { if (!r.ok) { throw new Error((d && d.error) || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				if (d && d.accepted) { libNotify('success', _tf('Search done: %d accepted, %d denied', d.accepted.length, d.denied ? d.denied.length : 0) + (d.accepted.length ? _t(' - the best one is downloading') : '')); }
				else { libNotify('success', kind === 'refresh' ? _t('Metadata refresh started') : _t('Search started')); }
			})
			.catch(function (e) { libNotify('error', e.message); });
	});
}

function libItemFilter(filter) { if (libCurrent) { openLibraryItem(libCurrent.type, libCurrent.id, filter); } }

// libDeletePart deletes one episode or file row; the row leaves the table so
// a running player keeps playing.
function libDeletePart(part, type, id, withFiles, btn) {
	var msg = withFiles
		? (part === 'episode' ? _t('Delete this episode AND its files on disk?') : _t('Delete this file on disk?')) + _t(' The files cannot be restored.')
		: (part === 'episode' ? _t('Remove this episode from the list?') : _t('Remove this file from the list?')) + _t(' Files on disk are kept.');
	confirmAction(_t('Please confirm'), msg, function () {
		var form = new URLSearchParams({ part: part, type: type, id: id, files: withFiles ? '1' : '0', csrf_token: libCsrf });
		fetch('/api/admin/library/delete-part', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				var row = btn && btn.closest('tr');
				if (row) { row.remove(); }
				libNotify('success', withFiles ? (d.files === 1 ? _t('1 file deleted') : _tf('%d files deleted', d.files)) : _t('Removed from the list'));
				libReloadGrid();
			})
			.catch(function (e) { libNotify('error', e.message); });
	});
}

function libDelete(type, id, withFiles, exclude) {
	var msg = (withFiles ? _t('Delete this entry AND its files on disk? The files cannot be restored.') : _t('Delete this entry from the list? Files on disk are kept.')) +
		(exclude ? _t(' The lists of this media group won\'t add it again.') : '');
	confirmAction(_t('Please confirm'), msg, function () {
		var form = new URLSearchParams({ type: type, id: id, files: withFiles ? '1' : '0', exclude: exclude ? '1' : '0', csrf_token: libCsrf });
		fetch('/api/admin/library/delete', { method: 'POST', credentials: 'same-origin', body: form, headers: { 'X-CSRF-Token': libCsrf } })
			.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
			.then(function (d) {
				bootstrap.Modal.getOrCreateInstance(document.getElementById('libModal')).hide();
				libNotify('success', withFiles ? _tf('Entry and %d files deleted', d.files) : _t('Entry deleted'));
				libReloadGrid();
			})
			.catch(function (e) { libNotify('error', e.message); });
	});
}
`

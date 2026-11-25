package api

import (
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// calendarModalsAndScripts returns the modal HTML and JavaScript for calendar functionality.
func calendarModalsAndScripts() gomponents.Node {
	return gomponents.Group([]gomponents.Node{
		// Event Details Modal
		html.Div(
			html.ID("eventDetailsModal"),
			html.Class("modal fade"),
			// No focus trap: the player bar below must keep the focus.
			html.Data("bs-focus", "false"),
			html.TabIndex("-1"),
			gomponents.Attr("aria-labelledby", "eventDetailsModalLabel"),
			gomponents.Attr("aria-hidden", "true"),
			html.Div(
				html.Class("modal-dialog modal-lg modal-dialog-centered modal-dialog-scrollable"),
				html.Div(
					html.Class("modal-content border-0 shadow"),
					html.Div(
						html.Class("modal-header align-items-start"),
						html.Div(
							html.H5(
								html.ID("eventDetailsModalLabel"),
								html.Class("modal-title"),
								gomponents.Text(t("Event Details")),
							),
							html.Div(html.ID("eventDetailsSubtitle"), html.Class("small text-muted mt-1")),
						),
						html.Button(
							html.Type("button"),
							html.Class("btn-close"),
							html.Data("bs-dismiss", "modal"),
							gomponents.Attr("aria-label", t("Close")),
						),
					),
					html.Div(
						html.ID("eventDetailsBody"),
						html.Class("modal-body pb-4"),
					),
				),
			),
		),

		// More Events Modal
		html.Div(
			html.ID("moreEventsModal"),
			html.Class("modal fade"),
			html.Data("bs-focus", "false"),
			html.TabIndex("-1"),
			gomponents.Attr("aria-labelledby", "moreEventsModalLabel"),
			gomponents.Attr("aria-hidden", "true"),
			html.Div(
				html.Class("modal-dialog"),
				html.Div(
					html.Class("modal-content"),
					html.Div(
						html.Class("modal-header"),
						html.H5(
							html.ID("moreEventsModalLabel"),
							html.Class("modal-title"),
							gomponents.Text(t("All Events")),
						),
						html.Button(
							html.Type("button"),
							html.Class("btn-close"),
							html.Data("bs-dismiss", "modal"),
							gomponents.Attr("aria-label", t("Close")),
						),
					),
					html.Div(
						html.ID("moreEventsBody"),
						html.Class("modal-body"),
					),
					html.Div(
						html.Class("modal-footer"),
						html.Button(
							html.Type("button"),
							html.Class("btn btn-secondary"),
							html.Data("bs-dismiss", "modal"),
							gomponents.Text(t("Close")),
						),
					),
				),
			),
		),

		// JavaScript for modal functionality
		html.Script(gomponents.Raw(`
// Poster/cover size in the details dialog.
(function () {
	var st = document.createElement('style');
	st.textContent = '.calendar-poster{width:150px;height:225px;object-fit:cover;align-self:flex-start;background:#f1f3f5}' +
		'.calendar-frame{width:100%;max-width:480px;aspect-ratio:16/9;object-fit:cover;background:#f1f3f5}' +
		'.calendar-thumb{width:96px;height:54px;object-fit:cover;background:#e9ecef}' +
		'.calendar-group-items{display:none}.calendar-group.open .calendar-group-items{display:block}' +
		'@media (max-width:575px){.calendar-poster{width:120px;height:180px}}';
	document.head.appendChild(st);
})();

// Dialogs opened from another dialog (an event from the "more" list, the
// search confirmation) go on top of it, with their backdrop between them.
document.addEventListener('show.bs.modal', function (e) {
	var open = document.querySelectorAll('.modal.show').length;
	if (!open) { return; }
	e.target.style.zIndex = 1055 + 20 * open;
	setTimeout(function () {
		var backdrops = document.querySelectorAll('.modal-backdrop');
		var bd = backdrops[backdrops.length - 1];
		if (bd) { bd.style.zIndex = 1050 + 20 * open; }
	}, 0);
});
document.addEventListener('hidden.bs.modal', function (e) {
	e.target.style.zIndex = '';
	// The dialog still open below keeps the page from scrolling behind it.
	if (document.querySelector('.modal.show')) { document.body.classList.add('modal-open'); }
});

function escapeCalendarHtml(value) {
	return String(value == null ? '' : value)
		.replace(/&/g, '&amp;')
		.replace(/</g, '&lt;')
		.replace(/>/g, '&gt;')
		.replace(/"/g, '&quot;')
		.replace(/'/g, '&#39;');
}

var calendarTypes = {
	movie: { label: _t('Movie'), icon: 'fa-film' },
	series: { label: _t('Episode'), icon: 'fa-tv' },
	album: { label: _t('Album'), icon: 'fa-compact-disc' },
	audiobook: { label: _t('Audiobook'), icon: 'fa-headphones' },
	book: { label: _t('Book'), icon: 'fa-book' }
};

var calendarStatusClass = { Downloaded: 'bg-success', 'Partly downloaded': 'bg-info text-dark', Monitored: 'bg-warning text-dark', Unmonitored: 'bg-secondary' };
// The events carry the status in English (the key above); this is what is shown.
var calendarStatusText = { Downloaded: _t('Downloaded'), 'Partly downloaded': _t('Partly downloaded'), Monitored: _t('Monitored'), Unmonitored: _t('Unmonitored') };

// showEventDetails opens the details dialog for an event object (the JSON of
// a calendarEvent). All text is escaped before it goes into innerHTML.
function showEventDetails(ev) {
	var esc = escapeCalendarHtml;
	var type = calendarTypes[ev.type] || { label: ev.type, icon: 'fa-calendar' };

	// Release dates are calendar days: build them as local dates, not UTC.
	var d = /^(\d{4})-(\d{2})-(\d{2})/.exec(ev.date || '');
	var date = d ? new Date(+d[1], +d[2] - 1, +d[3]) : null;
	var when = '';
	if (date) {
		var today = new Date(); today.setHours(0, 0, 0, 0);
		var days = Math.round((date - today) / 86400000);
		when = days === 0 ? _t('today') : days === 1 ? _t('tomorrow') : days === -1 ? _t('yesterday') :
			days > 0 ? _tf('in %s days', days) : _tf('%s days ago', Math.abs(days));
	}

	var episode = '';
	if (ev.type === 'series' && +ev.season > 0 && ev.episode > 0) {
		episode = 'S' + String(ev.season).padStart(2, '0') + 'E' + String(ev.episode).padStart(2, '0');
	}

	document.getElementById('eventDetailsModalLabel').innerHTML =
		'<i class="fa-solid ' + type.icon + ' me-2 text-primary"></i>' + esc(ev.show || ev.title);
	document.getElementById('eventDetailsSubtitle').textContent =
		(date ? date.toLocaleDateString(document.documentElement.lang || undefined, { weekday: 'long', year: 'numeric', month: 'long', day: 'numeric' }) : '') +
		(when ? ' · ' + when : '');

	var badges = '<span class="badge bg-primary me-1"><i class="fa-solid ' + type.icon + ' me-1"></i>' + esc(type.label) + '</span>' +
		'<span class="badge ' + (calendarStatusClass[ev.status] || 'bg-secondary') + ' me-1">' + esc(calendarStatusText[ev.status] || ev.status) + '</span>' +
		(ev.overdue ? '<span class="badge bg-danger me-1">' + _t('Overdue') + '</span>' : '') +
		(ev.new ? '<span class="badge bg-info me-1">' + _t('New since yesterday') + '</span>' : '') +
		(episode ? '<span class="badge bg-light text-dark border me-1">' + episode + '</span>' : '');

	var facts = [];
	// network holds the author of books and audiobooks and the artist of albums.
	var whoIcon = { book: 'fa-user-pen', audiobook: 'fa-user-pen', album: 'fa-microphone-lines' }[ev.type] || 'fa-tower-broadcast';
	if (ev.network) { facts.push('<i class="fa-solid ' + whoIcon + ' me-1"></i>' + esc(ev.network)); }
	if (ev.imdbRating > 0) { facts.push('<i class="fa-solid fa-star text-warning me-1"></i>' + Number(ev.imdbRating).toFixed(1)); }
	if (ev.runtime > 0) { facts.push('<i class="fa-regular fa-clock me-1"></i>' + ev.runtime + ' min'); }
	if (ev.year > 0 && ev.type !== 'series') { facts.push('<i class="fa-regular fa-calendar me-1"></i>' + ev.year); }

	var lists = (ev.listname || '').split(', ').filter(Boolean).map(function (l) {
		return '<span class="badge rounded-pill bg-secondary bg-opacity-10 text-secondary border me-1">' + esc(l) + '</span>';
	}).join('');

	var links = [];
	function link(url, label, icon) {
		links.push('<a class="btn btn-sm btn-outline-secondary" href="' + url + '" target="_blank" rel="noopener"><i class="' + icon + ' me-1"></i>' + label + '</a>');
	}
	// The links are built on the server, shared with the Library.
	(ev.links || []).forEach(function (l) { link(esc(l.url), esc(l.label), esc(l.icon)); });

	// Each list entry (one per media group) has its own file, Library entry
	// and search; several entries get one row each.
	var entries = ev.entries && ev.entries.length ? ev.entries :
		[{ id: ev.id, list: ev.listname, downloaded: ev.downloaded, monitored: ev.monitored, libraryUrl: ev.libraryUrl }];
	var actions = '', groupRows = '';
	if (entries.length === 1) {
		actions = calendarEntryActions(ev.type, entries[0], false);
	} else {
		groupRows = '<div class="list-group mb-3">' + entries.map(function (en) {
			var st = en.downloaded ? 'Downloaded' : en.monitored ? 'Monitored' : 'Unmonitored';
			return '<div class="list-group-item d-flex flex-wrap align-items-center gap-2">' +
				'<span class="fw-semibold me-1">' + esc(en.list || en.group || '') + '</span>' +
				'<span class="badge ' + calendarStatusClass[st] + '">' + calendarStatusText[st] + '</span>' +
				'<span class="flex-grow-1"></span>' + calendarEntryActions(ev.type, en, true) + '</div>';
		}).join('') + '</div>';
	}

	var poster = ev.posterUrl
		? '<img src="' + esc(ev.posterUrl) + '" alt="" class="rounded shadow-sm flex-shrink-0 calendar-poster" onerror="this.remove()">'
		: '';

	var html = '<div class="d-flex flex-column flex-sm-row gap-4">' + poster +
		'<div class="flex-grow-1" style="min-width:0">' +
		(ev.episodeTitle ? '<h5 class="mb-2">' + esc(ev.episodeTitle) + '</h5>' : '') +
		'<div class="mb-2">' + badges + '</div>' +
		(facts.length ? '<div class="d-flex flex-wrap gap-3 text-muted small mb-2">' + facts.map(function (f) { return '<span>' + f + '</span>'; }).join('') + '</div>' : '') +
		(groupRows || (lists ? '<div class="mb-3">' + lists + '</div>' : '')) +
		(ev.thumbUrl ? '<img src="' + esc(ev.thumbUrl) + '" alt="" class="rounded mb-3 d-block calendar-frame">' : '') +
		(ev.overview ? '<p class="mb-0" style="white-space:pre-line">' + esc(ev.overview) + '</p>' : '<p class="text-muted fst-italic mb-0">' + _t('No description available.') + '</p>') +
		'</div></div>';

	if (links.length || actions) {
		html += '<div class="d-flex flex-wrap gap-2 mt-4 pt-3 border-top">' + actions + (actions && links.length ? '<span class="flex-grow-1"></span>' : '') + links.join('') + '</div>';
	}

	document.getElementById('eventDetailsBody').innerHTML = html;

	if (typeof bootstrap !== 'undefined') {
		bootstrap.Modal.getOrCreateInstance(document.getElementById('eventDetailsModal')).show();
	}
}

// calendarEntryActions are Play / Open in Library / Search of one list entry;
// short gives icon-only search buttons for the per-group rows.
function calendarEntryActions(type, en, short) {
	var esc = escapeCalendarHtml, a = '';
	if (en.downloaded && en.id > 0 && type !== 'book') {
		a += '<button class="btn btn-sm btn-success" onclick="calendarPlay(\'' + type + '\', ' + en.id + ')"><i class="fa-solid fa-play me-1"></i>' + _t('Play') + '</button>';
	}
	if (en.libraryUrl) {
		a += '<a class="btn btn-sm btn-outline-primary" href="' + esc(en.libraryUrl) + '"><i class="fa-solid fa-photo-film me-1"></i>' + (short ? _t('Library') : _t('Open in Library')) + '</a>';
	}
	// Held back from searching (not monitored: dont_search): no search.
	if (en.downloaded || !(en.id > 0) || !en.monitored || !window.calendarCanSearch) { return a; }
	function search(byTitle, cls, icon, label) {
		a += '<button class="btn btn-sm ' + cls + '" title="' + label + '" onclick="searchCalendarEvent(' + en.id + ', \'' + type + '\', ' + byTitle + ')"><i class="fa-solid ' + icon + ' me-1"></i>' + (short ? label.replace(_t('Search by '), '') : label) + '</button>';
	}
	if (calendarSearchTypes[type]) {
		// Albums, audiobooks and books are always searched by artist/author and title.
		search(false, 'btn-primary', 'fa-magnifying-glass', _t('Search'));
	} else if (type === 'movie' || type === 'series') {
		search(false, 'btn-primary', 'fa-magnifying-glass', _t('Search by ') + (type === 'movie' ? 'IMDb ID' : 'TVDB ID'));
		search(true, 'btn-outline-primary', 'fa-magnifying-glass-plus', _t('Search by title'));
	}
	return a;
}

// Search endpoints of the list entries of albums, audiobooks and books.
var calendarSearchTypes = {
	album: { path: '/api/music/search/list/', noun: 'album' },
	audiobook: { path: '/api/audiobooks/search/list/', noun: 'audiobook' },
	book: { path: '/api/books/search/list/', noun: 'book' }
};

function searchCalendarEvent(id, type, searchByTitle) {
	var url;
	var confirmMsg;
	if (calendarSearchTypes[type]) {
		url = calendarSearchTypes[type].path + id + '?download=true';
		confirmMsg = _t('Start search for this ') + calendarSearchTypes[type].noun + '?';
	} else if (type === 'movie') {
		url = '/api/movies/search/list/' + id + '?searchByTitle=' + searchByTitle + '&download=true';
		confirmMsg = searchByTitle ? _t('Start search for this movie by Title?') : _t('Start search for this movie by IMDB ID?');
	} else {
		url = '/api/series/episodes/search/list/' + id + '?searchByTitle=' + searchByTitle + '&download=true';
		confirmMsg = searchByTitle ? _t('Start search for this episode by Title?') : _t('Start search for this episode by TVDB ID?');
	}

	confirmAction(_t('Please confirm'), confirmMsg, function() {
		gmdApiProxyFetch(url)
		.then(response => response.json().catch(() => ({})).then(data => {
			if (!response.ok) {
				throw new Error((data && data.error) || ('HTTP ' + response.status));
			}
			return data;
		}))
		.then(data => {
			var msg = _t('Search completed!\n');
			msg += _t('Accepted: ') + (data.accepted ? data.accepted.length : 0) + '\n';
			msg += _t('Denied: ') + (data.denied ? data.denied.length : 0);
			alert(msg);
			// Close the modal
			var modal = bootstrap.Modal.getInstance(document.getElementById('eventDetailsModal'));
			if (modal) modal.hide();
		})
		.catch(error => {
			alert(_t('Error starting search: ') + error.message);
		});
	})
}

// showMoreEvents lists all events of a day; query is the calendar's state
// (filters), the day replaces its date.
function showMoreEvents(date, query) {
	const modalTitle = document.getElementById('moreEventsModalLabel');
	const modalBody = document.getElementById('moreEventsBody');
	const parts = date.split('-');

	modalTitle.textContent = _t('Events on ') + new Date(+parts[0], +parts[1] - 1, +parts[2]).toLocaleDateString(document.documentElement.lang || undefined);
	modalBody.innerHTML = '<div class="text-center"><div class="spinner-border" role="status"><span class="visually-hidden">' + _t('Loading...') + '</span></div></div>';

	const modal = bootstrap.Modal.getOrCreateInstance(document.getElementById('moreEventsModal'));
	modal.show();

	var q = new URLSearchParams(query || '');
	q.set('date', date);
	htmx.ajax('GET', '/api/admin/calendar/day-events?' + q.toString(), { target: '#moreEventsBody', swap: 'innerHTML' });
}

// calendarSetParam reloads the calendar with one parameter changed.
function calendarSetParam(query, param, value) {
	var q = new URLSearchParams(query || '');
	if (value) { q.set(param, value); } else { q.delete(param); }
	if (param === 'group') { q.delete('genre'); }
	history.pushState(null, '', '/api/admin/calendar?' + q.toString());
	htmx.ajax('GET', '/api/admin/calendar/content?' + q.toString(), { target: '#calendar-content', swap: 'innerHTML' });
}

// calendarSearchURL is the search of an event's list entry; movies and
// episodes by ID or (byTitle) by title.
function calendarSearchURL(type, id, byTitle) {
	if (calendarSearchTypes[type]) { return calendarSearchTypes[type].path + id + '?download=true'; }
	if (type === 'movie') { return '/api/movies/search/list/' + id + '?searchByTitle=' + !!byTitle + '&download=true'; }
	return '/api/series/episodes/search/list/' + id + '?searchByTitle=' + !!byTitle + '&download=true';
}

// calendarBulkSearch searches overdue items one after the other.
function calendarBulkSearch(items, byTitle) {
	if (!items || !items.length) { return; }
	var question = items.length === 1
		? (byTitle ? _t('Search 1 missing item by title?') : _t('Search 1 missing item by ID?'))
		: (byTitle ? _tf('Search %s missing items by title? They are searched one after the other.', items.length)
			: _tf('Search %s missing items by ID? They are searched one after the other.', items.length));
	confirmAction(_t('Please confirm'), question, function () {
		var ok = 0, failed = 0, accepted = 0, i = 0;
		libNotify('info', _tf('Searching %d items…', items.length));
		(function next() {
			if (i >= items.length) {
				libNotify(failed ? 'error' : 'success', _tf('Search done: %d searched, %d accepted', ok, accepted) + (failed ? _tf(', %d failed', failed) : ''));
				return;
			}
			var it = items[i++];
			gmdApiProxyFetch(calendarSearchURL(it.type, it.id, byTitle))
				.then(function (r) { return r.json().catch(function () { return {}; }).then(function (d) { if (!r.ok) { throw new Error('HTTP ' + r.status); } return d; }); })
				.then(function (d) { ok++; accepted += d && d.accepted ? d.accepted.length : 0; }, function () { failed++; })
				.then(next);
		})();
	});
}

// calendarPlay plays a downloaded item in the bottom-bar player.
function calendarPlay(type, id) {
	fetch('/api/admin/library/playlist?type=' + encodeURIComponent(type) + '&id=' + id, { credentials: 'same-origin' })
		.then(function (r) { return r.json().then(function (d) { if (!r.ok) { throw new Error(d.error || ('HTTP ' + r.status)); } return d; }); })
		.then(function (d) {
			if (!d.list || !d.list.length) { throw new Error(_t('no file to play')); }
			lpPlayList(d.list.map(function (x) { return { kind: x.kind, id: String(x.id), label: x.label }; }), 0, d.entry, d.cover);
		})
		.catch(function (e) { libNotify('error', _t('Can\'t play: ') + e.message); });
}
		`)),
	})
}

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
			html.TabIndex("-1"),
			gomponents.Attr("aria-labelledby", "eventDetailsModalLabel"),
			gomponents.Attr("aria-hidden", "true"),
			html.Div(
				html.Class("modal-dialog modal-lg"),
				html.Div(
					html.Class("modal-content"),
					html.Div(
						html.Class("modal-header"),
						html.H5(
							html.ID("eventDetailsModalLabel"),
							html.Class("modal-title"),
							gomponents.Text("Event Details"),
						),
						html.Button(
							html.Type("button"),
							html.Class("btn-close"),
							html.Data("bs-dismiss", "modal"),
							gomponents.Attr("aria-label", "Close"),
						),
					),
					html.Div(
						html.ID("eventDetailsBody"),
						html.Class("modal-body"),
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

		// More Events Modal
		html.Div(
			html.ID("moreEventsModal"),
			html.Class("modal fade"),
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
							gomponents.Text("All Events"),
						),
						html.Button(
							html.Type("button"),
							html.Class("btn-close"),
							html.Data("bs-dismiss", "modal"),
							gomponents.Attr("aria-label", "Close"),
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
							gomponents.Text("Close"),
						),
					),
				),
			),
		),

		// JavaScript for modal functionality
		html.Script(gomponents.Raw(`
function showEventDetails(id, title, type, network, season, episode, date, overview, imdbId, thetvdbId, moviedbId, traktId, downloaded, listname) {
	const modalTitle = document.getElementById('eventDetailsModalLabel');
	const modalBody = document.getElementById('eventDetailsBody');

	modalTitle.textContent = title;

	let detailsHTML = '<div class="row justify-content-center">';
	detailsHTML += '<div class="col-md-10">';
	detailsHTML += '<h6><i class="fas fa-info-circle me-2"></i>Event Information</h6>';
	detailsHTML += '<table class="table table-sm table-borderless">';
	detailsHTML += '<tr><td class="fw-bold">Title:</td><td>' + title + '</td></tr>';
	detailsHTML += '<tr><td class="fw-bold">Type:</td><td><span class="badge bg-' + (type === 'movie' ? 'primary' : 'success') + '">' + type.charAt(0).toUpperCase() + type.slice(1) + '</span></td></tr>';

	if (listname) {
		detailsHTML += '<tr><td class="fw-bold">List:</td><td><span class="badge bg-secondary">' + listname + '</span></td></tr>';
	}

	detailsHTML += '<tr><td class="fw-bold">Date:</td><td>' + new Date(date).toLocaleDateString() + '</td></tr>';

	if (network) {
		detailsHTML += '<tr><td class="fw-bold">Network:</td><td>' + network + '</td></tr>';
	}

	if (type === 'series' && season > 0 && episode > 0) {
		detailsHTML += '<tr><td class="fw-bold">Episode:</td><td>S' + String(season).padStart(2, '0') + 'E' + String(episode).padStart(2, '0') + '</td></tr>';
	}

	if (overview) {
		detailsHTML += '<tr><td class="fw-bold">Description:</td><td>' + overview + '</td></tr>';
	}

	if (imdbId || thetvdbId || moviedbId || traktId) {
		detailsHTML += '<tr><td class="fw-bold">External Links:</td><td>';

		if (imdbId && imdbId !== '') {
			detailsHTML += '<a href="https://www.imdb.com/title/' + imdbId + '" target="_blank" class="btn btn-sm btn-outline-warning me-2">';
			detailsHTML += '<i class="fas fa-external-link-alt me-1"></i>IMDB</a>';
		}

		if (thetvdbId && thetvdbId > 0 && type === 'series') {
			detailsHTML += '<a href="https://thetvdb.com/search?query=' + thetvdbId + '" target="_blank" class="btn btn-sm btn-outline-info me-2">';
			detailsHTML += '<i class="fas fa-external-link-alt me-1"></i>TheTVDB</a>';
		}

		if (moviedbId && moviedbId > 0) {
			if (type === 'movie') {
				detailsHTML += '<a href="https://www.themoviedb.org/movie/' + moviedbId + '" target="_blank" class="btn btn-sm btn-outline-primary me-2">';
			} else {
				detailsHTML += '<a href="https://www.themoviedb.org/tv/' + moviedbId + '" target="_blank" class="btn btn-sm btn-outline-primary me-2">';
			}
			detailsHTML += '<i class="fas fa-external-link-alt me-1"></i>TMDB</a>';
		}

		if (traktId && traktId > 0) {
			if (type === 'movie') {
				detailsHTML += '<a href="https://trakt.tv/movies/' + traktId + '" target="_blank" class="btn btn-sm btn-outline-dark me-2">';
			} else {
				detailsHTML += '<a href="https://trakt.tv/shows/' + traktId + '" target="_blank" class="btn btn-sm btn-outline-dark me-2">';
			}
			detailsHTML += '<i class="fas fa-external-link-alt me-1"></i>Trakt</a>';
		}

		detailsHTML += '</td></tr>';
	}

	// Add search buttons if not downloaded
	if (!downloaded && id > 0) {
		detailsHTML += '<tr><td class="fw-bold">Actions:</td><td>';
		if (type === 'movie') {
			detailsHTML += '<button class="btn btn-sm btn-outline-dark me-2" onclick="searchCalendarEvent(' + id + ', \'movie\', false)" title="Search by IMDB"><i class="fa fa-search"></i> Search IMDB</button>';
			detailsHTML += '<button class="btn btn-sm btn-outline-dark" onclick="searchCalendarEvent(' + id + ', \'movie\', true)" title="Search by Title"><i class="fa fa-search-plus"></i> Search Title</button>';
		} else if (type === 'series') {
			detailsHTML += '<button class="btn btn-sm btn-outline-dark me-2" onclick="searchCalendarEvent(' + id + ', \'series\', false)" title="Search by TVDB"><i class="fa fa-search"></i> Search TVDB</button>';
			detailsHTML += '<button class="btn btn-sm btn-outline-dark" onclick="searchCalendarEvent(' + id + ', \'series\', true)" title="Search by Title"><i class="fa fa-search-plus"></i> Search Title</button>';
		}
		detailsHTML += '</td></tr>';
	}

	detailsHTML += '</table>';
	detailsHTML += '</div>';
	detailsHTML += '</div>';

	modalBody.innerHTML = detailsHTML;

	const modalElement = document.getElementById('eventDetailsModal');

	if (typeof bootstrap !== 'undefined') {
		const modal = new bootstrap.Modal(modalElement);
		modal.show();
	}
}

function searchCalendarEvent(id, type, searchByTitle) {
	var url;
	var confirmMsg;
	if (type === 'movie') {
		url = '/api/movies/search/list/' + id + '?apikey=' + encodeURIComponent(window.calendarApiKey || '') + '&searchByTitle=' + searchByTitle + '&download=true';
		confirmMsg = searchByTitle ? 'Start search for this movie by Title?' : 'Start search for this movie by IMDB ID?';
	} else {
		url = '/api/series/episodes/search/list/' + id + '?apikey=' + encodeURIComponent(window.calendarApiKey || '') + '&searchByTitle=' + searchByTitle + '&download=true';
		confirmMsg = searchByTitle ? 'Start search for this episode by Title?' : 'Start search for this episode by TVDB ID?';
	}

	if (confirm(confirmMsg)) {
		fetch(url, {
			method: 'GET',
			headers: {
				'X-CSRF-Token': document.querySelector('input[name="csrf_token"]')?.value || ''
			}
		})
		.then(response => response.json())
		.then(data => {
			var msg = 'Search completed!\n';
			msg += 'Accepted: ' + (data.accepted ? data.accepted.length : 0) + '\n';
			msg += 'Denied: ' + (data.denied ? data.denied.length : 0);
			alert(msg);
			// Close the modal
			var modal = bootstrap.Modal.getInstance(document.getElementById('eventDetailsModal'));
			if (modal) modal.hide();
		})
		.catch(error => {
			alert('Error starting search: ' + error.message);
		});
	}
}

function showMoreEvents(day, month, year) {
	const modalTitle = document.getElementById('moreEventsModalLabel');
	const modalBody = document.getElementById('moreEventsBody');

	modalTitle.textContent = 'Events for ' + month + ' ' + day + ', ' + year;
	modalBody.innerHTML = '<div class="text-center"><div class="spinner-border" role="status"><span class="visually-hidden">Loading...</span></div></div>';

	const modal = new bootstrap.Modal(document.getElementById('moreEventsModal'));
	modal.show();

	htmx.ajax('GET', '/api/admin/calendar/day-events?day=' + day + '&month=' + month + '&year=' + year, {
		target: '#moreEventsBody',
		swap: 'innerHTML'
	});
}
		`)),
	})
}

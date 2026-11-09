package api

import (
	"encoding/json"
	"strings"

	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// rowAction is one per-row button on the admin database table pages. The
// button, its click behaviour and its line in the help text all come from
// this one definition (rendered by tableScript), so they cannot drift apart.
//
// URL, Confirm and Done may contain {id} (the row's id column) and
// {listname} (the row's listname column).
type rowAction struct {
	Label   string `json:"label"`
	Icon    string `json:"icon"`  // Font Awesome class, e.g. "fa-rss"
	Color   string `json:"color"` // CSS colour of icon and tinted background
	Kind    string `json:"kind"`  // actionNavigate, actionJob or actionSearch
	URL     string `json:"url"`
	Confirm string `json:"confirm,omitempty"`
	Done    string `json:"done,omitempty"` // actionJob success message
	Help    string `json:"-"`
}

const (
	// actionNavigate opens URL in the browser.
	actionNavigate = "navigate"
	// actionJob calls URL through the admin proxy and reports success or error.
	actionJob = "job"
	// actionSearch calls URL through the admin proxy and reports accepted/denied counts.
	actionSearch = "search"
)

const (
	colorFiles    = "#198754"
	colorSearch   = "#fd7e14"
	colorSearch2  = "#6f42c1"
	colorRefresh  = "#20c997"
	colorList     = "#0aa2c0"
	colorRSS      = "#e8590c"
	colorDiscover = "#d63384"
)

func filesAction(filesTable, idColumn string) rowAction {
	return rowAction{
		Label: "View Files", Icon: "fa-file", Color: colorFiles, Kind: actionNavigate,
		URL:  "/api/admin/database/" + filesTable + "?filter-" + idColumn + "={id}",
		Help: "Files: View associated files",
	}
}

func subTableAction(label, target, filterColumn string) rowAction {
	return rowAction{
		Label: label, Icon: "fa-list", Color: colorList, Kind: actionNavigate,
		URL:  "/api/admin/database/" + target + "?filter-" + filterColumn + "={id}",
		Help: "List: " + label,
	}
}

// refreshAction refreshes the metadata of the row's own entry. The refresh
// routes take the db* id, which is the id column of the db* tables.
func refreshAction(apiGroup, what string) rowAction {
	return rowAction{
		Label: "Refresh Metadata", Icon: "fa-sync", Color: colorRefresh, Kind: actionJob,
		URL:     "/api/" + apiGroup + "/refresh/{id}",
		Confirm: "Refresh metadata for this " + what + "?",
		Done:    "Metadata refresh started for this " + what,
		Help:    "Refresh: Refresh this " + what + "'s metadata",
	}
}

func idSearchActions(apiPath, what, idName string) []rowAction {
	return []rowAction{
		{
			Label: "Search by " + idName, Icon: "fa-search", Color: colorSearch, Kind: actionSearch,
			URL:     apiPath + "{id}?searchByTitle=false&download=true",
			Confirm: "Start search for this " + what + " by " + idName + " ID?",
			Help:    "Search: Search by " + idName + " ID",
		},
		{
			Label: "Search by Title", Icon: "fa-search-plus", Color: colorSearch2, Kind: actionSearch,
			URL:     apiPath + "{id}?searchByTitle=true&download=true",
			Confirm: "Start search for this " + what + " by title?",
			Help:    "Search+: Search by title",
		},
	}
}

// listSearchAction starts the missing search for the row's whole list.
func listSearchAction(apiGroup, what string) rowAction {
	return rowAction{
		Label: "Search Missing in List", Icon: "fa-search-plus", Color: colorSearch2, Kind: actionJob,
		URL:     "/api/" + apiGroup + "/search/missing/{listname}",
		Confirm: "Start search for missing " + what + " in list \"{listname}\"?",
		Done:    "Search started for list {listname}",
		Help:    "Search+: Search missing " + what + " in this row's list",
	}
}

// authorSearchAction starts the artist/author based missing search for the
// row's whole list (the job picks the artists/authors itself).
func authorSearchAction(apiGroup, endpoint, what, icon string) rowAction {
	return rowAction{
		Label: "Search Missing " + what + " in List (" + endpoint + " search)", Icon: icon, Color: colorSearch, Kind: actionJob,
		URL:     "/api/" + apiGroup + "/search/" + endpoint + "/missing/{listname}",
		Confirm: "Search missing " + strings.ToLower(what) + " by " + endpoint + " in list \"{listname}\"?",
		Done:    "Search started for missing " + strings.ToLower(what) + " in list {listname}",
		Help:    "Search: Search missing " + strings.ToLower(what) + " by " + endpoint + " in this row's list",
	}
}

// tableActions lists the per-row buttons of each admin database table in
// addition to Edit and Delete, which every table has.
var tableActions = map[string][]rowAction{
	"movies":         append([]rowAction{filesAction("movie_files", "movie_id")}, idSearchActions("/api/movies/search/list/", "movie", "IMDB")...),
	"serie_episodes": append([]rowAction{filesAction("serie_episode_files", "serie_episode_id")}, idSearchActions("/api/series/episodes/search/list/", "episode", "TVDB")...),
	"series": {{
		Label: "RSS Search All Seasons", Icon: "fa-rss", Color: colorRSS, Kind: actionJob,
		URL:     "/api/series/searchrss/seasons/id/{id}",
		Confirm: "Start one RSS search per season for series #{id}?",
		Done:    "RSS season search started for series #{id}",
		Help:    "RSS: One RSS search per season",
	}},
	"dbmovies":     {refreshAction("movies", "movie")},
	"dbseries":     {refreshAction("series", "series")},
	"dbalbums":     {refreshAction("music", "album")},
	"dbaudiobooks": {refreshAction("audiobooks", "audiobook")},
	"dbbooks":      {refreshAction("books", "book")},
	"albums":       {filesAction("album_files", "album_id"), listSearchAction("music", "albums")},
	"audiobooks":   {filesAction("audiobook_files", "audiobook_id"), listSearchAction("audiobooks", "audiobooks")},
	"books":        {filesAction("book_files", "book_id"), listSearchAction("books", "books")},
	"dbartists":    {subTableAction("View Albums", "albums", "dbartist_id")},
	"artists": {
		subTableAction("View Albums", "albums", "artist_id"),
		authorSearchAction("music", "artists", "Albums", "fa-search"),
		{
			Label: "Discover Series Albums", Icon: "fa-layer-group", Color: colorDiscover, Kind: actionJob,
			URL:     "/api/music/discover/series/artist/{id}",
			Confirm: "Discover and import series albums for artist #{id}?",
			Done:    "Discover series albums queued for artist #{id}",
			Help:    "Discover: Discover and import all series albums",
		},
	},
	"dbauthors": {
		subTableAction("View Books", "books", "dbauthor_id"),
		subTableAction("View Audiobooks", "audiobooks", "dbauthor_id"),
	},
	"authors": {
		subTableAction("View Books", "books", "author_id"),
		subTableAction("View Audiobooks", "audiobooks", "author_id"),
		authorSearchAction("books", "authors", "Books", "fa-search"),
		authorSearchAction("audiobooks", "authors", "Audiobooks", "fa-search-plus"),
	},
}

// idColumnTables maps foreign-key columns to the table they point at; their
// cells (and responsive child-row values) link to that table's row.
var idColumnTables = map[string]string{
	"dbmovie_id":         "dbmovies",
	"movie_id":           "movies",
	"dbserie_id":         "dbseries",
	"serie_id":           "series",
	"serie_episode_id":   "serie_episodes",
	"dbserie_episode_id": "dbserie_episodes",
	"dbbook_id":          "dbbooks",
	"book_id":            "books",
	"dbauthor_id":        "dbauthors",
	"author_id":          "authors",
	"dbbook_series_id":   "dbbook_series",
	"book_series_id":     "book_series",
	"dbaudiobook_id":     "dbaudiobooks",
	"audiobook_id":       "audiobooks",
	"dbnarrator_id":      "dbnarrators",
	"dbalbum_id":         "dbalbums",
	"album_id":           "albums",
	"dbartist_id":        "dbartists",
	"artist_id":          "artists",
	"dbtrack_id":         "dbtracks",
}

// tableActionHelp is the help line above a table, built from its actions.
func tableActionHelp(tableName string) string {
	parts := []string{"Edit: Edit record", "Delete: Remove record"}
	seen := map[string]struct{}{}

	for _, a := range tableActions[tableName] {
		if _, ok := seen[a.Help]; ok || a.Help == "" {
			continue
		}

		seen[a.Help] = struct{}{}
		parts = append(parts, a.Help)
	}

	return strings.Join(parts, " | ")
}

// tablePageConfig is handed to tableScript as window.gmdTableConfig.
type tablePageConfig struct {
	Table   string      `json:"table"`
	CSRF    string      `json:"csrf"`
	Actions []rowAction `json:"actions"`
	// IDLinks maps the column alias of each FK column shown to its target table.
	IDLinks map[string]string `json:"idLinks"`
	// ChildLabels maps a FK column's header text to its target table, for the
	// responsive child rows that only show the header text.
	ChildLabels map[string]string `json:"childLabels"`
}

// tableScripts returns the config object and the generic table script.
// encoding/json escapes <, > and & so the config cannot close the script tag.
func tableScripts(tableInfo *TableInfo, csrfToken string, columnAliases []string) gomponents.Node {
	cfg := tablePageConfig{
		Table:       tableInfo.Name,
		CSRF:        csrfToken,
		Actions:     tableActions[tableInfo.Name],
		IDLinks:     map[string]string{},
		ChildLabels: map[string]string{},
	}
	if cfg.Actions == nil {
		cfg.Actions = []rowAction{}
	}

	for i, alias := range columnAliases {
		if target, ok := idColumnTables[alias]; ok {
			cfg.IDLinks[alias] = target
			cfg.ChildLabels[tableInfo.Columns[i].DisplayName] = target
		}
	}

	cfgJSON, err := json.Marshal(cfg)
	if err != nil {
		cfgJSON = []byte("{}")
	}

	return gomponents.Group([]gomponents.Node{
		html.Script(gomponents.Raw("window.gmdTableConfig = " + string(cfgJSON) + ";")),
		html.Script(gomponents.Raw(tableScript)),
	})
}

// tableScript drives every admin database table page from window.gmdTableConfig.
// It runs inline in the body, before the shared admin script at the end of
// the page, so it sets DataTables up itself instead of using initDataTable.
const tableScript = `
(function() {
	var cfg = window.gmdTableConfig;
	var oTable;

	// Cell values are raw DB strings (titles, overviews, filenames from
	// indexers/metadata); escape them before they become HTML.
	function esc(v) {
		if (v === null || v === undefined) {
			return '';
		}
		return String(v).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
	}

	function idLink(target, value) {
		return '<a href="/api/admin/database/' + target + '?id=' + encodeURIComponent(value) + '" class="id-link" title="View in ' + target + ' table">' + esc(value) + '</a>';
	}

	function columnIndex(alias) {
		var idx = -1;
		$('#table-data thead th').each(function(i) {
			if ($(this).attr('data-column-name') === alias) {
				idx = i;
			}
		});
		return idx;
	}

	function rowButtons(id) {
		var html = '<div class="d-flex gap-1 justify-content-center">' +
			'<button class="row-action btn-row-edit" style="--action-color:#0d6efd" data-id="' + id + '" data-bs-toggle="modal" data-bs-target="#editFormModal" aria-label="Edit" title="Edit"><i aria-hidden="true" class="fa fa-edit"></i></button>' +
			'<button class="row-action btn-row-delete" style="--action-color:#dc3545" data-id="' + id + '" aria-label="Delete" title="Delete"><i aria-hidden="true" class="fa fa-trash"></i></button>';
		cfg.actions.forEach(function(a, i) {
			html += '<button class="row-action btn-row-custom" style="--action-color:' + esc(a.color) + '" data-id="' + id + '" data-action="' + i + '" aria-label="' + esc(a.label) + '" title="' + esc(a.label) + '"><i aria-hidden="true" class="fa ' + esc(a.icon) + '"></i></button>';
		});
		return html + '</div>';
	}

	oTable = $('.datatable').DataTable({
		"bDestroy": true,
		"bFilter": true,
		"bSort": true,
		"bPaginate": true,
		responsive: true,
		"aaSorting": [[0, "desc"]],
		"bProcessing": true,
		"bServerSide": true,
		"sAjaxSource": "/api/admin/tablejson/" + encodeURIComponent(cfg.table),
		"fnServerData": function(sSource, aoData, fnCallback) {
			// filter-* URL parameters come from links such as "View Files".
			var urlParams = new URLSearchParams(window.location.search);
			urlParams.forEach(function(value, key) {
				if (key.indexOf('filter-') === 0) {
					aoData.push({ "name": key, "value": value });
				}
			});
			var idValue = urlParams.get('id');
			if (idValue && !urlParams.has('filter-id')) {
				aoData.push({ "name": "filter-id", "value": idValue });
			}
			$('.custom-filter').each(function() {
				var value = $(this).val();
				if (value) {
					aoData.push({ "name": $(this).attr('id'), "value": value });
				}
			});
			$.ajax({
				"dataType": 'json',
				"type": "POST",
				"url": sSource,
				"data": aoData,
				"headers": { "X-CSRF-Token": cfg.csrf },
				"success": fnCallback,
				"error": function(xhr, error, code) {
					console.error('Table data request failed:', error, code, xhr.responseText);
				}
			});
		},
		"columnDefs": [
			{
				"targets": -1,
				"data": null,
				"orderable": false,
				"searchable": false,
				"render": function(data, type, row) {
					return rowButtons(esc(row[0]));
				}
			},
			{
				"targets": "_all",
				"render": function(data, type, row, meta) {
					if (meta.col === row.length - 1) {
						return data;
					}
					var colName = $('#table-data thead th').eq(meta.col).attr('data-column-name');
					if (type === 'display' && cfg.idLinks[colName] && data && String(data) !== '0') {
						return idLink(cfg.idLinks[colName], data);
					}
					return type === 'display' ? esc(data) : data;
				}
			}
		]
	});

	// Responsive child rows show only the header text, so link by label.
	function linkChildRowIds() {
		$('.dtr-details li').each(function() {
			var target = cfg.childLabels[$(this).find('.dtr-title').text().trim()];
			var $data = $(this).find('.dtr-data');
			var value = $data.text().trim();
			if (target && value !== '' && value !== '0' && !isNaN(value)) {
				$data.html(idLink(target, value));
			}
		});
	}
	oTable.on('responsive-display', function(e, datatable, row, showHide) {
		if (showHide) {
			linkChildRowIds();
		}
	});
	oTable.on('draw', linkChildRowIds);

	$(document).on('change keyup input', '.custom-filter', function() {
		var element = $(this);
		clearTimeout(element.data('timer'));
		element.data('timer', setTimeout(function() {
			oTable.ajax.reload();
		}, element.is('select') ? 0 : 500));
	});

	$(document).on('click', '.btn-row-edit', function() {
		var id = $(this).data('id');
		$('#editFormModal .modal-body').html('<div class="text-center"><div class="spinner-border" role="status"><span class="sr-only">Loading...</span></div><p class="mt-2">Loading edit form...</p></div>');
		$.ajax({
			url: '/api/admin/tableedit/' + encodeURIComponent(cfg.table) + '/' + encodeURIComponent(id),
			type: 'GET',
			headers: { 'X-CSRF-Token': $('input[name="csrf_token"]').val() || '' }
		})
			.done(function(data) {
				$('#editFormModal .modal-body').html(data);
				setTimeout(function() {
					if (window.initSelect2Global) {
						window.initSelect2Global();
					}
				}, 100);
			})
			.fail(function(xhr, status, error) {
				var msg = 'Error loading form: ' + esc(error) + (xhr.status ? ' (HTTP ' + xhr.status + ')' : '');
				$('#editFormModal .modal-body').html('<div class="alert alert-danger">' + msg + '</div>');
			});
	});

	$(document).on('click', '.btn-row-delete', function() {
		var id = $(this).data('id');
		confirmAction('Please confirm', '⚠️ Are you sure you want to permanently delete this record?\n\nThis action cannot be undone.', function() {
			$.ajax({
				url: '/api/admin/table/' + encodeURIComponent(cfg.table) + '/delete/' + encodeURIComponent(id),
				type: 'POST',
				headers: { 'X-CSRF-Token': $('input[name="csrf_token"]').val() || '' },
				success: function() {
					oTable.ajax.reload();
					alert('Record deleted successfully');
				},
				error: function() {
					alert('Error deleting record');
				}
			});
		});
	});

	// fill replaces {id} and {listname}; enc URL-encodes the values for URLs.
	function fill(text, vals, enc) {
		return text.replace(/\{(id|listname)\}/g, function(m, key) {
			return enc ? encodeURIComponent(vals[key]) : vals[key];
		});
	}

	function responseError(r) {
		if (r.status === 204) {
			return Promise.reject(new Error('nothing done - the entry is not in any configured list'));
		}
		return r.json().catch(function() { return {}; }).then(function(d) {
			// Most routes send {"error": ...}; the search routes send a bare string.
			throw new Error((typeof d === 'string' && d) || d.error || ('HTTP ' + r.status));
		});
	}

	function runAction(a, vals) {
		var url = fill(a.url, vals, true);
		if (a.kind === 'navigate') {
			window.location.href = url;
			return;
		}
		gmdApiProxyFetch(url)
			.then(function(r) {
				if (!r.ok || r.status === 204) {
					return responseError(r);
				}
				return r.json().catch(function() { return {}; });
			})
			.then(function(d) {
				if (a.kind === 'search') {
					alert('Search completed!\nAccepted: ' + (d.accepted ? d.accepted.length : 0) + '\nDenied: ' + (d.denied ? d.denied.length : 0));
					oTable.ajax.reload();
				} else {
					alert(fill(a.done || 'Started', vals, false));
				}
			})
			.catch(function(err) {
				alert('Error: ' + err.message);
			});
	}

	$(document).on('click', '.btn-row-custom', function() {
		var a = cfg.actions[$(this).data('action')];
		var vals = { id: String($(this).data('id')), listname: '' };
		if ((a.url + (a.confirm || '')).indexOf('{listname}') !== -1) {
			var row = oTable.row($(this).closest('tr')).data();
			var idx = columnIndex('listname');
			if (idx === -1 || !row || !row[idx]) {
				alert('Could not determine the list name for this record');
				return;
			}
			vals.listname = String(row[idx]);
		}
		if (!a.confirm) {
			runAction(a, vals);
			return;
		}
		confirmAction('Please confirm', esc(fill(a.confirm, vals, false)), function() {
			runAction(a, vals);
		});
	});
})();
`

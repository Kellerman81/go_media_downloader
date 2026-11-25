// Folder browser for the path fields of the config and the setup wizard:
// an option to typing the path. gmdPathBrowse(fieldName) opens it on the
// field's current value; "Select this folder" writes the path back into
// the field. Every path field includes this file, so it defines itself once.
(function () {
	if (window.gmdPathBrowse) { return; }

	var state = { field: null, path: '' };

	function el(tag, cls, text) {
		var e = document.createElement(tag);
		if (cls) { e.className = cls; }
		if (text !== undefined) { e.textContent = text; }
		return e;
	}

	function modal() {
		var m = document.getElementById('gmdPathBrowser');
		if (m) { return m; }
		m = el('div', 'modal fade');
		m.id = 'gmdPathBrowser';
		m.tabIndex = -1;
		m.innerHTML =
			'<div class="modal-dialog modal-lg modal-dialog-scrollable"><div class="modal-content">' +
			'<div class="modal-header"><h5 class="modal-title"><i class="fa-solid fa-folder-tree me-2"></i>Choose a folder</h5>' +
			'<button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button></div>' +
			'<div class="modal-body">' +
			'<div class="input-group input-group-sm mb-2"><span class="input-group-text"><i class="fa-solid fa-folder-open"></i></span>' +
			'<input type="text" class="form-control font-monospace gmd-pb-path" aria-label="Folder">' +
			'<button type="button" class="btn btn-outline-secondary gmd-pb-go">Go</button></div>' +
			'<div class="d-flex flex-wrap gap-1 mb-2 gmd-pb-places"></div>' +
			'<div class="small text-warning mb-2 gmd-pb-note"></div>' +
			'<div class="list-group gmd-pb-list"></div></div>' +
			'<div class="modal-footer"><button type="button" class="btn btn-secondary" data-bs-dismiss="modal">Cancel</button>' +
			'<button type="button" class="btn btn-primary gmd-pb-select"><i class="fa-solid fa-check me-1"></i>Select this folder</button></div>' +
			'</div></div>';
		document.body.appendChild(m);
		m.querySelector('.gmd-pb-go').onclick = function () { load(m.querySelector('.gmd-pb-path').value); };
		m.querySelector('.gmd-pb-path').addEventListener('keydown', function (e) {
			if (e.key === 'Enter') { e.preventDefault(); load(this.value); }
		});
		m.querySelector('.gmd-pb-select').onclick = function () {
			var input = document.querySelector('[name="' + CSS.escape(state.field) + '"]');
			if (input) {
				input.value = state.path;
				input.dispatchEvent(new Event('input', { bubbles: true }));
				input.dispatchEvent(new Event('change', { bubbles: true }));
			}
			bootstrap.Modal.getOrCreateInstance(m).hide();
		};
		return m;
	}

	function join(dir, name) {
		return dir.replace(/[\\/]+$/, '') + (dir.indexOf('\\') >= 0 && dir.indexOf('/') < 0 ? '\\' : '/') + name;
	}

	function load(path) {
		var m = modal(), list = m.querySelector('.gmd-pb-list');
		list.innerHTML = '<div class="text-muted small p-2">Loading…</div>';
		fetch('/api/admin/paths/browse?path=' + encodeURIComponent(path || ''), { credentials: 'same-origin' })
			.then(function (r) { if (!r.ok) { throw new Error('HTTP ' + r.status); } return r.json(); })
			.then(function (d) {
				state.path = d.path;
				m.querySelector('.gmd-pb-path').value = d.path;
				m.querySelector('.gmd-pb-note').textContent = d.note || '';

				var places = m.querySelector('.gmd-pb-places');
				places.innerHTML = '';
				(d.places || []).forEach(function (p) {
					var b = el('button', 'btn btn-sm btn-outline-primary', p);
					b.type = 'button';
					b.onclick = function () { load(p); };
					places.appendChild(b);
				});

				list.innerHTML = '';
				if (d.parent) {
					var up = el('button', 'list-group-item list-group-item-action');
					up.type = 'button';
					up.innerHTML = '<i class="fa-solid fa-turn-up me-2"></i>..';
					up.title = d.parent;
					up.onclick = function () { load(d.parent); };
					list.appendChild(up);
				}
				d.dirs.forEach(function (name) {
					var b = el('button', 'list-group-item list-group-item-action text-truncate');
					b.type = 'button';
					var icon = el('i', 'fa-solid fa-folder text-warning me-2');
					b.appendChild(icon);
					b.appendChild(document.createTextNode(name));
					b.onclick = function () { load(join(d.path, name)); };
					list.appendChild(b);
				});
				if (!d.dirs.length) { list.appendChild(el('div', 'text-muted small p-2', 'No folders in here.')); }
				if (d.more) { list.appendChild(el('div', 'text-muted small p-2', 'Only the first folders are shown: type the path above to go deeper.')); }
			})
			.catch(function (e) { list.innerHTML = ''; list.appendChild(el('div', 'text-danger small p-2', 'Can\'t list the folder: ' + e.message)); });
	}

	window.gmdPathBrowse = function (fieldName) {
		var input = document.querySelector('[name="' + CSS.escape(fieldName) + '"]');
		state.field = fieldName;
		load(input ? input.value.trim() : '');
		bootstrap.Modal.getOrCreateInstance(modal()).show();
	};
}());

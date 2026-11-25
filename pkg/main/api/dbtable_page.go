package api

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

func buildDatabaseTableSelect() gomponents.Node {
	tables := database.GetAllTableNames()
	options := make([]gomponents.Node, 0, len(tables)+1)

	options = append(
		options,
		gomponents.El("option", gomponents.Attr("value", ""), gomponents.Text("— select table —")),
	)
	for _, t := range tables {
		options = append(
			options,
			gomponents.El("option", gomponents.Attr("value", t), gomponents.Text(t)),
		)
	}

	return gomponents.Group([]gomponents.Node{
		gomponents.El(
			"select",
			html.ID("dbTableSelect"),
			html.Class("form-select form-select-sm"),
			gomponents.Attr(
				"onchange",
				"if(this.value)window.location='/api/admin/database/'+this.value",
			),
			gomponents.Group(options),
		),
		html.Script(gomponents.Raw(
			`(function(){` +
				`var m=window.location.pathname.match(/\/database\/([^/?#]+)/);` +
				`if(m)document.getElementById('dbTableSelect').value=m[1];` +
				`})();`,
		)),
	})
}

// adminDatabaseTab component.
func adminDatabaseContent(tableName string, csrfToken string) gomponents.Node {
	tableColumns := getAdminTableColumns(tableName)
	tableDefault := database.GetTableDefaults(tableName)

	tableInfo := TableInfo{
		Name:    tableName,
		Columns: tableColumns,
		Rows: func() []map[string]any {
			if tableDefault.DefaultColumns == "" {
				// logger.Logtype("warning", 2).Str("table", tableName).Str("defaultColumns", tableDefault.DefaultColumns).Msg("Empty DefaultColumns")
				return []map[string]any{}
			}

			query := fmt.Sprintf(
				"SELECT %s FROM %s LIMIT 10",
				tableDefault.DefaultColumns,
				tableDefault.Table,
			)
			// logger.Logtype("debug", 1).Str("query", query).Msg("Executing preview query")
			return database.GetrowsType(tableDefault.Object, false, 10, query)
		}(),
		DeleteURL: fmt.Sprintf("/api/admin/table/%s/delete", tableName),
	}

	// Enhanced database content with modern design
	return html.Div(
		html.Input(html.Name("table-name"), html.Type("hidden"), html.ID("table-name")),

		adminModal(),
		adminAddModal(tableName, csrfToken),

		// Enhanced main container with modern styling
		html.Div(
			html.Class("config-section-enhanced"),

			// Enhanced status messages with modern alerts
			html.Div(html.Class("alert-success-enhanced d-none"), html.ID("db-success")),
			html.Div(html.Class("alert-danger-enhanced d-none"), html.ID("db-error")),

			// Enhanced page header with AdminKit design
			html.Div(
				html.Class("page-header-enhanced"),
				html.Div(
					html.Class("header-content"),
					html.Div(
						html.Class("header-icon-wrapper"),
						html.I(html.Class("fas fa-database header-icon")),
					),
					html.Div(
						html.Class("header-text"),
						html.H2(
							html.Class("header-title"),
							gomponents.Text(strings.ReplaceAll(tableName, "_", " ")),
						),
						html.P(
							html.Class("header-subtitle"),
							gomponents.Text(
								"Manage database records with advanced filtering and real-time operations",
							),
						),
					),
					html.Div(
						html.Class("ms-auto d-flex gap-2"),
						html.Button(
							html.Class("btn btn-outline-secondary btn-sm"),
							html.Type("button"),
							html.Data("bs-toggle", "modal"),
							html.Data("bs-target", "#addRecordModal"),
							html.I(html.Class("fas fa-plus me-1")),
							gomponents.Text("Add Record"),
						),
						html.Button(
							html.Class("btn btn-outline-secondary btn-sm"),
							html.Type("button"),
							html.ID("refresh-table"),
							html.I(html.Class("fas fa-sync-alt me-1")),
							gomponents.Text("Refresh"),
						),
					),
				),
			),

			// Button description region
			adminButtonDescription(tableName),

			// Enhanced custom filters
			renderCustomFilters(tableName),

			// Enhanced table container
			html.Div(
				html.Class("database-table-container"),
				html.ID("table-content"),
				renderTable(&tableInfo, csrfToken),
			),
		),
	)
}

// adminButtonDescription returns a description panel explaining the available action buttons for the given table.
func adminButtonDescription(tableName string) gomponents.Node {
	desc := tableActionHelp(tableName)

	return html.Div(
		html.Class("alert alert-info py-2 px-3 mb-3"),
		html.Style("font-size: 0.85rem;"),
		html.I(html.Class("fas fa-info-circle me-2")),
		gomponents.Text(desc),
	)
}

// adminModal component.
func adminModal() gomponents.Node {
	return html.Div(
		html.Class("modal fade"),
		html.ID("editFormModal"),

		html.Div(
			html.Class("modal-dialog modal-xl"),

			html.Div(
				html.Class("modal-content"),

				html.Div(
					html.Class("modal-header"),
					html.H5(html.Class("modal-title"), gomponents.Text("Edit Record")),
					html.Button(
						html.Class("btn-close"),
						gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"),
					),
				),

				html.Div(
					html.Class("modal-body"),
					html.ID("modal-body-content"),
					// The form content will be loaded here by the DataTables edit handler
				),
			),
		),
	)
}

// adminAddModal component.
func adminAddModal(tableName string, csrfToken string) gomponents.Node {
	// Get table columns to create empty data map - use form-specific columns to exclude joined columns
	emptyData := make(map[string]any)
	tableColumns := getAdminFormColumns(tableName)

	// Initialize empty data for all columns except auto-generated ones
	for _, col := range tableColumns {
		columnName := col.Name
		if strings.Contains(col.Name, " as ") {
			columnName = strings.Split(col.Name, " as ")[1]
		}

		// Skip auto-generated fields
		if columnName != "id" && columnName != "created_at" && columnName != "updated_at" {
			// Initialize boolean-like fields as integers (0 = false) so they render as switches
			if columnName == "missing" || columnName == "blacklisted" ||
				columnName == "dont_search" ||
				columnName == "dont_upgrade" ||
				columnName == "use_regex" ||
				columnName == "proper" ||
				columnName == "extended" ||
				columnName == "repack" ||
				columnName == "ignore_runtime" ||
				columnName == "adult" ||
				columnName == "search_specials" ||
				columnName == "quality_reached" {
				emptyData[columnName] = 0
			} else {
				emptyData[columnName] = ""
			}
		}
	}

	return html.Div(
		html.Class("modal fade"),
		html.ID("addRecordModal"),

		html.Div(
			html.Class("modal-dialog modal-lg"),

			html.Div(
				html.Class("modal-content"),

				html.Div(
					html.Class("modal-header"),
					html.H5(html.Class("modal-title"), gomponents.Text("Add New Record")),
					html.Button(
						html.Class("btn-close"),
						gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"),
					),
				),
				html.Div(
					html.Class("modal-body"),
					renderTableEditForm(tableName, emptyData, "new", csrfToken),
				),
			),
		),
	)
}

// adminJavaScript component.
func adminJavaScript() gomponents.Node {
	jsContent := `
			function showToaster(type, message) {								
						// Fallback for missing Bootstrap or when toasts don't work
						if (typeof bootstrap === 'undefined' || !bootstrap.Toast) {
										alert((type === 'success' ? '✓ ' : '✗ ') + message);
							return;
						}
						
						const toastContainer = document.getElementById('toastContainer');
						if (!toastContainer) {
										alert((type === 'success' ? '✓ ' : '✗ ') + message);
							return;
						}
						
						const toastId = 'toast-' + Date.now();
						
						// Determine toast styling based on type
						let bgClass, iconClass, headerText;
						switch(type) {
							case 'success':
								bgClass = 'bg-success text-white';
								iconClass = 'fas fa-check-circle';
								headerText = _t('Success');
								break;
							case 'error':
								bgClass = 'bg-danger text-white';
								iconClass = 'fas fa-exclamation-circle';
								headerText = _t('Error');
								break;
							case 'warning':
								bgClass = 'bg-warning text-dark';
								iconClass = 'fas fa-exclamation-triangle';
								headerText = _t('Warning');
								break;
							case 'info':
							default:
								bgClass = 'bg-info text-white';
								iconClass = 'fas fa-info-circle';
								headerText = _t('Info');
								break;
						}
						
						// Create toast HTML
						const toastHTML = ` + "`" + `
							<div id="${toastId}" class="toast align-items-center ${bgClass} border-0" role="alert" aria-live="assertive" aria-atomic="true">
								<div class="d-flex">
									<div class="toast-body d-flex align-items-center">
										<i class="${iconClass} me-2"></i>
										<span class="gmd-toast-msg"></span>
									</div>
									<button type="button" class="btn-close btn-close-white me-2 m-auto" data-bs-dismiss="toast" aria-label="Close"></button>
								</div>
							</div>
						` + "`" + `;
						
						// Add toast to container
						toastContainer.insertAdjacentHTML('beforeend', toastHTML);
						
						try {
							// Initialize and show the toast
							const toastElement = document.getElementById(toastId);
							// Messages carry server/provider text (titles, error strings) - never parse them as HTML.
							toastElement.querySelector('.gmd-toast-msg').textContent = String(message == null ? '' : message);
							const toast = new bootstrap.Toast(toastElement, {
								autohide: true,
								delay: type === 'error' ? 8000 : 4000  // Error messages stay longer
							});
							
							// Remove toast from DOM after it's hidden
							toastElement.addEventListener('hidden.bs.toast', function() {
								toastElement.remove();
							});
							
							toast.show();
									} catch (error) {
										alert((type === 'success' ? '✓ ' : '✗ ') + message);
						}
					}
					
					// Global error handler for AJAX requests
					window.showToaster = showToaster;

					// Route legacy alert() calls through the toast system for a consistent UI.
					// Type is inferred from the message so existing call sites need no changes.
					if (!window.__alertPatched) {
						window.__alertPatched = true;
						window.alert = function(message) {
							var msg = String(message == null ? '' : message);
							var type = 'info';
							if (/^\s*(error|failed|invalid|please|warning|unable|could not|cannot)\b/i.test(msg) || /\berror\b/i.test(msg)) {
								type = 'error';
							} else if (/^\s*success\b/i.test(msg) || /\b(success|saved|added|updated|deleted|copied|started)\b/i.test(msg)) {
								type = 'success';
							}
							showToaster(type, msg);
						};
					}

					// confirmAction shows a Bootstrap modal confirm and invokes onConfirm when accepted.
					// Replaces native confirm() so destructive actions match the app theme.
					window.confirmAction = function(title, message, onConfirm, opts) {
						opts = opts || {};
						if (typeof bootstrap === 'undefined' || !bootstrap.Modal) {
							// Fallback to native confirm if Bootstrap is unavailable.
							if (window.confirm((title ? title + '\n\n' : '') + (message || ''))) { onConfirm && onConfirm(); }
							return;
						}
						var existing = document.getElementById('globalConfirmModal');
						if (existing) { existing.remove(); }
						var confirmClass = opts.danger === false ? 'btn-primary' : 'btn-danger';
						var confirmLabel = opts.confirmLabel || _t('Confirm');
						var html = '' +
							'<div class="modal fade" id="globalConfirmModal" tabindex="-1" aria-hidden="true">' +
							'  <div class="modal-dialog modal-dialog-centered">' +
							'    <div class="modal-content">' +
							'      <div class="modal-header">' +
							'        <h5 class="modal-title"><i class="fas fa-question-circle me-2 text-warning"></i>' + (title || _t('Please confirm')) + '</h5>' +
							'        <button type="button" class="btn-close" data-bs-dismiss="modal" aria-label="Close"></button>' +
							'      </div>' +
							'      <div class="modal-body">' + (message || '') + '</div>' +
							'      <div class="modal-footer">' +
							'        <button type="button" class="btn btn-secondary" data-bs-dismiss="modal">' + _t('Cancel') + '</button>' +
							'        <button type="button" class="btn ' + confirmClass + '" id="globalConfirmOk">' + confirmLabel + '</button>' +
							'      </div>' +
							'    </div>' +
							'  </div>' +
							'</div>';
						document.body.insertAdjacentHTML('beforeend', html);
						var modalEl = document.getElementById('globalConfirmModal');
						var modal = new bootstrap.Modal(modalEl);
						modalEl.querySelector('#globalConfirmOk').addEventListener('click', function() {
							modal.hide();
							onConfirm && onConfirm();
						});
						modalEl.addEventListener('hidden.bs.modal', function() { modalEl.remove(); });
						modal.show();
					};

					// gmdApiProxyFetch calls the session-authenticated admin proxy
					// (admin_proxy.go) for the small set of external, apikey-gated
					// API actions the admin UI's own JS needs to trigger (search,
					// queue-cancel, etc.) - the real WebAPIKey never reaches the
					// browser this way; the proxy attaches it server-side after
					// re-checking the admin session + CSRF token.
					window.gmdApiProxyFetch = function(path) {
						var csrfInput = document.querySelector('input[name="csrf_token"]');
						var csrfToken = csrfInput ? csrfInput.value : '';
						return fetch('/api/admin/proxy?path=' + encodeURIComponent(path), {
							method: 'POST',
							headers: { 'X-CSRF-Token': csrfToken }
						});
					};

					// gmdGenerateApiKey fetches a fresh random API key and fills the
					// named field with it - it does not save the config itself, so
					// the admin must still click Save, exactly as if they had typed
					// a new value in by hand.
					window.gmdGenerateApiKey = function(fieldName) {
						var csrfInput = document.querySelector('input[name="csrf_token"]');
						var csrfToken = csrfInput ? csrfInput.value : '';
						fetch('/api/admin/generate-apikey', {
							method: 'POST',
							headers: { 'X-CSRF-Token': csrfToken }
						}).then(function(r) { return r.text(); }).then(function(key) {
							var el = document.querySelector('[name=' + CSS.escape(fieldName) + ']');
							// Secret fields render as password inputs; reveal the new key so the admin can note it.
							if (el) { el.value = key; if (el.type === 'password') { el.type = 'text'; } }
						});
					};

					// Route HTMX hx-confirm prompts through the styled modal instead of native confirm().
					if (!window.__htmxConfirmPatched) {
						window.__htmxConfirmPatched = true;
						document.addEventListener('htmx:confirm', function(e) {
							if (!e.detail || !e.detail.question) { return; }
							e.preventDefault();
							window.confirmAction('Please confirm', e.detail.question, function() {
								e.detail.issueRequest(true);
							});
						});
					}
			// Add CSS for Choices.js z-index fix
			if (!document.querySelector('#choices-css-fix')) {
				var style = document.createElement('style');
				style.id = 'choices-css-fix';
				style.textContent = '.choices { position: relative; } ' +
					'.choices[data-type*="select-one"] .choices__inner { cursor: pointer; } ' +
					'.choices__list--dropdown.is-active { transform: none !important; } ' +
					// Not in scrollable dialogs: their body must scroll (Library, Calendar).
					'.form-card, .form-cards-grid, .modal-dialog:not(.modal-dialog-scrollable) .modal-body, .card-body, .edit-form-container, .edit-form-fields, .config-section-enhanced { overflow: visible !important; } ' +
					'.choices.is-open .choices__list--dropdown { visibility: visible !important; opacity: 1 !important; pointer-events: auto !important; } ' +
					'.form-group-enhanced .form-check-input-modern:checked + label::after { content: "✓"; position: absolute; right: 10px; color: #28a745; font-weight: bold; } ' +
					'.form-group-enhanced { position: relative; } ' +
					'.form-group-enhanced .form-check-input-modern { position: relative; }';
				document.head.appendChild(style);
			}

			// Enhanced Choices.js initialization with AJAX support
			window.initChoicesGlobal = function() {
				// Check if Choices is available
				if (typeof Choices === 'undefined') {
					return;
				}
				
				// Check if Choices elements exist
				if (document.querySelectorAll('.choices-ajax').length === 0) {
					return;
				}
				
				// Remove duplicate elements with same ID to prevent conflicts
				var seenIds = {};
				document.querySelectorAll('.choices-ajax').forEach(function(element) {
					var id = element.id;
					if (seenIds[id]) {
						element.remove();
					} else {
						seenIds[id] = true;
					}
				});
				
				// Initialize AJAX-enabled Choices.js dropdowns
				document.querySelectorAll('.choices-ajax').forEach(function(element) {
					// Skip if already initialized
					if (element.classList.contains('choices__input')) {
						return;
					}
					var ajaxUrl = element.dataset.ajaxUrl;
					var selectedValue = element.dataset.selectedValue;
					var placeholder = element.dataset.placeholder || 'Search...';
					var csrfToken = document.querySelector('input[name="csrf_token"]').value || '';
					
					// Clear existing options
					element.innerHTML = '<option value="">' + placeholder + '</option>';
					
					var allowCustom = element.dataset.allowCustom === 'true';
					
					var choices = new Choices(element, {
						placeholder: true,
						placeholderValue: placeholder,
						searchPlaceholderValue: allowCustom ? 'Type to search or enter custom value...' : 'Type to search...',
						removeItemButton: true,
						searchEnabled: true,
						searchResultLimit: 50,
						renderChoiceLimit: 50,
						shouldSort: false,
						allowHTML: false, // labels are DB/provider titles - render as text
						addItems: allowCustom,
						editItems: false,  // Never allow editing of selected items
						addItemText: allowCustom ? 'Press Enter to add custom value' : '',
						duplicateItemsAllowed: false,
						position: 'auto'
					});
					
					// Store choices instance for later use
					element.choicesInstance = choices;
					
					// Ensure dropdown is never disabled
					choices.enable();
					
					// Fix dropdown positioning when opened
					element.addEventListener('choice', function() {
						setTimeout(function() {
							var dropdown = element.parentElement.querySelector('.choices__list--dropdown');
							if (dropdown) {
								var rect = element.getBoundingClientRect();
								var viewportHeight = window.innerHeight;
								var spaceBelow = viewportHeight - rect.bottom;
								var dropdownHeight = dropdown.offsetHeight || 200;
								
								if (spaceBelow < dropdownHeight && rect.top > dropdownHeight) {
									// Not enough space below, position above if there's room
									dropdown.style.top = 'auto';
									dropdown.style.bottom = '100%';
								} else {
									// Default position below
									dropdown.style.top = '100%';
									dropdown.style.bottom = 'auto';
								}
								dropdown.style.left = '0';
								dropdown.style.right = 'auto';
								dropdown.style.width = element.offsetWidth + 'px';
							}
						}, 10);
					});
					
					// Function to load choices from server
					function loadChoices(searchTerm) {
						// Optional: scope results to a media configuration whose value is
						// read from another element (e.g. the search page's MediaConfig).
						var extraParams = '';
						var configSource = element.dataset.configSource;
						if (configSource) {
							var configEl = document.getElementById(configSource);
							if (configEl && configEl.value) {
								extraParams = '&media_config=' + encodeURIComponent(configEl.value);
							}
						}
						fetch(ajaxUrl, {
							method: 'POST',
							headers: {
								'Content-Type': 'application/x-www-form-urlencoded',
								'X-CSRF-Token': csrfToken
							},
							body: 'search=' + encodeURIComponent(searchTerm || '') + '&page=1' + extraParams
						})
						.then(response => response.json())
						.then(data => {
							if (data.results && Array.isArray(data.results)) {
								choices.clearStore();
								if (data.results.length > 0) {
									choices.setChoices(data.results.map(function(item) {
										return {
											value: item.id,
											label: item.text,
											selected: false
										};
									}), 'value', 'label', true);
								} else {
									// Don't set disabled choices - just clear the store
									// This prevents the dropdown from becoming disabled
									choices.clearStore();
									choices.enable(); // Ensure it stays enabled
								}
							}
						})
						.catch(error => {
							console.error('Error loading choices:', error);
						});
					}

					// Expose the loader so external code (e.g. a media-config change
					// handler) can refresh this dropdown's options.
					element.choicesReload = function() {
						choices.clearStore();
						loadChoices('');
					};

					// Set up search event listener
					element.addEventListener('search', function(event) {
						var searchTerm = event.detail.value;
						if (searchTerm && searchTerm.length >= 2) {
							loadChoices(searchTerm);
						} else if (searchTerm.length === 0) {
							// Load initial options when search is cleared
							loadChoices('');
						}
					});

					// Load initial options on first open
					var initialLoadDone = false;
					element.addEventListener('showDropdown', function() {
						if (!initialLoadDone) {
							loadChoices('');
							initialLoadDone = true;
						}
					});

					// Load selected value if it exists
					if (selectedValue && selectedValue !== '') {
						fetch(ajaxUrl, {
							method: 'POST',
							headers: {
								'Content-Type': 'application/x-www-form-urlencoded',
								'X-CSRF-Token': csrfToken
							},
							body: 'id=' + encodeURIComponent(selectedValue)
						})
						.then(response => response.json())
						.then(data => {
							if (data.results && data.results.length > 0) {
								var selectedItem = data.results[0];
								
								// First add the selected item as a choice if it's not already there
								var existingChoice = choices._store.choices.find(function(choice) {
									return choice.value == selectedItem.id;
								});
								
								if (!existingChoice) {
									choices.setChoices([{
										value: selectedItem.id,
										label: selectedItem.text,
										selected: true
									}], 'value', 'label', false);
								} else {
									// If choice exists, just select it
									choices.setChoiceByValue(selectedItem.id.toString());
								}
								// Ensure dropdown stays enabled after loading selected value
								choices.enable();
							}
						})
						.catch(error => {
							console.error('Error loading selected value:', error);
							choices.enable(); // Ensure it stays enabled even on error
						});
					}
				});
			};
			
			// Centralized DataTable initialization function
			window.initDataTable = function(selector, options) {
				console.log('initDataTable called with:', selector, options);
				// Check if DataTables is available
				if (!$.fn.DataTable) {
					console.warn('DataTables not available for selector:', selector);
					return null;
				}
				
				// Default configuration
				var defaultConfig = {
					pageLength: 25,
					responsive: true,
					destroy: true,
					searching: true,
					ordering: true,
					paging: true,
					language: {
						search: "Filter:",
						lengthMenu: "Show _MENU_ entries per page",
						info: "Showing _START_ to _END_ of _TOTAL_ entries",
						zeroRecords: "No matching records found",
						emptyTable: "No data available in table"
					}
				};
				
				// Server-side specific defaults
				if (options && options.serverSide) {
					defaultConfig.processing = true;
					defaultConfig.serverSide = true;
					defaultConfig.order = [[ 0, "desc" ]];
				}
				
				// Merge provided options with defaults
				var config = $.extend(true, {}, defaultConfig, options || {});
				
				// Initialize DataTable
				var table = $(selector).DataTable(config);
				
				return table;
			};
			
			// Initialize Choices.js when Add Record Modal is shown
			$(document).on('shown.bs.modal', '#addRecordModal', function() {
				setTimeout(function() {
					if (window.initChoicesGlobal) {
						window.initChoicesGlobal();
					}
				}, 100);
			});
			
			// Initialize Choices.js when Edit Form Modal is shown  
			$(document).on('shown.bs.modal', '#editFormModal', function() {
				setTimeout(function() {
					if (window.initChoicesGlobal) {
						window.initChoicesGlobal();
					}
				}, 100);
			});
			
			// Enhanced form submission handling for tools sidebar
			document.addEventListener('DOMContentLoaded', function() {
				// Ensure form submissions work with both form-group and form-group-enhanced
				const forms = document.querySelectorAll('form');
				forms.forEach(function(form) {
					form.addEventListener('submit', function(e) {
						// Find all checkboxes in the form (both regular and modern)
						const checkboxes = form.querySelectorAll('input[type="checkbox"]');
						checkboxes.forEach(function(checkbox) {
							// Ensure checkbox values are properly serialized
							if (checkbox.checked && !checkbox.value) {
								checkbox.value = 'on';
							}
						});
					});
				});
				
				// Initialize Choices.js on page load
				if (window.initChoicesGlobal) {
					window.initChoicesGlobal();
				}
				
				// Add current page highlighting
				const currentPath = window.location.pathname;
				document.querySelectorAll('.sidebar-link[href]').forEach(function(link) {
					if (link.getAttribute('href') === currentPath) {
						const parentItem = link.closest('.sidebar-item');
						if (parentItem) {
							parentItem.classList.add('current');
							// Expand parent dropdown if this is a sub-item
							const parentDropdown = parentItem.closest('.sidebar-dropdown');
							if (parentDropdown) {
								parentDropdown.classList.add('show');
								const parentToggle = document.querySelector('[data-bs-target="#' + parentDropdown.id + '"]');
								if (parentToggle) parentToggle.classList.remove('collapsed');
							}
						}
					}
				});
			});
		`

	return html.Script(gomponents.Raw(jsContent))
}

// getDropdownOptionByID retrieves a single dropdown option by ID for preselection.
func getDropdownOptionByID(tableName, fieldName string, id *int) *map[string]any {
	switch tableName {
	case "dbmovies":
		if movie, err := database.Structscan[database.Dbmovie](
			"SELECT id, title FROM dbmovies WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(movie.ID, movie.Title)
		}

	case "dbseries":
		if serie, err := database.Structscan[database.Dbserie](
			"SELECT id, seriename FROM dbseries WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(serie.ID, serie.Seriename)
		}

	case "dbserie_episodes":
		if episode, err := database.Structscan[database.DbserieEpisode](
			"SELECT id, identifier, title FROM dbserie_episodes WHERE id = ?",
			false,
			id,
		); err == nil {
			label := fmt.Sprintf("%s - %s", episode.Identifier, episode.Title)
			return createSelect2OptionPtr(episode.ID, label)
		}

	case "movies":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbmovies.title || ' - ' || movies.listname, movies.id FROM movies LEFT JOIN dbmovies ON movies.dbmovie_id = dbmovies.id WHERE movies.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "series":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbseries.seriename || ' - ' || series.listname, series.id FROM series LEFT JOIN dbseries ON series.dbserie_id = dbseries.id WHERE series.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "serie_episodes":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT COALESCE(dbseries.seriename, 'Unknown Series') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.identifier IS NOT NULL AND dbserie_episodes.identifier != 'S00E00' THEN dbserie_episodes.identifier ELSE 'ID:' || serie_episodes.id END, 'Unknown') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.title IS NOT NULL AND TRIM(dbserie_episodes.title) != '' THEN dbserie_episodes.title ELSE 'Episode ' || COALESCE(dbserie_episodes.episode, 'Unknown') END, 'Unknown Episode') || ' (' || series.listname || ')', serie_episodes.id FROM serie_episodes LEFT JOIN dbserie_episodes ON serie_episodes.dbserie_episode_id = dbserie_episodes.id LEFT JOIN series ON serie_episodes.serie_id = series.id LEFT JOIN dbseries ON series.dbserie_id = dbseries.id WHERE serie_episodes.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "qualities":
		// Determine quality type based on field name
		var typeFilter string
		switch fieldName {
		case "resolution_id":
			typeFilter = " AND type = 1"
		case "quality_id":
			typeFilter = " AND type = 2"
		case "codec_id":
			typeFilter = " AND type = 3"
		case "audio_id":
			typeFilter = " AND type = 4"
		default:
			typeFilter = ""
		}

		query := fmt.Sprintf("SELECT id, name FROM qualities WHERE id = ?%s", typeFilter)
		if quality, err := database.Structscan[database.Qualities](query, false, id); err == nil {
			return createSelect2OptionPtr(quality.ID, quality.Name)
		}

	// Book tables
	case "dbbooks":
		if book, err := database.Structscan[database.Dbbook](
			"SELECT id, title FROM dbbooks WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(book.ID, book.Title)
		}

	case "dbauthors":
		if author, err := database.Structscan[database.Dbauthor](
			"SELECT id, name FROM dbauthors WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(author.ID, author.Name)
		}

	case "books":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT COALESCE((SELECT au.name FROM dbauthors au WHERE au.id = dbbooks.dbauthor_id) || ' - ', '') || dbbooks.title || ' - ' || books.listname, books.id FROM books LEFT JOIN dbbooks ON books.dbbook_id = dbbooks.id WHERE books.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	// Audiobook tables
	case "dbaudiobooks":
		if audiobook, err := database.Structscan[database.Dbaudiobook](
			"SELECT id, title FROM dbaudiobooks WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(audiobook.ID, audiobook.Title)
		}

	case "dbnarrators":
		if narrator, err := database.Structscan[database.Dbnarrator](
			"SELECT id, name FROM dbnarrators WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(narrator.ID, narrator.Name)
		}

	case "audiobooks":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT COALESCE((SELECT au.name FROM dbaudiobook_authors aba JOIN dbauthors au ON au.id = aba.dbauthor_id WHERE aba.dbaudiobook_id = dbaudiobooks.id LIMIT 1) || ' - ', '') || dbaudiobooks.title || ' - ' || audiobooks.listname, audiobooks.id FROM audiobooks LEFT JOIN dbaudiobooks ON audiobooks.dbaudiobook_id = dbaudiobooks.id WHERE audiobooks.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	// Music tables
	case "dbalbums":
		if album, err := database.Structscan[database.Dbalbum](
			"SELECT id, title FROM dbalbums WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(album.ID, album.Title)
		}

	case "dbartists":
		if artist, err := database.Structscan[database.Dbartist](
			"SELECT id, name FROM dbartists WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(artist.ID, artist.Name)
		}

	case "albums":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT COALESCE((SELECT ar.name FROM dbalbum_artists aa JOIN dbartists ar ON ar.id = aa.dbartist_id WHERE aa.dbalbum_id = dbalbums.id LIMIT 1) || ' - ', '') || dbalbums.title || ' - ' || albums.listname, albums.id FROM albums LEFT JOIN dbalbums ON albums.dbalbum_id = dbalbums.id WHERE albums.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	// Alternative title tables
	case "dbbook_titles":
		if title, err := database.Structscan[database.DbbookTitle](
			"SELECT id, title FROM dbbook_titles WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(title.ID, title.Title)
		}

	case "dbaudiobook_titles":
		if title, err := database.Structscan[database.DbaudiobookTitle](
			"SELECT id, title FROM dbaudiobook_titles WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(title.ID, title.Title)
		}

	case "dbalbum_titles":
		if title, err := database.Structscan[database.DbalbumTitle](
			"SELECT id, title FROM dbalbum_titles WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(title.ID, title.Title)
		}

	// Series and track tables
	case "dbbook_series":
		if series, err := database.Structscan[database.DbbookSeries](
			"SELECT id, name FROM dbbook_series WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(series.ID, series.Name)
		}

	case "book_series":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbbook_series.name || ' - ' || book_series.listname, book_series.id FROM book_series LEFT JOIN dbbook_series ON book_series.dbbook_series_id = dbbook_series.id WHERE book_series.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "dbtracks":
		if track, err := database.Structscan[database.Dbtrack](
			"SELECT id, title FROM dbtracks WHERE id = ?",
			false,
			id,
		); err == nil {
			return createSelect2OptionPtr(track.ID, track.Title)
		}

	// Tracked authors and artists
	case "authors":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbauthors.name || ' - ' || authors.listname, authors.id FROM authors LEFT JOIN dbauthors ON authors.dbauthor_id = dbauthors.id WHERE authors.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}

	case "artists":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](
			false,
			1,
			"SELECT dbartists.name || ' - ' || artists.listname, artists.id FROM artists LEFT JOIN dbartists ON artists.dbartist_id = dbartists.id WHERE artists.id = ?",
			id,
		)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}
	}

	return nil
}

func renderTableEditForm(
	table string,
	data map[string]any,
	id string,
	csrfToken string,
) gomponents.Node {
	formNodes := []gomponents.Node{
		html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
	}

	// logger.Logtype("info", 1).Any("csrf", csrfToken).Msg("testtable")

	// Get table columns for displaynames - use form-specific columns to exclude joined columns
	tableColumns := getAdminFormColumns(table)

	columnMap := make(map[string]string)
	for _, col := range tableColumns {
		cleanName := col.Name
		if strings.Contains(col.Name, " as ") {
			cleanName = strings.Split(col.Name, " as ")[1]
		}

		columnMap[cleanName] = col.DisplayName
	}

	// Helper function to get display name
	getColumnDisplayName := func(fieldName string) string {
		if displayName, exists := columnMap[fieldName]; exists {
			return displayName
		}

		// Fallback to formatted field name
		parts := strings.Split(fieldName, "_")

		var capitalizedParts []string
		for _, part := range parts {
			if len(part) > 0 {
				capitalizedParts = append(capitalizedParts, strings.ToTitle(strings.ToLower(part)))
			}
		}

		return strings.Join(capitalizedParts, " ")
	}
	// Sort keys to ensure consistent field ordering
	sortedKeys := make([]string, 0, len(data))
	for col := range data {
		sortedKeys = append(sortedKeys, col)
	}

	sort.Strings(sortedKeys)

	for _, col := range sortedKeys {
		fieldData := data[col]

		// Skip readonly fields entirely - don't include them in forms
		if col == "id" || col == "created_at" || col == "updated_at" || col == "lastscan" {
			continue
		}

		fieldName := "field-" + col
		displayName := getColumnDisplayName(col)

		// Generate field based on type and special cases
		formNodes = append(formNodes, generateFormField(col, fieldName, displayName, fieldData))
	}

	// Add save button to form
	formNodes = append(formNodes,
		html.Div(
			html.Class("mt-3"),
			html.Button(
				html.Type("submit"),
				html.Class("btn btn-primary me-2"),
				gomponents.Text("Save Changes"),
			),
			html.Button(
				html.Type("button"),
				html.Class("btn btn-secondary"),
				html.Data("bs-dismiss", "modal"),
				gomponents.Text("Cancel"),
			),
		),
	)

	// Determine form title and action based on whether we're adding or editing
	var (
		formTitle  string
		formAction string
	)

	if id == "new" {
		formTitle = "Add New Row"
		formAction = "/api/admin/table/" + table + "/insert"
	} else {
		formTitle = "Edit Row"
		formAction = "/api/admin/table/" + table + "/update/" + id
	}

	return html.Div(
		html.Class("edit-form-container"),
		// Enhanced form header
		html.Div(
			html.Class("edit-form-header"),
			html.H2(html.Class("edit-form-title"), gomponents.Text(formTitle)),
			html.P(
				html.Class("edit-form-subtitle"),
				gomponents.Text("Complete the form fields below and save your changes"),
			),
		),
		// Enhanced form with modern styling
		html.Form(
			html.Method("post"),
			html.Action(formAction),
			html.ID("editForm"),
			html.Class("edit-form-modern"),

			// Form fields container
			html.Div(
				html.Class("edit-form-fields"),
				gomponents.Group(formNodes),
			),

			// Form actions - moved to modal footer
			addEditFormJavascript(),
		),
	)
}

func addEditFormJavascript() gomponents.Node {
	return html.Script(gomponents.Raw(`
				// Select2 initialization is now handled by the global initSelect2Global function
				
				// Dropdown initialization is now handled by initChoicesGlobal (below)
				// Use class 'choices-ajax' with data-allow-custom="true" for custom value support
				
				var editForm = document.getElementById('editForm');
				if (editForm) {
					editForm.addEventListener('submit', function(e) {
					e.preventDefault();
					var formData = new FormData(this);
					
					// Convert form data to URL-encoded format
					var params = new URLSearchParams();
					for (var pair of formData.entries()) {
						params.append(pair[0], pair[1]);
					}
					
					// Get CSRF token from form
					var csrfToken = this.querySelector('input[name="csrf_token"]').value;
					
					fetch(this.action, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/x-www-form-urlencoded',
							'X-CSRF-Token': csrfToken,
						},
						body: params.toString()
					})
					.then(response => {
						return response.json();
					})
					.then(data => {
						// Find which modal this form is in and close it
						var $modal = $(this).closest('.modal');
						if ($modal.length) {
							$modal.modal('hide');
						}
						
						// Reload the table
						if (typeof oTable !== 'undefined') {
							oTable.ajax.reload();
						}
						
						// Show toaster notification - handle both formats
						if (data.success === true || data.success === "true") {
							if (typeof showToaster === 'function') {
								showToaster('success', 'Record saved successfully');
							}
						} else {
							if (typeof showToaster === 'function') {
								showToaster('error', data.error || 'Error saving record');
							}
						}
					})
					.catch(error => {
						console.error('Fetch error:', error);
						if (typeof showToaster === 'function') {
							showToaster('error', 'Network error: ' + error.message);
						} else {
							console.error('showToaster function not available');
							alert('Network error: ' + error.message);
						}
					});
					});
				}
			`))
}

func renderTable(tableInfo *TableInfo, csrfToken string) gomponents.Node {
	header := make([]gomponents.Node, 0, len(tableInfo.Columns)+1)
	aliases := make([]string, 0, len(tableInfo.Columns))

	for _, col := range tableInfo.Columns {
		var addnode gomponents.Node

		// Extract the alias from "table.column as alias" format for display
		columnAlias := col.Name
		if strings.Contains(strings.ToLower(col.Name), " as ") {
			parts := strings.Split(col.Name, " as ")
			if len(parts) == 2 {
				columnAlias = strings.TrimSpace(parts[1])
			}
		}

		// Check the alias (not the full col.Name) for priority assignment
		switch columnAlias {
		case "id":
			addnode = html.Data("priority", "1")
		case "title", "seriename", "name", "identifier", "listname", "filename", "year":
			addnode = html.Data("priority", "1")

		case "created_at", "updated_at", "release_date", "first_aired", "overview":
			addnode = html.Data("priority", "100000")
		}

		aliases = append(aliases, columnAlias)

		header = append(
			header,
			html.Th(
				html.Class("sorting"),
				html.Role("columnheader"),
				html.Data("column-name", columnAlias),
				addnode,
				gomponents.Text(col.DisplayName),
			),
		)
	}

	// Add Actions column header
	header = append(
		header,
		html.Th(
			html.Role("columnheader"),
			html.Data("priority", "2"),
			html.Data("sortable", "false"),
			html.Data("orderable", "false"),
			gomponents.Text("Actions"),
		),
	)

	return gomponents.Group(
		[]gomponents.Node{
			html.Div(html.Class("datatables-reponsive_wrapper"),
				html.Table(
					html.ID("table-data"),
					html.Class("table table-striped datatable"),
					html.THead(
						html.Tr(
							header...,
						),
					),
				),
				tableScripts(tableInfo, csrfToken, aliases),
			),
		})
}

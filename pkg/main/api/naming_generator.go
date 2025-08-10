package api

import (
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

type TemplateField struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Example     string `json:"example"`
	Template    string `json:"template"`
	Category    string `json:"category"`
}

type TemplateStructure struct {
	Name        string `json:"name"`
	Template    string `json:"template"`
	Description string `json:"description"`
	Example     string `json:"example"`
}

type NamingFieldsResponse struct {
	Fields     []TemplateField     `json:"fields"`
	Structures []TemplateStructure `json:"structures"`
	DataType   string              `json:"data_type"`
}

func renderNamingGeneratorPage(csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-code header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Advanced Template Generator")),
					html.P(html.Class("header-subtitle"),
						gomponents.Text("Generate Go templates with drag-and-drop fields and template structures")),
				),
			),
		),

		// Data Type Selection
		html.Div(
			html.Class("row mb-4"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #0d6efd 0%, #0056b3 100%); color: white; padding: 1.5rem;"),
						html.H5(html.Class("card-title mb-0"), html.Style("font-weight: 600;"),
							gomponents.Text("Data Source Selection")),
					),
					html.Div(
						html.Class("card-body p-4"),
						html.Div(html.Class("form-group mb-4"),
							html.Label(html.Class("form-label fw-semibold"),
								gomponents.Text("Select Data Type")),
							html.Select(
								html.Class("form-select form-select-lg"),
								html.ID("dataTypeSelector"),
								html.Option(html.Value(""), gomponents.Text("Choose data structure to work with")),
								html.Option(html.Value("parser"), gomponents.Text("Parser Type - Media file parsing and naming templates")),
								html.Option(html.Value("notification"), gomponents.Text("Notification Type - Media processing notifications")),
							),
							html.Small(html.Class("form-text text-muted"),
								gomponents.Text("Different data types provide different available fields")),
						),
					),
				),
			),
		),

		html.Div(
			html.Class("row mb-4"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #0d6efd 0%, #0056b3 100%); color: white; padding: 1.5rem;"),
						html.H5(html.Class("card-title mb-0"), html.Style("font-weight: 600;"),
							gomponents.Text("Examples")),
					),
					html.Div(
						html.Class("card-body p-4"),
						html.Div(html.Class("form-group mb-4"),
							gomponents.Text(`Parser Type: {{.Dbserie.Seriename}}/Season {{.DbserieEpisode.Season}}/{{.Dbserie.Seriename}} - S{{printf "%02s" .DbserieEpisode.Season}}{{range .Episodes}}E{{printf "%02d" . }}{{end}} - {{.DbserieEpisode.Title}} [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}] ({{.Source.Tvdb}})`),
							html.Br(),
							gomponents.Text(`Parser Type: {{.Dbserie.Seriename}}/{{.Dbserie.Seriename}} {{.DbserieEpisode.Identifier}} {{.EpisodeTitleSource}} - [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}]`),
							html.Br(),
							gomponents.Text(`Parser Type: {{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}{{if eq .Source.Extended true}} extended{{end}}] ({{.Source.Imdb}})`),
							html.Br(),
							gomponents.Text(`Notification Type: {{.Time}};{{.Title}};{{.Identifier}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}`),
							html.Br(),
							gomponents.Text(`Notification Type: {{.Title}} - moved from {{.SourcePath}} to {{.Targetpath}}{{if .Replaced }} Replaced: {{ range .Replaced }}{{.}},{{end}}{{end}}`),
							html.Br(),
						),
					),
				),
			),
		),

		// Template Builder Area
		html.Div(
			html.Class("row"),
			// Available Fields Panel
			html.Div(
				html.Class("col-md-4"),
				html.Div(
					html.Class("card border-0 shadow-sm h-100"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #28a745 0%, #20c997 100%); color: white; padding: 1rem;"),
						html.H6(html.Class("card-title mb-2"), html.Style("font-weight: 600;"),
							gomponents.Text("Available Fields & Structures")),
						html.Div(
							html.Class("input-group input-group-sm"),
							html.Input(
								html.Class("form-control"),
								html.Type("text"),
								html.ID("fieldSearch"),
								html.Placeholder("Search fields..."),
								html.Style("background: rgba(255,255,255,0.9); border: none;"),
							),
							html.Div(
								html.Class("input-group-text"),
								html.Style("background: rgba(255,255,255,0.9); border: none;"),
								html.I(html.Class("fas fa-search text-muted")),
							),
						),
					),
					html.Div(
						html.Class("card-body p-3"),
						html.Div(html.ID("fieldsContainer"),
							html.P(html.Class("text-muted text-center mt-4"),
								gomponents.Text("Select a data type to see available fields")),
						),
					),
				),
			),

			// Template Editor Panel
			html.Div(
				html.Class("col-md-8"),
				html.Div(
					html.Class("card border-0 shadow-sm h-100"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0 d-flex justify-content-between align-items-center"),
						html.Style("background: linear-gradient(135deg, #6f42c1 0%, #5a2d91 100%); color: white; padding: 1rem;"),
						html.H6(html.Class("card-title mb-0"), html.Style("font-weight: 600;"),
							gomponents.Text("Template Editor")),
						html.Div(html.Class("btn-group btn-group-sm"),
							html.Button(
								html.Class("btn btn-outline-light btn-sm"),
								html.ID("clearTemplate"),
								html.Type("button"),
								html.I(html.Class("fas fa-trash me-1")),
								gomponents.Text("Clear"),
							),
							html.Button(
								html.Class("btn btn-outline-light btn-sm"),
								html.ID("copyTemplate"),
								html.Type("button"),
								html.I(html.Class("fas fa-copy me-1")),
								gomponents.Text("Copy"),
							),
						),
					),
					html.Div(
						html.Class("card-body p-0"),
						html.Textarea(
							html.Class("form-control"),
							html.ID("templateEditor"),
							html.Style("height: 400px; border: none; font-family: 'Monaco', 'Consolas', 'Courier New', monospace; font-size: 14px; resize: vertical;"),
							html.Placeholder("Build your Go template here...\n\nDrag fields from the left panel or click to insert.\nUse template structures for conditional logic and loops."),
						),
						html.Div(
							html.Class("p-3 border-top bg-light"),
							html.Div(html.Class("d-flex justify-content-between align-items-center"),
								html.Div(html.Class("btn-group"),
									html.Button(
										html.Class("btn btn-success"),
										html.ID("previewTemplate"),
										html.Type("button"),
										html.I(html.Class("fas fa-eye me-2")),
										gomponents.Text("Preview Output"),
									),
									html.Button(
										html.Class("btn btn-info"),
										html.ID("verifyTemplate"),
										html.Type("button"),
										html.I(html.Class("fas fa-check-circle me-2")),
										gomponents.Text("Verify Template"),
									),
								),
								html.Small(html.Class("text-muted"),
									gomponents.Text("Drag fields from left panel or click to insert • Use Ctrl+A to select all")),
							),
						),
					),
				),
			),
		),

		// Preview Section
		html.Div(
			html.Class("row mt-4"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.ID("previewSection"),
					html.Class("card border-0 shadow-sm d-none"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #fd7e14 0%, #e55a2b 100%); color: white; padding: 1rem;"),
						html.H6(html.Class("card-title mb-0"), html.Style("font-weight: 600;"),
							gomponents.Text("Template Preview")),
					),
					html.Div(
						html.Class("card-body p-3"),
						html.Div(html.ID("previewContent")),
					),
				),
			),
		),

		// JavaScript for advanced functionality
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				var fieldData = {};
				var cursorPosition = 0;

				// Track cursor position in textarea
				$('#templateEditor').on('click keyup', function() {
					cursorPosition = this.selectionStart;
				});

				// Data type selector change
				$('#dataTypeSelector').on('change', function() {
					var dataType = $(this).val();
					if (dataType) {
						loadFieldsForDataType(dataType);
					} else {
						$('#fieldsContainer').html('<p class="text-muted text-center mt-4">Select a data type to see available fields</p>');
					}
				});

				function loadFieldsForDataType(dataType) {
					// Show loading
					$('#fieldsContainer').html('<div class="text-center mt-4"><i class="fas fa-spinner fa-spin"></i> Loading fields...</div>');
					
					// Fetch fields via AJAX
					$.ajax({
						url: '/api/admin/naming-generator/fields/' + dataType,
						method: 'GET',
						success: function(response) {
							fieldData = response;
							renderFieldsPanel(response);
						},
						error: function() {
							$('#fieldsContainer').html('<div class="alert alert-danger">Failed to load fields</div>');
						}
					});
				}

				function renderFieldsPanel(data) {
					var html = '<div class="accordion" id="fieldsAccordion">';
					var accordionId = 0;
					
					// Template Structures Section
					if (data.structures && data.structures.length > 0) {
						html += '<div class="accordion-item border-0 mb-2">';
						html += '<h6 class="accordion-header" id="heading-structures">';
						html += '<button class="accordion-button collapsed bg-light border-0" type="button" data-bs-toggle="collapse" data-bs-target="#collapse-structures" aria-expanded="false" aria-controls="collapse-structures" style="padding: 0.75rem; font-size: 0.9rem; font-weight: 600;">';
						html += '<i class="fas fa-code-branch me-2 text-info"></i>Template Structures';
						html += '</button>';
						html += '</h6>';
						html += '<div id="collapse-structures" class="accordion-collapse collapse" aria-labelledby="heading-structures" data-bs-parent="#fieldsAccordion">';
						html += '<div class="accordion-body p-2">';
						data.structures.forEach(function(structure) {
							html += '<div class="field-item structure-item mb-2" data-template="' + escapeHtml(structure.template) + '">';
							html += '<div class="d-flex justify-content-between align-items-center">';
							html += '<div>';
							html += '<strong class="text-info">' + structure.name + '</strong>';
							html += '<br><small class="text-muted">' + structure.description + '</small>';
							html += '</div>';
							html += '<button class="btn btn-outline-info btn-sm" onclick="insertAtCursor(\'' + structure.template.replace(/'/g, '\\\'') + '\')"><i class="fas fa-plus"></i></button>';
							html += '</div>';
							html += '</div>';
						});
						html += '</div>';
						html += '</div>';
						html += '</div>';
					}

					// Group fields by category
					var categories = {};
					data.fields.forEach(function(field) {
						if (!categories[field.category]) {
							categories[field.category] = [];
						}
						categories[field.category].push(field);
					});

					// Render field categories as collapsible accordion items (all collapsed by default)
					Object.keys(categories).forEach(function(category, index) {
						var categoryId = 'category-' + accordionId++;
						
						html += '<div class="accordion-item border-0 mb-2">';
						html += '<h6 class="accordion-header" id="heading-' + categoryId + '">';
						html += '<button class="accordion-button collapsed bg-light border-0" type="button" data-bs-toggle="collapse" data-bs-target="#collapse-' + categoryId + '" aria-expanded="false" aria-controls="collapse-' + categoryId + '" style="padding: 0.75rem; font-size: 0.9rem; font-weight: 600;">';
						html += getCategoryIcon(category) + ' ' + category;
						html += '</button>';
						html += '</h6>';
						html += '<div id="collapse-' + categoryId + '" class="accordion-collapse collapse" aria-labelledby="heading-' + categoryId + '" data-bs-parent="#fieldsAccordion">';
						html += '<div class="accordion-body p-2">';
						
						categories[category].forEach(function(field) {
							html += '<div class="field-item mb-2" data-template="' + escapeHtml(field.template) + '" data-field-name="' + field.name.toLowerCase() + '" data-field-description="' + field.description.toLowerCase() + '" data-category="' + category.toLowerCase() + '">';
							html += '<div class="d-flex justify-content-between align-items-center">';
							html += '<div>';
							html += '<strong>' + field.name + '</strong>';
							if (field.type) {
								html += ' <span class="badge bg-secondary">' + field.type + '</span>';
							}
							html += '<br><small class="text-muted">' + field.description + '</small>';
							if (field.example) {
								html += '<br><small class="text-success">Ex: ' + field.example + '</small>';
							}
							html += '</div>';
							html += '<button class="btn btn-outline-primary btn-sm" onclick="insertAtCursor(\'' + field.template.replace(/'/g, '\\\'') + '\')"><i class="fas fa-plus"></i></button>';
							html += '</div>';
							html += '</div>';
						});
						html += '</div>';
						html += '</div>';
						html += '</div>';
					});

					html += '</div>'; // Close accordion
					$('#fieldsContainer').html(html);
					
					// Initialize search functionality
					setupFieldSearch();
				}

				function getCategoryIcon(category) {
					var icons = {
						'Basic Information': '<i class="fas fa-info-circle"></i>',
						'Media Details': '<i class="fas fa-film"></i>',
						'File Information': '<i class="fas fa-file"></i>',
						'Dates & Times': '<i class="fas fa-clock"></i>',
						'Quality & Resolution': '<i class="fas fa-star"></i>',
						'External IDs': '<i class="fas fa-link"></i>',
						'Arrays & Lists': '<i class="fas fa-list"></i>',
						'Paths & Locations': '<i class="fas fa-folder"></i>'
					};
					return icons[category] || '<i class="fas fa-tag"></i>';
				}
				
				function setupFieldSearch() {
					var searchInput = document.getElementById('fieldSearch');
					if (!searchInput) return;
					
					searchInput.addEventListener('input', function() {
						var searchTerm = this.value.toLowerCase().trim();
						var fieldsContainer = document.getElementById('fieldsContainer');
						var accordionItems = fieldsContainer.querySelectorAll('.accordion-item');
						var hasVisibleResults = false;
						
						// If search is empty, collapse all sections
						if (!searchTerm) {
							accordionItems.forEach(function(item) {
								var collapse = item.querySelector('.accordion-collapse');
								if (collapse && collapse.classList.contains('show')) {
									collapse.classList.remove('show');
									var button = item.querySelector('.accordion-button');
									if (button) {
										button.classList.add('collapsed');
										button.setAttribute('aria-expanded', 'false');
									}
								}
								// Show all field items
								var fieldItems = item.querySelectorAll('.field-item');
								fieldItems.forEach(function(fieldItem) {
									fieldItem.style.display = 'block';
								});
							});
							return;
						}
						
						// Search through each accordion section
						accordionItems.forEach(function(item) {
							var fieldItems = item.querySelectorAll('.field-item');
							var hasMatches = false;
							
							// Check each field in this section
							fieldItems.forEach(function(fieldItem) {
								var fieldName = fieldItem.getAttribute('data-field-name') || '';
								var fieldDescription = fieldItem.getAttribute('data-field-description') || '';
								var category = fieldItem.getAttribute('data-category') || '';
								
								var matches = fieldName.includes(searchTerm) || 
										fieldDescription.includes(searchTerm) || 
										category.includes(searchTerm);
								
								if (matches) {
									fieldItem.style.display = 'block';
									hasMatches = true;
									hasVisibleResults = true;
								} else {
									fieldItem.style.display = 'none';
								}
							});
							
							// Auto-expand sections with matches, collapse others
							var collapse = item.querySelector('.accordion-collapse');
							var button = item.querySelector('.accordion-button');
							
							if (hasMatches) {
								if (collapse && !collapse.classList.contains('show')) {
									collapse.classList.add('show');
									if (button) {
										button.classList.remove('collapsed');
										button.setAttribute('aria-expanded', 'true');
									}
								}
							} else {
								if (collapse && collapse.classList.contains('show')) {
									collapse.classList.remove('show');
									if (button) {
										button.classList.add('collapsed');
										button.setAttribute('aria-expanded', 'false');
									}
								}
							}
						});
						
						// Show "no results" message if needed
						if (!hasVisibleResults && searchTerm) {
							var noResults = fieldsContainer.querySelector('.no-results-message');
							if (!noResults) {
								noResults = document.createElement('div');
								noResults.className = 'no-results-message alert alert-info text-center mt-3';
								noResults.innerHTML = '<i class="fas fa-search me-2"></i>No fields found matching "' + searchTerm + '"';
								fieldsContainer.appendChild(noResults);
							} else {
								noResults.innerHTML = '<i class="fas fa-search me-2"></i>No fields found matching "' + searchTerm + '"';
								noResults.style.display = 'block';
							}
						} else {
							var noResults = fieldsContainer.querySelector('.no-results-message');
							if (noResults) {
								noResults.style.display = 'none';
							}
						}
					});
					
					// Clear search when ESC is pressed
					searchInput.addEventListener('keydown', function(e) {
						if (e.key === 'Escape') {
							this.value = '';
							this.dispatchEvent(new Event('input'));
						}
					});
				}

				// Insert at cursor position
				window.insertAtCursor = function(text) {
					var textarea = document.getElementById('templateEditor');
					var value = textarea.value;
					var start = textarea.selectionStart;
					var end = textarea.selectionEnd;
					
					textarea.value = value.substring(0, start) + text + value.substring(end);
					textarea.focus();
					textarea.setSelectionRange(start + text.length, start + text.length);
				};

				// Clear template
				$('#clearTemplate').on('click', function() {
					if (confirm('Are you sure you want to clear the template?')) {
						$('#templateEditor').val('').focus();
					}
				});

				// Copy template
				$('#copyTemplate').on('click', function() {
					var template = $('#templateEditor').val();
					if (template.trim() === '') {
						showToaster('warning', 'Template is empty');
						return;
					}
					
					navigator.clipboard.writeText(template).then(function() {
						showToaster('success', 'Template copied to clipboard!');
					}, function(err) {
						showToaster('error', 'Failed to copy template');
					});
				});

				// Preview template
				$('#previewTemplate').on('click', function() {
					var template = $('#templateEditor').val();
					var dataType = $('#dataTypeSelector').val();
					
					if (template.trim() === '') {
						showToaster('warning', 'Please enter a template to preview');
						return;
					}
					
					if (!dataType) {
						showToaster('warning', 'Please select a data type');
						return;
					}

					$.ajax({
						url: '/api/admin/naming-generator/preview',
						method: 'POST',
						data: {
							template: template,
							data_type: dataType,
							csrf_token: $('input[name="csrf_token"]').val()
						},
						success: function(response) {
							displayPreview(response);
						},
						error: function(xhr) {
							var error = 'Preview failed';
							if (xhr.responseJSON && xhr.responseJSON.error) {
								error = xhr.responseJSON.error;
							}
							showToaster('error', error);
						}
					});
				});

				// Verify template
				$('#verifyTemplate').on('click', function() {
					var template = $('#templateEditor').val();
					var dataType = $('#dataTypeSelector').val();
					
					if (template.trim() === '') {
						showToaster('warning', 'Please enter a template to verify');
						return;
					}
					
					if (!dataType) {
						showToaster('warning', 'Please select a data type');
						return;
					}

					$.ajax({
						url: '/api/admin/naming-generator/verify',
						method: 'POST',
						data: {
							template: template,
							data_type: dataType,
							csrf_token: $('input[name="csrf_token"]').val()
						},
						success: function(response) {
							displayVerification(response);
						},
						error: function(xhr) {
							var error = 'Verification failed';
							if (xhr.responseJSON && xhr.responseJSON.error) {
								error = xhr.responseJSON.error;
							}
							showToaster('error', error);
						}
					});
				});

				function displayPreview(response) {
					var html = '<div class="row">';
					
					// Template
					html += '<div class="col-md-6 mb-3">';
					html += '<h6 class="fw-bold mb-2"><i class="fas fa-code me-2"></i>Template</h6>';
					html += '<pre class="bg-light p-3 rounded" style="font-size: 12px;"><code>' + escapeHtml(response.template) + '</code></pre>';
					html += '</div>';
					
					// Preview Output
					html += '<div class="col-md-6 mb-3">';
					html += '<h6 class="fw-bold mb-2"><i class="fas fa-eye me-2"></i>Preview Output</h6>';
					if (response.error) {
						html += '<div class="alert alert-danger"><small>' + escapeHtml(response.error) + '</small></div>';
					} else {
						html += '<pre class="bg-success bg-opacity-10 border border-success border-opacity-25 p-3 rounded" style="font-size: 12px;"><code>' + escapeHtml(response.output) + '</code></pre>';
					}
					html += '</div>';
					
					html += '</div>';
					
					$('#previewContent').html(html);
					$('#previewSection').removeClass('d-none');
					
					// Scroll to preview
					$('html, body').animate({
						scrollTop: $('#previewSection').offset().top - 100
					}, 500);
				}

				function displayVerification(response) {
					var html = '<div class="row">';
					
					// Template
					html += '<div class="col-md-6 mb-3">';
					html += '<h6 class="fw-bold mb-2"><i class="fas fa-code me-2"></i>Template</h6>';
					html += '<pre class="bg-light p-3 rounded" style="font-size: 12px;"><code>' + escapeHtml(response.template) + '</code></pre>';
					html += '</div>';
					
					// Verification Results
					html += '<div class="col-md-6 mb-3">';
					html += '<h6 class="fw-bold mb-2"><i class="fas fa-check-circle me-2"></i>Verification Results</h6>';
					
					if (response.valid) {
						html += '<div class="alert alert-success">';
						html += '<i class="fas fa-check-circle me-2"></i>';
						html += '<strong>Template is valid!</strong><br>';
						html += '<small>' + response.message + '</small>';
						html += '</div>';
						
						if (response.suggestions && response.suggestions.length > 0) {
							html += '<div class="alert alert-info">';
							html += '<strong>Suggestions:</strong><br>';
							html += '<ul class="mb-0 mt-1">';
							response.suggestions.forEach(function(suggestion) {
								html += '<li><small>' + escapeHtml(suggestion) + '</small></li>';
							});
							html += '</ul>';
							html += '</div>';
						}
					} else {
						html += '<div class="alert alert-danger">';
						html += '<i class="fas fa-exclamation-triangle me-2"></i>';
						html += '<strong>Template has issues:</strong><br>';
						html += '<small>' + escapeHtml(response.message) + '</small>';
						html += '</div>';
						
						if (response.errors && response.errors.length > 0) {
							html += '<div class="alert alert-warning">';
							html += '<strong>Issues found:</strong><br>';
							html += '<ul class="mb-0 mt-1">';
							response.errors.forEach(function(error) {
								html += '<li><small>' + escapeHtml(error) + '</small></li>';
							});
							html += '</ul>';
							html += '</div>';
						}
					}
					
					html += '</div>';
					html += '</div>';
					
					$('#previewContent').html(html);
					$('#previewSection').removeClass('d-none');
					
					// Scroll to preview
					$('html, body').animate({
						scrollTop: $('#previewSection').offset().top - 100
					}, 500);
				}

				function escapeHtml(text) {
					var map = {
						'&': '&amp;',
						'<': '&lt;',
						'>': '&gt;',
						'"': '&quot;',
						"'": '&#039;'
					};
					return text.replace(/[&<>"']/g, function(m) { return map[m]; });
				}

				// CSRF token
				$('<input>').attr({
					type: 'hidden',
					name: 'csrf_token',
					value: '`+csrfToken+`'
				}).appendTo('body');
			});
		`)),
	)
}

func HandleNamingFieldsForType(ctx *gin.Context) {
	dataType := ctx.Param("type")
	if dataType == "" {
		sendJSONError(ctx, http.StatusBadRequest, "Data type is required")
		return
	}

	fields, structures := getFieldsForDataType(dataType)

	response := NamingFieldsResponse{
		Fields:     fields,
		Structures: structures,
		DataType:   dataType,
	}

	ctx.JSON(http.StatusOK, response)
}

func HandleNamingPreview(ctx *gin.Context) {
	template := strings.TrimSpace(ctx.PostForm("template"))
	dataType := strings.TrimSpace(ctx.PostForm("data_type"))

	if template == "" {
		sendJSONError(ctx, http.StatusBadRequest, "Template is required")
		return
	}

	if dataType == "" {
		sendJSONError(ctx, http.StatusBadRequest, "Data type is required")
		return
	}

	// Generate preview with sample data
	var outstr string
	var err error
	switch dataType {
	case "notification":
		{
			outstr, err = structure.TestInputnotifier(template)
		}
	case "parser":
		{
			outstr, err = structure.TestParsertype(template)
		}
	}

	response := map[string]interface{}{
		"template": template,
		"output":   outstr,
	}

	if err != nil {
		response["error"] = err.Error()
	}

	ctx.JSON(http.StatusOK, response)
}

func HandleNamingVerify(ctx *gin.Context) {
	template := strings.TrimSpace(ctx.PostForm("template"))
	dataType := strings.TrimSpace(ctx.PostForm("data_type"))

	if template == "" {
		sendJSONError(ctx, http.StatusBadRequest, "Template is required")
		return
	}

	if dataType == "" {
		sendJSONError(ctx, http.StatusBadRequest, "Data type is required")
		return
	}

	// Verify template syntax and provide suggestions
	verification := verifyGoTemplate(template, dataType)

	response := map[string]interface{}{
		"template": template,
		"valid":    verification.Valid,
		"message":  verification.Message,
	}

	if len(verification.Errors) > 0 {
		response["errors"] = verification.Errors
	}

	if len(verification.Suggestions) > 0 {
		response["suggestions"] = verification.Suggestions
	}

	ctx.JSON(http.StatusOK, response)
}

func getFieldsForDataType(dataType string) ([]TemplateField, []TemplateStructure) {
	structures := []TemplateStructure{
		{
			Name:        "If Statement",
			Template:    "{{if .FIELD}}...{{end}}",
			Description: "Conditional block - shows content only if field exists",
			Example:     "{{if .Dbmovie.Year}}Released in {{.Dbmovie.Year}}{{end}}",
		},
		{
			Name:        "If-Else Statement",
			Template:    "{{if .FIELD}}...{{else}}...{{end}}",
			Description: "Conditional with fallback - shows different content based on condition",
			Example:     "{{if .Dbmovie.Year}}{{.Dbmovie.Year}}{{else}}Unknown Year{{end}}",
		},
		{
			Name:        "Range Loop",
			Template:    "{{range .ARRAY}}{{.}}{{end}}",
			Description: "Loop through array/slice - repeats content for each item",
			Example:     "{{range .Replaced}}{{.}}, {{end}}",
		},
		{
			Name:        "Range with Index",
			Template:    "{{range $index, $element := .ARRAY}}{{$index}}: {{$element}}{{end}}",
			Description: "Loop with index access",
			Example:     "{{range $i, $file := .Replaced}}{{$i}}: {{$file}}{{end}}",
		},
		{
			Name:        "With Statement",
			Template:    "{{with .FIELD}}...{{end}}",
			Description: "Execute block only if field is not empty/nil",
			Example:     "{{with .Dbmovie.ImdbID}}IMDB: {{.}}{{end}}",
		},
		{
			Name:        "Variable Assignment",
			Template:    "{{$var := .FIELD}}{{$var}}",
			Description: "Assign field to variable for reuse",
			Example:     "{{$title := .Dbmovie.Title}}Title: {{$title}}",
		},
	}

	switch dataType {
	case "parser":
		return getParserFields(), structures
	case "notification":
		return getNotificationFields(), structures
	default:
		return []TemplateField{}, structures
	}
}

func getParserFields() []TemplateField {
	return []TemplateField{
		// Dbmovie fields (nested struct)
		{Name: "Dbmovie.Title", Type: "string", Template: "{{.Dbmovie.Title}}", Description: "Movie title", Example: "Inception", Category: "Movie Information"},
		{Name: "Dbmovie.Year", Type: "int", Template: "{{.Dbmovie.Year}}", Description: "Movie year", Example: "2000", Category: "Movie Information"},
		{Name: "Dbmovie.OriginalTitle", Type: "string", Template: "{{.Dbmovie.OriginalTitle}}", Description: "Original movie title", Example: "Inception", Category: "Movie Information"},
		{Name: "Dbmovie.Overview", Type: "string", Template: "{{.Dbmovie.Overview}}", Description: "Movie plot summary", Example: "A thief who steals secrets...", Category: "Movie Information"},
		{Name: "Dbmovie.Tagline", Type: "string", Template: "{{.Dbmovie.Tagline}}", Description: "Movie tagline", Example: "Your mind is the scene of the crime", Category: "Movie Information"},
		{Name: "Dbmovie.Genres", Type: "string", Template: "{{.Dbmovie.Genres}}", Description: "Movie genres", Example: "Action, Sci-Fi, Thriller", Category: "Movie Information"},
		{Name: "Dbmovie.Runtime", Type: "int", Template: "{{.Dbmovie.Runtime}}", Description: "Movie runtime in minutes", Example: "148", Category: "Movie Information"},
		{Name: "Dbmovie.ReleaseDate", Type: "time", Template: "{{.Dbmovie.ReleaseDate}}", Description: "Movie release date", Example: "2010-07-16", Category: "Movie Information"},
		{Name: "Dbmovie.Status", Type: "string", Template: "{{.Dbmovie.Status}}", Description: "Release status", Example: "Released", Category: "Movie Information"},
		{Name: "Dbmovie.OriginalLanguage", Type: "string", Template: "{{.Dbmovie.OriginalLanguage}}", Description: "Original language", Example: "en", Category: "Movie Information"},
		{Name: "Dbmovie.SpokenLanguages", Type: "string", Template: "{{.Dbmovie.SpokenLanguages}}", Description: "Spoken languages", Example: "English, Japanese", Category: "Movie Information"},
		{Name: "Dbmovie.ImdbID", Type: "string", Template: "{{.Dbmovie.ImdbID}}", Description: "IMDB ID", Example: "tt1375666", Category: "Movie External IDs"},
		{Name: "Dbmovie.MoviedbID", Type: "int", Template: "{{.Dbmovie.MoviedbID}}", Description: "MovieDB ID", Example: "27205", Category: "Movie External IDs"},
		{Name: "Dbmovie.TraktID", Type: "int", Template: "{{.Dbmovie.TraktID}}", Description: "Trakt ID", Example: "1390", Category: "Movie External IDs"},
		{Name: "Dbmovie.FacebookID", Type: "string", Template: "{{.Dbmovie.FacebookID}}", Description: "Facebook page ID", Example: "inception", Category: "Movie External IDs"},
		{Name: "Dbmovie.InstagramID", Type: "string", Template: "{{.Dbmovie.InstagramID}}", Description: "Instagram ID", Example: "inception", Category: "Movie External IDs"},
		{Name: "Dbmovie.TwitterID", Type: "string", Template: "{{.Dbmovie.TwitterID}}", Description: "Twitter ID", Example: "inception", Category: "Movie External IDs"},
		{Name: "Dbmovie.VoteAverage", Type: "float32", Template: "{{.Dbmovie.VoteAverage}}", Description: "Average user rating", Example: "8.3", Category: "Movie Ratings"},
		{Name: "Dbmovie.VoteCount", Type: "int32", Template: "{{.Dbmovie.VoteCount}}", Description: "Number of votes", Example: "31280", Category: "Movie Ratings"},
		{Name: "Dbmovie.Popularity", Type: "float32", Template: "{{.Dbmovie.Popularity}}", Description: "Popularity score", Example: "108.6", Category: "Movie Ratings"},
		{Name: "Dbmovie.Budget", Type: "int", Template: "{{.Dbmovie.Budget}}", Description: "Production budget", Example: "160000000", Category: "Movie Financial"},
		{Name: "Dbmovie.Revenue", Type: "int", Template: "{{.Dbmovie.Revenue}}", Description: "Box office revenue", Example: "825532764", Category: "Movie Financial"},
		{Name: "Dbmovie.Poster", Type: "string", Template: "{{.Dbmovie.Poster}}", Description: "Poster image path", Example: "/poster.jpg", Category: "Movie Images"},
		{Name: "Dbmovie.Backdrop", Type: "string", Template: "{{.Dbmovie.Backdrop}}", Description: "Backdrop image path", Example: "/backdrop.jpg", Category: "Movie Images"},
		{Name: "Dbmovie.Slug", Type: "string", Template: "{{.Dbmovie.Slug}}", Description: "URL slug", Example: "inception-2010", Category: "Movie Information"},

		// Dbserie fields (nested struct)
		{Name: "Dbserie.Seriename", Type: "string", Template: "{{.Dbserie.Seriename}}", Description: "Series name", Example: "Breaking Bad", Category: "Series Information"},
		{Name: "Dbserie.Aliases", Type: "string", Template: "{{.Dbserie.Aliases}}", Description: "Alternative series names", Example: "Breaking Bad, BB", Category: "Series Information"},
		{Name: "Dbserie.Overview", Type: "string", Template: "{{.Dbserie.Overview}}", Description: "Series plot summary", Example: "A high school chemistry teacher...", Category: "Series Information"},
		{Name: "Dbserie.Status", Type: "string", Template: "{{.Dbserie.Status}}", Description: "Series status", Example: "Ended", Category: "Series Information"},
		{Name: "Dbserie.Firstaired", Type: "string", Template: "{{.Dbserie.Firstaired}}", Description: "First air date", Example: "2008-01-20", Category: "Series Information"},
		{Name: "Dbserie.Network", Type: "string", Template: "{{.Dbserie.Network}}", Description: "Broadcasting network", Example: "AMC", Category: "Series Information"},
		{Name: "Dbserie.Runtime", Type: "string", Template: "{{.Dbserie.Runtime}}", Description: "Episode runtime", Example: "47", Category: "Series Information"},
		{Name: "Dbserie.Language", Type: "string", Template: "{{.Dbserie.Language}}", Description: "Primary language", Example: "en", Category: "Series Information"},
		{Name: "Dbserie.Genre", Type: "string", Template: "{{.Dbserie.Genre}}", Description: "Series genres", Example: "Crime, Drama, Thriller", Category: "Series Information"},
		{Name: "Dbserie.Rating", Type: "string", Template: "{{.Dbserie.Rating}}", Description: "Content rating", Example: "TV-MA", Category: "Series Information"},
		{Name: "Dbserie.Siterating", Type: "string", Template: "{{.Dbserie.Siterating}}", Description: "Site user rating", Example: "9.5", Category: "Series Ratings"},
		{Name: "Dbserie.SiteratingCount", Type: "string", Template: "{{.Dbserie.SiteratingCount}}", Description: "Rating vote count", Example: "1,654,876", Category: "Series Ratings"},
		{Name: "Dbserie.ImdbID", Type: "string", Template: "{{.Dbserie.ImdbID}}", Description: "IMDB ID", Example: "tt0903747", Category: "Series External IDs"},
		{Name: "Dbserie.ThetvdbID", Type: "int", Template: "{{.Dbserie.ThetvdbID}}", Description: "TVDB ID", Example: "81189", Category: "Series External IDs"},
		{Name: "Dbserie.TraktID", Type: "int", Template: "{{.Dbserie.TraktID}}", Description: "Trakt ID", Example: "1388", Category: "Series External IDs"},
		{Name: "Dbserie.TvrageID", Type: "int", Template: "{{.Dbserie.TvrageID}}", Description: "TVRage ID", Example: "18164", Category: "Series External IDs"},
		{Name: "Dbserie.Facebook", Type: "string", Template: "{{.Dbserie.Facebook}}", Description: "Facebook page URL", Example: "https://www.facebook.com/BreakingBad", Category: "Series External IDs"},
		{Name: "Dbserie.Instagram", Type: "string", Template: "{{.Dbserie.Instagram}}", Description: "Instagram URL", Example: "https://www.instagram.com/breakingbad", Category: "Series External IDs"},
		{Name: "Dbserie.Twitter", Type: "string", Template: "{{.Dbserie.Twitter}}", Description: "Twitter URL", Example: "https://twitter.com/BreakingBad_AMC", Category: "Series External IDs"},
		{Name: "Dbserie.Banner", Type: "string", Template: "{{.Dbserie.Banner}}", Description: "Banner image path", Example: "/banner.jpg", Category: "Series Images"},
		{Name: "Dbserie.Poster", Type: "string", Template: "{{.Dbserie.Poster}}", Description: "Poster image path", Example: "/poster.jpg", Category: "Series Images"},
		{Name: "Dbserie.Fanart", Type: "string", Template: "{{.Dbserie.Fanart}}", Description: "Fanart image path", Example: "/fanart.jpg", Category: "Series Images"},
		{Name: "Dbserie.Identifiedby", Type: "string", Template: "{{.Dbserie.Identifiedby}}", Description: "Episode ID method", Example: "ep", Category: "Series Information"},
		{Name: "Dbserie.Slug", Type: "string", Template: "{{.Dbserie.Slug}}", Description: "URL slug", Example: "breaking-bad", Category: "Series Information"},

		// DbserieEpisode fields (nested struct)
		{Name: "DbserieEpisode.Title", Type: "string", Template: "{{.DbserieEpisode.Title}}", Description: "Episode title", Example: "Pilot", Category: "Episode Information"},
		{Name: "DbserieEpisode.Season", Type: "string", Template: "{{.DbserieEpisode.Season}}", Description: "Season number", Example: "1", Category: "Episode Information"},
		{Name: "DbserieEpisode.Episode", Type: "string", Template: "{{.DbserieEpisode.Episode}}", Description: "Episode number", Example: "1", Category: "Episode Information"},
		{Name: "DbserieEpisode.Identifier", Type: "string", Template: "{{.DbserieEpisode.Identifier}}", Description: "Episode identifier", Example: "S01E01", Category: "Episode Information"},
		{Name: "DbserieEpisode.Overview", Type: "string", Template: "{{.DbserieEpisode.Overview}}", Description: "Episode summary", Example: "Walter White begins cooking...", Category: "Episode Information"},
		{Name: "DbserieEpisode.FirstAired", Type: "time", Template: "{{.DbserieEpisode.FirstAired}}", Description: "Original air date", Example: "2008-01-20", Category: "Episode Information"},
		{Name: "DbserieEpisode.Runtime", Type: "int", Template: "{{.DbserieEpisode.Runtime}}", Description: "Episode runtime in minutes", Example: "58", Category: "Episode Information"},
		{Name: "DbserieEpisode.Poster", Type: "string", Template: "{{.DbserieEpisode.Poster}}", Description: "Episode poster image", Example: "/episode_poster.jpg", Category: "Episode Images"},

		// Source (ParseInfo) fields (nested struct pointer)
		{Name: "Source.Title", Type: "string", Template: "{{.Source.Title}}", Description: "Parsed media title", Example: "Breaking Bad", Category: "Source Information"},
		{Name: "Source.Year", Type: "uint16", Template: "{{.Source.Year}}", Description: "Parsed release year", Example: "2008", Category: "Source Information"},
		{Name: "Source.Season", Type: "int", Template: "{{.Source.Season}}", Description: "Parsed season number", Example: "1", Category: "Source Information"},
		{Name: "Source.Episode", Type: "int", Template: "{{.Source.Episode}}", Description: "Parsed episode number", Example: "1", Category: "Source Information"},
		{Name: "Source.Quality", Type: "string", Template: "{{.Source.Quality}}", Description: "Video quality", Example: "bluray", Category: "Source Quality"},
		{Name: "Source.Resolution", Type: "string", Template: "{{.Source.Resolution}}", Description: "Video resolution", Example: "1080p", Category: "Source Quality"},
		{Name: "Source.Codec", Type: "string", Template: "{{.Source.Codec}}", Description: "Video codec", Example: "x264", Category: "Source Quality"},
		{Name: "Source.Audio", Type: "string", Template: "{{.Source.Audio}}", Description: "Audio codec", Example: "AC3", Category: "Source Quality"},
		{Name: "Source.File", Type: "string", Template: "{{.Source.File}}", Description: "File path", Example: "/path/to/file.mkv", Category: "Source File"},
		{Name: "Source.Size", Type: "int64", Template: "{{.Source.Size}}", Description: "File size in bytes", Example: "1073741824", Category: "Source File"},
		{Name: "Source.Runtime", Type: "int", Template: "{{.Source.Runtime}}", Description: "Runtime in minutes", Example: "58", Category: "Source File"},
		{Name: "Source.Height", Type: "int", Template: "{{.Source.Height}}", Description: "Video height", Example: "1080", Category: "Source Quality"},
		{Name: "Source.Width", Type: "int", Template: "{{.Source.Width}}", Description: "Video width", Example: "1920", Category: "Source Quality"},
		{Name: "Source.Imdb", Type: "string", Template: "{{.Source.Imdb}}", Description: "IMDB ID from source", Example: "tt0903747", Category: "Source External IDs"},
		{Name: "Source.Tvdb", Type: "string", Template: "{{.Source.Tvdb}}", Description: "TVDB ID from source", Example: "tvdb81189", Category: "Source External IDs"},
		{Name: "Source.Identifier", Type: "string", Template: "{{.Source.Identifier}}", Description: "Source identifier", Example: "S01E01", Category: "Source Information"},
		{Name: "Source.Date", Type: "string", Template: "{{.Source.Date}}", Description: "Source release date", Example: "2008-01-20", Category: "Source Information"},
		{Name: "Source.Proper", Type: "bool", Template: "{{.Source.Proper}}", Description: "Is proper release", Example: "false", Category: "Source Quality"},
		{Name: "Source.Extended", Type: "bool", Template: "{{.Source.Extended}}", Description: "Is extended version", Example: "false", Category: "Source Quality"},
		{Name: "Source.Repack", Type: "bool", Template: "{{.Source.Repack}}", Description: "Is repack release", Example: "false", Category: "Source Quality"},
		{Name: "Source.Languages", Type: "[]string", Template: "{{range .Source.Languages}}{{.}}{{end}}", Description: "Available languages", Example: "English, Spanish", Category: "Source Information"},

		// Top-level parsertype fields
		{Name: "TitleSource", Type: "string", Template: "{{.TitleSource}}", Description: "Source title from filename", Example: "Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS", Category: "Parser Information"},
		{Name: "EpisodeTitleSource", Type: "string", Template: "{{.EpisodeTitleSource}}", Description: "Episode title from source", Example: "Pilot", Category: "Parser Information"},
		{Name: "Identifier", Type: "string", Template: "{{.Identifier}}", Description: "Parsed identifier", Example: "S01E01", Category: "Parser Information"},
		{Name: "Episodes", Type: "[]int", Template: "{{range .Episodes}}{{.}}{{end}}", Description: "Episode numbers array", Example: "1, 2, 3", Category: "Parser Information"},
	}
}

func getNotificationFields() []TemplateField {
	return []TemplateField{
		// Dbmovie fields (nested struct) - same as parser
		{Name: "Dbmovie.Title", Type: "string", Template: "{{.Dbmovie.Title}}", Description: "Movie title", Example: "Inception", Category: "Movie Information"},
		{Name: "Dbmovie.Year", Type: "int", Template: "{{.Dbmovie.Year}}", Description: "Movie year", Example: "2000", Category: "Movie Information"},
		{Name: "Dbmovie.OriginalTitle", Type: "string", Template: "{{.Dbmovie.OriginalTitle}}", Description: "Original movie title", Example: "Inception", Category: "Movie Information"},
		{Name: "Dbmovie.Overview", Type: "string", Template: "{{.Dbmovie.Overview}}", Description: "Movie plot summary", Example: "A thief who steals secrets...", Category: "Movie Information"},
		{Name: "Dbmovie.Tagline", Type: "string", Template: "{{.Dbmovie.Tagline}}", Description: "Movie tagline", Example: "Your mind is the scene of the crime", Category: "Movie Information"},
		{Name: "Dbmovie.Genres", Type: "string", Template: "{{.Dbmovie.Genres}}", Description: "Movie genres", Example: "Action, Sci-Fi, Thriller", Category: "Movie Information"},
		{Name: "Dbmovie.Runtime", Type: "int", Template: "{{.Dbmovie.Runtime}}", Description: "Movie runtime in minutes", Example: "148", Category: "Movie Information"},
		{Name: "Dbmovie.ReleaseDate", Type: "time", Template: "{{.Dbmovie.ReleaseDate}}", Description: "Movie release date", Example: "2010-07-16", Category: "Movie Information"},
		{Name: "Dbmovie.Status", Type: "string", Template: "{{.Dbmovie.Status}}", Description: "Release status", Example: "Released", Category: "Movie Information"},
		{Name: "Dbmovie.OriginalLanguage", Type: "string", Template: "{{.Dbmovie.OriginalLanguage}}", Description: "Original language", Example: "en", Category: "Movie Information"},
		{Name: "Dbmovie.SpokenLanguages", Type: "string", Template: "{{.Dbmovie.SpokenLanguages}}", Description: "Spoken languages", Example: "English, Japanese", Category: "Movie Information"},
		{Name: "Dbmovie.ImdbID", Type: "string", Template: "{{.Dbmovie.ImdbID}}", Description: "IMDB ID", Example: "tt1375666", Category: "Movie External IDs"},
		{Name: "Dbmovie.MoviedbID", Type: "int", Template: "{{.Dbmovie.MoviedbID}}", Description: "MovieDB ID", Example: "27205", Category: "Movie External IDs"},
		{Name: "Dbmovie.TraktID", Type: "int", Template: "{{.Dbmovie.TraktID}}", Description: "Trakt ID", Example: "1390", Category: "Movie External IDs"},
		{Name: "Dbmovie.FacebookID", Type: "string", Template: "{{.Dbmovie.FacebookID}}", Description: "Facebook page ID", Example: "inception", Category: "Movie External IDs"},
		{Name: "Dbmovie.InstagramID", Type: "string", Template: "{{.Dbmovie.InstagramID}}", Description: "Instagram ID", Example: "inception", Category: "Movie External IDs"},
		{Name: "Dbmovie.TwitterID", Type: "string", Template: "{{.Dbmovie.TwitterID}}", Description: "Twitter ID", Example: "inception", Category: "Movie External IDs"},
		{Name: "Dbmovie.VoteAverage", Type: "float32", Template: "{{.Dbmovie.VoteAverage}}", Description: "Average user rating", Example: "8.3", Category: "Movie Ratings"},
		{Name: "Dbmovie.VoteCount", Type: "int32", Template: "{{.Dbmovie.VoteCount}}", Description: "Number of votes", Example: "31280", Category: "Movie Ratings"},
		{Name: "Dbmovie.Popularity", Type: "float32", Template: "{{.Dbmovie.Popularity}}", Description: "Popularity score", Example: "108.6", Category: "Movie Ratings"},
		{Name: "Dbmovie.Budget", Type: "int", Template: "{{.Dbmovie.Budget}}", Description: "Production budget", Example: "160000000", Category: "Movie Financial"},
		{Name: "Dbmovie.Revenue", Type: "int", Template: "{{.Dbmovie.Revenue}}", Description: "Box office revenue", Example: "825532764", Category: "Movie Financial"},
		{Name: "Dbmovie.Poster", Type: "string", Template: "{{.Dbmovie.Poster}}", Description: "Poster image path", Example: "/poster.jpg", Category: "Movie Images"},
		{Name: "Dbmovie.Backdrop", Type: "string", Template: "{{.Dbmovie.Backdrop}}", Description: "Backdrop image path", Example: "/backdrop.jpg", Category: "Movie Images"},
		{Name: "Dbmovie.Slug", Type: "string", Template: "{{.Dbmovie.Slug}}", Description: "URL slug", Example: "inception-2010", Category: "Movie Information"},

		// Dbserie fields (nested struct)
		{Name: "Dbserie.Seriename", Type: "string", Template: "{{.Dbserie.Seriename}}", Description: "Series name", Example: "Breaking Bad", Category: "Series Information"},
		{Name: "Dbserie.Aliases", Type: "string", Template: "{{.Dbserie.Aliases}}", Description: "Alternative series names", Example: "Breaking Bad, BB", Category: "Series Information"},
		{Name: "Dbserie.Overview", Type: "string", Template: "{{.Dbserie.Overview}}", Description: "Series plot summary", Example: "A high school chemistry teacher...", Category: "Series Information"},
		{Name: "Dbserie.Status", Type: "string", Template: "{{.Dbserie.Status}}", Description: "Series status", Example: "Ended", Category: "Series Information"},
		{Name: "Dbserie.Firstaired", Type: "string", Template: "{{.Dbserie.Firstaired}}", Description: "First air date", Example: "2008-01-20", Category: "Series Information"},
		{Name: "Dbserie.Network", Type: "string", Template: "{{.Dbserie.Network}}", Description: "Broadcasting network", Example: "AMC", Category: "Series Information"},
		{Name: "Dbserie.Runtime", Type: "string", Template: "{{.Dbserie.Runtime}}", Description: "Episode runtime", Example: "47", Category: "Series Information"},
		{Name: "Dbserie.Language", Type: "string", Template: "{{.Dbserie.Language}}", Description: "Primary language", Example: "en", Category: "Series Information"},
		{Name: "Dbserie.Genre", Type: "string", Template: "{{.Dbserie.Genre}}", Description: "Series genres", Example: "Crime, Drama, Thriller", Category: "Series Information"},
		{Name: "Dbserie.Rating", Type: "string", Template: "{{.Dbserie.Rating}}", Description: "Content rating", Example: "TV-MA", Category: "Series Information"},
		{Name: "Dbserie.Siterating", Type: "string", Template: "{{.Dbserie.Siterating}}", Description: "Site user rating", Example: "9.5", Category: "Series Ratings"},
		{Name: "Dbserie.SiteratingCount", Type: "string", Template: "{{.Dbserie.SiteratingCount}}", Description: "Rating vote count", Example: "1,654,876", Category: "Series Ratings"},
		{Name: "Dbserie.ImdbID", Type: "string", Template: "{{.Dbserie.ImdbID}}", Description: "IMDB ID", Example: "tt0903747", Category: "Series External IDs"},
		{Name: "Dbserie.ThetvdbID", Type: "int", Template: "{{.Dbserie.ThetvdbID}}", Description: "TVDB ID", Example: "81189", Category: "Series External IDs"},
		{Name: "Dbserie.TraktID", Type: "int", Template: "{{.Dbserie.TraktID}}", Description: "Trakt ID", Example: "1388", Category: "Series External IDs"},
		{Name: "Dbserie.TvrageID", Type: "int", Template: "{{.Dbserie.TvrageID}}", Description: "TVRage ID", Example: "18164", Category: "Series External IDs"},
		{Name: "Dbserie.Facebook", Type: "string", Template: "{{.Dbserie.Facebook}}", Description: "Facebook page URL", Example: "https://www.facebook.com/BreakingBad", Category: "Series External IDs"},
		{Name: "Dbserie.Instagram", Type: "string", Template: "{{.Dbserie.Instagram}}", Description: "Instagram URL", Example: "https://www.instagram.com/breakingbad", Category: "Series External IDs"},
		{Name: "Dbserie.Twitter", Type: "string", Template: "{{.Dbserie.Twitter}}", Description: "Twitter URL", Example: "https://twitter.com/BreakingBad_AMC", Category: "Series External IDs"},
		{Name: "Dbserie.Banner", Type: "string", Template: "{{.Dbserie.Banner}}", Description: "Banner image path", Example: "/banner.jpg", Category: "Series Images"},
		{Name: "Dbserie.Poster", Type: "string", Template: "{{.Dbserie.Poster}}", Description: "Poster image path", Example: "/poster.jpg", Category: "Series Images"},
		{Name: "Dbserie.Fanart", Type: "string", Template: "{{.Dbserie.Fanart}}", Description: "Fanart image path", Example: "/fanart.jpg", Category: "Series Images"},
		{Name: "Dbserie.Identifiedby", Type: "string", Template: "{{.Dbserie.Identifiedby}}", Description: "Episode ID method", Example: "ep", Category: "Series Information"},
		{Name: "Dbserie.Slug", Type: "string", Template: "{{.Dbserie.Slug}}", Description: "URL slug", Example: "breaking-bad", Category: "Series Information"},

		// DbserieEpisode fields (nested struct)
		{Name: "DbserieEpisode.Title", Type: "string", Template: "{{.DbserieEpisode.Title}}", Description: "Episode title", Example: "Pilot", Category: "Episode Information"},
		{Name: "DbserieEpisode.Season", Type: "string", Template: "{{.DbserieEpisode.Season}}", Description: "Season number", Example: "1", Category: "Episode Information"},
		{Name: "DbserieEpisode.Episode", Type: "string", Template: "{{.DbserieEpisode.Episode}}", Description: "Episode number", Example: "1", Category: "Episode Information"},
		{Name: "DbserieEpisode.Identifier", Type: "string", Template: "{{.DbserieEpisode.Identifier}}", Description: "Episode identifier", Example: "S01E01", Category: "Episode Information"},
		{Name: "DbserieEpisode.Overview", Type: "string", Template: "{{.DbserieEpisode.Overview}}", Description: "Episode summary", Example: "Walter White begins cooking...", Category: "Episode Information"},
		{Name: "DbserieEpisode.FirstAired", Type: "time", Template: "{{.DbserieEpisode.FirstAired}}", Description: "Original air date", Example: "2008-01-20", Category: "Episode Information"},
		{Name: "DbserieEpisode.Runtime", Type: "int", Template: "{{.DbserieEpisode.Runtime}}", Description: "Episode runtime in minutes", Example: "58", Category: "Episode Information"},
		{Name: "DbserieEpisode.Poster", Type: "string", Template: "{{.DbserieEpisode.Poster}}", Description: "Episode poster image", Example: "/episode_poster.jpg", Category: "Episode Images"},

		// Top-level inputNotifier fields
		{Name: "Title", Type: "string", Template: "{{.Title}}", Description: "Media title", Example: "Inception", Category: "Basic Information"},
		{Name: "Year", Type: "string", Template: "{{.Year}}", Description: "Release year", Example: "2010", Category: "Basic Information"},
		{Name: "Season", Type: "string", Template: "{{.Season}}", Description: "Season number", Example: "1", Category: "Basic Information"},
		{Name: "Episode", Type: "string", Template: "{{.Episode}}", Description: "Episode number", Example: "5", Category: "Basic Information"},
		{Name: "Identifier", Type: "string", Template: "{{.Identifier}}", Description: "Media identifier", Example: "S01E05", Category: "Basic Information"},
		{Name: "Series", Type: "string", Template: "{{.Series}}", Description: "Series name", Example: "Breaking Bad", Category: "Basic Information"},
		{Name: "EpisodeTitle", Type: "string", Template: "{{.EpisodeTitle}}", Description: "Episode title", Example: "Pilot", Category: "Basic Information"},
		{Name: "Configuration", Type: "string", Template: "{{.Configuration}}", Description: "Configuration name", Example: "movies-4k", Category: "Basic Information"},

		// Paths & Locations
		{Name: "SourcePath", Type: "string", Template: "{{.SourcePath}}", Description: "Original file path", Example: "/downloads/movie.mkv", Category: "Paths & Locations"},
		{Name: "Targetpath", Type: "string", Template: "{{.Targetpath}}", Description: "Final organized path", Example: "/media/movies/Inception (2010)/Inception.mkv", Category: "Paths & Locations"},
		{Name: "Rootpath", Type: "string", Template: "{{.Rootpath}}", Description: "Root media path", Example: "/media/movies", Category: "Paths & Locations"},

		// External IDs
		{Name: "Imdb", Type: "string", Template: "{{.Imdb}}", Description: "IMDB ID", Example: "tt1375666", Category: "External IDs"},
		{Name: "Tvdb", Type: "string", Template: "{{.Tvdb}}", Description: "TVDB ID", Example: "290434", Category: "External IDs"},

		// Dates & Times
		{Name: "Time", Type: "string", Template: "{{.Time}}", Description: "Processing timestamp", Example: "2024-01-15 14:30:00", Category: "Dates & Times"},
		{Name: "Date", Type: "string", Template: "{{.Date}}", Description: "Processing date", Example: "2024-01-15", Category: "Dates & Times"},

		// Processing Information
		{Name: "ReplacedPrefix", Type: "string", Template: "{{.ReplacedPrefix}}", Description: "Prefix for replaced files", Example: "Replaced: ", Category: "Processing Information"},

		// Arrays & Lists
		{Name: "Replaced", Type: "[]string", Template: "{{range .Replaced}}{{.}}{{end}}", Description: "List of replaced files", Example: "old_file1.mkv, old_file2.mkv", Category: "Arrays & Lists"},

		// Source (ParseInfo) fields (nested struct pointer) - subset for notifications
		{Name: "Source.Title", Type: "string", Template: "{{.Source.Title}}", Description: "Parsed media title", Example: "Breaking Bad", Category: "Source Information"},
		{Name: "Source.Year", Type: "uint16", Template: "{{.Source.Year}}", Description: "Parsed release year", Example: "2008", Category: "Source Information"},
		{Name: "Source.Season", Type: "int", Template: "{{.Source.Season}}", Description: "Parsed season number", Example: "1", Category: "Source Information"},
		{Name: "Source.Episode", Type: "int", Template: "{{.Source.Episode}}", Description: "Parsed episode number", Example: "1", Category: "Source Information"},
		{Name: "Source.Quality", Type: "string", Template: "{{.Source.Quality}}", Description: "Video quality", Example: "bluray", Category: "Source Quality"},
		{Name: "Source.Resolution", Type: "string", Template: "{{.Source.Resolution}}", Description: "Video resolution", Example: "1080p", Category: "Source Quality"},
		{Name: "Source.Codec", Type: "string", Template: "{{.Source.Codec}}", Description: "Video codec", Example: "x264", Category: "Source Quality"},
		{Name: "Source.Audio", Type: "string", Template: "{{.Source.Audio}}", Description: "Audio codec", Example: "AC3", Category: "Source Quality"},
		{Name: "Source.File", Type: "string", Template: "{{.Source.File}}", Description: "File path", Example: "/path/to/file.mkv", Category: "Source File"},
		{Name: "Source.Size", Type: "int64", Template: "{{.Source.Size}}", Description: "File size in bytes", Example: "1073741824", Category: "Source File"},
		{Name: "Source.Runtime", Type: "int", Template: "{{.Source.Runtime}}", Description: "Runtime in minutes", Example: "58", Category: "Source File"},
		{Name: "Source.Height", Type: "int", Template: "{{.Source.Height}}", Description: "Video height", Example: "1080", Category: "Source Quality"},
		{Name: "Source.Width", Type: "int", Template: "{{.Source.Width}}", Description: "Video width", Example: "1920", Category: "Source Quality"},
		{Name: "Source.Imdb", Type: "string", Template: "{{.Source.Imdb}}", Description: "IMDB ID from source", Example: "tt0903747", Category: "Source External IDs"},
		{Name: "Source.Tvdb", Type: "string", Template: "{{.Source.Tvdb}}", Description: "TVDB ID from source", Example: "tvdb81189", Category: "Source External IDs"},
		{Name: "Source.Identifier", Type: "string", Template: "{{.Source.Identifier}}", Description: "Source identifier", Example: "S01E01", Category: "Source Information"},
		{Name: "Source.Date", Type: "string", Template: "{{.Source.Date}}", Description: "Source release date", Example: "2008-01-20", Category: "Source Information"},
		{Name: "Source.Proper", Type: "bool", Template: "{{.Source.Proper}}", Description: "Is proper release", Example: "false", Category: "Source Quality"},
		{Name: "Source.Extended", Type: "bool", Template: "{{.Source.Extended}}", Description: "Is extended version", Example: "false", Category: "Source Quality"},
		{Name: "Source.Repack", Type: "bool", Template: "{{.Source.Repack}}", Description: "Is repack release", Example: "false", Category: "Source Quality"},
		{Name: "Source.Languages", Type: "[]string", Template: "{{range .Source.Languages}}{{.}}{{end}}", Description: "Available languages", Example: "English, Spanish", Category: "Source Information"},
	}
}

type TemplateVerification struct {
	Valid       bool     `json:"valid"`
	Message     string   `json:"message"`
	Errors      []string `json:"errors,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

func verifyGoTemplate(templateStr, dataType string) TemplateVerification {
	verification := TemplateVerification{
		Valid:       true,
		Message:     "Template syntax appears valid",
		Errors:      []string{},
		Suggestions: []string{},
	}

	// Get available fields for the data type
	fields, _ := getFieldsForDataType(dataType)

	// Build a map of valid field references
	validFields := make(map[string]bool)

	// Add top-level fields based on data type
	switch dataType {
	case "parser":
		{
			validFields["TitleSource"] = true
			validFields["EpisodeTitleSource"] = true
			validFields["Identifier"] = true
			validFields["Episodes"] = true
		}
	case "notification":
		{
			validFields["Title"] = true
			validFields["Year"] = true
			validFields["Season"] = true
			validFields["Episode"] = true
			validFields["Series"] = true
			validFields["EpisodeTitle"] = true
			validFields["Configuration"] = true
			validFields["SourcePath"] = true
			validFields["Targetpath"] = true
			validFields["Rootpath"] = true
			validFields["Imdb"] = true
			validFields["Tvdb"] = true
			validFields["Time"] = true
			validFields["Date"] = true
			validFields["ReplacedPrefix"] = true
			validFields["Replaced"] = true
		}
	}

	// Add nested fields
	for _, field := range fields {
		validFields[field.Name] = true
	}

	// Check for common template syntax issues
	if strings.Contains(templateStr, "{{") && strings.Contains(templateStr, "}}") {
		// Check for unmatched braces
		openCount := strings.Count(templateStr, "{{")
		closeCount := strings.Count(templateStr, "}}")
		if openCount != closeCount {
			verification.Valid = false
			verification.Errors = append(verification.Errors, "Unmatched template braces - ensure every {{ has a matching }}")
		}

		// Check for nested template syntax errors
		if strings.Contains(templateStr, "{{{") || strings.Contains(templateStr, "}}}") {
			verification.Valid = false
			verification.Errors = append(verification.Errors, "Invalid template syntax - avoid triple braces {{{ or }}}")
		}

		// Find all template references
		templateRefs := findTemplateReferences(templateStr)

		// Verify field references
		for _, ref := range templateRefs {
			if ref == "" {
				continue
			}

			// Skip template functions and structures
			if isTemplateFunction(ref) {
				continue
			}

			// Extract field name from reference (handle range, if, with, etc.)
			fieldName := extractFieldName(ref)
			if fieldName != "" && !validFields[fieldName] {
				verification.Valid = false
				verification.Errors = append(verification.Errors, "Unknown field reference: "+fieldName)

				// Suggest similar field names
				suggestion := findSimilarField(fieldName, validFields)
				if suggestion != "" {
					verification.Suggestions = append(verification.Suggestions, "Did you mean '"+suggestion+"' instead of '"+fieldName+"'?")
				}
			}
		}

		// Add helpful suggestions
		if dataType == "parser" && !strings.Contains(templateStr, ".Dbmovie.") && !strings.Contains(templateStr, ".Dbserie.") {
			verification.Suggestions = append(verification.Suggestions, "Consider using nested fields like {{.Dbmovie.Title}} or {{.Dbserie.Seriename}} for richer data")
		}

		if !strings.Contains(templateStr, "{{if") && len(templateRefs) > 3 {
			verification.Suggestions = append(verification.Suggestions, "Consider using conditional statements like {{if .Field}}...{{end}} to handle optional fields")
		}

		if dataType == "notification" && !strings.Contains(templateStr, ".Replaced") && !strings.Contains(templateStr, "range") {
			verification.Suggestions = append(verification.Suggestions, "For notifications, consider showing replaced files using {{range .Replaced}}{{.}}{{end}}")
		}

	} else {
		verification.Valid = false
		verification.Errors = append(verification.Errors, "Template appears to be plain text - use Go template syntax with {{ }} for dynamic content")
	}

	if verification.Valid && len(verification.Errors) == 0 {
		verification.Message = "Template syntax is valid and references valid fields"
	} else if !verification.Valid {
		verification.Message = "Template has syntax or reference errors that need to be fixed"
	}

	return verification
}

func findTemplateReferences(template string) []string {
	var refs []string

	// Find all {{ ... }} patterns
	start := 0
	for {
		startIdx := strings.Index(template[start:], "{{")
		if startIdx == -1 {
			break
		}
		startIdx += start

		endIdx := strings.Index(template[startIdx:], "}}")
		if endIdx == -1 {
			break
		}
		endIdx += startIdx

		ref := strings.TrimSpace(template[startIdx+2 : endIdx])
		if ref != "" {
			refs = append(refs, ref)
		}

		start = endIdx + 2
	}

	return refs
}

func isTemplateFunction(ref string) bool {
	// Common Go template functions and structures
	functions := []string{"range", "if", "else", "end", "with", "printf", "eq", "ne", "lt", "le", "gt", "ge"}

	for _, fn := range functions {
		if strings.HasPrefix(ref, fn+" ") || ref == fn {
			return true
		}
	}

	// Variable assignments like $var := .Field
	if strings.Contains(ref, ":=") || strings.HasPrefix(ref, "$") {
		return true
	}

	return false
}

func extractFieldName(ref string) string {
	// Handle different template patterns
	ref = strings.TrimSpace(ref)

	// Skip template functions
	if isTemplateFunction(ref) {
		return ""
	}

	// Extract field from patterns like ".Field", "if .Field", "range .Array", etc.
	words := strings.Fields(ref)
	for _, word := range words {
		if strings.HasPrefix(word, ".") && len(word) > 1 {
			// Remove leading dot and extract the field path
			field := word[1:]
			// For nested fields like "Dbmovie.Title", we want the full path
			return field
		}
	}

	return ""
}

func findSimilarField(target string, validFields map[string]bool) string {
	target = strings.ToLower(target)
	bestMatch := ""
	bestScore := 0

	for field := range validFields {
		fieldLower := strings.ToLower(field)
		score := calculateSimilarity(target, fieldLower)
		if score > bestScore && score > 50 { // Only suggest if > 50% similar
			bestScore = score
			bestMatch = field
		}
	}

	return bestMatch
}

func calculateSimilarity(s1, s2 string) int {
	// Simple similarity calculation based on common characters
	if s1 == s2 {
		return 100
	}

	// Check if one contains the other
	if strings.Contains(s1, s2) || strings.Contains(s2, s1) {
		return 80
	}

	// Count common characters
	common := 0
	for _, char := range s1 {
		if strings.ContainsRune(s2, char) {
			common++
		}
	}

	maxLen := len(s1)
	if len(s2) > maxLen {
		maxLen = len(s2)
	}

	if maxLen == 0 {
		return 0
	}

	return (common * 100) / maxLen
}

package api

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

type RegexTestResult struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Pattern     string `json:"pattern"`
	Match       bool   `json:"match"`
	MatchString string `json:"match_string,omitempty"`
	Error       string `json:"error,omitempty"`
}

type RegexTestRequest struct {
	TestString string `json:"test_string"`
	TestType   string `json:"test_type"` // "config", "qualities", "global"
}

type RegexTestResponse struct {
	TestString string            `json:"test_string"`
	Results    []RegexTestResult `json:"results"`
}

func renderRegexTesterPage(csrfToken string) gomponents.Node {
	// Get all regex configurations
	regexConfigs := config.GetSettingsRegexAll()

	// Get all quality configurations
	qualityConfigs := config.GetSettingsQualityAll()

	// Get qualities with regex patterns from database
	qualities := database.GetrowsN[database.DbstaticTwoString](false, 200,
		"SELECT name, regex FROM qualities WHERE use_regex = 1 AND regex IS NOT NULL AND regex != '' ORDER BY name")

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-search-plus header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Regex Pattern Tester")),
					html.P(html.Class("header-subtitle"),
						gomponents.Text("Test strings against regex configurations, quality patterns, and global scan patterns")),
				),
			),
		),

		// Test Input Section
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
							gomponents.Text("Test Input")),
					),
					html.Div(
						html.Class("card-body p-4"),
						html.Form(
							html.ID("regexTestForm"),
							html.Method("post"),
							html.Action("/api/admin/regex-tester/test"),

							// CSRF Token
							html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),

							// Test String Input
							html.Div(
								html.Class("form-group mb-4"),
								html.Label(html.Class("form-label fw-semibold"), html.For("test_string"),
									gomponents.Text("Test String")),
								html.Textarea(
									html.Class("form-control"),
									html.ID("test_string"),
									html.Name("test_string"),
									html.Rows("3"),
									html.Placeholder("Enter the string you want to test against regex patterns..."),
									html.Style("font-family: 'Courier New', monospace;"),
								),
								html.Small(html.Class("form-text text-muted"),
									gomponents.Text("Enter a release name, filename, or any text to test against patterns")),
							),

							// Test Type Selection
							html.Div(
								html.Class("form-group mb-4"),
								html.Label(html.Class("form-label fw-semibold"),
									gomponents.Text("Test Against")),

								// Test type checkboxes with configuration selectors
								html.Div(html.Class("row"),
									// Regex Configurations
									html.Div(html.Class("col-md-4"),
										html.Div(html.Class("form-check form-switch mb-2"),
											html.Input(
												html.Class("form-check-input"),
												html.Style("margin-left: 35px;"),
												html.Type("checkbox"),
												html.ID("test_config"),
												html.Name("test_config"),
												html.Value("true"),
												html.Checked(),
											),
											html.Label(html.Class("form-check-label"), html.For("test_config"),
												gomponents.Text("Regex Configurations")),
										),
										html.Select(
											html.Class("form-select form-select-sm mt-2"),
											html.ID("selected_regex_config"),
											html.Name("selected_regex_config"),
											func() gomponents.Node {
												var options []gomponents.Node
												options = append(options, html.Option(html.Value(""), gomponents.Text("All Configurations")))
												for _, regexConfig := range regexConfigs {
													options = append(options, html.Option(html.Value(regexConfig.Name), gomponents.Text(regexConfig.Name)))
												}
												return gomponents.Group(options)
											}(),
										),
									),

									// Quality Patterns
									html.Div(html.Class("col-md-4"),
										html.Div(html.Class("form-check form-switch mb-2"),
											html.Input(
												html.Class("form-check-input"),
												html.Style("margin-left: 35px;"),
												html.Type("checkbox"),
												html.ID("test_qualities"),
												html.Name("test_qualities"),
												html.Value("true"),
												html.Checked(),
											),
											html.Label(html.Class("form-check-label"), html.For("test_qualities"),
												gomponents.Text("Quality Patterns")),
										),
										html.Select(
											html.Class("form-select form-select-sm mt-2"),
											html.ID("selected_quality_config"),
											html.Name("selected_quality_config"),
											func() gomponents.Node {
												var options []gomponents.Node
												options = append(options, html.Option(html.Value(""), gomponents.Text("All Quality Profiles")))
												for _, qualityConfig := range qualityConfigs {
													options = append(options, html.Option(html.Value(qualityConfig.Name), gomponents.Text(qualityConfig.Name)))
												}
												return gomponents.Group(options)
											}(),
										),
									),

									// Global Scan Patterns
									html.Div(html.Class("col-md-4"),
										html.Div(html.Class("form-check form-switch mb-2"),
											html.Input(
												html.Class("form-check-input"),
												html.Style("margin-left: 35px;"),
												html.Type("checkbox"),
												html.ID("test_global"),
												html.Name("test_global"),
												html.Value("true"),
												html.Checked(),
											),
											html.Label(html.Class("form-check-label"), html.For("test_global"),
												gomponents.Text("Global Scan Patterns")),
										),
									),
								),
							),

							// Test Button
							html.Div(
								html.Class("form-group text-center"),
								html.Button(
									html.Class("btn btn-primary btn-lg px-5"),
									html.Type("submit"),
									html.Style("border-radius: 25px; font-weight: 600; text-transform: uppercase; letter-spacing: 1px;"),
									html.I(html.Class("fas fa-play me-2")),
									gomponents.Text("Run Tests"),
								),
							),
						),
					),
				),
			),
		),

		// Results Section
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.ID("testResults"),
					html.Class("d-none"),
					html.Div(
						html.Class("card border-0 shadow-sm"),
						html.Style("border-radius: 15px; overflow: hidden;"),
						html.Div(
							html.Class("card-header border-0"),
							html.Style("background: linear-gradient(135deg, #28a745 0%, #20c997 100%); color: white; padding: 1.5rem;"),
							html.H5(html.Class("card-title mb-0"), html.Style("font-weight: 600;"),
								gomponents.Text("Test Results")),
						),
						html.Div(
							html.Class("card-body p-0"),
							html.Div(html.ID("resultsContent")),
						),
					),
				),
			),
		),

		// Information Section
		html.Div(
			html.Class("row mt-4"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden; background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%);"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style("background: linear-gradient(135deg, #6c757d 0%, #495057 100%); color: white; padding: 1.5rem;"),
						html.H5(html.Class("card-title mb-0"), html.Style("font-weight: 600;"),
							gomponents.Text("Available Patterns")),
					),
					html.Div(
						html.Class("card-body p-4"),
						html.Div(html.Class("row"),
							// Regex Configurations
							html.Div(html.Class("col-md-4 mb-4"),
								html.H6(html.Class("fw-bold mb-3"),
									html.I(html.Class("fas fa-cogs me-2 text-primary")),
									gomponents.Text("Regex Configurations")),
								func() gomponents.Node {
									if len(regexConfigs) == 0 {
										return html.P(html.Class("text-muted"), gomponents.Text("No regex configurations found"))
									}
									var items []gomponents.Node
									for _, regexConfig := range regexConfigs {
										items = append(items, html.Li(html.Class("mb-1"),
											html.Code(html.Class("text-primary"), gomponents.Text(regexConfig.Name))))
									}
									return html.Ul(html.Class("list-unstyled"), gomponents.Group(items))
								}(),
							),

							// Quality Patterns
							html.Div(html.Class("col-md-4 mb-4"),
								html.H6(html.Class("fw-bold mb-3"),
									html.I(html.Class("fas fa-star me-2 text-warning")),
									gomponents.Text("Quality Patterns")),
								func() gomponents.Node {
									if len(qualities) == 0 {
										return html.P(html.Class("text-muted"), gomponents.Text("No quality patterns found"))
									}
									var items []gomponents.Node
									for _, quality := range qualities {
										items = append(items, html.Li(html.Class("mb-1"),
											html.Code(html.Class("text-warning"), gomponents.Text(quality.Str1))))
									}
									return html.Ul(html.Class("list-unstyled"), gomponents.Group(items))
								}(),
							),

							// Global Scan Patterns
							html.Div(html.Class("col-md-4 mb-4"),
								html.H6(html.Class("fw-bold mb-3"),
									html.I(html.Class("fas fa-globe me-2 text-success")),
									gomponents.Text("Global Scan Patterns")),
								html.Ul(html.Class("list-unstyled"),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("season"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("episode"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("identifier"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("date"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("year"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("audio"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("imdb"))),
									html.Li(html.Class("mb-1"), html.Code(html.Class("text-success"), gomponents.Text("tvdb"))),
								),
							),
						),
					),
				),
			),
		),

		// JavaScript for form handling
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				// Bind to both the form and the submit button
				$('#regexTestForm').on('submit', function(e) {
					e.preventDefault();
					e.stopPropagation();
					handleFormSubmission();
					return false;
				});
				
				// Also bind to button click as backup
				$('#regexTestForm button[type="submit"]').on('click', function(e) {
					e.preventDefault();
					e.stopPropagation();
					handleFormSubmission();
					return false;
				});
				
				function handleFormSubmission() {
					var form = $('#regexTestForm');
					var formData = form.serialize();
					var submitBtn = form.find('button[type="submit"]');
					var originalText = submitBtn.html();
				
					// Show loading state
					submitBtn.prop('disabled', true).html('<i class="fas fa-spinner fa-spin me-2"></i>Testing...');
					$('#testResults').addClass('d-none');
					
					$.ajax({
						url: form.attr('action'),
						method: 'POST',
						data: formData,
						dataType: 'json',
						success: function(response) {
							displayResults(response);
							$('#testResults').removeClass('d-none');
							$('html, body').animate({
								scrollTop: $('#testResults').offset().top - 100
							}, 500);
						},
						error: function(xhr, status, error) {
							console.error('AJAX Error:', {xhr: xhr, status: status, error: error});
							console.error('Response Text:', xhr.responseText);
							if (typeof showToaster === 'function') {
								showToaster('error', 'Error testing regex patterns: ' + error);
							} else {
								alert('Error testing regex patterns: ' + error + '\nResponse: ' + xhr.responseText);
							}
						},
						complete: function() {
							submitBtn.prop('disabled', false).html(originalText);
						}
					});
				} // Close handleFormSubmission function
				
				function displayResults(response) {
					var html = '<div class="table-responsive">';
					html += '<table class="table table-hover mb-0">';
					html += '<thead class="table-light">';
					html += '<tr>';
					html += '<th style="border-top: none; color: #495057; font-weight: 600; padding: 1rem;">Type</th>';
					html += '<th style="border-top: none; color: #495057; font-weight: 600; padding: 1rem;">Name</th>';
					html += '<th style="border-top: none; color: #495057; font-weight: 600; padding: 1rem;">Pattern</th>';
					html += '<th style="border-top: none; color: #495057; font-weight: 600; padding: 1rem;">Match</th>';
					html += '<th style="border-top: none; color: #495057; font-weight: 600; padding: 1rem;">Captured</th>';
					html += '</tr>';
					html += '</thead>';
					html += '<tbody>';
					
					if (response.results && response.results.length > 0) {
						response.results.forEach(function(result) {
							html += '<tr style="border-left: 4px solid ' + (result.match ? '#28a745' : '#dc3545') + '; transition: all 0.2s;">';
							html += '<td style="padding: 1rem; vertical-align: middle;">';
							html += '<span class="badge ' + getTypeBadgeClass(result.type) + ' px-3 py-2" style="border-radius: 15px;">';
							html += '<i class="' + getTypeIcon(result.type) + ' me-1"></i>' + result.type;
							html += '</span></td>';
							html += '<td style="padding: 1rem; vertical-align: middle; font-family: monospace;">' + escapeHtml(result.name) + '</td>';
							html += '<td style="padding: 1rem; vertical-align: middle; font-family: monospace; font-size: 0.85rem; max-width: 300px; word-break: break-all;">' + escapeHtml(result.pattern) + '</td>';
							html += '<td style="padding: 1rem; vertical-align: middle; text-align: center;">';
							if (result.match) {
								html += '<span class="badge bg-success px-3 py-2" style="border-radius: 15px;"><i class="fas fa-check me-1"></i>Match</span>';
							} else {
								html += '<span class="badge bg-danger px-3 py-2" style="border-radius: 15px;"><i class="fas fa-times me-1"></i>No Match</span>';
							}
							html += '</td>';
							html += '<td style="padding: 1rem; vertical-align: middle; font-family: monospace; color: #28a745;">' + (result.match_string || '') + '</td>';
							html += '</tr>';
						});
					} else {
						html += '<tr><td colspan="5" class="text-center text-muted p-5">No results found</td></tr>';
					}
					
					html += '</tbody>';
					html += '</table>';
					html += '</div>';
					
					$('#resultsContent').html(html);
				}
				
				function getTypeBadgeClass(type) {
					switch(type) {
						case 'Config': return 'bg-primary';
						case 'Quality': return 'bg-warning';
						case 'Global': return 'bg-success';
						default: return 'bg-secondary';
					}
				}
				
				function getTypeIcon(type) {
					switch(type) {
						case 'Config': return 'fas fa-cogs';
						case 'Quality': return 'fas fa-star';
						case 'Global': return 'fas fa-globe';
						default: return 'fas fa-question';
					}
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
			});
		`)),
	)
}

func HandleRegexTesting(ctx *gin.Context) {
	var response RegexTestResponse

	testString := strings.TrimSpace(ctx.PostForm("test_string"))
	if testString == "" {
		sendJSONError(ctx, http.StatusBadRequest, "Test string is required")
		return
	}

	response.TestString = testString
	selectedRegexConfig := ctx.PostForm("selected_regex_config")
	selectedQualityConfig := ctx.PostForm("selected_quality_config")

	// Test Config Patterns
	if ctx.PostForm("test_config") == "on" || ctx.PostForm("test_config") == "true" {
		config.RangeSettingsRegex(func(name string, regexConfig *config.RegexConfig) {
			// Skip if a specific config is selected and this isn't it
			if selectedRegexConfig != "" && name != selectedRegexConfig {
				return
			}
			// Test Required patterns
			for _, pattern := range regexConfig.Required {
				if pattern == "" {
					continue
				}
				result := RegexTestResult{
					Type:    "Config",
					Name:    name + " (Required)",
					Pattern: pattern,
				}

				if compiled, err := regexp.Compile("(?i)" + pattern); err != nil {
					result.Error = err.Error()
				} else {
					matches := compiled.FindStringSubmatch(testString)
					result.Match = len(matches) > 0
					if result.Match && len(matches) > 1 {
						result.MatchString = strings.Join(matches[1:], ", ")
					}
				}
				response.Results = append(response.Results, result)
			}

			// Test Rejected patterns
			for _, pattern := range regexConfig.Rejected {
				if pattern == "" {
					continue
				}
				result := RegexTestResult{
					Type:    "Config",
					Name:    name + " (Rejected)",
					Pattern: pattern,
				}

				if compiled, err := regexp.Compile("(?i)" + pattern); err != nil {
					result.Error = err.Error()
				} else {
					matches := compiled.FindStringSubmatch(testString)
					result.Match = len(matches) > 0
					if result.Match && len(matches) > 1 {
						result.MatchString = strings.Join(matches[1:], ", ")
					}
				}
				response.Results = append(response.Results, result)
			}
		})
	}

	// Test Quality Patterns
	if ctx.PostForm("test_qualities") == "on" || ctx.PostForm("test_qualities") == "true" {
		if selectedQualityConfig != "" {
			// Test quality patterns from database filtered by name containing the quality config name
			qualities := database.GetrowsN[database.DbstaticTwoString](false, 200,
				"SELECT name, regex FROM qualities WHERE use_regex = 1 AND regex IS NOT NULL AND regex != '' AND LOWER(name) LIKE ? ORDER BY name", "%"+strings.ToLower(selectedQualityConfig)+"%")

			for _, quality := range qualities {
				result := RegexTestResult{
					Type:    "Quality",
					Name:    quality.Str1 + " (from " + selectedQualityConfig + ")",
					Pattern: quality.Str2,
				}

				if compiled, err := regexp.Compile("(?i)" + quality.Str2); err != nil {
					result.Error = err.Error()
				} else {
					matches := compiled.FindStringSubmatch(testString)
					result.Match = len(matches) > 0
					if result.Match && len(matches) > 1 {
						result.MatchString = strings.Join(matches[1:], ", ")
					}
				}
				response.Results = append(response.Results, result)
			}
		} else {
			// Test all quality patterns from database
			qualities := database.GetrowsN[database.DbstaticTwoString](false, 200,
				"SELECT name, regex FROM qualities WHERE use_regex = 1 AND regex IS NOT NULL AND regex != '' ORDER BY name")

			for _, quality := range qualities {
				result := RegexTestResult{
					Type:    "Quality",
					Name:    quality.Str1,
					Pattern: quality.Str2,
				}

				if compiled, err := regexp.Compile("(?i)" + quality.Str2); err != nil {
					result.Error = err.Error()
				} else {
					matches := compiled.FindStringSubmatch(testString)
					result.Match = len(matches) > 0
					if result.Match && len(matches) > 1 {
						result.MatchString = strings.Join(matches[1:], ", ")
					}
				}
				response.Results = append(response.Results, result)
			}
		}
	}

	// Test Global Scan Patterns
	if ctx.PostForm("test_global") == "on" || ctx.PostForm("test_global") == "true" {
		globalPatterns := []struct {
			name    string
			pattern string
		}{
			{"season", `(?i)(s?(\d{1,4}))(?: )?[ex]`},
			{"episode", `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`},
			{"identifier", `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex-]\d{1,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`},
			{"date", `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`},
			{"year", `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`},
			{"audio", `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`},
			{"imdb", `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`},
			{"tvdb", `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`},
		}

		for _, globalPattern := range globalPatterns {
			result := RegexTestResult{
				Type:    "Global",
				Name:    globalPattern.name,
				Pattern: globalPattern.pattern,
			}

			if compiled, err := regexp.Compile(globalPattern.pattern); err != nil {
				result.Error = err.Error()
			} else {
				matches := compiled.FindStringSubmatch(testString)
				result.Match = len(matches) > 0
				if result.Match && len(matches) > 1 {
					result.MatchString = strings.Join(matches[1:], ", ")
				}
			}
			response.Results = append(response.Results, result)
		}
	}

	ctx.JSON(http.StatusOK, response)
}

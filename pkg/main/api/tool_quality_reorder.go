package api

import (
	"fmt"
	stdhtml "html"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-json"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// HandleQualityReorderRules handles HTMX requests for loading reorder rules.
func HandleQualityReorderRules(c *gin.Context) {
	selectedProfile := c.Query("profile")
	if selectedProfile == "" {
		selectedProfile = c.PostForm("profile")
	}

	if selectedProfile == "" {
		selectedProfile = c.Query("selected_quality")
	}

	if selectedProfile == "" {
		selectedProfile = c.PostForm("selected_quality")
	}

	if selectedProfile == "" {
		c.String(
			http.StatusOK,
			`<p class="text-muted mb-0 text-center">Select a quality profile to load rules.</p>`,
		)

		return
	}

	// Get the selected quality profile configuration
	qualityConfig := config.GetSettingsQuality(selectedProfile)
	if qualityConfig == nil {
		c.String(
			http.StatusOK,
			`<p class="text-danger mb-0 text-center">Quality profile "<strong>`+stdhtml.EscapeString(selectedProfile)+`</strong>" not found.</p>`,
		)

		return
	}

	// Build HTML for displaying existing reorder rules from the profile
	var html strings.Builder

	if len(qualityConfig.QualityReorder) == 0 {
		html.WriteString(
			`<p class="text-muted mb-0 text-center">No reorder rules defined in profile: <strong>`,
		)
		html.WriteString(stdhtml.EscapeString(selectedProfile))
		html.WriteString(`</strong>. Click 'Add Rule' to create one.</p>`)
	} else {
		html.WriteString(`<p class="text-info mb-3 text-center">Showing `)
		html.WriteString(strconv.Itoa(len(qualityConfig.QualityReorder)))
		html.WriteString(` reorder rule(s) from profile: <strong>`)
		html.WriteString(stdhtml.EscapeString(selectedProfile))
		html.WriteString(`</strong></p>`)

		// Display each reorder rule
		for i, rule := range qualityConfig.QualityReorder {
			ruleID := fmt.Sprintf("rule_%d_%d", time.Now().Unix(), i)

			html.WriteString(`<div class="row mb-2 border rounded p-2 bg-white" data-rule-id="`)
			html.WriteString(ruleID)
			html.WriteString(`">`)

			// Type column
			html.WriteString(`<div class="col-md-2">`)
			html.WriteString(
				`<select class="form-select form-select-sm" name="rule_type" disabled>`,
			)
			html.WriteString(`<option value="`)
			html.WriteString(stdhtml.EscapeString(rule.ReorderType))
			html.WriteString(`" selected>`)
			html.WriteString(stdhtml.EscapeString(cases.Title(language.English).String(rule.ReorderType)))
			html.WriteString(`</option>`)
			html.WriteString(`</select>`)
			html.WriteString(`</div>`)

			// Pattern column
			html.WriteString(`<div class="col-md-3">`)
			html.WriteString(
				`<input type="text" class="form-control form-control-sm" name="rule_pattern" value="`,
			)
			html.WriteString(stdhtml.EscapeString(rule.Name))
			html.WriteString(`" placeholder="Pattern/Name">`)
			html.WriteString(`</div>`)

			// Priority column
			html.WriteString(`<div class="col-md-2">`)
			html.WriteString(
				`<input type="number" class="form-control form-control-sm" name="rule_priority" value="`,
			)
			html.WriteString(strconv.Itoa(rule.Newpriority))
			html.WriteString(`" min="-1000000" max="1000000" step="1">`)
			html.WriteString(`</div>`)

			// Enabled column
			html.WriteString(`<div class="col-md-3">`)
			html.WriteString(`<div class="form-check">`)
			html.WriteString(
				`<input type="checkbox" class="form-check-input" name="rule_enabled" checked>`,
			)
			html.WriteString(`<label class="form-check-label">Profile Rule (Editable)</label>`)
			html.WriteString(`</div>`)
			html.WriteString(`</div>`)

			// Actions column
			html.WriteString(`<div class="col-md-2">`)
			html.WriteString(`<small class="text-muted">From Config</small>`)
			html.WriteString(`</div>`)

			html.WriteString(`</div>`)
		}

		html.WriteString(`<hr class="my-3">`)
		html.WriteString(`<p class="text-muted small text-center mb-0">`)
		html.WriteString(`The rules above are from the quality profile configuration. `)
		html.WriteString(`Use 'Add Rule' to create temporary overrides for testing.`)
		html.WriteString(`</p>`)
	}

	c.String(http.StatusOK, html.String())
}

// HandleQualityReorderAddRule handles HTMX requests for adding a new reorder rule.
func HandleQualityReorderAddRule(c *gin.Context) {
	// Generate a new rule template
	ruleHTML := `
	<div class="row mb-2 border rounded p-2 bg-white" data-rule-id="` + fmt.Sprintf("%d", time.Now().Unix()) + `">
		<div class="col-md-2">
			<select class="form-select form-select-sm" name="rule_type">
				<option value="">Select Type</option>
				<option value="resolution">Resolution</option>
				<option value="quality">Quality</option>
				<option value="codec">Codec</option>
				<option value="audio">Audio</option>
				<option value="audio_format">Audio Format</option>
				<option value="position">Position</option>
				<option value="combined_res_qual">Resolution,Quality</option>
			</select>
		</div>
		<div class="col-md-3">
			<input type="text" class="form-control form-control-sm" name="rule_pattern" placeholder="Pattern (regex or exact match)" />
		</div>
		<div class="col-md-2">
			<input type="number" class="form-control form-control-sm" name="rule_priority" placeholder="±Priority" step="1" />
		</div>
		<div class="col-md-3">
			<div class="form-check">
				<input class="form-check-input" type="checkbox" name="rule_enabled" checked />
				<label class="form-check-label">Enabled</label>
			</div>
		</div>
		<div class="col-md-2">
			<button type="button" class="btn btn-sm btn-outline-danger" onclick="this.closest('[data-rule-id]').remove();">
				<i class="fas fa-trash"></i>
			</button>
		</div>
	</div>`

	c.String(http.StatusOK, ruleHTML)
}

// HandleQualityReorderResetRules handles HTMX requests for resetting all reorder rules.
func HandleQualityReorderResetRules(c *gin.Context) {
	c.String(
		http.StatusOK,
		`<p class="text-muted mb-0 text-center">No temporary reorder rules. Click 'Add Rule' to create one.</p>`,
	)
}

// contains checks if a string slice contains a specific string (case insensitive).
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, item) {
			return true
		}
	}

	return false
}

// previewCombinedReorder mirrors parser.handleCombinedReorder: the first
// combined_res_qual rule whose "resolution,quality" name matches (split at the
// first comma, case-insensitive, no trimming) sets the resolution priority and
// zeroes the quality priority. matched reports whether a rule applied.
func previewCombinedReorder(
	qual *config.QualityConfig,
	resolutionName, qualityName string,
	prioReso, prioQual int,
) (reso, quality int, matched bool) {
	for idx := range qual.QualityReorder {
		reorder := &qual.QualityReorder[idx]
		if !strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
			continue
		}

		reorderRes, reorderQual, ok := strings.Cut(reorder.Name, ",")
		if !ok {
			continue
		}

		if strings.EqualFold(reorderRes, resolutionName) &&
			strings.EqualFold(reorderQual, qualityName) {
			return reorder.Newpriority, 0, true
		}
	}

	return prioReso, prioQual, false
}

func HandleQualityReorder(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	selectedQuality := c.PostForm("selected_quality")
	if selectedQuality == "" {
		c.String(http.StatusOK, renderAlert("Please select a quality profile", "warning"))
		return
	}

	// Get filter parameters
	filterResolution := c.PostForm("filter_resolution")
	filterQuality := c.PostForm("filter_quality")
	wantedOnly := c.PostForm("wanted_only") == "on" || c.PostForm("wanted_only") == "true" ||
		c.PostForm("_wanted_only") == "on" ||
		c.PostForm("_wanted_only") == "true"

	// Get reorder rule modifications (JSON)
	reorderRulesJSON := c.PostForm("reorder_rules")

	// The page sends the full (enabled) rule set once its rule editor is
	// loaded; "" means the editor was never used, "[]" means all rules are
	// disabled/removed and must not fall back to the configured ones.
	rulesProvided := reorderRulesJSON != ""

	var tempReorderRules []config.QualityReorderConfig
	if rulesProvided && reorderRulesJSON != "[]" {
		// Parse JSON array of reorder rules
		type TempReorderRule struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			NewPriority int    `json:"new_priority"`
		}

		var tempRules []TempReorderRule
		if err := json.Unmarshal([]byte(reorderRulesJSON), &tempRules); err != nil {
			c.String(http.StatusOK, renderAlert("Invalid reorder rules: "+err.Error(), "danger"))
			return
		}

		for _, tempRule := range tempRules {
			tempReorderRules = append(tempReorderRules, config.QualityReorderConfig{
				Name:        tempRule.Name,
				ReorderType: tempRule.Type,
				Newpriority: tempRule.NewPriority,
			})
		}
	}

	originalConfig := config.GetSettingsQuality(selectedQuality)
	if originalConfig == nil {
		c.String(
			http.StatusOK,
			renderAlert("Quality profile not found: "+selectedQuality, "danger"),
		)

		return
	}

	// Create a copy of the config for temporary modifications
	qualityConfig := &config.QualityConfig{}

	*qualityConfig = *originalConfig

	// Apply temporary reorder rule modifications
	if rulesProvided {
		qualityConfig.QualityReorder = append([]config.QualityReorderConfig{}, tempReorderRules...)
		qualityConfig.QualityReorderLen = len(tempReorderRules)
	}

	// Get quality data from database and filter to wanted only
	allResolutions := database.DBConnect.GetresolutionsIn
	allQualities := database.DBConnect.GetqualitiesIn

	// Filter resolutions to only wanted ones (plus empty for no resolution)
	var getresolutions []database.Qualities

	regex0 := database.Qualities{Name: "", ID: 0, Priority: 0}

	getresolutions = append(getresolutions, regex0)

	for _, res := range allResolutions {
		// Apply wanted only filter if enabled
		includeResolution := true
		if wantedOnly {
			includeResolution = (len(qualityConfig.WantedResolution) == 0 || contains(qualityConfig.WantedResolution, res.Name))
		}

		if includeResolution {
			// Apply resolution filter if specified
			if filterResolution == "" || filterResolution == "all" || res.Name == filterResolution {
				getresolutions = append(getresolutions, res)
			}
		}
	}

	// Filter qualities to only wanted ones (plus empty for no quality)
	var getqualities []database.Qualities

	getqualities = append(getqualities, regex0)

	for _, qual := range allQualities {
		// Apply wanted only filter if enabled
		includeQuality := true
		if wantedOnly {
			includeQuality = (len(qualityConfig.WantedQuality) == 0 || contains(qualityConfig.WantedQuality, qual.Name))
		}

		if includeQuality {
			// Apply quality filter if specified
			if filterQuality == "" || filterQuality == "all" || qual.Name == filterQuality {
				getqualities = append(getqualities, qual)
			}
		}
	}

	// Only use empty codec and audio for simplicity (focus on resolution + quality)
	getaudios := make([]database.Qualities, 0, 1)

	getaudios = append(getaudios, regex0)

	getcodecs := make([]database.Qualities, 0, 1)

	getcodecs = append(getcodecs, regex0)

	// Include audio formats for music/audiobook profiles
	getaudioformats := make([]database.Qualities, 0, 1+len(database.DBConnect.GetaudioformatsIn))

	getaudioformats = append(getaudioformats, regex0)
	getaudioformats = append(getaudioformats, database.DBConnect.GetaudioformatsIn...)

	if len(getresolutions) <= 1 && len(getqualities) <= 1 && len(getaudioformats) <= 1 {
		c.String(
			http.StatusOK,
			renderAlert("No wanted quality combinations found for this profile", "info"),
		)

		return
	}

	type QualityCombination struct {
		Resolution            string
		Quality               string
		Codec                 string
		Audio                 string
		AudioFormat           string
		TotalPriority         int
		OriginalTotalPriority int
		ResoPriority          int
		QualPriority          int
		CodecPriority         int
		AudioPriority         int
		AudioFormatPriority   int
		OriginalResoPriority  int
		OriginalQualPriority  int
		ReorderApplied        bool
	}

	var combinations []QualityCombination

	// Generate combinations focusing on resolution + quality
	for idxreso := range getresolutions {
		for idxqual := range getqualities {
			for idxcodec := range getcodecs {
				for idxaudio := range getaudios {
					for idxaf := range getaudioformats {
						// Calculate ORIGINAL priorities (using original config without temp rules)
						originalResoPriority := getresolutions[idxreso].Gettypeidprioritysingle(
							"resolution",
							originalConfig,
						)
						originalQualPriority := getqualities[idxqual].Gettypeidprioritysingle(
							"quality",
							originalConfig,
						)
						originalCodecPriority := getcodecs[idxcodec].Gettypeidprioritysingle(
							"codec",
							originalConfig,
						)
						originalAudioPriority := getaudios[idxaudio].Gettypeidprioritysingle(
							"audio",
							originalConfig,
						)
						originalAFPriority := getaudioformats[idxaf].Gettypeidprioritysingle(
							"audio_format",
							originalConfig,
						)

						// Apply original config reorder rules to original priorities
						originalFinalResoPrio, originalFinalQualPrio, _ := previewCombinedReorder(
							originalConfig,
							getresolutions[idxreso].Name,
							getqualities[idxqual].Name,
							originalResoPriority,
							originalQualPriority,
						)

						// Calculate MODIFIED priorities (using modified config with temp rules)
						modifiedResoPriority := getresolutions[idxreso].Gettypeidprioritysingle(
							"resolution",
							qualityConfig,
						)
						modifiedQualPriority := getqualities[idxqual].Gettypeidprioritysingle(
							"quality",
							qualityConfig,
						)
						modifiedCodecPriority := getcodecs[idxcodec].Gettypeidprioritysingle(
							"codec",
							qualityConfig,
						)
						modifiedAudioPriority := getaudios[idxaudio].Gettypeidprioritysingle(
							"audio",
							qualityConfig,
						)
						modifiedAFPriority := getaudioformats[idxaf].Gettypeidprioritysingle(
							"audio_format",
							qualityConfig,
						)

						// Apply modified config reorder rules to modified priorities
						modifiedFinalResoPrio, modifiedFinalQualPrio, reorderApplied := previewCombinedReorder(
							qualityConfig,
							getresolutions[idxreso].Name,
							getqualities[idxqual].Name,
							modifiedResoPriority,
							modifiedQualPriority,
						)

						// Check if priorities changed due to temporary rules
						if modifiedFinalResoPrio != originalFinalResoPrio ||
							modifiedFinalQualPrio != originalFinalQualPrio ||
							modifiedAFPriority != originalAFPriority {
							reorderApplied = true
						}

						// Calculate total priorities
						originalTotalPriority := originalFinalResoPrio + originalFinalQualPrio + originalCodecPriority + originalAudioPriority + originalAFPriority
						modifiedTotalPriority := modifiedFinalResoPrio + modifiedFinalQualPrio + modifiedCodecPriority + modifiedAudioPriority + modifiedAFPriority

						combinations = append(combinations, QualityCombination{
							Resolution:            getresolutions[idxreso].Name,
							Quality:               getqualities[idxqual].Name,
							Codec:                 getcodecs[idxcodec].Name,
							Audio:                 getaudios[idxaudio].Name,
							AudioFormat:           getaudioformats[idxaf].Name,
							TotalPriority:         modifiedTotalPriority,
							OriginalTotalPriority: originalTotalPriority,
							ResoPriority:          modifiedFinalResoPrio,
							QualPriority:          modifiedFinalQualPrio,
							CodecPriority:         modifiedCodecPriority,
							AudioPriority:         modifiedAudioPriority,
							AudioFormatPriority:   modifiedAFPriority,
							OriginalResoPriority:  originalFinalResoPrio,
							OriginalQualPriority:  originalFinalQualPrio,
							ReorderApplied:        reorderApplied,
						})
					}
				}
			}
		}
	}

	// Sort by total priority (highest first)
	sort.Slice(combinations, func(i, j int) bool {
		return combinations[i].TotalPriority > combinations[j].TotalPriority
	})

	var rows []gomponents.Node
	for i, combo := range combinations {
		// Build combination display (only resolution + quality)
		combinationDisplay := ""
		if combo.Resolution != "" {
			combinationDisplay += combo.Resolution
		}

		if combo.Quality != "" {
			if combinationDisplay != "" {
				combinationDisplay += " + "
			}

			combinationDisplay += combo.Quality
		}

		if combo.AudioFormat != "" {
			if combinationDisplay != "" {
				combinationDisplay += " + "
			}

			combinationDisplay += combo.AudioFormat
		}

		if combinationDisplay == "" {
			combinationDisplay = "No specific resolution/quality"
		}

		// Build priority breakdown
		priorityBreakdown := fmt.Sprintf("%d", combo.ResoPriority)
		if combo.QualPriority >= 0 {
			priorityBreakdown += fmt.Sprintf("+%d", combo.QualPriority)
		} else {
			priorityBreakdown += fmt.Sprintf("%d", combo.QualPriority)
		}

		if combo.AudioFormatPriority != 0 {
			if combo.AudioFormatPriority >= 0 {
				priorityBreakdown += fmt.Sprintf("+%d", combo.AudioFormatPriority)
			} else {
				priorityBreakdown += fmt.Sprintf("%d", combo.AudioFormatPriority)
			}
		}

		// Calculate priority change
		priorityChange := combo.TotalPriority - combo.OriginalTotalPriority

		var (
			changeDisplay gomponents.Node
			changeClass   string
		)

		if priorityChange > 0 {
			changeDisplay = html.Span(
				html.Class("text-success"),
				gomponents.Text(fmt.Sprintf("+%d", priorityChange)),
			)
			changeClass = "table-success"
		} else if priorityChange < 0 {
			changeDisplay = html.Span(
				html.Class("text-danger"),
				gomponents.Text(fmt.Sprintf("%d", priorityChange)),
			)
			changeClass = "table-warning"
		} else {
			changeDisplay = html.Span(html.Class("text-muted"), gomponents.Text("0"))
			changeClass = ""
		}

		rows = append(rows, html.Tr(
			gomponents.If(changeClass != "", html.Class(changeClass)),
			html.Td(
				html.Class("text-center"),
				html.Span(
					html.Class("badge bg-secondary"),
					gomponents.Text(fmt.Sprintf("%d", i+1)),
				),
			),
			html.Td(gomponents.Text(combinationDisplay)),
			html.Td(gomponents.Text(fmt.Sprintf("%d", combo.OriginalTotalPriority))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", combo.TotalPriority))),
			html.Td(html.Class("text-center"), changeDisplay),
			html.Td(html.Class("small text-muted"), gomponents.Text(priorityBreakdown)),
		))
	}

	var reorderInfo gomponents.Node

	// Build filter info
	filterInfo := ""
	if filterResolution != "" && filterResolution != "all" {
		filterInfo += fmt.Sprintf("Resolution: %s", filterResolution)
	}

	if filterQuality != "" && filterQuality != "all" {
		if filterInfo != "" {
			filterInfo += ", "
		}

		filterInfo += fmt.Sprintf("Quality: %s", filterQuality)
	}

	wantedInfo := fmt.Sprintf("Wanted Resolutions: %d, Wanted Qualities: %d",
		len(originalConfig.WantedResolution), len(originalConfig.WantedQuality))
	if len(originalConfig.WantedResolution) == 0 && len(originalConfig.WantedQuality) == 0 {
		wantedInfo = "All resolutions and qualities allowed (no filtering configured)"
	}

	// Build reorder rules info
	originalRulesCount := originalConfig.QualityReorderLen
	tempRulesCount := len(tempReorderRules)
	totalRulesCount := qualityConfig.QualityReorderLen

	var rulesInfo string
	if rulesProvided {
		rulesInfo = fmt.Sprintf("Reorder Rules: %d configured, %d active in this preview (%d from the editor)",
			originalRulesCount, totalRulesCount, tempRulesCount)
	} else if originalRulesCount > 0 {
		rulesInfo = fmt.Sprintf("Reorder Rules: %d configured", originalRulesCount)
	} else {
		rulesInfo = "No reorder rules configured - using default priorities"
	}

	alertClass := "alert alert-secondary mb-3"

	iconClass := "fas fa-cog me-2"
	if tempRulesCount > 0 {
		alertClass = "alert alert-warning mb-3"
		iconClass = "fas fa-flask me-2"
	} else if totalRulesCount > 0 {
		alertClass = "alert alert-info mb-3"
		iconClass = "fas fa-info-circle me-2"
	}

	reorderInfo = html.Div(
		html.Class(alertClass),
		html.H6(
			html.Class("alert-heading"),
			html.I(html.Class(iconClass)),
			gomponents.If(
				tempRulesCount > 0,
				gomponents.Text("Testing Mode - Temporary Rules Active"),
			),
			gomponents.If(tempRulesCount == 0, gomponents.Text("Quality Profile Information")),
		),
		html.P(html.Class("mb-1"), gomponents.Text(wantedInfo)),
		html.P(html.Class("mb-1"), gomponents.Text(rulesInfo)),
		gomponents.If(
			tempRulesCount > 0,
			html.P(
				html.Class("mb-1 text-warning"),
				html.Strong(
					gomponents.Text(
						"⚠ Temporary rules are active - changes are not saved to configuration",
					),
				),
			),
		),
		gomponents.If(
			filterInfo != "",
			html.P(
				html.Class("mb-0"),
				html.Strong(gomponents.Text("Active Filters: ")),
				gomponents.Text(filterInfo),
			),
		),
	)

	result := html.Div(
		html.Class("mt-3"),
		reorderInfo,
		html.Div(
			html.Class("card border-0 shadow-sm"),
			html.Div(
				html.Class("card-header bg-light"),
				html.H5(html.Class("card-title mb-0"),
					html.I(html.Class("fas fa-list-ol me-2 text-primary")),
					gomponents.Text("Wanted Quality Priority Combinations: "+selectedQuality),
				),
				html.P(
					html.Class("text-muted mb-0 mt-1"),
					gomponents.Text(
						fmt.Sprintf(
							"Showing %d resolution + quality combinations (sorted by total priority)",
							len(combinations),
						),
					),
				),
			),
			html.Div(
				html.Class("card-body p-0"),
				html.Table(
					html.Class("table table-hover mb-0"),
					html.THead(
						html.Class("table-light"),
						html.Tr(
							html.Th(html.Class("text-center"), gomponents.Text("Rank")),
							html.Th(gomponents.Text("Resolution + Quality")),
							html.Th(gomponents.Text("Original Priority")),
							html.Th(gomponents.Text("Modified Priority")),
							html.Th(gomponents.Text("Change")),
							html.Th(gomponents.Text("Breakdown (Res±Qual)")),
						),
					),
					html.TBody(gomponents.Group(rows)),
				),
			),
		),
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, renderComponentToString(result))
}

// HandleQualityProfileRules returns the existing reorder rules for a quality profile as JSON.
func HandleQualityProfileRules(c *gin.Context) {
	profileName := c.Query("profile")
	if profileName == "" {
		c.JSON(400, gin.H{"error": "Profile name is required"})
		return
	}

	qualityConfig := config.GetSettingsQuality(profileName)
	if qualityConfig == nil {
		c.JSON(404, gin.H{"error": "Quality profile not found"})
		return
	}

	type RuleResponse struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		NewPriority int    `json:"new_priority"`
	}

	var rules []RuleResponse
	for _, rule := range qualityConfig.QualityReorder {
		rules = append(rules, RuleResponse{
			Name:        rule.Name,
			Type:        rule.ReorderType,
			NewPriority: rule.Newpriority,
		})
	}

	c.JSON(200, rules)
}

func renderQualityReorderPage(csrfToken string) gomponents.Node {
	// Get all available quality profiles
	qualityConfigs := config.GetSettingsQualityAll()

	qualityOptions := make([]gomponents.Node, 0, 1+len(qualityConfigs))

	qualityOptions = append(
		qualityOptions,
		html.Option(html.Value(""), gomponents.Text("-- Select Quality Profile --")),
	)
	for _, qc := range qualityConfigs {
		qualityOptions = append(
			qualityOptions,
			html.Option(html.Value(qc.Name), gomponents.Text(qc.Name)),
		)
	}

	// Get all resolutions and qualities from database for filter options
	allResolutions := database.DBConnect.GetresolutionsIn
	allQualities := database.DBConnect.GetqualitiesIn

	var resolutionOptions []gomponents.Node

	resolutionOptions = append(
		resolutionOptions,
		html.Option(html.Value("all"), gomponents.Text("All Resolutions")),
	)
	for _, res := range allResolutions {
		if res.Name != "" {
			resolutionOptions = append(
				resolutionOptions,
				html.Option(html.Value(res.Name), gomponents.Text(res.Name)),
			)
		}
	}

	var qualityFilterOptions []gomponents.Node

	qualityFilterOptions = append(
		qualityFilterOptions,
		html.Option(html.Value("all"), gomponents.Text("All Qualities")),
	)
	for _, qual := range allQualities {
		if qual.Name != "" {
			qualityFilterOptions = append(
				qualityFilterOptions,
				html.Option(html.Value(qual.Name), gomponents.Text(qual.Name)),
			)
		}
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		// Enhanced page header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-sort-amount-down header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Quality Reorder Testing")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Experiment with quality settings and preview the resulting order and priority",
						),
					),
				),
			),
		),

		// Main form
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm"),
					html.Style("border-radius: 15px; overflow: hidden;"),
					html.Div(
						html.Class("card-header border-0"),
						html.Style(
							"background: linear-gradient(135deg, #fff 0%, #f8f9fa 100%); padding: 1.5rem;",
						),
						html.H5(html.Class("card-title mb-0"),
							html.I(html.Class("fas fa-cog me-2 text-primary")),
							gomponents.Text("Quality Profile Selection"),
						),
					),
					html.Div(
						html.Class("card-body p-4"),
						html.Form(
							html.ID("quality-reorder-form"),
							hx.Post("/api/admin/quality-reorder"),
							hx.Target("#quality-results"),
							hx.Swap("innerHTML"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							html.Input(
								html.Type("hidden"),
								html.Name("csrf_token"),
								html.Value(csrfToken),
							),

							html.Div(
								html.Class("mb-4"),
								html.Label(
									html.Class("form-label fw-semibold"),
									gomponents.Text("Quality Profile"),
								),
								html.Select(
									html.Class("form-select"),
									html.Name("selected_quality"),
									html.ID("selected_quality"),
									html.Required(),
									gomponents.Attr("hx-get", "/api/admin/quality-reorder/rules"),
									gomponents.Attr("hx-target", "#reorder-rules-container"),
									gomponents.Attr("hx-trigger", "change"),
									gomponents.Attr("hx-vals", "js:{profile: this.value}"),
									gomponents.Attr(
										"hx-headers",
										`{"X-CSRF-Token": "`+csrfToken+`"}`,
									),
									gomponents.Attr("hx-indicator", "#loading-indicator"),
									gomponents.Group(qualityOptions),
								),
								html.Div(
									html.Class("form-text"),
									gomponents.Text(
										"Select a quality profile to view its current ordering",
									),
								),
							),

							html.Div(
								html.Class("row mb-4"),
								html.Div(
									html.Class("col-md-6"),
									html.Label(
										html.Class("form-label fw-semibold"),
										gomponents.Text("Filter by Resolution"),
									),
									html.Select(
										html.Class("form-select"),
										html.Name("filter_resolution"),
										html.ID("filter_resolution"),
										gomponents.Group(resolutionOptions),
									),
									html.Div(
										html.Class("form-text"),
										gomponents.Text("Filter results to specific resolution"),
									),
								),
								html.Div(
									html.Class("col-md-6"),
									html.Label(
										html.Class("form-label fw-semibold"),
										gomponents.Text("Filter by Quality"),
									),
									html.Select(
										html.Class("form-select"),
										html.Name("filter_quality"),
										html.ID("filter_quality"),
										gomponents.Group(qualityFilterOptions),
									),
									html.Div(
										html.Class("form-text"),
										gomponents.Text(
											"Filter results to specific quality source",
										),
									),
								),
							),

							// Additional Filter Options Row
							html.Div(
								html.Class("row mb-4"),
								html.Div(
									html.Class("col-12"),
									html.Div(
										html.Class("card border-0 shadow-sm"),
										html.Style("border-radius: 10px;"),
										html.Div(
											html.Class("card-header border-0"),
											html.Style(
												"background: linear-gradient(135deg, #f8f9fa 0%, #e9ecef 100%); padding: 1rem;",
											),
											html.H6(html.Class("card-title mb-0"),
												html.I(html.Class("fas fa-filter me-2 text-info")),
												gomponents.Text("Filter Options"),
											),
										),
										html.Div(
											html.Class("card-body p-3"),
											renderFormGroup("",
												map[string]string{
													"wanted_only": "When enabled, only shows qualities that are actively wanted by the selected profile. When disabled, shows all available qualities regardless of wanted status.",
												},
												map[string]string{
													"wanted_only": "Show Wanted Qualities Only",
												},
												"wanted_only", "checkbox", false, nil),
										),
									),
								),
							),

							// Reorder Rules Management Section
							html.Div(
								html.Class("mb-4"),
								html.Div(
									html.Class(
										"d-flex justify-content-between align-items-center mb-3",
									),
									html.H6(html.Class("mb-0 fw-semibold"),
										html.I(html.Class("fas fa-flask me-2 text-warning")),
										gomponents.Text("Temporary Reorder Rules"),
									),
									html.Div(
										html.Class("btn-group"),
										html.Button(
											html.Type("button"),
											html.Class("btn btn-sm btn-outline-primary"),
											hx.Post("/api/admin/quality-reorder/add-rule"),
											hx.Target("#reorder-rules-container"),
											hx.Swap("beforeend"),
											hx.Headers(createHTMXHeaders(csrfToken)),
											html.I(html.Class("fas fa-plus me-1")),
											gomponents.Text("Add Rule"),
										),
										html.Button(
											html.Type("button"),
											html.Class("btn btn-sm btn-outline-secondary"),
											hx.Post("/api/admin/quality-reorder/reset-rules"),
											hx.Target("#reorder-rules-container"),
											hx.Swap("innerHTML"),
											hx.Headers(createHTMXHeaders(csrfToken)),
											hx.Confirm(
												"Are you sure you want to reset all temporary reorder rules?",
											),
											html.I(html.Class("fas fa-undo me-1")),
											gomponents.Text("Reset"),
										),
									),
								),
								// Rules headers
								html.Div(
									html.Class("row mb-2 px-3"),
									html.Div(
										html.Class("col-md-2"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("TYPE"),
										),
									),
									html.Div(
										html.Class("col-md-3"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("PATTERN"),
										),
									),
									html.Div(
										html.Class("col-md-2"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("PRIORITY (+/-)"),
										),
									),
									html.Div(
										html.Class("col-md-3"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("ENABLED"),
										),
									),
									html.Div(
										html.Class("col-md-2"),
										html.Small(
											html.Class("text-muted fw-semibold"),
											gomponents.Text("ACTIONS"),
										),
									),
								),
								html.Div(
									html.ID("reorder-rules-container"),
									html.Class("border rounded p-3 bg-light"),
									html.P(
										html.Class("text-muted mb-0 text-center"),
										gomponents.Text(
											"No temporary reorder rules. Select a quality profile above to load rules.",
										),
									),
								),
								html.Div(
									html.ID("loading-indicator"),
									html.Class("htmx-indicator text-center p-2"),
									html.I(html.Class("fas fa-spinner fa-spin me-2")),
									gomponents.Text("Loading rules..."),
								),
								html.Input(
									html.Type("hidden"),
									html.Name("reorder_rules"),
									html.ID("reorder_rules_json"),
									html.Value(""),
								),
								html.Div(
									html.Class("form-text mt-2"),
									gomponents.Text(
										"Add temporary reorder rules to test priority changes without modifying the actual configuration. Changes are not saved permanently. Priorities can be negative to reduce ranking.",
									),
								),
							),

							html.Div(
								html.Class("d-flex gap-2"),
								html.Button(
									html.Type("submit"),
									html.Class("btn btn-primary px-4"),
									hx.Post("/api/admin/quality-reorder"),
									hx.Target("#quality-results"),
									hx.Swap("innerHTML"),
									hx.Headers(createHTMXHeaders(csrfToken)),
									hx.Include("#quality-reorder-form"),
									hx.Indicator("#loading-indicator"),
									html.I(html.Class("fas fa-eye me-2")),
									gomponents.Text("Preview Quality Order"),
								),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-outline-secondary px-4"),
									html.ID("refresh-profiles"),
									html.I(html.Class("fas fa-sync-alt me-2")),
									gomponents.Text("Refresh Profiles"),
								),
							),
						),
					),
				),
			),
		),

		// Results container
		html.Div(
			html.ID("quality-results"),
			html.Class("mt-4"),
		),

		// Instructions card
		html.Div(
			html.Class("row mt-4"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card border-0 shadow-sm bg-light"),
					html.Div(
						html.Class("card-body p-4"),
						html.H6(html.Class("card-title text-muted mb-3"),
							html.I(html.Class("fas fa-info-circle me-2")),
							gomponents.Text("How to Use This Tool"),
						),
						html.Ul(
							html.Class("list-unstyled mb-0 text-muted small"),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"Select a quality profile from the dropdown to view its current ordering",
								),
							),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"Use filters to focus on specific resolutions or quality sources",
								),
							),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-flask text-warning me-2")),
								gomponents.Text(
									"Add temporary reorder rules to test priority changes (positive values increase ranking, negative decrease)",
								),
							),
							html.Li(
								html.Class("mb-2"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"Enable/disable rules to see how they affect priority calculations",
								),
							),
							html.Li(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-check text-success me-2")),
								gomponents.Text(
									"The results table shows resolution + quality combinations ranked by total priority",
								),
							),
						),
					),
				),
			),
		),

		// Complete JavaScript for Quality Reorder
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				
				// Refresh button handler
				$('#refresh-profiles').on('click', function() {
					location.reload();
				});

				// Profile selection handler - removed jQuery handler as HTMX handles this directly

				// Set once the admin edits/removes a rule, so an emptied rule list is
				// sent as "[]" (no rules) instead of "" (use the configured rules).
				var rulesTouched = false;
				$(document).on('click change', '#reorder-rules-container', function() { rulesTouched = true; });

				// Function to collect temporary reorder rules and update the JSON field
				function updateReorderRulesJSON() {
					var rows = $('#reorder-rules-container [data-rule-id]');
					if (!rulesTouched && rows.length === 0) {
						$('#reorder_rules_json').val('');
						return '';
					}
					var rules = [];
					$('#reorder-rules-container [data-rule-id]').each(function() {
						var $rule = $(this);
						var type = $rule.find('select[name="rule_type"]').val();
						var pattern = $rule.find('input[name="rule_pattern"]').val();
						var priority = parseInt($rule.find('input[name="rule_priority"]').val()) || 0;
						var enabled = $rule.find('input[name="rule_enabled"]').is(':checked');
						
						if (type && pattern && enabled) {
							rules.push({
								name: pattern,
								type: type,
								new_priority: priority
							});
						}
					});
					
					var json = JSON.stringify(rules);
					$('#reorder_rules_json').val(json);
					return json;
				}
				
				// Update JSON before form submission. htmx has already collected the
				// form values when configRequest fires, so the parameter is set too.
				$(document).on('htmx:configRequest', function(evt) {
					if (evt.target.id === 'quality-reorder-form' || $(evt.target).closest('#quality-reorder-form').length) {
						var json = updateReorderRulesJSON();
						var d = evt.originalEvent && evt.originalEvent.detail;
						if (d && d.parameters) {
							if (typeof d.parameters.set === 'function') { d.parameters.set('reorder_rules', json); } else { d.parameters['reorder_rules'] = json; }
						}
					}
				});
				
				// Update JSON when rules change
				$(document).on('change', '#reorder-rules-container input, #reorder-rules-container select', function() {
					updateReorderRulesJSON();
				});
				
				// Filter change handlers - trigger HTMX form submission
				$('#filter_resolution, #filter_quality, #wanted_only, #_wanted_only').on('change', function() {
					if ($('#selected_quality').val()) {
						// Use HTMX to trigger form submission
						htmx.trigger('#quality-reorder-form', 'submit');
					}
				});

				document.addEventListener('htmx:responseError', function(evt) {
					console.error('HTMX Response Error:', evt.detail);
				});
			});
		`)),
	)
}

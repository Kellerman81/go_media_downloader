package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	gin "github.com/gin-gonic/gin"
	. "maragu.dev/gomponents"
	. "maragu.dev/gomponents/html"
)

// Constants for common strings and values
const (
	// HTTP Status Classes
	AlertSuccess = "card border-0 shadow-sm border-success mb-4"
	AlertDanger  = "card border-0 shadow-sm border-danger mb-4"
	AlertWarning = "card border-0 shadow-sm border-warning mb-4"
	AlertInfo    = "card border-0 shadow-sm border-info mb-4"

	// CSS Classes
	ClassFormControl     = "form-control"
	ClassFormGroup       = "form-group"
	ClassFormCheck       = "form-check"
	ClassFormCheckInput  = "form-check-input-modern"
	ClassFormCheckSwitch = "form-check form-switch mb-3"
	ClassFormLabel       = "form-label fw-semibold"
	ClassFormCheckLabel  = "form-check-label fw-semibold"
	ClassFormSelect      = "form-select me-2"
	ClassBtn             = "btn"
	ClassBtnPrimary      = "btn btn-primary"
	ClassBtnSecondary    = "btn btn-secondary"
	ClassBtnSuccess      = "btn btn-success"
	ClassBtnDanger       = "btn btn-danger"
	ClassBtnWarning      = "btn btn-warning"
	ClassBtnInfo         = "btn btn-info"
	ClassBtnSm           = "btn-sm"
	ClassTable           = "table table-sm"
	ClassListGroup       = "list-group"
	ClassListGroupItem   = "list-group-item"
	ClassCard            = "card shadow-sm"
	ClassCardHeader      = "card-header bg-gradient text-dark"
	ClassCardBody        = "card-body bg-light"
	ClassCardBorder      = "card border-primary shadow-sm"
	ClassBadge           = "badge bg-primary"
	ClassCollapse        = "collapse"
	ClassArrayItem       = "array-item-enhanced card border-primary shadow-sm"
	ClassDFlex           = "d-flex mb-2"
	ClassMb3             = "mb-3"

	// Common Form Field Types
	FieldTypeText     = "text"
	FieldTypePassword = "password"
	FieldTypeNumber   = "number"
	FieldTypeCheckbox = "checkbox"
	FieldTypeSelect   = "select"
	FieldTypeArray    = "array"

	// API Endpoints
	APIAdminFeedParse       = "/api/admin/feedparse"
	APIAdminFolderStructure = "/api/admin/folderstructure"

	// Default Values
	DefaultLimit             = 0
	MaxDisplayItems          = 20
	DefaultProcessingTimeout = 30 * time.Second
)

// Helper functions for common operations

// Optimized config builder with field caching
type OptimizedConfigBuilder struct {
	c      *gin.Context
	prefix string
	index  string
	cache  map[string]string // Cache form values to avoid repeated lookups
}

// NewOptimizedConfigBuilder creates a new optimized config builder
func NewOptimizedConfigBuilder(c *gin.Context, prefix, index string) *OptimizedConfigBuilder {
	return &OptimizedConfigBuilder{
		c:      c,
		prefix: prefix,
		index:  index,
		cache:  make(map[string]string, 20), // Pre-allocate for common case
	}
}

// getString gets a string value with caching
func (b *OptimizedConfigBuilder) getString(field string) string {
	key := fmt.Sprintf("%s_%s_%s", b.prefix, b.index, field)
	if cached, exists := b.cache[key]; exists {
		return cached
	}
	value := b.c.PostForm(key)
	b.cache[key] = value
	return value
}

// getInt gets an int value with caching and default
func (b *OptimizedConfigBuilder) getInt(field string, defaultValue int) int {
	str := b.getString(field)
	if str == "" {
		return defaultValue
	}
	if parsed, err := strconv.Atoi(str); err == nil {
		return parsed
	}
	return defaultValue
}

// getBool gets a bool value with caching
func (b *OptimizedConfigBuilder) getBool(field string) bool {
	str := b.getString(field)
	return str == "on" || str == "true"
}

// getFloat32 gets a float32 value with caching and default
func (b *OptimizedConfigBuilder) getFloat32(field string, defaultValue float32) float32 {
	str := b.getString(field)
	if str == "" {
		return defaultValue
	}
	if parsed, err := strconv.ParseFloat(str, 32); err == nil {
		return float32(parsed)
	}
	return defaultValue
}

// getStringArray gets a string array value with efficient parsing
func (b *OptimizedConfigBuilder) getStringArray(field string) []string {
	key := fmt.Sprintf("%s_%s_%s", b.prefix, b.index, field)
	values := b.c.PostFormArray(key)

	// Filter out empty strings
	result := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			result = append(result, strings.TrimSpace(v))
		}
	}
	return result
}

// getIntArray gets an int array value with efficient parsing
func (b *OptimizedConfigBuilder) getIntArray(field string) []int {
	key := fmt.Sprintf("%s_%s_%s", b.prefix, b.index, field)
	values := b.c.PostFormArray(key)

	result := make([]int, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			if parsed, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
				result = append(result, parsed)
			}
		}
	}
	return result
}

// Cache for expensive template and configuration lookups
type configCache struct {
	templates map[string]map[string][]string
	mutex     sync.RWMutex
	lastClear time.Time
}

// getTemplatesWithCache caches template lookups to avoid repeated config reads
func getTemplatesWithCache(templateType string) []string {
	globalConfigCache.mutex.RLock()

	// Clear cache every 5 minutes to ensure fresh data
	if time.Since(globalConfigCache.lastClear) > 5*time.Minute {
		globalConfigCache.mutex.RUnlock()
		globalConfigCache.mutex.Lock()
		globalConfigCache.templates = make(map[string]map[string][]string)
		globalConfigCache.lastClear = time.Now()
		globalConfigCache.mutex.Unlock()
		globalConfigCache.mutex.RLock()
	}

	if cached, exists := globalConfigCache.templates[templateType]; exists {
		globalConfigCache.mutex.RUnlock()
		return cached["options"]
	}
	globalConfigCache.mutex.RUnlock()

	// Get fresh data and cache it
	globalConfigCache.mutex.Lock()
	defer globalConfigCache.mutex.Unlock()

	var templates []string
	switch templateType {
	case "path":
		templateMap := config.GetSettingTemplatesFor("path")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "list":
		templateMap := config.GetSettingTemplatesFor("list")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "quality":
		templateMap := config.GetSettingTemplatesFor("quality")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "scheduler":
		templateMap := config.GetSettingTemplatesFor("scheduler")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	case "notification":
		templateMap := config.GetSettingTemplatesFor("notification")
		if opts, exists := templateMap["options"]; exists {
			templates = opts
		}
	default:
		templates = []string{}
	}

	globalConfigCache.templates[templateType] = map[string][]string{"options": templates}
	return templates
}

// BatchOperationResult represents the result of a batch operation
type BatchOperationResult struct {
	Successful int
	Failed     int
	Errors     []error
}

// processBatchOperation processes multiple items with error collection
func processBatchOperation[T any](items []T, processor func(T) error) BatchOperationResult {
	result := BatchOperationResult{
		Errors: make([]error, 0),
	}

	for _, item := range items {
		if err := processor(item); err != nil {
			result.Failed++
			result.Errors = append(result.Errors, err)
		} else {
			result.Successful++
		}
	}

	return result
}

// getStringBuilder gets a string builder from the pool
func getStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// putStringBuilder returns a string builder to the pool
func putStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
}

// getNodeSlice gets a Node slice from the pool
func getNodeSlice() []Node {
	slice := nodeSlicePool.Get().([]Node)
	return slice[:0] // Reset length but keep capacity
}

// putNodeSlice returns a Node slice to the pool
func putNodeSlice(slice []Node) {
	if cap(slice) <= 100 { // Only pool reasonably sized slices
		nodeSlicePool.Put(slice)
	}
}

// Generic render config pattern to reduce duplication
type RenderConfigOptions struct {
	Title          string
	Subtitle       string
	Icon           string
	AddButtonText  string
	AddEndpoint    string
	FormContainer  string
	SubmitEndpoint string
}

// extractFormKeys extracts form keys based on a prefix and field suffix pattern
// Returns a map of keys found in the form data
func extractFormKeys(c *gin.Context, prefix string, fieldSuffix string) map[string]bool {
	formKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, fieldSuffix) || !strings.Contains(key, prefix) {
			continue
		}
		parts := strings.Split(key, "_")
		if len(parts) > 1 {
			keyIndex := ""
			// Find the correct index position based on prefix structure
			if strings.Contains(prefix, "media_main_") {
				if len(parts) > 3 {
					keyIndex = parts[3] // media_main_movies_X_Name -> X
				}
			} else {
				keyIndex = parts[1] // downloader_X_Name -> X
			}
			if keyIndex != "" {
				formKeys[keyIndex] = true
			}
		}
	}
	return formKeys
}

// ConfigSectionOptions holds configuration for rendering config sections
type ConfigSectionOptions struct {
	SectionTitle    string
	SectionSubtitle string
	SectionIcon     string
	ContainerID     string
	AddButtonText   string
	AddFormPath     string
	UpdatePath      string
}

// getFormField builds a form field name and retrieves its value
func getFormField(c *gin.Context, prefix string, index string, fieldName string) string {
	if index == "" {
		fieldKey := fmt.Sprintf("%s_%s", prefix, fieldName)
		return c.PostForm(fieldKey)
	}
	fieldKey := fmt.Sprintf("%s_%s_%s", prefix, index, fieldName)
	return c.PostForm(fieldKey)
}

func getFormFieldArray(c *gin.Context, prefix string, index string, fieldName string) []string {
	if index == "" {
		fieldKey := fmt.Sprintf("%s_%s", prefix, fieldName)
		return c.PostFormArray(fieldKey)
	}
	fieldKey := fmt.Sprintf("%s_%s_%s", prefix, index, fieldName)
	return c.PostFormArray(fieldKey)
}

// getFormFieldInt builds a form field name and retrieves its integer value
func getFormFieldInt(c *gin.Context, prefix string, index string, fieldName string) (int, error) {
	value := getFormField(c, prefix, index, fieldName)
	if value == "" {
		return 0, nil
	}
	return strconv.Atoi(value)
}

// validateName checks if a name field is not empty

// validateNonNegative checks if a value is not negative

// getFormFieldBool builds a form field name and retrieves its boolean value
func getFormFieldBool(c *gin.Context, prefix string, index string, fieldName string) bool {
	value := getFormField(c, prefix, index, fieldName)
	return value == "on" || value == "true" || value == "1"
}

// createDownloaderConfig creates a DownloaderConfig from form data
func createDownloaderConfig(index string, c *gin.Context) config.DownloaderConfig {
	var cfg config.DownloaderConfig
	builder := NewConfigBuilder(c, fmt.Sprintf("downloader_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.DlType, "DLType").
		SetString(&cfg.Hostname, "Hostname").
		SetInt(&cfg.Port, "Port").
		SetString(&cfg.Username, "Username").
		SetString(&cfg.Password, "Password").
		SetString(&cfg.DelugeDlTo, "DelugeDlTo").
		SetString(&cfg.DelugeMoveTo, "DelugeMoveTo").
		SetInt(&cfg.Priority, "Priority").
		SetBool(&cfg.AddPaused, "AddPaused").
		SetBool(&cfg.DelugeMoveAfter, "DelugeMoveAfter").
		SetBool(&cfg.Enabled, "Enabled")

	return cfg
}

// saveDownloaderConfigs saves the downloader configurations
func saveDownloaderConfigs(configs []config.DownloaderConfig) error {
	return saveConfig(configs)
}

// createListsConfig creates a ListsConfig from form data
func createListsConfig(index string, c *gin.Context) config.ListsConfig {
	var cfg config.ListsConfig
	builder := NewConfigBuilder(c, fmt.Sprintf("lists_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.ListType, "ListType").
		SetString(&cfg.URL, "URL").
		SetString(&cfg.IMDBCSVFile, "IMDBCSVFile").
		SetString(&cfg.SeriesConfigFile, "SeriesConfigFile").
		SetString(&cfg.TraktUsername, "TraktUsername").
		SetString(&cfg.TraktListName, "TraktListName").
		SetString(&cfg.TraktListType, "TraktListType").
		SetStringArray(&cfg.Excludegenre, "ExcludeGenre").
		SetStringArray(&cfg.Includegenre, "IncludeGenre").
		SetStringArray(&cfg.TmdbDiscover, "TmdbDiscover").
		SetIntArray(&cfg.TmdbList, "TmdbList").
		SetString(&cfg.Limit, "Limit").
		SetInt(&cfg.MinVotes, "MinVotes").
		SetFloat32(&cfg.MinRating, "MinRating").
		SetBool(&cfg.RemoveFromList, "RemoveFromList").
		SetString(&cfg.PlexServerURL, "PlexServerURL").
		SetString(&cfg.PlexToken, "PlexToken").
		SetString(&cfg.PlexUsername, "PlexUsername").
		SetString(&cfg.JellyfinServerURL, "JellyfinServerURL").
		SetString(&cfg.JellyfinToken, "JellyfinToken").
		SetString(&cfg.JellyfinUsername, "JellyfinUsername").
		SetBool(&cfg.Enabled, "Enabled")

	return cfg
}

// saveListsConfigs saves the lists configurations
func saveListsConfigs(configs []config.ListsConfig) error {
	return saveConfig(configs)
}

// createIndexersConfig creates an IndexersConfig from form data
func createIndexersConfig(index string, c *gin.Context) config.IndexersConfig {
	var cfg config.IndexersConfig
	builder := NewConfigBuilder(c, fmt.Sprintf("indexers_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.IndexerType, "IndexerType").
		SetString(&cfg.URL, "URL").
		SetString(&cfg.Apikey, "Apikey").
		SetString(&cfg.Userid, "Userid").
		SetBool(&cfg.Enabled, "Enabled").
		SetBool(&cfg.Rssenabled, "Rssenabled").
		SetBool(&cfg.Addquotesfortitlequery, "Addquotesfortitlequery").
		SetUint16(&cfg.MaxEntries, "MaxEntries").
		SetString(&cfg.MaxEntriesStr, "MaxEntriesStr").
		SetUint8(&cfg.RssEntriesloop, "RssEntriesloop").
		SetBool(&cfg.OutputAsJSON, "OutputAsJSON").
		SetString(&cfg.Customapi, "Customapi").
		SetString(&cfg.Customurl, "Customurl").
		SetString(&cfg.Customrssurl, "Customrssurl").
		SetString(&cfg.Customrsscategory, "Customrsscategory").
		SetInt(&cfg.Limitercalls, "Limitercalls").
		SetUint8(&cfg.Limiterseconds, "Limiterseconds").
		SetInt(&cfg.LimitercallsDaily, "LimitercallsDaily").
		SetUint16(&cfg.MaxAge, "MaxAge").
		SetBool(&cfg.DisableTLSVerify, "DisableTLSVerify").
		SetBool(&cfg.DisableCompression, "DisableCompression").
		SetUint16(&cfg.TimeoutSeconds, "TimeoutSeconds").
		SetBool(&cfg.TrustWithIMDBIDs, "TrustWithIMDBIDs").
		SetBool(&cfg.TrustWithTVDBIDs, "TrustWithTVDBIDs").
		SetBool(&cfg.CheckTitleOnIDSearch, "CheckTitleOnIDSearch")

	return cfg
}

// saveIndexersConfigs saves the indexers configurations
func saveIndexersConfigs(configs []config.IndexersConfig) error {
	return saveConfig(configs)
}

// createPathsConfig creates a PathsConfig from form data
func createPathsConfig(index string, c *gin.Context) config.PathsConfig {
	var cfg config.PathsConfig
	builder := NewConfigBuilder(c, fmt.Sprintf("paths_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.Path, "Path").
		SetStringArray(&cfg.AllowedVideoExtensions, "AllowedVideoExtensions").
		SetStringArray(&cfg.AllowedOtherExtensions, "AllowedOtherExtensions").
		SetStringArray(&cfg.AllowedVideoExtensionsNoRename, "AllowedVideoExtensionsNoRename").
		SetStringArray(&cfg.AllowedOtherExtensionsNoRename, "AllowedOtherExtensionsNoRename").
		SetStringArray(&cfg.Blocked, "Blocked").
		SetStringArray(&cfg.Disallowed, "Disallowed").
		SetStringArray(&cfg.AllowedLanguages, "AllowedLanguages").
		SetInt(&cfg.MaxSize, "MaxSize").
		SetInt(&cfg.MinSize, "MinSize").
		SetInt(&cfg.MinVideoSize, "MinVideoSize").
		SetInt(&cfg.CleanupsizeMB, "CleanupsizeMB").
		SetInt(&cfg.UpgradeScanInterval, "UpgradeScanInterval").
		SetInt(&cfg.MissingScanInterval, "MissingScanInterval").
		SetInt(&cfg.MissingScanReleaseDatePre, "MissingScanReleaseDatePre").
		SetInt(&cfg.MaxRuntimeDifference, "MaxRuntimeDifference").
		SetString(&cfg.PresortFolderPath, "PresortFolderPath").
		SetString(&cfg.MoveReplacedTargetPath, "MoveReplacedTargetPath").
		SetString(&cfg.SetChmod, "SetChmod").
		SetString(&cfg.SetChmodFolder, "SetChmodFolder").
		SetBool(&cfg.Upgrade, "Upgrade").
		SetBool(&cfg.Replacelower, "Replacelower").
		SetBool(&cfg.Usepresort, "Usepresort").
		SetBool(&cfg.DeleteWrongLanguage, "DeleteWrongLanguage").
		SetBool(&cfg.DeleteDisallowed, "DeleteDisallowed").
		SetBool(&cfg.CheckRuntime, "CheckRuntime").
		SetBool(&cfg.DeleteWrongRuntime, "DeleteWrongRuntime").
		SetBool(&cfg.MoveReplaced, "MoveReplaced")

	return cfg
}

// savePathsConfigs saves the paths configurations
func savePathsConfigs(configs []config.PathsConfig) error {
	return saveConfig(configs)
}

// createNotificationConfig creates a NotificationConfig from form data
func createNotificationConfig(index string, c *gin.Context) config.NotificationConfig {
	var cfg config.NotificationConfig
	builder := NewConfigBuilder(c, fmt.Sprintf("notification_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.NotificationType, "NotificationType").
		SetString(&cfg.Apikey, "Apikey").
		SetString(&cfg.Recipient, "Recipient").
		SetString(&cfg.Outputto, "Outputto").
		SetString(&cfg.ServerURL, "ServerURL").
		SetString(&cfg.AppriseURLs, "AppriseURLs")

	return cfg
}

// saveNotificationConfigs saves the notification configurations
func saveNotificationConfigs(configs []config.NotificationConfig) error {
	return saveConfig(configs)
}

// fieldNameToUserFriendly converts field names like "LogFileSize" to "Log File Size"
func fieldNameToUserFriendly(fieldName string) string {
	// Add space before capital letters (except the first one)
	re := regexp.MustCompile(`([a-z])([A-Z])`)
	result := re.ReplaceAllString(fieldName, `${1} ${2}`)

	// Handle common abbreviations and acronyms
	replacements := map[string]string{
		"Url":     "URL",
		"Id":      "ID",
		"Db":      "Database",
		"Api":     "API",
		"Http":    "HTTP",
		"Ssl":     "SSL",
		"Tls":     "TLS",
		"Csv":     "CSV",
		"Json":    "JSON",
		"Xml":     "XML",
		"Html":    "HTML",
		"Imdb":    "IMDB",
		"Tvdb":    "TVDB",
		"Tmdb":    "TMDB",
		"Newznab": "Newznab",
		"Nzb":     "NZB",
		"Rss":     "RSS",
		"Mb":      "MB",
		"Gb":      "GB",
		"Kb":      "KB",
	}

	for old, new := range replacements {
		result = strings.ReplaceAll(result, old, new)
	}

	return result
}

// createCommentLines splits comment text into formatted lines
func createCommentLines(comment string) []Node {
	var lineNodes []Node
	lines := strings.Split(comment, "\n")
	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			lineNodes = append(lineNodes, Div(
				Style("margin-bottom: 0.25rem;"),
				Text(trimmedLine),
			))
		}
	}
	return lineNodes
}

// Save function (implement according to your storage method)
func saveConfig(configv any) error {
	d, err := json.Marshal(configv)
	if err != nil {
		logger.Logtype("info", 0).Err(err).Msg("log struct failed")
		return err
	}
	logger.Logtype("info", 1).Str("data", string(d)).Msg("log struct")
	return config.UpdateCfgEntryAny(configv)
}

// createRegexConfig creates a RegexConfig from form data
func createRegexConfig(index string, c *gin.Context) config.RegexConfig {
	var cfg config.RegexConfig
	builder := NewConfigBuilder(c, fmt.Sprintf("regex_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetStringArrayFromForm(&cfg.Required, "Required").
		SetStringArrayFromForm(&cfg.Rejected, "Rejected")

	return cfg
}

// saveRegexConfigs saves regex configurations
func saveRegexConfigs(configs []config.RegexConfig) error {
	return saveConfig(configs)
}

// validateRegexConfig validates regex configuration
// validateRegexConfig validates regex configuration

// filterStringArray filters out empty strings from array
func filterStringArray(input []string) []string {
	var filtered []string
	for _, item := range input {
		if item != "" {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

// createQualityReorderConfigs creates QualityReorderConfig slice from form data
func createQualityReorderConfigs(index string, c *gin.Context) []config.QualityReorderConfig {
	subformKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_Name") || !strings.Contains(key, "quality_") || !strings.Contains(key, "_reorder_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[3]] = true
	}

	var configs []config.QualityReorderConfig
	for reorderIndex := range subformKeys {
		nameField := fmt.Sprintf("quality_%s_reorder_%s_Name", index, reorderIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		addConfig := config.QualityReorderConfig{
			Name: name,
		}

		if reorderType := c.PostForm(fmt.Sprintf("quality_%s_reorder_%s_ReorderType", index, reorderIndex)); reorderType != "" {
			addConfig.ReorderType = reorderType
		}

		if newPriority := c.PostForm(fmt.Sprintf("quality_%s_reorder_%s_Newpriority", index, reorderIndex)); newPriority != "" {
			if priority, err := strconv.Atoi(newPriority); err == nil {
				addConfig.Newpriority = priority
			}
		}

		configs = append(configs, addConfig)
	}
	return configs
}

// createQualityIndexerConfigs creates QualityIndexerConfig slice from form data
func createQualityIndexerConfigs(index string, c *gin.Context) []config.QualityIndexerConfig {
	subformKeys := make(map[string]bool)
	for key := range c.Request.PostForm {
		if !strings.Contains(key, "_TemplateIndexer") || !strings.Contains(key, "quality_") || !strings.Contains(key, "_indexer_") {
			continue
		}
		subformKeys[strings.Split(key, "_")[3]] = true
	}

	var configs []config.QualityIndexerConfig
	for indexerIndex := range subformKeys {
		nameField := fmt.Sprintf("quality_%s_indexer_%s_TemplateIndexer", index, indexerIndex)
		name := c.PostForm(nameField)
		if name == "" {
			continue
		}

		indexerConfig := config.QualityIndexerConfig{
			TemplateIndexer: name,
		}

		if templateDownloader := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplateDownloader", index, indexerIndex)); templateDownloader != "" {
			indexerConfig.TemplateDownloader = templateDownloader
		}

		if templateRegex := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplateRegex", index, indexerIndex)); templateRegex != "" {
			indexerConfig.TemplateRegex = templateRegex
		}

		if templatePathNzb := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_TemplatePathNzb", index, indexerIndex)); templatePathNzb != "" {
			indexerConfig.TemplatePathNzb = templatePathNzb
		}

		if categoryDownloader := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_CategoryDownloader", index, indexerIndex)); categoryDownloader != "" {
			indexerConfig.CategoryDownloader = categoryDownloader
		}

		if additionalQueryParams := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_AdditionalQueryParams", index, indexerIndex)); additionalQueryParams != "" {
			indexerConfig.AdditionalQueryParams = additionalQueryParams
		}

		if skipEmptySize := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_SkipEmptySize", index, indexerIndex)); skipEmptySize == "on" {
			indexerConfig.SkipEmptySize = true
		}

		if historyCheckTitle := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_HistoryCheckTitle", index, indexerIndex)); historyCheckTitle == "on" {
			indexerConfig.HistoryCheckTitle = true
		}

		if categoriesIndexer := c.PostForm(fmt.Sprintf("quality_%s_indexer_%s_CategoriesIndexer", index, indexerIndex)); categoriesIndexer != "" {
			indexerConfig.CategoriesIndexer = categoriesIndexer
		}

		configs = append(configs, indexerConfig)
	}
	return configs
}

// createQualityConfig creates a QualityConfig from form data
func createQualityConfig(index string, c *gin.Context) config.QualityConfig {
	builder := NewConfigBuilder(c, "quality_main", index)

	var qualityConfig config.QualityConfig

	builder.SetStringRequired(&qualityConfig.Name, "Name")
	if qualityConfig.Name == "" {
		return config.QualityConfig{}
	}

	builder.
		SetStringArrayFromForm(&qualityConfig.WantedResolution, "WantedResolution").
		SetStringArrayFromForm(&qualityConfig.WantedQuality, "WantedQuality").
		SetStringArrayFromForm(&qualityConfig.WantedAudio, "WantedAudio").
		SetStringArrayFromForm(&qualityConfig.WantedCodec, "WantedCodec").
		SetStringArrayFromForm(&qualityConfig.TitleStripSuffixForSearch, "TitleStripSuffixForSearch").
		SetStringArrayFromForm(&qualityConfig.TitleStripPrefixForSearch, "TitleStripPrefixForSearch").
		SetString(&qualityConfig.CutoffResolution, "CutoffResolution").
		SetString(&qualityConfig.CutoffQuality, "CutoffQuality").
		SetBool(&qualityConfig.SearchForTitleIfEmpty, "SearchForTitleIfEmpty").
		SetBool(&qualityConfig.BackupSearchForTitle, "BackupSearchForTitle").
		SetBool(&qualityConfig.SearchForAlternateTitleIfEmpty, "SearchForAlternateTitleIfEmpty").
		SetBool(&qualityConfig.BackupSearchForAlternateTitle, "BackupSearchForAlternateTitle").
		SetBool(&qualityConfig.ExcludeYearFromTitleSearch, "ExcludeYearFromTitleSearch").
		SetBool(&qualityConfig.CheckUntilFirstFound, "CheckUntilFirstFound").
		SetBool(&qualityConfig.CheckTitle, "CheckTitle").
		SetBool(&qualityConfig.CheckTitleOnIDSearch, "CheckTitleOnIDSearch").
		SetBool(&qualityConfig.CheckYear, "CheckYear").
		SetBool(&qualityConfig.CheckYear1, "CheckYear1").
		SetBool(&qualityConfig.UseForPriorityResolution, "UseForPriorityResolution").
		SetBool(&qualityConfig.UseForPriorityQuality, "UseForPriorityQuality").
		SetBool(&qualityConfig.UseForPriorityAudio, "UseForPriorityAudio").
		SetBool(&qualityConfig.UseForPriorityCodec, "UseForPriorityCodec").
		SetBool(&qualityConfig.UseForPriorityOther, "UseForPriorityOther").
		SetInt(&qualityConfig.UseForPriorityMinDifference, "UseForPriorityMinDifference")

	// Parse nested configurations
	qualityConfig.QualityReorder = createQualityReorderConfigs(index, c)
	qualityConfig.Indexer = createQualityIndexerConfigs(index, c)

	return qualityConfig
}

// saveQualityConfigs saves quality configurations
func saveQualityConfigs(configs []config.QualityConfig) error {
	return saveConfig(configs)
}

// createSchedulerConfig creates a SchedulerConfig from form data
func createSchedulerConfig(index string, c *gin.Context) config.SchedulerConfig {
	prefix := "scheduler"

	addConfig := config.SchedulerConfig{
		Name: getFormField(c, prefix, index, "Name"),
	}

	// Update interval fields
	if val := getFormField(c, prefix, index, "IntervalImdb"); val != "" {
		addConfig.IntervalImdb = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeeds"); val != "" {
		addConfig.IntervalFeeds = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshSeries"); val != "" {
		addConfig.IntervalFeedsRefreshSeries = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshMovies"); val != "" {
		addConfig.IntervalFeedsRefreshMovies = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshSeriesFull"); val != "" {
		addConfig.IntervalFeedsRefreshSeriesFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshMoviesFull"); val != "" {
		addConfig.IntervalFeedsRefreshMoviesFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissing"); val != "" {
		addConfig.IntervalIndexerMissing = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgrade"); val != "" {
		addConfig.IntervalIndexerUpgrade = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissingFull"); val != "" {
		addConfig.IntervalIndexerMissingFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgradeFull"); val != "" {
		addConfig.IntervalIndexerUpgradeFull = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissingTitle"); val != "" {
		addConfig.IntervalIndexerMissingTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgradeTitle"); val != "" {
		addConfig.IntervalIndexerUpgradeTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerMissingFullTitle"); val != "" {
		addConfig.IntervalIndexerMissingFullTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerUpgradeFullTitle"); val != "" {
		addConfig.IntervalIndexerUpgradeFullTitle = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerRss"); val != "" {
		addConfig.IntervalIndexerRss = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanData"); val != "" {
		addConfig.IntervalScanData = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanDataMissing"); val != "" {
		addConfig.IntervalScanDataMissing = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanDataFlags"); val != "" {
		addConfig.IntervalScanDataFlags = val
	}
	if val := getFormField(c, prefix, index, "IntervalScanDataimport"); val != "" {
		addConfig.IntervalScanDataimport = val
	}
	if val := getFormField(c, prefix, index, "IntervalDatabaseBackup"); val != "" {
		addConfig.IntervalDatabaseBackup = val
	}
	if val := getFormField(c, prefix, index, "IntervalDatabaseCheck"); val != "" {
		addConfig.IntervalDatabaseCheck = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerRssSeasons"); val != "" {
		addConfig.IntervalIndexerRssSeasons = val
	}
	if val := getFormField(c, prefix, index, "IntervalIndexerRssSeasonsAll"); val != "" {
		addConfig.IntervalIndexerRssSeasonsAll = val
	}

	// Update cron fields
	if val := getFormField(c, prefix, index, "CronIndexerRssSeasonsAll"); val != "" {
		addConfig.CronIndexerRssSeasonsAll = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerRssSeasons"); val != "" {
		addConfig.CronIndexerRssSeasons = val
	}
	if val := getFormField(c, prefix, index, "CronImdb"); val != "" {
		addConfig.CronImdb = val
	}
	if val := getFormField(c, prefix, index, "CronFeeds"); val != "" {
		addConfig.CronFeeds = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshSeries"); val != "" {
		addConfig.CronFeedsRefreshSeries = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshMovies"); val != "" {
		addConfig.CronFeedsRefreshMovies = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshSeriesFull"); val != "" {
		addConfig.CronFeedsRefreshSeriesFull = val
	}
	if val := getFormField(c, prefix, index, "CronFeedsRefreshMoviesFull"); val != "" {
		addConfig.CronFeedsRefreshMoviesFull = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissing"); val != "" {
		addConfig.CronIndexerMissing = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgrade"); val != "" {
		addConfig.CronIndexerUpgrade = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissingFull"); val != "" {
		addConfig.CronIndexerMissingFull = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgradeFull"); val != "" {
		addConfig.CronIndexerUpgradeFull = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissingTitle"); val != "" {
		addConfig.CronIndexerMissingTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgradeTitle"); val != "" {
		addConfig.CronIndexerUpgradeTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerMissingFullTitle"); val != "" {
		addConfig.CronIndexerMissingFullTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerUpgradeFullTitle"); val != "" {
		addConfig.CronIndexerUpgradeFullTitle = val
	}
	if val := getFormField(c, prefix, index, "CronIndexerRss"); val != "" {
		addConfig.CronIndexerRss = val
	}
	if val := getFormField(c, prefix, index, "CronScanData"); val != "" {
		addConfig.CronScanData = val
	}
	if val := getFormField(c, prefix, index, "CronScanDataMissing"); val != "" {
		addConfig.CronScanDataMissing = val
	}
	if val := getFormField(c, prefix, index, "CronScanDataFlags"); val != "" {
		addConfig.CronScanDataFlags = val
	}
	if val := getFormField(c, prefix, index, "CronScanDataimport"); val != "" {
		addConfig.CronScanDataimport = val
	}
	if val := getFormField(c, prefix, index, "CronDatabaseBackup"); val != "" {
		addConfig.CronDatabaseBackup = val
	}
	if val := getFormField(c, prefix, index, "CronDatabaseCheck"); val != "" {
		addConfig.CronDatabaseCheck = val
	}

	return addConfig
}

// saveSchedulerConfigs saves scheduler configurations
func saveSchedulerConfigs(configs []config.SchedulerConfig) error {
	return saveConfig(configs)
}

// ================================================================================
// CONFIGBUILDER SYSTEM - OPTIMIZED CONFIGURATION PROCESSING
// ================================================================================

// ConfigBuilder provides a fluent interface for building config structs from form data
type ConfigBuilder struct {
	context *gin.Context
	prefix  string
	index   string
}

// NewConfigBuilder creates a new ConfigBuilder instance
func NewConfigBuilder(c *gin.Context, prefix, index string) *ConfigBuilder {
	return &ConfigBuilder{
		context: c,
		prefix:  prefix,
		index:   index,
	}
}

// SetString sets a string field if the form value is not empty
func (cb *ConfigBuilder) SetString(target *string, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		*target = value
	}
	return cb
}

// SetStringRequired sets a string field (always, even if empty)
func (cb *ConfigBuilder) SetStringRequired(target *string, fieldName string) *ConfigBuilder {
	*target = getFormField(cb.context, cb.prefix, cb.index, fieldName)
	return cb
}

// SetInt sets an int field if the form value is valid
func (cb *ConfigBuilder) SetInt(target *int, fieldName string) *ConfigBuilder {
	if value, err := getFormFieldInt(cb.context, cb.prefix, cb.index, fieldName); err == nil {
		*target = value
	}
	return cb
}

// SetBool sets a bool field from form checkbox/toggle values
func (cb *ConfigBuilder) SetBool(target *bool, fieldName string) *ConfigBuilder {
	*target = getFormFieldBool(cb.context, cb.prefix, cb.index, fieldName)
	return cb
}

// SetStringArray sets a string array field by splitting comma-separated values
func (cb *ConfigBuilder) SetStringArray(target *[]string, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		*target = strings.Split(value, ",")
		// Clean up whitespace
		for i, v := range *target {
			(*target)[i] = strings.TrimSpace(v)
		}
	}
	return cb
}

func (cb *ConfigBuilder) SetStringMultiSelectArray(target *[]string, fieldName string) *ConfigBuilder {
	value := getFormFieldArray(cb.context, cb.prefix, cb.index, fieldName)
	(*target) = value
	return cb
}

// SetFloat32 sets a float32 field if the form value is valid
func (cb *ConfigBuilder) SetFloat32(target *float32, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		if floatVal, err := strconv.ParseFloat(value, 32); err == nil {
			*target = float32(floatVal)
		}
	}
	return cb
}

// SetUint8 sets a uint8 field if the form value is valid
func (cb *ConfigBuilder) SetUint8(target *uint8, fieldName string) *ConfigBuilder {
	if value, err := getFormFieldInt(cb.context, cb.prefix, cb.index, fieldName); err == nil && value >= 0 && value <= 255 {
		*target = uint8(value)
	}
	return cb
}

// SetStringArrayFromForm sets a string array field from PostFormArray
func (cb *ConfigBuilder) SetStringArrayFromForm(target *[]string, fieldName string) *ConfigBuilder {
	fieldKey := fmt.Sprintf("%s_%s_%s", cb.prefix, cb.index, fieldName)
	if values := cb.context.PostFormArray(fieldKey); len(values) > 0 {
		*target = filterStringArray(values)
	}
	return cb
}

// SetUint16 sets a uint16 field if the form value is valid
func (cb *ConfigBuilder) SetUint16(target *uint16, fieldName string) *ConfigBuilder {
	if value, err := getFormFieldInt(cb.context, cb.prefix, cb.index, fieldName); err == nil && value >= 0 && value <= 65535 {
		*target = uint16(value)
	}
	return cb
}

// SetIntArray sets an int array field by parsing comma-separated values
func (cb *ConfigBuilder) SetIntArray(target *[]int, fieldName string) *ConfigBuilder {
	if value := getFormField(cb.context, cb.prefix, cb.index, fieldName); value != "" {
		parts := strings.Split(value, ",")
		var intArray []int
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				if intVal, err := strconv.Atoi(trimmed); err == nil {
					intArray = append(intArray, intVal)
				}
			}
		}
		if len(intArray) > 0 {
			*target = intArray
		}
	}
	return cb
}

func createImdbConfigFields(configv *config.ImdbConfig) []FormFieldDefinition {
	return []FormFieldDefinition{
		{Name: "Indexedtypes", Type: "selectarray", Value: configv.Indexedtypes, Options: map[string][]string{
			"options": {"movie", "tvMovie", "tvmovie", "tvSeries", "tvseries", "video"},
		}},
		{Name: "Indexedlanguages", Type: "array", Value: configv.Indexedlanguages},
		{Name: "Indexfull", Type: "checkbox", Value: configv.Indexfull},
		{Name: "ImdbIDSize", Type: "number", Value: configv.ImdbIDSize},
		{Name: "LoopSize", Type: "number", Value: configv.LoopSize},
		{Name: "UseMemory", Type: "checkbox", Value: configv.UseMemory},
		{Name: "UseCache", Type: "checkbox", Value: configv.UseCache},
	}
}

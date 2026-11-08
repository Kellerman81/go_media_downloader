package api

import (
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// Constants for common strings and values.
const (
	// AlertSuccess is a CSS card class for success alert state.
	AlertSuccess = "card border-0 shadow-sm border-success mb-4"
	AlertDanger  = "card border-0 shadow-sm border-danger mb-4"
	AlertWarning = "card border-0 shadow-sm border-warning mb-4"
	AlertInfo    = "card border-0 shadow-sm border-info mb-4"

	// ClassFormControl is the CSS class for form control elements.
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

	// FieldTypeText is the HTML input type value for text fields.
	FieldTypeText     = "text"
	FieldTypePassword = "password"
	FieldTypeNumber   = "number"
	FieldTypeCheckbox = "checkbox"
	FieldTypeSelect   = "select"
	FieldTypeArray    = "array"

	// APIAdminFeedParse is the URL path for the admin feed parse endpoint.
	APIAdminFeedParse       = "/api/admin/feedparse"
	APIAdminFolderStructure = "/api/admin/folderstructure"

	// DefaultLimit is the default pagination limit value (0 means no limit).
	DefaultLimit             = 0
	MaxDisplayItems          = 20
	DefaultProcessingTimeout = 30 * time.Second

	// arrayPresentSuffix names the hidden marker input that array widgets
	// render next to their entries, so a list emptied in the UI is still
	// posted (and saved as empty) instead of looking like an absent field.
	arrayPresentSuffix = "__present"
)

// Helper functions for common operations

// Cache for expensive template and configuration lookups.
// type configCache struct {
// 	templates map[string]map[string][]string
// 	mutex     sync.RWMutex
// 	lastClear time.Time
// }

// // getTemplatesWithCache caches template lookups to avoid repeated config reads.
// func getTemplatesWithCache(templateType string) []string {
// 	globalConfigCache.mutex.RLock()

// 	// Clear cache every 5 minutes to ensure fresh data
// 	if time.Since(globalConfigCache.lastClear) > 5*time.Minute {
// 		globalConfigCache.mutex.RUnlock()
// 		globalConfigCache.mutex.Lock()

// 		globalConfigCache.templates = make(map[string]map[string][]string)
// 		globalConfigCache.lastClear = time.Now()
// 		globalConfigCache.mutex.Unlock()
// 		globalConfigCache.mutex.RLock()
// 	}

// 	if cached, exists := globalConfigCache.templates[templateType]; exists {
// 		globalConfigCache.mutex.RUnlock()
// 		return cached["options"]
// 	}

// 	globalConfigCache.mutex.RUnlock()

// 	// Get fresh data and cache it
// 	globalConfigCache.mutex.Lock()
// 	defer globalConfigCache.mutex.Unlock()

// 	var templates []string
// 	switch templateType {
// 	case "path":
// 		templateMap := config.GetSettingTemplatesFor("path")
// 		if opts, exists := templateMap["options"]; exists {
// 			templates = opts
// 		}

// 	case "list":
// 		templateMap := config.GetSettingTemplatesFor("list")
// 		if opts, exists := templateMap["options"]; exists {
// 			templates = opts
// 		}

// 	case "quality":
// 		templateMap := config.GetSettingTemplatesFor("quality")
// 		if opts, exists := templateMap["options"]; exists {
// 			templates = opts
// 		}

// 	case "scheduler":
// 		templateMap := config.GetSettingTemplatesFor("scheduler")
// 		if opts, exists := templateMap["options"]; exists {
// 			templates = opts
// 		}

// 	case "notification":
// 		templateMap := config.GetSettingTemplatesFor("notification")
// 		if opts, exists := templateMap["options"]; exists {
// 			templates = opts
// 		}

// 	default:
// 		templates = []string{}
// 	}

// 	globalConfigCache.templates[templateType] = map[string][]string{"options": templates}

// 	return templates
// }

// BatchOperationResult represents the result of a batch operation.
type BatchOperationResult struct {
	Successful int
	Failed     int
	Errors     []error
}

// processBatchOperation processes multiple items with error collection.
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

// getStringBuilder gets a string builder from the pool.
func getStringBuilder() *strings.Builder {
	sb := stringBuilderPool.Get().(*strings.Builder)
	sb.Reset()
	return sb
}

// putStringBuilder returns a string builder to the pool.
func putStringBuilder(sb *strings.Builder) {
	stringBuilderPool.Put(sb)
}

// getNodeSlice gets a Node slice from the pool.
func getNodeSlice() []gomponents.Node {
	p := nodeSlicePool.Get().(*[]gomponents.Node)
	s := (*p)[:0]
	return s
}

// putNodeSlice returns a Node slice to the pool.
func putNodeSlice(slice []gomponents.Node) {
	if cap(slice) <= 100 { // Only pool reasonably sized slices
		nodeSlicePool.Put(&slice)
	}
}

// RenderConfigOptions holds options for rendering a config page.
type RenderConfigOptions struct {
	Title          string
	Subtitle       string
	Icon           string
	AddButtonText  string
	AddEndpoint    string
	FormContainer  string
	SubmitEndpoint string
}

// formRowIndexes returns the distinct numeric row indexes n of form keys
// shaped prefix+n+suffix, sorted ascending. Rows are rendered with increasing
// indexes and added rows get max+1, so this is the order the rows appear in
// the form. Keys whose middle part is not a number are ignored, which keeps
// nested rows that share the prefix (e.g. "..._data_0_dataimport_...") out.
func formRowIndexes(form url.Values, prefix, suffix string) []string {
	seen := make(map[int]struct{})
	for key := range form {
		mid, ok := strings.CutPrefix(key, prefix)
		if !ok {
			continue
		}

		mid, ok = strings.CutSuffix(mid, suffix)
		if !ok {
			continue
		}

		n, err := strconv.Atoi(mid)
		if err != nil || n < 0 {
			continue
		}

		seen[n] = struct{}{}
	}

	ints := make([]int, 0, len(seen))
	for n := range seen {
		ints = append(ints, n)
	}

	slices.Sort(ints)

	out := make([]string, len(ints))
	for i, n := range ints {
		out[i] = strconv.Itoa(n)
	}

	return out
}

// nextFormRowIndex returns an index that is not used by any posted row with
// the given prefix/suffix (max existing + 1), so a newly added row never
// collides with an existing one, even after rows were removed.
func nextFormRowIndex(form url.Values, prefix, suffix string) int {
	next := 0
	for _, idx := range formRowIndexes(form, prefix, suffix) {
		if n, _ := strconv.Atoi(idx); n >= next {
			next = n + 1
		}
	}

	return next
}

// nextRowIndexFromForm parses the posted form of an add-row request and returns
// the index for the new row. On a parse error it writes an empty response and
// returns false.
func nextRowIndexFromForm(ctx *gin.Context, prefix, suffix string) (int, bool) {
	if err := ctx.Request.ParseForm(); err != nil {
		ctx.String(http.StatusOK, "")
		return 0, false
	}

	return nextFormRowIndex(ctx.Request.PostForm, prefix, suffix), true
}

// renderAddedRow writes a rendered add-row fragment as HTML.
func renderAddedRow(ctx *gin.Context, form gomponents.Node) {
	var buf strings.Builder
	form.Render(&buf)
	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// ConfigSectionOptions holds configuration for rendering config sections.
type ConfigSectionOptions struct {
	SectionTitle    string
	SectionSubtitle string
	SectionIcon     string
	ContainerID     string
	AddButtonText   string
	AddFormPath     string
	UpdatePath      string
}

// getFormField builds a form field name and retrieves its value.
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

// validateName checks if a name field is not empty

// validateNonNegative checks if a value is not negative

// formBool interprets a posted checkbox/toggle value. Checkboxes without an
// explicit value attribute post "on".
func formBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on", "true", "1", "yes":
		return true
	default:
		return false
	}
}

// getFormFieldBool builds a form field name and retrieves its boolean value.
func getFormFieldBool(c *gin.Context, prefix string, index string, fieldName string) bool {
	return formBool(getFormField(c, prefix, index, fieldName))
}

// formFieldKey builds the posted key for a field, matching getFormField.
func formFieldKey(prefix, index, fieldName string) string {
	if index == "" {
		return prefix + "_" + fieldName
	}

	return prefix + "_" + index + "_" + fieldName
}

// formFieldPresent reports whether the field (or the presence marker that
// array widgets render, so an emptied list is still detectable) was posted.
func formFieldPresent(c *gin.Context, key string) bool {
	if _, ok := c.Request.PostForm[key]; ok {
		return true
	}

	_, ok := c.Request.PostForm[key+arrayPresentSuffix]

	return ok
}

// createDownloaderConfig creates a DownloaderConfig from form data.
func createDownloaderConfig(index string, c *gin.Context) (config.DownloaderConfig, error) {
	var cfg config.DownloaderConfig

	builder := NewConfigBuilder(c, fmt.Sprintf("downloader_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.DlType, "DlType").
		SetString(&cfg.Hostname, "Hostname").
		SetInt(&cfg.Port, "Port").
		SetString(&cfg.Username, "Username").
		SetString(&cfg.Password, "Password").
		SetString(&cfg.DelugeDlTo, "DelugeDlTo").
		SetString(&cfg.DelugeMoveTo, "DelugeMoveTo").
		SetInt(&cfg.Priority, "Priority").
		SetBool(&cfg.AddPaused, "AddPaused").
		SetBool(&cfg.DelugeMoveAfter, "DelugeMoveAfter").
		SetBool(&cfg.RemoveFailed, "RemoveFailed").
		SetBool(&cfg.Enabled, "Enabled")

	return cfg, builder.Err()
}

// saveDownloaderConfigs saves the downloader configurations.
func saveDownloaderConfigs(configs []config.DownloaderConfig) error {
	if err := saveConfig(configs); err != nil {
		return err
	}

	providers.ReloadClientProviders()

	return nil
}

// createListsConfig creates a ListsConfig from form data.
func createListsConfig(index string, c *gin.Context) (config.ListsConfig, error) {
	var cfg config.ListsConfig

	builder := NewConfigBuilder(c, fmt.Sprintf("lists_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.ListType, "ListType").
		SetString(&cfg.URL, "URL").
		SetString(&cfg.IMDBCSVFile, "IMDBCSVFile").
		SetString(&cfg.ManualConfigFile, "ManualConfigFile").
		SetString(&cfg.TraktUsername, "TraktUsername").
		SetString(&cfg.TraktListName, "TraktListName").
		SetString(&cfg.TraktListType, "TraktListType").
		SetStringArrayFromForm(&cfg.Excludegenre, "Excludegenre").
		SetStringArrayFromForm(&cfg.Includegenre, "Includegenre").
		SetStringArrayFromForm(&cfg.TmdbDiscover, "TmdbDiscover").
		SetIntArray(&cfg.TmdbList, "TmdbList").
		SetInt(&cfg.TmdbPages, "TmdbPages").
		SetString(&cfg.Limit, "Limit").
		SetInt(&cfg.MinVotes, "MinVotes").
		SetFloat32(&cfg.MinRating, "MinRating").
		SetBool(&cfg.RemoveFromList, "RemoveFromList").
		SetBool(&cfg.SkipEmptySize, "SkipEmptySize").
		SetString(&cfg.PlexServerURL, "PlexServerURL").
		SetString(&cfg.PlexToken, "PlexToken").
		SetString(&cfg.PlexUsername, "PlexUsername").
		SetString(&cfg.JellyfinServerURL, "JellyfinServerURL").
		SetString(&cfg.JellyfinToken, "JellyfinToken").
		SetString(&cfg.JellyfinUsername, "JellyfinUsername").
		// IRC announce
		SetString(&cfg.IRCServer, "IRCServer").
		SetBool(&cfg.IRCUseTLS, "IRCUseTLS").
		SetString(&cfg.IRCNick, "IRCNick").
		SetString(&cfg.IRCNickServPassword, "IRCNickServPassword").
		SetString(&cfg.IRCInviteCommand, "IRCInviteCommand").
		SetStringArrayFromForm(&cfg.IRCChannels, "IRCChannels").
		SetStringArrayFromForm(&cfg.IRCAnnounceNicks, "IRCAnnounceNicks").
		SetString(&cfg.IRCAnnounceRegex, "IRCAnnounceRegex").
		SetInt(&cfg.IRCReadSeconds, "IRCReadSeconds").
		SetBool(&cfg.Enabled, "Enabled").
		// Movie Scraper Configuration
		SetString(&cfg.MovieScraperType, "MovieScraperType").
		SetString(&cfg.MovieScraperStartURL, "MovieScraperStartURL").
		SetString(&cfg.MovieScraperSiteURL, "MovieScraperSiteURL").
		SetUint(&cfg.MovieScraperSiteID, "MovieScraperSiteID").
		SetString(&cfg.MovieSceneNodeXPath, "MovieSceneNodeXPath").
		SetString(&cfg.MovieTitleXPath, "MovieTitleXPath").
		SetString(&cfg.MovieYearXPath, "MovieYearXPath").
		SetString(&cfg.MovieImdbIDXPath, "MovieImdbIDXPath").
		SetString(&cfg.MovieURLXPath, "MovieURLXPath").
		SetString(&cfg.MovieRatingXPath, "MovieRatingXPath").
		SetString(&cfg.MovieGenreXPath, "MovieGenreXPath").
		SetString(&cfg.MovieReleaseDateXPath, "MovieReleaseDateXPath").
		SetString(&cfg.MovieTitleAttribute, "MovieTitleAttribute").
		SetString(&cfg.MovieURLAttribute, "MovieURLAttribute").
		SetString(&cfg.MoviePaginationType, "MoviePaginationType").
		SetInt(&cfg.MoviePageIncrement, "MoviePageIncrement").
		SetString(&cfg.MoviePageURLPattern, "MoviePageURLPattern").
		SetString(&cfg.MovieCSRFCookieName, "MovieCSRFCookieName").
		SetString(&cfg.MovieCSRFHeaderName, "MovieCSRFHeaderName").
		SetString(&cfg.MovieAPIURLPattern, "MovieAPIURLPattern").
		SetInt(&cfg.MoviePageStartIndex, "MoviePageStartIndex").
		SetString(&cfg.MovieResultsArrayPath, "MovieResultsArrayPath").
		SetString(&cfg.MovieTitleField, "MovieTitleField").
		SetString(&cfg.MovieYearField, "MovieYearField").
		SetString(&cfg.MovieImdbIDField, "MovieImdbIDField").
		SetString(&cfg.MovieURLField, "MovieURLField").
		SetString(&cfg.MovieRatingField, "MovieRatingField").
		SetString(&cfg.MovieGenreField, "MovieGenreField").
		SetString(&cfg.MovieReleaseDateField, "MovieReleaseDateField").
		SetString(&cfg.MovieDateFormat, "MovieDateFormat").
		SetInt(&cfg.MovieWaitSeconds, "MovieWaitSeconds").
		// Chart / bestseller scraper
		SetString(&cfg.ChartEntryNodeXPath, "ChartEntryNodeXPath").
		SetString(&cfg.ChartTitleXPath, "ChartTitleXPath").
		SetString(&cfg.ChartArtistXPath, "ChartArtistXPath").
		SetString(&cfg.ChartTitleAttribute, "ChartTitleAttribute").
		SetString(&cfg.ChartArtistAttribute, "ChartArtistAttribute").
		SetString(&cfg.ChartDefaultArtist, "ChartDefaultArtist").
		SetString(&cfg.ChartDateURLPattern, "ChartDateURLPattern").
		SetString(&cfg.ChartDateFormat, "ChartDateFormat")

	return cfg, builder.Err()
}

// saveListsConfigs saves the lists configurations.
func saveListsConfigs(configs []config.ListsConfig) error {
	return saveConfig(configs)
}

// createIndexersConfig creates an IndexersConfig from form data.
func createIndexersConfig(index string, c *gin.Context) (config.IndexersConfig, error) {
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

	return cfg, builder.Err()
}

// saveIndexersConfigs saves the indexers configurations.
func saveIndexersConfigs(configs []config.IndexersConfig) error {
	if err := saveConfig(configs); err != nil {
		return err
	}

	providers.ReloadClientProviders()

	return nil
}

// createPathsConfig creates a PathsConfig from form data.
func createPathsConfig(index string, c *gin.Context) (config.PathsConfig, error) {
	var cfg config.PathsConfig

	builder := NewConfigBuilder(c, fmt.Sprintf("paths_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.Path, "Path").
		SetStringArrayFromForm(&cfg.AllowedVideoExtensions, "AllowedVideoExtensions").
		SetStringArrayFromForm(&cfg.AllowedOtherExtensions, "AllowedOtherExtensions").
		SetStringArrayFromForm(&cfg.AllowedVideoExtensionsNoRename, "AllowedVideoExtensionsNoRename").
		SetStringArrayFromForm(&cfg.AllowedOtherExtensionsNoRename, "AllowedOtherExtensionsNoRename").
		SetStringArrayFromForm(&cfg.AllowedAudioExtensions, "AllowedAudioExtensions").
		SetStringArrayFromForm(&cfg.AllowedAudioExtensionsNoRename, "AllowedAudioExtensionsNoRename").
		SetStringArrayFromForm(&cfg.AllowedBookExtensions, "AllowedBookExtensions").
		SetStringArrayFromForm(&cfg.AllowedBookExtensionsNoRename, "AllowedBookExtensionsNoRename").
		SetStringArrayFromForm(&cfg.Blocked, "Blocked").
		SetStringArrayFromForm(&cfg.Disallowed, "Disallowed").
		SetStringArrayFromForm(&cfg.AllowedLanguages, "AllowedLanguages").
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

	return cfg, builder.Err()
}

// savePathsConfigs saves the paths configurations.
func savePathsConfigs(configs []config.PathsConfig) error {
	return saveConfig(configs)
}

// createNotificationConfig creates a NotificationConfig from form data.
func createNotificationConfig(index string, c *gin.Context) (config.NotificationConfig, error) {
	var cfg config.NotificationConfig

	builder := NewConfigBuilder(c, fmt.Sprintf("notifications_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetString(&cfg.NotificationType, "NotificationType").
		SetString(&cfg.Apikey, "Apikey").
		SetString(&cfg.Recipient, "Recipient").
		SetString(&cfg.Sender, "Sender").
		SetString(&cfg.Command, "Command").
		SetString(&cfg.Outputto, "Outputto").
		SetString(&cfg.ServerURL, "ServerURL").
		SetString(&cfg.AppriseURLs, "AppriseURLs").
		SetString(&cfg.SMTPServer, "SMTPServer").
		SetString(&cfg.SMTPPort, "SMTPPort").
		SetString(&cfg.SMTPFromEmail, "SMTPFromEmail").
		SetString(&cfg.SMTPToEmail, "SMTPToEmail").
		SetString(&cfg.SMTPUsername, "SMTPUsername").
		SetString(&cfg.SMTPPassword, "SMTPPassword")

	return cfg, builder.Err()
}

// saveNotificationConfigs saves the notification configurations.
func saveNotificationConfigs(configs []config.NotificationConfig) error {
	if err := saveConfig(configs); err != nil {
		return err
	}

	providers.RegisterNotificationProviders()

	return nil
}

// fieldNameToUserFriendly converts field names like "LogFileSize" to "Log File Size".
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

// createCommentLines splits comment text into formatted lines.
func createCommentLines(comment string) []gomponents.Node {
	var lineNodes []gomponents.Node

	lines := strings.SplitSeq(comment, "\n")
	for line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" {
			lineNodes = append(lineNodes, html.Div(
				html.Style("margin-bottom: 0.25rem;"),
				gomponents.Text(trimmedLine),
			))
		}
	}

	return lineNodes
}

// Save function (implement according to your storage method).
func saveConfig(configv any) error {
	logger.Logtype("info", 1).Str("type", fmt.Sprintf("%T", configv)).Msg("saving configuration")

	return config.UpdateCfgEntryAny(configv)
}

// createRegexConfig creates a RegexConfig from form data.
func createRegexConfig(index string, c *gin.Context) (config.RegexConfig, error) {
	var cfg config.RegexConfig

	builder := NewConfigBuilder(c, fmt.Sprintf("regex_%s", index), "")

	builder.
		SetStringRequired(&cfg.Name, "Name").
		SetStringArrayFromForm(&cfg.Required, "Required").
		SetStringArrayFromForm(&cfg.Rejected, "Rejected")

	return cfg, builder.Err()
}

// saveRegexConfigs saves regex configurations.
func saveRegexConfigs(configs []config.RegexConfig) error {
	return saveConfig(configs)
}

// validateRegexConfig validates regex configuration
// validateRegexConfig validates regex configuration

// filterStringArray filters out empty strings from array.
func filterStringArray(input []string) []string {
	var filtered []string
	for _, item := range input {
		if item != "" {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// createQualityReorderConfigs creates QualityReorderConfig slice from form data.
func createQualityReorderConfigs(
	index string,
	c *gin.Context,
) ([]config.QualityReorderConfig, error) {
	prefix := "quality_" + index + "_reorder"

	var (
		configs []config.QualityReorderConfig
		errs    []error
	)

	for _, reorderIndex := range formRowIndexes(c.Request.PostForm, prefix+"_", "_Name") {
		builder := NewConfigBuilder(c, prefix+"_"+reorderIndex, "")

		var addConfig config.QualityReorderConfig

		builder.SetStringRequired(&addConfig.Name, "Name")

		// Unnamed rules are dropped as before: the default config ships one.
		if addConfig.Name == "" {
			continue
		}

		builder.SetString(&addConfig.ReorderType, "ReorderType").
			SetInt(&addConfig.Newpriority, "Newpriority")

		if err := builder.Err(); err != nil {
			errs = append(errs, err)
		}

		configs = append(configs, addConfig)
	}

	return configs, errors.Join(errs...)
}

// createQualityIndexerConfigs creates QualityIndexerConfig slice from form data.
// The slice order is the indexer search order, so it follows the form order.
func createQualityIndexerConfigs(index string, c *gin.Context) []config.QualityIndexerConfig {
	prefix := "quality_" + index + "_indexer"

	var configs []config.QualityIndexerConfig

	for _, indexerIndex := range formRowIndexes(c.Request.PostForm, prefix+"_", "_TemplateIndexer") {
		builder := NewConfigBuilder(c, prefix+"_"+indexerIndex, "")

		var indexerConfig config.QualityIndexerConfig

		builder.SetStringRequired(&indexerConfig.TemplateIndexer, "TemplateIndexer")

		if indexerConfig.TemplateIndexer == "" {
			continue
		}

		builder.SetString(&indexerConfig.TemplateDownloader, "TemplateDownloader").
			SetString(&indexerConfig.TemplateRegex, "TemplateRegex").
			SetString(&indexerConfig.TemplatePathNzb, "TemplatePathNzb").
			SetString(&indexerConfig.CategoryDownloader, "CategoryDownloader").
			SetString(&indexerConfig.AdditionalQueryParams, "AdditionalQueryParams").
			SetBool(&indexerConfig.SkipEmptySize, "SkipEmptySize").
			SetBool(&indexerConfig.HistoryCheckTitle, "HistoryCheckTitle").
			SetString(&indexerConfig.CategoriesIndexer, "CategoriesIndexer")

		configs = append(configs, indexerConfig)
	}

	return configs
}

// createQualityConfig creates a QualityConfig from form data.
func createQualityConfig(index string, c *gin.Context) (config.QualityConfig, error) {
	builder := NewConfigBuilder(c, "quality_main", index)

	var qualityConfig config.QualityConfig

	builder.SetStringRequired(&qualityConfig.Name, "Name")

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
		SetBool(&qualityConfig.UseForPriorityAudioFormat, "UseForPriorityAudioFormat").
		SetBool(&qualityConfig.UseForPriorityAudioBitrate, "UseForPriorityAudioBitrate").
		SetBool(&qualityConfig.UseForPriorityCodec, "UseForPriorityCodec").
		SetBool(&qualityConfig.UseForPriorityOther, "UseForPriorityOther").
		SetBool(&qualityConfig.PreferLossless, "PreferLossless").
		SetInt(&qualityConfig.MinAudioBitrate, "MinAudioBitrate").
		SetStringArrayFromForm(&qualityConfig.WantedAudioFormats, "WantedAudioFormats").
		SetInt(&qualityConfig.UseForPriorityMinDifference, "UseForPriorityMinDifference")

	// Parse nested configurations
	reorder, err := createQualityReorderConfigs(index, c)
	qualityConfig.QualityReorder = reorder
	qualityConfig.Indexer = createQualityIndexerConfigs(index, c)

	return qualityConfig, errors.Join(builder.Err(), err)
}

// saveQualityConfigs saves quality configurations.
func saveQualityConfigs(configs []config.QualityConfig) error {
	return saveConfig(configs)
}

// createSchedulerConfig creates a SchedulerConfig from form data.
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

	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshBooks"); val != "" {
		addConfig.IntervalFeedsRefreshBooks = val
	}

	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshBooksFull"); val != "" {
		addConfig.IntervalFeedsRefreshBooksFull = val
	}

	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshAudiobooks"); val != "" {
		addConfig.IntervalFeedsRefreshAudiobooks = val
	}

	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshAudiobooksFull"); val != "" {
		addConfig.IntervalFeedsRefreshAudiobooksFull = val
	}

	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshMusic"); val != "" {
		addConfig.IntervalFeedsRefreshMusic = val
	}

	if val := getFormField(c, prefix, index, "IntervalFeedsRefreshMusicFull"); val != "" {
		addConfig.IntervalFeedsRefreshMusicFull = val
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

	if val := getFormField(c, prefix, index, "CronFeedsRefreshBooks"); val != "" {
		addConfig.CronFeedsRefreshBooks = val
	}

	if val := getFormField(c, prefix, index, "CronFeedsRefreshBooksFull"); val != "" {
		addConfig.CronFeedsRefreshBooksFull = val
	}

	if val := getFormField(c, prefix, index, "CronFeedsRefreshAudiobooks"); val != "" {
		addConfig.CronFeedsRefreshAudiobooks = val
	}

	if val := getFormField(c, prefix, index, "CronFeedsRefreshAudiobooksFull"); val != "" {
		addConfig.CronFeedsRefreshAudiobooksFull = val
	}

	if val := getFormField(c, prefix, index, "CronFeedsRefreshMusic"); val != "" {
		addConfig.CronFeedsRefreshMusic = val
	}

	if val := getFormField(c, prefix, index, "CronFeedsRefreshMusicFull"); val != "" {
		addConfig.CronFeedsRefreshMusicFull = val
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

	// Artist/author RSS scheduling (music, books, audiobooks)
	if val := getFormField(c, prefix, index, "IntervalIndexerRssArtists"); val != "" {
		addConfig.IntervalIndexerRssArtists = val
	}

	if val := getFormField(c, prefix, index, "IntervalIndexerRssArtistsUpgrade"); val != "" {
		addConfig.IntervalIndexerRssArtistsUpgrade = val
	}

	if val := getFormField(c, prefix, index, "CronIndexerRssArtists"); val != "" {
		addConfig.CronIndexerRssArtists = val
	}

	if val := getFormField(c, prefix, index, "CronIndexerRssArtistsUpgrade"); val != "" {
		addConfig.CronIndexerRssArtistsUpgrade = val
	}

	if val := getFormField(c, prefix, index, "IntervalIndexerRssAuthors"); val != "" {
		addConfig.IntervalIndexerRssAuthors = val
	}

	if val := getFormField(c, prefix, index, "IntervalIndexerRssAuthorsUpgrade"); val != "" {
		addConfig.IntervalIndexerRssAuthorsUpgrade = val
	}

	if val := getFormField(c, prefix, index, "CronIndexerRssAuthors"); val != "" {
		addConfig.CronIndexerRssAuthors = val
	}

	if val := getFormField(c, prefix, index, "CronIndexerRssAuthorsUpgrade"); val != "" {
		addConfig.CronIndexerRssAuthorsUpgrade = val
	}

	// Cache refresh scheduling
	if val := getFormField(c, prefix, index, "IntervalCacheRefresh"); val != "" {
		addConfig.IntervalCacheRefresh = val
	}

	if val := getFormField(c, prefix, index, "IntervalDownloadCheck"); val != "" {
		addConfig.IntervalDownloadCheck = val
	}

	if val := getFormField(c, prefix, index, "CronDownloadCheck"); val != "" {
		addConfig.CronDownloadCheck = val
	}

	if val := getFormField(c, prefix, index, "CronCacheRefresh"); val != "" {
		addConfig.CronCacheRefresh = val
	}

	return addConfig
}

// saveSchedulerConfigs saves scheduler configurations.
func saveSchedulerConfigs(configs []config.SchedulerConfig) error {
	if err := saveConfig(configs); err != nil {
		return err
	}

	scheduler.Reload()

	return nil
}

// ================================================================================
// CONFIGBUILDER SYSTEM - OPTIMIZED CONFIGURATION PROCESSING
// ================================================================================

// ConfigBuilder provides a fluent interface for building config structs from form data.
// Unparseable non-empty numeric input is collected and reported by Err instead
// of silently saving 0.
type ConfigBuilder struct {
	context *gin.Context
	prefix  string
	index   string
	errs    []error
}

// NewConfigBuilder creates a new ConfigBuilder instance.
func NewConfigBuilder(c *gin.Context, prefix, index string) *ConfigBuilder {
	return &ConfigBuilder{
		context: c,
		prefix:  prefix,
		index:   index,
	}
}

// Err returns all input errors collected while building, or nil.
func (cb *ConfigBuilder) Err() error {
	return errors.Join(cb.errs...)
}

func (cb *ConfigBuilder) key(fieldName string) string {
	return formFieldKey(cb.prefix, cb.index, fieldName)
}

func (cb *ConfigBuilder) invalid(fieldName, value, want string) {
	cb.errs = append(cb.errs, fmt.Errorf("%s: invalid value %q (expected %s)", cb.key(fieldName), value, want))
}

// parseIntField returns the trimmed int value of a field. ok is false when
// the field is empty or invalid; invalid input is recorded as an error.
func (cb *ConfigBuilder) parseIntField(fieldName string, minV, maxV int, want string) (int, bool) {
	value := strings.TrimSpace(getFormField(cb.context, cb.prefix, cb.index, fieldName))
	if value == "" {
		return 0, false
	}

	n, err := strconv.Atoi(value)
	if err != nil || n < minV || n > maxV {
		cb.invalid(fieldName, value, want)
		return 0, false
	}

	return n, true
}

// SetString sets a string field to the posted value. A field that was posted
// empty clears the target; the target is kept only when the field is absent.
func (cb *ConfigBuilder) SetString(target *string, fieldName string) *ConfigBuilder {
	if value, ok := cb.context.GetPostForm(cb.key(fieldName)); ok {
		*target = value
	}

	return cb
}

// SetStringRequired sets a string field (always, even if empty).
func (cb *ConfigBuilder) SetStringRequired(target *string, fieldName string) *ConfigBuilder {
	*target = getFormField(cb.context, cb.prefix, cb.index, fieldName)
	return cb
}

// SetInt sets an int field. Empty input sets 0.
func (cb *ConfigBuilder) SetInt(target *int, fieldName string) *ConfigBuilder {
	if !formFieldPresent(cb.context, cb.key(fieldName)) {
		return cb
	}

	if n, ok := cb.parseIntField(fieldName, math.MinInt, math.MaxInt, "a whole number"); ok {
		*target = n
	} else if strings.TrimSpace(getFormField(cb.context, cb.prefix, cb.index, fieldName)) == "" {
		*target = 0
	}

	return cb
}

// SetBool sets a bool field from form checkbox/toggle values.
func (cb *ConfigBuilder) SetBool(target *bool, fieldName string) *ConfigBuilder {
	*target = getFormFieldBool(cb.context, cb.prefix, cb.index, fieldName)
	return cb
}

func (cb *ConfigBuilder) SetStringMultiSelectArray(
	target *[]string,
	fieldName string,
) *ConfigBuilder {
	*target = filterStringArray(getFormFieldArray(cb.context, cb.prefix, cb.index, fieldName))
	return cb
}

// SetStringFloatMap sets a map[string]float64 field from the parallel
// "<fieldName>_key" and "<fieldName>_value" form arrays produced by the
// "floatmap" widget. Rows with an empty key are skipped; a non-numeric value is
// reported as an error. The target is replaced whenever the widget was posted.
func (cb *ConfigBuilder) SetStringFloatMap(
	target *map[string]float64,
	fieldName string,
) *ConfigBuilder {
	if !formFieldPresent(cb.context, cb.key(fieldName+"_key")) &&
		!formFieldPresent(cb.context, cb.key(fieldName)) {
		return cb
	}

	keys := getFormFieldArray(cb.context, cb.prefix, cb.index, fieldName+"_key")
	values := getFormFieldArray(cb.context, cb.prefix, cb.index, fieldName+"_value")

	result := make(map[string]float64, len(keys))

	for i, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" || i >= len(values) {
			continue
		}

		v := strings.TrimSpace(values[i])

		f, err := strconv.ParseFloat(v, 64)
		if err != nil {
			cb.invalid(fieldName+"["+k+"]", v, "a number")
			continue
		}

		result[k] = f
	}

	*target = result

	return cb
}

// SetFloat32 sets a float32 field. Empty input sets 0.
func (cb *ConfigBuilder) SetFloat32(target *float32, fieldName string) *ConfigBuilder {
	var f float64
	if cb.setFloat(&f, fieldName, 32) {
		*target = float32(f)
	}

	return cb
}

// SetFloat64 sets a float64 field. Empty input sets 0.
func (cb *ConfigBuilder) SetFloat64(target *float64, fieldName string) *ConfigBuilder {
	cb.setFloat(target, fieldName, 64)
	return cb
}

func (cb *ConfigBuilder) setFloat(target *float64, fieldName string, bitSize int) bool {
	if !formFieldPresent(cb.context, cb.key(fieldName)) {
		return false
	}

	value := strings.TrimSpace(getFormField(cb.context, cb.prefix, cb.index, fieldName))
	if value == "" {
		*target = 0
		return true
	}

	f, err := strconv.ParseFloat(value, bitSize)
	if err != nil {
		cb.invalid(fieldName, value, "a number")
		return false
	}

	*target = f

	return true
}

// SetUint8 sets a uint8 field. Empty input sets 0.
func (cb *ConfigBuilder) SetUint8(target *uint8, fieldName string) *ConfigBuilder {
	if !formFieldPresent(cb.context, cb.key(fieldName)) {
		return cb
	}

	if n, ok := cb.parseIntField(fieldName, 0, math.MaxUint8, "a number from 0 to 255"); ok {
		*target = uint8(n)
	} else if strings.TrimSpace(getFormField(cb.context, cb.prefix, cb.index, fieldName)) == "" {
		*target = 0
	}

	return cb
}

// SetStringArrayFromForm sets a string array field from PostFormArray, dropping
// empty entries. An emptied list clears the target; it is kept only when the
// field is absent from the form.
func (cb *ConfigBuilder) SetStringArrayFromForm(target *[]string, fieldName string) *ConfigBuilder {
	if formFieldPresent(cb.context, cb.key(fieldName)) {
		*target = filterStringArray(getFormFieldArray(cb.context, cb.prefix, cb.index, fieldName))
	}

	return cb
}

// SetUint16 sets a uint16 field. Empty input sets 0.
func (cb *ConfigBuilder) SetUint16(target *uint16, fieldName string) *ConfigBuilder {
	if !formFieldPresent(cb.context, cb.key(fieldName)) {
		return cb
	}

	if n, ok := cb.parseIntField(fieldName, 0, math.MaxUint16, "a number from 0 to 65535"); ok {
		*target = uint16(n)
	} else if strings.TrimSpace(getFormField(cb.context, cb.prefix, cb.index, fieldName)) == "" {
		*target = 0
	}

	return cb
}

// SetUint sets a uint field. Empty input sets 0.
func (cb *ConfigBuilder) SetUint(target *uint, fieldName string) *ConfigBuilder {
	if !formFieldPresent(cb.context, cb.key(fieldName)) {
		return cb
	}

	if n, ok := cb.parseIntField(fieldName, 0, math.MaxInt, "a non-negative whole number"); ok {
		*target = uint(n)
	} else if strings.TrimSpace(getFormField(cb.context, cb.prefix, cb.index, fieldName)) == "" {
		*target = 0
	}

	return cb
}

// SetIntArray sets an int array field from all posted values of the field
// (the "arrayint" widget posts one input per entry); each value may also hold
// a comma-separated list.
func (cb *ConfigBuilder) SetIntArray(target *[]int, fieldName string) *ConfigBuilder {
	if !formFieldPresent(cb.context, cb.key(fieldName)) {
		return cb
	}

	var intArray []int

	for _, value := range getFormFieldArray(cb.context, cb.prefix, cb.index, fieldName) {
		for part := range strings.SplitSeq(value, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}

			intVal, err := strconv.Atoi(trimmed)
			if err != nil {
				cb.invalid(fieldName, trimmed, "a whole number")
				continue
			}

			intArray = append(intArray, intVal)
		}
	}

	*target = intArray

	return cb
}

func createImdbConfigFields(configv *config.ImdbConfig) []FormFieldDefinition {
	return []FormFieldDefinition{
		{
			Name:  "Indexedtypes",
			Type:  "selectarray",
			Value: configv.Indexedtypes,
			Options: []SelectOption{
				{Value: "movie", Label: "movie"},
				{Value: "tvMovie", Label: "tvMovie"},
				{Value: "tvmovie", Label: "tvmovie"},
				{Value: "tvSeries", Label: "tvSeries"},
				{Value: "tvseries", Label: "tvseries"},
				{Value: "video", Label: "video"},
			},
		},
		{Name: "Indexedlanguages", Type: "array", Value: configv.Indexedlanguages},
		{Name: "Indexfull", Type: "checkbox", Value: configv.Indexfull},
		{Name: "ImdbIDSize", Type: "number", Value: configv.ImdbIDSize},
		{Name: "UseMemory", Type: "checkbox", Value: configv.UseMemory},
	}
}

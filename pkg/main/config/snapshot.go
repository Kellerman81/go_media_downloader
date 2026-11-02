package config

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// ConfigurationError represents configuration-related errors.
type ConfigurationError struct {
	Type    string
	Message string
	Cause   error
}

func (e *ConfigurationError) Error() string {
	return fmt.Sprintf("config %s: %s", e.Type, e.Message)
}

func (e *ConfigurationError) Unwrap() error {
	return e.Cause
}

// ConfigSnapshot represents a complete, validated configuration snapshot.
// A published snapshot is never mutated by the config package: every change
// builds a new snapshot from a deep copy of the TOML data.
type ConfigSnapshot struct {
	// Immutable configuration maps
	General      *GeneralConfig
	Imdb         *ImdbConfig
	Path         map[string]*PathsConfig
	Quality      map[string]*QualityConfig
	List         map[string]*ListsConfig
	Indexer      map[string]*IndexersConfig
	Regex        map[string]*RegexConfig
	Media        map[string]*MediaTypeConfig
	Notification map[string]*NotificationConfig
	Downloader   map[string]*DownloaderConfig
	Scheduler    map[string]*SchedulerConfig
	cachetoml    MainConfig
	ValidatedAt  time.Time
	// issues are non-fatal problems found while building (see configIssue).
	issues []configIssue
}

var (
	// Atomic configuration storage for lock-free reads.
	configSnapshot atomic.Value // *ConfigSnapshot

	errGeneralConfigNil = errors.New("general configuration cannot be nil")
)

// getCurrentConfig returns the current configuration snapshot (thread-safe).
func getCurrentConfig() *ConfigSnapshot {
	if snapshot, ok := configSnapshot.Load().(*ConfigSnapshot); ok {
		return snapshot
	}

	return nil
}

// cloneMediaTypeConfigs copies a media slice together with the nested slices
// whose elements buildConfigSnapshot writes resolved pointers into.
func cloneMediaTypeConfigs(src []MediaTypeConfig) []MediaTypeConfig {
	dst := slices.Clone(src)
	for i := range dst {
		dst[i].Data = slices.Clone(dst[i].Data)
		dst[i].DataImport = slices.Clone(dst[i].DataImport)
		dst[i].Lists = slices.Clone(dst[i].Lists)
		dst[i].Notification = slices.Clone(dst[i].Notification)
	}

	return dst
}

// cloneMainConfig returns a copy of src that shares no slice backing arrays
// that the config package mutates, so changes to the copy can never be seen
// through a previously published snapshot.
func cloneMainConfig(src *MainConfig) *MainConfig {
	dst := *src
	dst.Downloader = slices.Clone(src.Downloader)
	dst.Indexers = slices.Clone(src.Indexers)
	dst.Lists = slices.Clone(src.Lists)
	dst.Notification = slices.Clone(src.Notification)
	dst.Paths = slices.Clone(src.Paths)
	dst.Regex = slices.Clone(src.Regex)
	dst.Scheduler = slices.Clone(src.Scheduler)

	dst.Quality = slices.Clone(src.Quality)
	for i := range dst.Quality {
		dst.Quality[i].Indexer = slices.Clone(dst.Quality[i].Indexer)
	}

	dst.Media.Movies = cloneMediaTypeConfigs(src.Media.Movies)
	dst.Media.Series = cloneMediaTypeConfigs(src.Media.Series)
	dst.Media.Books = cloneMediaTypeConfigs(src.Media.Books)
	dst.Media.AudioBooks = cloneMediaTypeConfigs(src.Media.AudioBooks)
	dst.Media.Music = cloneMediaTypeConfigs(src.Media.Music)

	return &dst
}

func validationError(format string, args ...any) error {
	return &ConfigurationError{
		Type:    "validation",
		Message: fmt.Sprintf(format, args...),
	}
}

// configIssue is a problem that does not prevent loading a config from disk
// (configs that loaded before these checks existed must keep booting), but
// that rejects a user-initiated change which introduces it. key identifies
// the issue independent of slice positions so issues present before and
// after a change can be matched up.
type configIssue struct {
	key string
	msg string
}

type issueCollector struct {
	issues []configIssue
	seen   map[string]struct{}
}

func (c *issueCollector) add(key, format string, args ...any) {
	if c.seen == nil {
		c.seen = make(map[string]struct{})
	}

	msg := fmt.Sprintf(format, args...)
	if _, ok := c.seen[key+"\x00"+msg]; ok {
		return
	}

	c.seen[key+"\x00"+msg] = struct{}{}
	c.issues = append(c.issues, configIssue{key: key, msg: msg})
}

// checkNames reports empty names and names that collide case-insensitively.
// Snapshot maps are keyed by name, so only the first of a duplicate set is
// indexed. emptyFatal keeps the empty-name check fatal for the sections where
// it already was before issues existed.
func checkNames[T any](
	c *issueCollector,
	section string,
	items []T,
	name func(*T) string,
	emptyFatal bool,
) error {
	seen := make(map[string]int, len(items))
	for i := range items {
		nm := name(&items[i])
		if nm == "" {
			if emptyFatal {
				return validationError("%s configuration %d missing name", section, i)
			}

			c.add("empty|"+section, "%s configuration %d missing name", section, i)

			continue
		}

		key := strings.ToLower(nm)
		if j, ok := seen[key]; ok {
			c.add("dup|"+section+"|"+key,
				"duplicate %s name '%s' (entries %d and %d; names are compared case-insensitively, only the first is used)",
				section, nm, j, i)

			continue
		}

		seen[key] = i
	}

	return nil
}

// validateMediaTypeConfigs: legacy marks movie/series, whose empty-name and
// missing-data checks were always fatal.
func validateMediaTypeConfigs(c *issueCollector, section string, items []MediaTypeConfig, legacy bool) error {
	if err := checkNames(c, section, items, func(m *MediaTypeConfig) string { return m.Name }, legacy); err != nil {
		return err
	}

	for i := range items {
		if len(items[i].Data) == 0 {
			if legacy {
				return validationError("%s configuration '%s' missing data paths", section, items[i].Name)
			}

			c.add("nodata|"+section+"|"+items[i].Name,
				"%s configuration '%s' missing data paths", section, items[i].Name)
		}

		_ = checkNames(c, section+" '"+items[i].Name+"' list", items[i].Lists,
			func(l *MediaListsConfig) string { return l.Name }, false)
	}

	return nil
}

// validateConfiguration validates config. A returned error is fatal; non-fatal
// issues are added to c.
func validateConfiguration(config *MainConfig, c *issueCollector) error {
	if config == nil {
		return validationError("configuration is nil")
	}

	// Auto-correct general configuration first
	if err := validateGeneralConfig(&config.General); err != nil {
		return err
	}

	// Then validate the corrected values
	if config.General.WorkerFiles < 1 || config.General.WorkerFiles > 100 {
		return validationError("invalid worker_files count, must be 1-100")
	}

	if config.General.WorkerParse < 1 || config.General.WorkerParse > 100 {
		return validationError("invalid worker_parse count, must be 1-100")
	}

	if config.General.WorkerSearch > 100 {
		c.add("worker_search", "invalid worker_search count %d, must be 1-100", config.General.WorkerSearch)
	}

	media := []struct {
		section string
		items   []MediaTypeConfig
		legacy  bool
	}{
		{"movie", config.Media.Movies, true},
		{"series", config.Media.Series, true},
		{"book", config.Media.Books, false},
		{"audiobook", config.Media.AudioBooks, false},
		{"music", config.Media.Music, false},
	}

	var (
		mediaCount                        int
		usesQuality, usesPath, usesIndexr bool
	)

	for _, m := range media {
		mediaCount += len(m.items)
		if err := validateMediaTypeConfigs(c, m.section, m.items, m.legacy); err != nil {
			return err
		}

		for i := range m.items {
			if m.items[i].TemplateQuality != "" {
				usesQuality = true
			}

			for j := range m.items[i].Lists {
				if m.items[i].Lists[j].TemplateQuality != "" {
					usesQuality = true
				}
			}

			for j := range m.items[i].Data {
				if m.items[i].Data[j].TemplatePath != "" {
					usesPath = true
				}
			}

			for j := range m.items[i].DataImport {
				if m.items[i].DataImport[j].TemplatePath != "" {
					usesPath = true
				}
			}
		}
	}

	if mediaCount == 0 {
		return validationError(
			"at least one media type (movies, series, books, audiobooks or music) must be configured",
		)
	}

	for i := range config.Quality {
		for j := range config.Quality[i].Indexer {
			if config.Quality[i].Indexer[j].TemplateIndexer != "" {
				usesIndexr = true
			}
		}
	}

	if usesIndexr && len(config.Indexers) == 0 {
		return validationError("at least one indexer must be configured")
	}

	if err := checkNames(c, "indexer", config.Indexers, func(c *IndexersConfig) string { return c.Name }, true); err != nil {
		return err
	}

	for idx := range config.Indexers {
		if config.Indexers[idx].URL == "" {
			return validationError("indexer '%s' missing URL", config.Indexers[idx].Name)
		}
	}

	if usesPath && len(config.Paths) == 0 {
		return validationError("at least one path configuration must be defined")
	}

	if err := checkNames(c, "path", config.Paths, func(c *PathsConfig) string { return c.Name }, true); err != nil {
		return err
	}

	if usesQuality && len(config.Quality) == 0 {
		return validationError("at least one quality profile must be configured")
	}

	if err := checkNames(c, "quality", config.Quality, func(c *QualityConfig) string { return c.Name }, true); err != nil {
		return err
	}

	_ = checkNames(c, "list", config.Lists, func(c *ListsConfig) string { return c.Name }, false)
	_ = checkNames(c, "downloader", config.Downloader, func(c *DownloaderConfig) string { return c.Name }, false)
	_ = checkNames(c, "regex", config.Regex, func(c *RegexConfig) string { return c.Name }, false)
	_ = checkNames(c, "scheduler", config.Scheduler, func(c *SchedulerConfig) string { return c.Name }, false)
	_ = checkNames(c, "notification", config.Notification, func(c *NotificationConfig) string { return c.Name }, false)

	return nil
}

// putFirst indexes v under key unless key is empty or already present
// (compared case-insensitively) - the first entry of a duplicate set wins.
func putFirst[T any](m map[string]*T, key string, v *T) bool {
	if key == "" {
		return false
	}

	if _, ok := m[key]; ok {
		return false
	}

	for k := range m {
		if strings.EqualFold(k, key) {
			return false
		}
	}

	m[key] = v

	return true
}

// logIssues logs every non-fatal issue of a snapshot as a warning.
func logIssues(issues []configIssue) {
	for _, is := range issues {
		logger.Logtype("warn", 0).Str("issue", is.msg).Msg("config problem (not fatal when loading)")
	}
}

// newIssues returns the issues in after that were not already present in
// before (multiset comparison by key), i.e. the ones a change introduced.
func newIssues(before, after []configIssue) (introduced, preexisting []configIssue) {
	count := make(map[string]int, len(before))
	for _, is := range before {
		count[is.key]++
	}

	for _, is := range after {
		if count[is.key] > 0 {
			count[is.key]--
			preexisting = append(preexisting, is)

			continue
		}

		introduced = append(introduced, is)
	}

	return introduced, preexisting
}

// refResolver resolves template references and records dangling ones as issues.
type refResolver struct {
	issues *issueCollector
}

// resolveRef looks a template up by exact name first and falls back to a
// case-insensitive match, mirroring the EqualFold fallbacks in the accessors.
func resolveRef[T any](r *refResolver, m map[string]*T, kind, name, owner string) *T {
	if name == "" {
		return nil
	}

	if v, ok := m[name]; ok {
		return v
	}

	for k, v := range m {
		if strings.EqualFold(k, name) {
			return v
		}
	}

	r.issues.add("ref|"+kind+"|"+strings.ToLower(name)+"|"+owner,
		"%s references %s '%s', which is not configured", owner, kind, name)

	return nil
}

// buildConfigSnapshot creates a new configuration snapshot from TOML data.
// src is deep-copied first; neither src nor any published snapshot is modified.
func buildConfigSnapshot(src *MainConfig) (*ConfigSnapshot, error) {
	if src == nil {
		return nil, &ConfigurationError{
			Type:    "build",
			Message: "TOML configuration is nil",
		}
	}

	tomlConfig := cloneMainConfig(src)

	issues := &issueCollector{}

	// Validate configuration before building snapshot
	if err := validateConfiguration(tomlConfig, issues); err != nil {
		return nil, err
	}

	refs := &refResolver{issues: issues}

	snapshot := &ConfigSnapshot{
		General:     &tomlConfig.General,
		Imdb:        &tomlConfig.Imdbindexer,
		cachetoml:   *tomlConfig,
		ValidatedAt: time.Now(),
	}

	// Initialize maps with proper capacity
	snapshot.Downloader = make(map[string]*DownloaderConfig, len(tomlConfig.Downloader))
	snapshot.Indexer = make(map[string]*IndexersConfig, len(tomlConfig.Indexers))
	snapshot.List = make(map[string]*ListsConfig, len(tomlConfig.Lists))
	snapshot.Media = make(map[string]*MediaTypeConfig)
	snapshot.Notification = make(map[string]*NotificationConfig, len(tomlConfig.Notification))
	snapshot.Path = make(map[string]*PathsConfig, len(tomlConfig.Paths))
	snapshot.Quality = make(map[string]*QualityConfig, len(tomlConfig.Quality))
	snapshot.Regex = make(map[string]*RegexConfig, len(tomlConfig.Regex))
	snapshot.Scheduler = make(map[string]*SchedulerConfig, len(tomlConfig.Scheduler))

	// Set defaults for general configuration
	if snapshot.General.CacheDuration == 0 {
		snapshot.General.CacheDuration = 12
	}

	snapshot.General.CacheDuration2 = 2 * snapshot.General.CacheDuration
	if len(snapshot.General.MovieMetaSourcePriority) == 0 {
		snapshot.General.MovieMetaSourcePriority = []string{"imdb", "tmdb", "omdb", "trakt"}
	}

	// Build configuration maps
	for idx := range tomlConfig.Downloader {
		cfg := &tomlConfig.Downloader[idx]

		putFirst(snapshot.Downloader, cfg.Name, cfg)
	}

	for idx := range tomlConfig.Indexers {
		cfg := &tomlConfig.Indexers[idx]

		cfg.MaxEntriesStr = logger.IntToString(cfg.MaxEntries)
		putFirst(snapshot.Indexer, cfg.Name, cfg)
	}

	for idx := range tomlConfig.Lists {
		cfg := &tomlConfig.Lists[idx]

		cfg.ExcludegenreLen = len(cfg.Excludegenre)
		cfg.IncludegenreLen = len(cfg.Includegenre)
		putFirst(snapshot.List, cfg.Name, cfg)

		// Debug logging for list registration
		logger.Logtype(logger.StatusInfo, 0).
			Str("list_name", cfg.Name).
			Str("list_type", cfg.ListType).
			Msg("Registered list configuration")
	}

	for idx := range tomlConfig.Notification {
		cfg := &tomlConfig.Notification[idx]

		putFirst(snapshot.Notification, cfg.Name, cfg)
	}

	for idx := range tomlConfig.Paths {
		cfg := &tomlConfig.Paths[idx]

		cfg.AllowedLanguagesLen = len(cfg.AllowedLanguages)
		cfg.AllowedOtherExtensionsLen = len(cfg.AllowedOtherExtensions)
		cfg.AllowedOtherExtensionsNoRenameLen = len(cfg.AllowedOtherExtensionsNoRename)
		cfg.AllowedVideoExtensionsLen = len(cfg.AllowedVideoExtensions)
		cfg.AllowedVideoExtensionsNoRenameLen = len(cfg.AllowedVideoExtensionsNoRename)
		cfg.AllowedAudioExtensionsLen = len(cfg.AllowedAudioExtensions)
		cfg.AllowedAudioExtensionsNoRenameLen = len(cfg.AllowedAudioExtensionsNoRename)
		cfg.AllowedBookExtensionsLen = len(cfg.AllowedBookExtensions)
		cfg.AllowedBookExtensionsNoRenameLen = len(cfg.AllowedBookExtensionsNoRename)
		cfg.BlockedLen = len(cfg.Blocked)
		cfg.DisallowedLen = len(cfg.Disallowed)
		cfg.MaxSizeByte = int64(cfg.MaxSize) * 1024 * 1024
		cfg.MinSizeByte = int64(cfg.MinSize) * 1024 * 1024
		cfg.MinVideoSizeByte = int64(cfg.MinVideoSize) * 1024 * 1024
		putFirst(snapshot.Path, cfg.Name, cfg)
	}

	// CRITICAL: Process regex configs BEFORE quality configs so quality configs can reference them
	for idx := range tomlConfig.Regex {
		cfg := &tomlConfig.Regex[idx]

		cfg.RejectedLen = len(cfg.Rejected)
		cfg.RequiredLen = len(cfg.Required)
		putFirst(snapshot.Regex, cfg.Name, cfg)
	}

	for idx := range tomlConfig.Quality {
		cfg := &tomlConfig.Quality[idx]
		owner := "quality_" + cfg.Name
		// Build indexer configuration references
		cfg.IndexerCfg = make([]*IndexersConfig, len(cfg.Indexer))
		for idx2 := range cfg.Indexer {
			ind := &cfg.Indexer[idx2]
			ind.CfgDownloader = resolveRef(refs, snapshot.Downloader, "downloader", ind.TemplateDownloader, owner)
			ind.CfgIndexer = resolveRef(refs, snapshot.Indexer, "indexer", ind.TemplateIndexer, owner)
			cfg.IndexerCfg[idx2] = ind.CfgIndexer
			ind.CfgPath = resolveRef(refs, snapshot.Path, "path", ind.TemplatePathNzb, owner)
			ind.CfgRegex = resolveRef(refs, snapshot.Regex, "regex", ind.TemplateRegex, owner)
		}

		cfg.IndexerLen = len(cfg.Indexer)
		cfg.QualityReorderLen = len(cfg.QualityReorder)
		cfg.TitleStripPrefixForSearchLen = len(cfg.TitleStripPrefixForSearch)
		cfg.TitleStripSuffixForSearchLen = len(cfg.TitleStripSuffixForSearch)
		cfg.WantedAudioLen = len(cfg.WantedAudio)
		cfg.WantedAudioFormatsLen = len(cfg.WantedAudioFormats)
		cfg.WantedCodecLen = len(cfg.WantedCodec)
		cfg.WantedQualityLen = len(cfg.WantedQuality)
		cfg.WantedResolutionLen = len(cfg.WantedResolution)
		putFirst(snapshot.Quality, cfg.Name, cfg)
	}

	for idx := range tomlConfig.Scheduler {
		cfg := &tomlConfig.Scheduler[idx]

		putFirst(snapshot.Scheduler, cfg.Name, cfg)
	}

	// Setup media configurations
	media := []struct {
		items  []MediaTypeConfig
		prefix string
		isType uint
	}{
		{tomlConfig.Media.Movies, "movie_", MediaTypeMovie},
		{tomlConfig.Media.Series, "serie_", MediaTypeSeries},
		{tomlConfig.Media.Books, "book_", MediaTypeBook},
		{tomlConfig.Media.AudioBooks, "audiobook_", MediaTypeAudiobook},
		{tomlConfig.Media.Music, "music_", MediaTypeMusic},
	}

	for _, m := range media {
		for idx := range m.items {
			cfg := &m.items[idx]
			setupMediaTypeConfig(cfg, m.prefix, m.isType, snapshot, refs)
			setupMediaConfigLists(cfg, snapshot, refs)

			if cfg.Name != "" {
				putFirst(snapshot.Media, m.prefix+cfg.Name, cfg)
			}
		}
	}

	carryOverJobs(snapshot)

	snapshot.issues = issues.issues

	return snapshot, nil
}

// carryOverJobs keeps the job function maps that utils.LoadGlobalSchedulerConfig
// and utils.LoadSchedulerConfig attach to the live snapshot. They are not part
// of the TOML, so without this every rebuild would drop them until the next
// scheduler reload.
func carryOverJobs(snapshot *ConfigSnapshot) {
	current := getCurrentConfig()
	if current == nil {
		return
	}

	if snapshot.General.Jobs == nil && current.General != nil {
		snapshot.General.Jobs = current.General.Jobs
	}

	for key, cfg := range snapshot.Media {
		if cfg.Jobs != nil {
			continue
		}

		if old := current.Media[key]; old != nil {
			cfg.Jobs = old.Jobs
		}
	}
}

// validateGeneralConfig validates general configuration settings.
func validateGeneralConfig(cfg *GeneralConfig) error {
	if cfg == nil {
		return errGeneralConfigNil
	}

	if cfg.LogLevel == "" {
		cfg.LogLevel = "info" // Set default
	}

	if cfg.WorkerSearch <= 0 {
		cfg.WorkerSearch = 1 // Set minimum
	}

	if cfg.WorkerFiles <= 0 {
		cfg.WorkerFiles = 1 // Set minimum
	}

	if cfg.WorkerParse <= 0 {
		cfg.WorkerParse = 1 // Set minimum
	}

	return nil
}

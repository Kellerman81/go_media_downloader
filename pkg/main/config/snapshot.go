package config

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/pelletier/go-toml/v2"
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
}

// Clone creates a deep copy of the configuration snapshot.
func (s *ConfigSnapshot) Clone() *ConfigSnapshot {
	if s == nil {
		return nil
	}

	clone := &ConfigSnapshot{
		General:     s.General,
		Imdb:        s.Imdb,
		cachetoml:   s.cachetoml,
		ValidatedAt: s.ValidatedAt,
	}

	// Deep copy maps
	clone.Path = make(map[string]*PathsConfig, len(s.Path))
	for k, v := range s.Path {
		clone.Path[k] = v
	}

	clone.Quality = make(map[string]*QualityConfig, len(s.Quality))
	for k, v := range s.Quality {
		clone.Quality[k] = v
	}

	clone.List = make(map[string]*ListsConfig, len(s.List))
	for k, v := range s.List {
		clone.List[k] = v
	}

	clone.Indexer = make(map[string]*IndexersConfig, len(s.Indexer))
	for k, v := range s.Indexer {
		clone.Indexer[k] = v
	}

	clone.Regex = make(map[string]*RegexConfig, len(s.Regex))
	for k, v := range s.Regex {
		clone.Regex[k] = v
	}

	clone.Media = make(map[string]*MediaTypeConfig, len(s.Media))
	for k, v := range s.Media {
		clone.Media[k] = v
	}

	clone.Notification = make(map[string]*NotificationConfig, len(s.Notification))
	for k, v := range s.Notification {
		clone.Notification[k] = v
	}

	clone.Downloader = make(map[string]*DownloaderConfig, len(s.Downloader))
	for k, v := range s.Downloader {
		clone.Downloader[k] = v
	}

	clone.Scheduler = make(map[string]*SchedulerConfig, len(s.Scheduler))
	for k, v := range s.Scheduler {
		clone.Scheduler[k] = v
	}

	return clone
}

var (
	// Atomic configuration storage for lock-free reads.
	configSnapshot atomic.Value // *ConfigSnapshot

	// Configuration reload mutex - only used during reload operations.
	reloadMutex sync.Mutex
)

// getCurrentConfig returns the current configuration snapshot (thread-safe).
func getCurrentConfig() *ConfigSnapshot {
	if snapshot, ok := configSnapshot.Load().(*ConfigSnapshot); ok {
		return snapshot
	}
	return nil
}

// validateConfiguration performs comprehensive validation of configuration data.
func validateConfiguration(config *MainConfig) error {
	if config == nil {
		return &ConfigurationError{
			Type:    "validation",
			Message: "configuration is nil",
		}
	}

	// Auto-correct general configuration first
	if err := validateGeneralConfig(&config.General); err != nil {
		return err
	}

	// Then validate the corrected values
	if config.General.WorkerFiles < 1 || config.General.WorkerFiles > 100 {
		return &ConfigurationError{
			Type:    "validation",
			Message: "invalid worker_files count, must be 1-100",
		}
	}

	if config.General.WorkerParse < 1 || config.General.WorkerParse > 100 {
		return &ConfigurationError{
			Type:    "validation",
			Message: "invalid worker_parse count, must be 1-100",
		}
	}

	// Validate that at least one media type is configured
	if len(config.Media.Movies) == 0 && len(config.Media.Series) == 0 {
		return &ConfigurationError{
			Type:    "validation",
			Message: "at least one media type (movies or series) must be configured",
		}
	}

	// Validate that required templates exist for each media type
	for idx, movie := range config.Media.Movies {
		if movie.Name == "" {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("movie configuration %d missing name", idx),
			}
		}

		if len(movie.Data) == 0 {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("movie configuration '%s' missing data paths", movie.Name),
			}
		}
	}

	for idx, series := range config.Media.Series {
		if series.Name == "" {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("series configuration %d missing name", idx),
			}
		}

		if len(series.Data) == 0 {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("series configuration '%s' missing data paths", series.Name),
			}
		}
	}

	// Validate indexer configurations
	if len(config.Indexers) == 0 {
		return &ConfigurationError{
			Type:    "validation",
			Message: "at least one indexer must be configured",
		}
	}

	for idx, indexer := range config.Indexers {
		if indexer.Name == "" {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("indexer configuration %d missing name", idx),
			}
		}

		if indexer.URL == "" {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("indexer '%s' missing URL", indexer.Name),
			}
		}
	}

	// Validate path configurations
	if len(config.Paths) == 0 {
		return &ConfigurationError{
			Type:    "validation",
			Message: "at least one path configuration must be defined",
		}
	}

	for idx, path := range config.Paths {
		if path.Name == "" {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("path configuration %d missing name", idx),
			}
		}
	}

	// Validate quality configurations
	if len(config.Quality) == 0 {
		return &ConfigurationError{
			Type:    "validation",
			Message: "at least one quality profile must be configured",
		}
	}

	for idx, quality := range config.Quality {
		if quality.Name == "" {
			return &ConfigurationError{
				Type:    "validation",
				Message: fmt.Sprintf("quality configuration %d missing name", idx),
			}
		}
	}

	return nil
}

// buildConfigSnapshot creates a new configuration snapshot from TOML data.
func buildConfigSnapshot(tomlConfig *MainConfig, reload bool) (*ConfigSnapshot, error) {
	if tomlConfig == nil {
		return nil, &ConfigurationError{
			Type:    "build",
			Message: "TOML configuration is nil",
		}
	}

	// Validate configuration before building snapshot
	if err := validateConfiguration(tomlConfig); err != nil {
		return nil, err
	}

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

		snapshot.Downloader[cfg.Name] = cfg
	}

	for idx := range tomlConfig.Indexers {
		cfg := &tomlConfig.Indexers[idx]

		cfg.MaxEntriesStr = logger.IntToString(cfg.MaxEntries)
		snapshot.Indexer[cfg.Name] = cfg
	}

	for idx := range tomlConfig.Lists {
		cfg := &tomlConfig.Lists[idx]

		cfg.ExcludegenreLen = len(cfg.Excludegenre)
		cfg.IncludegenreLen = len(cfg.Includegenre)
		snapshot.List[cfg.Name] = cfg

		// Debug logging for list registration
		logger.Logtype(logger.StatusInfo, 0).
			Str("list_name", cfg.Name).
			Str("list_type", cfg.ListType).
			Msg("Registered list configuration")
	}

	for idx := range tomlConfig.Notification {
		cfg := &tomlConfig.Notification[idx]

		snapshot.Notification[cfg.Name] = cfg
	}

	for idx := range tomlConfig.Paths {
		cfg := &tomlConfig.Paths[idx]

		cfg.AllowedLanguagesLen = len(cfg.AllowedLanguages)
		cfg.AllowedOtherExtensionsLen = len(cfg.AllowedOtherExtensions)
		cfg.AllowedOtherExtensionsNoRenameLen = len(cfg.AllowedOtherExtensionsNoRename)
		cfg.AllowedVideoExtensionsLen = len(cfg.AllowedVideoExtensions)
		cfg.AllowedVideoExtensionsNoRenameLen = len(cfg.AllowedVideoExtensionsNoRename)
		cfg.BlockedLen = len(cfg.Blocked)
		cfg.DisallowedLen = len(cfg.Disallowed)
		cfg.MaxSizeByte = int64(cfg.MaxSize) * 1024 * 1024
		cfg.MinSizeByte = int64(cfg.MinSize) * 1024 * 1024
		cfg.MinVideoSizeByte = int64(cfg.MinVideoSize) * 1024 * 1024
		snapshot.Path[cfg.Name] = cfg
	}

	// CRITICAL: Process regex configs BEFORE quality configs so quality configs can reference them
	for idx := range tomlConfig.Regex {
		cfg := &tomlConfig.Regex[idx]

		cfg.RejectedLen = len(cfg.Rejected)
		cfg.RequiredLen = len(cfg.Required)
		snapshot.Regex[cfg.Name] = cfg
	}

	for idx := range tomlConfig.Quality {
		cfg := &tomlConfig.Quality[idx]
		// Build indexer configuration references
		cfg.IndexerCfg = make([]*IndexersConfig, len(cfg.Indexer))
		for idx2 := range cfg.Indexer {
			cfg.Indexer[idx2].CfgDownloader = snapshot.Downloader[cfg.Indexer[idx2].TemplateDownloader]
			cfg.Indexer[idx2].CfgIndexer = snapshot.Indexer[cfg.Indexer[idx2].TemplateIndexer]
			cfg.IndexerCfg[idx2] = snapshot.Indexer[cfg.Indexer[idx2].TemplateIndexer]
			cfg.Indexer[idx2].CfgPath = snapshot.Path[cfg.Indexer[idx2].TemplatePathNzb]
			cfg.Indexer[idx2].CfgRegex = snapshot.Regex[cfg.Indexer[idx2].TemplateRegex]
		}

		cfg.IndexerLen = len(cfg.Indexer)
		cfg.QualityReorderLen = len(cfg.QualityReorder)
		cfg.TitleStripPrefixForSearchLen = len(cfg.TitleStripPrefixForSearch)
		cfg.TitleStripSuffixForSearchLen = len(cfg.TitleStripSuffixForSearch)
		cfg.WantedAudioLen = len(cfg.WantedAudio)
		cfg.WantedCodecLen = len(cfg.WantedCodec)
		cfg.WantedQualityLen = len(cfg.WantedQuality)
		cfg.WantedResolutionLen = len(cfg.WantedResolution)
		snapshot.Quality[cfg.Name] = cfg
	}

	if !reload {
		for idx := range tomlConfig.Scheduler {
			cfg := &tomlConfig.Scheduler[idx]

			snapshot.Scheduler[cfg.Name] = cfg
		}
	} else {
		// Preserve existing scheduler configurations during reload
		if current := getCurrentConfig(); current != nil {
			for k, v := range current.Scheduler {
				snapshot.Scheduler[k] = v
			}
		}
	}

	// Setup media configurations
	for idx := range tomlConfig.Media.Movies {
		cfg := &tomlConfig.Media.Movies[idx]
		setupMediaTypeConfig(cfg, "movie_", false, snapshot)
		setupMediaConfigLists(cfg, snapshot)

		snapshot.Media["movie_"+cfg.Name] = cfg
	}

	for idx := range tomlConfig.Media.Series {
		cfg := &tomlConfig.Media.Series[idx]
		setupMediaTypeConfig(cfg, "serie_", true, snapshot)
		setupMediaConfigLists(cfg, snapshot)

		snapshot.Media["serie_"+cfg.Name] = cfg
	}

	return snapshot, nil
}

// SafeLoadAllSettings loads configuration with comprehensive validation and atomic swapping.
func SafeLoadAllSettings(reload bool) error {
	// Use separate reload mutex to prevent blocking reads during reload
	reloadMutex.Lock()
	defer reloadMutex.Unlock()

	logger.Logtype(logger.StatusInfo, 1).
		Bool("reload", reload).
		Msg("Starting safe configuration load")

	// Step 1: Read and parse TOML file
	tomlConfig, err := readConfigTomlSafe()
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Failed to read configuration file")

		return &ConfigurationError{
			Type:    "read",
			Message: "failed to read configuration file",
			Cause:   err,
		}
	}

	// Step 2: Build and validate new configuration snapshot
	newSnapshot, err := buildConfigSnapshot(tomlConfig, reload)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Failed to build configuration snapshot")
		return err
	}

	configSnapshot.Store(newSnapshot)

	logger.Logtype(logger.StatusInfo, 1).
		Time("validated_at", newSnapshot.ValidatedAt).
		Int("media_configs", len(newSnapshot.Media)).
		Int("indexers", len(newSnapshot.Indexer)).
		Int("quality_profiles", len(newSnapshot.Quality)).
		Msg("Configuration loaded successfully")

	return nil
}

// readConfigTomlSafe reads and parses the TOML configuration file safely.
func readConfigTomlSafe() (*MainConfig, error) {
	content, err := os.Open(Configfile)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file '%s': %w", Configfile, err)
	}
	defer content.Close()

	decoder := toml.NewDecoder(content)

	var config MainConfig

	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode TOML config: %w", err)
	}

	return &config, nil
}

// validateGeneralConfig validates general configuration settings.
func validateGeneralConfig(cfg *GeneralConfig) error {
	if cfg == nil {
		return errors.New("general configuration cannot be nil")
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

	return nil
}

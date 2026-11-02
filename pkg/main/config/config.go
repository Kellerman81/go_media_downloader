package config

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/pelletier/go-toml/v2"
)

const (
	MediaTypeMovie uint = iota
	MediaTypeSeries
	MediaTypeBook
	MediaTypeAudiobook
	MediaTypeMusic
	MediaTypeCustom uint = 999
)

var (
	Configfile       = "./config/config.toml"
	RandomizerSource = rand.NewSource(time.Now().UnixNano())
	mu               = sync.RWMutex{}
)

// GetConfigDir returns the directory path where configuration files are stored.
func GetConfigDir() string {
	return "./config"
}

// Slepping sleeps for a random or fixed number of seconds. If random is true,
// it will sleep for a random number of seconds between 1 and seconds. If random
// is false, it will sleep for the specified number of seconds. It uses the
// rand and time packages to generate the random sleep duration and sleep.
func Slepping(random bool, seconds int) {
	if random {
		n := rand.New(RandomizerSource).Intn(seconds) + 1 // n will be between 0 and 10
		time.Sleep(time.Duration(n) * time.Second)
	} else {
		time.Sleep(time.Duration(seconds) * time.Second)
	}
}

// LoadCfgDB loads the application configuration from a database file.
// It opens the configuration file, decodes the TOML data into a cache,
// and then populates various global settings maps with the configuration
// data. This allows the configuration system to be extended by adding
// new config types.
func LoadCfgDB(reload bool) error {
	if _, err := os.Stat(Configfile); errors.Is(err, os.ErrNotExist) {
		fmt.Println("Config file not found. Creating a new default config file.") //nolint:forbidigo // logger not initialized yet
		ClearCfg()
		WriteCfg()

		port, apikey := "9090", "mysecure"
		if general := GetSettingsGeneral(); general != nil {
			if general.WebPort != "" {
				port = general.WebPort
			}

			if general.WebAPIKey != "" {
				apikey = general.WebAPIKey
			}
		}

		fmt.Println("Starting with the default configuration - nothing is configured yet.") //nolint:forbidigo // logger not initialized yet
		fmt.Printf(                                                                         //nolint:forbidigo // logger not initialized yet
			"Open http://localhost:%s/api/admin in your browser (user 'admin', password '%s') and run the Setup Wizard to finish configuring the app.\n",
			port, apikey,
		)
	} else {
		fmt.Println("Config file found. Loading config.") //nolint:forbidigo // logger not initialized yet
	}

	// Load all settings first (this acquires and releases the config lock)
	return Loadallsettings(reload)
}

// Loadallsettings loads all configuration settings from the TOML configuration file
// and atomically publishes the resulting snapshot. It holds the config mutex so
// file reloads, UI updates and resets never interleave their builds/stores.
// The reload flag is kept for callers; every section, schedulers included, is
// always rebuilt from the file.
func Loadallsettings(_ bool) error {
	mu.Lock()
	defer mu.Unlock()

	defaultConfig, err := Readconfigtoml()
	if err != nil {
		fmt.Println(err) //nolint:forbidigo // logger not initialized yet during config loading
		return err
	}

	snapshot, err := buildConfigSnapshot(defaultConfig)
	if err != nil {
		fmt.Println(err) //nolint:forbidigo // logger not initialized yet
		return err
	}

	// At startup the logger is not initialized yet and would drop these.
	for _, is := range snapshot.issues {
		fmt.Println("Config warning:", is.msg) //nolint:forbidigo // logger may not be initialized yet
	}

	logIssues(snapshot.issues)
	configSnapshot.Store(snapshot)

	return nil
}

// Readconfigtoml reads and decodes the configuration file specified by Configfile into the global settings.cachetoml struct.
// It opens the configuration file, uses a TOML decoder to parse its contents, and handles any potential errors.
// Returns an error if the file cannot be opened or decoded, otherwise returns nil.
func Readconfigtoml() (*MainConfig, error) {
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

// setupMediaTypeConfig initializes common configuration for a media type config.
func setupMediaTypeConfig(
	mediaConfig *MediaTypeConfig,
	prefix string,
	isMediaType uint,
	snapshot *ConfigSnapshot,
	refs *refResolver,
) {
	owner := prefix + mediaConfig.Name

	// Initialize maps
	mediaConfig.DataMap = make(map[int]*MediaDataConfig, len(mediaConfig.Data))
	mediaConfig.DataImportMap = make(map[int]*MediaDataImportConfig, len(mediaConfig.DataImport))

	// Setup Data configs
	for idx2 := range mediaConfig.Data {
		data := &mediaConfig.Data[idx2]
		data.CfgPath = resolveRef(refs, snapshot.Path, "path", data.TemplatePath, owner)
		mediaConfig.DataMap[idx2] = data
	}

	// Setup DataImport configs
	for idx2 := range mediaConfig.DataImport {
		data := &mediaConfig.DataImport[idx2]
		data.CfgPath = resolveRef(refs, snapshot.Path, "path", data.TemplatePath, owner)
		mediaConfig.DataImportMap[idx2] = data
	}

	// Setup Notification configs
	for idx2 := range mediaConfig.Notification {
		n := &mediaConfig.Notification[idx2]
		n.CfgNotification = resolveRef(refs, snapshot.Notification, "notification", n.MapNotification, owner)
	}

	// Setup main configs
	mediaConfig.CfgQuality = resolveRef(refs, snapshot.Quality, "quality", mediaConfig.TemplateQuality, owner)
	mediaConfig.CfgScheduler = resolveRef(refs, snapshot.Scheduler, "scheduler", mediaConfig.TemplateScheduler, owner)
	mediaConfig.NamePrefix = owner
	mediaConfig.IsType = isMediaType

	// Setup Lists maps and related fields
	mediaConfig.ListsMap = make(map[string]*MediaListsConfig, len(mediaConfig.Lists))

	mediaConfig.ListsMapIdx = make(map[string]int, len(mediaConfig.Lists))
	mediaConfig.ListsQu = ""
	if len(mediaConfig.Lists) >= 1 {
		mediaConfig.ListsQu = strings.Repeat(",?", len(mediaConfig.Lists)-1)
	}

	mediaConfig.ListsLen = len(mediaConfig.Lists)
	mediaConfig.MetadataTitleLanguagesLen = len(mediaConfig.MetadataTitleLanguages)
	mediaConfig.DataLen = len(mediaConfig.Data)
	mediaConfig.ListsQualities = make([]string, 0, len(mediaConfig.Lists))

	// Pre-compute list names so handlers don't allocate []string on every lookup call.
	mediaConfig.ListsNames = make([]string, len(mediaConfig.Lists))
	for i := range mediaConfig.Lists {
		mediaConfig.ListsNames[i] = mediaConfig.Lists[i].Name
	}
}

// setupMediaListConfig initializes configuration for a media list.
func setupMediaListConfig(
	listConfig *MediaListsConfig,
	mediaConfig *MediaTypeConfig,
	idxsub int,
	snapshot *ConfigSnapshot,
	refs *refResolver,
) {
	owner := mediaConfig.NamePrefix + " list " + listConfig.Name
	listConfig.CfgList = resolveRef(refs, snapshot.List, "list", listConfig.TemplateList, owner)
	listConfig.CfgQuality = resolveRef(refs, snapshot.Quality, "quality", listConfig.TemplateQuality, owner)
	listConfig.CfgScheduler = resolveRef(refs, snapshot.Scheduler, "scheduler", listConfig.TemplateScheduler, owner)

	listConfig.IgnoreMapListsQu = ""
	if len(listConfig.IgnoreMapLists) >= 1 {
		listConfig.IgnoreMapListsQu = strings.Repeat(",?", len(listConfig.IgnoreMapLists)-1)
	}

	listConfig.IgnoreMapListsLen = len(listConfig.IgnoreMapLists)
	listConfig.ReplaceMapListsLen = len(listConfig.ReplaceMapLists)

	// Unnamed and duplicate (case-insensitive) lists stay in Lists so the file
	// round-trips, but are not indexed by name.
	if !putFirst(mediaConfig.ListsMap, listConfig.Name, listConfig) {
		return
	}

	mediaConfig.ListsMapIdx[listConfig.Name] = idxsub

	if !slices.Contains(mediaConfig.ListsQualities, listConfig.TemplateQuality) {
		mediaConfig.ListsQualities = append(mediaConfig.ListsQualities, listConfig.TemplateQuality)
	}
}

// setupMediaConfigLists processes all lists for a media configuration.
func setupMediaConfigLists(mediaConfig *MediaTypeConfig, snapshot *ConfigSnapshot, refs *refResolver) {
	for idxsub := range mediaConfig.Lists {
		setupMediaListConfig(&mediaConfig.Lists[idxsub], mediaConfig, idxsub, snapshot, refs)
	}
}

// populateConfigsInMap adds config entries to a map with prefix for GetCfgAll.
func populateConfigsInMap(configMap map[string]any,
	snapshot *ConfigSnapshot,
) {
	// Media configs use NamePrefix
	for key := range snapshot.Media {
		configMap[snapshot.Media[key].NamePrefix] = *snapshot.Media[key]
	}

	// All other configs use standard prefixes
	for key := range snapshot.Downloader {
		configMap["downloader_"+key] = *snapshot.Downloader[key]
	}

	for key := range snapshot.Indexer {
		configMap["indexer_"+key] = *snapshot.Indexer[key]
	}

	for key := range snapshot.List {
		configMap["list_"+key] = *snapshot.List[key]
	}

	for key := range snapshot.Notification {
		configMap["notification_"+key] = *snapshot.Notification[key]
	}

	for key := range snapshot.Path {
		configMap["path_"+key] = *snapshot.Path[key]
	}

	for key := range snapshot.Quality {
		configMap["quality_"+key] = *snapshot.Quality[key]
	}

	for key := range snapshot.Regex {
		configMap["regex_"+key] = *snapshot.Regex[key]
	}

	for key := range snapshot.Scheduler {
		configMap["scheduler_"+key] = *snapshot.Scheduler[key]
	}
}

// namesWithEmpty returns "" followed by the entry names in config-file order.
func namesWithEmpty[T any](items []T, name func(*T) string) []string {
	options := make([]string, 0, len(items)+1)
	options = append(options, "")
	for i := range items {
		options = append(options, name(&items[i]))
	}

	return options
}

// getTemplateOptionsForType returns template options for a specific config
// type, in config-file order so dropdowns are stable.
func getTemplateOptionsForType(configType string) []string {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	t := &currentSnapshot.cachetoml
	switch configType {
	case "downloader":
		return namesWithEmpty(t.Downloader, func(c *DownloaderConfig) string { return c.Name })
	case "indexer":
		return namesWithEmpty(t.Indexers, func(c *IndexersConfig) string { return c.Name })
	case "list":
		return namesWithEmpty(t.Lists, func(c *ListsConfig) string { return c.Name })
	case "notification":
		return namesWithEmpty(t.Notification, func(c *NotificationConfig) string { return c.Name })
	case "path":
		return namesWithEmpty(t.Paths, func(c *PathsConfig) string { return c.Name })
	case "quality":
		return namesWithEmpty(t.Quality, func(c *QualityConfig) string { return c.Name })
	case "regex":
		return namesWithEmpty(t.Regex, func(c *RegexConfig) string { return c.Name })
	case "scheduler":
		return namesWithEmpty(t.Scheduler, func(c *SchedulerConfig) string { return c.Name })
	default:
		return nil
	}
}

// GetCfgAll returns a map containing all the application configuration settings.
// It collects the settings from the various global config variables and organizes
// them into a single map indexed by config section name prefixes.
func GetCfgAll() map[string]any {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	q := make(map[string]any)

	q["general"] = currentSnapshot.General
	q["imdb"] = currentSnapshot.Imdb
	populateConfigsInMap(q, currentSnapshot)

	return q
}

// GetSettingTemplatesFor returns template options for configuration forms.
// It generates a map containing available configuration names for a given type
// (downloader, indexer, list, notification, path, quality, regex, scheduler).
// Used to populate dropdown menus and form options in the web interface.
func GetSettingTemplatesFor(key string) map[string][]string {
	options := getTemplateOptionsForType(key)
	if options == nil {
		return nil
	}

	return map[string][]string{"options": options}
}

func GetCfgAllJson() map[string]any {
	return GetCfgAll()
}

// confData extracts the typed payload of a Conf, accepting a value or a pointer.
func confData[T any](val Conf) (T, error) {
	switch d := val.Data.(type) {
	case T:
		return d, nil
	case *T:
		if d != nil {
			return *d, nil
		}
	}

	var zero T

	return zero, fmt.Errorf("config entry %q: expected %T, got %T", val.Name, zero, val.Data)
}

func setEntry[T any](dst *T, val Conf) error {
	data, err := confData[T](val)
	if err != nil {
		return err
	}

	*dst = data

	return nil
}

// upsertEntry replaces the entry with the same (case-insensitive) name or
// appends it. Callers operate on a cloned MainConfig, so in-place assignment
// never touches a published snapshot.
func upsertEntry[T any](s *[]T, val Conf, name func(*T) string) error {
	data, err := confData[T](val)
	if err != nil {
		return err
	}

	for i := range *s {
		if strings.EqualFold(name(&(*s)[i]), name(&data)) {
			(*s)[i] = data
			return nil
		}
	}

	*s = append(*s, data)

	return nil
}

// removeEntry builds a new slice without the named entry.
func removeEntry[T any](s *[]T, entry string, name func(*T) string) error {
	for i := range *s {
		if !strings.EqualFold(name(&(*s)[i]), entry) {
			continue
		}

		out := make([]T, 0, len(*s)-1)
		out = append(out, (*s)[:i]...)
		*s = append(out, (*s)[i+1:]...)

		return nil
	}

	return fmt.Errorf("config entry %q: %w", entry, logger.ErrNotFound)
}

func mediaName(c *MediaTypeConfig) string { return c.Name }

// mediaSliceFor maps a media entry key (e.g. "movie_EN") to its slice and prefix.
func mediaSliceFor(t *MainConfig, name string) (*[]MediaTypeConfig, string) {
	switch {
	case strings.HasPrefix(name, logger.StrSerie):
		return &t.Media.Series, "serie_"
	case strings.HasPrefix(name, logger.StrMovie):
		return &t.Media.Movies, "movie_"
	case strings.HasPrefix(name, "book_"):
		return &t.Media.Books, "book_"
	case strings.HasPrefix(name, "audiobook_"):
		return &t.Media.AudioBooks, "audiobook_"
	case strings.HasPrefix(name, "music_"):
		return &t.Media.Music, "music_"
	}

	return nil, ""
}

// updateTomlEntry updates a specific entry in the MainConfig TOML structure.
func updateTomlEntry(t *MainConfig, val Conf) error {
	switch {
	case strings.HasPrefix(val.Name, "general"):
		return setEntry(&t.General, val)
	case strings.HasPrefix(val.Name, "downloader_"):
		return upsertEntry(&t.Downloader, val, func(c *DownloaderConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, logger.StrImdb):
		return setEntry(&t.Imdbindexer, val)
	case strings.HasPrefix(val.Name, "indexer"):
		return upsertEntry(&t.Indexers, val, func(c *IndexersConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, "list"):
		return upsertEntry(&t.Lists, val, func(c *ListsConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, "notification"):
		return upsertEntry(&t.Notification, val, func(c *NotificationConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, "path"):
		return upsertEntry(&t.Paths, val, func(c *PathsConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, "quality"):
		return upsertEntry(&t.Quality, val, func(c *QualityConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, "regex"):
		return upsertEntry(&t.Regex, val, func(c *RegexConfig) string { return c.Name })
	case strings.HasPrefix(val.Name, "scheduler"):
		return upsertEntry(&t.Scheduler, val, func(c *SchedulerConfig) string { return c.Name })
	}

	if media, _ := mediaSliceFor(t, val.Name); media != nil {
		return upsertEntry(media, val, mediaName)
	}

	return fmt.Errorf("unknown config entry type %q", val.Name)
}

// deleteTomlEntry removes a specific entry from the MainConfig TOML structure.
func deleteTomlEntry(t *MainConfig, name string) error {
	switch {
	case strings.HasPrefix(name, "general"):
		t.General = GeneralConfig{}
		return nil
	case strings.HasPrefix(name, "downloader_"):
		return removeEntry(&t.Downloader, strings.TrimPrefix(name, "downloader_"),
			func(c *DownloaderConfig) string { return c.Name })
	case strings.HasPrefix(name, logger.StrImdb):
		t.Imdbindexer = ImdbConfig{}
		return nil
	case strings.HasPrefix(name, "indexer"):
		return removeEntry(&t.Indexers, strings.TrimPrefix(name, "indexer_"),
			func(c *IndexersConfig) string { return c.Name })
	case strings.HasPrefix(name, "list"):
		return removeEntry(&t.Lists, strings.TrimPrefix(name, "list_"),
			func(c *ListsConfig) string { return c.Name })
	case strings.HasPrefix(name, "notification"):
		return removeEntry(&t.Notification, strings.TrimPrefix(name, "notification_"),
			func(c *NotificationConfig) string { return c.Name })
	case strings.HasPrefix(name, "path"):
		return removeEntry(&t.Paths, strings.TrimPrefix(name, "path_"),
			func(c *PathsConfig) string { return c.Name })
	case strings.HasPrefix(name, "quality"):
		return removeEntry(&t.Quality, strings.TrimPrefix(name, "quality_"),
			func(c *QualityConfig) string { return c.Name })
	case strings.HasPrefix(name, "regex"):
		return removeEntry(&t.Regex, strings.TrimPrefix(name, "regex_"),
			func(c *RegexConfig) string { return c.Name })
	case strings.HasPrefix(name, "scheduler"):
		return removeEntry(&t.Scheduler, strings.TrimPrefix(name, "scheduler_"),
			func(c *SchedulerConfig) string { return c.Name })
	}

	if media, prefix := mediaSliceFor(t, name); media != nil {
		return removeEntry(media, strings.TrimPrefix(name, prefix), mediaName)
	}

	return fmt.Errorf("unknown config entry type %q", name)
}

// applyConfigChange runs mutate on a deep copy of the current TOML, validates
// it by building a snapshot, and only then writes the file and publishes the
// snapshot. An invalid change leaves both the file and the live config as-is.
func applyConfigChange(op, entry string, mutate func(*MainConfig) error) error {
	mu.Lock()
	defer mu.Unlock()

	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return &ConfigurationError{
			Type:    op,
			Message: "no configuration snapshot available",
		}
	}

	candidate := cloneMainConfig(&currentSnapshot.cachetoml)
	if err := mutate(candidate); err != nil {
		return &ConfigurationError{Type: op, Message: err.Error(), Cause: err}
	}

	cnt, err := toml.Marshal(candidate)
	if err != nil {
		logger.Logtype(logger.StatusError, 2).Err(err).Msg("Error marshaling config")
		return err
	}

	newSnapshot, err := buildConfigSnapshot(candidate)
	if err != nil {
		logger.Logtype(logger.StatusError, 2).Err(err).Str("entry", entry).Msg("Rejected invalid configuration change")
		return err
	}

	// Only problems this change introduces block it; problems already in the
	// file elsewhere must not make every save fail.
	introduced, preexisting := newIssues(currentSnapshot.issues, newSnapshot.issues)
	logIssues(preexisting)

	if len(introduced) > 0 {
		msgs := make([]string, len(introduced))
		for i := range introduced {
			msgs[i] = introduced[i].msg
		}

		err := &ConfigurationError{Type: "validation", Message: strings.Join(msgs, "; ")}
		logger.Logtype(logger.StatusError, 2).Err(err).Str("entry", entry).Msg("Rejected invalid configuration change")

		return err
	}

	if err := writeConfigFile(cnt); err != nil {
		logger.Logtype(logger.StatusError, 2).Err(err).Msg("Error writing config file")
		return err
	}

	configSnapshot.Store(newSnapshot)

	logger.Logtype(logger.StatusInfo, 2).
		Str("entry", entry).
		Str("operation", op).
		Msg("Configuration changed, snapshot rebuilt, and written to file")

	return nil
}

// writeConfigFile replaces Configfile atomically (temp file + rename in the
// same directory) so the fsnotify watcher never reads a half-written file.
// The temp name deliberately does not contain "config.toml", which the
// watcher matches on.
func writeConfigFile(cnt []byte) error {
	// Renaming over a symlink would replace the link itself.
	target := Configfile
	if resolved, err := filepath.EvalSymlinks(Configfile); err == nil {
		target = resolved
	}

	tmp, err := os.CreateTemp(filepath.Dir(target), ".cfgwrite-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp config file: %w", err)
	}

	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op after a successful rename

	if _, err := tmp.Write(cnt); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp config file: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("sync temp config file: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp config file: %w", err)
	}

	if err := os.Chmod(tmpName, 0o600); err != nil {
		return fmt.Errorf("chmod temp config file: %w", err)
	}

	if err := os.Rename(tmpName, target); err != nil {
		// A file bind-mounted on its own (docker -v file:file) cannot be
		// replaced by rename (EBUSY); fall back to rewriting it in place.
		logger.Logtype("warn", 0).Err(err).Str(logger.StrFile, target).
			Msg("Atomic config replace failed, rewriting in place")

		if werr := os.WriteFile(target, cnt, 0o600); werr != nil {
			return fmt.Errorf("replace config file: %w", errors.Join(err, werr))
		}
	}

	return nil
}

// UpdateCfgEntry updates (or adds) one configuration entry, validates the
// result, writes it to the config file and publishes the new snapshot.
func UpdateCfgEntry(configIn Conf) error {
	return applyConfigChange("update", configIn.Name, func(t *MainConfig) error {
		return updateTomlEntry(t, configIn)
	})
}

// UpdateCfgEntryAny replaces a whole config section with configIn.
func UpdateCfgEntryAny(configIn any) error {
	return applyConfigChange("update", fmt.Sprintf("%T", configIn), func(t *MainConfig) error {
		switch data := configIn.(type) {
		case GeneralConfig:
			t.General = data
		case *GeneralConfig:
			t.General = *data
		case []DownloaderConfig:
			t.Downloader = data
		case ImdbConfig:
			t.Imdbindexer = data
		case *ImdbConfig:
			t.Imdbindexer = *data
		case []IndexersConfig:
			t.Indexers = data
		case []ListsConfig:
			t.Lists = data
		case MediaConfig:
			t.Media = data
		case *MediaConfig:
			t.Media = *data
		case []NotificationConfig:
			t.Notification = data
		case []PathsConfig:
			t.Paths = data
		case []QualityConfig:
			t.Quality = data
		case []RegexConfig:
			t.Regex = data
		case []SchedulerConfig:
			t.Scheduler = data
		default:
			return fmt.Errorf("unsupported config section type %T", configIn)
		}

		return nil
	})
}

// DeleteCfgEntry deletes the configuration entry with the given name (e.g.
// "quality_HD", "music_Main"), validates the result, writes the config file
// and publishes the new snapshot.
func DeleteCfgEntry(name string) error {
	return applyConfigChange("delete", name, func(t *MainConfig) error {
		return deleteTomlEntry(t, name)
	})
}

// GetToml returns a copy of the current main configuration. The copy shares no
// slice backing arrays with the live snapshot, so callers may modify it freely.
func GetToml() MainConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return MainConfig{}
	}

	return *cloneMainConfig(&currentSnapshot.cachetoml)
}

// ClearCfg clears all configuration settings by deleting the config database file,
// resetting all config maps to empty maps, and reinitializing default settings.
// It wipes the existing config and starts fresh with defaults.
func ClearCfg() {
	// Same lock-ordering reasoning as UpdateCfgEntry/DeleteCfgEntry: this
	// unconditionally replaces the entire snapshot with hardcoded defaults,
	// so racing it against a concurrent UpdateCfgEntry/DeleteCfgEntry could
	// have either one silently clobber the other depending on timing.
	mu.Lock()
	defer mu.Unlock()

	defaultConfig := &MainConfig{
		General: GeneralConfig{
			LogLevel:       "Info",
			DBLogLevel:     "Info",
			LogFileCount:   5,
			LogFileSize:    5,
			LogCompress:    false,
			WebAPIKey:      "mysecure",
			WebPort:        "9090",
			WorkerMetadata: 1,
			WorkerFiles:    1,
			WorkerParse:    1,
			WorkerSearch:   1,
			WorkerIndexer:  1,
			// ConcurrentScheduler: 1,
			OmdbLimiterSeconds:       1,
			OmdbLimiterCalls:         1,
			TmdbLimiterSeconds:       1,
			TmdbLimiterCalls:         1,
			TraktLimiterSeconds:      1,
			TraktLimiterCalls:        1,
			TvdbLimiterSeconds:       1,
			TvdbLimiterCalls:         1,
			TvmazeLimiterSeconds:     1,
			TvmazeLimiterCalls:       1,
			PlexLimiterSeconds:       1,
			PlexLimiterCalls:         10,
			PlexTimeoutSeconds:       30,
			PlexDisableTLSVerify:     false,
			JellyfinLimiterSeconds:   1,
			JellyfinLimiterCalls:     10,
			JellyfinTimeoutSeconds:   30,
			JellyfinDisableTLSVerify: false,
			SchedulerDisabled:        true,
		},
		Scheduler: []SchedulerConfig{{
			Name:                       "Default",
			IntervalImdb:               "3d",
			IntervalFeeds:              "1d",
			IntervalFeedsRefreshSeries: "1d",
			IntervalFeedsRefreshMovies: "1d",
			IntervalIndexerMissing:     "40m",
			IntervalIndexerUpgrade:     "60m",
			IntervalIndexerRss:         "15m",
			IntervalScanData:           "1h",
			IntervalScanDataMissing:    "1d",
			IntervalScanDataimport:     "60m",
		}},
		Downloader: []DownloaderConfig{{
			Name:   "initial",
			DlType: "drone",
		}},
		Imdbindexer: ImdbConfig{
			Indexedtypes:     []string{logger.StrMovie},
			Indexedlanguages: []string{"US", "UK", "\\N"},
		},
		Indexers: []IndexersConfig{{
			Name:           "initial",
			IndexerType:    "newznab",
			URL:            "https://your-indexer-url.example.com",
			Limitercalls:   5,
			Limiterseconds: 20,
			MaxEntries:     100,
			RssEntriesloop: 2,
		}},
		Lists: []ListsConfig{{
			Name:     "initial",
			ListType: "traktmovieanticipated",
			Limit:    "20",
		}},
		Media: MediaConfig{
			Movies: []MediaTypeConfig{{
				Name:              "initial",
				TemplateQuality:   "initial",
				TemplateScheduler: "Default",
				Data:              []MediaDataConfig{{TemplatePath: "initial"}},
				DataImport:        []MediaDataImportConfig{{TemplatePath: "initial"}},
				Lists: []MediaListsConfig{{
					Name:              "initial",
					TemplateList:      "initial",
					TemplateQuality:   "initial",
					TemplateScheduler: "Default",
				}},
				Notification: []MediaNotificationConfig{{MapNotification: "initial"}},
			}},
			Series: []MediaTypeConfig{{
				Name:              "initial",
				TemplateQuality:   "initial",
				TemplateScheduler: "Default",
				Data:              []MediaDataConfig{{TemplatePath: "initial"}},
				DataImport:        []MediaDataImportConfig{{TemplatePath: "initial"}},
				Lists: []MediaListsConfig{{
					Name:              "initial",
					TemplateList:      "initial",
					TemplateQuality:   "initial",
					TemplateScheduler: "Default",
				}},
				Notification: []MediaNotificationConfig{{MapNotification: "initial"}},
			}},
		},
		Notification: []NotificationConfig{{
			Name:             "initial",
			NotificationType: "csv",
		}},
		Paths: []PathsConfig{{
			Name:                   "initial",
			AllowedVideoExtensions: []string{".avi", ".mkv", ".mp4"},
			AllowedOtherExtensions: []string{".idx", ".sub", ".srt"},
		}},
		Quality: []QualityConfig{{
			Name:           "initial",
			QualityReorder: []QualityReorderConfig{{}},
			Indexer: []QualityIndexerConfig{{
				TemplateIndexer:    "initial",
				TemplateDownloader: "initial",
				TemplateRegex:      "initial",
				TemplatePathNzb:    "initial",
			}},
		}},
		Regex: []RegexConfig{{
			Name: "initial",
		}},
	}

	// Build snapshot from default config
	snapshot, err := buildConfigSnapshot(defaultConfig)
	if err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Msg("Failed to build default config snapshot")
		return
	}

	logIssues(snapshot.issues)
	configSnapshot.Store(snapshot)

	logger.Logtype(logger.StatusInfo, 0).
		Msg("Configuration reset to defaults")
}

// WriteCfg writes the current configuration to Configfile. It marshals the
// snapshot's TOML data as loaded/updated - every section, all media types,
// in config-file order - and replaces the file atomically.
func WriteCfg() error {
	mu.Lock()
	defer mu.Unlock()

	settings := getCurrentConfig()
	if settings == nil {
		return &ConfigurationError{Type: "write", Message: "no configuration snapshot available"}
	}

	cnt, err := toml.Marshal(&settings.cachetoml)
	if err != nil {
		logger.Logtype("error", 1).Err(err).Msg("Error marshaling config")
		return err
	}

	if err := writeConfigFile(cnt); err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, Configfile).
			Err(err).
			Msg("Failed to write config file")

		return err
	}

	return nil
}

// BackupConfig copies the current config.toml to ./backup/ with a timestamp
// suffix, before some write is about to replace it - a manual, opt-in safety
// net separate from the app's regular database backups. Returns the backup
// file path.
func BackupConfig() (string, error) {
	if err := os.MkdirAll("./backup", 0o755); err != nil {
		return "", err
	}

	src, err := os.ReadFile(Configfile)
	if err != nil {
		return "", err
	}

	dest := "./backup/config.toml." + time.Now().Format("20060102_150405")

	if err := os.WriteFile(dest, src, 0o600); err != nil {
		return "", err
	}

	return dest, nil
}

// CheckGroup checks if a setting key exists in the given settings group.
// It takes a group name string and a key string as parameters.
// It returns a boolean indicating if the key exists in the given group.
func CheckGroup(prefix, name string) bool {
	if name == "" {
		return false
	}

	fullName := prefix + name

	snapshot := getCurrentConfig()
	if snapshot == nil {
		return false
	}

	// Check based on prefix type
	switch {
	case strings.HasPrefix(prefix, "quality_"):
		_, exists := snapshot.Quality[name]
		return exists

	case strings.HasPrefix(prefix, "path_"):
		_, exists := snapshot.Path[name]
		return exists

	case strings.HasPrefix(prefix, "media_"):
		_, exists := snapshot.Media[fullName]
		return exists

	case strings.HasPrefix(prefix, "list_"):
		_, exists := snapshot.List[name]
		return exists

	case strings.HasPrefix(prefix, "indexer_"):
		_, exists := snapshot.Indexer[name]
		return exists

	case strings.HasPrefix(prefix, "regex_"):
		_, exists := snapshot.Regex[name]
		return exists

	case strings.HasPrefix(prefix, "notification_"):
		_, exists := snapshot.Notification[name]
		return exists

	case strings.HasPrefix(prefix, "downloader_"):
		_, exists := snapshot.Downloader[name]
		return exists

	case strings.HasPrefix(prefix, "scheduler_"):
		_, exists := snapshot.Scheduler[name]
		return exists

	default:
		return false
	}
}

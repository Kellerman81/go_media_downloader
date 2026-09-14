package config

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
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

// Loadallsettings loads all configuration settings from the TOML configuration file.
// It performs a thread-safe reload of configuration by reading the TOML file,
// clearing existing settings, and repopulating them with fresh data.
// The reload parameter controls whether scheduler settings are preserved during reload.
func Loadallsettings(reload bool) error {
	defaultConfig, err := Readconfigtoml()
	if err != nil {
		fmt.Println(err) //nolint:forbidigo // logger not initialized yet during config loading
		return err
	}

	// Build snapshot from default config
	snapshot, err := buildConfigSnapshot(defaultConfig, reload)
	if err != nil {
		fmt.Println(err) //nolint:forbidigo // logger not initialized yet
		return err
	}

	// Store the default snapshot
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
) {
	// Initialize maps
	mediaConfig.DataMap = make(map[int]*MediaDataConfig, len(mediaConfig.Data))
	mediaConfig.DataImportMap = make(map[int]*MediaDataImportConfig, len(mediaConfig.DataImport))

	// Setup Data configs
	for idx2 := range mediaConfig.Data {
		mediaConfig.Data[idx2].CfgPath = snapshot.Path[mediaConfig.Data[idx2].TemplatePath]
		if isMediaType == 0 && mediaConfig.Data[idx2].AddFoundList != "" {
			mediaConfig.Data[idx2].AddFoundListCfg = snapshot.List[mediaConfig.Data[idx2].AddFoundList]
		}

		mediaConfig.DataMap[idx2] = &mediaConfig.Data[idx2]
	}

	// Setup DataImport configs
	for idx2 := range mediaConfig.DataImport {
		mediaConfig.DataImport[idx2].CfgPath = snapshot.Path[mediaConfig.DataImport[idx2].TemplatePath]
		mediaConfig.DataImportMap[idx2] = &mediaConfig.DataImport[idx2]
	}

	// Setup Notification configs
	for idx2 := range mediaConfig.Notification {
		mediaConfig.Notification[idx2].CfgNotification = snapshot.Notification[mediaConfig.Notification[idx2].MapNotification]
	}

	// Setup main configs
	mediaConfig.CfgQuality = snapshot.Quality[mediaConfig.TemplateQuality]
	mediaConfig.CfgScheduler = snapshot.Scheduler[mediaConfig.TemplateScheduler]
	mediaConfig.NamePrefix = prefix + mediaConfig.Name
	mediaConfig.IsType = isMediaType

	// Setup Lists maps and related fields
	mediaConfig.ListsMap = make(map[string]*MediaListsConfig, len(mediaConfig.Lists))

	mediaConfig.ListsMapIdx = make(map[string]int, len(mediaConfig.Lists))
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
) {
	listConfig.CfgList = snapshot.List[listConfig.TemplateList]
	listConfig.CfgQuality = snapshot.Quality[listConfig.TemplateQuality]
	listConfig.CfgScheduler = snapshot.Scheduler[listConfig.TemplateScheduler]

	if len(listConfig.IgnoreMapLists) >= 1 {
		listConfig.IgnoreMapListsQu = strings.Repeat(",?", len(listConfig.IgnoreMapLists)-1)
	}

	listConfig.IgnoreMapListsLen = len(listConfig.IgnoreMapLists)
	listConfig.ReplaceMapListsLen = len(listConfig.ReplaceMapLists)

	// Add quality to ListsQualities if not already present
	if !slices.Contains(mediaConfig.ListsQualities, listConfig.TemplateQuality) {
		mediaConfig.ListsQualities = append(mediaConfig.ListsQualities, listConfig.TemplateQuality)
	}

	// Setup maps
	mediaConfig.ListsMap[listConfig.Name] = listConfig
	mediaConfig.ListsMapIdx[listConfig.Name] = idxsub
}

// setupMediaConfigLists processes all lists for a media configuration.
func setupMediaConfigLists(mediaConfig *MediaTypeConfig, snapshot *ConfigSnapshot) {
	for idxsub := range mediaConfig.Lists {
		setupMediaListConfig(&mediaConfig.Lists[idxsub], mediaConfig, idxsub, snapshot)
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

// getTemplateOptionsForType returns template options for a specific config type.
func getTemplateOptionsForType(configType string) []string {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	var options []string
	switch configType {
	case "downloader":
		options = make([]string, 0, len(currentSnapshot.Downloader)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Downloader {
			options = append(options, cfg.Name)
		}

	case "indexer":
		options = make([]string, 0, len(currentSnapshot.Indexer)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Indexer {
			options = append(options, cfg.Name)
		}

	case "list":
		options = make([]string, 0, len(currentSnapshot.List)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.List {
			options = append(options, cfg.Name)
		}

	case "notification":
		options = make([]string, 0, len(currentSnapshot.Notification)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Notification {
			options = append(options, cfg.Name)
		}

	case "path":
		options = make([]string, 0, len(currentSnapshot.Path)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Path {
			options = append(options, cfg.Name)
		}

	case "quality":
		options = make([]string, 0, len(currentSnapshot.Quality)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Quality {
			options = append(options, cfg.Name)
		}

	case "regex":
		options = make([]string, 0, len(currentSnapshot.Regex)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Regex {
			options = append(options, cfg.Name)
		}

	case "scheduler":
		options = make([]string, 0, len(currentSnapshot.Scheduler)+1)

		options = append(options, "")
		for _, cfg := range currentSnapshot.Scheduler {
			options = append(options, cfg.Name)
		}

	default:
		return nil
	}

	return options
}

// UpdateCfg updates the application configuration settings based on the
// provided Conf struct. It iterates through the configIn slice, checks
// the prefix of the Name field to determine the config type, casts the
// Data field to the appropriate config struct, and saves the config
// values to the respective global settings maps. This allows the config
// system to be extended by adding new config types.
func UpdateCfg(configIn []Conf) {
	for _, val := range configIn {
		UpdateCfgEntry(val)
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

// updateTomlEntry updates a specific entry in the MainConfig TOML structure.
func updateTomlEntry(toml *MainConfig, val Conf) {
	switch {
	case strings.HasPrefix(val.Name, "general"):
		if data, ok := val.Data.(GeneralConfig); ok {
			toml.General = data
		}

	case strings.HasPrefix(val.Name, "downloader_"):
		data, ok := val.Data.(DownloaderConfig)
		if !ok {
			break
		}

		// Find and update the downloader in the slice
		found := false
		for i := range toml.Downloader {
			if toml.Downloader[i].Name != data.Name {
				continue
			}

			toml.Downloader[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Downloader = append(toml.Downloader, data)
		}

	case strings.HasPrefix(val.Name, logger.StrImdb):
		if data, ok := val.Data.(ImdbConfig); ok {
			toml.Imdbindexer = data
		}

	case strings.HasPrefix(val.Name, "indexer"):
		data, ok := val.Data.(IndexersConfig)
		if !ok {
			break
		}

		// Find and update the indexer in the slice
		found := false
		for i := range toml.Indexers {
			if toml.Indexers[i].Name != data.Name {
				continue
			}

			toml.Indexers[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Indexers = append(toml.Indexers, data)
		}

	case strings.HasPrefix(val.Name, "list"):
		data, ok := val.Data.(ListsConfig)
		if !ok {
			break
		}

		// Find and update the list in the slice
		found := false
		for i := range toml.Lists {
			if toml.Lists[i].Name != data.Name {
				continue
			}

			toml.Lists[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Lists = append(toml.Lists, data)
		}

	case strings.HasPrefix(val.Name, logger.StrSerie):
		data, ok := val.Data.(MediaTypeConfig)
		if !ok {
			break
		}

		// Find and update the series in the slice
		found := false
		for i := range toml.Media.Series {
			if toml.Media.Series[i].Name != data.Name {
				continue
			}

			toml.Media.Series[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Media.Series = append(toml.Media.Series, data)
		}

	case strings.HasPrefix(val.Name, logger.StrMovie):
		data, ok := val.Data.(MediaTypeConfig)
		if !ok {
			break
		}

		// Find and update the movie in the slice
		found := false
		for i := range toml.Media.Movies {
			if toml.Media.Movies[i].Name != data.Name {
				continue
			}

			toml.Media.Movies[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Media.Movies = append(toml.Media.Movies, data)
		}

	case strings.HasPrefix(val.Name, "notification"):
		data, ok := val.Data.(NotificationConfig)
		if !ok {
			break
		}

		// Find and update the notification in the slice
		found := false
		for i := range toml.Notification {
			if toml.Notification[i].Name != data.Name {
				continue
			}

			toml.Notification[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Notification = append(toml.Notification, data)
		}

	case strings.HasPrefix(val.Name, "path"):
		data, ok := val.Data.(PathsConfig)
		if !ok {
			break
		}

		// Find and update the path in the slice
		found := false
		for i := range toml.Paths {
			if toml.Paths[i].Name != data.Name {
				continue
			}

			toml.Paths[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Paths = append(toml.Paths, data)
		}

	case strings.HasPrefix(val.Name, "quality"):
		data, ok := val.Data.(QualityConfig)
		if !ok {
			break
		}

		// Find and update the quality in the slice
		found := false
		for i := range toml.Quality {
			if toml.Quality[i].Name != data.Name {
				continue
			}

			toml.Quality[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Quality = append(toml.Quality, data)
		}

	case strings.HasPrefix(val.Name, "regex"):
		data, ok := val.Data.(RegexConfig)
		if !ok {
			break
		}

		// Find and update the regex in the slice
		found := false
		for i := range toml.Regex {
			if toml.Regex[i].Name != data.Name {
				continue
			}

			toml.Regex[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Regex = append(toml.Regex, data)
		}

	case strings.HasPrefix(val.Name, "scheduler"):
		data, ok := val.Data.(SchedulerConfig)
		if !ok {
			break
		}

		// Find and update the scheduler in the slice
		found := false
		for i := range toml.Scheduler {
			if toml.Scheduler[i].Name != data.Name {
				continue
			}

			toml.Scheduler[i] = data
			found = true

			break
		}

		// If not found, append it
		if !found {
			toml.Scheduler = append(toml.Scheduler, data)
		}
	}
}

// UpdateCfgEntry updates the application configuration settings
// based on the provided Conf struct. It saves the config values to
// the config database.
func UpdateCfgEntry(configIn Conf) error {
	// Acquire config lock BEFORE opening database to maintain consistent lock ordering
	mu.Lock()
	defer mu.Unlock()

	// Get current snapshot
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return &ConfigurationError{
			Type:    "update",
			Message: "no configuration snapshot available",
		}
	}

	// Create a copy of the TOML config to modify
	updatedToml := currentSnapshot.cachetoml

	// Update the specific entry in the TOML structure
	updateTomlEntry(&updatedToml, configIn)

	// Marshal to TOML and write to file
	cnt, err := toml.Marshal(&updatedToml)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error marshaling config")
		return err
	}

	err = os.WriteFile(Configfile, cnt, 0o600)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error writing config file")
		return err
	}

	// Rebuild the snapshot from the updated TOML to fix all pointer links
	newSnapshot, err := buildConfigSnapshot(&updatedToml, true)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error rebuilding config snapshot")
		return err
	}

	// Update the global snapshot atomically
	configSnapshot.Store(newSnapshot)

	logger.Logtype(logger.StatusInfo, 0).
		Str("entry", configIn.Name).
		Msg("Configuration entry updated, snapshot rebuilt, and written to file")

	return nil
}

func UpdateCfgEntryAny(configIn any) error {
	mu.Lock()
	defer mu.Unlock()

	// Get current snapshot
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return &ConfigurationError{
			Type:    "update",
			Message: "no configuration snapshot available",
		}
	}

	// Create a copy of the TOML config to modify
	updatedToml := currentSnapshot.cachetoml

	// Update the appropriate field based on type
	switch data := configIn.(type) {
	case GeneralConfig:
		updatedToml.General = data
	case *GeneralConfig:
		updatedToml.General = *data
	case []DownloaderConfig:
		updatedToml.Downloader = data
	case ImdbConfig:
		updatedToml.Imdbindexer = data
	case *ImdbConfig:
		updatedToml.Imdbindexer = *data
	case []IndexersConfig:
		updatedToml.Indexers = data
	case []ListsConfig:
		updatedToml.Lists = data
	case MediaConfig:
		updatedToml.Media = data
	case *MediaConfig:
		updatedToml.Media = *data
	case []NotificationConfig:
		updatedToml.Notification = data
	case []PathsConfig:
		updatedToml.Paths = data
	case []QualityConfig:
		updatedToml.Quality = data
	case []RegexConfig:
		updatedToml.Regex = data
	case []SchedulerConfig:
		updatedToml.Scheduler = data
	}

	// Marshal to TOML and write to file
	cnt, err := toml.Marshal(&updatedToml)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error marshaling config")
		return err
	}

	err = os.WriteFile(Configfile, cnt, 0o600)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error writing config file")
		return err
	}

	// Rebuild the snapshot from the updated TOML to fix all pointer links
	newSnapshot, err := buildConfigSnapshot(&updatedToml, true)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error rebuilding config snapshot")
		return err
	}

	// Update the global snapshot atomically
	configSnapshot.Store(newSnapshot)

	// Update legacy global settings for backward compatibility (we already hold the lock)
	// updateLegacySettingsNoLock(newSnapshot)

	logger.Logtype(logger.StatusInfo, 0).
		Msg("Configuration updated, snapshot rebuilt, and written to file")

	return nil
}

// deleteTomlEntry removes a specific entry from the MainConfig TOML structure.
func deleteTomlEntry(toml *MainConfig, name string) {
	switch {
	case strings.HasPrefix(name, "general"):
		toml.General = GeneralConfig{}

	case strings.HasPrefix(name, "downloader_"):
		// Extract the actual name from the prefix
		actualName := strings.TrimPrefix(name, "downloader_")
		// Find and remove from slice
		for i := range toml.Downloader {
			if toml.Downloader[i].Name == actualName {
				toml.Downloader = append(toml.Downloader[:i], toml.Downloader[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, logger.StrImdb):
		toml.Imdbindexer = ImdbConfig{}

	case strings.HasPrefix(name, "indexer"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "indexer_")
		for i := range toml.Indexers {
			if toml.Indexers[i].Name == actualName {
				toml.Indexers = append(toml.Indexers[:i], toml.Indexers[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, "list"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "list_")
		for i := range toml.Lists {
			if toml.Lists[i].Name == actualName {
				toml.Lists = append(toml.Lists[:i], toml.Lists[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, logger.StrSerie):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "serie_")
		for i := range toml.Media.Series {
			if toml.Media.Series[i].Name == actualName {
				toml.Media.Series = append(toml.Media.Series[:i], toml.Media.Series[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, logger.StrMovie):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "movie_")
		for i := range toml.Media.Movies {
			if toml.Media.Movies[i].Name == actualName {
				toml.Media.Movies = append(toml.Media.Movies[:i], toml.Media.Movies[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, "notification"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "notification_")
		for i := range toml.Notification {
			if toml.Notification[i].Name == actualName {
				toml.Notification = append(toml.Notification[:i], toml.Notification[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, "path"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "path_")
		for i := range toml.Paths {
			if toml.Paths[i].Name == actualName {
				toml.Paths = append(toml.Paths[:i], toml.Paths[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, "quality"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "quality_")
		for i := range toml.Quality {
			if toml.Quality[i].Name == actualName {
				toml.Quality = append(toml.Quality[:i], toml.Quality[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, "regex"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "regex_")
		for i := range toml.Regex {
			if toml.Regex[i].Name == actualName {
				toml.Regex = append(toml.Regex[:i], toml.Regex[i+1:]...)
				break
			}
		}

	case strings.HasPrefix(name, "scheduler"):
		// Extract the actual name
		actualName := strings.TrimPrefix(name, "scheduler_")
		for i := range toml.Scheduler {
			if toml.Scheduler[i].Name == actualName {
				toml.Scheduler = append(toml.Scheduler[:i], toml.Scheduler[i+1:]...)
				break
			}
		}
	}
}

// DeleteCfgEntry deletes the configuration entry with the given name from
// the config database and clears the corresponding value in the in-memory
// config maps. It handles entries for all major configuration categories like
// general, downloader, indexer etc.
func DeleteCfgEntry(name string) error {
	// Acquire config lock BEFORE opening database to maintain consistent lock
	// ordering - UpdateCfgEntry/UpdateCfgEntryAny already do this; without it,
	// a concurrent update and delete (e.g. two requests to the external
	// config API) can race, each reading the same pre-change snapshot and
	// then overwriting the other's write to Configfile/configSnapshot with
	// its own, silently discarding whichever change lost the race.
	mu.Lock()
	defer mu.Unlock()

	// Get current snapshot
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return &ConfigurationError{
			Type:    "delete",
			Message: "no configuration snapshot available",
		}
	}

	// Create a copy of the TOML config to modify
	updatedToml := currentSnapshot.cachetoml

	// Delete the specific entry from the TOML structure
	deleteTomlEntry(&updatedToml, name)

	// Marshal to TOML and write to file
	cnt, err := toml.Marshal(&updatedToml)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error marshaling config")
		return err
	}

	err = os.WriteFile(Configfile, cnt, 0o600)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error writing config file")
		return err
	}

	// Rebuild the snapshot from the updated TOML to fix all pointer links
	newSnapshot, err := buildConfigSnapshot(&updatedToml, true)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Error rebuilding config snapshot")
		return err
	}

	// Update the global snapshot atomically
	configSnapshot.Store(newSnapshot)

	logger.Logtype(logger.StatusInfo, 0).
		Str("entry", name).
		Msg("Configuration entry deleted, snapshot rebuilt, and written to file")

	return nil
}

// GetToml returns the cached main configuration settings as a mainConfig struct.
// This function provides read-only access to the current configuration state.
func GetToml() MainConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return MainConfig{}
	}

	return currentSnapshot.cachetoml
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
	snapshot, err := buildConfigSnapshot(defaultConfig, false)
	if err != nil {
		logger.Logtype(logger.StatusError, 0).
			Err(err).
			Msg("Failed to build default config snapshot")
		return
	}

	// Store the default snapshot
	configSnapshot.Store(snapshot)

	logger.Logtype(logger.StatusInfo, 0).
		Msg("Configuration reset to defaults")
}

// WriteCfg marshals the application configuration structs into a TOML
// configuration file. It gathers all the global configuration structs,
// assembles them into a MainConfig struct, marshals to TOML and writes
// to the Configfile location.
func WriteCfg() {
	bla := GetMainConfig()

	cnt, err := toml.Marshal(bla)
	if err != nil {
		logger.Logtype("error", 1).Err(err).Msg("Error loading config")
	}

	if err := os.WriteFile(Configfile, cnt, 0o600); err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, Configfile).
			Err(err).
			Msg("Failed to write config file")
	}
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

// GetMainConfig assembles and returns a complete MainConfig structure.
// It collects all configuration settings from global maps and structures
// them into a single MainConfig that can be marshaled to TOML format.
func GetMainConfig() MainConfig {
	var bla MainConfig

	settings := getCurrentConfig()
	if settings == nil {
		return MainConfig{}
	}

	bla.General = *settings.General

	bla.Imdbindexer = *settings.Imdb
	for _, cfgdata := range settings.Downloader {
		bla.Downloader = append(bla.Downloader, *cfgdata)
	}

	for _, cfgdata := range settings.Indexer {
		bla.Indexers = append(bla.Indexers, *cfgdata)
	}

	for _, cfgdata := range settings.List {
		bla.Lists = append(bla.Lists, *cfgdata)
	}

	for _, cfgdata := range settings.cachetoml.Media.Series {
		if !strings.HasPrefix(cfgdata.NamePrefix, logger.StrSerie) {
			continue
		}

		bla.Media.Series = append(bla.Media.Series, cfgdata)
	}

	for _, cfgdata := range settings.cachetoml.Media.Movies {
		if !strings.HasPrefix(cfgdata.NamePrefix, logger.StrMovie) {
			continue
		}

		bla.Media.Movies = append(bla.Media.Movies, cfgdata)
	}

	for _, cfgdata := range settings.Notification {
		bla.Notification = append(bla.Notification, *cfgdata)
	}

	for _, cfgdata := range settings.Path {
		bla.Paths = append(bla.Paths, *cfgdata)
	}

	for _, cfgdata := range settings.Quality {
		bla.Quality = append(bla.Quality, *cfgdata)
	}

	for _, cfgdata := range settings.Regex {
		bla.Regex = append(
			bla.Regex,
			RegexConfig{Name: cfgdata.Name, Required: cfgdata.Required, Rejected: cfgdata.Rejected},
		)
	}

	for _, cfgdata := range settings.Scheduler {
		bla.Scheduler = append(bla.Scheduler, *cfgdata)
	}

	return bla
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

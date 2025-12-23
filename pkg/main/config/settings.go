package config

import "slices"

func GetSettingsGeneral() *GeneralConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.General
}

func GetSettingsImdb() *ImdbConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Imdb
}

func GetSettingsMedia(name string) *MediaTypeConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Media[name]
}

func GetSettingsMediaAll() *MediaConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return &currentSnapshot.cachetoml.Media
}

func GetSettingsPath(name string) *PathsConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Path[name]
}

func GetSettingsPathAll() []PathsConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Paths
}

func GetSettingsDownloaderAll() []DownloaderConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Downloader
}

func GetSettingsRegexAll() []RegexConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Regex
}

func GetSettingsQuality(name string) *QualityConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Quality[name]
}

func GetSettingsQualityAll() []QualityConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Quality
}

func GetSettingsQualityOk(name string) (*QualityConfig, bool) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil, false
	}

	val, ok := currentSnapshot.Quality[name]

	return val, ok
}

func GetSettingsQualityLen() int {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return 0
	}

	return len(currentSnapshot.Quality)
}

func GetSettingsScheduler(name string) *SchedulerConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Scheduler[name]
}

func GetSettingsSchedulerAll() []SchedulerConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Scheduler
}

func GetSettingsList(name string) *ListsConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.List[name]
}

func GetSettingsListAll() []ListsConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Lists
}

func GetSettingsMediaListAll() []string {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	retlists := make([]string, 0, 50)
	for _, cfg := range currentSnapshot.Media {
		for i := range cfg.Lists {
			if slices.Contains(retlists, cfg.Lists[i].Name) {
				continue
			}

			retlists = append(retlists, cfg.Lists[i].Name)
		}
	}

	return retlists
}

func TestSettingsList(name string) bool {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return false
	}

	_, ok := currentSnapshot.List[name]

	return ok
}

func GetSettingsIndexer(name string) *IndexersConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Indexer[name]
}

func GetSettingsIndexerAll() []IndexersConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Indexers
}

func GetSettingsNotification(name string) *NotificationConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.Notification[name]
}

func GetSettingsNotificationAll() []NotificationConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	return currentSnapshot.cachetoml.Notification
}

func RangeSettingsMedia(fn func(string, *MediaTypeConfig) error) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Media {
		fn(key, cfg)
	}
}

func RangeSettingsMediaLists(media string, fn func(*MediaListsConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for _, cfg := range currentSnapshot.Media[media].Lists {
		fn(&cfg)
	}
}

func RangeSettingsMediaBreak(fn func(string, *MediaTypeConfig) bool) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Media {
		if fn(key, cfg) {
			break
		}
	}
}

func RangeSettingsQuality(fn func(string, *QualityConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Quality {
		fn(key, cfg)
	}
}

func RangeSettingsList(fn func(string, *ListsConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.List {
		fn(key, cfg)
	}
}

func RangeSettingsIndexer(fn func(string, *IndexersConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Indexer {
		fn(key, cfg)
	}
}

func RangeSettingsScheduler(fn func(string, *SchedulerConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Scheduler {
		fn(key, cfg)
	}
}

func RangeSettingsNotification(fn func(string, *NotificationConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Notification {
		fn(key, cfg)
	}
}

func RangeSettingsPath(fn func(string, *PathsConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Path {
		fn(key, cfg)
	}
}

func RangeSettingsRegex(fn func(string, *RegexConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Regex {
		fn(key, cfg)
	}
}

func RangeSettingsDownloader(fn func(string, *DownloaderConfig)) {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return
	}

	for key, cfg := range currentSnapshot.Downloader {
		fn(key, cfg)
	}
}

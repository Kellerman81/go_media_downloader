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
		if cfg == nil {
			continue
		}

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
		if cfg == nil {
			continue
		}

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

// defaultMusicMetaSourcePriority is used when MusicMetaSourcePriority is empty.
var defaultMusicMetaSourcePriority = []string{"musicbrainz", "acoustid", "lastfm", "discogs", "deezer", "theaudiodb", "itunes"}

// GetMusicMetaSourcePriority returns the ordered list of music metadata providers to use.
// When MusicMetaSourcePriority is empty all providers are returned in the default order.
// Providers that require an API key ("acoustid", "lastfm") are excluded when their key
// is not configured, regardless of their position in the list.
// "musicbrainz" and "discogs" have no key requirement and are never auto-excluded.
func GetMusicMetaSourcePriority() []string {
	cfg := GetSettingsGeneral()
	if cfg == nil {
		return defaultMusicMetaSourcePriority
	}

	base := cfg.MusicMetaSourcePriority
	if len(base) == 0 {
		base = defaultMusicMetaSourcePriority
	}

	// Only filter providers whose required credentials are absent.
	// "musicbrainz" and "discogs" are unconditional — no key needed.
	out := make([]string, 0, len(base))
	for _, p := range base {
		switch p {
		case "acoustid":
			if cfg.AcoustIDAPIKey == "" {
				continue
			}
		case "lastfm":
			if cfg.LastFMAPIKey == "" {
				continue
			}
		}
		out = append(out, p)
	}
	return out
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

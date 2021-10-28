package config

import (
	"errors"
	"regexp"
	"strings"

	"github.com/recoilme/pudge"
)

var ConfigDB *pudge.Db
var configEntries map[string]interface{}

func OpenConfig(file string) (db *pudge.Db, err error) {
	cfg := &pudge.Config{
		SyncInterval: 1} // every second fsync
	mydb, err := pudge.Open(file, cfg)
	configEntries = make(map[string]interface{}, 100)
	return mydb, err
}

func CacheConfig() {
	keys, _ := ConfigDB.Keys([]byte("*"), 0, 0, true)

	for _, idx := range keys {
		if strings.HasPrefix(string(idx), "general") {
			var general GeneralConfig
			ConfigDB.Get("general", &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "imdb") {
			var general ImdbConfig
			ConfigDB.Get("imdb", &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "path_") {
			var general PathsConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "downloader_") {
			var general DownloaderConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "indexer_") {
			var general IndexersConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "list_") {
			var general ListsConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "movie_") {
			var general MediaTypeConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "serie_") {
			var general MediaTypeConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "notification_") {
			var general NotificationConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "quality_") {
			var general QualityConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "regex_") {
			var general RegexConfig
			ConfigDB.Get(string(idx), &general)
			general.RejectedRegex = make(map[string]*regexp.Regexp)
			general.RequiredRegex = make(map[string]*regexp.Regexp)
			for _, rowtitle := range general.Rejected {
				general.RejectedRegex[rowtitle] = regexp.MustCompile(rowtitle)
			}
			for _, rowtitle := range general.Required {
				general.RequiredRegex[rowtitle] = regexp.MustCompile(rowtitle)
			}
			configEntries[string(idx)] = general
			continue
		}
		if strings.HasPrefix(string(idx), "scheduler_") {
			var general SchedulerConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
	}
}

func ConfigGet(key string, val interface{}) error {
	if _, ok := configEntries[key]; ok {

		if strings.HasPrefix(key, "general") {
			*val.(*GeneralConfig) = configEntries[key].(GeneralConfig)
			return nil
		}
		if strings.HasPrefix(key, "imdb") {
			*val.(*ImdbConfig) = configEntries[key].(ImdbConfig)
			return nil
		}
		if strings.HasPrefix(key, "path_") {
			*val.(*PathsConfig) = configEntries[key].(PathsConfig)
			return nil
		}
		if strings.HasPrefix(key, "downloader_") {
			*val.(*DownloaderConfig) = configEntries[key].(DownloaderConfig)
			return nil
		}
		if strings.HasPrefix(key, "indexer_") {
			*val.(*IndexersConfig) = configEntries[key].(IndexersConfig)
			return nil
		}
		if strings.HasPrefix(key, "list_") {
			*val.(*ListsConfig) = configEntries[key].(ListsConfig)
			return nil
		}
		if strings.HasPrefix(key, "movie_") {
			*val.(*MediaTypeConfig) = configEntries[key].(MediaTypeConfig)
			return nil
		}
		if strings.HasPrefix(key, "serie_") {
			*val.(*MediaTypeConfig) = configEntries[key].(MediaTypeConfig)
			return nil
		}
		if strings.HasPrefix(key, "notification_") {
			*val.(*NotificationConfig) = configEntries[key].(NotificationConfig)
			return nil
		}
		if strings.HasPrefix(key, "quality_") {
			*val.(*QualityConfig) = configEntries[key].(QualityConfig)
			return nil
		}
		if strings.HasPrefix(key, "regex_") {
			*val.(*RegexConfig) = configEntries[key].(RegexConfig)
			return nil
		}
		if strings.HasPrefix(key, "scheduler_") {
			*val.(*SchedulerConfig) = configEntries[key].(SchedulerConfig)
			return nil
		}
		return nil
	} else {
		return errors.New("config not found")
	}
}

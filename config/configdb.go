package config

import (
	"errors"
	"regexp"
	"strings"
	"sync"

	"github.com/recoilme/pudge"
	"golang.org/x/oauth2"
)

var ConfigDB *pudge.Db
var configEntries map[string]interface{}
var cfglock = sync.RWMutex{}

func OpenConfig(file string) (db *pudge.Db, err error) {
	cfg := &pudge.Config{
		SyncInterval: 1} // every second fsync
	mydb, err := pudge.Open(file, cfg)
	configEntries = make(map[string]interface{}, 100)
	return mydb, err
}

func CacheConfig() {
	keys, _ := ConfigDB.Keys([]byte("*"), 0, 0, true)
	cfglock.Lock()
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
		if strings.HasPrefix(string(idx), "trakt_token") {
			var general oauth2.Token
			ConfigDB.Get("trakt_token", &general)
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
			var general RegexConfigIn
			ConfigDB.Get(string(idx), &general)
			var generalCache RegexConfig
			generalCache.Name = general.Name
			generalCache.Rejected = general.Rejected
			generalCache.Required = general.Required
			generalCache.RejectedRegex = make(map[string]*regexp.Regexp)
			generalCache.RequiredRegex = make(map[string]*regexp.Regexp)
			for _, rowtitle := range general.Rejected {
				generalCache.RejectedRegex[rowtitle] = regexp.MustCompile(rowtitle)
			}
			for _, rowtitle := range general.Required {
				generalCache.RequiredRegex[rowtitle] = regexp.MustCompile(rowtitle)
			}
			configEntries[string(idx)] = generalCache
			continue
		}
		if strings.HasPrefix(string(idx), "scheduler_") {
			var general SchedulerConfig
			ConfigDB.Get(string(idx), &general)
			configEntries[string(idx)] = general
			continue
		}
	}
	cfglock.Unlock()
}

func ConfigGetAll() map[string]interface{} {
	cfglock.RLock()
	defer cfglock.RUnlock()
	return configEntries
}

func ConfigGet(key string, val interface{}) error {
	cfglock.RLock()
	defer cfglock.RUnlock()
	if _, ok := configEntries[key]; ok {

		if strings.HasPrefix(key, "general") {
			*val.(*GeneralConfig) = configEntries[key].(GeneralConfig)
			return nil
		}
		if strings.HasPrefix(key, "imdb") {
			*val.(*ImdbConfig) = configEntries[key].(ImdbConfig)
			return nil
		}
		if strings.HasPrefix(key, "trakt_token") {
			*val.(*oauth2.Token) = configEntries[key].(oauth2.Token)
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

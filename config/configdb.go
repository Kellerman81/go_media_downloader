package config

import (
	"strings"

	"github.com/Kellerman81/go_media_downloader/logger"
)

func checkduplicateprefix(key string, prefix string) string {
	if strings.HasPrefix(key, prefix+prefix) {
		return strings.TrimPrefix(key, prefix)
	}
	return key
}
func ConfigCheck(key string) bool {
	// key = checkduplicateprefix(key, "list_")
	// key = checkduplicateprefix(key, "path_")
	// key = checkduplicateprefix(key, "indexer_")
	// key = checkduplicateprefix(key, "downloader_")
	// key = checkduplicateprefix(key, "regex_")
	// key = checkduplicateprefix(key, "notification_")
	// key = checkduplicateprefix(key, "scheduler_")
	// key = checkduplicateprefix(key, "movie_")
	// key = checkduplicateprefix(key, "serie_")
	// key = checkduplicateprefix(key, "quality_")
	for idx := range configEntries {
		if configEntries[idx].Name == key {
			return true
		}
	}
	logger.Log.Errorln("Config not found: ", key)
	return false
}

// func ConfigCheckold(name string) bool {
// 	success := true
// 	if _, ok := configEntries[name]; !ok {
// 		logger.Log.Errorln("Config not found: ", name)
// 		success = false
// 	}
// 	return success
// }

func ConfigGetAll() []*Conf {
	var b []*Conf
	for idx := range configEntries {
		b = append(b, &configEntries[idx])
	}
	return b
}

type Conf struct {
	Name string
	Data interface{}
}

var configEntries []Conf

func ConfigGet(key string) *Conf {
	key = strings.Replace(key, "list_list_", "list_", 1)
	key = strings.Replace(key, "path_path_", "path_", 1)
	key = strings.Replace(key, "indexer_indexer_", "indexer_", 1)
	key = strings.Replace(key, "downloader_downloader_", "downloader_", 1)
	key = strings.Replace(key, "regex_regex_", "regex_", 1)
	key = strings.Replace(key, "notification_notification_", "notification_", 1)
	key = strings.Replace(key, "scheduler_scheduler_", "scheduler_", 1)
	key = strings.Replace(key, "movie_movie_", "movie_", 1)
	key = strings.Replace(key, "serie_serie_", "serie_", 1)
	key = strings.Replace(key, "quality_quality_", "quality_", 1)
	if key != "general" && key != "imdb" && key != "trakt_token" {
		if !strings.Contains(key, "_") {
			logger.Log.Errorln("Config not found: ", key)
			return nil
		}
	}
	for idx := range configEntries {
		if configEntries[idx].Name == key {
			return &configEntries[idx]
		}
	}
	logger.Log.Errorln("Config not found: ", key)
	return nil
}

func ConfigGetPrefix(key string) []*Conf {
	var b []*Conf
	for idx := range configEntries {
		if strings.HasPrefix(configEntries[idx].Name, key) {
			b = append(b, &configEntries[idx])
		}
	}
	return b
}

func ConfigGetMediaListConfig(config string, name string) MediaListsConfig {
	cfg := ConfigGet(config).Data.(MediaTypeConfig)

	var cfg_list MediaListsConfig
	for idxlist := range cfg.Lists {
		if cfg.Lists[idxlist].Name == name {
			cfg_list = cfg.Lists[idxlist]
			return cfg_list
		}
	}
	return MediaListsConfig{}
}

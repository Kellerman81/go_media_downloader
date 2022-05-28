package config

import (
	"regexp"
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

type RegexSafe struct {
	Name string
	Re   regexp.Regexp
}

var regexEntries []RegexSafe

func RegexCheck(key string) bool {
	for idx := range regexEntries {
		if regexEntries[idx].Name == key {
			return true
		}
	}
	return false
}

func RegexGet(key string) *regexp.Regexp {
	for idx := range regexEntries {
		if regexEntries[idx].Name == key {
			return &regexEntries[idx].Re
		}
	}
	logger.Log.Errorln("Regex not found: ", key)
	return nil
}

func RegexAdd(key string, re regexp.Regexp) {
	if !RegexCheck(key) {
		regexEntries = append(regexEntries, RegexSafe{Name: key, Re: re})
	}
}

func RegexDelete(key string) {
	new := regexEntries[:0]
	for idx := range regexEntries {
		if regexEntries[idx].Name != key {
			new = append(new, regexEntries[idx])
		}
	}
	regexEntries = new
}

var configEntries []Conf

func ConfigGet(key string) *Conf {
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

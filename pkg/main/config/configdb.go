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
	if _, ok := MapConfigEntries[key]; ok {
		return true
	}
	logger.Log.Errorln("Config not found: ", key)
	return false
}

func ConfigGetAll() []Conf {
	var b []Conf
	defer logger.ClearVar(&b)
	for idx := range ConfigEntries {
		b = append(b, ConfigEntries[idx])
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

var RegexEntries []RegexSafe

func RegexCheck(key string) bool {
	for idx := range RegexEntries {
		if RegexEntries[idx].Name == key {
			return true
		}
	}
	return false
}

func RegexGet(key string) *regexp.Regexp {
	for idx := range RegexEntries {
		if RegexEntries[idx].Name == key {
			return &RegexEntries[idx].Re
		}
	}
	logger.Log.Errorln("Regex not found: ", key)
	return nil
}

func RegexAdd(key string, re regexp.Regexp) {
	if !RegexCheck(key) {
		RegexEntries = append(RegexEntries, RegexSafe{Name: key, Re: re})
	}
}

func RegexDelete(key string) {
	new := RegexEntries[:0]
	defer logger.ClearVar(&new)
	for idx := range RegexEntries {
		if RegexEntries[idx].Name != key {
			new = append(new, RegexEntries[idx])
		}
	}
	RegexEntries = new
}

var ConfigEntries []Conf
var MapConfigEntries map[string]*Conf

func ConfigGet(key string) *Conf {
	if val, ok := MapConfigEntries[key]; ok {
		return val
	}
	logger.Log.Errorln("Config not found: ", key)
	return nil
}

func ConfigGetPrefix(key string) []Conf {
	var b []Conf
	defer logger.ClearVar(&b)
	for idx := range ConfigEntries {
		if strings.HasPrefix(ConfigEntries[idx].Name, key) {
			b = append(b, ConfigEntries[idx])
		}
	}

	return b
}

func ConfigGetMediaListConfig(config string, name string) MediaListsConfig {
	cfgnotp := ConfigGet(config).Data
	if cfgnotp == nil {
		return MediaListsConfig{}
	}
	cfg := cfgnotp.(MediaTypeConfig)
	for idxlist := range cfg.Lists {
		if cfg.Lists[idxlist].Name == name {
			return cfg.Lists[idxlist]
		}
	}
	return MediaListsConfig{}
}

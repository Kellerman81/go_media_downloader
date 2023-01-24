package config

import (
	"reflect"
	"strings"

	"github.com/Kellerman81/go_media_downloader/logger"
	"golang.org/x/oauth2"
)

type Conf struct {
	Name string
	Data interface{}
}

func Check(key string) bool {
	return Cfg.Keys[key]
}

func RegexGetMatchesStr1Str2(key string, matchfor string) (string, string) {
	matches := logger.GlobalRegexCache.GetRegexpDirect(key).FindStringSubmatch(matchfor)
	if len(matches) >= 3 {
		return matches[1], matches[2]
	}
	if len(matches) >= 2 {
		return matches[1], ""
	}
	matches = nil
	return "", ""
}

func RegexGetMatchesFind(key string, matchfor string, mincount int) bool {
	if mincount == 1 {
		return logger.GlobalRegexCache.GetRegexpDirect(key).MatchString(matchfor)
	}
	matches := logger.GlobalRegexCache.GetRegexpDirect(key).FindStringSubmatchIndex(matchfor)
	if matches == nil {
		return false
	}
	if len(matches) == 0 {
		matches = nil
		return false
	}
	if matches[1] >= mincount {
		matches = nil
		return true
	}
	matches = nil
	return false
}

func GetTrakt(key string) *oauth2.Token {
	if logger.GlobalCache.Check(key, reflect.TypeOf(oauth2.Token{})) {
		value := logger.GlobalCache.GetData(key).Value.(oauth2.Token)
		return &value
	}
	return &oauth2.Token{}
}

func FindconfigTemplateOnList(typeprefix string, listname string) string {
	for idx := range Cfg.Media {
		if !strings.HasPrefix(idx, typeprefix) {
			continue
		}
		for idxlist := range Cfg.Media[idx].Lists {
			if Cfg.Media[idx].Lists[idxlist].Name == listname {
				return idx
			}
		}
	}
	return ""
}

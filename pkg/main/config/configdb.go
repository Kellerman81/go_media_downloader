package config

import (
	"reflect"
	"strings"

	"github.com/Kellerman81/go_media_downloader/logger"
	"golang.org/x/oauth2"
)

func ConfigCheck(key string) bool {
	return Cfg.Keys[key]
}

type Conf struct {
	Name string
	Data interface{}
}

func RegexCheck(key string) bool {
	return logger.GlobalRegexCache.CheckRegexp(key)
}

func RegexGetMatches(key string, matchfor string) []string {
	rgx := logger.GlobalRegexCache.GetRegexpDirect(key)
	defer func() {
		rgx = nil
	}()
	return rgx.FindStringSubmatch(matchfor)
}

func RegexGetMatchesStr1Str2(key string, matchfor string) (string, string) {
	rgx := logger.GlobalRegexCache.GetRegexpDirect(key)
	defer func() {
		rgx = nil
	}()
	matches := rgx.FindStringSubmatch(matchfor)
	defer logger.ClearVar(&matches)
	if len(matches) >= 2 {
		if len(matches) >= 3 {
			return matches[1], matches[2]
		} else {
			return matches[1], ""
		}
	} else {
		return "", ""
	}
}

func RegexGetMatchesFind(key string, matchfor string, mincount int) bool {
	rgx := logger.GlobalRegexCache.GetRegexpDirect(key)
	defer func() {
		rgx = nil
	}()
	return len(rgx.FindStringSubmatchIndex(matchfor)) >= mincount
}
func RegexGetAllMatches(key string, matchfor string, maxcount int) [][]string {
	rgx := logger.GlobalRegexCache.GetRegexpDirect(key)
	defer func() {
		rgx = nil
	}()
	return rgx.FindAllStringSubmatch(matchfor, maxcount)
}
func RegexGetLastMatches(key string, matchfor string, maxcount int) []string {
	rgx := logger.GlobalRegexCache.GetRegexpDirect(key)
	defer func() {
		rgx = nil
	}()
	matchest := rgx.FindAllStringSubmatch(matchfor, maxcount)
	defer logger.ClearVar(&matchest)
	if len(matchest) == 0 {
		return []string{}
	}
	return matchest[len(matchest)-1]
}

func ConfigGetTrakt(key string) *oauth2.Token {
	if logger.GlobalConfigCache.Check(key, reflect.TypeOf(oauth2.Token{})) {
		value := logger.GlobalConfigCache.GetData(key).Value.(oauth2.Token)
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
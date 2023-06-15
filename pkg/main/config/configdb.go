package config

import (
	"reflect"
	"regexp"
	"strings"

	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/Kellerman81/go_media_downloader/logger"
	"golang.org/x/oauth2"
)

type Conf struct {
	Name string
	Data interface{}
}

func Check(key string) bool {
	return checksettings(key)
	//return logger.Settings.CheckNoType(key)
}
func CheckGroup(group string, key string) bool {
	return checksettings(group + key)
	//return logger.Settings.CheckNoType(group + key)
}

func checksettings(key string) bool {
	if strings.HasPrefix(key, "general") {
		return true
	}
	if strings.HasPrefix(key, "downloader_") {
		_, exists := SettingsDownloader[key]
		return exists
	}
	if strings.HasPrefix(key, logger.StrImdb) {
		return true
	}
	if strings.HasPrefix(key, "indexer") {
		_, exists := SettingsIndexer[key]
		return exists
	}
	if strings.HasPrefix(key, "list") {
		_, exists := SettingsList[key]
		return exists
	}
	if strings.HasPrefix(key, logger.StrSerie) {
		_, exists := SettingsMedia[key]
		return exists
	}
	if strings.HasPrefix(key, logger.StrMovie) {
		_, exists := SettingsMedia[key]
		return exists
	}
	if strings.HasPrefix(key, "notification") {
		_, exists := SettingsNotification[key]
		return exists
	}
	if strings.HasPrefix(key, "path") {
		_, exists := SettingsPath[key]
		return exists
	}
	if strings.HasPrefix(key, "quality") {
		_, exists := SettingsQuality[key]
		return exists
	}
	if strings.HasPrefix(key, "regex") {
		_, exists := SettingsRegex[key]
		return exists
	}
	if strings.HasPrefix(key, "scheduler") {
		_, exists := SettingsScheduler[key]
		return exists
	}
	return false
}

func Getmatches(cached bool, key *string, matchfor *string) *[]int {
	var i *regexp.Regexp
	if !cached {
		i = regexp.MustCompile(*key)
	} else {
		i = logger.GlobalCacheRegex.GetRegexpDirect(key)
	}
	return logger.GetP(i.FindStringSubmatchIndex(*matchfor))
}
func RegexGetMatchesStr1Str2(cached bool, key *string, matchfor *string) (string, string) {
	matches := Getmatches(cached, key, matchfor)

	if len(*matches) == 0 {
		return "", ""
	}
	defer logger.Clear(matches)
	//ex Date := [0,8,-1,-1,0,8]
	if len(*matches) >= 6 && (*matches)[3] != -1 && (*matches)[5] != -1 {
		return (*matchfor)[(*matches)[2]:(*matches)[3]], (*matchfor)[(*matches)[4]:(*matches)[5]]
	}
	if len(*matches) >= 6 && (*matches)[3] == -1 && (*matches)[5] != -1 {
		return "", (*matchfor)[(*matches)[4]:(*matches)[5]]
	}
	if len(*matches) >= 4 && (*matches)[3] != -1 {
		return (*matchfor)[(*matches)[2]:(*matches)[3]], ""
	}
	//logger.Log.Debug().Ints("found", *matches).Str("search", *matchfor).Str("key", *key).Int("count", len(matches)).Msg("matches")
	//logger.LogAnyDebug("matches", logger.LoggerValue{Name: "search", Value: matchfor}, logger.LoggerValue{Name: "key", Value: key}, logger.LoggerValue{Name: "count", Value: len(matches)}, logger.LoggerValue{Name: "found", Value: matches})

	return "", ""
}

func RegexGetMatchesFind(key *string, matchfor string, mincount int) bool {
	if mincount == 1 {
		return len(logger.GlobalCacheRegex.GetRegexpDirect(key).FindStringIndex(matchfor)) >= 1
	}
	return len(logger.GlobalCacheRegex.GetRegexpDirect(key).FindAllStringIndex(matchfor, mincount)) >= mincount
}

func GetTrakt(key string) *oauth2.Token {
	if logger.GlobalCache.Check(key, reflect.TypeOf(oauth2.Token{})) {
		value := cache.GetDataT[oauth2.Token](logger.GlobalCache, key)
		return &value
	}
	return &oauth2.Token{}
}

func FindconfigTemplateNameOnList(typeprefix string, listname string) string {
	pre := "serie_"
	if strings.HasPrefix(typeprefix, logger.StrMovie) {
		pre = "movie_"
	}
	for idxm := range SettingsMedia {
		if strings.HasPrefix(SettingsMedia[idxm].NamePrefix, pre) {
			for idx := range SettingsMedia[idxm].Lists {
				if strings.EqualFold(SettingsMedia[idxm].Lists[idx].Name, listname) {
					return SettingsMedia[idxm].NamePrefix
				}
			}
		}
	}
	logger.Log.Debug().Str("type", typeprefix).Str("list", listname).Msg("config template not found")
	return ""
}

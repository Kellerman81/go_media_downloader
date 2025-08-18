package config

import (
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"golang.org/x/oauth2"
)

// Conf is a struct that contains a Name string field and a Data any field.
type Conf struct {
	// Name is a string field
	Name string
	// Data is an any field that can hold any type
	Data any
}

// CheckGroup checks if a setting key exists in the given settings group.
// It takes a group name string and a key string as parameters.
// It returns a boolean indicating if the key exists in the given group.
func CheckGroup(group string, key string) bool {
	mu.RLock()
	defer mu.RUnlock()
	var exists bool
	switch strings.TrimRight(group, "_") {
	case "general", logger.StrImdb:
		return true
	case "downloader":
		_, exists = settings.SettingsDownloader[key]
	case "indexer":
		_, exists = settings.SettingsIndexer[key]
	case "list":
		_, exists = settings.SettingsList[key]
	case logger.StrSerie, logger.StrMovie:
		_, exists = settings.SettingsMedia[key]
	case "notification":
		_, exists = settings.SettingsNotification[key]
	case "path":
		_, exists = settings.SettingsPath[key]
	case "quality":
		_, exists = settings.SettingsQuality[key]
	case "regex":
		_, exists = settings.SettingsRegex[key]
	case "scheduler":
		_, exists = settings.SettingsScheduler[key]
	}
	return exists
}

// GetTrakt returns the OAuth2 token for accessing the Trakt API.
// It first checks if the traktToken variable is nil, and if so logs
// a debug message and returns an empty oauth2.Token struct.
// Otherwise it returns the existing traktToken.
func GetTrakt() *oauth2.Token {
	mu.RLock()
	defer mu.RUnlock()
	if traktToken == nil {
		logger.Logtype("debug", 0).Msg("token empty")
		return &oauth2.Token{}
	}
	return traktToken
}

package config

import (
	"strings"

	"github.com/Kellerman81/go_media_downloader/logger"
	"golang.org/x/oauth2"
)

// Conf is a struct that contains a Name string field and a Data any field
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
	var exists bool
	switch strings.TrimRight(group, "_") {
	case "general", logger.StrImdb:
		return true
	case "downloader":
		_, exists = SettingsDownloader[key]
	case "indexer":
		_, exists = SettingsIndexer[key]
	case "list":
		_, exists = SettingsList[key]
	case logger.StrSerie, logger.StrMovie:
		_, exists = SettingsMedia[key]
	case "notification":
		_, exists = SettingsNotification[key]
	case "path":
		_, exists = SettingsPath[key]
	case "quality":
		_, exists = SettingsQuality[key]
	case "regex":
		_, exists = SettingsRegex[key]
	case "scheduler":
		_, exists = SettingsScheduler[key]
	}
	return exists
}

// GetTrakt returns the OAuth2 token for accessing the Trakt API.
// It first checks if the traktToken variable is nil, and if so logs
// a debug message and returns an empty oauth2.Token struct.
// Otherwise it returns the existing traktToken.
func GetTrakt() *oauth2.Token {
	if traktToken == nil {
		logger.LogDynamic("debug", "token empty")
		return &oauth2.Token{}
	}
	return traktToken
}

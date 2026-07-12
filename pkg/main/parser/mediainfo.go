package parser

import (
	"path/filepath"
	"runtime"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// mediaInfoTrack is one track entry from mediainfo JSON output.
// The "@type" value is capitalized by mediainfo ("General", "Video", "Audio").
type mediaInfoTrack struct {
	Type     string `json:"@type"`
	Format   string `json:"Format"`
	Duration string `json:"Duration"`
	CodecID  string `json:"CodecID,omitempty"`
	Width    string `json:"Width,omitempty"`
	Height   string `json:"Height,omitempty"`
	Language string `json:"Language,omitempty"`
}

type mediaInfoJSON struct {
	Media struct {
		Track []mediaInfoTrack `json:"track"`
	} `json:"media"`
}

// getmediainfoFilename returns the path to the mediainfo executable.
// It first checks if a custom path has been set in mediainfopath.
// If not, it constructs the default path based on the OS and the
// config.GetSettingsGeneral().MediainfoPath setting.
func getmediainfoFilename() string {
	if mediainfopath != "" {
		return mediainfopath
	}

	if runtime.GOOS == "windows" {
		mediainfopath = filepath.Join(config.GetSettingsGeneral().MediainfoPath, "mediainfo.exe")
	} else {
		mediainfopath = filepath.Join(config.GetSettingsGeneral().MediainfoPath, "mediainfo")
	}

	return mediainfopath
}

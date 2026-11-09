package providers

import (
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/whisper"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

var (
	whisperMu       sync.Mutex
	whisperURL      string
	whisperProvider *whisper.Provider
)

// GetWhisper returns the client for the configured whisper_url, or nil when
// none is set. It is built on first use and rebuilt when the URL changes, so a
// config reload needs no extra registration step.
func GetWhisper() *whisper.Provider {
	general := config.GetSettingsGeneral()
	if general == nil || general.WhisperURL == "" {
		return nil
	}

	whisperMu.Lock()
	defer whisperMu.Unlock()

	if whisperProvider == nil || whisperURL != general.WhisperURL {
		whisperProvider = whisper.NewProvider(general.WhisperURL, general.UserAgent)
		whisperURL = general.WhisperURL
	}

	return whisperProvider
}

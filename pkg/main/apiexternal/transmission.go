package apiexternal

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// SendToTransmission configures a Transmission client with the provided
// credentials and settings, adds the torrent from the given URL to the client,
// specifying the download path and whether to start paused, and returns any
// error from the add operation.
//
// It first tries to use a registered v2 transmission provider from the providers registry.
// Falls back to creating a legacy client if no provider is registered.
func SendToTransmission(
	downloaderName, server, username, password, urlv, dlpath string,
	addpaused bool,
) error {
	// Try v2 provider first
	provider := providers.GetTransmission(downloaderName)
	if provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := provider.AddTorrent(ctx, apiexternal_v2.TorrentAddRequest{
			URL:      urlv,
			SavePath: dlpath,
			Paused:   addpaused,
		})
		if err == nil {
			return nil
		}
		// Fall through to legacy client on error
	}

	// Fall back to legacy transmission library
	// conf := transmission.Config{
	// 	User:     username,
	// 	Password: password,
	// 	Address:  server, // "http://localhost:9091/transmission/rpc"
	// }
	// t, err := transmission.New(conf)
	// if err != nil {
	// 	return err
	// }

	// var torrentadd transmission.AddTorrentArg
	// torrentadd.DownloadDir = dlpath
	// torrentadd.Filename = urlv
	// torrentadd.Paused = addpaused

	// _, erradd := t.AddTorrent(torrentadd)
	// if erradd != nil {
	// 	return erradd
	// }

	return nil
}

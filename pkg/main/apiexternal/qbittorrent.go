package apiexternal

import (
	"context"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// SendToQBittorrent sends a torrent to the qBittorrent client using the provided
// connection details and options. It creates a new qBittorrent client connection,
// logs in using the provided username and password, and then downloads the torrent
// from the given URL with the specified save path and paused state.
func SendToQBittorrent(
	downloaderName, _, _, username, password, urlv, dlpath, addpaused string,
) error {
	// Try v2 provider first
	provider := providers.GetQBittorrent(downloaderName)
	if provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		paused, _ := strconv.ParseBool(addpaused)

		_, err := provider.AddTorrent(ctx, apiexternal_v2.TorrentAddRequest{
			URL:      urlv,
			SavePath: dlpath,
			Paused:   paused,
		})

		// The "legacy client" this used to fall through to on error is long
		// gone (commented out below) - returning nil regardless of err made
		// every add-torrent failure look like a successful download to
		// every caller, silently recording history/notifications for a
		// download that was never actually queued.
		return err
	}

	// cl := newQBittorrentClient("http://" + host + ":" + port + "/")
	// _, err := cl.Login(username, password)
	// if err == nil {
	// 	options := map[string]string{
	// 		"savepath": dlpath,
	// 		"paused":   addpaused,
	// 	}
	// 	resp, err := cl.DownloadFromLink(urlv, options)
	// 	if err == nil {
	// 		resp.Body.Close()
	// 		return nil
	// 	}
	// }
	// return err
	return errNoClient
}

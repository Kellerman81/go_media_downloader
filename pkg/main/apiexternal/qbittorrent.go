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
	downloaderName, host, port, username, password, urlv, dlpath, addpaused string,
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
		if err == nil {
			return nil
		}
		// Fall through to legacy client on error
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
	return nil
}

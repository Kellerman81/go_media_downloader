package apiexternal

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// SendToRtorrent sends a torrent file URL to an rTorrent
// instance for downloading. hostname and insecure specify the
// rTorrent server. urlv is the torrent file URL. dlpath is the
// download location path. name is the name to save the torrent
// as in rTorrent. Returns any error.
func SendToRtorrent(
	downloaderName, hostname string,
	insecure bool,
	urlv, dlpath, name string,
) error {
	// Try v2 provider first
	provider := providers.GetRTorrent(downloaderName)
	if provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := provider.AddTorrent(ctx, apiexternal_v2.TorrentAddRequest{
			URL:      urlv,
			SavePath: dlpath,
		})
		if err == nil {
			return nil
		}
		// Fall through to legacy client on error
	}
	// cl := rtorrent.New(hostname, insecure)

	// return cl.Add(urlv, rtorrent.DBasePath.SetValue(dlpath), rtorrent.DName.SetValue(name))
	return nil
}

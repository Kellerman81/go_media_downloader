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
	downloaderName, _ string,
	_ bool,
	urlv, dlpath, _ string,
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

		// The "legacy client" this used to fall through to on error is long
		// gone (commented out below) - returning nil regardless of err made
		// every add-torrent failure look like a successful download to
		// every caller, silently recording history/notifications for a
		// download that was never actually queued.
		return err
	}

	// cl := rtorrent.New(hostname, insecure)

	// return cl.Add(urlv, rtorrent.DBasePath.SetValue(dlpath), rtorrent.DName.SetValue(name))
	return errNoClient
}

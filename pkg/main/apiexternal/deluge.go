package apiexternal

import (
	"context"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// SendToDeluge connects to a Deluge server, authenticates, and adds a torrent from a magnet URI or URL.
// It configures options like download location, moving completed downloads, pausing on add, etc.
// Returns any error from the connection or add torrent operations.
func SendToDeluge(
	downloaderName string,
	_ string,
	_ int,
	_, _, urlv, dlpath string,
	moveafter bool,
	moveafterpath string,
	addpaused bool,
) error {
	// Try v2 provider first
	provider := providers.GetDeluge(downloaderName)
	if provider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		_, err := provider.AddTorrent(ctx, apiexternal_v2.TorrentAddRequest{
			URL:      urlv,
			SavePath: dlpath,
			Paused:   addpaused,
			Options: map[string]string{
				"move_completed":      strconv.FormatBool(moveafter),
				"move_completed_path": moveafterpath,
			},
		})

		// The "legacy client" this used to fall through to on error is long
		// gone (commented out below) - returning nil regardless of err made
		// every add-torrent failure (auth, connection refused, invalid
		// magnet, disk full, timeout) look like a successful download to
		// every caller, silently recording history/notifications for a
		// download that was never actually queued.
		return err
	}

	// cl := delugeclient.NewV2(delugeclient.Settings{
	// 	Hostname:             host,
	// 	Port:                 logger.IntToUint(port),
	// 	Login:                username,
	// 	Password:             password,
	// 	DebugServerResponses: true,
	// })

	// fls := false
	// // perform connection to Deluge server
	// err := cl.Connect()
	// if err != nil {
	// 	return err
	// }
	// if logger.HasPrefixI(urlv, "magnet") {
	// 	_, err = cl.AddTorrentMagnet(urlv, &delugeclient.Options{
	// 		DownloadLocation:  &dlpath,
	// 		MoveCompleted:     &moveafter,
	// 		MoveCompletedPath: &moveafterpath,
	// 		AutoManaged:       &fls,
	// 		AddPaused:         &addpaused,
	// 	})
	// } else {
	// 	_, err = cl.AddTorrentURL(urlv, &delugeclient.Options{
	// 		DownloadLocation:  &dlpath,
	// 		MoveCompleted:     &moveafter,
	// 		MoveCompletedPath: &moveafterpath,
	// 		AutoManaged:       &fls,
	// 		AddPaused:         &addpaused,
	// 	})
	// }
	return errNoClient
}

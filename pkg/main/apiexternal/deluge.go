package apiexternal

import (
	"github.com/Kellerman81/go_media_downloader/logger"
	delugeclient "github.com/gdm85/go-libdeluge"
)

func SendToDeluge(host string, port int, username string, password string, urlv string, dlpath string, moveafter bool, moveafterpath string, addpaused bool) error {
	cl := delugeclient.NewV2(delugeclient.Settings{
		Hostname:             host,
		Port:                 uint(port),
		Login:                username,
		Password:             password,
		DebugServerResponses: true,
	})

	fls := false
	// perform connection to Deluge server
	err := cl.Connect()
	if err == nil {
		if logger.HasPrefixI(urlv, "magnet") {
			_, err = cl.AddTorrentMagnet(urlv, &delugeclient.Options{
				DownloadLocation:  &dlpath,
				MoveCompleted:     &moveafter,
				MoveCompletedPath: &moveafterpath,
				AutoManaged:       &fls,
				AddPaused:         &addpaused,
			})
			if err != nil {
				return err
			}
		} else {
			_, err = cl.AddTorrentURL(urlv, &delugeclient.Options{
				DownloadLocation:  &dlpath,
				MoveCompleted:     &moveafter,
				MoveCompletedPath: &moveafterpath,
				AutoManaged:       &fls,
				AddPaused:         &addpaused,
			})
			if err != nil {
				return err
			}
		}
	}
	return err
}

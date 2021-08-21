package apiexternal

import (
	"strings"

	delugeclient "github.com/gdm85/go-libdeluge"
)

func SendToDeluge(host string, port int, username string, password string, url string, dlpath string, moveafter bool, moveafterpath string) error {
	deluge := delugeclient.NewV2(delugeclient.Settings{
		Hostname: host,
		Port:     uint(port),
		Login:    username,
		Password: password,
	})

	// perform connection to Deluge server
	err := deluge.Connect()
	if err == nil {
		if strings.HasPrefix(url, "magnet") {
			_, err = deluge.AddTorrentMagnet(url, &delugeclient.Options{
				DownloadLocation:  &dlpath,
				MoveCompleted:     &moveafter,
				MoveCompletedPath: &moveafterpath,
			})
		} else {
			_, err = deluge.AddTorrentURL(url, &delugeclient.Options{
				DownloadLocation:  &dlpath,
				MoveCompleted:     &moveafter,
				MoveCompletedPath: &moveafterpath,
			})
		}
	}
	return err
}

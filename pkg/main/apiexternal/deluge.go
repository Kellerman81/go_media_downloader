package apiexternal

import (
	"fmt"
	"strings"

	delugeclient "github.com/gdm85/go-libdeluge"
)

func SendToDeluge(host string, port int, username string, password string, url string, dlpath string, moveafter bool, moveafterpath string, addpaused bool) error {
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
		if strings.HasPrefix(url, "magnet") {
			_, err = cl.AddTorrentMagnet(url, &delugeclient.Options{
				DownloadLocation:  &dlpath,
				MoveCompleted:     &moveafter,
				MoveCompletedPath: &moveafterpath,
				AutoManaged:       &fls,
				AddPaused:         &addpaused,
			})
			if err != nil {
				fmt.Println(err)
			}
		} else {
			_, err = cl.AddTorrentURL(url, &delugeclient.Options{
				DownloadLocation:  &dlpath,
				MoveCompleted:     &moveafter,
				MoveCompletedPath: &moveafterpath,
				AutoManaged:       &fls,
				AddPaused:         &addpaused,
			})
			if err != nil {
				fmt.Println(err)
			}
		}
	} else {
		fmt.Println(err)
	}
	return err
}

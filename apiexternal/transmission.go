package apiexternal

import (
	"github.com/odwrtw/transmission"
)

func SendToTransmission(server string, username string, password string, url string, dlpath string, addpaused bool) error {
	conf := transmission.Config{
		User:     username,
		Password: password,
		Address:  server, // "http://localhost:9091/transmission/rpc"
	}
	t, err := transmission.New(conf)
	if err != nil {
		return err
	}

	var torrentadd transmission.AddTorrentArg
	torrentadd.DownloadDir = dlpath
	torrentadd.Filename = url
	torrentadd.Paused = addpaused

	_, erradd := t.AddTorrent(torrentadd)
	if erradd != nil {
		return erradd
	}
	t = nil

	return nil
}

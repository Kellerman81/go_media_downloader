package apiexternal

import (
	"github.com/odwrtw/transmission"
)

// SendToTransmission configures a Transmission client with the provided
// credentials and settings, adds the torrent from the given URL to the client,
// specifying the download path and whether to start paused, and returns any
// error from the add operation.
func SendToTransmission(server string, username string, password string, urlv string, dlpath string, addpaused bool) error {
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
	torrentadd.Filename = urlv
	torrentadd.Paused = addpaused

	_, erradd := t.AddTorrent(torrentadd)
	if erradd != nil {
		return erradd
	}

	return nil
}

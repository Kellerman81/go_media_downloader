package apiexternal

import (
	"github.com/mrobinsn/go-rtorrent/rtorrent"
)

// SendToRtorrent sends a torrent file URL to an rTorrent
// instance for downloading. hostname and insecure specify the
// rTorrent server. urlv is the torrent file URL. dlpath is the
// download location path. name is the name to save the torrent
// as in rTorrent. Returns any error.
func SendToRtorrent(hostname string, insecure bool, urlv, dlpath, name string) error {
	cl := rtorrent.New(hostname, insecure)

	return cl.Add(urlv, rtorrent.DBasePath.SetValue(dlpath), rtorrent.DName.SetValue(name))
}

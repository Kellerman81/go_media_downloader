package apiexternal

import (
	"github.com/mrobinsn/go-rtorrent/rtorrent"
)

func SendToRtorrent(hostname string, insecure bool, url string, dlpath string, name string) error {
	cl := rtorrent.New(hostname, insecure)
	defer func() {
		cl = nil
	}()
	return cl.Add(url, rtorrent.DBasePath.SetValue(dlpath), rtorrent.DName.SetValue(name))
}

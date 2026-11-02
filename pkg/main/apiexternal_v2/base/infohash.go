package base

import (
	"encoding/base32"
	"encoding/hex"
	"regexp"
	"strings"
)

// btihRe matches a hex (40) or base32 (32) BitTorrent info hash in a magnet
// link or a torrent URL that carries one.
var btihRe = regexp.MustCompile(`(?i)btih:([0-9a-f]{40}|[a-z2-7]{32})\b`)

// InfoHashFromURL returns the info hash embedded in a magnet/torrent URL as
// uppercase hex, or "" when there is none. Clients that don't return a hash
// from their add call (rTorrent, qBittorrent) use it to report one.
func InfoHashFromURL(u string) string {
	m := btihRe.FindStringSubmatch(u)
	if m == nil {
		return ""
	}

	if len(m[1]) == 40 {
		return strings.ToUpper(m[1])
	}

	raw, err := base32.StdEncoding.DecodeString(strings.ToUpper(m[1]))
	if err != nil {
		return ""
	}

	return strings.ToUpper(hex.EncodeToString(raw))
}

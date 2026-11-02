package base

import "testing"

func TestInfoHashFromURL(t *testing.T) {
	const hexHash = "C12FE1C06BBA254A9DC9F519B335AA7C1367A88A"

	for in, want := range map[string]string{
		"magnet:?xt=urn:btih:c12fe1c06bba254a9dc9f519b335aa7c1367a88a&dn=x": hexHash,
		"magnet:?xt=urn:btih:" + hexHash:                                    hexHash,
		// Base32 form of the same hash.
		"magnet:?xt=urn:btih:YEX6DQDLXISUVHOJ6UM3GNNKPQJWPKEK&tr=udp": hexHash,
		"https://tracker.example/download.php?id=1":                   "",
		"magnet:?xt=urn:btih:tooshort":                                "",
		"":                                                            "",
	} {
		if got := InfoHashFromURL(in); got != want {
			t.Errorf("InfoHashFromURL(%q) = %q, want %q", in, got, want)
		}
	}
}

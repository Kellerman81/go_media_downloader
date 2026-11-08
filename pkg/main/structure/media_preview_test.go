package structure

import (
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestMediaFileSkipReason(t *testing.T) {
	cfgp := &config.MediaTypeConfig{IsType: config.MediaTypeMovie}
	pathCfg := &config.PathsConfig{Disallowed: []string{"sample"}}

	for file, want := range map[string]string{
		"/imp/Movie/README":                  skipOther,
		"/imp/_UNPACK_Movie/movie.mkv":       skipUnpacking,
		"/imp/Movie/sample/movie-sample.mkv": skipDisallowed,
	} {
		if got := mediaFileSkipReason(cfgp, pathCfg, file); got != want {
			t.Errorf("%s: skip reason %q, want %q", file, got, want)
		}
	}
}

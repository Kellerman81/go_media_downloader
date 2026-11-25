package api

import (
	"net/http"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestLyrics: the lyrics tag of a file, in Subsonic and Jellyfin.
func TestLyrics(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	f := clientTestData(t)
	lyricsCache.Clear()

	song := filepath.Join(f.dir, "01 - Song.mp3")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "sine=duration=1",
		"-metadata", "lyrics=First line\nSecond line", "-c:a", "libmp3lame", song).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}

	w := f.do(t, http.MethodGet, "/subsonic/rest/getLyricsBySongId.view?u=admin&p=adminpass&id=tr31", nil, nil)
	if body := w.Body.String(); !strings.Contains(body, "<line>First line</line>") || !strings.Contains(body, "<line>Second line</line>") {
		t.Fatalf("subsonic: %s", body)
	}

	h := jfLogin(t, f, DefaultUsername, "adminpass")

	w = f.do(t, http.MethodGet, "/jellyfin/Audio/"+jfID(jfTrack, 31)+"/Lyrics", nil, h)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `{"Text":"Second line"}`) {
		t.Fatalf("jellyfin: %d %s", w.Code, w.Body)
	}
}

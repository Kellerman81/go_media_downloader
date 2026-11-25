package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestDirectPlayable(t *testing.T) {
	dp := &jfDeviceProfile{}
	dp.DirectPlayProfiles = append(dp.DirectPlayProfiles, struct {
		Container  string `json:"Container"`
		VideoCodec string `json:"VideoCodec"`
		AudioCodec string `json:"AudioCodec"`
		Type       string `json:"Type"`
	}{Container: "mp4,mkv", VideoCodec: "h264,hevc", AudioCodec: "aac,ac3", Type: "Video"})

	for _, tc := range []struct {
		container, v, a string
		want            bool
	}{{"mkv", "hevc", "ac3", true}, {"mp4", "h264", "aac", true}, {"avi", "h264", "aac", false}, {"mkv", "mpeg4", "aac", false}, {"mkv", "h264", "dts", false}} {
		if got := directPlayable(dp, tc.container, playProbe{VCodec: tc.v, ACodec: tc.a}); got != tc.want {
			t.Errorf("%+v: %v", tc, got)
		}
	}

	if !directPlayable(nil, "avi", playProbe{VCodec: "mpeg4"}) {
		t.Error("no profile: plays all")
	}
}

// TestJellyfinTranscode: a file the app can't play gets a transcoding URL
// that converts it to HLS.
func TestJellyfinTranscode(t *testing.T) {
	for _, tool := range []string{"ffmpeg", "ffprobe"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skip(tool + " not installed")
		}
	}

	f := clientTestData(t)

	g := config.GetSettingsGeneral()
	g.PlayerVideoProfiles, g.PlayerMaxTranscodes, g.CoverCacheDir = []string{"360p|360|600|96"}, 2, filepath.Join(t.TempDir(), "covers")

	movie := filepath.Join(f.dir, "movie.mkv")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc=size=320x240:rate=25:duration=4",
		"-f", "lavfi", "-i", "sine=duration=4", "-c:v", "mpeg4", "-c:a", "libmp3lame", movie).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}

	h := jfLogin(t, f, DefaultUsername, "adminpass")
	h.Set("Content-Type", "application/json")

	body := `{"DeviceProfile":{"DirectPlayProfiles":[{"Container":"mp4","VideoCodec":"h264","AudioCodec":"aac","Type":"Video"}]}}`

	w := f.do(t, http.MethodPost, "/jellyfin/Items/"+jfID(jfMovie, 1)+"/PlaybackInfo", strings.NewReader(body), h)

	var info struct {
		MediaSources  []jfMediaSource
		PlaySessionID string `json:"PlaySessionId"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil || len(info.MediaSources) != 1 {
		t.Fatalf("playback info: %d %s", w.Code, w.Body)
	}

	ms := info.MediaSources[0]
	if ms.SupportsDirectPlay || !ms.SupportsTranscoding || !strings.HasPrefix(ms.TranscodingURL, "/videos/") || ms.TranscodingSubProtocol != "hls" {
		t.Fatalf("source: %+v", ms)
	}

	w = f.do(t, http.MethodGet, "/jellyfin"+ms.TranscodingURL, nil, nil)

	m := regexp.MustCompile(`/jellyfin/hls/([0-9a-f]+)/index\.m3u8`).FindStringSubmatch(w.Body.String())
	if w.Code != http.StatusOK || m == nil {
		t.Fatalf("master: %d %s", w.Code, w.Body)
	}

	t.Cleanup(func() { hlsStop(m[1]) })

	w = f.do(t, http.MethodGet, m[0], nil, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "#EXTM3U") {
		t.Fatalf("media playlist: %d %s", w.Code, w.Body)
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/hls/0123456789abcdef01234567/index.m3u8", nil, nil); w.Code != http.StatusNotFound {
		t.Fatalf("unknown session: %d", w.Code)
	}

	// A file the app plays stays direct, without a transcoding URL (the web
	// client would use its HLS player on it).
	body = `{"DeviceProfile":{"DirectPlayProfiles":[{"Container":"mkv","VideoCodec":"mpeg4","AudioCodec":"mp3","Type":"Video"}]}}`
	w = f.do(t, http.MethodPost, "/jellyfin/Items/"+jfID(jfMovie, 1)+"/PlaybackInfo", strings.NewReader(body), h)

	if !strings.Contains(w.Body.String(), `"SupportsDirectPlay":true`) || strings.Contains(w.Body.String(), "TranscodingUrl") {
		t.Fatalf("direct: %s", w.Body)
	}
}

func TestJellyfinBitrateTest(t *testing.T) {
	f := clientTestData(t)
	h := jfLogin(t, f, DefaultUsername, "adminpass")

	if w := f.do(t, http.MethodGet, "/jellyfin/Playback/BitrateTest?size=500000", nil, h); w.Code != http.StatusOK || w.Body.Len() != 500000 {
		t.Fatalf("speed test: %d, %d bytes", w.Code, w.Body.Len())
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/Playback/BitrateTest?size=500000", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("without login: %d", w.Code)
	}
}

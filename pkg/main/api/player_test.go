package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// playerFixtures creates short media files with ffmpeg and registers them as
// movie and album files 1-4.
func playerFixtures(t *testing.T) map[string]string {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe not installed")
	}

	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN"), Music: mediaGroup("Music")})

	dir := t.TempDir()
	files := map[string][]string{
		"h264.mkv":  {"-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=3", "-f", "lavfi", "-i", "sine=duration=3", "-c:v", "libx264", "-c:a", "aac"},
		"mpeg4.avi": {"-f", "lavfi", "-i", "testsrc=size=1280x720:rate=25:duration=3", "-f", "lavfi", "-i", "sine=duration=3", "-c:v", "mpeg4", "-c:a", "libmp3lame"},
		"song.mp3":  {"-f", "lavfi", "-i", "sine=duration=3", "-c:a", "libmp3lame"},
		"song.wma":  {"-f", "lavfi", "-i", "sine=duration=3", "-c:a", "wmav2"},
	}

	paths := map[string]string{}

	for name, args := range files {
		p := filepath.Join(dir, name)
		if out, err := exec.Command("ffmpeg", append(append([]string{"-hide_banner", "-loglevel", "error", "-y"}, args...), p)...).CombinedOutput(); err != nil {
			t.Skipf("ffmpeg can't create %s: %v %s", name, err, out)
		}

		paths[name] = p
	}

	for _, q := range []string{
		"insert into movie_files (id, location) values (1, '" + paths["h264.mkv"] + "'), (2, '" + paths["mpeg4.avi"] + "')",
		"insert into album_files (id, location) values (3, '" + paths["song.mp3"] + "'), (4, '" + paths["song.wma"] + "')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	return paths
}

func playerRequest(t *testing.T, h gin.HandlerFunc, kind, id, query string, header http.Header) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/x?"+query, nil)

	for k, v := range header {
		c.Request.Header[k] = v
	}

	c.Params = gin.Params{{Key: "kind", Value: kind}, {Key: "id", Value: id}}
	h(c)

	return w
}

// probeBytes runs ffprobe on streamed output.
func probeBytes(t *testing.T, data []byte) (vcodec, acodec string, height int, duration float64) {
	t.Helper()

	f := filepath.Join(t.TempDir(), "out.mp4")
	if err := os.WriteFile(f, data, 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := probeMedia(t.Context(), f)
	if err != nil {
		t.Fatalf("output not readable: %v", err)
	}

	return p.VCodec, p.ACodec, p.Height, p.Duration
}

func TestPlayer(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := playerFixtures(t)

	info := func(kind, id string) map[string]any {
		w := playerRequest(t, HandlePlayerInfo, kind, id, "", nil)

		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil || w.Code != http.StatusOK {
			t.Fatalf("info %s/%s: %d %s", kind, id, w.Code, w.Body.String())
		}

		return out
	}

	for _, tc := range []struct{ kind, id, mode string }{
		{"movie", "1", "remux"}, {"movie", "2", "transcode"}, {"track", "3", "direct"}, {"track", "4", "transcode"},
	} {
		if got := info(tc.kind, tc.id); got["mode"] != tc.mode {
			t.Errorf("%s/%s mode = %v, want %s", tc.kind, tc.id, got["mode"], tc.mode)
		}
	}

	if q, _ := json.Marshal(info("movie", "2")["qualities"]); !strings.Contains(string(q), `"value":"720p"`) {
		t.Errorf("video qualities = %s", q)
	}

	if q, _ := json.Marshal(info("track", "3")["qualities"]); !strings.Contains(string(q), `"value":"128"`) ||
		!strings.Contains(string(q), `"converted":true,"kbps":128`) || !strings.Contains(string(q), `"converted":false`) {
		t.Errorf("audio qualities = %s", q)
	}

	// A measured conversion speed is reported with its quality.
	conversionSpeeds.Store("audio|64", 0.8)
	defer conversionSpeeds.Delete("audio|64")

	if q, _ := json.Marshal(info("track", "3")["qualities"]); !strings.Contains(string(q), `"kbps":64,"label":"64 kbit/s","speed":0.8`) {
		t.Errorf("qualities lack the speed: %s", q)
	}

	// Direct: the file itself, with range support.
	raw, _ := os.ReadFile(paths["song.mp3"])
	if w := playerRequest(t, HandlePlayerStream, "track", "3", "q=original", nil); !bytes.Equal(w.Body.Bytes(), raw) {
		t.Error("direct stream differs from the file")
	}

	if w := playerRequest(t, HandlePlayerStream, "track", "3", "q=original", http.Header{"Range": {"bytes=0-9"}}); w.Code != http.StatusPartialContent || w.Body.Len() != 10 {
		t.Errorf("range: %d, %d bytes", w.Code, w.Body.Len())
	}

	// Remux keeps H.264 and converts audio to AAC.
	w := playerRequest(t, HandlePlayerStream, "movie", "1", "q=original", nil)
	if v, a, _, _ := probeBytes(t, w.Body.Bytes()); v != "h264" || a != "aac" || w.Header().Get("Content-Type") != "video/mp4" {
		t.Errorf("remux: %s/%s %s", v, a, w.Header().Get("Content-Type"))
	}

	// Transcode to a profile: height limited, start position honored.
	w = playerRequest(t, HandlePlayerStream, "movie", "2", "q=480p&start=1", nil)
	if v, a, h, d := probeBytes(t, w.Body.Bytes()); v != "h264" || a != "aac" || h != 480 || d > 2.5 {
		t.Errorf("transcode: %s/%s height %d duration %.1f", v, a, h, d)
	}

	// Audio transcode at a bitrate, also of a file the browser could play.
	for _, id := range []string{"3", "4"} {
		w = playerRequest(t, HandlePlayerStream, "track", id, "q=64", nil)
		if v, a, _, _ := probeBytes(t, w.Body.Bytes()); v != "" || a != "mp3" || w.Header().Get("Content-Type") != "audio/mpeg" {
			t.Errorf("audio transcode of %s: %q/%q %s", id, v, a, w.Header().Get("Content-Type"))
		}
	}

	// Unknown quality, unknown file, transcode limit.
	if w := playerRequest(t, HandlePlayerStream, "movie", "2", "q=4k", nil); w.Code != http.StatusBadRequest {
		t.Errorf("unknown quality: %d", w.Code)
	}

	if w := playerRequest(t, HandlePlayerStream, "movie", "99", "", nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown file: %d", w.Code)
	}

	for range config.GetSettingsGeneral().PlayerMaxTranscodes {
		if !acquireTranscode() {
			t.Fatal("slot not free")
		}
	}

	if w := playerRequest(t, HandlePlayerStream, "movie", "2", "q=480p", nil); w.Code != http.StatusServiceUnavailable {
		t.Errorf("limit: %d", w.Code)
	}

	for range config.GetSettingsGeneral().PlayerMaxTranscodes {
		releaseTranscode()
	}

}

func TestStreamBuffer(t *testing.T) {
	// Data passes through in order, and the reader gets EOF after CloseWrite.
	b := newStreamBuffer(8)
	want := bytes.Repeat([]byte("0123456789"), 100)

	go func() {
		_, err := b.Write(want)
		b.CloseWrite(err)
	}()

	got, err := io.ReadAll(b)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("read %d bytes, err %v", len(got), err)
	}

	// A full buffer blocks the writer until the reader closes it.
	b = newStreamBuffer(4)
	res := make(chan error, 1)

	go func() {
		_, err := b.Write([]byte("more than four"))
		res <- err
	}()

	select {
	case err := <-res:
		t.Fatalf("write didn't block: %v", err)
	case <-time.After(50 * time.Millisecond):
	}

	b.Close()

	select {
	case err := <-res:
		if !errors.Is(err, errStreamClosed) {
			t.Errorf("write after close: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("write still blocked after Close")
	}
}

func TestFFmpegStderr(t *testing.T) {
	w := &ffmpegStderr{buf: newStreamBuffer(10)}

	// Progress before 10 s converted is start-up noise; messages are kept.
	fmt.Fprint(w, "out_time_us=4000000\nspeed=0.3x\n[aac @ 0x1] Too many bits\nout_time_us=12000000\nspe")
	fmt.Fprint(w, "ed=2.5x\nprogress=continue\n")

	if got := w.measured(); got != 2.5 {
		t.Errorf("speed = %v, want 2.5", got)
	}

	if got := w.messages(); got != "[aac @ 0x1] Too many bits\n" {
		t.Errorf("messages = %q", got)
	}

	// Once the buffer was full, ffmpeg waited for the client: not its speed.
	go w.buf.Write(make([]byte, 20))
	time.Sleep(50 * time.Millisecond)
	fmt.Fprint(w, "out_time_us=30000000\nspeed=1.0x\n")

	if got := w.measured(); got != 2.5 {
		t.Errorf("speed after a full buffer = %v, want 2.5", got)
	}

	w.buf.Close()
}

func TestPlayerSpeedTest(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/?kb=1500", nil)
	HandlePlayerSpeedTest(c)

	if w.Body.Len() != 1500<<10 || w.Header().Get("Content-Length") != strconv.Itoa(1500<<10) {
		t.Errorf("sent %d bytes, Content-Length %s", w.Body.Len(), w.Header().Get("Content-Length"))
	}
}

func TestPlayerHWAccel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := playerFixtures(t)

	const dev = "/dev/dri/renderD128"
	if _, err := os.Stat(dev); err != nil {
		t.Skip("no GPU render device")
	}

	if out, _ := exec.Command("ffmpeg", "-hide_banner", "-encoders").Output(); !strings.Contains(string(out), "h264_vaapi") {
		t.Skip("ffmpeg without h264_vaapi")
	}

	// The fixtures' test DB stays; only the GPU settings change.
	g := config.GetSettingsGeneral()
	oldHW, oldDev := g.PlayerHWAccel, g.PlayerHWDevice
	t.Cleanup(func() { g.PlayerHWAccel, g.PlayerHWDevice = oldHW, oldDev })

	general := func(device string) { g.PlayerHWAccel, g.PlayerHWDevice = "vaapi", device }

	hwFailedAt = time.Time{}
	t.Cleanup(func() { hwFailedAt = time.Time{} })

	// The GPU encodes; the output is H.264 at the profile's height.
	general(dev)

	args, usedHW, err := ffmpegArgs(paths["mpeg4.avi"], playKinds["movie"], playProbe{VCodec: "mpeg4", Height: 720}, "480p", 0, "vaapi", defaultSel)
	if err != nil || !usedHW || !strings.Contains(strings.Join(args, " "), "-c:v h264_vaapi") {
		t.Fatalf("vaapi args: %v %v %v", usedHW, err, args)
	}

	w := playerRequest(t, HandlePlayerStream, "movie", "2", "q=480p", nil)
	if v, _, h, _ := probeBytes(t, w.Body.Bytes()); v != "h264" || h != 480 {
		t.Skipf("this GPU can't encode H.264 with VAAPI here: %s/%d", v, h)
	}

	if !hwFailedAt.IsZero() {
		t.Error("working GPU marked as failed")
	}

	// A broken device: the same request falls back to the CPU, and the GPU
	// isn't tried again for a while.
	general("/dev/dri/does-not-exist")

	w = playerRequest(t, HandlePlayerStream, "movie", "2", "q=480p", nil)
	if v, _, h, _ := probeBytes(t, w.Body.Bytes()); v != "h264" || h != 480 {
		t.Errorf("fallback stream: %s/%d", v, h)
	}

	if hwFailedAt.IsZero() || playerHWAccel() != "none" {
		t.Error("failed GPU not remembered")
	}

	// Audio and remuxing never use the GPU.
	if _, usedHW, _ := ffmpegArgs(paths["song.mp3"], playKinds["track"], playProbe{ACodec: "mp3"}, "128", 0, "vaapi", defaultSel); usedHW {
		t.Error("audio uses the GPU")
	}

	if _, usedHW, _ := ffmpegArgs(paths["h264.mkv"], playKinds["movie"], playProbe{VCodec: "h264"}, "original", 0, "vaapi", defaultSel); usedHW {
		t.Error("remux uses the GPU")
	}
}

func TestPlayerBook(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := playerFixtures(t)

	for _, q := range []string{
		"insert into audiobook_files (id, location, dbaudiobook_id) values (11, '" + paths["song.mp3"] + "', 7), (12, '" + paths["song.wma"] + "', 7)",
		"insert into dbaudiobook_chapters (dbaudiobook_id, title, start_time_ms, position) values (7, 'One', 0, 1), (7, 'Two', 4000, 2), (7, 'Beyond', 60000, 3)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/?ids=11,12", nil)
	HandlePlayerBook(c)

	var out struct {
		Parts    []bookPart    `json:"parts"`
		Chapters []playChapter `json:"chapters"`
		Duration float64       `json:"duration"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err, w.Body.String())
	}

	// Two 3 s parts: the second starts where the first ends; the chapter
	// past the end is dropped.
	if len(out.Parts) != 2 || out.Parts[1].Start < 2.9 || out.Parts[1].Start > 3.2 || out.Duration < 5.8 ||
		len(out.Chapters) != 2 || out.Chapters[1].Title != "Two" || out.Chapters[1].Start != 4 {
		t.Errorf("book = %+v", out)
	}
}

func TestPlayerHLS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	paths := playerFixtures(t)

	g := config.GetSettingsGeneral()
	oldDir := g.CoverCacheDir
	g.CoverCacheDir = filepath.Join(t.TempDir(), "covers")
	t.Cleanup(func() { g.CoverCacheDir = oldDir })

	// Arguments: readrate before the input, keyframes per segment only when converting.
	args, _, err := hlsArgs(paths["mpeg4.avi"], playKinds["movie"], playProbe{VCodec: "mpeg4", Height: 720}, "480p", 5, "none", "/d", defaultSel)
	joined := strings.Join(args, " ")

	if err != nil || !strings.Contains(joined, "-readrate 2 -readrate_initial_burst 60 -i") || !strings.Contains(joined, "-force_key_frames") ||
		!strings.HasSuffix(joined, "-f hls -hls_time 4 -hls_playlist_type event -hls_segment_type mpegts -hls_segment_filename /d/seg%05d.ts /d/index.m3u8") ||
		strings.Contains(joined, "pipe:1") {
		t.Fatalf("hls args: %v %s", err, joined)
	}

	if args, _, _ := hlsArgs(paths["h264.mkv"], playKinds["movie"], playProbe{VCodec: "h264"}, "original", 0, "none", "/d", defaultSel); slices.Contains(args, "-force_key_frames") {
		t.Error("remux forces keyframes")
	}

	// A session: playlist, a segment at the profile's height, stop.
	w := playerRequest(t, HandlePlayerHLSStart, "movie", "2", "q=480p", nil)

	var start struct {
		Session string `json:"session"`
		URL     string `json:"url"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &start); err != nil || w.Code != http.StatusOK || start.Session == "" {
		t.Fatalf("start: %d %s", w.Code, w.Body.String())
	}

	file := func(name string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		c.Params = gin.Params{{Key: "sid", Value: start.Session}, {Key: "file", Value: name}}
		HandlePlayerHLSFile(c)

		return w
	}

	pl := file("index.m3u8")
	if pl.Code != http.StatusOK || !strings.Contains(pl.Body.String(), "seg00000.ts") || pl.Header().Get("Content-Type") != "application/vnd.apple.mpegurl" {
		t.Fatalf("playlist: %d %s", pl.Code, pl.Body.String())
	}

	if v, a, h, _ := probeBytes(t, file("seg00000.ts").Body.Bytes()); v != "h264" || a != "aac" || h != 480 {
		t.Errorf("segment: %s/%s %d", v, a, h)
	}

	if w := file("../../etc/passwd"); w.Code != http.StatusNotFound {
		t.Errorf("path outside the session: %d", w.Code)
	}

	file("stop")

	if _, err := os.Stat(filepath.Join(hlsRoot(), start.Session)); !os.IsNotExist(err) {
		t.Error("session folder kept after stop")
	}

	if w := file("index.m3u8"); w.Code != http.StatusNotFound {
		t.Errorf("stopped session still served: %d", w.Code)
	}

	// Audio isn't converted to HLS.
	if w := playerRequest(t, HandlePlayerHLSStart, "track", "4", "q=128", nil); w.Code != http.StatusBadRequest {
		t.Errorf("audio HLS: %d", w.Code)
	}
}

func TestPlayerKeyframeStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playerFixtures(t)

	// Keyframes every 5 s.
	src := filepath.Join(t.TempDir(), "gop.mkv")
	if out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=320x180:rate=25:duration=20", "-f", "lavfi", "-i", "sine=duration=20",
		"-c:v", "libx264", "-g", "125", "-keyint_min", "125", "-sc_threshold", "0", "-c:a", "ac3", src).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg: %v %s", err, out)
	}

	if k := keyframeStart(t.Context(), src, 7.3); k != 5 {
		t.Fatalf("keyframe before 7.3 = %v, want 5", k)
	}

	if err := database.ExecNErr("insert into movie_files (id, location) values (6, ?)", src); err != nil {
		t.Fatal(err)
	}

	// The repackaged stream starts at that keyframe, audio included: no
	// long first audio frame covering a gap.
	w := playerRequest(t, HandlePlayerStream, "movie", "6", "q=original&start=7.3", nil)
	if w.Code != http.StatusOK || w.Header().Get("X-Stream-Start") != "5.000" {
		t.Fatalf("stream: %d start %q", w.Code, w.Header().Get("X-Stream-Start"))
	}

	f := filepath.Join(t.TempDir(), "out.mp4")
	if err := os.WriteFile(f, w.Body.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "a:0", "-show_entries", "packet=duration_time",
		"-of", "csv=p=0", "-read_intervals", "%+#1", f).Output()
	if d, _ := strconv.ParseFloat(strings.TrimSpace(string(out)), 64); err != nil || d <= 0 || d > 0.1 {
		t.Fatalf("first audio frame lasts %q (%v)", out, err)
	}

	// A conversion starts where asked.
	if w := playerRequest(t, HandlePlayerStream, "movie", "6", "q=480p&start=7.3", nil); w.Header().Get("X-Stream-Start") != "7.300" {
		t.Fatalf("converted start %q", w.Header().Get("X-Stream-Start"))
	}
}

// TestClaimPlayerStream: a player's new stream stops its previous one and
// waits until that one has let go of its conversion slot.
func TestClaimPlayerStream(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	end1 := claimPlayerStream("p1", cancel1)

	var released time.Time

	go func() {
		<-ctx1.Done()
		time.Sleep(50 * time.Millisecond)

		released = time.Now()

		end1()
	}()

	_, cancel2 := context.WithCancel(context.Background())
	end2 := claimPlayerStream("p1", cancel2)

	if ctx1.Err() == nil || released.IsZero() || time.Since(released) > time.Second {
		t.Fatalf("previous stream: err=%v released=%v", ctx1.Err(), released)
	}

	// Another player is left alone; without an id nothing is tracked.
	ctx3, cancel3 := context.WithCancel(context.Background())
	end3 := claimPlayerStream("p2", cancel3)
	claimPlayerStream("", func() { t.Error("cancelled without id") })()

	if ctx3.Err() != nil {
		t.Fatal("other player cancelled")
	}

	end2()
	end3()

	if len(playerStreams) != 0 {
		t.Fatalf("claims left: %v", playerStreams)
	}
}

func TestAcquireTranscodeWait(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	limit := config.GetSettingsGeneral().PlayerMaxTranscodes
	for range limit {
		if !acquireTranscode() {
			t.Fatal("slot not free")
		}
	}

	// A slot freed while waiting is taken.
	time.AfterFunc(200*time.Millisecond, releaseTranscode)

	if !acquireTranscodeWait(context.Background(), 2*time.Second) {
		t.Fatal("freed slot not taken")
	}

	// None freed: gives up after the wait.
	if acquireTranscodeWait(context.Background(), 300*time.Millisecond) {
		t.Fatal("got a slot over the limit")
	}

	for range limit {
		releaseTranscode()
	}
}

// TestPlayMetaOf: what the player shows per kind, from the database and the
// file's tags (tags name a compilation's track and performer).
func TestPlayMetaOf(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	for _, q := range []string{
		"insert into dbartists (id, name) values (901, 'Various Artists')",
		"insert into dbalbums (id, title) values (901, 'Bravo Hits 22')",
		"insert into dbalbum_artists (dbalbum_id, dbartist_id, position) values (901, 901, 0)",
		"insert into album_files (id, location, dbalbum_id) values (901, '/m/01 - x.mp3', 901), (902, '/m/02 - Plain Name.mp3', 0)",
		"insert into dbmovies (id, title, year, genres) values (901, 'Monster und Mythen', 2018, 'Documentary,History')",
		"insert into movie_files (id, location, dbmovie_id) values (901, '/v/m.mkv', 901)",
		"insert into dbseries (id, seriename) values (901, 'Terra X')",
		"insert into dbserie_episodes (id, dbserie_id, identifier, title) values (901, 901, 'S2018E32', 'Der große Bluff')",
		"insert into serie_episode_files (id, location, dbserie_id, dbserie_episode_id) values (901, '/v/e.mkv', 901, 901)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	tags := map[string]string{"title": "Wannabe", "artist": "Spice Girls", "album": "tag album"}

	for _, tc := range []struct {
		kind, id, path string
		tags           map[string]string
		want           playMeta
	}{
		{"track", "901", "/m/01 - x.mp3", tags, playMeta{"Wannabe", "Spice Girls · Bravo Hits 22", "Spice Girls", "Bravo Hits 22", "/api/admin/library/cover/album/901"}},
		{"track", "902", "/m/02 - Plain Name.mp3", nil, playMeta{Title: "02 - Plain Name"}},
		{"movie", "901", "/v/m.mkv", nil, playMeta{"Monster und Mythen", "2018 · Documentary, History", "", "2018", "/api/admin/library/cover/movie/901"}},
		{"episode", "901", "/v/e.mkv", nil, playMeta{"Der große Bluff", "Terra X · S2018E32", "Terra X", "S2018E32", "/api/admin/library/cover/series/901"}},
	} {
		if got := playMetaOf(tc.kind, tc.id, tc.path, playProbe{Tags: tc.tags}); got != tc.want {
			t.Errorf("%s %s: got %+v, want %+v", tc.kind, tc.id, got, tc.want)
		}
	}
}

// TestStreamBufferBlocks: data crossing many pooled blocks comes out
// unchanged, written and read in odd sizes, with the limit below a block.
func TestStreamBufferBlocks(t *testing.T) {
	for _, limit := range []int{1000, streamBlockSize + 7, 3 * streamBlockSize} {
		want := make([]byte, 5*streamBlockSize+123)
		for i := range want {
			want[i] = byte(i * 7)
		}

		b := newStreamBuffer(limit)

		go func() {
			rest := want
			for i := 0; len(rest) > 0; i++ {
				n := min(len(rest), 1+(i*977)%70000)
				if _, err := b.Write(rest[:n]); err != nil {
					return
				}
				rest = rest[n:]
			}
			b.CloseWrite(nil)
		}()

		var got []byte

		p := make([]byte, 12345)
		for {
			n, err := b.Read(p)
			got = append(got, p[:n]...)

			if err != nil {
				break
			}
		}

		b.Close()

		if !bytes.Equal(got, want) {
			t.Fatalf("limit %d: got %d bytes, want %d (equal prefix %v)", limit, len(got), len(want), bytes.HasPrefix(want, got))
		}
	}
}

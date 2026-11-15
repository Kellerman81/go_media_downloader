package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

func TestParseSRT(t *testing.T) {
	cues := parseSRT("1\r\n00:00:01,500 --> 00:00:03,000\r\n<font color=\"red\">Hello</font> & <i>you</i>\r\n\r\n" +
		"2\n00:01:02.250 --> 00:01:04,000\n{\\an8}Two\nlines <x>\n\n3\n00:02:00,000 --> 00:02:01,000\n\n")

	want := []playCue{{1.5, 3, "Hello &amp; <i>you</i>"}, {62.25, 64, "Two\nlines"}}
	if !slices.Equal(cues, want) {
		t.Fatalf("cues = %+v, want %+v", cues, want)
	}
}

func TestLangCode(t *testing.T) {
	for in, want := range map[string]string{"eng": "en", "German": "de", "de": "de", "und": "", "": "", "xyz": "xyz"} {
		if got := langCode(in); got != want {
			t.Errorf("langCode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExternalSubs(t *testing.T) {
	write := func(p string) {
		t.Helper()

		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	// A movie folder: everything counts.
	dir := t.TempDir()
	for _, f := range []string{"Movie.mkv", "Movie.en.srt", "Movie.ger.forced.srt", "other.ass", "Movie.nfo", "Subs/Movie/2_English.srt"} {
		write(filepath.Join(dir, f))
	}

	got := externalSubs(filepath.Join(dir, "Movie.mkv"))

	type sub struct {
		name, label, lang string
		forced            bool
	}

	var subs []sub
	for _, s := range got {
		rel, _ := filepath.Rel(dir, s.Path)
		subs = append(subs, sub{rel, s.Label, s.Lang, s.Forced})
	}

	want := []sub{
		{"Movie.en.srt", "en", "en", false}, {"Movie.ger.forced.srt", "ger.forced", "de", true},
		{"Subs/Movie/2_English.srt", "2_English", "en", false}, {"other.ass", "other", "", false},
	}
	if !slices.Equal(subs, want) {
		t.Fatalf("subs = %+v, want %+v", subs, want)
	}

	// A season folder: only the episode's own files.
	dir = t.TempDir()
	for _, f := range []string{"S01E01.mkv", "S01E02.mkv", "S01E01.de.srt", "S01E02.de.srt", "notes.srt"} {
		write(filepath.Join(dir, f))
	}

	if got := externalSubs(filepath.Join(dir, "S01E01.mkv")); len(got) != 1 || filepath.Base(got[0].Path) != "S01E01.de.srt" {
		t.Fatalf("season folder: %+v", got)
	}
}

func TestFFmpegArgsTracks(t *testing.T) {
	// Loads a config with the default video profiles.
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	p := playProbe{VCodec: "h264", Height: 720}

	// Another audio track of an h264 file is a remux of that track.
	args, _, err := ffmpegArgs("/m.mkv", playKinds["movie"], p, "original", 0, "none", playSel{Audio: 1, Burn: -1})
	if err != nil || !slices.Contains(args, "0:a:1?") || !slices.Contains(args, "copy") {
		t.Fatalf("audio 1: %v %v", err, args)
	}

	// A burned-in subtitle converts, with the overlay ahead of the scaling.
	args, _, err = ffmpegArgs("/m.mkv", playKinds["movie"], p, "original", 0, "none", playSel{Burn: 2})
	if err != nil || slices.Contains(args, "copy") || slices.Contains(args, "-vf") {
		t.Fatalf("burn: %v %v", err, args)
	}

	i := slices.Index(args, "-filter_complex")
	if i < 0 || !strings.HasPrefix(args[i+1], "[0:v:0][0:s:2]overlay=") || !strings.HasSuffix(args[i+1], "[v]") || !slices.Contains(args, "[v]") {
		t.Fatalf("burn filter: %v", args)
	}
}

func TestPlayerTracks(t *testing.T) {
	gin.SetMode(gin.TestMode)
	playerFixtures(t)

	dir := t.TempDir()
	// The source subtitle stays out of the video's folder.
	srt := filepath.Join(t.TempDir(), "in.srt")

	if err := os.WriteFile(srt, []byte("1\n00:00:00,500 --> 00:00:02,000\nInside\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	video := filepath.Join(dir, "Film.mkv")

	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "testsrc=size=640x360:rate=25:duration=4", "-f", "lavfi", "-i", "sine=frequency=440:duration=4",
		"-f", "lavfi", "-i", "sine=frequency=880:duration=4", "-i", srt,
		"-map", "0", "-map", "1", "-map", "2", "-map", "3", "-map", "3", "-c:v", "libx264", "-c:a", "aac", "-c:s", "srt",
		"-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=ger", "-metadata:s:a:1", "title=Dub",
		"-disposition:a:1", "0", "-metadata:s:s:0", "language=eng", "-metadata:s:s:1", "language=fre", video).CombinedOutput()
	if err != nil {
		t.Skipf("ffmpeg can't create the file: %v %s", err, out)
	}

	// A Windows-1252 subtitle next to it ("Grüße").
	if err := os.WriteFile(filepath.Join(dir, "Film.de.srt"), []byte("1\n00:00:01,000 --> 00:00:03,000\nGr\xfc\xdfe\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := database.ExecNErr("insert into movie_files (id, location) values (5, ?)", video); err != nil {
		t.Fatal(err)
	}

	w := playerRequest(t, HandlePlayerInfo, "movie", "5", "", nil)

	var info struct {
		AudioTracks []playAudioInfo `json:"audioTracks"`
		Subtitles   []playSubInfo   `json:"subtitles"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil || w.Code != http.StatusOK {
		t.Fatalf("info: %d %s", w.Code, w.Body)
	}

	if len(info.AudioTracks) != 2 || info.AudioTracks[0].Lang != "en" || info.AudioTracks[1].Label != "German · AAC mono · Dub" {
		t.Fatalf("audio tracks: %+v", info.AudioTracks)
	}

	if len(info.Subtitles) != 3 || info.Subtitles[0].ID != "s0" || info.Subtitles[1].Lang != "fr" || info.Subtitles[2].ID != "x0" || info.Subtitles[2].Lang != "de" {
		t.Fatalf("subtitles: %+v", info.Subtitles)
	}

	// Converted subtitles are cached next to the cover cache.
	g := config.GetSettingsGeneral()
	oldDir := g.CoverCacheDir
	g.CoverCacheDir = filepath.Join(t.TempDir(), "covers")
	t.Cleanup(func() { g.CoverCacheDir = oldDir })

	for _, sub := range [][2]string{{"s0", "Inside"}, {"x0", "Grüße"}} {
		w := httptestSub(t, sub[0])

		var res struct{ Cues []playCue }
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || len(res.Cues) != 1 || res.Cues[0].Text != sub[1] {
			t.Fatalf("sub %s: %d %s", sub[0], w.Code, w.Body)
		}
	}

	// Both text streams of the file were converted in the first request.
	if cached, _ := filepath.Glob(filepath.Join(subCacheDir(), "*.json")); len(cached) != 3 {
		t.Fatalf("cached subtitles: %v", cached)
	}

	if w := httptestSub(t, "s5"); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown subtitle: %d", w.Code)
	}

	// The second audio track is streamed: the file is repackaged with it.
	w = playerRequest(t, HandlePlayerStream, "movie", "5", "q=original&a=1", nil)
	if vcodec, acodec, _, _ := probeBytes(t, w.Body.Bytes()); w.Code != http.StatusOK || vcodec != "h264" || acodec != "aac" {
		t.Fatalf("stream a=1: %d %s %s", w.Code, vcodec, acodec)
	}

	for _, q := range []string{"a=2", "burn=0"} {
		if w := playerRequest(t, HandlePlayerStream, "movie", "5", "q=original&"+q, nil); w.Code != http.StatusBadRequest {
			t.Fatalf("%s: %d", q, w.Code)
		}
	}
}

func httptestSub(t *testing.T, sid string) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Request = httptest.NewRequest(http.MethodGet, "/x", nil)
	c.Params = gin.Params{{Key: "kind", Value: "movie"}, {Key: "id", Value: "5"}, {Key: "sid", Value: sid}}
	HandlePlayerSub(c)

	return w
}

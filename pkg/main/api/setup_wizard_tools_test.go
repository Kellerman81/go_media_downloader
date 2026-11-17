package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

// wizardFakeTool writes a program that prints its "version".
func wizardFakeTool(t *testing.T, dir, name string) string {
	t.Helper()

	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\necho 'fake "+name+" 1.0'\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	return p
}

func wizardToolByKey(t *testing.T, key string) wizardTool {
	t.Helper()

	for _, tl := range wizardTools {
		if tl.key == key {
			return tl
		}
	}

	t.Fatalf("no tool %s", key)

	return wizardTool{}
}

func TestWizardFindTool(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell scripts")
	}

	bin, onPath := t.TempDir(), t.TempDir()
	ffprobe := wizardFakeTool(t, bin, "ffprobe")
	mediainfo := wizardFakeTool(t, bin, "mediainfo")
	ffmpeg := wizardFakeTool(t, bin, "ffmpeg")
	unrar := wizardFakeTool(t, onPath, "unrar")
	sevenza := wizardFakeTool(t, onPath, "7za")
	t.Setenv("PATH", onPath)

	// A folder setting, ffmpeg next to ffprobe, a name on the PATH, a full path.
	if got := wizardFindTool(wizardToolByKey(t, "ffprobe"), bin, ""); got != ffprobe {
		t.Errorf("ffprobe in folder: %q", got)
	}

	if got := wizardFindTool(wizardToolByKey(t, "ffmpeg"), "", bin); got != ffmpeg {
		t.Errorf("ffmpeg next to ffprobe: %q", got)
	}

	if got := wizardFindTool(wizardToolByKey(t, "unrar"), "", ""); got != unrar {
		t.Errorf("unrar on PATH: %q", got)
	}

	if got := wizardFindTool(wizardToolByKey(t, "7zip"), sevenza, ""); got != sevenza {
		t.Errorf("7-Zip by full path: %q", got)
	}

	if got := wizardFindTool(wizardToolByKey(t, "ffprobe"), "", ""); got != "" {
		t.Errorf("ffprobe not on PATH found: %q", got)
	}

	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	// The check: versions, 7za offered for a missing 7z, ffprobe given as
	// the program, MediaInfo given as a file where a folder is needed.
	results := wizardCheckTools(context.Background(), map[string]string{"ffprobe": ffprobe, "mediainfo": mediainfo})
	byKey := map[string]wizardToolResult{}

	for _, r := range results {
		byKey[r.tool.key] = r
	}

	if r := byKey["unrar"]; r.version != "fake unrar 1.0" {
		t.Errorf("unrar version: %+v", r)
	}

	if r := byKey["7zip"]; r.path != "" || r.hint != sevenza {
		t.Errorf("7za hint: %+v", r)
	}

	if r := byKey["ffprobe"]; r.path != ffprobe || r.version != "fake ffprobe 1.0" {
		t.Errorf("ffprobe as a file: %+v", r)
	}

	if r := byKey["mediainfo"]; r.path != "" || r.hint != bin {
		t.Errorf("file for a folder: %+v", r)
	}
}

func TestWizardToolNeeds(t *testing.T) {
	ffprobe := wizardToolByKey(t, "ffprobe")

	if need, _ := ffprobe.need(map[string]bool{"book": true}); need != "" {
		t.Errorf("books need ffprobe: %q", need)
	}

	if need, _ := ffprobe.need(map[string]bool{"video": true}); need != "required" {
		t.Errorf("video ffprobe: %q", need)
	}

	if need, _ := wizardToolByKey(t, "fpcalc").need(map[string]bool{"video": true}); need != "" {
		t.Errorf("fpcalc for video: %q", need)
	}

	// No groups chosen: everything counts.
	if kinds := wizardMediaKinds(wizardState{}); len(kinds) != 4 {
		t.Errorf("kinds without groups: %v", kinds)
	}
}

func TestWizardSaveToolsStep(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	form := url.Values{"wizard_tool_7zip": {" /usr/bin/7za "}, "wizard_tool_ffprobe": {"/opt/ff"},
		"wizard_player_hwaccel": {"cuda"}, "wizard_player_device": {"/dev/dri/renderD129"}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if msg := wizardSaveToolsStep(c); !strings.Contains(msg, "saved") {
		t.Fatalf("save: %s", msg)
	}

	g := config.GetSettingsGeneral()
	if g.SevenZipPath != "/usr/bin/7za" || g.FfprobePath != "/opt/ff" || g.PlayerHWAccel != "none" || g.PlayerHWDevice != "/dev/dri/renderD129" {
		t.Fatalf("saved: 7z %q ffprobe %q hw %q dev %q", g.SevenZipPath, g.FfprobePath, g.PlayerHWAccel, g.PlayerHWDevice)
	}
}

func TestWizardFindToolInitImdb(t *testing.T) {
	tool := wizardToolByKey(t, "initimdb")
	if tool.fixed == "" || tool.set != nil {
		t.Fatalf("init_imdb must be a fixed path without a setting: %+v", tool)
	}

	dir := t.TempDir()
	t.Chdir(dir)

	if got := wizardFindTool(tool, "", ""); got != "" {
		t.Fatalf("found a missing init_imdb: %q", got)
	}

	// A program that would leave a trace if it ran: the check must not run it.
	marker := filepath.Join(dir, "ran")
	if err := os.WriteFile(filepath.Join(dir, filepath.Base(tool.fixed)), []byte("#!/bin/sh\ntouch "+marker+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}

	results := wizardCheckTools(context.Background(), nil)
	for _, r := range results {
		if r.tool.key == "initimdb" && (r.path == "" || r.version != "") {
			t.Errorf("init_imdb result: path %q version %q", r.path, r.version)
		}
	}

	if _, err := os.Stat(marker); err == nil {
		t.Error("the check ran init_imdb")
	}

	if need, _ := tool.need(map[string]bool{"music": true}); need != "" {
		t.Errorf("music needs init_imdb: %q", need)
	}
}

// TestWizardToolFieldValue: a found tool fills its empty path field; the
// folder for ffprobe and MediaInfo.
func TestWizardToolFieldValue(t *testing.T) {
	for _, tc := range []struct {
		r    wizardToolResult
		want string
	}{
		{wizardToolResult{tool: wizardToolByKey(t, "ffprobe"), path: "/usr/bin/ffprobe"}, "/usr/bin"},
		{wizardToolResult{tool: wizardToolByKey(t, "mediainfo"), path: "/opt/mi/mediainfo"}, "/opt/mi"},
		{wizardToolResult{tool: wizardToolByKey(t, "unrar"), path: "/usr/bin/unrar"}, "/usr/bin/unrar"},
		{wizardToolResult{tool: wizardToolByKey(t, "7zip"), hint: "/usr/bin/7za"}, "/usr/bin/7za"},
		{wizardToolResult{tool: wizardToolByKey(t, "unrar"), setting: "/own/unrar", path: "/usr/bin/unrar"}, "/own/unrar"},
		{wizardToolResult{tool: wizardToolByKey(t, "tar")}, ""},
	} {
		if got := wizardToolFieldValue(tc.r); got != tc.want {
			t.Errorf("%s: %q, want %q", tc.r.tool.key, got, tc.want)
		}
	}
}

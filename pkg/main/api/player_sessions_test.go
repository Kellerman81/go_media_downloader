package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// callAs runs a handler for a POST form as the session's account.
func callAs(s *Session, h gin.HandlerFunc, form url.Values) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("session", s)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h(c)

	return w
}

// TestBandwidthLimiter: with a limit, sending takes as long as the rate
// says (less the burst); without one it doesn't wait.
func TestBandwidthLimiter(t *testing.T) {
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	g := config.GetSettingsGeneral()
	old := g.PlayerMaxBandwidth
	t.Cleanup(func() { g.PlayerMaxBandwidth = old })

	var l bandwidthLimiter

	g.PlayerMaxBandwidth = 0
	start := time.Now()

	for range 100 {
		if err := l.wait(context.Background(), bandwidthChunk); err != nil {
			t.Fatal(err)
		}
	}

	if d := time.Since(start); d > 50*time.Millisecond {
		t.Errorf("unlimited waited %v", d)
	}

	// 500 kB/s: the 6th 64 kB chunk may start 5 chunks (0.66 s) after the
	// first, less the 0.25 s burst.
	g.PlayerMaxBandwidth = 4
	start = time.Now()

	for range 400 << 10 / bandwidthChunk {
		if err := l.wait(context.Background(), bandwidthChunk); err != nil {
			t.Fatal(err)
		}
	}

	if d := time.Since(start); d < 350*time.Millisecond || d > 900*time.Millisecond {
		t.Errorf("400 kB at 4 Mbit/s took %v", d)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for range 20 {
		if err := l.wait(ctx, bandwidthChunk); err != nil {
			return // a cancelled stream stops waiting
		}
	}

	t.Error("a cancelled context kept waiting")
}

// TestPlayerProgressAndStop: progress feeds the history and Now Playing,
// the file info resumes there, and an admin's stop reaches the player with
// its message and keeps the file from streaming for a while.
func TestPlayerProgressAndStop(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	progress := func(s *Session, state, pos, id string) map[string]any {
		w := callAs(s, HandlePlayerProgress, url.Values{"sid": {"abcd1234"}, "kind": {"movie"}, "id": {id}, "state": {state},
			"pos": {pos}, "dur": {"1000"}, "title": {"Deutsch"}})

		var res map[string]any
		_ = json.Unmarshal(w.Body.Bytes(), &res)
		res["code"] = float64(w.Code)

		return res
	}

	if res := progress(anna, "start", "0", "11"); res["code"] != float64(http.StatusBadRequest) {
		t.Fatalf("progress for a file of another group: %v", res)
	}

	if res := progress(anna, "start", "0", "12"); res["stop"] != false {
		t.Fatalf("start: %v", res)
	}

	if res := progress(anna, "play", "420", "12"); res["stop"] != false {
		t.Fatalf("play: %v", res)
	}

	if got := database.PlayResume("anna", "movie", new(uint(12))); got != 420 {
		t.Errorf("resume at %v, want 420", got)
	}

	if list := nowPlayingList(); len(list) != 1 || list[0].User != "anna" || list[0].Position != 420 {
		t.Fatalf("now playing: %+v", list)
	}

	admin := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}
	if w := callAs(admin, HandleNowPlayingStop, url.Values{"sid": {"abcd1234"}, "message": {"Maintenance"}}); !strings.Contains(w.Body.String(), "Stopped") {
		t.Fatalf("stop: %s", w.Body)
	}

	if res := progress(anna, "play", "425", "12"); res["stop"] != true || res["message"] != "Maintenance" {
		t.Errorf("progress after the stop: %v", res)
	}

	if !streamBlocked("anna", "movie", "12") || streamBlocked("anna", "movie", "11") {
		t.Error("the stopped file isn't blocked (or another one is)")
	}

	if len(nowPlayingList()) != 0 {
		t.Error("the stopped player is still listed")
	}
}

// TestAccountSettings: an account saves its settings as a JSON object; the
// page gets them back escaped for a script element.
func TestAccountSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	post := func(body string) int {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		HandleAccountSettings(c)

		return c.Writer.Status()
	}

	if code := post(`["not", "an object"]`); code != http.StatusBadRequest {
		t.Errorf("array accepted: %d", code)
	}

	if code := post(`{"lp-vol":"0.3","note":"</script>"}`); code != http.StatusNoContent {
		t.Fatalf("save: %d", code)
	}

	if js := accountSettingsJS(adminAccess); !strings.Contains(js, `"lp-vol":"0.3"`) || strings.Contains(js, "</script>") {
		t.Errorf("settings for the page: %s", js)
	}
}

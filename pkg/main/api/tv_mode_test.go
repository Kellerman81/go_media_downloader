package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestTVMode: ?tv=1 switches TV mode (and with it app mode) on by cookie,
// ?tv=0 off; pages in TV mode carry the remote control script.
func TestTVMode(t *testing.T) {
	clientTestData(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/library?tv=1", nil)
	setAppMode(c)

	if !tvMode(c) || !appMode(c) || !strings.Contains(w.Header().Get("Set-Cookie"), tvCookie+"=1") {
		t.Fatalf("tv=1: tv %v, app %v, %s", tvMode(c), appMode(c), w.Header().Get("Set-Cookie"))
	}

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/library", nil)
	c.Request.AddCookie(&http.Cookie{Name: tvCookie, Value: "1"})

	if !tvMode(c) || !appMode(c) {
		t.Fatal("cookie: not in TV mode")
	}

	c, _ = gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/api/admin/library?app=0", nil)
	c.Request.AddCookie(&http.Cookie{Name: tvCookie, Value: "1"})

	if appMode(c) {
		t.Fatal("app=0 must win")
	}

	out := renderComponentToString(pageAs(&userAccess{Name: "admin", Role: "admin", App: true, TV: true}, "Library", false, false, false))
	if !strings.Contains(out, "gmd-tv") || !strings.Contains(out, "shown.bs.modal") {
		t.Fatal("page lacks the TV mode")
	}

	if out := renderComponentToString(pageAs(&userAccess{Name: "admin", Role: "admin"}, "Library", false, false, false)); strings.Contains(out, "gmd-tv") {
		t.Fatal("TV mode on a normal page")
	}
}

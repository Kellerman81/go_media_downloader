package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// useRememberSecret pins the server secret so tests never touch ./config.
func useRememberSecret(t *testing.T) {
	t.Helper()
	rememberSecretOnce.Do(func() {})

	old := rememberSecret
	rememberSecret = bytes.Repeat([]byte{7}, rememberSecretLen)

	t.Cleanup(func() { rememberSecret = old })
}

func TestParseRememberToken(t *testing.T) {
	secret := bytes.Repeat([]byte{1}, rememberSecretLen)
	key := rememberKey(secret, "pw")
	now := time.Now()
	valid := newRememberToken(key, DefaultUsername, now.Add(time.Hour))

	if user, ok := parseRememberToken(key, valid, now); !ok || user != DefaultUsername {
		t.Fatalf("valid token rejected: %q %v", user, ok)
	}

	parts := strings.Split(valid, ".")

	cases := map[string]struct {
		key   []byte
		token string
	}{
		"password changed": {rememberKey(secret, "other"), valid},
		"secret changed":   {rememberKey(bytes.Repeat([]byte{2}, rememberSecretLen), "pw"), valid},
		"expired":          {key, newRememberToken(key, DefaultUsername, now.Add(-time.Second))},
		"other user":       {key, newRememberToken(key, "bob", now.Add(time.Hour))},
		"expiry extended":  {key, parts[0] + ".9999999999." + parts[2]},
		"mac cut":          {key, parts[0] + "." + parts[1] + "." + parts[2][:10]},
		"no dots":          {key, "garbage"},
		"empty":            {key, ""},
	}
	for name, tc := range cases {
		if _, ok := parseRememberToken(tc.key, tc.token, now); ok {
			t.Errorf("%s: token accepted", name)
		}
	}
}

func TestLoadRememberSecret(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, rememberKeyFile)

	first := loadRememberSecret(path)
	if len(first) != rememberSecretLen {
		t.Fatalf("secret length %d", len(first))
	}

	if fi, err := os.Stat(path); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("key file not written 0600: %v %v", fi, err)
	}

	if !bytes.Equal(loadRememberSecret(path), first) {
		t.Fatal("secret changed between loads")
	}

	os.WriteFile(path, []byte("corrupt"), 0o600)

	if again := loadRememberSecret(path); bytes.Equal(again, first) || len(again) != rememberSecretLen {
		t.Fatal("corrupt key file not replaced")
	}

	if len(loadRememberSecret(filepath.Join(dir, "missing", rememberKeyFile))) != rememberSecretLen {
		t.Fatal("unwritable path must still yield an in-memory secret")
	}
}

func TestRememberMeSurvivesRestart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useRememberSecret(t)

	r := gin.New()
	g := r.Group("/api")
	g.POST("/login", handleLogin)
	g.GET("/logout", handleLogout)
	g.Use(protectAdminRoutes)
	g.GET("/admin/thing", func(c *gin.Context) { c.String(http.StatusOK, "done") })
	g.POST("/admin/thing", func(c *gin.Context) { c.String(http.StatusOK, "done") })

	login := func(remember bool) []*http.Cookie {
		form := url.Values{"username": {DefaultUsername}, "password": {DefaultPassword}}
		if remember {
			form.Set("remember-me", "remember-me")
		}

		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusFound || w.Header().Get("Location") != "/api/admin" {
			t.Fatalf("login failed: %d %s", w.Code, w.Header().Get("Location"))
		}

		return w.Result().Cookies()
	}

	do := func(method string, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/admin/thing", nil)
		for _, ck := range cookies {
			if ck.MaxAge >= 0 {
				req.AddCookie(ck)
			}
		}

		if csrf != "" {
			req.Header.Set("X-CSRF-Token", csrf)
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		return w
	}

	cookieValue := func(cookies []*http.Cookie, name string) string {
		for _, ck := range cookies {
			if ck.Name == name {
				return ck.Value
			}
		}

		return ""
	}

	restart := func() {
		sessionStore.mutex.Lock()
		sessionStore.sessions = make(map[string]*Session)
		sessionStore.mutex.Unlock()
	}

	remembered := login(true)
	csrf := cookieValue(remembered, csrfCookieName)

	if cookieValue(remembered, rememberCookieName) == "" {
		t.Fatal("no remember-me cookie set")
	}

	restart()

	w := do(http.MethodGet, remembered, "")
	if w.Code != http.StatusOK {
		t.Fatalf("remembered GET after restart: %d", w.Code)
	}

	if id := cookieValue(w.Result().Cookies(), "session_id"); id == "" ||
		id == cookieValue(remembered, "session_id") {
		t.Fatalf("no fresh session cookie after restore: %q", id)
	}

	restart()

	if w := do(http.MethodPost, remembered, csrf); w.Code != http.StatusOK {
		t.Fatalf("pre-restart CSRF token rejected after restore: %d", w.Code)
	}

	plain := login(false)
	restart()

	if w := do(http.MethodGet, plain, ""); w.Code != http.StatusFound {
		t.Fatalf("session without remember-me survived restart: %d", w.Code)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/logout", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	for _, ck := range w.Result().Cookies() {
		if ck.Name == rememberCookieName && ck.MaxAge >= 0 {
			t.Fatal("logout did not clear the remember-me cookie")
		}
	}
}

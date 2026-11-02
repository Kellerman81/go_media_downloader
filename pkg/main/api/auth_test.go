package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestGetSessionExpiredConcurrent(t *testing.T) {
	ss := &SessionStore{sessions: make(map[string]*Session)}
	for i := range 50 {
		id := "s" + string(rune('a'+i%26)) + string(rune('a'+i/26))
		ss.sessions[id] = &Session{ID: id, ExpiresAt: time.Now().Add(-time.Minute)}
	}

	ss.sessions["live"] = &Session{ID: "live", ExpiresAt: time.Now().Add(time.Hour)}

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			for id := range map[string]struct{}{"saa": {}, "sba": {}, "live": {}, "missing": {}} {
				ss.getSession(id)
			}
		})
	}

	wg.Wait()

	if _, ok := ss.getSession("live"); !ok {
		t.Fatal("live session lost")
	}

	if _, ok := ss.getSession("saa"); ok {
		t.Fatal("expired session returned")
	}

	if _, ok := ss.sessions["saa"]; ok {
		t.Fatal("expired session not deleted")
	}
}

func TestProtectAdminRoutesCSRF(t *testing.T) {
	gin.SetMode(gin.TestMode)

	sess := sessionStore.createSession("admin")
	defer sessionStore.deleteSession(sess.ID)

	calls := 0
	r := gin.New()
	g := r.Group("/api")
	g.Use(protectAdminRoutes)
	g.POST("/admin/thing", func(c *gin.Context) {
		calls++
		c.String(http.StatusOK, "done")
	})
	g.GET("/admin/thing", func(c *gin.Context) {
		calls++
		c.String(http.StatusOK, "done")
	})

	do := func(method, token, form string, withSession bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/api/admin/thing", strings.NewReader(form))
		if form != "" {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}

		if token != "" {
			req.Header.Set("X-CSRF-Token", token)
		}

		if withSession {
			req.AddCookie(&http.Cookie{Name: "session_id", Value: sess.ID})
		}

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		return w
	}

	tests := []struct {
		name        string
		method      string
		token       string
		form        string
		session     bool
		wantStatus  int
		wantHandler bool
	}{
		{"post no token", http.MethodPost, "", "", true, http.StatusForbidden, false},
		{"post wrong token", http.MethodPost, "nope", "", true, http.StatusForbidden, false},
		{"post header token", http.MethodPost, sess.CSRFToken, "", true, http.StatusOK, true},
		{"post form token", http.MethodPost, "", "csrf_token=" + url.QueryEscape(sess.CSRFToken), true, http.StatusOK, true},
		{"post no session", http.MethodPost, sess.CSRFToken, "", false, http.StatusFound, false},
		{"get with session", http.MethodGet, "", "", true, http.StatusOK, true},
		{"get no session", http.MethodGet, "", "", false, http.StatusFound, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls = 0
			w := do(tt.method, tt.token, tt.form, tt.session)

			if w.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %q)", w.Code, tt.wantStatus, w.Body.String())
			}

			if (calls > 0) != tt.wantHandler {
				t.Errorf("handler calls = %d, wantHandler %v", calls, tt.wantHandler)
			}

			if calls > 1 {
				t.Errorf("handler ran %d times", calls)
			}
		})
	}
}

func TestLoginRateLimiter(t *testing.T) {
	l := &loginRateLimiter{attempts: make(map[string]*loginAttempt)}
	now := time.Now()

	for range loginFreeAttempts - 1 {
		l.fail("1.2.3.4", now)
	}

	if l.blockedFor("1.2.3.4", now) != 0 {
		t.Fatal("blocked before limit")
	}

	l.fail("1.2.3.4", now)

	if got := l.blockedFor("1.2.3.4", now); got != loginBaseLockout {
		t.Fatalf("lockout = %v, want %v", got, loginBaseLockout)
	}

	l.fail("1.2.3.4", now)

	if got := l.blockedFor("1.2.3.4", now); got != 2*loginBaseLockout {
		t.Fatalf("lockout = %v, want %v", got, 2*loginBaseLockout)
	}

	for range 40 {
		l.fail("1.2.3.4", now)
	}

	if got := l.blockedFor("1.2.3.4", now); got != loginMaxLockout {
		t.Fatalf("lockout = %v, want cap %v", got, loginMaxLockout)
	}

	if l.blockedFor("5.6.7.8", now) != 0 {
		t.Fatal("other IP blocked")
	}

	l.reset("1.2.3.4")

	if l.blockedFor("1.2.3.4", now) != 0 {
		t.Fatal("still blocked after reset")
	}
}

package api

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
)

// "Remember me" is a stateless signed cookie, so it survives restarts without
// persisting sessions. Only one random server secret is stored on disk. The
// signing key also mixes in the account's password (the WebAPIKey for the
// built-in admin, the password hash for other accounts), so a new password
// revokes that account's remembered logins and deleting the key file all.
const (
	rememberCookieName = "remember_me"
	RememberDuration   = 30 * 24 * time.Hour
	rememberKeyFile    = "remember.key"
	rememberSecretLen  = 32
)

var (
	rememberSecretOnce sync.Once
	rememberSecret     []byte
)

func rememberServerSecret() []byte {
	rememberSecretOnce.Do(func() {
		rememberSecret = loadRememberSecret(
			filepath.Join(filepath.Dir(config.Configfile), rememberKeyFile),
		)
	})

	return rememberSecret
}

// loadRememberSecret reads the hex secret from path, creating it when missing
// or unreadable. If it can't be written the secret only lives in memory and
// remembered logins end at the next restart.
func loadRememberSecret(path string) []byte {
	if b, err := os.ReadFile(path); err == nil {
		key, err := hex.DecodeString(strings.TrimSpace(string(b)))
		if err == nil && len(key) == rememberSecretLen {
			return key
		}
	}

	key := make([]byte, rememberSecretLen)
	rand.Read(key)

	if err := os.WriteFile(path, []byte(hex.EncodeToString(key)), 0o600); err != nil {
		logger.Logtype("warn", 0).
			Err(err).
			Str("file", path).
			Msg("remember-me key not saved, remembered logins end at restart")
	}

	return key
}

func webPassword() string {
	if p := config.GetSettingsGeneral().WebAPIKey; p != "" {
		return p
	}

	return DefaultPassword
}

func rememberKey(secret []byte, password string) []byte {
	return rememberMAC(secret, "remember-me", password)
}

func currentRememberKey() []byte {
	return rememberKey(rememberServerSecret(), webPassword())
}

// rememberKeyFor is the remember-me key of an account; false for an unknown
// one.
func rememberKeyFor(user string) ([]byte, bool) {
	if user == DefaultUsername {
		return currentRememberKey(), true
	}

	u, err := database.GetUserByName(&user)
	if err != nil || u.ID == 0 {
		return nil, false
	}

	return rememberKey(rememberServerSecret(), u.PasswordHash), true
}

// rememberTokenUser is the user a token names, before its signature is
// checked.
func rememberTokenUser(token string) string {
	u, _, _ := strings.Cut(token, ".")

	user, err := base64.RawURLEncoding.DecodeString(u)
	if err != nil {
		return ""
	}

	return string(user)
}

func rememberMAC(key []byte, parts ...string) []byte {
	m := hmac.New(sha256.New, key)
	for _, p := range parts {
		m.Write([]byte(p))
		m.Write([]byte{0})
	}

	return m.Sum(nil)
}

// newRememberToken returns "base64(user).expiryUnix.hexMAC".
func newRememberToken(key []byte, user string, expires time.Time) string {
	u := base64.RawURLEncoding.EncodeToString([]byte(user))
	exp := strconv.FormatInt(expires.Unix(), 10)

	return u + "." + exp + "." + hex.EncodeToString(rememberMAC(key, "token", u, exp))
}

// parseRememberToken returns the user of a token that is correctly signed
// with key (the key of that user) and not expired at now.
func parseRememberToken(key []byte, token string, now time.Time) (string, bool) {
	u, rest, ok := strings.Cut(token, ".")
	if !ok {
		return "", false
	}

	exp, mac, ok := strings.Cut(rest, ".")
	if !ok {
		return "", false
	}

	got, err := hex.DecodeString(mac)
	if err != nil || !hmac.Equal(got, rememberMAC(key, "token", u, exp)) {
		return "", false
	}

	unix, err := strconv.ParseInt(exp, 10, 64)
	if err != nil || !now.Before(time.Unix(unix, 0)) {
		return "", false
	}

	user, err := base64.RawURLEncoding.DecodeString(u)
	if err != nil {
		return "", false
	}

	return string(user), true
}

// rememberCSRFToken derives the CSRF token of sessions started from token, so
// pages opened before a restart keep posting a valid token afterwards.
func rememberCSRFToken(key []byte, token string) string {
	return hex.EncodeToString(rememberMAC(key, "csrf", token)[:CSRFTokenLength])
}

// restoreRememberedSession starts a new session from a valid remember-me
// cookie and sets its session cookie. The caller sets the CSRF cookie.
func restoreRememberedSession(c *gin.Context) (*Session, bool) {
	token, err := c.Cookie(rememberCookieName)
	if err != nil || token == "" {
		return nil, false
	}

	user := rememberTokenUser(token)

	key, ok := rememberKeyFor(user)
	if !ok {
		return nil, false
	}

	if _, ok := parseRememberToken(key, token, time.Now()); !ok {
		return nil, false
	}

	acc := adminAccess
	if user != DefaultUsername {
		u, err := database.GetUserByName(&user)
		if err != nil || u.ID == 0 {
			return nil, false
		}

		acc = accessOfUser(u)
	}

	session := sessionStore.createSession(acc, rememberCSRFToken(key, token))
	setSessionCookie(c, session.ID, int(SessionDuration.Seconds()))

	return session, true
}

func setSessionCookie(c *gin.Context, id string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("session_id", id, maxAge, "/", "", requestIsHTTPS(c), true)
}

func setRememberCookie(c *gin.Context, token string, maxAge int) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie(rememberCookieName, token, maxAge, "/", "", requestIsHTTPS(c), true)
}

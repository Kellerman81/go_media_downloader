package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// SessionStore holds active sessions
type SessionStore struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
}

// Session represents a user session
type Session struct {
	ID        string
	CreatedAt time.Time
	ExpiresAt time.Time
	UserID    string
	CSRFToken string
}

var sessionStore = &SessionStore{
	sessions: make(map[string]*Session),
}

// generateSessionID creates a cryptographically secure session ID
func generateSessionID() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// generateCSRFToken creates a CSRF token for the session
func generateCSRFToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// createSession creates a new session for the user
func (ss *SessionStore) createSession(userID string) *Session {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	sessionID := generateSessionID()
	csrfToken := generateCSRFToken()

	session := &Session{
		ID:        sessionID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(24 * time.Hour), // 24 hour session
		UserID:    userID,
		CSRFToken: csrfToken,
	}

	ss.sessions[sessionID] = session
	return session
}

// getSession retrieves a session by ID
func (ss *SessionStore) getSession(sessionID string) (*Session, bool) {
	ss.mutex.RLock()
	defer ss.mutex.RUnlock()

	session, exists := ss.sessions[sessionID]
	if !exists || time.Now().After(session.ExpiresAt) {
		if exists {
			delete(ss.sessions, sessionID)
		}
		return nil, false
	}

	return session, true
}

// deleteSession removes a session
func (ss *SessionStore) deleteSession(sessionID string) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()
	delete(ss.sessions, sessionID)
}

// cleanupExpiredSessions removes expired sessions
func (ss *SessionStore) cleanupExpiredSessions() {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	now := time.Now()
	for id, session := range ss.sessions {
		if now.After(session.ExpiresAt) {
			delete(ss.sessions, id)
		}
	}
}

// authenticateUser checks if the provided credentials are valid
func authenticateUser(username, password string) bool {
	settings := config.GetSettingsGeneral()

	// For now, use a simple admin/password check
	// In production, this should be stored securely in database
	expectedUsername := "admin"
	expectedPassword := settings.WebAPIKey // Use API key as default password

	if expectedPassword == "" {
		expectedPassword = "admin" // Fallback default
	}

	return username == expectedUsername && password == expectedPassword
}

// requireAuth middleware checks for valid session
func requireAuth(c *gin.Context) {
	// Check for session cookie
	sessionCookie, err := c.Cookie("session_id")
	if err != nil {
		redirectToLogin(c)
		return
	}

	// Validate session
	session, exists := sessionStore.getSession(sessionCookie)
	if !exists {
		redirectToLogin(c)
		return
	}

	// Store session in context for later use
	c.Set("session", session)
	c.Set("csrf_token", session.CSRFToken)
	c.Next()
}

// requireCSRF middleware checks CSRF token for state-changing operations
func requireCSRF(c *gin.Context) {
	if c.Request.Method == "GET" || c.Request.Method == "HEAD" {
		c.Next()
		return
	}

	session, exists := c.Get("session")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "No session found"})
		c.Abort()
		return
	}

	sessionObj := session.(*Session)

	// Check CSRF token from header or form
	csrfToken := c.GetHeader("X-CSRF-Token")
	if csrfToken == "" {
		csrfToken = c.PostForm("csrf_token")
	}

	if csrfToken != sessionObj.CSRFToken {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid CSRF token"})
		c.Abort()
		return
	}

	c.Next()
}

// redirectToLogin redirects user to login page
func redirectToLogin(c *gin.Context) {
	// Clear any existing session cookie
	c.SetCookie("session_id", "", -1, "/", "", false, true)

	// Check if this is a web page request that should redirect to login
	// Admin and manage pages should redirect, not return JSON
	if strings.HasPrefix(c.Request.URL.Path, "/api/admin") ||
		strings.HasPrefix(c.Request.URL.Path, "/api/manage") ||
		!strings.HasPrefix(c.Request.URL.Path, "/api/") {
		// For web page requests, redirect to login page
		c.Redirect(http.StatusFound, "/api/login")
		c.Abort()
		return
	}

	// For other API requests (JSON endpoints), return JSON error
	c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
	c.Abort()
}

// loginPage renders the login form
func loginPage(c *gin.Context) {
	// Check if already logged in
	if sessionCookie, err := c.Cookie("session_id"); err == nil {
		if _, exists := sessionStore.getSession(sessionCookie); exists {
			c.Redirect(http.StatusFound, "/api/admin")
			return
		}
	}

	errorMsg := c.Query("error")

	page := html.Doctype(
		html.HTML(
			html.Lang("en"),
			html.Head(
				html.Meta(html.Charset("utf-8")),
				html.Meta(html.Name("viewport"), html.Content("width=device-width, initial-scale=1")),
				html.Title("Admin Login"),
				html.Link(html.Href("https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css"), html.Rel("stylesheet")),
			),
			html.Body(
				html.Div(html.Class("container mt-5"),
					html.Div(html.Class("row justify-content-center"),
						html.Div(html.Class("col-md-6 col-lg-4"),
							html.Div(html.Class("card"),
								html.Div(html.Class("card-header"),
									html.H4(html.Class("mb-0"), gomponents.Text("Admin Login")),
								),
								html.Div(html.Class("card-body"),
									gomponents.If(errorMsg != "",
										html.Div(html.Class("alert alert-danger"),
											gomponents.Text(errorMsg),
										),
									),
									html.Form(html.Method("POST"), html.Action("/api/login"),
										html.Div(html.Class("mb-3"),
											html.Label(html.For("username"), html.Class("form-label"), gomponents.Text("Username")),
											html.Input(html.Type("text"), html.Class("form-control"), html.ID("username"), html.Name("username"), html.Required()),
										),
										html.Div(html.Class("mb-3"),
											html.Label(html.For("password"), html.Class("form-label"), gomponents.Text("Password")),
											html.Input(html.Type("password"), html.Class("form-control"), html.ID("password"), html.Name("password"), html.Required()),
										),
										html.Button(html.Type("submit"), html.Class("btn btn-primary w-100"),
											gomponents.Text("Login"),
										),
									),
								),
							),
						),
					),
				),
			),
		),
	)

	c.Header("Content-Type", "text/html; charset=utf-8")
	var buf strings.Builder
	page.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// handleLogin processes login form submission
func handleLogin(c *gin.Context) {
	username := c.PostForm("username")
	password := c.PostForm("password")

	if !authenticateUser(username, password) {
		c.Redirect(http.StatusFound, "/api/login?error=Invalid+credentials")
		return
	}

	// Create session
	session := sessionStore.createSession(username)

	// Set secure session cookie
	c.SetCookie("session_id", session.ID, int(24*time.Hour.Seconds()), "/", "", false, true)

	// Redirect to admin panel
	c.Redirect(http.StatusFound, "/api/admin")
}

// handleLogout clears session and redirects to login
func handleLogout(c *gin.Context) {
	if sessionCookie, err := c.Cookie("session_id"); err == nil {
		sessionStore.deleteSession(sessionCookie)
	}

	c.SetCookie("session_id", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/api/login")
}

// protectAdminRoutes middleware automatically protects admin and manage routes
func protectAdminRoutes(c *gin.Context) {
	path := c.Request.URL.Path

	// Check if this is an admin or manage route (but not login/logout)
	if (strings.HasPrefix(path, "/api/admin") || strings.HasPrefix(path, "/api/manage")) &&
		path != "/api/login" && path != "/api/logout" {

		// Check for session cookie
		sessionCookie, err := c.Cookie("session_id")
		if err != nil {
			redirectToLogin(c)
			return
		}

		// Validate session
		session, exists := sessionStore.getSession(sessionCookie)
		if !exists {
			redirectToLogin(c)
			return
		}

		// Store session in context for later use
		c.Set("session", session)
		c.Set("csrf_token", session.CSRFToken)

		// Check CSRF for state-changing operations
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			csrfToken := c.GetHeader("X-CSRF-Token")
			if csrfToken == "" {
				csrfToken = c.PostForm("csrf_token")
			}

			if csrfToken != session.CSRFToken {
				c.JSON(http.StatusForbidden, gin.H{"error": "Invalid CSRF token"})
				c.Abort()
				return
			}
		}
	}

	c.Next()
}

// handleRootRedirect handles root path requests and redirects to admin or login
func handleRootRedirect(c *gin.Context) {
	// Check if user is already authenticated
	if sessionCookie, err := c.Cookie("session_id"); err == nil {
		if _, exists := sessionStore.getSession(sessionCookie); exists {
			// User is authenticated, redirect to admin panel
			c.Redirect(http.StatusFound, "/api/admin")
			return
		}
	}

	// User is not authenticated, redirect to login page
	c.Redirect(http.StatusFound, "/api/login")
}

// startSessionCleanup starts a goroutine to periodically clean expired sessions
func startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			sessionStore.cleanupExpiredSessions()
		}
	}()
}

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
	return generateSecureToken(SessionIDLength)
}

// generateCSRFToken creates a CSRF token for the session
func generateCSRFToken() string {
	return generateSecureToken(CSRFTokenLength)
}

// generateSecureToken creates a cryptographically secure token of specified length
func generateSecureToken(length int) string {
	bytes := make([]byte, length)
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
		ExpiresAt: time.Now().Add(SessionDuration),
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

// Default authentication constants
const (
	DefaultUsername = "admin"
	DefaultPassword = "admin"
	SessionDuration = 24 * time.Hour
	CSRFTokenLength = 16
	SessionIDLength = 32
)

// authenticateUser checks if the provided credentials are valid
func authenticateUser(username, password string) bool {
	settings := config.GetSettingsGeneral()

	// Use API key as password, fallback to default
	expectedPassword := settings.WebAPIKey
	if expectedPassword == "" {
		expectedPassword = DefaultPassword
	}

	return username == DefaultUsername && password == expectedPassword
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
		sendUnauthorized(c, "No session found")
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
		sendForbidden(c, "Invalid CSRF token")
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
	sendUnauthorized(c, "Authentication required")
}

// loginPage renders the enhanced login form
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
				html.Title("Go Media Downloader - Admin Login"),
				html.Link(html.Href("https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css"), html.Rel("stylesheet")),
				html.Link(html.Href("https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css"), html.Rel("stylesheet")),
				html.StyleEl(gomponents.Raw(`
					:root {
						--primary-gradient: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
						--success-gradient: linear-gradient(135deg, #11998e 0%, #38ef7d 100%);
						--glass-bg: rgba(255, 255, 255, 0.25);
						--glass-bg-dark: rgba(255, 255, 255, 0.1);
					}
					
					* {
						margin: 0;
						padding: 0;
						box-sizing: border-box;
					}
					
					body {
						font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
						background: linear-gradient(135deg, #667eea 0%, #764ba2 50%, #f093fb 100%);
						min-height: 100vh;
						display: flex;
						align-items: center;
						justify-content: center;
						position: relative;
						overflow: hidden;
					}
					
					/* Animated background elements */
					body::before {
						content: '';
						position: absolute;
						top: -50%;
						left: -50%;
						width: 200%;
						height: 200%;
						background: url('data:image/svg+xml,<svg width="60" height="60" viewBox="0 0 60 60" xmlns="http://www.w3.org/2000/svg"><g fill="none" fill-rule="evenodd"><g fill="%23ffffff" fill-opacity="0.05"><circle cx="30" cy="30" r="4"/></g></svg>');
						animation: drift 20s infinite linear;
						z-index: 1;
					}
					
					@keyframes drift {
						0% { transform: rotate(0deg) translate(-50%, -50%); }
						100% { transform: rotate(360deg) translate(-50%, -50%); }
					}
					
					.login-container {
						position: relative;
						z-index: 10;
						width: 100%;
						max-width: 400px;
						padding: 2rem;
					}
					
					.login-card {
						background: var(--glass-bg);
						backdrop-filter: blur(20px);
						border: 1px solid rgba(255, 255, 255, 0.2);
						border-radius: 20px;
						box-shadow: 0 25px 50px rgba(0, 0, 0, 0.2);
						padding: 2.5rem;
						animation: slideUp 0.8s cubic-bezier(0.25, 0.46, 0.45, 0.94);
						transition: all 0.3s ease;
					}
					
					.login-card:hover {
						transform: translateY(-5px);
						box-shadow: 0 30px 60px rgba(0, 0, 0, 0.3);
					}
					
					@keyframes slideUp {
						from {
							opacity: 0;
							transform: translateY(50px);
						}
						to {
							opacity: 1;
							transform: translateY(0);
						}
					}
					
					.login-header {
						text-align: center;
						margin-bottom: 2rem;
					}
					
					.login-logo {
						width: 80px;
						height: 80px;
						background: var(--primary-gradient);
						border-radius: 50%;
						display: flex;
						align-items: center;
						justify-content: center;
						margin: 0 auto 1rem;
						box-shadow: 0 10px 30px rgba(102, 126, 234, 0.4);
						animation: pulse 2s infinite;
					}
					
					@keyframes pulse {
						0%, 100% { transform: scale(1); }
						50% { transform: scale(1.05); }
					}
					
					.login-logo i {
						font-size: 2rem;
						color: white;
					}
					
					.login-title {
						color: white;
						font-size: 1.5rem;
						font-weight: 700;
						margin-bottom: 0.5rem;
						text-shadow: 0 2px 10px rgba(0, 0, 0, 0.3);
					}
					
					.login-subtitle {
						color: rgba(255, 255, 255, 0.8);
						font-size: 0.9rem;
						font-weight: 400;
					}
					
					.form-group {
						position: relative;
						margin-bottom: 1.5rem;
					}
					
					.form-control {
						background: var(--glass-bg-dark);
						border: 1px solid rgba(255, 255, 255, 0.3);
						border-radius: 12px;
						color: white;
						font-size: 1rem;
						padding: 1rem 1rem 1rem 3rem;
						width: 100%;
						transition: all 0.3s ease;
						backdrop-filter: blur(10px);
					}
					
					.form-control:focus {
						outline: none;
						border-color: rgba(255, 255, 255, 0.6);
						background: rgba(255, 255, 255, 0.15);
						box-shadow: 0 0 0 3px rgba(102, 126, 234, 0.3);
						transform: translateY(-2px);
					}
					
					.form-control::placeholder {
						color: rgba(255, 255, 255, 0.6);
					}
					
					.form-icon {
						position: absolute;
						left: 1rem;
						top: 50%;
						transform: translateY(-50%);
						color: rgba(255, 255, 255, 0.7);
						font-size: 1.1rem;
						transition: all 0.3s ease;
					}
					
					.form-group:focus-within .form-icon {
						color: white;
						transform: translateY(-50%) scale(1.1);
					}
					
					.btn-login {
						background: var(--success-gradient);
						border: none;
						border-radius: 12px;
						color: white;
						font-size: 1.1rem;
						font-weight: 600;
						padding: 1rem 2rem;
						width: 100%;
						cursor: pointer;
						transition: all 0.3s ease;
						margin-top: 0.5rem;
						box-shadow: 0 10px 30px rgba(17, 153, 142, 0.3);
						position: relative;
						overflow: hidden;
					}
					
					.btn-login::before {
						content: '';
						position: absolute;
						top: 0;
						left: -100%;
						width: 100%;
						height: 100%;
						background: linear-gradient(90deg, transparent, rgba(255,255,255,0.3), transparent);
						transition: left 0.5s;
					}
					
					.btn-login:hover::before {
						left: 100%;
					}
					
					.btn-login:hover {
						transform: translateY(-3px);
						box-shadow: 0 15px 40px rgba(17, 153, 142, 0.4);
					}
					
					.btn-login:active {
						transform: translateY(-1px);
					}
					
					.alert {
						background: rgba(220, 53, 69, 0.2);
						border: 1px solid rgba(220, 53, 69, 0.3);
						color: white;
						border-radius: 12px;
						padding: 1rem;
						margin-bottom: 1.5rem;
						backdrop-filter: blur(10px);
						animation: shake 0.5s ease-in-out;
					}
					
					@keyframes shake {
						0%, 100% { transform: translateX(0); }
						25% { transform: translateX(-5px); }
						75% { transform: translateX(5px); }
					}
					
					.loading {
						pointer-events: none;
						opacity: 0.7;
					}
					
					.loading .btn-login {
						background: #6c757d;
					}
					
					@media (max-width: 768px) {
						.login-container {
							padding: 1rem;
						}
						
						.login-card {
							padding: 2rem;
						}
					}
					
					/* Floating particles animation */
					.particle {
						position: absolute;
						background: rgba(255, 255, 255, 0.1);
						border-radius: 50%;
						animation: float 6s ease-in-out infinite;
					}
					
					@keyframes float {
						0%, 100% { 
							transform: translateY(0px) translateX(0px);
							opacity: 0.7;
						}
						50% { 
							transform: translateY(-20px) translateX(10px);
							opacity: 0.3;
						}
					}
				`)),
			),
			html.Body(
				// Floating particles
				html.Div(html.Class("particle"), html.Style("width: 4px; height: 4px; top: 20%; left: 10%; animation-delay: 0s;")),
				html.Div(html.Class("particle"), html.Style("width: 6px; height: 6px; top: 60%; left: 80%; animation-delay: 2s;")),
				html.Div(html.Class("particle"), html.Style("width: 3px; height: 3px; top: 40%; left: 70%; animation-delay: 4s;")),
				html.Div(html.Class("particle"), html.Style("width: 5px; height: 5px; top: 80%; left: 20%; animation-delay: 1s;")),
				html.Div(html.Class("particle"), html.Style("width: 4px; height: 4px; top: 30%; left: 90%; animation-delay: 3s;")),

				html.Div(html.Class("login-container"),
					html.Div(html.Class("login-card"),
						html.Div(html.Class("login-header"),
							html.Div(html.Class("login-logo"),
								html.I(html.Class("fas fa-download")),
							),
							html.H1(html.Class("login-title"), gomponents.Text("Go Media")),
							html.P(html.Class("login-subtitle"), gomponents.Text("Downloader Admin Panel")),
						),

						gomponents.If(errorMsg != "",
							html.Div(html.Class("alert"),
								html.I(html.Class("fas fa-exclamation-triangle me-2")),
								gomponents.Text(errorMsg),
							),
						),

						html.Form(
							html.Method("POST"),
							html.Action("/api/login"),
							html.ID("loginForm"),

							html.Div(html.Class("form-group"),
								html.I(html.Class("form-icon fas fa-user")),
								html.Input(
									html.Type("text"),
									html.Name("username"),
									html.ID("username"),
									html.Class("form-control"),
									html.Placeholder("Username"),
									html.Required(),
									gomponents.Attr("autocomplete", "username"),
								),
							),

							html.Div(html.Class("form-group"),
								html.I(html.Class("form-icon fas fa-lock")),
								html.Input(
									html.Type("password"),
									html.Name("password"),
									html.ID("password"),
									html.Class("form-control"),
									html.Placeholder("Password"),
									html.Required(),
									gomponents.Attr("autocomplete", "current-password"),
								),
							),

							html.Button(
								html.Type("submit"),
								html.Class("btn-login"),
								html.ID("loginBtn"),
								html.Span(html.ID("loginText"), gomponents.Text("Sign In")),
								html.I(html.Class("fas fa-spinner fa-spin d-none"), html.ID("loginSpinner")),
							),
						),
					),
				),

				html.Script(gomponents.Raw(`
					document.addEventListener('DOMContentLoaded', function() {
						const form = document.getElementById('loginForm');
						const loginBtn = document.getElementById('loginBtn');
						const loginText = document.getElementById('loginText');
						const loginSpinner = document.getElementById('loginSpinner');
						const usernameInput = document.getElementById('username');
						const passwordInput = document.getElementById('password');
						
						// Focus first input
						usernameInput.focus();
						
						// Add input validation styling
						[usernameInput, passwordInput].forEach(input => {
							input.addEventListener('input', function() {
								if (this.value.length > 0) {
									this.style.borderColor = 'rgba(56, 239, 125, 0.6)';
								} else {
									this.style.borderColor = 'rgba(255, 255, 255, 0.3)';
								}
							});
							
							input.addEventListener('keypress', function(e) {
								if (e.key === 'Enter' && this === usernameInput) {
									passwordInput.focus();
									e.preventDefault();
								}
							});
						});
						
						// Enhanced form submission
						form.addEventListener('submit', function(e) {
							// Add loading state
							document.body.classList.add('loading');
							loginText.textContent = 'Signing In...';
							loginSpinner.classList.remove('d-none');
							loginBtn.disabled = true;
							
							// Add slight delay for UX
							setTimeout(() => {
								// Form will submit naturally
							}, 500);
						});
						
						// Add enter key support
						document.addEventListener('keypress', function(e) {
							if (e.key === 'Enter' && !loginBtn.disabled) {
								form.submit();
							}
						});
						
						// Add some interactive particles on mouse move
						let mouseX = 0, mouseY = 0;
						document.addEventListener('mousemove', function(e) {
							mouseX = e.clientX;
							mouseY = e.clientY;
							
							// Create temporary particle
							if (Math.random() < 0.1) {
								const particle = document.createElement('div');
								particle.className = 'particle';
								particle.style.cssText = 'width: 2px; height: 2px; position: fixed; pointer-events: none; z-index: 1;';
								particle.style.left = mouseX + 'px';
								particle.style.top = mouseY + 'px';
								document.body.appendChild(particle);
								
								setTimeout(() => {
									particle.remove();
								}, 2000);
							}
						});
					});
				`)),
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

		// Use requireAuth for authentication
		requireAuth(c)
		if c.IsAborted() {
			return
		}

		// Use requireCSRF for CSRF protection
		requireCSRF(c)
		if c.IsAborted() {
			return
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

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

// SessionStore holds active sessions.
type SessionStore struct {
	sessions map[string]*Session
	mutex    sync.RWMutex
}

// Session represents a user session.
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

// generateSessionID creates a cryptographically secure session ID.
func generateSessionID() string {
	return generateSecureToken(SessionIDLength)
}

// generateCSRFToken creates a CSRF token for the session.
func generateCSRFToken() string {
	return generateSecureToken(CSRFTokenLength)
}

// generateSecureToken creates a cryptographically secure token of specified length.
func generateSecureToken(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// createSession creates a new session for the user.
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

// getSession retrieves a session by ID.
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

// deleteSession removes a session.
func (ss *SessionStore) deleteSession(sessionID string) {
	ss.mutex.Lock()
	defer ss.mutex.Unlock()

	delete(ss.sessions, sessionID)
}

// cleanupExpiredSessions removes expired sessions.
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

// Default authentication constants.
const (
	DefaultUsername = "admin"
	DefaultPassword = "admin"
	SessionDuration = 24 * time.Hour
	CSRFTokenLength = 16
	SessionIDLength = 32
)

// authenticateUser checks if the provided credentials are valid.
func authenticateUser(username, password string) bool {
	settings := config.GetSettingsGeneral()

	// Use API key as password, fallback to default
	expectedPassword := settings.WebAPIKey
	if expectedPassword == "" {
		expectedPassword = DefaultPassword
	}

	return username == DefaultUsername && password == expectedPassword
}

// requireAuth middleware checks for valid session.
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

// requireCSRF middleware checks CSRF token for state-changing operations.
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

// redirectToLogin redirects user to login page.
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

// loginPage renders the enhanced login form using AdminKit layout.
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
				html.Meta(
					html.Name("viewport"),
					html.Content("width=device-width, initial-scale=1, shrink-to-fit=no"),
				),
				html.Title("Go Media Downloader - Sign In"),
				html.Link(
					html.Href(
						"https://fonts.googleapis.com/css2?family=Inter:wght@300;400;600&display=swap",
					),
					html.Rel("stylesheet"),
				),
				html.Link(
					html.Href(
						"https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css",
					),
					html.Rel("stylesheet"),
				),
				html.Link(
					html.Href(
						"https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css",
					),
					html.Rel("stylesheet"),
				),
				html.StyleEl(gomponents.Raw(`
					body {
						font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
						background: #f5f7fb;
					}

					.d-table {
						display: table;
						width: 100%;
					}

					.d-table-cell {
						display: table-cell;
					}

					.text-center {
						text-align: center;
					}

					.card {
						border: 0;
						border-radius: 0.5rem;
						box-shadow: 0 0.5rem 1rem rgba(0, 0, 0, 0.15);
					}

					.card-body {
						padding: 2rem;
					}

					.form-control-lg {
						font-size: 1rem;
						padding: 0.75rem 1rem;
						border-radius: 0.375rem;
					}

					.form-label {
						font-weight: 600;
						color: #495057;
					}

					.btn-primary {
						background-color: #3b7ddd;
						border-color: #3b7ddd;
					}

					.btn-primary:hover {
						background-color: #326abc;
						border-color: #326abc;
					}

					.btn-lg {
						padding: 0.75rem 1rem;
						font-size: 1rem;
					}

					.lead {
						color: #6c757d;
					}

					.alert-danger {
						background-color: #f8d7da;
						border-color: #f5c2c7;
						color: #842029;
					}

					.form-check-input:checked {
						background-color: #3b7ddd;
						border-color: #3b7ddd;
					}

					.spinner-border-sm {
						width: 1rem;
						height: 1rem;
					}

					a {
						color: #3b7ddd;
						text-decoration: none;
					}

					a:hover {
						color: #326abc;
						text-decoration: underline;
					}

					.logo-icon {
						width: 64px;
						height: 64px;
						background: linear-gradient(135deg, #3b7ddd 0%, #6f42c1 100%);
						border-radius: 50%;
						display: flex;
						align-items: center;
						justify-content: center;
						margin: 0 auto 1rem;
						box-shadow: 0 4px 12px rgba(59, 125, 221, 0.3);
					}

					.logo-icon i {
						font-size: 1.75rem;
						color: white;
					}
				`)),
			),
			html.Body(
				html.Main(
					html.Class("d-flex w-100 h-100"),
					html.Div(
						html.Class("container d-flex flex-column"),
						html.Div(
							html.Class("row vh-100"),
							html.Div(
								html.Class(
									"col-sm-10 col-md-8 col-lg-6 col-xl-5 mx-auto d-table h-100",
								),
								html.Div(
									html.Class("d-table-cell align-middle"),

									// Header section
									html.Div(
										html.Class("text-center mt-4"),
										html.Div(
											html.Class("logo-icon"),
											html.I(html.Class("fas fa-download")),
										),
										html.H1(
											html.Class("h2"),
											gomponents.Text("Welcome back!"),
										),
										html.P(
											html.Class("lead"),
											gomponents.Text("Sign in to Go Media Downloader"),
										),
									),

									// Card section
									html.Div(
										html.Class("card"),
										html.Div(
											html.Class("card-body"),
											html.Div(
												html.Class("m-sm-3"),

												// Error message
												gomponents.If(errorMsg != "",
													html.Div(
														html.Class("alert alert-danger"),
														html.Role("alert"),
														html.I(
															html.Class(
																"fas fa-exclamation-triangle me-2",
															),
														),
														gomponents.Text(errorMsg),
													),
												),

												// Login form
												html.Form(
													html.Method("POST"),
													html.Action("/api/login"),
													html.ID("loginForm"),

													html.Div(
														html.Class("mb-3"),
														html.Label(
															html.Class("form-label"),
															html.For("username"),
															gomponents.Text("Username"),
														),
														html.Input(
															html.Type("text"),
															html.Class(
																"form-control form-control-lg",
															),
															html.Name("username"),
															html.ID("username"),
															html.Placeholder("Enter your username"),
															html.Required(),
															gomponents.Attr(
																"autocomplete",
																"username",
															),
														),
													),

													html.Div(
														html.Class("mb-3"),
														html.Label(
															html.Class("form-label"),
															html.For("password"),
															gomponents.Text("Password"),
														),
														html.Input(
															html.Type("password"),
															html.Class(
																"form-control form-control-lg",
															),
															html.Name("password"),
															html.ID("password"),
															html.Placeholder("Enter your password"),
															html.Required(),
															gomponents.Attr(
																"autocomplete",
																"current-password",
															),
														),
													),

													html.Div(
														html.Div(
															html.Class(
																"form-check align-items-center",
															),
															html.Input(
																html.ID("rememberMe"),
																html.Type("checkbox"),
																html.Class("form-check-input"),
																html.Name("remember-me"),
																html.Value("remember-me"),
															),
															html.Label(
																html.Class(
																	"form-check-label text-small",
																),
																html.For("rememberMe"),
																gomponents.Text("Remember me"),
															),
														),
													),

													html.Div(
														html.Class("d-grid gap-2 mt-3"),
														html.Button(
															html.Type("submit"),
															html.Class("btn btn-lg btn-primary"),
															html.ID("loginBtn"),
															html.Span(
																html.ID("loginText"),
																gomponents.Text("Sign in"),
															),
															html.Span(
																html.Class(
																	"spinner-border spinner-border-sm ms-2 d-none",
																),
																html.ID("loginSpinner"),
																html.Role("status"),
																gomponents.Attr(
																	"aria-hidden",
																	"true",
																),
															),
														),
													),
												),
											),
										),
									),

									// Footer link
									html.Div(
										html.Class("text-center mb-3"),
										html.A(
											html.Href(
												"https://github.com/Kellerman81/go_media_downloader",
											),
											html.Target("_blank"),
											html.Rel("noopener noreferrer"),
											html.I(html.Class("fab fa-github me-1")),
											gomponents.Text("View on GitHub"),
										),
									),
								),
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

						// Enhanced form submission
						form.addEventListener('submit', function(e) {
							// Add loading state
							loginText.textContent = 'Signing in...';
							loginSpinner.classList.remove('d-none');
							loginBtn.disabled = true;
						});

						// Enter key navigation
						usernameInput.addEventListener('keypress', function(e) {
							if (e.key === 'Enter') {
								e.preventDefault();
								passwordInput.focus();
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

// handleLogin processes login form submission.
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

// handleLogout clears session and redirects to login.
func handleLogout(c *gin.Context) {
	if sessionCookie, err := c.Cookie("session_id"); err == nil {
		sessionStore.deleteSession(sessionCookie)
	}

	c.SetCookie("session_id", "", -1, "/", "", false, true)
	c.Redirect(http.StatusFound, "/api/login")
}

// protectAdminRoutes middleware automatically protects admin and manage routes.
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

// NotFoundPage renders a 404 page in AdminKit layout.
func NotFoundPage(c *gin.Context) {
	page := html.Doctype(
		html.HTML(
			html.Lang("en"),
			html.Head(
				html.Meta(html.Charset("utf-8")),
				html.Meta(
					html.Name("viewport"),
					html.Content("width=device-width, initial-scale=1, shrink-to-fit=no"),
				),
				html.Title("404 - Page Not Found | Go Media Downloader"),
				html.Link(
					html.Href(
						"https://fonts.googleapis.com/css2?family=Inter:wght@300;400;600&display=swap",
					),
					html.Rel("stylesheet"),
				),
				html.Link(
					html.Href(
						"https://cdn.jsdelivr.net/npm/bootstrap@5.1.3/dist/css/bootstrap.min.css",
					),
					html.Rel("stylesheet"),
				),
				html.Link(
					html.Href(
						"https://cdnjs.cloudflare.com/ajax/libs/font-awesome/6.0.0/css/all.min.css",
					),
					html.Rel("stylesheet"),
				),
				html.StyleEl(gomponents.Raw(`
					body {
						font-family: 'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
						background: #f5f7fb;
					}

					.d-table {
						display: table;
						width: 100%;
					}

					.d-table-cell {
						display: table-cell;
					}

					.display-1 {
						font-size: 6rem;
						font-weight: 700;
						color: #3b7ddd;
					}

					.btn-primary {
						background-color: #3b7ddd;
						border-color: #3b7ddd;
					}

					.btn-primary:hover {
						background-color: #326abc;
						border-color: #326abc;
					}

					.btn-lg {
						padding: 0.75rem 1.5rem;
						font-size: 1rem;
					}

					a {
						color: #3b7ddd;
						text-decoration: none;
					}

					a:hover {
						color: #326abc;
						text-decoration: underline;
					}
				`)),
			),
			html.Body(
				html.Main(
					html.Class("d-flex w-100 h-100"),
					html.Div(
						html.Class("container d-flex flex-column"),
						html.Div(
							html.Class("row vh-100"),
							html.Div(
								html.Class(
									"col-sm-10 col-md-8 col-lg-6 col-xl-5 mx-auto d-table h-100",
								),
								html.Div(
									html.Class("d-table-cell align-middle"),

									html.Div(
										html.Class("text-center"),
										html.H1(
											html.Class("display-1 fw-bold"),
											gomponents.Text("404"),
										),
										html.P(
											html.Class("h2"),
											gomponents.Text("Page not found."),
										),
										html.P(
											html.Class("lead fw-normal mt-3 mb-4"),
											gomponents.Text(
												"The page you are looking for might have been removed or is temporarily unavailable.",
											),
										),
										html.A(
											html.Href("/api/admin"),
											html.Class("btn btn-primary btn-lg"),
											html.I(html.Class("fas fa-home me-2")),
											gomponents.Text("Return to Home"),
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
	c.Status(http.StatusNotFound)

	var buf strings.Builder
	page.Render(&buf)
	c.String(http.StatusNotFound, buf.String())
}

// handleRootRedirect handles root path requests and redirects to admin or login.
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

// startSessionCleanup starts a goroutine to periodically clean expired sessions.
func startSessionCleanup() {
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()

		for range ticker.C {
			sessionStore.cleanupExpiredSessions()
		}
	}()
}

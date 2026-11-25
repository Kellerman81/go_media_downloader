package api

import (
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderTraktAuthPage renders a page for Trakt authentication.
func renderTraktAuthPage(csrfToken string) gomponents.Node {
	// Check current token status
	token := traktToken()
	hasValidToken := token != nil && token.AccessToken != ""

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		renderEnhancedPageHeader(
			"fa-solid fa-key",
			"Trakt Authentication",
			"Authenticate with Trakt.tv to enable synchronized watchlists, ratings, and recommendations. This uses OAuth2 for secure authentication.",
		),

		// Current status display
		html.Div(
			func() gomponents.Node {
				if hasValidToken {
					return html.Class("card border-0 shadow-sm")
				}

				return html.Class("card border-0 shadow-sm border-warning")
			}(),
			html.Div(
				html.Class("card-body"),
				html.H5(
					html.Class("card-title fw-bold mb-3"),
					gomponents.Text("Current Authentication Status"),
				),
				html.Div(html.Class("d-flex align-items-center mb-3"), func() gomponents.Node {
					if hasValidToken {
						return html.Div(
							html.Span(
								html.Class("badge bg-success me-2"),
								html.I(html.Class("fas fa-check me-1")),
								gomponents.Text("Authenticated"),
							),
							html.Span(
								html.Class("text-success"),
								gomponents.Text("Trakt API access is enabled"),
							),
						)
					}

					return html.Div(
						html.Span(
							html.Class("badge bg-danger me-2"),
							html.I(html.Class("fas fa-times me-1")),
							gomponents.Text("Not Authenticated"),
						),
						html.Span(
							html.Class("text-danger"),
							gomponents.Text("Trakt API access is disabled"),
						),
					)
				}()),
				func() gomponents.Node {
					if hasValidToken {
						return html.Div(
							html.Class("mt-3"),
							html.H6(html.Class("fw-bold mb-2"), gomponents.Text("Token Details")),
							html.Ul(html.Class("list-unstyled"),
								html.Li(
									html.Class("mb-1"),
									html.Span(
										html.Class("badge bg-secondary me-2"),
										html.I(html.Class("fas fa-key me-1")),
										gomponents.Text("Access Token"),
									),
									html.Code(
										html.Class("text-muted"),
										gomponents.Text(token.AccessToken[:20]+"..."),
									),
								),
								html.Li(
									html.Class("mb-1"),
									html.Span(
										html.Class("badge bg-info me-2"),
										html.I(html.Class("fas fa-tag me-1")),
										gomponents.Text("Type"),
									),
									gomponents.Text(token.TokenType),
								),
								html.Li(
									html.Class("mb-0"),
									html.Span(
										html.Class("badge bg-warning me-2"),
										html.I(html.Class("fas fa-clock me-1")),
										gomponents.Text("Expiry"),
									),
									gomponents.Text(func() string {
										if token.Expiry.IsZero() {
											return "Never expires"
										}

										return token.Expiry.Format("2006-01-02 15:04:05")
									}()),
								),
							),
						)
					}

					return gomponents.Text("")
				}(),
			),
		),

		// Authentication form
		html.Form(
			html.Class("config-form"),
			html.ID("traktAuthForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Step 1: Get Authorization URL"),
					),
					html.P(
						gomponents.Text(
							"Click the button below to generate an authorization URL. This will open Trakt.tv in a new tab where you can authorize this application.",
						),
					),

					html.Button(
						html.Class("btn btn-info"),
						gomponents.Text("Get Trakt Authorization URL"),
						html.Type("button"),
						hx.Target("#authUrlResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"get_url\"}"),
					),
					html.Div(
						html.ID("authUrlResult"),
						html.Class("mt-3"),
						html.Style("min-height: 30px;"),
					),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Step 2: Enter Authorization Code"),
					),
					html.P(
						gomponents.Text(
							"After authorizing on Trakt.tv, you'll receive a code. Enter it below to complete the authentication process.",
						),
					),

					renderFormGroup("trakt", map[string]string{
						"AuthCode": "Enter the authorization code from Trakt.tv",
					}, map[string]string{
						"AuthCode": "Authorization Code",
					}, "AuthCode", "text", "", nil),

					html.Button(
						html.Class(ClassBtnSuccess),
						gomponents.Text("Store Trakt Token"),
						html.Type("button"),
						hx.Target("#tokenResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Include("#traktAuthForm"),
						hx.Vals("{\"action\": \"store_token\"}"),
					),
					html.Div(
						html.ID("tokenResult"),
						html.Class("mt-3"),
						html.Style("min-height: 30px;"),
					),
				),
			),
		),

		// Token management section
		func() gomponents.Node {
			if hasValidToken {
				return html.Div(
					html.Class("mt-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("Token Management")),
					html.Div(
						html.Class("btn-group"),
						html.Button(
							html.Class("btn btn-warning"),
							gomponents.Text("Refresh Token"),
							html.Type("button"),
							hx.Target("#refreshResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"refresh_token\"}"),
						),
						html.Button(
							html.Class("btn btn-danger ml-2"),
							gomponents.Text("Revoke Token"),
							html.Type("button"),
							hx.Target("#revokeResult"),
							hx.Swap("innerHTML"),
							hx.Post("/api/admin/traktauth"),
							hx.Headers(createHTMXHeaders(csrfToken)),
							hx.Vals("{\"action\": \"revoke_token\"}"),
							hx.Confirm(
								"Revoke the Trakt authentication? This will disable Trakt API access.",
							),
						),
					),
					html.Div(
						html.ID("refreshResult"),
						html.Class("mt-3"),
					),
					html.Div(
						html.ID("revokeResult"),
						html.Class("mt-3"),
					),
				)
			}

			return gomponents.Text("")
		}(),

		// API Test section
		func() gomponents.Node {
			if hasValidToken {
				return html.Div(
					html.Class("mt-4"),
					html.H5(html.Class("form-section-title"), gomponents.Text("API Test")),
					html.P(
						gomponents.Text(
							"Test the Trakt API connection by fetching popular movies:",
						),
					),
					html.Button(
						html.Class(ClassBtnSecondary),
						gomponents.Text("Test Trakt API"),
						html.Type("button"),
						hx.Target("#apiTestResult"),
						hx.Swap("innerHTML"),
						hx.Post("/api/admin/traktauth"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Vals("{\"action\": \"test_api\"}"),
					),
					html.Div(
						html.ID("apiTestResult"),
						html.Class("mt-3"),
						html.Style("min-height: 30px;"),
					),
				)
			}

			return gomponents.Text("")
		}(),

		// Instructions
		html.Div(
			html.Class("mt-4 card border-0 shadow-sm border-primary mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #cfe2ff 0%, #b6d7ff 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-primary me-3"),
						html.I(html.Class("fas fa-cog me-1")),
						gomponents.Text("Setup"),
					),
					html.H5(
						html.Class("card-title mb-0 text-primary fw-bold"),
						gomponents.Text("Setup Instructions"),
					),
				),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					html.Class("card-text text-muted mb-3"),
					gomponents.Text("Follow these steps to set up Trakt authentication"),
				),
				html.Ol(
					html.Class("mb-0 list-unstyled"),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"1. Make sure your Trakt Client ID and Client Secret are configured in the General settings",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"2. Click 'Get Trakt Authorization URL' to generate the authorization link",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"3. Visit the generated URL and authorize this application on Trakt.tv",
						),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text("4. Copy the authorization code from the redirect URL"),
					),
					html.Li(
						html.Class("mb-2"),
						gomponents.Text(
							"5. Paste the code in the form above and click 'Store Trakt Token'",
						),
					),
					html.Li(
						html.Class("mb-0"),
						gomponents.Text(
							"6. Your Trakt authentication is now complete and will be stored securely",
						),
					),
				),
			),
		),
	)
}

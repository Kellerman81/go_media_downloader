package api

import (
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// renderPushoverTestPage renders a page for testing Pushover notifications.
func renderPushoverTestPage(csrfToken string) gomponents.Node {
	// Get available notification configurations (any type, not just Pushover).
	notifications := config.GetSettingsNotificationAll()

	notificationConfigs := make([]string, 0, len(notifications))
	for i := range notifications {
		notificationConfigs = append(notificationConfigs, notifications[i].Name)
	}

	return html.Div(
		html.Class("config-section-enhanced"),

		// Enhanced page header with gradient background
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid fa-paper-plane header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Notification Test Message")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Send a test message through any configured notification to verify it is working correctly.",
						),
					),
				),
			),
		),

		html.Form(
			html.Class("config-form"),
			html.ID("pushoverForm"),

			html.Div(
				html.Class("row"),
				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Message Configuration"),
					),

					renderFormGroup("pushover", map[string]string{
						"NotificationConfig": "Select the notification configuration to test (its type determines how the message is sent)",
					}, map[string]string{
						"NotificationConfig": "Notification Config",
					}, "NotificationConfig", "select", "", map[string][]string{
						"options": notificationConfigs,
					}),

					renderFormGroup("pushover", map[string]string{
						"MessageTitle": "Title for the test message",
					}, map[string]string{
						"MessageTitle": "Message Title",
					}, "MessageTitle", "text", "Test Message from Media Downloader", nil),

					renderFormGroup("pushover", map[string]string{
						"MessageText": "Content of the test message",
					}, map[string]string{
						"MessageText": "Message Text",
					}, "MessageText", "textarea", "This is a test message to verify the notification configuration is working correctly.", nil),
				),

				html.Div(
					html.Class("col-md-6"),
					html.H5(
						html.Class("form-section-title"),
						gomponents.Text("Supported Notification Types"),
					),
					html.P(
						gomponents.Text(
							"The message is sent through the selected configuration with its own type and settings. "+
								"Every notification type works: "+strings.Join(providers.NotificationTypeNames(), ", ")+".",
						),
					),
					html.P(
						gomponents.Text("A CSV notification appends the message to its output file; exec runs its command."),
					),
					html.P(
						html.Class("mt-3"),
						html.Strong(gomponents.Text("Note: ")),
						gomponents.Text(
							"Make sure you have at least one notification configuration set up before testing.",
						),
					),
				),
			),

			html.Div(
				html.Class("form-group submit-group"),
				html.Button(
					html.Class("btn btn-primary"),
					gomponents.Text("Send Test Message"),
					html.Type("button"),
					hx.Target("#pushoverResults"),
					hx.Swap("innerHTML"),
					hx.Post("/api/admin/pushovertest"),
					hx.Headers("{\"X-CSRF-Token\": \""+csrfToken+"\"}"),
					hx.Include("#pushoverForm"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-secondary ml-2"),
					gomponents.Attr(
						"onclick",
						"document.getElementById('pushoverForm').reset(); document.getElementById('pushoverResults').innerHTML = '';",
					),
					gomponents.Text("Reset"),
				),
			),
		),

		html.Div(
			html.ID("pushoverResults"),
			html.Class("mt-4"),
			html.Style("min-height: 50px;"),
		),
	)
}

// HandlePushoverTest handles Pushover test message requests.
func HandlePushoverTest(c *gin.Context) {
	if err := c.Request.ParseForm(); err != nil {
		c.String(http.StatusOK, renderAlert("Failed to parse form data: "+err.Error(), "danger"))
		return
	}

	notificationConfig := c.PostForm("pushover_NotificationConfig")
	messageTitle := c.PostForm("pushover_MessageTitle")
	messageText := c.PostForm("pushover_MessageText")

	if notificationConfig == "" {
		c.String(
			http.StatusOK,
			renderAlert("Please select a notification configuration", "warning"),
		)

		return
	}

	// Get the notification configuration (resolved first so validation can depend
	// on its type - e.g. CSV has no message title).
	notifCfg := config.GetSettingsNotification(notificationConfig)
	if notifCfg == nil {
		c.String(http.StatusOK, renderAlert("Notification configuration not found", "danger"))
		return
	}

	isCSV := strings.EqualFold(notifCfg.NotificationType, "csv")

	if messageText == "" {
		c.String(http.StatusOK, renderAlert("Please provide message text", "warning"))
		return
	}

	if messageTitle == "" && !isCSV {
		c.String(http.StatusOK, renderAlert("Please provide a message title", "warning"))
		return
	}

	// Send the test message using the configuration's notification type.
	var err error

	switch strings.ToLower(notifCfg.NotificationType) {
	case "csv":
		scanner.AppendCsv(notifCfg.Outputto, messageText)
	default:
		if providers.IsSentNotification(notifCfg.NotificationType) {
			err = providers.SendNotification(notifCfg, messageText, messageTitle)
			break
		}

		c.String(
			http.StatusOK,
			renderAlert("Unsupported notification type: "+notifCfg.NotificationType, "warning"),
		)

		return
	}

	var result gomponents.Node
	if err != nil {
		result = html.Div(
			html.Class("card border-0 shadow-sm border-danger mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #f8d7da 0%, #f5c6cb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-danger me-3"),
						html.I(html.Class("fas fa-times-circle me-1")),
						gomponents.Text("Failed"),
					),
					html.H5(
						html.Class("card-title mb-0 text-danger fw-bold"),
						gomponents.Text("Message Send Failed"),
					),
				),
			),

			html.Div(
				html.Class("card border-0"),
				html.Style("background: rgba(220, 53, 69, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3 text-center"),
					html.I(
						html.Class("fas fa-exclamation-triangle mb-2"),
						html.Style("color: #dc3545; font-size: 2rem;"),
					),
					html.P(
						html.Class("mb-2"),
						html.Style("color: #495057;"),
						gomponents.Text("Failed to send "+notifCfg.NotificationType+" message:"),
					),
					html.P(
						html.Class("mb-2"),
						html.Style(
							"color: #dc3545; font-family: monospace; background: rgba(220, 53, 69, 0.1); padding: 0.5rem; border-radius: 4px;",
						),
						gomponents.Text(err.Error()),
					),
					html.P(
						html.Class("mb-0"),
						html.Style("color: #495057;"),
						gomponents.Text(
							"Please check your notification configuration and try again.",
						),
					),
				),
			),
		)
	} else {
		result = html.Div(
			html.Class("card border-0 shadow-sm border-success mb-4"),
			html.Div(
				html.Class("card-header border-0"),
				html.Style(
					"background: linear-gradient(135deg, #d4edda 0%, #c3e6cb 100%); border-radius: 15px 15px 0 0;",
				),
				html.Div(
					html.Class("d-flex align-items-center"),
					html.Span(
						html.Class("badge bg-success me-3"),
						html.I(html.Class("fas fa-paper-plane me-1")),
						gomponents.Text("Success"),
					),
					html.H5(
						html.Class("card-title mb-0 text-success fw-bold"),
						gomponents.Text("Message Sent Successfully"),
					),
				),
			),

			html.Div(
				html.Class("card border-0 mb-3"),
				html.Style("background: rgba(40, 167, 69, 0.05); border-radius: 8px;"),
				html.Div(
					html.Class("card-body p-3"),
					html.P(
						html.Class("mb-3 text-center"),
						html.Style("color: #495057;"),
						gomponents.Text(
							"Test message sent via "+notifCfg.NotificationType+" with the following details:",
						),
					),
					html.Div(
						html.Class("row g-2"),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-cogs me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Configuration"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(notificationConfig),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-sm-6"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-center"),
										html.I(
											html.Class("fas fa-user me-2"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Recipient"),
											),
											html.Div(
												html.Class("fw-semibold"),
												gomponents.Text(notifCfg.Recipient),
											),
										),
									),
								),
							),
						),
						html.Div(
							html.Class("col-12"),
							html.Div(
								html.Class("card border-0"),
								html.Style(
									"background: rgba(40, 167, 69, 0.1); border-radius: 6px;",
								),
								html.Div(
									html.Class("card-body p-2"),
									html.Div(
										html.Class("d-flex align-items-start"),
										html.I(
											html.Class("fas fa-envelope me-2 mt-1"),
											html.Style("color: #28a745;"),
										),
										html.Div(
											html.Class("flex-grow-1"),
											html.Div(
												html.Class("small fw-bold text-muted"),
												gomponents.Text("Message Details"),
											),
											html.Div(
												html.Class("fw-semibold mb-1"),
												gomponents.Text("Title: "+messageTitle),
											),
											html.Div(
												html.Class("text-muted small"),
												gomponents.Text(messageText),
											),
										),
									),
								),
							),
						),
					),
				),
			),

			html.Div(
				html.Class("alert alert-light border-0 mb-0"),
				html.Style(
					"background-color: rgba(40, 167, 69, 0.1); border-radius: 8px; padding: 0.75rem 1rem;",
				),
				html.Div(
					html.Class("d-flex align-items-start"),
					html.I(
						html.Class("fas fa-mobile-alt me-2 mt-1"),
						html.Style("color: #28a745; font-size: 0.9rem;"),
					),
					html.Small(
						html.Style("color: #495057; line-height: 1.4;"),
						html.Strong(gomponents.Text("Next Step: ")),
						gomponents.Text("Check your device to confirm the message was received."),
					),
				),
			),
		)
	}

	c.String(http.StatusOK, renderComponentToString(result))
}

package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderNotificationForm renders one row; idx is the row index used in the form field names.
func renderNotificationForm(idx int, configv *config.NotificationConfig, csrfToken string) gomponents.Node {
	comments := logger.GetFieldComments(configv)
	displayNames := logger.GetFieldDisplayNames(configv)
	group := "notifications_" + strconv.Itoa(idx)

	return renderOptimizedArrayItemForm("notifications", configv.Name, "Notification", configv,
		renderNotificationConfigSections(configv, group, comments, displayNames, csrfToken))
}

// notificationTestFields are the fields HandleTestNotificationConnection
// reads, by name suffix within the current row.
var notificationTestFields = []connTestField{
	{Key: "notificationtype", Suffix: "NotificationType"},
	{Key: "apikey", Suffix: "Apikey"},
	{Key: "recipient", Suffix: "Recipient"},
	{Key: "sender", Suffix: "Sender"},
	{Key: "command", Suffix: "Command"},
	{Key: "serverurl", Suffix: "ServerURL"},
	{Key: "appriseurls", Suffix: "AppriseURLs"},
	{Key: "outputto", Suffix: "Outputto"},
	{Key: "smtpserver", Suffix: "SMTPServer"},
	{Key: "smtpport", Suffix: "SMTPPort"},
	{Key: "smtpfromemail", Suffix: "SMTPFromEmail"},
	{Key: "smtptoemail", Suffix: "SMTPToEmail"},
	{Key: "smtpusername", Suffix: "SMTPUsername"},
	{Key: "smtppassword", Suffix: "SMTPPassword"},
}

// renderNotificationConfigSections organizes notification fields into logical groups.
func renderNotificationConfigSections(
	configv *config.NotificationConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
	csrfToken string,
) gomponents.Node {
	// Sanitize name for use in HTML ID (replace spaces and special characters)
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(configv.Name, " ", "-"), "_", "-")
	accordionId := "notificationConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-notification-"+configv.Name, true,
			[]FormFieldDefinition{
				{Name: "", Type: "removebutton", Value: "", Options: nil},
				{
					Name:    "",
					Type:    "custom",
					Value:   renderTemplateSelect("Template", notificationTemplates),
					Options: nil,
				},
				{Name: "Name", Type: "text", Value: configv.Name, Options: nil},
				{
					Name:  "NotificationType",
					Type:  "select",
					Value: configv.NotificationType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": providers.NotificationTypeNames(),
					}),
				},
				{
					Name:  "",
					Type:  "custom",
					Value: renderConnectionTestControl(csrfToken, "/api/admin/notifications/test", notificationTestFields),
				},
			}, group, comments, displayNames, accordionId),

		// Configuration
		renderConfigGroupWithParent(
			"Configuration",
			"config-notification-"+strings.ReplaceAll(
				strings.ReplaceAll(configv.Name, " ", "-"),
				"_",
				"-",
			),
			false,
			[]FormFieldDefinition{
				{Name: "Apikey", Type: "password", Value: configv.Apikey, Options: nil},
				{Name: "Recipient", Type: "text", Value: configv.Recipient, Options: nil},
				{Name: "Sender", Type: "text", Value: configv.Sender, Options: nil},
				{Name: "Command", Type: "text", Value: configv.Command, Options: nil},
				{Name: "Outputto", Type: "text", Value: configv.Outputto, Options: nil},
				{Name: "ServerURL", Type: "text", Value: configv.ServerURL, Options: nil},
				{Name: "AppriseURLs", Type: "text", Value: configv.AppriseURLs, Options: nil},
				{Name: "SMTPServer", Type: "text", Value: configv.SMTPServer, Options: nil},
				{Name: "SMTPPort", Type: "text", Value: configv.SMTPPort, Options: nil},
				{Name: "SMTPFromEmail", Type: "text", Value: configv.SMTPFromEmail, Options: nil},
				{Name: "SMTPToEmail", Type: "text", Value: configv.SMTPToEmail, Options: nil},
				{Name: "SMTPUsername", Type: "text", Value: configv.SMTPUsername, Options: nil},
				{Name: "SMTPPassword", Type: "password", Value: configv.SMTPPassword, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// renderNotificationConfig renders the notification configuration section.
func renderNotificationConfig(
	configv []config.NotificationConfig,
	csrfToken string,
) gomponents.Node {
	options := RenderConfigOptions{
		Title:          "Notification Configuration",
		Subtitle:       "Set up notification systems and alert mechanisms to stay informed about download status, errors, and system events.",
		Icon:           "bell",
		FormContainer:  "notificationContainer",
		AddButtonText:  "Add Notification",
		AddEndpoint:    "/api/manage/notification/form",
		SubmitEndpoint: "/api/admin/config/notification/update",
	}

	return renderGenericConfigSection(
		configv,
		csrfToken,
		options,
		func(i int, config config.NotificationConfig, _ string) gomponents.Node {
			return renderNotificationForm(i, &config, csrfToken)
		},
	)
}

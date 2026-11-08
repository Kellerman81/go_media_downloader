package providers

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/apprise"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/discord"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/execcmd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/gotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/join"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/mailgun"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/ntfy"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/prowl"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushbullet"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushcut"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushover"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushsafer"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sendgrid"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sendmail"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/signal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/simplepush"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/slack"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/telegram"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/webhook"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

var (
	errNotificationNotRegistered = errors.New("notification provider not registered")
	errMessageEmpty              = errors.New("message empty")
	errMessageTooLong            = errors.New("message too long")
	errTitleTooLong              = errors.New("title too long")
	// ErrNotificationField marks a notification entry missing a setting its
	// type requires.
	ErrNotificationField = errors.New("notification setting empty")
)

// notificationRegMu serializes RegisterNotificationProviders so overlapping
// reloads can't publish an older config's set last.
var notificationRegMu sync.Mutex

// parseServerURL splits a notification server URL into the host, port and
// SSL flag the gotify/apprise constructors take. A reverse-proxy sub-path is
// kept on the host ("host/sub"), which those constructors split off again.
// A missing scheme means http; a missing port means 80/443.
// Example: "https://example.com:8080/gotify" -> ("example.com/gotify", 8080, true).
func parseServerURL(serverURL string) (host string, port int, useSSL bool) {
	host, port, useSSL, path := base.SplitHostURL(serverURL, 0, 0)

	return host + path, port, useSSL
}

// RegisterNotificationProviders replaces the registered notification
// providers with the notification entries of the current config, so added,
// changed and removed entries (and a changed general user agent) take effect
// without a restart. A send already running keeps the provider it fetched.
func RegisterNotificationProviders() {
	cm, ok := apiexternal_v2.GetGlobalClientManager()
	if !ok {
		logger.Logtype(logger.StatusWarning, 0).
			Msg("Global ClientManager not available for notification registration")

		return
	}

	notificationRegMu.Lock()
	defer notificationRegMu.Unlock()

	cm.ReplaceNotificationProviders(buildNotificationProviders())
}

// notificationType describes one notification type. required names the
// config fields it can't work without; maxMsg/maxTitle are the service's
// limits (0 = none). csv has no build: it is written to a file, not sent.
type notificationType struct {
	name             string
	maxMsg, maxTitle int
	required         func(c *config.NotificationConfig) map[string]string
	build            func(c *config.NotificationConfig, userAgent string) (apiexternal_v2.NotificationProvider, error)
}

// emails splits SMTPToEmail into its comma-separated addresses.
func emails(list string) []string {
	var out []string
	for _, e := range strings.Split(list, ",") {
		if e = strings.TrimSpace(e); e != "" {
			out = append(out, e)
		}
	}

	return out
}

var notificationTypes = []notificationType{
	{name: "csv", required: func(c *config.NotificationConfig) map[string]string {
		return map[string]string{"output_to": c.Outputto}
	}},
	{
		name: "pushover", maxMsg: 1024, maxTitle: 250,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey, "recipient": c.Recipient}
		},
		build: func(c *config.NotificationConfig, ua string) (apiexternal_v2.NotificationProvider, error) {
			return pushover.NewProviderWithConfig(base.ClientConfig{
				Name:                      "pushover_" + c.Name,
				Timeout:                   30 * time.Second,
				AuthType:                  base.AuthNone, // Pushover handles auth via form parameters
				RateLimitCalls:            300,           // Pushover allows ~10,000/month = ~300/hour conservative
				RateLimitSeconds:          3600,
				CircuitBreakerThreshold:   3,
				CircuitBreakerTimeout:     30 * time.Second,
				CircuitBreakerHalfOpenMax: 1,
				UserAgent:                 ua,
			}, c.Apikey, c.Recipient), nil
		},
	},
	{
		name: "gotify", maxMsg: 4096, maxTitle: 256,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"server_url": c.ServerURL, "apikey": c.Apikey}
		},
		build: func(c *config.NotificationConfig, ua string) (apiexternal_v2.NotificationProvider, error) {
			host, port, useSSL := parseServerURL(c.ServerURL)

			return gotify.NewProviderWithConfig(base.ClientConfig{
				Name:                      "gotify_" + c.Name,
				Timeout:                   30 * time.Second,
				AuthType:                  base.AuthNone, // Gotify provider handles auth internally
				RateLimitCalls:            1000,          // Gotify is self-hosted, generous limit
				RateLimitSeconds:          3600,
				CircuitBreakerThreshold:   3,
				CircuitBreakerTimeout:     30 * time.Second,
				CircuitBreakerHalfOpenMax: 1,
				UserAgent:                 ua,
			}, host, port, c.Apikey, useSSL), nil
		},
	},
	{
		name: "pushbullet", maxMsg: 8192, maxTitle: 140,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return pushbullet.NewProvider(c.Apikey), nil
		},
	},
	{
		name: "apprise",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"server_url": c.ServerURL, "apprise_urls": c.AppriseURLs}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			host, port, useSSL := parseServerURL(c.ServerURL)
			return apprise.NewProvider(host, port, c.Apikey, strings.Split(c.AppriseURLs, ","), useSSL), nil
		},
	},
	{
		name: "sendmail",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"smtp_server": c.SMTPServer, "smtp_from": c.SMTPFromEmail, "smtp_to": c.SMTPToEmail}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			port, _ := strconv.Atoi(c.SMTPPort)
			if port == 0 {
				port = 587 // Default SMTP submission port
			}

			return sendmail.NewProvider(c.SMTPServer, port, c.SMTPFromEmail, emails(c.SMTPToEmail),
				c.SMTPUsername, c.SMTPPassword), nil
		},
	},
	{
		name: "discord", maxMsg: 4096, maxTitle: 256,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"server_url": c.ServerURL}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return discord.NewProvider(c.ServerURL), nil
		},
	},
	{
		name: "slack",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"server_url": c.ServerURL}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return slack.NewProvider(c.ServerURL), nil
		},
	},
	{
		name: "telegram", maxMsg: 3800, maxTitle: 200,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey, "recipient": c.Recipient}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return telegram.NewProvider(c.Apikey, c.Recipient), nil
		},
	},
	{
		name: "ntfy", maxMsg: 4096, maxTitle: 250,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"recipient": c.Recipient}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return ntfy.NewProvider(c.ServerURL, c.Recipient, c.Apikey), nil
		},
	},
	{
		name: "webhook",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"server_url": c.ServerURL}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return webhook.NewProvider(c.ServerURL), nil
		},
	},
	{
		// Join sends everything in the query string, so keep it URL-sized.
		name: "join", maxMsg: 1500, maxTitle: 200,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return join.NewProvider(c.Apikey, c.Recipient), nil
		},
	},
	{
		name: "prowl", maxMsg: 10000, maxTitle: 1024,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return prowl.NewProvider(c.Apikey), nil
		},
	},
	{
		name: "pushcut",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey, "recipient": c.Recipient}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return pushcut.NewProvider(c.Apikey, c.Recipient), nil
		},
	},
	{
		name: "pushsafer", maxMsg: 4096, maxTitle: 255,
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return pushsafer.NewProvider(c.Apikey, c.Recipient), nil
		},
	},
	{
		name: "simplepush",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return simplepush.NewProvider(c.Apikey), nil
		},
	},
	{
		name: "signal",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"server_url": c.ServerURL, "sender": c.Sender, "recipient": c.Recipient}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return signal.NewProvider(c.ServerURL, c.Sender, c.Recipient), nil
		},
	},
	{
		name: "mailgun",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey, "smtp_from": c.SMTPFromEmail, "smtp_to": c.SMTPToEmail}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return mailgun.NewProvider(c.Apikey, c.ServerURL, c.SMTPFromEmail, emails(c.SMTPToEmail))
		},
	},
	{
		name: "exec",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"command": c.Command}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return execcmd.NewProvider(c.Command), nil
		},
	},
	{
		name: "sendgrid",
		required: func(c *config.NotificationConfig) map[string]string {
			return map[string]string{"apikey": c.Apikey, "smtp_from": c.SMTPFromEmail, "smtp_to": c.SMTPToEmail}
		},
		build: func(c *config.NotificationConfig, _ string) (apiexternal_v2.NotificationProvider, error) {
			return sendgrid.NewProvider(c.Apikey, c.SMTPFromEmail, emails(c.SMTPToEmail))
		},
	},
}

// NotificationTypeNames returns every supported notification type, csv
// included, in the order the config screens offer them.
func NotificationTypeNames() []string {
	names := make([]string, len(notificationTypes))
	for i, t := range notificationTypes {
		names[i] = t.name
	}

	return names
}

func findNotificationType(name string) *notificationType {
	for i := range notificationTypes {
		if notificationTypes[i].name == name {
			return &notificationTypes[i]
		}
	}

	return nil
}

// IsSentNotification reports whether notifications of type name go through a
// provider (every type but csv).
func IsSentNotification(name string) bool {
	t := findNotificationType(name)
	return t != nil && t.build != nil
}

// CheckNotificationFields returns an error naming the first required field
// of cfg's type that is empty.
func CheckNotificationFields(cfg *config.NotificationConfig) error {
	t := findNotificationType(cfg.NotificationType)
	if t == nil {
		return fmt.Errorf("notification type %q: %w", cfg.NotificationType, errNotificationNotRegistered)
	}

	required := t.required(cfg)
	for _, field := range slices.Sorted(maps.Keys(required)) {
		if required[field] == "" {
			return fmt.Errorf("%s: %w", field, ErrNotificationField)
		}
	}

	return nil
}

// NewNotificationProvider builds the provider for one notification config,
// as registered for sending and used by the connection tests.
func NewNotificationProvider(cfg *config.NotificationConfig) (apiexternal_v2.NotificationProvider, error) {
	if err := CheckNotificationFields(cfg); err != nil {
		return nil, err
	}

	t := findNotificationType(cfg.NotificationType)
	if t.build == nil {
		return nil, fmt.Errorf("notification type %q: %w", cfg.NotificationType, errNotificationNotRegistered)
	}

	p, err := t.build(cfg, config.GetSettingsGeneral().UserAgent)
	if err != nil {
		return nil, err
	}

	// The constructors return typed nil pointers on bad input.
	if p == nil || reflect.ValueOf(p).IsNil() {
		return nil, fmt.Errorf("notification %q: invalid %s configuration", cfg.Name, cfg.NotificationType)
	}

	return p, nil
}

// buildNotificationProviders creates one provider per usable notification
// entry, keyed by the entry's name as SendNotification looks it up.
func buildNotificationProviders() map[string]apiexternal_v2.NotificationProvider {
	set := make(map[string]apiexternal_v2.NotificationProvider)

	config.RangeSettingsNotification(func(name string, notifCfg *config.NotificationConfig) {
		if !IsSentNotification(notifCfg.NotificationType) {
			return
		}

		provider, err := NewNotificationProvider(notifCfg)
		if err != nil {
			logger.Logtype(logger.StatusDebug, 0).
				Err(err).
				Str("notification", name).
				Msg("Notification provider not registered")

			return
		}

		set[name] = provider

		logger.Logtype(logger.StatusDebug, 0).
			Str("notification", name).
			Str("type", notifCfg.NotificationType).
			Msg("Registered notification provider")
	})

	return set
}

// SendNotification sends message/title through the registered provider of a
// notification entry of any type but csv. The limits are those of the
// respective services.
func SendNotification(cfg *config.NotificationConfig, message, title string) error {
	if !IsSentNotification(cfg.NotificationType) {
		return fmt.Errorf("notification type %q: %w", cfg.NotificationType, errNotificationNotRegistered)
	}

	if err := CheckNotificationFields(cfg); err != nil {
		return err
	}

	t := findNotificationType(cfg.NotificationType)

	switch {
	case message == "":
		return errMessageEmpty
	case t.maxMsg > 0 && len(message) > t.maxMsg:
		return errMessageTooLong
	case t.maxTitle > 0 && len(title) > t.maxTitle:
		return errTitleTooLong
	}

	cm, ok := apiexternal_v2.GetGlobalClientManager()
	if !ok {
		return errNotificationNotRegistered
	}

	provider, ok := cm.GetNotificationProvider(cfg.Name)
	if !ok {
		return errNotificationNotRegistered
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{
		Title:   title,
		Message: message,
	})

	return err
}

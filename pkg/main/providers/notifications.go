package providers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/apprise"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/gotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushbullet"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushover"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sendmail"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

var (
	errNotificationNotRegistered = errors.New("notification provider not registered")
	errMessageEmpty              = errors.New("message empty")
	errMessageTooLong            = errors.New("message too long")
	errTitleTooLong              = errors.New("title too long")
	errNotificationField         = errors.New("notification setting empty")
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

// buildNotificationProviders creates one provider per usable notification
// entry. The map keys are the names SendNotification looks up
// (pushover entries are prefixed with "pushover_").
func buildNotificationProviders() map[string]apiexternal_v2.NotificationProvider {
	set := make(map[string]apiexternal_v2.NotificationProvider)
	userAgent := config.GetSettingsGeneral().UserAgent

	config.RangeSettingsNotification(func(name string, notifCfg *config.NotificationConfig) {
		switch notifCfg.NotificationType {
		case "pushover":
			pushoverConfig := base.ClientConfig{
				Name:                      "pushover_" + notifCfg.Name,
				Timeout:                   30 * time.Second,
				AuthType:                  base.AuthNone, // Pushover handles auth via form parameters
				RateLimitCalls:            300,           // Pushover allows ~10,000/month = ~300/hour conservative
				RateLimitSeconds:          3600,          // 1 hour
				CircuitBreakerThreshold:   3,
				CircuitBreakerTimeout:     30 * time.Second,
				CircuitBreakerHalfOpenMax: 1,
				UserAgent:                 userAgent,
			}
			if provider := pushover.NewProviderWithConfig(
				pushoverConfig,
				notifCfg.Apikey,
				notifCfg.Recipient,
			); provider != nil {
				set["pushover_"+notifCfg.Name] = provider
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered pushover notification provider with rate limiting")
			}

		case "gotify":
			if notifCfg.ServerURL == "" || notifCfg.Apikey == "" {
				break
			}

			host, port, useSSL := parseServerURL(notifCfg.ServerURL)

			gotifyConfig := base.ClientConfig{
				Name:                      "gotify_" + name,
				Timeout:                   30 * time.Second,
				AuthType:                  base.AuthNone, // Gotify provider handles auth internally
				RateLimitCalls:            1000,          // Gotify is self-hosted, generous limit
				RateLimitSeconds:          3600,          // 1 hour
				CircuitBreakerThreshold:   3,
				CircuitBreakerTimeout:     30 * time.Second,
				CircuitBreakerHalfOpenMax: 1,
				UserAgent:                 userAgent,
			}
			if provider := gotify.NewProviderWithConfig(
				gotifyConfig,
				host,
				port,
				notifCfg.Apikey,
				useSSL,
			); provider != nil {
				set[name] = provider
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered gotify notification provider with rate limiting")
			}

		case "pushbullet":
			if notifCfg.Apikey == "" {
				break
			}

			if provider := pushbullet.NewProvider(notifCfg.Apikey); provider != nil {
				set[name] = provider
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered pushbullet notification provider")
			}

		case "apprise":
			if notifCfg.ServerURL == "" || notifCfg.AppriseURLs == "" {
				break
			}

			host, port, useSSL := parseServerURL(notifCfg.ServerURL)

			urls := strings.Split(notifCfg.AppriseURLs, ",")
			if provider := apprise.NewProvider(
				host,
				port,
				notifCfg.Apikey,
				urls,
				useSSL,
			); provider != nil {
				set[name] = provider
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered apprise notification provider")
			}

		case "sendmail":
			if notifCfg.SMTPServer == "" || notifCfg.SMTPFromEmail == "" ||
				notifCfg.SMTPToEmail == "" {
				break
			}

			port, _ := strconv.Atoi(notifCfg.SMTPPort)
			if port == 0 {
				port = 587 // Default SMTP submission port
			}

			toEmails := []string{notifCfg.SMTPToEmail}
			if provider := sendmail.NewProvider(notifCfg.SMTPServer, port, notifCfg.SMTPFromEmail, toEmails, notifCfg.SMTPUsername, notifCfg.SMTPPassword); provider != nil {
				set[name] = provider
				logger.Logtype(logger.StatusDebug, 0).
					Str("notification", name).
					Msg("Registered sendmail notification provider")
			}
		}
	})

	return set
}

// SendNotification sends message/title through the registered provider of a
// pushover, gotify, pushbullet or apprise notification entry. The limits are
// those of the respective services.
func SendNotification(cfg *config.NotificationConfig, message, title string) error {
	var (
		key              = cfg.Name
		maxMsg, maxTitle int
		required         map[string]string
		options          map[string]string
	)

	switch cfg.NotificationType {
	case "pushover":
		key = "pushover_" + cfg.Name
		maxMsg, maxTitle = 1024, 250
		required = map[string]string{"apikey": cfg.Apikey}
		options = map[string]string{"api_token": cfg.Apikey, "user_key": cfg.Recipient}
	case "gotify":
		maxMsg, maxTitle = 4096, 256
		required = map[string]string{"server_url": cfg.ServerURL, "apikey": cfg.Apikey}
		options = map[string]string{"server_url": cfg.ServerURL, "token": cfg.Apikey}
	case "pushbullet":
		maxMsg, maxTitle = 8192, 140
		required = map[string]string{"apikey": cfg.Apikey}
		options = map[string]string{"api_token": cfg.Apikey}
	case "apprise":
		required = map[string]string{"server_url": cfg.ServerURL, "apprise_urls": cfg.AppriseURLs}
		options = map[string]string{"server_url": cfg.ServerURL, "urls": cfg.AppriseURLs}
	default:
		return fmt.Errorf("notification type %q: %w", cfg.NotificationType, errNotificationNotRegistered)
	}

	for field, value := range required {
		if value == "" {
			return fmt.Errorf("%s: %w", field, errNotificationField)
		}
	}

	switch {
	case message == "":
		return errMessageEmpty
	case maxMsg > 0 && len(message) > maxMsg:
		return errMessageTooLong
	case maxTitle > 0 && len(title) > maxTitle:
		return errTitleTooLong
	}

	cm, ok := apiexternal_v2.GetGlobalClientManager()
	if !ok {
		return errNotificationNotRegistered
	}

	provider, ok := cm.GetNotificationProvider(key)
	if !ok {
		return errNotificationNotRegistered
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{
		Title:   title,
		Message: message,
		Options: options,
	})

	return err
}

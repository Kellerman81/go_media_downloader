package apiexternal

import (
	"context"
	"errors"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// SendAppriseMessage sends a message via Apprise API server with the given message, title, and URLs.
// The message must not be empty.
// The function returns an error if there is an error sending the message.
//
// It first tries to use a registered v2 apprise provider from the global ClientManager.
// Falls back to creating a legacy client if no provider is registered.
func SendAppriseMessage(configName, serverURL, message, title, urls string) error {
	if serverURL == "" {
		return errors.New("server URL empty")
	}

	if message == "" {
		return errors.New("message empty")
	}

	if urls == "" {
		return errors.New("notification URLs empty")
	}

	// Try v2 provider first
	if cm, exists := apiexternal_v2.GetGlobalClientManager(); exists {
		if provider, providerExists := cm.GetNotificationProvider(configName); providerExists {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Pass server URL and URLs in Options for dynamic credentials
			_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{
				Title:   title,
				Message: message,
				Options: map[string]string{
					"server_url": serverURL,
					"urls":       urls,
				},
			})
			if err == nil {
				return nil
			}
			// Log error but fall through to legacy client
			logger.Logtype("debug", 0).
				Err(err).
				Msg("v2 provider failed, falling back to legacy client")
		}
	}

	return errors.New("client empty")
}

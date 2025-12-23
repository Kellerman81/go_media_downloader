package apiexternal

import (
	"context"
	"errors"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// SendPushbulletMessage sends a Pushbullet push notification with the given message and title.
// The message must not be empty and must be less than 8192 characters.
// The title must be less than 140 characters.
// The function returns an error if the message or title are too long, or if there is an error sending the message.
//
// It first tries to use a registered v2 pushbullet provider from the global ClientManager.
// Falls back to creating a legacy client if no provider is registered.
func SendPushbulletMessage(configName, token, message, title string) error {
	if token == "" {
		return errors.New("token empty")
	}

	if message == "" {
		return errors.New("message empty")
	}

	if len(message) > 8192 {
		return errors.New("message too long")
	}

	if len(title) > 140 {
		return errors.New("title too long")
	}

	// Try v2 provider first
	if cm, exists := apiexternal_v2.GetGlobalClientManager(); exists {
		if provider, providerExists := cm.GetNotificationProvider(configName); providerExists {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			// Pass token in Options for dynamic credentials
			_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{
				Title:   title,
				Message: message,
				Options: map[string]string{
					"api_token": token,
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

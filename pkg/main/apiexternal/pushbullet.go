package apiexternal

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
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
		return errTokenEmpty
	}

	if message == "" {
		return errMessageEmpty
	}

	if len(message) > 8192 {
		return errMessageTooLong
	}

	if len(title) > 140 {
		return errTitleTooLong
	}

	cm, exists := apiexternal_v2.GetGlobalClientManager()
	if !exists {
		return errClientEmpty
	}

	provider, providerExists := cm.GetNotificationProvider(configName)
	if !providerExists {
		return errClientEmpty
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// No "legacy client" fallback exists anymore - returning errClientEmpty
	// here regardless of the real cause discarded the actual failure reason
	// from both the downloader's notify() log output and the "send test
	// notification" UI, which renders this error directly to the user.
	_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{
		Title:   title,
		Message: message,
		Options: map[string]string{
			"api_token": token,
		},
	})

	return err
}

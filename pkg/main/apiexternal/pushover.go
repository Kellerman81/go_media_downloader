package apiexternal

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// SendPushoverMessage sends a Pushover message with the given message, title, and recipient.
// The message must not be empty and must be less than 1024 characters.
// The title must be less than 250 characters.
// The function returns an error if the message or title are too long, or if there is an error sending the message.
//
// It first tries to use a registered v2 pushover provider from the global ClientManager.
// Falls back to creating a legacy client if no provider is registered.
func SendPushoverMessage(cfgname, apikey, message, title, recipient string) error {
	if apikey == "" {
		return errAPIKeyEmpty
	}

	if message == "" {
		return errMessageEmpty
	}

	if len(message) > 1024 {
		return errMessageTooLong
	}

	if len(title) > 250 {
		return errTitleTooLong
	}

	cm, exists := apiexternal_v2.GetGlobalClientManager()
	if !exists {
		return errClientEmpty
	}

	provider, providerExists := cm.GetNotificationProvider("pushover_" + cfgname)
	if !providerExists {
		return errClientEmpty
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Pass apikey and recipient in Options for dynamic credentials.
	// No "legacy client" fallback exists anymore (it never did in this
	// package) - returning errClientEmpty here regardless of the real
	// cause discarded the actual failure reason (bad API key, unreachable
	// server, rate limit, etc.) from both the downloader's notify() log
	// output and the "send test notification" UI, which renders this error
	// directly to the user.
	_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{
		Title:   title,
		Message: message,
		Options: map[string]string{
			"api_token": apikey,
			"user_key":  recipient,
		},
	})

	return err
}

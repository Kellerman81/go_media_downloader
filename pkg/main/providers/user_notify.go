package providers

import (
	"slices"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Messages to accounts: shown in the web interface until dismissed, and
// mailed to accounts with an address through the [[notification]] named
// in user_mail_notification.

// portalURL is the address the web interface was last reached at, for the
// links in mails sent by background jobs.
var portalURL atomic.Value

// SetPortalURL remembers the web interface's address (scheme://host).
func SetPortalURL(u string) {
	if old, _ := portalURL.Load().(string); old != u {
		portalURL.Store(u)
	}
}

// mailTypes are the notification types that send e-mail.
var mailTypes = map[string]bool{"sendmail": true, "mailgun": true, "sendgrid": true}

// pushTypes are the notification types an account can be pushed through,
// with the option that names the account's recipient.
var pushTypes = map[string]string{"ntfy": "topic", "pushover": "user_key", "telegram": "chat_id"}

// The events an account chooses the channels of in My Account.
const (
	EventRequest   = "request"   // its request approved or declined
	EventAvailable = "available" // a title it requested downloaded
	EventReport    = "report"    // a file it reported reviewed or handled
)

// Events are the events in the order My Account lists them.
var Events = []string{EventRequest, EventAvailable, EventReport}

// The channels of a message.
const (
	ChannelWeb  = "web"  // the bell in the web interface
	ChannelMail = "mail" // to the account's e-mail address
	ChannelPush = "push" // to the account's push recipient
)

// Channels are the channels in the order My Account lists them.
var Channels = []string{ChannelWeb, ChannelMail, ChannelPush}

// The account settings of the messages: notify_<event> holds the chosen
// channels comma-separated ("none": none; unset: all), push_target the
// account's push recipient.
const (
	notifySettingPrefix = "notify_"
	notifyNone          = "none"
	PushTargetSetting   = "push_target"
)

// NotifyChannels are the channels an account gets an event's messages on.
func NotifyChannels(username, event string) []string {
	switch v := database.GetUserSetting(username, notifySettingPrefix+event); v {
	case "":
		return Channels
	case notifyNone:
		return nil
	default:
		return strings.Split(v, ",")
	}
}

// SetNotifyChannels stores the channels of an event for an account.
func SetNotifyChannels(username, event string, channels []string) error {
	v := strings.Join(channels, ",")
	if v == "" {
		v = notifyNone
	}

	return database.SetUserSetting(username, notifySettingPrefix+event, v)
}

// PushNotification is the [[notification]] that pushes to accounts and the
// option naming the recipient; nil when none is configured or its type
// can't be sent to an account's own recipient.
func PushNotification() (*config.NotificationConfig, string) {
	cfg := config.GetSettingsNotification(config.GetSettingsGeneral().UserPushNotification)
	if cfg == nil || pushTypes[cfg.NotificationType] == "" {
		return nil, ""
	}

	return cfg, pushTypes[cfg.NotificationType]
}

// MailNotification is the [[notification]] that mails accounts (nil: none).
func MailNotification() *config.NotificationConfig {
	cfg := config.GetSettingsNotification(config.GetSettingsGeneral().UserMailNotification)
	if cfg == nil || !mailTypes[cfg.NotificationType] {
		return nil
	}

	return cfg
}

// NotifyUser tells an account something on the channels it chose for the
// event: a message in the web interface, a mail (with an address and a mail
// notification configured) and a push (with a recipient and a push
// notification configured). link is a page of the web interface
// ("/api/admin/...").
func NotifyUser(username, event, title, message, link string) {
	if username == "" {
		return
	}

	channels := NotifyChannels(username, event)

	if slices.Contains(channels, ChannelWeb) {
		if err := database.AddUserNotification(username, title, message, link); err != nil {
			logger.Logtype("error", 1).Err(err).Str("user", username).Msg("User notification not saved")
		}
	}

	body := message
	if base, _ := portalURL.Load().(string); base != "" && strings.HasPrefix(link, "/") {
		body += "\n\n" + base + link
	}

	if cfg := MailNotification(); cfg != nil && slices.Contains(channels, ChannelMail) {
		if email := database.GetUserEmail(username); email != "" {
			enqueueSend(func() {
				if err := sendNotificationTo(cfg, map[string]string{"to": email}, body, "Go Media: "+title); err != nil {
					logger.Logtype("warn", 0).Err(err).Str("user", username).Msg("User mail not sent")
				}
			})
		}
	}

	if cfg, option := PushNotification(); cfg != nil && slices.Contains(channels, ChannelPush) {
		if target := database.GetUserSetting(username, PushTargetSetting); target != "" {
			enqueueSend(func() {
				if err := sendNotificationTo(cfg, map[string]string{option: target}, body, title); err != nil {
					logger.Logtype("warn", 0).Err(err).Str("user", username).Msg("User push not sent")
				}
			})
		}
	}
}

// SendTestPush pushes a test message to a recipient right away (My
// Account's test button).
func SendTestPush(target, title, message string) error {
	cfg, option := PushNotification()
	if cfg == nil {
		return errNotificationNotRegistered
	}

	return sendNotificationTo(cfg, map[string]string{option: target}, message, title)
}

// sendNotificationTo is SendNotification with options naming another
// recipient ("to" for mail, the pushTypes option for push).
func sendNotificationTo(cfg *config.NotificationConfig, options map[string]string, message, title string) error {
	cm, ok := apiexternal_v2.GetGlobalClientManager()
	if !ok {
		return errNotificationNotRegistered
	}

	provider, ok := cm.GetNotificationProvider(cfg.Name)
	if !ok {
		return errNotificationNotRegistered
	}

	ctx, cancel := sendContext()
	defer cancel()

	_, err := provider.SendNotification(ctx, apiexternal_v2.NotificationRequest{Title: title, Message: message, Options: options})

	return err
}

// RequestMediaArrived tells the accounts that requested a list entry
// (approved requests) that its media was downloaded. kind as the media
// requests (movie, series, album, audiobook, book).
func RequestMediaArrived(kind string, entryID uint) {
	if entryID == 0 {
		return
	}

	for _, r := range database.TakeApprovedRequests(kind, entryID) {
		NotifyUser(r.Username, EventAvailable, "Now available: "+r.Title,
			r.Title+" you requested was downloaded and can be played in the Library.",
			"/api/admin/library#"+kind+"/"+strconv.FormatUint(uint64(entryID), 10))
	}
}

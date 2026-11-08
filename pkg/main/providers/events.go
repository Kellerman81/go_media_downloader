package providers

import (
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

// Notification events a media config's [[...notification]] entries can
// subscribe to.
const (
	EventAddedDownload = "added_download" // a release was sent to a download client
	EventAddedData     = "added_data"     // a file was organized into the library
	EventUpgradedData  = "upgraded_data"  // an organized file replaced older ones
	EventAddedToList   = "added_to_list"  // an entry was added to a movies/series/... list
	EventAddedToDB     = "added_to_db"    // a dbmovies/dbseries/... row was created
)

// NotificationEvents lists every event, in the order the config screens
// offer them.
var NotificationEvents = []string{
	EventAddedDownload, EventAddedData, EventUpgradedData, EventAddedToList, EventAddedToDB,
}

// HasEventNotification reports whether cfgp has a notification for event,
// so callers can skip building template data nobody uses.
func HasEventNotification(cfgp *config.MediaTypeConfig, event string) bool {
	if cfgp == nil {
		return false
	}

	for idx := range cfgp.Notification {
		if strings.EqualFold(cfgp.Notification[idx].Event, event) {
			return true
		}
	}

	return false
}

// eventQueueSize bounds the sends waiting for the sender goroutine. A list
// import can raise thousands of events; past this many, events are dropped
// instead of stalling the import on a rate-limited service.
const eventQueueSize = 1000

var (
	eventQueue     = make(chan func(), eventQueueSize)
	eventQueueOnce sync.Once
)

func enqueueSend(send func()) bool {
	eventQueueOnce.Do(func() {
		go func() {
			for f := range eventQueue {
				f()
			}
		}()
	})

	select {
	case eventQueue <- send:
		return true
	default:
		return false
	}
}

// NotifyEvent renders the title and message of each of cfgp's notifications
// for event with data and delivers them: csv entries are appended right
// away, the others are sent in order by one background sender. prepare, when
// set, runs before each entry is rendered (to set per-entry fields such as
// the replaced prefix).
func NotifyEvent(
	cfgp *config.MediaTypeConfig,
	event string,
	data any,
	prepare func(*config.MediaNotificationConfig),
) {
	if cfgp == nil {
		return
	}

	for idx := range cfgp.Notification {
		entry := &cfgp.Notification[idx]
		if !strings.EqualFold(entry.Event, event) {
			continue
		}

		cfgnot := entry.CfgNotification
		if cfgnot == nil {
			cfgnot = config.GetSettingsNotification(entry.MapNotification)
		}

		if cfgnot == nil {
			continue
		}

		if prepare != nil {
			prepare(entry)
		}

		bl, message, _ := logger.ParseStringTemplate(entry.Message, data)
		if bl {
			continue
		}

		if cfgnot.NotificationType == "csv" {
			scanner.AppendCsv(cfgnot.Outputto, message)
			continue
		}

		if !IsSentNotification(cfgnot.NotificationType) {
			continue
		}

		bl, title, _ := logger.ParseStringTemplate(entry.Title, data)
		if bl {
			continue
		}

		queued := enqueueSend(func() {
			if err := SendNotification(cfgnot, message, title); err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Str("type", cfgnot.NotificationType).
					Str("event", event).
					Msg("Error sending notification")

				return
			}

			logger.Logtype("info", 0).
				Str("type", cfgnot.NotificationType).
				Str("event", event).
				Msg("Notification sent")
		})
		if !queued {
			logger.Logtype("warn", 0).
				Str("notification", cfgnot.Name).
				Str("event", event).
				Msg("Notification queue full - notification dropped")
		}
	}
}

package api

import (
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
)

// accountEvent is the template data of the media_requested and
// media_reported notification events: what an account asks an admin for.
type accountEvent struct {
	Event         string // media_requested or media_reported
	Time          string
	Configuration string // the media group, e.g. "EN"
	List          string
	User          string
	Type          string // movie, series, album, audiobook, book
	Title         string
	Reason        string // reports: the reason's label
	Note          string // reports: the account's note
	File          string // reports: the file name
	URL           string // the Requests or Reports page
}

// mediaConfigOfList is the media group a list belongs to (nil: none).
func mediaConfigOfList(listname string) *config.MediaTypeConfig {
	var found *config.MediaTypeConfig

	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		if found == nil && slices.ContainsFunc(cfgp.Lists, func(l config.MediaListsConfig) bool { return strings.EqualFold(l.Name, listname) }) {
			found = cfgp
		}

		return nil
	})

	return found
}

// adminPageURL is the address of an admin page as the account's browser
// reached the server (behind a proxy: its forwarded host and scheme).
func adminPageURL(c *gin.Context, path string) string {
	scheme := "http"
	if c.Request.TLS != nil || strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") {
		scheme = "https"
	}

	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.Request.Host
	}

	return scheme + "://" + host + path
}

// notifyAccountEvent sends the notifications of the list's media group for
// an account's request or report.
func notifyAccountEvent(c *gin.Context, event, page string, e *accountEvent) {
	cfgp := mediaConfigOfList(e.List)
	if !providers.HasEventNotification(cfgp, event) {
		return
	}

	e.Event, e.Configuration, e.URL = event, cfgp.Name, adminPageURL(c, page)
	e.Time = logger.TimeGetNow().Format(logger.GetTimeFormat())

	providers.NotifyEvent(cfgp, event, e, nil)
}

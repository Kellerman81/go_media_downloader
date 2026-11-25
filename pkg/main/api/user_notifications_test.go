package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
)

// TestAccountMessages: an account saves which channels each of its events
// reaches it on and its push recipient; the card offers only its events,
// and push only with a push notification configured.
func TestAccountMessages(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	if err := database.ExecNErr("insert into users (username, password_hash, role) values ('ann', 'x', 'requestinguser')"); err != nil {
		t.Fatal(err)
	}

	session := &Session{UserID: "ann", Role: database.RoleRequestingUser, Groups: []string{"movie_EN"}, Features: []string{database.FeatureRequest}}

	post := func(form url.Values) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Set("session", session)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/account/messages", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		HandleAccountMessages(c)

		return w.Body.String()
	}

	// No mail notification set up: no mail column, no hint about it.
	card := renderComponentToString(renderMessagesForm("", session.access(), ""))
	if !strings.Contains(card, "Requested title downloaded") || strings.Contains(card, "Reported file handled") ||
		strings.Contains(card, `value="mail"`) || strings.Contains(card, "e-mail address") ||
		strings.Contains(card, `value="push"`) || strings.Contains(card, `name="push_target"`) {
		t.Errorf("card without push, reports:\n%s", card)
	}

	// The update reloads the config: the general setting after it.
	if err := config.UpdateCfgEntryAny([]config.NotificationConfig{
		{Name: "Push", NotificationType: "ntfy", Recipient: "admin-topic"},
		{Name: "Mail", NotificationType: "sendmail"},
	}); err != nil {
		t.Fatal(err)
	}

	g := config.GetSettingsGeneral()
	old, oldMail := g.UserPushNotification, g.UserMailNotification
	g.UserPushNotification, g.UserMailNotification = "Push", "Mail"

	t.Cleanup(func() { g.UserPushNotification, g.UserMailNotification = old, oldMail })

	// Mail set up, no address yet: the column is there but greyed out.
	card = renderComponentToString(renderMessagesForm("", session.access(), ""))
	if !regexp.MustCompile(`name="request" value="mail"[^>]*checked disabled`).MatchString(card) ||
		!strings.Contains(card, "Mail needs an e-mail address above.") {
		t.Errorf("card with mail, no address:\n%s", card)
	}

	out := post(url.Values{providers.EventRequest: {"web", "push", "bogus"}, "push_target": {" ann-topic "}})
	if !strings.Contains(out, "Saved.") || !strings.Contains(out, `value="ann-topic"`) || !strings.Contains(out, "ntfy topic") {
		t.Errorf("save:\n%s", out)
	}

	// Mail can't be chosen without an address: its stored choice (unset: on) stays.
	if got := providers.NotifyChannels("ann", providers.EventRequest); !slices.Equal(got, []string{"web", "push", "mail"}) {
		t.Errorf("request channels: %v", got)
	}

	if got := providers.NotifyChannels("ann", providers.EventAvailable); !slices.Equal(got, []string{"mail"}) {
		t.Errorf("unticked event: %v", got)
	}

	if got := providers.NotifyChannels("ann", providers.EventReport); len(got) != len(providers.Channels) {
		t.Errorf("event the account doesn't have changed: %v", got)
	}
}

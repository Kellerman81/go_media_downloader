package providers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestRequestMediaArrived: the account that requested a title gets a
// message with a link to it once its media arrived, once.
func TestRequestMediaArrived(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	for _, q := range []string{
		"insert into media_requests (username, kind, entry_id, listname, title, status) values ('bea', 'series', 3, 'L', 'Show', 'approved')",
		"insert into media_requests (username, kind, entry_id, listname, title, status) values ('cid', 'series', 3, 'L', 'Show', 'open')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	RequestMediaArrived("series", 3)
	RequestMediaArrived("series", 3)

	got := database.OpenUserNotifications("bea")
	if len(got) != 1 || got[0].Title != "Now available: Show" || got[0].Link != "/api/admin/library#series/3" {
		t.Errorf("bea's messages: %+v", got)
	}

	if len(database.OpenUserNotifications("cid")) != 0 {
		t.Error("an open (not approved) request was told")
	}
}

// TestNotifyUserChannels: an account gets an event's messages on the
// channels it chose (unset: all); push goes to its own ntfy topic.
func TestNotifyUserChannels(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	topics := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var msg struct{ Topic, Title string }

		json.NewDecoder(r.Body).Decode(&msg)
		topics <- msg.Topic + " " + msg.Title
	}))
	t.Cleanup(srv.Close)

	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")

	oldCM, _ := apiexternal_v2.GetGlobalClientManager()
	apiexternal_v2.SetGlobalClientManager(apiexternal_v2.NewClientManager())

	t.Cleanup(func() {
		config.Configfile = oldFile
		apiexternal_v2.SetGlobalClientManager(oldCM)
	})

	loadTestConfig(t, config.Configfile, &config.MainConfig{
		Notification: []config.NotificationConfig{{Name: "Push", NotificationType: "ntfy", ServerURL: srv.URL, Recipient: "admin-topic"}},
	})
	RegisterNotificationProviders()

	config.GetSettingsGeneral().UserPushNotification = "Push"

	if err := database.ExecNErr("insert into users (username, password_hash, role) values ('ann', 'x', 'requestinguser')"); err != nil {
		t.Fatal(err)
	}

	if got := NotifyChannels("ann", EventRequest); !slices.Equal(got, Channels) {
		t.Errorf("unset: %v, want all", got)
	}

	// Push without a recipient sends nothing; with one, to the account's topic.
	NotifyUser("ann", EventRequest, "Request approved: A", "A was approved.", "")

	if err := database.SetUserSetting("ann", PushTargetSetting, "ann-topic"); err != nil {
		t.Fatal(err)
	}

	NotifyUser("ann", EventRequest, "Request approved: B", "B was approved.", "")

	select {
	case got := <-topics:
		if got != "ann-topic Request approved: B" {
			t.Errorf("push: %q", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no push sent")
	}

	// Only the bell: no push.
	if err := SetNotifyChannels("ann", EventReport, []string{ChannelWeb}); err != nil {
		t.Fatal(err)
	}

	NotifyUser("ann", EventReport, "Report handled: C", "C was handled.", "")

	// Nothing at all: not even the bell.
	if err := SetNotifyChannels("ann", EventAvailable, nil); err != nil {
		t.Fatal(err)
	}

	if got := NotifyChannels("ann", EventAvailable); got != nil {
		t.Errorf("none: %v", got)
	}

	NotifyUser("ann", EventAvailable, "Now available: D", "D arrived.", "")

	select {
	case got := <-topics:
		t.Errorf("pushed although not chosen: %q", got)
	case <-time.After(300 * time.Millisecond):
	}

	var titles []string
	for _, n := range database.OpenUserNotifications("ann") {
		titles = append(titles, n.Title)
	}

	slices.Sort(titles)

	if want := []string{"Report handled: C", "Request approved: A", "Request approved: B"}; !slices.Equal(titles, want) {
		t.Errorf("bell: %v, want %v", titles, want)
	}

	if err := SendTestPush("my-topic", "Go Media", "test"); err != nil {
		t.Fatal(err)
	}

	if got := <-topics; got != "my-topic Go Media" {
		t.Errorf("test push: %q", got)
	}
}

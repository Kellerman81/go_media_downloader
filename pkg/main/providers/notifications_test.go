package providers

import (
	"errors"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestRegisterNotificationProvidersFollowsConfig(t *testing.T) {
	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")

	oldCM, _ := apiexternal_v2.GetGlobalClientManager()
	cm := apiexternal_v2.NewClientManager()
	apiexternal_v2.SetGlobalClientManager(cm)

	t.Cleanup(func() {
		config.Configfile = oldFile
		apiexternal_v2.SetGlobalClientManager(oldCM)
	})

	names := func() []string {
		return slices.Sorted(maps.Keys(cm.GetAllNotificationProviders()))
	}

	loadTestConfig(t, config.Configfile, &config.MainConfig{
		Notification: []config.NotificationConfig{
			{Name: "pb", NotificationType: "pushbullet", Apikey: "k1"},
			{Name: "po", NotificationType: "pushover", Apikey: "k2", Recipient: "u"},
			{Name: "got", NotificationType: "gotify", Apikey: "k3", ServerURL: "http://gotify.invalid"},
			{Name: "broken", NotificationType: "gotify"},
		},
	})
	RegisterNotificationProviders()

	if got, want := names(), []string{"got", "pb", "po"}; !slices.Equal(got, want) {
		t.Fatalf("after first load: got %v, want %v", got, want)
	}

	before, _ := cm.GetNotificationProvider("got")

	loadTestConfig(t, config.Configfile, &config.MainConfig{
		Notification: []config.NotificationConfig{
			{Name: "got", NotificationType: "gotify", Apikey: "k4", ServerURL: "http://other.invalid"},
		},
	})
	RegisterNotificationProviders()

	if got, want := names(), []string{"got"}; !slices.Equal(got, want) {
		t.Fatalf("after reload: got %v, want %v", got, want)
	}

	if after, _ := cm.GetNotificationProvider("got"); after == before {
		t.Fatal("changed gotify entry kept its old provider")
	}
}

func TestSendNotificationValidation(t *testing.T) {
	oldCM, _ := apiexternal_v2.GetGlobalClientManager()
	apiexternal_v2.SetGlobalClientManager(apiexternal_v2.NewClientManager())
	t.Cleanup(func() { apiexternal_v2.SetGlobalClientManager(oldCM) })

	long := strings.Repeat("x", 1025)

	for _, tt := range []struct {
		name           string
		cfg            config.NotificationConfig
		message, title string
		want           error
	}{
		{"csv is not sent", config.NotificationConfig{NotificationType: "csv"}, "m", "t", errNotificationNotRegistered},
		{"pushover without key", config.NotificationConfig{NotificationType: "pushover"}, "m", "t", ErrNotificationField},
		{"gotify without url", config.NotificationConfig{NotificationType: "gotify", Apikey: "k"}, "m", "t", ErrNotificationField},
		{"empty message", config.NotificationConfig{NotificationType: "pushbullet", Apikey: "k"}, "", "t", errMessageEmpty},
		{"pushover without user key", config.NotificationConfig{NotificationType: "pushover", Apikey: "k"}, "m", "t", ErrNotificationField},
		{"pushover message limit", config.NotificationConfig{NotificationType: "pushover", Apikey: "k", Recipient: "u"}, long, "t", errMessageTooLong},
		{"pushbullet title limit", config.NotificationConfig{NotificationType: "pushbullet", Apikey: "k"}, "m", long[:141], errTitleTooLong},
		{"apprise has no length limit", config.NotificationConfig{Name: "a", NotificationType: "apprise", ServerURL: "http://x", AppriseURLs: "u"}, long, long, errNotificationNotRegistered},
	} {
		if err := SendNotification(&tt.cfg, tt.message, tt.title); !errors.Is(err, tt.want) {
			t.Errorf("%s: err = %v, want %v", tt.name, err, tt.want)
		}
	}
}

func TestNewNotificationProviderEveryType(t *testing.T) {
	full := config.NotificationConfig{
		Name: "n", Apikey: "k", Recipient: "r", Sender: "+491", Outputto: "/tmp/x.csv", Command: "/bin/true",
		ServerURL: "https://example.invalid/hook", AppriseURLs: "json://x",
		SMTPServer: "smtp.invalid", SMTPFromEmail: "a@example.invalid", SMTPToEmail: "b@example.invalid, c@example.invalid",
	}

	for _, name := range NotificationTypeNames() {
		cfg := full
		cfg.NotificationType = name

		p, err := NewNotificationProvider(&cfg)
		if name == "csv" {
			if err == nil || IsSentNotification(name) {
				t.Error("csv must not build a provider")
			}

			continue
		}

		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}

		if string(p.GetProviderType()) != name {
			t.Errorf("%s: built a %s provider", name, p.GetProviderType())
		}

		empty := config.NotificationConfig{Name: "n", NotificationType: name}
		if _, err := NewNotificationProvider(&empty); !errors.Is(err, ErrNotificationField) && name != "ntfy" {
			t.Errorf("%s without settings: %v", name, err)
		}
	}
}

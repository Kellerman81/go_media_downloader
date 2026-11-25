package api

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestHandleAccountSetting: a known setting is stored in the account's
// settings, unknown keys and values are refused.
func TestHandleAccountSetting(t *testing.T) {
	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	s := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}

	for _, tc := range []struct {
		key, value string
		want       int
	}{
		{"theme", "dark", http.StatusOK}, // 204 is only written when the request ends
		{"theme", "purple", http.StatusBadRequest},
		{"lp-vol", "1", http.StatusBadRequest},
	} {
		w := callAs(s, HandleAccountSetting, url.Values{"key": {tc.key}, "value": {tc.value}})
		if w.Code != tc.want {
			t.Errorf("%s=%s: status %d, want %d", tc.key, tc.value, w.Code, tc.want)
		}
	}

	if got := database.GetUserSettings(DefaultUsername); got != `{"theme":"dark"}` {
		t.Errorf("stored settings: %s", got)
	}
}

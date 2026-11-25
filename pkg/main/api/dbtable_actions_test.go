package api

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"strings"
	"testing"
)

// Every proxied row button must pass the admin proxy allowlist, and every
// navigate button must point at an admin database page.
func TestTableActionsReachable(t *testing.T) {
	for table, actions := range tableActions {
		for _, a := range actions {
			url := strings.NewReplacer("{id}", "42", "{listname}", "My%20List").Replace(a.URL)
			path, _, _ := strings.Cut(url, "?")

			switch a.Kind {
			case actionJob, actionSearch:
				if _, ok := resolveProxyMethod(path); !ok {
					t.Errorf("%s %q: %s is not allowed by the admin proxy", table, a.Label, path)
				}
			case actionNavigate:
				if !strings.HasPrefix(path, "/api/admin/database/") {
					t.Errorf("%s %q: navigates to %s", table, a.Label, path)
				}
			default:
				t.Errorf("%s %q: unknown kind %q", table, a.Label, a.Kind)
			}

			if a.Label == "" || a.Icon == "" || a.Color == "" || a.Help == "" {
				t.Errorf("%s %q: label, icon, color and help are required", table, a.Label)
			}
		}
	}
}

func TestTableActionHelp(t *testing.T) {
	if got := tableActionHelp("no_such_table"); got != "Edit: Edit record | Delete: Remove record" {
		t.Errorf("default help = %q", got)
	}

	if got := tableActionHelp("series"); !strings.Contains(got, "RSS: One RSS search per season") {
		t.Errorf("series help = %q", got)
	}
}

// TestTranslationTables: every metadata_translations media type links to
// the table it comes from.
func TestTranslationTables(t *testing.T) {
	for mediaType := range database.TranslationFields {
		table, ok := translationTables[mediaType]
		if !ok {
			t.Errorf("no table for media type %s", mediaType)
			continue
		}

		if got := database.TranslationMediaType(table); got != mediaType {
			t.Errorf("table %s maps back to %q, want %q", table, got, mediaType)
		}
	}
}

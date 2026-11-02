package database

import (
	"slices"
	"testing"
	"time"
)

func TestTimeFromName(t *testing.T) {
	want := time.Date(2026, 3, 1, 12, 30, 5, 0, time.UTC)

	for name, w := range map[string]time.Time{
		"data.db.34.20260301_123005": want, // current form: version, then timestamp
		"data.db.20260301_123005":    want,
		"DATA.DB.34.20260301_123005": want,
		"data.db.34.garbage":         {},
		"data.db.20260301":           {},
		"imdb.db.20260301_123005":    {},
	} {
		if got := timeFromName(name, "data.db."); !got.Equal(w) {
			t.Errorf("timeFromName(%q) = %v, want %v", name, got, w)
		}
	}
}

func TestGetSettingTemplatesFor(t *testing.T) {
	prev := GetDBConnect()
	t.Cleanup(func() { SetDBConnect(prev) })

	next := prev
	next.GetqualitiesIn = []Qualities{{Name: "BluRay"}, {Name: "WEBDL"}}
	next.GetresolutionsIn = []Qualities{{Name: "1080p"}}
	next.GetaudiosIn = nil
	next.GetcodecsIn = []Qualities{{Name: "x265"}}
	SetDBConnect(next)

	tests := map[string][]string{
		"quality":    {"", "BluRay", "WEBDL"},
		"resolution": {"", "1080p"},
		"audio":      {""},
		"codec":      {"", "x265"},
	}

	for key, want := range tests {
		if got := GetSettingTemplatesFor(key)["options"]; !slices.Equal(got, want) {
			t.Errorf("GetSettingTemplatesFor(%q) = %v, want %v", key, got, want)
		}
	}

	if GetSettingTemplatesFor("other") != nil {
		t.Error("unknown key returned options")
	}
}

func TestPopulateAllSlugsFixesMissingAndStale(t *testing.T) {
	db := useMigratedMainDB(t)

	mustExec(t, db, "insert into dbbooks (id, title, slug) values (1, 'Wilde Wellen', ''), (2, 'Гарри Поттер', 'Garri-Potter'), (3, '!!!', '')")
	mustExec(t, db, "insert into dbartists (id, name, slug) values (1, 'Harry Styles', null)")

	PopulateAllSlugs(false)

	var books []string
	if err := db.Select(&books, "select slug from dbbooks order by id"); err != nil {
		t.Fatal(err)
	}

	want := []string{"wilde-wellen", "garri-potter", ""}
	for i := range want {
		if books[i] != want[i] {
			t.Errorf("dbbooks %d slug = %q, want %q", i+1, books[i], want[i])
		}
	}

	var artist string
	if err := db.Get(&artist, "select slug from dbartists where id = 1"); err != nil || artist != "harry-styles" {
		t.Errorf("dbartists slug = %q (%v), want harry-styles", artist, err)
	}
}

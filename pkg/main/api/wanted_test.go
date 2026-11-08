package api

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
)

func TestEscapeLikePattern(t *testing.T) {
	for in, want := range map[string]string{
		"plain":  "plain",
		"100%":   `100\%`,
		"a_b":    `a\_b`,
		`back\s`: `back\\s`,
	} {
		if got := escapeLikePattern(in); got != want {
			t.Errorf("escapeLikePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

// useWantedConfig loads a config with movie groups EN (lists "EN Hallmark",
// disabled "Solar Web") and DE ("DE Hallmark"), and a migrated test DB.
func useWantedConfig(t *testing.T) {
	t.Helper()

	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { config.Configfile = oldFile })

	group := func(name string, lists ...config.MediaListsConfig) config.MediaTypeConfig {
		for i := range lists {
			lists[i].TemplateList, lists[i].TemplateQuality = "listA", "HD"
		}

		return config.MediaTypeConfig{
			Name: name, TemplateQuality: "HD", TemplateScheduler: "Default",
			Data:  []config.MediaDataConfig{{TemplatePath: "alpha"}},
			Lists: lists,
		}
	}

	cfg := config.MainConfig{
		General:   config.GeneralConfig{WorkerFiles: 1, WorkerParse: 1, WorkerSearch: 1},
		Lists:     []config.ListsConfig{{Name: "listA"}},
		Paths:     []config.PathsConfig{{Name: "alpha"}},
		Quality:   []config.QualityConfig{{Name: "HD"}},
		Scheduler: []config.SchedulerConfig{{Name: "Default", IntervalImdb: "3d"}},
		Media: config.MediaConfig{Movies: []config.MediaTypeConfig{
			group("EN", config.MediaListsConfig{Name: "EN Hallmark", Enabled: true}, config.MediaListsConfig{Name: "Solar Web"}),
			group("DE", config.MediaListsConfig{Name: "DE Hallmark", Enabled: true}),
		}},
	}

	cnt, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(config.Configfile, cnt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.Loadallsettings(true); err != nil {
		t.Fatal(err)
	}

	for _, q := range []string{
		"insert into dbmovies (id, title, year) values (1, 'Gideon''s Trumpet', 1980), (2, 'Special Delivery', 2000)",
		"insert into movies (dbmovie_id, listname, missing) values (1, 'EN Hallmark', 1), (1, 'DE Hallmark', 1), " +
			"(2, 'Solar Web', 1), (2, 'Old List', 1), (2, 'EN Hallmark', 0)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}
}

func wantedTabBody(t *testing.T, query string) string {
	t.Helper()

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/admin/wanted/tab?media=movie"+query, nil)

	renderWantedTab(ctx)

	return w.Body.String()
}

func TestWantedTabMultipleLists(t *testing.T) {
	useWantedConfig(t)

	movies, _ := findWantedSection("movie")

	for _, tc := range []struct {
		f    wantedFilter
		want uint
	}{
		{wantedFilter{}, 4},
		{wantedFilter{Group: "EN"}, 2},
		{wantedFilter{Group: "DE"}, 1},
		{wantedFilter{List: "solar web"}, 1},
		{wantedFilter{Group: "DE", List: "EN Hallmark"}, 0},
		{wantedFilter{Group: "nope"}, 0},
		{wantedFilter{Search: "gideon", Group: "EN"}, 1},
	} {
		q, args := movies.countQuery(tc.f)
		if got := database.Getdatarow[uint](false, q, args...); got != tc.want {
			t.Errorf("%+v: %d rows, want %d", tc.f, got, tc.want)
		}
	}

	body := wantedTabBody(t, "")
	for _, want := range []string{
		"Gideon&#39;s Trumpet (1980)", "Media Group", ">EN<", ">DE<", "DE Hallmark",
		"Solar Web <span class=\"badge bg-secondary\">disabled</span>",
		"Old List <span class=\"badge bg-warning text-dark\">not configured</span>",
		"Showing 1–4 of 4",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("tab body lacks %q", want)
		}
	}

	if body := wantedTabBody(t, "&group=DE"); strings.Contains(body, "Special Delivery") || !strings.Contains(body, "Showing 1–1 of 1") {
		t.Errorf("group filter not applied: %s", body)
	}
}

func TestWantedPaginationKeepsFilters(t *testing.T) {
	var b strings.Builder

	wantedPagination("movie", wantedFilter{Search: "a b", Group: "EN", List: "EN Hallmark"}, 1, 3).Render(&b)

	if !strings.Contains(b.String(), "q=a+b&amp;group=EN&amp;list=EN+Hallmark&amp;page=2") {
		t.Errorf("pagination drops filters: %s", b.String())
	}
}

package api

import (
	"math/bits"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
)

func TestSeriesConfigPath(t *testing.T) {
	for _, bad := range []string{
		"", "../config.toml", "config.toml", "sub/series.toml", `..\x.toml`, "series.txt", "/etc/x.toml",
	} {
		if _, err := seriesConfigPath(bad); err == nil {
			t.Errorf("seriesConfigPath(%q) accepted", bad)
		}
	}

	if _, err := seriesConfigPath("series.toml"); err != nil {
		t.Errorf("seriesConfigPath(series.toml) rejected: %v", err)
	}
}

func TestMarshalSeriesConfig(t *testing.T) {
	in := &config.MainManualConfig{Config: []config.ManualConfig{
		{Name: "12 Monkeys", TvdbID: 272644},
		{Name: "Atlantis 2013", TvdbID: 248596, AlternateName: []string{"Atlantis (2013)"}, DontSearch: true, Source: "tvdb"},
	}}

	out, err := marshalSeriesConfig(in, map[string]any{"global": map[string]any{"identifiedby": "ep"}})
	if err != nil {
		t.Fatal(err)
	}

	text := string(out)
	want := "[global]\nidentifiedby = 'ep'\n" +
		"[[config]]\nname = '12 Monkeys'\ntvdb_id = 272644\n" +
		"[[config]]\nname = 'Atlantis 2013'\ntvdb_id = 248596\nalternatename = ['Atlantis (2013)']\ndont_search = true\nsource = 'tvdb'\n"

	if !strings.HasPrefix(text, "[global]") || !strings.Contains(text, want[strings.Index(want, "[[config]]"):]) {
		t.Errorf("got:\n%s\nwant entries:\n%s", text, want)
	}

	var back config.MainManualConfig
	if err := toml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(back.Config, in.Config) {
		t.Errorf("round trip:\n%+v\n%+v", back.Config, in.Config)
	}
}

func TestSeriesConfigSaveKeepsDefaultsAndOverrides(t *testing.T) {
	file := []byte(`[global]
identifiedby = 'date'
search = false
[[config]]
name = 'Unset'
[[config]]
name = 'Override'
dont_search = false
`)

	cfg, err := config.DecodeManualConfig(file)
	if err != nil {
		t.Fatal(err)
	}

	// Saving unchanged keeps the explicit false and adds nothing to the
	// series that follows [global].
	out, err := marshalSeriesConfig(cfg, map[string]any{"global": map[string]any{"identifiedby": "date", "search": false}})
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(out), "[[config]]\nname = 'Unset'\n[[config]]\nname = 'Override'\ndont_search = false\n") {
		t.Errorf("saved:\n%s", out)
	}

	back, err := config.DecodeManualConfig(out)
	if err != nil {
		t.Fatal(err)
	}

	if !back.Config[0].DontSearch || back.Config[1].DontSearch || back.Config[0].Identifiedby != "date" {
		t.Errorf("after save: %+v", back.Config)
	}

	// Editing: a value differing from the default becomes explicit, one equal
	// to it stays unset.
	edited := config.ManualConfig{Name: "Unset", Identifiedby: "ep", DontSearch: true}
	markSeriesOverrides(&edited, &cfg.Global)

	if !edited.Explicit.Has("identifiedby") || edited.Explicit.Has("dont_search") {
		t.Errorf("explicit after edit = %v", edited.Explicit)
	}

	// Built-in defaults posted back for an unset field stay unset.
	plain := config.ManualConfig{Name: "P", Source: "tvdb", Identifiedby: "ep"}
	markSeriesOverrides(&plain, &config.ManualGlobalConfig{})

	if plain.Source != "" || plain.Identifiedby != "" {
		t.Errorf("built-in defaults written: %+v", plain)
	}
}

func TestHandleSeriesConfigGlobal(t *testing.T) {
	// The config directory is ./config, relative to the working directory.
	t.Chdir(t.TempDir())

	if err := os.Mkdir("config", 0o700); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join("config", "series_t.toml")
	if err := os.WriteFile(file, []byte(`[global]
identifiedby = 'ep'
search = true
custom = 'kept'
[[config]]
name = 'Follows'
[[config]]
name = 'Own'
identifiedby = 'ep'
dont_search = false
`), 0o600); err != nil {
		t.Fatal(err)
	}

	post := func(form url.Values) {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		handleSeriesConfigGlobal(ctx)

		// A redirect without body leaves the status in gin's writer.
		if ctx.Writer.Status() != http.StatusSeeOther {
			t.Fatalf("status %d: %s", ctx.Writer.Status(), w.Body.String())
		}
	}

	post(url.Values{"file": {"series_t.toml"}, "identifiedby": {"date"}, "search": {"off"}, "upgrade": {""}})

	raw, _ := os.ReadFile(file)

	cfg, err := config.DecodeManualConfig(raw)
	if err != nil {
		t.Fatal(err)
	}

	follows, own := cfg.Config[0], cfg.Config[1]

	// The series without own values follow the new defaults (the old ones
	// weren't written into it); explicit values stay.
	if follows.Identifiedby != "date" || !follows.DontSearch || bits.OnesCount64(uint64(follows.Explicit)) != 1 {
		t.Errorf("follows = %+v\n%s", follows, raw)
	}

	if own.Identifiedby != "ep" || own.DontSearch {
		t.Errorf("own = %+v", own)
	}

	if cfg.Global.Upgrade != nil || !strings.Contains(string(raw), "custom = 'kept'") {
		t.Errorf("global = %+v\n%s", cfg.Global, raw)
	}

	// Clearing all defaults keeps only the unknown key.
	post(url.Values{"file": {"series_t.toml"}})

	raw, _ = os.ReadFile(file)
	if strings.Contains(string(raw), "identifiedby = 'date'") || !strings.Contains(string(raw), "[global]\ncustom = 'kept'") {
		t.Errorf("after clearing:\n%s", raw)
	}
}

func TestHandleSeriesConfigCreate(t *testing.T) {
	t.Chdir(t.TempDir())

	if err := os.Mkdir("config", 0o700); err != nil {
		t.Fatal(err)
	}

	post := func(name, kind string) int {
		w := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(w)
		form := url.Values{"name": {name}, "kind": {kind}}
		ctx.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		ctx.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

		handleSeriesConfigCreate(ctx)

		return ctx.Writer.Status()
	}

	if got := post("audio_new", "audiobook"); got != http.StatusSeeOther {
		t.Fatalf("create: status %d", got)
	}

	for _, bad := range [][2]string{
		{"audio_new.toml", "audiobook"}, // exists, never overwritten
		{"../escape", "series"},
		{"config", "series"}, // config.toml
		{"other", "movie"},   // no such kind
	} {
		if got := post(bad[0], bad[1]); got != http.StatusBadRequest {
			t.Errorf("%v: status %d, want 400", bad, got)
		}
	}

	if _, err := os.Stat(filepath.Join("config", "other.toml")); err == nil {
		t.Error("file of an unknown kind created")
	}

	// The empty file is listed with its kind although no list imports it.
	files, err := getSeriesConfigFiles()
	if err != nil || len(files) != 1 || files[0].Name != "audio_new.toml" || files[0].Kind != "audiobook" || files[0].Count != 0 {
		t.Fatalf("files = %+v, %v", files, err)
	}

	if got := manualConfigFileSuggestions(); !reflect.DeepEqual(got, []string{"./config/audio_new.toml"}) {
		t.Errorf("suggestions = %v", got)
	}

	// Saving entries keeps the kind marker.
	if err := saveSeriesConfigFile("audio_new.toml", &config.MainManualConfig{
		Config: []config.ManualConfig{{Name: "Some Author"}},
	}); err != nil {
		t.Fatal(err)
	}

	raw, _ := os.ReadFile(filepath.Join("config", "audio_new.toml"))
	if !strings.HasPrefix(string(raw), manualKindMarker+"audiobook\n") || !strings.Contains(string(raw), "name = 'Some Author'") {
		t.Errorf("saved file:\n%s", raw)
	}

	if files, _ := getSeriesConfigFiles(); len(files) != 1 || files[0].Kind != "audiobook" || files[0].Count != 1 {
		t.Errorf("after save: %+v", files)
	}

	var b strings.Builder
	if err := renderSeriesConfigContent(files, "audio_new.toml", &config.MainManualConfig{}, "tok").Render(&b); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{`action="/api/admin/seriesconfig/create"`, "No list imports this file yet", "<code>audiobookconfig</code>", "./config/audio_new.toml"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("page lacks %s", want)
		}
	}
}

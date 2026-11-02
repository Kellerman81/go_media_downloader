package providers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/qbittorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sabnzbd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/pelletier/go-toml/v2"
)

// loadTestConfig writes cfg, completed with the minimal sections a config
// must have to validate, as the config file and loads it.
func loadTestConfig(t *testing.T, file string, cfg *config.MainConfig) {
	t.Helper()

	cfg.General = config.GeneralConfig{WorkerFiles: 1, WorkerParse: 1, WorkerSearch: 1}
	cfg.Lists = []config.ListsConfig{{Name: "listA"}}
	cfg.Paths = []config.PathsConfig{{Name: "alpha"}}
	cfg.Quality = []config.QualityConfig{{Name: "HD"}}
	cfg.Scheduler = []config.SchedulerConfig{{Name: "Default", IntervalImdb: "3d"}}
	cfg.Media = config.MediaConfig{Movies: []config.MediaTypeConfig{{
		Name:              "M",
		TemplateQuality:   "HD",
		TemplateScheduler: "Default",
		Data:              []config.MediaDataConfig{{TemplatePath: "alpha"}},
		Lists:             []config.MediaListsConfig{{Name: "M-list", TemplateList: "listA", TemplateQuality: "HD"}},
	}}}

	cnt, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(file, cnt, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := config.Loadallsettings(true); err != nil {
		t.Fatalf("load: %v", err)
	}
}

// downloadClients lists the registered download clients without the
// indexers' NZB download clients.
func downloadClients() map[string]any {
	all := GetAllDownloadProviders()
	for name := range all {
		if strings.HasPrefix(name, "newznab_") {
			delete(all, name)
		}
	}

	return all
}

func TestReloadClientProvidersFollowsConfig(t *testing.T) {
	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")

	t.Cleanup(func() {
		config.Configfile = oldFile

		ReplaceDownloadProviders(DownloadProviders{})

		for name := range GetAllIndexers() {
			RemoveIndexer(name)
		}
	})

	// Disabled indexers skip the provider's capability fetch, so no network.
	idxA := config.IndexersConfig{Name: "idxA", IndexerType: "newznab", URL: "http://a.invalid", Apikey: "k"}
	idxB := config.IndexersConfig{Name: "idxB", IndexerType: "newznab", URL: "http://b.invalid", Apikey: "k"}

	loadTestConfig(t, config.Configfile, &config.MainConfig{
		Downloader: []config.DownloaderConfig{
			{Name: "qb", DlType: "qbittorrent", Hostname: "https://h/qbit", Enabled: true},
			{Name: "sab", DlType: "sabnzbd", Hostname: "h", Port: 8085, Password: "key", Enabled: true},
			{Name: "sabnokey", DlType: "sabnzbd", Hostname: "h", Enabled: true},
			{Name: "off", DlType: "nzbget", Hostname: "h", Enabled: false},
			{Name: "drone", DlType: "drone", Hostname: "h", Enabled: true},
		},
		Indexers: []config.IndexersConfig{idxA, idxB},
	})

	ReloadClientProviders()

	all := downloadClients()
	if len(all) != 2 || GetQBittorrent("qb") == nil || GetSABnzbd("sab") == nil {
		t.Fatalf("after first load: %v, want qbittorrent_qb and sabnzbd_sab", all)
	}

	oldQB := GetQBittorrent("qb")
	keptIdx := GetIndexer("idxA")

	if keptIdx == nil || GetIndexer("idxB") == nil {
		t.Fatalf("indexers not registered: %v", GetAllIndexers())
	}

	// Change qb's type, drop sab, enable the nzbget one, change idxB, drop
	// nothing else.
	idxB.Apikey = "changed"

	loadTestConfig(t, config.Configfile, &config.MainConfig{
		Downloader: []config.DownloaderConfig{
			{Name: "qb", DlType: "nzbget", Hostname: "h", Enabled: true},
			{Name: "off", DlType: "nzbget", Hostname: "h/nzb", Enabled: true},
		},
		Indexers: []config.IndexersConfig{idxA, idxB},
	})

	ReloadClientProviders()

	all = downloadClients()
	if len(all) != 2 || GetNZBGet("qb") == nil || GetNZBGet("off") == nil {
		t.Fatalf("after reload: %v, want nzbget_qb and nzbget_off", all)
	}

	if GetQBittorrent("qb") != nil || GetSABnzbd("sab") != nil {
		t.Fatalf("removed downloaders still registered: %v", all)
	}

	// A caller holding the replaced provider keeps a usable instance.
	if oldQB.GetProviderName() != "qbittorrent" {
		t.Fatal("replaced provider broken")
	}

	if GetIndexer("idxA") != keptIdx {
		t.Error("unchanged indexer was re-created")
	}

	if GetIndexer("idxB") == nil {
		t.Fatal("changed indexer missing")
	}

	loadTestConfig(t, config.Configfile, &config.MainConfig{Indexers: []config.IndexersConfig{idxB}})

	ReloadClientProviders()

	if left := downloadClients(); len(left) != 0 {
		t.Errorf("downloaders left after removing all: %v", left)
	}

	if GetIndexer("idxA") != nil || GetIndexer("idxB") == nil {
		t.Errorf("indexers after removing idxA: %v", GetAllIndexers())
	}
}

func TestNewDownloadClientKeepsSubPath(t *testing.T) {
	tests := []struct {
		cfg  config.DownloaderConfig
		want string
	}{
		{config.DownloaderConfig{DlType: "sabnzbd", Hostname: "https://h:443/sabnzbd/", Password: "k"}, "https://h:443/sabnzbd/api"},
		{config.DownloaderConfig{DlType: "sabnzbd", Hostname: "h", Password: "k"}, "http://h:8080/api"},
		{config.DownloaderConfig{DlType: "qbittorrent", Hostname: "http://h:9000/qbit"}, "http://h:9000/qbit/api/v2"},
		{config.DownloaderConfig{DlType: "nzbget", Hostname: "h/nzbget", Port: 1234}, "http://h:1234/nzbget/jsonrpc"},
	}

	for _, tt := range tests {
		client, err := NewDownloadClient(&tt.cfg)
		if err != nil {
			t.Fatalf("%+v: %v", tt.cfg, err)
		}

		var got string

		switch p := client.(type) {
		case *sabnzbd.Provider:
			got = p.GetBaseURL()
		case *qbittorrent.Provider:
			got = p.GetBaseURL()
		case *nzbget.Provider:
			got = p.GetBaseURL()
		}

		if got != tt.want {
			t.Errorf("%s %q: base URL %q, want %q", tt.cfg.DlType, tt.cfg.Hostname, got, tt.want)
		}
	}
}

func TestBuildBaseURL(t *testing.T) {
	tests := []struct {
		host      string
		port, def int
		want      string
	}{
		{"localhost", 8080, 0, "http://localhost:8080"},
		{"nas.local", 0, 6789, "http://nas.local:6789"},
		{"https://nas.local/sabnzbd/", 0, 0, "https://nas.local:443/sabnzbd"},
		{"nas.local:9091", 1234, 0, "http://nas.local:9091"},
	}

	for _, tt := range tests {
		if got := buildBaseURL(tt.host, tt.port, tt.def); got != tt.want {
			t.Errorf("buildBaseURL(%q, %d, %d) = %q, want %q", tt.host, tt.port, tt.def, got, tt.want)
		}
	}
}

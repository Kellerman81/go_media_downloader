package providers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
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

// clientOf returns downloader name's client if it is a T.
func clientOf[T any](name string) T {
	c, _ := GetDownloadClient(name).(T)
	return c
}

func TestReloadClientProvidersFollowsConfig(t *testing.T) {
	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")

	t.Cleanup(func() {
		config.Configfile = oldFile

		ReplaceDownloadProviders(nil)

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
	if len(all) != 2 || clientOf[*qbittorrent.Provider]("qb") == nil || clientOf[*sabnzbd.Provider]("sab") == nil {
		t.Fatalf("after first load: %v, want qbittorrent_qb and sabnzbd_sab", all)
	}

	oldQB := clientOf[*qbittorrent.Provider]("qb")
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
	if len(all) != 2 || clientOf[*nzbget.Provider]("qb") == nil || clientOf[*nzbget.Provider]("off") == nil {
		t.Fatalf("after reload: %v, want nzbget_qb and nzbget_off", all)
	}

	if clientOf[*qbittorrent.Provider]("qb") != nil || clientOf[*sabnzbd.Provider]("sab") != nil {
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

func TestNewDownloadClientEveryType(t *testing.T) {
	for _, typ := range DownloadClientTypes() {
		client, err := NewDownloadClient(&config.DownloaderConfig{
			Name: "d", DlType: typ.Name, Hostname: "localhost", Username: "u", Password: "p",
		})

		if typ.Name == "drone" {
			if !errors.Is(err, errNoClient) {
				t.Errorf("drone: %v", err)
			}

			continue
		}

		if err != nil {
			t.Errorf("%s: %v", typ.Name, err)
			continue
		}

		if _, ok := client.(interface{ TestConnection(context.Context) error }); !ok {
			t.Errorf("%s: no connection test", typ.Name)
		}

		_, torrent := client.(interface {
			AddTorrent(context.Context, apiexternal_v2.TorrentAddRequest) (*apiexternal_v2.TorrentAddResponse, error)
		})
		if torrent != typ.Torrent || IsTorrentClient(typ.Name) != typ.Torrent {
			t.Errorf("%s: torrent client %t, listed as %t", typ.Name, torrent, typ.Torrent)
		}
	}

	if _, err := NewDownloadClient(&config.DownloaderConfig{Name: "d", DlType: "tribler", Hostname: "h"}); !errors.Is(err, errNoAPIKey) {
		t.Errorf("tribler without API key: %v", err)
	}
}

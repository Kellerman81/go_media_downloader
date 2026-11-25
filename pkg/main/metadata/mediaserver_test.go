package metadata

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/pelletier/go-toml/v2"
)

// fakePlex answers the library endpoints the index and episode lookups use.
func fakePlex(t *testing.T, requests *int) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*requests++

		if r.URL.Query().Get("X-Plex-Token") != "tok" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/library/sections":
			w.Write([]byte(`{"MediaContainer":{"Directory":[{"key":"1","type":"movie"},{"key":"2","type":"show"}]}}`))
		case "/library/sections/1/all":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"10","title":"Inception","summary":"Plex plot","year":2010,"rating":8.8,"userRating":9.0,
				"duration":8880000,"originallyAvailableAt":"2010-07-16","Guid":[{"id":"imdb://tt1375666"},{"id":"tmdb://27205"}],
				"Genre":[{"tag":"Action"},{"tag":"Sci-Fi"}]}]}}`))
		case "/library/sections/2/all":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"ratingKey":"20","title":"Show","summary":"Show plot","Guid":[{"id":"tvdb://81189"}]}]}}`))
		case "/identity":
			w.Write([]byte(`{"MediaContainer":{"machineIdentifier":"abc123"}}`))
		case "/library/metadata/20/allLeaves":
			w.Write([]byte(`{"MediaContainer":{"Metadata":[{"parentIndex":1,"index":1,"title":"Pilot","summary":"Episode plot","originallyAvailableAt":"2008-01-20"}]}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)

	return srv
}

func useMediaServerConfig(t *testing.T, general config.GeneralConfig) {
	t.Helper()

	restore, err := database.UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	oldFile := config.Configfile
	config.Configfile = filepath.Join(t.TempDir(), "config.toml")
	t.Cleanup(func() { config.Configfile = oldFile })

	general.WorkerFiles, general.WorkerParse, general.WorkerSearch = 1, 1, 1

	cfg := config.MainConfig{
		General: general,
		Media: config.MediaConfig{Movies: []config.MediaTypeConfig{{
			Name: "EN", TemplateQuality: "HD", TemplateScheduler: "Default",
			Data:  []config.MediaDataConfig{{TemplatePath: "alpha"}},
			Lists: []config.MediaListsConfig{{Name: "EN List", TemplateList: "listA", TemplateQuality: "HD", Enabled: true}},
		}}},
		Lists:     []config.ListsConfig{{Name: "listA"}},
		Paths:     []config.PathsConfig{{Name: "alpha"}},
		Quality:   []config.QualityConfig{{Name: "HD"}},
		Scheduler: []config.SchedulerConfig{{Name: "Default", IntervalImdb: "3d"}},
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

	providers.RegisterMediaServerProviders()

	msIndexMu.Lock()
	msIndexes = map[string]*msIndex{}
	msIndexMu.Unlock()

	msServerIDMu.Lock()
	msServerIDs = map[string]string{}
	msServerIDMu.Unlock()
}

func TestMediaServerMovieAndSeries(t *testing.T) {
	requests := 0
	srv := fakePlex(t, &requests)
	useMediaServerConfig(t, config.GeneralConfig{
		PlexServerURL: srv.URL, PlexToken: "tok", MovieMetaSourcePlex: true, SerieMetaSourcePlex: true,
		SerieMetaSourcePriority: []string{"plex"},
	})

	movie := database.Dbmovie{ImdbID: "tt1375666", Overview: "kept"}
	if err := movieGetMediaServerMetadata(&movie, sourcePlex, false); err != nil {
		t.Fatal(err)
	}

	if movie.Overview != "kept" || movie.Genres != "Action, Sci-Fi" || movie.Runtime != 148 || movie.Year != 2010 ||
		movie.ReleaseDate.Time.Format("2006-01-02") != "2010-07-16" {
		t.Errorf("movie = %+v", movie)
	}

	// Matched by TMDb ID too; the index is cached, so no new listing.
	before := requests
	byTmdb := database.Dbmovie{MoviedbID: 27205}
	if err := movieGetMediaServerMetadata(&byTmdb, sourcePlex, false); err != nil || byTmdb.Overview != "Plex plot" {
		t.Errorf("by tmdb = %+v, %v", byTmdb, err)
	}

	if requests != before {
		t.Errorf("index not cached: %d requests", requests-before)
	}

	// The series priority loop uses Plex, and it may overwrite on refresh.
	serie := database.Dbserie{ThetvdbID: 81189, Overview: "old"}
	SerieGetMetadata(&serie, "de", "", false, false, true, nil)

	if serie.Overview != "Show plot" {
		t.Errorf("serie overview = %q", serie.Overview)
	}

	eps, err := MediaServerEpisodes(sourcePlex, "", 81189)
	if err != nil || len(eps) != 1 || eps[0].Overview != "Episode plot" || eps[0].Aired.Format("2006-01-02") != "2008-01-20" {
		t.Errorf("episodes = %+v, %v", eps, err)
	}

	// Unknown items are no error.
	if err := movieGetMediaServerMetadata(&database.Dbmovie{ImdbID: "tt0"}, sourcePlex, false); err != nil {
		t.Errorf("unknown movie: %v", err)
	}
}

// TestMediaServerIndexFailure: a failed listing isn't repeated for every
// item of a refresh.
func TestMediaServerIndexFailure(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(srv.Close)

	useMediaServerConfig(t, config.GeneralConfig{PlexServerURL: srv.URL, PlexToken: "tok", MovieMetaSourcePlex: true})

	if err := movieGetMediaServerMetadata(&database.Dbmovie{ImdbID: "tt1375666"}, sourcePlex, false); err == nil {
		t.Fatal("no error from a failing server")
	}

	before := requests
	if err := movieGetMediaServerMetadata(&database.Dbmovie{ImdbID: "tt0816692"}, sourcePlex, false); err == nil {
		t.Error("cached failure returned no error")
	}

	if requests != before {
		t.Errorf("listing repeated: %d requests", requests-before)
	}

	msIndexMu.Lock()
	msIndexes[sourcePlex+"/"+msMovie].built = time.Now().Add(-msIndexRetry)
	msIndexMu.Unlock()

	movieGetMediaServerMetadata(&database.Dbmovie{ImdbID: "tt0816692"}, sourcePlex, false)

	if requests == before {
		t.Error("no retry after msIndexRetry")
	}
}

func TestMediaServerSwitchedOff(t *testing.T) {
	requests := 0
	srv := fakePlex(t, &requests)
	useMediaServerConfig(t, config.GeneralConfig{PlexServerURL: srv.URL, PlexToken: "tok", SerieMetaSourcePlex: true})

	movie := database.Dbmovie{ImdbID: "tt1375666"}
	if err := movieGetMediaServerMetadata(&movie, sourcePlex, false); err != errMediaServerOff || movie.Overview != "" {
		t.Errorf("off: %v, %+v", err, movie)
	}

	// Plex is on for series, but not in serie_meta_source_priority (default list).
	if got := config.GetSettingsGeneral().SerieMetaSourcePriority; strings.Join(got, ",") != "tvdb,tvmaze,tmdb,trakt" {
		t.Errorf("default series priority = %v", got)
	}

	if eps, err := MediaServerEpisodes(sourceJellyfin, "", 81189); eps != nil || err != nil {
		t.Errorf("jellyfin off: %v, %v", eps, err)
	}

	if requests != 0 {
		t.Errorf("%d requests while switched off", requests)
	}
}

func TestMediaServerJellyfin(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("api_key") != "key" || r.URL.Path != "/Items" || r.URL.Query().Get("IncludeItemTypes") != "Movie" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Write([]byte(`{"Items":[{"Id":"j1","Name":"Inception","Overview":"JF plot","Taglines":["Your mind"],
			"RunTimeTicks":88800000000,"ProviderIds":{"Imdb":"tt1375666"}}],"TotalRecordCount":1}`))
	}))
	t.Cleanup(srv.Close)

	useMediaServerConfig(t, config.GeneralConfig{JellyfinServerURL: srv.URL, JellyfinToken: "key", MovieMetaSourceJellyfin: true})

	movie := database.Dbmovie{ImdbID: "tt1375666"}
	if err := movieGetMediaServerMetadata(&movie, sourceJellyfin, false); err != nil {
		t.Fatal(err)
	}

	if movie.Overview != "JF plot" || movie.Tagline != "Your mind" || movie.Runtime != 148 {
		t.Errorf("movie = %+v", movie)
	}
}

func TestMediaServerLinks(t *testing.T) {
	requests := 0
	srv := fakePlex(t, &requests)
	useMediaServerConfig(t, config.GeneralConfig{PlexServerURL: srv.URL + "/", PlexToken: "tok", MovieMetaSourcePlex: true})

	links := MediaServerLinks(msMovie, "tt1375666", 0, 0)
	if len(links) != 1 || links[0].Server != "Plex" ||
		links[0].URL != srv.URL+"/web/index.html#!/server/abc123/details?key=%2Flibrary%2Fmetadata%2F10" {
		t.Errorf("links = %+v", links)
	}

	if strings.Contains(links[0].URL, "tok") {
		t.Error("the link carries the token")
	}

	if links := MediaServerLinks(msMovie, "tt0", 0, 0); len(links) != 0 {
		t.Errorf("unknown movie: %+v", links)
	}
}

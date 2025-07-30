package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/pelletier/go-toml/v2"

	"github.com/mozillazg/go-unidecode"
	"github.com/mozillazg/go-unidecode/table"
	// "github.com/rainycape/unidecode".
)

// func TestMain(m *testing.M) {
// goleak.VerifyTestMain(m)
//}

// Test with: go.exe test -timeout 30s -v -run ^TestDir$ github.com/Kellerman81/go_media_downloader

// Init initializes the test environment for the Go Media Downloader application.
// It creates necessary directories, sets up configuration files, initializes the database,
// cache, logger, and external API clients (OMDB, TMDB, TVDB, Trakt). This function
// prepares the application state for running tests and benchmarks.
func Init() {
	os.Mkdir("./temp", 0o777)
	if !scanner.CheckFileExist(config.Configfile) {
		config.ClearCfg()
		config.WriteCfg()
	}
	config.LoadCfgDB(false)

	database.InitCache()
	logger.InitLogger(logger.Config{
		LogLevel:     "Warning",
		LogFileSize:  config.GetSettingsGeneral().LogFileSize,
		LogFileCount: config.GetSettingsGeneral().LogFileCount,
		LogCompress:  config.GetSettingsGeneral().LogCompress,
	})

	database.UpgradeDB()
	err := database.InitDB(config.GetSettingsGeneral().DBLogLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	apiexternal.NewOmdbClient(
		config.GetSettingsGeneral().OmdbAPIKey,
		config.GetSettingsGeneral().OmdbLimiterSeconds,
		config.GetSettingsGeneral().OmdbLimiterCalls,
		config.GetSettingsGeneral().OmdbDisableTLSVerify,
		config.GetSettingsGeneral().OmdbTimeoutSeconds,
	)
	apiexternal.NewTmdbClient(
		config.GetSettingsGeneral().TheMovieDBApiKey,
		config.GetSettingsGeneral().TmdbLimiterSeconds,
		config.GetSettingsGeneral().TmdbLimiterCalls,
		config.GetSettingsGeneral().TheMovieDBDisableTLSVerify,
		config.GetSettingsGeneral().TmdbTimeoutSeconds,
	)
	apiexternal.NewTvdbClient(
		config.GetSettingsGeneral().TvdbLimiterSeconds,
		config.GetSettingsGeneral().TvdbLimiterCalls,
		config.GetSettingsGeneral().TvdbDisableTLSVerify,
		config.GetSettingsGeneral().TvdbTimeoutSeconds,
	)
	apiexternal.NewTraktClient(
		config.GetSettingsGeneral().TraktClientID,
		config.GetSettingsGeneral().TraktClientSecret,
		config.GetSettingsGeneral().TraktLimiterSeconds,
		config.GetSettingsGeneral().TraktLimiterCalls,
		config.GetSettingsGeneral().TraktDisableTLSVerify,
		config.GetSettingsGeneral().TraktTimeoutSeconds,
	)
	worker.InitWorkerPools(
		config.GetSettingsGeneral().WorkerSearch,
		config.GetSettingsGeneral().WorkerFiles,
		config.GetSettingsGeneral().WorkerMetadata,
		config.GetSettingsGeneral().WorkerRSS,
		config.GetSettingsGeneral().WorkerIndexer,
	)

	database.InitImdbdb()

	logger.LogDynamicany0("info", "Check Database for Upgrades")
	// database.UpgradeDB()
	database.SetVars()

	parser.GenerateAllQualityPriorities()

	parser.LoadDBPatterns()
	parser.GenerateCutoffPriorities()
	database.Refreshhistorycacheurl(true, true)
	database.Refreshhistorycachetitle(true, true)
	database.RefreshMediaCacheList(true, true)
	database.RefreshMediaCacheDB(true, true)
	database.RefreshMediaCacheTitles(true, true)
	database.Refreshunmatchedcached(true, true)
	database.Refreshfilescached(true, true)

	database.Refreshhistorycacheurl(false, true)
	database.Refreshhistorycachetitle(false, true)
	database.RefreshMediaCacheList(false, true)
	database.RefreshMediaCacheDB(false, true)
	database.RefreshMediaCacheTitles(false, true)
	database.Refreshunmatchedcached(false, true)
	database.Refreshfilescached(false, true)
}

func TestCache(t *testing.T) {
	Init()
	database.RefreshMediaCacheDB(true, true)
	database.RefreshMediaCacheTitles(true, true)

	t.Log(
		database.GetCachedThreeStringArr(
			logger.GetStringsMap(true, logger.CacheDBMedia),
			true,
			true,
		),
	)
	t.Log(database.GetCachedTwoIntArr(logger.GetStringsMap(true, logger.CacheMedia), true, true))

	t.Log(
		database.GetCachedTwoStringArr(
			logger.GetStringsMap(true, logger.CacheMediaTitles),
			true,
			true,
		),
	)
}

func TestInitCfg(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		cnt, err := toml.Marshal(config.GetToml().Media)

		t.Log(err)
		t.Log(string(cnt))
	})
}

func TestDBType(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		dbrows := database.GetrowsType(database.Serie{}, false, 1000, "select * from series")
		if len(dbrows) == 0 {
			t.Error("no rows")
		}
		t.Log(dbrows)
	})
}

func TestGetResolutionFromDimensions(t *testing.T) {
	Init()
	tests := []struct {
		name     string
		width    int
		height   int
		expected string
	}{
		// Standard resolutions
		{"8K", 7680, 4320, "4320p"},
		{"8K", 8192, 4320, "4320p"},
		{"4K UHD", 4096, 2160, "2160p"},
		{"4K UHD", 3840, 2160, "2160p"},
		{"1440p QHD", 2560, 1440, "1440p"},
		{"1080p FHD", 1920, 1080, "1080p"},
		{"720p HD", 1280, 720, "720p"},
		{"576p PAL", 720, 576, "576p"},
		{"480p NTSC", 720, 480, "480p"},
		{"480p VGA", 640, 480, "480p"}, // 307.200  1.33
		{"360p", 640, 360, "360p"},     // 1.77
		{"360p", 576, 320, "240p"},
		{"360p", 624, 352, "240p"}, // 219.648 1.77
		{"240p", 426, 240, "240p"},

		// Ultra-wide resolutions
		{"Ultra-wide 1080p", 2560, 1080, "1080p"},
		{"Ultra-wide 1440p", 3440, 1440, "1440p"},
		{"Ultra-wide 4K", 5120, 2160, "2160p"},

		// Edge cases
		{"Very low resolution", 160, 120, "SD"},
		{"Just under 360p", 576, 320, "240p"},

		// Borderline cases
		{"Just under 360p", 640, 359, "360p"},
		{"Just over 360p", 640, 361, "480p"},
		{"Just under 1080p", 1920, 1079, "1080p"},
		{"Just over 1080p", 1920, 1081, "1080p"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := database.ParseInfo{
				Width:  tt.width,
				Height: tt.height,
			}
			result := m.Parseresolution()
			if result != tt.expected {
				t.Errorf("GetResolutionFromDimensions(%d, %d) = %s; expected %s",
					tt.width, tt.height, result, tt.expected)
			}
		})
	}
}

func TestWorker(t *testing.T) {
	Init()
	scheduler.InitScheduler()
	t.Run("test", func(t *testing.T) {
		worker.TestWorker("movie_EN", "timmi", "Feeds", logger.StrRss)
	})
}

func TestParseXML(t *testing.T) {
	url := "https://api.nzbgeek.info/api?apikey=&tvdbid=82701&season=1&ep=8&cat=5030&dl=1&t=tvsearch&extended=1"

	req, _ := http.NewRequest("GET", url, http.NoBody)
	cl := &http.Client{
		Timeout: time.Duration(10) * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   time.Duration(10) * time.Second,
			ResponseHeaderTimeout: time.Duration(10) * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:          20,
			MaxConnsPerHost:       10,
			DisableCompression:    false,
			DisableKeepAlives:     true,
			IdleConnTimeout:       120 * time.Second,
		},
	}
	resp, _ := cl.Do(req)
	d := xml.NewDecoder(resp.Body)
	// d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	d.DecodeElement(d, &xml.StartElement{})
}

// PrintMemUsage reads and displays current memory usage statistics for debugging purposes.
// It prints allocation information including current allocations, total allocations,
// heap statistics, system memory usage, and garbage collection counts.
func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	fmt.Printf("Alloc = %v", m.Alloc)
	fmt.Printf("\tTotalAlloc = %v", m.TotalAlloc)
	fmt.Printf("\tHeapAlloc = %v", m.HeapAlloc)
	fmt.Printf("\tHeapObjects = %v", m.HeapObjects)
	fmt.Printf("\tHeapReleased = %v", m.HeapReleased)
	fmt.Printf("\tSys = %v", m.Sys)
	fmt.Printf("\tNumGC = %v\n", m.NumGC)
}

func TestGetTmdb(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		// var movie database.Dbmovie
		// movie.ImdbID = "tt5971474"
		// movie.MoviedbID = 447277
		// metadata.Getmoviemetadata(&movie, true)
		// tmdbfind, _ := apiexternal.TmdbAPI.FindImdb("tt7214954")
		// t.Log(tmdbfind)
		// tmdbtitle, _ := apiexternal.GetTmdbMovieTitles(585511)
		// t.Log(tmdbtitle)
		// movie.GetImdbTitle(true)
		// t.Log(movie.Runtime)
		// tmdbdetails, _ := apiexternal.GetTmdbMovie(447277)
		// t.Log(tmdbdetails.Runtime)
		// tt := "tt5971474"
		// traktdetails, _ := apiexternal.GetTraktMovie(tt)
		// t.Log(traktdetails.Runtime)

		lst, err := apiexternal.GetTmdbList(8515441)
		t.Log(err)
		t.Log(lst)
		t.Log(len(lst.Items))
		// var dbserie database.Dbserie
		// dbserie.ThetvdbID = 85352
		// dbserie.GetMetadata("", true, true, true, true)
		// t.Log(dbserie)
		// t.Log(dbserie.ImdbID)
		// t.Log(dbserie.ID)
		// t.Log(dbserie.Seriename)
		// GetNewFilesTest("serie_EN", logger.StrSeries)
	})
}

func TestGetTvdb(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		var serie database.Dbserie
		tvdbdetails, _ := apiexternal.GetTvdbSeries(85352, "")
		if (serie.Seriename == "") && tvdbdetails.Data.SeriesName != "" {
			serie.Seriename = tvdbdetails.Data.SeriesName
		}
		t.Log(serie.Seriename)
		var dbserie database.Dbserie
		dbserie.ThetvdbID = 85352
		metadata.SerieGetMetadata(&dbserie, "", true, true, true, []string{})
		t.Log(dbserie)
		t.Log(dbserie.ImdbID)
		t.Log(dbserie.ID)
		t.Log(dbserie.Seriename)
		// GetNewFilesTest("serie_EN", logger.StrSeries)
	})
}

func TestSQL(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		a := database.GetrowsN[database.DbstaticThreeStringTwoInt](
			false,
			database.Getdatarow[uint](false, "select count() from dbmovies")+100,
			"select title, slug, imdb_id, year, id from dbmovies",
		)
		t.Log(a)
		t.Log(
			database.Getdatarow[int](
				false,
				"select id from dbmovies ORDER BY [id] ASC limit ?",
				1,
			),
		)
		t.Log(database.Getdatarow[uint](false, "select count() from dbmovies"))
		var i int
		database.Scanrowsdyn(false, "select id from dbmovies ORDER BY [id] ASC limit ?", &i, 1)
		t.Log(i)

		b := database.GetrowsN[string](
			false,
			database.Getdatarow[uint](false, "select count() from dbmovies")+100,
			"select distinct title from dbmovies",
		)
		t.Log(b)
		ab := database.GetrowsN[database.DbstaticTwoStringOneInt](
			false,
			database.Getdatarow[uint](
				false,
				"select count() from dbserie_alternates where title != ''",
			)+100,
			"select title, slug, dbserie_id from dbserie_alternates where title != ''",
		)
		t.Log(ab)
		t.Log(len(ab))
		t.Log(cap(ab))

		database.RefreshMediaCacheTitles(true, true)
		c := database.GetCachedTwoStringArr(logger.CacheDBSeriesAlt, true, true)
		t.Log(len(c))
		t.Log(cap(c))
		// a := database.GetCachedTypeObjArr[database.DbstaticTwoStringOneInt](logger.CacheDBSeries)
		// t.Log(database.GetrowsN[database.DbstaticThreeStringTwoInt](false, database.Getdatarow[uint][int](false, "select count() from dbmovies")+100, "select title, slug, imdb_id, year, id from dbmovies"))
		// var id uint = 1
		// c, err := database.GetDbmovieByID(&id)
		// t.Log(c)
		// t.Log(err)
	})
}

func TestGenIncQuery(t *testing.T) {
	Init()

	cfgp := config.GetSettingsMedia("serie_EN")
	searchmissing := true
	var searchinterval uint16 = 5
	t.Run("test", func(t *testing.T) {
		var scaninterval int
		var scandatepre int
		if cfgp.DataLen >= 1 && cfgp.Data[0].CfgPath != nil {
			if searchmissing {
				scaninterval = cfgp.Data[0].CfgPath.MissingScanInterval
				scandatepre = cfgp.Data[0].CfgPath.MissingScanReleaseDatePre
			} else {
				scaninterval = cfgp.Data[0].CfgPath.UpgradeScanInterval
			}
		}

		if cfgp.ListsLen == 0 {
			return
		}
		// args := make([]any, 0, len(cfgp.Lists)+2)
		args := logger.PLArrAny.Get()
		for idx := range cfgp.Lists {
			args.Arr = append(args.Arr, &cfgp.Lists[idx].Name)
		}

		bld := logger.PlAddBuffer.Get()

		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenTable)
		if searchmissing {
			bld.WriteStringMap(cfgp.Useseries, logger.SearchGenMissing)
			bld.WriteString(cfgp.ListsQu)
			bld.WriteStringMap(cfgp.Useseries, logger.SearchGenMissingEnd)
		} else {
			bld.WriteStringMap(cfgp.Useseries, logger.SearchGenReached)
			bld.WriteString(cfgp.ListsQu)
			bld.WriteByte(')')
		}
		if scaninterval != 0 {
			bld.WriteStringMap(cfgp.Useseries, logger.SearchGenLastScan)
			timeinterval := logger.TimeGetNow().AddDate(0, 0, 0-scaninterval)
			args.Arr = append(args.Arr, &timeinterval)
		}
		if scandatepre != 0 {
			bld.WriteStringMap(cfgp.Useseries, logger.SearchGenDate)
			timedatepre := logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
			args.Arr = append(args.Arr, &timedatepre)
		}
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenOrder)
		if searchinterval != 0 {
			bld.WriteString(" limit ")
			bld.WriteUInt16(searchinterval)
		}

		str := bld.String()
		tbl := database.GetrowsNuncached[database.DbstaticOneStringOneUInt](
			database.Getdatarow[uint](
				false,
				logger.JoinStrings("select count() ", str),
				args.Arr...),
			logger.JoinStrings(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenSelect), str),
			args.Arr,
		)
		logger.PlAddBuffer.Put(bld)
		logger.PLArrAny.Put(args)
		t.Log(tbl)
	})
}

func TestQueryMovie(b *testing.T) {
	Init()
	var id uint = 14027
	ctx := context.Background()
	results := searcher.NewSearcher(config.GetSettingsMedia("movie_EN"), nil, "", nil)
	err := results.MediaSearch(ctx, config.GetSettingsMedia("movie_EN"), id, false, false, false)
	// b.Log(results)
	bla, _ := json.Marshal(results)
	b.Log(string(bla))
	b.Log(err)
}

func TestTraktQuery(b *testing.T) {
	Init()
	data, err := apiexternal.Testaddtraktdbepisodes()
	b.Log(data)
	b.Log(err)
}

func TestRequest(t *testing.T) {
	Init()

	ctx, cancelc := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.thetvdb.com/series/341164",
		http.NoBody,
	)
	fmt.Println(err)
	resp, err := http.DefaultClient.Do(req)
	fmt.Println(err)
	fmt.Println(resp.Body)
	req, err = http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		"https://api.thetvdb.com/series/289431",
		http.NoBody,
	)
	fmt.Println(err)
	resp, err = http.DefaultClient.Do(req)
	fmt.Println(err)
	fmt.Println(resp.Body)
	cancelc()
	fmt.Println("Hello, 世界")
}

func TestGenQuery(t *testing.T) {
	Init()
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, loop *config.MediaTypeConfig) bool {
		cfgp = loop
		return true
	})
	t.Log(cfgp.Data[0].CfgPath.MissingScanInterval)
	searchmissing := true
	searchinterval := 100
	var scaninterval int
	var scandatepre int
	if cfgp.DataLen >= 1 && cfgp.Data[0].CfgPath != nil {
		if searchmissing {
			scaninterval = cfgp.Data[0].CfgPath.MissingScanInterval
			scandatepre = cfgp.Data[0].CfgPath.MissingScanReleaseDatePre
		} else {
			scaninterval = cfgp.Data[0].CfgPath.UpgradeScanInterval
		}
	}

	if cfgp.ListsLen == 0 {
		return
	}
	args := make([]any, 0, len(cfgp.Lists)+2)
	for i := range cfgp.Lists {
		args = append(args, &cfgp.Lists[i])
	}
	if scaninterval != 0 {
		if scandatepre == 0 {
			args = append(args, "")
		} else {
			args = append(args, "", "")
		}
	}
	if scandatepre != 0 {
		if scaninterval == 0 {
			args = append(args, "")
		} else {
			args = append(args, "", "")
		}
	}

	var bld bytes.Buffer
	bld.Grow(750)
	defer bld.Reset()
	var query string
	if !cfgp.Useseries {
		bld.WriteString(" from movies inner join dbmovies on dbmovies.id=movies.dbmovie_id where ")

		if searchmissing {
			bld.WriteString("dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?")
			bld.WriteString(cfgp.ListsQu)
			bld.WriteString(")")
		} else {
			bld.WriteString("dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?")
			bld.WriteString(cfgp.ListsQu)
			bld.WriteString(")")
		}
		if scaninterval != 0 {
			bld.WriteString(" and (movies.lastscan is null or movies.Lastscan < ?)")
			args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
		}
		if scandatepre != 0 {
			bld.WriteString(" and (dbmovies.release_date < ? or dbmovies.release_date is null)")
			if scaninterval != 0 {
				args[cfgp.ListsLen+1] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
			} else {
				args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
			}
		}
		bld.WriteString(" order by movies.Lastscan asc")
		if searchinterval != 0 {
			bld.WriteString(" limit ")
			bld.WriteString(strconv.Itoa(searchinterval))
		}
		query = "select movies.id " + bld.String()
	} else {
		bld.WriteString(" from serie_episodes inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id where ")

		if searchmissing {
			bld.WriteString("serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?")
			bld.WriteString(cfgp.ListsQu)
			bld.WriteString(") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)")
		} else {
			bld.WriteString("serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?")
			bld.WriteString(cfgp.ListsQu)
			bld.WriteString(")")
		}
		if scaninterval != 0 {
			bld.WriteString(" and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)")
			args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
		}
		if scandatepre != 0 {
			bld.WriteString(" and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)")
			if scaninterval != 0 {
				args[cfgp.ListsLen+1] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
			} else {
				args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
			}
		}
		bld.WriteString(" order by serie_episodes.Lastscan asc")
		if searchinterval != 0 {
			bld.WriteString(" limit ")
			bld.WriteString(strconv.Itoa(searchinterval))
		}
		query = "select serie_episodes.id " + bld.String()
	}
	t.Log("select count() " + bld.String())
	cntquery := database.Getdatarow[uint](false, "select count() "+bld.String(), args...)

	if cntquery == 0 || query == "" || len(args) == 0 {
		return
	}
}

func TestParse(t *testing.T) {
	Init()

	tests := []struct {
		name             string
		filename         string
		configKey        string
		qualityKey       string
		expectTitle      string
		expectIdentifier string
		expectYear       uint16
		expectQuality    string
		expectResolution string
		expectCodec      string
		expectAudio      string
	}{
		{
			name:             "German movie with Web quality",
			filename:         "Schneewittchen.2025.German.AC3.DL.1080p.Web.x265-FuN",
			configKey:        "movie_DE",
			qualityKey:       "SDDE",
			expectTitle:      "Schneewittchen",
			expectYear:       2025,
			expectQuality:    "webdl",
			expectResolution: "1080p",
			expectCodec:      "h265",
			expectAudio:      "ac3",
		},
		{
			name:             "English movie with BluRay quality",
			filename:         "The.Matrix.1999.1080p.BluRay.x264-RARBG",
			configKey:        "movie_EN",
			qualityKey:       "HD",
			expectTitle:      "The Matrix",
			expectYear:       1999,
			expectQuality:    "bluray",
			expectResolution: "1080p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
		{
			name:             "German series with HD quality",
			filename:         "Eureka.S04E04.720p.HDTV.x264-DIMENSION",
			configKey:        "serie_DE",
			qualityKey:       "HD",
			expectTitle:      "Eureka",
			expectIdentifier: "S04E04",
			expectYear:       0, // Series might not have year
			expectQuality:    "hdtv",
			expectResolution: "720p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
		{
			name:             "English series with SD quality",
			filename:         "Eureka.S04E04.480p.x264-ZMNT",
			configKey:        "serie_EN",
			qualityKey:       "SD",
			expectTitle:      "Eureka",
			expectIdentifier: "S04E04",
			expectYear:       0, // Series might not have year
			expectQuality:    "sdtv",
			expectResolution: "480p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
		{
			name:             "German series with HD quality",
			filename:         "Eureka.S04E04.720p.HDTV.x264-DIMENSION",
			configKey:        "serie_DE",
			qualityKey:       "HD",
			expectTitle:      "Eureka",
			expectIdentifier: "S04E04",
			expectYear:       0, // Series might not have year
			expectQuality:    "hdtv",
			expectResolution: "720p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
		{
			name:             "German series with HD quality multi episode",
			filename:         "Eureka.S04E04E05.720p.HDTV.x264-DIMENSION",
			configKey:        "serie_DE",
			qualityKey:       "HD",
			expectTitle:      "Eureka",
			expectIdentifier: "S04E04E05",
			expectYear:       0, // Series might not have year
			expectQuality:    "hdtv",
			expectResolution: "720p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
		{
			name:             "German series with HD quality multi episode dash sep",
			filename:         "Eureka.S04E04-E05.720p.HDTV.x264-DIMENSION",
			configKey:        "serie_DE",
			qualityKey:       "HD",
			expectTitle:      "Eureka",
			expectIdentifier: "S04E04-E05",
			expectYear:       0, // Series might not have year
			expectQuality:    "hdtv",
			expectResolution: "720p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
		{
			name:             "German series with HD quality multi episode dash",
			filename:         "Eureka.S04E04-05.720p.HDTV.x264-DIMENSION",
			configKey:        "serie_DE",
			qualityKey:       "HD",
			expectTitle:      "Eureka",
			expectIdentifier: "S04E04-05",
			expectYear:       0, // Series might not have year
			expectQuality:    "hdtv",
			expectResolution: "720p",
			expectCodec:      "h264",
			expectAudio:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Get configuration objects
			cfgp := config.GetSettingsMedia(tt.configKey)
			if cfgp == nil {
				t.Fatalf("Config key %s not found", tt.configKey)
			}

			quality := config.GetSettingsQuality(tt.qualityKey)
			if quality == nil {
				t.Fatalf("Quality key %s not found", tt.qualityKey)
			}

			// Parse the file
			m := parser.ParseFile(tt.filename, false, false, cfgp, -1)
			if m == nil {
				t.Fatal("ParseFile returned nil")
			}

			// Get database IDs and quality mapping
			parser.GetDBIDs(m, cfgp, true)
			parser.GetPriorityMapQual(m, cfgp, quality, false, true)

			// Assertions with meaningful error messages
			if m.Title != tt.expectTitle {
				t.Errorf("Expected title %q, got %q", tt.expectTitle, m.Title)
			}

			if tt.expectYear > 0 && m.Year != tt.expectYear {
				t.Errorf("Expected year %d, got %d", tt.expectYear, m.Year)
			}

			// Log results for debugging (only in verbose mode)
			if testing.Verbose() {
				t.Logf("Parsed result: %+v", m)
				t.Logf("Quality: %+v", m.Quality)
				t.Logf("QualityID: %d", m.QualityID)
				t.Logf("Title: %s", m.Title)
				t.Logf("Year: %d", m.Year)
			}

			// Additional validations
			if m.Quality != tt.expectQuality {
				t.Errorf("Expected quality %q, got %q", tt.expectQuality, m.Quality)
			}
			if m.Resolution != tt.expectResolution {
				t.Errorf("Expected quality %q, got %q", tt.expectResolution, m.Resolution)
			}
			if m.Codec != tt.expectCodec {
				t.Errorf("Expected quality %q, got %q", tt.expectCodec, m.Codec)
			}
			if m.Audio != tt.expectAudio {
				t.Errorf("Expected quality %q, got %q", tt.expectAudio, m.Audio)
			}
			if m.Identifier != tt.expectIdentifier {
				t.Errorf("Expected identifier %q, got %q", tt.expectIdentifier, m.Identifier)
			}
		})
	}
}

// Path sanitizes a string to be safe for use as a file or directory path.
// It removes potentially dangerous characters, handles path traversal attempts,
// and optionally removes path separators. The allowslash parameter controls
// whether forward and back slashes are preserved or removed.
func Path(s string, allowslash bool) string {
	if s == "" {
		return ""
	}
	s = logger.UnquoteUnescape(s)
	s = strings.ReplaceAll(s, "..", "")
	s = path.Clean(s)
	if !allowslash {
		s = strings.ReplaceAll(s, "\\", "")
		s = strings.ReplaceAll(s, "/", "")
	}
	s = strings.ReplaceAll(s, ":", "")
	s = strings.ReplaceAll(s, "*", "")
	s = strings.ReplaceAll(s, "?", "")
	s = strings.ReplaceAll(s, "\"", "")
	s = strings.ReplaceAll(s, "<", "")
	s = strings.ReplaceAll(s, ">", "")
	s = strings.ReplaceAll(s, "|", "")

	// if allowslash {
	// 	s = pathReplacerWOSeperator.Replace(s)
	// } else {
	// 	s = pathReplacerWSeperator.Replace(s)
	// }
	if s == "" {
		return ""
	}
	return strings.Trim(s, " ")
}

// Path2 is an alternative path sanitization function with optimized character checking.
// It performs similar sanitization to Path() but uses ContainsRune checks before
// replacement operations for better performance. Removes unsafe characters and
// handles path traversal attempts.
func Path2(s string, allowslash bool) string {
	if s == "" {
		return ""
	}
	s = logger.UnquoteUnescape(s)
	s = strings.ReplaceAll(s, "..", "")
	s = path.Clean(s)
	if !allowslash {
		if strings.ContainsRune(s, '\\') {
			s = strings.ReplaceAll(s, "\\", "")
		}
		if strings.ContainsRune(s, '/') {
			s = strings.ReplaceAll(s, "/", "")
		}
	}
	if strings.ContainsRune(s, ':') {
		s = strings.ReplaceAll(s, ":", "")
	}
	if strings.ContainsRune(s, '*') {
		s = strings.ReplaceAll(s, "*", "")
	}
	if strings.ContainsRune(s, '?') {
		s = strings.ReplaceAll(s, "?", "")
	}
	if strings.ContainsRune(s, '"') {
		s = strings.ReplaceAll(s, "\"", "")
	}
	if strings.ContainsRune(s, '<') {
		s = strings.ReplaceAll(s, "<", "")
	}
	if strings.ContainsRune(s, '>') {
		s = strings.ReplaceAll(s, ">", "")
	}
	if strings.ContainsRune(s, '|') {
		s = strings.ReplaceAll(s, "|", "")
	}
	// if allowslash {
	// 	s = pathReplacerWOSeperator.Replace(s)
	// } else {
	// 	s = pathReplacerWSeperator.Replace(s)
	// }
	if s == "" {
		return ""
	}
	if s[:1] == " " || s[len(s)-1:] == " " {
		return strings.Trim(s, " ")
	}
	return s
}

// joinCats converts a slice of category integers into a comma-separated string.
// It skips zero values and builds an efficient string representation using
// a bytes.Buffer. Used for formatting category lists in search operations.
func joinCats(cats []int) string {
	var b bytes.Buffer
	defer b.Reset()
	// b.Grow(30)
	for idx := range cats {
		if cats[idx] == 0 {
			continue
		}
		if b.Len() >= 1 {
			b.WriteString(",")
		}
		b.WriteString(strconv.Itoa(cats[idx]))
	}
	return b.String()
}

var substituteRune = map[rune]string{
	'&':  "and",
	'@':  "at",
	'"':  "",
	'\'': "",
	'’':  "",
	'_':  "",
	'‒':  "-",
	' ':  "-",
	'–':  "-",
	'—':  "-",
	'―':  "-",
	'ä':  "ae",
	'ö':  "oe",
	'ü':  "ue",
	'Ä':  "Ae",
	'Ö':  "Oe",
	'Ü':  "Ue",
	'ß':  "ss",
}

// unidecode2 converts Unicode characters to ASCII equivalents using a custom mapping.
// It performs character substitution (e.g., ä->ae, ß->ss), handles case conversion,
// and prevents consecutive duplicate characters. Used for normalizing text for
// file names and database operations.
func unidecode2(s string) string {
	var ret strings.Builder
	var laststr string
	ret.Grow(len(s))
	for i, r := range s {
		ss, ok := substituteRune[r]
		if ok {
			if ss == laststr {
				continue
			}
			ret.WriteString(ss)
			laststr = ss
			continue
		}
		laststr = ""
		if r < unicode.MaxASCII {
			c := s[i]
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
				ret.WriteByte(c)
			} else {
				ret.WriteRune(r)
			}
			continue
		}
		if r > 0xeffff {
			continue
		}

		section := r >> 8   // Chop off the last two hex digits
		position := r % 256 // Last two hex digits
		if tb, ok := table.Tables[section]; ok {
			if len(tb) > int(position) {
				if len(tb[position]) >= 1 {
					if tb[position][0] > unicode.MaxASCII {
						ret.WriteRune('-')
						continue
					}
				}
				ret.WriteString(tb[position])
			}
		}
	}
	return ret.String()
}

type A struct {
	Str1 string
	Str2 string
	Str3 string
	Str4 string
	Str5 string
	Str6 string
}

var sep = string(os.PathSeparator)

func FilepathJoinSeperator3(elem1 string, elem2 string) string {
	if len(elem1) == 0 {
		return elem2
	}
	if len(elem2) == 0 {
		return elem1
	}
	firstchar := elem1[len(elem1)-1]
	lastchar := elem2[0]
	keepelem1 := firstchar == ':' || os.IsPathSeparator(firstchar)
	keepelem2 := os.IsPathSeparator(lastchar)

	if keepelem1 {
		if keepelem2 {
			return elem1 + elem2[1:]
		}
		return elem1 + elem2
	}
	if keepelem2 {
		return elem1 + elem2
	}
	return elem1 + sep + elem2
	// return strings.TrimRight(elem1, "\\/") + sep + strings.TrimLeft(elem2, "\\/")
}

type UIntSet struct {
	Values []uint32
}

func NewUintSet() UIntSet {
	return UIntSet{}
}

func NewUintSetMaxSize(size int) UIntSet {
	return UIntSet{Values: make([]uint32, 0, size)}
}

func NewUintSetExactSize(size int) UIntSet {
	return UIntSet{Values: make([]uint32, size)}
}

func (s *UIntSet) Add(val uint32) {
	s.Values = append(s.Values, val)
}

func (s *UIntSet) Length() int {
	return len(s.Values)
}

func (s *UIntSet) Remove(valchk uint32) {
	newv := s.Values[:0]
	for _, val := range s.Values {
		if val != valchk {
			newv = append(newv, val)
		}
	}
	s.Values = newv
}

func (s *UIntSet) Contains(valchk uint32) bool {
	for _, val := range s.Values {
		if val == valchk {
			return true
		}
	}
	return false
}

func (s *UIntSet) Clear() {
	s.Values = nil
}

// Benchmarks

func BenchmarkQueryLower(b *testing.B) {
	Init()
	b.ResetTimer()
	str := ""
	var id1 uint = 32
	var id2 uint = 32
	var id3 uint = 32
	var id4 uint = 32
	for i := 0; i < b.N; i++ {
		// scanner.MoveFileDriveBufferNew(val, newpath)
		// scanner.MoveFileDriveBuffer(val, newpath)
		str = strconv.Itoa(
			int(id1),
		) + "_" + strconv.Itoa(
			int(id2),
		) + "_" + strconv.Itoa(
			int(id3),
		) + "_" + strconv.Itoa(
			int(id4),
		)
		// str = fmt.Sprint(id1, "_", id2, "_", id3, "_", id4)
	}
	logger.LogDynamicany0("info", str)
}

func BenchmarkQueryLower2(b *testing.B) {
	Init()
	b.ReportAllocs()
	b.ResetTimer()
	str := "Movie"
	for i := 0; i < b.N; i++ {
		if strings.EqualFold(str, logger.StrMovie) {
			continue
		}
	}
}

func BenchmarkArr1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		a := []string{"extended", "extended cut", "extended.cut", "extended-cut", "extended_cut"}
		_ = a
	}
}

func BenchmarkArr2(b *testing.B) {
	c := "extended,extended cut,extended.cut,extended-cut,extended_cut"
	for i := 0; i < b.N; i++ {
		a := strings.Split(c, ",")
		_ = a
	}
}

func BenchmarkQuery11(b *testing.B) {
	Init()
	b.ResetTimer()
	var cachetconst map[uint32]struct{}
	for i := 0; i < b.N; i++ {
		cachetconst = make(map[uint32]struct{}, 1200000)
		for i := 0; i < 1000000; i++ {
			cachetconst[uint32(i)] = struct{}{}
		}
		// _, _ = cachetconst[uint32(999555)]
	}
}

func BenchmarkQuery12(b *testing.B) {
	Init()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cachetconst := NewUintSetMaxSize(1200000)
		for i := 0; i < 1000000; i++ {
			cachetconst.Add(uint32(i))
		}
		_ = cachetconst.Contains(999555)
	}
}

func BenchmarkQuery14(b *testing.B) {
	Init()
	cfgFolder := "Y:\\completed\\Movies\\Morbius.2022.1080p.WEB-DL.x264.AAC-EVO. (tt5108870)"
	getconfig := "movie_EN"
	cfgp := config.GetSettingsMedia(getconfig)
	var cfgimport *config.MediaDataImportConfig
	for _, imp := range cfgp.DataImport {
		if strings.EqualFold(imp.TemplatePath, "en movies") {
			cfgimport = &imp
			break
		}
	}
	// structurevar := structure.NewStructure(cfgp, cfgimport, "en movies", "en movies import")
	// structurevar.Checkruntime = true
	// structurevar.Deletewronglanguage = false
	// structurevar.SetOrgadata(&structure.Organizerdata{})
	ctx := context.Background()
	defer ctx.Done()
	for i := 0; i < b.N; i++ {
		structure.OrganizeSingleFolder(ctx, cfgFolder, cfgp, cfgimport, "en movies", true, false, 0)
	}
}

func BenchmarkQuery15(b *testing.B) {
	Init()
	var x string
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importfeed.JobImportMovies(x, nil, -1, false)
	}
}

func BenchmarkAllowRange(b *testing.B) {
	Init()
	arr := database.GetrowsN[database.DbstaticThreeStringTwoInt](
		false,
		database.Getdatarow[uint](false, "select count() from dbmovies")+100,
		"select title, slug, imdb_id, year, id from dbmovies",
	)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for idx := range arr {
			_ = idx
		}
	}
	PrintMemUsage()
}

func BenchmarkAllowRange2(b *testing.B) {
	Init()
	arr := database.GetrowsN[database.DbstaticThreeStringTwoInt](
		false,
		database.Getdatarow[uint](false, "select count() from dbmovies")+100,
		"select title, slug, imdb_id, year, id from dbmovies",
	)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, r := range arr {
			_ = r
		}
	}
	PrintMemUsage()
}

func BenchmarkQuery4(b *testing.B) {
	Init()
	// additionalQueryParams := "&extended=1&maxsize=6291456000"
	// categories := "2030,2035,2040,2045"
	// apikey := ""
	// apiBaseURL := "https://api.nzbgeek.info"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// str := "SD"

		ctx := context.Background()
		searchresults := searcher.NewSearcher(nil, nil, logger.StrRss, nil)
		searchresults.SearchRSS(ctx, nil, nil, false, false)

		searchresults.Close()
		// clie.SearchWithIMDB(categories, "tt0120655", additional_query_params, "", 0, false)
		// clie.LoadRSSFeed(categories, 100, additional_query_params, "", "", "", 0, false)
		// apiexternal.QueryNewznabRSSLast(apiexternal.NzbIndexer{URL: apiBaseURL, Apikey: apikey, UserID: 0, AdditionalQueryParams: additional_query_params, LastRssId: "", Limitercalls: 10, Limiterseconds: 5}, 100, categories, 2)
		// parser.NewVideoFile("", "Y:\\completed\\MoviesDE\\Uncharted.2022.German.AC3LD.5.1.BDRip.x264-PS\\Uncharted.2022.German.AC3LD.5.1.BDRip.x264-PS (tt1464335).mkv", false)
		// searcher2 := searcher.NewSearcher("movie_EN", "SD")
		// movie, _ := database.GetMovies(database.Query{Limit: 1})
		// searcher2.MovieSearch(movie, false, true)

		// scanner.GetFilesDir("c:\\windows", []string{".dll"}, []string{}, []string{})
		// database.QueryDbserieTest4(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		// for idxserie := range dbseries {
		// importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkQuery5(b *testing.B) {
	Init()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// err := logger.DownloadFile("./temp", "", "imdblist.csv", "https://www.imdb.com/list/ls003672378/export")
		// if err != nil {
		// 	continue
		// }

		// filelist, err := os.Open("./temp/imdblist.csv")
		// if err != nil {
		// 	continue
		// }
		// defer filelist.Close()
		// records, err := csv.NewReader(bufio.NewReader(filelist)).ReadAll()
		// if err != nil {
		// 	continue
		// }

		var resp http.Response
		var err error
		// err := logger.GetUrlResponse("https://www.imdb.com/list/ls003672378/export", &resp)
		// if err != nil {
		// 	continue
		// }
		parserimdb := csv.NewReader(bufio.NewReader(resp.Body))
		parserimdb.ReuseRecord = true
		var d []database.Dbmovie
		_, _ = parserimdb.Read() // skip header
		var record []string
		var err2 error
		var year, votes int64
		var year32 uint16
		var votes32 int32
		var voteavg float64
		var voteavg32 float32
		for {
			record, err2 = parserimdb.Read()
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				logger.LogDynamicanyErr("error", "an error occurred while parsing csv", err)
				continue
			}
			year, err = strconv.ParseInt(record[10], 0, 16)
			if err != nil {
				continue
			}
			year32 = uint16(year)
			votes, err = strconv.ParseInt(record[12], 0, 32)
			if err != nil {
				continue
			}
			votes32 = int32(votes)
			voteavg, err = strconv.ParseFloat(record[8], 32)
			if err != nil {
				continue
			}
			voteavg32 = float32(voteavg)
			d = append(
				d,
				database.Dbmovie{
					ImdbID:      record[1],
					Title:       record[5],
					URL:         record[6],
					VoteAverage: voteavg32,
					Year:        year32,
					VoteCount:   votes32,
				},
			)
		}
		_ = d
		resp.Body.Close()
	}
}

func BenchmarkQuery6(b *testing.B) {
	Init()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var resp http.Response
		// err := logger.GetUrlResponse("https://www.imdb.com/list/ls003672378/export", &resp)
		// if err != nil {
		// 	continue
		// }

		defer resp.Body.Close()
		parserimdb := csv.NewReader(resp.Body)

		var d []database.Dbmovie
		_, _ = parserimdb.Read() // skip header
		records, _ := parserimdb.ReadAll()
		for _, record := range records {
			year, err := strconv.ParseInt(record[10], 0, 64)
			if err != nil {
				continue
			}
			votes, err := strconv.ParseInt(record[12], 0, 64)
			if err != nil {
				continue
			}
			voteavg, err := strconv.ParseFloat(record[8], 64)
			if err != nil {
				continue
			}
			d = append(
				d,
				database.Dbmovie{
					ImdbID:      record[1],
					Title:       record[5],
					URL:         record[6],
					VoteAverage: float32(voteavg),
					Year:        uint16(year),
					VoteCount:   int32(votes),
				},
			)
		}
		_ = d
	}
}

func BenchmarkQuery7(b *testing.B) {
	Init()
	// val := "C:\\temp\\movies\\movie.mkv"
	// newpath := "C:\\temp\\movies\\movie_temp.mkv"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// scanner.MoveFileDriveBufferNew(val, newpath)
		// scanner.MoveFileDriveBuffer(val, newpath)
		// scanner.MoveFileDrive(val, newpath)
	}
}

func Benchmark5Buffer3(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := 0; j < 10000; j++ {
			FilepathJoinSeperator3("/mnt/user/", "folder")
		}
	}
}

func BenchmarkByte1(b *testing.B) {
	byteArray := []byte{'J', 'A', 'N', 'E'}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		str1 := string(byteArray)
		_ = str1
	}
}

func BenchmarkByte3(b *testing.B) {
	byteArray := []byte{'J', 'A', 'N', 'E'}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		str1 := fmt.Sprintf("%s", byteArray)
		_ = str1
	}
}

func BenchmarkWalkDir(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		filepath.WalkDir("C:\\", func(path string, d fs.DirEntry, err error) error {
			return nil
		})
	}
}

func BenchmarkWalkDirOrg(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		filepath.WalkDir("C:\\", func(path string, d fs.DirEntry, err error) error {
			return nil
		})
	}
}

func BenchmarkQuery3(b *testing.B) {
	// Init()
	// b.ResetTimer()
	additionalQueryParams := "&extended=1&maxsize=6291456000"
	categories := []int{2030, 2035, 2040, 2045}
	episode := 1
	season := 10
	tvDBID := 55797
	apikey := ""
	apiPath := "/api"
	apiBaseURL := "https://api.nzbgeek.info"
	for i := 0; i < 100; i++ {
		var buildurl bytes.Buffer
		buildurl.WriteString(apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(apikey)
		buildurl.WriteString("&tvdbid=")
		buildurl.WriteString(strconv.Itoa(tvDBID))
		buildurl.WriteString("&season=")
		buildurl.WriteString(strconv.Itoa(season))
		buildurl.WriteString("&ep=")
		buildurl.WriteString(strconv.Itoa(episode))
		buildurl.WriteString("&limit=")
		buildurl.WriteString("100")
		buildurl.WriteString("&cat=")
		buildurl.WriteString(joinCats(categories))
		buildurl.WriteString("&dl=1&t=tvsearch")
		buildurl.WriteString("&o=json")
		buildurl.WriteString(additionalQueryParams)
		fmt.Println(buildurl.String())

		// database.CountRows("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserieTest3(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		// for idxserie := range dbseries {
		// importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkConvertSlice(b *testing.B) {
	a := "dhffghfdghfdhfjfgbcvnbvmzktuitzfhbdvcbx<ybfnhfgdhdbbvcxvxc"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c := []rune(a)[:0]
		for _, z := range a {
			c = append(c, z)
		}
		_ = c
	}
	PrintMemUsage()
}

func BenchmarkConvertSlice2(b *testing.B) {
	a := "dhffghfdghfdhfjfgbcvnbvmzktuitzfhbdvcbx<ybfnhfgdhdbbvcxvxc"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c := []byte(a)[:0]
		for _, z := range a {
			c = append(c, byte(z))
		}
		_ = c
	}
	PrintMemUsage()
}

func Benchmark2unidecode(b *testing.B) {
	str := `"Franä & Freddie's Diner	☺"`
	var str2 string
	for i := 0; i < b.N; i++ {
		str2 = unidecode.Unidecode(str)
	}
	b.Log(str2)
}

func Benchmark2unidecode2(b *testing.B) {
	str := `"Franä &—— Freddie's Diner	☺"`
	var str2 string
	for i := 0; i < b.N; i++ {
		str2 = unidecode2(str)
	}
	b.Log(str2)
}

func Benchmark3old(b *testing.B) {
	subRune := map[rune]bool{
		'a': true,
		'b': true,
		'c': true,
		'd': true,
		'e': true,
		'f': true,
		'g': true,
		'h': true,
		'i': true,
		'j': true,
		'k': true,
		'l': true,
		'm': true,
		'n': true,
		'o': true,
		'p': true,
		'q': true,
		'r': true,
		's': true,
		't': true,
		'u': true,
		'v': true,
		'w': true,
		'x': true,
		'y': true,
		'z': true,
		'0': true,
		'1': true,
		'2': true,
		'3': true,
		'4': true,
		'5': true,
		'6': true,
		'7': true,
		'8': true,
		'9': true,
		'-': true,
	}
	str := `"Franä & Freddie's Diner	☺"`
	for i := 0; i < b.N; i++ {
		if len(str) == 0 {
			return
		}
		var ok, cont bool
		var unwantedRunes []rune
		for _, r := range str {
			if _, ok = subRune[r]; !ok {
				cont = false
				for i := range unwantedRunes {
					if r == unwantedRunes[i] {
						cont = true
						break
					}
				}
				if !cont {
					unwantedRunes = append(unwantedRunes, r)
				}
			}
		}
		for idx := range unwantedRunes {
			str = strings.ReplaceAll(str, string(unwantedRunes[idx]), "-")
		}
		// clear(unwantedRunes)
	}
	b.Log(str)
}

func Benchmark3new(b *testing.B) {
	str := `"Franä &—— Freddie's Diner	☺"`
	rexexalllowernumber := regexp.MustCompile(`[^a-z0-9\-]`)

	for i := 0; i < b.N; i++ {
		if len(str) == 0 {
			return
		}
		str = rexexalllowernumber.ReplaceAllString(str, `-`)
	}
	b.Log(str)
}

func BenchmarkQueryString1(b *testing.B) {
	movieid := "Test123"
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		buf.WriteString("https://api.trakt.tv/movies/")
		buf.WriteString(movieid)
		buf.WriteString("/aliases")
		url := buf.String()
		_ = url
	}
}

func BenchmarkQuery1(b *testing.B) {
	additionalQueryParams := "&extended=1&maxsize=6291456000"
	categories := "2030, 2035, 2040, 2045"
	episode := 1
	season := 10
	tvDBID := 55797
	apikey := ""
	apiPath := "/api"
	apiBaseURL := "https://api.nzbgeek.info"
	// customurl := ""
	// query := "test"
	// addquotesfortitlequery := false
	// outputAsJson := false
	// searchtype := "query"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// urlv := apiBaseURL + apiPath + "?apikey=" + apikey
		// if len(customurl) >= 1 {
		// 	urlv = customurl
		// }
		// query = url.PathEscape(query)
		// if addquotesfortitlequery {
		// 	query = "%22" + query + "%22"
		// }
		// json := ""
		// if outputAsJson {
		// 	json = "&o=json"
		// }
		// _ = fmt.Sprintf("%s&q=%s&cat=%s&dl=1&t=%s%s%s", urlv, query, categories, searchtype, json, additional_query_params)
		// _ = urlv + "&q=" + query + "&cat=" + categories + "&dl=1&t=" + searchtype + json + additional_query_params
		// fmt.Println(urlv + "&q=" + query + "&cat=" + categories + "&dl=1&t=" + searchtype + json + additional_query_params)
		// continue
		var buildurl bytes.Buffer
		buildurl.WriteString(apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(apikey)
		buildurl.WriteString("&tvdbid=")
		buildurl.WriteString(strconv.Itoa(tvDBID))
		buildurl.WriteString("&season=")
		buildurl.WriteString(strconv.Itoa(season))
		buildurl.WriteString("&ep=")
		buildurl.WriteString(strconv.Itoa(episode))
		buildurl.WriteString("&limit=")
		buildurl.WriteString("100")
		buildurl.WriteString("&cat=")
		buildurl.WriteString(categories)
		buildurl.WriteString("&dl=1&t=tvsearch")
		buildurl.WriteString("&o=json")
		buildurl.WriteString(additionalQueryParams)
		_ = buildurl.String()
		// database.CountRowsTest1("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserieTest1(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		// for idxserie := range dbseries {
		// importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkQuery121(b *testing.B) {
	additionalQueryParams := "&extended=1&maxsize=6291456000"
	categories := []int{2030, 2035, 2040, 2045}
	episode := 1
	season := 10
	tvDBID := 55797
	apikey := ""
	apiPath := "/api"
	apiBaseURL := "https://api.nzbgeek.info"
	for i := 0; i < b.N; i++ {
		var buildurl string
		buildurl += apiBaseURL
		buildurl += apiPath
		buildurl += "?apikey="
		buildurl += apikey
		buildurl += "&tvdbid="
		buildurl += strconv.Itoa(tvDBID)
		buildurl += "&season="
		buildurl += strconv.Itoa(season)
		buildurl += "&ep="
		buildurl += strconv.Itoa(episode)
		buildurl += "&limit="
		buildurl += "100"
		buildurl += "&cat="
		buildurl += joinCats(categories)
		buildurl += "&dl=1&t=tvsearch"
		buildurl += "&o=json"
		buildurl += additionalQueryParams
		// database.CountRowsTest1("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserieTest1(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		// for idxserie := range dbseries {
		// importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkTestRange1(b *testing.B) {
	Init()
	b.ResetTimer()
	b.ReportAllocs()
	database.RefreshMediaCacheTitles(true, true)
	for i := 0; i < b.N; i++ {
		c := database.GetCachedTwoStringArr(logger.CacheDBSeriesAlt, true, true)
		for idx := range c {
			_ = c[idx]
		}
	}
	PrintMemUsage()
}

func BenchmarkTestRange2(b *testing.B) {
	Init()
	b.ResetTimer()
	b.ReportAllocs()
	database.RefreshMediaCacheTitles(true, true)
	for i := 0; i < b.N; i++ {
		for _, d := range database.GetCachedTwoStringArr(logger.CacheDBSeriesAlt, true, true) {
			_ = d
		}
	}
	PrintMemUsage()
}

func BenchmarkQuery2(b *testing.B) {
	// Init()
	// b.ResetTimer()
	additionalQueryParams := "&extended=1&maxsize=6291456000"
	categories := []int{2030, 2035, 2040, 2045}
	episode := 1
	season := 10
	tvDBID := 55797
	apikey := ""
	apiPath := "/api"
	apiBaseURL := "https://api.nzbgeek.info"
	for i := 0; i < b.N; i++ {
		var buildurl bytes.Buffer
		buildurl.WriteString(apiBaseURL)
		buildurl.WriteString(apiPath)
		buildurl.WriteString("?apikey=")
		buildurl.WriteString(apikey)
		buildurl.WriteString("&tvdbid=")
		buildurl.WriteString(strconv.Itoa(tvDBID))
		buildurl.WriteString("&season=")
		buildurl.WriteString(strconv.Itoa(season))
		buildurl.WriteString("&ep=")
		buildurl.WriteString(strconv.Itoa(episode))
		buildurl.WriteString("&limit=")
		buildurl.WriteString("100")
		buildurl.WriteString("&cat=")
		buildurl.WriteString(joinCats(categories))
		buildurl.WriteString("&dl=1&t=tvsearch")
		buildurl.WriteString("&o=json")
		buildurl.WriteString(additionalQueryParams)
		fmt.Println(buildurl.String())
		// database.CountRowsTest2("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserieTest2(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		// database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		// for idxserie := range dbseries {
		// importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkJoinString1(b *testing.B) {
	Init()
	str := "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = str + str
	}
}

func BenchmarkJoinString2(b *testing.B) {
	Init()
	str := "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = logger.JoinStrings(str, str)
	}
}

func BenchmarkJoinString3(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = str + str
	}
}

func BenchmarkJoinString4(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = logger.JoinStrings(str, str)
	}
}

func BenchmarkJoinString5(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = str + str + str + str
	}
}

func BenchmarkJoinString6(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"
	b.ResetTimer()
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = logger.JoinStrings(str, str, str, str)
	}
}

func BenchmarkGrowRemove(b *testing.B) {
	var str []string
	for j := 0; j < 1000; j++ {
		str = append(str, strconv.Itoa(j))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := []string{}
		// c = append(c, str...)

		// logger.Grow(&c, len(str))
		c = append(c, str...)
		// logger.RemoveFromStringArray(&str, "500")
		_ = c
	}
}

func BenchmarkPrio1(b *testing.B) {
	b.ReportAllocs()
	str := "Hallo123"
	for i := 0; i < b.N; i++ {
		str = logger.UnquoteUnescape(str)
	}
}

func BenchmarkClose1(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = apiexternal.Nzbwithprio{
			WantedTitle:      "ffff",
			WantedAlternates: []database.DbstaticTwoStringOneInt{{Str1: "ffff", Str2: "ffff"}},
			Quality:          "test",
			Listname:         "test",
		}
		// a.Close()
	}
	PrintMemUsage()
}

func BenchmarkClose2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = apiexternal.Nzbwithprio{
			WantedTitle:      "ffff",
			WantedAlternates: []database.DbstaticTwoStringOneInt{{Str1: "ffff", Str2: "ffff"}},
			Quality:          "test",
			Listname:         "test",
		}
		// a.Close()
	}
	PrintMemUsage()
}

func BenchmarkRepeatString1(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		str := strings.Repeat(",?", 10)
		_ = str
	}
	PrintMemUsage()
}

func BenchmarkRepeatString2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var bld strings.Builder
		bld.Grow(2 * 10)
		for i := 0; i < 10; i++ {
			bld.WriteString(",?")
		}
		str := bld.String()
		_ = str
	}
	PrintMemUsage()
}

func BenchmarkPrio2(b *testing.B) {
	str := "Hallo123"
	for i := 0; i < b.N; i++ {
		str = logger.UnquoteUnescape(str)
	}
}

func BenchmarkMakeRemove2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		str := make([]string, 0, 1000)
		for j := 0; j < 1000; j++ {
			str = append(str, strconv.Itoa(j))
		}
		str2 := str[:0]
		for x := range str {
			if str[x] != "500" {
				str2 = append(str2, str[x])
			}
		}
	}
}

func BenchmarkPath(b *testing.B) {
	str := "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = Path(str, false)
	}
}

func BenchmarkPath2(b *testing.B) {
	str := "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = Path2(str, false)
	}
}

func Benchmark1Concat(b *testing.B) { // 132 ns/op
	// ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for i := 0; i < b.N; i++ {
		s := "sadsadsa" + "dsadsakdas;k" + "8930984" + "8930984" + "8930984" + "8930984" + strconv.Itoa(
			23,
		)
		_ = s
	}
}

func Benchmark1Printf(b *testing.B) { // 56.7 ns/op
	// ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for i := 0; i < b.N; i++ {
		s := fmt.Sprintf(
			"%s%s%s%s%s%s%d",
			"sadsadsa",
			"dsadsakdas;k",
			"8930984",
			"8930984",
			"8930984",
			"8930984",
			23,
		)
		_ = s
	}
}

func Benchmark1Builder(b *testing.B) { // 58.5
	// ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for i := 0; i < b.N; i++ {
		var s strings.Builder
		s.WriteString("sadsadsa")
		s.WriteString("dsadsakdas;k")
		s.WriteString("8930984")
		s.WriteString("8930984")
		s.WriteString("8930984")
		s.WriteString("8930984")
		s.WriteString(strconv.Itoa(23))
		_ = s.String()
	}
}

package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/csv"
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
	"slices"
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
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/music"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scheduler"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/mozillazg/go-unidecode"
	"github.com/mozillazg/go-unidecode/table"
	"github.com/pelletier/go-toml/v2"
)

// Note: For database queries in tests, we use existing struct types from syncops package:
// - syncops.DbstaticTwoStringOneInt (Str1, Str2, Num) for artist/album pairs
// - syncops.DbstaticThreeStringTwoInt (Str1, Str2, Str3, Num1, Num2) for more complex queries

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
		config.GetSettingsGeneral().TraktRedirectUrl,
	)
	apiexternal.NewTVmazeClient(
		config.GetSettingsGeneral().TvmazeLimiterSeconds,
		config.GetSettingsGeneral().TvmazeLimiterCalls,
		config.GetSettingsGeneral().TvmazeDisableTLSVerify,
		config.GetSettingsGeneral().TvmazeTimeoutSeconds,
	)
	worker.InitWorkerPools(
		config.GetSettingsGeneral().WorkerSearch,
		config.GetSettingsGeneral().WorkerFiles,
		config.GetSettingsGeneral().WorkerMetadata,
		config.GetSettingsGeneral().WorkerRSS,
		config.GetSettingsGeneral().WorkerIndexer,
	)

	database.InitImdbdb()

	logger.Logtype("info", 0).Msg("Check Database for Upgrades")
	// database.UpgradeDB()
	database.SetVars()

	searcher.Init()
	parser.GenerateAllQualityPriorities()

	parser.LoadDBPatterns()
	parser.GenerateCutoffPriorities()
	database.Refreshhistorycacheurl(config.MediaTypeSeries, true)
	database.Refreshhistorycachetitle(config.MediaTypeSeries, true)
	database.RefreshMediaCacheList(config.MediaTypeSeries, true)
	database.RefreshMediaCacheDB(config.MediaTypeSeries, true)
	database.RefreshMediaCacheTitles(config.MediaTypeSeries, true)
	database.Refreshunmatchedcached(config.MediaTypeSeries, true)
	database.Refreshfilescached(config.MediaTypeSeries, true)

	database.Refreshhistorycacheurl(config.MediaTypeMovie, true)
	database.Refreshhistorycachetitle(config.MediaTypeMovie, true)
	database.RefreshMediaCacheList(config.MediaTypeMovie, true)
	database.RefreshMediaCacheDB(config.MediaTypeMovie, true)
	database.RefreshMediaCacheTitles(config.MediaTypeMovie, true)
	database.Refreshunmatchedcached(config.MediaTypeMovie, true)
	database.Refreshfilescached(config.MediaTypeMovie, true)
}

func TestCache(t *testing.T) {
	Init()
	database.RefreshMediaCacheDB(config.MediaTypeSeries, true)
	database.RefreshMediaCacheTitles(config.MediaTypeSeries, true)

	t.Log(
		database.GetCachedThreeStringArr(
			mtstrings.GetStringsMap(config.MediaTypeSeries, logger.CacheDBMedia),
			true,
			true,
		)[:4],
	)
	t.Log(
		database.GetCachedTwoIntArr(
			mtstrings.GetStringsMap(config.MediaTypeSeries, logger.CacheMedia),
			true,
			true,
		)[:4],
	)

	t.Log(
		database.GetCachedTwoStringArr(
			mtstrings.GetStringsMap(config.MediaTypeSeries, logger.CacheMediaTitles),
			true,
			true,
		)[:4],
	)
}

func TestInitCfg(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		_, err := toml.Marshal(config.GetToml().Media)

		t.Log(err)
		// t.Log(string(cnt))
	})
}

func TestDBType(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		dbrows := database.GetrowsType(database.Serie{}, false, 1000, "select * from series")
		if len(dbrows) == 0 {
			t.Error("no rows")
		}
		t.Log(len(dbrows))
		t.Log(dbrows[0])
		t.Log(dbrows[1])
		dbrows2 := database.GetrowsN[database.DbstaticTwoUint](
			false,
			10,
			"select id, dbserie_id from series",
		)
		if len(dbrows2) == 0 {
			t.Error("no rows 2")
		}
		t.Log(len(dbrows2))
		t.Log(dbrows2[0])
		t.Log(dbrows2[1])

		dbrows3 := database.GetrowsN[uint](
			false,
			10,
			"select id from series order by id asc",
		)
		if len(dbrows3) == 0 {
			t.Error("no rows 3")
		}
		t.Log(len(dbrows3))
		t.Log(dbrows3[0])
		t.Log(dbrows3[1])
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
		{"Just under 360p", 640, 359, "240p"},
		{"Just over 360p", 640, 361, "360p"},
		{"Just under 1080p", 1920, 1079, "720p"},
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

func TestGetResolutionFromDimensionsNew(t *testing.T) {
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
		{"360p", 576, 320, "240p"},     // 1.8 - height 320, rounds down to 240p
		{"360p", 624, 352, "240p"},     // 1.77 - height 352, rounds down to 240p
		{"240p", 426, 240, "240p"},

		// Ultra-wide resolutions - match by height
		{"Ultra-wide 1080p", 2560, 1080, "1080p"}, // 2.37:1 - height 1080
		{"Ultra-wide 1440p", 3440, 1440, "1440p"}, // 2.38:1 - height 1440
		{"Ultra-wide 4K", 5120, 2160, "2160p"},    // 2.37:1 - height 2160

		// Edge cases
		{"Very low resolution", 160, 120, "SD"},
		{"Just under 360p", 576, 320, "240p"}, // height 320, rounds down to 240p

		// Borderline cases - height based rounding
		{"Just under 360p", 640, 359, "240p"},    // height 359 < 360 threshold → 240p
		{"Exactly 360p", 640, 360, "360p"},       // height 360
		{"Just over 360p", 640, 361, "360p"},     // height 361 >= 360 threshold → 360p
		{"Just under 1080p", 1920, 1079, "720p"}, // height 1079 < 1080 threshold → 720p
		{"Exactly 1080p", 1920, 1080, "1080p"},
		{"Just over 1080p", 1920, 1081, "1080p"}, // height 1081 >= 1080 threshold → 1080p
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := database.ParseInfo{
				Width:  tt.width,
				Height: tt.height,
			}
			result := m.Parseresolution()
			if result != tt.expected {
				t.Errorf("GetResolutionFromDimensions(%d, %d) = %s; expected %s (aspect: %.2f)",
					tt.width, tt.height, result, tt.expected, float64(tt.width)/float64(tt.height))
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

func TestSerieGetMetadata(t *testing.T) {
	Init()
	// TVDB ID 79169 = Stargate SG-1 (well-known, stable metadata)
	const tvdbID = 79169
	t.Run("tvdb_metadata_populated", func(t *testing.T) {
		var dbserie database.Dbserie
		dbserie.ThetvdbID = tvdbID

		aliases := metadata.SerieGetMetadata(&dbserie, "", true, true, true, []string{})

		// Check TVDB provider is reachable
		if dbserie.Seriename == "" && dbserie.ImdbID == "" && dbserie.Status == "" {
			t.Error(
				"FATAL: no TVDB fields populated at all — provider may not be initialized or credentials missing",
			)
			return
		}

		// Report each field
		fields := []struct {
			name  string
			value string
		}{
			{"Seriename", dbserie.Seriename},
			{"Status", dbserie.Status},
			{"Firstaired", dbserie.Firstaired},
			{"Network", dbserie.Network},
			{"Runtime", dbserie.Runtime},
			{"Language", dbserie.Language},
			{"Genre", dbserie.Genre},
			{"Overview", dbserie.Overview},
			{"Rating", dbserie.Rating},
			{"Siterating", dbserie.Siterating},
			{"SiteratingCount", dbserie.SiteratingCount},
			{"Slug", dbserie.Slug},
			{"ImdbID", dbserie.ImdbID},
			{"Banner", dbserie.Banner},
			{"Poster", dbserie.Poster},
			{"Fanart", dbserie.Fanart},
		}
		for _, f := range fields {
			if f.value == "" {
				t.Logf("WARN: %s is empty", f.name)
			} else {
				t.Logf("OK:   %s = %q", f.name, f.value)
			}
		}
		t.Logf("TraktID = %d", dbserie.TraktID)
		t.Logf("ThetvdbID = %d", dbserie.ThetvdbID)
		t.Logf("Aliases returned: %v", aliases)

		if dbserie.Seriename == "" {
			t.Error("Seriename must not be empty after TVDB fetch")
		}
		if dbserie.Status == "" {
			t.Error("Status must not be empty after TVDB fetch")
		}
	})
}

func TestSQL(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		a := database.GetrowsN[syncops.DbstaticThreeStringTwoInt](
			false,
			database.Getdatarow[uint](false, "select count() from dbmovies")+100,
			"select title, slug, imdb_id, year, id from dbmovies limit 4",
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
			"select distinct title from dbmovies limit 4",
		)
		t.Log(b)
		ab := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
			false,
			database.Getdatarow[uint](
				false,
				"select count() from dbserie_alternates where title != ''",
			)+100,
			"select title, slug, dbserie_id from dbserie_alternates where title != '' limit 4",
		)
		t.Log(ab)
		t.Log(len(ab))
		t.Log(cap(ab))

		database.RefreshMediaCacheTitles(config.MediaTypeSeries, true)
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

		bld.WriteStringMap(cfgp.IsType, logger.SearchGenTable)
		if searchmissing {
			bld.WriteStringMap(cfgp.IsType, logger.SearchGenMissing)
			bld.WriteString(cfgp.ListsQu)
			bld.WriteStringMap(cfgp.IsType, logger.SearchGenMissingEnd)
		} else {
			bld.WriteStringMap(cfgp.IsType, logger.SearchGenReached)
			bld.WriteString(cfgp.ListsQu)
			bld.WriteByte(')')
		}
		if scaninterval != 0 {
			bld.WriteStringMap(cfgp.IsType, logger.SearchGenLastScan)
			timeinterval := logger.TimeGetNow().AddDate(0, 0, 0-scaninterval)
			args.Arr = append(args.Arr, &timeinterval)
		}
		if scandatepre != 0 {
			bld.WriteStringMap(cfgp.IsType, logger.SearchGenDate)
			timedatepre := logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
			args.Arr = append(args.Arr, &timedatepre)
		}
		bld.WriteStringMap(cfgp.IsType, logger.SearchGenOrder)
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
			logger.JoinStrings(mtstrings.GetStringsMap(cfgp.IsType, logger.SearchGenSelect), str),
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
	// bla, _ := json.Marshal(results)
	// b.Log(string(bla))
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
		args = append(args, &cfgp.Lists[i].Name)
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
	switch cfgp.IsType {
	case config.MediaTypeMovie:
		{
			bld.WriteString(
				" from movies inner join dbmovies on dbmovies.id=movies.dbmovie_id where ",
			)

			if searchmissing {
				bld.WriteString(
					"dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			} else {
				bld.WriteString(
					"dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
				)
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
		}
	case config.MediaTypeSeries:
		{
			bld.WriteString(
				" from serie_episodes inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id where ",
			)

			if searchmissing {
				bld.WriteString(
					"serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(
					") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)",
				)
			} else {
				bld.WriteString(
					"serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			}
			if scaninterval != 0 {
				bld.WriteString(
					" and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)",
				)
				args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
			}
			if scandatepre != 0 {
				bld.WriteString(
					" and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)",
				)
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
	case config.MediaTypeBook:
		{
			bld.WriteString(" from books inner join dbbooks on dbbooks.id=books.dbbook_id where ")

			if searchmissing {
				bld.WriteString("dbbooks.year != 0 and books.missing = 1 and books.listname in (?")
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			} else {
				bld.WriteString(
					"dbbooks.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			}
			if scaninterval != 0 {
				bld.WriteString(" and (books.lastscan is null or books.lastscan < ?)")
				args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
			}
			if scandatepre != 0 {
				bld.WriteString(" and (dbbooks.publish_date < ? or dbbooks.publish_date is null)")
				if scaninterval != 0 {
					args[cfgp.ListsLen+1] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
				} else {
					args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
				}
			}
			bld.WriteString(" order by books.lastscan asc")
			if searchinterval != 0 {
				bld.WriteString(" limit ")
				bld.WriteString(strconv.Itoa(searchinterval))
			}
			query = "select books.id " + bld.String()
		}
	case config.MediaTypeAudiobook:
		{
			bld.WriteString(
				" from audiobooks inner join dbaudiobooks on dbaudiobooks.id=audiobooks.dbaudiobook_id where ",
			)

			if searchmissing {
				bld.WriteString(
					"dbaudiobooks.year != 0 and audiobooks.missing = 1 and audiobooks.listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			} else {
				bld.WriteString(
					"dbaudiobooks.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			}
			if scaninterval != 0 {
				bld.WriteString(" and (audiobooks.lastscan is null or audiobooks.lastscan < ?)")
				args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
			}
			if scandatepre != 0 {
				bld.WriteString(
					" and (dbaudiobooks.release_date < ? or dbaudiobooks.release_date is null)",
				)
				if scaninterval != 0 {
					args[cfgp.ListsLen+1] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
				} else {
					args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
				}
			}
			bld.WriteString(" order by audiobooks.lastscan asc")
			if searchinterval != 0 {
				bld.WriteString(" limit ")
				bld.WriteString(strconv.Itoa(searchinterval))
			}
			query = "select audiobooks.id " + bld.String()
		}
	case config.MediaTypeMusic:
		{
			bld.WriteString(
				" from albums inner join dbalbums on dbalbums.id=albums.dbalbum_id where ",
			)

			if searchmissing {
				bld.WriteString(
					"dbalbums.year != 0 and albums.missing = 1 and albums.listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			} else {
				bld.WriteString(
					"dbalbums.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
				)
				bld.WriteString(cfgp.ListsQu)
				bld.WriteString(")")
			}
			if scaninterval != 0 {
				bld.WriteString(" and (albums.lastscan is null or albums.lastscan < ?)")
				args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
			}
			if scandatepre != 0 {
				bld.WriteString(" and (dbalbums.release_date < ? or dbalbums.release_date is null)")
				if scaninterval != 0 {
					args[cfgp.ListsLen+1] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
				} else {
					args[cfgp.ListsLen] = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
				}
			}
			bld.WriteString(" order by albums.lastscan asc")
			if searchinterval != 0 {
				bld.WriteString(" limit ")
				bld.WriteString(strconv.Itoa(searchinterval))
			}
			query = "select albums.id " + bld.String()
		}
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
			parser.GetDBIDs(m, cfgp, true, false)
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
	return slices.Contains(s.Values, valchk)
}

func (s *UIntSet) Clear() {
	s.Values = nil
}

// Benchmarks

func BenchmarkQueryLower(b *testing.B) {
	Init()

	str := ""
	var id1 uint = 32
	var id2 uint = 32
	var id3 uint = 32
	var id4 uint = 32
	for b.Loop() {
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
	logger.Logtype("info", 0).Msg(str)
}

func BenchmarkQueryLower2(b *testing.B) {
	Init()
	b.ReportAllocs()

	str := "Movie"
	for b.Loop() {
		if strings.EqualFold(str, logger.StrMovie) {
			continue
		}
	}
}

func BenchmarkArr1(b *testing.B) {
	for b.Loop() {
		a := []string{"extended", "extended cut", "extended.cut", "extended-cut", "extended_cut"}
		_ = a
	}
}

func BenchmarkArr2(b *testing.B) {
	c := "extended,extended cut,extended.cut,extended-cut,extended_cut"
	for b.Loop() {
		a := strings.Split(c, ",")
		_ = a
	}
}

func BenchmarkQuery11(b *testing.B) {
	Init()

	var cachetconst map[uint32]struct{}
	for b.Loop() {
		cachetconst = make(map[uint32]struct{}, 1200000)
		for i := range 1000000 {
			cachetconst[uint32(i)] = struct{}{}
		}
		// _, _ = cachetconst[uint32(999555)]
	}
}

func BenchmarkQuery12(b *testing.B) {
	Init()

	for b.Loop() {
		cachetconst := NewUintSetMaxSize(1200000)
		for i := range 1000000 {
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
	for b.Loop() {
		structure.OrganizeSingleFolder(ctx, cfgFolder, cfgp, cfgimport, "en movies", true, false, 0)
	}
}

func BenchmarkQuery15(b *testing.B) {
	Init()
	var x string

	for b.Loop() {
		importfeed.JobImportMovies(x, nil, -1, false)
	}
}

func BenchmarkAllowRange(b *testing.B) {
	Init()
	arr := database.GetrowsN[syncops.DbstaticThreeStringTwoInt](
		false,
		database.Getdatarow[uint](false, "select count() from dbmovies")+100,
		"select title, slug, imdb_id, year, id from dbmovies",
	)

	b.ReportAllocs()
	for b.Loop() {
		for idx := range arr {
			_ = idx
		}
	}
	PrintMemUsage()
}

func BenchmarkAllowRange2(b *testing.B) {
	Init()
	arr := database.GetrowsN[syncops.DbstaticThreeStringTwoInt](
		false,
		database.Getdatarow[uint](false, "select count() from dbmovies")+100,
		"select title, slug, imdb_id, year, id from dbmovies",
	)

	b.ReportAllocs()
	for b.Loop() {
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

	for b.Loop() {
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

	for b.Loop() {
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
				logger.Logtype("error", 0).Err(err).Msg("an error occurred while parsing csv")
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

	for b.Loop() {
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

	for b.Loop() {
		// scanner.MoveFileDriveBufferNew(val, newpath)
		// scanner.MoveFileDriveBuffer(val, newpath)
		// scanner.MoveFileDrive(val, newpath)
	}
}

func Benchmark5Buffer3(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		for range 10000 {
			FilepathJoinSeperator3("/mnt/user/", "folder")
		}
	}
}

func BenchmarkByte1(b *testing.B) {
	byteArray := []byte{'J', 'A', 'N', 'E'}
	b.ReportAllocs()
	for b.Loop() {
		str1 := string(byteArray)
		_ = str1
	}
}

func BenchmarkByte3(b *testing.B) {
	byteArray := []byte{'J', 'A', 'N', 'E'}
	b.ReportAllocs()
	for b.Loop() {
		str1 := string(byteArray)
		_ = str1
	}
}

func BenchmarkWalkDir(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		filepath.WalkDir("C:\\", func(path string, d fs.DirEntry, err error) error {
			return nil
		})
	}
}

func BenchmarkWalkDirOrg(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
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
	for range 100 {
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
	for b.Loop() {
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
	for b.Loop() {
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
	for b.Loop() {
		str2 = unidecode.Unidecode(str)
	}
	b.Log(str2)
}

func Benchmark2unidecode2(b *testing.B) {
	str := `"Franä &—— Freddie's Diner	☺"`
	var str2 string
	for b.Loop() {
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
	for b.Loop() {
		if len(str) == 0 {
			return
		}
		var ok, cont bool
		var unwantedRunes []rune
		for _, r := range str {
			if _, ok = subRune[r]; !ok {
				cont = slices.Contains(unwantedRunes, r)
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

	for b.Loop() {
		if len(str) == 0 {
			return
		}
		str = rexexalllowernumber.ReplaceAllString(str, `-`)
	}
	b.Log(str)
}

func BenchmarkQueryString1(b *testing.B) {
	movieid := "Test123"
	for b.Loop() {
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

	for b.Loop() {
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
	for b.Loop() {
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

	b.ReportAllocs()
	database.RefreshMediaCacheTitles(config.MediaTypeSeries, true)
	for b.Loop() {
		c := database.GetCachedTwoStringArr(logger.CacheDBSeriesAlt, true, true)
		for idx := range c {
			_ = c[idx]
		}
	}
	PrintMemUsage()
}

func BenchmarkTestRange2(b *testing.B) {
	Init()

	b.ReportAllocs()
	database.RefreshMediaCacheTitles(config.MediaTypeSeries, true)
	for b.Loop() {
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
	for b.Loop() {
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

	b.ReportAllocs()
	for b.Loop() {
		_ = str + str
	}
}

func BenchmarkJoinString2(b *testing.B) {
	Init()
	str := "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"

	b.ReportAllocs()
	for b.Loop() {
		_ = logger.JoinStrings(str, str)
	}
}

func BenchmarkJoinString3(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"

	b.ReportAllocs()
	for b.Loop() {
		_ = str + str
	}
}

func BenchmarkJoinString4(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"

	b.ReportAllocs()
	for b.Loop() {
		_ = logger.JoinStrings(str, str)
	}
}

func BenchmarkJoinString5(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"

	b.ReportAllocs()
	for b.Loop() {
		_ = str + str + str + str
	}
}

func BenchmarkJoinString6(b *testing.B) {
	Init()
	str := "dfgdTTTVdbfnh"

	b.ReportAllocs()
	for b.Loop() {
		_ = logger.JoinStrings(str, str, str, str)
	}
}

func BenchmarkGrowRemove(b *testing.B) {
	var str []string
	for j := range 1000 {
		str = append(str, strconv.Itoa(j))
	}

	for b.Loop() {
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
	for b.Loop() {
		str = logger.UnquoteUnescape(str)
	}
}

func BenchmarkClose1(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = apiexternal.Nzbwithprio{
			WantedTitle:      "ffff",
			WantedAlternates: []syncops.DbstaticTwoStringOneInt{{Str1: "ffff", Str2: "ffff"}},
			Quality:          "test",
			Listname:         "test",
		}
		// a.Close()
	}
	PrintMemUsage()
}

func BenchmarkClose2(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = apiexternal.Nzbwithprio{
			WantedTitle:      "ffff",
			WantedAlternates: []syncops.DbstaticTwoStringOneInt{{Str1: "ffff", Str2: "ffff"}},
			Quality:          "test",
			Listname:         "test",
		}
		// a.Close()
	}
	PrintMemUsage()
}

func BenchmarkRepeatString1(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		str := strings.Repeat(",?", 10)
		_ = str
	}
	PrintMemUsage()
}

func BenchmarkRepeatString2(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var bld strings.Builder
		bld.Grow(2 * 10)
		for range 10 {
			bld.WriteString(",?")
		}
		str := bld.String()
		_ = str
	}
	PrintMemUsage()
}

func BenchmarkPrio2(b *testing.B) {
	str := "Hallo123"
	for b.Loop() {
		str = logger.UnquoteUnescape(str)
	}
}

func BenchmarkMakeRemove2(b *testing.B) {
	for b.Loop() {
		str := make([]string, 0, 1000)
		for j := range 1000 {
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
	for b.Loop() {
		_ = Path(str, false)
	}
}

func BenchmarkPath2(b *testing.B) {
	str := "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for b.Loop() {
		_ = Path2(str, false)
	}
}

func Benchmark1Concat(b *testing.B) { // 132 ns/op
	// ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for b.Loop() {
		s := "sadsadsa" + "dsadsakdas;k" + "8930984" + "8930984" + "8930984" + "8930984" + strconv.Itoa(
			23,
		)
		_ = s
	}
}

func Benchmark1Printf(b *testing.B) { // 56.7 ns/op
	// ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for b.Loop() {
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
	for b.Loop() {
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

// TestMusicAlbumDatabaseLookup tests the artist-first database lookup for music albums.
// This test loads the database and tries to find existing albums using various NZB title formats.
// Run with: go test -v -run TestMusicAlbumDatabaseLookup
func TestMusicAlbumDatabaseLookup(t *testing.T) {
	Init()

	// Query existing albums with their artists from the database
	// Uses syncops.DbstaticTwoStringOneInt: Str1=artist, Str2=album, Num=id
	existingAlbums := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		100,
		`SELECT ar.name as str1, a.title as str2, a.id as num
		 FROM dbalbums a
		 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
		 JOIN dbartists ar ON aa.dbartist_id = ar.id
		 LIMIT 100`,
	)

	if len(existingAlbums) == 0 {
		t.Skip("No albums in database to test against")
		return
	}

	t.Logf("Found %d albums in database to test against", len(existingAlbums))

	// Test various NZB title formats against the database
	// album.Str1 = artist name, album.Str2 = album title, album.Num = dbalbum_id
	for i, album := range existingAlbums {
		if i >= 20 { // Limit to first 20 albums for speed
			break
		}

		t.Run(fmt.Sprintf("Album_%d_%s", i, album.Str2), func(t *testing.T) {
			// Generate various NZB-style title formats
			nzbFormats := []string{
				// Standard format: "Artist - Album"
				album.Str1 + " - " + album.Str2,
				// Scene format with dots: "Artist.Name-Album.Title"
				strings.ReplaceAll(
					album.Str1,
					" ",
					".",
				) + "-" + strings.ReplaceAll(
					album.Str2,
					" ",
					".",
				),
				// Scene format with year and group
				strings.ReplaceAll(
					album.Str1,
					" ",
					".",
				) + "-" + strings.ReplaceAll(
					album.Str2,
					" ",
					".",
				) + "-2020-GROUP",
				// Scene format with quality tags
				strings.ReplaceAll(
					album.Str1,
					" ",
					".",
				) + "-" + strings.ReplaceAll(
					album.Str2,
					" ",
					".",
				) + "-FLAC-2020-GROUP",
				// Underscore format
				strings.ReplaceAll(
					album.Str1,
					" ",
					"_",
				) + "-" + strings.ReplaceAll(
					album.Str2,
					" ",
					"_",
				),
			}

			for _, nzbTitle := range nzbFormats {
				m := &database.ParseInfo{
					File:  nzbTitle,
					Title: nzbTitle,
				}

				// Test the artist-first lookup
				m.FindDbalbumByArtistFirst()

				switch m.DbalbumID {
				case 0:
					t.Logf("FAILED to find: %s (expected DbalbumID: %d)", nzbTitle, album.Num)
				case album.Num:
					t.Logf("SUCCESS: Found album with title format: %s", nzbTitle)
				default:
					t.Logf(
						"PARTIAL: Found different album ID %d (expected %d) for: %s",
						m.DbalbumID,
						album.Num,
						nzbTitle,
					)
				}
			}
		})
	}
}

// TestMusicAlbumDatabaseLookupWithNZBTitles tests specific NZB title patterns
// that have been known to cause issues in parsing.
// Run with: go test -v -run TestMusicAlbumDatabaseLookupWithNZBTitles
func TestMusicAlbumDatabaseLookupWithNZBTitles(t *testing.T) {
	Init()

	// These are example NZB titles that should match albums in the database
	// You should update these based on what's actually in your database
	testCases := []struct {
		name           string
		nzbTitle       string
		expectedArtist string
		expectedAlbum  string
		expectMatch    bool
		description    string
	}{
		{
			name:           "Scene format with dots",
			nzbTitle:       "Alan.Silvestri-Predator.2-OST-1990-EOS",
			expectedArtist: "Alan Silvestri",
			expectedAlbum:  "Predator 2",
			expectMatch:    true,
			description:    "Scene release with dots as separators",
		},
		{
			name:           "Standard format",
			nzbTitle:       "Pink Floyd - The Dark Side of the Moon (1973) FLAC",
			expectedArtist: "Pink Floyd",
			expectedAlbum:  "The Dark Side of the Moon",
			expectMatch:    true,
			description:    "Standard artist - album format with year",
		},
		{
			name:           "Scene format with quality tags",
			nzbTitle:       "Metallica-Master_Of_Puppets-REMASTERED-CD-FLAC-2017-GROUP",
			expectedArtist: "Metallica",
			expectedAlbum:  "Master Of Puppets",
			expectMatch:    true,
			description:    "Scene format with underscores and quality tags",
		},
		{
			name:           "Multiple artists with ampersand",
			nzbTitle:       "Daft Punk & Pharrell Williams - Get Lucky (2013) MP3",
			expectedArtist: "Daft Punk",
			expectedAlbum:  "Get Lucky",
			expectMatch:    true,
			description:    "Multiple artists separated by &",
		},
		{
			name:           "OST/Soundtrack release",
			nzbTitle:       "Howard Ashman and Alan Menken-Little Shop Of Horrors-OST-CD-FLAC-1986-KINDA",
			expectedArtist: "Howard Ashman",
			expectedAlbum:  "Little Shop Of Horrors",
			expectMatch:    true,
			description:    "Soundtrack with multiple composers",
		},
		{
			name:           "Scene format with dot-dash-dot separator",
			nzbTitle:       "Paul.K.Lunow.-.Riaru.-.Willkommen.im.modernsten.Unternehmen.der.Welt",
			expectedArtist: "Paul K Lunow",
			expectedAlbum:  "Riaru Willkommen im modernsten Unternehmen der Welt",
			expectMatch:    true,
			description:    "German audiobook scene format with .-. separator",
		},
		{
			name:           "Audiobook with scene tags at end",
			nzbTitle:       "Hans Joachim Kulenkampff-Weihnachtsgeschichten-DE-AUDIOBOOK-CD-FLAC-2001-oNePiEcE",
			expectedArtist: "Hans Joachim Kulenkampff",
			expectedAlbum:  "Weihnachtsgeschichten",
			expectMatch:    true,
			description:    "Audiobook with country code, format, year and release group",
		},
		{
			name:           "Audiobook with underscores and scene tags",
			nzbTitle:       "Hans_Joachim_Kulenkampff-Glocken__Glockengelaeut.FLAC-DE-AUDIOBOOK",
			expectedArtist: "Hans Joachim Kulenkampff",
			expectedAlbum:  "Glocken Glockengelaeut",
			expectMatch:    true,
			description:    "Audiobook with underscores in name and scene tags",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &database.ParseInfo{
				File:  tc.nzbTitle,
				Title: tc.nzbTitle,
			}

			// Test the artist-first lookup
			m.FindDbalbumByArtistFirst()

			t.Logf("Input NZB title: %s", tc.nzbTitle)
			t.Logf("Expected Artist: %s, Expected Album: %s", tc.expectedArtist, tc.expectedAlbum)
			t.Logf("Result: DbalbumID=%d, Artist=%s", m.DbalbumID, m.Artist)

			if tc.expectMatch && m.DbalbumID == 0 {
				// Check if the artist exists in the database first
				var artistID uint
				database.Scanrowsdyn(
					false,
					"select id from dbartists where name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE",
					&artistID,
					&tc.expectedArtist,
					&tc.expectedArtist,
				)

				if artistID == 0 {
					t.Logf(
						"NOTE: Artist '%s' not found in database - this is expected if not imported",
						tc.expectedArtist,
					)
				} else {
					// Artist exists, check if album exists
					var albumID uint
					database.Scanrowsdyn(
						false,
						`SELECT a.id FROM dbalbums a
						 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
						 WHERE aa.dbartist_id = ? AND a.title LIKE ? COLLATE NOCASE LIMIT 1`,
						&albumID,
						&artistID,
						"%"+tc.expectedAlbum+"%",
					)
					if albumID == 0 {
						t.Logf(
							"NOTE: Album '%s' not found for artist ID %d - this is expected if not imported",
							tc.expectedAlbum,
							artistID,
						)
					} else {
						t.Errorf(
							"FAILED: Album '%s' (ID %d) exists but lookup failed",
							tc.expectedAlbum,
							albumID,
						)
					}
				}
			} else if m.DbalbumID != 0 {
				t.Logf("SUCCESS: Found DbalbumID=%d for '%s'", m.DbalbumID, tc.description)
			}
		})
	}
}

// TestAudiobookDatabaseLookup tests the author-first database lookup for audiobooks.
// Run with: go test -v -run TestAudiobookDatabaseLookup
func TestAudiobookDatabaseLookup(t *testing.T) {
	Init()

	// Query existing audiobooks with their authors from the database
	// Uses syncops.DbstaticTwoStringOneInt: Str1=author, Str2=title, Num=id
	existingBooks := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		100,
		`SELECT au.name as str1, ab.title as str2, ab.id as num
		 FROM dbaudiobooks ab
		 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
		 JOIN dbauthors au ON aa.dbauthor_id = au.id
		 LIMIT 100`,
	)

	if len(existingBooks) == 0 {
		t.Skip("No audiobooks in database to test against")
		return
	}

	t.Logf("Found %d audiobooks in database to test against", len(existingBooks))

	// Test various NZB title formats against the database
	// book.Str1 = author name, book.Str2 = book title, book.Num = dbaudiobook_id
	for i, book := range existingBooks {
		if i >= 20 { // Limit to first 20 books for speed
			break
		}

		t.Run(fmt.Sprintf("Audiobook_%d_%s", i, book.Str2), func(t *testing.T) {
			// Generate various NZB-style title formats
			nzbFormats := []string{
				// Standard format: "Author - Title"
				book.Str1 + " - " + book.Str2,
				// Scene format with dots
				strings.ReplaceAll(
					book.Str1,
					" ",
					".",
				) + "-" + strings.ReplaceAll(
					book.Str2,
					" ",
					".",
				),
				// Scene format with year
				book.Str1 + " - " + book.Str2 + " (2020)",
				// With audiobook tag
				book.Str1 + " - " + book.Str2 + " [Audiobook]",
			}

			for _, nzbTitle := range nzbFormats {
				m := &database.ParseInfo{
					File:  nzbTitle,
					Title: nzbTitle,
				}

				// Test the author-first lookup
				m.FindDbaudiobookByAuthorFirst()

				switch m.DbaudiobookID {
				case 0:
					t.Logf("FAILED to find: %s (expected DbaudiobookID: %d)", nzbTitle, book.Num)
				case book.Num:
					t.Logf("SUCCESS: Found audiobook with title format: %s", nzbTitle)
				default:
					t.Logf(
						"PARTIAL: Found different audiobook ID %d (expected %d) for: %s",
						m.DbaudiobookID,
						book.Num,
						nzbTitle,
					)
				}
			}
		})
	}
}

// TestAudiobookDatabaseLookupWithNZBTitles tests specific audiobook NZB title patterns.
// Run with: go test -v -run TestAudiobookDatabaseLookupWithNZBTitles
func TestAudiobookDatabaseLookupWithNZBTitles(t *testing.T) {
	Init()

	testCases := []struct {
		name           string
		nzbTitle       string
		expectedAuthor string
		expectedTitle  string
		expectMatch    bool
		description    string
	}{
		{
			name:           "Standard format",
			nzbTitle:       "Stephen King - The Shining (1977)",
			expectedAuthor: "Stephen King",
			expectedTitle:  "The Shining",
			expectMatch:    true,
			description:    "Standard author - title format with year",
		},
		{
			name:           "Scene format",
			nzbTitle:       "Stephen.King-The.Shining-Audiobook-MP3-128k-2020-GROUP",
			expectedAuthor: "Stephen King",
			expectedTitle:  "The Shining",
			expectMatch:    true,
			description:    "Scene format with quality tags",
		},
		{
			name:           "With narrator",
			nzbTitle:       "Brandon Sanderson - Mistborn Read by Michael Kramer (2006)",
			expectedAuthor: "Brandon Sanderson",
			expectedTitle:  "Mistborn",
			expectMatch:    true,
			description:    "With narrator in title",
		},
		{
			name:           "Dan Brown example",
			nzbTitle:       "Dan Brown - The Da Vinci Code [Audiobook]",
			expectedAuthor: "Dan Brown",
			expectedTitle:  "The Da Vinci Code",
			expectMatch:    true,
			description:    "Standard format with audiobook tag",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &database.ParseInfo{
				File:  tc.nzbTitle,
				Title: tc.nzbTitle,
			}

			// Test the author-first lookup
			m.FindDbaudiobookByAuthorFirst()

			t.Logf("Input NZB title: %s", tc.nzbTitle)
			t.Logf("Expected Author: %s, Expected Title: %s", tc.expectedAuthor, tc.expectedTitle)
			t.Logf("Result: DbaudiobookID=%d, Artist=%s", m.DbaudiobookID, m.Artist)

			if tc.expectMatch && m.DbaudiobookID == 0 {
				// Check if the author exists in the database first
				var authorID uint
				database.Scanrowsdyn(
					false,
					"select id from dbauthors where name = ? COLLATE NOCASE",
					&authorID,
					&tc.expectedAuthor,
				)

				if authorID == 0 {
					t.Logf(
						"NOTE: Author '%s' not found in database - this is expected if not imported",
						tc.expectedAuthor,
					)
				} else {
					// Author exists, check if book exists
					var bookID uint
					database.Scanrowsdyn(
						false,
						`SELECT ab.id FROM dbaudiobooks ab
						 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
						 WHERE aa.dbauthor_id = ? AND ab.title LIKE ? COLLATE NOCASE LIMIT 1`,
						&bookID,
						&authorID,
						"%"+tc.expectedTitle+"%",
					)
					if bookID == 0 {
						t.Logf(
							"NOTE: Book '%s' not found for author ID %d - this is expected if not imported",
							tc.expectedTitle,
							authorID,
						)
					} else {
						t.Errorf(
							"FAILED: Book '%s' (ID %d) exists but lookup failed",
							tc.expectedTitle,
							bookID,
						)
					}
				}
			} else if m.DbaudiobookID != 0 {
				t.Logf("SUCCESS: Found DbaudiobookID=%d for '%s'", m.DbaudiobookID, tc.description)
			}
		})
	}
}

// TestMusicGetDBIDsFull tests the complete GetDBIDsFull flow for music albums.
// This simulates what happens during an NZB search.
// Run with: go test -v -run TestMusicGetDBIDsFull
func TestMusicGetDBIDsFull(t *testing.T) {
	Init()

	// Get a music config if one exists
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(name string, loop *config.MediaTypeConfig) bool {
		if loop.IsType == config.MediaTypeMusic {
			cfgp = loop
			return true
		}
		return false
	})

	if cfgp == nil {
		t.Skip("No music media config found")
		return
	}

	t.Logf("Using music config: %s", cfgp.Name)

	// Get some existing albums to test against
	// Uses syncops.DbstaticTwoStringOneInt: Str1=artist, Str2=album, Num=id
	existingAlbums := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		50,
		`SELECT ar.name as str1, a.title as str2, a.id as num
		 FROM dbalbums a
		 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
		 JOIN dbartists ar ON aa.dbartist_id = ar.id
		 LIMIT 50`,
	)

	if len(existingAlbums) == 0 {
		t.Skip("No albums found in database")
		return
	}

	t.Logf("Testing %d albums", len(existingAlbums))

	// album.Str1 = artist name, album.Str2 = album title, album.Num = dbalbum_id
	for i, album := range existingAlbums {
		if i >= 10 { // Limit to 10 for speed
			break
		}

		t.Run(fmt.Sprintf("GetDBIDsFull_%s_%s", album.Str1, album.Str2), func(t *testing.T) {
			// Create an NZB-style title
			nzbTitle := fmt.Sprintf("%s-%s-FLAC-2020-GROUP",
				strings.ReplaceAll(album.Str1, " ", "."),
				strings.ReplaceAll(album.Str2, " ", "."))

			m := &database.ParseInfo{
				File:   nzbTitle,
				Title:  nzbTitle,
				ListID: -1,
			}
			// Parse the title first (simulating what the parser would do)
			m.Artist = album.Str1 // Set from parsed data
			m.Title = album.Str2  // Album name goes in Title field

			t.Logf("Testing NZB title: %s", nzbTitle)
			t.Logf("Expected: Artist=%s, Album=%s, DbalbumID=%d", album.Str1, album.Str2, album.Num)
			parser_v2.ParseFileP(nzbTitle, false, false, cfgp, -1, m)

			// Test FindDbalbumByArtistFirst directly
			// m.FindDbalbumByArtistFirst()
			parser.GetDBIDs(m, cfgp, true, false)
			if m.DbalbumID == 0 {
				// Fallback to FindDbalbumByTitle
				// m.Title = album.Str2
				// m.FindDbalbumByTitle()
			}

			t.Logf("Result: DbalbumID=%d, Artist=%s", m.DbalbumID, m.Artist)

			if m.DbalbumID == album.Num {
				t.Logf("SUCCESS: Found correct album")
			} else if m.DbalbumID != 0 {
				t.Logf("PARTIAL: Found album ID %d (expected %d)", m.DbalbumID, album.Num)
			} else {
				t.Logf("FAILED: Could not find album in database")
			}

			if m.AlbumID == 0 {
				t.Logf("FAILED: AlbumID not set")
			}
		})
	}
}

// TestAudiobooksGetDBIDsFull tests the complete GetDBIDsFull flow for audiobooks.
// This simulates what happens during an NZB search.
// Run with: go test -v -run TestAudiobooksGetDBIDsFull
func TestAudiobooksGetDBIDsFull(t *testing.T) {
	Init()

	// Get an audiobook config if one exists
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(name string, loop *config.MediaTypeConfig) bool {
		if loop.IsType == config.MediaTypeAudiobook {
			cfgp = loop
			return true
		}
		return false
	})

	if cfgp == nil {
		t.Skip("No audiobook media config found")
		return
	}

	t.Logf("Using audiobook config: %s", cfgp.Name)

	// Get some existing audiobooks to test against
	// Uses syncops.DbstaticTwoStringOneInt: Str1=author, Str2=audiobook title, Num=id
	existingAudiobooks := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		50,
		`SELECT au.name as str1, ab.title as str2, ab.id as num
		 FROM dbaudiobooks ab
		 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
		 JOIN dbauthors au ON aa.dbauthor_id = au.id
		 WHERE ab.title IS NOT NULL AND ab.title != '' AND au.name IS NOT NULL AND au.name != ''
		 LIMIT 50`,
	)

	if len(existingAudiobooks) == 0 {
		t.Skip("No audiobooks found in database")
		return
	}

	t.Logf("Testing %d audiobooks", len(existingAudiobooks))

	tested := 0
	// audiobook.Str1 = author name, audiobook.Str2 = audiobook title, audiobook.Num = dbaudiobook_id
	for _, audiobook := range existingAudiobooks {
		if tested >= 10 { // Limit to 10 for speed
			break
		}

		// Skip entries with empty author or title
		if audiobook.Str1 == "" || audiobook.Str2 == "" {
			continue
		}
		tested++

		t.Run(
			fmt.Sprintf("GetDBIDsFull_%s_%s", audiobook.Str1, audiobook.Str2),
			func(t *testing.T) {
				// Create an NZB-style title
				nzbTitle := fmt.Sprintf("%s-%s-MP3-64k-2020-GROUP",
					strings.ReplaceAll(audiobook.Str1, " ", "."),
					strings.ReplaceAll(audiobook.Str2, " ", "."))

				m := &database.ParseInfo{
					File:   nzbTitle,
					Title:  nzbTitle,
					ListID: -1,
				}

				t.Logf("Testing NZB title: %s", nzbTitle)
				t.Logf(
					"Expected: Author=%s, Audiobook=%s, DbaudiobookID=%d",
					audiobook.Str1,
					audiobook.Str2,
					audiobook.Num,
				)

				// Debug: Check if author exists in database
				var authorID uint
				database.Scanrowsdyn(false,
					"SELECT id FROM dbauthors WHERE name = ? COLLATE NOCASE",
					&authorID, &audiobook.Str1)
				t.Logf("Debug: Author '%s' lookup -> ID=%d", audiobook.Str1, authorID)

				// Debug: Check if audiobook is linked to author
				if authorID != 0 {
					var linkedAudiobookID uint
					database.Scanrowsdyn(false,
						`SELECT ab.id FROM dbaudiobooks ab
					 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
					 WHERE aa.dbauthor_id = ? AND ab.id = ?`,
						&linkedAudiobookID, &authorID, &audiobook.Num)
					t.Logf(
						"Debug: Audiobook %d linked to author %d -> found=%v",
						audiobook.Num,
						authorID,
						linkedAudiobookID != 0,
					)
				}

				// Debug: Check actual title and slug in database (separate queries since Scanrowsdyn only scans 1 value)
				var dbTitle, dbSlug string
				database.Scanrowsdyn(false,
					"SELECT title FROM dbaudiobooks WHERE id = ?",
					&dbTitle, &audiobook.Num)
				database.Scanrowsdyn(false,
					"SELECT slug FROM dbaudiobooks WHERE id = ?",
					&dbSlug, &audiobook.Num)
				t.Logf(
					"Debug: DB audiobook %d -> title='%s', slug='%s'",
					audiobook.Num,
					dbTitle,
					dbSlug,
				)

				// Debug: Test direct SQL query with known values
				var directQueryID uint
				database.Scanrowsdyn(false,
					`SELECT ab.id FROM dbaudiobooks ab
				 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
				 WHERE aa.dbauthor_id = ?
				 AND (ab.title = ? COLLATE NOCASE OR ab.slug = ?)
				 LIMIT 1`,
					&directQueryID, &authorID, &dbTitle, &dbSlug)
				t.Logf("Debug: Direct SQL query with authorID=%d, title='%s', slug='%s' -> ID=%d",
					authorID, dbTitle, dbSlug, directQueryID)

				// Parse and search
				parser_v2.ParseFileP(nzbTitle, false, false, cfgp, -1, m)
				t.Logf(
					"Debug: After parse - Title='%s', Artist='%s', File='%s'",
					m.Title,
					m.Artist,
					m.File,
				)

				// Debug: Simulate full cleanRawNZBTitle parsing
				cleanedTitle := nzbTitle
				if !strings.Contains(cleanedTitle, " ") {
					cleanedTitle = strings.ReplaceAll(cleanedTitle, ".", " ")
				}
				t.Logf("Debug: After dot replacement: '%s'", cleanedTitle)

				// Simulate pattern removal (like cleanRawNZBTitle)
				cleanPatterns := []string{"FLAC", "MP3", "AAC", "WEB", "CD", "OST"}
				upperTitle := strings.ToUpper(cleanedTitle)
				for _, pattern := range cleanPatterns {
					for _, sep := range []string{" ", "-"} {
						if idx := strings.LastIndex(upperTitle, sep+pattern); idx != -1 {
							endIdx := idx + len(sep) + len(pattern)
							if endIdx == len(cleanedTitle) ||
								(endIdx < len(cleanedTitle) && (cleanedTitle[endIdx] == ' ' || cleanedTitle[endIdx] == '-')) {
								cleanedTitle = cleanedTitle[:idx]
								upperTitle = strings.ToUpper(cleanedTitle)
								t.Logf("Debug: Removed '%s' -> '%s'", pattern, cleanedTitle)
							}
						}
					}
				}
				t.Logf("Debug: After pattern removal: '%s'", cleanedTitle)

				// Simulate split by first "-"
				if before, after, ok := strings.Cut(cleanedTitle, "-"); ok {
					potentialAuthor := strings.TrimSpace(before)
					potentialTitle := strings.TrimSpace(after)
					t.Logf(
						"Debug: Split by '-': author='%s', title='%s'",
						potentialAuthor,
						potentialTitle,
					)
				}

				// Debug: Test author lookup like tryFindAuthorAndAudiobook does
				testAuthor := "Agatha Christie"
				sluggedAuthor := logger.StringToSlug(testAuthor)
				authorIDsFromQuery := database.GetrowsN[uint](
					false,
					10,
					"select id from dbauthors where name = ? COLLATE NOCASE or name = ? COLLATE NOCASE",
					&testAuthor,
					&sluggedAuthor,
				)
				t.Logf(
					"Debug: Author lookup for '%s' (slug='%s') -> IDs=%v",
					testAuthor,
					sluggedAuthor,
					authorIDsFromQuery,
				)

				// Test with lowercase (like splitMultiArtist produces)
				lowercaseAuthor := strings.ToLower(testAuthor)
				lowercaseSlug := logger.StringToSlug(lowercaseAuthor)
				authorIDsLowercase := database.GetrowsN[uint](
					false,
					10,
					"select id from dbauthors where name = ? COLLATE NOCASE or name = ? COLLATE NOCASE",
					&lowercaseAuthor,
					&lowercaseSlug,
				)
				t.Logf(
					"Debug: Lowercase author lookup for '%s' (slug='%s') -> IDs=%v",
					lowercaseAuthor,
					lowercaseSlug,
					authorIDsLowercase,
				)

				// Test exact audiobook lookup like tryFindAuthorAndAudiobook does
				if len(authorIDsFromQuery) > 0 {
					testTitle := "16 Uhr 50 ab Paddington"
					testSlug := logger.StringToSlug(testTitle)
					var testAudiobookID uint
					testAuthorID := authorIDsFromQuery[0]
					database.Scanrowsdyn(false,
						`SELECT ab.id FROM dbaudiobooks ab
					 JOIN dbaudiobook_authors aa ON ab.id = aa.dbaudiobook_id
					 WHERE aa.dbauthor_id = ?
					 AND (ab.title = ? COLLATE NOCASE OR ab.slug = ?)
					 LIMIT 1`,
						&testAudiobookID, &testAuthorID, &testTitle, &testSlug)
					t.Logf(
						"Debug: Exact audiobook lookup with authorID=%d, title='%s', slug='%s' -> ID=%d",
						testAuthorID,
						testTitle,
						testSlug,
						testAudiobookID,
					)
				}

				// Test direct FindDbaudiobookByAuthorFirst (bypassing GetDBIDs)
				m2 := &database.ParseInfo{
					File:   nzbTitle,
					Title:  nzbTitle,
					ListID: -1,
				}
				m2.FindDbaudiobookByAuthorFirst()
				t.Logf(
					"Debug: Direct FindDbaudiobookByAuthorFirst -> DbaudiobookID=%d, Artist='%s'",
					m2.DbaudiobookID,
					m2.Artist,
				)

				// Test GetDBIDs which calls GetDBIDsFull
				parser.GetDBIDs(m, cfgp, true, false)

				t.Logf(
					"Result: DbaudiobookID=%d, AudiobookID=%d, Author=%s",
					m.DbaudiobookID,
					m.AudiobookID,
					m.Artist,
				)

				if m.DbaudiobookID == audiobook.Num {
					t.Logf("SUCCESS: Found correct audiobook")
				} else if m.DbaudiobookID != 0 {
					t.Logf(
						"PARTIAL: Found audiobook ID %d (expected %d)",
						m.DbaudiobookID,
						audiobook.Num,
					)
				} else {
					t.Logf("INFO: Could not find audiobook through NZB parsing flow")
				}
			},
		)
	}
}

// TestDebugDatabaseLookup is a helper test for debugging specific NZB titles.
// Update the testTitle variable to debug a specific case.
// Run with: go test -v -run TestDebugDatabaseLookup
func TestDebugDatabaseLookup(t *testing.T) {
	Init()

	// UPDATE THIS TITLE to debug specific cases
	testTitle := "Alan.Silvestri-Predator.2-OST-1990-EOS"

	t.Logf("=== Debugging NZB title: %s ===", testTitle)

	m := &database.ParseInfo{
		File:  testTitle,
		Title: testTitle,
	}

	// Clean the raw title (inline version of cleanRawNZBTitle)
	cleanedTitle := testTitle
	if !strings.Contains(cleanedTitle, " ") {
		cleanedTitle = strings.ReplaceAll(cleanedTitle, "_", " ")
		cleanedTitle = strings.ReplaceAll(cleanedTitle, ".", " ")
	}
	// Remove common quality/format indicators and year at the end
	cleanedTitle = regexp.MustCompile(`[\s\-]*(FLAC|MP3|AAC|WEB|CD|OST|PROPER|REPACK)[\s\-]*`).
		ReplaceAllString(cleanedTitle, " ")
	cleanedTitle = regexp.MustCompile(`[\s\-\[\(]*(19|20)\d{2}[\]\)]*\s*$`).
		ReplaceAllString(cleanedTitle, "")
	cleanedTitle = regexp.MustCompile(`-[A-Za-z0-9]{2,10}$`).ReplaceAllString(cleanedTitle, "")
	for strings.Contains(cleanedTitle, "  ") {
		cleanedTitle = strings.ReplaceAll(cleanedTitle, "  ", " ")
	}
	cleanedTitle = strings.TrimSpace(cleanedTitle)
	t.Logf("Cleaned title: %s", cleanedTitle)

	// Try to split and find artist/album
	if before, after, ok := strings.Cut(cleanedTitle, " - "); ok {
		potentialArtist := strings.TrimSpace(before)
		potentialAlbum := strings.TrimSpace(after)
		t.Logf("Split by ' - ': Artist='%s', Album='%s'", potentialArtist, potentialAlbum)
	}

	if before, after, ok := strings.Cut(cleanedTitle, "-"); ok {
		potentialArtist := strings.TrimSpace(before)
		potentialAlbum := strings.TrimSpace(after)
		potentialArtist = strings.ReplaceAll(potentialArtist, ".", " ")
		potentialAlbum = strings.ReplaceAll(potentialAlbum, ".", " ")
		t.Logf("Split by '-': Artist='%s', Album='%s'", potentialArtist, potentialAlbum)

		// Check if artist exists
		var artistID uint
		database.Scanrowsdyn(
			false,
			"select id from dbartists where name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE",
			&artistID,
			&potentialArtist,
			&potentialArtist,
		)
		t.Logf("Artist lookup for '%s': ID=%d", potentialArtist, artistID)

		if artistID != 0 {
			// Check albums for this artist - use simple string query
			albumTitles := database.GetrowsN[string](
				false,
				10,
				`SELECT a.title FROM dbalbums a
				 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
				 WHERE aa.dbartist_id = ?
				 LIMIT 10`,
				artistID,
			)
			t.Logf("Albums for artist ID %d:", artistID)
			for _, title := range albumTitles {
				t.Logf("  - Title='%s'", title)
			}
		}
	}

	// Run the actual lookup
	m.FindDbalbumByArtistFirst()
	t.Logf("FindDbalbumByArtistFirst result: DbalbumID=%d, Artist='%s'", m.DbalbumID, m.Artist)

	// If found, show what album was matched
	if m.DbalbumID != 0 {
		var matchedTitle string
		database.Scanrowsdyn(
			false,
			"SELECT title FROM dbalbums WHERE id = ?",
			&matchedTitle,
			m.DbalbumID,
		)
		t.Logf("Matched album title: '%s'", matchedTitle)
	}

	// Fallback
	if m.DbalbumID == 0 {
		m.FindDbalbumByTitle()
		t.Logf("FindDbalbumByTitle result: DbalbumID=%d", m.DbalbumID)
	}
}

// TestDebugArtistLookup helps debug artist lookup issues with special characters.
// Run with: go test -v -run TestDebugArtistLookup
func TestDebugArtistLookup(t *testing.T) {
	Init()

	// Get artists from database
	artists := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		10,
		`SELECT name as str1, sort_name as str2, id as num FROM dbartists LIMIT 10`,
	)

	t.Logf("First 10 artists in database:")
	for _, a := range artists {
		t.Logf(
			"  ID=%d, Name='%s', SortName='%s', NameBytes=%v",
			a.Num,
			a.Str1,
			a.Str2,
			[]byte(a.Str1),
		)
	}

	// Test specific artist lookup
	testArtists := []string{
		"¡MAYDAY!",
		"¡Mayday!",
		"MAYDAY",
		"Mayday",
	}

	for _, name := range testArtists {
		var artistID uint
		slugged := logger.StringToSlug(name)
		t.Logf("Testing artist: '%s' (slug: '%s', bytes: %v)", name, slugged, []byte(name))

		database.Scanrowsdyn(
			false,
			"select id from dbartists where name = ? COLLATE NOCASE",
			&artistID,
			&name,
		)
		t.Logf("  Exact name match: ID=%d", artistID)

		artistID = 0
		database.Scanrowsdyn(
			false,
			"select id from dbartists where sort_name = ? COLLATE NOCASE",
			&artistID,
			&name,
		)
		t.Logf("  Sort name match: ID=%d", artistID)

		artistID = 0
		database.Scanrowsdyn(
			false,
			"select id from dbartists where name LIKE ? COLLATE NOCASE",
			&artistID,
			"%"+name+"%",
		)
		t.Logf("  LIKE match: ID=%d", artistID)

		artistID = 0
		database.Scanrowsdyn(
			false,
			"select id from dbartists where name LIKE ? COLLATE NOCASE OR sort_name LIKE ? COLLATE NOCASE",
			&artistID,
			"%MAYDAY%",
			"%MAYDAY%",
		)
		t.Logf("  LIKE MAYDAY match: ID=%d", artistID)
	}

	// Check albums for artist ID 1
	t.Logf("\nAlbums for artist ID=1:")
	albums := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		20,
		`SELECT a.title as str1, a.slug as str2, a.id as num
		 FROM dbalbums a
		 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
		 WHERE aa.dbartist_id = 1
		 LIMIT 20`,
	)
	for _, a := range albums {
		t.Logf("  ID=%d, Title='%s', Slug='%s'", a.Num, a.Str1, a.Str2)
	}

	// Now test the actual lookup
	testTitle := "¡MAYDAY! - Believers"
	t.Logf("\n=== Testing direct lookup for: '%s' ===", testTitle)

	// Test artist lookup
	var artistID uint
	artistName := "¡MAYDAY!"
	sluggedArtist := logger.StringToSlug(artistName)
	t.Logf("Artist name: '%s', slugged: '%s'", artistName, sluggedArtist)

	database.Scanrowsdyn(
		false,
		"select id from dbartists where name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE or name = ? COLLATE NOCASE or sort_name = ? COLLATE NOCASE",
		&artistID,
		&artistName,
		&artistName,
		&sluggedArtist,
		&sluggedArtist,
	)
	t.Logf("Artist lookup result: ID=%d", artistID)

	if artistID != 0 {
		// Test album lookup
		albumTitle := "Believers"
		sluggedAlbum := logger.StringToSlug(albumTitle)
		t.Logf("Album title: '%s', slugged: '%s'", albumTitle, sluggedAlbum)

		var albumID uint
		database.Scanrowsdyn(false,
			`SELECT a.id FROM dbalbums a
			 JOIN dbalbum_artists aa ON a.id = aa.dbalbum_id
			 WHERE aa.dbartist_id = ?
			 AND (a.title = ? COLLATE NOCASE OR a.slug = ?)
			 LIMIT 1`,
			&albumID, &artistID, &albumTitle, &sluggedAlbum)
		t.Logf("Album lookup result: ID=%d", albumID)
	}

	// Test FindDbalbumByArtistFirst directly
	t.Logf("\n=== Testing FindDbalbumByArtistFirst ===")
	m := &database.ParseInfo{
		File:  testTitle,
		Title: testTitle,
	}
	m.FindDbalbumByArtistFirst()
	t.Logf("FindDbalbumByArtistFirst result: DbalbumID=%d, Artist='%s'", m.DbalbumID, m.Artist)

	// Check if album is in albums table (user's lists)
	if m.DbalbumID != 0 {
		albumEntries := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
			false,
			10,
			`SELECT listname as str1, '' as str2, id as num FROM albums WHERE dbalbum_id = ?`,
			m.DbalbumID,
		)
		t.Logf("All album entries for DbalbumID=%d:", m.DbalbumID)
		for _, e := range albumEntries {
			t.Logf("  AlbumID=%d, ListName='%s'", e.Num, e.Str1)
		}
		if len(albumEntries) == 0 {
			t.Logf("  (no entries found - album not in any user list)")
		}
	}
}

// TestMusicSearcherFlow tests the complete searcher flow for music albums.
// This simulates exactly what happens when processing NZB entries during search.
// Run with: go test -v -run TestMusicSearcherFlow
func TestMusicSearcherFlow(t *testing.T) {
	Init()

	// Get a music config if one exists
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(name string, loop *config.MediaTypeConfig) bool {
		if loop.IsType == config.MediaTypeMusic {
			cfgp = loop
			return true
		}
		return false
	})

	if cfgp == nil {
		t.Skip("No music media config found")
		return
	}

	t.Logf("Using music config: %s", cfgp.Name)
	t.Logf("Lists in config: %v", cfgp.Lists)
	t.Logf("UseMediaCache: %v", config.GetSettingsGeneral().UseMediaCache)

	// Get albums that are actually in the user's lists (from albums table, not just dbalbums)
	// This is what the searcher actually checks against
	existingAlbums := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false,
		20,
		`SELECT ar.name as str1, dba.title as str2, dba.id as num
		 FROM albums a
		 JOIN dbalbums dba ON a.dbalbum_id = dba.id
		 JOIN dbalbum_artists aa ON dba.id = aa.dbalbum_id
		 JOIN dbartists ar ON aa.dbartist_id = ar.id
		 LIMIT 20`,
	)

	if len(existingAlbums) == 0 {
		t.Skip("No albums found in database")
		return
	}

	t.Logf("Testing %d albums with full searcher flow", len(existingAlbums))

	// album.Str1 = artist name, album.Str2 = album title, album.Num = dbalbum_id
	for i, album := range existingAlbums {
		if i >= 10 {
			break
		}

		t.Run(fmt.Sprintf("SearcherFlow_%d_%s", i, album.Str2), func(t *testing.T) {
			// Generate various NZB-style title formats to test
			nzbFormats := []string{
				// Standard format: "Artist - Album"
				album.Str1 + " - " + album.Str2,
				// Scene format with dots: "Artist.Name-Album.Title-FLAC-2020-GROUP"
				strings.ReplaceAll(
					album.Str1,
					" ",
					".",
				) + "-" + strings.ReplaceAll(
					album.Str2,
					" ",
					".",
				) + "-FLAC-2020-GROUP",
				// Scene format without tags
				strings.ReplaceAll(
					album.Str1,
					" ",
					".",
				) + "-" + strings.ReplaceAll(
					album.Str2,
					" ",
					".",
				),
			}

			for _, nzbTitle := range nzbFormats {
				// Create a fresh ParseInfo
				m := database.PLParseInfo.Get()
				defer m.Close()

				// Step 1: Parse the file (exactly like searcher does)
				parser_v2.ParseFileP(
					nzbTitle,
					false, // usepath
					false, // usefolder
					cfgp,
					-1, // listid
					m,
				)

				t.Logf(
					"After ParseFileP - File: '%s', Title: '%s', Artist: '%s'",
					m.File,
					m.Title,
					m.Artist,
				)

				// Step 2: Get DB IDs (exactly like searcher does)
				err := music.Handler.GetDBIDsFull(m, cfgp, true, false)

				if err != nil {
					t.Logf("FAILED: GetDBIDsFull error for '%s': %v", nzbTitle, err)
					// Debug: check if DbalbumID was found
					t.Logf(
						"  Debug: DbalbumID=%d, AlbumID=%d, ListID=%d",
						m.DbalbumID,
						m.AlbumID,
						m.ListID,
					)
					if m.DbalbumID != 0 {
						// Check what list entries exist for this DbalbumID
						albumEntries := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
							false,
							10,
							"SELECT listname as str1, '' as str2, id as num FROM albums WHERE dbalbum_id = ?",
							m.DbalbumID,
						)
						t.Logf(
							"  Debug: Album entries in 'albums' table for DbalbumID=%d:",
							m.DbalbumID,
						)
						for _, e := range albumEntries {
							t.Logf("    AlbumID=%d, ListName='%s'", e.Num, e.Str1)
						}
						if len(albumEntries) == 0 {
							t.Logf("    (no entries found)")
						}
						t.Logf("  Debug: Config list[0].Name='%s'", cfgp.Lists[0].Name)
					}
				} else if m.DbalbumID == album.Num {
					t.Logf("SUCCESS: Found correct album (ID=%d) for '%s'", m.DbalbumID, nzbTitle)
				} else if m.DbalbumID != 0 {
					t.Logf(
						"PARTIAL: Found album ID %d (expected %d) for '%s'",
						m.DbalbumID,
						album.Num,
						nzbTitle,
					)
				} else {
					t.Logf("FAILED: No album found for '%s'", nzbTitle)
				}
			}
		})
	}
}

// TestVARSSArtistsPath tests the rssartists search path for Various Artists compilations.
// When the rssartists job searches Hydra for "Various Artists", it gets back NZB titles
// like "VA-Bravo Hits Vol.58-2CD-2007-MST". This test traces every step of
// ParseFileP → GetDBIDsFull → ValidateRSSIDs to show exactly where/why matching fails.
// Run with: go test -v -run TestVARSSArtistsPath
func TestVARSSArtistsPath(t *testing.T) {
	Init()

	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, loop *config.MediaTypeConfig) bool {
		if loop.IsType == config.MediaTypeMusic {
			cfgp = loop
			return true
		}
		return false
	})
	if cfgp == nil {
		t.Skip("No music media config found")
		return
	}
	t.Logf("Using music config: %s  lists: %d", cfgp.Name, len(cfgp.Lists))

	// NZB titles that Hydra would return when searching for "Various Artists" or "VA"
	nzbTitles := []string{
		"VA-Bravo Hits Vol.58-2CD-2007-MST",
		"VA-Bravo.Hits.Vol.58-2CD-2007-MST",
		"VA-Bravo Hits 58-2007-MST",
		"Various.Artists-Bravo.Hits.58-FLAC-2007-GROUP",
		"VA - Bravo Hits 58 (2007) FLAC",
		"Bravo Hits 58-2007-MST",
	}

	for _, nzbTitle := range nzbTitles {
		t.Run(nzbTitle, func(t *testing.T) {
			m := database.PLParseInfo.Get()
			defer m.Close()

			// Step 1: ParseFileP — same call as searcher's getmediadatarss
			parser_v2.ParseFileP(nzbTitle, false, false, cfgp, -1, m)
			t.Logf("ParseFileP:   File=%q  Title=%q  Artist=%q  Year=%d  MBZ=%q",
				m.File, m.Title, m.Artist, m.Year, m.MusicBrainzID)

			// Step 2: GetDBIDsFull — tries MBZ → UPC → FindDbalbumByArtistFirstFromWantedList → FindDbalbumByArtistFirst → FindDbalbumByTitle → findInLists
			err := music.Handler.GetDBIDsFull(m, cfgp, true, false)
			t.Logf("GetDBIDsFull: DbalbumID=%d  AlbumID=%d  ListID=%d  err=%v",
				m.DbalbumID, m.AlbumID, m.ListID, err)

			// Step 3: ValidateRSSIDs logic (checks DbalbumID!=0 AND AlbumID!=0)
			switch {
			case m.DbalbumID == 0:
				t.Logf("RESULT: REJECTED — DbalbumID=0 ('unwanted DBAlbum')")
			case m.AlbumID == 0:
				t.Logf(
					"RESULT: REJECTED — AlbumID=0 ('unwanted Album') for DbalbumID=%d",
					m.DbalbumID,
				)
				// Show what list entries exist for this DbalbumID
				entries := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
					false,
					10,
					"SELECT listname as str1, '' as str2, id as num FROM albums WHERE dbalbum_id = ?",
					m.DbalbumID,
				)
				if len(entries) == 0 {
					t.Logf(
						"  => no rows in albums table for DbalbumID=%d (not in any user list)",
						m.DbalbumID,
					)
				}
				for _, e := range entries {
					t.Logf("  => albums.id=%d  listname=%q", e.Num, e.Str1)
				}
			default:
				t.Logf(
					"RESULT: PASSED ValidateRSSIDs — DbalbumID=%d  AlbumID=%d",
					m.DbalbumID,
					m.AlbumID,
				)
			}
		})
	}
}

// TestBravoHits58MissingSearchPath tests the SearchGenMissing path for "Bravo Hits 58".
// SearchGenMissing picks missing albums and calls MediaSearch per album.
// MediaSearch builds a search query (FillSearchVar), queries Hydra, then for each
// returned NZB calls ParseFileP+GetDBIDsFull and checks CheckMediaMatch:
//
//	source.NzbalbumID == entry.Info.AlbumID
//
// This test traces that exact flow with the NZB "VA-Bravo Hits Vol.58-2CD-2007-MST".
// Run with: go test -v -run TestBravoHits58MissingSearchPath
func TestBravoHits58MissingSearchPath(t *testing.T) {
	Init()

	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, loop *config.MediaTypeConfig) bool {
		if loop.IsType == config.MediaTypeMusic {
			cfgp = loop
			return true
		}
		return false
	})
	if cfgp == nil {
		t.Skip("No music media config found")
		return
	}
	t.Logf("Using music config: %s", cfgp.Name)

	// Find "Bravo Hits" albums in the user's lists.
	// Str1=artist, Str2=title, Num=albums.id (the NzbalbumID used in CheckMediaMatch)
	bravoAlbums := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
		false, 5,
		`SELECT ar.name as str1, dba.title as str2, a.id as num
		 FROM albums a
		 JOIN dbalbums dba ON a.dbalbum_id = dba.id
		 JOIN dbalbum_artists aa ON dba.id = aa.dbalbum_id
		 JOIN dbartists ar ON aa.dbartist_id = ar.id
		 WHERE dba.title LIKE 'Bravo Hits%'
		 ORDER BY dba.title
		 LIMIT 5`,
	)
	if len(bravoAlbums) == 0 {
		t.Skip("No 'Bravo Hits' albums found in user's lists")
		return
	}

	for _, src := range bravoAlbums {
		sourceAlbumID := uint(src.Num)
		wantedTitle := src.Str2
		artistName := src.Str1

		// Replicate FillSearchVar: for VA, SearchFor = title only (no artist prefix)
		var searchFor string
		if artistName == "Various Artists" || artistName == "VA" || artistName == "Various" {
			searchFor = wantedTitle
		} else {
			searchFor = wantedTitle + " " + artistName
		}

		t.Logf("\n=== Source album: id=%d  title=%q  artist=%q  SearchFor=%q",
			sourceAlbumID, wantedTitle, artistName, searchFor)

		// NZB titles that Hydra could return for this search query
		nzbTitles := []string{
			"VA-Bravo Hits Vol.58-2CD-2007-MST",
			"VA-Bravo.Hits.Vol.58-2CD-2007-MST",
			"VA-Bravo Hits 58-2007-MST",
			"Bravo Hits 58 - 2007 - FLAC - GROUP",
		}

		for _, nzbTitle := range nzbTitles {
			t.Run(fmt.Sprintf("%s|%s", wantedTitle, nzbTitle), func(t *testing.T) {
				m := database.PLParseInfo.Get()
				defer m.Close()

				// Step 1: ParseFileP on the NZB title
				parser_v2.ParseFileP(nzbTitle, false, false, cfgp, -1, m)
				t.Logf("ParseFileP:   File=%q  Title=%q  Artist=%q  Year=%d",
					m.File, m.Title, m.Artist, m.Year)

				// Step 2: GetDBIDsFull — same as searcher's getmediadata
				err := music.Handler.GetDBIDsFull(m, cfgp, true, false)
				t.Logf("GetDBIDsFull: DbalbumID=%d  AlbumID=%d  ListID=%d  err=%v",
					m.DbalbumID, m.AlbumID, m.ListID, err)

				// Step 3: CheckMediaMatch — source.NzbalbumID == entry.Info.AlbumID
				mediaMatch := sourceAlbumID == m.AlbumID
				t.Logf("CheckMediaMatch: source.NzbalbumID=%d  entry.AlbumID=%d  match=%v",
					sourceAlbumID, m.AlbumID, mediaMatch)

				if !mediaMatch {
					t.Logf("RESULT: REJECTED by CheckMediaMatch")
				} else {
					// Step 4: ChecknzbtitleB — title validation (checktitle step)
					titleOK := database.ChecknzbtitleB(
						wantedTitle,
						"",
						nzbTitle,
						true,
						uint16(m.Year),
					)
					t.Logf("ChecknzbtitleB(wanted=%q, nzb=%q): %v", wantedTitle, nzbTitle, titleOK)
					if titleOK {
						t.Logf("RESULT: PASSED CheckMediaMatch and ChecknzbtitleB")
					} else {
						t.Logf("RESULT: REJECTED by ChecknzbtitleB despite CheckMediaMatch passing")
					}
				}
			})
		}
	}
}

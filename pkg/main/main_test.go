package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
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
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/slidingwindow"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"

	"github.com/mozillazg/go-unidecode"
	"github.com/mozillazg/go-unidecode/table"

	//"github.com/rainycape/unidecode"

	"golang.org/x/net/html/charset"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

//func TestMain(m *testing.M) {
//goleak.VerifyTestMain(m)
//}

//Test with: go.exe test -timeout 30s -v -run ^TestDir$ github.com/Kellerman81/go_media_downloader

func Benchmark1Concat(b *testing.B) { // 132 ns/op
	//ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for i := 0; i < b.N; i++ {
		s := "sadsadsa" + "dsadsakdas;k" + "8930984" + "8930984" + "8930984" + "8930984" + strconv.Itoa(23)
		_ = s
	}
}

func Benchmark1Printf(b *testing.B) { // 56.7 ns/op
	//ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
	for i := 0; i < b.N; i++ {
		s := fmt.Sprintf("%s%s%s%s%s%s%d", "sadsadsa", "dsadsakdas;k", "8930984", "8930984", "8930984", "8930984", 23)
		_ = s
	}
}

func Benchmark1Builder(b *testing.B) { // 58.5
	//ss := []string{"sadsadsa", "dsadsakdas;k", "8930984"}
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
func Init() {
	os.Mkdir("./temp", 0777)
	config.LoadCfgDB()

	database.InitCache()
	logger.InitLogger(logger.Config{
		LogLevel:     "Warning",
		LogFileSize:  config.SettingsGeneral.LogFileSize,
		LogFileCount: config.SettingsGeneral.LogFileCount,
		LogCompress:  config.SettingsGeneral.LogCompress,
	})
	if config.SettingsGeneral.WebPort == "" {
		//fmt.Println("Checked for general - config is missing", cfg_general)
		//os.Exit(0)
		config.ClearCfg()
		config.WriteCfg()
		config.LoadCfgDB()
	}
	err := database.InitDB(config.SettingsGeneral.DBLogLevel)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)

	}
	apiexternal.NewOmdbClient(config.SettingsGeneral.OmdbAPIKey, config.SettingsGeneral.OmdbLimiterSeconds, config.SettingsGeneral.OmdbLimiterCalls, config.SettingsGeneral.OmdbDisableTLSVerify, config.SettingsGeneral.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.SettingsGeneral.TheMovieDBApiKey, config.SettingsGeneral.TmdbLimiterSeconds, config.SettingsGeneral.TmdbLimiterCalls, config.SettingsGeneral.TheMovieDBDisableTLSVerify, config.SettingsGeneral.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.SettingsGeneral.TvdbLimiterSeconds, config.SettingsGeneral.TvdbLimiterCalls, config.SettingsGeneral.TvdbDisableTLSVerify, config.SettingsGeneral.TvdbTimeoutSeconds)
	apiexternal.NewTraktClient(config.SettingsGeneral.TraktClientID, config.SettingsGeneral.TraktClientSecret, config.SettingsGeneral.TraktLimiterSeconds, config.SettingsGeneral.TraktLimiterCalls, config.SettingsGeneral.TraktDisableTLSVerify, config.SettingsGeneral.TraktTimeoutSeconds)
	worker.InitWorkerPools(config.SettingsGeneral.WorkerSearch, config.SettingsGeneral.WorkerFiles, config.SettingsGeneral.WorkerMetadata)

	database.InitImdbdb()

	logger.LogDynamicany("info", "Check Database for Upgrades")
	//database.UpgradeDB()
	database.SetVars()

	parser.GenerateAllQualityPriorities()

	parser.LoadDBPatterns()
	parser.GenerateCutoffPriorities()
	database.Refreshhistorycache(true)
	database.RefreshMediaCache(true)
	database.RefreshMediaCacheTitles(true)
	database.Refreshunmatchedcached(true)
	database.Refreshfilescached(true)

	database.Refreshhistorycache(false)
	database.RefreshMediaCache(false)
	database.RefreshMediaCacheTitles(false)
	database.Refreshunmatchedcached(false)
	database.Refreshfilescached(false)
}

func TestStructure(t *testing.T) {
	Init()
	//configTemplate := "movie_EN"
	//structure.StructureSingleFolder("Y:\\completed\\MoviesDE\\Rot.2022.German.AC3.BDRiP.x264-SAVASTANOS", false, false, false, logger.StrMovie, "path_en movies import", "path_en movies", configTemplate)
}

func TestMain(t *testing.T) {
	tests := []struct {
		name string
	}{
		{name: "Test1"},
		{name: "Test2"},
		{name: "Test3"},
		{name: "Test4"},
		{name: "Test5"},
	}
	Init()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// dbseries, _ := database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

			// for idxserie := range dbseries {
			// 	importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
			// }
		})
	}
}

func TestGetDBIDAdded(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {

		//GetNewFilesTest("serie_EN", logger.StrSeries)
	})
}
func TestGetDBIDParse(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		cfgp := config.SettingsMedia["serie_EN"]
		q := config.SettingsQuality["SD"]
		//h, _ := json.Marshal(config.SettingsMedia)
		t.Log(cfgp.Useseries)
		//defer parse.Close()
		parse := parser.NewFileParser("Alias - S01E01 - Truth Be Told - 480P DVDRIP XVID - proper", cfgp, false, -1)
		parser.GetPriorityMapQual(parse, cfgp, q, true, true)
		err := parser.GetDBIDs(parse, cfgp, true)

		t.Log(err)
		j, _ := json.Marshal(parse)
		t.Log(string(j))
		t.Log(parse.ListID)
		t.Log(parse.DbserieID)
		t.Log(parse.SerieID)
		//GetNewFilesTest("serie_EN", logger.StrSeries)
	})
}

func TestToSlug(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		t.Log(logger.StringToSlug("Hanäl-&$§áfedfe_feoke"))
	})
}
func TestGetSerieAdded(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		//im.SetTitle("Eureka")

		//GetNewFilesTest("serie_EN", logger.StrSeries)
	})
}

func TestParseXML(t *testing.T) {
	url := "https://api.nzbgeek.info/api?apikey=&tvdbid=82701&season=1&ep=8&cat=5030&dl=1&t=tvsearch&extended=1"

	req, _ := http.NewRequest("GET", url, nil)
	cl := &http.Client{Timeout: time.Duration(10) * time.Second,
		Transport: &http.Transport{
			TLSHandshakeTimeout:   time.Duration(10) * time.Second,
			ResponseHeaderTimeout: time.Duration(10) * time.Second,
			TLSClientConfig:       &tls.Config{InsecureSkipVerify: true},
			MaxIdleConns:          20,
			MaxConnsPerHost:       10,
			DisableCompression:    false,
			DisableKeepAlives:     true,
			IdleConnTimeout:       120 * time.Second}}
	resp, _ := cl.Do(req)
	d := xml.NewDecoder(resp.Body)
	d.CharsetReader = charset.NewReaderLabel
	d.Strict = false
	d.DecodeElement(d, &xml.StartElement{})
}
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
func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}
func TestGetCsv(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		//var serie database.Dbmovie
		v := "https://datasets.imdbws.com/title.akas.tsv.gz"

		resp, err := http.Get(v)
		if err != nil {
			return
		}
		defer resp.Body.Close()

		gzreader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return
		}
		defer gzreader.Close()

		parseraka := csv.NewReader(gzreader)
		parseraka.Comma = '\t'
		parseraka.ReuseRecord = true
		parseraka.LazyQuotes = true
		_, _ = parseraka.Read() // skip header

		var record []string
		var csverr error
		for {
			record, csverr = parseraka.Read()
			if errors.Is(csverr, io.EOF) {
				break
			}
			if csverr != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing aka.. %v", csverr))
				continue
			}
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// For info on each, see: https://golang.org/pkg/runtime/#MemStats
		t.Log(fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc)))
		t.Log(fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc)))
		t.Log(fmt.Printf("\tSys = %v MiB", bToMb(m.Sys)))
		t.Log(fmt.Printf("\tNumGC = %v\n", m.NumGC))
		_ = record
		PrintMemUsage()
	})
}

func buildPrioStr(r uint, q uint, c uint, a uint) string {
	return strconv.Itoa(int(r)) + "_" + strconv.Itoa(int(q)) + "_" + strconv.Itoa(int(c)) + "_" + strconv.Itoa(int(a))
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
		a := apiexternal.Nzbwithprio{WantedTitle: "ffff", WantedAlternates: []database.DbstaticTwoStringOneInt{{Str1: "ffff", Str2: "ffff"}}, Quality: "test", Listname: "test"}
		a.Close()
	}
	PrintMemUsage()
}
func BenchmarkClose2(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		a := apiexternal.Nzbwithprio{WantedTitle: "ffff", WantedAlternates: []database.DbstaticTwoStringOneInt{{Str1: "ffff", Str2: "ffff"}}, Quality: "test", Listname: "test"}
		a.Close()
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
	var str = "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = Path(str, false)
	}
}
func BenchmarkPath2(b *testing.B) {
	var str = "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = Path2(str, false)
	}
}

// JoinStrings concatenates any number of strings together.
// It is optimized to avoid unnecessary allocations when there are few elements.
func JoinStringsTest(elems ...string) string {
	if len(elems) == 0 {
		return ""
	}
	if len(elems) == 1 {
		return elems[0]
	}
	if len(elems) == 2 {
		return elems[0] + elems[1]
	}
	if len(elems) == 3 {
		return elems[0] + elems[1] + elems[2]
	}

	b := logger.PlBuffer.Get()
	//b.Grow(Getstringarrlength(elems))
	for idx := range elems {
		if elems[idx] != "" {
			b.WriteString(elems[idx])
		}
	}
	defer logger.PlBuffer.Put(b)
	return b.String()
}

func BenchmarkJoinString1(b *testing.B) {
	var str = "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = str + str
	}
}
func BenchmarkJoinString2(b *testing.B) {
	var str = "/downloads/completed/MoviesDE/Die.nackte.?<>?::Kanone.2.5.GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510)/Die nackte Kanone 2.5 GERMAN.1991.DVDRiP.iNTERNAL.XViD-SKiLLED (tt0102510).avi"
	b.ReportAllocs()
	for j := 0; j < b.N; j++ {
		_ = JoinStringsTest(str, str)
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
		//c = append(c, str...)

		//logger.Grow(&c, len(str))
		c = append(c, str...)
		//logger.RemoveFromStringArray(&str, "500")
		_ = c
	}
}
func TestGetUrl(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		//url := fmt.Sprintf("S%sE%s", "01", "01")
		for i := 0; i < 1000000; i++ {
			url := buildPrioStr(10, 10, 10, 10)
			//url := fmt.Sprintf("aa%d", 5)
			_ = url
		}
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		// For info on each, see: https://golang.org/pkg/runtime/#MemStats
		t.Log(fmt.Printf("Alloc = %v MiB", m.Alloc))
		t.Log(fmt.Printf("\tTotalAlloc = %v MiB", m.TotalAlloc))
		t.Log(fmt.Printf("\tSys = %v MiB", m.Sys))
		t.Log(fmt.Printf("\tNumGC = %v\n", m.NumGC))
	})
}

func TestGetTmdb(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		var movie database.Dbmovie
		movie.ImdbID = "tt5971474"
		movie.MoviedbID = 447277
		metadata.Getmoviemetadata(&movie, true)
		//tmdbfind, _ := apiexternal.TmdbAPI.FindImdb("tt7214954")
		//t.Log(tmdbfind)
		//tmdbtitle, _ := apiexternal.GetTmdbMovieTitles(585511)
		//t.Log(tmdbtitle)
		movie.GetImdbTitle(&movie.ImdbID, true)
		t.Log(movie.Runtime)
		tmdbdetails, _ := apiexternal.GetTmdbMovie(447277)
		t.Log(tmdbdetails.Runtime)
		tt := "tt5971474"
		traktdetails, _ := apiexternal.GetTraktMovie(tt)
		t.Log(traktdetails.Runtime)
		//var dbserie database.Dbserie
		//dbserie.ThetvdbID = 85352
		//dbserie.GetMetadata("", true, true, true, true)
		//t.Log(dbserie)
		//t.Log(dbserie.ImdbID)
		//t.Log(dbserie.ID)
		//t.Log(dbserie.Seriename)
		//GetNewFilesTest("serie_EN", logger.StrSeries)
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
		//GetNewFilesTest("serie_EN", logger.StrSeries)
	})
}
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

func TestHtml(t *testing.T) {
	t.Run("test", func(t *testing.T) {
		tst, err := os.Open("tst.nzb")
		if err != nil {
			t.Log("not found")
			return
		}
		defer tst.Close()
		scanner := bufio.NewScanner(tst)
		if scanner.Scan() {
			str := strings.TrimSpace(scanner.Text())
			if len(str) >= 5 {
				if strings.EqualFold(str[:5], "<html") {
					t.Log("found html1")
					return
				}
			}

			if scanner.Scan() {
				str = strings.TrimSpace(scanner.Text())
				if len(str) >= 5 {
					if strings.EqualFold(str[:5], "<html") {
						t.Log("found html2")
						return
					}
				}
			}
		}
		t.Log("all ok")
	})
}

func TestMime(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		file := "Y:\\completed\\MoviesDE\\Paws.of.Fury.The.Legend.of.Hank.2022.German.AAC.WEBRip.x264-ZeroTwo\\Paws.of.Fury.The.Legend.of.Hank.2022.German.AAC.WEBRip.x264-ZeroTwo.mkv"
		filed, err := os.Open(file)
		if err != nil {
			return
		}
		defer filed.Close()

		image, _, err := image.DecodeConfig(filed)
		if err != nil {
			fmt.Println(err)
		}
		jsond, _ := json.Marshal(image)
		t.Log(string(jsond))
	})
}

func TestSQL(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		t.Log(database.GetdatarowN[int](false, "select id from dbmovies ORDER BY [id] ASC limit ?", 1))
		t.Log(database.GetdatarowN[int](false, "select count() from dbmovies"))
		var i int
		database.ScanrowsNdyn(false, "select id from dbmovies ORDER BY [id] ASC limit ?", &i, 1)
		t.Log(i)

		//a := database.GetCachedTypeObjArr[database.DbstaticTwoStringOneInt](logger.CacheDBSeries)
		a := database.GetrowsN[database.DbstaticThreeStringTwoInt](false, database.GetdatarowN[uint](false, "select count() from dbmovies")+100,
			"select title, slug, imdb_id, year, id from dbmovies")
		t.Log(a)
		//t.Log(database.GetrowsN[database.DbstaticThreeStringTwoInt](false, database.GetdatarowN[int](false, "select count() from dbmovies")+100, "select title, slug, imdb_id, year, id from dbmovies"))
	})
}

func TestDir(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		//bla, _ := scanner.GetFilesDir("W:\\", "de movies", false)
		//t.Log(bla)
		//t.Log(config.Cfg.Paths)
	})
}

func joinCats(cats []int) string {
	var b bytes.Buffer
	defer b.Reset()
	//b.Grow(30)
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

func ToAscii(str string) (string, error) {
	result, _, err := transform.String(transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn))), str)
	if err != nil {
		return "", err
	}
	return result, nil
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
		//clear(unwantedRunes)
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
func Benchmark2Ascii(b *testing.B) {
	str := `"Franä & Freddie's Diner	☺"`
	var str2 string
	for i := 0; i < b.N; i++ {
		str2, _ = ToAscii(str)
	}
	b.Log(str2)
}
func BenchmarkQueryString1(b *testing.B) {
	movieid := "Test123"
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		buf.Write([]byte("https://api.trakt.tv/movies/"))
		buf.Write([]byte(movieid))
		buf.Write([]byte("/aliases"))
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
		//_ = fmt.Sprintf("%s&q=%s&cat=%s&dl=1&t=%s%s%s", urlv, query, categories, searchtype, json, additional_query_params)
		//_ = urlv + "&q=" + query + "&cat=" + categories + "&dl=1&t=" + searchtype + json + additional_query_params
		//fmt.Println(urlv + "&q=" + query + "&cat=" + categories + "&dl=1&t=" + searchtype + json + additional_query_params)
		//continue
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
		//database.CountRowsTest1("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest1(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
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
		//database.CountRowsTest1("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest1(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkQuery2(b *testing.B) {
	//Init()
	//b.ResetTimer()
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
		//database.CountRowsTest2("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest2(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
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
	//return strings.TrimRight(elem1, "\\/") + sep + strings.TrimLeft(elem2, "\\/")
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
	//Init()
	//b.ResetTimer()
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

		//database.CountRows("dbseries", &database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest3(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func TestQueryXML1(b *testing.T) {
	Init()
	limiter := slidingwindow.NewLimiter(time.Duration(1)*time.Second, 10000000)
	c := apiexternal.NewClient(
		"test",
		true,
		true,
		&limiter,
		false,
		nil, 10)
	//results := make([]apiexternal.Nzbwithprio, 0, 100)
	var results apiexternal.NzbSlice
	urlv := "https://api.nzbgeek.info/rss?t=2000&limit=100&dl=1&r="
	c.DoXMLItem(config.SettingsIndexer["nzbgeek"], config.SettingsQuality["sd"], "", "nzbgeek.info", urlv, &results)
	//b.Log(results)
	//bla, _ := json.Marshal(results)
	//b.Log(string(bla))
}

func TestQueryMovie(b *testing.T) {
	Init()
	var id uint = 14027
	results := searcher.NewSearcher(config.SettingsMedia["movie_EN"], nil, "", 0)
	err := results.MediaSearch(config.SettingsMedia["movie_EN"], &id, false, false, false)
	//b.Log(results)
	bla, _ := json.Marshal(results)
	b.Log(string(bla))
	b.Log(err)
}

func TestEpiQuery(b *testing.T) {
	Init()
	var outid uint
	dbserie := 300
	season := 1
	episode := "3"
	database.ScanrowsNdyn(false, "select id from dbserie_episodes where dbserie_id = ? and season = ? and episode = ?", &outid, dbserie, season, episode)
	b.Log(outid)
}

func TestReconnectQuery(b *testing.T) {
	Init()
	var outid uint
	database.ScanrowsNdyn(true, "select count() from imdb_akas", &outid)
	b.Log(outid)
	database.CloseImdb()
	b.Log(os.Rename("./databases/imdb.db", "./databases/imdb.db.bak"))
	//database.ScanrowsNdyn(true, "select count() from imdb_akas", &outid)
	//b.Log(outid)
	b.Log(os.Rename("./databases/imdb.db.bak", "./databases/imdb.db"))
	b.Log(database.InitImdbdb())
	database.ScanrowsNdyn(true, "select count() from imdb_akas", &outid)
	b.Log(outid)
}

func TestTraktQuery(b *testing.T) {
	Init()
	data, err := apiexternal.Testaddtraktdbepisodes()
	b.Log(data)
	b.Log(err)
}

func TestQueryXML1new(b *testing.T) {
	Init()
	// c := apiexternal.NewClient(
	// 	true,
	// 	true,
	// 	slidingwindow.NewLimiter(time.Duration(1)*time.Second, 10000000),
	// 	false,
	// 	nil, 10)
	results := make([]apiexternal.Nzbwithprio, 0, 100)
	//c.DoXMLItem(config.SettingsIndexer["nzbgeek"], config.SettingsQuality["sd"], "", "nzbgeek.info", "https://api.nzbgeek.info/api?t=search&q=dogma&limit=100&extended=1&apikey=", &results)
	//b.Log(results)
	bla, _ := json.Marshal(results)
	b.Log(string(bla))
}

func BenchmarkAllowRange(b *testing.B) {
	Init()
	arr := database.GetrowsN[database.DbstaticThreeStringTwoInt](false, database.GetdatarowN[uint](false, "select count() from dbmovies")+100,
		"select title, slug, imdb_id, year, id from dbmovies")
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
	arr := database.GetrowsN[database.DbstaticThreeStringTwoInt](false, database.GetdatarowN[uint](false, "select count() from dbmovies")+100,
		"select title, slug, imdb_id, year, id from dbmovies")
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for _, r := range arr {
			_ = r
		}
	}
	PrintMemUsage()
}

func BenchmarkLog1(b *testing.B) {
	Init()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.GetLogger().Debug().Str("test", "test").Msg("test")
	}
	PrintMemUsage()
}
func BenchmarkLog2(b *testing.B) {
	Init()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.LogDynamicany("debug", "test", "test", "test")
	}
	PrintMemUsage()
}

func BenchmarkLog3(b *testing.B) {
	Init()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		logger.LogDynamicany("debug", "test", "test", "test")
	}
	PrintMemUsage()
}
func BenchmarkQueryXML1Item(b *testing.B) {
	Init()
	limiter := slidingwindow.NewLimiter(time.Duration(1)*time.Second, 10000000)
	c := apiexternal.NewClient(
		"test",
		true,
		true,
		&limiter,
		false,
		nil, 10)
	var results apiexternal.NzbSlice
	//results := make([]apiexternal.Nzbwithprio, 0, 100)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		results.Arr = results.Arr[:0]
		urlv := "https://api.nzbgeek.info/rss?t=2000&limit=100&dl=1&r="
		c.DoXMLItem(config.SettingsIndexer["nzbgeek"], config.SettingsQuality["sd"], "", "nzbgeek.info", urlv, &results)
		//c.DoXMLItem(config.SettingsIndexer["nzbgeek"], config.SettingsQuality["sd"], "", "nzbgeek.info", "https://api.nzbgeek.info/api?t=search&q=chinese&limit=100&extended=1&apikey=", &results)
	}
	_ = c
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
		//str := "SD"

		searchresults := searcher.NewSearcher(nil, nil, logger.StrRss, 0)
		searchresults.SearchRSS(nil, nil, false, false)

		searchresults.Close()
		//clie.SearchWithIMDB(categories, "tt0120655", additional_query_params, "", 0, false)
		//clie.LoadRSSFeed(categories, 100, additional_query_params, "", "", "", 0, false)
		//apiexternal.QueryNewznabRSSLast(apiexternal.NzbIndexer{URL: apiBaseURL, Apikey: apikey, UserID: 0, AdditionalQueryParams: additional_query_params, LastRssId: "", Limitercalls: 10, Limiterseconds: 5}, 100, categories, 2)
		//parser.NewVideoFile("", "Y:\\completed\\MoviesDE\\Uncharted.2022.German.AC3LD.5.1.BDRip.x264-PS\\Uncharted.2022.German.AC3LD.5.1.BDRip.x264-PS (tt1464335).mkv", false)
		//searcher2 := searcher.NewSearcher("movie_EN", "SD")
		//movie, _ := database.GetMovies(database.Query{Limit: 1})
		//searcher2.MovieSearch(movie, false, true)

		//scanner.GetFilesDir("c:\\windows", []string{".dll"}, []string{}, []string{})
		//database.QueryDbserieTest4(&database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
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
		_, _ = parserimdb.Read() //skip header
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
				logger.LogDynamicany("error", "an error occurred while parsing csv", err)
				continue
			}
			year, err = strconv.ParseInt(record[10], 0, 32)
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
			d = append(d, database.Dbmovie{ImdbID: record[1], Title: record[5], URL: record[6], VoteAverage: voteavg32, Year: year32, VoteCount: votes32})
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
		_, _ = parserimdb.Read() //skip header
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
			d = append(d, database.Dbmovie{ImdbID: record[1], Title: record[5], URL: record[6], VoteAverage: float32(voteavg), Year: uint16(year), VoteCount: int32(votes)})
		}
		_ = d
	}
}

func BenchmarkQuery7(b *testing.B) {
	Init()
	//val := "C:\\temp\\movies\\movie.mkv"
	//newpath := "C:\\temp\\movies\\movie_temp.mkv"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		//scanner.MoveFileDriveBufferNew(val, newpath)
		//scanner.MoveFileDriveBuffer(val, newpath)
		//scanner.MoveFileDrive(val, newpath)
	}
}

func TestSQLRepeat(t *testing.T) {
	Init()
	a := "a%"
	str := database.GetdatarowN[string](false, "select title from dbmovies where title like ? limit 1", &a)
	_ = str
	str = database.GetdatarowN[string](false, "select title from dbmovies where title like ? limit 1", &a)
	_ = str

}

func TestRequest(t *testing.T) {
	Init()

	ctx, cancelc := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.thetvdb.com/series/341164", nil)
	fmt.Println(err)
	resp, err := http.DefaultClient.Do(req)
	fmt.Println(err)
	fmt.Println(resp.Body)
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, "https://api.thetvdb.com/series/289431", nil)
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
	for _, loop := range config.SettingsMedia {
		cfgp = loop
		break
	}
	t.Log(cfgp.Data[0].CfgPath.MissingScanInterval)
	searchmissing := true
	searchinterval := 100
	var scaninterval uint8
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
	cntquery := database.GetdatarowN[uint](false, "select count() "+bld.String(), args...)

	if cntquery == 0 || query == "" || len(args) == 0 {
		return
	}
	tbl := database.GetrowsNuncached[uint](cntquery, query, args)
	t.Log(tbl)
}

func Files(fsys fs.FS) (paths []string) {
	fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if filepath.Ext(p) == ".ps1" {
			paths = append(paths, p)
		}
		return nil
	})
	return paths
}

func TestParse(t *testing.T) {
	Init()
	cfgp := config.SettingsMedia["movie_EN"]
	m := parser.NewFileParser("Dogma.1999.720p.BluRay.x264-x0r", cfgp, false, -1)
	t.Log(m)
	parser.GetDBIDs(m, cfgp, true)
	t.Log(m)
}
func TestRegexRepeat(t *testing.T) {
	Init()
	//logger.GlobalCacheRegex.GetRegexpDirect("RegexSeriesIdentifier").FindStringSubmatchIndex("S01E01")
	//logger.GlobalCacheRegex.GetRegexpDirect("RegexSeriesIdentifier").FindStringSubmatchIndex("S01E01")
}
func BenchmarkQuery9(b *testing.B) {
	Init()
	//configTemplate := "serie_X"
	//listConfig := "X"
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		//a := "Lois.and.Clark.The.New.Adventures.of.Superman.S02E18.Tempus.Fugitive.SDTV.x264.AAC (tvdb72468)"
		//logger.StringToSlug(a)
		//str := database.GetdatarowN[string](false, "select title from dbmovies where title like ? limit 1", "a%")
		var str string
		a := "a%"
		str = database.GetdatarowN[string](false, "select title from dbmovies where title like ? limit 1", &a)
		_ = str
		//parser.NewFileParser("Lois.and.Clark.The.New.Adventures.of.Superman.S02E18.Tempus.Fugitive.SDTV.x264.AAC (tvdb72468)", false, logger.StrSeries)
		//scanner.MoveFileDriveBufferNew(val, newpath)
		//scanner.MoveFileDriveBuffer(val, newpath)
		//config.ConfigGetMediaListConfig("", listConfig)
	}
	//json, _ := json.Marshal(config.ConfigGetAll())
	//fmt.Println(string(json))
}

func BenchmarkQueryLower(b *testing.B) {
	Init()
	b.ResetTimer()
	str := ""
	var id1 uint = 32
	var id2 uint = 32
	var id3 uint = 32
	var id4 uint = 32
	for i := 0; i < b.N; i++ {
		//scanner.MoveFileDriveBufferNew(val, newpath)
		//scanner.MoveFileDriveBuffer(val, newpath)
		str = strconv.Itoa(int(id1)) + "_" + strconv.Itoa(int(id2)) + "_" + strconv.Itoa(int(id3)) + "_" + strconv.Itoa(int(id4))
		//str = fmt.Sprint(id1, "_", id2, "_", id3, "_", id4)
	}
	logger.LogDynamicany("info", str)
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
		//_, _ = cachetconst[uint32(999555)]
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
	cfgp := config.SettingsMedia[getconfig]
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
	//structurevar.SetOrgadata(&structure.Organizerdata{})
	for i := 0; i < b.N; i++ {
		structure.OrganizeSingleFolder(cfgFolder, cfgp, cfgimport, "en movies", true, false, 0)
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

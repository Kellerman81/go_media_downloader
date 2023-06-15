package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/metadata"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/slidingwindow"
	"github.com/Kellerman81/go_media_downloader/structure"
	"golang.org/x/net/html/charset"
	"golang.org/x/oauth2"
)

//func TestMain(m *testing.M) {
//goleak.VerifyTestMain(m)
//}

//Test with: go.exe test -timeout 30s -v -run ^TestDir$ github.com/Kellerman81/go_media_downloader

func Init() {
	os.Mkdir("./temp", 0777)
	config.LoadCfgDB()

	logger.InitLogger(logger.Config{
		LogLevel:     config.SettingsGeneral.LogLevel,
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
	database.InitDB(config.SettingsGeneral.DBLogLevel)
	apiexternal.NewOmdbClient(config.SettingsGeneral.OmdbAPIKey, config.SettingsGeneral.Omdblimiterseconds, config.SettingsGeneral.Omdblimitercalls, config.SettingsGeneral.OmdbDisableTLSVerify, config.SettingsGeneral.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.SettingsGeneral.TheMovieDBApiKey, config.SettingsGeneral.Tmdblimiterseconds, config.SettingsGeneral.Tmdblimitercalls, config.SettingsGeneral.TheMovieDBDisableTLSVerify, config.SettingsGeneral.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.SettingsGeneral.Tvdblimiterseconds, config.SettingsGeneral.Tvdblimitercalls, config.SettingsGeneral.TvdbDisableTLSVerify, config.SettingsGeneral.TvdbTimeoutSeconds)
	if config.Check("trakt_token") {
		apiexternal.NewTraktClient(config.SettingsGeneral.TraktClientID, config.SettingsGeneral.TraktClientSecret, *config.GetTrakt("trakt_token"), config.SettingsGeneral.Traktlimiterseconds, config.SettingsGeneral.Traktlimitercalls, config.SettingsGeneral.TraktDisableTLSVerify, config.SettingsGeneral.TraktTimeoutSeconds)
	} else {
		apiexternal.NewTraktClient(config.SettingsGeneral.TraktClientID, config.SettingsGeneral.TraktClientSecret, oauth2.Token{}, config.SettingsGeneral.Traktlimiterseconds, config.SettingsGeneral.Traktlimitercalls, config.SettingsGeneral.TraktDisableTLSVerify, config.SettingsGeneral.TraktTimeoutSeconds)
	}
	database.InitImdbdb()

	logger.Log.Info().Msg("Check Database for Upgrades")
	//database.UpgradeDB()
	database.GetVars()
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

func TestSqlTable(t *testing.T) {
	Init()

	t.Run("test", func(t *testing.T) {
		ret := database.QueryStaticColumnsOneStringOneInt(false, 0, "select last_id, id from r_sshistories")
		t.Log(ret)
	})
}

func TestGetAdded(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		//imdb, f1, f2 := importfeed.MovieFindImdbIDByTitle("Firestarter", 1986, logger.StrRss, false)
		//t.Log(imdb)
		//t.Log(f1)
		//t.Log(f2)
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
	fmt.Printf("Alloc = %v MiB", bToMb(m.Alloc))
	fmt.Printf("\tTotalAlloc = %v MiB", bToMb(m.TotalAlloc))
	fmt.Printf("\tSys = %v MiB", bToMb(m.Sys))
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
		req, _ := http.NewRequest("GET", v, nil)
		// Get the data

		resp, err := apiexternal.WebClient.Do(req)
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
			if csverr == io.EOF {
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
	return logger.StringBuilder(logger.UintToString(r), logger.Underscore, logger.UintToString(q), logger.Underscore, logger.UintToString(c), logger.Underscore, logger.UintToString(a))
	//return logger.UintToString(r) + "_" + logger.UintToString(q) + "_" + logger.UintToString(c) + "_" + logger.UintToString(a)
}

func BenchmarkPrio1(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		str := "Hallo123"
		logger.Unquote(&str)
	}
}
func BenchmarkPrio2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		str := "Hallo123"
		logger.Unquote(&str)
	}
}

func BenchmarkMakeRemove2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		str := make([]string, 0, 1000)
		for j := 0; j < 1000; j++ {
			str = append(str, logger.IntToString(j))
		}
		str2 := str[:0]
		for x := range str {
			if str[x] != "500" {
				str2 = append(str2, str[x])
			}
		}
	}
}

func BenchmarkGrowRemove(b *testing.B) {
	var str []string
	for j := 0; j < 1000; j++ {
		str = append(str, logger.IntToString(j))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c := []string{}
		//c = append(c, str...)

		//logger.Grow(&c, len(str))
		c = append(c, str...)
		//logger.RemoveFromStringArray(&str, "500")
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
		//var serie database.Dbmovie
		//tmdbfind, _ := apiexternal.TmdbAPI.FindImdb("tt7214954")
		//t.Log(tmdbfind)
		tmdbtitle, _ := apiexternal.TmdbAPI.GetMovieTitles(585511)
		t.Log(tmdbtitle)
		tmdbdetails, _ := apiexternal.TmdbAPI.GetMovie(585511)
		t.Log(tmdbdetails)
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
		tvdbdetails, _ := apiexternal.TvdbAPI.GetSeries(85352, "")
		if (serie.Seriename == "") && tvdbdetails.Data.SeriesName != "" {
			serie.Seriename = tvdbdetails.Data.SeriesName
		}
		t.Log(serie.Seriename)
		var dbserie database.Dbserie
		dbserie.ThetvdbID = 85352
		metadata.SerieGetMetadata(&dbserie, "", true, true, true, true)
		t.Log(dbserie)
		t.Log(dbserie.ImdbID)
		t.Log(dbserie.ID)
		t.Log(dbserie.Seriename)
		//GetNewFilesTest("serie_EN", logger.StrSeries)
	})
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
func TestGetDB(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		var out []database.Dbmovie
		database.QueryDbmovie(database.Querywithargs{Limit: 10}, &out)
		t.Log(out)
		var dbm database.Dbmovie
		database.GetDbmovie(logger.FilterByID, 1)
		t.Log(dbm)
	})
}

func TestLst(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		var query database.Querywithargs
		query.InnerJoin = "Dbseries on series.dbserie_id=dbseries.id"
		query.Where = "series.listname = ? COLLATE NOCASE"
		rows := database.CountRows(logger.StrSeries, query)
		t.Log(rows)
		// limit := 0
		// page := 0
		//series, _ := database.QueryResultSeries(&query, "X")
		//t.Log(series)
	})
}

func TestSqlQuery(t *testing.T) {
	Init()
	listname := "DE"
	t.Run("test", func(t *testing.T) {
		rows := database.QueryCountColumn("series", "listname = ?", &listname)
		t.Log(rows)
		// limit := 0
		// page := 0
		//series, _ := database.QueryResultSeries(&query, "X")
		//t.Log(series)
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

func TestDir(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		//bla, _ := scanner.GetFilesDir("W:\\", "de movies", false)
		//t.Log(bla)
		//t.Log(config.Cfg.Paths)
	})
}

func TestFilter(t *testing.T) {
	Init()
	t.Run("testfilter", func(t *testing.T) {
		//bla, _ := scanner.GetFilesDir("W:\\", "de movies", false)
		//t.Log(bla)
		filter := "C:\test\video.rtf"
		bl := scanner.Filterfile(&filter, false, "path_de movies")
		t.Log(bl)
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
func BenchmarkXML1(b *testing.B) {
	xmlstr := `<rss xmlns:atom="http://www.w3.org/2005/Atom" xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/" version="2.0">
<channel><item>
<title>SHMN-Dogma-POM184-WEBFLAC-2023-AFO</title>
<guid isPermaLink="true">https://nzbgeek.info/geekseek.php?guid=64f715017aabd4cc1b616aa15bf47ca0</guid>
<link>https://api.nzbgeek.info/api?t=getid=64f715017aabd4cc1b616aa15bf47ca0apikey=</link>
<comments>https://nzbgeek.info/geekseek.php?guid=64f715017aabd4cc1b616aa15bf47ca0</comments>
<pubDate>Thu, 26 Jan 2023 13:47:38 +0000</pubDate>
<category>Audio > Lossless</category>
<description>SHMN-Dogma-POM184-WEBFLAC-2023-AFO</description>
<enclosure url="https://api.nzbgeek.info/api?t=getid=64f715017aabd4cc1b616aa15bf47ca0apikey=" length="98813000" type="application/x-nzb"/>
<newznab:attr name="category" value="3000"/>
<newznab:attr name="category" value="3040"/>
<newznab:attr name="size" value="98813000"/>
<newznab:attr name="guid" value="64f715017aabd4cc1b616aa15bf47ca0"/>
<newznab:attr name="grabs" value="7"/>
<newznab:attr name="usenetdate" value="Thu, 26 Jan 2023 13:47:38 +0000"/>
</item>
<item>
<title>Water.Gate.Bridge.2022.CHINESE.1080p.BluRay.REMUX.AVC.DTS-HD.MA.5.1-FGT</title>
<guid isPermaLink="true">https://nzbgeek.info/geekseek.php?guid=9d997fa89cbf18954cf1f30aee42369c</guid>
<link>https://api.nzbgeek.info/api?t=getid=9d997fa89cbf18954cf1f30aee42369capikey=</link>
<comments>https://nzbgeek.info/geekseek.php?guid=9d997fa89cbf18954cf1f30aee42369c</comments>
<pubDate>Tue, 24 Jan 2023 09:08:08 +0000</pubDate>
<category>Movies > Foreign</category>
<description>Water.Gate.Bridge.2022.CHINESE.1080p.BluRay.REMUX.AVC.DTS-HD.MA.5.1-FGT</description>
<enclosure url="https://api.nzbgeek.info/api?t=getid=9d997fa89cbf18954cf1f30aee42369capikey=" length="39185647000" type="application/x-nzb"/>
<newznab:attr name="category" value="2000"/>
<newznab:attr name="category" value="2010"/>
<newznab:attr name="size" value="39185647000"/>
<newznab:attr name="guid" value="9d997fa89cbf18954cf1f30aee42369c"/>
<newznab:attr name="imdbtitle" value="Water Gate Bridge"/>
<newznab:attr name="imdb" value="16194408"/>
<newznab:attr name="imdbplot" value="Sequel to The Battle at Lake Changjin Follows the Chinese Peoples Volunteers CPV soldiers on a new task, and now their battlefield is a crucial bridge on the retreat route of American troops"/>
<newznab:attr name="imdbscore" value="5.4"/>
<newznab:attr name="genre" value="Action, Drama, History"/>
<newznab:attr name="imdbyear" value="2022"/>
<newznab:attr name="imdbdirector" value="Hark Tsui, Kaige Chen, Dante Lam"/>
<newznab:attr name="imdbactors" value="Jing Wu, Jackson Yee, Michael Koltes"/>
<newznab:attr name="coverurl" value="https://api.nzbgeek.info/covers/movies/16194408-cover.jpg"/>
<newznab:attr name="runtime" value="153 min"/>
<newznab:attr name="language" value="Chinese"/>
<newznab:attr name="subs " value="English"/>
<newznab:attr name="grabs" value="30"/>
<newznab:attr name="usenetdate" value="Tue, 24 Jan 2023 09:08:08 +0000"/>
<newznab:attr name="thumbsup" value="1"/>
</item>
</channel>
</rss>`

	for i := 0; i < b.N; i++ {
		d := xml.NewDecoder(strings.NewReader(xmlstr))

		var a []apiexternal.NZB
		var startv, endv int64
		var name string
		var b apiexternal.NZB
		var t xml.Token
		var i, j int
		var err error

		for {
			t, err = d.RawToken()
			if err != nil {
				break
			}
			switch t.(type) {
			case xml.StartElement:
				if t.(xml.StartElement).Name.Local == "item" {

					startv = d.InputOffset()
					break //Switch not for
				}
				if endv > startv {
					break //Switch not for
				}
				name = t.(xml.StartElement).Name.Local
				switch t.(xml.StartElement).Name.Local {
				case "enclosure":
					i = logger.IndexFunc(logger.GetP(t.(xml.StartElement).Attr), func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "url") })
					if i == -1 {
						break //Switch not for
					}
					b.DownloadURL = t.(xml.StartElement).Attr[i].Value
					if strings.Contains(b.DownloadURL, ".torrent") || strings.Contains(b.DownloadURL, "magnet:?") {
						b.IsTorrent = true
					}
				case "attr":
					i = logger.IndexFunc(logger.GetP(t.(xml.StartElement).Attr), func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "name") })
					j = logger.IndexFunc(logger.GetP(t.(xml.StartElement).Attr), func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "value") })

					if i == -1 || j == -1 || t.(xml.StartElement).Attr[j].Value == "" {
						break //Switch not for
					}
					if strings.EqualFold(t.(xml.StartElement).Attr[i].Value, "size") {
						in, _ := strconv.ParseUint(t.(xml.StartElement).Attr[j].Value, 10, 64)
						b.Size = int64(in)
					} else if strings.EqualFold(t.(xml.StartElement).Attr[i].Value, "guid") && b.ID == "" {
						b.ID = t.(xml.StartElement).Attr[j].Value
					} else if strings.EqualFold(t.(xml.StartElement).Attr[i].Value, "tvdbid") {
						in, _ := strconv.ParseUint(t.(xml.StartElement).Attr[j].Value, 10, 0)
						b.TVDBID = int(in)
					} else if strings.EqualFold(t.(xml.StartElement).Attr[i].Value, "season") {
						b.Season = t.(xml.StartElement).Attr[j].Value
					} else if strings.EqualFold(t.(xml.StartElement).Attr[i].Value, "episode") {
						b.Episode = t.(xml.StartElement).Attr[j].Value
					} else if strings.EqualFold(t.(xml.StartElement).Attr[i].Value, "imdb") {
						b.IMDBID = t.(xml.StartElement).Attr[j].Value
					}
				}
			case xml.CharData:
				if endv > startv { //endv is previous
					break //Switch not for
				}

				if name == "title" {
					if b.Title == "" {
						b.Title = string(t.(xml.CharData))
					}
				} else if name == "guid" {
					if b.ID == "" {
						b.ID = string(t.(xml.CharData))
					}
				} else if name == "size" {
					in, _ := strconv.ParseInt(string(t.(xml.CharData)), 10, 64)
					b.Size = in
				}
			case xml.EndElement:
				if t.(xml.EndElement).Name.Local == "item" {
					endv = d.InputOffset()
					if startv >= endv || b.DownloadURL == "" {
						break //Switch not for
					}
					if b.ID == "" {
						b.ID = b.DownloadURL
					}
					//b.SourceEndpoint = apiBaseURL
					a = append(a, b)
					b = apiexternal.NZB{}
				} else if startv > endv {
					name = ""
				}
			}
		}
	}
}
func BenchmarkXML2(b *testing.B) {
	xmlstr := `<rss xmlns:atom="http://www.w3.org/2005/Atom" xmlns:newznab="http://www.newznab.com/DTD/2010/feeds/attributes/" version="2.0">
<channel><item>
<title>SHMN-Dogma-POM184-WEBFLAC-2023-AFO</title>
<guid isPermaLink="true">https://nzbgeek.info/geekseek.php?guid=64f715017aabd4cc1b616aa15bf47ca0</guid>
<link>https://api.nzbgeek.info/api?t=getid=64f715017aabd4cc1b616aa15bf47ca0apikey=</link>
<comments>https://nzbgeek.info/geekseek.php?guid=64f715017aabd4cc1b616aa15bf47ca0</comments>
<pubDate>Thu, 26 Jan 2023 13:47:38 +0000</pubDate>
<category>Audio > Lossless</category>
<description>SHMN-Dogma-POM184-WEBFLAC-2023-AFO</description>
<enclosure url="https://api.nzbgeek.info/api?t=getid=64f715017aabd4cc1b616aa15bf47ca0apikey=" length="98813000" type="application/x-nzb"/>
<newznab:attr name="category" value="3000"/>
<newznab:attr name="category" value="3040"/>
<newznab:attr name="size" value="98813000"/>
<newznab:attr name="guid" value="64f715017aabd4cc1b616aa15bf47ca0"/>
<newznab:attr name="grabs" value="7"/>
<newznab:attr name="usenetdate" value="Thu, 26 Jan 2023 13:47:38 +0000"/>
</item>
<item>
<title>Water.Gate.Bridge.2022.CHINESE.1080p.BluRay.REMUX.AVC.DTS-HD.MA.5.1-FGT</title>
<guid isPermaLink="true">https://nzbgeek.info/geekseek.php?guid=9d997fa89cbf18954cf1f30aee42369c</guid>
<link>https://api.nzbgeek.info/api?t=getid=9d997fa89cbf18954cf1f30aee42369capikey=</link>
<comments>https://nzbgeek.info/geekseek.php?guid=9d997fa89cbf18954cf1f30aee42369c</comments>
<pubDate>Tue, 24 Jan 2023 09:08:08 +0000</pubDate>
<category>Movies > Foreign</category>
<description>Water.Gate.Bridge.2022.CHINESE.1080p.BluRay.REMUX.AVC.DTS-HD.MA.5.1-FGT</description>
<enclosure url="https://api.nzbgeek.info/api?t=getid=9d997fa89cbf18954cf1f30aee42369capikey=" length="39185647000" type="application/x-nzb"/>
<newznab:attr name="category" value="2000"/>
<newznab:attr name="category" value="2010"/>
<newznab:attr name="size" value="39185647000"/>
<newznab:attr name="guid" value="9d997fa89cbf18954cf1f30aee42369c"/>
<newznab:attr name="imdbtitle" value="Water Gate Bridge"/>
<newznab:attr name="imdb" value="16194408"/>
<newznab:attr name="imdbplot" value="Sequel to The Battle at Lake Changjin Follows the Chinese Peoples Volunteers CPV soldiers on a new task, and now their battlefield is a crucial bridge on the retreat route of American troops"/>
<newznab:attr name="imdbscore" value="5.4"/>
<newznab:attr name="genre" value="Action, Drama, History"/>
<newznab:attr name="imdbyear" value="2022"/>
<newznab:attr name="imdbdirector" value="Hark Tsui, Kaige Chen, Dante Lam"/>
<newznab:attr name="imdbactors" value="Jing Wu, Jackson Yee, Michael Koltes"/>
<newznab:attr name="coverurl" value="https://api.nzbgeek.info/covers/movies/16194408-cover.jpg"/>
<newznab:attr name="runtime" value="153 min"/>
<newznab:attr name="language" value="Chinese"/>
<newznab:attr name="subs " value="English"/>
<newznab:attr name="grabs" value="30"/>
<newznab:attr name="usenetdate" value="Tue, 24 Jan 2023 09:08:08 +0000"/>
<newznab:attr name="thumbsup" value="1"/>
</item>
</channel>
</rss>`

	for i := 0; i < b.N; i++ {
		d := xml.NewDecoder(strings.NewReader(xmlstr))

		var a []apiexternal.NZB
		var b apiexternal.NZB
		var name string
		var s, t xml.Token

		//var t xml.Token
		var err error
		var i, j int
		var breaksecond bool

		for {
			s, err = d.RawToken()
			if err != nil {
				fmt.Println(err)
				break
			}
			switch s.(type) {
			case xml.StartElement:
				if s.(xml.StartElement).Name.Local == "item" {
					breaksecond = false
					for {
						t, err = d.RawToken()
						if err != nil {
							//fmt.Println(err)
							break
						}
						switch tt := t.(type) {
						case xml.StartElement:
							name = tt.Name.Local
							switch tt.Name.Local {
							case "enclosure":
								i = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "url") })
								if i == -1 {
									break //Switch not for
								}
								b.DownloadURL = tt.Attr[i].Value
								if strings.Contains(b.DownloadURL, ".torrent") || strings.Contains(b.DownloadURL, "magnet:?") {
									b.IsTorrent = true
								}
							case "attr":
								i = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "name") })
								j = logger.IndexFunc(&tt.Attr, func(e xml.Attr) bool { return strings.EqualFold(e.Name.Local, "value") })

								if i == -1 || j == -1 || tt.Attr[j].Value == "" {
									break //Switch not for
								}
								if strings.EqualFold(tt.Attr[i].Value, "size") {
									in, _ := strconv.ParseUint(tt.Attr[j].Value, 10, 64)
									b.Size = int64(in)
								} else if strings.EqualFold(tt.Attr[i].Value, "guid") && b.ID == "" {
									b.ID = tt.Attr[j].Value
								} else if strings.EqualFold(tt.Attr[i].Value, "tvdbid") {
									in, _ := strconv.ParseUint(tt.Attr[j].Value, 10, 0)
									b.TVDBID = int(in)
								} else if strings.EqualFold(tt.Attr[i].Value, "season") {
									b.Season = tt.Attr[j].Value
								} else if strings.EqualFold(tt.Attr[i].Value, "episode") {
									b.Episode = tt.Attr[j].Value
								} else if strings.EqualFold(tt.Attr[i].Value, "imdb") {
									b.IMDBID = tt.Attr[j].Value
								}
							}
						case xml.CharData:
							switch name {
							case "title":
								if b.Title == "" {
									b.Title = string(tt)
								}
							case "link":
								if b.DownloadURL == "" {
									b.DownloadURL = string(tt)
								}
							case "guid":
								if b.ID == "" {
									b.ID = string(tt)
								}
							case "size":
								in, _ := strconv.ParseInt(string(tt), 10, 64)
								b.Size = in
							}
						case xml.EndElement:
							if tt.Name.Local == "item" {
								a = append(a, b)
								b = apiexternal.NZB{}
								breaksecond = true
								break
							}
						}
						if breaksecond {
							break
						}
					}
				}
			}
		}
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

func BenchmarkStruct(b *testing.B) {
	var d A

	b.ReportAllocs()
	b.SetBytes(0)
	for i := 0; i < b.N; i++ {
		d.Str1 = "aa"
		d.Str2 = "sdkfdksfködsfsd"
		d.Str3 = "sdfkdskfmkdsmkfm"
		d.Str4 = "sdkfkdmdfkmsdkmfksm"
		d.Str5 = "sdfkweöfmöwemrmekmwflmlw"
		d.Str6 = "kkwmlfkmlmdslflsdlfmlwlmkekmlfmlfwdmkl"
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
func BenchmarkQueryXML1(b *testing.B) {
	Init()
	c := apiexternal.NewClient(
		true,
		true,
		slidingwindow.NewLimiter(time.Duration(1)*time.Second, 10000000),
		false,
		nil, 10)
	b.ResetTimer()
	for i := 0; i < 100; i++ {
		c.DoXML("nzbgeek", "sd", "", "nzbgeek.info", "https://api.nzbgeek.info/api?t=search&q=chinese&limit=100&extended=1&apikey=")
	}
}

func BenchmarkQuery4(b *testing.B) {
	Init()
	// additionalQueryParams := "&extended=1&maxsize=6291456000"
	// categories := "2030,2035,2040,2045"
	// apikey := ""
	// apiBaseURL := "https://api.nzbgeek.info"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		str := "SD"
		searcher.SearchRSS("", str, logger.StrMovie, true)
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
		var year32, votes32 int
		var voteavg float64
		var voteavg32 float32
		for {
			record, err2 = parserimdb.Read()
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				logger.Logerror(err, "an error occurred while parsing csv")
				continue
			}
			year, err = strconv.ParseInt(record[10], 0, 32)
			if err != nil {
				continue
			}
			year32 = int(year)
			votes, err = strconv.ParseInt(record[12], 0, 32)
			if err != nil {
				continue
			}
			votes32 = int(votes)
			voteavg, err = strconv.ParseFloat(record[8], 32)
			if err != nil {
				continue
			}
			voteavg32 = float32(voteavg)
			d = append(d, database.Dbmovie{ImdbID: record[1], Title: record[5], URL: record[6], VoteAverage: voteavg32, Year: year32, VoteCount: votes32})
		}
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
			d = append(d, database.Dbmovie{ImdbID: record[1], Title: record[5], URL: record[6], VoteAverage: float32(voteavg), Year: int(year), VoteCount: int(votes)})
		}
	}
}

func BenchmarkQuery7(b *testing.B) {
	Init()
	val := "C:\\temp\\movies\\movie.mkv"
	newpath := "C:\\temp\\movies\\movie_temp.mkv"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		//scanner.MoveFileDriveBufferNew(val, newpath)
		//scanner.MoveFileDriveBuffer(val, newpath)
		scanner.MoveFileDrive(val, newpath)

	}
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
		//str := database.QueryStringColumn("select title from dbmovies where title like ? limit 1", "a%")
		var str string
		_ = database.QueryColumn("select title from dbmovies where title like ? limit 1", &str, "a%")
		//_ = str
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
	logger.Log.Info().Msg(str)
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
	cfgDisableruntimecheck := true
	cfgDisabledeletewronglanguage := false
	cfgGrouptype := logger.StrMovie
	getconfig := "movie_EN"
	cacheunmatched := logger.StrSerieFileUnmatched
	if cfgGrouptype != logger.StrSeries {
		cacheunmatched = logger.StrMovieFileUnmatched
	}
	structurevar, _ := structure.NewStructure(getconfig, "", cfgGrouptype, "", "path_"+"en movies", "path_"+"en movies import")
	for i := 0; i < b.N; i++ {
		structure.OrganizeSingleFolder(cfgFolder, cfgDisableruntimecheck, cfgDisabledeletewronglanguage, cacheunmatched, structurevar)
	}
	structurevar.Close()
}
func BenchmarkQuery15(b *testing.B) {
	Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importfeed.JobImportMovies("", "", "", false)
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
	new := s.Values[:0]
	for _, val := range s.Values {
		if val != valchk {
			new = append(new, val)
		}
	}
	s.Values = new
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

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/utils"
	"go.uber.org/zap"
	"golang.org/x/net/html/charset"
	"golang.org/x/oauth2"
)

//func TestMain(m *testing.M) {
//goleak.VerifyTestMain(m)
//}

func Init() {
	os.Mkdir("./temp", 0777)
	config.LoadCfgDB(config.GetCfgFile())

	logger.InitLogger(logger.LoggerConfig{
		LogLevel:     config.Cfg.General.LogLevel,
		LogFileSize:  config.Cfg.General.LogFileSize,
		LogFileCount: config.Cfg.General.LogFileCount,
		LogCompress:  config.Cfg.General.LogCompress,
	})
	if config.Cfg.General.WebPort == "" {
		//fmt.Println("Checked for general - config is missing", cfg_general)
		//os.Exit(0)
		config.ClearCfg()
		config.WriteCfg()
		config.LoadCfgDB(config.GetCfgFile())
	}
	database.InitDb(config.Cfg.General.DBLogLevel)
	apiexternal.NewOmdbClient(config.Cfg.General.OmdbApiKey, config.Cfg.General.Omdblimiterseconds, config.Cfg.General.Omdblimitercalls, config.Cfg.General.OmdbDisableTLSVerify, config.Cfg.General.OmdbTimeoutSeconds)
	apiexternal.NewTmdbClient(config.Cfg.General.TheMovieDBApiKey, config.Cfg.General.Tmdblimiterseconds, config.Cfg.General.Tmdblimitercalls, config.Cfg.General.TheMovieDBDisableTLSVerify, config.Cfg.General.TmdbTimeoutSeconds)
	apiexternal.NewTvdbClient(config.Cfg.General.Tvdblimiterseconds, config.Cfg.General.Tvdblimitercalls, config.Cfg.General.TvdbDisableTLSVerify, config.Cfg.General.TvdbTimeoutSeconds)
	if config.ConfigCheck("trakt_token") {
		apiexternal.NewTraktClient(config.Cfg.General.TraktClientId, config.Cfg.General.TraktClientSecret, *config.ConfigGetTrakt("trakt_token"), config.Cfg.General.Traktlimiterseconds, config.Cfg.General.Traktlimitercalls, config.Cfg.General.TraktDisableTLSVerify, config.Cfg.General.TraktTimeoutSeconds)
	} else {
		apiexternal.NewTraktClient(config.Cfg.General.TraktClientId, config.Cfg.General.TraktClientSecret, oauth2.Token{}, config.Cfg.General.Traktlimiterseconds, config.Cfg.General.Traktlimitercalls, config.Cfg.General.TraktDisableTLSVerify, config.Cfg.General.TraktTimeoutSeconds)
	}
	database.InitImdbdb(config.Cfg.General.DBLogLevel)

	logger.Log.GlobalLogger.Info("Check Database for Upgrades")
	//database.UpgradeDB()
	database.GetVars()
	utils.InitRegex()
}

func TestStructure(t *testing.T) {
	Init()
	//configTemplate := "movie_EN"
	//structure.StructureSingleFolder("Y:\\completed\\MoviesDE\\Rot.2022.German.AC3.BDRiP.x264-SAVASTANOS", false, false, false, "movie", "path_en movies import", "path_en movies", configTemplate)
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

func TestGetAdded(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		imdb, f1, f2 := importfeed.MovieFindImdbIDByTitle("Firestarter", "1986", "rss", false)
		t.Log(imdb)
		t.Log(f1)
		t.Log(f2)
		//GetNewFilesTest("serie_EN", "series")
	})
}

func TestParseXML(t *testing.T) {
	url := "https://api.nzbgeek.info/api?apikey=rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7&tvdbid=82701&season=1&ep=8&cat=5030&dl=1&t=tvsearch&extended=1"

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

func TestGetTmdb(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		//var serie database.Dbmovie
		tmdbfind, _ := apiexternal.TmdbApi.FindImdb("tt7214954")
		t.Log(tmdbfind)
		tmdbtitle, _ := apiexternal.TmdbApi.GetMovieTitles("585511")
		t.Log(tmdbtitle)
		tmdbdetails, _ := apiexternal.TmdbApi.GetMovie("585511")
		t.Log(tmdbdetails)
		//var dbserie database.Dbserie
		//dbserie.ThetvdbID = 85352
		//dbserie.GetMetadata("", true, true, true, true)
		//t.Log(dbserie)
		//t.Log(dbserie.ImdbID)
		//t.Log(dbserie.ID)
		//t.Log(dbserie.Seriename)
		//GetNewFilesTest("serie_EN", "series")
	})
}
func TestGetTvdb(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		var serie database.Dbserie
		tvdbdetails, _ := apiexternal.TvdbApi.GetSeries(85352, "")
		if (serie.Seriename == "") && tvdbdetails.Data.SeriesName != "" {
			serie.Seriename = tvdbdetails.Data.SeriesName
		}
		t.Log(serie.Seriename)
		var dbserie database.Dbserie
		dbserie.ThetvdbID = 85352
		dbserie.GetMetadata("", true, true, true, true)
		t.Log(dbserie)
		t.Log(dbserie.ImdbID)
		t.Log(dbserie.ID)
		t.Log(dbserie.Seriename)
		//GetNewFilesTest("serie_EN", "series")
	})
}
func TestGetDB(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		out, _ := database.QueryDbmovie(database.Querywithargs{Query: database.Query{Limit: 10}})
		t.Log(out)
		dbm, _ := database.GetDbmovie(database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{1}})
		t.Log(dbm)
		mm := make(map[string]int)
		database.QueryStaticColumnsMapStringInt(&mm, database.Querywithargs{QueryString: "Select imdb_id, id from dbmovies limit 10"})
		t.Log(mm)
		//GetNewFilesTest("serie_EN", "series")
	})
}

func TestLst(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		var query database.Query
		query.InnerJoin = "Dbseries on series.dbserie_id=dbseries.id"
		query.Where = "series.listname = ? COLLATE NOCASE"
		rows, _ := database.CountRows("series", database.Querywithargs{Query: query})
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

func TestCache(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		logger.GlobalRegexCache.SetRegexp("ff", "[a-z]", 0)

		a := logger.GlobalRegexCache.GetRegexpDirect("ff")
		t.Log(a)
		a = nil
		a = logger.GlobalRegexCache.GetRegexpDirect("ff")
		t.Log(a)
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

func BenchmarkQuerySQL1(b *testing.B) {
	Init()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		database.QueryStaticColumnsOneStringOneInt(false, database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from dbmovies"}), database.Querywithargs{QueryString: "select imdb_id, id from dbmovies"})
	}
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
	apikey := "rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7"
	apiPath := "/api"
	apiBaseURL := "https://api.nzbgeek.info"
	// customurl := ""
	// query := "test"
	// addquotesfortitlequery := false
	// outputAsJson := false
	// searchtype := "query"
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
	apikey := "rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7"
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
	apikey := "rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7"
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
func BenchmarkQuery3(b *testing.B) {
	//Init()
	//b.ResetTimer()
	additionalQueryParams := "&extended=1&maxsize=6291456000"
	categories := []int{2030, 2035, 2040, 2045}
	episode := 1
	season := 10
	tvDBID := 55797
	apikey := "rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7"
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

func BenchmarkQuery4(b *testing.B) {
	Init()
	// additionalQueryParams := "&extended=1&maxsize=6291456000"
	// categories := "2030,2035,2040,2045"
	// apikey := "rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7"
	// apiBaseURL := "https://api.nzbgeek.info"
	s := searcher.NewSearcher(&config.MediaTypeConfig{}, "SD")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SearchRSS("movie", true)
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
				logger.Log.GlobalLogger.Error("an error occurred while parsing csv.. ", zap.Error(err))
				continue
			}
			if !importfeed.AllowMovieImport(record[1], "Watchlist") {
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
			if !importfeed.AllowMovieImport(record[1], "Watchlist") {
				continue
			}
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
		//d = nil
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
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
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
	logger.Log.GlobalLogger.Info(str)
}
func BenchmarkQueryLower2(b *testing.B) {
	Init()
	b.ResetTimer()
	str := "Movie"
	for i := 0; i < b.N; i++ {
		if strings.EqualFold(str, "movie") {
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
		_, _ = cachetconst[uint32(999555)]
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
	cfgDisabledisallowed := false
	cfgDisabledeletewronglanguage := false
	cfgGrouptype := "movie"
	//getconfig := "movie_EN"
	for i := 0; i < b.N; i++ {
		structure.StructureSingleFolder(cfgFolder, &config.MediaTypeConfig{}, structure.StructureConfig{Disableruntimecheck: cfgDisableruntimecheck, Disabledisallowed: cfgDisabledisallowed, Disabledeletewronglanguage: cfgDisabledeletewronglanguage, Grouptype: cfgGrouptype, Sourcepathstr: "path_" + "en movies", Targetpathstr: "path_" + "en movies import"})
	}
}
func BenchmarkQuery15(b *testing.B) {
	Init()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		importfeed.JobImportMovies("tt0120655", &config.MediaTypeConfig{}, "", false)
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
	//new = nil
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
	//s = nil
}

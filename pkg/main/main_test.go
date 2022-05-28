package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/utils"
)

//func TestMain(m *testing.M) {
//goleak.VerifyTestMain(m)
//}

func Init() {

	os.Mkdir("./temp", 0777)
	config.LoadCfgDB(config.Configfile)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WebPort == "" {
		//fmt.Println("Checked for general - config is missing", cfg_general)
		//os.Exit(0)
		config.ClearCfg()
		config.WriteCfg()
		config.LoadCfgDB(config.Configfile)
	}
	database.InitDb(cfg_general.DBLogLevel)

	database.DBImdb = database.InitImdbdb(cfg_general.DBLogLevel, "imdb")

	logger.Log.Infoln("Check Database for Upgrades")
	//database.UpgradeDB()
	database.GetVars()
	parser.LoadDBPatterns()
	utils.InitRegex()
}

func Test_structure(t *testing.T) {
	Init()
	configTemplate := "movie_EN"
	structure.StructureSingleFolder("Y:\\completed\\MoviesDE\\Rot.2022.German.AC3.BDRiP.x264-SAVASTANOS", false, false, false, "movie", "path_en movies import", "path_en movies", configTemplate)
}
func Test_main(t *testing.T) {
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
			// dbseries, _ := database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

			// for idxserie := range dbseries {
			// 	importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
			// }
		})
	}
}

func Test_GetAdded(t *testing.T) {
	Init()
	t.Run("test", func(t *testing.T) {
		imdb, f1, f2 := importfeed.MovieFindImdbIDByTitle("Firestarter", "1986", "rss", false)
		fmt.Println(imdb)
		fmt.Println(f1)
		fmt.Println(f2)
		//GetNewFilesTest("serie_EN", "series")
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
func BenchmarkQuery1(b *testing.B) {
	additional_query_params := "&extended=1&maxsize=6291456000"
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
		buildurl.WriteString(additional_query_params)
		fmt.Println(buildurl.String())
		//database.CountRowsTest1("dbseries", database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest1(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkQuery2(b *testing.B) {
	//Init()
	//b.ResetTimer()
	additional_query_params := "&extended=1&maxsize=6291456000"
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
		buildurl.WriteString(additional_query_params)
		fmt.Println(buildurl.String())
		//database.CountRowsTest2("dbseries", database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest2(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}
func BenchmarkQuery3(b *testing.B) {
	//Init()
	//b.ResetTimer()
	additional_query_params := "&extended=1&maxsize=6291456000"
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
		buildurl.WriteString(additional_query_params)
		fmt.Println(buildurl.String())

		//database.CountRows("dbseries", database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserieTest3(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})
		//database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

		//for idxserie := range dbseries {
		//importfeed.JobReloadDbSeries(dbseries[idxserie], "", "", true)
		//}
	}
}

func BenchmarkQuery4(b *testing.B) {
	Init()
	// additional_query_params := "&extended=1&maxsize=6291456000"
	// categories := "2030,2035,2040,2045"
	// apikey := "rEUDNavst5HxWG2SlhkuYg1WXC6qNSt7"
	// apiBaseURL := "https://api.nzbgeek.info"
	s := searcher.NewSearcher("movie_EN", "SD")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.SearchRSS("movie", true)
		//clie.SearchWithIMDB(categories, "tt0120655", additional_query_params, "", 0, false)
		//clie.LoadRSSFeed(categories, 100, additional_query_params, "", "", "", 0, false)
		//apiexternal.QueryNewznabRSSLast(apiexternal.NzbIndexer{URL: apiBaseURL, Apikey: apikey, UserID: 0, Additional_query_params: additional_query_params, LastRssId: "", Limitercalls: 10, Limiterseconds: 5}, 100, categories, 2)
		//parser.NewVideoFile("", "Y:\\completed\\MoviesDE\\Uncharted.2022.German.AC3LD.5.1.BDRip.x264-PS\\Uncharted.2022.German.AC3LD.5.1.BDRip.x264-PS (tt1464335).mkv", false)
		//searcher2 := searcher.NewSearcher("movie_EN", "SD")
		//movie, _ := database.GetMovies(database.Query{Limit: 1})
		//searcher2.MovieSearch(movie, false, true)

		//scanner.GetFilesDir("c:\\windows", []string{".dll"}, []string{}, []string{})
		//database.QueryDbserieTest4(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 20})

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

		resp, err := logger.GetUrlResponse("https://www.imdb.com/list/ls003672378/export")
		if err != nil {
			continue
		}
		defer resp.Body.Close()
		parserimdb := csv.NewReader(resp.Body)
		parserimdb.ReuseRecord = true
		var d []database.Dbmovie
		_, _ = parserimdb.Read() //skip header
		for {
			record, err2 := parserimdb.Read()
			if err2 == io.EOF {
				break
			}
			if err2 != nil {
				logger.Log.Errorln("an error occurred while parsing csv.. ", err)
				continue
			}
			if !importfeed.AllowMovieImport(record[1], "Watchlist") {
				continue
			}
			year, err := strconv.ParseInt(record[10], 0, 32)
			if err != nil {
				continue
			}
			year32 := int(year)
			votes, err := strconv.ParseInt(record[12], 0, 32)
			if err != nil {
				continue
			}
			votes32 := int(votes)
			voteavg, err := strconv.ParseFloat(record[8], 32)
			if err != nil {
				continue
			}
			voteavg32 := float32(voteavg)
			d = append(d, database.Dbmovie{ImdbID: record[1], Title: record[5], URL: record[6], VoteAverage: voteavg32, Year: year32, VoteCount: votes32})
		}
	}
}

func BenchmarkQuery6(b *testing.B) {
	Init()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := logger.GetUrlResponse("https://www.imdb.com/list/ls003672378/export")
		if err != nil {
			continue
		}

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
		d = nil
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
	configTemplate := "serie_X"
	listConfig := "X"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		//scanner.MoveFileDriveBufferNew(val, newpath)
		//scanner.MoveFileDriveBuffer(val, newpath)
		list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
		_ = list
	}
}
func BenchmarkQuery10(b *testing.B) {
	Init()
	b.ResetTimer()
	configTemplate := "serie_X"
	listConfig := "X"

	file := "C:\\temp\\eurogirlsongirls 21-02-08 blondie fesser and julia de lucia 1080P WEBRIP.mp4"
	for i := 0; i < b.N; i++ {
		list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
		if list.Name == "" {

			continue
		}
		if ok := utils.Checkignorelistsonpath(configTemplate, file, listConfig); !ok {

			continue
		}
		if ok := utils.Checkunmatched(configTemplate, file, listConfig); !ok {
			continue
		}
		counter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_id in (Select id from series where listname = ?) and serie_episode_id <> 0", file, listConfig)
		if counter >= 1 {
			continue
		}
		//scanner.MoveFileDriveBufferNew(val, newpath)
		//scanner.MoveFileDriveBuffer(val, newpath)
		//tst, err := parser.NewVideoFile("ffprobe.exe", "C:\\temp\\eurogirlsongirls 21-02-08 blondie fesser and julia de lucia 1080P WEBRIP.mp4", false)
		//fmt.Println(tst)
		//_ = tst
		// = err
		m, err := parser.NewFileParser(filepath.Base(file), true, "series")
		if err != nil {
			fmt.Println("err ", err)
			continue
		}
		defer m.Close()

		m.Resolution = strings.ToLower(m.Resolution)
		m.Audio = strings.ToLower(m.Audio)
		m.Codec = strings.ToLower(m.Codec)
		var titlebuilder bytes.Buffer
		titlebuilder.WriteString(m.Title)
		if m.Year != 0 {
			titlebuilder.WriteString(" (")
			titlebuilder.WriteString(strconv.Itoa(m.Year))
			titlebuilder.WriteString(")")
		}
		seriestitle := ""
		matched := config.RegexGet("RegexSeriesTitle").FindStringSubmatch(filepath.Base(file))
		if len(matched) >= 2 {
			seriestitle = matched[1]
		}
		matched = nil
		logger.Log.Debug("Parsed SerieEpisode: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " Matched: ", matched, " Identifier: ", m.Identifier, " Date: ", m.Date, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)

		series, entriesfound, err := m.FindSerieByParser(titlebuilder.String(), seriestitle, "X")

		//addunmatched := false
		//configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
		if err == nil {
			defer logger.ClearVar(series)
			if entriesfound >= 1 {
				m.GetPriority(configTemplate, list.Template_quality)
				errparsev := m.ParseVideoFile(file, configTemplate, list.Template_quality)
				if errparsev != nil {

					logger.Log.Error("Parse failed: ", errparsev)
					continue
				}
				continue

				dbserie, err := database.QueryColumnUint("Select dbserie_id from series where id = ?", series)
				testDbSeries, err := database.GetDbserie(database.Query{Select: "identifiedby", Where: "id = ?", WhereArgs: []interface{}{dbserie}})
				if err != nil {

					continue
				}

				for _, epi := range importfeed.GetEpisodeArray(testDbSeries.Identifiedby, m.Identifier) {
					var seriesEpisodeErr error
					epi = strings.Trim(epi, "-EX")
					if strings.ToLower(testDbSeries.Identifiedby) != "date" {
						epi = strings.TrimLeft(epi, "0")
					}
					if epi == "" {
						continue
					}
					logger.Log.Info("Episode Identifier: ", epi)

					seriesEpisode := database.SerieEpisode{}
					if strings.ToLower(testDbSeries.Identifiedby) == "date" {
						seriesEpisode, seriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.serie_id, Serie_episodes.ID, Serie_episodes.dbserie_episode_id, Serie_episodes.dbserie_id", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{series, strings.Replace(epi, ".", "-", -1)}})
					} else {
						seriesEpisode, seriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.serie_id, Serie_episodes.ID, Serie_episodes.dbserie_episode_id, Serie_episodes.dbserie_id", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Season = ? AND DbSerie_episodes.Episode = ?", WhereArgs: []interface{}{series, m.Season, epi}})
						if seriesEpisodeErr != nil {
							seriesEpisode, seriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.serie_id, Serie_episodes.ID, Serie_episodes.dbserie_episode_id, Serie_episodes.dbserie_id", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ? COLLATE NOCASE", WhereArgs: []interface{}{series, m.Identifier}})
						}
					}
					if seriesEpisodeErr == nil {
						counter, err = database.CountRowsStatic("Select count(id) from serie_episode_files where location = ? and serie_episode_id = ?", file, seriesEpisode.ID)
						if counter == 0 && err == nil {
							if seriesEpisode.DbserieID == 0 {
								logger.Log.Warn("Failed parse match sub1: ", file, " as ", m.Title)
								continue
							}

							rootpath, _ := database.QueryColumnString("Select rootpath from series where id = ?", series)
							if rootpath == "" && series != 0 {
								//updateRootpath(file, "series", series.ID, &configEntry.Data)
							}

							logger.Log.Info("Parsed and add: ", file, " as ", m.Title)

						} else {
							logger.Log.Info("Already Parsed: ", file)
						}
					} else {
						logger.Log.Debug("SerieEpisode not matched loop: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio, " Season ", m.Season, " Epi ", epi)
						logger.Log.Infoln("SerieEpisode not matched loop: ", file)
					}
				}
			} else {
				logger.Log.Debug("SerieEpisode not matched: ", file, " as Title: ", m.Title, " TitleYear:  ", titlebuilder.String(), " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
				logger.Log.Infoln("SerieEpisode not matched: ", file)
			}
		}
		//utils.JobImportSeriesParseV2("T:\\x\\_sites\\eurogirlsongirls\\eurogirlsongirls 21-02-08 blondie fesser and julia de lucia 1080P WEBRIP.mp4", false, "serie_X", "X", parser.NewDefaultPrio("serie_X", "SDSeriesX"))

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

func BenchmarkQuery13(b *testing.B) {
	Init()
	set := logger.NewStringSetMaxSize(120000)
	for i := 0; i < 120000; i++ {
		set.Add(strconv.Itoa(i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rand.Seed(time.Now().UnixNano())
		n := rand.Intn(120000)
		setmin := logger.NewStringSetMaxSize(1)
		setmin.Add(strconv.Itoa(n))
		//set.Difference(setmin)
		//set.Difference2(setmin)
		set.Difference3(setmin)
	}
}

func BenchmarkQuery14(b *testing.B) {
	Init()
	cfgFolder := "Y:\\completed\\Movies\\Morbius.2022.1080p.WEB-DL.x264.AAC-EVO. (tt5108870)"
	cfgDisableruntimecheck := true
	cfgDisabledisallowed := false
	cfgDisabledeletewronglanguage := false
	cfgGrouptype := "movie"
	getconfig := "movie_EN"
	for i := 0; i < b.N; i++ {
		structure.StructureSingleFolder(cfgFolder, cfgDisableruntimecheck, cfgDisabledisallowed, cfgDisabledeletewronglanguage, cfgGrouptype, "path_"+"en movies", "path_"+"en movies import", getconfig)
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
	new = nil
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
	s = nil
}

package utils

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/remeh/sizedwaitgroup"
)

func jobImportMovieParseV2(file string, configTemplate string, listConfig string, updatemissing bool, minPrio parser.ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	movie, movieerr := database.GetMovies(database.Query{Where: "id in (Select movie_id from movie_files where location = ?) and listname = ?", WhereArgs: []interface{}{file, listConfig}})
	if movieerr == nil {
		for idxignore := range list.Ignore_map_lists {
			countermi, _ := database.CountRowsStatic("Select count(id) from movies where dbmovie_id = ? and listname = ?", movie.DbmovieID, list.Ignore_map_lists[idxignore])
			//countermi, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{movie.DbmovieID, list.Ignore_map_lists[idxignore]}})
			if countermi >= 1 {
				return
			}
		}
	}

	counterparse, _ := database.CountRowsStatic("Select count(id) from movie_file_unmatcheds where filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", file, listConfig, time.Now().Add(time.Hour*-12))
	//parsetest, _ := database.QueryMovieFileUnmatched(database.Query{Where: "filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", WhereArgs: []interface{}{file, list.Name, time.Now().Add(time.Hour * -12)}})
	if counterparse >= 1 {
		return
	}
	logger.Log.Debug("Parse Movie: ", file)

	m, err := parser.NewFileParser(filepath.Base(file), false, "movie")
	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return
	}
	cfg_quality := config.ConfigGet("quality_" + list.Template_quality).Data.(config.QualityConfig)

	addunmatched := false
	if err == nil {
		m.Title = strings.Trim(m.Title, " ")
		m.StripTitlePrefixPostfix(list.Template_quality)
		m.Resolution = strings.ToLower(m.Resolution)
		m.Audio = strings.ToLower(m.Audio)
		m.Codec = strings.ToLower(m.Codec)
		m.Quality = strings.ToLower(m.Quality)
		logger.Log.Debug("Parsed Movie: ", file, " as ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)

		entriesfound := 0
		if entriesfound == 0 && len(m.Imdb) >= 1 {
			movies, _ := database.QueryMovies(database.Query{Select: "id, dbmovie_id, rootpath", Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ? COLLATE NOCASE) and listname = ?", WhereArgs: []interface{}{m.Imdb, listConfig}})
			entriesfound = len(movies)
			if len(movies) == 1 {
				movie = movies[0]
			}

		}
		if entriesfound == 0 && len(m.Imdb) >= 1 {
			movies, _ := database.QueryMovies(database.Query{Select: "id, dbmovie_id, rootpath", Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ? COLLATE NOCASE)", WhereArgs: []interface{}{m.Imdb}})
			if len(movies) >= 1 {

				return
			}
		}
		if entriesfound == 0 {
			getmovie, imdb, entriesfound := importfeed.MovieFindDbByTitle(m.Title, strconv.Itoa(m.Year), listConfig, cfg_quality.CheckYear1, "parse")
			if entriesfound >= 1 {
				m.Imdb = imdb
				movie = getmovie
			}
		}

		if movie.ID == 0 {
			if list.Addfound {
				configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
				if len(m.Imdb) >= 1 {
					sww := sizedwaitgroup.New(1)
					var dbmovie database.Dbmovie
					dbmovie.ImdbID = m.Imdb
					sww.Add()
					importfeed.JobImportMovies(dbmovie, configTemplate, listConfig, &sww)
					sww.Wait()
					movies, _ := database.QueryMovies(database.Query{Select: "id, dbmovie_id, rootpath", Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ? COLLATE NOCASE) and listname = ?", WhereArgs: []interface{}{m.Imdb, listConfig}})
					if len(movies) == 1 {
						movie = movies[0]
					}

				} else {
					lists := make([]string, 0, len(configEntry.Lists))
					for idxlisttest := range configEntry.Lists {
						lists = append(lists, configEntry.Lists[idxlisttest].Name)
					}
					getlist, _, entriesfound, dbmovie := importfeed.MovieFindListByTitle(m.Title, strconv.Itoa(m.Year), lists, "rss")

					if entriesfound >= 1 && getlist == listConfig {
						getmovie, getmovieerr := database.GetMovies(database.Query{Select: "id, dbmovie_id, rootpath", Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, getlist}})
						if getmovieerr == nil {
							movie = getmovie
						}
					} else if entriesfound == 0 && listConfig == configEntry.Data[0].AddFoundList && configEntry.Data[0].AddFound {
						//Search imdb!
						_, imdbget, imdbfound := importfeed.MovieFindDbByTitle(m.Title, strconv.Itoa(m.Year), listConfig, true, "rss")
						if imdbfound == 0 {
							sww := sizedwaitgroup.New(1)
							var dbmovie database.Dbmovie
							dbmovie.ImdbID = imdbget
							sww.Add()
							importfeed.JobImportMovies(dbmovie, configTemplate, listConfig, &sww)
							sww.Wait()
							movies, _ := database.QueryMovies(database.Query{Select: "id, dbmovie_id, rootpath", Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ? COLLATE NOCASE) and listname = ?", WhereArgs: []interface{}{imdbget, listConfig}})
							if len(movies) == 1 {
								movie = movies[0]
							}

						}
					}
				}
			}
		}
		if movie.ID >= 1 {

			m.GetPriority(configTemplate, list.Template_quality)
			errparsev := m.ParseVideoFile(file, configTemplate, list.Template_quality)
			if errparsev != nil {
				return
			}
			counterf, _ := database.CountRowsStatic("Select count(id) from movie_files where location=? and movie_id = ?", file, movie.ID)
			//counterf, _ := database.CountRows("movie_files", database.Query{Where: "location = ? AND movie_id = ?", WhereArgs: []interface{}{file, movie.ID}})
			if counterf == 0 {
				reached := false
				if m.Priority >= parser.NewCutoffPrio(configTemplate, list.Template_quality).Priority {
					reached = true
				}

				if movie.Rootpath == "" && movie.ID != 0 {
					rootpath := ""
					configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
					for idxpath := range configEntry.Data {
						if !config.ConfigCheck("path_" + configEntry.Data[idxpath].Template_path) {
							continue
						}
						cfg_path := config.ConfigGet("path_" + configEntry.Data[idxpath].Template_path).Data.(config.PathsConfig)

						pppath := cfg_path.Path
						if strings.Contains(file, pppath) {
							rootpath = pppath
							tempfoldername := strings.Replace(file, pppath, "", -1)
							tempfoldername = strings.TrimLeft(tempfoldername, "/\\")
							tempfoldername = filepath.Dir(tempfoldername)
							_, firstfolder := logger.Getrootpath(tempfoldername)
							rootpath = filepath.Join(rootpath, firstfolder)
							break
						}
					}
					database.UpdateColumn("movies", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
				}

				database.InsertArray("movie_files",
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id", "height", "width"},
					[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie.ID, movie.DbmovieID, m.Height, m.Width})
				if updatemissing {
					database.UpdateColumn("movies", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
					database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
				}

				database.DeleteRow("movie_file_unmatcheds", database.Query{Where: "filepath = ?", WhereArgs: []interface{}{file}})
			}
		} else {
			addunmatched = true
			logger.Log.Error("Movie Parse failed - not matched: ", file)
		}
	} else {
		addunmatched = true
		logger.Log.Error("Movie Parse failed: ", file)
	}

	if addunmatched {
		mjson, _ := json.Marshal(m)
		database.UpsertArray("movie_file_unmatcheds",
			[]string{"listname", "filepath", "last_checked", "parsed_data"},
			[]interface{}{listConfig, file, time.Now(), string(mjson)},
			database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, listConfig}})

	}
}

func getMissingIMDBMoviesV2(configTemplate string, listConfig string) []database.Dbmovie {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	if !list.Enabled {
		return []database.Dbmovie{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return []database.Dbmovie{}
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)

	if len(cfg_list.Url) >= 1 {
		filename := list.Template_list + ".csv"
		os.Remove(filepath.Join("./temp", filename))
		errdl := logger.DownloadFile("./temp", "", filename, cfg_list.Url)
		if errdl != nil {
			logger.Log.Error("Failed to read CSV from: ", cfg_list.Url)
			return []database.Dbmovie{}
		}
		imdbcsvfile, errfl := os.Open(filepath.Join("./temp", filename))
		defer func() {
			imdbcsvfile = nil
		}()
		if errfl != nil {
			logger.Log.Error("Failed to read CSV from: ", cfg_list.Url)
			return []database.Dbmovie{}
		}
		parserimdb := csv.NewReader(bufio.NewReader(imdbcsvfile))
		defer func() {
			parserimdb = nil
		}()
		_, _ = parserimdb.Read() //skip header
		var record []string
		var err error
		var d []database.Dbmovie
		for {
			record, err = parserimdb.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.Error(fmt.Errorf("an error occurred while parsing csv.. %v", err))
				continue
			}
			if !importfeed.AllowMovieImport(record[1], list.Template_list) {
				continue
			}
			year, _ := strconv.ParseInt(record[10], 0, 64)
			votes, _ := strconv.ParseInt(record[12], 0, 64)
			voteavg, _ := strconv.ParseFloat(record[8], 64)
			d = append(d, database.Dbmovie{ImdbID: record[1], Title: record[5], URL: record[6], VoteAverage: float32(voteavg), Year: int(year), VoteCount: int(votes)})
		}
		return d
	}
	return []database.Dbmovie{}
}

func getTraktUserPublicMovieList(configTemplate string, listConfig string) []database.Dbmovie {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	if !list.Enabled {
		return []database.Dbmovie{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return []database.Dbmovie{}
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)

	if len(cfg_list.TraktUsername) >= 1 && len(cfg_list.TraktListName) >= 1 {
		if len(cfg_list.TraktListType) == 0 {
			cfg_list.TraktListType = "movie,show"
		}
		data, err := apiexternal.TraktApi.GetUserList(cfg_list.TraktUsername, cfg_list.TraktListName, cfg_list.TraktListType, cfg_list.Limit)
		if err != nil {
			logger.Log.Error("Failed to read trakt list: ", cfg_list.TraktListName)
			return []database.Dbmovie{}
		}
		d := make([]database.Dbmovie, 0, len(data))

		for idx := range data {

			if !importfeed.AllowMovieImport(data[idx].Movie.Ids.Imdb, list.Template_list) {
				continue
			}
			year := data[idx].Movie.Year
			url := "https://www.imdb.com/title/" + data[idx].Movie.Ids.Imdb
			d = append(d, database.Dbmovie{ImdbID: data[idx].Movie.Ids.Imdb, Title: data[idx].Movie.Title, URL: url, Year: int(year)})
		}
		return d
	}
	return []database.Dbmovie{}
}

func Importnewmoviessingle(configTemplate string, listConfig string) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	logger.Log.Debug("get feeds for ", configTemplate, listConfig)
	results := feeds(configTemplate, listConfig)
	logger.Log.Debug("RESULT -get feeds for ", configTemplate, listConfig, len(results.Movies))

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerMetadata == 0 {
		cfg_general.WorkerMetadata = 1
	}

	swg := sizedwaitgroup.New(cfg_general.WorkerMetadata)
	dbmovies, _ := database.QueryStaticColumnsOneStringOneInt("Select imdb_id, id from dbmovies", "Select count(id) from dbmovies")
	for idxmovie := range results.Movies {
		founddbmovie := false
		foundmovie := false
		var id int
		for idxdbmovie := range dbmovies {
			if dbmovies[idxdbmovie].Str == results.Movies[idxmovie].ImdbID {
				id = dbmovies[idxdbmovie].Num
				founddbmovie = true
				break
			}
		}
		//dbmovie, dbmovieerr := database.GetDbmovie(database.Query{Select: "id", Where: "imdb_id=?", WhereArgs: []interface{}{results.Movies[idxmovie].ImdbID}})
		//if dbmovieerr == nil {
		//founddbmovie = true
		if founddbmovie {
			counter, _ := database.CountRowsStatic("Select count(id) from movies where dbmovie_id=? and listname=?", id, listConfig)
			//counter, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, list.Name}})
			if counter >= 1 {
				foundmovie = true
			}

			if len(list.Ignore_map_lists) >= 1 && !foundmovie {
				for idx := range list.Ignore_map_lists {
					counter, _ = database.CountRowsStatic("Select count(id) from movies where dbmovie_id=? and listname=?", id, list.Ignore_map_lists[idx])
					//counter, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, list.Ignore_map_lists[idx]}})
					if counter >= 1 {
						foundmovie = true
						break
					}
				}
			}
		}
		if !founddbmovie || !foundmovie {
			logger.Log.Info("Import Movie ", idxmovie, " of ", len(results.Movies), " imdb: ", results.Movies[idxmovie].ImdbID)
			swg.Add()
			go func(missing database.Dbmovie) {
				importfeed.JobImportMovies(missing, configTemplate, listConfig, &swg)
			}(results.Movies[idxmovie])
		}
	}
	swg.Wait()
}

func Getnewmovies(configTemplate string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	logger.Log.Info("Scan Movie File")
	filesfound := findFiles(configTemplate)

	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	for _, list := range row.Lists {
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			continue
		}
		added := scanner.GetFilesAdded(filesfound, list.Name, configTemplate, list.Name)
		logger.Log.Info("Find Movie File")
		for idxfile := range added {
			logger.Log.Info("Parse Movie ", idxfile, " path: ", added[idxfile])
			swf.Add()
			go func(file string) {
				jobImportMovieParseV2(file, configTemplate, list.Name, true, parser.NewDefaultPrio(configTemplate, list.Template_quality), &swf)
			}(added[idxfile])
		}
	}
	swf.Wait()
}
func getnewmoviessingle(configTemplate string, listConfig string) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return
	}

	logger.Log.Info("Scan Movie File")
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)

	added := scanner.GetFilesAdded(findFiles(configTemplate), configTemplate, listConfig, listConfig)
	logger.Log.Info("Find Movie File")
	for idxfile := range added {
		logger.Log.Info("Parse Movie ", idxfile, " path: ", added[idxfile])
		swf.Add()
		go func(file string) {
			jobImportMovieParseV2(file, configTemplate, listConfig, true, parser.NewDefaultPrio(configTemplate, list.Template_quality), &swf)
		}(added[idxfile])
	}
	swf.Wait()
}

func checkmissingmoviessingle(configTemplate string, listConfig string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)
	//moviefile, _ := database.QueryMovieFiles(database.Query{Select: "location", Where: "movie_id in (Select id from movies where listname = ?)", WhereArgs: []interface{}{list.Name}})
	for _, filerow := range database.QueryStaticColumnsOneStringNoError("Select location from movie_files where movie_id in (Select id from movies where listname = ?)", "Select count(id) from movie_files where movie_id in (Select id from movies where listname = ?)", listConfig) {
		swfile.Add()
		go func(file string) {
			jobImportFileCheck(file, "movie", &swfile)
		}(filerow.Str)
	}
	swfile.Wait()
}

func checkmissingmoviesflag(configTemplate string, listConfig string) {
	movies, _ := database.QueryMovies(database.Query{Select: "id, missing", Where: "listname = ?", WhereArgs: []interface{}{listConfig}})

	for idxmovie := range movies {
		counter, _ := database.CountRowsStatic("Select count(id) from movie_files where movie_id=?", movies[idxmovie].ID)
		//counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies[idxmovie].ID}})
		if counter >= 1 {
			if movies[idxmovie].Missing {
				database.UpdateColumn("Movies", "missing", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idxmovie].ID}})
			}
		} else {
			if !movies[idxmovie].Missing {
				database.UpdateColumn("Movies", "missing", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idxmovie].ID}})
			}
		}
	}

}

func checkreachedmoviesflag(configTemplate string, listConfig string) {
	movies, _ := database.QueryMovies(database.Query{Select: "id, quality_reached, quality_profile", Where: "listname=?", WhereArgs: []interface{}{listConfig}})
	for idxepi := range movies {
		if !config.ConfigCheck("quality_" + movies[idxepi].QualityProfile) {
			continue
		}

		reached := false
		if parser.GetHighestMoviePriorityByFiles(movies[idxepi], configTemplate, movies[idxepi].QualityProfile) >= parser.NewCutoffPrio(configTemplate, movies[idxepi].QualityProfile).Priority {
			reached = true
		}
		if movies[idxepi].QualityReached && !reached {
			database.UpdateColumn("movies", "quality_reached", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idxepi].ID}})
		}

		if !movies[idxepi].QualityReached && reached {
			database.UpdateColumn("movies", "quality_reached", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{movies[idxepi].ID}})
		}
	}

}

func moviesStructureSingle(configTemplate string, listConfig string) {

	if !config.ConfigCheck("general") {
		logger.Log.Debug("General not found")
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	//swfile := sizedwaitgroup.New(1)

	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	for idxpath := range row.DataImport {
		mappath := ""
		if !config.ConfigCheck("path_" + row.DataImport[idxpath].Template_path) {
			logger.Log.Debug("Path not found: ", "path_"+row.DataImport[idxpath].Template_path)
			continue
		}
		cfg_path_import := config.ConfigGet("path_" + row.DataImport[idxpath].Template_path).Data.(config.PathsConfig)

		var cfg_path config.PathsConfig
		if len(row.Data) >= 1 {
			mappath = row.Data[0].Template_path
			if !config.ConfigCheck("path_" + mappath) {
				logger.Log.Debug("Path not found: ", "path_"+mappath)
				continue
			}
			cfg_path = config.ConfigGet("path_" + mappath).Data.(config.PathsConfig)

		} else {
			logger.Log.Debug("No Path not found")
			continue
		}
		//swfile.Add()
		go func(source config.PathsConfig, target config.PathsConfig) {
			structure.StructureFolders("movie", source, target, configTemplate, listConfig)
			//swfile.Done()
		}(cfg_path_import, cfg_path)
	}
	//swfile.Wait()
}

func RefreshMovies() {

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	if cfg_general.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg_general.WorkerFiles)
	dbmovies, _ := database.QueryDbmovie(database.Query{})

	for idxmovie := range dbmovies {
		logger.Log.Info("Refresh Movie ", idxmovie, " of ", len(dbmovies), " imdb: ", dbmovies[idxmovie].ImdbID)
		sw.Add()
		go func(movie database.Dbmovie) {
			importfeed.JobReloadMovies(movie, "", "", &sw)
		}(dbmovies[idxmovie])
	}
	sw.Wait()
}

func RefreshMovie(id string) {
	dbmovies, _ := database.QueryDbmovie(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})

	sw := sizedwaitgroup.New(1)
	for idxmovie := range dbmovies {
		logger.Log.Info("Refresh Movie ", idxmovie, " of ", len(dbmovies), " imdb: ", dbmovies[idxmovie].ImdbID)
		sw.Add()
		go func(movie database.Dbmovie) {
			importfeed.JobReloadMovies(movie, "", "", &sw)
		}(dbmovies[idxmovie])
	}
	sw.Wait()
}

func RefreshMoviesInc() {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	if cfg_general.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg_general.WorkerFiles)
	dbmovies, _ := database.QueryDbmovie(database.Query{Limit: 100, OrderBy: "updated_at desc"})

	for idxmovie := range dbmovies {
		sw.Add()
		go func(movie database.Dbmovie) {
			importfeed.JobReloadMovies(movie, "", "", &sw)
		}(dbmovies[idxmovie])
	}
	sw.Wait()
}

func Movies_all_jobs(job string, force bool) {

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		if !config.ConfigCheck(idxmovie.Name) {
			continue
		}
		Movies_single_jobs(job, idxmovie.Name, "", force)
	}
}

func Movies_single_jobs(job string, configTemplate string, listname string, force bool) {
	jobName := job + "_movies"
	if configTemplate != "" {
		jobName += "_" + configTemplate
	}
	if listname != "" {
		jobName += "_" + listname
	}
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", configTemplate)
		return
	}

	job = strings.ToLower(job)
	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, configTemplate, "Movie", time.Now()})
	logger.Log.Info("Started Job: ", jobName)
	if config.ConfigCheck(configTemplate) {
		cfg_movie := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)

		if cfg_movie.Searchmissing_incremental == 0 {
			cfg_movie.Searchmissing_incremental = 20
		}
		if cfg_movie.Searchupgrade_incremental == 0 {
			cfg_movie.Searchupgrade_incremental = 20
		}

		if job == "rss" || job == "searchmissingfull" || job == "searchmissinginc" ||
			job == "searchupgradefull" || job == "searchupgradeinc" || job == "searchmissingfulltitle" ||
			job == "searchmissinginctitle" || job == "searchupgradefulltitle" || job == "searchupgradeinctitle" {
			for _, val := range apiexternal.NewznabClients {
				val.Client.Client.CloseIdleConnections()
			}
		}
		switch job {
		case "datafull":
			Getnewmovies(configTemplate)
		case "searchmissingfull":
			searcher.SearchMovieMissing(configTemplate, 0, false)
		case "searchmissinginc":
			searcher.SearchMovieMissing(configTemplate, cfg_movie.Searchmissing_incremental, false)
		case "searchupgradefull":
			searcher.SearchMovieUpgrade(configTemplate, 0, false)
		case "searchupgradeinc":
			searcher.SearchMovieUpgrade(configTemplate, cfg_movie.Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			searcher.SearchMovieMissing(configTemplate, 0, true)
		case "searchmissinginctitle":
			searcher.SearchMovieMissing(configTemplate, cfg_movie.Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			searcher.SearchMovieUpgrade(configTemplate, 0, true)
		case "searchupgradeinctitle":
			searcher.SearchMovieUpgrade(configTemplate, cfg_movie.Searchupgrade_incremental, true)
		}
		if listname != "" {
			logger.Log.Debug("Listname: ", listname)
			var templists []config.MediaListsConfig
			for idxlist := range cfg_movie.Lists {
				if cfg_movie.Lists[idxlist].Name == listname {
					templists = append(templists, cfg_movie.Lists[idxlist])
				}
			}
			logger.Log.Debug("Listname: found: ", templists)
			cfg_movie.Lists = templists
		}
		qualis := make(map[string]bool, 10)
		for idxlist := range cfg_movie.Lists {
			if _, ok := qualis[cfg_movie.Lists[idxlist].Template_quality]; !ok {
				qualis[cfg_movie.Lists[idxlist].Template_quality] = true
			}
			switch job {
			case "data":
				getnewmoviessingle(configTemplate, cfg_movie.Lists[idxlist].Name)
			case "checkmissing":
				checkmissingmoviessingle(configTemplate, cfg_movie.Lists[idxlist].Name)
			case "checkmissingflag":
				checkmissingmoviesflag(configTemplate, cfg_movie.Lists[idxlist].Name)
			case "checkreachedflag":
				checkreachedmoviesflag(configTemplate, cfg_movie.Lists[idxlist].Name)
			case "structure":
				moviesStructureSingle(configTemplate, cfg_movie.Lists[idxlist].Name)
			case "clearhistory":
				database.DeleteRow("movie_histories", database.Query{Where: "movie_id in (Select id from movies where listname=?)", WhereArgs: []interface{}{cfg_movie.Lists[idxlist].Name}})
			case "feeds":
				Importnewmoviessingle(configTemplate, cfg_movie.Lists[idxlist].Name)
			default:
				// other stuff
			}
		}
		for qual := range qualis {
			switch job {
			case "rss":
				searcher.SearchMovieRSS(configTemplate, qual)
			}
		}
		for key := range qualis {
			delete(qualis, key)
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", configTemplate)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", jobName)
	debug.FreeOSMemory()
}

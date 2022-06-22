package utils

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
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
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/shomali11/parallelizer"
)

func jobImportMovieParseV2(file string, configTemplate string, listConfig string, updatemissing bool) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if list.Name == "" {
		return
	}
	if ok := checkignorelistsonpath(configTemplate, file, listConfig); !ok {
		return
	}

	if ok := checkunmatched(configTemplate, file, listConfig); !ok {
		return
	}
	var counter int

	logger.Log.Debug("Parse Movie: ", file)

	m, err := parser.NewFileParser(filepath.Base(file), false, "movie")

	addunmatched := false
	if err == nil {
		defer m.Close()
		if !config.ConfigCheck("quality_" + list.Template_quality) {

			logger.Log.Error("Quality for List: " + list.Name + " not found")
			return
		}
		m.Title = strings.Trim(m.Title, " ")
		parser.StripTitlePrefixPostfix(&m, list.Template_quality)
		m.Resolution = strings.ToLower(m.Resolution)
		m.Audio = strings.ToLower(m.Audio)
		m.Codec = strings.ToLower(m.Codec)
		m.Quality = strings.ToLower(m.Quality)
		logger.Log.Debug("Parsed Movie: ", file, " as ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)

		movie, imdb, _, err := m.FindMovieByFile([]string{listConfig})
		defer logger.ClearVar(&movie)
		if err != nil {
			if list.Addfound {
				movie, err = m.AddMovieIfNotFound(listConfig, configTemplate)
				if err != nil {
					return
				}
				m.Imdb = imdb
			}
		}
		if movie >= 1 {

			m.GetPriority(configTemplate, list.Template_quality)
			errparsev := m.ParseVideoFile(file, configTemplate, list.Template_quality)
			if errparsev != nil {

				return
			}
			counter, err = database.CountRowsStatic("Select count(id) from movie_files where location = ? and movie_id = ?", file, movie)
			if counter == 0 && err == nil {
				reached := false
				if m.Priority >= parser.NewCutoffPrio(configTemplate, list.Template_quality).Priority {
					reached = true
				}
				rootpath, _ := database.QueryColumnString("Select rootpath from movies where id = ?", movie)
				dbmovie, _ := database.QueryColumnUint("Select dbmovie_id from movies where id = ?", movie)

				if rootpath == "" && movie != 0 {
					updateRootpath(file, "movies", movie, configTemplate)
				}

				database.InsertArray("movie_files",
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id", "height", "width"},
					[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie, dbmovie, m.Height, m.Width})
				if updatemissing {
					database.UpdateColumn("movies", "missing", false, database.Query{Where: "id = ?", WhereArgs: []interface{}{movie}})
					database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id = ?", WhereArgs: []interface{}{movie}})
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
		mjson, mjsonerr := json.Marshal(m)
		defer logger.ClearVar(&mjson)
		if mjsonerr == nil {
			database.UpsertArray("movie_file_unmatcheds",
				[]string{"listname", "filepath", "last_checked", "parsed_data"},
				[]interface{}{listConfig, file, time.Now(), string(mjson)},
				database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, listConfig}})
		}
	}

}

func getMissingIMDBMoviesV2(configTemplate string, listConfig string) ([]database.Dbmovie, error) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	if !list.Enabled {
		return []database.Dbmovie{}, errors.New("list not enabled")
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return []database.Dbmovie{}, errors.New("list not found")
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)

	if len(cfg_list.Url) >= 1 {
		resp, err := logger.GetUrlResponse(cfg_list.Url)
		if err != nil {
			logger.Log.Error("Failed to read CSV from: ", cfg_list.Url)
			return []database.Dbmovie{}, errors.New("list read fail")
		}

		defer resp.Body.Close()
		defer logger.ClearVar(resp)

		dbmovies, _ := database.QueryStaticColumnsOneStringOneInt("Select imdb_id, id from dbmovies", "Select count(id) from dbmovies")
		movies, _ := database.QueryStaticColumnsOneInt("Select dbmovie_id from movies where listname = ?", "Select count(id) from movies where listname = ?", listConfig)
		defer logger.ClearVar(&movies)
		defer logger.ClearVar(&dbmovies)

		parserimdb := csv.NewReader(bufio.NewReader(resp.Body))
		defer logger.ClearVar(parserimdb)
		parserimdb.ReuseRecord = true
		var d []database.Dbmovie
		defer logger.ClearVar(&d)
		_, _ = parserimdb.Read() //skip header

		for {
			record, err := parserimdb.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.Errorln("an error occurred while parsing csv.. ", err)
				continue
			}
			dbmovieid := 0
			for idx := range dbmovies {
				if dbmovies[idx].Str == record[1] {
					dbmovieid = dbmovies[idx].Num
					break
				}
			}
			moviefound := false
			if dbmovieid != 0 {
				for idx := range movies {
					if movies[idx].Num == dbmovieid {
						moviefound = true
						break
					}
				}
			}
			if moviefound {
				continue
			}
			if !importfeed.AllowMovieImport(record[1], list.Template_list) {
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
		return d, nil
	}
	return []database.Dbmovie{}, errors.New("list other error")
}

func getTraktUserPublicMovieList(configTemplate string, listConfig string) ([]database.Dbmovie, error) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)

	if !list.Enabled {
		return nil, errors.New("list not enabled")
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return nil, errors.New("list not found")
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)

	if len(cfg_list.TraktUsername) >= 1 && len(cfg_list.TraktListName) >= 1 {
		if len(cfg_list.TraktListType) == 0 {
			cfg_list.TraktListType = "movie,show"
		}
		data, err := apiexternal.TraktApi.GetUserList(cfg_list.TraktUsername, cfg_list.TraktListName, cfg_list.TraktListType, cfg_list.Limit)
		if err != nil {
			logger.Log.Error("Failed to read trakt list: ", cfg_list.TraktListName)
			return nil, errors.New("list not readable")
		}
		defer logger.ClearVar(&data)
		d := make([]database.Dbmovie, 0, len(data))
		defer logger.ClearVar(&d)
		for idx := range data {

			if !importfeed.AllowMovieImport(data[idx].Movie.Ids.Imdb, list.Template_list) {
				continue
			}
			year := data[idx].Movie.Year
			url := "https://www.imdb.com/title/" + data[idx].Movie.Ids.Imdb
			d = append(d, database.Dbmovie{ImdbID: data[idx].Movie.Ids.Imdb, Title: data[idx].Movie.Title, URL: url, Year: int(year)})
		}
		return d, nil
	}
	return nil, errors.New("list other error")
}

func Importnewmoviessingle(configTemplate string, listConfig string) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if list.Name == "" {
		return
	}

	logger.Log.Debug("get feeds for ", configTemplate, listConfig)

	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerMetadata == 0 {
		cfg_general.WorkerMetadata = 1
	}

	feed, err := feeds(configTemplate, listConfig)
	if err != nil {
		return
	}
	defer logger.ClearVar(&feed)
	dbmovies, _ := database.QueryStaticColumnsOneStringOneInt("Select imdb_id, id from dbmovies", "Select count(id) from dbmovies")

	movies, _ := database.QueryStaticColumnsOneStringOneInt("Select listname, dbmovie_id from movies", "Select count(id) from movies")

	defer logger.ClearVar(&dbmovies)
	defer logger.ClearVar(&movies)
	swg := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerMetadata))
	var foundmovie, founddbmovie bool
	var id int
	var imdbID string
	for idxmovie := range feed.Movies {
		imdbID = feed.Movies[idxmovie].ImdbID
		founddbmovie = false
		foundmovie = false
		id = 0
		for idxdbmovie := range dbmovies {
			if dbmovies[idxdbmovie].Str == imdbID {
				id = dbmovies[idxdbmovie].Num
				founddbmovie = true
				break
			}
		}
		if founddbmovie {
			for idxmovie2 := range movies {
				if movies[idxmovie2].Num == id && movies[idxmovie2].Str == listConfig {
					foundmovie = true
					break
				}
			}
			if len(list.Ignore_map_lists) >= 1 && !foundmovie {
				for idx := range list.Ignore_map_lists {
					for idxmovie2 := range movies {
						if movies[idxmovie2].Num == id && movies[idxmovie2].Str == list.Ignore_map_lists[idx] {
							foundmovie = true
							break
						}
					}
					if foundmovie {
						break
					}
				}
			}
		}
		if !founddbmovie || !foundmovie {
			logger.Log.Info("Import Movie ", idxmovie, " imdb: ", imdbID)
			swg.Add(func() {
				importfeed.JobImportMovies(imdbID, configTemplate, listConfig)
			})
		}
	}
	swg.Wait()
	swg.Close()
}

func checkmissingmoviessingle(configTemplate string, listConfig string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	filesfound := database.QueryStaticColumnsOneStringNoError("Select location from movie_files where movie_id in (Select id from movies where listname = ?)", "Select count(id) from movie_files where movie_id in (Select id from movies where listname = ?)", listConfig)
	defer logger.ClearVar(&filesfound)
	swf := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerFiles))

	var str string
	for idx := range filesfound {
		str = filesfound[idx].Str
		swf.Add(func() {
			jobImportFileCheck(str, "movie")
		})
	}
	swf.Wait()
	swf.Close()
}

func checkmissingmoviesflag(configTemplate string, listConfig string) {
	movies, _ := database.QueryMovies(database.Query{Select: "id, missing", Where: "listname = ?", WhereArgs: []interface{}{listConfig}})
	defer logger.ClearVar(&movies)

	for idxmovie := range movies {
		counter, _ := database.CountRowsStatic("Select count(id) from movie_files where movie_id = ?", movies[idxmovie].ID)
		if counter >= 1 {
			if movies[idxmovie].Missing {
				database.UpdateColumn("Movies", "missing", 0, database.Query{Where: "id = ?", WhereArgs: []interface{}{movies[idxmovie].ID}})
			}
		} else {
			if !movies[idxmovie].Missing {
				database.UpdateColumn("Movies", "missing", 1, database.Query{Where: "id = ?", WhereArgs: []interface{}{movies[idxmovie].ID}})
			}
		}
	}
}

func checkreachedmoviesflag(configTemplate string, listConfig string) {
	movies, _ := database.QueryMovies(database.Query{Select: "id, quality_reached, quality_profile", Where: "listname = ?", WhereArgs: []interface{}{listConfig}})
	defer logger.ClearVar(&movies)
	for idxepi := range movies {
		if !config.ConfigCheck("quality_" + movies[idxepi].QualityProfile) {
			logger.Log.Error("Quality for Movie: " + strconv.Itoa(int(movies[idxepi].ID)) + " not found")
			continue
		}

		reached := false
		if parser.GetHighestMoviePriorityByFiles(movies[idxepi].ID, configTemplate, movies[idxepi].QualityProfile) >= parser.NewCutoffPrio(configTemplate, movies[idxepi].QualityProfile).Priority {
			reached = true
		}
		if movies[idxepi].QualityReached && !reached {
			database.UpdateColumn("movies", "quality_reached", 0, database.Query{Where: "id = ?", WhereArgs: []interface{}{movies[idxepi].ID}})
		}

		if !movies[idxepi].QualityReached && reached {
			database.UpdateColumn("movies", "quality_reached", 1, database.Query{Where: "id = ?", WhereArgs: []interface{}{movies[idxepi].ID}})
		}
	}
}

func moviesStructureSingle(configTemplate string) {

	if !config.ConfigCheck("general") {
		logger.Log.Errorln("General not found")
		return
	}

	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	for idxpath := range row.DataImport {
		mappath := ""
		if !config.ConfigCheck("path_" + row.DataImport[idxpath].Template_path) {
			logger.Log.Errorln("Path not found: ", "path_"+row.DataImport[idxpath].Template_path)
			continue
		}

		if len(row.Data) >= 1 {
			mappath = row.Data[0].Template_path
			if !config.ConfigCheck("path_" + mappath) {
				logger.Log.Errorln("Path not found: ", "path_"+mappath)
				continue
			}
		} else {
			logger.Log.Debug("No Path not found")
			continue
		}
		structure.StructureFolders("movie", "path_"+row.DataImport[idxpath].Template_path, "path_"+mappath, configTemplate)
	}
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
	dbmovies, _ := database.QueryStaticColumnsOneString("Select imdb_id from dbmovies", "Select count(id) from dbmovies")
	defer logger.ClearVar(&dbmovies)
	for idxmovie, val := range dbmovies {
		logger.Log.Info("Refresh Movie ", idxmovie, " of ", len(dbmovies), " imdb: ", val.Str)
		importfeed.JobReloadMovies(val.Str, "", "")
	}
}

func RefreshMovie(id string) {
	dbmovies, _ := database.QueryStaticColumnsOneString("Select imdb_id from dbmovies where id = ?", "", id)
	defer logger.ClearVar(&dbmovies)
	for idxmovie, val := range dbmovies {
		logger.Log.Info("Refresh Movie ", idxmovie, " of ", len(dbmovies), " imdb: ", val.Str)
		importfeed.JobReloadMovies(val.Str, "", "")
	}
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
	dbmovies, _ := database.QueryStaticColumnsOneString("Select imdb_id from dbmovies order by updated_at desc limit 100", "Select count(id) from dbmovies order by updated_at desc limit 100")
	defer logger.ClearVar(&dbmovies)
	for idxmovie, val := range dbmovies {
		logger.Log.Info("Refresh Movie ", idxmovie, " of ", len(dbmovies), " imdb: ", val.Str)
		importfeed.JobReloadMovies(val.Str, "", "")
	}
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

		switch job {
		case "datafull":
			func() { GetNewFilesMap(configTemplate, "movie", "") }()
		case "searchmissingfull":
			func() { searcher.SearchMovieMissing(configTemplate, 0, false) }()
		case "searchmissinginc":
			func() { searcher.SearchMovieMissing(configTemplate, cfg_movie.Searchmissing_incremental, false) }()
		case "searchupgradefull":
			func() { searcher.SearchMovieUpgrade(configTemplate, 0, false) }()
		case "searchupgradeinc":
			func() { searcher.SearchMovieUpgrade(configTemplate, cfg_movie.Searchupgrade_incremental, false) }()
		case "searchmissingfulltitle":
			func() { searcher.SearchMovieMissing(configTemplate, 0, true) }()
		case "searchmissinginctitle":
			func() { searcher.SearchMovieMissing(configTemplate, cfg_movie.Searchmissing_incremental, true) }()
		case "searchupgradefulltitle":
			func() { searcher.SearchMovieUpgrade(configTemplate, 0, true) }()
		case "searchupgradeinctitle":
			func() { searcher.SearchMovieUpgrade(configTemplate, cfg_movie.Searchupgrade_incremental, true) }()
		case "structure":
			func() { moviesStructureSingle(configTemplate) }()
		}
		if listname != "" {
			cfg_movie.Lists = config.MedialistConfigFilterByListName(cfg_movie.Lists, listname)
		}
		qualis := make([]string, 0, len(cfg_movie.Lists))
		defer logger.ClearVar(&qualis)
		for idxlist := range cfg_movie.Lists {
			foundentry := false
			for idx2 := range qualis {
				if qualis[idx2] == cfg_movie.Lists[idxlist].Template_quality {
					foundentry = true
					break
				}
			}
			if !foundentry {
				qualis = append(qualis, cfg_movie.Lists[idxlist].Template_quality)
			}
			switch job {
			case "data":
				func() { GetNewFilesMap(configTemplate, "movie", cfg_movie.Lists[idxlist].Name) }()
			case "checkmissing":
				func() { checkmissingmoviessingle(configTemplate, cfg_movie.Lists[idxlist].Name) }()
			case "checkmissingflag":
				func() { checkmissingmoviesflag(configTemplate, cfg_movie.Lists[idxlist].Name) }()
			case "checkreachedflag":
				func() { checkreachedmoviesflag(configTemplate, cfg_movie.Lists[idxlist].Name) }()
			case "clearhistory":
				func() {
					database.DeleteRow("movie_histories", database.Query{Where: "movie_id in (Select id from movies where listname = ?)", WhereArgs: []interface{}{cfg_movie.Lists[idxlist].Name}})
				}()
			case "feeds":
				func() { Importnewmoviessingle(configTemplate, cfg_movie.Lists[idxlist].Name) }()
			default:
				// other stuff
			}
		}
		for idx := range qualis {
			switch job {
			case "rss":
				func() { searcher.SearchMovieRSS(configTemplate, qualis[idx]) }()
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", configTemplate)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", jobName)
	debug.FreeOSMemory()
}

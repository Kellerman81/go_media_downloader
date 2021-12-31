package utils

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
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

func JobImportMovieParseV2(file string, configEntry config.MediaTypeConfig, list config.MediaListsConfig, updatemissing bool, minPrio parser.ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	movie, movieerr := database.GetMovies(database.Query{Select: "movies.*", InnerJoin: "movie_files ON Movies.ID = movie_files.movie_id", Where: "movie_files.location = ? and movies.listname = ?", WhereArgs: []interface{}{file, list.Name}})
	if movieerr == nil {
		for idxignore := range list.Ignore_map_lists {
			countermi, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{movie.DbmovieID, list.Ignore_map_lists[idxignore]}})
			if countermi >= 1 {
				return
			}
		}
	}

	parsetest, _ := database.QueryMovieFileUnmatched(database.Query{Where: "filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", WhereArgs: []interface{}{file, list.Name, time.Now().Add(time.Hour * -12)}})
	if len(parsetest) >= 1 {
		return
	}
	logger.Log.Debug("Parse Movie: ", file)

	m, err := parser.NewFileParser(filepath.Base(file), false, "movie")

	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

	addunmatched := false
	if err == nil {
		m.Title = strings.Trim(m.Title, " ")
		m.StripTitlePrefixPostfix(cfg_quality)
		m.Resolution = strings.ToLower(m.Resolution)
		m.Audio = strings.ToLower(m.Audio)
		m.Codec = strings.ToLower(m.Codec)
		m.Quality = strings.ToLower(m.Quality)
		logger.Log.Debug("Parsed Movie: ", file, " as ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)

		entriesfound := 0
		if entriesfound == 0 && len(m.Imdb) >= 1 {
			movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? COLLATE NOCASE and Movies.listname = ?", WhereArgs: []interface{}{m.Imdb, list.Name}})
			entriesfound = len(movies)
			if len(movies) == 1 {
				movie = movies[0]
			}
		}
		if entriesfound == 0 && len(m.Imdb) >= 1 {
			movies, _ := database.QueryMovies(database.Query{Select: "movies.id", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? COLLATE NOCASE", WhereArgs: []interface{}{m.Imdb}})
			if len(movies) >= 1 {
				return
			}
		}
		if entriesfound == 0 {
			getmovie, imdb, entriesfound := importfeed.MovieFindDbByTitle(m.Title, strconv.Itoa(m.Year), list.Name, cfg_quality.CheckYear1, "parse")
			if entriesfound >= 1 {
				m.Imdb = imdb
				movie = getmovie
			}
		}

		if movie.ID == 0 {
			if list.Addfound {
				if len(m.Imdb) >= 1 {
					sww := sizedwaitgroup.New(1)
					var dbmovie database.Dbmovie
					dbmovie.ImdbID = m.Imdb
					sww.Add()
					importfeed.JobImportMovies(dbmovie, configEntry, list, &sww)
					sww.Wait()
					movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? COLLATE NOCASE and Movies.listname = ?", WhereArgs: []interface{}{m.Imdb, list.Name}})
					if len(movies) == 1 {
						movie = movies[0]
					}
				}
			}
		}
		if movie.ID >= 1 {
			cutoffPrio := parser.NewCutoffPrio(configEntry, cfg_quality)

			m.GetPriority(configEntry, cfg_quality)
			errparsev := m.ParseVideoFile(file, configEntry, cfg_quality)
			if errparsev != nil {
				return
			}
			counterf, _ := database.CountRows("movie_files", database.Query{Where: "location = ? AND movie_id = ?", WhereArgs: []interface{}{file, movie.ID}})
			if counterf == 0 {
				reached := false
				if m.Priority >= cutoffPrio.Priority {
					reached = true
				}

				if movie.Rootpath == "" && movie.ID != 0 {
					rootpath := ""
					for idxpath := range configEntry.Data {
						if !config.ConfigCheck("path_" + configEntry.Data[idxpath].Template_path) {
							continue
						}
						var cfg_path config.PathsConfig
						config.ConfigGet("path_"+configEntry.Data[idxpath].Template_path, &cfg_path)

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
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id"},
					[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie.ID, movie.DbmovieID})
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
		valuesupsert := make(map[string]interface{})
		valuesupsert["listname"] = list.Name
		valuesupsert["filepath"] = file
		valuesupsert["last_checked"] = time.Now()
		valuesupsert["parsed_data"] = string(mjson)
		database.Upsert("movie_file_unmatcheds", valuesupsert, database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, list.Name}})
		for key := range valuesupsert {
			delete(valuesupsert, key)
		}
		valuesupsert = nil
	}
}

func readCSVFromURL(url string) ([][]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		logger.Log.Error("Failed to get CSV from: ", url)
		return nil, err
	}

	defer resp.Body.Close()
	//reader.Comma = ';'
	data, err := csv.NewReader(resp.Body).ReadAll()
	if err != nil {
		logger.Log.Error("Failed to read CSV from: ", url)
		return nil, err
	}

	return data, nil
}

func getMissingIMDBMoviesV2(configEntry config.MediaTypeConfig, list config.MediaListsConfig) []database.Dbmovie {
	if !list.Enabled {
		return []database.Dbmovie{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return []database.Dbmovie{}
	}
	var cfg_list config.ListsConfig
	config.ConfigGet("list_"+list.Template_list, &cfg_list)

	if len(cfg_list.Url) >= 1 {
		data, err := readCSVFromURL(cfg_list.Url)
		if err != nil {
			logger.Log.Error("Failed to read CSV from: ", cfg_list.Url)
			return []database.Dbmovie{}
		}
		d := make([]database.Dbmovie, 0, len(data))

		for idx := range data {
			// skip header
			if idx == 0 {
				continue
			}

			if !importfeed.AllowMovieImport(data[idx][1], cfg_list) {
				continue
			}
			year, _ := strconv.ParseInt(data[idx][10], 0, 64)
			votes, _ := strconv.ParseInt(data[idx][12], 0, 64)
			voteavg, _ := strconv.ParseFloat(data[idx][8], 64)
			d = append(d, database.Dbmovie{ImdbID: data[idx][1], Title: data[idx][5], URL: data[idx][6], VoteAverage: float32(voteavg), Year: int(year), VoteCount: int(votes)})
		}
		return d
	}
	return []database.Dbmovie{}
}

func GetTraktUserPublicMovieList(configEntry config.MediaTypeConfig, list config.MediaListsConfig) []database.Dbmovie {
	if !list.Enabled {
		return []database.Dbmovie{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		return []database.Dbmovie{}
	}
	var cfg_list config.ListsConfig
	config.ConfigGet("list_"+list.Template_list, &cfg_list)

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

			if !importfeed.AllowMovieImport(data[idx].Movie.Ids.Imdb, cfg_list) {
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

func Importnewmoviessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	logger.Log.Debug("get feeds for ", row.Name, list.Name)
	results := Feeds(row, list)
	logger.Log.Debug("RESULT -get feeds for ", row.Name, list.Name, len(results.Movies))

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if cfg_general.WorkerMetadata == 0 {
		cfg_general.WorkerMetadata = 1
	}

	swg := sizedwaitgroup.New(cfg_general.WorkerMetadata)
	for idxmovie := range results.Movies {
		founddbmovie := false
		foundmovie := false
		dbmovie, dbmovieerr := database.GetDbmovie(database.Query{Select: "id", Where: "imdb_id=?", WhereArgs: []interface{}{results.Movies[idxmovie].ImdbID}})
		if dbmovieerr == nil {
			founddbmovie = true
			counter, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, list.Name}})
			if counter >= 1 {
				foundmovie = true
			}

			if len(list.Ignore_map_lists) >= 1 && !foundmovie {
				for idx := range list.Ignore_map_lists {
					counter, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id=? and listname=?", WhereArgs: []interface{}{dbmovie.ID, list.Ignore_map_lists[idx]}})
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
			importfeed.JobImportMovies(results.Movies[idxmovie], row, list, &swg)
		}
	}
	swg.Wait()
}

func Getnewmovies(row config.MediaTypeConfig) {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	logger.Log.Info("Scan Movie File")
	filesfound := findFiles(row)

	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for _, list := range row.Lists {
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			continue
		}
		var cfg_quality config.QualityConfig
		config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

		defaultPrio := &parser.ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
		defaultPrio.GetPriority(row, cfg_quality)

		logger.Log.Info("Find Movie File")
		for idxfile, file := range scanner.GetFilesAdded(filesfound, list.Name) {
			logger.Log.Info("Parse Movie ", idxfile, " path: ", file)
			swf.Add()
			JobImportMovieParseV2(file, row, list, true, *defaultPrio, &swf)
		}
	}
	swf.Wait()
}
func getnewmoviessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

	defaultPrio := &parser.ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
	defaultPrio.GetPriority(row, cfg_quality)

	logger.Log.Info("Scan Movie File")
	var filesfound []string
	for idxpath := range row.Data {
		if !config.ConfigCheck("path_" + row.Data[idxpath].Template_path) {
			continue
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+row.Data[idxpath].Template_path, &cfg_path)

		filesfound_add := scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
		filesfound = append(filesfound, filesfound_add...)
	}
	filesadded := scanner.GetFilesAdded(filesfound, list.Name)
	logger.Log.Info("Find Movie File")
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for idxfile := range filesadded {
		logger.Log.Info("Parse Movie ", idxfile, " of ", len(filesadded), " path: ", filesadded[idxfile])
		swf.Add()
		JobImportMovieParseV2(filesadded[idxfile], row, list, true, *defaultPrio, &swf)
	}
	swf.Wait()
}

func checkmissingmoviessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	movies, _ := database.QueryMovies(database.Query{Select: "id", Where: "listname = ?", WhereArgs: []interface{}{list.Name}})

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)
	for idx := range movies {
		moviefile, _ := database.QueryMovieFiles(database.Query{Select: "location", Where: "movie_id = ?", WhereArgs: []interface{}{movies[idx].ID}})
		for idxfile := range moviefile {
			swfile.Add()
			JobImportFileCheck(moviefile[idxfile].Location, "movie", &swfile)
		}
	}
	swfile.Wait()
}

func checkmissingmoviesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	movies, _ := database.QueryMovies(database.Query{Select: "id, missing", Where: "listname = ?", WhereArgs: []interface{}{list.Name}})

	for idxmovie := range movies {
		counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies[idxmovie].ID}})
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

func checkreachedmoviesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	movies, _ := database.QueryMovies(database.Query{Select: "id, quality_reached, quality_profile", Where: "listname=?", WhereArgs: []interface{}{list.Name}})
	for idxepi := range movies {
		if !config.ConfigCheck("quality_" + movies[idxepi].QualityProfile) {
			continue
		}
		var cfg_quality config.QualityConfig
		config.ConfigGet("quality_"+movies[idxepi].QualityProfile, &cfg_quality)

		MinimumPriority := parser.GetHighestMoviePriorityByFiles(movies[idxepi], row, cfg_quality)
		cutoffPrio := parser.NewCutoffPrio(row, cfg_quality)
		reached := false
		if MinimumPriority >= cutoffPrio.Priority {
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

func moviesStructureSingle(row config.MediaTypeConfig, list config.MediaListsConfig) {

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	if cfg_general.WorkerFiles == 0 {
		cfg_general.WorkerFiles = 1
	}

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)

	for idxpath := range row.DataImport {
		mappath := ""
		if !config.ConfigCheck("path_" + row.DataImport[idxpath].Template_path) {
			continue
		}
		var cfg_path_import config.PathsConfig
		config.ConfigGet("path_"+row.DataImport[idxpath].Template_path, &cfg_path_import)

		var cfg_path config.PathsConfig
		if len(row.Data) >= 1 {
			mappath = row.Data[0].Template_path
			if !config.ConfigCheck("path_" + mappath) {
				continue
			}
			config.ConfigGet("path_"+mappath, &cfg_path)
		} else {
			continue
		}
		swfile.Add()
		structure.StructureFolders("movie", cfg_path_import, cfg_path, row, list)
		swfile.Done()
	}
	swfile.Wait()
}

func RefreshMovies() {

	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
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
		importfeed.JobReloadMovies(dbmovies[idxmovie], config.MediaTypeConfig{}, config.MediaListsConfig{}, &sw)
	}
	sw.Wait()
}

func RefreshMovie(id string) {
	dbmovies, _ := database.QueryDbmovie(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})

	sw := sizedwaitgroup.New(1)
	for idxmovie := range dbmovies {
		logger.Log.Info("Refresh Movie ", idxmovie, " of ", len(dbmovies), " imdb: ", dbmovies[idxmovie].ImdbID)
		sw.Add()
		importfeed.JobReloadMovies(dbmovies[idxmovie], config.MediaTypeConfig{}, config.MediaListsConfig{}, &sw)
	}
	sw.Wait()
}

func RefreshMoviesInc() {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
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
		importfeed.JobReloadMovies(dbmovies[idxmovie], config.MediaTypeConfig{}, config.MediaListsConfig{}, &sw)
	}
	sw.Wait()
}

func Movies_all_jobs(job string, force bool) {

	movie_keys, _ := config.ConfigDB.Keys([]byte("movie_*"), 0, 0, true)

	for _, idxmovie := range movie_keys {
		var cfg_movie config.MediaTypeConfig
		config.ConfigGet(string(idxmovie), &cfg_movie)

		Movies_single_jobs(job, cfg_movie.Name, "", force)
	}
}

func Movies_single_jobs(job string, typename string, listname string, force bool) {
	jobName := job + "_movies"
	if typename != "" {
		jobName += "_" + typename
	}
	if listname != "" {
		jobName += "_" + listname
	}
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if cfg_general.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", typename)
		return
	}

	job = strings.ToLower(job)
	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, typename, "Movie", time.Now()})
	logger.Log.Info("Started Job: ", jobName)
	ok, _ := config.ConfigDB.Has("movie_" + typename)
	if ok {
		var cfg_movie config.MediaTypeConfig
		config.ConfigGet("movie_"+typename, &cfg_movie)
		if cfg_movie.Searchmissing_incremental == 0 {
			cfg_movie.Searchmissing_incremental = 20
		}
		if cfg_movie.Searchupgrade_incremental == 0 {
			cfg_movie.Searchupgrade_incremental = 20
		}

		switch job {
		case "datafull":
			Getnewmovies(cfg_movie)
		case "searchmissingfull":
			searcher.SearchMovieMissing(cfg_movie, 0, false)
		case "searchmissinginc":
			searcher.SearchMovieMissing(cfg_movie, cfg_movie.Searchmissing_incremental, false)
		case "searchupgradefull":
			searcher.SearchMovieUpgrade(cfg_movie, 0, false)
		case "searchupgradeinc":
			searcher.SearchMovieUpgrade(cfg_movie, cfg_movie.Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			searcher.SearchMovieMissing(cfg_movie, 0, true)
		case "searchmissinginctitle":
			searcher.SearchMovieMissing(cfg_movie, cfg_movie.Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			searcher.SearchMovieUpgrade(cfg_movie, 0, true)
		case "searchupgradeinctitle":
			searcher.SearchMovieUpgrade(cfg_movie, cfg_movie.Searchupgrade_incremental, true)
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
				getnewmoviessingle(cfg_movie, cfg_movie.Lists[idxlist])
			case "checkmissing":
				checkmissingmoviessingle(cfg_movie, cfg_movie.Lists[idxlist])
			case "checkmissingflag":
				checkmissingmoviesflag(cfg_movie, cfg_movie.Lists[idxlist])
			case "checkreachedflag":
				checkreachedmoviesflag(cfg_movie, cfg_movie.Lists[idxlist])
			case "structure":
				moviesStructureSingle(cfg_movie, cfg_movie.Lists[idxlist])
			case "clearhistory":
				database.DeleteRow("movie_histories", database.Query{Where: "movie_id in (Select id from movies where listname=?)", WhereArgs: []interface{}{typename}})
			case "feeds":
				Importnewmoviessingle(cfg_movie, cfg_movie.Lists[idxlist])
			default:
				// other stuff
			}
		}
		for qual := range qualis {
			switch job {
			case "rss":
				searcher.SearchMovieRSS(cfg_movie, qual)
			}
		}
		for key := range qualis {
			delete(qualis, key)
		}
		qualis = nil
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", typename)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", jobName)
	debug.FreeOSMemory()
}

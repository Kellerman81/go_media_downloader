package utils

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/cache"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"go.uber.org/zap"
)

func jobImportMovieParseV2(file string, cfg string, listname string, updatemissing bool, addfound bool) bool {
	m, err := parser.NewFileParser(filepath.Base(file), false, "movie")

	addunmatched := false
	if err == nil {
		defer m.Close()
		m.Title = strings.Trim(m.Title, " ")

		//keep list empty for auto detect list since the default list is in the listconfig!
		parser.GetDbIDs("movie", m, cfg, "", true)
		if m.MovieID != 0 && m.Listname != "" {
			listname = m.Listname
		}

		if listname == "" {
			return false
		}
		template_Quality := config.Cfg.Media[cfg].ListsMap[listname].Template_quality
		if !config.ConfigCheck("quality_" + template_Quality) {

			logger.Log.GlobalLogger.Error("Quality for List: " + listname + " not found")
			return false
		}
		var counter int
		if m.MovieID == 0 && listname != "" && addfound {
			m.MovieID, err = parser.AddMovieIfNotFound(m, listname, cfg)
			if err != nil {
				return false
			}
		}
		if m.MovieID >= 1 {

			parser.GetPriorityMap(m, cfg, template_Quality, true)
			err = parser.ParseVideoFile(m, file, cfg, template_Quality)
			if err != nil {
				logger.Log.GlobalLogger.Error("Parse failed", zap.Error(err))
				return false
			}
			counter, err = database.CountRowsStatic("select count() from movie_files where location = ? and movie_id = ?", file, m.MovieID)
			if counter == 0 && err == nil {
				ok := false
				if m.Priority >= parser.NewCutoffPrio(cfg, template_Quality) {
					ok = true
				}
				rootpath, _ := database.QueryColumnString("select rootpath from movies where id = ?", m.MovieID)

				if rootpath == "" && m.MovieID != 0 {
					updateRootpath(file, "movies", m.MovieID, cfg)
				}

				database.InsertNamed("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :movie_id, :dbmovie_id, :height, :width)",
					database.MovieFile{
						Location:       file,
						Filename:       filepath.Base(file),
						Extension:      filepath.Ext(file),
						QualityProfile: template_Quality,
						ResolutionID:   m.ResolutionID,
						QualityID:      m.QualityID,
						CodecID:        m.CodecID,
						AudioID:        m.AudioID,
						Proper:         m.Proper,
						Repack:         m.Repack,
						Extended:       m.Extended,
						MovieID:        m.MovieID,
						DbmovieID:      m.DbmovieID,
						Height:         m.Height,
						Width:          m.Width})
				if updatemissing {
					database.UpdateColumnStatic("Update movies set missing = ? where id = ?", 0, m.MovieID)
					database.UpdateColumnStatic("Update movies set quality_reached = ? where id = ?", ok, m.MovieID)
				}

				database.DeleteRowStatic("Delete from movie_file_unmatcheds where filepath = ?", file)
				return true
			}
		} else {
			addunmatched = true
			logger.Log.GlobalLogger.Error("Movie Parse failed - not matched", zap.String("file", file))
		}
	} else {
		addunmatched = true
		logger.Log.GlobalLogger.Error("Movie Parse failed", zap.String("file", file))
	}

	if addunmatched {
		mjson := ""
		if m.AudioID != 0 {
			mjson += " Audioid: " + strconv.FormatUint(uint64(m.AudioID), 10)
		}
		if m.CodecID != 0 {
			mjson += " Codecid: " + strconv.FormatUint(uint64(m.CodecID), 10)
		}
		if m.QualityID != 0 {
			mjson += " Qualityid: " + strconv.FormatUint(uint64(m.QualityID), 10)
		}
		if m.ResolutionID != 0 {
			mjson += " Resolutionid: " + strconv.FormatUint(uint64(m.ResolutionID), 10)
		}
		if m.Listname != "" {
			mjson += " Listname: " + m.Listname
		}
		if m.Title != "" {
			mjson += " Title: " + m.Title
		}
		if m.Imdb != "" {
			mjson += " Imdb: " + m.Imdb
		}
		if m.Year != 0 {
			mjson += " Year: " + strconv.Itoa(m.Year)
		}

		id, _ := database.QueryColumnUint("select id from movie_file_unmatcheds where filepath = ? and listname = ?", file, listname)
		if id == 0 {
			database.InsertNamed("Insert into movie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", database.MovieFileUnmatched{Listname: listname, Filepath: file, LastChecked: sql.NullTime{Time: time.Now(), Valid: true}, ParsedData: mjson})
		} else {
			database.UpdateColumnStatic("Update movie_file_unmatcheds SET last_checked = ? where id = ?", sql.NullTime{Time: time.Now(), Valid: true}, id)
			database.UpdateColumnStatic("Update movie_file_unmatcheds SET parsed_data = ? where id = ?", mjson, id)
		}
	}
	return false
}

func getMissingIMDBMoviesV2(cfg string, listname string) (*feedResults, error) {
	url := config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Url
	if len(url) >= 1 {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			logger.Log.GlobalLogger.Error("Failed to read CSV from", zap.String("url", url))
			return nil, errNoListRead
		}
		resp, err := logger.WebClient.Do(req)
		if err != nil || resp == nil {
			logger.Log.GlobalLogger.Error("Failed to read CSV from", zap.String("url", url))
			return nil, errNoListRead
		}

		defer resp.Body.Close()

		parserimdb := csv.NewReader(resp.Body)
		parserimdb.ReuseRecord = true

		var d feedResults
		cnt, ok := logger.GlobalCounter[url]

		if ok {
			d.Movies = make([]string, 0, cnt)
		}
		var record []string
		for {
			record, err = parserimdb.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.GlobalLogger.Error("Failed to get row", zap.Error(err))
				return nil, errors.New("list csv import error")
			}
			if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
				logger.Log.GlobalLogger.Warn("skipped row", zap.String("imdb", record[1]))
				continue
			}
			d.Movies = append(d.Movies, record[1])
		}
		logger.GlobalCounter[url] = len(d.Movies)
		return &d, nil
	} else {
		logger.Log.GlobalLogger.Error("Failed to get url")
	}
	return nil, errNoListOther
}

func getTraktUserPublicMovieList(cfg string, listname string) (*feedResults, error) {

	if !config.Cfg.Media[cfg].ListsMap[listname].Enabled {
		return nil, errNoListEnabled
	}
	if !config.ConfigCheck("list_" + config.Cfg.Media[cfg].ListsMap[listname].Template_list) {
		return nil, errNoList
	}

	if len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktUsername) >= 1 && len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListName) >= 1 {
		if len(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListType) == 0 {
			return nil, errors.New("not show or movie")
		}
		data, err := apiexternal.TraktApi.GetUserList(config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktUsername, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListName, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListType, config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].Limit)
		if err != nil {
			logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", config.Cfg.Lists[config.Cfg.Media[cfg].ListsMap[listname].Template_list].TraktListName))
			return nil, errNoListRead
		}
		var d feedResults

		d.Movies = logger.CopyFunc(data.Entries, func(elem apiexternal.TraktUserList) string {
			return elem.Movie.Ids.Imdb
		})
		data = nil
		return &d, nil
	}
	return nil, errNoListOther
}

func importnewmoviessingle(cfg string, listname string) {
	logger.Log.GlobalLogger.Debug("get feeds for ", zap.String("config", cfg), zap.String("Listname", listname))

	feed, err := feeds(cfg, listname)
	if err != nil {
		return
	}
	defer feed.Close()

	if len(feed.Movies) >= 1 {

		if listname == "" {
			return
		}

		template_list := config.Cfg.Media[cfg].ListsMap[listname].Template_list

		foundmovie := false
		var id uint

		var dbmovies []database.Dbstatic_OneStringOneInt
		var movies []database.Dbstatic_OneStringOneInt

		if len(feed.Movies) > 900 {
			dbmovies, _ = database.QueryStaticColumnsOneStringOneInt("select imdb_id, id from dbmovies", false, database.CountRowsStaticNoError("select count() from dbmovies"))

			movies, _ = database.QueryStaticColumnsOneStringOneInt("select lower(listname), dbmovie_id from movies", false, database.CountRowsStaticNoError("select count() from movies"))
		} else {
			imdb_args := logger.CopyFunc(feed.Movies, func(elem string) interface{} {
				return elem
			})
			dbmovies, _ = database.QueryStaticColumnsOneStringOneInt("select imdb_id, id from dbmovies where imdb_id IN (?"+strings.Repeat(",?", len(imdb_args)-1)+")", false, 0, imdb_args...)

			movies, _ = database.QueryStaticColumnsOneStringOneInt("select lower(movies.listname), movies.dbmovie_id from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id IN (?"+strings.Repeat(",?", len(imdb_args)-1)+")", false, 0, imdb_args...)
		}
		var imdbids []string = make([]string, 0, len(feed.Movies)) //= make([]string, 0, 50)
		if len(feed.Movies) >= 1 {

			ignore := config.Cfg.Media[cfg].ListsMap[listname].Ignore_map_lists
			for idxmovie := range feed.Movies {
				if feed.Movies[idxmovie] == "" {
					continue
				}
				foundmovie = false
				id = 0
				for idxsdbmovie := range dbmovies {
					if dbmovies[idxsdbmovie].Str == feed.Movies[idxmovie] {
						id = uint(dbmovies[idxsdbmovie].Num)
						break
					}
				}
				if id != 0 {
					for idxsmovie := range movies {
						if movies[idxsmovie].Num == int(id) {
							if strings.EqualFold(movies[idxsmovie].Str, listname) {
								foundmovie = true
								//logger.Log.GlobalLogger.Debug("not allowwed movie1", zap.String("imdb", feed.Movies[idxmovie]))
								break
							}
							if len(ignore) >= 1 && !foundmovie {
								for idx := range ignore {
									if strings.EqualFold(movies[idxsmovie].Str, ignore[idx]) {
										foundmovie = true
										//logger.Log.GlobalLogger.Debug("not allowwed movie2", zap.String("imdb", feed.Movies[idxmovie]))
										break
									}
								}
								if foundmovie {
									break
								}
							}
						}
					}
				}
				if !foundmovie {
					if importfeed.AllowMovieImport(feed.Movies[idxmovie], template_list) {
						imdbids = append(imdbids, feed.Movies[idxmovie])
						logger.Log.GlobalLogger.Debug("allowwed movie", zap.String("imdb", feed.Movies[idxmovie]))
					} else {
						logger.Log.GlobalLogger.Debug("not allowwed movie", zap.String("imdb", feed.Movies[idxmovie]))
					}
				}
			}
		}
		importmoviesbyimdbids(&logger.InStringArrayStruct{Arr: imdbids}, cfg, listname)

	}

}
func importmoviesbyimdbids(imdbids *logger.InStringArrayStruct, cfg string, listname string) {
	workergroup := logger.WorkerPools["Metadata"].Group()
	for idxmovie := range imdbids.Arr {
		imdbID := imdbids.Arr[idxmovie]
		logger.Log.GlobalLogger.Info("Import Movie ", zap.Int("row", idxmovie), zap.String("imdb", imdbID))
		workergroup.Submit(func() {
			importfeed.JobImportMovies(imdbID, cfg, listname, true)
		})
	}
	workergroup.Wait()
	imdbids.Close()
}

func checkmissingmoviessingle(cfg string, listname string) {
	filesfound := database.QueryStaticStringArray("select location as str from movie_files where movie_id in (select id from movies where listname = ?)", false, database.CountRowsStaticNoError("select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"), listname)
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			//workergroup.Submit(func() {
			jobImportFileCheck(filesfound[idx], "movie")
			//})
		}
	}
	filesfound = nil
}

func checkmissingmoviesflag(cfg string, listname string) {
	movies, _ := database.QueryMovies(&database.Query{Select: "id, missing", Where: "listname = ?"}, listname)

	var counter int
	for idxmovie := range movies {
		counter, _ = database.CountRowsStatic("select count() from movie_files where movie_id = ?", movies[idxmovie].ID)
		if counter >= 1 {
			if movies[idxmovie].Missing {
				database.UpdateColumnStatic("Update movies set missing = ? where id = ?", 0, movies[idxmovie].ID)
			}
		} else {
			if !movies[idxmovie].Missing {
				database.UpdateColumnStatic("Update movies set missing = ? where id = ?", 1, movies[idxmovie].ID)
			}
		}
	}
	movies = nil
}

func checkreachedmoviesflag(cfg string, listname string) {
	movies, _ := database.QueryMovies(&database.Query{Select: "id, quality_reached, quality_profile", Where: "listname = ?"}, listname)
	var reached bool
	for idxepi := range movies {
		if !config.ConfigCheck("quality_" + movies[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error("Quality for Movie: " + strconv.Itoa(int(movies[idxepi].ID)) + " not found")
			continue
		}

		reached = false
		if searcher.GetHighestMoviePriorityByFiles(movies[idxepi].ID, cfg, movies[idxepi].QualityProfile) >= parser.NewCutoffPrio(cfg, movies[idxepi].QualityProfile) {
			reached = true
		}
		if movies[idxepi].QualityReached && !reached {
			database.UpdateColumnStatic("Update movies set quality_reached = ? where id = ?", 0, movies[idxepi].ID)
		}

		if !movies[idxepi].QualityReached && reached {
			database.UpdateColumnStatic("Update movies set quality_reached = ? where id = ?", 1, movies[idxepi].ID)
		}
	}
	movies = nil
}

func moviesStructureSingle(cfg string) {
	if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].Data[0].Template_path) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", config.Cfg.Media[cfg].Data[0].Template_path))
		return
	}
	var lastMoviesStructure *cache.CacheReturn
	var ok bool
	var pathvar string

	for idx := range config.Cfg.Media[cfg].DataImport {
		if !config.ConfigCheck("path_" + config.Cfg.Media[cfg].DataImport[idx].Template_path) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", config.Cfg.Media[cfg].DataImport[idx].Template_path))
			continue
		}
		pathvar = config.Cfg.Paths[config.Cfg.Media[cfg].DataImport[idx].Template_path].Path
		lastMoviesStructure, ok = logger.GlobalCache.Get("lastMoviesStructure")
		if ok {
			if lastMoviesStructure.Value.(string) == pathvar {
				time.Sleep(time.Duration(15) * time.Second)
			}
		}
		logger.GlobalCache.Set("lastMoviesStructure", pathvar, 5*time.Minute)

		structure.StructureFolders("movie", config.Cfg.Media[cfg].DataImport[idx].Template_path, config.Cfg.Media[cfg].Data[0].Template_path, cfg)
	}
	lastMoviesStructure = nil
}

func RefreshMovies() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshmoviesquery("select imdb_id as str from dbmovies", database.CountRowsStaticNoError("select count() from dbmovies"))
}

func RefreshMovie(id string) {
	refreshmoviesquery("select imdb_id as str from dbmovies where id = ?", 1, id)
}

func RefreshMoviesInc() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshmoviesquery("select imdb_id as str from dbmovies order by updated_at desc limit 100", 100)
}

func refreshmoviesquery(query string, count int, args ...interface{}) {
	dbmovies := database.QueryStaticStringArray(query, false, count, args...)
	for idxmovie := range dbmovies {
		listname, _ := database.QueryColumnString("select listname from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?)", dbmovies[idxmovie])

		logger.Log.GlobalLogger.Info("Refresh Movie ", zap.Int("row", idxmovie), zap.Int("of rows", len(dbmovies)), zap.String("imdb", dbmovies[idxmovie]))
		importfeed.JobImportMovies(dbmovies[idxmovie], config.FindconfigTemplateOnList("movie_", listname), listname, false)
	}
}

func Movies_all_jobs(job string, force bool) {
	for idx := range config.Cfg.Movies {
		Movies_single_jobs(job, "movie_"+config.Cfg.Movies[idx].Name, "", force)
	}
}

func Movies_single_jobs(job string, cfg string, listname string, force bool) {
	jobName := job
	if cfg != "" {
		jobName += "_" + cfg
	}
	if listname != "" {
		jobName += "_" + listname
	}

	if config.Cfg.General.SchedulerDisabled && !force {
		logger.Log.GlobalLogger.Info("Skipped Job", zap.String("Job", job), zap.String("config", cfg))
		return
	}

	dbresult, _ := database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: job, JobGroup: cfg, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	logger.Log.GlobalLogger.Info("Started Job", zap.String("Job", jobName))
	lists := config.Cfg.Media[cfg].Lists
	searchmissing_incremental := config.Cfg.Media[cfg].Searchmissing_incremental
	searchupgrade_incremental := config.Cfg.Media[cfg].Searchupgrade_incremental
	if searchmissing_incremental == 0 {
		searchmissing_incremental = 20
	}
	if searchupgrade_incremental == 0 {
		searchupgrade_incremental = 20
	}

	switch job {
	case "datafull":
		getNewFilesMap(cfg, "movie", "")
	case "searchmissingfull":
		searcher.SearchMovieMissing(cfg, 0, false)
	case "searchmissinginc":
		searcher.SearchMovieMissing(cfg, searchmissing_incremental, false)
	case "searchupgradefull":
		searcher.SearchMovieUpgrade(cfg, 0, false)
	case "searchupgradeinc":
		searcher.SearchMovieUpgrade(cfg, searchupgrade_incremental, false)
	case "searchmissingfulltitle":
		searcher.SearchMovieMissing(cfg, 0, true)
	case "searchmissinginctitle":
		searcher.SearchMovieMissing(cfg, searchmissing_incremental, true)
	case "searchupgradefulltitle":
		searcher.SearchMovieUpgrade(cfg, 0, true)
	case "searchupgradeinctitle":
		searcher.SearchMovieUpgrade(cfg, searchupgrade_incremental, true)
	case "structure":
		moviesStructureSingle(cfg)
	}
	if listname != "" {
		lists = []config.MediaListsConfig{config.Cfg.Media[cfg].ListsMap[listname]}
	}
	var qualis []string = make([]string, len(lists))
	for idxlist := range lists {
		qualis[idxlist] = lists[idxlist].Template_quality

		switch job {
		case "data":
			getNewFilesMap(cfg, "movie", lists[idxlist].Name)
		case "checkmissing":
			checkmissingmoviessingle(cfg, lists[idxlist].Name)
		case "checkmissingflag":
			checkmissingmoviesflag(cfg, lists[idxlist].Name)
		case "checkreachedflag":
			checkreachedmoviesflag(cfg, lists[idxlist].Name)
		case "clearhistory":
			database.DeleteRow("movie_histories", &database.Query{Where: "movie_id in (Select id from movies where listname = ? COLLATE NOCASE)"}, lists[idxlist].Name)
		case "feeds":
			importnewmoviessingle(cfg, lists[idxlist].Name)
		default:
			// other stuff
		}
	}
	unique := unique(&logger.InStringArrayStruct{Arr: qualis})
	for idxuni := range unique {
		switch job {
		case "rss":
			searcher.SearchMovieRSS(cfg, unique[idxuni])
		}
	}
	unique = nil
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumnStatic("Update job_histories set ended = ? where id = ?", time.Now().In(logger.TimeZone), dbid)
	}
	logger.Log.GlobalLogger.Info("Ended Job", zap.String("Job", jobName))
}

func unique(s *logger.InStringArrayStruct) []string {
	inResult := &logger.InStringArrayStruct{}
	result := s.Arr[:0]
	for idx := range s.Arr {
		if !logger.InStringArrayCaseSensitive(s.Arr[idx], inResult) {
			inResult.Arr = append(inResult.Arr, s.Arr[idx])
			result = append(result, s.Arr[idx])
		}
	}
	inResult.Close()
	s.Close()
	return result
}

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
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"go.uber.org/zap"
)

func jobImportMovieParseV2(file string, cfgp *config.MediaTypeConfig, listname string, updatemissing bool, addfound bool) bool {
	m, err := parser.NewFileParser(filepath.Base(file), false, "movie")

	addunmatched := false
	if err == nil {
		defer m.Close()
		m.Title = strings.TrimSpace(m.Title)

		//keep list empty for auto detect list since the default list is in the listconfig!
		parser.GetDbIDs("movie", m, cfgp, "", true)
		if m.MovieID != 0 && m.Listname != "" {
			listname = m.Listname
		}

		if listname == "" {
			return false
		}
		templateQuality := cfgp.ListsMap[listname].TemplateQuality
		if !config.ConfigCheck("quality_" + templateQuality) {

			logger.Log.GlobalLogger.Error("Quality for List: " + listname + " not found")
			return false
		}
		var counter int
		if m.MovieID == 0 && listname != "" && addfound {
			m.MovieID, err = parser.AddMovieIfNotFound(m, listname, cfgp)
			if err != nil {
				return false
			}
		}
		if m.MovieID >= 1 {

			parser.GetPriorityMap(m, cfgp, templateQuality, true, false)
			err = parser.ParseVideoFile(m, file, cfgp, templateQuality)
			if err != nil {
				logger.Log.GlobalLogger.Error("Parse failed", zap.Error(err))
				return false
			}
			counter, err = database.CountRowsStatic(database.Querywithargs{QueryString: "select count() from movie_files where location = ? and movie_id = ?", Args: []interface{}{file, m.MovieID}})
			if counter == 0 && err == nil {
				ok := false
				if m.Priority >= parser.NewCutoffPrio(cfgp, templateQuality) {
					ok = true
				}
				rootpath, _ := database.QueryColumnString(database.Querywithargs{QueryString: "select rootpath from movies where id = ?", Args: []interface{}{m.MovieID}})

				if rootpath == "" && m.MovieID != 0 {
					updateRootpath(file, "movies", m.MovieID, cfgp)
				}

				database.InsertNamed("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :movie_id, :dbmovie_id, :height, :width)",
					database.MovieFile{
						Location:       file,
						Filename:       filepath.Base(file),
						Extension:      filepath.Ext(file),
						QualityProfile: templateQuality,
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
					updatemoviesmissing(0, m.MovieID)

					okint := 0
					if ok {
						okint = 1
					}
					updatemoviesreached(okint, m.MovieID)
				}

				database.DeleteRowStatic(database.Querywithargs{QueryString: "Delete from movie_file_unmatcheds where filepath = ?", Args: []interface{}{file}})
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
		var bld strings.Builder
		defer bld.Reset()
		if m.AudioID != 0 {
			bld.WriteString(" Audioid: " + strconv.FormatUint(uint64(m.AudioID), 10))
		}
		if m.CodecID != 0 {
			bld.WriteString(" Codecid: " + strconv.FormatUint(uint64(m.CodecID), 10))
		}
		if m.QualityID != 0 {
			bld.WriteString(" Qualityid: " + strconv.FormatUint(uint64(m.QualityID), 10))
		}
		if m.ResolutionID != 0 {
			bld.WriteString(" Resolutionid: " + strconv.FormatUint(uint64(m.ResolutionID), 10))
		}
		if m.Listname != "" {
			bld.WriteString(" Listname: " + m.Listname)
		}
		if m.Title != "" {
			bld.WriteString(" Title: " + m.Title)
		}
		if m.Imdb != "" {
			bld.WriteString(" Imdb: " + m.Imdb)
		}
		if m.Year != 0 {
			bld.WriteString(" Year: " + strconv.Itoa(m.Year))
		}

		id, _ := database.QueryColumnUint(database.Querywithargs{QueryString: "select id from movie_file_unmatcheds where filepath = ? and listname = ?", Args: []interface{}{file, listname}})
		if id == 0 {
			database.InsertNamed("Insert into movie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", database.MovieFileUnmatched{Listname: listname, Filepath: file, LastChecked: sql.NullTime{Time: time.Now(), Valid: true}, ParsedData: bld.String()})
		} else {
			database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update movie_file_unmatcheds SET last_checked = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, id}})
			database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update movie_file_unmatcheds SET parsed_data = ? where id = ?", Args: []interface{}{bld.String(), id}})
		}
	}
	return false
}

func updatemoviesreached(reached int, dbmovieid uint) {
	database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update movies set quality_reached = ? where id = ?", Args: []interface{}{reached, dbmovieid}})
}

func updatemoviesmissing(missing int, dbmovieid uint) {
	database.UpdateColumnStatic(database.Querywithargs{QueryString: "Update movies set missing = ? where id = ?", Args: []interface{}{missing, dbmovieid}})
}

func getMissingIMDBMoviesV2(cfgplist *config.MediaListsConfig) (*feedResults, error) {
	url := config.Cfg.Lists[cfgplist.TemplateList].Url
	if url != "" {
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

		d := new(feedResults)
		cnt, ok := logger.GlobalCounter[url]

		if ok {
			d.Movies = logger.GrowSliceBy(d.Movies, cnt)
		}
		var record []string
		for {
			record, err = parserimdb.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Log.GlobalLogger.Error("Failed to get row", zap.Error(err))
				d = nil
				return nil, errors.New("list csv import error")
			}
			if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
				logger.Log.GlobalLogger.Warn("skipped row", zap.String("imdb", record[1]))
				continue
			}
			d.Movies = append(d.Movies, record[1])
		}
		logger.GlobalCounter[url] = len(d.Movies)
		parserimdb = nil
		record = nil
		return d, nil
	} else {
		logger.Log.GlobalLogger.Error("Failed to get url")
	}
	return nil, errNoListOther
}

func getTraktUserPublicMovieList(cfgplist *config.MediaListsConfig) (*feedResults, error) {

	if !cfgplist.Enabled {
		return nil, errNoListEnabled
	}
	if !config.ConfigCheck("list_" + cfgplist.TemplateList) {
		return nil, errNoList
	}

	if config.Cfg.Lists[cfgplist.TemplateList].TraktUsername != "" && config.Cfg.Lists[cfgplist.TemplateList].TraktListName != "" {
		if config.Cfg.Lists[cfgplist.TemplateList].TraktListType == "" {
			return nil, errors.New("not show or movie")
		}
		data, err := apiexternal.TraktApi.GetUserList(config.Cfg.Lists[cfgplist.TemplateList].TraktUsername, config.Cfg.Lists[cfgplist.TemplateList].TraktListName, config.Cfg.Lists[cfgplist.TemplateList].TraktListType, config.Cfg.Lists[cfgplist.TemplateList].Limit)
		if err != nil {
			logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", config.Cfg.Lists[cfgplist.TemplateList].TraktListName))
			return nil, errNoListRead
		}
		d := new(feedResults)

		d.Movies = logger.CopyFunc(data.Entries, func(elem apiexternal.TraktUserList) string {
			return elem.Movie.Ids.Imdb
		})
		data = nil
		return d, nil
	}
	return nil, errNoListOther
}

func importnewmoviessingle(cfgp *config.MediaTypeConfig, listname string) {
	if listname == "" {
		return
	}
	logger.Log.GlobalLogger.Debug("get feeds for ", zap.String("config", cfgp.NamePrefix), zap.String("Listname", listname))

	feed, err := feeds(cfgp, listname)
	if err != nil {
		return
	}
	defer feed.Close()

	lenfeed := len(feed.Movies)
	if lenfeed >= 1 {

		foundmovie := false

		var dbmovies []database.Dbstatic_OneStringOneInt
		var movies []database.Dbstatic_OneStringOneInt

		if lenfeed > 900 {
			dbmovies, _ = database.QueryStaticColumnsOneStringOneInt(false, database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from dbmovies"}), database.Querywithargs{QueryString: "select imdb_id, id from dbmovies"})

			movies, _ = database.QueryStaticColumnsOneStringOneInt(false, database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from movies"}), database.Querywithargs{QueryString: "select lower(listname), dbmovie_id from movies"})
		} else {
			imdbArgs := logger.CopyFunc(feed.Movies, func(elem string) interface{} {
				return elem
			})
			dbmovies, _ = database.QueryStaticColumnsOneStringOneInt(false, 0, database.Querywithargs{QueryString: "select imdb_id, id from dbmovies where imdb_id IN (?" + strings.Repeat(",?", len(imdbArgs)-1) + ")", Args: imdbArgs})

			movies, _ = database.QueryStaticColumnsOneStringOneInt(false, 0, database.Querywithargs{QueryString: "select lower(movies.listname), movies.dbmovie_id from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id IN (?" + strings.Repeat(",?", len(imdbArgs)-1) + ")", Args: imdbArgs})
			imdbArgs = nil
		}
		imdbids := &logger.InStringArrayStruct{Arr: make([]string, 0, lenfeed)}
		if lenfeed >= 1 {

			var intid int
			ignorearr := cfgp.ListsMap[listname].IgnoreMapLists
			lenignore := len(ignorearr)
			for idxmovie := range feed.Movies {
				if feed.Movies[idxmovie] == "" {
					continue
				}
				foundmovie = false
				intid = 0
				for idxsdbmovie := range dbmovies {
					if dbmovies[idxsdbmovie].Str == feed.Movies[idxmovie] {
						intid = dbmovies[idxsdbmovie].Num
						break
					}
				}
				if intid != 0 {
					for idxsmovie := range movies {
						if movies[idxsmovie].Num == intid {
							if strings.EqualFold(movies[idxsmovie].Str, listname) {
								foundmovie = true
								//logger.Log.GlobalLogger.Debug("not allowwed movie1", zap.String("imdb", feed.Movies[idxmovie]))
								break
							}
							if lenignore >= 1 && !foundmovie {
								for idx := range ignorearr {
									if strings.EqualFold(movies[idxsmovie].Str, ignorearr[idx]) {
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
					if importfeed.AllowMovieImport(feed.Movies[idxmovie], cfgp.ListsMap[listname].TemplateList) {
						imdbids.Arr = append(imdbids.Arr, feed.Movies[idxmovie])
					} else {
						logger.Log.GlobalLogger.Debug("not allowwed movie", zap.String("imdb", feed.Movies[idxmovie]))
					}
				}
			}
			ignorearr = nil
		}
		importmoviesbyimdbids(imdbids, cfgp, listname)
		dbmovies = nil
		movies = nil
		imdbids.Close()
	}

}
func importmoviesbyimdbids(imdbids *logger.InStringArrayStruct, cfgp *config.MediaTypeConfig, listname string) {
	workergroup := logger.WorkerPools["Metadata"].Group()
	for idxmovie := range imdbids.Arr {
		imdbID := imdbids.Arr[idxmovie]
		logger.Log.GlobalLogger.Info("Import Movie ", zap.Int("row", idxmovie), zap.String("imdb", imdbID))
		workergroup.Submit(func() {
			importfeed.JobImportMovies(imdbID, cfgp, listname, true)
		})
	}
	workergroup.Wait()
	imdbids.Close()
}

func checkmissingmoviessingle(cfgp *config.MediaTypeConfig, listname string) {
	filesfound := database.QueryStaticStringArray(false, database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"}), database.Querywithargs{QueryString: "select location as str from movie_files where movie_id in (select id from movies where listname = ?)", Args: []interface{}{listname}})
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			//workergroup.Submit(func() {
			jobImportFileCheck(filesfound[idx], "movie")
			//})
		}
	}
	filesfound = nil
}

func checkmissingmoviesflag(cfgp *config.MediaTypeConfig, listname string) {
	movies, _ := database.QueryMovies(database.Querywithargs{Query: database.Query{Select: "id, missing", Where: "listname = ?"}, Args: []interface{}{listname}})

	var counter int
	querycount := "select count() from movie_files where movie_id = ?"
	for idxmovie := range movies {
		counter, _ = database.CountRowsStatic(database.Querywithargs{QueryString: querycount, Args: []interface{}{movies[idxmovie].ID}})
		if counter >= 1 {
			if movies[idxmovie].Missing {
				updatemoviesmissing(0, movies[idxmovie].ID)
			}
		} else {
			if !movies[idxmovie].Missing {
				updatemoviesmissing(1, movies[idxmovie].ID)
			}
		}
	}
	movies = nil
}

func checkreachedmoviesflag(cfgp *config.MediaTypeConfig, listname string) {
	movies, _ := database.QueryMovies(database.Querywithargs{Query: database.Query{Select: "id, quality_reached, quality_profile", Where: "listname = ?"}, Args: []interface{}{listname}})
	var reached bool
	for idxepi := range movies {
		if !config.ConfigCheck("quality_" + movies[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error("Quality for Movie: " + strconv.Itoa(int(movies[idxepi].ID)) + " not found")
			continue
		}

		reached = false
		if searcher.GetHighestMoviePriorityByFiles(movies[idxepi].ID, cfgp, movies[idxepi].QualityProfile) >= parser.NewCutoffPrio(cfgp, movies[idxepi].QualityProfile) {
			reached = true
		}
		if movies[idxepi].QualityReached && !reached {
			updatemoviesreached(0, movies[idxepi].ID)
		}

		if !movies[idxepi].QualityReached && reached {
			updatemoviesreached(1, movies[idxepi].ID)
		}
	}
	movies = nil
}

var lastMoviesStructure string

func moviesStructureSingle(cfgp *config.MediaTypeConfig) {
	if !config.ConfigCheck("path_" + cfgp.Data[0].TemplatePath) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.Data[0].TemplatePath))
		return
	}
	var pathvar string

	for idx := range cfgp.DataImport {
		if !config.ConfigCheck("path_" + cfgp.DataImport[idx].TemplatePath) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.DataImport[idx].TemplatePath))
			continue
		}
		pathvar = config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].Path
		if lastMoviesStructure == pathvar {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastMoviesStructure = pathvar
		structure.StructureFolders("movie", cfgp.DataImport[idx].TemplatePath, cfgp.Data[0].TemplatePath, cfgp)
	}
}

func RefreshMovies() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshmoviesquery("select imdb_id as str from dbmovies", database.CountRowsStaticNoError(database.Querywithargs{QueryString: "select count() from dbmovies"}))
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
	dbmovies := database.QueryStaticStringArray(false, count, database.Querywithargs{QueryString: query, Args: args})
	querylist := "select listname from movies where dbmovie_id in (select id from dbmovies where imdb_id = ?)"
	var listname string
	var cfgp config.MediaTypeConfig
	for idxmovie := range dbmovies {
		listname, _ = database.QueryColumnString(database.Querywithargs{QueryString: querylist, Args: []interface{}{dbmovies[idxmovie]}})

		logger.Log.GlobalLogger.Info("Refresh Movie ", zap.Int("row", idxmovie), zap.Int("of rows", len(dbmovies)), zap.String("imdb", dbmovies[idxmovie]))
		cfgp = config.Cfg.Media[config.FindconfigTemplateOnList("movie_", listname)]
		importfeed.JobImportMovies(dbmovies[idxmovie], &cfgp, listname, false)
	}
	dbmovies = nil
	args = nil
}

func MoviesAllJobs(job string, force bool) {
	var cfgp config.MediaTypeConfig
	for idx := range config.Cfg.Movies {
		cfgp = config.Cfg.Media["movie_"+config.Cfg.Movies[idx].Name]
		MoviesSingleJobs(job, &cfgp, "", force)
	}
}

const jobstarted string = "Started Job"
const jobended string = "Ended Job"

func MoviesSingleJobs(job string, cfgp *config.MediaTypeConfig, listname string, force bool) {
	jobName := job
	if cfgp.Name != "" {
		jobName += "_" + cfgp.NamePrefix
	}
	if listname != "" {
		jobName += "_" + listname
	}

	if config.Cfg.General.SchedulerDisabled && !force {
		logger.Log.GlobalLogger.Info("Skipped Job", zap.String("Job", job), zap.String("config", cfgp.NamePrefix))
		return
	}

	//dbresult, _ := database.InsertNamed("Insert into job_histories (job_type, job_group, job_category, started) values (:job_type, :job_group, :job_category, :started)", database.JobHistory{JobType: job, JobGroup: cfg, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	dbresult, _ := insertjobhistory(database.JobHistory{JobType: job, JobGroup: cfgp.NamePrefix, JobCategory: "Serie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	logger.Log.GlobalLogger.Info(jobstarted, zap.String("Job", jobName))
	lists := cfgp.Lists
	searchmissingIncremental := cfgp.SearchmissingIncremental
	searchupgradeIncremental := cfgp.SearchupgradeIncremental
	if searchmissingIncremental == 0 {
		searchmissingIncremental = 20
	}
	if searchupgradeIncremental == 0 {
		searchupgradeIncremental = 20
	}

	switch job {
	case "datafull":
		getNewFilesMap(cfgp, "movie", "")
	case "searchmissingfull":
		searcher.SearchMovieMissing(cfgp, 0, false)
	case "searchmissinginc":
		searcher.SearchMovieMissing(cfgp, searchmissingIncremental, false)
	case "searchupgradefull":
		searcher.SearchMovieUpgrade(cfgp, 0, false)
	case "searchupgradeinc":
		searcher.SearchMovieUpgrade(cfgp, searchupgradeIncremental, false)
	case "searchmissingfulltitle":
		searcher.SearchMovieMissing(cfgp, 0, true)
	case "searchmissinginctitle":
		searcher.SearchMovieMissing(cfgp, searchmissingIncremental, true)
	case "searchupgradefulltitle":
		searcher.SearchMovieUpgrade(cfgp, 0, true)
	case "searchupgradeinctitle":
		searcher.SearchMovieUpgrade(cfgp, searchupgradeIncremental, true)
	case "structure":
		moviesStructureSingle(cfgp)
	}
	if listname != "" {
		lists = []config.MediaListsConfig{cfgp.ListsMap[listname]}
	}
	var qualis []string = make([]string, len(lists))
	for idxlist := range lists {
		qualis[idxlist] = lists[idxlist].TemplateQuality

		switch job {
		case "data":
			getNewFilesMap(cfgp, "movie", lists[idxlist].Name)
		case "checkmissing":
			checkmissingmoviessingle(cfgp, lists[idxlist].Name)
		case "checkmissingflag":
			checkmissingmoviesflag(cfgp, lists[idxlist].Name)
		case "checkreachedflag":
			checkreachedmoviesflag(cfgp, lists[idxlist].Name)
		case "clearhistory":
			database.DeleteRow("movie_histories", database.Querywithargs{Query: database.Query{Where: "movie_id in (Select id from movies where listname = ? COLLATE NOCASE)"}, Args: []interface{}{lists[idxlist].Name}})
		case "feeds":
			importnewmoviessingle(cfgp, lists[idxlist].Name)
		default:
			// other stuff
		}
	}
	lists = nil
	unique := unique(&logger.InStringArrayStruct{Arr: qualis})
	for idxuni := range unique {
		switch job {
		case "rss":
			searcher.SearchMovieRSS(cfgp, unique[idxuni])
		}
	}
	unique = nil
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		endjobhistory(uint(dbid))
	}
	logger.Log.GlobalLogger.Info(jobended, zap.String("Job", jobName))
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

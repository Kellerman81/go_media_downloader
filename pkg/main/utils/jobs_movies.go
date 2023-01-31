package utils

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
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

const queryidunmatched = "select id from movie_file_unmatcheds where filepath = ? and listname = ?"
const queryrootpathmovies = "select rootpath from movies where id = ?"
const querycountfilesmovies = "select count() from movie_files where location = ? and movie_id = ?"
const queryidmoviesbyimdb = "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE"
const queryidmoviesbylistname = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
const queryimdbmoviesbyid = "select imdb_id from dbmovies where id = ?"
const jobstarted = "Started Job"
const jobended = "Ended Job"

var errNotAdded = errors.New("not added")
var lastMoviesStructure string

func jobImportMovieParseV2(imp *importstruct) {
	defer imp.close()
	m := parser.NewFileParser(filepath.Base(imp.path), false, "movie")
	defer m.Close()
	m.Title = strings.TrimSpace(m.Title)

	//keep list empty for auto detect list since the default list is in the listconfig!
	parser.GetDbIDs("movie", m, imp.cfgp, "", true)
	if m.MovieID != 0 && m.Listname != "" {
		imp.listname = m.Listname
	}

	if imp.listname == "" {
		return
	}
	if !config.Check("quality_" + imp.cfgp.ListsMap[imp.listname].TemplateQuality) {
		logger.Log.GlobalLogger.Error("Quality for List: " + imp.listname + " not found")
		return
	}
	var counter int
	var err error
	if m.MovieID == 0 && imp.listname != "" && imp.addfound {
		if m.Imdb != "" {
			if m.DbmovieID == 0 {
				m.DbmovieID = importfeed.JobImportMovies(m.Imdb, imp.cfgp, imp.listname, true)
			}
			if m.MovieID == 0 {
				database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbyimdb, Args: []interface{}{m.Imdb, imp.listname}}, &m.MovieID)
			}
		}
	}
	if m.MovieID == 0 && imp.listname != "" && imp.addfound {
		dbmovie, found, found1 := importfeed.MovieFindDbIDByTitle(m.Imdb, m.Title, m.Year, "rss", imp.cfgp.Data[0].AddFound)
		if (found || found1) && imp.listname == imp.cfgp.Data[0].AddFoundList && imp.cfgp.Data[0].AddFound {
			if database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{dbmovie, imp.listname}}, &m.MovieID) != nil {
				if m.Imdb == "" {
					database.QueryColumn(&database.Querywithargs{QueryString: queryimdbmoviesbyid, Args: []interface{}{dbmovie}}, &m.Imdb)
				}
				if m.Imdb != "" {
					importfeed.JobImportMovies(m.Imdb, imp.cfgp, imp.listname, true)
					database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{dbmovie, imp.listname}}, &m.MovieID)
				}
				if m.MovieID == 0 {
					err = errNotAdded
				}
			}
		} else if imp.listname == imp.cfgp.Data[0].AddFoundList && imp.cfgp.Data[0].AddFound {
			imdbID, _, _ := importfeed.MovieFindImdbIDByTitle(m.Title, m.Year, "rss", imp.cfgp.Data[0].AddFound)
			if m.DbmovieID == 0 {
				m.DbmovieID = importfeed.JobImportMovies(imdbID, imp.cfgp, imp.listname, true)
			}
			if m.MovieID == 0 {
				database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{dbmovie, imp.listname}}, &m.MovieID)
			}
			if m.MovieID == 0 {
				err = errNotAdded
			}
		}
	}
	if err != nil {
		return
	}
	if m.MovieID == 0 {
		var id uint
		database.QueryColumn(&database.Querywithargs{QueryString: queryidunmatched, Args: []interface{}{imp.path, imp.listname}}, &id)
		if id == 0 {
			database.InsertStatic(&database.Querywithargs{QueryString: "Insert into movie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", Args: []interface{}{imp.listname, imp.path, sql.NullTime{Time: time.Now(), Valid: true}, buildparsedstring(m)}})
		} else {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movie_file_unmatcheds SET last_checked = ? where id = ?", Args: []interface{}{sql.NullTime{Time: time.Now(), Valid: true}, id}})
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movie_file_unmatcheds SET parsed_data = ? where id = ?", Args: []interface{}{buildparsedstring(m), id}})
		}
		return
	}

	parser.GetPriorityMap(m, imp.cfgp, imp.cfgp.ListsMap[imp.listname].TemplateQuality, true, false)
	err = parser.ParseVideoFile(m, imp.path, imp.cfgp.ListsMap[imp.listname].TemplateQuality)
	if err != nil {
		logger.Log.GlobalLogger.Error("Parse failed", zap.String("file", imp.path), zap.Error(err))
		return
	}
	database.QueryColumn(&database.Querywithargs{QueryString: querycountfilesmovies, Args: []interface{}{imp.path, m.MovieID}}, &counter)
	if counter >= 1 {
		return
	}
	var okint int
	if m.Priority >= parser.NewCutoffPrio(imp.cfgp, imp.cfgp.ListsMap[imp.listname].TemplateQuality) {
		okint = 1
	}
	var rootpath string
	database.QueryColumn(&database.Querywithargs{QueryString: queryrootpathmovies, Args: []interface{}{m.MovieID}}, &rootpath)

	if rootpath == "" && m.MovieID != 0 {
		updateRootpath(imp.path, "movies", m.MovieID, imp.cfgp)
	}

	database.InsertNamed("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :movie_id, :dbmovie_id, :height, :width)",
		database.MovieFile{
			Location:       imp.path,
			Filename:       filepath.Base(imp.path),
			Extension:      filepath.Ext(imp.path),
			QualityProfile: imp.cfgp.ListsMap[imp.listname].TemplateQuality,
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
	if imp.updatemissing {
		updatemoviesmissing(0, m.MovieID)

		updatemoviesreached(okint, m.MovieID)
	}

	database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from movie_file_unmatcheds where filepath = ?", Args: []interface{}{imp.path}})
}

func updatemoviesreached(reached int, dbmovieid uint) {
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set quality_reached = ? where id = ?", Args: []interface{}{reached, dbmovieid}})
}

func updatemoviesmissing(missing int, dbmovieid uint) {
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set missing = ? where id = ?", Args: []interface{}{missing, dbmovieid}})
}

func getMissingIMDBMoviesV2(templatelist string) (*feedResults, error) {
	cfglist := config.Cfg.Lists[templatelist]
	defer cfglist.Close()
	if cfglist.URL == "" {
		logger.Log.GlobalLogger.Error("Failed to get url")
		return nil, errNoListOther
	}
	req, err := http.NewRequest("GET", cfglist.URL, nil)
	if err != nil {
		logger.Log.GlobalLogger.Error("Failed to read CSV from", zap.String("url", cfglist.URL))
		return nil, errNoListRead
	}
	resp, err := logger.WebClient.Do(req)
	if err != nil || resp == nil {
		logger.Log.GlobalLogger.Error("Failed to read CSV from", zap.String("url", cfglist.URL))
		return nil, errNoListRead
	}

	defer resp.Body.Close()

	parserimdb := csv.NewReader(resp.Body)
	parserimdb.ReuseRecord = true

	var d feedResults
	cnt, ok := logger.GlobalCounter[cfglist.URL]

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
			d.Close()
			parserimdb = nil
			record = nil
			return nil, errors.New("list csv import error")
		}
		if record[1] == "" || record[1] == "Const" || record[1] == "tconst" {
			logger.Log.GlobalLogger.Warn("skipped row", zap.String("imdb", record[1]))
			continue
		}
		d.Movies = append(d.Movies, record[1])
	}
	record = nil

	logger.GlobalMu.Lock()
	logger.GlobalCounter[cfglist.URL] = len(d.Movies)
	logger.GlobalMu.Unlock()
	parserimdb = nil
	return &d, nil
}

func getTraktUserPublicMovieList(templatelist string) (*feedResults, error) {
	if !config.Check("list_" + templatelist) {
		return nil, errNoList
	}

	cfglist := config.Cfg.Lists[templatelist]
	defer cfglist.Close()

	if cfglist.TraktUsername == "" || cfglist.TraktListName == "" {
		return nil, errors.New("no username")
	}
	if cfglist.TraktListType == "" {
		return nil, errors.New("not show or movie")
	}
	data, err := apiexternal.TraktAPI.GetUserList(cfglist.TraktUsername, cfglist.TraktListName, cfglist.TraktListType, cfglist.Limit)
	if err != nil {
		logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", cfglist.TraktListName))
		return nil, errNoListRead
	}
	d := feedResults{Movies: []string{}}
	for idx := range data.Entries {
		d.Movies = append(d.Movies, data.Entries[idx].Movie.Ids.Imdb)
	}

	data.Close()
	return &d, nil
}

func importnewmoviessingle(cfgp *config.MediaTypeConfig, listname string) {
	if listname == "" {
		return
	}
	logger.Log.GlobalLogger.Debug("get feeds for ", zap.Stringp("config", &cfgp.NamePrefix), zap.Stringp("Listname", &listname))

	feed, err := feeds(cfgp, listname)
	if err != nil {
		return
	}
	defer feed.Close()

	lenfeed := len(feed.Movies)
	if lenfeed == 0 {
		return
	}

	var foundmovie bool

	var dbmovies []database.DbstaticOneStringOneInt
	var movies []database.DbstaticOneStringOneInt

	if lenfeed > 900 {
		database.QueryStaticColumnsOneStringOneInt(false, database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from dbmovies"}), &database.Querywithargs{QueryString: "select imdb_id, id from dbmovies"}, &dbmovies)

		database.QueryStaticColumnsOneStringOneInt(false, database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from movies"}), &database.Querywithargs{QueryString: "select lower(listname), dbmovie_id from movies"}, &movies)
	} else {
		imdbArgs := make([]interface{}, lenfeed)
		for idx := range feed.Movies {
			imdbArgs[idx] = feed.Movies[idx]
		}
		database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{DontCache: true, QueryString: "select imdb_id, id from dbmovies where imdb_id IN (?" + strings.Repeat(",?", len(imdbArgs)-1) + ")", Args: imdbArgs}, &dbmovies)

		database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{DontCache: true, QueryString: "select lower(movies.listname), movies.dbmovie_id from movies inner join dbmovies on dbmovies.id = movies.dbmovie_id where dbmovies.imdb_id IN (?" + strings.Repeat(",?", len(imdbArgs)-1) + ")", Args: imdbArgs}, &movies)
		imdbArgs = nil
	}
	imdbids := logger.InStringArrayStruct{Arr: make([]string, 0, lenfeed)}
	defer imdbids.Close()
	var intid int
	lenignore := len(cfgp.ListsMap[listname].IgnoreMapLists)
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
		if intid == 0 && importfeed.AllowMovieImport(feed.Movies[idxmovie], cfgp.ListsMap[listname].TemplateList) {
			imdbids.Arr = append(imdbids.Arr, feed.Movies[idxmovie])
			continue
		}
		if intid == 0 {
			logger.Log.GlobalLogger.Debug("not allowed movie", zap.String("imdb", feed.Movies[idxmovie]))
			continue
		}
		for idxsmovie := range movies {
			if movies[idxsmovie].Num != intid {
				continue
			}
			if strings.EqualFold(movies[idxsmovie].Str, listname) {
				foundmovie = true
				//logger.Log.GlobalLogger.Debug("not allowed movie1", zap.String("imdb", feed.Movies[idxmovie]))
				break
			}
			if lenignore == 0 {
				continue
			}
			for idx := range cfgp.ListsMap[listname].IgnoreMapLists {
				if strings.EqualFold(movies[idxsmovie].Str, cfgp.ListsMap[listname].IgnoreMapLists[idx]) {
					foundmovie = true
					//logger.Log.GlobalLogger.Debug("not allowwed movie2", zap.String("imdb", feed.Movies[idxmovie]))
					break
				}
			}
			if foundmovie {
				break
			}
		}
		if !foundmovie && importfeed.AllowMovieImport(feed.Movies[idxmovie], cfgp.ListsMap[listname].TemplateList) {
			imdbids.Arr = append(imdbids.Arr, feed.Movies[idxmovie])
			continue
		}
		if !foundmovie {
			logger.Log.GlobalLogger.Debug("not allowed movie", zap.String("imdb", feed.Movies[idxmovie]))
		}
	}
	workermovieimport(cfgp, listname, &imdbids)
	imdbids.Close()
	dbmovies = nil
	movies = nil
}

func workermovieimport(cfgp *config.MediaTypeConfig, listname string, imdbids *logger.InStringArrayStruct) {
	workergroup := logger.WorkerPools["Metadata"].Group()
	for idxmovie := range imdbids.Arr {
		imdbID := imdbids.Arr[idxmovie]
		logger.Log.GlobalLogger.Info("Import Movie ", zap.Int("row", idxmovie), zap.Stringp("imdb", &imdbID))
		workergroup.Submit(func() {
			importfeed.JobImportMovies(imdbID, cfgp, listname, true)
		})
	}
	workergroup.Wait()
}

func checkmissingmoviessingle(listname string) {
	var filesfound []string
	database.QueryStaticStringArray(false, database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)"}), &database.Querywithargs{QueryString: "select location from movie_files where movie_id in (select id from movies where listname = ?)", Args: []interface{}{listname}}, &filesfound)
	if len(filesfound) >= 1 {
		for idx := range filesfound {
			//workergroup.Submit(func() {
			jobImportFileCheck(filesfound[idx], "movie")
			//})
		}
	}
	filesfound = nil
}

func checkmissingmoviesflag(listname string) {
	var movies []database.Movie
	database.QueryMovies(&database.Querywithargs{Query: database.Query{Select: "id, missing", Where: "listname = ?"}, Args: []interface{}{listname}}, &movies)

	var counter int
	querycount := "select count() from movie_files where movie_id = ?"
	for idxmovie := range movies {
		database.QueryColumn(&database.Querywithargs{QueryString: querycount, Args: []interface{}{movies[idxmovie].ID}}, &counter)
		if counter >= 1 && movies[idxmovie].Missing {
			updatemoviesmissing(0, movies[idxmovie].ID)
			continue
		}
		if counter == 0 && !movies[idxmovie].Missing {
			updatemoviesmissing(1, movies[idxmovie].ID)
		}
	}
	movies = nil
}

func checkreachedmoviesflag(cfgp *config.MediaTypeConfig, listname string) {
	var movies []database.Movie
	database.QueryMovies(&database.Querywithargs{Query: database.Query{Select: "id, quality_reached, quality_profile", Where: "listname = ?"}, Args: []interface{}{listname}}, &movies)
	var reached bool
	for idxepi := range movies {
		if !config.Check("quality_" + movies[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error(fmt.Sprintf("Quality for Movie: %d not found", movies[idxepi].ID))
			continue
		}

		reached = false
		if searcher.GetHighestMoviePriorityByFiles(false, true, movies[idxepi].ID, cfgp, movies[idxepi].QualityProfile) >= parser.NewCutoffPrio(cfgp, movies[idxepi].QualityProfile) {
			reached = true
		}
		if movies[idxepi].QualityReached && !reached {
			updatemoviesreached(0, movies[idxepi].ID)
			continue
		}

		if !movies[idxepi].QualityReached && reached {
			updatemoviesreached(1, movies[idxepi].ID)
		}
	}
	movies = nil
}

func RefreshMovies() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshmoviesquery("select imdb_id, (Select listname from movies where dbmovie_id=dbmovies.id limit 1) from dbmovies", database.CountRowsStaticNoError(&database.Querywithargs{QueryString: "select count() from dbmovies"}))
}

func RefreshMovie(id string) {
	refreshmoviesquery("select imdb_id, (Select listname from movies where dbmovie_id=dbmovies.id limit 1) from dbmovies where id = ?", 1, id)
}

func RefreshMoviesInc() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshmoviesquery("select imdb_id, (Select listname from movies where dbmovie_id=dbmovies.id limit 1) from dbmovies order by updated_at desc limit 100", 100)
}

func refreshmoviesquery(query string, count int, args ...interface{}) {
	var dbmovies []database.DbstaticTwoString
	database.QueryStaticColumnsTwoString(false, count, &database.Querywithargs{QueryString: query, Args: args}, &dbmovies)
	var cfgp config.MediaTypeConfig
	var oldlistname string
	for idxmovie := range dbmovies {
		logger.Log.GlobalLogger.Info("Refresh Movie ", zap.Int("row", idxmovie), zap.Int("of rows", len(dbmovies)), zap.Stringp("imdb", &dbmovies[idxmovie].Str1))
		if oldlistname != dbmovies[idxmovie].Str2 {
			cfgp.Close()
			cfgp = config.Cfg.Media[config.FindconfigTemplateOnList("movie_", dbmovies[idxmovie].Str2)]
			oldlistname = dbmovies[idxmovie].Str2
		}
		importfeed.JobImportMovies(dbmovies[idxmovie].Str1, &cfgp, dbmovies[idxmovie].Str2, false)
	}
	cfgp.Close()
	dbmovies = nil
}

func MoviesAllJobs(job string, force bool) {
	for idx := range config.Cfg.Movies {
		MoviesSingleJobs(job, config.Cfg.Movies[idx].NamePrefix, "", force)
	}
}

func MoviesSingleJobs(job string, cfgpstr string, listname string, force bool) {
	cfgp := config.Cfg.Media[cfgpstr]
	defer cfgp.Close()
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
	dbresult, _ := insertjobhistory(&database.JobHistory{JobType: job, JobGroup: cfgp.NamePrefix, JobCategory: "Movie", Started: sql.NullTime{Time: time.Now().In(logger.TimeZone), Valid: true}})
	logger.Log.GlobalLogger.Info(jobstarted, zap.Stringp("Job", &jobName))
	searchmissingIncremental := cfgp.SearchmissingIncremental
	searchupgradeIncremental := cfgp.SearchupgradeIncremental
	if searchmissingIncremental == 0 {
		searchmissingIncremental = 20
	}
	if searchupgradeIncremental == 0 {
		searchupgradeIncremental = 20
	}

	var searchmovie, searchtitle, searchmissing bool
	var searchinterval int
	switch job {
	case "datafull":
		getNewFilesMap(&cfgp, "")
	case "searchmissingfull":
		searchmovie = true
		searchmissing = true
	case "searchmissinginc":
		searchmovie = true
		searchmissing = true
		searchinterval = searchmissingIncremental
	case "searchupgradefull":
		searchmovie = true
	case "searchupgradeinc":
		searchmovie = true
		searchinterval = searchupgradeIncremental
	case "searchmissingfulltitle":
		searchmovie = true
		searchmissing = true
		searchtitle = true
	case "searchmissinginctitle":
		searchmovie = true
		searchmissing = true
		searchtitle = true
		searchinterval = searchmissingIncremental
	case "searchupgradefulltitle":
		searchmovie = true
		searchtitle = true
	case "searchupgradeinctitle":
		searchmovie = true
		searchtitle = true
		searchinterval = searchupgradeIncremental
	case "structure":
		if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.Data[0].TemplatePath))
			return
		}

		for idx := range cfgp.DataImport {
			if !config.Check("path_" + cfgp.DataImport[idx].TemplatePath) {
				logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.DataImport[idx].TemplatePath))
				continue
			}
			if lastMoviesStructure == config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].Path {
				time.Sleep(time.Duration(15) * time.Second)
			}
			lastMoviesStructure = config.Cfg.Paths[cfgp.DataImport[idx].TemplatePath].Path
			structure.OrganizeFolders("movie", cfgp.DataImport[idx].TemplatePath, cfgp.Data[0].TemplatePath, &cfgp)
		}
	}
	if searchmovie {
		searcher.SearchMovie(&cfgp, searchmissing, searchinterval, searchtitle)
	}

	if job == "data" || job == "checkmissing" || job == "checkmissingflag" || job == "checkreachedflag" || job == "clearhistory" || job == "feeds" || job == "rss" {
		var qualis logger.InStringArrayStruct

		for _, list := range getjoblists(&cfgp, listname) {
			if !logger.InStringArray(list.TemplateQuality, &qualis) {
				qualis.Arr = append(qualis.Arr, list.TemplateQuality)
			}
			switch job {
			case "data":
				getNewFilesMap(&cfgp, list.Name)
			case "checkmissing":
				checkmissingmoviessingle(list.Name)
			case "checkmissingflag":
				checkmissingmoviesflag(list.Name)
			case "checkreachedflag":
				checkreachedmoviesflag(&cfgp, list.Name)
			case "clearhistory":
				database.DeleteRow("movie_histories", &database.Querywithargs{Query: database.Query{Where: "movie_id in (Select id from movies where listname = ? COLLATE NOCASE)"}, Args: []interface{}{list.Name}})
			case "feeds":
				importnewmoviessingle(&cfgp, list.Name)
			default:
				// other stuff
			}
		}
		if job == "rss" {
			for idxqual := range qualis.Arr {
				switch job {
				case "rss":
					searcher.SearchMovieRSS(&cfgp, qualis.Arr[idxqual])
				}
			}
		}
		qualis.Close()
	}
	if dbresult != nil {
		dbid, _ := dbresult.LastInsertId()
		endjobhistory(dbid)
	}
	logger.Log.GlobalLogger.Info(jobended, zap.Stringp("Job", &jobName))
}

func unique(s *logger.InStringArrayStruct) *logger.InStringArrayStruct {
	inResult := logger.InStringArrayStruct{Arr: s.Arr[:0]}
	for idx := range s.Arr {
		if !logger.InStringArrayCaseSensitive(s.Arr[idx], &inResult) {
			inResult.Arr = append(inResult.Arr, s.Arr[idx])
		}
	}
	return &inResult
}

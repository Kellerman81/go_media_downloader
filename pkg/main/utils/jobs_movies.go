package utils

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"go.uber.org/zap"
	"golang.org/x/exp/slices"
)

const queryidunmatched = "select id from movie_file_unmatcheds where filepath = ? and listname = ?"
const queryrootpathmovies = "select rootpath from movies where id = ?"
const querycountfilesmovies = "select count() from movie_files where location = ? and movie_id = ?"
const queryidmoviesbyimdb = "select id from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ? COLLATE NOCASE"
const queryidmoviesbylistname = "select id from movies where dbmovie_id = ? and listname = ? COLLATE NOCASE"
const queryimdbmoviesbyid = "select imdb_id from dbmovies where id = ?"
const jobstarted = "Started Job"
const jobended = "Ended Job"

func jobImportMovieParseV2(path string, updatemissing bool, cfgp *config.MediaTypeConfig, listname string, addfound bool) {
	m := parser.NewFileParser(filepath.Base(path), false, "movie")
	defer m.Close()
	m.Title = strings.TrimSpace(m.Title)

	//keep list empty for auto detect list since the default list is in the listconfig!
	parser.GetDbIDs("movie", m, cfgp, "", true)
	if m.MovieID != 0 && m.Listname != "" {
		listname = m.Listname
	}

	if listname == "" {
		return
	}
	templatequality := cfgp.ListsMap[listname].TemplateQuality
	if !config.Check("quality_" + templatequality) {
		logger.Log.GlobalLogger.Error("Quality for List: " + listname + " not found")
		return
	}

	var counter int
	var err error
	if m.MovieID == 0 && listname != "" && addfound {
		if m.Imdb != "" {
			if m.DbmovieID == 0 {
				m.DbmovieID = importfeed.JobImportMovies(m.Imdb, cfgp, listname, true)
			}
			if m.MovieID == 0 {
				database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbyimdb, Args: []interface{}{m.Imdb, listname}}, &m.MovieID)
			}
		}
	}
	if m.MovieID == 0 && listname != "" && addfound {
		addfound := cfgp.Data[0].AddFound
		addFoundList := cfgp.Data[0].AddFoundList
		dbmovie, found, found1 := importfeed.MovieFindDbIDByTitle(m.Imdb, m.Title, m.Year, "rss", addfound)
		if (found || found1) && listname == addFoundList && addfound {
			if database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{dbmovie, listname}}, &m.MovieID) != nil {
				if m.Imdb == "" {
					database.QueryColumn(&database.Querywithargs{QueryString: queryimdbmoviesbyid, Args: []interface{}{dbmovie}}, &m.Imdb)
				}
				if m.Imdb != "" {
					importfeed.JobImportMovies(m.Imdb, cfgp, listname, true)
					database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{dbmovie, listname}}, &m.MovieID)
				}
				if m.MovieID == 0 {
					return
				}
			}
		} else if listname == addFoundList && addfound {
			imdbID, _, _ := importfeed.MovieFindImdbIDByTitle(m.Title, m.Year, "rss", addfound)
			if m.DbmovieID == 0 {
				m.DbmovieID = importfeed.JobImportMovies(imdbID, cfgp, listname, true)
			}
			if m.MovieID == 0 {
				database.QueryColumn(&database.Querywithargs{QueryString: queryidmoviesbylistname, Args: []interface{}{dbmovie, listname}}, &m.MovieID)
			}
			if m.MovieID == 0 {
				return
			}
		}
	}
	if err != nil {
		return
	}
	if m.MovieID == 0 {
		var id uint
		database.QueryColumn(&database.Querywithargs{QueryString: queryidunmatched, Args: []interface{}{path, listname}}, &id)
		if id == 0 {
			logger.InsertStringsArrCache("movie_file_unmatcheds_cached", path)
			database.InsertStatic(&database.Querywithargs{QueryString: "Insert into movie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (:listname, :filepath, :last_checked, :parsed_data)", Args: []interface{}{listname, path, logger.SqlTimeGetNow(), buildparsedstring(m)}})
		} else {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movie_file_unmatcheds SET last_checked = ? where id = ?", Args: []interface{}{logger.SqlTimeGetNow(), id}})
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movie_file_unmatcheds SET parsed_data = ? where id = ?", Args: []interface{}{buildparsedstring(m), id}})
		}
		return
	}
	cfgqual := config.Cfg.Quality[templatequality]
	defer cfgqual.Close()

	parser.GetPriorityMapQual(m, cfgp, &cfgqual, true, false)
	err = parser.ParseVideoFile(m, path, templatequality)
	if err != nil {
		logger.Log.GlobalLogger.Error("Parse failed", zap.String("file", path), zap.Error(err))
		return
	}
	database.QueryColumn(&database.Querywithargs{QueryString: querycountfilesmovies, Args: []interface{}{path, m.MovieID}}, &counter)
	if counter >= 1 {
		return
	}
	var okint int
	if m.Priority >= parser.NewCutoffPrio(cfgp, &cfgqual) {
		okint = 1
	}
	var rootpath string
	database.QueryColumn(&database.Querywithargs{QueryString: queryrootpathmovies, Args: []interface{}{m.MovieID}}, &rootpath)

	if rootpath == "" && m.MovieID != 0 {
		updateRootpath(path, "movies", m.MovieID, cfgp)
	}

	logger.InsertStringsArrCache("movie_files_cached", path)
	database.InsertNamed("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :movie_id, :dbmovie_id, :height, :width)",
		database.MovieFile{
			Location:       path,
			Filename:       filepath.Base(path),
			Extension:      filepath.Ext(path),
			QualityProfile: templatequality,
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

		updatemoviesreached(okint, m.MovieID)
	}

	var id uint
	database.QueryColumn(&database.Querywithargs{QueryString: "select id from movie_file_unmatcheds where filepath = ?", Args: []interface{}{path}}, &id)
	if id != 0 {
		logger.DeleteFromStringsArrCache("movie_file_unmatcheds_cached", path)
		database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from movie_file_unmatcheds where filepath = ?", Args: []interface{}{path}})
	}
}

func updatemoviesreached(reached int, dbmovieid uint) {
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set quality_reached = ? where id = ?", Args: []interface{}{reached, dbmovieid}})
}

func updatemoviesmissing(missing int, dbmovieid uint) {
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set missing = ? where id = ?", Args: []interface{}{missing, dbmovieid}})
}

func getMissingIMDBMoviesV2(templatelist string, cfglist *config.ListsConfig) (*feedResults, error) {
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

	d := new(feedResults)
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
			continue
		}
		d.Movies = append(d.Movies, record[1])
	}
	record = nil

	logger.GlobalMu.Lock()
	logger.GlobalCounter[cfglist.URL] = len(d.Movies)
	logger.GlobalMu.Unlock()
	parserimdb = nil
	return d, nil
}

func getTraktUserPublicMovieList(templatelist string, cfglist *config.ListsConfig) (*feedResults, error) {
	if !config.Check("list_" + templatelist) {
		return nil, errNoList
	}

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

	cfglist := config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList]
	feed, err := feeds(cfgp, listname, &cfglist)
	if err != nil {
		cfglist.Close()
		return
	}
	lenfeed := len(feed.Movies)
	if lenfeed == 0 {
		cfglist.Close()
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
	imdbids := &logger.InStringArrayStruct{Arr: make([]string, 0, lenfeed)}
	var intid int
	ignorelists := cfgp.ListsMap[listname].IgnoreMapLists
	lenignore := len(ignorelists)
	for _, feedentry := range feed.Movies {
		if feedentry == "" {
			continue
		}
		foundmovie = false

		intid = slices.IndexFunc(dbmovies, func(c database.DbstaticOneStringOneInt) bool { return c.Str == feedentry })

		// for idxsdbmovie := range dbmovies {
		// 	if dbmovies[idxsdbmovie].Str == feedentry {
		// 		intid = dbmovies[idxsdbmovie].Num
		// 		break
		// 	}
		// }
		if intid == -1 && importfeed.AllowMovieImport(feedentry, &cfglist) {
			imdbids.Arr = append(imdbids.Arr, feedentry)
			continue
		}
		if intid == -1 {
			logger.Log.GlobalLogger.Debug("not allowed movie", zap.String("imdb", feedentry))
			continue
		}
		intid = dbmovies[intid].Num
		for idxsmovie := range movies {
			if movies[idxsmovie].Num != intid {
				continue
			}
			if strings.EqualFold(movies[idxsmovie].Str, listname) {
				foundmovie = true
				//logger.Log.GlobalLogger.Debug("not allowed movie1", zap.String("imdb", feedentry))
				break
			}
			if lenignore == 0 {
				continue
			}
			if !foundmovie {
				foundmovie = slices.ContainsFunc(ignorelists, func(c string) bool { return strings.EqualFold(c, movies[idxsmovie].Str) })
			}

			// for idx := range ignorelists {
			// 	if strings.EqualFold(movies[idxsmovie].Str, ignorelists[idx]) {
			// 		foundmovie = true
			// 		//logger.Log.GlobalLogger.Debug("not allowwed movie2", zap.String("imdb", feedentry))
			// 		break
			// 	}
			// }
			if foundmovie {
				break
			}
		}
		if !foundmovie && importfeed.AllowMovieImport(feedentry, &cfglist) {
			imdbids.Arr = append(imdbids.Arr, feedentry)
			continue
		}
		if !foundmovie {
			logger.Log.GlobalLogger.Debug("not allowed movie", zap.String("imdb", feedentry))
		}
	}
	cfglist.Close()
	workermovieimport(cfgp, listname, imdbids)
	imdbids.Close()
	feed.Close()
	ignorelists = nil

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
		if searcher.GetHighestMoviePriorityByFilesGetQual(false, true, movies[idxepi].ID, cfgp, movies[idxepi].QualityProfile) >= parser.NewCutoffPrioGetQual(cfgp, movies[idxepi].QualityProfile) {
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
	var cfgp *config.MediaTypeConfig
	var oldlistname string
	for idxmovie := range dbmovies {
		logger.Log.GlobalLogger.Info("Refresh Movie ", zap.Int("row", idxmovie), zap.Int("of rows", len(dbmovies)), zap.Stringp("imdb", &dbmovies[idxmovie].Str1))
		if oldlistname != dbmovies[idxmovie].Str2 {
			cfgp.Close()
			cfgp = config.FindconfigTemplateOnList("movie_", dbmovies[idxmovie].Str2)
			oldlistname = dbmovies[idxmovie].Str2
		}
		importfeed.JobImportMovies(dbmovies[idxmovie].Str1, cfgp, dbmovies[idxmovie].Str2, false)
	}
	cfgp.Close()
	dbmovies = nil
}

func MoviesAllJobs(job string, force bool) {
	for idx := range config.Cfg.Movies {
		SingleJobs("movie", job, config.Cfg.Movies[idx].NamePrefix, "", force)
	}
}

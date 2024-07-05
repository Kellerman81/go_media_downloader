package importfeed

import (
	"errors"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

var (
	errIgnoredMovie          = errors.New("movie ignored")
	errJobRunning            = errors.New("job running")
	errVoteLow               = errors.New("error vote count too low")
	errVoteRateLow           = errors.New("error average vote too low")
	errIncludedGenreNotFound = errors.New("included genre not found")
	importJobRunning         = logger.NewSynchedMap[struct{}](10)
	defaultproviders         = []string{"imdb", "tmdb", "omdb"}
)

//var importJobRunning string

// checkimdbyearsingle checks if the year from IMDB matches the year
// in the FileParser, allowing a 1 year difference. It returns true if
// the years match or are within 1 year difference.
func checkimdbyearsingle(m *database.ParseInfo, cfgp *config.MediaTypeConfig, imdb string, haveyear uint16) bool {
	if haveyear == 0 || m.Year == 0 || imdb == "" {
		return false
	}
	if haveyear == m.Year {
		return true
	}

	if haveyear == m.Year+1 || haveyear == m.Year-1 {
		if database.GetMediaQualityConfig(cfgp, MovieFindDBIDByImdb(imdb)).CheckYear1 {
			return true
		}
	}
	return false
}

// MovieFindDBIDByImdb looks up the database ID for a movie by its IMDB ID.
// It takes a string containing the IMDB ID and returns the uint database ID.
// It first checks the cache if enabled, otherwise queries the database directly.
// If no match is found, it returns 0.
func MovieFindDBIDByImdb(imdb string) uint {
	if imdb == "" {
		return 0
	}
	logger.AddImdbPrefixP(&imdb)
	if config.SettingsGeneral.UseMediaCache {
		return database.CacheThreeStringIntIndexFunc(logger.CacheDBMovie, imdb)
	}
	return database.Getdatarow1[uint](false, "select id from dbmovies where imdb_id = ?", imdb) //sqlpointerr
}

// MovieFindImdbIDByTitle searches for a movie's IMDB ID by its title.
// It first searches the database and caches by title and slugified title.
// If not found, it can search external APIs based on config settings.
// It populates the ParseInfo struct with the found IMDB ID and other data.
func MovieFindImdbIDByTitle(addifnotfound bool, m *database.ParseInfo, cfgp *config.MediaTypeConfig) {
	if m.Title == "" {
		m.Cleanimdbdbmovie()
		return
	}

	m.TempTitle = m.Title
	m.Findmoviedbidbytitle()
	if m.DbmovieID == 0 {
		m.TempTitle = logger.StringToSlug(m.Title)
		m.Findmoviedbidbytitle()
	}
	if m.DbmovieID != 0 {
		if config.SettingsGeneral.UseMediaCache {
			m.CacheThreeStringIntIndexFuncGetImdb(logger.CacheDBMovie, m.DbmovieID)
			return
		}
		_ = database.Scanrows1dyn(false, "select imdb_id from dbmovies where id = ?", &m.Imdb, &m.DbmovieID)
		return
	}

	if !addifnotfound {
		m.Cleanimdbdbmovie()
		return
	}
	slug := logger.StringToSlug(m.Title)

	for _, provider := range getsearchprovider(false) {
	breakswitch:
		switch provider {
		case logger.StrImdb:
			if config.SettingsGeneral.MovieMetaSourceImdb {
				arr := database.GetrowsNsize[database.DbstaticOneStringOneInt](true, "select count() from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", "select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", &m.Title, &m.Title, &slug)
				for idx := range arr {
					if checkimdbyearsingle(m, cfgp, arr[idx].Str, uint16(arr[idx].Num)) {
						m.Imdb = arr[idx].Str
						m.DbmovieID = MovieFindDBIDByImdb(arr[idx].Str)
						//clear(arr)
						break breakswitch
					}
				}
				//clear(arr)

				var haveyear uint16

				arr2 := database.GetrowsN[string](true, database.QueryImdbAkaCountByTitleSlug(m.Title, slug), "select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?", &m.Title, &slug)
				for idx := range arr2 {
					_ = database.Scanrows1dyn(true, "select start_year from imdb_titles where tconst = ?", &haveyear, &arr2[idx])
					if checkimdbyearsingle(m, cfgp, arr2[idx], haveyear) {
						m.Imdb = arr2[idx]
						m.DbmovieID = MovieFindDBIDByImdb(arr2[idx])
						//clear(arr2)
						break breakswitch
					}
				}
				//clear(arr2)
				m.Cleanimdbdbmovie()
			}
		case "tmdb":
			if config.SettingsGeneral.MovieMetaSourceTmdb {
				if m.Title == "" {
					m.Cleanimdbdbmovie()
					break breakswitch
				}

				tbl, err := apiexternal.SearchTmdbMovie(m.Title)
				if err != nil {
					m.Cleanimdbdbmovie()
					break breakswitch
				}
				var haveyear uint16
				for idx := range tbl.Results {
					_ = database.Scanrows1dyn(false, "select imdb_id from dbmovies where moviedb_id = ?", &m.Imdb, &tbl.Results[idx].ID)
					if m.Imdb == "" {
						moviedbexternal, err := apiexternal.GetTmdbMovieExternal(tbl.Results[idx].ID)
						if err != nil {
							continue
						}
						m.Imdb = moviedbexternal.ImdbID
						_ = database.Scanrows1dyn(false, "select id from dbmovies where imdb_id = ?", &m.DbmovieID, &m.Imdb)
						//clear(tbl.Results)
						break breakswitch
					}
					if m.Imdb == "" {
						continue
					}
					_ = database.Scanrows1dyn(true, "select start_year from imdb_titles where tconst = ?", &haveyear, &m.Imdb)
					if checkimdbyearsingle(m, cfgp, m.Imdb, haveyear) {
						_ = database.Scanrows1dyn(false, "select id from dbmovies where imdb_id = ?", &m.DbmovieID, &m.Imdb)
						//clear(tbl.Results)
						break breakswitch
					}
				}
				//clear(tbl.Results)
				m.Cleanimdbdbmovie()
			}
		case "omdb":
			if config.SettingsGeneral.MovieMetaSourceOmdb {
				tbl, err := apiexternal.SearchOmdbMovie(m.Title, "")
				if err != nil {
					m.Cleanimdbdbmovie()
					break breakswitch
				}
				for idx := range tbl.Search {
					if checkimdbyearsingle(m, cfgp, tbl.Search[idx].ImdbID, logger.StringToUInt16(tbl.Search[idx].Year)) {
						m.Imdb = tbl.Search[idx].ImdbID
						_ = database.Scanrows1dyn(false, "select id from dbmovies where imdb_id = ?", &m.DbmovieID, &tbl.Search[idx].ImdbID)
						//clear(tbl.Search)
						break breakswitch
					}
				}
				//clear(tbl.Search)
				m.Cleanimdbdbmovie()
			}
		default:
			continue
		}
		if m.Imdb != "" {
			return
		}
	}
	m.Cleanimdbdbmovie()
}

// JobImportMoviesByList imports or updates a list of movies in parallel.
// It takes a list of movie titles/IDs, an index, a media type config, a list ID,
// and a flag for whether to add new movies.
// It logs the import result for each movie.
func JobImportMoviesByList(wg *pool.SizedWaitGroup, entry string, idx int, cfgp *config.MediaTypeConfig, listid int8, addnew bool) {
	if wg != nil {
		defer logger.HandlePanic()
		defer wg.Done()
	}
	logger.LogDynamicany("info", "Import/Update Movie", &logger.StrMovie, entry, &logger.StrRow, idx) //logpointerr
	_, err := JobImportMovies(entry, cfgp, listid, addnew)
	if err != nil {
		if err.Error() != "movie ignored" {
			logger.LogDynamicany("error", "Import/Update Failed", err, &logger.StrImdb, entry) //logpointerr
		}
	}
}

// JobImportMovies imports a movie into the database and specified list
// given its IMDb ID. It handles checking if the movie exists, adding
// it if needed, updating metadata, and adding it to the target list.
func JobImportMovies(imdb string, cfgp *config.MediaTypeConfig, listid int8, addnew bool) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}
	if imdb == "" {
		return 0, logger.ErrImdbEmpty
	}
	if importJobRunning.Check(imdb) {
		return 0, errJobRunning
	}
	importJobRunning.Set(imdb, struct{}{})
	defer importJobRunning.Delete(imdb)

	var dbmovieadded bool

	dbid := MovieFindDBIDByImdb(imdb)
	checkdbmovie := dbid >= 1

	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(database.Getdatarow1[string](false, "select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id=?)", imdb))
	}
	if !checkdbmovie && addnew {
		if listid == -1 {
			return 0, logger.ErrCfgpNotFound
		}
		_, err := AllowMovieImport(imdb, cfgp.Lists[listid].CfgList)
		if err != nil {
			return 0, err
		}

		if database.Getdatarow1[uint](false, "select id from dbmovies where imdb_id = ?", imdb) == 0 { //sqlpointerr
			logger.LogDynamicany("debug", "Insert dbmovie for", &logger.StrJob, imdb)            //logpointerr
			dbresult, err := database.ExecNid("insert into dbmovies (Imdb_ID) VALUES (?)", imdb) //sqlpointerr
			if err != nil {
				return 0, err
			}
			dbid = uint(dbresult)
			dbmovieadded = true
		}
	}
	if dbid == 0 {
		dbid = MovieFindDBIDByImdb(imdb)
	}
	if dbid == 0 {
		return 0, logger.ErrNotFoundDbmovie
	}

	if dbmovieadded || !addnew {
		logger.LogDynamicany("debug", "Get metadata for", &logger.StrJob, imdb) //logpointerr
		dbmovie, err := database.GetDbmovieByID(&dbid)
		if err != nil {
			return 0, errIgnoredMovie
		}
		if dbmovie.ImdbID == "" {
			dbmovie.ImdbID = imdb
		}
		if !dbmovieadded && dbmovie.UpdatedAt.After(logger.TimeGetNow().Add(-1*time.Hour)) {
			//update only if updated more than an hour ago
			logger.LogDynamicany("debug", "Skipped update metadata for dbmovie ", &logger.StrJob, &dbmovie.ImdbID)
		} else {
			metadata.Getmoviemetadata(dbmovie, true)

			database.ExecN("update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?",
				&dbmovie.Title, &dbmovie.ReleaseDate, &dbmovie.Year, &dbmovie.Adult, &dbmovie.Budget, &dbmovie.Genres, &dbmovie.OriginalLanguage, &dbmovie.OriginalTitle, &dbmovie.Overview, &dbmovie.Popularity, &dbmovie.Revenue, &dbmovie.Runtime, &dbmovie.SpokenLanguages, &dbmovie.Status, &dbmovie.Tagline, &dbmovie.VoteAverage, &dbmovie.VoteCount, &dbmovie.TraktID, &dbmovie.MoviedbID, &dbmovie.ImdbID, &dbmovie.FreebaseMID, &dbmovie.FreebaseID, &dbmovie.FacebookID, &dbmovie.InstagramID, &dbmovie.TwitterID, &dbmovie.URL, &dbmovie.Backdrop, &dbmovie.Poster, &dbmovie.Slug, &dbid) //sqlpointer

			metadata.Getmoviemetatitles(dbmovie, cfgp)
			if dbmovieadded {
				if config.SettingsGeneral.UseMediaCache {
					database.AppendCache(logger.CacheDBMovie, database.DbstaticThreeStringTwoInt{Str1: dbmovie.Title, Str2: dbmovie.Slug, Str3: imdb, Num1: int(dbmovie.Year), Num2: dbmovie.ID})
				}
			}

			if dbmovie.Title == "" {
				_ = database.Scanrows1dyn(false, database.QueryDbmovieTitlesGetTitleByIDLmit1, &dbmovie.Title, &dbid)
				if dbmovie.Title != "" {
					database.ExecN("update dbmovies SET Title = ? where id = ?", &dbmovie.Title, &dbid)
				}
			}
		}
	}

	if !addnew {
		return dbid, nil
	}
	if dbid == 0 {
		dbid = MovieFindDBIDByImdb(imdb)
		if dbid == 0 {
			return 0, logger.ErrNotFoundMovie
		}
	}
	if listid == -1 {
		return 0, logger.ErrListnameEmpty
	}

	err := Checkaddmovieentry(&dbid, &cfgp.Lists[listid], imdb)
	if err != nil {
		return 0, err
	}
	return dbid, nil
}

// Checkaddmovieentry checks if a movie with the given ID should be added to
// the given list. It handles ignore lists, replacing existing lists, and
// inserting into the DB if needed.
func Checkaddmovieentry(dbid *uint, cfgplist *config.MediaListsConfig, imdb string) error {
	if dbid == nil || *dbid == 0 {
		return nil
	}
	if cfgplist == nil || cfgplist.Name == "" {
		return logger.ErrListnameEmpty
	}
	var getid uint
	if cfgplist.IgnoreMapListsLen >= 1 {
		if config.SettingsGeneral.UseMediaCache {
			if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
				return elem.Num1 == *dbid && (elem.Str == cfgplist.Name || strings.EqualFold(elem.Str, cfgplist.Name) || logger.SlicesContainsI(cfgplist.IgnoreMapLists, elem.Str))
			}) {
				return errIgnoredMovie
			}
		} else {
			args := logger.PLArrAny.Get()
			//args := make([]any, 0, cfgplist.IgnoreMapListsLen+1)
			for idx := range cfgplist.IgnoreMapLists {
				args.Arr = append(args.Arr, &cfgplist.IgnoreMapLists[idx])
			}
			args.Arr = append(args.Arr, dbid)
			_ = database.ScanrowsNArr(false, logger.JoinStrings("select count() from movies where listname in (?", cfgplist.IgnoreMapListsQu, ") and dbmovie_id = ?"), &getid, args.Arr) //JoinStrings
			logger.PLArrAny.Put(args)
			if getid >= 1 {
				return errIgnoredMovie
			}
		}
	}
	if cfgplist.ReplaceMapListsLen >= 1 {
		if config.SettingsGeneral.UseMediaCache {
			var replaced bool
			arr := database.GetCachedTypeObjArr[database.DbstaticOneStringTwoInt](logger.CacheMovie, false)
			for idx := range arr {
				if arr[idx].Num1 != *dbid {
					continue
				}
				if !strings.EqualFold(arr[idx].Str, cfgplist.Name) && logger.SlicesContainsI(cfgplist.ReplaceMapLists, arr[idx].Str) {
					if cfgplist.TemplateQuality == "" {
						database.ExecN("update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, dbid, &arr[idx].Str)
					} else {
						database.ExecN("update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &cfgplist.TemplateQuality, dbid, &arr[idx].Str)
					}
					replaced = true
				}
			}
			if replaced {
				database.RefreshMediaCache(false)
			}
		} else {
			var replaced bool
			arr := database.GetrowsNsize[string](false, "select count() from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", "select listname from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", dbid, &cfgplist.Name)
			for idx := range arr {
				if !logger.SlicesContainsI(cfgplist.ReplaceMapLists, arr[idx]) {
					continue
				}
				if cfgplist.TemplateQuality == "" {
					database.ExecN("update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, dbid, &arr[idx])
				} else {
					database.ExecN("update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &cfgplist.TemplateQuality, dbid, &arr[idx])
				}
				replaced = true
			}
			//clear(arr)
			if replaced {
				database.RefreshMediaCache(false)
			}
		}
	}

	database.ScanrowsNdyn(false, "select count() from movies where dbmovie_id = ? and listname = ?", &getid, dbid, &cfgplist.Name)
	if cfgplist.IgnoreMapListsLen >= 1 {
		if getid == 0 {
			args := logger.PLArrAny.Get()
			//args := make([]any, 0, cfgplist.IgnoreMapListsLen+1)
			for idx := range cfgplist.IgnoreMapLists {
				args.Arr = append(args.Arr, &cfgplist.IgnoreMapLists[idx])
			}
			args.Arr = append(args.Arr, dbid)
			_ = database.ScanrowsNArr(false, logger.JoinStrings("select count() from movies where listname in (?", cfgplist.IgnoreMapListsQu, ") and dbmovie_id = ?"), &getid, args.Arr) //JoinStrings
			logger.PLArrAny.Put(args)
		}
	}
	if getid == 0 {
		logger.LogDynamicany("debug", "Insert Movie for", &logger.StrImdb, imdb) //logpointerr
		movieid, err := database.ExecNid("Insert into movies (missing, listname, dbmovie_id, quality_profile) values (1, ?, ?, ?)", &cfgplist.Name, dbid, &cfgplist.TemplateQuality)
		if err != nil {
			return err
		}
		if config.SettingsGeneral.UseMediaCache {
			database.AppendCache(logger.CacheMovie, database.DbstaticOneStringTwoInt{Str: cfgplist.Name, Num1: *dbid, Num2: uint(movieid)})
		}
	}
	return nil
}

// AllowMovieImport checks if a movie can be imported based on the
// list configuration settings for minimum votes, minimum rating, excluded
// genres, and included genres. It returns a bool indicating if the import
// is allowed and an error if it is disallowed.
func AllowMovieImport(imdb string, listcfg *config.ListsConfig) (bool, error) {
	if imdb == "" {
		return false, logger.ErrNotFound
	}
	var i int
	if listcfg.MinVotes != 0 {
		database.ScanrowsNdyn(true, "select count() from imdb_ratings where tconst = ? and num_votes < ?", &i, imdb, &listcfg.MinVotes) //sqlpointerr
		if i >= 1 {
			return false, errVoteLow
		}
	}
	if listcfg.MinRating != 0 {
		database.ScanrowsNdyn(true, "select count() from imdb_ratings where tconst = ? and average_rating < ?", &i, imdb, &listcfg.MinRating) //sqlpointerr
		if i >= 1 {
			return false, errVoteRateLow
		}
	}

	if listcfg.ExcludegenreLen == 0 && listcfg.IncludegenreLen == 0 {
		return true, nil
	}
	genrearr := database.Getrows1size[string](true, "select count(genre) from imdb_genres where tconst = ?", "select genre from imdb_genres where tconst = ?", imdb) //sqlpointerr
	if len(genrearr) == 0 {
		return true, nil
	}
	var excludeby string

	//labexcluded:
	for idx := range listcfg.Excludegenre {
		if logger.SlicesContainsI(genrearr, listcfg.Excludegenre[idx]) {
			excludeby = listcfg.Excludegenre[idx]
			break
		}
	}
	if excludeby != "" && listcfg.ExcludegenreLen >= 1 {
		//clear(genrearr)
		return false, errors.New(logger.JoinStrings("excluded by ", excludeby))
	}

	var includebygenre bool

	//labincluded:
	for idx := range listcfg.Includegenre {
		if logger.SlicesContainsI(genrearr, listcfg.Includegenre[idx]) {
			includebygenre = true
			break
		}
	}
	//clear(genrearr)

	if !includebygenre && listcfg.IncludegenreLen >= 1 {
		return false, errIncludedGenreNotFound
	}
	return true, nil
}

// getsearchprovider returns the priority ordered list of metadata sources to use for searching movies based on searchtyperss.
// If searchtyperss is true, it will return MovieRSSMetaSourcePriority from config.
// If searchtyperss is false, it will return MovieParseMetaSourcePriority from config.
// If neither are set, it returns a default order of "imdb", "tmdb", "omdb".
func getsearchprovider(searchtyperss bool) []string {
	if searchtyperss {
		if len(config.SettingsGeneral.MovieRSSMetaSourcePriority) >= 1 {
			return config.SettingsGeneral.MovieRSSMetaSourcePriority
		}
	} else {
		if len(config.SettingsGeneral.MovieParseMetaSourcePriority) >= 1 {
			return config.SettingsGeneral.MovieParseMetaSourcePriority
		}
	}
	return defaultproviders
}

// JobImportDBSeriesStatic wraps jobImportDBSeries to import a series from a DbstaticTwoStringOneInt row containing the TVDB ID and name.
func JobImportDBSeriesStatic(row *database.DbstaticTwoStringOneRInt, cfgp *config.MediaTypeConfig, checkall, addnew bool) error {
	return jobImportDBSeries(&config.SerieConfig{TvdbID: row.Num, Name: row.Str1}, cfgp, cfgp.GetMediaListsEntryListID(row.Str2), checkall, addnew)
}

// JobImportDBSeries imports a series into the database and media lists from a SerieConfig.
// It handles adding new series or updating existing ones, refreshing metadata from TheTVDB,
// adding missing episodes, and updating the series lists table.
func JobImportDBSeries(wg *pool.SizedWaitGroup, series *config.SerieConfig, idxserie int, cfgp *config.MediaTypeConfig, listid int8, checkall, addnew bool) {
	defer logger.HandlePanic()
	defer wg.Done()
	logger.LogDynamicany("info", "Import/Update Serie", &logger.StrSeries, &series.Name, &logger.StrRow, &idxserie)
	err := jobImportDBSeries(series, cfgp, listid, checkall, addnew)
	if err != nil {
		logger.LogDynamicany("error", "Import/Update Failed", err, &logger.StrSeries, &series.TvdbID)
	}
}

// jobImportDBSeries imports a series into the database and media lists from a SerieConfig.
// It handles adding new series or updating existing ones, refreshing metadata from TheTVDB,
// adding missing episodes, and updating the series lists table.
func jobImportDBSeries(serieconfig *config.SerieConfig, cfgp *config.MediaTypeConfig, listid int8, checkall, addnew bool) error {
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	jobName := serieconfig.Name
	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(database.Getdatarow1[string](false, "select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", &serieconfig.TvdbID))
	}
	if jobName == "" && listid >= 0 {
		jobName = cfgp.Lists[listid].Name
	}
	if jobName == "" {
		return errors.New("jobname missing")
	}

	if importJobRunning.Check(jobName) {
		return errJobRunning
	}
	importJobRunning.Set(jobName, struct{}{})
	defer importJobRunning.Delete(jobName)
	if !addnew && listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(database.Getdatarow1[string](false, "select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", &serieconfig.TvdbID))
	}

	var dbserieadded bool
	var dbid uint
	if addnew && (serieconfig.Source == "none" || strings.EqualFold(serieconfig.Source, "none")) {
		if database.Getdatarow1[uint](false, "select count() from dbseries where seriename = ? COLLATE NOCASE", &serieconfig.Name) == 0 {
			dbid = insertdbserie(serieconfig)
			if dbid != 0 {
				dbserieadded = true
			}
		} else {
			_ = database.Scanrows1dyn(false, database.QueryDbseriesGetIDByName, &dbid, &serieconfig.Name)
		}
		if dbid == 0 {
			return logger.ErrNoID
		}
	}
	//var addserietitle database.DbstaticTwoStringOneInt
	if !addnew && (serieconfig.Source == "none" || strings.EqualFold(serieconfig.Source, "none")) {
		_ = database.Scanrows1dyn(false, database.QueryDbseriesGetIDByName, &dbid, &serieconfig.Name)
		if dbid >= 1 {
			for idx := range serieconfig.AlternateName {
				addalternateserietitle(&dbid, &serieconfig.AlternateName[idx], "")
			}
		}
	}
	if serieconfig.Source == "" || serieconfig.Source == logger.StrTvdb || strings.EqualFold(serieconfig.Source, logger.StrTvdb) {
		getid := database.Getdatarow1[uint](false, "select count() from dbseries where thetvdb_id = ?", &serieconfig.TvdbID)
		if addnew && getid == 0 {
			dbid = insertdbserie(serieconfig)
			if dbid != 0 {
				dbserieadded = true
			}
		} else if getid == 0 {
			return logger.ErrNotAllowed
		} else if getid >= 1 {
			_ = database.Scanrows1dyn(false, database.QueryDbseriesGetIDByTvdb, &dbid, &serieconfig.TvdbID)
		}
		if dbid != 0 {
			for idx := range serieconfig.AlternateName {
				addalternateserietitle(&dbid, &serieconfig.AlternateName[idx], "")
			}
		}
		if dbid != 0 && (dbserieadded || !addnew) && serieconfig.TvdbID != 0 {
			//Update Metadata
			dbserie, err := database.GetDbserieByID(&dbid)
			if err != nil {
				return logger.ErrNotFoundDbserie
			}
			if dbserie.Seriename == "" {
				dbserie.Seriename = serieconfig.Name
			}
			if dbserie.Identifiedby == "" {
				dbserie.Identifiedby = serieconfig.Identifiedby
			}
			if !dbserieadded && dbserie.UpdatedAt.After(logger.TimeGetNow().Add(-1*time.Hour)) {
				//update only if updated more than an hour ago
				logger.LogDynamicany("debug", "Skipped update metadata for dbserie ", &logger.StrTvdb, &serieconfig.TvdbID)
			} else {
				logger.LogDynamicany("debug", "Get metadata for", &logger.StrTvdb, &serieconfig.TvdbID)

				database.ExecN("update dbseries SET Seriename = ?, Aliases = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?",
					&dbserie.Seriename, &dbserie.Aliases, &dbserie.Season, &dbserie.Status, &dbserie.Firstaired, &dbserie.Network, &dbserie.Runtime, &dbserie.Language, &dbserie.Genre, &dbserie.Overview, &dbserie.Rating, &dbserie.Siterating, &dbserie.SiteratingCount, &dbserie.Slug, &dbserie.TraktID, &dbserie.ImdbID, &dbserie.ThetvdbID, &dbserie.FreebaseMID, &dbserie.FreebaseID, &dbserie.TvrageID, &dbserie.Facebook, &dbserie.Instagram, &dbserie.Twitter, &dbserie.Banner, &dbserie.Poster, &dbserie.Fanart, &dbserie.Identifiedby, &dbserie.ID)

				//size+10
				titles := database.GetrowsN[database.DbstaticOneStringOneUInt](false, database.Getdatarow1[uint](false, "select count() from dbserie_alternates where dbserie_id = ?", &dbserie.ID)+10, "select title, id from dbserie_alternates where dbserie_id = ?", &dbserie.ID)

				lenarr := cfgp.MetadataTitleLanguagesLen

				if config.SettingsGeneral.SerieAlternateTitleMetaSourceImdb && dbserie.ImdbID != "" {
					logger.AddImdbPrefixP(&dbserie.ImdbID)
					arr := database.Getrows1size[database.DbstaticThreeString](true, "select count() from imdb_akas where tconst = ?", "select title, region, slug from imdb_akas where tconst = ?", &dbserie.ImdbID)
					for idx := range arr {
						if database.GetDbStaticOneStringOneIntIdx(titles, arr[idx].Str1) != -1 {
							continue
						}

						if logger.SlicesContainsI(serieconfig.DisallowedName, arr[idx].Str1) {
							continue
						}
						if lenarr == 0 || logger.SlicesContainsI(cfgp.MetadataTitleLanguages, arr[idx].Str2) {
							titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbid, Str: arr[idx].Str1})
							addalternateserietitle(&dbserie.ID, &arr[idx].Str1, arr[idx].Str2)
						}
					}
					//clear(arr)
				}
				if config.SettingsGeneral.SerieAlternateTitleMetaSourceTrakt && (dbserie.TraktID != 0 || dbserie.ImdbID != "") {
					tbl, _ := apiexternal.GetTraktSerieAliases(dbserie)
					for idx := range tbl {
						if database.GetDbStaticOneStringOneIntIdx(titles, tbl[idx].Title) != -1 {
							continue
						}
						if logger.SlicesContainsI(serieconfig.DisallowedName, tbl[idx].Title) {
							continue
						}

						if lenarr == 0 || logger.SlicesContainsI(cfgp.MetadataTitleLanguages, tbl[idx].Country) {
							titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbid, Str: tbl[idx].Title})
							addalternateserietitle(&dbserie.ID, &tbl[idx].Title, tbl[idx].Country)
						}
					}
					//clear(tbl)
				}

				tbl := metadata.SerieGetMetadata(dbserie, cfgp.MetadataLanguage, config.SettingsGeneral.SerieMetaSourceTmdb, config.SettingsGeneral.SerieMetaSourceTrakt, !addnew, serieconfig.AlternateName)
				for idx := range tbl {
					if database.GetDbStaticOneStringOneIntIdx(titles, tbl[idx]) != -1 {
						continue
					}
					addalternateserietitle(&dbid, &tbl[idx], "")
					titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbserie.ID, Str: tbl[idx]})
				}
				//clear(tbl)
				if database.GetDbStaticOneStringOneIntIdx(titles, serieconfig.Name) == -1 {
					addalternateserietitle(&dbid, &serieconfig.Name, "")
					titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbserie.ID, Str: serieconfig.Name})
				}

				if database.GetDbStaticOneStringOneIntIdx(titles, dbserie.Seriename) == -1 {
					addalternateserietitle(&dbid, &dbserie.Seriename, "")
				}

				for idx := range serieconfig.DisallowedName {
					database.ExecN("delete from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE", &dbserie.ID, &serieconfig.DisallowedName[idx])
				}

				if (checkall || dbserieadded || !addnew) && (serieconfig.Source == "" || serieconfig.Source == logger.StrTvdb || strings.EqualFold(serieconfig.Source, logger.StrTvdb)) {
					logger.LogDynamicany("debug", "Get episodes for", &logger.StrTvdb, &serieconfig.TvdbID)

					if dbserie.ThetvdbID != 0 {
						apiexternal.UpdateTvdbSeriesEpisodes(dbserie.ThetvdbID, cfgp.MetadataLanguage, &dbserie.ID)
					}
					if config.SettingsGeneral.SerieMetaSourceTrakt && dbserie.ImdbID != "" {
						apiexternal.UpdateTraktSerieSeasonsAndEpisodes(dbserie.ImdbID, &dbserie.ID)
					}
				}
				//clear(titles)
			}
		}
	}

	if dbid == 0 {
		return logger.ErrNotFoundDbserie
	}

	if addnew {
		//Add Entry in SeriesTable

		if listid == -1 {
			return logger.ErrListnameEmpty
		}
		arr := database.Getrows1size[database.DbstaticOneStringOneInt](false, database.QuerySeriesCountByDBID, "select lower(listname), id from series where dbserie_id = ?", &dbid)
		for idx := range arr {
			if logger.SlicesContainsI(cfgp.Lists[listid].IgnoreMapLists, arr[idx].Str) {
				//clear(arr)
				return errors.New("series skip2 for")
			}
			if logger.SlicesContainsI(cfgp.Lists[listid].ReplaceMapLists, arr[idx].Str) {
				database.ExecN("update series SET listname = ?, dbserie_id = ? where id = ?", &cfgp.Lists[listid].Name, &dbid, &arr[idx].Num)
			}
		}
		//clear(arr)
		getid := database.GetdatarowN[uint](false, "select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE", dbid, &cfgp.Lists[listid].Name)
		if getid == 0 {
			logger.LogDynamicany("debug", "Add series for", &logger.StrListname, &cfgp.Lists[listid].Name, &logger.StrTvdb, &serieconfig.TvdbID)
			serieid, err := database.ExecNid("Insert into series (dbserie_id, listname, rootpath, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?)", &dbid, &cfgp.Lists[listid].Name, &serieconfig.Target, &serieconfig.SearchSpecials, &serieconfig.DontSearch, &serieconfig.DontUpgrade) //sqlpointer
			if err != nil {
				return err
			}
			if config.SettingsGeneral.UseMediaCache {
				database.AppendCache(logger.CacheSeries, database.DbstaticOneStringTwoInt{Str: cfgp.Lists[listid].Name, Num1: dbid, Num2: uint(serieid)})
			}
		} else {
			database.ExecN("update series SET search_specials=?, dont_search=?, dont_upgrade=? where dbserie_id = ? and listname = ?", &serieconfig.SearchSpecials, &serieconfig.DontSearch, &serieconfig.DontUpgrade, &dbid, &cfgp.Lists[listid].Name)
		}
	}

	dbseries := database.Getrows1size[uint](false, database.QueryDbserieEpisodesCountByDBID, "select id from dbserie_episodes where dbserie_id = ?", &dbid)
	episodes := database.Getrows1size[database.DbstaticTwoUint](false, "select count() from serie_episodes where dbserie_id = ?",
		"select dbserie_episode_id, serie_id from serie_episodes where dbserie_id = ?", &dbid)

	arr := database.Getrows1size[uint](false, database.QuerySeriesCountByDBID, "select id from series where dbserie_id = ?", &dbid)
	var quality string
	for idx := range arr {
		database.Scanrows1dyn(false, "select quality_profile from serie_episodes where serie_id = ? and length(quality_profile)>=1 limit 1", &quality, &arr[idx])
		if quality == "" {
			quality = cfgp.TemplateQuality
		}

		//LabDBSeries:
		for idxdb := range dbseries {
			if database.ArrStructContains(episodes, database.DbstaticTwoUint{Num1: dbseries[idxdb], Num2: arr[idx]}) {
				continue
			}
			// for idxepi := range episodes {
			// 	if episodes[idxepi].Num2 == arr[idx] && episodes[idxepi].Num1 == dbseries[idxdb] {
			// 		continue LabDBSeries
			// 	}
			// }
			database.ExecN("Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, 1, ?, ?)", &dbid, &arr[idx], &quality, &dbseries[idxdb])
		}
		//checkandaddserieepisodes(dbseries, episodes, &arr[idx], dbid, cfgp)
	}
	//clear(arr)
	//clear(episodes)
	//clear(dbseries)
	return nil
}

// insertdbserie inserts a new dbseries row into the dbseries table.
// It takes the name, aliases slice, TheTVDB ID and identifiedby string as parameters.
// It inserts a new row into the dbseries table with those values.
// It returns the auto generated uint ID for the new row.
// It also handles caching and inserting alternate names.
func insertdbserie(serieconfig *config.SerieConfig) uint {
	getid := database.Getdatarow1[uint](false, "select id from dbseries where seriename = ? COLLATE NOCASE", &serieconfig.Name)
	if getid >= 1 {
		return getid
	}
	logger.LogDynamicany("debug", "Insert dbseries for", &logger.StrTvdb, &serieconfig.TvdbID)
	inres, err := database.ExecNid("insert into dbseries (seriename, aliases, thetvdb_id, identifiedby) values (?, ?, ?, ?)",
		&serieconfig.Name, strings.Join(serieconfig.AlternateName, ","), &serieconfig.TvdbID, &serieconfig.Identifiedby)
	if err != nil {
		return 0
	}
	dbid := uint(inres)

	if config.SettingsGeneral.UseMediaCache {
		database.AppendCache(logger.CacheDBSeries, database.DbstaticTwoStringOneInt{Str1: serieconfig.Name, Str2: logger.StringToSlug(serieconfig.Name), Num: dbid})
	}

	for idx := range serieconfig.AlternateName {
		if serieconfig.AlternateName[idx] == "" {
			continue
		}
		addalternateserietitle(&dbid, &serieconfig.AlternateName[idx], "")
	}
	addalternateserietitle(&dbid, &serieconfig.Name, "")
	return dbid
}

// addAlternateSerieTitle adds an alternate title for the given series ID.
// It checks if the title already exists for that ID, and if not, inserts
// it into the dbserie_alternates table. The region parameter indicates
// the language/region for the title. It also adds the title to the cache
// if enabled.
func addalternateserietitle(dbserieid *uint, title *string, region string) {
	if database.GetdatarowN[uint](false, "select count() from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE", dbserieid, title) == 0 {
		slug := logger.StringToSlug(*title)

		database.ExecN("Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)", title, &slug, dbserieid, region) //sqlpointerr
		if config.SettingsGeneral.UseMediaCache {
			database.AppendCache(logger.CacheDBSeriesAlt, database.DbstaticTwoStringOneInt{Str1: *title, Str2: slug, Num: *dbserieid})
		}
	}
}

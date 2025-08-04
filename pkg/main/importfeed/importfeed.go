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
)

var (
	errIgnoredMovie          = errors.New("movie ignored")
	errJobRunning            = errors.New("job running")
	errVoteLow               = errors.New("error vote count too low")
	errVoteRateLow           = errors.New("error average vote too low")
	errIncludedGenreNotFound = errors.New("included genre not found")
	errSeriesSkipped         = errors.New("series skip for")
	importJobRunning         = logger.NewSyncMap[struct{}](10)
	defaultproviders         = []string{"imdb", "tmdb", "omdb"}
)

// var importJobRunning string

// checkimdbyearsingle checks if the year from IMDB matches the year
// in the FileParser, allowing a 1 year difference. It returns true if
// the years match or are within 1 year difference.
func checkimdbyearsingle(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	imdb *string,
	haveyear int,
) bool {
	if haveyear == 0 || m.Year == 0 || *imdb == "" {
		return false
	}
	if haveyear == int(m.Year) {
		return true
	}

	if haveyear == int(m.Year+1) || haveyear == int(m.Year-1) {
		m.TempID = MovieFindDBIDByImdb(imdb)
		if cfgp.GetMediaQualityConfigStr(
			database.Getdatarow[string](
				false,
				logger.GetStringsMap(cfgp.Useseries, logger.DBQualityMediaByID),
				&m.TempID,
			),
		).CheckYear1 {
			return true
		}
	}
	return false
}

// MovieFindDBIDByImdb looks up the database ID for a movie by its IMDB ID.
// It takes a string containing the IMDB ID and returns the uint database ID.
// It first checks the cache if enabled, otherwise queries the database directly.
// If no match is found, it returns 0.
func MovieFindDBIDByImdb(imdb *string) uint {
	if config.GetSettingsGeneral().UseMediaCache {
		id := database.CacheThreeStringIntIndexFunc(logger.CacheDBMovie, imdb)
		if id != 0 {
			return id
		}
		return 0
	}
	return database.Getdatarow[uint](false, "select id from dbmovies where imdb_id = ?", imdb)
}

// MovieFindDBIDByTmdbID looks up the database ID for a movie by its TMDB ID.
// It takes a pointer to an integer containing the TMDB ID and returns the uint database ID.
// If no match is found, it returns 0.
func MovieFindDBIDByTmdbID(tmdbid *int) uint {
	return database.Getdatarow[uint](false, "select id from dbmovies where moviedb_id = ?", tmdbid)
}

// MovieFindImdbIDByTitle searches for a movie's IMDB ID by its title.
// It first searches the database and caches by title and slugified title.
// If not found, it can search external APIs based on config settings.
// It populates the ParseInfo struct with the found IMDB ID and other data.
func MovieFindImdbIDByTitle(
	addifnotfound bool,
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
) {
	if m == nil {
		return
	}
	if m.Title == "" {
		m.Cleanimdbdbmovie()
		return
	}

	m.TempTitle = logger.TrimSpace(m.Title)
	m.Findmoviedbidbytitle(false)
	if m.DbmovieID == 0 {
		m.Findmoviedbidbytitle(true)
	}
	if m.DbmovieID != 0 {
		if config.GetSettingsGeneral().UseMediaCache {
			m.CacheThreeStringIntIndexFuncGetImdb()
			if m.DbmovieID != 0 {
				return
			}
		} else {
			database.Scanrowsdyn(false, "select imdb_id from dbmovies where id = ?", &m.Imdb, &m.DbmovieID)
			return
		}
	}

	if !addifnotfound {
		m.Cleanimdbdbmovie()
		return
	}
	slug := logger.StringToSlug(m.Title)

	for _, val := range getsearchprovider(false) {
		if processprovider(val, m, cfgp, &slug) {
			continue
		}

		if m.Imdb != "" {
			return
		}
	}
	m.Cleanimdbdbmovie()
}

// processprovider is a helper function that processes a given provider to search for a movie's IMDB ID based on its title.
// It first checks the IMDB database, then the TMDB database, and finally the OMDB database, depending on the configured settings.
// It populates the provided ParseInfo struct with the found IMDB ID and other data.
// The function returns true if a valid IMDB ID was found, and false otherwise.
func processprovider(
	provider string,
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	slug *string,
) bool {
	var haveyear int
	switch provider {
	case logger.StrImdb:
		if config.GetSettingsGeneral().MovieMetaSourceImdb {
			arr := database.GetrowsN[database.DbstaticOneStringOneInt](
				true,
				database.Getdatarow[uint](
					false,
					"select count() from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)",
					&m.Title,
					&m.Title,
					slug,
				),
				"select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)",
				&m.Title,
				&m.Title,
				slug,
			)
			for idx := range arr {
				if checkimdbyearsingle(m, cfgp, &arr[idx].Str, arr[idx].Num) {
					m.Imdb = arr[idx].Str
					m.MovieFindDBIDByImdbParser()
					return true
				}
			}

			arr2 := database.Getrowssize[string](
				true,
				"select count() from (select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?)",
				"select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?",
				&m.Title,
				slug,
			)
			for idx := range arr2 {
				database.Scanrowsdyn(
					true,
					"select start_year from imdb_titles where tconst = ?",
					&haveyear,
					&arr2[idx],
				)
				if checkimdbyearsingle(m, cfgp, &arr2[idx], haveyear) {
					m.Imdb = arr2[idx]
					m.MovieFindDBIDByImdbParser()
					return true
				}
			}
			m.Cleanimdbdbmovie()
		}
	case "tmdb":
		if config.GetSettingsGeneral().MovieMetaSourceTmdb {
			if m.Title == "" {
				m.Cleanimdbdbmovie()
				return true
			}

			tbl, err := apiexternal.SearchTmdbMovie(m.Title)
			if err != nil {
				m.Cleanimdbdbmovie()
				return true
			}
			for idx := range tbl.Results {
				database.Scanrowsdyn(
					false,
					"select imdb_id from dbmovies where moviedb_id = ?",
					&m.Imdb,
					&tbl.Results[idx].ID,
				)
				if m.Imdb == "" {
					moviedbexternal, err := apiexternal.GetTmdbMovieExternal(tbl.Results[idx].ID)
					if err != nil {
						continue
					}
					m.Imdb = moviedbexternal.ImdbID
					m.MovieFindDBIDByImdbParser()
					return true
				}
				if m.Imdb == "" {
					continue
				}
				database.Scanrowsdyn(
					true,
					"select start_year from imdb_titles where tconst = ?",
					&haveyear,
					&m.Imdb,
				)
				if checkimdbyearsingle(m, cfgp, &m.Imdb, haveyear) {
					m.MovieFindDBIDByImdbParser()
					return true
				}
			}
			m.Cleanimdbdbmovie()
		}
	case "omdb":
		if config.GetSettingsGeneral().MovieMetaSourceOmdb {
			tbl, err := apiexternal.SearchOmdbMovie(m.Title, "")
			if err != nil {
				m.Cleanimdbdbmovie()
				return true
			}
			for idx := range tbl.Search {
				if checkimdbyearsingle(
					m,
					cfgp,
					&tbl.Search[idx].ImdbID,
					logger.StringToInt(tbl.Search[idx].Year),
				) {
					m.Imdb = tbl.Search[idx].ImdbID
					m.MovieFindDBIDByImdbParser()
					return true
				}
			}
			m.Cleanimdbdbmovie()
		}
	default:
		return true
	}
	return false
}

// JobImportMoviesByList imports or updates a list of movies in parallel.
// It takes a list of movie titles/IDs, an index, a media type config, a list ID,
// and a flag for whether to add new movies.
// It logs the import result for each movie.
func JobImportMoviesByList(
	entry string,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) error {
	if listid == -1 {
		return errors.New("listid not set")
	}
	logger.Logtype("info", 0).
		Str(logger.StrMovie, entry).
		Int(logger.StrRow, idx).
		Msg("Import/Update Movie")
	_, err := JobImportMovies(entry, cfgp, listid, addnew)
	if err != nil && err.Error() != "movie ignored" {
		logger.LogDynamicany1StringErr(
			"error",
			"Import/Update Failed",
			err,
			logger.StrImdb,
			entry,
		) // logpointerr
		return err
	}
	return nil
}

// JobImportMovies imports a movie into the database and specified list
// given its IMDb ID. It handles checking if the movie exists, adding
// it if needed, updating metadata, and adding it to the target list.
func JobImportMovies(
	imdb string,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}
	if imdb == "" {
		return 0, logger.ErrImdbEmpty
	}
	if importJobRunning.Check(imdb) {
		return 0, errJobRunning
	}
	importJobRunning.Add(imdb, struct{}{}, 0, false, 0)
	defer importJobRunning.Delete(imdb)

	var dbmovieadded bool
	var dbmovie database.Dbmovie
	dbmovie.ImdbID = imdb
	dbmovie.MovieFindDBIDByImdbParser()
	checkdbmovie := dbmovie.ID >= 1

	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(
			database.Getdatarow[string](
				false,
				"select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id=?)",
				&dbmovie.ImdbID,
			),
		)
	}
	if !checkdbmovie && addnew {
		if listid == -1 {
			return 0, logger.ErrCfgpNotFound
		}
		_, err := AllowMovieImport(&dbmovie.ImdbID, cfgp.Lists[listid].CfgList)
		if err != nil {
			return 0, err
		}

		if database.Getdatarow[uint](
			false,
			"select id from dbmovies where imdb_id = ?",
			&dbmovie.ImdbID,
		) == 0 {
			logger.LogDynamicany1String(
				"debug",
				"Insert dbmovie for",
				logger.StrJob,
				imdb,
			) // logpointerr
			dbresult, err := database.ExecNid(
				"insert into dbmovies (Imdb_ID) VALUES (?)",
				&dbmovie.ImdbID,
			)
			if err != nil {
				return 0, err
			}
			dbmovie.ID = logger.Int64ToUint(dbresult)
			dbmovieadded = true
		}
	}
	if dbmovie.ID == 0 {
		dbmovie.MovieFindDBIDByImdbParser()
	}
	if dbmovie.ID == 0 {
		return 0, logger.ErrNotFoundDbmovie
	}

	if dbmovieadded || !addnew {
		logger.LogDynamicany1String("debug", "Get metadata for", logger.StrJob, imdb) // logpointerr
		err := dbmovie.GetDbmovieByIDP(&dbmovie.ID)
		if err != nil {
			return 0, errIgnoredMovie
		}
		if dbmovie.ImdbID == "" {
			dbmovie.ImdbID = imdb
		}
		if !dbmovieadded &&
			logger.TimeAfter(dbmovie.UpdatedAt, logger.TimeGetNow().Add(-1*time.Hour)) {
			// update only if updated more than an hour ago
			logger.LogDynamicany1String(
				"debug",
				"Skipped update metadata for dbmovie",
				logger.StrJob,
				dbmovie.ImdbID,
			)
		} else {
			metadata.Getmoviemetadata(&dbmovie, true, true, cfgp, dbmovieadded)
		}
	}

	if !addnew {
		return dbmovie.ID, nil
	}
	if dbmovie.ID == 0 {
		dbmovie.MovieFindDBIDByImdbParser()
		if dbmovie.ID == 0 {
			return 0, logger.ErrNotFoundMovie
		}
	}
	if listid == -1 {
		return 0, logger.ErrListnameEmpty
	}

	err := Checkaddmovieentry(&dbmovie.ID, &cfgp.Lists[listid], imdb)
	if err != nil {
		return 0, err
	}
	return dbmovie.ID, nil
}

// Checkaddmovieentry checks if a movie with the given ID should be added to
// the given list. It handles ignore lists, replacing existing lists, and
// inserting into the DB if needed.
func Checkaddmovieentry(dbid *uint, cfgplist *config.MediaListsConfig, imdb string) error {
	if cfgplist == nil || cfgplist.Name == "" {
		return logger.ErrListnameEmpty
	}
	var getcount uint
	if cfgplist.IgnoreMapListsLen >= 1 {
		if config.GetSettingsGeneral().UseMediaCache {
			dbidn := *dbid
			if database.CacheOneStringTwoIntIndexFunc(
				logger.CacheMovie,
				func(elem *database.DbstaticOneStringTwoInt) bool {
					return elem.Num1 == dbidn &&
						(elem.Str == cfgplist.Name || strings.EqualFold(elem.Str, cfgplist.Name) || logger.SlicesContainsI(cfgplist.IgnoreMapLists, elem.Str))
				},
			) {
				return errIgnoredMovie
			}
		} else {
			args := logger.PLArrAny.Get()
			for idx := range cfgplist.IgnoreMapLists {
				args.Arr = append(args.Arr, &cfgplist.IgnoreMapLists[idx])
			}
			args.Arr = append(args.Arr, dbid)
			database.ScanrowsNArr(false, logger.JoinStrings("select count() from movies where listname in (?", cfgplist.IgnoreMapListsQu, ") and dbmovie_id = ?"), &getcount, args.Arr)
			logger.PLArrAny.Put(args)
			if getcount >= 1 {
				return errIgnoredMovie
			}
		}
	}
	if cfgplist.ReplaceMapListsLen >= 1 {
		if config.GetSettingsGeneral().UseMediaCache {
			var replaced bool
			arr := database.GetCachedTwoIntArr(logger.CacheMovie, false, true)
			for idx := range arr {
				if arr[idx].Num1 != *dbid {
					continue
				}
				if !strings.EqualFold(arr[idx].Str, cfgplist.Name) &&
					logger.SlicesContainsI(cfgplist.ReplaceMapLists, arr[idx].Str) {
					if cfgplist.TemplateQuality == "" {
						database.ExecN(
							"update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE",
							&cfgplist.Name,
							dbid,
							&arr[idx].Str,
						)
					} else {
						database.ExecN("update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &cfgplist.TemplateQuality, dbid, &arr[idx].Str)
					}
					replaced = true
				}
			}
			if replaced {
				database.RefreshMediaCacheList(false, true)
			}
		} else {
			var replaced bool
			arr := database.Getrowssize[string](false, "select count() from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", "select listname from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", dbid, &cfgplist.Name)
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
			if replaced {
				database.RefreshMediaCacheList(false, true)
			}
		}
	}

	database.Scanrowsdyn(
		false,
		"select count() from movies where dbmovie_id = ? and listname = ?",
		&getcount,
		dbid,
		&cfgplist.Name,
	)
	if cfgplist.IgnoreMapListsLen >= 1 {
		if getcount == 0 {
			args := logger.PLArrAny.Get()
			for idx := range cfgplist.IgnoreMapLists {
				args.Arr = append(args.Arr, &cfgplist.IgnoreMapLists[idx])
			}
			args.Arr = append(args.Arr, dbid)
			database.ScanrowsNArr(
				false,
				logger.JoinStrings(
					"select count() from movies where listname in (?",
					cfgplist.IgnoreMapListsQu,
					") and dbmovie_id = ?",
				),
				&getcount,
				args.Arr,
			)
			logger.PLArrAny.Put(args)
		}
	}
	if getcount == 0 {
		logger.LogDynamicany1String(
			"debug",
			"Insert Movie for",
			logger.StrImdb,
			imdb,
		) // logpointerr
		movieid, err := database.ExecNid(
			"Insert into movies (missing, listname, dbmovie_id, quality_profile) values (1, ?, ?, ?)",
			&cfgplist.Name,
			dbid,
			&cfgplist.TemplateQuality,
		)
		if err != nil {
			return err
		}
		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				logger.CacheMovie,
				database.DbstaticOneStringTwoInt{
					Str:  cfgplist.Name,
					Num1: *dbid,
					Num2: logger.Int64ToUint(movieid),
				},
			)
		}
	}
	return nil
}

// AllowMovieImport checks if a movie can be imported based on the
// list configuration settings for minimum votes, minimum rating, excluded
// genres, and included genres. It returns a bool indicating if the import
// is allowed and an error if it is disallowed.
func AllowMovieImport(imdb *string, listcfg *config.ListsConfig) (bool, error) {
	if imdb == nil || *imdb == "" {
		return false, errIgnoredMovie
	}
	if listcfg.MinVotes != 0 {
		if database.Getdatarow[uint](
			true,
			"select count() from imdb_ratings where tconst = ? and num_votes < ?",
			imdb,
			&listcfg.MinVotes,
		) >= 1 {
			return false, errVoteLow
		}
	}
	if listcfg.MinRating != 0 {
		if database.Getdatarow[uint](
			true,
			"select count() from imdb_ratings where tconst = ? and average_rating < ?",
			imdb,
			&listcfg.MinRating,
		) >= 1 {
			return false, errVoteRateLow
		}
	}

	if listcfg.ExcludegenreLen == 0 && listcfg.IncludegenreLen == 0 {
		return true, nil
	}
	genrearr := database.Getrowssize[string](
		true,
		"select count() from imdb_genres where tconst = ?",
		"select genre from imdb_genres where tconst = ?",
		imdb,
	)
	if len(genrearr) == 0 {
		return true, nil
	}

	for idx := range listcfg.Excludegenre {
		if logger.SlicesContainsI(genrearr, listcfg.Excludegenre[idx]) {
			return false, errors.New(("excluded by " + listcfg.Excludegenre[idx]))
		}
	}

	for idx := range listcfg.Includegenre {
		if logger.SlicesContainsI(genrearr, listcfg.Includegenre[idx]) {
			return true, nil
		}
	}

	if listcfg.IncludegenreLen >= 1 {
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
		if len(config.GetSettingsGeneral().MovieRSSMetaSourcePriority) >= 1 {
			return config.GetSettingsGeneral().MovieRSSMetaSourcePriority
		}
	} else {
		if len(config.GetSettingsGeneral().MovieParseMetaSourcePriority) >= 1 {
			return config.GetSettingsGeneral().MovieParseMetaSourcePriority
		}
	}
	return defaultproviders
}

// JobImportDBSeriesStatic wraps jobImportDBSeries to import a series from a DbstaticTwoStringOneInt row containing the TVDB ID and name.
func JobImportDBSeriesStatic(
	row *database.DbstaticTwoStringOneRInt,
	cfgp *config.MediaTypeConfig,
) error {
	return jobImportDBSeries(
		&config.SerieConfig{TvdbID: row.Num, Name: row.Str1},
		cfgp,
		cfgp.GetMediaListsEntryListID(row.Str2),
		true,
		false,
	)
}

// JobImportDBSeries imports a series into the database and media lists from a SerieConfig.
// It handles adding new series or updating existing ones, refreshing metadata from TheTVDB,
// adding missing episodes, and updating the series lists table.
func JobImportDBSeries(
	serie *config.SerieConfig,
	idxserie int,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	logger.Logtype("info", 0).
		Str(logger.StrSeries, serie.Name).
		Int(logger.StrRow, idxserie).
		Msg("Import/Update Serie")
	err := jobImportDBSeries(serie, cfgp, listid, false, true)
	if err != nil {
		logger.LogDynamicany1IntErr(
			"error",
			"Import/Update Failed",
			err,
			logger.StrSeries,
			serie.TvdbID,
		)
		return err
	}
	return nil
}

// jobImportDBSeries imports a series into the database and media lists from a SerieConfig.
// It handles adding new series or updating existing ones, refreshing metadata from TheTVDB,
// adding missing episodes, and updating the series lists table.
func jobImportDBSeries(
	serieconfig *config.SerieConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	checkall, addnew bool,
) error {
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	jobName := serieconfig.Name
	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(
			database.Getdatarow[string](
				false,
				"select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)",
				&serieconfig.TvdbID,
			),
		)
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
	importJobRunning.Add(jobName, struct{}{}, 0, false, 0)
	defer importJobRunning.Delete(jobName)
	if !addnew && listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(
			database.Getdatarow[string](
				false,
				"select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)",
				&serieconfig.TvdbID,
			),
		)
	}

	var dbserieadded bool

	var dbserie database.Dbserie
	var count uint
	if serieconfig.Source == "none" || strings.EqualFold(serieconfig.Source, "none") {
		database.Scanrowsdyn(
			false,
			"select id from dbseries where seriename = ? COLLATE NOCASE",
			&dbserie.ID,
			&serieconfig.Name,
		)
		if addnew {
			if dbserie.ID == 0 {
				insertdbserie(serieconfig, &dbserie)
				if dbserie.ID != 0 {
					dbserieadded = true
				}
			}
			if dbserie.ID == 0 {
				return logger.ErrNoID
			}
		}
		if dbserie.ID >= 1 {
			for idx := range serieconfig.AlternateName {
				addalternateserietitle(&dbserie.ID, &serieconfig.AlternateName[idx])
			}
		}
	}

	if serieconfig.Source == "" || serieconfig.Source == logger.StrTvdb ||
		strings.EqualFold(serieconfig.Source, logger.StrTvdb) {
		database.Scanrowsdyn(
			false,
			"select count() from dbseries where thetvdb_id = ?",
			&count,
			&serieconfig.TvdbID,
		)
		if count == 0 {
			if addnew {
				insertdbserie(serieconfig, &dbserie)
				if dbserie.ID != 0 {
					dbserieadded = true
				}
			} else {
				return logger.ErrNotAllowed
			}
		} else if count >= 1 {
			database.Scanrowsdyn(false, database.QueryDbseriesGetIDByTvdb, &dbserie.ID, &serieconfig.TvdbID)
		}

		if dbserie.ID != 0 {
			for idx := range serieconfig.AlternateName {
				addalternateserietitle(&dbserie.ID, &serieconfig.AlternateName[idx])
			}
		}
		if dbserie.ID != 0 && (dbserieadded || !addnew) && serieconfig.TvdbID != 0 {
			// Update Metadata
			err := dbserie.GetDbserieByIDP(&dbserie.ID)
			if err != nil {
				return logger.ErrNotFoundDbserie
			}
			if dbserie.Seriename == "" {
				dbserie.Seriename = serieconfig.Name
			}
			if dbserie.Identifiedby == "" {
				dbserie.Identifiedby = serieconfig.Identifiedby
			}
			if !dbserieadded &&
				logger.TimeAfter(dbserie.UpdatedAt, logger.TimeGetNow().Add(-1*time.Hour)) {
				// update only if updated more than an hour ago
				logger.LogDynamicany1Int(
					"debug",
					"Skipped update metadata for dbserie ",
					logger.StrTvdb,
					serieconfig.TvdbID,
				)
			} else {
				logger.LogDynamicany1Int("debug", "Get metadata for", logger.StrTvdb, serieconfig.TvdbID)

				database.ExecN("update dbseries SET Seriename = ?, Aliases = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?",
					&dbserie.Seriename, &dbserie.Aliases, &dbserie.Season, &dbserie.Status, &dbserie.Firstaired, &dbserie.Network, &dbserie.Runtime, &dbserie.Language, &dbserie.Genre, &dbserie.Overview, &dbserie.Rating, &dbserie.Siterating, &dbserie.SiteratingCount, &dbserie.Slug, &dbserie.TraktID, &dbserie.ImdbID, &dbserie.ThetvdbID, &dbserie.FreebaseMID, &dbserie.FreebaseID, &dbserie.TvrageID, &dbserie.Facebook, &dbserie.Instagram, &dbserie.Twitter, &dbserie.Banner, &dbserie.Poster, &dbserie.Fanart, &dbserie.Identifiedby, &dbserie.ID)
				database.Scanrowsdyn(false, "select count() from dbserie_alternates where dbserie_id = ?", &count, &dbserie.ID)
				titles := database.GetrowsN[database.DbstaticOneStringOneUInt](false, count+10, "select title, id from dbserie_alternates where dbserie_id = ?", &dbserie.ID)

				if config.GetSettingsGeneral().SerieAlternateTitleMetaSourceImdb && dbserie.ImdbID != "" {
					dbserie.ImdbID = logger.AddImdbPrefix(dbserie.ImdbID)
					arr := database.Getrowssize[database.DbstaticThreeString](true, "select count() from imdb_akas where tconst = ?", "select title, region, slug from imdb_akas where tconst = ?", &dbserie.ImdbID)
					for idx := range arr {
						if database.GetDBStaticOneStringOneIntIdx(titles, arr[idx].Str1) != -1 {
							continue
						}

						if logger.SlicesContainsI(serieconfig.DisallowedName, arr[idx].Str1) {
							continue
						}
						if cfgp.MetadataTitleLanguagesLen == 0 || logger.SlicesContainsI(cfgp.MetadataTitleLanguages, arr[idx].Str2) {
							titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbserie.ID, Str: arr[idx].Str1})
							addalternateserietitle(&dbserie.ID, &arr[idx].Str1, &arr[idx].Str2)
						}
					}
				}
				if config.GetSettingsGeneral().SerieAlternateTitleMetaSourceTrakt && (dbserie.TraktID != 0 || dbserie.ImdbID != "") {
					tbl := apiexternal.GetTraktSerieAliases(&dbserie)
					for idx := range tbl {
						if database.GetDBStaticOneStringOneIntIdx(titles, tbl[idx].Title) != -1 {
							continue
						}
						if logger.SlicesContainsI(serieconfig.DisallowedName, tbl[idx].Title) {
							continue
						}

						if cfgp.MetadataTitleLanguagesLen == 0 || logger.SlicesContainsI(cfgp.MetadataTitleLanguages, tbl[idx].Country) {
							titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbserie.ID, Str: tbl[idx].Title})
							addalternateserietitle(&dbserie.ID, &tbl[idx].Title, &tbl[idx].Country)
						}
					}
				}

				tbl := metadata.SerieGetMetadata(&dbserie, cfgp.MetadataLanguage, config.GetSettingsGeneral().SerieMetaSourceTmdb, config.GetSettingsGeneral().SerieMetaSourceTrakt, !addnew, serieconfig.AlternateName)
				for idx := range tbl {
					if database.GetDBStaticOneStringOneIntIdx(titles, tbl[idx]) != -1 {
						continue
					}
					addalternateserietitle(&dbserie.ID, &tbl[idx])
					titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbserie.ID, Str: tbl[idx]})
				}
				if database.GetDBStaticOneStringOneIntIdx(titles, serieconfig.Name) == -1 {
					addalternateserietitle(&dbserie.ID, &serieconfig.Name)
					titles = append(titles, database.DbstaticOneStringOneUInt{Num: dbserie.ID, Str: serieconfig.Name})
				}

				if database.GetDBStaticOneStringOneIntIdx(titles, dbserie.Seriename) == -1 {
					addalternateserietitle(&dbserie.ID, &dbserie.Seriename)
				}

				for idx := range serieconfig.DisallowedName {
					database.ExecN("delete from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE", &dbserie.ID, &serieconfig.DisallowedName[idx])
				}

				if (checkall || dbserieadded || !addnew) && (serieconfig.Source == "" || serieconfig.Source == logger.StrTvdb || strings.EqualFold(serieconfig.Source, logger.StrTvdb)) {
					logger.LogDynamicany1Int("debug", "Get episodes for", logger.StrTvdb, serieconfig.TvdbID)

					if dbserie.ThetvdbID != 0 {
						apiexternal.UpdateTvdbSeriesEpisodes(dbserie.ThetvdbID, cfgp.MetadataLanguage, &dbserie.ID)
					}
					if config.GetSettingsGeneral().SerieMetaSourceTrakt && dbserie.ImdbID != "" {
						apiexternal.UpdateTraktSerieSeasonsAndEpisodes(dbserie.ImdbID, &dbserie.ID)
					}
				}
			}
		}
	}

	if dbserie.ID == 0 {
		return logger.ErrNotFoundDbserie
	}

	if addnew {
		// Add Entry in SeriesTable

		if listid == -1 {
			return logger.ErrListnameEmpty
		}
		arr := database.Getrowssize[database.DbstaticOneStringOneInt](
			false,
			database.QuerySeriesCountByDBID,
			"select lower(listname), id from series where dbserie_id = ?",
			&dbserie.ID,
		)
		for idx := range arr {
			if logger.SlicesContainsI(cfgp.Lists[listid].IgnoreMapLists, arr[idx].Str) {
				return errSeriesSkipped
			}
			if logger.SlicesContainsI(cfgp.Lists[listid].ReplaceMapLists, arr[idx].Str) {
				database.ExecN(
					"update series SET listname = ?, dbserie_id = ? where id = ?",
					&cfgp.Lists[listid].Name,
					&dbserie.ID,
					&arr[idx].Num,
				)
			}
		}
		if database.Getdatarow[uint](
			false,
			"select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE",
			&dbserie.ID,
			&cfgp.Lists[listid].Name,
		) == 0 {
			logger.LogDynamicany2StrAny(
				"debug",
				"Add series for",
				logger.StrListname,
				cfgp.Lists[listid].Name,
				logger.StrTvdb,
				&serieconfig.TvdbID,
			)
			serieid, err := database.ExecNid(
				"Insert into series (dbserie_id, listname, rootpath, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?)",
				&dbserie.ID,
				&cfgp.Lists[listid].Name,
				&serieconfig.Target,
				&serieconfig.SearchSpecials,
				&serieconfig.DontSearch,
				&serieconfig.DontUpgrade,
			)
			if err != nil {
				return err
			}
			if config.GetSettingsGeneral().UseMediaCache {
				database.AppendCacheTwoInt(
					logger.CacheSeries,
					database.DbstaticOneStringTwoInt{
						Str:  cfgp.Lists[listid].Name,
						Num1: dbserie.ID,
						Num2: logger.Int64ToUint(serieid),
					},
				)
			}
		} else {
			database.ExecN("update series SET search_specials=?, dont_search=?, dont_upgrade=? where dbserie_id = ? and listname = ?", &serieconfig.SearchSpecials, &serieconfig.DontSearch, &serieconfig.DontUpgrade, &dbserie.ID, &cfgp.Lists[listid].Name)
		}
	}

	dbseries := database.Getrowssize[uint](
		false,
		database.QueryDbserieEpisodesCountByDBID,
		"select id from dbserie_episodes where dbserie_id = ?",
		&dbserie.ID,
	)
	episodes := database.Getrowssize[database.DbstaticTwoUint](
		false,
		"select count() from serie_episodes where dbserie_id = ?",
		"select dbserie_episode_id, serie_id from serie_episodes where dbserie_id = ?",
		&dbserie.ID,
	)

	arr := database.Getrowssize[uint](
		false,
		database.QuerySeriesCountByDBID,
		"select id from series where dbserie_id = ?",
		&dbserie.ID,
	)
	for idx := range arr {
		quality := database.Getdatarow[string](
			false,
			"select quality_profile from serie_episodes where serie_id = ? and quality_profile != '' and quality_profile is not NULL limit 1",
			&arr[idx],
		)
		if quality == "" {
			quality = cfgp.TemplateQuality
		}

	labeldbser:
		for idxdb := range dbseries {
			for idxepi := range episodes {
				if episodes[idxepi].Num1 == dbseries[idxdb] && episodes[idxepi].Num2 == arr[idx] {
					continue labeldbser
				}
			}
			database.ExecN("Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, 1, ?, ?)", &dbserie.ID, &arr[idx], &quality, &dbseries[idxdb])
		}
	}
	return nil
}

// insertdbserie inserts a new dbseries row into the dbseries table.
// It takes the name, aliases slice, TheTVDB ID and identifiedby string as parameters.
// It inserts a new row into the dbseries table with those values.
// It returns the auto generated uint ID for the new row.
// It also handles caching and inserting alternate names.
func insertdbserie(serieconfig *config.SerieConfig, dbserie *database.Dbserie) {
	database.Scanrowsdyn(
		false,
		"select id from dbseries where seriename = ? COLLATE NOCASE",
		&dbserie.ID,
		&serieconfig.Name,
	)
	if dbserie.ID >= 1 {
		return
	}
	logger.LogDynamicany1Int("debug", "Insert dbseries for", logger.StrTvdb, serieconfig.TvdbID)
	aliases := logger.JoinStringsSep(serieconfig.AlternateName, ",")
	inres, err := database.ExecNid(
		"insert into dbseries (seriename, aliases, thetvdb_id, identifiedby) values (?, ?, ?, ?)",
		&serieconfig.Name,
		&aliases,
		&serieconfig.TvdbID,
		&serieconfig.Identifiedby,
	)
	if err != nil {
		dbserie.ID = 0
		return
	}
	dbserie.ID = logger.Int64ToUint(inres)

	if config.GetSettingsGeneral().UseMediaCache {
		database.AppendCacheThreeString(
			logger.CacheDBSeries,
			database.DbstaticThreeStringTwoInt{
				Str1: serieconfig.Name,
				Str2: logger.StringToSlug(serieconfig.Name),
				Num2: dbserie.ID,
			},
		)
	}

	for idx := range serieconfig.AlternateName {
		if serieconfig.AlternateName[idx] == "" {
			continue
		}
		addalternateserietitle(&dbserie.ID, &serieconfig.AlternateName[idx])
	}
	addalternateserietitle(&dbserie.ID, &serieconfig.Name)
}

// addAlternateSerieTitle adds an alternate title for the given series ID.
// It checks if the title already exists for that ID, and if not, inserts
// it into the dbserie_alternates table. The region parameter indicates
// the language/region for the title. It also adds the title to the cache
// if enabled.
func addalternateserietitle(dbserieid *uint, title *string, regionin ...*string) {
	if database.Getdatarow[uint](
		false,
		"select count() from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE",
		dbserieid,
		title,
	) == 0 {
		slug := logger.StringToSlug(*title)

		if len(regionin) > 0 {
			database.ExecN(
				"Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)",
				title,
				&slug,
				dbserieid,
				regionin[0],
			)
		} else {
			database.ExecN("Insert into dbserie_alternates (title, slug, dbserie_id) values (?, ?, ?)", title, &slug, dbserieid)
		}
		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoString(
				logger.CacheDBSeriesAlt,
				database.DbstaticTwoStringOneInt{Str1: *title, Str2: slug, Num: *dbserieid},
			)
		}
	}
}

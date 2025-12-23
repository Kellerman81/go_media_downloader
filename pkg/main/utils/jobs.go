package utils

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

var (
	NumGo = "Num Goroutines"
	v0    uint8
	v1    uint8 = 1
)

// insertjobhistory inserts a new record into the job_histories table to track when a job starts.
// It takes the job type, media config, and current time as parameters.
// It returns the auto-generated id for the inserted row.
func insertjobhistory(jobtype string, cfgp *config.MediaTypeConfig) int64 {
	jobcategory := logger.StrSeries
	if !cfgp.Useseries {
		jobcategory = logger.StrMovie
	}

	result, err := database.ExecNid(
		"Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, datetime('now','localtime'))",
		jobtype,
		&cfgp.Name,
		&jobcategory,
	)
	if err == nil {
		return result
	}

	return 0
}

// InitialFillSeries performs the initial database fill for TV series.
// It refreshes the unmatched and files caches, inserts job history records,
// calls importnewseriessingle to import new episodes from the configured lists,
// calls getNewFilesMap to scan for new files, and clears caches when done.
func InitialFillSeries() {
	logger.Logtype("info", 0).
		Msg("Starting initial DB fill for series")

	database.Refreshunmatchedcached(true, true)
	database.Refreshfilescached(true, true)

	ctx := context.Background()

	var err error
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !media.Useseries {
			return nil
		}

		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			if idx2 > 127 {
				continue
			}

			err = importnewseriessingle(ctx, media, &media.Lists[idx2], idx2)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("Import new series failed")
			}
		}

		database.ExecN(database.QueryUpdateHistory, &dbid)

		return nil
	})
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !media.Useseries {
			return nil
		}

		dbid := insertjobhistory(logger.StrDataFull, media)
		for idxi := range media.Data {
			newfilesloop(ctx, media, &media.Data[idxi])
		}

		database.ExecN(database.QueryUpdateHistory, &dbid)

		return nil
	})

	database.ClearCaches()
}

// InitialFillMovies performs the initial database fill for movies.
// It refreshes the unmatched and files caches, inserts job history records,
// calls importnewmoviessingle to import new movies from the configured lists,
// calls newFilesMap to scan for new files, and clears caches when done.
func InitialFillMovies() {
	logger.Logtype("info", 0).
		Msg("Starting initial DB fill for movies")

	database.Refreshunmatchedcached(false, true)
	database.Refreshfilescached(false, true)

	var err error

	ctx := context.Background()
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if media.Useseries {
			return nil
		}

		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			if idx2 > 127 {
				continue
			}

			err = importnewmoviessingle(ctx, media, &media.Lists[idx2], idx2)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("Import new movies failed")
			}
		}

		database.ExecN(database.QueryUpdateHistory, &dbid)

		return nil
	})

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if media.Useseries {
			return nil
		}

		dbid := insertjobhistory(logger.StrDataFull, media)
		for idxi := range media.Data {
			newfilesloop(ctx, media, &media.Data[idxi])
		}

		database.ExecN(database.QueryUpdateHistory, &dbid)

		return nil
	})

	database.ClearCaches()
}

// FillImdb refreshes the IMDB database by calling the init_imdb executable.
// It inserts a record into the job history table, executes the IMDB update,
// reloads the IMDB database, and updates the job history record when done.
func FillImdb() {
	dbinsert, _ := database.ExecNid(
		"Insert into job_histories (job_type, job_group, job_category, started) values (?, 'RefreshImdb', ?, datetime('now','localtime'))",
		&logger.StrData,
		&logger.StrMovie,
	)

	data, err := parser.ExecCmdString[[]byte]("", logger.StrImdb)
	if err == nil {
		logger.Logtype("info", 1).
			Str("response", data).
			Msg("imdb response")
		database.ExchangeImdbDB()
	}

	if dbinsert != 0 {
		database.ExecN(database.QueryUpdateHistory, dbinsert)
	}
}

// newfilesloop processes a directory of files, checking for new or unmatched files, and importing them into the media database.
// It uses a worker pool to parallelize the file processing, and handles various checks and logic for determining the appropriate
// media list ID and importing the file data.
func newfilesloop(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	if data.CfgPath == nil {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, data.TemplatePath).
			Msg("config not found")
		return errors.New("config not found")
	}

	if cfgp == nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.TemplatePath).
			Err(logger.ErrCfgpNotFound).
			Msg("parse failed cfgp")

		return errors.New("parse failed cfgp")
	}

	pl := worker.WorkerPoolParse.NewGroupContext(ctx)
	glblistid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	err := filepath.WalkDir(
		data.CfgPath.Path,
		func(fpath string, info fs.DirEntry, errw error) error {
			if errw != nil {
				return errw
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if config.GetSettingsGeneral().UseFileCache {
				if database.SlicesCacheContains(cfgp.Useseries, logger.CacheFiles, fpath) {
					return nil
				}

				if database.SlicesCacheContains(cfgp.Useseries, logger.CacheUnmatched, fpath) {
					return nil
				}
			} else {
				if database.Getdatarow[uint](false, logger.GetStringsMap(cfgp.Useseries, logger.DBCountFilesLocation), fpath) >= 1 {
					return nil
				}

				if database.Getdatarow[uint](false, logger.GetStringsMap(cfgp.Useseries, logger.DBCountUnmatchedPath), fpath) >= 1 {
					return nil
				}
			}

			ok, _ := scanner.CheckExtensions(true, false, data.CfgPath, filepath.Ext(info.Name()))

			// Check IgnoredPaths

			if ok && data.CfgPath.BlockedLen >= 1 {
				if logger.SlicesContainsPart2I(data.CfgPath.Blocked, fpath) {
					return nil
				}
			}

			if !ok {
				return nil
			}

			pl.Submit(func() {
				defer logger.HandlePanic()

				m := parser.ParseFile(fpath, true, true, cfgp, -1)
				if m == nil {
					return // errors.New("parse failed")
				}
				defer m.Close()

				err := parser.GetDBIDs(m, cfgp, true)
				if err != nil {
					logger.Logtype("warn", 1).
						Str(logger.StrFile, fpath).
						Err(err).
						Msg(logger.ParseFailedIDs)

					return // err
				}

				listid := glblistid
				if m.ListID != -1 && glblistid == -1 {
					listid = m.ListID
				}

				if cfgp.Useseries && m.SerieID != 0 && m.ListID == -1 && listid == -1 {
					listid = database.GetMediaListIDGetListname(cfgp, &m.SerieID)
					m.ListID = listid
				}

				if !cfgp.Useseries && m.MovieID != 0 && m.ListID == -1 && listid == -1 {
					listid = database.GetMediaListIDGetListname(cfgp, &m.MovieID)
					m.ListID = listid
				}

				if listid == -1 {
					return // errors.New("listid not found")
				}

				if cfgp.Useseries {
					err = jobImportSeriesParseV2(m, fpath, cfgp, &cfgp.Lists[listid])
				} else {
					err = jobImportMovieParseV2(m, fpath, cfgp, &cfgp.Lists[listid], data.AddFound)
				}

				if err != nil {
					logger.Logtype("error", 1).
						Str(logger.StrFile, fpath).
						Err(err).
						Msg("Error Importing")

					return // err
				}
			})

			return nil
		},
	)

	errjobs := pl.Wait()
	if errjobs != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.CfgPath.Path).
			Err(errjobs).
			Msg("Error walking jobs")
	}

	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.CfgPath.Path).
			Err(err).
			Msg("Error walking directory")
	}

	return err
}

// SingleJobs runs a single maintenance job for the given media config and list.
// It handles running jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the
// job string and dispatched to internal functions. List can be empty to run for all lists.
func SingleJobs(
	rootctx context.Context,
	job, cfgpstr, listname string,
	force bool,
	key uint32,
) (finalErr error) {
	var dbinsert int64

	// Panic recovery to ensure job completion tracking
	defer func() {
		if r := recover(); r != nil {
			logger.Logtype("error", 0).
				Str("job", job).
				Uint32("key", key).
				Any("panic", logger.Stack()).
				Msg("SingleJobs: Job panicked, ensuring completion tracking")

			// Always update job history on panic
			if dbinsert != 0 {
				database.ExecN(database.QueryUpdateHistory, &dbinsert)
			}

			// Convert panic to error
			if err, ok := r.(error); ok {
				finalErr = err
			} else {
				finalErr = errors.New("job panicked")
			}
		}
	}()

	if job == "" || cfgpstr == "" || (config.GetSettingsGeneral().SchedulerDisabled && !force) {
		logjob("skipped Job", cfgpstr, listname, job)
		return nil
	}

	if err := logger.CheckContextEnded(rootctx); err != nil {
		logjob("Job cancelled - context ended", cfgpstr, listname, job)
		return err
	}

	cfgp := config.GetSettingsMedia(cfgpstr)
	if cfgpstr != "" && cfgp == nil {
		config.LoadCfgDB(true)

		cfgp = config.GetSettingsMedia(cfgpstr)
		if cfgp == nil {
			logjob("config not found", cfgpstr, listname, job)
			return errors.New("config not found")
		}

		if !cfgp.Useseries && (job == logger.StrRssSeasons || job == logger.StrRssSeasonsAll) {
			return nil
		}
	}

	logjob("Started Job", cfgpstr, listname, job)
	Refreshcache(cfgp.Useseries)

	dbinsert = insertjobhistory(job, cfgp)

	idxlist := -2
	if job == logger.StrData || job == logger.StrRss || job == logger.StrReachedFlag ||
		job == logger.StrClearHistory ||
		job == logger.StrFeeds ||
		job == logger.StrCheckMissing ||
		job == logger.StrCheckMissingFlag {
		if job == logger.StrRss {
			for idx := range cfgp.ListsQualities {
				if err := logger.CheckContextEnded(rootctx); err != nil {
					return err
				}

				searcher.NewSearcher(cfgp, config.GetSettingsQuality(cfgp.ListsQualities[idx]), logger.StrRss, nil).
					SearchRSS(rootctx, cfgp, config.GetSettingsQuality(cfgp.ListsQualities[idx]), true, true)
			}
		} else {
			if _, ok := cfgp.ListsMapIdx[listname]; ok {
				idxlist = cfgp.ListsMapIdx[listname]
			}
		}
	} else {
		idxlist = -1
	}

	var err error
	if idxlist != -2 {
		err = runjoblistfunc(rootctx, job, cfgp, idxlist)
		if err != nil {
			logger.Logtype("error", 0).
				Str("job", job).
				Uint32("key", key).
				Err(err).
				Msg("SingleJobs: Error running runjoblistfunc")
		}
	}

	logjob("Ended Job", cfgpstr, listname, job)

	// Always update job history completion, even on error
	if dbinsert != 0 {
		database.ExecN(database.QueryUpdateHistory, &dbinsert)
	}

	return err
}

// logjob logs information about a job, including the action, configuration, list name, and job name.
// It also logs the current number of goroutines.
func logjob(act, cfgp, listname, job string) {
	logger.Logtype("info", 1).
		Str(logger.StrConfig, cfgp).
		Str(logger.StrJob, job).
		Str(logger.StrListname, listname).
		Int(NumGo, runtime.NumGoroutine()).
		Msg(act)
}

// Refreshcache refreshes various database caches used for performance.
// It refreshes the history cache, media cache, media titles cache,
// unmatched cache, and files cache.
// The useseries parameter determines if it should refresh for series
// or movies.
func Refreshcache(useseries bool) {
	for _, str := range []string{logger.CacheMediaTitles, logger.CacheFiles, logger.CacheUnmatched, "CacheHistoryUrl", "CacheHistoryTitle", logger.CacheMedia, logger.CacheDBMedia} {
		database.RefreshCached(logger.GetStringsMap(useseries, str), false)
	}
}

// runjoblistfunc executes the specified job for the given media config and list index.
// It handles running various maintenance jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the job string and dispatch
// to the appropriate internal functions. List index of -1 runs the job for all lists in the config.
func runjoblistfunc(
	rootctx context.Context,
	job string,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if job == "" || cfgp == nil {
		return errors.New("job or config not found")
	}

	if err := logger.CheckContextEnded(rootctx); err != nil {
		return err
	}

	switch job {
	case logger.StrData, logger.StrDataFull:
		var err error
		for _, data := range cfgp.DataMap {
			if err := logger.CheckContextEnded(rootctx); err != nil {
				return err
			}

			if errsub := newfilesloop(rootctx, cfgp, data); errsub != nil {
				err = errsub
			}
		}

		return err

	case logger.StrCheckMissing:
		return checkmissing(rootctx, cfgp.Useseries, &cfgp.Lists[listid])
	case "cleanqueue":
		return worker.Cleanqueue()
	case logger.StrCheckMissingFlag:
		return checkmissingflag(rootctx, cfgp.Useseries, &cfgp.Lists[listid])
	case logger.StrReachedFlag:
		if !cfgp.Useseries {
			return checkreachedmoviesflag(rootctx, &cfgp.Lists[listid])
		} else {
			return checkreachedepisodesflag(rootctx, &cfgp.Lists[listid])
		}

	case logger.StrStructure:
		err := structurefolders(rootctx, cfgp)
		return err

	case logger.StrRssSeasons:
		return searcher.SearchSeriesRSSSeasons(rootctx, cfgp)
	case logger.StrRssSeasonsAll:
		return searcher.SearchSeriesRSSSeasonsAll(rootctx, cfgp)
	case "refreshinc":
		if !cfgp.Useseries {
			return refreshmovies(
				rootctx, cfgp,
				database.GetrowsN[string](
					false,
					100,
					"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100",
				),
			)
		} else {
			return refreshseries(rootctx, cfgp, database.GetrowsN[database.DbstaticTwoStringOneRInt](false, 20, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' order by updated_at asc limit 20"))
		}

	case "refresh":
		if !cfgp.Useseries {
			return refreshmovies(
				rootctx, cfgp,
				database.GetrowsN[string](
					false,
					database.Getdatarow[uint](false, "select count() from dbmovies"),
					"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id",
				),
			)
		} else {
			return refreshseries(rootctx, cfgp, database.GetrowsN[database.DbstaticTwoStringOneRInt](false, database.Getdatarow[uint](false, "select count() from dbseries"), "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries"))
		}

	case logger.StrClearHistory:
		if !cfgp.Useseries {
			database.ExecN(
				"delete from movie_histories where movie_id in (Select id from movies where listname = ? COLLATE NOCASE)",
				&cfgp.Lists[listid].Name,
			)
		} else {
			database.ExecN("delete from serie_episode_histories where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", &cfgp.Lists[listid].Name)
		}

		return nil

	case logger.StrFeeds:
		if cfgp.Lists[listid].CfgList == nil {
			logger.Logtype("error", 1).
				Str(logger.StrListname, cfgp.Lists[listid].Name).
				Msg("import feeds failed - no cfgp list")
		} else {
			var err error
			if !cfgp.Useseries {
				err = importnewmoviessingle(rootctx, cfgp, &cfgp.Lists[listid], listid)
			} else {
				err = importnewseriessingle(rootctx, cfgp, &cfgp.Lists[listid], listid)
			}

			if err != nil && !errors.Is(err, logger.ErrDisabled) {
				logger.Logtype("error", 1).
					Str(logger.StrListname, cfgp.Lists[listid].Name).
					Err(err).
					Msg("import feeds failed")
			}

			return err
		}

	case logger.StrRss:
		return nil
	case logger.StrSearchMissingFull,
		logger.StrSearchMissingInc,
		logger.StrSearchUpgradeFull,
		logger.StrSearchUpgradeInc,
		logger.StrSearchMissingFullTitle,
		logger.StrSearchMissingIncTitle,
		logger.StrSearchUpgradeFullTitle,
		logger.StrSearchUpgradeIncTitle:
		var (
			searchinterval             uint16
			searchmissing, searchtitle bool
		)

		if strings.Contains(job, "missing") {
			searchmissing = true
		}

		if strings.Contains(job, "title") {
			searchtitle = true
		}

		if strings.Contains(job, "inctitle") {
			searchtitle = true

			searchinterval = cfgp.SearchmissingIncremental
			if searchinterval == 0 {
				searchinterval = 20
			}
		}

		if strings.HasSuffix(job, "inc") {
			searchinterval = cfgp.SearchmissingIncremental
			if searchinterval == 0 {
				searchinterval = 20
			}
		}

		return jobsearchmedia(rootctx, cfgp, searchmissing, searchtitle, searchinterval)

	default:
		logger.Logtype("error", 1).
			Str(logger.StrJob, job).
			Msg("Switch not found")
		return errors.New("switch not found")
	}

	return nil
}

// jobsearchmedia searches for media items that need to be searched for new releases
// or missing files based on the provided config, search type, and interval.
// It builds a database query, executes it to get a list of media IDs,
// then calls MediaSearch on each ID to perform the actual search.
func jobsearchmedia(
	ctx context.Context, cfgp *config.MediaTypeConfig,
	searchmissing, searchtitle bool,
	searchinterval uint16,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	var (
		scaninterval int
		scandatepre  int
	)

	if cfgp.DataLen >= 1 && cfgp.Data[0].CfgPath != nil {
		if searchmissing {
			scaninterval = cfgp.Data[0].CfgPath.MissingScanInterval
			scandatepre = cfgp.Data[0].CfgPath.MissingScanReleaseDatePre
		} else {
			scaninterval = cfgp.Data[0].CfgPath.UpgradeScanInterval
		}
	}

	if cfgp.ListsLen == 0 {
		return errors.New("no lists")
	}

	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	if len(args.Arr) == 0 {
		return errors.New("no lists")
	}

	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	bld.WriteStringMap(cfgp.Useseries, logger.SearchGenTable)

	if searchmissing {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenMissing)
		bld.WriteString(cfgp.ListsQu)
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenMissingEnd)
	} else {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenReached)
		bld.WriteString(cfgp.ListsQu)
		bld.WriteByte(')')
	}

	if scaninterval != 0 {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenLastScan)

		timeinterval := logger.TimeGetNow().AddDate(0, 0, 0-scaninterval)

		args.Arr = append(args.Arr, &timeinterval)
	}

	if scandatepre != 0 {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenDate)

		timedatepre := logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)

		args.Arr = append(args.Arr, &timedatepre)
	}

	bld.WriteStringMap(cfgp.Useseries, logger.SearchGenOrder)

	if searchinterval != 0 {
		bld.WriteString(" limit ")
		bld.WriteUInt16(searchinterval)
	}

	str := bld.String()

	var err error
	for _, tbl := range database.GetrowsNuncached[database.DbstaticOneStringOneUInt](database.Getdatarow[uint](false, logger.JoinStrings("select count() ", str), args.Arr...), logger.JoinStrings(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenSelect), str), args.Arr) {
		if errsub := searcher.NewSearcher(cfgp, cfgp.GetMediaQualityConfigStr(tbl.Str), "", nil).
			MediaSearch(ctx, cfgp, tbl.Num, searchtitle, true, true); errsub != nil {
			err = errsub
		}
	}

	return err
}

// checkmissing checks for missing files for the given media list.
// It queries for file locations, checks if they exist, and updates
// the database to set missing flags on media items with no files.
// It handles both movies and series based on the useseries flag.
func checkmissing(rootctx context.Context, useseries bool, listcfg *config.MediaListsConfig) error {
	arrfiles := database.GetrowsN[syncops.DbstaticOneStringTwoInt](
		false,
		database.Getdatarow[uint](
			false,
			logger.GetStringsMap(useseries, logger.DBCountFilesByLocation),
		),
		logger.GetStringsMap(useseries, logger.DBIDsFilesByLocation),
	)
	arr := database.Getrowssize[string](
		false,
		logger.GetStringsMap(useseries, logger.DBCountFilesByList),
		logger.GetStringsMap(useseries, logger.DBLocationFilesByList),
		&listcfg.Name,
	)

	var err error
	for idx := range arr {
		if err := logger.CheckContextEnded(rootctx); err != nil {
			return err
		}

		if scanner.CheckFileExist(arr[idx]) {
			continue
		}

		if errsub := checkmissingfiles(useseries, &arr[idx], arrfiles); errsub != nil {
			err = errsub
		}
	}

	return err
}

// func Checkruntimes(cfg *config.MediaTypeConfig) {
// 	arr := database.GetrowsN[database.DbstaticOneStringTwoInt](false, database.Getdatarow(false, logger.GetStringsMap(cfg.Useseries, logger.DBCountFilesByLocation)), logger.GetStringsMap(cfg.Useseries, logger.DBIDsFilesByLocation))
// 	for idx := range arr {
// 		Checkruntimesfiles(cfg, &arr[idx])
// 	}
// }

// checkmissingfiles checks for missing files for a given media item.
// It deletes the file record if missing, and updates the missing flag on the media item if it has no more files.
// It takes the query to count files for the media item, the table to delete from,
// the table to update the missing flag, the query to get the file ID and media item ID,
// and the file location that was found missing.
func checkmissingfiles(
	useseries bool,
	row *string,
	arrfiles []syncops.DbstaticOneStringTwoInt,
) error {
	subquerycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)
	deletestmt := logger.JoinStrings(
		"delete from ",
		logger.GetStringsMap(useseries, logger.TableFiles),
		" where id = ?",
	)
	updatestmt := logger.JoinStrings(
		"update ",
		logger.GetStringsMap(useseries, logger.TableMedia),
		" set missing = 1 where id = ?",
	)

	var errret error
	for idx := range arrfiles {
		if arrfiles[idx].Str != *row {
			continue
		}

		logger.Logtype("info", 1).
			Str(logger.StrFile, *row).
			Msg("File was removed")

		err := database.ExecNErr(deletestmt, &arrfiles[idx].Num1)
		if err != nil {
			errret = err
			continue
		}

		if database.Getdatarow[uint](false, subquerycount, &arrfiles[idx].Num2) == 0 {
			database.ExecN(updatestmt, &arrfiles[idx].Num2)
		}
	}

	return errret
}

// checkmissingflag checks for missing files for the given media list.
// It updates the missing flag in the database based on file count.
func checkmissingflag(
	rootctx context.Context,
	useseries bool,
	listcfg *config.MediaListsConfig,
) error {
	queryupdate := logger.GetStringsMap(useseries, logger.DBUpdateMissing)
	querycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)

	var counter int

	arr := database.Getrowssize[database.DbstaticOneIntOneBool](
		false,
		logger.GetStringsMap(useseries, logger.DBCountMediaByList),
		logger.GetStringsMap(useseries, logger.DBIDMissingMediaByList),
		&listcfg.Name,
	)
	for idx := range arr {
		if err := logger.CheckContextEnded(rootctx); err != nil {
			return err
		}

		database.Scanrowsdyn(false, querycount, &counter, &arr[idx].Num)

		if counter >= 1 && arr[idx].Bl {
			database.ExecN(queryupdate, &v0, &arr[idx].Num)
		}

		if counter == 0 && !arr[idx].Bl {
			database.ExecN(queryupdate, &v1, &arr[idx].Num)
		}
	}

	return nil
}

// LoadGlobalSchedulerConfig initializes the global scheduler job functions that are
// not media-specific. These jobs include database maintenance, backup operations,
// and system-wide tasks. The functions are registered in the general settings
// job map for use by the worker scheduler system.
func LoadGlobalSchedulerConfig() {
	config.GetSettingsGeneral().Jobs = map[string]func(uint32, context.Context) error{
		"RefreshImdb": func(key uint32, ctx context.Context) error {
			FillImdb()
			worker.RemoveQueueEntry(key)
			return nil
		},
		"CheckDatabase": func(key uint32, ctx context.Context) error {
			worker.RemoveQueueEntry(key)

			if database.DBIntegrityCheck() != "ok" {
				os.Exit(100)
			}

			return nil
		},
		"BackupDatabase": func(key uint32, ctx context.Context) error {
			if config.GetSettingsGeneral().DatabaseBackupStopTasks {
				worker.StopCronWorker()
				worker.CloseWorkerPools()
			}

			worker.RemoveQueueEntry(key)

			backupto := logger.JoinStrings(
				"./backup/data.db.",
				database.GetVersion(),
				logger.StrDot,
				time.Now().Format("20060102_150405"),
			)

			err := database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)
			if config.GetSettingsGeneral().DatabaseBackupStopTasks {
				worker.InitWorkerPools(
					config.GetSettingsGeneral().WorkerSearch,
					config.GetSettingsGeneral().WorkerFiles,
					config.GetSettingsGeneral().WorkerMetadata,
					config.GetSettingsGeneral().WorkerRSS,
					config.GetSettingsGeneral().WorkerIndexer,
				)
				worker.StartCronWorker()
			}

			return err
		},
		"RefreshCache": func(key uint32, ctx context.Context) error {
			logger.Logtype("info", 0).Msg("Starting scheduled cache refresh")

			// Refresh all cache types individually for better control
			cacheTypes := []string{
				logger.CacheMovie,
				logger.CacheSeries,
				logger.CacheDBMovie,
				logger.CacheDBSeries,
				logger.CacheDBSeriesAlt,
				logger.CacheTitlesMovie,
				logger.CacheUnmatchedMovie,
				logger.CacheUnmatchedSeries,
				logger.CacheFilesMovie,
				logger.CacheFilesSeries,
				logger.CacheHistoryURLMovie,
				logger.CacheHistoryTitleMovie,
				logger.CacheHistoryURLSeries,
				logger.CacheHistoryTitleSeries,
			}

			for _, cacheType := range cacheTypes {
				logger.Logtype("debug", 0).Str("type", cacheType).Msg("Refreshing cache")
				database.RefreshCached(cacheType, true)
			}

			logger.Logtype("info", 0).Msg("Completed scheduled cache refresh")
			worker.RemoveQueueEntry(key)

			return nil
		},
	}
}

// LoadSchedulerConfig initializes the media-specific scheduler job functions for each
// configured media type (movies and series). These jobs include search operations,
// data processing, feed imports, and maintenance tasks. The function iterates through
// all media configurations and registers appropriate job functions based on whether
// the media type is for series or movies.
func LoadSchedulerConfig() {
	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		if cfgp.Useseries {
			cfgp.Jobs = map[string]func(uint32, context.Context) error{
				logger.StrSearchMissingInc: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)

					return SingleJobs(
						ctx,
						logger.StrSearchMissingInc,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
				},
				logger.StrSearchMissingFull: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)

					return SingleJobs(
						ctx,
						logger.StrSearchMissingFull,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
				},
				logger.StrSearchUpgradeInc: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)

					return SingleJobs(
						ctx,
						logger.StrSearchUpgradeInc,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
				},
				logger.StrSearchUpgradeFull: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)

					return SingleJobs(
						ctx,
						logger.StrSearchUpgradeFull,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
				},
				logger.StrSearchMissingIncTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx,
						logger.StrSearchMissingIncTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					defer worker.RemoveQueueEntry(key)

					return err
				},
				logger.StrSearchMissingFullTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx,
						logger.StrSearchMissingFullTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					defer worker.RemoveQueueEntry(key)

					return err
				},
				logger.StrSearchUpgradeIncTitle: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)

					return SingleJobs(ctx,
						logger.StrSearchUpgradeIncTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
				},
				logger.StrSearchUpgradeFullTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx,
						logger.StrSearchUpgradeFullTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					defer worker.RemoveQueueEntry(key)

					return err
				},
				logger.StrRss: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrRss, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrDataFull: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrDataFull, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrStructure: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrStructure, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrFeeds: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrFeeds, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrCheckMissing: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrCheckMissing, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrCheckMissingFlag: func(key uint32, ctx context.Context) error {
					err := SingleJobs(
						ctx,
						logger.StrCheckMissingFlag,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					defer worker.RemoveQueueEntry(key)

					return err
				},
				logger.StrUpgradeFlag: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrUpgradeFlag, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrRssSeasons: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrRssSeasons, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrRssSeasonsAll: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrRssSeasonsAll, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				"refreshseriesfull": func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, "refresh", cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				"refreshseriesinc": func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, "refreshinc", cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
			}
		} else {
			cfgp.Jobs = map[string]func(uint32, context.Context) error{
				logger.StrSearchMissingInc: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)
					return SingleJobs(ctx, logger.StrSearchMissingInc, cfgp.NamePrefix, "", false, key)
				},
				logger.StrSearchMissingFull: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)
					return SingleJobs(ctx, logger.StrSearchMissingFull, cfgp.NamePrefix, "", false, key)
				},
				logger.StrSearchUpgradeInc: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)
					return SingleJobs(ctx, logger.StrSearchUpgradeInc, cfgp.NamePrefix, "", false, key)
				},
				logger.StrSearchUpgradeFull: func(key uint32, ctx context.Context) error {
					defer worker.RemoveQueueEntry(key)
					return SingleJobs(ctx, logger.StrSearchUpgradeFull, cfgp.NamePrefix, "", false, key)
				},
				logger.StrSearchMissingIncTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrSearchMissingIncTitle, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrSearchMissingFullTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrSearchMissingFullTitle, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrSearchUpgradeIncTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrSearchUpgradeIncTitle, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrSearchUpgradeFullTitle: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrSearchUpgradeFullTitle, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrRss: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrRss, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrDataFull: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrDataFull, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrStructure: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrStructure, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrFeeds: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrFeeds, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrCheckMissing: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrCheckMissing, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrCheckMissingFlag: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrCheckMissingFlag, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				logger.StrUpgradeFlag: func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, logger.StrUpgradeFlag, cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				"refreshmoviesfull": func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, "refresh", cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
				"refreshmoviesinc": func(key uint32, ctx context.Context) error {
					err := SingleJobs(ctx, "refreshinc", cfgp.NamePrefix, "", false, key)
					defer worker.RemoveQueueEntry(key)
					return err
				},
			}
		}

		return nil
	})
}

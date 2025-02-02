package utils

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
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
	result, err := database.ExecNid("Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, datetime('now','localtime'))", jobtype, &cfgp.Name, &jobcategory)
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
	logger.LogDynamicany0("info", "Starting initial DB fill for series")

	database.Refreshunmatchedcached(true, true)
	database.Refreshfilescached(true, true)

	var err error
	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			if idx2 > 127 {
				continue
			}
			err = importnewseriessingle(media, &media.Lists[idx2], idx2)
			if err != nil {
				logger.LogDynamicanyErr("error", "Import new series failed", err)
			}
		}
		database.Exec1(database.QueryUpdateHistory, &dbid)
	}
	ctx := context.Background()
	defer ctx.Done()
	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrDataFull, media)
		for idxi := range media.Data {
			newfilesloop(ctx, media, &media.Data[idxi])
		}
		database.Exec1(database.QueryUpdateHistory, &dbid)
	}
	ctx.Done()

	database.ClearCaches()
}

// InitialFillMovies performs the initial database fill for movies.
// It refreshes the unmatched and files caches, inserts job history records,
// calls importnewmoviessingle to import new movies from the configured lists,
// calls newFilesMap to scan for new files, and clears caches when done.
func InitialFillMovies() {
	logger.LogDynamicany0("info", "Starting initial DB fill for movies")

	database.Refreshunmatchedcached(false, true)
	database.Refreshfilescached(false, true)
	var err error
	for _, media := range config.SettingsMedia {
		if media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			if idx2 > 127 {
				continue
			}
			err = importnewmoviessingle(media, &media.Lists[idx2], idx2)
			if err != nil {
				logger.LogDynamicanyErr("error", "Import new movies failed", err)
			}
		}
		database.Exec1(database.QueryUpdateHistory, dbid)
	}

	ctx := context.Background()
	defer ctx.Done()
	for _, media := range config.SettingsMedia {
		if media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrDataFull, media)

		for idxi := range media.Data {
			newfilesloop(ctx, media, &media.Data[idxi])
		}
		database.Exec1(database.QueryUpdateHistory, dbid)
	}
	ctx.Done()

	database.ClearCaches()
}

// FillImdb refreshes the IMDB database by calling the init_imdb executable.
// It inserts a record into the job history table, executes the IMDB update,
// reloads the IMDB database, and updates the job history record when done.
func FillImdb() {
	dbinsert, _ := database.ExecNid("Insert into job_histories (job_type, job_group, job_category, started) values (?, 'RefreshImdb', ?, datetime('now','localtime'))", &logger.StrData, &logger.StrMovie)
	data, err := parser.ExecCmdString[[]byte]("", logger.StrImdb)
	if err == nil {
		logger.LogDynamicany1String("info", "imdb response", "response", data)
		database.ExchangeImdbDB()
	}
	if dbinsert != 0 {
		database.Exec1(database.QueryUpdateHistory, dbinsert)
	}
}

// newfilesloop processes a directory of files, checking for new or unmatched files, and importing them into the media database.
// It uses a worker pool to parallelize the file processing, and handles various checks and logic for determining the appropriate
// media list ID and importing the file data.
func newfilesloop(ctx context.Context, cfgp *config.MediaTypeConfig, data *config.MediaDataConfig) {
	if data.CfgPath == nil {
		logger.LogDynamicany1String("error", "config not found", logger.StrConfig, data.TemplatePath)
		return
	}
	if cfgp == nil {
		logger.LogDynamicany1StringErr("error", "parse failed cfgp", logger.ErrCfgpNotFound, logger.StrFile, data.TemplatePath)
		return
	}

	pl := worker.WorkerPoolParse.NewGroupContext(ctx)
	glblistid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	filepath.WalkDir(data.CfgPath.Path, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if err := ctx.Err(); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}
		if config.SettingsGeneral.UseFileCache {
			if database.SlicesCacheContains(cfgp.Useseries, logger.CacheFiles, fpath) {
				return nil
			}
			if database.SlicesCacheContains(cfgp.Useseries, logger.CacheUnmatched, fpath) {
				return nil
			}
		} else {
			if database.Getdatarow1[uint](false, logger.GetStringsMap(cfgp.Useseries, logger.DBCountFilesLocation), fpath) >= 1 {
				return nil
			}
			if database.Getdatarow1[uint](false, logger.GetStringsMap(cfgp.Useseries, logger.DBCountUnmatchedPath), fpath) >= 1 {
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

		pl.SubmitErr(func() error {
			defer logger.HandlePanic()
			m := parser.ParseFile(fpath, true, true, cfgp, -1)
			if m == nil {
				return nil
			}
			defer m.Close()

			err := parser.GetDBIDs(m, cfgp, true)
			if err != nil {
				logger.LogDynamicany1StringErr("warn", logger.ParseFailedIDs, err, logger.StrFile, fpath)
				return nil
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
				return nil
			}
			if cfgp.Useseries {
				err = jobImportSeriesParseV2(m, fpath, cfgp, &cfgp.Lists[listid])
			} else {
				err = jobImportMovieParseV2(m, fpath, cfgp, &cfgp.Lists[listid], data.AddFound)
			}

			if err != nil {
				logger.LogDynamicany1StringErr("error", "Error Importing", err, logger.StrFile, fpath)
			}
			return nil
		})
		return nil
	})

	pl.Wait()
}

// SingleJobs runs a single maintenance job for the given media config and list.
// It handles running jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the
// job string and dispatched to internal functions. List can be empty to run for all lists.
func SingleJobs(job, cfgpstr, listname string, force bool, key uint32) {
	defer worker.RemoveQueueEntry(key)
	if job == "" || cfgpstr == "" || (config.SettingsGeneral.SchedulerDisabled && !force) {
		logjob("skipped Job", cfgpstr, listname, job)
		return
	}

	cfgp := config.SettingsMedia[cfgpstr]
	if cfgpstr != "" && cfgp == nil {
		config.LoadCfgDB()
		cfgp = config.SettingsMedia[cfgpstr]
		if cfgp == nil {
			logjob("config not found", cfgpstr, listname, job)
			return
		}
		if !cfgp.Useseries && (job == logger.StrRssSeasons || job == logger.StrRssSeasonsAll) {
			return
		}
	}
	logjob("Started Job", cfgpstr, listname, job)
	Refreshcache(cfgp.Useseries)
	dbinsert := insertjobhistory(job, cfgp)

	idxlist := -2
	if job == logger.StrData || job == logger.StrRss || job == logger.StrReachedFlag || job == logger.StrClearHistory || job == logger.StrFeeds || job == logger.StrCheckMissing || job == logger.StrCheckMissingFlag {
		if job == logger.StrRss {
			ctx := context.Background()
			defer ctx.Done()
			for idx := range cfgp.ListsQualities {
				searcher.NewSearcher(cfgp, config.SettingsQuality[cfgp.ListsQualities[idx]], logger.StrRss, nil).SearchRSS(ctx, cfgp, config.SettingsQuality[cfgp.ListsQualities[idx]], true, true)
			}
			ctx.Done()
		} else {
			if _, ok := cfgp.ListsMapIdx[listname]; ok {
				idxlist = cfgp.ListsMapIdx[listname]
			}
		}
	} else {
		idxlist = -1
	}
	if idxlist != -2 {
		runjoblistfunc(job, cfgp, idxlist)
	}
	worker.RemoveQueueEntry(key)
	logjob("Ended Job", cfgpstr, listname, job)

	if dbinsert != 0 {
		database.Exec1(database.QueryUpdateHistory, dbinsert)
	}
}

// logjob logs information about a job, including the action, configuration, list name, and job name.
// It also logs the current number of goroutines.
func logjob(act, cfgp, listname, job string) {
	logger.Logtype("info", 1).Str(logger.StrConfig, cfgp).Str(logger.StrJob, job).Str(logger.StrListname, listname).Int(NumGo, runtime.NumGoroutine()).Msg(act)
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
func runjoblistfunc(job string, cfgp *config.MediaTypeConfig, listid int) {
	if job == "" || cfgp == nil {
		return
	}
	switch job {
	case logger.StrData, logger.StrDataFull:
		ctx := context.Background()
		defer ctx.Done()
		for _, data := range cfgp.DataMap {
			newfilesloop(ctx, cfgp, data)
		}
		ctx.Done()
	case logger.StrCheckMissing:
		checkmissing(cfgp.Useseries, &cfgp.Lists[listid])
	case "cleanqueue":
		worker.Cleanqueue()
	case logger.StrCheckMissingFlag:
		checkmissingflag(cfgp.Useseries, &cfgp.Lists[listid])
	case logger.StrReachedFlag:
		if !cfgp.Useseries {
			checkreachedmoviesflag(&cfgp.Lists[listid])
		} else {
			checkreachedepisodesflag(&cfgp.Lists[listid])
		}
	case logger.StrStructure:
		ctx := context.Background()
		defer ctx.Done()
		structurefolders(ctx, cfgp)
		ctx.Done()
	case logger.StrRssSeasons:
		searcher.SearchSeriesRSSSeasons(cfgp)
	case logger.StrRssSeasonsAll:
		searcher.SearchSeriesRSSSeasonsAll(cfgp)
	case "refreshinc":
		if !cfgp.Useseries {
			refreshmovies(cfgp, database.Getrows0[string](false, 100, "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100"))
		} else {
			refreshseries(cfgp, database.Getrows0[database.DbstaticTwoStringOneRInt](false, 20, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20"))
		}
	case "refresh":
		if !cfgp.Useseries {
			refreshmovies(cfgp, database.Getrows0[string](false, database.Getdatarow0(false, "select count() from dbmovies"), "select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id group by dbmovies.imdb_id"))
		} else {
			refreshseries(cfgp, database.Getrows0[database.DbstaticTwoStringOneRInt](false, database.Getdatarow0(false, "select count() from dbseries"), "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0"))
		}
	case logger.StrClearHistory:
		if !cfgp.Useseries {
			database.Exec1("delete from movie_histories where movie_id in (Select id from movies where listname = ? COLLATE NOCASE)", &cfgp.Lists[listid].Name)
		} else {
			database.Exec1("delete from serie_episode_histories where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", &cfgp.Lists[listid].Name)
		}
	case logger.StrFeeds:
		if cfgp.Lists[listid].CfgList == nil {
			logger.LogDynamicany1String("error", "import feeds failed - no cfgp list", logger.StrListname, cfgp.Lists[listid].Name)
		} else {
			var err error
			if !cfgp.Useseries {
				err = importnewmoviessingle(cfgp, &cfgp.Lists[listid], listid)
			} else {
				err = importnewseriessingle(cfgp, &cfgp.Lists[listid], listid)
			}
			if err != nil && !errors.Is(err, logger.ErrDisabled) {
				logger.LogDynamicany1StringErr("error", "import feeds failed", err, logger.StrListname, cfgp.Lists[listid].Name)
			}
		}
	case logger.StrRss:
		return
	case logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
		var searchinterval uint16
		var searchmissing, searchtitle bool
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
		jobsearchmedia(cfgp, searchmissing, searchtitle, searchinterval)
	default:
		logger.LogDynamicany1String("error", "Switch not found", logger.StrJob, job) // logpointerr
	}
}

// jobsearchmedia searches for media items that need to be searched for new releases
// or missing files based on the provided config, search type, and interval.
// It builds a database query, executes it to get a list of media IDs,
// then calls MediaSearch on each ID to perform the actual search.
func jobsearchmedia(cfgp *config.MediaTypeConfig, searchmissing, searchtitle bool, searchinterval uint16) {
	var scaninterval int
	var scandatepre int
	if cfgp.DataLen >= 1 && cfgp.Data[0].CfgPath != nil {
		if searchmissing {
			scaninterval = cfgp.Data[0].CfgPath.MissingScanInterval
			scandatepre = cfgp.Data[0].CfgPath.MissingScanReleaseDatePre
		} else {
			scaninterval = cfgp.Data[0].CfgPath.UpgradeScanInterval
		}
	}

	if cfgp.ListsLen == 0 {
		return
	}
	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)
	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}
	if len(args.Arr) == 0 {
		return
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
	ctx := context.Background()
	for _, tbl := range database.GetrowsNuncached[database.DbstaticOneStringOneUInt](database.GetdatarowNArg(false, logger.JoinStrings("select count() ", str), args.Arr), logger.JoinStrings(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenSelect), str), args.Arr) {
		searcher.NewSearcher(cfgp, cfgp.GetMediaQualityConfigStr(tbl.Str), "", nil).MediaSearch(ctx, cfgp, tbl.Num, searchtitle, true, true)
	}
}

// checkmissing checks for missing files for the given media list.
// It queries for file locations, checks if they exist, and updates
// the database to set missing flags on media items with no files.
// It handles both movies and series based on the useseries flag.
func checkmissing(useseries bool, listcfg *config.MediaListsConfig) {
	arrfiles := database.Getrows0[database.DbstaticOneStringTwoInt](false, database.Getdatarow0(false, logger.GetStringsMap(useseries, logger.DBCountFilesByLocation)), logger.GetStringsMap(useseries, logger.DBIDsFilesByLocation))
	arr := database.Getrows1size[string](false, logger.GetStringsMap(useseries, logger.DBCountFilesByList), logger.GetStringsMap(useseries, logger.DBLocationFilesByList), &listcfg.Name)
	for idx := range arr {
		if scanner.CheckFileExist(arr[idx]) {
			continue
		}
		checkmissingfiles(useseries, &arr[idx], arrfiles)
	}
}

// func Checkruntimes(cfg *config.MediaTypeConfig) {
// 	arr := database.Getrows0[database.DbstaticOneStringTwoInt](false, database.Getdatarow0(false, logger.GetStringsMap(cfg.Useseries, logger.DBCountFilesByLocation)), logger.GetStringsMap(cfg.Useseries, logger.DBIDsFilesByLocation))
// 	for idx := range arr {
// 		Checkruntimesfiles(cfg, &arr[idx])
// 	}
// }

// checkmissingfiles checks for missing files for a given media item.
// It deletes the file record if missing, and updates the missing flag on the media item if it has no more files.
// It takes the query to count files for the media item, the table to delete from,
// the table to update the missing flag, the query to get the file ID and media item ID,
// and the file location that was found missing.
func checkmissingfiles(useseries bool, row *string, arrfiles []database.DbstaticOneStringTwoInt) {
	subquerycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)
	deletestmt := logger.JoinStrings("delete from ", logger.GetStringsMap(useseries, logger.TableFiles), " where id = ?")
	updatestmt := logger.JoinStrings("update ", logger.GetStringsMap(useseries, logger.TableMedia), " set missing = 1 where id = ?")
	var err error
	for idx := range arrfiles {
		if arrfiles[idx].Str != *row {
			continue
		}
		logger.LogDynamicany1String("info", "File was removed", logger.StrFile, *row)
		err = database.Exec1Err(deletestmt, &arrfiles[idx].Num1)
		if err != nil {
			continue
		}

		if database.Getdatarow1[uint](false, subquerycount, &arrfiles[idx].Num2) == 0 {
			database.Exec1(updatestmt, &arrfiles[idx].Num2)
		}
	}
}

// checkmissingflag checks for missing files for the given media list.
// It updates the missing flag in the database based on file count.
func checkmissingflag(useseries bool, listcfg *config.MediaListsConfig) {
	queryupdate := logger.GetStringsMap(useseries, logger.DBUpdateMissing)
	querycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)

	var counter int

	arr := database.Getrows1size[database.DbstaticOneIntOneBool](false, logger.GetStringsMap(useseries, logger.DBCountMediaByList), logger.GetStringsMap(useseries, logger.DBIDMissingMediaByList), &listcfg.Name)
	for idx := range arr {
		database.Scanrows1dyn(false, querycount, &counter, &arr[idx].Num)
		if counter >= 1 && arr[idx].Bl {
			database.Exec2(queryupdate, &v0, &arr[idx].Num)
		}
		if counter == 0 && !arr[idx].Bl {
			database.Exec2(queryupdate, &v1, &arr[idx].Num)
		}
	}
}

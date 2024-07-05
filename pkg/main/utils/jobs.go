package utils

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// insertjobhistory inserts a new record into the job_histories table to track when a job starts.
// It takes the job type, media config, and current time as parameters.
// It returns the auto-generated id for the inserted row.
func insertjobhistory(jobtype string, cfgp *config.MediaTypeConfig) int64 {
	jobcategory := logger.StrSeries
	if !cfgp.Useseries {
		jobcategory = logger.StrMovie
	}
	result, err := database.ExecNid("Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, datetime('now','localtime'))", jobtype, &cfgp.Name, &jobcategory) //sqlpointerr
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
	logger.LogDynamicany("info", "Starting initial DB fill for series")

	database.Refreshunmatchedcached(true)
	database.Refreshfilescached(true)

	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			err := importnewseriessingle(media, &media.Lists[idx2], int8(idx2))

			if err != nil {
				logger.LogDynamicany("error", "Import new series failed", err)
			}
		}
		database.Exec1(database.QueryUpdateHistory, &dbid)
	}
	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrDataFull, media)
		for idxi := range media.Data {
			newfilesloop(media, &media.Data[idxi])
		}
		database.Exec1(database.QueryUpdateHistory, &dbid)
	}

	database.ClearCaches()
}

// InitialFillMovies performs the initial database fill for movies.
// It refreshes the unmatched and files caches, inserts job history records,
// calls importnewmoviessingle to import new movies from the configured lists,
// calls newFilesMap to scan for new files, and clears caches when done.
func InitialFillMovies() {
	logger.LogDynamicany("info", "Starting initial DB fill for movies")

	database.Refreshunmatchedcached(false)
	database.Refreshfilescached(false)
	for _, media := range config.SettingsMedia {
		if media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			err := importnewmoviessingle(media, &media.Lists[idx2], int8(idx2))
			if err != nil {
				logger.LogDynamicany("error", "Import new movies failed", err)
			}
		}
		database.Exec1(database.QueryUpdateHistory, &dbid)
	}

	for _, media := range config.SettingsMedia {
		if media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrDataFull, media)

		for idxi := range media.Data {
			newfilesloop(media, &media.Data[idxi])
		}
		database.Exec1(database.QueryUpdateHistory, &dbid)
	}

	database.ClearCaches()
}

// getImdbFilename returns the path to the init_imdb executable
// based on the current OS. For Windows it returns init_imdb.exe,
// for other OSes it returns ./init_imdb.
func getImdbFilename() string {
	var p string
	if runtime.GOOS == "windows" {
		p = "init_imdb.exe"
	} else {
		p = "./init_imdb"
	}
	// if !filepath.IsAbs(p) {
	// 	p, _ = filepath.Abs(p)
	// }
	return p
}

// FillImdb refreshes the IMDB database by calling the init_imdb executable.
// It inserts a record into the job history table, executes the IMDB update,
// reloads the IMDB database, and updates the job history record when done.
func FillImdb() {
	dbinsert, _ := database.ExecNid("Insert into job_histories (job_type, job_group, job_category, started) values (?, 'RefreshImdb', ?, datetime('now','localtime'))", &logger.StrData, &logger.StrMovie)
	data, _, err := parser.ExecCmd(getImdbFilename(), "", logger.StrImdb)
	if err == nil {
		logger.LogDynamicany("info", "imdb response", "repsonse", data)
		database.ExchangeImdbDB()
	}
	if dbinsert != 0 {
		database.Exec1(database.QueryUpdateHistory, &dbinsert)
	}
	//clear(data)
}

// newfilesloop processes a directory of files, checking for new or unmatched files, and importing them into the media database.
// It uses a worker pool to parallelize the file processing, and handles various checks and logic for determining the appropriate
// media list ID and importing the file data.
func newfilesloop(cfgp *config.MediaTypeConfig, data *config.MediaDataConfig) {
	if data.CfgPath == nil {
		logger.LogDynamicany("error", "config not found", "template", &data.TemplatePath)
		return
	}
	if cfgp == nil {
		logger.LogDynamicany("error", "parse failed cfgp", &logger.ErrCfgpNotFound, &logger.StrFile, &data.TemplatePath)
		return
	}

	//newFilesloop(cfgp, &cfgp.Data[idxi])
	//workergroup := worker.GetPoolParserGroup()
	var toprocess []string
	_ = filepath.WalkDir(data.CfgPath.Path, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}

		//CheckUnmatched
		if config.SettingsGeneral.UseFileCache {
			if database.SlicesCacheContains(cfgp.Useseries, logger.CacheFiles, fpath) {
				return nil
			}
			if database.SlicesCacheContains(cfgp.Useseries, logger.CacheUnmatched, fpath) {
				return nil
			}
		} else {
			if database.Getdatarow1Map[uint](false, cfgp.Useseries, logger.DBCountFilesLocation, fpath) >= 1 { //sqlpointerr
				return nil
			}
			if database.Getdatarow1Map[uint](false, cfgp.Useseries, logger.DBCountUnmatchedPath, fpath) >= 1 { //sqlpointerr
				return nil
			}
		}
		ok, _ := scanner.CheckExtensions(true, false, data.CfgPath, filepath.Ext(info.Name()))

		//Check IgnoredPaths

		if ok && data.CfgPath.BlockedLen >= 1 {
			if logger.SlicesContainsPart2I(data.CfgPath.Blocked, fpath) {
				return nil
			}
		}
		if !ok {
			return nil
		}

		toprocess = append(toprocess, fpath)

		return nil
	})
	wg := pool.NewSizedGroup(int(config.SettingsGeneral.WorkerParse))
	glblistid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
	for idx := range toprocess {
		wg.Add()
		go processnewfolder(wg, toprocess[idx], cfgp, glblistid, data)
	}
	wg.Wait()
	wg.Close()
	clear(toprocess)
}

func processnewfolder(wg *pool.SizedWaitGroup, fpath string, cfgp *config.MediaTypeConfig, glblistid int8, data *config.MediaDataConfig) {
	defer logger.HandlePanic()
	defer wg.Done()
	m := parser.ParseFile(fpath, true, true, cfgp, -1)
	if m == nil {
		return
	}
	defer m.Close()

	err := parser.GetDBIDs(m, cfgp, true)
	if err != nil {
		//m.TempTitle = fpath
		m.LogTempTitle("warn", logger.ParseFailedIDs, err, fpath)
		return
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
		return
	}
	if cfgp.Useseries {
		err = jobImportSeriesParseV2(m, fpath, cfgp, &cfgp.Lists[listid])
	} else {
		err = jobImportMovieParseV2(m, fpath, cfgp, &cfgp.Lists[listid], data.AddFound)
	}

	if err != nil {
		//m.TempTitle = fpath
		m.LogTempTitle("error", "Error Importing", err, fpath)
	}
}

// SingleJobs runs a single maintenance job for the given media config and list.
// It handles running jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the
// job string and dispatched to internal functions. List can be empty to run for all lists.
func SingleJobs(job, cfgpstr, listname string, force bool) {
	if job == "" || cfgpstr == "" {
		logger.LogDynamicany("info", "skipped job", &logger.StrJob, job, &logger.StrConfig, cfgpstr, &logger.StrListname, listname)
		return
	}
	if config.SettingsGeneral.SchedulerDisabled && !force {
		logger.LogDynamicany("debug", "skipped job", &logger.StrJob, job)
		return
	}
	if !config.SettingsMedia[cfgpstr].Useseries && (job == logger.StrRssSeasons || job == logger.StrRssSeasonsAll) {
		return
	}
	logjob("Started Job", cfgpstr, listname, job)
	//logger.LogDynamicany("info", "Started Job", &logger.StrConfig, cfgpstr, &logger.StrJob, job, &logger.StrListname, listname, "Num Goroutines", runtime.NumGoroutine())

	cfgp := config.SettingsMedia[cfgpstr]
	if cfgpstr != "" && cfgp == nil {
		config.LoadCfgDB()
		cfgp = config.SettingsMedia[cfgpstr]
		if cfgp == nil {
			logger.LogDynamicany("error", "config not found", &logger.StrConfig, cfgp)
			return
		}
	}
	Refreshcache(cfgp.Useseries)
	dbinsert := insertjobhistory(job, cfgp)

	if job == logger.StrData || job == logger.StrRss || job == logger.StrReachedFlag || job == logger.StrClearHistory || job == logger.StrFeeds || job == logger.StrCheckMissing || job == logger.StrCheckMissingFlag {
		if job == logger.StrRss {
			for idx := range cfgp.ListsQualities {
				searcher.NewSearcher(cfgp, config.SettingsQuality[cfgp.ListsQualities[idx]], logger.StrRss, 0).SearchRSS(cfgp, config.SettingsQuality[cfgp.ListsQualities[idx]], true, true)
			}
		} else {
			for idxlist := range cfgp.Lists {
				if listname != "" && listname != cfgp.Lists[idxlist].Name {
					continue
				}
				runjoblistfunc(job, cfgp, int8(idxlist))
			}
		}
	} else {
		runjoblistfunc(job, cfgp, -1)
	}
	logjob("Ended Job", cfgpstr, listname, job)

	if dbinsert != 0 {
		database.Exec1(database.QueryUpdateHistory, &dbinsert)
	}
}

func logjob(act string, cfgp string, listname string, job string) {
	logger.LogDynamicany("info", act, &logger.StrConfig, cfgp, &logger.StrJob, job, &logger.StrListname, listname, "Num Goroutines", runtime.NumGoroutine()) //logpointerr
}

// Refreshcache refreshes various database caches used for performance.
// It refreshes the history cache, media cache, media titles cache,
// unmatched cache, and files cache.
// The useseries parameter determines if it should refresh for series
// or movies.
func Refreshcache(useseries bool) {
	database.Refreshhistorycache(useseries)
	database.RefreshMediaCache(useseries)
	database.RefreshMediaCacheTitles(useseries)
	database.Refreshunmatchedcached(useseries)
	database.Refreshfilescached(useseries)
}

// runjoblistfunc executes the specified job for the given media config and list index.
// It handles running various maintenance jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the job string and dispatch
// to the appropriate internal functions. List index of -1 runs the job for all lists in the config.
func runjoblistfunc(job string, cfgp *config.MediaTypeConfig, listid int8) {
	if job == "" || cfgp == nil {
		return
	}
	switch job {
	case logger.StrData, logger.StrDataFull:
		for idxi := range cfgp.Data {
			newfilesloop(cfgp, &cfgp.Data[idxi])
		}
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
		structurefolders(cfgp)
	case logger.StrRssSeasons:
		searcher.SearchSeriesRSSSeasons(cfgp)
	case logger.StrRssSeasonsAll:
		searcher.SearchSeriesRSSSeasonsAll(cfgp)
	case "refreshinc":
		if !cfgp.Useseries {
			refreshMoviesInc(cfgp)
		} else {
			refreshSeriesInc(cfgp)
		}
	case "refresh":
		if !cfgp.Useseries {
			refreshMovies(cfgp)
		} else {
			refreshSeries(cfgp)
		}
	case logger.StrClearHistory:
		if !cfgp.Useseries {
			database.ExecN("delete from movie_histories where movie_id in (Select id from movies where listname = ? COLLATE NOCASE)", &cfgp.Lists[listid].Name)
		} else {
			database.ExecN("delete from serie_episode_histories where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", &cfgp.Lists[listid].Name)
		}
	case logger.StrFeeds:
		if cfgp.Lists[listid].CfgList == nil {
			logger.LogDynamicany("error", "import feeds failed - no cfgp list", &logger.StrListname, &cfgp.Lists[listid].Name)
			break
		}
		var err error
		if !cfgp.Useseries {
			err = importnewmoviessingle(cfgp, &cfgp.Lists[listid], listid)
		} else {
			err = importnewseriessingle(cfgp, &cfgp.Lists[listid], listid)
		}
		if err != nil && err != logger.ErrDisabled {
			logger.LogDynamicany("error", "import feeds failed", err, &logger.StrListname, &cfgp.Lists[listid].Name)
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
		if strings.HasSuffix(job, "inc") {
			searchinterval = cfgp.SearchmissingIncremental
			if searchinterval == 0 {
				searchinterval = 20
			}
		}
		jobsearchmedia(cfgp, searchmissing, searchtitle, searchinterval)
	default:
		logger.LogDynamicany("error", "Switch not found", &logger.StrJob, job) //logpointerr
	}
}

// jobsearchmedia searches for media items that need to be searched for new releases
// or missing files based on the provided config, search type, and interval.
// It builds a database query, executes it to get a list of media IDs,
// then calls MediaSearch on each ID to perform the actual search.
func jobsearchmedia(cfgp *config.MediaTypeConfig, searchmissing, searchtitle bool, searchinterval uint16) {
	var scaninterval uint8
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
	//args := make([]any, 0, len(cfgp.Lists)+2)
	args := logger.PLArrAny.Get()
	for idx := range cfgp.Lists {
		args.Arr = append(args.Arr, &cfgp.Lists[idx].Name)
	}

	bld := logger.PlAddBuffer.Get()

	bld.WriteStringMap(cfgp.Useseries, logger.SearchGenTable)
	if searchmissing {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenMissing)
		bld.WriteString(cfgp.ListsQu)
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenMissingEnd)
	} else {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenReached)
		bld.WriteString(cfgp.ListsQu)
		bld.WriteRune(')')
	}
	var timeinterval time.Time
	if scaninterval != 0 {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenLastScan)
		timeinterval = logger.TimeGetNow().AddDate(0, 0, 0-int(scaninterval))
		args.Arr = append(args.Arr, &timeinterval)
	}
	var timedatepre time.Time
	if scandatepre != 0 {
		bld.WriteStringMap(cfgp.Useseries, logger.SearchGenDate)
		timedatepre = logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)
		args.Arr = append(args.Arr, &timedatepre)
	}
	bld.WriteStringMap(cfgp.Useseries, logger.SearchGenOrder)
	if searchinterval != 0 {
		bld.WriteString(" limit ")
		bld.WriteUInt16(searchinterval)
	}

	str := bld.String()
	tbl := database.GetrowsNuncached[database.DbstaticOneStringOneUInt](database.GetdatarowNArg[uint](false, logger.JoinStrings("select count() ", str), args.Arr), logger.JoinStrings(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenSelect), str), args.Arr)
	logger.PlAddBuffer.Put(bld)
	logger.PLArrAny.Put(args)
	if len(tbl) == 0 {
		return
	}

	for idx := range tbl {
		searcher.NewSearcher(cfgp, cfgp.GetMediaQualityConfigStr(tbl[idx].Str), "", 0).MediaSearch(cfgp, &tbl[idx].Num, searchtitle, true, true)
	}
	//clear(tbl)
}

// checkmissing checks for missing files for the given media list.
// It queries for file locations, checks if they exist, and updates
// the database to set missing flags on media items with no files.
// It handles both movies and series based on the useseries flag.
func checkmissing(useseries bool, listcfg *config.MediaListsConfig) {
	arr := database.Getrows1size[string](false, logger.GetStringsMap(useseries, logger.DBCountFilesByList), logger.GetStringsMap(useseries, logger.DBLocationFilesByList), &listcfg.Name)
	for idx := range arr {
		if scanner.CheckFileExist(arr[idx]) {
			continue
		}
		checkmissingfiles(useseries, &arr[idx])
	}
	//clear(arr)
}

// checkmissingfiles checks for missing files for a given media item.
// It deletes the file record if missing, and updates the missing flag on the media item if it has no more files.
// It takes the query to count files for the media item, the table to delete from,
// the table to update the missing flag, the query to get the file ID and media item ID,
// and the file location that was found missing.
func checkmissingfiles(useseries bool, row *string) {
	subquerycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)
	table := logger.GetStringsMap(useseries, logger.TableFiles)
	updatetable := logger.GetStringsMap(useseries, logger.TableMedia)
	var counter int
	arr := database.Getrows1size[database.DbstaticTwoUint](false, logger.GetStringsMap(useseries, logger.DBCountFilesByLocation), logger.GetStringsMap(useseries, logger.DBIDsFilesByLocation), row)
	for idx := range arr {
		logger.LogDynamicany("info", "File was removed", &logger.StrFile, row)
		_, err := database.Exec1(logger.JoinStrings("delete from ", table, " where id = ?"), &arr[idx].Num1)
		if err == nil {
			_ = database.Scanrows1dyn(false, subquerycount, &counter, &arr[idx].Num2)
			if counter == 0 {
				database.Exec1(logger.JoinStrings("update ", updatetable, " set missing = 1 where id = ?"), &arr[idx].Num2)
			}
		}
	}
	//clear(arr)
}

// checkmissingflag checks for missing files for the given media list.
// It updates the missing flag in the database based on file count.
func checkmissingflag(useseries bool, listcfg *config.MediaListsConfig) {
	queryupdate := logger.GetStringsMap(useseries, logger.DBUpdateMissing)
	querycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)

	var counter int

	var v0 uint8 = 0
	var v1 uint8 = 1
	arr := database.Getrows1size[database.DbstaticOneIntOneBool](false, logger.GetStringsMap(useseries, logger.DBCountMediaByList), logger.GetStringsMap(useseries, logger.DBIDMissingMediaByList), &listcfg.Name)
	for idx := range arr {
		_ = database.Scanrows1dyn(false, querycount, &counter, &arr[idx].Num)
		if counter >= 1 && arr[idx].Bl {
			database.ExecN(queryupdate, &v0, &arr[idx].Num)
		}
		if counter == 0 && !arr[idx].Bl {
			database.ExecN(queryupdate, &v1, &arr[idx].Num)
		}
	}
	//clear(arr)
}

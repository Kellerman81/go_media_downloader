package utils

import (
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

const blimit = " limit "

// insertjobhistory inserts a new record into the job_histories table to track when a job starts.
// It takes the job type, media config, and current time as parameters.
// It returns the auto-generated id for the inserted row.
func insertjobhistory(jobtype string, cfgp *config.MediaTypeConfig) int64 {
	jobcategory := logger.StrSeries
	if !cfgp.Useseries {
		jobcategory = logger.StrMovie
	}
	result, err := database.ExecNid("Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, datetime('now','localtime'))", &jobtype, &cfgp.Name, &jobcategory)
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
	logger.LogDynamic("info", "Starting initial DB fill for series")

	database.Refreshunmatchedcached(true)
	database.Refreshfilescached(true)

	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := 0; idx2 < len(media.Lists); idx2++ {
			err := importnewseriessingle(media, &media.Lists[idx2], idx2)

			if err != nil {
				logger.LogDynamic("error", "Import new series failed", logger.NewLogFieldValue(err))
			}
		}
		database.ExecN(database.QueryUpdateHistory, &dbid)
	}
	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrDataFull, media)
		newFilesMap(media)
		database.ExecN(database.QueryUpdateHistory, &dbid)
	}

	database.ClearCaches()
}

// InitialFillMovies performs the initial database fill for movies.
// It refreshes the unmatched and files caches, inserts job history records,
// calls importnewmoviessingle to import new movies from the configured lists,
// calls newFilesMap to scan for new files, and clears caches when done.
func InitialFillMovies() {
	logger.LogDynamic("info", "Starting initial DB fill for movies")

	database.Refreshunmatchedcached(false)
	database.Refreshfilescached(false)
	for _, media := range config.SettingsMedia {
		if media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := 0; idx2 < len(media.Lists); idx2++ {
			err := importnewmoviessingle(media, &media.Lists[idx2], idx2)
			if err != nil {
				logger.LogDynamic("error", "Import new movies failed", logger.NewLogFieldValue(err))
			}
		}
		database.ExecN(database.QueryUpdateHistory, &dbid)
	}

	for _, media := range config.SettingsMedia {
		if media.Useseries {
			continue
		}
		dbid := insertjobhistory(logger.StrDataFull, media)
		newFilesMap(media)
		database.ExecN(database.QueryUpdateHistory, &dbid)
	}

	database.ClearCaches()
}

// getImdbFilename returns the path to the init_imdb executable
// based on the current OS. For Windows it returns init_imdb.exe,
// for other OSes it returns ./init_imdb.
func getImdbFilename() string {
	if runtime.GOOS == "windows" {
		return "init_imdb.exe"
	}
	return "./init_imdb"
}

// FillImdb refreshes the IMDB database by calling the init_imdb executable.
// It inserts a record into the job history table, executes the IMDB update,
// reloads the IMDB database, and updates the job history record when done.
func FillImdb() {
	dbinsert, _ := database.ExecNid("Insert into job_histories (job_type, job_group, job_category, started) values (?, 'RefreshImdb', ?, datetime('now','localtime'))", &logger.StrData, &logger.StrMovie)
	out := parser.ExecCmd(getImdbFilename(), "", logger.StrImdb)
	defer out.Close()
	if out.Err == nil {
		logger.LogDynamic("info", "imdb response", logger.NewLogField("repsonse", string(out.Out)))
		database.ExchangeImdbDB()
	}
	if dbinsert != 0 {
		database.ExecN(database.QueryUpdateHistory, &dbinsert)
	}
}

// getNewFilesMap walks through the MediaDataConfig for the given
// MediaTypeConfig, and calls getNewFilesloop on each one to scan for new files.
func newFilesMap(cfgp *config.MediaTypeConfig) {
	for idxi := range cfgp.Data {
		newfilesloop(cfgp, &cfgp.Data[idxi])
	}
}

func newfilesloop(cfgp *config.MediaTypeConfig, data *config.MediaDataConfig) {
	if data.CfgPath == nil {
		logger.LogDynamic("error", "config not found", logger.NewLogField("template", &data.TemplatePath))
		return
	}
	//newFilesloop(cfgp, &cfgp.Data[idxi])
	workergroup := worker.WorkerPoolParse.Group()
	filedata := scanner.NewFileData{Cfgp: cfgp, PathCfg: data.CfgPath, Listid: config.GetMediaListsEntryListID(cfgp, data.AddFoundList), Checkfiles: true, AddFound: data.AddFound}
	_ = filepath.WalkDir(data.CfgPath.Path, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() || scanner.Filterfiles(&fpath, &filedata) {
			return nil
		}
		workergroup.Submit(func() {
			if filedata.Cfgp == nil {
				logger.LogDynamic("error", "parse failed cfgp", logger.NewLogFieldValue(logger.ErrCfgpNotFound), logger.NewLogField(logger.StrFile, &fpath))
				return
			}
			m := parser.ParseFile(fpath, true, true, filedata.Cfgp, -1)
			if m == nil {
				return
			}
			defer apiexternal.ParserPool.Put(m)

			err := parser.GetDBIDs(m)
			if err != nil {
				logger.LogDynamic("error", "parse failed ids", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, &fpath))
				return
			}

			if m.M.ListID != -1 && filedata.Listid == -1 {
				filedata.Listid = m.M.ListID
			}

			if filedata.Cfgp.Useseries && m.M.SerieID != 0 && m.M.ListID == -1 && filedata.Listid == -1 {
				filedata.Listid = database.GetMediaListIDGetListname(filedata.Cfgp, m.M.SerieID)
				m.M.ListID = filedata.Listid
			}
			if !filedata.Cfgp.Useseries && m.M.MovieID != 0 && m.M.ListID == -1 && filedata.Listid == -1 {
				filedata.Listid = database.GetMediaListIDGetListname(filedata.Cfgp, m.M.MovieID)
				m.M.ListID = filedata.Listid
			}
			if filedata.Listid != -1 {
				if filedata.Cfgp.Useseries {
					err = jobImportSeriesParseV2(m, fpath, true, filedata.Cfgp, filedata.Listid, &m.Cfgp.Lists[filedata.Listid])
				} else {
					err = jobImportMovieParseV2(m, fpath, true, filedata.Cfgp, filedata.Listid, &m.Cfgp.Lists[filedata.Listid], filedata.AddFound)
				}

				if err != nil {
					logger.LogDynamic("error", "Error Importing", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, &fpath))
				}
			}
		})
		return nil
	})
	workergroup.Wait()
}

// SingleJobs runs a single maintenance job for the given media config and list.
// It handles running jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the
// job string and dispatched to internal functions. List can be empty to run for all lists.
func SingleJobs(job, cfgpstr, listname string, force bool) {
	if job == "" || cfgpstr == "" {
		logger.LogDynamic("info", "skipped job", logger.NewLogField(logger.StrJob, &job), logger.NewLogField("cfgp", &cfgpstr), logger.NewLogField(logger.StrListname, &listname))
		return
	}
	if config.SettingsGeneral.SchedulerDisabled && !force {
		logger.LogDynamic("debug", "skipped job", logger.NewLogField(logger.StrJob, &job))
		return
	}
	if !config.SettingsMedia[cfgpstr].Useseries && (job == logger.StrRssSeasons || job == logger.StrRssSeasonsAll) {
		return
	}
	logger.LogDynamic("info", jobstarted, logger.NewLogField("cfg", &cfgpstr), logger.NewLogField(logger.StrJob, &job), logger.NewLogField("list", &listname), logger.NewLogField("Num Goroutines", runtime.NumGoroutine()))

	cfgp := config.SettingsMedia[cfgpstr]
	if cfgpstr != "" && cfgp == nil {
		config.LoadCfgDB()
		cfgp = config.SettingsMedia[cfgpstr]
		if cfgp == nil {
			logger.LogDynamic("error", "config not found", logger.NewLogField("cfg", &cfgpstr))
			return
		}
	}
	Refreshcache(cfgp.Useseries)
	dbinsert := insertjobhistory(job, cfgp)

	if job == logger.StrData || job == logger.StrRss || job == logger.StrReachedFlag || job == logger.StrClearHistory || job == logger.StrFeeds || job == logger.StrCheckMissing || job == logger.StrCheckMissingFlag {
		if job == logger.StrRss {
			for idx := range cfgp.ListsQualities {
				searcher.NewSearcher(cfgp, config.SettingsQuality[cfgp.ListsQualities[idx]], logger.StrRss, &logger.V0).SearchRSS(cfgp, config.SettingsQuality[cfgp.ListsQualities[idx]], false, true, true)
			}
		} else {
			for idxlist := range cfgp.Lists {
				if listname != "" && listname != cfgp.Lists[idxlist].Name {
					continue
				}
				runjoblistfunc(job, cfgp, idxlist)
			}
		}
	} else {
		runjoblistfunc(job, cfgp, -1)
	}
	logger.LogDynamic("info", jobended, logger.NewLogField("cfg", &cfgpstr), logger.NewLogField(logger.StrJob, &job), logger.NewLogField("list", &listname))

	if dbinsert != 0 {
		database.ExecN(database.QueryUpdateHistory, &dbinsert)
	}
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
func runjoblistfunc(job string, cfgp *config.MediaTypeConfig, listid int) {
	if job == "" || cfgp == nil {
		return
	}
	switch job {
	case logger.StrData, logger.StrDataFull:
		newFilesMap(cfgp)
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
			logger.LogDynamic("error", "import feeds failed - no cfgp list", logger.NewLogField("Listname", &cfgp.Lists[listid].Name))
			break
		}
		var err error
		if !cfgp.Useseries {
			err = importnewmoviessingle(cfgp, &cfgp.Lists[listid], listid)
		} else {
			err = importnewseriessingle(cfgp, &cfgp.Lists[listid], listid)
		}
		if err != nil {
			logger.LogDynamic("error", "import feeds failed", logger.NewLogFieldValue(err), logger.NewLogField("Listname", &cfgp.Lists[listid].Name))
		}
	case logger.StrRss:
		return
	case logger.StrSearchMissingFull, logger.StrSearchMissingInc, logger.StrSearchUpgradeFull, logger.StrSearchUpgradeInc, logger.StrSearchMissingFullTitle, logger.StrSearchMissingIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeIncTitle:
		var searchinterval int
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
		logger.LogDynamic("error", "Switch not found", logger.NewLogField(logger.StrJob, &job))
	}
}

// jobsearchmedia searches for media items that need to be searched for new releases
// or missing files based on the provided config, search type, and interval.
// It builds a database query, executes it to get a list of media IDs,
// then calls MediaSearch on each ID to perform the actual search.
func jobsearchmedia(cfgp *config.MediaTypeConfig, searchmissing, searchtitle bool, searchinterval int) {
	var scaninterval, scandatepre int
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
	n := cfgp.ListsLen
	if searchinterval != 0 {
		n++
	}
	if scandatepre != 0 {
		n++
	}
	args := make([]any, 0, n)
	for i := range cfgp.Lists {
		args = append(args, &cfgp.Lists[i].Name)
	}

	bld := logger.PlBuffer.Get()
	bld.Grow(750)

	bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenTable))
	if searchmissing {
		bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenMissing))
		bld.WriteString(cfgp.ListsQu)
		bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenMissingEnd))
	} else {
		bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenReached))
		bld.WriteString(cfgp.ListsQu)
		bld.WriteRune(')')
	}
	if scaninterval != 0 {
		bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenLastScan))
		args = append(args, logger.TimeGetNow().AddDate(0, 0, 0-scaninterval))
	}
	if scandatepre != 0 {
		bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenDate))
		args = append(args, logger.TimeGetNow().AddDate(0, 0, 0+scandatepre))
	}
	bld.WriteString(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenOrder))
	if searchinterval != 0 {
		bld.WriteString(blimit)
		logger.BuilderAddInt(bld, searchinterval)
	}

	str := bld.String()
	tbl := database.GetrowsNuncached[database.DbstaticOneStringOneUInt](database.GetdatarowN[int](false, logger.JoinStrings("select count() ", str), args...), logger.JoinStrings(logger.GetStringsMap(cfgp.Useseries, logger.SearchGenSelect), str), args)
	logger.PlBuffer.Put(bld)

	//clear(args)
	args = nil
	if len(tbl) == 0 {
		return
	}

	for idx := range tbl {
		searcher.NewSearcher(cfgp, database.GetMediaQualityConfigStr(cfgp, tbl[idx].Str), "", &logger.V0).MediaSearch(cfgp, &tbl[idx].Num, searchtitle, true, true)
	}
	//clear(tbl)
	tbl = nil
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
		checkmissingfiles(useseries, arr[idx])
	}
	//clear(arr)
	arr = nil
}

// checkmissingfiles checks for missing files for a given media item.
// It deletes the file record if missing, and updates the missing flag on the media item if it has no more files.
// It takes the query to count files for the media item, the table to delete from,
// the table to update the missing flag, the query to get the file ID and media item ID,
// and the file location that was found missing.
func checkmissingfiles(useseries bool, row string) {
	subquerycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)
	table := logger.GetStringsMap(useseries, logger.TableFiles)
	updatetable := logger.GetStringsMap(useseries, logger.TableMedia)
	var counter int
	arr := database.Getrows1size[database.DbstaticTwoUint](false, logger.GetStringsMap(useseries, logger.DBCountFilesByLocation), logger.GetStringsMap(useseries, logger.DBIDsFilesByLocation), &row)
	for idx := range arr {
		logger.LogDynamic("info", "File was removed", logger.NewLogField(logger.StrFile, &row))
		_, err := database.ExecN(logger.JoinStrings("delete from ", table, " where id = ?"), &arr[idx].Num1)
		if err == nil {
			_ = database.ScanrowsNdyn(false, subquerycount, &counter, &arr[idx].Num2)
			if counter == 0 {
				database.ExecN(logger.JoinStrings("update ", updatetable, " set missing = 1 where id = ?"), &arr[idx].Num2)
			}
		}
	}
	//clear(arr)
	arr = nil
}

// checkmissingflag checks for missing files for the given media list.
// It updates the missing flag in the database based on file count.
func checkmissingflag(useseries bool, listcfg *config.MediaListsConfig) {
	queryupdate := logger.GetStringsMap(useseries, logger.DBUpdateMissing)
	querycount := logger.GetStringsMap(useseries, logger.DBCountFilesByMediaID)

	var counter int
	v1 := 1
	arr := database.Getrows1size[database.DbstaticOneIntOneBool](false, logger.GetStringsMap(useseries, logger.DBCountMediaByList), logger.GetStringsMap(useseries, logger.DBIDMissingMediaByList), &listcfg.Name)
	for idx := range arr {
		_ = database.ScanrowsNdyn(false, querycount, &counter, &arr[idx].Num)
		if counter >= 1 && arr[idx].Bl {
			database.ExecN(queryupdate, &logger.V0, &arr[idx].Num)
		}
		if counter == 0 && !arr[idx].Bl {
			database.ExecN(queryupdate, &v1, &arr[idx].Num)
		}
	}
	//clear(arr)
	arr = nil
}

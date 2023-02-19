package scheduler

import (
	"math/rand"
	"os"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/tasks"
	"github.com/Kellerman81/go_media_downloader/utils"
)

var QueueFeeds *tasks.Dispatcher
var QueueData *tasks.Dispatcher
var QueueSearch *tasks.Dispatcher

func converttime(interval string) time.Duration {
	if logger.StringContainsRune(interval, 'd') {
		dur, _ := time.ParseDuration(logger.IntToString(logger.StringToInt(logger.StringDeleteRune(interval, 'd', 1))*24) + "h")
		return dur
	}
	dur, _ := time.ParseDuration(interval)
	return dur
}
func convertcron(interval string) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	if logger.StringContainsRune(interval, 'd') {
		return "0 " + logger.IntToString(rand.Intn(60)) + " " + logger.IntToString(rand.Intn(24)) + " */" + logger.StringDeleteRune(interval, 'd', 1) + " * *"
	}
	if logger.StringContainsRune(interval, 'h') {
		return "0 " + logger.IntToString(rand.Intn(60)) + " */" + logger.StringDeleteRune(interval, 'h', 1) + " * * *"
	}
	return "0 */" + logger.StringDeleteRune(interval, 'm', 1) + " * * * *"
}

func getSchCfg(schedulerTemplate string, cfgpstr string, groupvar string, typvar string, name string) (intervalstr string, cronstr string, title string, fun func()) {
	title = groupvar + "_" + typvar + "_" + name
	switch groupvar {
	case "searchmissinginc":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerMissing
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerMissing
	case "searchmissingfull":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerMissingFull
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerMissingFull
	case "searchupgradeinc":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerUpgrade
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerUpgrade
	case "searchupgradefull":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerUpgradeFull
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerUpgradeFull
	case "searchmissinginctitle":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerMissingTitle
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerMissingTitle
	case "searchmissingfulltitle":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerMissingFullTitle
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerMissingFullTitle
	case "searchupgradeinctitle":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerUpgradeTitle
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerUpgradeTitle
	case "searchupgradefulltitle":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerUpgradeFullTitle
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerUpgradeFullTitle
	case "rss":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerRss
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerRss
	case "datafull":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalScanData
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronScanData
	case "structure":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalScanDataimport
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronScanDataimport
	case "feeds":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalFeeds
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronFeeds
	case "checkmissing":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalScanDataMissing
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronScanDataMissing
	case "checkmissingflag":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalScanDataFlags
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronScanDataFlags
	case "checkupgradeflag":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalScanDataFlags
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronScanDataFlags
	case "rssseasons":
		intervalstr = config.Cfg.Scheduler[schedulerTemplate].IntervalIndexerRssSeasons
		cronstr = config.Cfg.Scheduler[schedulerTemplate].CronIndexerRssSeasons
	default:
		return
	}
	fun = func() { utils.SingleJobs(typvar, groupvar, cfgpstr, "", false) }
	return
}

func getGlobalSchCfg(scheduler *config.SchedulerConfig, groupvar string) (intervalstr string, cronstr string, title string, fun func()) {
	switch groupvar {
	case "refreshseriesfull":
		intervalstr = scheduler.IntervalFeedsRefreshSeriesFull
		cronstr = scheduler.CronFeedsRefreshSeriesFull
		title = logger.StrRefreshSeries
		fun = func() {
			utils.RefreshSeries()
		}
	case "refreshseries":
		intervalstr = scheduler.IntervalFeedsRefreshSeries
		cronstr = scheduler.CronFeedsRefreshSeries
		title = logger.StrRefreshSeriesInc
		fun = func() {
			utils.RefreshSeriesInc()
		}
	case "refreshmoviesfull":
		intervalstr = scheduler.IntervalFeedsRefreshMoviesFull
		cronstr = scheduler.CronFeedsRefreshMoviesFull
		title = logger.StrRefreshMovies
		fun = func() {
			utils.RefreshMovies()
		}
	case "refreshmovies":
		intervalstr = scheduler.IntervalFeedsRefreshMovies
		cronstr = scheduler.CronFeedsRefreshMovies
		title = logger.StrRefreshMoviesInc
		fun = func() {
			utils.RefreshMoviesInc()
		}
	case "imdb":
		intervalstr = scheduler.IntervalImdb
		cronstr = scheduler.CronImdb
		title = "Refresh IMDB"
		fun = func() {
			utils.FillImdb()
		}
	case "checkdb":
		intervalstr = scheduler.IntervalDatabaseCheck
		cronstr = scheduler.CronDatabaseCheck
		title = "Check Database"
		fun = func() {
			if database.DbIntegrityCheck() != "ok" {
				os.Exit(100)
			}
		}
	case "backupdb":
		intervalstr = scheduler.IntervalDatabaseBackup
		cronstr = scheduler.CronDatabaseBackup
		title = "Backup Database"
		fun = func() {
			database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.Cfg.General.MaxDatabaseBackups)
		}
	}
	return
}
func InitScheduler() {
	QueueFeeds = tasks.NewDispatcher("Feed", 1, 40)
	QueueFeeds.Start()

	QueueData = tasks.NewDispatcher("Data", 1, 40)
	QueueData.Start()

	QueueSearch = tasks.NewDispatcher("Search", config.Cfg.General.ConcurrentScheduler, 40)
	QueueSearch.Start()

	typevars := []string{"searchmissinginc", "searchmissingfull", "searchupgradeinc", "searchupgradefull", "searchmissinginctitle", "searchmissingfulltitle", "searchupgradeinctitle", "searchupgradefulltitle", "rss", "datafull", "structure", "feeds", "checkmissing", "checkmissingflag", "checkupgradeflag", "rssseasons"}
	var usequeue *tasks.Dispatcher

	for idxconfig := range config.Cfg.Movies {
		if !config.Check("scheduler_" + config.Cfg.Movies[idxconfig].TemplateScheduler) {
			continue
		}
		for idxtyp := range typevars {
			switch typevars[idxtyp] {
			case "datafull", "structure", "checkmissing", "checkmissingflag", "checkupgradeflag":
				usequeue = QueueData
			case "feeds":
				usequeue = QueueFeeds
			default:
				usequeue = QueueSearch
			}
			intervalstr, cronstr, title, fun := getSchCfg(config.Cfg.Movies[idxconfig].TemplateScheduler, config.Cfg.Movies[idxconfig].NamePrefix, typevars[idxtyp], "movie", config.Cfg.Movies[idxconfig].Name)
			if intervalstr != "" {
				if config.Cfg.General.UseCronInsteadOfInterval {
					usequeue.DispatchCron(title, fun, convertcron(intervalstr))
				} else {
					usequeue.DispatchEvery(title, fun, converttime(intervalstr))
				}
			}
			if cronstr != "" {
				usequeue.DispatchCron(title, fun, cronstr)
			}
		}
	}

	for idxconfig := range config.Cfg.Series {

		if !config.Check("scheduler_" + config.Cfg.Series[idxconfig].TemplateScheduler) {
			continue
		}

		for idxtyp := range typevars {
			switch typevars[idxtyp] {
			case "datafull", "structure", "checkmissing", "checkmissingflag", "checkupgradeflag":
				usequeue = QueueData
			case "feeds":
				usequeue = QueueFeeds
			default:
				usequeue = QueueSearch
			}
			intervalstr, cronstr, title, fun := getSchCfg(config.Cfg.Series[idxconfig].TemplateScheduler, config.Cfg.Series[idxconfig].NamePrefix, typevars[idxtyp], "series", config.Cfg.Series[idxconfig].Name)
			if intervalstr != "" {
				if config.Cfg.General.UseCronInsteadOfInterval {
					usequeue.DispatchCron(title, fun, convertcron(intervalstr))
				} else {
					usequeue.DispatchEvery(title, fun, converttime(intervalstr))
				}
			}
			if cronstr != "" {
				usequeue.DispatchCron(title, fun, cronstr)
			}
		}
	}

	var defaultschedule config.SchedulerConfig
	if !config.Check("scheduler_Default") {
		defaultschedule = config.SchedulerConfig{
			Name:                       "Default",
			IntervalImdb:               "3d",
			IntervalFeeds:              "1d",
			IntervalFeedsRefreshSeries: "1d",
			IntervalFeedsRefreshMovies: "1d",
			IntervalIndexerMissing:     "40m",
			IntervalIndexerUpgrade:     "60m",
			IntervalIndexerRss:         "15m",
			IntervalScanData:           "1h",
			IntervalScanDataMissing:    "1d",
			IntervalScanDataimport:     "60m",
		}
		config.UpdateCfgEntry(config.Conf{Name: "scheduler_Default", Data: defaultschedule})
		config.WriteCfg()
	} else {
		defaultschedule = config.Cfg.Scheduler["Default"]

	}

	typevars = []string{"backupdb", "checkdb", "imdb", "refreshmovies", "refreshmoviesfull", "refreshseries", "refreshseriesfull"}
	for idxtyp := range typevars {
		switch typevars[idxtyp] {
		case "backupdb", "checkdb":
			usequeue = QueueData
		default:
			usequeue = QueueFeeds
		}
		intervalstr, cronstr, title, fun := getGlobalSchCfg(&defaultschedule, typevars[idxtyp])
		if intervalstr != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				usequeue.DispatchCron(title, fun, convertcron(intervalstr))
			} else {
				usequeue.DispatchEvery(title, fun, converttime(intervalstr))
			}
		}
		if cronstr != "" {
			usequeue.DispatchCron(title, fun, cronstr)
		}
	}
}

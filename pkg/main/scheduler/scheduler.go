package scheduler

import (
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/Kellerman81/go_media_downloader/worker"
	"github.com/alitto/pond"
	"github.com/robfig/cron/v3"
)

var (
	Crons  []*cron.Cron
	Ticker []*time.Ticker
)

func converttime(interval string) time.Duration {
	if strings.ContainsRune(interval, 'd') {
		logger.StringDeleteRuneP(&interval, 'd')
		interval = logger.IntToString(logger.StringToInt(interval)*24) + "h"
	}
	dur, _ := time.ParseDuration(interval)
	return dur
}
func convertcron(interval string) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	if strings.ContainsRune(interval, 'd') {
		logger.StringDeleteRuneP(&interval, 'd')
		return "0 " + logger.IntToString(rand.Intn(60)) + " " + logger.IntToString(rand.Intn(24)) + " */" + interval + " * *"
	}
	if strings.ContainsRune(interval, 'h') {
		logger.StringDeleteRuneP(&interval, 'h')
		return "0 " + logger.IntToString(rand.Intn(60)) + " */" + interval + " * * *"
	}
	logger.StringDeleteRuneP(&interval, 'm')
	return "0 */" + interval + " * * * *"
}

func getSchCfg(schedulerTemplate string, cfgpstr string, groupvar string, typvar string, name string, cfg *worker.Job) (intervalstr string, cronstr string) {
	cfg.Name = groupvar + logger.Underscore + typvar + logger.Underscore + name
	switch groupvar {
	case logger.StrSearchMissingInc:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerMissing
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerMissing
	case logger.StrSearchMissingFull:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerMissingFull
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerMissingFull
	case logger.StrSearchUpgradeInc:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerUpgrade
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerUpgrade
	case logger.StrSearchUpgradeFull:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerUpgradeFull
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerUpgradeFull
	case logger.StrSearchMissingIncTitle:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerMissingTitle
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerMissingTitle
	case logger.StrSearchMissingFullTitle:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerMissingFullTitle
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerMissingFullTitle
	case logger.StrSearchUpgradeIncTitle:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerUpgradeTitle
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerUpgradeTitle
	case logger.StrSearchUpgradeFullTitle:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerUpgradeFullTitle
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerUpgradeFullTitle
	case logger.StrRss:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerRss
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerRss
	case logger.StrDataFull:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalScanData
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronScanData
	case logger.StrStructure:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalScanDataimport
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronScanDataimport
	case logger.StrFeeds:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalFeeds
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronFeeds
	case logger.StrCheckMissing:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalScanDataMissing
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronScanDataMissing
	case logger.StrCheckMissingFlag:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalScanDataFlags
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronScanDataFlags
	case logger.StrUpgradeFlag:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalScanDataFlags
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronScanDataFlags
	case logger.StrRssSeasons:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerRssSeasons
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerRssSeasons
	case logger.StrRssSeasonsAll:
		intervalstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].IntervalIndexerRssSeasonsAll
		cronstr = config.SettingsScheduler["scheduler_"+schedulerTemplate].CronIndexerRssSeasonsAll
	default:
		return
	}
	cfg.Run = func() { utils.SingleJobs(typvar, groupvar, cfgpstr, "", false) }
	return
}

func getGlobalSchCfg(groupvar string, cfg *worker.Job) (intervalstr string, cronstr string) {
	switch groupvar {
	case "refreshseriesfull":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalFeedsRefreshSeriesFull
		cronstr = config.SettingsScheduler["scheduler_Default"].CronFeedsRefreshSeriesFull
		cfg.Name = logger.StrRefreshSeries
		cfg.Run = func() {
			utils.SingleJobs(logger.StrSeries, "refresh", logger.StrSeries, "", false)
		}
	case "refreshseries":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalFeedsRefreshSeries
		cronstr = config.SettingsScheduler["scheduler_Default"].CronFeedsRefreshSeries
		cfg.Name = logger.StrRefreshSeriesInc
		cfg.Run = func() {
			utils.SingleJobs(logger.StrSeries, "refreshinc", logger.StrSeries, "", false)
		}
	case "refreshmoviesfull":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalFeedsRefreshMoviesFull
		cronstr = config.SettingsScheduler["scheduler_Default"].CronFeedsRefreshMoviesFull
		cfg.Name = logger.StrRefreshMovies
		cfg.Run = func() {
			utils.SingleJobs(logger.StrMovie, "refresh", logger.StrMovie, "", false)
		}
	case "refreshmovies":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalFeedsRefreshMovies
		cronstr = config.SettingsScheduler["scheduler_Default"].CronFeedsRefreshMovies
		cfg.Name = logger.StrRefreshMoviesInc
		cfg.Run = func() {
			utils.SingleJobs(logger.StrMovie, "refreshinc", logger.StrMovie, "", false)
		}
	case "imdb":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalImdb
		cronstr = config.SettingsScheduler["scheduler_Default"].CronImdb
		cfg.Name = "Refresh IMDB"
		cfg.Run = func() {
			utils.FillImdb()
		}
	case "checkdb":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalDatabaseCheck
		cronstr = config.SettingsScheduler["scheduler_Default"].CronDatabaseCheck
		cfg.Name = "Check Database"
		cfg.Run = func() {
			if database.DBIntegrityCheck() != "ok" {
				os.Exit(100)
			}
		}
	case "backupdb":
		intervalstr = config.SettingsScheduler["scheduler_Default"].IntervalDatabaseBackup
		cronstr = config.SettingsScheduler["scheduler_Default"].CronDatabaseBackup
		cfg.Name = "Backup Database"
		cfg.Run = func() {
			database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.SettingsGeneral.MaxDatabaseBackups)
		}
	}
	return
}

var typevarsglobal = []string{"backupdb", "checkdb", "imdb", "refreshmovies", "refreshmoviesfull", "refreshseries", "refreshseriesfull"}
var typevars = []string{logger.StrSearchMissingInc, logger.StrSearchMissingFull, logger.StrSearchUpgradeInc, logger.StrSearchUpgradeFull, logger.StrSearchMissingIncTitle, logger.StrSearchMissingFullTitle, logger.StrSearchUpgradeIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrRss, logger.StrDataFull, logger.StrStructure, logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrUpgradeFlag, logger.StrRssSeasons, logger.StrRssSeasonsAll}

func InitScheduler() {

	for idx := range config.SettingsMedia {
		if !config.CheckGroup("scheduler_", config.SettingsMedia[idx].TemplateScheduler) {
			continue
		}
		groupname := logger.StrSeries
		if config.SettingsMedia[idx].NamePrefix[:5] == logger.StrMovie {
			groupname = logger.StrMovie
		}
		for idxtyp := range typevars {
			var usequeue *pond.WorkerPool
			var usequeuename string
			switch typevars[idxtyp] {
			case logger.StrDataFull, logger.StrStructure, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrUpgradeFlag:
				usequeue = worker.WorkerPoolFiles
				usequeuename = "Data"
			case logger.StrFeeds:
				usequeue = worker.WorkerPoolMetadata
				usequeuename = "Feeds"
			default:
				usequeue = worker.WorkerPoolSearch
				usequeuename = "Search"
			}
			job := worker.Job{Queue: usequeuename}
			i, c := getSchCfg(config.SettingsMedia[idx].TemplateScheduler, config.SettingsMedia[idx].NamePrefix, typevars[idxtyp], groupname, config.SettingsMedia[idx].Name, &job)

			schedulerdispatch(&job, usequeue, usequeuename, i, c)
		}
	}

	if !config.CheckGroup("scheduler_", "Default") {
		config.UpdateCfgEntry(config.Conf{Name: "scheduler_Default", Data: config.SchedulerConfig{
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
		}})
		config.WriteCfg()
	}

	for idx := range typevarsglobal {
		var usequeue *pond.WorkerPool
		var usequeuename string
		switch typevarsglobal[idx] {
		case "backupdb", "checkdb":
			usequeue = worker.WorkerPoolFiles
			usequeuename = "Data"
		default:
			usequeue = worker.WorkerPoolMetadata
			usequeuename = "Feeds"
		}
		job := worker.Job{Queue: usequeuename}
		i, c := getGlobalSchCfg(typevarsglobal[idx], &job)
		schedulerdispatch(&job, usequeue, usequeuename, i, c)
	}
}

func schedulerdispatch(cfg *worker.Job, usequeue *pond.WorkerPool, usequeuename string, intervalstr string, cronstr string) {
	var err error
	if intervalstr != "" {
		if config.SettingsGeneral.UseCronInsteadOfInterval {
			err = worker.DispatchCron(usequeue, convertcron(intervalstr), cfg)
		} else {
			err = worker.DispatchEvery(usequeue, converttime(intervalstr), cfg)
		}
		if err != nil {
			logger.Logerror(err, "Cron")
		}
	}

	if cronstr != "" {
		err = worker.DispatchCron(usequeue, cronstr, cfg)
		if err != nil {
			logger.Logerror(err, "Cron")
		}
	}
	if cronstr == "" && intervalstr == "" {
		logger.Log.Debug().Str("job", cfg.Name).Msg("cron cfg not found")
	}
}

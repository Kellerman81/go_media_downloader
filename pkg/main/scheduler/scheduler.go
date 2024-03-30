package scheduler

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// InitScheduler is called at startup to initialize the scheduler. This includes checking for the existence of the scheduler and setting up the
func InitScheduler() {
	for _, cfgp := range config.SettingsMedia {
		name := cfgp.Name
		groupnamestr := logger.StrSeries
		if !cfgp.Useseries {
			groupnamestr = logger.StrMovie
		}
		for _, str := range []string{logger.StrSearchMissingInc, logger.StrSearchMissingFull, logger.StrSearchUpgradeInc, logger.StrSearchUpgradeFull, logger.StrSearchMissingIncTitle, logger.StrSearchMissingFullTitle, logger.StrSearchUpgradeIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrRss, logger.StrDataFull, logger.StrStructure, logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrUpgradeFlag, logger.StrRssSeasons, logger.StrRssSeasonsAll} {
			str := str
			var usequeuename string
			var intervalstr, cronstr string
			switch str {
			case logger.StrDataFull, logger.StrStructure, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrUpgradeFlag:
				usequeuename = "Data"
			case logger.StrFeeds:
				usequeuename = "Feeds"
			default:
				usequeuename = "Search"
			}
			jobname := str + logger.Underscore + groupnamestr + logger.Underscore + name
			switch str {
			case logger.StrSearchMissingInc:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerMissing
				cronstr = cfgp.CfgScheduler.CronIndexerMissing
			case logger.StrSearchMissingFull:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerMissingFull
				cronstr = cfgp.CfgScheduler.CronIndexerMissingFull
			case logger.StrSearchUpgradeInc:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerUpgrade
				cronstr = cfgp.CfgScheduler.CronIndexerUpgrade
			case logger.StrSearchUpgradeFull:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerUpgradeFull
				cronstr = cfgp.CfgScheduler.CronIndexerUpgradeFull
			case logger.StrSearchMissingIncTitle:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerMissingTitle
				cronstr = cfgp.CfgScheduler.CronIndexerMissingTitle
			case logger.StrSearchMissingFullTitle:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerMissingFullTitle
				cronstr = cfgp.CfgScheduler.CronIndexerMissingFullTitle
			case logger.StrSearchUpgradeIncTitle:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerUpgradeTitle
				cronstr = cfgp.CfgScheduler.CronIndexerUpgradeTitle
			case logger.StrSearchUpgradeFullTitle:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerUpgradeFullTitle
				cronstr = cfgp.CfgScheduler.CronIndexerUpgradeFullTitle
			case logger.StrRss:
				intervalstr = cfgp.CfgScheduler.IntervalIndexerRss
				cronstr = cfgp.CfgScheduler.CronIndexerRss
			case logger.StrDataFull:
				intervalstr = cfgp.CfgScheduler.IntervalScanData
				cronstr = cfgp.CfgScheduler.CronScanData
			case logger.StrStructure:
				intervalstr = cfgp.CfgScheduler.IntervalScanDataimport
				cronstr = cfgp.CfgScheduler.CronScanDataimport
			case logger.StrFeeds:
				intervalstr = cfgp.CfgScheduler.IntervalFeeds
				cronstr = cfgp.CfgScheduler.CronFeeds
			case logger.StrCheckMissing:
				intervalstr = cfgp.CfgScheduler.IntervalScanDataMissing
				cronstr = cfgp.CfgScheduler.CronScanDataMissing
			case logger.StrCheckMissingFlag:
				intervalstr = cfgp.CfgScheduler.IntervalScanDataFlags
				cronstr = cfgp.CfgScheduler.CronScanDataFlags
			case logger.StrUpgradeFlag:
				intervalstr = cfgp.CfgScheduler.IntervalScanDataFlags
				cronstr = cfgp.CfgScheduler.CronScanDataFlags
			case logger.StrRssSeasons:
				if !cfgp.Useseries {
					intervalstr = cfgp.CfgScheduler.IntervalIndexerRssSeasons
					cronstr = cfgp.CfgScheduler.CronIndexerRssSeasons
				}
			case logger.StrRssSeasonsAll:
				if !cfgp.Useseries {
					intervalstr = cfgp.CfgScheduler.IntervalIndexerRssSeasonsAll
					cronstr = cfgp.CfgScheduler.CronIndexerRssSeasonsAll
				}
			default:
				continue
			}
			if (intervalstr == "" && cronstr == "") || str == "" {
				continue
			}
			cfgpstr := cfgp.NamePrefix

			//i, c := getSchCfg(templateScheduler, cfgpstr, str, groupname, name, &job)

			schedulerdispatch(intervalstr, cronstr, jobname, usequeuename, func() {
				utils.SingleJobs(str, cfgpstr, "", false)
			})
		}
	}

	if !config.CheckGroup("scheduler_", "Default") {
		config.UpdateCfgEntry(config.Conf{Name: "Default", Data: config.SchedulerConfig{
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

	for _, str := range []string{"backupdb", "checkdb", "imdb", "refreshmovies", "refreshmoviesfull", "refreshseries", "refreshseriesfull"} {
		str := str
		var usequeuename, name string
		var intervalstr, cronstr string
		var fn func()
		switch str {
		case "backupdb", "checkdb":
			usequeuename = "Data"
		default:
			usequeuename = "Feeds"
		}
		switch str {
		case "refreshseriesfull":
			cfgpidx := ""
			for idx, cfgp := range config.SettingsMedia {
				if cfgp.Useseries {
					cfgpidx = idx
					break
				}
			}
			if cfgpidx == "" {
				break
			}
			intervalstr = config.SettingsScheduler["Default"].IntervalFeedsRefreshSeriesFull
			cronstr = config.SettingsScheduler["Default"].CronFeedsRefreshSeriesFull
			name = logger.StrRefreshSeries
			cfgpstr := config.SettingsMedia[cfgpidx].NamePrefix
			fn = func() {
				utils.SingleJobs("refresh", cfgpstr, "", false)
			}
		case "refreshseries":
			cfgpidx := ""
			for idx, cfgp := range config.SettingsMedia {
				if cfgp.Useseries {
					cfgpidx = idx
					break
				}
			}
			if cfgpidx == "" {
				break
			}
			intervalstr = config.SettingsScheduler["Default"].IntervalFeedsRefreshSeries
			cronstr = config.SettingsScheduler["Default"].CronFeedsRefreshSeries
			name = logger.StrRefreshSeriesInc
			cfgpstr := config.SettingsMedia[cfgpidx].NamePrefix
			fn = func() {
				utils.SingleJobs("refreshinc", cfgpstr, "", false)
			}
		case "refreshmoviesfull":
			cfgpidx := ""
			for idx, cfgp := range config.SettingsMedia {
				if !cfgp.Useseries {
					cfgpidx = idx
					break
				}
			}
			if cfgpidx == "" {
				break
			}
			intervalstr = config.SettingsScheduler["Default"].IntervalFeedsRefreshMoviesFull
			cronstr = config.SettingsScheduler["Default"].CronFeedsRefreshMoviesFull
			name = logger.StrRefreshMovies
			cfgpstr := config.SettingsMedia[cfgpidx].NamePrefix
			fn = func() {
				utils.SingleJobs("refresh", cfgpstr, "", false)
			}
		case "refreshmovies":
			cfgpidx := ""
			for idx, cfgp := range config.SettingsMedia {
				if !cfgp.Useseries {
					cfgpidx = idx
					break
				}
			}
			if cfgpidx == "" {
				break
			}
			intervalstr = config.SettingsScheduler["Default"].IntervalFeedsRefreshMovies
			cronstr = config.SettingsScheduler["Default"].CronFeedsRefreshMovies
			name = logger.StrRefreshMoviesInc
			cfgpstr := config.SettingsMedia[cfgpidx].NamePrefix
			fn = func() {
				utils.SingleJobs("refreshinc", cfgpstr, "", false)
			}
		case logger.StrImdb:
			intervalstr = config.SettingsScheduler["Default"].IntervalImdb
			cronstr = config.SettingsScheduler["Default"].CronImdb
			name = "Refresh IMDB"
			fn = func() {
				utils.FillImdb()
			}
		case "checkdb":
			intervalstr = config.SettingsScheduler["Default"].IntervalDatabaseCheck
			cronstr = config.SettingsScheduler["Default"].CronDatabaseCheck
			name = "Check Database"
			fn = func() {
				if database.DBIntegrityCheck() != "ok" {
					os.Exit(100)
				}
			}
		case "backupdb":
			intervalstr = config.SettingsScheduler["Default"].IntervalDatabaseBackup
			cronstr = config.SettingsScheduler["Default"].CronDatabaseBackup
			name = "Backup Database"
			fn = func() {
				if config.SettingsGeneral.DatabaseBackupStopTasks {
					worker.StopCronWorker()
					worker.CloseWorkerPools()
				}
				database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.SettingsGeneral.MaxDatabaseBackups)
				if config.SettingsGeneral.DatabaseBackupStopTasks {
					worker.InitWorkerPools(config.SettingsGeneral.WorkerIndexer, config.SettingsGeneral.WorkerParse, config.SettingsGeneral.WorkerSearch, config.SettingsGeneral.WorkerFiles, config.SettingsGeneral.WorkerMetadata)
					worker.StartCronWorker()
				}
			}
		default:
			intervalstr = ""
			cronstr = ""
			continue
		}
		if intervalstr == "" && cronstr == "" {
			continue
		}

		//i, c := getGlobalSchCfg(typevarsglobal[idx], job)
		schedulerdispatch(intervalstr, cronstr, name, usequeuename, fn)
	}
}

// schedulerdispatch dispatches jobs to the worker queues based on the provided interval or cron schedule.
// It handles converting interval durations to cron expressions and dispatching the jobs.
// It also handles any errors from the dispatching.
func schedulerdispatch(intervalstr string, cronstr string, name string, queue string, fn func()) {
	if intervalstr != "" {
		if config.SettingsGeneral.UseCronInsteadOfInterval {
			//worker.AddCronJob(cfg)
			rand.New(rand.NewSource(time.Now().UnixNano()))
			if strings.ContainsRune(intervalstr, 'd') {
				intervalstr = strings.Replace(intervalstr, "d", "", 1)
				cronstr = "0 " + strconv.Itoa(rand.Intn(60)) + " " + strconv.Itoa(rand.Intn(24)) + " */" + intervalstr + " * *"
			} else if strings.ContainsRune(intervalstr, 'h') {
				intervalstr = strings.Replace(intervalstr, "h", "", 1)
				cronstr = "0 " + strconv.Itoa(rand.Intn(60)) + " */" + intervalstr + " * * *"
			} else {
				intervalstr = strings.Replace(intervalstr, "m", "", 1)
				cronstr = "0 */" + intervalstr + " * * * *"
			}
		} else {
			if strings.ContainsRune(intervalstr, 'd') {
				intervalstr = strings.Replace(intervalstr, "d", "", 1)
				intervalstr = strconv.Itoa(logger.StringToInt(intervalstr)*24) + "h"
			}
			dur, _ := time.ParseDuration(intervalstr)
			err := worker.DispatchEvery(dur, name, queue, fn)

			if err != nil {
				logger.LogDynamic("error", "Cron", logger.NewLogFieldValue(err))
			}
		}
	}

	if cronstr != "" {
		//worker.AddCronJob(cfg)
		err := worker.DispatchCron(cronstr, name, queue, fn)
		if err != nil {
			logger.LogDynamic("error", "Cron", logger.NewLogFieldValue(err))
		}
	}
}

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

// InitScheduler is called at startup to initialize the scheduler. This includes checking for the existence of the scheduler and setting up the.
func InitScheduler() {
	config.GetSettingsGeneral().Jobs = map[string]func(uint32){
		"RefreshImdb": func(key uint32) {
			utils.FillImdb()
			worker.RemoveQueueEntry(key)
		},
		"CheckDatabase": func(key uint32) {
			worker.RemoveQueueEntry(key)
			if database.DBIntegrityCheck() != "ok" {
				os.Exit(100)
			}
		},
		"BackupDatabase": func(key uint32) {
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
			database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)
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
		},
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
	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) {
		if cfgp.Useseries {
			cfgp.Jobs = map[string]func(uint32){
				logger.StrSearchMissingInc: func(key uint32) {
					utils.SingleJobs(logger.StrSearchMissingInc, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchMissingFull: func(key uint32) {
					utils.SingleJobs(logger.StrSearchMissingFull, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeInc: func(key uint32) {
					utils.SingleJobs(logger.StrSearchUpgradeInc, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeFull: func(key uint32) {
					utils.SingleJobs(logger.StrSearchUpgradeFull, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchMissingIncTitle: func(key uint32) {
					utils.SingleJobs(
						logger.StrSearchMissingIncTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchMissingFullTitle: func(key uint32) {
					utils.SingleJobs(
						logger.StrSearchMissingFullTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeIncTitle: func(key uint32) {
					utils.SingleJobs(
						logger.StrSearchUpgradeIncTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeFullTitle: func(key uint32) {
					utils.SingleJobs(
						logger.StrSearchUpgradeFullTitle,
						cfgp.NamePrefix,
						"",
						false,
						key,
					)
					worker.RemoveQueueEntry(key)
				},
				logger.StrRss: func(key uint32) {
					utils.SingleJobs(logger.StrRss, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrDataFull: func(key uint32) {
					utils.SingleJobs(logger.StrDataFull, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrStructure: func(key uint32) {
					utils.SingleJobs(logger.StrStructure, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrFeeds: func(key uint32) {
					utils.SingleJobs(logger.StrFeeds, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrCheckMissing: func(key uint32) {
					utils.SingleJobs(logger.StrCheckMissing, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrCheckMissingFlag: func(key uint32) {
					utils.SingleJobs(logger.StrCheckMissingFlag, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrUpgradeFlag: func(key uint32) {
					utils.SingleJobs(logger.StrUpgradeFlag, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrRssSeasons: func(key uint32) {
					utils.SingleJobs(logger.StrRssSeasons, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrRssSeasonsAll: func(key uint32) {
					utils.SingleJobs(logger.StrRssSeasonsAll, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				"refreshseriesfull": func(key uint32) {
					utils.SingleJobs("refresh", cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				"refreshseriesinc": func(key uint32) {
					utils.SingleJobs("refreshinc", cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
			}
		} else {
			cfgp.Jobs = map[string]func(uint32){
				logger.StrSearchMissingInc: func(key uint32) {
					utils.SingleJobs(logger.StrSearchMissingInc, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchMissingFull: func(key uint32) {
					utils.SingleJobs(logger.StrSearchMissingFull, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeInc: func(key uint32) {
					utils.SingleJobs(logger.StrSearchUpgradeInc, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeFull: func(key uint32) {
					utils.SingleJobs(logger.StrSearchUpgradeFull, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchMissingIncTitle: func(key uint32) {
					utils.SingleJobs(logger.StrSearchMissingIncTitle, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchMissingFullTitle: func(key uint32) {
					utils.SingleJobs(logger.StrSearchMissingFullTitle, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeIncTitle: func(key uint32) {
					utils.SingleJobs(logger.StrSearchUpgradeIncTitle, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrSearchUpgradeFullTitle: func(key uint32) {
					utils.SingleJobs(logger.StrSearchUpgradeFullTitle, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrRss: func(key uint32) {
					utils.SingleJobs(logger.StrRss, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrDataFull: func(key uint32) {
					utils.SingleJobs(logger.StrDataFull, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrStructure: func(key uint32) {
					utils.SingleJobs(logger.StrStructure, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrFeeds: func(key uint32) {
					utils.SingleJobs(logger.StrFeeds, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrCheckMissing: func(key uint32) {
					utils.SingleJobs(logger.StrCheckMissing, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrCheckMissingFlag: func(key uint32) {
					utils.SingleJobs(logger.StrCheckMissingFlag, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				logger.StrUpgradeFlag: func(key uint32) {
					utils.SingleJobs(logger.StrUpgradeFlag, cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				"refreshmoviesfull": func(key uint32) {
					utils.SingleJobs("refresh", cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
				"refreshmoviesinc": func(key uint32) {
					utils.SingleJobs("refreshinc", cfgp.NamePrefix, "", false, key)
					worker.RemoveQueueEntry(key)
				},
			}
		}
		name := cfgp.Name
		groupnamestr := logger.StrSeries
		if !cfgp.Useseries {
			groupnamestr = logger.StrMovie
		}
		for _, str := range []string{"refreshseriesfull", "refreshseriesinc", "refreshmoviesfull", "refreshmoviesinc", logger.StrSearchMissingInc, logger.StrSearchMissingFull, logger.StrSearchUpgradeInc, logger.StrSearchUpgradeFull, logger.StrSearchMissingIncTitle, logger.StrSearchMissingFullTitle, logger.StrSearchUpgradeIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrRss, logger.StrDataFull, logger.StrStructure, logger.StrFeeds, logger.StrCheckMissing, logger.StrCheckMissingFlag, logger.StrUpgradeFlag, logger.StrRssSeasons, logger.StrRssSeasonsAll} {
			var usequeuename string
			var intervalstr, cronstr string
			switch str {
			case logger.StrDataFull,
				logger.StrStructure,
				logger.StrCheckMissing,
				logger.StrCheckMissingFlag,
				logger.StrUpgradeFlag:
				usequeuename = "Data"
			case logger.StrFeeds,
				"refreshseriesfull",
				"refreshmoviesfull",
				"refreshseriesinc",
				"refreshmoviesinc":
				usequeuename = "Feeds"
			case logger.StrRss,
				logger.StrRssSeasons,
				logger.StrRssSeasonsAll:
				usequeuename = "RSS"
			default:
				usequeuename = "Search"
			}
			jobname := logger.JoinStrings(
				str,
				logger.Underscore,
				groupnamestr,
				logger.Underscore,
				name,
			)
			switch str {
			case "refreshseriesfull":
				intervalstr = config.GetSettingsScheduler("Default").IntervalFeedsRefreshSeriesFull
				cronstr = config.GetSettingsScheduler("Default").CronFeedsRefreshSeriesFull
			case "refreshseriesinc":
				intervalstr = config.GetSettingsScheduler("Default").IntervalFeedsRefreshSeries
				cronstr = config.GetSettingsScheduler("Default").CronFeedsRefreshSeries
			case "refreshmoviesfull":
				intervalstr = config.GetSettingsScheduler("Default").IntervalFeedsRefreshMoviesFull
				cronstr = config.GetSettingsScheduler("Default").CronFeedsRefreshMoviesFull
			case "refreshmoviesinc":
				intervalstr = config.GetSettingsScheduler("Default").IntervalFeedsRefreshMovies
				cronstr = config.GetSettingsScheduler("Default").CronFeedsRefreshMovies
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

			schedulerdispatch(cfgp.NamePrefix, intervalstr, cronstr, jobname, usequeuename, str)
		}
	})

	for _, str := range []string{"backupdb", "checkdb", "imdb"} {
		var usequeuename, name string
		var intervalstr, cronstr string
		// var fn func(uint32)
		var jobname string
		switch str {
		case "backupdb", "checkdb":
			usequeuename = "Data"
		default:
			usequeuename = "Feeds"
		}
		switch str {
		case logger.StrImdb:
			intervalstr = config.GetSettingsScheduler("Default").IntervalImdb
			cronstr = config.GetSettingsScheduler("Default").CronImdb
			name = "Refresh IMDB"
			jobname = "RefreshImdb"
		case "checkdb":
			intervalstr = config.GetSettingsScheduler("Default").IntervalDatabaseCheck
			cronstr = config.GetSettingsScheduler("Default").CronDatabaseCheck
			name = "Check Database"
			jobname = "CheckDatabase"
		case "backupdb":
			intervalstr = config.GetSettingsScheduler("Default").IntervalDatabaseBackup
			cronstr = config.GetSettingsScheduler("Default").CronDatabaseBackup
			name = "Backup Database"
			jobname = "BackupDatabase"
		default:
			continue
		}
		if intervalstr == "" && cronstr == "" {
			continue
		}

		schedulerdispatch("", intervalstr, cronstr, name, usequeuename, jobname)
	}
}

// schedulerdispatch dispatches jobs to the worker queues based on the provided interval or cron schedule.
// It handles converting interval durations to cron expressions and dispatching the jobs.
// It also handles any errors from the dispatching.
func schedulerdispatch(
	cfgpstr string,
	intervalstr string,
	cronstr string,
	name string,
	queue string,
	jobname string,
) {
	if intervalstr != "" {
		if config.GetSettingsGeneral().UseCronInsteadOfInterval {
			rand.New(rand.NewSource(time.Now().UnixNano()))
			if strings.ContainsRune(intervalstr, 'd') {
				intervalstr = strings.Replace(intervalstr, "d", "", 1)
				cronstr = "0 " + strconv.Itoa(
					rand.Intn(60),
				) + logger.StrSpace + strconv.Itoa(
					rand.Intn(24),
				) + " */" + intervalstr + " * *"
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
			err := worker.DispatchEvery(cfgpstr, dur, name, queue, jobname)
			if err != nil {
				logger.LogDynamicanyErr("error", "Cron", err)
			}
		}
	}

	if cronstr != "" {
		err := worker.DispatchCron(cfgpstr, cronstr, name, queue, jobname)
		if err != nil {
			logger.LogDynamicanyErr("error", "Cron", err)
		}
	}
}

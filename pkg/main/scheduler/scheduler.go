package scheduler

import (
	"math/rand"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/tasks"
	"github.com/Kellerman81/go_media_downloader/utils"
)

var QueueFeeds *tasks.Dispatcher
var QueueData *tasks.Dispatcher
var QueueSearch *tasks.Dispatcher

func converttime(interval string) time.Duration {
	if strings.Contains(interval, "d") {
		dur, _ := time.ParseDuration(logger.IntToString(logger.StringToInt(strings.Replace(interval, "d", "", 1))*24) + "h")
		return dur
	}
	dur, _ := time.ParseDuration(interval)
	return dur
}
func convertcron(interval string) string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	if strings.Contains(interval, "d") {
		return "0 " + logger.IntToString(rand.Intn(60)) + " " + logger.IntToString(rand.Intn(24)) + " */" + strings.Replace(interval, "d", "", 1) + " * *"
	}
	if strings.Contains(interval, "h") {
		return "0 " + logger.IntToString(rand.Intn(60)) + " */" + strings.Replace(interval, "h", "", 1) + " * * *"
	}
	return "0 */" + strings.Replace(interval, "m", "", 1) + " * * * *"
}

func InitScheduler() {
	QueueFeeds = tasks.NewDispatcher("Feed", 1, 40)
	QueueFeeds.Start()

	QueueData = tasks.NewDispatcher("Data", 1, 40)
	QueueData.Start()

	QueueSearch = tasks.NewDispatcher("Search", config.Cfg.General.ConcurrentScheduler, 40)
	QueueSearch.Start()

	for idxconfig := range config.Cfg.Movies {
		if !config.Check("scheduler_" + config.Cfg.Movies[idxconfig].TemplateScheduler) {
			continue
		}
		cfgpstr := config.Cfg.Movies[idxconfig].NamePrefix
		schedule := config.Cfg.Scheduler[config.Cfg.Movies[idxconfig].TemplateScheduler]

		if schedule.IntervalIndexerMissing != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginc_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissinginc", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissing))
			} else {
				QueueSearch.DispatchEvery("searchmissinginc_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissinginc", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissing))
			}
		}
		if schedule.CronIndexerMissing != "" {
			QueueSearch.DispatchCron("searchmissinginc_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchmissinginc", cfgpstr, "", false)
			}, schedule.CronIndexerMissing)
		}
		if schedule.IntervalIndexerMissingFull != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissingfull", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissingFull))
			} else {
				QueueSearch.DispatchEvery("searchmissingfull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissingfull", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissingFull))
			}
		}
		if schedule.CronIndexerMissingFull != "" {
			QueueSearch.DispatchCron("searchmissingfull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchmissingfull", cfgpstr, "", false)
			}, schedule.CronIndexerMissingFull)
		}
		if schedule.IntervalIndexerUpgrade != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinc_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradeinc", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgrade))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinc_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradeinc", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgrade))
			}
		}
		if schedule.CronIndexerUpgrade != "" {
			QueueSearch.DispatchCron("searchupgradeinc_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchupgradeinc", cfgpstr, "", false)
			}, schedule.CronIndexerUpgrade)
		}
		if schedule.IntervalIndexerUpgradeFull != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradefull", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgradeFull))
			} else {
				QueueSearch.DispatchEvery("searchupgradefull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradefull", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgradeFull))
			}
		}
		if schedule.CronIndexerUpgradeFull != "" {
			QueueSearch.DispatchCron("searchupgradefull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchupgradefull", cfgpstr, "", false)
			}, schedule.CronIndexerUpgradeFull)
		}

		if schedule.IntervalIndexerMissingTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginctitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissinginctitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissingTitle))
			} else {
				QueueSearch.DispatchEvery("searchmissinginctitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissinginctitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissingTitle))
			}
		}
		if schedule.CronIndexerMissingTitle != "" {
			QueueSearch.DispatchCron("searchmissinginctitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchmissinginctitle", cfgpstr, "", false)
			}, schedule.CronIndexerMissingTitle)
		}
		if schedule.IntervalIndexerMissingFullTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfulltitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissingfulltitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissingFullTitle))
			} else {
				QueueSearch.DispatchEvery("searchmissingfulltitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchmissingfulltitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissingFullTitle))
			}
		}
		if schedule.CronIndexerMissingFullTitle != "" {
			QueueSearch.DispatchCron("searchmissingfulltitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchmissingfulltitle", cfgpstr, "", false)
			}, schedule.CronIndexerMissingFullTitle)
		}
		if schedule.IntervalIndexerUpgradeTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinctitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradeinctitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgradeTitle))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinctitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradeinctitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgradeTitle))
			}
		}
		if schedule.CronIndexerUpgradeTitle != "" {
			QueueSearch.DispatchCron("searchupgradeinctitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchupgradeinctitle", cfgpstr, "", false)
			}, schedule.CronIndexerUpgradeTitle)
		}
		if schedule.IntervalIndexerUpgradeFullTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefulltitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradefulltitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgradeFullTitle))
			} else {
				QueueSearch.DispatchEvery("searchupgradefulltitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("searchupgradefulltitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgradeFullTitle))
			}
		}
		if schedule.CronIndexerUpgradeFullTitle != "" {
			QueueSearch.DispatchCron("searchupgradefulltitle_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("searchupgradefulltitle", cfgpstr, "", false)
			}, schedule.CronIndexerUpgradeFullTitle)
		}
		if schedule.IntervalIndexerRss != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rss_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("rss", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerRss))
			} else {
				QueueSearch.DispatchEvery("rss_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("rss", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerRss))
			}
		}
		if schedule.CronIndexerRss != "" {
			QueueSearch.DispatchCron("rss_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("rss", cfgpstr, "", false)
			}, schedule.CronIndexerRss)
		}

		if schedule.IntervalScanData != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("datafull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.SingleJobs("movie", "datafull", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanData))
			} else {
				QueueData.DispatchEvery("datafull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.SingleJobs("movie", "datafull", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanData))
			}
		}
		if schedule.CronScanData != "" {
			QueueData.DispatchCron("datafull_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.SingleJobs("movie", "datafull", cfgpstr, "", false)
			}, schedule.CronScanData)
		}
		if schedule.IntervalScanDataimport != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("structure_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.SingleJobs("movie", "structure", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanDataimport))
			} else {
				QueueData.DispatchEvery("structure_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.SingleJobs("movie", "structure", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanDataimport))
			}
		}
		if schedule.CronScanDataimport != "" {
			QueueData.DispatchCron("structure_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.SingleJobs("movie", "structure", cfgpstr, "", false)
			}, schedule.CronScanDataimport)
		}
		if schedule.IntervalFeeds != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueFeeds.DispatchCron("feeds_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("feeds", cfgpstr, "", false)
				}, convertcron(schedule.IntervalFeeds))
			} else {
				QueueFeeds.DispatchEvery("feeds_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("feeds", cfgpstr, "", false)
				}, converttime(schedule.IntervalFeeds))
			}
		}
		if schedule.CronFeeds != "" {
			QueueFeeds.DispatchCron("feeds_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("feeds", cfgpstr, "", false)
			}, schedule.CronFeeds)
		}

		if schedule.IntervalScanDataMissing != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissing_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("checkmissing", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanDataMissing))
			} else {
				QueueData.DispatchEvery("checkmissing_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("checkmissing", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanDataMissing))
			}
		}
		if schedule.CronScanDataMissing != "" {
			QueueData.DispatchCron("checkmissing_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("checkmissing", cfgpstr, "", false)
			}, schedule.CronScanDataMissing)
		}
		if schedule.IntervalScanDataFlags != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissingflag_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("checkupgradeflag", cfgpstr, "", false)
					utils.MoviesSingleJobs("checkmissingflag", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanDataFlags))
			} else {
				QueueData.DispatchEvery("checkmissingflag_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
					utils.MoviesSingleJobs("checkupgradeflag", cfgpstr, "", false)
					utils.MoviesSingleJobs("checkmissingflag", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanDataFlags))
			}
		}
		if schedule.CronScanDataFlags != "" {
			QueueData.DispatchCron("checkmissingflag_movies_"+config.Cfg.Movies[idxconfig].Name, func() {
				utils.MoviesSingleJobs("checkupgradeflag", cfgpstr, "", false)
				utils.MoviesSingleJobs("checkmissingflag", cfgpstr, "", false)
			}, schedule.CronScanDataFlags)
		}
		// schedule.Close()
	}

	for idxconfig := range config.Cfg.Series {

		if !config.Check("scheduler_" + config.Cfg.Series[idxconfig].TemplateScheduler) {
			continue
		}
		cfgpstr := config.Cfg.Series[idxconfig].NamePrefix
		schedule := config.Cfg.Scheduler[config.Cfg.Series[idxconfig].TemplateScheduler]

		if schedule.IntervalIndexerMissing != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginc_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissinginc", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissing))
			} else {
				QueueSearch.DispatchEvery("searchmissinginc_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissinginc", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissing))
			}
		}
		if schedule.CronIndexerMissing != "" {
			QueueSearch.DispatchCron("searchmissinginc_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchmissinginc", cfgpstr, "", false)
			}, schedule.CronIndexerMissing)
		}
		if schedule.IntervalIndexerMissingFull != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfull_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissingfull", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissingFull))
			} else {
				QueueSearch.DispatchEvery("searchmissingfull_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissingfull", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissingFull))
			}
		}
		if schedule.CronIndexerMissingFull != "" {
			QueueSearch.DispatchCron("searchmissingfull_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchmissingfull", cfgpstr, "", false)
			}, schedule.CronIndexerMissingFull)
		}
		if schedule.IntervalIndexerUpgrade != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinc_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradeinc", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgrade))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinc_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradeinc", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgrade))
			}
		}
		if schedule.CronIndexerUpgrade != "" {
			QueueSearch.DispatchCron("searchupgradeinc_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchupgradeinc", cfgpstr, "", false)
			}, schedule.CronIndexerUpgrade)
		}
		if schedule.IntervalIndexerUpgradeFull != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefull_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradefull", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgradeFull))
			} else {
				QueueSearch.DispatchEvery("searchupgradefull_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradefull", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgradeFull))
			}
		}
		if schedule.CronIndexerUpgradeFull != "" {
			QueueSearch.DispatchCron("searchupgradefull_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchupgradefull", cfgpstr, "", false)
			}, schedule.CronIndexerUpgradeFull)
		}

		if schedule.IntervalIndexerMissingTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginctitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissinginctitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissingTitle))
			} else {
				QueueSearch.DispatchEvery("searchmissinginctitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissinginctitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissingTitle))
			}
		}
		if schedule.CronIndexerMissingTitle != "" {
			QueueSearch.DispatchCron("searchmissinginctitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchmissinginctitle", cfgpstr, "", false)
			}, schedule.CronIndexerMissingTitle)
		}
		if schedule.IntervalIndexerMissingFullTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfulltitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissingfulltitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerMissingFullTitle))
			} else {
				QueueSearch.DispatchEvery("searchmissingfulltitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchmissingfulltitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerMissingFullTitle))
			}
		}
		if schedule.CronIndexerMissingFullTitle != "" {
			QueueSearch.DispatchCron("searchmissingfulltitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchmissingfulltitle", cfgpstr, "", false)
			}, schedule.CronIndexerMissingFullTitle)
		}
		if schedule.IntervalIndexerUpgradeTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinctitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradeinctitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgradeTitle))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinctitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradeinctitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgradeTitle))
			}
		}
		if schedule.CronIndexerUpgradeTitle != "" {
			QueueSearch.DispatchCron("searchupgradeinctitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchupgradeinctitle", cfgpstr, "", false)
			}, schedule.CronIndexerUpgradeTitle)
		}
		if schedule.IntervalIndexerUpgradeFullTitle != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefulltitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradefulltitle", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerUpgradeFullTitle))
			} else {
				QueueSearch.DispatchEvery("searchupgradefulltitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("searchupgradefulltitle", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerUpgradeFullTitle))
			}
		}
		if schedule.CronIndexerUpgradeFullTitle != "" {
			QueueSearch.DispatchCron("searchupgradefulltitle_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("searchupgradefulltitle", cfgpstr, "", false)
			}, schedule.CronIndexerUpgradeFullTitle)
		}

		if schedule.IntervalIndexerRssSeasons != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rssseasons_series_"+config.Cfg.Series[idxconfig].Name, func() {
					searcher.SearchSeriesRSSSeasons(cfgpstr)
				}, convertcron(schedule.IntervalIndexerRssSeasons))
			} else {
				QueueSearch.DispatchEvery("rssseasons_series_"+config.Cfg.Series[idxconfig].Name, func() {
					searcher.SearchSeriesRSSSeasons(cfgpstr)
				}, converttime(schedule.IntervalIndexerRssSeasons))
			}
		}
		if schedule.CronIndexerRssSeasons != "" {
			QueueSearch.DispatchCron("rssseasons_series_"+config.Cfg.Series[idxconfig].Name, func() {
				searcher.SearchSeriesRSSSeasons(cfgpstr)
			}, schedule.CronIndexerRssSeasons)
		}

		if schedule.IntervalIndexerRss != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rss_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("rss", cfgpstr, "", false)
				}, convertcron(schedule.IntervalIndexerRss))
			} else {
				QueueSearch.DispatchEvery("rss_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("rss", cfgpstr, "", false)
				}, converttime(schedule.IntervalIndexerRss))
			}
		}
		if schedule.CronIndexerRss != "" {
			QueueSearch.DispatchCron("rss_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("rss", cfgpstr, "", false)
			}, schedule.CronIndexerRss)
		}

		if schedule.IntervalScanData != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("datafull_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SingleJobs("series", "datafull", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanData))
			} else {
				QueueData.DispatchEvery("datafull_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SingleJobs("series", "datafull", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanData))
			}
		}
		if schedule.CronScanData != "" {
			QueueData.DispatchCron("datafull_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SingleJobs("series", "datafull", cfgpstr, "", false)
			}, schedule.CronScanData)
		}
		if schedule.IntervalScanDataimport != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("structure_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SingleJobs("series", "structure", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanDataimport))
			} else {
				QueueData.DispatchEvery("structure_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SingleJobs("series", "structure", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanDataimport))
			}
		}
		if schedule.CronScanDataimport != "" {
			QueueData.DispatchCron("structure_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SingleJobs("series", "structure", cfgpstr, "", false)
			}, schedule.CronScanDataimport)
		}
		if schedule.IntervalFeeds != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueFeeds.DispatchCron("feeds_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("feeds", cfgpstr, "", false)
				}, convertcron(schedule.IntervalFeeds))
			} else {
				QueueFeeds.DispatchEvery("feeds_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("feeds", cfgpstr, "", false)
				}, converttime(schedule.IntervalFeeds))
			}
		}
		if schedule.CronFeeds != "" {
			QueueFeeds.DispatchCron("feeds_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("feeds", cfgpstr, "", false)
			}, schedule.CronFeeds)
		}

		if schedule.IntervalScanDataMissing != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissing_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("checkmissing", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanDataMissing))
			} else {
				QueueData.DispatchEvery("checkmissing_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("checkmissing", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanDataMissing))
			}
		}
		if schedule.CronScanDataMissing != "" {
			QueueData.DispatchCron("checkmissing_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("checkmissing", cfgpstr, "", false)
			}, schedule.CronScanDataMissing)
		}
		if schedule.IntervalScanDataFlags != "" {
			if config.Cfg.General.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissingflag_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("checkupgradeflag", cfgpstr, "", false)
					utils.SeriesSingleJobs("checkmissingflag", cfgpstr, "", false)
				}, convertcron(schedule.IntervalScanDataFlags))
			} else {
				QueueData.DispatchEvery("checkmissingflag_series_"+config.Cfg.Series[idxconfig].Name, func() {
					utils.SeriesSingleJobs("checkupgradeflag", cfgpstr, "", false)
					utils.SeriesSingleJobs("checkmissingflag", cfgpstr, "", false)
				}, converttime(schedule.IntervalScanDataFlags))
			}
		}
		if schedule.CronScanDataFlags != "" {
			QueueData.DispatchCron("checkmissingflag_series_"+config.Cfg.Series[idxconfig].Name, func() {
				utils.SeriesSingleJobs("checkupgradeflag", cfgpstr, "", false)
				utils.SeriesSingleJobs("checkmissingflag", cfgpstr, "", false)
			}, schedule.CronScanDataFlags)
		}
		// schedule.Close()
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
	if defaultschedule.IntervalFeedsRefreshSeriesFull != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron(logger.StrRefreshSeries, func() {
				utils.RefreshSeries()
			}, convertcron(defaultschedule.IntervalFeedsRefreshSeriesFull))
		} else {
			QueueFeeds.DispatchEvery(logger.StrRefreshSeries, func() {
				utils.RefreshSeries()
			}, converttime(defaultschedule.IntervalFeedsRefreshSeriesFull))
		}
	}
	QueueData.DispatchCron("freemem", func() { debug.FreeOSMemory() }, "0 0 */2 * * *")
	if defaultschedule.CronFeedsRefreshSeriesFull != "" {
		QueueFeeds.DispatchCron(logger.StrRefreshSeries, func() {
			utils.RefreshSeries()
		}, defaultschedule.CronFeedsRefreshSeriesFull)
	}
	if defaultschedule.IntervalFeedsRefreshMoviesFull != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron(logger.StrRefreshMovies, func() {
				utils.RefreshMovies()
			}, convertcron(defaultschedule.IntervalFeedsRefreshMoviesFull))
		} else {
			QueueFeeds.DispatchEvery(logger.StrRefreshMovies, func() {
				utils.RefreshMovies()
			}, converttime(defaultschedule.IntervalFeedsRefreshMoviesFull))
		}
	}
	if defaultschedule.CronFeedsRefreshMoviesFull != "" {
		QueueFeeds.DispatchCron(logger.StrRefreshMovies, func() {
			utils.RefreshMovies()
		}, defaultschedule.CronFeedsRefreshMoviesFull)
	}
	if defaultschedule.IntervalFeedsRefreshSeries != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron(logger.StrRefreshSeriesInc, func() {
				utils.RefreshSeriesInc()
			}, convertcron(defaultschedule.IntervalFeedsRefreshSeries))
		} else {
			QueueFeeds.DispatchEvery(logger.StrRefreshSeriesInc, func() {
				utils.RefreshSeriesInc()
			}, converttime(defaultschedule.IntervalFeedsRefreshSeries))
		}
	}
	if defaultschedule.CronFeedsRefreshSeries != "" {
		QueueFeeds.DispatchCron(logger.StrRefreshSeriesInc, func() {
			utils.RefreshSeriesInc()
		}, defaultschedule.CronFeedsRefreshSeries)
	}
	if defaultschedule.IntervalFeedsRefreshMovies != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron(logger.StrRefreshMoviesInc, func() {
				utils.RefreshMoviesInc()
			}, convertcron(defaultschedule.IntervalFeedsRefreshMovies))
		} else {
			QueueFeeds.DispatchEvery(logger.StrRefreshMoviesInc, func() {
				utils.RefreshMoviesInc()
			}, converttime(defaultschedule.IntervalFeedsRefreshMovies))
		}
	}
	if defaultschedule.CronFeedsRefreshMovies != "" {
		QueueFeeds.DispatchCron(logger.StrRefreshMoviesInc, func() {
			utils.RefreshMoviesInc()
		}, defaultschedule.CronFeedsRefreshMovies)
	}
	if defaultschedule.IntervalImdb != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron("Refresh IMDB", func() {
				utils.FillImdb()
			}, convertcron(defaultschedule.IntervalImdb))
		} else {
			QueueFeeds.DispatchEvery("Refresh IMDB", func() {
				utils.FillImdb()
			}, converttime(defaultschedule.IntervalImdb))
		}
	}
	if defaultschedule.CronImdb != "" {
		QueueFeeds.DispatchCron("Refresh IMDB", func() {
			utils.FillImdb()

		}, defaultschedule.CronImdb)
	}

	if defaultschedule.IntervalDatabaseCheck != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueData.DispatchCron("Check Database", func() {
				if database.DbIntegrityCheck() != "ok" {
					os.Exit(100)
				}
			}, convertcron(defaultschedule.IntervalDatabaseCheck))
		} else {
			QueueData.DispatchEvery("Check Database", func() {
				if database.DbIntegrityCheck() != "ok" {
					os.Exit(100)
				}
			}, converttime(defaultschedule.IntervalDatabaseCheck))
		}
	}
	if defaultschedule.CronDatabaseCheck != "" {
		QueueData.DispatchCron("Check Database", func() {
			if database.DbIntegrityCheck() != "ok" {
				os.Exit(100)
			}
		}, defaultschedule.CronDatabaseCheck)
	}
	if defaultschedule.IntervalDatabaseBackup != "" {
		if config.Cfg.General.UseCronInsteadOfInterval {
			QueueData.DispatchCron("Backup Database", func() {
				database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.Cfg.General.MaxDatabaseBackups)
			}, convertcron(defaultschedule.IntervalDatabaseBackup))
		} else {
			QueueData.DispatchEvery("Backup Database", func() {
				database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.Cfg.General.MaxDatabaseBackups)
			}, converttime(defaultschedule.IntervalDatabaseBackup))
		}
	}
	if defaultschedule.CronDatabaseBackup != "" {
		QueueData.DispatchCron("Backup Database", func() {
			database.Backup("./backup/data.db."+database.GetVersion()+"."+time.Now().Format("20060102_150405"), config.Cfg.General.MaxDatabaseBackups)
		}, defaultschedule.CronDatabaseBackup)
	}
	// defaultschedule.Close()
}

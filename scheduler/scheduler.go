package scheduler

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/tasks"
	"github.com/Kellerman81/go_media_downloader/utils"
)

func converttime(interval string) time.Duration {
	if strings.Contains(interval, "d") {
		intvar, _ := strconv.Atoi(strings.Replace(interval, "d", "", 1))
		dur, _ := time.ParseDuration(strconv.Itoa(intvar*24) + "h")
		return dur
	} else {
		dur, _ := time.ParseDuration(interval)
		return dur
	}
}
func convertcron(interval string) string {
	if strings.Contains(interval, "d") {
		h := strconv.Itoa(rand.Intn(24))
		m := strconv.Itoa(rand.Intn(60))
		return "0 " + m + " " + h + " */" + strings.Replace(interval, "d", "", 1) + " * *"
	}
	if strings.Contains(interval, "h") {
		m := strconv.Itoa(rand.Intn(60))
		return "0 " + m + " */" + strings.Replace(interval, "h", "", 1) + " * * *"
	}
	return "0 */" + strings.Replace(interval, "m", "", 1) + " * * * *"
}

var QueueFeeds *tasks.Dispatcher
var QueueData *tasks.Dispatcher
var QueueSearch *tasks.Dispatcher

func InitScheduler() {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	QueueFeeds = tasks.NewDispatcher("Feed", 1, 100)
	QueueFeeds.Start()

	QueueData = tasks.NewDispatcher("Data", 1, 100)
	QueueData.Start()

	QueueSearch = tasks.NewDispatcher("Search", cfg_general.ConcurrentScheduler, 100)
	QueueSearch.Start()

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		configTemplate := *idxmovie
		if !config.ConfigCheck(configTemplate.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)

		if !config.ConfigCheck("scheduler_" + cfg_movie.Template_scheduler) {
			continue
		}
		schedule := config.ConfigGet("scheduler_" + cfg_movie.Template_scheduler).Data.(config.SchedulerConfig)

		if schedule.Interval_indexer_missing != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginc", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing))
			} else {
				QueueSearch.DispatchEvery("searchmissinginc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginc", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing))
			}
		}
		if schedule.Cron_indexer_missing != "" {
			QueueSearch.DispatchCron("searchmissinginc_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissinginc", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing)
		}
		if schedule.Interval_indexer_missing_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfull", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full))
			} else {
				QueueSearch.DispatchEvery("searchmissingfull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfull", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full))
			}
		}
		if schedule.Cron_indexer_missing_full != "" {
			QueueSearch.DispatchCron("searchmissingfull_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissingfull", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing_full)
		}
		if schedule.Interval_indexer_upgrade != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinc", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinc", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade))
			}
		}
		if schedule.Cron_indexer_upgrade != "" {
			QueueSearch.DispatchCron("searchupgradeinc_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradeinc", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade)
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefull", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full))
			} else {
				QueueSearch.DispatchEvery("searchupgradefull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefull", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full))
			}
		}
		if schedule.Cron_indexer_upgrade_full != "" {
			QueueSearch.DispatchCron("searchupgradefull_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradefull", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full)
		}

		if schedule.Interval_indexer_missing_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginctitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_title))
			} else {
				QueueSearch.DispatchEvery("searchmissinginctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginctitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_title))
			}
		}
		if schedule.Cron_indexer_missing_title != "" {
			QueueSearch.DispatchCron("searchmissinginctitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissinginctitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing_title)
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfulltitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full_title))
			} else {
				QueueSearch.DispatchEvery("searchmissingfulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfulltitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full_title))
			}
		}
		if schedule.Cron_indexer_missing_full_title != "" {
			QueueSearch.DispatchCron("searchmissingfulltitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissingfulltitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing_full_title)
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinctitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinctitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_title))
			}
		}
		if schedule.Cron_indexer_upgrade_title != "" {
			QueueSearch.DispatchCron("searchupgradeinctitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradeinctitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade_title)
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefulltitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradefulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefulltitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full_title))
			}
		}
		if schedule.Cron_indexer_upgrade_full_title != "" {
			QueueSearch.DispatchCron("searchupgradefulltitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradefulltitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full_title)
		}
		if schedule.Interval_indexer_rss != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rss_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("rss", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_rss))
			} else {
				QueueSearch.DispatchEvery("rss_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("rss", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_rss))
			}
		}
		if schedule.Cron_indexer_rss != "" {
			QueueSearch.DispatchCron("rss_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("rss", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_rss)
		}

		if schedule.Interval_scan_data != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("datafull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("datafull", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_data))
			} else {
				QueueData.DispatchEvery("datafull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("datafull", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_data))
			}
		}
		if schedule.Cron_scan_data != "" {
			QueueData.DispatchCron("datafull_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("datafull", configTemplate.Name, "", false)
			}, schedule.Cron_scan_data)
		}
		if schedule.Interval_scan_dataimport != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("structure_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("structure", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_dataimport))
			} else {
				QueueData.DispatchEvery("structure_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("structure", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_dataimport))
			}
		}
		if schedule.Cron_scan_dataimport != "" {
			QueueData.DispatchCron("structure_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("structure", configTemplate.Name, "", false)
			}, schedule.Cron_scan_dataimport)
		}
		if schedule.Interval_feeds != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueFeeds.DispatchCron("feeds_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("feeds", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_feeds))
			} else {
				QueueFeeds.DispatchEvery("feeds_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("feeds", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_feeds))
			}
		}
		if schedule.Cron_feeds != "" {
			QueueFeeds.DispatchCron("feeds_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("feeds", configTemplate.Name, "", false)
			}, schedule.Cron_feeds)
		}

		if schedule.Interval_scan_data_missing != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissing_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("checkmissing", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_data_missing))
			} else {
				QueueData.DispatchEvery("checkmissing_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("checkmissing", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_data_missing))
			}
		}
		if schedule.Cron_scan_data_missing != "" {
			QueueData.DispatchCron("checkmissing_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("checkmissing", configTemplate.Name, "", false)
			}, schedule.Cron_scan_data_missing)
		}
		if schedule.Interval_scan_data_flags != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissingflag_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("checkupgradeflag", configTemplate.Name, "", false)
					utils.Movies_single_jobs("checkmissingflag", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_data_flags))
			} else {
				QueueData.DispatchEvery("checkmissingflag_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("checkupgradeflag", configTemplate.Name, "", false)
					utils.Movies_single_jobs("checkmissingflag", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_data_flags))
			}
		}
		if schedule.Cron_scan_data_flags != "" {
			QueueData.DispatchCron("checkmissingflag_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("checkupgradeflag", configTemplate.Name, "", false)
				utils.Movies_single_jobs("checkmissingflag", configTemplate.Name, "", false)
			}, schedule.Cron_scan_data_flags)
		}
	}

	for _, idxserie := range config.ConfigGetPrefix("serie_") {
		configTemplate := *idxserie
		if !config.ConfigCheck(configTemplate.Name) {
			continue
		}
		if config.ConfigGet(configTemplate.Name).Data == nil {
			continue
		}
		cfg_serie := config.ConfigGet(configTemplate.Name).Data.(config.MediaTypeConfig)

		if !config.ConfigCheck("scheduler_" + cfg_serie.Template_scheduler) {
			continue
		}
		schedule := config.ConfigGet("scheduler_" + cfg_serie.Template_scheduler).Data.(config.SchedulerConfig)

		if schedule.Interval_indexer_missing != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginc", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing))
			} else {
				QueueSearch.DispatchEvery("searchmissinginc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginc", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing))
			}
		}
		if schedule.Cron_indexer_missing != "" {
			QueueSearch.DispatchCron("searchmissinginc_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissinginc", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing)
		}
		if schedule.Interval_indexer_missing_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfull", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full))
			} else {
				QueueSearch.DispatchEvery("searchmissingfull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfull", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full))
			}
		}
		if schedule.Cron_indexer_missing_full != "" {
			QueueSearch.DispatchCron("searchmissingfull_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissingfull", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing_full)
		}
		if schedule.Interval_indexer_upgrade != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinc", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinc", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade))
			}
		}
		if schedule.Cron_indexer_upgrade != "" {
			QueueSearch.DispatchCron("searchupgradeinc_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradeinc", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade)
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefull", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full))
			} else {
				QueueSearch.DispatchEvery("searchupgradefull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefull", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full))
			}
		}
		if schedule.Cron_indexer_upgrade_full != "" {
			QueueSearch.DispatchCron("searchupgradefull_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradefull", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full)
		}

		if schedule.Interval_indexer_missing_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginctitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_title))
			} else {
				QueueSearch.DispatchEvery("searchmissinginctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginctitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_title))
			}
		}
		if schedule.Cron_indexer_missing_title != "" {
			QueueSearch.DispatchCron("searchmissinginctitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissinginctitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing_title)
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfulltitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full_title))
			} else {
				QueueSearch.DispatchEvery("searchmissingfulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfulltitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full_title))
			}
		}
		if schedule.Cron_indexer_missing_full_title != "" {
			QueueSearch.DispatchCron("searchmissingfulltitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissingfulltitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_missing_full_title)
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinctitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinctitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_title))
			}
		}
		if schedule.Cron_indexer_upgrade_title != "" {
			QueueSearch.DispatchCron("searchupgradeinctitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradeinctitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade_title)
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefulltitle", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradefulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefulltitle", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full_title))
			}
		}
		if schedule.Cron_indexer_upgrade_full_title != "" {
			QueueSearch.DispatchCron("searchupgradefulltitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradefulltitle", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full_title)
		}
		if schedule.Interval_indexer_rss != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rss_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("rss", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_indexer_rss))
			} else {
				QueueSearch.DispatchEvery("rss_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("rss", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_indexer_rss))
			}
		}
		if schedule.Cron_indexer_rss != "" {
			QueueSearch.DispatchCron("rss_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("rss", configTemplate.Name, "", false)
			}, schedule.Cron_indexer_rss)
		}

		if schedule.Interval_scan_data != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("datafull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("datafull", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_data))
			} else {
				QueueData.DispatchEvery("datafull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("datafull", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_data))
			}
		}
		if schedule.Cron_scan_data != "" {
			QueueData.DispatchCron("datafull_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("datafull", configTemplate.Name, "", false)
			}, schedule.Cron_scan_data)
		}
		if schedule.Interval_scan_dataimport != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("structure_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("structure", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_dataimport))
			} else {
				QueueData.DispatchEvery("structure_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("structure", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_dataimport))
			}
		}
		if schedule.Cron_scan_dataimport != "" {
			QueueData.DispatchCron("structure_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("structure", configTemplate.Name, "", false)
			}, schedule.Cron_scan_dataimport)
		}
		if schedule.Interval_feeds != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueFeeds.DispatchCron("feeds_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("feeds", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_feeds))
			} else {
				QueueFeeds.DispatchEvery("feeds_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("feeds", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_feeds))
			}
		}
		if schedule.Cron_feeds != "" {
			QueueFeeds.DispatchCron("feeds_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("feeds", configTemplate.Name, "", false)
			}, schedule.Cron_feeds)
		}

		if schedule.Interval_scan_data_missing != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissing_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("checkmissing", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_data_missing))
			} else {
				QueueData.DispatchEvery("checkmissing_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("checkmissing", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_data_missing))
			}
		}
		if schedule.Cron_scan_data_missing != "" {
			QueueData.DispatchCron("checkmissing_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("checkmissing", configTemplate.Name, "", false)
			}, schedule.Cron_scan_data_missing)
		}
		if schedule.Interval_scan_data_flags != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("checkmissingflag_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("checkupgradeflag", configTemplate.Name, "", false)
					utils.Series_single_jobs("checkmissingflag", configTemplate.Name, "", false)
				}, convertcron(schedule.Interval_scan_data_flags))
			} else {
				QueueData.DispatchEvery("checkmissingflag_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("checkupgradeflag", configTemplate.Name, "", false)
					utils.Series_single_jobs("checkmissingflag", configTemplate.Name, "", false)
				}, converttime(schedule.Interval_scan_data_flags))
			}
		}
		if schedule.Cron_scan_data_flags != "" {
			QueueData.DispatchCron("checkmissingflag_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("checkupgradeflag", configTemplate.Name, "", false)
				utils.Series_single_jobs("checkmissingflag", configTemplate.Name, "", false)
			}, schedule.Cron_scan_data_flags)
		}
	}

	var defaultschedule config.SchedulerConfig
	if !config.ConfigCheck("scheduler_Default") {
		defaultschedule = config.SchedulerConfig{
			Name:                          "Default",
			Interval_imdb:                 "3d",
			Interval_feeds:                "1d",
			Interval_feeds_refresh_series: "1d",
			Interval_feeds_refresh_movies: "1d",
			Interval_indexer_missing:      "40m",
			Interval_indexer_upgrade:      "60m",
			Interval_indexer_rss:          "15m",
			Interval_scan_data:            "1h",
			Interval_scan_data_missing:    "1d",
			Interval_scan_dataimport:      "60m",
		}
		config.UpdateCfgEntry(config.Conf{Name: "scheduler_Default", Data: defaultschedule})
		config.WriteCfg()
	} else {
		defaultschedule = config.ConfigGet("scheduler_Default").Data.(config.SchedulerConfig)

	}
	if defaultschedule.Interval_feeds_refresh_series_full != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron("Refresh Series", func() {
				utils.RefreshSeries()
			}, convertcron(defaultschedule.Interval_feeds_refresh_series_full))
		} else {
			QueueFeeds.DispatchEvery("Refresh Series", func() {
				utils.RefreshSeries()
			}, converttime(defaultschedule.Interval_feeds_refresh_series_full))
		}
	}
	if defaultschedule.Cron_feeds_refresh_series_full != "" {
		QueueFeeds.DispatchCron("Refresh Series", func() {
			utils.RefreshSeries()
		}, defaultschedule.Cron_feeds_refresh_series_full)
	}
	if defaultschedule.Interval_feeds_refresh_movies_full != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron("Refresh Movies", func() {
				utils.RefreshMovies()
			}, convertcron(defaultschedule.Interval_feeds_refresh_movies_full))
		} else {
			QueueFeeds.DispatchEvery("Refresh Movies", func() {
				utils.RefreshMovies()
			}, converttime(defaultschedule.Interval_feeds_refresh_movies_full))
		}
	}
	if defaultschedule.Cron_feeds_refresh_movies_full != "" {
		QueueFeeds.DispatchCron("Refresh Movies", func() {
			utils.RefreshMovies()
		}, defaultschedule.Cron_feeds_refresh_movies_full)
	}
	if defaultschedule.Interval_feeds_refresh_series != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron("Refresh Series Incremental", func() {
				utils.RefreshSeriesInc()
			}, convertcron(defaultschedule.Interval_feeds_refresh_series))
		} else {
			QueueFeeds.DispatchEvery("Refresh Series Incremental", func() {
				utils.RefreshSeriesInc()
			}, converttime(defaultschedule.Interval_feeds_refresh_series))
		}
	}
	if defaultschedule.Cron_feeds_refresh_series != "" {
		QueueFeeds.DispatchCron("Refresh Series Incremental", func() {
			utils.RefreshSeriesInc()
		}, defaultschedule.Cron_feeds_refresh_series)
	}
	if defaultschedule.Interval_feeds_refresh_movies != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron("Refresh Movies Incremental", func() {
				utils.RefreshMoviesInc()
			}, convertcron(defaultschedule.Interval_feeds_refresh_movies))
		} else {
			QueueFeeds.DispatchEvery("Refresh Movies Incremental", func() {
				utils.RefreshMoviesInc()
			}, converttime(defaultschedule.Interval_feeds_refresh_movies))
		}
	}
	if defaultschedule.Cron_feeds_refresh_movies != "" {
		QueueFeeds.DispatchCron("Refresh Movies Incremental", func() {
			utils.RefreshMoviesInc()
		}, defaultschedule.Cron_feeds_refresh_movies)
	}
	if defaultschedule.Interval_imdb != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueFeeds.DispatchCron("Refresh IMDB", func() {
				//utils.InitFillImdb()
				utils.FillImdb()
			}, convertcron(defaultschedule.Interval_imdb))
		} else {
			QueueFeeds.DispatchEvery("Refresh IMDB", func() {
				//utils.InitFillImdb()
				utils.FillImdb()
			}, converttime(defaultschedule.Interval_imdb))
		}
	}
	if defaultschedule.Cron_imdb != "" {
		QueueFeeds.DispatchCron("Refresh IMDB", func() {
			//utils.InitFillImdb()
			utils.FillImdb()
		}, defaultschedule.Cron_imdb)
	}

	if defaultschedule.Interval_database_check != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueData.DispatchCron("Check Database", func() {
				str := database.DbIntegrityCheck()
				if str != "ok" {
					os.Exit(100)
				}
			}, convertcron(defaultschedule.Interval_database_check))
		} else {
			QueueData.DispatchEvery("Check Database", func() {
				str := database.DbIntegrityCheck()
				if str != "ok" {
					os.Exit(100)
				}
			}, converttime(defaultschedule.Interval_database_check))
		}
	}
	if defaultschedule.Cron_database_check != "" {
		QueueData.DispatchCron("Check Database", func() {
			str := database.DbIntegrityCheck()
			if str != "ok" {
				os.Exit(100)
			}
		}, defaultschedule.Cron_database_check)
	}
	if defaultschedule.Interval_database_backup != "" {
		if cfg_general.UseCronInsteadOfInterval {
			QueueData.DispatchCron("Backup Database", func() {
				database.Backup(database.DB, fmt.Sprintf("%s.%s.%s", "./backup/data.db", database.DBVersion, time.Now().Format("20060102_150405")), cfg_general.MaxDatabaseBackups)
			}, convertcron(defaultschedule.Interval_database_backup))
		} else {
			QueueData.DispatchEvery("Backup Database", func() {
				database.Backup(database.DB, fmt.Sprintf("%s.%s.%s", "./backup/data.db", database.DBVersion, time.Now().Format("20060102_150405")), cfg_general.MaxDatabaseBackups)
			}, converttime(defaultschedule.Interval_database_backup))
		}
	}
	if defaultschedule.Cron_database_backup != "" {
		QueueData.DispatchCron("Backup Database", func() {
			database.Backup(database.DB, fmt.Sprintf("%s.%s.%s", "./backup/data.db", database.DBVersion, time.Now().Format("20060102_150405")), cfg_general.MaxDatabaseBackups)
		}, defaultschedule.Cron_database_backup)
	}
}

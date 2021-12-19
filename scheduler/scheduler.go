package scheduler

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"runtime"
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
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	QueueFeeds = tasks.NewDispatcher("Feed", 1, 100)
	QueueFeeds.Start()

	QueueData = tasks.NewDispatcher("Data", 1, 100)
	QueueData.Start()

	QueueSearch = tasks.NewDispatcher("Search", cfg_general.ConcurrentScheduler, 100)
	QueueSearch.Start()

	movie_keys, _ := config.ConfigDB.Keys([]byte("movie_*"), 0, 0, true)

	for _, idxmovie := range movie_keys {
		var cfg_movie config.MediaTypeConfig
		config.ConfigGet(string(idxmovie), &cfg_movie)

		if !config.ConfigCheck("scheduler_" + cfg_movie.Template_scheduler) {
			continue
		}
		var schedule config.SchedulerConfig
		config.ConfigGet("scheduler_"+cfg_movie.Template_scheduler, &schedule)

		if schedule.Interval_indexer_missing != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginc", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing))
			} else {
				QueueSearch.DispatchEvery("searchmissinginc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginc", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing))
			}
		}
		if schedule.Cron_indexer_missing != "" {
			QueueSearch.DispatchCron("searchmissinginc_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissinginc", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_missing)
		}
		if schedule.Interval_indexer_missing_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfull", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full))
			} else {
				QueueSearch.DispatchEvery("searchmissingfull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfull", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full))
			}
		}
		if schedule.Cron_indexer_missing_full != "" {
			QueueSearch.DispatchCron("searchmissingfull_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissingfull", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_missing_full)
		}
		if schedule.Interval_indexer_upgrade != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinc", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinc_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinc", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade))
			}
		}
		if schedule.Cron_indexer_upgrade != "" {
			QueueSearch.DispatchCron("searchupgradeinc_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradeinc", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_upgrade)
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefull", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full))
			} else {
				QueueSearch.DispatchEvery("searchupgradefull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefull", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full))
			}
		}
		if schedule.Cron_indexer_upgrade_full != "" {
			QueueSearch.DispatchCron("searchupgradefull_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradefull", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full)
		}

		if schedule.Interval_indexer_missing_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginctitle", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_title))
			} else {
				QueueSearch.DispatchEvery("searchmissinginctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissinginctitle", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_title))
			}
		}
		if schedule.Cron_indexer_missing_title != "" {
			QueueSearch.DispatchCron("searchmissinginctitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissinginctitle", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_missing_title)
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfulltitle", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full_title))
			} else {
				QueueSearch.DispatchEvery("searchmissingfulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchmissingfulltitle", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full_title))
			}
		}
		if schedule.Cron_indexer_missing_full_title != "" {
			QueueSearch.DispatchCron("searchmissingfulltitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchmissingfulltitle", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_missing_full_title)
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinctitle", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinctitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradeinctitle", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_title))
			}
		}
		if schedule.Cron_indexer_upgrade_title != "" {
			QueueSearch.DispatchCron("searchupgradeinctitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradeinctitle", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_upgrade_title)
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefulltitle", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradefulltitle_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("searchupgradefulltitle", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full_title))
			}
		}
		if schedule.Cron_indexer_upgrade_full_title != "" {
			QueueSearch.DispatchCron("searchupgradefulltitle_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("searchupgradefulltitle", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full_title)
		}
		if schedule.Interval_indexer_rss != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rss_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("rss", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_rss))
			} else {
				QueueSearch.DispatchEvery("rss_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("rss", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_indexer_rss))
			}
		}
		if schedule.Cron_indexer_rss != "" {
			QueueSearch.DispatchCron("rss_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("rss", cfg_movie.Name, "", false)
			}, schedule.Cron_indexer_rss)
		}

		if schedule.Interval_scan_data != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("datafull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("datafull", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_scan_data))
			} else {
				QueueData.DispatchEvery("datafull_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("datafull", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_scan_data))
			}
		}
		if schedule.Cron_scan_data != "" {
			QueueData.DispatchCron("datafull_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("datafull", cfg_movie.Name, "", false)
			}, schedule.Cron_scan_data)
		}
		if schedule.Interval_scan_dataimport != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("structure_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("structure", cfg_movie.Name, "", false)
				}, convertcron(schedule.Interval_scan_dataimport))
			} else {
				QueueData.DispatchEvery("structure_movies_"+cfg_movie.Name, func() {
					utils.Movies_single_jobs("structure", cfg_movie.Name, "", false)
				}, converttime(schedule.Interval_scan_dataimport))
			}
		}
		if schedule.Cron_scan_dataimport != "" {
			QueueData.DispatchCron("structure_movies_"+cfg_movie.Name, func() {
				utils.Movies_single_jobs("structure", cfg_movie.Name, "", false)
			}, schedule.Cron_scan_dataimport)
		}

		for idxlist := range cfg_movie.Lists {
			if !cfg_movie.Lists[idxlist].Enabled {
				continue
			}
			if !config.ConfigCheck("list_" + cfg_movie.Lists[idxlist].Template_list) {
				continue
			}
			var cfg_list config.ListsConfig
			config.ConfigGet("list_"+cfg_movie.Lists[idxlist].Template_list, &cfg_list)
			if !cfg_list.Enabled {
				continue
			}
			if cfg_movie.Lists[idxlist].Template_scheduler != "" {
				if !config.ConfigCheck("scheduler_" + cfg_movie.Lists[idxlist].Template_scheduler) {
					continue
				}
				config.ConfigGet("scheduler_"+cfg_movie.Lists[idxlist].Template_scheduler, &schedule)
			}
			if schedule.Interval_feeds != "" {
				if cfg_general.UseCronInsteadOfInterval {
					QueueFeeds.DispatchCron("feeds_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
						utils.Movies_single_jobs("feeds", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
					}, convertcron(schedule.Interval_feeds))
				} else {
					QueueFeeds.DispatchEvery("feeds_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
						utils.Movies_single_jobs("feeds", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
					}, converttime(schedule.Interval_feeds))
				}
			}
			if schedule.Cron_feeds != "" {
				QueueFeeds.DispatchCron("feeds_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
					utils.Movies_single_jobs("feeds", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
				}, schedule.Cron_feeds)
			}

			if schedule.Interval_scan_data_missing != "" {
				if cfg_general.UseCronInsteadOfInterval {
					QueueData.DispatchCron("checkmissing_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
						utils.Movies_single_jobs("checkmissing", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
					}, convertcron(schedule.Interval_scan_data_missing))

					QueueData.DispatchCron("checkmissingflag_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
						utils.Movies_single_jobs("checkmissingflag", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
					}, convertcron(schedule.Interval_scan_data_missing))
				} else {
					QueueData.DispatchEvery("checkmissing_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
						utils.Movies_single_jobs("checkmissing", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
					}, converttime(schedule.Interval_scan_data_missing))

					QueueData.DispatchEvery("checkmissingflag_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
						utils.Movies_single_jobs("checkmissingflag", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
					}, converttime(schedule.Interval_scan_data_missing))
				}
			}
			if schedule.Cron_scan_data_missing != "" {
				QueueData.DispatchCron("checkmissing_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
					utils.Movies_single_jobs("checkmissing", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
				}, schedule.Cron_scan_data_missing)

				QueueData.DispatchCron("checkmissingflag_movies_"+cfg_movie.Name+"_"+cfg_movie.Lists[idxlist].Name, func() {
					utils.Movies_single_jobs("checkmissingflag", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
				}, schedule.Cron_scan_data_missing)
			}
		}
	}

	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigGet(string(idxserie), &cfg_serie)

		if !config.ConfigCheck("scheduler_" + cfg_serie.Template_scheduler) {
			continue
		}
		var schedule config.SchedulerConfig
		config.ConfigGet("scheduler_"+cfg_serie.Template_scheduler, &schedule)

		if schedule.Interval_indexer_missing != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginc", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing))
			} else {
				QueueSearch.DispatchEvery("searchmissinginc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginc", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing))
			}
		}
		if schedule.Cron_indexer_missing != "" {
			QueueSearch.DispatchCron("searchmissinginc_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissinginc", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_missing)
		}
		if schedule.Interval_indexer_missing_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfull", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full))
			} else {
				QueueSearch.DispatchEvery("searchmissingfull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfull", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full))
			}
		}
		if schedule.Cron_indexer_missing_full != "" {
			QueueSearch.DispatchCron("searchmissingfull_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissingfull", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_missing_full)
		}
		if schedule.Interval_indexer_upgrade != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinc", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinc_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinc", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade))
			}
		}
		if schedule.Cron_indexer_upgrade != "" {
			QueueSearch.DispatchCron("searchupgradeinc_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradeinc", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_upgrade)
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefull", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full))
			} else {
				QueueSearch.DispatchEvery("searchupgradefull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefull", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full))
			}
		}
		if schedule.Cron_indexer_upgrade_full != "" {
			QueueSearch.DispatchCron("searchupgradefull_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradefull", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full)
		}

		if schedule.Interval_indexer_missing_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissinginctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginctitle", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_title))
			} else {
				QueueSearch.DispatchEvery("searchmissinginctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissinginctitle", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_title))
			}
		}
		if schedule.Cron_indexer_missing_title != "" {
			QueueSearch.DispatchCron("searchmissinginctitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissinginctitle", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_missing_title)
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchmissingfulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfulltitle", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_missing_full_title))
			} else {
				QueueSearch.DispatchEvery("searchmissingfulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchmissingfulltitle", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_missing_full_title))
			}
		}
		if schedule.Cron_indexer_missing_full_title != "" {
			QueueSearch.DispatchCron("searchmissingfulltitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchmissingfulltitle", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_missing_full_title)
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradeinctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinctitle", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradeinctitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradeinctitle", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_title))
			}
		}
		if schedule.Cron_indexer_upgrade_title != "" {
			QueueSearch.DispatchCron("searchupgradeinctitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradeinctitle", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_upgrade_title)
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("searchupgradefulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefulltitle", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_upgrade_full_title))
			} else {
				QueueSearch.DispatchEvery("searchupgradefulltitle_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("searchupgradefulltitle", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_upgrade_full_title))
			}
		}
		if schedule.Cron_indexer_upgrade_full_title != "" {
			QueueSearch.DispatchCron("searchupgradefulltitle_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("searchupgradefulltitle", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_upgrade_full_title)
		}
		if schedule.Interval_indexer_rss != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueSearch.DispatchCron("rss_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("rss", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_indexer_rss))
			} else {
				QueueSearch.DispatchEvery("rss_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("rss", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_indexer_rss))
			}
		}
		if schedule.Cron_indexer_rss != "" {
			QueueSearch.DispatchCron("rss_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("rss", cfg_serie.Name, "", false)
			}, schedule.Cron_indexer_rss)
		}

		if schedule.Interval_scan_data != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("datafull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("datafull", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_scan_data))
			} else {
				QueueData.DispatchEvery("datafull_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("datafull", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_scan_data))
			}
		}
		if schedule.Cron_scan_data != "" {
			QueueData.DispatchCron("datafull_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("datafull", cfg_serie.Name, "", false)
			}, schedule.Cron_scan_data)
		}
		if schedule.Interval_scan_dataimport != "" {
			if cfg_general.UseCronInsteadOfInterval {
				QueueData.DispatchCron("structure_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("structure", cfg_serie.Name, "", false)
				}, convertcron(schedule.Interval_scan_dataimport))
			} else {
				QueueData.DispatchEvery("structure_series_"+cfg_serie.Name, func() {
					utils.Series_single_jobs("structure", cfg_serie.Name, "", false)
				}, converttime(schedule.Interval_scan_dataimport))
			}
		}
		if schedule.Cron_scan_dataimport != "" {
			QueueData.DispatchCron("structure_series_"+cfg_serie.Name, func() {
				utils.Series_single_jobs("structure", cfg_serie.Name, "", false)
			}, schedule.Cron_scan_dataimport)
		}

		for idxlist := range cfg_serie.Lists {
			if !cfg_serie.Lists[idxlist].Enabled {
				continue
			}

			if !config.ConfigCheck("list_" + cfg_serie.Lists[idxlist].Template_list) {
				continue
			}
			var cfg_list config.ListsConfig
			config.ConfigGet("list_"+cfg_serie.Lists[idxlist].Template_list, &cfg_list)
			if !cfg_list.Enabled {
				continue
			}

			if !config.ConfigCheck("scheduler_" + cfg_serie.Template_scheduler) {
				continue
			}
			config.ConfigGet("scheduler_"+cfg_serie.Template_scheduler, &schedule)
			if cfg_serie.Lists[idxlist].Template_scheduler != "" {
				if !config.ConfigCheck("scheduler_" + cfg_serie.Lists[idxlist].Template_scheduler) {
					continue
				}
				config.ConfigGet("scheduler_"+cfg_serie.Lists[idxlist].Template_scheduler, &schedule)
			}
			if schedule.Interval_feeds != "" {
				if cfg_general.UseCronInsteadOfInterval {
					QueueFeeds.DispatchCron("feeds_series_"+cfg_serie.Name+"_"+cfg_serie.Lists[idxlist].Name, func() {
						utils.Series_single_jobs("feeds", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
					}, convertcron(schedule.Interval_feeds))
				} else {
					QueueFeeds.DispatchEvery("feeds_series_"+cfg_serie.Name+"_"+cfg_serie.Lists[idxlist].Name, func() {
						utils.Series_single_jobs("feeds", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
					}, converttime(schedule.Interval_feeds))
				}
			}
			if schedule.Cron_feeds != "" {
				QueueFeeds.DispatchCron("feeds_series_"+cfg_serie.Name+"_"+cfg_serie.Lists[idxlist].Name, func() {
					utils.Series_single_jobs("feeds", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
				}, schedule.Cron_feeds)
			}

			if schedule.Interval_scan_data_missing != "" {
				if cfg_general.UseCronInsteadOfInterval {
					QueueData.DispatchCron("checkmissing_series_"+cfg_serie.Name+"_"+cfg_serie.Lists[idxlist].Name, func() {
						utils.Series_single_jobs("checkmissing", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
						utils.Series_single_jobs("checkmissingflag", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
					}, convertcron(schedule.Interval_scan_data_missing))
				} else {
					QueueData.DispatchEvery("checkmissing_series_"+cfg_serie.Name+"_"+cfg_serie.Lists[idxlist].Name, func() {
						utils.Series_single_jobs("checkmissing", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
						utils.Series_single_jobs("checkmissingflag", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
					}, converttime(schedule.Interval_scan_data_missing))
				}
			}
			if schedule.Cron_scan_data_missing != "" {
				QueueData.DispatchCron("checkmissing_series_"+cfg_serie.Name+"_"+cfg_serie.Lists[idxlist].Name, func() {
					utils.Series_single_jobs("checkmissing", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
					utils.Series_single_jobs("checkmissingflag", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
				}, schedule.Cron_scan_data_missing)
			}
		}
	}

	var defaultschedule config.SchedulerConfig
	if !config.ConfigCheck("scheduler_Default") {
		configs := config.ConfigGetAll()
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
		configs["scheduler_Default"] = defaultschedule
		config.UpdateCfg(configs)
		config.WriteCfg()
	} else {
		config.ConfigGet("scheduler_"+"Default", &defaultschedule)
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
				file := "./init_imdb"
				if runtime.GOOS == "windows" {
					file = "init_imdb.exe"
				}
				exec.Command(file).Run()
				if _, err := os.Stat(file); !os.IsNotExist(err) {
					database.DBImdb.Close()
					os.Remove("./imdb.db")
					os.Rename("./imdbtemp.db", "./imdb.db")
					dbnew := database.InitImdbdb("info", "imdb")
					dbnew.SetMaxOpenConns(5)
					database.DBImdb = dbnew
				}
			}, convertcron(defaultschedule.Interval_imdb))
		} else {
			QueueFeeds.DispatchEvery("Refresh IMDB", func() {
				//utils.InitFillImdb()
				file := "./init_imdb"
				if runtime.GOOS == "windows" {
					file = "init_imdb.exe"
				}
				exec.Command(file).Run()
				if _, err := os.Stat(file); !os.IsNotExist(err) {
					database.DBImdb.Close()
					os.Remove("./imdb.db")
					os.Rename("./imdbtemp.db", "./imdb.db")
					dbnew := database.InitImdbdb("info", "imdb")
					dbnew.SetMaxOpenConns(5)
					database.DBImdb = dbnew
				}
			}, converttime(defaultschedule.Interval_imdb))
		}
	}
	if defaultschedule.Cron_imdb != "" {
		QueueFeeds.DispatchCron("Refresh IMDB", func() {
			//utils.InitFillImdb()
			file := "./init_imdb"
			if runtime.GOOS == "windows" {
				file = "init_imdb.exe"
			}
			exec.Command(file).Run()
			if _, err := os.Stat(file); !os.IsNotExist(err) {
				database.DBImdb.Close()
				os.Remove("./imdb.db")
				os.Rename("./imdbtemp.db", "./imdb.db")
				dbnew := database.InitImdbdb("info", "imdb")
				dbnew.SetMaxOpenConns(5)
				database.DBImdb = dbnew
			}
		}, defaultschedule.Cron_imdb)
	}

	if defaultschedule.Interval_database_check != "" {
		QueueData.DispatchEvery("Check Database", func() {
			str := database.DbIntegrityCheck()
			if str != "ok" {
				os.Exit(100)
			}
		}, converttime(defaultschedule.Interval_database_check))
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
		QueueData.DispatchEvery("Backup Database", func() {
			database.Backup(database.DB, fmt.Sprintf("%s.%s.%s", "./backup/data.db", database.DBVersion, time.Now().Format("20060102_150405")))
		}, converttime(defaultschedule.Interval_database_backup))
	}
	if defaultschedule.Cron_database_backup != "" {
		QueueData.DispatchCron("Backup Database", func() {
			database.Backup(database.DB, fmt.Sprintf("%s.%s.%s", "./backup/data.db", database.DBVersion, time.Now().Format("20060102_150405")))
		}, defaultschedule.Cron_database_backup)
	}
}

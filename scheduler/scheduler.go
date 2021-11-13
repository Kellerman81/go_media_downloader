package scheduler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
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

var QueueFeeds *tasks.Dispatcher
var QueueData *tasks.Dispatcher
var QueueSearch *tasks.Dispatcher

func InitScheduler() {
	if !config.ConfigCheck("general") {
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	QueueFeeds = tasks.NewDispatcher(1, 100)
	QueueFeeds.Start()

	QueueData = tasks.NewDispatcher(1, 100)
	QueueData.Start()

	QueueSearch = tasks.NewDispatcher(cfg_general.ConcurrentScheduler, 100)
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
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchmissinginc", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing))
		}
		if schedule.Interval_indexer_missing_full != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchmissingfull", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full))
		}
		if schedule.Interval_indexer_upgrade != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchupgradeinc", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade))
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchupgradefull", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full))
		}

		if schedule.Interval_indexer_missing_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchmissinginctitle", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_title))
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchmissingfulltitle", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full_title))
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchupgradeinctitle", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_title))
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("searchupgradefulltitle", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full_title))
		}
		if schedule.Interval_indexer_rss != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Movies_single_jobs("rss", cfg_movie.Name, "", false)
			}, converttime(schedule.Interval_indexer_rss))
		}

		for idxlist := range cfg_movie.Lists {
			if cfg_movie.Lists[idxlist].Template_scheduler != "" {
				if !config.ConfigCheck("scheduler_" + cfg_movie.Lists[idxlist].Template_scheduler) {
					continue
				}
				config.ConfigGet("scheduler_"+cfg_movie.Lists[idxlist].Template_scheduler, &schedule)
			}
			if schedule.Interval_feeds != "" {
				QueueFeeds.DispatchEvery(func() {
					utils.Movies_single_jobs("feeds", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_feeds))
			}

			if schedule.Interval_scan_data_missing != "" {
				QueueData.DispatchEvery(func() {
					utils.Movies_single_jobs("checkmissing", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_scan_data_missing))

				QueueData.DispatchEvery(func() {
					utils.Movies_single_jobs("checkmissingflag", cfg_movie.Name, cfg_movie.Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_scan_data_missing))
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

	if defaultschedule.Interval_scan_data != "" {
		QueueData.DispatchEvery(func() {
			utils.Movies_all_jobs("datafull", false)
		}, converttime(defaultschedule.Interval_scan_data))
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
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchmissinginc", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing))
		}
		if schedule.Interval_indexer_missing_full != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchmissingfull", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full))
		}
		if schedule.Interval_indexer_upgrade != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchupgradeinc", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade))
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchupgradefull", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full))
		}

		if schedule.Interval_indexer_missing_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchmissinginctitle", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_title))
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchmissingfulltitle", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full_title))
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchupgradeinctitle", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_title))
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("searchupgradefulltitle", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full_title))
		}
		if schedule.Interval_indexer_rss != "" {
			QueueSearch.DispatchEvery(func() {
				utils.Series_single_jobs("rss", cfg_serie.Name, "", false)
			}, converttime(schedule.Interval_indexer_rss))
		}
		for idxlist := range cfg_serie.Lists {
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
				QueueFeeds.DispatchEvery(func() {
					utils.Series_single_jobs("feeds", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_feeds))
			}

			if schedule.Interval_scan_data_missing != "" {
				QueueData.DispatchEvery(func() {
					utils.Series_single_jobs("checkmissing", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
					utils.Series_single_jobs("checkmissingflag", cfg_serie.Name, cfg_serie.Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_scan_data_missing))
			}
		}
	}
	if defaultschedule.Interval_scan_data != "" {
		QueueData.DispatchEvery(func() {
			utils.Series_all_jobs("datafull", false)
		}, converttime(defaultschedule.Interval_scan_data))
	}
	if defaultschedule.Interval_scan_dataimport != "" {
		QueueData.DispatchEvery(func() {
			utils.Series_all_jobs("structure", false)
		}, converttime(defaultschedule.Interval_scan_dataimport))

		QueueData.DispatchEvery(func() {
			utils.Movies_all_jobs("structure", false)
		}, converttime(defaultschedule.Interval_scan_dataimport))
	}

	if defaultschedule.Interval_feeds_refresh_series_full != "" {
		QueueFeeds.DispatchEvery(func() {
			utils.RefreshSeries()
		}, converttime(defaultschedule.Interval_feeds_refresh_series_full))
	}
	if defaultschedule.Interval_feeds_refresh_movies_full != "" {
		QueueFeeds.DispatchEvery(func() {
			utils.RefreshMovies()
		}, converttime(defaultschedule.Interval_feeds_refresh_movies_full))
	}
	if defaultschedule.Interval_feeds_refresh_series != "" {
		QueueFeeds.DispatchEvery(func() {
			utils.RefreshSeriesInc()
		}, converttime(defaultschedule.Interval_feeds_refresh_series))
	}
	if defaultschedule.Interval_feeds_refresh_movies != "" {
		QueueFeeds.DispatchEvery(func() {
			utils.RefreshMoviesInc()
		}, converttime(defaultschedule.Interval_feeds_refresh_movies))
	}
	if defaultschedule.Interval_imdb != "" {
		QueueFeeds.DispatchEvery(func() {
			utils.InitFillImdb()
		}, converttime(defaultschedule.Interval_imdb))
	}
}

type joblist struct {
	Type      string
	Lastrun   time.Time
	Nextrun   time.Time
	Runcount  int
	Scheduled string
	IsRunning bool
	Tags      []string
}

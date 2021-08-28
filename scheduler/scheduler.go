package scheduler

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/api"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/tasks"
	"github.com/Kellerman81/go_media_downloader/utils"
	"github.com/gin-gonic/gin"
)

// var ScheduleFeeds *gocron.Scheduler
// var ScheduleSearch *gocron.Scheduler
// var ScheduleData *gocron.Scheduler

// func ClearScheduler() {
// 	ScheduleData.Clear()
// 	ScheduleFeeds.Clear()
// 	ScheduleSearch.Clear()
// }
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
func InitScheduler() {
	what := []string{"General", "Regex", "Quality", "Path", "Indexer", "Scheduler", "Movie", "Serie", "Imdb", "Downloader", "Notification", "List"}
	cfg, f, _ := config.LoadCfg(what, config.Configfile)
	config.Watch(f, config.Configfile, what)

	feeds := tasks.NewDispatcher(cfg.General.ConcurrentScheduler, 100)
	feeds.Start()

	data := tasks.NewDispatcher(cfg.General.ConcurrentScheduler, 100)
	data.Start()

	search := tasks.NewDispatcher(cfg.General.ConcurrentScheduler, 100)
	search.Start()

	for idx := range cfg.Movie {
		schedule := cfg.Scheduler[cfg.Movie[idx].Template_scheduler]
		if schedule.Interval_indexer_missing != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchmissinginc", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing))
		}
		if schedule.Interval_indexer_missing_full != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchmissingfull", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full))
		}
		if schedule.Interval_indexer_upgrade != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchupgradeinc", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade))
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchupgradefull", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full))
		}

		if schedule.Interval_indexer_missing_title != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchmissinginctitle", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_title))
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchmissingfulltitle", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full_title))
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchupgradeinctitle", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_title))
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "searchupgradefulltitle", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full_title))
		}
		if schedule.Interval_indexer_rss != "" {
			search.DispatchEvery(func() {
				api.Movies_single_jobs(cfg, "rss", cfg.Movie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_rss))
		}

		for idxlist := range cfg.Movie[idx].Lists {
			if cfg.Movie[idx].Lists[idxlist].Template_scheduler != "" {
				schedule = cfg.Scheduler[cfg.Movie[idx].Lists[idxlist].Template_scheduler]
			}
			if schedule.Interval_feeds != "" {
				feeds.DispatchEvery(func() {
					api.Movies_single_jobs(cfg, "feeds", cfg.Movie[idx].Name, cfg.Movie[idx].Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_feeds))
			}

			if schedule.Interval_scan_data_missing != "" {
				data.DispatchEvery(func() {
					api.Movies_single_jobs(cfg, "checkmissing", cfg.Movie[idx].Name, cfg.Movie[idx].Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_scan_data_missing))

				data.DispatchEvery(func() {
					api.Movies_single_jobs(cfg, "checkmissingflag", cfg.Movie[idx].Name, cfg.Movie[idx].Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_scan_data_missing))
			}
		}
	}

	defaultschedule := cfg.Scheduler["Default"]
	if defaultschedule.Interval_scan_data != "" {
		data.DispatchEvery(func() {
			api.Movies_all_jobs_cfg(cfg, "data", false)
		}, converttime(defaultschedule.Interval_scan_data))
	}

	for idx := range cfg.Serie {
		schedule := cfg.Scheduler[cfg.Serie[idx].Template_scheduler]
		if schedule.Interval_indexer_missing != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchmissinginc", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing))
		}
		if schedule.Interval_indexer_missing_full != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchmissingfull", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full))
		}
		if schedule.Interval_indexer_upgrade != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchupgradeinc", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade))
		}
		if schedule.Interval_indexer_upgrade_full != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchupgradefull", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full))
		}

		if schedule.Interval_indexer_missing_title != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchmissinginctitle", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_title))
		}
		if schedule.Interval_indexer_missing_full_title != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchmissingfulltitle", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_missing_full_title))
		}
		if schedule.Interval_indexer_upgrade_title != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchupgradeinctitle", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_title))
		}
		if schedule.Interval_indexer_upgrade_full_title != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "searchupgradefulltitle", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_upgrade_full_title))
		}
		if schedule.Interval_indexer_rss != "" {
			search.DispatchEvery(func() {
				api.Series_single_jobs(cfg, "rss", cfg.Serie[idx].Name, "", false)
			}, converttime(schedule.Interval_indexer_rss))
		}
		for idxlist := range cfg.Serie[idx].Lists {
			schedule := cfg.Scheduler[cfg.Serie[idx].Template_scheduler]
			if cfg.Serie[idx].Lists[idxlist].Template_scheduler != "" {
				schedule = cfg.Scheduler[cfg.Serie[idx].Lists[idxlist].Template_scheduler]
			}
			if schedule.Interval_feeds != "" {
				feeds.DispatchEvery(func() {
					api.Series_single_jobs(cfg, "feeds", cfg.Serie[idx].Name, cfg.Serie[idx].Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_feeds))
			}

			if schedule.Interval_scan_data_missing != "" {
				data.DispatchEvery(func() {
					api.Series_single_jobs(cfg, "checkmissing", cfg.Serie[idx].Name, cfg.Serie[idx].Lists[idxlist].Name, false)
					api.Series_single_jobs(cfg, "checkmissingflag", cfg.Serie[idx].Name, cfg.Serie[idx].Lists[idxlist].Name, false)
				}, converttime(schedule.Interval_scan_data_missing))
			}
		}
	}
	if defaultschedule.Interval_scan_data != "" {
		data.DispatchEvery(func() {
			api.Series_all_jobs_cfg(cfg, "data", false)
		}, converttime(defaultschedule.Interval_scan_data))
	}
	if defaultschedule.Interval_scan_dataimport != "" {
		data.DispatchEvery(func() {
			api.Series_all_jobs_cfg(cfg, "structure", false)
		}, converttime(defaultschedule.Interval_scan_dataimport))

		data.DispatchEvery(func() {
			api.Movies_all_jobs_cfg(cfg, "structure", false)
		}, converttime(defaultschedule.Interval_scan_dataimport))
	}

	if defaultschedule.Interval_feeds_refresh_series_full != "" {
		feeds.DispatchEvery(func() {
			api.RefreshSeries(cfg)
		}, converttime(defaultschedule.Interval_feeds_refresh_series_full))
	}
	if defaultschedule.Interval_feeds_refresh_movies_full != "" {
		feeds.DispatchEvery(func() {
			api.RefreshMovies(cfg)
		}, converttime(defaultschedule.Interval_feeds_refresh_movies_full))
	}
	if defaultschedule.Interval_feeds_refresh_series != "" {
		feeds.DispatchEvery(func() {
			api.RefreshSeriesInc(cfg)
		}, converttime(defaultschedule.Interval_feeds_refresh_series))
	}
	if defaultschedule.Interval_feeds_refresh_movies != "" {
		feeds.DispatchEvery(func() {
			api.RefreshMoviesInc(cfg)
		}, converttime(defaultschedule.Interval_feeds_refresh_movies))
	}
	if defaultschedule.Interval_imdb != "" {
		feeds.DispatchEvery(func() {
			utils.InitFillImdb()
		}, converttime(defaultschedule.Interval_imdb))
	}
}

func AddSchedulerRoutes(rg *gin.RouterGroup) {
	//	movies := rg.Group("/movies")
	// movies.GET("/:ID", GetMovie)
	// movies.GET("/", GetMovies)
	// movies.POST("/", PostMovie)
	// movies.DELETE("/:ID", DeleteMovie)
	// movies.PUT("/:ID", UpdateMovie)
	//rg.GET("/jobs", apiGetJobs)
	//rg.GET("/runtag/:group/:name", apiRunJobs)
	//rg.GET("/removetag/:group/:name", apiRemoveJobs)

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

// func apiRemoveJobs(ctx *gin.Context) {
// 	switch ctx.Param("group") {
// 	case "Data":
// 		ScheduleData.RemoveByTag(ctx.Param("name"))
// 	case "Feeds":
// 		ScheduleFeeds.RemoveByTag(ctx.Param("name"))
// 	case "Search":
// 		ScheduleSearch.RemoveByTag(ctx.Param("name"))
// 	}
// }

// func apiRunJobs(ctx *gin.Context) {
// 	switch ctx.Param("group") {
// 	case "Data":
// 		ScheduleData.RunByTag(ctx.Param("name"))
// 	case "Feeds":
// 		ScheduleFeeds.RunByTag(ctx.Param("name"))
// 	case "Search":
// 		ScheduleSearch.RunByTag(ctx.Param("name"))
// 	}
// }

// func apiGetJobs(ctx *gin.Context) {
// 	alljobs := []joblist{}
// 	jobs := ScheduleData.Jobs()
// 	for _, job := range jobs {
// 		var addjob joblist
// 		addjob.Type = "Data"
// 		addjob.Lastrun = job.LastRun()
// 		addjob.Nextrun = job.NextRun()
// 		addjob.Runcount = job.RunCount()
// 		addjob.Scheduled = job.ScheduledAtTime()
// 		addjob.Tags = job.Tags()
// 		addjob.IsRunning = ScheduleData.IsRunning()
// 		alljobs = append(alljobs, addjob)
// 	}

// 	jobs = ScheduleFeeds.Jobs()
// 	for _, job := range jobs {
// 		var addjob joblist
// 		addjob.Type = "Feeds"
// 		addjob.Lastrun = job.LastRun()
// 		addjob.Nextrun = job.NextRun()
// 		addjob.Runcount = job.RunCount()
// 		addjob.Scheduled = job.ScheduledAtTime()
// 		addjob.Tags = job.Tags()
// 		addjob.IsRunning = ScheduleFeeds.IsRunning()
// 		alljobs = append(alljobs, addjob)
// 	}

// 	jobs = ScheduleSearch.Jobs()
// 	for _, job := range jobs {
// 		var addjob joblist
// 		addjob.Type = "Search"
// 		addjob.Lastrun = job.LastRun()
// 		addjob.Nextrun = job.NextRun()
// 		addjob.Runcount = job.RunCount()
// 		addjob.Scheduled = job.ScheduledAtTime()
// 		addjob.Tags = job.Tags()
// 		addjob.IsRunning = ScheduleSearch.IsRunning()
// 		alljobs = append(alljobs, addjob)
// 	}

// 	ctx.JSON(http.StatusOK, gin.H{"data": alljobs})
// }

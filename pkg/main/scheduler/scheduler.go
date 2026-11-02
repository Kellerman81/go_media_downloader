package scheduler

import (
	"errors"
	"fmt"
	"hash/fnv"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies" // Register movie handler
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series" // Register series handler
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// scheduleSpec is everything that determines how one job is scheduled. Two
// equal specs produce the same schedule, which is what lets Reload keep
// unchanged schedules (and their ticker phase) untouched.
type scheduleSpec struct {
	cfgp     string
	interval string
	cron     string
	name     string
	queue    string
	jobname  string
	useCron  bool
}

type activeSchedule struct {
	spec scheduleSpec
	id   uint32
}

var (
	// applyMu serializes InitScheduler/Reload; active is only touched under it.
	applyMu sync.Mutex
	active  = make(map[string]activeSchedule)
	// initialized gates Reload: a config reload that arrives before startup
	// has created the cron workers must not dispatch anything yet.
	initialized atomic.Bool

	errEmptyInterval = errors.New("empty interval")
)

// mediaJobNames are the per-media-config schedules, in registration order.
var mediaJobNames = []string{
	"refreshseriesfull", "refreshseriesinc", "refreshmoviesfull", "refreshmoviesinc",
	"refreshbooksfull", "refreshbooksinc", "refreshaudiobooksfull", "refreshaudiobooksinc",
	"refreshmusicfull", "refreshmusicinc",
	logger.StrSearchMissingInc, logger.StrSearchMissingFull, logger.StrSearchUpgradeInc,
	logger.StrSearchUpgradeFull, logger.StrSearchMissingIncTitle, logger.StrSearchMissingFullTitle,
	logger.StrSearchUpgradeIncTitle, logger.StrSearchUpgradeFullTitle, logger.StrRss,
	logger.StrDataFull, logger.StrStructure, logger.StrFeeds, logger.StrCheckMissing,
	logger.StrCheckMissingFlag, logger.StrUpgradeFlag, logger.StrRssSeasons,
	logger.StrRssSeasonsAll, logger.StrRssArtists, logger.StrRssArtistsUpgrade,
	logger.StrRssAuthors, logger.StrRssAuthorsUpgrade,
}

// InitScheduler is called at startup to initialize the scheduler: it makes
// sure the Default scheduler template exists, registers the job functions and
// dispatches every configured schedule.
func InitScheduler() {
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
			IntervalCacheRefresh:       "6h",
		}})
		config.WriteCfg()
	}

	initialized.Store(true)
	Reload()
}

// Reload re-registers the job functions for the current config and, once
// InitScheduler has run, brings the dispatched schedules in line with it. Schedules whose settings did not change
// are left alone; removed or changed ones are unregistered first. Unregistering
// only stops a schedule from submitting new jobs - jobs already queued or
// running are not affected.
func Reload() {
	// Held across the job-map rebuild too: the config watcher and web UI
	// saves can call Reload concurrently.
	applyMu.Lock()
	defer applyMu.Unlock()

	utils.LoadGlobalSchedulerConfig()
	utils.LoadSchedulerConfig()

	if initialized.Load() {
		apply(collectSchedules())
	}
}

// apply must be called with applyMu held.
func apply(specs []scheduleSpec) {
	desired := make(map[string]scheduleSpec, len(specs))
	for _, s := range specs {
		desired[s.name] = s
	}

	removed := 0

	for name, a := range active {
		if d, ok := desired[name]; ok && d == a.spec {
			continue
		}

		worker.RemoveSchedule(a.id)
		delete(active, name)

		removed++
	}

	added := 0

	for _, s := range specs {
		if _, ok := active[s.name]; ok {
			continue
		}

		if id, ok := dispatch(s); ok {
			active[s.name] = activeSchedule{spec: s, id: id}
			added++
		}
	}

	if removed > 0 || added > 0 {
		logger.Logtype("info", 0).
			Int("added", added).
			Int("removed", removed).
			Int("active", len(active)).
			Msg("Scheduler updated")
	}
}

// collectSchedules builds the schedule list from the current config.
func collectSchedules() []scheduleSpec {
	useCron := config.GetSettingsGeneral().UseCronInsteadOfInterval
	def := config.GetSettingsScheduler("Default")

	var specs []scheduleSpec

	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		if cfgp.CfgScheduler == nil {
			logger.Logtype("error", 0).
				Str("config", cfgp.NamePrefix).
				Str("template", cfgp.TemplateScheduler).
				Msg("Scheduler template not found - no jobs scheduled for this media config")

			return nil
		}

		groupnamestr := mediatype.GetCategoryName(cfgp.IsType)
		h := mediatype.Get(cfgp.IsType)

		for _, str := range mediaJobNames {
			intervalstr, cronstr := mediaSchedule(cfgp, def, h, str)
			if intervalstr == "" && cronstr == "" {
				continue
			}

			if cfgp.Jobs[str] == nil {
				// The refresh* schedules come from the global Default
				// template and apply only to the type they name.
				if !strings.HasPrefix(str, "refresh") {
					logger.Logtype("warn", 0).
						Str("config", cfgp.NamePrefix).
						Str(logger.StrJob, str).
						Msg("Schedule configured for a job this media type does not have - ignored")
				}

				continue
			}

			specs = append(specs, scheduleSpec{
				cfgp:     cfgp.NamePrefix,
				interval: intervalstr,
				cron:     cronstr,
				name:     logger.JoinStrings(str, logger.Underscore, groupnamestr, logger.Underscore, cfgp.Name),
				queue:    mediaQueue(str),
				jobname:  str,
				useCron:  useCron,
			})
		}

		return nil
	})

	if def == nil {
		logger.Logtype("error", 0).
			Msg("Scheduler template Default not found - global jobs not scheduled")

		return specs
	}

	for _, g := range []struct {
		interval, cron, name, queue, jobname string
	}{
		{def.IntervalDatabaseBackup, def.CronDatabaseBackup, "Backup Database", worker.QueueData, "BackupDatabase"},
		{def.IntervalDatabaseCheck, def.CronDatabaseCheck, "Check Database", worker.QueueData, "CheckDatabase"},
		{def.IntervalImdb, def.CronImdb, "Refresh IMDB", worker.QueueFeeds, "RefreshImdb"},
		{def.IntervalCacheRefresh, def.CronCacheRefresh, "Refresh Cache", worker.QueueData, "RefreshCache"},
	} {
		if g.interval == "" && g.cron == "" {
			continue
		}

		specs = append(specs, scheduleSpec{
			interval: g.interval,
			cron:     g.cron,
			name:     g.name,
			queue:    g.queue,
			jobname:  g.jobname,
			useCron:  useCron,
		})
	}

	return specs
}

func mediaQueue(str string) string {
	if strings.HasPrefix(str, "refresh") {
		return worker.QueueFeeds
	}

	switch str {
	case logger.StrDataFull,
		logger.StrStructure,
		logger.StrCheckMissing,
		logger.StrCheckMissingFlag,
		logger.StrUpgradeFlag:
		return worker.QueueData
	case logger.StrFeeds:
		return worker.QueueFeeds
	case logger.StrRss,
		logger.StrRssSeasons,
		logger.StrRssSeasonsAll,
		logger.StrRssArtists,
		logger.StrRssArtistsUpgrade,
		logger.StrRssAuthors,
		logger.StrRssAuthorsUpgrade:
		return worker.QueueRSS
	default:
		return worker.QueueSearch
	}
}

// mediaSchedule returns the interval and cron strings configured for job str.
func mediaSchedule(
	cfgp *config.MediaTypeConfig,
	def *config.SchedulerConfig,
	h mediatype.Handler,
	str string,
) (string, string) {
	s := cfgp.CfgScheduler

	switch str {
	case "refreshseriesfull":
		if def != nil {
			return def.IntervalFeedsRefreshSeriesFull, def.CronFeedsRefreshSeriesFull
		}
	case "refreshseriesinc":
		if def != nil {
			return def.IntervalFeedsRefreshSeries, def.CronFeedsRefreshSeries
		}
	case "refreshmoviesfull":
		if def != nil {
			return def.IntervalFeedsRefreshMoviesFull, def.CronFeedsRefreshMoviesFull
		}
	case "refreshmoviesinc":
		if def != nil {
			return def.IntervalFeedsRefreshMovies, def.CronFeedsRefreshMovies
		}
	case "refreshbooksfull":
		if def != nil {
			return def.IntervalFeedsRefreshBooksFull, def.CronFeedsRefreshBooksFull
		}
	case "refreshbooksinc":
		if def != nil {
			return def.IntervalFeedsRefreshBooks, def.CronFeedsRefreshBooks
		}
	case "refreshaudiobooksfull":
		if def != nil {
			return def.IntervalFeedsRefreshAudiobooksFull, def.CronFeedsRefreshAudiobooksFull
		}
	case "refreshaudiobooksinc":
		if def != nil {
			return def.IntervalFeedsRefreshAudiobooks, def.CronFeedsRefreshAudiobooks
		}
	case "refreshmusicfull":
		if def != nil {
			return def.IntervalFeedsRefreshMusicFull, def.CronFeedsRefreshMusicFull
		}
	case "refreshmusicinc":
		if def != nil {
			return def.IntervalFeedsRefreshMusic, def.CronFeedsRefreshMusic
		}
	case logger.StrSearchMissingInc:
		return s.IntervalIndexerMissing, s.CronIndexerMissing
	case logger.StrSearchMissingFull:
		return s.IntervalIndexerMissingFull, s.CronIndexerMissingFull
	case logger.StrSearchUpgradeInc:
		return s.IntervalIndexerUpgrade, s.CronIndexerUpgrade
	case logger.StrSearchUpgradeFull:
		return s.IntervalIndexerUpgradeFull, s.CronIndexerUpgradeFull
	case logger.StrSearchMissingIncTitle:
		return s.IntervalIndexerMissingTitle, s.CronIndexerMissingTitle
	case logger.StrSearchMissingFullTitle:
		return s.IntervalIndexerMissingFullTitle, s.CronIndexerMissingFullTitle
	case logger.StrSearchUpgradeIncTitle:
		return s.IntervalIndexerUpgradeTitle, s.CronIndexerUpgradeTitle
	case logger.StrSearchUpgradeFullTitle:
		return s.IntervalIndexerUpgradeFullTitle, s.CronIndexerUpgradeFullTitle
	case logger.StrRss:
		return s.IntervalIndexerRss, s.CronIndexerRss
	case logger.StrDataFull:
		return s.IntervalScanData, s.CronScanData
	case logger.StrStructure:
		return s.IntervalScanDataimport, s.CronScanDataimport
	case logger.StrFeeds:
		return s.IntervalFeeds, s.CronFeeds
	case logger.StrCheckMissing:
		return s.IntervalScanDataMissing, s.CronScanDataMissing
	case logger.StrCheckMissingFlag, logger.StrUpgradeFlag:
		// Both flag jobs share one setting; their job names differ, so they
		// are dispatched as two separate schedules.
		return s.IntervalScanDataFlags, s.CronScanDataFlags
	case logger.StrRssSeasons, logger.StrRssSeasonsAll:
		if h != nil {
			return h.GetSchedulerRssSeasons(s, str)
		}
	case logger.StrRssArtists,
		logger.StrRssArtistsUpgrade,
		logger.StrRssAuthors,
		logger.StrRssAuthorsUpgrade:
		if h != nil {
			return h.GetSchedulerRssArtistsAuthors(s, str)
		}
	}

	return "", ""
}

// dispatch registers one schedule with the worker. An explicit cron string
// wins over the interval, so a job is never scheduled twice.
func dispatch(s scheduleSpec) (uint32, bool) {
	if s.cron != "" {
		if s.interval != "" {
			logger.Logtype("info", 0).
				Str("name", s.name).
				Str("cron", s.cron).
				Str("interval", s.interval).
				Msg("Both cron and interval configured - using cron")
		}

		return dispatchCron(s, s.cron)
	}

	dur, err := parseInterval(s.interval)
	if err != nil {
		logger.Logtype("error", 0).
			Err(err).
			Str("name", s.name).
			Str("interval", s.interval).
			Msg("Invalid scheduler interval - skipping this job's schedule")

		return 0, false
	}

	if s.useCron {
		if cronstr, ok := intervalToCron(dur, s.name); ok {
			return dispatchCron(s, cronstr)
		}

		logger.Logtype("info", 0).
			Str("name", s.name).
			Str("interval", s.interval).
			Msg("Interval has no exact cron equivalent - using interval timer")
	}

	id, err := worker.DispatchEvery(s.cfgp, dur, s.name, s.queue, s.jobname)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Str("name", s.name).Msg("Cron interval")
		return 0, false
	}

	return id, true
}

func dispatchCron(s scheduleSpec, cronstr string) (uint32, bool) {
	id, err := worker.DispatchCron(s.cfgp, cronstr, s.name, s.queue, s.jobname)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Str("name", s.name).Str("cron", cronstr).Msg("Cron")
		return 0, false
	}

	logger.Logtype("debug", 0).Str("name", s.name).Str("cron", cronstr).Msg("Cron added")

	return id, true
}

// parseInterval parses a scheduler interval: Go duration syntax ("90m",
// "1h30m") optionally preceded by weeks and/or days ("1w", "2d", "1d12h").
func parseInterval(str string) (time.Duration, error) {
	rest := strings.ToLower(strings.TrimSpace(str))
	if rest == "" {
		return 0, errEmptyInterval
	}

	var total time.Duration

	for _, u := range []struct {
		suffix byte
		unit   time.Duration
	}{{'w', 7 * 24 * time.Hour}, {'d', 24 * time.Hour}} {
		i := 0
		for i < len(rest) && rest[i] >= '0' && rest[i] <= '9' {
			i++
		}

		if i == 0 || i >= len(rest) || rest[i] != u.suffix {
			continue
		}

		n, err := strconv.Atoi(rest[:i])
		if err != nil {
			return 0, fmt.Errorf("interval %q: %w", str, err)
		}

		total += time.Duration(n) * u.unit
		rest = rest[i+1:]
	}

	if rest != "" {
		d, err := time.ParseDuration(rest)
		if err != nil {
			return 0, fmt.Errorf("interval %q: %w", str, err)
		}

		total += d
	}

	if total <= 0 {
		return 0, fmt.Errorf("interval %q: must be positive", str)
	}

	return total, nil
}

// intervalToCron converts an interval to an equivalent 5-field cron
// expression when one exists: whole minutes dividing an hour, whole hours
// dividing a day, one day or one week. The minute/hour/weekday offset is
// derived from the job name, so jobs are spread out but keep the same slot
// across restarts.
func intervalToCron(d time.Duration, name string) (string, bool) {
	h := fnv.New32a()
	h.Write([]byte(name))
	seed := int(h.Sum32() & 0x7fffffff)

	minute := strconv.Itoa(seed % 60)
	hour := strconv.Itoa((seed / 60) % 24)

	switch {
	case d%time.Minute != 0:
		return "", false
	case d < time.Hour:
		n := int(d / time.Minute)
		if 60%n != 0 {
			return "", false
		}

		if n == 1 {
			return "* * * * *", true
		}

		return strconv.Itoa(seed%n) + "/" + strconv.Itoa(n) + " * * * *", true
	case d%time.Hour != 0:
		return "", false
	case d < 24*time.Hour:
		n := int(d / time.Hour)
		if 24%n != 0 {
			return "", false
		}

		if n == 1 {
			return minute + " * * * *", true
		}

		return minute + " " + strconv.Itoa((seed/60)%n) + "/" + strconv.Itoa(n) + " * * *", true
	case d == 24*time.Hour:
		return minute + " " + hour + " * * *", true
	case d == 7*24*time.Hour:
		return minute + " " + hour + " * * " + strconv.Itoa((seed/1440)%7), true
	}

	return "", false
}

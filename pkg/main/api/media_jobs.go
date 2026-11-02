package api

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

// mediaJobDispatchConfig holds the per-media-type parameters that vary across
// the otherwise-identical movies/series/books/audiobooks/music job-dispatch
// endpoints (apimoviesAllJobs/apimoviesJobs, apiseriesAllJobs/apiseriesJobs,
// apiBooksAllJobs/apiBooksJobs, apiAudiobooksAllJobs/apiAudiobooksJobs,
// apiMusicAllJobs/apiMusicJobs).
type mediaJobDispatchConfig struct {
	// TypeName is the singular media type name used in log messages,
	// e.g. "book", "audiobook", "music".
	TypeName string
	// NamePrefix is the config key prefix without the trailing underscore
	// (e.g. logger.StrMovie, logger.StrSerie, logger.StrBook, "music"); a
	// config belongs to this media type when its NamePrefix starts with
	// NamePrefix+"_".
	NamePrefix string
	// AllowedJobs is the comma-separated allowlist passed to validateJobParam.
	AllowedJobs string
}

// sendDispatchError reports a failed worker.Dispatch: 409 when the job is
// already queued (or was submitted within the duplicate window), 500 otherwise.
func sendDispatchError(c *gin.Context, err error) {
	if errors.Is(err, worker.ErrNotQueued) {
		sendJSONError(c, http.StatusConflict, "Job already queued: "+err.Error())
		return
	}

	sendJSONError(c, http.StatusInternalServerError, "Job not started: "+err.Error())
}

// findMediaConfigByName resolves a media config of one type by its configured
// name: exact key first, then a case-insensitive match on the name.
func findMediaConfigByName(prefix, name string) *config.MediaTypeConfig {
	if cfgp := config.GetSettingsMedia(prefix + "_" + name); cfgp != nil {
		return cfgp
	}

	var found *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, prefix+"_") && strings.EqualFold(media.Name, name) {
			found = media
			return true
		}

		return false
	})

	return found
}

// findMediaConfigByList returns the config of one media type that owns the
// list listname (case-insensitive) together with the list's configured name.
func findMediaConfigByList(prefix, listname string) (*config.MediaTypeConfig, string) {
	if listname == "" {
		return nil, ""
	}

	var (
		found *config.MediaTypeConfig
		name  string
	)

	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if !strings.HasPrefix(media.NamePrefix, prefix+"_") {
			return false
		}

		if idx := media.GetMediaListsEntryListID(listname); idx != -1 {
			found = media
			name = media.Lists[idx].Name

			return true
		}

		return false
	})

	return found, name
}

// mediaConfigForList returns the config of one media type owning listname,
// falling back to the first config of that type; nil when there is none.
func mediaConfigForList(prefix, listname string) *config.MediaTypeConfig {
	if cfgp, _ := findMediaConfigByList(prefix, listname); cfgp != nil {
		return cfgp
	}

	var first *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, prefix+"_") {
			first = media
			return true
		}

		return false
	})

	return first
}

// dispatchConfigFunc dispatches fn for cfgp on the Feeds queue under name and
// writes the response; 404 when cfgp is nil.
func dispatchConfigFunc(
	c *gin.Context,
	cfgp *config.MediaTypeConfig,
	typeName, name string,
	fn func(context.Context, *config.MediaTypeConfig) error,
) {
	if cfgp == nil {
		sendNotFound(c, "No "+typeName+" configuration found")
		return
	}

	if err := worker.Dispatch(name, func(_ uint32, ctx context.Context) error {
		return fn(ctx, cfgp)
	}, "Feeds"); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, StrStarted)
}

// dispatchConfigFuncAll dispatches fn once per config of one media type, with
// the config name appended to name so the dispatches don't collide.
func dispatchConfigFuncAll(
	c *gin.Context,
	prefix, typeName, name string,
	fn func(context.Context, *config.MediaTypeConfig) error,
) {
	var (
		foundConfigs, started int
		firstErr              error
	)

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, prefix+"_") {
			return nil
		}

		foundConfigs++

		cfgp := media
		if err := worker.Dispatch(name+"_"+cfgp.NamePrefix, func(_ uint32, ctx context.Context) error {
			return fn(ctx, cfgp)
		}, "Feeds"); err != nil {
			if firstErr == nil {
				firstErr = err
			}

			return nil
		}

		started++

		return nil
	})

	if foundConfigs == 0 {
		sendNotFound(c, "No "+typeName+" configuration found")
		return
	}

	sendDispatchResult(c, StrStarted, started, firstErr)
}

// parseUintParam parses the uint path parameter name; writes a 400 and
// returns false when it is missing or not a number.
func parseUintParam(c *gin.Context, name string) (uint, bool) {
	v, err := strconv.ParseUint(c.Param(name), 10, 32)
	if err != nil {
		sendBadRequest(c, "Invalid "+name)
		return 0, false
	}

	return uint(v), true
}

// dispatchClearHistory dispatches the clearhistory job for name, which is
// either a media config name (all its lists) or a list name (that list only).
func dispatchClearHistory(c *gin.Context, prefix, typeName, name string) {
	var listname string

	media := findMediaConfigByName(prefix, name)
	if media == nil {
		media, listname = findMediaConfigByList(prefix, name)
	}

	if media == nil {
		sendNotFound(c, typeName+" configuration or list "+name+" not found")
		return
	}

	cfgpstr := media.NamePrefix

	jobname := logger.StrClearHistory + "_" + cfgpstr
	if listname != "" {
		jobname += "_" + listname
	}

	if err := worker.Dispatch(jobname, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, logger.StrClearHistory, cfgpstr, listname, true, key)
	}, "Data"); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, StrStarted)
}

// dispatchNamedConfigJob dispatches a per-config job for the config named
// name (or, failing that, the config owning the list named name) and writes
// the response.
func dispatchNamedConfigJob(
	c *gin.Context,
	prefix, typeName, name, job, jobnamePrefix, queue, successMsg string,
) {
	media := findMediaConfigByName(prefix, name)
	if media == nil {
		media, _ = findMediaConfigByList(prefix, name)
	}

	if media == nil {
		sendNotFound(c, typeName+" configuration or list "+name+" not found")
		return
	}

	cfgpstr := media.NamePrefix
	if err := worker.Dispatch(jobnamePrefix+cfgpstr, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, job, cfgpstr, "", true, key)
	}, queue); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, successMsg)
}

// mediaJobQueue returns the worker queue a job is dispatched to.
func mediaJobQueue(job string) string {
	switch job {
	case logger.StrSearchMissingFull,
		logger.StrSearchMissingInc,
		logger.StrSearchUpgradeFull,
		logger.StrSearchUpgradeInc,
		logger.StrSearchMissingFullTitle,
		logger.StrSearchMissingIncTitle,
		logger.StrSearchUpgradeFullTitle,
		logger.StrSearchUpgradeIncTitle:
		return "Search"
	case logger.StrRss, logger.StrRssSeasons, logger.StrRssSeasonsAll:
		return "RSS"
	case logger.StrFeeds, "refresh", "refreshinc":
		return "Feeds"
	default:
		return "Data"
	}
}

// dispatchMediaJobsForConfig dispatches one job for one media config. Per-list
// jobs (feeds, checkmissing, checkmissingflag, checkreachedflag) are dispatched
// once per enabled list. The job name always contains the config (and list)
// so that the same operation on different configs/lists is not rejected by
// worker.Dispatch's duplicate detection. Only plain strings are captured by
// the job closures - never the request's gin.Context, which gin recycles once
// the handler returns.
func dispatchMediaJobsForConfig(
	job, typeName string,
	media *config.MediaTypeConfig,
) (int, error) {
	cfgpstr := media.NamePrefix
	queue := mediaJobQueue(job)

	switch job {
	case "":
		return 0, nil

	case logger.StrFeeds,
		logger.StrCheckMissing,
		logger.StrCheckMissingFlag,
		logger.StrReachedFlag:
		var (
			started  int
			firstErr error
		)

		for idxi := range media.Lists {
			if !media.Lists[idxi].Enabled || media.Lists[idxi].CfgList == nil ||
				!media.Lists[idxi].CfgList.Enabled {
				logger.Logtype("debug", 1).
					Str("list", media.Lists[idxi].Name).
					Msg("Skipping disabled " + typeName + " list")

				continue
			}

			listname := media.Lists[idxi].Name

			logger.Logtype("debug", 1).
				Str("job", job).
				Str("list", listname).
				Str("queue", queue).
				Msg("Dispatching " + typeName + " job")

			if err := worker.Dispatch(
				job+"_"+cfgpstr+"_"+listname,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, job, cfgpstr, listname, true, key)
				},
				queue,
			); err != nil {
				if firstErr == nil {
					firstErr = err
				}

				continue
			}

			started++
		}

		if started == 0 && firstErr == nil {
			logger.Logtype("warn", 1).
				Str("job", job).
				Str("media", cfgpstr).
				Int("total_lists", len(media.Lists)).
				Msg("No enabled " + typeName + " lists found to dispatch job")
		}

		return started, firstErr

	default:
		// refresh/refreshinc run unforced so they honour SchedulerDisabled, as before.
		force := job != "refresh" && job != "refreshinc"
		if err := worker.Dispatch(
			job+"_"+cfgpstr,
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, job, cfgpstr, "", force, key)
			},
			queue,
		); err != nil {
			return 0, err
		}

		return 1, nil
	}
}

// sendDispatchResult writes the response for a job endpoint after dispatching.
func sendDispatchResult(c *gin.Context, successMsg string, started int, err error) {
	switch {
	case started > 0:
		sendSuccess(c, successMsg)
	case err != nil:
		sendDispatchError(c, err)
	default:
		sendSuccess(c, StrNothingDone)
	}
}

// dispatchJobAllConfigs dispatches job for every config of one media type
// (prefix without the trailing underscore) and writes the response.
func dispatchJobAllConfigs(c *gin.Context, prefix, typeName, job, successMsg string) {
	var (
		foundConfigs, started int
		firstErr              error
	)

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, prefix+"_") {
			return nil
		}

		foundConfigs++

		logger.Logtype("debug", 2).
			Str("job", job).
			Str("media", media.NamePrefix).
			Int("lists", len(media.Lists)).
			Msg("Processing " + typeName + " media config")

		n, err := dispatchMediaJobsForConfig(job, typeName, media)

		started += n
		if err != nil && firstErr == nil {
			firstErr = err
		}

		return nil
	})

	if foundConfigs == 0 {
		logger.Logtype("warn", 1).
			Str("job", job).
			Msg("No " + typeName + " media configurations found")
		sendNotFound(c, "No "+typeName+" media configurations found")

		return
	}

	sendDispatchResult(c, successMsg, started, firstErr)
}

// dispatchMediaAllJobs implements the shared "start a job for every
// configuration of one media type" endpoint body.
func dispatchMediaAllJobs(c *gin.Context, mt mediaJobDispatchConfig) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, mt.AllowedJobs) {
		sendJSONError(c, http.StatusBadRequest, "Job "+jobParam+" not allowed!")
		return
	}

	dispatchJobAllConfigs(c, mt.NamePrefix, mt.TypeName, jobParam, "Job "+jobParam+" started")
}

// dispatchMediaJob implements the shared "start a job for one named
// configuration" endpoint body.
func dispatchMediaJob(c *gin.Context, mt mediaJobDispatchConfig) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, mt.AllowedJobs) {
		sendJSONError(c, http.StatusBadRequest, "Job "+jobParam+" not allowed!")
		return
	}

	name := c.Param(StrName)

	media := findMediaConfigByName(mt.NamePrefix, name)
	if media == nil {
		sendNotFound(c, mt.TypeName+" configuration "+name+" not found")
		return
	}

	started, err := dispatchMediaJobsForConfig(jobParam, mt.TypeName, media)
	sendDispatchResult(c, "Job "+jobParam+" started", started, err)
}

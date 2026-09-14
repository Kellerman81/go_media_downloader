package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

// mediaJobDispatchConfig holds the per-media-type parameters that vary across
// the otherwise-identical books/audiobooks/music job-dispatch endpoints
// (apiBooksAllJobs/apiBooksJobs, apiAudiobooksAllJobs/apiAudiobooksJobs,
// apiMusicAllJobs/apiMusicJobs).
type mediaJobDispatchConfig struct {
	// TypeName is the singular media type name used in log messages,
	// e.g. "book", "audiobook", "music".
	TypeName string
	// NamePrefix is both the value checked via
	// strings.HasPrefix(media.NamePrefix, NamePrefix) to select which
	// configs belong to this media type, and the prefix used to build a
	// single config's cfgp string (e.g. logger.StrBook, logger.StrAudiobook,
	// "music").
	NamePrefix string
	// Plural is used to build worker job-dispatch keys and the
	// refresh/refreshinc job names, e.g. "books", "audiobooks", "music".
	Plural string
	// AllowedJobs is the comma-separated allowlist passed to validateJobParam.
	AllowedJobs string
}

// dispatchMediaAllJobs implements the shared "start a job for every
// configuration of one media type" endpoint body used by
// apiBooksAllJobs, apiAudiobooksAllJobs and apiMusicAllJobs.
func dispatchMediaAllJobs(c *gin.Context, mt mediaJobDispatchConfig) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, mt.AllowedJobs) {
		sendJSONError(c, http.StatusBadRequest, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	foundConfigs := 0

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, mt.NamePrefix) {
			return nil
		}

		foundConfigs++

		logger.Logtype("debug", 2).
			Str("job", jobParam).
			Str("media", media.NamePrefix).
			Int("lists", len(media.Lists)).
			Msg("Processing " + mt.TypeName + " media config")

		cfgpstr := media.NamePrefix

		switch c.Param(StrJobLower) {
		case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
			worker.Dispatch(
				c.Param(StrJobLower)+"_"+cfgpstr,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
				},
				"Data",
			)

		case logger.StrSearchMissingFull,
			logger.StrSearchMissingInc,
			logger.StrSearchUpgradeFull,
			logger.StrSearchUpgradeInc,
			logger.StrSearchMissingFullTitle,
			logger.StrSearchMissingIncTitle,
			logger.StrSearchUpgradeFullTitle,
			logger.StrSearchUpgradeIncTitle:
			worker.Dispatch(
				c.Param(StrJobLower)+"_"+cfgpstr,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
				},
				"Search",
			)

		case logger.StrRss:
			worker.Dispatch(
				c.Param(StrJobLower)+"_"+cfgpstr,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
				},
				"RSS",
			)

		case logger.StrFeeds,
			logger.StrCheckMissing,
			logger.StrCheckMissingFlag,
			logger.StrReachedFlag:
			var err error

			dispatchedLists := 0
			for idxi := range media.Lists {
				if !media.Lists[idxi].Enabled {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Msg("Skipping disabled " + mt.TypeName + " list")
					continue
				}

				if media.Lists[idxi].CfgList == nil {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Msg("Skipping " + mt.TypeName + " list with nil CfgList")
					continue
				}

				if !config.GetSettingsList(media.Lists[idxi].TemplateList).Enabled {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Str("template", media.Lists[idxi].TemplateList).
						Msg("Skipping " + mt.TypeName + " list with disabled template")

					continue
				}

				listname := media.Lists[idxi].Name

				queueName := "Data"
				if c.Param(StrJobLower) == logger.StrFeeds {
					queueName = "Feeds"
				}

				logger.Logtype("debug", 2).
					Str("job", c.Param(StrJobLower)).
					Str("list", listname).
					Str("queue", queueName).
					Msg("Dispatching " + mt.TypeName + " job")

				dispatchedLists++

				if errsub := worker.Dispatch(
					c.Param(StrJobLower)+"_"+cfgpstr+"_"+listname,
					func(key uint32, ctx context.Context) error {
						return utils.SingleJobs(
							ctx,
							c.Param(StrJobLower),
							cfgpstr,
							listname,
							true,
							key,
						)
					},
					queueName,
				); errsub != nil {
					err = errsub
				}
			}

			if dispatchedLists == 0 {
				logger.Logtype("warn", 1).
					Str("job", c.Param(StrJobLower)).
					Str("media", media.NamePrefix).
					Int("total_lists", len(media.Lists)).
					Msg("No enabled " + mt.TypeName + " lists found to dispatch job")
			}

			return err

		case "refresh":
			return worker.Dispatch(
				"refresh_"+mt.Plural,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
				},
				"Feeds",
			)

		case "refreshinc":
			return worker.Dispatch(
				"refreshinc_"+mt.Plural,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, "refreshinc", cfgpstr, "", false, key)
				},
				"Feeds",
			)

		case "":
			return nil
		default:
			return worker.Dispatch(
				c.Param(StrJobLower)+"_"+cfgpstr,
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
				},
				"Data",
			)
		}

		return nil
	})

	if foundConfigs == 0 {
		logger.Logtype("warn", 1).
			Str("job", jobParam).
			Msg("No " + mt.TypeName + " media configurations found")
	}

	sendSuccess(c, returnval)
}

// dispatchMediaJob implements the shared "start a job for one named
// configuration" endpoint body used by apiBooksJobs, apiAudiobooksJobs and
// apiMusicJobs.
func dispatchMediaJob(c *gin.Context, mt mediaJobDispatchConfig) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, mt.AllowedJobs) {
		sendJSONError(c, http.StatusBadRequest, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	cfgpstr := mt.NamePrefix + "_" + c.Param("name")

	switch c.Param(StrJobLower) {
	case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
		worker.Dispatch(
			c.Param(StrJobLower)+"_"+mt.Plural+"_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Data",
		)

	case logger.StrSearchMissingFull,
		logger.StrSearchMissingInc,
		logger.StrSearchUpgradeFull,
		logger.StrSearchUpgradeInc,
		logger.StrSearchMissingFullTitle,
		logger.StrSearchMissingIncTitle,
		logger.StrSearchUpgradeFullTitle,
		logger.StrSearchUpgradeIncTitle:
		worker.Dispatch(
			c.Param(StrJobLower)+"_"+mt.Plural+"_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Search",
		)

	case logger.StrRss:
		worker.Dispatch(
			c.Param(StrJobLower)+"_"+mt.Plural+"_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"RSS",
		)

	case logger.StrFeeds,
		logger.StrCheckMissing,
		logger.StrCheckMissingFlag,
		logger.StrReachedFlag:
		config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
			if !strings.HasPrefix(media.NamePrefix, mt.NamePrefix) {
				return nil
			}

			if strings.EqualFold(media.Name, c.Param("name")) {
				for idxlist := range media.Lists {
					if !media.Lists[idxlist].Enabled {
						continue
					}

					if media.Lists[idxlist].CfgList == nil {
						continue
					}

					if !config.GetSettingsList(media.Lists[idxlist].TemplateList).Enabled {
						continue
					}

					listname := media.Lists[idxlist].Name
					if c.Param(StrJobLower) == logger.StrFeeds {
						worker.Dispatch(
							c.Param(
								StrJobLower,
							)+"_"+mt.Plural+"_"+media.Name+"_"+media.Lists[idxlist].Name,
							func(key uint32, ctx context.Context) error {
								return utils.SingleJobs(
									ctx,
									c.Param(StrJobLower),
									cfgpstr,
									listname,
									true,
									key,
								)
							},
							"Feeds",
						)
					}

					if c.Param(StrJobLower) == logger.StrCheckMissing ||
						c.Param(StrJobLower) == logger.StrCheckMissingFlag ||
						c.Param(StrJobLower) == logger.StrReachedFlag {
						worker.Dispatch(
							c.Param(
								StrJobLower,
							)+"_"+mt.Plural+"_"+media.Name+"_"+media.Lists[idxlist].Name,
							func(key uint32, ctx context.Context) error {
								return utils.SingleJobs(
									ctx,
									c.Param(StrJobLower),
									cfgpstr,
									listname,
									true,
									key,
								)
							},
							"Data",
						)
					}
				}
			}

			return nil
		})

	case "refresh":
		worker.Dispatch(
			"refresh_"+mt.Plural+"_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "refreshinc":
		worker.Dispatch(
			"refreshinc_"+mt.Plural+"_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refreshinc", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "":
		break
	default:
		worker.Dispatch(
			c.Param(StrJobLower)+"_"+mt.Plural+"_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Data",
		)
	}

	sendSuccess(c, returnval)
}

package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

// allowed jobs for audiobooks.
const allowedjobsaudiobooksstr = "rss,rssauthors,rssauthorsupgrade,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// AddAudiobooksRoutes adds routes for audiobook management.
func AddAudiobooksRoutes(routeraudiobooks *gin.RouterGroup) {
	routeraudiobooks.Use(checkauth)
	{
		routeraudiobooks.GET("/all/refresh", apirefreshAudiobooksInc)
		routeraudiobooks.GET("/all/refreshall", apirefreshAudiobooks)
		routeraudiobooks.GET("/refresh/:id", apirefreshAudiobook)

		routeraudiobooks.GET("/tag/all", apiRetagAllAudiobooks)
		routeraudiobooks.GET("/tag/author/:id", apiRetagAuthorAudiobooks)
		routeraudiobooks.GET("/tag/:id", apiRetagAudiobook)

		routeraudiobooks.GET("/", apiAudiobooksDBList)
		routeraudiobooks.GET("/list/:name", apiAudiobooksListGet)
		routeraudiobooks.DELETE("/:id", apiAudiobookDelete)

		routeraudiobooks.GET("/job/:job", apiAudiobooksAllJobs)
		routeraudiobooks.GET("/job/:job/:name", apiAudiobooksJobs)

		routeraudiobooks.GET("/author/add/:name/:listname", apiAudiobooksAddAuthor)
		routeraudiobooks.GET("/rss/search/list/:group", apiAudiobooksRssSearchList)

		routeraudiobookssearch := routeraudiobooks.Group("/search")
		{
			routeraudiobookssearch.GET("/list/:id", apiAudiobooksSearchList)
			routeraudiobookssearch.GET("/history/clear/:name", apiAudiobooksClearHistoryName)
			routeraudiobookssearch.GET("/history/clearid/:id", apiAudiobooksClearHistoryID)
			routeraudiobookssearch.GET("/authors/missing/:name", apiAudiobooksSearchAuthorsMissing)
			routeraudiobookssearch.GET("/authors/upgrade/:name", apiAudiobooksSearchAuthorsUpgrade)
		}
	}
}

// @Summary      List Audiobooks (Database)
// @Description  List all audiobooks from the database
// @Tags         audiobook
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks [get].
func apiAudiobooksDBList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from dbaudiobooks")

	query := "select id,created_at,updated_at,title,asin,audible_id,runtime_minutes,chapter_count,release_date,publisher,language,abridged,cover_url,sample_url,average_rating,ratings_count,year,slug,dbbook_id,description from dbaudiobooks"
	if params.Order != "" {
		query += " order by " + params.Order
	}

	if params.Limit > 0 {
		query += " limit " + strconv.Itoa(
			int(params.Limit), //nolint:gosec // safe: value within target type range
		) + " offset " + strconv.Itoa(
			int(params.Offset), //nolint:gosec // safe: value within target type range
		)
	}

	data := database.StructscanT[database.Dbaudiobook](false, params.Limit, query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      List Audiobooks by List Name
// @Description  List audiobooks filtered by list name
// @Tags         audiobook
// @Param        name   path      string  true   "List Name"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/list/{name} [get].
func apiAudiobooksListGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](
		false,
		"select count() from audiobooks where listname = ?",
		&name,
	)

	query := "select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbaudiobook_id,author_id,book_series_id from audiobooks where listname = ?"
	if params.Order != "" {
		query += " order by " + params.Order
	}

	if params.Limit > 0 {
		query += " limit " + strconv.Itoa(
			int(params.Limit), //nolint:gosec // safe: value within target type range
		) + " offset " + strconv.Itoa(
			int(params.Offset), //nolint:gosec // safe: value within target type range
		)
	}

	data := database.StructscanT[database.Audiobook](false, params.Limit, query, &name)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Delete Audiobook
// @Description  Deletes an audiobook from the database
// @Tags         audiobook
// @Param        id     path      int     true   "Audiobook ID"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  gin.H
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/{id} [delete].
func apiAudiobookDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Delete audiobook files first
	database.ExecN("DELETE FROM audiobook_files WHERE audiobook_id = ?", &id)
	// Delete the audiobook
	database.ExecN("DELETE FROM audiobooks WHERE id = ?", &id)

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// @Summary      Start All Audiobook Jobs
// @Description  Starts a Job for all audiobook configurations
// @Tags         audiobook
// @Param        job    path      string  true   "Job Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string  "returns job name started"
// @Failure      204    {object}  string  "error message"
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/job/{job} [get].
func apiAudiobooksAllJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsaudiobooksstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	foundConfigs := 0

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			return nil
		}

		foundConfigs++

		logger.Logtype("debug", 2).
			Str("job", jobParam).
			Str("media", media.NamePrefix).
			Int("lists", len(media.Lists)).
			Msg("Processing audiobook media config")

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
						Msg("Skipping disabled audiobook list")
					continue
				}

				if media.Lists[idxi].CfgList == nil {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Msg("Skipping audiobook list with nil CfgList")
					continue
				}

				if !config.GetSettingsList(media.Lists[idxi].TemplateList).Enabled {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Str("template", media.Lists[idxi].TemplateList).
						Msg("Skipping audiobook list with disabled template")

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
					Msg("Dispatching audiobook job")

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
					Msg("No enabled audiobook lists found to dispatch job")
			}

			return err

		case "refresh":
			return worker.Dispatch(
				"refresh_audiobooks",
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
				},
				"Feeds",
			)

		case "refreshinc":
			return worker.Dispatch(
				"refreshinc_audiobooks",
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
			Msg("No audiobook media configurations found")
	}

	sendSuccess(c, returnval)
}

// @Summary      Start Audiobook Jobs
// @Description  Starts a Job for a specific audiobook configuration
// @Tags         audiobook
// @Param        job    path      string  true   "Job Name"
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string  "returns job name started"
// @Failure      204    {object}  string  "error message"
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/job/{job}/{name} [get].
func apiAudiobooksJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsaudiobooksstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	cfgpstr := "audiobook_" + c.Param("name")

	switch c.Param(StrJobLower) {
	case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
		worker.Dispatch(
			c.Param(StrJobLower)+"_audiobooks_"+c.Param("name"),
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
			c.Param(StrJobLower)+"_audiobooks_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Search",
		)

	case logger.StrRss:
		worker.Dispatch(
			c.Param(StrJobLower)+"_audiobooks_"+c.Param("name"),
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
			if !strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
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
							)+"_audiobooks_"+media.Name+"_"+media.Lists[idxlist].Name,
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
							)+"_audiobooks_"+media.Name+"_"+media.Lists[idxlist].Name,
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
			"refresh_audiobooks_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "refreshinc":
		worker.Dispatch(
			"refreshinc_audiobooks_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refreshinc", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "":
		break
	default:
		worker.Dispatch(
			c.Param(StrJobLower)+"_audiobooks_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Data",
		)
	}

	sendSuccess(c, returnval)
}

// @Summary      RSS Search Audiobooks List
// @Description  Trigger RSS search for audiobook list
// @Tags         audiobook
// @Param        group  path      string  true   "Group Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/rss/search/list/{group} [get].
func apiAudiobooksRssSearchList(c *gin.Context) {
	cfgpstr := "audiobook_" + c.Param("group")
	worker.Dispatch(
		"rss_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRss, cfgpstr, "", true, key)
		},
		"RSS",
	)
	sendSuccess(c, "RSS search started for "+cfgpstr)
}

// @Summary      Search Audiobooks by List
// @Description  Search for all audiobooks in a list
// @Tags         audiobook
// @Param        id     path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/search/list/{id} [get].
func apiAudiobooksSearchList(c *gin.Context) {
	listName := c.Param(StrID)
	cfgpstr := "audiobook_" + listName
	worker.Dispatch(
		"searchlist_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrSearchMissingInc, cfgpstr, "", true, key)
		},
		"Search",
	)
	sendSuccess(c, "Search started for audiobook list "+listName)
}

// @Summary      Clear Audiobook History by Name
// @Description  Clear search history for audiobook list
// @Tags         audiobook
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/search/history/clear/{name} [get].
func apiAudiobooksClearHistoryName(c *gin.Context) {
	name := c.Param("name")
	database.ExecN("DELETE FROM audiobook_histories WHERE listname = ?", &name)
	sendSuccess(c, "History cleared for "+name)
}

// @Summary      Clear Audiobook History by ID
// @Description  Clear search history entry by ID
// @Tags         audiobook
// @Param        id     path      int     true   "History ID"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/search/history/clearid/{id} [get].
func apiAudiobooksClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	database.ExecN("DELETE FROM audiobook_histories WHERE id = ?", &id)
	sendSuccess(c, "History entry cleared")
}

// @Summary      Search Missing Audiobooks by Author
// @Description  Search for missing audiobooks by author name (up to 20 random authors)
// @Tags         audiobook
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/search/authors/missing/{name} [get].
func apiAudiobooksSearchAuthorsMissing(c *gin.Context) {
	listName := c.Param("name")
	cfgpstr := "audiobook_" + listName
	worker.Dispatch(
		"searchauthors_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRssAuthors, cfgpstr, "", true, key)
		},
		"Author Search",
	)
	sendSuccess(c, "Author search (missing) started for audiobook list "+listName)
}

// @Summary      Search Upgrade Audiobooks by Author
// @Description  Search for audiobooks needing quality upgrade by author name (up to 20 random authors)
// @Tags         audiobook
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/audiobooks/search/authors/upgrade/{name} [get].
func apiAudiobooksSearchAuthorsUpgrade(c *gin.Context) {
	listName := c.Param("name")
	cfgpstr := "audiobook_" + listName
	worker.Dispatch(
		"searchauthorsupgrade_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRssAuthorsUpgrade, cfgpstr, "", true, key)
		},
		"Author Search",
	)
	sendSuccess(c, "Author search (upgrade) started for audiobook list "+listName)
}

// @Summary      Refresh Audiobooks
// @Description  Refreshes Audiobook Metadata
// @Tags         audiobook
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/audiobooks/all/refreshall [get].
func apirefreshAudiobooks(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			cfgp = media
			return true
		}

		return false
	})
	worker.Dispatch(logger.StrRefreshAudiobooks, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, "refresh", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Refresh a single Audiobook
// @Description  Refreshes specific Audiobook Metadata
// @Tags         audiobook
// @Param        id   path      int  true  "Audiobook ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/audiobooks/refresh/{id} [get].
func apirefreshAudiobook(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			cfgp = media
			return true
		}

		return false
	})

	id := c.Param(logger.StrID)
	worker.Dispatch(
		"Refresh Single Audiobook_"+id,
		func(_ uint32, _ context.Context) error {
			return utils.RefreshAudiobook(cfgp, &id)
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

// @Summary      Refresh Audiobooks (Incremental)
// @Description  Refreshes Audiobook Metadata
// @Tags         audiobook
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/audiobooks/all/refresh [get].
func apiRetagAudiobook(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			cfgp = media
			return true
		}

		return false
	})

	id := c.Param(logger.StrID)
	worker.Dispatch(
		"Retag Audiobook_"+id,
		func(_ uint32, _ context.Context) error {
			dbID, _ := strconv.ParseUint(id, 10, 32)
			return utils.RetagAudiobook(c, cfgp, uint(dbID))
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

func apiRetagAuthorAudiobooks(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			cfgp = media
			return true
		}

		return false
	})

	id := c.Param(logger.StrID)
	worker.Dispatch(
		"Retag Author Audiobooks_"+id,
		func(_ uint32, _ context.Context) error {
			authorID, _ := strconv.ParseUint(id, 10, 32)
			return utils.RetagAuthorAudiobooks(c, cfgp, uint(authorID))
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

func apiRetagAllAudiobooks(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			cfgp = media
			return true
		}

		return false
	})

	worker.Dispatch(
		"Retag All Audiobooks",
		func(_ uint32, _ context.Context) error {
			return utils.RetagAllAudiobooks(c, cfgp)
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

func apirefreshAudiobooksInc(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, logger.StrAudiobook) {
			cfgp = media
			return true
		}

		return false
	})
	worker.Dispatch(logger.StrRefreshAudiobooksInc, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, "refreshinc", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Add Audiobook Author and Import Audiobooks
// @Description  Adds an Audnex author (by Audible ASIN) to the given list and imports all their audiobooks in the background.
// @Tags         audiobook
// @Param        name      path   string  true  "Media config name (without 'audiobooks_' prefix)"
// @Param        listname  path   string  true  "List name within the media config"
// @Param        author_id query  string  true  "Audnex/Audible author ASIN"
// @Param        apikey    query  string  true  "apikey"
// @Success      200  {object}  string
// @Failure      400  {object}  string
// @Failure      401  {object}  Jsonerror
// @Router       /api/audiobooks/author/add/{name}/{listname} [get].
func apiAudiobooksAddAuthor(c *gin.Context) {
	mediaName := c.Param(StrName)
	listName := c.Param("listname")
	authorID := c.Query("author_id")

	if authorID == "" {
		sendBadRequest(c, "author_id query parameter is required")
		return
	}

	cfgpstr := "audiobooks_" + mediaName

	cfgp := config.GetSettingsMedia(cfgpstr)
	if cfgp == nil {
		sendBadRequest(c, "audiobooks config not found: "+cfgpstr)
		return
	}

	listid, ok := cfgp.ListsMapIdx[listName]
	if !ok {
		sendBadRequest(c, "list "+listName+" not found in "+cfgpstr)
		return
	}

	authorIDCopy := authorID
	worker.Dispatch(
		"add_author_"+authorID+"_"+cfgpstr,
		func(_ uint32, ctx context.Context) error {
			// Import all audiobooks by this author into dbaudiobooks and audiobooks.
			// CheckaddAudiobookEntry (called internally) creates the authors tracking entry.
			return importfeed.JobImportDBAudiobook(ctx, &config.ManualConfig{
				AuthorID: authorIDCopy,
			}, 0, cfgp, listid)
		},
		"Data",
	)

	sendSuccess(c, "Author "+authorID+" queued for import into "+cfgpstr+"/"+listName)
}

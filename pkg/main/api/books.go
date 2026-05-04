package api

import (
	"context"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	gin "github.com/gin-gonic/gin"
)

// allowed jobs for books.
const allowedjobsbooksstr = "rss,rssauthors,rssauthorsupgrade,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// AddBooksRoutes adds routes for book management.
func AddBooksRoutes(routerbooks *gin.RouterGroup) {
	routerbooks.Use(checkauth)
	{
		routerbooks.GET("/", apiBooksDBList)
		routerbooks.GET("/list/:name", apiBooksListGet)
		routerbooks.DELETE("/:id", apiBookDelete)

		routerbooks.GET("/job/:job", apiBooksAllJobs)
		routerbooks.GET("/job/:job/:name", apiBooksJobs)

		routerbooks.GET("/feeds/date/:name/:listname", apiBooksFeedsDate)

		routerbooks.GET("/rss/search/list/:group", apiBooksRssSearchList)

		routerbookssearch := routerbooks.Group("/search")
		{
			routerbookssearch.GET("/list/:id", apiBooksSearchList)
			routerbookssearch.GET("/history/clear/:name", apiBooksClearHistoryName)
			routerbookssearch.GET("/history/clearid/:id", apiBooksClearHistoryID)
			routerbookssearch.GET("/authors/missing/:name", apiBooksSearchAuthorsMissing)
			routerbookssearch.GET("/authors/upgrade/:name", apiBooksSearchAuthorsUpgrade)
		}
	}
}

// @Summary      List Books (Database)
// @Description  List all books from the database
// @Tags         book
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/books [get].
func apiBooksDBList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from dbbooks")

	query := "select id,created_at,updated_at,title,original_title,isbn_13,isbn_10,asin,openlibrary_id,goodreads_id,description,publisher,publish_date,page_count,language,genres,cover_url,dbauthor_id,dbbook_series_id,series_position,average_rating,ratings_count,year,slug from dbbooks"
	if params.Order != "" {
		query += " order by " + params.Order
	}

	if params.Limit > 0 {
		query += " limit " + itoa(
			int(params.Limit),
		) + " offset " + itoa(
			int(params.Offset),
		)
	}

	data := database.StructscanT[database.Dbbook](false, params.Limit, query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      List Books by List Name
// @Description  List books filtered by list name
// @Tags         book
// @Param        name   path      string  true   "List Name"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/list/{name} [get].
func apiBooksListGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from books where listname = ?", &name)

	query := "select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbbook_id,book_series_id,author_id from books where listname = ?"
	if params.Order != "" {
		query += " order by " + params.Order
	}

	if params.Limit > 0 {
		query += " limit " + itoa(
			int(params.Limit),
		) + " offset " + itoa(
			int(params.Offset),
		)
	}

	data := database.StructscanT[database.Book](false, params.Limit, query, &name)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Delete Book
// @Description  Deletes a book from the database
// @Tags         book
// @Param        id     path      int     true   "Book ID"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  gin.H
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/{id} [delete].
func apiBookDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Delete book files first
	database.ExecN("DELETE FROM book_files WHERE book_id = ?", &id)
	// Delete the book
	database.ExecN("DELETE FROM books WHERE id = ?", &id)

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// itoa converts an integer to a string.
func itoa(i int) string {
	return strconv.Itoa(i)
}

// @Summary      Start All Book Jobs
// @Description  Starts a Job for all book configurations
// @Tags         book
// @Param        job    path      string  true   "Job Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string  "returns job name started"
// @Failure      204    {object}  string  "error message"
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/job/{job} [get].
func apiBooksAllJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsbooksstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	foundConfigs := 0

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, logger.StrBook) {
			return nil
		}

		foundConfigs++

		logger.Logtype("debug", 2).
			Str("job", jobParam).
			Str("media", media.NamePrefix).
			Int("lists", len(media.Lists)).
			Msg("Processing book media config")

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
						Msg("Skipping disabled book list")
					continue
				}

				if media.Lists[idxi].CfgList == nil {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Msg("Skipping book list with nil CfgList")
					continue
				}

				if !config.GetSettingsList(media.Lists[idxi].TemplateList).Enabled {
					logger.Logtype("debug", 2).
						Str("list", media.Lists[idxi].Name).
						Str("template", media.Lists[idxi].TemplateList).
						Msg("Skipping book list with disabled template")

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
					Msg("Dispatching book job")

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
					Msg("No enabled book lists found to dispatch job")
			}

			return err

		case "refresh":
			return worker.Dispatch(
				"refresh_books",
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
				},
				"Feeds",
			)

		case "refreshinc":
			return worker.Dispatch(
				"refreshinc_books",
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
			Msg("No book media configurations found")
	}

	sendSuccess(c, returnval)
}

// @Summary      Start Book Jobs
// @Description  Starts a Job for a specific book configuration
// @Tags         book
// @Param        job    path      string  true   "Job Name"
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string  "returns job name started"
// @Failure      204    {object}  string  "error message"
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/job/{job}/{name} [get].
func apiBooksJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsbooksstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	cfgpstr := "book_" + c.Param("name")

	switch c.Param(StrJobLower) {
	case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
		worker.Dispatch(
			c.Param(StrJobLower)+"_books_"+c.Param("name"),
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
			c.Param(StrJobLower)+"_books_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Search",
		)

	case logger.StrRss:
		worker.Dispatch(
			c.Param(StrJobLower)+"_books_"+c.Param("name"),
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
			if !strings.HasPrefix(media.NamePrefix, logger.StrBook) {
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
							c.Param(StrJobLower)+"_books_"+media.Name+"_"+media.Lists[idxlist].Name,
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
							c.Param(StrJobLower)+"_books_"+media.Name+"_"+media.Lists[idxlist].Name,
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
			"refresh_books_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "refreshinc":
		worker.Dispatch(
			"refreshinc_books_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refreshinc", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "":
		break
	default:
		worker.Dispatch(
			c.Param(StrJobLower)+"_books_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Data",
		)
	}

	sendSuccess(c, returnval)
}

// @Summary      RSS Search Books List
// @Description  Trigger RSS search for book list
// @Tags         book
// @Param        group  path      string  true   "Group Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/rss/search/list/{group} [get].
func apiBooksRssSearchList(c *gin.Context) {
	cfgpstr := "book_" + c.Param("group")
	worker.Dispatch(
		"rss_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRss, cfgpstr, "", true, key)
		},
		"RSS",
	)
	sendSuccess(c, "RSS search started for "+cfgpstr)
}

// @Summary      Search Books by List
// @Description  Search for all books in a list
// @Tags         book
// @Param        id     path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/search/list/{id} [get].
func apiBooksSearchList(c *gin.Context) {
	listName := c.Param(StrID)
	cfgpstr := "book_" + listName
	worker.Dispatch(
		"searchlist_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrSearchMissingInc, cfgpstr, "", true, key)
		},
		"Search",
	)
	sendSuccess(c, "Search started for book list "+listName)
}

// @Summary      Clear Book History by Name
// @Description  Clear search history for book list
// @Tags         book
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/search/history/clear/{name} [get].
func apiBooksClearHistoryName(c *gin.Context) {
	name := c.Param("name")
	database.ExecN("DELETE FROM book_histories WHERE listname = ?", &name)
	sendSuccess(c, "History cleared for "+name)
}

// @Summary      Clear Book History by ID
// @Description  Clear search history entry by ID
// @Tags         book
// @Param        id     path      int     true   "History ID"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/search/history/clearid/{id} [get].
func apiBooksClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	database.ExecN("DELETE FROM book_histories WHERE id = ?", &id)
	sendSuccess(c, "History entry cleared")
}

// @Summary      Search Missing Books by Author
// @Description  Search for missing books by author name (up to 20 random authors)
// @Tags         book
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/search/authors/missing/{name} [get].
func apiBooksSearchAuthorsMissing(c *gin.Context) {
	listName := c.Param("name")
	cfgpstr := "book_" + listName
	worker.Dispatch(
		"searchauthors_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRssAuthors, cfgpstr, "", true, key)
		},
		"Author Search",
	)
	sendSuccess(c, "Author search (missing) started for book list "+listName)
}

// @Summary      Search Upgrade Books by Author
// @Description  Search for books needing quality upgrade by author name (up to 20 random authors)
// @Tags         book
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/books/search/authors/upgrade/{name} [get].
func apiBooksSearchAuthorsUpgrade(c *gin.Context) {
	listName := c.Param("name")
	cfgpstr := "book_" + listName
	worker.Dispatch(
		"searchauthorsupgrade_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRssAuthorsUpgrade, cfgpstr, "", true, key)
		},
		"Author Search",
	)
	sendSuccess(c, "Author search (upgrade) started for book list "+listName)
}

// @Summary      Import Book Bestseller List for a Specific Date
// @Description  Runs the feeds import for a single book bestseller list using a date-specific URL
//
//	built from the list's chart_date_url_pattern and chart_date_format settings.
//
// @Tags         book
// @Param        name      path   string  true  "Media config name (e.g. mybooks)"
// @Param        listname  path   string  true  "List name (e.g. spiegel-hardcover)"
// @Param        date      query  string  true  "Date in YYYY-MM-DD format"
// @Param        apikey    query  string  true  "apikey"
// @Success      200  {object}  string  "returns started"
// @Failure      400  {object}  string  "error message"
// @Failure      401  {object}  Jsonerror
// @Router       /api/books/feeds/date/{name}/{listname} [get].
func apiBooksFeedsDate(c *gin.Context) {
	mediaName := c.Param(StrName)
	listName := c.Param("listname")
	dateStr := c.Query("date")

	if dateStr == "" {
		sendBadRequest(c, "date query parameter is required (YYYY-MM-DD)")
		return
	}

	var dispatched bool

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, "books") {
			return nil
		}

		if !strings.EqualFold(media.Name, mediaName) {
			return nil
		}

		for idxlist := range media.Lists {
			if !strings.EqualFold(media.Lists[idxlist].Name, listName) {
				continue
			}

			if !media.Lists[idxlist].Enabled || media.Lists[idxlist].CfgList == nil {
				sendBadRequest(c, "list "+listName+" is disabled or not configured")
				return nil
			}

			overrideURL, err := buildChartDateURL(media.Lists[idxlist].CfgList, dateStr)
			if err != nil {
				sendBadRequest(c, err.Error())
				return nil
			}

			idxCopy := idxlist
			cfgp := media

			worker.Dispatch(
				"feeds_date_books_"+media.Name+"_"+listName,
				func(_ uint32, ctx context.Context) error {
					return utils.ImportFeedsWithURL(
						ctx,
						cfgp,
						&cfgp.Lists[idxCopy],
						idxCopy,
						overrideURL,
					)
				},
				"Feeds",
			)

			dispatched = true

			return nil
		}

		return nil
	})

	if !dispatched {
		sendBadRequest(c, "list "+listName+" not found in books config "+mediaName)
		return
	}

	sendSuccess(c, "Feeds date import started for "+listName+" ("+dateStr+")")
}

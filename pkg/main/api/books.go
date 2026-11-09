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
	"github.com/gin-gonic/gin"
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

		routerbooks.GET("/refresh/:id", apirefreshBook)

		routerbooks.GET("/job/:job", apiBooksAllJobs)
		routerbooks.GET("/job/:job/:name", apiBooksJobs)

		routerbooks.GET("/feeds/date/:name/:listname", apiBooksFeedsDate)

		routerbooks.GET("/rss/search/list/:group", apiBooksRssSearchList)

		routerbookssearch := routerbooks.Group("/search")
		{
			routerbookssearch.GET("/list/:id", apiBooksSearchList)
			routerbookssearch.GET("/missing/:name", apiBooksSearchMissing)
			routerbookssearch.GET("/history/clear/:name", apiBooksClearHistoryName)
			routerbookssearch.GET("/history/clearid/:id", apiBooksClearHistoryID)
			routerbookssearch.GET("/authors/missing/:name", apiBooksSearchAuthorsMissing)
			routerbookssearch.GET("/authors/upgrade/:name", apiBooksSearchAuthorsUpgrade)
		}
	}
}

// @Summary      List books (metadata)
// @Description  Returns the books of the dbbooks table (metadata independent of lists), paginated, with the total count.
// @Tags         books · data
// @Param        limit  query     int     false  "Rows per page (default 100, max 1000)"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Dbbook}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/books [get].
func apiBooksDBList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	boundPagination(&params)

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

// @Summary      List the books of a list
// @Description  Returns the book entries of one list, paginated, with the total count.
// @Tags         books · data
// @Param        name   path      string  true   "Name of the list (case-insensitive), as configured in the media group"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Book}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/books/list/{name} [get].
func apiBooksListGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from books where listname = ? COLLATE NOCASE", &name)

	query := "select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbbook_id,book_series_id,author_id from books where listname = ? COLLATE NOCASE"
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

// @Summary      Remove a book from a list
// @Description  Deletes one book list entry and its file rows. Files on disk are not touched.
// @Tags         books · data
// @Param        id     path      int     true   "books (list entry) ID"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonsuccess  "Deleted"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      500  {object}  Jsonerror  "Deleting the files or the row failed"
// @Router       /api/books/{id} [delete].
func apiBookDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Delete book files first
	if err := database.ExecNErr("DELETE FROM book_files WHERE book_id = ?", &id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete book files: " + err.Error()})
		return
	}
	// Delete the book
	if err := database.ExecNErr("DELETE FROM books WHERE id = ?", &id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete book: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// itoa converts an integer to a string.
func itoa(i int) string {
	return strconv.Itoa(i)
}

// @Summary      Refresh the metadata of one book
// @Description  Queues a metadata refresh of one book (OpenLibrary, Goodreads). Returns immediately; the work runs in the job queue.
// @Tags         books · jobs
// @Param        id   path      int  true  "ID of the book in the dbbooks table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/refresh/{id} [get].
func apirefreshBook(c *gin.Context) {
	id := c.Param(logger.StrID)
	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM books WHERE dbbook_id = ? LIMIT 1",
		&id,
	)
	dispatchConfigFunc(
		c,
		mediaConfigForList(logger.StrBook, listname),
		logger.StrBook,
		"Refresh Single Book_"+id,
		func(_ context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RefreshBook(cfgp, &id)
		},
	)
}

// @Summary      Start a job for all book groups
// @Description  Queues the given job for every book media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         books · jobs
// @Param        job    path      string  true   "Job to start, one of: rss, rssauthors, rssauthorsupgrade, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/job/{job} [get].
func apiBooksAllJobs(c *gin.Context) {
	dispatchMediaAllJobs(c, mediaJobDispatchConfig{
		TypeName:    "book",
		NamePrefix:  logger.StrBook,
		AllowedJobs: allowedjobsbooksstr,
	})
}

// @Summary      Start a job for one book group
// @Description  Queues the given job for one book media group and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         books · jobs
// @Param        job    path      string  true   "Job to start, one of: rss, rssauthors, rssauthorsupgrade, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name   path      string  true   "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/job/{job}/{name} [get].
func apiBooksJobs(c *gin.Context) {
	dispatchMediaJob(c, mediaJobDispatchConfig{
		TypeName:    "book",
		NamePrefix:  logger.StrBook,
		AllowedJobs: allowedjobsbooksstr,
	})
}

// @Summary      Preview the RSS search of a book group
// @Description  Reads the indexers' RSS feeds for one book group and returns the accepted and denied releases. Nothing is downloaded.
// @Tags         books · search
// @Param        group  path      string  true   "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/books/rss/search/list/{group} [get].
func apiBooksRssSearchList(c *gin.Context) {
	rssPreviewGroup(c, logger.StrBook)
}

// @Summary      Search a book and show the results
// @Description  Searches the indexers for one book list entry and waits for the result: returns the accepted and denied releases with the reasons. download=true also downloads the best accepted release.
// @Tags         books · search
// @Param        id             path      int     true   "books (list entry) ID"
// @Param        searchByTitle  query     string  false  "true: also search by title"
// @Param        download       query     bool    false  "true: download the best accepted release"
// @Param        apikey         query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/books/search/list/{id} [get].
func apiBooksSearchList(c *gin.Context) {
	searchListEntry(c, logger.StrBook, "books")
}

// @Summary      Search missing books of a group
// @Description  Queues an incremental search for the missing books of a book group; accepted releases are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         books · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonsearchstarted  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/search/missing/{name} [get].
func apiBooksSearchMissing(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, listName, logger.StrSearchMissingInc, logger.StrSearchMissingInc+"_", "Search", "Search started for book list "+listName)
}

// @Summary      Clear the download history of a group or list
// @Description  Queues deleting the book download history of a book media group (or of one of its lists). Returns immediately; the work runs in the job queue.
// @Tags         books · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/search/history/clear/{name} [get].
func apiBooksClearHistoryName(c *gin.Context) {
	dispatchClearHistory(c, logger.StrBook, logger.StrBook, c.Param(StrName))
}

// @Summary      Clear the download history of a book
// @Description  Deletes all download history rows of one book list entry, so its releases can be downloaded again. Returns the number of deleted rows.
// @Tags         books · search
// @Param        id     path      int     true   "books (list entry) ID"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "Number of deleted history rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/books/search/history/clearid/{id} [get].
func apiBooksClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	inres, inerr := database.DeleteRow("book_histories", "book_id = ?", id)
	handleDBInsertOrUpdate(c, inres, inerr, false)
}

// @Summary      Search missing books by author
// @Description  Queues indexer searches by author name (up to 20 random authors with missing books) for a book media group; one search can find several books. Accepted releases are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         books · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonauthorsearchmissing  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/search/authors/missing/{name} [get].
func apiBooksSearchAuthorsMissing(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, listName, logger.StrRssAuthors, logger.StrRssAuthors+"_", "Search", "Author search (missing) started for book list "+listName)
}

// @Summary      Search book upgrades by author
// @Description  Queues indexer searches by author name (up to 20 random authors with books below their quality cutoff) for a book media group. Accepted upgrades are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         books · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonauthorsearchupgrade  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/search/authors/upgrade/{name} [get].
func apiBooksSearchAuthorsUpgrade(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, listName, logger.StrRssAuthorsUpgrade, logger.StrRssAuthorsUpgrade+"_", "Search", "Author search (upgrade) started for book list "+listName)
}

// @Summary      Import a bestseller list of a given date
// @Description  Queues the feeds import of one book bestseller list for the given date: the URL is built from the list's chart_date_url_pattern and chart_date_format settings. Returns immediately; the import runs in the job queue.
// @Tags         books · jobs
// @Param        name      path   string  true  "Media config name (e.g. mybooks)"
// @Param        listname  path   string  true  "List name (e.g. spiegel-hardcover)"
// @Param        date      query  string  true  "Date in YYYY-MM-DD format"
// @Param        apikey    query  string  true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonfeedsdate  "'Feeds date import started for <list> (<date>)'"
// @Failure      400  {object}  Jsonbadrequest  "date missing or invalid, or list not found/disabled"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/books/feeds/date/{name}/{listname} [get].
func apiBooksFeedsDate(c *gin.Context) {
	mediaName := c.Param(StrName)
	listName := c.Param("listname")
	dateStr := c.Query("date")

	if dateStr == "" {
		sendBadRequest(c, "date query parameter is required (YYYY-MM-DD)")
		return
	}

	// responded is set by every path inside the closure that already wrote
	// a response, so the trailing writes below never double-write.
	var found, responded bool

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrBook+"_") {
			return nil
		}

		if !strings.EqualFold(media.Name, mediaName) {
			return nil
		}

		for idxlist := range media.Lists {
			if !strings.EqualFold(media.Lists[idxlist].Name, listName) {
				continue
			}

			found = true

			if !media.Lists[idxlist].Enabled || media.Lists[idxlist].CfgList == nil {
				responded = true

				sendBadRequest(c, "list "+listName+" is disabled or not configured")

				return nil
			}

			overrideURL, err := buildChartDateURL(media.Lists[idxlist].CfgList, dateStr)
			if err != nil {
				responded = true

				sendBadRequest(c, err.Error())

				return nil
			}

			idxCopy := idxlist
			cfgp := media

			if err := worker.Dispatch(
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
			); err != nil {
				responded = true

				sendDispatchError(c, err)
			}

			return nil
		}

		return nil
	})

	if responded {
		return
	}

	if !found {
		sendBadRequest(c, "list "+listName+" not found in books config "+mediaName)
		return
	}

	sendSuccess(c, "Feeds date import started for "+listName+" ("+dateStr+")")
}

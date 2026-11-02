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

// @Summary      Refresh a single Book
// @Description  Refreshes specific Book Metadata
// @Tags         book
// @Param        id   path      int  true  "Book ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
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
	dispatchMediaAllJobs(c, mediaJobDispatchConfig{
		TypeName:    "book",
		NamePrefix:  logger.StrBook,
		AllowedJobs: allowedjobsbooksstr,
	})
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
	dispatchMediaJob(c, mediaJobDispatchConfig{
		TypeName:    "book",
		NamePrefix:  logger.StrBook,
		AllowedJobs: allowedjobsbooksstr,
	})
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
	group := c.Param("group")
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, group, logger.StrRss, logger.StrRss+"_", "RSS", "RSS search started for book_"+group)
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
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, listName, logger.StrSearchMissingInc, logger.StrSearchMissingInc+"_", "Search", "Search started for book list "+listName)
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
	dispatchClearHistory(c, logger.StrBook, logger.StrBook, c.Param(StrName))
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

	err := database.ExecNErr("DELETE FROM book_histories WHERE id = ?", &id)
	handleDBError(c, err, "History entry cleared")
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
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, listName, logger.StrRssAuthors, logger.StrRssAuthors+"_", "Search", "Author search (missing) started for book list "+listName)
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
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrBook, logger.StrBook, listName, logger.StrRssAuthorsUpgrade, logger.StrRssAuthorsUpgrade+"_", "Search", "Author search (upgrade) started for book list "+listName)
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

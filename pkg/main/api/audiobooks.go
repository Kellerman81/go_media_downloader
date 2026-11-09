package api

import (
	"context"
	"net/http"
	"strconv"

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
			routeraudiobookssearch.GET("/missing/:name", apiAudiobooksSearchMissing)
			routeraudiobookssearch.GET("/history/clear/:name", apiAudiobooksClearHistoryName)
			routeraudiobookssearch.GET("/history/clearid/:id", apiAudiobooksClearHistoryID)
			routeraudiobookssearch.GET("/authors/missing/:name", apiAudiobooksSearchAuthorsMissing)
			routeraudiobookssearch.GET("/authors/upgrade/:name", apiAudiobooksSearchAuthorsUpgrade)
		}
	}
}

// @Summary      List audiobooks (metadata)
// @Description  Returns the audiobooks of the dbaudiobooks table (metadata independent of lists), paginated, with the total count.
// @Tags         audiobooks · data
// @Param        limit  query     int     false  "Rows per page (default 100, max 1000)"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Dbaudiobook}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/audiobooks [get].
func apiAudiobooksDBList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	boundPagination(&params)

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

// @Summary      List the audiobooks of a list
// @Description  Returns the audiobook entries of one list, paginated, with the total count.
// @Tags         audiobooks · data
// @Param        name   path      string  true   "Name of the list (case-insensitive), as configured in the media group"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Audiobook}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/audiobooks/list/{name} [get].
func apiAudiobooksListGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](
		false,
		"select count() from audiobooks where listname = ? COLLATE NOCASE",
		&name,
	)

	query := "select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbaudiobook_id,author_id,book_series_id from audiobooks where listname = ? COLLATE NOCASE"
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

// @Summary      Remove an audiobook from a list
// @Description  Deletes one audiobook list entry and its file rows. Files on disk are not touched.
// @Tags         audiobooks · data
// @Param        id     path      int     true   "audiobooks (list entry) ID"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonsuccess  "Deleted"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      500  {object}  Jsonerror  "Deleting the files or the row failed"
// @Router       /api/audiobooks/{id} [delete].
func apiAudiobookDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Delete audiobook files first
	if err := database.ExecNErr("DELETE FROM audiobook_files WHERE audiobook_id = ?", &id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete audiobook files: " + err.Error()})
		return
	}
	// Delete the audiobook
	if err := database.ExecNErr("DELETE FROM audiobooks WHERE id = ?", &id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete audiobook: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// @Summary      Start a job for all audiobook groups
// @Description  Queues the given job for every audiobook media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · jobs
// @Param        job    path      string  true   "Job to start, one of: rss, rssauthors, rssauthorsupgrade, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/job/{job} [get].
func apiAudiobooksAllJobs(c *gin.Context) {
	dispatchMediaAllJobs(c, mediaJobDispatchConfig{
		TypeName:    "audiobook",
		NamePrefix:  logger.StrAudiobook,
		AllowedJobs: allowedjobsaudiobooksstr,
	})
}

// @Summary      Start a job for one audiobook group
// @Description  Queues the given job for one audiobook media group and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · jobs
// @Param        job    path      string  true   "Job to start, one of: rss, rssauthors, rssauthorsupgrade, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name   path      string  true   "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/job/{job}/{name} [get].
func apiAudiobooksJobs(c *gin.Context) {
	dispatchMediaJob(c, mediaJobDispatchConfig{
		TypeName:    "audiobook",
		NamePrefix:  logger.StrAudiobook,
		AllowedJobs: allowedjobsaudiobooksstr,
	})
}

// @Summary      Preview the RSS search of an audiobook group
// @Description  Reads the indexers' RSS feeds for one audiobook group and returns the accepted and denied releases. Nothing is downloaded.
// @Tags         audiobooks · search
// @Param        group  path      string  true   "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/audiobooks/rss/search/list/{group} [get].
func apiAudiobooksRssSearchList(c *gin.Context) {
	rssPreviewGroup(c, logger.StrAudiobook)
}

// @Summary      Search an audiobook and show the results
// @Description  Searches the indexers for one audiobook list entry and waits for the result: returns the accepted and denied releases with the reasons. download=true also downloads the best accepted release.
// @Tags         audiobooks · search
// @Param        id             path      int     true   "audiobooks (list entry) ID"
// @Param        searchByTitle  query     string  false  "true: also search by title"
// @Param        download       query     bool    false  "true: download the best accepted release"
// @Param        apikey         query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/audiobooks/search/list/{id} [get].
func apiAudiobooksSearchList(c *gin.Context) {
	searchListEntry(c, logger.StrAudiobook, "audiobooks")
}

// @Summary      Search missing audiobooks of a group
// @Description  Queues an incremental search for the missing audiobooks of a audiobook group; accepted releases are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonsearchstarted  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/search/missing/{name} [get].
func apiAudiobooksSearchMissing(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrAudiobook, logger.StrAudiobook, listName, logger.StrSearchMissingInc, logger.StrSearchMissingInc+"_", "Search", "Search started for audiobook list "+listName)
}

// @Summary      Clear the download history of a group or list
// @Description  Queues deleting the audiobook download history of an audiobook media group (or of one of its lists). Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/search/history/clear/{name} [get].
func apiAudiobooksClearHistoryName(c *gin.Context) {
	dispatchClearHistory(c, logger.StrAudiobook, logger.StrAudiobook, c.Param(StrName))
}

// @Summary      Clear the download history of an audiobook
// @Description  Deletes all download history rows of one audiobook list entry, so its releases can be downloaded again. Returns the number of deleted rows.
// @Tags         audiobooks · search
// @Param        id     path      int     true   "audiobooks (list entry) ID"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "Number of deleted history rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/audiobooks/search/history/clearid/{id} [get].
func apiAudiobooksClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	inres, inerr := database.DeleteRow("audiobook_histories", "audiobook_id = ?", id)
	handleDBInsertOrUpdate(c, inres, inerr, false)
}

// @Summary      Search missing audiobooks by author
// @Description  Queues indexer searches by author name (up to 20 random authors with missing audiobooks) for an audiobook media group. Accepted releases are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonauthorsearchmissing  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/search/authors/missing/{name} [get].
func apiAudiobooksSearchAuthorsMissing(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrAudiobook, logger.StrAudiobook, listName, logger.StrRssAuthors, logger.StrRssAuthors+"_", "Search", "Author search (missing) started for audiobook list "+listName)
}

// @Summary      Search audiobook upgrades by author
// @Description  Queues indexer searches by author name (up to 20 random authors with audiobooks below their quality cutoff) for an audiobook media group. Accepted upgrades are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonauthorsearchupgrade  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/search/authors/upgrade/{name} [get].
func apiAudiobooksSearchAuthorsUpgrade(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, logger.StrAudiobook, logger.StrAudiobook, listName, logger.StrRssAuthorsUpgrade, logger.StrRssAuthorsUpgrade+"_", "Search", "Author search (upgrade) started for audiobook list "+listName)
}

// @Summary      Refresh the metadata of all audiobooks
// @Description  Queues a metadata refresh (Audnex, Audible) of every audiobook in the lists of all audiobook media groups. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/all/refreshall [get].
func apirefreshAudiobooks(c *gin.Context) {
	dispatchJobAllConfigs(c, logger.StrAudiobook, logger.StrAudiobook, "refresh", StrStarted)
}

// @Summary      Refresh the metadata of one audiobook
// @Description  Queues a metadata refresh of one audiobook. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · jobs
// @Param        id   path      int  true  "ID of the audiobook in the dbaudiobooks table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/refresh/{id} [get].
func apirefreshAudiobook(c *gin.Context) {
	id := c.Param(logger.StrID)
	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM audiobooks WHERE dbaudiobook_id = ? LIMIT 1",
		&id,
	)
	dispatchConfigFunc(
		c,
		mediaConfigForList(logger.StrAudiobook, listname),
		logger.StrAudiobook,
		"Refresh Single Audiobook_"+id,
		func(_ context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RefreshAudiobook(cfgp, &id)
		},
	)
}

// @Summary      Rewrite the audio tags of one audiobook
// @Description  Queues rewriting the audio tags of the audiobook's files from the stored metadata (no new matching). Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · tags
// @Param        id     path      int     true  "ID of the audiobook in the dbaudiobooks table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/tag/{id} [get].
func apiRetagAudiobook(c *gin.Context) {
	dbID, ok := parseUintParam(c, logger.StrID)
	if !ok {
		return
	}

	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM audiobooks WHERE dbaudiobook_id = ? LIMIT 1",
		&dbID,
	)
	dispatchConfigFunc(
		c,
		mediaConfigForList(logger.StrAudiobook, listname),
		logger.StrAudiobook,
		"Retag Audiobook_"+c.Param(logger.StrID),
		func(ctx context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RetagAudiobook(ctx, cfgp, dbID)
		},
	)
}

// @Summary      Rewrite the audio tags of an author's audiobooks
// @Description  Queues rewriting the audio tags of all audiobooks of one author from the stored metadata. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · tags
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Param        id     path      int     true   "ID of the author in the dbauthors table"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/tag/author/{id} [get].
func apiRetagAuthorAudiobooks(c *gin.Context) {
	authorID, ok := parseUintParam(c, logger.StrID)
	if !ok {
		return
	}

	dispatchConfigFunc(
		c,
		mediaConfigForList(logger.StrAudiobook, ""),
		logger.StrAudiobook,
		"Retag Author Audiobooks_"+c.Param(logger.StrID),
		func(ctx context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RetagAuthorAudiobooks(ctx, cfgp, authorID)
		},
	)
}

// @Summary      Rewrite the audio tags of all audiobooks
// @Description  Queues rewriting the audio tags of every audiobook of every audiobook media group from the stored metadata. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · tags
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/tag/all [get].
func apiRetagAllAudiobooks(c *gin.Context) {
	dispatchConfigFuncAll(c, logger.StrAudiobook, logger.StrAudiobook, "Retag All Audiobooks", utils.RetagAllAudiobooks)
}

// @Summary      Refresh audiobook metadata (incremental)
// @Description  Queues a metadata refresh of the 100 least recently updated audiobooks of each audiobook media group. Returns immediately; the work runs in the job queue.
// @Tags         audiobooks · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/all/refresh [get].
func apirefreshAudiobooksInc(c *gin.Context) {
	dispatchJobAllConfigs(c, logger.StrAudiobook, logger.StrAudiobook, "refreshinc", StrStarted)
}

// @Summary      Add an author and import their audiobooks
// @Description  Adds an Audnex author (by Audible ASIN) to the given list and imports all their audiobooks in the background.
// @Tags         audiobooks · data
// @Param        name      path   string  true  "Media config name (without 'audiobook_' prefix)"
// @Param        listname  path   string  true  "List name within the media config"
// @Param        author_id query  string  true  "Audnex/Audible author ASIN"
// @Param        apikey    query  string  true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonauthorqueued  "'Author <id> queued for import into <group>/<list>'"
// @Failure      400  {object}  Jsonbadrequest  "author_id missing, or media group/list not found"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/audiobooks/author/add/{name}/{listname} [get].
func apiAudiobooksAddAuthor(c *gin.Context) {
	mediaName := c.Param(StrName)
	listName := c.Param("listname")
	authorID := c.Query("author_id")

	if authorID == "" {
		sendBadRequest(c, "author_id query parameter is required")
		return
	}

	cfgpstr := logger.StrAudiobook + "_" + mediaName

	cfgp := config.GetSettingsMedia(cfgpstr)
	if cfgp == nil {
		sendBadRequest(c, "audiobook config not found: "+cfgpstr)
		return
	}

	listid, ok := cfgp.ListsMapIdx[listName]
	if !ok {
		sendBadRequest(c, "list "+listName+" not found in "+cfgpstr)
		return
	}

	authorIDCopy := authorID
	if err := worker.Dispatch(
		"add_author_"+authorID+"_"+cfgpstr,
		func(_ uint32, ctx context.Context) error {
			// Import all audiobooks by this author into dbaudiobooks and audiobooks.
			// CheckaddAudiobookEntry (called internally) creates the authors tracking entry.
			return importfeed.JobImportDBAudiobook(ctx, &config.ManualConfig{
				AuthorID: authorIDCopy,
			}, 0, cfgp, listid)
		},
		"Data",
	); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, "Author "+authorID+" queued for import into "+cfgpstr+"/"+listName)
}

package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

// maxConcurrentEpisodeSearches bounds how many per-episode MediaSearch calls
// run at once for a single full-series or single-season search request (see
// searchEpisodesConcurrently). Each MediaSearch call fans out to every
// configured indexer and waits for the slowest one, so running episodes
// fully sequentially made a full-season search (e.g. 24 episodes) take 24
// sequential indexer round-trips. A small, bounded worker count captures
// most of the wall-clock win without driving materially more concurrent
// load at the shared indexer rate limiters/circuit breakers than the rest
// of the app already does.
const maxConcurrentEpisodeSearches = 4

// searchEpisodesConcurrently runs a MediaSearch for each of the given episode
// IDs with bounded concurrency (at most maxConcurrentEpisodeSearches at a
// time), downloading any accepted results per episode. Used by both
// apiSeriesSearch (all episodes) and apiSeriesSearchSeason (one season).
//
// Each goroutine gets its own *searcher.ConfigSearcher via
// searcher.NewSearcher, which is pooled (sync.Pool-backed) and freshly reset
// per Get - so concurrent use here is exactly the pool's intended usage
// pattern, not a hazard. Every episode is still searched exactly once;
// errors matching logger.ErrDisabled are skipped silently (not logged, not
// returned) same as before; any other search error is logged and stored -
// mirroring the old sequential loop's "err = errsub" (last one wins), except
// under concurrency "last" means last to finish rather than last by episode
// order.
func searchEpisodesConcurrently(
	ctx context.Context,
	media *config.MediaTypeConfig,
	episodes []uint,
) error {
	wg := pool.NewSizedGroup(maxConcurrentEpisodeSearches)

	var (
		mu      sync.Mutex
		lasterr error
	)

	for idxepisode := range episodes {
		episodeID := episodes[idxepisode]

		wg.Add()

		go func() {
			defer wg.Done()

			results := searcher.NewSearcher(media, nil, "", nil)
			defer results.Close()

			errsub := results.MediaSearch(ctx, media, episodeID, true, false, false)
			if errsub != nil {
				if !errors.Is(errsub, logger.ErrDisabled) {
					logger.Logtype("error", 2).
						Any(logger.StrID, &episodeID).
						Str("typ", logger.StrSeries).
						Err(errsub).
						Msg("Search Failed")

					mu.Lock()
					lasterr = errsub
					mu.Unlock()
				}
			} else if len(results.Accepted) >= 1 {
				results.Download()
			}
		}()
	}

	wg.Wait()

	return lasterr
}

func AddSeriesRoutes(routerseries *gin.RouterGroup) {
	routerseries.Use(checkauth)
	{
		routerseries.GET("/", apiSeriesGet)
		routerseries.POST("/", updateDBSeries)
		routerseries.DELETE("/:id", apiSeriesDelete)

		routerseries.GET("/list/:name", apiSeriesListGet)
		routerseries.POST("/list/", updateSeries)
		routerseries.DELETE("/list/:id", apiSeriesListDelete)

		routerseries.GET("/metadata/:tvdb", apiSeriesMetadataGet)

		routerseries.GET("/all/refresh", apirefreshSeriesInc)
		routerseries.GET("/all/refreshall", apirefreshSeries)
		routerseries.GET("/refresh/:id", apirefreshSerie)

		routerseries.GET("/job/:job", apiseriesAllJobs)
		routerseries.GET("/job/:job/:name", apiseriesJobs)

		routerseries.GET("/unmatched", apiSeriesUnmatched)

		routerseries.GET("/episodes", apiSeriesEpisodesGet)
		routerseries.POST("/episodes", updateDBEpisode)

		routerseries.GET("/episodes/:id", apiSeriesEpisodesGetSingle)
		routerseries.DELETE("/episodes/:id", apiSeriesEpisodesDelete)
		routerseries.POST("/episodes/list/", updateEpisode)
		routerseries.GET("/episodes/list/:id", apiSeriesEpisodesListGet)
		routerseries.DELETE("/episodes/list/:id", apiSeriesEpisodesListDelete)
		routerseries.GET("/rss/search/list/:group", apiSeriesRssSearchList)

		routerseriessearch := routerseries.Group("/search")
		{
			routerseriessearch.GET("/id/:id", apiSeriesSearch)
			routerseriessearch.GET("/id/:id/:season", apiSeriesSearchSeason)
			routerseriessearch.GET("/history/clear/:name", apiSeriesClearHistoryName)
			routerseriessearch.GET("/history/clearid/:id", apiSeriesClearHistoryID)
		}

		routerseriessearchrss := routerseries.Group("/searchrss")
		{
			routerseriessearchrss.GET("/id/:id", apiSeriesSearchRSS)
			routerseriessearchrss.GET("/id/:id/:season", apiSeriesSearchRSSSeason)
			routerseriessearchrss.GET("/list/id/:id", apiSeriesSearchRSSList)
			routerseriessearchrss.GET("/seasons/id/:id", apiSeriesSearchRSSSeasons)
		}

		routerseriesepisodessearch := routerseries.Group("/episodes/search")
		{
			routerseriesepisodessearch.GET("/id/:id", apiSeriesEpisodeSearch)
			routerseriesepisodessearch.GET("/list/:id", apiSeriesEpisodeSearchList)
			routerseriesepisodessearch.POST("/download/:id", apiSeriesEpisodeSearchDownload)
		}
	}
}

const querybydbserieid = "dbserie_id = ?"

// @Summary      List series (metadata)
// @Description  Returns the series of the dbseries table - one row per series with its metadata, independent of lists - paginated, with the total count.
// @Tags         series · data
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Dbserie}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series [get].
func apiSeriesGet(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)

	rows := database.Getdatarow[uint](false, "select count() from dbseries")
	data := database.QueryDbserie(query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Delete a series everywhere
// @Description  Deletes a series' metadata (dbseries), its episodes, its entries in all lists and their history and file rows. Files on disk are not touched.
// @Tags         series · data
// @Param        id   path      int  true  "ID of the series in the dbseries table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Finished"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/{id} [delete].
func apiSeriesDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	database.DeleteRow("serie_episode_files", querybydbserieid, id)
	database.DeleteRow("serie_episode_histories", querybydbserieid, id)
	database.DeleteRow("serie_episodes", querybydbserieid, id)
	database.DeleteRow("dbserie_episodes", querybydbserieid, id)
	database.DeleteRow(logger.StrSeries, querybydbserieid, id)

	_, err := database.DeleteRow("dbseries", logger.FilterByID, id)

	handleDBError(ctx, err, StrOK)
}

// @Summary      List the series of a list
// @Description  Returns the entries of one list (series table joined with dbseries), paginated, with the total count.
// @Tags         series · data
// @Param        name   path      string  true   "Name of the list (case-insensitive), as configured in the media group"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.ResultSeries}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series/list/{name} [get].
func apiSeriesListGet(ctx *gin.Context) {
	listName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)
	query := buildQueryWithWhere(params, "series.listname = ? COLLATE NOCASE")

	query.InnerJoin = "dbseries on series.dbserie_id=dbseries.id"

	rows := database.Getdatarow[uint](
		false,
		"select count() from series where listname = ? COLLATE NOCASE",
		&listName,
	)
	data := database.QueryResultSeries(query, listName)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Remove a series from a list
// @Description  Deletes one list entry (series table) with its episode, history and file rows. The series metadata and other lists stay. Files on disk are not touched.
// @Tags         series · data
// @Param        id   path      int  true  "Series list entry ID (series.id)"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Finished"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/list/{id} [delete].
func apiSeriesListDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Deletes exactly one list entry (series.id) and its children; keying on
	// dbserie_id would remove the show from every list it is in.
	database.DeleteRow("serie_episode_files", "serie_id = ?", id)
	database.DeleteRow("serie_episode_histories", "serie_id = ?", id)
	database.DeleteRow("serie_episodes", "serie_id = ?", id)

	_, err := database.DeleteRow(logger.StrSeries, logger.FilterByID, id)

	handleDBError(ctx, err, StrOK)
}

// @Summary      List unmatched episode files
// @Description  Returns the episode files the organizer could not match (serie_file_unmatcheds), paginated, with the total count.
// @Tags         series · data
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.SerieFileUnmatched}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series/unmatched [get].
func apiSeriesUnmatched(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)

	rows := database.Getdatarow[uint](false, "select count() from serie_file_unmatcheds")
	data := database.QuerySerieFileUnmatched(query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      List episodes (metadata)
// @Description  Returns the episodes of the dbserie_episodes table (metadata of all series), paginated, with the total count.
// @Tags         series · episodes
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.DbserieEpisode}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series/episodes [get].
func apiSeriesEpisodesGet(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)

	rows := database.Getdatarow[uint](false, "select count() from dbserie_episodes")
	data := database.QueryDbserieEpisodes(query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      List the episodes of a series (metadata)
// @Description  Returns the dbserie_episodes rows of one series, paginated, with the total count.
// @Tags         series · episodes
// @Param        id     path      int     true   "ID of the series in the dbseries table"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.DbserieEpisode}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series/episodes/{id} [get].
func apiSeriesEpisodesGetSingle(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)
	query := buildQueryWithWhere(params, querybydbserieid)

	rows := database.Getdatarow[uint](
		false,
		"select count() from dbserie_episodes where dbserie_id = ?",
		&id,
	)
	data := database.QueryDbserieEpisodes(query, id)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Delete an episode everywhere
// @Description  Deletes an episode's metadata (dbserie_episodes) and its entries, history and file rows in all lists. Files on disk are not touched.
// @Tags         series · episodes
// @Param        id   path      int  true  "ID of the episode in the dbserie_episodes table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Finished"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/episodes/{id} [delete].
func apiSeriesEpisodesDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	database.DeleteRow("serie_episode_files", "dbserie_episode_id = ?", id)
	database.DeleteRow("serie_episode_histories", "dbserie_episode_id = ?", id)
	database.DeleteRow("serie_episodes", "dbserie_episode_id = ?", id)

	_, err := database.DeleteRow("dbserie_episodes", logger.FilterByID, id)

	handleDBError(ctx, err, StrOK)
}

// @Summary      List the episodes of a series list entry
// @Description  Returns the serie_episodes rows (missing, quality, ...) of one series list entry joined with their metadata, paginated, with the total count.
// @Tags         series · episodes
// @Param        id     path      int     true   "series (list entry) ID"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.ResultSerieEpisodes}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series/episodes/list/{id} [get].
func apiSeriesEpisodesListGet(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)
	query := buildQueryWithWhere(params, "series.id = ?")

	query.InnerJoin = "dbserie_episodes on serie_episodes.dbserie_episode_id=dbserie_episodes.id inner join series on series.id=serie_episodes.serie_id"

	rows := database.Getdatarow[uint](
		false,
		"select count() from serie_episodes where serie_id = ?",
		&id,
	)
	data := database.QueryResultSerieEpisodes(query, id)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Remove an episode from a list entry
// @Description  Deletes one serie_episodes row with its history and file rows. The episode metadata stays. Files on disk are not touched.
// @Tags         series · episodes
// @Param        id   path      int  true  "ID of the list entry in the serie_episodes table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Finished"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/episodes/list/{id} [delete].
func apiSeriesEpisodesListDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	database.DeleteRow("serie_episode_files", "serie_episode_id = ?", id)
	database.DeleteRow("serie_episode_histories", "serie_episode_id = ?", id)

	_, err := database.DeleteRow("serie_episodes", logger.FilterByID, id)

	handleDBError(ctx, err, StrOK)
}

const allowedjobsseriesstr = "rss,rssseasons,rssseasonsall,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// @Summary      Start a job for all series groups
// @Description  Queues the given job for every series media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         series · jobs
// @Param        job  path      string  true  "Job to start, one of: rss, rssseasons, rssseasonsall, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/job/{job} [get].
func apiseriesAllJobs(c *gin.Context) {
	dispatchMediaAllJobs(c, mediaJobDispatchConfig{
		TypeName:    logger.StrSerie,
		NamePrefix:  logger.StrSerie,
		AllowedJobs: allowedjobsseriesstr,
	})
}

// @Summary      Start a job for one series group
// @Description  Queues the given job for one series media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         series · jobs
// @Param        job   path      string  true  "Job to start, one of: rss, rssseasons, rssseasonsall, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name  path      string  true  "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/job/{job}/{name} [get].
func apiseriesJobs(c *gin.Context) {
	dispatchMediaJob(c, mediaJobDispatchConfig{
		TypeName:    logger.StrSerie,
		NamePrefix:  logger.StrSerie,
		AllowedJobs: allowedjobsseriesstr,
	})
}

// @Summary      Add or update series metadata
// @Description  Inserts the dbseries row when its id is 0 or unknown, otherwise updates it. Returns the number of affected rows.
// @Tags         series · data
// @Param        series  body      database.Dbserie  true  "Series metadata row (dbseries); an ID that does not exist yet creates a new row"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "ID of the new row, or the number of updated rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series [post].
func updateDBSeries(c *gin.Context) {
	var dbserie database.Dbserie
	if !bindJSONWithValidation(c, &dbserie) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from dbseries where id != 0 and id = ?",
		&dbserie.ID,
	)

	var (
		inres sql.Result
		inerr error
	)

	if counter == 0 {
		inres, inerr = database.InsertArray(
			"dbseries",
			[]string{
				"Seriename",
				"Season",
				"Status",
				"Firstaired",
				"Network",
				"Runtime",
				"Language",
				"Genre",
				"Overview",
				"Rating",
				"Siterating",
				"Siterating_Count",
				"Slug",
				"Trakt_ID",
				"Imdb_ID",
				"Thetvdb_ID",
				"Freebase_M_ID",
				"Freebase_ID",
				"Tvrage_ID",
				"Facebook",
				"Instagram",
				"Twitter",
				"Banner",
				"Poster",
				"Fanart",
				"Identifiedby",
			},
			dbserie.Seriename,
			dbserie.Season,
			dbserie.Status,
			dbserie.Firstaired,
			dbserie.Network,
			dbserie.Runtime,
			dbserie.Language,
			dbserie.Genre,
			dbserie.Overview,
			dbserie.Rating,
			dbserie.Siterating,
			dbserie.SiteratingCount,
			dbserie.Slug,
			dbserie.TraktID,
			dbserie.ImdbID,
			dbserie.ThetvdbID,
			dbserie.FreebaseMID,
			dbserie.FreebaseID,
			dbserie.TvrageID,
			dbserie.Facebook,
			dbserie.Instagram,
			dbserie.Twitter,
			dbserie.Banner,
			dbserie.Poster,
			dbserie.Fanart,
			dbserie.Identifiedby,
		)
	} else {
		inres, inerr = database.UpdateArray(
			"dbseries",
			[]string{
				"Seriename",
				"Season",
				"Status",
				"Firstaired",
				"Network",
				"Runtime",
				"Language",
				"Genre",
				"Overview",
				"Rating",
				"Siterating",
				"Siterating_Count",
				"Slug",
				"Trakt_ID",
				"Imdb_ID",
				"Thetvdb_ID",
				"Freebase_M_ID",
				"Freebase_ID",
				"Tvrage_ID",
				"Facebook",
				"Instagram",
				"Twitter",
				"Banner",
				"Poster",
				"Fanart",
				"Identifiedby",
			},
			"id != 0 and id = ?",
			dbserie.Seriename,
			dbserie.Season,
			dbserie.Status,
			dbserie.Firstaired,
			dbserie.Network,
			dbserie.Runtime,
			dbserie.Language,
			dbserie.Genre,
			dbserie.Overview,
			dbserie.Rating,
			dbserie.Siterating,
			dbserie.SiteratingCount,
			dbserie.Slug,
			dbserie.TraktID,
			dbserie.ImdbID,
			dbserie.ThetvdbID,
			dbserie.FreebaseMID,
			dbserie.FreebaseID,
			dbserie.TvrageID,
			dbserie.Facebook,
			dbserie.Instagram,
			dbserie.Twitter,
			dbserie.Banner,
			dbserie.Poster,
			dbserie.Fanart,
			dbserie.Identifiedby,
			dbserie.ID,
		)
	}

	handleDBInsertOrUpdate(c, inres, inerr, counter == 0)
}

// @Summary      Add or update episode metadata
// @Description  Inserts the dbserie_episodes row when its id is 0 or unknown, otherwise updates it. Returns the number of affected rows.
// @Tags         series · episodes
// @Param        episode  body      database.DbserieEpisode  true  "Episode metadata row (dbserie_episodes); an ID that does not exist yet creates a new row"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "ID of the new row, or the number of updated rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/episodes [post].
func updateDBEpisode(c *gin.Context) {
	var dbserieepisode database.DbserieEpisode
	if !bindJSONWithValidation(c, &dbserieepisode) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from dbserie_episodes where id != 0 and id = ?",
		&dbserieepisode.ID,
	)

	var (
		inres sql.Result
		inerr error
	)

	if counter == 0 {
		inres, inerr = database.InsertArray(
			"dbserie_episodes",
			[]string{
				"episode",
				"season",
				"identifier",
				"title",
				"first_aired",
				"overview",
				"poster",
				"dbserie_id",
			},
			dbserieepisode.Episode,
			dbserieepisode.Season,
			dbserieepisode.Identifier,
			dbserieepisode.Title,
			dbserieepisode.FirstAired,
			dbserieepisode.Overview,
			dbserieepisode.Poster,
			dbserieepisode.DbserieID,
		)
	} else {
		inres, inerr = database.UpdateArray(
			"dbserie_episodes",
			[]string{
				"episode",
				"season",
				"identifier",
				"title",
				"first_aired",
				"overview",
				"poster",
				"dbserie_id",
			},
			"id != 0 and id = ?",
			dbserieepisode.Episode,
			dbserieepisode.Season,
			dbserieepisode.Identifier,
			dbserieepisode.Title,
			dbserieepisode.FirstAired,
			dbserieepisode.Overview,
			dbserieepisode.Poster,
			dbserieepisode.DbserieID,
			dbserieepisode.ID,
		)
	}

	handleDBInsertOrUpdate(c, inres, inerr, counter == 0)
}

// @Summary      Add or update a series list entry
// @Description  Inserts the series (list entry) row when its id is 0 or unknown, otherwise updates it. Returns the number of affected rows.
// @Tags         series · data
// @Param        series  body      database.Serie  true  "Series list entry (series table); an ID that does not exist yet creates a new row"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "ID of the new row, or the number of updated rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/list [post].
func updateSeries(c *gin.Context) {
	var serie database.Serie
	if !bindJSONWithValidation(c, &serie) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from series where id != 0 and id = ?",
		&serie.ID,
	)

	var (
		inres sql.Result
		inerr error
	)

	if counter == 0 {
		inres, inerr = database.InsertArray(
			logger.StrSeries,
			[]string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			serie.DbserieID,
			serie.Listname,
			serie.Rootpath,
			serie.DontUpgrade,
			serie.DontSearch,
		)
	} else {
		inres, inerr = database.UpdateArray(
			logger.StrSeries,
			[]string{"dbserie_id", "listname", "rootpath", "dont_upgrade", "dont_search"},
			"id != 0 and id = ?",
			serie.DbserieID,
			serie.Listname,
			serie.Rootpath,
			serie.DontUpgrade,
			serie.DontSearch,
			serie.ID,
		)
	}

	handleDBInsertOrUpdate(c, inres, inerr, counter == 0)
}

// @Summary      Add or update an episode list entry
// @Description  Inserts the serie_episodes row when its id is 0 or unknown, otherwise updates it (missing, quality profile, blacklisted, ...). Returns the number of affected rows.
// @Tags         series · episodes
// @Param        episode  body      database.SerieEpisode  true  "Episode list entry (serie_episodes); an ID that does not exist yet creates a new row"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "ID of the new row, or the number of updated rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/episodes/list [post].
func updateEpisode(c *gin.Context) {
	var serieepisode database.SerieEpisode
	if !bindJSONWithValidation(c, &serieepisode) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from serie_episodes where id != 0 and id = ?",
		&serieepisode.ID,
	)

	var (
		inres sql.Result
		inerr error
	)

	if counter == 0 {
		inres, inerr = database.InsertArray(
			"serie_episodes",
			[]string{
				"dbserie_id",
				"serie_id",
				"missing",
				"quality_profile",
				"dbserie_episode_id",
				"blacklisted",
				"quality_reached",
				"dont_upgrade",
				"dont_search",
			},
			serieepisode.DbserieID,
			serieepisode.SerieID,
			serieepisode.Missing,
			serieepisode.QualityProfile,
			serieepisode.DbserieEpisodeID,
			serieepisode.Blacklisted,
			serieepisode.QualityReached,
			serieepisode.DontUpgrade,
			serieepisode.DontSearch,
		)
	} else {
		inres, inerr = database.UpdateArray(
			"serie_episodes",
			[]string{
				"dbserie_id",
				"serie_id",
				"missing",
				"quality_profile",
				"dbserie_episode_id",
				"blacklisted",
				"quality_reached",
				"dont_upgrade",
				"dont_search",
			},
			"id != 0 and id = ?",
			serieepisode.DbserieID,
			serieepisode.SerieID,
			serieepisode.Missing,
			serieepisode.QualityProfile,
			serieepisode.DbserieEpisodeID,
			serieepisode.Blacklisted,
			serieepisode.QualityReached,
			serieepisode.DontUpgrade,
			serieepisode.DontSearch,
			serieepisode.ID,
		)
	}

	handleDBInsertOrUpdate(c, inres, inerr, counter == 0)
}

// @Summary      Refresh the metadata of one series
// @Description  Queues a metadata refresh of one series and its episodes from the configured sources. Returns immediately; the work runs in the job queue.
// @Tags         series · jobs
// @Param        id   path      int  true  "ID of the series in the dbseries table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/refresh/{id} [get].
func apirefreshSerie(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	// Find this series' list so we pick the series config that actually owns
	// it - mirrors apirefreshMovie (movies.go): there may be several series
	// configs, and picking the first one found would fail (or silently
	// refresh under the wrong config) when the series belongs to a
	// different config's list.
	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM series WHERE dbserie_id = ? LIMIT 1",
		&id,
	)

	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return false
		}

		if cfgp == nil {
			cfgp = media // fall back to the first series config
		}

		if listname != "" && media.GetMediaListsEntryListID(listname) != -1 {
			cfgp = media // this config owns the series' list
			return true
		}

		return false
	})

	// Include the id in the dispatch name - worker.Dispatch rejects a
	// submission sharing a name with one already queued, so a bare
	// "Refresh Single Serie" name would silently drop a concurrent request
	// for a different series.
	if cfgp == nil {
		sendNotFound(c, "No series configuration found")
		return
	}

	if err := worker.Dispatch("Refresh Single Serie_"+id, func(_ uint32, _ context.Context) error {
		return utils.RefreshSerie(cfgp, &id)
	}, "Feeds"); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, StrStarted)
}

// @Summary      Refresh the metadata of all series
// @Description  Queues a metadata refresh of every series in the lists of all series media groups. Returns immediately; the work runs in the job queue.
// @Tags         series · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/all/refreshall [get].
func apirefreshSeries(c *gin.Context) {
	dispatchJobAllConfigs(c, logger.StrSerie, logger.StrSerie, "refresh", StrStarted)
}

// @Summary      Refresh series metadata (incremental)
// @Description  Queues a metadata refresh of the 20 least recently updated continuing series of each series media group. Returns immediately; the work runs in the job queue.
// @Tags         series · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/all/refresh [get].
func apirefreshSeriesInc(c *gin.Context) {
	dispatchJobAllConfigs(c, logger.StrSerie, logger.StrSerie, "refreshinc", StrStarted)
}

// @Summary      Search all episodes of a series and download
// @Description  Queues a search of the indexers for every episode of one series list entry and downloads the best accepted release per episode. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        id   path      int  true  "series (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Series not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/search/id/{id} [get].
func apiSeriesSearch(c *gin.Context) {
	serie, _ := database.GetSeries(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := worker.Dispatch(
					"searchseries_series_"+media.Name+"_"+strconv.Itoa(
						int(serie.ID),
					),
					func(_ uint32, ctx context.Context) error {
						episodes := database.GetrowsN[uint](
							false,
							database.Getdatarow[uint](
								false,
								"select count() from serie_episodes where serie_id = ?",
								&serie.ID,
							),
							"select id from serie_episodes where serie_id = ?",
							&serie.ID,
						)

						return searchEpisodesConcurrently(ctx, media, episodes)
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search the episodes of one season and download
// @Description  Queues a search of the indexers for every episode of one season of a series list entry and downloads accepted releases. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        id   path      int  true  "series (list entry) ID"
// @Param        season  path      string  true  "Season number as stored for the series, e.g. 1 (0 for specials)"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Series not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/search/id/{id}/{season} [get].
func apiSeriesSearchSeason(c *gin.Context) {
	serie, _ := database.GetSeries(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)
	season := c.Param("season")

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := worker.Dispatch(
					"searchseriesseason_series_"+media.Name+"_"+strconv.Itoa(
						int(serie.ID), //nolint:gosec // safe: value within target type range
					)+"_"+season,
					func(_ uint32, ctx context.Context) error {
						episodes := database.GetrowsN[uint](
							false,
							database.Getdatarow[uint](
								false,
								"select count() from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)",
								&serie.ID,
								&season,
							),
							"select id from serie_episodes where serie_id = ? and dbserie_episode_id in (select id from dbserie_episodes where season = ?)",
							serie.ID,
							season,
						)

						return searchEpisodesConcurrently(ctx, media, episodes)
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search a series with one indexer call
// @Description  Queues one indexer search by TVDB ID for the whole series (no season) and downloads accepted releases for its missing episodes and upgrades. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        id      path      int     true  "series (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Series not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/searchrss/id/{id} [get].
func apiSeriesSearchRSS(c *gin.Context) {
	serie, _ := database.GetSeries(
		database.Querywithargs{Select: "id, listname", Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := worker.Dispatch(
					"searchseriesrss_series_"+media.Name+"_"+strconv.Itoa(
						int(serie.ID), //nolint:gosec // safe: value within target type range
					),
					func(uint32, context.Context) error {
						s, err := searcher.SearchSerieRSSSeasonSingle(&serie.ID, "", false, media)
						s.Close()
						return err
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search a series with one call and show the results
// @Description  Searches the indexers once by TVDB ID for the whole series, downloads accepted releases, and returns the accepted and denied releases.
// @Tags         series · search
// @Param        id      path      int     true  "series (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Series not found"
// @Router       /api/series/searchrss/list/id/{id} [get].
func apiSeriesSearchRSSList(c *gin.Context) {
	serie, _ := database.GetSeries(
		database.Querywithargs{Select: "id, listname", Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				found = true

				searchresults, err := searcher.SearchSerieRSSSeasonSingle(
					&serie.ID,
					"",
					false,
					media,
				)
				if err != nil {
					c.JSON(http.StatusNoContent, "Failed")
					return err
				}
				defer searchresults.Close()

				if len(searchresults.Raw.Arr) == 0 {
					c.JSON(http.StatusNoContent, "Failed")
					return err
				}

				c.JSON(
					http.StatusOK,
					gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied},
				)
				// searchresults.Close()
				return err
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search every season of a series
// @Description  Queues one indexer search per season (by TVDB ID and season) for a series list entry and downloads accepted releases, e.g. season packs. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        id      path      int     true  "series (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Series not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/searchrss/seasons/id/{id} [get].
func apiSeriesSearchRSSSeasons(c *gin.Context) {
	serie, err := database.GetSeries(
		database.Querywithargs{Select: "id, listname", Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if err != nil || serie == nil {
		sendNotFound(c, "Series not found")
		return
	}

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := worker.Dispatch(
					"searchseriesrssseasons_series_"+media.Name+"_"+strconv.Itoa(
						int(serie.ID), //nolint:gosec // safe: value within target type range
					),
					func(_ uint32, ctx context.Context) error {
						return searcher.SearchSerieRSSSeasonsAll(ctx, media, serie.ID)
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search one season with one indexer call
// @Description  Queues one indexer search by TVDB ID and season for a series list entry and downloads accepted releases, e.g. a season pack. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        id      path      int     true  "series (list entry) ID"
// @Param        season  path      string  true  "Season number as stored for the series, e.g. 1 (0 for specials)"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Series not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/searchrss/id/{id}/{season} [get].
func apiSeriesSearchRSSSeason(c *gin.Context) {
	serie, _ := database.GetSeries(
		database.Querywithargs{Select: "id, listname", Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)
	season := c.Param("season")

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := worker.Dispatch(
					"searchseriesrssseason_series_"+media.Name+"_"+strconv.Itoa(
						int(serie.ID), //nolint:gosec // safe: value within target type range
					)+"_"+season,
					func(uint32, context.Context) error {
						s, err := searcher.SearchSerieRSSSeasonSingle(
							&serie.ID,
							season,
							true,
							media,
						)
						s.Close()

						return err
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search an episode and download
// @Description  Queues a search of the indexers for one episode (missing or upgrade) and downloads the best accepted release. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        id   path      int  true  "serie_episodes (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Episode or series not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/episodes/search/id/{id} [get].
func apiSeriesEpisodeSearch(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param(logger.StrID))
	uid := uint(id) //nolint:gosec // safe: value within target type range
	dbid := c.Param(logger.StrID)
	serieid := database.Getdatarow[uint](false, database.QuerySerieEpisodesGetSerieIDByID, &dbid)
	serie, _ := database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, serieid)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := worker.Dispatch(
					"searchseriesepisode_series_"+media.Name+"_"+strconv.Itoa(id),
					func(_ uint32, ctx context.Context) error {
						results := searcher.NewSearcher(media, nil, "", nil)

						err := results.MediaSearch(ctx, media, uid, true, false, false)
						if err != nil {
							if !errors.Is(err, logger.ErrDisabled) {
								logger.Logtype("error", 2).
									Any(logger.StrID, &uid).
									Str("typ", logger.StrSeries).
									Err(err).
									Msg("Search Failed")

								return err
							}
						} else {
							if len(results.Accepted) >= 1 {
								results.Download()
							}
						}

						results.Close()

						return nil
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search an episode and show the results
// @Description  Searches the indexers for one episode and waits for the result: returns the accepted and denied releases with the reasons. download=true also downloads the best accepted release.
// @Tags         series · search
// @Param        id             path      int     true   "serie_episodes (list entry) ID"
// @Param        searchByTitle  query     string  false  "true: also search by title, not only by TVDB ID"
// @Param        download       query     bool    false  "true: download the best accepted release"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Episode or series not found"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/series/episodes/search/list/{id} [get].
func apiSeriesEpisodeSearchList(c *gin.Context) {
	serieepi, _ := database.GetSerieEpisodes(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if serieepi == nil {
		sendNotFound(c, "Episode not found")
		return
	}
	// defer logger.ClearVar(&serieepi)

	serie, _ := database.GetSeries(
		database.Querywithargs{Where: logger.FilterByID},
		serieepi.SerieID,
	)
	if serie == nil {
		sendNotFound(c, "Series not found")
		return
	}
	// defer logger.ClearVar(&serie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" || queryParam == "on" {
			titlesearch = true
		}
	}

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				found = true

				ctx := context.Background()
				searchresults := searcher.NewSearcher(media, nil, "", nil)

				err := searchresults.MediaSearch(ctx, media, serieepi.ID, titlesearch, false, false)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					searchresults.Close()
					return err
				}

				if val, ok := c.GetQuery("download"); ok && strings.EqualFold(val, "true") {
					searchresults.Download()
				}

				c.JSON(
					http.StatusOK,
					gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied},
				)
				searchresults.Close()
				// searchnow.Close()
				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Preview the RSS search of a series group
// @Description  Reads the indexers' RSS feeds for one series media group and returns the accepted and denied releases. Nothing is downloaded.
// @Tags         series · search
// @Param        group  path      string  true  "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/series/rss/search/list/{group} [get].
func apiSeriesRssSearchList(c *gin.Context) {
	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		if strings.EqualFold(media.Name, c.Param("group")) {
			found = true

			templatequality := media.TemplateQuality
			ctx := context.Background()
			searchresults := searcher.NewSearcher(media, media.CfgQuality, logger.StrRss, nil)

			err := searchresults.SearchRSS(
				ctx,
				media,
				config.GetSettingsQuality(templatequality),
				false,
				false,
			)
			if err != nil {
				str := "failed with " + err.Error()
				c.JSON(http.StatusNotFound, str)
				searchresults.Close()
				return err
			}

			c.JSON(
				http.StatusOK,
				gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied},
			)
			searchresults.Close()

			return nil
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Download a chosen release for an episode
// @Description  Sends the given release (e.g. one from /episodes/search/list) for the episode to the download client of the indexer's quality profile entry.
// @Tags         series · search
// @Param        nzb  body      apiexternal_v2.Nzbwithprio  true  "Nzb: Req. Title, Indexer, tvdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "ID of the list entry in the serie_episodes table"
// @Param        indexer query  string                  true  "Indexer config name the release came from"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id or body, or IDs do not match"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Episode or series not found"
// @Failure      500  {object}  Jsonerror  "Download failed"
// @Router       /api/series/episodes/search/download/{id} [post].
func apiSeriesEpisodeSearchDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param(logger.StrID), 10, 64)
	if err != nil || id == 0 {
		sendBadRequest(c, "Invalid episode id")
		return
	}

	serieepi, err := database.GetSerieEpisodes(
		database.Querywithargs{Select: "id, serie_id, missing", Where: logger.FilterByID},
		uint(id),
	)
	if err != nil || serieepi == nil || serieepi.ID == 0 {
		sendNotFound(c, "Episode not found")
		return
	}

	serie, err := database.GetSeries(
		database.Querywithargs{Where: logger.FilterByID},
		serieepi.SerieID,
	)
	if err != nil || serie == nil {
		sendNotFound(c, "Series not found")
		return
	}

	var nzb apiexternal_v2.Nzbwithprio
	if !bindJSONWithValidation(c, &nzb) {
		return
	}

	if nzb.NzbepisodeID != 0 && nzb.NzbepisodeID != serieepi.ID {
		sendBadRequest(c, "nzb_episode_id does not match the episode id in the path")
		return
	}

	nzb.NzbepisodeID = serieepi.ID

	if !setManualDownloadIndexer(c, &nzb) {
		return
	}

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrSerie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, serie.Listname) {
				if err := downloader.DownloadSeriesEpisode(media, &nzb); err != nil {
					sendJSONError(c, http.StatusInternalServerError, err.Error())
				} else {
					sendSuccess(c, StrStarted)
				}

				found = true

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Clear the download history of a group or list
// @Description  Queues deleting the episode download history of a series media group (or of one of its lists), so releases can be downloaded again. Returns immediately; the work runs in the job queue.
// @Tags         series · search
// @Param        name  path      string  true  "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/series/search/history/clear/{name} [get].
func apiSeriesClearHistoryName(c *gin.Context) {
	dispatchClearHistory(c, logger.StrSerie, logger.StrSerie, c.Param(StrName))
}

// @Summary      Clear the download history of an episode
// @Description  Deletes all download history rows of one episode list entry, so its releases can be downloaded again. Returns the number of deleted rows.
// @Tags         series · search
// @Param        id   path      string  true  "serie_episodes (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "Number of deleted history rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/series/search/history/clearid/{id} [get].
func apiSeriesClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	inres, inerr := database.DeleteRow("serie_episode_histories", "serie_episode_id = ?", id)
	handleDBInsertOrUpdate(c, inres, inerr, false)
}

// @Summary      Look up series metadata
// @Description  Fetches the metadata of a series by TVDB ID from TVDB, or from the given provider, and returns it. update=1 also saves it to dbseries.
// @Tags         series · data
// @Param        tvdb      path   int     true   "TheTVDB ID of the series"
// @Param        provider  query  string  false  "Provider: tvdb (default), tmdb, trakt"
// @Param        update    query  string  false  "Save to DB: 1"
// @Param        apikey    query  string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=database.Dbserie}  "Series metadata from the configured providers (not saved)"
// @Failure      400  {object}  Jsonbadrequest  "invalid tvdb id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/series/metadata/{tvdb} [get].
func apiSeriesMetadataGet(ctx *gin.Context) {
	tvdbID, err := strconv.Atoi(ctx.Param("tvdb"))
	if err != nil || tvdbID == 0 {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid tvdb id"})
		return
	}

	var dbserie database.Dbserie

	dbserie.ThetvdbID = tvdbID

	var querytmdb, querytrakt bool
	if queryParam, ok := ctx.GetQuery("provider"); ok {
		switch queryParam {
		case "tmdb":
			querytmdb = true
		case "trakt":
			querytrakt = true
		}
	} else {
		querytmdb = config.GetSettingsGeneral().SerieMetaSourceTmdb
		querytrakt = config.GetSettingsGeneral().SerieMetaSourceTrakt
	}

	metadata.SerieGetMetadata(&dbserie, "", querytmdb, querytrakt, true, nil)

	if queryParam, ok := ctx.GetQuery("update"); ok && queryParam == "1" {
		database.Scanrowsdyn(false, database.QueryDbseriesGetIDByTvdb, &dbserie.ID, &tvdbID)

		if dbserie.ID == 0 {
			inres, inErr := database.ExecNid(
				"insert into dbseries (seriename, thetvdb_id, identifiedby) values (?, ?, ?)",
				&dbserie.Seriename,
				&tvdbID,
				&dbserie.Identifiedby,
			)
			if inErr == nil {
				dbserie.ID = logger.Int64ToUint(inres)
			}
		}

		if dbserie.ID != 0 {
			database.ExecN(
				"update dbseries SET Seriename = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?",
				&dbserie.Seriename,
				&dbserie.Season,
				&dbserie.Status,
				&dbserie.Firstaired,
				&dbserie.Network,
				&dbserie.Runtime,
				&dbserie.Language,
				&dbserie.Genre,
				&dbserie.Overview,
				&dbserie.Rating,
				&dbserie.Siterating,
				&dbserie.SiteratingCount,
				&dbserie.Slug,
				&dbserie.TraktID,
				&dbserie.ImdbID,
				&dbserie.ThetvdbID,
				&dbserie.FreebaseMID,
				&dbserie.FreebaseID,
				&dbserie.TvrageID,
				&dbserie.Facebook,
				&dbserie.Instagram,
				&dbserie.Twitter,
				&dbserie.Banner,
				&dbserie.Poster,
				&dbserie.Fanart,
				&dbserie.Identifiedby,
				&dbserie.ID,
			)
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"data": dbserie})
}

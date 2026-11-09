// Package api provides the movies API handlers.
package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
)

func AddMoviesRoutes(routermovies *gin.RouterGroup) {
	routermovies.Use(checkauth)
	{
		routermovies.GET("/all/refresh", apirefreshMoviesInc)
		routermovies.GET("/all/refreshall", apirefreshMovies)
		routermovies.GET("/refresh/:id", apirefreshMovie)

		routermovies.GET("/unmatched", apiMovieListUnmatched)
		routermovies.GET("/", apiMovieList)
		routermovies.POST("/", updateDBMovie)
		routermovies.DELETE("/:id", apiMovieDelete)

		routermovies.POST("/list/", updateMovie)

		routermovies.GET("/list/:name", apiMovieListGet)
		routermovies.GET("/metadata/:imdb", apiMovieMetadataGet)
		routermovies.DELETE("/list/:id", movieDeleteList)

		routermovies.GET("/job/:job", apimoviesAllJobs)
		routermovies.GET("/job/:job/:name", apimoviesJobs)

		routermovies.GET("/rss/search/list/:group", apiMoviesRssSearchList)

		routermoviessearch := routermovies.Group("/search")
		{
			routermoviessearch.GET("/id/:id", apimoviesSearch)
			routermoviessearch.GET("/list/:id", apimoviesSearchList)
			routermoviessearch.POST("/download/:id", apimoviesSearchDownload)
			routermoviessearch.GET("/history/clear/:name", apimoviesClearHistoryName)
			routermoviessearch.GET("/history/clearid/:id", apiMoviesClearHistoryID)
		}
	}
}

// @Summary      List movies (metadata)
// @Description  Returns the movies of the dbmovies table - one row per movie with its metadata, independent of lists - paginated, with the total count.
// @Tags         movies · data
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Dbmovie}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/movies [get].
func apiMovieList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)

	rows := database.Getdatarow[uint](false, "select count() from dbmovies")
	data := database.QueryDbmovie(query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      List unmatched movie files
// @Description  Returns the movie files the organizer could not match to a movie (movie_file_unmatcheds), paginated, with the total count.
// @Tags         movies · data
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.MovieFileUnmatched}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/movies/unmatched [get].
func apiMovieListUnmatched(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)

	rows := database.Getdatarow[uint](false, "select count() from movie_file_unmatcheds")
	data := database.QueryMovieFileUnmatched(query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Delete a movie everywhere
// @Description  Deletes a movie's metadata row (dbmovies) and its entries in all lists. Files on disk are not touched.
// @Tags         movies · data
// @Param        id   path      int  true  "ID of the movie in the dbmovies table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Finished"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/movies/{id} [delete].
func apiMovieDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	database.DeleteRow("movies", "dbmovie_id=?", id)

	_, err := database.DeleteRow("dbmovies", "id=?", id)

	handleDBError(ctx, err, StrOK)
}

// @Summary      List the movies of a list
// @Description  Returns the entries of one list (movies table joined with dbmovies): missing flag, quality profile, reached quality, blacklisted and the movie metadata, paginated, with the total count.
// @Tags         movies · data
// @Param        name   path      string  true   "List Name: ex. EN"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.ResultMovies}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/movies/list/{name} [get].
func apiMovieListGet(ctx *gin.Context) {
	listName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)
	query := buildQueryWithWhere(params, "movies.listname = ? COLLATE NOCASE")

	query.InnerJoin = "dbmovies on movies.dbmovie_id=dbmovies.id"

	rows := database.Getdatarow[uint](
		false,
		"select count() from movies where listname = ? COLLATE NOCASE",
		&listName,
	)
	data := database.QueryResultMovies(query, listName)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Look up movie metadata
// @Description  Fetches the metadata of a movie by IMDB ID from all configured sources, or only from the given provider, and returns it. update=1 also saves it to dbmovies.
// @Tags         movies · data
// @Param        imdb   path      string  true   "imdb id: ex. tt123456"
// @Param        provider  query     string  false  "Use only this provider: imdb, tmdb, omdb or trakt; without it all configured providers are combined"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondata{data=database.Dbmovie}  "Movie metadata from the configured providers (not saved)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/movies/metadata/{imdb} [get].
func apiMovieMetadataGet(ctx *gin.Context) {
	var imdb, omdb, tmdb, trakt bool

	var dbmovie database.Dbmovie

	dbmovie.ImdbID = ctx.Param("imdb")
	if queryParam, ok := ctx.GetQuery("provider"); ok {
		switch queryParam {
		case logger.StrImdb:
			imdb = true
		case "omdb":
			omdb = true
		case "tmdb":
			tmdb = true
		case "trakt":
			trakt = true
		}

		metadata.MovieGetMetadata(&dbmovie, imdb, tmdb, omdb, trakt)
	} else {
		metadata.Getmoviemetadata(&dbmovie, true, false, nil, false)
	}

	if queryParam, ok := ctx.GetQuery("update"); ok && queryParam == "1" {
		dbmovie.MovieFindDBIDByImdbParser()

		if dbmovie.ID == 0 {
			dbresult, err := database.ExecNid(
				"insert into dbmovies (Imdb_ID) VALUES (?)",
				&dbmovie.ImdbID,
			)
			if err == nil {
				dbmovie.ID = uint(dbresult) //nolint:gosec // safe: value within target type range
			}
		}

		if dbmovie.ID != 0 {
			if err := database.ExecNErr(
				"update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?",
				&dbmovie.Title,
				&dbmovie.ReleaseDate,
				&dbmovie.Year,
				&dbmovie.Adult,
				&dbmovie.Budget,
				&dbmovie.Genres,
				&dbmovie.OriginalLanguage,
				&dbmovie.OriginalTitle,
				&dbmovie.Overview,
				&dbmovie.Popularity,
				&dbmovie.Revenue,
				&dbmovie.Runtime,
				&dbmovie.SpokenLanguages,
				&dbmovie.Status,
				&dbmovie.Tagline,
				&dbmovie.VoteAverage,
				&dbmovie.VoteCount,
				&dbmovie.TraktID,
				&dbmovie.MoviedbID,
				&dbmovie.ImdbID,
				&dbmovie.FreebaseMID,
				&dbmovie.FreebaseID,
				&dbmovie.FacebookID,
				&dbmovie.InstagramID,
				&dbmovie.TwitterID,
				&dbmovie.URL,
				&dbmovie.Backdrop,
				&dbmovie.Poster,
				&dbmovie.Slug,
				&dbmovie.ID,
			); err != nil {
				logger.Logtype("error", 0).Err(err).Msg("Failed to persist movie metadata")
			}
		}
	}

	ctx.JSON(http.StatusOK, gin.H{"data": dbmovie})
}

// @Summary      Remove a movie from a list
// @Description  Deletes one list entry (movies table). The movie's metadata and its entries in other lists stay. Files on disk are not touched.
// @Tags         movies · data
// @Param        id   path      int  true  "movies (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonok  "Finished"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/movies/list/{id} [delete].
func movieDeleteList(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	_, err := database.DeleteRow("movies", "id=?", id)
	handleDBError(ctx, err, StrOK)
}

const allowedjobsmoviesstr = "rss,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// @Summary      Start a job for all movie groups
// @Description  Queues the given job for every movie media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         movies · jobs
// @Param        job  path      string  true  "Job to start, one of: rss, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/job/{job} [get].
func apimoviesAllJobs(c *gin.Context) {
	dispatchMediaAllJobs(c, mediaJobDispatchConfig{
		TypeName:    logger.StrMovie,
		NamePrefix:  logger.StrMovie,
		AllowedJobs: allowedjobsmoviesstr,
	})
}

// @Summary      Start a job for one movie group
// @Description  Queues the given job for one movie media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         movies · jobs
// @Param        job   path      string  true  "Job to start, one of: rss, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name  path      string  true  "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/job/{job}/{name} [get].
func apimoviesJobs(c *gin.Context) {
	dispatchMediaJob(c, mediaJobDispatchConfig{
		TypeName:    logger.StrMovie,
		NamePrefix:  logger.StrMovie,
		AllowedJobs: allowedjobsmoviesstr,
	})
}

// @Summary      Add or update movie metadata
// @Description  Inserts the dbmovies row when its id is 0 or unknown, otherwise updates it. Returns the number of affected rows.
// @Tags         movies · data
// @Param        movie  body      database.Dbmovie  true  "Movie metadata row (dbmovies); an ID that does not exist yet creates a new row"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "ID of the new row, or the number of updated rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/movies [post].
func updateDBMovie(c *gin.Context) {
	var dbmovie database.Dbmovie
	if !bindJSONWithValidation(c, &dbmovie) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from dbmovies where id != 0 and id = ?",
		&dbmovie.ID,
	)

	var (
		inres sql.Result
		err   error
	)

	if counter == 0 {
		inres, err = database.InsertArray(
			"dbmovies",
			[]string{
				"Title",
				"Release_Date",
				"Year",
				"Adult",
				"Budget",
				"Genres",
				"Original_Language",
				"Original_Title",
				"Overview",
				"Popularity",
				"Revenue",
				"Runtime",
				"Spoken_Languages",
				"Status",
				"Tagline",
				"Vote_Average",
				"Vote_Count",
				"Trakt_ID",
				"Moviedb_ID",
				"Imdb_ID",
				"Freebase_M_ID",
				"Freebase_ID",
				"Facebook_ID",
				"Instagram_ID",
				"Twitter_ID",
				"URL",
				"Backdrop",
				"Poster",
				"Slug",
			},
			dbmovie.Title,
			dbmovie.ReleaseDate,
			dbmovie.Year,
			dbmovie.Adult,
			dbmovie.Budget,
			dbmovie.Genres,
			dbmovie.OriginalLanguage,
			dbmovie.OriginalTitle,
			dbmovie.Overview,
			dbmovie.Popularity,
			dbmovie.Revenue,
			dbmovie.Runtime,
			dbmovie.SpokenLanguages,
			dbmovie.Status,
			dbmovie.Tagline,
			dbmovie.VoteAverage,
			dbmovie.VoteCount,
			dbmovie.TraktID,
			dbmovie.MoviedbID,
			dbmovie.ImdbID,
			dbmovie.FreebaseMID,
			dbmovie.FreebaseID,
			dbmovie.FacebookID,
			dbmovie.InstagramID,
			dbmovie.TwitterID,
			dbmovie.URL,
			dbmovie.Backdrop,
			dbmovie.Poster,
			dbmovie.Slug,
		)
	} else {
		inres, err = database.UpdateArray(
			"dbmovies",
			[]string{
				"Title",
				"Release_Date",
				"Year",
				"Adult",
				"Budget",
				"Genres",
				"Original_Language",
				"Original_Title",
				"Overview",
				"Popularity",
				"Revenue",
				"Runtime",
				"Spoken_Languages",
				"Status",
				"Tagline",
				"Vote_Average",
				"Vote_Count",
				"Trakt_ID",
				"Moviedb_ID",
				"Imdb_ID",
				"Freebase_M_ID",
				"Freebase_ID",
				"Facebook_ID",
				"Instagram_ID",
				"Twitter_ID",
				"URL",
				"Backdrop",
				"Poster",
				"Slug",
			},
			"id != 0 and id = ?",
			dbmovie.Title,
			dbmovie.ReleaseDate,
			dbmovie.Year,
			dbmovie.Adult,
			dbmovie.Budget,
			dbmovie.Genres,
			dbmovie.OriginalLanguage,
			dbmovie.OriginalTitle,
			dbmovie.Overview,
			dbmovie.Popularity,
			dbmovie.Revenue,
			dbmovie.Runtime,
			dbmovie.SpokenLanguages,
			dbmovie.Status,
			dbmovie.Tagline,
			dbmovie.VoteAverage,
			dbmovie.VoteCount,
			dbmovie.TraktID,
			dbmovie.MoviedbID,
			dbmovie.ImdbID,
			dbmovie.FreebaseMID,
			dbmovie.FreebaseID,
			dbmovie.FacebookID,
			dbmovie.InstagramID,
			dbmovie.TwitterID,
			dbmovie.URL,
			dbmovie.Backdrop,
			dbmovie.Poster,
			dbmovie.Slug,
			dbmovie.ID,
		)
	}

	handleDBInsertOrUpdate(c, inres, err, counter == 0)
}

// @Summary      Add or update a movie list entry
// @Description  Inserts the movies (list entry) row when its id is 0 or unknown, otherwise updates it (missing, list, quality profile, blacklisted, ...). Returns the number of affected rows.
// @Tags         movies · data
// @Param        movie  body      database.Movie  true  "Movie list entry (movies table); an ID that does not exist yet creates a new row"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "ID of the new row, or the number of updated rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/movies/list [post].
func updateMovie(c *gin.Context) {
	var movie database.Movie
	if !bindJSONWithValidation(c, &movie) {
		return
	}

	counter := database.Getdatarow[uint](
		false,
		"select count() from movies where id != 0 and id = ?",
		&movie.ID,
	)

	var (
		inres sql.Result
		err   error
	)

	if counter == 0 {
		inres, err = database.InsertArray(
			"movies",
			[]string{
				"missing",
				"listname",
				"dbmovie_id",
				"quality_profile",
				"blacklisted",
				"quality_reached",
				"dont_upgrade",
				"dont_search",
				"rootpath",
			},
			movie.Missing,
			movie.Listname,
			movie.DbmovieID,
			movie.QualityProfile,
			movie.Blacklisted,
			movie.QualityReached,
			movie.DontUpgrade,
			movie.DontSearch,
			movie.Rootpath,
		)
	} else {
		inres, err = database.UpdateArray(
			"movies",
			[]string{
				"missing",
				"listname",
				"dbmovie_id",
				"quality_profile",
				"blacklisted",
				"quality_reached",
				"dont_upgrade",
				"dont_search",
				"rootpath",
			},
			"id != 0 and id = ?",
			movie.Missing,
			movie.Listname,
			movie.DbmovieID,
			movie.QualityProfile,
			movie.Blacklisted,
			movie.QualityReached,
			movie.DontUpgrade,
			movie.DontSearch,
			movie.Rootpath,
			movie.ID,
		)
	}

	handleDBInsertOrUpdate(c, inres, err, counter == 0)
}

// @Summary      Search a movie and download
// @Description  Queues a search of the indexers for one list entry (missing or upgrade, by ID and title) and downloads the best accepted release. Returns immediately; the work runs in the job queue.
// @Tags         movies · search
// @Param        id   path      int  true  "movies (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Movie not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/search/id/{id} [get].
func apimoviesSearch(c *gin.Context) {
	movie, _ := database.GetMovies(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	if movie == nil {
		sendNotFound(c, "Movie not found")
		return
	}
	// defer logger.ClearVar(&movie)
	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, movie.Listname) {
				found = true

				if err := worker.Dispatch(
					"searchmovie_movies_"+media.Name+"_"+strconv.Itoa(
						int(movie.ID),
					),
					func(_ uint32, ctx context.Context) error {
						searchvar := searcher.NewSearcher(media, nil, "", nil)

						err := searchvar.MediaSearch(ctx, media, movie.ID, true, false, false)
						if err != nil {
							if err != nil && !errors.Is(err, logger.ErrDisabled) {
								logger.Logtype("error", 2).
									Any("id", &movie.ID).
									Str("typ", logger.StrMovie).
									Err(err).
									Msg("Search Failed")
							}
						} else {
							if len(searchvar.Accepted) >= 1 {
								searchvar.Download()
							}
						}

						searchvar.Close()

						return err
					},
					"Search",
				); err != nil {
					sendDispatchError(c, err)
					return nil
				}

				sendSuccess(c, StrStarted)

				return nil
			}
		}

		return nil
	})

	if !found {
		sendJSONError(c, http.StatusNoContent, StrNothingDone)
	}
}

// @Summary      Search a movie and show the results
// @Description  Searches the indexers for one list entry and waits for the result: returns the accepted and denied releases with the reasons. download=true also downloads the best accepted release.
// @Tags         movies · search
// @Param        id             path      int     true   "movies (list entry) ID"
// @Param        searchByTitle  query     string  false  "true: also search by title, not only by IMDB ID"
// @Param        download       query     bool    false  "true: download the best accepted release"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/movies/search/list/{id} [get].
func apimoviesSearchList(c *gin.Context) {
	searchListEntry(c, logger.StrMovie, "movies")
}

// @Summary      Preview the RSS search of a movie group
// @Description  Reads the indexers' RSS feeds for one movie media group and returns the accepted and denied releases. Nothing is downloaded.
// @Tags         movies · search
// @Param        group  path      string  true  "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/movies/rss/search/list/{group} [get].
func apiMoviesRssSearchList(c *gin.Context) {
	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}

		if strings.EqualFold(media.Name, c.Param("group")) {
			found = true

			ctx := context.Background()
			searchresults := searcher.NewSearcher(media, media.CfgQuality, logger.StrRss, nil)

			err := searchresults.SearchRSS(ctx, media, media.CfgQuality, false, false)
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

// @Summary      Download a chosen release for a movie
// @Description  Sends the given release (e.g. one from /search/list) for the list entry to the download client of the indexer's quality profile entry.
// @Tags         movies · search
// @Param        nzb  body      apiexternal_v2.Nzbwithprio  true  "Nzb: Req. Title, Indexer, imdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "ID of the list entry in the movies table"
// @Param        indexer query  string                  true  "Indexer config name the release came from"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id or body, or IDs do not match"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Movie not found"
// @Failure      500  {object}  Jsonerror  "Download failed"
// @Router       /api/movies/search/download/{id} [post].
func apimoviesSearchDownload(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param(logger.StrID), 10, 64)
	if err != nil || id == 0 {
		sendBadRequest(c, "Invalid movie id")
		return
	}

	movie, err := database.GetMovies(database.Querywithargs{Where: logger.FilterByID}, uint(id))
	if err != nil || movie == nil || movie.ID == 0 {
		sendNotFound(c, "Movie not found")
		return
	}

	var nzb apiexternal_v2.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if nzb.NzbmovieID != 0 && nzb.NzbmovieID != movie.ID {
		sendBadRequest(c, "nzb_movie_id does not match the movie id in the path")
		return
	}

	nzb.NzbmovieID = movie.ID

	if !setManualDownloadIndexer(c, &nzb) {
		return
	}

	found := false

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}

		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, movie.Listname) {
				if err := downloader.DownloadMovie(media, &nzb); err != nil {
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

// @Summary      Refresh the metadata of all movies
// @Description  Queues a metadata refresh of every movie in the lists of all movie media groups. Returns immediately; the work runs in the job queue.
// @Tags         movies · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/all/refreshall [get].
func apirefreshMovies(c *gin.Context) {
	dispatchJobAllConfigs(c, logger.StrMovie, logger.StrMovie, "refresh", StrStarted)
}

// @Summary      Refresh the metadata of one movie
// @Description  Queues a metadata refresh of one movie from the configured sources. Returns immediately; the work runs in the job queue.
// @Tags         movies · jobs
// @Param        id   path      int  true  "ID of the movie in the dbmovies table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/refresh/{id} [get].
func apirefreshMovie(c *gin.Context) {
	id := c.Param(logger.StrID)

	// Find this movie's list so we pick the movie config that actually owns it.
	// There may be several movie configs; picking the first one would fail with
	// "listid not set" when the movie belongs to a different config's list.
	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM movies WHERE dbmovie_id = ? LIMIT 1",
		&id,
	)

	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return false
		}

		if cfgp == nil {
			cfgp = media // fall back to the first movie config
		}

		if listname != "" && media.GetMediaListsEntryListID(listname) != -1 {
			cfgp = media // this config owns the movie's list
			return true
		}

		return false
	})

	if cfgp == nil {
		sendNotFound(c, "No movie configuration found")
		return
	}

	if err := worker.Dispatch(
		"Refresh Single Movie_"+id,
		func(_ uint32, _ context.Context) error {
			return utils.RefreshMovie(cfgp, &id)
		},
		"Feeds",
	); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, StrStarted)
}

// @Summary      Refresh movie metadata (incremental)
// @Description  Queues a metadata refresh of the 100 least recently updated movies of each movie media group. Returns immediately; the work runs in the job queue.
// @Tags         movies · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/all/refresh [get].
func apirefreshMoviesInc(c *gin.Context) {
	dispatchJobAllConfigs(c, logger.StrMovie, logger.StrMovie, "refreshinc", StrStarted)
}

// @Summary      Clear the download history of a group or list
// @Description  Queues deleting the download history of a movie media group (or of one of its lists), so its releases can be downloaded again. Returns immediately; the work runs in the job queue.
// @Tags         movies · search
// @Param        name  path      string  true  "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/movies/search/history/clear/{name} [get].
func apimoviesClearHistoryName(c *gin.Context) {
	dispatchClearHistory(c, logger.StrMovie, logger.StrMovie, c.Param(StrName))
}

// @Summary      Clear the download history of a movie
// @Description  Deletes all download history rows of one list entry, so its releases can be downloaded again. Returns the number of deleted rows.
// @Tags         movies · search
// @Param        id   path      string  true  "movies (list entry) ID"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "Number of deleted history rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/movies/search/history/clearid/{id} [get].
func apiMoviesClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	inres, inerr := database.DeleteRow("movie_histories", "movie_id = ?", id)
	handleDBInsertOrUpdate(c, inres, inerr, false)
}

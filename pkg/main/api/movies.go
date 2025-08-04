// movies
package api

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloader"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	gin "github.com/gin-gonic/gin"
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

// @Summary      List Movies
// @Description  List Movies
// @Tags         movie
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.Dbmovie}
// @Failure      401    {object}  Jsonerror
// @Router       /api/movies [get].
func apiMovieList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)
	
	rows := database.Getdatarow[uint](false, "select count() from dbmovies")
	data := database.QueryDbmovie(query)
	
	sendJSONResponse(ctx, http.StatusOK, data, int(rows))
}

// @Summary      List Unmatched Movies
// @Description  List Unmatched Movies
// @Tags         movie
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.MovieFileUnmatched}
// @Failure      401    {object}  Jsonerror
// @Router       /api/movies/unmatched [get].
func apiMovieListUnmatched(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	query := buildQuery(params)
	
	rows := database.Getdatarow[uint](false, "select count() from movie_file_unmatcheds")
	data := database.QueryMovieFileUnmatched(query)
	
	sendJSONResponse(ctx, http.StatusOK, data, int(rows))
}

// @Summary      Delete Movies
// @Description  Deletes Movies from all lists
// @Tags         movie
// @Param        id   path      int  true  "Movie ID: ex. 1"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
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

// @Summary      List Movies (List)
// @Description  Lists Movies from a list
// @Tags         movie
// @Param        name   path      string  true   "List Name: ex. EN"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondatarows{data=[]database.ResultMovies}
// @Failure      401    {object}  Jsonerror
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

	sendJSONResponse(ctx, http.StatusOK, data, int(rows))
}

// @Summary      Get Movie Metadata
// @Description  Gets metadata of a movie (for testing)
// @Tags         movie
// @Param        imdb   path      string  true   "imdb id: ex. tt123456"
// @Param        provider  query     string  false  "Provider: imdb,tmdb,omdb,trakt"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}   Jsondata{data=database.Dbmovie}
// @Failure      401    {object}  Jsonerror
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
				dbmovie.ID = uint(dbresult)
			}
		}
		database.ExecN(
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
		)
	}
	ctx.JSON(http.StatusOK, gin.H{"data": dbmovie})
}

// @Summary      Delete a Movie (List)
// @Description  Deletes a Movie from a list
// @Tags         movie
// @Param        id   path      int  true  "Movie ID: ex. 1"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns ok"
// @Failure      401  {object}  Jsonerror
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

// @Summary      Start Jobs (All Lists)
// @Description  Starts a Job
// @Tags         movie
// @Param        job  path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns job name started"
// @Failure      204  {object}  string "error message"
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/job/{job} [get].
func apimoviesAllJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsmoviesstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"

		// defer cfgMovie.Close()
		// defer cfg_list.Close()
		config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
			if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
				return nil
			}

			cfgpstr := media.NamePrefix

			switch c.Param(StrJobLower) {
			case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
				worker.Dispatch(c.Param(StrJobLower)+"_"+cfgpstr, func(key uint32) error {
					return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
				}, "Data")
			case logger.StrSearchMissingFull,
				logger.StrSearchMissingInc,
				logger.StrSearchUpgradeFull,
				logger.StrSearchUpgradeInc,
				logger.StrSearchMissingFullTitle,
				logger.StrSearchMissingIncTitle,
				logger.StrSearchUpgradeFullTitle,
				logger.StrSearchUpgradeIncTitle:
				worker.Dispatch(c.Param(StrJobLower)+"_"+cfgpstr, func(key uint32) error {
					return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
				}, "Search")
			case logger.StrRss:
				worker.Dispatch(c.Param(StrJobLower)+"_"+cfgpstr, func(key uint32) error {
					return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
				}, "RSS")
			case logger.StrFeeds,
				logger.StrCheckMissing,
				logger.StrCheckMissingFlag,
				logger.StrReachedFlag:
				var err error
				for idxi := range media.Lists {
					if !media.Lists[idxi].Enabled {
						continue
					}
					if media.Lists[idxi].CfgList == nil {
						continue
					}

					if !config.GetSettingsList(media.Lists[idxi].TemplateList).Enabled {
						continue
					}
					listname := media.Lists[idxi].Name
					if c.Param(StrJobLower) == logger.StrFeeds {
						if errsub := worker.Dispatch(
							c.Param(StrJobLower)+"_"+cfgpstr+"_"+listname,
							func(key uint32) error {
								return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, listname, true, key)
							},
							"Feeds",
						); errsub != nil {
							err = errsub
						}
					} else {
						if errsub := worker.Dispatch(
							c.Param(StrJobLower)+"_"+cfgpstr+"_"+listname,
							func(key uint32) error {
								return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, listname, true, key)
							},
							"Data",
						); errsub != nil {
							err = errsub
						}
					}
					if c.Param(StrJobLower) == logger.StrCheckMissing ||
						c.Param(StrJobLower) == logger.StrCheckMissingFlag ||
						c.Param(StrJobLower) == logger.StrReachedFlag {
						if errsub := worker.Dispatch(
							c.Param(StrJobLower)+"_"+cfgpstr+"_"+listname,
							func(key uint32) error {
								return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, listname, true, key)
							},
							"Data",
						); errsub != nil {
							err = errsub
						}
					}
					// cfg_list.Close()
				}
				return err
			case "refresh":
				return worker.Dispatch(logger.StrRefreshMovies, func(key uint32) error {
					return utils.SingleJobs("refresh", cfgpstr, "", false, key)
				}, "Feeds")
			case "refreshinc":
				return worker.Dispatch(logger.StrRefreshMoviesInc, func(key uint32) error {
					return utils.SingleJobs("refreshinc", cfgpstr, "", false, key)
				}, "Feeds")
			case "":
				return nil
			default:
				return worker.Dispatch(c.Param(StrJobLower)+"_"+cfgpstr, func(key uint32) error {
					return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
				}, "Data")
			}
			return nil
		})
		sendSuccess(c, returnval)
}

// @Summary      Start Jobs
// @Description  Starts a Job
// @Tags         movie
// @Param        job   path      string  true  "Job Name one of: rss, data, datafull, checkmissing, checkmissingflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name  path      string  true  "List Name: ex. list"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns job name started"
// @Failure      204  {object}  string "error message"
// @Failure      401   {object}  Jsonerror
// @Router       /api/movies/job/{job}/{name} [get].
func apimoviesJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsmoviesstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
		cfgpstr := "movie_" + c.Param("name")

		switch c.Param(StrJobLower) {
		case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
			worker.Dispatch(c.Param(StrJobLower)+"_movies_"+c.Param("name"), func(key uint32) error {
				return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
			}, "Data")
		case logger.StrSearchMissingFull,
			logger.StrSearchMissingInc,
			logger.StrSearchUpgradeFull,
			logger.StrSearchUpgradeInc,
			logger.StrSearchMissingFullTitle,
			logger.StrSearchMissingIncTitle,
			logger.StrSearchUpgradeFullTitle,
			logger.StrSearchUpgradeIncTitle:
			worker.Dispatch(c.Param(StrJobLower)+"_movies_"+c.Param("name"), func(key uint32) error {
				return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
			}, "Search")
		case logger.StrRss:
			worker.Dispatch(c.Param(StrJobLower)+"_movies_"+c.Param("name"), func(key uint32) error {
				return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
			}, "RSS")
		case logger.StrFeeds,
			logger.StrCheckMissing,
			logger.StrCheckMissingFlag,
			logger.StrReachedFlag:

			// defer cfgMovie.Close()
			// defer cfg_list.Close()
			config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
				if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
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
							logger.LogDynamicany2StrAny(
								"debug",
								"add job",
								logger.StrTitle,
								media.Name,
								"List",
								&media.Lists[idxlist].Name,
							)
							worker.Dispatch(
								c.Param(
									StrJobLower,
								)+"_movies_"+media.Name+"_"+media.Lists[idxlist].Name,
								func(key uint32) error {
									return utils.SingleJobs(
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
								)+"_movies_"+media.Name+"_"+media.Lists[idxlist].Name,
								func(key uint32) error {
									return utils.SingleJobs(
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
						// cfg_list.Close()
					}
				}
				// cfgMovie.Close()
				return nil
			})
		case "refresh":
			worker.Dispatch(logger.StrRefreshMovies, func(key uint32) error {
				return utils.SingleJobs("refresh", cfgpstr, "", false, key)
			}, "Feeds")
		case "refreshinc":
			worker.Dispatch(logger.StrRefreshMoviesInc, func(key uint32) error {
				return utils.SingleJobs("refreshinc", cfgpstr, "", false, key)
			}, "Feeds")
		case "":
			break
		default:
			worker.Dispatch(c.Param(StrJobLower)+"_movies_"+c.Param("name"), func(key uint32) error {
				return utils.SingleJobs(c.Param(StrJobLower), cfgpstr, "", true, key)
			}, "Data")
		}
		sendSuccess(c, returnval)
}

// @Summary      Update Movie (Global)
// @Description  Updates or creates a movie
// @Tags         movie
// @Param        movie  body      database.Dbmovie  true  "Movie"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}  int64
// @Failure      403    {object}  error
// @Failure      400    {object}  Jsonerror
// @Failure      401    {object}  Jsonerror
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
	var inres sql.Result
	var err error
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
		inres, err = database.UpdateArray("dbmovies", []string{"Title", "Release_Date", "Year", "Adult", "Budget", "Genres", "Original_Language", "Original_Title", "Overview", "Popularity", "Revenue", "Runtime", "Spoken_Languages", "Status", "Tagline", "Vote_Average", "Vote_Count", "Trakt_ID", "Moviedb_ID", "Imdb_ID", "Freebase_M_ID", "Freebase_ID", "Facebook_ID", "Instagram_ID", "Twitter_ID", "URL", "Backdrop", "Poster", "Slug"},
			"id != 0 and id = ?", dbmovie.Title, dbmovie.ReleaseDate, dbmovie.Year, dbmovie.Adult, dbmovie.Budget, dbmovie.Genres, dbmovie.OriginalLanguage, dbmovie.OriginalTitle, dbmovie.Overview, dbmovie.Popularity, dbmovie.Revenue, dbmovie.Runtime, dbmovie.SpokenLanguages, dbmovie.Status, dbmovie.Tagline, dbmovie.VoteAverage, dbmovie.VoteCount, dbmovie.TraktID, dbmovie.MoviedbID, dbmovie.ImdbID, dbmovie.FreebaseMID, dbmovie.FreebaseID, dbmovie.FacebookID, dbmovie.InstagramID, dbmovie.TwitterID, dbmovie.URL, dbmovie.Backdrop, dbmovie.Poster, dbmovie.Slug, dbmovie.ID)
	}
	handleDBInsertOrUpdate(c, inres, err, counter == 0)
}

// @Summary      Update Movie (List)
// @Description  Updates or creates a movie in a list
// @Tags         movie
// @Param        movie  body      database.Movie  true  "Movie"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}  int64
// @Failure      403    {object}  error
// @Failure      400    {object}  Jsonerror
// @Failure      401    {object}  Jsonerror
// @Router       /api/movies/list [post].
func updateMovie(c *gin.Context) {
	var movie database.Movie
	if !bindJSONWithValidation(c, &movie) {
		return
	}
	counter := database.Getdatarow[uint](
		false,
		"select count() from dbmovies where id != 0 and id = ?",
		&movie.ID,
	)
	var inres sql.Result
	var err error
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
		inres, err = database.UpdateArray("dbmovies", []string{"missing", "listname", "dbmovie_id", "quality_profile", "blacklisted", "quality_reached", "dont_upgrade", "dont_search", "rootpath"},
			"id != 0 and id = ?", movie.Missing, movie.Listname, movie.DbmovieID, movie.QualityProfile, movie.Blacklisted, movie.QualityReached, movie.DontUpgrade, movie.DontSearch, movie.Rootpath, movie.ID)
	}
	handleDBInsertOrUpdate(c, inres, err, counter == 0)
}

// @Summary      Search a movie
// @Description  Searches for upgrades and missing
// @Tags         movie
// @Param        id   path      int  true  "Movie ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      204  {object}  string "nothing done"
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/search/id/{id} [get].
func apimoviesSearch(c *gin.Context) {
	movie, _ := database.GetMovies(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	// defer logger.ClearVar(&movie)
	var idxlist int
	var err error
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}

		for idxlist = range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, movie.Listname) {
				worker.Dispatch(
					"searchmovie_movies_"+media.Name+"_"+strconv.Itoa(int(movie.ID)),
					func(uint32) error {
						ctx := context.Background()
						searchvar := searcher.NewSearcher(media, nil, "", nil)
						err = searchvar.MediaSearch(ctx, media, movie.ID, true, false, false)

						if err != nil {
							if err != nil && !errors.Is(err, logger.ErrDisabled) {
								logger.LogDynamicany(
									"error",
									"Search Failed",
									"id",
									&movie.ID,
									"typ",
									&logger.StrMovie,
									err,
								)
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
				)
				sendSuccess(c, StrStarted)
				return nil
			}
		}
		return nil
	})
	sendJSONError(c, http.StatusNoContent, StrNothingDone)
}

// @Summary      Search a movie (List ok, nok)
// @Description  Searches for upgrades and missing
// @Tags         movie
// @Param        id             path      int     true   "Movie ID"
// @Param        searchByTitle  query     string  false  "searchByTitle"
// @Param        download       query     bool    false  "download"
// @Param        apikey query     string    true  "apikey"
// @Success      200            {object}  Jsonresults
// @Failure      401            {object}  string
// @Router       /api/movies/search/list/{id} [get].
func apimoviesSearchList(c *gin.Context) {
	movie, _ := database.GetMovies(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	// defer logger.ClearVar(&movie)

	titlesearch := false
	if queryParam, ok := c.GetQuery("searchByTitle"); ok {
		if queryParam == "true" || queryParam == "yes" {
			titlesearch = true
		}
	}
	var err error
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, movie.Listname) {
				ctx := context.Background()
				searchresults := searcher.NewSearcher(media, nil, "", nil)
				err = searchresults.MediaSearch(ctx, media, movie.ID, titlesearch, false, false)
				if err != nil {
					str := "failed with " + err.Error()
					c.JSON(http.StatusNotFound, str)
					searchresults.Close()
					return err
				}
				if _, ok := c.GetQuery("download"); ok {
					searchresults.Download()
				}
				c.JSON(
					http.StatusOK,
					gin.H{"accepted": searchresults.Accepted, "denied": searchresults.Denied},
				)
				// searchnow.Close()
				searchresults.Close()
				return nil
			}
		}
		return nil
	})
	sendJSONError(c, http.StatusNoContent, StrNothingDone)
}

// @Summary      Movie RSS (list ok, nok)
// @Description  Movie RSS
// @Tags         movie
// @Param        group  path      string  true  "Group Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200    {object}  Jsonresults
// @Failure      401    {object}  Jsonerror
// @Router       /api/movies/rss/search/list/{group} [get].
func apiMoviesRssSearchList(c *gin.Context) {
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}
		if strings.EqualFold(media.Name, c.Param("group")) {
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
	sendJSONError(c, http.StatusNoContent, StrNothingDone)
}

// @Summary      Download a movie (manual)
// @Description  Downloads a release after select
// @Tags         movie
// @Param        nzb  body      apiexternal.Nzbwithprio  true  "Nzb: Req. Title, Indexer, imdbid, downloadurl, parseinfo"
// @Param        id   path      int                     true  "Movie ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      204  {object}  string "nothing done"
// @Failure      400  {object}  Jsonerror
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/search/download/{id} [post].
func apimoviesSearchDownload(c *gin.Context) {
	movie, _ := database.GetMovies(
		database.Querywithargs{Where: logger.FilterByID},
		c.Param(logger.StrID),
	)
	// defer logger.ClearVar(&movie)

	var nzb apiexternal.Nzbwithprio
	if err := c.ShouldBindJSON(&nzb); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	// defer logger.ClearVar(&nzb)
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, logger.StrMovie) {
			return nil
		}
		for idxlist := range media.Lists {
			if strings.EqualFold(media.Lists[idxlist].Name, movie.Listname) {
				downloader.DownloadMovie(media, &nzb)
				sendSuccess(c, StrStarted)
				return nil
			}
		}
		return nil
	})
	sendJSONError(c, http.StatusNoContent, StrNothingDone)
}

// @Summary      Refresh Movies
// @Description  Refreshes Movie Metadata
// @Tags         movie
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/all/refreshall [get].
func apirefreshMovies(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if media.NamePrefix[:5] == logger.StrMovie {
			cfgp = media
			return true
		}
		return false
	})
	worker.Dispatch(logger.StrRefreshMovies, func(key uint32) error {
		return utils.SingleJobs("refresh", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Refresh a Movie
// @Description  Refreshes specific Movie Metadata
// @Tags         movie
// @Param        id   path      int  true  "Movie ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/refresh/{id} [get].
func apirefreshMovie(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if media.NamePrefix[:5] == logger.StrMovie {
			cfgp = media
			return true
		}
		return false
	})
	id := c.Param(logger.StrID)
	worker.Dispatch("Refresh Single Movie_"+c.Param(logger.StrID), func(uint32) error {
		return utils.RefreshMovie(cfgp, &id)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Refresh Movies (Incremental)
// @Description  Refreshes Movie Metadata
// @Tags         movie
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/all/refresh [get].
func apirefreshMoviesInc(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if media.NamePrefix[:5] == logger.StrMovie {
			cfgp = media
			return true
		}
		return false
	})
	worker.Dispatch(logger.StrRefreshMoviesInc, func(key uint32) error {
		return utils.SingleJobs("refreshinc", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Clear History (Full List)
// @Description  Clear Movies Download History
// @Tags         movie
// @Param        name  path      string  true  "List Name"
// @Param        apikey query     string    true  "apikey"
// @Success      200   {object}  string "returns started"
// @Failure      401   {object}  Jsonerror
// @Router       /api/movies/search/history/clear/{name} [get].
func apimoviesClearHistoryName(c *gin.Context) {
	utils.SingleJobs(logger.StrClearHistory, "movie_"+c.Param("name"), "", true, 0)
	sendSuccess(c, StrStarted)
}

// @Summary      Clear History (Single Item)
// @Description  Clear Episode Download History
// @Tags         movie
// @Param        id   path      string  true  "Movie ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  int64
// @Failure      403  {object}  error
// @Failure      401  {object}  Jsonerror
// @Router       /api/movies/search/history/clearid/{id} [get].
func apiMoviesClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	inres, inerr := database.DeleteRow("movie_histories", "movie_id = ?", id)
	handleDBInsertOrUpdate(c, inres, inerr, false)
}

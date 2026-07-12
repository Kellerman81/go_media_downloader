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

// allowed jobs for music.
const allowedjobsmusicstr = "rss,rssartists,rssartistsupgrade,data,datafull,checkmissing,checkmissingflag,checkreachedflag,structure,searchmissingfull,searchmissinginc,searchupgradefull,searchupgradeinc,searchmissingfulltitle,searchmissinginctitle,searchupgradefulltitle,searchupgradeinctitle,clearhistory,feeds,refresh,refreshinc"

// AddMusicRoutes adds routes for music/album management.
func AddMusicRoutes(routermusic *gin.RouterGroup) {
	routermusic.Use(checkauth)
	{
		routermusic.GET("/all/refresh", apirefreshMusicInc)
		routermusic.GET("/all/refreshall", apirefreshMusic)
		routermusic.GET("/refresh/:id", apirefreshAlbum)

		routermusic.GET("/tag/all", apiRetagAllAlbums)
		routermusic.GET("/tag/artist/:id", apiRetagArtistAlbums)
		routermusic.GET("/tag/:id", apiRetagAlbum)

		routermusic.GET("/", apiMusicDBList)
		routermusic.GET("/list/:name", apiMusicListGet)
		routermusic.DELETE("/:id", apiMusicDelete)

		// Artist routes
		routermusic.GET("/artists", apiArtistsList)
		routermusic.GET("/artist/add/:name/:listname", apiMusicAddArtist)

		routermusic.GET("/job/:job", apiMusicAllJobs)
		routermusic.GET("/job/:job/:name", apiMusicJobs)

		routermusic.GET("/feeds/date/:name/:listname", apiMusicFeedsDate)

		routermusic.GET("/rss/search/list/:group", apiMusicRssSearchList)

		routermusic.GET("/discover/series/artist/:id", apiMusicDiscoverSeriesForArtist)

		routermusicsearch := routermusic.Group("/search")
		{
			routermusicsearch.GET("/list/:id", apiMusicSearchList)
			routermusicsearch.GET("/history/clear/:name", apiMusicClearHistoryName)
			routermusicsearch.GET("/history/clearid/:id", apiMusicClearHistoryID)
			routermusicsearch.GET("/artists/missing/:name", apiMusicSearchArtistsMissing)
			routermusicsearch.GET("/artists/upgrade/:name", apiMusicSearchArtistsUpgrade)
		}
	}
}

// @Summary      List Albums (Database)
// @Description  List all albums from the database
// @Tags         music
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/music [get].
func apiMusicDBList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from dbalbums")

	query := "select id,created_at,updated_at,title,musicbrainz_release_group_id,musicbrainz_release_id,discogs_master_id,discogs_release_id,spotify_id,upc,release_date,release_type,format,label,country,total_tracks,total_runtime_ms,genres,styles,cover_url,year,slug from dbalbums"
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

	data := database.StructscanT[database.Dbalbum](false, params.Limit, query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      List Albums by List Name
// @Description  List albums filtered by list name
// @Tags         music
// @Param        name   path      string  true   "List Name"
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/list/{name} [get].
func apiMusicListGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from albums where listname = ?", &name)

	query := "select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbalbum_id,artist_id from albums where listname = ?"
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

	data := database.StructscanT[database.Album](false, params.Limit, query, &name)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Delete Album
// @Description  Deletes an album from the database
// @Tags         music
// @Param        id     path      int     true   "Album ID"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  gin.H
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/{id} [delete].
func apiMusicDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Delete album files first
	database.ExecN("DELETE FROM album_files WHERE album_id = ?", &id)
	// Delete the album
	database.ExecN("DELETE FROM albums WHERE id = ?", &id)

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// @Summary      List Artists
// @Description  List all artists from the database
// @Tags         music
// @Param        limit  query     int     false  "Limit"
// @Param        page   query     int     false  "Page"
// @Param        order  query     string  false  "Order By"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  Jsondatarows
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/artists [get].
func apiArtistsList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from dbartists")

	query := "select id,created_at,updated_at,name,sort_name,musicbrainz_id,discogs_id,spotify_id,artist_type,country,begin_date,end_date,disambiguation,bio,image_url,genres from dbartists"
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

	data := database.StructscanT[database.Dbartist](false, params.Limit, query)

	sendJSONResponse(
		ctx,
		http.StatusOK,
		data,
		int(rows),
	)
}

// @Summary      Start All Music Jobs
// @Description  Starts a Job for all music configurations
// @Tags         music
// @Param        job    path      string  true   "Job Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string  "returns job name started"
// @Failure      204    {object}  string  "error message"
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/job/{job} [get].
func apiMusicAllJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsmusicstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, "music") {
			return nil
		}

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

				queueName := "Data"
				if c.Param(StrJobLower) == logger.StrFeeds {
					queueName = "Feeds"
				}

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

			return err

		case "refresh":
			return worker.Dispatch(
				"refresh_music",
				func(key uint32, ctx context.Context) error {
					return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
				},
				"Feeds",
			)

		case "refreshinc":
			return worker.Dispatch(
				"refreshinc_music",
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
	sendSuccess(c, returnval)
}

// @Summary      Start Music Jobs
// @Description  Starts a Job for a specific music configuration
// @Tags         music
// @Param        job    path      string  true   "Job Name"
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string  "returns job name started"
// @Failure      204    {object}  string  "error message"
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/job/{job}/{name} [get].
func apiMusicJobs(c *gin.Context) {
	jobParam := c.Param(StrJobLower)
	if !validateJobParam(jobParam, allowedjobsmusicstr) {
		sendJSONError(c, http.StatusNoContent, "Job "+jobParam+" not allowed!")
		return
	}

	returnval := "Job " + jobParam + " started"
	cfgpstr := "music_" + c.Param("name")

	switch c.Param(StrJobLower) {
	case "data", logger.StrDataFull, logger.StrStructure, logger.StrClearHistory:
		worker.Dispatch(
			c.Param(StrJobLower)+"_music_"+c.Param("name"),
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
			c.Param(StrJobLower)+"_music_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Search",
		)

	case logger.StrRss:
		worker.Dispatch(
			c.Param(StrJobLower)+"_music_"+c.Param("name"),
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
			if !strings.HasPrefix(media.NamePrefix, "music") {
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
							c.Param(StrJobLower)+"_music_"+media.Name+"_"+media.Lists[idxlist].Name,
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
							c.Param(StrJobLower)+"_music_"+media.Name+"_"+media.Lists[idxlist].Name,
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
			"refresh_music_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refresh", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "refreshinc":
		worker.Dispatch(
			"refreshinc_music_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, "refreshinc", cfgpstr, "", false, key)
			},
			"Feeds",
		)

	case "":
		break
	default:
		worker.Dispatch(
			c.Param(StrJobLower)+"_music_"+c.Param("name"),
			func(key uint32, ctx context.Context) error {
				return utils.SingleJobs(ctx, c.Param(StrJobLower), cfgpstr, "", true, key)
			},
			"Data",
		)
	}

	sendSuccess(c, returnval)
}

// @Summary      RSS Search Music List
// @Description  Trigger RSS search for music list
// @Tags         music
// @Param        group  path      string  true   "Group Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/rss/search/list/{group} [get].
func apiMusicRssSearchList(c *gin.Context) {
	cfgpstr := "music_" + c.Param("group")
	worker.Dispatch(
		"rss_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRss, cfgpstr, "", true, key)
		},
		"RSS",
	)
	sendSuccess(c, "RSS search started for "+cfgpstr)
}

// @Summary      Search Albums by List
// @Description  Search for all albums in a list
// @Tags         music
// @Param        id     path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/search/list/{id} [get].
func apiMusicSearchList(c *gin.Context) {
	listName := c.Param(StrID)
	cfgpstr := "music_" + listName
	worker.Dispatch(
		"searchlist_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrSearchMissingInc, cfgpstr, "", true, key)
		},
		"Search",
	)
	sendSuccess(c, "Search started for music list "+listName)
}

// @Summary      Clear Music History by Name
// @Description  Clear search history for music list
// @Tags         music
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/search/history/clear/{name} [get].
func apiMusicClearHistoryName(c *gin.Context) {
	name := c.Param("name")
	database.ExecN("DELETE FROM album_histories WHERE listname = ?", &name)
	sendSuccess(c, "History cleared for "+name)
}

// @Summary      Clear Music History by ID
// @Description  Clear search history entry by ID
// @Tags         music
// @Param        id     path      int     true   "History ID"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/search/history/clearid/{id} [get].
func apiMusicClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	database.ExecN("DELETE FROM album_histories WHERE id = ?", &id)
	sendSuccess(c, "History entry cleared")
}

// @Summary      Search Missing Albums by Artist
// @Description  Search for missing albums by artist name (up to 20 random artists)
// @Tags         music
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/search/artists/missing/{name} [get].
func apiMusicSearchArtistsMissing(c *gin.Context) {
	listName := c.Param("name")
	cfgpstr := "music_" + listName
	worker.Dispatch(
		"searchartists_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRssArtists, cfgpstr, "", true, key)
		},
		"Artist Search",
	)
	sendSuccess(c, "Artist search (missing) started for music list "+listName)
}

// @Summary      Search Upgrade Albums by Artist
// @Description  Search for albums needing quality upgrade by artist name (up to 20 random artists)
// @Tags         music
// @Param        name   path      string  true   "List Name"
// @Param        apikey query     string  true   "apikey"
// @Success      200    {object}  string
// @Failure      401    {object}  Jsonerror
// @Router       /api/music/search/artists/upgrade/{name} [get].
func apiMusicSearchArtistsUpgrade(c *gin.Context) {
	listName := c.Param("name")
	cfgpstr := "music_" + listName
	worker.Dispatch(
		"searchartistsupgrade_"+cfgpstr,
		func(key uint32, ctx context.Context) error {
			return utils.SingleJobs(ctx, logger.StrRssArtistsUpgrade, cfgpstr, "", true, key)
		},
		"Artist Search",
	)
	sendSuccess(c, "Artist search (upgrade) started for music list "+listName)
}

// @Summary      Refresh Music
// @Description  Refreshes Album Metadata from MusicBrainz
// @Tags         music
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/music/all/refreshall [get].
func apirefreshMusic(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, "music") {
			cfgp = media
			return true
		}

		return false
	})
	worker.Dispatch(logger.StrRefreshMusic, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, "refresh", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Refresh a single Album
// @Description  Refreshes specific Album Metadata from MusicBrainz
// @Tags         music
// @Param        id   path      int  true  "Album ID"
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/music/refresh/{id} [get].
func apirefreshAlbum(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, "music") {
			cfgp = media
			return true
		}

		return false
	})

	id := c.Param(logger.StrID)
	worker.Dispatch(
		"Refresh Single Album_"+id,
		func(_ uint32, _ context.Context) error {
			return utils.RefreshAlbum(cfgp, &id)
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

// @Summary      Refresh Music (Incremental)
// @Description  Refreshes Album Metadata from MusicBrainz
// @Tags         music
// @Param        apikey query     string    true  "apikey"
// @Success      200  {object}  string "returns started"
// @Failure      401  {object}  Jsonerror
// @Router       /api/music/all/refresh [get].
func apiRetagAlbum(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, "music") {
			cfgp = media
			return true
		}

		return false
	})

	id := c.Param(logger.StrID)
	worker.Dispatch(
		"Retag Album_"+id,
		func(_ uint32, _ context.Context) error {
			dbID, _ := strconv.ParseUint(id, 10, 32)
			return utils.RetagAlbum(c, cfgp, uint(dbID))
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

func apiRetagArtistAlbums(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, "music") {
			cfgp = media
			return true
		}

		return false
	})

	id := c.Param(logger.StrID)
	worker.Dispatch(
		"Retag Artist Albums_"+id,
		func(_ uint32, _ context.Context) error {
			artistID, _ := strconv.ParseUint(id, 10, 32)
			return utils.RetagArtistAlbums(c, cfgp, uint(artistID))
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

func apiRetagAllAlbums(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, "music") {
			cfgp = media
			return true
		}

		return false
	})

	worker.Dispatch(
		"Retag All Albums",
		func(_ uint32, _ context.Context) error {
			return utils.RetagAllAlbums(c, cfgp)
		},
		"Feeds",
	)
	sendSuccess(c, StrStarted)
}

func apirefreshMusicInc(c *gin.Context) {
	var cfgp *config.MediaTypeConfig
	config.RangeSettingsMediaBreak(func(_ string, media *config.MediaTypeConfig) bool {
		if strings.HasPrefix(media.NamePrefix, "music") {
			cfgp = media
			return true
		}

		return false
	})
	worker.Dispatch(logger.StrRefreshMusicInc, func(key uint32, ctx context.Context) error {
		return utils.SingleJobs(ctx, "refreshinc", cfgp.NamePrefix, "", false, key)
	}, "Feeds")
	sendSuccess(c, StrStarted)
}

// @Summary      Import Music Chart for a Specific Date
// @Description  Runs the feeds import for a single music chart list using a date-specific URL
//
//	built from the list's chart_date_url_pattern and chart_date_format settings.
//
// @Tags         music
// @Param        name      path   string  true  "Media config name (e.g. mycharts)"
// @Param        listname  path   string  true  "List name (e.g. offiziellecharts-compilations)"
// @Param        date      query  string  true  "Date in YYYY-MM-DD format"
// @Param        apikey    query  string  true  "apikey"
// @Success      200  {object}  string  "returns started"
// @Failure      400  {object}  string  "error message"
// @Failure      401  {object}  Jsonerror
// @Router       /api/music/feeds/date/{name}/{listname} [get].
func apiMusicFeedsDate(c *gin.Context) {
	mediaName := c.Param(StrName)
	listName := c.Param("listname")
	dateStr := c.Query("date")

	if dateStr == "" {
		sendBadRequest(c, "date query parameter is required (YYYY-MM-DD)")
		return
	}

	cfgpstr := "music_" + mediaName

	var dispatched bool

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !strings.HasPrefix(media.NamePrefix, "music") {
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
				"feeds_date_"+cfgpstr+"_"+listName,
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
		sendBadRequest(c, "list "+listName+" not found in music config "+mediaName)
		return
	}

	sendSuccess(c, "Feeds date import started for "+listName+" ("+dateStr+")")
}

// @Summary      Add Music Artist and Import Albums
// @Description  Adds a MusicBrainz artist to dbartists and the given list (track_mode=albums), then imports all their albums in the background.
// @Tags         music
// @Param        name      path   string  true  "Media config name (without 'music_' prefix)"
// @Param        listname  path   string  true  "List name within the media config"
// @Param        artist_id query  string  true  "MusicBrainz artist ID"
// @Param        apikey    query  string  true  "apikey"
// @Success      200  {object}  string
// @Failure      400  {object}  string
// @Failure      401  {object}  Jsonerror
// @Router       /api/music/artist/add/{name}/{listname} [get].
func apiMusicAddArtist(c *gin.Context) {
	mediaName := c.Param(StrName)
	listName := c.Param("listname")
	artistID := c.Query("artist_id")

	if artistID == "" {
		sendBadRequest(c, "artist_id query parameter is required")
		return
	}

	cfgpstr := "music_" + mediaName

	cfgp := config.GetSettingsMedia(cfgpstr)
	if cfgp == nil {
		sendBadRequest(c, "music config not found: "+cfgpstr)
		return
	}

	listid, ok := cfgp.ListsMapIdx[listName]
	if !ok {
		sendBadRequest(c, "list "+listName+" not found in "+cfgpstr)
		return
	}

	artistIDCopy := artistID
	worker.Dispatch(
		"add_artist_"+artistID+"_"+cfgpstr,
		func(_ uint32, ctx context.Context) error {
			// Add artist to dbartists and artists tracking table with track_mode=albums.
			_, err := importfeed.JobImportArtist(ctx, &importfeed.ArtistConfig{
				MusicBrainzID: artistIDCopy,
				TrackMode:     "albums",
			}, cfgp, listid, true)
			if err != nil {
				logger.Logtype("error", 0).
					Str("artist_id", artistIDCopy).
					Err(err).
					Msg("apiMusicAddArtist: JobImportArtist failed")

				return err
			}

			// Import all albums by this artist into dbalbums and albums.
			return importfeed.JobImportDBAlbum(ctx, &config.ManualConfig{
				ArtistID: artistIDCopy,
			}, 0, cfgp, listid)
		},
		"Data",
	)

	sendSuccess(c, "Artist "+artistID+" queued for import into "+cfgpstr+"/"+listName)
}

// apiMusicDiscoverSeriesForArtist discovers and imports series albums for all albums of a tracked artist.
// It finds albums belonging to the artist that have a series_name set and queues DiscoverAndAddSeriesAlbums
// for each one in a background job.
func apiMusicDiscoverSeriesForArtist(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	// Resolve listname from artist row
	type artistRow struct {
		Listname string `db:"listname"`
	}

	ar := database.StructscanT[artistRow](
		false,
		1,
		"SELECT listname FROM artists WHERE id = ?",
		&id,
	)
	if len(ar) == 0 {
		sendBadRequest(c, "artist not found")
		return
	}

	cfgp, listid := findMusicCfgpAndListID(ar[0].Listname)
	if cfgp == nil {
		sendBadRequest(c, "music config not found for list "+ar[0].Listname)
		return
	}

	// Find albums for this artist that belong to a series (have series_name + release_group_id set)
	type seriesAlbumRow struct {
		ReleaseGroupID string `db:"musicbrainz_release_group_id"`
		Title          string `db:"title"`
	}

	albums := database.StructscanT[seriesAlbumRow](
		false,
		database.Getdatarow[uint](
			false,
			`SELECT count() FROM albums a INNER JOIN dbalbums da ON da.id = a.dbalbum_id WHERE a.artist_id = ? AND da.series_name != '' AND da.musicbrainz_release_group_id != ''`,
			&id,
		),
		`SELECT da.musicbrainz_release_group_id, da.title
		 FROM albums a
		 INNER JOIN dbalbums da ON da.id = a.dbalbum_id
		 WHERE a.artist_id = ?
		   AND da.series_name != ''
		   AND da.musicbrainz_release_group_id != ''`,
		&id,
	)

	if len(albums) == 0 {
		sendSuccess(c, "No series albums found for artist #"+id)
		return
	}

	cfgpCopy := cfgp
	listidCopy := listid

	worker.Dispatch(
		"discover_series_artist_"+id,
		func(_ uint32, ctx context.Context) error {
			for _, album := range albums {
				importfeed.DiscoverAndAddSeriesAlbums(
					ctx,
					album.ReleaseGroupID,
					album.Title,
					cfgpCopy,
					listidCopy,
					nil,
					nil,
				)
			}

			return nil
		},
		"Data",
	)

	sendSuccess(c, "Discover series albums queued for artist #"+id+
		" ("+strconv.Itoa(len(albums))+" series found)")
}

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
			routermusicsearch.GET("/missing/:name", apiMusicSearchMissing)
			routermusicsearch.GET("/history/clear/:name", apiMusicClearHistoryName)
			routermusicsearch.GET("/history/clearid/:id", apiMusicClearHistoryID)
			routermusicsearch.GET("/artists/missing/:name", apiMusicSearchArtistsMissing)
			routermusicsearch.GET("/artists/upgrade/:name", apiMusicSearchArtistsUpgrade)
		}
	}
}

// @Summary      List albums (metadata)
// @Description  Returns the albums of the dbalbums table (metadata independent of lists), paginated, with the total count.
// @Tags         music · data
// @Param        limit  query     int     false  "Rows per page (default 100, max 1000)"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Dbalbum}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/music [get].
func apiMusicDBList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	boundPagination(&params)

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

// @Summary      List the albums of a list
// @Description  Returns the album entries of one list (missing, quality profile, reached quality, root path, ...), paginated, with the total count.
// @Tags         music · data
// @Param        name   path      string  true   "Name of the list (case-insensitive), as configured in the media group"
// @Param        limit  query     int     false  "Rows per page; without it all rows are returned"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Album}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/music/list/{name} [get].
func apiMusicListGet(ctx *gin.Context) {
	name, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	params := parsePaginationParams(ctx)

	rows := database.Getdatarow[uint](false, "select count() from albums where listname = ? COLLATE NOCASE", &name)

	query := "select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbalbum_id,artist_id from albums where listname = ? COLLATE NOCASE"
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

// @Summary      Remove an album from a list
// @Description  Deletes one album list entry and its file rows. Files on disk are not touched.
// @Tags         music · data
// @Param        id     path      int     true   "albums (list entry) ID"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonsuccess  "Deleted"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      500  {object}  Jsonerror  "Deleting the files or the row failed"
// @Router       /api/music/{id} [delete].
func apiMusicDelete(ctx *gin.Context) {
	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Delete album files first
	if err := database.ExecNErr("DELETE FROM album_files WHERE album_id = ?", &id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete album files: " + err.Error()})
		return
	}
	// Delete the album
	if err := database.ExecNErr("DELETE FROM albums WHERE id = ?", &id); err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete album: " + err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{"success": true})
}

// @Summary      List artists
// @Description  Returns the artists of the dbartists table (names and MusicBrainz/Discogs/Spotify IDs), paginated, with the total count.
// @Tags         music · artists
// @Param        limit  query     int     false  "Rows per page (default 100, max 1000)"
// @Param        page   query     int     false  "Page number, starting at 1; only used together with limit"
// @Param        order  query     string  false  "Sort order: column names with optional asc/desc, e.g. 'id desc' or 'year desc, title'"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsondatarows{data=[]database.Dbartist}  "One page of rows and the total row count"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Router       /api/music/artists [get].
func apiArtistsList(ctx *gin.Context) {
	params := parsePaginationParams(ctx)
	boundPagination(&params)

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

// @Summary      Start a job for all music groups
// @Description  Queues the given job for every music media group (list jobs like feeds once per enabled list) and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         music · jobs
// @Param        job    path      string  true   "Job to start, one of: rss, rssartists, rssartistsupgrade, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/job/{job} [get].
func apiMusicAllJobs(c *gin.Context) {
	dispatchMediaAllJobs(c, mediaJobDispatchConfig{
		TypeName:    "music",
		NamePrefix:  "music",
		AllowedJobs: allowedjobsmusicstr,
	})
}

// @Summary      Start a job for one music group
// @Description  Queues the given job for one music media group and returns how many were started. Returns immediately; the work runs in the job queue.
// @Tags         music · jobs
// @Param        job    path      string  true   "Job to start, one of: rss, rssartists, rssartistsupgrade, data, datafull, checkmissing, checkmissingflag, checkreachedflag, structure, searchmissingfull, searchmissinginc, searchupgradefull, searchupgradeinc, searchmissingfulltitle, searchmissinginctitle, searchupgradefulltitle, searchupgradeinctitle, clearhistory, feeds, refresh, refreshinc"
// @Param        name   path      string  true   "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonjobstarted  "'Job <job> started', or 'Nothing Done'"
// @Failure      400  {object}  Jsonbadrequest  "Job is not allowed for this media type"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/job/{job}/{name} [get].
func apiMusicJobs(c *gin.Context) {
	dispatchMediaJob(c, mediaJobDispatchConfig{
		TypeName:    "music",
		NamePrefix:  "music",
		AllowedJobs: allowedjobsmusicstr,
	})
}

// @Summary      Preview the RSS search of a music group
// @Description  Reads the indexers' RSS feeds for one music group and returns the accepted and denied releases. Nothing is downloaded.
// @Tags         music · search
// @Param        group  path      string  true   "Media group name without type prefix, e.g. EN"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/music/rss/search/list/{group} [get].
func apiMusicRssSearchList(c *gin.Context) {
	rssPreviewGroup(c, "music")
}

// @Summary      Search an album and show the results
// @Description  Searches the indexers for one album list entry and waits for the result: returns the accepted and denied releases with the reasons. download=true also downloads the best accepted release.
// @Tags         music · search
// @Param        id             path      int     true   "albums (list entry) ID"
// @Param        searchByTitle  query     string  false  "true: also search by title"
// @Param        download       query     bool    false  "true: download the best accepted release"
// @Param        apikey         query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonresults  "Releases the indexers returned, split into accepted and denied with the reason"
// @Success      204  "Nothing Done - no media group or nothing to search (no body)"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  string  "failed with <search error> (plain JSON string)"
// @Router       /api/music/search/list/{id} [get].
func apiMusicSearchList(c *gin.Context) {
	searchListEntry(c, "music", "albums")
}

// @Summary      Search missing albums of a group
// @Description  Queues an incremental search for the missing albums of a music group; accepted releases are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         music · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonsearchstarted  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/search/missing/{name} [get].
func apiMusicSearchMissing(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, "music", "music", listName, logger.StrSearchMissingInc, logger.StrSearchMissingInc+"_", "Search", "Search started for music list "+listName)
}

// @Summary      Clear the download history of a group or list
// @Description  Queues deleting the album download history of a music media group (or of one of its lists). Returns immediately; the work runs in the job queue.
// @Tags         music · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/search/history/clear/{name} [get].
func apiMusicClearHistoryName(c *gin.Context) {
	dispatchClearHistory(c, "music", "music", c.Param(StrName))
}

// @Summary      Clear the download history of an album
// @Description  Deletes all download history rows of one album list entry, so its releases can be downloaded again. Returns the number of deleted rows.
// @Tags         music · search
// @Param        id     path      int     true   "albums (list entry) ID"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsoncount  "Number of deleted history rows"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      403  {object}  Jsonforbidden  "Database error"
// @Router       /api/music/search/history/clearid/{id} [get].
func apiMusicClearHistoryID(c *gin.Context) {
	id, ok := getParamID(c, StrID)
	if !ok {
		return
	}

	inres, inerr := database.DeleteRow("album_histories", "album_id = ?", id)
	handleDBInsertOrUpdate(c, inres, inerr, false)
}

// @Summary      Search missing albums by artist
// @Description  Queues indexer searches by artist name (up to 20 random artists with missing albums) for a music media group; one search can find several albums. Accepted releases are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         music · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonartistsearchmissing  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/search/artists/missing/{name} [get].
func apiMusicSearchArtistsMissing(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, "music", "music", listName, logger.StrRssArtists, logger.StrRssArtists+"_", "Search", "Artist search (missing) started for music list "+listName)
}

// @Summary      Search album upgrades by artist
// @Description  Queues indexer searches by artist name (up to 20 random artists with albums below their quality cutoff) for a music media group. Accepted upgrades are downloaded. Returns immediately; the work runs in the job queue.
// @Tags         music · search
// @Param        name   path      string  true   "Media group name, or the name of one of its lists (the whole group is processed)"
// @Param        apikey query     string  true   "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonartistsearchupgrade  "Search queued; the text names the list"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "Media group or list not found"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/search/artists/upgrade/{name} [get].
func apiMusicSearchArtistsUpgrade(c *gin.Context) {
	listName := c.Param(StrName)
	dispatchNamedConfigJob(c, "music", "music", listName, logger.StrRssArtistsUpgrade, logger.StrRssArtistsUpgrade+"_", "Search", "Artist search (upgrade) started for music list "+listName)
}

// @Summary      Refresh the metadata of all albums
// @Description  Queues a metadata refresh (MusicBrainz and the other music providers) of every album in the lists of all music media groups. Returns immediately; the work runs in the job queue.
// @Tags         music · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/all/refreshall [get].
func apirefreshMusic(c *gin.Context) {
	dispatchJobAllConfigs(c, "music", "music", "refresh", StrStarted)
}

// @Summary      Refresh the metadata of one album
// @Description  Queues a metadata refresh of one album. Returns immediately; the work runs in the job queue.
// @Tags         music · jobs
// @Param        id   path      int  true  "ID of the album in the dbalbums table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/refresh/{id} [get].
func apirefreshAlbum(c *gin.Context) {
	id := c.Param(logger.StrID)
	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM albums WHERE dbalbum_id = ? LIMIT 1",
		&id,
	)
	dispatchConfigFunc(
		c,
		mediaConfigForList("music", listname),
		"music",
		"Refresh Single Album_"+id,
		func(_ context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RefreshAlbum(cfgp, &id)
		},
	)
}

// @Summary      Rewrite the audio tags of one album
// @Description  Queues rewriting the audio tags of the album's files from the stored metadata (no new matching). Returns immediately; the work runs in the job queue.
// @Tags         music · tags
// @Param        id     path      int     true  "ID of the album in the dbalbums table"
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      400  {object}  Jsonbadrequest  "Invalid id"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/tag/{id} [get].
func apiRetagAlbum(c *gin.Context) {
	dbID, ok := parseUintParam(c, logger.StrID)
	if !ok {
		return
	}

	listname := database.Getdatarow[string](
		false,
		"SELECT listname FROM albums WHERE dbalbum_id = ? LIMIT 1",
		&dbID,
	)
	dispatchConfigFunc(
		c,
		mediaConfigForList("music", listname),
		"music",
		"Retag Album_"+c.Param(logger.StrID),
		func(ctx context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RetagAlbum(ctx, cfgp, dbID)
		},
	)
}

// @Summary      Rewrite the audio tags of an artist's albums
// @Description  Queues rewriting the audio tags of all albums of one artist from the stored metadata. Returns immediately; the work runs in the job queue.
// @Tags         music · tags
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Param        id     path      int     true   "ID of the artist in the dbartists table"
// @Success      200  {object}  Jsonstarted  "Queued as a background job"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No media group has this entry"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/tag/artist/{id} [get].
func apiRetagArtistAlbums(c *gin.Context) {
	artistID, ok := parseUintParam(c, logger.StrID)
	if !ok {
		return
	}

	dispatchConfigFunc(
		c,
		mediaConfigForList("music", ""),
		"music",
		"Retag Artist Albums_"+c.Param(logger.StrID),
		func(ctx context.Context, cfgp *config.MediaTypeConfig) error {
			return utils.RetagArtistAlbums(ctx, cfgp, artistID)
		},
	)
}

// @Summary      Rewrite the audio tags of all albums
// @Description  Queues rewriting the audio tags of every album of every music media group from the stored metadata. Returns immediately; the work runs in the job queue.
// @Tags         music · tags
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/tag/all [get].
func apiRetagAllAlbums(c *gin.Context) {
	dispatchConfigFuncAll(c, "music", "music", "Retag All Albums", utils.RetagAllAlbums)
}

// @Summary      Refresh album metadata (incremental)
// @Description  Queues a metadata refresh of the 100 least recently updated albums of each music media group. Returns immediately; the work runs in the job queue.
// @Tags         music · jobs
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonstarted  "Queued as a background job; 'Nothing Done' when no configuration started it"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      404  {object}  Jsonnotfound  "No configuration of this media type"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/all/refresh [get].
func apirefreshMusicInc(c *gin.Context) {
	dispatchJobAllConfigs(c, "music", "music", "refreshinc", StrStarted)
}

// @Summary      Import a music chart of a given date
// @Description  Queues the feeds import of one music chart list for the given date: the URL is built from the list's chart_date_url_pattern and chart_date_format settings. Returns immediately; the import runs in the job queue.
// @Tags         music · jobs
// @Param        name      path   string  true  "Media config name (e.g. mycharts)"
// @Param        listname  path   string  true  "List name (e.g. offiziellecharts-compilations)"
// @Param        date      query  string  true  "Date in YYYY-MM-DD format"
// @Param        apikey    query  string  true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonfeedsdate  "'Feeds date import started for <list> (<date>)'"
// @Failure      400  {object}  Jsonbadrequest  "date missing or invalid, or list not found/disabled"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
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

	// responded is set by every path inside the closure that already wrote
	// a response, so the trailing writes below never double-write.
	var found, responded bool

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if found || !strings.HasPrefix(media.NamePrefix, "music_") {
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
		sendBadRequest(c, "list "+listName+" not found in music config "+mediaName)
		return
	}

	sendSuccess(c, "Feeds date import started for "+listName+" ("+dateStr+")")
}

// @Summary      Add an artist and import their albums
// @Description  Adds a MusicBrainz artist to dbartists and the given list (track_mode=albums), then imports all their albums in the background.
// @Tags         music · artists
// @Param        name      path   string  true  "Media config name (without 'music_' prefix)"
// @Param        listname  path   string  true  "List name within the media config"
// @Param        artist_id query  string  true  "MusicBrainz artist ID"
// @Param        apikey    query  string  true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  Jsonartistqueued  "'Artist <id> queued for import into <group>/<list>'"
// @Failure      400  {object}  Jsonbadrequest  "artist_id missing, or media group/list not found"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
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
	if err := worker.Dispatch(
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
	); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, "Artist "+artistID+" queued for import into "+cfgpstr+"/"+listName)
}

// apiMusicDiscoverSeriesForArtist discovers and imports series albums for all albums of a tracked artist.
// It finds albums belonging to the artist that have a series_name set and queues DiscoverAndAddSeriesAlbums
// for each one in a background job.
// @Summary      Import the series albums of an artist
// @Description  For every album of the tracked artist that belongs to a series (e.g. a compilation series), queues finding and adding the other albums of that series. Returns immediately; the work runs in the job queue.
// @Tags         music · artists
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Param        id     path      int     true   "artists (list entry) ID"
// @Success      200  {object}  Jsondiscoverseries  "'Discover series albums queued for artist #<id> (<n> series found)', or 'No series albums found for artist #<id>'"
// @Failure      400  {object}  Jsonbadrequest  "Artist or media group not found"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      409  {object}  Jsonconflict  "The job is already queued"
// @Failure      500  {object}  Jsonservererror  "The job could not be queued"
// @Router       /api/music/discover/series/artist/{id} [get].
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

	if err := worker.Dispatch(
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
	); err != nil {
		sendDispatchError(c, err)
		return
	}

	sendSuccess(c, "Discover series albums queued for artist #"+id+
		" ("+strconv.Itoa(len(albums))+" series found)")
}

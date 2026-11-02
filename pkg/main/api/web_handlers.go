package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/gin-gonic/gin"
)

// dropdownListnameColumns maps a per-config item dropdown table to the listname
// column used to scope its options to a selected media configuration's lists.
var dropdownListnameColumns = map[string]string{
	"movies":     "movies.listname",
	"series":     "series.listname",
	"albums":     "albums.listname",
	"books":      "books.listname",
	"audiobooks": "audiobooks.listname",
	"artists":    "artists.listname",
	"authors":    "authors.listname",
}

// apiAdminDropdownData provides AJAX endpoint for dynamic dropdown data loading.
func apiAdminDropdownData(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, "table")
	if !ok {
		return
	}

	fieldName, ok := getParamID(ctx, "field")
	if !ok {
		return
	}

	// Handle both GET and POST parameters
	var search, page, idLookup, mediaConfig string
	if ctx.Request.Method == "POST" {
		search = ctx.PostForm("search")
		page = ctx.DefaultPostForm("page", "1")
		idLookup = ctx.PostForm("id")
		mediaConfig = ctx.PostForm("media_config")
	} else {
		search = ctx.Query("search")
		page = ctx.DefaultQuery("page", "1")
		idLookup = ctx.Query("id")
		mediaConfig = ctx.Query("media_config")
	}

	// Limit search length to prevent header size issues
	if len(search) > 100 {
		search = search[:100]
	}

	// Convert page to int
	pageNum := 1
	if p, err := strconv.Atoi(page); err == nil && p > 0 {
		pageNum = p
	}

	// Set page size
	pageSize := 25
	offset := (pageNum - 1) * pageSize

	// If this is an ID lookup request, handle it separately
	if idLookup != "" {
		if idVal, err := strconv.Atoi(idLookup); err == nil {
			option := getDropdownOptionByID(tableName, fieldName, idVal)
			if option != nil {
				sendSelect2Response(ctx, []map[string]any{*option}, false)
				return
			}
		}

		// If ID lookup fails, return empty result
		sendSelect2Response(ctx, []map[string]any{}, false)

		return
	}

	var (
		options []map[string]any
		hasMore bool
	)

	// Build search filter
	searchFilter := ""

	searchArgs := []any{}
	if search != "" {
		switch tableName {
		case "dbmovies":
			searchFilter = " WHERE title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbseries":
			searchFilter = " WHERE seriename LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbserie_episodes":
			searchFilter = " WHERE (identifier LIKE ? OR title LIKE ?)"

			searchArgs = append(searchArgs, "%"+search+"%", "%"+search+"%")

		case "movies":
			searchFilter = " WHERE dbmovies.title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "series":
			searchFilter = " WHERE dbseries.seriename LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "serie_episodes":
			// The search page's episode picker (field serie_id) sends the
			// selected series id as the search term; every other caller sends
			// free text, where a number is an episode title/identifier term.
			if seriesID, err := strconv.Atoi(search); err == nil && seriesID > 0 && fieldName == "serie_id" {
				searchFilter = " WHERE series.id = ?"

				searchArgs = append(searchArgs, seriesID)
			} else {
				searchFilter = " WHERE (dbserie_episodes.title LIKE ? OR dbserie_episodes.identifier LIKE ? OR dbseries.seriename LIKE ?)"

				searchArgs = append(searchArgs, "%"+search+"%", "%"+search+"%", "%"+search+"%")
			}

		case "qualities":
			searchFilter = " WHERE name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		// Book tables
		case "dbbooks":
			searchFilter = " WHERE title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbauthors":
			searchFilter = " WHERE name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbbook_series":
			searchFilter = " WHERE name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "books":
			searchFilter = " WHERE dbbooks.title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "authors":
			searchFilter = " WHERE dbauthors.name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "book_series":
			searchFilter = " WHERE dbbook_series.name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		// Audiobook tables
		case "dbaudiobooks":
			searchFilter = " WHERE title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbnarrators":
			searchFilter = " WHERE name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "audiobooks":
			searchFilter = " WHERE dbaudiobooks.title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		// Music tables
		case "dbalbums":
			searchFilter = " WHERE title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbartists":
			searchFilter = " WHERE name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "dbtracks":
			searchFilter = " WHERE title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "artists":
			searchFilter = " WHERE dbartists.name LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")

		case "albums":
			searchFilter = " WHERE dbalbums.title LIKE ?"

			searchArgs = append(searchArgs, "%"+search+"%")
		}
	}

	// When a media configuration is supplied, restrict per-config item lookups to
	// rows whose listname belongs to that config's lists. This keeps the content
	// dropdown (movie/series/album/...) scoped to the selected configuration.
	if mediaConfig != "" {
		if col, ok := dropdownListnameColumns[tableName]; ok {
			if cfgp := config.GetSettingsMedia(mediaConfig); cfgp != nil && cfgp.ListsLen > 0 {
				if searchFilter == "" {
					searchFilter = " WHERE " + col + " IN (?" + cfgp.ListsQu + ")"
				} else {
					searchFilter += " AND " + col + " IN (?" + cfgp.ListsQu + ")"
				}

				for i := range cfgp.ListsNames {
					searchArgs = append(searchArgs, cfgp.ListsNames[i])
				}
			}
		}
	}

	// Add pagination args
	searchArgs = append(searchArgs, pageSize+1, offset)

	switch tableName {
	case "dbmovies":
		query := fmt.Sprintf(
			"SELECT title, id FROM dbmovies%s ORDER BY title LIMIT ? OFFSET ?",
			searchFilter,
		)
		movies := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(movies) > pageSize
		if hasMore {
			movies = movies[:pageSize]
		}

		for _, movie := range movies {
			options = append(options, createSelect2Option(movie.Num, movie.Str))
		}

	case "dbseries":
		query := fmt.Sprintf(
			"SELECT seriename, id FROM dbseries%s ORDER BY seriename LIMIT ? OFFSET ?",
			searchFilter,
		)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}

		for _, serie := range series {
			options = append(options, createSelect2Option(serie.Num, serie.Str))
		}

	case "dbserie_episodes":
		query := fmt.Sprintf(
			"SELECT identifier, title, id FROM dbserie_episodes%s ORDER BY identifier LIMIT ? OFFSET ?",
			searchFilter,
		)
		episodes := database.GetrowsN[syncops.DbstaticTwoStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(episodes) > pageSize
		if hasMore {
			episodes = episodes[:pageSize]
		}

		for _, episode := range episodes {
			label := fmt.Sprintf("%s - %s", episode.Str1, episode.Str2)

			options = append(options, createSelect2Option(episode.Num, label))
		}

	case "movies":
		query := fmt.Sprintf(
			"SELECT COALESCE(dbmovies.title, '(missing #' || movies.dbmovie_id || ')') || ' - ' || movies.listname, movies.id FROM movies LEFT JOIN dbmovies ON movies.dbmovie_id = dbmovies.id%s ORDER BY dbmovies.title LIMIT ? OFFSET ?",
			searchFilter,
		)
		movies := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(movies) > pageSize
		if hasMore {
			movies = movies[:pageSize]
		}

		for _, movie := range movies {
			options = append(options, createSelect2Option(movie.Num, movie.Str))
		}

	case "series":
		query := fmt.Sprintf(
			"SELECT COALESCE(dbseries.seriename, '(missing #' || series.dbserie_id || ')') || ' - ' || series.listname, series.id FROM series LEFT JOIN dbseries ON series.dbserie_id = dbseries.id%s ORDER BY dbseries.seriename LIMIT ? OFFSET ?",
			searchFilter,
		)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}

		for _, serie := range series {
			options = append(options, createSelect2Option(serie.Num, serie.Str))
		}

	case "serie_episodes":
		query := fmt.Sprintf(
			"SELECT COALESCE(dbseries.seriename, 'Unknown Series') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.identifier IS NOT NULL AND dbserie_episodes.identifier != 'S00E00' THEN dbserie_episodes.identifier ELSE 'ID:' || serie_episodes.id END, 'Unknown') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.title IS NOT NULL AND TRIM(dbserie_episodes.title) != '' THEN dbserie_episodes.title ELSE 'Episode ' || COALESCE(dbserie_episodes.episode, 'Unknown') END, 'Unknown Episode') || ' (' || COALESCE(series.listname, '?') || ')', serie_episodes.id FROM serie_episodes LEFT JOIN dbserie_episodes ON serie_episodes.dbserie_episode_id = dbserie_episodes.id LEFT JOIN series ON serie_episodes.serie_id = series.id LEFT JOIN dbseries ON series.dbserie_id = dbseries.id%s ORDER BY dbseries.seriename, series.listname, dbserie_episodes.season, dbserie_episodes.episode LIMIT ? OFFSET ?",
			searchFilter,
		)
		episodes := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(episodes) > pageSize
		if hasMore {
			episodes = episodes[:pageSize]
		}

		for _, episode := range episodes {
			options = append(options, createSelect2Option(episode.Num, episode.Str))
		}

	case "qualities":
		// Determine quality type based on field name
		var typeFilter string
		switch fieldName {
		case "resolution_id":
			typeFilter = " AND type = 1"
		case "quality_id":
			typeFilter = " AND type = 2"
		case "codec_id":
			typeFilter = " AND type = 3"
		case "audio_id":
			typeFilter = " AND type = 4"
		default:
			typeFilter = "" // Show all if field name doesn't match known types
		}

		// Update search filter to include type filter
		if searchFilter == "" {
			searchFilter = " WHERE 1=1" + typeFilter
		} else {
			searchFilter += typeFilter
		}

		query := fmt.Sprintf(
			"SELECT name, id FROM qualities%s ORDER BY name LIMIT ? OFFSET ?",
			searchFilter,
		)
		qualities := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(qualities) > pageSize
		if hasMore {
			qualities = qualities[:pageSize]
		}

		for _, quality := range qualities {
			options = append(options, createSelect2Option(quality.Num, quality.Str))
		}

	// Book tables
	case "dbbooks":
		query := fmt.Sprintf(
			"SELECT title, id FROM dbbooks%s ORDER BY title LIMIT ? OFFSET ?",
			searchFilter,
		)
		books := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(books) > pageSize
		if hasMore {
			books = books[:pageSize]
		}

		for _, book := range books {
			options = append(options, createSelect2Option(book.Num, book.Str))
		}

	case "dbauthors":
		query := fmt.Sprintf(
			"SELECT name, id FROM dbauthors%s ORDER BY name LIMIT ? OFFSET ?",
			searchFilter,
		)
		authors := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(authors) > pageSize
		if hasMore {
			authors = authors[:pageSize]
		}

		for _, author := range authors {
			options = append(options, createSelect2Option(author.Num, author.Str))
		}

	case "books":
		query := fmt.Sprintf(
			"SELECT COALESCE((SELECT au.name FROM dbauthors au WHERE au.id = dbbooks.dbauthor_id) || ' - ', '') || COALESCE(dbbooks.title, '(missing #' || books.dbbook_id || ')') || ' - ' || books.listname, books.id FROM books LEFT JOIN dbbooks ON books.dbbook_id = dbbooks.id%s ORDER BY CASE WHEN dbbooks.title IS NULL OR dbbooks.title = '' THEN 1 ELSE 0 END, dbbooks.title LIMIT ? OFFSET ?",
			searchFilter,
		)
		books := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(books) > pageSize
		if hasMore {
			books = books[:pageSize]
		}

		for _, book := range books {
			options = append(options, createSelect2Option(book.Num, book.Str))
		}

	// Audiobook tables
	case "dbaudiobooks":
		query := fmt.Sprintf(
			"SELECT title, id FROM dbaudiobooks%s ORDER BY title LIMIT ? OFFSET ?",
			searchFilter,
		)
		audiobooks := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(audiobooks) > pageSize
		if hasMore {
			audiobooks = audiobooks[:pageSize]
		}

		for _, audiobook := range audiobooks {
			options = append(options, createSelect2Option(audiobook.Num, audiobook.Str))
		}

	case "dbnarrators":
		query := fmt.Sprintf(
			"SELECT name, id FROM dbnarrators%s ORDER BY name LIMIT ? OFFSET ?",
			searchFilter,
		)
		narrators := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(narrators) > pageSize
		if hasMore {
			narrators = narrators[:pageSize]
		}

		for _, narrator := range narrators {
			options = append(options, createSelect2Option(narrator.Num, narrator.Str))
		}

	case "audiobooks":
		query := fmt.Sprintf(
			"SELECT COALESCE((SELECT au.name FROM dbaudiobook_authors aba JOIN dbauthors au ON au.id = aba.dbauthor_id WHERE aba.dbaudiobook_id = dbaudiobooks.id LIMIT 1) || ' - ', '') || COALESCE(dbaudiobooks.title, '(missing #' || audiobooks.dbaudiobook_id || ')') || ' - ' || audiobooks.listname, audiobooks.id FROM audiobooks LEFT JOIN dbaudiobooks ON audiobooks.dbaudiobook_id = dbaudiobooks.id%s ORDER BY CASE WHEN dbaudiobooks.title IS NULL OR dbaudiobooks.title = '' THEN 1 ELSE 0 END, dbaudiobooks.title LIMIT ? OFFSET ?",
			searchFilter,
		)
		audiobooks := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(audiobooks) > pageSize
		if hasMore {
			audiobooks = audiobooks[:pageSize]
		}

		for _, audiobook := range audiobooks {
			options = append(options, createSelect2Option(audiobook.Num, audiobook.Str))
		}

	// Music tables
	case "dbalbums":
		query := fmt.Sprintf(
			"SELECT title, id FROM dbalbums%s ORDER BY title LIMIT ? OFFSET ?",
			searchFilter,
		)
		albums := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(albums) > pageSize
		if hasMore {
			albums = albums[:pageSize]
		}

		for _, album := range albums {
			options = append(options, createSelect2Option(album.Num, album.Str))
		}

	case "dbartists":
		query := fmt.Sprintf(
			"SELECT name, id FROM dbartists%s ORDER BY name LIMIT ? OFFSET ?",
			searchFilter,
		)
		artists := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(artists) > pageSize
		if hasMore {
			artists = artists[:pageSize]
		}

		for _, artist := range artists {
			options = append(options, createSelect2Option(artist.Num, artist.Str))
		}

	case "albums":
		query := fmt.Sprintf(
			"SELECT COALESCE((SELECT ar.name FROM dbalbum_artists aa JOIN dbartists ar ON ar.id = aa.dbartist_id WHERE aa.dbalbum_id = dbalbums.id LIMIT 1) || ' - ', '') || COALESCE(dbalbums.title, '(missing #' || albums.dbalbum_id || ')') || ' - ' || albums.listname, albums.id FROM albums LEFT JOIN dbalbums ON albums.dbalbum_id = dbalbums.id%s ORDER BY CASE WHEN dbalbums.title IS NULL OR dbalbums.title = '' THEN 1 ELSE 0 END, dbalbums.title LIMIT ? OFFSET ?",
			searchFilter,
		)
		albums := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(albums) > pageSize
		if hasMore {
			albums = albums[:pageSize]
		}

		for _, album := range albums {
			options = append(options, createSelect2Option(album.Num, album.Str))
		}

	// Additional book tables
	case "dbbook_series":
		query := fmt.Sprintf(
			"SELECT name, id FROM dbbook_series%s ORDER BY name LIMIT ? OFFSET ?",
			searchFilter,
		)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}

		for _, s := range series {
			options = append(options, createSelect2Option(s.Num, s.Str))
		}

	case "authors":
		query := fmt.Sprintf(
			"SELECT COALESCE(dbauthors.name, '(missing #' || authors.dbauthor_id || ')') || ' - ' || authors.listname, authors.id FROM authors LEFT JOIN dbauthors ON authors.dbauthor_id = dbauthors.id%s ORDER BY dbauthors.name LIMIT ? OFFSET ?",
			searchFilter,
		)
		authors := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(authors) > pageSize
		if hasMore {
			authors = authors[:pageSize]
		}

		for _, author := range authors {
			options = append(options, createSelect2Option(author.Num, author.Str))
		}

	case "book_series":
		query := fmt.Sprintf(
			"SELECT COALESCE(dbbook_series.name, '(missing #' || book_series.dbbook_series_id || ')') || ' - ' || book_series.listname, book_series.id FROM book_series LEFT JOIN dbbook_series ON book_series.dbbook_series_id = dbbook_series.id%s ORDER BY dbbook_series.name LIMIT ? OFFSET ?",
			searchFilter,
		)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}

		for _, s := range series {
			options = append(options, createSelect2Option(s.Num, s.Str))
		}

	// Additional music tables
	case "dbtracks":
		// Unfiltered, sort by the primary key: sorting ~1M rows by track_number
		// took seconds per open and listed every album's track 1 first.
		order := " ORDER BY id DESC"
		if searchFilter != "" {
			order = " ORDER BY title"
		}

		query := fmt.Sprintf(
			"SELECT title, id FROM dbtracks%s%s LIMIT ? OFFSET ?",
			searchFilter, order,
		)
		tracks := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(tracks) > pageSize
		if hasMore {
			tracks = tracks[:pageSize]
		}

		for _, track := range tracks {
			options = append(options, createSelect2Option(track.Num, track.Str))
		}

	case "artists":
		query := fmt.Sprintf(
			"SELECT COALESCE(dbartists.name, '(missing #' || artists.dbartist_id || ')') || ' - ' || artists.listname, artists.id FROM artists LEFT JOIN dbartists ON artists.dbartist_id = dbartists.id%s ORDER BY dbartists.name LIMIT ? OFFSET ?",
			searchFilter,
		)
		artists := database.GetrowsN[database.DbstaticOneStringOneInt](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(artists) > pageSize
		if hasMore {
			artists = artists[:pageSize]
		}

		for _, artist := range artists {
			options = append(options, createSelect2Option(artist.Num, artist.Str))
		}
	}

	// Return Select2 compatible JSON response
	sendSelect2Response(ctx, options, hasMore)
}

// @Summary      Insert Table Record
// @Description  Inserts a new record into the specified table
// @Tags         admin
// @Param        name   path     string                 true "Table name"
// @Param        data   body     map[string]any true "Record data"
// @Param        apikey query    string                 true "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Router       /api/admin/table/{name}/insert [post].
func apiAdminTableInsert(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	var data map[string]any

	// Handle both JSON and form data
	contentType := ctx.GetHeader("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := ctx.BindJSON(&data); err != nil {
			sendErrorResponse(ctx, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		// Handle form data
		if err := ctx.Request.ParseForm(); err != nil {
			sendErrorResponse(ctx, http.StatusBadRequest, err.Error())
			return
		}

		data = make(map[string]any)
		for key, values := range ctx.Request.PostForm {
			if !strings.HasPrefix(key, "field-") {
				continue
			}

			key = strings.TrimPrefix(key, "field-")
			if len(values) > 0 {
				data[key] = values[0] // Take first value if multiple
			}
		}
	}

	err := insertAdminRecord(tableName, data)
	sendOperationResult(ctx, err)
}

// @Summary      Update Table Record
// @Description  Updates a record in the specified table
// @Tags         admin
// @Param        name   path     string                 true "Table name"
// @Param        index  path     int                    true "Record index"
// @Param        data   body     map[string]any true "Record data"
// @Param        apikey query    string                 true "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Router       /api/admin/table/{name}/update/{index} [post].
func apiAdminTableUpdate(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	indexStr, ok := getParamID(ctx, "index")
	if !ok {
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		sendErrorResponse(ctx, http.StatusBadRequest, "invalid index")
		return
	}

	var data map[string]any

	// Handle both JSON and form data
	contentType := ctx.GetHeader("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := ctx.BindJSON(&data); err != nil {
			sendErrorResponse(ctx, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		// Handle form data
		if err := ctx.Request.ParseForm(); err != nil {
			sendErrorResponse(ctx, http.StatusBadRequest, err.Error())
			return
		}

		data = make(map[string]any)
		for key, values := range ctx.Request.PostForm {
			if !strings.HasPrefix(key, "field-") {
				continue
			}

			key = strings.TrimPrefix(key, "field-")
			if len(values) > 0 {
				data[key] = values[0] // Take first value if multiple
			}
		}

		// Unchecked checkboxes are not posted at all; without this an edit
		// form could never switch a flag column back off.
		if database.GetTableDefaults(tableName).Table == "" {
			sendBadRequest(ctx, "unknown table: "+tableName)
			return
		}

		for col := range getColumnTypes(tableName) {
			if _, posted := data[col]; !posted && isCheckboxFieldRefactored(col) {
				data[col] = "0"
			}
		}
	}

	err = updateAdminRecord(tableName, index, data)
	sendOperationResult(ctx, err)
}

// @Summary      Delete Table Record
// @Description  Deletes a record from the specified table
// @Tags         admin
// @Param        name   path     string true "Table name"
// @Param        index  path     int    true "Record index"
// @Param        apikey query    string true "apikey"
// @Success      200    {object} gin.H
// @Failure      400    {object} Jsonerror
// @Failure      401    {object} Jsonerror
// @Router       /api/admin/table/{name}/delete/{index} [post].
func apiAdminTableDelete(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	indexStr, ok := getParamID(ctx, "index")
	if !ok {
		return
	}

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		sendErrorResponse(ctx, http.StatusBadRequest, "invalid index")
		return
	}

	err = deleteAdminRecord(tableName, index)
	sendOperationResult(ctx, err)
}

// @Summary      Admin Web Interface
// @Description  Serves the admin web interface
// @Tags         admin
// @Param        apikey query     string    true  "apikey"
// @Success      200    {string}  string  "HTML content"
// @Failure      401    {object}  Jsonerror
// @Router       /api/admin [get].
func apiAdminInterface(ctx *gin.Context) {
	if sessVal, exists := ctx.Get("session"); exists {
		if sess, ok := sessVal.(*Session); ok && !sess.WizardDismissed && wizardConfigLooksDefault() {
			ctx.Redirect(http.StatusFound, "/api/admin/wizard")
			return
		}
	}

	// Generate HTML using gomponents
	csrfToken := getCSRFToken(ctx)
	pageContent := adminPage()

	ctx.Header("Content-Type", "text/html; charset=utf-8")
	// Add CSRF token as a JavaScript variable in the head
	pageWithCSRF := strings.Replace(pageContent, "</head>",
		fmt.Sprintf("<script>window.csrfToken = '%s';</script></head>", csrfToken), 1)
	ctx.String(http.StatusOK, pageWithCSRF)
}

func apiAdminTableDataJson(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, "table")
	if !ok {
		return
	}

	// Table and column names below are interpolated into SQL, so only tables
	// known to GetTableDefaults are accepted. Every Table there starts with
	// tableName itself, which is therefore also safe to count from.
	tabledefault := database.GetTableDefaults(tableName)
	if tabledefault.Table == "" {
		sendBadRequest(ctx, "unknown table")
		return
	}

	size, start := tableJSONPaging(getParam(ctx, "iDisplayLength", "10"), getParam(ctx, "iDisplayStart", "0"))
	columns := strings.Split(tabledefault.DefaultColumns, ",")
	orderby := tableJSONOrderBy(
		columns,
		getParam(ctx, "iSortCol_0", ""),
		getParam(ctx, "sSortDir_0", "asc"),
		tabledefault.DefaultOrderBy,
	)
	searchValue := getParamValue(ctx, "sSearch")

	customFilters, customArgs := buildCustomFilters(tableName, ctx)

	var final, total int

	database.Scanrowsdyn(false, "select Count(*) as frequency FROM "+tableName, &total)

	var queryArgs []any

	whereClause := ""

	if searchValue != "" || customFilters != "" {
		var conditions []string

		if searchValue != "" && tabledefault.DefaultQuery != "" {
			queryCondition := strings.TrimSpace(tabledefault.DefaultQuery)
			queryCondition = strings.TrimPrefix(queryCondition, "WHERE ")
			queryCondition = strings.TrimPrefix(queryCondition, "where ")
			conditions = append(conditions, queryCondition)

			for range tabledefault.DefaultQueryParamCount {
				queryArgs = append(queryArgs, "%"+searchValue+"%")
			}
		}

		if customFilters != "" {
			conditions = append(conditions, customFilters)
			queryArgs = append(queryArgs, customArgs...)
		}

		whereClause = tableJSONWhere(conditions)
	}

	data := database.GetrowsType(
		tabledefault.Object,
		false,
		uint(size),
		"select "+tabledefault.DefaultColumns+" from "+tabledefault.Table+" "+whereClause+" "+orderby+" LIMIT ?, ?",
		append(queryArgs, start, size)...)
	retdata := make([][]string, 0, len(data))

	for _, loop := range data {
		row := make([]string, 0, len(columns))
		for _, v := range columns {
			v = strings.TrimSpace(v)
			if logger.ContainsI(v, " as ") {
				v = strings.Split(v, " as ")[1]
			}

			row = append(row, fmt.Sprint(loop[v]))
		}

		retdata = append(retdata, row)
	}

	if whereClause == "" {
		final = total
	} else {
		database.Scanrowsdyn(
			false,
			"select Count(*) as frequency FROM "+tabledefault.Table+" "+whereClause,
			&final,
			queryArgs...)
	}

	sendDataTablesResponse(ctx, total, final, retdata)
}

// tableJSONMaxPageSize caps DataTables page requests; "-1" (show all) would
// otherwise pull an entire 1M-row table into memory.
const tableJSONMaxPageSize = 1000

func tableJSONPaging(lengthParam, startParam string) (size, start int) {
	size, _ = strconv.Atoi(lengthParam)
	if size <= 0 || size > tableJSONMaxPageSize {
		size = tableJSONMaxPageSize
	}

	start, _ = strconv.Atoi(startParam)
	if start < 0 {
		start = 0
	}

	return size, start
}

// tableJSONOrderBy builds the ORDER BY clause. The column comes from the
// table's own column list by index and the direction is restricted to
// asc/desc, so no client string reaches the SQL. Without a sort column the
// table's DefaultOrderBy applies.
func tableJSONOrderBy(columns []string, sortColParam, dirParam, defaultOrderBy string) string {
	if sortColParam == "" && strings.TrimSpace(defaultOrderBy) != "" {
		return strings.TrimSpace(defaultOrderBy)
	}

	order := "id"
	if len(columns) > 0 {
		orderidx := logger.StringToInt(sortColParam)
		if orderidx < 0 || orderidx >= len(columns) {
			orderidx = 0
		}

		order = strings.TrimSpace(columns[orderidx])
		if logger.ContainsI(order, " as ") {
			order = strings.TrimSpace(strings.Split(order, " as ")[1])
		}
	}

	direction := "asc"
	if strings.EqualFold(strings.TrimSpace(dirParam), "desc") {
		direction = "desc"
	}

	return "order by " + order + " " + direction
}

// tableJSONWhere parenthesises each condition so an OR inside the search
// clause can't swallow the filters ANDed after it.
func tableJSONWhere(conditions []string) string {
	if len(conditions) == 0 {
		return ""
	}

	return "WHERE (" + strings.Join(conditions, ") AND (") + ")"
}

func apiAdminTableDataEditForm(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, StrName)
	if !ok {
		return
	}

	id, ok := getParamID(ctx, StrID)
	if !ok {
		return
	}

	// Validate the table against the known admin tables; this also guards the
	// table name interpolated into the query below.
	if database.GetTableDefaults(tableName).Table == "" {
		sendBadRequest(ctx, "unknown table: "+tableName)
		return
	}

	// Load the row generically with every real column so the edit form always
	// exposes the same fields as the add form (which uses all columns). MapScan
	// tolerates legacy non-numeric values in numeric columns, so the per-table
	// CAST/COALESCE scans are no longer needed.
	rows := database.GetrowsType(nil, false, 1, "SELECT * FROM "+tableName+" WHERE id = ?", id)
	if len(rows) == 0 {
		sendBadRequest(ctx, "record not found")
		return
	}

	rowMap := rows[0]

	var buf strings.Builder
	renderTableEditForm(tableName, rowMap, id, getCSRFToken(ctx)).Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

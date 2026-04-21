package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	gin "github.com/gin-gonic/gin"
)

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
	var search, page, idLookup string
	if ctx.Request.Method == "POST" {
		search = ctx.PostForm("search")
		page = ctx.DefaultPostForm("page", "1")
		idLookup = ctx.PostForm("id")
	} else {
		search = ctx.Query("search")
		page = ctx.DefaultQuery("page", "1")
		idLookup = ctx.Query("id")
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
			// Check if search is a series ID (numeric) or a text search
			if seriesID, err := strconv.Atoi(search); err == nil && seriesID > 0 {
				// Search is a series ID - filter episodes by this series
				searchFilter = " WHERE series.id = ?"

				searchArgs = append(searchArgs, seriesID)
			} else if search != "" {
				// Search is text - filter by episode title
				searchFilter = " WHERE dbserie_episodes.title LIKE ?"

				searchArgs = append(searchArgs, "%"+search+"%")
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

		case "listnames":
			if tableName == "movies" {
				searchFilter = " WHERE listname LIKE ?"
			} else {
				searchFilter = " WHERE listname LIKE ?"
			}

			searchArgs = append(searchArgs, "%"+search+"%")

		case "quality_profiles":
			if tableName == "movies" {
				searchFilter = " WHERE quality_profile LIKE ?"
			} else {
				searchFilter = " WHERE quality_profile LIKE ?"
			}

			searchArgs = append(searchArgs, "%"+search+"%")
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
			"SELECT dbmovies.title || ' - ' || movies.listname, movies.id FROM movies LEFT JOIN dbmovies ON movies.dbmovie_id = dbmovies.id%s ORDER BY dbmovies.title LIMIT ? OFFSET ?",
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
			"SELECT dbseries.seriename || ' - ' || series.listname, series.id FROM series LEFT JOIN dbseries ON series.dbserie_id = dbseries.id%s ORDER BY dbseries.seriename LIMIT ? OFFSET ?",
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
			"SELECT COALESCE(dbseries.seriename, 'Unknown Series') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.identifier IS NOT NULL AND dbserie_episodes.identifier != 'S00E00' THEN dbserie_episodes.identifier ELSE 'ID:' || serie_episodes.id END, 'Unknown') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.title IS NOT NULL AND TRIM(dbserie_episodes.title) != '' THEN dbserie_episodes.title ELSE 'Episode ' || COALESCE(dbserie_episodes.episode, 'Unknown') END, 'Unknown Episode') || ' (' || series.listname || ')', serie_episodes.id FROM serie_episodes LEFT JOIN dbserie_episodes ON serie_episodes.dbserie_episode_id = dbserie_episodes.id LEFT JOIN series ON serie_episodes.serie_id = series.id LEFT JOIN dbseries ON series.dbserie_id = dbseries.id%s ORDER BY dbseries.seriename, series.listname, dbserie_episodes.season, dbserie_episodes.episode LIMIT ? OFFSET ?",
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
			searchFilter = searchFilter + typeFilter
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

	case "listnames":
		var query string
		switch tableName {
		case "movies":
			query = fmt.Sprintf(
				"SELECT DISTINCT listname, listname FROM movies%s ORDER BY listname LIMIT ? OFFSET ?",
				searchFilter,
			)

		case "series":
			query = fmt.Sprintf(
				"SELECT DISTINCT listname, listname FROM series%s ORDER BY listname LIMIT ? OFFSET ?",
				searchFilter,
			)

		default:
			query = fmt.Sprintf(
				"SELECT DISTINCT listname, listname FROM %s%s ORDER BY listname LIMIT ? OFFSET ?",
				tableName,
				searchFilter,
			)
		}

		listnames := database.GetrowsN[database.DbstaticTwoString](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(listnames) > pageSize
		if hasMore {
			listnames = listnames[:pageSize]
		}

		for _, listname := range listnames {
			options = append(options, createSelect2OptionString(listname.Str1, listname.Str1))
		}

	case "quality_profiles":
		var query string
		switch tableName {
		case "movies":
			query = fmt.Sprintf(
				"SELECT DISTINCT quality_profile, quality_profile FROM movies%s ORDER BY quality_profile LIMIT ? OFFSET ?",
				searchFilter,
			)

		case "series":
			query = fmt.Sprintf(
				"SELECT DISTINCT quality_profile, quality_profile FROM serie_episodes%s ORDER BY quality_profile LIMIT ? OFFSET ?",
				searchFilter,
			)

		case "movie_histories":
			query = fmt.Sprintf(
				"SELECT DISTINCT quality_profile, quality_profile FROM movie_histories%s ORDER BY quality_profile LIMIT ? OFFSET ?",
				searchFilter,
			)

		case "serie_episode_histories":
			query = fmt.Sprintf(
				"SELECT DISTINCT quality_profile, quality_profile FROM serie_episode_histories%s ORDER BY quality_profile LIMIT ? OFFSET ?",
				searchFilter,
			)

		default:
			query = fmt.Sprintf(
				"SELECT DISTINCT quality_profile, quality_profile FROM %s%s ORDER BY quality_profile LIMIT ? OFFSET ?",
				tableName,
				searchFilter,
			)
		}

		profiles := database.GetrowsN[database.DbstaticTwoString](
			false,
			uint(pageSize+1),
			query,
			searchArgs...)

		hasMore = len(profiles) > pageSize
		if hasMore {
			profiles = profiles[:pageSize]
		}

		for _, profile := range profiles {
			if profile.Str1 != "" {
				options = append(options, createSelect2OptionString(profile.Str1, profile.Str1))
			}
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
			"SELECT dbbooks.title || ' - ' || books.listname, books.id FROM books LEFT JOIN dbbooks ON books.dbbook_id = dbbooks.id%s ORDER BY dbbooks.title LIMIT ? OFFSET ?",
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
			"SELECT dbaudiobooks.title || ' - ' || audiobooks.listname, audiobooks.id FROM audiobooks LEFT JOIN dbaudiobooks ON audiobooks.dbaudiobook_id = dbaudiobooks.id%s ORDER BY dbaudiobooks.title LIMIT ? OFFSET ?",
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
			"SELECT dbalbums.title || ' - ' || albums.listname, albums.id FROM albums LEFT JOIN dbalbums ON albums.dbalbum_id = dbalbums.id%s ORDER BY dbalbums.title LIMIT ? OFFSET ?",
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
			"SELECT dbauthors.name || ' - ' || authors.listname, authors.id FROM authors LEFT JOIN dbauthors ON authors.dbauthor_id = dbauthors.id%s ORDER BY dbauthors.name LIMIT ? OFFSET ?",
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
			"SELECT dbbook_series.name || ' - ' || book_series.listname, book_series.id FROM book_series LEFT JOIN dbbook_series ON book_series.dbbook_series_id = dbbook_series.id%s ORDER BY dbbook_series.name LIMIT ? OFFSET ?",
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
		query := fmt.Sprintf(
			"SELECT title, id FROM dbtracks%s ORDER BY track_number LIMIT ? OFFSET ?",
			searchFilter,
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
			"SELECT dbartists.name || ' - ' || artists.listname, artists.id FROM artists LEFT JOIN dbartists ON artists.dbartist_id = dbartists.id%s ORDER BY dbartists.name LIMIT ? OFFSET ?",
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
// @Success      200    {object} gin.H{"success": bool}
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
// @Success      200    {object} gin.H{"success": bool}
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
// @Success      200    {object} gin.H{"success": bool}
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

	// logger.Logtype("info", 1).Str("method", ctx.Request.Method).Msg("table")

	size, _ := strconv.Atoi(getParam(ctx, "iDisplayLength", "10"))
	start, _ := strconv.Atoi(getParam(ctx, "iDisplayStart", "0"))
	// order := "id"
	tabledefault := database.GetTableDefaults(tableName)
	columns := strings.Split(tabledefault.DefaultColumns, ",")

	orderid := getParam(ctx, "iSortCol_0", "0")

	order := columns[logger.StringToInt(orderid)]
	if logger.ContainsI(order, " as ") {
		order = strings.Split(order, " as ")[1]
	}

	direction := getParam(ctx, "sSortDir_0", "asc")

	orderby := "order by " + order + " " + direction
	searchValue := getParamValue(ctx, "sSearch")

	// Build custom filters
	customFilters, customArgs := buildCustomFilters(tableName, ctx)

	var final, total int
	// Handle special case for tables with JOINs
	countTable := tableName
	switch tableName {
	case "dbmovie_titles":
		countTable = "dbmovie_titles"
	case "dbserie_alternates":
		countTable = "dbserie_alternates"
	case "dbserie_episodes":
		countTable = "dbserie_episodes"
	case "movies":
		countTable = "movies"
	case "series":
		countTable = "series"
	case "serie_episodes":
		countTable = "serie_episodes"
	case "movie_file_unmatcheds":
		countTable = "movie_file_unmatcheds"
	case "serie_file_unmatcheds":
		countTable = "serie_file_unmatcheds"
	case "movie_files":
		countTable = "movie_files"
	case "serie_episode_files":
		countTable = "serie_episode_files"
	case "movie_histories":
		countTable = "movie_histories"
	case "serie_episode_histories":
		countTable = "serie_episode_histories"

	// Book tables
	case "dbbooks":
		countTable = "dbbooks"
	case "dbauthors":
		countTable = "dbauthors"
	case "dbbook_titles":
		countTable = "dbbook_titles"
	case "dbbook_series":
		countTable = "dbbook_series"
	case "books":
		countTable = "books"
	case "book_files":
		countTable = "book_files"
	case "authors":
		countTable = "authors"
	case "book_series":
		countTable = "book_series"
	case "book_file_unmatcheds":
		countTable = "book_file_unmatcheds"
	case "book_histories":
		countTable = "book_histories"

	// Audiobook tables
	case "dbaudiobooks":
		countTable = "dbaudiobooks"
	case "dbnarrators":
		countTable = "dbnarrators"
	case "dbaudiobook_titles":
		countTable = "dbaudiobook_titles"
	case "audiobooks":
		countTable = "audiobooks"
	case "audiobook_files":
		countTable = "audiobook_files"
	case "audiobook_file_unmatcheds":
		countTable = "audiobook_file_unmatcheds"
	case "audiobook_histories":
		countTable = "audiobook_histories"

	// Music tables
	case "dbalbums":
		countTable = "dbalbums"
	case "dbartists":
		countTable = "dbartists"
	case "dbalbum_titles":
		countTable = "dbalbum_titles"
	case "dbtracks":
		countTable = "dbtracks"
	case "albums":
		countTable = "albums"
	case "album_files":
		countTable = "album_files"
	case "artists":
		countTable = "artists"
	case "album_file_unmatcheds":
		countTable = "album_file_unmatcheds"
	case "album_histories":
		countTable = "album_histories"
	}

	database.Scanrowsdyn(false, "select Count(*) as frequency FROM "+countTable, &total)

	// Build the complete WHERE clause
	var (
		whereClause string
		queryArgs   []any
	)

	if searchValue != "" || customFilters != "" {
		var conditions []string

		// Add general search condition
		if searchValue != "" {
			var aux []any
			for range tabledefault.DefaultQueryParamCount {
				aux = append(aux, "%"+searchValue+"%")
			}

			if tabledefault.DefaultQuery != "" {
				// Trim both " where " and " WHERE " prefixes (case insensitive)
				queryCondition := strings.TrimSpace(tabledefault.DefaultQuery)

				queryCondition = strings.TrimPrefix(queryCondition, "WHERE ")
				queryCondition = strings.TrimPrefix(queryCondition, "where ")
				conditions = append(
					conditions,
					queryCondition,
				)
				queryArgs = append(queryArgs, aux...)
			}
		}

		// Add custom filters
		if customFilters != "" {
			conditions = append(conditions, customFilters)
			queryArgs = append(queryArgs, customArgs...)
		}

		if len(conditions) > 0 {
			whereClause = "WHERE " + strings.Join(conditions, " AND ")
		}

		data := database.GetrowsType(
			tabledefault.Object,
			false,
			1000,
			"select "+tabledefault.DefaultColumns+" from "+tabledefault.Table+" "+whereClause+" "+orderby+" LIMIT ?, ?",
			append(queryArgs, start, size)...)
		retdata := make([][]string, 0, len(data))

		splitted := strings.Split(tabledefault.DefaultColumns, ",")
		for _, loop := range data {
			row := make([]string, 0, len(splitted))
			for _, v := range splitted {
				v = strings.TrimSpace(v)
				if logger.ContainsI(v, " as ") {
					v = strings.Split(v, " as ")[1]
				}

				row = append(row, fmt.Sprint(loop[v]))
			}

			retdata = append(retdata, row)
		}

		// Count filtered results
		database.Scanrowsdyn(
			false,
			"select Count(*) as frequency FROM "+tabledefault.Table+" "+whereClause,
			&final,
			queryArgs...)
		sendDataTablesResponse(ctx, total, final, retdata)

		return
	} else {
		data := database.GetrowsType(
			tabledefault.Object,
			false,
			1000,
			"select "+tabledefault.DefaultColumns+" from "+tabledefault.Table+" "+orderby+" LIMIT ?, ?",
			start,
			size,
		)
		retdata := make([][]string, 0, len(data))

		splitted := strings.Split(tabledefault.DefaultColumns, ",")
		for _, loop := range data {
			row := make([]string, 0, len(splitted))
			for _, v := range splitted {
				v = strings.TrimSpace(v)
				if logger.ContainsI(v, " as ") {
					v = strings.Split(v, " as ")[1]
				}

				row = append(row, fmt.Sprint(loop[v]))
			}

			retdata = append(retdata, row)
		}

		database.Scanrowsdyn(false, "select Count(*) as frequency FROM "+countTable, &final)
		sendDataTablesResponse(ctx, total, final, retdata)
	}
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

	var rowMap map[string]any

	switch tableName {
	case "dbmovies":
		// Get real movie data using StructscanT
		movie, err := database.Structscan[database.Dbmovie](
			"SELECT ID, Title, Year, Imdb_id, Original_title, overview, runtime, genres, original_language, status, vote_average, vote_count, popularity, budget, revenue, created_at, updated_at FROM dbmovies WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":                movie.ID,
			"title":             movie.Title,
			"year":              movie.Year,
			"imdb_id":           movie.ImdbID,
			"original_title":    movie.OriginalTitle,
			"overview":          movie.Overview,
			"runtime":           movie.Runtime,
			"genres":            movie.Genres,
			"original_language": movie.OriginalLanguage,
			"status":            movie.Status,
			"vote_average":      movie.VoteAverage,
			"vote_count":        movie.VoteCount,
			"popularity":        movie.Popularity,
			"budget":            movie.Budget,
			"revenue":           movie.Revenue,
			"created_at":        movie.CreatedAt,
			"updated_at":        movie.UpdatedAt,
		}

	case "dbmovie_titles":
		dbmovietitle, err := database.Structscan[database.DbmovieTitle](
			"SELECT id, title, slug, region, created_at, updated_at, dbmovie_id FROM dbmovie_titles WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         dbmovietitle.ID,
			"title":      dbmovietitle.Title,
			"slug":       dbmovietitle.Slug,
			"region":     dbmovietitle.Region,
			"created_at": dbmovietitle.CreatedAt,
			"updated_at": dbmovietitle.UpdatedAt,
			"dbmovie_id": dbmovietitle.DbmovieID,
		}

	case "dbseries":
		// Get real series data using StructscanT
		serie, err := database.Structscan[database.Dbserie](
			"SELECT id, seriename, imdb_id, thetvdb_id, status, firstaired, network, runtime, language, genre, overview, rating, created_at, updated_at FROM dbseries WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         serie.ID,
			"seriename":  serie.Seriename,
			"imdb_id":    serie.ImdbID,
			"thetvdb_id": serie.ThetvdbID,
			"status":     serie.Status,
			"firstaired": serie.Firstaired,
			"network":    serie.Network,
			"runtime":    serie.Runtime,
			"language":   serie.Language,
			"genre":      serie.Genre,
			"overview":   serie.Overview,
			"rating":     serie.Rating,
			"created_at": serie.CreatedAt,
			"updated_at": serie.UpdatedAt,
		}

	case "dbserie_episodes":
		// Get real episode data using StructscanT
		episode, err := database.Structscan[database.DbserieEpisode](
			"SELECT id, title, season, episode, identifier, first_aired, overview, runtime, dbserie_id, created_at, updated_at FROM dbserie_episodes WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":          episode.ID,
			"title":       episode.Title,
			"season":      episode.Season,
			"episode":     episode.Episode,
			"identifier":  episode.Identifier,
			"first_aired": episode.FirstAired.Time,
			"overview":    episode.Overview,
			"runtime":     episode.Runtime,
			"dbserie_id":  episode.DbserieID,
			"created_at":  episode.CreatedAt,
			"updated_at":  episode.UpdatedAt,
		}

	case "dbserie_alternates":
		// Get real alternate data using StructscanT
		alt, err := database.Structscan[database.DbserieAlternate](
			"SELECT id, title, slug, region, dbserie_id, created_at, updated_at FROM dbserie_alternates WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         alt.ID,
			"title":      alt.Title,
			"slug":       alt.Slug,
			"region":     alt.Region,
			"dbserie_id": alt.DbserieID,
			"created_at": alt.CreatedAt,
			"updated_at": alt.UpdatedAt,
		}

	case "movies":
		// Get real movies table data using StructscanT
		movie, err := database.Structscan[database.Movie](
			"SELECT id, listname, rootpath, dbmovie_id, quality_profile, quality_reached, missing, blacklisted, dont_upgrade, dont_search, created_at, updated_at FROM movies WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              movie.ID,
			"listname":        movie.Listname,
			"rootpath":        movie.Rootpath,
			"dbmovie_id":      movie.DbmovieID,
			"quality_profile": movie.QualityProfile,
			"quality_reached": movie.QualityReached,
			"missing":         movie.Missing,
			"blacklisted":     movie.Blacklisted,
			"dont_upgrade":    movie.DontUpgrade,
			"dont_search":     movie.DontSearch,
			"created_at":      movie.CreatedAt,
			"updated_at":      movie.UpdatedAt,
		}

	case "movie_file_unmatcheds":
		// Get real movie file unmatched data using StructscanT
		movieFileUnmatched, err := database.Structscan[database.MovieFileUnmatched](
			"SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM movie_file_unmatcheds WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           movieFileUnmatched.ID,
			"listname":     movieFileUnmatched.Listname,
			"filepath":     movieFileUnmatched.Filepath,
			"parsed_data":  movieFileUnmatched.ParsedData,
			"last_checked": movieFileUnmatched.LastChecked.Time,
			"created_at":   movieFileUnmatched.CreatedAt,
			"updated_at":   movieFileUnmatched.UpdatedAt,
		}

	case "movie_histories":
		// Get real movie history data using StructscanT
		movieHistory, err := database.Structscan[database.MovieHistory](
			"SELECT id, title, url, indexer, target, quality_profile, created_at, updated_at, downloaded_at, resolution_id, quality_id, codec_id, audio_id, movie_id, dbmovie_id, blacklisted FROM movie_histories WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              movieHistory.ID,
			"title":           movieHistory.Title,
			"url":             movieHistory.URL,
			"indexer":         movieHistory.Indexer,
			"target":          movieHistory.Target,
			"quality_profile": movieHistory.QualityProfile,
			"created_at":      movieHistory.CreatedAt,
			"updated_at":      movieHistory.UpdatedAt,
			"downloaded_at":   movieHistory.DownloadedAt,
			"resolution_id":   movieHistory.ResolutionID,
			"quality_id":      movieHistory.QualityID,
			"codec_id":        movieHistory.CodecID,
			"audio_id":        movieHistory.AudioID,
			"movie_id":        movieHistory.MovieID,
			"dbmovie_id":      movieHistory.DbmovieID,
			"blacklisted":     movieHistory.Blacklisted,
		}

	case "movie_files":
		// Get real movie files data using StructscanT
		movieFile, err := database.Structscan[database.MovieFile](
			"SELECT id, location, extension, quality_profile, created_at, updated_at, resolution_id, quality_id, codec_id, audio_id, movie_id, dbmovie_id, height, width, proper, extended, repack FROM movie_files WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              movieFile.ID,
			"location":        movieFile.Location,
			"extension":       movieFile.Extension,
			"quality_profile": movieFile.QualityProfile,
			"created_at":      movieFile.CreatedAt,
			"updated_at":      movieFile.UpdatedAt,
			"resolution_id":   movieFile.ResolutionID,
			"quality_id":      movieFile.QualityID,
			"codec_id":        movieFile.CodecID,
			"audio_id":        movieFile.AudioID,
			"movie_id":        movieFile.MovieID,
			"dbmovie_id":      movieFile.DbmovieID,
			"height":          movieFile.Height,
			"width":           movieFile.Width,
			"proper":          movieFile.Proper,
			"extended":        movieFile.Extended,
			"repack":          movieFile.Repack,
		}

	case "series":
		// Get real series table data using StructscanT
		serie, err := database.Structscan[database.Serie](
			"SELECT id, listname, rootpath, dbserie_id, dont_upgrade, dont_search, created_at, updated_at FROM series WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           serie.ID,
			"listname":     serie.Listname,
			"rootpath":     serie.Rootpath,
			"dbserie_id":   serie.DbserieID,
			"dont_upgrade": serie.DontUpgrade,
			"dont_search":  serie.DontSearch,
			"created_at":   serie.CreatedAt,
			"updated_at":   serie.UpdatedAt,
		}

	case "serie_episodes":
		// Get real serie episodes data using StructscanT
		episode, err := database.Structscan[database.SerieEpisode](
			"SELECT id, quality_profile, lastscan, created_at, updated_at, dbserie_episode_id, serie_id, dbserie_id, blacklisted, quality_reached, missing, dont_upgrade, dont_search, ignore_runtime FROM serie_episodes WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":                 episode.ID,
			"quality_profile":    episode.QualityProfile,
			"lastscan":           episode.Lastscan.Time,
			"created_at":         episode.CreatedAt,
			"updated_at":         episode.UpdatedAt,
			"dbserie_episode_id": episode.DbserieEpisodeID,
			"serie_id":           episode.SerieID,
			"dbserie_id":         episode.DbserieID,
			"blacklisted":        episode.Blacklisted,
			"quality_reached":    episode.QualityReached,
			"missing":            episode.Missing,
			"dont_upgrade":       episode.DontUpgrade,
			"dont_search":        episode.DontSearch,
			"ignore_runtime":     episode.IgnoreRuntime,
		}

	case "serie_episode_files":
		// Get real serie files
		serieFile, err := database.Structscan[database.SerieEpisodeFile](
			"SELECT id, location, filename, extension, quality_profile, created_at, updated_at, resolution_id, quality_id, codec_id, audio_id, serie_id, serie_episode_id, dbserie_id, dbserie_episode_id, proper, extended, repack FROM serie_episode_files WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":                 serieFile.ID,
			"location":           serieFile.Location,
			"filename":           serieFile.Filename,
			"extension":          serieFile.Extension,
			"quality_profile":    serieFile.QualityProfile,
			"created_at":         serieFile.CreatedAt,
			"updated_at":         serieFile.UpdatedAt,
			"resolution_id":      serieFile.ResolutionID,
			"quality_id":         serieFile.QualityID,
			"codec_id":           serieFile.CodecID,
			"audio_id":           serieFile.AudioID,
			"serie_id":           serieFile.SerieID,
			"serie_episode_id":   serieFile.SerieEpisodeID,
			"dbserie_episode_id": serieFile.DbserieEpisodeID,
			"dbserie_id":         serieFile.DbserieID,
			"height":             serieFile.Height,
			"width":              serieFile.Width,
			"proper":             serieFile.Proper,
			"extended":           serieFile.Extended,
			"repack":             serieFile.Repack,
		}

	case "serie_file_unmatcheds":
		// Get real serie file unmatched data using StructscanT
		serieFileUnmatched, err := database.Structscan[database.SerieFileUnmatched](
			"SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM serie_file_unmatcheds WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           serieFileUnmatched.ID,
			"listname":     serieFileUnmatched.Listname,
			"filepath":     serieFileUnmatched.Filepath,
			"parsed_data":  serieFileUnmatched.ParsedData,
			"last_checked": serieFileUnmatched.LastChecked.Time,
			"created_at":   serieFileUnmatched.CreatedAt,
			"updated_at":   serieFileUnmatched.UpdatedAt,
		}

	case "serie_episode_histories":
		// Get real serie episode history data using StructscanT
		serieEpisodeHistory, err := database.Structscan[database.SerieEpisodeHistory](
			"SELECT id, title, url, indexer, target, quality_profile, created_at, updated_at, downloaded_at, resolution_id, quality_id, codec_id, audio_id, serie_id, serie_episode_id, dbserie_id, dbserie_episode_id FROM serie_episode_histories WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":                 serieEpisodeHistory.ID,
			"title":              serieEpisodeHistory.Title,
			"url":                serieEpisodeHistory.URL,
			"indexer":            serieEpisodeHistory.Indexer,
			"target":             serieEpisodeHistory.Target,
			"quality_profile":    serieEpisodeHistory.QualityProfile,
			"created_at":         serieEpisodeHistory.CreatedAt,
			"updated_at":         serieEpisodeHistory.UpdatedAt,
			"downloaded_at":      serieEpisodeHistory.DownloadedAt,
			"resolution_id":      serieEpisodeHistory.ResolutionID,
			"quality_id":         serieEpisodeHistory.QualityID,
			"codec_id":           serieEpisodeHistory.CodecID,
			"audio_id":           serieEpisodeHistory.AudioID,
			"serie_id":           serieEpisodeHistory.SerieID,
			"serie_episode_id":   serieEpisodeHistory.SerieEpisodeID,
			"dbserie_episode_id": serieEpisodeHistory.DbserieEpisodeID,
			"dbserie_id":         serieEpisodeHistory.DbserieID,
		}

	case "job_histories":
		// Get real job history data using StructscanT
		job, err := database.Structscan[database.JobHistory](
			"SELECT id, job_type, job_category, job_group, started, ended, created_at, updated_at FROM job_histories WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           job.ID,
			"job_type":     job.JobType,
			"job_category": job.JobCategory,
			"job_group":    job.JobGroup,
			"started":      job.Started.Time,
			"ended":        job.Ended.Time,
			"created_at":   job.CreatedAt,
			"updated_at":   job.UpdatedAt,
		}

	case "qualities":
		// Get real quality data using StructscanT
		quality, err := database.Structscan[database.Qualities](
			"SELECT id, name, regex, strings, created_at, updated_at, type, priority, regexgroup, use_regex FROM qualities WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         quality.ID,
			"name":       quality.Name,
			"regex":      quality.Regex,
			"strings":    quality.Strings,
			"created_at": quality.CreatedAt,
			"updated_at": quality.UpdatedAt,
			"type":       quality.QualityType,
			"priority":   quality.Priority,
			"regexgroup": quality.Regexgroup,
			"use_regex":  quality.UseRegex,
		}

	// Book tables
	case "dbbooks":
		book, err := database.Structscan[database.Dbbook](
			"SELECT id, title, original_title, isbn_13, isbn_10, asin, openlibrary_id, goodreads_id, description, publisher, publish_date, page_count, language, genres, cover_url, dbauthor_id, dbbook_series_id, series_position, average_rating, ratings_count, year, slug, created_at, updated_at FROM dbbooks WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":               book.ID,
			"title":            book.Title,
			"original_title":   book.OriginalTitle,
			"isbn_13":          book.ISBN13,
			"isbn_10":          book.ISBN10,
			"asin":             book.ASIN,
			"openlibrary_id":   book.OpenlibraryID,
			"goodreads_id":     book.GoodreadsID,
			"description":      book.Description,
			"publisher":        book.Publisher,
			"publish_date":     book.PublishDate.Time,
			"page_count":       book.PageCount,
			"language":         book.Language,
			"genres":           book.Genres,
			"cover_url":        book.CoverURL,
			"dbauthor_id":      book.DbauthorID,
			"dbbook_series_id": book.DbbookSeriesID,
			"series_position":  book.SeriesPosition,
			"average_rating":   book.AverageRating,
			"ratings_count":    book.RatingsCount,
			"year":             book.Year,
			"slug":             book.Slug,
			"created_at":       book.CreatedAt,
			"updated_at":       book.UpdatedAt,
		}

	case "dbauthors":
		author, err := database.Structscan[database.Dbauthor](
			"SELECT id, name, aliases, bio, birth_date, death_date, goodreads_id, openlibrary_id, website, image_url, created_at, updated_at FROM dbauthors WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":             author.ID,
			"name":           author.Name,
			"aliases":        author.Aliases,
			"bio":            author.Bio,
			"birth_date":     author.BirthDate,
			"death_date":     author.DeathDate,
			"goodreads_id":   author.GoodreadsID,
			"openlibrary_id": author.OpenlibraryID,
			"website":        author.Website,
			"image_url":      author.ImageURL,
			"created_at":     author.CreatedAt,
			"updated_at":     author.UpdatedAt,
		}

	case "dbbook_titles":
		bookTitle, err := database.Structscan[database.DbbookTitle](
			"SELECT id, title, slug, region, dbbook_id, created_at, updated_at FROM dbbook_titles WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         bookTitle.ID,
			"title":      bookTitle.Title,
			"slug":       bookTitle.Slug,
			"region":     bookTitle.Region,
			"dbbook_id":  bookTitle.DbbookID,
			"created_at": bookTitle.CreatedAt,
			"updated_at": bookTitle.UpdatedAt,
		}

	case "dbbook_series":
		bookSeries, err := database.Structscan[database.DbbookSeries](
			"SELECT id, name, description, goodreads_id, openlibrary_id, created_at, updated_at FROM dbbook_series WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":             bookSeries.ID,
			"name":           bookSeries.Name,
			"description":    bookSeries.Description,
			"goodreads_id":   bookSeries.GoodreadsID,
			"openlibrary_id": bookSeries.OpenlibraryID,
			"created_at":     bookSeries.CreatedAt,
			"updated_at":     bookSeries.UpdatedAt,
		}

	case "books":
		book, err := database.Structscan[database.Book](
			"SELECT id, quality_profile, listname, rootpath, lastscan, created_at, updated_at, dbbook_id, book_series_id, author_id, blacklisted, quality_reached, missing, dont_upgrade, dont_search FROM books WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              book.ID,
			"quality_profile": book.QualityProfile,
			"listname":        book.Listname,
			"rootpath":        book.Rootpath,
			"lastscan":        book.Lastscan.Time,
			"created_at":      book.CreatedAt,
			"updated_at":      book.UpdatedAt,
			"dbbook_id":       book.DbbookID,
			"book_series_id":  book.BookSeriesID,
			"author_id":       book.AuthorID,
			"blacklisted":     book.Blacklisted,
			"quality_reached": book.QualityReached,
			"missing":         book.Missing,
			"dont_upgrade":    book.DontUpgrade,
			"dont_search":     book.DontSearch,
		}

	case "book_files":
		bookFile, err := database.Structscan[database.BookFile](
			"SELECT id, location, filename, extension, format, quality_profile, created_at, updated_at, book_id, dbbook_id, file_size FROM book_files WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              bookFile.ID,
			"location":        bookFile.Location,
			"filename":        bookFile.Filename,
			"extension":       bookFile.Extension,
			"format":          bookFile.Format,
			"quality_profile": bookFile.QualityProfile,
			"created_at":      bookFile.CreatedAt,
			"updated_at":      bookFile.UpdatedAt,
			"book_id":         bookFile.BookID,
			"dbbook_id":       bookFile.DbbookID,
			"file_size":       bookFile.FileSize,
		}

	case "authors":
		author, err := database.Structscan[database.Author](
			"SELECT id, listname, track_mode, created_at, updated_at, dbauthor_id, dont_search FROM authors WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":          author.ID,
			"listname":    author.Listname,
			"track_mode":  author.TrackMode,
			"created_at":  author.CreatedAt,
			"updated_at":  author.UpdatedAt,
			"dbauthor_id": author.DbauthorID,
			"dont_search": author.DontSearch,
		}

	case "book_series":
		bookSeries, err := database.Structscan[database.BookSeries](
			"SELECT id, listname, created_at, updated_at, dbbook_series_id, author_id, dont_search FROM book_series WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":               bookSeries.ID,
			"listname":         bookSeries.Listname,
			"created_at":       bookSeries.CreatedAt,
			"updated_at":       bookSeries.UpdatedAt,
			"dbbook_series_id": bookSeries.DbbookSeriesID,
			"author_id":        bookSeries.AuthorID,
			"dont_search":      bookSeries.DontSearch,
		}

	case "book_file_unmatcheds":
		bookFileUnmatched, err := database.Structscan[database.BookFileUnmatched](
			"SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM book_file_unmatcheds WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           bookFileUnmatched.ID,
			"listname":     bookFileUnmatched.Listname,
			"filepath":     bookFileUnmatched.Filepath,
			"parsed_data":  bookFileUnmatched.ParsedData,
			"last_checked": bookFileUnmatched.LastChecked.Time,
			"created_at":   bookFileUnmatched.CreatedAt,
			"updated_at":   bookFileUnmatched.UpdatedAt,
		}

	case "book_histories":
		bookHistory, err := database.Structscan[database.BookHistory](
			"SELECT id, title, url, indexer, type, target, quality_profile, created_at, updated_at, downloaded_at, book_id, dbbook_id, blacklisted FROM book_histories WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              bookHistory.ID,
			"title":           bookHistory.Title,
			"url":             bookHistory.URL,
			"indexer":         bookHistory.Indexer,
			"type":            bookHistory.HistoryType,
			"target":          bookHistory.Target,
			"quality_profile": bookHistory.QualityProfile,
			"created_at":      bookHistory.CreatedAt,
			"updated_at":      bookHistory.UpdatedAt,
			"downloaded_at":   bookHistory.DownloadedAt,
			"book_id":         bookHistory.BookID,
			"dbbook_id":       bookHistory.DbbookID,
			"blacklisted":     bookHistory.Blacklisted,
		}

	// Audiobook tables
	case "dbaudiobooks":
		audiobook, err := database.Structscan[database.Dbaudiobook](
			"SELECT id, title, asin, audible_id, runtime_minutes, chapter_count, release_date, publisher, language, abridged, cover_url, sample_url, average_rating, ratings_count, year, slug, dbbook_id, description, created_at, updated_at FROM dbaudiobooks WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              audiobook.ID,
			"title":           audiobook.Title,
			"asin":            audiobook.ASIN,
			"audible_id":      audiobook.AudibleID,
			"runtime_minutes": audiobook.RuntimeMinutes,
			"chapter_count":   audiobook.ChapterCount,
			"release_date":    audiobook.ReleaseDate.Time,
			"publisher":       audiobook.Publisher,
			"language":        audiobook.Language,
			"abridged":        audiobook.Abridged,
			"cover_url":       audiobook.CoverURL,
			"sample_url":      audiobook.SampleURL,
			"average_rating":  audiobook.AverageRating,
			"ratings_count":   audiobook.RatingsCount,
			"year":            audiobook.Year,
			"slug":            audiobook.Slug,
			"dbbook_id":       audiobook.DbbookID,
			"description":     audiobook.Description,
			"created_at":      audiobook.CreatedAt,
			"updated_at":      audiobook.UpdatedAt,
		}

	case "dbnarrators":
		narrator, err := database.Structscan[database.Dbnarrator](
			"SELECT id, name, audible_id, bio, image_url, created_at, updated_at FROM dbnarrators WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         narrator.ID,
			"name":       narrator.Name,
			"audible_id": narrator.AudibleID,
			"bio":        narrator.Bio,
			"image_url":  narrator.ImageURL,
			"created_at": narrator.CreatedAt,
			"updated_at": narrator.UpdatedAt,
		}

	case "dbaudiobook_titles":
		audiobookTitle, err := database.Structscan[database.DbaudiobookTitle](
			"SELECT id, title, slug, region, dbaudiobook_id, created_at, updated_at FROM dbaudiobook_titles WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":             audiobookTitle.ID,
			"title":          audiobookTitle.Title,
			"slug":           audiobookTitle.Slug,
			"region":         audiobookTitle.Region,
			"dbaudiobook_id": audiobookTitle.DbaudiobookID,
			"created_at":     audiobookTitle.CreatedAt,
			"updated_at":     audiobookTitle.UpdatedAt,
		}

	case "audiobooks":
		audiobook, err := database.Structscan[database.Audiobook](
			"SELECT id, quality_profile, listname, rootpath, lastscan, created_at, updated_at, dbaudiobook_id, author_id, book_series_id, blacklisted, quality_reached, missing, dont_upgrade, dont_search FROM audiobooks WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              audiobook.ID,
			"quality_profile": audiobook.QualityProfile,
			"listname":        audiobook.Listname,
			"rootpath":        audiobook.Rootpath,
			"lastscan":        audiobook.Lastscan.Time,
			"created_at":      audiobook.CreatedAt,
			"updated_at":      audiobook.UpdatedAt,
			"dbaudiobook_id":  audiobook.DbaudiobookID,
			"author_id":       audiobook.AuthorID,
			"book_series_id":  audiobook.BookSeriesID,
			"blacklisted":     audiobook.Blacklisted,
			"quality_reached": audiobook.QualityReached,
			"missing":         audiobook.Missing,
			"dont_upgrade":    audiobook.DontUpgrade,
			"dont_search":     audiobook.DontSearch,
		}

	case "audiobook_files":
		audiobookFile, err := database.Structscan[database.AudiobookFile](
			"SELECT id, location, filename, extension, format, quality_profile, created_at, updated_at, audiobook_id, dbaudiobook_id, file_size, bitrate, runtime_ms, track_number, disc_number FROM audiobook_files WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              audiobookFile.ID,
			"location":        audiobookFile.Location,
			"filename":        audiobookFile.Filename,
			"extension":       audiobookFile.Extension,
			"format":          audiobookFile.Format,
			"quality_profile": audiobookFile.QualityProfile,
			"created_at":      audiobookFile.CreatedAt,
			"updated_at":      audiobookFile.UpdatedAt,
			"audiobook_id":    audiobookFile.AudiobookID,
			"dbaudiobook_id":  audiobookFile.DbaudiobookID,
			"file_size":       audiobookFile.FileSize,
			"bitrate":         audiobookFile.Bitrate,
			"runtime_ms":      audiobookFile.RuntimeMs,
			"track_number":    audiobookFile.TrackNumber,
			"disc_number":     audiobookFile.DiscNumber,
		}

	case "audiobook_file_unmatcheds":
		audiobookFileUnmatched, err := database.Structscan[database.AudiobookFileUnmatched](
			"SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM audiobook_file_unmatcheds WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           audiobookFileUnmatched.ID,
			"listname":     audiobookFileUnmatched.Listname,
			"filepath":     audiobookFileUnmatched.Filepath,
			"parsed_data":  audiobookFileUnmatched.ParsedData,
			"last_checked": audiobookFileUnmatched.LastChecked.Time,
			"created_at":   audiobookFileUnmatched.CreatedAt,
			"updated_at":   audiobookFileUnmatched.UpdatedAt,
		}

	case "audiobook_histories":
		audiobookHistory, err := database.Structscan[database.AudiobookHistory](
			"SELECT id, title, url, indexer, type, target, quality_profile, created_at, updated_at, downloaded_at, audiobook_id, dbaudiobook_id, blacklisted FROM audiobook_histories WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              audiobookHistory.ID,
			"title":           audiobookHistory.Title,
			"url":             audiobookHistory.URL,
			"indexer":         audiobookHistory.Indexer,
			"type":            audiobookHistory.HistoryType,
			"target":          audiobookHistory.Target,
			"quality_profile": audiobookHistory.QualityProfile,
			"created_at":      audiobookHistory.CreatedAt,
			"updated_at":      audiobookHistory.UpdatedAt,
			"downloaded_at":   audiobookHistory.DownloadedAt,
			"audiobook_id":    audiobookHistory.AudiobookID,
			"dbaudiobook_id":  audiobookHistory.DbaudiobookID,
			"blacklisted":     audiobookHistory.Blacklisted,
		}

	// Music tables
	case "dbalbums":
		album, err := database.Structscan[database.Dbalbum](
			"SELECT id, title, musicbrainz_release_group_id, musicbrainz_release_id, discogs_master_id, discogs_release_id, spotify_id, upc, release_date, release_type, format, label, country, total_tracks, total_runtime_ms, genres, styles, cover_url, year, slug, created_at, updated_at FROM dbalbums WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":                           album.ID,
			"title":                        album.Title,
			"musicbrainz_release_group_id": album.MusicbrainzReleaseGroupID,
			"musicbrainz_release_id":       album.MusicbrainzReleaseID,
			"discogs_master_id":            album.DiscogsMasterID,
			"discogs_release_id":           album.DiscogsReleaseID,
			"spotify_id":                   album.SpotifyID,
			"upc":                          album.UPC,
			"release_date":                 album.ReleaseDate.Time,
			"release_type":                 album.ReleaseType,
			"format":                       album.Format,
			"label":                        album.Label,
			"country":                      album.Country,
			"total_tracks":                 album.TotalTracks,
			"total_runtime_ms":             album.TotalRuntimeMs,
			"genres":                       album.Genres,
			"styles":                       album.Styles,
			"cover_url":                    album.CoverURL,
			"year":                         album.Year,
			"slug":                         album.Slug,
			"created_at":                   album.CreatedAt,
			"updated_at":                   album.UpdatedAt,
		}

	case "dbartists":
		artist, err := database.Structscan[database.Dbartist](
			"SELECT id, name, sort_name, musicbrainz_id, discogs_id, spotify_id, artist_type, country, begin_date, end_date, disambiguation, bio, image_url, genres, created_at, updated_at FROM dbartists WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":             artist.ID,
			"name":           artist.Name,
			"sort_name":      artist.SortName,
			"musicbrainz_id": artist.MusicbrainzID,
			"discogs_id":     artist.DiscogsID,
			"spotify_id":     artist.SpotifyID,
			"artist_type":    artist.ArtistType,
			"country":        artist.Country,
			"begin_date":     artist.BeginDate,
			"end_date":       artist.EndDate,
			"disambiguation": artist.Disambiguation,
			"bio":            artist.Bio,
			"image_url":      artist.ImageURL,
			"genres":         artist.Genres,
			"created_at":     artist.CreatedAt,
			"updated_at":     artist.UpdatedAt,
		}

	case "dbalbum_titles":
		albumTitle, err := database.Structscan[database.DbalbumTitle](
			"SELECT id, title, slug, region, dbalbum_id, created_at, updated_at FROM dbalbum_titles WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":         albumTitle.ID,
			"title":      albumTitle.Title,
			"slug":       albumTitle.Slug,
			"region":     albumTitle.Region,
			"dbalbum_id": albumTitle.DbalbumID,
			"created_at": albumTitle.CreatedAt,
			"updated_at": albumTitle.UpdatedAt,
		}

	case "dbtracks":
		track, err := database.Structscan[database.Dbtrack](
			"SELECT id, title, musicbrainz_recording_id, isrc, acoustid, created_at, updated_at, runtime_ms, dbalbum_id, disc_number, track_number, explicit FROM dbtracks WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":                       track.ID,
			"title":                    track.Title,
			"musicbrainz_recording_id": track.MusicbrainzRecordingID,
			"isrc":                     track.ISRC,
			"acoustid":                 track.AcoustID,
			"created_at":               track.CreatedAt,
			"updated_at":               track.UpdatedAt,
			"runtime_ms":               track.RuntimeMs,
			"dbalbum_id":               track.DbalbumID,
			"disc_number":              track.DiscNumber,
			"track_number":             track.TrackNumber,
			"explicit":                 track.Explicit,
		}

	case "artists":
		artist, err := database.Structscan[database.Artist](
			"SELECT id, listname, track_mode, created_at, updated_at, dbartist_id, dont_search FROM artists WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":          artist.ID,
			"listname":    artist.Listname,
			"track_mode":  artist.TrackMode,
			"created_at":  artist.CreatedAt,
			"updated_at":  artist.UpdatedAt,
			"dbartist_id": artist.DbartistID,
			"dont_search": artist.DontSearch,
		}

	case "albums":
		album, err := database.Structscan[database.Album](
			"SELECT id, quality_profile, listname, rootpath, lastscan, created_at, updated_at, dbalbum_id, artist_id, blacklisted, quality_reached, missing, dont_upgrade, dont_search FROM albums WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              album.ID,
			"quality_profile": album.QualityProfile,
			"listname":        album.Listname,
			"rootpath":        album.Rootpath,
			"lastscan":        album.Lastscan.Time,
			"created_at":      album.CreatedAt,
			"updated_at":      album.UpdatedAt,
			"dbalbum_id":      album.DbalbumID,
			"artist_id":       album.ArtistID,
			"blacklisted":     album.Blacklisted,
			"quality_reached": album.QualityReached,
			"missing":         album.Missing,
			"dont_upgrade":    album.DontUpgrade,
			"dont_search":     album.DontSearch,
		}

	case "album_files":
		albumFile, err := database.Structscan[database.AlbumFile](
			"SELECT id, location, filename, extension, format, quality_profile, acoustid, created_at, updated_at, album_id, dbalbum_id, dbtrack_id, file_size, bitrate, sample_rate, bit_depth, runtime_ms, disc_number, track_number FROM album_files WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              albumFile.ID,
			"location":        albumFile.Location,
			"filename":        albumFile.Filename,
			"extension":       albumFile.Extension,
			"format":          albumFile.Format,
			"quality_profile": albumFile.QualityProfile,
			"acoustid":        albumFile.AcoustID,
			"created_at":      albumFile.CreatedAt,
			"updated_at":      albumFile.UpdatedAt,
			"album_id":        albumFile.AlbumID,
			"dbalbum_id":      albumFile.DbalbumID,
			"dbtrack_id":      albumFile.DbtrackID,
			"file_size":       albumFile.FileSize,
			"bitrate":         albumFile.Bitrate,
			"sample_rate":     albumFile.SampleRate,
			"bit_depth":       albumFile.BitDepth,
			"runtime_ms":      albumFile.RuntimeMs,
			"disc_number":     albumFile.DiscNumber,
			"track_number":    albumFile.TrackNumber,
		}

	case "album_file_unmatcheds":
		albumFileUnmatched, err := database.Structscan[database.AlbumFileUnmatched](
			"SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM album_file_unmatcheds WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":           albumFileUnmatched.ID,
			"listname":     albumFileUnmatched.Listname,
			"filepath":     albumFileUnmatched.Filepath,
			"parsed_data":  albumFileUnmatched.ParsedData,
			"last_checked": albumFileUnmatched.LastChecked.Time,
			"created_at":   albumFileUnmatched.CreatedAt,
			"updated_at":   albumFileUnmatched.UpdatedAt,
		}

	case "album_histories":
		albumHistory, err := database.Structscan[database.AlbumHistory](
			"SELECT id, title, url, indexer, type, target, quality_profile, created_at, updated_at, downloaded_at, album_id, dbalbum_id, blacklisted FROM album_histories WHERE ID = ?",
			false,
			id,
		)
		if err != nil {
			sendBadRequest(ctx, err.Error())
			return
		}

		rowMap = map[string]any{
			"id":              albumHistory.ID,
			"title":           albumHistory.Title,
			"url":             albumHistory.URL,
			"indexer":         albumHistory.Indexer,
			"type":            albumHistory.HistoryType,
			"target":          albumHistory.Target,
			"quality_profile": albumHistory.QualityProfile,
			"created_at":      albumHistory.CreatedAt,
			"updated_at":      albumHistory.UpdatedAt,
			"downloaded_at":   albumHistory.DownloadedAt,
			"album_id":        albumHistory.AlbumID,
			"dbalbum_id":      albumHistory.DbalbumID,
			"blacklisted":     albumHistory.Blacklisted,
		}
	}

	var buf strings.Builder
	renderTableEditForm(tableName, rowMap, id, getCSRFToken(ctx)).Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

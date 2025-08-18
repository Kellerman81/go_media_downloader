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

// apiAdminDropdownData provides AJAX endpoint for dynamic dropdown data loading
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

	var options []map[string]any
	var hasMore bool

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
		query := fmt.Sprintf("SELECT title, id FROM dbmovies%s ORDER BY title LIMIT ? OFFSET ?", searchFilter)
		movies := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(movies) > pageSize
		if hasMore {
			movies = movies[:pageSize]
		}
		for _, movie := range movies {
			options = append(options, createSelect2Option(movie.Num, movie.Str))
		}
	case "dbseries":
		query := fmt.Sprintf("SELECT seriename, id FROM dbseries%s ORDER BY seriename LIMIT ? OFFSET ?", searchFilter)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}
		for _, serie := range series {
			options = append(options, createSelect2Option(serie.Num, serie.Str))
		}
	case "dbserie_episodes":
		query := fmt.Sprintf("SELECT identifier, title, id FROM dbserie_episodes%s ORDER BY identifier LIMIT ? OFFSET ?", searchFilter)
		episodes := database.GetrowsN[syncops.DbstaticTwoStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(episodes) > pageSize
		if hasMore {
			episodes = episodes[:pageSize]
		}
		for _, episode := range episodes {
			label := fmt.Sprintf("%s - %s", episode.Str1, episode.Str2)
			options = append(options, createSelect2Option(episode.Num, label))
		}
	case "movies":
		query := fmt.Sprintf("SELECT dbmovies.title || ' - ' || movies.listname, movies.id FROM movies LEFT JOIN dbmovies ON movies.dbmovie_id = dbmovies.id%s ORDER BY dbmovies.title LIMIT ? OFFSET ?", searchFilter)
		movies := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(movies) > pageSize
		if hasMore {
			movies = movies[:pageSize]
		}
		for _, movie := range movies {
			options = append(options, createSelect2Option(movie.Num, movie.Str))
		}
	case "series":
		query := fmt.Sprintf("SELECT dbseries.seriename || ' - ' || series.listname, series.id FROM series LEFT JOIN dbseries ON series.dbserie_id = dbseries.id%s ORDER BY dbseries.seriename LIMIT ? OFFSET ?", searchFilter)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}
		for _, serie := range series {
			options = append(options, createSelect2Option(serie.Num, serie.Str))
		}
	case "serie_episodes":
		query := fmt.Sprintf("SELECT COALESCE(dbseries.seriename, 'Unknown Series') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.identifier IS NOT NULL AND dbserie_episodes.identifier != 'S00E00' THEN dbserie_episodes.identifier ELSE 'ID:' || serie_episodes.id END, 'Unknown') || ' - ' || COALESCE(CASE WHEN dbserie_episodes.title IS NOT NULL AND TRIM(dbserie_episodes.title) != '' THEN dbserie_episodes.title ELSE 'Episode ' || COALESCE(dbserie_episodes.episode, 'Unknown') END, 'Unknown Episode') || ' (' || series.listname || ')', serie_episodes.id FROM serie_episodes LEFT JOIN dbserie_episodes ON serie_episodes.dbserie_episode_id = dbserie_episodes.id LEFT JOIN series ON serie_episodes.serie_id = series.id LEFT JOIN dbseries ON series.dbserie_id = dbseries.id%s ORDER BY dbseries.seriename, series.listname, dbserie_episodes.season, dbserie_episodes.episode LIMIT ? OFFSET ?", searchFilter)
		episodes := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
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

		query := fmt.Sprintf("SELECT name, id FROM qualities%s ORDER BY name LIMIT ? OFFSET ?", searchFilter)
		qualities := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
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
			query = fmt.Sprintf("SELECT DISTINCT listname, listname FROM movies%s ORDER BY listname LIMIT ? OFFSET ?", searchFilter)
		case "series":
			query = fmt.Sprintf("SELECT DISTINCT listname, listname FROM series%s ORDER BY listname LIMIT ? OFFSET ?", searchFilter)
		default:
			query = fmt.Sprintf("SELECT DISTINCT listname, listname FROM %s%s ORDER BY listname LIMIT ? OFFSET ?", tableName, searchFilter)
		}
		listnames := database.GetrowsN[database.DbstaticTwoString](false, uint(pageSize+1), query, searchArgs...)
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
			query = fmt.Sprintf("SELECT DISTINCT quality_profile, quality_profile FROM movies%s ORDER BY quality_profile LIMIT ? OFFSET ?", searchFilter)
		case "series":
			query = fmt.Sprintf("SELECT DISTINCT quality_profile, quality_profile FROM serie_episodes%s ORDER BY quality_profile LIMIT ? OFFSET ?", searchFilter)
		case "movie_histories":
			query = fmt.Sprintf("SELECT DISTINCT quality_profile, quality_profile FROM movie_histories%s ORDER BY quality_profile LIMIT ? OFFSET ?", searchFilter)
		case "serie_episode_histories":
			query = fmt.Sprintf("SELECT DISTINCT quality_profile, quality_profile FROM serie_episode_histories%s ORDER BY quality_profile LIMIT ? OFFSET ?", searchFilter)
		default:
			query = fmt.Sprintf("SELECT DISTINCT quality_profile, quality_profile FROM %s%s ORDER BY quality_profile LIMIT ? OFFSET ?", tableName, searchFilter)
		}
		profiles := database.GetrowsN[database.DbstaticTwoString](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(profiles) > pageSize
		if hasMore {
			profiles = profiles[:pageSize]
		}
		for _, profile := range profiles {
			if profile.Str1 != "" {
				options = append(options, createSelect2OptionString(profile.Str1, profile.Str1))
			}
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
// @Router       /api/admin/table/{name}/insert [post]
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
			if strings.HasPrefix(key, "field-") == false {
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
// @Router       /api/admin/table/{name}/update/{index} [post]
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
			if strings.HasPrefix(key, "field-") == false {
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
// @Router       /api/admin/table/{name}/delete/{index} [post]
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
// @Router       /api/admin [get]
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
	var direction string
	direction = getParam(ctx, "sSortDir_0", "asc")

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
	}
	database.Scanrowsdyn(false, "select Count(*) as frequency FROM "+countTable, &total)

	// Build the complete WHERE clause
	var whereClause string
	var queryArgs []any

	if searchValue != "" || customFilters != "" {
		var conditions []string

		// Add general search condition
		if searchValue != "" {
			var aux []any
			for i := 0; i < tabledefault.DefaultQueryParamCount; i++ {
				aux = append(aux, "%"+searchValue+"%")
			}
			if tabledefault.DefaultQuery != "" {
				conditions = append(conditions, strings.TrimPrefix(tabledefault.DefaultQuery, "WHERE "))
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

		data := database.GetrowsType(tabledefault.Object, false, 1000, "select "+tabledefault.DefaultColumns+" from "+tabledefault.Table+" "+whereClause+" "+orderby+" LIMIT ?, ?", append(queryArgs, start, size)...)
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
		database.Scanrowsdyn(false, "select Count(*) as frequency FROM "+tabledefault.Table+" "+whereClause, &final, queryArgs...)
		sendDataTablesResponse(ctx, total, final, retdata)
		return
	} else {
		data := database.GetrowsType(tabledefault.Object, false, 1000, "select "+tabledefault.DefaultColumns+" from "+tabledefault.Table+" "+orderby+" LIMIT ?, ?", start, size)
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
		movie, err := database.Structscan[database.Dbmovie]("SELECT ID, Title, Year, Imdb_id, Original_title, overview, runtime, genres, original_language, status, vote_average, vote_count, popularity, budget, revenue, created_at, updated_at FROM dbmovies WHERE ID = ?", false, id)
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
		dbmovietitle, err := database.Structscan[database.DbmovieTitle]("SELECT id, title, slug, region, created_at, updated_at, dbmovie_id FROM dbmovie_titles WHERE ID = ?", false, id)
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
		serie, err := database.Structscan[database.Dbserie]("SELECT id, seriename, imdb_id, thetvdb_id, status, firstaired, network, runtime, language, genre, overview, rating, created_at, updated_at FROM dbseries WHERE ID = ?", false, id)
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
		episode, err := database.Structscan[database.DbserieEpisode]("SELECT id, title, season, episode, identifier, first_aired, overview, runtime, dbserie_id, created_at, updated_at FROM dbserie_episodes WHERE ID = ?", false, id)
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
		alt, err := database.Structscan[database.DbserieAlternate]("SELECT id, title, slug, region, dbserie_id, created_at, updated_at FROM dbserie_alternates WHERE ID = ?", false, id)
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
		movie, err := database.Structscan[database.Movie]("SELECT id, listname, rootpath, dbmovie_id, quality_profile, quality_reached, missing, blacklisted, dont_upgrade, dont_search, created_at, updated_at FROM movies WHERE ID = ?", false, id)
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
		movieFileUnmatched, err := database.Structscan[database.MovieFileUnmatched]("SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM movie_file_unmatcheds WHERE ID = ?", false, id)
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
		movieHistory, err := database.Structscan[database.MovieHistory]("SELECT id, title, url, indexer, target, quality_profile, created_at, updated_at, downloaded_at, resolution_id, quality_id, codec_id, audio_id, movie_id, dbmovie_id, blacklisted FROM movie_histories WHERE ID = ?", false, id)
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
		movieFile, err := database.Structscan[database.MovieFile]("SELECT id, location, extension, quality_profile, created_at, updated_at, resolution_id, quality_id, codec_id, audio_id, movie_id, dbmovie_id, height, width, proper, extended, repack FROM movie_files WHERE ID = ?", false, id)
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
		serie, err := database.Structscan[database.Serie]("SELECT id, listname, rootpath, dbserie_id, dont_upgrade, dont_search, created_at, updated_at FROM series WHERE ID = ?", false, id)
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
		episode, err := database.Structscan[database.SerieEpisode]("SELECT id, quality_profile, lastscan, created_at, updated_at, dbserie_episode_id, serie_id, dbserie_id, blacklisted, quality_reached, missing, dont_upgrade, dont_search, ignore_runtime FROM serie_episodes WHERE ID = ?", false, id)
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
		serieFile, err := database.Structscan[database.SerieEpisodeFile]("SELECT id, location, filename, extension, quality_profile, created_at, updated_at, resolution_id, quality_id, codec_id, audio_id, serie_id, serie_episode_id, dbserie_id, dbserie_episode_id, proper, extended, repack FROM serie_episode_files WHERE ID = ?", false, id)
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
		serieFileUnmatched, err := database.Structscan[database.SerieFileUnmatched]("SELECT id, listname, filepath, parsed_data, last_checked, created_at, updated_at FROM serie_file_unmatcheds WHERE ID = ?", false, id)
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
		serieEpisodeHistory, err := database.Structscan[database.SerieEpisodeHistory]("SELECT id, title, url, indexer, target, quality_profile, created_at, updated_at, downloaded_at, resolution_id, quality_id, codec_id, audio_id, serie_id, serie_episode_id, dbserie_id, dbserie_episode_id FROM serie_episode_histories WHERE ID = ?", false, id)
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
		job, err := database.Structscan[database.JobHistory]("SELECT id, job_type, job_category, job_group, started, ended, created_at, updated_at FROM job_histories WHERE ID = ?", false, id)
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
		quality, err := database.Structscan[database.Qualities]("SELECT id, name, regex, strings, created_at, updated_at, type, priority, regexgroup, use_regex FROM qualities WHERE ID = ?", false, id)
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
	}
	var buf strings.Builder
	renderTableEditForm(tableName, rowMap, id, getCSRFToken(ctx)).Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	gin "github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// TableInfo holds information about database tables
type TableInfo struct {
	Name      string           `json:"name"`
	Columns   []ColumnInfo     `json:"columns"`
	Rows      []map[string]any `json:"rows"`
	RowsTyped any              `json:"rowsTyped"`
	DeleteURL string           `json:"deleteURL"`
}

// ColumnInfo holds information about table columns
type ColumnInfo struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DisplayName string `json:"displayName"`
}

// ConfigSection represents a configuration section for display
type ConfigSection struct {
	Name string         `json:"name"`
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

// HTML generation helper functions
func formInput(inputType, name, id, class, value string, attrs ...gomponents.Node) gomponents.Node {
	allAttrs := []gomponents.Node{
		html.Type(inputType),
		html.Name(name),
		html.ID(id),
		html.Class(class),
	}
	if value != "" {
		allAttrs = append(allAttrs, html.Value(value))
	}
	allAttrs = append(allAttrs, attrs...)
	return html.Input(allAttrs...)
}

func formTextInput(name, id, value string, attrs ...gomponents.Node) gomponents.Node {
	return formInput("text", name, id, "form-control", value, attrs...)
}

func formPasswordInput(name, id, value string, attrs ...gomponents.Node) gomponents.Node {
	return formInput("password", name, id, "form-control", value, attrs...)
}

func formGroupWithLabel(id, labelText string, input gomponents.Node) gomponents.Node {
	return html.Div(html.Class("mb-3"),
		html.Label(html.For(id), html.Class("form-label"), gomponents.Text(labelText)),
		input,
	)
}

func createOption(value, text string, selected bool) gomponents.Node {
	attrs := []gomponents.Node{
		html.Value(value),
		gomponents.Text(text),
	}
	if selected {
		attrs = append(attrs, html.Selected())
	}
	return html.Option(attrs...)
}

func sendDataTablesResponse(ctx *gin.Context, total, final int, data any) {
	ctx.JSON(http.StatusOK, gin.H{
		"sEcho":                getParamValue(ctx, "sEcho"),
		"iTotalRecords":        total,
		"iTotalDisplayRecords": final,
		"aaData":               data,
	})
}

func sendSuccessResponse(ctx *gin.Context, message string, data any) {
	response := gin.H{
		"success": true,
		"message": message,
	}
	if data != nil {
		response["data"] = data
	}
	ctx.JSON(http.StatusOK, response)
}

func sendErrorResponse(ctx *gin.Context, statusCode int, message string) {
	ctx.JSON(statusCode, gin.H{
		"success": false,
		"error":   message,
	})
}

func sendOperationResult(ctx *gin.Context, err error) {
	response := gin.H{
		"success": err == nil,
	}
	if err != nil {
		response["error"] = err.Error()
	} else {
		response["error"] = ""
	}
	ctx.JSON(http.StatusOK, response)
}

func sendSelect2Response(ctx *gin.Context, results []map[string]any, hasMore bool) {
	ctx.JSON(http.StatusOK, gin.H{
		"results": results,
		"pagination": gin.H{
			"more": hasMore,
		},
	})
}

func createSelect2Option(id any, text string) map[string]any {
	return map[string]any{
		"id":   id,
		"text": text,
	}
}

func createSelect2OptionPtr(id any, text string) *map[string]any {
	option := map[string]any{
		"id":   id,
		"text": text,
	}
	return &option
}

func createSelect2OptionString(value string, text string) map[string]any {
	return map[string]any{
		"id":   value,
		"text": text,
	}
}

func formCheckboxInput(name, id string, checked bool, attrs ...gomponents.Node) gomponents.Node {
	allAttrs := []gomponents.Node{
		html.Type("checkbox"),
		html.Name(name),
		html.ID(id),
		html.Class("form-check-input form-control"),
	}
	if checked {
		allAttrs = append(allAttrs, html.Checked())
	}
	allAttrs = append(allAttrs, attrs...)
	return html.Input(allAttrs...)
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

// DataTables response structure
type DataTablesResponse struct {
	Echo                int        `json:"draw"`
	TotalRecords        int        `json:"recordsTotal"`
	TotalDisplayRecords int        `json:"recordsFiltered"`
	Data                [][]string `json:"data"`
}

// DataTables request parameters
type DataTablesRequest struct {
	Echo           int
	DisplayStart   int
	DisplayLength  int
	Search         string
	SortingCols    int
	SortColumns    []int
	SortDirections []string
	Searchable     []bool
	ColumnSearches []string
}

// getParam retrieves a parameter from either GET query or POST form data
func getParam(ctx *gin.Context, key, defaultValue string) string {
	if ctx.Request.Method == "POST" {
		return ctx.DefaultPostForm(key, defaultValue)
	}
	return ctx.DefaultQuery(key, defaultValue)
}

// getParamValue retrieves a parameter from either GET query or POST form data (no default)
func getParamValue(ctx *gin.Context, key string) string {
	if ctx.Request.Method == "POST" {
		return ctx.PostForm(key)
	}
	return ctx.Query(key)
}

func apiAdminTableDataJson(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, "table")
	if !ok {
		return
	}

	logger.LogDynamicany1String("info", "table", "method", ctx.Request.Method)

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

// buildCustomFilters creates WHERE clause conditions based on custom filter parameters
func buildCustomFilters(tableName string, ctx *gin.Context) (string, []any) {
	var conditions []string
	var args []any

	switch tableName {
	case "dbmovies":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if year := getParamValue(ctx, "filter-year"); year != "" {
			conditions = append(conditions, "year = ?")
			args = append(args, year)
		}
		if imdbID := getParamValue(ctx, "filter-imdb_id"); imdbID != "" {
			conditions = append(conditions, "imdb_id LIKE ?")
			args = append(args, "%"+imdbID+"%")
		}

	case "dbmovie_titles":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dt.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if movieTitle := getParamValue(ctx, "filter-movie_title"); movieTitle != "" {
			conditions = append(conditions, "dm.title LIKE ?")
			args = append(args, "%"+movieTitle+"%")
		}
		if region := getParamValue(ctx, "filter-region"); region != "" {
			conditions = append(conditions, "dt.region LIKE ?")
			args = append(args, "%"+region+"%")
		}

	case "dbseries":
		if seriename := getParamValue(ctx, "filter-seriename"); seriename != "" {
			conditions = append(conditions, "seriename LIKE ?")
			args = append(args, "%"+seriename+"%")
		}
		if tvdbID := getParamValue(ctx, "filter-thetvdb_id"); tvdbID != "" {
			conditions = append(conditions, "thetvdb_id = ?")
			args = append(args, tvdbID)
		}

	case "movies":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dbmovies.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if year := getParamValue(ctx, "filter-year"); year != "" {
			conditions = append(conditions, "dbmovies.year = ?")
			args = append(args, year)
		}
		if imdbID := getParamValue(ctx, "filter-imdb_id"); imdbID != "" {
			conditions = append(conditions, "dbmovies.imdb_id LIKE ?")
			args = append(args, "%"+imdbID+"%")
		}
		if listname := getParamValue(ctx, "filter-listname"); listname != "" {
			conditions = append(conditions, "movies.listname = ?")
			args = append(args, listname)
		}
		if qualityReached := getParamValue(ctx, "filter-quality_reached"); qualityReached != "" {
			conditions = append(conditions, "movies.quality_reached = ?")
			args = append(args, qualityReached)
		}
		if missing := getParamValue(ctx, "filter-missing"); missing != "" {
			conditions = append(conditions, "movies.missing = ?")
			args = append(args, missing)
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "movies.quality_profile = ?")
			args = append(args, quality)
		}
		if rootpath := getParamValue(ctx, "filter-rootpath"); rootpath != "" {
			conditions = append(conditions, "movies.rootpath LIKE ?")
			args = append(args, "%"+rootpath+"%")
		}

	case "series":
		if seriename := getParamValue(ctx, "filter-seriename"); seriename != "" {
			conditions = append(conditions, "dbseries.seriename LIKE ?")
			args = append(args, "%"+seriename+"%")
		}
		if listname := getParamValue(ctx, "filter-listname"); listname != "" {
			conditions = append(conditions, "series.listname = ?")
			args = append(args, listname)
		}
		if rootpath := getParamValue(ctx, "filter-rootpath"); rootpath != "" {
			conditions = append(conditions, "series.rootpath LIKE ?")
			args = append(args, "%"+rootpath+"%")
		}
		if dontUpgrade := getParamValue(ctx, "filter-dont_upgrade"); dontUpgrade != "" {
			conditions = append(conditions, "series.dont_upgrade = ?")
			args = append(args, dontUpgrade)
		}
		if dontSearch := getParamValue(ctx, "filter-dont_search"); dontSearch != "" {
			conditions = append(conditions, "series.dont_search = ?")
			args = append(args, dontSearch)
		}
		if searchSpecials := getParamValue(ctx, "filter-search_specials"); searchSpecials != "" {
			conditions = append(conditions, "series.search_specials = ?")
			args = append(args, searchSpecials)
		}
		if ignoreRuntime := getParamValue(ctx, "filter-ignore_runtime"); ignoreRuntime != "" {
			conditions = append(conditions, "series.ignore_runtime = ?")
			args = append(args, ignoreRuntime)
		}

	case "movie_files", "serie_episode_files":
		if filename := getParamValue(ctx, "filter-filename"); filename != "" {
			conditions = append(conditions, "filename LIKE ?")
			args = append(args, "%"+filename+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if resolution := getParamValue(ctx, "filter-resolution"); resolution != "" {
			conditions = append(conditions, "resolution LIKE ?")
			args = append(args, "%"+resolution+"%")
		}

	case "dbserie_alternates":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dsa.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if seriesName := getParamValue(ctx, "filter-series_name"); seriesName != "" {
			conditions = append(conditions, "ds.seriename LIKE ?")
			args = append(args, "%"+seriesName+"%")
		}
		if region := getParamValue(ctx, "filter-region"); region != "" {
			conditions = append(conditions, "dsa.region LIKE ?")
			args = append(args, "%"+region+"%")
		}

	case "dbserie_episodes":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dse.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if seriesName := getParamValue(ctx, "filter-series_name"); seriesName != "" {
			conditions = append(conditions, "ds.seriename LIKE ?")
			args = append(args, "%"+seriesName+"%")
		}
		if season := getParamValue(ctx, "filter-season"); season != "" {
			conditions = append(conditions, "dse.season = ?")
			args = append(args, season)
		}
		if episode := getParamValue(ctx, "filter-episode"); episode != "" {
			conditions = append(conditions, "dse.episode = ?")
			args = append(args, episode)
		}
		if identifier := getParamValue(ctx, "filter-identifier"); identifier != "" {
			conditions = append(conditions, "dse.identifier = ?")
			args = append(args, identifier)
		}

	case "serie_episodes":
		if episodeTitle := getParamValue(ctx, "filter-episode_title"); episodeTitle != "" {
			conditions = append(conditions, "dse.title LIKE ?")
			args = append(args, "%"+episodeTitle+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "se.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if missing := getParamValue(ctx, "filter-missing"); missing != "" {
			switch missing {
			case "1":
				conditions = append(conditions, "se.missing = 1")
			case "0":
				conditions = append(conditions, "se.missing = 0")
			}
		}

	case "job_histories":
		if jobType := getParamValue(ctx, "filter-job_type"); jobType != "" {
			conditions = append(conditions, "job_type LIKE ?")
			args = append(args, "%"+jobType+"%")
		}
		if jobCategory := getParamValue(ctx, "filter-job_category"); jobCategory != "" {
			conditions = append(conditions, "job_category LIKE ?")
			args = append(args, "%"+jobCategory+"%")
		}
		if jobGroup := getParamValue(ctx, "filter-job_group"); jobGroup != "" {
			conditions = append(conditions, "job_group LIKE ?")
			args = append(args, "%"+jobGroup+"%")
		}
		if ended := getParamValue(ctx, "filter-ended"); ended != "" {
			switch ended {
			case "1":
				conditions = append(conditions, "ended IS NOT NULL")
			case "0":
				conditions = append(conditions, "ended IS NULL")
			}
		}
		if startedDate := getParamValue(ctx, "filter-started_date"); startedDate != "" {
			conditions = append(conditions, "DATE(started) = ?")
			args = append(args, startedDate)
		}

	case "qualities":
		if qualityType := getParamValue(ctx, "filter-type"); qualityType != "" {
			conditions = append(conditions, "type = ?")
			args = append(args, qualityType)
		}
		if name := getParamValue(ctx, "filter-name"); name != "" {
			conditions = append(conditions, "name LIKE ?")
			args = append(args, "%"+name+"%")
		}
		if useRegex := getParamValue(ctx, "filter-use_regex"); useRegex != "" {
			conditions = append(conditions, "use_regex = ?")
			args = append(args, useRegex)
		}
		if priority := getParamValue(ctx, "filter-priority"); priority != "" {
			conditions = append(conditions, "priority = ?")
			args = append(args, priority)
		}

	case "movie_file_unmatcheds":
		if filepath := getParamValue(ctx, "filter-filepath"); filepath != "" {
			conditions = append(conditions, "mfu.filepath LIKE ?")
			args = append(args, "%"+filepath+"%")
		}
		if listname := getParamValue(ctx, "filter-listname"); listname != "" {
			conditions = append(conditions, "mfu.listname LIKE ?")
			args = append(args, "%"+listname+"%")
		}
		if qualityProfile := getParamValue(ctx, "filter-movie_quality_profile"); qualityProfile != "" {
			conditions = append(conditions, "m.quality_profile LIKE ?")
			args = append(args, "%"+qualityProfile+"%")
		}

	case "serie_file_unmatcheds":
		if filepath := getParamValue(ctx, "filter-filepath"); filepath != "" {
			conditions = append(conditions, "sfu.filepath LIKE ?")
			args = append(args, "%"+filepath+"%")
		}
		if listname := getParamValue(ctx, "filter-listname"); listname != "" {
			conditions = append(conditions, "sfu.listname LIKE ?")
			args = append(args, "%"+listname+"%")
		}
		if rootpath := getParamValue(ctx, "filter-series_rootpath"); rootpath != "" {
			conditions = append(conditions, "s.rootpath LIKE ?")
			args = append(args, "%"+rootpath+"%")
		}

	case "movie_histories":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "mh.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if indexer := getParamValue(ctx, "filter-indexer"); indexer != "" {
			conditions = append(conditions, "mh.indexer LIKE ?")
			args = append(args, "%"+indexer+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "mh.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if downloadedDate := getParamValue(ctx, "filter-downloaded_date"); downloadedDate != "" {
			conditions = append(conditions, "DATE(mh.downloaded_at) = ?")
			args = append(args, downloadedDate)
		}

	case "serie_episode_histories":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "seh.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if episodeTitle := getParamValue(ctx, "filter-episode_title"); episodeTitle != "" {
			conditions = append(conditions, "dse.title LIKE ?")
			args = append(args, "%"+episodeTitle+"%")
		}
		if indexer := getParamValue(ctx, "filter-indexer"); indexer != "" {
			conditions = append(conditions, "seh.indexer LIKE ?")
			args = append(args, "%"+indexer+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "seh.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}

	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), args
}

// }

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

// getReferenceTable determines the reference table name from a foreign key field
func getReferenceTable(fieldName string) string {
	if !strings.HasSuffix(fieldName, "_id") {
		return ""
	}

	if strings.EqualFold(fieldName, "imdb_id") || strings.EqualFold(fieldName, "thetvdb_id") || strings.EqualFold(fieldName, "freebase_m_id") || strings.EqualFold(fieldName, "freebase_id") || strings.EqualFold(fieldName, "tvrage_id") || strings.EqualFold(fieldName, "trakt_id") || strings.EqualFold(fieldName, "moviedb_id") || strings.EqualFold(fieldName, "facebook_id") || strings.EqualFold(fieldName, "instagram_id") || strings.EqualFold(fieldName, "twitter_id") {
		return ""
	}
	// Map common field names to their reference tables
	referenceMap := map[string]string{
		"dbmovie_id":         "dbmovies",
		"dbserie_id":         "dbseries",
		"dbserie_episode_id": "dbserie_episodes",
		"movie_id":           "movies",
		"serie_id":           "series",
		"serie_episode_id":   "serie_episodes",
		"resolution_id":      "qualities",
		"quality_id":         "qualities",
		"codec_id":           "qualities",
		"audio_id":           "qualities",
	}

	if refTable, exists := referenceMap[fieldName]; exists {
		return refTable
	}

	// Default: remove _id suffix and add 's' for pluralization
	baseName := strings.TrimSuffix(fieldName, "_id")
	if strings.HasSuffix(baseName, "y") {
		return strings.TrimSuffix(baseName, "y") + "ies"
	}
	return baseName + "s"
}

func renderTableEditForm(table string, data map[string]any, id string, csrfToken string) gomponents.Node {
	formNodes := []gomponents.Node{
		html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
	}

	// Get table columns for displaynames - use form-specific columns to exclude joined columns
	tableColumns := getAdminFormColumns(table)
	columnMap := make(map[string]string)
	for _, col := range tableColumns {
		cleanName := col.Name
		if strings.Contains(col.Name, " as ") {
			cleanName = strings.Split(col.Name, " as ")[1]
		}
		columnMap[cleanName] = col.DisplayName
	}

	// Helper function to get display name from column map
	getColumnDisplayName := func(columnMap map[string]string, fieldName string) string {
		if displayName, exists := columnMap[fieldName]; exists {
			return displayName
		}
		// Fallback to formatted field name with proper capitalization
		parts := strings.Split(fieldName, "_")
		var capitalizedParts []string
		for _, part := range parts {
			if len(part) > 0 {
				capitalizedParts = append(capitalizedParts, strings.ToTitle(strings.ToLower(part)))
			}
		}
		return strings.Join(capitalizedParts, " ")
	}
	// Sort keys to ensure consistent field ordering
	var sortedKeys []string
	for col := range data {
		sortedKeys = append(sortedKeys, col)
	}
	sort.Strings(sortedKeys)

	for _, col := range sortedKeys {
		fieldData := data[col]

		// Skip readonly fields entirely - don't include them in forms
		if col == "id" || col == "created_at" || col == "updated_at" || col == "lastscan" {
			continue
		}

		// Check if this is a quality_profile, listname, or quality_type field that should be a config dropdown
		if col == "quality_profile" || col == "listname" || (col == "quality_type" && table == "qualities") {
			currentValue := ""
			if fieldData != nil {
				currentValue = fmt.Sprintf("%v", fieldData)
			}

			var options []gomponents.Node
			options = append(options, createOption("", "-- Select or type custom --", false))

			// Add config options based on field type
			switch col {
			case "quality_profile":
				qualityConfigs := config.GetSettingsQualityAll()
				for _, qc := range qualityConfigs {
					options = append(options, createOption(qc.Name, qc.Name, currentValue == qc.Name))
				}
			case "listname":
				for _, lc := range config.GetSettingsMediaListAll() {
					options = append(options, createOption(lc, lc, currentValue == lc))
				}
			case "quality_type":
				// Quality type options: 1 = Resolution, 2 = Quality, 3 = Codec, 4 = Audio
				qualityTypes := map[string]string{
					"1": "Resolution",
					"2": "Quality",
					"3": "Codec",
					"4": "Audio",
				}
				for value, label := range qualityTypes {
					options = append(options, createOption(value, label, currentValue == value))
				}
			}

			formNodes = append(formNodes, html.Div(
				html.Class("mb-3"),
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				html.Select(
					html.Class("form-control config-dropdown"),
					html.ID("field-"+col),
					html.Name(col),
					html.Data("allow-custom", "true"),
					gomponents.Group(options),
				),
			))
			continue
		}

		// Check if this is a foreign key field that should be a dropdown
		if strings.HasSuffix(col, "_id") && col != "id" {
			refTable := getReferenceTable(col)
			if refTable != "" {
				// Remove static options loading for better performance
				// Convert current value to string for comparison
				currentValue := ""
				if fieldData != nil {
					switch v := fieldData.(type) {
					case int:
						currentValue = fmt.Sprintf("%d", v)
					case int64:
						currentValue = fmt.Sprintf("%d", v)
					case uint:
						currentValue = fmt.Sprintf("%d", v)
					case uint64:
						currentValue = fmt.Sprintf("%d", v)
					case float64:
						// Handle float64 (JSON unmarshaling default)
						if v == float64(int64(v)) {
							currentValue = fmt.Sprintf("%.0f", v)
						} else {
							currentValue = fmt.Sprintf("%v", v)
						}
					case string:
						currentValue = v
					default:
						currentValue = fmt.Sprintf("%v", v)
					}
				}

				// Create AJAX-powered dropdown with current value
				var optionNodes []gomponents.Node
				optionNodes = append(optionNodes, createOption("", "-- Select --", false))

				// If there's a current value, add it as a selected option (will be replaced by AJAX)
				if currentValue != "" {
					optionNodes = append(optionNodes, createOption(currentValue, "Loading...", true))
				}

				formNodes = append(formNodes, html.Div(
					html.Class("mb-3"),
					html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
					html.Select(
						html.Class("form-control select2-ajax"),
						html.ID("field-"+col),
						html.Name(col),
						html.Data("ajax-url", "/api/admin/dropdown/"+refTable+"/"+col),
						html.Data("selected-value", currentValue),
						html.Data("placeholder", "Search..."),
						gomponents.Group(optionNodes),
					),
				))
				continue
			}
		}

		switch val := (fieldData).(type) {
		case bool:
			formNodes = append(formNodes, html.Div(
				html.Class("form-check form-switch"),
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				formCheckboxInput(col, "field-"+col, val),
			))
		case string:
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				formTextInput(col, "field-"+col, val),
			))
		case int:
			if col == "missing" || col == "blacklisted" || col == "dont_search" || col == "dont_upgrade" || col == "use_regex" || col == "proper" || col == "extended" || col == "repack" || col == "ignore_runtime" || col == "adult" || col == "search_specials" || col == "quality_reached" {
				checked, _ := strconv.ParseBool(fmt.Sprintf("%v", fieldData))
				formNodes = append(formNodes, html.Div(
					html.Class("form-check form-switch"),
					html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
					formCheckboxInput(col, "field-"+col, checked, html.Value(fmt.Sprintf("%v", fieldData))),
				))
			} else {
				formNodes = append(formNodes, html.Div(
					html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
					html.Input(html.Class("form-control"), html.ID("field-"+col), html.Type("number"), html.Name(col), html.Value(fmt.Sprintf("%v", fieldData))),
				))
			}
		case time.Time:
			valformat := val.Format("2006-01-02")
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				html.Input(html.Class("form-control datepicker"), html.ID("field-"+col), html.Type("date"), html.Name(col), html.Value(valformat)),
			))
		case sql.NullTime:
			valformat := val.Time.Format("2006-01-02")
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				html.Input(html.Class("form-control datepicker"), html.ID("field-"+col), html.Type("date"), html.Name(col), html.Value(valformat)),
			))
		default:
			formNodes = append(formNodes, html.Div(
				html.Label(html.Class("form-label"), html.For(col), gomponents.Text(getColumnDisplayName(columnMap, col))),
				formTextInput(col, "field-"+col, fmt.Sprintf("%v", fieldData)),
			))
		}
	}

	// Add save button to form
	formNodes = append(formNodes,
		html.Div(
			html.Class("mt-3"),
			html.Button(
				html.Type("submit"),
				html.Class("btn btn-primary me-2"),
				gomponents.Text("Save Changes"),
			),
			html.Button(
				html.Type("button"),
				html.Class("btn btn-secondary"),
				html.Data("bs-dismiss", "modal"),
				gomponents.Text("Cancel"),
			),
		),
	)

	// Determine form title and action based on whether we're adding or editing
	var formTitle string
	var formAction string
	if id == "new" {
		formTitle = "Add New Row"
		formAction = "/api/admin/table/" + table + "/insert"
	} else {
		formTitle = "Edit Row"
		formAction = "/api/admin/table/" + table + "/update/" + id
	}

	return html.Div(
		html.H2(gomponents.Text(formTitle)),
		html.Form(
			html.Method("post"),
			html.Action(formAction),

			html.ID("edit-form"),
			gomponents.Group(formNodes),
			addEditFormJavascript(),
		),
	)
}

func addEditFormJavascript() gomponents.Node {
	return html.Script(gomponents.Raw(`
				// Select2 initialization is now handled by the global initSelect2Global function
				
				// Initialize config dropdowns with custom input support
				function initConfigDropdowns() {
					$('.config-dropdown').each(function() {
						var $select = $(this);
						var $container = $select.parent();
						var currentValue = $select.val();
						var fieldName = $select.attr('name');
						
						// Create a container with both select and text input
						var $wrapper = $('<div class="config-dropdown-wrapper position-relative"></div>');
						var $toggleBtn = $('<button type="button" class="btn btn-outline-secondary btn-sm position-absolute" style="right: 5px; top: 50%; transform: translateY(-50%); z-index: 10;">Custom</button>');
						var $textInput = $('<input type="text" class="form-control" style="display: none;" name="' + fieldName + '" placeholder="Enter custom value...">');
						
						$container.append($wrapper);
						$wrapper.append($select).append($toggleBtn).append($textInput);
						
						// Set initial values
						if (currentValue && !$select.find('option[value="' + currentValue + '"]').length) {
							// Current value is custom, show text input
							$select.hide();
							$textInput.show().val(currentValue);
							$toggleBtn.text('Select');
							$select.attr('name', ''); // Remove name so it doesn't submit
							$textInput.attr('name', fieldName);
						}
						
						// Toggle between select and text input
						$toggleBtn.click(function() {
							if ($select.is(':visible')) {
								// Switch to custom text input
								$select.hide();
								$textInput.show().focus();
								$toggleBtn.text('Select');
								$select.attr('name', '');
								$textInput.attr('name', fieldName);
							} else {
								// Switch to select dropdown
								$textInput.hide();
								$select.show();
								$toggleBtn.text('Custom');
								$textInput.attr('name', '');
								$select.attr('name', fieldName);
							}
						});
					});
				}
				
				// Initialize config dropdowns when page loads
				$(document).ready(function() {
					initConfigDropdowns();
				});
				
				var editForm = document.getElementById('edit-form');
				if (editForm) {
					editForm.addEventListener('submit', function(e) {
					e.preventDefault();
					var formData = new FormData(this);
					
					// Convert form data to URL-encoded format
					var params = new URLSearchParams();
					for (var pair of formData.entries()) {
						params.append(pair[0], pair[1]);
					}
					
					// Get CSRF token from form
					var csrfToken = this.querySelector('input[name="csrf_token"]').value;
					
					fetch(this.action, {
						method: 'POST',
						headers: {
							'Content-Type': 'application/x-www-form-urlencoded',
							'X-CSRF-Token': csrfToken,
						},
						body: params.toString()
					})
					.then(response => {
						if (response.ok) {
							// Find which modal this form is in and close it
							var $modal = $(this).closest('.modal');
							if ($modal.length) {
								$modal.modal('hide');
							}
							oTable.ajax.reload();
							alert('Record saved successfully');
						} else {
							alert('Error saving record');
						}
					})
					.catch(error => {
						alert('Error saving record: ' + error);
					});
					});
				}
			`))
}

type Mdata struct {
	Mdata any `json:"aaData"`
}

// renderCustomFilters creates table-specific filter fields for enhanced searching
func renderCustomFilters(tableName string) gomponents.Node {
	var filterFields []gomponents.Node

	switch tableName {
	case "dbmovies":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by title...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Year")),
				html.Input(html.Class("form-control custom-filter"), html.Type("number"),
					html.ID("filter-year"), html.Placeholder("Year...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("IMDB ID")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-imdb_id"), html.Placeholder("tt1234567...")),
			),
		}
	case "dbmovie_titles":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by title...")),
			),
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Movie Name")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-movie_title"), html.Placeholder("Filter by movie name...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Region")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-region"), html.Placeholder("Region...")),
			),
		}
	case "dbseries":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-seriename"), html.Placeholder("Filter by series name...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("TVDB ID")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-thetvdb_id"), html.Placeholder("TVDB ID...")),
			),
		}
	case "movies":
		var qualoptions []gomponents.Node
		qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
		qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
		qualoptions = append(qualoptions, createOption("", "All Profiles", false))
		qualityConfigs := config.GetSettingsQualityAll()
		for _, qc := range qualityConfigs {
			qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
		}
		var listoptions []gomponents.Node
		listoptions = append(listoptions, html.Class("form-control custom-filter"))
		listoptions = append(listoptions, html.ID("filter-listname"))
		listoptions = append(listoptions, createOption("", "All Lists", false))
		for _, lc := range config.GetSettingsMediaListAll() {
			listoptions = append(listoptions, createOption(lc, lc, false))
		}
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by title...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Year")),
				html.Input(html.Class("form-control custom-filter"), html.Type("number"),
					html.ID("filter-year"), html.Placeholder("Year...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("IMDB ID")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-imdb_id"), html.Placeholder("tt1234567...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Listname")),
				html.Select(listoptions...),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality Reached")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-quality_reached"),
					createOption("", "All", false),
					createOption("1", "Yes", false),
					createOption("0", "No", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Missing")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-missing"),
					createOption("", "All", false),
					createOption("1", "Missing", false),
					createOption("0", "Available", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
				html.Select(qualoptions...),
			),
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Rootpath")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-rootpath"), html.Placeholder("Filter by rootpath...")),
			),
		}
	case "series":
		var listoptions []gomponents.Node
		listoptions = append(listoptions, html.Class("form-control custom-filter"))
		listoptions = append(listoptions, html.ID("filter-listname"))
		listoptions = append(listoptions, createOption("", "All Lists", false))
		for _, lc := range config.GetSettingsMediaListAll() {
			listoptions = append(listoptions, createOption(lc, lc, false))
		}
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-seriename"), html.Placeholder("Filter by series name...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Listname")),
				html.Select(listoptions...),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Rootpath")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-rootpath"), html.Placeholder("Filter by rootpath...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Don't Upgrade")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-dont_upgrade"),
					createOption("", "All", false),
					createOption("1", "Yes", false),
					createOption("0", "No", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Don't Search")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-dont_search"),
					createOption("", "All", false),
					createOption("1", "Yes", false),
					createOption("0", "No", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Search Specials")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-search_specials"),
					createOption("", "All", false),
					createOption("1", "Yes", false),
					createOption("0", "No", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Ignore Runtime")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-ignore_runtime"),
					createOption("", "All", false),
					createOption("1", "Yes", false),
					createOption("0", "No", false),
				),
			),
		}
	case "movie_files", "serie_episode_files":
		var qualoptions []gomponents.Node
		qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
		qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
		qualoptions = append(qualoptions, createOption("", "All Profiles", false))
		qualityConfigs := config.GetSettingsQualityAll()
		for _, qc := range qualityConfigs {
			qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
		}
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Filename")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-filename"), html.Placeholder("Filter by filename...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Resolution")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-resolution"), html.Placeholder("1080p, 720p...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
				html.Select(qualoptions...),
			),
		}
	case "dbserie_alternates":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by title...")),
			),
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-series_name"), html.Placeholder("Filter by series name...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Region")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-region"), html.Placeholder("Region...")),
			),
		}
	case "dbserie_episodes":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by episode title...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Series Name")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-series_name"), html.Placeholder("Filter by series name...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Season")),
				html.Input(html.Class("form-control custom-filter"), html.Type("number"),
					html.ID("filter-season"), html.Placeholder("Season...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Episode")),
				html.Input(html.Class("form-control custom-filter"), html.Type("number"),
					html.ID("filter-episode"), html.Placeholder("Episode...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Identifier")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-identifier"), html.Placeholder("Identifier...")),
			),
		}
	case "serie_episodes":
		var qualoptions []gomponents.Node
		qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
		qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
		qualoptions = append(qualoptions, createOption("", "All Profiles", false))
		qualityConfigs := config.GetSettingsQualityAll()
		for _, qc := range qualityConfigs {
			qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
		}
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-episode_title"), html.Placeholder("Filter by episode title...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
				html.Select(qualoptions...),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Missing")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-missing"),
					createOption("", "All", false),
					createOption("1", "Missing", false),
					createOption("0", "Available", false),
				),
			),
		}
	case "job_histories":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Job Type")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-job_type"), html.Placeholder("Filter by job type...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Job Category")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-job_category"), html.Placeholder("Filter by category...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Job Group")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-job_group"), html.Placeholder("Filter by group...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Status")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-ended"),
					createOption("", "All", false),
					createOption("1", "Completed", false),
					createOption("0", "Running", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Started Date")),
				html.Input(html.Class("form-control custom-filter"), html.Type("date"),
					html.ID("filter-started_date"), html.Placeholder("Started date...")),
			),
		}
	case "qualities":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Type")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-type"),
					createOption("", "All Types", false),
					createOption("0", "Movies", false),
					createOption("1", "Series", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Name")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-name"), html.Placeholder("Filter by name...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Use Regex")),
				html.Select(html.Class("form-control custom-filter"), html.ID("filter-use_regex"),
					createOption("", "All", false),
					createOption("1", "Yes", false),
					createOption("0", "No", false),
				),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Priority")),
				html.Input(html.Class("form-control custom-filter"), html.Type("number"),
					html.ID("filter-priority"), html.Placeholder("Priority...")),
			),
		}
	case "movie_histories":
		var qualoptions []gomponents.Node
		qualoptions = append(qualoptions, html.Class("form-control custom-filter"))
		qualoptions = append(qualoptions, html.ID("filter-quality_profile"))
		qualoptions = append(qualoptions, createOption("", "All Profiles", false))
		qualityConfigs := config.GetSettingsQualityAll()
		for _, qc := range qualityConfigs {
			qualoptions = append(qualoptions, createOption(qc.Name, qc.Name, false))
		}
		var listoptions []gomponents.Node
		listoptions = append(listoptions, html.Class("form-control custom-filter"))
		listoptions = append(listoptions, html.ID("filter-listname"))
		listoptions = append(listoptions, createOption("", "All Lists", false))
		for _, lc := range config.GetSettingsMediaListAll() {
			listoptions = append(listoptions, createOption(lc, lc, false))
		}
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by title...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Indexer")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-indexer"), html.Placeholder("Filter by indexer...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
				html.Select(qualoptions...),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Downloaded Date")),
				html.Input(html.Class("form-control custom-filter"), html.Type("date"),
					html.ID("filter-downloaded_date"), html.Placeholder("Downloaded date...")),
			),
		}
	case "movie_file_unmatcheds":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Filepath")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-filepath"), html.Placeholder("Filter by filepath...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Listname")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-listname"), html.Placeholder("Filter by listname...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality Profile")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-movie_quality_profile"), html.Placeholder("Quality...")),
			),
		}
	case "serie_file_unmatcheds":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-4"),
				html.Label(html.Class("form-label"), gomponents.Text("Filepath")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-filepath"), html.Placeholder("Filter by filepath...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Listname")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-listname"), html.Placeholder("Filter by listname...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Root Path")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-series_rootpath"), html.Placeholder("Root path...")),
			),
		}
	case "serie_episode_histories":
		filterFields = []gomponents.Node{
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-title"), html.Placeholder("Filter by title...")),
			),
			html.Div(html.Class("col-md-3"),
				html.Label(html.Class("form-label"), gomponents.Text("Episode Title")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-episode_title"), html.Placeholder("Filter by episode title...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Indexer")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-indexer"), html.Placeholder("Indexer...")),
			),
			html.Div(html.Class("col-md-2"),
				html.Label(html.Class("form-label"), gomponents.Text("Quality")),
				html.Input(html.Class("form-control custom-filter"), html.Type("text"),
					html.ID("filter-quality_profile"), html.Placeholder("Quality...")),
			),
		}
	default:
		// Return empty div for tables without custom filters
		return html.Div()
	}

	if len(filterFields) == 0 {
		return html.Div()
	}

	// Add clear filters button
	filterFields = append(filterFields,
		html.Div(html.Class("col-md-2 d-flex align-items-end"),
			html.Button(html.Class("btn btn-secondary me-2"), html.ID("apply-filters"),
				gomponents.Text("Apply Filters")),
			html.Button(html.Class("btn btn-outline-secondary"), html.ID("clear-filters"),
				gomponents.Text("Clear")),
		),
	)

	return html.Div(html.Class("card mb-3"),
		html.Div(html.Class("card-header"),
			html.H5(html.Class("card-title mb-0"), gomponents.Text("Filters")),
		),
		html.Div(html.Class("card-body"),
			html.Div(html.Class("row g-3"),
				gomponents.Group(filterFields),
			),
		),
		html.Script(gomponents.Raw(`
			$(document).ready(function() {
				// Apply filters button
				$('#apply-filters').click(function() {
					if (typeof oTable !== 'undefined') {
						oTable.ajax.reload();
					}
				});
				
				// Clear filters button
				$('#clear-filters').click(function() {
					$('.custom-filter').val('');
					if (typeof oTable !== 'undefined') {
						oTable.ajax.reload();
					}
				});
				
				// Apply filters on Enter key
				$('.custom-filter').keypress(function(e) {
					if (e.which === 13) {
						if (typeof oTable !== 'undefined') {
							oTable.ajax.reload();
						}
					}
				});
			});
		`)),
	)
}

func renderTable(tableInfo *TableInfo, csrfToken string) gomponents.Node {
	var header []gomponents.Node
	// var footer []gomponents.Node
	var o []Mdata

	for _, col := range tableInfo.Columns {
		var addnode gomponents.Node
		var addsort gomponents.Node
		switch col.Name {
		case "id":
			addnode = html.Data("priority", "1")
		case "title", "seriename", "name", "identifier", "listname", "filename", "year":
			addnode = html.Data("priority", "1")

		case "created_at", "updated_at", "release_date", "first_aired", "overview":
			addnode = html.Data("priority", "100000")
		}
		var setname string = col.Name

		if logger.ContainsI(setname, " as ") {
			setname = strings.Split(setname, " as ")[1]
		}

		// Use displayname from column info
		header = append(header, html.Th(html.Class("sorting"), html.Role("columnheader"), addnode, addsort, gomponents.Text(col.DisplayName)))
		o = append(o, Mdata{Mdata: col.Name})
		// footer = append(footer, html.Th(html.Role("columnfooter"), html.Input(html.Type("text"), html.Name("search_"+col.Name), html.Value("Search "+col.Name), html.Class("search_init"))))
	}
	// Add Actions column header
	header = append(header, html.Th(html.Role("columnheader"), html.Data("priority", "2"), html.Data("sortable", "false"), html.Data("orderable", "false"), gomponents.Text("Actions")))
	o = append(o, Mdata{Mdata: "actions"})

	return gomponents.Group(
		[]gomponents.Node{
			html.Div(html.Class("datatables-reponsive_wrapper"),
				html.Table(
					html.ID("table-data"),
					html.Class("table table-striped datatable"),
					// html.Style("width: 100%"),
					html.THead(
						html.Tr(
							header...,
						),
					),
					//html.TFoot(html.Tr(
					//	footer...,
					//)),
				),
				html.Script(gomponents.Rawf(`
					var oTable;
					oTable = $('.datatable').DataTable({						
						"bDestroy": true,
						"bFilter": true,
						"bSort": true,
						"bPaginate": true,
						responsive: true,
						"aaSorting": [[ 0, "desc" ]],
						"bProcessing": true,
        				"bServerSide": true,
        				"sAjaxSource": "/api/admin/tablejson/%s",
						"fnServerData": function (sSource, aoData, fnCallback) {
							// Add custom filter parameters
							$('.custom-filter').each(function() {
								var id = $(this).attr('id');
								var value = $(this).val();
								if (value) {
									aoData.push({ "name": id, "value": value });
								}
							});
							
							$.ajax({
								"dataType": 'json',
								"type": "POST",
								"url": sSource,
								"data": aoData,
								"headers": {
									"X-CSRF-Token": "%s"
								},
								"success": fnCallback
							});
						},
						"columnDefs": [
							{
								"targets": -1,
								"data": null,
								"orderable": false,
								"searchable": false,
								"render": function (data, type, row, meta) {
									var id = row[0]; // Assuming ID is first column
									return '<button class="btn btn-sm btn-primary edit-btn" data-id="' + id + '" data-bs-toggle="modal" data-bs-target="#editFormModal">Edit</button> ' +
										   '<button class="btn btn-sm btn-danger delete-btn" data-id="' + id + '">Delete</button>';
								}
							}
						]
						%s
					});
					
					// Handle custom filter changes - trigger table refresh
					$(document).on('change keyup input', '.custom-filter', function() {
						var delay = 500; // Delay in milliseconds for text inputs
						var element = $(this);
						
						// Clear existing timer
						clearTimeout(element.data('timer'));
						
						// Set new timer
						element.data('timer', setTimeout(function() {
							oTable.ajax.reload();
						}, element.is('select') ? 0 : delay)); // No delay for select elements
					});
					
					// Handle Edit button clicks
					$(document).on('click', '.edit-btn', function() {
						var id = $(this).data('id');
						$('#editFormModal .modal-body').html('<div class="text-center"><div class="spinner-border" role="status"></div></div>');
						$.get('/api/admin/tableedit/%s/' + id + '?apikey=%s', function(data) {
							$('#editFormModal .modal-body').html(data);
							// Initialize Select2 after form is loaded
							setTimeout(function() {
								if (window.initSelect2Global) {
									window.initSelect2Global();
								}
							}, 100);
						});
					});
					
					// Handle Delete button clicks
					$(document).on('click', '.delete-btn', function() {
						var id = $(this).data('id');
						if (confirm('Are you sure you want to delete this record?')) {
							$.ajax({
								url: '/api/admin/table/%s/delete/' + id + '?apikey=%s',
								type: 'POST',
								headers: {
									'X-CSRF-Token': $('input[name="csrf_token"]').val() || ''
								},
								success: function(data) {
									oTable.ajax.reload();
									alert('Record deleted successfully');
								},
								error: function() {
									alert('Error deleting record');
								}
							});
						}
					});
					`, tableInfo.Name, csrfToken, "", tableInfo.Name, config.GetSettingsGeneral().WebAPIKey, tableInfo.Name, config.GetSettingsGeneral().WebAPIKey)),
			),
		})
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

// Helper functions for admin functionality

func getAdminTableColumns(tableName string) []ColumnInfo {
	tableDefault := database.GetTableDefaults(tableName)
	// For PRAGMA table_info queries, we need to handle the specific result structure
	// PRAGMA table_info returns: cid, name, type, notnull, dflt_value, pk

	// Try to get column information using string queries
	nameQuery := fmt.Sprintf("SELECT name, type FROM pragma_table_info('%s')", tableName)

	columnNames := database.GetrowsN[database.DbstaticTwoString](false, 100, nameQuery)

	columnsIn := strings.Split(tableDefault.DefaultColumns, ",")

	// Get the struct type for reflection
	var structType reflect.Type
	if tableDefault.Object != nil {
		structType = reflect.TypeOf(tableDefault.Object)
	}

	var columns []ColumnInfo
	for _, name := range columnsIn {
		name = strings.TrimSpace(name)
		columnType := "TEXT"
		for _, testname := range columnNames {
			if strings.EqualFold(testname.Str1, name) {
				columnType = testname.Str2
				break
			}
		}

		// Clean the field name for display name lookup
		cleanName := name
		if strings.Contains(name, " as ") {
			cleanName = strings.Split(name, " as ")[1]
		}

		// Get displayname using reflection
		displayName := getStructFieldDisplayName(structType, cleanName)

		columns = append(columns, ColumnInfo{
			Name:        name,
			Type:        columnType,
			DisplayName: displayName,
		})
	}

	// If no columns found, provide minimal fallback
	if len(columns) == 0 {
		columns = append(columns, ColumnInfo{
			Name:        "id",
			Type:        "INTEGER",
			DisplayName: "ID",
		})
	}

	return columns
}

// getAdminFormColumns returns only the actual table columns (excluding joined columns) for forms
func getAdminFormColumns(tableName string) []ColumnInfo {
	// For forms, we only want the actual table columns, not joined columns
	nameQuery := fmt.Sprintf("SELECT name, type FROM pragma_table_info('%s')", tableName)
	columnNames := database.GetrowsN[database.DbstaticTwoString](false, 100, nameQuery)

	// Get the struct type for reflection
	tableDefault := database.GetTableDefaults(tableName)
	var structType reflect.Type
	if tableDefault.Object != nil {
		structType = reflect.TypeOf(tableDefault.Object)
	}

	var columns []ColumnInfo
	for _, colInfo := range columnNames {
		name := colInfo.Str1
		columnType := colInfo.Str2

		// Get displayname using reflection
		displayName := getStructFieldDisplayName(structType, name)

		columns = append(columns, ColumnInfo{
			Name:        name,
			Type:        columnType,
			DisplayName: displayName,
		})
	}

	return columns
}

// getStructFieldDisplayName uses reflection to get displayname tag from struct field
func getStructFieldDisplayName(structType reflect.Type, fieldName string) string {
	// First try to get descriptive name from our field mappings
	descriptiveName := getDescriptiveFieldName(fieldName)
	if descriptiveName != "" {
		return descriptiveName
	}

	if structType != nil {
		// Convert database field name to struct field name
		structFieldName := dbFieldToStructField(fieldName)

		// Find the field in the struct
		if field, found := structType.FieldByName(structFieldName); found {
			if displayName := field.Tag.Get("displayname"); displayName != "" {
				return displayName
			}
		}
	}

	// Fallback to formatted field name with proper capitalization
	parts := strings.Split(fieldName, "_")
	var capitalizedParts []string
	for _, part := range parts {
		if len(part) > 0 {
			capitalizedParts = append(capitalizedParts, strings.ToTitle(strings.ToLower(part)))
		}
	}
	return strings.Join(capitalizedParts, " ")
}

// FieldMapping holds both struct field name and display name for a database field
type FieldMapping struct {
	StructField string
	DisplayName string
}

// getFieldMapping returns both struct field name and descriptive display name for a database field
func getFieldMapping(dbField string) FieldMapping {
	switch dbField {
	case "id":
		return FieldMapping{"ID", "ID"}
	case "created_at":
		return FieldMapping{"CreatedAt", "Created At"}
	case "updated_at":
		return FieldMapping{"UpdatedAt", "Updated At"}
	case "title":
		return FieldMapping{"Title", "Title"}
	case "year":
		return FieldMapping{"Year", "Release Year"}
	case "overview":
		return FieldMapping{"Overview", "Plot Overview"}
	case "runtime":
		return FieldMapping{"Runtime", "Runtime Minutes"}
	case "genres":
		return FieldMapping{"Genres", "Movie Genres"}
	case "status":
		return FieldMapping{"Status", "Current Status"}
	case "popularity":
		return FieldMapping{"Popularity", "Popularity Score"}
	case "budget":
		return FieldMapping{"Budget", "Production Budget"}
	case "revenue":
		return FieldMapping{"Revenue", "Box Office Revenue"}
	case "slug":
		return FieldMapping{"Slug", "URL Slug"}
	case "region":
		return FieldMapping{"Region", "Release Region"}
	case "seriename":
		return FieldMapping{"Seriename", "Series Name"}
	case "firstaired":
		return FieldMapping{"Firstaired", "First Aired Date"}
	case "network":
		return FieldMapping{"Network", "Broadcasting Network"}
	case "language":
		return FieldMapping{"Language", "Primary Language"}
	case "genre":
		return FieldMapping{"Genre", "Series Genre"}
	case "rating":
		return FieldMapping{"Rating", "User Rating"}
	case "season":
		return FieldMapping{"Season", "Season Number"}
	case "episode":
		return FieldMapping{"Episode", "Episode Number"}
	case "identifier":
		return FieldMapping{"Identifier", "Episode Identifier"}
	case "listname":
		return FieldMapping{"Listname", "Media List Name"}
	case "rootpath":
		return FieldMapping{"Rootpath", "Root Directory Path"}
	case "missing":
		return FieldMapping{"Missing", "Is Missing"}
	case "blacklisted":
		return FieldMapping{"Blacklisted", "Is Blacklisted"}
	case "filepath":
		return FieldMapping{"Filepath", "File Path"}
	case "url":
		return FieldMapping{"URL", "Download URL"}
	case "indexer":
		return FieldMapping{"Indexer", "Source Indexer"}
	case "target":
		return FieldMapping{"Target", "Download Target"}
	case "location":
		return FieldMapping{"Location", "File Location"}
	case "extension":
		return FieldMapping{"Extension", "File Extension"}
	case "height":
		return FieldMapping{"Height", "Video Height"}
	case "width":
		return FieldMapping{"Width", "Video Width"}
	case "proper":
		return FieldMapping{"Proper", "Is Proper Release"}
	case "extended":
		return FieldMapping{"Extended", "Is Extended Cut"}
	case "repack":
		return FieldMapping{"Repack", "Is Repack"}
	case "lastscan":
		return FieldMapping{"Lastscan", "Last Scan Time"}
	case "filename":
		return FieldMapping{"Filename", "File Name"}
	case "started":
		return FieldMapping{"Started", "Job Start Time"}
	case "ended":
		return FieldMapping{"Ended", "Job End Time"}
	case "name":
		return FieldMapping{"Name", "Name"}
	case "regex":
		return FieldMapping{"Regex", "Regular Expression Pattern"}
	case "strings":
		return FieldMapping{"Strings", "Match Strings"}
	case "type":
		return FieldMapping{"QualityType", "Quality Type"}
	case "priority":
		return FieldMapping{"Priority", "Priority Level"}
	case "regexgroup":
		return FieldMapping{"Regexgroup", "Regex Group"}
	case "tagline":
		return FieldMapping{"Tagline", "Movie Tagline"}
	case "quality_profile":
		return FieldMapping{"QualityProfile", "Quality Profile"}
	case "quality_type":
		return FieldMapping{"QualityType", "Quality Type"}
	case "quality_reached":
		return FieldMapping{"QualityReached", "Quality Reached"}
	case "dont_upgrade":
		return FieldMapping{"DontUpgrade", "Don't Upgrade"}
	case "dont_search":
		return FieldMapping{"DontSearch", "Don't Search"}
	case "search_specials":
		return FieldMapping{"SearchSpecials", "Search Specials"}
	case "ignore_runtime":
		return FieldMapping{"IgnoreRuntime", "Ignore Runtime"}
	case "dbmovie_id":
		return FieldMapping{"DbmovieID", "Database Movie ID"}
	case "dbserie_id":
		return FieldMapping{"DbserieID", "Database Series ID"}
	case "dbserie_episode_id":
		return FieldMapping{"DbserieEpisodeID", "Database Episode ID"}
	case "resolution_id":
		return FieldMapping{"ResolutionID", "Resolution ID"}
	case "quality_id":
		return FieldMapping{"QualityID", "Quality ID"}
	case "codec_id":
		return FieldMapping{"CodecID", "Codec ID"}
	case "audio_id":
		return FieldMapping{"AudioID", "Audio ID"}
	case "movie_id":
		return FieldMapping{"MovieID", "Movie ID"}
	case "serie_id":
		return FieldMapping{"SerieID", "Series ID"}
	case "serie_episode_id":
		return FieldMapping{"SerieEpisodeID", "Series Episode ID"}
	case "parsed_data":
		return FieldMapping{"ParsedData", "Parsed Data"}
	case "last_checked":
		return FieldMapping{"LastChecked", "Last Checked"}
	case "downloaded_at":
		return FieldMapping{"DownloadedAt", "Downloaded At"}
	case "job_type":
		return FieldMapping{"JobType", "Job Type"}
	case "job_category":
		return FieldMapping{"JobCategory", "Job Category"}
	case "job_group":
		return FieldMapping{"JobGroup", "Job Group"}
	case "use_regex":
		return FieldMapping{"UseRegex", "Use Regex"}
	case "imdb_id":
		return FieldMapping{"ImdbID", "IMDB ID"}
	case "original_language":
		return FieldMapping{"OriginalLanguage", "Original Language"}
	case "original_title":
		return FieldMapping{"OriginalTitle", "Original Title"}
	case "vote_average":
		return FieldMapping{"VoteAverage", "Vote Average"}
	case "vote_count":
		return FieldMapping{"VoteCount", "Vote Count"}
	case "first_aired":
		return FieldMapping{"FirstAired", "First Aired"}
	case "thetvdb_id":
		return FieldMapping{"ThetvdbID", "TheTVDB ID"}
	case "trakt_id":
		return FieldMapping{"TraktID", "Trakt ID"}
	case "moviedb_id":
		return FieldMapping{"MoviedbID", "MovieDB ID"}
	case "freebase_m_id":
		return FieldMapping{"FreebaseMID", "Freebase MID"}
	case "freebase_id":
		return FieldMapping{"FreebaseID", "Freebase ID"}
	case "facebook_id":
		return FieldMapping{"FacebookID", "Facebook ID"}
	case "instagram_id":
		return FieldMapping{"InstagramID", "Instagram ID"}
	case "twitter_id":
		return FieldMapping{"TwitterID", "Twitter ID"}
	case "tvrage_id":
		return FieldMapping{"TvrageID", "TVRage ID"}
	case "siterating_count":
		return FieldMapping{"SiteratingCount", "Site Rating Count"}
	case "episode_title":
		return FieldMapping{"Title", "Episode Title"}
	case "movie_title":
		return FieldMapping{"Title", "Movie Title"}
	case "series_name":
		return FieldMapping{"Seriename", "Series Name"}
	case "spoken_languages":
		return FieldMapping{"SpokenLanguages", "Spoken Languages"}
	case "release_date":
		return FieldMapping{"ReleaseDate", "Release Date"}
	case "last_id":
		return FieldMapping{"LastID", "Last ID"}
	case "last_fail":
		return FieldMapping{"LastFail", "Last Fail"}
	default:
		// Convert snake_case to PascalCase with proper capitalization
		parts := strings.Split(dbField, "_")
		structField := ""
		displayParts := make([]string, 0, len(parts))
		for _, part := range parts {
			if len(part) > 0 {
				structField += strings.ToTitle(strings.ToLower(part))
				displayParts = append(displayParts, strings.ToTitle(strings.ToLower(part)))
			}
		}
		displayName := strings.Join(displayParts, " ")
		return FieldMapping{structField, displayName}
	}
}

// dbFieldToStructField converts database field names to Go struct field names (backward compatibility)
func dbFieldToStructField(dbField string) string {
	return getFieldMapping(dbField).StructField
}

// getDescriptiveFieldName returns descriptive field names (backward compatibility)
func getDescriptiveFieldName(fieldName string) string {
	mapping := getFieldMapping(fieldName)
	if mapping.DisplayName == "" {
		return ""
	}
	return mapping.DisplayName
}

func insertAdminRecord(tableName string, data map[string]any) error {
	if tableName == "" || len(data) == 0 {
		return fmt.Errorf("table name and data are required")
	}

	var columns []string
	var values []any

	for col, val := range data {
		// Skip created_at and updated_at columns as they should be managed by the database
		if col == "id" || col == "created_at" || col == "updated_at" || col == "csrf_token" {
			continue
		}
		if val != "" && val != nil { // Skip empty values
			columns = append(columns, col)
			values = append(values, val)
		}
	}

	if len(columns) == 0 {
		return fmt.Errorf("no data to insert")
	}

	// Use project's database insert method
	_, err := database.InsertArray(tableName, columns, values...)
	return err
}

func updateAdminRecord(tableName string, id int, data map[string]any) error {
	if tableName == "" || len(data) == 0 {
		return fmt.Errorf("table name and data are required")
	}

	// Check if record exists by ID
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", tableName)
	count := database.Getdatarow[int](false, query, id)

	if count == 0 {
		return fmt.Errorf("record not found")
	}

	// Build update data
	var columns []string
	var values []any

	for col, val := range data {
		// Don't update id, created_at, updated_at, or csrf_token columns
		if col != "id" && col != "created_at" && col != "updated_at" && col != "csrf_token" {
			columns = append(columns, col)
			values = append(values, val)
		}
	}

	if len(columns) == 0 {
		return fmt.Errorf("no data to update")
	}

	// Add id as the where condition
	values = append(values, id)
	whereClause := "id = ?"

	// Use project's database update method
	_, err := database.UpdateArray(tableName, columns, whereClause, values...)
	return err
}

func deleteAdminRecord(tableName string, id int) error {
	if tableName == "" {
		return fmt.Errorf("table name is required")
	}

	// Check if record exists by ID
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", tableName)
	count := database.Getdatarow[int](false, query, id)

	if count == 0 {
		return fmt.Errorf("record not found")
	}

	// Use project's database delete method
	_, err := database.DeleteRow(tableName, "id = ?", id)
	return err
}

func adminPage() string {
	pageNode := page("Go Media Downloader", false, false, false)

	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)
	return buf.String()
}

// adminPage generates the HTML page using gomponents
// adminPageConfig - consolidated handler for all config pages
func adminPageConfig(ctx *gin.Context) {
	configType, ok := getParamID(ctx, "configtype")
	if !ok {
		return
	}

	var pageNode gomponents.Node
	csrfToken := getCSRFToken(ctx)

	switch configType {
	case "general":
		configv := config.GetSettingsGeneral()
		pageNode = page("Config General", true, false, false, renderGeneralConfig(configv, csrfToken))
	case "imdb":
		configv := config.GetSettingsImdb()
		pageNode = page("Config Imdb", true, false, false, renderImdbConfig(configv, csrfToken))
	case "media":
		configv := config.GetSettingsMediaAll()
		pageNode = page("Config Media", true, false, false, renderMediaConfig(configv, csrfToken))
	case "downloader":
		configv := config.GetSettingsDownloaderAll()
		pageNode = page("Config Downloader", true, false, false, renderDownloaderConfig(configv, csrfToken))
	case "indexers":
		configv := config.GetSettingsIndexerAll()
		pageNode = page("Config Indexer", true, false, false, renderIndexersConfig(configv, csrfToken))
	case "lists":
		configv := config.GetSettingsListAll()
		pageNode = page("Config Lists", true, false, false, renderListsConfig(configv, csrfToken))
	case "paths":
		configv := config.GetSettingsPathAll()
		pageNode = page("Config Paths", true, false, false, renderPathsConfig(configv, csrfToken))
	case "notifications":
		configv := config.GetSettingsNotificationAll()
		pageNode = page("Config Notifications", true, false, false, renderNotificationConfig(configv, csrfToken))
	case "quality":
		configv := config.GetSettingsQualityAll()
		pageNode = page("Config Quality", true, false, false, renderQualityConfig(configv, csrfToken))
	case "regex":
		configv := config.GetSettingsRegexAll()
		pageNode = page("Config Regex", true, false, false, renderRegexConfig(configv, csrfToken))
	case "scheduler":
		configv := config.GetSettingsSchedulerAll()
		pageNode = page("Config Scheduler", true, false, false, renderSchedulerConfig(configv, csrfToken))
	default:
		sendNotFound(ctx, "unknown config type: "+configType)
		return
	}

	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageTestParse(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("String Parse Test", false, false, true, renderTestParsePage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageMovieMetadata(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Movie Metadata Lookup", false, false, true, renderMovieMetadataPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageTraktAuth(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Trakt Authentication", false, false, true, renderTraktAuthPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageNamingTest(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Naming Convention Test", false, false, true, renderNamingTestPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageJobManagement(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Job Management", false, false, true, renderJobManagementPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageDebugStats(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Debug Statistics", false, false, true, renderDebugStatsPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageDatabaseMaintenance(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Database Maintenance", false, false, true, renderDatabaseMaintenancePage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageSearchDownload(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Search & Download", false, false, true, renderSearchDownloadPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPagePushoverTest(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Pushover Test", false, false, true, renderPushoverTestPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageLogViewer(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Log Viewer", false, false, true, renderLogViewerPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageFeedParsing(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Feed Parser & Results", false, false, true, renderFeedParsingPage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageFolderStructure(ctx *gin.Context) {
	csrfToken := getCSRFToken(ctx)
	pageNode := page("Folder Structure Organizer", false, false, true, renderFolderStructurePage(csrfToken))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func adminPageGrid(ctx *gin.Context) {
	grid, ok := getParamID(ctx, "grid")
	if !ok {
		return
	}
	switch grid {
	case "queue":
		renderQueuePage(ctx)
	case "scheduler":
		renderSchedulerPage(ctx)
	case "stats":
		renderStatsPage(ctx)
	}
}

// adminPageDatabase - consolidated handler for all database table pages
func adminPageDatabase(ctx *gin.Context) {
	tableName, ok := getParamID(ctx, "tablename")
	if !ok {
		return
	}

	// Create page title from table name
	pageTitle := "Database " + strings.ToTitle(strings.ReplaceAll(tableName, "_", " "))

	pageNode := page(pageTitle, false, true, false, adminDatabaseContent(tableName, getCSRFToken(ctx)))
	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func AdminPageAny(ctx *gin.Context, pageTitle string, content gomponents.Node) {
	pageNode := page(pageTitle, false, false, true, content)
	// Render to string
	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func page(_ string, activeConfig bool, activeDatabase bool, activeManagement bool, addcontent ...gomponents.Node) gomponents.Node {
	return html.Doctype(
		html.HTML(
			html.Lang("en"),
			html.Head(
				html.Meta(html.Charset("utf-8")),
				html.Meta(html.Name("viewport"), html.Content("width=device-width, initial-scale=1")),
				html.Title("Media Downloader Management"),

				// Load jQuery first
				html.Script(html.Src("https://code.jquery.com/jquery-3.7.1.min.js")),
				html.Link(html.Rel("stylesheet"), html.Href("/static/css/light.css")), // https://cdn.jsdelivr.net/npm/@adminkit/core@3.4.0/dist/css/app.min.css
				// Select2 CSS and JS for searchable dropdowns
				html.Link(html.Rel("stylesheet"), html.Href("https://cdn.jsdelivr.net/npm/select2@4.1.0-rc.0/dist/css/select2.min.css")),
				html.Link(html.Rel("stylesheet"), html.Href("https://cdn.jsdelivr.net/npm/select2-bootstrap-5-theme@1.3.0/dist/select2-bootstrap-5-theme.min.css")),
				html.Script(html.Src("/static/js/app.js")),
				html.Script(html.Src("/static/js/datatables.js")),
				html.Script(html.Src("https://cdn.jsdelivr.net/npm/select2@4.1.0-rc.0/dist/js/select2.min.js")),

				html.Script(html.Src("https://unpkg.com/htmx.org")),
				html.Style(`
					.config-section { margin-bottom: 2rem; }
					.array-item { 
						border: 1px solid #dee2e6; 
						border-radius: 0.375rem; 
						padding: 1rem; 
						margin-bottom: 1rem; 
						background-color: #f8f9fa;
					}
					.array-item-header {
						display: flex;
						justify-content: between;
						align-items: center;
						margin-bottom: 1rem;
					}
					.btn-sm { font-size: 0.875rem; }
					.nested-array {
						border-left: 3px solid #0d6efd;
						padding-left: 1rem;
						margin-left: 1rem;
					}
				`),
				// adminStyles(),
			),
			html.Body(
				html.Div(html.Class("wrapper"),
					createNavbar(activeConfig, activeDatabase, activeManagement),
					html.Div(html.Class("main"),
						html.Nav(html.Class("navbar navbar-expand navbar-light navbar-bg"),
							html.A(html.Class("sidebar-toggle js-sidebar-toggle"),
								html.I(html.Class("hamburger align-self-center")),
							),
						),
						html.Main(html.Class("content"),
							html.Div(html.Class("container-fluid p-0"),
								// html.H1(html.Class("h3 mb-3"), gomponents.Text(headertext)),
								html.Div(
									append([]gomponents.Node{html.Class("row")}, addcontent...)...),
							),
						),
					),
				),

				adminJavaScript(),
			),
		),
	)
}

func createNavbar(activeConfig bool, activeDatabase bool, activeManagement bool) gomponents.Node {
	collapsed := "sidebar-dropdown list-unstyled collapse"
	uncollapsed := "sidebar-dropdown list-unstyled"

	cssRootConfig := collapsed
	cssRootDatabase := collapsed
	cssRootManagement := collapsed

	if activeConfig {
		cssRootConfig = uncollapsed
	}
	if activeDatabase {
		cssRootDatabase = uncollapsed
	}
	if activeManagement {
		cssRootManagement = uncollapsed
	}
	return html.Nav(
		html.ID("sidebar"),
		html.Class("sidebar js-sidebar"),
		html.Div(
			html.Class("sidebar-content js-simplebar"),
			html.A(
				html.Class("sidebar-brand"),
				html.Href("index.html"),
				html.Span(
					html.Class("sidebar-brand-text align-middle"),
					gomponents.Text("Go Media Downloader"),
				),
			),
			html.Ul(html.Class("sidebar-nav "),
				html.Li(html.Class("sidebar-header"), gomponents.Text("Pages")),
				html.Li(html.Class("sidebar-item active"),
					html.A(html.Data("bs-target", "#Configuration"), html.Data("bs-toggle", "collapse"), html.Class("sidebar-link collapsed"),
						html.Span(html.Class("align-middle"), gomponents.Text("Configuration")),
					),
					html.Ul(html.Class(cssRootConfig), html.ID("Configuration"),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/general"),
								html.Span(html.Class("align-middle"), gomponents.Text("General")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/imdb"),
								html.Span(html.Class("align-middle"), gomponents.Text("Imdb")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/media"),
								html.Span(html.Class("align-middle"), gomponents.Text("Media")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/downloader"),
								html.Span(html.Class("align-middle"), gomponents.Text("Downloader")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/lists"),
								html.Span(html.Class("align-middle"), gomponents.Text("Lists")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/indexers"),
								html.Span(html.Class("align-middle"), gomponents.Text("Indexers")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/paths"),
								html.Span(html.Class("align-middle"), gomponents.Text("Paths")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/notifications"),
								html.Span(html.Class("align-middle"), gomponents.Text("Notifications")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/regex"),
								html.Span(html.Class("align-middle"), gomponents.Text("Regex")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/quality"),
								html.Span(html.Class("align-middle"), gomponents.Text("Quality")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/scheduler"),
								html.Span(html.Class("align-middle"), gomponents.Text("Scheduler")),
							),
						),
					),
				),
				html.Li(html.Class("sidebar-item inactive"),
					html.A(html.Data("bs-target", "#Database"), html.Data("bs-toggle", "collapse"), html.Class("sidebar-link collapsed"),
						html.Span(html.Class("align-middle"), gomponents.Text("Database")),
					),
					html.Ul(html.Class(cssRootDatabase), html.ID("Database"),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/dbmovies"),
								html.Span(html.Class("align-middle"), gomponents.Text("DBMovies")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/dbmovie_titles"),
								html.Span(html.Class("align-middle"), gomponents.Text("DBMovie Titles")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/dbseries"),
								html.Span(html.Class("align-middle"), gomponents.Text("DBSeries")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/dbserie_episodes"),
								html.Span(html.Class("align-middle"), gomponents.Text("DBSerie Episodes")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/dbserie_alternates"),
								html.Span(html.Class("align-middle"), gomponents.Text("DBSerie Alternates")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/movies"),
								html.Span(html.Class("align-middle"), gomponents.Text("Movies")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/movie_files"),
								html.Span(html.Class("align-middle"), gomponents.Text("Movie Files")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/movie_histories"),
								html.Span(html.Class("align-middle"), gomponents.Text("Movie Histories")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/movie_file_unmatcheds"),
								html.Span(html.Class("align-middle"), gomponents.Text("Movie Unmatcheds")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/series"),
								html.Span(html.Class("align-middle"), gomponents.Text("Series")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/serie_episodes"),
								html.Span(html.Class("align-middle"), gomponents.Text("Serie Episodes")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/serie_episode_files"),
								html.Span(html.Class("align-middle"), gomponents.Text("Serie Episode Files")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/serie_episode_histories"),
								html.Span(html.Class("align-middle"), gomponents.Text("Serie Episode Histories")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/serie_file_unmatcheds"),
								html.Span(html.Class("align-middle"), gomponents.Text("Serie Episode Unmatcheds")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/qualities"),
								html.Span(html.Class("align-middle"), gomponents.Text("Qualities")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/database/job_histories"),
								html.Span(html.Class("align-middle"), gomponents.Text("Job Histories")),
							),
						),
					),
				),
				html.Li(html.Class("sidebar-item inactive"),
					html.A(html.Data("bs-target", "#Management"), html.Data("bs-toggle", "collapse"), html.Class("sidebar-link collapsed"),
						html.Span(html.Class("align-middle"), gomponents.Text("Management")),
					),
					html.Ul(html.Class(cssRootManagement), html.ID("Management"),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/grid/queue"),
								html.Span(html.Class("align-middle"), gomponents.Text("Queue")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/grid/scheduler"),
								html.Span(html.Class("align-middle"), gomponents.Text("Scheduler")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/grid/stats"),
								html.Span(html.Class("align-middle"), gomponents.Text("Stats")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/jobmanagement"),
								html.Span(html.Class("align-middle"), gomponents.Text("Job Management")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/debugstats"),
								html.Span(html.Class("align-middle"), gomponents.Text("Debug Statistics")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/dbmaintenance"),
								html.Span(html.Class("align-middle"), gomponents.Text("Database Maintenance")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/searchdownload"),
								html.Span(html.Class("align-middle"), gomponents.Text("Search & Download")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/pushovertest"),
								html.Span(html.Class("align-middle"), gomponents.Text("Pushover Test")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/logviewer"),
								html.Span(html.Class("align-middle"), gomponents.Text("Log Viewer")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/feedparse"),
								html.Span(html.Class("align-middle"), gomponents.Text("Feed Parser")),
							),
						),
						// html.Li(html.Class("sidebar-item"),
						// 	html.A(html.Class("sidebar-link"), html.Href("/api/admin/folderstructure"),
						// 		html.Span(html.Class("align-middle"), gomponents.Text("Folder Organizer")),
						// 	),
						// ),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/testparse"),
								html.Span(html.Class("align-middle"), gomponents.Text("Test Parsing")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/moviemetadata"),
								html.Span(html.Class("align-middle"), gomponents.Text("Test Metadata")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/traktauth"),
								html.Span(html.Class("align-middle"), gomponents.Text("Trakt Authenticate")),
							),
						),
						html.Li(html.Class("sidebar-item"),
							html.A(html.Class("sidebar-link"), html.Href("/api/admin/namingtest"),
								html.Span(html.Class("align-middle"), gomponents.Text("Test Naming")),
							),
						),
					),
				),
			),
			html.Div(html.Class("simplebar-track simplebar-horizontal"), html.Style("visibility: hidden;"),
				html.Div(html.Class("simplebar-scrollbar"), html.Style("width: 0px; display: none;")),
			),
			html.Div(html.Class("simplebar-track simplebar-vertical"), html.Style("visibility: visible;"),
				html.Div(html.Class("simplebar-scrollbar"), html.Style("height: 933px; transform: translate3d(0px, 0px, 0px); display: block;")),
			),
		),
	)
}

// adminDatabaseTab component
func adminDatabaseContent(tableName string, csrfToken string) gomponents.Node {
	tableColumns := getAdminTableColumns(tableName)
	tableDefault := database.GetTableDefaults(tableName)

	tableInfo := TableInfo{
		Name:      tableName,
		Columns:   tableColumns,
		Rows:      database.GetrowsType(tableDefault.Object, false, 10, fmt.Sprintf("SELECT %s FROM %s LIMIT 10", tableDefault.DefaultColumns, tableDefault.Table)),
		DeleteURL: fmt.Sprintf("/api/admin/table/%s/delete", tableName),
	}

	return html.Div(
		html.Input(html.Name("table-name"), html.Type("hidden"), html.ID("table-name")),

		adminModal(),
		adminAddModal(tableName, csrfToken),
		html.Div(
			html.Class("config-section"),

			html.Div(html.Class("success-msg"), html.ID("db-success")),
			html.Div(html.Class("error-msg"), html.ID("db-error")),

			// Add custom filters for specific tables
			renderCustomFilters(tableName),

			// Add Record button
			html.Div(
				html.Class("mb-3"),
				html.Button(
					html.Class("btn btn-primary"),
					html.Type("button"),
					html.Data("bs-toggle", "modal"),
					html.Data("bs-target", "#addRecordModal"),
					gomponents.Text("Add Record"),
				),
			),

			html.Div(
				html.ID("table-content"),
				renderTable(&tableInfo, csrfToken),
			),
		),
	)
}

// adminModal component
func adminModal() gomponents.Node {
	return html.Div(
		html.Class("modal fade"),
		html.ID("editFormModal"),

		html.Div(
			html.Class("modal-dialog"),

			html.Div(
				html.Class("modal-content"),

				html.Div(
					html.Class("modal-header"),
					html.H5(html.ID("modal-title"), gomponents.Text("Edit Record")),
					html.Button(
						html.Class("btn-close"),
						gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"),
					),
				),
				html.Div(
					html.Class("modal-body"),
					// The form content will be loaded here by the DataTables edit handler
				),
			),
		),
	)
}

// adminAddModal component
func adminAddModal(tableName string, csrfToken string) gomponents.Node {
	// Get table columns to create empty data map - use form-specific columns to exclude joined columns
	emptyData := make(map[string]any)
	tableColumns := getAdminFormColumns(tableName)

	// Initialize empty data for all columns except auto-generated ones
	for _, col := range tableColumns {
		columnName := col.Name
		if strings.Contains(col.Name, " as ") {
			columnName = strings.Split(col.Name, " as ")[1]
		}

		// Skip auto-generated fields
		if columnName != "id" && columnName != "created_at" && columnName != "updated_at" {
			// Initialize boolean-like fields as integers (0 = false) so they render as switches
			if columnName == "missing" || columnName == "blacklisted" || columnName == "dont_search" || columnName == "dont_upgrade" || columnName == "use_regex" || columnName == "proper" || columnName == "extended" || columnName == "repack" || columnName == "ignore_runtime" || columnName == "adult" || columnName == "search_specials" || columnName == "quality_reached" {
				emptyData[columnName] = 0
			} else {
				emptyData[columnName] = ""
			}
		}
	}

	return html.Div(
		html.Class("modal fade"),
		html.ID("addRecordModal"),

		html.Div(
			html.Class("modal-dialog modal-lg"),

			html.Div(
				html.Class("modal-content"),

				html.Div(
					html.Class("modal-header"),
					html.H5(html.Class("modal-title"), gomponents.Text("Add New Record")),
					html.Button(
						html.Class("btn-close"),
						gomponents.Attr("data-bs-dismiss", "modal"),
						gomponents.Attr("aria-label", "Close"),
					),
				),
				html.Div(
					html.Class("modal-body"),
					renderTableEditForm(tableName, emptyData, "new", csrfToken),
				),
			),
		),
	)
}

// adminJavaScript component
func adminJavaScript() gomponents.Node {
	jsContent := `
			// Utility function to show messages
			function showMessage(elementId, message) {
				const element = document.getElementById(elementId);
				if (element) {
					element.textContent = message;
					element.style.display = 'block';
					setTimeout(() => { 
						element.style.display = 'none'; 
					}, 3000);
				}
			}
			
			// Global function to initialize Select2 dropdowns - can be called from anywhere
			window.initSelect2Global = function() {
				// Check if jQuery is available
				if (typeof $ === 'undefined') {
					return;
				}
				
				// Check if Select2 is available
				if (typeof $.fn.select2 === 'undefined') {
					return;
				}
				
				// Check if Select2 elements exist
				if ($('.select2-ajax').length === 0) {
					return;
				}
				
				// Remove duplicate elements with same ID to prevent conflicts
				var seenIds = {};
				$('.select2-ajax').each(function() {
					var id = $(this).attr('id');
					if (seenIds[id]) {
						$(this).remove();
					} else {
						seenIds[id] = true;
					}
				});
				
				$('.select2-ajax').each(function(index) {
					try {
						var $this = $(this);
						
						// Skip if already initialized
						if ($this.hasClass('select2-hidden-accessible')) {
							return;
						}
						
						var ajaxUrl = $this.data('ajax-url');
						var selectedValue = $this.data('selected-value');
						var csrfToken = $('input[name="csrf_token"]').val() || '';
						
						// Determine which modal this element is in
						var $modal = $this.closest('.modal');
						var dropdownParent = $modal.length ? $modal : $('body');
						
						// Remove any existing options to force AJAX loading
						$this.empty().append('<option value="">-- Select --</option>');
						
						$this.select2({
							placeholder: 'Search...',
							allowClear: true,
							width: '100%',
							dropdownParent: dropdownParent,
							ajax: {
								url: ajaxUrl,
								type: 'POST',
								dataType: 'json',
								delay: 250,
								headers: {
									"X-CSRF-Token": csrfToken
								},
								data: function (params) {
									return {
										search: params.term || '',
										page: params.page || 1
									};
								},
								processResults: function (data, params) {
									params.page = params.page || 1;
									return {
										results: data.results || [],
										pagination: {
											more: false
										}
									};
								},
								cache: false
							},
							minimumInputLength: 0
						});
						
						// Load the selected option if there's a value
						if (selectedValue && selectedValue !== '') {
							$.ajax({
								url: ajaxUrl,
								type: 'POST',
								dataType: 'json',
								headers: {
									"X-CSRF-Token": csrfToken
								},
								data: {
									id: selectedValue
								}
							}).then(function (data) {
								if (data.results && data.results.length > 0) {
									var selectedItem = data.results[0];
									// Create option and set as selected
									var option = new Option(selectedItem.text, selectedItem.id, true, true);
									$this.append(option).trigger('change');
								}
							}).catch(function(error) {
								// Silently handle errors in selected option loading
							});
						}
					} catch (error) {
						// Silently handle Select2 initialization errors
					}
				});
			};
			
			// Initialize Select2 when Add Record Modal is shown
			$(document).on('shown.bs.modal', '#addRecordModal', function() {
				setTimeout(function() {
					if (window.initSelect2Global) {
						window.initSelect2Global();
					}
				}, 100);
			});
			
			// Initialize Select2 when Edit Form Modal is shown  
			$(document).on('shown.bs.modal', '#editFormModal', function() {
				setTimeout(function() {
					if (window.initSelect2Global) {
						window.initSelect2Global();
					}
				}, 100);
			});
		`
	return html.Script(html.Type("text/javascript"),
		gomponents.Raw(jsContent),
	)
}

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
			searchFilter = " WHERE dm.title LIKE ?"
			searchArgs = append(searchArgs, "%"+search+"%")
		case "series":
			searchFilter = " WHERE ds.seriename LIKE ?"
			searchArgs = append(searchArgs, "%"+search+"%")
		case "serie_episodes":
			searchFilter = " WHERE dse.title LIKE ?"
			searchArgs = append(searchArgs, "%"+search+"%")
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
		episodes := database.GetrowsN[database.DbstaticTwoStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(episodes) > pageSize
		if hasMore {
			episodes = episodes[:pageSize]
		}
		for _, episode := range episodes {
			label := fmt.Sprintf("%s - %s", episode.Str1, episode.Str2)
			options = append(options, createSelect2Option(episode.Num, label))
		}
	case "movies":
		query := fmt.Sprintf("SELECT dm.title || ' - ' || m.listname, m.id FROM movies m LEFT JOIN dbmovies dm ON m.dbmovie_id = dm.id%s ORDER BY dm.title LIMIT ? OFFSET ?", searchFilter)
		movies := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(movies) > pageSize
		if hasMore {
			movies = movies[:pageSize]
		}
		for _, movie := range movies {
			options = append(options, createSelect2Option(movie.Num, movie.Str))
		}
	case "series":
		query := fmt.Sprintf("SELECT ds.seriename || ' - ' || s.listname, s.id FROM series s LEFT JOIN dbseries ds ON s.dbserie_id = ds.id%s ORDER BY ds.seriename LIMIT ? OFFSET ?", searchFilter)
		series := database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(pageSize+1), query, searchArgs...)
		hasMore = len(series) > pageSize
		if hasMore {
			series = series[:pageSize]
		}
		for _, serie := range series {
			options = append(options, createSelect2Option(serie.Num, serie.Str))
		}
	case "serie_episodes":
		query := fmt.Sprintf("SELECT dse.identifier || ' - ' || dse.title || ' - ' || s.listname, se.id FROM serie_episodes se LEFT JOIN dbserie_episodes dse ON se.dbserie_episode_id = dse.id LEFT JOIN series s ON se.serie_id = s.id%s ORDER BY s.listname, dse.identifier LIMIT ? OFFSET ?", searchFilter)
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

// getDropdownOptionByID retrieves a single dropdown option by ID for preselection
func getDropdownOptionByID(tableName, fieldName string, id int) *map[string]any {
	switch tableName {
	case "dbmovies":
		if movie, err := database.Structscan[database.Dbmovie]("SELECT id, title FROM dbmovies WHERE id = ?", false, id); err == nil {
			return createSelect2OptionPtr(movie.ID, movie.Title)
		}
	case "dbseries":
		if serie, err := database.Structscan[database.Dbserie]("SELECT id, seriename FROM dbseries WHERE id = ?", false, id); err == nil {
			return createSelect2OptionPtr(serie.ID, serie.Seriename)
		}
	case "dbserie_episodes":
		if episode, err := database.Structscan[database.DbserieEpisode]("SELECT id, identifier, title FROM dbserie_episodes WHERE id = ?", false, id); err == nil {
			label := fmt.Sprintf("%s - %s", episode.Identifier, episode.Title)
			return createSelect2OptionPtr(episode.ID, label)
		}
	case "movies":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 1, "SELECT dm.title || ' - ' || m.listname, m.id FROM movies m LEFT JOIN dbmovies dm ON m.dbmovie_id = dm.id WHERE m.id = ?", id)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}
	case "series":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 1, "SELECT ds.seriename || ' - ' || s.listname, s.id FROM series s LEFT JOIN dbseries ds ON s.dbserie_id = ds.id WHERE s.id = ?", id)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
		}
	case "serie_episodes":
		result := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 1, "SELECT dse.identifier || ' - ' || dse.title || ' - ' || s.listname, se.id FROM serie_episodes se LEFT JOIN dbserie_episodes dse ON se.dbserie_episode_id = dse.id LEFT JOIN series s ON se.serie_id = s.id WHERE se.id = ?", id)
		if len(result) > 0 {
			return createSelect2OptionPtr(result[0].Num, result[0].Str)
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
			typeFilter = ""
		}

		query := fmt.Sprintf("SELECT id, name FROM qualities WHERE id = ?%s", typeFilter)
		if quality, err := database.Structscan[database.Qualities](query, false, id); err == nil {
			return createSelect2OptionPtr(quality.ID, quality.Name)
		}
	}
	return nil
}

// renderQueuePage renders the queue monitoring page
func renderQueuePage(ctx *gin.Context) {
	pageNode := page("Queue Monitor", false, false, true, renderQueueGrid())
	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderQueueGrid creates a grid showing active queue items
func renderQueueGrid() gomponents.Node {
	var queueData []map[string]any

	for i, value := range worker.GetQueues() {
		queueData = append(queueData, map[string]any{
			"id":      i,
			"queue":   value.Queue,
			"job":     value.Name,
			"added":   value.Added.Format("2006-01-02 15:04:05"),
			"started": value.Started.Format("2006-01-02 15:04:05"),
		})
	}
	var rows []gomponents.Node
	for _, item := range queueData {
		rows = append(rows, html.Tr(
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["id"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["queue"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["job"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["added"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["started"]))),
		))
	}
	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("5"),
				html.Class("text-center text-muted"),
				gomponents.Text("No queue items found"),
			),
		))
	}
	return html.Div(
		html.Class("container-fluid"),
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card"),
					html.Div(
						html.Class("card-header"),
						html.H5(html.Class("card-title"), gomponents.Text("Active Queue Items")),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("table-responsive"),
							html.Table(
								html.Class("table table-striped"),
								html.THead(
									html.Tr(
										html.Th(gomponents.Text("ID")),
										html.Th(gomponents.Text("Queue")),
										html.Th(gomponents.Text("Job")),
										html.Th(gomponents.Text("Added")),
										html.Th(gomponents.Text("Started")),
									),
								),
								html.TBody(
									rows...,
								),
							),
						),
					),
				),
			),
		),
		html.Script(
			gomponents.Raw(`
				// Auto-refresh every 10 seconds
				setInterval(function() {
					window.location.reload();
				}, 10000);
			`),
		),
	)
}

// renderSchedulerPage renders the scheduler monitoring page
func renderSchedulerPage(ctx *gin.Context) {
	pageNode := page("Scheduler Monitor", false, false, true, renderSchedulerGrid())
	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

// renderSchedulerGrid creates a grid showing scheduler status
func renderSchedulerGrid() gomponents.Node {
	var schedulerData []map[string]any

	for i, value := range worker.GetSchedules() {
		schedulerData = append(schedulerData, map[string]any{
			"id":        i,
			"job":       value.JobName,
			"lastrun":   value.LastRun.Format("2006-01-02 15:04:05"),
			"nextrun":   value.NextRun.Format("2006-01-02 15:04:05"),
			"isrunning": value.IsRunning,
		})
	}

	var rows []gomponents.Node
	for _, item := range schedulerData {
		isRunning := item["isrunning"].(bool)
		statusBadge := html.Span(
			html.Class("badge bg-secondary"),
			gomponents.Text("false"),
		)
		if isRunning {
			statusBadge = html.Span(
				html.Class("badge bg-success"),
				gomponents.Text("true"),
			)
		}

		rows = append(rows, html.Tr(
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["id"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["job"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["lastrun"]))),
			html.Td(gomponents.Text(fmt.Sprintf("%v", item["nextrun"]))),
			html.Td(statusBadge),
		))
	}
	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("5"),
				html.Class("text-center text-muted"),
				gomponents.Text("No scheduled jobs found"),
			),
		))
	}
	return html.Div(
		html.Class("container-fluid"),
		html.Div(
			html.Class("row"),
			html.Div(
				html.Class("col-12"),
				html.Div(
					html.Class("card"),
					html.Div(
						html.Class("card-header"),
						html.H5(html.Class("card-title"), gomponents.Text("Scheduled Jobs")),
					),
					html.Div(
						html.Class("card-body"),
						html.Div(
							html.Class("table-responsive"),
							html.Table(
								html.Class("table table-striped"),
								html.THead(
									html.Tr(
										html.Th(gomponents.Text("ID")),
										html.Th(gomponents.Text("Job")),
										html.Th(gomponents.Text("Last Run")),
										html.Th(gomponents.Text("Next Run")),
										html.Th(gomponents.Text("Is Running")),
									),
								),
								html.TBody(
									rows...,
								),
							),
						),
					),
				),
			),
		),
		html.Script(
			gomponents.Raw(`
				// Auto-refresh every 60 seconds
				setInterval(function() {
					window.location.reload();
				}, 60000);
			`),
		),
	)
}

func renderStatsPage(ctx *gin.Context) {
	pageNode := page("Media Statistics", false, false, true, renderStatsGrid())
	var buf strings.Builder
	pageNode.Render(&buf)
	ctx.Header("Content-Type", "text/html; charset=utf-8")
	ctx.String(http.StatusOK, buf.String())
}

func renderStatsGrid() gomponents.Node {
	var statsData []map[string]any
	id := 0

	movieLists := database.GetrowsN[string](
		false,
		5,
		"select distinct listname from movies where listname is not null and listname != ''",
	)
	for idx := range movieLists {
		all := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE",
			&movieLists[idx],
		)
		missing := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE and missing=1",
			&movieLists[idx],
		)
		reached := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE and quality_reached=1",
			&movieLists[idx],
		)
		upgrade := database.Getdatarow[uint](
			false,
			"select count(*) from movies where listname = ? COLLATE NOCASE and quality_reached=0 and missing=0",
			&movieLists[idx],
		)

		statsData = append(statsData, map[string]any{
			"id":       id,
			"typ":      "movies",
			"list":     movieLists[idx],
			"total":    all,
			"missing":  missing,
			"finished": reached,
			"upgrade":  upgrade,
		})
		id++
	}

	seriesLists := database.GetrowsN[string](
		false,
		5,
		"select distinct listname from series where listname is not null and listname != ''",
	)
	for idx := range seriesLists {
		all := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
			&seriesLists[idx],
		)
		missing := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and missing=1",
			&seriesLists[idx],
		)
		reached := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and quality_reached=1",
			&seriesLists[idx],
		)
		upgrade := database.Getdatarow[uint](
			false,
			"select count(*) from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE) and quality_reached=0 and missing=0",
			&seriesLists[idx],
		)

		statsData = append(statsData, map[string]any{
			"id":       id,
			"typ":      "episodes",
			"list":     seriesLists[idx],
			"total":    all,
			"missing":  missing,
			"finished": reached,
			"upgrade":  upgrade,
		})
		id++
	}

	var rows []gomponents.Node
	for _, stat := range statsData {
		typ := stat["typ"].(string)
		var typeBadge gomponents.Node
		if typ == "movies" {
			typeBadge = html.Span(
				html.Class("badge bg-primary"),
				gomponents.Text("Movies"),
			)
		} else {
			typeBadge = html.Span(
				html.Class("badge bg-success"),
				gomponents.Text("Episodes"),
			)
		}

		rows = append(rows, html.Tr(
			html.Td(gomponents.Text(fmt.Sprintf("%d", stat["id"].(int)))),
			html.Td(typeBadge),
			html.Td(gomponents.Text(stat["list"].(string))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", stat["total"].(uint)))),
			html.Td(
				html.Span(
					html.Class("badge bg-warning"),
					gomponents.Text(fmt.Sprintf("%d", stat["missing"].(uint))),
				),
			),
			html.Td(
				html.Span(
					html.Class("badge bg-success"),
					gomponents.Text(fmt.Sprintf("%d", stat["finished"].(uint))),
				),
			),
			html.Td(
				html.Span(
					html.Class("badge bg-info"),
					gomponents.Text(fmt.Sprintf("%d", stat["upgrade"].(uint))),
				),
			),
		))
	}

	if len(rows) == 0 {
		rows = append(rows, html.Tr(
			html.Td(
				html.ColSpan("7"),
				html.Class("text-center text-muted"),
				gomponents.Text("No statistics available"),
			),
		))
	}

	return html.Div(
		html.Class("card"),
		html.Div(
			html.Class("card-header"),
			html.H5(
				html.Class("card-title mb-0"),
				gomponents.Text("Media Statistics"),
			),
		),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("table-responsive"),
				html.Table(
					html.Class("table table-striped table-hover"),
					html.THead(
						html.Tr(
							html.Th(gomponents.Text("ID")),
							html.Th(gomponents.Text("Type")),
							html.Th(gomponents.Text("List")),
							html.Th(gomponents.Text("Total")),
							html.Th(gomponents.Text("Missing")),
							html.Th(gomponents.Text("Finished")),
							html.Th(gomponents.Text("Upgradable")),
						),
					),
					html.TBody(rows...),
				),
			),
		),
	)
}

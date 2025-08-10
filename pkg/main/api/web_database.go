package api

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	gin "github.com/gin-gonic/gin"
)

// Database operations and metadata functions

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

func buildCustomFilters(tableName string, ctx *gin.Context) (string, []any) {
	var conditions []string
	var args []any

	// Define filter mappings based on table structure
	filterMappings := map[string]map[string]FilterMapping{
		"dbmovies": {
			"title":             {Column: "title", Operator: "LIKE"},
			"year":              {Column: "year", Operator: "="},
			"imdb_id":           {Column: "imdb_id", Operator: "LIKE"},
			"vote_average":      {Column: "vote_average", Operator: ">="},
			"runtime":           {Column: "runtime", Operator: ">="},
			"original_language": {Column: "original_language", Operator: "="},
			"adult":             {Column: "adult", Operator: "="},
			"status":            {Column: "status", Operator: "LIKE"},
		},
		"movies": {
			"quality_profile": {Column: "quality_profile", Operator: "LIKE"},
			"listname":        {Column: "listname", Operator: "LIKE"},
			"rootpath":        {Column: "rootpath", Operator: "LIKE"},
			"quality_reached": {Column: "quality_reached", Operator: "="},
			"missing":         {Column: "missing", Operator: "="},
		},
		"dbseries": {
			"seriename":  {Column: "seriename", Operator: "LIKE"},
			"status":     {Column: "status", Operator: "LIKE"},
			"genre":      {Column: "genre", Operator: "LIKE"},
			"imdb_id":    {Column: "imdb_id", Operator: "LIKE"},
			"thetvdb_id": {Column: "thetvdb_id", Operator: "="},
		},
		"qualities": {
			"type":      {Column: "type", Operator: "="},
			"name":      {Column: "name", Operator: "LIKE"},
			"regex":     {Column: "regex", Operator: "LIKE"},
			"strings":   {Column: "strings", Operator: "LIKE"},
			"priority":  {Column: "priority", Operator: "="},
			"use_regex": {Column: "use_regex", Operator: "="},
		},
		"series": {
			"listname": {Column: "listname", Operator: "LIKE"},
			"rootpath": {Column: "rootpath", Operator: "LIKE"},
		},
		"movie_files": {
			"location":        {Column: "location", Operator: "LIKE"},
			"filename":        {Column: "filename", Operator: "LIKE"},
			"extension":       {Column: "extension", Operator: "="},
			"quality_profile": {Column: "quality_profile", Operator: "LIKE"},
		},
		"serie_episode_files": {
			"location":  {Column: "location", Operator: "LIKE"},
			"filename":  {Column: "filename", Operator: "LIKE"},
			"extension": {Column: "extension", Operator: "="},
		},
		"dbmovie_titles": {
			"title":       {Column: "dbmovie_titles.title", Operator: "LIKE"},
			"movie_title": {Column: "dbmovies.title", Operator: "LIKE"},
			"region":      {Column: "dbmovie_titles.region", Operator: "LIKE"},
		},
		"job_histories": {
			"job_type":     {Column: "job_type", Operator: "LIKE"},
			"job_group":    {Column: "job_group", Operator: "LIKE"},
			"job_category": {Column: "job_category", Operator: "LIKE"},
			"started":      {Column: "started", Operator: ">="},
			"ended":        {Column: "ended", Operator: ">="},
		},
	}

	// Apply filters dynamically
	if mappings, exists := filterMappings[tableName]; exists {
		for filterName, mapping := range mappings {
			if value := getParamValue(ctx, "filter-"+filterName); value != "" {
				switch mapping.Operator {
				case "LIKE":
					conditions = append(conditions, mapping.Column+" LIKE ?")
					args = append(args, "%"+value+"%")
				case "=":
					conditions = append(conditions, mapping.Column+" = ?")
					args = append(args, value)
				case ">=":
					conditions = append(conditions, mapping.Column+" >= ?")
					args = append(args, value)
				}
			}
		}
	}

	// Handle legacy cases and additional custom logic
	switch tableName {

	case "dbmovie_titles":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dbmovie_titles.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if movieTitle := getParamValue(ctx, "filter-movie_title"); movieTitle != "" {
			conditions = append(conditions, "dbmovies.title LIKE ?")
			args = append(args, "%"+movieTitle+"%")
		}
		if region := getParamValue(ctx, "filter-region"); region != "" {
			conditions = append(conditions, "dbmovie_titles.region LIKE ?")
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

	case "movie_files":
		if filename := getParamValue(ctx, "filter-filename"); filename != "" {
			conditions = append(conditions, "movie_files.filename LIKE ?")
			args = append(args, "%"+filename+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "movie_files.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if resolution := getParamValue(ctx, "filter-resolution"); resolution != "" {
			conditions = append(conditions, "movie_files.resolution LIKE ?")
			args = append(args, "%"+resolution+"%")
		}
	case "serie_episode_files":
		if filename := getParamValue(ctx, "filter-filename"); filename != "" {
			conditions = append(conditions, "serie_episode_files.filename LIKE ?")
			args = append(args, "%"+filename+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "serie_episode_files.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if resolution := getParamValue(ctx, "filter-resolution"); resolution != "" {
			conditions = append(conditions, "serie_episode_files.resolution LIKE ?")
			args = append(args, "%"+resolution+"%")
		}

	case "dbserie_alternates":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dbserie_alternates.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if seriesName := getParamValue(ctx, "filter-series_name"); seriesName != "" {
			conditions = append(conditions, "dbseries.seriename LIKE ?")
			args = append(args, "%"+seriesName+"%")
		}
		if region := getParamValue(ctx, "filter-region"); region != "" {
			conditions = append(conditions, "dbserie_alternates.region LIKE ?")
			args = append(args, "%"+region+"%")
		}

	case "dbserie_episodes":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "dbserie_episodes.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if seriesName := getParamValue(ctx, "filter-series_name"); seriesName != "" {
			conditions = append(conditions, "dbseries.seriename LIKE ?")
			args = append(args, "%"+seriesName+"%")
		}
		if season := getParamValue(ctx, "filter-season"); season != "" {
			conditions = append(conditions, "dbserie_episodes.season = ?")
			args = append(args, season)
		}
		if episode := getParamValue(ctx, "filter-episode"); episode != "" {
			conditions = append(conditions, "dbserie_episodes.episode = ?")
			args = append(args, episode)
		}
		if identifier := getParamValue(ctx, "filter-identifier"); identifier != "" {
			conditions = append(conditions, "dbserie_episodes.identifier = ?")
			args = append(args, identifier)
		}

	case "serie_episodes":
		if episodeTitle := getParamValue(ctx, "filter-episode_title"); episodeTitle != "" {
			conditions = append(conditions, "dbserie_episodes.title LIKE ?")
			args = append(args, "%"+episodeTitle+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "serie_episodes.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if missing := getParamValue(ctx, "filter-missing"); missing != "" {
			switch missing {
			case "1":
				conditions = append(conditions, "serie_episodes.missing = 1")
			case "0":
				conditions = append(conditions, "serie_episodes.missing = 0")
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
			conditions = append(conditions, "movie_file_unmatcheds.filepath LIKE ?")
			args = append(args, "%"+filepath+"%")
		}
		if listname := getParamValue(ctx, "filter-listname"); listname != "" {
			conditions = append(conditions, "movie_file_unmatcheds.listname LIKE ?")
			args = append(args, "%"+listname+"%")
		}
		if qualityProfile := getParamValue(ctx, "filter-movie_quality_profile"); qualityProfile != "" {
			conditions = append(conditions, "movies.quality_profile LIKE ?")
			args = append(args, "%"+qualityProfile+"%")
		}

	case "serie_file_unmatcheds":
		if filepath := getParamValue(ctx, "filter-filepath"); filepath != "" {
			conditions = append(conditions, "serie_file_unmatcheds.filepath LIKE ?")
			args = append(args, "%"+filepath+"%")
		}
		if listname := getParamValue(ctx, "filter-listname"); listname != "" {
			conditions = append(conditions, "serie_file_unmatcheds.listname LIKE ?")
			args = append(args, "%"+listname+"%")
		}
		if rootpath := getParamValue(ctx, "filter-series_rootpath"); rootpath != "" {
			conditions = append(conditions, "series.rootpath LIKE ?")
			args = append(args, "%"+rootpath+"%")
		}

	case "movie_histories":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "movie_histories.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if indexer := getParamValue(ctx, "filter-indexer"); indexer != "" {
			conditions = append(conditions, "movie_histories.indexer LIKE ?")
			args = append(args, "%"+indexer+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "movie_histories.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}
		if downloadedDate := getParamValue(ctx, "filter-downloaded_date"); downloadedDate != "" {
			conditions = append(conditions, "DATE(movie_histories.downloaded_at) = ?")
			args = append(args, downloadedDate)
		}

	case "serie_episode_histories":
		if title := getParamValue(ctx, "filter-title"); title != "" {
			conditions = append(conditions, "serie_episode_histories.title LIKE ?")
			args = append(args, "%"+title+"%")
		}
		if episodeTitle := getParamValue(ctx, "filter-episode_title"); episodeTitle != "" {
			conditions = append(conditions, "dbserie_episodes.title LIKE ?")
			args = append(args, "%"+episodeTitle+"%")
		}
		if indexer := getParamValue(ctx, "filter-indexer"); indexer != "" {
			conditions = append(conditions, "serie_episode_histories.indexer LIKE ?")
			args = append(args, "%"+indexer+"%")
		}
		if quality := getParamValue(ctx, "filter-quality_profile"); quality != "" {
			conditions = append(conditions, "serie_episode_histories.quality_profile LIKE ?")
			args = append(args, "%"+quality+"%")
		}

	}

	if len(conditions) == 0 {
		return "", nil
	}

	return strings.Join(conditions, " AND "), args
}

package api

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
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

// getAdminFormColumns returns only the actual table columns (excluding joined columns) for forms.
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

	columns := make([]ColumnInfo, 0, len(columnNames))
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

// getStructFieldDisplayName uses reflection to get displayname tag from struct field.
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

// getFieldMapping returns both struct field name and descriptive display name for a database field.
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

	// Book fields
	case "dbbook_id":
		return FieldMapping{"DbbookID", "Database Book ID"}
	case "dbauthor_id":
		return FieldMapping{"DbauthorID", "Database Author ID"}
	case "book_id":
		return FieldMapping{"BookID", "Book ID"}
	case "isbn_13":
		return FieldMapping{"ISBN13", "ISBN-13"}
	case "isbn_10":
		return FieldMapping{"ISBN10", "ISBN-10"}
	case "asin":
		return FieldMapping{"ASIN", "Amazon ASIN"}
	case "openlibrary_id":
		return FieldMapping{"OpenLibraryID", "OpenLibrary ID"}
	case "goodreads_id":
		return FieldMapping{"GoodreadsID", "Goodreads ID"}
	case "page_count":
		return FieldMapping{"PageCount", "Page Count"}
	case "publish_date":
		return FieldMapping{"PublishDate", "Publish Date"}
	case "series_position":
		return FieldMapping{"SeriesPosition", "Series Position"}
	case "dbbook_series_id":
		return FieldMapping{"DbbookSeriesID", "Book Series ID"}

	// Audiobook fields
	case "dbaudiobook_id":
		return FieldMapping{"DbaudiobookID", "Database Audiobook ID"}
	case "dbnarrator_id":
		return FieldMapping{"DbnarratorID", "Database Narrator ID"}
	case "audiobook_id":
		return FieldMapping{"AudiobookID", "Audiobook ID"}
	case "audible_id":
		return FieldMapping{"AudibleID", "Audible ID"}
	case "runtime_minutes":
		return FieldMapping{"RuntimeMinutes", "Runtime (minutes)"}
	case "chapter_count":
		return FieldMapping{"ChapterCount", "Chapter Count"}
	case "abridged":
		return FieldMapping{"Abridged", "Is Abridged"}
	case "narrator":
		return FieldMapping{"Narrator", "Narrator Name"}

	// Music fields
	case "dbalbum_id":
		return FieldMapping{"DbalbumID", "Database Album ID"}
	case "dbartist_id":
		return FieldMapping{"DbartistID", "Database Artist ID"}
	case "album_id":
		return FieldMapping{"AlbumID", "Album ID"}
	case "musicbrainz_id":
		return FieldMapping{"MusicBrainzID", "MusicBrainz ID"}
	case "discogs_id":
		return FieldMapping{"DiscogsID", "Discogs ID"}
	case "track_count":
		return FieldMapping{"TrackCount", "Track Count"}
	case "disc_count":
		return FieldMapping{"DiscCount", "Disc Count"}
	case "catalog_number":
		return FieldMapping{"CatalogNumber", "Catalog Number"}
	case "label":
		return FieldMapping{"Label", "Record Label"}
	case "barcode":
		return FieldMapping{"Barcode", "Barcode"}

	// Fields with poor auto-generated display names
	case "upc":
		return FieldMapping{"UPC", "UPC (Barcode)"}
	case "isrc":
		return FieldMapping{"ISRC", "ISRC"}
	case "acoustid":
		return FieldMapping{"AcoustID", "AcoustID"}
	case "cover_url":
		return FieldMapping{"CoverURL", "Cover URL"}
	case "sample_url":
		return FieldMapping{"SampleURL", "Sample URL"}
	case "image_url":
		return FieldMapping{"ImageURL", "Image URL"}
	case "total_runtime_ms":
		return FieldMapping{"TotalRuntimeMs", "Total Runtime (ms)"}
	case "runtime_ms":
		return FieldMapping{"RuntimeMs", "Runtime (ms)"}
	case "identifiedby":
		return FieldMapping{"Identifiedby", "Identified By"}
	case "siterating":
		return FieldMapping{"Siterating", "Site Rating"}
	case "fanart":
		return FieldMapping{"Fanart", "Fan Art"}
	case "dbtrack_id":
		return FieldMapping{"DbtrackID", "Track Reference"}
	case "book_series_id":
		return FieldMapping{"BookSeriesID", "Book Series ID"}
	case "musicbrainz_release_group_id":
		return FieldMapping{"MusicBrainzReleaseGroupID", "MusicBrainz Release Group ID"}
	case "musicbrainz_release_id":
		return FieldMapping{"MusicBrainzReleaseID", "MusicBrainz Release ID"}
	case "musicbrainz_recording_id":
		return FieldMapping{"MusicBrainzRecordingID", "MusicBrainz Recording ID"}
	case "discogs_master_id":
		return FieldMapping{"DiscogsMasterID", "Discogs Master ID"}
	case "discogs_release_id":
		return FieldMapping{"DiscogsReleaseID", "Discogs Release ID"}
	case "spotify_id":
		return FieldMapping{"SpotifyID", "Spotify ID"}
	case "total_tracks":
		return FieldMapping{"TotalTracks", "Total Tracks"}
	case "release_type":
		return FieldMapping{"ReleaseType", "Release Type"}
	case "explicit":
		return FieldMapping{"Explicit", "Explicit Content"}
	case "track_mode":
		return FieldMapping{"TrackMode", "Track Mode"}
	case "aliases":
		return FieldMapping{"Aliases", "Aliases"}
	case "sort_name":
		return FieldMapping{"SortName", "Sort Name"}
	case "artist_type":
		return FieldMapping{"ArtistType", "Artist Type"}
	case "begin_date":
		return FieldMapping{"BeginDate", "Begin Date"}
	case "end_date":
		return FieldMapping{"EndDate", "End Date"}
	case "birth_date":
		return FieldMapping{"BirthDate", "Birth Date"}
	case "death_date":
		return FieldMapping{"DeathDate", "Death Date"}
	case "average_rating":
		return FieldMapping{"AverageRating", "Average Rating"}
	case "ratings_count":
		return FieldMapping{"RatingsCount", "Ratings Count"}
	case "file_size":
		return FieldMapping{"FileSize", "File Size"}
	case "sample_rate":
		return FieldMapping{"SampleRate", "Sample Rate"}
	case "bit_depth":
		return FieldMapping{"BitDepth", "Bit Depth"}
	case "album_title":
		return FieldMapping{"Title", "Album Title"}
	case "audiobook_title":
		return FieldMapping{"Title", "Audiobook Title"}
	case "book_title":
		return FieldMapping{"Title", "Book Title"}
	case "author_name":
		return FieldMapping{"Name", "Author Name"}
	case "artist_name":
		return FieldMapping{"Name", "Artist Name"}
	case "narrator_name":
		return FieldMapping{"Name", "Narrator Name"}
	default:
		// Convert snake_case to PascalCase with proper capitalization
		parts := strings.Split(dbField, "_")

		var structField strings.Builder

		displayParts := make([]string, 0, len(parts))
		for _, part := range parts {
			if len(part) > 0 {
				structField.WriteString(strings.ToTitle(strings.ToLower(part)))

				displayParts = append(displayParts, strings.ToTitle(strings.ToLower(part)))
			}
		}

		displayName := strings.Join(displayParts, " ")

		return FieldMapping{structField.String(), displayName}
	}
}

// dbFieldToStructField converts database field names to Go struct field names (backward compatibility).
func dbFieldToStructField(dbField string) string {
	return getFieldMapping(dbField).StructField
}

// getDescriptiveFieldName returns descriptive field names (backward compatibility).
func getDescriptiveFieldName(fieldName string) string {
	mapping := getFieldMapping(fieldName)
	if mapping.DisplayName == "" {
		return ""
	}

	return mapping.DisplayName
}

// getColumnTypes returns a map of column names to their SQLite types for a table.
func getColumnTypes(tableName string) map[string]string {
	nameQuery := fmt.Sprintf("SELECT name, type FROM pragma_table_info('%s')", tableName)
	columnNames := database.GetrowsN[database.DbstaticTwoString](false, 100, nameQuery)

	columnTypes := make(map[string]string, len(columnNames))
	for _, col := range columnNames {
		columnTypes[strings.ToLower(col.Str1)] = strings.ToUpper(col.Str2)
	}

	return columnTypes
}

// isIntegerType checks if a SQLite type should be treated as an integer.
func isIntegerType(sqliteType string) bool {
	upper := strings.ToUpper(sqliteType)
	return strings.Contains(upper, "INT") || upper == "BOOLEAN"
}

// isRealType checks if a SQLite type should be treated as a floating-point number.
func isRealType(sqliteType string) bool {
	upper := strings.ToUpper(sqliteType)
	return strings.Contains(upper, "REAL") || strings.Contains(upper, "FLOA") ||
		strings.Contains(upper, "DOUBT") || strings.Contains(upper, "NUMERIC") ||
		strings.Contains(upper, "DECIMAL")
}

// convertValueForColumn converts a string value to the appropriate type based on column type.
// For numeric columns an empty or non-numeric input is coerced to 0 (never written
// back as a string), so the form/API cannot store "" in an integer/real column —
// which the strict sqlite driver would later refuse to scan.
func convertValueForColumn(val any, colName string, columnTypes map[string]string) any {
	strVal, ok := val.(string)
	if !ok {
		return val
	}

	// Check if this is a checkbox field first
	if isCheckboxFieldRefactored(colName) {
		if strVal == "on" || strVal == "true" || strVal == "1" || strVal == "yes" {
			return 1
		}

		return 0
	}

	colType, exists := columnTypes[strings.ToLower(colName)]
	if !exists {
		return val
	}

	// Integer column: always return an int (0 for empty/non-numeric).
	if isIntegerType(colType) {
		if intVal, err := strconv.Atoi(strings.TrimSpace(strVal)); err == nil {
			return intVal
		}

		return 0
	}

	// Real column: always return a float (0 for empty/non-numeric).
	if isRealType(colType) {
		if f, err := strconv.ParseFloat(strings.TrimSpace(strVal), 64); err == nil {
			return f
		}

		return float64(0)
	}

	return val
}

func insertAdminRecord(tableName string, data map[string]any) error {
	if tableName == "" || len(data) == 0 {
		return errors.New("table name and data are required")
	}

	// Get column types for the table
	columnTypes := getColumnTypes(tableName)

	var (
		columns []string
		values  []any
	)

	for col, val := range data {
		// Skip created_at and updated_at columns as they should be managed by the database
		if col == "id" || col == "created_at" || col == "updated_at" || col == "csrf_token" {
			continue
		}

		if val != "" && val != nil { // Skip empty values
			columns = append(columns, col)
			values = append(values, convertValueForColumn(val, col, columnTypes))
		}
	}

	if len(columns) == 0 {
		return errors.New("no data to insert")
	}

	// Use project's database insert method
	_, err := database.InsertArray(tableName, columns, values...)

	return err
}

func updateAdminRecord(tableName string, id int, data map[string]any) error {
	if tableName == "" || len(data) == 0 {
		return errors.New("table name and data are required")
	}

	// Check if record exists by ID
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", tableName)
	count := database.Getdatarow[int](false, query, id)

	if count == 0 {
		return errors.New("record not found")
	}

	// Get column types for the table
	columnTypes := getColumnTypes(tableName)

	// Build update data
	var (
		columns []string
		values  []any
	)

	for col, val := range data {
		// Don't update id, created_at, updated_at, or csrf_token columns
		if col != "id" && col != "created_at" && col != "updated_at" && col != "csrf_token" {
			columns = append(columns, col)
			values = append(values, convertValueForColumn(val, col, columnTypes))
		}
	}

	if len(columns) == 0 {
		return errors.New("no data to update")
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
		return errors.New("table name is required")
	}

	// Check if record exists by ID
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE id = ?", tableName)
	count := database.Getdatarow[int](false, query, id)

	if count == 0 {
		return errors.New("record not found")
	}

	// Use project's database delete method
	_, err := database.DeleteRow(tableName, "id = ?", id)

	return err
}

// getReferenceTable determines the reference table name from a foreign key field.
func getReferenceTable(fieldName string) string {
	if !strings.HasSuffix(fieldName, "_id") {
		return ""
	}

	if strings.EqualFold(fieldName, "imdb_id") || strings.EqualFold(fieldName, "thetvdb_id") ||
		strings.EqualFold(fieldName, "freebase_m_id") ||
		strings.EqualFold(fieldName, "freebase_id") ||
		strings.EqualFold(fieldName, "tvrage_id") ||
		strings.EqualFold(fieldName, "trakt_id") ||
		strings.EqualFold(fieldName, "moviedb_id") ||
		strings.EqualFold(fieldName, "facebook_id") ||
		strings.EqualFold(fieldName, "instagram_id") ||
		strings.EqualFold(fieldName, "twitter_id") {
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
		// Book tables
		"dbbook_id":        "dbbooks",
		"dbauthor_id":      "dbauthors",
		"book_id":          "books",
		"dbbook_series_id": "dbbook_series",
		// Audiobook tables
		"dbaudiobook_id": "dbaudiobooks",
		"dbnarrator_id":  "dbnarrators",
		"audiobook_id":   "audiobooks",
		// Music tables
		"dbalbum_id":  "dbalbums",
		"dbartist_id": "dbartists",
		"album_id":    "albums",
		"dbtrack_id":  "dbtracks",
		// Book series
		"book_series_id": "book_series",
	}

	if refTable, exists := referenceMap[fieldName]; exists {
		return refTable
	}

	// Default: remove _id suffix and add 's' for pluralization
	baseName := strings.TrimSuffix(fieldName, "_id")
	if before, ok := strings.CutSuffix(baseName, "y"); ok {
		return before + "ies"
	}

	return baseName + "s"
}

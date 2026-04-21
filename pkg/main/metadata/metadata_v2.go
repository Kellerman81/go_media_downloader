package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/discogs"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/spotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// -----------------------------------------------------------------------------
// Generic Field Update Functions (reusable across all media types)
// -----------------------------------------------------------------------------

// UpdateString updates a string field if empty or overwrite is true.
// Optional transform function can modify the value before assignment.
// Returns true if the field was updated.
func UpdateString(
	field *string,
	newValue string,
	overwrite bool,
	transform func(string) string,
) bool {
	if (*field == "" || overwrite) && newValue != "" {
		if transform != nil {
			*field = transform(newValue)
		} else {
			*field = newValue
		}

		return true
	}

	return false
}

// UpdateInt updates an int field if zero or overwrite is true.
func UpdateInt(field *int, newValue int, overwrite bool) bool {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
		return true
	}

	return false
}

// UpdateInt32 updates an int32 field if zero or overwrite is true.
func UpdateInt32(field *int32, newValue int32, overwrite bool) bool {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
		return true
	}

	return false
}

// UpdateInt64 updates an int64 field if zero or overwrite is true.
func UpdateInt64(field *int64, newValue int64, overwrite bool) bool {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
		return true
	}

	return false
}

// UpdateUint updates a uint field if zero or overwrite is true.
func UpdateUint(field *uint, newValue uint, overwrite bool) bool {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
		return true
	}

	return false
}

// UpdateUint16 updates a uint16 field if zero or overwrite is true.
func UpdateUint16(field *uint16, newValue uint16, overwrite bool) bool {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
		return true
	}

	return false
}

// UpdateFloat32 updates a float32 field if zero or overwrite is true.
func UpdateFloat32(field *float32, newValue float32, overwrite bool) bool {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
		return true
	}

	return false
}

// UpdateBool updates a bool field if false or overwrite is true.
func UpdateBool(field *bool, newValue bool, overwrite bool) bool {
	if (!*field && newValue) || overwrite {
		*field = newValue
		return true
	}

	return false
}

// UpdateNullTime updates a sql.NullTime field if not valid or overwrite is true.
func UpdateNullTime(field *sql.NullTime, newValue sql.NullTime, overwrite bool) bool {
	if (!field.Valid || overwrite) && newValue.Valid {
		*field = newValue
		return true
	}

	return false
}

// -----------------------------------------------------------------------------
// Common Utility Functions
// -----------------------------------------------------------------------------

// ParseDateString parses a date string in "2006-01-02" format to sql.NullTime.
func ParseDateString(date string) sql.NullTime {
	if date == "" {
		return sql.NullTime{}
	}

	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: t, Valid: true}
}

// TimeToNullTime converts a time.Time to sql.NullTime.
func TimeToNullTime(t time.Time) sql.NullTime {
	if t.IsZero() {
		return sql.NullTime{}
	}

	return sql.NullTime{Time: t, Valid: true}
}

// ExtractYearFromDate extracts the year from a sql.NullTime.
func ExtractYearFromDate(date sql.NullTime) uint16 {
	if !date.Valid || date.Time.Year() == 0 {
		return 0
	}

	return uint16(date.Time.Year())
}

// ExtractYearFromTime extracts the year from a time.Time.
func ExtractYearFromTime(t time.Time) uint16 {
	if t.IsZero() || t.Year() == 0 {
		return 0
	}

	return uint16(t.Year())
}

// GenerateSlug creates a URL-friendly slug from a title.
func GenerateSlug(title string) string {
	if title == "" {
		return ""
	}

	return logger.StringToSlug(title)
}

// CleanTitle sanitizes a title string by unquoting and handling HTML entities.
func CleanTitle(title string) string {
	return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
}

// BuildCommaSeparatedString builds a comma-separated string from a slice.
func BuildCommaSeparatedString(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	}

	// Use strings.Join for 2+ items - it's optimized and handles capacity well
	return logger.JoinStringsSep(items, ",")
}

// -----------------------------------------------------------------------------
// Title Management Functions (reusable for all media types)
// -----------------------------------------------------------------------------

// TitleConfig contains configuration for title table operations.
type TitleConfig struct {
	TableName      string // Database table name (e.g., "dbmovie_titles", "dbbook_titles")
	ParentIDColumn string // Foreign key column name (e.g., "dbmovie_id", "dbbook_id")
	CacheKey       string // Cache key for this title type
}

// GetTitleConfigs returns the title configuration for each media type.
func GetTitleConfigs() map[uint]TitleConfig {
	return map[uint]TitleConfig{
		config.MediaTypeMovie: {
			TableName:      "dbmovie_titles",
			ParentIDColumn: "dbmovie_id",
			CacheKey:       logger.CacheTitlesMovie,
		},
		config.MediaTypeSeries: {
			TableName:      "dbserie_alternates",
			ParentIDColumn: "dbserie_id",
			CacheKey:       logger.CacheDBSeriesAlt,
		},
		config.MediaTypeBook: {
			TableName:      "dbbook_titles",
			ParentIDColumn: "dbbook_id",
			CacheKey:       "CacheTitlesBook",
		},
		config.MediaTypeAudiobook: {
			TableName:      "dbaudiobook_titles",
			ParentIDColumn: "dbaudiobook_id",
			CacheKey:       "CacheTitlesAudiobook",
		},
		config.MediaTypeMusic: {
			TableName:      "dbalbum_titles",
			ParentIDColumn: "dbalbum_id",
			CacheKey:       "CacheTitlesAlbum",
		},
	}
}

// AddAlternateTitle adds an alternate title to the database if it doesn't exist.
func AddAlternateTitle(
	mediaType uint,
	parentID uint,
	title, region string,
	existingTitles []database.DbstaticTwoString,
) bool {
	if title == "" || database.GetDBStaticTwoStringIdx1(existingTitles, title) != -1 {
		return false
	}

	cfg, ok := GetTitleConfigs()[mediaType]
	if !ok {
		return false
	}

	// Check if title already exists
	var count int

	countQuery := "select count() from " + cfg.TableName + " where " + cfg.ParentIDColumn + " = ? and title = ? COLLATE NOCASE"
	database.Scanrowsdyn(false, countQuery, &count, &parentID, &title)

	if count > 0 {
		return false
	}

	// Insert new title
	slug := GenerateSlug(title)
	insertQuery := "INSERT INTO " + cfg.TableName + " (title, slug, " + cfg.ParentIDColumn + ", region) VALUES (?, ?, ?, ?)"
	database.ExecN(insertQuery, &title, &slug, &parentID, &region)

	// Update cache if enabled
	if config.GetSettingsGeneral().UseMediaCache && cfg.CacheKey != "" {
		database.AppendCacheTwoString(
			cfg.CacheKey,
			syncops.DbstaticTwoStringOneInt{Num: parentID, Str1: title, Str2: slug},
		)
	}

	return true
}

// GetExistingTitles retrieves existing alternate titles for a media item.
func GetExistingTitles(mediaType uint, parentID uint) []database.DbstaticTwoString {
	cfg, ok := GetTitleConfigs()[mediaType]
	if !ok {
		return nil
	}

	countQuery := "select count() from " + cfg.TableName + " where " + cfg.ParentIDColumn + " = ?"
	selectQuery := "select title, slug from " + cfg.TableName + " where " + cfg.ParentIDColumn + " = ?"

	return database.Getrowssize[database.DbstaticTwoString](
		false,
		countQuery,
		selectQuery,
		&parentID,
	)
}

// ShouldProcessTitle checks if a title should be added based on language filters.
func ShouldProcessTitle(
	region, title string,
	existingTitles []database.DbstaticTwoString,
	allowedLanguages []string,
) bool {
	if title == "" || database.GetDBStaticTwoStringIdx1(existingTitles, title) != -1 {
		return false
	}

	if len(allowedLanguages) >= 1 && region != "" {
		return logger.SlicesContainsI(allowedLanguages, region)
	}

	return true
}

// -----------------------------------------------------------------------------
// Provider Singletons (thread-safe lazy initialization using sync.Once)
// -----------------------------------------------------------------------------

var (
	openLibraryProvider *openlibrary.Provider
	audibleProvider     *audible.Provider
	audnexProvider      *audnex.Provider
	musicbrainzProvider *musicbrainz.Provider
	goodreadsProvider   *goodreads.Provider
	discogsProvider     *discogs.Provider
	spotifyProvider     *spotify.Provider

	// sync.Once instances for thread-safe initialization.
	openLibraryOnce sync.Once
	audibleOnce     sync.Once
	audnexOnce      sync.Once
	musicbrainzOnce sync.Once
	goodreadsOnce   sync.Once
	discogsOnce     sync.Once
	spotifyOnce     sync.Once
)

func getOpenLibraryProvider() *openlibrary.Provider {
	openLibraryOnce.Do(func() {
		openLibraryProvider = openlibrary.NewProvider()
	})
	return openLibraryProvider
}

func getAudibleProvider(region audible.Region) *audible.Provider {
	audibleOnce.Do(func() {
		if region == "" {
			region = audible.RegionUS
		}

		audibleProvider = audible.NewProviderWithRegion(region)
	})

	return audibleProvider
}

func getAudnexProvider() *audnex.Provider {
	audnexOnce.Do(func() {
		audnexProvider = audnex.NewProvider()
	})
	return audnexProvider
}

func getMusicBrainzProvider() *musicbrainz.Provider {
	musicbrainzOnce.Do(func() {
		musicbrainzProvider = musicbrainz.NewProvider()
	})
	return musicbrainzProvider
}

func getGoodreadsProvider(apiKey string) *goodreads.Provider {
	goodreadsOnce.Do(func() {
		goodreadsProvider = goodreads.NewProvider(apiKey)
	})
	return goodreadsProvider
}

func getDiscogsProvider() *discogs.Provider {
	discogsOnce.Do(func() {
		token := config.GetSettingsGeneral().DiscogsToken
		if token != "" {
			discogsProvider = discogs.NewProviderWithToken(token)
		} else {
			discogsProvider = discogs.NewProvider()
		}
	})

	return discogsProvider
}

func getSpotifyProvider() *spotify.Provider {
	spotifyOnce.Do(func() {
		settings := config.GetSettingsGeneral()
		clientID := settings.SpotifyClientID
		clientSecret := settings.SpotifyClientSecret

		if clientID == "" || clientSecret == "" {
			return
		}

		spotifyProvider = spotify.NewProvider(clientID, clientSecret)
		if settings.SpotifyRegion != "" {
			spotifyProvider.SetRegion(settings.SpotifyRegion)
		}
	})

	return spotifyProvider
}

// -----------------------------------------------------------------------------
// Book Metadata Functions
// -----------------------------------------------------------------------------

// BookGetMetadata retrieves metadata for a book from configured sources.
func BookGetMetadata(ctx context.Context, book *database.Dbbook, overwrite bool) error {
	logger.Logtype("info", 1).
		Str(logger.StrTitle, book.Title).
		Msg("Get book metadata for")
	defer logger.Logtype("info", 1).
		Str(logger.StrTitle, book.Title).
		Msg("ended get book metadata for")

	// Try OpenLibrary first (by ISBN)
	if book.ISBN13 != "" || book.ISBN10 != "" {
		isbn := book.ISBN13
		if isbn == "" {
			isbn = book.ISBN10
		}

		if err := bookUpdateFromOpenLibrary(ctx, book, isbn, overwrite); err != nil {
			logger.Logtype("debug", 2).Err(err).Msg("OpenLibrary lookup failed")
		}
	}

	// Update slug if we have a title
	if book.Title != "" && (book.Slug == "" || overwrite) {
		book.Slug = GenerateSlug(book.Title)
	}

	return nil
}

// bookUpdateFromOpenLibrary updates book metadata from OpenLibrary.
func bookUpdateFromOpenLibrary(
	ctx context.Context,
	book *database.Dbbook,
	isbn string,
	overwrite bool,
) error {
	provider := getOpenLibraryProvider()

	details, err := provider.SearchByISBN(ctx, isbn)
	if err != nil {
		return err
	}

	applyBookDetails(book, details, overwrite)

	return nil
}

// applyBookDetails applies API book details to a database book record.
func applyBookDetails(book *database.Dbbook, details *apiexternal_v2.BookDetails, overwrite bool) {
	if details == nil {
		return
	}

	UpdateString(&book.Title, details.Title, overwrite, CleanTitle)
	UpdateString(&book.Description, details.Description, overwrite, nil)
	UpdateString(&book.Publisher, details.Publisher, overwrite, nil)
	UpdateString(&book.Language, details.Language, overwrite, nil)
	UpdateString(&book.ISBN13, details.ISBN13, overwrite, nil)
	UpdateString(&book.ISBN10, details.ISBN10, overwrite, nil)
	UpdateString(&book.ASIN, details.ASIN, overwrite, nil)
	UpdateString(&book.OpenlibraryID, details.OpenLibraryID, overwrite, nil)
	UpdateString(&book.GoodreadsID, details.GoodreadsID, overwrite, nil)
	UpdateString(&book.CoverURL, details.CoverURL, overwrite, nil)
	UpdateString(&book.SeriesPosition, details.SeriesPosition, overwrite, nil)

	UpdateInt(&book.PageCount, details.PageCount, overwrite)
	UpdateFloat32(&book.AverageRating, float32(details.AverageRating), overwrite)
	UpdateInt32(&book.RatingsCount, int32(details.RatingsCount), overwrite)

	if len(details.Genres) > 0 {
		UpdateString(&book.Genres, BuildCommaSeparatedString(details.Genres), overwrite, nil)
	}

	if details.PublishDate.IsZero() {
		return
	}

	UpdateNullTime(&book.PublishDate, TimeToNullTime(details.PublishDate), overwrite)

	if book.Year == 0 || overwrite {
		book.Year = ExtractYearFromTime(details.PublishDate)
	}
}

// BookSearchByTitle searches for books by title and author.
func BookSearchByTitle(
	ctx context.Context,
	title, author string,
	limit int,
) ([]apiexternal_v2.BookSearchResult, error) {
	provider := getOpenLibraryProvider()
	return provider.SearchBooks(ctx, title, author, limit)
}

// AuthorGetMetadata retrieves metadata for an author from configured sources.
func AuthorGetMetadata(ctx context.Context, author *database.Dbauthor, overwrite bool) error {
	if author.OpenlibraryID == "" {
		return nil
	}

	provider := getOpenLibraryProvider()

	details, err := provider.GetAuthorByID(ctx, author.OpenlibraryID)
	if err != nil {
		return err
	}

	applyAuthorDetails(author, details, overwrite)

	return nil
}

// applyAuthorDetails applies API author details to a database author record.
func applyAuthorDetails(
	author *database.Dbauthor,
	details *apiexternal_v2.AuthorDetails,
	overwrite bool,
) {
	if details == nil {
		return
	}

	UpdateString(&author.Name, details.Name, overwrite, nil)
	UpdateString(&author.Bio, details.Bio, overwrite, nil)
	UpdateString(&author.Website, details.Website, overwrite, nil)
	UpdateString(&author.ImageURL, details.ImageURL, overwrite, nil)
	UpdateString(&author.OpenlibraryID, details.OpenLibraryID, overwrite, nil)
	UpdateString(&author.GoodreadsID, details.GoodreadsID, overwrite, nil)

	if !details.BirthDate.IsZero() {
		author.BirthDate = details.BirthDate.Format("2006-01-02")
	}

	if !details.DeathDate.IsZero() {
		author.DeathDate = details.DeathDate.Format("2006-01-02")
	}
}

// -----------------------------------------------------------------------------
// Audiobook Metadata Functions
// -----------------------------------------------------------------------------

// AudiobookGetMetadata retrieves metadata for an audiobook from configured sources.
func AudiobookGetMetadata(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	audiobook *database.Dbaudiobook,
	overwrite bool,
) error {
	logger.Logtype("info", 1).
		Str(logger.StrTitle, audiobook.Title).
		Msg("Get audiobook metadata for")
	defer logger.Logtype("info", 1).
		Str(logger.StrTitle, audiobook.Title).
		Msg("ended get audiobook metadata for")

	// Try Audible first (by ASIN)
	if audiobook.ASIN != "" {
		audibleErr := audiobookUpdateFromAudible(
			ctx,
			audible.Region(cfgp.AudibleRegion),
			audiobook,
			overwrite,
		)
		if audibleErr != nil {
			logger.Logtype("debug", 2).Err(audibleErr).Msg("Audible lookup failed, trying Audnex")
			// If Audible failed completely, try Audnex
			if err := audiobookUpdateFromAudnex(
				ctx,
				audible.Region(cfgp.AudibleRegion),
				audiobook,
				overwrite,
			); err != nil {
				logger.Logtype("debug", 2).Err(err).Msg("Audnex lookup also failed")
			}
		} else {
			// Audible succeeded, but check if we need ISBN/Goodreads ID from Audnex
			// This gives us the book linkage data that Audible might not provide
			if audiobook.DbbookID == 0 {
				logger.Logtype("debug", 2).
					Msg("Audible succeeded but no book link, trying Audnex for additional metadata")

				if err := audiobookUpdateFromAudnex(
					ctx,
					audible.Region(cfgp.AudibleRegion),
					audiobook,
					false,
				); err != nil {
					logger.Logtype("debug", 2).Err(err).Msg("Audnex supplemental lookup failed")
				}
			}
		}
	}

	// If we still don't have book linkage after Audible/Audnex, try Goodreads
	if audiobook.DbbookID == 0 && audiobook.Title != "" {
		logger.Logtype("debug", 2).Msg("No book link after Audible/Audnex, trying Goodreads")

		if err := audiobookSupplementFromGoodreads(ctx, audiobook); err != nil {
			logger.Logtype("debug", 2).Err(err).Msg("Goodreads supplemental lookup failed")
		}
	}

	// Update slug if we have a title
	if audiobook.Title != "" && (audiobook.Slug == "" || overwrite) {
		audiobook.Slug = GenerateSlug(audiobook.Title)
	}

	return nil
}

// audiobookUpdateFromAudible updates audiobook metadata from Audible.
func audiobookUpdateFromAudnex(
	ctx context.Context,
	region audible.Region,
	audiobook *database.Dbaudiobook,
	overwrite bool,
) error {
	provider := getAudnexProvider()

	details, err := provider.GetBookByASIN(ctx, audiobook.ASIN, string(region))
	if err != nil {
		return err
	}

	applyAudiobookDetails(audiobook, details, overwrite)

	return nil
}

// audiobookUpdateFromAudible updates audiobook metadata from Audible.
func audiobookUpdateFromAudible(
	ctx context.Context,
	region audible.Region,
	audiobook *database.Dbaudiobook,
	overwrite bool,
) error {
	provider := getAudibleProvider(region)

	details, err := provider.SearchByASIN(ctx, audiobook.ASIN)
	if err != nil {
		return err
	}

	applyAudiobookDetails(audiobook, details, overwrite)

	return nil
}

// audiobookSupplementFromGoodreads supplements audiobook metadata using Goodreads API.
// This function searches for book metadata by title and author to find ISBN and Goodreads ID.
// It uses Levenshtein distance to find the best match, similar to beets-audible plugin.
func audiobookSupplementFromGoodreads(ctx context.Context, audiobook *database.Dbaudiobook) error {
	apiKey := config.GetSettingsGeneral().GoodreadsAPIKey
	if apiKey == "" {
		return fmt.Errorf("Goodreads API key not configured")
	}

	provider := getGoodreadsProvider(apiKey)

	// Get the primary author name for the search query
	var authorName string
	database.Scanrowsdyn(false,
		`SELECT a.name FROM dbauthors a
		 JOIN dbaudiobook_authors aa ON a.id = aa.dbauthor_id
		 WHERE aa.dbaudiobook_id = ?
		 ORDER BY aa.position ASC
		 LIMIT 1`,
		&authorName, &audiobook.ID)

	// Construct search query
	searchQuery := audiobook.Title
	if authorName != "" {
		searchQuery += " " + authorName
	}

	logger.Logtype("debug", 2).
		Str("query", searchQuery).
		Msg("Searching Goodreads for audiobook metadata")

	// Search Goodreads for the book - get multiple results to find best match
	results, err := provider.SearchBooks(ctx, searchQuery, 10)
	if err != nil {
		return fmt.Errorf("Goodreads search error: %w", err)
	}

	if len(results) == 0 {
		return fmt.Errorf("no Goodreads results found")
	}

	// Find the best match using Levenshtein distance (like beets-audible)
	bestMatch := goodreadsFindBestMatch(results, audiobook.Title, authorName)
	if bestMatch == nil {
		return fmt.Errorf("no suitable Goodreads match found")
	}

	logger.Logtype("debug", 2).
		Str("matched_title", bestMatch.Title).
		Str("matched_id", bestMatch.ID).
		Msg("Found best Goodreads match")

	// Get detailed book info from the best match
	bookDetails, err := provider.GetBookByID(ctx, bestMatch.ID)
	if err != nil {
		return fmt.Errorf("Goodreads book lookup error: %w", err)
	}

	// Create a mock AudiobookDetails to use with applyAudiobookDetails
	details := &apiexternal_v2.AudiobookDetails{
		Title:          bookDetails.Title,
		Description:    bookDetails.Description,
		Publisher:      bookDetails.Publisher,
		Language:       bookDetails.Language,
		ReleaseYear:    bookDetails.PublishYear,
		ISBN:           bookDetails.ISBN13,
		GoodreadsID:    bookDetails.GoodreadsID,
		SeriesName:     bookDetails.SeriesName,
		SeriesPosition: bookDetails.SeriesPosition,
	}

	if !bookDetails.PublishDate.IsZero() {
		details.ReleaseDate = bookDetails.PublishDate
	}

	logger.Logtype("debug", 1).
		Str("title", bookDetails.Title).
		Str("isbn", bookDetails.ISBN13).
		Str("goodreads_id", bookDetails.GoodreadsID).
		Int("release_year", bookDetails.PublishYear).
		Msg("Found Goodreads match for audiobook")

	// Apply the book details (but don't overwrite existing audiobook-specific data)
	applyAudiobookDetails(audiobook, details, false)

	return nil
}

// goodreadsFindBestMatch finds the best matching book from Goodreads results using Levenshtein distance.
// This mimics the approach used in beets-audible plugin for better matching accuracy.
func goodreadsFindBestMatch(
	results []apiexternal_v2.BookSearchResult,
	title, author string,
) *apiexternal_v2.BookSearchResult {
	if len(results) == 0 {
		return nil
	}

	// Normalize inputs for comparison
	normalizedTitle := strings.ToLower(strings.TrimSpace(cleanGoodreadsTitle(title)))
	normalizedAuthor := strings.ToLower(strings.ReplaceAll(author, " ", ""))

	var bestMatch *apiexternal_v2.BookSearchResult

	bestDistance := -1

	for i := range results {
		result := &results[i]

		// Clean and normalize result title (remove series info after parentheses)
		resultTitle := strings.ToLower(strings.TrimSpace(cleanGoodreadsTitle(result.Title)))

		// Normalize author name (remove spaces like "James S. A. Corey" -> "JamesS.A.Corey")
		resultAuthor := ""
		if len(result.Authors) > 0 {
			resultAuthor = strings.ToLower(strings.ReplaceAll(result.Authors[0], " ", ""))
		}

		// Calculate Levenshtein distance for both title and author
		titleDistance := levenshteinDistance(normalizedTitle, resultTitle)
		authorDistance := levenshteinDistance(normalizedAuthor, resultAuthor)

		// Combined distance (weighted: title is more important)
		combinedDistance := titleDistance*2 + authorDistance

		if bestDistance == -1 || combinedDistance < bestDistance {
			bestDistance = combinedDistance
			bestMatch = result
		}

		logger.Logtype("debug", 3).
			Str("result_title", result.Title).
			Str("cleaned_title", resultTitle).
			Int("title_distance", titleDistance).
			Int("author_distance", authorDistance).
			Int("combined_distance", combinedDistance).
			Msg("Goodreads match candidate")
	}

	return bestMatch
}

// cleanGoodreadsTitle removes series and other non-title info that Goodreads puts in parentheses.
func cleanGoodreadsTitle(title string) string {
	// Find first opening parenthesis and trim everything after it
	if idx := strings.IndexByte(title, '('); idx != -1 {
		title = title[:idx]
	}

	return strings.TrimSpace(title)
}

// levenshteinPool provides reusable slices for Levenshtein distance calculation.
// This reduces allocations when computing distances for multiple strings.
var levenshteinPool = sync.Pool{
	New: func() any {
		// Pre-allocate with reasonable capacity for typical strings
		return &levenshteinBuffers{
			prev: make([]int, 0, 256),
			curr: make([]int, 0, 256),
		}
	},
}

type levenshteinBuffers struct {
	prev []int
	curr []int
}

// levenshteinDistance calculates the Levenshtein distance between two strings.
// This is used for fuzzy matching of titles and authors.
// Optimized to use sync.Pool to reduce allocations.
func levenshteinDistance(s1, s2 string) int {
	len1, len2 := len(s1), len(s2)

	if len1 == 0 {
		return len2
	}

	if len2 == 0 {
		return len1
	}

	// Get buffers from pool
	bufs := levenshteinPool.Get().(*levenshteinBuffers)
	defer levenshteinPool.Put(bufs)

	// Ensure capacity and reset length
	needed := len2 + 1
	if cap(bufs.prev) < needed {
		bufs.prev = make([]int, needed)
		bufs.curr = make([]int, needed)
	} else {
		bufs.prev = bufs.prev[:needed]
		bufs.curr = bufs.curr[:needed]
	}

	prev := bufs.prev
	curr := bufs.curr

	for j := range prev {
		prev[j] = j
	}

	for i := 1; i <= len1; i++ {
		curr[0] = i
		for j := 1; j <= len2; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}

			curr[j] = min(
				prev[j]+1,      // deletion
				curr[j-1]+1,    // insertion
				prev[j-1]+cost, // substitution
			)
		}

		prev, curr = curr, prev
	}

	return prev[len2]
}

// applyAudiobookDetails applies API audiobook details to a database audiobook record.
func applyAudiobookDetails(
	audiobook *database.Dbaudiobook,
	details *apiexternal_v2.AudiobookDetails,
	overwrite bool,
) {
	if details == nil {
		return
	}

	UpdateString(&audiobook.Title, details.Title, overwrite, CleanTitle)
	UpdateString(&audiobook.ASIN, details.ASIN, overwrite, nil)
	UpdateString(&audiobook.AudibleID, details.ID, overwrite, nil)
	UpdateString(&audiobook.Description, details.Description, overwrite, nil)
	UpdateString(&audiobook.Publisher, details.Publisher, overwrite, nil)
	UpdateString(&audiobook.Language, details.Language, overwrite, nil)
	UpdateString(&audiobook.CoverURL, details.CoverURL, overwrite, nil)

	UpdateInt(&audiobook.RuntimeMinutes, details.RuntimeMinutes, overwrite)
	UpdateFloat32(&audiobook.AverageRating, float32(details.AverageRating), overwrite)
	UpdateInt32(&audiobook.RatingsCount, int32(details.RatingsCount), overwrite)

	if !details.ReleaseDate.IsZero() {
		UpdateNullTime(&audiobook.ReleaseDate, TimeToNullTime(details.ReleaseDate), overwrite)

		if audiobook.Year == 0 || overwrite {
			audiobook.Year = ExtractYearFromTime(details.ReleaseDate)
		}
	}

	// Populate dbauthors and dbaudiobook_authors tables
	if len(details.Authors) > 0 && audiobook.ID > 0 {
		for idx, authorName := range details.Authors {
			if authorName == "" {
				continue
			}

			// Get or create author in dbauthors
			var dbauthorID uint

			authorSlug := logger.StringToSlug(authorName)
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbauthors WHERE name = ? COLLATE NOCASE OR slug = ?",
				&dbauthorID,
				&authorName,
				&authorSlug,
			)

			if dbauthorID == 0 {
				// Create new author
				result, err := database.ExecNid(
					"INSERT INTO dbauthors (name, slug) VALUES (?, ?)",
					&authorName,
					&authorSlug,
				)
				if err == nil {
					dbauthorID = logger.Int64ToUint(result)
				}
			}

			if dbauthorID == 0 {
				continue
			}

			// Link author to audiobook in dbaudiobook_authors
			var existingRelation uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbaudiobook_authors WHERE dbaudiobook_id = ? AND dbauthor_id = ?",
				&existingRelation,
				&audiobook.ID,
				&dbauthorID,
			)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					"INSERT INTO dbaudiobook_authors (dbaudiobook_id, dbauthor_id, position) VALUES (?, ?, ?)",
					&audiobook.ID,
					&dbauthorID,
					&idx,
				)
			}
		}
	}

	// Populate dbnarrators and dbaudiobook_narrators tables
	if len(details.Narrators) > 0 && audiobook.ID > 0 {
		for idx, narratorName := range details.Narrators {
			if narratorName == "" {
				continue
			}

			// Get or create narrator in dbnarrators
			var dbnarratorID uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbnarrators WHERE name = ?",
				&dbnarratorID,
				&narratorName,
			)

			if dbnarratorID == 0 {
				// Create new narrator
				result, err := database.ExecNid(
					"INSERT INTO dbnarrators (name) VALUES (?)",
					&narratorName,
				)
				if err == nil {
					dbnarratorID = logger.Int64ToUint(result)
				}
			}

			if dbnarratorID == 0 {
				continue
			}

			// Link narrator to audiobook in dbaudiobook_narrators
			var existingRelation uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbaudiobook_narrators WHERE dbaudiobook_id = ? AND dbnarrator_id = ?",
				&existingRelation,
				&audiobook.ID,
				&dbnarratorID,
			)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					"INSERT INTO dbaudiobook_narrators (dbaudiobook_id, dbnarrator_id, position) VALUES (?, ?, ?)",
					&audiobook.ID,
					&dbnarratorID,
					&idx,
				)
			}
		}
	}

	// Populate dbaudiobook_chapters table
	if len(details.Chapters) > 0 && audiobook.ID > 0 {
		for i := range details.Chapters {
			// Check if chapter already exists
			var existingChapter uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbaudiobook_chapters WHERE dbaudiobook_id = ? AND chapter_number = ?",
				&existingChapter,
				&audiobook.ID,
				&details.Chapters[i].ChapterNumber,
			)

			if existingChapter == 0 {
				endTimeMs := details.Chapters[i].StartOffsetMs + details.Chapters[i].LengthMs

				_, _ = database.ExecNid(
					"INSERT INTO dbaudiobook_chapters (dbaudiobook_id, title, chapter_number, position, start_time_ms, end_time_ms, runtime_ms) VALUES (?, ?, ?, ?, ?, ?, ?)",
					&audiobook.ID,
					&details.Chapters[i].Title,
					&details.Chapters[i].ChapterNumber,
					&details.Chapters[i].Number,
					&details.Chapters[i].StartOffsetMs,
					&endTimeMs,
					&details.Chapters[i].LengthMs,
				)
			}
		}
	}

	// Populate dbbooks and link audiobook if we have book metadata
	if audiobook.ID == 0 ||
		(details.ISBN == "" && details.GoodreadsID == "" && details.Title == "") {
		return
	}

	var dbbookID uint

	// Try to find existing book by ISBN or Goodreads ID
	if details.ISBN != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbbooks WHERE isbn_13 = ?",
			&dbbookID,
			&details.ISBN,
		)
	}

	if dbbookID == 0 && details.GoodreadsID != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbbooks WHERE goodreads_id = ?",
			&dbbookID,
			&details.GoodreadsID,
		)
	}

	// If no existing book found, create one
	if dbbookID == 0 {
		var publishDate string
		if !details.ReleaseDate.IsZero() {
			publishDate = details.ReleaseDate.Format("2006-01-02")
		}

		result, err := database.ExecNid(
			"INSERT INTO dbbooks (title, isbn_13, goodreads_id, description, publisher, language, publish_date, year, series_position) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&details.Title,
			&details.ISBN,
			&details.GoodreadsID,
			&details.Description,
			&details.Publisher,
			&details.Language,
			&publishDate,
			&details.ReleaseYear,
			&details.SeriesPosition,
		)
		if err == nil {
			dbbookID = logger.Int64ToUint(result)
		}
	}

	// Handle series information if we have a series name
	if dbbookID > 0 && details.SeriesName != "" {
		var seriesID uint

		seriesSlug := logger.StringToSlug(details.SeriesName)

		// Try to find existing series by name or slug
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbbook_series WHERE name = ? COLLATE NOCASE OR slug = ?",
			&seriesID,
			&details.SeriesName,
			&seriesSlug,
		)

		// If no existing series found, create one
		if seriesID == 0 {
			result, err := database.ExecNid(
				"INSERT INTO dbbook_series (name, slug, goodreads_id) VALUES (?, ?, ?)",
				&details.SeriesName, &seriesSlug, &details.GoodreadsID,
			)
			if err == nil {
				seriesID = logger.Int64ToUint(result)
			}
		}

		// Link book to series
		if seriesID > 0 {
			database.ExecN(
				"UPDATE dbbooks SET dbbook_series_id = ?, series_position = ? WHERE id = ?",
				&seriesID, &details.SeriesPosition, &dbbookID,
			)
		}
	}

	// Link audiobook to book
	if dbbookID > 0 && audiobook.DbbookID == 0 {
		database.ExecN(
			"UPDATE dbaudiobooks SET dbbook_id = ? WHERE id = ?",
			&dbbookID, &audiobook.ID,
		)

		audiobook.DbbookID = dbbookID
	}
}

// AudiobookSearchByTitle searches for audiobooks by title.
func AudiobookSearchByTitle(
	ctx context.Context,
	region audible.Region,
	title string,
	limit int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	provider := getAudibleProvider(region)
	return provider.SearchByTitle(ctx, title, limit)
}

// AudiobookSearchByAuthor searches for audiobooks by author.
func AudiobookSearchByAuthor(
	ctx context.Context,
	region audible.Region,
	author string,
	limit int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	provider := getAudibleProvider(region)
	return provider.SearchByAuthor(ctx, author, limit)
}

// NarratorGetMetadata retrieves metadata for a narrator from configured sources.
func NarratorGetMetadata(ctx context.Context, narrator *database.Dbnarrator, overwrite bool) error {
	// Audible doesn't provide a separate narrator API - narrator info comes with audiobook details
	return nil
}

// -----------------------------------------------------------------------------
// Music Metadata Functions
// -----------------------------------------------------------------------------

// AlbumGetMetadata retrieves metadata for a music album from configured sources.
func AlbumGetMetadata(ctx context.Context, album *database.Dbalbum, overwrite bool) error {
	logger.Logtype("info", 1).
		Str(logger.StrTitle, album.Title).
		Msg("Get album metadata for")
	defer logger.Logtype("info", 1).
		Str(logger.StrTitle, album.Title).
		Msg("ended get album metadata for")

	// Try MusicBrainz first (by release ID or barcode)
	if album.MusicbrainzReleaseID != "" {
		if err := albumUpdateFromMusicBrainz(ctx, album, overwrite); err != nil {
			logger.Logtype("debug", 2).Err(err).Msg("MusicBrainz lookup failed")
		}
	} else if album.UPC != "" {
		// Try by barcode
		if err := albumUpdateFromMusicBrainzByBarcode(ctx, album, overwrite); err != nil {
			logger.Logtype("debug", 2).Err(err).Msg("MusicBrainz barcode lookup failed")
		}
	}

	// If MusicBrainz didn't provide full metadata, try Discogs as fallback
	// Discogs can be queried by release ID or by barcode
	if album.DiscogsReleaseID != "" {
		logger.Logtype("debug", 2).Msg("Trying Discogs by release ID")

		if err := albumUpdateFromDiscogs(ctx, album, overwrite); err != nil {
			logger.Logtype("debug", 2).Err(err).Msg("Discogs lookup by release ID failed")
		}
	} else if album.UPC != "" && (album.MusicbrainzReleaseID == "" || album.Label == "") {
		// Try Discogs by barcode if we don't have MB data or missing key fields
		logger.Logtype("debug", 2).Msg("Trying Discogs by barcode")

		if err := albumUpdateFromDiscogsByBarcode(ctx, album, overwrite); err != nil {
			logger.Logtype("debug", 2).Err(err).Msg("Discogs barcode lookup failed")
		}
	}

	// Update slug if we have a title
	if album.Title != "" && (album.Slug == "" || overwrite) {
		album.Slug = GenerateSlug(album.Title)
	}

	return nil
}

// albumUpdateFromMusicBrainz updates album metadata from MusicBrainz.
func albumUpdateFromMusicBrainz(
	ctx context.Context,
	album *database.Dbalbum,
	overwrite bool,
) error {
	provider := getMusicBrainzProvider()

	details, err := provider.GetReleaseByID(ctx, album.MusicbrainzReleaseID)
	if err != nil {
		return err
	}

	applyAlbumDetails(album, details, overwrite)

	return nil
}

// albumUpdateFromMusicBrainzByBarcode updates album metadata from MusicBrainz using barcode.
func albumUpdateFromMusicBrainzByBarcode(
	ctx context.Context,
	album *database.Dbalbum,
	overwrite bool,
) error {
	provider := getMusicBrainzProvider()

	details, err := provider.GetReleaseByBarcode(ctx, album.UPC)
	if err != nil {
		return err
	}

	applyAlbumDetails(album, details, overwrite)

	return nil
}

// albumUpdateFromDiscogs updates album metadata from Discogs by release ID.
func albumUpdateFromDiscogs(ctx context.Context, album *database.Dbalbum, overwrite bool) error {
	provider := getDiscogsProvider()

	// Parse Discogs release ID as integer
	var releaseID int
	if _, err := fmt.Sscanf(album.DiscogsReleaseID, "%d", &releaseID); err != nil {
		return fmt.Errorf("invalid Discogs release ID: %w", err)
	}

	details, err := provider.GetReleaseByID(ctx, releaseID)
	if err != nil {
		return err
	}

	applyAlbumDetails(album, details, overwrite)

	return nil
}

// albumUpdateFromDiscogsByBarcode updates album metadata from Discogs using barcode.
func albumUpdateFromDiscogsByBarcode(
	ctx context.Context,
	album *database.Dbalbum,
	overwrite bool,
) error {
	provider := getDiscogsProvider()

	details, err := provider.GetReleaseByBarcode(ctx, album.UPC)
	if err != nil {
		return err
	}

	applyAlbumDetails(album, details, overwrite)

	return nil
}

// applyAlbumDetails applies API release details to a database album record.
func applyAlbumDetails(
	album *database.Dbalbum,
	details *apiexternal_v2.ReleaseDetails,
	overwrite bool,
) {
	if details == nil {
		return
	}

	UpdateString(&album.Title, details.Title, overwrite, CleanTitle)
	UpdateString(&album.MusicbrainzReleaseID, details.MusicBrainzID, overwrite, nil)
	UpdateString(&album.MusicbrainzReleaseGroupID, details.ReleaseGroupID, overwrite, nil)
	UpdateString(&album.DiscogsReleaseID, details.DiscogsID, overwrite, nil)
	UpdateString(&album.SpotifyID, details.SpotifyID, overwrite, nil)

	if details.MasterID > 0 {
		masterIDStr := fmt.Sprintf("%d", details.MasterID)
		UpdateString(&album.DiscogsMasterID, masterIDStr, overwrite, nil)
	}

	UpdateString(&album.Label, details.Label, overwrite, nil)
	UpdateString(&album.Country, details.Country, overwrite, nil)
	UpdateString(&album.ReleaseType, details.Type, overwrite, nil)
	UpdateString(&album.Format, details.Format, overwrite, nil)
	UpdateString(&album.CoverURL, details.CoverURL, overwrite, nil)
	UpdateString(&album.UPC, details.Barcode, overwrite, nil)

	UpdateInt(&album.TotalTracks, details.TrackCount, overwrite)

	if len(details.Genres) > 0 {
		UpdateString(&album.Genres, BuildCommaSeparatedString(details.Genres), overwrite, nil)
	}

	if len(details.Styles) > 0 {
		UpdateString(&album.Styles, BuildCommaSeparatedString(details.Styles), overwrite, nil)
	}

	if !details.ReleaseDate.IsZero() {
		UpdateNullTime(&album.ReleaseDate, TimeToNullTime(details.ReleaseDate), overwrite)

		if album.Year == 0 || overwrite {
			album.Year = ExtractYearFromTime(details.ReleaseDate)
		}
	}

	// Populate dbartists and dbalbum_artists tables
	if len(details.Artists) > 0 && album.ID > 0 {
		for idx, artistRef := range details.Artists {
			if artistRef.Name == "" {
				continue
			}

			// Get or create artist in dbartists
			var dbartistID uint

			artistSlug := logger.StringToSlug(artistRef.Name)
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR slug = ?",
				&dbartistID,
				&artistRef.Name,
				&artistSlug,
			)

			// Get MusicBrainz ID if artist exists
			var artistMBID string
			if dbartistID > 0 {
				database.Scanrowsdyn(
					false,
					"SELECT musicbrainz_id FROM dbartists WHERE id = ?",
					&artistMBID,
					&dbartistID,
				)
			}

			artistWasCreated := false
			if dbartistID == 0 {
				// Create new artist
				artistMBID = artistRef.ID

				result, err := database.ExecNid(
					"INSERT INTO dbartists (name, slug, musicbrainz_id) VALUES (?, ?, ?)",
					&artistRef.Name, &artistSlug, &artistMBID,
				)
				if err == nil {
					dbartistID = logger.Int64ToUint(result)
					artistWasCreated = true
				}
			}

			// If artist was just created and has MusicBrainz ID, fetch full metadata including aliases
			if artistWasCreated && artistMBID != "" && dbartistID > 0 {
				var dbartist database.Dbartist
				if err := dbartist.GetDbartistByIDP(&dbartistID); err == nil {
					// Fetch and apply artist metadata (including aliases)
					_ = ArtistGetMetadata(context.TODO(), &dbartist, false)
					// Update the artist record with fetched metadata
					_, _ = database.ExecNid(
						`UPDATE dbartists SET sort_name = ?, discogs_id = ?, artist_type = ?, country = ?,
						 disambiguation = ?, bio = ?, image_url = ?, genres = ?, begin_date = ?, end_date = ?,
						 updated_at = current_timestamp WHERE id = ?`,
						&dbartist.SortName,
						&dbartist.DiscogsID,
						&dbartist.ArtistType,
						&dbartist.Country,
						&dbartist.Disambiguation,
						&dbartist.Bio,
						&dbartist.ImageURL,
						&dbartist.Genres,
						&dbartist.BeginDate,
						&dbartist.EndDate,
						&dbartistID,
					)
				}
			}

			if dbartistID == 0 {
				continue
			}

			// Link artist to album in dbalbum_artists
			var existingRelation uint
			database.Scanrowsdyn(false,
				"SELECT id FROM dbalbum_artists WHERE dbalbum_id = ? AND dbartist_id = ?",
				&existingRelation, &album.ID, &dbartistID)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					"INSERT INTO dbalbum_artists (dbalbum_id, dbartist_id, position) VALUES (?, ?, ?)",
					&album.ID,
					&dbartistID,
					&idx,
				)
			}
		}
	}

	// Populate dbtracks and dbtrack_artists tables
	if len(details.Tracks) == 0 || album.ID == 0 {
		return
	}

	var existingTrack, dbtrackID uint
	for i := range details.Tracks {
		existingTrack = 0
		dbtrackID = 0
		// Check if track already exists
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbtracks WHERE dbalbum_id = ? AND disc_number = ? AND track_number = ?",
			&existingTrack,
			&album.ID,
			&details.Tracks[i].DiscNumber,
			&details.Tracks[i].Position,
		)

		if existingTrack == 0 {
			// Create new track
			runtimeMs := details.Tracks[i].Duration.Milliseconds()

			result, err := database.ExecNid(
				"INSERT INTO dbtracks (dbalbum_id, title, track_number, disc_number, runtime_ms, acoustid, musicbrainz_recording_id) VALUES (?, ?, ?, ?, ?, ?, ?)",
				&album.ID,
				&details.Tracks[i].Title,
				&details.Tracks[i].Position,
				&details.Tracks[i].DiscNumber,
				&runtimeMs,
				&details.Tracks[i].AcoustID,
				&details.Tracks[i].MusicBrainzID,
			)
			if err == nil {
				dbtrackID = logger.Int64ToUint(result)
			}
		} else {
			dbtrackID = existingTrack
		}

		// Add track artists
		if dbtrackID == 0 || len(details.Tracks[i].Artists) == 0 {
			continue
		}

		var (
			existingRelation uint
			artistID         uint
			artistSlug       string
		)

		for idx, artistRef := range details.Tracks[i].Artists {
			if artistRef.Name == "" {
				continue
			}

			artistID = 0
			existingRelation = 0

			artistSlug = logger.StringToSlug(artistRef.Name)
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbartists WHERE name = ? COLLATE NOCASE OR slug = ?",
				&artistID,
				&artistRef.Name,
				&artistSlug,
			)

			if artistID == 0 {
				result, err := database.ExecNid(
					"INSERT INTO dbartists (name, slug, musicbrainz_id) VALUES (?, ?, ?)",
					&artistRef.Name, &artistSlug, &artistRef.ID,
				)
				if err == nil {
					artistID = logger.Int64ToUint(result)
				}
			} else if artistRef.ID != "" {
				_, _ = database.ExecNid(
					`UPDATE dbartists SET musicbrainz_id = ? WHERE id = ? AND (musicbrainz_id IS NULL OR musicbrainz_id = "")`,
					&artistRef.ID,
					&artistID,
				)
			}

			if artistID == 0 {
				continue
			}

			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbtrack_artists WHERE dbtrack_id = ? AND dbartist_id = ?",
				&existingRelation,
				&dbtrackID,
				&artistID,
			)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					"INSERT INTO dbtrack_artists (dbtrack_id, dbartist_id, position) VALUES (?, ?, ?)",
					&dbtrackID,
					&artistID,
					&idx,
				)
			}
		}
	}
}

// AlbumSearchByTitle searches for albums by title.
func AlbumSearchByTitle(
	ctx context.Context,
	title string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	provider := getMusicBrainzProvider()
	results, _, err := provider.SearchReleases(ctx, title, limit, 0)
	return results, err
}

// ArtistGetMetadata retrieves metadata for a music artist from configured sources.
func ArtistGetMetadata(ctx context.Context, artist *database.Dbartist, overwrite bool) error {
	if artist.MusicbrainzID == "" {
		return nil
	}

	provider := getMusicBrainzProvider()

	details, err := provider.GetArtistByID(ctx, artist.MusicbrainzID)
	if err != nil {
		return err
	}

	applyArtistDetails(artist, details, overwrite)

	return nil
}

// applyArtistDetails applies API artist details to a database artist record.
func applyArtistDetails(
	artist *database.Dbartist,
	details *apiexternal_v2.ArtistDetails,
	overwrite bool,
) {
	if details == nil {
		return
	}

	UpdateString(&artist.Name, details.Name, overwrite, nil)
	UpdateString(&artist.SortName, details.SortName, overwrite, nil)
	UpdateString(&artist.MusicbrainzID, details.MusicBrainzID, overwrite, nil)
	UpdateString(&artist.DiscogsID, details.DiscogsID, overwrite, nil)
	UpdateString(&artist.ArtistType, details.Type, overwrite, nil)
	UpdateString(&artist.Country, details.Country, overwrite, nil)
	UpdateString(&artist.Disambiguation, details.Disambiguation, overwrite, nil)
	UpdateString(&artist.Bio, details.Bio, overwrite, nil)
	UpdateString(&artist.ImageURL, details.ImageURL, overwrite, nil)

	if len(details.Genres) > 0 {
		UpdateString(&artist.Genres, BuildCommaSeparatedString(details.Genres), overwrite, nil)
	}

	if !details.BeginDate.IsZero() {
		artist.BeginDate = details.BeginDate.Format("2006-01-02")
	}

	if !details.EndDate.IsZero() {
		artist.EndDate = details.EndDate.Format("2006-01-02")
	}

	// Add artist aliases to dbartist_aliases table
	if len(details.Aliases) == 0 || artist.ID == 0 {
		return
	}

	for _, aliasName := range details.Aliases {
		if aliasName == "" || aliasName == artist.Name {
			continue
		}

		// Check if alias already exists
		var existingAlias uint
		database.Scanrowsdyn(false,
			"SELECT id FROM dbartist_aliases WHERE dbartist_id = ? AND alias = ?",
			&existingAlias, &artist.ID, &aliasName)

		if existingAlias == 0 {
			// Insert new alias
			aliasSlug := logger.StringToSlug(aliasName)

			_, _ = database.ExecNid(
				"INSERT INTO dbartist_aliases (dbartist_id, alias, slug) VALUES (?, ?, ?)",
				&artist.ID, &aliasName, &aliasSlug,
			)
		}
	}
}

// ArtistSearchByName searches for artists by name.
func ArtistSearchByName(
	ctx context.Context,
	name string,
	limit int,
) ([]apiexternal_v2.ArtistSearchResult, error) {
	provider := getMusicBrainzProvider()
	return provider.SearchArtists(ctx, name, limit)
}

// TrackGetMetadata retrieves metadata for a music track from configured sources.
func TrackGetMetadata(ctx context.Context, track *database.Dbtrack, overwrite bool) error {
	if track.MusicbrainzRecordingID == "" && track.ISRC == "" {
		return nil
	}

	provider := getMusicBrainzProvider()

	var (
		details *apiexternal_v2.Track
		err     error
	)

	if track.MusicbrainzRecordingID != "" {
		details, err = provider.GetRecordingByID(ctx, track.MusicbrainzRecordingID)
	} else if track.ISRC != "" {
		details, err = provider.GetRecordingByISRC(ctx, track.ISRC)
	}

	if err != nil {
		return err
	}

	applyTrackDetails(track, details, overwrite)

	return nil
}

// applyTrackDetails applies API track details to a database track record.
func applyTrackDetails(track *database.Dbtrack, details *apiexternal_v2.Track, overwrite bool) {
	if details == nil {
		return
	}

	UpdateString(&track.Title, details.Title, overwrite, nil)
	UpdateString(&track.MusicbrainzRecordingID, details.MusicBrainzID, overwrite, nil)
	UpdateString(&track.ISRC, details.ISRC, overwrite, nil)
	UpdateString(&track.AcoustID, details.AcoustID, overwrite, nil)

	if details.DurationMs > 0 {
		UpdateInt64(&track.RuntimeMs, int64(details.DurationMs), overwrite)
	}
}

// -----------------------------------------------------------------------------
// Unified Metadata Functions
// -----------------------------------------------------------------------------

// GetMetadataForType retrieves metadata for any media type.
func GetMetadataForType(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	mediaType uint,
	id uint,
	overwrite bool,
) error {
	switch mediaType {
	case config.MediaTypeMovie:
		var movie database.Dbmovie
		if err := movie.GetDbmovieByIDP(&id); err != nil {
			return err
		}

		// Use existing movie metadata functions
		MovieGetMetadata(&movie, true, true, true, true)

		return nil

	case config.MediaTypeSeries:
		var serie database.Dbserie
		if err := serie.GetDbserieByIDP(&id); err != nil {
			return err
		}

		// Use existing series metadata functions
		SerieGetMetadata(&serie, "", true, true, overwrite, nil)

		return nil

	case config.MediaTypeBook:
		var book database.Dbbook
		if err := book.GetDbbookByIDP(&id); err != nil {
			return err
		}

		return BookGetMetadata(ctx, &book, overwrite)

	case config.MediaTypeAudiobook:
		var audiobook database.Dbaudiobook
		if err := audiobook.GetDbaudiobookByIDP(&id); err != nil {
			return err
		}

		return AudiobookGetMetadata(ctx, cfgp, &audiobook, overwrite)

	case config.MediaTypeMusic:
		var album database.Dbalbum
		if err := album.GetDbalbumByIDP(&id); err != nil {
			return err
		}

		return AlbumGetMetadata(ctx, &album, overwrite)
	}

	return nil
}

// -----------------------------------------------------------------------------
// Runtime Validation (shared across video and audio media)
// -----------------------------------------------------------------------------

// Common invalid runtime values (placeholders that should be skipped).
// Sorted for binary search.
var invalidRuntimesV2 = []int{1, 2, 3, 4, 60, 90, 120}

// IsValidRuntimeV2 checks if a runtime value is valid.
// Uses binary search on sorted slice for O(log n) performance.
func IsValidRuntimeV2(runtime int) bool {
	if runtime <= 0 {
		return false
	}

	_, found := slices.BinarySearch(invalidRuntimesV2, runtime)

	return !found
}

// ShouldUpdateRuntimeV2 determines if runtime should be updated.
func ShouldUpdateRuntimeV2(currentRuntime, newRuntime int, overwrite bool) bool {
	if newRuntime == 0 {
		return false
	}

	if overwrite && IsValidRuntimeV2(newRuntime) {
		return true
	}

	if !IsValidRuntimeV2(currentRuntime) && IsValidRuntimeV2(newRuntime) {
		return true
	}

	return currentRuntime == 0
}

// -----------------------------------------------------------------------------
// Cache Update Functions
// -----------------------------------------------------------------------------

// UpdateMediaCache updates the cache entry for a media item.
func UpdateMediaCache(mediaType uint, id uint, title, slug, identifier string, year int) {
	if !config.GetSettingsGeneral().UseMediaCache {
		return
	}

	switch mediaType {
	case config.MediaTypeMovie:
		database.AppendCacheThreeString(
			logger.CacheDBMovie,
			syncops.DbstaticThreeStringTwoInt{
				Str1: title,
				Str2: slug,
				Str3: identifier, // IMDB ID for movies
				Num1: year,
				Num2: id,
			},
		)

	case config.MediaTypeSeries:
		database.AppendCacheThreeString(
			logger.CacheDBSeries,
			syncops.DbstaticThreeStringTwoInt{
				Str1: title,
				Str2: slug,
				Str3: identifier, // IMDB ID for series
				Num1: year,
				Num2: id,
			},
		)

	case config.MediaTypeBook:
		// Cache for books using ISBN as identifier
		database.AppendCacheThreeString(
			"CacheDBBook",
			syncops.DbstaticThreeStringTwoInt{
				Str1: title,
				Str2: slug,
				Str3: identifier, // ISBN for books
				Num1: year,
				Num2: id,
			},
		)

	case config.MediaTypeAudiobook:
		// Cache for audiobooks using ASIN as identifier
		database.AppendCacheThreeString(
			"CacheDBAudiobook",
			syncops.DbstaticThreeStringTwoInt{
				Str1: title,
				Str2: slug,
				Str3: identifier, // ASIN for audiobooks
				Num1: year,
				Num2: id,
			},
		)

	case config.MediaTypeMusic:
		// Cache for albums using MusicBrainz ID as identifier
		database.AppendCacheThreeString(
			"CacheDBAlbum",
			syncops.DbstaticThreeStringTwoInt{
				Str1: title,
				Str2: slug,
				Str3: identifier, // MusicBrainz ID for albums
				Num1: year,
				Num2: id,
			},
		)
	}
}

package importfeed

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audnex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/goodreads"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// JobImportDBBook imports a book into the database and media lists from a ManualConfig.
// It supports three import modes based on which fields are set in the config:
//   - Full author: AuthorName/AuthorID set - imports all books by this author
//   - Book series: BookSeriesName/BookSeriesID set - imports all books in the series
//   - Single book: Only Name set - imports a single book
func JobImportDBBook(
	ctx context.Context, book *config.ManualConfig,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	// Determine import mode based on which fields are set
	var (
		importMode string
		importName string
	)

	if book.AuthorName != "" || book.AuthorID != "" {
		importMode = "author"

		importName = book.AuthorName
		if importName == "" {
			importName = book.AuthorID
		}
	} else if book.BookSeriesName != "" || book.BookSeriesID != "" {
		importMode = "book_series"

		importName = book.BookSeriesName
		if importName == "" {
			importName = book.BookSeriesID
		}
	} else {
		importMode = "single"
		importName = book.Name
	}

	logger.Logtype("info", 0).
		Str("book", importName).
		Str("mode", importMode).
		Int(logger.StrRow, idx).
		Msg("Import/Update Book")

	err := jobImportDBBook(ctx, book, cfgp, listid, importMode)
	recordImportResult(ctx, err, errBookIgnored, errJobRunning)

	return err
}

// jobImportDBBook performs the actual book import based on the import mode.
func jobImportDBBook(
	ctx context.Context,
	book *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	importMode string,
) error {
	// Use providers from registry
	olProvider := providers.GetOpenLibrary()
	grProvider := providers.GetGoodreads()

	switch importMode {
	case "author":
		return importBooksByAuthor(ctx, book, cfgp, listid, olProvider, grProvider)
	case "book_series":
		return importBooksBySeries(ctx, book, cfgp, listid, olProvider, grProvider)
	case "single":
		return importSingleBook(ctx, book, cfgp, listid, olProvider, grProvider)
	}

	return nil
}

// importBooksByAuthor imports all books by an author.
func importBooksByAuthor(
	ctx context.Context,
	book *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	olProvider *openlibrary.Provider,
	grProvider *goodreads.Provider,
) error {
	var (
		authorID   string
		authorName string
	)

	// Determine which ID to use

	if book.AuthorID != "" {
		authorID = book.AuthorID
	}

	if book.AuthorName != "" {
		authorName = book.AuthorName
	}

	// Try to get author's books from providers
	if grProvider != nil && authorID != "" {
		// Use Goodreads if we have an author ID
		books, err := grProvider.GetBooksByAuthor(ctx, authorID, 1)
		if err == nil && len(books) > 0 {
			for i := range books {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := addBookToDatabase(ctx, &books[i], cfgp, listid, authorName); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("book", books[i].Title).
						Msg("Failed to add book")
				}
			}

			return nil
		}
	}

	// Fallback to OpenLibrary search by author name
	if authorName != "" {
		books, err := olProvider.SearchBooks(ctx, "", authorName, 100)
		if err == nil {
			for i := range books {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := addBookToDatabase(ctx, &books[i], cfgp, listid, authorName); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("book", books[i].Title).
						Msg("Failed to add book")
				}
			}
		}

		return err
	}

	return logger.ErrNotFound
}

// importBooksBySeries imports all books in a series.
func importBooksBySeries(
	ctx context.Context,
	book *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	olProvider *openlibrary.Provider,
	grProvider *goodreads.Provider,
) error {
	var (
		seriesID   string
		seriesName string
	)

	if book.BookSeriesID != "" {
		seriesID = book.BookSeriesID
	}

	if book.BookSeriesName != "" {
		seriesName = book.BookSeriesName
	}

	// Try Goodreads series lookup
	if grProvider != nil && seriesID != "" {
		series, err := grProvider.GetSeriesByID(ctx, seriesID)
		if err == nil && series != nil {
			for i := range series.Books {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := addBookToDatabase(ctx, &series.Books[i], cfgp, listid, ""); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("book", series.Books[i].Title).
						Msg("Failed to add book")
				}
			}

			return nil
		}
	}

	// Fallback: search by series name
	if seriesName != "" {
		books, err := olProvider.SearchBooks(ctx, seriesName, "", 100)
		if err == nil {
			for i := range books {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := addBookToDatabase(ctx, &books[i], cfgp, listid, ""); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("book", books[i].Title).
						Msg("Failed to add book")
				}
			}
		}

		return err
	}

	return logger.ErrNotFound
}

// importSingleBook imports a single book by name or ID.
func importSingleBook(
	ctx context.Context,
	book *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	olProvider *openlibrary.Provider,
	grProvider *goodreads.Provider,
) error {
	// Search by name
	if book.Name != "" {
		books, err := olProvider.SearchBooks(ctx, book.Name, "", 10)
		if err == nil && len(books) > 0 {
			// Add only the first (best) match
			return addBookToDatabase(ctx, &books[0], cfgp, listid, "")
		}

		// Try Goodreads as fallback
		if grProvider != nil {
			grBooks, err := grProvider.SearchBooks(ctx, book.Name, 1)
			if err == nil && len(grBooks) > 0 {
				return addBookToDatabase(ctx, &grBooks[0], cfgp, listid, "")
			}
		}
	}

	return logger.ErrNotFound
}

// addBookToDatabase adds a book search result to the database.
func addBookToDatabase(
	_ context.Context,
	book *apiexternal_v2.BookSearchResult,
	cfgp *config.MediaTypeConfig,
	listid int,
	authorName string,
) error {
	if book == nil || book.Title == "" {
		return logger.ErrNotFound
	}

	// Check if book already exists by various IDs
	var existingID uint
	if book.ISBN13 != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbbooks WHERE isbn_13 = ?",
			&existingID,
			&book.ISBN13,
		)
	}

	if existingID == 0 && book.ISBN10 != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbbooks WHERE isbn_10 = ?",
			&existingID,
			&book.ISBN10,
		)
	}

	if existingID == 0 && book.ID != "" {
		// ID could be Goodreads ID or OpenLibrary ID depending on provider
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbbooks WHERE goodreads_id = ? OR openlibrary_id = ?",
			&existingID,
			&book.ID,
			&book.ID,
		)
	}

	var dbbookID uint

	slug := logger.StringToSlugCached(book.Title)
	year := uint16(book.PublishYear) //nolint:gosec // safe: value within target type range

	if existingID > 0 {
		dbbookID = existingID
		// Update existing book with any new metadata
		_, _ = database.ExecNid(
			`UPDATE dbbooks SET
				description = CASE WHEN description = '' OR description IS NULL THEN ? ELSE description END,
				cover_url = CASE WHEN cover_url = '' OR cover_url IS NULL THEN ? ELSE cover_url END,
				updated_at = current_timestamp
			 WHERE id = ?`,
			&book.Description, &book.CoverURL, &dbbookID,
		)
		logger.Logtype("debug", 1).
			Str("book", book.Title).
			Uint("id", dbbookID).
			Msg("Book already exists in database")
	} else {
		// Determine provider ID fields
		var goodreadsID, openlibraryID string
		switch book.ProviderType {
		case apiexternal_v2.ProviderGoodreads:
			goodreadsID = book.ID
		case apiexternal_v2.ProviderOpenLibrary:
			openlibraryID = book.ID
		default:
			// Other providers don't map to goodreads/openlibrary IDs
		}

		// Insert new dbbook with all available metadata
		result, err := database.ExecNid(
			`INSERT INTO dbbooks (title, isbn_13, isbn_10, slug, year, description, cover_url, goodreads_id, openlibrary_id, series_name, series_position)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			&book.Title,
			&book.ISBN13,
			&book.ISBN10,
			&slug,
			&year,
			&book.Description,
			&book.CoverURL,
			&goodreadsID,
			&openlibraryID,
			&book.SeriesName,
			&book.SeriesPosition,
		)
		if err != nil {
			return err
		}

		dbbookID = logger.Int64ToUint(result)
		logger.Logtype("info", 0).
			Str("book", book.Title).
			Uint("id", dbbookID).
			Msg("Added book to database")

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheThreeString(
				logger.CacheDBBook,
				syncops.DbstaticThreeStringTwoInt{
					Str1: book.Title,
					Str2: slug,
					Str3: book.ISBN13,
					Num1: int(year),
					Num2: dbbookID,
				},
			)
		}
	}

	// Add authors to the database and create relationships
	// First add authors from the book result
	for idx, author := range book.Authors {
		if author == "" {
			continue
		}

		authorID := addOrGetAuthor(author)
		if authorID > 0 {
			// Check if relationship already exists
			var existingRelation uint
			database.Scanrowsdyn(false,
				"SELECT id FROM dbbook_authors WHERE dbbook_id = ? AND dbauthor_id = ?",
				&existingRelation, &dbbookID, &authorID)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					`INSERT INTO dbbook_authors (dbbook_id, dbauthor_id, role, position)
					 VALUES (?, ?, 'author', ?)`,
					&dbbookID, &authorID, &idx,
				)
			}
		}
	}

	// Also add the authorName parameter if provided and not already in Authors list
	if authorName != "" {
		found := slices.Contains(book.Authors, authorName)

		if !found {
			authorID := addOrGetAuthor(authorName)
			if authorID > 0 {
				var existingRelation uint
				database.Scanrowsdyn(false,
					"SELECT id FROM dbbook_authors WHERE dbbook_id = ? AND dbauthor_id = ?",
					&existingRelation, &dbbookID, &authorID)

				if existingRelation == 0 {
					_, _ = database.ExecNid(
						`INSERT INTO dbbook_authors (dbbook_id, dbauthor_id, role, position)
						 VALUES (?, ?, 'author', ?)`,
						&dbbookID, &authorID, len(book.Authors),
					)
				}
			}
		}
	}

	// Add alternate title if subtitle exists
	if book.Subtitle != "" {
		fullTitle := book.Title + ": " + book.Subtitle
		fullSlug := logger.StringToSlugCached(fullTitle)

		var existingTitle uint
		database.Scanrowsdyn(false,
			"SELECT id FROM dbbook_titles WHERE dbbook_id = ? AND slug = ?",
			&existingTitle, &dbbookID, &fullSlug)

		if existingTitle == 0 {
			_, _ = database.ExecNid(
				`INSERT INTO dbbook_titles (dbbook_id, title, slug)
				 VALUES (?, ?, ?)`,
				&dbbookID, &fullTitle, &fullSlug,
			)
		}
	}

	// Get or create tracked authors for authors on the book.
	// For single-author: track with track_mode='books'.
	// For 2-3 author collaborations: primary gets 'books', secondary get 'none'.
	// For anthologies (4+ authors): skip creating new tracking entries entirely.
	var trackedAuthorID uint

	isMultiAuthor := len(book.Authors) > 1
	isCompilation := len(book.Authors) > 3
	listName := cfgp.Lists[listid].Name

	if !isCompilation {
		for idx, author := range book.Authors {
			if author == "" {
				continue
			}

			var dbauthorID uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbauthors WHERE name = ?",
				&dbauthorID,
				&author,
			)

			if dbauthorID == 0 {
				continue
			}

			// Check if tracked author already exists
			var existingTrackedID uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM authors WHERE dbauthor_id = ? AND listname = ?",
				&existingTrackedID,
				&dbauthorID,
				&listName,
			)

			// Determine if this is the primary author we want to track
			isPrimary := idx == 0 || !isMultiAuthor ||
				(authorName != "" && strings.EqualFold(author, authorName))

			trackMode := "books"
			if isMultiAuthor && !isPrimary {
				trackMode = "none"
			}

			if existingTrackedID == 0 {
				result, err := database.ExecNid(
					`INSERT INTO authors (dbauthor_id, listname, track_mode, dont_search)
					 VALUES (?, ?, ?, 0)`,
					&dbauthorID, &listName, &trackMode,
				)
				if err == nil {
					newID := logger.Int64ToUint(result)
					if isPrimary {
						trackedAuthorID = newID
					}

					logger.Logtype("debug", 1).
						Str("author", author).
						Str("list", listName).
						Str("track_mode", trackMode).
						Msg("Added author to tracking list")
				}
			} else if isPrimary {
				trackedAuthorID = existingTrackedID
			}
		}
	}

	// For anthologies or if authorName wasn't found above, find/create tracking entry for authorName only.
	if authorName != "" && trackedAuthorID == 0 {
		var dbauthorID uint
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbauthors WHERE name = ?",
			&dbauthorID,
			&authorName,
		)

		if dbauthorID > 0 {
			var existingTrackedID uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM authors WHERE dbauthor_id = ? AND listname = ?",
				&existingTrackedID,
				&dbauthorID,
				&listName,
			)

			if existingTrackedID == 0 {
				result, err := database.ExecNid(
					`INSERT INTO authors (dbauthor_id, listname, track_mode, dont_search)
					 VALUES (?, ?, 'books', 0)`,
					&dbauthorID, &listName,
				)
				if err == nil {
					trackedAuthorID = logger.Int64ToUint(result)
				}
			} else {
				trackedAuthorID = existingTrackedID
			}
		}
	}

	// Check if tracked book already exists for this list
	var trackedID uint
	database.Scanrowsdyn(
		false,
		"SELECT id FROM books WHERE dbbook_id = ? AND listname = ?",
		&trackedID,
		&dbbookID,
		&listName,
	)

	if trackedID == 0 {
		// Add to tracked books
		bookID, err := database.ExecNid(
			`INSERT INTO books (dbbook_id, listname, missing, quality_profile, author_id)
			 VALUES (?, ?, 1, ?, ?)`,
			&dbbookID, &listName, &cfgp.Lists[listid].CfgQuality.Name, &trackedAuthorID,
		)
		if err != nil {
			return err
		}

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				logger.CacheBook,
				syncops.DbstaticOneStringTwoInt{
					Str:  listName,
					Num1: dbbookID,
					Num2: logger.Int64ToUint(bookID),
				},
			)
		}

		logger.Logtype("info", 0).
			Str("book", book.Title).
			Str("list", listName).
			Msg("Added book to tracking list")
	} else if trackedAuthorID > 0 {
		// Update existing book to link to author if not already linked
		_, _ = database.ExecNid(
			`UPDATE books SET author_id = ? WHERE id = ? AND author_id = 0`,
			&trackedAuthorID, &trackedID,
		)
	}

	return nil
}

// JobImportDBAudiobook imports an audiobook into the database and media lists from a ManualConfig.
// It supports three import modes based on which fields are set in the config:
//   - Full author: AuthorName/AuthorID set - imports all audiobooks by this author
//   - Book series: BookSeriesName/BookSeriesID set - imports all audiobooks in the series
//   - Single audiobook: Only Name set - imports a single audiobook
func JobImportDBAudiobook(
	ctx context.Context, audiobook *config.ManualConfig,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	// Determine import mode based on which fields are set
	var (
		importMode string
		importName string
	)

	if audiobook.AuthorName != "" || audiobook.AuthorID != "" {
		importMode = "author"

		importName = audiobook.AuthorName
		if importName == "" {
			importName = audiobook.AuthorID
		}
	} else if audiobook.BookSeriesName != "" || audiobook.BookSeriesID != "" {
		importMode = "book_series"

		importName = audiobook.BookSeriesName
		if importName == "" {
			importName = audiobook.BookSeriesID
		}
	} else {
		importMode = "single"
		importName = audiobook.Name
	}

	logger.Logtype("info", 0).
		Str("audiobook", importName).
		Str("mode", importMode).
		Int(logger.StrRow, idx).
		Msg("Import/Update Audiobook")

	err := jobImportDBAudiobook(ctx, audiobook, cfgp, listid, importMode)
	recordImportResult(ctx, err, errAudiobookIgnored, errJobRunning)

	return err
}

// getOrCreateAudibleProvider returns an Audible provider for the specified region.
// It creates and registers a new provider if one doesn't exist for the region.
func getOrCreateAudibleProvider(region string) *audible.Provider {
	if region == "" {
		region = "us"
	}

	// Try to get existing provider
	provider := providers.GetAudible(region)
	if provider != nil {
		return provider
	}

	// Create new provider for this region
	provider = audible.NewProviderWithConfig(base.ClientConfig{
		Timeout:          30 * time.Second,
		RateLimitCalls:   5,
		RateLimitSeconds: 1,
		UserAgent:        config.GetSettingsGeneral().UserAgent,
	}, audible.Region(region))

	providers.SetAudible(region, provider)
	logger.Logtype(logger.StatusDebug, 0).
		Str("region", region).
		Msg("Created and registered Audible provider for region")

	return provider
}

// jobImportDBAudiobook performs the actual audiobook import based on the import mode.
func jobImportDBAudiobook(
	ctx context.Context,
	audiobook *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	importMode string,
) error {
	// Get Audible provider with region from config
	audibleProvider := getOrCreateAudibleProvider(cfgp.AudibleRegion)

	// Use providers from registry
	audnexProvider := providers.GetAudnex()
	olProvider := providers.GetOpenLibrary()
	grProvider := providers.GetGoodreads()

	switch importMode {
	case "author":
		return importAudiobooksByAuthor(
			ctx,
			audiobook,
			cfgp,
			listid,
			audibleProvider,
			audnexProvider,
			olProvider,
			grProvider,
		)

	case "book_series":
		return importAudiobooksBySeries(
			ctx,
			audiobook,
			cfgp,
			listid,
			audibleProvider,
			audnexProvider,
			olProvider,
			grProvider,
		)

	case "single":
		return importSingleAudiobook(
			ctx,
			audiobook,
			cfgp,
			listid,
			audibleProvider,
			audnexProvider,
			olProvider,
			grProvider,
		)
	}

	return nil
}

// importAudiobooksByAuthor imports all audiobooks by an author.
// Progressive data collection strategy:
// 1. Search Audible for audiobooks by author (unless circuit is open)
// 2. Fallback to Audnex if Audible fails or circuit is open
// 3. For each result, build a complete record by querying multiple providers
// 4. Save the merged record to database.
func importAudiobooksByAuthor(
	ctx context.Context,
	audiobook *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	audibleProvider *audible.Provider,
	audnexProvider *audnex.Provider,
	olProvider *openlibrary.Provider,
	grProvider *goodreads.Provider,
) error {
	authorID := audiobook.AuthorID

	authorName := audiobook.AuthorName
	if authorName == "" {
		authorName = authorID
	}

	// Check if Audible circuit is open - if so, try Audnex first
	audibleAvailable := audibleProvider != nil && audibleProvider.CheckFreeNonBlocking() == nil

	// Try Audnex first if Audible circuit is open and we have an author ASIN
	if !audibleAvailable && audnexProvider != nil && authorID != "" {
		audiobooks, err := audnexProvider.GetAuthorBooks(ctx, authorID)
		if err == nil && len(audiobooks) > 0 {
			logger.Logtype("debug", 0).
				Str("author", authorName).
				Msg("Using Audnex fallback (Audible circuit open)")

			for i := range audiobooks {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := importAndMergeAudiobook(
					ctx,
					audiobooks[i].ASIN,
					&audiobooks[i],
					audnexProvider,
					grProvider,
					cfgp,
					listid,
				); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("audiobook", audiobooks[i].Title).
						Str("asin", audiobooks[i].ASIN).
						Msg("Failed to import audiobook")
				}
			}

			return nil
		}
	}

	// Try Audible - primary source for audiobooks with proper metadata
	if audibleAvailable && authorName != "" {
		audiobooks, err := audibleProvider.SearchByAuthor(ctx, authorName, 100)
		if err == nil && len(audiobooks) > 0 {
			for i := range audiobooks {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				// Build complete audiobook record - pass search result to avoid redundant API calls
				if err := importAndMergeAudiobook(
					ctx,
					audiobooks[i].ASIN,
					&audiobooks[i],
					audnexProvider,
					grProvider,
					cfgp,
					listid,
				); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("audiobook", audiobooks[i].Title).
						Str("asin", audiobooks[i].ASIN).
						Msg("Failed to import audiobook")
				}
			}

			return nil
		}

		// Audible query failed - try Audnex as fallback
		if err != nil {
			logger.Logtype("debug", 0).
				Err(err).
				Str("author", authorName).
				Msg("Audible query failed, trying Audnex fallback")
		}
	}

	// Fallback to Audnex if we have an author ASIN (provides chapter data)
	if audnexProvider != nil && authorID != "" {
		audiobooks, err := audnexProvider.GetAuthorBooks(ctx, authorID)
		if err == nil && len(audiobooks) > 0 {
			for i := range audiobooks {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				// Build complete audiobook record - pass search result to avoid redundant API calls
				if err := importAndMergeAudiobook(
					ctx,
					audiobooks[i].ASIN,
					&audiobooks[i],
					audnexProvider,
					grProvider,
					cfgp,
					listid,
				); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("audiobook", audiobooks[i].Title).
						Str("asin", audiobooks[i].ASIN).
						Msg("Failed to import audiobook")
				}
			}

			return nil
		}
	}

	// Last resort: OpenLibrary search by author name (no audiobook-specific data)
	if olProvider != nil && authorName != "" {
		books, err := olProvider.SearchBooks(ctx, "", authorName, 100)
		if err == nil {
			for i := range books {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := addAudiobookFromBookToDatabase(ctx, &books[i], cfgp, listid); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("audiobook", books[i].Title).
						Msg("Failed to add book as audiobook")
				}
			}
		}

		return err
	}

	return logger.ErrNotFound
}

// importAndMergeAudiobook progressively builds a complete audiobook record by querying multiple providers.
// This follows the strategy: use initial data from search, get chapters from Audnex, merge, then save once.
// The initial parameter allows passing data from search results to avoid redundant API calls.
func importAndMergeAudiobook(
	ctx context.Context,
	asin string,
	initial *apiexternal_v2.AudiobookSearchResult, // Initial data from search (optional)
	audnexProvider *audnex.Provider,
	_ *goodreads.Provider,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if asin == "" {
		return logger.ErrNotFound
	}

	// Initialize merged record with ASIN
	merged := &apiexternal_v2.AudiobookDetails{
		ASIN: asin,
		ID:   asin,
	}

	var chapters []apiexternal_v2.AudiobookChapter

	// Step 1: Use initial data from search result if provided
	// This avoids redundant Audible API calls that can get rate limited
	if initial != nil {
		merged.Title = initial.Title
		merged.Subtitle = initial.Subtitle
		merged.Authors = initial.Authors
		merged.Narrators = initial.Narrators
		merged.CoverURL = initial.CoverURL
		merged.Description = initial.Description
		merged.Series = initial.Series
		merged.SeriesName = initial.SeriesName
		merged.SeriesPosition = initial.SeriesPosition
		merged.Duration = initial.Duration
		merged.RuntimeMinutes = initial.RuntimeMinutes
		merged.ReleaseDate = initial.ReleaseDate
		merged.ReleaseYear = initial.ReleaseYear
		logger.Logtype("debug", 1).
			Str("asin", asin).
			Str("title", merged.Title).
			Msg("Using initial search data")
	}

	// Step 2: Get chapters from Audnex (only source for chapter data)
	if audnexProvider != nil {
		audnexChapters, err := audnexProvider.GetChaptersByASIN(ctx, asin, cfgp.AudibleRegion)
		if err == nil && len(audnexChapters) > 0 {
			chapters = audnexChapters
			logger.Logtype("debug", 1).
				Str("asin", asin).
				Int("chapters", len(chapters)).
				Msg("Got chapters from Audnex")
		}

		// Get additional metadata from Audnex (description, publisher, language, etc.)
		// Audnex provides richer description and additional fields that search results don't have
		audnexDetails, err := audnexProvider.GetBookByASIN(ctx, asin, cfgp.AudibleRegion)
		if err == nil && audnexDetails != nil {
			mergeAudiobookDetails(merged, audnexDetails)
			logger.Logtype("debug", 1).Str("asin", asin).Msg("Merged Audnex data")
		}
	}

	// Step 3: Try Goodreads for additional metadata (ratings, descriptions)
	// Note: Goodreads lookup would require ISBN or Goodreads ID, which we might not have
	// This is a placeholder for future enhancement

	// Validation: Must have at least title to proceed
	if merged.Title == "" {
		return logger.ErrNotFound
	}

	// Step 4: Save the complete merged record to database
	return addAudiobookDetailToDatabase(ctx, merged, chapters, cfgp, listid)
}

// mergeAudiobookDetails merges non-empty fields from source into target.
// Only overwrites empty fields in target - never replaces existing data.
func mergeAudiobookDetails(target, source *apiexternal_v2.AudiobookDetails) {
	if source == nil {
		return
	}

	// Core identifiers
	if target.ID == "" && source.ID != "" {
		target.ID = source.ID
	}

	if target.ASIN == "" && source.ASIN != "" {
		target.ASIN = source.ASIN
	}

	// Title and description
	if target.Title == "" && source.Title != "" {
		target.Title = source.Title
	}

	if target.Subtitle == "" && source.Subtitle != "" {
		target.Subtitle = source.Subtitle
	}

	if target.Description == "" && source.Description != "" {
		target.Description = source.Description
	}

	if target.Summary == "" && source.Summary != "" {
		target.Summary = source.Summary
	}

	// Authors and narrators
	if len(target.Authors) == 0 && len(source.Authors) > 0 {
		target.Authors = source.Authors
		target.AuthorIDs = source.AuthorIDs
	}

	if len(target.Narrators) == 0 && len(source.Narrators) > 0 {
		target.Narrators = source.Narrators
		target.NarratorIDs = source.NarratorIDs
	}

	// Series information
	if target.Series == "" && source.Series != "" {
		target.Series = source.Series
	}

	if target.SeriesASIN == "" && source.SeriesASIN != "" {
		target.SeriesASIN = source.SeriesASIN
	}

	if target.SeriesPosition == "" && source.SeriesPosition != "" {
		target.SeriesPosition = source.SeriesPosition
	}

	// Duration and runtime
	if target.Duration == 0 && source.Duration > 0 {
		target.Duration = source.Duration
	}

	if target.RuntimeMinutes == 0 && source.RuntimeMinutes > 0 {
		target.RuntimeMinutes = source.RuntimeMinutes
	}

	// Publisher info
	if target.Publisher == "" && source.Publisher != "" {
		target.Publisher = source.Publisher
	}

	if target.Language == "" && source.Language != "" {
		target.Language = source.Language
	}

	// Release date
	if target.ReleaseDate.IsZero() && !source.ReleaseDate.IsZero() {
		target.ReleaseDate = source.ReleaseDate
	}

	if target.ReleaseYear == 0 && source.ReleaseYear > 0 {
		target.ReleaseYear = source.ReleaseYear
	}

	// Cover image
	if target.CoverURL == "" && source.CoverURL != "" {
		target.CoverURL = source.CoverURL
	}

	// Genres and categories
	if len(target.Genres) == 0 && len(source.Genres) > 0 {
		target.Genres = source.Genres
	}

	if len(target.Categories) == 0 && len(source.Categories) > 0 {
		target.Categories = source.Categories
	}

	// Ratings
	if target.Rating == 0 && source.Rating > 0 {
		target.Rating = source.Rating
		target.RatingCount = source.RatingCount
	}

	if target.AverageRating == 0 && source.AverageRating > 0 {
		target.AverageRating = source.AverageRating
		target.RatingsCount = source.RatingsCount
	}

	// ISBN
	if target.ISBN == "" && source.ISBN != "" {
		target.ISBN = source.ISBN
	}

	// Chapters (prefer source if it has more chapters)
	if len(target.Chapters) < len(source.Chapters) {
		target.Chapters = source.Chapters
	}

	// Provider type (use source if target not set)
	if target.ProviderType == "" && source.ProviderType != "" {
		target.ProviderType = source.ProviderType
	}
}

// importAudiobooksBySeries imports all audiobooks in a series.
func importAudiobooksBySeries(
	ctx context.Context,
	audiobook *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	_ *audible.Provider,
	audnexProvider *audnex.Provider,
	olProvider *openlibrary.Provider,
	_ *goodreads.Provider,
) error {
	seriesID := audiobook.BookSeriesID

	seriesName := audiobook.BookSeriesName
	if seriesName == "" {
		seriesName = seriesID
	}

	// Try Audnex first if we have a series ASIN
	if audnexProvider != nil && seriesID != "" {
		// series, err := audnexProvider.(ctx, seriesID)
		// if err == nil && series != nil && len(series.Books) > 0 {
		// 	for _, b := range series.Books {
		// 		if err := logger.CheckContextEnded(ctx); err != nil {
		// 			return err
		// 		}
		// 		// Try using ISBN13 as ASIN (Audible uses ISBN for some books)
		// 		asin := b.ISBN13
		// 		if asin == "" {
		// 			asin = b.ISBN10
		// 		}
		// 		if asin != "" {
		// 			// Build complete audiobook record from multiple providers
		// 			// No initial search result available - will fetch from Audnex
		// 			if err := importAndMergeAudiobook(ctx, asin, nil, audnexProvider, grProvider, cfgp, listid); err != nil {
		// 				logger.Logtype("error", 1).Err(err).Str("audiobook", b.Title).Str("asin", asin).Msg("Failed to import audiobook")
		// 			}
		// 		} else {
		// 			// Fallback to book data if no ASIN/ISBN available
		// 			if err := addAudiobookFromBookToDatabase(ctx, &b, cfgp, listid); err != nil {
		// 				logger.Logtype("error", 1).Err(err).Str("audiobook", b.Title).Msg("Failed to add audiobook")
		// 			}
		// 		}
		// 	}
		// 	return nil
		// }
	}

	// Fallback to OpenLibrary search
	if seriesName != "" {
		books, err := olProvider.SearchBooks(ctx, seriesName, "", 100)
		if err == nil {
			for i := range books {
				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if err := addAudiobookFromBookToDatabase(ctx, &books[i], cfgp, listid); err != nil {
					logger.Logtype("error", 1).
						Err(err).
						Str("audiobook", books[i].Title).
						Msg("Failed to add audiobook")
				}
			}
		}

		return err
	}

	return logger.ErrNotFound
}

// importSingleAudiobook imports a single audiobook by name.
// Falls back to OpenLibrary if Audible circuit is open or query fails.
func importSingleAudiobook(
	ctx context.Context,
	audiobook *config.ManualConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	audibleProvider *audible.Provider,
	audnexProvider *audnex.Provider,
	olProvider *openlibrary.Provider,
	grProvider *goodreads.Provider,
) error {
	// Check if Audible circuit is open
	audibleAvailable := audibleProvider != nil && audibleProvider.CheckFreeNonBlocking() == nil

	// If Audible circuit is open, skip directly to OpenLibrary fallback
	if !audibleAvailable {
		logger.Logtype("debug", 0).
			Str("name", audiobook.Name).
			Msg("Audible circuit open, using OpenLibrary fallback")
	}

	// Try Audible search first (if circuit is not open)
	if audibleAvailable && audiobook.Name != "" {
		audiobooks, err := audibleProvider.SearchAudiobooks(ctx, audiobook.Name, 10)
		if err == nil && len(audiobooks) > 0 {
			// Build complete audiobook record - pass search result to avoid redundant API calls
			return importAndMergeAudiobook(
				ctx,
				audiobooks[0].ASIN,
				&audiobooks[0],
				audnexProvider,
				grProvider,
				cfgp,
				listid,
			)
		}

		if err != nil {
			logger.Logtype("debug", 0).
				Err(err).
				Str("name", audiobook.Name).
				Msg("Audible query failed, trying OpenLibrary fallback")
		}
	}

	// Fallback to OpenLibrary search
	if olProvider != nil && audiobook.Name != "" {
		books, err := olProvider.SearchBooks(ctx, audiobook.Name, "", 10)
		if err == nil && len(books) > 0 {
			return addAudiobookFromBookToDatabase(ctx, &books[0], cfgp, listid)
		}
	}

	return logger.ErrNotFound
}

// addOrGetAuthor finds an existing author by name or creates a new one.
// Returns the author ID.
func addOrGetAuthor(authorName string) uint {
	if authorName == "" {
		return 0
	}

	// Check if author already exists
	var authorID uint

	authorSlug := logger.StringToSlugCached(authorName)
	database.Scanrowsdyn(
		false,
		"SELECT id FROM dbauthors WHERE name = ? COLLATE NOCASE OR slug = ?",
		&authorID,
		&authorName,
		&authorSlug,
	)

	if authorID > 0 {
		return authorID
	}

	// Insert new author
	result, err := database.ExecNid(
		`INSERT INTO dbauthors (name, slug) VALUES (?, ?)`,
		&authorName,
		&authorSlug,
	)
	if err != nil {
		logger.Logtype("error", 1).Err(err).Str("author", authorName).Msg("Failed to insert author")
		return 0
	}

	return logger.Int64ToUint(result)
}

// addOrGetNarrator finds an existing narrator by name or creates a new one.
// Returns the narrator ID.
func addOrGetNarrator(narratorName string) uint {
	if narratorName == "" {
		return 0
	}

	// Check if narrator already exists
	var narratorID uint
	database.Scanrowsdyn(
		false,
		"SELECT id FROM dbnarrators WHERE name = ?",
		&narratorID,
		&narratorName,
	)

	if narratorID > 0 {
		return narratorID
	}

	// Insert new narrator
	result, err := database.ExecNid(
		`INSERT INTO dbnarrators (name) VALUES (?)`,
		&narratorName,
	)
	if err != nil {
		logger.Logtype("error", 1).
			Err(err).
			Str("narrator", narratorName).
			Msg("Failed to insert narrator")

		return 0
	}

	return logger.Int64ToUint(result)
}

// addAudiobookDetailToDatabase adds an audiobook with full details (from Audnex/Audible) to the database.
// This includes chapters, narrators, runtime, and other audiobook-specific metadata.
func addAudiobookDetailToDatabase(
	_ context.Context,
	details *apiexternal_v2.AudiobookDetails,
	chapters []apiexternal_v2.AudiobookChapter,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if details == nil || details.Title == "" {
		return logger.ErrNotFound
	}

	// Language filtering - check if audiobook language matches configured languages
	if details.Language != "" && cfgp.MetadataTitleLanguagesLen > 0 {
		languageAllowed := false

		for i := range cfgp.MetadataTitleLanguages {
			if strings.EqualFold(details.Language, cfgp.MetadataTitleLanguages[i]) ||
				logger.HasPrefixI(details.Language, cfgp.MetadataTitleLanguages[i]+"-") {
				languageAllowed = true
				break
			}
		}

		if !languageAllowed {
			logger.Logtype("debug", 1).
				Str("audiobook", details.Title).
				Str("language", details.Language).
				Msg("Skipping audiobook - language not in MetadataTitleLanguages")

			return nil
		}
	}

	// Check if audiobook already exists by various IDs
	var existingID uint
	if details.ASIN != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbaudiobooks WHERE asin = ?",
			&existingID,
			&details.ASIN,
		)
	}

	if existingID == 0 && details.ID != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbaudiobooks WHERE audible_id = ?",
			&existingID,
			&details.ID,
		)
	}

	var dbaudiobookID uint

	slug := logger.StringToSlugCached(details.Title)
	year := uint16(details.ReleaseYear) //nolint:gosec // safe: value within target type range

	if existingID > 0 {
		dbaudiobookID = existingID
		// Update existing audiobook with any new metadata
		_, _ = database.ExecNid(
			`UPDATE dbaudiobooks SET
				description = CASE WHEN description = '' OR description IS NULL THEN ? ELSE description END,
				cover_url = CASE WHEN cover_url = '' OR cover_url IS NULL THEN ? ELSE cover_url END,
				runtime_minutes = CASE WHEN runtime_minutes = 0 THEN ? ELSE runtime_minutes END,
				publisher = CASE WHEN publisher = '' OR publisher IS NULL THEN ? ELSE publisher END,
				language = CASE WHEN language = '' OR language IS NULL THEN ? ELSE language END,
				chapter_count = CASE WHEN chapter_count = 0 THEN ? ELSE chapter_count END,
				series_name = CASE WHEN series_name = '' OR series_name IS NULL THEN ? ELSE series_name END,
				series_position = CASE WHEN series_position = '' OR series_position IS NULL THEN ? ELSE series_position END,
				updated_at = current_timestamp
			 WHERE id = ?`,
			&details.Description, &details.CoverURL, &details.RuntimeMinutes,
			&details.Publisher, &details.Language, len(chapters),
			&details.SeriesName, &details.SeriesPosition, &dbaudiobookID,
		)
		logger.Logtype("debug", 1).
			Str("audiobook", details.Title).
			Uint("id", dbaudiobookID).
			Msg("Audiobook already exists in database")
	} else {
		// Insert new dbaudiobook with all available metadata
		result, err := database.ExecNid(
			`INSERT INTO dbaudiobooks (title, asin, audible_id, slug, year, description, cover_url, language,
			 runtime_minutes, chapter_count, publisher, average_rating, ratings_count, series_name, series_position)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			&details.Title,
			&details.ASIN,
			&details.ID,
			&slug,
			&year,
			&details.Description,
			&details.CoverURL,
			&details.Language,
			&details.RuntimeMinutes,
			len(chapters),
			&details.Publisher,
			&details.AverageRating,
			&details.RatingsCount,
			&details.SeriesName,
			&details.SeriesPosition,
		)
		if err != nil {
			return err
		}

		dbaudiobookID = logger.Int64ToUint(result)
		logger.Logtype("info", 0).
			Str("audiobook", details.Title).
			Uint("id", dbaudiobookID).
			Msg("Added audiobook to database")

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheThreeString(
				logger.CacheDBAudiobook,
				syncops.DbstaticThreeStringTwoInt{
					Str1: details.Title,
					Str2: slug,
					Str3: details.ASIN,
					Num1: int(year),
					Num2: dbaudiobookID,
				},
			)
		}
	}

	// Add authors to the database and create relationships
	for idx, authorName := range details.Authors {
		if authorName == "" {
			continue
		}

		authorID := addOrGetAuthor(authorName)
		if authorID > 0 {
			var existingRelation uint
			database.Scanrowsdyn(false,
				"SELECT id FROM dbaudiobook_authors WHERE dbaudiobook_id = ? AND dbauthor_id = ?",
				&existingRelation, &dbaudiobookID, &authorID)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					`INSERT INTO dbaudiobook_authors (dbaudiobook_id, dbauthor_id, role, position)
					 VALUES (?, ?, 'author', ?)`,
					&dbaudiobookID, &authorID, &idx,
				)
			}
		}
	}

	// Add narrators to the database and create relationships
	for idx, narratorName := range details.Narrators {
		if narratorName == "" {
			continue
		}

		narratorID := addOrGetNarrator(narratorName)
		if narratorID > 0 {
			var existingRelation uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbaudiobook_narrators WHERE dbaudiobook_id = ? AND dbnarrator_id = ?",
				&existingRelation,
				&dbaudiobookID,
				&narratorID,
			)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					`INSERT INTO dbaudiobook_narrators (dbaudiobook_id, dbnarrator_id, position)
					 VALUES (?, ?, ?)`,
					&dbaudiobookID, &narratorID, &idx,
				)
			}
		}
	}

	// Add chapters to the database
	for i := range chapters {
		var existingChapter uint
		database.Scanrowsdyn(false,
			"SELECT id FROM dbaudiobook_chapters WHERE dbaudiobook_id = ? AND chapter_number = ?",
			&existingChapter, &dbaudiobookID, &chapters[i].ChapterNumber)

		if existingChapter == 0 {
			// Calculate end_time_ms as start_time_ms + runtime_ms
			endTimeMs := chapters[i].StartOffsetMs + chapters[i].LengthMs

			_, _ = database.ExecNid(
				`INSERT INTO dbaudiobook_chapters (dbaudiobook_id, title, chapter_number, position, start_time_ms, end_time_ms, runtime_ms)
				 VALUES (?, ?, ?, ?, ?, ?, ?)`,
				&dbaudiobookID,
				&chapters[i].Title,
				&chapters[i].ChapterNumber,
				&chapters[i].Number,
				&chapters[i].StartOffsetMs,
				&endTimeMs,
				&chapters[i].LengthMs,
			)
		}
	}

	// Add main title to titles table
	mainSlug := logger.StringToSlugCached(details.Title)

	var existingMainTitle uint
	database.Scanrowsdyn(false,
		"SELECT id FROM dbaudiobook_titles WHERE dbaudiobook_id = ? AND slug = ?",
		&existingMainTitle, &dbaudiobookID, &mainSlug)

	if existingMainTitle == 0 {
		_, _ = database.ExecNid(
			`INSERT INTO dbaudiobook_titles (dbaudiobook_id, title, slug)
			 VALUES (?, ?, ?)`,
			&dbaudiobookID, &details.Title, &mainSlug,
		)
	}

	// Add alternate title if subtitle exists
	if details.Subtitle != "" {
		fullTitle := details.Title + ": " + details.Subtitle
		fullSlug := logger.StringToSlugCached(fullTitle)

		var existingTitle uint
		database.Scanrowsdyn(false,
			"SELECT id FROM dbaudiobook_titles WHERE dbaudiobook_id = ? AND slug = ?",
			&existingTitle, &dbaudiobookID, &fullSlug)

		if existingTitle == 0 {
			_, _ = database.ExecNid(
				`INSERT INTO dbaudiobook_titles (dbaudiobook_id, title, slug)
				 VALUES (?, ?, ?)`,
				&dbaudiobookID, &fullTitle, &fullSlug,
			)
		}
	}

	// Get or create tracked authors for authors on the audiobook.
	// For single-author: track with track_mode='audiobooks'.
	// For 2-3 author collaborations: primary gets 'audiobooks', secondary get 'none'.
	// For anthologies (4+ authors): skip creating new tracking entries entirely.
	var trackedAuthorID uint

	isMultiAuthor := len(details.Authors) > 1
	isCompilation := len(details.Authors) > 3
	listName2 := cfgp.Lists[listid].Name

	if !isCompilation {
		for idx, authorName := range details.Authors {
			if authorName == "" {
				continue
			}

			var dbauthorID uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM dbauthors WHERE name = ?",
				&dbauthorID,
				&authorName,
			)

			if dbauthorID == 0 {
				continue
			}

			// Check if tracked author already exists
			var existingTrackedID uint
			database.Scanrowsdyn(
				false,
				"SELECT id FROM authors WHERE dbauthor_id = ? AND listname = ?",
				&existingTrackedID,
				&dbauthorID,
				&listName2,
			)

			// Determine if this is the primary author we want to track
			isPrimary := idx == 0 || !isMultiAuthor

			trackMode := "audiobooks"
			if isMultiAuthor && !isPrimary {
				trackMode = "none"
			}

			if existingTrackedID == 0 {
				result, err := database.ExecNid(
					`INSERT INTO authors (dbauthor_id, listname, track_mode, dont_search)
					 VALUES (?, ?, ?, 0)`,
					&dbauthorID, &listName2, &trackMode,
				)
				if err == nil {
					newID := logger.Int64ToUint(result)
					if isPrimary {
						trackedAuthorID = newID
					}

					logger.Logtype("debug", 1).
						Str("author", authorName).
						Str("list", listName2).
						Str("track_mode", trackMode).
						Msg("Added author to tracking list")
				}
			} else if isPrimary {
				trackedAuthorID = existingTrackedID
			}
		}
	}

	// For anthologies or if primary wasn't found, find existing tracking entry for first author
	if trackedAuthorID == 0 && len(details.Authors) > 0 && details.Authors[0] != "" {
		var dbauthorID uint
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbauthors WHERE name = ?",
			&dbauthorID,
			&details.Authors[0],
		)

		if dbauthorID > 0 {
			database.Scanrowsdyn(
				false,
				"SELECT id FROM authors WHERE dbauthor_id = ? AND listname = ?",
				&trackedAuthorID,
				&dbauthorID,
				&listName2,
			)
		}
	}

	// Check if tracked audiobook already exists
	listName := cfgp.Lists[listid].Name

	var trackedID uint
	database.Scanrowsdyn(
		false,
		"SELECT id FROM audiobooks WHERE dbaudiobook_id = ? AND listname = ?",
		&trackedID,
		&dbaudiobookID,
		&listName,
	)

	if trackedID == 0 {
		// Note: book_series_id will remain 0 for now until we implement series tracking
		audiobookID, err := database.ExecNid(
			`INSERT INTO audiobooks (dbaudiobook_id, listname, missing, quality_profile, author_id, book_series_id)
			 VALUES (?, ?, 1, ?, ?, 0)`,
			&dbaudiobookID,
			&listName,
			&cfgp.Lists[listid].CfgQuality.Name,
			&trackedAuthorID,
		)
		if err != nil {
			return err
		}

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				logger.CacheAudiobook,
				syncops.DbstaticOneStringTwoInt{
					Str:  listName,
					Num1: dbaudiobookID,
					Num2: logger.Int64ToUint(audiobookID),
				},
			)
		}

		logger.Logtype("info", 0).
			Str("audiobook", details.Title).
			Str("list", listName).
			Msg("Added audiobook to tracking list")
	} else if trackedAuthorID > 0 {
		// Update existing audiobook to link to author if not already linked
		_, _ = database.ExecNid(
			`UPDATE audiobooks SET author_id = ? WHERE id = ? AND author_id = 0`,
			&trackedAuthorID, &trackedID,
		)
	}

	return nil
}

// addAudiobookFromBookToDatabase adds an audiobook from a book search result (from OpenLibrary/Goodreads).
// This is a fallback when audiobook-specific metadata is not available.
// Note: Language filtering not applied here since BookSearchResult doesn't include language data.
// Language filtering should be done when using Audible/Audnex providers which have proper language metadata.
func addAudiobookFromBookToDatabase(
	_ context.Context,
	book *apiexternal_v2.BookSearchResult,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if book == nil || book.Title == "" {
		return logger.ErrNotFound
	}

	// Check if audiobook already exists by various IDs
	var existingID uint
	if book.ISBN13 != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbaudiobooks WHERE asin = ?",
			&existingID,
			&book.ISBN13,
		)
	}

	if existingID == 0 && book.ISBN10 != "" {
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbaudiobooks WHERE asin = ?",
			&existingID,
			&book.ISBN10,
		)
	}

	// Don't check book.ID for audible_id since it's not an actual Audible ASIN (it's an OpenLibrary work ID)

	var dbaudiobookID uint

	slug := logger.StringToSlugCached(book.Title)
	year := uint16(book.PublishYear) //nolint:gosec // safe: value within target type range

	if existingID > 0 {
		dbaudiobookID = existingID
		_, _ = database.ExecNid(
			`UPDATE dbaudiobooks SET
				description = CASE WHEN description = '' OR description IS NULL THEN ? ELSE description END,
				cover_url = CASE WHEN cover_url = '' OR cover_url IS NULL THEN ? ELSE cover_url END,
				updated_at = current_timestamp
			 WHERE id = ?`,
			&book.Description, &book.CoverURL, &dbaudiobookID,
		)
		logger.Logtype("debug", 1).
			Str("audiobook", book.Title).
			Uint("id", dbaudiobookID).
			Msg("Audiobook already exists in database")
	} else {
		// Don't store OpenLibrary work IDs (starting with "/works/") or OpenLibrary IDs (starting with "OL") in audible_id field
		// The audible_id field should only contain actual Audible ASINs (which typically start with "B0")
		var audibleID *string
		if book.ID != "" && !strings.Contains(book.ID, "/works/") &&
			!strings.HasPrefix(book.ID, "OL") {
			// This might be an actual ASIN, store it
			audibleID = &book.ID
		}

		// Default language to empty string since BookSearchResult doesn't provide language data
		language := ""

		result, err := database.ExecNid(
			`INSERT INTO dbaudiobooks (title, asin, audible_id, slug, year, description, cover_url, language)
			 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			&book.Title,
			&book.ISBN13,
			audibleID,
			&slug,
			&year,
			&book.Description,
			&book.CoverURL,
			&language,
		)
		if err != nil {
			return err
		}

		dbaudiobookID = logger.Int64ToUint(result)
		logger.Logtype("info", 0).
			Str("audiobook", book.Title).
			Uint("id", dbaudiobookID).
			Msg("Added audiobook to database")

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheThreeString(
				logger.CacheDBAudiobook,
				syncops.DbstaticThreeStringTwoInt{
					Str1: book.Title,
					Str2: slug,
					Str3: book.ISBN13,
					Num1: int(year),
					Num2: dbaudiobookID,
				},
			)
		}
	}

	// Add authors
	for idx, authorName := range book.Authors {
		if authorName == "" {
			continue
		}

		authorID := addOrGetAuthor(authorName)
		if authorID > 0 {
			var existingRelation uint
			database.Scanrowsdyn(false,
				"SELECT id FROM dbaudiobook_authors WHERE dbaudiobook_id = ? AND dbauthor_id = ?",
				&existingRelation, &dbaudiobookID, &authorID)

			if existingRelation == 0 {
				_, _ = database.ExecNid(
					`INSERT INTO dbaudiobook_authors (dbaudiobook_id, dbauthor_id, role, position)
					 VALUES (?, ?, 'author', ?)`,
					&dbaudiobookID, &authorID, &idx,
				)
			}
		}
	}

	// Add alternate title if subtitle exists
	if book.Subtitle != "" {
		fullTitle := book.Title + ": " + book.Subtitle
		fullSlug := logger.StringToSlugCached(fullTitle)

		var existingTitle uint
		database.Scanrowsdyn(false,
			"SELECT id FROM dbaudiobook_titles WHERE dbaudiobook_id = ? AND slug = ?",
			&existingTitle, &dbaudiobookID, &fullSlug)

		if existingTitle == 0 {
			_, _ = database.ExecNid(
				`INSERT INTO dbaudiobook_titles (dbaudiobook_id, title, slug)
				 VALUES (?, ?, ?)`,
				&dbaudiobookID, &fullTitle, &fullSlug,
			)
		}
	}

	// Check if tracked audiobook already exists
	listName := cfgp.Lists[listid].Name

	var trackedID uint
	database.Scanrowsdyn(
		false,
		"SELECT id FROM audiobooks WHERE dbaudiobook_id = ? AND listname = ?",
		&trackedID,
		&dbaudiobookID,
		&listName,
	)

	if trackedID == 0 {
		audiobookID, err := database.ExecNid(
			`INSERT INTO audiobooks (dbaudiobook_id, listname, missing, quality_profile)
			 VALUES (?, ?, 1, ?)`,
			&dbaudiobookID, &listName, &cfgp.Lists[listid].CfgQuality.Name,
		)
		if err != nil {
			return err
		}

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				logger.CacheAudiobook,
				syncops.DbstaticOneStringTwoInt{
					Str:  listName,
					Num1: dbaudiobookID,
					Num2: logger.Int64ToUint(audiobookID),
				},
			)
		}

		logger.Logtype("info", 0).
			Str("audiobook", book.Title).
			Str("list", listName).
			Msg("Added audiobook to tracking list")
	}

	return nil
}

// DiscoverAndAddAuthorAudiobooks discovers other audiobooks by the same author and adds them to the database.
// This is called after successfully importing an audiobook to automatically add the author's bibliography.
func DiscoverAndAddAuthorAudiobooks(
	ctx context.Context,
	authorName string,
	cfgp *config.MediaTypeConfig,
	listid int,
	_ int,
) int {
	if authorName == "" || listid == -1 {
		return 0
	}

	logger.Logtype("info", 1).
		Str("author", authorName).
		Str("list", cfgp.Lists[listid].Name).
		Msg("Discovering other audiobooks by author")

	// Get Audnex provider from registry
	audnexProvider := providers.GetAudnex()
	if audnexProvider == nil {
		logger.Logtype("debug", 1).
			Str("author", authorName).
			Msg("Audnex provider not available")
		return 0
	}

	// First, search for the author to get their ASIN
	authors, err := audnexProvider.SearchAuthorByName(ctx, authorName)
	if err != nil || len(authors) == 0 {
		logger.Logtype("debug", 1).
			Str("author", authorName).
			Err(err).
			Msg("Failed to find author on Audnex")

		return 0
	}

	// Get the best match (first result)
	authorASIN := authors[0].ID

	logger.Logtype("debug", 1).
		Str("author", authorName).
		Str("asin", authorASIN).
		Str("matched_name", authors[0].Name).
		Msg("Found author on Audnex")

	// Get all audiobooks by this author
	audiobooks, err := audnexProvider.GetAuthorBooks(ctx, authorASIN)
	if err != nil || len(audiobooks) == 0 {
		logger.Logtype("debug", 1).
			Str("author", authorName).
			Str("asin", authorASIN).
			Err(err).
			Msg("Failed to get audiobooks by author from Audnex")

		return 0
	}

	logger.Logtype("info", 1).
		Str("author", authorName).
		Int("audiobooks_found", len(audiobooks)).
		Msg("Found audiobooks by author")

	// Get Goodreads provider for additional metadata
	grProvider := providers.GetGoodreads()

	// Add each audiobook to the database
	audiobooksAdded := 0
	for i := range audiobooks {
		if err := ctx.Err(); err != nil {
			break
		}

		// Skip if we already have this audiobook
		var existingID uint
		if audiobooks[i].ASIN != "" {
			database.Scanrowsdyn(false, "SELECT id FROM dbaudiobooks WHERE asin = ?",
				&existingID, &audiobooks[i].ASIN)
		}

		if existingID == 0 && audiobooks[i].ID != "" {
			database.Scanrowsdyn(false, "SELECT id FROM dbaudiobooks WHERE audible_id = ?",
				&existingID, &audiobooks[i].ID)
		}

		if existingID > 0 {
			logger.Logtype("debug", 2).
				Str("audiobook", audiobooks[i].Title).
				Str("author", authorName).
				Msg("Audiobook already in database, skipping")

			continue
		}

		// Import and merge audiobook data from multiple sources
		err = importAndMergeAudiobook(
			ctx,
			audiobooks[i].ASIN,
			&audiobooks[i],
			audnexProvider,
			grProvider,
			cfgp,
			listid,
		)
		if err == nil {
			audiobooksAdded++

			logger.Logtype("info", 1).
				Str("audiobook", audiobooks[i].Title).
				Str("author", authorName).
				Msg("Added audiobook from author discovery")
		} else {
			logger.Logtype("debug", 2).
				Str("audiobook", audiobooks[i].Title).
				Str("author", authorName).
				Err(err).
				Msg("Failed to add audiobook from author discovery")
		}
	}

	if audiobooksAdded > 0 {
		logger.Logtype("info", 0).
			Str("author", authorName).
			Int("audiobooks_added", audiobooksAdded).
			Int("total_found", len(audiobooks)).
			Msg("Author discovery completed")
	}

	return audiobooksAdded
}

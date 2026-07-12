package importfeed

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/musicbrainz"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// -----------------------------------------------------------------------------
// Common Errors
// -----------------------------------------------------------------------------

var (
	errBookIgnored              = errors.New("book ignored")
	errAudiobookIgnored         = errors.New("audiobook ignored")
	errAlbumIgnored             = errors.New("album ignored")
	errArtistIgnored            = errors.New("artist ignored")
	errAuthorIgnored            = errors.New("author ignored")
	errIsbnEmpty                = errors.New("isbn empty")
	errBookNotFoundInDatabase   = errors.New("book not found in database")
	errAuthorIdentifierEmpty    = errors.New("author identifier empty")
	errAuthorNotFoundInDatabase = errors.New("author not found in database")
	errAsinEmpty                = errors.New("asin empty")
	errAudiobookNotFoundInDB    = errors.New("audiobook not found in database")
	errAlbumIdentifierEmpty     = errors.New("album identifier empty")
	errAlbumNotFoundInDatabase  = errors.New("album not found in database")
	errArtistIdentifierEmpty    = errors.New("artist identifier empty")
	errArtistNotFoundInDatabase = errors.New("artist not found in database")
	errUseJobImportDBSeries     = errors.New("use JobImportDBSeries for series")
	errUnsupportedMediaType     = errors.New("unsupported media type")

	// Provider singletons. sync.OnceValue makes the lazy initialization safe for
	// concurrent imports - the previous nil-check-then-assign pattern was a data
	// race and could construct duplicate providers, each with its own rate
	// limiter (defeating e.g. MusicBrainz's 1 request/second limit).
	getOpenLibraryProvider = sync.OnceValue(openlibrary.NewProvider)
	getAudibleProvider     = sync.OnceValue(audible.NewProvider)
	getMusicBrainzProvider = sync.OnceValue(musicbrainz.NewProvider)
)

// -----------------------------------------------------------------------------
// Generic Import Helper Functions
// -----------------------------------------------------------------------------

// ImportConfig contains common configuration for import operations.
type ImportConfig struct {
	MediaType      uint
	Cfgp           *config.MediaTypeConfig
	ListID         int
	AddNew         bool
	UpdateMetadata bool
}

// ImportResult contains the result of an import operation.
type ImportResult struct {
	DBID       uint
	Added      bool
	Updated    bool
	Error      error
	Identifier string // ISBN, ASIN, MusicBrainz ID, etc.
}

// jobImportEntryByList is the shared implementation for the JobImport*ByList family.
// itemLabel is used as the log field key ("book", "audiobook", "album"),
// idFieldName as the error-log key ("isbn", "asin", "musicbrainz_id"),
// and ignoredErr is the sentinel that suppresses the error log.
func jobImportEntryByList(
	ctx context.Context,
	entry string,
	idx int,
	listid int,
	itemLabel, idFieldName, importMsg string,
	ignoredErr error,
	importFn func() (uint, error),
) error {
	if listid == -1 {
		return errListidNotSet
	}

	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	logger.Logtype("info", 0).
		Str(itemLabel, entry).
		Int(logger.StrRow, idx).
		Msg(importMsg)

	_, err := importFn()
	recordImportResult(ctx, err, ignoredErr, errJobRunning)

	if err != nil && !errors.Is(err, ignoredErr) {
		logger.Logtype("error", 1).
			Str(idFieldName, entry).
			Err(err).
			Msg("Import/Update Failed")

		return err
	}

	return nil
}

// checkaddTrackerEntry is the shared implementation for CheckaddAuthorEntry and
// CheckaddArtistEntry.  table and idCol are the DB table / column for the tracker
// row (e.g. "authors" / "dbauthor_id").
func checkaddTrackerEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	table, idCol string,
	trackMode string,
	dontSearch bool,
) error {
	if cfgplist == nil || cfgplist.Name == "" {
		return logger.ErrListnameEmpty
	}

	// Deny a second tracker row from a sibling list of the same media group config.
	if existsInConfigList(table, idCol, dbid, cfgp) {
		return nil
	}

	if trackMode == "" {
		trackMode = "all"
	}

	if _, err := database.ExecNid(
		"Insert into "+table+" (listname, "+idCol+", track_mode, dont_search) values (?, ?, ?, ?)",
		&cfgplist.Name,
		dbid,
		&trackMode,
		&dontSearch,
	); err != nil {
		return err
	}

	return nil
}

// mediaListEntrySpec parameterises the checkaddMediaEntry helper for different
// media types (audiobooks / albums).
type mediaListEntrySpec struct {
	table           string // row table:  "audiobooks" / "albums"
	idCol           string // FK column:  "dbaudiobook_id" / "dbalbum_id"
	ignoredErr      error
	joinTable       string // e.g. "dbaudiobook_authors" / "dbalbum_artists"
	joinParentCol   string // e.g. "dbauthor_id" / "dbartist_id"
	parentTable     string // "authors" / "artists"
	parentIDCol     string // "dbauthor_id" / "dbartist_id"
	parentTrackMode string // inserted into track_mode column
	parentLabel     string // "author" / "artist" (for logs)
	fnLabel         string // calling function name (for logs)
	logItemType     string // "Audiobook" / "Album" (for logs)
	itemInsertSQL   string // full INSERT INTO audiobooks/albums SQL
	idFieldName     string // "asin" / "identifier" (for logs)
	cacheKey        string // "CacheAudiobook" / "CacheAlbum"
}

// checkaddMediaEntry is the shared implementation for CheckaddAudiobookEntry and
// CheckaddAlbumEntry.
func checkaddMediaEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	idValue string,
	s mediaListEntrySpec,
) error {
	if cfgplist == nil || cfgplist.Name == "" {
		return logger.ErrListnameEmpty
	}

	var getcount uint

	if cfgplist.IgnoreMapListsLen >= 1 {
		args := logger.PLArrAny.Get()
		for idx := range cfgplist.IgnoreMapLists {
			args.Arr = append(args.Arr, &cfgplist.IgnoreMapLists[idx])
		}

		args.Arr = append(args.Arr, dbid)
		database.ScanrowsNArr(
			false,
			logger.JoinStrings(
				"select count() from "+s.table+" where listname in (?",
				cfgplist.IgnoreMapListsQu,
				") and "+s.idCol+" = ?",
			),
			&getcount,
			args.Arr,
		)
		logger.PLArrAny.Put(args)

		if getcount >= 1 {
			return s.ignoredErr
		}
	}

	// Deny a second insert from a sibling list of the same media group config.
	getcount = 0
	if existsInConfigList(s.table, s.idCol, dbid, cfgp) {
		getcount = 1
	}

	if getcount == 0 {
		logger.Logtype("debug", 1).
			Str(s.idFieldName, idValue).
			Msg("Insert " + s.logItemType + " for")

		var parentID uint
		database.Scanrowsdyn(false,
			"SELECT "+s.joinParentCol+" FROM "+s.joinTable+
				" WHERE "+s.idCol+" = ? ORDER BY position ASC LIMIT 1",
			&parentID, dbid)

		logger.Logtype("debug", 1).
			Uint("db"+s.logItemType+"_id", *dbid).
			Uint(s.parentLabel+"_id", parentID).
			Msg(s.fnLabel + ": Found " + s.parentLabel + " from " + s.joinTable)

		var trackedParentID uint
		if parentID > 0 {
			database.Scanrowsdyn(false,
				"SELECT id FROM "+s.parentTable+" WHERE "+s.parentIDCol+" = ? AND listname = ?",
				&trackedParentID, &parentID, &cfgplist.Name)

			if trackedParentID == 0 {
				result, err := database.ExecNid(
					"INSERT INTO "+s.parentTable+" ("+s.parentIDCol+", listname, track_mode, dont_search) VALUES (?, ?, ?, 0)",
					&parentID,
					&cfgplist.Name,
					&s.parentTrackMode,
				)
				if err == nil {
					trackedParentID = logger.Int64ToUint(result)
					logger.Logtype("debug", 1).
						Uint("tracked_"+s.parentLabel+"_id", trackedParentID).
						Str("listname", cfgplist.Name).
						Msg(s.fnLabel + ": Created tracked " + s.parentLabel)
				} else {
					logger.Logtype("error", 1).
						Err(err).
						Uint(s.parentLabel+"_id", parentID).
						Msg(s.fnLabel + ": Failed to create tracked " + s.parentLabel)
				}
			} else {
				logger.Logtype("debug", 1).
					Uint("tracked_"+s.parentLabel+"_id", trackedParentID).
					Msg(s.fnLabel + ": Found existing tracked " + s.parentLabel)
			}
		}

		logger.Logtype("debug", 1).
			Uint("tracked_"+s.parentLabel+"_id", trackedParentID).
			Msg(s.fnLabel + ": Inserting " + s.logItemType + " with " + s.parentLabel + "_id")

		mediaid, err := database.ExecNid(
			s.itemInsertSQL,
			&cfgplist.Name,
			dbid,
			&cfgplist.TemplateQuality,
			&trackedParentID,
		)
		if err != nil {
			return err
		}

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				s.cacheKey,
				syncops.DbstaticOneStringTwoInt{
					Str:  cfgplist.Name,
					Num1: *dbid,
					Num2: logger.Int64ToUint(mediaid),
				},
			)
		}
	}

	return nil
}

// importJobTTL bounds how long an import-job key stays claimed. A crashed or
// hung import would otherwise block re-imports of that identifier until restart.
const importJobTTL = 30 * time.Minute

// startJobIfAbsent atomically claims the import-job key and returns true.
// Returns false when an import for the same identifier is already running.
// This replaces the racy Check-then-Add pattern: with concurrent imports of
// the same identifier, exactly one caller wins.
func startJobIfAbsent(identifier string) bool {
	return importJobRunning.AddIfAbsent(
		identifier,
		struct{}{},
		time.Now().Add(importJobTTL).UnixNano(),
	)
}

// endJob marks a job as completed. The delete operates directly on the map
// (its methods are mutex-protected) - a queued MapTypeStructEmpty delete is
// routed to whichever map is registered under that type and historically
// never reached importJobRunning, leaving entries to linger until their TTL
// and wrongly blocking re-imports.
func endJob(identifier string) {
	importJobRunning.Delete(identifier)
}

// shouldUpdateMetadata checks if metadata should be updated based on last update time.
func shouldUpdateMetadata(updatedAt time.Time) bool {
	return !logger.TimeAfter(updatedAt, logger.TimeGetNow().Add(-1*time.Hour))
}

// -----------------------------------------------------------------------------
// Book Import Functions
// -----------------------------------------------------------------------------

// BookConfig represents configuration for importing a book.
type BookConfig struct {
	ISBN13        string
	ISBN10        string
	ASIN          string
	Title         string
	Author        string
	GoodreadsID   string
	OpenlibraryID string
	AlternateName []string
	Target        string
	DontSearch    bool
	DontUpgrade   bool
}

// JobImportBooksByList imports or updates a list of books in parallel.
func JobImportBooksByList(
	ctx context.Context,
	entry string,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) error {
	return jobImportEntryByList(ctx, entry, idx, listid,
		"book", "isbn", "Import/Update Book", errBookIgnored,
		func() (uint, error) { return JobImportBooks(ctx, entry, cfgp, listid, addnew) },
	)
}

// JobImportBooks imports a book into the database and specified list given its ISBN.
func JobImportBooks(
	ctx context.Context,
	isbn string,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}

	if isbn == "" {
		return 0, errIsbnEmpty
	}

	if !startJobIfAbsent(isbn) {
		return 0, errJobRunning
	}

	defer endJob(isbn)

	var (
		dbbookadded bool
		dbbook      database.Dbbook
	)

	// Try to find existing book by ISBN
	dbbook.ID = BookFindDBIDByISBN(&isbn)

	checkdbbook := dbbook.ID >= 1

	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(
			database.Getdatarow[string](
				false,
				"select listname from books where dbbook_id in (Select id from dbbooks where isbn_13=? or isbn_10=?)",
				&isbn,
				&isbn,
			),
		)
	}

	if !checkdbbook && addnew {
		if listid == -1 {
			return 0, logger.ErrCfgpNotFound
		}

		// Check if book already exists
		if database.Getdatarow[uint](
			false,
			"select id from dbbooks where isbn_13 = ? or isbn_10 = ?",
			&isbn, &isbn,
		) == 0 {
			logger.Logtype("debug", 1).
				Str("job", isbn).
				Msg("Insert dbbook for")

			dbresult, err := database.ExecNid(
				"insert into dbbooks (isbn_13) VALUES (?)",
				&isbn,
			)
			if err != nil {
				return 0, err
			}

			dbbook.ID = logger.Int64ToUint(dbresult)
			dbbookadded = true
		}
	}

	if dbbook.ID == 0 {
		dbbook.ID = BookFindDBIDByISBN(&isbn)
	}

	if dbbook.ID == 0 {
		return 0, errBookNotFoundInDatabase
	}

	// Update metadata if needed
	if dbbookadded || !addnew {
		logger.Logtype("debug", 1).
			Str("job", isbn).
			Msg("Get metadata for")

		err := dbbook.GetDbbookByIDP(&dbbook.ID)
		if err != nil {
			return 0, errBookIgnored
		}

		if dbbook.ISBN13 == "" && dbbook.ISBN10 == "" {
			dbbook.ISBN13 = isbn
		}

		if !dbbookadded && !shouldUpdateMetadata(dbbook.UpdatedAt) {
			logger.Logtype("debug", 1).
				Str("job", isbn).
				Msg("Skipped update metadata for dbbook")
		} else {
			metadata.BookGetMetadata(ctx, &dbbook, true)
			updateDbbook(&dbbook)
		}
	}

	if !addnew {
		return dbbook.ID, nil
	}

	if dbbook.ID == 0 {
		dbbook.ID = BookFindDBIDByISBN(&isbn)
		if dbbook.ID == 0 {
			return 0, logger.ErrNotFoundBook
		}
	}

	if listid == -1 {
		return 0, logger.ErrListnameEmpty
	}

	err := CheckaddBookEntry(&dbbook.ID, cfgp, &cfgp.Lists[listid], isbn)
	if err != nil {
		return 0, err
	}

	return dbbook.ID, nil
}

// BookFindDBIDByISBN looks up the database ID for a book by its ISBN.
func BookFindDBIDByISBN(isbn *string) uint {
	return database.Getdatarow[uint](
		false,
		"select id from dbbooks where isbn_13 = ? or isbn_10 = ?",
		isbn, isbn,
	)
}

// CheckaddBookEntry checks if a book should be added to the given list.
// Insertion is denied when the book already exists under any sibling list of
// cfgp (one entry per media group config).
func CheckaddBookEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	isbn string,
) error {
	if cfgplist == nil || cfgplist.Name == "" {
		return logger.ErrListnameEmpty
	}

	var getcount uint

	// Check ignore lists
	if cfgplist.IgnoreMapListsLen >= 1 {
		args := logger.PLArrAny.Get()
		for idx := range cfgplist.IgnoreMapLists {
			args.Arr = append(args.Arr, &cfgplist.IgnoreMapLists[idx])
		}

		args.Arr = append(args.Arr, dbid)
		database.ScanrowsNArr(
			false,
			logger.JoinStrings(
				"select count() from books where listname in (?",
				cfgplist.IgnoreMapListsQu,
				") and dbbook_id = ?",
			),
			&getcount,
			args.Arr,
		)
		logger.PLArrAny.Put(args)

		if getcount >= 1 {
			return errBookIgnored
		}
	}

	// Check if already present under any sibling list of this media group config
	getcount = 0
	if existsInConfigList("books", "dbbook_id", dbid, cfgp) {
		getcount = 1
	}

	if getcount == 0 {
		logger.Logtype("debug", 1).
			Str("isbn", isbn).
			Msg("Insert Book for")

		bookid, err := database.ExecNid(
			"Insert into books (missing, listname, dbbook_id, quality_profile) values (1, ?, ?, ?)",
			&cfgplist.Name,
			dbid,
			&cfgplist.TemplateQuality,
		)
		if err != nil {
			return err
		}

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoInt(
				"CacheBook",
				syncops.DbstaticOneStringTwoInt{
					Str:  cfgplist.Name,
					Num1: *dbid,
					Num2: logger.Int64ToUint(bookid),
				},
			)
		}
	}

	return nil
}

// updateDbbook updates the dbbook record in the database.
func updateDbbook(dbbook *database.Dbbook) {
	database.ExecN(
		"update dbbooks SET title = ?, original_title = ?, isbn_13 = ?, isbn_10 = ?, asin = ?, openlibrary_id = ?, goodreads_id = ?, description = ?, publisher = ?, publish_date = ?, page_count = ?, language = ?, genres = ?, cover_url = ?, dbauthor_id = ?, dbbook_series_id = ?, series_position = ?, average_rating = ?, ratings_count = ?, year = ?, slug = ? where id = ?",
		&dbbook.Title,
		&dbbook.OriginalTitle,
		&dbbook.ISBN13,
		&dbbook.ISBN10,
		&dbbook.ASIN,
		&dbbook.OpenlibraryID,
		&dbbook.GoodreadsID,
		&dbbook.Description,
		&dbbook.Publisher,
		&dbbook.PublishDate,
		&dbbook.PageCount,
		&dbbook.Language,
		&dbbook.Genres,
		&dbbook.CoverURL,
		&dbbook.DbauthorID,
		&dbbook.DbbookSeriesID,
		&dbbook.SeriesPosition,
		&dbbook.AverageRating,
		&dbbook.RatingsCount,
		&dbbook.Year,
		&dbbook.Slug,
		&dbbook.ID,
	)
}

// BookSearchByTitle searches for books by title using OpenLibrary.
func BookSearchByTitle(
	ctx context.Context,
	title, author string,
	limit int,
) ([]apiexternal_v2.BookSearchResult, error) {
	provider := getOpenLibraryProvider()
	return provider.SearchBooks(ctx, title, author, limit)
}

// -----------------------------------------------------------------------------
// Author Import Functions
// -----------------------------------------------------------------------------

// AuthorConfig represents configuration for importing an author.
type AuthorConfig struct {
	Name          string
	GoodreadsID   string
	OpenlibraryID string
	AlternateName []string
	TrackMode     string // "all", "series_only", "manual"
	DontSearch    bool
}

// JobImportAuthor imports an author into the database.
func JobImportAuthor(
	ctx context.Context,
	authorConfig *AuthorConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}

	if authorConfig.Name == "" && authorConfig.OpenlibraryID == "" &&
		authorConfig.GoodreadsID == "" {
		return 0, errAuthorIdentifierEmpty
	}

	jobName := authorConfig.Name
	if jobName == "" {
		jobName = authorConfig.OpenlibraryID
	}

	if !startJobIfAbsent(jobName) {
		return 0, errJobRunning
	}

	defer endJob(jobName)

	var (
		dbauthoradded bool
		dbauthor      database.Dbauthor
	)

	// Try to find existing author
	if authorConfig.OpenlibraryID != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbauthors where openlibrary_id = ?",
			&dbauthor.ID,
			&authorConfig.OpenlibraryID,
		)
	}

	if dbauthor.ID == 0 && authorConfig.GoodreadsID != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbauthors where goodreads_id = ?",
			&dbauthor.ID,
			&authorConfig.GoodreadsID,
		)
	}

	if dbauthor.ID == 0 && authorConfig.Name != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbauthors where name = ? COLLATE NOCASE",
			&dbauthor.ID,
			&authorConfig.Name,
		)
	}

	if dbauthor.ID == 0 && addnew {
		logger.Logtype("debug", 1).
			Str("author", authorConfig.Name).
			Msg("Insert dbauthor for")

		authorSlug := logger.StringToSlugCached(authorConfig.Name)

		dbresult, err := database.ExecNid(
			"insert into dbauthors (name, slug, openlibrary_id, goodreads_id) VALUES (?, ?, ?, ?)",
			&authorConfig.Name,
			&authorSlug,
			&authorConfig.OpenlibraryID,
			&authorConfig.GoodreadsID,
		)
		if err != nil {
			return 0, err
		}

		dbauthor.ID = logger.Int64ToUint(dbresult)
		dbauthoradded = true
	}

	if dbauthor.ID == 0 {
		return 0, errAuthorNotFoundInDatabase
	}

	// Update metadata if needed
	if dbauthoradded || !addnew {
		err := dbauthor.GetDbauthorByIDP(&dbauthor.ID)
		if err != nil {
			return 0, errAuthorIgnored
		}

		if !dbauthoradded && !shouldUpdateMetadata(dbauthor.UpdatedAt) {
			logger.Logtype("debug", 1).
				Str("author", authorConfig.Name).
				Msg("Skipped update metadata for dbauthor")
		} else {
			metadata.AuthorGetMetadata(ctx, &dbauthor, true)
			updateDbauthor(&dbauthor)
		}
	}

	// Add to tracking list if needed
	if addnew && listid >= 0 {
		err := CheckaddAuthorEntry(&dbauthor.ID, cfgp, &cfgp.Lists[listid], authorConfig)
		if err != nil {
			return 0, err
		}
	}

	return dbauthor.ID, nil
}

// CheckaddAuthorEntry checks if an author should be added to tracking.
func CheckaddAuthorEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	authorConfig *AuthorConfig,
) error {
	return checkaddTrackerEntry(dbid, cfgp, cfgplist, "authors", "dbauthor_id",
		authorConfig.TrackMode, authorConfig.DontSearch)
}

// updateDbauthor updates the dbauthor record in the database.
func updateDbauthor(dbauthor *database.Dbauthor) {
	database.ExecN(
		"update dbauthors SET name = ?, aliases = ?, bio = ?, birth_date = ?, death_date = ?, goodreads_id = ?, openlibrary_id = ?, website = ?, image_url = ? where id = ?",
		&dbauthor.Name,
		&dbauthor.Aliases,
		&dbauthor.Bio,
		&dbauthor.BirthDate,
		&dbauthor.DeathDate,
		&dbauthor.GoodreadsID,
		&dbauthor.OpenlibraryID,
		&dbauthor.Website,
		&dbauthor.ImageURL,
		&dbauthor.ID,
	)
}

// -----------------------------------------------------------------------------
// Audiobook Import Functions
// -----------------------------------------------------------------------------

// AudiobookConfig represents configuration for importing an audiobook.
type AudiobookConfig struct {
	ASIN          string
	AudibleID     string
	Title         string
	Author        string
	Narrator      string
	AlternateName []string
	Target        string
	DontSearch    bool
	DontUpgrade   bool
}

// JobImportAudiobooksByList imports or updates a list of audiobooks in parallel.
func JobImportAudiobooksByList(
	ctx context.Context,
	entry string,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) error {
	return jobImportEntryByList(ctx, entry, idx, listid,
		"audiobook", "asin", "Import/Update Audiobook", errAudiobookIgnored,
		func() (uint, error) { return JobImportAudiobooks(ctx, entry, cfgp, listid, addnew) },
	)
}

// JobImportAudiobooks imports an audiobook into the database given its ASIN.
func JobImportAudiobooks(
	ctx context.Context,
	asin string,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}

	if asin == "" {
		return 0, errAsinEmpty
	}

	if !startJobIfAbsent(asin) {
		return 0, errJobRunning
	}

	defer endJob(asin)

	var (
		dbaudiobookadded bool
		dbaudiobook      database.Dbaudiobook
	)

	// Try to find existing audiobook by ASIN
	dbaudiobook.ID = AudiobookFindDBIDByASIN(&asin)

	checkdbaudiobook := dbaudiobook.ID >= 1

	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(
			database.Getdatarow[string](
				false,
				"select listname from audiobooks where dbaudiobook_id in (Select id from dbaudiobooks where asin=?)",
				&asin,
			),
		)
	}

	if !checkdbaudiobook && addnew {
		if listid == -1 {
			return 0, logger.ErrCfgpNotFound
		}

		if database.Getdatarow[uint](
			false,
			"select id from dbaudiobooks where asin = ?",
			&asin,
		) == 0 {
			logger.Logtype("debug", 1).
				Str("job", asin).
				Msg("Insert dbaudiobook for")

			dbresult, err := database.ExecNid(
				"insert into dbaudiobooks (asin) VALUES (?)",
				&asin,
			)
			if err != nil {
				return 0, err
			}

			dbaudiobook.ID = logger.Int64ToUint(dbresult)
			dbaudiobookadded = true
		}
	}

	if dbaudiobook.ID == 0 {
		dbaudiobook.ID = AudiobookFindDBIDByASIN(&asin)
	}

	if dbaudiobook.ID == 0 {
		return 0, errAudiobookNotFoundInDB
	}

	// Update metadata if needed
	if dbaudiobookadded || !addnew {
		logger.Logtype("debug", 1).
			Str("job", asin).
			Msg("Get metadata for")

		err := dbaudiobook.GetDbaudiobookByIDP(&dbaudiobook.ID)
		if err != nil {
			return 0, errAudiobookIgnored
		}

		if dbaudiobook.ASIN == "" {
			dbaudiobook.ASIN = asin
		}

		if !dbaudiobookadded && !shouldUpdateMetadata(dbaudiobook.UpdatedAt) {
			logger.Logtype("debug", 1).
				Str("job", asin).
				Msg("Skipped update metadata for dbaudiobook")
		} else {
			metadata.AudiobookGetMetadata(ctx, cfgp, &dbaudiobook, true)
			updateDbaudiobook(&dbaudiobook)
		}
	}

	if !addnew {
		return dbaudiobook.ID, nil
	}

	if dbaudiobook.ID == 0 {
		dbaudiobook.ID = AudiobookFindDBIDByASIN(&asin)
		if dbaudiobook.ID == 0 {
			return 0, logger.ErrNotFoundAudiobook
		}
	}

	if listid == -1 {
		return 0, logger.ErrListnameEmpty
	}

	err := CheckaddAudiobookEntry(&dbaudiobook.ID, cfgp, &cfgp.Lists[listid], asin)
	if err != nil {
		return 0, err
	}

	return dbaudiobook.ID, nil
}

// AudiobookFindDBIDByASIN looks up the database ID for an audiobook by its ASIN.
func AudiobookFindDBIDByASIN(asin *string) uint {
	return database.Getdatarow[uint](
		false,
		"select id from dbaudiobooks where asin = ?",
		asin,
	)
}

// CheckaddAudiobookEntry checks if an audiobook should be added to the given list.
func CheckaddAudiobookEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	asin string,
) error {
	return checkaddMediaEntry(dbid, cfgp, cfgplist, asin, mediaListEntrySpec{
		table:           "audiobooks",
		idCol:           "dbaudiobook_id",
		ignoredErr:      errAudiobookIgnored,
		joinTable:       "dbaudiobook_authors",
		joinParentCol:   "dbauthor_id",
		parentTable:     "authors",
		parentIDCol:     "dbauthor_id",
		parentTrackMode: "audiobooks",
		parentLabel:     "author",
		fnLabel:         "CheckaddAudiobookEntry",
		logItemType:     "Audiobook",
		itemInsertSQL:   "Insert into audiobooks (missing, listname, dbaudiobook_id, quality_profile, author_id) values (1, ?, ?, ?, ?)",
		idFieldName:     "asin",
		cacheKey:        "CacheAudiobook",
	})
}

// updateDbaudiobook updates the dbaudiobook record in the database.
func updateDbaudiobook(dbaudiobook *database.Dbaudiobook) {
	database.ExecN(
		"update dbaudiobooks SET title = ?, asin = ?, audible_id = ?, runtime_minutes = ?, chapter_count = ?, release_date = ?, publisher = ?, language = ?, abridged = ?, cover_url = ?, sample_url = ?, average_rating = ?, ratings_count = ?, year = ?, slug = ?, dbbook_id = ?, description = ? where id = ?",
		&dbaudiobook.Title,
		&dbaudiobook.ASIN,
		&dbaudiobook.AudibleID,
		&dbaudiobook.RuntimeMinutes,
		&dbaudiobook.ChapterCount,
		&dbaudiobook.ReleaseDate,
		&dbaudiobook.Publisher,
		&dbaudiobook.Language,
		&dbaudiobook.Abridged,
		&dbaudiobook.CoverURL,
		&dbaudiobook.SampleURL,
		&dbaudiobook.AverageRating,
		&dbaudiobook.RatingsCount,
		&dbaudiobook.Year,
		&dbaudiobook.Slug,
		&dbaudiobook.DbbookID,
		&dbaudiobook.Description,
		&dbaudiobook.ID,
	)
}

// AudiobookSearchByTitle searches for audiobooks by title using Audible.
func AudiobookSearchByTitle(
	ctx context.Context,
	title string,
	limit int,
) ([]apiexternal_v2.AudiobookSearchResult, error) {
	provider := getAudibleProvider()
	return provider.SearchByTitle(ctx, title, limit)
}

// -----------------------------------------------------------------------------
// Music Album Import Functions
// -----------------------------------------------------------------------------

// AlbumConfig represents configuration for importing a music album.
type AlbumConfig struct {
	MusicBrainzID string
	DiscogsID     string
	UPC           string
	Title         string
	Artist        string
	AlternateName []string
	Target        string
	DontSearch    bool
	DontUpgrade   bool
}

// JobImportAlbumsByList imports or updates a list of albums in parallel.
func JobImportAlbumsByList(
	ctx context.Context,
	entry string,
	idx int,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) error {
	return jobImportEntryByList(ctx, entry, idx, listid,
		"album", "musicbrainz_id", "Import/Update Album", errAlbumIgnored,
		func() (uint, error) { return JobImportAlbums(ctx, entry, cfgp, listid, addnew) },
	)
}

// JobImportAlbums imports an album into the database given its MusicBrainz ID or UPC.
func JobImportAlbums(
	ctx context.Context,
	identifier string,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}

	if identifier == "" {
		return 0, errAlbumIdentifierEmpty
	}

	if !startJobIfAbsent(identifier) {
		return 0, errJobRunning
	}

	defer endJob(identifier)

	var (
		dbalbumadded bool
		dbalbum      database.Dbalbum
	)

	// Try to find existing album by MusicBrainz ID or UPC
	dbalbum.ID = AlbumFindDBIDByIdentifier(&identifier)

	checkdbalbum := dbalbum.ID >= 1

	if listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(
			database.Getdatarow[string](
				false,
				"select listname from albums where dbalbum_id in (Select id from dbalbums where musicbrainz_release_id=? or upc=?)",
				&identifier,
				&identifier,
			),
		)
	}

	if !checkdbalbum && addnew {
		if listid == -1 {
			return 0, logger.ErrCfgpNotFound
		}

		if database.Getdatarow[uint](
			false,
			"select id from dbalbums where musicbrainz_release_id = ? or upc = ?",
			&identifier, &identifier,
		) == 0 {
			logger.Logtype("debug", 1).
				Str("job", identifier).
				Msg("Insert dbalbum for")

			// Determine if identifier is UPC (numeric) or MusicBrainz ID
			var (
				dbresult int64
				err      error
			)

			if isUPC(identifier) {
				dbresult, err = database.ExecNid(
					"insert into dbalbums (upc) VALUES (?)",
					&identifier,
				)
			} else {
				dbresult, err = database.ExecNid(
					"insert into dbalbums (musicbrainz_release_id) VALUES (?)",
					&identifier,
				)
			}

			if err != nil {
				return 0, err
			}

			dbalbum.ID = logger.Int64ToUint(dbresult)
			dbalbumadded = true
		}
	}

	if dbalbum.ID == 0 {
		dbalbum.ID = AlbumFindDBIDByIdentifier(&identifier)
	}

	if dbalbum.ID == 0 {
		return 0, errAlbumNotFoundInDatabase
	}

	// Update metadata if needed
	if dbalbumadded || !addnew {
		logger.Logtype("debug", 1).
			Str("job", identifier).
			Msg("Get metadata for")

		err := dbalbum.GetDbalbumByIDP(&dbalbum.ID)
		if err != nil {
			return 0, errAlbumIgnored
		}

		if dbalbum.MusicbrainzReleaseID == "" && dbalbum.UPC == "" {
			if isUPC(identifier) {
				dbalbum.UPC = identifier
			} else {
				dbalbum.MusicbrainzReleaseID = identifier
			}
		}

		if !dbalbumadded && !shouldUpdateMetadata(dbalbum.UpdatedAt) {
			logger.Logtype("debug", 1).
				Str("job", identifier).
				Msg("Skipped update metadata for dbalbum")
		} else {
			metadata.AlbumGetMetadata(ctx, &dbalbum, true)
			updateDbalbum(&dbalbum)
		}
	}

	if !addnew {
		return dbalbum.ID, nil
	}

	if dbalbum.ID == 0 {
		dbalbum.ID = AlbumFindDBIDByIdentifier(&identifier)
		if dbalbum.ID == 0 {
			return 0, logger.ErrNotFoundAlbum
		}
	}

	if listid == -1 {
		return 0, logger.ErrListnameEmpty
	}

	err := CheckaddAlbumEntry(&dbalbum.ID, cfgp, &cfgp.Lists[listid], identifier)
	if err != nil {
		return 0, err
	}

	return dbalbum.ID, nil
}

// AlbumFindDBIDByIdentifier looks up the database ID for an album by MusicBrainz ID or UPC.
func AlbumFindDBIDByIdentifier(identifier *string) uint {
	return database.Getdatarow[uint](
		false,
		"select id from dbalbums where musicbrainz_release_id = ? or upc = ?",
		identifier, identifier,
	)
}

// isUPC checks if a string looks like a UPC/barcode (all numeric, 12-14 digits).
func isUPC(s string) bool {
	if len(s) < 12 || len(s) > 14 {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// CheckaddAlbumEntry checks if an album should be added to the given list.
func CheckaddAlbumEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	identifier string,
) error {
	return checkaddMediaEntry(dbid, cfgp, cfgplist, identifier, mediaListEntrySpec{
		table:           "albums",
		idCol:           "dbalbum_id",
		ignoredErr:      errAlbumIgnored,
		joinTable:       "dbalbum_artists",
		joinParentCol:   "dbartist_id",
		parentTable:     "artists",
		parentIDCol:     "dbartist_id",
		parentTrackMode: "albums",
		parentLabel:     "artist",
		fnLabel:         "CheckaddAlbumEntry",
		logItemType:     "Album",
		itemInsertSQL:   "Insert into albums (missing, listname, dbalbum_id, quality_profile, artist_id) values (1, ?, ?, ?, ?)",
		idFieldName:     "identifier",
		cacheKey:        "CacheAlbum",
	})
}

// updateDbalbum updates the dbalbum record in the database.
func updateDbalbum(dbalbum *database.Dbalbum) {
	database.ExecN(
		"update dbalbums SET title = ?, musicbrainz_release_group_id = ?, musicbrainz_release_id = ?, discogs_master_id = ?, discogs_release_id = ?, spotify_id = ?, upc = ?, release_date = ?, release_type = ?, format = ?, label = ?, country = ?, total_tracks = ?, total_runtime_ms = ?, genres = ?, styles = ?, cover_url = ?, year = ?, slug = ? where id = ?",
		&dbalbum.Title,
		&dbalbum.MusicbrainzReleaseGroupID,
		&dbalbum.MusicbrainzReleaseID,
		&dbalbum.DiscogsMasterID,
		&dbalbum.DiscogsReleaseID,
		&dbalbum.SpotifyID,
		&dbalbum.UPC,
		&dbalbum.ReleaseDate,
		&dbalbum.ReleaseType,
		&dbalbum.Format,
		&dbalbum.Label,
		&dbalbum.Country,
		&dbalbum.TotalTracks,
		&dbalbum.TotalRuntimeMs,
		&dbalbum.Genres,
		&dbalbum.Styles,
		&dbalbum.CoverURL,
		&dbalbum.Year,
		&dbalbum.Slug,
		&dbalbum.ID,
	)
}

// AlbumSearchByTitle searches for albums by title using MusicBrainz.
func AlbumSearchByTitle(
	ctx context.Context,
	title string,
	limit int,
) ([]apiexternal_v2.ReleaseSearchResult, error) {
	provider := getMusicBrainzProvider()
	results, _, err := provider.SearchReleases(ctx, title, limit, 0)
	return results, err
}

// -----------------------------------------------------------------------------
// Music Artist Import Functions
// -----------------------------------------------------------------------------

// ArtistConfig represents configuration for importing a music artist.
type ArtistConfig struct {
	MusicBrainzID string
	DiscogsID     string
	Name          string
	AlternateName []string
	TrackMode     string // "all", "albums_only", "manual"
	DontSearch    bool
}

// JobImportArtist imports a music artist into the database.
func JobImportArtist(
	ctx context.Context,
	artistConfig *ArtistConfig,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}

	if artistConfig.Name == "" && artistConfig.MusicBrainzID == "" && artistConfig.DiscogsID == "" {
		return 0, errArtistIdentifierEmpty
	}

	jobName := artistConfig.Name
	if jobName == "" {
		jobName = artistConfig.MusicBrainzID
	}

	if !startJobIfAbsent(jobName) {
		return 0, errJobRunning
	}

	defer endJob(jobName)

	var (
		dbartistadded bool
		dbartist      database.Dbartist
	)

	// Try to find existing artist
	if artistConfig.MusicBrainzID != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbartists where musicbrainz_id = ?",
			&dbartist.ID,
			&artistConfig.MusicBrainzID,
		)
	}

	if dbartist.ID == 0 && artistConfig.DiscogsID != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbartists where discogs_id = ?",
			&dbartist.ID,
			&artistConfig.DiscogsID,
		)
	}

	if dbartist.ID == 0 && artistConfig.Name != "" {
		artistSlug := logger.StringToSlugCached(artistConfig.Name)
		database.Scanrowsdyn(
			false,
			"select id from dbartists where name = ? COLLATE NOCASE or slug = ?",
			&dbartist.ID,
			&artistConfig.Name,
			&artistSlug,
		)
	}

	// Back-fill MusicBrainzID from the row if we found the artist by Discogs/name.
	if dbartist.ID > 0 && artistConfig.MusicBrainzID == "" {
		database.Scanrowsdyn(
			false,
			`select musicbrainz_id from dbartists where id = ?`,
			&artistConfig.MusicBrainzID,
			&dbartist.ID,
		)
	}

	if dbartist.ID == 0 && addnew {
		logger.Logtype("debug", 1).
			Str("artist", artistConfig.Name).
			Msg("Insert dbartist for")

		artistSlug := logger.StringToSlugCached(artistConfig.Name)

		dbresult, err := database.ExecNid(
			"insert into dbartists (name, slug, musicbrainz_id, discogs_id) VALUES (?, ?, ?, ?)",
			&artistConfig.Name,
			&artistSlug,
			&artistConfig.MusicBrainzID,
			&artistConfig.DiscogsID,
		)
		if err != nil {
			return 0, err
		}

		dbartist.ID = logger.Int64ToUint(dbresult)
		dbartistadded = true
	}

	if dbartist.ID == 0 {
		return 0, errArtistNotFoundInDatabase
	}

	// Update metadata if needed.
	// Also force a re-fetch when the artist has no name yet (e.g. a previous metadata
	// call failed and left the row with name=""), regardless of whether we just
	// inserted it or found an existing row.
	if dbartistadded || !addnew || dbartist.Name == "" {
		err := dbartist.GetDbartistByIDP(&dbartist.ID)
		if err != nil {
			return 0, errArtistIgnored
		}

		if !dbartistadded && dbartist.Name != "" && !shouldUpdateMetadata(dbartist.UpdatedAt) {
			logger.Logtype("debug", 1).
				Str("artist", artistConfig.Name).
				Msg("Skipped update metadata for dbartist")
		} else {
			if err := metadata.ArtistGetMetadata(ctx, &dbartist, true); err != nil {
				logger.Logtype("warn", 0).
					Str("mbid", artistConfig.MusicBrainzID).
					Err(err).
					Msg("ArtistGetMetadata failed; artist will be saved without name")
			}

			if dbartist.Name == "" {
				logger.Logtype("warn", 0).
					Str("mbid", artistConfig.MusicBrainzID).
					Msg("artist name still empty after metadata fetch")
			}

			updateDbartist(&dbartist)
		}
	}

	// Add to tracking list if needed
	if addnew && listid >= 0 {
		err := CheckaddArtistEntry(&dbartist.ID, cfgp, &cfgp.Lists[listid], artistConfig)
		if err != nil {
			return 0, err
		}
	}

	return dbartist.ID, nil
}

// CheckaddArtistEntry checks if an artist should be added to tracking.
func CheckaddArtistEntry(
	dbid *uint,
	cfgp *config.MediaTypeConfig,
	cfgplist *config.MediaListsConfig,
	artistConfig *ArtistConfig,
) error {
	return checkaddTrackerEntry(dbid, cfgp, cfgplist, "artists", "dbartist_id",
		artistConfig.TrackMode, artistConfig.DontSearch)
}

// updateDbartist updates the dbartist record in the database.
func updateDbartist(dbartist *database.Dbartist) {
	database.ExecN(
		"update dbartists SET name = ?, sort_name = ?, musicbrainz_id = ?, discogs_id = ?, spotify_id = ?, artist_type = ?, country = ?, begin_date = ?, end_date = ?, disambiguation = ?, bio = ?, image_url = ?, genres = ? where id = ?",
		&dbartist.Name,
		&dbartist.SortName,
		&dbartist.MusicbrainzID,
		&dbartist.DiscogsID,
		&dbartist.SpotifyID,
		&dbartist.ArtistType,
		&dbartist.Country,
		&dbartist.BeginDate,
		&dbartist.EndDate,
		&dbartist.Disambiguation,
		&dbartist.Bio,
		&dbartist.ImageURL,
		&dbartist.Genres,
		&dbartist.ID,
	)
}

// ArtistSearchByName searches for artists by name using MusicBrainz.
func ArtistSearchByName(
	ctx context.Context,
	name string,
	limit int,
) ([]apiexternal_v2.ArtistSearchResult, error) {
	provider := getMusicBrainzProvider()
	return provider.SearchArtists(ctx, name, limit)
}

// -----------------------------------------------------------------------------
// Unified Import Function
// -----------------------------------------------------------------------------

// JobImportByType provides a unified entry point for importing any media type.
func JobImportByType(
	ctx context.Context,
	mediaType uint,
	identifier string,
	cfgp *config.MediaTypeConfig,
	listid int,
	addnew bool,
) (uint, error) {
	switch mediaType {
	case config.MediaTypeMovie:
		return JobImportMovies(identifier, cfgp, listid, addnew, false)

	case config.MediaTypeSeries:
		// Series uses different config structure - call jobImportDBSeries directly
		return 0, errUseJobImportDBSeries

	case config.MediaTypeBook:
		return JobImportBooks(ctx, identifier, cfgp, listid, addnew)

	case config.MediaTypeAudiobook:
		return JobImportAudiobooks(ctx, identifier, cfgp, listid, addnew)

	case config.MediaTypeMusic:
		return JobImportAlbums(ctx, identifier, cfgp, listid, addnew)

	default:
		return 0, errUnsupportedMediaType
	}
}

// -----------------------------------------------------------------------------
// Alternate Title Management (unified for all media types)
// -----------------------------------------------------------------------------

// addAlternateTitle adds an alternate title for any media type.
func addAlternateTitle(mediaType uint, dbid *uint, title *string, regionin ...*string) {
	cfg, ok := metadata.GetTitleConfigs()[mediaType]
	if !ok {
		return
	}

	// Check if title already exists
	if database.Getdatarow[uint](
		false,
		"select count() from "+cfg.TableName+" where "+cfg.ParentIDColumn+" = ? and title = ? COLLATE NOCASE",
		dbid,
		title,
	) != 0 {
		return
	}

	slug := logger.StringToSlugCachedP(title)

	if len(regionin) > 0 && regionin[0] != nil {
		database.ExecN(
			"Insert into "+cfg.TableName+" (title, slug, "+cfg.ParentIDColumn+", region) values (?, ?, ?, ?)",
			title,
			&slug,
			dbid,
			regionin[0],
		)
	} else {
		database.ExecN(
			"Insert into "+cfg.TableName+" (title, slug, "+cfg.ParentIDColumn+") values (?, ?, ?)",
			title,
			&slug,
			dbid,
		)
	}

	if config.GetSettingsGeneral().UseMediaCache && cfg.CacheKey != "" {
		database.AppendCacheTwoString(
			cfg.CacheKey,
			syncops.DbstaticTwoStringOneInt{Str1: *title, Str2: slug, Num: *dbid},
		)
	}
}

// AddBookAlternateTitle adds an alternate title for a book.
func AddBookAlternateTitle(dbbookid *uint, title *string, region ...*string) {
	addAlternateTitle(config.MediaTypeBook, dbbookid, title, region...)
}

// AddAudiobookAlternateTitle adds an alternate title for an audiobook.
func AddAudiobookAlternateTitle(dbaudiobookid *uint, title *string, region ...*string) {
	addAlternateTitle(config.MediaTypeAudiobook, dbaudiobookid, title, region...)
}

// AddAlbumAlternateTitle adds an alternate title for an album.
func AddAlbumAlternateTitle(dbalbumid *uint, title *string, region ...*string) {
	addAlternateTitle(config.MediaTypeMusic, dbalbumid, title, region...)
}

// -----------------------------------------------------------------------------
// Identifier Lookup Functions
// -----------------------------------------------------------------------------

// FindBookByTitle searches for a book in the database or external APIs by title.
func FindBookByTitle(
	ctx context.Context,
	title string,
	author string,
	year uint16,
) (*database.Dbbook, error) {
	// First check database
	var dbbook database.Dbbook
	database.Scanrowsdyn(
		false,
		"select id from dbbooks where title = ? COLLATE NOCASE",
		&dbbook.ID,
		&title,
	)

	if dbbook.ID != 0 {
		dbbook.GetDbbookByIDP(&dbbook.ID)
		return &dbbook, nil
	}

	// Search external API
	results, err := BookSearchByTitle(ctx, title, author, 5)
	if err != nil {
		return nil, err
	}

	for i := range results {
		// Check year if provided
		if year != 0 && results[i].PublishYear != 0 {
			if results[i].PublishYear != int(year) && results[i].PublishYear != int(year+1) &&
				results[i].PublishYear != int(year-1) {
				continue
			}
		}

		// Check author if provided
		if author != "" && len(results[i].Authors) > 0 {
			found := false
			for j := range results[i].Authors {
				if logger.ContainsI(results[i].Authors[j], author) {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		// Create dbbook from result
		dbbook.Title = results[i].Title
		dbbook.ISBN13 = results[i].ISBN13
		dbbook.ISBN10 = results[i].ISBN10

		dbbook.OpenlibraryID = results[i].ID
		if results[i].PublishYear != 0 {
			dbbook.Year = uint16(
				results[i].PublishYear,
			)
		}

		return &dbbook, nil
	}

	return nil, logger.ErrNotFoundBook
}

// FindAudiobookByTitle searches for an audiobook in the database or external APIs by title.
func FindAudiobookByTitle(
	ctx context.Context,
	title string,
	author string,
) (*database.Dbaudiobook, error) {
	// First check database
	var dbaudiobook database.Dbaudiobook
	database.Scanrowsdyn(
		false,
		"select id from dbaudiobooks where title = ? COLLATE NOCASE",
		&dbaudiobook.ID,
		&title,
	)

	if dbaudiobook.ID != 0 {
		dbaudiobook.GetDbaudiobookByIDP(&dbaudiobook.ID)
		return &dbaudiobook, nil
	}

	// Search external API
	results, err := AudiobookSearchByTitle(ctx, title, 5)
	if err != nil {
		return nil, err
	}

	for i := range results {
		// Check author if provided
		if author != "" && len(results[i].Authors) > 0 {
			found := false
			for j := range results[i].Authors {
				if logger.ContainsI(results[i].Authors[j], author) {
					found = true
					break
				}
			}

			if !found {
				continue
			}
		}

		// Create dbaudiobook from result
		dbaudiobook.Title = results[i].Title
		dbaudiobook.ASIN = results[i].ASIN
		dbaudiobook.AudibleID = results[i].ID

		dbaudiobook.RuntimeMinutes = results[i].RuntimeMinutes
		if results[i].ReleaseYear != 0 {
			dbaudiobook.Year = uint16(
				results[i].ReleaseYear,
			)
		}

		return &dbaudiobook, nil
	}

	return nil, logger.ErrNotFoundAudiobook
}

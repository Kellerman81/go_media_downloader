// Package books implements the mediatype.Handler interface for book media.
// It registers itself automatically via init() and provides all book-specific
// logic for searching, parsing, organizing, and processing book files.
package books

import (
	"context"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

// handler implements mediatype.Handler for books.
type handler struct {
	refreshFunc  mediatype.RefreshFunc
	dataFullFunc mediatype.DataFullFunc
}

var (
	// Handler is the singleton instance.
	Handler = &handler{}

	// StringsMap contains all book-specific string mappings for SQL queries, cache names, etc.
	StringsMap = map[string]string{
		"CacheDBMedia":             "CacheDBBook",
		"DBCountDBMedia":           "select count() from dbbooks",
		"DBCacheDBMedia":           "select title, slug, isbn_13, year, id from dbbooks",
		"CacheMedia":               "CacheBook",
		"DBCountMedia":             "select count() from books",
		"DBCacheMedia":             "select lower(listname), dbbook_id, id from books",
		"CacheHistoryTitle":        "CacheHistoryTitleBook",
		"CacheHistoryUrl":          "CacheHistoryUrlBook",
		"DBHistoriesUrl":           "select distinct url from book_histories",
		"DBHistoriesTitle":         "select distinct title from book_histories",
		"DBCountHistoriesUrl":      "select count() from (select distinct url from book_histories)",
		"DBCountHistoriesTitle":    "select count() from (select distinct title from book_histories)",
		"CacheMediaTitles":         "CacheTitlesBook",
		"DBCountDBTitles":          "select count() from dbbook_titles where title != ''",
		"DBCacheDBTitles":          "select title, slug, dbbook_id from dbbook_titles where title != ''",
		"CacheFiles":               "CacheFilesBook",
		"DBCountFiles":             "select count() from book_files",
		"DBCacheFiles":             "select location from book_files",
		"CacheUnmatched":           "CacheUnmatchedBook",
		"DBCountUnmatched":         "select count() from book_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBRemoveUnmatched":        "delete from book_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		"DBCacheUnmatched":         "select filepath from book_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBCountFilesLocation":     "select count() from book_files where location = ?",
		"DBCountUnmatchedPath":     "select count() from book_file_unmatcheds where filepath = ?",
		"DBCountDBTitlesDBID":      "select count() from (select distinct title, slug from dbbook_titles where dbbook_id = ? and title != '')",
		"DBDistinctDBTitlesDBID":   "select distinct title, slug, dbbook_id from dbbook_titles where dbbook_id = ? and title != ''",
		"DBMediaTitlesID":          "select year, title, slug from dbbooks where id = ?",
		"DBFilesQuality":           "select 0, 0, 0, 0, 0, 0, 0 from book_files where id = ?",
		"DBCountFilesByList":       "select count() from book_files where book_id in (select id from books where listname = ? COLLATE NOCASE)",
		"DBLocationFilesByList":    "select location from book_files where book_id in (select id from books where listname = ? COLLATE NOCASE)",
		"DBIDsFilesByLocation":     "select location, id, book_id from book_files",
		"DBCountFilesByMediaID":    "select count() from book_files where book_id = ?",
		"DBCountFilesByLocation":   "select count() from book_files",
		"TableFiles":               "book_files",
		"TableMedia":               "books",
		"DBCountMediaByList":       "select count() from books where listname = ? COLLATE NOCASE",
		"DBIDMissingMediaByList":   "select id,missing from books where listname = ? COLLATE NOCASE",
		"DBUpdateMissing":          "update books set missing = ? where id = ?",
		"DBListnameByMediaID":      "select listname from books where id = ?",
		"DBRootPathFromMediaID":    "select rootpath from books where id = ?",
		"DBDeleteFileByIDLocation": "delete from book_files where book_id = ? and location = ?",
		"DBCountHistoriesByTitle":  "select count() from book_histories where title = ?",
		"DBCountHistoriesByUrl":    "select count() from book_histories where url = ?",
		"DBLocationIDFilesByID":    "select location, id from book_files where book_id = ?",
		"DBFilePrioFilesByID":      "select location, book_id, id, 0, 0, 0, 0, 0, 0, 0 from book_files where book_id = ?",
		"DBAudioFilePrioFilesByID": "select location, book_id, id, format, 0, 0, 0 from book_files where book_id = ?",
		"UpdateMediaLastscan":      "update books set lastscan = datetime('now','localtime') where id = ?",
		"DBQualityMediaByID":       "select quality_profile from books where id = ?",
		"SearchGenSelect":          "select books.quality_profile, books.id ",
		"SearchGenTable":           " from books inner join dbbooks on dbbooks.id=books.dbbook_id where ",
		"SearchGenMissing":         "books.missing = 1 and books.listname in (?",
		"SearchGenMissingEnd":      ")",
		"SearchGenReached":         "quality_reached = 0 and missing = 0 and listname in (?",
		"SearchGenLastScan":        " and (books.lastscan is null or books.Lastscan < ?)",
		"SearchGenDate":            " and (dbbooks.publish_date < ? or dbbooks.publish_date is null)",
		"SearchGenOrder":           " order by books.Lastscan asc",
		"DBIDUnmatchedPathList":    "select id from book_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		"InsertUnmatched":          "Insert into book_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		"UpdateUnmatched":          "update book_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		"GetRSSData":               "select books.dont_search, books.dont_upgrade, books.listname, books.quality_profile, dbbooks.title from books inner join dbbooks ON dbbooks.id=books.dbbook_id where books.id = ?",
		"GetOrganizeData":          "select dbbook_id, rootpath, listname from books where id = ?",
		"ClearHistoryByList":       "delete from book_histories where book_id in (Select id from books where listname = ? COLLATE NOCASE)",
		"QueryMediaByList":         "select id, quality_reached, quality_profile from books where listname = ? COLLATE NOCASE",
		"QueryMediaCountByList":    "select count() from books where listname = ? COLLATE NOCASE",
		"UpdateQualityReached":     "update books set quality_reached = ? where id = ?",
		"SelectRootpath":           "select rootpath from books where id = ?",
		"InsertFile":               "insert into book_files (location, filename, extension, quality_profile, book_id, dbbook_id) values (?, ?, ?, ?, ?, ?)",
		"UpdateMissingByID":        "update books set missing = 0 where id = ?",
		"UpdateQualityReachedByID": "update books set quality_reached = ? where id = ?",
		"DeleteUnmatchedByPath":    "delete from book_file_unmatcheds where filepath = ?",
		"SelectRuntime":            "select page_count from dbbooks where id = ?",
		"InsertFileOrganize":       "insert into book_files (location, filename, extension, quality_profile, book_id, dbbook_id) values (?, ?, ?, ?, ?, ?)",
		"UpdateMissingReached":     "update books SET missing = 0, quality_reached = ? where id = ?",
		// Author-based search queries
		"SearchAuthorsMissing":    "SELECT DISTINCT da.name, da.id FROM dbauthors da JOIN authors auth ON da.id = auth.dbauthor_id JOIN books b ON b.author_id = auth.id WHERE auth.track_mode != 'none' AND auth.dont_search = 0 AND b.missing = 1 AND b.listname IN (?",
		"SearchAuthorsMissingEnd": ") ORDER BY RANDOM() LIMIT 20",
		"SearchAuthorsUpgrade":    "SELECT DISTINCT da.name, da.id FROM dbauthors da JOIN authors auth ON da.id = auth.dbauthor_id JOIN books b ON b.author_id = auth.id WHERE auth.track_mode != 'none' AND auth.dont_search = 0 AND b.missing = 0 AND b.quality_reached = 0 AND b.listname IN (?",
		"SearchAuthorsUpgradeEnd": ") ORDER BY RANDOM() LIMIT 20",
	}
)

func init() {
	mediatype.Register(Handler)
	mtstrings.Register(config.MediaTypeBook, StringsMap)
}

// RegisterRefresh sets the refresh function for books.
func RegisterRefresh(fn mediatype.RefreshFunc) {
	Handler.refreshFunc = fn
}

// RegisterDataFull sets the data full function for books.
func RegisterDataFull(fn mediatype.DataFullFunc) {
	Handler.dataFullFunc = fn
}

// GetType returns the media type constant for books.
func (h *handler) GetType() uint {
	return config.MediaTypeBook
}

// GetCategoryName returns the category name for job history.
func (h *handler) GetCategoryName() string {
	return "book"
}

// GetTableName returns the database table name for books.
func (h *handler) GetTableName() string {
	return "books"
}

// GetDBIDs retrieves database IDs for the parsed book info.
// It attempts ISBN lookup first, then falls back to title-based search.
func (h *handler) GetDBIDs(info *database.ParseInfo) error {
	// Handle ISBN lookup
	if info.ISBN != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbbooks where isbn_13 = ? or isbn_10 = ?",
			&info.DbbookID,
			&info.ISBN, &info.ISBN,
		)
	}

	if info.DbbookID == 0 {
		return logger.ErrNotFoundDbbook
	}

	return nil
}

// GetDBIDsFull retrieves database IDs for a book with full search capabilities.
// It attempts ISBN lookup first, then falls back to title-based search,
// and finally finds the book in configured lists.
func (h *handler) GetDBIDsFull(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowSearchTitle bool,
	_ bool,
) error {
	// Handle ISBN lookup
	if m.ISBN != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbbooks where isbn_13 = ? or isbn_10 = ?",
			&m.DbbookID,
			&m.ISBN, &m.ISBN,
		)
	}

	// Title-based search if ISBN lookup failed
	if m.DbbookID == 0 && m.Title != "" && allowSearchTitle && cfgp.Name != "" {
		// Strip title prefixes/postfixes
		for _, lst := range cfgp.ListsMap {
			if lst.TemplateQuality != "" {
				m.StripTitlePrefixPostfixGetQual(lst.CfgQuality)
			}
		}

		m.Title = logger.TrimSpace(m.Title)

		// Try author-first lookup prioritizing books in the wanted list
		m.FindDbbookByAuthorFirstFromWantedList(cfgp.ListsNames)

		// Fallback to regular author-first lookup
		if m.DbbookID == 0 {
			m.FindDbbookByAuthorFirst()
		}

		// Fallback to traditional title-based search
		if m.DbbookID == 0 {
			m.FindDbbookByTitle()
		}
	}

	if m.DbbookID == 0 {
		return logger.ErrNotFoundDbbook
	}

	// Find book in lists (if not already found by wanted-list search)
	if m.BookID != 0 {
		return nil
	}

	return h.findInLists(m, cfgp)
}

// findInLists attempts to locate a book in configured media lists by its database ID.
func (h *handler) findInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			"select id from books where dbbook_id = ? and listname = ? COLLATE NOCASE",
			&m.BookID,
			&m.DbbookID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.BookID == 0 && cfgp.Name != "" && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.BookID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheBook,
					m.DbbookID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(
					false,
					"select id from books where listname = ? COLLATE NOCASE and dbbook_id = ?",
					&m.BookID,
					&cfgp.Lists[idx].Name,
					&m.DbbookID,
				)
			}

			if m.BookID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.BookID == 0 {
		return logger.ErrNotFoundBook
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.BookID)
	}

	return nil
}

// ValidateIDs checks if all required IDs are set for books.
func (h *handler) ValidateIDs(info *database.ParseInfo) bool {
	return info.BookID != 0 && info.DbbookID != 0
}

// SetTempID sets the temporary ID from BookID.
func (h *handler) SetTempID(info *database.ParseInfo) {
	info.TempID = info.BookID
}

// SetDBID sets the DbbookID field.
func (h *handler) SetDBID(info *database.ParseInfo, dbid uint) {
	info.DbbookID = dbid
}

// GetDBID returns the DbbookID field.
func (h *handler) GetDBID(info *database.ParseInfo) uint {
	return info.DbbookID
}

// GetMediaID returns the BookID.
func (h *handler) GetMediaID(info *database.ParseInfo) uint {
	return info.BookID
}

// SetMediaID sets the BookID.
func (h *handler) SetMediaID(info *database.ParseInfo, id uint) {
	info.BookID = id
}

// GetListID retrieves the list ID for the book.
func (h *handler) GetListID(cfgp *config.MediaTypeConfig, info *database.ParseInfo) int {
	if info.BookID != 0 {
		return database.GetMediaListIDGetListname(cfgp, &info.BookID)
	}

	return -1
}

// ClearUntrustedID clears the ISBN if indexer is not trusted.
func (h *handler) ClearUntrustedID(entry *apiexternal_v2.Nzbwithprio) {
	// Books don't have a specific trust flag like IMDB/TVDB
	// Could be extended if needed
}

// SetNzbID sets the NzbbookID field.
func (h *handler) SetNzbID(entry *apiexternal_v2.Nzbwithprio, mediaid uint) {
	entry.NzbbookID = mediaid
}

// SetEntryTempID sets the temp ID from NzbbookID.
func (h *handler) SetEntryTempID(entry *apiexternal_v2.Nzbwithprio) {
	entry.Info.TempID = entry.NzbbookID
}

// PerformIDSearch executes a search - books use query-based search only.
func (h *handler) PerformIDSearch(
	indcfg *config.IndexersConfig,
	quality *config.QualityConfig,
	entry *apiexternal_v2.Nzbwithprio,
	cats int,
	raw *apiexternal.NzbSlice,
) error {
	// Books don't support ID-based search like IMDB/TVDB
	// They use query-based search only
	return nil
}

// ClearUnmatchedCache removes the file from the book unmatched cache.
func (h *handler) ClearUnmatchedCache(fpath string) {
	database.SlicesCacheContainsDelete(logger.CacheUnmatchedBook, fpath)
}

// ShortenYearPattern returns true - books shorten all patterns including year.
func (h *handler) ShortenYearPattern() bool {
	return true
}

// GenerateIdentifier does nothing for books - books don't have episode identifiers.
func (h *handler) GenerateIdentifier(info *database.ParseInfo, onlyIfEmpty bool) {
	// Books don't have identifiers like series do
}

// GetSchedulerRssSeasons returns empty strings for books - no RSS seasons jobs.
func (h *handler) GetSchedulerRssSeasons(
	scheduler *config.SchedulerConfig,
	jobType string,
) (string, string) {
	return "", ""
}

// GetSchedulerRssArtistsAuthors returns the interval and cron strings for author-based RSS searches.
func (h *handler) GetSchedulerRssArtistsAuthors(
	scheduler *config.SchedulerConfig,
	jobType string,
) (string, string) {
	switch jobType {
	case logger.StrRssAuthors:
		return scheduler.IntervalIndexerRssAuthors, scheduler.CronIndexerRssAuthors
	case logger.StrRssAuthorsUpgrade:
		return scheduler.IntervalIndexerRssAuthorsUpgrade, scheduler.CronIndexerRssAuthorsUpgrade
	}

	return "", ""
}

// Refresh refreshes book data.
func (h *handler) Refresh(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if h.refreshFunc != nil {
		return h.refreshFunc(ctx, cfgp, data)
	}

	return nil
}

// DataFull performs full data refresh for books.
func (h *handler) DataFull() {
	if h.dataFullFunc != nil {
		h.dataFullFunc()
	}
}

// SearchConfigByName returns nil, false for books - config file search is not supported.
func (h *handler) SearchConfigByName(
	searchName string,
	listCfg *config.MediaListsConfig,
) (*config.ManualConfig, bool) {
	return nil, false
}

// RecordDownloadHistory records a book download in the book_histories table.
func (h *handler) RecordDownloadHistory(nzb *apiexternal_v2.Nzbwithprio, targetPath string) error {
	var (
		bookID, dbbookID uint
		qualityProfile   string
	)

	if nzb.NzbbookID != 0 {
		bookID = nzb.NzbbookID
		database.Scanrowsdyn(
			false,
			"select quality_profile from books where id = ?",
			&qualityProfile,
			&nzb.NzbbookID,
		)
		database.Scanrowsdyn(
			false,
			"select dbbook_id from books where id = ?",
			&dbbookID,
			&nzb.NzbbookID,
		)
	}

	database.ExecN(
		"Insert into book_histories (title, url, target, indexer, downloaded_at, book_id, dbbook_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?)",
		&nzb.NZB.Title,
		&nzb.NZB.DownloadURL,
		&targetPath,
		&nzb.NZB.Indexer.Name,
		&bookID,
		&dbbookID,
		&qualityProfile,
	)

	return nil
}

// GetDownloadTargetFolder returns the target folder name for a book download.
// Returns title with author (e.g., "Book Title - Author Name").
func (h *handler) GetDownloadTargetFolder(
	nzb *apiexternal_v2.Nzbwithprio,
	dbExternalID string,
) string {
	if nzb.NZB.Title != "" {
		return nzb.NZB.Title
	}

	if nzb.Info.Title != "" {
		return nzb.Info.Title
	}

	return ""
}

// FillSearchVar fills search variables from the database for the given book ID.
// Sets NzbbookID, loads data from DB, validates required fields.
func (h *handler) FillSearchVar(entry *apiexternal_v2.Nzbwithprio, mediaid uint) error {
	entry.NzbbookID = mediaid
	if mediaid == 0 {
		return logger.ErrNotFoundDbbook
	}

	var authorName string

	database.GetdatarowArgs(
		"select books.dbbook_id, books.dont_search, books.dont_upgrade, books.listname, books.quality_profile, dbbooks.year, dbbooks.title from books inner join dbbooks ON dbbooks.id=books.dbbook_id where books.id = ?",
		&entry.NzbbookID,
		&entry.Dbid,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Listname,
		&entry.Quality,
		&entry.Info.Year,
		&entry.WantedTitle,
	)

	if entry.Dbid == 0 {
		return logger.ErrNotFoundDbbook
	}

	if entry.DontSearch {
		return logger.ErrDisabled
	}

	// Get the primary author name for this book
	database.GetdatarowArgs(
		"select dbauthors.name from dbbook_authors inner join dbauthors ON dbauthors.id=dbbook_authors.dbauthor_id where dbbook_authors.dbbook_id = ? order by dbbook_authors.position limit 1",
		&entry.Dbid,
		&authorName,
	)

	// Build search query: "Title Author"
	if authorName != "" {
		entry.SearchFor = entry.WantedTitle + " " + authorName
	} else {
		entry.SearchFor = entry.WantedTitle
	}

	return nil
}

// GetNzbID returns the NzbbookID field.
func (h *handler) GetNzbID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.NzbbookID
}

// GetNzbIDP returns the NzbaudiobookID field.
func (h *handler) GetNzbIDP(entry *apiexternal_v2.Nzbwithprio) *uint {
	return &entry.NzbbookID
}

// CheckMediaMatch checks if the entry's BookID matches the source's NzbbookID.
func (h *handler) CheckMediaMatch(source, entry *apiexternal_v2.Nzbwithprio) bool {
	return source.NzbbookID == entry.Info.BookID
}

// GetUnwantedReason returns "unwanted Book".
func (h *handler) GetUnwantedReason() string {
	return "unwanted Book"
}

// GetFoundID returns entry.Info.BookID for logging.
func (h *handler) GetFoundID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.BookID
}

// ValidateRSSIDs validates DbbookID and BookID for RSS processing.
// Returns error reason string if invalid, empty string if valid.
func (h *handler) ValidateRSSIDs(entry *apiexternal_v2.Nzbwithprio) string {
	if entry.Info.DbbookID == 0 {
		return "unwanted DBBook"
	}

	if entry.Info.BookID == 0 {
		return "unwanted Book"
	}

	return ""
}

// SetRSSIDs sets entry.Dbid = DbbookID, entry.NzbbookID = BookID.
func (h *handler) SetRSSIDs(entry *apiexternal_v2.Nzbwithprio) {
	entry.Dbid = entry.Info.DbbookID
	entry.NzbbookID = entry.Info.BookID
}

// GetRSSMediaID returns entry.Info.BookID for getrssdata.
func (h *handler) GetRSSMediaID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.BookID
}

// CheckCorrectID validates that the entry's ISBN matches the source's.
// Returns true if IDs don't match (should skip), false if they match or can't compare.
func (h *handler) CheckCorrectID(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
) (bool, string, string) {
	// Books don't have a standard ID like IMDB/TVDB in NZB results
	// ISBN matching could be added if indexers support it
	return false, "", ""
}

// GetRuntimeBonus returns 0 - books don't have runtime bonus.
func (h *handler) GetRuntimeBonus(info *database.ParseInfo) int {
	return 0
}

// SkipMultipleFiles returns true - books should be single files.
func (h *handler) SkipMultipleFiles() bool {
	return true
}

// FillNotifyData fills notification data for books from dbbooks table.
// Returns title, year. Other fields are empty for books.
func (h *handler) FillNotifyData(
	id *uint,
) (title, year, externalID, series, season, episode, identifier string, ok bool) {
	var dbbook database.Dbbook
	if dbbook.GetDbbookByIDP(id) != nil {
		return "", "", "", "", "", "", "", false
	}

	return dbbook.Title, logger.IntToString(dbbook.Year), dbbook.ISBN13, "", "", "", "", true
}

// FillNamingData fills NamingData for books from the database.
// Returns clearFolder=true for books (folder name should be cleared when rootpath exists).
func (h *handler) FillNamingData(
	dbid *uint,
	videofile string,
	m *database.ParseInfo,
	data *mediatype.NamingData,
) (bool, bool) {
	// Books need a different approach - we'll use Dbmovie fields for compatibility
	// In a full implementation, this would use book-specific data structures
	var dbbook database.Dbbook
	if dbbook.GetDbbookByIDP(dbid) != nil {
		return false, false
	}

	logger.Path(&dbbook.Title, false)

	// Extract title from filename
	data.TitleSource = filepath.Base(videofile)
	data.TitleSource = logger.Trim(data.TitleSource, '.')
	logger.Path(&data.TitleSource, false)
	logger.StringReplaceWithP(&data.TitleSource, '.', ' ')

	return false, true // clearFolder=false for books
}

// GetRefreshIncData returns data for incremental refresh (100 most recently updated ISBN IDs).
func (h *handler) GetRefreshIncData() any {
	return database.GetrowsN[string](
		false,
		100,
		"select distinct dbbooks.isbn_13 from dbbooks inner join books on books.dbbook_id = dbbooks.id where dbbooks.isbn_13 != '' group by dbbooks.isbn_13 order by dbbooks.updated_at desc limit 100",
	)
}

// GetRefreshFullData returns data for full refresh (all ISBN IDs).
func (h *handler) GetRefreshFullData() any {
	return database.GetrowsN[string](
		false,
		database.Getdatarow[uint](false, "select count() from dbbooks where isbn_13 != ''"),
		"select distinct dbbooks.isbn_13 from dbbooks inner join books on books.dbbook_id = dbbooks.id where dbbooks.isbn_13 != '' group by dbbooks.isbn_13",
	)
}

// GetSchedulerJobNames returns the job name pairs for books scheduler configuration.
func (h *handler) GetSchedulerJobNames() [][2]string {
	return [][2]string{
		{logger.StrSearchMissingInc, logger.StrSearchMissingInc},
		{logger.StrSearchMissingFull, logger.StrSearchMissingFull},
		{logger.StrSearchUpgradeInc, logger.StrSearchUpgradeInc},
		{logger.StrSearchUpgradeFull, logger.StrSearchUpgradeFull},
		{logger.StrSearchMissingIncTitle, logger.StrSearchMissingIncTitle},
		{logger.StrSearchMissingFullTitle, logger.StrSearchMissingFullTitle},
		{logger.StrSearchUpgradeIncTitle, logger.StrSearchUpgradeIncTitle},
		{logger.StrSearchUpgradeFullTitle, logger.StrSearchUpgradeFullTitle},
		{logger.StrRss, logger.StrRss},
		{logger.StrRssAuthors, logger.StrRssAuthors},
		{logger.StrRssAuthorsUpgrade, logger.StrRssAuthorsUpgrade},
		{logger.StrDataFull, logger.StrDataFull},
		{logger.StrStructure, logger.StrStructure},
		{logger.StrFeeds, logger.StrFeeds},
		{logger.StrCheckMissing, logger.StrCheckMissing},
		{logger.StrCheckMissingFlag, logger.StrCheckMissingFlag},
		{logger.StrUpgradeFlag, logger.StrUpgradeFlag},
		{"refreshbooksfull", "refresh"},
		{"refreshbooksinc", "refreshinc"},
	}
}

// CleanupAfterRemove handles cleanup after a book file is removed.
func (h *handler) CleanupAfterRemove(
	folder, rootpath, pathCfgName string,
	walkCleanupFn func(string),
	_ func(),
) error {
	if pathCfgName == "" {
		return logger.ErrPathTemplateNotFound
	}

	if !scanner.CheckFileExist(folder) {
		return logger.ErrNotFound
	}

	walkCleanupFn(rootpath)

	return nil
}

// MoveOtherFilesAfterOrganize handles moving additional files after main book is organized.
func (h *handler) MoveOtherFilesAfterOrganize(params *mediatype.MoveOtherFilesParams) error {
	if params.PathCfgName == "" {
		return logger.ErrPathTemplateNotFound
	}

	if !scanner.CheckFileExist(params.Folder) {
		return logger.ErrNotFound
	}

	params.WalkCleanupFn(params.Rootpath, params.TargetPath, params.Filename)
	params.NotifyFn()
	params.CleanupFolderFn()

	return nil
}

// CheckExtensions validates if a file extension is allowed for books.
// Books use ebook extensions (e.g., .epub, .pdf, .mobi).
func (h *handler) CheckExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	return mediatype.CheckBookExtensions(pathcfg, ext)
}

// SupportsIDSearch returns false - books use query-based search only (no IMDB/TVDB).
func (h *handler) SupportsIDSearch() bool { return false }

// SupportsSeasonSearch returns false - books don't have season/episode structure.
func (h *handler) SupportsSeasonSearch() bool { return false }

// RequiresYearCheck returns false - books don't require strict year matching.
func (h *handler) RequiresYearCheck() bool { return false }

// HasSearchID returns false - books use query-based search only (no standard ID in NZB results).
func (h *handler) HasSearchID(_ *apiexternal_v2.Nzbwithprio) bool { return false }

// SupportsAbsoluteEpisode returns false - books don't have episode structure.
func (h *handler) SupportsAbsoluteEpisode() bool { return false }

// HandleRSSListID does nothing for books - list ID is determined by book lookup.
func (h *handler) HandleRSSListID(_ *apiexternal_v2.Nzbwithprio, _ int) {}

// CheckEpisodeMatch returns false - books don't have episode validation.
func (h *handler) CheckEpisodeMatch(
	_, _ *apiexternal_v2.Nzbwithprio,
	_ string,
	_ func(string, *apiexternal_v2.Nzbwithprio),
) bool {
	return false
}

// SupportsVideoFile returns false - books use ebook files, not video.
func (h *handler) SupportsVideoFile() bool { return false }

// GetRuntimeMultiplier returns 1 - books are single items.
func (h *handler) GetRuntimeMultiplier(_ *database.ParseInfo) int { return 1 }

// ShouldCheckOldFilePriority returns false - books don't check file priority.
func (h *handler) ShouldCheckOldFilePriority() bool { return false }

// HasConfiguredExtensions returns true if book extensions are configured.
func (h *handler) HasConfiguredExtensions(pathcfg *config.PathsConfig) bool {
	return pathcfg.AllowedBookExtensionsLen > 0 || pathcfg.AllowedBookExtensionsNoRenameLen > 0
}

// IsExternalIDImdb returns false - books don't use IMDB.
func (h *handler) IsExternalIDImdb() bool { return false }

// UsesGroupedFileProcessing returns false - books are processed individually.
func (h *handler) UsesGroupedFileProcessing() bool { return false }

// GetCacheUnmatchedKey returns the cache key for unmatched books.
func (h *handler) GetCacheUnmatchedKey() string { return logger.CacheUnmatchedBook }

// GetCacheFilesKey returns the cache key for book files.
func (h *handler) GetCacheFilesKey() string { return logger.CacheFilesBook }

// UsesListNameAsQualityProfile returns false - books use quality config name.
func (h *handler) UsesListNameAsQualityProfile() bool { return false }

// GetStringsMap returns a book-specific string for the given key.
func (h *handler) GetStringsMap(key string) string {
	return StringsMap[key]
}

// trimStringInclAfterString trims the input string starting from the first occurrence of trim.
func trimStringInclAfterString(input, trim string) string {
	if trim == "" {
		return input
	}

	idx := logger.IndexI(input, trim)
	if idx != -1 {
		return input[:idx]
	}

	return input
}

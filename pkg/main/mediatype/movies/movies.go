// Package movies implements the mediatype.Handler interface for movie media.
// It registers itself automatically via init() and provides all movie-specific
// logic for searching, parsing, organizing, and processing movie files.
package movies

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

// handler implements mediatype.Handler for movies.
type handler struct {
	// Registered function implementations - set by other packages to avoid circular imports
	// organizeFunc mediatype.OrganizeFunc
	// importParseFunc mediatype.ImportParseFunc
	refreshFunc mediatype.RefreshFunc
	// importNewFunc   mediatype.ImportNewFunc
	// initialFillFunc mediatype.InitialFillFunc
	dataFullFunc mediatype.DataFullFunc
}

var (
	// Handler is the singleton instance.
	Handler = &handler{}

	// StringsMap contains all movie-specific string mappings for SQL queries, cache names, etc.
	// Exported so mtstrings package can access it without going through Handler (avoids import cycle).
	StringsMap = map[string]string{
		"CacheDBMedia":             "CacheDBMovie",
		"DBCountDBMedia":           "select count() from dbmovies",
		"DBCacheDBMedia":           "select title, slug, imdb_id, year, id from dbmovies",
		"CacheMedia":               "CacheMovie",
		"DBCountMedia":             "select count() from movies",
		"DBCacheMedia":             "select lower(listname), dbmovie_id, id from movies",
		"CacheHistoryTitle":        "CacheHistoryTitleMovie",
		"CacheHistoryUrl":          "CacheHistoryUrlMovie",
		"DBHistoriesUrl":           "select distinct url from movie_histories",
		"DBHistoriesTitle":         "select distinct title from movie_histories",
		"DBCountHistoriesUrl":      "select count() from (select distinct url from movie_histories)",
		"DBCountHistoriesTitle":    "select count() from (select distinct title from movie_histories)",
		"CacheMediaTitles":         "CacheTitlesMovie",
		"DBCountDBTitles":          "select count() from dbmovie_titles where title != ''",
		"DBCacheDBTitles":          "select title, slug, dbmovie_id from dbmovie_titles where title != ''",
		"CacheFiles":               "CacheFilesMovie",
		"DBCountFiles":             "select count() from movie_files",
		"DBCacheFiles":             "select location from movie_files",
		"CacheUnmatched":           "CacheUnmatchedMovie",
		"DBCountUnmatched":         "select count() from movie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBRemoveUnmatched":        "delete from movie_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		"DBCacheUnmatched":         "select filepath from movie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBCountFilesLocation":     "select count() from movie_files where location = ?",
		"DBCountUnmatchedPath":     "select count() from movie_file_unmatcheds where filepath = ?",
		"DBCountDBTitlesDBID":      "select count() from (select distinct title, slug from dbmovie_titles where dbmovie_id = ? and title != '')",
		"DBDistinctDBTitlesDBID":   "select distinct title, slug, dbmovie_id from dbmovie_titles where dbmovie_id = ? and title != ''",
		"DBMediaTitlesID":          "select year, title, slug from dbmovies where id = ?",
		"DBFilesQuality":           "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?",
		"DBCountFilesByList":       "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
		"DBLocationFilesByList":    "select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
		"DBIDsFilesByLocation":     "select location, id, movie_id from movie_files",
		"DBCountFilesByMediaID":    "select count() from movie_files where movie_id = ?",
		"DBCountFilesByLocation":   "select count() from movie_files",
		"TableFiles":               "movie_files",
		"TableMedia":               "movies",
		"DBCountMediaByList":       "select count() from movies where listname = ? COLLATE NOCASE",
		"DBIDMissingMediaByList":   "select id,missing from movies where listname = ? COLLATE NOCASE",
		"DBUpdateMissing":          "update movies set missing = ? where id = ?",
		"DBListnameByMediaID":      "select listname from movies where id = ?",
		"DBRootPathFromMediaID":    "select rootpath from movies where id = ?",
		"DBDeleteFileByIDLocation": "delete from movie_files where movie_id = ? and location = ?",
		"DBCountHistoriesByTitle":  "select count() from movie_histories where title = ?",
		"DBCountHistoriesByUrl":    "select count() from movie_histories where url = ?",
		"DBLocationIDFilesByID":    "select location, id from movie_files where movie_id = ?",
		"DBFilePrioFilesByID":      "select location, movie_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from movie_files where movie_id = ?",
		"UpdateMediaLastscan":      "update movies set lastscan = datetime('now','localtime') where id = ?",
		"DBQualityMediaByID":       "select quality_profile from movies where id = ?",
		"SearchGenSelect":          "select movies.quality_profile, movies.id ",
		"SearchGenTable":           " from movies inner join dbmovies on dbmovies.id=movies.dbmovie_id where ",
		"SearchGenMissing":         "dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?",
		"SearchGenMissingEnd":      ")",
		"SearchGenReached":         "dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
		"SearchGenLastScan":        " and (movies.lastscan is null or movies.Lastscan < ?)",
		"SearchGenDate":            " and (dbmovies.release_date < ? or dbmovies.release_date is null)",
		"SearchGenOrder":           " order by movies.Lastscan asc",
		"DBIDUnmatchedPathList":    "select id from movie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		"InsertUnmatched":          "Insert into movie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		"UpdateUnmatched":          "update movie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		"GetRSSData":               "select movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?",
		"GetOrganizeData":          "select dbmovie_id, rootpath, listname from movies where id = ?",
		"ClearHistoryByList":       "delete from movie_histories where movie_id in (Select id from movies where listname = ? COLLATE NOCASE)",
		"QueryMediaByList":         "select id, quality_reached, quality_profile from movies where listname = ? COLLATE NOCASE",
		"QueryMediaCountByList":    "select count() from movies where listname = ? COLLATE NOCASE",
		"UpdateQualityReached":     "update movies set quality_reached = ? where id = ?",
		"SelectRootpath":           "select rootpath from movies where id = ?",
		"InsertFile":               "insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"UpdateMissingByID":        "update movies set missing = 0 where id = ?",
		"UpdateQualityReachedByID": "update movies set quality_reached = ? where id = ?",
		"DeleteUnmatchedByPath":    "delete from movie_file_unmatcheds where filepath = ?",
		"SelectRuntime":            "select runtime from dbmovies where id = ?",
		"InsertFileOrganize":       "insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		"UpdateMissingReached":     "update movies SET missing = 0, quality_reached = ? where id = ?",
	}
)

func init() {
	mediatype.Register(Handler)
	mtstrings.Register(config.MediaTypeMovie, StringsMap)
}

// RegisterOrganize sets the organize function for movies
// func RegisterOrganize(fn mediatype.OrganizeFunc) {
// 	Handler.organizeFunc = fn
// }

// RegisterImportParse sets the import parse function for movies
// func RegisterImportParse(fn mediatype.ImportParseFunc) {
// 	Handler.importParseFunc = fn
// }

// RegisterRefresh sets the refresh function for movies.
func RegisterRefresh(fn mediatype.RefreshFunc) {
	Handler.refreshFunc = fn
}

// RegisterImportNew sets the import new function for movies
// func RegisterImportNew(fn mediatype.ImportNewFunc) {
// 	Handler.importNewFunc = fn
// }

// RegisterInitialFill sets the initial fill function for movies
// func RegisterInitialFill(fn mediatype.InitialFillFunc) {
// 	Handler.initialFillFunc = fn
// }

// RegisterDataFull sets the data full function for movies.
func RegisterDataFull(fn mediatype.DataFullFunc) {
	Handler.dataFullFunc = fn
}

// GetType returns the media type constant for movies.
func (*handler) GetType() uint {
	return config.MediaTypeMovie
}

// GetCategoryName returns the category name for job history.
func (*handler) GetCategoryName() string {
	return logger.StrMovie
}

// GetTableName returns the database table name for movies.
func (*handler) GetTableName() string {
	return "movies"
}

// GetDBIDs retrieves database IDs for the parsed movie info.
// It attempts IMDB lookup first, then falls back to title-based search.
func (*handler) GetDBIDs(info *database.ParseInfo) error {
	// Handle IMDB lookup with padding optimization
	if info.Imdb != "" {
		if !logger.HasPrefixI(info.Imdb, logger.StrTt) {
			sourceimdb := info.Imdb

			paddings := []string{"", "0", "00", "000", "0000"}
			for i := range paddings {
				if len(sourceimdb)+len(paddings[i]) >= 7 && paddings[i] != "" {
					break
				}

				info.Imdb = paddings[i] + sourceimdb
				info.MovieFindDBIDByImdbParser()

				if info.DbmovieID != 0 {
					break
				}
			}
		} else {
			info.MovieFindDBIDByImdbParser()
		}
	}

	if info.DbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}

	return nil
}

// GetDBIDsFull retrieves database IDs for a movie with full search capabilities.
// It attempts IMDB lookup first, then falls back to title-based search,
// and finally finds the movie in configured lists.
func (h *handler) GetDBIDsFull(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowSearchTitle bool,
	addFound bool,
) error {
	// Handle IMDB lookup with padding optimization
	if m.Imdb != "" {
		if !logger.HasPrefixI(m.Imdb, logger.StrTt) {
			sourceimdb := m.Imdb

			paddings := []string{"", "0", "00", "000", "0000"}
			for i := range paddings {
				if len(sourceimdb)+len(paddings[i]) >= 7 && paddings[i] != "" {
					break
				}

				m.Imdb = paddings[i] + sourceimdb
				m.MovieFindDBIDByImdbParser()

				if m.DbmovieID != 0 {
					break
				}
			}
		} else {
			m.MovieFindDBIDByImdbParser()
		}
	}

	// Title-based search if IMDB lookup failed
	if m.DbmovieID == 0 && m.Title != "" && allowSearchTitle && cfgp.Name != "" {
		// Strip title prefixes/postfixes
		for _, lst := range cfgp.ListsMap {
			if lst.TemplateQuality != "" {
				m.StripTitlePrefixPostfixGetQual(lst.CfgQuality)
			}
		}

		m.Title = logger.TrimSpace(m.Title)

		if m.Imdb == "" {
			importfeed.MovieFindImdbIDByTitle(addFound, m, cfgp)
		}

		if m.Imdb != "" && m.DbmovieID == 0 {
			m.MovieFindDBIDByImdbParser()
		}
	}

	if m.DbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}

	// Find movie in lists
	return h.findInLists(m, cfgp)
}

// findInLists attempts to locate a movie in configured media lists by its database ID.
func (*handler) findInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			database.QueryMoviesGetIDByDBIDListname,
			&m.MovieID,
			&m.DbmovieID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.MovieID == 0 && cfgp.Name != "" && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.MovieID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheMovie,
					m.DbmovieID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(
					false,
					"select id from movies where listname = ? COLLATE NOCASE and dbmovie_id = ?",
					&m.MovieID,
					&cfgp.Lists[idx].Name,
					&m.DbmovieID,
				)
			}

			if m.MovieID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.MovieID == 0 {
		return logger.ErrNotFoundMovie
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.MovieID)
	}

	return nil
}

// ValidateIDs checks if all required IDs are set for movies.
func (*handler) ValidateIDs(info *database.ParseInfo) bool {
	return info.MovieID != 0 && info.DbmovieID != 0
}

// SetTempID sets the temporary ID from MovieID.
func (*handler) SetTempID(info *database.ParseInfo) {
	info.TempID = info.MovieID
}

// SetDBID sets the DbmovieID field.
func (*handler) SetDBID(info *database.ParseInfo, dbid uint) {
	info.DbmovieID = dbid
}

// GetDBID returns the DbmovieID field.
func (*handler) GetDBID(info *database.ParseInfo) uint {
	return info.DbmovieID
}

// GetMediaID returns the MovieID.
func (*handler) GetMediaID(info *database.ParseInfo) uint {
	return info.MovieID
}

// SetMediaID sets the MovieID.
func (*handler) SetMediaID(info *database.ParseInfo, id uint) {
	info.MovieID = id
}

// GetListID retrieves the list ID for the movie.
func (*handler) GetListID(cfgp *config.MediaTypeConfig, info *database.ParseInfo) int {
	if info.MovieID != 0 {
		return database.GetMediaListIDGetListname(cfgp, &info.MovieID)
	}

	return -1
}

// ClearUntrustedID clears the IMDB ID if indexer is not trusted.
func (*handler) ClearUntrustedID(entry *apiexternal_v2.Nzbwithprio) {
	if !entry.NZB.Indexer.TrustWithIMDBIDs {
		entry.Info.Imdb = ""
	}
}

// SetNzbID sets the NzbmovieID field.
func (*handler) SetNzbID(entry *apiexternal_v2.Nzbwithprio, mediaid uint) {
	entry.NzbmovieID = mediaid
}

// SetEntryTempID sets the temp ID from NzbmovieID.
func (*handler) SetEntryTempID(entry *apiexternal_v2.Nzbwithprio) {
	entry.Info.TempID = entry.NzbmovieID
}

// PerformIDSearch executes a search by IMDB ID.
func (*handler) PerformIDSearch(
	indcfg *config.IndexersConfig,
	quality *config.QualityConfig,
	entry *apiexternal_v2.Nzbwithprio,
	cats int,
	raw *apiexternal.NzbSlice,
) error {
	if entry.Info.Imdb == "" {
		return nil
	}

	_, _, err := apiexternal.QueryNewznabMovieImdb(
		indcfg, quality, logger.Trim(entry.Info.Imdb, 't'), cats, raw,
	)

	return err
}

// ClearUnmatchedCache removes the file from the movie unmatched cache.
func (*handler) ClearUnmatchedCache(fpath string) {
	database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, fpath)
}

// ShortenYearPattern returns true - movies shorten all patterns including year.
func (*handler) ShortenYearPattern() bool {
	return true
}

// GenerateIdentifier does nothing for movies - movies don't have episode identifiers.
func (h *handler) GenerateIdentifier(_ *database.ParseInfo, onlyIfEmpty bool) {
	// Movies don't have identifiers like series do
}

// GetSchedulerRssSeasons returns empty strings for movies - no RSS seasons jobs.
func (*handler) GetSchedulerRssSeasons(
	_ *config.SchedulerConfig,
	_ string,
) (string, string) {
	return "", ""
}

// GetSchedulerRssArtistsAuthors returns empty strings for movies - no artist/author jobs.
func (*handler) GetSchedulerRssArtistsAuthors(
	_ *config.SchedulerConfig,
	_ string,
) (string, string) {
	return "", ""
}

// Organize organizes a movie file into the proper folder structure
// func (h *handler) Organize(org any, data any, info *database.ParseInfo, qualcfg *config.QualityConfig, deleteWrongLang, checkRuntime bool) error {
// 	if h.organizeFunc != nil {
// 		return h.organizeFunc(org, data, info, qualcfg, deleteWrongLang, checkRuntime)
// 	}
// 	return nil
// }

// ImportParse imports and parses a movie file
// func (h *handler) ImportParse(info *database.ParseInfo, fpath string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, addFound bool) error {
// 	if h.importParseFunc != nil {
// 		return h.importParseFunc(info, fpath, cfgp, list, addFound)
// 	}
// 	return nil
// }

// Refresh refreshes movie data.
func (h *handler) Refresh(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if h.refreshFunc != nil {
		return h.refreshFunc(ctx, cfgp, data)
	}

	return nil
}

// ImportNew imports new movies from feeds
// func (h *handler) ImportNew(ctx context.Context, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int) error {
// 	if h.importNewFunc != nil {
// 		return h.importNewFunc(ctx, cfgp, list, listid)
// 	}
// 	return nil
// }

// InitialFill performs initial database fill for movies
// func (h *handler) InitialFill() {
// 	if h.initialFillFunc != nil {
// 		h.initialFillFunc()
// 	}
// }

// DataFull performs full data refresh for movies.
func (h *handler) DataFull() {
	if h.dataFullFunc != nil {
		h.dataFullFunc()
	}
}

// SearchConfigByName returns nil, false for movies - config file search is not supported.
func (*handler) SearchConfigByName(
	_ string,
	_ *config.MediaListsConfig,
) (*config.ManualConfig, bool) {
	return nil, false
}

// RecordDownloadHistory records a movie download in the movie_histories table.
func (*handler) RecordDownloadHistory(nzb *apiexternal_v2.Nzbwithprio, targetPath string) error {
	var (
		movieID, dbmovieID uint
		qualityProfile     string
	)

	if nzb.NzbmovieID != 0 {
		movieID = nzb.NzbmovieID
		database.Scanrowsdyn(
			false,
			"select quality_profile from movies where id = ?",
			&qualityProfile,
			&nzb.NzbmovieID,
		)
		database.Scanrowsdyn(
			false,
			"select dbmovie_id from movies where id = ?",
			&dbmovieID,
			&nzb.NzbmovieID,
		)
	}

	database.ExecN(
		"Insert into movie_histories (title, url, target, indexer, downloaded_at, movie_id, dbmovie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?, ?, ?, ?, ?)",
		&nzb.NZB.Title,
		&nzb.NZB.DownloadURL,
		&targetPath,
		&nzb.NZB.Indexer.Name,
		&movieID,
		&dbmovieID,
		&nzb.Info.ResolutionID,
		&nzb.Info.QualityID,
		&nzb.Info.CodecID,
		&nzb.Info.AudioID,
		&qualityProfile,
	)

	return nil
}

// GetDownloadTargetFolder returns the target folder name for a movie download.
// Returns title with IMDB ID (e.g., "Movie Title (tt1234567)").
func (*handler) GetDownloadTargetFolder(nzb *apiexternal_v2.Nzbwithprio, dbImdbID string) string {
	if dbImdbID != "" {
		return logger.JoinStrings(nzb.NZB.Title, " (", dbImdbID, ")")
	}

	if nzb.NZB.IMDBID != "" {
		imdbID := logger.AddImdbPrefix(nzb.NZB.IMDBID)
		if nzb.NZB.Title == "" {
			return logger.JoinStrings(
				nzb.Info.Title,
				"[",
				nzb.Info.Resolution,
				logger.StrSpace,
				nzb.Info.Quality,
				"] (",
				imdbID,
				")",
			)
		}

		return logger.JoinStrings(nzb.NZB.Title, " (", imdbID, ")")
	}

	return ""
}

// GetExternalIDFromDB returns the IMDB ID from the database entity.
// Expects *database.Dbmovie, returns ImdbID.
func (*handler) GetExternalIDFromDB(dbEntity any) string {
	if dbmovie, ok := dbEntity.(*database.Dbmovie); ok {
		return dbmovie.ImdbID
	}

	return ""
}

// FillSearchVar fills search variables from the database for the given movie ID.
// Sets NzbmovieID, loads data from DB, validates required fields.
func (*handler) FillSearchVar(entry *apiexternal_v2.Nzbwithprio, mediaid uint) error {
	entry.NzbmovieID = mediaid
	if mediaid == 0 {
		return logger.ErrNotFoundDbmovie
	}

	database.GetdatarowArgs(
		"select movies.dbmovie_id, movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.year, dbmovies.imdb_id, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?",
		&entry.NzbmovieID,
		&entry.Dbid,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Listname,
		&entry.Quality,
		&entry.Info.Year,
		&entry.Info.Imdb,
		&entry.WantedTitle,
	)

	if entry.Dbid == 0 {
		return logger.ErrNotFoundDbmovie
	}

	if entry.DontSearch {
		return logger.ErrDisabled
	}

	if entry.Info.Year == 0 {
		return logger.ErrYearEmpty
	}

	return nil
}

// GetNzbID returns the NzbmovieID field.
func (*handler) GetNzbID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.NzbmovieID
}

// GetNzbIDP returns the NzbaudiobookID field.
func (*handler) GetNzbIDP(entry *apiexternal_v2.Nzbwithprio) *uint {
	return &entry.NzbmovieID
}

// CheckMediaMatch checks if the entry's MovieID matches the source's NzbmovieID.
func (*handler) CheckMediaMatch(source, entry *apiexternal_v2.Nzbwithprio) bool {
	return source.NzbmovieID == entry.Info.MovieID
}

// GetUnwantedReason returns "unwanted Movie".
func (*handler) GetUnwantedReason() string {
	return "unwanted Movie"
}

// GetFoundID returns entry.Info.MovieID for logging.
func (*handler) GetFoundID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.MovieID
}

// ValidateRSSIDs validates DbmovieID and MovieID for RSS processing.
// Returns error reason string if invalid, empty string if valid.
func (*handler) ValidateRSSIDs(entry *apiexternal_v2.Nzbwithprio) string {
	if entry.Info.DbmovieID == 0 {
		return "unwanted DBMovie"
	}

	if entry.Info.MovieID == 0 {
		return "unwanted Movie"
	}

	return ""
}

// SetRSSIDs sets entry.Dbid = DbmovieID, entry.NzbmovieID = MovieID.
func (*handler) SetRSSIDs(entry *apiexternal_v2.Nzbwithprio) {
	entry.Dbid = entry.Info.DbmovieID
	entry.NzbmovieID = entry.Info.MovieID
}

// GetRSSMediaID returns entry.Info.MovieID for getrssdata.
func (*handler) GetRSSMediaID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.MovieID
}

// CheckCorrectID validates that the entry's IMDB ID matches the source's.
// Returns true if IDs don't match (should skip), false if they match or can't compare.
// Also returns found and wanted ID strings for logging.
func (*handler) CheckCorrectID(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
) (bool, string, string) {
	if entry.NZB.IMDBID == "" || entry.NZB.IMDBID == "tt0000000" ||
		sourceentry.Info.Imdb == "" {
		return false, "", ""
	}

	if sourceentry.Info.Imdb == entry.NZB.IMDBID {
		return false, "", ""
	}

	// Compare with prefix/zero trimming
	if logger.TrimLeft(sourceentry.Info.Imdb, 't', '0') ==
		logger.TrimLeft(entry.NZB.IMDBID, 't', '0') {
		return false, "", ""
	}

	entry.Reason = "not matched imdb"

	return true, entry.NZB.IMDBID, sourceentry.Info.Imdb
}

// GetRuntimeBonus returns 10 for extended editions, 0 otherwise.
func (*handler) GetRuntimeBonus(info *database.ParseInfo) int {
	if info.Extended {
		return 10
	}

	return 0
}

// SkipMultipleFiles returns true - movies should be single files.
func (*handler) SkipMultipleFiles() bool {
	return true
}

// FillNotifyData fills notification data for movies from dbmovies table.
// Returns title, year, imdb. Series/season/episode/identifier are empty for movies.
func (h *handler) FillNotifyData(
	id *uint,
) (title, year, externalID, series, season, episode, identifier string, ok bool) {
	var dbmovie database.Dbmovie
	if dbmovie.GetDbmovieByIDP(id) != nil {
		return "", "", "", "", "", "", "", false
	}

	return dbmovie.Title, logger.IntToString(dbmovie.Year), dbmovie.ImdbID, "", "", "", "", true
}

// FillNamingData fills NamingData for movies from the database.
// Handles Dbmovie lookup, TitleSource extraction, IMDB prefix, and source field updates.
// Returns clearFolder=true for movies (folder name should be cleared when rootpath exists).
func (*handler) FillNamingData(
	dbid *uint,
	videofile string,
	m *database.ParseInfo,
	data *mediatype.NamingData,
) (bool, bool) {
	if data.Dbmovie.GetDbmovieByIDP(dbid) != nil {
		return false, false
	}

	logger.Path(&data.Dbmovie.Title, false)

	// Extract title from filename
	data.TitleSource = filepath.Base(videofile)
	data.TitleSource = trimStringInclAfterString(data.TitleSource, m.Quality)
	data.TitleSource = trimStringInclAfterString(data.TitleSource, m.Resolution)

	if m.Year != 0 {
		idx := strings.Index(data.TitleSource, logger.IntToString(m.Year))
		if idx != -1 {
			data.TitleSource = data.TitleSource[:idx]
		}
	}

	data.TitleSource = logger.Trim(data.TitleSource, '.')
	logger.Path(&data.TitleSource, false)
	logger.StringReplaceWithP(&data.TitleSource, '.', ' ')

	// Fallback title lookup
	if data.Dbmovie.Title == "" {
		database.Scanrowsdyn(
			false,
			database.QueryDbmovieTitlesGetTitleByIDLmit1,
			&data.Dbmovie.Title,
			dbid,
		)

		if data.Dbmovie.Title == "" {
			data.Dbmovie.Title = data.TitleSource
		}
	}

	if data.Dbmovie.Year == 0 {
		data.Dbmovie.Year = m.Year
	}

	// Update source IMDB
	if m.Imdb == "" {
		m.Imdb = data.Dbmovie.ImdbID
	}

	if m.Imdb != "" {
		m.Imdb = logger.AddImdbPrefix(m.Imdb)
	}

	logger.StringRemoveAllRunesP(&m.Title, '/')
	logger.Path(&m.Title, false)

	return true, true // clearFolder=true for movies
}

// refreshListArgs returns the list-name bind arguments for cfgp, matching the
// "listname in (?" + cfgp.ListsQu + ")" placeholder count.
func refreshListArgs(cfgp *config.MediaTypeConfig) []any {
	args := make([]any, 0, len(cfgp.Lists))
	for i := range cfgp.Lists {
		args = append(args, &cfgp.Lists[i].Name)
	}

	return args
}

// GetRefreshIncData returns data for incremental refresh (100 most recently updated
// IMDB IDs in the config's lists).
func (*handler) GetRefreshIncData(cfgp *config.MediaTypeConfig) any {
	if cfgp == nil || cfgp.ListsLen == 0 {
		return []string(nil)
	}

	args := refreshListArgs(cfgp)

	return database.GetrowsN[string](
		false,
		100,
		"select distinct dbmovies.imdb_id from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where movies.listname in (?"+cfgp.ListsQu+") group by dbmovies.imdb_id order by dbmovies.updated_at desc limit 100",
		args...,
	)
}

// GetRefreshFullData returns data for full refresh (all IMDB IDs in the config's lists).
func (*handler) GetRefreshFullData(cfgp *config.MediaTypeConfig) any {
	if cfgp == nil || cfgp.ListsLen == 0 {
		return []string(nil)
	}

	args := refreshListArgs(cfgp)
	join := "from dbmovies inner join movies on movies.dbmovie_id = dbmovies.id where movies.listname in (?" + cfgp.ListsQu + ")"

	return database.GetrowsN[string](
		false,
		database.Getdatarow[uint](false, "select count(distinct dbmovies.imdb_id) "+join, args...),
		"select distinct dbmovies.imdb_id "+join+" group by dbmovies.imdb_id",
		args...,
	)
}

// GetSchedulerJobNames returns the job name pairs for movies scheduler configuration.
// Each pair is [schedulerJobName, singleJobName].
func (*handler) GetSchedulerJobNames() [][2]string {
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
		{logger.StrDataFull, logger.StrDataFull},
		{logger.StrStructure, logger.StrStructure},
		{logger.StrFeeds, logger.StrFeeds},
		{logger.StrCheckMissing, logger.StrCheckMissing},
		{logger.StrCheckMissingFlag, logger.StrCheckMissingFlag},
		{logger.StrUpgradeFlag, logger.StrUpgradeFlag},
		{"refreshmoviesfull", "refresh"},
		{"refreshmoviesinc", "refreshinc"},
	}
}

// CleanupAfterRemove handles cleanup after a movie file is removed.
// For movies: validates path config and folder, then calls walkcleanup on the rootpath.
func (*handler) CleanupAfterRemove(
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

// MoveOtherFilesAfterOrganize handles moving additional files after main movie is organized.
// For movies: validates path config and folder, calls walkcleanup, then notify and cleanup.
func (*handler) MoveOtherFilesAfterOrganize(params *mediatype.MoveOtherFilesParams) error {
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

// CheckExtensions validates if a file extension is allowed for movies.
// Movies use video extensions (e.g., .mkv, .mp4, .avi).
func (h *handler) CheckExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	return mediatype.CheckVideoExtensions(pathcfg, ext)
}

// SupportsIDSearch returns true - movies support IMDB ID-based search.
func (*handler) SupportsIDSearch() bool { return true }

// SupportsSeasonSearch returns false - movies don't have season/episode structure.
func (*handler) SupportsSeasonSearch() bool { return false }

// RequiresYearCheck returns true - movies require strict year matching in searches.
func (*handler) RequiresYearCheck() bool { return true }

// HasSearchID returns true if the movie entry has a valid IMDB ID.
func (*handler) HasSearchID(entry *apiexternal_v2.Nzbwithprio) bool {
	return entry.Info.Imdb != ""
}

// SupportsAbsoluteEpisode returns false - movies don't have episode structure.
func (*handler) SupportsAbsoluteEpisode() bool { return false }

// HandleRSSListID sets the ListID for movie RSS entries.
func (*handler) HandleRSSListID(entry *apiexternal_v2.Nzbwithprio, addinlistid int) {
	if addinlistid != -1 {
		entry.Info.ListID = addinlistid
	}
}

// CheckEpisodeMatch returns false - movies don't have episode validation.
func (*handler) CheckEpisodeMatch(
	_, _ *apiexternal_v2.Nzbwithprio,
	_ string,
	_ func(string, *apiexternal_v2.Nzbwithprio),
) bool {
	return false
}

// SupportsVideoFile returns true - movies use video files.
func (*handler) SupportsVideoFile() bool { return true }

// GetRuntimeMultiplier returns 1 - movies are single files.
func (*handler) GetRuntimeMultiplier(_ *database.ParseInfo) int { return 1 }

// ShouldCheckOldFilePriority returns true - movies check existing file quality before organizing.
func (*handler) ShouldCheckOldFilePriority() bool { return true }

// HasConfiguredExtensions returns true if video extensions are configured.
func (*handler) HasConfiguredExtensions(pathcfg *config.PathsConfig) bool {
	return pathcfg.AllowedVideoExtensionsLen > 0 || pathcfg.AllowedVideoExtensionsNoRenameLen > 0
}

// IsExternalIDImdb returns true - movies use IMDB for external IDs.
func (*handler) IsExternalIDImdb() bool { return true }

// UsesGroupedFileProcessing returns false - movies are processed individually.
func (*handler) UsesGroupedFileProcessing() bool { return false }

// GetCacheUnmatchedKey returns the cache key for unmatched movies.
func (*handler) GetCacheUnmatchedKey() string { return logger.CacheUnmatchedMovie }

// GetCacheFilesKey returns the cache key for movie files.
func (*handler) GetCacheFilesKey() string { return logger.CacheFilesMovie }

// UsesListNameAsQualityProfile returns false - movies use quality config name.
func (*handler) UsesListNameAsQualityProfile() bool { return false }

// GetStringsMap returns a movie-specific string for the given key.
func (*handler) GetStringsMap(key string) string {
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

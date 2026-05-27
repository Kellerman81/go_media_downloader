// Package audiobooks implements the mediatype.Handler interface for audiobook media.
// It registers itself automatically via init() and provides all audiobook-specific
// logic for searching, parsing, organizing, and processing audiobook files.
package audiobooks

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/pelletier/go-toml/v2"
)

// handler implements mediatype.Handler for audiobooks.
type handler struct {
	refreshFunc  mediatype.RefreshFunc
	dataFullFunc mediatype.DataFullFunc
}

var (
	// Handler is the singleton instance.
	Handler = &handler{}

	// StringsMap contains all audiobook-specific string mappings for SQL queries, cache names, etc.
	StringsMap = map[string]string{
		"CacheDBMedia":             "CacheDBAudiobook",
		"DBCountDBMedia":           "select count() from dbaudiobooks",
		"DBCacheDBMedia":           "select title, slug, asin, year, id from dbaudiobooks",
		"CacheMedia":               "CacheAudiobook",
		"DBCountMedia":             "select count() from audiobooks",
		"DBCacheMedia":             "select lower(listname), dbaudiobook_id, id from audiobooks",
		"CacheRootpath":            "CacheRootpathAudiobook",
		"DBCountRootpath":          "select count() from (select distinct rootpath from audiobooks where rootpath != '' and exists (select 1 from audiobook_files where audiobook_id = audiobooks.id))",
		"DBCacheRootpath":          "select distinct rootpath from audiobooks where rootpath != '' and exists (select 1 from audiobook_files where audiobook_id = audiobooks.id)",
		"CacheHistoryTitle":        "CacheHistoryTitleAudiobook",
		"CacheHistoryUrl":          "CacheHistoryUrlAudiobook",
		"DBHistoriesUrl":           "select distinct url from audiobook_histories",
		"DBHistoriesTitle":         "select distinct title from audiobook_histories",
		"DBCountHistoriesUrl":      "select count() from (select distinct url from audiobook_histories)",
		"DBCountHistoriesTitle":    "select count() from (select distinct title from audiobook_histories)",
		"CacheMediaTitles":         "CacheTitlesAudiobook",
		"DBCountDBTitles":          "select count() from dbaudiobook_titles where title != ''",
		"DBCacheDBTitles":          "select title, slug, dbaudiobook_id from dbaudiobook_titles where title != ''",
		"CacheFiles":               "CacheFilesAudiobook",
		"DBCountFiles":             "select count() from audiobook_files",
		"DBCacheFiles":             "select location from audiobook_files",
		"CacheUnmatched":           "CacheUnmatchedAudiobook",
		"DBCountUnmatched":         "select count() from audiobook_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBRemoveUnmatched":        "delete from audiobook_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		"DBCacheUnmatched":         "select filepath from audiobook_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBCountFilesLocation":     "select count() from audiobook_files where location = ?",
		"DBCountUnmatchedPath":     "select count() from audiobook_file_unmatcheds where filepath = ?",
		"DBCountDBTitlesDBID":      "select count() from (select distinct title, slug from dbaudiobook_titles where dbaudiobook_id = ? and title != '')",
		"DBDistinctDBTitlesDBID":   "select distinct title, slug, dbaudiobook_id from dbaudiobook_titles where dbaudiobook_id = ? and title != ''",
		"DBMediaTitlesID":          "select year, title, slug from dbaudiobooks where id = ?",
		"DBFilesQuality":           "select 0, 0, 0, 0, 0, 0, 0 from audiobook_files where id = ?",
		"DBCountFilesByList":       "select count() from audiobook_files where audiobook_id in (select id from audiobooks where listname = ? COLLATE NOCASE)",
		"DBLocationFilesByList":    "select location from audiobook_files where audiobook_id in (select id from audiobooks where listname = ? COLLATE NOCASE)",
		"DBIDsFilesByLocation":     "select location, id, audiobook_id from audiobook_files",
		"DBCountFilesByMediaID":    "select count() from audiobook_files where audiobook_id = ?",
		"DBCountFilesByLocation":   "select count() from audiobook_files",
		"TableFiles":               "audiobook_files",
		"TableMedia":               "audiobooks",
		"DBCountMediaByList":       "select count() from audiobooks where listname = ? COLLATE NOCASE",
		"DBIDMissingMediaByList":   "select id,missing from audiobooks where listname = ? COLLATE NOCASE",
		"DBUpdateMissing":          "update audiobooks set missing = ? where id = ?",
		"DBListnameByMediaID":      "select listname from audiobooks where id = ?",
		"DBRootPathFromMediaID":    "select rootpath from audiobooks where id = ?",
		"DBDeleteFileByIDLocation": "delete from audiobook_files where audiobook_id = ? and location = ?",
		"DBCountHistoriesByTitle":  "select count() from audiobook_histories where title = ?",
		"DBCountHistoriesByUrl":    "select count() from audiobook_histories where url = ?",
		"DBLocationIDFilesByID":    "select location, id from audiobook_files where audiobook_id = ?",
		"DBFilePrioFilesByID":      "select location, audiobook_id, id, 0, 0, 0, 0, 0, 0, 0 from audiobook_files where audiobook_id = ?",
		"DBAudioFilePrioFilesByID": "select location, audiobook_id, id, format, bitrate, 0, 0 from audiobook_files where audiobook_id = ?",
		"UpdateMediaLastscan":      "update audiobooks set lastscan = datetime('now','localtime') where id = ?",
		"DBQualityMediaByID":       "select quality_profile from audiobooks where id = ?",
		"SearchGenSelect":          "select audiobooks.quality_profile, audiobooks.id ",
		"SearchGenTable":           " from audiobooks inner join dbaudiobooks on dbaudiobooks.id=audiobooks.dbaudiobook_id where ",
		"SearchGenMissing":         "audiobooks.missing = 1 and audiobooks.listname in (?",
		"SearchGenMissingEnd":      ")",
		"SearchGenReached":         "quality_reached = 0 and missing = 0 and listname in (?",
		"SearchGenLastScan":        " and (audiobooks.lastscan is null or audiobooks.Lastscan < ?)",
		"SearchGenDate":            " and (dbaudiobooks.release_date < ? or dbaudiobooks.release_date is null)",
		"SearchGenOrder":           " order by audiobooks.Lastscan asc",
		"DBIDUnmatchedPathList":    "select id from audiobook_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		"InsertUnmatched":          "Insert into audiobook_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		"UpdateUnmatched":          "update audiobook_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		"GetRSSData":               "select audiobooks.dont_search, audiobooks.dont_upgrade, audiobooks.listname, audiobooks.quality_profile, dbaudiobooks.title from audiobooks inner join dbaudiobooks ON dbaudiobooks.id=audiobooks.dbaudiobook_id where audiobooks.id = ?",
		"GetOrganizeData":          "select dbaudiobook_id, rootpath, listname from audiobooks where id = ?",
		"ClearHistoryByList":       "delete from audiobook_histories where audiobook_id in (Select id from audiobooks where listname = ? COLLATE NOCASE)",
		"QueryMediaByList":         "select id, quality_reached, quality_profile from audiobooks where listname = ? COLLATE NOCASE",
		"QueryMediaCountByList":    "select count() from audiobooks where listname = ? COLLATE NOCASE",
		"UpdateQualityReached":     "update audiobooks set quality_reached = ? where id = ?",
		"SelectRootpath":           "select rootpath from audiobooks where id = ?",
		"InsertFile":               "insert into audiobook_files (location, filename, extension, quality_profile, audiobook_id, dbaudiobook_id) values (?, ?, ?, ?, ?, ?)",
		"UpdateMissingByID":        "update audiobooks set missing = 0 where id = ?",
		"UpdateQualityReachedByID": "update audiobooks set quality_reached = ? where id = ?",
		"DeleteUnmatchedByPath":    "delete from audiobook_file_unmatcheds where filepath = ?",
		"SelectRuntime":            "select runtime_minutes from dbaudiobooks where id = ?",
		"InsertFileOrganize":       "insert into audiobook_files (location, filename, extension, quality_profile, audiobook_id, dbaudiobook_id) values (?, ?, ?, ?, ?, ?)",
		"UpdateMissingReached":     "update audiobooks SET missing = 0, quality_reached = ? where id = ?",
		// Author-based search queries
		"SearchAuthorsMissing":    "SELECT DISTINCT da.name, da.id FROM dbauthors da JOIN authors auth ON da.id = auth.dbauthor_id JOIN audiobooks ab ON ab.author_id = auth.id WHERE auth.track_mode != 'none' AND auth.dont_search = 0 AND ab.missing = 1 AND ab.listname IN (?",
		"SearchAuthorsMissingEnd": ") ORDER BY RANDOM() LIMIT 20",
		"SearchAuthorsUpgrade":    "SELECT DISTINCT da.name, da.id FROM dbauthors da JOIN authors auth ON da.id = auth.dbauthor_id JOIN audiobooks ab ON ab.author_id = auth.id WHERE auth.track_mode != 'none' AND auth.dont_search = 0 AND ab.missing = 0 AND ab.quality_reached = 0 AND ab.listname IN (?",
		"SearchAuthorsUpgradeEnd": ") ORDER BY RANDOM() LIMIT 20",
	}
)

func init() {
	mediatype.Register(Handler)
	mtstrings.Register(config.MediaTypeAudiobook, StringsMap)
}

// RegisterRefresh sets the refresh function for audiobooks.
func RegisterRefresh(fn mediatype.RefreshFunc) {
	Handler.refreshFunc = fn
}

// RegisterDataFull sets the data full function for audiobooks.
func RegisterDataFull(fn mediatype.DataFullFunc) {
	Handler.dataFullFunc = fn
}

// GetType returns the media type constant for audiobooks.
func (h *handler) GetType() uint {
	return config.MediaTypeAudiobook
}

// GetCategoryName returns the category name for job history.
func (h *handler) GetCategoryName() string {
	return "audiobook"
}

// GetTableName returns the database table name for audiobooks.
func (h *handler) GetTableName() string {
	return "audiobooks"
}

// GetDBIDs retrieves database IDs for the parsed audiobook info.
// It attempts ASIN lookup first, then falls back to title-based search.
func (h *handler) GetDBIDs(info *database.ParseInfo) error {
	// Handle ASIN lookup
	if info.ASIN != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbaudiobooks where asin = ?",
			&info.DbaudiobookID,
			&info.ASIN,
		)
	}

	if info.DbaudiobookID == 0 {
		return logger.ErrNotFoundDbaudiobook
	}

	return nil
}

// GetDBIDsFull retrieves database IDs for an audiobook with full search capabilities.
// It attempts ASIN lookup first, then falls back to title-based search,
// and finally finds the audiobook in configured lists.
func (h *handler) GetDBIDsFull(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowSearchTitle bool,
	_ bool,
) error {
	// Handle ASIN lookup
	if m.ASIN != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbaudiobooks where asin = ?",
			&m.DbaudiobookID,
			&m.ASIN,
		)
	}

	// Title-based search if ASIN lookup failed
	if m.DbaudiobookID == 0 && m.Title != "" && allowSearchTitle && cfgp.Name != "" {
		// Strip title prefixes/postfixes
		for _, lst := range cfgp.ListsMap {
			if lst.TemplateQuality != "" {
				m.StripTitlePrefixPostfixGetQual(lst.CfgQuality)
			}
		}

		m.Title = logger.TrimSpace(m.Title)

		// Try author-first lookup prioritizing audiobooks in the wanted list
		m.FindDbaudiobookByAuthorFirstFromWantedList(cfgp.ListsNames)

		// Fallback to regular author-first lookup
		if m.DbaudiobookID == 0 {
			m.FindDbaudiobookByAuthorFirst()
		}

		// Fallback to traditional title-based search
		if m.DbaudiobookID == 0 {
			m.FindDbaudiobookByTitle()
		}
	}

	if m.DbaudiobookID == 0 {
		return logger.ErrNotFoundDbaudiobook
	}

	// Find audiobook in lists (if not already found by wanted-list search)
	if m.AudiobookID != 0 {
		return nil
	}

	return h.findInLists(m, cfgp)
}

// findInLists attempts to locate an audiobook in configured media lists by its database ID.
func (h *handler) findInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			"select id from audiobooks where dbaudiobook_id = ? and listname = ? COLLATE NOCASE",
			&m.AudiobookID,
			&m.DbaudiobookID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.AudiobookID == 0 && cfgp.Name != "" && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.AudiobookID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheAudiobook,
					m.DbaudiobookID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(
					false,
					"select id from audiobooks where listname = ? COLLATE NOCASE and dbaudiobook_id = ?",
					&m.AudiobookID,
					&cfgp.Lists[idx].Name,
					&m.DbaudiobookID,
				)
			}

			if m.AudiobookID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.AudiobookID == 0 {
		return logger.ErrNotFoundAudiobook
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.AudiobookID)
	}

	return nil
}

// ValidateIDs checks if all required IDs are set for audiobooks.
func (h *handler) ValidateIDs(info *database.ParseInfo) bool {
	return info.AudiobookID != 0 && info.DbaudiobookID != 0
}

// SetTempID sets the temporary ID from AudiobookID.
func (h *handler) SetTempID(info *database.ParseInfo) {
	info.TempID = info.AudiobookID
}

// SetDBID sets the DbaudiobookID field.
func (h *handler) SetDBID(info *database.ParseInfo, dbid uint) {
	info.DbaudiobookID = dbid
}

// GetDBID returns the DbaudiobookID field.
func (h *handler) GetDBID(info *database.ParseInfo) uint {
	return info.DbaudiobookID
}

// GetMediaID returns the AudiobookID.
func (h *handler) GetMediaID(info *database.ParseInfo) uint {
	return info.AudiobookID
}

// SetMediaID sets the AudiobookID.
func (h *handler) SetMediaID(info *database.ParseInfo, id uint) {
	info.AudiobookID = id
}

// GetListID retrieves the list ID for the audiobook.
func (h *handler) GetListID(cfgp *config.MediaTypeConfig, info *database.ParseInfo) int {
	if info.AudiobookID != 0 {
		return database.GetMediaListIDGetListname(cfgp, &info.AudiobookID)
	}

	return -1
}

// ClearUntrustedID clears the ASIN if indexer is not trusted.
func (h *handler) ClearUntrustedID(entry *apiexternal_v2.Nzbwithprio) {
	// Audiobooks don't have a specific trust flag like IMDB/TVDB
	// Could be extended if needed
}

// SetNzbID sets the NzbaudiobookID field.
func (h *handler) SetNzbID(entry *apiexternal_v2.Nzbwithprio, mediaid uint) {
	entry.NzbaudiobookID = mediaid
}

// SetEntryTempID sets the temp ID from NzbaudiobookID.
func (h *handler) SetEntryTempID(entry *apiexternal_v2.Nzbwithprio) {
	entry.Info.TempID = entry.NzbaudiobookID
}

// PerformIDSearch executes a search - audiobooks use query-based search only.
func (h *handler) PerformIDSearch(
	indcfg *config.IndexersConfig,
	quality *config.QualityConfig,
	entry *apiexternal_v2.Nzbwithprio,
	cats int,
	raw *apiexternal.NzbSlice,
) error {
	// Audiobooks don't support ID-based search like IMDB/TVDB
	// They use query-based search only
	return nil
}

// ClearUnmatchedCache removes the file from the audiobook unmatched cache.
func (h *handler) ClearUnmatchedCache(fpath string) {
	database.SlicesCacheContainsDelete(logger.CacheUnmatchedAudiobook, fpath)
}

// ShortenYearPattern returns true - audiobooks shorten all patterns including year.
func (h *handler) ShortenYearPattern() bool {
	return true
}

// GenerateIdentifier does nothing for audiobooks - audiobooks don't have episode identifiers.
func (h *handler) GenerateIdentifier(info *database.ParseInfo, onlyIfEmpty bool) {
	// Audiobooks don't have identifiers like series do
}

// GetSchedulerRssSeasons returns empty strings for audiobooks - no RSS seasons jobs.
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

// Refresh refreshes audiobook data.
func (h *handler) Refresh(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if h.refreshFunc != nil {
		return h.refreshFunc(ctx, cfgp, data)
	}

	return nil
}

// DataFull performs full data refresh for audiobooks.
func (h *handler) DataFull() {
	if h.dataFullFunc != nil {
		h.dataFullFunc()
	}
}

// SearchConfigByName searches the audiobook config file for matching author name, book series, or alternate names.
// Returns the matching SerieConfig and true if found, or nil and false if not found.
func (h *handler) SearchConfigByName(
	searchName string,
	listCfg *config.MediaListsConfig,
) (*config.ManualConfig, bool) {
	if listCfg.CfgList.ManualConfigFile == "" {
		return nil, false
	}

	configs, err := loadAudiobookConfig(listCfg.CfgList.ManualConfigFile)
	if err != nil {
		logger.Logtype(logger.StatusDebug, 1).
			Err(err).
			Str("config_file", listCfg.CfgList.ManualConfigFile).
			Msg("Failed to load audiobook config file")

		return nil, false
	}

	// Search for matching name, author, or book series in this config file
	for idx := range configs {
		// Check primary name
		if strings.EqualFold(strings.TrimSpace(configs[idx].Name), searchName) {
			return &configs[idx], true
		}

		// Check author name
		if configs[idx].AuthorName != "" &&
			strings.EqualFold(strings.TrimSpace(configs[idx].AuthorName), searchName) {
			return &configs[idx], true
		}

		// Check book series name
		if configs[idx].BookSeriesName != "" &&
			strings.EqualFold(strings.TrimSpace(configs[idx].BookSeriesName), searchName) {
			return &configs[idx], true
		}

		// Check alternate names
		for i := range configs[idx].AlternateName {
			if strings.EqualFold(strings.TrimSpace(configs[idx].AlternateName[i]), searchName) {
				return &configs[idx], true
			}
		}
	}

	return nil, false
}

// loadAudiobookConfig loads audiobook configuration from a TOML file.
func loadAudiobookConfig(file string) ([]config.ManualConfig, error) {
	content, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer content.Close()

	var s config.MainManualConfig

	err = toml.NewDecoder(content).Decode(&s)

	return s.Config, err
}

// RecordDownloadHistory records an audiobook download in the audiobook_histories table.
func (h *handler) RecordDownloadHistory(nzb *apiexternal_v2.Nzbwithprio, targetPath string) error {
	var (
		audiobookID, dbaudiobookID uint
		qualityProfile             string
	)

	if nzb.NzbaudiobookID != 0 {
		audiobookID = nzb.NzbaudiobookID
		database.Scanrowsdyn(
			false,
			"select quality_profile from audiobooks where id = ?",
			&qualityProfile,
			&nzb.NzbaudiobookID,
		)
		database.Scanrowsdyn(
			false,
			"select dbaudiobook_id from audiobooks where id = ?",
			&dbaudiobookID,
			&nzb.NzbaudiobookID,
		)
	}

	database.ExecN(
		"Insert into audiobook_histories (title, url, target, indexer, downloaded_at, audiobook_id, dbaudiobook_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?)",
		&nzb.NZB.Title,
		&nzb.NZB.DownloadURL,
		&targetPath,
		&nzb.NZB.Indexer.Name,
		&audiobookID,
		&dbaudiobookID,
		&qualityProfile,
	)

	return nil
}

// GetDownloadTargetFolder returns the target folder name for an audiobook download.
// Returns title with author (e.g., "Audiobook Title - Author Name").
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

// FillSearchVar fills search variables from the database for the given audiobook ID.
// Sets NzbaudiobookID, loads data from DB, validates required fields.
func (h *handler) FillSearchVar(entry *apiexternal_v2.Nzbwithprio, mediaid uint) error {
	entry.NzbaudiobookID = mediaid
	if mediaid == 0 {
		return logger.ErrNotFoundDbaudiobook
	}

	var (
		authorName                 string
		seriesName, seriesPosition string
	)

	database.GetdatarowArgs(
		"select audiobooks.dbaudiobook_id, audiobooks.dont_search, audiobooks.dont_upgrade, audiobooks.listname, audiobooks.quality_profile, dbaudiobooks.year, dbaudiobooks.title, dbaudiobooks.series_name, dbaudiobooks.series_position from audiobooks inner join dbaudiobooks ON dbaudiobooks.id=audiobooks.dbaudiobook_id where audiobooks.id = ?",
		&entry.NzbaudiobookID,
		&entry.Dbid,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Listname,
		&entry.Quality,
		&entry.Info.Year,
		&entry.WantedTitle,
		&seriesName,
		&seriesPosition,
	)

	if entry.Dbid == 0 {
		return logger.ErrNotFoundDbaudiobook
	}

	if entry.DontSearch {
		return logger.ErrDisabled
	}

	// If series info is available, search by series name + episode number
	if seriesName != "" && seriesPosition != "" {
		entry.SearchFor = seriesName + " " + seriesPosition
		return nil
	}

	// Fall back to "Title Author" search
	database.GetdatarowArgs(
		"select dbauthors.name from dbaudiobook_authors inner join dbauthors ON dbauthors.id=dbaudiobook_authors.dbauthor_id where dbaudiobook_authors.dbaudiobook_id = ? order by dbaudiobook_authors.position limit 1",
		&entry.Dbid,
		&authorName,
	)

	if authorName != "" {
		entry.SearchFor = entry.WantedTitle + " " + authorName
	} else {
		entry.SearchFor = entry.WantedTitle
	}

	return nil
}

// GetNzbID returns the NzbaudiobookID field.
func (h *handler) GetNzbID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.NzbaudiobookID
}

// GetNzbIDP returns the NzbaudiobookID field.
func (h *handler) GetNzbIDP(entry *apiexternal_v2.Nzbwithprio) *uint {
	return &entry.NzbaudiobookID
}

// CheckMediaMatch checks if the entry's AudiobookID matches the source's NzbaudiobookID.
func (h *handler) CheckMediaMatch(source, entry *apiexternal_v2.Nzbwithprio) bool {
	return source.NzbaudiobookID == entry.Info.AudiobookID
}

// GetUnwantedReason returns "unwanted Audiobook".
func (h *handler) GetUnwantedReason() string {
	return "unwanted Audiobook"
}

// GetFoundID returns entry.Info.AudiobookID for logging.
func (h *handler) GetFoundID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.AudiobookID
}

// ValidateRSSIDs validates DbaudiobookID and AudiobookID for RSS processing.
// Returns error reason string if invalid, empty string if valid.
func (h *handler) ValidateRSSIDs(entry *apiexternal_v2.Nzbwithprio) string {
	if entry.Info.DbaudiobookID == 0 {
		return "unwanted DBAudiobook"
	}

	if entry.Info.AudiobookID == 0 {
		return "unwanted Audiobook"
	}

	return ""
}

// SetRSSIDs sets entry.Dbid = DbaudiobookID, entry.NzbaudiobookID = AudiobookID.
func (h *handler) SetRSSIDs(entry *apiexternal_v2.Nzbwithprio) {
	entry.Dbid = entry.Info.DbaudiobookID
	entry.NzbaudiobookID = entry.Info.AudiobookID
}

// GetRSSMediaID returns entry.Info.AudiobookID for getrssdata.
func (h *handler) GetRSSMediaID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.AudiobookID
}

// CheckCorrectID validates that the entry's ASIN matches the source's.
// Returns true if IDs don't match (should skip), false if they match or can't compare.
func (h *handler) CheckCorrectID(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
) (bool, string, string) {
	// Audiobooks don't have a standard ID like IMDB/TVDB in NZB results
	// ASIN matching could be added if indexers support it
	return false, "", ""
}

// GetRuntimeBonus returns 0 - audiobooks don't have runtime bonus.
func (h *handler) GetRuntimeBonus(info *database.ParseInfo) int {
	return 0
}

// SkipMultipleFiles returns false - audiobooks can have multiple files (chapters).
func (h *handler) SkipMultipleFiles() bool {
	return false
}

// FillNotifyData fills notification data for audiobooks from dbaudiobooks table.
// Returns title, year. Other fields are empty for audiobooks.
func (h *handler) FillNotifyData(
	id *uint,
) (title, year, externalID, series, season, episode, identifier string, ok bool) {
	var dbaudiobook database.Dbaudiobook
	if dbaudiobook.GetDbaudiobookByIDP(id) != nil {
		return "", "", "", "", "", "", "", false
	}

	return dbaudiobook.Title, logger.IntToString(
		dbaudiobook.Year,
	), dbaudiobook.ASIN, "", "", "", "", true
}

// FillNamingData fills NamingData for audiobooks from the database.
// Returns clearFolder=true for audiobooks (folder name should be cleared when rootpath exists).
func (h *handler) FillNamingData(
	dbid *uint,
	videofile string,
	m *database.ParseInfo,
	data *mediatype.NamingData,
) (bool, bool) {
	if data.Dbaudiobook.GetDbaudiobookByIDP(dbid) != nil {
		return false, false
	}

	logger.Path(&data.Dbaudiobook.Title, false)

	// Extract track title and number from filename
	data.TitleSource = filepath.Base(videofile)
	data.TitleSource = logger.Trim(data.TitleSource, '.')
	logger.Path(&data.TitleSource, false)
	logger.StringReplaceWithP(&data.TitleSource, '.', ' ')

	data.Title = data.TitleSource

	// Try to extract track number from filename or ParseInfo
	if m != nil && m.Episode > 0 {
		data.Track = m.Episode
	}

	// Get linked book information if available
	if data.Dbaudiobook.DbbookID > 0 {
		if data.Dbbook.GetDbbookByIDP(&data.Dbaudiobook.DbbookID) == nil {
			// Get series information if book is part of a series
			if data.Dbbook.DbbookSeriesID > 0 {
				database.Scanrowsdyn(
					false,
					"SELECT name, description, goodreads_id, openlibrary_id FROM dbbook_series WHERE id = ?",
					&data.BookSeries.Name,
					&data.BookSeries.Description,
					&data.BookSeries.GoodreadsID,
					&data.BookSeries.OpenlibraryID,
					&data.Dbbook.DbbookSeriesID,
				)

				data.BookSeries.ID = data.Dbbook.DbbookSeriesID
			}
		}
	}

	// Get primary author information
	var authorID uint
	database.Scanrowsdyn(false,
		`SELECT dbauthor_id FROM dbaudiobook_authors
		 WHERE dbaudiobook_id = ?
		 ORDER BY position ASC
		 LIMIT 1`,
		&authorID, dbid)

	if authorID > 0 {
		var author database.Dbauthor
		if author.GetDbauthorByIDP(&authorID) == nil {
			data.Author = author
		}
	}

	return false, true // clearFolder=false for audiobooks (they need their own folder)
}

// GetRefreshIncData returns data for incremental refresh (100 most recently updated ASIN IDs).
func (h *handler) GetRefreshIncData() any {
	return database.GetrowsN[string](
		false,
		100,
		"select distinct dbaudiobooks.asin from dbaudiobooks inner join audiobooks on audiobooks.dbaudiobook_id = dbaudiobooks.id where dbaudiobooks.asin != '' group by dbaudiobooks.asin order by dbaudiobooks.updated_at desc limit 100",
	)
}

// GetRefreshFullData returns data for full refresh (all ASIN IDs).
func (h *handler) GetRefreshFullData() any {
	return database.GetrowsN[string](
		false,
		database.Getdatarow[uint](false, "select count() from dbaudiobooks where asin != ''"),
		"select distinct dbaudiobooks.asin from dbaudiobooks inner join audiobooks on audiobooks.dbaudiobook_id = dbaudiobooks.id where dbaudiobooks.asin != '' group by dbaudiobooks.asin",
	)
}

// GetSchedulerJobNames returns the job name pairs for audiobooks scheduler configuration.
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
		{"refreshaudiobooksfull", "refresh"},
		{"refreshaudiobooksinc", "refreshinc"},
	}
}

// CleanupAfterRemove handles cleanup after an audiobook file is removed.
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

// MoveOtherFilesAfterOrganize handles moving additional files after main audiobook is organized.
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

// CheckExtensions validates if a file extension is allowed for audiobooks.
// Audiobooks use audio extensions (e.g., .mp3, .m4a, .flac).
func (h *handler) CheckExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	return mediatype.CheckAudioExtensions(pathcfg, ext)
}

// SupportsIDSearch returns false - audiobooks use query-based search only (no IMDB/TVDB).
func (h *handler) SupportsIDSearch() bool { return false }

// SupportsSeasonSearch returns false - audiobooks don't have season/episode structure.
func (h *handler) SupportsSeasonSearch() bool { return false }

// RequiresYearCheck returns false - audiobooks don't require strict year matching.
func (h *handler) RequiresYearCheck() bool { return false }

// HasSearchID returns false - audiobooks use query-based search only (no standard ID in NZB results).
func (h *handler) HasSearchID(_ *apiexternal_v2.Nzbwithprio) bool { return false }

// SupportsAbsoluteEpisode returns false - audiobooks don't have episode structure.
func (h *handler) SupportsAbsoluteEpisode() bool { return false }

// HandleRSSListID does nothing for audiobooks - list ID is determined by audiobook lookup.
func (h *handler) HandleRSSListID(_ *apiexternal_v2.Nzbwithprio, _ int) {}

// CheckEpisodeMatch returns false - audiobooks don't have episode validation.
func (h *handler) CheckEpisodeMatch(
	_, _ *apiexternal_v2.Nzbwithprio,
	_ string,
	_ func(string, *apiexternal_v2.Nzbwithprio),
) bool {
	return false
}

// SupportsVideoFile returns false - audiobooks use audio files, not video.
func (h *handler) SupportsVideoFile() bool { return false }

// GetRuntimeMultiplier returns 1 - audiobooks are single items.
func (h *handler) GetRuntimeMultiplier(_ *database.ParseInfo) int { return 1 }

// ShouldCheckOldFilePriority returns false - audiobooks don't check file priority.
func (h *handler) ShouldCheckOldFilePriority() bool { return false }

// HasConfiguredExtensions returns true if audio extensions are configured.
func (h *handler) HasConfiguredExtensions(pathcfg *config.PathsConfig) bool {
	return pathcfg.AllowedAudioExtensionsLen > 0 || pathcfg.AllowedAudioExtensionsNoRenameLen > 0
}

// IsExternalIDImdb returns false - audiobooks don't use IMDB.
func (h *handler) IsExternalIDImdb() bool { return false }

// UsesGroupedFileProcessing returns true - audiobooks group audio files by folder.
func (h *handler) UsesGroupedFileProcessing() bool { return true }

// GetCacheUnmatchedKey returns the cache key for unmatched audiobooks.
func (h *handler) GetCacheUnmatchedKey() string { return logger.CacheUnmatchedAudiobook }

// GetCacheFilesKey returns the cache key for audiobook files.
func (h *handler) GetCacheFilesKey() string { return logger.CacheFilesAudiobook }

// UsesListNameAsQualityProfile returns false - audiobooks use quality config name.
func (h *handler) UsesListNameAsQualityProfile() bool { return false }

// GetStringsMap returns an audiobook-specific string for the given key.
func (h *handler) GetStringsMap(key string) string {
	return StringsMap[key]
}

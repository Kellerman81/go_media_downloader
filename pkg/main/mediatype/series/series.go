// Package series implements the mediatype.Handler interface for TV series media.
// It registers itself automatically via init() and provides all series-specific
// logic for searching, parsing, organizing, and processing TV series files.
package series

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/pelletier/go-toml/v2"
)

// handler implements mediatype.Handler for TV series.
type handler struct {
	// Registered function implementations - set by other packages to avoid circular imports
	// organizeFunc mediatype.OrganizeFunc
	// importParseFunc mediatype.ImportParseFunc
	refreshFunc mediatype.RefreshFunc
	// importNewFunc   mediatype.ImportNewFunc
	// initialFillFunc mediatype.InitialFillFunc
	dataFullFunc mediatype.DataFullFunc
}

// Handler is the singleton instance.
var Handler = &handler{}

func init() {
	mediatype.Register(Handler)
	mtstrings.Register(config.MediaTypeSeries, StringsMap)
}

// RegisterOrganize sets the organize function for series
// func RegisterOrganize(fn mediatype.OrganizeFunc) {
// 	Handler.organizeFunc = fn
// }

// // RegisterImportParse sets the import parse function for series
// func RegisterImportParse(fn mediatype.ImportParseFunc) {
// 	Handler.importParseFunc = fn
// }

// RegisterRefresh sets the refresh function for series.
func RegisterRefresh(fn mediatype.RefreshFunc) {
	Handler.refreshFunc = fn
}

// RegisterImportNew sets the import new function for series
// func RegisterImportNew(fn mediatype.ImportNewFunc) {
// 	Handler.importNewFunc = fn
// }

// RegisterInitialFill sets the initial fill function for series
// func RegisterInitialFill(fn mediatype.InitialFillFunc) {
// 	Handler.initialFillFunc = fn
// }

// RegisterDataFull sets the data full function for series.
func RegisterDataFull(fn mediatype.DataFullFunc) {
	Handler.dataFullFunc = fn
}

// GetType returns the media type constant for series.
func (h *handler) GetType() uint {
	return config.MediaTypeSeries
}

// GetCategoryName returns the category name for job history.
func (h *handler) GetCategoryName() string {
	return logger.StrSeries
}

// GetTableName returns the database table name for series.
func (h *handler) GetTableName() string {
	return "series"
}

// GetDBIDs retrieves database IDs for the parsed series info.
// It attempts TVDB lookup first, then falls back to title-based search.
func (h *handler) GetDBIDs(info *database.ParseInfo) error {
	// TVDB lookup
	if info.Tvdb != "" {
		database.Scanrowsdyn(false, database.QueryDbseriesGetIDByTvdb, &info.DbserieID, &info.Tvdb)
	}

	if info.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}

	return nil
}

// GetDBIDsFull retrieves database IDs for a series with full search capabilities.
// It attempts TVDB lookup first, then falls back to title-based search,
// sets the episode ID, and finds the series in configured lists.
func (h *handler) GetDBIDsFull(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowSearchTitle bool,
	_ bool,
) error {
	// TVDB lookup
	if m.Tvdb != "" {
		database.Scanrowsdyn(false, database.QueryDbseriesGetIDByTvdb, &m.DbserieID, &m.Tvdb)
	}

	// Title-based search
	if m.DbserieID == 0 && m.Title != "" && (allowSearchTitle || m.Tvdb == "") {
		if m.Year != 0 {
			titleWithYear := logger.JoinStrings(m.Title, " (", logger.IntToString(m.Year), ")")
			m.FindDbserieByNameWithSlug(titleWithYear)
		}

		if m.DbserieID == 0 {
			m.FindDbserieByNameWithSlug(m.Title)
		}
	}

	// Regex fallback
	if m.DbserieID == 0 && m.File != "" {
		m.RegexGetMatchesStr1(cfgp)
	}

	if m.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}

	// Set episode ID
	m.SetDBEpisodeIDfromM()

	if m.DbserieEpisodeID == 0 {
		return logger.ErrNotFoundDbserieEpisode
	}

	// Find series in lists
	return h.findInLists(m, cfgp)
}

// findInLists attempts to locate a series in configured media lists by its database ID.
func (h *handler) findInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			database.QuerySeriesGetIDByDBIDListname,
			&m.SerieID,
			&m.DbserieID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.SerieID == 0 && cfgp != nil && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.SerieID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheSeries,
					m.DbserieID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(
					false,
					database.QuerySeriesGetIDByDBIDListname,
					&m.SerieID,
					&m.DbserieID,
					&cfgp.Lists[idx].Name,
				)
			}

			if m.SerieID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.SerieID == 0 {
		m.DbserieEpisodeID = 0
		m.SerieEpisodeID = 0
		return logger.ErrNotFoundSerie
	}

	m.SetEpisodeIDfromM()

	if m.SerieEpisodeID == 0 {
		return logger.ErrNotFoundEpisode
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.SerieID)
	}

	return nil
}

// ValidateIDs checks if all required IDs are set for series.
func (h *handler) ValidateIDs(info *database.ParseInfo) bool {
	return info.DbserieEpisodeID != 0 &&
		info.DbserieID != 0 &&
		info.SerieEpisodeID != 0 &&
		info.SerieID != 0
}

// SetTempID sets the temporary ID from SerieID.
func (h *handler) SetTempID(info *database.ParseInfo) {
	info.TempID = info.SerieID
}

// SetDBID sets the DbserieID field.
func (h *handler) SetDBID(info *database.ParseInfo, dbid uint) {
	info.DbserieID = dbid
}

// GetDBID returns the DbserieID field.
func (h *handler) GetDBID(info *database.ParseInfo) uint {
	return info.DbserieID
}

// GetMediaID returns the SerieID.
func (h *handler) GetMediaID(info *database.ParseInfo) uint {
	return info.SerieID
}

// SetMediaID sets the SerieID.
func (h *handler) SetMediaID(info *database.ParseInfo, id uint) {
	info.SerieID = id
}

// GetListID retrieves the list ID for the series.
func (h *handler) GetListID(cfgp *config.MediaTypeConfig, info *database.ParseInfo) int {
	if info.SerieID != 0 {
		return database.GetMediaListIDGetListname(cfgp, &info.SerieID)
	}

	return -1
}

// ClearUntrustedID clears the TVDB ID if indexer is not trusted.
func (h *handler) ClearUntrustedID(entry *apiexternal_v2.Nzbwithprio) {
	if !entry.NZB.Indexer.TrustWithTVDBIDs {
		entry.Info.Tvdb = ""
	}
}

// SetNzbID sets the NzbepisodeID field.
func (h *handler) SetNzbID(entry *apiexternal_v2.Nzbwithprio, mediaid uint) {
	entry.NzbepisodeID = mediaid
}

// SetEntryTempID sets the temp ID from NzbepisodeID.
func (h *handler) SetEntryTempID(entry *apiexternal_v2.Nzbwithprio) {
	entry.Info.TempID = entry.NzbepisodeID
}

// PerformIDSearch executes a search by TVDB ID.
func (h *handler) PerformIDSearch(
	indcfg *config.IndexersConfig,
	quality *config.QualityConfig,
	entry *apiexternal_v2.Nzbwithprio,
	cats int,
	raw *apiexternal.NzbSlice,
) error {
	if entry.NZB.TVDBID == 0 {
		return nil
	}

	_, _, err := apiexternal.QueryNewznabTvTvdb(
		indcfg, quality, entry.NZB.TVDBID, cats,
		entry.NZB.Season, entry.NZB.Episode, true, true, raw,
	)

	return err
}

// ClearUnmatchedCache removes the file from the series unmatched cache.
func (h *handler) ClearUnmatchedCache(fpath string) {
	database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, fpath)
}

// ShortenYearPattern returns false - series does not shorten year pattern.
func (h *handler) ShortenYearPattern() bool {
	return false
}

// GenerateIdentifier generates the episode identifier for series.
func (h *handler) GenerateIdentifier(info *database.ParseInfo, onlyIfEmpty bool) {
	if onlyIfEmpty && info.Identifier != "" {
		return
	}

	if info.Identifier == "" && info.SeasonStr != "" && info.EpisodeStr != "" {
		info.GenerateIdentifierString()
	}

	if info.Date != "" && info.Identifier == "" {
		info.Identifier = info.Date
	}

	info.Identifier = logger.Trim(info.Identifier, '-', '.', ' ')
}

// GetSchedulerRssSeasons returns the interval and cron strings for RSS seasons jobs.
func (h *handler) GetSchedulerRssSeasons(
	scheduler *config.SchedulerConfig,
	jobType string,
) (string, string) {
	switch jobType {
	case logger.StrRssSeasons:
		return scheduler.IntervalIndexerRssSeasons, scheduler.CronIndexerRssSeasons
	case logger.StrRssSeasonsAll:
		return scheduler.IntervalIndexerRssSeasonsAll, scheduler.CronIndexerRssSeasonsAll
	}

	return "", ""
}

// GetSchedulerRssArtistsAuthors returns empty strings for series - no artist/author jobs.
func (h *handler) GetSchedulerRssArtistsAuthors(
	scheduler *config.SchedulerConfig,
	jobType string,
) (string, string) {
	return "", ""
}

// Organize organizes a series file into the proper folder structure
// func (h *handler) Organize(org any, data any, info *database.ParseInfo, qualcfg *config.QualityConfig, deleteWrongLang, checkRuntime bool) error {
// 	if h.organizeFunc != nil {
// 		return h.organizeFunc(org, data, info, qualcfg, deleteWrongLang, checkRuntime)
// 	}
// 	return nil
// }

// ImportParse imports and parses a series file
// func (h *handler) ImportParse(info *database.ParseInfo, fpath string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, addFound bool) error {
// 	if h.importParseFunc != nil {
// 		return h.importParseFunc(info, fpath, cfgp, list, addFound)
// 	}
// 	return nil
// }

// Refresh refreshes series data.
func (h *handler) Refresh(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if h.refreshFunc != nil {
		return h.refreshFunc(ctx, cfgp, data)
	}

	return nil
}

// ImportNew imports new series from feeds
// func (h *handler) ImportNew(ctx context.Context, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int) error {
// 	if h.importNewFunc != nil {
// 		return h.importNewFunc(ctx, cfgp, list, listid)
// 	}
// 	return nil
// }

// InitialFill performs initial database fill for series
// func (h *handler) InitialFill() {
// 	if h.initialFillFunc != nil {
// 		h.initialFillFunc()
// 	}
// }

// DataFull performs full data refresh for series.
func (h *handler) DataFull() {
	if h.dataFullFunc != nil {
		h.dataFullFunc()
	}
}

// SearchConfigByName searches the series config file for matching name or alternate names.
// Returns the matching SerieConfig and true if found, or nil and false if not found.
func (h *handler) SearchConfigByName(
	searchName string,
	listCfg *config.MediaListsConfig,
) (*config.ManualConfig, bool) {
	if listCfg.CfgList.ManualConfigFile == "" {
		return nil, false
	}

	seriesConfigs, err := loadSeriesConfig(listCfg.CfgList.ManualConfigFile)
	if err != nil {
		logger.Logtype(logger.StatusDebug, 1).
			Err(err).
			Str("config_file", listCfg.CfgList.ManualConfigFile).
			Msg("Failed to load series config file")

		return nil, false
	}

	// Search for matching series name in this config file
	for idx := range seriesConfigs {
		if strings.EqualFold(strings.TrimSpace(seriesConfigs[idx].Name), searchName) {
			return &seriesConfigs[idx], true
		}

		// Also check alternate names
		for i := range seriesConfigs[idx].AlternateName {
			if strings.EqualFold(
				strings.TrimSpace(seriesConfigs[idx].AlternateName[i]),
				searchName,
			) {
				return &seriesConfigs[idx], true
			}
		}
	}

	return nil, false
}

// loadSeriesConfig loads series configuration from a TOML file.
func loadSeriesConfig(file string) ([]config.ManualConfig, error) {
	content, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer content.Close()

	var s config.MainManualConfig

	err = toml.NewDecoder(content).Decode(&s)

	return s.Config, err
}

// RecordDownloadHistory records a series episode download in the serie_episode_histories table.
func (h *handler) RecordDownloadHistory(nzb *apiexternal_v2.Nzbwithprio, targetPath string) error {
	var (
		serieID, dbserieID, dbserieEpisodeID uint
		qualityProfile                       string
	)

	if nzb.NzbepisodeID != 0 {
		database.Scanrowsdyn(
			false,
			database.QuerySerieEpisodesGetSerieIDByID,
			&serieID,
			&nzb.NzbepisodeID,
		)
		database.Scanrowsdyn(
			false,
			database.QuerySerieEpisodesGetDBSerieIDByID,
			&dbserieID,
			&nzb.NzbepisodeID,
		)
		database.Scanrowsdyn(
			false,
			"select quality_profile from serie_episodes where id = ?",
			&qualityProfile,
			&nzb.NzbepisodeID,
		)
		database.Scanrowsdyn(
			false,
			database.QuerySerieEpisodesGetDBSerieEpisodeIDByID,
			&dbserieEpisodeID,
			&nzb.NzbepisodeID,
		)
	}

	database.ExecN(
		"Insert into serie_episode_histories (title, url, target, indexer, downloaded_at, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&nzb.NZB.Title,
		&nzb.NZB.DownloadURL,
		&targetPath,
		&nzb.NZB.Indexer.Name,
		&serieID,
		&nzb.NzbepisodeID,
		&dbserieEpisodeID,
		&dbserieID,
		&nzb.Info.ResolutionID,
		&nzb.Info.QualityID,
		&nzb.Info.CodecID,
		&nzb.Info.AudioID,
		&qualityProfile,
	)

	return nil
}

const strTvdbid = " (tvdb"

// GetDownloadTargetFolder returns the target folder name for a series download.
// Returns title with TVDB ID (e.g., "Series Title (tvdb12345)").
func (h *handler) GetDownloadTargetFolder(nzb *apiexternal_v2.Nzbwithprio, dbTvdbID string) string {
	if dbTvdbID != "" {
		if nzb.NZB.Title == "" {
			return logger.JoinStrings(
				nzb.Info.Title,
				"[",
				nzb.Info.Resolution,
				logger.StrSpace,
				nzb.Info.Quality,
				"]",
				strTvdbid,
				dbTvdbID,
				")",
			)
		}

		return logger.JoinStrings(nzb.NZB.Title, strTvdbid, dbTvdbID, ")")
	}

	if nzb.NZB.TVDBID != 0 {
		tvdbStr := strconv.Itoa(nzb.NZB.TVDBID)
		if nzb.NZB.Title == "" {
			return logger.JoinStrings(
				nzb.Info.Title,
				"[",
				nzb.Info.Resolution,
				logger.StrSpace,
				nzb.Info.Quality,
				"]",
				strTvdbid,
				tvdbStr,
				")",
			)
		}

		return logger.JoinStrings(nzb.NZB.Title, strTvdbid, tvdbStr, ")")
	}

	return ""
}

// GetExternalIDFromDB returns the TVDB ID from the database entity.
// Expects *database.Dbserie, returns ThetvdbID as string if non-zero.
func (h *handler) GetExternalIDFromDB(dbEntity any) string {
	if dbserie, ok := dbEntity.(*database.Dbserie); ok && dbserie.ThetvdbID != 0 {
		return strconv.Itoa(dbserie.ThetvdbID)
	}

	return ""
}

// FillSearchVar fills search variables from the database for the given episode ID.
// Sets NzbepisodeID, loads data from DB, validates required fields.
func (h *handler) FillSearchVar(entry *apiexternal_v2.Nzbwithprio, mediaid uint) error {
	entry.NzbepisodeID = mediaid
	if mediaid == 0 {
		return logger.ErrNotFoundEpisode
	}

	database.GetdatarowArgs(
		"select serie_episodes.dbserie_episode_id, serie_episodes.dbserie_id, serie_episodes.serie_id, serie_episodes.dont_search, serie_episodes.dont_upgrade, serie_episodes.quality_profile, series.listname, dbseries.thetvdb_id, dbseries.seriename, dbserie_episodes.season, dbserie_episodes.episode, dbserie_episodes.identifier, dbserie_episodes.absolute_episode from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id inner join dbserie_episodes ON dbserie_episodes.id=serie_episodes.dbserie_episode_id where serie_episodes.id = ?",
		&entry.NzbepisodeID,
		&entry.Info.DbserieEpisodeID,
		&entry.Dbid,
		&entry.Info.SerieID,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Quality,
		&entry.Listname,
		&entry.NZB.TVDBID,
		&entry.WantedTitle,
		&entry.NZB.Season,
		&entry.NZB.Episode,
		&entry.Info.Identifier,
		&entry.Info.AbsoluteEpisode,
	)

	if entry.Info.DbserieEpisodeID == 0 || entry.Dbid == 0 || entry.Info.SerieID == 0 {
		return logger.ErrNotFound
	}

	if entry.DontSearch {
		return logger.ErrDisabled
	}

	entry.Info.DbserieID = entry.Dbid

	return nil
}

// GetNzbID returns the NzbepisodeID field.
func (h *handler) GetNzbID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.NzbepisodeID
}

// GetNzbIDP returns the NzbaudiobookID field.
func (h *handler) GetNzbIDP(entry *apiexternal_v2.Nzbwithprio) *uint {
	return &entry.NzbepisodeID
}

// CheckMediaMatch checks if the entry's SerieEpisodeID matches the source's NzbepisodeID.
func (h *handler) CheckMediaMatch(source, entry *apiexternal_v2.Nzbwithprio) bool {
	return entry.Info.SerieEpisodeID == source.NzbepisodeID
}

// GetUnwantedReason returns "unwanted Episode".
func (h *handler) GetUnwantedReason() string {
	return "unwanted Episode"
}

// GetFoundID returns entry.Info.SerieEpisodeID for logging.
func (h *handler) GetFoundID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.SerieEpisodeID
}

// ValidateRSSIDs validates all required IDs for RSS processing.
// Returns error reason string if invalid, empty string if valid.
func (h *handler) ValidateRSSIDs(entry *apiexternal_v2.Nzbwithprio) string {
	if entry.Info.SerieID == 0 {
		return "unwanted Serie"
	}

	if entry.Info.DbserieID == 0 {
		return "unwanted DBSerie"
	}

	if entry.Info.DbserieEpisodeID == 0 {
		return "unwanted DBEpisode"
	}

	if entry.Info.SerieEpisodeID == 0 {
		return "unwanted Episode"
	}

	return ""
}

// SetRSSIDs sets entry.Dbid = DbserieID, entry.NzbepisodeID = SerieEpisodeID.
func (h *handler) SetRSSIDs(entry *apiexternal_v2.Nzbwithprio) {
	entry.NzbepisodeID = entry.Info.SerieEpisodeID
	entry.Dbid = entry.Info.DbserieID
}

// GetRSSMediaID returns entry.Info.SerieEpisodeID for getrssdata.
func (h *handler) GetRSSMediaID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.SerieEpisodeID
}

// CheckCorrectID validates that the entry's TVDB ID matches the source's.
// Returns true if IDs don't match (should skip), false if they match or can't compare.
// Also returns found and wanted ID strings for logging.
func (h *handler) CheckCorrectID(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
) (bool, string, string) {
	if sourceentry.NZB.TVDBID == 0 || entry.NZB.TVDBID == 0 {
		return false, "", ""
	}

	if sourceentry.NZB.TVDBID == entry.NZB.TVDBID {
		return false, "", ""
	}

	entry.Reason = "not matched tvdb"

	return true, strconv.Itoa(entry.NZB.TVDBID), strconv.Itoa(sourceentry.NZB.TVDBID)
}

// GetRuntimeBonus returns 0 - series don't have extended edition bonus.
func (h *handler) GetRuntimeBonus(info *database.ParseInfo) int {
	return 0
}

// SkipMultipleFiles returns false - series can have multiple episode files.
func (h *handler) SkipMultipleFiles() bool {
	return false
}

// FillNotifyData fills notification data for series from dbseries/dbserie_episodes tables.
// Returns title (series name), year (firstaired), tvdb, series, season, episode, identifier.
func (h *handler) FillNotifyData(
	id *uint,
) (title, year, externalID, series, season, episode, identifier string, ok bool) {
	var dbserieEpisode database.DbserieEpisode
	if dbserieEpisode.GetDbserieEpisodesByIDP(id) != nil {
		return "", "", "", "", "", "", "", false
	}

	var dbserie database.Dbserie
	if dbserie.GetDbserieByIDP(&dbserieEpisode.DbserieID) != nil {
		return "", "", "", "", "", "", "", false
	}

	return dbserie.Seriename,
		dbserie.Firstaired,
		strconv.Itoa(dbserie.ThetvdbID),
		dbserie.Seriename,
		dbserieEpisode.Season,
		dbserieEpisode.Episode,
		dbserieEpisode.Identifier,
		true
}

// FillNamingData fills NamingData for series from the database.
// Handles Dbserie, DbserieEpisode lookups, TitleSource, EpisodeTitleSource, Episodes, and TVDB.
// Returns clearFolder=false for series (folder name handling is more complex).
func (h *handler) FillNamingData(
	dbid *uint,
	videofile string,
	m *database.ParseInfo,
	data *mediatype.NamingData,
) (bool, bool) {
	database.Scanrowsdyn(false, database.QuerySerieEpisodesGetDBSerieIDByID, &data.Dbserie.ID, dbid)
	database.Scanrowsdyn(
		false,
		database.QuerySerieEpisodesGetDBSerieEpisodeIDByID,
		&data.DbserieEpisode.ID,
		dbid,
	)

	if data.DbserieEpisode.ID == 0 || data.Dbserie.ID == 0 ||
		data.Dbserie.GetDbserieByIDP(&data.Dbserie.ID) != nil ||
		data.DbserieEpisode.GetDbserieEpisodesByIDP(&data.DbserieEpisode.ID) != nil {
		return false, false
	}

	// Populate Serie entry for aliases and per-list data
	var serieID uint
	database.Scanrowsdyn(false, "select serie_id from serie_episodes where id = ?", &serieID, dbid)

	if serieID != 0 {
		data.Serie.GetSerieByIDP(&serieID)
	}

	var episodetitle, serietitle string
	if len(m.Episodes) > 0 {
		episodetitle = database.Getdatarow[string](
			false,
			"select title from dbserie_episodes where id = ?",
			&m.Episodes[0].Num2,
		)
	}

	serietitle = database.Getdatarow[string](
		false,
		"select seriename from dbseries where id = ?",
		&m.DbserieID,
	)
	if (serietitle == "" || episodetitle == "") && m.Identifier != "" {
		serietitleparse, episodetitleparse := database.RegexGetMatchesStr1Str2(
			false,
			logger.JoinStrings(`^(.*)(?i)`, m.Identifier, `(?:\.| |-)(.*)$`),
			filepath.Base(videofile),
		)
		if serietitle != "" && episodetitleparse != "" {
			logger.StringReplaceWithP(&episodetitleparse, '.', ' ')

			episodetitleparse = trimStringInclAfterString(episodetitleparse, "XXX")
			episodetitleparse = trimStringInclAfterString(episodetitleparse, m.Quality)
			episodetitleparse = trimStringInclAfterString(episodetitleparse, m.Resolution)
			episodetitleparse = logger.Trim(episodetitleparse, '.', ' ')

			serietitleparse = logger.Trim(serietitleparse, '.')

			logger.StringReplaceWithP(&serietitleparse, '.', ' ')
		}

		if episodetitle == "" {
			episodetitle = episodetitleparse
		}

		if serietitle == "" {
			serietitle = serietitleparse
		}
	}

	if data.Dbserie.Seriename == "" {
		database.Scanrowsdyn(
			false,
			"select title from dbserie_alternates where dbserie_id = ?",
			&data.Dbserie.Seriename,
			&data.Dbserie.ID,
		)

		if data.Dbserie.Seriename == "" {
			data.Dbserie.Seriename = serietitle
		}
	}

	logger.StringRemoveAllRunesP(&data.Dbserie.Seriename, '/')

	if data.DbserieEpisode.Title == "" {
		data.DbserieEpisode.Title = episodetitle
	}

	logger.StringRemoveAllRunesP(&data.DbserieEpisode.Title, '/')

	logger.Path(&data.Dbserie.Seriename, false)
	logger.Path(&data.DbserieEpisode.Title, false)

	// Fill episodes array
	data.Episodes = make([]int, len(m.Episodes))
	for idx := range m.Episodes {
		database.Scanrowsdyn(
			false,
			"select episode from dbserie_episodes where id = ? and episode != ''",
			&data.Episodes[idx],
			&m.Episodes[idx].Num2,
		)
	}

	data.TitleSource = serietitle
	logger.Path(&data.TitleSource, false)
	logger.StringRemoveAllRunesP(&data.TitleSource, '/')

	data.EpisodeTitleSource = episodetitle
	logger.Path(&data.EpisodeTitleSource, false)
	logger.StringRemoveAllRunesP(&data.EpisodeTitleSource, '/')

	// Update source TVDB
	if m.Tvdb == "0" || m.Tvdb == "" || m.Tvdb == "tvdb" || strings.EqualFold(m.Tvdb, "tvdb") {
		m.Tvdb = strconv.Itoa(data.Dbserie.ThetvdbID)
	}

	if m.Tvdb != "" && len(m.Tvdb) >= 1 && !logger.HasPrefixI(m.Tvdb, logger.StrTvdb) {
		m.Tvdb = logger.StrTvdb + m.Tvdb
	}

	return false, true // clearFolder=false for series (more complex handling needed)
}

// GetRefreshIncData returns data for incremental refresh (20 continuing series, oldest updated first).
func (h *handler) GetRefreshIncData() any {
	return database.GetrowsN[database.DbstaticTwoStringOneRInt](
		false,
		20,
		"select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' order by updated_at asc limit 20",
	)
}

// GetRefreshFullData returns data for full refresh (all series).
func (h *handler) GetRefreshFullData() any {
	return database.GetrowsN[database.DbstaticTwoStringOneRInt](
		false,
		database.Getdatarow[uint](false, "select count() from dbseries"),
		"select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries",
	)
}

// GetSchedulerJobNames returns the job name pairs for series scheduler configuration.
// Each pair is [schedulerJobName, singleJobName].
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
		{logger.StrDataFull, logger.StrDataFull},
		{logger.StrStructure, logger.StrStructure},
		{logger.StrFeeds, logger.StrFeeds},
		{logger.StrCheckMissing, logger.StrCheckMissing},
		{logger.StrCheckMissingFlag, logger.StrCheckMissingFlag},
		{logger.StrUpgradeFlag, logger.StrUpgradeFlag},
		{logger.StrRssSeasons, logger.StrRssSeasons},
		{logger.StrRssSeasonsAll, logger.StrRssSeasonsAll},
		{"refreshseriesfull", "refresh"},
		{"refreshseriesinc", "refreshinc"},
	}
}

// CleanupAfterRemove handles cleanup after a series episode file is removed.
// For series: removes files with other allowed extensions (uses removeOtherFilesFn callback).
func (h *handler) CleanupAfterRemove(
	_, _, _ string,
	_ func(string),
	removeOtherFilesFn func(),
) error {
	removeOtherFilesFn()
	return nil
}

// MoveOtherFilesAfterOrganize handles moving additional files after main series episode is organized.
// For series: moves files with other allowed extensions, then calls notify and cleanup.
func (h *handler) MoveOtherFilesAfterOrganize(params *mediatype.MoveOtherFilesParams) error {
	fileext := filepath.Ext(params.MediaFile)

	for idx := range params.AllowedOtherExtensions {
		if fileext == params.AllowedOtherExtensions[idx] {
			continue
		}

		also := strings.Replace(params.MediaFile, fileext, params.AllowedOtherExtensions[idx], 1)

		err := params.MoveFileFn(also, params.TargetPath, params.Filename)
		if err != nil && !errors.Is(err, logger.ErrNotFound) {
			logger.Logtype("error", 1).
				Str(logger.StrFile, also).
				Err(err).
				Msg("file move")
		}
	}

	params.NotifyFn()
	params.CleanupFolderFn()

	return nil
}

// CheckExtensions validates if a file extension is allowed for series.
// Series use video extensions (e.g., .mkv, .mp4, .avi).
func (h *handler) CheckExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	return mediatype.CheckVideoExtensions(pathcfg, ext)
}

// SupportsIDSearch returns true - series support TVDB ID-based search.
func (h *handler) SupportsIDSearch() bool { return true }

// SupportsSeasonSearch returns true - series have season/episode structure.
func (h *handler) SupportsSeasonSearch() bool { return true }

// RequiresYearCheck returns false - series don't require strict year matching.
func (h *handler) RequiresYearCheck() bool { return false }

// HasSearchID returns true if the series entry has a valid TVDB ID.
func (h *handler) HasSearchID(entry *apiexternal_v2.Nzbwithprio) bool {
	return entry.NZB.TVDBID != 0
}

// SupportsAbsoluteEpisode returns true - series support absolute episode numbering (e.g., anime).
func (h *handler) SupportsAbsoluteEpisode() bool { return true }

// HandleRSSListID does nothing for series - list ID is determined by episode lookup.
func (h *handler) HandleRSSListID(_ *apiexternal_v2.Nzbwithprio, _ int) {}

// CheckEpisodeMatch validates if the entry matches the expected episode.
// For series: validates season/episode identifiers against the source entry.
// Returns true if entry should be skipped (doesn't match).
func (h *handler) CheckEpisodeMatch(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
	searchActionType string,
	logdenied func(string, *apiexternal_v2.Nzbwithprio),
) bool {
	// Validate absolute episode match
	if entry.Info.AbsoluteEpisode != 0 && sourceentry.Info.AbsoluteEpisode != 0 {
		if entry.Info.AbsoluteEpisode == sourceentry.Info.AbsoluteEpisode {
			return false // Match found
		}
	}

	// Validate season/episode match
	if sourceentry.NZB.Season != "" && sourceentry.NZB.Episode != "" {
		if entry.NZB.Season == sourceentry.NZB.Season &&
			entry.NZB.Episode == sourceentry.NZB.Episode {
			return false // Match found
		}
	}

	// Validate identifier match for date-based episodes
	if sourceentry.Info.Identifier != "" && entry.Info.Identifier != "" {
		if entry.Info.Identifier == sourceentry.Info.Identifier {
			return false // Match found
		}
	}

	// For RSS search, be more lenient - allow if any episode info matches
	if searchActionType == "rss" {
		return false
	}

	// No match found
	entry.Reason = "episode mismatch"
	if logdenied != nil {
		logdenied("episode mismatch", entry)
	}

	return true
}

// SupportsVideoFile returns true - series use video files.
func (h *handler) SupportsVideoFile() bool { return true }

// GetRuntimeMultiplier returns the number of episodes in the file.
func (h *handler) GetRuntimeMultiplier(m *database.ParseInfo) int {
	if len(m.Episodes) > 0 {
		return len(m.Episodes)
	}

	return 1
}

// ShouldCheckOldFilePriority returns false - series handle priority differently per episode.
func (h *handler) ShouldCheckOldFilePriority() bool { return false }

// HasConfiguredExtensions returns true if video extensions are configured.
func (h *handler) HasConfiguredExtensions(pathcfg *config.PathsConfig) bool {
	return pathcfg.AllowedVideoExtensionsLen > 0 || pathcfg.AllowedVideoExtensionsNoRenameLen > 0
}

// IsExternalIDImdb returns false - series use TVDB for external IDs.
func (h *handler) IsExternalIDImdb() bool { return false }

// UsesGroupedFileProcessing returns false - series are processed individually per episode.
func (h *handler) UsesGroupedFileProcessing() bool { return false }

// GetCacheUnmatchedKey returns the cache key for unmatched series.
func (h *handler) GetCacheUnmatchedKey() string { return logger.CacheUnmatchedSeries }

// GetCacheFilesKey returns the cache key for series files.
func (h *handler) GetCacheFilesKey() string { return logger.CacheFilesSeries }

// UsesListNameAsQualityProfile returns true - series uses list name for episode quality tracking.
func (h *handler) UsesListNameAsQualityProfile() bool { return true }

// StringsMap contains all series-specific string mappings for SQL queries, cache names, etc.
// Exported so mtstrings package can access it without going through Handler (avoids import cycle).
var StringsMap = map[string]string{
	"CacheDBMedia":             "CacheDBSeries",
	"DBCountDBMedia":           "select count() from dbseries",
	"DBCacheDBMedia":           "select seriename, slug, '', 0, id from dbseries",
	"CacheMedia":               "CacheSeries",
	"DBCountMedia":             "select count() from series",
	"DBCacheMedia":             "select lower(listname), dbserie_id, id from series",
	"CacheHistoryTitle":        "CacheHistoryTitleSeries",
	"CacheHistoryUrl":          "CacheHistoryUrlSeries",
	"DBHistoriesUrl":           "select distinct url from serie_episode_histories",
	"DBHistoriesTitle":         "select distinct title from serie_episode_histories",
	"DBCountHistoriesUrl":      "select count() from (select distinct url from serie_episode_histories)",
	"DBCountHistoriesTitle":    "select count() from (select distinct title from serie_episode_histories)",
	"CacheMediaTitles":         "CacheDBSeriesAlt",
	"DBCountDBTitles":          "select count() from dbserie_alternates where title != ''",
	"DBCacheDBTitles":          "select title, slug, dbserie_id from dbserie_alternates where title != ''",
	"CacheFiles":               "CacheFilesSeries",
	"DBCountFiles":             "select count() from serie_episode_files",
	"DBCacheFiles":             "select location from serie_episode_files",
	"CacheUnmatched":           "CacheUnmatchedSeries",
	"DBCountUnmatched":         "select count() from serie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
	"DBRemoveUnmatched":        "delete from serie_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
	"DBCacheUnmatched":         "select filepath from serie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
	"DBCountFilesLocation":     "select count() from serie_episode_files where location = ?",
	"DBCountUnmatchedPath":     "select count() from serie_file_unmatcheds where filepath = ?",
	"DBCountDBTitlesDBID":      "select count() from (select distinct title, slug from dbserie_alternates where dbserie_id = ? and title != '')",
	"DBDistinctDBTitlesDBID":   "select distinct title, slug, dbserie_id from dbserie_alternates where dbserie_id = ? and title != ''",
	"DBMediaTitlesID":          "select 0, seriename, slug from dbseries where id = ?",
	"DBFilesQuality":           "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?",
	"DBCountFilesByList":       "select count() from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
	"DBLocationFilesByList":    "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
	"DBIDsFilesByLocation":     "select location, id, serie_episode_id from serie_episode_files",
	"DBCountFilesByMediaID":    "select count() from serie_episode_files where serie_episode_id = ?",
	"DBCountFilesByLocation":   "select count() from serie_episode_files",
	"TableFiles":               "serie_episode_files",
	"TableMedia":               "serie_episodes",
	"DBCountMediaByList":       "select count() from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
	"DBIDMissingMediaByList":   "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
	"DBUpdateMissing":          "update serie_episodes set missing = ? where id = ?",
	"DBListnameByMediaID":      "select listname from series where id = ?",
	"DBRootPathFromMediaID":    "select rootpath from series where id = ?",
	"DBDeleteFileByIDLocation": "delete from serie_episode_files where serie_id = ? and location = ?",
	"DBCountHistoriesByTitle":  "select count() from serie_episode_histories where title = ?",
	"DBCountHistoriesByUrl":    "select count() from serie_episode_histories where url = ?",
	"DBLocationIDFilesByID":    "select location, id from serie_episode_files where serie_episode_id = ?",
	"DBFilePrioFilesByID":      "select location, serie_episode_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from serie_episode_files where serie_episode_id = ?",
	"UpdateMediaLastscan":      "update serie_episodes set lastscan = datetime('now','localtime') where id = ?",
	"DBQualityMediaByID":       "select quality_profile from serie_episodes where id = ?",
	"SearchGenSelect":          "select serie_episodes.quality_profile, serie_episodes.id ",
	"SearchGenTable":           " from serie_episodes inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id where ",
	"SearchGenMissing":         "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
	"SearchGenMissingEnd":      ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)",
	"SearchGenReached":         "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
	"SearchGenLastScan":        " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)",
	"SearchGenDate":            " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)",
	"SearchGenOrder":           " order by serie_episodes.Lastscan asc",
	"DBIDUnmatchedPathList":    "select id from serie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
	"InsertUnmatched":          "Insert into serie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
	"UpdateUnmatched":          "update serie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
	"GetRSSData":               "select serie_episodes.dont_search, serie_episodes.dont_upgrade, series.listname, serie_episodes.quality_profile, dbseries.seriename from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id where serie_episodes.id = ?",
	"GetOrganizeData":          "select dbserie_id, rootpath, listname from series where id = ?",
	"ClearHistoryByList":       "delete from serie_episode_histories where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
	"QueryMediaByList":         "select id, quality_reached, quality_profile from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
	"QueryMediaCountByList":    "select count() from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
	"UpdateQualityReached":     "update serie_episodes set quality_reached = ? where id = ?",
	"SelectRootpath":           "select rootpath from series where id = ?",
	"InsertFile":               "insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
	"UpdateMissingByID":        "update serie_episodes set missing = 0 where id = ?",
	"UpdateQualityReachedByID": "update serie_episodes set quality_reached = ? where id = ?",
	"UpdateQualityProfileByID": "update serie_episodes set quality_profile = ? where id = ?",
	"DeleteUnmatchedByPath":    "delete from serie_file_unmatcheds where filepath = ?",
	"CountFileByLocationAndID": "select count() from serie_episode_files where location = ? and serie_episode_id = ?",
	"SelectRuntime":            "select runtime from dbseries where id = ?",
	"SelectEpisodeRuntime":     "select runtime, season from dbserie_episodes where id = ?",
	"SelectIdentifiedBy":       "select identifiedby from dbseries where id = ?",
	"SelectIgnoreRuntime":      "select ignore_runtime from serie_episodes where id = ?",
	"InsertFileOrganize":       "insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
	"UpdateMissingReached":     "update serie_episodes SET missing = 0, quality_reached = ? where id = ?",
}

// GetStringsMap returns a series-specific string for the given key.
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

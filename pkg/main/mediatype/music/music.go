// Package music implements the mediatype.Handler interface for music/album media.
// It registers itself automatically via init() and provides all music-specific
// logic for searching, parsing, organizing, and processing music files.
package music

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

// handler implements mediatype.Handler for music/albums.
type handler struct {
	refreshFunc  mediatype.RefreshFunc
	dataFullFunc mediatype.DataFullFunc
}

var (
	// Handler is the singleton instance.
	Handler = &handler{}

	// StringsMap contains all music-specific string mappings for SQL queries, cache names, etc.
	StringsMap = map[string]string{
		"CacheDBMedia":             "CacheDBAlbum",
		"DBCountDBMedia":           "select count() from dbalbums",
		"DBCacheDBMedia":           "select title, slug, musicbrainz_release_id, year, id from dbalbums",
		"CacheMedia":               "CacheAlbum",
		"DBCountMedia":             "select count() from albums",
		"DBCacheMedia":             "select lower(listname), dbalbum_id, id from albums",
		"CacheRootpath":            "CacheRootpathAlbum",
		"DBCountRootpath":          "select count() from (select distinct rootpath from albums where rootpath != '' and exists (select 1 from album_files where album_id = albums.id))",
		"DBCacheRootpath":          "select distinct rootpath from albums where rootpath != '' and exists (select 1 from album_files where album_id = albums.id)",
		"CacheHistoryTitle":        "CacheHistoryTitleAlbum",
		"CacheHistoryUrl":          "CacheHistoryUrlAlbum",
		"DBHistoriesUrl":           "select distinct url from album_histories",
		"DBHistoriesTitle":         "select distinct title from album_histories",
		"DBCountHistoriesUrl":      "select count() from (select distinct url from album_histories)",
		"DBCountHistoriesTitle":    "select count() from (select distinct title from album_histories)",
		"CacheMediaTitles":         "CacheTitlesAlbum",
		"DBCountDBTitles":          "select count() from dbalbum_titles where title != ''",
		"DBCacheDBTitles":          "select title, slug, dbalbum_id from dbalbum_titles where title != ''",
		"CacheFiles":               "CacheFilesAlbum",
		"DBCountFiles":             "select count() from album_files",
		"DBCacheFiles":             "select location from album_files",
		"CacheUnmatched":           "CacheUnmatchedAlbum",
		"DBCountUnmatched":         "select count() from album_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBRemoveUnmatched":        "delete from album_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		"DBCacheUnmatched":         "select filepath from album_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		"DBCountFilesLocation":     "select count() from album_files where location = ?",
		"DBCountUnmatchedPath":     "select count() from album_file_unmatcheds where filepath = ?",
		"DBCountDBTitlesDBID":      "select count() from (select distinct title, slug from dbalbum_titles where dbalbum_id = ? and title != '')",
		"DBDistinctDBTitlesDBID":   "select distinct title, slug, dbalbum_id from dbalbum_titles where dbalbum_id = ? and title != ''",
		"DBMediaTitlesID":          "select year, title, slug from dbalbums where id = ?",
		"DBFilesQuality":           "select 0, 0, 0, 0, 0, 0, 0 from album_files where id = ?",
		"DBCountFilesByList":       "select count() from album_files where album_id in (select id from albums where listname = ? COLLATE NOCASE)",
		"DBLocationFilesByList":    "select location from album_files where album_id in (select id from albums where listname = ? COLLATE NOCASE)",
		"DBIDsFilesByLocation":     "select location, id, album_id from album_files",
		"DBCountFilesByMediaID":    "select count() from album_files where album_id = ?",
		"DBCountFilesByLocation":   "select count() from album_files",
		"TableFiles":               "album_files",
		"TableMedia":               "albums",
		"DBCountMediaByList":       "select count() from albums where listname = ? COLLATE NOCASE",
		"DBIDMissingMediaByList":   "select id,missing from albums where listname = ? COLLATE NOCASE",
		"DBUpdateMissing":          "update albums set missing = ? where id = ?",
		"DBListnameByMediaID":      "select listname from albums where id = ?",
		"DBRootPathFromMediaID":    "select rootpath from albums where id = ?",
		"DBDeleteFileByIDLocation": "delete from album_files where album_id = ? and location = ?",
		"DBCountHistoriesByTitle":  "select count() from album_histories where title = ?",
		"DBCountHistoriesByUrl":    "select count() from album_histories where url = ?",
		"DBLocationIDFilesByID":    "select location, id from album_files where album_id = ?",
		"DBFilePrioFilesByID":      "select location, album_id, id, 0, 0, 0, 0, 0, 0, 0 from album_files where album_id = ?",
		"DBAudioFilePrioFilesByID": "select location, album_id, id, format, bitrate, sample_rate, bit_depth from album_files where album_id = ?",
		"UpdateMediaLastscan":      "update albums set lastscan = datetime('now','localtime') where id = ?",
		"DBQualityMediaByID":       "select quality_profile from albums where id = ?",
		"SearchGenSelect":          "select albums.quality_profile, albums.id ",
		"SearchGenTable":           " from albums inner join dbalbums on dbalbums.id=albums.dbalbum_id where ",
		"SearchGenMissing":         "dbalbums.year != 0 and albums.missing = 1 and albums.listname in (?",
		"SearchGenMissingEnd":      ")",
		"SearchGenReached":         "dbalbums.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
		"SearchGenLastScan":        " and (albums.lastscan is null or albums.Lastscan < ?)",
		"SearchGenDate":            " and (dbalbums.release_date < ? or dbalbums.release_date is null)",
		"SearchGenOrder":           " order by albums.Lastscan asc",
		"DBIDUnmatchedPathList":    "select id from album_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		"InsertUnmatched":          "Insert into album_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		"UpdateUnmatched":          "update album_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		"GetRSSData":               "select albums.dont_search, albums.dont_upgrade, albums.listname, albums.quality_profile, dbalbums.title from albums inner join dbalbums ON dbalbums.id=albums.dbalbum_id where albums.id = ?",
		"GetOrganizeData":          "select dbalbum_id, rootpath, listname from albums where id = ?",
		"ClearHistoryByList":       "delete from album_histories where album_id in (Select id from albums where listname = ? COLLATE NOCASE)",
		"QueryMediaByList":         "select id, quality_reached, quality_profile from albums where listname = ? COLLATE NOCASE",
		"QueryMediaCountByList":    "select count() from albums where listname = ? COLLATE NOCASE",
		"UpdateQualityReached":     "update albums set quality_reached = ? where id = ?",
		"SelectRootpath":           "select rootpath from albums where id = ?",
		"InsertFile":               "insert into album_files (location, filename, extension, quality_profile, album_id, dbalbum_id, dbtrack_id) values (?, ?, ?, ?, ?, ?, ?)",
		"UpdateMissingByID":        "update albums set missing = 0 where id = ?",
		"UpdateQualityReachedByID": "update albums set quality_reached = ? where id = ?",
		"DeleteUnmatchedByPath":    "delete from album_file_unmatcheds where filepath = ?",
		"SelectRuntime":            "select total_runtime_ms from dbalbums where id = ?",
		"InsertFileOrganize":       "insert into album_files (location, filename, extension, quality_profile, album_id, dbalbum_id, dbtrack_id) values (?, ?, ?, ?, ?, ?, ?)",
		"UpdateMissingReached":     "update albums SET missing = 0, quality_reached = ? where id = ?",
		// Artist-based search queries
		"SearchArtistsMissing":    "SELECT DISTINCT da.name, da.id FROM dbartists da JOIN artists art ON da.id = art.dbartist_id JOIN albums a ON a.artist_id = art.id WHERE art.track_mode != 'none' AND art.dont_search = 0 AND a.missing = 1 AND a.listname IN (?",
		"SearchArtistsMissingEnd": ") ORDER BY RANDOM() LIMIT 20",
		"SearchArtistsUpgrade":    "SELECT DISTINCT da.name, da.id FROM dbartists da JOIN artists art ON da.id = art.dbartist_id JOIN albums a ON a.artist_id = art.id WHERE art.track_mode != 'none' AND art.dont_search = 0 AND a.missing = 0 AND a.quality_reached = 0 AND a.listname IN (?",
		"SearchArtistsUpgradeEnd": ") ORDER BY RANDOM() LIMIT 20",
	}
)

func init() {
	mediatype.Register(Handler)
	mtstrings.Register(config.MediaTypeMusic, StringsMap)
}

// RegisterRefresh sets the refresh function for music.
func RegisterRefresh(fn mediatype.RefreshFunc) {
	Handler.refreshFunc = fn
}

// RegisterDataFull sets the data full function for music.
func RegisterDataFull(fn mediatype.DataFullFunc) {
	Handler.dataFullFunc = fn
}

// GetType returns the media type constant for music.
func (*handler) GetType() uint {
	return config.MediaTypeMusic
}

// GetCategoryName returns the category name for job history.
func (*handler) GetCategoryName() string {
	return "music"
}

// GetTableName returns the database table name for albums.
func (*handler) GetTableName() string {
	return "albums"
}

// GetDBIDs retrieves database IDs for the parsed album info.
// It attempts MusicBrainz ID or UPC lookup first, then falls back to title-based search.
func (*handler) GetDBIDs(info *database.ParseInfo) error {
	// Handle MusicBrainz ID or UPC lookup
	if info.MusicBrainzID != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbalbums where musicbrainz_release_id = ? or musicbrainz_release_group_id = ?",
			&info.DbalbumID,
			&info.MusicBrainzID,
			&info.MusicBrainzID,
		)
	}

	if info.DbalbumID == 0 && info.UPC != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbalbums where upc = ?",
			&info.DbalbumID,
			&info.UPC,
		)
	}

	if info.DbalbumID == 0 {
		return logger.ErrNotFoundDbalbum
	}

	return nil
}

// GetDBIDsFull retrieves database IDs for an album with full search capabilities.
// It attempts MusicBrainz/UPC lookup first, then falls back to title-based search,
// and finally finds the album in configured lists.
func (h *handler) GetDBIDsFull(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowSearchTitle bool,
	_ bool,
) error {
	// Handle MusicBrainz ID lookup
	if m.MusicBrainzID != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbalbums where musicbrainz_release_id = ? or musicbrainz_release_group_id = ?",
			&m.DbalbumID,
			&m.MusicBrainzID,
			&m.MusicBrainzID,
		)
	}

	// Handle UPC lookup
	if m.DbalbumID == 0 && m.UPC != "" {
		database.Scanrowsdyn(
			false,
			"select id from dbalbums where upc = ?",
			&m.DbalbumID,
			&m.UPC,
		)
	}

	// Title-based search if ID lookup failed
	if m.DbalbumID == 0 && m.Title != "" && allowSearchTitle && cfgp.Name != "" {
		// Strip title prefixes/postfixes
		for _, lst := range cfgp.ListsMap {
			if lst.TemplateQuality != "" {
				m.StripTitlePrefixPostfixGetQual(lst.CfgQuality)
			}
		}

		m.Title = logger.TrimSpace(m.Title)

		// Try artist-first lookup prioritizing albums in the wanted list
		// This ensures we find the correct dbalbum when there are multiple releases
		m.FindDbalbumByArtistFirstFromWantedList(cfgp.ListsNames)

		// Fallback to regular artist-first lookup (finds any dbalbum by that artist)
		if m.DbalbumID == 0 {
			m.FindDbalbumByArtistFirst()
		}

		// Fallback to traditional title-based search
		if m.DbalbumID == 0 {
			m.FindDbalbumByTitle()
		}
	}

	if m.DbalbumID == 0 {
		return logger.ErrNotFoundDbalbum
	}

	// Find album in lists (if not already found by wanted-list search)
	if m.AlbumID != 0 {
		return nil
	}

	return h.findInLists(m, cfgp)
}

// findInLists attempts to locate an album in configured media lists by its database ID.
func (*handler) findInLists(m *database.ParseInfo, cfgp *config.MediaTypeConfig) error {
	if m.ListID != -1 {
		database.Scanrowsdyn(
			false,
			"select id from albums where dbalbum_id = ? and listname = ? COLLATE NOCASE",
			&m.AlbumID,
			&m.DbalbumID,
			&cfgp.Lists[m.ListID].Name,
		)
	}

	if m.AlbumID == 0 && cfgp.Name != "" && m.ListID == -1 {
		for idx := range cfgp.Lists {
			if config.GetSettingsGeneral().UseMediaCache {
				m.AlbumID = database.CacheOneStringTwoIntIndexFuncRet(
					logger.CacheAlbum,
					m.DbalbumID,
					cfgp.Lists[idx].Name,
				)
			} else {
				database.Scanrowsdyn(
					false,
					"select id from albums where listname = ? COLLATE NOCASE and dbalbum_id = ?",
					&m.AlbumID,
					&cfgp.Lists[idx].Name,
					&m.DbalbumID,
				)
			}

			if m.AlbumID != 0 {
				m.ListID = idx
				break
			}
		}
	}

	if m.AlbumID == 0 {
		return logger.ErrNotFoundAlbum
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.AlbumID)
	}

	return nil
}

// ValidateIDs checks if all required IDs are set for albums.
func (*handler) ValidateIDs(info *database.ParseInfo) bool {
	return info.AlbumID != 0 && info.DbalbumID != 0
}

// SetTempID sets the temporary ID from AlbumID.
func (*handler) SetTempID(info *database.ParseInfo) {
	info.TempID = info.AlbumID
}

// SetDBID sets the DbalbumID field.
func (*handler) SetDBID(info *database.ParseInfo, dbid uint) {
	info.DbalbumID = dbid
}

// GetDBID returns the DbalbumID field.
func (*handler) GetDBID(info *database.ParseInfo) uint {
	return info.DbalbumID
}

// GetMediaID returns the AlbumID.
func (*handler) GetMediaID(info *database.ParseInfo) uint {
	return info.AlbumID
}

// SetMediaID sets the AlbumID.
func (*handler) SetMediaID(info *database.ParseInfo, id uint) {
	info.AlbumID = id
}

// GetListID retrieves the list ID for the album.
func (*handler) GetListID(cfgp *config.MediaTypeConfig, info *database.ParseInfo) int {
	if info.AlbumID != 0 {
		return database.GetMediaListIDGetListname(cfgp, &info.AlbumID)
	}

	return -1
}

// ClearUntrustedID clears the MusicBrainz ID if indexer is not trusted.
func (h *handler) ClearUntrustedID(_ *apiexternal_v2.Nzbwithprio) {
	// Music doesn't have a specific trust flag like IMDB/TVDB
	// Could be extended if needed
}

// SetNzbID sets the NzbalbumID field.
func (*handler) SetNzbID(entry *apiexternal_v2.Nzbwithprio, mediaid uint) {
	entry.NzbalbumID = mediaid
}

// SetEntryTempID sets the temp ID from NzbalbumID.
func (*handler) SetEntryTempID(entry *apiexternal_v2.Nzbwithprio) {
	entry.Info.TempID = entry.NzbalbumID
}

// PerformIDSearch executes a search - music uses query-based search only.
func (*handler) PerformIDSearch(
	_ *config.IndexersConfig,
	_ *config.QualityConfig,
	_ *apiexternal_v2.Nzbwithprio,
	_ int,
	_ *apiexternal.NzbSlice,
) error {
	// Music doesn't support ID-based search like IMDB/TVDB
	// They use query-based search only
	return nil
}

// ClearUnmatchedCache removes the file from the album unmatched cache.
func (*handler) ClearUnmatchedCache(fpath string) {
	database.SlicesCacheContainsDelete(logger.CacheUnmatchedAlbum, fpath)
}

// ShortenYearPattern returns true - music shortens all patterns including year.
func (*handler) ShortenYearPattern() bool {
	return true
}

// GenerateIdentifier does nothing for music - albums don't have episode identifiers.
func (h *handler) GenerateIdentifier(_ *database.ParseInfo, onlyIfEmpty bool) {
	// Albums don't have identifiers like series do
}

// GetSchedulerRssSeasons returns empty strings for music - no RSS seasons jobs.
func (*handler) GetSchedulerRssSeasons(
	_ *config.SchedulerConfig,
	_ string,
) (string, string) {
	return "", ""
}

// GetSchedulerRssArtistsAuthors returns the interval and cron strings for artist-based RSS searches.
func (*handler) GetSchedulerRssArtistsAuthors(
	scheduler *config.SchedulerConfig,
	jobType string,
) (string, string) {
	switch jobType {
	case logger.StrRssArtists:
		return scheduler.IntervalIndexerRssArtists, scheduler.CronIndexerRssArtists
	case logger.StrRssArtistsUpgrade:
		return scheduler.IntervalIndexerRssArtistsUpgrade, scheduler.CronIndexerRssArtistsUpgrade
	}

	return "", ""
}

// Refresh refreshes music data.
func (h *handler) Refresh(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if h.refreshFunc != nil {
		return h.refreshFunc(ctx, cfgp, data)
	}

	return nil
}

// DataFull performs full data refresh for music.
func (h *handler) DataFull() {
	if h.dataFullFunc != nil {
		h.dataFullFunc()
	}
}

// SearchConfigByName searches the music config file for matching artist name, album series, or alternate names.
// Returns the matching SerieConfig and true if found, or nil and false if not found.
func (*handler) SearchConfigByName(
	searchName string,
	listCfg *config.MediaListsConfig,
) (*config.ManualConfig, bool) {
	if listCfg.CfgList.ManualConfigFile == "" {
		return nil, false
	}

	configs, err := loadMusicConfig(listCfg.CfgList.ManualConfigFile)
	if err != nil {
		logger.Logtype(logger.StatusDebug, 1).
			Err(err).
			Str("config_file", listCfg.CfgList.ManualConfigFile).
			Msg("Failed to load music config file")

		return nil, false
	}

	// Search for matching name, artist, or album series in this config file
	for idx := range configs {
		// Check primary name
		if strings.EqualFold(strings.TrimSpace(configs[idx].Name), searchName) {
			return &configs[idx], true
		}

		// Check artist name
		if configs[idx].ArtistName != "" &&
			strings.EqualFold(strings.TrimSpace(configs[idx].ArtistName), searchName) {
			return &configs[idx], true
		}

		// Check album series name
		if configs[idx].AlbumSeriesName != "" &&
			strings.EqualFold(strings.TrimSpace(configs[idx].AlbumSeriesName), searchName) {
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

// loadMusicConfig loads music configuration from a TOML file.
func loadMusicConfig(file string) ([]config.ManualConfig, error) {
	content, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer content.Close()

	var s config.MainManualConfig

	err = toml.NewDecoder(content).Decode(&s)

	return s.Config, err
}

// RecordDownloadHistory records an album download in the album_histories table.
func (*handler) RecordDownloadHistory(nzb *apiexternal_v2.Nzbwithprio, targetPath string) error {
	var (
		albumID, dbalbumID uint
		qualityProfile     string
	)

	if nzb.NzbalbumID != 0 {
		albumID = nzb.NzbalbumID
		database.Scanrowsdyn(
			false,
			"select quality_profile from albums where id = ?",
			&qualityProfile,
			&nzb.NzbalbumID,
		)
		database.Scanrowsdyn(
			false,
			"select dbalbum_id from albums where id = ?",
			&dbalbumID,
			&nzb.NzbalbumID,
		)
	}

	database.ExecN(
		"Insert into album_histories (title, url, target, indexer, downloaded_at, album_id, dbalbum_id, quality_profile) VALUES (?, ?, ?, ?, datetime('now','localtime'), ?, ?, ?)",
		&nzb.NZB.Title,
		&nzb.NZB.DownloadURL,
		&targetPath,
		&nzb.NZB.Indexer.Name,
		&albumID,
		&dbalbumID,
		&qualityProfile,
	)

	return nil
}

// GetDownloadTargetFolder returns the target folder name for an album download.
// Returns artist - album title format.
func (*handler) GetDownloadTargetFolder(
	nzb *apiexternal_v2.Nzbwithprio,
	_ string,
) string {
	if nzb.NZB.Title != "" {
		return nzb.NZB.Title
	}

	if nzb.Info.Title != "" {
		return nzb.Info.Title
	}

	return ""
}

// FillSearchVar fills search variables from the database for the given album ID.
// Sets NzbalbumID, loads data from DB, validates required fields.
func (*handler) FillSearchVar(entry *apiexternal_v2.Nzbwithprio, mediaid uint) error {
	entry.NzbalbumID = mediaid
	if mediaid == 0 {
		return logger.ErrNotFoundDbalbum
	}

	var artistName string

	database.GetdatarowArgs(
		"select albums.dbalbum_id, albums.dont_search, albums.dont_upgrade, albums.listname, albums.quality_profile, dbalbums.year, dbalbums.title from albums inner join dbalbums ON dbalbums.id=albums.dbalbum_id where albums.id = ?",
		&entry.NzbalbumID,
		&entry.Dbid,
		&entry.DontSearch,
		&entry.DontUpgrade,
		&entry.Listname,
		&entry.Quality,
		&entry.Info.Year,
		&entry.WantedTitle,
	)

	if entry.Dbid == 0 {
		return logger.ErrNotFoundDbalbum
	}

	if entry.DontSearch {
		return logger.ErrDisabled
	}

	// Get the primary artist name for this album
	database.GetdatarowArgs(
		"select dbartists.name from dbalbum_artists inner join dbartists ON dbartists.id=dbalbum_artists.dbartist_id where dbalbum_artists.dbalbum_id = ? order by dbalbum_artists.position limit 1",
		&entry.Dbid,
		&artistName,
	)

	// Build search query: "Title Artist"
	if artistName != "" && artistName != "Various Artists" && artistName != "VA" &&
		artistName != "Various" {
		entry.SearchFor = entry.WantedTitle + " " + artistName
	} else {
		entry.SearchFor = entry.WantedTitle
	}

	return nil
}

// GetNzbID returns the NzbalbumID field.
func (*handler) GetNzbID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.NzbalbumID
}

// GetNzbIDP returns the NzbaudiobookID field.
func (*handler) GetNzbIDP(entry *apiexternal_v2.Nzbwithprio) *uint {
	return &entry.NzbalbumID
}

// CheckMediaMatch checks if the entry's AlbumID matches the source's NzbalbumID.
func (*handler) CheckMediaMatch(source, entry *apiexternal_v2.Nzbwithprio) bool {
	return source.NzbalbumID == entry.Info.AlbumID
}

// GetUnwantedReason returns "unwanted Album".
func (*handler) GetUnwantedReason() string {
	return "unwanted Album"
}

// GetFoundID returns entry.Info.AlbumID for logging.
func (*handler) GetFoundID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.AlbumID
}

// ValidateRSSIDs validates DbalbumID and AlbumID for RSS processing.
// Returns error reason string if invalid, empty string if valid.
func (*handler) ValidateRSSIDs(entry *apiexternal_v2.Nzbwithprio) string {
	if entry.Info.DbalbumID == 0 {
		return "unwanted DBAlbum"
	}

	if entry.Info.AlbumID == 0 {
		return "unwanted Album"
	}

	return ""
}

// SetRSSIDs sets entry.Dbid = DbalbumID, entry.NzbalbumID = AlbumID.
func (*handler) SetRSSIDs(entry *apiexternal_v2.Nzbwithprio) {
	entry.Dbid = entry.Info.DbalbumID
	entry.NzbalbumID = entry.Info.AlbumID
}

// GetRSSMediaID returns entry.Info.AlbumID for getrssdata.
func (*handler) GetRSSMediaID(entry *apiexternal_v2.Nzbwithprio) uint {
	return entry.Info.AlbumID
}

// CheckCorrectID validates that the entry's MusicBrainz ID matches the source's.
// Returns true if IDs don't match (should skip), false if they match or can't compare.
func (*handler) CheckCorrectID(
	_, entry *apiexternal_v2.Nzbwithprio,
) (bool, string, string) {
	// Music doesn't have a standard ID like IMDB/TVDB in NZB results
	// MusicBrainz ID matching could be added if indexers support it
	return false, "", ""
}

// GetRuntimeBonus returns 0 - albums don't have runtime bonus.
func (h *handler) GetRuntimeBonus(_ *database.ParseInfo) int {
	return 0
}

// SkipMultipleFiles returns false - albums can have multiple track files.
func (*handler) SkipMultipleFiles() bool {
	return false
}

// FillNotifyData fills notification data for albums from dbalbums table.
// Returns title, year. Other fields are empty for albums.
func (h *handler) FillNotifyData(
	id *uint,
) (title, year, externalID, series, season, episode, identifier string, ok bool) {
	var dbalbum database.Dbalbum
	if dbalbum.GetDbalbumByIDP(id) != nil {
		return "", "", "", "", "", "", "", false
	}

	return dbalbum.Title, logger.IntToString(
		dbalbum.Year,
	), dbalbum.MusicbrainzReleaseID, "", "", "", "", true
}

// FillNamingData fills NamingData for albums from the database.
// Returns clearFolder=true for albums (folder name should be cleared when rootpath exists).
func (*handler) FillNamingData(
	dbid *uint,
	videofile string,
	m *database.ParseInfo,
	data *mediatype.NamingData,
) (bool, bool) {
	if data.Dbalbum.GetDbalbumByIDP(dbid) != nil {
		return false, false
	}

	logger.Path(&data.Dbalbum.Title, false)

	// Extract track title and number from filename
	data.TitleSource = filepath.Base(videofile)
	data.TitleSource = logger.Trim(data.TitleSource, '.')
	logger.Path(&data.TitleSource, false)
	logger.StringReplaceWithP(&data.TitleSource, '.', ' ')

	data.Title = data.TitleSource

	// Try to extract track number from ParseInfo (Episode field is reused for track number)
	if m != nil && m.Episode > 0 {
		data.Track = m.Episode
	}

	// Get the primary artist
	var artistID uint
	database.GetdatarowArgs(
		"select dbartist_id from dbalbum_artists where dbalbum_id = ? order by position limit 1",
		*dbid,
		&artistID,
	)

	if artistID > 0 {
		data.Artist.GetDbartistByIDP(&artistID)
	}

	// Set AlbumArtist: for compilations (4+ artists), use "Various Artists".
	// Otherwise use the same as Artist.
	var artistCount int
	database.GetdatarowArgs(
		"select count() from dbalbum_artists where dbalbum_id = ?",
		*dbid,
		&artistCount,
	)

	if artistCount > 3 {
		data.AlbumArtist.Name = "Various Artists"
	} else if data.Artist.Name != "" {
		data.AlbumArtist = data.Artist
	}

	// Get the maximum disc number for this album to determine if it's multi-disc
	// Store it in Source.Season as a temporary holder (Season is not used for music)
	if m != nil {
		maxDisc := database.Getdatarow[uint16](
			false,
			"select COALESCE(max(disc_number), 0) from dbtracks where dbalbum_id = ?",
			*dbid,
		)

		m.Season = int(maxDisc) // Store total disc count in Season field for template access
	}

	return false, true // clearFolder=false for albums (they need their own folder)
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
// MusicBrainz IDs in the config's lists).
func (*handler) GetRefreshIncData(cfgp *config.MediaTypeConfig) any {
	if cfgp == nil || cfgp.ListsLen == 0 {
		return []string(nil)
	}

	args := refreshListArgs(cfgp)

	return database.GetrowsN[string](
		false,
		100,
		"select distinct dbalbums.musicbrainz_release_id from dbalbums inner join albums on albums.dbalbum_id = dbalbums.id where dbalbums.musicbrainz_release_id != '' and albums.listname in (?"+cfgp.ListsQu+") group by dbalbums.musicbrainz_release_id order by dbalbums.updated_at desc limit 100",
		args...,
	)
}

// GetRefreshFullData returns data for full refresh (all MusicBrainz IDs in the config's lists).
func (*handler) GetRefreshFullData(cfgp *config.MediaTypeConfig) any {
	if cfgp == nil || cfgp.ListsLen == 0 {
		return []string(nil)
	}

	args := refreshListArgs(cfgp)
	join := "from dbalbums inner join albums on albums.dbalbum_id = dbalbums.id where dbalbums.musicbrainz_release_id != '' and albums.listname in (?" + cfgp.ListsQu + ")"

	return database.GetrowsN[string](
		false,
		database.Getdatarow[uint](
			false,
			"select count(distinct dbalbums.musicbrainz_release_id) "+join,
			args...,
		),
		"select distinct dbalbums.musicbrainz_release_id "+join+" group by dbalbums.musicbrainz_release_id",
		args...,
	)
}

// GetSchedulerJobNames returns the job name pairs for music scheduler configuration.
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
		{logger.StrRssArtists, logger.StrRssArtists},
		{logger.StrRssArtistsUpgrade, logger.StrRssArtistsUpgrade},
		{logger.StrDataFull, logger.StrDataFull},
		{logger.StrStructure, logger.StrStructure},
		{logger.StrFeeds, logger.StrFeeds},
		{logger.StrCheckMissing, logger.StrCheckMissing},
		{logger.StrCheckMissingFlag, logger.StrCheckMissingFlag},
		{logger.StrUpgradeFlag, logger.StrUpgradeFlag},
		{"refreshmusicfull", "refresh"},
		{"refreshmusicinc", "refreshinc"},
	}
}

// CleanupAfterRemove handles cleanup after an album file is removed.
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

// MoveOtherFilesAfterOrganize handles moving additional files after main album is organized.
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

// CheckExtensions validates if a file extension is allowed for music.
// Music uses audio extensions (e.g., .mp3, .flac, .ogg).
func (h *handler) CheckExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	return mediatype.CheckAudioExtensions(pathcfg, ext)
}

// SupportsIDSearch returns false - music uses query-based search only (no IMDB/TVDB).
func (*handler) SupportsIDSearch() bool { return false }

// SupportsSeasonSearch returns false - music doesn't have season/episode structure.
func (*handler) SupportsSeasonSearch() bool { return false }

// RequiresYearCheck returns false - music doesn't require strict year matching.
func (*handler) RequiresYearCheck() bool { return false }

// HasSearchID returns false - music uses query-based search only (no standard ID in NZB results).
func (*handler) HasSearchID(_ *apiexternal_v2.Nzbwithprio) bool { return false }

// SupportsAbsoluteEpisode returns false - music doesn't have episode structure.
func (*handler) SupportsAbsoluteEpisode() bool { return false }

// HandleRSSListID does nothing for music - list ID is determined by album lookup.
func (*handler) HandleRSSListID(_ *apiexternal_v2.Nzbwithprio, _ int) {}

// CheckEpisodeMatch returns false - music doesn't have episode validation.
func (*handler) CheckEpisodeMatch(
	_, _ *apiexternal_v2.Nzbwithprio,
	_ string,
	_ func(string, *apiexternal_v2.Nzbwithprio),
) bool {
	return false
}

// SupportsVideoFile returns false - music uses audio files, not video.
func (*handler) SupportsVideoFile() bool { return false }

// GetRuntimeMultiplier returns 1 - albums are single items.
func (*handler) GetRuntimeMultiplier(_ *database.ParseInfo) int { return 1 }

// ShouldCheckOldFilePriority returns false - music doesn't check file priority.
func (*handler) ShouldCheckOldFilePriority() bool { return false }

// HasConfiguredExtensions returns true if audio extensions are configured.
func (*handler) HasConfiguredExtensions(pathcfg *config.PathsConfig) bool {
	return pathcfg.AllowedAudioExtensionsLen > 0 || pathcfg.AllowedAudioExtensionsNoRenameLen > 0
}

// IsExternalIDImdb returns false - music doesn't use IMDB.
func (*handler) IsExternalIDImdb() bool { return false }

// UsesGroupedFileProcessing returns true - music albums group tracks by folder.
func (*handler) UsesGroupedFileProcessing() bool { return true }

// GetCacheUnmatchedKey returns the cache key for unmatched albums.
func (*handler) GetCacheUnmatchedKey() string { return logger.CacheUnmatchedAlbum }

// GetCacheFilesKey returns the cache key for album files.
func (*handler) GetCacheFilesKey() string { return logger.CacheFilesAlbum }

// UsesListNameAsQualityProfile returns false - music uses quality config name.
func (*handler) UsesListNameAsQualityProfile() bool { return false }

// GetStringsMap returns a music-specific string for the given key.
func (*handler) GetStringsMap(key string) string {
	return StringsMap[key]
}

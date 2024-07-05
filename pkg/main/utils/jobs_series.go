package utils

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
)

var (
	errUnmatched = errors.New("unmatched")
)

// jobImportSeriesParseV2 parses a video file for a series episode.
// It matches the file to episodes needing import, inserts the file info,
// updates episode status, and handles caching.
func jobImportSeriesParseV2(m *database.ParseInfo, pathv string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig) error {
	if list == nil {
		return logger.ErrListnameEmpty
	}
	if list.CfgQuality == nil {
		return logger.ErrListnameEmpty
	}
	if m.DbserieID == 0 || m.SerieID == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name)
		return errUnmatched
	}

	err := m.Getepisodestoimport()
	if err != nil || len(m.Episodes) == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name)
		return err
	}

	parser.GetPriorityMapQual(m, cfgp, list.CfgQuality, true, false)
	err = parser.ParseVideoFile(m, pathv, list.CfgQuality)
	if err != nil {
		return err
	}

	reached := 0
	if m.Priority >= list.CfgQuality.CutoffPriority {
		reached = 1
	}

	basefile := filepath.Base(pathv)
	extfile := filepath.Ext(pathv)
	var count uint
	m.TempTitle = pathv
	for idx := range m.Episodes {
		database.ScanrowsNdyn(false, "select count() from serie_episode_files where location = ? and serie_episode_id = ?", &count, &m.TempTitle, &m.Episodes[idx].Num1)
		if count >= 1 {
			continue
		}

		database.ExecN("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&m.TempTitle, &basefile, &extfile, &list.Name, &m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID, &m.Proper, &m.Repack, &m.Extended, &m.SerieID, &m.Episodes[idx].Num1, &m.Episodes[idx].Num2, &m.DbserieID, &m.Height, &m.Width)

		//if updatemissing {
		database.Exec1("update serie_episodes set missing = 0 where id = ?", &m.Episodes[idx].Num1)
		database.ExecN("update serie_episodes set quality_reached = ? where id = ?", &reached, &m.Episodes[idx].Num1)
		if list.Name != "" {
			database.ExecN("update serie_episodes set quality_profile = ? where id = ?", &list.Name, &m.Episodes[idx].Num1)
		}
		//}

		database.Exec1("delete from serie_file_unmatcheds where filepath = ?", &m.TempTitle)
	}

	if config.SettingsGeneral.UseMediaCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, pathv)
		database.AppendCache(logger.CacheFilesSeries, pathv)
	}
	if m.SerieID != 0 {
		database.Scanrows1dyn(false, "select rootpath from series where id = ?", &m.TempTitle, &m.SerieID)
		if m.TempTitle == "" {
			structure.UpdateRootpath(pathv, logger.StrSeries, &m.SerieID, cfgp)
		}
	}
	return nil
}

// RefreshSerie refreshes the database data for a single series.
// It accepts a MediaTypeConfig and the series ID as a string.
// It converts the ID to an int, and calls refreshseries to refresh
// that single series, passing the config, a limit of 1 row, a query
// to select the series data, and the series ID as a query arg.
func RefreshSerie(cfgp *config.MediaTypeConfig, id string) {
	idint := logger.StringToInt(id)
	refreshseries(cfgp, database.GetrowsN[database.DbstaticTwoStringOneRInt](false, 1, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?", &idint))
}

// refreshSeries calls refreshseries to refresh all series data from the database.
// It passes the MediaTypeConfig, gets a count of all series, runs a query to select
// series data, and passes no query args.
func refreshSeries(cfgp *config.MediaTypeConfig) {
	refreshseries(cfgp, database.GetrowsN[database.DbstaticTwoStringOneRInt](false, database.GetdatarowN[uint](false, "select count() from dbseries"), "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0"))
}

// refreshSeriesInc incrementally refreshes series data for continuing shows from the database.
// It calls refreshseries, passing the MediaTypeConfig, a limit of 20 rows, a query to select
// continuing shows ordered by updated_at, and no query args.
func refreshSeriesInc(cfgp *config.MediaTypeConfig) {
	refreshseries(cfgp, database.GetrowsN[database.DbstaticTwoStringOneRInt](false, 20, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20"))
}

// refreshseries queries the database for series to refresh, iterates through the results, and calls
// JobImportDBSeriesStatic to refresh each one. It accepts a MediaTypeConfig, count of rows to process,
// query to run, and optional query argument. It returns a slice of DbstaticTwoStringOneInt structs
// containing series data.
func refreshseries(cfgp *config.MediaTypeConfig, tbl []database.DbstaticTwoStringOneRInt) {
	if len(tbl) == 0 {
		return
	}
	of := len(tbl)
	for idx := range tbl {
		logger.LogDynamicany("info", "Refresh Serie", &logger.StrTvdb, &tbl[idx].Num, "row", idx, "of", &of) //logpointer
		if err := importfeed.JobImportDBSeriesStatic(&tbl[idx], cfgp, true, false); err != nil {
			logger.LogDynamicany("error", "Import series failed", err, &logger.StrTvdb, &tbl[idx].Num)
		}
	}
}

// SeriesAllJobs runs the specified job for all configured media types that use series.
// It loops through each configured media type, and calls SingleJobs to run the job if
// the media type uses series.
func SeriesAllJobs(job string, force bool) {
	if job == "" {
		return
	}
	logger.LogDynamicany("debug", "Started Jobfor all", &logger.StrJob, &job)
	for _, media := range config.SettingsMedia {
		if !media.Useseries {
			continue
		}
		SingleJobs(job, media.NamePrefix, "", force)
	}
}

// structurefolders organizes the files in the folders configured for the given
// MediaTypeConfig into the folder structure defined by the templates. It loops
// through each configured folder, gets the template, and calls
// structuresinglefolder to organize the files.
func structurefolders(cfgp *config.MediaTypeConfig) {
	if cfgp.DataLen == 0 || len(cfgp.DataImport) == 0 {
		return
	}
	if cfgp.Data[0].CfgPath == nil {
		logger.LogDynamicany("error", "Path not found", &logger.StrConfig, &cfgp.Data[0].TemplatePath)
		return
	}
	if !cfgp.Structure {
		logger.LogDynamicany("error", "structure not allowed", &logger.StrConfig, &cfgp.NamePrefix)
		return
	}

	var defaulttemplate string
	if cfgp.DataLen >= 1 {
		defaulttemplate = cfgp.Data[0].TemplatePath
	}

	for idxi := range cfgp.DataImport {
		structurefolderloop(cfgp, &cfgp.DataImport[idxi], idxi, defaulttemplate)
	}
}

// structurefolderloop organizes media files in a folder structure based on the configuration.
// It creates a new structure instance, checks the source and target paths, and then
// iterates through the folders in the source path, organizing each folder.
func structurefolderloop(cfgp *config.MediaTypeConfig, data *config.MediaDataImportConfig, idxi int, defaulttemplate string) {
	if data.CfgPath == nil {
		logger.LogDynamicany("error", "Path not found", &logger.StrConfig, &data.TemplatePath)
		return
	}

	if idxi > 0 && cfgp.DataImport[idxi-1].CfgPath.Path == data.CfgPath.Path {
		return
	}

	entry, err := os.ReadDir(data.CfgPath.Path)
	if err == nil {
		for idx := range entry {
			if entry[idx].IsDir() {
				structure.OrganizeSingleFolder(filepath.Join(data.CfgPath.Path, entry[idx].Name()), cfgp, data, defaulttemplate, data.CfgPath.CheckRuntime, data.CfgPath.DeleteWrongLanguage, 0)
			}
		}
	}
}

// importnewseriessingle imports new series from a feed into the database.
// It gets the feed for the given list, checks for new series, and spawns
// goroutine workers to import each new series in parallel.
func importnewseriessingle(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int8) error {
	logger.LogDynamicany("info", "get feeds for", &logger.StrConfig, &cfgp.NamePrefix, &logger.StrListname, &cfgp.Lists[listid].Name)
	feed, err := feeds(cfgp, list)
	if err != nil {
		return err
	}
	if feed == nil || len(feed.Series) == 0 {
		feed.Close()
		return nil
	}

	//workergroup := worker.GetPoolParserGroup()
	wg := pool.NewSizedGroup(int(config.SettingsGeneral.WorkerParse))
	for idxserie2 := range feed.Series {
		//workergroup.Submit(func() {
		wg.Add()
		go importfeed.JobImportDBSeries(wg, &feed.Series[idxserie2], idxserie2, cfgp, listid, false, true)
	}
	wg.Wait()
	wg.Close()
	feed.Close()
	return nil
}

// checkreachedepisodesflag checks if episodes in a media list have reached
// their target quality profile based on existing files. It updates the
// quality_reached flag in the database accordingly.
func checkreachedepisodesflag(listcfg *config.MediaListsConfig) {
	arr := database.QuerySerieEpisodes(&listcfg.Name)
	for idx := range arr {
		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.LogDynamicany("debug", "Quality for Episode not found", &logger.StrID, &arr[idx].ID)
			continue
		}
		minPrio, _ := searcher.Getpriobyfiles(true, &arr[idx].ID, false, -1, config.SettingsQuality[arr[idx].QualityProfile])
		reached := 0
		if minPrio >= config.SettingsQuality[arr[idx].QualityProfile].CutoffPriority {
			reached = 1
		}
		if arr[idx].QualityReached && reached == 0 {
			database.Exec1("update Serie_episodes set quality_reached = 0 where id = ?", &arr[idx].ID)
			continue
		}

		if !arr[idx].QualityReached && reached == 1 {
			database.Exec1("update Serie_episodes set quality_reached = 1 where id = ?", &arr[idx].ID)
		}
	}
	//clear(arr)
}

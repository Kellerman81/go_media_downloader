package utils

import (
	"errors"
	"io/fs"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

const (
	jobstarted = "Started Job"
	jobended   = "Ended Job"
)

// jobImportSeriesParseV2 parses a video file for a series episode.
// It matches the file to episodes needing import, inserts the file info,
// updates episode status, and handles caching.
func jobImportSeriesParseV2(m *apiexternal.FileParser, pathv string, updatemissing bool, cfgp *config.MediaTypeConfig, listid int, list *config.MediaListsConfig) error {
	//keep list empty for auto detect list since the default list is in the listconfig!
	if listid == -1 || m.M.DbserieID == 0 || m.M.SerieID == 0 {
		structure.AddUnmatched(cfgp, &pathv, &cfgp.Lists[listid].Name, m)
		return errors.New("unmatched")
	}
	if list.CfgQuality == nil {
		return logger.ErrListnameEmpty
	}

	tblepi, err := structure.Getepisodestoimport(&m.M.SerieID, &m.M.DbserieID, m)
	if err != nil || len(tblepi) == 0 {
		structure.AddUnmatched(cfgp, &pathv, &list.Name, m)
		return err
	}

	parser.GetPriorityMapQual(&m.M, cfgp, list.CfgQuality, true, false)
	err = parser.ParseVideoFile(m, pathv, list.CfgQuality)
	if err != nil {
		//clear(tblepi)
		tblepi = nil
		return err
	}

	reached := 0
	if m.M.Priority >= list.CfgQuality.CutoffPriority {
		reached = 1
	}

	basefile := filepath.Base(pathv)
	extfile := filepath.Ext(pathv)

	var count uint
	for idx := range tblepi {
		database.ScanrowsNdyn(false, "select count() from serie_episode_files where location = ? and serie_episode_id = ?", &count, &pathv, &tblepi[idx].Num1)
		if count >= 1 {
			continue
		}

		database.ExecN("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&pathv, &basefile, &extfile, &list.Name, &m.M.ResolutionID, &m.M.QualityID, &m.M.CodecID, &m.M.AudioID, &m.M.Proper, &m.M.Repack, &m.M.Extended, &m.M.SerieID, &tblepi[idx].Num1, &tblepi[idx].Num2, &m.M.DbserieID, &m.M.Height, &m.M.Width)

		if updatemissing {
			database.ExecN("update serie_episodes set missing = 0 where id = ?", &tblepi[idx].Num1)
			database.ExecN("update serie_episodes set quality_reached = ? where id = ?", &reached, &tblepi[idx].Num1)
			if list.Name != "" {
				database.ExecN("update serie_episodes set quality_profile = ? where id = ?", &list.Name, &tblepi[idx].Num1)
			}
		}

		database.ExecN("delete from serie_file_unmatcheds where filepath = ?", &pathv)
	}
	//clear(tblepi)
	tblepi = nil

	if config.SettingsGeneral.UseMediaCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, pathv)
		database.AppendStringCache(logger.CacheFilesSeries, pathv)
	}
	if m.M.SerieID != 0 && database.GetdatarowN[string](false, "select rootpath from series where id = ?", &m.M.SerieID) == "" {
		structure.UpdateRootpath(pathv, logger.StrSeries, &m.M.SerieID, cfgp)
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
	refreshseries(cfgp, 1, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?", &idint)
}

// refreshSeries calls refreshseries to refresh all series data from the database.
// It passes the MediaTypeConfig, gets a count of all series, runs a query to select
// series data, and passes no query args.
func refreshSeries(cfgp *config.MediaTypeConfig) {
	refreshseries(cfgp, database.GetdatarowN[int](false, "select count() from dbseries"), "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0", nil)
}

// refreshSeriesInc incrementally refreshes series data for continuing shows from the database.
// It calls refreshseries, passing the MediaTypeConfig, a limit of 20 rows, a query to select
// continuing shows ordered by updated_at, and no query args.
func refreshSeriesInc(cfgp *config.MediaTypeConfig) {
	refreshseries(cfgp, 20, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20", nil)
}

// refreshseries queries the database for series to refresh, iterates through the results, and calls
// JobImportDBSeriesStatic to refresh each one. It accepts a MediaTypeConfig, count of rows to process,
// query to run, and optional query argument. It returns a slice of DbstaticTwoStringOneInt structs
// containing series data.
func refreshseries(cfgp *config.MediaTypeConfig, count int, query string, arg *int) {
	if count == 0 {
		return
	}
	tbl := getrefreshseries(count, query, arg)
	if len(tbl) == 0 {
		return
	}

	for idx := range tbl {
		logger.LogDynamic("info", "Refresh Serie", logger.NewLogField(logger.StrTvdb, &tbl[idx].Num), logger.NewLogField("row", &idx), logger.NewLogField("of", len(tbl)))
		if err := importfeed.JobImportDBSeriesStatic(&tbl[idx], cfgp, true, false); err != nil {
			logger.LogDynamic("error", "Import series failed", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrTvdb, &tbl[idx].Num))
		}
	}
	//clear(tbl)
	tbl = nil
}

// getrefreshseries queries the database to get series data for refreshing.
// It accepts a count of rows to return, a query string, and an optional query arg.
// If the count is 0, it returns nil.
// If an arg is passed, it executes the query with the arg.
// Otherwise, it executes the query without an arg.
// It returns a slice of DbstaticTwoStringOneInt structs containing the series data.
func getrefreshseries(count int, query string, arg *int) []database.DbstaticTwoStringOneInt {
	if count == 0 {
		return nil
	}
	if arg != nil {
		return database.GetrowsN[database.DbstaticTwoStringOneInt](false, count, query, arg)
	}
	return database.GetrowsN[database.DbstaticTwoStringOneInt](false, count, query)
}

// SeriesAllJobs runs the specified job for all configured media types that use series.
// It loops through each configured media type, and calls SingleJobs to run the job if
// the media type uses series.
func SeriesAllJobs(job string, force bool) {
	if job == "" {
		return
	}
	logger.LogDynamic("debug", "Started Jobfor all", logger.NewLogField(logger.StrJob, &job))
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
		logger.LogDynamic("error", "Path not found", logger.NewLogField("config", &cfgp.Data[0].TemplatePath))
		return
	}
	if !cfgp.Structure {
		logger.LogDynamic("error", "structure not allowed", logger.NewLogField("config", &cfgp.NamePrefix))
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

func structurefolderloop(cfgp *config.MediaTypeConfig, data *config.MediaDataImportConfig, idxi int, defaulttemplate string) {
	if data.CfgPath == nil {
		logger.LogDynamic("error", "Path not found", logger.NewLogField("config", &data.TemplatePath))
		return
	}

	if idxi > 0 && cfgp.DataImport[idxi-1].CfgPath.Path == data.CfgPath.Path {
		return
	}

	structurevar := structure.NewStructure(cfgp, data, data.TemplatePath, defaulttemplate)
	if structurevar == nil {
		logger.LogDynamic("error", "structure not found", logger.NewLogField("config", &data.TemplatePath))
		return
	}
	defer structurevar.Close()
	if structurevar.SourcepathCfg == nil {
		logger.LogDynamic("error", "structure source not found", logger.NewLogField("config", &data.TemplatePath))
		return
	}
	if structurevar.TargetpathCfg == nil {
		logger.LogDynamic("error", "structure target not found", logger.NewLogField("config", &data.TemplatePath))
		return
	}

	structurevar.Checkruntime = structurevar.SourcepathCfg.CheckRuntime
	structurevar.Deletewronglanguage = structurevar.SourcepathCfg.DeleteWrongLanguage

	_ = filepath.WalkDir(structurevar.SourcepathCfg.Path, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if !info.IsDir() || fpath == structurevar.SourcepathCfg.Path {
			return nil
		}
		structurevar.OrganizeSingleFolder(fpath)
		return filepath.SkipDir
	})
}

// importnewseriessingle imports new series from a feed into the database.
// It gets the feed for the given list, checks for new series, and spawns
// goroutine workers to import each new series in parallel.
func importnewseriessingle(cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, listid int) error {
	logger.LogDynamic("info", "get feeds for", logger.NewLogField("config", &cfgp.NamePrefix), logger.NewLogField(logger.StrListname, &cfgp.Lists[listid].Name))
	feed, err := feeds(cfgp, list)
	if err != nil {
		return err
	}
	defer feed.Close()
	if feed == nil {
		return logger.ErrNotFound
	}
	if len(feed.Series.Serie) == 0 {
		return nil
	}

	workergroup := worker.WorkerPoolParse.Group()
	for idxserie2 := range feed.Series.Serie {
		workergroup.Submit(func() {
			importfeed.JobImportDBSeries(&feed.Series, idxserie2, cfgp, listid, false, true)
		})
	}
	workergroup.Wait()
	return nil
}

// checkreachedepisodesflag checks if episodes in a media list have reached
// their target quality profile based on existing files. It updates the
// quality_reached flag in the database accordingly.
func checkreachedepisodesflag(listcfg *config.MediaListsConfig) {
	arr := database.QuerySerieEpisodes(&listcfg.Name)
	for idx := range arr {
		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.LogDynamic("debug", "Quality for Episode not found", logger.NewLogField(logger.StrID, int(arr[idx].ID)))
			continue
		}
		minPrio, _ := searcher.Getpriobyfiles(true, &arr[idx].ID, false, -1, config.SettingsQuality[arr[idx].QualityProfile])
		reached := 0
		if minPrio >= config.SettingsQuality[arr[idx].QualityProfile].CutoffPriority {
			reached = 1
		}
		if arr[idx].QualityReached && reached == 0 {
			database.ExecN("update Serie_episodes set quality_reached = 0 where id = ?", &arr[idx].ID)
			continue
		}

		if !arr[idx].QualityReached && reached == 1 {
			database.ExecN("update Serie_episodes set quality_reached = 1 where id = ?", &arr[idx].ID)
		}
	}
	//clear(arr)
	arr = nil
}

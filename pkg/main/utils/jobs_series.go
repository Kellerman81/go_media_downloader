package utils

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

var errUnmatched = errors.New("unmatched")

// jobImportSeriesParseV2 parses a video file for a series episode.
// It matches the file to episodes needing import, inserts the file info,
// updates episode status, and handles caching.
func jobImportSeriesParseV2(
	m *database.ParseInfo,
	pathv string,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
) error {
	if list == nil {
		return logger.ErrListnameEmpty
	}
	if list.CfgQuality == nil {
		return logger.ErrListnameEmpty
	}
	if m.DbserieID == 0 || m.SerieID == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name, errUnmatched)
		return errUnmatched
	}

	err := m.Getepisodestoimport()
	if err != nil || len(m.Episodes) == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name, err)
		return err
	}

	parser.GetPriorityMapQual(m, cfgp, list.CfgQuality, true, false)
	m.File = pathv
	err = parser.ParseVideoFile(m, list.CfgQuality)
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
	for idx := range m.Episodes {
		database.Scanrowsdyn(
			false,
			"select count() from serie_episode_files where location = ? and serie_episode_id = ?",
			&count,
			&m.File,
			&m.Episodes[idx].Num1,
		)
		if count >= 1 {
			continue
		}

		database.ExecN(
			"insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&m.File,
			&basefile,
			&extfile,
			&list.Name,
			&m.ResolutionID,
			&m.QualityID,
			&m.CodecID,
			&m.AudioID,
			&m.Proper,
			&m.Repack,
			&m.Extended,
			&m.SerieID,
			&m.Episodes[idx].Num1,
			&m.Episodes[idx].Num2,
			&m.DbserieID,
			&m.Height,
			&m.Width,
		)

		database.ExecN("update serie_episodes set missing = 0 where id = ?", &m.Episodes[idx].Num1)
		database.ExecN(
			"update serie_episodes set quality_reached = ? where id = ?",
			&reached,
			&m.Episodes[idx].Num1,
		)
		if list.Name != "" {
			database.ExecN(
				"update serie_episodes set quality_profile = ? where id = ?",
				&list.Name,
				&m.Episodes[idx].Num1,
			)
		}

		database.ExecN("delete from serie_file_unmatcheds where filepath = ?", &m.File)
	}

	if config.GetSettingsGeneral().UseMediaCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, pathv)
		database.AppendCache(logger.CacheFilesSeries, pathv)
	}
	if m.SerieID != 0 {
		if database.Getdatarow[string](
			false,
			"select rootpath from series where id = ?",
			&m.SerieID,
		) == "" {
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
func RefreshSerie(cfgp *config.MediaTypeConfig, id *string) error {
	return refreshseries(
		context.Background(),
		cfgp,
		database.GetrowsN[database.DbstaticTwoStringOneRInt](
			false,
			1,
			"select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?",
			id,
		),
	)
}

// refreshseries queries the database for series to refresh, iterates through the results, and calls
// JobImportDBSeriesStatic to refresh each one. It accepts a MediaTypeConfig, count of rows to process,
// query to run, and optional query argument. It returns a slice of DbstaticTwoStringOneInt structs
// containing series data.
func refreshseries(ctx context.Context, cfgp *config.MediaTypeConfig, tbl []database.DbstaticTwoStringOneRInt) error {
	if len(tbl) == 0 {
		return nil
	}
	of := len(tbl)
	var err error
	for idx := range tbl {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}
		logger.Logtype("info", 0).
			Int(logger.StrTvdb, tbl[idx].Num).
			Int("row", idx).
			Int("of", of).
			Msg("Refresh Serie")
		errsub := importfeed.JobImportDBSeriesStatic(&tbl[idx], cfgp)
		if errsub != nil {
			logger.Logtype("error", 1).
				Int(logger.StrTvdb, tbl[idx].Num).
				Err(errsub).
				Msg("Import series failed")
			err = errsub
		}
	}
	return err
}

// SeriesAllJobs runs the specified job for all configured media types that use series.
// It loops through each configured media type, and calls SingleJobs to run the job if
// the media type uses series.
func SeriesAllJobs(job string, force bool) {
	if job == "" {
		return
	}
	ctx := context.Background()
	logger.Logtype("debug", 1).
		Str(logger.StrJob, job).
		Msg("Started Jobfor all")
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if !media.Useseries {
			return nil
		}
		return SingleJobs(ctx, job, media.NamePrefix, "", force, 0)
	})
}

// structurefolders organizes the files in the folders configured for the given
// MediaTypeConfig into the folder structure defined by the templates. It loops
// through each configured folder, gets the template, and calls
// structuresinglefolder to organize the files.
func structurefolders(ctx context.Context, cfgp *config.MediaTypeConfig) error {
	if cfgp.DataLen == 0 || len(cfgp.DataImport) == 0 || len(cfgp.Data) == 0 {
		return nil
	}
	if cfgp.Data[0].CfgPath == nil {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, cfgp.Data[0].TemplatePath).
			Msg("Path not found")
		return errors.New("Path not found")
	}
	if !cfgp.Structure {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, cfgp.NamePrefix).
			Msg("structure not allowed")
		return nil
	}

	var defaulttemplate string
	if cfgp.DataLen >= 1 {
		defaulttemplate = cfgp.Data[0].TemplatePath
	}

	var errret error
	for idxi, dataimport := range cfgp.DataImportMap {
		if err := logger.CheckContextEnded(ctx); err != nil {
			return err
		}
		if dataimport.CfgPath == nil {
			logger.Logtype("error", 1).
				Str(logger.StrConfig, dataimport.TemplatePath).
				Msg("Path not found")
			continue
		}

		if idxi > 0 && cfgp.DataImport[idxi-1].CfgPath.Path == dataimport.CfgPath.Path {
			continue
		}

		entry, err := os.ReadDir(dataimport.CfgPath.Path)
		if err != nil {
			logger.Logtype("error", 1).
				Str(logger.StrFile, dataimport.CfgPath.Path).
				Err(err).
				Msg("Error reading directory")
			errret = err
			continue
		}
		for idx := range entry {
			if err := logger.CheckContextEnded(ctx); err != nil {
				return err
			}
			if entry[idx].IsDir() {
				err = structure.OrganizeSingleFolder(
					ctx,
					filepath.Join(dataimport.CfgPath.Path, entry[idx].Name()),
					cfgp,
					dataimport,
					defaulttemplate,
					dataimport.CfgPath.CheckRuntime,
					dataimport.CfgPath.DeleteWrongLanguage,
					0,
				)
				if err != nil {
					logger.Logtype("error", 1).
						Str(logger.StrFile, dataimport.CfgPath.Path).
						Err(err).
						Msg("Error organizing folder")
					errret = err
				}
			}
		}
	}
	return errret
}

func ReturnFeeds(feed *feedResults) {
	plfeeds.Put(feed)
}

// importnewseriessingle imports new series from a feed into the database.
// It gets the feed for the given list, checks for new series, and spawns
// goroutine workers to import each new series in parallel.
func importnewseriessingle(
	ctx context.Context, cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	listid int,
) error {
	logger.Logtype("info", 2).
		Str(logger.StrConfig, cfgp.NamePrefix).
		Str(logger.StrListname, cfgp.Lists[listid].Name).
		Msg("get feeds for")
	if !list.Enabled || !list.CfgList.Enabled {
		return logger.ErrDisabled
	}
	if list.CfgList == nil {
		return errors.New("list template not found")
	}

	feed, err := Feeds(cfgp, list, false)
	if err != nil {
		plfeeds.Put(feed)
		return err
	}
	defer plfeeds.Put(feed)
	if len(feed.Series) == 0 {
		return nil
	}

	pl := worker.WorkerPoolParse.NewGroupContext(ctx)
	for idxserie2 := range feed.Series {
		pl.Submit(func() {
			defer logger.HandlePanic()
			importfeed.JobImportDBSeries(ctx, &feed.Series[idxserie2], idxserie2, cfgp, listid)
		})
	}
	errjobs := pl.Wait()
	if errjobs != nil {
		logger.Logtype("error", 0).
			Err(errjobs).
			Msg("Error importing series")
	}
	return nil
}

// checkreachedepisodesflag checks if episodes in a media list have reached
// their target quality profile based on existing files. It updates the
// quality_reached flag in the database accordingly.
func checkreachedepisodesflag(rootctx context.Context, listcfg *config.MediaListsConfig) error {
	var minPrio, reached int
	arr := database.QuerySerieEpisodes(&listcfg.Name)
	for idx := range arr {
		if err := logger.CheckContextEnded(rootctx); err != nil {
			return err
		}
		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.Logtype("debug", 1).
				Uint(logger.StrID, arr[idx].ID).
				Msg("Quality for Episode not found")
			continue
		}
		minPrio, _ = searcher.Getpriobyfiles(
			true,
			&arr[idx].ID,
			false,
			-1,
			config.GetSettingsQuality(arr[idx].QualityProfile),
			false,
		)
		reached = 0
		if minPrio >= config.GetSettingsQuality(arr[idx].QualityProfile).CutoffPriority {
			reached = 1
		}
		if arr[idx].QualityReached && reached == 0 {
			database.ExecN(
				"update Serie_episodes set quality_reached = 0 where id = ?",
				&arr[idx].ID,
			)
			continue
		}

		if !arr[idx].QualityReached && reached == 1 {
			database.ExecN(
				"update Serie_episodes set quality_reached = 1 where id = ?",
				&arr[idx].ID,
			)
		}
	}
	return nil
}

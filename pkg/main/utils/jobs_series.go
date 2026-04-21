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
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/alitto/pond/v2"
)

func init() {
	// Register series-specific functions with the mediatype handler
	// series.RegisterImportParse(jobImportSeriesParseV2Wrapper)
	series.RegisterRefresh(refreshSeriesWrapper)
	// series.RegisterInitialFill(InitialFillSeries)
}

// Wrapper functions to match the mediatype function signatures

// func jobImportSeriesParseV2Wrapper(info *database.ParseInfo, fpath string, cfgp *config.MediaTypeConfig, list *config.MediaListsConfig, addFound bool) error {
// 	// Series doesn't use addFound parameter, uses unified function
// 	return jobImportParseCommon(info, fpath, cfgp, list, addFound)
// }

func refreshSeriesWrapper(ctx context.Context, cfgp *config.MediaTypeConfig, data any) error {
	if arr, ok := data.([]database.DbstaticTwoStringOneRInt); ok {
		return refreshseries(ctx, cfgp, arr)
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
func refreshseries(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	tbl []database.DbstaticTwoStringOneRInt,
) error {
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
		return errors.New("path not found")
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

	isAudioType := cfgp.IsType == config.MediaTypeMusic || cfgp.IsType == config.MediaTypeAudiobook

	var pl pond.TaskGroup
	if isAudioType {
		pl = worker.WorkerPoolParse.NewGroupContext(ctx)
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

			if !entry[idx].IsDir() {
				continue
			}

			folderPath := filepath.Join(dataimport.CfgPath.Path, entry[idx].Name())

			if isAudioType {
				// For music/audiobooks, recurse into subdirectories to find album folders
				// This handles Artist/Album1, Artist/Album2 structures
				structureFolderRecursive(
					ctx,
					folderPath,
					cfgp,
					dataimport,
					defaulttemplate,
					pl,
				)
			} else {
				err = structure.OrganizeSingleFolder(
					ctx,
					folderPath,
					cfgp,
					dataimport,
					defaulttemplate,
					dataimport.CfgPath.CheckRuntime,
					dataimport.CfgPath.DeleteWrongLanguage,
					0,
				)
				if err != nil {
					logger.Logtype("error", 1).
						Str(logger.StrFile, folderPath).
						Err(err).
						Msg("Error organizing folder")

					errret = err
				}
			}
		}
	}

	if pl != nil {
		if err := pl.Wait(); err != nil {
			errret = err
		}
	}

	return errret
}

// structureFolderRecursive recursively processes directories for music/audiobook types.
// If a folder contains audio files or multi-disc subfolders, it's treated as an album folder.
// Otherwise, its subdirectories are recursed into (handling Artist/Album nesting).
func structureFolderRecursive(
	ctx context.Context,
	folderPath string,
	cfgp *config.MediaTypeConfig,
	dataimport *config.MediaDataImportConfig,
	defaulttemplate string,
	pl pond.TaskGroup,
) {
	if ctx.Err() != nil {
		return
	}

	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return
	}

	hasAudioFiles := false
	hasMultiDisc := false

	var subDirs []string

	for i := range entries {
		if entries[i].IsDir() {
			if isMultiDiscSubfolder(entries[i].Name()) {
				hasMultiDisc = true
			}

			subDirs = append(
				subDirs,
				logger.JoinStringsSep([]string{folderPath, entries[i].Name()}, pathSep),
			)
		} else {
			ext := filepath.Ext(entries[i].Name())
			if ok, _ := scanner.CheckExtensionsType(
				cfgp.IsType,
				false,
				dataimport.CfgPath,
				ext,
			); ok {
				hasAudioFiles = true
			}
		}
	}

	// If this folder has audio files or multi-disc subfolders, treat it as an album
	if hasAudioFiles || hasMultiDisc {
		fp := folderPath
		pl.Submit(func() {
			defer logger.HandlePanic()
			structure.OrganizeSingleFolder(
				ctx,
				fp,
				cfgp,
				dataimport,
				defaulttemplate,
				dataimport.CfgPath.CheckRuntime,
				dataimport.CfgPath.DeleteWrongLanguage,
				0,
			)
		})
		return
	}

	// Otherwise recurse into subdirectories
	for _, subDir := range subDirs {
		if ctx.Err() != nil {
			return
		}

		structureFolderRecursive(
			ctx,
			subDir,
			cfgp,
			dataimport,
			defaulttemplate,
			pl,
		)
	}
}

func ReturnFeeds(feed *feedResults) {
	plfeeds.Put(feed)
}

// importnewseriessingle imports new series from a feed into the database.
// It gets the feed for the given list, checks for new series, and spawns
// goroutine workers to import each new series in parallel.
// func importnewseriessingle(
// 	ctx context.Context, cfgp *config.MediaTypeConfig,
// 	list *config.MediaListsConfig,
// 	listid int,
// ) error {
// 	logger.Logtype("info", 2).
// 		Str(logger.StrConfig, cfgp.NamePrefix).
// 		Str(logger.StrListname, cfgp.Lists[listid].Name).
// 		Msg("get feeds for")

// 	if !list.Enabled || !list.CfgList.Enabled {
// 		return logger.ErrDisabled
// 	}

// 	if list.CfgList == nil {
// 		return errors.New("list template not found")
// 	}

// 	feed, err := Feeds(cfgp, list, false)
// 	if err != nil {
// 		plfeeds.Put(feed)
// 		return err
// 	}
// 	defer plfeeds.Put(feed)

// 	if len(feed.Series) == 0 {
// 		return nil
// 	}

// 	pl := worker.WorkerPoolParse.NewGroupContext(ctx)
// 	for idxserie2 := range feed.Series {
// 		pl.Submit(func() {
// 			defer logger.HandlePanic()

// 			importfeed.JobImportDBSeries(ctx, &feed.Series[idxserie2], idxserie2, cfgp, listid)
// 		})
// 	}

// 	errjobs := pl.Wait()
// 	if errjobs != nil {
// 		logger.Logtype("error", 0).
// 			Err(errjobs).
// 			Msg("Error importing series")
// 	}

// 	return nil
// }

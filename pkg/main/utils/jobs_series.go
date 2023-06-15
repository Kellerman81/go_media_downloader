package utils

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/structure"
	"github.com/Kellerman81/go_media_downloader/worker"
)

func updateRootpath(file *string, objtype string, objid uint, cfgpstr string) {
	var path, firstfolder string
	for idx := range config.SettingsMedia[cfgpstr].Data {
		if !config.CheckGroup("path_", config.SettingsMedia[cfgpstr].Data[idx].TemplatePath) {
			continue
		}
		path = config.SettingsPath["path_"+config.SettingsMedia[cfgpstr].Data[idx].TemplatePath].Path
		if !logger.ContainsI(*file, path) {
			continue
		}
		firstfolder = strings.TrimLeft(strings.ReplaceAll(*file, path, ""), "/\\")
		_, firstfolder = logger.Getrootpath(filepath.Dir(firstfolder))
		database.UpdateColumnStatic("Update "+objtype+" set rootpath = ? where id = ?", logger.PathJoin(path, firstfolder), objid)
		return
	}

}

// might change listname
func jobImportSeriesParseV2(path string, updatemissing bool, cfgpstr string, listname string) error {
	if structure.CheckUnmatched(cfgpstr, &path) {
		return nil
	}
	if structure.CheckFiles(cfgpstr, &path) {
		return nil
	}
	// if logger.GlobalCache.CheckStringArrValue(logger.StrSerieFileUnmatched, path) {
	// 	return nil
	// }

	m := parser.ParseFile(&path, true, true, logger.StrSeries, true)
	defer m.Close()
	//keep list empty for auto detect list since the default list is in the listconfig!
	err := parser.GetDBIDs(&m.M, cfgpstr, "", true)
	if err != nil {
		return err
	}
	if m.M.SerieID != 0 && m.M.Listname != "" {
		listname = m.M.Listname
	}

	if listname == "" {
		return errors.New("listname empty")
	}

	if m.M.DbserieID == 0 || m.M.SerieID == 0 {
		seriesSetUnmatched(&m.M, &path, listname)
		return errors.New("unmatched")
	}

	checkfiles, err := structure.Getepisodestoimport(&m.M, m.M.SerieID, m.M.DbserieID)
	if err != nil || checkfiles == nil {
		seriesSetUnmatched(&m.M, &path, listname)
		return err
	}
	if len(*checkfiles) == 0 {
		seriesSetUnmatched(&m.M, &path, listname)
		return nil
	}

	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var templatequality string
	if i != -1 {
		templatequality = config.SettingsMedia[cfgpstr].Lists[i].TemplateQuality
	}

	parser.GetPriorityMapQual(&m.M, cfgpstr, templatequality, true, false)
	err = parser.ParseVideoFile(&m.M, &path, templatequality)
	if err != nil {
		logger.Clear(checkfiles)
		return err
	}
	var reached bool
	if m.M.Priority >= parser.NewCutoffPrio(cfgpstr, templatequality) {
		reached = true
	}
	basefile := filepath.Base(path)
	extfile := filepath.Ext(path)
	var filecount int
	for idx := range *checkfiles {
		filecount = database.QueryIntColumn("select count() from serie_episode_files where location = ? and serie_episode_id = ?", &path, (*checkfiles)[idx].Num1)
		if filecount != 0 {
			continue
		}
		if config.SettingsGeneral.UseMediaCache {
			database.CacheFilesSeries = append(database.CacheFilesSeries, path)
		}
		//cache.Append(logger.GlobalCache, "serie_episode_files_cached", path)
		database.InsertStatic("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			path, basefile, extfile, templatequality, m.M.ResolutionID, m.M.QualityID, m.M.CodecID, m.M.AudioID, m.M.Proper, m.M.Repack, m.M.Extended, m.M.SerieID, (*checkfiles)[idx].Num1, (*checkfiles)[idx].Num2, m.M.DbserieID, m.M.Height, m.M.Width)

		if updatemissing {
			database.UpdateColumnStatic("Update serie_episodes set missing = ? where id = ?", 0, (*checkfiles)[idx].Num1)
			database.UpdateColumnStatic("Update serie_episodes set quality_reached = ? where id = ?", reached, (*checkfiles)[idx].Num1)
			if templatequality != "" {
				database.UpdateColumnStatic("Update serie_episodes set quality_profile = ? where id = ?", templatequality, (*checkfiles)[idx].Num1)
			}
		}

		if database.QueryUintColumn("select id from serie_file_unmatcheds where filepath = ?", &path) != 0 {
			if config.SettingsGeneral.UseMediaCache {
				ti := logger.IndexFunc(&database.CacheUnmatchedSeries, func(elem string) bool { return elem == path })
				if ti != -1 {
					logger.Delete(&database.CacheUnmatchedSeries, ti)
				}
			}
			//logger.DeleteFromStringsCache(logger.StrSerieFileUnmatched, path)
			database.DeleteRowStatic(false, "Delete from serie_file_unmatcheds where filepath = ?", path)
		}
	}
	if database.QueryStringColumn(database.QuerySeriesGetRootpathByID, m.M.SerieID) == "" && m.M.SerieID != 0 {
		updateRootpath(&path, logger.StrSeries, m.M.SerieID, cfgpstr)
	}

	logger.Clear(checkfiles)
	return nil
}

func seriesSetUnmatched(m *apiexternal.ParseInfo, file *string, listname string) {
	id := database.QueryUintColumn("select id from serie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE", file, &listname)
	if id == 0 {
		if config.SettingsGeneral.UseMediaCache {
			database.CacheUnmatchedSeries = append(database.CacheUnmatchedSeries, *file)
		}
		//cache.Append(logger.GlobalCache, logger.StrSerieFileUnmatched, file)
		database.InsertStatic("Insert into serie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (?, ?, ?, ?)", listname, file, database.SQLTimeGetNow(), apiexternal.Buildparsedstring(m))
	} else {
		database.UpdateColumnStatic("Update serie_file_unmatcheds SET last_checked = ? where id = ?", database.SQLTimeGetNow(), id)
		database.UpdateColumnStatic("Update serie_file_unmatcheds SET parsed_data = ? where id = ?", apiexternal.Buildparsedstring(m), id)
	}
}

func RefreshSerie(id string) {
	refreshseries(1, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?", logger.StringToInt(id))
}

func RefreshSeries() {
	refreshseries(database.QueryCountColumn("dbseries", ""), "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0", 0)
}

func RefreshSeriesInc() {
	refreshseries(20, "select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20", 0)
}

func getrefreshseries(count int, query string, arg int) *[]database.DbstaticTwoStringOneInt {
	if arg != 0 {
		return database.QueryStaticColumnsTwoStringOneInt(false, count, query, arg)
	} else {
		return database.QueryStaticColumnsTwoStringOneInt(false, count, query)
	}
}
func refreshseries(count int, query string, arg int) {
	tbl := getrefreshseries(count, query, arg)
	var err error
	for idx := range *tbl {
		logger.Log.Debug().Int(logger.StrTvdb, (*tbl)[idx].Num).Msg("Refresh Serie")
		//logger.LogAnyDebug("refresh serie", logger.LoggerValue{Name: logger.StrTvdb, Value: (*tbl)[idx].Num}, logger.LoggerValue{Name: "row", Value: idx})
		err = importfeed.JobImportDBSeries(&config.SerieConfig{TvdbID: (*tbl)[idx].Num, Name: (*tbl)[idx].Str1}, config.FindconfigTemplateNameOnList("serie_", (*tbl)[idx].Str2), (*tbl)[idx].Str2, true, false)
		if err != nil {
			//logger.LogAnyError(err, "Import series failed", logger.LoggerValue{Name: logger.StrTvdb, Value: (*tbl)[idx].Num})
			logger.Log.Error().Err(err).Int(logger.StrTvdb, (*tbl)[idx].Num).Msg("Import series failed")
		}
	}
	logger.Clear(tbl)
}

func SeriesAllJobs(job string, force bool) {
	logger.Log.Info().Str(logger.StrJob, job).Msg("Started Jobfor all")
	for idxp := range config.SettingsMedia {
		if !strings.HasPrefix(config.SettingsMedia[idxp].NamePrefix, logger.StrSerie) {
			continue
		}
		SingleJobs(logger.StrSeries, job, config.SettingsMedia[idxp].NamePrefix, "", force)
	}
}

var (
	structureJobRunning string
)

func structurefolders(cfgpstr string, typ string) {
	if len(config.SettingsMedia[cfgpstr].Data) == 0 {
		return
	}
	if !config.CheckGroup("path_", config.SettingsMedia[cfgpstr].Data[0].TemplatePath) {
		//logger.LogerrorStr(nil, "config", config.SettingsMedia[cfgpstr].Data[0].TemplatePath, "Path not found")
		logger.Log.Error().Err(nil).Str("config", config.SettingsMedia[cfgpstr].Data[0].TemplatePath).Msg("Path not found")
		return
	}

	var defaulttemplate string
	if len(config.SettingsMedia[cfgpstr].Data) >= 1 {
		defaulttemplate = config.SettingsMedia[cfgpstr].Data[0].TemplatePath
	}

	cacheunmatched := logger.StrSerieFileUnmatched
	if typ != logger.StrSeries {
		cacheunmatched = logger.StrMovieFileUnmatched
	}
	lastStructure = ""
	var path string
	var files []fs.DirEntry
	var err error
	for idx := range config.SettingsMedia[cfgpstr].DataImport {
		if !config.CheckGroup("path_", config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath) {
			//logger.LogerrorStr(nil, "config", config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath, "Path not found")
			logger.Log.Error().Err(nil).Str("config", config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath).Msg("Path not found")

			continue
		}

		path = config.SettingsPath["path_"+config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath].Path
		if lastStructure == path {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastStructure = path

		if !config.SettingsMedia[cfgpstr].Structure {
			//logger.LogerrorStr(nil, "config", config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath, "structure not allowed")
			logger.Log.Error().Err(nil).Str("config", config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath).Msg("structure not allowed")
			continue
		}

		if structureJobRunning == config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath {
			logger.Log.Debug().Str(logger.StrJob, config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath).Msg("Job already running")
			continue
		}
		structureJobRunning = config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath

		files, err = os.ReadDir(path)
		if err != nil {
			continue
		}
		structurevar, _ := structure.NewStructure(cfgpstr, "", typ, "", config.SettingsMedia[cfgpstr].DataImport[idx].TemplatePath, defaulttemplate)

		for idxfile := range files {
			if files[idxfile].IsDir() {
				structure.OrganizeSingleFolder(logger.PathJoin(path, files[idxfile].Name()),
					false, false, cacheunmatched, structurevar)
			}
		}
		logger.Clear(&files)
		structurevar.Close()
	}
	lastStructure = ""
	structureJobRunning = ""
}

func importnewseriessingle(cfgpstr string, listname string) error {
	logger.Log.Info().Str(logger.StrListname, listname).Msg("Get Serie Config")
	//logger.LogAnyDebug("get serie config", logger.LoggerValue{Name: logger.StrListname, Value: listname})
	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var strtemplate string
	if i != -1 {
		strtemplate = config.SettingsMedia[cfgpstr].Lists[i].TemplateList
	}
	feed, err := feeds(cfgpstr, listname, strtemplate)
	if err != nil {
		return err
	}
	if len(feed.Series.Serie) >= 1 {
		for idxserie := range feed.Series.Serie {
			logger.Log.Debug().Str(logger.StrSeries, feed.Series.Serie[idxserie].Name).Int("row", idxserie).Msg("Import Serie")
			//logger.LogAnyDebug("Import Serie", logger.LoggerValue{Name: logger.StrSeries, Value: feed.Series.Serie[idxserie].Name}, logger.LoggerValue{Name: "row", Value: idxserie})

			serie := feed.Series.Serie[idxserie]
			worker.WorkerPoolMetadata.Submit(func() {
				err := importfeed.JobImportDBSeries(&serie, cfgpstr, listname, false, true)
				if err != nil {
					//logger.LogAnyError(err, "Import series failed", logger.LoggerValue{Name: "Serie", Value: serie.TvdbID})
					logger.Log.Error().Err(err).Int("Serie", serie.TvdbID).Msg("Import series failed")
				}
			})
		}
	}
	feed.Close()
	return nil
}

func checkreachedepisodesflag(cfgpstr string, listname string) {
	tbl := database.QuerySerieEpisodes("select id, quality_reached, quality_profile from serie_episodes where serie_id in (Select id from series where listname = ? COLLATE NOCASE)", &listname)
	var reached bool
	for idx := range *tbl {
		if !config.CheckGroup("quality_", (*tbl)[idx].QualityProfile) {
			logger.Log.Debug().Int(logger.StrID, int((*tbl)[idx].ID)).Msg("Quality for Episode not found")
			continue
		}
		reached = false
		if searcher.GetHighestEpisodePriorityByFiles(false, true, (*tbl)[idx].ID, (*tbl)[idx].QualityProfile) >= parser.NewCutoffPrio(cfgpstr, (*tbl)[idx].QualityProfile) {
			reached = true
		}
		if (*tbl)[idx].QualityReached && !reached {
			database.UpdateColumnStatic("Update Serie_episodes set quality_reached = ? where id = ?", 0, (*tbl)[idx].ID)
			continue
		}

		if !(*tbl)[idx].QualityReached && reached {
			database.UpdateColumnStatic("Update Serie_episodes set quality_reached = ? where id = ?", 1, (*tbl)[idx].ID)
		}
	}
	logger.Clear(tbl)
}

func getTraktUserPublicShowList(templatelist string) (*feedResults, error) {
	if !config.CheckGroup("list_", templatelist) {
		return nil, errors.New("list template not found")
	}
	if config.SettingsList["list_"+templatelist].TraktListType == "" {
		return nil, errors.New("list type empty")
	}
	if config.SettingsList["list_"+templatelist].TraktUsername == "" || config.SettingsList["list_"+templatelist].TraktListName == "" {
		return nil, errors.New("username empty")
	}
	data, err := apiexternal.TraktAPI.GetUserList(config.SettingsList["list_"+templatelist].TraktUsername, config.SettingsList["list_"+templatelist].TraktListName, config.SettingsList["list_"+templatelist].TraktListType, config.SettingsList["list_"+templatelist].Limit)
	if err != nil || data == nil {
		return nil, err
	}
	if len(*data) == 0 {
		return nil, logger.ErrNotFound
	}
	results := feedResults{Series: config.MainSerieConfig{Serie: make([]config.SerieConfig, 0, len(*data))}}
	for idx := range *data {
		results.Series.Serie = append(results.Series.Serie, config.SerieConfig{
			Name: (*data)[idx].Serie.Title, TvdbID: (*data)[idx].Serie.Ids.Tvdb,
		})
	}
	logger.Clear(data)
	return &results, nil
}

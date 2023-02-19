package utils

import (
	"errors"
	"fmt"
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
	"go.uber.org/zap"
)

const serieepiunmatched = "SerieEpisode not matched episode - serieepisode not found"
const seriedbunmatched = "SerieEpisode not matched episode - dbserieepisode not found"
const queryrootpathseries = "select rootpath from series where id = ?"
const querycountfilesseries = "select count() from serie_episode_files where location = ? and serie_episode_id = ?"
const queryidseriesbyseriesdbepisode = "select id from serie_episodes where serie_id = ? and dbserie_episode_id = ?"
const queryidentifiedbyseries = "select lower(identifiedby) from dbseries where id = ?"
const querycountfilesserieslocation = "select count() from serie_episode_files where location = ?"

var lastStructure string

func updateRootpath(file string, objtype string, objid uint, cfgp *config.MediaTypeConfig) {
	var firstfolder, templatepath, pathstr string
	for idx := range cfgp.Data {
		templatepath = cfgp.Data[idx].TemplatePath
		if !config.Check("path_" + templatepath) {
			continue
		}
		pathstr = config.Cfg.Paths[templatepath].Path
		if !strings.Contains(file, pathstr) {
			continue
		}
		_, firstfolder = logger.Getrootpath(filepath.Dir(strings.TrimLeft(strings.ReplaceAll(file, pathstr, ""), "/\\")))
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update " + objtype + " set rootpath = ? where id = ?", Args: []interface{}{filepath.Join(pathstr, firstfolder), objid}})
		return
	}

}

func jobImportSeriesParseV2(path string, updatemissing bool, cfgp *config.MediaTypeConfig, listname string, addfound bool) {
	var counter int
	database.QueryColumn(&database.Querywithargs{QueryString: querycountfilesserieslocation, Args: []interface{}{path}}, &counter)
	if counter >= 1 {
		return
	}
	m := parser.NewFileParser(filepath.Base(path), true, "series")
	defer m.Close()
	//keep list empty for auto detect list since the default list is in the listconfig!
	parser.GetDbIDs("series", m, cfgp, "", true)
	if m.SerieID != 0 && m.Listname != "" {
		listname = m.Listname
	}

	if listname == "" {
		return
	}

	if m.DbserieID == 0 || m.SerieID == 0 {
		seriesSetUnmatched(m, path, listname)
		return
	}
	var identifiedby string
	if database.QueryColumn(&database.Querywithargs{QueryString: queryidentifiedbyseries, Args: []interface{}{m.DbserieID}}, &identifiedby) != nil {
		return
	}

	checkfiles := seriesgetcheckfiles(m, identifiedby, path, listname)
	if len(checkfiles) == 0 {
		seriesSetUnmatched(m, path, listname)
		return
	}

	var reached bool
	templatequality := cfgp.ListsMap[listname].TemplateQuality
	cfgqual := config.Cfg.Quality[templatequality]
	parser.GetPriorityMapQual(m, cfgp, &cfgqual, true, false)
	err := parser.ParseVideoFile(m, path, templatequality)
	if err != nil {
		logger.Log.GlobalLogger.Error("Parse failed", zap.String("file", path), zap.Error(err))
		cfgqual.Close()
		return
	}
	if m.Priority >= parser.NewCutoffPrio(cfgp, &cfgqual) {
		reached = true
	}
	cfgqual.Close()
	basefile := filepath.Base(path)
	extfile := filepath.Ext(path)
	for idx := range checkfiles {
		if database.CountRowsStaticNoError(&database.Querywithargs{QueryString: querycountfilesseries, Args: []interface{}{path, checkfiles[idx].Num1}}) != 0 {
			continue
		}
		logger.InsertStringsArrCache("serie_episode_files_cached", path)
		database.InsertNamed("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :height, :width)",
			database.SerieEpisodeFile{
				Location:         path,
				Filename:         basefile,
				Extension:        extfile,
				QualityProfile:   templatequality,
				ResolutionID:     m.ResolutionID,
				QualityID:        m.QualityID,
				CodecID:          m.CodecID,
				AudioID:          m.AudioID,
				Proper:           m.Proper,
				Repack:           m.Repack,
				Extended:         m.Extended,
				SerieID:          m.SerieID,
				SerieEpisodeID:   checkfiles[idx].Num1,
				DbserieEpisodeID: checkfiles[idx].Num2,
				DbserieID:        m.DbserieID,
				Height:           m.Height,
				Width:            m.Width})

		if updatemissing {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set missing = ? where id = ?", Args: []interface{}{0, checkfiles[idx].Num1}})
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{reached, checkfiles[idx].Num1}})
			if templatequality != "" {
				database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes set quality_profile = ? where id = ?", Args: []interface{}{templatequality, checkfiles[idx].Num1}})
			}
		}

		var id uint
		database.QueryColumn(&database.Querywithargs{QueryString: "select id from serie_file_unmatcheds where filepath = ?", Args: []interface{}{path}}, &id)
		if id != 0 {
			logger.DeleteFromStringsArrCache("serie_file_unmatcheds_cached", path)
			database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from serie_file_unmatcheds where filepath = ?", Args: []interface{}{path}})
		}
	}
	var rootpath string
	err = database.QueryColumn(&database.Querywithargs{QueryString: queryrootpathseries, Args: []interface{}{m.SerieID}}, &rootpath)
	if rootpath == "" && m.SerieID != 0 && err == nil {
		updateRootpath(path, "series", m.SerieID, cfgp)
	}
}

func seriesgetcheckfiles(m *apiexternal.ParseInfo, identifiedby string, path string, listname string) []database.DbstaticTwoUint {
	episodeArray := importfeed.GetEpisodeArray(identifiedby, m.Identifier)
	if episodeArray == nil {
		return []database.DbstaticTwoUint{}
	}
	defer episodeArray.Close()
	lenarray := len(episodeArray.Arr)
	if lenarray == 0 {
		return []database.DbstaticTwoUint{}
	}

	checkfiles := make([]database.DbstaticTwoUint, 0, lenarray)
	if lenarray == 1 && m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
		checkfiles = append(checkfiles, database.DbstaticTwoUint{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID})
		//logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", m.DbserieEpisodeID), zap.Uint("serieepisodeid", m.SerieEpisodeID))
	} else {
		var dbserieepisodeid, serieepisodeid uint
		cntseries := database.Querywithargs{QueryString: queryidseriesbyseriesdbepisode}
		for idx := range episodeArray.Arr {
			if episodeArray.Arr[idx] == "" {
				continue
			}
			episodeArray.Arr[idx] = strings.Trim(episodeArray.Arr[idx], "-EX")
			if identifiedby != "date" {
				episodeArray.Arr[idx] = strings.TrimLeft(episodeArray.Arr[idx], "0")
			}

			dbserieepisodeid = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(m.DbserieID, m.Identifier, m.SeasonStr, episodeArray.Arr[idx])
			if dbserieepisodeid != 0 {
				cntseries.Args = []interface{}{m.SerieID, dbserieepisodeid}
				database.QueryColumn(&cntseries, &serieepisodeid)
				if serieepisodeid != 0 {
					checkfiles = append(checkfiles, database.DbstaticTwoUint{Num1: serieepisodeid, Num2: dbserieepisodeid})
					//logger.Log.GlobalLogger.Debug("SerieEpisode matched - found", zap.String("file", file), zap.String("title", m.Title), zap.String("Resolution", m.Resolution), zap.String("Quality", m.Quality), zap.String("Codec", m.Codec), zap.String("Audio", m.Audio), zap.String("Identifier", m.Identifier), zap.Uint("dbserieepisodeid", dbserieepisodeid), zap.Uint("serieepisodeid", serieepisodeid))
				} else {
					logger.Log.GlobalLogger.Debug(serieepiunmatched, zap.Stringp("file", &path))
					checkfiles = nil
					return []database.DbstaticTwoUint{}
				}
			} else {
				logger.Log.GlobalLogger.Debug(seriedbunmatched, zap.Stringp("file", &path))
				checkfiles = nil
				return []database.DbstaticTwoUint{}
			}
		}
	}
	return checkfiles
}

func seriesSetUnmatched(m *apiexternal.ParseInfo, file string, listname string) {
	var id uint
	database.QueryColumn(&database.Querywithargs{QueryString: "select id from serie_file_unmatcheds where filepath = ? and listname = ?", Args: []interface{}{file, listname}}, &id)
	if id == 0 {
		logger.InsertStringsArrCache("serie_file_unmatcheds", file)
		database.InsertStatic(&database.Querywithargs{QueryString: "Insert into serie_file_unmatcheds (listname, filepath, last_checked, parsed_data) values (?, ?, ?, ?)", Args: []interface{}{listname, file, logger.SqlTimeGetNow(), buildparsedstring(m)}})
	} else {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_file_unmatcheds SET last_checked = ? where id = ?", Args: []interface{}{logger.SqlTimeGetNow(), id}})
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_file_unmatcheds SET parsed_data = ? where id = ?", Args: []interface{}{buildparsedstring(m), id}})
	}
}

func RefreshSerie(id string) {
	refreshseriesquery("select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where id = ?", id)
}

func RefreshSeries() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}
	refreshseriesquery("select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where thetvdb_id != 0")
}

func RefreshSeriesInc() {
	if config.Cfg.General.SchedulerDisabled {
		return
	}

	refreshseriesquery("select seriename, (Select listname from series where dbserie_id=dbseries.id limit 1), thetvdb_id from dbseries where status = 'Continuing' and thetvdb_id != 0 order by updated_at asc limit 20")
}

func refreshseriesquery(query string, args ...interface{}) {
	var dbseries []database.DbstaticTwoStringOneInt
	database.QueryStaticColumnsTwoStringOneInt(false, 0, &database.Querywithargs{QueryString: query, Args: args}, &dbseries)
	var oldlistname string
	var cfgp *config.MediaTypeConfig
	for idxserie := range dbseries {
		logger.Log.GlobalLogger.Info("Refresh Serie ", zap.Int("row", idxserie), zap.Int("row count", len(dbseries)), zap.Int("tvdb", dbseries[idxserie].Num))
		if oldlistname != dbseries[idxserie].Str2 {
			cfgp.Close()
			cfgp = config.FindconfigTemplateOnList("serie_", dbseries[idxserie].Str2)
			oldlistname = dbseries[idxserie].Str2
		}
		importfeed.JobImportDbSeries(&config.SerieConfig{TvdbID: dbseries[idxserie].Num, Name: dbseries[idxserie].Str1}, cfgp, dbseries[idxserie].Str2, true, false)
	}
	cfgp.Close()
	dbseries = nil
}

func SeriesAllJobs(job string, force bool) {
	logger.Log.GlobalLogger.Info("Started Jobfor all", zap.Stringp("Job", &job))
	for idx := range config.Cfg.Series {
		SingleJobs("series", job, config.Cfg.Series[idx].NamePrefix, "", force)
	}
}

func structurefolders(cfgp *config.MediaTypeConfig, typ string) {
	if !config.Check("path_" + cfgp.Data[0].TemplatePath) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("config", cfgp.Data[0].TemplatePath))
		return
	}

	var templatepath string
	for idx := range cfgp.DataImport {
		templatepath = cfgp.DataImport[idx].TemplatePath
		if !config.Check("path_" + templatepath) {
			logger.Log.GlobalLogger.Error("Path not found", zap.String("config", templatepath))

			continue
		}

		if lastStructure == config.Cfg.Paths[templatepath].Path {
			time.Sleep(time.Duration(15) * time.Second)
		}
		lastStructure = config.Cfg.Paths[templatepath].Path

		structure.OrganizeFolders(typ, templatepath, cfgp.Data[0].TemplatePath, cfgp)
	}
}

func getjoblists(cfgp *config.MediaTypeConfig, listname string) []config.MediaListsConfig {
	if listname != "" {
		return []config.MediaListsConfig{cfgp.ListsMap[listname]}
	}
	return cfgp.Lists
}

func importnewseriessingle(cfgp *config.MediaTypeConfig, listname string) {
	logger.Log.GlobalLogger.Info("Get Serie Config ", zap.Stringp("Listname", &listname))
	cfglist := config.Cfg.Lists[cfgp.ListsMap[listname].TemplateList]
	feed, err := feeds(cfgp, listname, &cfglist)
	cfglist.Close()
	if err != nil {
		return
	}
	if len(feed.Series.Serie) >= 1 {
		workergroup := logger.WorkerPools["Metadata"].Group()
		for idxserie := range feed.Series.Serie {
			serie := feed.Series.Serie[idxserie]
			logger.Log.GlobalLogger.Info("Import Serie ", zap.Int("row", idxserie), zap.Stringp("serie", &serie.Name))
			workergroup.Submit(func() {
				importfeed.JobImportDbSeries(&serie, cfgp, listname, false, true)
			})
		}
		workergroup.Wait()
	}
	feed.Close()
}

func checkreachedepisodesflag(cfgp *config.MediaTypeConfig, listname string) {
	var episodes []database.SerieEpisode
	database.QuerySerieEpisodes(&database.Querywithargs{Query: database.Query{Select: "id, quality_reached, quality_profile", Where: "serie_id in (Select id from series where listname = ? COLLATE NOCASE)"}, Args: []interface{}{listname}}, &episodes)
	var reached bool
	for idxepi := range episodes {
		if !config.Check("quality_" + episodes[idxepi].QualityProfile) {
			logger.Log.GlobalLogger.Error(fmt.Sprintf("Quality for Episode: %d not found", episodes[idxepi].ID))
			continue
		}
		reached = false
		if searcher.GetHighestEpisodePriorityByFilesGetQual(false, true, episodes[idxepi].ID, cfgp, episodes[idxepi].QualityProfile) >= parser.NewCutoffPrioGetQual(cfgp, episodes[idxepi].QualityProfile) {
			reached = true
		}
		if episodes[idxepi].QualityReached && !reached {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update Serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{0, episodes[idxepi].ID}})
			continue
		}

		if !episodes[idxepi].QualityReached && reached {
			database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update Serie_episodes set quality_reached = ? where id = ?", Args: []interface{}{1, episodes[idxepi].ID}})
		}
	}
	episodes = nil
}

func getTraktUserPublicShowList(templatelist string, cfglist *config.ListsConfig) (*feedResults, error) {
	if !config.Check("list_" + templatelist) {
		return nil, errNoList
	}
	if cfglist.TraktListType == "" {
		return nil, errors.New("not show")
	}
	if cfglist.TraktUsername == "" || cfglist.TraktListName == "" {
		return nil, errors.New("no user")
	}
	data, err := apiexternal.TraktAPI.GetUserList(cfglist.TraktUsername, cfglist.TraktListName, cfglist.TraktListType, cfglist.Limit)
	if err != nil {
		logger.Log.GlobalLogger.Error("Failed to read trakt list", zap.String("Listname", cfglist.TraktListName))
		return nil, errNoListRead
	}
	results := feedResults{Series: config.MainSerieConfig{Serie: []config.SerieConfig{}}}
	for idx := range data.Entries {
		results.Series.Serie = append(results.Series.Serie, config.SerieConfig{
			Name: data.Entries[idx].Serie.Title, TvdbID: data.Entries[idx].Serie.Ids.Tvdb,
		})
	}
	data.Close()
	return &results, nil
}

package utils

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/remeh/sizedwaitgroup"
)

var SeriesImportJobRunning map[string]bool

func JobImportDbSeries(cfg config.Cfg, serieconfig config.SerieConfig, configEntry config.MediaTypeConfig, list config.MediaListsConfig, checkall bool, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := serieconfig.Name
	if jobName == "" {
		jobName = list.Name
	}
	defer func() {
		database.ReadWriteMu.Lock()
		delete(SeriesImportJobRunning, jobName)
		database.ReadWriteMu.Unlock()
		wg.Done()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := SeriesImportJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		SeriesImportJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()

	var dbserie database.Dbserie
	dbserieadded := false

	if strings.EqualFold(serieconfig.Source, "none") {
		dbserie.Seriename = serieconfig.Name
		dbserie.Identifiedby = serieconfig.Identifiedby

		finddbserie, _ := database.GetDbserie(database.Query{Where: "Seriename = ?", WhereArgs: []interface{}{serieconfig.Name}})

		cdbserie, _ := database.CountRows("dbseries", database.Query{Where: "Seriename = ?", WhereArgs: []interface{}{serieconfig.Name}})
		if cdbserie == 0 {
			dbserieadded = true
			inres, inreserr := database.InsertArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
				[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby})
			if inreserr != nil {
				logger.Log.Error(inreserr)
				return
			}
			newid, newiderr := inres.LastInsertId()
			if newiderr != nil {
				logger.Log.Error(newiderr)
				return
			}
			dbserie.ID = uint(newid)
		} else {
			dbserie = finddbserie
		}
	} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
		dbserie.ThetvdbID = serieconfig.TvdbID
		dbserie.Identifiedby = serieconfig.Identifiedby
		finddbserie, _ := database.GetDbserie(database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
		cdbserie, _ := database.CountRows("dbseries", database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
		if cdbserie == 0 {
			logger.Log.Debug("DbSeries get metadata for: ", serieconfig.TvdbID)
			addaliases := dbserie.GetMetadata("", cfg.General.SerieMetaSourceTmdb, cfg.Imdb.Indexedlanguages, cfg.General.SerieMetaSourceTrakt, false)
			if dbserie.Seriename == "" {
				addaliases = dbserie.GetMetadata(configEntry.Metadata_language, cfg.General.SerieMetaSourceTmdb, cfg.Imdb.Indexedlanguages, cfg.General.SerieMetaSourceTrakt, false)
			}
			serieconfig.AlternateName = append(serieconfig.AlternateName, addaliases...)
			dbserieadded = true
			cdbserie2, _ := database.CountRows("dbseries", database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
			if cdbserie2 == 0 {
				inres, inreserr := database.InsertArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
					[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby})
				if inreserr != nil {
					logger.Log.Error(inreserr)
					return
				}
				newid, newiderr := inres.LastInsertId()
				if newiderr != nil {
					logger.Log.Error(newiderr)
					return
				}
				dbserie.ID = uint(newid)
			} else {
				dbserie, _ = database.GetDbserie(database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{serieconfig.TvdbID}})
			}
			logger.Log.Debug("DbSeries get metadata end for: ", serieconfig.TvdbID)
		} else {
			dbserie = finddbserie
		}
	}

	serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
	serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
	for idxalt := range serieconfig.AlternateName {
		countera, _ := database.CountRows("dbserie_alternates", database.Query{Where: "Dbserie_id = ? and title = ?", WhereArgs: []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt]}})
		if countera == 0 {
			database.InsertArray("dbserie_alternates", []string{"dbserie_id", "title"}, []interface{}{dbserie.ID, serieconfig.AlternateName[idxalt]})
		}
	}

	var serie database.Serie
	findserie, serieerr := database.GetSeries(database.Query{Where: "Dbserie_id = ? and listname = ?", WhereArgs: []interface{}{dbserie.ID, list.Name}})
	if serieerr != nil {
		logger.Log.Debug("Series add for: ", serieconfig.TvdbID)
		inres, inreserr := database.InsertArray("series", []string{"dbserie_id", "listname", "rootpath"}, []interface{}{dbserie.ID, list.Name, serieconfig.Target})
		if inreserr != nil {
			logger.Log.Error(inreserr)
			return
		}
		newid, newiderr := inres.LastInsertId()
		if newiderr != nil {
			logger.Log.Error(newiderr)
			return
		}
		serie.ID = uint(newid)
	} else {
		serie = findserie
	}
	if checkall || dbserieadded {
		if strings.EqualFold(serieconfig.Source, "none") {
			//Don't add episodes automatically
		} else if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, "tvdb") {
			logger.Log.Debug("DbSeries get episodes for: ", serieconfig.TvdbID)
			episode := dbserie.GetEpisodes(configEntry.Metadata_language, cfg.General.SerieMetaSourceTrakt)
			adddbepisodes := make([]database.DbserieEpisode, 0, len(episode))
			for idxepi := range episode {
				countere, _ := database.CountRows("dbserie_episodes", database.Query{Where: "Dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{dbserie.ID, episode[idxepi].Season, episode[idxepi].Episode}})
				if countere == 0 {
					dbserieepisode := episode[idxepi]
					dbserieepisode.DbserieID = dbserie.ID
					adddbepisodes = append(adddbepisodes, dbserieepisode)
				}
			}
			if len(adddbepisodes) >= 1 {
				database.ReadWriteMu.Lock()
				database.DB.NamedExec("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (:episode, :season, :identifier, :title, :first_aired, :overview, :poster, :dbserie_id)", adddbepisodes)
				database.ReadWriteMu.Unlock()
				// for idxepi := range adddbepisodes {
				// 	database.InsertArray("dbserie_episodes", []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"},
				// 		[]interface{}{adddbepisodes[idxepi].Episode, adddbepisodes[idxepi].Season, adddbepisodes[idxepi].Identifier, adddbepisodes[idxepi].Title, adddbepisodes[idxepi].FirstAired, adddbepisodes[idxepi].Overview, adddbepisodes[idxepi].Poster, adddbepisodes[idxepi].DbserieID})
				// }
			}
		}

		dbepisode, _ := database.QueryDbserieEpisodes(database.Query{Where: "dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})
		for idxdbepi := range dbepisode {
			counterie, _ := database.CountRows("serie_episodes", database.Query{Where: "serie_id = ? and Dbserie_episode_id = ?", WhereArgs: []interface{}{serie.ID, dbepisode[idxdbepi].ID}})
			if counterie == 0 {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, serie.ID, true, list.Template_quality, dbepisode[idxdbepi].ID})
			}
		}

		logger.Log.Debug("DbSeries add episodes end for: ", serieconfig.TvdbID)
	} else {
		dbepisode, _ := database.QueryDbserieEpisodes(database.Query{Where: "dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})
		for idxdbepi := range dbepisode {
			counterid, _ := database.CountRows("serie_episodes", database.Query{Where: "serie_id = ? and Dbserie_episode_id = ?", WhereArgs: []interface{}{serie.ID, dbepisode[idxdbepi].ID}})
			if counterid == 0 {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, serie.ID, true, list.Template_quality, dbepisode[idxdbepi].ID})
			}
		}
		logger.Log.Debug("DbSeries add episodes end for: ", serieconfig.TvdbID)
	}
}

func JobReloadDbSeries(cfg config.Cfg, dbserie database.Dbserie, configEntry config.MediaTypeConfig, list config.MediaListsConfig, checkall bool, wg *sizedwaitgroup.SizedWaitGroup) {
	jobName := dbserie.Seriename
	if jobName == "" {
		jobName = list.Name
	}
	defer func() {
		database.ReadWriteMu.Lock()
		delete(SeriesImportJobRunning, jobName)
		database.ReadWriteMu.Unlock()
		wg.Done()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := SeriesImportJobRunning[jobName]; nok {
		if SeriesImportJobRunning[jobName] {
			logger.Log.Debug("Job already running: ", jobName)
			database.ReadWriteMu.Unlock()
			return
		}
	} else {
		SeriesImportJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()

	logger.Log.Debug("DbSeries Add for: ", dbserie.ThetvdbID)

	dbserie, _ = database.GetDbserie(database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{dbserie.ThetvdbID}})
	logger.Log.Debug("DbSeries get metadata for: ", dbserie.ThetvdbID)
	addaliases := dbserie.GetMetadata("", cfg.General.SerieMetaSourceTmdb, cfg.Imdb.Indexedlanguages, cfg.General.SerieMetaSourceTrakt, false)
	if dbserie.Seriename == "" {
		addaliases = dbserie.GetMetadata(configEntry.Metadata_language, cfg.General.SerieMetaSourceTmdb, cfg.Imdb.Indexedlanguages, cfg.General.SerieMetaSourceTrakt, false)
	}
	alternateNames := make([]string, 0, len(addaliases)+1)
	alternateNames = append(alternateNames, addaliases...)
	alternateNames = append(alternateNames, dbserie.Seriename)

	database.UpdateArray("dbseries", []string{"Seriename", "Aliases", "Season", "Status", "Firstaired", "Network", "Runtime", "Language", "Genre", "Overview", "Rating", "Siterating", "Siterating_Count", "Slug", "Trakt_ID", "Imdb_ID", "Thetvdb_ID", "Freebase_M_ID", "Freebase_ID", "Tvrage_ID", "Facebook", "Instagram", "Twitter", "Banner", "Poster", "Fanart", "Identifiedby"},
		[]interface{}{dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby},
		database.Query{Where: "id=?", WhereArgs: []interface{}{dbserie.ID}})

	logger.Log.Debug("DbSeries get metadata end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries add titles for: ", dbserie.ThetvdbID)
	for idxalt := range alternateNames {
		counter, _ := database.CountRows("dbserie_alternates", database.Query{Where: "Dbserie_id = ? and title = ?", WhereArgs: []interface{}{dbserie.ID, alternateNames[idxalt]}})
		if counter == 0 {
			database.InsertArray("dbserie_alternates",
				[]string{"dbserie_id", "title"},
				[]interface{}{dbserie.ID, alternateNames[idxalt]})
		}
	}

	logger.Log.Debug("DbSeries add titles end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries add serie end for: ", dbserie.ThetvdbID)

	logger.Log.Debug("DbSeries get episodes for: ", dbserie.ThetvdbID)
	episode := dbserie.GetEpisodes(configEntry.Metadata_language, cfg.General.SerieMetaSourceTrakt)
	logger.Log.Debug("DbSeries get episodes end for: ", dbserie.ThetvdbID)
	adddbepisodes := make([]database.DbserieEpisode, 0, len(episode))
	for idxdbepi := range episode {
		counter, _ := database.CountRows("dbserie_episodes", database.Query{Where: "Dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{dbserie.ID, episode[idxdbepi].Season, episode[idxdbepi].Episode}})
		if counter == 0 {
			dbserieepisode := episode[idxdbepi]
			dbserieepisode.DbserieID = dbserie.ID
			adddbepisodes = append(adddbepisodes, dbserieepisode)
		}
	}
	if len(adddbepisodes) >= 1 {
		database.ReadWriteMu.Lock()
		database.DB.NamedExec("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (:episode, :season, :identifier, :title, :first_aired, :overview, :poster, :dbserie_id)", adddbepisodes)
		database.ReadWriteMu.Unlock()
		// for idxepi := range adddbepisodes {
		// 	database.InsertArray("dbserie_episodes", []string{"episode", "season", "identifier", "title", "first_aired", "overview", "poster", "dbserie_id"},
		// 		[]interface{}{adddbepisodes[idxepi].Episode, adddbepisodes[idxepi].Season, adddbepisodes[idxepi].Identifier, adddbepisodes[idxepi].Title, adddbepisodes[idxepi].FirstAired, adddbepisodes[idxepi].Overview, adddbepisodes[idxepi].Poster, adddbepisodes[idxepi].DbserieID})
		// }
	}

	foundseries, _ := database.QuerySeries(database.Query{Where: "Dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})
	for idxserie := range foundseries {
		dbepisode, _ := database.QueryDbserieEpisodes(database.Query{Where: "Dbserie_id = ?", WhereArgs: []interface{}{dbserie.ID}})

		for idxdbepi := range dbepisode {
			counter, _ := database.CountRows("serie_episodes", database.Query{Where: "Serie_id = ? and Dbserie_episode_id = ?", WhereArgs: []interface{}{foundseries[idxserie].ID, dbepisode[idxdbepi].ID}})
			if counter == 0 {
				database.InsertArray("serie_episodes",
					[]string{"dbserie_id", "serie_id", "missing", "quality_profile", "dbserie_episode_id"},
					[]interface{}{dbserie.ID, foundseries[idxserie].ID, true, list.Template_quality, dbepisode[idxdbepi].ID})
			}
		}
	}

	logger.Log.Debug("DbSeries add episodes end for: ", dbserie.ThetvdbID)
}

func findSerieByParser(m ParseInfo, titleyear string, seriestitle string, listname string) (database.Serie, int) {
	var entriesfound int

	if m.Tvdb != "" {
		findseries, _ := database.QuerySeries(database.Query{Select: "series.*", InnerJoin: "Dbseries ON Dbseries.ID = Series.Dbserie_id", Where: "DbSeries.thetvdb_id = ? AND Series.listname = ?", WhereArgs: []interface{}{strings.Replace(m.Tvdb, "tvdb", "", -1), listname}})

		if len(findseries) == 1 {
			entriesfound = len(findseries)
			return findseries[0], entriesfound
		}
	}
	if entriesfound == 0 && titleyear != "" {
		findseries, _ := database.QuerySeries(database.Query{Select: "series.*", InnerJoin: "Dbseries ON Dbseries.ID = Series.Dbserie_id", Where: "DbSeries.Seriename = ? COLLATE NOCASE AND Series.listname = ?", WhereArgs: []interface{}{titleyear, listname}})

		if len(findseries) == 1 {
			entriesfound = len(findseries)
			return findseries[0], entriesfound
		}
	}
	if entriesfound == 0 && seriestitle != "" {
		findseries, _ := database.QuerySeries(database.Query{Select: "Series.*", InnerJoin: "Dbseries ON Dbseries.ID = Series.Dbserie_id", Where: "DbSeries.Seriename = ? COLLATE NOCASE AND Series.listname = ?", WhereArgs: []interface{}{seriestitle, listname}})

		if len(findseries) == 1 {
			entriesfound = len(findseries)
			return findseries[0], entriesfound
		}
	}
	if entriesfound == 0 && m.Title != "" {
		findseries, _ := database.QuerySeries(database.Query{Select: "Series.*", InnerJoin: "Dbseries ON Dbseries.ID = Series.Dbserie_id", Where: "DbSeries.Seriename = ? COLLATE NOCASE AND Series.listname = ?", WhereArgs: []interface{}{m.Title, listname}})

		if len(findseries) == 1 {
			entriesfound = len(findseries)
			return findseries[0], entriesfound
		}
	}
	if entriesfound == 0 && titleyear != "" {
		dbseries, _ := database.QueryDbserie(database.Query{Select: "dbseries.*", InnerJoin: "Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id", Where: "Dbserie_alternates.Title = ? COLLATE NOCASE", WhereArgs: []interface{}{titleyear}})
		if len(dbseries) >= 1 {
			findseries, _ := database.QuerySeries(database.Query{Where: "DbSerie_id = ? AND listname = ?", WhereArgs: []interface{}{dbseries[0].ID, listname}})

			if len(findseries) == 1 {
				entriesfound = len(findseries)
				return findseries[0], entriesfound
			}
		}
	}
	if entriesfound == 0 && seriestitle != "" {
		dbseries, _ := database.QueryDbserie(database.Query{Select: "dbseries.*", InnerJoin: "Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id", Where: "Dbserie_alternates.Title = ? COLLATE NOCASE", WhereArgs: []interface{}{seriestitle}})
		if len(dbseries) >= 1 {
			findseries, _ := database.QuerySeries(database.Query{Where: "DbSerie_id = ? AND listname = ?", WhereArgs: []interface{}{dbseries[0].ID, listname}})

			if len(findseries) == 1 {
				entriesfound = len(findseries)
				return findseries[0], entriesfound
			}
		}
	}
	if entriesfound == 0 && m.Title != "" {
		dbseries, _ := database.QueryDbserie(database.Query{Select: "dbseries.*", InnerJoin: "Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id", Where: "Dbserie_alternates.Title = ? COLLATE NOCASE", WhereArgs: []interface{}{m.Title}})
		if len(dbseries) >= 1 {
			findseries, _ := database.QuerySeries(database.Query{Where: "DbSerie_id = ? AND listname = ?", WhereArgs: []interface{}{dbseries[0].ID, listname}})

			if len(findseries) == 1 {
				entriesfound = len(findseries)
				return findseries[0], entriesfound
			}
		}
	}
	return database.Serie{}, 0
}
func getEpisodeArray(identifiedby string, str1 string, str2 string) []string {
	episodeArray := make([]string, 0, 10)
	if strings.EqualFold(identifiedby, "date") {
		str1 = str2
		str1 = strings.Replace(str1, " ", "-", -1)
		str1 = strings.Replace(str1, ".", "-", -1)
	}
	str1 = strings.ToUpper(str1)
	if strings.Contains(str1, "E") {
		episodeArray = strings.Split(str1, "E")
	} else if strings.Contains(str1, "X") {
		episodeArray = strings.Split(str1, "X")
	} else if strings.Contains(str1, "-") && !strings.EqualFold(identifiedby, "date") {
		episodeArray = strings.Split(str1, "-")
	}
	if len(episodeArray) == 0 && strings.EqualFold(identifiedby, "date") {
		episodeArray = append(episodeArray, str1)
	}
	return episodeArray
}
func JobImportSeriesParseV2(cfg config.Cfg, file string, updatemissing bool, configEntry config.MediaTypeConfig, list config.MediaListsConfig, minPrio ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	logger.Log.Debug("Series Parse: ", file)

	filecounter, _ := database.CountRows("serie_episode_files", database.Query{InnerJoin: "Series ON Series.ID = Serie_episode_files.serie_id", Where: "Serie_episode_files.location = ? and series.listname = ? and Serie_episode_files.serie_episode_id <> 0", WhereArgs: []interface{}{file, list.Name}})
	if filecounter >= 1 {
		return
	}

	parsecounter, _ := database.CountRows("serie_file_unmatcheds", database.Query{Where: "filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", WhereArgs: []interface{}{file, list.Name, time.Now().Add(time.Hour * -12)}})
	if parsecounter >= 1 {
		return
	}

	m, err := NewFileParser(cfg, filepath.Base(file), true, "series")
	if err != nil {
		return
	}
	m.Resolution = strings.ToLower(m.Resolution)
	m.Audio = strings.ToLower(m.Audio)
	m.Codec = strings.ToLower(m.Codec)
	m.Quality = strings.ToLower(m.Quality)
	yearstr := strconv.Itoa(m.Year)
	var titleyear string
	if m.Year != 0 {
		titleyear = m.Title + " (" + yearstr + ")"
	} else {
		titleyear = m.Title
	}
	seriestitle := ""
	re, _ := regexp.Compile(`^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)[ex](?:[0-9]{1,3})(?:[^0-9]|$))`)
	matched := re.FindStringSubmatch(filepath.Base(file))
	if len(matched) >= 2 {
		seriestitle = matched[1]
	}
	logger.Log.Debug("Parsed SerieEpisode: ", file, " as Title: ", m.Title, " TitleYear:  ", titleyear, " Matched: ", matched, " Identifier: ", m.Identifier, " Date: ", m.Date, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
	//logger.Log.Debug("Parse Data: ", m)

	//find dbseries
	series, entriesfound := findSerieByParser(*m, titleyear, seriestitle, list.Name)
	if entriesfound >= 1 {
		cutoffPrio := NewCutoffPrio(cfg, configEntry, cfg.Quality[list.Template_quality])

		m.GetPriority(configEntry, cfg.Quality[list.Template_quality])
		errparsev := m.ParseVideoFile(file, configEntry, cfg.Quality[list.Template_quality])
		if errparsev != nil {
			return
		}
		r := regexp.MustCompile(`(?i)s?[0-9]{1,4}((?:(?:-?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`)
		teststr := r.FindStringSubmatch(m.Identifier)
		if len(teststr) == 0 {
			logger.Log.Warn("Failed parse identifier: ", file, " as ", m.Title, m.Identifier)
			return
		}

		testDbSeries, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})

		episodeArray := getEpisodeArray(testDbSeries.Identifiedby, teststr[1], teststr[2])

		for _, episodestr := range episodeArray {
			episodestr = strings.Trim(episodestr, "-")
			episodestr = strings.Trim(episodestr, "E")
			episodestr = strings.Trim(episodestr, "X")
			epi := strings.TrimLeft(episodestr, "0")
			if epi == "" {
				continue
			}
			logger.Log.Debug("Episode Identifier: ", epi)

			var SeriesEpisode database.SerieEpisode
			var SeriesEpisodeErr error
			if strings.EqualFold(testDbSeries.Identifiedby, "date") {
				SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ?", WhereArgs: []interface{}{series.ID, strings.Replace(epi, ".", "-", -1)}})
			} else {
				SeriesEpisode, SeriesEpisodeErr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Season = ? AND DbSerie_episodes.Episode = ?", WhereArgs: []interface{}{series.ID, m.Season, epi}})
			}
			if SeriesEpisodeErr == nil {
				_, SeriesEpisodeFileerr := database.GetSerieEpisodeFiles(database.Query{Where: "location = ? AND serie_episode_id = ?", WhereArgs: []interface{}{file, SeriesEpisode.ID}})
				if SeriesEpisodeFileerr != nil {
					if SeriesEpisode.DbserieID == 0 {
						logger.Log.Warn("Failed parse match sub1: ", file, " as ", m.Title)
						continue
					}
					reached := false
					if m.Priority >= cutoffPrio.Priority {
						reached = true
					}
					if series.Rootpath == "" && series.ID != 0 {
						rootpath := ""
						for idxpath := range configEntry.Data {
							pppath := cfg.Path[configEntry.Data[idxpath].Template_path].Path
							if strings.Contains(file, pppath) {
								rootpath = pppath
								tempfoldername := strings.Replace(file, pppath, "", -1)
								tempfoldername = strings.TrimLeft(tempfoldername, "/")
								tempfoldername = strings.TrimLeft(tempfoldername, "\\")
								tempfoldername = filepath.Dir(tempfoldername)
								_, firstfolder := getrootpath(tempfoldername)
								rootpath = filepath.Join(rootpath, firstfolder)
								break
								// if strings.Contains(tempfoldername, "/") {
								// 	folders := strings.Split(tempfoldername, "/")
								// 	rootpath = filepath.Join(rootpath, folders[0])
								// }
								// if strings.Contains(tempfoldername, "\\") {
								// 	folders := strings.Split(tempfoldername, "\\")
								// 	rootpath = filepath.Join(rootpath, folders[0])
								// }
								// if !strings.Contains(tempfoldername, "/") && !strings.Contains(tempfoldername, "\\") {
								// 	rootpath = filepath.Join(rootpath, tempfoldername)
								// }
							}
						}
						database.UpdateColumn("series", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{series.ID}})
					}

					logger.Log.Debug("Parsed and add: ", file, " as ", m.Title)

					counterif, _ := database.CountRows("serie_episode_files", database.Query{Where: "location = ? AND serie_episode_id = ?", WhereArgs: []interface{}{file, SeriesEpisode.ID}})
					if counterif == 0 {
						database.InsertArray("serie_episode_files",
							[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id"},
							[]interface{}{file, filepath.Base(file), filepath.Ext(file), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, SeriesEpisode.SerieID, SeriesEpisode.ID, SeriesEpisode.DbserieEpisodeID, SeriesEpisode.DbserieID})
					}
					if updatemissing {
						database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.ID}})
						database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.ID}})
						database.UpdateColumn("serie_episodes", "quality_profile", list.Template_quality, database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.ID}})
					}

					database.DeleteRow("serie_file_unmatcheds", database.Query{Where: "filepath = ?", WhereArgs: []interface{}{file}})

				} else {
					logger.Log.Debug("Already Parsed: ", file)
				}
			} else {
				mjson, _ := json.Marshal(m)
				valuesupsert := make(map[string]interface{})
				valuesupsert["listname"] = list.Name
				valuesupsert["filepath"] = file
				valuesupsert["last_checked"] = time.Now()
				valuesupsert["parsed_data"] = string(mjson)
				database.Upsert("serie_file_unmatcheds", valuesupsert, database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, list.Name}})

				logger.Log.Debug("SerieEpisode not matched loop: ", file, " as Title: ", m.Title, " TitleYear:  ", titleyear, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio, " Season ", m.Season, " Epi ", epi)
			}
		}
	} else {
		mjson, _ := json.Marshal(m)
		valuesupsert := make(map[string]interface{})
		valuesupsert["listname"] = list.Name
		valuesupsert["filepath"] = file
		valuesupsert["last_checked"] = time.Now()
		valuesupsert["parsed_data"] = string(mjson)
		database.Upsert("serie_file_unmatcheds", valuesupsert, database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, list.Name}})

		logger.Log.Debug("SerieEpisode not matched: ", file, " as Title: ", m.Title, " TitleYear:  ", titleyear, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
	}
}

var SeriesStructureJobRunning map[string]bool

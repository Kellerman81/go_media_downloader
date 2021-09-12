package utils

import (
	"encoding/json"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/remeh/sizedwaitgroup"
)

var SeriesImportJobRunning map[string]bool

func JobImportDbSeries(serieconfig config.SerieConfig, configEntry config.MediaTypeConfig, list config.MediaListsConfig, checkall bool, wg *sizedwaitgroup.SizedWaitGroup) {
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

	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	hasImdb, _ := config.ConfigDB.Has("imdb")
	if !hasImdb {
		logger.Log.Errorln("IMDB Config not found")
		return
	}
	var cfg_imdb config.ImdbConfig
	config.ConfigDB.Get("imdb", &cfg_imdb)

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
			addaliases := dbserie.GetMetadata("", cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
			if dbserie.Seriename == "" {
				addaliases = dbserie.GetMetadata(configEntry.Metadata_language, cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
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
			episode := dbserie.GetEpisodes(configEntry.Metadata_language, cfg_general.SerieMetaSourceTrakt)
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

func JobReloadDbSeries(dbserie database.Dbserie, configEntry config.MediaTypeConfig, list config.MediaListsConfig, checkall bool, wg *sizedwaitgroup.SizedWaitGroup) {
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

	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	hasImdb, _ := config.ConfigDB.Has("imdb")
	if !hasImdb {
		logger.Log.Errorln("IMDB Config not found")
		return
	}
	var cfg_imdb config.ImdbConfig
	config.ConfigDB.Get("imdb", &cfg_imdb)

	dbserie, _ = database.GetDbserie(database.Query{Where: "Thetvdb_id = ?", WhereArgs: []interface{}{dbserie.ThetvdbID}})
	logger.Log.Debug("DbSeries get metadata for: ", dbserie.ThetvdbID)
	addaliases := dbserie.GetMetadata("", cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
	if dbserie.Seriename == "" {
		addaliases = dbserie.GetMetadata(configEntry.Metadata_language, cfg_general.SerieMetaSourceTmdb, cfg_imdb.Indexedlanguages, cfg_general.SerieMetaSourceTrakt, false)
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
	episode := dbserie.GetEpisodes(configEntry.Metadata_language, cfg_general.SerieMetaSourceTrakt)
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
		foundserie, foundentries := findseriebyname(titleyear, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && seriestitle != "" {
		foundserie, foundentries := findseriebyname(seriestitle, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && m.Title != "" {
		foundserie, foundentries := findseriebyname(m.Title, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && titleyear != "" {
		foundserie, foundentries := findseriebyalternatename(titleyear, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && seriestitle != "" {
		foundserie, foundentries := findseriebyalternatename(seriestitle, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	if entriesfound == 0 && m.Title != "" {
		foundserie, foundentries := findseriebyalternatename(m.Title, listname)
		if foundentries == 1 {
			entriesfound = foundentries
			return foundserie, entriesfound
		}
	}
	return database.Serie{}, 0
}
func findseriebyname(title string, listname string) (database.Serie, int) {
	entriesfound := 0
	findseries, _ := database.QuerySeries(database.Query{Select: "Series.*", InnerJoin: "Dbseries ON Dbseries.ID = Series.Dbserie_id", Where: "DbSeries.Seriename = ? COLLATE NOCASE AND Series.listname = ?", WhereArgs: []interface{}{title, listname}})
	if len(findseries) == 0 {
		titleslug := logger.StringToSlug(title)
		findseries, _ = database.QuerySeries(database.Query{Select: "series.*", InnerJoin: "Dbseries ON Dbseries.ID = Series.Dbserie_id", Where: "DbSeries.Slug = ? COLLATE NOCASE AND Series.listname = ?", WhereArgs: []interface{}{titleslug, listname}})
	}

	if len(findseries) == 1 {
		entriesfound = len(findseries)
		return findseries[0], entriesfound
	}
	return database.Serie{}, 0
}
func findseriebyalternatename(title string, listname string) (database.Serie, int) {
	entriesfound := 0
	dbseries, _ := database.QueryDbserie(database.Query{Select: "dbseries.*", InnerJoin: "Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id", Where: "Dbserie_alternates.Title = ? COLLATE NOCASE", WhereArgs: []interface{}{title}})
	if len(dbseries) == 0 {
		titleslug := logger.StringToSlug(title)
		dbseries, _ = database.QueryDbserie(database.Query{Select: "dbseries.*", InnerJoin: "Dbserie_alternates on Dbserie_alternates.dbserie_id = dbseries.id", Where: "Dbserie_alternates.Slug = ? COLLATE NOCASE", WhereArgs: []interface{}{titleslug}})
	}
	if len(dbseries) >= 1 {
		findseries, _ := database.QuerySeries(database.Query{Where: "DbSerie_id = ? AND listname = ?", WhereArgs: []interface{}{dbseries[0].ID, listname}})

		if len(findseries) == 1 {
			entriesfound = len(findseries)
			return findseries[0], entriesfound
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
func JobImportSeriesParseV2(file string, updatemissing bool, configEntry config.MediaTypeConfig, list config.MediaListsConfig, minPrio ParseInfo, wg *sizedwaitgroup.SizedWaitGroup) {
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

	m, err := NewFileParser(filepath.Base(file), true, "series")
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
	addunmatched := false
	if entriesfound >= 1 {

		hasQuality, _ := config.ConfigDB.Has("quality_" + list.Template_quality)
		if !hasQuality {
			logger.Log.Errorln("Quality Config not found: ", "quality_"+list.Template_quality)
			return
		}
		var cfg_quality config.QualityConfig
		config.ConfigDB.Get("quality_"+list.Template_quality, &cfg_quality)

		cutoffPrio := NewCutoffPrio(configEntry, cfg_quality)

		m.GetPriority(configEntry, cfg_quality)
		errparsev := m.ParseVideoFile(file, configEntry, cfg_quality)
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

		for _, epi := range episodeArray {
			epi = strings.Trim(epi, "-EX")
			if strings.ToLower(testDbSeries.Identifiedby) != "date" {
				epi = strings.TrimLeft(epi, "0")
			}
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
							hasPath, _ := config.ConfigDB.Has("path_" + configEntry.Data[idxpath].Template_path)
							if !hasPath {
								logger.Log.Errorln("Path Config not found: ", "path_"+configEntry.Data[idxpath].Template_path)
								continue
							}
							var cfg_path config.PathsConfig
							config.ConfigDB.Get("path_"+configEntry.Data[idxpath].Template_path, &cfg_path)

							pppath := cfg_path.Path
							if strings.Contains(file, pppath) {
								rootpath = pppath
								tempfoldername := strings.Replace(file, pppath, "", -1)
								tempfoldername = strings.TrimLeft(tempfoldername, "/\\")
								tempfoldername = filepath.Dir(tempfoldername)
								_, firstfolder := getrootpath(tempfoldername)
								rootpath = filepath.Join(rootpath, firstfolder)
								break
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
				addunmatched = true
				logger.Log.Debug("SerieEpisode not matched loop: ", file, " as Title: ", m.Title, " TitleYear:  ", titleyear, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio, " Season ", m.Season, " Epi ", epi)
			}
		}
	} else {
		addunmatched = true
		logger.Log.Debug("SerieEpisode not matched: ", file, " as Title: ", m.Title, " TitleYear:  ", titleyear, " ", m.Resolution, " ", m.Quality, " ", m.Codec, " ", m.Audio)
	}
	if addunmatched {
		mjson, _ := json.Marshal(m)
		valuesupsert := make(map[string]interface{})
		valuesupsert["listname"] = list.Name
		valuesupsert["filepath"] = file
		valuesupsert["last_checked"] = time.Now()
		valuesupsert["parsed_data"] = string(mjson)
		database.Upsert("serie_file_unmatcheds", valuesupsert, database.Query{Where: "filepath = ? and listname = ?", WhereArgs: []interface{}{file, list.Name}})
	}
}

var SeriesStructureJobRunning map[string]bool

func RefreshSeries() {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	if cfg_general.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg_general.WorkerFiles)
	dbseries, _ := database.QueryDbserie(database.Query{})
	for idxserie := range dbseries {
		sw.Add()
		JobReloadDbSeries(dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func RefreshSeriesInc() {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	if cfg_general.SchedulerDisabled {
		return
	}
	sw := sizedwaitgroup.New(cfg_general.WorkerFiles)
	dbseries, _ := database.QueryDbserie(database.Query{Where: "status = 'Continuing'", OrderBy: "updated_at asc", Limit: 100})

	for idxserie := range dbseries {
		sw.Add()
		JobReloadDbSeries(dbseries[idxserie], config.MediaTypeConfig{}, config.MediaListsConfig{}, true, &sw)
	}
	sw.Wait()
}

func Series_all_jobs(job string, force bool) {
	serie_keys, _ := config.ConfigDB.Keys([]byte("serie_*"), 0, 0, true)

	logger.Log.Info("Started Job: ", job, " for all")
	for _, idxserie := range serie_keys {
		var cfg_serie config.MediaTypeConfig
		config.ConfigDB.Get(string(idxserie), &cfg_serie)

		Series_single_jobs(job, cfg_serie.Name, "", force)
	}
}

var SerieJobRunning map[string]bool

func Series_single_jobs(job string, typename string, listname string, force bool) {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	if cfg_general.SchedulerDisabled && !force {
		logger.Log.Info("Skipped Job: ", job, " for ", typename)
		return
	}
	jobName := job + typename + listname
	defer func() {
		database.ReadWriteMu.Lock()
		delete(SerieJobRunning, jobName)
		database.ReadWriteMu.Unlock()
	}()
	database.ReadWriteMu.Lock()
	if _, nok := SerieJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		SerieJobRunning[jobName] = true
	}
	database.ReadWriteMu.Unlock()
	logger.Log.Info("Started Job: ", job, " for ", typename)

	dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
		[]interface{}{job, typename, "Serie", time.Now()})
	ok, _ := config.ConfigDB.Has("serie_" + typename)
	if ok {
		var cfg_serie config.MediaTypeConfig
		config.ConfigDB.Get("serie_"+typename, &cfg_serie)

		switch job {
		case "searchmissingfull":
			SearchSerieMissing(cfg_serie, 0, false)
		case "searchmissinginc":
			SearchSerieMissing(cfg_serie, cfg_serie.Searchmissing_incremental, false)
		case "searchupgradefull":
			SearchSerieUpgrade(cfg_serie, 0, false)
		case "searchupgradeinc":
			SearchSerieUpgrade(cfg_serie, cfg_serie.Searchupgrade_incremental, false)
		case "searchmissingfulltitle":
			SearchSerieMissing(cfg_serie, 0, true)
		case "searchmissinginctitle":
			SearchSerieMissing(cfg_serie, cfg_serie.Searchmissing_incremental, true)
		case "searchupgradefulltitle":
			SearchSerieUpgrade(cfg_serie, 0, true)
		case "searchupgradeinctitle":
			SearchSerieUpgrade(cfg_serie, cfg_serie.Searchupgrade_incremental, true)

		}
		qualis := make(map[string]bool, 10)
		for idxlist := range cfg_serie.Lists {
			if cfg_serie.Lists[idxlist].Name != listname && listname != "" {
				continue
			}
			if _, ok := qualis[cfg_serie.Lists[idxlist].Template_quality]; !ok {
				qualis[cfg_serie.Lists[idxlist].Template_quality] = true
			}
			switch job {
			case "data":
				config.Slepping(true, 6)
				getnewepisodessingle(cfg_serie, cfg_serie.Lists[idxlist])
			case "checkmissing":
				checkmissingepisodessingle(cfg_serie, cfg_serie.Lists[idxlist])
			case "checkmissingflag":
				checkmissingepisodesflag(cfg_serie, cfg_serie.Lists[idxlist])
			case "structure":
				seriesStructureSingle(cfg_serie, cfg_serie.Lists[idxlist])
			case "clearhistory":
				database.DeleteRow("serie_episode_histories", database.Query{InnerJoin: "serie_episodes ON serie_episodes.id = serie_episode_histories.serie_episode_id inner join series on series.id = serie_episodes.serie_id", Where: "series.listname=?", WhereArgs: []interface{}{typename}})
			case "feeds":
				config.Slepping(true, 6)
				Importnewseriessingle(cfg_serie, cfg_serie.Lists[idxlist])
			default:
				// other stuff
			}
		}
		for qual := range qualis {
			switch job {
			case "rss":
				SearchSerieRSS(cfg_serie, qual)
			}
		}
	} else {
		logger.Log.Info("Skipped Job Type not matched: ", job, " for ", typename)
	}
	dbid, _ := dbresult.LastInsertId()
	database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})
	logger.Log.Info("Ended Job: ", job, " for ", typename)
	debug.FreeOSMemory()
}

var Lastseries string

func Importnewseriessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	results := Feeds(row, list)

	logger.Log.Info("Get Serie Config", list.Name)
	logger.Log.Info("Workers: ", cfg_general.WorkerMetadata)
	swg := sizedwaitgroup.New(cfg_general.WorkerMetadata)
	for idxserie := range results.Series.Serie {
		if strings.EqualFold(Lastseries, results.Series.Serie[idxserie].Name) && Lastseries != "" {
			config.Slepping(false, 5)
		}
		Lastseries = results.Series.Serie[idxserie].Name
		logger.Log.Info("Import Serie ", idxserie, " of ", len(results.Series.Serie), " name: ", results.Series.Serie[idxserie].Name)
		swg.Add()
		JobImportDbSeries(results.Series.Serie[idxserie], row, list, false, &swg)
	}
	swg.Wait()
}

var LastSeriesPath string
var LastSeriesFilePath string

func getnewepisodessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	hasQuality, _ := config.ConfigDB.Has("quality_" + list.Template_quality)
	if !hasQuality {
		logger.Log.Errorln("Quality Config not found: ", "quality_"+list.Template_quality)
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigDB.Get("quality_"+list.Template_quality, &cfg_quality)

	defaultPrio := &ParseInfo{Quality: row.DefaultQuality, Resolution: row.DefaultResolution}
	defaultPrio.GetPriority(row, cfg_quality)

	logger.Log.Info("Scan SerieEpisodeFile")
	filesfound := make([]string, 0, 5000)
	for idxpath := range row.Data {
		hasPath, _ := config.ConfigDB.Has("path_" + row.Data[idxpath].Template_path)
		if !hasPath {
			logger.Log.Errorln("Path Config not found: ", "path_"+row.Data[idxpath].Template_path)
			continue
		}
		var cfg_path config.PathsConfig
		config.ConfigDB.Get("path_"+row.Data[idxpath].Template_path, &cfg_path)

		if strings.EqualFold(LastSeriesPath, cfg_path.Path) && LastSeriesPath != "" {
			time.Sleep(time.Duration(5) * time.Second)
		}
		LastSeriesPath = cfg_path.Path
		filesfound_add := scanner.GetFilesGoDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
		filesfound = append(filesfound, filesfound_add...)
	}
	filesadded := scanner.GetFilesSeriesAdded(filesfound, list.Name)
	logger.Log.Info("Find SerieEpisodeFile")
	logger.Log.Info("Workers: ", cfg_general.WorkerParse)
	swf := sizedwaitgroup.New(cfg_general.WorkerParse)
	for idxfile := range filesadded {
		if strings.EqualFold(LastSeriesFilePath, filesadded[idxfile]) && LastSeriesFilePath != "" {
			time.Sleep(time.Duration(5) * time.Second)
		}
		LastSeriesFilePath = filesadded[idxfile]
		logger.Log.Info("Parse Serie ", idxfile, " of ", len(filesadded), " path: ", filesadded[idxfile])
		swf.Add()
		JobImportSeriesParseV2(filesadded[idxfile], true, row, list, *defaultPrio, &swf)
	}
	swf.Wait()
}

func checkmissingepisodesflag(row config.MediaTypeConfig, list config.MediaListsConfig) {
	episodes, _ := database.QuerySerieEpisodes(database.Query{Select: "serie_episodes.*", InnerJoin: " series on series.id = serie_episodes.serie_id", Where: "series.listname=?", WhereArgs: []interface{}{list.Name}})
	for idxepi := range episodes {
		counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episodes[idxepi].ID}})
		if counter >= 1 {
			if episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 0, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		} else {
			if !episodes[idxepi].Missing {
				database.UpdateColumn("Serie_episodes", "missing", 1, database.Query{Where: "id=?", WhereArgs: []interface{}{episodes[idxepi].ID}})
			}
		}
	}
}

func checkmissingepisodessingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	series, _ := database.QuerySeries(database.Query{Select: "id", Where: "listname=?", WhereArgs: []interface{}{list.Name}})

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)
	for idx := range series {
		seriefile, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "location", Where: "Serie_id=?", WhereArgs: []interface{}{series[idx].ID}})

		for idxfile := range seriefile {
			swfile.Add()
			JobImportFileCheck(seriefile[idxfile].Location, "serie", &swfile)
		}
	}
	swfile.Wait()
}

var LastSeriesStructure string

func seriesStructureSingle(row config.MediaTypeConfig, list config.MediaListsConfig) {
	hasGeneral, _ := config.ConfigDB.Has("general")
	if !hasGeneral {
		logger.Log.Errorln("General Config not found")
		return
	}
	var cfg_general config.GeneralConfig
	config.ConfigDB.Get("general", &cfg_general)

	swfile := sizedwaitgroup.New(cfg_general.WorkerFiles)

	for idxpath := range row.DataImport {
		mappathimport := row.DataImport[idxpath].Template_path
		hasPathimport, _ := config.ConfigDB.Has("path_" + mappathimport)
		if !hasPathimport {
			logger.Log.Errorln("Path Config not found: ", "path_"+mappathimport)
			continue
		}
		var cfg_path_import config.PathsConfig
		config.ConfigDB.Get("path_"+mappathimport, &cfg_path_import)

		hasPath, _ := config.ConfigDB.Has("path_" + row.Data[0].Template_path)
		if !hasPath {
			logger.Log.Errorln("Path Config not found: ", "path_"+row.Data[0].Template_path)
			continue
		}
		var cfg_path config.PathsConfig
		if len(row.Data) >= 1 {
			config.ConfigDB.Get("path_"+row.Data[0].Template_path, &cfg_path)
		}
		if strings.EqualFold(LastSeriesStructure, cfg_path_import.Path) && LastSeriesStructure != "" {
			time.Sleep(time.Duration(15) * time.Second)
		}
		LastSeriesStructure = cfg_path_import.Path
		swfile.Add()

		StructureFolders("series", cfg_path_import, cfg_path, row, list)
		swfile.Done()
	}
	swfile.Wait()
}

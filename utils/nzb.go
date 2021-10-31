package utils

import (
	"io"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/newznab"
)

type nzbwithprio struct {
	Prio       int
	Indexer    string
	ParseInfo  ParseInfo
	NZB        newznab.NZB
	Nzbmovie   database.Movie
	Nzbepisode database.SerieEpisode
}

func filter_size_nzbs(configEntry config.MediaTypeConfig, indexer config.QualityIndexerConfig, rownzb newznab.NZB) bool {
	for idx := range configEntry.DataImport {

		if indexer.Skip_empty_size && rownzb.Size == 0 {
			logger.Log.Debug("Skipped - Size missing: ", rownzb.Title)
			return true
		}
		if !config.ConfigCheck("path_" + configEntry.DataImport[idx].Template_path) {
			return false
		}
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+configEntry.DataImport[idx].Template_path, &cfg_path)

		if cfg_path.MinSize != 0 {
			if rownzb.Size < int64(cfg_path.MinSize*1024*1024) && rownzb.Size != 0 {
				logger.Log.Debug("Skipped - MinSize not matched: ", rownzb.Title)
				return true
			}
		}

		if cfg_path.MaxSize != 0 {
			if rownzb.Size > int64(cfg_path.MaxSize*1024*1024) {
				logger.Log.Debug("Skipped - MaxSize not matched: ", rownzb.Title)
				return true
			}
		}
	}
	return false
}
func filter_regex_nzbs(regexconfig config.RegexConfig, title string, wantedtitle string, wantedalternates []string) bool {
	for _, rowtitle := range regexconfig.Rejected {
		//rowrejected := regexp.MustCompile(rowtitle)
		rowrejected := regexconfig.RejectedRegex[rowtitle]
		teststrwanted := rowrejected.FindStringSubmatch(wantedtitle)
		if len(teststrwanted) >= 1 {
			continue
		}
		breakfor := false
		for idx := range wantedalternates {
			teststrwanted := rowrejected.FindStringSubmatch(wantedalternates[idx])
			if len(teststrwanted) >= 1 {
				breakfor = true
				break
			}
		}
		if breakfor {
			break
		}
		teststr := rowrejected.FindStringSubmatch(title)
		if len(teststr) >= 1 {
			logger.Log.Debug("Skipped - Regex rejected: ", title, " Regex ", rowtitle)
			return true
		}
	}
	requiredmatched := false
	for _, rowtitle := range regexconfig.Required {
		//rowrequired := regexp.MustCompile(rowtitle)
		rowrequired := regexconfig.RequiredRegex[rowtitle]

		teststr := rowrequired.FindStringSubmatch(title)
		if len(teststr) >= 1 {
			logger.Log.Debug("Regex required matched: ", title, " Regex ", rowtitle)
			requiredmatched = true
			break
		}
	}
	if len(regexconfig.Required) >= 1 && !requiredmatched {
		logger.Log.Debug("Skipped - required not matched: ", title)
		return true
	}
	return false
}
func filter_movies_nzbs(configEntry config.MediaTypeConfig, quality config.QualityConfig, indexer config.QualityIndexerConfig, nzbs []newznab.NZB, movieid uint, seriesepisodeid uint, minPrio int, movie database.Dbmovie, serie database.Dbserie, title string, alttitles []string, year string) []nzbwithprio {
	retnzb := make([]nzbwithprio, 0, len(nzbs))
	for idx := range nzbs {
		toskip := false
		if len(strings.Trim(Path(nzbs[idx].Title, false), " ")) <= 3 {
			logger.Log.Debug("Skipped - Title too short: ", nzbs[idx].Title)
			toskip = true
			continue
		}
		if filter_size_nzbs(configEntry, indexer, nzbs[idx]) {
			toskip = true
			continue
		}
		if movieid >= 1 {
			counterh1, _ := database.CountRows("movie_histories", database.Query{Where: "movie_id = ? and url = ?", WhereArgs: []interface{}{movieid, nzbs[idx].DownloadURL}})
			if counterh1 >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if indexer.History_check_title {
				counterh2, _ := database.CountRows("movie_histories", database.Query{Where: "movie_id = ? and title = ?", WhereArgs: []interface{}{movieid, nzbs[idx].Title}})
				if counterh2 >= 1 {
					logger.Log.Debug("Skipped - Already Downloaded (Title): ", nzbs[idx].Title)
					toskip = true
					continue
				}
			}
			tempimdb := nzbs[idx].IMDBID
			tempimdb = strings.TrimPrefix(tempimdb, "tt")
			tempimdb = strings.TrimPrefix(tempimdb, "0")
			tempimdb = strings.TrimPrefix(tempimdb, "0")
			tempimdb = strings.TrimPrefix(tempimdb, "0")
			tempimdb = strings.TrimPrefix(tempimdb, "0")

			wantedimdb := movie.ImdbID
			wantedimdb = strings.TrimPrefix(wantedimdb, "tt")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			wantedimdb = strings.TrimPrefix(wantedimdb, "0")
			if wantedimdb != tempimdb && len(wantedimdb) >= 1 && len(tempimdb) >= 1 {
				logger.Log.Debug("Skipped - Imdb not match: ", nzbs[idx].Title, " - imdb in nzb: ", tempimdb, " imdb wanted: ", wantedimdb)
				toskip = true
				continue
			}
		} else {
			counterh1, _ := database.CountRows("movie_histories", database.Query{Where: "url = ?", WhereArgs: []interface{}{nzbs[idx].DownloadURL}})
			if counterh1 >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if indexer.History_check_title {
				counterh2, _ := database.CountRows("movie_histories", database.Query{Where: "title = ?", WhereArgs: []interface{}{nzbs[idx].Title}})
				if counterh2 >= 1 {
					logger.Log.Debug("Skipped - Already Downloaded (Title): ", nzbs[idx].Title)
					toskip = true
					continue
				}
			}
		}
		alternatenames := []string{}
		foundalternate, _ := database.QueryDbmovieTitle(database.Query{Select: "id", Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.ID}})
		for idxalt := range foundalternate {
			alternatenames = append(alternatenames, foundalternate[idxalt].Title)
		}
		if !config.ConfigCheck("regex_" + indexer.Template_regex) {
			toskip = true
			continue
		}
		var cfg_regex config.RegexConfig
		config.ConfigGet("regex_"+indexer.Template_regex, &cfg_regex)

		if filter_regex_nzbs(cfg_regex, nzbs[idx].Title, movie.Title, alternatenames) {
			toskip = true
			continue
		}
		if !toskip {
			m, _ := NewFileParser(nzbs[idx].Title, false, "movie")
			for idxstrip := range quality.TitleStripSuffixForSearch {
				if strings.HasSuffix(strings.ToLower(m.Title), strings.ToLower(quality.TitleStripSuffixForSearch[idxstrip])) {
					m.Title = trimStringInclAfterStringInsensitive(m.Title, quality.TitleStripSuffixForSearch[idxstrip])
					m.Title = strings.Trim(m.Title, " ")
				}
			}
			if quality.CheckYear && !quality.CheckYear1 && !strings.Contains(nzbs[idx].Title, year) && len(year) >= 1 && year != "0" {
				logger.Log.Debug("Skipped - unwanted year: ", nzbs[idx].Title, " wanted ", year)
				continue
			} else {
				if quality.CheckYear1 && len(year) >= 1 && year != "0" {
					yearint, _ := strconv.Atoi(year)
					if !strings.Contains(nzbs[idx].Title, strconv.Itoa(yearint+1)) && !strings.Contains(nzbs[idx].Title, strconv.Itoa(yearint-1)) && !strings.Contains(nzbs[idx].Title, strconv.Itoa(yearint)) {
						logger.Log.Debug("Skipped - unwanted year: ", nzbs[idx].Title, " wanted (+-1) ", yearint)
						continue
					}
				}
			}
			if quality.CheckTitle {
				titlefound := false
				if quality.CheckTitle && checknzbtitle(title, m.Title) && len(title) >= 1 {
					titlefound = true
				}
				alttitlefound := false
				for idxtitle := range alttitles {
					if checknzbtitle(alttitles[idxtitle], m.Title) {
						alttitlefound = true
						break
					}
				}
				if len(alttitles) == 0 && !titlefound {
					logger.Log.Debug("Skipped - unwanted title: ", nzbs[idx].Title, " wanted ", title)
					continue
				}
				if len(alttitles) >= 1 && !alttitlefound && !titlefound {
					logger.Log.Debug("Skipped - unwanted title and alternate: ", nzbs[idx].Title, " wanted ", title, " ", alttitles)
					continue
				}
			}
			m.GetPriority(configEntry, quality)
			wanted_release_resolution := false
			for idxqual := range quality.Wanted_resolution {
				if strings.EqualFold(quality.Wanted_resolution[idxqual], m.Resolution) {
					wanted_release_resolution = true
					break
				}
			}
			wanted_release_quality := false
			for idxqual := range quality.Wanted_quality {
				if !strings.EqualFold(quality.Wanted_quality[idxqual], m.Quality) {
					wanted_release_quality = true
					break
				}
			}
			if len(quality.Wanted_resolution) >= 1 && !wanted_release_resolution {
				logger.Log.Debug("Skipped - unwanted resolution: ", nzbs[idx].Title)
				continue
			}
			if len(quality.Wanted_quality) >= 1 && !wanted_release_quality {
				logger.Log.Debug("Skipped - unwanted quality: ", nzbs[idx].Title)
				continue
			}
			if m.Priority != 0 {
				if minPrio != 0 {
					if m.Priority <= minPrio {
						logger.Log.Debug("Skipped - Prio lower: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
						continue
					}
					logger.Log.Debug("ok - prio higher: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
				}
				setmovie, _ := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{movieid}})
				nzbprio := nzbwithprio{m.Priority, indexer.Template_indexer, *m, nzbs[idx], setmovie, database.SerieEpisode{}}
				retnzb = append(retnzb, nzbprio)
			} else {
				logger.Log.Debug("Skipped - Prio not matched: ", nzbs[idx].Title)
			}
		}
	}
	return retnzb
}

func checknzbtitle(movietitle string, nzbtitle string) bool {
	logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	if strings.EqualFold(movietitle, nzbtitle) {
		return true
	}
	movietitle = logger.StringToSlug(movietitle)
	nzbtitle = logger.StringToSlug(nzbtitle)
	logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	return strings.EqualFold(movietitle, nzbtitle)
}

func filter_series_nzbs(configEntry config.MediaTypeConfig, quality config.QualityConfig, indexer config.QualityIndexerConfig, nzbs []newznab.NZB, movieid uint, seriesepisodeid uint, minPrio int, movie database.Dbmovie, serie database.Dbserie) []nzbwithprio {
	retnzb := make([]nzbwithprio, 0, len(nzbs))
	for idx := range nzbs {
		toskip := false
		if len(strings.Trim(Path(nzbs[idx].Title, false), " ")) <= 3 {
			logger.Log.Debug("Skipped - Title too short: ", nzbs[idx].Title)
			toskip = true
			continue
		}
		if filter_size_nzbs(configEntry, indexer, nzbs[idx]) {
			toskip = true
			continue
		}
		if toskip {
			continue
		}
		if seriesepisodeid >= 1 {
			counterh1, _ := database.CountRows("serie_episode_histories", database.Query{Where: "serie_episode_id = ? and url = ?", WhereArgs: []interface{}{seriesepisodeid, nzbs[idx].DownloadURL}})
			if counterh1 >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if indexer.History_check_title {
				counterh2, _ := database.CountRows("serie_episode_histories", database.Query{Where: "serie_episode_id = ? and title = ?", WhereArgs: []interface{}{seriesepisodeid, nzbs[idx].Title}})
				if counterh2 >= 1 {
					logger.Log.Debug("Skipped - Already Downloaded (Title): ", nzbs[idx].Title)
					toskip = true
					continue
				}
			}
			if strconv.Itoa(serie.ThetvdbID) != nzbs[idx].TVDBID && serie.ThetvdbID >= 1 && len(nzbs[idx].TVDBID) >= 1 {
				logger.Log.Debug("Skipped - Tvdb not match: ", nzbs[idx].Title, " - Tvdb in nzb: ", nzbs[idx].TVDBID, " Tvdb wanted: ", serie.ThetvdbID)
				toskip = true
				continue
			}
			if quality.CheckTitle || (serie.ThetvdbID == 0 && quality.BackupSearchForTitle) {
				toskip = true
				if serie.Seriename != "" {
					if !strings.HasPrefix(logger.StringToSlug(nzbs[idx].Title), logger.StringToSlug(serie.Seriename)) {
						foundalternate, _ := database.QueryDbserieAlternates(database.Query{Select: "dbserie_alternates.title", InnerJoin: "dbseries on dbseries.id = dbserie_alternates.dbserie_id", Where: "dbseries.seriename=?", WhereArgs: []interface{}{serie.Seriename}})
						for idxalt := range foundalternate {
							if strings.HasPrefix(logger.StringToSlug(nzbs[idx].Title), logger.StringToSlug(foundalternate[idxalt].Title)) {
								toskip = false
								break
							}
						}
					} else {
						toskip = false
					}
					if !toskip {
						foundepi, foundepierr := database.GetSerieEpisodes(database.Query{Select: "dbserie_episode_id", Where: "id = ?", WhereArgs: []interface{}{seriesepisodeid}})
						if foundepierr == nil {
							founddbepi, founddbepierr := database.GetDbserieEpisodes(database.Query{Select: "identifier", Where: "id = ?", WhereArgs: []interface{}{foundepi.DbserieEpisodeID}})
							if founddbepierr == nil {
								alt_identifier := strings.TrimPrefix(founddbepi.Identifier, "S")
								alt_identifier = strings.TrimPrefix(alt_identifier, "0")
								alt_identifier = strings.Replace(alt_identifier, "E", "x", -1)
								if strings.Contains(strings.ToLower(nzbs[idx].Title), strings.ToLower(founddbepi.Identifier)) ||
									strings.Contains(strings.ToLower(nzbs[idx].Title), strings.ToLower(strings.Replace(founddbepi.Identifier, "-", ".", -1))) ||
									strings.Contains(strings.ToLower(nzbs[idx].Title), strings.ToLower(strings.Replace(founddbepi.Identifier, "-", " ", -1))) ||
									strings.Contains(strings.ToLower(nzbs[idx].Title), strings.ToLower(alt_identifier)) ||
									strings.Contains(strings.ToLower(nzbs[idx].Title), strings.ToLower(strings.Replace(alt_identifier, "-", ".", -1))) ||
									strings.Contains(strings.ToLower(nzbs[idx].Title), strings.ToLower(strings.Replace(alt_identifier, "-", " ", -1))) {
									toskip = false
								} else {
									toskip = true
									logger.Log.Debug("Skipped - seriename provided dbepi found but identifier not match ", founddbepi.Identifier)
									continue
								}
							} else {
								toskip = true
								logger.Log.Debug("Skipped - seriename provided dbepi not found", serie.Seriename)
								continue
							}
						} else {
							toskip = true
							logger.Log.Debug("Skipped - seriename provided epi not found", serie.Seriename)
							continue
						}
					} else {
						logger.Log.Debug("Skipped - seriename provided but not found ", serie.Seriename)
					}
				} else {
					logger.Log.Debug("Skipped - seriename not provided or searchfortitle disabled")
				}
				if toskip {
					logger.Log.Debug("Skipped - wrong seriename - wanted: ", serie.Seriename, " have: ", nzbs[idx].Title)
				}
			}
		} else {
			counterh1, _ := database.CountRows("serie_episode_histories", database.Query{Where: "url = ?", WhereArgs: []interface{}{nzbs[idx].DownloadURL}})
			if counterh1 >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if indexer.History_check_title {
				counterh2, _ := database.CountRows("serie_episode_histories", database.Query{Where: "title = ?", WhereArgs: []interface{}{nzbs[idx].Title}})
				if counterh2 >= 1 {
					logger.Log.Debug("Skipped - Already Downloaded (Title): ", nzbs[idx].Title)
					toskip = true
					continue
				}
			}
		}
		alternatenames := []string{}
		foundalternate, _ := database.QueryDbserieAlternates(database.Query{Select: "dbserie_alternates.title", InnerJoin: "dbseries on dbseries.id = dbserie_alternates.dbserie_id", Where: "dbseries.seriename=?", WhereArgs: []interface{}{serie.Seriename}})
		for idxalt := range foundalternate {
			alternatenames = append(alternatenames, foundalternate[idxalt].Title)
		}
		if !config.ConfigCheck("regex_" + indexer.Template_regex) {
			toskip = true
			continue
		}
		var cfg_regex config.RegexConfig
		config.ConfigGet("regex_"+indexer.Template_regex, &cfg_regex)

		if filter_regex_nzbs(cfg_regex, nzbs[idx].Title, serie.Seriename, alternatenames) {
			toskip = true
			continue
		}
		if !toskip {
			m, _ := NewFileParser(nzbs[idx].Title, true, "series")
			m.GetPriority(configEntry, quality)
			wanted_release_resolution := false
			for idxqual := range quality.Wanted_resolution {
				if strings.EqualFold(quality.Wanted_resolution[idxqual], m.Resolution) {
					wanted_release_resolution = true
					break
				}
			}
			wanted_release_quality := false
			for idxqual := range quality.Wanted_quality {
				if !strings.EqualFold(quality.Wanted_quality[idxqual], m.Quality) {
					wanted_release_quality = true
					break
				}
			}
			if len(quality.Wanted_resolution) >= 1 && !wanted_release_resolution {
				logger.Log.Debug("Skipped - unwanted resolution: ", nzbs[idx].Title)
				continue
			}
			if len(quality.Wanted_quality) >= 1 && !wanted_release_quality {
				logger.Log.Debug("Skipped - unwanted quality: ", nzbs[idx].Title)
				continue
			}
			if m.Priority != 0 {
				if minPrio != 0 {
					if m.Priority <= minPrio {
						logger.Log.Debug("Skipped - Prio lower: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
						continue
					}
					logger.Log.Debug("ok - prio higher: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
				}

				setserieepisode, _ := database.GetSerieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{seriesepisodeid}})
				nzbprio := nzbwithprio{m.Priority, indexer.Template_indexer, *m, nzbs[idx], database.Movie{}, setserieepisode}
				retnzb = append(retnzb, nzbprio)
			} else {
				logger.Log.Debug("Skipped - Prio not matched: ", nzbs[idx].Title)
			}
		}
	}
	return retnzb
}

func filter_series_rss_nzbs(configEntry config.MediaTypeConfig, quality config.QualityConfig, lists []string, indexer config.QualityIndexerConfig, nzbs []newznab.NZB) []nzbwithprio {
	retnzb := make([]nzbwithprio, 0, len(nzbs))
	for idx := range nzbs {
		toskip := false
		if len(strings.Trim(Path(nzbs[idx].Title, false), " ")) <= 3 {
			logger.Log.Debug("Skipped - Title too short: ", nzbs[idx].Title)
			toskip = true
			continue
		}
		if toskip {
			continue
		}
		if filter_size_nzbs(configEntry, indexer, nzbs[idx]) {
			toskip = true
			continue
		}
		if toskip {
			continue
		}
		minPrio := 0
		counter, _ := database.CountRows("serie_episode_histories", database.Query{Where: "url = ?", WhereArgs: []interface{}{nzbs[idx].DownloadURL}})
		if counter >= 1 {
			logger.Log.Debug("Skipped - Already Downloaded: ", nzbs[idx].Title)
			toskip = true
			continue
		}
		if indexer.History_check_title {
			counter, _ = database.CountRows("serie_episode_histories", database.Query{Where: "title = ?", WhereArgs: []interface{}{nzbs[idx].Title}})
			if counter >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded (Title): ", nzbs[idx].Title)
				toskip = true
				continue
			}
		}
		regextitle := ""
		regexalternate := []string{}
		var foundepisode database.SerieEpisode
		if len(nzbs[idx].TVDBID) >= 1 {
			founddbserie, founddbserieerr := database.GetDbserie(database.Query{Select: "id, identifiedby, seriename", Where: "thetvdb_id = ?", WhereArgs: []interface{}{nzbs[idx].TVDBID}})

			if founddbserieerr != nil {
				logger.Log.Debug("Skipped - Not Wanted DB Serie: ", nzbs[idx].Title)
				toskip = true
				continue
			}

			regextitle = founddbserie.Seriename
			foundalternate, _ := database.QueryDbserieAlternates(database.Query{Select: "title", Where: "dbserie_id=?", WhereArgs: []interface{}{founddbserie.ID}})
			for idxalt := range foundalternate {
				regexalternate = append(regexalternate, foundalternate[idxalt].Title)
			}
			args := []interface{}{}
			args = append(args, founddbserie.ID)
			for idx := range lists {
				args = append(args, lists[idx])
			}
			foundserie, foundserieerr := database.GetSeries(database.Query{Select: "id", Where: "dbserie_id = ? and listname IN (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: args})
			if foundserieerr != nil {
				logger.Log.Debug("Skipped - Not Wanted Serie: ", nzbs[idx].Title)
				toskip = true
				continue
			}

			var founddbepisode database.DbserieEpisode
			var founddbepisodeerr error
			if strings.EqualFold(founddbserie.Identifiedby, "date") {
				tempparse, _ := NewFileParser(nzbs[idx].Title, true, "series")
				if tempparse.Date == "" {
					logger.Log.Debug("Skipped - Date wanted but not found: ", nzbs[idx].Title)
					toskip = true
					continue
				}
				tempparse.Date = strings.Replace(tempparse.Date, ".", "-", -1)
				tempparse.Date = strings.Replace(tempparse.Date, " ", "-", -1)
				founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Identifier = ?", WhereArgs: []interface{}{founddbserie.ID, tempparse.Date}})
				if founddbepisodeerr != nil {
					logger.Log.Debug("Skipped - Not Wanted DB Episode: ", nzbs[idx].Title)
					toskip = true
					continue
				}
			} else {
				founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{founddbserie.ID, nzbs[idx].Season, nzbs[idx].Episode}})
				if founddbepisodeerr != nil {
					logger.Log.Debug("Skipped - Not Wanted DB Episode: ", nzbs[idx].Title)
					toskip = true
					continue
				}
			}
			var foundepisodeerr error
			foundepisode, foundepisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{founddbepisode.ID, foundserie.ID}})
			if foundepisodeerr != nil {
				logger.Log.Debug("Skipped - Not Wanted Episode: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if foundepisode.DontSearch || foundepisode.DontUpgrade || (!foundepisode.Missing && foundepisode.QualityReached) {
				logger.Log.Debug("Skipped - Notwanted or Already reached: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if foundepisode.QualityProfile != quality.Name {
				logger.Log.Debug("Skipped - wrong quality set: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			minPrio = getHighestEpisodePriorityByFiles(foundepisode, configEntry, quality)

			// foundfiles, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "filename", Where: "serie_episode_id = ?", WhereArgs: []interface{}{foundepisode.ID}})
			// for idxfile := range foundfiles {
			// 	m, _ := NewFileParser(cfg, foundfiles[idxfile].Filename, true, "series")
			// 	if m.Priority == 0 {
			// 		minPrio = m.Priority
			// 	} else {
			// 		if minPrio < m.Priority {
			// 			minPrio = m.Priority
			// 		}
			// 	}
			// }
		} else {
			if quality.BackupSearchForTitle {
				tempparse, _ := NewFileParser(nzbs[idx].Title, true, "series")
				founddbserie, founddbserieerr := database.GetDbserie(database.Query{Select: "id, identifiedby, seriename", Where: "seriename = ?", WhereArgs: []interface{}{tempparse.Title}})
				if founddbserieerr != nil {
					founddbserie_alt, founddbserie_alterr := database.GetDbserieAlternates(database.Query{Select: "dbserie_id", Where: "Title = ?", WhereArgs: []interface{}{tempparse.Title}})
					if founddbserie_alterr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Serie: ", nzbs[idx].Title)
						toskip = true
						continue
					} else {
						founddbserie, _ = database.GetDbserie(database.Query{Select: "id, identifiedby, seriename", Where: "id = ?", WhereArgs: []interface{}{founddbserie_alt.DbserieID}})
					}
				}
				regextitle = founddbserie.Seriename
				foundalternate, _ := database.QueryDbserieAlternates(database.Query{Select: "title", Where: "dbserie_id=?", WhereArgs: []interface{}{founddbserie.ID}})
				for idxalt := range foundalternate {
					regexalternate = append(regexalternate, foundalternate[idxalt].Title)
				}
				args := []interface{}{}
				args = append(args, founddbserie.ID)
				for idx := range lists {
					args = append(args, lists[idx])
				}
				foundserie, foundserieerr := database.GetSeries(database.Query{Select: "id", Where: "dbserie_id = ? and listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: args})
				if foundserieerr != nil {
					logger.Log.Debug("Skipped - Not Wanted Serie: ", nzbs[idx].Title)
					toskip = true
					continue
				}
				var founddbepisode database.DbserieEpisode
				var founddbepisodeerr error
				if strings.EqualFold(founddbserie.Identifiedby, "date") {
					if tempparse.Date == "" {
						logger.Log.Debug("Skipped - Date wanted but not found: ", nzbs[idx].Title)
						toskip = true
						continue
					}
					tempparse.Date = strings.Replace(tempparse.Date, ".", "-", -1)
					tempparse.Date = strings.Replace(tempparse.Date, " ", "-", -1)
					founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Identifier = ?", WhereArgs: []interface{}{founddbserie.ID, tempparse.Date}})

					if founddbepisodeerr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Episode: ", nzbs[idx].Title)
						toskip = true
						continue
					}
				} else {
					founddbepisode, founddbepisodeerr = database.GetDbserieEpisodes(database.Query{Select: "id", Where: "dbserie_id = ? and Season = ? and Episode = ?", WhereArgs: []interface{}{founddbserie.ID, nzbs[idx].Season, nzbs[idx].Episode}})
					if founddbepisodeerr != nil {
						logger.Log.Debug("Skipped - Not Wanted DB Episode: ", nzbs[idx].Title)
						toskip = true
						continue
					}
				}
				var foundepisodeerr error
				foundepisode, foundepisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{founddbepisode.ID, foundserie.ID}})
				if foundepisodeerr != nil {
					logger.Log.Debug("Skipped - Not Wanted Episode: ", nzbs[idx].Title)
					toskip = true
					continue
				}
				minPrio = getHighestEpisodePriorityByFiles(foundepisode, configEntry, quality)

				// foundfiles, _ := database.QuerySerieEpisodeFiles(database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{foundepisode.ID}})
				// for idxfile := range foundfiles {
				// 	m, _ := NewFileParser(cfg, foundfiles[idxfile].Filename, true, "series")
				// 	if m.Priority == 0 {
				// 		minPrio = m.Priority
				// 	} else {
				// 		if minPrio < m.Priority {
				// 			minPrio = m.Priority
				// 		}
				// 	}
				// }
			} else {
				logger.Log.Debug("Skipped - no tvbdid: ", nzbs[idx].Title)
				continue
			}
		}
		if !config.ConfigCheck("regex_" + indexer.Template_regex) {
			toskip = true
			continue
		}
		var cfg_regex config.RegexConfig
		config.ConfigGet("regex_"+indexer.Template_regex, &cfg_regex)

		if filter_regex_nzbs(cfg_regex, nzbs[idx].Title, regextitle, regexalternate) {
			toskip = true
			continue
		}
		if !toskip {
			m, _ := NewFileParser(nzbs[idx].Title, true, "series")
			m.GetPriority(configEntry, quality)
			wanted_release_resolution := false
			for idxqual := range quality.Wanted_resolution {
				if strings.EqualFold(quality.Wanted_resolution[idxqual], m.Resolution) {
					wanted_release_resolution = true
					break
				}
			}
			wanted_release_quality := false
			for idxqual := range quality.Wanted_quality {
				if !strings.EqualFold(quality.Wanted_quality[idxqual], m.Quality) {
					wanted_release_quality = true
					break
				}
			}
			if len(quality.Wanted_resolution) >= 1 && !wanted_release_resolution {
				logger.Log.Debug("Skipped - unwanted resolution: ", nzbs[idx].Title)
				continue
			}
			if len(quality.Wanted_quality) >= 1 && !wanted_release_quality {
				logger.Log.Debug("Skipped - unwanted quality: ", nzbs[idx].Title)
				continue
			}
			if m.Priority != 0 {
				if minPrio != 0 {
					if m.Priority <= minPrio {
						logger.Log.Debug("Skipped - Prio lower: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
						continue
					}
					logger.Log.Debug("ok - prio higher: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)

				}
				retnzb = append(retnzb, nzbwithprio{m.Priority, indexer.Template_indexer, *m, nzbs[idx], database.Movie{}, foundepisode})
			} else {
				logger.Log.Debug("Skipped - Prio not matched: ", nzbs[idx].Title)
			}
		}
	}
	return retnzb
}

func filter_movies_rss_nzbs(configEntry config.MediaTypeConfig, quality config.QualityConfig, lists []string, indexer config.QualityIndexerConfig, nzbs []newznab.NZB) []nzbwithprio {
	retnzb := make([]nzbwithprio, 0, len(nzbs))
	for idx := range nzbs {
		toskip := false
		if len(strings.Trim(Path(nzbs[idx].Title, false), " ")) <= 3 {
			logger.Log.Debug("Skipped - Title too short: ", nzbs[idx].Title)
			toskip = true
			continue
		}
		if toskip {
			continue
		}
		if filter_size_nzbs(configEntry, indexer, nzbs[idx]) {
			toskip = true
			continue
		}
		if toskip {
			continue
		}
		minPrio := 0

		counter, _ := database.CountRows("movie_histories", database.Query{Where: "url = ?", WhereArgs: []interface{}{nzbs[idx].DownloadURL}})
		if counter >= 1 {
			logger.Log.Debug("Skipped - Already Downloaded: ", nzbs[idx].Title)
			toskip = true
			continue
		}
		if indexer.History_check_title {
			counter, _ = database.CountRows("movie_histories", database.Query{Where: "title = ?", WhereArgs: []interface{}{nzbs[idx].Title}})
			if counter >= 1 {
				logger.Log.Debug("Skipped - Already Downloaded (Title): ", nzbs[idx].Title)
				toskip = true
				continue
			}
		}
		var foundmovie database.Movie
		if len(nzbs[idx].IMDBID) == 0 && quality.BackupSearchForTitle {
			m, _ := NewFileParser(nzbs[idx].Title, false, "movie")
			for idxstrip := range quality.TitleStripSuffixForSearch {
				if strings.HasSuffix(strings.ToLower(m.Title), strings.ToLower(quality.TitleStripSuffixForSearch[idxstrip])) {
					m.Title = trimStringInclAfterStringInsensitive(m.Title, quality.TitleStripSuffixForSearch[idxstrip])
					m.Title = strings.Trim(m.Title, " ")
				}
			}
			_, imdb, entriesfound := movieFindListByTitle(m.Title, strconv.Itoa(m.Year), lists, quality.CheckYear1, "rss")
			if entriesfound >= 1 {
				nzbs[idx].IMDBID = imdb
			}
		}
		regextitle := ""
		regexalternate := []string{}
		if len(nzbs[idx].IMDBID) >= 1 {
			var founddbmovie database.Dbmovie
			var founddbmovieerr error
			searchimdb := nzbs[idx].IMDBID
			if !strings.HasPrefix(searchimdb, "tt") {
				searchimdb = "tt" + nzbs[idx].IMDBID
			}
			founddbmovie, founddbmovieerr = database.GetDbmovie(database.Query{Select: "id, title", Where: "imdb_id = ?", WhereArgs: []interface{}{searchimdb}})

			if founddbmovieerr != nil {
				logger.Log.Debug("Skipped - Not Wanted DB Movie: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			regextitle = founddbmovie.Title

			foundalternate, _ := database.QueryDbmovieTitle(database.Query{Select: "id", Where: "dbmovie_id=?", WhereArgs: []interface{}{founddbmovie.ID}})
			for idxalt := range foundalternate {
				regexalternate = append(regexalternate, foundalternate[idxalt].Title)
			}
			args := []interface{}{}
			args = append(args, founddbmovie.ID)
			for idx := range lists {
				args = append(args, lists[idx])
			}
			var foundmovieerr error
			foundmovie, foundmovieerr = database.GetMovies(database.Query{Where: "dbmovie_id = ? and listname in (?" + strings.Repeat(",?", len(lists)-1) + ")", WhereArgs: args})
			if foundmovieerr != nil {
				logger.Log.Debug("Skipped - Not Wanted Movie: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if foundmovie.DontSearch || foundmovie.DontUpgrade || (!foundmovie.Missing && foundmovie.QualityReached) {
				logger.Log.Debug("Skipped - Notwanted or Already reached: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			if foundmovie.QualityProfile != quality.Name {
				logger.Log.Debug("Skipped - wrong quality set: ", nzbs[idx].Title)
				toskip = true
				continue
			}
			minPrio = getHighestMoviePriorityByFiles(foundmovie, configEntry, quality)
			// foundfiles, _ := database.QueryMovieFiles(database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{foundmovie.ID}})
			// for idxfile := range foundfiles {
			// 	m, _ := NewFileParser(cfg, foundfiles[idxfile].Filename, false, "movie")
			// 	m.GetPriority(configEntry, list)
			// 	if minPrio == 0 {
			// 		minPrio = m.Priority
			// 	} else {
			// 		if minPrio < m.Priority {
			// 			minPrio = m.Priority
			// 		}
			// 	}
			// }
		} else {
			logger.Log.Debug("Skipped - no imdbid: ", nzbs[idx].Title)
			continue
		}
		if !config.ConfigCheck("regex_" + indexer.Template_regex) {
			toskip = true
			continue
		}
		var cfg_regex config.RegexConfig
		config.ConfigGet("regex_"+indexer.Template_regex, &cfg_regex)

		if filter_regex_nzbs(cfg_regex, nzbs[idx].Title, regextitle, regexalternate) {
			toskip = true
			continue
		}
		if !toskip {
			m, _ := NewFileParser(nzbs[idx].Title, false, "movie")
			m.GetPriority(configEntry, quality)
			wanted_release_resolution := false
			for idxqual := range quality.Wanted_resolution {
				if strings.EqualFold(quality.Wanted_resolution[idxqual], m.Resolution) {
					wanted_release_resolution = true
					break
				}
			}
			wanted_release_quality := false
			for idxqual := range quality.Wanted_quality {
				if !strings.EqualFold(quality.Wanted_quality[idxqual], m.Quality) {
					wanted_release_quality = true
					break
				}
			}
			if len(quality.Wanted_resolution) >= 1 && !wanted_release_resolution {
				logger.Log.Debug("Skipped - unwanted resolution: ", nzbs[idx].Title)
				continue
			}
			if len(quality.Wanted_quality) >= 1 && !wanted_release_quality {
				logger.Log.Debug("Skipped - unwanted quality: ", nzbs[idx].Title)
				continue
			}
			if m.Priority != 0 {
				if minPrio != 0 {
					if m.Priority <= minPrio {
						logger.Log.Debug("Skipped - Prio lower: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
						continue
					}
					logger.Log.Debug("ok - prio higher: ", nzbs[idx].Title, " old prio ", minPrio, " found prio ", m.Priority)
				}
				nzbprio := nzbwithprio{m.Priority, indexer.Template_indexer, *m, nzbs[idx], foundmovie, database.SerieEpisode{}}
				retnzb = append(retnzb, nzbprio)
			} else {
				logger.Log.Debug("Skipped - Prio not matched: ", nzbs[idx].Title)
			}
		}
	}
	return retnzb
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func downloadFile(saveIn string, fileprefix string, filename string, url string) error {

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	if len(filename) == 0 {
		filename = path.Base(resp.Request.URL.String())
	}
	var filepath string
	if len(fileprefix) >= 1 {
		filename = fileprefix + filename
	}
	filepath = path.Join(saveIn, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	return err
}

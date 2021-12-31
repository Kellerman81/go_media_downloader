package utils

import (
	"os"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/remeh/sizedwaitgroup"
)

func JobImportFileCheck(file string, dbtype string, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	if _, err := os.Stat(file); os.IsNotExist(err) {
		if strings.EqualFold(dbtype, "movie") {
			moviesf, _ := database.QueryMovieFiles(database.Query{Select: "id, movie_id", Where: "location=?", WhereArgs: []interface{}{file}})
			for idx := range moviesf {
				movie_id := moviesf[idx].MovieID
				_, sqlerr := database.DeleteRow("movie_files", database.Query{Where: "id=?", WhereArgs: []interface{}{moviesf[idx].ID}})
				if sqlerr == nil {
					counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id=?", WhereArgs: []interface{}{movie_id}})
					if counter == 0 {
						database.UpdateColumn("movies", "missing", true, database.Query{Where: "id=?", WhereArgs: []interface{}{movie_id}})
					}
				}
			}
		} else {
			seriefiles, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "id, serie_episode_id", Where: "location=?", WhereArgs: []interface{}{file}})
			for idx := range seriefiles {
				episode_id := seriefiles[idx].SerieEpisodeID
				_, sqlerr := database.DeleteRow("serie_episode_files", database.Query{Where: "id=?", WhereArgs: []interface{}{seriefiles[idx].ID}})
				if sqlerr == nil {
					counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{episode_id}})
					if counter == 0 {
						database.UpdateColumn("serie_episodes", "missing", true, database.Query{Where: "id=?", WhereArgs: []interface{}{episode_id}})
					}
				}
			}
		}
	}
}

type feedResults struct {
	Series config.MainSerieConfig
	Movies []database.Dbmovie
}

func Feeds(configEntry config.MediaTypeConfig, list config.MediaListsConfig) feedResults {
	if !list.Enabled {
		logger.Log.Debug("Error - Group list not enabled")
		return feedResults{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		logger.Log.Debug("Error - list not found")
		return feedResults{}
	}
	var cfg_list config.ListsConfig
	config.ConfigGet("list_"+list.Template_list, &cfg_list)

	if !cfg_list.Enabled {
		logger.Log.Debug("Error - list not enabled")
		return feedResults{}
	}

	if strings.EqualFold(cfg_list.Type, "seriesconfig") {
		return feedResults{Series: config.LoadSerie(cfg_list.Series_config_file)}
	}
	if strings.EqualFold(cfg_list.Type, "traktpublicshowlist") {
		return feedResults{Series: GetTraktUserPublicShowList(configEntry, list)}
	}

	if strings.EqualFold(cfg_list.Type, "newznabrss") {
		searchnow := searcher.NewSearcher(configEntry, list.Template_quality)
		searchresults := searchnow.GetRSSFeed("movie", list)
		for idxres := range searchresults.Nzbs {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idxres].NZB.Title)
			downloadnow := downloader.NewDownloader(configEntry, "rss")
			if searchresults.Nzbs[idxres].Nzbmovie.ID != 0 {
				downloadnow.SetMovie(searchresults.Nzbs[idxres].Nzbmovie)
				downloadnow.DownloadNzb(searchresults.Nzbs[idxres])
			} else if searchresults.Nzbs[idxres].Nzbepisode.ID != 0 {
				downloadnow.SetSeriesEpisode(searchresults.Nzbs[idxres].Nzbepisode)
				downloadnow.DownloadNzb(searchresults.Nzbs[idxres])
			}
		}
		return feedResults{}
	}
	if strings.EqualFold(cfg_list.Type, "imdbcsv") {
		return feedResults{Movies: getMissingIMDBMoviesV2(configEntry, list)}
	}
	if strings.EqualFold(cfg_list.Type, "traktpublicmovielist") {
		return feedResults{Movies: GetTraktUserPublicMovieList(configEntry, list)}
	}
	if strings.EqualFold(cfg_list.Type, "traktmoviepopular") {
		traktpopular, err := apiexternal.TraktApi.GetMoviePopular(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))

			for idx := range traktpopular {
				if len(traktpopular[idx].Ids.Imdb) == 0 {
					continue
				}

				if !importfeed.AllowMovieImport(traktpopular[idx].Ids.Imdb, cfg_list) {
					continue
				}
				dbentry := database.Dbmovie{ImdbID: traktpopular[idx].Ids.Imdb, Title: traktpopular[idx].Title, Year: traktpopular[idx].Year}
				d = append(d, dbentry)
			}
			return feedResults{Movies: d}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktmovieanticipated") {
		traktpopular, err := apiexternal.TraktApi.GetMovieAnticipated(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))

			for idx := range traktpopular {
				if len(traktpopular[idx].Movie.Ids.Imdb) == 0 {
					continue
				}
				if !importfeed.AllowMovieImport(traktpopular[idx].Movie.Ids.Imdb, cfg_list) {
					continue
				}
				dbentry := database.Dbmovie{ImdbID: traktpopular[idx].Movie.Ids.Imdb, Title: traktpopular[idx].Movie.Title, Year: traktpopular[idx].Movie.Year}
				d = append(d, dbentry)
			}
			return feedResults{Movies: d}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktmovietrending") {
		traktpopular, err := apiexternal.TraktApi.GetMovieTrending(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))

			for idx := range traktpopular {
				if len(traktpopular[idx].Movie.Ids.Imdb) == 0 {
					continue
				}
				if !importfeed.AllowMovieImport(traktpopular[idx].Movie.Ids.Imdb, cfg_list) {
					continue
				}
				dbentry := database.Dbmovie{ImdbID: traktpopular[idx].Movie.Ids.Imdb, Title: traktpopular[idx].Movie.Title, Year: traktpopular[idx].Movie.Year}
				d = append(d, dbentry)
			}
			return feedResults{Movies: d}
		}
	}

	if strings.EqualFold(cfg_list.Type, "traktseriepopular") {
		traktpopular, err := apiexternal.TraktApi.GetSeriePopular(cfg_list.Limit)
		if err == nil {
			d := make([]config.SerieConfig, 0, len(traktpopular))

			for idx := range traktpopular {
				if traktpopular[idx].Ids.Tvdb == 0 {
					continue
				}
				dbentry := config.SerieConfig{Name: traktpopular[idx].Title, TvdbID: traktpopular[idx].Ids.Tvdb}
				d = append(d, dbentry)
			}
			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktserieanticipated") {
		traktpopular, err := apiexternal.TraktApi.GetSerieAnticipated(cfg_list.Limit)
		if err == nil {
			d := make([]config.SerieConfig, 0, len(traktpopular))

			for idx := range traktpopular {
				if traktpopular[idx].Serie.Ids.Tvdb == 0 {
					continue
				}
				dbentry := config.SerieConfig{Name: traktpopular[idx].Serie.Title, TvdbID: traktpopular[idx].Serie.Ids.Tvdb}
				d = append(d, dbentry)
			}
			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}
	}
	if strings.EqualFold(cfg_list.Type, "traktserietrending") {
		traktpopular, err := apiexternal.TraktApi.GetSerieTrending(cfg_list.Limit)
		if err == nil {
			d := make([]config.SerieConfig, 0, len(traktpopular))

			for idx := range traktpopular {
				if traktpopular[idx].Serie.Ids.Tvdb == 0 {
					continue
				}
				dbentry := config.SerieConfig{Name: traktpopular[idx].Serie.Title, TvdbID: traktpopular[idx].Serie.Ids.Tvdb}
				d = append(d, dbentry)
			}
			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}
	}
	logger.Log.Error("Feed Config not found - template: ", list.Template_list, " - type: ", cfg_list, " - name: ", cfg_list.Name)
	return feedResults{}
}

func findFiles(row config.MediaTypeConfig) []string {
	if len(row.Data) == 1 {
		if config.ConfigCheck("path_" + row.Data[0].Template_path) {
			var cfg_path config.PathsConfig
			config.ConfigGet("path_"+row.Data[0].Template_path, &cfg_path)

			return scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
		}
	} else {
		var filesfound []string
		for idxpath := range row.Data {
			if !config.ConfigCheck("path_" + row.Data[idxpath].Template_path) {
				continue
			}
			var cfg_path config.PathsConfig
			config.ConfigGet("path_"+row.Data[idxpath].Template_path, &cfg_path)

			filesfound = append(filesfound, scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)...)
		}
		return filesfound
	}
	return []string{}
}

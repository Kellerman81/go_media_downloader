package utils

import (
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/downloader"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/Kellerman81/go_media_downloader/sizedwaitgroup"
)

func jobImportFileCheck(file string, dbtype string, wg *sizedwaitgroup.SizedWaitGroup) {
	defer wg.Done()
	if !scanner.CheckFileExist(file) {
		if strings.EqualFold(dbtype, "movie") {
			moviesf, _ := database.QueryStaticColumnsTwoInt("select id, movie_id from movie_files where location=?", "select count(id) from movie_files where location=?", file)
			for idx := range moviesf {
				logger.Log.Debug("File was removed: ", file)
				_, sqlerr := database.DeleteRow("movie_files", database.Query{Where: "id=?", WhereArgs: []interface{}{moviesf[idx].Num1}})
				if sqlerr == nil {
					counter, _ := database.CountRowsStatic("Select count(id) from movie_files where movie_id = ?", moviesf[idx].Num2)
					//counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id=?", WhereArgs: []interface{}{movie_id}})
					if counter == 0 {
						database.UpdateColumn("movies", "missing", true, database.Query{Where: "id=?", WhereArgs: []interface{}{moviesf[idx].Num2}})
					}
				}
			}
		} else {
			seriefiles, _ := database.QueryStaticColumnsTwoInt("select id, serie_episode_id from serie_episode_files where location=?", "select count(id) from serie_episode_files where location=?", file)
			for idx := range seriefiles {
				logger.Log.Debug("File was removed: ", file)
				_, sqlerr := database.DeleteRow("serie_episode_files", database.Query{Where: "id=?", WhereArgs: []interface{}{seriefiles[idx].Num1}})
				if sqlerr == nil {
					counter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where serie_episode_id = ?", seriefiles[idx].Num2)
					//counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id=?", WhereArgs: []interface{}{episode_id}})
					if counter == 0 {
						database.UpdateColumn("serie_episodes", "missing", true, database.Query{Where: "id=?", WhereArgs: []interface{}{seriefiles[idx].Num2}})
					}
				}
			}
		}
	}
}

func InitRegex() {

	ident, _ := regexp.Compile(`(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`)
	title, _ := regexp.Compile(`^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)(?: )?[ex](?:[0-9]{1,3})(?:[^0-9]|$))`)

	config.RegexSeriesIdentifier = *ident
	config.RegexSeriesTitle = *title
}

func InitialFillSeries() {
	logger.Log.Infoln("Starting initial DB fill for series")

	for _, idxserie := range config.ConfigGetPrefix("serie_") {
		if !config.ConfigCheck(idxserie.Name) {
			continue
		}
		cfg_serie := config.ConfigGet(idxserie.Name).Data.(config.MediaTypeConfig)

		job := strings.ToLower("feeds")
		dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
		for idxlist := range cfg_serie.Lists {
			Importnewseriessingle(idxserie.Name, cfg_serie.Lists[idxlist].Name)
		}
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

	}
	for _, idxserie := range config.ConfigGetPrefix("serie_") {
		if !config.ConfigCheck(idxserie.Name) {
			continue
		}
		cfg_serie := config.ConfigGet(idxserie.Name).Data.(config.MediaTypeConfig)

		job := strings.ToLower("datafull")
		dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
		Getnewepisodes(idxserie.Name)
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

	}
}

func InitialFillMovies() {
	logger.Log.Infoln("Starting initial DB fill for movies")

	FillImdb()

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		if !config.ConfigCheck(idxmovie.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(idxmovie.Name).Data.(config.MediaTypeConfig)

		job := strings.ToLower("feeds")
		dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})
		for idxlist := range cfg_movie.Lists {
			Importnewmoviessingle(idxmovie.Name, cfg_movie.Lists[idxlist].Name)
		}
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

	}

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		if !config.ConfigCheck(idxmovie.Name) {
			continue
		}
		cfg_movie := config.ConfigGet(idxmovie.Name).Data.(config.MediaTypeConfig)

		job := strings.ToLower("datafull")
		dbresult, _ := database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})

		Getnewmovies(idxmovie.Name)
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id=?", WhereArgs: []interface{}{dbid}})

	}
}

func FillImdb() {
	file := "./init_imdb"
	if runtime.GOOS == "windows" {
		file = "init_imdb.exe"
	}
	cmd := exec.Command(file)
	out, errexec := cmd.Output()
	logger.Log.Infoln(string(out))
	if scanner.CheckFileExist(file) && errexec == nil {
		database.DBImdb.Close()
		os.Remove("./imdb.db")
		os.Rename("./imdbtemp.db", "./imdb.db")
		database.DBImdb = database.InitImdbdb("info", "imdb")
	}
	cmd = nil
	out = nil
}

type feedResults struct {
	Series config.MainSerieConfig
	Movies []database.Dbmovie
}

func feeds(configTemplate string, listConfig string) feedResults {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if !list.Enabled {
		logger.Log.Debug("Error - Group list not enabled")
		return feedResults{}
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		logger.Log.Debug("Error - list not found")
		return feedResults{}
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)

	if !cfg_list.Enabled {
		logger.Log.Debug("Error - list not enabled")
		return feedResults{}
	}

	if strings.EqualFold(cfg_list.Type, "seriesconfig") {
		return feedResults{Series: config.LoadSerie(cfg_list.Series_config_file)}
	}
	if strings.EqualFold(cfg_list.Type, "traktpublicshowlist") {
		return feedResults{Series: getTraktUserPublicShowList(configTemplate, listConfig)}
	}

	if strings.EqualFold(cfg_list.Type, "newznabrss") {
		searchnow := searcher.NewSearcher(configTemplate, list.Template_quality)
		searchresults := searchnow.GetRSSFeed("movie", listConfig)
		for idxres := range searchresults.Nzbs {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idxres].NZB.Title)
			downloadnow := downloader.NewDownloader(configTemplate, "rss")
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
		return feedResults{Movies: getMissingIMDBMoviesV2(configTemplate, listConfig)}
	}
	if strings.EqualFold(cfg_list.Type, "traktpublicmovielist") {
		return feedResults{Movies: getTraktUserPublicMovieList(configTemplate, listConfig)}
	}
	if strings.EqualFold(cfg_list.Type, "traktmoviepopular") {
		traktpopular, err := apiexternal.TraktApi.GetMoviePopular(cfg_list.Limit)
		if err == nil {
			d := make([]database.Dbmovie, 0, len(traktpopular))
			for idx := range traktpopular {
				if len(traktpopular[idx].Ids.Imdb) == 0 {
					continue
				}

				if !importfeed.AllowMovieImport(traktpopular[idx].Ids.Imdb, list.Template_list) {
					continue
				}

				d = append(d, database.Dbmovie{ImdbID: traktpopular[idx].Ids.Imdb, Title: traktpopular[idx].Title, Year: traktpopular[idx].Year})
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
				if !importfeed.AllowMovieImport(traktpopular[idx].Movie.Ids.Imdb, list.Template_list) {
					continue
				}

				d = append(d, database.Dbmovie{ImdbID: traktpopular[idx].Movie.Ids.Imdb, Title: traktpopular[idx].Movie.Title, Year: traktpopular[idx].Movie.Year})
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
				if !importfeed.AllowMovieImport(traktpopular[idx].Movie.Ids.Imdb, list.Template_list) {
					continue
				}

				d = append(d, database.Dbmovie{ImdbID: traktpopular[idx].Movie.Ids.Imdb, Title: traktpopular[idx].Movie.Title, Year: traktpopular[idx].Movie.Year})
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
				d = append(d, config.SerieConfig{Name: traktpopular[idx].Title, TvdbID: traktpopular[idx].Ids.Tvdb})
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
				d = append(d, config.SerieConfig{Name: traktpopular[idx].Serie.Title, TvdbID: traktpopular[idx].Serie.Ids.Tvdb})
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
				d = append(d, config.SerieConfig{Name: traktpopular[idx].Serie.Title, TvdbID: traktpopular[idx].Serie.Ids.Tvdb})
			}

			return feedResults{Series: config.MainSerieConfig{Serie: d}}
		}

	}
	logger.Log.Error("Feed Config not found - template: ", list.Template_list, " - type: ", cfg_list, " - name: ", cfg_list.Name)
	return feedResults{}
}

func findFiles(configTemplate string) []string {
	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if len(row.Data) == 1 {
		if config.ConfigCheck("path_" + row.Data[0].Template_path) {
			cfg_path := config.ConfigGet("path_" + row.Data[0].Template_path).Data.(config.PathsConfig)

			return scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
		}
	} else {
		var filesfound []string
		for idxpath := range row.Data {
			if !config.ConfigCheck("path_" + row.Data[idxpath].Template_path) {
				continue
			}
			cfg_path := config.ConfigGet("path_" + row.Data[idxpath].Template_path).Data.(config.PathsConfig)
			if idxpath == 0 {
				filesfound = scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)
			} else {
				filesfound = append(filesfound, scanner.GetFilesDir(cfg_path.Path, cfg_path.AllowedVideoExtensions, cfg_path.AllowedVideoExtensionsNoRename, cfg_path.Blocked)...)
			}
		}
		return filesfound
	}
	return []string{}
}

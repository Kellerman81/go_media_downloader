package utils

import (
	"bytes"
	"database/sql"
	"errors"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"runtime/debug"
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
	"github.com/shomali11/parallelizer"
)

func jobImportFileCheck(file string, dbtype string) {
	var files []database.Dbstatic_TwoInt
	defer logger.ClearVar(&files)
	var query, querycount, subquerycount, table, updatetable string

	if !scanner.CheckFileExist(file) {
		if strings.EqualFold(dbtype, "movie") {
			query = "select id, movie_id from movie_files where location = ?"
			querycount = "select count(id) from movie_files where location = ?"
			subquerycount = "Select count(id) from movie_files where movie_id = ?"
			table = "movie_files"
			updatetable = "movies"
		} else {
			query = "select id, serie_episode_id from serie_episode_files where location = ?"
			querycount = "select count(id) from serie_episode_files where location = ?"
			subquerycount = "Select count(id) from serie_episode_files where serie_episode_id = ?"
			table = "serie_episode_files"
			updatetable = "serie_episodes"
		}
		files, _ = database.QueryStaticColumnsTwoInt(query, querycount, file)
		for idx := range files {
			logger.Log.Debug("File was removed: ", file)
			_, sqlerr := database.DeleteRow(table, database.Query{Where: "id = ?", WhereArgs: []interface{}{files[idx].Num1}})
			if sqlerr == nil {
				counter, sqlerr := database.CountRowsStatic(subquerycount, files[idx].Num2)
				if counter == 0 && sqlerr == nil {
					database.UpdateColumn(updatetable, "missing", true, database.Query{Where: "id = ?", WhereArgs: []interface{}{files[idx].Num2}})
				}
			}
		}
	}
}

func InitRegex() {
	config.RegexAdd("RegexSeriesIdentifier", *regexp.MustCompile(`(?i)s?[0-9]{1,4}((?:(?:(?: )?-?(?: )?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`))
	config.RegexAdd("RegexSeriesTitle", *regexp.MustCompile(`^(.*)(?i)(?:(?:\.| - |-)S(?:[0-9]+)(?: )?[ex](?:[0-9]{1,3})(?:[^0-9]|$))`))
}

func InitialFillSeries() {
	logger.Log.Infoln("Starting initial DB fill for series")

	var cfg_serie config.MediaTypeConfig
	var job string
	var dbresult sql.Result
	for _, idxserie := range config.ConfigGetPrefix("serie_") {
		if !config.ConfigCheck(idxserie.Name) {
			continue
		}
		cfg_serie = config.ConfigGet(idxserie.Name).Data.(config.MediaTypeConfig)

		job = strings.ToLower("feeds")
		dbresult, _ = database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
		for idxlist := range cfg_serie.Lists {
			Importnewseriessingle(idxserie.Name, cfg_serie.Lists[idxlist].Name)
		}
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})

	}
	for _, idxserie := range config.ConfigGetPrefix("serie_") {
		if !config.ConfigCheck(idxserie.Name) {
			continue
		}
		cfg_serie = config.ConfigGet(idxserie.Name).Data.(config.MediaTypeConfig)

		job = strings.ToLower("datafull")
		dbresult, _ = database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_serie.Name, "Serie", time.Now()})
		GetNewFilesMap(idxserie.Name, "series", "")
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})

	}
}

func InitialFillMovies() {
	logger.Log.Infoln("Starting initial DB fill for movies")

	FillImdb()
	debug.FreeOSMemory()
	var cfg_movie config.MediaTypeConfig
	var job string
	var dbresult sql.Result
	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		if !config.ConfigCheck(idxmovie.Name) {
			continue
		}
		cfg_movie = config.ConfigGet(idxmovie.Name).Data.(config.MediaTypeConfig)

		job = strings.ToLower("feeds")
		dbresult, _ = database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})
		for idxlist := range cfg_movie.Lists {
			Importnewmoviessingle(idxmovie.Name, cfg_movie.Lists[idxlist].Name)
		}
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})

	}

	for _, idxmovie := range config.ConfigGetPrefix("movie_") {
		if !config.ConfigCheck(idxmovie.Name) {
			continue
		}
		cfg_movie = config.ConfigGet(idxmovie.Name).Data.(config.MediaTypeConfig)

		job = strings.ToLower("datafull")
		dbresult, _ = database.InsertArray("job_histories", []string{"job_type", "job_group", "job_category", "started"},
			[]interface{}{job, cfg_movie.Name, "Movie", time.Now()})
		GetNewFilesMap(idxmovie.Name, "movie", "")
		dbid, _ := dbresult.LastInsertId()
		database.UpdateColumn("job_histories", "ended", time.Now(), database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})

	}
}

func FillImdb() {
	file := "./init_imdb"
	if runtime.GOOS == "windows" {
		file = "init_imdb.exe"
	}
	cmd := exec.Command(file)
	defer logger.ClearVar(cmd)

	var stdoutBuf bytes.Buffer
	cmd.Stdout = io.MultiWriter(os.Stdout, &stdoutBuf)

	errexec := cmd.Run()
	if scanner.CheckFileExist(file) && errexec == nil {
		logger.Log.Infoln(stdoutBuf.String())
		database.DBImdb.Close()
		os.Remove("./databases/imdb.db")
		os.Rename("./databases/imdbtemp.db", "./databases/imdb.db")
		database.DBImdb = database.InitImdbdb("info", "imdb")
	}
}

type feedResults struct {
	Series config.MainSerieConfig
	Movies []database.Dbmovie
}

func feeds(configTemplate string, listConfig string) (feedResults, error) {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	if !list.Enabled {
		logger.Log.Debug("Error - Group list not enabled")
		return feedResults{}, errors.New("group list not enabled")
	}
	if !config.ConfigCheck("list_" + list.Template_list) {
		logger.Log.Debug("Error - list not found")
		return feedResults{}, errors.New("list not found")
	}
	cfg_list := config.ConfigGet("list_" + list.Template_list).Data.(config.ListsConfig)

	if !cfg_list.Enabled {
		logger.Log.Debug("Error - list not enabled")
		return feedResults{}, errors.New("list not enabled")
	}

	switch strings.ToLower(cfg_list.ListType) {
	case "seriesconfig":
		return feedResults{Series: config.LoadSerie(cfg_list.Series_config_file)}, nil
	case "traktpublicshowlist":
		serielist, err := getTraktUserPublicShowList(configTemplate, listConfig)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&serielist)
		return feedResults{Series: serielist}, nil
	case "imdbcsv":
		movielist, err := getMissingIMDBMoviesV2(configTemplate, listConfig)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Movies: movielist}, nil
	case "traktpublicmovielist":
		movielist, err := getTraktUserPublicMovieList(configTemplate, listConfig)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Movies: movielist}, nil
	case "traktmoviepopular":
		movielist, err := gettractmoviefeeds("popular", cfg_list.Limit, list.Template_list)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Series: movielist.Series}, nil
	case "traktmovieanticipated":
		movielist, err := gettractmoviefeeds("anticipated", cfg_list.Limit, list.Template_list)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Movies: movielist.Movies}, nil
	case "traktmovietrending":
		movielist, err := gettractmoviefeeds("trending", cfg_list.Limit, list.Template_list)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Movies: movielist.Movies}, nil
	case "traktseriepopular":
		movielist, err := gettractseriefeeds("popular", cfg_list.Limit)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Series: movielist.Series}, nil
	case "traktserieanticipated":
		movielist, err := gettractseriefeeds("anticipated", cfg_list.Limit)
		if err != nil {
			return feedResults{}, err
		}
		return feedResults{Series: movielist.Series}, nil
	case "traktserietrending":
		movielist, err := gettractseriefeeds("trending", cfg_list.Limit)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&movielist)
		return feedResults{Series: movielist.Series}, nil
	case "newznabrss":
		searchnow := searcher.NewSearcher(configTemplate, list.Template_quality)
		defer searchnow.Close()
		searchresults, err := searchnow.GetRSSFeed("movie", listConfig)
		if err != nil {
			return feedResults{}, err
		}
		defer logger.ClearVar(&searchresults)
		for idxres := range searchresults.Nzbs {
			logger.Log.Debug("nzb found - start downloading: ", searchresults.Nzbs[idxres].NZB.Title)
			downloadnow := downloader.NewDownloader(configTemplate)
			if searchresults.Nzbs[idxres].NzbmovieID != 0 {
				downloadnow.SetMovie(searchresults.Nzbs[idxres].NzbmovieID)
				downloadnow.DownloadNzb(searchresults.Nzbs[idxres])
			} else if searchresults.Nzbs[idxres].NzbepisodeID != 0 {
				downloadnow.SetSeriesEpisode(searchresults.Nzbs[idxres].NzbepisodeID)
				downloadnow.DownloadNzb(searchresults.Nzbs[idxres])
			}
			downloadnow.Close()
		}
		return feedResults{}, nil
	}

	logger.Log.Error("Feed Config not found - template: ", list.Template_list, " - type: ", cfg_list, " - name: ", cfg_list.Name)
	return feedResults{}, errors.New("feed Config not found")
}

func gettractmoviefeeds(traktlist string, limit int, templateList string) (feedResults, error) {
	var traktpopular []apiexternal.TraktMovie
	var err error
	switch traktlist {
	case "popular":
		traktpopular, err = apiexternal.TraktApi.GetMoviePopular(limit)
		break
	case "trending":
		traktpopular, err = apiexternal.TraktApi.GetMovieTrending(limit)
		break
	case "anticipated":
		traktpopular, err = apiexternal.TraktApi.GetMovieAnticipated(limit)
		break
	default:
		return feedResults{}, errors.New("wrong type")

	}
	defer logger.ClearVar(&traktpopular)

	if err == nil {
		d := make([]database.Dbmovie, 0, len(traktpopular))
		defer logger.ClearVar(&d)
		for idx := range traktpopular {
			if len(traktpopular[idx].Ids.Imdb) == 0 {
				continue
			}

			if !importfeed.AllowMovieImport(traktpopular[idx].Ids.Imdb, templateList) {
				continue
			}

			d = append(d, database.Dbmovie{ImdbID: traktpopular[idx].Ids.Imdb, Title: traktpopular[idx].Title, Year: traktpopular[idx].Year})
		}

		return feedResults{Movies: d}, nil
	}
	return feedResults{}, err
}

func gettractseriefeeds(traktlist string, limit int) (feedResults, error) {
	var traktpopular []apiexternal.TraktSerie
	var err error
	switch traktlist {
	case "popular":
		traktpopular, err = apiexternal.TraktApi.GetSeriePopular(limit)
		break
	case "trending":
		traktpopular, err = apiexternal.TraktApi.GetSerieTrending(limit)
		break
	case "anticipated":
		traktpopular, err = apiexternal.TraktApi.GetSerieAnticipated(limit)
		break
	default:
		return feedResults{}, errors.New("wrong type")

	}
	defer logger.ClearVar(&traktpopular)

	if err == nil {
		d := make([]config.SerieConfig, 0, len(traktpopular))
		defer logger.ClearVar(&d)
		for idx := range traktpopular {
			if traktpopular[idx].Ids.Tvdb == 0 {
				continue
			}
			d = append(d, config.SerieConfig{Name: traktpopular[idx].Title, TvdbID: traktpopular[idx].Ids.Tvdb})
		}

		return feedResults{Series: config.MainSerieConfig{Serie: d}}, nil
	}
	return feedResults{}, err
}

func findFilesMap(configTemplate string) (logger.StringSet, logger.StringSet, error) {
	filesfound := logger.NewStringSet()
	defer filesfound.Clear()
	fileswanted := logger.NewStringSet()
	defer fileswanted.Clear()
	row := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if len(row.Data) == 1 {
		if config.ConfigCheck("path_" + row.Data[0].Template_path) {
			cfg_path := config.ConfigGet("path_" + row.Data[0].Template_path).Data.(config.PathsConfig)

			return scanner.GetFilesDirNewOld(configTemplate, cfg_path.Path, "path_"+row.Data[0].Template_path)
		}
	} else {
		var filesfound_add logger.StringSet
		var fileswanted_add logger.StringSet
		defer filesfound_add.Clear()
		defer fileswanted_add.Clear()
		var err error
		for idxpath := range row.Data {
			if !config.ConfigCheck("path_" + row.Data[idxpath].Template_path) {
				continue
			}
			cfg_path := config.ConfigGet("path_" + row.Data[idxpath].Template_path).Data.(config.PathsConfig)
			filesfound_add, fileswanted_add, err = scanner.GetFilesDirNewOld(configTemplate, cfg_path.Path, "path_"+row.Data[idxpath].Template_path)
			if err == nil {
				if idxpath == 0 {
					filesfound = filesfound_add
					fileswanted = fileswanted_add
				} else {
					filesfound.Union(filesfound_add)
					fileswanted.Union(fileswanted_add)
				}
			}
		}
		return filesfound, fileswanted, nil
	}
	return logger.StringSet{}, logger.StringSet{}, errors.New("no data")
}

func Checkignorelistsonpath(configTemplate string, file string, listConfig string) bool {
	return checkignorelistsonpath(configTemplate, file, listConfig)
}
func checkignorelistsonpath(configTemplate string, file string, listConfig string) bool {
	list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
	var querypath, queryignore string
	if strings.HasPrefix(configTemplate, "movie_") {
		querypath = "Select dbmovie_id from movies where id in (Select movie_id from movie_files where location = ?) and listname = ?"
		queryignore = "Select count(id) from movies where dbmovie_id = ? and listname = ?"
	} else {
		querypath = "Select serie_episodes.dbserie_episode_id from serie_episodes inner join series on series.id=serie_episodes.serie_id where serie_episodes.id in (Select serie_episode_id from serie_episode_files where location = ?) and series.listname = ?"
		queryignore = "Select count(serie_episodes.id) from serie_episodes inner join series on series.id=serie_episodes.serie_id where serie_episodes.dbserie_episode_id = ? and series.listname = ?"
	}
	entries, err := database.QueryStaticColumnsOneInt(querypath, "", file, list.Name)
	defer logger.ClearVar(&entries)
	if err == nil && len(entries) >= 1 {
		if list.Ignore_map_lists != nil {
			if len(list.Ignore_map_lists) >= 1 {
				for idx := range list.Ignore_map_lists {
					counter, _ := database.CountRowsStatic(queryignore, entries[0].Num, list.Ignore_map_lists[idx])
					if counter >= 1 {
						return false
					}
				}
			}
		}
	}
	return true
}

func Checkunmatched(configTemplate string, file string, listname string) bool {
	return checkunmatched(configTemplate, file, listname)
}
func checkunmatched(configTemplate string, file string, listname string) bool {
	var query string
	if strings.HasPrefix(configTemplate, "movie_") {
		query = "Select count(id) from movie_file_unmatcheds where filepath = ? and listname = ? and (last_checked > ? or last_checked is null)"
	} else {
		query = "Select count(id) from serie_file_unmatcheds where filepath = ? and listname = ? and (last_checked > ? or last_checked is null)"
	}
	counter, _ := database.CountRowsStatic(query, file, listname, time.Now().Add(time.Hour*-12))
	if counter >= 1 {
		return false
	}
	return true
}

func GetNewFilesMap(configTemplate string, mediatype string, checklist string) {
	if !config.ConfigCheck("general") {
		return
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.WorkerParse == 0 {
		cfg_general.WorkerParse = 1
	}

	logger.Log.Info("Scan Files")
	filesfound, fileswanted, err := findFilesMap(configTemplate)
	if err == nil {
		defer logger.ClearVar(&filesfound)
		defer logger.ClearVar(&fileswanted)

		logger.Log.Info("Workers: ", cfg_general.WorkerParse)
		var fileshavedb, listunmatched []database.Dbstatic_OneString
		defer logger.ClearVar(&fileshavedb)
		defer logger.ClearVar(&listunmatched)
		var mapfilesUnmatched, filesaddedwanted, filesadded, mapfilesHaveSub logger.StringSet
		defer mapfilesUnmatched.Clear()
		defer filesaddedwanted.Clear()
		defer filesadded.Clear()
		defer mapfilesHaveSub.Clear()

		var tablename string
		if strings.HasPrefix(configTemplate, "movie_") {
			tablename = "movie_file_unmatcheds"
		} else {
			tablename = "serie_file_unmatcheds"
		}
		for _, list := range config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig).Lists {
			if checklist != list.Name && checklist != "" {
				continue
			}
			if !config.ConfigCheck("quality_" + list.Template_quality) {
				logger.Log.Error("Quality for List: " + list.Name + " not found")
				continue
			}
			listunmatched, _ = database.QueryStaticColumnsOneString("select filepath FROM "+tablename+" WHERE listname = ? and (last_checked > ? or last_checked is null)", "select count(id) FROM "+tablename+" WHERE listname = ? and (last_checked > ? or last_checked is null)", list.Name, time.Now().Add(time.Hour*-12))
			mapfilesUnmatched = logger.NewStringSetExactSize(len(listunmatched))
			if len(listunmatched) >= 1 {
				for idx2 := range listunmatched {
					mapfilesUnmatched.Add(listunmatched[idx2].Str)
				}
			}
			listunmatched = nil
			if len(fileswanted.Values) >= 1 {
				filesaddedwanted = fileswanted
				if mapfilesUnmatched.Length() >= 1 {
					filesaddedwanted.Difference(mapfilesUnmatched)
				}
				//Wanted

				if len(filesaddedwanted.Values) >= 1 {
					swf := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerParse))
					idxfile := 0
					for idx := range filesaddedwanted.Values {
						filestr := filesaddedwanted.Values[idx]
						idxfile += 1
						logger.Log.Info("Parse ", idxfile, " path: ", filestr)
						if strings.HasPrefix(configTemplate, "movie_") {
							swf.Add(func() {
								jobImportMovieParseV2(filestr, configTemplate, list.Name, true)
							})
						} else {
							swf.Add(func() {
								jobImportSeriesParseV2(filestr, true, configTemplate, list.Name)
							})
						}
					}
					swf.Wait()
					swf.Close()
					filesaddedwanted.Clear()
				}
			}

			if len(filesfound.Values) >= 1 {
				if strings.HasPrefix(configTemplate, "movie_") {
					fileshavedb, err = database.QueryStaticColumnsOneString("Select movie_files.location from movie_files inner join movies on movies.id=movie_files.movie_id where movies.listname = ?", "Select count(movie_files.id) from movie_files inner join movies on movies.id=movie_files.movie_id where movies.listname = ?", list.Name)
				} else {
					fileshavedb, err = database.QueryStaticColumnsOneString("Select serie_episode_files.location from serie_episode_files inner join series on series.id=serie_episode_files.serie_id where series.listname = ?", "Select count(serie_episode_files.id) from serie_episode_files inner join series on series.id=serie_episode_files.serie_id where series.listname = ?", list.Name)
				}

				if err != nil {
					logger.Log.Error("File Struct error", err)
					continue
				}
				filesadded = filesfound
				if len(fileshavedb) >= 1 {
					mapfilesHave := logger.NewStringSetExactSize(len(fileshavedb))
					for idx3 := range fileshavedb {
						mapfilesHave.Add(fileshavedb[idx3].Str)
					}
					fileshavedb = nil

					filesadded.Difference(mapfilesHave)
					mapfilesHaveSub.Clear()
				}
				if mapfilesUnmatched.Length() >= 1 {
					filesadded.Difference(mapfilesUnmatched)
				}
				mapfilesUnmatched.Clear()

				for idx2 := range list.Ignore_map_lists {
					if strings.HasPrefix(configTemplate, "movie_") {
						fileshavedb, err = database.QueryStaticColumnsOneString("Select movie_files.location from movie_files inner join movies on movies.id=movie_files.movie_id where movies.listname = ?", "Select count(movie_files.id) from movie_files inner join movies on movies.id=movie_files.movie_id where movies.listname = ?", list.Ignore_map_lists[idx2])
					} else {
						fileshavedb, err = database.QueryStaticColumnsOneString("Select serie_episode_files.location from serie_episode_files inner join series on series.id=serie_episode_files.serie_id where series.listname = ?", "Select count(serie_episode_files.id) from serie_episode_files inner join series on series.id=serie_episode_files.serie_id where series.listname = ?", list.Ignore_map_lists[idx2])
					}

					mapfilesHaveSub = logger.NewStringSetExactSize(len(fileshavedb))
					for idx3 := range fileshavedb {
						mapfilesHaveSub.Add(fileshavedb[idx3].Str)
					}
					fileshavedb = nil
					filesadded.Difference(mapfilesHaveSub)
					mapfilesHaveSub.Clear()
				}

				for idx2 := range list.Replace_map_lists {
					if strings.HasPrefix(configTemplate, "movie_") {
						fileshavedb, err = database.QueryStaticColumnsOneString("Select movie_files.location from movie_files inner join movies on movies.id=movie_files.movie_id where movies.listname = ?", "Select count(movie_files.id) from movie_files inner join movies on movies.id=movie_files.movie_id where movies.listname = ?", list.Replace_map_lists[idx2])
					} else {
						fileshavedb, err = database.QueryStaticColumnsOneString("Select serie_episode_files.location from serie_episode_files inner join series on series.id=serie_episode_files.serie_id where series.listname = ?", "Select count(serie_episode_files.id) from serie_episode_files inner join series on series.id=serie_episode_files.serie_id where series.listname = ?", list.Replace_map_lists[idx2])
					}

					mapfilesHaveSub = logger.NewStringSetExactSize(len(fileshavedb))
					for idx3 := range fileshavedb {
						mapfilesHaveSub.Add(fileshavedb[idx3].Str)
					}
					fileshavedb = nil
					filesadded.Difference(mapfilesHaveSub)
					mapfilesHaveSub.Clear()
				}
				if len(fileswanted.Values) >= 1 {
					filesadded.Difference(fileswanted)
				}

				if len(filesadded.Values) >= 1 {
					idxfile := 0
					swf := parallelizer.NewGroup(parallelizer.WithPoolSize(cfg_general.WorkerParse))
					for idx := range filesadded.Values {
						filestr := filesadded.Values[idx]
						idxfile += 1
						logger.Log.Info("Parse ", idxfile, " path: ", filestr)
						if strings.HasPrefix(configTemplate, "movie_") {
							swf.Add(func() {
								jobImportMovieParseV2(filestr, configTemplate, list.Name, true)
							})
						} else {
							swf.Add(func() {
								jobImportSeriesParseV2(filestr, true, configTemplate, list.Name)
							})
						}
					}
					swf.Wait()
					swf.Close()
					filesadded.Clear()
				}
			}
		}
	}
}

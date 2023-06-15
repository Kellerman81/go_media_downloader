package importfeed

import (
	"errors"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/metadata"
)

var importJobRunning string

func JobImportMovies(imdbid string, cfgpstr string, listname string, addnew bool) (uint, error) {
	if config.SettingsMedia[cfgpstr].Name == "" {
		return 0, logger.ErrCfgpNotFound
	}
	if !addnew && (config.SettingsMedia[cfgpstr].Name == "" || listname == "") {
		listname = database.QueryStringColumn("select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id=?)", &imdbid)
	}
	if imdbid == "" {
		return 0, errors.New("imdb missing")
	}
	if importJobRunning == imdbid {
		return 0, errors.New("job running")
	}
	importJobRunning = imdbid

	var checkdbmovie bool
	if config.SettingsGeneral.UseMediaCache {
		checkdbmoviei := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdbid) })
		if checkdbmoviei == -1 && !addnew {
			return 0, logger.ErrNotFoundDbmovie
		}
		checkdbmovie = checkdbmoviei >= 0
	} else {
		checkdbmovie = database.QueryIntColumn("select count() from dbmovies where imdb_id = ?", &imdbid) >= 1
	}
	i := config.GetMediaListsEntryIndex(cfgpstr, listname)
	var strtemplate string
	if i != -1 {
		strtemplate = config.SettingsMedia[cfgpstr].Lists[i].TemplateList
	}
	if !config.CheckGroup("list_", strtemplate) {
		return 0, errors.New("list template not found")
	}
	var dbmovieadded bool
	var dbmovieID uint

	if !checkdbmovie && addnew {
		_, err := AllowMovieImport(&imdbid, strtemplate)
		if err != nil {
			return 0, err
		}

		logger.Log.Debug().Str(logger.StrJob, imdbid).Msg("Insert dbmovie for")
		dbresult, err := database.InsertStatic("insert into dbmovies (Imdb_ID) VALUES (?)", &imdbid)
		if err != nil {
			return 0, err
		}
		dbmovieID = uint(database.InsertRetID(dbresult))
		dbmovieadded = true
	}
	if dbmovieID == 0 {
		if config.SettingsGeneral.UseMediaCache {
			ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdbid) })
			if ti != -1 {
				dbmovieID = uint(database.CacheDBMovie[ti].Num1)
			}
		} else {
			dbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdbid)
		}
		//dbmovieID = uint(cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdbid) }).Num1)

		//database.QueryColumn(database.QueryDbmoviesGetIDByImdb, &dbmovieID, imdbid)
	}
	if dbmovieID == 0 {
		return 0, logger.ErrNotFoundDbmovie
	}

	if dbmovieadded || !addnew {
		var err error
		dbmovieID, err = adddbmovie(&imdbid, cfgpstr, dbmovieadded)
		if err != nil {
			return dbmovieID, err
		}
		//dbmovie.Close()
	}

	if !addnew {
		return dbmovieID, nil
	}
	if dbmovieID == 0 {
		if config.SettingsGeneral.UseMediaCache {
			ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdbid) })
			if ti != -1 {
				dbmovieID = uint(database.CacheDBMovie[ti].Num1)
			}
		} else {
			dbmovieID = database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdbid)
		}
		//dbmovieID = uint(cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdbid) }).Num1)
	}
	if listname == "" {
		return 0, errors.New("listname empty")
	}
	listid := config.GetMediaListsEntryIndex(cfgpstr, listname)
	if config.SettingsMedia[cfgpstr].Lists[listid].Name == "" {
		return 0, errors.New("movie list empty")
	}

	if config.SettingsGeneral.UseMediaCache {
		if logger.IndexFunc(&database.CacheMovie, func(elem database.DbstaticOneStringOneInt) bool {
			return elem.Num == int(dbmovieID) && (strings.EqualFold(elem.Str, config.SettingsMedia[cfgpstr].Lists[listid].Name) || logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapLists, elem.Str))
		}) != -1 {
			return 0, errors.New("movie ignored")
		}
	} else {
		// args := make([]interface{}, 0, len(config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapLists)+1)
		// args = append(args, &dbmovieID)
		// args = append(args, &config.SettingsMedia[cfgpstr].Lists[listid].Name)
		// for idx := range config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapLists {
		// 	args = append(args, &config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapLists[idx])
		// }
		if database.QueryIntColumn("select count() from movies where dbmovie_id = ? and listname in ("+logger.StringsRepeat("?", ",?", len(config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapLists))+")", append([]interface{}{&dbmovieID, &config.SettingsMedia[cfgpstr].Lists[listid].Name}, config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapListsInt...)...) >= 1 {
			//logger.Clear(&args)
			return 0, errors.New("movie ignored")
		}
		//logger.Clear(&args)
	}
	// if cache.CheckFunc(logger.GlobalCache, "movies_cached", func(elem database.DbstaticOneStringOneInt) bool {
	// 	return elem.Num == int(dbmovieID) && (strings.EqualFold(elem.Str, config.SettingsMedia[cfgpstr].Lists[listid].Name) || logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[listid].IgnoreMapLists, elem.Str))
	// }) {
	// 	//is ignored or same list
	// 	return 0, errors.New("movie ignored")
	// }
	// tbl := cache.GetAllFunc(logger.GlobalCache, "movies_cached", func(elem database.DbstaticOneStringOneInt) bool {
	// 	return elem.Num == int(dbmovieID) && !strings.EqualFold(elem.Str, config.SettingsMedia[cfgpstr].Lists[listid].Name) && logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[listid].ReplaceMapLists, elem.Str)
	// })
	if config.SettingsGeneral.UseMediaCache {
		for idx := range database.CacheMovie {
			if database.CacheMovie[idx].Num != int(dbmovieID) {
				continue
			}
			if !strings.EqualFold(database.CacheMovie[idx].Str, config.SettingsMedia[cfgpstr].Lists[listid].Name) && logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[listid].ReplaceMapLists, database.CacheMovie[idx].Str) {

				if config.SettingsMedia[cfgpstr].Lists[listid].TemplateQuality == "" {
					database.UpdateColumnStatic("Update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", config.SettingsMedia[cfgpstr].Lists[listid].Name, dbmovieID, database.CacheMovie[idx].Str)
				} else {
					database.UpdateColumnStatic("Update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", config.SettingsMedia[cfgpstr].Lists[listid].Name, config.SettingsMedia[cfgpstr].Lists[listid].TemplateQuality, dbmovieID, database.CacheMovie[idx].Str)
				}
			}
		}
	} else {
		foundrows := database.QueryStaticStringArray(false, 2, "select listname FROM movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", dbmovieID, config.SettingsMedia[cfgpstr].Lists[listid].Name)
		for idx := range *foundrows {
			if logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[listid].ReplaceMapLists, (*foundrows)[idx]) {
				if config.SettingsMedia[cfgpstr].Lists[listid].TemplateQuality == "" {
					database.UpdateColumnStatic("Update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", config.SettingsMedia[cfgpstr].Lists[listid].Name, dbmovieID, (*foundrows)[idx])
				} else {
					database.UpdateColumnStatic("Update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", config.SettingsMedia[cfgpstr].Lists[listid].Name, config.SettingsMedia[cfgpstr].Lists[listid].TemplateQuality, dbmovieID, (*foundrows)[idx])
				}
			}
		}
		logger.Clear(foundrows)
	}
	//logger.Clear(tbl)
	//if len(*tbl) >= 1 {
	//	return 0, nil // errors.New("movie skipped")
	//}

	count := database.QueryIntColumn("select count() from movies where dbmovie_id = ? and listname = ?", dbmovieID, config.SettingsMedia[cfgpstr].Lists[listid].Name)
	if count == 0 {
		count = database.QueryIntColumn("select count() from movies where dbmovie_id = ? and listname in ("+logger.StringsRepeat("?", ",?", len(config.SettingsMedia[cfgpstr].Lists))+")", append([]interface{}{dbmovieID}, config.SettingsMedia[cfgpstr].ListsInt...)...)
		if count == 0 {
			logger.Log.Info().Str(logger.StrImdb, imdbid).Msg("Insert Movie for")
			_, err := database.InsertStatic("Insert into movies (missing, listname, dbmovie_id, quality_profile) values (?, ?, ?, ?)", true, config.SettingsMedia[cfgpstr].Lists[listid].Name, dbmovieID, config.SettingsMedia[cfgpstr].Lists[listid].TemplateQuality)
			if err != nil {
				return dbmovieID, err
			}
			if config.SettingsGeneral.UseMediaCache {
				database.CacheMovie = append(database.CacheMovie, database.DbstaticOneStringOneInt{Str: config.SettingsMedia[cfgpstr].Lists[listid].Name, Num: int(dbmovieID)})
			}
		}
	}
	//cache.Append(logger.GlobalCache, "movies_cached", database.DbstaticOneStringOneInt{Str: config.SettingsMedia[cfgpstr].Lists[listid].Name, Num: int(dbmovieID)})

	return dbmovieID, nil
}

func adddbmovie(imdbid *string, cfgpstr string, dbmovieadded bool) (uint, error) {
	logger.Log.Debug().Str(logger.StrJob, *imdbid).Msg("Get metadata for")
	//logger.LogAnyDebug("get metadata", logger.LoggerValue{Name: "imdb", Value: imdbid})
	dbmovie, err := database.GetDbmovie(database.FilterByImdb, imdbid)
	if err != nil {
		return 0, err
	}
	defer logger.ClearVar(dbmovie)
	metadata.Getmoviemetadata(dbmovie, true)

	database.UpdateColumnStatic(database.QueryupdatemovieStatic,
		&dbmovie.Title, &dbmovie.ReleaseDate, &dbmovie.Year, &dbmovie.Adult, &dbmovie.Budget, &dbmovie.Genres, &dbmovie.OriginalLanguage, &dbmovie.OriginalTitle, &dbmovie.Overview, &dbmovie.Popularity, &dbmovie.Revenue, &dbmovie.Runtime, &dbmovie.SpokenLanguages, &dbmovie.Status, &dbmovie.Tagline, &dbmovie.VoteAverage, &dbmovie.VoteCount, &dbmovie.TraktID, &dbmovie.MoviedbID, &dbmovie.ImdbID, &dbmovie.FreebaseMID, &dbmovie.FreebaseID, &dbmovie.FacebookID, &dbmovie.InstagramID, &dbmovie.TwitterID, &dbmovie.URL, &dbmovie.Backdrop, &dbmovie.Poster, &dbmovie.Slug, &dbmovie.ID)

	metadata.Getmoviemetatitles(dbmovie, cfgpstr)
	if dbmovieadded {
		if config.SettingsGeneral.UseMediaCache {
			database.CacheDBMovie = append(database.CacheDBMovie, database.DbstaticThreeStringOneInt{Str1: dbmovie.Title, Str2: dbmovie.Slug, Str3: *imdbid, Num1: int(dbmovie.ID)})
		}
		//cache.Append(logger.GlobalCache, "dbmovies_cached", database.DbstaticThreeStringOneInt{Str1: dbmovie.Title, Str2: dbmovie.Slug, Str3: imdbid, Num1: int(dbmovie.ID)})
	}

	if dbmovie.Title == "" {
		dbmovie.Title = database.QueryStringColumn(database.QueryDbmovieTitlesGetTitleByIDLmit1, dbmovie.ID)
		if dbmovie.Title != "" {
			database.UpdateColumnStatic("Update dbmovies SET Title = ? where id = ?", &dbmovie.Title, dbmovie.ID)
		}
	}
	id := dbmovie.ID
	dbmovie = nil
	return id, nil
}

func AllowMovieImport(imdb *string, templatelist string) (bool, error) {
	if config.SettingsList["list_"+templatelist].MinVotes != 0 {
		if database.QueryImdbIntColumn(database.QueryImdbRatingsCountByImdbVotes, &imdb, config.SettingsList["list_"+templatelist].MinVotes) >= 1 {
			return false, errors.New("error vote count too low")
		}
	}
	if config.SettingsList["list_"+templatelist].MinRating != 0 {
		if database.QueryImdbIntColumn(database.QueryImdbRatingsCountByImdbRating, &imdb, config.SettingsList["list_"+templatelist].MinRating) >= 1 {
			return false, errors.New("error average vote too low")
		}
	}

	if len(config.SettingsList["list_"+templatelist].Excludegenre) == 0 && len(config.SettingsList["list_"+templatelist].Includegenre) == 0 {
		return true, nil
	}
	genrearr := database.QueryStaticStringArray(true, 5, database.QueryImdbGenresGetGenreByImdb, &imdb)
	var excludeby string

	for idx := range config.SettingsList["list_"+templatelist].Excludegenre {
		if logger.ContainsStringsI(genrearr, config.SettingsList["list_"+templatelist].Excludegenre[idx]) {
			excludeby = config.SettingsList["list_"+templatelist].Excludegenre[idx]
			break
		}
	}
	if excludeby != "" && len(config.SettingsList["list_"+templatelist].Excludegenre) >= 1 {
		logger.Clear(genrearr)
		return false, errors.New("excluded by " + excludeby)
	}

	var includebygenre bool
	for idx := range config.SettingsList["list_"+templatelist].Includegenre {
		if logger.ContainsStringsI(genrearr, config.SettingsList["list_"+templatelist].Includegenre[idx]) {
			includebygenre = true
			break
		}
	}

	logger.Clear(genrearr)
	if !includebygenre && len(config.SettingsList["list_"+templatelist].Includegenre) >= 1 {
		return false, errors.New("included genre not found")
	}
	return true, nil
}

func checkifdbmovieyearmatches(dbmovieyear int, haveyear int) (bool, bool) {
	if dbmovieyear == 0 || haveyear == 0 {
		return false, false
	}
	if dbmovieyear == haveyear {
		return true, false
	}

	if dbmovieyear == haveyear+1 || dbmovieyear == haveyear-1 {

		return false, true
	}
	return false, false
}

func findqualityyear1(imdbID *string) bool {
	qualityTemplate := database.QueryStringColumn(database.QueryMoviesGetQualityByImdb, &imdbID)
	if qualityTemplate == "" {
		return false
	}
	return config.SettingsQuality["quality_"+qualityTemplate].CheckYear1
}

func findimdbbytitle(title *string, slugged *string, yearint int) (string, bool, bool) {
	tbl := database.QueryStaticColumnsOneStringOneInt(true, database.QueryImdbIntColumn(database.QueryImdbTitlesCountByTitleSlug, &title, &title, &slugged), database.QueryImdbTitlesGetImdbYearByTitleSlug, &title, &title, &slugged)
	defer logger.Clear(tbl)
	var found, found1 bool
	for idx := range *tbl {
		found, found1 = checkimdbyear(&(*tbl)[idx].Str, (*tbl)[idx].Num, yearint)
		if found || found1 {
			return (*tbl)[idx].Str, found, found1
		}
	}

	tblaka := database.QueryStaticStringArray(true, database.QueryImdbIntColumn(database.QueryImdbAkasCountByTitleSlug, &title, &slugged), database.QueryImdbAkasGetImdbByTitleSlug, &title, &slugged)
	defer logger.Clear(tblaka)

	for idxaka := range *tblaka {
		found, found1 = checkimdbyear(&(*tblaka)[idxaka], database.QueryImdbIntColumn(database.QueryImdbTitlesGetYearByImdb, &(*tblaka)[idxaka]), yearint)
		if found || found1 {
			return (*tblaka)[idxaka], found, found1
		}
	}

	return "", false, false
}

func findtmdbbytitle(title *string, yearint int) (string, bool, bool) {
	getmovie, _ := apiexternal.TmdbAPI.SearchMovie(title)
	defer getmovie.Close()
	if len(getmovie.Results) == 0 {
		return "", false, false
	}
	var imdbID string
	var moviedbexternal *apiexternal.TheMovieDBTVExternal
	var err error
	var found, found1 bool
	for idx2 := range getmovie.Results {
		imdbID = database.QueryStringColumn(database.QueryDbmoviesGetImdbByMoviedb, getmovie.Results[idx2].ID)
		if imdbID == "" {
			moviedbexternal, err = apiexternal.TmdbAPI.GetMovieExternal(getmovie.Results[idx2].ID)
			if err == nil {
				imdbID = moviedbexternal.ImdbID
				logger.ClearVar(moviedbexternal)
			} else {
				continue
			}
		}
		if imdbID == "" {
			continue
		}
		found, found1 = checkimdbyear(&imdbID, database.QueryImdbIntColumn(database.QueryImdbTitlesGetYearByImdb, &imdbID), yearint)
		if found || found1 {
			return imdbID, found, found1
		}
	}
	return "", false, false
}

func checkimdbyear(imdbID *string, wantyear int, year int) (bool, bool) {
	found, found1 := checkifdbmovieyearmatches(wantyear, year)
	if found {
		return found, found1
	}
	if found1 && findqualityyear1(imdbID) {
		return found, found1
	}
	return false, false
}

func findomdbbytitle(title *string, yearint int) (string, bool, bool) {
	searchomdb, err := apiexternal.OmdbAPI.SearchMovie(title, "")
	if err != nil {
		return "", false, false
	}
	defer searchomdb.Close()
	if len(searchomdb.Search) == 0 {
		return "", false, false
	}
	var found, found1 bool
	for idximdb := range searchomdb.Search {
		found, found1 = checkimdbyear(&searchomdb.Search[idximdb].ImdbID, logger.StringToInt(searchomdb.Search[idximdb].Year), yearint)
		if found || found1 {
			return searchomdb.Search[idximdb].ImdbID, found, found1
		}
	}
	return "", false, false
}

// Changes the source string
func StripTitlePrefixPostfixGetQual(title string, qualityTemplate string) error {
	if qualityTemplate == "" {
		return errors.New("qualitytemplate not found")
	}
	for idx := range config.SettingsQuality["quality_"+qualityTemplate].TitleStripSuffixForSearch {
		if logger.ContainsI(title, config.SettingsQuality["quality_"+qualityTemplate].TitleStripSuffixForSearch[idx]) {
			logger.SplitByStrMod(&title, config.SettingsQuality["quality_"+qualityTemplate].TitleStripSuffixForSearch[idx])
			continue
		}
		//title = strings.Trim(logger.TrimStringInclAfterStringInsensitive(title, list[idx]), " ")
	}
	for idx := range config.SettingsQuality["quality_"+qualityTemplate].TitleStripPrefixForSearch {
		if logger.HasPrefixI(title, config.SettingsQuality["quality_"+qualityTemplate].TitleStripPrefixForSearch[idx]) {
			logger.SplitByStrModRight(&title, config.SettingsQuality["quality_"+qualityTemplate].TitleStripPrefixForSearch[idx])
		}
	}
	return nil
}

func MovieFindDBIDByTitleSimple(imdb string, title *string, year int, addifnotfound bool) uint {
	if imdb == "" {
		imdb, _, _ = MovieFindImdbIDByTitle(title, year, "", addifnotfound)
		if imdb == "" {
			return 0
		}
	}
	if config.SettingsGeneral.UseMediaCache {
		ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdb) })
		if ti != -1 {
			return uint(database.CacheDBMovie[ti].Num1)
		}
	} else {
		return database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdb)
	}
	return 0
	//return uint(cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdb) }).Num1)
}
func MovieFindDBIDByTitle(imdb string, title *string, year int, searchtype string, addifnotfound bool) (uint, bool, bool) {
	var found1, found2 bool
	if imdb == "" {
		imdb, found1, found2 = MovieFindImdbIDByTitle(title, year, searchtype, addifnotfound)
		if !found1 && !found2 {
			return 0, false, false
		}
	} else {
		found1 = true
	}

	if config.SettingsGeneral.UseMediaCache {
		ti := logger.IndexFunc(&database.CacheDBMovie, func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdb) })
		if ti != -1 {
			return uint(database.CacheDBMovie[ti].Num1), found1, found2
		}
	} else {
		return database.QueryUintColumn(database.QueryDbmoviesGetIDByImdb, &imdb), found1, found2
	}
	return 0, found1, found2
	//return uint(cache.GetFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool { return strings.EqualFold(elem.Str3, imdb) }).Num1), found1, found2
}

var defaultsearchprovider = []string{"imdb", "tmdb", "omdb"}

func MovieFindImdbIDByTitle(title *string, year int, searchtype string, addifnotfound bool) (string, bool, bool) {
	if *title == "" || year == 0 {
		return "", false, false
	}

	// tbl := cache.GetAllFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool {
	// 	return strings.EqualFold(elem.Str1, title)
	// })
	// defer logger.Clear(tbl)
	if config.SettingsGeneral.UseMediaCache {
		var found, found1 bool
		for idx := range database.CacheDBMovie {
			if !strings.EqualFold(database.CacheDBMovie[idx].Str1, *title) {
				continue
			}
			found, found1 = checkifdbmovieyearmatches(database.QueryIntColumn(database.QueryDbmoviesGetYearByID, database.CacheDBMovie[idx].Num1), year)
			if found || found1 {
				return database.CacheDBMovie[idx].Str3, found, found1
			}
		}
	} else {
		foundrow := database.QueryIntColumn("select id FROM dbmovies where title = ?", title)
		if foundrow != 0 {
			var imdb, nn string
			var haveyear int
			database.QueryDbmovieData(uint(foundrow), &haveyear, &imdb, &nn)
			found, found1 := checkifdbmovieyearmatches(haveyear, year)
			if found || found1 {
				return imdb, found, found1
			}
		}
	}
	// tblaka := cache.GetAllFunc(logger.GlobalCache, "dbmovietitles_title_slug_cache", func(elem database.DbstaticTwoStringOneInt) bool {
	// 	return strings.EqualFold(elem.Str1, title)
	// })
	// defer logger.Clear(tblaka)
	if config.SettingsGeneral.UseMediaCache {
		var imdb, nn string
		var haveyear int
		var found, found1 bool

		for idx := range database.CacheTitlesMovie {
			if !strings.EqualFold(database.CacheTitlesMovie[idx].Str1, *title) {
				continue
			}
			imdb, nn = "", ""
			haveyear = 0
			database.QueryDbmovieData(uint(database.CacheTitlesMovie[idx].Num), &haveyear, &imdb, &nn)
			found, found1 = checkifdbmovieyearmatches(haveyear, year)
			if found || found1 {
				return imdb, found, found1
			}
		}
	} else {
		foundrow := database.QueryIntColumn("select dbmovie_id FROM dbmovie_titles where title = ?", title)
		if foundrow != 0 {
			var imdb, nn string
			var haveyear int
			database.QueryDbmovieData(uint(foundrow), &haveyear, &imdb, &nn)
			found, found1 := checkifdbmovieyearmatches(haveyear, year)
			if found || found1 {
				return imdb, found, found1
			}
		}
	}

	// now check slugged
	slugged := logger.StringToSlug(*title)
	// tbl = cache.GetAllFunc(logger.GlobalCache, "dbmovies_cached", func(elem database.DbstaticThreeStringOneInt) bool {
	// 	return strings.EqualFold(elem.Str2, slugged)
	// })
	// defer logger.Clear(tbl)
	if config.SettingsGeneral.UseMediaCache {
		var found, found1 bool

		for idx := range database.CacheDBMovie {
			if !strings.EqualFold(database.CacheDBMovie[idx].Str2, slugged) {
				continue
			}
			found, found1 = checkifdbmovieyearmatches(database.QueryIntColumn(database.QueryDbmoviesGetYearByID, database.CacheDBMovie[idx].Num1), year)
			if found || found1 {
				return database.CacheDBMovie[idx].Str3, found, found1
			}
		}
	} else {
		foundrow := database.QueryIntColumn("select id FROM dbmovies where slug = ?", &slugged)
		if foundrow != 0 {
			var imdb, nn string
			var haveyear int
			database.QueryDbmovieData(uint(foundrow), &haveyear, &imdb, &nn)
			found, found1 := checkifdbmovieyearmatches(haveyear, year)
			if found || found1 {
				return imdb, found, found1
			}
		}
	}

	// tblaka = cache.GetAllFunc(logger.GlobalCache, "dbmovietitles_title_slug_cache", func(elem database.DbstaticTwoStringOneInt) bool {
	// 	return strings.EqualFold(elem.Str2, slugged)
	// })
	// defer logger.Clear(tblaka)
	if config.SettingsGeneral.UseMediaCache {
		var imdb, nn string
		var haveyear int
		var found, found1 bool
		for idx := range database.CacheTitlesMovie {
			if !strings.EqualFold(database.CacheTitlesMovie[idx].Str2, slugged) {
				continue
			}
			imdb, nn = "", ""
			haveyear = 0
			database.QueryDbmovieData(uint(database.CacheTitlesMovie[idx].Num), &haveyear, &imdb, &nn)
			found, found1 = checkifdbmovieyearmatches(haveyear, year)
			if found || found1 {
				return imdb, found, found1
			}
		}
	} else {
		foundrow := database.QueryIntColumn("select dbmovie_id FROM dbmovie_titles where slug = ?", &slugged)
		if foundrow != 0 {
			var imdb, nn string
			var haveyear int
			database.QueryDbmovieData(uint(foundrow), &haveyear, &imdb, &nn)
			found, found1 := checkifdbmovieyearmatches(haveyear, year)
			if found || found1 {
				return imdb, found, found1
			}
		}
	}

	if !addifnotfound {
		return "", false, false
	}

	tblprov := getsearchprovider(searchtype)
	var found, found1 bool
	var imdb string
	for idx := range *tblprov {
		switch (*tblprov)[idx] {
		case "imdb":
			if config.SettingsGeneral.MovieMetaSourceImdb {
				imdb, found, found1 = findimdbbytitle(title, &slugged, year)
			}
		case "tmdb":
			if config.SettingsGeneral.MovieMetaSourceTmdb {
				imdb, found, found1 = findtmdbbytitle(title, year)
			}
		case "omdb":
			if config.SettingsGeneral.MovieMetaSourceOmdb {
				imdb, found, found1 = findomdbbytitle(title, year)
			}
		default:
			continue
		}
		if found || found1 {
			//logger.LogAnyDebug("Find movie by title - found", logger.LoggerValue{Name: "imdb", Value: imdb}, logger.LoggerValue{Name: "title", Value: title}, logger.LoggerValue{Name: "year", Value: year})
			logger.Log.Debug().Str(logger.StrTitle, *title).Str(logger.StrImdb, imdb).Int(logger.StrYear, year).Msg("Find movie by title - found")
			logger.Clear(tblprov)
			return imdb, found, found1
		}
	}
	logger.Clear(tblprov)
	return "", false, false
}

func getsearchprovider(searchtype string) *[]string {
	if searchtype == logger.StrRss {
		if len(config.SettingsGeneral.MovieRSSMetaSourcePriority) >= 1 {
			return &config.SettingsGeneral.MovieRSSMetaSourcePriority
		}
	} else {
		if len(config.SettingsGeneral.MovieParseMetaSourcePriority) >= 1 {
			return &config.SettingsGeneral.MovieParseMetaSourcePriority
		}
	}
	return &defaultsearchprovider
}

func FindDbserieByName(title string) uint {
	var foundrow int
	if config.SettingsGeneral.UseMediaCache {
		ti := logger.IndexFunc(&database.CacheTitlesSeries, func(elem database.DbstaticTwoStringOneInt) bool { return strings.EqualFold(elem.Str1, title) })
		if ti != -1 {
			foundrow = database.CacheTitlesSeries[ti].Num
		}
	} else {
		foundrow = database.QueryIntColumn(database.QueryDbseriesGetIDByName, &title)
	}
	// foundrow := cache.GetFunc(logger.GlobalCache, "dbseries_title_slug_cache", func(elem database.DbstaticTwoStringOneInt) bool {
	// 	return strings.EqualFold(elem.Str1, title)
	// }).Num
	if foundrow != 0 {
		return uint(foundrow)
	}
	slugged := logger.StringToSlug(title)

	if config.SettingsGeneral.UseMediaCache {
		ti := logger.IndexFunc(&database.CacheTitlesSeries, func(elem database.DbstaticTwoStringOneInt) bool { return strings.EqualFold(elem.Str2, slugged) })
		if ti != -1 {
			return uint(database.CacheTitlesSeries[ti].Num)
		}
	} else {
		foundrow = database.QueryIntColumn(database.QueryDbseriesGetIDBySlug, &slugged)
		if foundrow != 0 {
			return uint(foundrow)
		}
	}
	// foundrow = cache.GetFunc(logger.GlobalCache, "dbseries_title_slug_cache", func(elem database.DbstaticTwoStringOneInt) bool {
	// 	return strings.EqualFold(elem.Str2, slugged)
	// }).Num
	// if foundrow != 0 {
	// 	return uint(foundrow)
	// }

	return database.QueryUintColumn(database.QueryDbserieAlternatesGetDBIDByTitleSlug, &title, &slugged)
}

func FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid uint, identifier *string, season string, episode string) uint {
	var id uint
	if season != "" && episode != "" {
		if season[:1] == "0" {
			season = strings.TrimLeft(season, "0")
		}
		if episode[:1] == "0" {
			episode = strings.TrimLeft(episode, "0")
		}
		id = database.QueryUintColumn(database.QueryDbserieEpisodesGetIDByDBIDSeasonEpisode, &dbserieid, &season, &episode)
		if id != 0 {
			return id
		}
	}
	if *identifier != "" {
		id = database.QueryUintColumn(database.QueryDbserieEpisodesGetIDByDBIDIdentifier, &dbserieid, identifier)
		if id != 0 {
			return id
		}
		if strings.ContainsRune(*identifier, '.') {
			id = database.QueryUintColumn(database.QueryDbserieEpisodesGetIDByDBIDIdentifier, &dbserieid, logger.StringReplaceRuneS(*identifier, '.', "-"))
			if id != 0 {
				return id
			}
		}
		if strings.ContainsRune(*identifier, ' ') {
			id = database.QueryUintColumn(database.QueryDbserieEpisodesGetIDByDBIDIdentifier, &dbserieid, logger.StringReplaceRuneS(*identifier, ' ', "-"))
			if id != 0 {
				return id
			}
		}
	}
	return 0
}
func GetEpisodeArray(identifiedby string, identifier *string) ([]string, error) {
	str1, str2 := config.RegexGetMatchesStr1Str2(true, &logger.StrRegexSeriesIdentifier, identifier)
	if str1 == "" && str2 == "" {
		return nil, errors.New("no identifier regex match")
	}

	if identifiedby == logger.StrDate {
		logger.StringReplaceRuneP(&str2, ' ', "-")
		logger.StringReplaceRuneP(&str2, '.', "-")
		return []string{str2}, nil
	}
	var splitby string
	if strings.ContainsRune(str1, 'E') {
		splitby = "E"
	} else if strings.ContainsRune(str1, 'e') {
		splitby = "e"
	} else if strings.ContainsRune(str1, 'X') {
		splitby = "X"
	} else if strings.ContainsRune(str1, 'x') {
		splitby = "x"
	} else if identifiedby != logger.StrDate && strings.ContainsRune(str1, '-') {
		splitby = "-"
	}
	if splitby != "" {
		strs := strings.Split(str1, splitby)
		if len(strs) >= 1 {
			if strs[0] == "" {
				strs = strs[1:]
			}
			if len(strs) == 1 && splitby != "-" {
				if strings.ContainsRune(strs[0], '-') {
					strs = strings.Split(strs[0], "-")
				}
			}
			for idx := range strs {
				strs[idx] = strings.Trim(strs[idx], "_-. ")
			}
		}
		return strs, nil
	}
	return nil, errors.New("nothing to split by")
}

func padNumberWithZero(value int) string {
	if value >= 10 {
		return logger.IntToString(value)
	}
	return "0" + logger.IntToString(value)
}
func JobImportDBSeries(serieconfig *config.SerieConfig, cfgpstr string, listname string, checkall bool, addnew bool) error {
	defer serieconfig.Close()
	if cfgpstr == "" {
		return logger.ErrCfgpNotFound
	}
	jobName := serieconfig.Name
	cfglist := config.GetMediaListsEntryIndex(cfgpstr, listname)
	if jobName == "" {
		jobName = config.SettingsMedia[cfgpstr].Lists[cfglist].Name
	}
	if jobName == "" {
		return errors.New("jobname missing")
	}

	if importJobRunning == jobName {
		return errors.New("already running")
	}
	importJobRunning = jobName
	if !addnew && (config.SettingsMedia[cfgpstr].Name == "" || listname == "") {
		listname = database.QueryStringColumn("select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", serieconfig.TvdbID)
		cfglist = config.GetMediaListsEntryIndex(cfgpstr, listname)
	}
	if config.SettingsMedia[cfgpstr].Name == "" || listname == "" {
		return logger.ErrCfgpNotFound
	}

	var err error
	var dbserie *database.Dbserie
	if serieconfig.TvdbID != 0 {
		dbserie, err = database.GetDbserieByID(database.QueryUintColumn(database.QueryDbseriesGetIDByTvdb, serieconfig.TvdbID))
		if err != nil && !addnew {
			return errors.New("adding not allowed")
		}
	} else {
		dbserie = &database.Dbserie{}
	}
	defer logger.ClearVar(&dbserie)
	var dbserieadded bool
	if dbserie.Seriename == "" {
		dbserie.Seriename = serieconfig.Name
	}
	if dbserie.Identifiedby == "" {
		dbserie.Identifiedby = serieconfig.Identifiedby
	}
	//defer dbserie.Close()

	if strings.EqualFold(serieconfig.Source, "none") && addnew {
		dbserieadded, err = dbserieNone(serieconfig, dbserie)
		if err != nil {
			return err
		}
	}
	if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, logger.StrTvdb) {
		err = refreshtvdb(dbserie, dbserieadded, serieconfig, cfgpstr, addnew, checkall)
		if err != nil {
			return err
		}
	}

	if dbserie.ID == 0 {
		return errors.New("dbid not found")
	}

	if addnew {
		//Add Entry in SeriesTable

		if listname == "" {
			return errors.New("listname empty")
		}
		if config.SettingsMedia[cfgpstr].Lists[cfglist].Name == "" {
			return errors.New("serie list empty")
		}

		tbl := database.QueryStaticColumnsOneStringOneInt(false, database.QueryIntColumn("select count() from series where dbserie_id = ?", dbserie.ID), "select lower(listname), id from series where dbserie_id = ?", dbserie.ID)
		defer logger.Clear(tbl)
		for idx := range *tbl {
			if logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[cfglist].IgnoreMapLists, (*tbl)[idx].Str) {
				return errors.New("series skip2 for")
			}

			if logger.ContainsStringsI(&config.SettingsMedia[cfgpstr].Lists[cfglist].ReplaceMapLists, (*tbl)[idx].Str) {
				database.UpdateColumnStatic("Update series SET listname = ?, dbserie_id = ? where id = ?", config.SettingsMedia[cfgpstr].Lists[cfglist].Name, dbserie.ID, (*tbl)[idx].Num)
			}
		}

		//var serie database.Serie

		if database.QueryIntColumn("select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE", &dbserie.ID, &config.SettingsMedia[cfgpstr].Lists[cfglist].Name) == 0 {
			logger.Log.Debug().Str(logger.StrListname, config.SettingsMedia[cfgpstr].Lists[cfglist].Name).Int(logger.StrTvdb, serieconfig.TvdbID).Msg("Add series for")
			_, err := database.InsertStatic("Insert into series (dbserie_id, listname, rootpath, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?)", dbserie.ID, config.SettingsMedia[cfgpstr].Lists[cfglist].Name, serieconfig.Target, serieconfig.SearchSpecials, serieconfig.DontSearch, serieconfig.DontUpgrade)
			if err != nil {
				return err
			}
		}
	}

	dbseries := database.QueryStaticIntArray(database.QueryIntColumn("select count() from dbserie_episodes where dbserie_id = ?", dbserie.ID), database.QueryDbserieEpisodesGetIDByDBID, dbserie.ID)
	episodes := database.QueryStaticColumnsTwoInt(true,
		database.QueryIntColumn("select count() from serie_episodes where dbserie_id = ?", &dbserie.ID),
		database.QuerySerieEpisodesGetDBEpisodeIDSerieIDByDBID, &dbserie.ID)

	tblseries := database.QueryStaticIntArray(database.QueryIntColumn("select count() from series where dbserie_id = ?", dbserie.ID), database.QuerySeriesGetIDByDBID, dbserie.ID)
	var cont bool
	var quality string
	for idxserie := range *tblseries {
		quality = database.QueryStringColumn("select quality_profile from serie_episodes where serie_id = ? limit 1", (*tblseries)[idxserie])
		if quality == "" {
			quality = config.SettingsMedia[cfgpstr].TemplateQuality
		}
		for dbidx := range *dbseries {
			cont = false
			for idxi := range *episodes {
				if (*episodes)[idxi].Num2 == (*tblseries)[idxserie] && (*episodes)[idxi].Num1 == (*dbseries)[dbidx] {
					cont = true
					break
				}
			}
			if !cont {
				database.InsertStatic("Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, ?, ?, ?)", dbserie.ID, (*tblseries)[idxserie], true, quality, (*dbseries)[dbidx])
			}
			//if !logger.ContainsFunc(&episodes, func(e database.DbstaticTwoInt) bool {
			//	return e.Num2 == tblseries[idxserie] && e.Num1 == dbseries[dbidx]
			//}) {
			//database.InsertStatic("Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, ?, ?, ?)", dbserie.ID, tblseries[idxserie], true, quality, dbseries[dbidx])
			//}
		}
	}
	logger.Clear(tblseries)
	logger.Clear(dbseries)
	logger.Clear(episodes)
	return nil
}

func refreshtvdb(dbserie *database.Dbserie, dbserieadded bool, serieconfig *config.SerieConfig, cfgpstr string, addnew bool, checkall bool) error {
	var err error
	if dbserie.ID == 0 {
		dbserieadded, err = dbserieTVDB(serieconfig, dbserie, addnew)
		if err != nil {
			return err
		}
	}
	if dbserie.ID != 0 && (dbserieadded || !addnew) {
		//Update Metadata
		dbserieTVDBTitles(serieconfig, dbserie, cfgpstr, addnew)

		if (checkall || dbserieadded || !addnew) && (serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, logger.StrTvdb)) {
			logger.Log.Debug().Int(logger.StrTvdb, serieconfig.TvdbID).Msg("Get episodes for")
			//logger.LogAnyDebug("get episodes", logger.LoggerValue{Name: "tvdb", Value: dbserie.ThetvdbID})

			if dbserie.ThetvdbID != 0 {
				tvdbdetails, err := apiexternal.TvdbAPI.GetSeriesEpisodes(dbserie.ThetvdbID, config.SettingsMedia[cfgpstr].MetadataLanguage)

				if err == nil && len(tvdbdetails.Data) >= 1 {
					tbl := database.QueryStaticColumnsTwoString(false, database.QueryIntColumn("select count() from dbserie_episodes where dbserie_id = ?", dbserie.ID), database.QueryDbserieEpisodesGetSeasonEpisodeByDBID, dbserie.ID)
					var cont bool
					var strepisode, strseason string
					for idx := range tvdbdetails.Data {
						strepisode = logger.IntToString(tvdbdetails.Data[idx].AiredEpisodeNumber)
						strseason = logger.IntToString(tvdbdetails.Data[idx].AiredSeason)

						cont = false
						for idxi := range *tbl {
							if strings.EqualFold((*tbl)[idxi].Str1, strseason) && strings.EqualFold((*tbl)[idxi].Str2, strepisode) {
								cont = true
								break
							}
						}
						if cont {
							continue
						}
						//if logger.ContainsFunc(&tbl, func(c database.DbstaticTwoString) bool {
						//	return strings.EqualFold(c.Str1, strseason) && strings.EqualFold(c.Str2, strepisode)
						//}) {
						//	continue
						//}
						database.InsertStatic("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
							strepisode, strseason, "S"+padNumberWithZero(tvdbdetails.Data[idx].AiredSeason)+"E"+padNumberWithZero(tvdbdetails.Data[idx].AiredEpisodeNumber), tvdbdetails.Data[idx].EpisodeName, database.ParseDate(tvdbdetails.Data[idx].FirstAired), tvdbdetails.Data[idx].Overview, tvdbdetails.Data[idx].Poster, dbserie.ID)

					}
					logger.Clear(tbl)
				} else {
					//logger.LogAnyDebug("get tvdb episodes failed", logger.LoggerValue{Name: "tvdb", Value: dbserie.ThetvdbID})
					logger.Log.Debug().Int(logger.StrTvdb, dbserie.ThetvdbID).Msg("Serie tvdb episodes not found for")
				}
				tvdbdetails.Close()
			}
			if config.SettingsGeneral.SerieMetaSourceTrakt && dbserie.ImdbID != "" {
				seasons, err := apiexternal.TraktAPI.GetSerieSeasons(dbserie.ImdbID)
				if err == nil && seasons != nil && len(*seasons) >= 1 {
					//var identifier string
					tbl := database.QueryStaticColumnsTwoString(false, database.QueryIntColumn("select count() from dbserie_episodes where dbserie_id = ?", dbserie.ID), database.QueryDbserieEpisodesGetSeasonEpisodeByDBID, dbserie.ID)

					var episodes *[]apiexternal.TraktSerieSeasonEpisodes
					var cont bool
					var strepisode, strseason string
					for idxseason := range *seasons {
						episodes, err = apiexternal.TraktAPI.GetSerieSeasonEpisodes(dbserie.ImdbID, (*seasons)[idxseason].Number)
						if err == nil {
							for idxepi := range *episodes {
								strepisode = logger.IntToString((*episodes)[idxepi].Episode)
								strseason = logger.IntToString((*episodes)[idxepi].Season)

								cont = false
								for idxi := range *tbl {
									if strings.EqualFold((*tbl)[idxi].Str1, strseason) && strings.EqualFold((*tbl)[idxi].Str2, strepisode) {
										cont = true
										break
									}
								}
								if cont {
									continue
								}
								//if logger.ContainsFunc(&tbl, func(c database.DbstaticTwoString) bool {
								//	return strings.EqualFold(c.Str1, strseason) && strings.EqualFold(c.Str2, strepisode)
								//}) {
								//	continue
								//}
								database.InsertStatic("insert into dbserie_episodes (episode, season, identifier, title, first_aired, overview, runtime, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
									strepisode, strseason, "S"+padNumberWithZero((*episodes)[idxepi].Season)+"E"+padNumberWithZero((*episodes)[idxepi].Episode), (*episodes)[idxepi].Title, database.TimeToSQLTime((*episodes)[idxepi].FirstAired, true), (*episodes)[idxepi].Overview, (*episodes)[idxepi].Runtime, dbserie.ID)
								//else {
								// 	if episodes.Episodes[idxepi].Title != "" {
								// 		UpdateColumnStatic("update dbserie_episodes set title = ? where id = ? and title = ''", episodes.Episodes[idxepi].Title, counter)
								// 	}
								// 	if !episodes.Episodes[idxepi].FirstAired.IsZero() {
								// 		UpdateColumnStatic("update dbserie_episodes set first_aired = ? where id = ? and first_aired is null", sql.NullTime{Time: episodes.Episodes[idxepi].FirstAired, Valid: true}, counter)
								// 	}
								// 	if episodes.Episodes[idxepi].Overview != "" {
								// 		UpdateColumnStatic("update dbserie_episodes set overview = ? where id = ? and overview = ''", episodes.Episodes[idxepi].Overview, counter)
								// 	}
								// 	if episodes.Episodes[idxepi].Runtime != 0 {
								// 		UpdateColumnStatic("update dbserie_episodes set runtime = ? where id = ? and Runtime = 0", episodes.Episodes[idxepi].Runtime, counter)
								// 	}
								// }
							}
						} else {
							//logger.LogAnyDebug("get trakt episodes failed", logger.LoggerValue{Name: "imdb", Value: dbserie.ImdbID}, logger.LoggerValue{Name: "season", Value: (*seasons)[idxseason].Number})
							logger.Log.Debug().Str(logger.StrImdb, dbserie.ImdbID).Int("season", (*seasons)[idxseason].Number).Msg("Serie trakt episodes not found for")
						}
						logger.Clear(episodes)
					}
					logger.Clear(tbl)
				} else {
					//logger.LogAnyDebug("get trakt episodes failed", logger.LoggerValue{Name: "tvdb", Value: dbserie.ThetvdbID})
					logger.Log.Info().Str(logger.StrImdb, dbserie.ImdbID).Msg("Serie trakt seasons not found for")
				}
				logger.Clear(seasons)
			}
		}
	}
	return nil
}

func dbserieNone(serieconfig *config.SerieConfig, dbserie *database.Dbserie) (bool, error) {
	counter := database.QueryIntColumn("select count() from dbseries where seriename = ? COLLATE NOCASE", &serieconfig.Name)
	if counter == 0 {
		inres, err := database.InsertStatic("insert into dbseries (seriename, aliases, season, status, firstaired, network, runtime, language, genre, overview, rating, siterating, siterating_count, slug, trakt_id, imdb_id, thetvdb_id, freebase_m_id, freebase_id, tvrage_id, facebook, instagram, twitter, banner, poster, fanart, identifiedby) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			dbserie.Seriename, dbserie.Aliases, dbserie.Season, dbserie.Status, dbserie.Firstaired, dbserie.Network, dbserie.Runtime, dbserie.Language, dbserie.Genre, dbserie.Overview, dbserie.Rating, dbserie.Siterating, dbserie.SiteratingCount, dbserie.Slug, dbserie.TraktID, dbserie.ImdbID, dbserie.ThetvdbID, dbserie.FreebaseMID, dbserie.FreebaseID, dbserie.TvrageID, dbserie.Facebook, dbserie.Instagram, dbserie.Twitter, dbserie.Banner, dbserie.Poster, dbserie.Fanart, dbserie.Identifiedby)
		if err != nil {
			return false, err
		}
		dbserie.ID = uint(database.InsertRetID(inres))

		if config.SettingsGeneral.UseMediaCache {
			database.CacheTitlesSeries = append(database.CacheTitlesSeries, database.DbstaticTwoStringOneInt{Str1: dbserie.Seriename, Str2: logger.StringToSlug(dbserie.Seriename), Num: int(dbserie.ID)})
		}
		//cache.Append(logger.GlobalCache, "dbseries_title_slug_cache", database.DbstaticTwoStringOneInt{Str1: dbserie.Seriename, Str2: logger.StringToSlug(dbserie.Seriename), Num: int(dbserie.ID)})

		serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
		serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)
		var counter int
		for idxalt := range serieconfig.AlternateName {
			if serieconfig.AlternateName[idxalt] == "" {
				continue
			}
			counter = database.QueryIntColumn("select count() from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE", dbserie.ID, &serieconfig.AlternateName[idxalt])
			if counter == 0 {
				continue
			}
			if counter == 0 {
				database.InsertStatic("Insert into dbserie_alternates (title, slug, dbserie_id) values (?, ?, ?)", &serieconfig.AlternateName[idxalt], logger.StringToSlug(serieconfig.AlternateName[idxalt]), dbserie.ID)
			}
		}
		return true, nil
	} else {
		dbserie.ID = database.QueryUintColumn(database.QueryDbseriesGetIDByName, &serieconfig.Name)
		if dbserie.ID == 0 {
			return false, errors.New("id not fetched")
		}
	}
	return false, nil
}

func dbserieTVDB(serieconfig *config.SerieConfig, dbserie *database.Dbserie, addnew bool) (bool, error) {
	dbserie.ThetvdbID = serieconfig.TvdbID
	counter := database.QueryIntColumn("select count() from dbseries where thetvdb_id = ?", serieconfig.TvdbID)
	if counter == 0 && addnew {
		if !config.Check("imdb") {
			return false, errors.New("imdb config missing")
		}
		logger.Log.Debug().Int(logger.StrTvdb, serieconfig.TvdbID).Msg("Insert dbseries for")
		inres, err := database.InsertStatic("insert into dbseries (seriename, thetvdb_id, identifiedby) values (?, ?, ?)", dbserie.Seriename, dbserie.ThetvdbID, dbserie.Identifiedby)
		if err != nil {
			return false, err
		}
		dbserie.ID = uint(database.InsertRetID(inres))

		if config.SettingsGeneral.UseMediaCache {
			database.CacheTitlesSeries = append(database.CacheTitlesSeries, database.DbstaticTwoStringOneInt{Str1: dbserie.Seriename, Str2: logger.StringToSlug(dbserie.Seriename), Num: int(dbserie.ID)})
		}
		//cache.Append(logger.GlobalCache, "dbseries_title_slug_cache", database.DbstaticTwoStringOneInt{Str1: dbserie.Seriename, Str2: logger.StringToSlug(dbserie.Seriename), Num: int(dbserie.ID)})
		return true, nil
	}
	return false, nil
}

func dbserieTVDBTitles(serieconfig *config.SerieConfig, dbserie *database.Dbserie, cfgpstr string, addnew bool) {
	logger.Log.Debug().Int(logger.StrTvdb, serieconfig.TvdbID).Msg("Get metadata for")
	//logger.LogAnyDebug("get tvdb aliases", logger.LoggerValue{Name: "tvdb", Value: dbserie.ThetvdbID})
	addaliases, _ := metadata.SerieGetMetadata(dbserie, config.SettingsMedia[cfgpstr].MetadataLanguage, config.SettingsGeneral.SerieMetaSourceTmdb, config.SettingsGeneral.SerieMetaSourceTrakt, !addnew, true)
	if dbserie.Seriename == "" {
		addaliases.Close()
		addaliases, _ = metadata.SerieGetMetadata(dbserie, "", config.SettingsGeneral.SerieMetaSourceTmdb, config.SettingsGeneral.SerieMetaSourceTrakt, !addnew, true)
	}
	serieconfig.AlternateName = append(serieconfig.AlternateName, addaliases.Data.Aliases...)
	serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
	serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)

	addaliases.Close()

	database.UpdateColumnStatic(database.QueryupdateseriesStatic,
		&dbserie.Seriename, &dbserie.Aliases, &dbserie.Season, &dbserie.Status, &dbserie.Firstaired, &dbserie.Network, &dbserie.Runtime, &dbserie.Language, &dbserie.Genre, &dbserie.Overview, &dbserie.Rating, &dbserie.Siterating, &dbserie.SiteratingCount, &dbserie.Slug, &dbserie.TraktID, &dbserie.ImdbID, &dbserie.ThetvdbID, &dbserie.FreebaseMID, &dbserie.FreebaseID, &dbserie.TvrageID, &dbserie.Facebook, &dbserie.Instagram, &dbserie.Twitter, &dbserie.Banner, &dbserie.Poster, &dbserie.Fanart, &dbserie.Identifiedby, &dbserie.ID)

	titles := database.QueryStaticColumnsOneStringOneInt(false, database.QueryCountColumn("dbserie_alternates", "dbserie_id = ?", dbserie.ID), "select title, id from dbserie_alternates where dbserie_id = ?", dbserie.ID)

	count := database.QueryIntColumn("select count() from dbserie_alternates where dbserie_id = ?", dbserie.ID)
	if count == 0 {
		count = 15
	}

	titlegroup := make([]database.DbserieAlternate, 0, count)
	//var regionok bool
	lenarr := len(config.SettingsMedia[cfgpstr].MetadataTitleLanguages)
	if config.SettingsGeneral.SerieAlternateTitleMetaSourceImdb && dbserie.ImdbID != "" {
		tblaka := database.QueryImdbAka(database.Querywithargs{Where: database.FilterByTconst}, logger.AddImdbPrefix(dbserie.ImdbID))
		var cont bool
		for idxaka := range *tblaka {
			cont = false
			for idxi := range *titles {
				if strings.EqualFold((*titles)[idxi].Str, (*tblaka)[idxaka].Title) {
					cont = true
					break
				}
			}
			if cont {
				continue
			}
			//if logger.ContainsFunc(&titles, func(c database.DbstaticOneStringOneInt) bool {
			//	return strings.EqualFold(c.Str, tblaka[idxaka].Title)
			//}) {
			//	continue
			//}
			if lenarr == 0 {
				titlegroup = append(titlegroup, database.DbserieAlternate{DbserieID: dbserie.ID, Title: (*tblaka)[idxaka].Title, Slug: (*tblaka)[idxaka].Slug, Region: (*tblaka)[idxaka].Region})
			} else {
				for idxq := range config.SettingsMedia[cfgpstr].MetadataTitleLanguages {
					if strings.EqualFold(config.SettingsMedia[cfgpstr].MetadataTitleLanguages[idxq], (*tblaka)[idxaka].Region) {
						titlegroup = append(titlegroup, database.DbserieAlternate{DbserieID: dbserie.ID, Title: (*tblaka)[idxaka].Title, Slug: (*tblaka)[idxaka].Slug, Region: (*tblaka)[idxaka].Region})
						break
					}
				}
			}
		}
		logger.Clear(tblaka)
	}
	if config.SettingsGeneral.SerieAlternateTitleMetaSourceTrakt && (dbserie.TraktID != 0 || dbserie.ImdbID != "") {
		queryid := dbserie.ImdbID
		if dbserie.TraktID != 0 {
			queryid = logger.IntToString(dbserie.TraktID)
		}
		traktaliases, err := apiexternal.TraktAPI.GetSerieAliases(queryid)
		if err == nil && traktaliases != nil && len(*traktaliases) >= 1 {
			//logger.Grow(&titlegroup, len(traktaliases))
			//titlegroup = logger.GrowSliceBy(titlegroup, len(traktaliases.Aliases))
			var cont bool
			for idxalias := range *traktaliases {
				cont = false
				for idxi := range titlegroup {
					if strings.EqualFold(titlegroup[idxi].Title, (*traktaliases)[idxalias].Title) {
						cont = true
						break
					}
				}
				if cont {
					continue
				}
				//if logger.ContainsFunc(&titlegroup, func(c database.DbserieAlternate) bool {
				//	return strings.EqualFold(c.Title, traktaliases[idxalias].Title)
				//}) {
				//	continue
				//}

				cont = false
				for idxi := range *titles {
					if strings.EqualFold((*titles)[idxi].Str, (*traktaliases)[idxalias].Title) {
						cont = true
						break
					}
				}
				if cont {
					continue
				}
				//if logger.ContainsFunc(&titles, func(c database.DbstaticOneStringOneInt) bool {
				//	return strings.EqualFold(c.Str, traktaliases[idxalias].Title)
				//}) {
				//	continue
				//}
				if lenarr == 0 {
					titlegroup = append(titlegroup, database.DbserieAlternate{DbserieID: dbserie.ID, Title: (*traktaliases)[idxalias].Title, Slug: logger.StringToSlug((*traktaliases)[idxalias].Title), Region: (*traktaliases)[idxalias].Country})
				} else {
					for idxq := range config.SettingsMedia[cfgpstr].MetadataTitleLanguages {
						if strings.EqualFold(config.SettingsMedia[cfgpstr].MetadataTitleLanguages[idxq], (*traktaliases)[idxalias].Country) {
							titlegroup = append(titlegroup, database.DbserieAlternate{DbserieID: dbserie.ID, Title: (*traktaliases)[idxalias].Title, Slug: logger.StringToSlug((*traktaliases)[idxalias].Title), Region: (*traktaliases)[idxalias].Country})
							break
						}
					}
				}
			}
		} else {
			//logger.LogAnyDebug("get trakt aliases not found", logger.LoggerValue{Name: "tvdb", Value: dbserie.ThetvdbID})
			logger.Log.Debug().Int(logger.StrTvdb, dbserie.ThetvdbID).Msg("Serie trakt aliases not found for")
		}

		logger.Clear(traktaliases)
	}

	var cont bool
	for idxadd := range serieconfig.AlternateName {
		cont = false
		for idxi := range titlegroup {
			if strings.EqualFold(titlegroup[idxi].Title, serieconfig.AlternateName[idxadd]) {
				cont = true
				break
			}
		}
		if !cont {
			titlegroup = append(titlegroup, database.DbserieAlternate{Title: serieconfig.AlternateName[idxadd]})
		}
		//if !logger.ContainsFunc(&titlegroup, func(c database.DbserieAlternate) bool {
		//	return strings.EqualFold(c.Title, serieconfig.AlternateName[idxadd])
		//}) {
		//	titlegroup = append(titlegroup, database.DbserieAlternate{Title: serieconfig.AlternateName[idxadd]})
		//}
	}
	if len(titlegroup) < 10 {
		titlegroup = titlegroup[:len(titlegroup):len(titlegroup)]
	}
	for idxalt := range titlegroup {
		if titlegroup[idxalt].Title == "" {
			continue
		}
		cont = false
		for idxi := range *titles {
			if strings.EqualFold((*titles)[idxi].Str, titlegroup[idxalt].Title) {
				cont = true
				break
			}
		}
		if !cont {
			database.InsertStatic("Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)", titlegroup[idxalt].Title, titlegroup[idxalt].Slug, dbserie.ID, titlegroup[idxalt].Region)
		}
		//if !logger.ContainsFunc(&titles, func(c database.DbstaticOneStringOneInt) bool {
		//	return strings.EqualFold(c.Str, titlegroup[idxalt].Title)
		//}) {
		//database.InsertStatic("Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)", titlegroup[idxalt].Title, titlegroup[idxalt].Slug, dbserie.ID, titlegroup[idxalt].Region)
		//}
	}
	logger.Clear(&titlegroup)
	logger.Clear(titles)
}

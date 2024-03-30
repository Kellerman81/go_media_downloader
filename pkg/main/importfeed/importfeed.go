package importfeed

import (
	"errors"
	"strings"
	"sync"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
)

var importJobRunning = sync.Map{}

//var importJobRunning string

// checkimdbyearsingle checks if the year from IMDB matches the year
// in the FileParser, allowing a 1 year difference. It returns true if
// the years match or are within 1 year difference.
func checkimdbyearsingle(m *apiexternal.FileParser, imdb *string, haveyear int) bool {
	if haveyear == 0 || m.M.Year == 0 || imdb == nil {
		return false
	}
	if haveyear == m.M.Year {
		return true
	}

	if haveyear == m.M.Year+1 || haveyear == m.M.Year-1 {
		id := MovieFindDBIDByImdb(imdb)
		if database.GetMediaQualityConfig(m.Cfgp, &id).CheckYear1 {
			return true
		}
	}
	return false
}

// StripTitlePrefixPostfixGetQual strips any configured prefix or suffix from the
// title in the provided ParseInfo, using the provided QualityConfig. It modifies
// the Title field on the ParseInfo in place.
func StripTitlePrefixPostfixGetQual(m *database.ParseInfo, quality *config.QualityConfig) {
	if m.Title == "" {
		return
	}

	for idx := range quality.TitleStripSuffixForSearch {
		if !logger.ContainsI(m.Title, quality.TitleStripSuffixForSearch[idx]) {
			continue
		}
		idx2 := logger.IndexI(m.Title, quality.TitleStripSuffixForSearch[idx])
		if idx2 != -1 {
			if m.Title[:idx2] != "" {
				str2 := m.Title[:idx2]
				switch str2[len(str2)-1:] {
				case "-", ".", " ":
					m.Title = strings.TrimRight(str2, "-. ")
				}
			}
		}
	}
	for idx := range quality.TitleStripPrefixForSearch {
		if !logger.HasPrefixI(m.Title, quality.TitleStripPrefixForSearch[idx]) {
			continue
		}
		idx2 := logger.IndexI(m.Title, quality.TitleStripPrefixForSearch[idx])
		if idx2 != -1 {
			str2 := m.Title[idx2+len(quality.TitleStripPrefixForSearch[idx]):]
			if str2 != "" {
				switch str2[0] {
				case '-', '.', ' ':
					m.Title = strings.TrimLeft(str2, "-. ")
				}
			}
		}
	}
}

// MovieFindDBIDByImdb looks up the database ID for a movie by its IMDB ID.
// It takes a string containing the IMDB ID and returns the uint database ID.
// It first checks the cache if enabled, otherwise queries the database directly.
// If no match is found, it returns 0.
func MovieFindDBIDByImdb(imdb *string) uint {
	if imdb == nil || *imdb == "" {
		return 0
	}
	if config.SettingsGeneral.UseMediaCache {
		return database.CacheThreeStringIntIndexFunc(logger.CacheDBMovie, imdb)
	}
	return database.GetdatarowN[uint](false, "select id from dbmovies where imdb_id = ?", &imdb)
}

// moviegetimdbtitle checks if the found dbid of a movie has the correct year (also gets imdb)
// It takes a dbid and a ParseInfo struct pointer.
// It checks the dbid against the database, compares the year to the ParseInfo Year field,
// and returns a bool indicating if it matches an IMDB entry.
func moviegetimdbtitle(dbid int, m *database.ParseInfo) bool {
	if config.SettingsGeneral.UseMediaCache {
		year := database.CacheThreeStringIntIndexFuncGetYear(logger.CacheDBMovie, dbid)
		if year != 0 {
			if m.Year == 0 {
				return false
			}
			if m.Year == year || m.Year == year+1 || m.Year == year-1 {
				return true
			}
		}
		return false
	}
	year := database.GetdatarowN[int](false, "select year from dbmovies where id = ?", &dbid)
	if year == 0 {
		return false
	}
	if year != 0 && m.Year == 0 {
		return false
	}
	if m.Year == year || m.Year == year+1 || m.Year == year-1 {
		return true
	}
	return false
}

// moviegetimdbtitleparser checks if the found dbid of a movie has the correct year (also gets imdb)
// It takes a ParseInfo struct pointer, checks the DbmovieID field against the database,
// compares the year to the Year field, and returns a bool indicating if it matches an IMDB entry.
func moviegetimdbtitleparser(m *database.ParseInfo) bool {
	if config.SettingsGeneral.UseMediaCache {
		intdb := int(m.DbmovieID)
		year := database.CacheThreeStringIntIndexFuncGetYear(logger.CacheDBMovie, intdb)
		if year == 0 {
			return false
		}
		if year != 0 && m.Year == 0 {
			return false
		}
		if m.Year == year || m.Year == year+1 || m.Year == year-1 {
			return true
		}
		return false
	}
	year := database.GetdatarowN[int](false, "select year from dbmovies where id = ?", &m.DbmovieID)
	if year == 0 {
		return false
	}
	if year != 0 && m.Year == 0 {
		return false
	}
	if m.Year == year || m.Year == year+1 || m.Year == year-1 {
		return true
	}
	return false
}

// MovieFindImdbIDByTitle searches for a movie's IMDB ID by its title.
// It first searches the database and caches by title and slugified title.
// If not found, it can search external APIs based on config settings.
// It populates the ParseInfo struct with the found IMDB ID and other data.
func MovieFindImdbIDByTitle(addifnotfound bool, m *apiexternal.FileParser) {
	if m.M.Title == "" {
		cleanimdbdbmovie(m)
		return
	}

	findmoviedbidbytitle(&m.M)
	if m.M.DbmovieID == 0 {
		findmoviedbidbytitleslug(&m.M)
	}
	if m.M.DbmovieID != 0 {
		if config.SettingsGeneral.UseMediaCache {
			database.CacheThreeStringIntIndexFuncGetImdb(logger.CacheDBMovie, int(m.M.DbmovieID), &m.M)
			return
		}
		_ = database.ScanrowsNdyn(false, "select imdb_id from dbmovies where id = ?", &m.M.Imdb, &m.M.DbmovieID)
		return
	}

	if !addifnotfound {
		cleanimdbdbmovie(m)
		return
	}
	arrp := getsearchprovider(false)
	slug := logger.StringToSlug(m.M.Title)
	if slug == "" {
		slug = "please-fix-title88vv77b9gg"
	}
	for idxp := range arrp {
	breakswitch:
		switch arrp[idxp] {
		case logger.StrImdb:
			if config.SettingsGeneral.MovieMetaSourceImdb {
				arr := database.GetrowsN[database.DbstaticOneStringOneInt](true, database.GetdatarowN[int](true, "select count() from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", &m.M.Title, &m.M.Title, &slug), "select tconst,start_year from imdb_titles where (primary_title = ? COLLATE NOCASE or original_title = ? COLLATE NOCASE or slug = ?)", &m.M.Title, &m.M.Title, &slug)
				for idx := range arr {
					if checkimdbyearsingle(m, &arr[idx].Str, arr[idx].Num) {
						m.M.Imdb = arr[idx].Str
						m.M.DbmovieID = uint(arr[idx].Num)
						clear(arr)
						break breakswitch
					}
				}
				clear(arr)

				var haveyear int

				arr2 := database.GetrowsN[string](true, database.QueryImdbAkaCountByTitleSlug(&m.M.Title, &slug), "select distinct tconst from imdb_akas where title = ? COLLATE NOCASE or slug = ?", &m.M.Title, &slug)
				for idx := range arr2 {
					_ = database.ScanrowsNdyn(true, "select start_year from imdb_titles where tconst = ?", &haveyear, &arr2[idx])
					if checkimdbyearsingle(m, &arr2[idx], haveyear) {
						m.M.Imdb = arr2[idx]
						m.M.DbmovieID = MovieFindDBIDByImdb(&arr2[idx])
						clear(arr2)
						break breakswitch
					}
				}
				cleanimdbdbmovie(m)
				clear(arr2)
			}
		case "tmdb":
			if config.SettingsGeneral.MovieMetaSourceTmdb {
				if m.M.Title == "" {
					cleanimdbdbmovie(m)
					break breakswitch
				}

				tbl, err := apiexternal.SearchTmdbMovie(m.M.Title)
				if err != nil {
					cleanimdbdbmovie(m)
					break breakswitch
				}
				var haveyear int
				for idx := range tbl {
					_ = database.ScanrowsNdyn(false, "select imdb_id from dbmovies where moviedb_id = ?", &m.M.Imdb, &tbl[idx].ID)
					if m.M.Imdb == "" {
						moviedbexternal, err := apiexternal.GetTmdbMovieExternal(tbl[idx].ID)
						if err != nil {
							continue
						}
						m.M.Imdb = moviedbexternal.ImdbID
						_ = database.ScanrowsNdyn(false, "select id from dbmovies where imdb_id = ?", &m.M.DbmovieID, &m.M.Imdb)
						moviedbexternal.Close()
						clear(tbl)
						break breakswitch
					}
					if m.M.Imdb == "" {
						continue
					}
					_ = database.ScanrowsNdyn(true, "select start_year from imdb_titles where tconst = ?", &haveyear, &m.M.Imdb)
					if checkimdbyearsingle(m, &m.M.Imdb, haveyear) {
						_ = database.ScanrowsNdyn(false, "select id from dbmovies where imdb_id = ?", &m.M.DbmovieID, &m.M.Imdb)

						clear(tbl)
						break breakswitch
					}
				}
				cleanimdbdbmovie(m)
				clear(tbl)
			}
		case "omdb":
			if config.SettingsGeneral.MovieMetaSourceOmdb {
				tbl, err := apiexternal.SearchOmdbMovie(m.M.Title, "")
				if err != nil {
					cleanimdbdbmovie(m)
					clear(tbl)
					break breakswitch
				}
				for idx := range tbl {
					if checkimdbyearsingle(m, &tbl[idx].ImdbID, logger.StringToInt(tbl[idx].Year)) {
						m.M.Imdb = tbl[idx].ImdbID
						_ = database.ScanrowsNdyn(false, "select id from dbmovies where imdb_id = ?", &m.M.DbmovieID, &tbl[idx].ImdbID)
						clear(tbl)
						break breakswitch
					}
				}
				cleanimdbdbmovie(m)
				clear(tbl)
			}
		default:
			continue
		}
		if m.M.Imdb != "" {
			return
		}
	}
	cleanimdbdbmovie(m)
}

// cleanimdbdbmovie clears the Imdb and DbmovieID fields in the FileParser struct to empty values.
// This is used to reset the state when a lookup fails.
func cleanimdbdbmovie(m *apiexternal.FileParser) {
	m.M.Imdb = ""
	m.M.DbmovieID = 0
}

// findmoviedbidbytitle searches for a movie in the database by its title.
// It first checks the title cache, then does a database query on dbmovies by title,
// followed by a query on dbmovie_titles by title if needed.
// It attempts to find the movie's id and populate ParseInfo.DbmovieID with it.
func findmoviedbidbytitle(m *database.ParseInfo) {
	if config.SettingsGeneral.UseMediaCache {
		getdbmovieidbytitleincache(m, m.Title)
		return
	}
	_ = database.ScanrowsNdyn(false, "select id from dbmovies where title = ? COLLATE NOCASE", &m.DbmovieID, &m.Title)
	if m.DbmovieID != 0 && moviegetimdbtitleparser(m) {
		return
	}
	_ = database.ScanrowsNdyn(false, "select dbmovie_id from dbmovie_titles where title = ? COLLATE NOCASE", &m.DbmovieID, &m.Title)
	if m.DbmovieID != 0 && !moviegetimdbtitleparser(m) {
		m.DbmovieID = 0
	}
}

// findmoviedbidbytitleslug searches for a movie in the database by its sluggified title.
// It first checks the title cache, then does a database query on dbmovies by slug,
// followed by a query on dbmovie_titles by slug if needed.
// It attempts to find the movie's id and populate ParseInfo.DbmovieID with it.
func findmoviedbidbytitleslug(m *database.ParseInfo) {
	if config.SettingsGeneral.UseMediaCache {
		getdbmovieidbytitleincache(m, logger.StringToSlug(m.Title))
		return
	}
	title := logger.StringToSlug(m.Title)
	if title == "" {
		return
	}
	_ = database.ScanrowsNdyn(false, "select id from dbmovies where slug = ?", &m.DbmovieID, &title)
	if m.DbmovieID != 0 && moviegetimdbtitleparser(m) {
		return
	}
	_ = database.ScanrowsNdyn(false, "select dbmovie_id from dbmovie_titles where slug = ?", &m.DbmovieID, &title)
	if m.DbmovieID != 0 && !moviegetimdbtitleparser(m) {
		m.DbmovieID = 0
	}
}

// getdbmovieidbytitleincache checks the title cache for a movie ID by title.
// It first checks the dbmovies cache, then the dbmovie_titles cache.
// If found, it populates ParseInfo.DbmovieID with the ID if it matches the IMDB title.
func getdbmovieidbytitleincache(m *database.ParseInfo, title string) {
	if title == "" {
		return
	}
	a := database.GetCachedTypeObjArr[database.DbstaticThreeStringTwoInt](logger.CacheDBMovie)
	for idx := range a {
		if strings.EqualFold(a[idx].Str1, title) || strings.EqualFold(a[idx].Str2, title) {
			if moviegetimdbtitle(a[idx].Num2, m) {
				m.DbmovieID = uint(a[idx].Num2)
				return
			}
		}
	}
	b := database.GetCachedTypeObjArr[database.DbstaticTwoStringOneInt](logger.CacheTitlesMovie)
	for idx := range b {
		if strings.EqualFold(b[idx].Str1, title) || strings.EqualFold(b[idx].Str2, title) {
			if moviegetimdbtitle(b[idx].Num, m) {
				m.DbmovieID = uint(b[idx].Num)
				return
			}
		}
	}
	m.DbmovieID = 0
}

// JobImportMoviesByList imports or updates a list of movies in parallel.
// It takes a list of movie titles/IDs, an index, a media type config, a list ID,
// and a flag for whether to add new movies.
// It logs the import result for each movie.
func JobImportMoviesByList(list []string, idx int, cfgp *config.MediaTypeConfig, listid int, addnew bool) {
	logger.LogDynamic("info", "Import/Update Movie", logger.NewLogField(logger.StrMovie, list[idx]), logger.NewLogField("row", idx))
	_, err := JobImportMovies(list[idx], cfgp, listid, addnew)
	if err != nil {
		if err.Error() != "movie ignored" {
			logger.LogDynamic("error", "Import/Update Failed", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrImdb, list[idx]))
		}
	}
}

// JobImportMovies imports a movie into the database and specified list
// given its IMDb ID. It handles checking if the movie exists, adding
// it if needed, updating metadata, and adding it to the target list.
func JobImportMovies(imdb string, cfgp *config.MediaTypeConfig, listid int, addnew bool) (uint, error) {
	if cfgp.Name == "" {
		return 0, logger.ErrCfgpNotFound
	}
	if imdb == "" {
		return 0, logger.ErrImdbEmpty
	}
	_, ok := importJobRunning.Load(imdb)
	if ok {
		return 0, errors.New("job running")
	}
	importJobRunning.Store(imdb, struct{}{})
	defer importJobRunning.Delete(imdb)

	var dbmovieadded bool

	dbid := MovieFindDBIDByImdb(&imdb)
	checkdbmovie := dbid >= 1

	if listid == -1 {
		listid = config.GetMediaListsEntryListID(cfgp, database.GetdatarowN[string](false, "select listname from movies where dbmovie_id in (Select id from dbmovies where imdb_id=?)", &imdb))
	}
	if !checkdbmovie && addnew {
		if listid == -1 {
			return 0, logger.ErrCfgpNotFound
		}
		_, err := AllowMovieImport(imdb, cfgp.Lists[listid].CfgList)
		if err != nil {
			return 0, err
		}

		if database.GetdatarowN[uint](false, "select id from dbmovies where imdb_id = ?", &imdb) == 0 {
			logger.LogDynamic("debug", "Insert dbmovie for", logger.NewLogField(logger.StrJob, &imdb))
			dbresult, err := database.ExecNid("insert into dbmovies (Imdb_ID) VALUES (?)", &imdb)
			if err != nil {
				return 0, err
			}
			dbid = uint(dbresult)
			dbmovieadded = true
		}
	}
	if dbid == 0 {
		dbid = MovieFindDBIDByImdb(&imdb)
	}
	if dbid == 0 {
		return 0, logger.ErrNotFoundDbmovie
	}

	if dbmovieadded || !addnew {
		logger.LogDynamic("debug", "Get metadata for", logger.NewLogField(logger.StrJob, &imdb))
		dbmovie, err := database.GetDbmovieByID(&dbid)
		if err != nil {
			return 0, err
		}

		metadata.Getmoviemetadata(&dbmovie, true)

		database.ExecN("update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?",
			&dbmovie.Title, &dbmovie.ReleaseDate, &dbmovie.Year, &dbmovie.Adult, &dbmovie.Budget, &dbmovie.Genres, &dbmovie.OriginalLanguage, &dbmovie.OriginalTitle, &dbmovie.Overview, &dbmovie.Popularity, &dbmovie.Revenue, &dbmovie.Runtime, &dbmovie.SpokenLanguages, &dbmovie.Status, &dbmovie.Tagline, &dbmovie.VoteAverage, &dbmovie.VoteCount, &dbmovie.TraktID, &dbmovie.MoviedbID, &dbmovie.ImdbID, &dbmovie.FreebaseMID, &dbmovie.FreebaseID, &dbmovie.FacebookID, &dbmovie.InstagramID, &dbmovie.TwitterID, &dbmovie.URL, &dbmovie.Backdrop, &dbmovie.Poster, &dbmovie.Slug, &dbmovie.ID)

		metadata.Getmoviemetatitles(&dbmovie, cfgp)
		if dbmovieadded {
			if config.SettingsGeneral.UseMediaCache {
				database.AppendThreeStringTwoIntCache(logger.CacheDBMovie, database.DbstaticThreeStringTwoInt{Str1: dbmovie.Title, Str2: dbmovie.Slug, Str3: imdb, Num1: dbmovie.Year, Num2: int(dbmovie.ID)})
			}
		}

		if dbmovie.Title == "" {
			_ = database.ScanrowsNdyn(false, database.QueryDbmovieTitlesGetTitleByIDLmit1, &dbmovie.Title, &dbmovie.ID)
			if dbmovie.Title != "" {
				database.ExecN("update dbmovies SET Title = ? where id = ?", &dbmovie.Title, &dbmovie.ID)
			}
		}
	}

	if !addnew {
		return dbid, nil
	}
	if dbid == 0 {
		dbid = MovieFindDBIDByImdb(&imdb)
		if dbid == 0 {
			return 0, logger.ErrNotFoundMovie
		}
	}
	if listid == -1 {
		return 0, errors.New("movie list empty")
	}

	err := Checkaddmovieentry(dbid, &cfgp.Lists[listid], &imdb)
	if err != nil {
		return 0, err
	}
	return dbid, nil
}

// Checkaddmovieentry checks if a movie with the given ID should be added to
// the given list. It handles ignore lists, replacing existing lists, and
// inserting into the DB if needed.
func Checkaddmovieentry(dbid uint, cfgplist *config.MediaListsConfig, imdb *string) error {
	if dbid == 0 {
		return nil
	}
	if cfgplist == nil || cfgplist.Name == "" {
		return errors.New("movie list empty")
	}
	var getid uint
	if cfgplist.IgnoreMapListsLen >= 1 {
		if config.SettingsGeneral.UseMediaCache {
			id := int(dbid)
			if database.CacheOneStringTwoIntIndexFunc(logger.CacheMovie, func(elem database.DbstaticOneStringTwoInt) bool {
				return elem.Num1 == id && (strings.EqualFold(elem.Str, cfgplist.Name) || logger.SlicesContainsI(cfgplist.IgnoreMapLists, elem.Str))
			}) {
				return errors.New("movie ignored")
			}
		} else {
			args := make([]any, cfgplist.IgnoreMapListsLen+1)
			for i := range cfgplist.IgnoreMapLists {
				args[i] = &cfgplist.IgnoreMapLists[i]
			}
			args[cfgplist.IgnoreMapListsLen] = &dbid
			_ = database.ScanrowsNdyn(false, logger.JoinStrings("select count() from movies where listname in (?", cfgplist.IgnoreMapListsQu, ") and dbmovie_id = ?"), &getid, args...)
			clear(args)
			if getid >= 1 {
				return errors.New("movie ignored")
			}
		}
	}
	if cfgplist.ReplaceMapListsLen >= 1 {
		if config.SettingsGeneral.UseMediaCache {
			var replaced bool
			intdbid := int(dbid)
			a := database.GetCachedTypeObjArr[database.DbstaticOneStringTwoInt](logger.CacheMovie)
			for idx := range a {
				if a[idx].Num1 != intdbid {
					continue
				}
				if !strings.EqualFold(a[idx].Str, cfgplist.Name) && logger.SlicesContainsI(cfgplist.ReplaceMapLists, a[idx].Str) {
					if cfgplist.TemplateQuality == "" {
						database.ExecN("update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &dbid, &a[idx].Str)
					} else {
						database.ExecN("update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &cfgplist.TemplateQuality, &dbid, &a[idx].Str)
					}
					replaced = true
				}
			}
			if replaced {
				database.RefreshMediaCache(false)
			}
		} else {
			var replaced bool
			arr := database.GetrowsN[string](false, database.GetdatarowN[int](false, "select count() from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", &dbid, &cfgplist.Name), "select listname from movies where dbmovie_id = ? and listname != ? COLLATE NOCASE", &dbid, &cfgplist.Name)
			for idx := range arr {
				if !logger.SlicesContainsI(cfgplist.ReplaceMapLists, arr[idx]) {
					continue
				}
				if cfgplist.TemplateQuality == "" {
					database.ExecN("update movies SET listname = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &dbid, &arr[idx])
				} else {
					database.ExecN("update movies SET listname = ?, quality_profile = ? where dbmovie_id = ? and listname = ? COLLATE NOCASE", &cfgplist.Name, &cfgplist.TemplateQuality, &dbid, &arr[idx])
				}
				replaced = true
			}
			if replaced {
				database.RefreshMediaCache(false)
			}
			clear(arr)
		}
	}

	database.ScanrowsNdyn(false, "select count() from movies where dbmovie_id = ? and listname = ?", &getid, &dbid, &cfgplist.Name)
	if cfgplist.IgnoreMapListsLen >= 1 {
		if getid == 0 {
			args := make([]any, cfgplist.IgnoreMapListsLen+1)
			for i := range cfgplist.IgnoreMapLists {
				args[i] = &cfgplist.IgnoreMapLists[i]
			}
			args[cfgplist.IgnoreMapListsLen] = &dbid
			_ = database.ScanrowsNdyn(false, logger.JoinStrings("select count() from movies where listname in (?", cfgplist.IgnoreMapListsQu, ") and dbmovie_id = ?"), &getid, args...)
			clear(args)
		}
	}
	if getid == 0 {
		logger.LogDynamic("debug", "Insert Movie for", logger.NewLogField(logger.StrImdb, imdb))
		movieid, err := database.ExecNid("Insert into movies (missing, listname, dbmovie_id, quality_profile) values (1, ?, ?, ?)", &cfgplist.Name, &dbid, &cfgplist.TemplateQuality)
		if err != nil {
			return err
		}
		if config.SettingsGeneral.UseMediaCache {
			database.AppendOneStringTwoIntCache(logger.CacheMovie, database.DbstaticOneStringTwoInt{Str: cfgplist.Name, Num1: int(dbid), Num2: int(movieid)})
		}
	}
	return nil
}

// AllowMovieImport checks if a movie can be imported based on the
// list configuration settings for minimum votes, minimum rating, excluded
// genres, and included genres. It returns a bool indicating if the import
// is allowed and an error if it is disallowed.
func AllowMovieImport(imdb string, listcfg *config.ListsConfig) (bool, error) {
	if imdb == "" {
		return false, logger.ErrNotFound
	}
	var i int
	if listcfg.MinVotes != 0 {
		database.ScanrowsNdyn(true, "select count() from imdb_ratings where tconst = ? and num_votes < ?", &i, &imdb, &listcfg.MinVotes)
		if i >= 1 {
			return false, errors.New("error vote count too low")
		}
	}
	if listcfg.MinRating != 0 {
		database.ScanrowsNdyn(true, "select count() from imdb_ratings where tconst = ? and average_rating < ?", &i, &imdb, &listcfg.MinRating)
		if i >= 1 {
			return false, errors.New("error average vote too low")
		}
	}

	if listcfg.ExcludegenreLen == 0 && listcfg.IncludegenreLen == 0 {
		return true, nil
	}
	genrearr := database.Getrows1size[string](true, "select count(genre) from imdb_genres where tconst = ?", "select genre from imdb_genres where tconst = ?", &imdb)
	if len(genrearr) == 0 {
		return true, nil
	}
	var excludeby string

	//labexcluded:
	for idx := range listcfg.Excludegenre {
		if logger.SlicesContainsI(genrearr, listcfg.Excludegenre[idx]) {
			excludeby = listcfg.Excludegenre[idx]
			break
		}
	}
	if excludeby != "" && listcfg.ExcludegenreLen >= 1 {
		clear(genrearr)
		return false, errors.New("excluded by " + excludeby)
	}

	var includebygenre bool

	//labincluded:
	for idx := range listcfg.Includegenre {
		if logger.SlicesContainsI(genrearr, listcfg.Includegenre[idx]) {
			includebygenre = true
			break
		}
	}

	clear(genrearr)
	if !includebygenre && listcfg.IncludegenreLen >= 1 {
		return false, errors.New("included genre not found")
	}
	return true, nil
}

// getsearchprovider returns the priority ordered list of metadata sources to use for searching movies based on searchtyperss.
// If searchtyperss is true, it will return MovieRSSMetaSourcePriority from config.
// If searchtyperss is false, it will return MovieParseMetaSourcePriority from config.
// If neither are set, it returns a default order of "imdb", "tmdb", "omdb".
func getsearchprovider(searchtyperss bool) []string {
	if searchtyperss {
		if len(config.SettingsGeneral.MovieRSSMetaSourcePriority) >= 1 {
			return config.SettingsGeneral.MovieRSSMetaSourcePriority
		}
	} else {
		if len(config.SettingsGeneral.MovieParseMetaSourcePriority) >= 1 {
			return config.SettingsGeneral.MovieParseMetaSourcePriority
		}
	}
	return []string{"imdb", "tmdb", "omdb"}
}

// JobImportDBSeriesStatic wraps jobImportDBSeries to import a series from a DbstaticTwoStringOneInt row containing the TVDB ID and name.
func JobImportDBSeriesStatic(row *database.DbstaticTwoStringOneInt, cfgp *config.MediaTypeConfig, listid int, checkall, addnew bool) error {
	s := &config.SerieConfig{TvdbID: row.Num, Name: row.Str1}
	defer s.Close()
	return jobImportDBSeries(s, cfgp, listid, checkall, addnew)
}

// JobImportDBSeries imports a series into the database and media lists from a SerieConfig.
// It handles adding new series or updating existing ones, refreshing metadata from TheTVDB,
// adding missing episodes, and updating the series lists table.
func JobImportDBSeries(series *config.MainSerieConfig, idxserie int, cfgp *config.MediaTypeConfig, listid int, checkall, addnew bool) {
	logger.LogDynamic("info", "Import/Update Serie", logger.NewLogField(logger.StrSeries, series.Serie[idxserie].Name), logger.NewLogField("row", idxserie))
	err := jobImportDBSeries(&series.Serie[idxserie], cfgp, listid, checkall, addnew)
	if err != nil {
		logger.LogDynamic("error", "Import/Update Failed", logger.NewLogFieldValue(err), logger.NewLogField("Serie", series.Serie[idxserie].TvdbID))
	}
}

// jobImportDBSeries imports a series into the database and media lists from a SerieConfig.
// It handles adding new series or updating existing ones, refreshing metadata from TheTVDB,
// adding missing episodes, and updating the series lists table.
func jobImportDBSeries(serieconfig *config.SerieConfig, cfgp *config.MediaTypeConfig, listid int, checkall, addnew bool) error {
	if cfgp == nil {
		return logger.ErrCfgpNotFound
	}
	jobName := serieconfig.Name
	if listid == -1 {
		listid = config.GetMediaListsEntryListID(cfgp, database.GetdatarowN[string](false, "select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", &serieconfig.TvdbID))
	}
	if jobName == "" && listid >= 0 {
		jobName = cfgp.Lists[listid].Name
	}
	if jobName == "" {
		return errors.New("jobname missing")
	}

	_, ok := importJobRunning.Load(jobName)
	if ok {
		return errors.New("job running")
	}
	importJobRunning.Store(jobName, struct{}{})
	defer importJobRunning.Delete(jobName)
	if !addnew && listid == -1 {
		listid = config.GetMediaListsEntryListID(cfgp, database.GetdatarowN[string](false, "select listname from series where dbserie_id in (Select id from dbseries where thetvdb_id=?)", &serieconfig.TvdbID))
	}

	var dbserieadded bool
	var dbid uint
	if addnew && strings.EqualFold(serieconfig.Source, "none") {
		if database.GetdatarowN[int](false, "select count() from dbseries where seriename = ? COLLATE NOCASE", &serieconfig.Name) == 0 {
			dbid = insertdbserie(serieconfig.Name, serieconfig.AlternateName, serieconfig.TvdbID, serieconfig.Identifiedby)
			if dbid != 0 {
				dbserieadded = true
			}
		} else {
			_ = database.ScanrowsNdyn(false, database.QueryDbseriesGetIDByName, &dbid, &serieconfig.Name)
		}
		if dbid == 0 {
			return errors.New("id not fetched")
		}
	}
	//var addserietitle database.DbstaticTwoStringOneInt
	if !addnew && strings.EqualFold(serieconfig.Source, "none") {
		_ = database.ScanrowsNdyn(false, database.QueryDbseriesGetIDByName, &dbid, &serieconfig.Name)
		if dbid >= 1 {
			for idx := range serieconfig.AlternateName {
				addserietitle := database.DbstaticTwoStringOneInt{Num: int(dbid), Str1: serieconfig.AlternateName[idx]}
				addalternateserietitle(&addserietitle, "")
			}
		}
	}
	if serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, logger.StrTvdb) {
		getid := database.GetdatarowN[int](false, "select count() from dbseries where thetvdb_id = ?", &serieconfig.TvdbID)
		if addnew && getid == 0 {
			dbid = insertdbserie(serieconfig.Name, serieconfig.AlternateName, serieconfig.TvdbID, serieconfig.Identifiedby)
			if dbid != 0 {
				dbserieadded = true
			}
		} else if getid == 0 {
			return errors.New("add not wanted")
		} else if getid >= 1 {
			_ = database.ScanrowsNdyn(false, database.QueryDbseriesGetIDByTvdb, &dbid, &serieconfig.TvdbID)
		}
		if dbid != 0 {
			for idx := range serieconfig.AlternateName {
				addserietitle := database.DbstaticTwoStringOneInt{Num: int(dbid), Str1: serieconfig.AlternateName[idx]}
				addalternateserietitle(&addserietitle, "")
			}
		}
		if dbid != 0 && (dbserieadded || !addnew) && serieconfig.TvdbID != 0 {
			//Update Metadata
			dbserie, err := database.GetDbserieByID(&dbid)
			if err != nil {
				return errors.New("db fetch failed")
			}
			if dbserie.Seriename == "" {
				dbserie.Seriename = serieconfig.Name
			}
			if dbserie.Identifiedby == "" {
				dbserie.Identifiedby = serieconfig.Identifiedby
			}
			logger.LogDynamic("debug", "Get metadata for", logger.NewLogField(logger.StrTvdb, serieconfig.TvdbID))
			serieconfig.AlternateName = metadata.SerieGetMetadata(&dbserie, cfgp.MetadataLanguage, config.SettingsGeneral.SerieMetaSourceTmdb, config.SettingsGeneral.SerieMetaSourceTrakt, !addnew, serieconfig.AlternateName)
			serieconfig.AlternateName = append(serieconfig.AlternateName, serieconfig.Name)
			serieconfig.AlternateName = append(serieconfig.AlternateName, dbserie.Seriename)

			database.ExecN("update dbseries SET Seriename = ?, Aliases = ?, Season = ?, Status = ?, Firstaired = ?, Network = ?, Runtime = ?, Language = ?, Genre = ?, Overview = ?, Rating = ?, Siterating = ?, Siterating_Count = ?, Slug = ?, Trakt_ID = ?, Imdb_ID = ?, Thetvdb_ID = ?, Freebase_M_ID = ?, Freebase_ID = ?, Tvrage_ID = ?, Facebook = ?, Instagram = ?, Twitter = ?, Banner = ?, Poster = ?, Fanart = ?, Identifiedby = ? where id = ?",
				&dbserie.Seriename, &dbserie.Aliases, &dbserie.Season, &dbserie.Status, &dbserie.Firstaired, &dbserie.Network, &dbserie.Runtime, &dbserie.Language, &dbserie.Genre, &dbserie.Overview, &dbserie.Rating, &dbserie.Siterating, &dbserie.SiteratingCount, &dbserie.Slug, &dbserie.TraktID, &dbserie.ImdbID, &dbserie.ThetvdbID, &dbserie.FreebaseMID, &dbserie.FreebaseID, &dbserie.TvrageID, &dbserie.Facebook, &dbserie.Instagram, &dbserie.Twitter, &dbserie.Banner, &dbserie.Poster, &dbserie.Fanart, &dbserie.Identifiedby, &dbserie.ID)

			//size+10
			titles := database.Getrows1size[database.DbstaticOneStringOneInt](false, "select count() from dbserie_alternates where dbserie_id = ?", "select title, id from dbserie_alternates where dbserie_id = ?", &dbserie.ID)

			lenarr := cfgp.MetadataTitleLanguagesLen

			if config.SettingsGeneral.SerieAlternateTitleMetaSourceImdb && dbserie.ImdbID != "" {
				queryid := logger.AddImdbPrefix(dbserie.ImdbID)
				arr := database.Getrows1size[database.DbstaticThreeString](true, "select count() from imdb_akas where tconst = ?", "select title, region, slug from imdb_akas where tconst = ?", &queryid)
				for idx := range arr {
					if database.GetDbStaticOneStringOneIntIdx(titles, arr[idx].Str1) != -1 {
						continue
					}

					if logger.SlicesContainsI(serieconfig.DisallowedName, arr[idx].Str1) {
						continue
					}
					if lenarr == 0 || logger.SlicesContainsI(cfgp.MetadataTitleLanguages, arr[idx].Str2) {
						addentry := database.DbstaticOneStringOneInt{Num: int(dbid), Str: arr[idx].Str1}
						titles = append(titles, addentry)
						addserietitle := database.DbstaticTwoStringOneInt{Num: int(dbserie.ID), Str1: arr[idx].Str1}
						addalternateserietitle(&addserietitle, arr[idx].Str2)
					}
				}
				clear(arr)
			}
			if config.SettingsGeneral.SerieAlternateTitleMetaSourceTrakt && (dbserie.TraktID != 0 || dbserie.ImdbID != "") {
				tbl := apiexternal.GetTraktSerieAliases(&dbserie)
				for idx := range tbl {
					if database.GetDbStaticOneStringOneIntIdx(titles, tbl[idx].Title) != -1 {
						continue
					}
					if logger.SlicesContainsI(serieconfig.DisallowedName, tbl[idx].Title) {
						continue
					}

					if lenarr == 0 || logger.SlicesContainsI(cfgp.MetadataTitleLanguages, tbl[idx].Country) {
						addentry := database.DbstaticOneStringOneInt{Num: int(dbid), Str: tbl[idx].Title}
						titles = append(titles, addentry)
						addserietitle := database.DbstaticTwoStringOneInt{Num: int(dbserie.ID), Str1: tbl[idx].Title}
						addalternateserietitle(&addserietitle, tbl[idx].Country)
					}
				}
				clear(tbl)
			}

			for idx := range serieconfig.AlternateName {
				if database.GetDbStaticOneStringOneIntIdx(titles, serieconfig.AlternateName[idx]) != -1 {
					continue
				}
				addserietitle := database.DbstaticTwoStringOneInt{Num: int(dbid), Str1: serieconfig.AlternateName[idx]}
				addalternateserietitle(&addserietitle, "")
				addentry := database.DbstaticOneStringOneInt{Num: int(dbserie.ID)}
				addentry.Str = serieconfig.AlternateName[idx]
				titles = append(titles, addentry)
			}
			for idx := range serieconfig.DisallowedName {
				database.ExecN("delete from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE", &dbserie.ID, &serieconfig.DisallowedName[idx])
			}

			if (checkall || dbserieadded || !addnew) && (serieconfig.Source == "" || strings.EqualFold(serieconfig.Source, logger.StrTvdb)) {
				logger.LogDynamic("debug", "Get episodes for", logger.NewLogField(logger.StrTvdb, serieconfig.TvdbID))

				if dbserie.ThetvdbID != 0 {
					apiexternal.UpdateTvdbSeriesEpisodes(dbserie.ThetvdbID, cfgp.MetadataLanguage, dbserie.ID)
				}
				if config.SettingsGeneral.SerieMetaSourceTrakt && dbserie.ImdbID != "" {
					apiexternal.UpdateTraktSerieSeasonsAndEpisodes(dbserie.ImdbID, &dbserie.ID)
				}
			}
			clear(titles)
		}
	}

	if dbid == 0 {
		return logger.ErrNotFoundDbserie
	}

	if addnew {
		//Add Entry in SeriesTable

		if listid == -1 {
			return logger.ErrListnameEmpty
		}
		arr := database.Getrows1size[database.DbstaticOneStringOneInt](false, database.QuerySeriesCountByDBID, "select lower(listname), id from series where dbserie_id = ?", &dbid)
		for idx := range arr {
			if logger.SlicesContainsI(cfgp.Lists[listid].IgnoreMapLists, arr[idx].Str) {
				return errors.New("series skip2 for")
			}
			if logger.SlicesContainsI(cfgp.Lists[listid].ReplaceMapLists, arr[idx].Str) {
				database.ExecN("update series SET listname = ?, dbserie_id = ? where id = ?", &cfgp.Lists[listid].Name, &dbid, &arr[idx].Num)
			}
		}
		clear(arr)
		getid := database.GetdatarowN[int](false, "select count() from series where dbserie_id = ? and listname = ? COLLATE NOCASE", &dbid, &cfgp.Lists[listid].Name)
		if getid == 0 {
			logger.LogDynamic("debug", "Add series for", logger.NewLogField(logger.StrListname, cfgp.Lists[listid].Name), logger.NewLogField(logger.StrTvdb, serieconfig.TvdbID))
			serieid, err := database.ExecNid("Insert into series (dbserie_id, listname, rootpath, search_specials, dont_search, dont_upgrade) values (?, ?, ?, ?, ?, ?)", &dbid, &cfgp.Lists[listid].Name, &serieconfig.Target, &serieconfig.SearchSpecials, &serieconfig.DontSearch, &serieconfig.DontUpgrade)
			if err != nil {
				return err
			}
			if config.SettingsGeneral.UseMediaCache {
				database.AppendOneStringTwoIntCache(logger.CacheSeries, database.DbstaticOneStringTwoInt{Str: cfgp.Lists[listid].Name, Num1: int(dbid), Num2: int(serieid)})
			}
		} else {
			database.ExecN("update series SET search_specials=?, dont_search=?, dont_upgrade=? where dbserie_id = ? and listname = ?", &serieconfig.SearchSpecials, &serieconfig.DontSearch, &serieconfig.DontUpgrade, &dbid, &cfgp.Lists[listid].Name)
		}
	}

	dbseries := database.Getrows1size[int](false, database.QueryDbserieEpisodesCountByDBID, "select id from dbserie_episodes where dbserie_id = ?", &dbid)
	episodes := database.Getrows1size[database.DbstaticTwoInt](false, "select count() from serie_episodes where dbserie_id = ?",
		"select dbserie_episode_id, serie_id from serie_episodes where dbserie_id = ?", &dbid)

	arr := database.Getrows1size[int](false, database.QuerySeriesCountByDBID, "select id from series where dbserie_id = ?", &dbid)
	for idx := range arr {
		checkandaddserieepisodes(dbseries, episodes, arr[idx], dbid, cfgp)
	}

	clear(arr)
	clear(dbseries)
	clear(episodes)
	return nil
}

// checkandaddserieepisodes checks for any episodes in the dbseries slice that are not already associated with the given serieid. It adds any missing episodes, setting them as missing with the default quality profile.
func checkandaddserieepisodes(dbseries []int, episodes []database.DbstaticTwoInt, serieid int, dbid uint, cfgp *config.MediaTypeConfig) {
	quality := database.GetdatarowN[string](false, "select quality_profile from serie_episodes where serie_id = ? and length(quality_profile)>=1 limit 1", &serieid)
	if quality == "" {
		quality = cfgp.TemplateQuality
	}

LabDBSeries:
	for idxdb := range dbseries {
		for idxepi := range episodes {
			if episodes[idxepi].Num2 == serieid && episodes[idxepi].Num1 == dbseries[idxdb] {
				continue LabDBSeries
			}
		}
		database.ExecN("Insert into serie_episodes (dbserie_id, serie_id, missing, quality_profile, dbserie_episode_id) values (?, ?, 1, ?, ?)", &dbid, &serieid, &quality, &dbseries[idxdb])
	}
}

// insertdbserie inserts a new dbseries row into the dbseries table.
// It takes the name, aliases slice, TheTVDB ID and identifiedby string as parameters.
// It inserts a new row into the dbseries table with those values.
// It returns the auto generated uint ID for the new row.
// It also handles caching and inserting alternate names.
func insertdbserie(name string, aliases []string, tbdbid int, identifiedby string) uint {
	logger.LogDynamic("debug", "Insert dbseries for", logger.NewLogField(logger.StrTvdb, tbdbid))
	inres, err := database.ExecNid("insert into dbseries (seriename, aliases, thetvdb_id, identifiedby) values (?, ?, ?, ?)",
		&name, strings.Join(aliases, ","), &tbdbid, &identifiedby)
	if err != nil {
		return 0
	}
	dbid := uint(inres)

	if config.SettingsGeneral.UseMediaCache {
		database.AppendTwoStringIntCache(logger.CacheDBSeries, database.DbstaticTwoStringOneInt{Str1: name, Str2: logger.StringToSlug(name), Num: int(dbid)})
	}

	aliases = append(aliases, name)
	for idx := range aliases {
		if aliases[idx] == "" {
			continue
		}
		addserietitle := database.DbstaticTwoStringOneInt{Num: int(dbid), Str1: aliases[idx]}
		addalternateserietitle(&addserietitle, "")
	}
	return dbid
}

// addAlternateSerieTitle adds an alternate title for the given series ID.
// It checks if the title already exists for that ID, and if not, inserts
// it into the dbserie_alternates table. The region parameter indicates
// the language/region for the title. It also adds the title to the cache
// if enabled.
func addalternateserietitle(addserietitle *database.DbstaticTwoStringOneInt, region string) {
	if database.GetdatarowN[uint](false, "select count() from dbserie_alternates where dbserie_id = ? and title = ? COLLATE NOCASE", &addserietitle.Num, &addserietitle.Str1) == 0 {
		addserietitle.Str2 = logger.StringToSlug(addserietitle.Str1)

		database.ExecN("Insert into dbserie_alternates (title, slug, dbserie_id, region) values (?, ?, ?, ?)", &addserietitle.Str1, &addserietitle.Str2, &addserietitle.Num, &region)
		if config.SettingsGeneral.UseMediaCache {
			database.AppendTwoStringIntCache(logger.CacheDBSeriesAlt, *addserietitle)
		}
	}
}

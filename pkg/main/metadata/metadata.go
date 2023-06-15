package metadata

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

func MovieGetTitles(movie *database.Dbmovie, cfgpstr string) *[]database.DbmovieTitle {
	count := database.QueryIntColumn("select count() from dbmovie_titles where dbmovie_id = ?", movie.ID)
	if count == 0 {
		count = 15
	}
	result := make([]database.DbmovieTitle, 0, count)
	lenarr := len(config.SettingsMedia[cfgpstr].MetadataTitleLanguages)
	if config.SettingsGeneral.MovieAlternateTitleMetaSourceImdb && movie.ImdbID != "" {
		movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)
		tbl := database.QueryStaticColumnsThreeString("select region, title, slug from imdb_akas where tconst = ?", &movie.ImdbID)
		var contfor bool
		for idx := range *tbl {
			contfor = true
			for idxq := range config.SettingsMedia[cfgpstr].MetadataTitleLanguages {
				if strings.EqualFold(config.SettingsMedia[cfgpstr].MetadataTitleLanguages[idxq], (*tbl)[idx].Str1) {
					contfor = false
					break
				}
			}
			if lenarr >= 1 && contfor {
				continue
			}
			result = append(result, database.DbmovieTitle{DbmovieID: movie.ID, Title: (*tbl)[idx].Str2, Slug: (*tbl)[idx].Str3, Region: (*tbl)[idx].Str1})
		}
		logger.Clear(tbl)
	}
	if config.SettingsGeneral.MovieAlternateTitleMetaSourceTmdb && movie.MoviedbID != 0 {
		moviedbtitles, err := apiexternal.TmdbAPI.GetMovieTitles(movie.MoviedbID)
		if err == nil && len(moviedbtitles.Titles) >= 1 {
			//logger.Grow(&result, len(moviedbtitles.Titles))
			//result = logger.GrowSliceBy(result, len(moviedbtitles.Titles))
			var contfor, cont bool
			for idx := range moviedbtitles.Titles {
				contfor = true
				for idxq := range config.SettingsMedia[cfgpstr].MetadataTitleLanguages {
					if strings.EqualFold(config.SettingsMedia[cfgpstr].MetadataTitleLanguages[idxq], moviedbtitles.Titles[idx].Iso31661) {
						contfor = false
						break
					}
				}
				if lenarr >= 1 && contfor {
					continue
				}
				cont = false
				for idxi := range result {
					if strings.EqualFold(result[idxi].Title, moviedbtitles.Titles[idx].Title) {
						cont = true
						break
					}
				}
				if cont {
					continue
				}
				//if logger.ContainsFunc(&result, func(c database.DbmovieTitle) bool {
				//	return strings.EqualFold(c.Title, moviedbtitles.Titles[idx].Title)
				//}) {
				//	continue
				//}
				result = append(result, database.DbmovieTitle{DbmovieID: movie.ID, Title: moviedbtitles.Titles[idx].Title, Slug: logger.StringToSlug(moviedbtitles.Titles[idx].Title), Region: moviedbtitles.Titles[idx].Iso31661})
			}
		}
		moviedbtitles.Close()
	}
	if config.SettingsGeneral.MovieAlternateTitleMetaSourceTrakt && movie.ImdbID != "" {
		traktaliases, err := apiexternal.TraktAPI.GetMovieAliases(movie.ImdbID)
		if err != nil || traktaliases == nil || len(*traktaliases) == 0 {
			logger.Clear(traktaliases)
			return &result
		}
		//logger.Grow(&result, len(traktaliases))
		//result = logger.GrowSliceBy(result, len(traktaliases.Aliases))
		var contfor, cont bool
		for idxalias := range *traktaliases {
			contfor = true
			for idxq := range config.SettingsMedia[cfgpstr].MetadataTitleLanguages {
				if strings.EqualFold(config.SettingsMedia[cfgpstr].MetadataTitleLanguages[idxq], (*traktaliases)[idxalias].Country) {
					contfor = false
					break
				}
			}
			if lenarr >= 1 && contfor {
				continue
			}
			cont = false
			for idxi := range result {
				if strings.EqualFold(result[idxi].Title, (*traktaliases)[idxalias].Title) {
					cont = true
					break
				}
			}
			if cont {
				continue
			}
			//if logger.ContainsFunc(&result, func(c database.DbmovieTitle) bool {
			//	return strings.EqualFold(c.Title, traktaliases[idxalias].Title)
			//}) {
			//	continue
			//}
			result = append(result, database.DbmovieTitle{DbmovieID: movie.ID, Title: (*traktaliases)[idxalias].Title, Slug: logger.StringToSlug((*traktaliases)[idxalias].Title), Region: (*traktaliases)[idxalias].Country})
		}
		logger.Clear(traktaliases)
	}
	if len(result) < 10 {
		result = result[:len(result):len(result)]
	}
	return &result
}

func MovieGetImdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	queryimdbid := logger.AddImdbPrefix(movie.ImdbID)
	imdbdata, err := database.GetImdbTitle("select * from imdb_titles where tconst = ?", &queryimdbid)
	if err != nil {
		return
	}
	if (movie.Title == "" || overwrite) && imdbdata.PrimaryTitle != "" {
		movie.Title = imdbdata.PrimaryTitle
	}
	if (movie.Year == 0 || overwrite) && imdbdata.StartYear != 0 {
		movie.Year = imdbdata.StartYear
	}
	if (!movie.Adult && imdbdata.IsAdult) || overwrite {
		movie.Adult = imdbdata.IsAdult
	}
	if (movie.Genres == "" || overwrite) && imdbdata.Genres != "" {
		movie.Genres = imdbdata.Genres
	}
	if (movie.OriginalTitle == "" || overwrite) && imdbdata.OriginalTitle != "" {
		movie.OriginalTitle = imdbdata.OriginalTitle
	}
	if (movie.Runtime == 0 || overwrite) && imdbdata.RuntimeMinutes != 0 {
		if movie.Runtime != 0 && (imdbdata.RuntimeMinutes == 1 || imdbdata.RuntimeMinutes == 90) {
			logger.Log.Debug().Str("imdb", movie.ImdbID).Msg("skipped imdb movie runtime for")
		} else {
			movie.Runtime = imdbdata.RuntimeMinutes
		}
	}
	if (movie.Slug == "" || overwrite) && imdbdata.Slug != "" {
		movie.Slug = imdbdata.Slug
	}
	if (movie.URL == "" || overwrite) && queryimdbid != "" {
		movie.URL = "https://www.imdb.com/title/" + queryimdbid
	}
	imdbratedata, err := database.GetImdbRating("select * from imdb_ratings where tconst = ?", &queryimdbid)
	if err != nil {
		return
	}
	if (movie.VoteAverage == 0 || overwrite) && imdbratedata.AverageRating != 0 {
		movie.VoteAverage = imdbratedata.AverageRating
	}
	if (movie.VoteCount == 0 || overwrite) && imdbratedata.NumVotes != 0 {
		movie.VoteCount = imdbratedata.NumVotes
	}
}

func MovieGetTmdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.MoviedbID == 0 {
		if movie.ImdbID == "" {
			return
		}
		moviedb, err := apiexternal.TmdbAPI.FindImdb(movie.ImdbID)
		if err != nil {
			return
		}
		if len(moviedb.MovieResults) >= 1 {
			movie.MoviedbID = moviedb.MovieResults[0].ID
			moviedb.Close()
		} else {
			moviedb.Close()
			return
		}
	}
	moviedbdetails, err := apiexternal.TmdbAPI.GetMovie(movie.MoviedbID)
	if err != nil {
		return
	}
	if (!movie.Adult && moviedbdetails.Adult) || overwrite {
		movie.Adult = moviedbdetails.Adult
	}
	if (movie.Title == "" || overwrite) && moviedbdetails.Title != "" {
		movie.Title = moviedbdetails.Title
		logger.HTMLUnescape(&movie.Title)
		logger.Unquote(&movie.Title)
	}
	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}
	if (movie.Budget == 0 || overwrite) && moviedbdetails.Budget != 0 {
		movie.Budget = moviedbdetails.Budget
	}
	if moviedbdetails.ReleaseDate != "" && !movie.ReleaseDate.Valid {
		movie.ReleaseDate = *database.ParseDate(moviedbdetails.ReleaseDate)
		if (movie.Year == 0 || overwrite) && movie.ReleaseDate.Time.Year() != 0 {
			movie.Year = movie.ReleaseDate.Time.Year()
		}
	}
	if (movie.Genres == "" || overwrite) && len(moviedbdetails.Genres) != 0 {
		movie.Genres = logger.Join(&moviedbdetails.Genres, func(elem *apiexternal.TheMovieDBMovieGenres) string { return elem.Name }, ",")
		// for idxgenre := range moviedbdetails.Genres {
		// 	if movie.Genres != "" {
		// 		movie.Genres += ","
		// 	}
		// 	movie.Genres += moviedbdetails.Genres[idxgenre].Name
		// }
	}
	if (movie.OriginalLanguage == "" || overwrite) && moviedbdetails.OriginalLanguage != "" {
		movie.OriginalLanguage = moviedbdetails.OriginalLanguage
	}
	if (movie.OriginalTitle == "" || overwrite) && moviedbdetails.OriginalTitle != "" {
		movie.OriginalTitle = moviedbdetails.OriginalTitle
	}
	if (movie.Overview == "" || overwrite) && moviedbdetails.Overview != "" {
		movie.Overview = moviedbdetails.Overview
	}
	if (movie.Popularity == 0 || overwrite) && moviedbdetails.Popularity != 0 {
		movie.Popularity = moviedbdetails.Popularity
	}
	if (movie.Revenue == 0 || overwrite) && moviedbdetails.Revenue != 0 {
		movie.Revenue = moviedbdetails.Revenue
	}
	if (movie.Runtime == 0) && moviedbdetails.Runtime != 0 {
		if movie.Runtime != 0 && (moviedbdetails.Runtime == 1 || moviedbdetails.Runtime == 90) {
			logger.Log.Debug().Str("imdb", movie.ImdbID).Msg("skipped moviedb movie runtime for")
		} else {
			movie.Runtime = moviedbdetails.Runtime
		}
	}
	if (movie.SpokenLanguages == "" || overwrite) && len(moviedbdetails.SpokenLanguages) != 0 {
		movie.SpokenLanguages = logger.Join(&moviedbdetails.SpokenLanguages, func(elem *apiexternal.TheMovieDBMovieSpokenLanguages) string { return elem.EnglishName }, ",")
		// movie.SpokenLanguages = ""

		// for idxlang := range moviedbdetails.SpokenLanguages {
		// 	if movie.SpokenLanguages != "" {
		// 		movie.SpokenLanguages += ","
		// 	}
		// 	movie.SpokenLanguages += moviedbdetails.SpokenLanguages[idxlang].EnglishName
		// }
	}
	if (movie.Status == "" || overwrite) && moviedbdetails.Status != "" {
		movie.Status = moviedbdetails.Status
	}
	if (movie.Tagline == "" || overwrite) && moviedbdetails.Tagline != "" {
		movie.Tagline = moviedbdetails.Tagline
	}
	if (movie.VoteAverage == 0 || overwrite) && moviedbdetails.VoteAverage != 0 {
		movie.VoteAverage = moviedbdetails.VoteAverage
	}
	if (movie.VoteCount == 0 || overwrite) && moviedbdetails.VoteCount != 0 {
		movie.VoteCount = moviedbdetails.VoteCount
	}
	if (movie.Poster == "" || overwrite) && moviedbdetails.Poster != "" {
		movie.Poster = moviedbdetails.Poster
	}
	if (movie.Backdrop == "" || overwrite) && moviedbdetails.Backdrop != "" {
		movie.Backdrop = moviedbdetails.Backdrop
	}
	if (movie.MoviedbID == 0 || overwrite) && moviedbdetails.ID != 0 {
		movie.MoviedbID = moviedbdetails.ID
	}
	moviedbdetails.Close()
}

func MovieGetOmdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	omdbdetails, err := apiexternal.OmdbAPI.GetMovie(movie.ImdbID)
	if err != nil {
		return
	}
	if (movie.Title == "" || overwrite) && omdbdetails.Title != "" {
		movie.Title = omdbdetails.Title
		logger.HTMLUnescape(&movie.Title)
		logger.Unquote(&movie.Title)
	}
	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}
	if (movie.Genres == "" || overwrite) && omdbdetails.Genre != "" {
		movie.Genres = omdbdetails.Genre
	}
	if (movie.VoteCount == 0 || overwrite) && omdbdetails.ImdbVotes != "" {
		movie.VoteCount = logger.StringToInt(omdbdetails.ImdbVotes)
	}
	if (movie.VoteAverage == 0 || overwrite) && omdbdetails.ImdbRating != "" {
		movie.VoteAverage = float32(logger.StringToInt(omdbdetails.ImdbRating))
	}
	if (movie.Year == 0 || overwrite) && omdbdetails.Year != "" {
		movie.Year = logger.StringToInt(omdbdetails.Year)
	}
	if (movie.URL == "" || overwrite) && omdbdetails.Website != "" {
		movie.URL = omdbdetails.Website
	}
	if (movie.Overview == "" || overwrite) && omdbdetails.Plot != "" {
		movie.Overview = omdbdetails.Plot
	}
	logger.ClearVar(omdbdetails)
}

func MovieGetTraktMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	traktdetails, err := apiexternal.TraktAPI.GetMovie(movie.ImdbID)
	if err != nil {
		return
	}
	if (movie.Title == "" || overwrite) && traktdetails.Title != "" {
		movie.Title = traktdetails.Title
		logger.HTMLUnescape(&movie.Title)
		logger.Unquote(&movie.Title)
	}
	if (movie.Slug == "" || overwrite) && traktdetails.Ids.Slug != "" {
		movie.Slug = traktdetails.Ids.Slug
	}
	if (movie.Genres == "" || overwrite) && len(traktdetails.Genres) != 0 {
		movie.Genres = strings.Join(traktdetails.Genres, ",")
	}
	if (movie.VoteCount == 0 || overwrite) && traktdetails.Votes != 0 {
		movie.VoteCount = traktdetails.Votes
	}
	if (movie.VoteAverage == 0 || overwrite) && traktdetails.Rating != 0 {
		movie.VoteAverage = traktdetails.Rating
	}
	if (movie.Year == 0 || overwrite) && traktdetails.Year != 0 {
		movie.Year = traktdetails.Year
	}
	if (movie.Overview == "" || overwrite) && traktdetails.Overview != "" {
		movie.Overview = traktdetails.Overview
	}
	if (movie.Runtime == 0 || overwrite) && traktdetails.Runtime != 0 {
		if movie.Runtime != 0 && (traktdetails.Runtime == 1 || traktdetails.Runtime == 90) {
			logger.Log.Debug().Str("imdb", movie.ImdbID).Msg("skipped trakt movie runtime for")
		} else {
			movie.Runtime = traktdetails.Runtime
		}
	}
	if (movie.Status == "" || overwrite) && traktdetails.Status != "" {
		movie.Status = traktdetails.Status
	}
	if (movie.MoviedbID == 0 || overwrite) && traktdetails.Ids.Tmdb != 0 {
		movie.MoviedbID = traktdetails.Ids.Tmdb
	}
	if (movie.TraktID == 0 || overwrite) && traktdetails.Ids.Trakt != 0 {
		movie.TraktID = traktdetails.Ids.Trakt
	}
	if (!movie.ReleaseDate.Valid || overwrite) && traktdetails.Released != "" {
		movie.ReleaseDate = *database.ParseDate(traktdetails.Released)
	}
	if (movie.OriginalLanguage == "" || overwrite) && traktdetails.Language != "" {
		movie.OriginalLanguage = traktdetails.Language
	}
	if (movie.Tagline == "" || overwrite) && traktdetails.Tagline != "" {
		movie.Tagline = traktdetails.Tagline
	}
	traktdetails.Close()
}
func MovieGetMetadata(movie *database.Dbmovie, queryimdb bool, querytmdb bool, queryomdb bool, querytrakt bool) {
	//logger.LogAnyInfo("get metadata for", logger.LoggerValue{Name: "imdb", Value: movie.ImdbID})
	logger.Log.Info().Str(logger.StrTitle, movie.ImdbID).Msg("Get Metadata for")

	if queryimdb {
		MovieGetImdbMetadata(movie, false)
	}
	if querytmdb {
		MovieGetTmdbMetadata(movie, false)
	}
	if queryomdb {
		MovieGetOmdbMetadata(movie, false)
	}
	if querytrakt {
		MovieGetTraktMetadata(movie, false)
	}
	logger.Log.Info().Str("imdb", movie.ImdbID).Msg("ended get metadata for")
	//logger.LogAnyInfo("ended get metadata for", logger.LoggerValue{Name: "imdb", Value: movie.ImdbID})
}

func Getmoviemetadata(movie *database.Dbmovie, refresh bool) {
	if len(config.SettingsGeneral.MovieMetaSourcePriority) >= 1 {
		for idxmeta := range config.SettingsGeneral.MovieMetaSourcePriority {
			switch config.SettingsGeneral.MovieMetaSourcePriority[idxmeta] {
			case "imdb":
				MovieGetImdbMetadata(movie, refresh)
			case "tmdb":
				MovieGetTmdbMetadata(movie, false)
			case "omdb":
				MovieGetOmdbMetadata(movie, false)
			case "trakt":
				MovieGetTraktMetadata(movie, false)
			}
		}
	} else {
		MovieGetMetadata(movie, config.SettingsGeneral.MovieMetaSourceImdb, config.SettingsGeneral.MovieMetaSourceTmdb, config.SettingsGeneral.MovieMetaSourceOmdb, config.SettingsGeneral.MovieMetaSourceTrakt)
	}
}

func Getmoviemetatitles(movie *database.Dbmovie, cfgpstr string) {
	if config.SettingsMedia[cfgpstr].Name == "" {
		return
	}
	titles := database.QueryStaticStringArray(false,
		0, //QueryIntColumn(Querydatabase.DbmovieTitlesCountByDBID, movie.ID),
		database.QueryDbmovieTitlesGetTitleByID, &movie.ID)
	titlegroup := MovieGetTitles(movie, cfgpstr)
	//titles.Arr = slices.Grow(titles.Arr, len(titlegroup))
	for idx := range *titlegroup {
		if (*titlegroup)[idx].Title == "" {
			continue
		}
		if !logger.ContainsStringsI(titles, (*titlegroup)[idx].Title) {
			if config.SettingsGeneral.UseMediaCache {
				database.CacheTitlesMovie = append(database.CacheTitlesMovie, database.DbstaticTwoStringOneInt{Str1: (*titlegroup)[idx].Title, Str2: (*titlegroup)[idx].Slug, Num: int(movie.ID)})
			}
			//cache.Append(logger.GlobalCache, "database.DbmovieTitles_title_slug_cache", database.DbstaticTwoStringOneInt{Str1: titlegroup[idx].Title, Str2: titlegroup[idx].Slug, Num: int(movie.ID)})
			database.InsertStatic("Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)", (*titlegroup)[idx].Title, (*titlegroup)[idx].Slug, movie.ID, (*titlegroup)[idx].Region)
			*titles = append(*titles, (*titlegroup)[idx].Title)
		}
	}
	logger.Clear(titlegroup)
	logger.Clear(titles)
}

func SerieGetMetadataTmdb(serie *database.Dbserie, overwrite bool) error {
	if serie.ThetvdbID == 0 || (serie.Seriename != "" && !overwrite) {
		return errors.New("tvdb not found or no overwrite")
	}
	moviedb, err := apiexternal.TmdbAPI.FindTvdb(serie.ThetvdbID)
	if err != nil {
		return err
	}
	if len(moviedb.TvResults) == 0 {
		return errors.New("tmdb not found")
	}
	if (serie.Seriename == "" || overwrite) && moviedb.TvResults[0].Name != "" {
		serie.Seriename = moviedb.TvResults[0].Name
	}
	moviedb.Close()
	return nil
}
func SerieGetMetadataTrakt(serie *database.Dbserie, overwrite bool) error {
	if serie.ImdbID == "" {
		return errors.New("imdb empty")
	}
	traktdetails, err := apiexternal.TraktAPI.GetSerie(serie.ImdbID)
	if err != nil {
		return err
	}
	if (serie.Genre == "" || overwrite) && len(traktdetails.Genres) >= 1 {
		serie.Genre = strings.Join(traktdetails.Genres, ",")
	}
	if (serie.Language == "" || overwrite) && traktdetails.Language != "" {
		serie.Language = traktdetails.Language
	}
	if (serie.Network == "" || overwrite) && traktdetails.Network != "" {
		serie.Network = traktdetails.Network
	}
	if (serie.Overview == "" || overwrite) && traktdetails.Overview != "" {
		serie.Overview = traktdetails.Overview
	}
	if (serie.Rating == "" || overwrite) && traktdetails.Rating != 0 {
		serie.Rating = strconv.FormatFloat(float64(traktdetails.Rating), 'f', 4, 64) //fmt.Sprintf("%f", traktdetails.Rating)
	}
	if (serie.Runtime == "" || overwrite) && traktdetails.Runtime != 0 {
		if serie.Runtime != "0" && (traktdetails.Runtime == 1 || traktdetails.Runtime == 90) {
			logger.Log.Debug().Str("imdb", serie.ImdbID).Msg("skipped serie runtime for")
		} else {
			serie.Runtime = logger.IntToString(traktdetails.Runtime)
		}
	}
	if (serie.Seriename == "" || overwrite) && traktdetails.Title != "" {
		serie.Seriename = traktdetails.Title
	}
	if (serie.Slug == "" || overwrite) && traktdetails.Ids.Slug != "" {
		serie.Slug = traktdetails.Ids.Slug
	}
	if (serie.Status == "" || overwrite) && traktdetails.Status != "" {
		serie.Status = traktdetails.Status
	}
	if (serie.ThetvdbID == 0 || overwrite) && traktdetails.Ids.Tvdb != 0 {
		serie.ThetvdbID = traktdetails.Ids.Tvdb
	}
	if (serie.TraktID == 0 || overwrite) && traktdetails.Ids.Trakt != 0 {
		serie.TraktID = traktdetails.Ids.Trakt
	}
	if (serie.TvrageID == 0 || overwrite) && traktdetails.Ids.Tvrage != 0 {
		serie.TvrageID = traktdetails.Ids.Tvrage
	}
	if (serie.Firstaired == "" || overwrite) && traktdetails.FirstAired.String() != "" {
		serie.Firstaired = traktdetails.FirstAired.String()
	}
	traktdetails.Close()
	return nil
}
func SerieGetMetadataTvdb(serie *database.Dbserie, language string, overwrite bool) (*apiexternal.TheTVDBSeries, error) {
	if serie.ThetvdbID == 0 {
		return nil, errors.New("no tvdbid")
	}
	tvdbdetails, err := apiexternal.TvdbAPI.GetSeries(serie.ThetvdbID, language)
	if err != nil {
		return nil, err
	}
	if (serie.Seriename == "" || overwrite) && tvdbdetails.Data.SeriesName != "" {
		serie.Seriename = tvdbdetails.Data.SeriesName
	}
	if (serie.Aliases == "" || overwrite) && len(tvdbdetails.Data.Aliases) >= 1 {
		serie.Aliases = strings.Join(tvdbdetails.Data.Aliases, ",")
	}
	if (serie.Season == "" || overwrite) && tvdbdetails.Data.Season != "" {
		serie.Season = tvdbdetails.Data.Season
	}
	if (serie.Status == "" || overwrite) && tvdbdetails.Data.Status != "" {
		serie.Status = tvdbdetails.Data.Status
	}
	if (serie.Firstaired == "" || overwrite) && tvdbdetails.Data.FirstAired != "" {
		serie.Firstaired = tvdbdetails.Data.FirstAired
	}
	if (serie.Network == "" || overwrite) && tvdbdetails.Data.Network != "" {
		serie.Network = tvdbdetails.Data.Network
	}
	if (serie.Runtime == "" || overwrite) && tvdbdetails.Data.Runtime != "" {
		serie.Runtime = tvdbdetails.Data.Runtime
	}
	if (serie.Language == "" || overwrite) && tvdbdetails.Data.Language != "" {
		serie.Language = tvdbdetails.Data.Language
	}
	if (serie.Genre == "" || overwrite) && len(tvdbdetails.Data.Genre) >= 1 {
		serie.Genre = strings.Join(tvdbdetails.Data.Genre, ",")
	}
	if (serie.Overview == "" || overwrite) && tvdbdetails.Data.Overview != "" {
		serie.Overview = tvdbdetails.Data.Overview
	}
	if (serie.Rating == "" || overwrite) && tvdbdetails.Data.Rating != "" {
		serie.Rating = tvdbdetails.Data.Rating
	}
	if (serie.Siterating == "" || overwrite) && tvdbdetails.Data.SiteRating != 0 {
		serie.Siterating = strconv.FormatFloat(float64(tvdbdetails.Data.SiteRating), 'f', 1, 32)
	}
	if (serie.SiteratingCount == "" || overwrite) && tvdbdetails.Data.SiteRatingCount != 0 {
		serie.SiteratingCount = logger.IntToString(tvdbdetails.Data.SiteRatingCount)
	}
	if (serie.Slug == "" || overwrite) && tvdbdetails.Data.Slug != "" {
		serie.Slug = tvdbdetails.Data.Slug
	}
	if (serie.Banner == "" || overwrite) && tvdbdetails.Data.Banner != "" {
		serie.Banner = tvdbdetails.Data.Banner
	}
	if (serie.Poster == "" || overwrite) && tvdbdetails.Data.Poster != "" {
		serie.Poster = tvdbdetails.Data.Poster
	}
	if (serie.Fanart == "" || overwrite) && tvdbdetails.Data.Fanart != "" {
		serie.Fanart = tvdbdetails.Data.Fanart
	}
	if (serie.ImdbID == "" || overwrite) && tvdbdetails.Data.ImdbID != "" {
		serie.ImdbID = tvdbdetails.Data.ImdbID
	}
	return tvdbdetails, nil
}
func SerieGetMetadata(serie *database.Dbserie, language string, querytmdb bool, querytrakt bool, overwrite bool, returnaliases bool) (*apiexternal.TheTVDBSeries, error) {
	aliases, err := SerieGetMetadataTvdb(serie, language, overwrite)
	if err != nil {
		//logger.LogAnyError(err, "get tvdb metadata", logger.LoggerValue{Name: "id", Value: serie.ID})
		logger.Log.Error().Err(err).Uint("ID", serie.ID).Msg("Get Tvdb data")
	}

	if querytmdb {
		err = SerieGetMetadataTmdb(serie, false)
		if err != nil {
			//logger.LogAnyError(err, "get tmdb metadata", logger.LoggerValue{Name: "id", Value: serie.ID})
			logger.Log.Error().Err(err).Uint("ID", serie.ID).Msg("Get Tmdb data")
		}
	}
	if querytrakt && serie.ImdbID != "" {
		err = SerieGetMetadataTrakt(serie, false)
		if err != nil {
			//logger.LogAnyError(err, "get trakt metadata", logger.LoggerValue{Name: "id", Value: serie.ID})
			logger.Log.Error().Err(err).Uint("ID", serie.ID).Msg("Get Trakt data")
		} else {
			if returnaliases {
				traktaliases, err := apiexternal.TraktAPI.GetSerieAliases(serie.ImdbID)

				if err == nil && traktaliases != nil && len(*traktaliases) >= 1 {
					lenarr := len(config.SettingsImdb.Indexedlanguages)
					for idxalias := range *traktaliases {
						if logger.ContainsStringsI(&aliases.Data.Aliases, (*traktaliases)[idxalias].Title) {
							continue
						}
						if lenarr >= 1 && logger.ContainsStringsI(&config.SettingsImdb.Indexedlanguages, (*traktaliases)[idxalias].Country) {
							aliases.Data.Aliases = append(aliases.Data.Aliases, (*traktaliases)[idxalias].Title)
						}
					}
				}
				logger.Clear(traktaliases)
			}
		}
	}
	return aliases, err
}

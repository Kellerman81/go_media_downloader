package metadata

import (
	"errors"
	"html"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

// checkaddmovietitlewoslug adds a movie title to the dbmovie_titles table if it does not already exist.
// It takes the title string, movie ID uint, region string, and current movie titles slice.
// It returns nothing.
// It checks if the title already exists for that movie ID, slugifies the title,
// inserts into dbmovie_titles if it does not exist, and updates the cache if enabled.
func checkaddmovietitlewoslug(addentry database.DbstaticTwoStringOneInt, region string, titles []database.DbstaticTwoString) {
	if addentry.Str1 == "" {
		return
	}

	if database.GetDbStaticTwoStringIdx1(titles, addentry.Str1) != -1 {
		return
	}
	addentry.Str2 = logger.StringToSlug(addentry.Str1)
	if database.GetdatarowN[int](false, "select count() from dbmovie_titles where dbmovie_id = ? and title = ? COLLATE NOCASE", &addentry.Num, &addentry.Str1) == 0 {
		database.ExecN("Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)", &addentry.Str1, &addentry.Str2, &addentry.Num, &region)
		if config.SettingsGeneral.UseMediaCache {
			database.AppendTwoStringIntCache(logger.CacheTitlesMovie, addentry)
		}
	}
}

// movieGetImdbMetadata fetches movie metadata from IMDB.
// It takes a pointer to a Dbmovie struct and a bool indicating
// whether to overwrite existing data.
// It adds the "tt" prefix to the IMDB ID if missing, fetches
// the title from the IMDB API, and clears temporary variables.
func movieGetImdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)
	database.GetImdbTitle(&movie.ImdbID, movie, overwrite)
}

// movieGetTmdbMetadata fetches movie metadata from TMDb.
// It takes a pointer to a Dbmovie struct and a bool indicating whether to overwrite existing data.
// It finds the TMDb ID if missing using the IMDb ID.
// It fetches details from the TMDb API and populates Dbmovie fields if empty or overwrite is true.
// It closes the TMDb response when done.
func movieGetTmdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.MoviedbID == 0 {
		if movie.ImdbID == "" {
			return
		}
		moviedb, err := apiexternal.FindTmdbImdb(movie.ImdbID)
		if err != nil {
			return
		}
		if len(moviedb) == 0 {
			return
		}
		movie.MoviedbID = moviedb[0].ID
		clear(moviedb)
	}
	moviedbdetails, err := apiexternal.GetTmdbMovie(movie.MoviedbID)
	if err != nil {
		return
	}
	if (!movie.Adult && moviedbdetails.Adult) || overwrite {
		movie.Adult = moviedbdetails.Adult
	}
	if (movie.Title == "" || overwrite) && moviedbdetails.Title != "" {
		if strings.ContainsRune(moviedbdetails.Title, '&') {
			movie.Title = html.UnescapeString(moviedbdetails.Title)
		}
		movie.Title = logger.UnquoteUnescape(movie.Title)
	}
	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}
	if (movie.Budget == 0 || overwrite) && moviedbdetails.Budget != 0 {
		movie.Budget = moviedbdetails.Budget
	}
	if moviedbdetails.ReleaseDate != "" && !movie.ReleaseDate.Valid {
		movie.ReleaseDate = database.ParseDate(moviedbdetails.ReleaseDate)
		if (movie.Year == 0 || overwrite) && movie.ReleaseDate.Time.Year() != 0 {
			movie.Year = movie.ReleaseDate.Time.Year()
		}
	}
	if (movie.Genres == "" || overwrite) && len(moviedbdetails.Genres) != 0 {
		bldgenre := logger.PlBuffer.Get()
		for idx := range moviedbdetails.Genres {
			if movie.Genres != "" {
				bldgenre.WriteRune(',')
			}
			bldgenre.WriteString(moviedbdetails.Genres[idx].Name)
		}
		movie.Genres = bldgenre.String()
		logger.PlBuffer.Put(bldgenre)
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
	if (movie.Runtime == 0 || movie.Runtime == 1 || movie.Runtime == 2 || movie.Runtime == 3 || movie.Runtime == 60 || movie.Runtime == 90 || movie.Runtime == 120 || overwrite) && moviedbdetails.Runtime != 0 {
		if movie.Runtime != 0 && (moviedbdetails.Runtime == 1 || moviedbdetails.Runtime == 2 || moviedbdetails.Runtime == 3 || moviedbdetails.Runtime == 4 || moviedbdetails.Runtime == 60 || moviedbdetails.Runtime == 90 || moviedbdetails.Runtime == 120) {
			logger.LogDynamic("debug", "skipped moviedb movie runtime for", logger.NewLogField(logger.StrImdb, movie.ImdbID))
		} else {
			movie.Runtime = moviedbdetails.Runtime
		}
	}
	if (movie.SpokenLanguages == "" || overwrite) && len(moviedbdetails.SpokenLanguages) != 0 {
		bldlang := logger.PlBuffer.Get()
		for idx := range moviedbdetails.SpokenLanguages {
			if movie.SpokenLanguages != "" {
				bldlang.WriteRune(',')
			}
			bldlang.WriteString(moviedbdetails.SpokenLanguages[idx].EnglishName)
		}
		movie.SpokenLanguages = bldlang.String()
		logger.PlBuffer.Put(bldlang)
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

// movieGetOmdbMetadata retrieves movie metadata from the OMDB API and merges it into the provided Dbmovie struct.
// It will overwrite existing data in the Dbmovie if the overwrite param is true.
// The OMDB API is queried using the ImdbID field in the Dbmovie.
func movieGetOmdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	omdbdetails, err := apiexternal.GetOmdbMovie(movie.ImdbID)
	if err != nil {
		return
	}
	if (movie.Title == "" || overwrite) && omdbdetails.Title != "" {
		movie.Title = omdbdetails.Title
		if strings.ContainsRune(movie.Title, '&') {
			movie.Title = html.UnescapeString(movie.Title)
		}
		movie.Title = logger.UnquoteUnescape(movie.Title)
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
	omdbdetails.Close()
}

// movieGetTraktMetadata retrieves movie metadata from the Trakt API and merges it into the provided Dbmovie struct.
// It will overwrite existing data in the Dbmovie if the overwrite param is true.
// The Trakt API is queried using the ImdbID field in the Dbmovie.
func movieGetTraktMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}
	traktdetails, err := apiexternal.GetTraktMovie(movie.ImdbID)
	if err != nil {
		return
	}
	if (movie.Title == "" || overwrite) && traktdetails.Title != "" {
		movie.Title = traktdetails.Title
		if strings.ContainsRune(movie.Title, '&') {
			movie.Title = html.UnescapeString(movie.Title)
		}
		movie.Title = logger.UnquoteUnescape(movie.Title)
	}
	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
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
	if (movie.Runtime == 0 || movie.Runtime == 1 || movie.Runtime == 2 || movie.Runtime == 3 || movie.Runtime == 60 || movie.Runtime == 90 || movie.Runtime == 120 || overwrite) && traktdetails.Runtime != 0 {
		if movie.Runtime != 0 && (traktdetails.Runtime == 1 || traktdetails.Runtime == 2 || traktdetails.Runtime == 3 || traktdetails.Runtime == 4 || traktdetails.Runtime == 60 || traktdetails.Runtime == 90 || traktdetails.Runtime == 120) {
			logger.LogDynamic("debug", "skipped trakt movie runtime for", logger.NewLogField(logger.StrImdb, movie.ImdbID))
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
		movie.ReleaseDate = database.ParseDate(traktdetails.Released)
	}
	if (movie.OriginalLanguage == "" || overwrite) && traktdetails.Language != "" {
		movie.OriginalLanguage = traktdetails.Language
	}
	if (movie.Tagline == "" || overwrite) && traktdetails.Tagline != "" {
		movie.Tagline = traktdetails.Tagline
	}
	traktdetails.Close()
}

// movieGetMetadata retrieves metadata for the given movie from multiple sources based on the input flags.
// It queries IMDb, TMDb, OMDb and Trakt APIs based on the queryimdb, querytmdb, queryomdb and querytrakt flags passed in.
// Results from each source are cached and merged into the movie struct.
func MovieGetMetadata(movie *database.Dbmovie, queryimdb bool, querytmdb bool, queryomdb bool, querytrakt bool) {
	logger.LogDynamic("info", "Get Metadata for", logger.NewLogField(logger.StrTitle, movie.ImdbID))

	if queryimdb {
		movieGetImdbMetadata(movie, false)
	}
	if querytmdb {
		movieGetTmdbMetadata(movie, false)
	}
	if queryomdb {
		movieGetOmdbMetadata(movie, false)
	}
	if querytrakt {
		movieGetTraktMetadata(movie, false)
	}
	logger.LogDynamic("info", "ended get metadata for", logger.NewLogField(logger.StrImdb, movie.ImdbID))
}

// Getmoviemetadata retrieves metadata for the given movie from the configured
// priority of metadata sources, refreshing cached data if refresh is true.
func Getmoviemetadata(movie *database.Dbmovie, refresh bool) {
	for idx := range config.SettingsGeneral.MovieMetaSourcePriority {
		switch config.SettingsGeneral.MovieMetaSourcePriority[idx] {
		case logger.StrImdb:
			movieGetImdbMetadata(movie, refresh)
		case "tmdb":
			movieGetTmdbMetadata(movie, false)
		case "omdb":
			movieGetOmdbMetadata(movie, false)
		case "trakt":
			movieGetTraktMetadata(movie, false)
		}
	}
}

// Getmoviemetatitles retrieves alternate titles for a movie from various metadata
// sources like IMDb, TMDb, and Trakt based on configured settings. It adds any
// unique titles to the database.
func Getmoviemetatitles(movie *database.Dbmovie, cfgp *config.MediaTypeConfig) {
	if cfgp.Name == "" {
		return
	}

	//size +5
	titles := database.Getrows1size[database.DbstaticTwoString](false, "select count() from dbmovie_titles where dbmovie_id = ?",
		"select title, slug from dbmovie_titles where dbmovie_id = ?", &movie.ID)

	lenarr := cfgp.MetadataTitleLanguagesLen
	if config.SettingsGeneral.MovieAlternateTitleMetaSourceImdb && movie.ImdbID != "" {
		movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)
		arr := database.Getrows1size[database.DbstaticThreeString](true, "select count() from imdb_akas where tconst = ?", "select region, title, slug from imdb_akas where tconst = ?", &movie.ImdbID)
		var getid int
		addentry := database.DbstaticTwoStringOneInt{Num: int(movie.ID)}
		for idx := range arr {
			if lenarr >= 1 {
				if arr[idx].Str1 != "" && !logger.SlicesContainsI(cfgp.MetadataTitleLanguages, arr[idx].Str1) {
					//if !testlanguage(cfgp, arr[idx].Str1) {
					continue
				}
			}
			if arr[idx].Str2 == "" {
				continue
			}

			if database.GetDbStaticTwoStringIdx1(titles, arr[idx].Str2) != -1 {
				continue
			}
			if arr[idx].Str3 == "" {
				arr[idx].Str3 = logger.StringToSlug(arr[idx].Str2)
			}
			if _ = database.ScanrowsNdyn(false, "select count() from dbmovie_titles where dbmovie_id = ? and title = ? COLLATE NOCASE", &getid, &movie.ID, &arr[idx].Str2); getid == 0 {
				database.ExecN("Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)", &arr[idx].Str2, &arr[idx].Str3, &movie.ID, &arr[idx].Str1)
				if config.SettingsGeneral.UseMediaCache {
					addentry.Str1 = arr[idx].Str2
					addentry.Str2 = arr[idx].Str3
					database.AppendTwoStringIntCache(logger.CacheTitlesMovie, addentry)
				}
			}
		}
		clear(arr)
	}
	var addentry database.DbstaticTwoStringOneInt
	addentry.Num = int(movie.ID)
	if config.SettingsGeneral.MovieAlternateTitleMetaSourceTmdb && movie.MoviedbID != 0 {
		tbl, err := apiexternal.GetTmdbMovieTitles(movie.MoviedbID)
		if err == nil {
			for idx := range tbl {
				if lenarr >= 1 {
					if tbl[idx].Iso31661 != "" && !logger.SlicesContainsI(cfgp.MetadataTitleLanguages, tbl[idx].Iso31661) {
						//if !testlanguage(cfgp, tbl.Titles[idx].Iso31661) {
						continue
					}
				}
				addentry.Str1 = tbl[idx].Title
				checkaddmovietitlewoslug(addentry, tbl[idx].Iso31661, titles)
			}
			clear(tbl)
		}
	}
	if config.SettingsGeneral.MovieAlternateTitleMetaSourceTrakt && movie.ImdbID != "" {
		arr := apiexternal.GetTraktMovieAliases(movie.ImdbID)
		for idx := range arr {
			if lenarr >= 1 {
				if arr[idx].Country != "" && !logger.SlicesContainsI(cfgp.MetadataTitleLanguages, arr[idx].Country) {
					//if !testlanguage(cfgp, arr[idx].Country) {
					continue
				}
			}
			addentry.Str1 = arr[idx].Title
			checkaddmovietitlewoslug(addentry, arr[idx].Country, titles)
		}
		clear(arr)
	}
	clear(titles)
}

// serieGetMetadataTmdb queries TheMovieDB API to get metadata for the given serie.
// It populates various serie fields like name, slug etc if empty or overwrite is set.
// It handles API errors and logs them.
func serieGetMetadataTmdb(serie *database.Dbserie, overwrite bool) error {
	if serie.ThetvdbID == 0 || (serie.Seriename != "" && !overwrite) {
		return logger.ErrTvdbEmpty
	}
	moviedb, err := apiexternal.FindTmdbTvdb(serie.ThetvdbID)
	if err != nil {
		return err
	}
	if len(moviedb) == 0 {
		return errors.New("tmdb not found")
	}
	if (serie.Seriename == "" || overwrite) && moviedb[0].Name != "" {
		serie.Seriename = moviedb[0].Name
	}
	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlug(serie.Seriename)
	}
	return nil
}

// serieGetMetadataTrakt queries the Trakt API to get metadata for the given serie.
// It populates various serie fields like name, status, genres etc if empty or overwrite is set.
// It handles API errors and logs them.
func serieGetMetadataTrakt(serie *database.Dbserie, overwrite bool) error {
	if serie.ImdbID == "" {
		return logger.ErrImdbEmpty
	}
	traktdetails, err := apiexternal.GetTraktSerie(serie.ImdbID)
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
			logger.LogDynamic("debug", "skipped serie runtime for", logger.NewLogField(logger.StrImdb, serie.ImdbID))
		} else {
			serie.Runtime = strconv.Itoa(traktdetails.Runtime)
		}
	}
	if (serie.Seriename == "" || overwrite) && traktdetails.Title != "" {
		serie.Seriename = traktdetails.Title
	}
	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlug(serie.Seriename)
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

// serieGetMetadataTvdb queries TheTVDB API to get metadata for the given serie.
// It takes a pointer to a Dbserie, language, overwrite flag, and existing aliases.
// It returns updated aliases after appending new ones from TheTVDB.
// It populates serie fields like name, status, runtime etc if empty or overwrite is set.
// It handles API errors and logs them.
func serieGetMetadataTvdb(serie *database.Dbserie, language string, overwrite bool, aliases []string) ([]string, error) {
	if serie.ThetvdbID == 0 {
		return aliases, logger.ErrTvdbEmpty
	}
	tvdbdetails, err := apiexternal.GetTvdbSeries(serie.ThetvdbID, language)
	if err != nil {
		if language != "" {
			tvdbdetails, err = apiexternal.GetTvdbSeries(serie.ThetvdbID, "")
		}
		if err != nil {
			return aliases, err
		}
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
		serie.SiteratingCount = strconv.Itoa(tvdbdetails.Data.SiteRatingCount)
	}
	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlug(serie.Seriename)
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
	aliases = append(aliases, tvdbdetails.Data.Aliases...)

	tvdbdetails.Close()
	return aliases, nil
}

// SerieGetMetadata retrieves metadata for the given serie from various sources like
// TVDB, TMDB, Trakt etc. based on the provided flags. It takes in a pointer to a
// Dbserie struct, language string, flags to query TMDB and Trakt APIs, a flag to
// overwrite existing metadata, and a slice of existing aliases.

// It returns a slice of aliases after querying the different metadata sources. The
// function handles errors from the API calls and logs them. It progressively
// queries more APIs, using data from previous ones. Overall it populates serie
// metadata like status, first air date, network etc. from TVDB, and collects
// aliases from Trakt using the IMDB ID.
func SerieGetMetadata(serie *database.Dbserie, language string, querytmdb, querytrakt, overwrite bool, aliases []string) []string {
	logger.LogDynamic("info", "Get Metadata for", logger.NewLogField(logger.StrTitle, serie.Seriename))
	defer logger.LogDynamic("info", "ended get metadata for", logger.NewLogField(logger.StrTitle, serie.Seriename))

	aliases, err := serieGetMetadataTvdb(serie, language, overwrite, aliases)
	if err != nil {
		logger.LogDynamic("error", "Get Tvdb data", logger.NewLogFieldValue(err), logger.NewLogField("ID", serie.ID))
	}
	if querytmdb && serie.ThetvdbID != 0 {
		err = serieGetMetadataTmdb(serie, false)
		if err != nil {
			logger.LogDynamic("error", "Get Tmdb data", logger.NewLogFieldValue(err), logger.NewLogField("ID", serie.ID))
		}
	}
	if querytrakt && serie.ImdbID != "" {
		err = serieGetMetadataTrakt(serie, false)
		if err != nil {
			logger.LogDynamic("error", "Get Trakt data", logger.NewLogFieldValue(err), logger.NewLogField("ID", serie.ID))
			return aliases
		}
		if len(config.SettingsImdb.Indexedlanguages) == 0 {
			return aliases
		}
		//labtraktalias:
		tbl := apiexternal.GetTraktSerieAliases(serie)
		if len(tbl) == 0 {
			return aliases
		}
		//aliases = slices.Grow(aliases, len(tbl))
		for idx := range tbl {
			if !logger.SlicesContainsI(aliases, tbl[idx].Title) && logger.SlicesContainsI(config.SettingsImdb.Indexedlanguages, tbl[idx].Country) {
				aliases = append(aliases, tbl[idx].Title)
			}
		}
		clear(tbl)
	}
	return aliases
}

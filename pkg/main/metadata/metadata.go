package metadata

import (
	"database/sql"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

var (
	errTmdbNotFound = errors.New("tmdb not found")

	// Common runtime values that should be skipped.
	invalidRuntimes = []int{1, 2, 3, 4, 60, 90, 120}
)

// checkaddmovietitlewoslug adds a movie title to the dbmovie_titles table if it does not already exist.
// It takes the title string, movie ID uint, region string, and current movie titles slice.
// It returns nothing.
// It checks if the title already exists for that movie ID, slugifies the title,
// inserts into dbmovie_titles if it does not exist, and updates the cache if enabled.
func checkaddmovietitlewoslug(
	checkid *int,
	dbmovieid *uint,
	title *string,
	region *string,
	titles []database.DbstaticTwoString,
) {
	if title == nil || *title == "" || database.GetDBStaticTwoStringIdx1(titles, *title) != -1 {
		return
	}
	database.Scanrowsdyn(
		false,
		"select count() from dbmovie_titles where dbmovie_id = ? and title = ? COLLATE NOCASE",
		checkid,
		dbmovieid,
		title,
	)
	if *checkid == 0 {
		slug := logger.StringToSlug(*title)
		database.ExecN(
			"Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)",
			title,
			&slug,
			dbmovieid,
			region,
		)
		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoString(
				logger.CacheTitlesMovie,
				database.DbstaticTwoStringOneInt{Num: *dbmovieid, Str1: *title, Str2: slug},
			)
		}
	}
}

// ParseDate parses a date string in "2006-01-02" format and returns a sql.NullTime.
// Returns a null sql.NullTime if the date string is empty or fails to parse.
func parseDate(date string) sql.NullTime {
	if date == "" {
		return sql.NullTime{}
	}
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: t, Valid: true}
}

// isValidRuntime checks if a runtime value is valid (not in the invalid list).
func isValidRuntime(runtime int) bool {
	return !slices.Contains(invalidRuntimes, runtime)
}

// shouldUpdateRuntime determines if runtime should be updated based on current and new values.
func shouldUpdateRuntime(currentRuntime, newRuntime int, overwrite bool) bool {
	if newRuntime == 0 {
		return false
	}

	// Always update if overwrite is true and new runtime is valid
	if overwrite && isValidRuntime(newRuntime) {
		return true
	}

	// Update if current runtime is invalid and new runtime is valid
	if !isValidRuntime(currentRuntime) && isValidRuntime(newRuntime) {
		return true
	}

	// Update if current runtime is 0
	return currentRuntime == 0 && isValidRuntime(newRuntime)
}

// buildGenreString efficiently builds a comma-separated genre string.
func buildGenreString(genres []apiexternal.TheMovieDBMovieGenres) string {
	if len(genres) == 0 {
		return ""
	}

	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	for i, genre := range genres {
		if i > 0 {
			bld.WriteByte(',')
		}
		bld.WriteString(genre.Name)
	}

	return bld.String()
}

// buildLanguageString efficiently builds a comma-separated language string.
func buildLanguageString(languages []apiexternal.TheMovieDBMovieLanguages) string {
	if len(languages) == 0 {
		return ""
	}

	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	for i, lang := range languages {
		if i > 0 {
			bld.WriteByte(',')
		}
		bld.WriteString(lang.EnglishName)
	}

	return bld.String()
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
		if err != nil || len(moviedb.MovieResults) == 0 {
			return
		}
		movie.MoviedbID = moviedb.MovieResults[0].ID
	}
	moviedbdetails, err := apiexternal.GetTmdbMovie(movie.MoviedbID)
	if err != nil {
		return
	}

	updateStringField(&movie.Title, moviedbdetails.Title, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})

	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}

	updateBoolField(&movie.Adult, moviedbdetails.Adult, overwrite)
	updateIntField(&movie.Budget, moviedbdetails.Budget, overwrite)
	updateFloatField(&movie.Popularity, moviedbdetails.Popularity, overwrite)
	updateIntField(&movie.Revenue, moviedbdetails.Revenue, overwrite)
	updateInt32Field(&movie.VoteCount, moviedbdetails.VoteCount, overwrite)
	updateFloatField(&movie.VoteAverage, moviedbdetails.VoteAverage, overwrite)

	updateStringField(&movie.OriginalLanguage, moviedbdetails.OriginalLanguage, overwrite, nil)
	updateStringField(&movie.OriginalTitle, moviedbdetails.OriginalTitle, overwrite, nil)
	updateStringField(&movie.Overview, moviedbdetails.Overview, overwrite, nil)
	updateStringField(&movie.Status, moviedbdetails.Status, overwrite, nil)
	updateStringField(&movie.Tagline, moviedbdetails.Tagline, overwrite, nil)
	updateStringField(&movie.Poster, moviedbdetails.Poster, overwrite, nil)
	updateStringField(&movie.Backdrop, moviedbdetails.Backdrop, overwrite, nil)

	// Handle release date and year
	if moviedbdetails.ReleaseDate != "" && !movie.ReleaseDate.Valid {
		movie.ReleaseDate = parseDate(moviedbdetails.ReleaseDate)
		if (movie.Year == 0 || overwrite) && movie.ReleaseDate.Time.Year() != 0 {
			movie.Year = uint16(movie.ReleaseDate.Time.Year())
		}
	}

	// Handle genres
	if (movie.Genres == "" || overwrite) && len(moviedbdetails.Genres) > 0 {
		movie.Genres = buildGenreString(moviedbdetails.Genres)
	}

	// Handle spoken languages
	if (movie.SpokenLanguages == "" || overwrite) && len(moviedbdetails.SpokenLanguages) > 0 {
		movie.SpokenLanguages = buildLanguageString(moviedbdetails.SpokenLanguages)
	}

	// Handle runtime with validation
	if shouldUpdateRuntime(movie.Runtime, moviedbdetails.Runtime, overwrite) {
		if movie.Runtime != 0 && !isValidRuntime(moviedbdetails.Runtime) {
			logger.LogDynamicany1String(
				"debug",
				"skipped moviedb movie runtime for",
				logger.StrImdb,
				movie.ImdbID,
			)
		} else {
			movie.Runtime = moviedbdetails.Runtime
		}
	}

	if (movie.MoviedbID == 0 || overwrite) && moviedbdetails.ID != 0 {
		movie.MoviedbID = moviedbdetails.ID
	}
}

// updateStringField updates a string field with a new value if the current field is empty or overwrite is true.
// It allows an optional transformation function to modify the new value before assignment.
// If no transform function is provided, the new value is assigned directly.
// The update occurs only when the new value is non-empty.
func updateStringField(
	field *string,
	newValue string,
	overwrite bool,
	transform func(string) string,
) {
	if (*field == "" || overwrite) && newValue != "" {
		if transform != nil {
			*field = transform(newValue)
		} else {
			*field = newValue
		}
	}
}

// updateBoolField updates a bool field with a new value if the current field is false or overwrite is true.
// It ensures that only true values are used to update the field, with an optional overwrite behavior.
func updateBoolField(field *bool, newValue bool, overwrite bool) {
	if (!*field && newValue) || overwrite {
		*field = newValue
	}
}

// updateIntField updates an int field with a new value if the current field is zero or overwrite is true.
// It ensures that only non-zero values are used to update the field, with an optional overwrite behavior.
func updateIntField(field *int, newValue int, overwrite bool) {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
	}
}

// updateInt32Field updates an int32 field with a new value if the current field is zero or overwrite is true.
// It ensures that only non-zero values are used to update the field, with an optional overwrite behavior.
func updateInt32Field(field *int32, newValue int32, overwrite bool) {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
	}
}

// updateFloatField updates a float32 field with a new value if the current field is zero or overwrite is true.
// It ensures that only non-zero values are used to update the field, with an optional overwrite behavior.
func updateFloatField(field *float32, newValue float32, overwrite bool) {
	if (*field == 0 || overwrite) && newValue != 0 {
		*field = newValue
	}
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
	updateStringField(&movie.Title, omdbdetails.Title, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})

	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}

	updateStringField(&movie.Genres, omdbdetails.Genre, overwrite, nil)
	updateStringField(&movie.URL, omdbdetails.Website, overwrite, nil)
	updateStringField(&movie.Overview, omdbdetails.Plot, overwrite, nil)

	if (movie.VoteCount == 0 || overwrite) && omdbdetails.ImdbVotes != "" {
		movie.VoteCount = logger.StringToInt32(omdbdetails.ImdbVotes)
	}

	if (movie.VoteAverage == 0 || overwrite) && omdbdetails.ImdbRating != "" {
		movie.VoteAverage = float32(logger.StringToInt(omdbdetails.ImdbRating))
	}

	if (movie.Year == 0 || overwrite) && omdbdetails.Year != "" {
		movie.Year = logger.StringToUInt16(omdbdetails.Year)
	}

	if (movie.Runtime == 0 || overwrite) && omdbdetails.Runtime != "" {
		movie.Runtime = logger.StringToInt(omdbdetails.Runtime)
	}
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
	updateStringField(&movie.Title, traktdetails.Title, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})

	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlug(movie.Title)
	}

	if (movie.Genres == "" || overwrite) && len(traktdetails.Genres) > 0 {
		movie.Genres = strings.Join(traktdetails.Genres, ",")
	}

	updateInt32Field(&movie.VoteCount, traktdetails.Votes, overwrite)
	updateFloatField(&movie.VoteAverage, traktdetails.Rating, overwrite)
	updateStringField(&movie.Overview, traktdetails.Overview, overwrite, nil)
	updateStringField(&movie.Status, traktdetails.Status, overwrite, nil)
	updateStringField(&movie.OriginalLanguage, traktdetails.Language, overwrite, nil)
	updateStringField(&movie.Tagline, traktdetails.Tagline, overwrite, nil)

	if (movie.Year == 0 || overwrite) && traktdetails.Year != 0 {
		movie.Year = traktdetails.Year
	}

	// Handle runtime with validation
	if shouldUpdateRuntime(movie.Runtime, traktdetails.Runtime, overwrite) {
		if movie.Runtime != 0 && !isValidRuntime(traktdetails.Runtime) {
			logger.LogDynamicany1String(
				"debug",
				"skipped trakt movie runtime for",
				logger.StrImdb,
				movie.ImdbID,
			)
		} else {
			movie.Runtime = traktdetails.Runtime
		}
	}

	if (movie.MoviedbID == 0 || overwrite) && traktdetails.IDs.Tmdb != 0 {
		movie.MoviedbID = traktdetails.IDs.Tmdb
	}

	if (movie.TraktID == 0 || overwrite) && traktdetails.IDs.Trakt != 0 {
		movie.TraktID = traktdetails.IDs.Trakt
	}

	if (!movie.ReleaseDate.Valid || overwrite) && traktdetails.Released != "" {
		movie.ReleaseDate = parseDate(traktdetails.Released)
	}
}

// MovieGetMetadata retrieves metadata for the given movie from multiple sources based on the input flags.
// It queries IMDb, TMDb, OMDb and Trakt APIs based on the queryimdb, querytmdb, queryomdb and querytrakt flags passed in.
// Results from each source are cached and merged into the movie struct.
func MovieGetMetadata(movie *database.Dbmovie, queryimdb, querytmdb, queryomdb, querytrakt bool) {
	logger.LogDynamicany1String("info", "Get Metadata for", logger.StrTitle, movie.ImdbID)
	defer logger.LogDynamicany1String(
		"info",
		"ended get metadata for",
		logger.StrImdb,
		movie.ImdbID,
	)
	if queryimdb {
		movie.MovieGetImdbMetadata(false)
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
}

// Getmoviemetadata retrieves metadata for the given movie from the configured
// priority of metadata sources, refreshing cached data if refresh is true.
func Getmoviemetadata(movie *database.Dbmovie, refresh bool) {
	for idx := range config.GetSettingsGeneral().MovieMetaSourcePriority {
		switch config.GetSettingsGeneral().MovieMetaSourcePriority[idx] {
		case logger.StrImdb:
			movie.MovieGetImdbMetadata(refresh)
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

	// size +5
	titles := database.Getrowssize[database.DbstaticTwoString](
		false,
		"select count() from dbmovie_titles where dbmovie_id = ?",
		"select title, slug from dbmovie_titles where dbmovie_id = ?",
		&movie.ID,
	)

	var checkid int

	// Process IMDb alternate titles
	if config.GetSettingsGeneral().MovieAlternateTitleMetaSourceImdb && movie.ImdbID != "" {
		processImdbAlternateTitles(movie, cfgp, titles, &checkid)
	}

	// Process TMDb alternate titles
	if config.GetSettingsGeneral().MovieAlternateTitleMetaSourceTmdb && movie.MoviedbID != 0 {
		processTmdbAlternateTitles(movie, cfgp, titles, &checkid)
	}

	// Process Trakt alternate titles
	if config.GetSettingsGeneral().MovieAlternateTitleMetaSourceTrakt && movie.ImdbID != "" {
		processTraktAlternateTitles(movie, cfgp, titles, &checkid)
	}
}

// processImdbAlternateTitles processes alternate titles from IMDb.
func processImdbAlternateTitles(
	movie *database.Dbmovie,
	cfgp *config.MediaTypeConfig,
	titles []database.DbstaticTwoString,
	checkid *int,
) {
	movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)

	arr := database.Getrowssize[database.DbstaticThreeString](
		true,
		"select count() from imdb_akas where tconst = ?",
		"select region, title, slug from imdb_akas where tconst = ?",
		&movie.ImdbID,
	)

	for idx := range arr {
		aka := &arr[idx]
		if !shouldProcessTitle(aka.Str1, aka.Str2, cfgp, titles) {
			continue
		}

		if aka.Str3 == "" {
			aka.Str3 = logger.StringToSlug(aka.Str2)
		}

		insertMovieTitle(checkid, &movie.ID, &aka.Str2, &aka.Str3, &aka.Str1)
	}
}

// processTmdbAlternateTitles processes alternate titles from TMDb.
func processTmdbAlternateTitles(
	movie *database.Dbmovie,
	cfgp *config.MediaTypeConfig,
	titles []database.DbstaticTwoString,
	checkid *int,
) {
	tbl, err := apiexternal.GetTmdbMovieTitles(movie.MoviedbID)
	if err != nil {
		return
	}

	for idx := range tbl.Titles {
		title := &tbl.Titles[idx]
		if !shouldProcessTitle(title.Iso31661, title.Title, cfgp, titles) {
			continue
		}

		checkaddmovietitlewoslug(checkid, &movie.ID, &title.Title, &title.Iso31661, titles)
	}
}

// processTraktAlternateTitles processes alternate titles from Trakt.
func processTraktAlternateTitles(
	movie *database.Dbmovie,
	cfgp *config.MediaTypeConfig,
	titles []database.DbstaticTwoString,
	checkid *int,
) {
	arr := apiexternal.GetTraktMovieAliases(movie.ImdbID)

	for idx := range arr {
		alias := &arr[idx]
		if !shouldProcessTitle(alias.Country, alias.Title, cfgp, titles) {
			continue
		}

		checkaddmovietitlewoslug(checkid, &movie.ID, &alias.Title, &alias.Country, titles)
	}
}

// shouldProcessTitle checks if a title should be processed based on language filters.
func shouldProcessTitle(
	region, title string,
	cfgp *config.MediaTypeConfig,
	titles []database.DbstaticTwoString,
) bool {
	if title == "" || database.GetDBStaticTwoStringIdx1(titles, title) != -1 {
		return false
	}

	if cfgp.MetadataTitleLanguagesLen >= 1 && region != "" {
		return logger.SlicesContainsI(cfgp.MetadataTitleLanguages, region)
	}

	return true
}

// insertMovieTitle inserts a movie title into the database.
func insertMovieTitle(checkid *int, movieID *uint, title, slug, region *string) {
	database.Scanrowsdyn(
		false,
		"select count() from dbmovie_titles where dbmovie_id = ? and title = ? COLLATE NOCASE",
		checkid,
		movieID,
		title,
	)

	if *checkid == 0 {
		database.ExecN(
			"Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)",
			title,
			slug,
			movieID,
			region,
		)

		if config.GetSettingsGeneral().UseMediaCache {
			database.AppendCacheTwoString(
				logger.CacheTitlesMovie,
				database.DbstaticTwoStringOneInt{
					Num:  *movieID,
					Str1: *title,
					Str2: *slug,
				},
			)
		}
	}
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
	if len(moviedb.TvResults) == 0 {
		return errTmdbNotFound
	}
	updateStringField(&serie.Seriename, moviedb.TvResults[0].Name, overwrite, nil)
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
	updateStringField(&serie.Seriename, traktdetails.Title, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})
	updateStringField(&serie.Language, traktdetails.Language, overwrite, nil)
	updateStringField(&serie.Network, traktdetails.Network, overwrite, nil)
	updateStringField(&serie.Overview, traktdetails.Overview, overwrite, nil)
	updateStringField(&serie.Status, traktdetails.Status, overwrite, nil)

	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlug(serie.Seriename)
	}

	// Handle genres
	if (serie.Genre == "" || overwrite) && len(traktdetails.Genres) > 0 {
		serie.Genre = strings.Join(traktdetails.Genres, ",")
	}

	// Handle numeric fields with string conversion
	if (serie.Rating == "" || overwrite) && traktdetails.Rating != 0 {
		serie.Rating = strconv.FormatFloat(float64(traktdetails.Rating), 'f', 4, 64)
	}

	// Handle runtime with validation
	if shouldUpdateSerieRuntime(serie.Runtime, traktdetails.Runtime, overwrite) {
		if serie.Runtime != "0" && !isValidRuntime(traktdetails.Runtime) {
			logger.LogDynamicany1String(
				"debug",
				"skipped serie runtime for",
				logger.StrImdb,
				serie.ImdbID,
			)
		} else {
			serie.Runtime = strconv.Itoa(traktdetails.Runtime)
		}
	}

	// Handle IDs
	if (serie.ThetvdbID == 0 || overwrite) && traktdetails.IDs.Tvdb != 0 {
		serie.ThetvdbID = traktdetails.IDs.Tvdb
	}
	if (serie.TraktID == 0 || overwrite) && traktdetails.IDs.Trakt != 0 {
		serie.TraktID = traktdetails.IDs.Trakt
	}
	if (serie.TvrageID == 0 || overwrite) && traktdetails.IDs.Tvrage != 0 {
		serie.TvrageID = traktdetails.IDs.Tvrage
	}

	// Handle dates
	if (serie.Firstaired == "" || overwrite) && traktdetails.FirstAired.String() != "" {
		serie.Firstaired = traktdetails.FirstAired.String()
	}
	return nil
}

// shouldUpdateSerieRuntime determines whether the runtime of a series should be updated.
// It checks if the new runtime is valid and if it should replace the current runtime
// based on the overwrite flag and current runtime's validity.
// Returns true if the runtime should be updated, false otherwise.
func shouldUpdateSerieRuntime(currentRuntime string, newRuntime int, overwrite bool) bool {
	if newRuntime == 0 {
		return false
	}

	if overwrite && isValidRuntime(newRuntime) {
		return true
	}

	currentRuntimeInt, _ := strconv.Atoi(currentRuntime)
	return (currentRuntime == "" || currentRuntime == "0" || !isValidRuntime(currentRuntimeInt)) &&
		isValidRuntime(newRuntime)
}

// serieGetMetadataTvdb queries TheTVDB API to get metadata for the given serie.
// It takes a pointer to a Dbserie, language, overwrite flag, and existing aliases.
// It returns updated aliases after appending new ones from TheTVDB.
// It populates serie fields like name, status, runtime etc if empty or overwrite is set.
// It handles API errors and logs them.
func serieGetMetadataTvdb(
	serie *database.Dbserie,
	language string,
	overwrite bool,
	aliases []string,
) ([]string, error) {
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
	updateStringField(&serie.Seriename, tvdbdetails.Data.SeriesName, overwrite, nil)
	updateStringField(&serie.Season, tvdbdetails.Data.Season, overwrite, nil)
	updateStringField(&serie.Status, tvdbdetails.Data.Status, overwrite, nil)
	updateStringField(&serie.Firstaired, tvdbdetails.Data.FirstAired, overwrite, nil)
	updateStringField(&serie.Network, tvdbdetails.Data.Network, overwrite, nil)
	updateStringField(&serie.Runtime, tvdbdetails.Data.Runtime, overwrite, nil)
	updateStringField(&serie.Language, tvdbdetails.Data.Language, overwrite, nil)
	updateStringField(&serie.Overview, tvdbdetails.Data.Overview, overwrite, nil)
	updateStringField(&serie.Rating, tvdbdetails.Data.Rating, overwrite, nil)
	updateStringField(&serie.Banner, tvdbdetails.Data.Banner, overwrite, nil)
	updateStringField(&serie.Poster, tvdbdetails.Data.Poster, overwrite, nil)
	updateStringField(&serie.Fanart, tvdbdetails.Data.Fanart, overwrite, nil)
	updateStringField(&serie.ImdbID, tvdbdetails.Data.ImdbID, overwrite, nil)

	// Handle aliases
	if (serie.Aliases == "" || overwrite) && len(tvdbdetails.Data.Aliases) > 0 {
		serie.Aliases = strings.Join(tvdbdetails.Data.Aliases, ",")
	}

	// Handle genres
	if (serie.Genre == "" || overwrite) && len(tvdbdetails.Data.Genre) > 0 {
		serie.Genre = strings.Join(tvdbdetails.Data.Genre, ",")
	}

	// Handle numeric fields with string conversion
	if (serie.Siterating == "" || overwrite) && tvdbdetails.Data.SiteRating != 0 {
		serie.Siterating = strconv.FormatFloat(float64(tvdbdetails.Data.SiteRating), 'f', 1, 32)
	}

	if (serie.SiteratingCount == "" || overwrite) && tvdbdetails.Data.SiteRatingCount != 0 {
		serie.SiteratingCount = strconv.Itoa(tvdbdetails.Data.SiteRatingCount)
	}

	// Update slug if needed
	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlug(serie.Seriename)
	}

	// Return updated aliases
	if len(tvdbdetails.Data.Aliases) > 0 {
		return slices.Concat(aliases, tvdbdetails.Data.Aliases), nil
	}
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
func SerieGetMetadata(
	serie *database.Dbserie,
	language string,
	querytmdb, querytrakt, overwrite bool,
	aliases []string,
) []string {
	logger.LogDynamicany1String("info", "Get Metadata for", logger.StrTitle, serie.Seriename)
	defer logger.LogDynamicany1String(
		"info",
		"ended get metadata for",
		logger.StrTitle,
		serie.Seriename,
	)

	aliases, err := serieGetMetadataTvdb(serie, language, overwrite, aliases)
	if err != nil {
		logger.LogDynamicany1UIntErr("error", "Get Tvdb data", err, logger.StrID, serie.ID)
	}
	if querytmdb && serie.ThetvdbID != 0 {
		err = serieGetMetadataTmdb(serie, false)
		if err != nil {
			logger.LogDynamicany1UIntErr("error", "Get Tmdb data", err, logger.StrID, serie.ID)
		}
	}
	if querytrakt && serie.ImdbID != "" {
		err = serieGetMetadataTrakt(serie, false)
		if err != nil {
			logger.LogDynamicany1UIntErr("error", "Get Trakt data", err, logger.StrID, serie.ID)
			return aliases
		}
		if len(config.GetSettingsImdb().Indexedlanguages) > 0 {
			aliases = processTraktSerieAliases(serie, aliases)
		}
	}
	return aliases
}

// processTraktSerieAliases processes aliases from Trakt for a given series, adding new aliases
// based on configured indexed languages. It retrieves Trakt aliases for the series and
// appends unique aliases that match the indexed language settings to the existing aliases.
//
// It takes a database series pointer and an existing slice of aliases as input, and returns
// an updated slice of aliases after processing Trakt aliases.
func processTraktSerieAliases(serie *database.Dbserie, aliases []string) []string {
	traktAliases := apiexternal.GetTraktSerieAliases(serie)

	for _, alias := range traktAliases {
		if shouldAddAlias(alias.Title, alias.Country, aliases) {
			aliases = append(aliases, alias.Title)
		}
	}

	return aliases
}

// shouldAddAlias determines if an alias should be added based on existing aliases and language settings.
func shouldAddAlias(title, country string, existingAliases []string) bool {
	return !logger.SlicesContainsI(existingAliases, title) &&
		logger.SlicesContainsI(config.GetSettingsImdb().Indexedlanguages, country)
}

package metadata

import (
	"context"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

var (
	errTmdbNotFound = errors.New("tmdb not found")

	// invalidRuntimes is a sorted slice for binary search.
	// This is more memory efficient than a map for small sets.
	// Only genuinely-impossible junk/placeholder minute counts belong here -
	// 60/90/120 used to be included too, but those are extremely common real
	// movie/TV runtimes, and their presence here silently blocked legitimate
	// runtime updates (see shouldUpdateRuntime/shouldUpdateSerieRuntime).
	invalidRuntimes = []int{1, 2, 3, 4}
)

// isValidRuntime checks if a runtime value is valid (not in the invalid list).
// Uses binary search on sorted slice for O(log n) performance with less memory than map.
func isValidRuntime(runtime int) bool {
	_, found := slices.BinarySearch(invalidRuntimes, runtime)
	return !found
}

// shouldUpdateRuntime determines if runtime should be updated based on current and new values.
func shouldUpdateRuntime(currentRuntime, newRuntime int, overwrite bool) bool {
	// A junk/placeholder newRuntime must never be accepted, whether this is
	// populating an empty value or overwriting an existing one - the old
	// `return currentRuntime == 0` fallback below bypassed this check
	// entirely on first population, letting junk values through unfiltered.
	if newRuntime == 0 || !isValidRuntime(newRuntime) {
		return false
	}

	if currentRuntime == 0 {
		return true
	}

	return overwrite || !isValidRuntime(currentRuntime)
}

// buildCommaSeparated efficiently builds a comma-separated string from a slice.
// Uses a pooled buffer to reduce allocations.
// The getName function extracts the string value from each element.
func buildCommaSeparated[T any](items []T, getName func(T) string) string {
	if len(items) == 0 {
		return ""
	}

	// Fast path for single item - avoid buffer allocation
	if len(items) == 1 {
		return getName(items[0])
	}

	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	for i, item := range items {
		if i > 0 {
			bld.WriteByte(',')
		}

		bld.WriteString(getName(item))
	}

	return bld.String()
}

// buildGenreString efficiently builds a comma-separated genre string.
func buildGenreString(genres []apiexternal_v2.Genre) string {
	return buildCommaSeparated(genres, func(g apiexternal_v2.Genre) string {
		return g.Name
	})
}

// buildLanguageString efficiently builds a comma-separated language string.
func buildLanguageString(languages []apiexternal_v2.SpokenLanguage) string {
	return buildCommaSeparated(languages, func(l apiexternal_v2.SpokenLanguage) string {
		return l.EnglishName
	})
}

// movieGetTmdbMetadata fetches movie metadata from TMDb.
// It takes a pointer to a Dbmovie struct and a bool indicating whether to overwrite existing data.
// It finds the TMDb ID if missing using the IMDb ID.
// It fetches details from the TMDb API and populates Dbmovie fields if empty or overwrite is true.
// It closes the TMDb response when done.
func movieGetTmdbMetadata(movie *database.Dbmovie, overwrite bool) error {
	provider := providers.GetTMDB()
	if provider == nil {
		return nil
	}

	if movie.MoviedbID == 0 {
		if movie.ImdbID == "" {
			return nil
		}

		moviedb, err := provider.FindMovieByIMDbID(context.Background(), movie.ImdbID)
		if err != nil || moviedb == nil || len(moviedb.MovieResults) == 0 {
			if errors.Is(err, logger.ErrNotFound) {
				return nil
			}

			return err
		}

		if moviedb.MovieResults[0].ID == 0 {
			return logger.ErrNotFound
		}

		movie.MoviedbID = moviedb.MovieResults[0].ID
	}

	moviedbdetails, err := provider.GetMovieByID(context.Background(), movie.MoviedbID)
	if err != nil {
		return err
	}

	UpdateString(&movie.Title, moviedbdetails.Title, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})

	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlugCached(movie.Title)
	}

	UpdateBool(&movie.Adult, moviedbdetails.Adult, overwrite)
	UpdateInt(&movie.Budget, int(moviedbdetails.Budget), overwrite)
	UpdateFloat32(&movie.Popularity, float32(moviedbdetails.Popularity), overwrite)
	UpdateInt(&movie.Revenue, int(moviedbdetails.Revenue), overwrite)
	UpdateInt32(&movie.VoteCount, int32(moviedbdetails.VoteCount), overwrite)
	UpdateFloat32(&movie.VoteAverage, float32(moviedbdetails.VoteAverage), overwrite)

	UpdateString(&movie.OriginalLanguage, moviedbdetails.OriginalLanguage, overwrite, nil)
	UpdateString(&movie.OriginalTitle, moviedbdetails.OriginalTitle, overwrite, nil)
	UpdateString(&movie.Overview, moviedbdetails.Overview, overwrite, nil)
	UpdateString(&movie.Status, moviedbdetails.Status, overwrite, nil)
	UpdateString(&movie.Tagline, moviedbdetails.Tagline, overwrite, nil)
	UpdateString(&movie.Poster, moviedbdetails.PosterPath, overwrite, nil)
	UpdateString(&movie.Backdrop, moviedbdetails.BackdropPath, overwrite, nil)

	// Handle release date and year (overwrite applies to the date like the year)
	if UpdateNullTime(&movie.ReleaseDate, TimeToNullTime(moviedbdetails.ReleaseDate), overwrite) {
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
			logger.Logtype("debug", 1).
				Str(logger.StrImdb, movie.ImdbID).
				Msg("skipped moviedb movie runtime for")
		} else {
			movie.Runtime = moviedbdetails.Runtime
		}
	}

	if (movie.MoviedbID == 0 || overwrite) && moviedbdetails.ID != 0 {
		movie.MoviedbID = moviedbdetails.ID
	}

	return nil
}

// movieGetOmdbMetadata retrieves movie metadata from the OMDB API and merges it into the provided Dbmovie struct.
// It will overwrite existing data in the Dbmovie if the overwrite param is true.
// The OMDB API is queried using the ImdbID field in the Dbmovie.
func movieGetOmdbMetadata(movie *database.Dbmovie, overwrite bool) {
	if movie.ImdbID == "" {
		return
	}

	provider := providers.GetOMDB()
	if provider == nil {
		return
	}

	omdbdetails, err := provider.GetDetailsByIMDb(context.Background(), movie.ImdbID)
	if err != nil {
		return
	}

	UpdateString(&movie.Title, omdbValue(omdbdetails.Title), overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})

	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlugCached(movie.Title)
	}

	UpdateString(&movie.Genres, omdbValue(buildGenreString(omdbdetails.Genres)), overwrite, nil)
	UpdateString(&movie.URL, omdbValue(omdbdetails.Homepage), overwrite, nil)
	UpdateString(&movie.Overview, omdbValue(omdbdetails.Overview), overwrite, nil)

	if omdbdetails.VoteCount > 0 {
		UpdateInt32(&movie.VoteCount, int32(omdbdetails.VoteCount), overwrite)
	}

	if omdbdetails.VoteAverage > 0 {
		UpdateFloat32(&movie.VoteAverage, float32(math.Round(omdbdetails.VoteAverage*10)/10), overwrite)
	}

	if omdbdetails.Year > 0 && (movie.Year == 0 || overwrite) {
		movie.Year = uint16(omdbdetails.Year)
	}

	if omdbdetails.Runtime > 0 && shouldUpdateRuntime(movie.Runtime, omdbdetails.Runtime, overwrite) {
		movie.Runtime = omdbdetails.Runtime
	}
}

// omdbValue returns "" for OMDb's "N/A" placeholder, so it never overwrites
// real values.
func omdbValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "N/A" {
		return ""
	}

	return v
}

// movieGetTraktMetadata retrieves movie metadata from the Trakt API and merges it into the provided Dbmovie struct.
// It will overwrite existing data in the Dbmovie if the overwrite param is true.
// The Trakt API is queried using the ImdbID field in the Dbmovie.
func movieGetTraktMetadata(movie *database.Dbmovie, overwrite bool) error {
	if movie.ImdbID == "" {
		return nil
	}

	traktdetails, err := providers.TraktMovieByIMDb(context.Background(), movie.ImdbID)
	if err != nil {
		return err
	}

	UpdateString(&movie.Title, traktdetails.Title, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})

	if (movie.Slug == "" || overwrite) && movie.Title != "" {
		movie.Slug = logger.StringToSlugCached(movie.Title)
	}

	if (movie.Genres == "" || overwrite) && len(traktdetails.Genres) > 0 {
		movie.Genres = buildGenreString(traktdetails.Genres)
	}

	UpdateInt32(&movie.VoteCount, int32(traktdetails.VoteCount), overwrite)
	UpdateFloat32(&movie.VoteAverage, float32(traktdetails.VoteAverage), overwrite)
	UpdateString(&movie.Overview, traktdetails.Overview, overwrite, nil)
	UpdateString(&movie.Status, traktdetails.Status, overwrite, nil)
	UpdateString(&movie.OriginalLanguage, traktdetails.OriginalLanguage, overwrite, nil)
	UpdateString(&movie.Tagline, traktdetails.Tagline, overwrite, nil)

	if (movie.Year == 0 || overwrite) && traktdetails.Year > 0 {
		movie.Year = uint16(traktdetails.Year)
	}

	// Handle runtime with validation
	if shouldUpdateRuntime(movie.Runtime, traktdetails.Runtime, overwrite) {
		if movie.Runtime != 0 && !isValidRuntime(traktdetails.Runtime) {
			logger.Logtype("debug", 1).
				Str(logger.StrImdb, movie.ImdbID).
				Msg("skipped trakt movie runtime for")
		} else {
			movie.Runtime = traktdetails.Runtime
		}
	}

	if (movie.TraktID == 0 || overwrite) && traktdetails.ID != 0 {
		movie.TraktID = traktdetails.ID
	}

	UpdateNullTime(&movie.ReleaseDate, TimeToNullTime(traktdetails.ReleaseDate), overwrite)

	return nil
}

// MovieGetMetadata retrieves metadata for the given movie from multiple sources based on the input flags.
// It queries IMDb, TMDb, OMDb and Trakt APIs based on the queryimdb, querytmdb, queryomdb and querytrakt flags passed in.
// Results from each source are cached and merged into the movie struct.
func MovieGetMetadata(movie *database.Dbmovie, queryimdb, querytmdb, queryomdb, querytrakt bool) {
	logger.Logtype("info", 1).
		Str(logger.StrTitle, movie.ImdbID).
		Msg("Get Metadata for")
	defer logger.Logtype("info", 1).
		Str(logger.StrImdb, movie.ImdbID).
		Msg("ended get metadata for")

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
func Getmoviemetadata(
	dbmovie *database.Dbmovie,
	refresh bool,
	updateNTitle bool,
	cfgp *config.MediaTypeConfig,
	dbmovieadded bool,
) {
	// Cache config lookup to avoid repeated map access
	generalSettings := config.GetSettingsGeneral()
	priorities := generalSettings.MovieMetaSourcePriority

	var errTmdb, errTrakt, errImdb error
	for i := range priorities {
		// On refresh, only the highest-priority source may overwrite existing
		// fields; every later source fills gaps only. This keeps the source
		// priority intact (with the default order that means IMDb data leads)
		// while making a forced refresh actually replace stale values.
		overwrite := refresh && i == 0

		switch priorities[i] {
		case logger.StrImdb:
			errImdb = dbmovie.MovieGetImdbMetadata(overwrite)
		case "tmdb":
			errTmdb = movieGetTmdbMetadata(dbmovie, overwrite)
		case "omdb":
			movieGetOmdbMetadata(dbmovie, overwrite)
		case "trakt":
			errTrakt = movieGetTraktMetadata(dbmovie, overwrite)
		}
	}

	if !updateNTitle {
		return
	}

	database.ExecN(
		"update dbmovies SET Title = ? , Release_Date = ? , Year = ? , Adult = ? , Budget = ? , Genres = ? , Original_Language = ? , Original_Title = ? , Overview = ? , Popularity = ? , Revenue = ? , Runtime = ? , Spoken_Languages = ? , Status = ? , Tagline = ? , Vote_Average = ? , Vote_Count = ? , Trakt_ID = ? , Moviedb_ID = ? , Imdb_ID = ? , Freebase_M_ID = ? , Freebase_ID = ? , Facebook_ID = ? , Instagram_ID = ? , Twitter_ID = ? , URL = ? , Backdrop = ? , Poster = ? , Slug = ? where id = ?",
		&dbmovie.Title,
		&dbmovie.ReleaseDate,
		&dbmovie.Year,
		&dbmovie.Adult,
		&dbmovie.Budget,
		&dbmovie.Genres,
		&dbmovie.OriginalLanguage,
		&dbmovie.OriginalTitle,
		&dbmovie.Overview,
		&dbmovie.Popularity,
		&dbmovie.Revenue,
		&dbmovie.Runtime,
		&dbmovie.SpokenLanguages,
		&dbmovie.Status,
		&dbmovie.Tagline,
		&dbmovie.VoteAverage,
		&dbmovie.VoteCount,
		&dbmovie.TraktID,
		&dbmovie.MoviedbID,
		&dbmovie.ImdbID,
		&dbmovie.FreebaseMID,
		&dbmovie.FreebaseID,
		&dbmovie.FacebookID,
		&dbmovie.InstagramID,
		&dbmovie.TwitterID,
		&dbmovie.URL,
		&dbmovie.Backdrop,
		&dbmovie.Poster,
		&dbmovie.Slug,
		&dbmovie.ID,
	)

	if cfgp.Name != "" {
		// size +5
		titles := database.Getrowssize[database.DbstaticTwoString](
			false,
			"select count() from dbmovie_titles where dbmovie_id = ?",
			"select title, slug from dbmovie_titles where dbmovie_id = ?",
			&dbmovie.ID,
		)

		// Collect the allowed title languages once for the whole run.
		allowedLangs := collectTitleLanguages()

		hasImdb := generalSettings.MovieAlternateTitleMetaSourceImdb && dbmovie.ImdbID != "" &&
			errImdb == nil
		hasTmdb := generalSettings.MovieAlternateTitleMetaSourceTmdb &&
			dbmovie.MoviedbID != 0 && errTmdb == nil
		hasTrakt := generalSettings.MovieAlternateTitleMetaSourceTrakt && dbmovie.ImdbID != "" &&
			errTrakt == nil

		processMovieAlternateTitles(
			dbmovie,
			titles,
			allowedLangs,
			hasImdb,
			hasTmdb,
			hasTrakt,
			generalSettings.UseMediaCache,
		)
	}

	if dbmovieadded {
		if generalSettings.UseMediaCache {
			database.AppendCacheThreeString(
				logger.CacheDBMovie,
				syncops.DbstaticThreeStringTwoInt{
					Str1: dbmovie.Title,
					Str2: dbmovie.Slug,
					Str3: dbmovie.ImdbID,
					Num1: int(dbmovie.Year),
					Num2: dbmovie.ID,
				},
			)
		}
	}

	if dbmovie.Title != "" {
		return
	}

	database.Scanrowsdyn(
		false,
		database.QueryDbmovieTitlesGetTitleByIDLmit1,
		&dbmovie.Title,
		&dbmovie.ID,
	)

	if dbmovie.Title != "" {
		database.ExecN(
			"update dbmovies SET Title = ? where id = ?",
			&dbmovie.Title,
			&dbmovie.ID,
		)
	}
}

// Getmoviemetatitles retrieves alternate titles for a movie from various metadata
// sources like IMDb, TMDb, and Trakt based on configured settings. It adds any
// unique titles to the database.
func Getmoviemetatitles(movie *database.Dbmovie, cfgp *config.MediaTypeConfig) {
	if cfgp.Name == "" {
		return
	}

	// Cache config lookup once
	gs := config.GetSettingsGeneral()

	// Early exit if no alternate title sources are enabled
	hasImdb := gs.MovieAlternateTitleMetaSourceImdb && movie.ImdbID != ""
	hasTmdb := gs.MovieAlternateTitleMetaSourceTmdb && movie.MoviedbID != 0
	hasTrakt := gs.MovieAlternateTitleMetaSourceTrakt && movie.ImdbID != ""

	if !hasImdb && !hasTmdb && !hasTrakt {
		return
	}

	titles := database.Getrowssize[database.DbstaticTwoString](
		false,
		"select count() from dbmovie_titles where dbmovie_id = ?",
		"select title, slug from dbmovie_titles where dbmovie_id = ?",
		&movie.ID,
	)

	// Collect the allowed title languages once for the whole run.
	allowedLangs := collectTitleLanguages()

	processMovieAlternateTitles(
		movie,
		titles,
		allowedLangs,
		hasImdb,
		hasTmdb,
		hasTrakt,
		gs.UseMediaCache,
	)
}

// processMovieAlternateTitles collects alternate titles for movie from every
// enabled source (imdb_akas, TMDb, Trakt) in memory and inserts all newly
// discovered ones in a single batched transaction, instead of running a
// per-title existence check plus an insert-if-missing round trip (imdb_akas
// alone can carry dozens to hundreds of rows for a single movie). Shared by
// Getmoviemetatitles and Getmoviemetadata so both call sites get the same
// batching behavior.
func processMovieAlternateTitles(
	movie *database.Dbmovie,
	titles []database.DbstaticTwoString,
	allowedLangs []string,
	hasImdb, hasTmdb, hasTrakt bool,
	useMediaCache bool,
) {
	if !hasImdb && !hasTmdb && !hasTrakt {
		return
	}

	// seen mirrors the per-title "does it already exist" check the old code
	// ran against the live table (COLLATE NOCASE) for every single title: it
	// starts out with the titles already in the DB and grows as candidates
	// are staged below, so a title proposed twice - by the same source or by
	// two different sources - is only staged once, in the same source order
	// as before (imdb, then tmdb, then trakt).
	seen := make(map[string]struct{}, len(titles)+8)
	for i := range titles {
		seen[strings.ToLower(titles[i].Str1)] = struct{}{}
	}

	var batch []batchedInsert

	var cacheAdds []syncops.DbstaticTwoStringOneInt

	addAlternateTitle := func(title, slug, region string) {
		key := strings.ToLower(title)
		if _, ok := seen[key]; ok {
			return
		}

		seen[key] = struct{}{}

		if slug == "" {
			slug = logger.StringToSlugCached(title)
		}

		batch = append(batch, batchedInsert{
			query: "Insert into dbmovie_titles (title, slug, dbmovie_id, region) values (?, ?, ?, ?)",
			args:  []any{title, slug, movie.ID, region},
		})

		if useMediaCache {
			cacheAdds = append(cacheAdds, syncops.DbstaticTwoStringOneInt{
				Num:  movie.ID,
				Str1: title,
				Str2: slug,
			})
		}
	}

	if hasImdb {
		collectImdbAlternateTitles(movie, titles, allowedLangs, addAlternateTitle)
	}

	if hasTmdb {
		collectTmdbAlternateTitles(movie, titles, allowedLangs, addAlternateTitle)
	}

	if hasTrakt {
		collectTraktAlternateTitles(movie, titles, allowedLangs, addAlternateTitle)
	}

	runBatchedInserts(batch)

	for i := range cacheAdds {
		database.AppendCacheTwoString(logger.CacheTitlesMovie, cacheAdds[i])
	}
}

// collectImdbAlternateTitles gathers alternate titles for movie from IMDb and
// stages each one accepted by ShouldProcessTitle via addAlternateTitle,
// instead of checking-then-inserting them into the DB one at a time.
func collectImdbAlternateTitles(
	movie *database.Dbmovie,
	titles []database.DbstaticTwoString,
	allowedLangs []string,
	addAlternateTitle func(title, slug, region string),
) {
	movie.ImdbID = logger.AddImdbPrefix(movie.ImdbID)

	arr := database.Getrowssize[database.DbstaticThreeString](
		true,
		"select count() from imdb_akas where tconst = ?",
		"select region, title, slug from imdb_akas where tconst = ?",
		&movie.ImdbID,
	)

	if len(arr) == 0 {
		return
	}

	for idx := range arr {
		aka := &arr[idx]
		if !ShouldProcessTitle(aka.Str1, aka.Str2, titles, allowedLangs) {
			continue
		}

		addAlternateTitle(aka.Str2, aka.Str3, aka.Str1)
	}
}

// collectTmdbAlternateTitles gathers alternate titles for movie from TMDb and
// stages each one accepted by ShouldProcessTitle via addAlternateTitle,
// instead of checking-then-inserting them into the DB one at a time.
func collectTmdbAlternateTitles(
	movie *database.Dbmovie,
	titles []database.DbstaticTwoString,
	allowedLangs []string,
	addAlternateTitle func(title, slug, region string),
) {
	provider := providers.GetTMDB()
	if provider == nil || movie.MoviedbID == 0 {
		return
	}

	tbl, err := provider.GetMovieAlternativeTitles(context.Background(), movie.MoviedbID)
	if err != nil {
		return
	}

	addAlternateTitles(tbl, titles, allowedLangs, addAlternateTitle)
}

// addAlternateTitles stages each title accepted by ShouldProcessTitle.
func addAlternateTitles(
	tbl []apiexternal_v2.AlternativeTitle,
	titles []database.DbstaticTwoString,
	allowedLangs []string,
	addAlternateTitle func(title, slug, region string),
) {
	for idx := range tbl {
		title := &tbl[idx]
		if !ShouldProcessTitle(title.ISO3166_1, title.Title, titles, allowedLangs) {
			continue
		}

		addAlternateTitle(title.Title, "", title.ISO3166_1)
	}
}

// collectTraktAlternateTitles gathers alternate titles for movie from Trakt
// and stages each one accepted by ShouldProcessTitle via addAlternateTitle,
// instead of checking-then-inserting them into the DB one at a time.
func collectTraktAlternateTitles(
	movie *database.Dbmovie,
	titles []database.DbstaticTwoString,
	allowedLangs []string,
	addAlternateTitle func(title, slug, region string),
) {
	provider := providers.GetTrakt()
	if provider == nil || movie.TraktID == 0 {
		return
	}

	arr, err := provider.GetMovieAlternativeTitles(context.Background(), movie.TraktID)
	if err != nil {
		return
	}

	addAlternateTitles(arr, titles, allowedLangs, addAlternateTitle)
}

// collectTitleLanguages returns the union of MetadataTitleLanguages across all
// media configs. An empty result means no restrictions are configured (allow
// all). Collected once per processing run instead of re-scanning every media
// config for every single alternate title.
func collectTitleLanguages() []string {
	var langs []string

	config.RangeSettingsMedia(func(_ string, mediaCfg *config.MediaTypeConfig) error {
		if mediaCfg == nil {
			return nil
		}

		for _, lang := range mediaCfg.MetadataTitleLanguages {
			if !logger.SlicesContainsI(langs, lang) {
				langs = append(langs, lang)
			}
		}

		return nil
	})

	return langs
}

// serieGetMetadataTmdb queries TheMovieDB API to get metadata for the given serie.
// It populates various serie fields like name, slug etc if empty or overwrite is set.
// It handles API errors and logs them.
func serieGetMetadataTmdb(serie *database.Dbserie, overwrite bool) error {
	if serie.ThetvdbID == 0 {
		return logger.ErrTvdbEmpty
	}

	// Name already present and no overwrite requested: nothing to do. This is
	// the normal case for every known series - returning an error here made
	// the caller log it at error level on every metadata run.
	if serie.Seriename != "" && !overwrite {
		return nil
	}

	provider := providers.GetTMDB()
	if provider == nil {
		return errTmdbNotFound
	}

	moviedb, err := provider.FindSeriesByTVDbID(context.Background(), serie.ThetvdbID)
	if err != nil {
		return err
	}

	if moviedb == nil {
		return errTmdbNotFound
	}

	UpdateString(&serie.Seriename, moviedb.Name, overwrite, nil)

	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlugCached(serie.Seriename)
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

	traktdetails, err := providers.TraktSerieByIMDb(context.Background(), serie.ImdbID)
	if err != nil {
		return err
	}

	UpdateString(&serie.Seriename, traktdetails.Name, overwrite, func(title string) string {
		return logger.UnquoteUnescape(logger.Checkhtmlentities(title))
	})
	UpdateString(&serie.Language, traktdetails.OriginalLanguage, overwrite, nil)

	if len(traktdetails.Networks) > 0 {
		UpdateString(&serie.Network, traktdetails.Networks[0].Name, overwrite, nil)
	}

	UpdateString(&serie.Overview, traktdetails.Overview, overwrite, nil)
	UpdateString(&serie.Status, traktdetails.Status, overwrite, nil)

	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlugCached(serie.Seriename)
	}

	// Handle genres
	if (serie.Genre == "" || overwrite) && len(traktdetails.Genres) > 0 {
		serie.Genre = buildGenreString(traktdetails.Genres)
	}

	// Handle numeric fields with string conversion
	if (serie.Rating == "" || overwrite) && float32(traktdetails.VoteAverage) != 0 {
		serie.Rating = strconv.FormatFloat(float64(float32(traktdetails.VoteAverage)), 'f', 4, 64)
	}

	var runtime int
	if len(traktdetails.EpisodeRunTime) > 0 {
		runtime = traktdetails.EpisodeRunTime[0]
	}

	// Handle runtime with validation
	if shouldUpdateSerieRuntime(serie.Runtime, runtime, overwrite) {
		if serie.Runtime != "0" && !isValidRuntime(runtime) {
			logger.Logtype("debug", 1).
				Str(logger.StrImdb, serie.ImdbID).
				Msg("skipped serie runtime for")
		} else {
			serie.Runtime = strconv.Itoa(runtime)
		}
	}

	// Handle IDs
	if (serie.ThetvdbID == 0 || overwrite) && traktdetails.TVDbID != 0 {
		serie.ThetvdbID = traktdetails.TVDbID
	}

	if (serie.TraktID == 0 || overwrite) && traktdetails.ID != 0 {
		serie.TraktID = traktdetails.ID
	}

	// Handle dates
	if !traktdetails.FirstAirDate.IsZero() {
		UpdateString(&serie.Firstaired, traktdetails.FirstAirDate.Format("2006-01-02"), overwrite, nil)
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

	provider := providers.GetTVDB()
	if provider == nil {
		return aliases, logger.ErrNotFound
	}

	tvdbdetails, err := provider.GetSeriesByID(context.Background(), serie.ThetvdbID, language)
	if err != nil {
		if language != "" {
			tvdbdetails, err = provider.GetSeriesByID(context.Background(), serie.ThetvdbID, "")
		}

		if err != nil {
			return aliases, err
		}
	}

	UpdateString(&serie.Seriename, tvdbdetails.Name, overwrite, nil)
	UpdateString(&serie.Status, tvdbdetails.Status, overwrite, nil)

	if !tvdbdetails.FirstAirDate.IsZero() {
		UpdateString(&serie.Firstaired, tvdbdetails.FirstAirDate.Format("2006-01-02"), overwrite, nil)
	}

	if len(tvdbdetails.Networks) > 0 {
		UpdateString(&serie.Network, tvdbdetails.Networks[0].Name, overwrite, nil)
	}

	if len(tvdbdetails.EpisodeRunTime) > 0 {
		UpdateString(&serie.Runtime, strconv.Itoa(tvdbdetails.EpisodeRunTime[0]), overwrite, nil)
	}

	UpdateString(&serie.Language, tvdbdetails.OriginalLanguage, overwrite, nil)
	UpdateString(&serie.Overview, tvdbdetails.Overview, overwrite, nil)
	UpdateString(&serie.Poster, tvdbdetails.PosterPath, overwrite, nil)
	UpdateString(&serie.Fanart, tvdbdetails.BackdropPath, overwrite, nil)
	UpdateString(&serie.ImdbID, tvdbdetails.IMDbID, overwrite, nil)

	// Handle genres
	if (serie.Genre == "" || overwrite) && len(tvdbdetails.Genres) > 0 {
		serie.Genre = buildGenreString(tvdbdetails.Genres)
	}

	// Handle numeric fields with string conversion
	if rating := float32(tvdbdetails.VoteAverage); (serie.Siterating == "" || overwrite) && rating != 0 {
		serie.Siterating = strconv.FormatFloat(float64(rating), 'f', 1, 32)
	}

	if (serie.SiteratingCount == "" || overwrite) && tvdbdetails.VoteCount != 0 {
		serie.SiteratingCount = strconv.Itoa(tvdbdetails.VoteCount)
	}

	// Update slug if needed
	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		serie.Slug = logger.StringToSlugCached(serie.Seriename)
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
	logger.Logtype("info", 1).
		Str(logger.StrTitle, serie.Seriename).
		Msg("Get Metadata for")
	defer logger.Logtype("info", 1).
		Str(logger.StrTitle, serie.Seriename).
		Msg("ended get metadata for")

	aliases, err := serieGetMetadataTvdb(serie, language, overwrite, aliases)
	if err != nil {
		logger.Logtype("error", 1).
			Uint(logger.StrID, serie.ID).
			Err(err).
			Msg("Get Tvdb data")
	}

	// TVmaze only fills gaps behind TVDB; it may overwrite only when TVDB
	// supplied nothing.
	err = serieGetMetadataTvmaze(serie, overwrite && err != nil)
	if err != nil && !errors.Is(err, logger.ErrNotFound) {
		logger.Logtype("error", 2).
			Str(logger.StrTitle, serie.Seriename).
			Uint(logger.StrID, serie.ID).
			Err(err).
			Msg("Failed to retrieve TVmaze metadata")
	}

	if querytmdb && serie.ThetvdbID != 0 {
		err = serieGetMetadataTmdb(serie, false)
		if err != nil {
			logger.Logtype("error", 1).
				Uint(logger.StrID, serie.ID).
				Err(err).
				Msg("Get Tmdb data")
		}
	}

	if querytrakt && serie.ImdbID != "" {
		err = serieGetMetadataTrakt(serie, false)
		if err != nil {
			logger.Logtype("error", 1).
				Uint(logger.StrID, serie.ID).
				Err(err).
				Msg("Get Trakt data")

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
	traktAliases := providers.TraktSerieAliases(context.Background(), serie.TraktID)
	if len(traktAliases) == 0 {
		return aliases
	}

	// Cache indexed languages to avoid repeated config lookups
	indexedLangs := config.GetSettingsImdb().Indexedlanguages

	// Pre-allocate capacity for potential additions
	result := aliases
	for i := range traktAliases {
		if traktAliases[i].Title != "" &&
			!logger.SlicesContainsI(result, traktAliases[i].Title) &&
			logger.SlicesContainsI(indexedLangs, traktAliases[i].ISO3166_1) {
			result = append(result, traktAliases[i].Title)
		}
	}

	return result
}

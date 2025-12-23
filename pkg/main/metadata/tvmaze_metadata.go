package metadata

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// serieGetMetadataTvmaze fetches series metadata from TVmaze API and updates the serie struct.
// It tries to find the show using TVDB ID first, then IMDB ID if available.
// Returns error if API calls fail or if no identifiers are available.
func serieGetMetadataTvmaze(serie *database.Dbserie, overwrite bool) error {
	logger.Logtype("debug", 2).
		Str(logger.StrTitle, serie.Seriename).
		Str(logger.StrID, strconv.Itoa(int(serie.ID))).
		Msg("Starting TVmaze metadata retrieval")

	var (
		show         *apiexternal_v2.SeriesDetails
		err          error
		lookupMethod string
	)

	// Try to find show by TVDB ID first

	if serie.ThetvdbID != 0 {
		logger.Logtype("debug", 2).
			Str(logger.StrTvdb, strconv.Itoa(serie.ThetvdbID)).
			Str(logger.StrTitle, serie.Seriename).
			Msg("TVmaze lookup by TVDB ID")

		lookupMethod = "TVDB ID"

		show, err = apiexternal.GetTVmazeShowByTVDBID(serie.ThetvdbID)
		if err != nil && serie.ImdbID != "" {
			logger.Logtype("debug", 2).
				Str(logger.StrImdb, serie.ImdbID).
				Str(logger.StrTitle, serie.Seriename).
				Msg("TVmaze TVDB lookup failed, trying IMDB fallback")

			lookupMethod = "IMDB ID (fallback)"

			var imdbResult *apiexternal_v2.FindByIMDbResult

			imdbResult, err = apiexternal.GetTVmazeShowByIMDBID(serie.ImdbID)
			if err == nil && imdbResult != nil && len(imdbResult.TVResults) > 0 {
				// Get full details using the TVmaze ID from search result
				show, err = apiexternal.GetTVmazeShowByID(imdbResult.TVResults[0].ID)
			}
		}
	} else if serie.ImdbID != "" {
		logger.Logtype("debug", 2).Str(logger.StrImdb, serie.ImdbID).Str(logger.StrTitle, serie.Seriename).Msg("TVmaze lookup by IMDB ID")

		lookupMethod = "IMDB ID"

		var imdbResult *apiexternal_v2.FindByIMDbResult

		imdbResult, err = apiexternal.GetTVmazeShowByIMDBID(serie.ImdbID)
		if err == nil && imdbResult != nil && len(imdbResult.TVResults) > 0 {
			// Get full details using the TVmaze ID from search result
			show, err = apiexternal.GetTVmazeShowByID(imdbResult.TVResults[0].ID)
		}
	} else {
		logger.Logtype("debug", 1).Str(logger.StrTitle, serie.Seriename).Msg("TVmaze lookup skipped - no identifiers available")
		return logger.ErrNotFound
	}

	if err != nil {
		logger.Logtype("error", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str("method", lookupMethod).
			Err(err).
			Msg("TVmaze API request failed")

		return err
	}

	if show == nil {
		logger.Logtype("debug", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str("method", lookupMethod).
			Msg("TVmaze show not found")

		return logger.ErrNotFound
	}

	logger.Logtype("info", 5).
		Str(logger.StrTitle, show.Name).
		Int("tvmaze_id", show.ID).
		Str("method", lookupMethod).
		Str("status", show.Status).
		Str("network", getNetworkName(show)).
		Msg("TVmaze show found (v2)")

	// Track what gets updated for logging
	var updatedFields []string

	// Update series fields using TVmaze data
	oldName := serie.Seriename
	updateStringField(&serie.Seriename, show.Name, overwrite, nil)

	if serie.Seriename != oldName {
		updatedFields = append(updatedFields, "name")
	}

	oldStatus := serie.Status
	updateStringField(&serie.Status, show.Status, overwrite, nil)

	if serie.Status != oldStatus {
		updatedFields = append(updatedFields, "status")
	}

	oldNetwork := serie.Network
	updateStringField(&serie.Network, getNetworkName(show), overwrite, nil)

	if serie.Network != oldNetwork {
		updatedFields = append(updatedFields, "network")
	}

	oldLanguage := serie.Language
	updateStringField(&serie.Language, show.OriginalLanguage, overwrite, nil)

	if serie.Language != oldLanguage {
		updatedFields = append(updatedFields, "language")
	}

	oldOverview := serie.Overview
	updateStringField(&serie.Overview, show.Overview, overwrite, nil)

	if serie.Overview != oldOverview {
		updatedFields = append(updatedFields, "overview")
	}

	// Handle runtime - v2 API provides EpisodeRunTime as array
	oldRuntime := serie.Runtime
	if len(show.EpisodeRunTime) > 0 && show.EpisodeRunTime[0] > 0 {
		if shouldUpdateSerieRuntime(serie.Runtime, show.EpisodeRunTime[0], overwrite) {
			serie.Runtime = strconv.Itoa(show.EpisodeRunTime[0])
			logger.Logtype("debug", 2).
				Str(logger.StrTitle, serie.Seriename).
				Str("runtime", strconv.Itoa(show.EpisodeRunTime[0])).
				Msg("Updated runtime from TVmaze")
		}
	}

	if serie.Runtime != oldRuntime {
		updatedFields = append(updatedFields, "runtime")
	}

	// Update rating if available
	oldRating := serie.Rating
	if show.VoteAverage > 0 {
		if serie.Rating == "" || overwrite {
			serie.Rating = strconv.FormatFloat(show.VoteAverage, 'f', 1, 64)
			logger.Logtype("debug", 2).
				Str(logger.StrTitle, serie.Seriename).
				Str("rating", serie.Rating).
				Msg("Updated rating from TVmaze")
		}
	}

	if serie.Rating != oldRating {
		updatedFields = append(updatedFields, "rating")
	}

	// Update premiere date
	oldFirstaired := serie.Firstaired
	if !show.FirstAirDate.IsZero() {
		updateStringField(&serie.Firstaired, show.FirstAirDate.Format("2006-01-02"), overwrite, nil)
	}

	if serie.Firstaired != oldFirstaired {
		updatedFields = append(updatedFields, "premiered")
	}

	// Update genre information
	oldGenre := serie.Genre
	if len(show.Genres) > 0 {
		var genreNames []string
		for _, genre := range show.Genres {
			genreNames = append(genreNames, genre.Name)
		}

		genres := strings.Join(genreNames, ",")
		updateStringField(&serie.Genre, genres, overwrite, nil)

		if serie.Genre != oldGenre {
			logger.Logtype("debug", 2).
				Str(logger.StrTitle, serie.Seriename).
				Str("genres", genres).
				Msg("Updated genres from TVmaze")
		}
	}

	if serie.Genre != oldGenre {
		updatedFields = append(updatedFields, "genres")
	}

	// Update images if available
	oldPoster := serie.Poster
	if show.PosterPath != "" {
		updateStringField(&serie.Poster, show.PosterPath, overwrite, nil)
	}

	if serie.Poster != oldPoster {
		updatedFields = append(updatedFields, "poster")
	}

	oldBanner := serie.Banner
	if show.BackdropPath != "" {
		updateStringField(&serie.Banner, show.BackdropPath, overwrite, nil)
	}

	if serie.Banner != oldBanner {
		updatedFields = append(updatedFields, "banner")
	}

	// Update external IDs if we don't have them yet
	var newIDs []string
	if serie.ThetvdbID == 0 && show.TVDbID > 0 {
		serie.ThetvdbID = show.TVDbID

		newIDs = append(newIDs, "TVDB")

		logger.Logtype("info", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str(logger.StrTvdb, strconv.Itoa(show.TVDbID)).
			Msg("Found TVDB ID from TVmaze")
	}

	if serie.ImdbID == "" && show.IMDbID != "" {
		serie.ImdbID = show.IMDbID

		newIDs = append(newIDs, "IMDB")

		logger.Logtype("info", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str(logger.StrImdb, show.IMDbID).
			Msg("Found IMDB ID from TVmaze")
	}

	if len(newIDs) > 0 {
		updatedFields = append(updatedFields, "external_ids")
	}

	// Update slug
	oldSlug := serie.Slug
	if serie.Slug == "" || overwrite {
		serie.Slug = logger.StringToSlug(serie.Seriename)
	}

	if serie.Slug != oldSlug {
		updatedFields = append(updatedFields, "slug")
	}

	// Log summary of updates
	if len(updatedFields) > 0 {
		logger.Logtype("info", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str("fields", strings.Join(updatedFields, ",")).
			Msg("TVmaze metadata updated")
	} else {
		logger.Logtype("debug", 1).Str(logger.StrTitle, serie.Seriename).Msg("No TVmaze metadata updates needed")
	}

	return nil
}

// getNetworkName extracts network name from TVmaze show data.
func getNetworkName(show *apiexternal_v2.SeriesDetails) string {
	if len(show.Networks) > 0 && show.Networks[0].Name != "" {
		return show.Networks[0].Name
	}
	return ""
}

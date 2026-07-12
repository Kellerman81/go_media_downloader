package metadata

import (
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// fieldTracker efficiently tracks which fields were updated during metadata retrieval.
// Uses a bitmask for O(1) operations instead of slice appends.
type fieldTracker uint16

const (
	fieldName fieldTracker = 1 << iota
	fieldStatus
	fieldNetwork
	fieldLanguage
	fieldOverview
	fieldRuntime
	fieldRating
	fieldPremiered
	fieldGenres
	fieldPoster
	fieldBanner
	fieldExternalIDs
	fieldSlug
)

// fieldNames maps bitmask values to field names for logging.
var fieldNames = [...]string{
	"name", "status", "network", "language", "overview",
	"runtime", "rating", "premiered", "genres", "poster",
	"banner", "external_ids", "slug",
}

// String returns a comma-separated list of updated field names.
func (f fieldTracker) String() string {
	if f == 0 {
		return ""
	}

	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	first := true
	for i, name := range fieldNames {
		if f&(1<<i) == 0 {
			continue
		}

		if !first {
			bld.WriteByte(',')
		}

		bld.WriteString(name)

		first = false
	}

	return bld.String()
}

// serieGetMetadataTvmaze fetches series metadata from TVmaze API and updates the serie struct.
// It tries to find the show using TVDB ID first, then IMDB ID if available.
// Returns error if API calls fail or if no identifiers are available.
func serieGetMetadataTvmaze(serie *database.Dbserie, overwrite bool) error {
	logger.Logtype("debug", 2).
		Str(logger.StrTitle, serie.Seriename).
		Uint(logger.StrID, serie.ID).
		Msg("Starting TVmaze metadata retrieval")

	show, lookupMethod, err := lookupTVmazeShow(serie)
	if err != nil {
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

	// Apply metadata updates and track changes
	updated := applyTVmazeMetadata(serie, show, overwrite)

	// Log summary of updates
	if updated != 0 {
		logger.Logtype("info", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str("fields", updated.String()).
			Msg("TVmaze metadata updated")
	} else {
		logger.Logtype("debug", 1).
			Str(logger.StrTitle, serie.Seriename).
			Msg("No TVmaze metadata updates needed")
	}

	return nil
}

// lookupTVmazeShow attempts to find a show on TVmaze using available identifiers.
// Returns the show details, the lookup method used, and any error.
func lookupTVmazeShow(serie *database.Dbserie) (*apiexternal_v2.SeriesDetails, string, error) {
	if serie.ThetvdbID != 0 {
		return lookupByTVDBID(serie)
	}

	if serie.ImdbID != "" {
		return lookupByIMDBID(serie.ImdbID, serie.Seriename, "IMDB ID")
	}

	logger.Logtype("debug", 1).
		Str(logger.StrTitle, serie.Seriename).
		Msg("TVmaze lookup skipped - no identifiers available")

	return nil, "", logger.ErrNotFound
}

// lookupByTVDBID attempts to find a show using TVDB ID, with IMDB fallback.
func lookupByTVDBID(serie *database.Dbserie) (*apiexternal_v2.SeriesDetails, string, error) {
	logger.Logtype("debug", 2).
		Int(logger.StrTvdb, serie.ThetvdbID).
		Str(logger.StrTitle, serie.Seriename).
		Msg("TVmaze lookup by TVDB ID")

	show, err := apiexternal.GetTVmazeShowByTVDBID(serie.ThetvdbID)
	if err == nil {
		return show, "TVDB ID", nil
	}

	// Try IMDB fallback if TVDB lookup failed
	if serie.ImdbID != "" {
		logger.Logtype("debug", 2).
			Str(logger.StrImdb, serie.ImdbID).
			Str(logger.StrTitle, serie.Seriename).
			Msg("TVmaze TVDB lookup failed, trying IMDB fallback")

		return lookupByIMDBID(serie.ImdbID, serie.Seriename, "IMDB ID (fallback)")
	}

	return nil, "TVDB ID", err
}

// lookupByIMDBID attempts to find a show using IMDB ID.
func lookupByIMDBID(
	imdbID, serieName, method string,
) (*apiexternal_v2.SeriesDetails, string, error) {
	logger.Logtype("debug", 2).
		Str(logger.StrImdb, imdbID).
		Str(logger.StrTitle, serieName).
		Msg("TVmaze lookup by IMDB ID")

	imdbResult, err := apiexternal.GetTVmazeShowByIMDBID(imdbID)
	if err != nil {
		logger.Logtype("error", 2).
			Str(logger.StrTitle, serieName).
			Str("method", method).
			Err(err).
			Msg("TVmaze API request failed")

		return nil, method, err
	}

	if imdbResult == nil || len(imdbResult.TVResults) == 0 {
		return nil, method, nil
	}

	// Get full details using the TVmaze ID from search result
	show, err := apiexternal.GetTVmazeShowByID(imdbResult.TVResults[0].ID)
	if err != nil {
		logger.Logtype("error", 2).
			Str(logger.StrTitle, serieName).
			Str("method", method).
			Err(err).
			Msg("TVmaze API request failed")

		return nil, method, err
	}

	return show, method, nil
}

// applyTVmazeMetadata applies show metadata to the series and returns which fields were updated.
func applyTVmazeMetadata(
	serie *database.Dbserie,
	show *apiexternal_v2.SeriesDetails,
	overwrite bool,
) fieldTracker {
	var updated fieldTracker

	// Update basic string fields
	if UpdateString(&serie.Seriename, show.Name, overwrite, nil) {
		updated |= fieldName
	}

	if UpdateString(&serie.Status, show.Status, overwrite, nil) {
		updated |= fieldStatus
	}

	if UpdateString(&serie.Network, getNetworkName(show), overwrite, nil) {
		updated |= fieldNetwork
	}

	if UpdateString(&serie.Language, show.OriginalLanguage, overwrite, nil) {
		updated |= fieldLanguage
	}

	if UpdateString(&serie.Overview, show.Overview, overwrite, nil) {
		updated |= fieldOverview
	}

	// Handle runtime
	if len(show.EpisodeRunTime) > 0 && show.EpisodeRunTime[0] > 0 {
		if shouldUpdateSerieRuntime(serie.Runtime, show.EpisodeRunTime[0], overwrite) {
			serie.Runtime = strconv.Itoa(show.EpisodeRunTime[0])

			updated |= fieldRuntime
		}
	}

	// Handle rating
	if show.VoteAverage > 0 && (serie.Rating == "" || overwrite) {
		serie.Rating = strconv.FormatFloat(show.VoteAverage, 'f', 1, 64)

		updated |= fieldRating
	}

	// Handle premiere date
	if !show.FirstAirDate.IsZero() {
		if UpdateString(
			&serie.Firstaired,
			show.FirstAirDate.Format("2006-01-02"),
			overwrite,
			nil,
		) {
			updated |= fieldPremiered
		}
	}

	// Handle genres
	if len(show.Genres) > 0 {
		genres := buildCommaSeparated(show.Genres, func(g apiexternal_v2.Genre) string {
			return g.Name
		})
		if UpdateString(&serie.Genre, genres, overwrite, nil) {
			updated |= fieldGenres
		}
	}

	// Handle images
	if show.PosterPath != "" {
		if UpdateString(&serie.Poster, show.PosterPath, overwrite, nil) {
			updated |= fieldPoster
		}
	}

	if show.BackdropPath != "" {
		if UpdateString(&serie.Banner, show.BackdropPath, overwrite, nil) {
			updated |= fieldBanner
		}
	}

	// Handle external IDs (only update if missing)
	if applyExternalIDs(serie, show) {
		updated |= fieldExternalIDs
	}

	// Update slug
	if (serie.Slug == "" || overwrite) && serie.Seriename != "" {
		newSlug := logger.StringToSlugCached(serie.Seriename)
		if serie.Slug != newSlug {
			serie.Slug = newSlug

			updated |= fieldSlug
		}
	}

	return updated
}

// applyExternalIDs updates missing external IDs and returns true if any were added.
func applyExternalIDs(serie *database.Dbserie, show *apiexternal_v2.SeriesDetails) bool {
	var added bool

	if serie.ThetvdbID == 0 && show.TVDbID > 0 {
		serie.ThetvdbID = show.TVDbID
		added = true

		logger.Logtype("info", 2).
			Str(logger.StrTitle, serie.Seriename).
			Int(logger.StrTvdb, show.TVDbID).
			Msg("Found TVDB ID from TVmaze")
	}

	if serie.ImdbID == "" && show.IMDbID != "" {
		serie.ImdbID = show.IMDbID
		added = true

		logger.Logtype("info", 2).
			Str(logger.StrTitle, serie.Seriename).
			Str(logger.StrImdb, show.IMDbID).
			Msg("Found IMDB ID from TVmaze")
	}

	return added
}

// getNetworkName extracts network name from TVmaze show data.
func getNetworkName(show *apiexternal_v2.SeriesDetails) string {
	if len(show.Networks) > 0 && show.Networks[0].Name != "" {
		return show.Networks[0].Name
	}

	return ""
}

package providers

import (
	"context"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// TraktMovieByIMDb looks a movie up on Trakt by IMDB ID and returns its full
// details.
func TraktMovieByIMDb(ctx context.Context, imdbID string) (*apiexternal_v2.MovieDetails, error) {
	provider := GetTrakt()
	if provider == nil || imdbID == "" {
		return nil, logger.ErrNotFound
	}

	found, err := provider.FindMovieByIMDbID(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	if found == nil || len(found.MovieResults) == 0 {
		return nil, logger.ErrNotFound
	}

	return provider.GetMovieByID(ctx, found.MovieResults[0].ID)
}

// TraktSerieByIMDb looks a show up on Trakt by IMDB ID and returns its full
// details; ID is the Trakt ID.
func TraktSerieByIMDb(ctx context.Context, imdbID string) (*apiexternal_v2.SeriesDetails, error) {
	provider := GetTrakt()
	if provider == nil || imdbID == "" {
		return nil, logger.ErrNotFound
	}

	found, err := provider.FindSeriesByIMDbID(ctx, imdbID)
	if err != nil {
		return nil, err
	}

	if found == nil || len(found.TVResults) == 0 {
		return nil, logger.ErrNotFound
	}

	return provider.GetSeriesByID(ctx, found.TVResults[0].ID)
}

// TraktSerieAliases returns the show's alternative titles, nil when Trakt is
// not set up, the Trakt ID is unknown or the request fails.
func TraktSerieAliases(ctx context.Context, traktID int) []apiexternal_v2.AlternativeTitle {
	provider := GetTrakt()
	if provider == nil || traktID == 0 {
		return nil
	}

	titles, err := provider.GetSeriesAlternativeTitles(ctx, traktID)
	if err != nil {
		return nil
	}

	return titles
}

// TMDBMovieIMDbID searches TMDB for a movie by exact title (or original
// title) and returns its IMDB ID. With year > 0 only a candidate whose
// release year confirms it is accepted.
func TMDBMovieIMDbID(ctx context.Context, title string, year int) (string, error) {
	provider := GetTMDB()
	if provider == nil || title == "" {
		return "", logger.ErrNotFound
	}

	results, err := provider.SearchMovies(ctx, title, 0)
	if err != nil {
		return "", err
	}

	bestID := 0

	for i := range results {
		r := &results[i]
		if !strings.EqualFold(r.Title, title) && !strings.EqualFold(r.OriginalTitle, title) {
			continue
		}

		// A candidate without release date can't confirm a requested year.
		if year > 0 && (r.ReleaseDate.IsZero() || r.ReleaseDate.Year() != year) {
			continue
		}

		bestID = r.ID

		break
	}

	if bestID == 0 {
		return "", logger.ErrNotFound
	}

	ids, err := provider.GetMovieExternalIDs(ctx, bestID)
	if err != nil {
		return "", err
	}

	if ids == nil || ids.IMDbID == "" {
		return "", logger.ErrNotFound
	}

	return ids.IMDbID, nil
}

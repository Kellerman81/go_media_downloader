package metadata

import (
	"context"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// The cast, recommendations and similar titles of movies and series are
// saved with their metadata (database.SaveMediaExtras). Movies get them
// with the TMDB details the import loads anyway (append_to_response);
// series, whose metadata comes from TVDB, cost two TMDB requests (find by
// TVDB id, details), made whenever a TMDB key is set.

// CastFrom converts TMDB credits for saving (at most database.MaxStoredCast).
func CastFrom(c *apiexternal_v2.Credits) []database.MediaCast {
	if c == nil {
		return nil
	}

	out := make([]database.MediaCast, 0, min(len(c.Cast), database.MaxStoredCast))
	for _, m := range c.Cast[:min(len(c.Cast), database.MaxStoredCast)] {
		out = append(out, database.MediaCast{PersonID: m.ID, Name: m.Name, Character: m.Character, Profile: m.ProfilePath})
	}

	return out
}

func yearOf(t time.Time) int {
	if t.IsZero() {
		return 0
	}

	return t.Year()
}

// RelatedMovies converts TMDB movie results for saving.
func RelatedMovies(rs []apiexternal_v2.MovieSearchResult) []database.MediaRelated {
	out := make([]database.MediaRelated, 0, len(rs))
	for _, r := range rs {
		out = append(out, database.MediaRelated{TMDB: r.ID, Title: r.Title, Year: yearOf(r.ReleaseDate), Rating: r.VoteAverage, Poster: r.PosterPath})
	}

	return out
}

// RelatedSeries converts TMDB series results for saving.
func RelatedSeries(rs []apiexternal_v2.SeriesSearchResult) []database.MediaRelated {
	out := make([]database.MediaRelated, 0, len(rs))
	for _, r := range rs {
		out = append(out, database.MediaRelated{TMDB: r.ID, Title: r.Name, Year: yearOf(r.FirstAirDate), Rating: r.VoteAverage, Poster: r.PosterPath})
	}

	return out
}

// SaveMovieExtras saves the cast and related titles of a dbmovies row from
// its TMDB details.
func SaveMovieExtras(dbmovieID uint, d *apiexternal_v2.MovieDetails) {
	if d == nil {
		return
	}

	if err := database.SaveMediaExtras("movie", dbmovieID, CastFrom(d.Credits), RelatedMovies(d.Recommendations), RelatedMovies(d.Similar)); err != nil {
		logger.Logtype("warn", 1).Err(err).Uint(logger.StrID, dbmovieID).Msg("Movie cast not saved")
	}
}

// serieSaveTmdbExtras loads the cast and related titles of a series from
// TMDB and saves them; lang is the group's metadata language, asked when
// TMDB finds nothing in English.
func serieSaveTmdbExtras(serie *database.Dbserie, lang string) {
	provider := providers.GetTMDB()
	if provider == nil || serie.ID == 0 || serie.ThetvdbID == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	// The find returns the full details, cast and related titles included.
	d, err := provider.WithFallbackLanguage(lang).FindSeriesByTVDbID(ctx, serie.ThetvdbID)
	if err != nil || d == nil || d.ID == 0 {
		logger.Logtype("debug", 1).Err(err).Uint(logger.StrID, serie.ID).Msg("Series cast not loaded")
		return
	}

	if err := database.SaveMediaExtras("series", serie.ID, CastFrom(d.Credits), RelatedSeries(d.Recommendations), RelatedSeries(d.Similar)); err != nil {
		logger.Logtype("warn", 1).Err(err).Uint(logger.StrID, serie.ID).Msg("Series cast not saved")
	}
}

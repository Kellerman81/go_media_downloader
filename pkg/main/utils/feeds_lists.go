package utils

import (
	"context"
	"iter"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// tmdbPageSize is the fixed number of results TMDB returns per list page.
const tmdbPageSize = 20

// traktPageSize is the largest page Trakt serves.
const traktPageSize = 100

// listLimit parses a list's configured limit, def when unset or invalid.
func listLimit(limit string, def int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(limit)); err == nil && n > 0 {
		return n
	}

	return def
}

// tmdbFetchPages collects up to maxItems results page by page. A failing
// first page is an error; a later failure keeps what was collected.
func tmdbFetchPages[T any](
	maxItems int,
	fetch func(page int) ([]T, int, error),
) ([]T, error) {
	pages := (maxItems + tmdbPageSize - 1) / tmdbPageSize
	out := make([]T, 0, min(maxItems, pages*tmdbPageSize))

	for page := 1; page <= pages; page++ {
		res, totalPages, err := fetch(page)
		if err != nil {
			if page == 1 {
				return nil, err
			}

			logger.Logtype("warn", 1).
				Err(err).
				Int("page", page).
				Msg("TMDB list page request failed, using results collected so far")

			break
		}

		out = append(out, res...)

		if len(out) >= maxItems || len(res) == 0 || (totalPages > 0 && page >= totalPages) {
			break
		}
	}

	if len(out) > maxItems {
		out = out[:maxItems]
	}

	return out, nil
}

// tmdbMovieList pages through a TMDB movie list up to the configured limit.
func tmdbMovieList(
	limit string,
	fetch func(ctx context.Context, page int) (*apiexternal_v2.PopularMoviesResponse, error),
) ([]apiexternal_v2.MovieSearchResult, error) {
	return tmdbFetchPages(
		listLimit(limit, tmdbPageSize),
		func(page int) ([]apiexternal_v2.MovieSearchResult, int, error) {
			r, err := fetch(context.Background(), page)
			if err != nil || r == nil {
				return nil, 0, err
			}

			return r.Results, r.TotalPages, nil
		},
	)
}

// tmdbSerieList pages through a TMDB series list up to the configured limit.
func tmdbSerieList(
	limit string,
	fetch func(ctx context.Context, page int) (*apiexternal_v2.PopularSeriesResponse, error),
) ([]apiexternal_v2.SeriesSearchResult, error) {
	return tmdbFetchPages(
		listLimit(limit, tmdbPageSize),
		func(page int) ([]apiexternal_v2.SeriesSearchResult, int, error) {
			r, err := fetch(context.Background(), page)
			if err != nil || r == nil {
				return nil, 0, err
			}

			return r.Results, r.TotalPages, nil
		},
	)
}

// traktPaging splits limit into pages of at most traktPageSize.
func traktPaging(limit int) (pages, pageSize int) {
	pageSize = min(limit, traktPageSize)
	return (limit + pageSize - 1) / pageSize, pageSize
}

// traktSeq yields up to the configured limit (default 20) of a paged Trakt
// list. fetch returns a page's items and how many entries the page had, so a
// short page ends the list even when entries were dropped.
func traktSeq[T any](
	list, limit string,
	fetch func(ctx context.Context, page, size int) ([]T, int, error),
) iter.Seq[T] {
	return func(yield func(T) bool) {
		maxItems := listLimit(limit, 20)
		pages, size := traktPaging(maxItems)
		count := 0

		for page := 1; page <= pages; page++ {
			items, n, err := fetch(context.Background(), page, size)
			if err != nil {
				logger.Logtype("error", 1).
					Err(err).
					Str("list", list).
					Int("page", page).
					Msg("Trakt list request failed")

				return
			}

			for i := range items {
				if !yield(items[i]) {
					return
				}

				count++
				if count >= maxItems {
					return
				}
			}

			if n < size {
				return
			}
		}
	}
}

// traktMovies adapts a Trakt movie list call to traktSeq.
func traktMovies(
	call func(ctx context.Context, page, size int, extra string) (*apiexternal_v2.PopularMoviesResponse, error),
	extra string,
) func(ctx context.Context, page, size int) ([]apiexternal_v2.MovieSearchResult, int, error) {
	return func(ctx context.Context, page, size int) ([]apiexternal_v2.MovieSearchResult, int, error) {
		r, err := call(ctx, page, size, extra)
		if err != nil || r == nil {
			return nil, 0, err
		}

		return r.Results, len(r.Results), nil
	}
}

// traktSeries adapts a Trakt series list call to traktSeq.
func traktSeries(
	call func(ctx context.Context, page, size int, extra string) (*apiexternal_v2.PopularSeriesResponse, error),
	extra string,
) func(ctx context.Context, page, size int) ([]apiexternal_v2.SeriesSearchResult, int, error) {
	return func(ctx context.Context, page, size int) ([]apiexternal_v2.SeriesSearchResult, int, error) {
		r, err := call(ctx, page, size, extra)
		if err != nil || r == nil {
			return nil, 0, err
		}

		return r.Results, len(r.Results), nil
	}
}

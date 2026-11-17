package api

import (
	"cmp"
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
)

// Trailers of movies and series from TMDB, in the metadata language of the
// entry's media group, else in English.

// libTrailer is the trailer the detail dialog offers.
type libTrailer struct {
	Key      string `json:"key"` // YouTube video id
	Name     string `json:"name"`
	Language string `json:"language"`
}

type trailerCacheEntry struct {
	trailer *libTrailer
	until   time.Time
}

var trailerCache sync.Map // type|db id|language -> trailerCacheEntry

const trailerCacheTime = 24 * time.Hour

// listLanguage is the metadata language (ISO 639-1) of the media group a
// list belongs to; "en" when it has none.
func listLanguage(listname string) string {
	lang := ""
	if cfgp := mediaConfigOfList(listname); cfgp != nil {
		lang = cfgp.MetadataLanguage
	}

	// "de", "de-DE", "German", "ger": the trailers are asked for by
	// ISO 639-1 code.
	lang, _, _ = strings.Cut(lang, "-")
	if lang = langCode(lang); len(lang) != 2 {
		return "en"
	}

	return lang
}

// pickTrailer chooses a YouTube video in lang, else in English: trailers
// before teasers, official ones first, then the newest.
func pickTrailer(videos []apiexternal_v2.Video, lang string) *libTrailer {
	rank := func(v *apiexternal_v2.Video) int {
		score := 0
		if strings.EqualFold(v.Language, lang) {
			score += 100
		}

		switch v.Type {
		case "Trailer":
			score += 20
		case "Teaser":
			score += 10
		default:
			return -1
		}

		if v.Official {
			score += 5
		}

		return score
	}

	var best *apiexternal_v2.Video

	for i := range videos {
		v := &videos[i]
		if v.Site != "YouTube" || v.Key == "" || rank(v) < 0 ||
			(!strings.EqualFold(v.Language, lang) && !strings.EqualFold(v.Language, "en")) {
			continue
		}

		if best == nil || cmp.Or(cmp.Compare(rank(v), rank(best)), v.PublishedAt.Compare(best.PublishedAt)) > 0 {
			best = v
		}
	}

	if best == nil {
		return nil
	}

	return &libTrailer{Key: best.Key, Name: best.Name, Language: strings.ToLower(best.Language)}
}

// findTrailer looks the trailer up at TMDB.
func findTrailer(ctx context.Context, typ string, id *uint64, lang string) (*libTrailer, error) {
	tmdb := providers.GetTMDB()
	if tmdb == nil {
		return nil, nil
	}

	languages := []string{lang, "en"}
	if lang == "en" {
		languages = languages[:1]
	}

	var (
		videos []apiexternal_v2.Video
		err    error
	)

	switch typ {
	case "movie":
		tmdbID := database.Getdatarow[int](false, "SELECT COALESCE(moviedb_id, 0) FROM dbmovies WHERE id = ?", id)
		if tmdbID == 0 {
			imdb := database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbmovies WHERE id = ?", id)
			if imdb == "" {
				return nil, nil
			}

			found, err := tmdb.FindMovieByIMDbID(ctx, imdb)
			if err != nil || len(found.MovieResults) == 0 {
				return nil, err
			}

			tmdbID = found.MovieResults[0].ID
		}

		videos, err = tmdb.GetMovieVideos(ctx, tmdbID, languages...)
	case "series":
		tvdb := database.Getdatarow[int](false, "SELECT COALESCE(thetvdb_id, 0) FROM dbseries WHERE id = ?", id)
		if tvdb == 0 {
			return nil, nil
		}

		details, ferr := tmdb.FindSeriesByTVDbID(ctx, tvdb)
		if ferr != nil {
			return nil, ferr
		}

		videos, err = tmdb.GetSeriesVideos(ctx, details.ID, languages...)
	}

	if err != nil {
		return nil, err
	}

	return pickTrailer(videos, lang), nil
}

// HandleLibraryTrailer returns the trailer of a movie or series
// (/admin/library/trailer?type=&id=<db id>&list=), 204 without one.
func HandleLibraryTrailer(c *gin.Context) {
	typ := c.Query("type")
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)

	if (typ != "movie" && typ != "series") || err != nil || !accessOf(c).allowsDB(libKindByType(typ), &id) {
		c.Status(http.StatusNoContent)
		return
	}

	lang := listLanguage(c.Query("list"))
	key := typ + "|" + c.Query("id") + "|" + lang

	if v, ok := trailerCache.Load(key); ok && time.Now().Before(v.(trailerCacheEntry).until) {
		replyTrailer(c, v.(trailerCacheEntry).trailer)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	t, err := findTrailer(ctx, typ, &id, lang)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}

	trailerCache.Store(key, trailerCacheEntry{trailer: t, until: time.Now().Add(trailerCacheTime)})
	replyTrailer(c, t)
}

func replyTrailer(c *gin.Context, t *libTrailer) {
	if t == nil {
		c.Status(http.StatusNoContent)
		return
	}

	c.JSON(http.StatusOK, t)
}

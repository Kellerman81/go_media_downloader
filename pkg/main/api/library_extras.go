package api

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The "More" tab of a movie or series: cast, videos, recommendations,
// similar titles and reviews from TMDB, else (no TMDB key or no TMDB id)
// cast, related titles and videos from Trakt.

const tmdbImageBase = "https://image.tmdb.org/t/p/"

// libTMDB is the TMDB provider for a dbmovies / dbseries row: asking again
// in its groups' metadata language when TMDB finds nothing (nil: no TMDB).
func libTMDB(typ string, id *uint64) *tmdb.Provider {
	p := providers.GetTMDB()
	if p == nil {
		return nil
	}

	return p.WithFallbackLanguage(metadata.ListedLanguage(typ, uint(*id), 0)) //nolint:gosec // a rowid
}

// tmdbIDOf is the TMDB id of a dbmovies / dbseries row (0: not found).
func tmdbIDOf(ctx context.Context, p *tmdb.Provider, typ string, id *uint64) int {
	switch typ {
	case "movie":
		if tid := database.Getdatarow[int](false, "SELECT COALESCE(moviedb_id, 0) FROM dbmovies WHERE id = ?", id); tid > 0 {
			return tid
		}

		imdb := database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbmovies WHERE id = ?", id)
		if imdb == "" {
			return 0
		}

		if found, err := p.FindMovieByIMDbID(ctx, imdb); err == nil && len(found.MovieResults) > 0 {
			return found.MovieResults[0].ID
		}
	case "series":
		if tvdb := database.Getdatarow[int](false, "SELECT COALESCE(thetvdb_id, 0) FROM dbseries WHERE id = ?", id); tvdb > 0 {
			if d, err := p.FindSeriesByTVDbID(ctx, tvdb); err == nil {
				return d.ID
			}
		}

		imdb := database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbseries WHERE id = ?", id)
		if imdb == "" {
			return 0
		}

		if found, err := p.FindSeriesByIMDbID(ctx, imdb); err == nil && len(found.TVResults) > 0 {
			return found.TVResults[0].ID
		}
	}

	return 0
}

// traktIDOf is the stored Trakt id of a dbmovies / dbseries row.
func traktIDOf(typ string, id *uint64) int {
	table := map[string]string{"movie": "dbmovies", "series": "dbseries"}[typ]
	if table == "" {
		return 0
	}

	return database.Getdatarow[int](false, "SELECT COALESCE(trakt_id, 0) FROM "+table+" WHERE id = ?", id)
}

// libVideos are the videos of a movie or series in lang (and English):
// TMDB's, else Trakt's.
func libVideos(ctx context.Context, typ string, id *uint64, lang string) ([]apiexternal_v2.Video, error) {
	languages := []string{lang, "en"}
	if lang == "en" {
		languages = languages[:1]
	}

	if p := libTMDB(typ, id); p != nil {
		if tid := tmdbIDOf(ctx, p, typ, id); tid > 0 {
			if typ == "movie" {
				return p.GetMovieVideos(ctx, tid, languages...)
			}

			return p.GetSeriesVideos(ctx, tid, languages...)
		}
	}

	if t := providers.GetTrakt(); t != nil {
		if tid := traktIDOf(typ, id); tid > 0 {
			if typ == "movie" {
				return t.GetMovieVideos(ctx, tid)
			}

			return t.GetSeriesVideos(ctx, tid)
		}
	}

	return nil, nil
}

// extraTitle is a recommended or similar movie or series.
type extraTitle struct {
	ID      int // TMDB id (Trakt results: 0 unless known)
	Title   string
	Year    int
	Rating  float64
	Poster  string
	EntryID uint // its list entry in the media group (0: not in the library)
}

// libExtras is what the More tab shows.
type libExtras struct {
	Source          string
	Cast            []apiexternal_v2.CastMember
	Videos          []apiexternal_v2.Video
	Recommendations []extraTitle
	Similar         []extraTitle
	Reviews         []tmdb.Review
}

type extrasCacheEntry struct {
	extras *libExtras
	until  time.Time
}

var extrasCache sync.Map // type|db id|lang -> extrasCacheEntry

// knownYear is the year of a date, 0 when it is unknown.
func knownYear(t time.Time) int {
	if t.IsZero() {
		return 0
	}

	return t.Year()
}

func movieTitles(rs []apiexternal_v2.MovieSearchResult) []extraTitle {
	out := make([]extraTitle, 0, len(rs))
	for _, r := range rs {
		out = append(out, extraTitle{ID: r.ID, Title: r.Title, Year: knownYear(r.ReleaseDate), Rating: r.VoteAverage, Poster: r.PosterPath})
	}

	return out
}

func seriesTitles(rs []apiexternal_v2.SeriesSearchResult) []extraTitle {
	out := make([]extraTitle, 0, len(rs))
	for _, r := range rs {
		out = append(out, extraTitle{ID: r.ID, Title: r.Name, Year: knownYear(r.FirstAirDate), Rating: r.VoteAverage, Poster: r.PosterPath})
	}

	return out
}

// storedTitles converts saved related titles.
func storedTitles(rs []database.MediaRelated) []extraTitle {
	out := make([]extraTitle, 0, len(rs))
	for _, r := range rs {
		out = append(out, extraTitle{ID: r.TMDB, Title: r.Title, Year: r.Year, Rating: r.Rating, Poster: r.Poster})
	}

	return out
}

// storedExtras fills the cast and related titles the metadata import saved;
// false when there are none (not refreshed since, or no TMDB).
func storedExtras(e *libExtras, typ string, id uint64) bool {
	for _, c := range database.MediaCastOf(typ, id) {
		e.Cast = append(e.Cast, apiexternal_v2.CastMember{ID: c.PersonID, Name: c.Name, Character: c.Character, ProfilePath: c.Profile})
	}

	e.Recommendations = storedTitles(database.MediaRelatedOf(typ, id, database.RelationRecommended))
	e.Similar = storedTitles(database.MediaRelatedOf(typ, id, database.RelationSimilar))

	return len(e.Cast)+len(e.Recommendations)+len(e.Similar) > 0
}

// loadExtras fetches the More tab's data; nil when neither TMDB nor Trakt
// knows the title. The cast and related titles come from the database when
// the import saved them; videos and reviews are always loaded.
func loadExtras(ctx context.Context, typ string, id *uint64, lang string) *libExtras {
	e := &libExtras{}
	e.Videos, _ = libVideos(ctx, typ, id, lang)

	if storedExtras(e, typ, *id) {
		e.Source = "TMDB"

		if p := providers.GetTMDB(); p != nil {
			if tid := tmdbIDOf(ctx, p, typ, id); tid > 0 {
				if typ == "movie" {
					e.Reviews, _ = p.GetMovieReviews(ctx, tid)
				} else {
					e.Reviews, _ = p.GetSeriesReviews(ctx, tid)
				}
			}
		}

		return e
	}

	if p := libTMDB(typ, id); p != nil {
		if tid := tmdbIDOf(ctx, p, typ, id); tid > 0 {
			e.Source = "TMDB"

			var credits *apiexternal_v2.Credits

			var related [2][]database.MediaRelated

			if typ == "movie" {
				credits, _ = p.GetMovieCredits(ctx, tid)
				recs, _ := p.GetMovieRecommendations(ctx, tid)
				sim, _ := p.GetSimilarMovies(ctx, tid)
				e.Recommendations, e.Similar = movieTitles(recs), movieTitles(sim)
				related = [2][]database.MediaRelated{metadata.RelatedMovies(recs), metadata.RelatedMovies(sim)}
				e.Reviews, _ = p.GetMovieReviews(ctx, tid)
			} else {
				credits, _ = p.GetSeriesCredits(ctx, tid)
				recs, _ := p.GetSeriesRecommendations(ctx, tid)
				sim, _ := p.GetSimilarSeries(ctx, tid)
				e.Recommendations, e.Similar = seriesTitles(recs), seriesTitles(sim)
				related = [2][]database.MediaRelated{metadata.RelatedSeries(recs), metadata.RelatedSeries(sim)}
				e.Reviews, _ = p.GetSeriesReviews(ctx, tid)
			}

			if credits != nil {
				e.Cast = credits.Cast

				// Saved like the import does, so the cast filter finds this
				// title before its next refresh.
				if err := database.SaveMediaExtras(typ, uint(*id), metadata.CastFrom(credits), related[0], related[1]); err != nil { //nolint:gosec // a rowid
					logger.Logtype("warn", 0).Err(err).Str("type", typ).Msg("Cast not saved")
				}
			}

			return e
		}
	}

	if t := providers.GetTrakt(); t != nil {
		if tid := traktIDOf(typ, id); tid > 0 {
			e.Source = "Trakt"

			var credits *apiexternal_v2.Credits

			if typ == "movie" {
				credits, _ = t.GetMovieCredits(ctx, tid)
				sim, _ := t.GetSimilarMovies(ctx, tid)
				e.Similar = movieTitles(sim)
			} else {
				credits, _ = t.GetSeriesCredits(ctx, tid)
				sim, _ := t.GetSimilarSeries(ctx, tid)
				e.Similar = seriesTitles(sim)
			}

			if credits != nil {
				e.Cast = credits.Cast
			}

			return e
		}
	}

	if len(e.Videos) == 0 {
		return nil
	}

	return e
}

// markInLibrary sets the list entry of titles the media group has: movies
// by TMDB id, series by name and first-air year.
func markInLibrary(typ string, lists []string, titles []extraTitle) {
	if len(titles) == 0 || len(lists) == 0 {
		return
	}

	in := " COLLATE NOCASE IN (?" + strings.Repeat(", ?", len(lists)-1) + ")"
	largs := make([]any, len(lists))

	for i := range lists {
		largs[i] = lists[i]
	}

	for i := range titles {
		ti := &titles[i]

		if typ == "movie" {
			if ti.ID > 0 {
				ti.EntryID = database.Getdatarow[uint](false, "SELECT e.id FROM movies e INNER JOIN dbmovies d ON d.id = e.dbmovie_id "+
					"WHERE d.moviedb_id = ? AND e.listname"+in+" LIMIT 1", append([]any{ti.ID}, largs...)...)
			}

			continue
		}

		ti.EntryID = database.Getdatarow[uint](false, "SELECT e.id FROM series e INNER JOIN dbseries d ON d.id = e.dbserie_id "+
			"WHERE d.seriename = ? COLLATE NOCASE AND (? = 0 OR substr(d.firstaired, 1, 4) = ?) AND e.listname"+in+" LIMIT 1",
			append([]any{ti.Title, ti.Year, strconv.Itoa(ti.Year)}, largs...)...)
	}
}

// extrasOf returns the More tab's data of a movie or series (db ID),
// cached for a day; nil when unknown.
func extrasOf(ctx context.Context, typ string, id uint64, lang string) *libExtras {
	key := typ + "|" + strconv.FormatUint(id, 10) + "|" + lang

	if v, ok := extrasCache.Load(key); ok && time.Now().Before(v.(extrasCacheEntry).until) {
		return v.(extrasCacheEntry).extras
	}

	e := loadExtras(ctx, typ, &id, lang)
	if ctx.Err() == nil {
		extrasCache.Store(key, extrasCacheEntry{extras: e, until: time.Now().Add(trailerCacheTime)})
	}

	return e
}

// HandleLibraryExtras renders the More tab of a movie or series
// (/admin/library/extras?type=&id=<db id>&list=).
func HandleLibraryExtras(c *gin.Context) {
	typ := c.Query("type")
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	c.Header("Content-Type", "text/html")

	if (typ != "movie" && typ != "series") || err != nil || !accessOf(c).allowsDB(libKindByType(typ), &id) {
		c.String(http.StatusNotFound, renderAlert(t("Unknown entry"), "warning"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 20*time.Second)
	e := extrasOf(ctx, typ, id, listLanguage(c.Query("list")))

	cancel()

	if e == nil {
		c.String(http.StatusOK, renderComponentToString(html.P(html.Class("text-muted small"),
			gomponents.Text(t("No details available - TMDB or Trakt has to be configured and know this title.")))))

		return
	}

	// The in-library marks depend on the account's groups: not cached.
	var lists []string
	if cfgp := mediaConfigOfList(c.Query("list")); cfgp != nil && accessOf(c).AllowsGroup(cfgp.NamePrefix) {
		for i := range cfgp.Lists {
			lists = append(lists, cfgp.Lists[i].Name)
		}
	}

	recs, sim := slices.Clone(e.Recommendations), slices.Clone(e.Similar)
	markInLibrary(typ, lists, recs)
	markInLibrary(typ, lists, sim)

	c.String(http.StatusOK, renderComponentToString(renderExtras(typ, e, recs, sim)))
}

func renderExtras(typ string, e *libExtras, recs, sim []extraTitle) gomponents.Node {
	section := func(title string, n int, body gomponents.Node) gomponents.Node {
		if n == 0 {
			return nil
		}

		return html.Div(html.Class("mb-4"), html.H6(html.Class("text-muted mb-2"), gomponents.Text(title)), body)
	}

	row := func(nodes []gomponents.Node) gomponents.Node {
		return html.Div(html.Class("d-flex gap-3 overflow-auto pb-2 lib-extras-row"), gomponents.Group(nodes))
	}

	cast := make([]gomponents.Node, 0, 20)
	for _, m := range e.Cast[:min(len(e.Cast), 20)] {
		img := gomponents.Node(html.Div(html.Class("lib-extras-face d-flex align-items-center justify-content-center bg-secondary-subtle text-secondary"),
			html.I(html.Class("fa-solid fa-user fa-2x"))))
		if m.ProfilePath != "" {
			img = html.Img(html.Class("lib-extras-face"), html.Src(tmdbImageBase+"w185"+m.ProfilePath), html.Alt(""), gomponents.Attr("loading", "lazy"))
		}

		cast = append(cast, html.Div(html.Class("lib-extras-item"), html.Role("button"),
			html.Title(tf("Show the titles with %s", m.Name)), gomponents.Attr("onclick", "libShowCast("+jsString(typ)+", "+jsString(m.Name)+")"), img,
			html.Div(html.Class("small fw-semibold text-truncate"), html.Title(m.Name), gomponents.Text(m.Name)),
			html.Div(html.Class("small text-muted text-truncate"), html.Title(m.Character), gomponents.Text(m.Character))))
	}

	videos := slices.Clone(e.Videos)
	slices.SortStableFunc(videos, func(a, b apiexternal_v2.Video) int {
		order := map[string]int{"Trailer": 0, "Teaser": 1}
		ra, ok := order[a.Type]
		if !ok {
			ra = 2
		}

		rb, ok := order[b.Type]
		if !ok {
			rb = 2
		}

		return cmp.Or(cmp.Compare(ra, rb), b.PublishedAt.Compare(a.PublishedAt))
	})

	vids := make([]gomponents.Node, 0, len(videos))
	for _, v := range videos {
		if v.Site != "YouTube" || v.Key == "" {
			continue
		}

		vids = append(vids, html.Button(html.Type("button"), html.Class("btn p-0 border-0 text-start lib-extras-video"),
			gomponents.Attr("onclick", "libShowTrailer({key: "+jsString(v.Key)+", name: "+jsString(v.Name)+", language: "+jsString(v.Language)+"})"),
			html.Img(html.Src("https://i.ytimg.com/vi/"+v.Key+"/mqdefault.jpg"), html.Alt(""), gomponents.Attr("loading", "lazy")),
			html.Div(html.Class("small fw-semibold text-truncate"), html.Title(v.Name), gomponents.Text(v.Name)),
			html.Div(html.Class("small text-muted"), gomponents.Text(strings.TrimSpace(v.Type+" · "+strings.ToUpper(v.Language))))))
	}

	titles := func(ts []extraTitle) gomponents.Node {
		nodes := make([]gomponents.Node, 0, len(ts))
		for _, ti := range ts[:min(len(ts), 20)] {
			img := gomponents.Node(html.Div(html.Class("lib-extras-poster bg-secondary-subtle")))
			if ti.Poster != "" {
				img = html.Img(html.Class("lib-extras-poster"), html.Src(tmdbImageBase+"w185"+ti.Poster), html.Alt(""), gomponents.Attr("loading", "lazy"))
			}

			sub := ""
			if ti.Year > 0 {
				sub = strconv.Itoa(ti.Year)
			}

			if ti.Rating > 0 {
				sub = strings.TrimPrefix(sub+" · ★ "+strconv.FormatFloat(ti.Rating, 'f', 1, 64), " · ")
			}

			attrs := []gomponents.Node{html.Class("lib-extras-item"), html.Title(ti.Title)}

			switch {
			case ti.EntryID > 0:
				attrs = append(attrs, html.Role("button"), gomponents.Attr("onclick", "openLibraryItem("+jsString(typ)+", "+strconv.FormatUint(uint64(ti.EntryID), 10)+")"))
			case ti.ID > 0:
				attrs = append(attrs, html.Role("button"), gomponents.Attr("onclick", "window.open("+
					jsString("https://www.themoviedb.org/"+map[bool]string{true: "movie", false: "tv"}[typ == "movie"]+"/"+strconv.Itoa(ti.ID))+", '_blank', 'noopener')"))
			}

			nodes = append(nodes, html.Div(append(attrs,
				html.Div(html.Class("position-relative"), img,
					gomponents.If(ti.EntryID > 0, html.Span(html.Class("badge bg-success position-absolute top-0 end-0 m-1"), gomponents.Text(t("In library"))))),
				html.Div(html.Class("small fw-semibold text-truncate"), gomponents.Text(ti.Title)),
				html.Div(html.Class("small text-muted"), gomponents.Text(sub)))...))
		}

		return row(nodes)
	}

	reviews := make([]gomponents.Node, 0, 5)
	for _, r := range e.Reviews[:min(len(e.Reviews), 5)] {
		head := r.Author
		if r.Rating > 0 {
			head += " · ★ " + strconv.FormatFloat(r.Rating, 'f', 1, 64)
		}

		if len(r.CreatedAt) >= 10 {
			head += " · " + r.CreatedAt[:10]
		}

		short, rest := r.Content, ""
		if runes := []rune(r.Content); len(runes) > 400 {
			short, rest = string(runes[:400]), string(runes[400:])
		}

		reviews = append(reviews, html.Div(html.Class("border rounded p-2 mb-2"),
			html.Div(html.Class("small fw-semibold mb-1"), gomponents.Text(head)),
			html.Div(html.Class("small"), html.Style("white-space:pre-line"), gomponents.Text(short),
				gomponents.If(rest != "", html.Details(html.Class("d-inline"), html.Summary(html.Class("d-inline text-primary"), gomponents.Text(t("… more"))),
					gomponents.Text(rest))))))
	}

	return html.Div(
		html.StyleEl(gomponents.Raw(`.lib-extras-item{flex:0 0 120px;width:120px;cursor:default}
.lib-extras-item[role=button]{cursor:pointer}
.lib-extras-face{width:120px;height:150px;object-fit:cover;border-radius:.4rem;display:block;margin-bottom:.25rem}
.lib-extras-poster{width:120px;height:180px;object-fit:cover;border-radius:.4rem;display:block;margin-bottom:.25rem}
.lib-extras-video{flex:0 0 220px;width:220px}
.lib-extras-video img{width:220px;height:124px;object-fit:cover;border-radius:.4rem;display:block;margin-bottom:.25rem}`)),
		section(t("Cast"), len(cast), row(cast)),
		section(t("Videos"), len(vids), row(vids)),
		section(t("Recommendations"), len(recs), titles(recs)),
		section(map[bool]string{true: t("Related"), false: t("Similar")}[e.Source == "Trakt"], len(sim), titles(sim)),
		section(t("Reviews"), len(reviews), gomponents.Group(reviews)),
		html.P(html.Class("small text-muted mb-0"), gomponents.Text("Source: "+e.Source)),
	)
}

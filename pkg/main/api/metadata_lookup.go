package api

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/goccy/go-json"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// mdAppProvider is the provider key of the app's own lookup, which combines
// the configured sources and can update the database.
const mdAppProvider = "app"

var (
	errMDNotConfigured = errors.New("this provider is not configured (missing API key or disabled)")
	errMDBadID         = errors.New("the ID has the wrong format for this provider")
	errMDNoResult      = errors.New("the provider found nothing for this ID")
)

// mdProvider is one entry of the metadata page's provider dropdown.
type mdProvider struct {
	Kind    string // movie, series, episode, book, audiobook, music
	Key     string
	Label   string
	IDLabel string // what the ID field holds for this provider
	Test    string // an ID known to return a result
	// lookup is nil for the app lookup, which runs the existing handlers.
	lookup func(ctx context.Context, id string, season, episode int) (any, error)
}

func mdInt(id string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(id))
	if err != nil {
		return 0, errMDBadID
	}

	return n, nil
}

// mdProviders lists every metadata provider per media type with a test ID.
// The test values: The Matrix (1999), Breaking Bad S01E01, Dune (Frank
// Herbert), Project Hail Mary (audiobook) and Daft Punk - Discovery (Homework
// for TheAudioDB, whose free key lists only that album).
var mdProviders = []mdProvider{
	{Kind: "movie", Key: mdAppProvider, Label: "App lookup (all sources, can update the database)", IDLabel: "IMDb ID", Test: "tt0133093"},
	{
		Kind: "movie", Key: "imdb", Label: "IMDb (local database)", IDLabel: "IMDb ID", Test: "tt0133093",
		lookup: func(_ context.Context, id string, _, _ int) (any, error) { return imdbLocalTitle(id) },
	},
	{
		Kind: "movie", Key: "tmdb", Label: "TMDB", IDLabel: "TMDB movie ID", Test: "603",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetTMDB(); p != nil {
				return p.GetMovieByID(ctx, n)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "movie", Key: "omdb", Label: "OMDb", IDLabel: "IMDb ID", Test: "tt0133093",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetOMDB(); p != nil {
				return p.GetDetailsByIMDb(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "movie", Key: "trakt", Label: "Trakt", IDLabel: "IMDb ID", Test: "tt0133093",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if providers.GetTrakt() == nil {
				return nil, errMDNotConfigured
			}

			return providers.TraktMovieByIMDb(ctx, id)
		},
	},

	{Kind: "series", Key: mdAppProvider, Label: "App lookup (all sources, can update the database)", IDLabel: "IMDb ID (tt...) or TVDB ID", Test: "81189"},
	{
		Kind: "series", Key: "imdb", Label: "IMDb (local database)", IDLabel: "IMDb ID", Test: "tt0903747",
		lookup: func(_ context.Context, id string, _, _ int) (any, error) { return imdbLocalTitle(id) },
	},
	{
		Kind: "series", Key: "tvdb", Label: "TheTVDB", IDLabel: "TVDB series ID", Test: "81189",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetTVDB(); p != nil {
				return p.GetSeriesByID(ctx, n, "eng")
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "series", Key: "tmdb", Label: "TMDB", IDLabel: "TMDB series ID", Test: "1396",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetTMDB(); p != nil {
				return p.GetSeriesByID(ctx, n)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "series", Key: "trakt", Label: "Trakt", IDLabel: "IMDb ID", Test: "tt0903747",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if providers.GetTrakt() == nil {
				return nil, errMDNotConfigured
			}

			return providers.TraktSerieByIMDb(ctx, id)
		},
	},
	{
		Kind: "series", Key: "tvmaze", Label: "TVmaze", IDLabel: "TVmaze show ID", Test: "169",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetTVMaze(); p != nil {
				return p.GetSeriesByID(ctx, n)
			}

			return nil, errMDNotConfigured
		},
	},

	{Kind: "episode", Key: mdAppProvider, Label: "App lookup (all sources, can update the database)", IDLabel: "Series IMDb ID (tt...) or TVDB ID", Test: "81189"},
	{
		Kind: "episode", Key: "imdb", Label: "IMDb (local database)",
		IDLabel: "Episode IMDb ID (needs tvEpisode in indexed_types)", Test: "tt0959621",
		lookup: func(_ context.Context, id string, _, _ int) (any, error) { return imdbLocalTitle(id) },
	},
	{
		Kind: "episode", Key: "tvdb", Label: "TheTVDB", IDLabel: "TVDB series ID", Test: "81189",
		lookup: func(ctx context.Context, id string, season, episode int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetTVDB(); p != nil {
				return p.GetEpisodeDetails(ctx, n, season, episode)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "episode", Key: "tmdb", Label: "TMDB", IDLabel: "TMDB series ID", Test: "1396",
		lookup: func(ctx context.Context, id string, season, episode int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetTMDB(); p != nil {
				return p.GetEpisodeDetails(ctx, n, season, episode)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "episode", Key: "trakt", Label: "Trakt", IDLabel: "Series IMDb ID", Test: "tt0903747",
		lookup: func(ctx context.Context, id string, season, episode int) (any, error) {
			p := providers.GetTrakt()
			if p == nil {
				return nil, errMDNotConfigured
			}

			series, err := providers.TraktSerieByIMDb(ctx, id)
			if err != nil {
				return nil, err
			}

			return p.GetEpisodeDetails(ctx, series.ID, season, episode)
		},
	},

	{Kind: "book", Key: mdAppProvider, Label: "App lookup (all sources, can update the database)", IDLabel: "ISBN", Test: "9780441013593"},
	{
		Kind: "book", Key: "openlibrary", Label: "OpenLibrary", IDLabel: "OpenLibrary work ID", Test: "OL893414W",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetOpenLibrary(); p != nil {
				return p.GetBookByID(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "book", Key: "goodreads", Label: "Goodreads", IDLabel: "Goodreads book ID", Test: "234225",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetGoodreads(); p != nil {
				return p.GetBookByID(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},

	{Kind: "audiobook", Key: mdAppProvider, Label: "App lookup (all sources, can update the database)", IDLabel: "ASIN", Test: "B08G9PRS1K"},
	{
		Kind: "audiobook", Key: "openlibrary", Label: "OpenLibrary (book)", IDLabel: "OpenLibrary work ID", Test: "OL21745884W",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetOpenLibrary(); p != nil {
				return p.GetBookByID(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "audiobook", Key: "goodreads", Label: "Goodreads (book)", IDLabel: "Goodreads book ID", Test: "54493402",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetGoodreads(); p != nil {
				return p.GetBookByID(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "audiobook", Key: "audnex", Label: "Audnex", IDLabel: "ASIN", Test: "B08G9PRS1K",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetAudnex(); p != nil {
				return p.GetBookByASIN(ctx, id, "us")
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "audiobook", Key: "audible", Label: "Audible (US)", IDLabel: "ASIN", Test: "B08G9PRS1K",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			return providers.SharedAudible(string(audible.RegionUS)).SearchByASIN(ctx, id)
		},
	},

	{Kind: "music", Key: mdAppProvider, Label: "App lookup (all sources, can update the database)", IDLabel: "MusicBrainz release ID", Test: "30e45d86-707d-4042-9df5-93724557aadb"},
	{
		Kind: "music", Key: "musicbrainz", Label: "MusicBrainz", IDLabel: "MusicBrainz release ID", Test: "30e45d86-707d-4042-9df5-93724557aadb",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetMusicBrainz(); p != nil {
				return p.GetReleaseByID(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "music", Key: "discogs", Label: "Discogs", IDLabel: "Discogs release ID", Test: "2879",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetDiscogs(); p != nil {
				return p.GetReleaseByID(ctx, n)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "music", Key: "deezer", Label: "Deezer", IDLabel: "Deezer album ID", Test: "302127",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetDeezer(); p != nil {
				return p.GetAlbumByID(ctx, n)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "music", Key: "itunes", Label: "iTunes", IDLabel: "iTunes collection ID", Test: "697194953",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			n, err := mdInt(id)
			if err != nil {
				return nil, err
			}

			if p := providers.GetITunes(); p != nil {
				return p.GetAlbumTracks(ctx, n)
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "music", Key: "lastfm", Label: "Last.fm", IDLabel: "Artist | Album", Test: "Daft Punk | Discovery",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			artist, album, ok := strings.Cut(id, "|")
			if !ok {
				return nil, errMDBadID
			}

			if p := providers.GetLastFM(); p != nil {
				return p.GetAlbumInfo(ctx, strings.TrimSpace(artist), strings.TrimSpace(album), "")
			}

			return nil, errMDNotConfigured
		},
	},
	{
		Kind: "music", Key: "theaudiodb", Label: "TheAudioDB", IDLabel: "TheAudioDB album ID", Test: "2112973",
		lookup: func(ctx context.Context, id string, _, _ int) (any, error) {
			if p := providers.GetTheAudioDB(); p != nil {
				return p.GetTracksByAlbumID(ctx, id)
			}

			return nil, errMDNotConfigured
		},
	},
}

// mdKinds are the media types in dropdown order.
var mdKinds = []string{"movie", "series", "episode", "book", "audiobook", "music"}

func findMDProvider(kind, key string) (*mdProvider, bool) {
	if key == "" {
		key = mdAppProvider
	}

	i := slices.IndexFunc(mdProviders, func(p mdProvider) bool { return p.Kind == kind && p.Key == key })
	if i < 0 {
		return nil, false
	}

	return &mdProviders[i], true
}

// mdProvidersJSON is the provider table the page script fills the dropdown,
// the ID label and the test value from.
func mdProvidersJSON() string {
	type entry struct {
		Key     string `json:"key"`
		Label   string `json:"label"`
		IDLabel string `json:"idLabel"`
		Test    string `json:"test"`
	}

	byKind := make(map[string][]entry, len(mdKinds))
	for _, p := range mdProviders {
		byKind[p.Kind] = append(byKind[p.Kind], entry{p.Key, p.Label, p.IDLabel, p.Test})
	}

	b, _ := json.Marshal(byKind)

	return string(b)
}

// runMDLookup runs a provider's direct lookup and renders what it returned.
func runMDLookup(p *mdProvider, id string, season, episode int) string {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	start := time.Now()
	result, err := p.lookup(ctx, id, season, episode)
	took := time.Since(start).Round(time.Millisecond)

	if err == nil && isNilResult(result) {
		err = errMDNoResult
	}

	if err != nil {
		return renderAlert(fmt.Sprintf("%s lookup failed after %s: %v", p.Label, took, err), "danger")
	}

	return renderComponentToString(renderMDResult(p, id, took, result))
}

func isNilResult(v any) bool {
	if v == nil {
		return true
	}

	b, err := json.Marshal(v)

	return err == nil && string(b) == "null"
}

// mdRows flattens a provider result into field / value rows: scalars as
// they are, lists and objects as compact JSON. Empty values are left out.
func mdRows(result any) [][2]string {
	b, err := json.Marshal(result)
	if err != nil {
		return [][2]string{{"error", err.Error()}}
	}

	var fields map[string]any
	if err := json.Unmarshal(b, &fields); err != nil {
		return [][2]string{{"value", string(b)}}
	}

	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	rows := make([][2]string, 0, len(keys))

	for _, k := range keys {
		var v string

		switch t := fields[k].(type) {
		case nil:
			continue
		case string:
			v = t
		case bool:
			if !t {
				continue
			}

			v = "true"
		case float64:
			if t == 0 {
				continue
			}

			v = strconv.FormatFloat(t, 'f', -1, 64)
		default:
			raw, _ := json.Marshal(t)
			v = string(raw)

			if v == "[]" || v == "{}" {
				continue
			}

			if len(v) > 600 {
				v = v[:600] + "…"
			}
		}

		if v == "" || v == "0001-01-01T00:00:00Z" {
			continue
		}

		rows = append(rows, [2]string{k, v})
	}

	return rows
}

func renderMDResult(p *mdProvider, id string, took time.Duration, result any) gomponents.Node {
	rows := mdRows(result)

	trs := make([]gomponents.Node, 0, len(rows))
	for _, r := range rows {
		trs = append(trs, html.Tr(
			html.Th(html.Class("text-muted fw-normal text-nowrap"), html.Style("width: 14rem;"), gomponents.Text(r[0])),
			html.Td(html.Style("word-break: break-word;"), gomponents.Text(r[1])),
		))
	}

	return html.Div(
		html.Class("card border-0 shadow-sm"),
		html.Div(
			html.Class("card-header bg-transparent d-flex justify-content-between align-items-center"),
			html.Strong(gomponents.Text(p.Label+" · "+p.IDLabel+" "+id)),
			html.Span(html.Class("badge bg-success"), gomponents.Textf("%d fields · %s", len(rows), took)),
		),
		html.Div(
			html.Class("card-body p-0"),
			html.Table(html.Class("table table-sm table-striped mb-0"), html.TBody(trs...)),
		),
	)
}

// imdbTitle is a row of the local IMDb database with its rating.
type imdbTitle struct {
	Tconst         string  `db:"tconst" json:"imdb_id"`
	TitleType      string  `db:"title_type" json:"title_type"`
	PrimaryTitle   string  `db:"primary_title" json:"primary_title"`
	OriginalTitle  string  `db:"original_title" json:"original_title"`
	IsAdult        bool    `db:"is_adult" json:"is_adult"`
	StartYear      int     `db:"start_year" json:"start_year"`
	EndYear        int     `db:"end_year" json:"end_year"`
	RuntimeMinutes int     `db:"runtime_minutes" json:"runtime_minutes"`
	Genres         string  `db:"genres" json:"genres"`
	Rating         float64 `db:"rating" json:"rating"`
	Votes          int     `db:"votes" json:"votes"`

	AlternativeTitles []string `db:"-" json:"alternative_titles,omitempty"`
}

var errMDNotInIMDb = errors.New("not in the local IMDb database - check that its title type is in [imdbindexer] indexed_types and that the IMDb import has run")

// imdbLocalTitle reads a title, its rating and alternative titles from the
// local IMDb database (the init_imdb import).
func imdbLocalTitle(id string) (*imdbTitle, error) {
	id = strings.TrimSpace(id)
	if !strings.HasPrefix(id, "tt") {
		return nil, errMDBadID
	}

	rows := database.StructscanT[imdbTitle](true, 1,
		"select t.tconst, coalesce(t.title_type, '') as title_type, coalesce(t.primary_title, '') as primary_title, "+
			"coalesce(t.original_title, '') as original_title, coalesce(t.is_adult, 0) as is_adult, "+
			"coalesce(t.start_year, 0) as start_year, coalesce(t.end_year, 0) as end_year, "+
			"coalesce(t.runtime_minutes, 0) as runtime_minutes, coalesce(t.genres, '') as genres, "+
			"coalesce(r.average_rating, 0) as rating, coalesce(r.num_votes, 0) as votes "+
			"from imdb_titles t left join imdb_ratings r on r.tconst = t.tconst where t.tconst = ?", id)
	if len(rows) == 0 {
		return nil, errMDNotInIMDb
	}

	t := &rows[0]
	t.AlternativeTitles = database.GetrowsN[string](true, 50,
		"select distinct title || case when region != '' then ' [' || region || ']' else '' end "+
			"from imdb_akas where tconst = ? and title != ? order by region limit 50", id, t.PrimaryTitle)

	return t, nil
}

package metadata

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/jellyfin"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/plex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// The user's own Plex and Jellyfin servers as metadata sources. Neither
// server can be searched by external ID, so each library is listed once and
// kept as an index (IMDB/TMDb/TVDB ID -> item) for msIndexTTL.

const (
	sourcePlex     = "plex"
	sourceJellyfin = "jellyfin"

	msMovie = "movie"
	msShow  = "show"

	msIndexTTL = time.Hour
)

var errMediaServerOff = errors.New("media server source is not enabled or not configured")

// msItem is a library item of either server.
type msItem struct {
	Key      string // Plex ratingKey or Jellyfin item ID
	Title    string
	Overview string
	Tagline  string
	Genres   string
	Year     int
	Runtime  int // minutes
	Released time.Time
	Season   int    // episodes
	Episode  int    // episodes
	Image    string // Plex thumb path, or "1" when Jellyfin has a primary image
}

type msIndex struct {
	built  time.Time
	byIMDb map[string]*msItem
	byTMDb map[int]*msItem
	byTVDb map[int]*msItem
}

var (
	msIndexMu sync.Mutex
	msIndexes = map[string]*msIndex{} // "<server>/<type>"
)

// mediaServerEnabled reports whether a server source is switched on for
// movies or shows and its server is configured.
func mediaServerEnabled(server, kind string) bool {
	g := config.GetSettingsGeneral()

	switch server {
	case sourcePlex:
		on := g.MovieMetaSourcePlex
		if kind == msShow {
			on = g.SerieMetaSourcePlex
		}

		return on && g.PlexServerURL != "" && g.PlexToken != "" && providers.GetPlex() != nil
	case sourceJellyfin:
		on := g.MovieMetaSourceJellyfin
		if kind == msShow {
			on = g.SerieMetaSourceJellyfin
		}

		return on && g.JellyfinServerURL != "" && g.JellyfinToken != "" && providers.GetJellyfin() != nil
	}

	return false
}

func parseMSDate(s string) time.Time {
	for _, layout := range []string{"2006-01-02", time.RFC3339, "2006-01-02T15:04:05.0000000Z"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}

	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t
		}
	}

	return time.Time{}
}

func fromPlex(it *plex.LibraryItem) *msItem {
	return &msItem{
		Key: it.RatingKey, Title: it.Title, Overview: it.Summary, Tagline: it.Tagline, Genres: it.Genres(),
		Year: it.Year, Runtime: it.Duration / 60000, Released: parseMSDate(it.OriginallyAvailableAt),
		Season: it.ParentIndex, Episode: it.Index, Image: it.Thumb,
	}
}

func fromJellyfin(it *jellyfin.LibraryItem) *msItem {
	m := &msItem{
		Key: it.Id, Title: it.Name, Overview: it.Overview, Year: it.ProductionYear, Runtime: it.RuntimeMinutes(),
		Released: parseMSDate(it.PremiereDate), Season: it.ParentIndexNumber, Episode: it.IndexNumber,
	}

	if it.ImageTags["Primary"] != "" {
		m.Image = "1"
	}

	if len(it.Taglines) > 0 {
		m.Tagline = it.Taglines[0]
	}

	for i, g := range it.Genres {
		if i > 0 {
			m.Genres += ", "
		}

		m.Genres += g
	}

	return m
}

// mediaServerIndex returns the (cached) index of a server's movies or shows.
func mediaServerIndex(ctx context.Context, server, kind string) (*msIndex, error) {
	key := server + "/" + kind

	msIndexMu.Lock()
	defer msIndexMu.Unlock()

	if idx := msIndexes[key]; idx != nil && time.Since(idx.built) < msIndexTTL {
		return idx, nil
	}

	g := config.GetSettingsGeneral()
	idx := &msIndex{built: time.Now(), byIMDb: map[string]*msItem{}, byTMDb: map[int]*msItem{}, byTVDb: map[int]*msItem{}}

	add := func(item *msItem, imdb string, tmdb, tvdb int) {
		if imdb != "" {
			idx.byIMDb[imdb] = item
		}

		if tmdb > 0 {
			idx.byTMDb[tmdb] = item
		}

		if tvdb > 0 {
			idx.byTVDb[tvdb] = item
		}
	}

	switch server {
	case sourcePlex:
		items, err := providers.GetPlex().LibraryItems(ctx, g.PlexServerURL, g.PlexToken, kind)
		if err != nil {
			return nil, err
		}

		for i := range items {
			add(fromPlex(&items[i]), items[i].IMDbID(), items[i].TMDbID(), items[i].TVDbID())
		}
	case sourceJellyfin:
		itemType := "Movie"
		if kind == msShow {
			itemType = "Series"
		}

		items, err := providers.GetJellyfin().LibraryItems(ctx, g.JellyfinServerURL, g.JellyfinToken, itemType)
		if err != nil {
			return nil, err
		}

		for i := range items {
			add(fromJellyfin(&items[i]), items[i].IMDbID(), items[i].TMDbID(), items[i].TVDbID())
		}
	}

	msIndexes[key] = idx

	return idx, nil
}

// mediaServerLookup finds an item by any of its IDs; nil if the server
// doesn't have it.
func mediaServerLookup(server, kind, imdb string, tmdb, tvdb int) (*msItem, error) {
	if !mediaServerEnabled(server, kind) {
		return nil, errMediaServerOff
	}

	idx, err := mediaServerIndex(context.Background(), server, kind)
	if err != nil {
		return nil, err
	}

	if it := idx.byTVDb[tvdb]; tvdb > 0 && it != nil {
		return it, nil
	}

	if it := idx.byIMDb[imdb]; imdb != "" && it != nil {
		return it, nil
	}

	if it := idx.byTMDb[tmdb]; tmdb > 0 && it != nil {
		return it, nil
	}

	return nil, nil
}

// movieGetMediaServerMetadata fills a movie from Plex or Jellyfin.
func movieGetMediaServerMetadata(movie *database.Dbmovie, server string, overwrite bool) error {
	it, err := mediaServerLookup(server, msMovie, movie.ImdbID, movie.MoviedbID, 0)
	if err != nil || it == nil {
		return err
	}

	UpdateString(&movie.Overview, it.Overview, overwrite, nil)
	UpdateString(&movie.Tagline, it.Tagline, overwrite, nil)
	UpdateString(&movie.Genres, it.Genres, overwrite, nil)

	if it.Year > 0 && (movie.Year == 0 || overwrite) {
		movie.Year = uint16(it.Year)
	}

	if it.Runtime > 0 && shouldUpdateRuntime(movie.Runtime, it.Runtime, overwrite) {
		movie.Runtime = it.Runtime
	}

	if !it.Released.IsZero() && (!movie.ReleaseDate.Valid || overwrite) {
		movie.ReleaseDate.Time, movie.ReleaseDate.Valid = it.Released, true
	}

	return nil
}

// serieGetMediaServerMetadata fills a series from Plex or Jellyfin.
func serieGetMediaServerMetadata(serie *database.Dbserie, server string, overwrite bool) error {
	it, err := mediaServerLookup(server, msShow, serie.ImdbID, 0, serie.ThetvdbID)
	if err != nil || it == nil {
		return err
	}

	UpdateString(&serie.Overview, it.Overview, overwrite, nil)
	UpdateString(&serie.Genre, it.Genres, overwrite, nil)

	if !it.Released.IsZero() {
		UpdateString(&serie.Firstaired, it.Released.Format("2006-01-02"), overwrite, nil)
	}

	if it.Runtime > 0 {
		UpdateString(&serie.Runtime, strconv.Itoa(it.Runtime), overwrite, nil)
	}

	return nil
}

// MediaServerEpisode is an episode as Plex or Jellyfin know it.
type MediaServerEpisode struct {
	Season, Episode int
	Title, Overview string
	Aired           time.Time
}

// MediaServerEpisodes returns the episodes the server has of a series; nil
// when the source is off or the server doesn't have the series.
func MediaServerEpisodes(server string, imdb string, tvdb int) ([]MediaServerEpisode, error) {
	it, err := mediaServerLookup(server, msShow, imdb, 0, tvdb)
	if err != nil || it == nil {
		if errors.Is(err, errMediaServerOff) {
			return nil, nil
		}

		return nil, err
	}

	g := config.GetSettingsGeneral()
	ctx := context.Background()

	var items []*msItem

	switch server {
	case sourcePlex:
		eps, err := providers.GetPlex().ShowEpisodes(ctx, g.PlexServerURL, g.PlexToken, it.Key)
		if err != nil {
			return nil, err
		}

		for i := range eps {
			items = append(items, fromPlex(&eps[i]))
		}
	case sourceJellyfin:
		eps, err := providers.GetJellyfin().SeriesEpisodes(ctx, g.JellyfinServerURL, g.JellyfinToken, it.Key)
		if err != nil {
			return nil, err
		}

		for i := range eps {
			items = append(items, fromJellyfin(&eps[i]))
		}
	}

	out := make([]MediaServerEpisode, 0, len(items))
	for _, e := range items {
		if e.Episode > 0 {
			out = append(out, MediaServerEpisode{Season: e.Season, Episode: e.Episode, Title: e.Title, Overview: e.Overview, Aired: e.Released})
		}
	}

	logger.Logtype("debug", 1).Str("server", server).Int("episodes", len(out)).Msg("Media server episodes")

	return out, nil
}

// MediaServerPoster returns the URL of the poster a media server has for a
// movie or series (kind "movie" or "show"), trying the enabled servers in the
// order of the movie or series source priority. The URL carries the server's
// token: fetch it server-side only, never store or log it. server names the
// source for logs.
func MediaServerPoster(kind, imdb string, tmdb, tvdb int) (posterURL, server string) {
	g := config.GetSettingsGeneral()

	priority := g.MovieMetaSourcePriority
	if kind == msShow {
		priority = g.SerieMetaSourcePriority
	}

	for _, src := range priority {
		if src != sourcePlex && src != sourceJellyfin {
			continue
		}

		it, err := mediaServerLookup(src, kind, imdb, tmdb, tvdb)
		if err != nil || it == nil || it.Image == "" {
			continue
		}

		if src == sourcePlex {
			q := url.Values{"width": {"342"}, "height": {"513"}, "minSize": {"1"}, "url": {it.Image}, "X-Plex-Token": {g.PlexToken}}
			return strings.TrimSuffix(g.PlexServerURL, "/") + "/photo/:/transcode?" + q.Encode(), src
		}

		q := url.Values{"maxWidth": {"342"}, "api_key": {g.JellyfinToken}}

		return strings.TrimSuffix(g.JellyfinServerURL, "/") + "/Items/" + url.PathEscape(it.Key) + "/Images/Primary?" + q.Encode(), src
	}

	return "", ""
}

// MediaServerLink is an item's page in a media server's web app.
type MediaServerLink struct {
	Server string `json:"server"`
	URL    string `json:"url"`
}

var (
	msServerIDMu sync.Mutex
	msServerIDs  = map[string]string{} // server -> machine/server ID
)

// mediaServerID returns (and keeps) the ID a server's web links need.
func mediaServerID(ctx context.Context, server string) (string, error) {
	msServerIDMu.Lock()
	defer msServerIDMu.Unlock()

	if id := msServerIDs[server]; id != "" {
		return id, nil
	}

	g := config.GetSettingsGeneral()

	var (
		id  string
		err error
	)

	switch server {
	case sourcePlex:
		id, err = providers.GetPlex().MachineID(ctx, g.PlexServerURL, g.PlexToken)
	case sourceJellyfin:
		id, err = providers.GetJellyfin().ServerID(ctx, g.JellyfinServerURL, g.JellyfinToken)
	}

	if err != nil {
		return "", err
	}

	msServerIDs[server] = id

	return id, nil
}

// MediaServerLinks returns the pages of a movie or series (kind "movie" or
// "show") in the enabled Plex and Jellyfin servers that have it. The links
// hold no token: the web app asks the user to sign in.
func MediaServerLinks(kind, imdb string, tmdb, tvdb int) []MediaServerLink {
	g := config.GetSettingsGeneral()

	var links []MediaServerLink

	for _, server := range []string{sourcePlex, sourceJellyfin} {
		it, err := mediaServerLookup(server, kind, imdb, tmdb, tvdb)
		if err != nil || it == nil {
			continue
		}

		id, err := mediaServerID(context.Background(), server)
		if err != nil || id == "" {
			continue
		}

		if server == sourcePlex {
			links = append(links, MediaServerLink{"Plex", strings.TrimSuffix(g.PlexServerURL, "/") + "/web/index.html#!/server/" + id +
				"/details?key=" + url.QueryEscape("/library/metadata/"+it.Key)})
		} else {
			links = append(links, MediaServerLink{"Jellyfin", strings.TrimSuffix(g.JellyfinServerURL, "/") + "/web/index.html#/details?id=" +
				url.QueryEscape(it.Key) + "&serverId=" + url.QueryEscape(id)})
		}
	}

	return links
}

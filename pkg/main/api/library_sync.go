package api

import (
	"context"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/jellyfin"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/plex"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
)

// The watched state of one account (watched_sync_user) is synced with a
// Plex server (as plex_token's account) and a Jellyfin server (as
// watched_sync_jellyfin_user): what is watched on one side becomes watched
// on the other; unwatching isn't passed on. Where the server stopped in a
// film is taken over when the account has no history of it. Movies match
// by IMDb or TMDB ID, episodes by their series' TVDB or IMDb ID with season
// and episode number.

const watchedSyncJob = "SyncWatchedState"

func init() {
	utils.RegisterGlobalJob(watchedSyncJob, func(ctx context.Context) error {
		syncWatchedState(ctx)
		return nil
	})
}

// syncItem is a movie or episode of a media server with its state.
type syncItem struct {
	Key             string // the server's ID of it
	Episode         bool
	Imdb            string // the movie's, or the series' of an episode
	Tmdb, Tvdb      int    // the same
	Season, Number  int    // of an episode
	Watched         bool
	Position, Total float64 // seconds; Position > 0: stopped there
}

// syncKeys are the keys an item matches by.
func syncKeys(episode bool, imdb string, tmdb, tvdb, season, number int) []string {
	var keys []string

	suffix := ""
	if episode {
		suffix = ":" + strconv.Itoa(season) + ":" + strconv.Itoa(number)
	}

	if strings.HasPrefix(imdb, "tt") {
		keys = append(keys, "imdb:"+imdb+suffix)
	}

	if tmdb > 0 && !episode {
		keys = append(keys, "tmdb:"+strconv.Itoa(tmdb))
	}

	if tvdb > 0 && episode {
		keys = append(keys, "tvdb:"+strconv.Itoa(tvdb)+suffix)
	}

	return keys
}

// syncLocal is a movie or episode of the library: its entry (movies or
// serie_episodes row) and first file.
type syncLocal struct {
	Entry   uint64  `db:"entry"`
	File    uint    `db:"file"`
	Imdb    string  `db:"imdb"`
	Tmdb    int     `db:"tmdb"`
	Tvdb    int     `db:"tvdb"`
	Season  int     `db:"season"`
	Number  int     `db:"number"`
	Watched bool    `db:"watched"`
	Started bool    `db:"started"`
	Runtime float64 `db:"runtime"`
}

// syncLocals loads the account's movies or episodes with files and its
// state of them.
func syncLocals(user string, groups []libGroup, episode bool) []syncLocal {
	w := strconv.FormatFloat(database.Watched, 'f', 2, 64)

	if !episode {
		in, args := listsOf(groupsOfType(groups, "movie"))

		return database.StructscanT[syncLocal](false, 1024, `SELECT e.id AS entry, min(f.id) AS file, COALESCE(d.imdb_id, '') AS imdb,
			COALESCE(d.moviedb_id, 0) AS tmdb, 0 AS tvdb, 0 AS season, 0 AS number,
			EXISTS (SELECT 1 FROM play_history h INNER JOIN movie_files hf ON hf.id = h.file_id WHERE hf.movie_id = e.id
				AND h.username = ? AND h.kind = 'movie' AND h.completion >= `+w+`) AS watched,
			EXISTS (SELECT 1 FROM play_history h INNER JOIN movie_files hf ON hf.id = h.file_id WHERE hf.movie_id = e.id
				AND h.username = ? AND h.kind = 'movie') AS started, COALESCE(max(f.runtime_seconds), 0) AS runtime
			FROM movie_files f INNER JOIN movies e ON e.id = f.movie_id INNER JOIN dbmovies d ON d.id = e.dbmovie_id
			WHERE e.listname`+in+` GROUP BY e.id`, append([]any{user, user}, args...)...)
	}

	in, args := listsOf(groupsOfType(groups, "series"))

	return database.StructscanT[syncLocal](false, 4096, `SELECT se.id AS entry, min(f.id) AS file, COALESCE(d.imdb_id, '') AS imdb, 0 AS tmdb,
		COALESCE(d.thetvdb_id, 0) AS tvdb, CAST(COALESCE(de.season, '0') AS INTEGER) AS season,
		CAST(COALESCE(de.episode, '0') AS INTEGER) AS number,
		EXISTS (SELECT 1 FROM play_history h INNER JOIN serie_episode_files hf ON hf.id = h.file_id WHERE hf.serie_episode_id = se.id
			AND h.username = ? AND h.kind = 'episode' AND h.completion >= `+w+`) AS watched,
		EXISTS (SELECT 1 FROM play_history h INNER JOIN serie_episode_files hf ON hf.id = h.file_id WHERE hf.serie_episode_id = se.id
			AND h.username = ? AND h.kind = 'episode') AS started, COALESCE(max(f.runtime_seconds), 0) AS runtime
		FROM serie_episode_files f INNER JOIN serie_episodes se ON se.id = f.serie_episode_id
		INNER JOIN series s ON s.id = se.serie_id INNER JOIN dbseries d ON d.id = s.dbserie_id
		INNER JOIN dbserie_episodes de ON de.id = se.dbserie_episode_id
		WHERE s.listname`+in+` GROUP BY se.id`, append([]any{user, user}, args...)...)
}

func groupsOfType(groups []libGroup, typ string) []libGroup {
	var out []libGroup

	for _, g := range groups {
		if g.Kind.Type == typ {
			out = append(out, g)
		}
	}

	return out
}

// syncWatched merges a server's state with the account's: watched there
// becomes watched here and the other way (mark), a position there is taken
// over when the account has no history. It returns how many items were
// marked here and there.
func syncWatched(ctx context.Context, user string, groups []libGroup, items []syncItem, mark func(context.Context, string) error) (int, int) {
	locals := map[string]*syncLocal{}

	for _, episode := range []bool{false, true} {
		rows := syncLocals(user, groups, episode)
		for i := range rows {
			r := &rows[i]
			for _, k := range syncKeys(episode, r.Imdb, r.Tmdb, r.Tvdb, r.Season, r.Number) {
				locals[k] = r
			}
		}
	}

	imported, exported := 0, 0
	seen := map[*syncLocal]bool{}

	for _, it := range items {
		if ctx.Err() != nil {
			break
		}

		var l *syncLocal

		for _, k := range syncKeys(it.Episode, it.Imdb, it.Tmdb, it.Tvdb, it.Season, it.Number) {
			if l = locals[k]; l != nil {
				break
			}
		}

		if l == nil || seen[l] {
			continue
		}

		seen[l] = true

		kind, table, column := "movie", "movie_files", "movie_id"
		if it.Episode {
			kind, table, column = "episode", "serie_episode_files", "serie_episode_id"
		}

		switch {
		case it.Watched && !l.Watched:
			if database.SetEntryWatched(user, kind, table, column, &l.Entry, true) == nil {
				imported++
			}
		case l.Watched && !it.Watched:
			if err := mark(ctx, it.Key); err == nil {
				exported++
			} else {
				logger.Logtype("debug", 0).Err(err).Str("item", it.Key).Msg("Watched sync: not marked on the server")
			}
		case !l.Started && it.Position > 0:
			total := it.Total
			if total <= 0 {
				total = l.Runtime
			}

			if total > 0 && database.ImportPlayPosition(user, kind, &l.File, it.Position, total) == nil {
				imported++
			}
		}
	}

	return imported, exported
}

// syncWatchedState runs the sync with the configured servers.
func syncWatchedState(ctx context.Context) {
	g := config.GetSettingsGeneral()

	user := strings.TrimSpace(g.WatchedSyncUser)
	if user == "" {
		return
	}

	acc := adminAccess
	if !strings.EqualFold(user, DefaultUsername) {
		u, err := database.GetUserByName(&user)
		if err != nil || u.ID == 0 {
			logger.Logtype("warn", 0).Str("user", user).Msg("Watched sync: unknown account")
			return
		}

		acc = accessOfUser(u)
	}

	groups := groupsOf(acc, "movie", "series")

	if g.WatchedSyncPlex && g.PlexServerURL != "" && g.PlexToken != "" && providers.GetPlex() != nil {
		items, err := plexSyncItems(ctx, g.PlexServerURL, g.PlexToken)
		if err != nil {
			logger.Logtype("warn", 0).Err(err).Msg("Watched sync: Plex not read")
		} else {
			in, out := syncWatched(ctx, accName(acc), groups, items, func(ctx context.Context, key string) error {
				return providers.GetPlex().Scrobble(ctx, g.PlexServerURL, g.PlexToken, key)
			})
			logger.Logtype("info", 0).Int("from_plex", in).Int("to_plex", out).Msg("Watched sync with Plex")
		}
	}

	if ju := strings.TrimSpace(g.WatchedSyncJellyfinUser); ju != "" && g.JellyfinServerURL != "" && g.JellyfinToken != "" &&
		providers.GetJellyfin() != nil {
		items, err := jellyfinSyncItems(ctx, g.JellyfinServerURL, g.JellyfinToken, ju)
		if err != nil {
			logger.Logtype("warn", 0).Err(err).Msg("Watched sync: Jellyfin not read")
		} else {
			in, out := syncWatched(ctx, accName(acc), groups, items, func(ctx context.Context, key string) error {
				return providers.GetJellyfin().MarkPlayed(ctx, g.JellyfinServerURL, g.JellyfinToken, ju, key)
			})
			logger.Logtype("info", 0).Int("from_jellyfin", in).Int("to_jellyfin", out).Msg("Watched sync with Jellyfin")
		}
	}
}

// plexSyncItems reads the movies and episodes of a Plex server.
func plexSyncItems(ctx context.Context, serverURL, token string) ([]syncItem, error) {
	p := providers.GetPlex()

	movies, err := p.LibraryItems(ctx, serverURL, token, "movie")
	if err != nil {
		return nil, err
	}

	shows, err := p.LibraryItems(ctx, serverURL, token, "show")
	if err != nil {
		return nil, err
	}

	episodes, err := p.AllEpisodes(ctx, serverURL, token)
	if err != nil {
		return nil, err
	}

	out := make([]syncItem, 0, len(movies)+len(episodes))
	for i := range movies {
		out = append(out, plexSyncItem(&movies[i], nil))
	}

	byKey := make(map[string]*plex.LibraryItem, len(shows))
	for i := range shows {
		byKey[shows[i].RatingKey] = &shows[i]
	}

	for i := range episodes {
		if show := byKey[episodes[i].GrandparentRatingKey]; show != nil {
			out = append(out, plexSyncItem(&episodes[i], show))
		}
	}

	return out, nil
}

func plexSyncItem(it *plex.LibraryItem, show *plex.LibraryItem) syncItem {
	s := syncItem{
		Key: it.RatingKey, Watched: it.ViewCount > 0, Position: float64(it.ViewOffset) / 1000, Total: float64(it.Duration) / 1000,
		Imdb: it.IMDbID(), Tmdb: it.TMDbID(),
	}

	if show != nil {
		s.Episode, s.Season, s.Number = true, it.ParentIndex, it.Index
		s.Imdb, s.Tvdb, s.Tmdb = show.IMDbID(), show.TVDbID(), 0
	}

	return s
}

// jellyfinSyncItems reads the movies and episodes of a Jellyfin server as
// a user sees them.
func jellyfinSyncItems(ctx context.Context, serverURL, apiKey, username string) ([]syncItem, error) {
	p := providers.GetJellyfin()

	var out []syncItem

	movies, err := p.UserItems(ctx, serverURL, apiKey, username, "Movie")
	if err != nil {
		return nil, err
	}

	for i := range movies {
		out = append(out, jellyfinSyncItem(&movies[i], nil))
	}

	series, err := p.UserItems(ctx, serverURL, apiKey, username, "Series")
	if err != nil {
		return nil, err
	}

	byID := make(map[string]*jellyfin.LibraryItem, len(series))
	for i := range series {
		byID[series[i].Id] = &series[i]
	}

	episodes, err := p.UserItems(ctx, serverURL, apiKey, username, "Episode")
	if err != nil {
		return nil, err
	}

	for i := range episodes {
		if s := byID[episodes[i].SeriesID]; s != nil {
			out = append(out, jellyfinSyncItem(&episodes[i], s))
		}
	}

	return out, nil
}

func jellyfinSyncItem(it *jellyfin.LibraryItem, series *jellyfin.LibraryItem) syncItem {
	ids := it.ProviderIds
	if series != nil {
		ids = series.ProviderIds
	}

	providerID := func(name string) string {
		for k, v := range ids {
			if strings.EqualFold(k, name) {
				return v
			}
		}

		return ""
	}

	tmdb, _ := strconv.Atoi(providerID("Tmdb"))
	tvdb, _ := strconv.Atoi(providerID("Tvdb"))

	s := syncItem{
		Key: it.Id, Watched: it.UserData.Played, Position: float64(it.UserData.PlaybackPositionTicks) / 1e7,
		Total: float64(it.RunTimeTicks) / 1e7, Imdb: providerID("Imdb"), Tmdb: tmdb,
	}

	if series != nil {
		s.Episode, s.Season, s.Number, s.Tvdb, s.Tmdb = true, it.ParentIndexNumber, it.IndexNumber, tvdb, 0
	}

	return s
}

package importfeed

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// collectedEpisode is one episode gathered from TVDB and/or Trakt before it
// is written to dbserie_episodes.
type collectedEpisode struct {
	Season         int
	Episode        int
	AbsoluteNumber int
	Title          string
	FirstAired     time.Time
	Overview       string
	Poster         string
}

func episodeKey(season, episode int) string {
	return strconv.Itoa(season) + "-" + strconv.Itoa(episode)
}

// collectTvdbSeriesEpisodes fetches all episodes of a series from TVDB,
// keyed by "season-episode".
func collectTvdbSeriesEpisodes(id int, language string) (map[string]*collectedEpisode, error) {
	episodes := make(map[string]*collectedEpisode)

	provider := providers.GetTVDB()
	if provider == nil {
		return episodes, errTvdbNotInitialized
	}

	apiEpisodes, err := provider.GetAllEpisodes(context.Background(), id, language)
	if err != nil {
		logger.Logtype("error", 1).
			Int("series_id", id).
			Err(err).
			Msg("Error getting episodes from v2 provider")

		return episodes, err
	}

	for i := range apiEpisodes {
		ep := &apiEpisodes[i]

		episodes[episodeKey(ep.SeasonNumber, ep.EpisodeNumber)] = &collectedEpisode{
			Season:         ep.SeasonNumber,
			Episode:        ep.EpisodeNumber,
			AbsoluteNumber: ep.AbsoluteNumber,
			Title:          ep.Name,
			FirstAired:     ep.AirDate,
			Overview:       ep.Overview,
			Poster:         ep.StillPath,
		}
	}

	return episodes, nil
}

// mergeTraktIntoCollectedEpisodes fills empty fields of the collected
// episodes from Trakt and adds the episodes only Trakt knows.
func mergeTraktIntoCollectedEpisodes(imdbID string, episodes map[string]*collectedEpisode) {
	provider := providers.GetTrakt()
	if provider == nil || imdbID == "" {
		return
	}

	ctx := context.Background()

	findResult, err := provider.FindSeriesByIMDbID(ctx, imdbID)
	if err != nil || findResult == nil || len(findResult.TVResults) == 0 {
		return
	}

	seriesID := findResult.TVResults[0].ID

	seasons, err := provider.GetAllSeasons(ctx, seriesID)
	if err != nil {
		return
	}

	for i := range seasons {
		seasonEpisodes, err := provider.GetSeasonEpisodes(ctx, seriesID, seasons[i].SeasonNumber)
		if err != nil {
			continue
		}

		for j := range seasonEpisodes {
			ep := seasonEpisodes[j]
			key := episodeKey(ep.SeasonNumber, ep.EpisodeNumber)

			existing, ok := episodes[key]
			if !ok {
				// Trakt has no episode images, so Poster stays empty.
				episodes[key] = &collectedEpisode{
					Season:         ep.SeasonNumber,
					Episode:        ep.EpisodeNumber,
					AbsoluteNumber: ep.AbsoluteNumber,
					Title:          ep.Name,
					FirstAired:     ep.AirDate,
					Overview:       ep.Overview,
				}

				continue
			}

			if existing.Title == "" && ep.Name != "" {
				existing.Title = ep.Name
			}

			if existing.FirstAired.IsZero() && !ep.AirDate.IsZero() {
				existing.FirstAired = ep.AirDate
			}

			if existing.Overview == "" && ep.Overview != "" {
				existing.Overview = ep.Overview
			}

			if existing.AbsoluteNumber == 0 && ep.AbsoluteNumber != 0 {
				existing.AbsoluteNumber = ep.AbsoluteNumber
			}
		}
	}
}

// mergeMediaServerIntoCollectedEpisodes fills gaps from the user's Plex or
// Jellyfin server and adds the episodes only it knows. Nothing happens while
// the source is off or the server lacks the series.
func mergeMediaServerIntoCollectedEpisodes(server, imdb string, tvdb int, episodes map[string]*collectedEpisode) {
	eps, err := metadata.MediaServerEpisodes(server, imdb, tvdb)
	if err != nil {
		logger.Logtype("warn", 1).Err(err).Str("source", server).Int(logger.StrTvdb, tvdb).
			Msg("Get media server episodes")

		return
	}

	for _, ep := range eps {
		key := episodeKey(ep.Season, ep.Episode)

		existing, ok := episodes[key]
		if !ok {
			episodes[key] = &collectedEpisode{
				Season: ep.Season, Episode: ep.Episode, Title: ep.Title, FirstAired: ep.Aired, Overview: ep.Overview,
			}

			continue
		}

		if existing.Title == "" {
			existing.Title = ep.Title
		}

		if existing.Overview == "" {
			existing.Overview = ep.Overview
		}

		if existing.FirstAired.IsZero() {
			existing.FirstAired = ep.Aired
		}
	}
}

// writeCollectedEpisodesToDB updates the series' existing episodes and
// inserts the missing ones.
func writeCollectedEpisodesToDB(episodes map[string]*collectedEpisode, dbid *uint) {
	if len(episodes) == 0 {
		return
	}

	tbl := database.Getrowssize[database.DbstaticTwoString](
		false,
		database.QueryDbserieEpisodesCountByDBID,
		database.QueryDbserieEpisodesGetSeasonEpisodeByDBID,
		dbid,
	)

	for _, ep := range episodes {
		epi := strconv.Itoa(ep.Episode)
		seas := strconv.Itoa(ep.Season)

		// An unknown air date is NULL, not year 1.
		var firstAired any
		if !ep.FirstAired.IsZero() {
			firstAired = ep.FirstAired
		}

		if slices.Contains(tbl, database.DbstaticTwoString{Str1: seas, Str2: epi}) {
			// A refresh that lacks a value (e.g. no overview in the metadata
			// language) keeps the stored one instead of blanking it.

			database.ExecN(
				`UPDATE dbserie_episodes SET
					title = CASE WHEN ? <> '' THEN ? ELSE title END,
					first_aired = COALESCE(?, first_aired),
					overview = CASE WHEN ? <> '' THEN ? ELSE overview END,
					poster = CASE WHEN ? <> '' THEN ? ELSE poster END,
					absolute_episode = CASE WHEN ? <> 0 THEN ? ELSE absolute_episode END,
					updated_at = CURRENT_TIMESTAMP
				WHERE dbserie_id = ? AND season = ? AND episode = ?`,
				&ep.Title, &ep.Title,
				firstAired,
				&ep.Overview, &ep.Overview,
				&ep.Poster, &ep.Poster,
				&ep.AbsoluteNumber, &ep.AbsoluteNumber,
				dbid,
				&seas,
				&epi,
			)

			continue
		}

		ident := fmt.Sprintf("S%02dE%02d", ep.Season, ep.Episode)
		database.ExecN(
			"INSERT INTO dbserie_episodes (episode, season, identifier, title, first_aired, overview, poster, absolute_episode, dbserie_id) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&epi,
			&seas,
			&ident,
			&ep.Title,
			firstAired,
			&ep.Overview,
			&ep.Poster,
			&ep.AbsoluteNumber,
			dbid,
		)
	}
}

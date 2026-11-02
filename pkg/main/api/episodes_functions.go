package api

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
)

// triggerEpisodeDownloads runs a search for each episode. It is meant to run
// inside a worker job; ctx cancellation stops it between episodes.
func triggerEpisodeDownloads(
	ctx context.Context,
	episodes []database.SerieEpisode,
	qualityProfile string,
) *EpisodeDownloadResults {
	results := &EpisodeDownloadResults{
		TotalEpisodes: len(episodes),
		Details:       make([]string, 0),
	}

	for i, episode := range episodes {
		if ctx.Err() != nil {
			results.SkippedEpisodes += len(episodes) - i
			break
		}

		// Episodes the user set to dont_search are never searched here.
		if episode.DontSearch {
			results.SkippedEpisodes++
			continue
		}

		// Get series information
		series := database.StructscanT[database.Serie](false, 1,
			"SELECT * FROM series WHERE id = ?", episode.SerieID)
		if len(series) == 0 {
			results.FailedDownloads++

			results.Details = append(
				results.Details,
				fmt.Sprintf("Series not found for episode ID %d", episode.ID),
			)

			continue
		}

		serieData := series[0]

		// Get episode details
		dbEpisode := database.StructscanT[database.DbserieEpisode](false, 1,
			"SELECT * FROM dbserie_episodes WHERE id = ?", episode.DbserieEpisodeID)
		if len(dbEpisode) == 0 {
			results.FailedDownloads++

			results.Details = append(
				results.Details,
				fmt.Sprintf("Episode details not found for ID %d", episode.ID),
			)

			continue
		}

		episodeData := dbEpisode[0]

		// Find media configuration for this series
		var mediaConfig *config.MediaTypeConfig
		config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
			// Check if this series belongs to this media config
			for _, listName := range media.Lists {
				if listName.Name == serieData.Listname {
					mediaConfig = media
					return errors.New("found") // Break the loop
				}
			}

			return nil
		})

		if mediaConfig == nil {
			results.FailedDownloads++

			results.Details = append(
				results.Details,
				fmt.Sprintf("No media configuration found for series ID %d", serieData.ID),
			)

			continue
		}

		// Get quality configuration
		var qualityConfig *config.QualityConfig
		if qualityProfile != "" && qualityProfile != "default" {
			if qc, exists := config.GetSettingsQualityOk(qualityProfile); exists {
				qualityConfig = qc
			}
		}

		if qualityConfig == nil {
			// Use media config's quality config
			if mediaConfig.CfgQuality != nil {
				qualityConfig = mediaConfig.CfgQuality
			}
		}

		if qualityConfig == nil {
			results.FailedDownloads++

			results.Details = append(
				results.Details,
				fmt.Sprintf("No quality configuration found for series ID %d", serieData.ID),
			)

			continue
		}

		// Trigger search for this episode
		err := triggerEpisodeSearch(ctx, episode.ID, episodeData, mediaConfig, qualityConfig)
		if err != nil {
			results.FailedDownloads++

			seasonNum, _ := strconv.Atoi(episodeData.Season)
			episodeNum, _ := strconv.Atoi(episodeData.Episode)

			results.Details = append(
				results.Details,
				fmt.Sprintf("Search failed for series %d S%02dE%02d: %v",
					serieData.ID, seasonNum, episodeNum, err),
			)
		} else {
			results.TriggeredDownloads++

			seasonNum, _ := strconv.Atoi(episodeData.Season)
			episodeNum, _ := strconv.Atoi(episodeData.Episode)

			results.Details = append(
				results.Details,
				fmt.Sprintf("Search triggered for series %d S%02dE%02d",
					serieData.ID, seasonNum, episodeNum),
			)
		}
	}

	return results
}

// triggerEpisodeSearch initiates a real search for a specific episode.
// serieEpisodeID is the concrete serie_episodes.id (the per-list episode
// instance) - MediaSearch requires this, not the dbserie_episodes.id used
// only for the episode's descriptive metadata.
func triggerEpisodeSearch(
	ctx context.Context,
	serieEpisodeID uint,
	episode database.DbserieEpisode,
	mediaConfig *config.MediaTypeConfig,
	qualityConfig *config.QualityConfig,
) error {
	// Create searcher instance
	searcherInstance := searcher.NewSearcher(
		mediaConfig,
		qualityConfig,
		"search",
		&serieEpisodeID,
	)
	if searcherInstance == nil {
		return errors.New("failed to create searcher instance")
	}
	defer searcherInstance.Close()

	// Log the search attempt
	logger.Logtype("info", 3).
		Str("season", episode.Season).
		Str("episode", episode.Episode).
		Str("title", episode.Title).
		Msg("Triggering episode search")

	// MediaSearch updates serie_episodes.lastscan itself.
	return searcherInstance.MediaSearch(ctx, mediaConfig, serieEpisodeID, false, false, false)
}

// EpisodeDownloadResults holds the results of episode download operations.
type EpisodeDownloadResults struct {
	TotalEpisodes      int
	TriggeredDownloads int
	FailedDownloads    int
	SkippedEpisodes    int
	Details            []string
}

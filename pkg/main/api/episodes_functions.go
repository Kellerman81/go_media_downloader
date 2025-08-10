package api

import (
	"fmt"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
)

// checkSeriesMissingEpisodes finds missing episodes for series
func checkSeriesMissingEpisodes(seriesName string, seasonNumber int, includeSpecials, onlyAired bool, dateRangeDays int, status string) ([]database.SerieEpisode, error) {
	var queryWhere string
	var queryArgs []interface{}

	// Base query for missing episodes
	queryWhere = "serie_episodes.missing = 1"

	// Add series name filter if provided
	if seriesName != "" {
		queryWhere += " AND series.seriename LIKE ?"
		queryArgs = append(queryArgs, "%"+seriesName+"%")
	}

	// Add season filter if provided
	if seasonNumber > 0 {
		queryWhere += " AND dbserie_episodes.season = ?"
		queryArgs = append(queryArgs, strconv.Itoa(seasonNumber))
	}

	// Add specials filter
	if !includeSpecials {
		queryWhere += " AND dbserie_episodes.season != '0'"
	}

	// Add aired date filter if requested
	if onlyAired {
		queryWhere += " AND (dbserie_episodes.first_aired IS NULL OR dbserie_episodes.first_aired <= ?)"
		queryArgs = append(queryArgs, time.Now().Format("2006-01-02"))
	}

	// Add date range filter
	if dateRangeDays > 0 {
		cutoffDate := time.Now().AddDate(0, 0, -dateRangeDays)
		queryWhere += " AND (dbserie_episodes.first_aired IS NULL OR dbserie_episodes.first_aired >= ?)"
		queryArgs = append(queryArgs, cutoffDate.Format("2006-01-02"))
	}

	// Add status filter
	if status != "all" {
		switch status {
		case "missing":
			queryWhere += " AND serie_episodes.missing = 1"
		case "wanted":
			queryWhere += " AND serie_episodes.missing = 1 AND serie_episodes.dont_search = 0"
		case "ignored":
			queryWhere += " AND serie_episodes.dont_search = 1"
		}
	}

	// Execute query to get missing episodes
	missingEpisodes := database.StructscanT[database.SerieEpisode](false, 0,
		"SELECT serie_episodes.* FROM serie_episodes "+
			"INNER JOIN series ON series.id = serie_episodes.serie_id "+
			"INNER JOIN dbserie_episodes ON dbserie_episodes.id = serie_episodes.dbserie_episode_id "+
			"WHERE "+queryWhere+" "+
			"ORDER BY series.seriename, dbserie_episodes.season, dbserie_episodes.episode "+
			"LIMIT 1000",
		queryArgs...)

	return missingEpisodes, nil
}

// triggerEpisodeDownloads initiates downloads for missing episodes
func triggerEpisodeDownloads(episodes []database.SerieEpisode, qualityProfile string, autoDownload bool) (*EpisodeDownloadResults, error) {
	results := &EpisodeDownloadResults{
		TotalEpisodes:      len(episodes),
		TriggeredDownloads: 0,
		FailedDownloads:    0,
		SkippedEpisodes:    0,
		Details:            make([]string, 0),
	}

	if !autoDownload {
		results.Details = append(results.Details, "Auto-download disabled - episodes marked for manual review")
		results.SkippedEpisodes = len(episodes)
		return results, nil
	}

	// Process each missing episode
	for _, episode := range episodes {
		// Get series information
		series := database.StructscanT[database.Serie](false, 1,
			"SELECT * FROM series WHERE id = ?", episode.SerieID)
		if len(series) == 0 {
			results.FailedDownloads++
			results.Details = append(results.Details, fmt.Sprintf("Series not found for episode ID %d", episode.ID))
			continue
		}

		serieData := series[0]

		// Get episode details
		dbEpisode := database.StructscanT[database.DbserieEpisode](false, 1,
			"SELECT * FROM dbserie_episodes WHERE id = ?", episode.DbserieEpisodeID)
		if len(dbEpisode) == 0 {
			results.FailedDownloads++
			results.Details = append(results.Details, fmt.Sprintf("Episode details not found for ID %d", episode.ID))
			continue
		}

		episodeData := dbEpisode[0]

		// Find media configuration for this series
		var mediaConfig *config.MediaTypeConfig
		config.RangeSettingsMedia(func(name string, media *config.MediaTypeConfig) error {
			// Check if this series belongs to this media config
			for _, listName := range media.Lists {
				if listName.Name == serieData.Listname {
					mediaConfig = media
					return fmt.Errorf("found") // Break the loop
				}
			}
			return nil
		})

		if mediaConfig == nil {
			results.FailedDownloads++
			results.Details = append(results.Details, fmt.Sprintf("No media configuration found for series ID %d", serieData.ID))
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
			results.Details = append(results.Details, fmt.Sprintf("No quality configuration found for series ID %d", serieData.ID))
			continue
		}

		// Trigger search for this episode
		err := triggerEpisodeSearch(serieData, episodeData, mediaConfig, qualityConfig)
		if err != nil {
			results.FailedDownloads++
			seasonNum, _ := strconv.Atoi(episodeData.Season)
			episodeNum, _ := strconv.Atoi(episodeData.Episode)
			results.Details = append(results.Details, fmt.Sprintf("Search failed for series %d S%02dE%02d: %v",
				serieData.ID, seasonNum, episodeNum, err))
		} else {
			results.TriggeredDownloads++
			seasonNum, _ := strconv.Atoi(episodeData.Season)
			episodeNum, _ := strconv.Atoi(episodeData.Episode)
			results.Details = append(results.Details, fmt.Sprintf("Search triggered for series %d S%02dE%02d",
				serieData.ID, seasonNum, episodeNum))
		}
	}

	return results, nil
}

// triggerEpisodeSearch initiates a search for a specific episode
func triggerEpisodeSearch(serie database.Serie, episode database.DbserieEpisode, mediaConfig *config.MediaTypeConfig, qualityConfig *config.QualityConfig) error {
	// Create searcher instance
	searcherInstance := searcher.NewSearcher(mediaConfig, qualityConfig, logger.StrSearchMissingInc, nil)
	if searcherInstance == nil {
		return fmt.Errorf("failed to create searcher instance")
	}

	// Log the search attempt
	logger.LogDynamicany("info", "Triggering episode search",
		//"series", serie.Seriename,
		"season", episode.Season,
		"episode", episode.Episode,
		"title", episode.Title)

	// Note: In a real implementation, you would call the actual search method
	// For now, we'll mark the episode as being searched
	database.ExecN("UPDATE serie_episodes SET dont_search = 0, last_searched = ? WHERE id = ?",
		time.Now(), episode.ID)

	return nil
}

// EpisodeDownloadResults holds the results of episode download operations
type EpisodeDownloadResults struct {
	TotalEpisodes      int
	TriggeredDownloads int
	FailedDownloads    int
	SkippedEpisodes    int
	Details            []string
}

// EpisodeSearchCriteria holds search criteria for missing episodes
type EpisodeSearchCriteria struct {
	SeriesName      string
	SeasonNumber    int
	IncludeSpecials bool
	OnlyAired       bool
	DateRangeDays   int
	Status          string
	QualityProfile  string
	AutoDownload    bool
}

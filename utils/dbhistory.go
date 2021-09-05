package utils

import (
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
)

func getHighestMoviePriorityByFiles(movies database.Movie, configEntry config.MediaTypeConfig, quality config.QualityConfig) (minPrio int) {
	counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
	if counter >= 1 {
		foundfiles, _ := database.QueryMovieFiles(database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
		for idxfile := range foundfiles {
			prio := getMovieDBPriority(foundfiles[idxfile], configEntry, quality)
			if minPrio == 0 {
				minPrio = prio
			} else {
				if minPrio < prio {
					minPrio = prio
				}
			}
		}
		return minPrio
	} else {
		return 0
	}
}

func GetHighestMoviePriorityByFiles(movies database.Movie, configEntry config.MediaTypeConfig, quality config.QualityConfig) (minPrio int) {
	counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
	if counter >= 1 {
		foundfiles, _ := database.QueryMovieFiles(database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
		for idxfile := range foundfiles {
			prio := getMovieDBPriority(foundfiles[idxfile], configEntry, quality)
			if minPrio == 0 {
				minPrio = prio
			} else {
				if minPrio < prio {
					minPrio = prio
				}
			}
		}
		return minPrio
	} else {
		return 0
	}
}

func getHighestEpisodePriorityByFiles(episode database.SerieEpisode, configEntry config.MediaTypeConfig, quality config.QualityConfig) int {
	counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episode.ID}})
	minPrio := 0
	if counter >= 1 {
		foundfiles, _ := database.QuerySerieEpisodeFiles(database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episode.ID}})
		for idxfile := range foundfiles {
			prio := getSerieDBPriority(foundfiles[idxfile], configEntry, quality)
			if minPrio == 0 {
				minPrio = prio
			} else {
				if minPrio < prio {
					minPrio = prio
				}
			}
		}
	}
	return minPrio
}

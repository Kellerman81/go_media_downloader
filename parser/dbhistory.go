package parser

import (
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
)

func GetHighestMoviePriorityByFiles(movies database.Movie, configEntry config.MediaTypeConfig, quality config.QualityConfig) (minPrio int) {
	counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
	if counter >= 1 {
		foundfiles, _ := database.QueryMovieFiles(database.Query{Select: "location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack", Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
		for idxfile := range foundfiles {
			prio := GetMovieDBPriority(foundfiles[idxfile], configEntry, quality)
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

func GetHighestEpisodePriorityByFiles(episode database.SerieEpisode, configEntry config.MediaTypeConfig, quality config.QualityConfig) int {
	counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episode.ID}})
	minPrio := 0
	if counter >= 1 {
		foundfiles, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack", Where: "serie_episode_id = ?", WhereArgs: []interface{}{episode.ID}})
		for idxfile := range foundfiles {
			prio := GetSerieDBPriority(foundfiles[idxfile], configEntry, quality)
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

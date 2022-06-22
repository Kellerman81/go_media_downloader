package parser

import (
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

func GetHighestMoviePriorityByFiles(movieid uint, configTemplate string, qualityTemplate string) (minPrio int) {
	foundfiles, err := database.QueryStaticColumnsOneInt("select id from movie_files where movie_id = ?", "select count(id) from movie_files where movie_id = ?", movieid)
	if err != nil {
		return 0
	}
	defer logger.ClearVar(&foundfiles)

	var prio int
	for idx := range foundfiles {
		prio = GetMovieDBPriorityById(uint(foundfiles[idx].Num), configTemplate, qualityTemplate)
		if minPrio == 0 {
			minPrio = prio
		} else {
			if minPrio < prio {
				minPrio = prio
			}
		}
	}
	return minPrio
}

func GetHighestEpisodePriorityByFiles(episodeid uint, configTemplate string, qualityTemplate string) int {
	foundfiles, err := database.QueryStaticColumnsOneInt("select id from serie_episode_files where serie_episode_id = ?", "select count(id) from serie_episode_files where serie_episode_id = ?", episodeid)
	if err != nil {
		return 0
	}
	defer logger.ClearVar(&foundfiles)
	minPrio := 0
	var prio int
	for idx := range foundfiles {
		prio = GetSerieDBPriorityById(uint(foundfiles[idx].Num), configTemplate, qualityTemplate)
		if minPrio == 0 {
			minPrio = prio
		} else {
			if minPrio < prio {
				minPrio = prio
			}
		}
	}
	return minPrio
}

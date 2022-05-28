package parser

import (
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

func querymoviequalityids(query string, querycount string, args ...interface{}) ([]database.MovieFile, error) {
	defer logger.ClearVar(&args)
	rowcount, _ := database.CountRowsStatic(querycount, args...)
	rows, err := database.DB.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	returnarray := make([]database.MovieFile, 0, rowcount)
	defer logger.ClearVar(&returnarray)

	defer rows.Close()
	for rows.Next() {
		var location string
		var resolution_id uint
		var quality_id uint
		var codec_id uint
		var audio_id uint
		var proper bool
		var extended bool
		var repack bool
		var err2 error
		err2 = rows.Scan(&location, &resolution_id, &quality_id, &codec_id, &audio_id, &proper, &extended, &repack)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return nil, err2
		}
		returnarray = append(returnarray, database.MovieFile{Location: location, ResolutionID: resolution_id, QualityID: quality_id, CodecID: codec_id, AudioID: audio_id, Proper: proper, Extended: extended, Repack: repack})
	}
	return returnarray, nil
}

func queryseriequalityids(query string, querycount string, args ...interface{}) ([]database.SerieEpisodeFile, error) {
	defer logger.ClearVar(&args)
	rowcount, _ := database.CountRowsStatic(querycount, args...)
	rows, err := database.DB.Queryx(query, args...)
	if err != nil {
		return nil, err
	}
	returnarray := make([]database.SerieEpisodeFile, 0, rowcount)
	defer logger.ClearVar(&returnarray)

	defer rows.Close()
	for rows.Next() {
		var location string
		var resolution_id uint
		var quality_id uint
		var codec_id uint
		var audio_id uint
		var proper bool
		var extended bool
		var repack bool
		var err2 error
		err2 = rows.Scan(&location, &resolution_id, &quality_id, &codec_id, &audio_id, &proper, &extended, &repack)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return nil, err2
		}
		returnarray = append(returnarray, database.SerieEpisodeFile{Location: location, ResolutionID: resolution_id, QualityID: quality_id, CodecID: codec_id, AudioID: audio_id, Proper: proper, Extended: extended, Repack: repack})
	}
	return returnarray, nil
}

func GetHighestMoviePriorityByFiles(movieid uint, configTemplate string, qualityTemplate string) (minPrio int) {
	counter, _ := database.CountRowsStatic("Select count(id) from movie_files where movie_id = ?", movieid)
	if counter >= 1 {
		foundfiles, err := querymoviequalityids("select location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where movie_id = ?", "select count(id) from movie_files where movie_id = ?", movieid)
		if err != nil {
			return 0
		}
		defer logger.ClearVar(&foundfiles)
		var prio int
		for idx := range foundfiles {
			prio = GetMovieDBPriority(foundfiles[idx], configTemplate, qualityTemplate)
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

func GetHighestEpisodePriorityByFiles(episodeid uint, configTemplate string, qualityTemplate string) int {
	counter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where serie_episode_id = ?", episodeid)
	minPrio := 0
	if counter >= 1 {
		foundfiles, err := queryseriequalityids("select location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where serie_episode_id = ?", "select count(id) from serie_episode_files where serie_episode_id = ?", episodeid)
		if err != nil {
			return 0
		}
		defer logger.ClearVar(&foundfiles)
		var prio int
		for idx := range foundfiles {
			prio = GetSerieDBPriority(foundfiles[idx], configTemplate, qualityTemplate)
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

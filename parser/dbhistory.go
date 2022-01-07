package parser

import (
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

func querymoviequalityids(query string, querycount string, args ...interface{}) ([]database.MovieFile, error) {
	rowcount, _ := database.CountRowsStatic(querycount, args...)
	rows, err := database.DB.Queryx(query, args...)
	if err != nil {
		return []database.MovieFile{}, err
	}
	returnarray := make([]database.MovieFile, 0, rowcount)

	defer rows.Close()
	var location string
	var resolution_id uint
	var quality_id uint
	var codec_id uint
	var audio_id uint
	var proper bool
	var extended bool
	var repack bool
	for rows.Next() {
		err2 := rows.Scan(&location, &resolution_id, &quality_id, &codec_id, &audio_id, &proper, &extended, &repack)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []database.MovieFile{}, err2
		}
		returnarray = append(returnarray, database.MovieFile{Location: location, ResolutionID: resolution_id, QualityID: quality_id, CodecID: codec_id, AudioID: audio_id, Proper: proper, Extended: extended, Repack: repack})
	}
	return returnarray, nil
}

func queryseriequalityids(query string, querycount string, args ...interface{}) ([]database.SerieEpisodeFile, error) {
	rowcount, _ := database.CountRowsStatic(querycount, args...)
	rows, err := database.DB.Queryx(query, args...)
	if err != nil {
		return []database.SerieEpisodeFile{}, err
	}
	returnarray := make([]database.SerieEpisodeFile, 0, rowcount)

	defer rows.Close()
	var location string
	var resolution_id uint
	var quality_id uint
	var codec_id uint
	var audio_id uint
	var proper bool
	var extended bool
	var repack bool
	for rows.Next() {
		err2 := rows.Scan(&location, &resolution_id, &quality_id, &codec_id, &audio_id, &proper, &extended, &repack)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []database.SerieEpisodeFile{}, err2
		}
		returnarray = append(returnarray, database.SerieEpisodeFile{Location: location, ResolutionID: resolution_id, QualityID: quality_id, CodecID: codec_id, AudioID: audio_id, Proper: proper, Extended: extended, Repack: repack})
	}
	return returnarray, nil
}

func GetHighestMoviePriorityByFiles(movies database.Movie, configTemplate string, qualityTemplate string) (minPrio int) {
	counter, _ := database.CountRowsStatic("Select count(id) from movie_files where movie_id = ?", movies.ID)

	//counter, _ := database.CountRows("movie_files", database.Query{Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
	if counter >= 1 {
		foundfiles, _ := querymoviequalityids("select location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where movie_id = ?", "select count(id) from movie_files where movie_id = ?", movies.ID)
		//foundfiles, _ := database.QueryMovieFiles(database.Query{Select: "location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack", Where: "movie_id = ?", WhereArgs: []interface{}{movies.ID}})
		for idxfile := range foundfiles {
			prio := GetMovieDBPriority(foundfiles[idxfile], configTemplate, qualityTemplate)
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

func GetHighestEpisodePriorityByFiles(episode database.SerieEpisode, configTemplate string, qualityTemplate string) int {
	counter, _ := database.CountRowsStatic("Select count(id) from serie_episode_files where serie_episode_id = ?", episode.ID)
	//counter, _ := database.CountRows("serie_episode_files", database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{episode.ID}})
	minPrio := 0
	if counter >= 1 {
		foundfiles, _ := queryseriequalityids("select location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where serie_episode_id = ?", "select count(id) from serie_episode_files where serie_episode_id = ?", episode.ID)
		//foundfiles, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack", Where: "serie_episode_id = ?", WhereArgs: []interface{}{episode.ID}})
		for idxfile := range foundfiles {
			prio := GetSerieDBPriority(foundfiles[idxfile], configTemplate, qualityTemplate)
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

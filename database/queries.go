package database

import (
	"database/sql"
	"errors"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
)

type Query struct {
	Select    string
	Where     string
	WhereArgs []interface{}
	OrderBy   string
	Limit     uint64
	Offset    uint64
	InnerJoin string
}

func GetDbmovie(qu Query) (Dbmovie, error) {
	qu.Limit = 1
	results, err := QueryDbmovie(qu)
	if err != nil {
		return Dbmovie{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return Dbmovie{}, errors.New("no result")
}
func QueryDbmovie(qu Query) ([]Dbmovie, error) {
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("dbmovies", qu)
	if counter == 0 || counterr != nil {
		return []Dbmovie{}, nil
	}
	query := buildquery(columns, "dbmovies", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []Dbmovie{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Dbmovie, 0, counter)
	for rows.Next() {
		item := Dbmovie{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []Dbmovie{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func QueryDbmovieJson(qu Query) ([]DbmovieJson, error) {
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("dbmovies", qu)
	if counter == 0 || counterr != nil {
		return []DbmovieJson{}, nil
	}
	query := buildquery(columns, "dbmovies", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []DbmovieJson{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbmovieJson, 0, counter)
	for rows.Next() {
		item := DbmovieJson{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []DbmovieJson{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetDbmovieTitle(qu Query) (DbmovieTitle, error) {
	qu.Limit = 1
	results, err := QueryDbmovieTitle(qu)
	if err != nil {
		return DbmovieTitle{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return DbmovieTitle{}, errors.New("no result")
}
func QueryDbmovieTitle(qu Query) ([]DbmovieTitle, error) {
	columns := "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("dbmovie_titles", qu)
	if counter == 0 || counterr != nil {
		return []DbmovieTitle{}, nil
	}
	query := buildquery(columns, "dbmovie_titles", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []DbmovieTitle{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbmovieTitle, 0, counter)
	for rows.Next() {
		item := DbmovieTitle{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []DbmovieTitle{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetDbserie(qu Query) (Dbserie, error) {
	qu.Limit = 1
	results, err := QueryDbserie(qu)
	if err != nil {
		return Dbserie{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return Dbserie{}, errors.New("no result")
}
func QueryDbserie(qu Query) ([]Dbserie, error) {
	columns := "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("dbseries", qu)
	if counter == 0 || counterr != nil {
		return []Dbserie{}, nil
	}
	query := buildquery(columns, "Dbseries", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []Dbserie{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Dbserie, 0, counter)
	for rows.Next() {
		item := Dbserie{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []Dbserie{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetDbserieEpisodes(qu Query) (DbserieEpisode, error) {
	qu.Limit = 1
	results, err := QueryDbserieEpisodes(qu)
	if err != nil {
		return DbserieEpisode{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return DbserieEpisode{}, errors.New("no result")
}
func QueryDbserieEpisodes(qu Query) ([]DbserieEpisode, error) {
	columns := "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("dbserie_episodes", qu)
	if counter == 0 || counterr != nil {
		return []DbserieEpisode{}, nil
	}
	query := buildquery(columns, "dbserie_episodes", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []DbserieEpisode{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbserieEpisode, 0, counter)
	for rows.Next() {
		item := DbserieEpisode{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []DbserieEpisode{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}
func GetDbserieAlternates(qu Query) (DbserieAlternate, error) {
	qu.Limit = 1
	results, err := QueryDbserieAlternates(qu)
	if err != nil {
		return DbserieAlternate{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return DbserieAlternate{}, errors.New("no result")
}
func QueryDbserieAlternates(qu Query) ([]DbserieAlternate, error) {
	columns := "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("dbserie_alternates", qu)
	if counter == 0 || counterr != nil {
		return []DbserieAlternate{}, nil
	}
	query := buildquery(columns, "dbserie_alternates", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []DbserieAlternate{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbserieAlternate, 0, counter)
	for rows.Next() {
		item := DbserieAlternate{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []DbserieAlternate{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetSeries(qu Query) (Serie, error) {
	qu.Limit = 1
	results, err := QuerySeries(qu)
	if err != nil {
		return Serie{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return Serie{}, errors.New("no result")
}
func QuerySeries(qu Query) ([]Serie, error) {
	columns := "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("series", qu)
	if counter == 0 || counterr != nil {
		return []Serie{}, nil
	}
	query := buildquery(columns, "series", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []Serie{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Serie, 0, counter)
	for rows.Next() {
		item := Serie{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []Serie{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetSerieEpisodes(qu Query) (SerieEpisode, error) {
	qu.Limit = 1
	results, err := QuerySerieEpisodes(qu)
	if err != nil {
		return SerieEpisode{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return SerieEpisode{}, errors.New("no result")
}
func QuerySerieEpisodes(qu Query) ([]SerieEpisode, error) {
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("serie_episodes", qu)
	if counter == 0 || counterr != nil {
		return []SerieEpisode{}, nil
	}
	query := buildquery(columns, "serie_episodes", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []SerieEpisode{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieEpisode, 0, counter)
	for rows.Next() {
		item := SerieEpisode{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []SerieEpisode{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetSerieEpisodeHistory(qu Query) (SerieEpisodeHistory, error) {
	qu.Limit = 1
	results, err := QuerySerieEpisodeHistory(qu)
	if err != nil {
		return SerieEpisodeHistory{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return SerieEpisodeHistory{}, errors.New("no result")
}
func QuerySerieEpisodeHistory(qu Query) ([]SerieEpisodeHistory, error) {
	columns := "id,created_at,updated_at,title,url,indexer,type,target,downloaded_at,blacklisted,quality_profile,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("serie_episode_histories", qu)
	if counter == 0 || counterr != nil {
		return []SerieEpisodeHistory{}, nil
	}
	query := buildquery(columns, "serie_episode_histories", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []SerieEpisodeHistory{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieEpisodeHistory, 0, counter)
	for rows.Next() {
		item := SerieEpisodeHistory{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []SerieEpisodeHistory{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetSerieEpisodeFiles(qu Query) (SerieEpisodeFile, error) {
	qu.Limit = 1
	results, err := QuerySerieEpisodeFiles(qu)
	if err != nil {
		return SerieEpisodeFile{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return SerieEpisodeFile{}, errors.New("no result")
}
func QuerySerieEpisodeFiles(qu Query) ([]SerieEpisodeFile, error) {
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("serie_episode_files", qu)
	if counter == 0 || counterr != nil {
		return []SerieEpisodeFile{}, nil
	}
	query := buildquery(columns, "serie_episode_files", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []SerieEpisodeFile{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieEpisodeFile, 0, counter)
	for rows.Next() {
		item := SerieEpisodeFile{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []SerieEpisodeFile{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetMovies(qu Query) (Movie, error) {
	qu.Limit = 1
	results, err := QueryMovies(qu)
	if err != nil {
		return Movie{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return Movie{}, errors.New("no result")
}
func QueryMovies(qu Query) ([]Movie, error) {
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("movies", qu)
	if counter == 0 || counterr != nil {
		if counterr != nil {
			logger.Log.Error(counterr)
			return []Movie{}, counterr
		} else {
			return []Movie{}, nil
		}
	}
	query := buildquery(columns, "movies", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []Movie{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Movie, 0, counter)
	for rows.Next() {
		item := Movie{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []Movie{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetMovieFiles(qu Query) (MovieFile, error) {
	qu.Limit = 1
	results, err := QueryMovieFiles(qu)
	if err != nil {
		return MovieFile{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return MovieFile{}, errors.New("no result")
}
func QueryMovieFiles(qu Query) ([]MovieFile, error) {
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("movie_files", qu)
	if counter == 0 || counterr != nil {
		return []MovieFile{}, nil
	}
	query := buildquery(columns, "movie_files", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []MovieFile{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]MovieFile, 0, counter)
	for rows.Next() {
		item := MovieFile{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []MovieFile{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetMovieHistory(qu Query) (MovieHistory, error) {
	qu.Limit = 1
	results, err := QueryMovieHistory(qu)
	if err != nil {
		return MovieHistory{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return MovieHistory{}, errors.New("no result")
}
func QueryMovieHistory(qu Query) ([]MovieHistory, error) {
	columns := "id,created_at,updated_at,title,url,indexer,type,target,downloaded_at,blacklisted,quality_profile,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("movie_histories", qu)
	if counter == 0 || counterr != nil {
		return []MovieHistory{}, nil
	}
	query := buildquery(columns, "movie_histories", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []MovieHistory{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]MovieHistory, 0, counter)
	for rows.Next() {
		item := MovieHistory{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []MovieHistory{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetRssHistory(qu Query) (RSSHistory, error) {
	qu.Limit = 1
	results, err := QueryRssHistory(qu)
	if err != nil {
		return RSSHistory{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return RSSHistory{}, errors.New("no result")
}
func QueryRssHistory(qu Query) ([]RSSHistory, error) {
	columns := "id,created_at,updated_at,config,list,indexer,last_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("r_sshistories", qu)
	if counter == 0 || counterr != nil {
		return []RSSHistory{}, nil
	}
	query := buildquery(columns, "r_sshistories", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []RSSHistory{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]RSSHistory, 0, counter)
	for rows.Next() {
		item := RSSHistory{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []RSSHistory{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}
func GetQualities(qu Query) (Qualities, error) {
	qu.Limit = 1
	results, err := QueryQualities(qu)
	if err != nil {
		return Qualities{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return Qualities{}, errors.New("no result")
}
func QueryQualities(qu Query) ([]Qualities, error) {
	columns := "id,created_at,updated_at,type,name,regex,strings,priority,use_regex"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("qualities", qu)
	if counter == 0 || counterr != nil {
		return []Qualities{}, nil
	}
	query := buildquery(columns, "qualities", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []Qualities{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Qualities, 0, counter)
	for rows.Next() {
		item := Qualities{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []Qualities{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}
func GetJobHistory(qu Query) (JobHistory, error) {
	qu.Limit = 1
	results, err := QueryJobHistory(qu)
	if err != nil {
		return JobHistory{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return JobHistory{}, errors.New("no result")
}
func QueryJobHistory(qu Query) ([]JobHistory, error) {
	columns := "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("job_histories", qu)
	if counter == 0 || counterr != nil {
		return []JobHistory{}, nil
	}
	query := buildquery(columns, "job_histories", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []JobHistory{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]JobHistory, 0, counter)
	for rows.Next() {
		item := JobHistory{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []JobHistory{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}
func GetIndexerFails(qu Query) (IndexerFail, error) {
	qu.Limit = 1
	results, err := QueryIndexerFails(qu)
	if err != nil {
		return IndexerFail{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return IndexerFail{}, errors.New("no result")
}
func QueryIndexerFails(qu Query) ([]IndexerFail, error) {
	columns := "id,created_at,updated_at,indexer,last_fail"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("indexer_fails", qu)
	if counter == 0 || counterr != nil {
		return []IndexerFail{}, nil
	}
	query := buildquery(columns, "indexer_fails", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []IndexerFail{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]IndexerFail, 0, counter)
	for rows.Next() {
		item := IndexerFail{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []IndexerFail{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetSerieFileUnmatched(qu Query) (SerieFileUnmatched, error) {
	qu.Limit = 1
	results, err := QuerySerieFileUnmatched(qu)
	if err != nil {
		return SerieFileUnmatched{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return SerieFileUnmatched{}, errors.New("no result")
}
func QuerySerieFileUnmatched(qu Query) ([]SerieFileUnmatched, error) {
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("serie_file_unmatcheds", qu)
	if counter == 0 || counterr != nil {
		return []SerieFileUnmatched{}, nil
	}
	query := buildquery(columns, "serie_file_unmatcheds", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []SerieFileUnmatched{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieFileUnmatched, 0, counter)
	for rows.Next() {
		item := SerieFileUnmatched{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []SerieFileUnmatched{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetMovieFileUnmatched(qu Query) (MovieFileUnmatched, error) {
	qu.Limit = 1
	results, err := QueryMovieFileUnmatched(qu)
	if err != nil {
		return MovieFileUnmatched{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return MovieFileUnmatched{}, errors.New("no result")
}
func QueryMovieFileUnmatched(qu Query) ([]MovieFileUnmatched, error) {
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("movie_file_unmatcheds", qu)
	if counter == 0 || counterr != nil {
		return []MovieFileUnmatched{}, nil
	}
	query := buildquery(columns, "movie_file_unmatcheds", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []MovieFileUnmatched{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]MovieFileUnmatched, 0, counter)
	for rows.Next() {
		item := MovieFileUnmatched{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []MovieFileUnmatched{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}
func GetResultMovies(qu Query) (ResultMovies, error) {
	qu.Limit = 1
	results, err := QueryResultMovies(qu)
	if err != nil {
		return ResultMovies{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ResultMovies{}, errors.New("no result")
}
func QueryResultMovies(qu Query) ([]ResultMovies, error) {
	columns := `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("movies", qu)
	if counter == 0 || counterr != nil {
		return []ResultMovies{}, nil
	}
	query := buildquery(columns, "movies", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ResultMovies{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ResultMovies, 0, counter)
	for rows.Next() {
		item := ResultMovies{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ResultMovies{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetResultSeries(qu Query) (ResultSeries, error) {
	qu.Limit = 1
	results, err := QueryResultSeries(qu)
	if err != nil {
		return ResultSeries{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ResultSeries{}, errors.New("no result")
}
func QueryResultSeries(qu Query) ([]ResultSeries, error) {
	columns := `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("series", qu)
	if counter == 0 || counterr != nil {
		return []ResultSeries{}, nil
	}
	query := buildquery(columns, "series", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ResultSeries{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ResultSeries, 0, counter)
	for rows.Next() {
		item := ResultSeries{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ResultSeries{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetResultSerieEpisodes(qu Query) (ResultSerieEpisodes, error) {
	qu.Limit = 1
	results, err := QueryResultSerieEpisodes(qu)
	if err != nil {
		return ResultSerieEpisodes{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ResultSerieEpisodes{}, errors.New("no result")
}
func QueryResultSerieEpisodes(qu Query) ([]ResultSerieEpisodes, error) {
	columns := `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := CountRows("serie_episodes", qu)
	if counter == 0 || counterr != nil {
		return []ResultSerieEpisodes{}, nil
	}
	query := buildquery(columns, "serie_episodes", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DB.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ResultSerieEpisodes{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ResultSerieEpisodes, 0, counter)
	for rows.Next() {
		item := ResultSerieEpisodes{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ResultSerieEpisodes{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetImdbGenre(qu Query) (ImdbGenres, error) {
	qu.Limit = 1
	results, err := QueryImdbGenre(qu)
	if err != nil {
		return ImdbGenres{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ImdbGenres{}, errors.New("no result")
}
func QueryImdbGenre(qu Query) ([]ImdbGenres, error) {
	columns := "id,created_at,updated_at,Tconst,Genre"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := ImdbCountRows("imdb_genres", qu)
	if counter == 0 || counterr != nil {
		return []ImdbGenres{}, nil
	}
	query := buildquery(columns, "imdb_genres", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DBImdb.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ImdbGenres{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbGenres, 0, counter)
	for rows.Next() {
		item := ImdbGenres{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ImdbGenres{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetImdbRating(qu Query) (ImdbRatings, error) {
	qu.Limit = 1
	results, err := QueryImdbRating(qu)
	if err != nil {
		return ImdbRatings{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ImdbRatings{}, errors.New("no result")
}
func QueryImdbRating(qu Query) ([]ImdbRatings, error) {
	columns := "id,created_at,updated_at,Tconst,num_votes,average_rating"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := ImdbCountRows("imdb_ratings", qu)
	if counter == 0 || counterr != nil {
		return []ImdbRatings{}, nil
	}
	query := buildquery(columns, "imdb_ratings", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DBImdb.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ImdbRatings{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbRatings, 0, counter)
	for rows.Next() {
		item := ImdbRatings{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ImdbRatings{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetImdbAka(qu Query) (ImdbAka, error) {
	qu.Limit = 1
	results, err := QueryImdbAka(qu)
	if err != nil {
		return ImdbAka{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ImdbAka{}, errors.New("no result")
}
func QueryImdbAka(qu Query) ([]ImdbAka, error) {
	columns := "id,created_at,updated_at,Tconst,ordering,title,slug,region,language,types,attributes,is_original_title"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter, counterr := ImdbCountRows("imdb_akas", qu)
	if counter == 0 || counterr != nil {
		return []ImdbAka{}, nil
	}
	query := buildquery(columns, "imdb_akas", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DBImdb.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ImdbAka{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbAka, 0, counter)
	for rows.Next() {
		item := ImdbAka{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ImdbAka{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func GetImdbTitle(qu Query) (ImdbTitle, error) {
	qu.Limit = 1
	results, err := QueryImdbTitle(qu)
	if err != nil {
		return ImdbTitle{}, err
	}
	if len(results) >= 1 {
		return results[0], nil
	}
	return ImdbTitle{}, errors.New("no result")
}
func QueryImdbTitle(qu Query) ([]ImdbTitle, error) {
	columns := "Tconst,title_type,primary_title,slug,original_title,is_adult,start_year,end_year,runtime_minutes,genres"
	if qu.Select != "" {
		columns = qu.Select
	}

	counter, counterr := ImdbCountRows("imdb_titles", qu)
	if counter == 0 || counterr != nil {
		return []ImdbTitle{}, nil
	}
	query := buildquery(columns, "imdb_titles", qu, false)
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	rows, err := DBImdb.Queryx(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return []ImdbTitle{}, err
	}

	defer rows.Close()
	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbTitle, 0, counter)
	for rows.Next() {
		item := ImdbTitle{}
		err2 := rows.StructScan(&item)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []ImdbTitle{}, err2
		}
		result = append(result, item)
	}
	return result, nil
}

func buildquery(columns string, table string, qu Query, count bool) string {
	var query strings.Builder
	query.WriteString("select ")

	if qu.InnerJoin != "" {
		if strings.Contains(columns, table+".") {
			query.WriteString(columns + " from " + table)
		} else {
			if count {
				query.WriteString("count(*) from " + table)
			} else {
				query.WriteString(table + ".* from " + table)
			}
		}
		query.WriteString(" inner join " + qu.InnerJoin)
	} else {
		query.WriteString(columns + " from " + table)
	}
	if qu.Where != "" {
		query.WriteString(" where " + qu.Where)
	}
	if qu.OrderBy != "" {
		query.WriteString(" order by " + qu.OrderBy)
	}
	if qu.Limit != 0 {
		if qu.Offset != 0 {
			query.WriteString(" limit " + strconv.Itoa(int(qu.Offset)) + ", " + strconv.Itoa(int(qu.Limit)))
		} else {
			query.WriteString(" limit " + strconv.Itoa(int(qu.Limit)))
		}
	}
	return query.String()
}

//Uses column id
func CountRows(table string, qu Query) (int, error) {
	// if qu.InnerJoin != "" {
	// 	query = buildquery("count("+table+".*)", table, qu)
	// } else {
	//}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	qu.Offset = 0
	qu.Limit = 0
	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", buildquery("count(*)", table, qu, true), " -args: ", qu.WhereArgs)
	}
	var counter int
	rows, err := DB.Query(buildquery("count(*)", table, qu, true), qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", buildquery("count(*)", table, qu, true), " error: ", err)
		return 0, err
	}
	defer rows.Close()
	rows.Next()
	rows.Scan(&counter)
	return counter, nil
}

func insertmapprepare(table string, insert map[string]interface{}) (string, []interface{}) {
	query := "INSERT INTO " + table + " ("
	i := 0
	columns := ""
	values := ""
	args := make([]interface{}, 0, len(insert))
	for idx, val := range insert {
		if i != 0 {
			columns += ","
			values += ","
		}
		i += 1
		columns += idx
		values += "?"
		args = append(args, val)
	}
	query += columns + ") VALUES (" + values + ")"
	return query, args
}
func InsertRowMap(table string, insert map[string]interface{}) (sql.Result, error) {
	query, args := insertmapprepare(table, insert)
	result, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.Error("Insert: ", table, " values: ", insert, " error: ", err)
	}
	return result, err
}

func insertarrayprepare(table string, columns []string) string {
	query := "INSERT INTO " + table + " ("
	cols := ""
	vals := ""
	for idx := range columns {
		if idx != 0 {
			cols += ","
			vals += ","
		}
		cols += columns[idx]
		vals += "?"
	}
	query += cols + ") VALUES (" + vals + ")"
	return query
}
func InsertArray(table string, columns []string, values []interface{}) (sql.Result, error) {
	query := insertarrayprepare(table, columns)
	result, err := dbexec("main", query, values)
	if err != nil {
		logger.Log.Error("Insert: ", table, " values: ", columns, values, " error: ", err)
	}
	return result, err
}

func updatemapprepare(table string, update map[string]interface{}, qu Query) (string, []interface{}) {
	query := "UPDATE " + table + " SET "
	i := 0
	args := make([]interface{}, 0, len(update))
	for idx, val := range update {
		if i != 0 {
			query += ","
		}
		i += 1
		query += idx + " = ?"
		args = append(args, val)
	}
	if qu.Where != "" {
		query += " where " + qu.Where
	}
	if len(qu.WhereArgs) >= 1 {
		args = append(args, qu.WhereArgs...)
	}
	return query, args
}
func UpdateRowMap(table string, update map[string]interface{}, qu Query) (sql.Result, error) {
	query, args := updatemapprepare(table, update, qu)
	result, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.Error("Update: ", table, " values: ", update, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}
func dbexec(dbtype string, query string, args []interface{}) (sql.Result, error) {
	var result sql.Result
	var err error
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", args)
	}
	ReadWriteMu.Lock()
	if dbtype == "imdb" {
		result, err = DBImdb.Exec(query, args...)
	} else {
		result, err = DB.Exec(query, args...)
	}
	ReadWriteMu.Unlock()
	if err != nil {
		logger.Log.Debug("error query. ", query, " arguments. ", args)

	}
	return result, err
}
func updatearrayprepare(table string, columns []string, values []interface{}, qu Query) (string, []interface{}) {
	query := "UPDATE " + table + " SET "
	for idx := range columns {
		if idx != 0 {
			query += ","
		}
		query += columns[idx] + " = ?"
	}
	if qu.Where != "" {
		query += " where " + qu.Where
	}
	if len(qu.WhereArgs) >= 1 {
		values = append(values, qu.WhereArgs...)
	}
	return query, values
}
func UpdateArray(table string, columns []string, values []interface{}, qu Query) (sql.Result, error) {
	query, args := updatearrayprepare(table, columns, values, qu)
	result, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.Error("Update: ", table, " values: ", columns, values, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

func updatecolprepare(table string, column string, value interface{}, qu Query) (string, []interface{}) {
	query := "UPDATE " + table + " SET " + column + " = ?"
	if qu.Where != "" {
		query += " where " + qu.Where
	}
	args := make([]interface{}, 0, len(qu.WhereArgs)+1)
	args = append(args, value)
	if len(qu.WhereArgs) >= 1 {
		args = append(args, qu.WhereArgs...)
	}
	return query, args
}
func UpdateColumn(table string, column string, value interface{}, qu Query) (sql.Result, error) {
	query, args := updatecolprepare(table, column, value, qu)
	result, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.Error("Update: ", table, " values: ", column, value, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

func DeleteRow(table string, qu Query) (sql.Result, error) {
	query := "DELETE FROM " + table
	if qu.Where != "" {
		query += " where " + qu.Where
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	ReadWriteMu.Lock()
	result, err := DB.Exec(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Delete: ", table, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	ReadWriteMu.Unlock()
	return result, err
}

func Upsert(table string, update map[string]interface{}, qu Query) (sql.Result, error) {
	var counter int
	counter, _ = CountRows(table, qu)
	if counter == 0 {
		result, err := InsertRowMap(table, update)
		if err != nil {
			logger.Log.Error("Upsert-insert: ", table, " values: ", update, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
		}
		return result, err
	}
	result, err := UpdateRowMap(table, update, qu)
	if err != nil {
		logger.Log.Error("Upsert-update: ", table, " values: ", update, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

//Uses column id
func ImdbCountRows(table string, qu Query) (int, error) {
	// if qu.InnerJoin != "" {
	// 	query = buildquery("count("+table+".*)", table, qu)
	// } else {
	query := buildquery("count(*)", table, qu, true)
	//}

	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
	qu.Limit = 0
	qu.Offset = 0
	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	var counter int
	rows, err := DBImdb.Query(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Query: ", query, " error: ", err)
		return 0, err
	}
	defer rows.Close()
	rows.Next()
	rows.Scan(&counter)
	return counter, nil
}

func ImdbInsertRowMap(table string, insert map[string]interface{}) (sql.Result, error) {
	query, args := insertmapprepare(table, insert)
	result, err := dbexec("imdb", query, args)
	if err != nil {
		logger.Log.Error("Insert: ", table, " values: ", insert, " error: ", err)
	}
	return result, err
}
func ImdbInsertArray(table string, columns []string, values []interface{}) (sql.Result, error) {
	query := insertarrayprepare(table, columns)
	result, err := dbexec("imdb", query, values)
	if err != nil {
		logger.Log.Error("Insert: ", table, " values: ", columns, values, " error: ", err)
	}
	return result, err
}

func ImdbUpdateRowMap(table string, update map[string]interface{}, qu Query) (sql.Result, error) {
	query, args := updatemapprepare(table, update, qu)
	result, err := dbexec("imdb", query, args)
	if err != nil {
		logger.Log.Error("Update: ", table, " values: ", update, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}
func ImdbUpdateArray(table string, columns []string, values []interface{}, qu Query) (sql.Result, error) {
	query, args := updatearrayprepare(table, columns, values, qu)
	result, err := dbexec("imdb", query, args)
	if err != nil {
		logger.Log.Error("Update: ", table, " values: ", columns, values, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

func ImdbUpdateColumn(table string, column string, value interface{}, qu Query) (sql.Result, error) {
	query, args := updatecolprepare(table, column, value, qu)
	result, err := dbexec("imdb", query, args)
	if err != nil {
		logger.Log.Error("Update: ", table, " values: ", column, value, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

func ImdbDeleteRow(table string, qu Query) (sql.Result, error) {
	query := "DELETE FROM " + table
	if qu.Where != "" {
		query += " where " + qu.Where
	}
	ReadWriteMu.Lock()
	result, err := DBImdb.Exec(query, qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Delete: ", table, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	ReadWriteMu.Unlock()
	return result, err
}

func ImdbUpsert(table string, update map[string]interface{}, qu Query) (sql.Result, error) {
	var counter int
	counter, _ = ImdbCountRows(table, qu)
	if counter == 0 {
		result, err := ImdbInsertRowMap(table, update)
		if err != nil {
			logger.Log.Error("Upsert-insert: ", table, " values: ", update, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
		}
		return result, err
	}
	result, err := ImdbUpdateRowMap(table, update, qu)
	if err != nil {
		logger.Log.Error("Upsert-update: ", table, " values: ", update, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

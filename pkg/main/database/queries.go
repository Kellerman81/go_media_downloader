package database

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/jmoiron/sqlx"
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

type tquerycache struct {
	Values []querycacheEntry
}
type querycacheEntry struct {
	Query     string
	Statement sqlx.Stmt
}

var Querycache tquerycache

func (s *tquerycache) Add(query string, imdb bool) {
	if imdb {
		tt, err := DBImdb.Preparex(query)
		if err == nil {
			s.Values = append(s.Values, querycacheEntry{Query: query, Statement: *tt})
		} else {
			fmt.Println("error generating query: "+query+" error: ", err)
		}
	} else {
		tt, err := DB.Preparex(query)
		if err == nil {
			s.Values = append(s.Values, querycacheEntry{Query: query, Statement: *tt})
		} else {
			fmt.Println("error generating query: "+query+" error: ", err)
		}
	}
	return
}
func (s *tquerycache) Contains(query string) bool {
	for idx := range s.Values {
		if strings.EqualFold(s.Values[idx].Query, query) {
			return true
		}
	}
	return false
}
func (s *tquerycache) Get(query string) *sqlx.Stmt {
	for idx := range s.Values {
		if strings.EqualFold(s.Values[idx].Query, query) {
			return &s.Values[idx].Statement
		}
	}
	return nil
}
func getstatement(query string, imdb bool) *sqlx.Stmt {
	if !Querycache.Contains(query) {
		Querycache.Add(query, imdb)
	}
	return Querycache.Get(query)
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

func queryStructScan(table string, columns string, qu Query, counter int, targetobj interface{}) (bool, error) {
	query := buildquery(columns, table, qu, false)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}

	arr := reflect.ValueOf(targetobj).Elem()
	v := reflect.New(reflect.TypeOf(targetobj).Elem().Elem())
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if counter == 1 {
		err := getstatement(query, false).QueryRowx(qu.WhereArgs...).StructScan(v.Interface())
		if err != nil {
			logger.Log.Error("Query2: ", query, " error: ", err)
			return true, err
		}
		arr.Set(reflect.Append(arr, v.Elem()))
	} else {
		rows, err := getstatement(query, false).Queryx(qu.WhereArgs...)
		if err != nil {
			logger.Log.Error("Query: ", query, " error: ", err)
			return true, err
		}

		defer rows.Close()

		for rows.Next() {
			err = rows.StructScan(v.Interface())
			if err != nil {
				logger.Log.Error("Query2: ", query, " error: ", err)
				return true, err
			}
			arr.Set(reflect.Append(arr, v.Elem()))
		}
	}

	return false, nil
}
func queryIMDBStructScan(table string, columns string, qu Query, counter int, targetobj interface{}) (bool, error) {
	query := buildquery(columns, table, qu, false)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	arr := reflect.ValueOf(targetobj).Elem()
	v := reflect.New(reflect.TypeOf(targetobj).Elem().Elem())

	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if counter == 1 {
		err := getstatement(query, true).QueryRowx(qu.WhereArgs...).StructScan(v.Interface())
		if err != nil {
			logger.Log.Error("Query2: ", query, " error: ", err)
			return true, err
		}
		arr.Set(reflect.Append(arr, v.Elem()))
	} else {
		rows, err := getstatement(query, true).Queryx(qu.WhereArgs...)
		if err != nil {
			logger.Log.Error("Query: ", query, " error: ", err)
			return true, err
		}

		defer rows.Close()

		for rows.Next() {
			err = rows.StructScan(v.Interface())
			if err != nil {
				logger.Log.Error("Query2: ", query, " error: ", err)
				return true, err
			}
			arr.Set(reflect.Append(arr, v.Elem()))
		}
	}
	return false, nil
}

func QueryDbmovie(qu Query) ([]Dbmovie, error) {
	table := "dbmovies"
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows(table, qu)
		if counter == 0 || counterr != nil {
			return []Dbmovie{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Dbmovie, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []Dbmovie{}, err
	}
	return result, nil
}

func QueryDbmovieJson(qu Query) ([]DbmovieJson, error) {
	table := "dbmovies"
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows(table, qu)
		if counter == 0 || counterr != nil {
			return []DbmovieJson{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbmovieJson, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []DbmovieJson{}, err
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
	table := "dbmovie_titles"
	columns := "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows(table, qu)
		if counter == 0 || counterr != nil {
			return []DbmovieTitle{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbmovieTitle, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []DbmovieTitle{}, err
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
	table := "dbseries"
	columns := "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("dbseries", qu)
		if counter == 0 || counterr != nil {
			return []Dbserie{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Dbserie, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []Dbserie{}, err
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
	table := "dbserie_episodes"
	columns := "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("dbserie_episodes", qu)
		if counter == 0 || counterr != nil {
			return []DbserieEpisode{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbserieEpisode, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []DbserieEpisode{}, err
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
	table := "dbserie_alternates"
	columns := "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("dbserie_alternates", qu)
		if counter == 0 || counterr != nil {
			return []DbserieAlternate{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]DbserieAlternate, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []DbserieAlternate{}, err
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
	table := "series"
	columns := "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("series", qu)
		if counter == 0 || counterr != nil {
			return []Serie{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Serie, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []Serie{}, err
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
	table := "serie_episodes"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("serie_episodes", qu)
		if counter == 0 || counterr != nil {
			return []SerieEpisode{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieEpisode, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []SerieEpisode{}, err
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
	table := "serie_episode_histories"
	columns := "id,created_at,updated_at,title,url,indexer,type,target,downloaded_at,blacklisted,quality_profile,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("serie_episode_histories", qu)
		if counter == 0 || counterr != nil {
			return []SerieEpisodeHistory{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieEpisodeHistory, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []SerieEpisodeHistory{}, err
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
	table := "serie_episode_files"
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("serie_episode_files", qu)
		if counter == 0 || counterr != nil {
			return []SerieEpisodeFile{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieEpisodeFile, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []SerieEpisodeFile{}, err
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
	table := "movies"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("movies", qu)
		if counter == 0 || counterr != nil {
			if counterr != nil {
				return []Movie{}, counterr
			} else {
				return []Movie{}, nil
			}
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Movie, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []Movie{}, err
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
	table := "movie_files"
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("movie_files", qu)
		if counter == 0 || counterr != nil {
			return []MovieFile{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]MovieFile, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []MovieFile{}, err
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
	table := "movie_histories"
	columns := "id,created_at,updated_at,title,url,indexer,type,target,downloaded_at,blacklisted,quality_profile,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("movie_histories", qu)
		if counter == 0 || counterr != nil {
			return []MovieHistory{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]MovieHistory, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []MovieHistory{}, err
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
	table := "r_sshistories"
	columns := "id,created_at,updated_at,config,list,indexer,last_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("r_sshistories", qu)
		if counter == 0 || counterr != nil {
			return []RSSHistory{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]RSSHistory, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []RSSHistory{}, err
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
	table := "qualities"
	columns := "id,created_at,updated_at,type,name,regex,strings,priority,use_regex"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("qualities", qu)
		if counter == 0 || counterr != nil {
			return []Qualities{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Qualities, 0, counter)
	//defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []Qualities{}, err
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
	table := "job_histories"
	columns := "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("job_histories", qu)
		if counter == 0 || counterr != nil {
			return []JobHistory{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]JobHistory, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []JobHistory{}, err
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
	table := "indexer_fails"
	columns := "id,created_at,updated_at,indexer,last_fail"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("indexer_fails", qu)
		if counter == 0 || counterr != nil {
			return []IndexerFail{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]IndexerFail, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []IndexerFail{}, err
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
	table := "serie_file_unmatcheds"
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("serie_file_unmatcheds", qu)
		if counter == 0 || counterr != nil {
			return []SerieFileUnmatched{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]SerieFileUnmatched, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []SerieFileUnmatched{}, err
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
	table := "movie_file_unmatcheds"
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("movie_file_unmatcheds", qu)
		if counter == 0 || counterr != nil {
			return []MovieFileUnmatched{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]MovieFileUnmatched, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []MovieFileUnmatched{}, err
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
	table := "movies"
	columns := `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("movies", qu)
		if counter == 0 || counterr != nil {
			return []ResultMovies{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ResultMovies, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ResultMovies{}, err
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
	table := "series"
	columns := `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("series", qu)
		if counter == 0 || counterr != nil {
			return []ResultSeries{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ResultSeries, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ResultSeries{}, err
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
	table := "serie_episodes"
	columns := `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows("serie_episodes", qu)
		if counter == 0 || counterr != nil {
			return []ResultSerieEpisodes{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ResultSerieEpisodes, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ResultSerieEpisodes{}, err
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
	table := "imdb_genres"
	columns := "id,created_at,updated_at,Tconst,Genre"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = ImdbCountRows("imdb_genres", qu)
		if counter == 0 || counterr != nil {
			return []ImdbGenres{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbGenres, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryIMDBStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ImdbGenres{}, err
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
	table := "imdb_ratings"
	columns := "id,created_at,updated_at,Tconst,num_votes,average_rating"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = ImdbCountRows("imdb_ratings", qu)
		if counter == 0 || counterr != nil {
			return []ImdbRatings{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbRatings, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryIMDBStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ImdbRatings{}, err
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
	table := "imdb_akas"
	columns := "id,created_at,updated_at,Tconst,ordering,title,slug,region,language,types,attributes,is_original_title"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = ImdbCountRows("imdb_akas", qu)
		if counter == 0 || counterr != nil {
			return []ImdbAka{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbAka, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryIMDBStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ImdbAka{}, err
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
	table := "imdb_titles"
	columns := "Tconst,title_type,primary_title,slug,original_title,is_adult,start_year,end_year,runtime_minutes,genres"
	if qu.Select != "" {
		columns = qu.Select
	}

	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = ImdbCountRows("imdb_titles", qu)
		if counter == 0 || counterr != nil {
			return []ImdbTitle{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]ImdbTitle, 0, counter)
	defer logger.ClearVar(&result)
	failed, err := queryIMDBStructScan(table, columns, qu, counter, &result)
	if failed {
		return []ImdbTitle{}, err
	}
	return result, nil
}

func buildquery(columns string, table string, qu Query, count bool) string {
	var query bytes.Buffer
	defer query.Reset()
	query.WriteString("select ")

	if qu.InnerJoin != "" {
		if strings.Contains(strings.ToLower(columns), strings.ToLower(table)+".") {
			query.WriteString(columns)
		} else {
			if count {
				query.WriteString("count(*)")
			} else {
				query.WriteString(table)
				query.WriteString(".*")
			}
		}
		query.WriteString(" from ")
		query.WriteString(table)
		query.WriteString(" inner join ")
		query.WriteString(qu.InnerJoin)
	} else {
		query.WriteString(columns)
		query.WriteString(" from ")
		query.WriteString(table)
	}
	if qu.Where != "" {
		query.WriteString(" where ")
		query.WriteString(qu.Where)
	}
	if qu.OrderBy != "" {
		query.WriteString(" order by ")
		query.WriteString(qu.OrderBy)
	}
	if qu.Limit != 0 {
		if qu.Offset != 0 {
			query.WriteString(" limit ")
			query.WriteString(strconv.Itoa(int(qu.Offset)))
			query.WriteString(", ")
			query.WriteString(strconv.Itoa(int(qu.Limit)))
		} else {
			query.WriteString(" limit ")
			query.WriteString(strconv.Itoa(int(qu.Limit)))
		}
	}
	return query.String()
}

func QueryStructStatic(query string, getstruct interface{}, args ...interface{}) error {
	defer logger.ClearVar(&args)
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, false).QueryRowx(args...).StructScan(&getstruct)
	if err != nil {
		return err
	}
	return nil
}

//requires 2 columns - string and uint (location and id)
func QueryDbfiles(query string, querycount string, args ...interface{}) ([]Dbfiles, error) {
	defer logger.ClearVar(&args)
	rowcount, _ := CountRowsStatic(querycount, args...)

	returnarray := make([]Dbfiles, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	var location string
	var id uint

	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 {
		rows := getstatement(query, false).QueryRow(args...).Scan(&location, &id)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbfiles{}, nil
		}
		returnarray = append(returnarray, Dbfiles{Location: location, ID: id})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbfiles{}, err
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&location, &id)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbfiles{}, err2
			}
			returnarray = append(returnarray, Dbfiles{Location: location, ID: id})
		}
	}

	return returnarray, nil
}

func getStructType(dest interface{}) reflect.Type {
	return reflect.TypeOf(dest).Elem().Elem()
}

type Dbstatic_OneInt struct {
	Num int
}

//requires 1 column - int
func QueryStaticColumnsOneInt(query string, querycount string, args ...interface{}) ([]Dbstatic_OneInt, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}

	var num int
	returnarray := make([]Dbstatic_OneInt, 0, rowcount)
	defer logger.ClearVar(&returnarray)

	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, false).QueryRow(args...).Scan(&num)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_OneInt{}, nil
		}
		returnarray = append(returnarray, Dbstatic_OneInt{Num: num})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbstatic_OneInt{}, err
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&num)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_OneInt{}, err2
			}
			returnarray = append(returnarray, Dbstatic_OneInt{Num: num})
		}
	}
	return returnarray, nil
}

//Select has to be in it
func QueryStaticColumnsOneIntQueryObject(table string, qu Query) ([]Dbstatic_OneInt, error) {
	counter := int(qu.Limit)
	var counterr error
	if qu.Limit == 0 {
		counter, counterr = CountRows(table, qu)
		if counter == 0 || counterr != nil {
			return []Dbstatic_OneInt{}, nil
		}
	}

	if qu.Limit >= 1 && qu.Limit < uint64(counter) {
		counter = int(qu.Limit)
	}
	result := make([]Dbstatic_OneInt, 0, counter)
	defer logger.ClearVar(&result)

	var num int
	query := buildquery(qu.Select, table, qu, false)
	rows, err := getstatement(query, false).Query(qu.WhereArgs...)
	if err != nil {
		return []Dbstatic_OneInt{}, err
	}

	defer rows.Close()
	var err2 error
	for rows.Next() {
		err2 = rows.Scan(&num)
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []Dbstatic_OneInt{}, err2
		}
		result = append(result, Dbstatic_OneInt{Num: num})
	}
	return result, nil
}

type Dbstatic_OneString struct {
	Str string
}

//requires 1 column - string
func QueryStaticColumnsOneString(query string, querycount string, args ...interface{}) ([]Dbstatic_OneString, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	var singlerow bool
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
		if rowcount == 1 {
			singlerow = true
		}
	} else {
		rowcount = 1
	}

	returnarray := make([]Dbstatic_OneString, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	var str string
	var err2 error
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if singlerow {
		rows := getstatement(query, false).QueryRow(args...).Scan(&str)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_OneString{}, nil
		}
		returnarray = append(returnarray, Dbstatic_OneString{Str: str})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbstatic_OneString{}, err
		}

		defer rows.Close()
		for rows.Next() {
			err2 = rows.Scan(&str)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_OneString{}, err2
			}
			returnarray = append(returnarray, Dbstatic_OneString{Str: str})
		}
	}
	return returnarray, nil
}

//requires 1 column - string
func QueryStaticStringArray(query string, querycount string, args ...interface{}) []string {
	defer logger.ClearVar(&args)
	var rowcount int
	var singlerow bool
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
		if rowcount == 1 {
			singlerow = true
		}
	} else {
		rowcount = 1
	}

	returnarray := make([]string, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	var str string
	var err2 error
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if singlerow {
		rows := getstatement(query, false).QueryRow(args...).Scan(&str)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []string{}
		}
		returnarray = append(returnarray, str)
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []string{}
		}

		defer rows.Close()
		for rows.Next() {
			err2 = rows.Scan(&str)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []string{}
			}
			returnarray = append(returnarray, str)
		}
	}
	return returnarray
}

//requires 1 column - string
func QueryStaticColumnsOneStringNoError(query string, querycount string, args ...interface{}) []Dbstatic_OneString {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}
	var str string
	returnarray := make([]Dbstatic_OneString, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, false).QueryRow(args...).Scan(&str)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_OneString{}
		}
		returnarray = append(returnarray, Dbstatic_OneString{Str: str})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbstatic_OneString{}
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&str)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_OneString{}
			}
			returnarray = append(returnarray, Dbstatic_OneString{Str: str})
		}
	}
	return returnarray
}

//requires 1 column - string
func QueryImdbStaticColumnsOneString(query string, querycount string, args ...interface{}) ([]Dbstatic_OneString, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = ImdbCountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}
	var str string
	returnarray := make([]Dbstatic_OneString, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, true).QueryRow(args...).Scan(&str)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_OneString{}, nil
		}
		returnarray = append(returnarray, Dbstatic_OneString{Str: str})
	} else {
		rows, err := getstatement(query, true).Query(args...)
		if err != nil {
			return []Dbstatic_OneString{}, err
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&str)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_OneString{}, err2
			}
			returnarray = append(returnarray, Dbstatic_OneString{Str: str})
		}
	}
	return returnarray, nil
}

type Dbstatic_TwoString struct {
	Str1 string
	Str2 string
}

//requires 2 columns - string
func QueryStaticColumnsTwoString(query string, querycount string, args ...interface{}) ([]Dbstatic_TwoString, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}
	var str1, str2 string
	returnarray := make([]Dbstatic_TwoString, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, false).QueryRow(args...).Scan(&str1, &str2)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_TwoString{}, nil
		}
		returnarray = append(returnarray, Dbstatic_TwoString{Str1: str1, Str2: str2})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbstatic_TwoString{}, err
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&str1, &str2)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_TwoString{}, err2
			}
			returnarray = append(returnarray, Dbstatic_TwoString{Str1: str1, Str2: str2})
		}
	}
	return returnarray, nil
}

type Dbstatic_OneStringOneInt struct {
	Str string
	Num int
}

//requires 2 columns- string and int
func QueryStaticColumnsOneStringOneInt(query string, querycount string, args ...interface{}) ([]Dbstatic_OneStringOneInt, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}

	returnarray := make([]Dbstatic_OneStringOneInt, 0, rowcount)
	defer logger.ClearVar(&returnarray)

	var str string
	var num int
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, false).QueryRow(args...).Scan(&str, &num)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_OneStringOneInt{}, nil
		}
		returnarray = append(returnarray, Dbstatic_OneStringOneInt{Str: str, Num: num})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbstatic_OneStringOneInt{}, err
		}
		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&str, &num)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_OneStringOneInt{}, err2
			}
			returnarray = append(returnarray, Dbstatic_OneStringOneInt{Str: str, Num: num})
		}
	}
	return returnarray, nil
}

//requires 2 columns- string and int
func QueryImdbStaticColumnsOneStringOneInt(query string, querycount string, args ...interface{}) ([]Dbstatic_OneStringOneInt, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = ImdbCountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}
	var str string
	var num int
	returnarray := make([]Dbstatic_OneStringOneInt, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, true).QueryRow(args...).Scan(&str, &num)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_OneStringOneInt{}, nil
		}
		returnarray = append(returnarray, Dbstatic_OneStringOneInt{Str: str, Num: num})
	} else {
		rows, err := getstatement(query, true).Query(args...)
		if err != nil {
			return []Dbstatic_OneStringOneInt{}, err
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&str, &num)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_OneStringOneInt{}, err2
			}
			returnarray = append(returnarray, Dbstatic_OneStringOneInt{Str: str, Num: num})
		}
	}
	return returnarray, nil
}

type Dbstatic_TwoInt struct {
	Num1 int
	Num2 int
}

//requires 2 columns- int and int
func QueryStaticColumnsTwoInt(query string, querycount string, args ...interface{}) ([]Dbstatic_TwoInt, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}
	var num1, num2 int
	returnarray := make([]Dbstatic_TwoInt, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	if rowcount == 1 && len(querycount) >= 1 {
		rows := getstatement(query, false).QueryRow(args...).Scan(&num1, &num2)
		if rows != nil {
			logger.Log.Error("Query2: ", query, " error")
			return []Dbstatic_TwoInt{}, nil
		}
		returnarray = append(returnarray, Dbstatic_TwoInt{Num1: num1, Num2: num2})
	} else {
		rows, err := getstatement(query, false).Query(args...)
		if err != nil {
			return []Dbstatic_TwoInt{}, err
		}

		defer rows.Close()
		var err2 error
		for rows.Next() {
			err2 = rows.Scan(&num1, &num2)
			if err2 != nil {
				logger.Log.Error("Query2: ", query, " error: ", err2)
				return []Dbstatic_TwoInt{}, err2
			}
			returnarray = append(returnarray, Dbstatic_TwoInt{Num1: num1, Num2: num2})
		}
	}
	return returnarray, nil
}
func QueryStructStaticArray(query string, querycount string, getstruct interface{}, args ...interface{}) ([]interface{}, error) {
	defer logger.ClearVar(&args)
	var rowcount int
	if len(querycount) >= 1 {
		rowcount, _ = CountRowsStatic(querycount, args...)
	} else {
		rowcount = 1
	}
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	rows, err := getstatement(query, false).Queryx(args...)
	if err != nil {
		return []interface{}{}, err
	}
	returnarray := make([]interface{}, 0, rowcount)
	defer logger.ClearVar(&returnarray)
	defer rows.Close()
	v := reflect.New(reflect.TypeOf(getstruct).Elem())
	var err2 error
	for rows.Next() {
		err2 = rows.StructScan(v.Interface())
		if err2 != nil {
			logger.Log.Error("Query2: ", query, " error: ", err2)
			return []interface{}{}, err2
		}
		returnarray = append(returnarray, v.Elem().Interface())
	}
	return returnarray, nil
}

func getrowcount(query string, args ...interface{}) (int, error) {
	defer logger.ClearVar(&args)
	var counter int
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, false).QueryRow(args...).Scan(&counter)
	if err != nil {
		return 0, errors.New("no row")
	}
	return counter, nil
}
func getrowcountimdb(query string, args ...interface{}) (int, error) {
	defer logger.ClearVar(&args)
	var counter int
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, true).QueryRow(args...).Scan(&counter)

	if err != nil {
		return 0, errors.New("no row")
	}
	return counter, nil
}

//Uses column id
func CountRows(table string, qu Query) (int, error) {
	qu.Offset = 0
	qu.Limit = 0
	return getrowcount(buildquery("count(*)", table, qu, true), qu.WhereArgs...)
}

func CountRowsStatic(query string, args ...interface{}) (int, error) {
	defer logger.ClearVar(&args)
	return getrowcount(query, args...)
}

func CountRowsStaticNoError(query string, args ...interface{}) int {
	defer logger.ClearVar(&args)
	count, _ := getrowcount(query, args...)
	return count
}

func QueryColumnStatic(query string, args ...interface{}) (interface{}, error) {
	defer logger.ClearVar(&args)
	var ret interface{}
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, false).QueryRow(args...).Scan(&ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func QueryColumnString(query string, args ...interface{}) (string, error) {
	defer logger.ClearVar(&args)
	var ret string
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, false).QueryRow(args...).Scan(&ret)
	if err != nil {
		return "", err
	}
	return ret, nil
}

func QueryColumnUint(query string, args ...interface{}) (uint, error) {
	defer logger.ClearVar(&args)
	var ret uint
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, false).QueryRow(args...).Scan(&ret)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

func QueryImdbColumnString(query string, args ...interface{}) (string, error) {
	defer logger.ClearVar(&args)
	var ret string
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, true).QueryRow(args...).Scan(&ret)
	if err != nil {
		return "", err
	}
	return ret, nil
}

func QueryImdbColumnUint(query string, args ...interface{}) (uint, error) {
	defer logger.ClearVar(&args)
	var ret uint
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, true).QueryRow(args...).Scan(&ret)
	if err != nil {
		return 0, err
	}
	return ret, nil
}

func QueryColumnBool(query string, args ...interface{}) (bool, error) {
	defer logger.ClearVar(&args)
	var ret bool
	ReadWriteMu.RLock()
	defer ReadWriteMu.RUnlock()
	err := getstatement(query, false).QueryRow(args...).Scan(&ret)
	if err != nil {
		return false, err
	}
	return ret, nil
}

func insertarrayprepare(table string, columns []string) string {
	var query bytes.Buffer
	defer query.Reset()
	query.WriteString("INSERT INTO ")
	query.WriteString(table)
	query.WriteString(" (")
	var cols bytes.Buffer
	var vals bytes.Buffer
	for idx := range columns {
		if idx != 0 {
			cols.WriteString(",")
			vals.WriteString(",")
		}
		cols.WriteString(columns[idx])
		vals.WriteString("?")
	}
	query.WriteString(cols.String())
	query.WriteString(") VALUES (")
	query.WriteString(vals.String())
	query.WriteString(")")
	cols.Reset()
	vals.Reset()
	return query.String()
}
func InsertArray(table string, columns []string, values []interface{}) (sql.Result, error) {
	query := insertarrayprepare(table, columns)
	result, err := dbexec("main", query, values)
	if err != nil {
		logger.Log.Error("Insert: ", table, " values: ", columns, values, " error: ", err)
	}
	return result, err
}

func dbexec(dbtype string, query string, args []interface{}) (sql.Result, error) {
	var result sql.Result
	var err error
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", args)
	}
	imdb := false
	if dbtype == "imdb" {
		imdb = true
	}
	ReadWriteMu.Lock()
	defer ReadWriteMu.Unlock()
	result, err = getstatement(query, imdb).Exec(args...)
	if err != nil {
		logger.Log.Debug("error query. ", query, " arguments. ", args)

	}
	return result, err
}
func updatearrayprepare(table string, columns []string, values []interface{}, qu Query) (string, []interface{}) {
	var query bytes.Buffer
	defer query.Reset()
	query.WriteString("UPDATE ")
	query.WriteString(table)
	query.WriteString(" SET ")
	for idx := range columns {
		if idx != 0 {
			query.WriteString(",")
		}
		query.WriteString(columns[idx])
		query.WriteString(" = ?")
	}
	if qu.Where != "" {
		query.WriteString(" where ")
		query.WriteString(qu.Where)

		if len(qu.WhereArgs) >= 1 {
			for idx := range qu.WhereArgs {
				values = append(values, qu.WhereArgs[idx])
			}
		}
	}
	return query.String(), values
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
	var query bytes.Buffer
	defer query.Reset()
	query.WriteString("UPDATE ")
	query.WriteString(table)
	query.WriteString(" SET ")
	query.WriteString(column)
	query.WriteString(" = ?")
	if qu.Where != "" {
		query.WriteString(" where ")
		query.WriteString(qu.Where)
	}
	args := make([]interface{}, 0, len(qu.WhereArgs)+1)
	args = append(args, value)
	if len(qu.WhereArgs) >= 1 {
		args = append(args, qu.WhereArgs...)
	}
	return query.String(), args
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
	var query bytes.Buffer
	defer query.Reset()
	query.WriteString("DELETE FROM ")
	query.WriteString(table)
	if qu.Where != "" {
		query.WriteString(" where ")
		query.WriteString(qu.Where)
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.EqualFold(cfg_general.DBLogLevel, "debug") {
		logger.Log.Debug("query count: ", query, " -args: ", qu.WhereArgs)
	}
	ReadWriteMu.Lock()
	defer ReadWriteMu.Unlock()
	result, err := getstatement(query.String(), false).Exec(qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Delete: ", table, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	query.Reset()
	return result, err
}

func UpsertArray(table string, columns []string, values []interface{}, qu Query) (sql.Result, error) {
	var counter int
	counter, _ = CountRows(table, qu)
	if counter == 0 {
		result, err := InsertArray(table, columns, values)
		if err != nil {
			logger.Log.Error("Upsert-insert: ", table, " values: ", columns, values, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
		}
		return result, err
	}
	result, err := UpdateArray(table, columns, values, qu)
	if err != nil {
		logger.Log.Error("Upsert-update: ", table, " values: ", columns, values, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

//Uses column id
func ImdbCountRows(table string, qu Query) (int, error) {
	qu.Offset = 0
	qu.Limit = 0
	return getrowcountimdb(buildquery("count(*)", table, qu, true), qu.WhereArgs...)
}

func ImdbCountRowsStatic(query string, args ...interface{}) (int, error) {
	return getrowcountimdb(query, args...)
}

func ImdbInsertArray(table string, columns []string, values []interface{}) (sql.Result, error) {
	query := insertarrayprepare(table, columns)
	result, err := dbexec("imdb", query, values)
	if err != nil {
		logger.Log.Error("Insert: ", table, " values: ", columns, values, " error: ", err)
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
	defer ReadWriteMu.Unlock()
	result, err := getstatement(query, true).Exec(qu.WhereArgs...)
	if err != nil {
		logger.Log.Error("Delete: ", table, " where: ", qu.Where, " whereargs: ", qu.WhereArgs, " error: ", err)
	}
	return result, err
}

func DbQuickCheck() string {
	ReadWriteMu.Lock()
	defer ReadWriteMu.Unlock()
	rows, _ := DB.Query("PRAGMA quick_check;")
	defer rows.Close()
	rows.Next()
	var str string
	rows.Scan(&str)
	return str
}

func DbIntegrityCheck() string {
	ReadWriteMu.Lock()
	defer ReadWriteMu.Unlock()
	rows, _ := DB.Query("PRAGMA integrity_check;")
	defer rows.Close()
	rows.Next()
	var str string
	rows.Scan(&str)
	return str
}

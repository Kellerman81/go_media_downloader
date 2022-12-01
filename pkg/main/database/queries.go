package database

import (
	"database/sql"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

type Query struct {
	Select    string
	Where     string
	OrderBy   string
	Limit     uint64
	Offset    uint64
	InnerJoin string
}

var QueryFilterByID Query = Query{Where: logger.FilterByID}

const strWhere string = " where "
const strMustBePointer string = " must be a pointer: ErrNotAPointer"

func getstatement(key string, imdb bool) *sqlx.Stmt {
	if logger.GlobalStmtCache.Check(key) {
		return logger.GlobalStmtCache.GetData(key)
	}
	val, err := getdb(imdb).Preparex(key)
	if err == nil {
		logger.GlobalStmtCache.SetStmt(key, val, time.Minute*30)
		return val
	} else {
		logger.Log.GlobalLogger.Error("error generating query: "+key, zap.Error(err))
	}
	return nil
}

func getdb(imdb bool) *sqlx.DB {
	if imdb {
		return DBImdb
	}
	return DB
}
func getnamedstatement(key string, imdb bool) *sqlx.NamedStmt {
	if logger.GlobalStmtNamedCache.Check(key) {
		return logger.GlobalStmtNamedCache.GetData(key)
	}
	val, err := getdb(imdb).PrepareNamed(key)
	if err == nil {
		logger.GlobalStmtNamedCache.SetNamedStmt(key, val, time.Minute*30)
		return val
	} else {
		logger.Log.GlobalLogger.Error("error generating query: "+key, zap.Error(err))
	}
	return nil
}

func GetDbmovie(qu Querywithargs) (Dbmovie, error) {
	defer qu.close()
	table := "dbmovies"
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result Dbmovie
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

func getrows(imdb bool, named bool, qu *Querywithargs) (*sqlx.Rows, error) {
	if named {
		return getnamedstatement(qu.QueryString, imdb).Queryx(qu.Args[0])
	} else {
		return getstatement(qu.QueryString, imdb).Queryx(qu.Args...)
	}
}
func getrowssql(imdb bool, named bool, qu *Querywithargs) (*sql.Rows, error) {
	if named {
		return getnamedstatement(qu.QueryString, imdb).Query(qu.Args[0])
	} else {
		return getstatement(qu.QueryString, imdb).Query(qu.Args...)
	}
}

func getrow(imdb bool, named bool, qu *Querywithargs) *sqlx.Row {
	if named {
		return getnamedstatement(qu.QueryString, imdb).QueryRowx(qu.Args[0])
	} else {
		return getstatement(qu.QueryString, imdb).QueryRowx(qu.Args...)
	}
}
func getrowsql(imdb bool, qu *Querywithargs) *sql.Row {
	return getstatement(qu.QueryString, imdb).QueryRow(qu.Args...)
}
func getrowvalue(imdb bool, obj interface{}, qu *Querywithargs) error {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	return getrowsql(imdb, qu).Scan(obj)
}
func getrowvalueobj(imdb bool, qu *Querywithargs) (interface{}, error) {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	var obj interface{}
	err := getrowsql(imdb, qu).Scan(obj)
	return obj, err
}
func execsql(imdb bool, named bool, qu *Querywithargs) (sql.Result, error) {

	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	if named {
		return getnamedstatement(qu.QueryString, imdb).Exec(qu.Args[0])
	} else {
		return getstatement(qu.QueryString, imdb).Exec(qu.Args...)
	}
}

type Querywithargs struct {
	Query       Query
	QueryString string
	Args        []interface{}
}

func (q *Querywithargs) close() {
	if q != nil {
		q.Args = nil
		q = nil
	}
}

func queryStructScanSingle(table string, imdb bool, columns string, targetobj interface{}, qu *Querywithargs) error {

	buildquery(columns, table, qu, false)

	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query count", zap.String("Query", qu.QueryString), zap.Any("args", qu.Args))
	}

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	vType := reflect.TypeOf(targetobj)
	if k := vType.Kind(); k != reflect.Ptr {
		return errors.New(k.String() + strMustBePointer)
	}
	sliceItem := reflect.New(vType.Elem()).Elem()
	err := getrow(imdb, false, qu).StructScan(sliceItem.Addr().Interface())

	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return err
	}
	reflect.Indirect(reflect.ValueOf(targetobj)).Set(sliceItem)
	return nil
}
func queryStructScan(table string, imdb bool, columns string, counter int, targetobj interface{}, qu *Querywithargs) error {
	buildquery(columns, table, qu, false)
	return queryStructScanQuery(imdb, targetobj, false, qu)
}

func queryStructScanQuery(imdb bool, targetobj interface{}, simpletype bool, qu *Querywithargs) error {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := getrows(imdb, false, qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return err
	}
	defer rows.Close()

	vType := reflect.TypeOf(targetobj)
	if k := vType.Kind(); k != reflect.Ptr {
		return errors.New(k.String() + strMustBePointer)
	}
	sliceType := vType.Elem()
	if reflect.Slice != sliceType.Kind() {
		return errors.New(sliceType.String() + " must be a slice: ErrNotASlicePointer")
	}
	sliceVal := reflect.Indirect(reflect.ValueOf(targetobj))
	sliceItem := reflect.New(sliceType.Elem()).Elem()
	for rows.Next() {
		if simpletype {
			err = rows.Scan(sliceItem.Addr().Interface())
		} else {
			err = rows.StructScan(sliceItem.Addr().Interface())
		}
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return err
		}
		sliceVal.Set(reflect.Append(sliceVal, sliceItem))
	}

	return nil
}

func QueryDbmovie(qu Querywithargs) ([]Dbmovie, error) {
	defer qu.close()
	table := "dbmovies"
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}

	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []Dbmovie
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryDbmovieTitle(qu Querywithargs) ([]DbmovieTitle, error) {
	defer qu.close()

	table := "dbmovie_titles"
	columns := "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []DbmovieTitle
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetDbserie(qu Querywithargs) (Dbserie, error) {
	defer qu.close()

	table := "dbseries"
	columns := "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result Dbserie
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

func QueryDbserie(qu Querywithargs) ([]Dbserie, error) {
	defer qu.close()

	table := "dbseries"
	columns := "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []Dbserie
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetDbserieEpisodes(qu Querywithargs) (DbserieEpisode, error) {
	defer qu.close()

	table := "dbserie_episodes"
	columns := "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result DbserieEpisode
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}
func QueryDbserieEpisodes(qu Querywithargs) ([]DbserieEpisode, error) {
	defer qu.close()

	table := "dbserie_episodes"
	columns := "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []DbserieEpisode
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryDbserieAlternates(qu Querywithargs) ([]DbserieAlternate, error) {
	defer qu.close()

	table := "dbserie_alternates"
	columns := "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []DbserieAlternate
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetSeries(qu Querywithargs) (Serie, error) {
	defer qu.close()

	table := "series"
	columns := "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result Serie
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

func GetSerieEpisodes(qu Querywithargs) (SerieEpisode, error) {
	defer qu.close()

	table := "serie_episodes"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result SerieEpisode
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}
func QuerySerieEpisodes(qu Querywithargs) ([]SerieEpisode, error) {
	defer qu.close()

	table := "serie_episodes"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []SerieEpisode
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetSerieEpisodeFiles(qu Querywithargs) (SerieEpisodeFile, error) {
	defer qu.close()

	table := "serie_episode_files"
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result SerieEpisodeFile
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

func GetMovies(qu Querywithargs) (Movie, error) {
	defer qu.close()

	table := "movies"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result Movie
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}
func QueryMovies(qu Querywithargs) ([]Movie, error) {
	defer qu.close()

	table := "movies"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []Movie
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetMovieFiles(qu Querywithargs) (MovieFile, error) {
	defer qu.close()

	table := "movie_files"
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result MovieFile
	err := queryStructScanSingle(table, false, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

func QueryQualities(qu Querywithargs) ([]Qualities, error) {
	defer qu.close()

	table := "qualities"
	columns := "id,created_at,updated_at,type,name,regex,strings,priority,use_regex"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}

	var result []Qualities
	err := queryStructScan(table, false, columns, -1, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryJobHistory(qu Querywithargs) ([]JobHistory, error) {
	defer qu.close()

	table := "job_histories"
	columns := "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []JobHistory
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QuerySerieFileUnmatched(qu Querywithargs) ([]SerieFileUnmatched, error) {
	defer qu.close()

	table := "serie_file_unmatcheds"
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []SerieFileUnmatched
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryMovieFileUnmatched(qu Querywithargs) ([]MovieFileUnmatched, error) {
	defer qu.close()

	table := "movie_file_unmatcheds"
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []MovieFileUnmatched
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryResultMovies(qu Querywithargs) ([]ResultMovies, error) {
	defer qu.close()

	table := "movies"
	columns := `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []ResultMovies
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryResultSeries(qu Querywithargs) ([]ResultSeries, error) {
	defer qu.close()

	table := "series"
	columns := `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []ResultSeries
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryResultSerieEpisodes(qu Querywithargs) ([]ResultSerieEpisodes, error) {
	defer qu.close()

	table := "serie_episodes"
	columns := `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []ResultSerieEpisodes
	err := queryStructScan(table, false, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetImdbRating(qu Querywithargs) (ImdbRatings, error) {
	defer qu.close()

	table := "imdb_ratings"
	columns := "id,created_at,updated_at,tconst,num_votes,average_rating"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}

	var result ImdbRatings
	err := queryStructScanSingle(table, true, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

func QueryImdbAka(qu Querywithargs) ([]ImdbAka, error) {
	defer qu.close()

	table := "imdb_akas"
	columns := "id,created_at,updated_at,tconst,ordering,title,slug,region,language,types,attributes,is_original_title"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = int(qu.Query.Limit)
	}

	var result []ImdbAka
	err := queryStructScan(table, true, columns, counter, &result, &qu)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetImdbTitle(qu Querywithargs) (ImdbTitle, error) {
	defer qu.close()

	table := "imdb_titles"
	columns := "tconst,title_type,primary_title,slug,original_title,is_adult,start_year,end_year,runtime_minutes,genres"
	if qu.Query.Select != "" {
		columns = qu.Query.Select
	}
	var result ImdbTitle
	err := queryStructScanSingle(table, true, columns, &result, &qu)
	if err != nil {
		return result, err
	}
	return result, nil
}

const strfrom string = " from "
const strselect string = "select "
const strinner string = " inner join "
const strorder string = " order by "
const strlimit string = " limit "

func buildquery(columns string, table string, qu *Querywithargs, count bool) {
	var bld strings.Builder
	bld.Grow(len(columns) + len(qu.Query.Where) + len(qu.Query.InnerJoin) + 200)
	bld.WriteString(strselect)
	defer bld.Reset()

	if strings.Contains(columns, table+".") {
		bld.WriteString(columns)
	} else {
		if count {
			bld.WriteString("count()")
		} else {
			if qu.Query.InnerJoin != "" {
				bld.WriteString(table + ".*")
			} else {
				bld.WriteString(columns)
			}
		}
	}
	bld.WriteString(strfrom)
	bld.WriteString(table)
	if qu.Query.InnerJoin != "" {
		bld.WriteString(strinner + qu.Query.InnerJoin)
	}
	if qu.Query.Where != "" {
		bld.WriteString(strWhere + qu.Query.Where)
	}
	if qu.Query.OrderBy != "" {
		bld.WriteString(strorder + qu.Query.OrderBy)
	}
	if qu.Query.Limit != 0 {
		if qu.Query.Offset != 0 {
			bld.WriteString(logger.StringBuild(strlimit, strconv.Itoa(int(qu.Query.Offset)), ",", strconv.Itoa(int(qu.Query.Limit))))
		} else {
			bld.WriteString(strlimit + strconv.Itoa(int(qu.Query.Limit)))
		}
	}
	qu.QueryString = bld.String()
}

type Dbstatic_OneIntOneBool struct {
	Num int  `db:"num"`
	Bl  bool `db:"bl"`
}

// requires 1 column - int
func QueryStaticColumnsOneIntOneBool(qu Querywithargs) ([]Dbstatic_OneIntOneBool, error) {
	defer qu.close()

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := getrowssql(false, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return []Dbstatic_OneIntOneBool{}, err
	}
	defer rows.Close()
	var result []Dbstatic_OneIntOneBool

	var num int
	var bl bool

	for rows.Next() {
		err = rows.Scan(&num, &bl)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_OneIntOneBool{Num: num, Bl: bl})
	}

	return result, nil
}

type Dbstatic_OneInt struct {
	Num int `db:"num"`
}

// Select has to be in it
func QueryStaticColumnsOneIntQueryObject(table string, qu Querywithargs) ([]Dbstatic_OneInt, error) {
	defer qu.close()

	buildquery(qu.Query.Select, table, &qu, false)

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(false, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return []Dbstatic_OneInt{}, err
	}
	defer rows.Close()
	var result []Dbstatic_OneInt

	var num int

	for rows.Next() {
		err = rows.Scan(&num)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_OneInt{Num: num})
	}

	return result, nil
}

// requires 1 column - string
func QueryStaticStringArray(imdb bool, size int, qu Querywithargs) []string {
	defer qu.close()

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(imdb, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return []string{}
	}
	defer rows.Close()
	var result []string
	if size != 0 {
		result = logger.GrowSliceBy(result, size)
	}

	result = rowsloop(rows, qu.QueryString, result)
	//var str string
	// for rows.Next() {
	// 	err = rows.Scan(&str)
	// 	if err != nil {
	// 		logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
	// 		return []string{}
	// 	}
	// 	result = append(result, str)
	// }

	return result
}

// Filter any Slice
// ex.
//
//	b := Filter(a.Elements, func(e Element) bool {
//		return strings.Contains(strings.ToLower(e.Name), strings.ToLower("woman"))
//	})
func rowsloop[T any](rows *sql.Rows, query string, result []T) []T {
	var err error
	var u T
	for rows.Next() {
		err = rows.Scan(&u)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
			break
		} else {
			result = append(result, u)
		}
	}
	return result
}

// requires 1 column - string
func QueryStaticIntArray(size int, qu Querywithargs) []int {
	defer qu.close()

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(false, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return []int{}
	}
	defer rows.Close()
	var result []int
	if size != 0 {
		result = logger.GrowSliceBy(result, size)
	}

	result = rowsloop(rows, qu.QueryString, result)
	// var num int

	// for rows.Next() {
	// 	err = rows.Scan(&num)
	// 	if err != nil {
	// 		logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
	// 		return []int{}
	// 	}
	// 	result = append(result, num)
	// }

	return result
}

func QueryStaticColumnsMapStringInt(result *map[string]int, qu Querywithargs) error {
	defer qu.close()
	err := queryStaticColumnsMap(false, result, true, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("QueryStruct", zap.String("Query", qu.QueryString), zap.Error(err))
		} else {
			return nil
		}
		return err
	}
	return nil
}

func queryStaticColumnsMap(imdb bool, targetobj interface{}, usefield2 bool, qu *Querywithargs) error {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := getrowssql(imdb, false, qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("QueryStruct", zap.String("Query", qu.QueryString), zap.Error(err))
		} else {
			return nil
		}
		return err
	}
	defer rows.Close()

	vType := reflect.TypeOf(targetobj)
	if k := vType.Kind(); k != reflect.Ptr {
		return errors.New(k.String() + strMustBePointer)
	}
	mapType := vType.Elem()
	if reflect.Map != mapType.Kind() {
		return errors.New(mapType.String() + " must be a map: ErrNotASlicePointer")
	}
	mapVal := reflect.Indirect(reflect.ValueOf(targetobj))
	field1get := reflect.New(mapType.Key()).Elem()
	field2get := reflect.New(mapType.Elem()).Elem()
	valtrue := reflect.ValueOf(true)
	for rows.Next() {
		if !usefield2 {
			err = rows.Scan(field1get.Addr().Interface())
		} else {
			err = rows.Scan(field1get.Addr().Interface(), field2get.Addr().Interface())
		}

		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return err
		}

		if !usefield2 {
			mapVal.SetMapIndex(field1get, valtrue)
		} else {
			mapVal.SetMapIndex(field1get, field2get)
		}
	}

	return nil
}

type Dbstatic_OneStringOneInt struct {
	Str string `db:"str"`
	Num int    `db:"num"`
}

// requires 2 columns- string and int
func QueryStaticColumnsOneStringOneInt(imdb bool, size int, qu Querywithargs) (result []Dbstatic_OneStringOneInt, err error) {
	defer qu.close()

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(imdb, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return
	}
	defer rows.Close()
	if size != 0 {
		result = logger.GrowSliceBy(result, size)
	}

	var num int
	var str string
	for rows.Next() {
		err = rows.Scan(&str, &num)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_OneStringOneInt{Num: num, Str: str})
	}
	return
}

type Dbstatic_TwoInt struct {
	Num1 int `db:"num1"`
	Num2 int `db:"num2"`
}

// requires 2 columns- int and int
func QueryStaticColumnsTwoInt(qu Querywithargs) ([]Dbstatic_TwoInt, error) {
	defer qu.close()
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(false, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return []Dbstatic_TwoInt{}, err
	}
	defer rows.Close()

	var result []Dbstatic_TwoInt

	var num1, num2 int

	for rows.Next() {
		err = rows.Scan(&num1, &num2)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_TwoInt{Num1: num1, Num2: num2})
	}

	return result, nil
}

type Dbstatic_ThreeString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
}

// requires 2 columns- int and int
func QueryStaticColumnsThreeString(imdb bool, qu Querywithargs) ([]Dbstatic_ThreeString, error) {
	defer qu.close()
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(imdb, false, &qu)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", qu.QueryString), zap.Error(err))
		}
		return []Dbstatic_ThreeString{}, err
	}
	defer rows.Close()
	var result []Dbstatic_ThreeString

	var str1, str2, str3 string

	for rows.Next() {
		err = rows.Scan(&str1, &str2, &str3)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", qu.QueryString), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_ThreeString{Str1: str1, Str2: str2, Str3: str3})
	}

	return result, nil
}

// Uses column id
func CountRows(table string, qu Querywithargs) (int, error) {
	defer qu.close()
	var obj int
	qu.Query.Offset = 0
	qu.Query.Limit = 0
	buildquery("count()", table, &qu, true)
	err := getrowvalue(false, &obj, &qu)

	return obj, err
}

func CountRowsStatic(qu Querywithargs) (int, error) {
	defer qu.close()
	var obj int
	err := getrowvalue(false, &obj, &qu)
	return obj, err
}

func CountRowsStaticNoError(qu Querywithargs) int {
	defer qu.close()
	var obj int
	getrowvalue(false, &obj, &qu)
	return obj
}

func QueryColumnString(qu Querywithargs) (string, error) {
	defer qu.close()
	var obj string
	err := getrowvalue(false, &obj, &qu)
	return obj, err
}

func QueryColumnUint(qu Querywithargs) (uint, error) {
	defer qu.close()
	var obj uint
	err := getrowvalue(false, &obj, &qu)
	return obj, err
}

func QueryImdbColumnUint(qu Querywithargs) (uint, error) {
	defer qu.close()
	var obj uint
	err := getrowvalue(true, &obj, &qu)
	return obj, err
}

func QueryColumnBool(qu Querywithargs) (bool, error) {
	defer qu.close()
	var obj bool
	err := getrowvalue(false, &obj, &qu)
	return obj, err
}

func insertarrayprepare(table string, columns *logger.InStringArrayStruct) string {
	cols := ""
	vals := ""
	for idx := range columns.Arr {
		if idx != 0 {
			cols += ","
			vals += ","
		}
		cols += columns.Arr[idx]
		vals += "?"
	}
	return logger.StringBuild("insert into ", table, " (", cols, ") values (", vals, ")")
}
func InsertStatic(qu Querywithargs) (sql.Result, error) {
	defer qu.close()

	result, err := dbexec("main", &qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", qu.QueryString), zap.Any("Values", qu.Args), zap.Error(err))
	}

	return result, err
}
func InsertNamed(query string, obj interface{}) (sql.Result, error) {
	result, err := dbexecnamed("main", query, obj)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", query), zap.Any("values", obj), zap.Error(err))
	}

	return result, err
}
func InsertArray(table string, columns *logger.InStringArrayStruct, values []interface{}) (sql.Result, error) {
	defer columns.Close()
	result, err := dbexec("main", &Querywithargs{Args: values, QueryString: insertarrayprepare(table, columns)})
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Table", table), zap.Any("Colums", columns), zap.Any("Values", values), zap.Error(err))
	}
	return result, err
}

func DbexecUncached(query string, args []interface{}) (sql.Result, error) {

	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query exec", zap.String("Query", query), zap.Any("args", args))
	}

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	result, err := getdb(false).Exec(query, args...)

	if err != nil {
		logger.Log.GlobalLogger.Debug("error query. ", zap.String("Query", query), zap.Any("Values", args), zap.Error(err))

	}
	return result, err
}
func Dbexec(dbtype string, qu Querywithargs) (sql.Result, error) {
	defer qu.close()

	return dbexec(dbtype, &qu)
}
func dbexec(dbtype string, qu *Querywithargs) (sql.Result, error) {
	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query exec", zap.String("Query", qu.QueryString), zap.Any("args", qu.Args))
	}

	return execsql(dbtype == "imdb", false, qu)
}
func dbexecnamed(dbtype string, query string, obj interface{}) (sql.Result, error) {
	return execsql(dbtype == "imdb", true, &Querywithargs{QueryString: query, Args: []interface{}{obj}})
}
func updatearrayprepare(table string, columns *logger.InStringArrayStruct, qu *Querywithargs) string {
	cols := ""
	for idx := range columns.Arr {
		if idx != 0 {
			cols += ","
		}
		cols += columns.Arr[idx] + " = ?"
	}
	if qu.Query.Where != "" {
		return logger.StringBuild("update ", table, " set ", cols, strWhere, qu.Query.Where)
	}
	return logger.StringBuild("update ", table, " set ", cols)
}
func UpdateArray(table string, columns *logger.InStringArrayStruct, values []interface{}, qu Querywithargs) (sql.Result, error) {
	defer qu.close()

	defer columns.Close()
	result, err := dbexec("main", &Querywithargs{Args: append(values, qu.Args...), QueryString: updatearrayprepare(table, columns, &qu)})
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Table", table), zap.Any("Columns", columns), zap.Any("Values", values), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}
	return result, err
}
func UpdateNamed(query string, obj interface{}) (sql.Result, error) {
	result, err := dbexecnamed("main", query, obj)
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Query", query), zap.Any("Values", obj), zap.Error(err))
	}

	return result, err
}

func updatecolprepare(table string, column string, qu *Query) string {
	if qu.Where != "" {
		return logger.StringBuild("update ", table, " set ", column, " = ?", strWhere, qu.Where)
	} else {
		return logger.StringBuild("update ", table, " set ", column, " = ?")
	}
}
func UpdateColumn(table string, column string, value interface{}, qu Querywithargs) (sql.Result, error) {
	defer qu.close()
	result, err := dbexec("main", &Querywithargs{Args: append([]interface{}{value}, qu.Args...), QueryString: updatecolprepare(table, column, &qu.Query)})
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Table", table), zap.String("Column", column), zap.Any("Value", value), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}

	return result, err
}
func UpdateColumnStatic(qu Querywithargs) error {
	defer qu.close()

	_, err := dbexec("main", &qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Query", qu.QueryString), zap.Any("Values", qu.Args), zap.Error(err))
	}

	return err
}

func DeleteRow(table string, qu Querywithargs) (sql.Result, error) {
	defer qu.close()
	if qu.Query.Where != "" {
		qu.QueryString = logger.StringBuild("delete from ", table, strWhere, qu.Query.Where)
	} else {
		qu.QueryString = logger.StringBuild("delete from ", table)
	}
	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query count", zap.String("Query", qu.QueryString), zap.Any("args", qu.Args))
	}

	result, err := execsql(false, false, &qu)

	if err != nil {
		logger.Log.GlobalLogger.Error("Delete", zap.String("Table", table), zap.String("Where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}

	return result, err
}
func DeleteRowStatic(qu Querywithargs) error {
	defer qu.close()

	_, err := dbexec("main", &qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", qu.QueryString), zap.Any("Values", qu.Args), zap.Error(err))
	}

	return err
}

func UpsertNamed(table string, columns *logger.InStringArrayStruct, obj interface{}, wherenamed string, qu Querywithargs) (sql.Result, error) {
	defer qu.close()
	var counter int
	defer columns.Close()

	counter, _ = CountRows(table, qu)
	var bld strings.Builder
	defer bld.Reset()
	if counter == 0 {
		bld.WriteString("Insert into " + table + " (")
		var bld2 strings.Builder
		defer bld2.Reset()
		bld2.WriteString(") values (")
		for idx := range columns.Arr {
			if idx >= 1 {
				bld.WriteString(", ")
				bld2.WriteString(", ")
			}
			bld.WriteString(columns.Arr[idx])
			bld2.WriteString(":" + columns.Arr[idx])
		}
		qu.QueryString = bld.String() + bld2.String() + ")"
		qu.Args = []interface{}{obj}
		result, err := execsql(false, true, &qu)
		if err != nil {
			logger.Log.GlobalLogger.Error("Upsert-insert", zap.String("Table", table), zap.Any("Columns", columns), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
		}

		return result, err
	}
	bld.WriteString("Update " + table + " SET ")
	for idx := range columns.Arr {
		if idx >= 1 {
			bld.WriteString(", ")
		}
		bld.WriteString(columns.Arr[idx] + "= :" + columns.Arr[idx])
	}
	qu.QueryString = bld.String() + strWhere + wherenamed
	qu.Args = []interface{}{obj}
	result, err := execsql(false, true, &qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Upsert-update", zap.String("table", table), zap.Any("Columns", columns), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}

	return result, err
}

func ImdbCountRowsStatic(qu Querywithargs) (int, error) {
	defer qu.close()
	var obj int
	err := getrowvalue(true, &obj, &qu)
	return obj, err
}

func DbQuickCheck() string {
	logger.Log.GlobalLogger.Info("Check Database for Errors 1")
	str, _ := QueryColumnString(Querywithargs{QueryString: "PRAGMA quick_check;"})
	logger.Log.GlobalLogger.Info("Check Database for Errors 2")
	return str
}

func DbIntegrityCheck() string {
	str, _ := QueryColumnString(Querywithargs{QueryString: "PRAGMA integrity_check;"})
	return str
}

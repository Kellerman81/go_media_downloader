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

func GetDbmovie(qu *Query, args ...interface{}) (result Dbmovie, err error) {
	table := "dbmovies"
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func getrows(query string, imdb bool, named bool, args []interface{}) (*sqlx.Rows, error) {
	if named {
		stmt := getnamedstatement(query, imdb)
		return stmt.Queryx(args[0])
	} else {
		stmt := getstatement(query, imdb)
		return stmt.Queryx(args...)
	}
}
func getrowssql(query string, imdb bool, named bool, args []interface{}) (*sql.Rows, error) {
	if named {
		stmt := getnamedstatement(query, imdb)
		return stmt.Query(args[0])
	} else {
		stmt := getstatement(query, imdb)
		return stmt.Query(args...)
	}
}

func getrow(query string, imdb bool, named bool, args []interface{}) *sqlx.Row {
	if named {
		stmt := getnamedstatement(query, imdb)
		return stmt.QueryRowx(args[0])
	} else {
		stmt := getstatement(query, imdb)
		return stmt.QueryRowx(args...)
	}
}
func getrowsql(query string, imdb bool, args []interface{}) *sql.Row {
	stmt := getstatement(query, imdb)
	return stmt.QueryRow(args...)
}
func getrowvalue(query string, imdb bool, obj interface{}, args []interface{}) error {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	return getrowsql(query, imdb, args).Scan(obj)
}
func execsql(query string, imdb bool, named bool, args []interface{}) (sql.Result, error) {
	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	var res sql.Result
	var err error
	if named {
		res, err = getnamedstatement(query, imdb).Exec(args[0])
	} else {
		res, err = getstatement(query, imdb).Exec(args...)
	}
	return res, err
}
func queryStructScanSingle(table string, imdb bool, columns string, qu *Query, targetobj interface{}, args []interface{}) error {
	query := buildquery(columns, table, qu, false)

	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query count", zap.String("Query", query), zap.Any("args", args))
	}

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	vType := reflect.TypeOf(targetobj)
	if k := vType.Kind(); k != reflect.Ptr {
		return errors.New(k.String() + " must be a pointer: ErrNotAPointer")
	}
	sliceVal := reflect.Indirect(reflect.ValueOf(targetobj))
	sliceItem := reflect.New(vType.Elem()).Elem()
	err := getrow(query, imdb, false, args).StructScan(sliceItem.Addr().Interface())

	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
		}
		return err
	}
	sliceVal.Set(sliceItem)

	return nil
}
func queryStructScan(table string, imdb bool, columns string, qu *Query, counter int, targetobj interface{}, args []interface{}) error {
	return queryStructScanQuery(buildquery(columns, table, qu, false), imdb, targetobj, false, args)
}

func queryStructScanQuery(query string, imdb bool, targetobj interface{}, simpletype bool, args []interface{}) error {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := getrows(query, imdb, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return err
	}
	defer rows.Close()

	vType := reflect.TypeOf(targetobj)
	if k := vType.Kind(); k != reflect.Ptr {
		return errors.New(k.String() + " must be a pointer: ErrNotAPointer")
	}
	sliceType := vType.Elem()
	if reflect.Slice != sliceType.Kind() {
		return errors.New(sliceType.String() + " must be a slice: ErrNotASlicePointer")
	}
	sliceVal := reflect.Indirect(reflect.ValueOf(targetobj))
	itemType := sliceType.Elem()
	sliceItem := reflect.New(itemType).Elem()
	for rows.Next() {
		if simpletype {
			err = rows.Scan(sliceItem.Addr().Interface())
		} else {
			err = rows.StructScan(sliceItem.Addr().Interface())
		}
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
			return err
		}
		sliceVal.Set(reflect.Append(sliceVal, sliceItem))
	}

	return nil
}

func QueryDbmovie(qu *Query, args ...interface{}) ([]Dbmovie, error) {
	table := "dbmovies"
	columns := "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []Dbmovie //= make([]Dbmovie, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryDbmovieTitle(qu *Query, args ...interface{}) ([]DbmovieTitle, error) {

	table := "dbmovie_titles"
	columns := "id,created_at,updated_at,dbmovie_id,title,slug,region"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []DbmovieTitle //= make([]DbmovieTitle, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetDbserie(qu *Query, args ...interface{}) (result Dbserie, err error) {

	table := "dbseries"
	columns := "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func QueryDbserie(qu *Query, args ...interface{}) ([]Dbserie, error) {

	table := "dbseries"
	columns := "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []Dbserie //= make([]Dbserie, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetDbserieEpisodes(qu *Query, args ...interface{}) (result DbserieEpisode, err error) {

	table := "dbserie_episodes"
	columns := "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}
func QueryDbserieEpisodes(qu *Query, args ...interface{}) ([]DbserieEpisode, error) {

	table := "dbserie_episodes"
	columns := "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []DbserieEpisode //= make([]DbserieEpisode, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryDbserieAlternates(qu *Query, args ...interface{}) ([]DbserieAlternate, error) {

	table := "dbserie_alternates"
	columns := "id,created_at,updated_at,title,slug,region,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []DbserieAlternate //= make([]DbserieAlternate, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetSeries(qu *Query, args ...interface{}) (result Serie, err error) {

	table := "series"
	columns := "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func GetSerieEpisodes(qu *Query, args ...interface{}) (result SerieEpisode, err error) {

	table := "serie_episodes"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}
func QuerySerieEpisodes(qu *Query, args ...interface{}) ([]SerieEpisode, error) {

	table := "serie_episodes"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []SerieEpisode //= make([]SerieEpisode, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetSerieEpisodeFiles(qu *Query, args ...interface{}) (result SerieEpisodeFile, err error) {

	table := "serie_episode_files"
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func GetMovies(qu *Query, args ...interface{}) (result Movie, err error) {

	table := "movies"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}
func QueryMovies(qu *Query, args ...interface{}) ([]Movie, error) {

	table := "movies"
	columns := "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []Movie //= make([]Movie, 0, 100)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetMovieFiles(qu *Query, args ...interface{}) (result MovieFile, err error) {

	table := "movie_files"
	columns := "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, false, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func QueryQualities(qu *Query, args ...interface{}) ([]Qualities, error) {

	table := "qualities"
	columns := "id,created_at,updated_at,type,name,regex,strings,priority,use_regex"
	if qu.Select != "" {
		columns = qu.Select
	}

	var result []Qualities //= make([]Qualities, 0, 20)
	err := queryStructScan(table, false, columns, qu, -1, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryJobHistory(qu *Query, args ...interface{}) ([]JobHistory, error) {

	table := "job_histories"
	columns := "id,created_at,updated_at,job_type,job_category,job_group,started,ended"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []JobHistory //= make([]JobHistory, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QuerySerieFileUnmatched(qu *Query, args ...interface{}) ([]SerieFileUnmatched, error) {

	table := "serie_file_unmatcheds"
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []SerieFileUnmatched //= make([]SerieFileUnmatched, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryMovieFileUnmatched(qu *Query, args ...interface{}) ([]MovieFileUnmatched, error) {

	table := "movie_file_unmatcheds"
	columns := "id,created_at,updated_at,listname,filepath,last_checked,parsed_data"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []MovieFileUnmatched //= make([]MovieFileUnmatched, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryResultMovies(qu *Query, args ...interface{}) ([]ResultMovies, error) {

	table := "movies"
	columns := `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []ResultMovies //= make([]ResultMovies, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryResultSeries(qu *Query, args ...interface{}) ([]ResultSeries, error) {

	table := "series"
	columns := `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []ResultSeries //= make([]ResultSeries, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryResultSerieEpisodes(qu *Query, args ...interface{}) ([]ResultSerieEpisodes, error) {

	table := "serie_episodes"
	columns := `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []ResultSerieEpisodes //= make([]ResultSerieEpisodes, 0, 20)
	err := queryStructScan(table, false, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetImdbRating(qu *Query, args ...interface{}) (result ImdbRatings, err error) {

	table := "imdb_ratings"
	columns := "id,created_at,updated_at,tconst,num_votes,average_rating"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, true, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func QueryImdbAka(qu *Query, args ...interface{}) ([]ImdbAka, error) {

	table := "imdb_akas"
	columns := "id,created_at,updated_at,tconst,ordering,title,slug,region,language,types,attributes,is_original_title"
	if qu.Select != "" {
		columns = qu.Select
	}
	counter := -1
	if qu.Limit >= 1 {
		counter = int(qu.Limit)
	}

	var result []ImdbAka //= make([]ImdbAka, 0, 10)
	err := queryStructScan(table, true, columns, qu, counter, &result, args)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetImdbTitle(qu *Query, args ...interface{}) (result ImdbTitle, err error) {

	table := "imdb_titles"
	columns := "tconst,title_type,primary_title,slug,original_title,is_adult,start_year,end_year,runtime_minutes,genres"
	if qu.Select != "" {
		columns = qu.Select
	}

	err = queryStructScanSingle(table, true, columns, qu, &result, args)
	if err != nil {
		return result, err
	}
	return result, nil
}

func buildquery(columns string, table string, qu *Query, count bool) string {
	returnv := ""
	if strings.Contains(columns, table+".") {
		returnv = columns
	} else {
		if count {
			returnv = "count()"
		} else {
			if qu.InnerJoin != "" {
				returnv = table + ".*"
			} else {
				returnv = columns
			}
		}
	}
	inner := ""
	if qu.InnerJoin != "" {
		inner = " inner join " + qu.InnerJoin
	}
	where := ""
	if qu.Where != "" {
		where = " where " + qu.Where
	}
	orderby := ""
	if qu.OrderBy != "" {
		orderby = " order by " + qu.OrderBy
	}
	limit := ""
	if qu.Limit != 0 {
		if qu.Offset != 0 {
			limit = " limit " + strconv.Itoa(int(qu.Offset)) + "," + strconv.Itoa(int(qu.Limit))
		} else {
			limit = " limit " + strconv.Itoa(int(qu.Limit))
		}
	}
	return "select " + returnv + " from " + table + inner + where + orderby + limit
}

type Dbstatic_OneIntOneBool struct {
	Num int  `db:"num"`
	Bl  bool `db:"bl"`
}

// requires 1 column - int
func QueryStaticColumnsOneIntOneBool(query string, args ...interface{}) ([]Dbstatic_OneIntOneBool, error) {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	var result []Dbstatic_OneIntOneBool //= make([]Dbstatic_OneIntOneBool, 0, 20)
	rows, err := getrowssql(query, false, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return result, err
	}
	defer rows.Close()

	var num int
	var bl bool

	for rows.Next() {
		err = rows.Scan(&num, &bl)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
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
func QueryStaticColumnsOneIntQueryObject(table string, qu *Query, args ...interface{}) ([]Dbstatic_OneInt, error) {

	query := buildquery(qu.Select, table, qu, false)

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	var result []Dbstatic_OneInt //= make([]Dbstatic_OneInt, 0, 20)
	rows, err := getrowssql(query, false, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return result, err
	}
	defer rows.Close()

	var num int

	for rows.Next() {
		err = rows.Scan(&num)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_OneInt{Num: num})
	}

	return result, nil
}

// requires 1 column - string
func QueryStaticStringArray(query string, imdb bool, size int, args ...interface{}) []string {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	var result []string //= make([]string, 0, size)
	if size != 0 {
		result = make([]string, 0, size)
	}
	rows, err := getrowssql(query, imdb, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return result
	}
	defer rows.Close()

	var str string

	for rows.Next() {
		err = rows.Scan(&str)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
			return []string{}
		}
		result = append(result, str)
	}

	return result
}

// requires 1 column - string
func QueryStaticIntArray(query string, size int, args ...interface{}) []int {
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	var result []int //= make([]int, 0, size)
	if size != 0 {
		result = make([]int, 0, size)
	}
	rows, err := getrowssql(query, false, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return result
	}
	defer rows.Close()

	var num int

	for rows.Next() {
		err = rows.Scan(&num)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
			return []int{}
		}
		result = append(result, num)
	}

	return result
}

func QueryStaticColumnsMapStringInt(query string, result *map[string]int, args ...interface{}) error {
	err := queryStaticColumnsMap(query, false, result, true, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("QueryStruct", zap.String("Query", query), zap.Error(err))
		} else {
			return nil
		}
		return err
	}
	return nil
}

func queryStaticColumnsMap(query string, imdb bool, targetobj interface{}, usefield2 bool, args []interface{}) error {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := getrowssql(query, imdb, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("QueryStruct", zap.String("Query", query), zap.Error(err))
		} else {
			return nil
		}
		return err
	}
	defer rows.Close()

	vType := reflect.TypeOf(targetobj)
	if k := vType.Kind(); k != reflect.Ptr {
		return errors.New(k.String() + " must be a pointer: ErrNotAPointer")
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
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
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
func QueryStaticColumnsOneStringOneInt(query string, imdb bool, size int, args ...interface{}) (result []Dbstatic_OneStringOneInt, err error) {

	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(query, imdb, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return
	}
	defer rows.Close()
	if size != 0 {
		result = make([]Dbstatic_OneStringOneInt, 0, size)
	}

	var num int
	var str string
	for rows.Next() {
		err = rows.Scan(&str, &num)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
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
func QueryStaticColumnsTwoInt(query string, args ...interface{}) ([]Dbstatic_TwoInt, error) {
	var result []Dbstatic_TwoInt //= make([]Dbstatic_TwoInt, 0, 5)
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(query, false, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return result, err
	}
	defer rows.Close()

	var num1, num2 int

	for rows.Next() {
		err = rows.Scan(&num1, &num2)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
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
func QueryStaticColumnsThreeString(query string, imdb bool, args ...interface{}) ([]Dbstatic_ThreeString, error) {

	var result []Dbstatic_ThreeString //= make([]Dbstatic_ThreeString, 0, 5)
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	rows, err := getrowssql(query, imdb, false, args)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", query), zap.Error(err))
		}
		return result, err
	}
	defer rows.Close()

	var str1, str2, str3 string

	for rows.Next() {
		err = rows.Scan(&str1, &str2, &str3)
		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", query), zap.Error(err))
			return nil, err
		}
		result = append(result, Dbstatic_ThreeString{Str1: str1, Str2: str2, Str3: str3})
	}

	return result, nil
}

// Uses column id
func CountRows(table string, qu *Query, args ...interface{}) (obj int, err error) {

	qu.Offset = 0
	qu.Limit = 0
	err = getrowvalue(buildquery("count()", table, qu, true), false, &obj, args)

	return obj, err
}

func CountRowsStatic(query string, args ...interface{}) (obj int, err error) {

	err = getrowvalue(query, false, &obj, args)
	return obj, err
}

func CountRowsStaticNoError(query string, args ...interface{}) (obj int) {

	getrowvalue(query, false, &obj, args)
	return obj
}

func QueryColumnString(query string, args ...interface{}) (obj string, err error) {

	err = getrowvalue(query, false, &obj, args)
	return obj, err
}

func QueryColumnUint(query string, args ...interface{}) (obj uint, err error) {

	err = getrowvalue(query, false, &obj, args)
	return obj, err
}

func QueryImdbColumnUint(query string, args ...interface{}) (obj uint, err error) {

	err = getrowvalue(query, true, &obj, args)
	return obj, err
}

func QueryColumnBool(query string, args ...interface{}) (obj bool, err error) {

	err = getrowvalue(query, false, &obj, args)
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
	return "insert into " + table + " (" + cols + ") values (" + vals + ")"
}
func InsertStatic(query string, args ...interface{}) (sql.Result, error) {

	result, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", query), zap.Any("Values", args), zap.Error(err))
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
	result, err := dbexec("main", insertarrayprepare(table, columns), values)
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
func Dbexec(dbtype string, query string, args []interface{}) (sql.Result, error) {

	return dbexec(dbtype, query, args)
}
func dbexec(dbtype string, query string, args []interface{}) (sql.Result, error) {
	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query exec", zap.String("Query", query), zap.Any("args", args))
	}

	return execsql(query, dbtype == "imdb", false, args)
}
func dbexecnamed(dbtype string, query string, obj interface{}) (sql.Result, error) {
	return execsql(query, dbtype == "imdb", true, []interface{}{obj})
}
func updatearrayprepare(table string, columns *logger.InStringArrayStruct, qu *Query, args ...interface{}) string {
	cols := ""
	for idx := range columns.Arr {
		if idx != 0 {
			cols += ","
		}
		cols += columns.Arr[idx] + " = ?"
	}
	where := ""
	if qu.Where != "" {
		where = " where " + qu.Where
	}
	return "update " + table + " set " + cols + where
}
func UpdateArray(table string, columns *logger.InStringArrayStruct, values []interface{}, qu *Query, args ...interface{}) (sql.Result, error) {

	defer columns.Close()
	params := append(values, args...)
	result, err := dbexec("main", updatearrayprepare(table, columns, qu), params)
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Table", table), zap.Any("Columns", columns), zap.Any("Values", values), zap.String("where", qu.Where), zap.Any("Values", args), zap.Error(err))
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
	where := ""
	if qu.Where != "" {
		where = " where " + qu.Where
	}
	return "update " + table + " set " + column + " = ?" + where
}
func UpdateColumn(table string, column string, value interface{}, qu *Query, args ...interface{}) (sql.Result, error) {
	params := append([]interface{}{value}, args...)
	result, err := dbexec("main", updatecolprepare(table, column, qu), params)
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Table", table), zap.String("Column", column), zap.Any("Value", value), zap.String("where", qu.Where), zap.Any("Values", args), zap.Error(err))
	}

	return result, err
}
func UpdateColumnStatic(query string, args ...interface{}) error {

	_, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Query", query), zap.Any("Values", args), zap.Error(err))
	}

	return err
}

func DeleteRow(table string, qu *Query, args ...interface{}) (sql.Result, error) {
	where := ""
	if qu.Where != "" {
		where = " where " + qu.Where
	}
	query := "delete from " + table + where

	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query count", zap.String("Query", query), zap.Any("args", args))
	}

	result, err := execsql(query, false, false, args)

	if err != nil {
		logger.Log.GlobalLogger.Error("Delete", zap.String("Table", table), zap.String("Where", qu.Where), zap.Any("Values", args), zap.Error(err))
	}

	return result, err
}
func DeleteRowStatic(query string, args ...interface{}) error {

	_, err := dbexec("main", query, args)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", query), zap.Any("Values", args), zap.Error(err))
	}

	return err
}

func UpsertNamed(table string, columns *logger.InStringArrayStruct, obj interface{}, wherenamed string, qu *Query, args ...interface{}) (sql.Result, error) {
	var counter int
	defer columns.Close()

	counter, _ = CountRows(table, qu)
	if counter == 0 {
		query := "Insert into " + table + " ("
		query2 := ") values ("
		for idx := range columns.Arr {
			if idx >= 1 {
				query = query + ", "
				query2 = query2 + ", "
			}
			query = query + columns.Arr[idx]
			query2 = query2 + ":" + columns.Arr[idx]
		}
		query = query + query2 + ")"
		result, err := execsql(query, false, true, []interface{}{obj})
		if err != nil {
			logger.Log.GlobalLogger.Error("Upsert-insert", zap.String("Table", table), zap.Any("Columns", columns), zap.String("where", qu.Where), zap.Any("Values", args), zap.Error(err))
		}

		return result, err
	}
	query := "Update " + table + " SET "
	for idx := range columns.Arr {
		if idx >= 1 {
			query = query + ", "
		}
		query = query + columns.Arr[idx] + "= :" + columns.Arr[idx]
	}
	query = query + " where " + wherenamed
	result, err := execsql(query, false, true, []interface{}{obj})
	if err != nil {
		logger.Log.GlobalLogger.Error("Upsert-update", zap.String("table", table), zap.Any("Columns", columns), zap.String("where", qu.Where), zap.Any("Values", args), zap.Error(err))
	}

	return result, err
}

func ImdbCountRowsStatic(query string, args ...interface{}) (obj int, err error) {

	err = getrowvalue(query, true, &obj, args)
	return obj, err
}

func DbQuickCheck() string {
	logger.Log.GlobalLogger.Info("Check Database for Errors 1")
	str, _ := QueryColumnString("PRAGMA quick_check;")
	logger.Log.GlobalLogger.Info("Check Database for Errors 2")
	return str
}

func DbIntegrityCheck() string {
	str, _ := QueryColumnString("PRAGMA integrity_check;")
	return str
}

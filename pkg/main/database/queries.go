package database

import (
	"database/sql"
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
	Limit     int
	Offset    int
	InnerJoin string
}

type searchdb struct {
	table   string
	columns string
	imdb    bool
	size    int
	qu      *Querywithargs
}

type Querywithargs struct {
	Query       Query
	QueryString string
	DontCache   bool
	Args        []interface{}
}

type DbstaticOneIntOneBool struct {
	Num int  `db:"num"`
	Bl  bool `db:"bl"`
}

type DbstaticOneInt struct {
	Num int `db:"num"`
}

type DbstaticOneStringOneInt struct {
	Str string `db:"str"`
	Num int    `db:"num"`
}

type DbstaticTwoStringOneInt struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Num  int    `db:"num"`
}

type DbstaticTwoInt struct {
	Num1 int `db:"num1"`
	Num2 int `db:"num2"`
}
type DbstaticTwoUint struct {
	Num1 uint `db:"num1"`
	Num2 uint `db:"num2"`
}

type DbstaticThreeString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
	Str3 string `db:"str3"`
}

type DbstaticTwoString struct {
	Str1 string `db:"str1"`
	Str2 string `db:"str2"`
}

const strWhere = " where "

var QueryFilterByID = Query{Where: logger.FilterByID}
var QueryFilterByImdb = Query{Where: "imdb_id = ?"}
var QueryFilterByTvdb = Query{Where: "thetvdb_id = ?"}
var QueryFilterByTconst = Query{Where: "tconst = ?"}

func preparex(imdb bool, qu *Querywithargs) (*sqlx.Stmt, error) {
	if imdb {
		return dbImdb.Preparex(qu.QueryString)
	} else {
		return dbData.Preparex(qu.QueryString)
	}
}
func getstatement(qu *Querywithargs, imdb bool) *sqlx.Stmt {
	if logger.GlobalStmtCache.Check(qu.QueryString) {
		return logger.GlobalStmtCache.GetData(qu.QueryString)
	}
	val, err := preparex(imdb, qu)
	if err == nil {
		logger.GlobalStmtCache.SetStmt(qu.QueryString, val, time.Minute*30)
		return val
	}
	logger.Log.GlobalLogger.Error("error generating query", zap.String("query", qu.QueryString), zap.Error(err))
	return nil
}

func preparenamed(imdb bool, qu *Querywithargs) (*sqlx.NamedStmt, error) {
	if imdb {
		return dbImdb.PrepareNamed(qu.QueryString)
	} else {
		return dbData.PrepareNamed(qu.QueryString)
	}
}
func getnamedstatement(qu *Querywithargs, imdb bool) *sqlx.NamedStmt {
	if logger.GlobalStmtNamedCache.Check(qu.QueryString) {
		return logger.GlobalStmtNamedCache.GetData(qu.QueryString)
	}

	val, err := preparenamed(imdb, qu)
	if err == nil {
		logger.GlobalStmtNamedCache.SetNamedStmt(qu.QueryString, val, time.Minute*30)
		return val
	}
	logger.Log.GlobalLogger.Error("error generating query", zap.String("query", qu.QueryString), zap.Error(err))
	return nil
}

func (s *searchdb) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.qu.Close()
	s = nil
}

func GetDbmovie(qu *Querywithargs, result *Dbmovie) error {
	//var result Dbmovie
	return queryObject(&searchdb{
		table:   "dbmovies",
		columns: "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id",
		qu:      qu,
	}, true, result)
}

func execsql(imdb bool, named bool, qu *Querywithargs) (sql.Result, error) {

	readWriteMu.Lock()
	defer readWriteMu.Unlock()
	defer qu.Close()
	if named {
		return getnamedstatement(qu, imdb).Exec(qu.Args[0])
	}
	return getstatement(qu, imdb).Exec(qu.Args...)
}

func (q *Querywithargs) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if q == nil {
		return
	}
	q.Args = nil
	q = nil
}

func queryObject[T any](s *searchdb, complex bool, obj *T) error {
	defer s.Close()
	if s.qu.QueryString == "" {
		if s.qu.Query.Select != "" {
			s.columns = s.qu.Query.Select
		}
		s.buildquery(false)
	}
	var err error
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()
	if complex {
		err = getstatement(s.qu, s.imdb).QueryRowx(s.qu.Args...).StructScan(obj)
	} else {
		err = getstatement(s.qu, s.imdb).QueryRow(s.qu.Args...).Scan(obj)
	}
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", s.qu.QueryString), zap.Error(err))
		}
		var noop T
		*obj = noop
		return err
	}
	return nil
}

func queryx(s *searchdb) (*sqlx.Rows, error) {
	if s.qu.DontCache {
		if s.imdb {
			return dbImdb.Queryx(s.qu.QueryString, s.qu.Args...)
		} else {
			return dbData.Queryx(s.qu.QueryString, s.qu.Args...)
		}
	} else {
		return getstatement(s.qu, s.imdb).Queryx(s.qu.Args...)
	}
}
func queryComplexScan[T any](s *searchdb, targetobj *[]T) error {
	defer s.Close()
	if s.qu.QueryString == "" {
		if s.qu.Query.Select != "" {
			s.columns = s.qu.Query.Select
		}
		s.buildquery(false)
	}
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := queryx(s)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", s.qu.QueryString), zap.Error(err))
		}
		return err
	}
	defer rows.Close()

	if s.size > 0 {
		*targetobj = make([]T, 0, s.size)
	}

	for rows.Next() {
		var u T
		err = rows.StructScan(&u)

		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", s.qu.QueryString), zap.Error(err))
			return err
		}
		*targetobj = append(*targetobj, u)
	}
	err = rows.Err()
	if err != nil {
		logger.Log.GlobalLogger.Error("Query3", zap.String("Query", s.qu.QueryString), zap.Error(err))
		return err
	}

	return nil
}

func query(s *searchdb) (*sql.Rows, error) {
	if s.qu.DontCache {
		if s.imdb {
			return dbImdb.Query(s.qu.QueryString, s.qu.Args...)
		} else {
			return dbData.Query(s.qu.QueryString, s.qu.Args...)
		}
	} else {
		return getstatement(s.qu, s.imdb).Query(s.qu.Args...)
	}
}
func querySimpleScan[T any](s *searchdb, targetobj *[]T) error {
	defer s.Close()
	if s.qu.QueryString == "" {
		if s.qu.Query.Select != "" {
			s.columns = s.qu.Query.Select
		}
		s.buildquery(false)
	}
	var typevar int
	switch any(targetobj).(type) {
	case *[]DbstaticThreeString:
		typevar = 1
	case *[]DbstaticOneInt:
		typevar = 2
	case *[]string:
		typevar = 3
	case *[]int:
		typevar = 4
	case *[]DbstaticOneIntOneBool:
		typevar = 5
	case *[]DbstaticOneStringOneInt:
		typevar = 6
	case *[]DbstaticTwoInt:
		typevar = 7
	case *[]DbstaticTwoString:
		typevar = 8
	case *[]DbstaticTwoStringOneInt:
		typevar = 9
	case *[]uint:
		typevar = 10
	}
	readWriteMu.RLock()
	defer readWriteMu.RUnlock()

	rows, err := query(s)
	if err != nil {
		if err != sql.ErrNoRows {
			logger.Log.GlobalLogger.Error("Query", zap.String("Query", s.qu.QueryString), zap.Error(err))
		}
		return err
	}
	defer rows.Close()

	if s.size > 0 {
		*targetobj = make([]T, 0, s.size)
	}

	var str1, str2, str3 string
	var num1, num2 int
	var bl bool
	for rows.Next() {
		var u T
		switch typevar {
		case 1:
			err = rows.Scan(&str1, &str2, &str3)
			if err == nil {
				setThreeStringFields(&u, str1, str2, str3)
			}
		case 2:
			err = rows.Scan(&num1)
			if err == nil {
				setOneIntFields(&u, num1)
			}
		case 3, 4, 10:
			err = rows.Scan(&u)
		case 5:
			err = rows.Scan(&num1, &bl)
			if err == nil {
				setOneIntOneBoolFields(&u, num1, bl)
			}
		case 6:
			err = rows.Scan(&str1, &num1)
			if err == nil {
				setOneStringOneIntFields(&u, str1, num1)
			}
		case 7:
			err = rows.Scan(&num1, &num2)
			if err == nil {
				setTwoIntFields(&u, num1, num2)
			}
		case 8:
			err = rows.Scan(&str1, &str2)
			if err == nil {
				setTwoStringFields(&u, str1, str2)
			}
		case 9:
			err = rows.Scan(&str1, &str2, &num1)
			if err == nil {
				setTwoStringOneIntFields(&u, str1, str2, num1)
			}
		}

		if err != nil {
			logger.Log.GlobalLogger.Error("Query2", zap.String("Query", s.qu.QueryString), zap.Error(err))
			return err
		}
		*targetobj = append(*targetobj, u)
	}
	err = rows.Err()
	if err != nil {
		logger.Log.GlobalLogger.Error("Query3", zap.String("Query", s.qu.QueryString), zap.Error(err))
		return err
	}

	return nil
}

func QueryDbmovie(qu *Querywithargs, result *[]Dbmovie) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "dbmovies",
		columns: "id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func QueryDbmovieTitle(qu *Querywithargs, result *[]DbmovieTitle) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "dbmovie_titles",
		columns: "id,created_at,updated_at,dbmovie_id,title,slug,region",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func GetDbserie(qu *Querywithargs, result *Dbserie) error {
	//var result Dbserie
	return queryObject(&searchdb{
		table:   "dbseries",
		columns: "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id",
		qu:      qu,
	}, true, result)
}

func QueryDbserie(qu *Querywithargs, result *[]Dbserie) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "dbseries",
		columns: "id,created_at,updated_at,seriename,aliases,season,status,firstaired,network,runtime,language,genre,overview,rating,siterating,siterating_count,slug,imdb_id,thetvdb_id,freebase_m_id,freebase_id,tvrage_id,facebook,instagram,twitter,banner,poster,fanart,identifiedby, trakt_id",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func GetDbserieEpisodes(qu *Querywithargs, result *DbserieEpisode) error {

	//var result DbserieEpisode
	return queryObject(&searchdb{
		table:   "dbserie_episodes",
		columns: "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id",
		qu:      qu,
	}, true, result)
}
func QueryDbserieEpisodes(qu *Querywithargs, result *[]DbserieEpisode) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "dbserie_episodes",
		columns: "id,created_at,updated_at,episode,season,identifier,title,first_aired,overview,poster,runtime,dbserie_id",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}
func QueryDbserieAlternates(qu *Querywithargs, result *[]DbserieAlternate) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "dbserie_alternates",
		columns: "id,created_at,updated_at,title,slug,region,dbserie_id",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func GetSeries(qu *Querywithargs, result *Serie) error {
	//var result Serie
	return queryObject(&searchdb{
		table:   "series",
		columns: "id,created_at,updated_at,listname,rootpath,dbserie_id,dont_upgrade,dont_search",
		qu:      qu,
	}, true, result)
}

func GetSerieEpisodes(qu *Querywithargs, result *SerieEpisode) error {
	//var result SerieEpisode
	return queryObject(&searchdb{
		table:   "serie_episodes",
		columns: "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id",
		qu:      qu,
	}, true, result)
}
func QuerySerieEpisodes(qu *Querywithargs, result *[]SerieEpisode) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "serie_episodes",
		columns: "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,dbserie_episode_id,serie_id,dbserie_id",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func GetSerieEpisodeFiles(qu *Querywithargs, result *SerieEpisodeFile) error {
	//var result SerieEpisodeFile
	return queryObject(&searchdb{
		table:   "serie_episode_files",
		columns: "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,serie_id,serie_episode_id,dbserie_episode_id,dbserie_id",
		qu:      qu,
	}, true, result)
}

func GetMovies(qu *Querywithargs, result *Movie) error {
	//var result Movie
	return queryObject(&searchdb{
		table:   "movies",
		columns: "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id",
		qu:      qu,
	}, true, result)
}
func QueryMovies(qu *Querywithargs, result *[]Movie) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "movies",
		columns: "id,created_at,updated_at,lastscan,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbmovie_id",
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func GetMovieFiles(qu *Querywithargs, result *MovieFile) error {
	//var result MovieFile
	return queryObject(&searchdb{
		table:   "movie_files",
		columns: "id,created_at,updated_at,location,filename,extension,quality_profile,proper,extended,repack,height,width,resolution_id,quality_id,codec_id,audio_id,movie_id,dbmovie_id",
		qu:      qu,
	}, true, result)
}

func QueryQualities(qu *Querywithargs, result *[]Qualities) error {
	err := queryComplexScan(&searchdb{
		table:   "qualities",
		columns: "id,created_at,updated_at,type,name,regex,strings,priority,use_regex",
		qu:      qu,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}
func QueryJobHistory(qu *Querywithargs) ([]JobHistory, error) {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	var result []JobHistory
	err := queryComplexScan(&searchdb{
		table:   "job_histories",
		columns: "id,created_at,updated_at,job_type,job_category,job_group,started,ended",
		qu:      qu,
		size:    counter,
	}, &result)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QuerySerieFileUnmatched(qu *Querywithargs) ([]SerieFileUnmatched, error) {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	var result []SerieFileUnmatched
	err := queryComplexScan(&searchdb{
		table:   "serie_file_unmatcheds",
		columns: "id,created_at,updated_at,listname,filepath,last_checked,parsed_data",
		qu:      qu,
		size:    counter,
	}, &result)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryMovieFileUnmatched(qu *Querywithargs) ([]MovieFileUnmatched, error) {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	var result []MovieFileUnmatched
	err := queryComplexScan(&searchdb{
		table:   "movie_file_unmatcheds",
		columns: "id,created_at,updated_at,listname,filepath,last_checked,parsed_data",
		qu:      qu,
		size:    counter,
	}, &result)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryResultMovies(qu *Querywithargs) ([]ResultMovies, error) {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	var result []ResultMovies
	err := queryComplexScan(&searchdb{
		table:   "movies",
		columns: `dbmovies.id as dbmovie_id,dbmovies.created_at,dbmovies.updated_at,dbmovies.title,dbmovies.release_date,dbmovies.year,dbmovies.adult,dbmovies.budget,dbmovies.genres,dbmovies.original_language,dbmovies.original_title,dbmovies.overview,dbmovies.popularity,dbmovies.revenue,dbmovies.runtime,dbmovies.spoken_languages,dbmovies.status,dbmovies.tagline,dbmovies.vote_average,dbmovies.vote_count,dbmovies.moviedb_id,dbmovies.imdb_id,dbmovies.freebase_m_id,dbmovies.freebase_id,dbmovies.facebook_id,dbmovies.instagram_id,dbmovies.twitter_id,dbmovies.url,dbmovies.backdrop,dbmovies.poster,dbmovies.slug,dbmovies.trakt_id,movies.listname,movies.lastscan,movies.blacklisted,movies.quality_reached,movies.quality_profile,movies.rootpath,movies.missing,movies.id as id`,
		qu:      qu,
		size:    counter,
	}, &result)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}
func QueryResultSeries(qu *Querywithargs) ([]ResultSeries, error) {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	var result []ResultSeries
	err := queryComplexScan(&searchdb{
		table:   "series",
		columns: `dbseries.id as dbserie_id,dbseries.created_at,dbseries.updated_at,dbseries.seriename,dbseries.aliases,dbseries.season,dbseries.status,dbseries.firstaired,dbseries.network,dbseries.runtime,dbseries.language,dbseries.genre,dbseries.overview,dbseries.rating,dbseries.siterating,dbseries.siterating_count,dbseries.slug,dbseries.imdb_id,dbseries.thetvdb_id,dbseries.freebase_m_id,dbseries.freebase_id,dbseries.tvrage_id,dbseries.facebook,dbseries.instagram,dbseries.twitter,dbseries.banner,dbseries.poster,dbseries.fanart,dbseries.identifiedby,dbseries.trakt_id,series.listname,series.rootpath,series.id as id`,
		qu:      qu,
		size:    counter,
	}, &result)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func QueryResultSerieEpisodes(qu *Querywithargs) ([]ResultSerieEpisodes, error) {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	var result []ResultSerieEpisodes
	err := queryComplexScan(&searchdb{
		table:   "serie_episodes",
		columns: `dbserie_episodes.id as dbserie_episode_id,dbserie_episodes.created_at,dbserie_episodes.updated_at,dbserie_episodes.episode,dbserie_episodes.season,dbserie_episodes.identifier,dbserie_episodes.title,dbserie_episodes.first_aired,dbserie_episodes.overview,dbserie_episodes.poster,dbserie_episodes.dbserie_id,dbserie_episodes.runtime,series.listname,series.rootpath,serie_episodes.lastscan,serie_episodes.blacklisted,serie_episodes.quality_reached,serie_episodes.quality_profile,serie_episodes.missing,serie_episodes.id as id`,
		qu:      qu,
		size:    counter,
	}, &result)
	if err != nil && err != sql.ErrNoRows {
		return result, err
	}

	return result, nil
}

func GetImdbRating(qu *Querywithargs, result *ImdbRatings) error {
	return queryObject(&searchdb{
		table:   "imdb_ratings",
		columns: "id,created_at,updated_at,tconst,num_votes,average_rating",
		imdb:    true,
		qu:      qu,
	}, true, result)
}

func QueryImdbAka(qu *Querywithargs, result *[]ImdbAka) error {
	counter := -1
	if qu.Query.Limit >= 1 {
		counter = qu.Query.Limit
	}

	err := queryComplexScan(&searchdb{
		table:   "imdb_akas",
		columns: "id,created_at,updated_at,tconst,ordering,title,slug,region,language,types,attributes,is_original_title",
		imdb:    true,
		qu:      qu,
		size:    counter,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}

	return nil
}

func GetImdbTitle(qu *Querywithargs, result *ImdbTitle) error {
	return queryObject(&searchdb{
		table:   "imdb_titles",
		columns: "tconst,title_type,primary_title,slug,original_title,is_adult,start_year,end_year,runtime_minutes,genres",
		imdb:    true,
		qu:      qu,
	}, true, result)
}

func (s *searchdb) buildquery(count bool) {
	var bld strings.Builder
	if count {
		bld.Grow(len(s.table) + len(s.qu.Query.Where) + len(s.qu.Query.InnerJoin) + len(s.qu.Query.OrderBy) + 50)
	} else {
		bld.Grow(len(s.table) + len(s.columns) + len(s.qu.Query.Where) + len(s.qu.Query.InnerJoin) + len(s.qu.Query.OrderBy) + 50)
	}
	bld.WriteString("select ")

	if strings.Contains(s.columns, s.table+".") {
		bld.WriteString(s.columns)
	} else {
		if count {
			bld.WriteString("count()")
		} else {
			if s.qu.Query.InnerJoin != "" {
				bld.WriteString(s.table)
				bld.WriteString(".*")
			} else {
				bld.WriteString(s.columns)
			}
		}
	}
	bld.WriteString(" from ")
	bld.WriteString(s.table)
	if s.qu.Query.InnerJoin != "" {
		bld.WriteString(" inner join ")
		bld.WriteString(s.qu.Query.InnerJoin)
	}
	if s.qu.Query.Where != "" {
		bld.WriteString(" where ")
		bld.WriteString(s.qu.Query.Where)
	}
	if s.qu.Query.OrderBy != "" {
		bld.WriteString(" order by ")
		bld.WriteString(s.qu.Query.OrderBy)
	}
	if s.qu.Query.Limit != 0 {
		if s.qu.Query.Offset != 0 {
			bld.WriteString(" limit ")
			bld.WriteString(logger.IntToString(s.qu.Query.Offset) + "," + logger.IntToString(s.qu.Query.Limit))
		} else {
			bld.WriteString(" limit ")
			bld.WriteString(logger.IntToString(s.qu.Query.Limit))
		}
	}
	s.qu.QueryString = bld.String()
	bld.Reset()
}

func setOneIntOneBoolFields(v interface{}, value ...interface{}) {
	v.(*DbstaticOneIntOneBool).Num = value[0].(int)
	v.(*DbstaticOneIntOneBool).Bl = value[1].(bool)
	// value = nil
}

// requires 1 column - int
func QueryStaticColumnsOneIntOneBool(qu *Querywithargs, result *[]DbstaticOneIntOneBool) error {
	err := querySimpleScan(&searchdb{qu: qu}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func setOneIntFields(v interface{}, value ...interface{}) {
	v.(*DbstaticOneInt).Num = value[0].(int)
	// value = nil
}

// Select has to be in it
func QueryStaticColumnsOneUintQueryObject(table string, qu *Querywithargs, result *[]uint) error {
	s := searchdb{table: table, qu: qu, columns: qu.Query.Select}
	s.buildquery(false)

	err := querySimpleScan(&s, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

// requires 1 column - string
func QueryStaticStringArray(imdb bool, size int, qu *Querywithargs, result *[]string) error {
	err := querySimpleScan(&searchdb{
		qu:   qu,
		size: size,
		imdb: imdb,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

// requires 1 column - string
func QueryStaticIntArray(size int, qu *Querywithargs, result *[]int) error {
	err := querySimpleScan(&searchdb{
		qu:   qu,
		size: size,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func QueryStaticUintArray(size int, qu *Querywithargs, result *[]uint) error {
	err := querySimpleScan(&searchdb{
		qu:   qu,
		size: size,
	}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func setOneStringOneIntFields(v interface{}, value ...interface{}) {
	v.(*DbstaticOneStringOneInt).Str = value[0].(string)
	v.(*DbstaticOneStringOneInt).Num = value[1].(int)
	// value = nil
}

// requires 2 columns- string and int
func QueryStaticColumnsOneStringOneInt(imdb bool, size int, qu *Querywithargs, result *[]DbstaticOneStringOneInt) error {
	err := querySimpleScan(&searchdb{qu: qu, imdb: imdb, size: size}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func setTwoStringOneIntFields(v interface{}, value ...interface{}) {
	v.(*DbstaticTwoStringOneInt).Str1 = value[0].(string)
	v.(*DbstaticTwoStringOneInt).Str2 = value[1].(string)
	v.(*DbstaticTwoStringOneInt).Num = value[2].(int)
	// value = nil
}

// requires 2 columns- string and int
func QueryStaticColumnsTwoStringOneInt(imdb bool, size int, qu *Querywithargs, result *[]DbstaticTwoStringOneInt) error {
	err := querySimpleScan(&searchdb{qu: qu, imdb: imdb, size: size}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func setTwoIntFields(v interface{}, value ...interface{}) {
	v.(*DbstaticTwoInt).Num1 = value[0].(int)
	v.(*DbstaticTwoInt).Num2 = value[1].(int)
	// value = nil
}

// requires 2 columns- int and int
func QueryStaticColumnsTwoInt(qu *Querywithargs, result *[]DbstaticTwoInt) error {
	err := querySimpleScan(&searchdb{qu: qu}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func setThreeStringFields(v interface{}, value ...interface{}) {
	v.(*DbstaticThreeString).Str1 = value[0].(string)
	v.(*DbstaticThreeString).Str2 = value[1].(string)
	v.(*DbstaticThreeString).Str3 = value[2].(string)
	// value = nil
}

// requires 2 columns- int and int
func QueryStaticColumnsThreeString(imdb bool, qu *Querywithargs, result *[]DbstaticThreeString) error {
	err := querySimpleScan(&searchdb{qu: qu, imdb: imdb}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

func setTwoStringFields(v interface{}, value ...interface{}) {
	v.(*DbstaticTwoString).Str1 = value[0].(string)
	v.(*DbstaticTwoString).Str2 = value[1].(string)
	// value = nil
}

// requires 2 columns- int and int
func QueryStaticColumnsTwoString(imdb bool, count int, qu *Querywithargs, result *[]DbstaticTwoString) error {
	err := querySimpleScan(&searchdb{qu: qu, imdb: imdb, size: count}, result)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	return nil
}

// Uses column id
func CountRows(table string, qu *Querywithargs) (int, error) {
	var obj int
	qu.Query.Offset = 0
	qu.Query.Limit = 0
	qu.Query.Select = "count()"
	s := searchdb{table: table, qu: qu}
	s.buildquery(true)
	err := queryObject(&s, false, &obj)
	return obj, err
}

func CountRowsStaticNoError(qu *Querywithargs) int {
	var obj int
	if queryObject(&searchdb{qu: qu}, false, &obj) != nil {
		obj = 0
	}
	return obj
}

func QueryColumn[T any](qu *Querywithargs, obj *T) error {
	return queryObject(&searchdb{qu: qu}, false, obj)
}
func QueryImdbColumn[T any](qu *Querywithargs, obj *T) error {
	return queryObject(&searchdb{qu: qu, imdb: true}, false, obj)
}

func insertarrayprepare(table string, columns *logger.InStringArrayStruct) string {
	var cols, vals string
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
func InsertStatic(qu *Querywithargs) (sql.Result, error) {
	result, err := dbexec(qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", qu.QueryString), zap.Any("Values", qu.Args), zap.Error(err))
	}

	qu.Close()
	return result, err
}
func InsertNamedOpen(query string, obj interface{}) (sql.Result, error) {
	result, err := execsql(false, true, &Querywithargs{QueryString: query, Args: []interface{}{obj}})
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", query), zap.Any("values", obj), zap.Error(err))
	}

	return result, err
}
func InsertNamed(query string, obj interface{}) (sql.Result, error) {
	result, err := execsql(false, true, &Querywithargs{QueryString: query, Args: []interface{}{obj}})
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", query), zap.Any("values", obj), zap.Error(err))
	}
	obj = nil

	return result, err
}
func InsertArray(table string, columns *logger.InStringArrayStruct, values []interface{}) (sql.Result, error) {
	result, err := dbexec(&Querywithargs{Args: values, QueryString: insertarrayprepare(table, columns)})
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Table", table), zap.Any("Colums", columns), zap.Any("Values", values), zap.Error(err))
	}
	columns.Close()
	return result, err
}

func Dbexec(dbtype string, qu *Querywithargs) (sql.Result, error) {
	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query exec", zap.String("Query", qu.QueryString), zap.Any("args", qu.Args))
	}

	return execsql(dbtype == "imdb", false, qu)
}
func dbexec(qu *Querywithargs) (sql.Result, error) {
	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query exec", zap.String("Query", qu.QueryString), zap.Any("args", qu.Args))
	}

	return execsql(false, false, qu)
}
func updatearrayprepare(table string, columns *logger.InStringArrayStruct, qu *Querywithargs) string {
	var cols string
	for idx := range columns.Arr {
		if idx != 0 {
			cols += ","
		}
		cols += columns.Arr[idx] + " = ?"
	}
	if qu.Query.Where != "" {
		return "update " + table + " set " + cols + strWhere + qu.Query.Where
	}
	return "update " + table + " set " + cols
}
func UpdateArray(table string, columns *logger.InStringArrayStruct, values []interface{}, qu *Querywithargs) (sql.Result, error) {

	result, err := dbexec(&Querywithargs{Args: append(values, qu.Args...), QueryString: updatearrayprepare(table, columns, qu)})
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Table", table), zap.Any("Columns", columns), zap.Any("Values", values), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}
	columns.Close()
	qu.Close()
	return result, err
}
func UpdateNamed(query string, obj interface{}) (sql.Result, error) {
	result, err := execsql(false, true, &Querywithargs{QueryString: query, Args: []interface{}{obj}})
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Query", query), zap.Any("Values", obj), zap.Error(err))
	}

	return result, err
}

func updatecolprepare(table string, column string, qu *Query) string {
	if qu.Where != "" {
		return "update " + table + " set " + column + " = ?" + strWhere + qu.Where
	}
	return "update " + table + " set " + column + " = ?"
}
func UpdateColumn(table string, column string, value interface{}, qu *Querywithargs) (sql.Result, error) {

	result, err := dbexec(&Querywithargs{Args: append([]interface{}{value}, qu.Args...), QueryString: updatecolprepare(table, column, &qu.Query)})
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Table", table), zap.String("Column", column), zap.Any("Value", value), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}
	return result, err
}
func UpdateColumnStatic(qu *Querywithargs) error {
	_, err := dbexec(qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Update", zap.String("Query", qu.QueryString), zap.Any("Values", qu.Args), zap.Error(err))
	}
	return err
}

func DeleteRow(table string, qu *Querywithargs) (sql.Result, error) {
	if qu.Query.Where != "" {
		qu.QueryString = "delete from " + table + strWhere + qu.Query.Where
	} else {
		qu.QueryString = "delete from " + table
	}
	if DBLogLevel == "debug" {
		logger.Log.GlobalLogger.Debug("query count", zap.String("Query", qu.QueryString), zap.Any("args", qu.Args))
	}

	result, err := execsql(false, false, qu)

	if err != nil {
		logger.Log.GlobalLogger.Error("Delete", zap.String("Table", table), zap.String("Where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}
	return result, err
}
func DeleteRowStatic(qu *Querywithargs) error {
	_, err := dbexec(qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Insert", zap.String("Query", qu.QueryString), zap.Any("Values", qu.Args), zap.Error(err))
	}
	return err
}

func UpsertNamed(table string, columns *logger.InStringArrayStruct, obj interface{}, wherenamed string, qu *Querywithargs) (sql.Result, error) {
	var counter int

	counter, _ = CountRows(table, qu)
	var bld strings.Builder
	bld.Grow(200)
	if counter == 0 {
		bld.WriteString("Insert into " + table + " (")
		var bld2 strings.Builder
		bld2.Grow(200)
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
		bld2.Reset()
		bld.Reset()
		qu.Args = []interface{}{obj}
		result, err := execsql(false, true, qu)
		if err != nil {
			logger.Log.GlobalLogger.Error("Upsert-insert", zap.String("Table", table), zap.Any("Columns", columns), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
		}
		columns.Close()
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
	result, err := execsql(false, true, qu)
	if err != nil {
		logger.Log.GlobalLogger.Error("Upsert-update", zap.String("table", table), zap.Any("Columns", columns), zap.String("where", qu.Query.Where), zap.Any("Values", qu.Args), zap.Error(err))
	}
	columns.Close()
	return result, err
}

func ImdbCountRowsStatic(qu *Querywithargs) (int, error) {
	var obj int
	err := queryObject(&searchdb{qu: qu, imdb: true}, false, &obj)
	return obj, err
}

func DbQuickCheck() string {
	logger.Log.GlobalLogger.Info("Check Database for Errors 1")
	var str string
	queryObject(&searchdb{qu: &Querywithargs{QueryString: "PRAGMA quick_check;"}}, false, &str)
	logger.Log.GlobalLogger.Info("Check Database for Errors 2")
	return str
}

func DbIntegrityCheck() string {
	var str string
	queryObject(&searchdb{qu: &Querywithargs{QueryString: "PRAGMA integrity_check;"}}, false, &str)
	return str
}

package logger

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"

	"github.com/mozillazg/go-unidecode/table"
	"github.com/rs/zerolog"
)

type SynchedMap[t any] struct {
	mu sync.RWMutex
	m  map[string]t
}
type SynchedMapuint32[t any] struct {
	mu sync.RWMutex
	m  map[uint32]t
}

type addBuffer struct {
	bytes.Buffer
}

type mapmovieserie struct {
	Movie string
	Serie string
}

func NewSynchedMap[t any](size int) *SynchedMap[t] {
	return &SynchedMap[t]{
		mu: sync.RWMutex{},
		m:  make(map[string]t, size),
	}
}
func (s *SynchedMap[t]) Set(key string, value t) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}
func (s *SynchedMap[t]) Delete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}
func (s *SynchedMap[t]) Get(key string) t {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key]
}
func (s *SynchedMap[t]) Check(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.m[key]
	return ok
}
func (s *SynchedMap[t]) IterateMap(fn func(e t) bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for key := range s.m {
		if fn(s.m[key]) {
			return true
		}
	}
	return false
}
func (s *SynchedMap[t]) FuncMap(fn func(e t) bool) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found []string
	for key := range s.m {
		if fn(s.m[key]) {
			found = append(found, key)
		}
	}
	return found
}

func NewSynchedMapuint32[t any](size int) *SynchedMapuint32[t] {
	return &SynchedMapuint32[t]{
		mu: sync.RWMutex{},
		m:  make(map[uint32]t, size),
	}
}
func (s *SynchedMapuint32[t]) Set(key uint32, value t) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}
func (s *SynchedMapuint32[t]) Delete(key uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}
func (s *SynchedMapuint32[t]) Get(key uint32) t {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.m[key]
}
func (s *SynchedMapuint32[t]) GetMap() map[uint32]t {
	return s.m
}

func (s *SynchedMapuint32[t]) IterateMap(fn func(e t) bool) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for key := range s.m {
		if fn(s.m[key]) {
			return true
		}
	}
	return false
}
func (s *SynchedMapuint32[t]) FuncMap(fn func(e t) bool) []uint32 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var found []uint32
	for key := range s.m {
		if fn(s.m[key]) {
			found = append(found, key)
		}
	}
	return found
}
func (s *SynchedMapuint32[t]) Check(key uint32) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.m[key]
	return ok
}

const (
	ParseFailedIDs            = "parse failed ids"
	FilterByID                = "id = ?"
	StrRefreshMovies          = "Refresh Movies"
	StrRefreshMoviesInc       = "Refresh Movies Incremental"
	StrRefreshSeries          = "Refresh Series"
	StrRefreshSeriesInc       = "Refresh Series Incremental"
	StrDebug                  = "debug"
	StrDate                   = "date"
	StrSearchMissingInc       = "searchmissinginc"
	StrSearchMissingFull      = "searchmissingfull"
	StrSearchMissingIncTitle  = "searchmissinginctitle"
	StrSearchMissingFullTitle = "searchmissingfulltitle"
	StrSearchUpgradeInc       = "searchupgradeinc"
	StrSearchUpgradeFull      = "searchupgradefull"
	StrSearchUpgradeIncTitle  = "searchupgradeinctitle"
	StrSearchUpgradeFullTitle = "searchupgradefulltitle"
	StrStructure              = "structure"
	StrCheckMissing           = "checkmissing"
	StrCheckMissingFlag       = "checkmissingflag"
	StrUpgradeFlag            = "checkupgradeflag"
	StrReachedFlag            = "checkreachedflag"
	StrClearHistory           = "clearhistory"
	StrRssSeasonsAll          = "rssseasonsall"
	StrSerie                  = "serie"
	StrRssSeasons             = "rssseasons"
	StrRss                    = "rss"
	StrFeeds                  = "feeds"
	StrDataFull               = "datafull"
	Underscore                = "_"
	StrTt                     = "tt"
	CacheDBMedia              = "CacheDBMedia"
	DBCountDBMedia            = "DBCountDBMedia"
	DBCacheDBMedia            = "DBCacheDBMedia"
	CacheMedia                = "CacheMedia"
	DBCountMedia              = "DBCountMedia"
	DBCacheMedia              = "DBCacheMedia"
	CacheHistoryTitle         = "CacheHistoryTitle"
	CacheHistoryUrl           = "CacheHistoryUrl"
	DBHistoriesUrl            = "DBHistoriesUrl"
	DBHistoriesTitle          = "DBHistoriesTitle"
	DBCountHistoriesUrl       = "DBCountHistoriesUrl"
	DBCountHistoriesTitle     = "DBCountHistoriesTitle"
	CacheMediaTitles          = "CacheMediaTitles"
	DBCountDBTitles           = "DBCountDBTitles"
	DBCacheDBTitles           = "DBCacheDBTitles"
	CacheFiles                = "CacheFiles"
	DBCountFiles              = "DBCountFiles"
	DBCacheFiles              = "DBCacheFiles"
	CacheUnmatched            = "CacheUnmatched"
	DBCountUnmatched          = "DBCountUnmatched"
	DBCacheUnmatched          = "DBCacheUnmatched"
	DBCountFilesLocation      = "DBCountFilesLocation"
	DBCountUnmatchedPath      = "DBCountUnmatchedPath"
	DBCountDBTitlesDBID       = "DBCountDBTitlesDBID"
	DBDistinctDBTitlesDBID    = "DBDistinctDBTitlesDBID"
	DBMediaTitlesID           = "DBMediaTitlesID"
	DBFilesQuality            = "DBFilesQuality"
	DBCountFilesByList        = "DBCountFilesByList"
	DBLocationFilesByList     = "DBLocationFilesByList"
	DBIDsFilesByLocation      = "DBIDsFilesByLocation"
	DBCountFilesByMediaID     = "DBCountFilesByMediaID"
	DBCountFilesByLocation    = "DBCountFilesByLocation"
	TableFiles                = "TableFiles"
	TableMedia                = "TableMedia"
	DBCountMediaByList        = "DBCountMediaByList"
	DBIDMissingMediaByList    = "DBIDMissingMediaByList"
	DBUpdateMissing           = "DBUpdateMissing"
	DBListnameByMediaID       = "DBListnameByMediaID"
	DBRootPathFromMediaID     = "DBRootPathFromMediaID"
	DBDeleteFileByIDLocation  = "DBDeleteFileByIDLocation"
	DBCountHistoriesByTitle   = "DBCountHistoriesByTitle"
	DBCountHistoriesByUrl     = "DBCountHistoriesByUrl"
	DBLocationIDFilesByID     = "DBLocationIDFilesByID"
	DBFilePrioFilesByID       = "DBFilePrioFilesByID"
	UpdateMediaLastscan       = "UpdateMediaLastscan"
	DBQualityMediaByID        = "DBQualityMediaByID"
	SearchGenSelect           = "SearchGenSelect"
	SearchGenTable            = "SearchGenTable"
	SearchGenMissing          = "SearchGenMissing"
	SearchGenMissingEnd       = "SearchGenMissingEnd"
	SearchGenReached          = "SearchGenReached"
	SearchGenLastScan         = "SearchGenLastScan"
	SearchGenDate             = "SearchGenDate"
	SearchGenOrder            = "SearchGenOrder"
	CacheMovie                = "CacheMovie"
	CacheSeries               = "CacheSeries"
	CacheDBMovie              = "CacheDBMovie"
	CacheDBSeries             = "CacheDBSeries"
	CacheDBSeriesAlt          = "CacheDBSeriesAlt"
	CacheTitlesMovie          = "CacheTitlesMovie"
	CacheUnmatchedMovie       = "CacheUnmatchedMovie"
	CacheUnmatchedSeries      = "CacheUnmatchedSeries"
	CacheFilesMovie           = "CacheFilesMovie"
	CacheFilesSeries          = "CacheFilesSeries"
	CacheHistoryUrlMovie      = "CacheHistoryUrlMovie"
	CacheHistoryTitleMovie    = "CacheHistoryTitleMovie"
	CacheHistoryUrlSeries     = "CacheHistoryUrlSeries"
	CacheHistoryTitleSeries   = "CacheHistoryTitleSeries"
	DBIDUnmatchedPathList     = "DBIDUnmatchedPathList"
	DBMovieDetails            = "select id,created_at,updated_at,title,release_date,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?"
)

type arrany struct {
	Arr []any
}

// global vars
var (
	V0                       = 0
	Strstructure             = "structure"
	StrMovie                 = "movie"
	StrSeries                = "series"
	StrID                    = "id"
	StrWaitfor               = "waitfor"
	StrURL                   = "Url"
	StrStatus                = "status"
	StrImdb                  = "imdb"
	StrFound                 = "found"
	StrWanted                = "wanted"
	StrTitle                 = "Title"
	StrAccepted              = "accepted"
	StrDenied                = "denied"
	StrRow                   = "row"
	StrDot                   = "."
	StrDash                  = "-"
	StrSpace                 = " "
	StrIndexer               = "indexer"
	StrJob                   = "Job"
	StrTvdb                  = "tvdb"
	StrSeason                = "season"
	StrListname              = "Listname"
	StrPath                  = "Path"
	StrFile                  = "File"
	StrSize                  = "Size"
	StrConfig                = "config"
	StrReason                = "reason"
	StrPriority              = "Priority"
	StrData                  = "data"
	ErrNoID                  = errors.New("no id")
	ErrNotFound              = errors.New("not found")
	ErrNotAllowed            = errors.New("not allowed")
	ErrDisabled              = errors.New("disabled")
	ErrToWait                = errors.New("please wait")
	ErrContextCanceled       = errors.New("context canceled")
	Errnoresults             = errors.New("no results")
	ErrNotFoundDbmovie       = errors.New("dbmovie not found")
	ErrNotFoundMovie         = errors.New("movie not found")
	ErrNotFoundDbserie       = errors.New("dbserie not found")
	ErrCfgpNotFound          = errors.New("cfgpstr not found")
	ErrNotFoundEpisode       = errors.New("episode not found")
	ErrListnameEmpty         = errors.New("listname empty")
	ErrListnameTemplateEmpty = errors.New("listname template empty")
	ErrTvdbEmpty             = errors.New("tvdb empty")
	ErrImdbEmpty             = errors.New("imdb empty")
	ErrTracksEmpty           = errors.New("tracks empty")
	PlAddBuffer              = pool.NewPool(100, 10, func(b *addBuffer) {
		b.Grow(2000)
	}, func(b *addBuffer) {
		b.Reset()
	})
	PlBuffer = pool.NewPool(100, 10, func(b *bytes.Buffer) {
		b.Grow(2000)
	}, func(b *bytes.Buffer) {
		b.Reset()
	})

	PLArrAny = pool.NewPool[arrany](100, 10, func(a *arrany) { *&a.Arr = make([]any, 0, 20) }, func(a *arrany) {
		clear(a.Arr)
		a.Arr = a.Arr[:0]
	})

	mapSrings = map[string]mapmovieserie{
		CacheDBMedia: {
			Movie: CacheDBMovie,
			Serie: CacheDBSeries},
		DBCountDBMedia: {
			Movie: "select count() from dbmovies",
			Serie: "select count() from dbseries"},
		DBCacheDBMedia: {
			Movie: "select title, slug, imdb_id, year, id from dbmovies",
			Serie: "select seriename, slug, id from dbseries"},
		CacheMedia: {
			Movie: CacheMovie,
			Serie: CacheSeries},
		DBCountMedia: {
			Movie: "select count() from movies",
			Serie: "select count() from series"},
		DBCacheMedia: {
			Movie: "select lower(listname), dbmovie_id, id from movies",
			Serie: "select lower(listname), dbserie_id, id from series"},
		CacheHistoryTitle: {
			Movie: CacheHistoryTitleMovie,
			Serie: CacheHistoryTitleSeries},
		CacheHistoryUrl: {
			Movie: CacheHistoryUrlMovie,
			Serie: CacheHistoryUrlSeries},
		DBHistoriesUrl: {
			Movie: "select distinct url from movie_histories",
			Serie: "select distinct url from serie_episode_histories"},
		DBHistoriesTitle: {
			Movie: "select distinct title from movie_histories",
			Serie: "select distinct title from serie_episode_histories"},
		DBCountHistoriesUrl: {
			Movie: "select count() from (select distinct url from movie_histories)",
			Serie: "select count() from (select distinct url from serie_episode_histories)"},
		DBCountHistoriesTitle: {
			Movie: "select count() from (select distinct title from movie_histories)",
			Serie: "select count() from (select distinct title from serie_episode_histories)"},
		CacheMediaTitles: {
			Movie: CacheTitlesMovie,
			Serie: CacheDBSeriesAlt},
		DBCountDBTitles: {
			Movie: "select count() from dbmovie_titles",
			Serie: "select count() from dbserie_alternates"},
		DBCacheDBTitles: {
			Movie: "select title, slug, dbmovie_id from dbmovie_titles",
			Serie: "select title, slug, dbserie_id from dbserie_alternates"},
		CacheFiles: {
			Movie: CacheFilesMovie,
			Serie: CacheFilesSeries},
		DBCountFiles: {
			Movie: "select count() from movie_files",
			Serie: "select count() from serie_episode_files"},
		DBCacheFiles: {
			Movie: "select location from movie_files",
			Serie: "select location from serie_episode_files"},
		CacheUnmatched: {
			Movie: CacheUnmatchedMovie,
			Serie: CacheUnmatchedSeries},
		DBCountUnmatched: {
			Movie: "select count() from movie_file_unmatcheds where (last_checked > datetime('now',?) or last_checked is null)",
			Serie: "select count() from serie_file_unmatcheds where (last_checked > datetime('now',?) or last_checked is null)"},
		"DBRemoveUnmatched": {
			Movie: "delete from movie_file_unmatcheds where (last_checked < datetime('now',?) and last_checked is not null)",
			Serie: "delete from serie_file_unmatcheds where (last_checked < datetime('now',?) and last_checked is not null)"},
		DBCacheUnmatched: {
			Movie: "select filepath from movie_file_unmatcheds where (last_checked > datetime('now',?) or last_checked is null)",
			Serie: "select filepath from serie_file_unmatcheds where (last_checked > datetime('now',?) or last_checked is null)"},
		DBCountFilesLocation: {
			Movie: "select count() from movie_files where location = ?",
			Serie: "select count() from serie_episode_files where location = ?"},
		DBCountUnmatchedPath: {
			Movie: "select count() from movie_file_unmatcheds where filepath = ?",
			Serie: "select count() from serie_file_unmatcheds where filepath = ?"},
		DBCountDBTitlesDBID: {
			Movie: "select count() from (select distinct title, slug from dbmovie_titles where dbmovie_id = ? and title != '')",
			Serie: "select count() from (select distinct title, slug from dbserie_alternates where dbserie_id = ? and title != '')"},
		DBDistinctDBTitlesDBID: {
			Movie: "select distinct title, slug, 0 from dbmovie_titles where dbmovie_id = ? and title != ''",
			Serie: "select distinct title, slug, 0 from dbserie_alternates where dbserie_id = ? and title != ''"},
		DBMediaTitlesID: {
			Movie: "select year, title, slug from dbmovies where id = ?",
			Serie: "select 0, seriename, slug from dbseries where id = ?"},
		DBFilesQuality: {
			Movie: "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?",
			Serie: "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?"},
		DBCountFilesByList: {
			Movie: "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
			Serie: "select count() from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)"},
		DBLocationFilesByList: {
			Movie: "select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
			Serie: "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)"},
		DBIDsFilesByLocation: {
			Movie: "select id, movie_id from movie_files where location = ?",
			Serie: "select id, serie_episode_id from serie_episode_files where location = ?"},
		DBCountFilesByMediaID: {
			Movie: "select count() from movie_files where movie_id = ?",
			Serie: "select count() from serie_episode_files where serie_episode_id = ?"},
		DBCountFilesByLocation: {
			Movie: "select count() from movie_files where location = ?",
			Serie: "select count() from serie_episode_files where location = ?"},
		TableFiles: {
			Movie: "movie_files",
			Serie: "serie_episode_files"},
		TableMedia: {
			Movie: "movies",
			Serie: "serie_episodes"},
		DBCountMediaByList: {
			Movie: "select count() from movies where listname = ? COLLATE NOCASE",
			Serie: "select count() from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)"},
		DBIDMissingMediaByList: {
			Movie: "select id,missing from movies where listname = ? COLLATE NOCASE",
			Serie: "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)"},
		DBUpdateMissing: {
			Movie: "update movies set missing = ? where id = ?",
			Serie: "update serie_episodes set missing = ? where id = ?"},
		DBListnameByMediaID: {
			Movie: "select listname from movies where id = ?",
			Serie: "select listname from series where id = ?"},
		DBRootPathFromMediaID: {
			Movie: "select rootpath from movies where id = ?",
			Serie: "select rootpath from series where id = ?"},
		DBDeleteFileByIDLocation: {
			Movie: "delete from movie_files where movie_id = ? and location = ?",
			Serie: "delete from serie_episode_files where serie_id = ? and location = ?"},
		DBCountHistoriesByTitle: {
			Movie: "select count() from movie_histories where title = ?",
			Serie: "select count() from serie_episode_histories where title = ?"},
		DBCountHistoriesByUrl: {
			Movie: "select count() from movie_histories where url = ?",
			Serie: "select count() from serie_episode_histories where url = ?"},
		DBLocationIDFilesByID: {
			Movie: "select location, id from movie_files where movie_id = ?",
			Serie: "select location, id from serie_episode_files where serie_episode_id = ?"},
		DBFilePrioFilesByID: {
			Movie: "select location, movie_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from movie_files where movie_id = ?",
			Serie: "select location, serie_episode_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from serie_episode_files where serie_episode_id = ?"},
		UpdateMediaLastscan: {
			Movie: "update movies set lastscan = datetime('now','localtime') where id = ?",
			Serie: "update serie_episodes set lastscan = datetime('now','localtime') where id = ?"},
		DBQualityMediaByID: {
			Movie: "select quality_profile from movies where id = ?",
			Serie: "select quality_profile from serie_episodes where id = ?"},
		SearchGenSelect: {
			Movie: "select movies.quality_profile, movies.id ",
			Serie: "select serie_episodes.quality_profile, serie_episodes.id "},
		SearchGenTable: {
			Movie: " from movies inner join dbmovies on dbmovies.id=movies.dbmovie_id where ",
			Serie: " from serie_episodes inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id where "},
		SearchGenMissing: {
			Movie: "dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?",
			Serie: "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?"},
		SearchGenMissingEnd: {
			Movie: ")",
			Serie: ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)"},
		SearchGenReached: {
			Movie: "dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
			Serie: "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?"},
		SearchGenLastScan: {
			Movie: " and (movies.lastscan is null or movies.Lastscan < ?)",
			Serie: " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)"},
		SearchGenDate: {
			Movie: " and (dbmovies.release_date < ? or dbmovies.release_date is null)",
			Serie: " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)"},
		SearchGenOrder: {
			Movie: " order by movies.Lastscan asc",
			Serie: " order by serie_episodes.Lastscan asc"},
		DBIDUnmatchedPathList: {
			Movie: "select id from movie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
			Serie: "select id from serie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE"},
		"InsertUnmatched": {
			Movie: "Insert into movie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
			Serie: "Insert into serie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))"},
		"UpdateUnmatched": {
			Movie: "update movie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
			Serie: "update serie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?"},
	}
)

// local vars
var (
	timeFormat = time.RFC3339Nano
	log        zerolog.Logger
	timeZone   = *time.UTC
	textparser = template.New("master")
	subRune    = map[rune]struct{}{
		'a': {},
		'b': {},
		'c': {},
		'd': {},
		'e': {},
		'f': {},
		'g': {},
		'h': {},
		'i': {},
		'j': {},
		'k': {},
		'l': {},
		'm': {},
		'n': {},
		'o': {},
		'p': {},
		'q': {},
		'r': {},
		's': {},
		't': {},
		'u': {},
		'v': {},
		'w': {},
		'x': {},
		'y': {},
		'z': {},
		'0': {},
		'1': {},
		'2': {},
		'3': {},
		'4': {},
		'5': {},
		'6': {},
		'7': {},
		'8': {},
		'9': {},
		'-': {},
	}
	substituteRuneSpace = map[rune]string{
		'&':  "and",
		'@':  "at",
		'"':  "",
		'\'': "",
		'’':  "",
		'_':  "",
		' ':  "-",
		'‒':  "-",
		'–':  "-",
		'—':  "-",
		'―':  "-",
		'ä':  "ae",
		'ö':  "oe",
		'ü':  "ue",
		'Ä':  "Ae",
		'Ö':  "Oe",
		'Ü':  "Ue",
		'ß':  "ss",
	}
	diacriticsmap = map[rune]string{
		'ä': "ae",
		'ö': "oe",
		'ü': "ue",
		'Ä': "Ae",
		'Ö': "Oe",
		'Ü': "Ue",
		'ß': "ss",
	}
	pathmap = map[rune]struct{}{
		':':  {},
		'*':  {},
		'?':  {},
		'\\': {},
		'<':  {},
		'>':  {},
		'|':  {},
	}
)

// IntToString converts any numeric type to a string.
// It handles all integer types, including signed and unsigned.
func IntToString(a any) string {
	var i int
	switch tt := a.(type) {
	case uint:
		i = int(tt)
	case uint8:
		i = int(tt)
	case uint16:
		i = int(tt)
	case uint32:
		i = int(tt)
	case uint64:
		i = int(tt)
	case int8:
		i = int(tt)
	case int16:
		i = int(tt)
	case int32:
		i = int(tt)
	case int64:
		i = int(tt)
	case int:
		i = tt
	}
	if i == 0 {
		return "0"
	}
	if i == 1 {
		return "1"
	}
	return strconv.Itoa(i)
}

func AnyToString(a any) string {
	return fmt.Sprintf("%v", a)
}

// WriteInt writes the string representation of the given integer i to the buffer.
func (b *addBuffer) WriteInt(i int) {
	b.WriteString(strconv.Itoa(i))
}

// WriteInt8 writes the given int8 value to the buffer as a string.
func (b *addBuffer) WriteInt8(i int8) {
	b.WriteString(IntToString(i))
}

// WriteUInt16 writes the given uint16 value to the buffer as a string.
func (b *addBuffer) WriteUInt16(i uint16) {
	b.WriteString(IntToString(i))
}

// WriteUInt writes the string representation of the given unsigned integer to the buffer.
func (b *addBuffer) WriteUInt(i uint) {
	b.WriteString(IntToString(i))
}

// WriteStringMap writes a string to the buffer based on the provided boolean and string parameters.
// If useseries is true, it writes the value from the Mapstringsseries map using the typestr key.
// If useseries is false, it writes the value from the Mapstringsmovies map using the typestr key.
func (b *addBuffer) WriteStringMap(useseries bool, typestr string) {
	b.WriteString(GetStringsMap(useseries, typestr))
}

// WriteUrl writes the provided string to the underlying buffer, first escaping it
// using url.QueryEscape. This is a convenience method for writing URL-encoded
// strings to the buffer.
func (b *addBuffer) WriteUrl(s string) {
	if s == "" {
		return
	}
	b.WriteString(url.QueryEscape(s))
}

// GetTimeZone returns a pointer to the time.Location representing the
// timezone used for formatting logs. This allows checking the current
// timezone.
func GetTimeZone() *time.Location {
	return &timeZone
}

// GetTimeFormat returns the time format string used for formatting logs.
// This allows checking the current time format.
func GetTimeFormat() string {
	return timeFormat
}

// ParseStringTemplate parses a text/template string into a template.Template, caches it, and executes it with the given data.
// It returns the executed template string and any error encountered.
func ParseStringTemplate(message string, messagedata any) (bool, string) {
	if message == "" {
		return false, ""
	}
	tmplmessage := textparser.Lookup(message)
	if tmplmessage == nil {
		var err error
		tmplmessage, err = textparser.New(message).Parse(message)
		if err != nil {
			LogDynamicany("error", "template", err)
			return true, ""
		}
	}
	doc := PlBuffer.Get()
	defer PlBuffer.Put(doc)
	if err := tmplmessage.Execute(doc, messagedata); err != nil {
		LogDynamicany("error", "template", err)
		return true, ""
	}
	return false, doc.String()
}

//go:noinline
func StringToSlug(instr string) string {
	return string(StringToSlugBytes(instr))
}

//go:noinline
func BytesToString2(instr []byte) string {
	if len(instr) == 0 {
		return ""
	}
	return string(instr)
}

func StringToByteArr(instr string) []byte {
	if instr == "" {
		return nil
	}
	ret := PlBuffer.Get()
	defer PlBuffer.Put(ret)
	ret.WriteString(instr)
	return ret.Bytes()
}

// StringToSlug converts the given string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens.
func StringToSlugBytes(instr string) []byte {
	if instr == "" {
		return nil
	}
	ret := PlBuffer.Get()
	defer PlBuffer.Put(ret)
	var laststr string
	var lastrune rune
	//var c byte
	if strings.ContainsRune(instr, '&') {
		instr = html.UnescapeString(instr)
	}
	//ret.Grow(len(instr) + 10)
	var section rune
	var position rune
	for _, r := range instr {
		if val, ok := substituteRuneSpace[r]; ok {
			if laststr != "" && val == laststr {
				continue
			}
			if lastrune == '-' && val == StrDash {
				continue
			}
			ret.WriteString(val)
			laststr = val
			if val == StrDash {
				lastrune = '-'
			} else {
				lastrune = ' '
			}
			continue
		}
		if laststr != "" {
			laststr = ""
		}

		if r < unicode.MaxASCII {
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			if _, ok := subRune[r]; !ok {
				if lastrune == '-' {
					continue
				}
				lastrune = '-'
				ret.WriteRune('-')
			} else {
				if lastrune == '-' && r == '-' {
					continue
				}
				lastrune = r
				ret.WriteRune(r)
			}
			continue
		}
		if r > 0xeffff {
			continue
		}

		section = r >> 8   // Chop off the last two hex digits
		position = r % 256 // Last two hex digits
		if tb, ok := table.Tables[section]; ok {
			if len(tb) > int(position) {
				if len(tb[position]) >= 1 {
					if tb[position][0] > unicode.MaxASCII && lastrune != '-' {
						lastrune = '-'
						ret.WriteRune('-')
						continue
					}
				}
				if lastrune == '-' && tb[position] == StrDash {
					continue
				}
				ret.WriteString(tb[position])
			}
		}
	}

	return bytes.TrimLeft(bytes.TrimRight(ret.Bytes(), "- "), "- ")
}

// AddImdbPrefix prepends the imdb prefix if it doesn't already exist.
func AddImdbPrefix(str string) string {
	AddImdbPrefixP(&str)
	return str
}

// AddImdbPrefixP adds the "tt" prefix to the given string if it doesn't already have the prefix.
// If the string is nil or has a length less than 1, this function does nothing.
func AddImdbPrefixP(str *string) {
	if str != nil && len(*str) >= 1 && !HasPrefixI(*str, StrTt) {
		*str = JoinStrings(StrTt, *str) //JoinStrings
	}
}

// AddTvdbPrefix prepends the tvdb prefix if it doesn't already exist.
func AddTvdbPrefix(str string) string {
	if len(str) >= 1 && !HasPrefixI(str, StrTvdb) {
		//return JoinStrings(StrTvdb, str)
		return JoinStrings(StrTvdb, str)
	}
	return str
}

// Path sanitizes the given string by cleaning it with path.Clean, unquoting
// and unescaping it, optionally removing slashes, and replacing invalid
// characters. It returns the cleaned path string.
func Path(s string, allowslash bool) string {
	if s == "" {
		return ""
	}
	s = path.Clean(UnquoteUnescape(s))
	if !allowslash {
		s = StringRemoveAllRunesMulti(s, '\\', '/')
		//s = StringRemoveAllRunes(s, '\\')
		//s = StringRemoveAllRunes(s, '/')
	}
	bld := PlBuffer.Get()
	defer PlBuffer.Put(bld)
	//bld.Grow(len(s))
	var bl bool
	for _, z := range s {
		if _, ok := pathmap[z]; !ok {
			bld.WriteRune(z)
		} else {
			bl = true
		}
	}
	if bl {
		s = bld.String()
	}

	if s == "" {
		return ""
	}
	if s != "" && (s[:1] == StrSpace || s[len(s)-1:] == StrSpace) {
		return strings.TrimSpace(s)
	}
	return s
}

// ContainsI checks if string a contains string b, ignoring case.
// It first checks for a direct match with strings.Contains.
// If not found, it does a case-insensitive search by looping through a
// and comparing substrings with EqualFold.
func ContainsI(a string, b string) bool {
	if strings.Contains(a, b) {
		return true
	}
	if len(a) < len(b) {
		return false
	}
	lb := len(b)
	for i := 0; i < len(a)-lb+1; i++ {
		if strings.EqualFold(a[i:i+lb], b) {
			return true
		}
	}
	return false
}
func ContainsByteI(a []byte, b []byte) bool {
	if bytes.Contains(a, b) {
		return true
	}
	if len(a) < len(b) {
		return false
	}
	lb := len(b)
	for i := 0; i < len(a)-lb+1; i++ {
		if bytes.EqualFold(a[i:i+lb], b) {
			return true
		}
	}
	return false
}

// ContainsInt checks if the string a contains the string representation of
// the integer b. It converts b to a string using strconv.Itoa and calls
// strings.Contains to check for a match.
func ContainsInt(a string, b uint16) bool {
	return strings.Contains(a, IntToString(b))
}

// HasPrefixI checks if string s starts with prefix, ignoring case.
// It first checks for a direct match with strings.HasPrefix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from 0 to len(prefix) with prefix using EqualFold.
func HasPrefixI(s, prefix string) bool {
	return len(s) >= len(prefix) && (s[0:len(prefix)] == prefix || strings.EqualFold(s[0:len(prefix)], prefix))
}

// HasSuffixI checks if string s ends with suffix, ignoring case.
// It first checks for a direct match with strings.HasSuffix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from len(s)-len(suffix) to len(s) with suffix
// using EqualFold.
func HasSuffixI(s, suffix string) bool {
	return len(s) >= len(suffix) && (s[len(s)-len(suffix):] == suffix || strings.EqualFold(s[len(s)-len(suffix):], suffix))
}

// JoinStrings concatenates any number of strings together.
// It is optimized to avoid unnecessary allocations when there are few elements.
//
//go:noinline
func JoinStrings(elems ...string) string {
	if len(elems) == 1 {
		return elems[0]
	}
	return string(joinStringsByte(elems))
}

//go:noinline
func JoinStringsByte(elems ...string) []byte {
	return joinStringsByte(elems)
}

// JoinStringsByte concatenates the given string elements into a single byte slice.
// If the input slice is empty, it returns nil. Otherwise, it uses a buffer to
// efficiently concatenate the strings and returns the resulting byte slice.
func joinStringsByte(elems []string) []byte {
	if len(elems) == 0 {
		return nil
	}

	b := PlBuffer.Get()
	defer PlBuffer.Put(b)
	//b.Grow(Getstringarrlength(elems))
	for idx := range elems {
		if elems[idx] != "" {
			b.WriteString(elems[idx])
		}
	}
	return b.Bytes()
}

func Getstringarrlength(elems []string) int {
	var l int
	for idx := range elems {
		if elems[idx] != "" {
			l += len(elems[idx])
		}
	}
	return l
}

// IndexI searches for the first case-insensitive instance of b in a.
// It returns the index of the first match, or -1 if no match is found.
func IndexI(a string, b string) int {
	i := strings.Index(a, b)
	if i != -1 {
		return i
	}

	if len(b) > len(a) {
		return -1
	}

	lb := len(b)
	for i := 0; i < len(a)-lb+1; i++ {
		if strings.EqualFold(a[i:i+lb], b) {
			return i
		}
	}

	return -1
}

// IndexILast searches for the last case-insensitive instance of b in a.
// It returns the index of the last match, or -1 if no match is found.
func IndexILast(a string, b string) int {
	lb := len(b)
	for i := len(a) - 1; i >= 0; i-- {
		if a[i:i+lb] == b || strings.EqualFold(a[i:i+lb], b) {
			return i
		}
	}
	return -1
}

// StringToFileMode converts a string representing a file mode in octal to a uint32.
// It returns 0 if the string is empty or cannot be parsed.
func StringToFileMode(s string) fs.FileMode {
	if s == "" {
		return 0
	}
	in, err := strconv.ParseUint(s, 8, 0)
	if err != nil {
		return 0
	}
	return fs.FileMode(uint32(in))
}

// StringToInt converts the given string to an int.
// It uses stringToUint64 to convert to a uint64 first,
// then converts the result to an int.
func StringToInt(s string) int {
	if s == "" || s == "0" {
		return 0
	}
	if strings.ContainsRune(s, '.') || strings.ContainsRune(s, ',') {
		in, err := strconv.ParseFloat(StringReplaceWith(s, ',', '.'), 64)
		if err != nil {
			return 0
		}
		return int(in)
	}
	in, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return in
}

// StringToUInt16 converts a string to a uint16 value.
// It does this by first converting the string to an int64 using StringToInt,
// and then casting the result to a uint16.
func StringToUInt16(s string) uint16 {
	if s == "" || s == "0" {
		return 0
	}
	return uint16(StringToInt(s))
}

// StringToInt64 converts the given string to an int64.
// It uses stringToUint64 to convert to a uint64 first,
// then converts the result to an int64.
func StringToInt64(s string) int64 {
	if s == "" || s == "0" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return int64(StringToInt(s))
	}
	return i
}

// TimeGetNow returns the current time in the time zone specified by the
// global timeZone variable.
func TimeGetNow() time.Time {
	return time.Now().In(&timeZone)
}

// UnquoteUnescape unquotes a quoted string and unescapes HTML entities.
// It first tries to unquote the string as a quoted string literal.
// If that succeeds and the unquoted string contains HTML entities,
// it unescapes the HTML entities.
// If unquoting fails, it just unescapes any HTML entities in the original string.
func UnquoteUnescape(s string) string {
	if strings.Contains(s, "\\u") {
		if u, err := strconv.Unquote(s); err == nil {
			s = u
		}
	}
	if strings.ContainsRune(s, '&') {
		s = html.UnescapeString(s)
	}
	return s
}

// SplitByLR splits str into left and right substrings by the last occurrence of splitby byte.
// It returns the left substring before the split byte and the right substring after.
// If splitby byte is not found or invalid, an empty string and the original str are returned.
func SplitByLR(str string, splitby byte) (string, string) { // left, right
	idx := strings.LastIndexByte(str, splitby)
	if idx == -1 || idx == 0 || idx == len(str) {
		return "", str
	}
	return str[:idx], str[idx+1:]
}

// Contains reports whether v is present in s - case insensitive.
func SlicesContainsI(s []string, v string) bool {
	for idx := range s {
		if v == s[idx] || strings.EqualFold(v, s[idx]) {
			return true
		}
	}
	return false
}

// Contains reports whether v is contained in s - case insensitive.
func SlicesContainsPart2I(s []string, v string) bool {
	for idx := range s {
		if ContainsI(v, s[idx]) {
			return true
		}
	}
	return false
}

// Contains reports whether v is contained in s.
func Contains[S ~[]E, E comparable](s S, v E) bool {
	for idx := range s {
		if v == s[idx] {
			return true
		}
	}
	return false
}

// StringRemoveAllRunes removes all occurrences of the rune r from s.
func StringRemoveAllRunes(s string, r rune) string {
	StringRemoveAllRunesP(&s, r)
	return s
}

// StringRemoveAllRunesP removes all occurrences of the rune r from the string pointed to by s.
// It modifies the string in-place by creating a new buffer, writing all runes except r to the buffer,
// and then updating the original string with the buffer's contents.
func StringRemoveAllRunesP(s *string, r rune) {
	if !strings.ContainsRune(*s, r) {
		return
	}

	out := PlBuffer.Get()
	defer PlBuffer.Put(out)
	//out.Grow(len(*s))
	for _, z := range *s {
		if r != z {
			out.WriteRune(z)
		}
	}
	*s = out.String()
}
func BytesRemoveAllRunesP(s []byte, r rune) []byte {
	if !bytes.ContainsRune(s, r) {
		return s
	}

	out := s[:0]
	for _, z := range s {
		if byte(r) != z {
			out = append(out, z)
		}
	}
	return out
}

// StringRemoveAllRunes removes all occurrences of the rune r from s.
func StringRemoveAllRunesMulti(s string, r ...rune) string {
	out := PlBuffer.Get()
	defer PlBuffer.Put(out)
	//out.Grow(len(s))
	var bl bool
contloop:
	for _, z := range s {
		for _, y := range r {
			if y == z {
				bl = true
				continue contloop
			}
		}
		out.WriteRune(z)
	}
	if !bl {
		return s
	}
	return out.String()
}

// StringReplaceWith replaces all occurrences of the rune r in s with the rune t.
// It returns a new string with the replacements.
func StringReplaceWith(s string, r rune, t rune) string {
	StringReplaceWithP(&s, r, t)
	return s
}

// StringReplaceWithP replaces all occurrences of the rune r in the string s with the rune t.
// It modifies the input string s in-place.
func StringReplaceWithP(s *string, r rune, t rune) {
	if !strings.ContainsRune(*s, r) {
		return
	}
	buf := PlBuffer.Get()
	defer PlBuffer.Put(buf)
	var bl bool
	for _, z := range *s {
		if z == r {
			buf.WriteRune(t)
			bl = true
		} else {
			buf.WriteRune(z)
		}
	}
	if bl {
		*s = buf.String()
	}
}

// StringReplaceWith replaces all occurrences of the rune r in s with the rune t.
// It returns a new string with the replacements.
func StringReplaceWithDual(s string, r1 rune, t1 rune, r2 rune, t2 rune) string {
	if !strings.ContainsRune(s, r1) && !strings.ContainsRune(s, r2) {
		return s
	}
	buf := PlBuffer.Get()
	defer PlBuffer.Put(buf)
	//buf.Grow(len(s))
	for _, z := range s {
		if z == r1 {
			buf.WriteRune(t1)
		} else if z == r2 {
			buf.WriteRune(t2)
		} else {
			buf.WriteRune(z)
		}
	}
	return buf.String()
}

// StringReplaceWith replaces all occurrences of the rune r in s with the rune t.
// It returns a new string with the replacements.
func ByteReplaceWithByte(s []byte, r rune, t rune) []byte {
	if !bytes.ContainsRune(s, r) {
		return s
	}
	buf := PlBuffer.Get()
	defer PlBuffer.Put(buf)
	//buf.Grow(len(s))
	for _, z := range s {
		if rune(z) == r {
			buf.WriteRune(t)
		} else {
			buf.WriteByte(z)
		}
	}
	return buf.Bytes()
}

// GetStringsMap returns the map of strings for the given type based on
// whether to use the series or movies map. If useseries is true, it returns
// the mapstringsseries map, otherwise it returns the mapstringsmovies map.
func GetStringsMap(useseries bool, typestr string) string {
	if useseries {
		return mapSrings[typestr].Serie
	}
	return mapSrings[typestr].Movie
}

// diacriticsReplacer replaces diacritic marks in the input string s
// with their ASCII equivalents, based on the provided diacriticsmap.
// It calls the replacer function to perform the replacements.
func DiacriticsReplacer(s string) string {
	bld := PlBuffer.Get()
	defer PlBuffer.Put(bld)
	//bld.Grow(len(s) + 2)
	var bl bool
	for _, z := range s {
		if r, ok := diacriticsmap[z]; ok {
			bld.WriteString(r)
			bl = true
		} else {
			bld.WriteRune(z)
		}
	}
	if !bl {
		return s
	}
	return bld.String()
	//return replacer(diacriticsmap, s)
}

// CheckContextEnded checks if the provided context has been canceled or has expired.
// If the context has been canceled or has expired, it returns the context's error.
// Otherwise, it returns nil.
func CheckContextEnded(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// Abort / return early
		return ctx.Err()
	default:
	}
	return nil
}

func HandlePanic() {
	// detect if panic occurs or not
	a := recover()

	if a != nil {
		LogDynamicany("error", "Recovered from panic", "RECOVER", Stack(), "vap", a)
	}
}

func Stack() string {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			return string(buf[:n])
		}
		buf = make([]byte, 2*len(buf))
	}
}

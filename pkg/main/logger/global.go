package logger

import (
	"bytes"
	"context"
	"errors"
	"html"
	"io/fs"
	"net/url"
	"path"
	"runtime"
	"strconv"
	"strings"
	"text/template"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"

	"github.com/mozillazg/go-unidecode/table"
	"github.com/rs/zerolog"
)

type AddBuffer struct {
	bytes.Buffer
}

type mapmovieserie struct {
	Movie string
	Serie string
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
	StrCheckMissing           = "checkmissing"
	StrCheckMissingFlag       = "checkmissingflag"
	StrUpgradeFlag            = "checkupgradeflag"
	StrReachedFlag            = "checkreachedflag"
	StrClearHistory           = "clearhistory"
	StrRssSeasonsAll          = "rssseasonsall"
	StrSerie                  = "serie"
	StrRssSeasons             = "rssseasons"
	StrRss                    = "rss"
	Underscore                = "_"
	StrTt                     = "tt"
	CacheDBMedia              = "CacheDBMedia"
	DBCountDBMedia            = "DBCountDBMedia"
	DBCacheDBMedia            = "DBCacheDBMedia"
	CacheMedia                = "CacheMedia"
	DBCountMedia              = "DBCountMedia"
	DBCacheMedia              = "DBCacheMedia"
	CacheHistoryTitle         = "CacheHistoryTitle"
	CacheHistoryURL           = "CacheHistoryUrl"
	DBHistoriesURL            = "DBHistoriesUrl"
	DBHistoriesTitle          = "DBHistoriesTitle"
	DBCountHistoriesURL       = "DBCountHistoriesUrl"
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
	DBCountHistoriesByURL     = "DBCountHistoriesByUrl"
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
	CacheHistoryURLMovie      = "CacheHistoryUrlMovie"
	CacheHistoryTitleMovie    = "CacheHistoryTitleMovie"
	CacheHistoryURLSeries     = "CacheHistoryUrlSeries"
	CacheHistoryTitleSeries   = "CacheHistoryTitleSeries"
	DBIDUnmatchedPathList     = "DBIDUnmatchedPathList"
	DBMovieDetails            = "select id,created_at,updated_at,title,year,adult,budget,genres,original_language,original_title,overview,popularity,revenue,runtime,spoken_languages,status,tagline,vote_average,vote_count,moviedb_id,imdb_id,freebase_m_id,freebase_id,facebook_id,instagram_id,twitter_id,url,backdrop,poster,slug,trakt_id from dbmovies where id = ?"

	Strstructure = "structure"
	StrStatus    = "status"
	StrRow       = "row"
	StrDot       = "."
	StrDash      = "-"
	StrSpace     = " "
	StrIndexer   = "indexer"
	StrSize      = "Size"
)

type Arrany struct {
	Arr []any
}

// global vars.
var (
	StrFeeds       = "feeds"
	StrDataFull    = "datafull"
	StrStructure   = "structure"
	V0             = 0
	StrMovie       = "movie"
	StrSeries      = "series"
	StrID          = "id"
	StrWaitfor     = "waitfor"
	StrURL         = "Url"
	StrImdb        = "imdb"
	StrFound       = "found"
	StrWanted      = "wanted"
	StrTitle       = "Title"
	StrAccepted    = "accepted"
	StrDenied      = "denied"
	StrJob         = "Job"
	StrPath        = "Path"
	StrFile        = "File"
	StrListname    = "Listname"
	StrData        = "data"
	StrTvdb        = "tvdb"
	StrSeason      = "season"
	StrConfig      = "config"
	StrReason      = "reason"
	StrPriority    = "Priority"
	StrMinPrio     = "minimum prio"
	StrQuality     = "Quality"
	ArrHTMLEntitys = []string{
		"&AElig",
		"&AMP",
		"&Aacute",
		"&Acirc",
		"&Agrave",
		"&Aring",
		"&Atilde",
		"&Auml",
		"&COPY",
		"&Ccedil",
		"&ETH",
		"&Eacute",
		"&Ecirc",
		"&Egrave",
		"&Euml",
		"&GT",
		"&Iacute",
		"&Icirc",
		"&Igrave",
		"&Iuml",
		"&LT",
		"&Ntilde",
		"&Oacute",
		"&Ocirc",
		"&Ograve",
		"&Oslash",
		"&Otilde",
		"&Ouml",
		"&QUOT",
		"&REG",
		"&THORN",
		"&Uacute",
		"&Ucirc",
		"&Ugrave",
		"&Uuml",
		"&Yacute",
		"&aacute",
		"&acirc",
		"&acute",
		"&aelig",
		"&agrave",
		"&amp",
		"&aring",
		"&atilde",
		"&auml",
		"&brvbar",
		"&ccedil",
		"&cedil",
		"&cent",
		"&copy",
		"&curren",
		"&deg",
		"&divide",
		"&eacute",
		"&ecirc",
		"&egrave",
		"&eth",
		"&euml",
		"&gt",
		"&iacute",
		"&icirc",
		"&iexcl",
		"&igrave",
		"&iquest",
		"&iuml",
		"&laquo",
		"&lt",
		"&macr",
		"&micro",
		"&middot",
		"&nbsp",
		"&not",
		"&ntilde",
		"&oacute",
		"&ocirc",
		"&ograve",
		"&ordf",
		"&ordm",
		"&oslash",
		"&otilde",
		"&ouml",
		"&para",
		"&plusmn",
		"&pound",
		"&quot",
		"&raquo",
		"&reg",
		"&sect",
		"&shy",
		"&szlig",
		"&thorn",
		"&times",
		"&uacute",
		"&ucirc",
		"&ugrave",
		"&uml",
		"&uuml",
		"&yacute",
		"&yen",
		"&yuml",
	}
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
	PlAddBuffer              pool.Poolobj[AddBuffer]
	PlBuffer                 pool.Poolobj[bytes.Buffer]
	PLArrAny                 pool.Poolobj[Arrany]

	mapSrings = map[string]mapmovieserie{
		"CacheDBMedia": {
			Movie: "CacheDBMovie",
			Serie: "CacheDBSeries",
		},
		"DBCountDBMedia": {
			Movie: "select count() from dbmovies",
			Serie: "select count() from dbseries",
		},
		"DBCacheDBMedia": {
			Movie: "select title, slug, imdb_id, year, id from dbmovies",
			Serie: "select seriename, slug, '', 0, id from dbseries",
		},
		"CacheMedia": {
			Movie: "CacheMovie",
			Serie: "CacheSeries",
		},
		"DBCountMedia": {
			Movie: "select count() from movies",
			Serie: "select count() from series",
		},
		"DBCacheMedia": {
			Movie: "select lower(listname), dbmovie_id, id from movies",
			Serie: "select lower(listname), dbserie_id, id from series",
		},
		"CacheHistoryTitle": {
			Movie: CacheHistoryTitleMovie,
			Serie: CacheHistoryTitleSeries,
		},
		"CacheHistoryUrl": {
			Movie: CacheHistoryURLMovie,
			Serie: CacheHistoryURLSeries,
		},
		"DBHistoriesUrl": {
			Movie: "select distinct url from movie_histories",
			Serie: "select distinct url from serie_episode_histories",
		},
		"DBHistoriesTitle": {
			Movie: "select distinct title from movie_histories",
			Serie: "select distinct title from serie_episode_histories",
		},
		"DBCountHistoriesUrl": {
			Movie: "select count() from (select distinct url from movie_histories)",
			Serie: "select count() from (select distinct url from serie_episode_histories)",
		},
		"DBCountHistoriesTitle": {
			Movie: "select count() from (select distinct title from movie_histories)",
			Serie: "select count() from (select distinct title from serie_episode_histories)",
		},
		"CacheMediaTitles": {
			Movie: "CacheTitlesMovie",
			Serie: "CacheDBSeriesAlt",
		},
		"DBCountDBTitles": {
			Movie: "select count() from dbmovie_titles where title != ''",
			Serie: "select count() from dbserie_alternates where title != ''",
		},
		"DBCacheDBTitles": {
			Movie: "select title, slug, dbmovie_id from dbmovie_titles where title != ''",
			Serie: "select title, slug, dbserie_id from dbserie_alternates where title != ''",
		},
		"CacheFiles": {
			Movie: CacheFilesMovie,
			Serie: CacheFilesSeries,
		},
		"DBCountFiles": {
			Movie: "select count() from movie_files",
			Serie: "select count() from serie_episode_files",
		},
		"DBCacheFiles": {
			Movie: "select location from movie_files",
			Serie: "select location from serie_episode_files",
		},
		"CacheUnmatched": {
			Movie: CacheUnmatchedMovie,
			Serie: CacheUnmatchedSeries,
		},
		"DBCountUnmatched": {
			Movie: "select count() from movie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
			Serie: "select count() from serie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		},
		"DBRemoveUnmatched": {
			Movie: "delete from movie_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
			Serie: "delete from serie_file_unmatcheds where (last_checked < datetime('now','-'||?||' hours') and last_checked is not null)",
		},
		"DBCacheUnmatched": {
			Movie: "select filepath from movie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
			Serie: "select filepath from serie_file_unmatcheds where (last_checked > datetime('now','-'||?||' hours') or last_checked is null)",
		},
		"DBCountFilesLocation": {
			Movie: "select count() from movie_files where location = ?",
			Serie: "select count() from serie_episode_files where location = ?",
		},
		"DBCountUnmatchedPath": {
			Movie: "select count() from movie_file_unmatcheds where filepath = ?",
			Serie: "select count() from serie_file_unmatcheds where filepath = ?",
		},
		"DBCountDBTitlesDBID": {
			Movie: "select count() from (select distinct title, slug from dbmovie_titles where dbmovie_id = ? and title != '')",
			Serie: "select count() from (select distinct title, slug from dbserie_alternates where dbserie_id = ? and title != '')",
		},
		"DBDistinctDBTitlesDBID": {
			Movie: "select distinct title, slug, dbmovie_id from dbmovie_titles where dbmovie_id = ? and title != ''",
			Serie: "select distinct title, slug, dbserie_id from dbserie_alternates where dbserie_id = ? and title != ''",
		},
		"DBMediaTitlesID": {
			Movie: "select year, title, slug from dbmovies where id = ?",
			Serie: "select 0, seriename, slug from dbseries where id = ?",
		},
		"DBFilesQuality": {
			Movie: "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?",
			Serie: "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?",
		},
		"DBCountFilesByList": {
			Movie: "select count() from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
			Serie: "select count() from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		},
		"DBLocationFilesByList": {
			Movie: "select location from movie_files where movie_id in (select id from movies where listname = ? COLLATE NOCASE)",
			Serie: "select location from serie_episode_files where serie_id in (Select id from series where listname = ? COLLATE NOCASE)",
		},
		"DBIDsFilesByLocation": {
			Movie: "select location, id, movie_id from movie_files",
			Serie: "select location, id, serie_episode_id from serie_episode_files",
		},
		"DBCountFilesByMediaID": {
			Movie: "select count() from movie_files where movie_id = ?",
			Serie: "select count() from serie_episode_files where serie_episode_id = ?",
		},
		"DBCountFilesByLocation": {
			Movie: "select count() from movie_files",
			Serie: "select count() from serie_episode_files",
		},
		"TableFiles": {
			Movie: "movie_files",
			Serie: "serie_episode_files",
		},
		"TableMedia": {
			Movie: "movies",
			Serie: "serie_episodes",
		},
		"DBCountMediaByList": {
			Movie: "select count() from movies where listname = ? COLLATE NOCASE",
			Serie: "select count() from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
		},
		"DBIDMissingMediaByList": {
			Movie: "select id,missing from movies where listname = ? COLLATE NOCASE",
			Serie: "select id, missing from serie_episodes where serie_id in (select id from series where listname = ? COLLATE NOCASE)",
		},
		"DBUpdateMissing": {
			Movie: "update movies set missing = ? where id = ?",
			Serie: "update serie_episodes set missing = ? where id = ?",
		},
		"DBListnameByMediaID": {
			Movie: "select listname from movies where id = ?",
			Serie: "select listname from series where id = ?",
		},
		"DBRootPathFromMediaID": {
			Movie: "select rootpath from movies where id = ?",
			Serie: "select rootpath from series where id = ?",
		},
		"DBDeleteFileByIDLocation": {
			Movie: "delete from movie_files where movie_id = ? and location = ?",
			Serie: "delete from serie_episode_files where serie_id = ? and location = ?",
		},
		"DBCountHistoriesByTitle": {
			Movie: "select count() from movie_histories where title = ?",
			Serie: "select count() from serie_episode_histories where title = ?",
		},
		"DBCountHistoriesByUrl": {
			Movie: "select count() from movie_histories where url = ?",
			Serie: "select count() from serie_episode_histories where url = ?",
		},
		"DBLocationIDFilesByID": {
			Movie: "select location, id from movie_files where movie_id = ?",
			Serie: "select location, id from serie_episode_files where serie_episode_id = ?",
		},
		"DBFilePrioFilesByID": {
			Movie: "select location, movie_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from movie_files where movie_id = ?",
			Serie: "select location, serie_episode_id, id, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended from serie_episode_files where serie_episode_id = ?",
		},
		"UpdateMediaLastscan": {
			Movie: "update movies set lastscan = datetime('now','localtime') where id = ?",
			Serie: "update serie_episodes set lastscan = datetime('now','localtime') where id = ?",
		},
		"DBQualityMediaByID": {
			Movie: "select quality_profile from movies where id = ?",
			Serie: "select quality_profile from serie_episodes where id = ?",
		},
		"SearchGenSelect": {
			Movie: "select movies.quality_profile, movies.id ",
			Serie: "select serie_episodes.quality_profile, serie_episodes.id ",
		},
		"SearchGenTable": {
			Movie: " from movies inner join dbmovies on dbmovies.id=movies.dbmovie_id where ",
			Serie: " from serie_episodes inner join dbserie_episodes on dbserie_episodes.id=serie_episodes.dbserie_episode_id inner join series on series.id=serie_episodes.serie_id where ",
		},
		"SearchGenMissing": {
			Movie: "dbmovies.year != 0 and movies.missing = 1 and movies.listname in (?",
			Serie: "serie_episodes.missing = 1 and ((dbserie_episodes.season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
		},
		"SearchGenMissingEnd": {
			Movie: ")",
			Serie: ") and serie_episodes.dbserie_episode_id in (select id from dbserie_episodes group by dbserie_id, identifier having count() = 1)",
		},
		"SearchGenReached": {
			Movie: "dbmovies.year != 0 and quality_reached = 0 and missing = 0 and listname in (?",
			Serie: "serie_episodes.missing = 0 and serie_episodes.quality_reached = 0 and ((dbserie_episodes.Season != '0' and series.search_specials=0) or (series.search_specials=1)) and series.listname in (?",
		},
		"SearchGenLastScan": {
			Movie: " and (movies.lastscan is null or movies.Lastscan < ?)",
			Serie: " and (serie_episodes.lastscan is null or serie_episodes.lastscan < ?)",
		},
		"SearchGenDate": {
			Movie: " and (dbmovies.release_date < ? or dbmovies.release_date is null)",
			Serie: " and (dbserie_episodes.first_aired < ? or dbserie_episodes.first_aired is null)",
		},
		"SearchGenOrder": {
			Movie: " order by movies.Lastscan asc",
			Serie: " order by serie_episodes.Lastscan asc",
		},
		"DBIDUnmatchedPathList": {
			Movie: "select id from movie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
			Serie: "select id from serie_file_unmatcheds where filepath = ? and listname = ? COLLATE NOCASE",
		},
		"InsertUnmatched": {
			Movie: "Insert into movie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
			Serie: "Insert into serie_file_unmatcheds (parsed_data, listname, filepath, last_checked) values (?, ?, ?, datetime('now','localtime'))",
		},
		"UpdateUnmatched": {
			Movie: "update movie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
			Serie: "update serie_file_unmatcheds SET parsed_data = ?, last_checked = datetime('now','localtime') where id = ?",
		},
		"GetRSSData": {
			Movie: "select movies.dont_search, movies.dont_upgrade, movies.listname, movies.quality_profile, dbmovies.title from movies inner join dbmovies ON dbmovies.id=movies.dbmovie_id where movies.id = ?",
			Serie: "select serie_episodes.dont_search, serie_episodes.dont_upgrade, series.listname, serie_episodes.quality_profile, dbseries.seriename from serie_episodes inner join series ON series.id=serie_episodes.serie_id inner join dbseries ON dbseries.id=serie_episodes.dbserie_id where serie_episodes.id = ?",
		},
		"GetOrganizeData": {
			Movie: "select dbmovie_id, rootpath, listname from movies where id = ?",
			Serie: "select dbserie_id, rootpath, listname from series where id = ?",
		},
	}
)

// local vars.
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
	subRuneSet = [256]bool{
		'a': true, 'b': true, 'c': true, 'd': true, 'e': true, 'f': true, 'g': true, 'h': true,
		'i': true, 'j': true, 'k': true, 'l': true, 'm': true, 'n': true, 'o': true, 'p': true,
		'q': true, 'r': true, 's': true, 't': true, 'u': true, 'v': true, 'w': true, 'x': true,
		'y': true, 'z': true, '0': true, '1': true, '2': true, '3': true, '4': true, '5': true,
		'6': true, '7': true, '8': true, '9': true, '-': true,
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
	diacriticslowermap = map[rune]rune{
		'ä': 'ä',
		'ö': 'ö',
		'ü': 'ü',
		'Ä': 'ä',
		'Ö': 'ö',
		'Ü': 'ü',
		'ß': 'ß',
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
func IntToString(a uint16) string {
	return strconv.Itoa(int(a))
}

// WriteInt writes the string representation of the given integer i to the buffer.
func (b *AddBuffer) WriteInt(i int) {
	b.WriteString(strconv.Itoa(i))
}

// WriteUInt16 writes the given uint16 value to the buffer as a string.
func (b *AddBuffer) WriteUInt16(i uint16) {
	b.WriteInt(int(i))
}

// WriteUInt writes the string representation of the given unsigned integer to the buffer.
func (b *AddBuffer) WriteUInt(i uint) {
	b.WriteInt(int(i))
}

// WriteStringMap writes a string to the buffer based on the provided boolean and string parameters.
// If useseries is true, it writes the value from the Mapstringsseries map using the typestr key.
// If useseries is false, it writes the value from the Mapstringsmovies map using the typestr key.
func (b *AddBuffer) WriteStringMap(useseries bool, typestr string) {
	b.WriteString(GetStringsMap(useseries, typestr))
}

// WriteURL writes the given string to the buffer after escaping it for use in a URL.
func (b *AddBuffer) WriteURL(s string) {
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
			LogDynamicanyErr("error", "template", err)
			return true, ""
		}
	}
	doc := PlBuffer.Get()
	defer PlBuffer.Put(doc)
	if err := tmplmessage.Execute(doc, messagedata); err != nil {
		LogDynamicanyErr("error", "template", err)
		return true, ""
	}
	return false, doc.String()
}

func BytesToString(b []byte) string {
	bld := PlBuffer.Get()
	defer PlBuffer.Put(bld)
	bld.Write(b)
	return bld.String()
}

// StringToSlug converts the given string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens.
func StringToSlug(instr string) string {
	return string(StringToSlugBytes(instr)) // BytesToString
}

// Checkhtmlentities checks if the input string contains HTML entities, and if so,
// unescapes the entities using html.UnescapeString. If the input string does not
// contain any HTML entities, it is returned as-is.
func Checkhtmlentities(instr string) string {
	if !strings.ContainsRune(instr, '&') {
		return instr
	}
	if strings.ContainsRune(instr, ';') {
		return html.UnescapeString(instr)
	}
	for idx := range ArrHTMLEntitys {
		if strings.Contains(instr, ArrHTMLEntitys[idx]) {
			return html.UnescapeString(instr)
		}
	}
	return instr
}

// StringToSlug converts the given string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens.
func StringToSlugBytes(instr string) []byte {
	ret := PlBuffer.Get()
	defer PlBuffer.Put(ret)
	stringToSlugBuffer(ret, Checkhtmlentities(instr))
	return bytes.Trim(ret.Bytes(), "- ")
}

// stringToSlugBuffer converts the given input string to a slug format by replacing
// unwanted characters, transliterating accented characters, replacing multiple
// hyphens with a single hyphen, and trimming leading/trailing hyphens. The result
// is written to the provided bytes.Buffer.
func stringToSlugBuffer(ret *bytes.Buffer, instr string) {
	if len(instr) == 0 {
		return
	}
	var (
		lastrune, section, position rune
		laststr                     string
	)
	for _, r := range instr {
		if val, ok := substituteRuneSpace[r]; ok {
			if (laststr == "" || val != laststr) && (lastrune != '-' || val != StrDash) {
				ret.WriteString(val)
				laststr = val

				if val == StrDash {
					lastrune = '-'
				} else {
					lastrune = ' '
				}
			}
			continue
		}

		if laststr != "" {
			laststr = ""
		}

		switch {
		case r < unicode.MaxASCII:
			if 'A' <= r && r <= 'Z' {
				r += 'a' - 'A'
			}
			// if _, ok = subRune[r]; ok {
			if r < 256 && subRuneSet[r] {
				if lastrune != '-' || r != '-' {
					ret.WriteRune(r)
					lastrune = r
				}
			} else if lastrune != '-' {
				ret.WriteByte('-')
				lastrune = '-'
			}
		case r <= 0xeffff:
			section = r >> 8
			position = r % 256
			if tb, ok := table.Tables[section]; ok && len(tb) > int(position) {
				if len(tb[position]) >= 1 && tb[position][0] > unicode.MaxASCII && lastrune != '-' {
					ret.WriteByte('-')
					lastrune = '-'
				} else if lastrune != '-' || tb[position] != StrDash {
					ret.WriteString(tb[position])
				}
			}
		}
	}
}

// AddImdbPrefixP adds the "tt" prefix to the given string if it doesn't already have the prefix.
// If the string is nil or has a length less than 1, this function does nothing.
func AddImdbPrefixP(str string) string {
	if len(str) >= 1 && !HasPrefixI(str, StrTt) {
		return JoinStrings(StrTt, str)
	}
	return str
}

// Path sanitizes the given string by cleaning it with path.Clean, unquoting
// and unescaping it, optionally removing slashes, and replacing invalid
// characters. It returns the cleaned path string.
func Path(s *string, allowslash bool) {
	if s == nil || *s == "" {
		return
	}
	newpath := path.Clean(UnquoteUnescape(*s))
	if !allowslash {
		StringRemoveAllRunesP(&newpath, '\\', '/')
	}
	bld := PlBuffer.Get()
	defer PlBuffer.Put(bld)

	var bl bool
	for _, z := range newpath {
		if r, ok := diacriticsmap[z]; ok {
			bld.WriteString(r)
			bl = true
		} else if _, ok := pathmap[z]; !ok {
			bld.WriteRune(z)
		} else {
			bl = true
		}
	}
	if bl {
		*s = TrimSpace(bld.String())
	}
}

// TrimSpace returns a slice of the string s, with all leading and trailing
// Unicode code points that are considered whitespace removed.
// If s is empty, TrimSpace returns s.
func TrimSpace(s string) string {
	if len(s) == 0 {
		return s
	}
	return Trim(s, ' ')
}

// Trim returns a slice of the string s, with all leading and trailing Unicode code points contained in cutset removed.
// If s is empty or cutset is empty, Trim returns s.
func Trim(s string, cutset ...rune) string {
	if len(s) == 0 {
		return s
	}
	i := getfirstinstring(s, cutset)
	j := getlastinstring(s, cutset)
	if i == -1 && j == -1 {
		return s
	}
	if i == -1 {
		return s[:j]
	}
	if j == -1 {
		return s[i:]
	}
	return s[i:j]
}

// TrimLeft returns a slice of the string s, with all leading Unicode code points contained in cutset removed.
// If s is empty or cutset is empty, TrimLeft returns s.
func TrimLeft(s string, cutset ...rune) string {
	if len(s) == 0 {
		return s
	}
	if i := getfirstinstring(s, cutset); i != -1 {
		return s[i:]
	}
	return s
}

// getfirstinstring returns the index of the first character in the string s that is not in the cutset.
// If no such character is found, it returns -1.
func getfirstinstring(s string, cutset []rune) int {
	for idx, y := range s {
		found := false
		for _, z := range cutset {
			if y == z {
				found = true
				break
			}
		}

		if !found {
			if idx == 0 {
				return -1
			}
			return idx
		}
	}
	return -1
}

// getlastinstring returns the index of the last character in the string s that is not in the cutset.
// If no such character is found, it returns -1.
func getlastinstring(s string, cutset []rune) int {
	for idx := len(s) - 1; idx >= 0; idx-- {
		found := false
		var x rune
		for idx2, y := range s {
			if idx2 == idx {
				x = y
				break
			}
		}
		for _, z := range cutset {
			if x == z {
				found = true
				break
			}
		}
		if !found && idx == 0 {
			return -1
		}
		if !found && idx > 0 {
			return idx + 1
		}
	}
	return -1
}

// TrimRight returns a slice of the string s, with all trailing
// Unicode code points contained in cutset removed.
func TrimRight(s string, cutset ...rune) string {
	if len(s) == 0 {
		return s
	}
	if i := getlastinstring(s, cutset); i != -1 {
		return s[:i]
	}
	return s
}

// ContainsI checks if string a contains string b, ignoring case.
// It first checks for a direct match with strings.Contains.
// If not found, it does a case-insensitive search by looping through a
// and comparing substrings with EqualFold.
func ContainsI(a, b string) bool {
	return IndexI(a, b) != -1
}

// ContainsByteI checks if the byte slice a contains the byte slice b, ignoring case.
// It first checks for a direct match using bytes.Contains.
// If not found, it does a case-insensitive search by converting both a and b to lowercase
// and then checking if the lowercase a contains the lowercase b.
func ContainsByteI(a, b []byte) bool {
	if bytes.Contains(a, b) {
		return true
	}
	if len(a) < len(b) {
		return false
	}
	return bytes.Contains(bytes.ToLower(a), bytes.ToLower(b))
}

// ContainsInt checks if the string a contains the string representation of
// the integer b. It converts b to a string using strconv.Itoa and calls
// strings.Contains to check for a match.
func ContainsInt(a string, b uint16) bool {
	return strings.Contains(a, strconv.Itoa(int(b)))
}

// HasPrefixI checks if string s starts with prefix, ignoring case.
// It first checks for a direct match with strings.HasPrefix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from 0 to len(prefix) with prefix using EqualFold.
func HasPrefixI(s, prefix string) bool {
	return len(s) >= len(prefix) &&
		(s[0:len(prefix)] == prefix || strings.EqualFold(s[0:len(prefix)], prefix))
}

// TimeAfter returns true if the time a is after the time b.
// If the Unix timestamps of a and b are equal, it compares the
// nanosecond parts of the times to determine the order.
func TimeAfter(a, b time.Time) bool {
	as := a.Unix()
	bs := b.Unix()
	if as == bs {
		return a.UnixNano() > b.UnixNano()
	}
	return as > bs
}

// HasSuffixI checks if string s ends with suffix, ignoring case.
// It first checks for a direct match with strings.HasSuffix.
// If not found, it does a case-insensitive check by comparing
// the substring of s from len(s)-len(suffix) to len(s) with suffix
// using EqualFold.
func HasSuffixI(s, suffix string) bool {
	return len(s) >= len(suffix) &&
		(s[len(s)-len(suffix):] == suffix || strings.EqualFold(s[len(s)-len(suffix):], suffix))
}

// JoinStrings concatenates any number of strings together.
// It is optimized to avoid unnecessary allocations when there are few elements.
func JoinStrings(elems ...string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	case 2:
		if elems[0] == "" {
			return elems[1]
		}
		if elems[1] == "" {
			return elems[0]
		}
	}
	b := PlBuffer.Get()
	defer PlBuffer.Put(b)
	for idx := range elems {
		if elems[idx] != "" {
			b.WriteString(elems[idx])
		}
	}
	return b.String()
}

// JoinStringsSep concatenates the elements of the provided slice of strings
// into a single string, separated by the provided separator string.
// It is optimized to avoid unnecessary allocations when there are few elements.
func JoinStringsSep(elems []string, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}
	b := PlBuffer.Get()
	defer PlBuffer.Put(b)
	l := len(elems)
	for idx, val := range elems {
		if val != "" {
			b.WriteString(val)
			if idx < l-1 {
				b.WriteString(sep)
			}
		}
	}
	return b.String()
}

// IndexI searches for the first case-insensitive instance of b in a.
// It returns the index of the first match, or -1 if no match is found.
func IndexI(a, b string) int {
	if i := strings.Index(a, b); i != -1 {
		return i
	}

	if len(b) > len(a) {
		return -1
	}
	hasUppera, hasUpperb := false, false
	isASCIIb := true
	for _, c := range a {
		if c >= utf8.RuneSelf {
			if _, ok := diacriticslowermap[c]; !ok {
				// isASCIIa = false
				break
			}
		}
		hasUppera = hasUppera || ('A' <= c && c <= 'Z') || c == 'Ö' || c == 'Ü' || c == 'Ä'
	}

	for _, c := range b {
		if c >= utf8.RuneSelf {
			_, ok := diacriticslowermap[c]
			if !ok {
				isASCIIb = false
				break
			}
		}
		hasUpperb = hasUpperb || ('A' <= c && c <= 'Z') || c == 'Ö' || c == 'Ü' || c == 'Ä'
	}

	if isASCIIb {
		if !hasUppera && !hasUpperb {
			return strings.Index(a, b)
		}
		bufa := PlBuffer.Get()
		defer PlBuffer.Put(bufa)
		for _, c := range a {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			} else if c >= utf8.RuneSelf {
				d, ok := diacriticslowermap[c]
				if ok {
					c = d
				}
			}
			bufa.WriteRune(c)
		}

		bufb := PlBuffer.Get()
		defer PlBuffer.Put(bufb)
		for _, c := range b {
			if 'A' <= c && c <= 'Z' {
				c += 'a' - 'A'
			} else if c >= utf8.RuneSelf {
				d, ok := diacriticslowermap[c]
				if ok {
					c = d
				}
			}
			bufb.WriteRune(c)
		}

		return bytes.Index(bufa.Bytes(), bufb.Bytes())
	}
	return strings.Index(strings.Map(unicode.ToLower, a), strings.Map(unicode.ToLower, b))
}

// IntToUint converts an int64 to a uint, returning 0 if the input is negative.
func Int64ToUint(in int64) uint {
	if in < 0 {
		return 0
	}
	return uint(in)
}

// IntToUint converts an int to a uint, returning 0 if the input is negative.
func IntToUint(in int) uint {
	if in < 0 {
		return 0
	}
	return uint(in)
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

// StringToDuration converts the given string to a time.Duration.
// It first tries to parse the string as a float and then cast it to time.Duration.
// If that fails, it tries to parse the string directly as an int and then cast it to time.Duration.
// If both attempts fail, it returns 0.
func StringToDuration(s string) time.Duration {
	if s == "" || s == "0" {
		return 0
	}
	if strings.ContainsRune(s, '.') || strings.ContainsRune(s, ',') {
		in, err := strconv.ParseFloat(StringReplaceWith(s, ',', '.'), 64)
		if err != nil {
			return 0
		}
		return time.Duration(in)
	}
	in, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return time.Duration(in)
}

// StringToInt32 converts the given string to an int32.
// It first tries to parse the string as a float and then cast it to int32.
// If that fails, it tries to parse the string directly as an int32.
// If both attempts fail, it returns 0.
func StringToInt32(s string) int32 {
	if s == "" || s == "0" {
		return 0
	}
	if strings.ContainsRune(s, '.') || strings.ContainsRune(s, ',') {
		in, err := strconv.ParseFloat(StringReplaceWith(s, ',', '.'), 64)
		if err != nil {
			return 0
		}
		return int32(in)
	}
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return 0
	}
	return int32(i)
}

// StringToUInt16 converts a string to a uint16 value.
// It does this by first converting the string to an int64 using StringToInt,
// and then casting the result to a uint16.
func StringToUInt16(s string) uint16 {
	if s == "" || s == "0" {
		return 0
	}
	i, err := strconv.ParseInt(s, 10, 16)
	if err != nil {
		return 0
	}
	return uint16(i)
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
		return 0
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
		u, err := strconv.Unquote(s)
		if err == nil {
			return Checkhtmlentities(u)
		}
	}
	return Checkhtmlentities(s)
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

// Contains reports whether s is contained in v - case insensitive.
func SlicesContainsPart2I(s []string, v string) bool {
	for idx := range s {
		if ContainsI(v, s[idx]) {
			return true
		}
	}
	return false
}

// StringRemoveAllRunesP removes all occurrences of the given runes from the string pointed to by s.
// It modifies the string in-place.
// If s is nil or an empty string, or if the slice of runes r is empty, this function does nothing.
// If the slice of runes r contains only one rune and that rune is not present in the string, this function does nothing.
// If the slice of runes r contains more than one rune and none of them are present in the string, this function does nothing.
// Otherwise, this function creates a new buffer, writes all the characters from the string that are not in the slice of runes r, and updates the string pointed to by s with the new content.
func StringRemoveAllRunesP(s *string, r ...byte) {
	if s == nil || *s == "" {
		return
	}
	if len(r) == 0 {
		return
	}
	if len(r) == 1 && !strings.ContainsRune(*s, rune(r[0])) {
		return
	}
	if len(r) > 1 {
		var bl bool
		for idx := range r {
			if strings.ContainsRune(*s, rune(r[idx])) {
				bl = true
				break
			}
		}
		if !bl {
			return
		}
	}
	out := PlBuffer.Get()
	defer PlBuffer.Put(out)
	for idx := range *s {
		if isruneinbyteslice((*s)[idx], r) {
			continue
		}
		out.WriteByte((*s)[idx])
	}
	*s = out.String()
}

// isruneinbyteslice checks if the given byte r is present in the slice of bytes rs.
// It returns true if r is found in rs, false otherwise.
func isruneinbyteslice(r byte, rs []byte) bool {
	for idx := range rs {
		if r == rs[idx] {
			return true
		}
	}
	return false
}

// StringReplaceWith replaces all occurrences of the byte r in s with the byte t.
// It returns a new string with the replacements.
func StringReplaceWith(s string, r, t byte) string {
	if s == "" {
		return s
	}
	if !strings.ContainsRune(s, rune(r)) {
		return s
	}
	buf := PlBuffer.Get()
	defer PlBuffer.Put(buf)
	for idx := range s {
		if s[idx] == r {
			buf.WriteByte(t)
		} else {
			buf.WriteByte(s[idx])
		}
	}
	return buf.String()
}

// StringReplaceWithP replaces all occurrences of the rune r in the given string pointer s with the rune t.
// It modifies the string in-place.
func StringReplaceWithP(s *string, r, t byte) {
	if s == nil || *s == "" {
		return
	}
	if !strings.ContainsRune(*s, rune(r)) {
		return
	}
	buf := PlBuffer.Get()
	defer PlBuffer.Put(buf)
	for idx := range *s {
		if (*s)[idx] == r {
			buf.WriteByte(t)
		} else {
			buf.WriteByte((*s)[idx])
		}
	}
	*s = buf.String()
}

// StringReplaceWithStr replaces all occurrences of the string r in s with the string t.
// It returns a new string with the replacements.
func StringReplaceWithStr(s, r, t string) string {
	if s == "" {
		return s
	}
	if !strings.Contains(s, r) {
		return s
	}
	// Compute number of replacements.
	n := strings.Count(s, r)
	if n == 0 {
		return s // avoid allocation
	}

	// Apply replacements to buffer.
	buf := PlBuffer.Get()
	defer PlBuffer.Put(buf)
	start := 0
	lenr := len(r)
	for i := range n {
		j := start
		if lenr == 0 {
			if i > 0 {
				_, wid := utf8.DecodeRuneInString(s[start:])
				j += wid
			}
		} else {
			j += strings.Index(s[start:], r)
		}
		buf.WriteString(s[start:j])
		buf.WriteString(t)
		start = j + lenr
	}
	buf.WriteString(s[start:])
	return buf.String()
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

// CheckContextEnded checks if the provided context has been canceled or has expired.
// If the context has been canceled or has expired, it returns the context's error.
// Otherwise, it returns nil.
func CheckContextEnded(ctx context.Context) error {
	select {
	case <-ctx.Done():
		// Abort / return early
		return ctx.Err()
	default:
		return nil
	}
}

// HandlePanic recovers from a panic and logs the recovered value along with the stack trace.
func HandlePanic() {
	// detect if panic occurs or not
	a := recover()
	if a != nil {
		LogDynamicany2StrAny("error", "Recovered from panic", "RECOVER", Stack(), "vap", a)
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

// TryTimeParse attempts to parse the given string `s` using the provided time layout `layout`.
// It returns the parsed time.Time value and a boolean indicating whether the parsing was successful.
func TryTimeParse(layout string, s string) (time.Time, bool) {
	sleeptime, err := time.Parse(layout, s)
	return sleeptime, err == nil
}

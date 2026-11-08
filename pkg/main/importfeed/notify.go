package importfeed

import (
	"strconv"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// ListEvent is the template data of the added_to_list and added_to_db
// notification events. The flat fields mirror those of the added_data
// event; the Db* row matching Type is filled in as well.
type ListEvent struct {
	Event         string // added_to_list or added_to_db
	Time          string
	Configuration string // the media config, e.g. "EN"
	List          string // the list the entry went into; empty for added_to_db
	Type          string // movie, serie, book, audiobook or album

	Title      string
	Year       string
	Identifier string // IMDb ID, TVDB ID, ISBN-13, ASIN or MusicBrainz release ID
	Imdb       string
	Tvdb       string
	DBID       uint // the dbmovies/dbseries/... row

	Dbmovie     database.Dbmovie
	Dbserie     database.Dbserie
	Dbbook      database.Dbbook
	Dbaudiobook database.Dbaudiobook
	Dbalbum     database.Dbalbum
}

// fillListEvent loads the db row of e.DBID for cfgp's media type.
func fillListEvent(e *ListEvent, isType uint) bool {
	switch isType {
	case config.MediaTypeMovie:
		if e.Dbmovie.GetDbmovieByIDP(&e.DBID) != nil {
			return false
		}

		e.Type, e.Title, e.Year = "movie", e.Dbmovie.Title, yearString(int(e.Dbmovie.Year))
		e.Imdb, e.Identifier = e.Dbmovie.ImdbID, e.Dbmovie.ImdbID

	case config.MediaTypeSeries:
		if e.Dbserie.GetDbserieByIDP(&e.DBID) != nil {
			return false
		}

		e.Type, e.Title, e.Imdb = "serie", e.Dbserie.Seriename, e.Dbserie.ImdbID
		if len(e.Dbserie.Firstaired) >= 4 {
			e.Year = e.Dbserie.Firstaired[:4]
		}

		if e.Dbserie.ThetvdbID != 0 {
			e.Tvdb = strconv.Itoa(e.Dbserie.ThetvdbID)
			e.Identifier = e.Tvdb
		}

	case config.MediaTypeBook:
		if e.Dbbook.GetDbbookByIDP(&e.DBID) != nil {
			return false
		}

		e.Type, e.Title, e.Year, e.Identifier = "book", e.Dbbook.Title, yearString(int(e.Dbbook.Year)), e.Dbbook.ISBN13

	case config.MediaTypeAudiobook:
		if e.Dbaudiobook.GetDbaudiobookByIDP(&e.DBID) != nil {
			return false
		}

		e.Type, e.Title, e.Year = "audiobook", e.Dbaudiobook.Title, yearString(int(e.Dbaudiobook.Year))
		e.Identifier = e.Dbaudiobook.ASIN

	case config.MediaTypeMusic:
		if e.Dbalbum.GetDbalbumByIDP(&e.DBID) != nil {
			return false
		}

		e.Type, e.Title, e.Year = "album", e.Dbalbum.Title, yearString(int(e.Dbalbum.Year))
		e.Identifier = e.Dbalbum.MusicbrainzReleaseID

	default:
		return false
	}

	return true
}

// NotifyAdded sends cfgp's notifications for an added_to_list or
// added_to_db event about the db row dbid. listName is the list the entry
// was added to (empty for added_to_db). It does nothing, not even the db
// lookup, when cfgp has no notification for the event.
func NotifyAdded(cfgp *config.MediaTypeConfig, event, listName string, dbid uint) {
	if dbid == 0 || !providers.HasEventNotification(cfgp, event) {
		return
	}

	e := &ListEvent{
		Event:         event,
		Time:          logger.TimeGetNow().Format(logger.GetTimeFormat()),
		Configuration: cfgp.Name,
		List:          listName,
		DBID:          dbid,
	}

	if !fillListEvent(e, cfgp.IsType) {
		logger.Logtype("debug", 1).
			Uint("dbid", dbid).
			Str("event", event).
			Msg("No db row for notification")

		return
	}

	providers.NotifyEvent(cfgp, event, e, nil)
}

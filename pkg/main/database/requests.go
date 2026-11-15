package database

import "errors"

// MediaRequest is media a requestinguser added (media_requests): its list
// entry has dont_search set until an admin approves or rejects it.
type MediaRequest struct {
	ID        uint   `db:"id"`
	Username  string `db:"username"`
	Kind      string `db:"kind"` // movie, series, album, audiobook, book
	EntryID   uint   `db:"entry_id"`
	Listname  string `db:"listname"`
	Title     string `db:"title"`
	CreatedAt string `db:"created_at"`
}

// RequestKinds maps a request kind to its list entry table.
var RequestKinds = map[string]string{
	"movie": "movies", "series": "series", "album": "albums", "audiobook": "audiobooks", "book": "books",
}

// requestDBColumns is each list entry table's column of its metadata row.
var requestDBColumns = map[string]string{
	"movies": "dbmovie_id", "series": "dbserie_id", "albums": "dbalbum_id", "audiobooks": "dbaudiobook_id", "books": "dbbook_id",
}

// ErrRequestDuplicate: the target list already has the requested title.
var ErrRequestDuplicate = errors.New("the list already has this title")

// RequestEntryQuality is the quality profile of a request's list entry.
func RequestEntryQuality(r *MediaRequest) string {
	table, ok := RequestKinds[r.Kind]
	if !ok {
		return ""
	}

	return Getdatarow[string](false, "select coalesce(quality_profile, '') from "+table+" where id = ?", &r.EntryID)
}

// GetMediaRequests returns the open requests, oldest first.
func GetMediaRequests() []MediaRequest {
	return StructscanT[MediaRequest](false, 0,
		"select id, username, kind, entry_id, listname, title, created_at from media_requests order by id")
}

// GetMediaRequest returns one request.
func GetMediaRequest(id *uint) (*MediaRequest, error) {
	return Structscan[MediaRequest](
		"select id, username, kind, entry_id, listname, title, created_at from media_requests where id = ?", false, id)
}

// CountMediaRequests is the number of open requests.
func CountMediaRequests() uint {
	return Getdatarow[uint](false, "select count() from media_requests")
}

// HoldMediaRequest records a requested list entry and keeps it from being
// searched until it is approved (a series' dont_search holds back all its
// episodes: the searches check it).
func HoldMediaRequest(r *MediaRequest) error {
	table, ok := RequestKinds[r.Kind]
	if !ok {
		return ErrUserInvalid
	}

	if err := ExecNErr("update "+table+" set dont_search = 1 where id = ?", &r.EntryID); err != nil {
		return err
	}

	id, err := ExecNid("insert into media_requests (username, kind, entry_id, listname, title) values (?, ?, ?, ?, ?)",
		&r.Username, &r.Kind, &r.EntryID, &r.Listname, &r.Title)
	if err == nil {
		r.ID = uint(id) //nolint:gosec // row ids are positive
	}

	return err
}

// ApproveMediaRequest lets the entry be searched and closes the request.
// listname and quality ("" keeps the entry's) move it to another list or
// quality profile first; a series' episodes get the quality too.
func ApproveMediaRequest(r *MediaRequest, listname, quality string) error {
	table, ok := RequestKinds[r.Kind]
	if !ok {
		return ErrUserInvalid
	}

	if listname != "" {
		col := requestDBColumns[table]
		if Getdatarow[bool](false, "select exists(select 1 from "+table+" where "+col+" = (select "+col+" from "+table+
			" where id = ?) and listname = ? collate nocase and id <> ?)", &r.EntryID, &listname, &r.EntryID) {
			return ErrRequestDuplicate
		}

		if err := ExecNErr("update "+table+" set listname = ? where id = ?", &listname, &r.EntryID); err != nil {
			return err
		}
	}

	if quality != "" {
		if err := ExecNErr("update "+table+" set quality_profile = ? where id = ?", &quality, &r.EntryID); err != nil {
			return err
		}

		if r.Kind == "series" {
			if err := ExecNErr("update serie_episodes set quality_profile = ? where serie_id = ?", &quality, &r.EntryID); err != nil {
				return err
			}
		}
	}

	if err := ExecNErr("update "+table+" set dont_search = 0 where id = ?", &r.EntryID); err != nil {
		return err
	}

	return CloseMediaRequest(&r.ID)
}

// CloseMediaRequest removes a request (approved, rejected, or its entry is
// gone).
func CloseMediaRequest(id *uint) error {
	return ExecNErr("delete from media_requests where id = ?", id)
}

// NewEntryIDs returns the ids of kind's list entries in listname created
// after afterID.
func NewEntryIDs(kind string, afterID uint, listname *string) []uint {
	table, ok := RequestKinds[kind]
	if !ok {
		return nil
	}

	return GetrowsN[uint](false, 10,
		"select id from "+table+" where id > ? and listname = ? collate nocase order by id", &afterID, listname)
}

// MaxEntryID is the highest list entry id of kind (0 for none).
func MaxEntryID(kind string) uint {
	table, ok := RequestKinds[kind]
	if !ok {
		return 0
	}

	return Getdatarow[uint](false, "select coalesce(max(id), 0) from "+table)
}

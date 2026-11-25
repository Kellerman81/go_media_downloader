package api

import (
	"fmt"
	"hash/crc32"
	"path"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// The items of the Jellyfin API: the media groups are the libraries
// (views); movies, series with seasons and episodes, albums with tracks,
// artists, audiobooks with their parts are the list entries and files of
// the groups, with files only. Item IDs are 32 hex digits like Jellyfin's
// GUIDs: two for the kind, the rest the row ID.

const (
	jfView byte = iota + 1
	jfMovie
	jfSeries
	jfSeason // series ID << 16 | season number
	jfEpisode
	jfAlbum
	jfArtist
	jfTrack
	jfBook     // audiobooks entry
	jfBookPart // audiobook_files row
	jfMovieFile
	jfEpisodeFile
	jfUser
	jfServer
	jfPlaylists // the playlists library
	jfPlaylist  // playlists row
	jfGenre     // crc32 of the name
	jfPerson    // TMDB person ID
	jfBoxSet    // TMDB collection ID
	jfSmart     // saved view (crc32 of its name)
)

func jfID(kind byte, n uint64) string { return fmt.Sprintf("%02x%030x", kind, n) }

// parseJFID reads an item ID (with or without the GUID dashes).
func parseJFID(s string) (byte, uint64, bool) {
	s = strings.ToLower(strings.ReplaceAll(s, "-", ""))
	if len(s) != 32 {
		return 0, 0, false
	}

	k, err := strconv.ParseUint(s[:2], 16, 8)
	if err != nil {
		return 0, 0, false
	}

	n, err := strconv.ParseUint(strings.TrimLeft(s[2:], "0"), 16, 64)
	if err != nil && strings.TrimLeft(s[2:], "0") != "" {
		return 0, 0, false
	}

	return byte(k), n, true
}

// jfGroupID is the view ID of a media group.
func jfGroupID(name string) string { return jfID(jfView, uint64(crc32.ChecksumIEEE([]byte(name)))) }

// jfUserID is the user ID of an account.
func jfUserID(name string) string {
	return jfID(jfUser, uint64(crc32.ChecksumIEEE([]byte(strings.ToLower(name)))))
}

// jfCollectionTypes are the views' collection types by library type.
var jfCollectionTypes = map[string]string{"movie": "movies", "series": "tvshows", "album": "music", "audiobook": "books"}

// jfItem is the part of Jellyfin's BaseItemDto the apps use.
type jfItem struct {
	Name                    string            `json:"Name"`
	OriginalTitle           string            `json:"OriginalTitle,omitempty"`
	ServerID                string            `json:"ServerId"`
	ID                      string            `json:"Id"`
	Etag                    string            `json:"Etag,omitempty"`
	DateCreated             string            `json:"DateCreated,omitempty"`
	SortName                string            `json:"SortName,omitempty"`
	PremiereDate            string            `json:"PremiereDate,omitempty"`
	Overview                string            `json:"Overview,omitempty"`
	Genres                  []string          `json:"Genres,omitempty"`
	GenreItems              []jfNameID        `json:"GenreItems,omitempty"`
	RemoteTrailers          []jfTrailer       `json:"RemoteTrailers,omitempty"`
	People                  []jfPersonDto     `json:"People,omitempty"`
	CommunityRating         float64           `json:"CommunityRating,omitempty"`
	RunTimeTicks            int64             `json:"RunTimeTicks,omitempty"`
	ProductionYear          int               `json:"ProductionYear,omitempty"`
	IndexNumber             *int              `json:"IndexNumber,omitempty"`
	ParentIndexNumber       *int              `json:"ParentIndexNumber,omitempty"`
	ProviderIDs             map[string]string `json:"ProviderIds,omitempty"`
	IsFolder                bool              `json:"IsFolder"`
	ParentID                string            `json:"ParentId,omitempty"`
	Type                    string            `json:"Type"`
	CollectionType          string            `json:"CollectionType,omitempty"`
	UserData                *jfUserData       `json:"UserData,omitempty"`
	ChildCount              int               `json:"ChildCount,omitempty"`
	RecursiveItemCount      int               `json:"RecursiveItemCount,omitempty"`
	SeriesName              string            `json:"SeriesName,omitempty"`
	SeriesID                string            `json:"SeriesId,omitempty"`
	SeasonID                string            `json:"SeasonId,omitempty"`
	SeasonName              string            `json:"SeasonName,omitempty"`
	SeriesPrimaryImageTag   string            `json:"SeriesPrimaryImageTag,omitempty"`
	Album                   string            `json:"Album,omitempty"`
	AlbumID                 string            `json:"AlbumId,omitempty"`
	AlbumPrimaryImageTag    string            `json:"AlbumPrimaryImageTag,omitempty"`
	AlbumArtist             string            `json:"AlbumArtist,omitempty"`
	Artists                 []string          `json:"Artists,omitempty"`
	ArtistItems             []jfNameID        `json:"ArtistItems,omitempty"`
	AlbumArtists            []jfNameID        `json:"AlbumArtists,omitempty"`
	MediaType               string            `json:"MediaType,omitempty"`
	LocationType            string            `json:"LocationType"`
	VideoType               string            `json:"VideoType,omitempty"`
	Container               string            `json:"Container,omitempty"`
	ImageTags               map[string]string `json:"ImageTags"`
	BackdropImageTags       []string          `json:"BackdropImageTags"`
	ParentBackdropItemID    string            `json:"ParentBackdropItemId,omitempty"`
	ParentBackdropImageTags []string          `json:"ParentBackdropImageTags,omitempty"`
	ParentThumbItemID       string            `json:"ParentThumbItemId,omitempty"`
	ParentThumbImageTag     string            `json:"ParentThumbImageTag,omitempty"`
	PrimaryImageAspectRatio float64           `json:"PrimaryImageAspectRatio,omitempty"`
	MediaSources            []jfMediaSource   `json:"MediaSources,omitempty"`
	MediaStreams            []jfMediaStream   `json:"MediaStreams,omitempty"`
	PlaylistItemID          string            `json:"PlaylistItemId,omitempty"`
	CanDownload             bool              `json:"CanDownload"`
	CanDelete               bool              `json:"CanDelete"`
	PlayAccess              string            `json:"PlayAccess,omitempty"`
	Path                    string            `json:"Path,omitempty"`
}

type jfNameID struct {
	Name string `json:"Name"`
	ID   string `json:"Id"`
}

// jfUserData is the account's state of an item.
type jfUserData struct {
	PlayedPercentage      float64 `json:"PlayedPercentage,omitempty"`
	UnplayedItemCount     *int    `json:"UnplayedItemCount,omitempty"`
	PlaybackPositionTicks int64   `json:"PlaybackPositionTicks"`
	PlayCount             int     `json:"PlayCount"`
	IsFavorite            bool    `json:"IsFavorite"`
	LastPlayedDate        string  `json:"LastPlayedDate,omitempty"`
	Played                bool    `json:"Played"`
	Key                   string  `json:"Key"`
	ItemID                string  `json:"ItemId"`
}

// jfRow is a row of any item query (the columns not used by a kind are
// empty).
type jfRow struct {
	ID         uint64  `db:"id"`
	DbID       uint64  `db:"db_id"`
	Db2        uint64  `db:"db2"` // dbseries of an episode, dbalbums of a track, dbaudiobooks of a part
	Name       string  `db:"name"`
	Original   string  `db:"original"`
	Year       int     `db:"year"`
	Overview   string  `db:"overview"`
	Premiere   string  `db:"premiere"`
	Rating     float64 `db:"rating"`
	Genres     string  `db:"genres"`
	Created    string  `db:"created"`
	Imdb       string  `db:"imdb"`
	Tmdb       string  `db:"tmdb"`
	Tvdb       string  `db:"tvdb"`
	ParentID   uint64  `db:"parent_id"`
	ParentName string  `db:"parent_name"`
	Idx        int     `db:"idx"`
	Pidx       int     `db:"pidx"`
	FileID     uint64  `db:"file_id"`
	Location   string  `db:"loc"`
	Runtime    float64 `db:"runtime"`
	ArtistID   uint64  `db:"artist_id"`
	Artist     string  `db:"artist"`
	Children   int     `db:"children"`
	Backdrop   string  `db:"backdrop"`
	Size       int64   `db:"size"`
	Bitrate    int     `db:"bitrate"` // kbit/s, 0 unknown
}

// jfKind is how the items of a kind are queried. SQL selects the jfRow
// columns and ends with "e.listname" for the lists' IN clause; Files
// selects the file IDs of the item with ID "%s", PlayKind their
// play_history kind; Folder items count as played when all their files
// are.
type jfKind struct {
	Code     byte
	Type     string
	LibType  string // the library type of its groups
	SQL      string
	Files    string
	PlayKind string
	Folder   bool
}

const jfNone = `'' AS original, '' AS imdb, '' AS tmdb, '' AS tvdb, '' AS backdrop`

// jfAlbumArtist is the first artist of the album d.
const jfAlbumArtist = "(SELECT dbartist_id FROM dbalbum_artists WHERE dbalbum_id = d.id ORDER BY position LIMIT 1)"

var jfKinds = map[byte]*jfKind{
	jfMovie: {
		Code: jfMovie, Type: "Movie", LibType: "movie", PlayKind: "movie",
		SQL: `SELECT e.id, d.id AS db_id, 0 AS db2, COALESCE(d.title, '') AS name, COALESCE(d.original_title, '') AS original,
			COALESCE(d.year, 0) AS year, COALESCE(d.overview, '') AS overview, COALESCE(d.release_date, '') AS premiere,
			COALESCE(d.vote_average, 0) AS rating, COALESCE(d.genres, '') AS genres, COALESCE(e.created_at, '') AS created,
			COALESCE(d.imdb_id, '') AS imdb, CAST(COALESCE(d.moviedb_id, '') AS TEXT) AS tmdb, '' AS tvdb, 0 AS parent_id,
			'' AS parent_name, 0 AS idx, 0 AS pidx, f.id AS file_id, f.location AS loc,
			COALESCE(NULLIF(f.runtime_seconds, 0), d.runtime * 60, 0) AS runtime, 0 AS artist_id, '' AS artist, 0 AS children,
			COALESCE(d.backdrop, '') AS backdrop, COALESCE(f.file_size, 0) AS size, 0 AS bitrate
			FROM movies e INNER JOIN dbmovies d ON d.id = e.dbmovie_id
			INNER JOIN movie_files f ON f.id = (SELECT min(id) FROM movie_files WHERE movie_id = e.id)
			WHERE e.listname`,
		Files: "SELECT id FROM movie_files WHERE movie_id = %s",
	},
	jfSeries: {
		Code: jfSeries, Type: "Series", LibType: "series", PlayKind: "episode", Folder: true,
		SQL: `SELECT e.id, d.id AS db_id, 0 AS db2, COALESCE(d.seriename, '') AS name, '' AS original,
			CAST(COALESCE(substr(d.firstaired, 1, 4), '0') AS INTEGER) AS year, COALESCE(d.overview, '') AS overview,
			COALESCE(d.firstaired, '') AS premiere, CAST(COALESCE(d.rating, '0') AS REAL) AS rating, COALESCE(d.genre, '') AS genres,
			COALESCE(e.created_at, '') AS created, COALESCE(d.imdb_id, '') AS imdb, '' AS tmdb,
			CAST(COALESCE(d.thetvdb_id, '') AS TEXT) AS tvdb, 0 AS parent_id, '' AS parent_name, 0 AS idx, 0 AS pidx, 0 AS file_id,
			'' AS loc, CAST(COALESCE(d.runtime, '0') AS INTEGER) * 60 AS runtime, 0 AS artist_id, '' AS artist,
			(SELECT count() FROM serie_episode_files f WHERE f.serie_id = e.id) AS children, COALESCE(d.fanart, '') AS backdrop,
			0 AS size, 0 AS bitrate
			FROM series e INNER JOIN dbseries d ON d.id = e.dbserie_id
			WHERE EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_id = e.id) AND e.listname`,
		Files: "SELECT id FROM serie_episode_files WHERE serie_id = %s",
	},
	jfEpisode: {
		Code: jfEpisode, Type: "Episode", LibType: "series", PlayKind: "episode",
		SQL: `SELECT e.id, de.id AS db_id, d.id AS db2, COALESCE(de.title, '') AS name, '' AS original,
			CAST(COALESCE(substr(de.first_aired, 1, 4), '0') AS INTEGER) AS year, COALESCE(de.overview, '') AS overview,
			COALESCE(de.first_aired, '') AS premiere, 0 AS rating, '' AS genres, COALESCE(f.created_at, '') AS created,
			'' AS imdb, '' AS tmdb, '' AS tvdb, s.id AS parent_id, COALESCE(d.seriename, '') AS parent_name,
			CAST(COALESCE(de.episode, '0') AS INTEGER) AS idx, CAST(COALESCE(de.season, '0') AS INTEGER) AS pidx,
			f.id AS file_id, f.location AS loc,
			COALESCE(NULLIF(f.runtime_seconds, 0), NULLIF(de.runtime, 0) * 60, CAST(COALESCE(d.runtime, '0') AS INTEGER) * 60) AS runtime,
			0 AS artist_id, '' AS artist, 0 AS children, COALESCE(de.poster, '') AS backdrop, COALESCE(f.file_size, 0) AS size,
			0 AS bitrate
			FROM serie_episodes e INNER JOIN series s ON s.id = e.serie_id INNER JOIN dbseries d ON d.id = s.dbserie_id
			INNER JOIN dbserie_episodes de ON de.id = e.dbserie_episode_id
			INNER JOIN serie_episode_files f ON f.id = (SELECT min(id) FROM serie_episode_files WHERE serie_episode_id = e.id)
			WHERE s.listname`,
		Files: "SELECT id FROM serie_episode_files WHERE serie_episode_id = %s",
	},
	jfAlbum: {
		Code: jfAlbum, Type: "MusicAlbum", LibType: "album", PlayKind: "track", Folder: true,
		SQL: `SELECT e.id, d.id AS db_id, 0 AS db2, COALESCE(d.title, '') AS name, ` + jfNone + `,
			COALESCE(d.year, 0) AS year, '' AS overview, COALESCE(d.release_date, '') AS premiere, 0 AS rating,
			COALESCE(d.genres, '') AS genres, COALESCE(e.created_at, '') AS created, 0 AS parent_id, '' AS parent_name, 0 AS idx,
			0 AS pidx, 0 AS file_id, '' AS loc,
			(SELECT COALESCE(sum(f.runtime_ms), 0) / 1000.0 FROM album_files f WHERE f.album_id = e.id) AS runtime,
			COALESCE(ar.id, 0) AS artist_id, COALESCE(ar.name, '') AS artist,
			(SELECT count() FROM album_files f WHERE f.album_id = e.id) AS children, 0 AS size, 0 AS bitrate
			FROM albums e INNER JOIN dbalbums d ON d.id = e.dbalbum_id LEFT JOIN dbartists ar ON ar.id = ` + jfAlbumArtist + `
			WHERE EXISTS (SELECT 1 FROM album_files f WHERE f.album_id = e.id) AND e.listname`,
		Files: "SELECT id FROM album_files WHERE album_id = %s",
	},
	jfTrack: {
		Code: jfTrack, Type: "Audio", LibType: "album", PlayKind: "track",
		SQL: `SELECT f.id, COALESCE(tr.id, 0) AS db_id, d.id AS db2, COALESCE(tr.title, '') AS name, ` + jfNone + `,
			COALESCE(d.year, 0) AS year, '' AS overview, '' AS premiere, 0 AS rating, COALESCE(d.genres, '') AS genres,
			COALESCE(f.created_at, '') AS created, e.id AS parent_id, COALESCE(d.title, '') AS parent_name,
			COALESCE(f.track_number, 0) AS idx, COALESCE(f.disc_number, 0) AS pidx, f.id AS file_id, f.location AS loc,
			COALESCE(f.runtime_ms, 0) / 1000.0 AS runtime, COALESCE(ar.id, 0) AS artist_id, COALESCE(ar.name, '') AS artist,
			0 AS children, COALESCE(f.file_size, 0) AS size, COALESCE(f.bitrate, 0) AS bitrate
			FROM album_files f INNER JOIN albums e ON e.id = f.album_id INNER JOIN dbalbums d ON d.id = e.dbalbum_id
			LEFT JOIN dbtracks tr ON tr.id = f.dbtrack_id
			LEFT JOIN dbartists ar ON ar.id = COALESCE((SELECT ta.dbartist_id FROM dbtrack_artists ta WHERE ta.dbtrack_id = f.dbtrack_id
				ORDER BY ta.position LIMIT 1), ` + jfAlbumArtist + `)
			WHERE e.listname`,
		Files: "SELECT %s AS id",
	},
	jfBook: {
		Code: jfBook, Type: "MusicAlbum", LibType: "audiobook", PlayKind: "audiobook", Folder: true,
		SQL: `SELECT e.id, d.id AS db_id, 0 AS db2, COALESCE(d.title, '') AS name, ` + jfNone + `,
			COALESCE(d.year, 0) AS year, COALESCE(d.description, '') AS overview, COALESCE(d.release_date, '') AS premiere,
			COALESCE(d.average_rating, 0) * 2 AS rating, '' AS genres, COALESCE(e.created_at, '') AS created, 0 AS parent_id,
			'' AS parent_name, 0 AS idx, 0 AS pidx, 0 AS file_id, '' AS loc,
			COALESCE(NULLIF((SELECT sum(f.runtime_ms) FROM audiobook_files f WHERE f.audiobook_id = e.id), 0) / 1000.0,
				d.runtime_minutes * 60, 0) AS runtime,
			COALESCE(au.id, 0) AS artist_id, COALESCE(au.name, '') AS artist,
			(SELECT count() FROM audiobook_files f WHERE f.audiobook_id = e.id) AS children, 0 AS size, 0 AS bitrate
			FROM audiobooks e INNER JOIN dbaudiobooks d ON d.id = e.dbaudiobook_id
			LEFT JOIN dbauthors au ON au.id = (SELECT dbauthor_id FROM dbaudiobook_authors WHERE dbaudiobook_id = d.id ORDER BY position LIMIT 1)
			WHERE EXISTS (SELECT 1 FROM audiobook_files f WHERE f.audiobook_id = e.id) AND e.listname`,
		Files: "SELECT id FROM audiobook_files WHERE audiobook_id = %s",
	},
	jfBookPart: {
		Code: jfBookPart, Type: "AudioBook", LibType: "audiobook", PlayKind: "audiobook",
		SQL: `SELECT f.id, 0 AS db_id, d.id AS db2, '' AS name, ` + jfNone + `, COALESCE(d.year, 0) AS year, '' AS overview,
			'' AS premiere, 0 AS rating, '' AS genres, COALESCE(f.created_at, '') AS created, e.id AS parent_id,
			COALESCE(d.title, '') AS parent_name, COALESCE(f.track_number, 0) AS idx, COALESCE(f.disc_number, 0) AS pidx,
			f.id AS file_id, f.location AS loc, COALESCE(f.runtime_ms, 0) / 1000.0 AS runtime, 0 AS artist_id,
			COALESCE((SELECT au.name FROM dbaudiobook_authors daa INNER JOIN dbauthors au ON au.id = daa.dbauthor_id
				WHERE daa.dbaudiobook_id = d.id ORDER BY daa.position LIMIT 1), '') AS artist,
			0 AS children, COALESCE(f.file_size, 0) AS size, COALESCE(f.bitrate, 0) AS bitrate
			FROM audiobook_files f INNER JOIN audiobooks e ON e.id = f.audiobook_id INNER JOIN dbaudiobooks d ON d.id = e.dbaudiobook_id
			WHERE e.listname`,
		Files: "SELECT %s AS id",
	},
	jfArtist: {
		Code: jfArtist, Type: "MusicArtist", LibType: "album", PlayKind: "track", Folder: true,
		// Albums without an artist are under ID 0 ("Unknown artist").
		SQL: `SELECT COALESCE(ar.id, 0) AS id, COALESCE(ar.id, 0) AS db_id, 0 AS db2, COALESCE(ar.name, '') AS name, ` + jfNone + `,
			0 AS year, COALESCE(max(ar.bio), '') AS overview, '' AS premiere, 0 AS rating, COALESCE(max(ar.genres), '') AS genres,
			COALESCE(min(e.created_at), '') AS created, 0 AS parent_id, '' AS parent_name, 0 AS idx, 0 AS pidx, 0 AS file_id,
			'' AS loc, 0 AS runtime, COALESCE(ar.id, 0) AS artist_id, COALESCE(ar.name, '') AS artist, count() AS children,
			0 AS size, 0 AS bitrate
			FROM albums e INNER JOIN dbalbums d ON d.id = e.dbalbum_id LEFT JOIN dbartists ar ON ar.id = ` + jfAlbumArtist + `
			WHERE EXISTS (SELECT 1 FROM album_files f WHERE f.album_id = e.id) AND e.listname`,
		Files: "SELECT f.id FROM dbalbum_artists daa INNER JOIN albums e ON e.dbalbum_id = daa.dbalbum_id " +
			"INNER JOIN album_files f ON f.album_id = e.id WHERE daa.dbartist_id = %s",
	},
}

// jfTypeKinds are the kinds by Jellyfin item type (IncludeItemTypes). An
// audiobook is a MusicAlbum of its parts (the web client lists and plays
// only those; Jellyfin itself has no item for a multi-file audiobook), so
// MusicAlbum stands for albums and audiobooks; the library decides which.
var jfTypeKinds = map[string][]byte{
	"movie": {jfMovie}, "series": {jfSeries}, "episode": {jfEpisode}, "musicalbum": {jfAlbum, jfBook}, "audio": {jfTrack},
	"musicartist": {jfArtist}, "audiobook": {jfBookPart}, "folder": {jfBook},
}

// jfQuery narrows an item query.
type jfQuery struct {
	Groups  []libGroup
	Cond    string // on the outer query's columns (x.)
	Args    []any
	Order   string
	Start   int
	Limit   int  // 0: all
	NoTotal bool // the total isn't needed (-1)
}

// jfRows runs an item query: the rows of a page and the total.
func jfRows(k *jfKind, q jfQuery) ([]jfRow, int) {
	groups := make([]libGroup, 0, len(q.Groups))
	for _, g := range q.Groups {
		if g.Kind.Type == k.LibType {
			groups = append(groups, g)
		}
	}

	in, args := listsOf(groups)

	inner := k.SQL + in
	if k.Code == jfArtist {
		inner += " GROUP BY 1"
	}

	args = append(args, q.Args...)
	where := " WHERE 1"

	if q.Cond != "" {
		where += q.Cond
	}

	total := -1
	if !q.NoTotal {
		total = database.Getdatarow[int](false, "SELECT count() FROM ("+inner+") x"+where, args...)
	}

	order := q.Order
	if order == "" {
		order = "x.name COLLATE NOCASE"
	}

	limit := ""
	if q.Limit > 0 {
		limit = " LIMIT " + strconv.Itoa(q.Limit) + " OFFSET " + strconv.Itoa(max(q.Start, 0))
	} else if q.Start > 0 {
		limit = " LIMIT -1 OFFSET " + strconv.Itoa(q.Start)
	}

	rows := database.StructscanT[jfRow](false, uint(min(max(q.Limit, 16), 1000)), //nolint:gosec // bounded
		"SELECT * FROM ("+inner+") x"+where+" ORDER BY "+order+", x.id"+limit, args...)

	return rows, total
}

// jfFilesOf is the SQL of the file IDs of an item, with idExpr as its ID.
func (k *jfKind) filesOf(idExpr string) string { return fmt.Sprintf(k.Files, idExpr) }

// jfPlayedCond is the condition "the account played the item" on x.id,
// with its arguments; folders when all their files are played.
func (k *jfKind) playedCond(user string) (string, []any) {
	w := strconv.FormatFloat(database.Watched, 'f', 2, 64)
	if !k.Folder {
		return "EXISTS (SELECT 1 FROM play_history h WHERE h.username = ? AND h.kind = ? AND h.completion >= " + w +
			" AND h.file_id IN (" + k.filesOf("x.id") + "))", []any{user, k.PlayKind}
	}

	return "NOT EXISTS (SELECT 1 FROM (" + k.filesOf("x.id") + ") ff WHERE NOT EXISTS (SELECT 1 FROM play_history h " +
		"WHERE h.username = ? AND h.kind = ? AND h.file_id = ff.id AND h.completion >= " + w + "))", []any{user, k.PlayKind}
}

// resumableCond is "the account stopped in the middle of the item".
func (k *jfKind) resumableCond(user string) (string, []any) {
	return "EXISTS (SELECT 1 FROM play_history h WHERE h.username = ? AND h.kind = ? AND h.position > 0 AND h.completion < " +
		strconv.FormatFloat(database.Watched, 'f', 2, 64) + " AND h.file_id IN (" + k.filesOf("x.id") + "))", []any{user, k.PlayKind}
}

// jfSortColumns are the columns of Jellyfin's sort names.
var jfSortColumns = map[string]string{
	"sortname": "x.name COLLATE NOCASE", "name": "x.name COLLATE NOCASE", "datecreated": "x.created",
	"premieredate": "x.premiere", "productionyear": "x.year", "communityrating": "x.rating", "criticrating": "x.rating",
	"random": "random()", "indexnumber": "x.idx", "parentindexnumber": "x.pidx", "runtime": "x.runtime",
	"albumartist": "x.artist COLLATE NOCASE", "artist": "x.artist COLLATE NOCASE", "album": "x.parent_name COLLATE NOCASE",
	"seriessortname": "x.parent_name COLLATE NOCASE", "dateplayed": "x.created", "playcount": "x.created",
}

// jfOrder turns SortBy and SortOrder (comma lists) into an ORDER BY.
func jfOrder(sortBy, sortOrder string, def string) string {
	orders := strings.Split(sortOrder, ",")

	var out []string

	for i, s := range strings.Split(sortBy, ",") {
		col := jfSortColumns[strings.ToLower(strings.TrimSpace(s))]
		if col == "" {
			continue
		}

		dir := ""
		if i < len(orders) && strings.EqualFold(strings.TrimSpace(orders[i]), "descending") ||
			i >= len(orders) && len(orders) > 0 && strings.EqualFold(strings.TrimSpace(orders[0]), "descending") {
			dir = " DESC"
		}

		if col != "random()" {
			col += dir
		}

		out = append(out, col)
	}

	if len(out) == 0 {
		return def
	}

	return strings.Join(out, ", ")
}

// jfSeasonRow is a season of a series with files.
type jfSeasonRow struct {
	Season int    `db:"season"`
	Files  int    `db:"files"`
	Name   string `db:"name"`
	DbID   uint64 `db:"db_id"`
}

// jfSeasons returns the seasons with files of a series the account may
// see (nil: none or not allowed).
func jfSeasons(groups []libGroup, seriesID uint64) []jfSeasonRow {
	in, args := listsOf(groups)

	return database.StructscanT[jfSeasonRow](false, 32,
		"SELECT CAST(COALESCE(de.season, '0') AS INTEGER) AS season, count() AS files, COALESCE(d.seriename, '') AS name, "+
			"d.id AS db_id FROM serie_episode_files f INNER JOIN dbserie_episodes de ON de.id = f.dbserie_episode_id "+
			"INNER JOIN series s ON s.id = f.serie_id INNER JOIN dbseries d ON d.id = s.dbserie_id "+
			"WHERE f.serie_id = ? AND s.listname"+in+" GROUP BY 1 ORDER BY 1", append([]any{seriesID}, args...)...)
}

// jfSeasonFiles is the SQL of the files of a season (series ID, season).
const jfSeasonFiles = "SELECT f.id FROM serie_episode_files f INNER JOIN dbserie_episodes de ON de.id = f.dbserie_episode_id " +
	"WHERE f.serie_id = ? AND CAST(COALESCE(de.season, '0') AS INTEGER) = ?"

// jfDate turns a stored date ("2011-09-19 00:00:00 +0000 UTC",
// "2016-04-27") into Jellyfin's format.
func jfDate(s string) string {
	if len(s) < 10 || s[4] != '-' || s[7] != '-' {
		return ""
	}

	return s[:10] + "T00:00:00.0000000Z"
}

// jfCreated turns a row's created_at into Jellyfin's format.
func jfCreated(s string) string {
	if len(s) >= 19 && s[4] == '-' {
		return s[:10] + "T" + s[11:19] + ".0000000Z"
	}

	return ""
}

func intPtr(n int) *int { return &n }

func splitGenres(s string) []string {
	var out []string

	for g := range strings.SplitSeq(s, ",") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}

	return out
}

// ticks are Jellyfin's time unit: 100 ns.
func ticks(seconds float64) int64 { return int64(seconds * 1e7) }

// item builds the item of a row; lang's translations are put in by
// jfTranslate.
func (k *jfKind) item(r *jfRow, sid string) jfItem {
	it := jfItem{
		Name: r.Name, ServerID: sid, ID: jfID(k.Code, r.ID), Type: k.Type, IsFolder: k.Folder, LocationType: "FileSystem",
		DateCreated: jfCreated(r.Created), PremiereDate: jfDate(r.Premiere), Overview: r.Overview, Genres: splitGenres(r.Genres),
		CommunityRating: r.Rating, RunTimeTicks: ticks(r.Runtime), ProductionYear: r.Year, ImageTags: map[string]string{},
		BackdropImageTags: []string{}, PlayAccess: "Full",
	}

	if r.Original != "" && r.Original != r.Name {
		it.OriginalTitle = r.Original
	}

	ids := map[string]string{}
	if strings.HasPrefix(r.Imdb, "tt") {
		ids["Imdb"] = r.Imdb
	}

	if r.Tmdb != "" && r.Tmdb != "0" {
		ids["Tmdb"] = r.Tmdb
	}

	if r.Tvdb != "" && r.Tvdb != "0" {
		ids["Tvdb"] = r.Tvdb
	}

	if len(ids) > 0 {
		it.ProviderIDs = ids
	}

	switch k.Code {
	case jfMovie:
		it.MediaType, it.VideoType, it.PrimaryImageAspectRatio = "Video", "VideoFile", 2.0/3
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")
		it.Container = strings.TrimPrefix(strings.ToLower(path.Ext(r.Location)), ".")

		if r.Backdrop != "" {
			it.BackdropImageTags = []string{jfImageTag(it.ID, "Backdrop")}
		}
	case jfSeries:
		it.PrimaryImageAspectRatio, it.RecursiveItemCount = 2.0/3, r.Children
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")

		if r.Backdrop != "" {
			it.BackdropImageTags = []string{jfImageTag(it.ID, "Backdrop")}
		}
	case jfEpisode:
		series := jfID(jfSeries, r.ParentID)
		it.MediaType, it.VideoType, it.PrimaryImageAspectRatio = "Video", "VideoFile", 16.0/9
		it.IndexNumber, it.ParentIndexNumber = intPtr(r.Idx), intPtr(r.Pidx)
		it.SeriesName, it.SeriesID = r.ParentName, series
		it.SeasonID, it.ParentID = jfID(jfSeason, r.ParentID<<16|uint64(r.Pidx)), jfID(jfSeason, r.ParentID<<16|uint64(r.Pidx))
		it.SeasonName = jfSeasonName(r.Pidx)
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")
		it.SeriesPrimaryImageTag = jfImageTag(series, "Primary")
		it.ParentBackdropItemID, it.ParentBackdropImageTags = series, []string{jfImageTag(series, "Backdrop")}
		it.Container = strings.TrimPrefix(strings.ToLower(path.Ext(r.Location)), ".")
	case jfAlbum:
		it.PrimaryImageAspectRatio, it.ChildCount, it.RecursiveItemCount = 1, r.Children, r.Children
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")

		if r.Name == "" {
			it.Name = path.Base(path.Dir(strings.ReplaceAll(jfFirstLocation("album_files", "album_id", r.ID), "\\", "/")))
		}

		if r.ArtistID > 0 {
			a := jfNameID{Name: r.Artist, ID: jfID(jfArtist, r.ArtistID)}
			it.AlbumArtist, it.Artists, it.AlbumArtists, it.ArtistItems = r.Artist, []string{r.Artist}, []jfNameID{a}, []jfNameID{a}
		}
	case jfTrack:
		album := jfID(jfAlbum, r.ParentID)
		it.MediaType, it.PrimaryImageAspectRatio = "Audio", 1
		it.IndexNumber, it.ParentIndexNumber = intPtr(r.Idx), intPtr(max(r.Pidx, 1))
		it.Album, it.AlbumID, it.ParentID = r.ParentName, album, album
		it.AlbumPrimaryImageTag = jfImageTag(album, "Primary")
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")
		it.Container = strings.TrimPrefix(strings.ToLower(path.Ext(r.Location)), ".")

		if it.Name == "" {
			it.Name = subTrackNumberRe.ReplaceAllString(fileTitle(r.Location), "")
		}

		if r.ArtistID > 0 {
			a := jfNameID{Name: r.Artist, ID: jfID(jfArtist, r.ArtistID)}
			it.AlbumArtist, it.Artists, it.AlbumArtists, it.ArtistItems = r.Artist, []string{r.Artist}, []jfNameID{a}, []jfNameID{a}
		}
	case jfArtist:
		it.PrimaryImageAspectRatio, it.ChildCount = 1, r.Children
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")
	case jfBook:
		it.PrimaryImageAspectRatio, it.ChildCount, it.RecursiveItemCount = 1, r.Children, r.Children
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")

		// The author as the album artist (the web client's album view needs
		// the artist lists); the link leads back to the audiobook: authors
		// aren't music artists.
		if r.Artist != "" {
			a := jfNameID{Name: r.Artist, ID: it.ID}
			it.AlbumArtist, it.Artists, it.AlbumArtists, it.ArtistItems = r.Artist, []string{r.Artist}, []jfNameID{a}, []jfNameID{a}
		}
	case jfBookPart:
		book := jfID(jfBook, r.ParentID)
		it.Name = fileTitle(r.Location)
		it.MediaType, it.PrimaryImageAspectRatio, it.ParentID = "Audio", 1, book
		it.IndexNumber, it.Album, it.AlbumID = intPtr(r.Idx), r.ParentName, book
		it.ImageTags["Primary"] = jfImageTag(it.ID, "Primary")
		it.AlbumPrimaryImageTag = jfImageTag(book, "Primary")
		it.Container = strings.TrimPrefix(strings.ToLower(path.Ext(r.Location)), ".")

		if r.Artist != "" {
			a := jfNameID{Name: r.Artist, ID: book}
			it.AlbumArtist, it.Artists, it.AlbumArtists, it.ArtistItems = r.Artist, []string{r.Artist}, []jfNameID{a}, []jfNameID{a}
		}
	}

	it.GenreItems = jfGenreItems(it.Genres)
	it.SortName = strings.ToLower(it.Name)
	it.Etag = strconv.FormatUint(uint64(crc32.ChecksumIEEE([]byte(it.ID+r.Created))), 16)

	return it
}

// jfSeasonName is the name of a season.
func jfSeasonName(n int) string {
	if n == 0 {
		return "Specials"
	}

	return "Season " + strconv.Itoa(n)
}

// jfFirstLocation is the location of an entry's first file.
func jfFirstLocation(table, fk string, id uint64) string {
	return database.Getdatarow[string](false, "SELECT location FROM "+table+" WHERE "+fk+" = ? ORDER BY id LIMIT 1", &id)
}

// jfSeasonItem builds the item of a season.
func jfSeasonItem(seriesID uint64, s *jfSeasonRow, sid string) jfItem {
	series := jfID(jfSeries, seriesID)
	id := jfID(jfSeason, seriesID<<16|uint64(s.Season)) //nolint:gosec // season numbers are small

	return jfItem{
		Name: jfSeasonName(s.Season), ServerID: sid, ID: id, Type: "Season", IsFolder: true, LocationType: "FileSystem",
		IndexNumber: intPtr(s.Season), SeriesName: s.Name, SeriesID: series, ParentID: series, ChildCount: s.Files,
		RecursiveItemCount: s.Files, PrimaryImageAspectRatio: 2.0 / 3, PlayAccess: "Full",
		ImageTags: map[string]string{"Primary": jfImageTag(id, "Primary")}, SeriesPrimaryImageTag: jfImageTag(series, "Primary"),
		BackdropImageTags: []string{}, ParentBackdropItemID: series, ParentBackdropImageTags: []string{jfImageTag(series, "Backdrop")},
		SortName: strings.ToLower(jfSeasonName(s.Season)),
	}
}

// jfItems makes the items of rows with the translations into lang (no
// account state: jfBuild adds it).
func jfItems(k *jfKind, rows []jfRow, sid, lang string) []jfItem {
	items := make([]jfItem, len(rows))
	for i := range rows {
		items[i] = k.item(&rows[i], sid)
	}

	jfTranslate(k, rows, items, lang)

	return items
}

// jfTranslate puts the translations into lang into the items of rows, and
// names the unknown artist.
func jfTranslate(k *jfKind, rows []jfRow, items []jfItem, lang string) {
	if len(rows) == 0 {
		return
	}

	if k.Code == jfArtist || k.Code == jfAlbum || k.Code == jfTrack {
		unknown := trText(lang, t("Unknown artist"))

		for i := range rows {
			if rows[i].ArtistID != 0 {
				continue
			}

			rows[i].Artist = unknown
			a := jfNameID{Name: unknown, ID: jfID(jfArtist, 0)}
			items[i].AlbumArtist, items[i].Artists, items[i].AlbumArtists, items[i].ArtistItems = unknown, []string{unknown}, []jfNameID{a}, []jfNameID{a}

			if k.Code == jfArtist {
				rows[i].Name, items[i].Name, items[i].SortName = unknown, unknown, "\uffff"
				items[i].AlbumArtist, items[i].Artists, items[i].AlbumArtists, items[i].ArtistItems = "", nil, nil, nil
			}
		}
	}

	ids := func(f func(*jfRow) uint64) []uint {
		out := make([]uint, 0, len(rows))
		for i := range rows {
			out = append(out, uint(f(&rows[i])))
		}

		return out
	}

	switch k.Code {
	case jfMovie, jfSeries, jfBook:
		mediaType := map[byte]string{jfMovie: "dbmovie", jfSeries: "dbserie", jfBook: "dbaudiobook"}[k.Code]
		trs := database.TranslationsIn(mediaType, lang, ids(func(r *jfRow) uint64 { return r.DbID }))

		for i := range rows {
			tr := trs[uint(rows[i].DbID)]
			if v := tr["title"]; v != "" {
				if items[i].OriginalTitle == "" && v != items[i].Name {
					items[i].OriginalTitle = items[i].Name
				}

				items[i].Name, items[i].SortName = v, strings.ToLower(v)
			}

			if v := firstText(tr["overview"], tr["description"]); v != "" {
				items[i].Overview = v
			}
		}
	case jfEpisode:
		eps := database.TranslationsIn("dbserie_episode", lang, ids(func(r *jfRow) uint64 { return r.DbID }))
		series := database.TranslationsIn("dbserie", lang, ids(func(r *jfRow) uint64 { return r.Db2 }))

		for i := range rows {
			items[i].Name = firstText(eps[uint(rows[i].DbID)]["title"], items[i].Name)
			items[i].Overview = firstText(eps[uint(rows[i].DbID)]["overview"], items[i].Overview)
			items[i].SeriesName = firstText(series[uint(rows[i].Db2)]["title"], items[i].SeriesName)
		}
	case jfBookPart:
		books := database.TranslationsIn("dbaudiobook", lang, ids(func(r *jfRow) uint64 { return r.Db2 }))

		for i := range rows {
			items[i].Album = firstText(books[uint(rows[i].Db2)]["title"], items[i].Album)
		}
	}
}

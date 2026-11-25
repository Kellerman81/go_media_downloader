package api

import (
	"crypto/md5" //nolint:gosec // the Subsonic token login is defined with MD5
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// The Subsonic API (/subsonic/rest/<method>.view) serves the music of the
// account's music groups to Subsonic apps: artists (the album's first
// artist), albums (list entries with files) and songs (album_files).
// IDs: "ar" + dbartists.id, "al" + albums.id, "tr" + album_files.id.

const (
	subsonicVersion = "1.16.1"
	subsonicXmlns   = "http://subsonic.org/restapi"
)

// Subsonic error codes.
const (
	subErrGeneric  = 0
	subErrMissing  = 10
	subErrLogin    = 40
	subErrTokenLDA = 41
	subErrNoAccess = 50 // user is not authorized for the given operation
	subErrNotFound = 70
)

type subResp struct {
	XMLName       xml.Name `xml:"subsonic-response" json:"-"`
	Xmlns         string   `xml:"xmlns,attr" json:"-"`
	Status        string   `xml:"status,attr" json:"status"`
	Version       string   `xml:"version,attr" json:"version"`
	Type          string   `xml:"type,attr" json:"type"`
	ServerVersion string   `xml:"serverVersion,attr" json:"serverVersion"`
	OpenSubsonic  bool     `xml:"openSubsonic,attr" json:"openSubsonic"`

	Error         *subError      `xml:"error,omitempty" json:"error,omitempty"`
	License       *subLicense    `xml:"license,omitempty" json:"license,omitempty"`
	Extensions    []subExtension `xml:"openSubsonicExtensions,omitempty" json:"openSubsonicExtensions,omitempty"`
	MusicFolders  *subFolders    `xml:"musicFolders,omitempty" json:"musicFolders,omitempty"`
	Indexes       *subIndexes    `xml:"indexes,omitempty" json:"indexes,omitempty"`
	Artists       *subIndexes    `xml:"artists,omitempty" json:"artists,omitempty"`
	Artist        *subArtist     `xml:"artist,omitempty" json:"artist,omitempty"`
	ArtistInfo    *subArtistInfo `xml:"artistInfo,omitempty" json:"artistInfo,omitempty"`
	ArtistInfo2   *subArtistInfo `xml:"artistInfo2,omitempty" json:"artistInfo2,omitempty"`
	Album         *subAlbum      `xml:"album,omitempty" json:"album,omitempty"`
	AlbumInfo     *subAlbumInfo  `xml:"albumInfo,omitempty" json:"albumInfo,omitempty"`
	Song          *subChild      `xml:"song,omitempty" json:"song,omitempty"`
	Directory     *subDirectory  `xml:"directory,omitempty" json:"directory,omitempty"`
	AlbumList     *subChildList  `xml:"albumList,omitempty" json:"albumList,omitempty"`
	AlbumList2    *subAlbumList  `xml:"albumList2,omitempty" json:"albumList2,omitempty"`
	RandomSongs   *subSongs      `xml:"randomSongs,omitempty" json:"randomSongs,omitempty"`
	SongsByGenre  *subSongs      `xml:"songsByGenre,omitempty" json:"songsByGenre,omitempty"`
	TopSongs      *subSongs      `xml:"topSongs,omitempty" json:"topSongs,omitempty"`
	SimilarSongs  *subSongs      `xml:"similarSongs,omitempty" json:"similarSongs,omitempty"`
	SimilarSongs2 *subSongs      `xml:"similarSongs2,omitempty" json:"similarSongs2,omitempty"`
	Genres        *subGenres     `xml:"genres,omitempty" json:"genres,omitempty"`
	SearchResult2 *subSearch     `xml:"searchResult2,omitempty" json:"searchResult2,omitempty"`
	SearchResult3 *subSearch     `xml:"searchResult3,omitempty" json:"searchResult3,omitempty"`
	Starred       *subSearch     `xml:"starred,omitempty" json:"starred,omitempty"`
	Starred2      *subSearch     `xml:"starred2,omitempty" json:"starred2,omitempty"`
	Playlists     *subPlaylists  `xml:"playlists,omitempty" json:"playlists,omitempty"`
	Playlist      *subPlaylist   `xml:"playlist,omitempty" json:"playlist,omitempty"`
	NowPlaying    *subEmptyList  `xml:"nowPlaying,omitempty" json:"nowPlaying,omitempty"`
	ScanStatus    *subScanStatus `xml:"scanStatus,omitempty" json:"scanStatus,omitempty"`
	User          *subUser       `xml:"user,omitempty" json:"user,omitempty"`
	Bookmarks     *subEmptyList  `xml:"bookmarks,omitempty" json:"bookmarks,omitempty"`
	Lyrics        *subLyrics     `xml:"lyrics,omitempty" json:"lyrics,omitempty"`
	LyricsList    *subLyricsList `xml:"lyricsList,omitempty" json:"lyricsList,omitempty"`
	Podcasts      *subEmptyList  `xml:"podcasts,omitempty" json:"podcasts,omitempty"`
	RadioStations *subEmptyList  `xml:"internetRadioStations,omitempty" json:"internetRadioStations,omitempty"`
}

type subError struct {
	Code    int    `xml:"code,attr" json:"code"`
	Message string `xml:"message,attr" json:"message"`
}

type subLicense struct {
	Valid bool `xml:"valid,attr" json:"valid"`
}

type subExtension struct {
	Name     string `xml:"name,attr" json:"name"`
	Versions []int  `xml:"versions" json:"versions"`
}

type subFolder struct {
	ID   int    `xml:"id,attr" json:"id"`
	Name string `xml:"name,attr" json:"name"`
}

type subFolders struct {
	Folders []subFolder `xml:"musicFolder" json:"musicFolder"`
}

type subIndexes struct {
	LastModified    int64      `xml:"lastModified,attr,omitempty" json:"lastModified,omitempty"`
	IgnoredArticles string     `xml:"ignoredArticles,attr" json:"ignoredArticles"`
	Index           []subIndex `xml:"index" json:"index"`
}

type subIndex struct {
	Name    string      `xml:"name,attr" json:"name"`
	Artists []subArtist `xml:"artist" json:"artist"`
}

type subArtist struct {
	ID         string     `xml:"id,attr" json:"id"`
	Name       string     `xml:"name,attr" json:"name"`
	CoverArt   string     `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	AlbumCount int        `xml:"albumCount,attr,omitempty" json:"albumCount,omitempty"`
	Albums     []subAlbum `xml:"album,omitempty" json:"album,omitempty"`
}

type subArtistInfo struct {
	Biography      string `xml:"biography,omitempty" json:"biography,omitempty"`
	LargeImageURL  string `xml:"largeImageUrl,omitempty" json:"largeImageUrl,omitempty"`
	MediumImageURL string `xml:"mediumImageUrl,omitempty" json:"mediumImageUrl,omitempty"`
}

type subAlbumInfo struct {
	Notes string `xml:"notes,omitempty" json:"notes,omitempty"`
}

type subAlbum struct {
	ID        string     `xml:"id,attr" json:"id"`
	Name      string     `xml:"name,attr" json:"name"`
	Title     string     `xml:"title,attr,omitempty" json:"title,omitempty"`
	Artist    string     `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	ArtistID  string     `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	CoverArt  string     `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	SongCount int        `xml:"songCount,attr" json:"songCount"`
	Duration  int        `xml:"duration,attr" json:"duration"`
	Created   string     `xml:"created,attr,omitempty" json:"created,omitempty"`
	Year      int        `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre     string     `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	PlayCount int        `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	Songs     []subChild `xml:"song,omitempty" json:"song,omitempty"`
}

type subChild struct {
	ID          string `xml:"id,attr" json:"id"`
	Parent      string `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	IsDir       bool   `xml:"isDir,attr" json:"isDir"`
	Title       string `xml:"title,attr" json:"title"`
	Album       string `xml:"album,attr,omitempty" json:"album,omitempty"`
	Artist      string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Track       int    `xml:"track,attr,omitempty" json:"track,omitempty"`
	Year        int    `xml:"year,attr,omitempty" json:"year,omitempty"`
	Genre       string `xml:"genre,attr,omitempty" json:"genre,omitempty"`
	CoverArt    string `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Size        int64  `xml:"size,attr,omitempty" json:"size,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty" json:"contentType,omitempty"`
	Suffix      string `xml:"suffix,attr,omitempty" json:"suffix,omitempty"`
	Duration    int    `xml:"duration,attr,omitempty" json:"duration,omitempty"`
	BitRate     int    `xml:"bitRate,attr,omitempty" json:"bitRate,omitempty"`
	Path        string `xml:"path,attr,omitempty" json:"path,omitempty"`
	PlayCount   int    `xml:"playCount,attr,omitempty" json:"playCount,omitempty"`
	DiscNumber  int    `xml:"discNumber,attr,omitempty" json:"discNumber,omitempty"`
	Created     string `xml:"created,attr,omitempty" json:"created,omitempty"`
	AlbumID     string `xml:"albumId,attr,omitempty" json:"albumId,omitempty"`
	ArtistID    string `xml:"artistId,attr,omitempty" json:"artistId,omitempty"`
	Type        string `xml:"type,attr,omitempty" json:"type,omitempty"`
	MediaType   string `xml:"mediaType,attr,omitempty" json:"mediaType,omitempty"`
	SongCount   int    `xml:"songCount,attr,omitempty" json:"songCount,omitempty"`
}

type subDirectory struct {
	ID       string     `xml:"id,attr" json:"id"`
	Parent   string     `xml:"parent,attr,omitempty" json:"parent,omitempty"`
	Name     string     `xml:"name,attr" json:"name"`
	Children []subChild `xml:"child" json:"child"`
}

type subChildList struct {
	Albums []subChild `xml:"album" json:"album"`
}

type subAlbumList struct {
	Albums []subAlbum `xml:"album" json:"album"`
}

type subSongs struct {
	Songs []subChild `xml:"song" json:"song"`
}

type subGenre struct {
	Name       string `xml:",chardata" json:"value"`
	SongCount  int    `xml:"songCount,attr" json:"songCount"`
	AlbumCount int    `xml:"albumCount,attr" json:"albumCount"`
}

type subGenres struct {
	Genres []subGenre `xml:"genre" json:"genre"`
}

type subSearch struct {
	Artists []subArtist `xml:"artist" json:"artist"`
	Albums  []subAlbum  `xml:"album" json:"album"`
	Songs   []subChild  `xml:"song" json:"song"`
}

type subEmptyList struct{}

type subLyrics struct {
	Artist string `xml:"artist,attr,omitempty" json:"artist,omitempty"`
	Title  string `xml:"title,attr,omitempty" json:"title,omitempty"`
	Text   string `xml:",chardata" json:"value"`
}

type subLyricsLine struct {
	Value string `xml:",chardata" json:"value"`
}

type subStructuredLyrics struct {
	Lang          string          `xml:"lang,attr" json:"lang"`
	Synced        bool            `xml:"synced,attr" json:"synced"`
	DisplayArtist string          `xml:"displayArtist,attr,omitempty" json:"displayArtist,omitempty"`
	DisplayTitle  string          `xml:"displayTitle,attr,omitempty" json:"displayTitle,omitempty"`
	Lines         []subLyricsLine `xml:"line" json:"line"`
}

type subLyricsList struct {
	Structured []subStructuredLyrics `xml:"structuredLyrics" json:"structuredLyrics"`
}

type subPlaylist struct {
	ID        string     `xml:"id,attr" json:"id"`
	Name      string     `xml:"name,attr" json:"name"`
	Owner     string     `xml:"owner,attr" json:"owner"`
	Public    bool       `xml:"public,attr" json:"public"`
	SongCount int        `xml:"songCount,attr" json:"songCount"`
	Duration  int        `xml:"duration,attr" json:"duration"`
	Created   string     `xml:"created,attr" json:"created"`
	Changed   string     `xml:"changed,attr" json:"changed"`
	CoverArt  string     `xml:"coverArt,attr,omitempty" json:"coverArt,omitempty"`
	Entries   []subChild `xml:"entry,omitempty" json:"entry,omitempty"`
}

type subPlaylists struct {
	Playlists []subPlaylist `xml:"playlist" json:"playlist"`
}

type subScanStatus struct {
	Scanning bool `xml:"scanning,attr" json:"scanning"`
	Count    int  `xml:"count,attr" json:"count"`
}

type subUser struct {
	Username          string `xml:"username,attr" json:"username"`
	ScrobblingEnabled bool   `xml:"scrobblingEnabled,attr" json:"scrobblingEnabled"`
	AdminRole         bool   `xml:"adminRole,attr" json:"adminRole"`
	SettingsRole      bool   `xml:"settingsRole,attr" json:"settingsRole"`
	DownloadRole      bool   `xml:"downloadRole,attr" json:"downloadRole"`
	UploadRole        bool   `xml:"uploadRole,attr" json:"uploadRole"`
	PlaylistRole      bool   `xml:"playlistRole,attr" json:"playlistRole"`
	CoverArtRole      bool   `xml:"coverArtRole,attr" json:"coverArtRole"`
	CommentRole       bool   `xml:"commentRole,attr" json:"commentRole"`
	PodcastRole       bool   `xml:"podcastRole,attr" json:"podcastRole"`
	StreamRole        bool   `xml:"streamRole,attr" json:"streamRole"`
	JukeboxRole       bool   `xml:"jukeboxRole,attr" json:"jukeboxRole"`
	ShareRole         bool   `xml:"shareRole,attr" json:"shareRole"`
	Folders           []int  `xml:"folder" json:"folder"`
}

// subParam reads a parameter from the query or a posted form.
func subParam(c *gin.Context, name string) string {
	if v, ok := c.GetQuery(name); ok {
		return v
	}

	return c.PostForm(name)
}

func subInt(c *gin.Context, name string, def int) int {
	if n, err := strconv.Atoi(subParam(c, name)); err == nil {
		return n
	}

	return def
}

func newSubResp() *subResp {
	return &subResp{
		Xmlns: subsonicXmlns, Status: "ok", Version: subsonicVersion, Type: "gomediadownloader",
		ServerVersion: "1", OpenSubsonic: true,
	}
}

// writeSub sends the response as XML, or as JSON with f=json.
func writeSub(c *gin.Context, r *subResp) {
	switch subParam(c, "f") {
	case "json", "jsonp":
		out, err := json.Marshal(map[string]*subResp{"subsonic-response": r})
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		if cb := subParam(c, "callback"); subParam(c, "f") == "jsonp" && subCallbackRe.MatchString(cb) {
			c.Data(http.StatusOK, "text/javascript; charset=utf-8", []byte(cb+"("+string(out)+");"))
			return
		}

		c.Data(http.StatusOK, "application/json; charset=utf-8", out)
	default:
		out, err := xml.Marshal(r)
		if err != nil {
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		c.Data(http.StatusOK, "text/xml; charset=utf-8", append([]byte(xml.Header), out...))
	}
}

var subCallbackRe = regexp.MustCompile(`^[A-Za-z_$][A-Za-z0-9_$.]{0,63}$`)

func subFail(c *gin.Context, code int, msg string) {
	r := newSubResp()
	r.Status, r.Error = "failed", &subError{Code: code, Message: msg}
	writeSub(c, r)
}

// subAuth logs the app in: u with p (plain or "enc:" hex), or t and s
// (md5(password + salt)), which only works for the admin login: the
// accounts' passwords are stored as hashes.
func subAuth(c *gin.Context) bool {
	user := subParam(c, "u")
	if user == "" {
		subFail(c, subErrMissing, "Required parameter is missing: u")
		return false
	}

	if tok := subParam(c, "t"); tok != "" {
		salt := subParam(c, "s")
		sum := md5.Sum([]byte(webPassword() + salt)) //nolint:gosec // defined by the protocol

		if user != DefaultUsername || salt == "" ||
			subtle.ConstantTimeCompare([]byte(strings.ToLower(tok)), []byte(hex.EncodeToString(sum[:]))) != 1 {
			if user != DefaultUsername {
				subFail(c, subErrTokenLDA, "Token login is not supported for this account, use the password login")
			} else {
				subFail(c, subErrLogin, "Wrong username or password")
			}

			return false
		}

		setClientSession(c, adminAccess)

		return true
	}

	pass := subParam(c, "p")
	if enc, ok := strings.CutPrefix(pass, "enc:"); ok {
		b, err := hex.DecodeString(enc)
		if err != nil {
			subFail(c, subErrLogin, "Wrong username or password")
			return false
		}

		pass = string(b)
	}

	acc, err := clientAuthenticate(c, user, pass)
	if errors.Is(err, errClientNoApps) {
		subFail(c, subErrNoAccess, "This account may not use apps and devices: an admin can allow it under Users")
		return false
	}

	if err != nil {
		subFail(c, subErrLogin, "Wrong username or password")
		return false
	}

	setClientSession(c, acc)

	return true
}

// HandleSubsonic dispatches a Subsonic call (/subsonic/rest/:method).
func HandleSubsonic(c *gin.Context) {
	method := strings.TrimSuffix(strings.TrimPrefix(c.Param("method"), "/"), ".view")

	if !subAuth(c) {
		return
	}

	h, ok := subsonicMethods[method]
	if !ok {
		subFail(c, subErrNotFound, "Unknown method: "+method)
		return
	}

	h(c)
}

var subsonicMethods map[string]func(*gin.Context)

func init() {
	ok := func(c *gin.Context) { writeSub(c, newSubResp()) }
	with := func(set func(*subResp)) func(*gin.Context) {
		return func(c *gin.Context) {
			r := newSubResp()
			set(r)
			writeSub(c, r)
		}
	}

	subsonicMethods = map[string]func(*gin.Context){
		"ping":       ok,
		"getLicense": with(func(r *subResp) { r.License = &subLicense{Valid: true} }),
		"getOpenSubsonicExtensions": with(func(r *subResp) {
			r.Extensions = []subExtension{{Name: "formPost", Versions: []int{1}}, {Name: "songLyrics", Versions: []int{1}}}
		}),
		"getMusicFolders":   subMusicFolders,
		"getIndexes":        func(c *gin.Context) { subArtistIndex(c, false) },
		"getArtists":        func(c *gin.Context) { subArtistIndex(c, true) },
		"getArtist":         subGetArtist,
		"getArtistInfo":     func(c *gin.Context) { subGetArtistInfo(c, false) },
		"getArtistInfo2":    func(c *gin.Context) { subGetArtistInfo(c, true) },
		"getAlbum":          subGetAlbum,
		"getAlbumInfo":      with(func(r *subResp) { r.AlbumInfo = &subAlbumInfo{} }),
		"getAlbumInfo2":     with(func(r *subResp) { r.AlbumInfo = &subAlbumInfo{} }),
		"getSong":           subGetSong,
		"getMusicDirectory": subGetDirectory,
		"getAlbumList":      func(c *gin.Context) { subGetAlbumList(c, false) },
		"getAlbumList2":     func(c *gin.Context) { subGetAlbumList(c, true) },
		"getRandomSongs":    subGetRandomSongs,
		"getSongsByGenre":   subGetSongsByGenre,
		"getGenres":         subGetGenres,
		"search2":           func(c *gin.Context) { subSearchAll(c, false) },
		"search3":           func(c *gin.Context) { subSearchAll(c, true) },
		"getCoverArt":       subGetCoverArt,
		"stream":            subStream,
		"download":          subDownload,
		"scrobble":          subScrobble,
		"getStarred":        func(c *gin.Context) { subGetStarred(c, false) },
		"getStarred2":       func(c *gin.Context) { subGetStarred(c, true) },
		"star":              func(c *gin.Context) { subStar(c, true) },
		"unstar":            func(c *gin.Context) { subStar(c, false) },
		"setRating":         ok,
		"getPlaylists":      subGetPlaylists,
		"getPlaylist":       subGetPlaylist,
		"createPlaylist":    subCreatePlaylist,
		"updatePlaylist":    subUpdatePlaylist,
		"deletePlaylist":    subDeletePlaylist,
		"getNowPlaying":     with(func(r *subResp) { r.NowPlaying = &subEmptyList{} }),
		"getScanStatus":     with(func(r *subResp) { r.ScanStatus = &subScanStatus{} }),
		"getTopSongs":       with(func(r *subResp) { r.TopSongs = &subSongs{} }),
		"getSimilarSongs":   with(func(r *subResp) { r.SimilarSongs = &subSongs{} }),
		"getSimilarSongs2":  with(func(r *subResp) { r.SimilarSongs2 = &subSongs{} }),
		"getBookmarks":      with(func(r *subResp) { r.Bookmarks = &subEmptyList{} }),
		"getLyrics":         subGetLyrics,
		"getLyricsBySongId": subGetLyricsBySongID,
		"getPodcasts":       with(func(r *subResp) { r.Podcasts = &subEmptyList{} }),
		"getInternetRadioStations": with(func(r *subResp) {
			r.RadioStations = &subEmptyList{}
		}),
		"getUser":       subGetUser,
		"savePlayQueue": ok,
	}
}

// subGroups are the account's music groups, or the one musicFolderId
// names (1-based).
func subGroups(c *gin.Context) []libGroup {
	groups := groupsOf(accessOf(c), "album")

	if n := subInt(c, "musicFolderId", 0); n > 0 {
		if n > len(groups) {
			return nil
		}

		return groups[n-1 : n]
	}

	return groups
}

func subMusicFolders(c *gin.Context) {
	lang := requestLanguage(c)
	r := newSubResp()
	r.MusicFolders = &subFolders{Folders: []subFolder{}}

	groups := groupsOf(accessOf(c), "album")
	for i := range groups {
		r.MusicFolders.Folders = append(r.MusicFolders.Folders, subFolder{ID: i + 1, Name: groupTitle(&groups[i], lang)})
	}

	writeSub(c, r)
}

func subGetUser(c *gin.Context) {
	acc := accessOf(c)
	r := newSubResp()
	r.User = &subUser{
		Username: acc.Name, ScrobblingEnabled: true, AdminRole: acc.Admin(), DownloadRole: acc.CanDownload(),
		CoverArtRole: true, StreamRole: true, Folders: []int{},
	}

	for i := range groupsOf(acc, "album") {
		r.User.Folders = append(r.User.Folders, i+1)
	}

	writeSub(c, r)
}

// --- artists ----------------------------------------------------------------

// subArticles are skipped when sorting artists into the index.
const subArticles = "The El La Los Las Le Les Die Der Das"

// The music of a request: the artists, albums and songs of the Jellyfin
// item queries (jfRows), so both APIs show the same. Conditions and orders
// are on the query's columns (x.).

// subQuery runs an item query of the account's music groups without a
// total.
func subQuery(c *gin.Context, code byte, cond string, args []any, order string, start, limit int) ([]jfRow, []jfItem) {
	k := jfKinds[code]
	rows, _ := jfRows(k, jfQuery{Groups: subGroups(c), Cond: cond, Args: args, Order: order, Start: start, Limit: limit, NoTotal: true})

	return rows, jfItems(k, rows, "", requestLanguage(c))
}

func subArtistID(n uint64) string { return "ar" + strconv.FormatUint(n, 10) }
func subAlbumID(n uint64) string  { return "al" + strconv.FormatUint(n, 10) }
func subSongID(n uint64) string   { return "tr" + strconv.FormatUint(n, 10) }

// subArtists returns artists (ID 0: the albums without one).
func subArtists(c *gin.Context, cond string, args []any, start, limit int) []subArtist {
	rows, items := subQuery(c, jfArtist, cond, args, "", start, limit)

	out := make([]subArtist, 0, len(rows))
	for i := range rows {
		id := subArtistID(rows[i].ID)
		out = append(out, subArtist{ID: id, Name: items[i].Name, CoverArt: id, AlbumCount: rows[i].Children})
	}

	return out
}

// subIndexKey is the index letter of an artist name ("#" for the rest).
func subIndexKey(name string) string {
	for art := range strings.FieldsSeq(subArticles) {
		if rest, ok := strings.CutPrefix(name, art+" "); ok && rest != "" {
			name = rest
			break
		}
	}

	for _, r := range name {
		if unicode.IsLetter(r) {
			return strings.ToUpper(string(r))
		}

		break
	}

	return "#"
}

func subArtistIndex(c *gin.Context, id3 bool) {
	idx := &subIndexes{IgnoredArticles: subArticles, Index: []subIndex{}}
	if !id3 {
		idx.LastModified = time.Now().UnixMilli()
	}

	byKey := map[string]int{}

	for _, a := range subArtists(c, "", nil, 0, 0) {
		if !id3 {
			a.AlbumCount, a.CoverArt = 0, ""
		}

		key := subIndexKey(a.Name)

		i, ok := byKey[key]
		if !ok {
			i = len(idx.Index)
			byKey[key] = i
			idx.Index = append(idx.Index, subIndex{Name: key})
		}

		idx.Index[i].Artists = append(idx.Index[i].Artists, a)
	}

	slices.SortFunc(idx.Index, func(a, b subIndex) int { return strings.Compare(a.Name, b.Name) })

	r := newSubResp()
	if id3 {
		r.Artists = idx
	} else {
		r.Indexes = idx
	}

	writeSub(c, r)
}

// subID returns the number of an ID with the prefix.
func subID(c *gin.Context, prefix string) (uint64, bool) {
	s, ok := strings.CutPrefix(subParam(c, "id"), prefix)
	if !ok {
		return 0, false
	}

	n, err := strconv.ParseUint(s, 10, 64)

	return n, err == nil
}

func subGetArtist(c *gin.Context) {
	id, ok := subID(c, "ar")
	if !ok {
		subFail(c, subErrNotFound, "Artist not found")
		return
	}

	art, albums, ok := subArtistAlbums(c, id)
	if !ok {
		subFail(c, subErrNotFound, "Artist not found")
		return
	}

	art.Albums = albums

	r := newSubResp()
	r.Artist = art
	writeSub(c, r)
}

// subArtistAlbums returns an artist with the albums of it the account may
// see; false when there are none.
func subArtistAlbums(c *gin.Context, id uint64) (*subArtist, []subAlbum, bool) {
	albums := subAlbums(c, " AND x.artist_id = ?", []any{id}, "x.year, x.name COLLATE NOCASE", 0, 0)
	if len(albums) == 0 {
		return nil, nil, false
	}

	return &subArtist{ID: subArtistID(id), Name: albums[0].Artist, CoverArt: subArtistID(id), AlbumCount: len(albums)}, albums, true
}

func subGetArtistInfo(c *gin.Context, id3 bool) {
	id, ok := subID(c, "ar")
	if !ok {
		subFail(c, subErrNotFound, "Artist not found")
		return
	}

	bio := database.Getdatarow[string](false, "SELECT COALESCE(bio, '') FROM dbartists WHERE id = ?", &id)
	info := &subArtistInfo{Biography: bio}

	r := newSubResp()
	if id3 {
		r.ArtistInfo2 = info
	} else {
		r.ArtistInfo = info
	}

	writeSub(c, r)
}

// --- albums -----------------------------------------------------------------

// subAlbums returns albums (cond and order on the album query).
func subAlbums(c *gin.Context, cond string, args []any, order string, start, limit int) []subAlbum {
	rows, items := subQuery(c, jfAlbum, cond, args, order, start, limit)

	out := make([]subAlbum, 0, len(rows))
	for i := range rows {
		r := &rows[i]
		id := subAlbumID(r.ID)
		out = append(out, subAlbum{
			ID: id, Name: items[i].Name, Artist: r.Artist, ArtistID: subArtistID(r.ArtistID), CoverArt: id, SongCount: r.Children,
			Duration: int(r.Runtime), Created: subTime(r.Created), Year: r.Year, Genre: firstGenre(r.Genres),
		})
	}

	return out
}

// child is the album as a folder (the non-ID3 calls).
func (a *subAlbum) child() subChild {
	return subChild{
		ID: a.ID, Parent: a.ArtistID, IsDir: true, Title: a.Name, Album: a.Name, Artist: a.Artist, Year: a.Year,
		Genre: a.Genre, CoverArt: a.CoverArt, Created: a.Created, Duration: a.Duration, SongCount: a.SongCount,
		AlbumID: a.ID, ArtistID: a.ArtistID,
	}
}

func firstGenre(genres string) string {
	g, _, _ := strings.Cut(genres, ",")
	return strings.TrimSpace(g)
}

// subTime turns a database time into ISO 8601.
func subTime(s string) string {
	if s == "" {
		return ""
	}

	return opdsTime(s)
}

func subGetAlbum(c *gin.Context) {
	id, ok := subID(c, "al")
	if !ok {
		subFail(c, subErrNotFound, "Album not found")
		return
	}

	albums := subAlbums(c, " AND x.id = ?", []any{id}, "", 0, 1)
	if len(albums) == 0 {
		subFail(c, subErrNotFound, "Album not found")
		return
	}

	a := albums[0]
	a.Songs = subSongList(c, " AND x.parent_id = ?", []any{id}, subTrackOrder, 0, 0)

	r := newSubResp()
	r.Album = &a
	writeSub(c, r)
}

// subGenreCond matches albums and songs of a genre in their comma
// separated list.
const subGenreCond = " AND (',' || replace(x.genres, ', ', ',') || ',') LIKE ?"

// The account's plays of an album x (the user as argument).
const (
	subPlaysExpr = "(SELECT COALESCE(sum(h.plays), 0) FROM play_history h INNER JOIN album_files f ON f.id = h.file_id " +
		"WHERE h.username = ? AND h.kind = 'track' AND f.album_id = x.id)"
	subLastPlayedExpr = "(SELECT max(h.updated_at) FROM play_history h INNER JOIN album_files f ON f.id = h.file_id " +
		"WHERE h.username = ? AND h.kind = 'track' AND f.album_id = x.id AND h.plays > 0)"
)

// subAlbumListOrder returns the ORDER BY and extra condition of an album
// list type, with their arguments (the condition's first).
func subAlbumListOrder(c *gin.Context) (order, cond string, args []any, ok bool) {
	user := accName(accessOf(c))

	switch subParam(c, "type") {
	case "random":
		return "random()", "", nil, true
	case "newest":
		return "x.id DESC", "", nil, true
	case "alphabeticalByName":
		return "x.name COLLATE NOCASE", "", nil, true
	case "alphabeticalByArtist":
		return "x.artist COLLATE NOCASE, x.year, x.name COLLATE NOCASE", "", nil, true
	case "byYear":
		from, to := subInt(c, "fromYear", 0), subInt(c, "toYear", 9999)
		if from > to {
			return "x.year DESC", " AND x.year BETWEEN ? AND ?", []any{to, from}, true
		}

		return "x.year", " AND x.year BETWEEN ? AND ?", []any{from, to}, true
	case "byGenre":
		return "x.name COLLATE NOCASE", subGenreCond, []any{"%," + subParam(c, "genre") + ",%"}, true
	case "frequent", "highest":
		return subPlaysExpr + " DESC", " AND " + subPlaysExpr + " > 0", []any{user, user}, true
	case "recent":
		return subLastPlayedExpr + " DESC", " AND " + subLastPlayedExpr + " IS NOT NULL", []any{user, user}, true
	case "starred":
		cond, args := favoriteCond(user, "album", "x.id")
		return "x.name COLLATE NOCASE", " AND " + cond, args, true
	}

	return "", "", nil, false
}

func subGetAlbumList(c *gin.Context, id3 bool) {
	order, cond, args, ok := subAlbumListOrder(c)
	if !ok {
		subFail(c, subErrMissing, "Required parameter is missing or unknown: type")
		return
	}

	albums := subAlbums(c, cond, args, order, max(subInt(c, "offset", 0), 0), min(max(subInt(c, "size", 10), 1), 500))

	r := newSubResp()

	if id3 {
		r.AlbumList2 = &subAlbumList{Albums: albums}
	} else {
		r.AlbumList = &subChildList{Albums: []subChild{}}
		for i := range albums {
			r.AlbumList.Albums = append(r.AlbumList.Albums, albums[i].child())
		}
	}

	writeSub(c, r)
}

func subGetGenres(c *gin.Context) {
	in, args := listsOf(subGroups(c))

	rows := database.StructscanT[struct {
		Genres string `db:"genres"`
		Songs  int    `db:"songs"`
	}](false, 1024, "SELECT d.genres, (SELECT count() FROM album_files f WHERE f.album_id = e.id) AS songs FROM albums e "+
		"INNER JOIN dbalbums d ON d.id = e.dbalbum_id WHERE COALESCE(d.genres, '') != '' AND e.listname"+in+
		" AND EXISTS (SELECT 1 FROM album_files f WHERE f.album_id = e.id)", args...)

	counts := map[string]*subGenre{}

	for _, r := range rows {
		for g := range strings.SplitSeq(r.Genres, ",") {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}

			if counts[g] == nil {
				counts[g] = &subGenre{Name: g}
			}

			counts[g].AlbumCount++
			counts[g].SongCount += r.Songs
		}
	}

	out := make([]subGenre, 0, len(counts))
	for _, g := range counts {
		out = append(out, *g)
	}

	slices.SortFunc(out, func(a, b subGenre) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })

	r := newSubResp()
	r.Genres = &subGenres{Genres: out}
	writeSub(c, r)
}

// --- songs ------------------------------------------------------------------

const subTrackOrder = "x.pidx, x.idx, x.loc"

// subTrackNumberRe is the track number in front of a file name.
var subTrackNumberRe = regexp.MustCompile(`^(\d{1,2}[-.])?\d{1,3}\s*[-._)]?\s+`)

// subSongList returns songs (cond and order on the track query).
func subSongList(c *gin.Context, cond string, args []any, order string, start, limit int) []subChild {
	rows, items := subQuery(c, jfTrack, cond, args, order, start, limit)

	out := make([]subChild, 0, len(rows))
	for i := range rows {
		out = append(out, subSong(&rows[i], &items[i]))
	}

	return out
}

func subSong(r *jfRow, it *jfItem) subChild {
	album := subAlbumID(r.ParentID)

	s := subChild{
		ID: subSongID(r.ID), Parent: album, Title: it.Name, Album: r.ParentName, Artist: r.Artist, Track: r.Idx, Year: r.Year,
		Genre: firstGenre(r.Genres), CoverArt: album, Size: r.Size, ContentType: mediaContentType(r.Location),
		Suffix: strings.TrimPrefix(strings.ToLower(path.Ext(r.Location)), "."), Duration: int(r.Runtime), BitRate: r.Bitrate,
		Path: strings.TrimPrefix(strings.ReplaceAll(r.Location, "\\", "/"), "/"), DiscNumber: r.Pidx, Created: subTime(r.Created),
		AlbumID: album, ArtistID: subArtistID(r.ArtistID), Type: "music", MediaType: "song",
	}

	if s.BitRate == 0 && r.Runtime > 0 {
		s.BitRate = int(float64(r.Size*8) / r.Runtime / 1000)
	}

	return s
}

func subGetSong(c *gin.Context) {
	id, ok := subID(c, "tr")
	if !ok {
		subFail(c, subErrNotFound, "Song not found")
		return
	}

	songs := subSongList(c, " AND x.id = ?", []any{id}, "", 0, 1)
	if len(songs) == 0 {
		subFail(c, subErrNotFound, "Song not found")
		return
	}

	r := newSubResp()
	r.Song = &songs[0]
	writeSub(c, r)
}

func subGetRandomSongs(c *gin.Context) {
	cond, args := "", []any{}

	if g := subParam(c, "genre"); g != "" {
		cond += subGenreCond
		args = append(args, "%,"+g+",%")
	}

	if y := subInt(c, "fromYear", 0); y > 0 {
		cond += " AND x.year >= ?"
		args = append(args, y)
	}

	if y := subInt(c, "toYear", 0); y > 0 {
		cond += " AND x.year <= ?"
		args = append(args, y)
	}

	r := newSubResp()
	r.RandomSongs = &subSongs{Songs: subSongList(c, cond, args, "random()", 0, min(max(subInt(c, "size", 10), 1), 500))}
	writeSub(c, r)
}

func subGetSongsByGenre(c *gin.Context) {
	r := newSubResp()
	r.SongsByGenre = &subSongs{Songs: subSongList(c, subGenreCond, []any{"%," + subParam(c, "genre") + ",%"},
		"x.parent_name, "+subTrackOrder, max(subInt(c, "offset", 0), 0), min(max(subInt(c, "count", 10), 1), 500))}
	writeSub(c, r)
}

// subGetDirectory browses by folders: an artist's albums or an album's
// songs.
func subGetDirectory(c *gin.Context) {
	r := newSubResp()

	if id, ok := subID(c, "ar"); ok {
		art, albums, ok := subArtistAlbums(c, id)
		if !ok {
			subFail(c, subErrNotFound, "Directory not found")
			return
		}

		r.Directory = &subDirectory{ID: art.ID, Name: art.Name, Children: make([]subChild, 0, len(albums))}
		for i := range albums {
			r.Directory.Children = append(r.Directory.Children, albums[i].child())
		}

		writeSub(c, r)

		return
	}

	if id, ok := subID(c, "al"); ok {
		albums := subAlbums(c, " AND x.id = ?", []any{id}, "", 0, 1)
		if len(albums) == 0 {
			subFail(c, subErrNotFound, "Directory not found")
			return
		}

		a := albums[0]
		r.Directory = &subDirectory{
			ID: a.ID, Parent: a.ArtistID, Name: a.Name, Children: subSongList(c, " AND x.parent_id = ?", []any{id}, subTrackOrder, 0, 0),
		}

		writeSub(c, r)

		return
	}

	subFail(c, subErrNotFound, "Directory not found")
}

func subSearchAll(c *gin.Context, id3 bool) {
	q := strings.Trim(strings.TrimSpace(subParam(c, "query")), `"*`)
	like := "%" + q + "%"

	page := func(count, offset string) (int, int) {
		return max(subInt(c, offset, 0), 0), min(max(subInt(c, count, 20), 0), 500)
	}

	res := &subSearch{Artists: []subArtist{}, Albums: []subAlbum{}, Songs: []subChild{}}

	if start, n := page("artistCount", "artistOffset"); n > 0 {
		res.Artists = subArtists(c, " AND x.name LIKE ?", []any{like}, start, n)
	}

	if start, n := page("albumCount", "albumOffset"); n > 0 {
		res.Albums = subAlbums(c, " AND (x.name LIKE ? OR x.artist LIKE ?)", []any{like, like}, "x.name COLLATE NOCASE", start, n)
	}

	if start, n := page("songCount", "songOffset"); n > 0 {
		res.Songs = subSongList(c, " AND (x.name LIKE ? OR x.loc LIKE ? OR x.artist LIKE ?)", []any{like, like, like},
			"x.parent_name, "+subTrackOrder, start, n)
	}

	r := newSubResp()
	if id3 {
		r.SearchResult3 = res
	} else {
		r.SearchResult2 = res
	}

	writeSub(c, r)
}

// --- files ------------------------------------------------------------------

func subGetCoverArt(c *gin.Context) {
	id := subParam(c, "id")
	full := subInt(c, "size", 0) == 0 || subInt(c, "size", 0) > 400

	var albumID uint64

	switch {
	case strings.HasPrefix(id, "al"):
		albumID, _ = strconv.ParseUint(id[2:], 10, 64)
	case strings.HasPrefix(id, "tr"):
		n, _ := strconv.ParseUint(id[2:], 10, 64)
		albumID = database.Getdatarow[uint64](false, "SELECT album_id FROM album_files WHERE id = ?", &n)
	case strings.HasPrefix(id, "ar"):
		n, _ := strconv.ParseUint(id[2:], 10, 64)
		if subArtistImage(c, n, full) {
			return
		}

		in, args := listsOf(subGroups(c))
		albumID = database.Getdatarow[uint64](false, "SELECT e.id FROM albums e INNER JOIN dbalbums d ON d.id = e.dbalbum_id WHERE "+
			"COALESCE("+jfAlbumArtist+", 0) = ? AND EXISTS (SELECT 1 FROM album_files f WHERE f.album_id = e.id) AND e.listname"+in+" LIMIT 1",
			append([]any{n}, args...)...)
	}

	k := libKindByType("album")
	if albumID == 0 || !accessOf(c).allowsEntry(k, &albumID) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	dbID := database.Getdatarow[uint64](false, "SELECT dbalbum_id FROM albums WHERE id = ?", &albumID)
	serveCover(c, "album", dbID, full)
}

// subArtistImage sends an artist's own image, if it has one.
func subArtistImage(c *gin.Context, id uint64, full bool) bool {
	img := database.Getdatarow[string](false, "SELECT COALESCE(image_url, '') FROM dbartists WHERE id = ?", &id)
	if !strings.HasPrefix(img, "http") || len(subArtists(c, " AND x.id = ?", []any{id}, 0, 1)) == 0 {
		return false
	}

	p, err := cachedCover("artist", id, img, img)
	if err != nil {
		return false
	}

	if !full {
		p = coverThumb(p)
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(p)

	return true
}

// subStream plays a song: the file as it is, or converted to MP3 when the
// app asks for a lower bitrate or another format.
func subStream(c *gin.Context) {
	id, ok := subID(c, "tr")
	if !ok {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	format := subParam(c, "format")
	maxRate := subInt(c, "maxBitRate", 0)

	loc := database.Getdatarow[string](false, "SELECT location FROM album_files WHERE id = ?", &id)
	ext := strings.TrimPrefix(strings.ToLower(path.Ext(loc)), ".")

	rate := database.Getdatarow[int](false, "SELECT COALESCE(bitrate, 0) FROM album_files WHERE id = ?", &id)
	if rate == 0 {
		size := database.Getdatarow[int64](false,
			"SELECT CASE WHEN runtime_ms > 0 THEN file_size * 8 / runtime_ms ELSE 0 END FROM album_files WHERE id = ?", &id)
		rate = int(size)
	}

	if format == "raw" || ((format == "" || format == ext) && (maxRate == 0 || (rate > 0 && rate <= maxRate))) {
		serveMediaFile(c, "track", id)
		return
	}

	kbps := maxRate
	if kbps <= 0 || kbps > 320 {
		kbps = 320
	}

	c.Params = gin.Params{{Key: "kind", Value: "track"}, {Key: "id", Value: strconv.FormatUint(id, 10)}}
	streamPlayFile(c, strconv.Itoa(kbps))
}

func subDownload(c *gin.Context) {
	id, ok := subID(c, "tr")
	if !ok || !accessOf(c).CanDownload() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	serveMediaFile(c, "track", id)
}

// subScrobble records a song played to the end (submission) in the
// account's history and the statistics; "now playing" reports show it in
// Now Playing.
func subScrobble(c *gin.Context) {
	acc := accessOf(c)

	ids := c.QueryArray("id")
	if len(ids) == 0 {
		ids = c.PostFormArray("id")
	}

	submission := subParam(c, "submission") != "false"

	for _, s := range ids {
		n, err := strconv.ParseUint(strings.TrimPrefix(s, "tr"), 10, 64)
		if err != nil {
			continue
		}

		rows, items := subQuery(c, jfTrack, " AND x.id = ?", []any{n}, "", 0, 1)
		if len(rows) == 0 {
			continue
		}

		song := subSong(&rows[0], &items[0])
		title := song.Artist + " - " + song.Title
		dur := fileSeconds(c, "track", n, rows[0].Location)
		sid := "subsonic" + strconv.FormatUint(n, 36)

		if !submission {
			_, _ = recordPlayProgress(acc.Name, playReport{
				SID: sid, Kind: "track", FileID: n, State: "start", Dur: dur, Title: title, Mode: "direct", Quality: "original",
				Client: c.ClientIP(),
			})

			continue
		}

		fid := uint(n)
		h := database.PlayHistory{Username: acc.Name, Kind: "track", FileID: fid, Title: title, Position: dur, Duration: dur}

		if err := database.PlayProgress(&h, true); err == nil {
			session := database.StartPlaySession(acc.Name, "track", fid, title, "direct", "original", false)
			database.AddPlaySeconds(session, dur, "direct")
		}

		_, _ = recordPlayProgress(acc.Name, playReport{SID: sid, Kind: "track", FileID: n, State: "stop", Client: c.ClientIP()})
	}

	writeSub(c, newSubResp())
}

// --- favorites and playlists ------------------------------------------------

// subParams are all values of a parameter (query and form).
func subParams(c *gin.Context, name string) []string {
	if v := c.QueryArray(name); len(v) > 0 {
		return v
	}

	return c.PostFormArray(name)
}

// subTrackItem is the playlist item of a song ID.
func subTrackItem(id string) (database.PlaylistItem, bool) {
	n, err := strconv.ParseUint(strings.TrimPrefix(id, "tr"), 10, 64)
	if err != nil {
		return database.PlaylistItem{}, false
	}

	album := database.Getdatarow[uint](false, "SELECT album_id FROM album_files WHERE id = ?", &n)

	return database.PlaylistItem{Kind: "track", EntryID: album, ItemID: uint(n)}, album != 0
}

// subStar stars (adds to the favorites) songs and albums, or unstars them.
// Artists can't be favorites.
func subStar(c *gin.Context, on bool) {
	acc := accessOf(c)

	for _, id := range append(subParams(c, "id"), subParams(c, "albumId")...) {
		if n, ok := strings.CutPrefix(id, "al"); ok {
			if e, err := strconv.ParseUint(n, 10, 64); err == nil {
				_ = setFavorite(acc, "album", uint(e), 0, on)
			}

			continue
		}

		if it, ok := subTrackItem(id); ok {
			_ = setFavorite(acc, "track", it.EntryID, it.ItemID, on)
		}
	}

	writeSub(c, newSubResp())
}

func subGetStarred(c *gin.Context, id3 bool) {
	user := accName(accessOf(c))
	res := &subSearch{Artists: []subArtist{}}

	cond, args := favoriteCond(user, "album", "x.id")
	res.Albums = subAlbums(c, " AND "+cond, args, "x.name COLLATE NOCASE", 0, 0)

	cond, args = favoriteCond(user, "track", "x.id")
	res.Songs = subSongList(c, " AND "+cond, args, "x.parent_name, "+subTrackOrder, 0, 0)

	r := newSubResp()
	if id3 {
		r.Starred2 = res
	} else {
		r.Starred = res
	}

	writeSub(c, r)
}

// subPlaylistSongs returns the songs a playlist plays, in its order, with
// the playlist item of each.
func subPlaylistSongs(c *gin.Context, id uint) ([]subChild, []uint, int) {
	files := playlistFiles(accessOf(c), id, libPlayAllMax)

	var ids []any

	for _, f := range files {
		if f.Kind == "track" {
			ids = append(ids, f.ID)
		}
	}

	if len(ids) == 0 {
		return nil, nil, len(files)
	}

	rows, items := subQuery(c, jfTrack, " AND x.id IN (?"+strings.Repeat(", ?", len(ids)-1)+")", ids, "", 0, 0)

	byID := map[uint64]subChild{}
	for i := range rows {
		byID[rows[i].ID] = subSong(&rows[i], &items[i])
	}

	var (
		songs []subChild
		from  []uint
	)

	for _, f := range files {
		if s, ok := byID[uint64(f.ID)]; ok && f.Kind == "track" {
			songs = append(songs, s)
			from = append(from, f.Item)
		}
	}

	return songs, from, len(files)
}

// subPlaylistOf describes a playlist; ok is false for a playlist with
// other media only (music apps show music).
func subPlaylistOf(c *gin.Context, p *database.Playlist, entries bool) (subPlaylist, bool) {
	songs, _, files := subPlaylistSongs(c, p.ID)

	out := subPlaylist{
		ID: strconv.FormatUint(uint64(p.ID), 10), Name: p.Name, Owner: p.Username, Public: p.Shared, SongCount: len(songs),
		Created: subTime(p.Updated), Changed: subTime(p.Updated),
	}

	for _, s := range songs {
		out.Duration += s.Duration
	}

	if len(songs) > 0 {
		out.CoverArt = songs[0].CoverArt
	}

	if entries {
		out.Entries = songs
		if out.Entries == nil {
			out.Entries = []subChild{}
		}
	}

	return out, len(songs) > 0 || files == 0
}

func subGetPlaylists(c *gin.Context) {
	r := newSubResp()
	r.Playlists = &subPlaylists{Playlists: []subPlaylist{}}

	for _, p := range database.GetPlaylists(accName(accessOf(c))) {
		if pl, ok := subPlaylistOf(c, &p, false); ok {
			r.Playlists.Playlists = append(r.Playlists.Playlists, pl)
		}
	}

	for _, v := range savedViews(accName(accessOf(c))) {
		if pl, ok := subSmartPlaylist(c, &v, false); ok {
			r.Playlists.Playlists = append(r.Playlists.Playlists, pl)
		}
	}

	writeSub(c, r)
}

// subSmartPlaylist describes a saved view of music as a playlist (ID
// "sv" + its ID) of the songs of the albums it finds; false for other
// views.
func subSmartPlaylist(c *gin.Context, v *savedView, entries bool) (subPlaylist, bool) {
	code, rows := smartRows(accessOf(c), requestLanguage(c), v)
	if code != jfAlbum {
		return subPlaylist{}, false
	}

	var songs []subChild

	for i := range rows {
		songs = append(songs, subSongList(c, " AND x.parent_id = ?", []any{rows[i].ID}, subTrackOrder, 0, 0)...)
		if len(songs) >= libPlayAllMax {
			break
		}
	}

	out := subPlaylist{
		ID: "sv" + strconv.FormatUint(smartID(v), 10), Name: v.Name, Owner: accName(accessOf(c)), SongCount: len(songs),
		Created: subTime(""), Changed: subTime(""),
	}

	for _, s := range songs {
		out.Duration += s.Duration
	}

	if len(songs) > 0 {
		out.CoverArt = songs[0].CoverArt
	}

	if entries {
		out.Entries = songs
		if out.Entries == nil {
			out.Entries = []subChild{}
		}
	}

	return out, true
}

// subPlaylistParam is the playlist a parameter names that the account may
// see.
func subPlaylistParam(c *gin.Context, name string) (*database.Playlist, bool) {
	n, err := strconv.ParseUint(subParam(c, name), 10, 64)
	if err != nil {
		return nil, false
	}

	id := uint(n)

	p, err := database.GetPlaylist(accName(accessOf(c)), &id)

	return p, err == nil && p.ID != 0
}

func subGetPlaylist(c *gin.Context) {
	if n, ok := subID(c, "sv"); ok {
		if v, ok := smartView(accName(accessOf(c)), n); ok {
			if pl, ok := subSmartPlaylist(c, v, true); ok {
				r := newSubResp()
				r.Playlist = &pl
				writeSub(c, r)

				return
			}
		}

		subFail(c, subErrNotFound, "Playlist not found")

		return
	}

	p, ok := subPlaylistParam(c, "id")
	if !ok {
		subFail(c, subErrNotFound, "Playlist not found")
		return
	}

	pl, _ := subPlaylistOf(c, p, true)

	r := newSubResp()
	r.Playlist = &pl
	writeSub(c, r)
}

// subAddSongs appends songs to one of the account's playlists.
func subAddSongs(c *gin.Context, id uint, songIDs []string) error {
	var in []database.PlaylistItem

	for _, s := range songIDs {
		if it, ok := subTrackItem(s); ok {
			in = append(in, it)
		}
	}

	acc := accessOf(c)
	_, err := database.AddPlaylistItems(accName(acc), &id, playlistItemsFor(acc, in))

	return err
}

// subCreatePlaylist makes a playlist of songs, or with playlistId replaces
// the songs of one.
func subCreatePlaylist(c *gin.Context) {
	user := accName(accessOf(c))

	p := &database.Playlist{Username: user, Name: subParam(c, "name")}

	if old, ok := subPlaylistParam(c, "playlistId"); ok {
		p = old
		if old.Username != user {
			subFail(c, 50, "Not your playlist")
			return
		}

		for _, it := range database.GetPlaylistItems(&p.ID) {
			_ = database.RemovePlaylistItem(user, &p.ID, &it.ID)
		}
	} else if err := database.SavePlaylist(p); err != nil {
		subFail(c, subErrGeneric, err.Error())
		return
	}

	if err := subAddSongs(c, p.ID, subParams(c, "songId")); err != nil {
		subFail(c, subErrGeneric, err.Error())
		return
	}

	pl, _ := subPlaylistOf(c, p, true)

	r := newSubResp()
	r.Playlist = &pl
	writeSub(c, r)
}

// subUpdatePlaylist renames, shares, adds songs to and removes entries
// from one of the account's playlists. An entry is removed only when its
// playlist item is that one song (not a whole album).
func subUpdatePlaylist(c *gin.Context) {
	user := accName(accessOf(c))

	p, ok := subPlaylistParam(c, "playlistId")
	if !ok || p.Username != user {
		subFail(c, subErrNotFound, "Playlist not found")
		return
	}

	if name, public := subParam(c, "name"), subParam(c, "public"); name != "" || public != "" {
		if name != "" {
			p.Name = name
		}

		if public != "" {
			p.Shared = public == "true"
		}

		if err := database.SavePlaylist(p); err != nil {
			subFail(c, subErrGeneric, err.Error())
			return
		}
	}

	if remove := subParams(c, "songIndexToRemove"); len(remove) > 0 {
		_, from, _ := subPlaylistSongs(c, p.ID)
		kinds := map[uint]string{}

		for _, it := range database.GetPlaylistItems(&p.ID) {
			kinds[it.ID] = it.Kind
		}

		for _, s := range remove {
			if i, err := strconv.Atoi(s); err == nil && i >= 0 && i < len(from) && kinds[from[i]] == "track" {
				_ = database.RemovePlaylistItem(user, &p.ID, &from[i])
			}
		}
	}

	if err := subAddSongs(c, p.ID, subParams(c, "songIdToAdd")); err != nil {
		subFail(c, subErrGeneric, err.Error())
		return
	}

	writeSub(c, newSubResp())
}

func subDeletePlaylist(c *gin.Context) {
	p, ok := subPlaylistParam(c, "id")
	if !ok || database.DeletePlaylist(accName(accessOf(c)), &p.ID) != nil {
		subFail(c, subErrNotFound, "Playlist not found")
		return
	}

	writeSub(c, newSubResp())
}

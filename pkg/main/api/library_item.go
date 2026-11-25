package api

import (
	"cmp"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// libItem is the detail of one list entry.
type libItem struct {
	libRow
	Kind    *libKind
	Filter  string                   // the grid's filter, applied to the episodes of a series
	Watched map[uint]float64         // the account's completion of the files, by file id
	Last    uint                     // the file the account played last (Play goes on there)
	Lang    string                   // the account's language (metadata translations)
	Favs    map[string]map[uint]bool // the account's favorites (favoriteSet)
}

func loadLibraryItem(k *libKind, id *uint64) *libItem {
	genre := "''"
	if k.Genre != "" {
		genre = "COALESCE(" + k.Genre + ", '')"
	}

	rows := database.StructscanT[libRow](false, 1,
		"SELECT "+k.Select+", "+genre+" AS genre FROM "+k.From+" WHERE e.id = ?", id)
	if len(rows) == 0 {
		return nil
	}

	libFillTitles(k, rows)

	return &libItem{libRow: rows[0], Kind: k}
}

// HandleLibraryItem renders the detail dialog body (/admin/library/item).
func HandleLibraryItem(c *gin.Context) {
	k := libKindByType(c.Query("type"))
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)

	if k == nil || err != nil || !accessOf(c).allowsEntry(k, &id) {
		c.String(http.StatusNotFound, renderAlert(t("Unknown entry"), "warning"))
		return
	}

	it := loadLibraryItem(k, &id)
	if it == nil {
		c.String(http.StatusNotFound, renderAlert(t("The entry no longer exists"), "warning"))
		return
	}

	it.Lang = requestLanguage(c)
	it.Favs = favoriteSet(accName(accessOf(c)))
	rows := []libRow{it.libRow}
	libTranslate(k, rows, it.Lang)
	it.libRow = rows[0]

	it.Filter = c.Query("filter")
	if it.Filter != "have" && it.Filter != "missing" {
		it.Filter = "all"
	}

	if pk := cmp.Or(libPlayKinds[k.Type], watchKinds[k.Type]); pk != "" {
		it.Watched = database.PlayCompletions(accName(accessOf(c)), pk, k.Files, k.FilesFK, &id)
		it.Last = database.LastPlayedFile(accName(accessOf(c)), pk, k.Files, k.FilesFK, &id)
	}

	c.String(http.StatusOK, renderComponentToString(renderLibraryItem(it)))
}

type libFile struct {
	ID       uint   `db:"id"`
	Location string `db:"location"`
	Quality  string `db:"quality_profile"`
}

type libHistory struct {
	Title   string `db:"title"`
	Indexer string `db:"indexer"`
	When    string `db:"downloaded_at"`
}

type libEpisode struct {
	FileID   uint   `db:"file_id"`
	ID       uint   `db:"id"`
	DbID     uint   `db:"db_id"`
	Ident    string `db:"identifier"`
	Title    string `db:"title"`
	Aired    string `db:"aired"`
	Missing  bool   `db:"missing"`
	HasFiles bool   `db:"have"`
	Source   string `db:"scraper_url"`
	Files    string `db:"files"` // the names of its files (the list's search)
	// DontSearch: the episode or its series is held back from searching.
	DontSearch bool `db:"dont_search"`
}

// libTrack is a track of an album with the file it's downloaded in (0:
// missing): by dbtrack_id, else by disc and track number.
type libTrack struct {
	ID      uint   `db:"id"`
	Disc    int    `db:"disc"`
	Number  int    `db:"number"`
	Title   string `db:"title"`
	Runtime int64  `db:"runtime_ms"`
	FileID  uint   `db:"file_id"`
}

func libAlbumTracks(albumID, dbalbumID uint) []libTrack {
	return database.StructscanT[libTrack](false, 50,
		`SELECT t.id, COALESCE(t.disc_number, 0) AS disc, COALESCE(t.track_number, 0) AS number, COALESCE(t.title, '') AS title,
			COALESCE(t.runtime_ms, 0) AS runtime_ms,
			COALESCE((SELECT f.id FROM album_files f WHERE f.album_id = ? AND (f.dbtrack_id = t.id OR COALESCE(f.dbtrack_id, 0) = 0
				AND (f.track_number = t.track_number AND MAX(COALESCE(f.disc_number, 0), 1) = MAX(COALESCE(t.disc_number, 0), 1)
					OR t.title != '' AND instr(lower(f.filename), lower(t.title)) > 0))
				ORDER BY f.dbtrack_id = t.id DESC, f.track_number = t.track_number DESC, f.id LIMIT 1), 0) AS file_id
		FROM dbtracks t WHERE t.dbalbum_id = ? ORDER BY t.disc_number, t.track_number LIMIT 500`, &albumID, &dbalbumID)
}

// libTrackCounts counts the tracks without a file and the files no track
// matched (numbered or named unlike the release): with such files left a
// track without one is unclear rather than missing.
func libTrackCounts(tracks []libTrack, fileCount int) (unmatched, spare int) {
	used := map[uint]bool{}

	for _, tr := range tracks {
		if tr.FileID == 0 {
			unmatched++
		} else {
			used[tr.FileID] = true
		}
	}

	return unmatched, max(fileCount-len(used), 0)
}

// libMissingReason says next to the status badge what an entry lacks: the
// tracks of an album not downloaded, or that the missing flag is out of
// date (files are there; the job "Check missing flag" corrects it).
func libMissingReason(it *libItem, fileCount int, tracks []libTrack) gomponents.Node {
	var text string

	unmatched, spare := libTrackCounts(tracks, fileCount)

	switch {
	case unmatched > 0 && spare > 0:
		text = tf("%d tracks without a matching file, %d files without a track", unmatched, spare)
	case unmatched > 0 && (fileCount > 0 || it.Missing):
		text = tf("%d of %d tracks missing", unmatched, len(tracks))
	case !it.Missing:
	case fileCount == 0:
		text = t("Nothing downloaded yet")
	case len(tracks) > 0:
		text = t("All tracks are there - the missing flag is out of date")
	default:
		text = tf("%d files are there - the missing flag is out of date", fileCount)
	}

	if text == "" {
		return nil
	}

	return html.Span(html.Class("small text-muted"), gomponents.Text(text))
}

func renderLibraryItem(it *libItem) gomponents.Node {
	k := it.Kind

	files := database.StructscanT[libFile](false, 50,
		"SELECT id, location, COALESCE(quality_profile, '') AS quality_profile FROM "+k.Files+" WHERE "+k.FilesFK+" = ? ORDER BY "+
			libFileOrder[k.Type]+" LIMIT 500", &it.ID)
	history := database.StructscanT[libHistory](false, 25,
		"SELECT title, COALESCE(indexer, '') AS indexer, COALESCE(substr(downloaded_at, 1, 16), '') AS downloaded_at FROM "+
			k.History+" WHERE "+k.FilesFK+" = ? ORDER BY id DESC LIMIT 25", &it.ID)

	// Counts of all rows; the tables show the first ones.
	fileCount := database.Getdatarow[int](false, "SELECT count() FROM "+k.Files+" WHERE "+k.FilesFK+" = ?", &it.ID)

	var (
		episodes          []libEpisode
		episodeCount, all int
	)

	if k.Type == "series" {
		epWhere := map[string]string{
			"have":    " AND EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_episode_id = se.id)",
			"missing": " AND se.missing = 1",
		}[it.Filter]

		all = database.Getdatarow[int](false, "SELECT count() FROM serie_episodes WHERE serie_id = ?", &it.ID)
		episodeCount = database.Getdatarow[int](false, "SELECT count() FROM serie_episodes se WHERE se.serie_id = ?"+epWhere, &it.ID)
		episodes = database.StructscanT[libEpisode](false, 100,
			`SELECT se.id, dse.id AS db_id, COALESCE(dse.identifier, '') AS identifier, COALESCE(dse.title, '') AS title,
				COALESCE(substr(dse.first_aired, 1, 10), '') AS aired, se.missing, COALESCE(dse.scraper_url, '') AS scraper_url,
				COALESCE((SELECT f.id FROM serie_episode_files f WHERE f.serie_episode_id = se.id ORDER BY f.id LIMIT 1), 0) AS file_id,
				EXISTS (SELECT 1 FROM serie_episode_files f WHERE f.serie_episode_id = se.id) AS have,
				COALESCE((SELECT group_concat(f.filename, ' ') FROM serie_episode_files f WHERE f.serie_episode_id = se.id), '') AS files,
				(se.dont_search OR s.dont_search) AS dont_search
			FROM serie_episodes se INNER JOIN dbserie_episodes dse ON dse.id = se.dbserie_episode_id
				INNER JOIN series s ON s.id = se.serie_id
			WHERE se.serie_id = ?`+epWhere+` ORDER BY CAST(dse.season AS INTEGER) DESC, CAST(dse.episode AS INTEGER) DESC, dse.identifier DESC LIMIT 1000`, &it.ID)

		ids := make([]uint, 0, len(episodes))
		for i := range episodes {
			ids = append(ids, episodes[i].DbID)
		}

		trs := database.TranslationsIn("dbserie_episode", it.Lang, ids)
		for i := range episodes {
			episodes[i].Title = firstText(trs[episodes[i].DbID]["title"], episodes[i].Title)
		}
	}

	var tracks []libTrack
	if k.Type == "album" {
		tracks = libAlbumTracks(it.ID, it.DbID)
	}

	_, spareFiles := libTrackCounts(tracks, fileCount)
	multiDisc := slices.ContainsFunc(tracks, func(tr libTrack) bool { return tr.Disc > 1 })

	// The grid's filter applies to the tracks like to the episodes.
	shownTracks := tracks
	if it.Filter != "all" {
		shownTracks = slices.DeleteFunc(slices.Clone(tracks), func(tr libTrack) bool { return (tr.FileID > 0) != (it.Filter == "have") })
	}

	fileNames := make(map[uint]string, len(files))
	for _, f := range files {
		fileNames[f.ID] = path.Base(f.Location)
	}

	trackRows := make([]gomponents.Node, 0, len(shownTracks))

	for i, tr := range shownTracks {
		num := strconv.Itoa(tr.Number)
		if multiDisc {
			num = fmt.Sprintf("%d-%02d", max(tr.Disc, 1), tr.Number)
		}

		length := ""
		if tr.Runtime > 0 {
			sec := tr.Runtime / 1000
			length = fmt.Sprintf("%d:%02d", sec/60, sec%60)
		}

		var status gomponents.Node

		switch {
		case tr.FileID > 0:
			status = html.Span(html.Class("badge bg-success"), gomponents.Text(t("Downloaded")))
		case spareFiles > 0:
			status = html.Span(html.Class("badge bg-secondary"), html.Title(t("A file of the album may hold it: its number and name match no track")),
				gomponents.Text(t("Not matched")))
		default:
			status = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text(t("Missing")))
		}

		trackRows = append(trackRows, html.Tr(libListRow(i, tr.Title, num, tr.Title, fileNames[tr.FileID]), libWatchState(it.Watched, tr.FileID),
			html.Td(html.Class("small text-nowrap font-monospace"), gomponents.Text(num)),
			html.Td(html.Class("small"), gomponents.Text(tr.Title)),
			html.Td(html.Class("small text-nowrap"), gomponents.Text(length)),
			html.Td(status)))
	}

	meta := []string{}
	if it.Sub != "" {
		meta = append(meta, it.Sub)
	}

	if it.Year > 0 {
		meta = append(meta, strconv.Itoa(it.Year))
	}

	meta = append(meta, it.Listname)

	tab := func(id, label string, count int, active bool) gomponents.Node {
		class := "nav-link"
		if active {
			class += " active"
		}

		if id == "libTabEdit" {
			class += " lib-admin"
		}

		return html.Li(html.Class("nav-item"), html.Button(html.Class(class), html.Type("button"),
			gomponents.Attr("data-bs-toggle", "tab"), gomponents.Attr("data-bs-target", "#"+id),
			gomponents.Text(label), gomponents.If(count >= 0, html.Span(html.Class("badge bg-secondary ms-1"), gomponents.Text(strconv.Itoa(count))))))
	}

	pane := func(id string, active bool, children ...gomponents.Node) gomponents.Node {
		class := "tab-pane fade pt-3"
		if active {
			class += " show active"
		}

		return html.Div(html.Class(class), html.ID(id), gomponents.Group(children))
	}

	playKind := libPlayKinds[k.Type]

	fileRows := make([]gomponents.Node, 0, len(files))
	for i, f := range files {
		fileRows = append(fileRows, html.Tr(libListRow(i, path.Base(f.Location), path.Base(f.Location), f.Quality), libWatchState(it.Watched, f.ID),
			html.Td(html.Class("text-nowrap"),
				gomponents.If(playKind != "", libPlayButton(playKind, f.ID, "files", i, path.Base(f.Location),
					it.Watched[f.ID] >= database.Watched, f.ID == it.Last)),
				libFileButtons(k, f.ID, path.Base(f.Location)),
				gomponents.If(k.Type == "album", libPlaylistAddMenu(libPlaylistItemsJSON("track", it.ID, f.ID), "", "btn-outline-success py-0 me-1")),
				gomponents.If(k.Type == "album", libFavoriteButton("track", it.ID, f.ID, it.Favs["track"][f.ID], true)),
				gomponents.If(playKind != "", libWatchedMark(playKind, f.ID, it.Watched)),
				gomponents.If(k.Type == "book", libReadButton(f.ID, f.Location, it.Title, readFrom(it.Watched[f.ID]), true)),
				gomponents.If(k.Type == "book", libWatchedMark("book", f.ID, it.Watched))),
			html.Td(html.Class("font-monospace small text-break"), gomponents.Text(f.Location)),
			html.Td(html.Class("small"), gomponents.Text(f.Quality)),
			html.Td(html.Class("text-end text-nowrap"), libPartDeleteButtons("file", k.Type, f.ID, true))))
	}

	histRows := make([]gomponents.Node, 0, len(history))
	for _, h := range history {
		histRows = append(histRows, html.Tr(html.Td(html.Class("small text-nowrap"), gomponents.Text(h.When)),
			html.Td(html.Class("small text-break"), gomponents.Text(h.Title)), html.Td(html.Class("small"), gomponents.Text(h.Indexer))))
	}

	epRows := make([]gomponents.Node, 0, len(episodes))
	for i, e := range episodes {
		status := html.Span(html.Class("badge bg-light text-dark border"), gomponents.Text("—"))

		switch {
		case e.HasFiles:
			status = html.Span(html.Class("badge bg-success"), gomponents.Text(t("Downloaded")))
		case e.Missing:
			status = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text(t("Missing")))
		}

		var thumb gomponents.Node = html.Td()
		if e.FileID > 0 {
			thumb = html.Td(html.Img(html.Class("lib-ep-thumb rounded"), gomponents.Attr("loading", "lazy"), html.Alt(""),
				html.Src("/api/admin/library/thumb/episode/"+strconv.FormatUint(uint64(e.FileID), 10))))
		}

		epRows = append(epRows, html.Tr(libListRow(i, e.Title, e.Ident, e.Title, e.Aired, e.Files), libWatchState(it.Watched, e.FileID),
			thumb,
			html.Td(html.Class("small text-nowrap font-monospace"), gomponents.Text(e.Ident)),
			html.Td(html.Class("small"), gomponents.Text(e.Title)),
			html.Td(html.Class("small text-nowrap"), gomponents.Text(e.Aired)),
			html.Td(status),
			html.Td(html.Class("text-end text-nowrap"),
				// The list is newest first; the playlist runs oldest first.
				gomponents.If(e.FileID > 0, libWatchedMark("episode", e.FileID, it.Watched)),
				gomponents.If(e.FileID > 0, libPlayButton("episode", e.FileID, "episodes", len(episodes)-i, e.Ident+" "+e.Title,
					it.Watched[e.FileID] >= database.Watched, e.FileID == it.Last)),
				gomponents.If(e.FileID > 0, libFileButtons(k, e.FileID, e.Ident+" "+e.Title)),
				gomponents.If(e.FileID > 0, libPlaylistAddMenu(libPlaylistItemsJSON("episode", it.ID, e.ID), "", "btn-outline-success py-0 me-1")),
				gomponents.If(e.FileID > 0, libFavoriteButton("episode", it.ID, e.ID, it.Favs["episode"][e.ID], true)),
				gomponents.If(!e.HasFiles && !e.DontSearch, html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-primary py-0 me-1 lib-search"),
					html.Title(t("Search this episode")),
					gomponents.Attr("onclick", fmt.Sprintf("libAction('search', %s)", jsString("/api/series/episodes/search/list/"+strconv.FormatUint(uint64(e.ID), 10)+"?download=true"))),
					html.I(html.Class("fa-solid fa-magnifying-glass")))),
				gomponents.If(libHTTPURL(e.Source), html.A(html.Class("btn btn-sm btn-outline-secondary py-0 me-1"), html.Href(e.Source),
					html.Target("_blank"), html.Rel("noopener"), html.Title(t("The page this episode was scraped from")),
					html.I(html.Class("fa-solid fa-globe")))),
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary py-0 me-1 lib-admin"), html.Title(t("Edit this episode")),
					gomponents.Attr("onclick", fmt.Sprintf("libEdit(%s, %d, %s, %d, %s)", jsString("serie_episodes"), e.ID,
						jsString("dbserie_episodes"), e.DbID, jsString(e.Ident))),
					html.I(html.Class("fa-solid fa-pen"))),
				libPartDeleteButtons("episode", k.Type, e.ID, e.HasFiles)),
		))
	}

	table := func(head []string, rows []gomponents.Node, total int, empty string) gomponents.Node {
		if len(rows) == 0 {
			return html.P(html.Class("text-muted small"), gomponents.Text(empty))
		}

		var note gomponents.Node
		if total > len(rows) {
			note = html.P(html.Class("text-muted small mt-2 mb-0"), gomponents.Text(tf("Showing %d of %d.", len(rows), total)))
		}

		ths := make([]gomponents.Node, 0, len(head))
		for _, h := range head {
			ths = append(ths, html.Th(html.Class("small"), gomponents.Text(h)))
		}

		return gomponents.Group([]gomponents.Node{html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(gomponents.Group(ths))), html.TBody(gomponents.Group(rows)))),
			html.P(html.Class("text-muted small mt-2 mb-0 d-none lib-list-none"), gomponents.Text(t("Nothing matches the search."))), note})
	}

	idStr := strconv.FormatUint(uint64(it.ID), 10)
	refreshURL := map[string]string{"movie": "movies", "series": "series", "album": "music", "audiobook": "audiobooks", "book": "books"}[k.Type]
	searchURL := k.SearchURL + idStr

	// Series: only the episodes the missing search would take, and the
	// button only when there are any (specials only count when wanted).
	searchable, searchArgs := true, "?download=true"

	if k.Type == "series" {
		searchURL, searchArgs = "/api/series/search/id/"+idStr, searchArgs+"&missing=true"
		searchable = database.Getdatarow[int](false, "select count() from "+seriesWantedMissing, &it.ID) > 0
	}

	var playFirst gomponents.Node
	if playKind != "" && len(files) > 0 {
		list := "files"
		if k.Type == "series" {
			list = "episodes"
		}

		playFirst = html.Button(html.Type("button"), html.Class("btn btn-sm btn-success"),
			gomponents.Attr("onclick", "libPlayFirst("+jsString(list)+")"),
			html.I(html.Class("fa-solid fa-play me-1")), gomponents.Text(t("Play")))

		// Series and albums also play shuffled.
		shuffle := map[string]string{"series": t("Play the downloaded episodes in random order"), "album": t("Play the tracks in random order")}[k.Type]
		if k.Type == "album" && len(files) < 2 ||
			k.Type == "series" && !slices.ContainsFunc(episodes, func(e libEpisode) bool { return e.FileID > 0 }) {
			shuffle = ""
		}

		if shuffle != "" {
			playFirst = gomponents.Group([]gomponents.Node{playFirst,
				html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success"),
					html.Title(shuffle), gomponents.Attr("aria-label", shuffle),
					gomponents.Attr("onclick", "libPlayRandom("+jsString(list)+")"),
					html.I(html.Class("fa-solid fa-shuffle")))})
		}
	}

	actions := html.Div(html.Class("d-flex flex-wrap gap-2 mt-3"),
		playFirst,
		gomponents.If(playKind != "", libPlaylistAddMenu(libPlaylistItemsJSON(k.Type, it.ID, 0), "", "btn-outline-success")),
		gomponents.If(playKind != "", libFavoriteButton(k.Type, it.ID, 0, it.Favs[k.Type][it.ID], false)),
		gomponents.If(k.Type == "book", libFirstReadButton(files, it)),
		libEntryDownload(k, it.ID, len(files)),
		// Filled with "Open in Plex/Jellyfin" after the dialog shows.
		gomponents.If(k.Type == "movie" || k.Type == "series", html.Span(html.ID("libLinks"), html.Class("d-contents"),
			html.Data("type", k.Type), html.Data("id", strconv.FormatUint(uint64(it.DbID), 10)), html.Data("list", it.Listname))),
		gomponents.If(!it.DontSearch && searchable, libSearchMenu(func(title bool) string {
			return fmt.Sprintf("libAction('search', %s)", jsString(searchURL+searchArgs+"&searchByTitle="+strconv.FormatBool(title)))
		}, map[bool]string{true: t("Search missing episodes"), false: t("Search")}[k.Type == "series"])),
		gomponents.If(it.DontSearch, html.Span(html.Class("badge bg-secondary align-self-center"),
			html.Title(t("Not searched (dont_search): a media request not approved yet, or switched off")),
			html.I(html.Class("fa-solid fa-ban me-1")), gomponents.Text(t("Not searched")))),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-primary lib-admin"),
			html.Title(t("Refresh metadata")), gomponents.Attr("aria-label", t("Refresh metadata")),
			gomponents.Attr("onclick", fmt.Sprintf("libAction('refresh', %s)", jsString("/api/"+refreshURL+"/refresh/"+strconv.FormatUint(uint64(it.DbID), 10)))),
			html.I(html.Class("fa-solid fa-rotate"))),
		html.Span(html.Class("flex-grow-1")),
		libDeleteMenu(func(files, exclude bool) string {
			return fmt.Sprintf("libDelete(%s, %d, %t, %t)", jsString(k.Type), it.ID, files, exclude)
		}, fileCount > 0, map[bool]string{true: t("and its file"), false: tf("and its %d files", fileCount)}[fileCount == 1]),
	)

	// Filled after the dialog shows (libLoadCollection), for admins.
	var collection gomponents.Node
	if k.Type == "movie" {
		collection = html.Div(html.ID("libCollection"), html.Class("lib-admin mt-3"),
			html.Data("id", strconv.FormatUint(uint64(it.DbID), 10)), html.Data("list", it.Listname))
	}

	coverClass := "rounded shadow-sm lib-detail-cover"
	if k.Square {
		coverClass += " lib-detail-square"
	}

	overview := it.Overview
	if overview == "" {
		overview = t("No description available.")
	}

	return html.Div(
		html.Data("title", it.Title),
		html.Div(html.Class("d-flex flex-column flex-md-row gap-4"),
			html.Img(html.Class(coverClass), html.Src(libCoverURL(k, &it.libRow)), html.Alt("")),
			html.Div(html.Class("flex-grow-1"), html.Style("min-width:0"),
				html.Div(html.Class("mb-2 d-flex gap-2 align-items-center flex-wrap"),
					html.Span(html.Class("badge bg-primary"), html.I(html.Class("fa-solid "+k.Icon+" me-1")), gomponents.Text(k.Label)),
					libStatusBadge(&it.libRow), libMissingReason(it, fileCount, tracks), libRatingBadge(k, it.DbID)),
				html.Div(html.Class("text-muted small mb-2"), gomponents.Text(strings.Join(meta, " · "))),
				libGenreBadges(k.Type, it.Genre),
				html.P(html.Class("mb-0"), html.Style("white-space:pre-line"), gomponents.Text(overview)),
				actions,
				libExtLinksNode(libExtLinks(it)),
				collection,
			),
		),
		html.Ul(html.Class("nav nav-tabs mt-4"), html.Role("tablist"),
			gomponents.If(k.Type == "series", tab("libTabEpisodes", t("Episodes"), episodeCount, true)),
			gomponents.If(len(tracks) > 0, tab("libTabTracks", t("Tracks"), len(shownTracks), true)),
			tab("libTabFiles", t("Files"), fileCount, k.Type != "series" && len(tracks) == 0),
			tab("libTabHistory", t("History"), len(history), false),
			gomponents.If(k.Type == "movie" || k.Type == "series", tab("libTabMore", t("More"), -1, false)),
			tab("libTabEdit", t("Edit"), -1, false),
		),
		html.Div(html.Class("tab-content"),
			gomponents.If(k.Type == "series", pane("libTabEpisodes", true,
				libFilterNote("episodes", it.Filter, episodeCount, all),
				gomponents.If(len(episodes) > 1, libListTools(t("Filter by episode, title or file"), true, "episode",
					[2]string{"episode", t("Episode")}, [2]string{"title", t("Title")}, [2]string{"random", t("Random")})),
				table([]string{"", t("Episode"), t("Title"), t("Aired"), t("Status"), ""}, epRows, episodeCount,
					map[bool]string{true: t("No episodes."), false: t("No episodes match the filter.")}[it.Filter == "all"]))),
			gomponents.If(len(tracks) > 0, pane("libTabTracks", true,
				libFilterNote("tracks", it.Filter, len(shownTracks), len(tracks)),
				gomponents.If(len(shownTracks) > 1, libListTools(t("Filter by number, title or file"), false, "track",
					[2]string{"track", t("Track")}, [2]string{"title", t("Title")}, [2]string{"random", t("Random")})),
				table([]string{"#", t("Title"), t("Length"), t("Status")}, trackRows, len(shownTracks), t("No tracks match the filter.")))),
			pane("libTabFiles", k.Type != "series" && len(tracks) == 0,
				gomponents.If(len(files) > 1, libFileListTools(k.Type, cmp.Or(libPlayKinds[k.Type], watchKinds[k.Type]))),
				table([]string{"", t("File"), t("Quality"), ""}, fileRows, fileCount, t("No files."))),
			pane("libTabHistory", false, table([]string{t("Downloaded"), t("Release"), t("Indexer")}, histRows, len(histRows), t("No downloads yet."))),
			// Cast, videos, recommendations, reviews: loaded when the tab opens.
			gomponents.If(k.Type == "movie" || k.Type == "series", pane("libTabMore", false,
				html.Div(html.ID("libExtras"), html.Data("type", k.Type), html.Data("id", strconv.FormatUint(uint64(it.DbID), 10)),
					html.Data("list", it.Listname), html.P(html.Class("text-muted small"), gomponents.Text(t("Loading…")))))),
			pane("libTabEdit", false,
				html.Div(html.ID("libEditForms"),
					html.Data("entry-table", strings.Fields(k.From)[0]), html.Data("entry-id", idStr),
					html.Data("db-table", k.DbTable), html.Data("db-id", strconv.FormatUint(uint64(it.DbID), 10)),
					gomponents.If(k.Type == "series", html.Data("episodes", "1")),
					html.P(html.Class("text-muted small"), gomponents.Text(t("Loading…"))))),
		),
	)
}

// libFilterNote says that the episodes or tracks follow the grid filter,
// with a button to show all of them.
func libFilterNote(list, filter string, shown, all int) gomponents.Node {
	// Nothing to show all of when the filter hides nothing.
	if filter == "all" || shown == all {
		return nil
	}

	label := map[string]string{"have": t("downloaded"), "missing": t("missing")}[filter]

	text, button := tf("Showing the %d %s of %d episodes, like the filter of the grid.", shown, label, all), t("Show all episodes")
	if list == "tracks" {
		text, button = tf("Showing the %d %s of %d tracks, like the filter of the grid.", shown, label, all), t("Show all tracks")
	}

	return html.Div(html.Class("d-flex flex-wrap align-items-center gap-2 small text-muted mb-2"),
		html.I(html.Class("fa-solid fa-filter")),
		gomponents.Text(text),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-link p-0"), gomponents.Attr("onclick", "libItemFilter('all')"),
			gomponents.Text(button)))
}

// libListRow marks a row of a list in the dialog: its place in the natural
// order, what it sorts by as title and the texts the search box looks in.
func libListRow(i int, title string, search ...string) gomponents.Node {
	return gomponents.Group([]gomponents.Node{html.Data("i", strconv.Itoa(i)), html.Data("title", title),
		html.Data("q", strings.ToLower(strings.Join(search, " ")))})
}

// libWatchState marks a row with the account's state of its file
// (data-ws: watched, progress or unwatched; no file: unwatched) for the
// list's watched filter.
func libWatchState(watched map[uint]float64, fileID uint) gomponents.Node {
	c := watched[fileID]
	id := gomponents.If(fileID > 0, html.Data("wid", strconv.FormatUint(uint64(fileID), 10)))

	switch {
	case fileID > 0 && c >= database.Watched:
		return gomponents.Group{html.Data("ws", "watched"), id}
	case fileID > 0 && c > 0:
		return gomponents.Group{html.Data("ws", "progress"), id}
	}

	return gomponents.Group{html.Data("ws", "unwatched"), id}
}

// libListTools is the search box, the watched filter (watched) and the
// order of a list in the dialog (libFilterList, libSortList); the first
// order is the natural one, which plays from the end with playRev (the
// episodes are listed newest first).
func libListTools(placeholder string, playRev bool, watchKind string, orders ...[2]string) gomponents.Node {
	var order gomponents.Node

	if len(orders) > 0 {
		opts := make([]gomponents.Node, 0, len(orders))
		for _, o := range orders {
			opts = append(opts, html.Option(html.Value(o[0]), gomponents.Text(o[1])))
		}

		order = html.Div(html.Class("d-flex align-items-center gap-2"),
			html.Span(html.Class("small text-muted"), gomponents.Text(t("Order"))),
			html.Div(html.Class("input-group input-group-sm w-auto"),
				html.Select(html.Class("form-select form-select-sm lib-list-sort"), gomponents.Attr("aria-label", t("Order")),
					gomponents.Attr("onchange", "libSortList(this)"), gomponents.Group(opts)),
				html.Button(html.Type("button"), html.Class("btn btn-outline-secondary lib-list-dir"),
					html.Title(t("Reverse the order")), gomponents.Attr("aria-label", t("Reverse the order")), gomponents.Attr("onclick", "libToggleListDir(this)"),
					html.I(html.Class("fa-solid fa-arrow-down-wide-short")))))
	}

	return html.Div(html.Class("d-flex flex-wrap align-items-center gap-3 mb-2 lib-list-tools"), gomponents.If(playRev, html.Data("play-rev", "1")),
		html.Div(html.Class("input-group input-group-sm"), html.Style("max-width:20rem"),
			html.Span(html.Class("input-group-text"), html.I(html.Class("fa-solid fa-magnifying-glass"))),
			html.Input(html.Type("search"), html.Class("form-control form-control-sm lib-list-q"), html.Placeholder(placeholder),
				gomponents.Attr("aria-label", placeholder), gomponents.Attr("oninput", "libFilterList(this)"))),
		gomponents.If(watchKind != "", html.Select(html.Class("form-select form-select-sm w-auto lib-list-ws"), gomponents.Attr("aria-label", t("Watched / heard")),
			gomponents.Attr("onchange", "libFilterList(this)"),
			html.Option(html.Value(""), gomponents.Text(t("All"))),
			html.Option(html.Value("unwatched"), gomponents.Text(t("Not watched / heard"))),
			html.Option(html.Value("progress"), gomponents.Text(t("In progress"))),
			html.Option(html.Value("watched"), gomponents.Text(t("Watched / heard"))))),
		gomponents.If(watchKind != "", libMarkShownMenu(watchKind)),
		order)
}

// libMarkShownMenu marks all rows the list shows (all, or those the filters
// leave) watched or not watched.
func libMarkShownMenu(kind string) gomponents.Node {
	item := func(on bool, icon, label string) gomponents.Node {
		return html.Li(html.Button(html.Type("button"), html.Class("dropdown-item"),
			gomponents.Attr("onclick", "libMarkShown(this, "+jsString(kind)+", "+strconv.FormatBool(on)+")"),
			html.I(html.Class(icon+" me-2")), gomponents.Text(label)))
	}

	return html.Div(html.Class("dropdown"),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-secondary dropdown-toggle"), html.Data("bs-toggle", "dropdown"),
			html.Aria("expanded", "false"), html.I(html.Class("fa-solid fa-check-double me-1")), gomponents.Text(t("Mark all shown"))),
		html.Ul(html.Class("dropdown-menu"),
			item(true, "fa-solid fa-check", t("as watched / heard")),
			item(false, "fa-regular fa-eye-slash", t("as not watched / heard"))))
}

// libFileListTools: the files of albums and audiobooks also sort (they
// play in track order); the others are listed by path.
func libFileListTools(kind, watchKind string) gomponents.Node {
	if kind != "album" && kind != "audiobook" {
		return libListTools(t("Filter by file or quality"), false, watchKind)
	}

	return libListTools(t("Filter by file or quality"), false, watchKind,
		[2]string{"track", t("Track")}, [2]string{"title", t("File name")}, [2]string{"random", t("Random")})
}

// libPartDeleteButtons are the delete buttons of an episode or a file row:
// remove the row only, and (when it has files) also delete them on disk.
func libPartDeleteButtons(part, kind string, id uint, hasFiles bool) gomponents.Node {
	call := func(withFiles bool) gomponents.Node {
		return gomponents.Attr("onclick", fmt.Sprintf("libDeletePart(%s, %s, %d, %t, this)", jsString(part), jsString(kind), id, withFiles))
	}

	title := map[string]string{"episode": t("Remove this episode from the list (files on disk are kept)"), "file": t("Remove this file from the list (the file on disk is kept)")}[part]
	titleFiles := map[string]string{"episode": t("Delete this episode and its files on disk"), "file": t("Delete this file on disk")}[part]

	return gomponents.Group([]gomponents.Node{
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger py-0 me-1 lib-admin"), html.Title(title),
			gomponents.Attr("aria-label", title), call(false), html.I(html.Class("fa-solid fa-trash"))),
		gomponents.If(hasFiles, html.Button(html.Type("button"), html.Class("btn btn-sm btn-danger py-0 lib-admin"), html.Title(titleFiles),
			gomponents.Attr("aria-label", titleFiles), call(true), html.I(html.Class("fa-solid fa-trash-can")))),
	})
}

func libHTTPURL(u string) bool {
	return strings.HasPrefix(u, "https://") || strings.HasPrefix(u, "http://")
}

// libExtLinks are the pages of an entry on the metadata sites, like the
// calendar shows them (IMDb, TMDB, TheTVDB, Trakt, the scraped site,
// MusicBrainz, Discogs, Audible, Goodreads, ...).
func libExtLinks(it *libItem) []calendarExtLink {
	var links []calendarExtLink

	add := func(label, icon, u string) { links = append(links, calendarExtLink{Label: label, URL: u, Icon: icon}) }

	switch it.Kind.Type {
	case "movie":
		m := database.StructscanT[struct {
			IMDb  string `db:"imdb_id"`
			TMDb  int    `db:"moviedb_id"`
			Trakt int    `db:"trakt_id"`
			URL   string `db:"url"`
		}](false, 1, "SELECT COALESCE(imdb_id, '') AS imdb_id, COALESCE(moviedb_id, 0) AS moviedb_id, COALESCE(trakt_id, 0) AS trakt_id, "+
			"COALESCE(url, '') AS url FROM dbmovies WHERE id = ?", &it.DbID)
		if len(m) == 0 {
			return nil
		}

		links = movieLinks(m[0].IMDb, m[0].TMDb, m[0].Trakt, m[0].URL)
	case "series":
		sr := database.StructscanT[struct {
			IMDb  string `db:"imdb_id"`
			TVDb  int    `db:"thetvdb_id"`
			Trakt int    `db:"trakt_id"`
		}](false, 1, "SELECT COALESCE(imdb_id, '') AS imdb_id, COALESCE(thetvdb_id, 0) AS thetvdb_id, COALESCE(trakt_id, 0) AS trakt_id "+
			"FROM dbseries WHERE id = ?", &it.DbID)
		if len(sr) == 0 {
			return nil
		}

		links = seriesLinks(sr[0].IMDb, sr[0].TVDb, sr[0].Trakt)

		// Scraped series (sites): the site of their newest scraped episode.
		src := database.Getdatarow[string](false, "SELECT COALESCE(scraper_url, '') FROM dbserie_episodes WHERE dbserie_id = ? AND scraper_url != '' ORDER BY id DESC LIMIT 1", &it.DbID)
		if u, err := url.Parse(src); err == nil && libHTTPURL(src) && u.Host != "" {
			add(t("Source"), "fa-solid fa-globe", u.Scheme+"://"+u.Host+"/")
		}
	case "album":
		a := database.StructscanT[AlbumCalendarQuery](false, 1, `SELECT COALESCE(musicbrainz_release_group_id, '') AS mb_group,
			COALESCE(musicbrainz_release_id, '') AS mb_release, COALESCE(discogs_master_id, '') AS discogs_master,
			COALESCE(discogs_release_id, '') AS discogs_release, COALESCE(deezer_id, '') AS deezer, COALESCE(itunes_id, '') AS itunes
			FROM dbalbums WHERE id = ?`, &it.DbID)
		if len(a) == 0 {
			return nil
		}

		a[0].Artist, a[0].Title = it.Sub, it.Title
		if it.NoTitle {
			a[0].Artist = "" // no Last.fm search for a folder name
		}

		return albumLinks(&a[0])
	case "audiobook":
		ab := AudiobookCalendarQuery{Title: it.Title, Author: it.Sub,
			ASIN: database.Getdatarow[string](false, "SELECT COALESCE(asin, '') FROM dbaudiobooks WHERE id = ?", &it.DbID)}

		return audiobookLinks(&ab, audibleRegionOf(it.Listname))
	case "book":
		b := database.StructscanT[BookCalendarQuery](false, 1, `SELECT COALESCE(isbn_13, '') AS isbn, COALESCE(openlibrary_id, '') AS openlibrary,
			COALESCE(goodreads_id, '') AS goodreads FROM dbbooks WHERE id = ?`, &it.DbID)
		if len(b) == 0 {
			return nil
		}

		b[0].Title = it.Title

		return bookLinks(&b[0], it.Sub)
	}

	return links
}

func libExtLinksNode(links []calendarExtLink) gomponents.Node {
	if len(links) == 0 {
		return nil
	}

	nodes := make([]gomponents.Node, 0, len(links))
	for _, l := range links {
		nodes = append(nodes, html.A(html.Class("btn btn-sm btn-outline-secondary"), html.Href(l.URL), html.Target("_blank"), html.Rel("noopener"),
			html.I(html.Class(l.Icon+" me-1")), gomponents.Text(l.Label)))
	}

	return html.Div(html.Class("d-flex flex-wrap gap-2 mt-2"), gomponents.Group(nodes))
}

// HandleLibraryDeletePart deletes one episode (part=episode, a serie_episodes
// ID) or one file row (part=file) of a list entry, with the files on disk
// when files=1 (/admin/library/delete-part). Only IDs are taken, never paths.
func HandleLibraryDeletePart(c *gin.Context) {
	k := libKindByType(c.PostForm("type"))
	part := c.PostForm("part")
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)

	if k == nil || err != nil || (part == "episode" && k.Type != "series") || (part != "episode" && part != "file") {
		c.JSON(http.StatusNotFound, gin.H{"error": t("unknown episode or file")})
		return
	}

	rowTable, filesQuery := k.Files, "SELECT location FROM "+k.Files+" WHERE id = ?"
	if part == "episode" {
		rowTable, filesQuery = "serie_episodes", "SELECT location FROM serie_episode_files WHERE serie_episode_id = ?"
	}

	if database.Getdatarow[int](false, "SELECT count() FROM "+rowTable+" WHERE id = ?", &id) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": map[bool]string{true: t("The episode no longer exists."), false: t("The file no longer exists.")}[part == "episode"]})
		return
	}

	removed := 0

	if c.PostForm("files") == "1" {
		for _, loc := range database.GetrowsN[string](false, 10, filesQuery, &id) {
			if loc == "" {
				continue
			}

			if _, err := scanner.RemoveFile(loc); err != nil {
				logger.Logtype("error", 0).Err(err).Str("file", loc).Msg("Library: delete file")
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("a file could not be deleted (%d deleted before); the %s was kept", removed, part),
				})

				return
			}

			removed++
		}
	}

	if part == "episode" {
		// Same rows as DELETE /api/series/episodes/list/:id.
		database.DeleteRow("serie_episode_files", "serie_episode_id = ?", &id)
		database.DeleteRow("serie_episode_histories", "serie_episode_id = ?", &id)
	}

	if _, err := database.DeleteRow(rowTable, "id = ?", &id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Logtype("info", 0).Str("type", k.Type).Str("part", part).Uint64(logger.StrID, id).Int("files_deleted", removed).
		Msg("Library: deleted")

	c.JSON(http.StatusOK, gin.H{"deleted": true, "files": removed})
}

// HandleLibraryDelete deletes a list entry, with its files on disk when
// files=1 (/admin/library/delete). Only IDs are taken, never paths.
func HandleLibraryDelete(c *gin.Context) {
	k := libKindByType(c.PostForm("type"))
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)

	if k == nil || err != nil || loadLibraryItem(k, &id) == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": t("entry not found")})
		return
	}

	removed, failed := 0, 0

	if c.PostForm("files") == "1" {
		for _, loc := range database.GetrowsN[string](false, 100,
			"SELECT location FROM "+k.Files+" WHERE "+k.FilesFK+" = ?", &id) {
			if _, err := scanner.RemoveFile(loc); err != nil {
				failed++

				logger.Logtype("error", 0).Err(err).Str("file", loc).Msg("Library: delete file")

				continue
			}

			removed++
		}

		if failed > 0 {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("%d files could not be deleted (%d deleted); the entry was kept", failed, removed),
			})

			return
		}
	}

	if c.PostForm("exclude") == "1" {
		if err := excludeLibraryEntry(k, &id); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": t("not excluded, the entry was kept: ") + err.Error()})
			return
		}
	}

	if err := deleteLibraryEntry(k, &id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	logger.Logtype("info", 0).Str("type", k.Type).Uint64(logger.StrID, id).Int("files_deleted", removed).
		Bool("excluded", c.PostForm("exclude") == "1").Msg("Library: entry deleted")

	c.JSON(http.StatusOK, gin.H{"deleted": true, "files": removed})
}

// deleteLibraryEntry removes a list entry with its file, history (and for a
// series episode) rows; files on disk stay.
func deleteLibraryEntry(k *libKind, id *uint64) error {
	// Same rows as the per-type list delete endpoints.
	if k.Type == "series" {
		database.DeleteRow("serie_episode_files", "serie_id = ?", id)
		database.DeleteRow("serie_episode_histories", "serie_id = ?", id)
		database.DeleteRow("serie_episodes", "serie_id = ?", id)
	} else {
		database.DeleteRow(k.Files, k.FilesFK+" = ?", id)
		database.DeleteRow(k.History, k.FilesFK+" = ?", id)
	}

	_, err := database.DeleteRow(k.entryTable(), "id = ?", id)

	return err
}

// libGenreBadges shows an item's genres; a click filters the grid by one.
func libGenreBadges(kind, genres string) gomponents.Node {
	names := splitGenres(genres)
	if len(names) == 0 {
		return nil
	}

	badges := make([]gomponents.Node, 0, len(names))
	for _, g := range names {
		badges = append(badges, html.Button(html.Type("button"), html.Class("badge rounded-pill bg-secondary border-0 lib-genre"),
			html.Title(t("Show all with this genre")), gomponents.Attr("onclick", "libShowGenre("+jsString(kind)+", "+jsString(g)+")"),
			gomponents.Text(g)))
	}

	return html.Div(html.Class("d-flex flex-wrap gap-1 mb-2"), html.I(html.Class("fa-solid fa-tags text-muted small me-1 align-self-center"),
		html.Title(t("Genre")), gomponents.Attr("aria-label", t("Genre"))), gomponents.Group(badges))
}

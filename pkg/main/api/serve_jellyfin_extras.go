package api

import (
	"context"
	"hash/crc32"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// What the Jellyfin apps show around an item: genres to browse by, the
// trailer, the cast and similar titles of the library (the Library's More
// tab: TMDB or Trakt, cached for a day).

// jfGenreNames remembers the genre of each genre ID handed out (IDs are a
// hash of the name).
var jfGenreNames sync.Map // genre ID -> name

func jfGenreID(name string) string {
	id := jfID(jfGenre, uint64(crc32.ChecksumIEEE([]byte(strings.ToLower(name)))))
	jfGenreNames.Store(id, name)

	return id
}

// jfGenreItems links an item's genres.
func jfGenreItems(genres []string) []jfNameID {
	out := make([]jfNameID, 0, len(genres))
	for _, g := range genres {
		out = append(out, jfNameID{Name: g, ID: jfGenreID(g)})
	}

	return out
}

// jfGenreKinds are the queries whose genres a library type has (tracks
// have their album's).
var jfGenreKinds = map[string]byte{"movie": jfMovie, "series": jfSeries, "album": jfAlbum}

// jfGenreCounts counts the genres of the groups' items.
func jfGenreCounts(groups []libGroup) map[string]int {
	counts := map[string]int{}

	for _, g := range groups {
		code, ok := jfGenreKinds[g.Kind.Type]
		if !ok {
			continue
		}

		rows, _ := jfRows(jfKinds[code], jfQuery{Groups: []libGroup{g}, Cond: " AND x.genres != ''", NoTotal: true})
		for i := range rows {
			for _, name := range splitGenres(rows[i].Genres) {
				counts[name]++
			}
		}
	}

	return counts
}

// jfGenres lists the genres of a library (ParentId) or of all.
func jfGenres(c *gin.Context, _ map[string]string) {
	groups := jfGroups(c)
	if g := jfGroupOf(groups, jfParam(c, "parentId")); g != nil {
		groups = []libGroup{*g}
	}

	music := strings.Contains(strings.ToLower(c.Request.URL.Path), "musicgenres")

	var use []libGroup

	for _, g := range groups {
		if (g.Kind.Type == "album") == music {
			use = append(use, g)
		}
	}

	counts := jfGenreCounts(use)

	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}

	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })

	if s := strings.ToLower(jfParam(c, "searchTerm")); s != "" {
		names = slices.DeleteFunc(names, func(n string) bool { return !strings.Contains(strings.ToLower(n), s) })
	}

	typ := "Genre"
	if music {
		typ = "MusicGenre"
	}

	items := make([]jfItem, 0, len(names))
	for _, n := range names {
		items = append(items, jfItem{
			Name: n, ServerID: jfServerID(), ID: jfGenreID(n), Type: typ, IsFolder: true, LocationType: "FileSystem",
			ChildCount: counts[n], ImageTags: map[string]string{}, BackdropImageTags: []string{}, SortName: strings.ToLower(n),
		})
	}

	start, limit := jfInt(c, "startIndex"), jfInt(c, "limit")
	total := len(items)
	items = items[min(max(start, 0), total):]

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	jfResult(c, items, total, start)
}

// jfGenreNamesOf returns the names of genre IDs (from the IDs handed out,
// else by going through the library's genres).
func jfGenreNamesOf(groups []libGroup, ids []string) []string {
	var (
		out     []string
		missing bool
	)

	for _, id := range ids {
		if v, ok := jfGenreNames.Load(strings.ReplaceAll(id, "-", "")); ok {
			out = append(out, v.(string))
		} else {
			missing = true
		}
	}

	if missing {
		for n := range jfGenreCounts(groups) {
			if slices.Contains(ids, jfGenreID(n)) && !slices.Contains(out, n) {
				out = append(out, n)
			}
		}
	}

	return out
}

// --- trailer and cast -------------------------------------------------------

type jfTrailer struct {
	URL  string `json:"Url"`
	Name string `json:"Name,omitempty"`
}

type jfPersonDto struct {
	Name            string `json:"Name"`
	ID              string `json:"Id"`
	Role            string `json:"Role,omitempty"`
	Type            string `json:"Type"`
	PrimaryImageTag string `json:"PrimaryImageTag,omitempty"`
}

// jfPersonImages remembers the TMDB profile picture of each person ID
// handed out (cast loaded live, not saved by the import yet).
var jfPersonImages sync.Map // TMDB person ID -> profile path

// jfPersonNames remembers their names likewise.
var jfPersonNames sync.Map // TMDB person ID -> name

// jfPersonOf is a person's name and TMDB picture: from the saved cast,
// else from the cast handed out since the start.
func jfPersonOf(personID uint64) (name, profile string, ok bool) {
	if p, found := database.MediaPerson(personID); found {
		return p.Name, p.Profile, true
	}

	if v, found := jfPersonNames.Load(personID); found {
		name = v.(string)
		if img, has := jfPersonImages.Load(personID); has {
			profile = img.(string)
		}

		return name, profile, true
	}

	return "", "", false
}

// jfPersonItem is a person as an item (an actor's page in the apps).
func jfPersonItem(personID uint64, name, profile string) jfItem {
	id := jfID(jfPerson, personID)
	it := jfItem{Name: name, ServerID: jfServerID(), ID: id, Type: "Person", SortName: strings.ToLower(name),
		LocationType: "FileSystem", ImageTags: map[string]string{}, BackdropImageTags: []string{}, PlayAccess: "Full",
		ProviderIDs: map[string]string{"Tmdb": strconv.FormatUint(personID, 10)}}

	if profile != "" {
		it.ImageTags["Primary"] = jfImageTag(id, "primary")
	}

	return it
}

// jfPersonCastCond limits titles to a cast member (the apps' personIds):
// movies and series by media_cast; other kinds have none.
func jfPersonCastCond(code byte, personIDs []uint64) (string, []any) {
	kind := map[byte]string{jfMovie: "movie", jfSeries: "series"}[code]
	if kind == "" || len(personIDs) == 0 {
		return " AND 0", nil
	}

	args := []any{kind}
	for _, id := range personIDs {
		args = append(args, id)
	}

	return " AND EXISTS (SELECT 1 FROM media_cast mc WHERE mc.kind = ? AND mc.db_id = x.db_id AND mc.person_id IN (?" +
		strings.Repeat(", ?", len(personIDs)-1) + "))", args
}

// jfPersons answers the people search (/Persons?searchTerm=): the cast
// of the account's movies and series whose name matches.
func jfPersons(c *gin.Context, _ map[string]string) {
	term := strings.TrimSpace(jfParam(c, "searchTerm"))
	if term == "" {
		jfResult(c, nil, 0, 0)
		return
	}

	var conds []string

	args := []any{"%" + term + "%"}

	for _, g := range jfGroups(c) {
		parent := map[string]string{"movie": "movies m WHERE m.dbmovie_id", "series": "series m WHERE m.dbserie_id"}[g.Kind.Type]
		if parent == "" || len(g.Lists) == 0 {
			continue
		}

		conds = append(conds, "(mc.kind = ? AND EXISTS (SELECT 1 FROM "+parent+" = mc.db_id AND m.listname COLLATE NOCASE IN (?"+
			strings.Repeat(", ?", len(g.Lists)-1)+")))")
		args = append(args, g.Kind.Type)

		for _, l := range g.Lists {
			args = append(args, l)
		}
	}

	if len(conds) == 0 {
		jfResult(c, nil, 0, 0)
		return
	}

	limit := jfInt(c, "limit")
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	rows := database.StructscanT[database.MediaCast](false, uint(limit), //nolint:gosec // 1..100
		"SELECT person_id, MAX(name) AS name, '' AS character, MAX(profile) AS profile FROM media_cast mc WHERE person_id > 0 AND name LIKE ? AND ("+
			strings.Join(conds, " OR ")+") GROUP BY person_id ORDER BY count() DESC, name LIMIT "+strconv.Itoa(limit), args...)

	items := make([]jfItem, 0, len(rows))
	for _, r := range rows {
		items = append(items, jfPersonItem(uint64(r.PersonID), r.Name, r.Profile)) //nolint:gosec // positive
	}

	jfResult(c, items, len(items), 0)
}

func init() {
	jfRoute2(http.MethodGet, "persons", false, jfPersons)
}

// jfMaxCast is how many of the cast an item names.
const jfMaxCast = 20

// jfAddDetails puts the trailer and the cast into a movie or series item
// (from TMDB or Trakt; a few seconds the first time, then cached).
func jfAddDetails(c *gin.Context, it *jfItem, code byte, dbID uint64) {
	typ := map[byte]string{jfMovie: "movie", jfSeries: "series"}[code]
	if typ == "" || dbID == 0 {
		return
	}

	lang := requestLanguage(c)

	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()

	if tr, err := trailerOf(ctx, typ, dbID, lang); err == nil && tr != nil {
		it.RemoteTrailers = []jfTrailer{{URL: "https://www.youtube.com/watch?v=" + tr.Key, Name: tr.Name}}
	}

	e := extrasOf(ctx, typ, dbID, lang)
	if e == nil {
		return
	}

	for _, m := range e.Cast {
		if len(it.People) == jfMaxCast {
			break
		}

		p := jfPersonDto{Name: m.Name, ID: jfID(jfPerson, uint64(max(m.ID, 0))), Role: m.Character, Type: "Actor"} //nolint:gosec // TMDB IDs are positive
		if m.ID > 0 {
			jfPersonNames.Store(uint64(m.ID), m.Name) //nolint:gosec // positive
		}

		if m.ProfilePath != "" && m.ID > 0 {
			jfPersonImages.Store(uint64(m.ID), m.ProfilePath) //nolint:gosec // positive
			p.PrimaryImageTag = jfImageTag(p.ID, "primary")
		}

		it.People = append(it.People, p)
	}
}

// jfSimilar lists the recommended and similar titles of an item that the
// account's library has.
func jfSimilar(c *gin.Context, p map[string]string) {
	code, n, ok := parseJFID(p["id"])
	typ := map[byte]string{jfMovie: "movie", jfSeries: "series"}[code]

	if !ok || typ == "" {
		jfResult(c, nil, 0, 0)
		return
	}

	k := jfKinds[code]
	groups := jfGroups(c)

	rows, _ := jfRows(k, jfQuery{Groups: groups, Cond: " AND x.id = ?", Args: []any{n}, NoTotal: true})
	if len(rows) == 0 {
		jfResult(c, nil, 0, 0)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	e := extrasOf(ctx, typ, rows[0].DbID, requestLanguage(c))
	if e == nil {
		jfResult(c, nil, 0, 0)
		return
	}

	var lists []string

	for _, g := range groups {
		if g.Kind.Type == typ {
			lists = append(lists, g.Lists...)
		}
	}

	titles := append(slices.Clone(e.Recommendations), e.Similar...)
	markInLibrary(typ, lists, titles)

	var ids []any

	for _, t := range titles {
		if t.EntryID != 0 && uint64(t.EntryID) != n && !slices.Contains(ids, any(t.EntryID)) {
			ids = append(ids, t.EntryID)
		}
	}

	limit := jfInt(c, "limit")
	if limit > 0 && len(ids) > limit {
		ids = ids[:limit]
	}

	if len(ids) == 0 {
		jfResult(c, nil, 0, 0)
		return
	}

	found, _ := jfRows(k, jfQuery{Groups: groups, Cond: " AND x.id IN (?" + strings.Repeat(", ?", len(ids)-1) + ")", Args: ids, NoTotal: true})
	items := jfBuild(c, k, found)

	jfResult(c, items, len(items), 0)
}

func init() {
	for _, p := range []string{"genres", "musicgenres"} {
		jfRoute2(http.MethodGet, p, false, jfGenres)
	}

	for _, p := range []string{"items/{id}/similar", "shows/{id}/similar", "movies/{id}/similar"} {
		jfRoute2(http.MethodGet, p, false, jfSimilar)
	}
}

// --- collections (box sets) -------------------------------------------------

// jfBoxSetsViewID is the collections library.
var jfBoxSetsViewID = jfID(jfBoxSet, 0)

// jfBoxSetGroup is a TMDB collection with the account's movies of it.
type jfBoxSetGroup struct {
	ID     int
	Name   string
	Movies []jfRow
}

// movieCollectionRef is the collection of a dbmovies row.
type movieCollectionRef struct {
	DbID uint64 `db:"id"`
	ID   int    `db:"collection_id"`
	Name string `db:"collection_name"`
}

// movieCollections maps the dbmovies rows in a collection to it.
func movieCollections() map[uint64]movieCollectionRef {
	refs := database.StructscanT[movieCollectionRef](false, 0,
		"SELECT id, collection_id, collection_name FROM dbmovies WHERE collection_id > 0")

	out := make(map[uint64]movieCollectionRef, len(refs))
	for _, r := range refs {
		out[r.DbID] = r
	}

	return out
}

// jfBoxSetsOf groups the movies of the groups by their TMDB collection
// (dbmovies.collection_id), those with at least two movies, by name.
func jfBoxSetsOf(groups []libGroup) []jfBoxSetGroup {
	rows, _ := jfRows(jfKinds[jfMovie], jfQuery{Groups: groups, Order: "x.year", NoTotal: true})
	colls := movieCollections()

	byID := map[int]*jfBoxSetGroup{}

	for _, r := range rows {
		ref, ok := colls[r.DbID]
		if !ok {
			continue
		}

		coll, name := ref.ID, ref.Name

		if byID[coll] == nil {
			byID[coll] = &jfBoxSetGroup{ID: coll, Name: name}
		}

		byID[coll].Movies = append(byID[coll].Movies, r)
	}

	out := make([]jfBoxSetGroup, 0, len(byID))
	for _, g := range byID {
		if len(g.Movies) >= 2 {
			out = append(out, *g)
		}
	}

	slices.SortFunc(out, func(a, b jfBoxSetGroup) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })

	return out
}

func jfMovieGroups(groups []libGroup) []libGroup {
	var out []libGroup

	for _, g := range groups {
		if g.Kind.Type == "movie" {
			out = append(out, g)
		}
	}

	return out
}

func jfBoxSetItem(g *jfBoxSetGroup) jfItem {
	id := jfID(jfBoxSet, uint64(g.ID)) //nolint:gosec // TMDB IDs are positive
	it := jfItem{
		Name: g.Name, ServerID: jfServerID(), ID: id, Type: "BoxSet", IsFolder: true, LocationType: "FileSystem",
		ChildCount: len(g.Movies), RecursiveItemCount: len(g.Movies), PrimaryImageAspectRatio: 2.0 / 3, PlayAccess: "Full",
		ImageTags: map[string]string{"Primary": jfImageTag(id, "primary")}, BackdropImageTags: []string{},
		SortName: strings.ToLower(g.Name), ParentID: jfBoxSetsViewID, ProviderIDs: map[string]string{"Tmdb": strconv.Itoa(g.ID)},
	}

	if len(g.Movies) > 0 {
		it.ProductionYear = g.Movies[0].Year
		if g.Movies[0].Backdrop != "" {
			it.BackdropImageTags = []string{jfImageTag(id, "backdrop")}
		}
	}

	return it
}

func jfBoxSetsView(lang string) jfItem {
	return jfItem{
		Name: trText(lang, t("Collections")), ServerID: jfServerID(), ID: jfBoxSetsViewID, Type: "CollectionFolder", IsFolder: true,
		CollectionType: "boxsets", LocationType: "FileSystem", ImageTags: map[string]string{}, BackdropImageTags: []string{},
		PlayAccess: "Full", SortName: "￾",
	}
}

// jfBoxSetList answers the collections of the groups' movies.
func jfBoxSetList(c *gin.Context, groups []libGroup) {
	sets := jfBoxSetsOf(jfMovieGroups(groups))

	items := make([]jfItem, 0, len(sets))
	for i := range sets {
		items = append(items, jfBoxSetItem(&sets[i]))
	}

	start, limit := jfInt(c, "startIndex"), jfInt(c, "limit")
	total := len(items)
	items = items[min(max(start, 0), total):]

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}

	jfResult(c, items, total, start)
}

// jfBoxSetMovies answers the account's movies of a collection.
func jfBoxSetMovies(c *gin.Context, coll int) {
	for _, g := range jfBoxSetsOf(jfMovieGroups(jfGroups(c))) {
		if g.ID == coll {
			items := jfBuild(c, jfKinds[jfMovie], g.Movies)
			jfResult(c, items, len(items), 0)

			return
		}
	}

	jfResult(c, nil, 0, 0)
}

// jfBoxSetFirstMovie is the dbmovies row of a collection's first movie
// (for its images).
func jfBoxSetFirstMovie(coll int) uint64 {
	return database.Getdatarow[uint64](false, "SELECT id FROM dbmovies WHERE collection_id = ? ORDER BY year LIMIT 1", coll)
}

// jfNoWebClient answers the web client's address while the web client
// isn't loaded (yet, or the download failed): only the apps with their own
// player work then.
func jfNoWebClient(c *gin.Context, _ map[string]string) {
	lang := browserLanguage(c.GetHeader("Accept-Language"))
	esc := func(s string) string { return xmlEscape(trText(lang, s)) }

	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(`<!doctype html><html><head><meta charset="utf-8">`+
		`<meta name="viewport" content="width=device-width, initial-scale=1"><title>`+xmlEscape(serverName())+`</title>`+
		`<style>body{font-family:system-ui,sans-serif;max-width:40rem;margin:2rem auto;padding:0 1rem;line-height:1.5;color:#212529;background:#fff}`+
		`@media (prefers-color-scheme:dark){body{color:#e9ecef;background:#19222c}a{color:#6ea8fe}}</style></head><body>`+
		`<h1>`+xmlEscape(serverName())+`</h1><p>`+esc(t("This address is for Jellyfin apps that bring their own player: Swiftfin, Infuse, Findroid and the Jellyfin apps for TVs. The Jellyfin web client and the official Jellyfin app for iPhone and iPad, which shows it, are not included."))+
		`</p><p><a href="/api/">`+esc(t("Open the web interface"))+`</a></p></body></html>`))
}

func init() {
	jfRoute2(http.MethodGet, "", true, jfWebRoot)
	jfRoute2(http.MethodGet, "web/*", true, jfWeb)
	jfRoute2(http.MethodGet, "web", true, jfWeb)
}

package api

import (
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The Collections view of a movie group: the TMDB collections (film series,
// dbmovies.collection_id, saved by the metadata import) with at least two
// of the group's movies that match the filter, like the apps' box sets. A
// click shows the collection's movies in the grid (the coll filter,
// libShowCollection).

// libCollection is a collection as the view shows it.
type libCollection struct {
	ID     int    `db:"id"`
	Name   string `db:"name"`
	Movies int    `db:"movies"`
	From   int    `db:"y_from"`
	To     int    `db:"y_to"`
	First  string `db:"first"` // year-dbmovies ID of the oldest movie (its cover)
}

// renderLibraryCollections renders a page of the collections of a movie group.
func renderLibraryCollections(g *libGroup, q libQuery) gomponents.Node {
	search := q.Search
	q.Search = "" // the search is on the collection names, not the movies

	where, args := libWhere(g, q)
	inner := libInner(g.Kind, "0", "")

	having := "count() >= 2"
	if search != "" {
		having += " AND name LIKE ?"
		args = append(args, "%"+search+"%")
	}

	grouped := "SELECT d.collection_id AS id, MAX(d.collection_name) AS name, count() AS movies, MIN(e.year) AS y_from, MAX(e.year) AS y_to, " +
		"MIN(printf('%04d-%d', e.year, e.db_id)) AS first FROM (" + inner + ") e INNER JOIN dbmovies d ON d.id = e.db_id WHERE " + where +
		" AND d.collection_id > 0 GROUP BY d.collection_id HAVING " + having

	total := database.Getdatarow[int](false, "SELECT count() FROM ("+grouped+")", args...)
	if total == 0 {
		return html.P(html.Class("text-muted text-center py-5"),
			gomponents.Text(t("No collections with at least two movies here. The collection of a movie is saved when its metadata is refreshed.")))
	}

	order := "name COLLATE NOCASE"
	if q.Sort == "year" {
		order = "y_from, name COLLATE NOCASE"
	}

	if q.Rev {
		order = strings.ReplaceAll(order, ", ", " DESC, ") + " DESC"
	}

	colls := database.StructscanT[libCollection](false, q.capacity(), "SELECT * FROM ("+grouped+") ORDER BY "+order+q.limit(), args...)

	cards := make([]gomponents.Node, 0, len(colls))
	for _, c := range colls {
		_, first, _ := strings.Cut(c.First, "-")

		years := strconv.Itoa(c.From)
		if c.To != c.From {
			years += "–" + strconv.Itoa(c.To)
		}

		cards = append(cards, html.Div(html.Class("lib-card"), html.Title(c.Name),
			gomponents.Attr("onclick", "libShowCollection("+strconv.Itoa(c.ID)+")"), html.Style("cursor:pointer"),
			html.Div(html.Class("lib-cover"),
				html.Img(html.Src("/api/admin/library/cover/movie/"+first), html.Alt(""), gomponents.Attr("loading", "lazy")),
				html.Span(html.Class("badge text-bg-primary position-absolute top-0 end-0 m-1"), gomponents.Text(strconv.Itoa(c.Movies)))),
			html.Div(html.Class("lib-title text-truncate"), gomponents.Text(c.Name)),
			html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(tf("%d movies", c.Movies)+" · "+years))))
	}

	return gomponents.Group([]gomponents.Node{
		libCountLine(q, total, "%d collections", false),
		html.Div(html.Class("lib-grid"), gomponents.Group(cards)),
		libPager(q, total),
	})
}

// libCollectionChip names the collection the grid shows the movies of,
// with a button back to all movies.
func libCollectionChip(g *libGroup, q libQuery) gomponents.Node {
	if q.Coll == 0 || g.Kind.Type != "movie" {
		return nil
	}

	name := database.Getdatarow[string](false, "SELECT collection_name FROM dbmovies WHERE collection_id = ? LIMIT 1", q.Coll)

	return html.Div(html.Class("mb-2"),
		html.Span(html.Class("badge bg-primary fs-6 fw-normal"),
			html.I(html.Class("fa-solid fa-layer-group me-1")), gomponents.Text(name),
			html.Button(html.Type("button"), html.Class("btn-close btn-close-white ms-2"), html.Style("font-size:.6rem"),
				html.Title(t("All movies")), gomponents.Attr("aria-label", t("All movies")), gomponents.Attr("onclick", "libShowCollection(0)"))))
}

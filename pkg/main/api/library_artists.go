package api

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The artist view of music groups: the artists of the group's albums
// (the first artist of each album; "Unknown artist" for albums without
// one), the same artists the apps show (jfRows). A click shows the
// artist's albums in the grid (the artist filter, libShowArtist).

// renderLibraryArtists renders a page of the artists of a music group.
func renderLibraryArtists(g *libGroup, q libQuery) gomponents.Node {
	cond, args := "", []any{}
	if q.Search != "" {
		cond, args = " AND x.name LIKE ?", []any{"%" + q.Search + "%"}
	}

	start, limit := 0, q.Size
	if q.Size > 0 {
		start = (q.Page - 1) * q.Size
	}

	k := jfKinds[jfArtist]
	rows, total := jfRows(k, jfQuery{Groups: []libGroup{*g}, Cond: cond, Args: args, Start: start, Limit: limit})
	items := jfItems(k, rows, "", q.Lang)

	if total == 0 {
		return html.P(html.Class("text-muted text-center py-5"), gomponents.Text(t("Nothing found.")))
	}

	cards := make([]gomponents.Node, 0, len(rows))

	for i := range rows {
		id := strconv.FormatUint(rows[i].ID, 10)
		cards = append(cards, html.Div(html.Class("lib-card"), html.Title(items[i].Name),
			gomponents.Attr("onclick", "libShowArtist("+jsString(id)+")"), html.Style("cursor:pointer"),
			html.Div(html.Class("lib-cover"),
				html.Img(html.Src("/api/admin/library/artist-image/"+id), html.Alt(""), gomponents.Attr("loading", "lazy"))),
			html.Div(html.Class("lib-title text-truncate"), gomponents.Text(items[i].Name)),
			html.Div(html.Class("lib-sub text-truncate"), gomponents.Text(tf("%d albums", rows[i].Children)))))
	}

	return gomponents.Group([]gomponents.Node{
		libCountLine(q, total, "%d artists", false),
		html.Div(html.Class("lib-grid lib-square"), gomponents.Group(cards)),
		libPager(q, total),
	})
}

// libArtistChip names the artist the grid shows the albums of, with a
// button back to all albums.
func libArtistChip(g *libGroup, q libQuery) gomponents.Node {
	if q.Artist == "" || g.Kind.Type != "album" {
		return nil
	}

	artist, _ := strconv.ParseUint(q.Artist, 10, 64)
	rows, _ := jfRows(jfKinds[jfArtist], jfQuery{Groups: []libGroup{*g}, Cond: " AND x.id = ?", Args: []any{artist}, NoTotal: true})
	items := jfItems(jfKinds[jfArtist], rows, "", q.Lang)

	name := q.Artist
	if len(items) > 0 {
		name = items[0].Name
	}

	return html.Div(html.Class("mb-2"),
		html.Span(html.Class("badge bg-primary fs-6 fw-normal"),
			html.I(html.Class("fa-solid fa-user me-1")), gomponents.Text(name),
			html.Button(html.Type("button"), html.Class("btn-close btn-close-white ms-2"), html.Style("font-size:.6rem"),
				html.Title(t("All albums")), gomponents.Attr("aria-label", t("All albums")), gomponents.Attr("onclick", "libShowArtist('')"))))
}

// HandleLibraryArtistImage serves an artist's image, else the cover of one
// of its albums (/admin/library/artist-image/:id, a dbartists ID; 0: the
// albums without an artist).
func HandleLibraryArtistImage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	groups := groupsOf(accessOf(c), "album")

	rows, _ := jfRows(jfKinds[jfArtist], jfQuery{Groups: groups, Cond: " AND x.id = ?", Args: []any{id}, NoTotal: true})
	if len(rows) == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	path, err := jfImageFile(jfArtist, id, "primary")
	if err != nil || id == 0 {
		// An album of the artist the account may see.
		albums, _ := jfRows(jfKinds[jfAlbum], jfQuery{Groups: groups, Cond: " AND x.artist_id = ?", Args: []any{id}, Limit: 1, NoTotal: true})
		if len(albums) > 0 {
			path, err = libraryCoverFile("album", albums[0].DbID)
		}
	}

	if err != nil {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "image/svg+xml", []byte(coverPlaceholder))

		return
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(coverThumb(path))
}

package api

import (
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// libCastChip names the cast member the grid shows the titles of, with a
// button back to all (the cast filter, libShowCast).
func libCastChip(g *libGroup, q libQuery) gomponents.Node {
	if q.Cast == "" || libCastKind(g.Kind) == "" {
		return nil
	}

	return html.Div(html.Class("mb-2"),
		html.Span(html.Class("badge bg-primary fs-6 fw-normal"),
			html.I(html.Class("fa-solid fa-user me-1")), gomponents.Text(tf("Titles with %s", q.Cast)),
			html.Button(html.Type("button"), html.Class("btn-close btn-close-white ms-2"), html.Style("font-size:.6rem"),
				html.Title(t("Show all again")), gomponents.Attr("aria-label", t("Show all again")), gomponents.Attr("onclick", "libShowCast('', '')"))))
}

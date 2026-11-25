package api

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The play history section of the statistics page: what the accounts
// played in the Library player (play_history).

const playStatDays = 30

var playKindLabels = map[string]string{"movie": "Movies", "episode": "Episodes", "track": "Music", "audiobook": "Audiobooks", "book": "Books"}

func webStatisticsPlays(ctx *gin.Context) {
	renderStatHTML(ctx, html.Div(
		renderPlayStats(database.PlayStatsBy("username", playStatDays), database.PlayStatsBy("kind", playStatDays),
			database.RecentPlays("", 50)),
		renderViewStats(computeViewStats(database.PlaySessionsSince(playStatDays)))))
}

// viewStats are the viewing statistics of the play sessions.
type viewStats struct {
	Top      []titleStat // most played titles
	Heat     [7][24]int  // plays by weekday (Sunday first) and hour
	Modes    map[string]int
	Peak     int    // most streams at the same time
	PeakAt   string // when
	Sessions int
}

type titleStat struct {
	Title string
	Plays int
	Hours float64
}

func computeViewStats(sessions []database.PlaySession) viewStats {
	v := viewStats{Modes: map[string]int{}, Sessions: len(sessions)}
	byTitle := map[string]*titleStat{}

	type edge struct {
		at    time.Time
		delta int
	}

	edges := make([]edge, 0, 2*len(sessions))

	for _, s := range sessions {
		start, err1 := time.ParseInLocation(time.DateTime, s.StartedAt, time.Local)
		end, err2 := time.ParseInLocation(time.DateTime, s.EndedAt, time.Local)

		if err1 == nil {
			v.Heat[start.Weekday()][start.Hour()]++

			if err2 == nil && end.After(start) {
				edges = append(edges, edge{start, 1}, edge{end, -1})
			}
		}

		v.Modes[cmp.Or(s.Mode, "unknown")]++

		t := byTitle[s.Title]
		if t == nil {
			t = &titleStat{Title: s.Title}
			byTitle[s.Title] = t
		}

		if !s.Resumed {
			t.Plays++
		}

		t.Hours += s.Seconds / 3600
	}

	// Ends before starts at the same moment: back-to-back plays don't overlap.
	slices.SortFunc(edges, func(a, b edge) int { return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.delta, b.delta)) })

	cur := 0
	for _, e := range edges {
		cur += e.delta
		if cur > v.Peak {
			v.Peak, v.PeakAt = cur, e.at.Format("2006-01-02 15:04")
		}
	}

	for _, t := range byTitle {
		v.Top = append(v.Top, *t)
	}

	slices.SortFunc(v.Top, func(a, b titleStat) int { return cmp.Or(cmp.Compare(b.Plays, a.Plays), cmp.Compare(b.Hours, a.Hours)) })
	v.Top = v.Top[:min(len(v.Top), 15)]

	return v
}

var playModeLabels = map[string]string{
	"direct": "Direct play", "remux": "Repackaged (remux)", "transcode": "Converted", "hls": "Converted (HLS)", "unknown": "Unknown",
}

func renderViewStats(v viewStats) gomponents.Node {
	if v.Sessions == 0 {
		return nil
	}

	topRows := make([]gomponents.Node, 0, len(v.Top))
	for _, t := range v.Top {
		topRows = append(topRows, html.Tr(html.Td(gomponents.Text(t.Title)),
			html.Td(html.Class("text-end"), gomponents.Text(strconv.Itoa(t.Plays))),
			html.Td(html.Class("text-end"), gomponents.Text(fmt.Sprintf("%.1f h", t.Hours)))))
	}

	modeRows := make([]gomponents.Node, 0, len(v.Modes))
	for _, m := range []string{"direct", "remux", "transcode", "hls", "unknown"} {
		if n := v.Modes[m]; n > 0 {
			modeRows = append(modeRows, html.Tr(html.Td(gomponents.Text(playModeLabels[m])),
				html.Td(html.Class("text-end"), gomponents.Text(strconv.Itoa(n))),
				html.Td(html.Style("width: 40%"), shareBar(n, v.Sessions, "bg-info"))))
		}
	}

	return statSection("bg-info text-white", "fa-solid fa-chart-column", "Viewing",
		html.Div(html.Class("row"),
			html.Div(html.Class("col-lg-6"),
				html.H6(html.Class("text-muted"), gomponents.Text("Most played, last "+strconv.Itoa(playStatDays)+" days")),
				statTable([]gomponents.Node{html.Th(gomponents.Text("Title")), html.Th(html.Class("text-end"), gomponents.Text("Plays")),
					html.Th(html.Class("text-end"), gomponents.Text("Played"))}, topRows)),
			html.Div(html.Class("col-lg-6"),
				html.H6(html.Class("text-muted"), gomponents.Text("How it played")),
				statTable([]gomponents.Node{html.Th(gomponents.Text("Mode")), html.Th(html.Class("text-end"), gomponents.Text("Plays")),
					html.Th(gomponents.Text("Share"))}, modeRows),
				html.P(html.Class("small text-muted mt-3"), gomponents.Textf("Most streams at the same time: %d", v.Peak),
					gomponents.If(v.PeakAt != "", gomponents.Text(" ("+v.PeakAt+")"))))),
		html.H6(html.Class("text-muted mt-4"), gomponents.Text("When (plays by weekday and hour)")),
		renderPlayHeatmap(&v.Heat))
}

// renderPlayHeatmap is a weekday × hour grid, darker for more plays.
func renderPlayHeatmap(heat *[7][24]int) gomponents.Node {
	most := 1
	for d := range heat {
		for h := range heat[d] {
			most = max(most, heat[d][h])
		}
	}

	head := []gomponents.Node{html.Th()}
	for h := range 24 {
		head = append(head, html.Th(html.Class("text-center small fw-normal text-muted p-0"), gomponents.Text(strconv.Itoa(h))))
	}

	rows := make([]gomponents.Node, 0, 7)

	// Monday first.
	for _, d := range []time.Weekday{1, 2, 3, 4, 5, 6, 0} {
		cells := []gomponents.Node{html.Th(html.Class("small fw-normal text-muted pe-2"), gomponents.Text(d.String()[:3]))}
		for h := range 24 {
			n := heat[d][h]
			alpha := 0.06 + 0.94*float64(n)/float64(most)
			if n == 0 {
				alpha = 0.03
			}

			cells = append(cells, html.Td(html.Style(fmt.Sprintf("background:rgba(13,110,253,%.2f);height:1.4rem;min-width:1.2rem", alpha)),
				html.Title(fmt.Sprintf("%s %02d:00 - %d plays", d, h, n))))
		}

		rows = append(rows, html.Tr(cells...))
	}

	return html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm table-borderless mb-0"),
		html.Style("border-collapse:separate;border-spacing:2px"),
		html.THead(html.Tr(head...)), html.TBody(rows...)))
}

func renderPlayStats(byUser, byKind []database.PlayStat, recent []database.PlayHistory) gomponents.Node {
	sumRows := func(stats []database.PlayStat, label func(string) string) []gomponents.Node {
		rows := make([]gomponents.Node, 0, len(stats))
		for _, s := range stats {
			rows = append(rows, html.Tr(
				html.Td(html.Strong(gomponents.Text(label(s.Key)))),
				html.Td(html.Class("text-end"), gomponents.Text(strconv.Itoa(s.Plays))),
				html.Td(html.Class("text-end"), gomponents.Text(fmt.Sprintf("%.1f h", s.Hours)))))
		}

		if len(rows) == 0 {
			rows = append(rows, emptyRow(3, "Nothing played"))
		}

		return rows
	}

	head := func(first string) []gomponents.Node {
		return []gomponents.Node{html.Th(gomponents.Text(first)), html.Th(html.Class("text-end"), gomponents.Text("Plays")),
			html.Th(html.Class("text-end"), gomponents.Text("Played"))}
	}

	recentRows := make([]gomponents.Node, 0, len(recent))
	for i := range recent {
		h := &recent[i]
		recentRows = append(recentRows, html.Tr(
			html.Td(html.Class("small text-nowrap"), gomponents.Text(shortTime(h.UpdatedAt))),
			html.Td(gomponents.Text(h.Username)),
			html.Td(html.Class("small"), gomponents.Text(playKindLabels[h.Kind])),
			html.Td(html.Class("small"), gomponents.Text(h.Title)),
			html.Td(html.Style("width: 16%"), shareBar(int(h.Completion*100), 100, "bg-success")),
			html.Td(html.Class("text-end"), gomponents.Text(strconv.Itoa(h.Plays)))))
	}

	if len(recentRows) == 0 {
		recentRows = append(recentRows, emptyRow(6, "Nothing played yet"))
	}

	return statSection("bg-success text-white", "fa-solid fa-circle-play", "Play History",
		html.Div(html.Class("row"),
			html.Div(html.Class("col-md-6"),
				html.H6(html.Class("text-muted"), gomponents.Text("By account, last "+strconv.Itoa(playStatDays)+" days")),
				statTable(head("Account"), sumRows(byUser, func(k string) string { return k }))),
			html.Div(html.Class("col-md-6"),
				html.H6(html.Class("text-muted"), gomponents.Text("By type, last "+strconv.Itoa(playStatDays)+" days")),
				statTable(head("Type"), sumRows(byKind, func(k string) string { return playKindLabels[k] })))),
		html.H6(html.Class("text-muted mt-4"), gomponents.Text("Latest plays")),
		html.Div(html.Style("max-height: 420px; overflow-y: auto;"),
			statTable([]gomponents.Node{
				html.Th(gomponents.Text("Last played")), html.Th(gomponents.Text("Account")), html.Th(gomponents.Text("Type")),
				html.Th(gomponents.Text("Title")), html.Th(gomponents.Text("Completion")), html.Th(html.Class("text-end"), gomponents.Text("Plays")),
			}, recentRows)),
	)
}

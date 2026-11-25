package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// escapeJSString escapes a string for safe use in JavaScript.
func escapeJSString(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\r")

	return s
}

// calendarTypeFilters are the type buttons; a type only gets one when a media
// group of it is configured.
var calendarTypeFilters = []struct {
	Key, Label, Icon string
	configured       func(*config.MediaConfig) bool
}{
	{"movies", t("Movies"), "fa-film", func(m *config.MediaConfig) bool { return len(m.Movies) > 0 }},
	{"series", t("Series"), "fa-tv", func(m *config.MediaConfig) bool { return len(m.Series) > 0 }},
	{"albums", t("Albums"), "fa-compact-disc", func(m *config.MediaConfig) bool { return len(m.Music) > 0 }},
	{"audiobooks", t("Audiobooks"), "fa-headphones", func(m *config.MediaConfig) bool { return len(m.AudioBooks) > 0 }},
	{"books", t("Books"), "fa-book", func(m *config.MediaConfig) bool { return len(m.Books) > 0 }},
}

// calendarLink is a toolbar control that reloads the calendar with another state.
func calendarLink(st calendarState, change func(*calendarState), class string, nodes ...gomponents.Node) gomponents.Node {
	q := st.Query(change)

	return html.Button(append([]gomponents.Node{
		html.Type("button"),
		html.Class(class),
		hx.Get("/api/admin/calendar/content?" + q),
		hx.Target("#calendar-content"),
		hx.PushURL("/api/admin/calendar?" + q),
	}, nodes...)...)
}

func calendarPeriodLabel(st calendarState) string {
	start, end := st.Range()

	if st.View == "month" {
		return start.Format("January 2006")
	}

	return start.Format("Jan 2") + " – " + end.Format("Jan 2, 2006")
}

// calendarSearchItems are the missing monitored entries of the overdue
// events as {type, id} JSON for the "search overdue" buttons.
func calendarSearchItems(events []calendarEvent) (string, int) {
	type item struct {
		Type string `json:"type"`
		ID   uint   `json:"id"`
	}

	items := []item{}

	for _, e := range events {
		if !e.Overdue {
			continue
		}

		for _, en := range e.Entries {
			if en.Monitored && !en.Downloaded && en.ID > 0 {
				items = append(items, item{e.Type, en.ID})
			}
		}
	}

	data, _ := json.Marshal(items)

	return string(data), len(items)
}

// calendarSearchButton searches all overdue events of a list, one by one:
// by ID (IMDb / TVDB) or by title. Albums, audiobooks and books are always
// searched by title, so they get one button.
func calendarSearchButton(st calendarState, events []calendarEvent, label string) gomponents.Node {
	if st.access != nil && !st.access.CanSearch() {
		return nil
	}

	data, n := calendarSearchItems(events)
	if n == 0 {
		return nil
	}

	main := fmt.Sprintf("%s (%d)", label, n)
	run := func(byTitle bool) gomponents.Node {
		return gomponents.Group([]gomponents.Node{
			html.Data("items", data),
			gomponents.Attr("onclick", fmt.Sprintf("calendarBulkSearch(JSON.parse(this.dataset.items), %t)", byTitle)),
		})
	}

	button := html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger"),
		html.Title(t("Search everything that is released and still missing, one after the other")), run(false),
		html.I(html.Class("fa-solid fa-magnifying-glass me-1")), gomponents.Text(main))
	if !slices.ContainsFunc(events, func(e calendarEvent) bool { return e.Overdue && (e.Type == "movie" || e.Type == "series") }) {
		return button
	}

	// The button searches by ID; the menu picks ID or title.
	item := func(byTitle bool, icon, text string) gomponents.Node {
		return html.Li(html.Button(html.Type("button"), html.Class("dropdown-item"), run(byTitle),
			html.I(html.Class("fa-solid "+icon+" me-2")), gomponents.Text(text)))
	}

	return html.Div(html.Class("btn-group"),
		button,
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-danger dropdown-toggle dropdown-toggle-split"),
			html.Data("bs-toggle", "dropdown"), html.Aria("expanded", "false"),
			html.Span(html.Class("visually-hidden"), gomponents.Text(t("Choose how to search")))),
		html.Ul(html.Class("dropdown-menu dropdown-menu-end"),
			item(false, "fa-magnifying-glass", t("Search by ID (IMDb / TVDB)")),
			item(true, "fa-magnifying-glass-plus", t("Search by title"))))
}

// calendarSelects are the media group and genre selects; they reload the
// calendar with the changed parameter.
func calendarSelects(st calendarState, genres []string) gomponents.Node {
	sel := func(param, label, current string, opts [][2]string) gomponents.Node {
		nodes := []gomponents.Node{html.Option(html.Value(""), gomponents.Text(label))}
		for _, o := range opts {
			nodes = append(nodes, html.Option(html.Value(o[0]), gomponents.If(strings.EqualFold(o[0], current), html.Selected()), gomponents.Text(o[1])))
		}

		return html.Select(html.Class("form-select form-select-sm w-auto"), gomponents.Attr("aria-label", label),
			html.Data("query", st.Query(nil)),
			gomponents.Attr("onchange", "calendarSetParam(this.dataset.query, "+jsString(param)+", this.value)"), gomponents.Group(nodes))
	}

	var groups [][2]string
	for _, g := range libraryGroups() {
		if st.access == nil || st.access.AllowsGroup(g.Name) {
			groups = append(groups, [2]string{g.Name, g.Name})
		}
	}

	gen := make([][2]string, 0, len(genres)+1)
	for _, g := range genres {
		gen = append(gen, [2]string{g, g})
	}

	if st.Genre != "" && !slices.ContainsFunc(genres, func(g string) bool { return strings.EqualFold(g, st.Genre) }) {
		gen = append(gen, [2]string{st.Genre, st.Genre})
	}

	return html.Div(html.Class("d-flex flex-wrap gap-2"),
		gomponents.If(len(groups) > 1, sel("group", t("All media groups"), st.Group, groups)),
		gomponents.If(len(gen) > 0, sel("genre", t("Any genre"), st.Genre, gen)))
}

func renderCalendarToolbar(st calendarState, events []calendarEvent, genres []string) gomponents.Node {
	button := func(active bool) string {
		if active {
			return "btn btn-sm btn-primary"
		}

		return "btn btn-sm btn-outline-primary"
	}

	views := []gomponents.Node{}
	viewLabels := map[string]string{"month": t("Month"), "week": t("Week"), "agenda": t("Agenda")}
	for _, v := range calendarViews {
		views = append(views, calendarLink(st, func(s *calendarState) { s.View = v }, button(st.View == v),
			gomponents.Text(viewLabels[v])))
	}

	types := []gomponents.Node{calendarLink(st, func(s *calendarState) { s.Filter = "all" }, button(st.Filter == "all"), gomponents.Text(t("All")))}
	if media := config.GetSettingsMediaAll(); media != nil {
		for _, tp := range calendarTypeFilters {
			if tp.configured(media) || st.Filter == tp.Key {
				types = append(types, calendarLink(st, func(s *calendarState) { s.Filter = tp.Key }, button(st.Filter == tp.Key),
					html.I(html.Class("fa-solid "+tp.Icon+" me-1")), gomponents.Text(t(tp.Label))))
			}
		}
	}

	today := time.Now()
	todayDate := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)

	var downloaded, partial, overdue, fresh int

	for _, e := range events {
		if e.Downloaded {
			downloaded++
		}

		if e.Partial {
			partial++
		}

		if e.Overdue {
			overdue++
		}

		if e.New {
			fresh++
		}
	}

	summary := tf("%d events", len(events))
	if !st.OnlyMissing {
		summary += tf(" · %d downloaded · %d missing", downloaded, len(events)-downloaded-partial)
	}

	if partial > 0 {
		summary += tf(" · %d partly downloaded", partial)
	}

	if overdue > 0 {
		summary += tf(" · %d overdue", overdue)
	}

	if fresh > 0 {
		summary += tf(" · %d new since yesterday", fresh)
	}

	// Laid out explicitly: the theme's form-switch padding pushes the input
	// out of the card.
	missingSwitch := html.Div(
		html.Class("form-check form-switch mb-0 ps-0 d-flex align-items-center gap-2"),
		html.Input(
			html.Class("form-check-input m-0"), html.Style("float: none;"), html.Type("checkbox"), html.ID("calOnlyMissing"),
			gomponents.If(st.OnlyMissing, html.Checked()),
			hx.Get("/api/admin/calendar/content?"+st.Query(func(s *calendarState) { s.OnlyMissing = !s.OnlyMissing })),
			hx.Target("#calendar-content"),
			hx.PushURL("/api/admin/calendar?"+st.Query(func(s *calendarState) { s.OnlyMissing = !s.OnlyMissing })),
			hx.Trigger("change"),
		),
		html.Label(html.Class("form-check-label"), html.For("calOnlyMissing"), gomponents.Text(t("Only missing"))),
	)

	return html.Div(
		html.Class("mb-3"),
		html.Div(
			html.Class("d-flex flex-wrap align-items-center justify-content-between gap-2 mb-2"),
			html.Div(html.Class("btn-group"), gomponents.Attr("role", "group"), gomponents.Group(views)),
			html.Div(
				html.Class("d-flex align-items-center gap-2"),
				calendarLink(st, func(s *calendarState) { s.Date = s.Step(-1) }, "btn btn-sm btn-outline-secondary",
					gomponents.Attr("aria-label", t("Previous")), html.I(html.Class("fas fa-chevron-left"))),
				calendarLink(st, func(s *calendarState) { s.Date = todayDate }, "btn btn-sm btn-outline-secondary", gomponents.Text(t("Today"))),
				calendarLink(st, func(s *calendarState) { s.Date = s.Step(1) }, "btn btn-sm btn-outline-secondary",
					gomponents.Attr("aria-label", t("Next")), html.I(html.Class("fas fa-chevron-right"))),
				html.H5(html.Class("mb-0 ms-2"), gomponents.Text(calendarPeriodLabel(st))),
			),
			html.Div(html.Class("btn-group flex-wrap"), gomponents.Attr("role", "group"), gomponents.Group(types)),
		),
		html.Div(
			html.Class("d-flex flex-wrap align-items-center justify-content-between gap-2"),
			html.Div(html.Class("d-flex flex-wrap align-items-center gap-3"),
				missingSwitch,
				calendarSelects(st, genres),
				html.Small(html.Class("text-muted"), gomponents.Text(summary)),
			),
			html.Div(html.Class("d-flex flex-wrap gap-2"),
				calendarSearchButton(st, events, t("Search overdue")),
				html.A(
					html.Class("btn btn-sm btn-outline-secondary"),
					html.Href("/api/admin/calendar/ical?"+st.Query(nil)),
					html.Title(t("Download the events shown here as an iCalendar file")),
					html.I(html.Class("fas fa-download me-1")),
					gomponents.Text(t("Download .ics")),
				),
			),
		),
	)
}

// CalendarContentHandler renders the toolbar and the view for the query state.
func CalendarContentHandler(c *gin.Context) {
	st := calendarStateFromQuery(c)
	start, end := st.Range()
	base := st.filter(getCalendarEvents(start, end, st.Filter), false)
	genres := calendarGenres(base)
	events := base

	if st.Genre != "" {
		events = slices.DeleteFunc(slices.Clone(base), func(e calendarEvent) bool { return !calendarHasGenre(e.genre, st.Genre) })
	}

	var view gomponents.Node

	switch st.View {
	case "month":
		view = renderMonthView(st, events)
	case "week":
		view = renderWeekView(st, events)
	default:
		view = renderAgendaView(events)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(http.StatusOK, renderComponentToString(html.Div(renderCalendarToolbar(st, events, genres), view)))
}

func calendarToday() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func dayKey(t time.Time) string { return t.Format("2006-01-02") }

func eventsByDay(events []calendarEvent) map[string][]calendarEvent {
	out := make(map[string][]calendarEvent)
	for _, e := range events {
		out[dayKey(e.Date)] = append(out[dayKey(e.Date)], e)
	}

	return out
}

func renderAgendaView(events []calendarEvent) gomponents.Node {
	if len(events) == 0 {
		return html.P(html.Class("text-muted text-center py-4"), gomponents.Text(t("No events in this period.")))
	}

	byDay := eventsByDay(events)
	today := dayKey(calendarToday())

	var (
		sections []gomponents.Node
		seen     = map[string]bool{}
	)

	for _, e := range events {
		day := dayKey(e.Date)
		if seen[day] {
			continue
		}

		seen[day] = true

		heading := []gomponents.Node{gomponents.Text(e.Date.Format("Monday, January 2, 2006"))}
		class := "text-secondary border-bottom pb-1 mb-2"

		switch {
		case day == today:
			class = "text-primary border-bottom border-primary pb-1 mb-2"

			heading = append(heading, html.Span(html.Class("badge bg-primary ms-2"), gomponents.Text(t("Today"))))
		case day < today:
			class += " opacity-75"
		}

		sections = append(sections, html.Div(
			html.Class("mb-3"),
			html.H6(html.Class(class), gomponents.Group(heading)),
			html.Div(html.Class("list-group"), gomponents.Group(renderEventRows(byDay[day]))),
		))
	}

	return html.Div(sections...)
}

// renderEventRows renders events as compact clickable list rows.
func renderEventRows(events []calendarEvent) []gomponents.Node {
	rows := make([]gomponents.Node, 0, len(events))

	for _, event := range events {
		title := []gomponents.Node{html.Strong(gomponents.Text(event.Title))}
		if ep := episodeCode(event); ep != "" {
			title = append(title, html.Span(html.Class("text-muted ms-2"), gomponents.Text(ep)))
		}

		meta := []gomponents.Node{}
		if event.Listname != "" {
			meta = append(meta, html.Span(html.Class("badge bg-secondary me-1"), gomponents.Text(event.Listname)))
		}

		if event.Network != "" {
			meta = append(meta, html.Span(html.Class("text-info me-2"), gomponents.Text(event.Network)))
		}

		if event.IMDBRating > 0 {
			meta = append(meta, html.Span(html.Class("text-warning"), html.I(html.Class("fas fa-star me-1")),
				gomponents.Text(formatRating(event.IMDBRating))))
		}

		if event.New {
			title = append(title, html.Span(html.Class("badge bg-info ms-2"), gomponents.Text(t("New"))))
		}

		var thumb gomponents.Node = html.I(html.Class("fa-solid " + calendarTypeIcon(event.Type) + " text-muted mt-1"))
		if event.ThumbURL != "" {
			thumb = html.Img(html.Class("rounded flex-shrink-0 calendar-thumb"), html.Src(event.ThumbURL), html.Alt(""), gomponents.Attr("loading", "lazy"))
		}

		rows = append(rows, html.Button(
			html.Type("button"),
			html.Class("list-group-item list-group-item-action d-flex align-items-start gap-3 py-2"),
			html.Style("border-left: 4px solid "+getEventBadgeColor(event)+";"),
			calendarEventAttrs(event),
			thumb,
			html.Div(
				html.Class("flex-grow-1 text-start"),
				html.Div(gomponents.Group(title)),
				gomponents.If(len(meta) > 0, html.Div(html.Class("small"), gomponents.Group(meta))),
				gomponents.If(event.Overview != "", html.Div(html.Class("small text-muted"), gomponents.Text(truncateText(event.Overview, 160)))),
			),
			html.Span(html.Class("badge"), html.Style("background-color: "+getEventBadgeColor(event)+";"), gomponents.Text(calendarStatusLabel(event))),
		))
	}

	return rows
}

func calendarTypeIcon(t string) string {
	switch t {
	case "movie":
		return "fa-film"
	case "series":
		return "fa-tv"
	case "album":
		return "fa-compact-disc"
	case "audiobook":
		return "fa-headphones"
	case "book":
		return "fa-book"
	}

	return "fa-calendar"
}

// monthMaxEvents is how many events a month cell lists before "+N more".
const monthMaxEvents = 3

func renderMonthView(st calendarState, events []calendarEvent) gomponents.Node {
	first, last := st.Range()
	byDay := eventsByDay(events)
	today := calendarToday()

	header := make([]gomponents.Node, 0, 7)
	for _, d := range []string{t("Sun"), t("Mon"), t("Tue"), t("Wed"), t("Thu"), t("Fri"), t("Sat")} {
		header = append(header, html.Div(html.Class("col py-2"), gomponents.Text(d)))
	}

	rows := []gomponents.Node{html.Div(html.Class("row g-0 text-center fw-bold bg-light border-bottom small"), gomponents.Group(header))}

	day := first.AddDate(0, 0, -int(first.Weekday()))
	for day.Compare(last) <= 0 {
		cells := make([]gomponents.Node, 0, 7)

		for range 7 {
			cells = append(cells, renderMonthCell(st, day, byDay[dayKey(day)], day.Month() == first.Month(), day.Equal(today)))
			day = day.AddDate(0, 0, 1)
		}

		rows = append(rows, html.Div(html.Class("row g-0 border-bottom"), gomponents.Group(cells)))
	}

	return html.Div(html.Class("border rounded overflow-hidden"), gomponents.Group(rows))
}

func renderMonthCell(st calendarState, day time.Time, events []calendarEvent, inMonth, isToday bool) gomponents.Node {
	if !inMonth {
		return html.Div(html.Class("col border-end bg-light"), html.Style("min-height: 110px;"))
	}

	number := html.Div(html.Class("small fw-bold mb-1"), gomponents.Text(strconv.Itoa(day.Day())))
	if isToday {
		number = html.Div(html.Class("mb-1"), html.Span(html.Class("badge rounded-pill bg-primary"), gomponents.Text(strconv.Itoa(day.Day()))))
	}

	content := []gomponents.Node{number}

	groups := groupCalendarEvents(events)
	for i, grp := range groups {
		if i == monthMaxEvents {
			content = append(content, html.Button(
				html.Type("button"),
				html.Class("btn btn-link btn-sm p-0 small"),
				html.Data("query", st.Query(nil)),
				gomponents.Attr("onclick", fmt.Sprintf("showMoreEvents('%s', this.dataset.query)", dayKey(day))),
				gomponents.Text(tf("+%d more", calendarEventCount(groups[monthMaxEvents:]))),
			))

			break
		}

		pill := func(event calendarEvent) gomponents.Node {
			label := event.Title
			if ep := episodeCode(event); ep != "" {
				label = ep + " " + label
			}

			if event.New {
				label = "★ " + label
			}

			return html.Div(
				html.Class("badge d-block text-start text-truncate mb-1 fw-normal"),
				html.Style("cursor: pointer; background-color: "+getEventBadgeColor(event)+";"),
				html.Title(label),
				calendarEventAttrs(event),
				gomponents.Text(label),
			)
		}

		if len(grp) == 1 {
			content = append(content, pill(grp[0]))
			continue
		}

		content = append(content, calendarGroupNode(grp, pill))
	}

	class := "col border-end p-1"
	if isToday {
		class += " bg-primary bg-opacity-10"
	}

	return html.Div(html.Class(class), html.Style("min-height: 110px; min-width: 0;"), gomponents.Group(content))
}

func renderWeekView(st calendarState, events []calendarEvent) gomponents.Node {
	first, _ := st.Range()
	byDay := eventsByDay(events)
	today := calendarToday()

	cols := make([]gomponents.Node, 0, 7)

	for i := range 7 {
		day := first.AddDate(0, 0, i)

		headClass := "text-center fw-bold py-2 border-bottom small"
		if day.Equal(today) {
			headClass += " bg-primary text-white"
		} else {
			headClass += " bg-light"
		}

		card := func(event calendarEvent) gomponents.Node {
			label := []gomponents.Node{html.Strong(gomponents.Text(event.Title))}
			if event.New {
				label = append(label, html.Span(html.Class("badge bg-info ms-1"), gomponents.Text(t("New"))))
			}

			if ep := episodeCode(event); ep != "" {
				label = append(label, html.Div(html.Class("opacity-75"), gomponents.Text(ep)))
			}

			if event.Listname != "" {
				label = append(label, html.Div(html.Class("opacity-75 text-truncate"), gomponents.Text(event.Listname)))
			}

			return html.Div(
				html.Class("rounded p-2 mb-2 small"),
				html.Style("cursor: pointer; border-left: 4px solid "+getEventBadgeColor(event)+"; background-color: "+
					getEventBackgroundColor(event)+"; color: "+getEventTextColor(event)+";"),
				calendarEventAttrs(event),
				gomponents.Group(label),
			)
		}

		items := []gomponents.Node{}
		for _, grp := range groupCalendarEvents(byDay[dayKey(day)]) {
			if len(grp) == 1 {
				items = append(items, card(grp[0]))
			} else {
				items = append(items, calendarGroupNode(grp, card))
			}
		}

		cols = append(cols, html.Div(
			html.Class("col border-end"),
			html.Style("min-width: 0;"),
			html.Div(html.Class(headClass), gomponents.Text(day.Format("Mon 2"))),
			html.Div(html.Class("p-1"), html.Style("min-height: 300px;"), gomponents.Group(items)),
		))
	}

	return html.Div(html.Class("border rounded overflow-hidden"), html.Div(html.Class("row g-0"), gomponents.Group(cols)))
}

// CalendarDayEventsHandler lists the events of one day (?date=, with the
// calendar's filter and missing state) for the "+N more" dialog.
func CalendarDayEventsHandler(c *gin.Context) {
	st := calendarStateFromQuery(c)

	events := st.filter(getCalendarEvents(st.Date, st.Date, st.Filter), true)

	var content gomponents.Node = html.P(html.Class("text-muted text-center"), gomponents.Text(t("No events on this day.")))
	if len(events) > 0 {
		content = gomponents.Group([]gomponents.Node{
			gomponents.If(calendarSearchButton(st, events, "") != nil, html.Div(html.Class("mb-2 text-end"), calendarSearchButton(st, events, t("Search all missing of this day")))),
			html.Div(html.Class("list-group"), gomponents.Group(renderEventRows(events))),
		})
	}

	c.String(http.StatusOK, renderComponentToString(content))
}

// calendarStatusLabel is the status, with "Overdue" for released monitored
// items still missing.
func calendarStatusLabel(event calendarEvent) string {
	if event.Overdue && !event.Partial {
		return t("Overdue")
	}

	// The status stays English in the data (the calendar script maps it to
	// its colors); only what is shown is translated.
	return t(event.Status)
}

// groupCalendarEvents groups the episodes of one series (on one day) so a
// site series with many episodes takes one line; other events stay single.
func groupCalendarEvents(events []calendarEvent) [][]calendarEvent {
	var (
		groups [][]calendarEvent
		index  = map[string]int{}
	)

	for _, e := range events {
		key := ""
		if e.Type == "series" {
			key = e.Show
			if key == "" {
				key = e.Title
			}
		}

		if i, ok := index[key]; ok && key != "" {
			groups[i] = append(groups[i], e)
			continue
		}

		if key != "" {
			index[key] = len(groups)
		}

		groups = append(groups, []calendarEvent{e})
	}

	return groups
}

func calendarEventCount(groups [][]calendarEvent) int {
	n := 0
	for _, g := range groups {
		n += len(g)
	}

	return n
}

// calendarGroupNode is a collapsed series ("girlsway ×3") that shows its
// episodes on click; its colour is the most urgent of its episodes.
func calendarGroupNode(grp []calendarEvent, item func(calendarEvent) gomponents.Node) gomponents.Node {
	worst := grp[0]
	for _, e := range grp {
		if e.Overdue || (!e.Downloaded && worst.Downloaded) {
			worst = e
		}
	}

	name := grp[0].Show
	if name == "" {
		name = grp[0].Title
	}

	children := make([]gomponents.Node, 0, len(grp))
	for _, e := range grp {
		children = append(children, item(e))
	}

	return html.Div(html.Class("calendar-group"),
		html.Div(
			html.Class("badge d-block text-start text-truncate mb-1 fw-normal"),
			html.Style("cursor: pointer; background-color: "+getEventBadgeColor(worst)+";"),
			html.Title(fmt.Sprintf("%s: %d episodes (click to show them)", name, len(grp))),
			gomponents.Attr("onclick", "this.parentElement.classList.toggle('open')"),
			html.I(html.Class("fa-solid fa-layer-group me-1")),
			gomponents.Textf("%s ×%d", name, len(grp)),
		),
		html.Div(html.Class("calendar-group-items ps-2"), gomponents.Group(children)),
	)
}

func getEventBadgeColor(event calendarEvent) string {
	if event.Downloaded {
		return "#28a745"
	}

	if event.Partial {
		return "#138496"
	}

	if event.Overdue {
		return "#dc3545"
	}

	if event.Monitored {
		return "#d39e00"
	}

	return "#6c757d"
}

func getEventBackgroundColor(event calendarEvent) string {
	if event.Downloaded {
		return "#d4edda"
	}

	if event.Partial {
		return "#d1ecf1"
	}

	if event.Overdue {
		return "#f8d7da"
	}

	if event.Monitored {
		return "#fff3cd"
	}

	return "#e2e3e5"
}

func getEventTextColor(event calendarEvent) string {
	if event.Downloaded {
		return "#155724"
	}

	if event.Partial {
		return "#0c5460"
	}

	if event.Overdue {
		return "#721c24"
	}

	if event.Monitored {
		return "#856404"
	}

	return "#383d41"
}

func formatRating(rating float64) string {
	return strconv.FormatFloat(rating, 'f', 1, 64)
}

// calendarEventAttrs makes an element open the event details dialog: the
// event goes along as JSON in data-event (attribute-escaped by gomponents).
func calendarEventAttrs(event calendarEvent) gomponents.Node {
	data, err := json.Marshal(event)
	if err != nil {
		return nil
	}

	return gomponents.Group([]gomponents.Node{
		html.Data("event", string(data)),
		gomponents.Attr("onclick", "showEventDetails(JSON.parse(this.dataset.event))"),
	})
}

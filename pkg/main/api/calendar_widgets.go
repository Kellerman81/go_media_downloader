package api

import (
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
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

// CalendarControlsHandler renders the calendar view controls.
func CalendarControlsHandler(c *gin.Context) {
	currentView := c.DefaultQuery("view", "agenda")
	currentFilter := c.DefaultQuery("filter", "all")

	content := html.Div(
		html.Class("row mb-4"),
		html.Div(
			html.Class("col-md-6"),
			html.Div(
				html.Class("btn-group"),
				gomponents.Attr("role", "group"),
				html.Button(
					html.Class(getViewButtonClass("month", currentView)),
					html.ID("btn-month-view"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=month&filter="+currentFilter,
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view=month&filter="+currentFilter+"', '#calendar-controls')",
					),
					gomponents.Text("Month"),
				),
				html.Button(
					html.Class(getViewButtonClass("week", currentView)),
					html.ID("btn-week-view"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=week&filter="+currentFilter,
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view=week&filter="+currentFilter+"', '#calendar-controls')",
					),
					gomponents.Text("Week"),
				),
				html.Button(
					html.Class(getViewButtonClass("agenda", currentView)),
					html.ID("btn-agenda-view"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=agenda&filter="+currentFilter,
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view=agenda&filter="+currentFilter+"', '#calendar-controls')",
					),
					gomponents.Text("Agenda"),
				),
			),
		),
		html.Div(
			html.Class("col-md-6 text-end"),
			html.Div(
				html.Class("btn-group"),
				gomponents.Attr("role", "group"),
				html.Button(
					html.Class(getFilterButtonClass("movies", currentFilter)),
					html.ID("btn-movies"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view="+currentView+"&filter=movies",
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view="+currentView+"&filter=movies', '#calendar-controls')",
					),
					gomponents.Text("Movies"),
				),
				html.Button(
					html.Class(getFilterButtonClass("series", currentFilter)),
					html.ID("btn-series"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view="+currentView+"&filter=series",
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view="+currentView+"&filter=series', '#calendar-controls')",
					),
					gomponents.Text("Series"),
				),
				html.Button(
					html.Class(getFilterButtonClass("albums", currentFilter)),
					html.ID("btn-albums"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view="+currentView+"&filter=albums",
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view="+currentView+"&filter=albums', '#calendar-controls')",
					),
					gomponents.Text("Albums"),
				),
				html.Button(
					html.Class(getFilterButtonClass("audiobooks", currentFilter)),
					html.ID("btn-audiobooks"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view="+currentView+"&filter=audiobooks",
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view="+currentView+"&filter=audiobooks', '#calendar-controls')",
					),
					gomponents.Text("Audiobooks"),
				),
				html.Button(
					html.Class(getFilterButtonClass("all", currentFilter)),
					html.ID("btn-all"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view="+currentView+"&filter=all",
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr(
						"hx-on::after-request",
						"htmx.ajax('GET', '/api/admin/calendar/controls?view="+currentView+"&filter=all', '#calendar-controls')",
					),
					gomponents.Text("All"),
				),
			),
		),
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	content.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// CalendarContentHandler renders the calendar content based on view and filter.
func CalendarContentHandler(c *gin.Context) {
	view := c.DefaultQuery("view", "agenda")
	filter := c.DefaultQuery("filter", "all")
	dateParam := c.Query("date")

	var (
		baseDate time.Time
		err      error
	)

	if dateParam != "" {
		baseDate, err = time.Parse("2006-01-02", dateParam)
		if err != nil {
			baseDate = time.Now()
		}
	} else {
		baseDate = time.Now()
	}

	var start, end time.Time
	switch view {
	case "month":
		start = time.Date(baseDate.Year(), baseDate.Month(), 1, 0, 0, 0, 0, baseDate.Location()).
			AddDate(0, 0, -7)
		end = start.AddDate(0, 2, 0)

	case "week":
		start = baseDate.AddDate(0, 0, -int(baseDate.Weekday())).AddDate(0, 0, -7)
		end = start.AddDate(0, 0, 21)

	default:
		start = time.Now().AddDate(0, 0, -30)
		end = time.Now().AddDate(0, 0, 60)
	}

	events := getCalendarEvents(start, end, filter)

	var content gomponents.Node

	switch view {
	case "month":
		content = renderMonthViewWithFilter(events, filter, baseDate)
	case "week":
		content = renderWeekViewWithFilter(events, filter, baseDate)
	default:
		content = renderAgendaView(events)
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	content.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

func renderAgendaView(events []calendarEvent) gomponents.Node {
	if len(events) == 0 {
		return html.P(
			html.Class("text-muted text-center py-4"),
			gomponents.Text("No upcoming events"),
		)
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Date.Before(events[j].Date)
	})

	groupedEvents := make(map[string][]calendarEvent)
	for _, event := range events {
		date := event.Date.Format("2006-01-02")

		groupedEvents[date] = append(groupedEvents[date], event)
	}

	var dates []string
	for date := range groupedEvents {
		dates = append(dates, date)
	}

	sort.Strings(dates)

	var sections []gomponents.Node
	for _, date := range dates {
		dayEvents := groupedEvents[date]
		parsedDate, _ := time.Parse("2006-01-02", date)

		sections = append(sections, html.Div(
			html.Class("agenda-date mb-3"),
			html.H5(
				html.Class("text-secondary border-bottom pb-2"),
				gomponents.Text(parsedDate.Format("Monday, January 2, 2006")),
			),
			gomponents.Group(renderEventCards(dayEvents)),
		))
	}

	return html.Div(sections...)
}

func renderMonthViewWithFilter(
	events []calendarEvent,
	filter string,
	monthDate ...time.Time,
) gomponents.Node {
	var baseDate time.Time
	if len(monthDate) > 0 {
		baseDate = monthDate[0]
	} else {
		baseDate = time.Now()
	}

	currentMonth := baseDate.Month()
	currentYear := baseDate.Year()

	eventsByDay := make(map[int][]calendarEvent)
	for _, event := range events {
		if event.Date.Month() == currentMonth && event.Date.Year() == currentYear {
			day := event.Date.Day()

			eventsByDay[day] = append(eventsByDay[day], event)
		}
	}

	monthNames := []string{
		"", "January", "February", "March", "April", "May", "June",
		"July", "August", "September", "October", "November", "December",
	}

	firstDay := time.Date(currentYear, currentMonth, 1, 0, 0, 0, 0, time.UTC)
	lastDay := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, time.UTC)
	daysInMonth := lastDay.Day()
	startDay := int(firstDay.Weekday())

	var calendarRows []gomponents.Node

	calendarRows = append(calendarRows, html.Div(
		html.Class("row text-center fw-bold bg-light py-3 border-bottom"),
		html.Style("font-size: 0.875rem;"),
		html.Div(html.Class("col"), gomponents.Text("Sun")),
		html.Div(html.Class("col"), gomponents.Text("Mon")),
		html.Div(html.Class("col"), gomponents.Text("Tue")),
		html.Div(html.Class("col"), gomponents.Text("Wed")),
		html.Div(html.Class("col"), gomponents.Text("Thu")),
		html.Div(html.Class("col"), gomponents.Text("Fri")),
		html.Div(html.Class("col"), gomponents.Text("Sat")),
	))

	currentDate := 1
	for week := range 6 {
		if currentDate > daysInMonth {
			break
		}

		var weekCells []gomponents.Node
		for day := range 7 {
			cellContent := []gomponents.Node{}

			if week != 0 || day >= startDay {
				if currentDate <= daysInMonth {
					cellContent = append(cellContent, html.Div(
						html.Class("day-number fw-bold mb-2"),
						html.Style("color: #495057; font-size: 1rem;"),
						gomponents.Text(strconv.Itoa(currentDate)),
					))

					if dayEvents, exists := eventsByDay[currentDate]; exists {
						for i, event := range dayEvents {
							if i >= 3 {
								break
							}

							cellContent = append(cellContent, html.Div(
								html.Class("event-item badge d-block mb-1 text-truncate"),
								html.Style(
									"font-size: 0.7rem; padding: 4px 8px; color: white; background-color: "+getEventBadgeColor(
										event,
									)+"; border-radius: 4px; cursor: pointer;",
								),
								html.Title(event.Title),
								gomponents.Attr("onclick", buildShowEventDetailsCall(event)),
								gomponents.Text(truncateText(event.Title, 12)),
							))
						}

						if len(dayEvents) > 3 {
							cellContent = append(cellContent, html.Small(
								html.Class("text-muted"),
								html.Style("cursor: pointer; text-decoration: underline;"),
								gomponents.Attr(
									"onclick",
									"showMoreEvents("+strconv.Itoa(
										currentDate,
									)+", '"+currentMonth.String()+"', "+strconv.Itoa(
										currentYear,
									)+")",
								),
								gomponents.Text("+"+strconv.Itoa(len(dayEvents)-3)+" more"),
							))
						}
					}
				}

				currentDate++
			}

			weekCells = append(weekCells, html.Div(
				html.Class("col border-end p-3"),
				html.Style("min-height: 120px; background-color: #fff;"),
				gomponents.Group(cellContent),
			))
		}

		calendarRows = append(calendarRows, html.Div(
			html.Class("row border-top"),
			gomponents.Group(weekCells),
		))
	}

	prevMonth := baseDate.AddDate(0, -1, 0)
	nextMonth := baseDate.AddDate(0, 1, 0)

	return html.Div(
		html.Class("month-view"),
		html.Div(
			html.Class("month-header mb-4 p-3 bg-light rounded"),
			html.Style("border: 1px solid #dee2e6;"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-center"),
				html.Button(
					html.Class("btn btn-outline-secondary btn-sm"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=month&filter="+filter+"&date="+prevMonth.Format(
							"2006-01-02",
						),
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr("hx-trigger", "click"),
					html.I(html.Class("fas fa-chevron-left me-1")),
					gomponents.Text("Prev"),
				),
				html.H4(
					html.Class("mb-0 fw-bold"),
					html.Style("color: #495057;"),
					gomponents.Text(monthNames[currentMonth]+" "+strconv.Itoa(currentYear)),
				),
				html.Button(
					html.Class("btn btn-outline-secondary btn-sm"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=month&filter="+filter+"&date="+nextMonth.Format(
							"2006-01-02",
						),
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Text("Next"),
					html.I(html.Class("fas fa-chevron-right ms-1")),
				),
			),
		),
		html.Div(
			html.Class("calendar-grid border rounded"),
			html.Style("border-color: #dee2e6 !important; background-color: #fff;"),
			gomponents.Group(calendarRows),
		),
	)
}

func renderWeekViewWithFilter(
	events []calendarEvent,
	filter string,
	weekDate ...time.Time,
) gomponents.Node {
	var baseDate time.Time
	if len(weekDate) > 0 {
		baseDate = weekDate[0]
	} else {
		baseDate = time.Now()
	}

	startOfWeek := baseDate.AddDate(0, 0, -int(baseDate.Weekday()))

	eventsByDay := make([][]calendarEvent, 7)

	endOfWeek := startOfWeek.AddDate(0, 0, 7)
	for _, event := range events {
		if (event.Date.Equal(startOfWeek) || event.Date.After(startOfWeek)) &&
			event.Date.Before(endOfWeek) {
			dayOfWeek := int(event.Date.Weekday())

			eventsByDay[dayOfWeek] = append(eventsByDay[dayOfWeek], event)
		}
	}

	dayNames := []string{
		"Sunday",
		"Monday",
		"Tuesday",
		"Wednesday",
		"Thursday",
		"Friday",
		"Saturday",
	}

	var weekCells []gomponents.Node

	for day := range 7 {
		currentDay := startOfWeek.AddDate(0, 0, day)

		var dayContent []gomponents.Node

		dayContent = append(dayContent, html.Div(
			html.Class("day-date fw-bold mb-2"),
			gomponents.Text(strconv.Itoa(currentDay.Day())),
		))

		for _, event := range eventsByDay[day] {
			dayContent = append(dayContent, html.Div(
				html.Class("event-item card card-body p-2 mb-2"),
				html.Style(
					"font-size: 0.8em; border-left: 4px solid "+getEventBadgeColor(
						event,
					)+" !important; background-color: "+getEventBackgroundColor(
						event,
					)+"; cursor: pointer;",
				),
				gomponents.Attr("onclick", buildShowEventDetailsCall(event)),
				html.Strong(
					html.Style("color: "+getEventTextColor(event)+";"),
					gomponents.Text(event.Title),
				),
				func() gomponents.Node {
					if event.Type == "series" && event.Season != "0" && event.Season != "" &&
						event.Episode > 0 {
						return gomponents.Group([]gomponents.Node{
							html.Br(),
							html.Small(
								html.Style("color: "+getEventTextColor(event)+"; opacity: 0.8;"),
								gomponents.Text(
									"S"+padZero(
										logger.StringToInt(event.Season),
									)+"E"+padZero(
										event.Episode,
									),
								),
							),
						})
					}

					return nil
				}(),
				func() gomponents.Node {
					if event.Listname != "" {
						return gomponents.Group([]gomponents.Node{
							html.Br(),
							html.Small(
								html.Class("badge bg-secondary"),
								html.Style("font-size: 0.7em;"),
								gomponents.Text(event.Listname),
							),
						})
					}

					return nil
				}(),
				func() gomponents.Node {
					if event.Network != "" {
						return gomponents.Group([]gomponents.Node{
							html.Br(),
							html.Small(
								html.Class("text-muted"),
								gomponents.Text(event.Network),
							),
						})
					}

					return nil
				}(),
			))
		}

		weekCells = append(weekCells, html.Div(
			html.Class("col border-end p-2"),
			html.Style("min-height: 300px;"),
			gomponents.Group(dayContent),
		))
	}

	prevWeek := startOfWeek.AddDate(0, 0, -7)
	nextWeek := startOfWeek.AddDate(0, 0, 7)

	return html.Div(
		html.Class("week-view"),
		html.Div(
			html.Class("week-header mb-3"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-center"),
				html.Button(
					html.Class("btn btn-outline-secondary"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=week&filter="+filter+"&date="+prevWeek.Format(
							"2006-01-02",
						),
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr("hx-trigger", "click"),
					html.I(html.Class("fas fa-chevron-left me-1")),
					gomponents.Text("Previous"),
				),
				html.H4(
					html.Class("mb-0"),
					gomponents.Text("Week of "+startOfWeek.Format("January 2, 2006")),
				),
				html.Button(
					html.Class("btn btn-outline-secondary"),
					gomponents.Attr(
						"hx-get",
						"/api/admin/calendar/content?view=week&filter="+filter+"&date="+nextWeek.Format(
							"2006-01-02",
						),
					),
					gomponents.Attr("hx-target", "#calendar-content"),
					gomponents.Attr("hx-swap", "innerHTML"),
					gomponents.Attr("hx-trigger", "click"),
					gomponents.Text("Next"),
					html.I(html.Class("fas fa-chevron-right ms-1")),
				),
			),
		),
		html.Div(
			html.Class("row text-center fw-bold bg-light py-2"),
			gomponents.Group(func() []gomponents.Node {
				var headers []gomponents.Node
				for _, name := range dayNames {
					headers = append(headers, html.Div(html.Class("col"), gomponents.Text(name)))
				}

				return headers
			}()),
		),
		html.Div(
			html.Class("row border-top"),
			gomponents.Group(weekCells),
		),
	)
}

func renderEventCards(events []calendarEvent) []gomponents.Node {
	var cards []gomponents.Node

	for _, event := range events {
		cards = append(cards, html.Div(
			html.Class("card mb-2"),
			html.Style("cursor: pointer;"),
			gomponents.Attr("onclick", buildShowEventDetailsCall(event)),
			html.Div(
				html.Class("card-body py-3"),
				html.Div(
					html.Class("row align-items-center"),
					html.Div(
						html.Class("col-md-8"),
						html.H6(
							html.Class("mb-1"),
							gomponents.Text(event.Title),
							func() gomponents.Node {
								if event.Type == "series" && event.Season != "0" &&
									event.Season != "" &&
									event.Episode > 0 {
									return html.Small(
										html.Class("text-muted ms-2"),
										gomponents.Text(
											"S"+padZero(
												logger.StringToInt(event.Season),
											)+"E"+padZero(
												event.Episode,
											),
										),
									)
								}

								return nil
							}(),
						),
						func() gomponents.Node {
							if event.Listname != "" {
								return html.Small(
									html.Class("badge bg-secondary me-2 mb-1"),
									gomponents.Text(event.Listname),
								)
							}

							return nil
						}(),
						func() gomponents.Node {
							if event.Overview != "" {
								truncated := event.Overview
								if len(truncated) > 200 {
									truncated = truncated[:200] + "..."
								}

								return html.P(
									html.Class("text-muted mb-2"),
									html.Style("font-size: 0.9rem; line-height: 1.4;"),
									gomponents.Text(truncated),
								)
							}

							return nil
						}(),
						func() gomponents.Node {
							if event.Network != "" {
								return html.Small(
									html.Class("text-info"),
									gomponents.Text(event.Network),
								)
							}

							return nil
						}(),
					),
					html.Div(
						html.Class("col-md-4 text-end"),
						html.Span(
							html.Class("badge"),
							html.Style("background-color: "+getEventBadgeColor(event)+";"),
							gomponents.Text(event.Status),
						),
						func() gomponents.Node {
							if event.IMDBRating > 0 {
								return gomponents.Group([]gomponents.Node{
									html.Br(),
									html.Small(
										html.Class("text-warning"),
										gomponents.Text("* "+formatRating(event.IMDBRating)),
									),
								})
							}

							return nil
						}(),
					),
				),
			),
		))
	}

	return cards
}

// CalendarDayEventsHandler fetches all events for a specific day.
func CalendarDayEventsHandler(c *gin.Context) {
	dayStr := c.Query("day")
	monthStr := c.Query("month")
	yearStr := c.DefaultQuery("year", strconv.Itoa(time.Now().Year()))

	day, err := strconv.Atoi(dayStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid day parameter")
		return
	}

	year, err := strconv.Atoi(yearStr)
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid year parameter")
		return
	}

	months := map[string]time.Month{
		"January": time.January, "February": time.February, "March": time.March,
		"April": time.April, "May": time.May, "June": time.June,
		"July": time.July, "August": time.August, "September": time.September,
		"October": time.October, "November": time.November, "December": time.December,
	}

	month, exists := months[monthStr]
	if !exists {
		c.String(http.StatusBadRequest, "Invalid month parameter")
		return
	}

	targetDate := time.Date(year, month, day, 0, 0, 0, 0, time.UTC)

	startOfDay := targetDate
	endOfDay := targetDate.Add(24 * time.Hour)
	allEvents := getCalendarEvents(startOfDay, endOfDay, "all")

	var dayEvents []calendarEvent

	for _, event := range allEvents {
		if event.Date.Year() == targetDate.Year() &&
			event.Date.Month() == targetDate.Month() &&
			event.Date.Day() == targetDate.Day() {
			dayEvents = append(dayEvents, event)
		}
	}

	content := html.Div()
	if len(dayEvents) == 0 {
		content = html.P(
			html.Class("text-muted text-center"),
			gomponents.Text("No events scheduled for this day."),
		)
	} else {
		var eventCards []gomponents.Node
		for _, event := range dayEvents {
			eventCards = append(eventCards, html.Div(
				html.Class("card mb-2"),
				html.Style("cursor: pointer;"),
				gomponents.Attr("onclick", buildShowEventDetailsCall(event)),
				html.Div(
					html.Class("card-body py-2"),
					html.H6(
						html.Class("card-title mb-1"),
						gomponents.Text(event.Title),
						func() gomponents.Node {
							if event.Type == "series" && event.Season != "0" &&
								event.Season != "" &&
								event.Episode > 0 {
								return html.Small(
									html.Class("text-muted ms-2"),
									gomponents.Text(
										"S"+padZero(
											logger.StringToInt(event.Season),
										)+"E"+padZero(
											event.Episode,
										),
									),
								)
							}

							return nil
						}(),
					),
					func() gomponents.Node {
						if event.Listname != "" {
							return html.Small(
								html.Class("badge bg-secondary me-2 mb-1"),
								gomponents.Text(event.Listname),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if event.Overview != "" {
							truncated := event.Overview
							if len(truncated) > 150 {
								truncated = truncated[:150] + "..."
							}

							return html.P(
								html.Class("card-text text-muted small"),
								gomponents.Text(truncated),
							)
						}

						return nil
					}(),
					html.Div(
						html.Class("d-flex justify-content-between align-items-center"),
						func() gomponents.Node {
							if event.Network != "" {
								return html.Small(
									html.Class("text-info"),
									gomponents.Text(event.Network),
								)
							}

							return html.Span()
						}(),
						html.Span(
							html.Class("badge"),
							html.Style("background-color: "+getEventBadgeColor(event)),
							gomponents.Text(event.Status),
						),
					),
				),
			))
		}

		content = html.Div(gomponents.Group(eventCards))
	}

	var buf strings.Builder
	content.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// Helper functions.
func getViewButtonClass(view, current string) string {
	base := "btn btn-outline-secondary"
	if view == current {
		return base + " active"
	}

	return base
}

func getFilterButtonClass(filter, current string) string {
	base := "btn btn-outline-secondary"
	if filter == current {
		return base + " active"
	}

	return base
}

func getEventBadgeColor(event calendarEvent) string {
	if event.Downloaded {
		return "#28a745"
	}

	if event.Monitored {
		return "#ffc107"
	}

	return "#6c757d"
}

func getEventBackgroundColor(event calendarEvent) string {
	if event.Downloaded {
		return "#d4edda"
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

	if event.Monitored {
		return "#856404"
	}

	return "#383d41"
}

func padZero(num int) string {
	if num < 10 {
		return "0" + strconv.Itoa(num)
	}

	return strconv.Itoa(num)
}

func formatRating(rating float64) string {
	return strconv.FormatFloat(rating, 'f', 1, 64)
}

// buildShowEventDetailsCall generates the onclick attribute for showing event details.
func buildShowEventDetailsCall(event calendarEvent) string {
	downloaded := "false"
	if event.Downloaded {
		downloaded = "true"
	}

	return "showEventDetails(" + strconv.FormatUint(
		uint64(event.ID),
		10,
	) + ", '" + escapeJSString(
		event.Title,
	) + "', '" + escapeJSString(
		event.Type,
	) + "', '" + escapeJSString(
		event.Network,
	) + "', " + strconv.Itoa(
		logger.StringToInt(event.Season),
	) + ", " + strconv.Itoa(
		event.Episode,
	) + ", '" + event.Date.Format(
		"2006-01-02",
	) + "', '" + escapeJSString(
		event.Overview,
	) + "', '" + escapeJSString(
		event.IMDBID,
	) + "', " + strconv.Itoa(
		event.TheTVDBID,
	) + ", " + strconv.Itoa(
		event.MovieDBID,
	) + ", " + strconv.Itoa(
		event.TraktID,
	) + ", " + downloaded + ", '" + escapeJSString(
		event.Listname,
	) + "')"
}

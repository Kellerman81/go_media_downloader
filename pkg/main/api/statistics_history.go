package api

import (
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// historyMedia describes one media type's tables for the history-based
// statistics. Items are always aliased "i"; from may join the table that
// carries the list name (episodes take it from their series).
type historyMedia struct {
	label     string
	kind      string // mediaTypeConfigured kind; empty = always shown
	from      string
	listCol   string
	hist      string
	histKey   string
	titleFrom string // joins aliased "t"/"te" for titleExpr
	titleExpr string
	resExpr   string // current resolution of a video item, empty for audio/books
	color     string
}

var historyMedias = []historyMedia{
	{
		label:     "Movies",
		from:      "movies i",
		listCol:   "i.listname",
		hist:      "movie_histories",
		histKey:   "movie_id",
		titleFrom: "JOIN dbmovies t ON t.id = i.dbmovie_id",
		titleExpr: "t.title",
		resExpr:   "(SELECT q.name FROM movie_files f JOIN qualities q ON q.id = f.resolution_id WHERE f.movie_id = i.id LIMIT 1)",
		color:     "#3b7ddd",
	},
	{
		label:     "Episodes",
		from:      "serie_episodes i JOIN series s ON s.id = i.serie_id",
		listCol:   "s.listname",
		hist:      "serie_episode_histories",
		histKey:   "serie_episode_id",
		titleFrom: "JOIN dbseries t ON t.id = i.dbserie_id JOIN dbserie_episodes te ON te.id = i.dbserie_episode_id",
		titleExpr: "t.seriename || ' ' || te.identifier",
		resExpr:   "(SELECT q.name FROM serie_episode_files f JOIN qualities q ON q.id = f.resolution_id WHERE f.serie_episode_id = i.id LIMIT 1)",
		color:     "#1cbb8c",
	},
	{
		label:     "Music",
		kind:      "music",
		from:      "albums i",
		listCol:   "i.listname",
		hist:      "album_histories",
		histKey:   "album_id",
		titleFrom: "JOIN dbalbums t ON t.id = i.dbalbum_id",
		titleExpr: "t.title",
		color:     "#dc3545",
	},
	{
		label:     "Books",
		kind:      "book",
		from:      "books i",
		listCol:   "i.listname",
		hist:      "book_histories",
		histKey:   "book_id",
		titleFrom: "JOIN dbbooks t ON t.id = i.dbbook_id",
		titleExpr: "t.title",
		color:     "#fcb92c",
	},
	{
		label:     "Audiobooks",
		kind:      "audiobook",
		from:      "audiobooks i",
		listCol:   "i.listname",
		hist:      "audiobook_histories",
		histKey:   "audiobook_id",
		titleFrom: "JOIN dbaudiobooks t ON t.id = i.dbaudiobook_id",
		titleExpr: "t.title",
		color:     "#17a2b8",
	},
}

// activeHistoryMedias returns the media types that are configured.
func activeHistoryMedias() []historyMedia {
	out := make([]historyMedia, 0, len(historyMedias))
	for _, m := range historyMedias {
		if m.kind == "" || mediaTypeConfigured(m.kind) {
			out = append(out, m)
		}
	}

	return out
}

// daysAgo returns the date n days back in the "YYYY-MM-DD" form every stored
// timestamp starts with, so it compares correctly as a string.
func daysAgo(n int) string {
	return time.Now().AddDate(0, 0, -n).Format("2006-01-02")
}

func countsByStr(rows []database.DbstaticOneStringOneInt) map[string]int {
	m := make(map[string]int, len(rows))
	for _, r := range rows {
		m[r.Str] += r.Num
	}

	return m
}

// --- Download activity -------------------------------------------------------

// activityRange is a selectable time frame of the download activity section.
// Downloads are grouped into buckets keyed by a prefix of downloaded_at, which
// always starts with "YYYY-MM-DD HH:MM:SS" (older rows add fractions and a zone).
type activityRange struct {
	key, label, chartTitle string
	buckets                int
	sqlKey                 string // SQL expression giving a row's bucket key
	keyFormat, labelFormat string // Go layouts of the bucket key and chart label
	start                  func(now time.Time) time.Time
	step                   func(t time.Time, i int) time.Time
}

var activityRanges = []activityRange{
	{
		key: "hour", label: "Last hour", chartTitle: "Downloads per 5 minutes (last hour)", buckets: 12,
		sqlKey:    "substr(downloaded_at, 1, 14) || printf('%02d', CAST(substr(downloaded_at, 15, 2) AS INTEGER) / 5 * 5)",
		keyFormat: "2006-01-02 15:04", labelFormat: "15:04",
		start: func(now time.Time) time.Time { return now.Truncate(5 * time.Minute).Add(-55 * time.Minute) },
		step:  func(t time.Time, i int) time.Time { return t.Add(time.Duration(i) * 5 * time.Minute) },
	},
	{
		key: "day", label: "Last day", chartTitle: "Downloads per hour (last 24 hours)", buckets: 24,
		sqlKey:    "substr(downloaded_at, 1, 13)",
		keyFormat: "2006-01-02 15", labelFormat: "15:00",
		start: func(now time.Time) time.Time { return now.Truncate(time.Hour).Add(-23 * time.Hour) },
		step:  func(t time.Time, i int) time.Time { return t.Add(time.Duration(i) * time.Hour) },
	},
	{
		key: "week", label: "Last week", chartTitle: "Downloads per day (last 7 days)", buckets: 7,
		sqlKey:    "substr(downloaded_at, 1, 10)",
		keyFormat: "2006-01-02", labelFormat: "Mon 02",
		start: func(now time.Time) time.Time { return startOfDay(now).AddDate(0, 0, -6) },
		step:  func(t time.Time, i int) time.Time { return t.AddDate(0, 0, i) },
	},
	{
		key: "month", label: "Last month", chartTitle: "Downloads per day (last 30 days)", buckets: 30,
		sqlKey:    "substr(downloaded_at, 1, 10)",
		keyFormat: "2006-01-02", labelFormat: "Jan 02",
		start: func(now time.Time) time.Time { return startOfDay(now).AddDate(0, 0, -29) },
		step:  func(t time.Time, i int) time.Time { return t.AddDate(0, 0, i) },
	},
	{
		key: "year", label: "Last year", chartTitle: "Downloads per month (last 12 months)", buckets: 12,
		sqlKey:    "substr(downloaded_at, 1, 7)",
		keyFormat: "2006-01", labelFormat: "Jan 2006",
		start: func(now time.Time) time.Time {
			return time.Date(now.Year(), now.Month()-11, 1, 0, 0, 0, 0, now.Location())
		},
		step: func(t time.Time, i int) time.Time { return t.AddDate(0, i, 0) },
	},
}

// defaultActivityRange is used when no or an unknown range is requested.
const defaultActivityRange = "year"

func findActivityRange(key string) activityRange {
	for _, r := range activityRanges {
		if r.key == key {
			return r
		}
	}

	return findActivityRange(defaultActivityRange)
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

type downloadActivity struct {
	Range     activityRange
	Labels    []string
	Media     []historyMedia
	PerBucket [][]int // [media][bucket]
	Total     []int
	Repeat    []int
	Indexers  []indexerShare
	Looping   []repeatedItem
}

// repeatedItem is an item downloaded again and again - usually a grab that
// never satisfies it (failed or wrong import), so it stays wanted.
type repeatedItem struct {
	Type      string
	Title     string `db:"title"`
	Downloads int    `db:"downloads"`
	Releases  int    `db:"releases"`
	Last      string `db:"last"`
}

type indexerShare struct {
	Name    string
	PerType []int
	Total   int
}

func getDownloadActivity(rng activityRange) downloadActivity {
	start := rng.start(time.Now())

	a := downloadActivity{Range: rng, Media: activeHistoryMedias()}
	keys := make([]string, rng.buckets)

	for i := range rng.buckets {
		t := rng.step(start, i)
		keys[i] = t.Format(rng.keyFormat)
		a.Labels = append(a.Labels, t.Format(rng.labelFormat))
	}

	since := start.Format("2006-01-02 15:04:05")
	indexers := map[string]*indexerShare{}

	for mi, m := range a.Media {
		byBucket := countsByStr(database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(rng.buckets),
			"SELECT "+rng.sqlKey+", COUNT(*) FROM "+m.hist+
				" WHERE downloaded_at >= ? GROUP BY 1", since))

		row := make([]int, rng.buckets)
		total := 0

		for i, key := range keys {
			row[i] = byBucket[key]
			total += row[i]
		}

		a.PerBucket = append(a.PerBucket, row)
		a.Total = append(a.Total, total)
		a.Repeat = append(a.Repeat, database.Getdatarow[int](false,
			"SELECT COUNT(*) FROM "+m.hist+" h WHERE h.downloaded_at >= ? AND EXISTS (SELECT 1 FROM "+
				m.hist+" p WHERE p."+m.histKey+" = h."+m.histKey+" AND p.downloaded_at < h.downloaded_at)",
			since))

		for _, r := range database.StructscanT[repeatedItem](false, 10,
			"SELECT "+m.titleExpr+" AS title, COUNT(*) AS downloads, COUNT(DISTINCT h.title) AS releases,"+
				" MAX(h.downloaded_at) AS last FROM "+m.hist+" h, "+m.from+" "+m.titleFrom+
				" WHERE i.id = h."+m.histKey+" AND h.downloaded_at >= ? GROUP BY h."+m.histKey+
				" HAVING COUNT(*) >= 3 ORDER BY downloads DESC LIMIT 10", since) {
			r.Type = m.label
			a.Looping = append(a.Looping, r)
		}

		for _, r := range database.GetrowsN[database.DbstaticOneStringOneInt](false, 0,
			"SELECT COALESCE(NULLIF(indexer, ''), '(unknown)'), COUNT(*) FROM "+m.hist+
				" WHERE downloaded_at >= ? GROUP BY 1", since) {
			s := indexers[r.Str]
			if s == nil {
				s = &indexerShare{Name: r.Str, PerType: make([]int, len(a.Media))}
				indexers[r.Str] = s
			}

			s.PerType[mi] += r.Num
			s.Total += r.Num
		}
	}

	for _, s := range indexers {
		a.Indexers = append(a.Indexers, *s)
	}

	slices.SortFunc(a.Indexers, func(x, y indexerShare) int { return y.Total - x.Total })
	slices.SortFunc(a.Looping, func(x, y repeatedItem) int { return y.Downloads - x.Downloads })

	if len(a.Looping) > 15 {
		a.Looping = a.Looping[:15]
	}

	return a
}

func renderDownloadActivity(a downloadActivity) gomponents.Node {
	type dataset struct {
		Label           string `json:"label"`
		Data            []int  `json:"data"`
		BackgroundColor string `json:"backgroundColor"`
	}

	datasets := make([]dataset, 0, len(a.Media))
	for i, m := range a.Media {
		datasets = append(datasets, dataset{m.label, a.PerBucket[i], m.color})
	}

	// json.Marshal escapes <, > and &, so the data is safe inside <script>.
	labelsJSON, _ := json.Marshal(a.Labels)
	dataJSON, _ := json.Marshal(datasets)

	summaryRows := make([]gomponents.Node, 0, len(a.Media))
	for i, m := range a.Media {
		summaryRows = append(summaryRows, html.Tr(
			html.Td(html.Strong(gomponents.Text(m.label))),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", a.Total[i])),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", a.Repeat[i])),
			html.Td(html.Class("text-end"), gomponents.Text(percent(a.Repeat[i], a.Total[i]))),
		))
	}

	indexerTotal := 0
	for _, s := range a.Indexers {
		indexerTotal += s.Total
	}

	indexerHead := []gomponents.Node{html.Th(gomponents.Text("Indexer"))}
	for _, m := range a.Media {
		indexerHead = append(indexerHead, html.Th(html.Class("text-end"), gomponents.Text(m.label)))
	}

	indexerHead = append(indexerHead,
		html.Th(html.Class("text-end"), gomponents.Text("Total")),
		html.Th(html.Style("width: 25%"), gomponents.Text("Share")))

	indexerRows := make([]gomponents.Node, 0, len(a.Indexers))
	for _, s := range a.Indexers {
		cells := []gomponents.Node{html.Td(html.Strong(gomponents.Text(s.Name)))}
		for _, n := range s.PerType {
			cells = append(cells, html.Td(html.Class("text-end"), gomponents.Textf("%d", n)))
		}

		cells = append(cells,
			html.Td(html.Class("text-end"), gomponents.Textf("%d", s.Total)),
			html.Td(shareBar(s.Total, indexerTotal, "bg-primary")))
		indexerRows = append(indexerRows, html.Tr(cells...))
	}

	if len(indexerRows) == 0 {
		indexerRows = append(indexerRows, emptyRow(len(indexerHead), "No downloads in this period"))
	}

	period := strings.ToLower(a.Range.label)

	// htmx settles a swapped-in element that shares an id with the one it
	// replaces by giving it the old attributes first and restoring its own
	// afterwards - which, after Chart.js has sized the canvas, resets it to
	// the default 300x150. A fresh id per render keeps htmx out of it.
	canvasID := fmt.Sprintf("stat-downloads-chart-%d", time.Now().UnixNano())

	return html.Div(html.ID("stat-activity"), statSection("bg-primary text-white", "fa-solid fa-download", "Download Activity",
		activityRangeButtons(a.Range.key, "/api/admin/statistics/activity", "stat-activity"),
		html.H6(html.Class("text-muted"), gomponents.Text(a.Range.chartTitle)),
		html.Div(html.Style("height: 280px;"), html.Canvas(html.ID(canvasID))),
		html.Script(gomponents.Raw(`
(function() {
	var el = document.getElementById('`+canvasID+`');
	if (!el || !window.Chart) { return; }
	Chart.helpers.each(Chart.instances, function(c) {
		if (!document.body.contains(c.canvas)) { c.destroy(); }
	});
	new Chart(el, {
		type: 'bar',
		data: {labels: `+string(labelsJSON)+`, datasets: `+string(dataJSON)+`},
		options: {
			responsive: true,
			maintainAspectRatio: false,
			scales: {
				xAxes: [{stacked: true, gridLines: {display: false}}],
				yAxes: [{stacked: true, ticks: {beginAtZero: true, precision: 0}}]
			}
		}
	});
})();
`)),
		html.Div(
			html.Class("row mt-4"),
			html.Div(
				html.Class("col-lg-4"),
				html.H6(html.Class("text-muted"), gomponents.Text(a.Range.label)),
				statTable(
					[]gomponents.Node{
						html.Th(gomponents.Text("Type")),
						html.Th(html.Class("text-end"), gomponents.Text("Downloads")),
						html.Th(
							html.Class("text-end"),
							html.Title("Downloads of items that had been downloaded before (upgrades and re-grabs)"),
							gomponents.Text("Repeat"),
						),
						html.Th(html.Class("text-end"), gomponents.Text("Repeat %")),
					},
					summaryRows,
				),
			),
			html.Div(
				html.Class("col-lg-8"),
				html.H6(html.Class("text-muted"), gomponents.Text("Downloads per indexer ("+period+")")),
				statTable(indexerHead, indexerRows),
			),
		),
		html.H6(html.Class("text-muted mt-4"), gomponents.Text("Most re-downloaded items ("+period+", 3+ downloads)")),
		html.P(
			html.Class("text-muted small mb-2"),
			gomponents.Text("An item downloaded over and over usually means its downloads never satisfy it - the import fails or matches something else - so it stays wanted and is grabbed again."),
		),
		statTable([]gomponents.Node{
			html.Th(gomponents.Text("Item")),
			html.Th(gomponents.Text("Type")),
			html.Th(html.Class("text-end"), gomponents.Text("Downloads")),
			html.Th(html.Class("text-end"), html.Title("Distinct release names grabbed"), gomponents.Text("Releases")),
			html.Th(gomponents.Text("Last download")),
		}, loopingRows(a.Looping)),
	))
}

// activityRangeButtons switches a statistics section to another time frame;
// the section, wrapped in an element with the given id, reloads itself in place.
func activityRangeButtons(current, endpoint, target string) gomponents.Node {
	buttons := make([]gomponents.Node, 0, len(activityRanges)+2)
	buttons = append(buttons, html.Class("btn-group btn-group-sm mb-3"), html.Role("group"),
		gomponents.Attr("aria-label", "Time frame"))

	for _, r := range activityRanges {
		cls := "btn btn-outline-primary"
		if r.key == current {
			cls = "btn btn-primary"
		}

		buttons = append(buttons, html.Button(
			html.Type("button"),
			html.Class(cls),
			gomponents.Attr("aria-pressed", fmt.Sprintf("%t", r.key == current)),
			hx.Get(endpoint+"?range="+r.key),
			hx.Target("#"+target),
			hx.Swap("outerHTML"),
			gomponents.Text(r.label),
		))
	}

	return html.Div(buttons...)
}

func loopingRows(items []repeatedItem) []gomponents.Node {
	if len(items) == 0 {
		return []gomponents.Node{emptyRow(5, "No item was downloaded 3 or more times")}
	}

	rows := make([]gomponents.Node, 0, len(items))
	for _, it := range items {
		cls := "text-end"
		if it.Downloads >= 10 {
			cls += " text-danger fw-semibold"
		}

		rows = append(rows, html.Tr(
			html.Td(gomponents.Text(truncateString(it.Title, 70))),
			html.Td(gomponents.Text(it.Type)),
			html.Td(html.Class(cls), gomponents.Textf("%d", it.Downloads)),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", it.Releases)),
			html.Td(html.Class("text-nowrap"), html.Small(gomponents.Text(truncateString(it.Last, 19)))),
		))
	}

	return rows
}

// --- Wanted backlog ----------------------------------------------------------

var backlogBuckets = []string{"< 30 days", "30–180 days", "180 days – 1 year", "> 1 year", "unknown"}

type backlogStats struct {
	Media     []historyMedia
	Buckets   [][]int // [media][bucket]
	TimeToDL  []timeToDownload
	ListStats []listBacklog
}

type timeToDownload struct {
	Count       int
	MedianDays  float64
	Percentile9 float64
}

type listBacklog struct {
	List        string
	Type        string
	Items       int
	Missing     int
	Downloads90 int
}

func getBacklogStats() backlogStats {
	b := backlogStats{Media: activeHistoryMedias()}
	since90 := daysAgo(90)
	since365 := daysAgo(365)

	for _, m := range b.Media {
		ages := countsByStr(database.GetrowsN[database.DbstaticOneStringOneInt](false, uint(len(backlogBuckets)),
			`SELECT CASE WHEN d IS NULL THEN 'unknown'
				WHEN d < 30 THEN '< 30 days'
				WHEN d < 180 THEN '30–180 days'
				WHEN d < 365 THEN '180 days – 1 year'
				ELSE '> 1 year' END, COUNT(*)
			FROM (SELECT julianday('now') - julianday(i.created_at) AS d FROM `+m.from+` WHERE i.missing = 1)
			GROUP BY 1`))

		row := make([]int, len(backlogBuckets))
		for i, bucket := range backlogBuckets {
			row[i] = ages[bucket]
		}

		b.Buckets = append(b.Buckets, row)

		// Items re-added after an earlier download give negative spans; skip them.
		days := slices.DeleteFunc(database.GetrowsN[float64](false, 0,
			"SELECT julianday(h.d) - julianday(i.created_at) FROM "+m.from+
				" JOIN (SELECT "+m.histKey+" AS k, MIN(downloaded_at) AS d FROM "+m.hist+
				" GROUP BY k) h ON h.k = i.id WHERE i.created_at >= ?", since365),
			func(d float64) bool { return d < 0 })
		slices.Sort(days)

		b.TimeToDL = append(b.TimeToDL, timeToDownload{
			Count:       len(days),
			MedianDays:  quantile(days, 0.5),
			Percentile9: quantile(days, 0.9),
		})

		downloads := countsByStr(database.GetrowsN[database.DbstaticOneStringOneInt](false, 0,
			"SELECT "+m.listCol+", COUNT(*) FROM "+m.hist+" h, "+m.from+
				" WHERE i.id = h."+m.histKey+" AND h.downloaded_at >= ? GROUP BY 1", since90))

		for _, r := range database.GetrowsN[syncops.DbstaticOneStringTwoInt](false, 0,
			"SELECT "+m.listCol+", COUNT(*), SUM(i.missing) FROM "+m.from+" GROUP BY 1") {
			b.ListStats = append(b.ListStats, listBacklog{
				List:        r.Str,
				Type:        m.label,
				Items:       int(r.Num1),
				Missing:     int(r.Num2),
				Downloads90: downloads[r.Str],
			})
		}
	}

	slices.SortFunc(b.ListStats, func(x, y listBacklog) int { return y.Missing - x.Missing })

	return b
}

// quantile returns the q-quantile of sorted values (nearest rank).
func quantile(sorted []float64, q float64) float64 {
	if len(sorted) == 0 {
		return 0
	}

	return sorted[int(math.Ceil(q*float64(len(sorted))))-1]
}

func renderBacklog(b backlogStats) gomponents.Node {
	ageHead := []gomponents.Node{
		html.Th(gomponents.Text("Type")),
		html.Th(html.Class("text-end"), gomponents.Text("Missing")),
	}
	for _, bucket := range backlogBuckets {
		ageHead = append(ageHead, html.Th(html.Class("text-end"), gomponents.Text(bucket)))
	}

	ageHead = append(ageHead,
		html.Th(
			html.Class("text-end"),
			html.Title("For items added in the last 365 days: days from adding to the first download"),
			gomponents.Text("Time to first download (median / 90%)"),
		))

	ageRows := make([]gomponents.Node, 0, len(b.Media))
	for i, m := range b.Media {
		missing := 0
		for _, n := range b.Buckets[i] {
			missing += n
		}

		cells := []gomponents.Node{
			html.Td(html.Strong(gomponents.Text(m.label))),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", missing)),
		}
		for j, n := range b.Buckets[i] {
			cls := "text-end"
			if j == 3 && n > 0 {
				cls += " text-danger fw-semibold"
			}

			cells = append(cells, html.Td(html.Class(cls), gomponents.Textf("%d", n)))
		}

		ttd := "-"
		if t := b.TimeToDL[i]; t.Count > 0 {
			ttd = fmt.Sprintf("%.1f d / %.1f d (%d items)", t.MedianDays, t.Percentile9, t.Count)
		}

		cells = append(cells, html.Td(html.Class("text-end"), gomponents.Text(ttd)))
		ageRows = append(ageRows, html.Tr(cells...))
	}

	listRows := make([]gomponents.Node, 0, len(b.ListStats))
	for _, l := range b.ListStats {
		listRows = append(listRows, html.Tr(
			html.Td(html.Strong(gomponents.Text(l.List))),
			html.Td(gomponents.Text(l.Type)),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", l.Items)),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", l.Missing)),
			html.Td(shareBar(l.Missing, l.Items, "bg-warning")),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", l.Downloads90)),
		))
	}

	if len(listRows) == 0 {
		listRows = append(listRows, emptyRow(6, "No lists"))
	}

	return statSection("bg-warning text-dark", "fa-solid fa-hourglass-half", "Wanted Backlog",
		html.H6(html.Class("text-muted"), gomponents.Text("Missing items by time on the wanted list")),
		statTable(ageHead, ageRows),
		html.H6(html.Class("text-muted mt-4"), gomponents.Text("Per list")),
		html.Div(
			html.Style("max-height: 420px; overflow-y: auto;"),
			statTable([]gomponents.Node{
				html.Th(gomponents.Text("List")),
				html.Th(gomponents.Text("Type")),
				html.Th(html.Class("text-end"), gomponents.Text("Items")),
				html.Th(html.Class("text-end"), gomponents.Text("Missing")),
				html.Th(html.Style("width: 20%"), gomponents.Text("Missing %")),
				html.Th(html.Class("text-end"), gomponents.Text("Downloads (90 days)")),
			}, listRows),
		),
	)
}

// --- Job runtimes ------------------------------------------------------------

type jobRuntime struct {
	JobType     string  `db:"job_type"`
	Category    string  `db:"job_category"`
	Group       string  `db:"job_group"`
	Runs        int     `db:"runs"`
	AvgSec      float64 `db:"avg_sec"`
	MaxSec      float64 `db:"max_sec"`
	TotalSec    float64 `db:"total_sec"`
	Unfinished  int     `db:"unfinished"`
	LastStarted string  `db:"last_started"`
}

// defaultJobRange keeps the job runtimes section at its former 30 day window.
const defaultJobRange = "month"

// getJobRuntimes aggregates the job runs within the time frame. Runs whose end
// lies before their start (local-time stamps across a DST switch) get no duration.
func getJobRuntimes(rng activityRange) []jobRuntime {
	return database.StructscanT[jobRuntime](false, 0, `
		SELECT job_type, COALESCE(job_category, '') AS job_category, COALESCE(job_group, '') AS job_group,
			COUNT(*) AS runs, COALESCE(AVG(d), 0) AS avg_sec, COALESCE(MAX(d), 0) AS max_sec,
			COALESCE(SUM(d), 0) AS total_sec, SUM(ended IS NULL) AS unfinished,
			COALESCE(MAX(started), '') AS last_started
		FROM (SELECT job_type, job_category, job_group, started, ended,
				CASE WHEN ended >= started THEN (julianday(ended) - julianday(started)) * 86400 END AS d
			FROM job_histories WHERE created_at >= ?)
		GROUP BY job_type, job_category, job_group
		ORDER BY total_sec DESC`, rng.start(time.Now()).Format("2006-01-02 15:04:05"))
}

func renderJobRuntimes(rng activityRange, jobs []jobRuntime) gomponents.Node {
	busiest := 0.0
	if len(jobs) > 0 {
		busiest = jobs[0].TotalSec
	}

	rows := make([]gomponents.Node, 0, len(jobs))
	for _, j := range jobs {
		unfinished := gomponents.Node(gomponents.Text("0"))
		if j.Unfinished > 0 {
			unfinished = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Textf("%d", j.Unfinished))
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(j.JobType))),
			html.Td(gomponents.Text(j.Category)),
			html.Td(gomponents.Text(j.Group)),
			html.Td(html.Class("text-end"), gomponents.Textf("%d", j.Runs)),
			html.Td(html.Class("text-end"), gomponents.Text(formatDuration(int64(j.AvgSec)))),
			html.Td(html.Class("text-end"), gomponents.Text(formatDuration(int64(j.MaxSec)))),
			html.Td(html.Class("text-end"), gomponents.Text(formatDuration(int64(j.TotalSec)))),
			html.Td(shareBar(int(j.TotalSec), int(busiest), "bg-info")),
			html.Td(html.Class("text-center"), unfinished),
			html.Td(html.Class("text-nowrap"), html.Small(gomponents.Text(j.LastStarted))),
		))
	}

	if len(rows) == 0 {
		rows = append(rows, emptyRow(10, "No job runs in this time frame"))
	}

	return html.Div(html.ID("stat-jobs"), statSection("bg-info text-white", "fa-solid fa-stopwatch",
		"Job Runtimes ("+strings.ToLower(rng.label)+")",
		activityRangeButtons(rng.key, "/api/admin/statistics/jobs", "stat-jobs"),
		html.P(
			html.Class("text-muted small"),
			gomponents.Text("Sorted by total busy time. \"Not finished\" includes jobs that are running right now."),
		),
		html.Div(
			html.Style("max-height: 520px; overflow-y: auto;"),
			statTable([]gomponents.Node{
				html.Th(gomponents.Text("Job")),
				html.Th(gomponents.Text("Category")),
				html.Th(gomponents.Text("Group")),
				html.Th(html.Class("text-end"), gomponents.Text("Runs")),
				html.Th(html.Class("text-end"), gomponents.Text("Avg")),
				html.Th(html.Class("text-end"), gomponents.Text("Max")),
				html.Th(html.Class("text-end"), gomponents.Text("Total")),
				html.Th(html.Style("width: 15%"), gomponents.Text("vs. busiest job")),
				html.Th(html.Class("text-center"), gomponents.Text("Not finished")),
				html.Th(gomponents.Text("Last start")),
			}, rows),
		),
	))
}

// --- Library quality ---------------------------------------------------------

type distribution struct {
	Title string
	Rows  []database.DbstaticOneStringOneInt
}

type belowCutoff struct {
	Label string
	Count int
	Items []database.DbstaticThreeString // title, resolution, list
}

type libraryQuality struct {
	Distributions []distribution
	BelowCutoff   []belowCutoff
}

func getLibraryQuality() libraryQuality {
	var q libraryQuality

	dist := func(title, query string) {
		rows := database.GetrowsN[database.DbstaticOneStringOneInt](false, 0, query)
		if len(rows) > 0 {
			q.Distributions = append(q.Distributions, distribution{title, rows})
		}
	}

	for _, v := range []struct{ label, files string }{
		{"Movies", "movie_files"},
		{"Episodes", "serie_episode_files"},
	} {
		for _, col := range []struct{ name, id string }{{"resolution", "resolution_id"}, {"codec", "codec_id"}} {
			dist(v.label+" by "+col.name, "SELECT COALESCE(q.name, 'unknown'), COUNT(*) FROM "+v.files+
				" f LEFT JOIN qualities q ON q.id = f."+col.id+" GROUP BY 1 ORDER BY 2 DESC")
		}
	}

	if mediaTypeConfigured("music") {
		dist("Music files by format", "SELECT COALESCE(NULLIF(format, ''), 'unknown'), COUNT(*) FROM album_files GROUP BY 1 ORDER BY 2 DESC")
	}

	if mediaTypeConfigured("audiobook") {
		dist("Audiobook files by format", "SELECT COALESCE(NULLIF(format, ''), 'unknown'), COUNT(*) FROM audiobook_files GROUP BY 1 ORDER BY 2 DESC")
	}

	for _, m := range activeHistoryMedias() {
		where := " WHERE i.missing = 0 AND i.quality_reached = 0 AND i.dont_upgrade = 0"

		res := "''"
		if m.resExpr != "" {
			res = "COALESCE(" + m.resExpr + ", '')"
		}

		q.BelowCutoff = append(q.BelowCutoff, belowCutoff{
			Label: m.label,
			Count: database.Getdatarow[int](false, "SELECT COUNT(*) FROM "+m.from+where),
			Items: database.GetrowsN[database.DbstaticThreeString](false, 10,
				"SELECT "+m.titleExpr+", "+res+", "+m.listCol+" FROM "+m.from+" "+m.titleFrom+where+
					" ORDER BY i.updated_at DESC LIMIT 10"),
		})
	}

	return q
}

func renderLibraryQuality(q libraryQuality) gomponents.Node {
	distCols := make([]gomponents.Node, 0, len(q.Distributions))
	for _, d := range q.Distributions {
		total := 0
		for _, r := range d.Rows {
			total += r.Num
		}

		rows := make([]gomponents.Node, 0, len(d.Rows))
		for _, r := range d.Rows {
			rows = append(rows, html.Tr(
				html.Td(gomponents.Text(r.Str)),
				html.Td(html.Class("text-end"), gomponents.Textf("%d", r.Num)),
				html.Td(html.Style("width: 45%"), shareBar(r.Num, total, "bg-success")),
			))
		}

		distCols = append(distCols, html.Div(
			html.Class("col-lg-6 col-xl-4 mb-3"),
			html.H6(html.Class("text-muted"), gomponents.Text(d.Title)),
			html.Div(html.Style("max-height: 260px; overflow-y: auto;"), statTable(nil, rows)),
		))
	}

	cutoffCols := make([]gomponents.Node, 0, len(q.BelowCutoff))
	for _, b := range q.BelowCutoff {
		rows := make([]gomponents.Node, 0, len(b.Items))
		for _, it := range b.Items {
			res := gomponents.Node(nil)
			if it.Str2 != "" {
				res = html.Span(html.Class("badge bg-secondary ms-1"), gomponents.Text(it.Str2))
			}

			rows = append(rows, html.Tr(
				html.Td(gomponents.Text(truncateString(it.Str1, 60)), res),
				html.Td(html.Small(html.Class("text-muted"), gomponents.Text(it.Str3))),
			))
		}

		if len(rows) == 0 {
			rows = append(rows, emptyRow(2, "Everything has reached its cutoff"))
		}

		cutoffCols = append(cutoffCols, html.Div(
			html.Class("col-lg-6 mb-3"),
			html.H6(
				html.Class("text-muted"),
				gomponents.Textf("%s below cutoff: ", b.Label),
				html.Span(html.Class("badge bg-warning text-dark"), gomponents.Textf("%d", b.Count)),
				html.Small(gomponents.Text(" (most recently updated)")),
			),
			statTable(nil, rows),
		))
	}

	return statSection("bg-success text-white", "fa-solid fa-gem", "Library Quality",
		html.Div(append([]gomponents.Node{html.Class("row")}, distCols...)...),
		html.H6(html.Class("text-muted mt-2"), gomponents.Text("Upgrade candidates (have a file, quality cutoff not reached, upgrades allowed)")),
		html.Div(append([]gomponents.Node{html.Class("row")}, cutoffCols...)...),
	)
}

// --- Shared rendering helpers ------------------------------------------------

func statSection(headerClass, icon, title string, body ...gomponents.Node) gomponents.Node {
	return html.Div(
		html.Div(
			html.Class("card-header "+headerClass),
			html.H5(html.Class("mb-0"), html.I(html.Class(icon+" me-2")), gomponents.Text(title)),
		),
		html.Div(append([]gomponents.Node{html.Class("card-body")}, body...)...),
	)
}

func statTable(head, rows []gomponents.Node) gomponents.Node {
	return html.Table(
		html.Class("table table-sm table-hover mb-0"),
		gomponents.If(len(head) > 0, html.THead(html.Tr(head...))),
		html.TBody(rows...),
	)
}

func emptyRow(cols int, text string) gomponents.Node {
	return html.Tr(html.Td(
		html.ColSpan(fmt.Sprint(cols)),
		html.Class("text-center text-muted"),
		gomponents.Text(text),
	))
}

func percent(n, total int) string {
	if total == 0 {
		return "-"
	}

	return fmt.Sprintf("%.1f%%", float64(n)*100/float64(total))
}

// shareBar renders n/total as a labelled progress bar.
func shareBar(n, total int, barClass string) gomponents.Node {
	pct := 0.0
	if total > 0 {
		pct = float64(n) * 100 / float64(total)
	}

	return html.Div(
		html.Class("d-flex align-items-center gap-2"),
		html.Div(
			html.Class("progress flex-grow-1"),
			html.Style("height: 8px;"),
			html.Div(
				html.Class("progress-bar "+barClass),
				html.Style(fmt.Sprintf("width: %.1f%%", pct)),
			),
		),
		html.Small(html.Class("text-muted text-nowrap"), gomponents.Textf("%.0f%%", pct)),
	)
}

// --- Handlers ----------------------------------------------------------------

func renderStatHTML(ctx *gin.Context, node gomponents.Node) {
	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = node.Render(ctx.Writer)
}

func webStatisticsActivity(ctx *gin.Context) {
	renderStatHTML(ctx, renderDownloadActivity(getDownloadActivity(findActivityRange(ctx.Query("range")))))
}

func webStatisticsBacklog(ctx *gin.Context) {
	renderStatHTML(ctx, renderBacklog(getBacklogStats()))
}

func webStatisticsJobs(ctx *gin.Context) {
	rng := findActivityRange(ctx.DefaultQuery("range", defaultJobRange))
	renderStatHTML(ctx, renderJobRuntimes(rng, getJobRuntimes(rng)))
}

func webStatisticsQuality(ctx *gin.Context) {
	renderStatHTML(ctx, renderLibraryQuality(getLibraryQuality()))
}

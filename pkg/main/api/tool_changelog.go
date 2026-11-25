package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// The changelog page lists the commits on GitHub and marks which of them
// this build holds: by its commit hash, or, when it was built without one
// (the production folder is no git checkout), by its build date.

// Build information, set by main from the linker flags.
var (
	buildVersion string
	buildHash    string
	buildDate    string
)

// SetBuildInfo stores the version, commit and date the binary was built
// with.
func SetBuildInfo(version, hash, date string) {
	buildVersion, buildHash, buildDate = version, hash, date
}

const changelogRepo = "Kellerman81/go_media_downloader"

// changelogURL is GitHub's commit list of the repository.
var changelogURL = "https://api.github.com/repos/" + changelogRepo + "/commits"

const (
	changelogPerPage = 100
	changelogTTL     = 30 * time.Minute
)

// ghCommit is the part of a GitHub commit the page shows.
type ghCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
		Committer struct {
			Date time.Time `json:"date"`
		} `json:"committer"`
	} `json:"commit"`
}

type changelogPage struct {
	commits []ghCommit
	at      time.Time
}

// Cached pages: GitHub allows 60 calls an hour without a login.
var (
	changelogMu    sync.Mutex
	changelogCache = map[int]changelogPage{}
)

// fetchChangelog loads a page of commits (newest first), from the cache
// while it is fresh.
func fetchChangelog(ctx context.Context, page int) ([]ghCommit, error) {
	changelogMu.Lock()
	cached, ok := changelogCache[page]
	changelogMu.Unlock()

	if ok && time.Since(cached.at) < changelogTTL {
		return cached.commits, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		changelogURL+"?per_page="+strconv.Itoa(changelogPerPage)+"&page="+strconv.Itoa(page), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "go-media-downloader")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if ok {
			return cached.commits, nil
		}

		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if ok {
			return cached.commits, nil
		}

		return nil, errors.New("GitHub: " + resp.Status)
	}

	var commits []ghCommit
	if err := json.NewDecoder(resp.Body).Decode(&commits); err != nil {
		return nil, err
	}

	changelogMu.Lock()
	changelogCache[page] = changelogPage{commits: commits, at: time.Now()}
	changelogMu.Unlock()

	return commits, nil
}

// knownHash reports whether the build carries a commit hash.
func knownHash(h string) bool {
	h = strings.TrimSpace(h)
	return len(h) >= 7 && h != "unknown"
}

// inBuild reports for each commit (newest first) whether this build holds
// it, and whether that was decided by the hash (else by the build date;
// nil when neither is known). before is true when the build's commit is on
// an earlier page: everything here is newer.
func inBuild(commits []ghCommit, hash, date string, before bool) ([]bool, string) {
	in := make([]bool, len(commits))

	if knownHash(hash) {
		found := before
		for i := range commits {
			if !found && strings.HasPrefix(commits[i].SHA, strings.TrimSpace(hash)) {
				found = true
			}

			in[i] = found
		}

		return in, "hash"
	}

	day, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(date), time.Local)
	if err != nil {
		return nil, ""
	}

	end := day.AddDate(0, 0, 1)
	for i := range commits {
		in[i] = commits[i].Commit.Committer.Date.Before(end)
	}

	return in, "date"
}

// buildPage finds the page of the build's commit (0 not among the first
// pages): the pages before it are all newer.
func buildPage(ctx context.Context, upTo int) int {
	if !knownHash(buildHash) {
		return 0
	}

	for p := 1; p <= upTo; p++ {
		commits, err := fetchChangelog(ctx, p)
		if err != nil {
			return 0
		}

		for i := range commits {
			if strings.HasPrefix(commits[i].SHA, strings.TrimSpace(buildHash)) {
				return p
			}
		}

		if len(commits) < changelogPerPage {
			return 0
		}
	}

	return 0
}

func adminPageChangelog(ctx *gin.Context) {
	page, _ := strconv.Atoi(ctx.Query("page"))
	page = max(page, 1)

	commits, err := fetchChangelog(ctx.Request.Context(), page)

	// Pages before the build's page are all newer than the build.
	bp := 0
	if err == nil && knownHash(buildHash) {
		bp = buildPage(ctx.Request.Context(), max(page, 5))
	}

	renderPageIn(ctx, "Changelog", false, renderChangelogPage(commits, err, page, bp))
}

func renderChangelogPage(commits []ghCommit, err error, page, buildPg int) gomponents.Node {
	in, by := inBuild(commits, buildHash, buildDate, buildPg > 0 && buildPg < page)
	if by == "hash" && buildPg == 0 {
		// The build's commit isn't on GitHub (yet), or very old: no claim
		// per commit.
		in, by = nil, "missing"
	}

	return html.Div(html.Class("container-fluid"),
		html.Div(html.Class("page-header-enhanced"),
			html.Div(html.Class("header-content"),
				html.Div(html.Class("header-icon-wrapper"), html.I(html.Class("fa-solid fa-code-commit header-icon"))),
				html.Div(html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Changelog")),
					html.P(html.Class("header-subtitle"),
						gomponents.Text("The commits on GitHub, and which of them this build contains.")),
				),
			),
		),
		renderBuildCard(commits, in, by, page),
		gomponents.If(err != nil, html.Div(html.Class("alert alert-warning"),
			html.I(html.Class("fa-solid fa-triangle-exclamation me-2")),
			gomponents.Textf("Can't load the commits from GitHub: %v", err))),
		gomponents.If(err == nil && len(commits) == 0, html.Div(html.Class("alert alert-info"), gomponents.Text("No commits on this page."))),
		renderCommitList(commits, in, map[bool]int{true: changelogFirstDays}[page == 1], renderChangelogPager(page, len(commits))),
	)
}

// renderBuildCard shows the build and how many newer commits there are.
func renderBuildCard(commits []ghCommit, in []bool, by string, page int) gomponents.Node {
	hash := strings.TrimSpace(buildHash)

	hashNode := gomponents.Node(gomponents.Text("not recorded"))
	if knownHash(hash) {
		hashNode = html.A(html.Href("https://github.com/"+changelogRepo+"/commit/"+hash), html.Target("_blank"), html.Rel("noopener"),
			html.Code(gomponents.Text(hash)))
	}

	newer := 0
	for i := range in {
		if !in[i] {
			newer++
		}
	}

	var note gomponents.Node

	switch {
	case len(commits) == 0:
	case by == "date":
		note = gomponents.Textf("Built without a commit hash: commits up to the build date count as included. %d on this page are newer.", newer)
	case by == "missing":
		note = gomponents.Text("This build's commit isn't among the recent commits on GitHub (not pushed yet, or older).")
	case by == "" && page == 1:
		note = gomponents.Text("Neither a commit hash nor a build date was recorded for this build.")
	case newer > 0:
		note = gomponents.Textf("%d newer commits on this page are not in this build.", newer)
	case by == "hash":
		note = gomponents.Text("All commits on this page are in this build.")
	}

	row := func(label string, v gomponents.Node) gomponents.Node {
		return html.Tr(html.Th(html.Class("text-muted fw-normal pe-4"), gomponents.Text(label)), html.Td(v))
	}

	orDash := func(s string) string {
		if strings.TrimSpace(s) == "" {
			return "–"
		}

		return s
	}

	return html.Div(html.Class("card mb-3"),
		html.Div(html.Class("card-header"), html.H5(html.Class("card-title mb-0"), gomponents.Text("This build"))),
		html.Div(html.Class("card-body"),
			html.Table(html.Class("table table-sm table-borderless mb-2 w-auto"),
				html.TBody(
					row("Version", gomponents.Text(orDash(buildVersion))),
					row("Commit", hashNode),
					row("Build date", gomponents.Text(orDash(buildDate))),
				)),
			gomponents.If(note != nil, html.P(html.Class("mb-0 small text-muted"), note)),
		),
	)
}

// changelogFirstDays are the days the first page shows before a button
// shows the older ones.
const changelogFirstDays = 5

// renderCommitList shows the commits grouped by day, then the pager. With
// days > 0 only the newest days show; a button shows the rest and the pager.
func renderCommitList(commits []ghCommit, in []bool, days int, pager gomponents.Node) gomponents.Node {
	if len(commits) == 0 {
		return pager
	}

	var (
		groups []gomponents.Node
		sizes  []int // commits per day
		items  []gomponents.Node
		day    string
	)

	flush := func() {
		if len(items) > 0 {
			groups = append(groups, html.Div(html.Class("card mb-3"),
				html.Div(html.Class("card-header py-2"), html.Strong(gomponents.Text(day))),
				html.Ul(html.Class("list-group list-group-flush"), gomponents.Group(items))))
			sizes = append(sizes, len(items))
			items = nil
		}
	}

	for i := range commits {
		c := &commits[i]

		d := c.Commit.Committer.Date.Local().Format("2006-01-02")
		if d != day {
			flush()

			day = d
		}

		title, body, _ := strings.Cut(strings.TrimSpace(c.Commit.Message), "\n")
		body = strings.TrimSpace(body)

		included := in == nil || in[i]
		short := c.SHA
		if len(short) > 7 {
			short = short[:7]
		}

		isBuild := knownHash(buildHash) && strings.HasPrefix(c.SHA, strings.TrimSpace(buildHash))

		items = append(items, html.Li(html.Class("list-group-item"+map[bool]string{false: " bg-body-tertiary"}[included]),
			html.Div(html.Class("d-flex flex-wrap align-items-baseline gap-2"),
				html.A(html.Href(c.HTMLURL), html.Target("_blank"), html.Rel("noopener"), html.Class("font-monospace small"), gomponents.Text(short)),
				html.Span(html.Class("fw-semibold"+map[bool]string{false: " text-muted"}[included]), gomponents.Text(title)),
				gomponents.If(isBuild, html.Span(html.Class("badge bg-success"), gomponents.Text("This build"))),
				gomponents.If(in != nil && !included, html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("Not in this build"))),
				html.Span(html.Class("ms-auto small text-muted"),
					gomponents.Text(c.Commit.Author.Name+" · "+c.Commit.Committer.Date.Local().Format("15:04"))),
			),
			gomponents.If(body != "", html.Details(html.Class("mt-1"),
				html.Summary(html.Class("small text-muted"), gomponents.Text("Details")),
				html.Pre(html.Class("small mb-0 mt-1"), html.Style("white-space:pre-wrap"), gomponents.Text(body)))),
		))
	}

	flush()

	if days <= 0 || len(groups) <= days {
		return gomponents.Group(append(groups, pager))
	}

	hidden := 0
	for _, n := range sizes[days:] {
		hidden += n
	}

	return gomponents.Group([]gomponents.Node{
		gomponents.Group(groups[:days]),
		html.Button(html.Type("button"), html.Class("btn btn-outline-secondary mb-3"),
			gomponents.Attr("onclick", "this.nextElementSibling.classList.remove('d-none');this.remove()"),
			html.I(html.Class("fa-solid fa-chevron-down me-1")), gomponents.Textf("Show older commits (%d more)", hidden)),
		html.Div(html.Class("d-none"), gomponents.Group(groups[days:]), pager),
	})
}

// renderChangelogPager links to newer and older pages.
func renderChangelogPager(page, count int) gomponents.Node {
	if page <= 1 && count < changelogPerPage {
		return nil
	}

	return html.Nav(html.Class("d-flex justify-content-between mb-3"),
		gomponents.If(page > 1, html.A(html.Class("btn btn-outline-secondary"), html.Href("/api/admin/changelog?page="+strconv.Itoa(page-1)),
			html.I(html.Class("fa-solid fa-chevron-left me-1")), gomponents.Text("Newer"))),
		html.Span(),
		gomponents.If(count >= changelogPerPage, html.A(html.Class("btn btn-outline-secondary"), html.Href("/api/admin/changelog?page="+strconv.Itoa(page+1)),
			gomponents.Text("Older"), html.I(html.Class("fa-solid fa-chevron-right ms-1")))),
	)
}

// buildSummary is the build in one line: version, short commit, date.
func buildSummary() string {
	parts := make([]string, 0, 3)

	if v := strings.TrimSpace(buildVersion); v != "" {
		parts = append(parts, v)
	}

	if h := strings.TrimSpace(buildHash); knownHash(h) {
		parts = append(parts, h[:min(len(h), 7)])
	}

	if d := strings.TrimSpace(buildDate); d != "" {
		parts = append(parts, "built "+d)
	}

	if len(parts) == 0 {
		return "development build"
	}

	return strings.Join(parts, " · ")
}

// renderBuildFooter shows the build in the footer; for admins it links to
// the changelog.
func renderBuildFooter(admin bool) gomponents.Node {
	text := html.Small(html.Class("text-muted"), html.Title("Version · commit · build date"), gomponents.Text(buildSummary()))
	if !admin {
		return text
	}

	return html.A(html.Href("/api/admin/changelog"), html.Class("text-decoration-none"), html.Title("Changelog"), text)
}

package api

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// mtKind is one flavour of the match tester page.
type mtKind struct {
	Key        string
	Title      string
	Subtitle   string
	Icon       string
	ConfigNoun string
	FolderNoun string
	FileNoun   string
}

var mtKinds = []*mtKind{
	{
		Key:        "album",
		Title:      "Album Match Tester",
		Subtitle:   "Run the album matcher read-only over music or audiobook folders, compare every candidate from the database and the metadata providers, and organize the folders you pick with the candidate you pick. Nothing is written until you process an entry.",
		Icon:       "fa-compact-disc",
		ConfigNoun: "music or audiobook",
		FolderNoun: "album folders",
		FileNoun:   "audio",
	},
	{
		Key:        "movie",
		Title:      "Movie Match Tester",
		Subtitle:   "Run the movie lookup read-only over import folders, compare what the structure job picks with every candidate from the database, IMDb, TMDB and OMDb, see the rename and the quality decision, and organize the files you pick with the movie you pick. Nothing is written until you process an entry.",
		Icon:       "fa-film",
		ConfigNoun: "movie",
		FolderNoun: "folders",
		FileNoun:   "movie",
	},
	{
		Key:        "serie",
		Title:      "Series Match Tester",
		Subtitle:   "Run the series and episode lookup read-only over import folders - one entry per episode file - compare what the structure job picks with every candidate from the database, TVDB and Trakt, see the rename and the quality decision, and organize the files you pick with the series you pick. Nothing is written until you process an entry.",
		Icon:       "fa-tv",
		ConfigNoun: "series",
		FolderNoun: "folders",
		FileNoun:   "episode",
	},
	{
		Key:        "book",
		Title:      "Book Match Tester",
		Subtitle:   "Run the book lookup read-only over import folders, compare what the structure job picks with every candidate from the database and OpenLibrary, see the rename, and organize the files you pick with the book you pick. Nothing is written until you process an entry.",
		Icon:       "fa-book",
		ConfigNoun: "book",
		FolderNoun: "folders",
		FileNoun:   "book",
	},
}

func (k *mtKind) album() bool {
	return k.Key == "album"
}

// mtKindByKey returns the kind with key; the album tester by default.
func mtKindByKey(key string) *mtKind {
	for _, k := range mtKinds {
		if k.Key == key {
			return k
		}
	}

	return mtKinds[0]
}

// mtKindOf returns the kind testing media type isType, or nil.
func mtKindOf(isType uint) *mtKind {
	switch isType {
	case config.MediaTypeMusic, config.MediaTypeAudiobook:
		return mtKinds[0]
	case config.MediaTypeMovie:
		return mtKinds[1]
	case config.MediaTypeSeries:
		return mtKinds[2]
	case config.MediaTypeBook:
		return mtKinds[3]
	}

	return nil
}

// configs returns the names of the media configs the kind tests.
func (k *mtKind) configs() []string {
	media := config.GetSettingsMediaAll()
	if media == nil {
		return nil
	}

	var lists [][]config.MediaTypeConfig

	switch k.Key {
	case "album":
		lists = [][]config.MediaTypeConfig{media.Music, media.AudioBooks}
	case "movie":
		lists = [][]config.MediaTypeConfig{media.Movies}
	case "serie":
		lists = [][]config.MediaTypeConfig{media.Series}
	case "book":
		lists = [][]config.MediaTypeConfig{media.Books}
	}

	var out []string

	for _, l := range lists {
		for i := range l {
			out = append(out, l[i].NamePrefix)
		}
	}

	return out
}

func (k *mtKind) columns() int {
	if k.album() {
		return 9
	}

	return 7
}

func (k *mtKind) tableHead() gomponents.Node {
	heads := []string{"", "Folder / file", "Have album", "Wanted album", "Have artist", "Wanted artist", "Tracks", "Score", "Status"}
	if !k.album() {
		heads = []string{"", "File", "Release", "Have", "Wanted", "Score", "Status"}
	}

	cells := make([]gomponents.Node, 0, len(heads))
	for _, h := range heads {
		cells = append(cells, html.Th(gomponents.Text(h)))
	}

	return html.Tr(cells...)
}

// ── Entries ──────────────────────────────────────────────────────────────────

// mediaCandidate returns the chosen candidate of a media entry, or its best.
func (e *mtEntry) mediaCandidate() *importfeed.MediaCandidate {
	if e.Media == nil {
		return nil
	}

	if e.Chosen != "" {
		if c := e.Media.Set.Candidate(e.Chosen); c != nil {
			return c
		}
	}

	return e.Media.Set.Best()
}

// bestDist returns the distance of the entry's best candidate, +Inf without.
func (e *mtEntry) bestDist() float64 {
	switch {
	case e.Media != nil:
		if b := e.Media.Set.Best(); b != nil {
			return b.Dist
		}
	case e.Entry != nil:
		if b := e.Entry.Best(); b != nil {
			return b.Dist
		}
	}

	return math.Inf(1)
}

// testMediaFolder previews every media file of folder.
func (s *mtSession) testMediaFolder(folder string) []*mtEntry {
	cfgp, data, _ := s.mtConfig()
	cache := importfeed.NewMediaSearchCache()

	var out []*mtEntry

	for _, f := range structure.CollectMediaFiles(s.ctx, folder, cfgp, data.CfgPath) {
		if s.ctx.Err() != nil {
			break
		}

		out = append(out, &mtEntry{Media: structure.PreviewMediaFile(s.ctx, f, folder, cfgp, data, cache)})
	}

	return out
}

// handleMediaEntry shows a media entry's candidates, the field comparison of
// the chosen one and what organizing with it would do.
func (s *mtSession) handleMediaEntry(ctx *gin.Context, id int) {
	s.mu.Lock()
	e, ok := s.entries[id]

	if ok && e.Media != nil && ctx.Query("cand") != "" && e.Media.Set.Candidate(ctx.Query("cand")) != nil {
		e.Chosen = ctx.Query("cand")
	}

	var (
		c             *importfeed.MediaCandidate
		mediaID, dbID uint
	)

	if ok && e.Media != nil {
		c = e.mediaCandidate()
		if c != nil {
			mediaID, dbID = c.MediaID, c.DBID
		}
	}
	s.mu.Unlock()

	if !ok || e.Media == nil {
		mtRender(ctx, mtAlert("warning", "Entry not found."))
		return
	}

	cfgp, data, dataCfg := s.mtConfig()

	var (
		plan    *structure.MediaFilePlan
		planErr string
	)

	switch {
	case c == nil || cfgp == nil:
	case mediaID == 0 && dbID == 0:
		planErr = "Not in the database yet - the target path is computed once it is imported."
	default:
		var err error

		plan, err = structure.PlanMediaFile(ctx, e.Media.File, e.Media.Folder, cfgp, data,
			mtDefaultTemplate(cfgp), mediaID, dbID, mtImportList(cfgp, data))
		if err != nil {
			planErr = err.Error()
		}
	}

	strong, medium := importfeed.RecThresholds(dataCfg)

	mtRender(ctx,
		renderMTMediaDetail(s, e, c, plan, planErr, strong, medium),
		renderMTMediaSummaryCells(e, c, strong, medium, true),
	)
}

// mtDispatchMedia organizes one media file on the Data queue: the chosen
// candidate's list entry is created when needed, then the file is organized
// with it.
func mtDispatchMedia(
	s *mtSession,
	e *mtEntry,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	setStatus func(status, msg string),
) {
	err := worker.Dispatch("matchtester_"+cfgp.NamePrefix+"_"+e.Media.File, func(_ uint32, ctx context.Context) error {
		setStatus(mtRunning, "")

		s.mu.Lock()
		c := e.mediaCandidate()
		s.mu.Unlock()

		imp, err := importfeed.ImportMediaCandidate(ctx, c, cfgp, mtImportList(cfgp, data))
		if err != nil {
			setStatus(mtFailed, "creating the list entry: "+err.Error())
			return err
		}

		// A retry after a failed organize must not import again.
		s.mu.Lock()
		c.MediaID, c.DBID, c.ListName = imp.MediaID, imp.DBID, imp.ListName
		s.mu.Unlock()

		err = structure.OrganizeMediaFile(ctx, e.Media.File, e.Media.Folder, cfgp, data,
			mtDefaultTemplate(cfgp), imp.MediaID)
		if err != nil {
			setStatus(mtFailed, err.Error())
			return err
		}

		setStatus(mtDone, "")

		return nil
	}, worker.QueueData)
	if err != nil {
		setStatus(mtFailed, "not queued: "+err.Error())
	}
}

// ── Rendering ────────────────────────────────────────────────────────────────

func mtTitleYear(title string, year int) string {
	suffix := " (" + strconv.Itoa(year) + ")"
	if year == 0 || strings.HasSuffix(title, suffix) {
		return title
	}

	return title + suffix
}

// mtMediaHaveInfo is the parsed id, episode or author shown under the title.
func mtMediaHaveInfo(q *importfeed.MediaQuery) string {
	switch q.IsType {
	case config.MediaTypeSeries:
		return q.Identifier
	case config.MediaTypeMovie:
		return q.Imdb
	case config.MediaTypeBook:
		return strings.TrimSpace(q.Author + mtIf(q.ISBN != "", " ISBN "+q.ISBN))
	}

	return ""
}

// mtMediaSource says where a candidate lives: a list entry, a database row
// outside the config's lists, or only at a provider.
func mtMediaSource(c *importfeed.MediaCandidate) string {
	switch {
	case c.DBID == 0:
		return c.Source + " (new)"
	case c.MediaID == 0:
		return "database #" + strconv.FormatUint(uint64(c.DBID), 10) + " (not in a list)"
	default:
		return "list " + c.ListName
	}
}

func mtMatcherBadge(c *importfeed.MediaCandidate) gomponents.Node {
	return gomponents.If(c.Matcher, html.Span(
		html.Class("badge bg-info text-dark ms-1"),
		html.Title("the structure job identifies the file as this entry"),
		gomponents.Text("matcher"),
	))
}

func renderMTMediaEntryRow(s *mtSession, e *mtEntry, strong, medium float64) gomponents.Node {
	p := e.Media
	c := e.mediaCandidate()

	fileCell := html.Td(
		html.Title(p.File),
		mtDetailLink(s, e, filepath.Base(p.File)),
		html.Div(html.Class("small text-muted"), gomponents.Text(filepath.Base(p.Folder))),
		gomponents.If(p.Unmatched, html.Span(
			html.Class("badge bg-secondary"),
			html.Title("on the unmatched list: the structure job skips this file until the list entry expires"),
			gomponents.Text("unmatched list"),
		)),
		gomponents.If(p.Err != "" && c == nil, html.Div(html.Class("small text-danger"), gomponents.Text(p.Err))),
	)

	return html.Tr(
		html.ID("mt-row-"+strconv.Itoa(e.ID)),
		mtSelectCell(s, e, c != nil),
		fileCell,
		html.Td(html.Class("small"), gomponents.Text(p.Release)),
		html.Td(
			gomponents.Text(mtTitleYear(p.Query.Title, p.Query.Year)),
			html.Div(html.Class("small text-muted"), gomponents.Text(mtMediaHaveInfo(&p.Query))),
		),
		renderMTMediaSummaryCells(e, c, strong, medium, false),
		renderMTStatus(s, e, false),
	)
}

// renderMTMediaSummaryCells renders the wanted and score cells of a media
// entry row; oob swaps them in after a candidate change.
func renderMTMediaSummaryCells(e *mtEntry, c *importfeed.MediaCandidate, strong, medium float64, oob bool) gomponents.Node {
	idStr := strconv.Itoa(e.ID)

	cell := func(suffix string, children ...gomponents.Node) gomponents.Node {
		return html.Td(
			html.ID("mt-"+suffix+"-"+idStr),
			gomponents.If(oob, hx.SwapOOB("true")),
			gomponents.Group(children),
		)
	}

	if c == nil {
		return gomponents.Group([]gomponents.Node{
			cell("wanted", html.Span(html.Class("text-muted"), gomponents.Text("—"))),
			cell("score", html.Span(html.Class("badge bg-secondary"), gomponents.Text("no match"))),
		})
	}

	have := mtTitleYear(e.Media.Query.Title, e.Media.Query.Year)

	return gomponents.Group([]gomponents.Node{
		cell("wanted",
			mtDiffText(mtTitleYear(c.Title, c.Year), have),
			mtMatcherBadge(c),
			html.Div(html.Class("small text-muted"), gomponents.Text(mtMediaSource(c))),
		),
		cell("score", mtScoreBadge(c.Dist, strong, medium)),
	})
}

func renderMTMediaDetail(
	s *mtSession,
	e *mtEntry,
	chosen *importfeed.MediaCandidate,
	plan *structure.MediaFilePlan,
	planErr string,
	strong, medium float64,
) gomponents.Node {
	p := e.Media
	q := &p.Query
	idStr := strconv.Itoa(e.ID)

	matcher := "The structure job identifies this file as the entry marked “matcher”."
	if m := p.Set.Matcher(); m == nil {
		matcher = "The structure job finds no entry for this file."
	}

	if p.MatcherErr != "" {
		matcher += " Its lookup ends with: " + p.MatcherErr + "."
	}

	info := []gomponents.Node{
		html.Strong(gomponents.Text(p.File)),
		html.Div(html.Class("small text-muted"), gomponents.Text(strings.Join(mtNonEmpty(
			"parsed: "+mtTitleYear(q.Title, q.Year),
			mtMediaHaveInfo(q),
			p.Release,
		), " · "))),
		html.Div(html.Class("small"), gomponents.Text(matcher)),
	}

	if p.Set == nil || len(p.Set.Candidates) == 0 {
		return html.Div(
			html.Class("p-2 border rounded bg-body-tertiary"),
			gomponents.Group(info),
			mtAlert("warning", "No candidates found ("+p.Err+")."),
		)
	}

	extraHead := map[uint]string{config.MediaTypeSeries: "Episodes", config.MediaTypeBook: "Author"}[q.IsType]

	candRows := make([]gomponents.Node, 0, len(p.Set.Candidates))
	for _, c := range p.Set.Candidates {
		isChosen := chosen != nil && c.Key == chosen.Key

		var extra string

		switch q.IsType {
		case config.MediaTypeSeries:
			found := 0
			for _, ep := range c.Episodes {
				if ep.DBEpisodeID != 0 {
					found++
				}
			}

			extra = fmt.Sprintf("%d / %d", found, len(c.Episodes))
			if c.DBID == 0 {
				extra = "after import"
			}
		case config.MediaTypeBook:
			extra = c.Author
		}

		candRows = append(candRows, html.Tr(
			gomponents.If(isChosen, html.Class("table-active")),
			html.Td(
				gomponents.If(isChosen, html.Span(html.Class("badge bg-primary"), gomponents.Text("chosen"))),
				gomponents.If(!isChosen, html.Button(
					html.Class("btn btn-outline-primary btn-sm"),
					html.Type("button"),
					hx.Get(fmt.Sprintf("%s/entry?sid=%s&id=%d&cand=%s", mtRoute, s.ID, e.ID, url.QueryEscape(c.Key))),
					hx.Target("#mt-detail-"+idStr),
					hx.Swap("innerHTML"),
					gomponents.Text("Use"),
				)),
			),
			html.Td(mtScoreBadge(c.Dist, strong, medium)),
			html.Td(
				mtDiffText(c.Title, q.Title),
				mtMatcherBadge(c),
				gomponents.If(c.MatchedAs != "",
					html.Div(html.Class("small text-muted"), gomponents.Text("matched as “"+c.MatchedAs+"”"))),
			),
			html.Td(gomponents.Text(yearText(c.Year))),
			html.Td(html.Class("small"), gomponents.Text(c.ExternalID)),
			gomponents.If(extraHead != "", html.Td(html.Class("small"), gomponents.Text(extra))),
			html.Td(html.Class("small"), gomponents.Text(mtMediaSource(c))),
		))
	}

	headCells := []gomponents.Node{
		html.Th(), html.Th(gomponents.Text("Score")), html.Th(gomponents.Text("Title")),
		html.Th(gomponents.Text("Year")), html.Th(gomponents.Text("ID")),
	}
	if extraHead != "" {
		headCells = append(headCells, html.Th(gomponents.Text(extraHead)))
	}

	headCells = append(headCells, html.Th(gomponents.Text("Source")))

	var chosenNodes []gomponents.Node
	if chosen != nil {
		chosenNodes = append(chosenNodes, renderMTMediaFields(chosen, strong, medium))
		if q.IsType == config.MediaTypeSeries && len(chosen.Episodes) > 0 {
			chosenNodes = append(chosenNodes, renderMTMediaEpisodes(chosen))
		}

		listName := ""
		if cfgp, data, _ := s.mtConfig(); cfgp != nil {
			if l := mtImportList(cfgp, data); l >= 0 {
				listName = cfgp.Lists[l].Name
			}
		}

		chosenNodes = append(chosenNodes, renderMTMediaPlan(chosen, plan, planErr, s.Kind, listName))
	}

	return html.Div(
		html.Class("p-2 border rounded bg-body-tertiary"),
		gomponents.Group(info),
		html.Table(
			html.Class("table table-sm mb-3 mt-2"),
			html.THead(html.Tr(headCells...)),
			html.TBody(gomponents.Group(candRows)),
		),
		gomponents.Group(chosenNodes),
	)
}

func yearText(y int) string {
	if y == 0 {
		return ""
	}

	return strconv.Itoa(y)
}

func mtNonEmpty(values ...string) []string {
	out := values[:0]
	for _, v := range values {
		if strings.TrimSpace(v) != "" && v != "parsed: " {
			out = append(out, v)
		}
	}

	return out
}

// renderMTMediaFields renders the field comparison of the chosen candidate.
func renderMTMediaFields(c *importfeed.MediaCandidate, strong, medium float64) gomponents.Node {
	rows := make([]gomponents.Node, 0, len(c.Fields))
	for _, f := range c.Fields {
		var score gomponents.Node = html.Span(html.Class("text-muted small"), gomponents.Text("not scored"))
		if f.Scored {
			score = mtScoreBadge(f.Dist, strong, medium)
		}

		rows = append(rows, html.Tr(
			html.Th(html.Class("fw-normal text-muted"), gomponents.Text(f.Name)),
			html.Td(gomponents.Text(f.Have)),
			html.Td(mtDiffText(f.Wanted, f.Have)),
			html.Td(score),
		))
	}

	return html.Div(
		html.H6(html.Class("mt-2"), gomponents.Text("Comparison with "+mtTitleYear(c.Title, c.Year))),
		html.Table(
			html.Class("table table-sm mb-3"),
			html.THead(html.Tr(
				html.Th(gomponents.Text("Field")), html.Th(gomponents.Text("File")),
				html.Th(gomponents.Text("Candidate")), html.Th(gomponents.Text("Score")),
			)),
			html.TBody(rows...),
		),
	)
}

// renderMTMediaEpisodes lists the file's episodes in the chosen series.
func renderMTMediaEpisodes(c *importfeed.MediaCandidate) gomponents.Node {
	rows := make([]gomponents.Node, 0, len(c.Episodes))
	for _, ep := range c.Episodes {
		var status gomponents.Node

		switch {
		case c.DBID == 0:
			status = html.Span(html.Class("text-muted"), gomponents.Text("known after import"))
		case ep.DBEpisodeID == 0:
			status = html.Span(html.Class("badge bg-danger"), gomponents.Text("not in the series"))
		case c.MediaID == 0:
			status = html.Span(html.Class("text-muted"), gomponents.Text("list entry created on import"))
		case ep.EpisodeID == 0:
			status = html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("not in the list entry"))
		case ep.Files > 0:
			status = gomponents.Text(fmt.Sprintf("%d file(s) in the library", ep.Files))
		default:
			status = html.Span(html.Class("badge bg-success"), gomponents.Text("missing - wanted"))
		}

		rows = append(rows, html.Tr(
			html.Td(gomponents.Text(ep.Identifier)),
			html.Td(gomponents.Text(ep.Title)),
			html.Td(status),
		))
	}

	return html.Table(
		html.Class("table table-sm mb-3"),
		html.THead(html.Tr(
			html.Th(gomponents.Text("Episode")), html.Th(gomponents.Text("Title")), html.Th(gomponents.Text("Library")),
		)),
		html.TBody(rows...),
	)
}

// renderMTMediaPlan shows what organizing the file with c would do.
func renderMTMediaPlan(
	c *importfeed.MediaCandidate,
	plan *structure.MediaFilePlan,
	planErr string,
	kind *mtKind,
	listName string,
) gomponents.Node {
	title := html.H6(html.Class("mt-2"), gomponents.Text("Organize plan"))

	var notes []gomponents.Node
	if c.MediaID == 0 {
		notes = append(notes, html.Div(html.Class("small text-muted mb-1"), gomponents.Text(
			"Processing first adds this "+kind.FileNoun+" to the list “"+listName+"”"+
				mtIf(c.DBID == 0, " and creates its database entry")+
				" (the import path's add_found_list, else the config's first list).")))
	}

	if planErr != "" {
		return html.Div(title, gomponents.Group(notes), mtAlert("info", planErr))
	}

	if plan == nil {
		return html.Div(title, gomponents.Group(notes))
	}

	row := func(name string, value gomponents.Node) gomponents.Node {
		return html.Tr(html.Th(html.Class("fw-normal text-muted"), gomponents.Text(name)), html.Td(value))
	}

	rows := []gomponents.Node{}
	if plan.TargetFile != "" {
		rows = append(rows,
			row("Target folder", html.Code(gomponents.Text(plan.TargetFolder))),
			row("Target file", html.Code(gomponents.Text(plan.TargetFile))),
		)
	}

	if plan.Release != "" || plan.Priority != 0 {
		rows = append(rows, row("Quality", gomponents.Text(fmt.Sprintf("%s (priority %d)", plan.Release, plan.Priority))))
	}

	if len(plan.Languages) > 0 {
		rows = append(rows, row("Languages", gomponents.Text(strings.Join(plan.Languages, ", "))))
	}

	if plan.RuntimeWanted != 0 || plan.RuntimeFound != 0 {
		rows = append(rows, row("Runtime", gomponents.Text(fmt.Sprintf("%d min (wanted %d min)", plan.RuntimeFound, plan.RuntimeWanted))))
	}

	if len(plan.OldFiles) > 0 {
		files := make([]gomponents.Node, 0, len(plan.OldFiles)+1)
		files = append(files, html.Div(gomponents.Text(fmt.Sprintf(
			"%d file(s), priority %d - %s", len(plan.OldFiles), plan.OldPriority, coalesce(plan.OldFilesFate, "kept")))))

		for _, f := range plan.OldFiles {
			files = append(files, html.Div(html.Class("small text-muted"), html.Code(gomponents.Text(f))))
		}

		rows = append(rows, row("Library files", gomponents.Group(files)))
	}

	// The first blocking issue ends the organize; later checks are not
	// reached, so only that one can delete the file.
	issues := make([]gomponents.Node, 0, len(plan.Issues))
	blocked := false

	for _, is := range plan.Issues {
		switch {
		case blocked:
			issues = append(issues, html.Div(html.Class("small text-muted mb-1"),
				gomponents.Text("Later check, not reached: "+is.Text)))
		case is.Deletes:
			blocked = true

			issues = append(issues, mtAlert("danger", "Not organized, the import file is deleted: "+is.Text))
		case is.Blocks:
			blocked = true

			issues = append(issues, mtAlert("warning", "Not organized: "+is.Text))
		default:
			issues = append(issues, mtAlert("info", is.Text))
		}
	}

	if !blocked && plan.TargetFile != "" {
		issues = append(issues, mtAlert("success", "Organized to the target above."))
	}

	return html.Div(
		title,
		gomponents.Group(notes),
		html.Table(html.Class("table table-sm mb-2"), html.TBody(rows...)),
		gomponents.Group(issues),
		html.Div(html.Class("small text-muted"), gomponents.Text(
			"A manually chosen entry skips the quality profile's check_title, which only guards automatic matches.")),
	)
}

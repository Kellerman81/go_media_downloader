package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// The match tester runs the matcher read-only over the folders of a
// data_import path or a move_unprocessed reason folder, shows every candidate
// with its distance, and organizes the entries the user picks with the
// candidate the user picks. Album folders (music, audiobooks) are one entry
// each; movie, series and book folders get one entry per media file (see
// match_tester_media.go). Folders are tested a page at a time; viewing a page
// tests the next one in the background.

const (
	mtPageSize     = 10
	mtParallel     = 2
	mtSessionTTL   = 2 * time.Hour
	mtPollInterval = "every 3s"
	mtRoute        = "/api/admin/matchtester"
)

// Entry processing states.
const (
	mtQueued  = "queued"
	mtRunning = "running"
	mtDone    = "done"
	mtFailed  = "failed"
)

type mtEntry struct {
	ID       int
	Entry    *importfeed.PreviewEntry    // album entries
	Media    *structure.MediaFilePreview // movie, episode and book entries
	Chosen   string                      // candidate key; empty = best
	Selected bool
	Status   string
	Message  string
}

// hasCandidate reports whether the entry can be processed.
func (e *mtEntry) hasCandidate() bool {
	if e.Media != nil {
		return e.mediaCandidate() != nil
	}

	return e.candidate() != nil
}

// candidate returns the chosen candidate, or the best one.
func (e *mtEntry) candidate() *importfeed.PreviewCandidate {
	if e.Chosen != "" {
		if c := e.Entry.Candidate(e.Chosen); c != nil {
			return c
		}
	}

	return e.Entry.Best()
}

type mtFolder struct {
	Path    string
	Done    bool
	Entries []*mtEntry
}

type mtSession struct {
	ID        string
	Kind      *mtKind
	CfgName   string
	ImportIdx int
	Label     string
	Folders   []string

	mu       sync.Mutex
	pages    map[int][]*mtFolder
	entries  map[int]*mtEntry
	nextID   int
	lastUsed time.Time
	ctx      context.Context
	cancel   context.CancelFunc
}

var (
	mtSessions   = make(map[string]*mtSession)
	mtSessionsMu sync.Mutex

	errMTSession = errors.New("match tester session expired - start a new test")
)

// mtConfig returns the media config, its data_import entry and the matcher
// data config of a session; cfgp is nil when the config is gone.
func (s *mtSession) mtConfig() (*config.MediaTypeConfig, *config.MediaDataImportConfig, *config.MediaDataConfig) {
	cfgp := config.GetSettingsMedia(s.CfgName)
	if cfgp == nil || s.ImportIdx >= len(cfgp.DataImport) {
		return nil, nil, nil
	}

	data := &cfgp.DataImport[s.ImportIdx]

	return cfgp, data, structure.AlbumDataConfig(data, cfgp)
}

func (s *mtSession) pageCount() int {
	return (len(s.Folders) + mtPageSize - 1) / mtPageSize
}

// ensurePage starts testing the folders of page n unless that already
// happened, and returns its folders.
func (s *mtSession) ensurePage(n int) []*mtFolder {
	if n < 0 || n >= s.pageCount() {
		return nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if p, ok := s.pages[n]; ok {
		return p
	}

	end := min((n+1)*mtPageSize, len(s.Folders))

	p := make([]*mtFolder, 0, end-n*mtPageSize)
	for _, f := range s.Folders[n*mtPageSize : end] {
		p = append(p, &mtFolder{Path: f})
	}

	s.pages[n] = p

	go s.testFolders(p)

	return p
}

// testFolders previews the folders mtParallel at a time.
func (s *mtSession) testFolders(folders []*mtFolder) {
	defer logger.HandlePanic()

	cfgp, _, dataCfg := s.mtConfig()

	sem := make(chan struct{}, mtParallel)

	var wg sync.WaitGroup

	for _, f := range folders {
		wg.Add(1)

		sem <- struct{}{}

		go func() {
			defer func() {
				<-sem
				wg.Done()
			}()
			defer logger.HandlePanic()

			var entries []*mtEntry

			switch {
			case cfgp == nil:
				entries = []*mtEntry{{Entry: &importfeed.PreviewEntry{Folder: f.Path, Err: "media config not found"}}}
			case s.ctx.Err() != nil:
			case s.Kind.album():
				for _, p := range importfeed.PreviewAudioFolder(s.ctx, f.Path, cfgp, dataCfg) {
					entries = append(entries, &mtEntry{Entry: p})
				}
			default:
				entries = s.testMediaFolder(f.Path)
			}

			strong, _ := importfeed.RecThresholds(dataCfg)

			s.mu.Lock()
			defer s.mu.Unlock()

			for _, e := range entries {
				s.nextID++
				e.ID = s.nextID

				e.Selected = e.bestDist() < strong

				f.Entries = append(f.Entries, e)
				s.entries[e.ID] = e
			}

			f.Done = true
		}()
	}

	wg.Wait()
}

func mtGetSession(ctx *gin.Context) (*mtSession, error) {
	mtSessionsMu.Lock()
	defer mtSessionsMu.Unlock()

	s, ok := mtSessions[ctx.Query("sid")]
	if !ok {
		s, ok = mtSessions[ctx.PostForm("sid")]
	}

	if !ok {
		return nil, errMTSession
	}

	s.lastUsed = time.Now()

	return s, nil
}

func mtNewSession(kind *mtKind, cfgName string, importIdx int, label string, folders []string) *mtSession {
	b := make([]byte, 8)
	rand.Read(b)

	ctx, cancel := context.WithCancel(context.Background())
	s := &mtSession{
		ID:        hex.EncodeToString(b),
		Kind:      kind,
		CfgName:   cfgName,
		ImportIdx: importIdx,
		Label:     label,
		Folders:   folders,
		pages:     make(map[int][]*mtFolder),
		entries:   make(map[int]*mtEntry),
		lastUsed:  time.Now(),
		ctx:       ctx,
		cancel:    cancel,
	}

	mtSessionsMu.Lock()
	defer mtSessionsMu.Unlock()

	for id, old := range mtSessions {
		if time.Since(old.lastUsed) > mtSessionTTL {
			old.cancel()
			delete(mtSessions, id)
		}
	}

	mtSessions[s.ID] = s

	return s
}

// ── Sources ──────────────────────────────────────────────────────────────────

type mtSource struct {
	Value string // "<importIdx>|<path>"
	Label string
}

// mtSources lists the data_import paths of cfgp and the reason folders below
// their move_unprocessed paths.
func mtSources(cfgp *config.MediaTypeConfig) []mtSource {
	var out []mtSource

	seen := make(map[string]bool)

	for i := range cfgp.DataImport {
		imp := &cfgp.DataImport[i]
		if imp.CfgPath != nil && imp.CfgPath.Path != "" && !seen[imp.CfgPath.Path] {
			seen[imp.CfgPath.Path] = true
			out = append(out, mtSource{
				Value: strconv.Itoa(i) + "|" + imp.CfgPath.Path,
				Label: "Import: " + imp.CfgPath.Path + " (" + imp.TemplatePath + ")",
			})
		}

		if imp.MoveUnprocessed == "" {
			continue
		}

		reasons, err := os.ReadDir(imp.MoveUnprocessed)
		if err != nil {
			continue
		}

		for _, r := range reasons {
			p := filepath.Join(imp.MoveUnprocessed, r.Name())
			if !r.IsDir() || seen[p] {
				continue
			}

			seen[p] = true

			n := 0
			if sub, err := os.ReadDir(p); err == nil {
				for _, e := range sub {
					if e.IsDir() {
						n++
					}
				}
			}

			out = append(out, mtSource{
				Value: strconv.Itoa(i) + "|" + p,
				Label: fmt.Sprintf("Unprocessed: %s (%d folders)", r.Name(), n),
			})
		}
	}

	return out
}

// mtResolveSource checks value against the sources of cfgp, so only
// configured paths can be scanned.
func mtResolveSource(cfgp *config.MediaTypeConfig, value string) (int, string, string, bool) {
	for _, src := range mtSources(cfgp) {
		if src.Value != value {
			continue
		}

		idx, path, _ := strings.Cut(value, "|")
		i, err := strconv.Atoi(idx)

		return i, path, src.Label, err == nil
	}

	return 0, "", "", false
}

// mtCollectFolders returns the folders below root that the structure job
// would process: every album folder for music and audiobooks, every direct
// subfolder otherwise.
func mtCollectFolders(
	ctx context.Context,
	root string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	var out []string

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		if !mtKindOf(cfgp.IsType).album() {
			out = append(out, filepath.Join(root, e.Name()))
			continue
		}

		for _, f := range structure.CollectAlbumFolders(ctx, filepath.Join(root, e.Name()), cfgp, data.CfgPath) {
			if logger.ContainsI(f, "_unpack") ||
				logger.SlicesContainsPart2I(data.CfgPath.Disallowed, f) ||
				(data.CfgPath.BlockedLen >= 1 && logger.SlicesContainsPart2I(data.CfgPath.Blocked, f)) {
				continue
			}

			out = append(out, f)
		}
	}

	return out
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func adminPageMatchTester(ctx *gin.Context) {
	kind := mtKindByKey(ctx.Query("kind"))
	pageNode := page(kind.Title, false, false, true, renderMatchTesterPage(getCSRFToken(ctx), kind))

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func mtRender(ctx *gin.Context, nodes ...gomponents.Node) {
	var buf strings.Builder
	for _, n := range nodes {
		n.Render(&buf)
	}

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

func mtAlert(class, msg string) gomponents.Node {
	return html.Div(html.Class("alert alert-"+class+" mb-2"), gomponents.Text(msg))
}

// HandleMatchTesterSources returns the source options of the chosen config.
func HandleMatchTesterSources(ctx *gin.Context) {
	cfgp := config.GetSettingsMedia(ctx.Query("MediaConfig"))
	if cfgp == nil {
		mtRender(ctx, html.Option(html.Value(""), gomponents.Text("-- select a media config --")))
		return
	}

	sources := mtSources(cfgp)
	if len(sources) == 0 {
		mtRender(ctx, html.Option(html.Value(""), gomponents.Text("-- no import or unprocessed paths --")))
		return
	}

	nodes := make([]gomponents.Node, 0, len(sources))
	for _, src := range sources {
		nodes = append(nodes, html.Option(html.Value(src.Value), gomponents.Text(src.Label)))
	}

	mtRender(ctx, nodes...)
}

// HandleMatchTesterStart collects the album folders of the chosen source and
// starts testing the first page.
func HandleMatchTesterStart(ctx *gin.Context) {
	cfgName := ctx.PostForm("MediaConfig")

	cfgp := config.GetSettingsMedia(cfgName)

	kind := mtKindByKey(ctx.PostForm("kind"))
	if cfgp == nil || mtKindOf(cfgp.IsType) != kind {
		mtRender(ctx, mtAlert("danger", "Select a "+kind.ConfigNoun+" media config."))
		return
	}

	idx, root, label, ok := mtResolveSource(cfgp, ctx.PostForm("Source"))
	if !ok || cfgp.DataImport[idx].CfgPath == nil {
		mtRender(ctx, mtAlert("danger", "Select a source folder."))
		return
	}

	folders := mtCollectFolders(ctx, root, cfgp, &cfgp.DataImport[idx])
	if len(folders) == 0 {
		mtRender(ctx, mtAlert("info", "No "+kind.FolderNoun+" found in "+root+"."))
		return
	}

	s := mtNewSession(kind, cfgName, idx, label, folders)
	s.ensurePage(0)
	s.ensurePage(1)

	mtRender(ctx, renderMTPage(s, 0, getCSRFToken(ctx)))
}

// HandleMatchTesterPage shows page n and starts testing page n+1.
func HandleMatchTesterPage(ctx *gin.Context) {
	s, err := mtGetSession(ctx)
	if err != nil {
		mtRender(ctx, mtAlert("warning", err.Error()))
		return
	}

	n, _ := strconv.Atoi(ctx.Query("page"))
	n = max(0, min(n, s.pageCount()-1))

	s.ensurePage(n)
	s.ensurePage(n + 1)

	mtRender(ctx, renderMTPage(s, n, getCSRFToken(ctx)))
}

// HandleMatchTesterFolder returns a folder's rows once it is tested; until
// then the placeholder row polls this handler.
func HandleMatchTesterFolder(ctx *gin.Context) {
	s, err := mtGetSession(ctx)
	if err != nil {
		mtRender(ctx, html.Tr(html.Td(html.ColSpan("9"), gomponents.Text(err.Error()))))
		return
	}

	n, _ := strconv.Atoi(ctx.Query("page"))
	i, _ := strconv.Atoi(ctx.Query("idx"))

	folders := s.ensurePage(n)
	if i < 0 || i >= len(folders) {
		mtRender(ctx, gomponents.Text(""))
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	mtRender(ctx, renderMTFolder(s, n, i, folders[i])...)
}

// HandleMatchTesterEntry shows an entry's candidates and the track comparison
// of the chosen one. With cand set, that candidate becomes the chosen one.
func HandleMatchTesterEntry(ctx *gin.Context) {
	s, err := mtGetSession(ctx)
	if err != nil {
		mtRender(ctx, mtAlert("warning", err.Error()))
		return
	}

	id, _ := strconv.Atoi(ctx.Query("id"))

	if !s.Kind.album() {
		s.handleMediaEntry(ctx, id)
		return
	}

	s.mu.Lock()
	e, ok := s.entries[id]

	if ok && ctx.Query("cand") != "" && e.Entry.Candidate(ctx.Query("cand")) != nil {
		e.Chosen = ctx.Query("cand")
	}

	var c *importfeed.PreviewCandidate
	if ok {
		c = e.candidate()
	}
	s.mu.Unlock()

	if !ok {
		mtRender(ctx, mtAlert("warning", "Entry not found."))
		return
	}

	cfgp, data, dataCfg := s.mtConfig()

	// The rename plan needs the database row; external candidates get theirs
	// on import.
	var plan mtPlan
	if c != nil && c.DBID != 0 && cfgp != nil {
		plan = mtRenamePlan(ctx, cfgp, data, e.Entry, c.DBID)
	}

	strong, medium := importfeed.RecThresholds(dataCfg)

	mtRender(ctx,
		renderMTDetail(s, e, c, plan, strong, medium),
		renderMTSummaryCells(e, c, strong, medium, true),
	)
}

// HandleMatchTesterSelect stores whether an entry is selected for processing.
func HandleMatchTesterSelect(ctx *gin.Context) {
	s, err := mtGetSession(ctx)
	if err != nil {
		ctx.Status(http.StatusGone)
		return
	}

	id, _ := strconv.Atoi(ctx.PostForm("id"))

	s.mu.Lock()
	if e, ok := s.entries[id]; ok {
		e.Selected = ctx.PostForm("sel_"+strconv.Itoa(id)) != ""
	}
	s.mu.Unlock()

	ctx.Status(http.StatusNoContent)
}

// HandleMatchTesterStatus returns an entry's status cell.
func HandleMatchTesterStatus(ctx *gin.Context) {
	s, err := mtGetSession(ctx)
	if err != nil {
		mtRender(ctx, html.Td(gomponents.Text("expired")))
		return
	}

	id, _ := strconv.Atoi(ctx.Query("id"))

	s.mu.Lock()
	defer s.mu.Unlock()

	e, ok := s.entries[id]
	if !ok {
		mtRender(ctx, html.Td())
		return
	}

	mtRender(ctx, renderMTStatus(s, e, false))
}

// HandleMatchTesterProcess queues the selected entries of a page for
// organizing with their chosen candidate.
func HandleMatchTesterProcess(ctx *gin.Context) {
	s, err := mtGetSession(ctx)
	if err != nil {
		mtRender(ctx, mtAlert("warning", err.Error()))
		return
	}

	cfgp, data, _ := s.mtConfig()
	if cfgp == nil {
		mtRender(ctx, mtAlert("danger", "Media config not found."))
		return
	}

	n, _ := strconv.Atoi(ctx.PostForm("page"))

	var (
		queued  []*mtEntry
		skipped int
	)

	s.mu.Lock()
	for _, f := range s.pages[n] {
		for _, e := range f.Entries {
			if !e.Selected || e.Status == mtQueued || e.Status == mtRunning || e.Status == mtDone {
				continue
			}

			if !e.hasCandidate() {
				skipped++
				continue
			}

			e.Status = mtQueued
			e.Message = ""
			queued = append(queued, e)
		}
	}
	s.mu.Unlock()

	for _, e := range queued {
		mtDispatch(s, e, cfgp, data)
	}

	msg := fmt.Sprintf("Queued %d entries for organizing.", len(queued))
	if skipped > 0 {
		msg += fmt.Sprintf(" %d selected entries have no candidate.", skipped)
	}

	nodes := []gomponents.Node{mtAlert("info", msg)}

	s.mu.Lock()
	for _, e := range queued {
		nodes = append(nodes, renderMTStatus(s, e, true))
	}
	s.mu.Unlock()

	mtRender(ctx, nodes...)
}

// mtDispatch organizes one entry on the Data queue: the chosen candidate's
// database row is created when needed, then the folder (or episode file) is
// force-matched to it.
func mtDispatch(s *mtSession, e *mtEntry, cfgp *config.MediaTypeConfig, data *config.MediaDataImportConfig) {
	setStatus := func(status, msg string) {
		s.mu.Lock()
		e.Status = status
		e.Message = msg
		s.mu.Unlock()
	}

	if e.Media != nil {
		mtDispatchMedia(s, e, cfgp, data, setStatus)
		return
	}

	name := "matchtester_" + cfgp.NamePrefix + "_" + e.Entry.Folder
	if e.Entry.File != "" {
		name += "_" + filepath.Base(e.Entry.File)
	}

	err := worker.Dispatch(name, func(_ uint32, ctx context.Context) error {
		setStatus(mtRunning, "")

		s.mu.Lock()
		c := e.candidate()
		s.mu.Unlock()

		listid := mtImportList(cfgp, data)

		dbID, err := importfeed.ImportPreviewCandidate(ctx, c, e.Entry, cfgp, listid)
		if err != nil {
			setStatus(mtFailed, "creating database entry: "+err.Error())
			return err
		}

		// A retry after a failed organize must not create the row again.
		s.mu.Lock()
		c.DBID = dbID
		s.mu.Unlock()

		org := structure.NewStructure(cfgp, data.TemplatePath, mtDefaultTemplate(cfgp), false, false, 0)
		if org == nil {
			setStatus(mtFailed, "path template not found")
			return logger.ErrNotFound
		}
		defer org.Close()

		_, err = org.ForceMatchAlbumFolder(ctx, e.Entry.Folder, cfgp, data,
			structure.AlbumForce{DBID: dbID, File: e.Entry.File}, false)
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

// mtImportList returns the list new entries are imported into: the
// add_found_list of the data_import entry, else the config's first list.
func mtImportList(cfgp *config.MediaTypeConfig, data *config.MediaDataImportConfig) int {
	listid := cfgp.GetMediaListsEntryListID(structure.AlbumDataConfig(data, cfgp).AddFoundList)
	if listid == -1 && len(cfgp.Lists) > 0 {
		listid = 0
	}

	return listid
}

func mtDefaultTemplate(cfgp *config.MediaTypeConfig) string {
	if cfgp.DataLen >= 1 {
		return cfgp.Data[0].TemplatePath
	}

	return ""
}

// mtPlan is what a forced organize of an album entry would do.
type mtPlan struct {
	Files   map[string]string // target filename by source basename
	Dir     string
	Err     string
	Upgrade *structure.AlbumUpgradeCheck
}

// mtRenamePlan computes the plan of an entry matched to dbID.
func mtRenamePlan(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	entry *importfeed.PreviewEntry,
	dbID uint,
) mtPlan {
	org := structure.NewStructure(cfgp, data.TemplatePath, mtDefaultTemplate(cfgp), false, false, 0)
	if org == nil {
		return mtPlan{Err: "path template not found"}
	}
	defer org.Close()

	preview, err := org.ForceMatchAlbumFolder(ctx, entry.Folder, cfgp, data,
		structure.AlbumForce{DBID: dbID, File: entry.File}, true)
	if err != nil {
		return mtPlan{Err: err.Error()}
	}

	plan := mtPlan{
		Files:   make(map[string]string, len(preview.Tracks)),
		Dir:     preview.FolderTarget,
		Upgrade: &preview.Upgrade,
	}
	for _, t := range preview.Tracks {
		plan.Files[t.Source] = t.Target
	}

	return plan
}

// ── Rendering ────────────────────────────────────────────────────────────────

func renderMatchTesterPage(csrfToken string, kind *mtKind) gomponents.Node {
	configs := kind.configs()

	options := []gomponents.Node{html.Option(html.Value(""), gomponents.Text("-- select --"))}
	for _, c := range configs {
		options = append(options, html.Option(html.Value(c), gomponents.Text(c)))
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fa-solid "+kind.Icon+" header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text(kind.Title)),
					html.P(html.Class("header-subtitle"), gomponents.Text(kind.Subtitle)),
				),
			),
		),
		html.Form(
			html.Class("config-form"),
			html.ID("matchTesterForm"),
			html.Input(html.Type("hidden"), html.Name("kind"), html.Value(kind.Key)),
			html.Div(
				html.Class("row g-3 align-items-end"),
				html.Div(
					html.Class("col-md-4"),
					html.Label(html.Class("form-label"), html.For("mtMediaConfig"), gomponents.Text("Media Configuration")),
					html.Select(
						html.Class("form-select"),
						html.ID("mtMediaConfig"),
						html.Name("MediaConfig"),
						hx.Get(mtRoute+"/sources"),
						hx.Target("#mtSource"),
						hx.Trigger("change"),
						gomponents.Group(options),
					),
				),
				html.Div(
					html.Class("col-md-6"),
					html.Label(html.Class("form-label"), html.For("mtSource"), gomponents.Text("Source")),
					html.Select(
						html.Class("form-select"),
						html.ID("mtSource"),
						html.Name("Source"),
						html.Option(html.Value(""), gomponents.Text("-- select a media config --")),
					),
				),
				html.Div(
					html.Class("col-md-2"),
					html.Button(
						html.Class(ClassBtnPrimary+" w-100"),
						html.Type("button"),
						hx.Post(mtRoute+"/start"),
						hx.Target("#mtResults"),
						hx.Swap("innerHTML"),
						hx.Headers(createHTMXHeaders(csrfToken)),
						hx.Include("#matchTesterForm"),
						hx.Indicator("#mtStartSpinner"),
						html.Span(html.ID("mtStartSpinner"), html.Class("htmx-indicator spinner-border spinner-border-sm me-1")),
						gomponents.Text("Start Test"),
					),
				),
			),
		),
		html.Div(
			html.Class("small text-muted mt-2"),
			gomponents.Text(fmt.Sprintf(
				"Folders are tested %d per page; while you review a page the next one is tested in the background. Score = similarity (1 − distance): ",
				mtPageSize,
			)),
			html.Span(html.Class("badge bg-success"), gomponents.Text("strong")),
			gomponents.Text(" below strong_rec_thresh, "),
			html.Span(html.Class("badge bg-warning text-dark"), gomponents.Text("medium")),
			gomponents.Text(" up to medium_rec_thresh, "),
			html.Span(html.Class("badge bg-danger"), gomponents.Text("weak")),
			gomponents.Text(" above. Strong matches are preselected."),
		),
		html.Div(html.ID("mtResults"), html.Class("mt-4")),
	)
}

func renderMTPage(s *mtSession, n int, csrfToken string) gomponents.Node {
	folders := s.ensurePage(n)
	pages := s.pageCount()

	s.mu.Lock()
	defer s.mu.Unlock()

	rows := make([]gomponents.Node, 0, len(folders))
	for i, f := range folders {
		rows = append(rows, renderMTFolder(s, n, i, f)...)
	}

	pager := func(target int, label string, enabled bool) gomponents.Node {
		if !enabled {
			return html.Button(html.Class(ClassBtnSecondary+" btn-sm"), html.Disabled(), gomponents.Text(label))
		}

		return html.Button(
			html.Class(ClassBtnSecondary+" btn-sm"),
			html.Type("button"),
			hx.Get(fmt.Sprintf("%s/page?sid=%s&page=%d", mtRoute, s.ID, target)),
			hx.Target("#mtResults"),
			hx.Swap("innerHTML"),
			gomponents.Text(label),
		)
	}

	return html.Div(
		html.Div(
			html.Class("d-flex flex-wrap align-items-center gap-2 mb-2"),
			html.Strong(gomponents.Text(s.Label)),
			html.Span(html.Class("text-muted"), gomponents.Text(fmt.Sprintf(
				"— %d %s, page %d of %d", len(s.Folders), s.Kind.FolderNoun, n+1, pages,
			))),
			html.Div(
				html.Class("ms-auto d-flex gap-2"),
				pager(n-1, "« Previous", n > 0),
				pager(n+1, "Next »", n+1 < pages),
				html.Button(
					html.Class(ClassBtnSuccess+" btn-sm"),
					html.Type("button"),
					hx.Post(mtRoute+"/process"),
					hx.Vals(fmt.Sprintf(`{"sid": %q, "page": "%d"}`, s.ID, n)),
					hx.Headers(createHTMXHeaders(csrfToken)),
					hx.Target("#mtProcessResult"),
					hx.Swap("innerHTML"),
					hx.Confirm("Organize the selected entries with their chosen candidates? Files will be moved and missing database entries created."),
					html.I(html.Class("fa-solid fa-play me-1")),
					gomponents.Text("Process selected"),
				),
			),
		),
		html.Div(html.ID("mtProcessResult")),
		html.Div(
			html.Class("table-responsive"),
			html.Table(
				html.Class("table table-sm table-hover align-middle"),
				html.THead(s.Kind.tableHead()),
				html.TBody(
					html.ID("mtBody"),
					hx.Headers(createHTMXHeaders(csrfToken)),
					gomponents.Group(rows),
				),
			),
		),
	)
}

// renderMTFolder renders a folder's entry rows, or a polling placeholder
// while it is untested. Callers hold s.mu.
func renderMTFolder(s *mtSession, n, i int, f *mtFolder) []gomponents.Node {
	rowID := fmt.Sprintf("mt-folder-%d-%d", n, i)

	if !f.Done {
		return []gomponents.Node{html.Tr(
			html.ID(rowID),
			hx.Get(fmt.Sprintf("%s/folder?sid=%s&page=%d&idx=%d", mtRoute, s.ID, n, i)),
			hx.Trigger(mtPollInterval),
			hx.Swap("outerHTML"),
			html.Td(),
			html.Td(html.Class("text-muted"), html.Title(f.Path), gomponents.Text(filepath.Base(f.Path))),
			html.Td(
				html.ColSpan(strconv.Itoa(s.Kind.columns()-2)),
				html.Class("text-muted"),
				html.Span(html.Class("spinner-border spinner-border-sm me-2")),
				gomponents.Text("testing…"),
			),
		)}
	}

	_, _, dataCfg := s.mtConfig()
	strong, medium := importfeed.RecThresholds(dataCfg)

	out := make([]gomponents.Node, 0, 2*len(f.Entries))
	for _, e := range f.Entries {
		row := renderMTEntryRow
		if e.Media != nil {
			row = renderMTMediaEntryRow
		}

		out = append(out, row(s, e, strong, medium), html.Tr(
			html.Class("d-none"),
			html.ID(fmt.Sprintf("mt-detail-row-%d", e.ID)),
			html.Td(html.ColSpan(strconv.Itoa(s.Kind.columns())), html.ID(fmt.Sprintf("mt-detail-%d", e.ID))),
		))
	}

	if len(f.Entries) == 0 {
		out = append(out, html.Tr(
			html.Td(),
			html.Td(html.Class("text-muted"), html.Title(f.Path), gomponents.Text(filepath.Base(f.Path))),
			html.Td(html.ColSpan(strconv.Itoa(s.Kind.columns()-2)), html.Class("text-muted"),
				gomponents.Text("no "+s.Kind.FileNoun+" files")),
		))
	}

	return out
}

// mtSelectCell renders the selection checkbox of an entry that has a
// candidate, an empty cell otherwise.
func mtSelectCell(s *mtSession, e *mtEntry, selectable bool) gomponents.Node {
	if !selectable {
		return html.Td()
	}

	return html.Td(html.Input(
		html.Type("checkbox"),
		html.Class("form-check-input"),
		html.Name("sel_"+strconv.Itoa(e.ID)),
		gomponents.If(e.Selected, html.Checked()),
		hx.Post(mtRoute+"/select"),
		hx.Vals(fmt.Sprintf(`{"sid": %q, "id": "%d"}`, s.ID, e.ID)),
		hx.Trigger("change"),
		hx.Swap("none"),
	))
}

// mtDetailLink renders the link that opens an entry's detail row.
func mtDetailLink(s *mtSession, e *mtEntry, name string) gomponents.Node {
	idStr := strconv.Itoa(e.ID)

	return html.A(
		html.Href("#"),
		hx.Get(fmt.Sprintf("%s/entry?sid=%s&id=%d", mtRoute, s.ID, e.ID)),
		hx.Target("#mt-detail-"+idStr),
		hx.Swap("innerHTML"),
		gomponents.Attr("hx-on::after-request",
			"document.getElementById('mt-detail-row-"+idStr+"').classList.remove('d-none')"),
		gomponents.Text(name),
	)
}

func renderMTEntryRow(s *mtSession, e *mtEntry, strong, medium float64) gomponents.Node {
	p := e.Entry
	c := e.candidate()

	name := filepath.Base(p.Folder)
	if p.File != "" {
		name = filepath.Base(p.File)
	}

	idStr := strconv.Itoa(e.ID)

	checkbox := mtSelectCell(s, e, c != nil)

	folderCell := html.Td(
		html.Title(coalesce(p.File, p.Folder)),
		mtDetailLink(s, e, name),
		gomponents.If(p.Err != "" && c == nil,
			html.Div(html.Class("small text-danger"), gomponents.Text(p.Err))),
	)

	return html.Tr(
		html.ID("mt-row-"+idStr),
		checkbox,
		folderCell,
		html.Td(gomponents.Text(p.Album)),
		renderMTSummaryCells(e, c, strong, medium, false),
		renderMTStatus(s, e, false),
	)
}

// renderMTSummaryCells renders the wanted album/artist, track and score
// cells of an entry row; oob swaps them in after a candidate change. The
// have-artist cell sits between wanted album and wanted artist, so it is part
// of the group.
func renderMTSummaryCells(e *mtEntry, c *importfeed.PreviewCandidate, strong, medium float64, oob bool) gomponents.Node {
	idStr := strconv.Itoa(e.ID)
	p := e.Entry

	cell := func(suffix string, children ...gomponents.Node) gomponents.Node {
		return html.Td(
			html.ID("mt-"+suffix+"-"+idStr),
			gomponents.If(oob, hx.SwapOOB("true")),
			gomponents.Group(children),
		)
	}

	if c == nil {
		return gomponents.Group([]gomponents.Node{
			cell("walbum", html.Span(html.Class("text-muted"), gomponents.Text("—"))),
			cell("hartist", gomponents.Text(p.Artist)),
			cell("wartist", html.Span(html.Class("text-muted"), gomponents.Text("—"))),
			cell("tracks", gomponents.Text(strconv.Itoa(p.Files))),
			cell("score", html.Span(html.Class("badge bg-secondary"), gomponents.Text("no match"))),
		})
	}

	return gomponents.Group([]gomponents.Node{
		cell("walbum",
			mtDiffText(c.Title, p.Album),
			html.Div(html.Class("small text-muted"), gomponents.Text(mtCandidateSource(c))),
		),
		cell("hartist", gomponents.Text(p.Artist)),
		cell("wartist", mtDiffText(c.Artist, p.Artist)),
		cell("tracks", gomponents.Text(fmt.Sprintf("%d / %d", p.Files, c.Tracks))),
		cell("score", mtScoreBadge(c.Dist, strong, medium)),
	})
}

func renderMTStatus(s *mtSession, e *mtEntry, oob bool) gomponents.Node {
	id := "mt-status-" + strconv.Itoa(e.ID)

	var (
		poll  bool
		inner gomponents.Node
	)

	switch e.Status {
	case mtQueued:
		poll = true
		inner = html.Span(html.Class("badge bg-info"), gomponents.Text("queued"))
	case mtRunning:
		poll = true
		inner = html.Span(html.Class("badge bg-primary"),
			html.Span(html.Class("spinner-border spinner-border-sm me-1")), gomponents.Text("running"))
	case mtDone:
		inner = html.Span(html.Class("badge bg-success"), gomponents.Text("organized"))
	case mtFailed:
		inner = html.Span(html.Class("badge bg-danger"), html.Title(e.Message), gomponents.Text("failed"))
	default:
		inner = gomponents.Text("")
	}

	return html.Td(
		html.ID(id),
		gomponents.If(oob, hx.SwapOOB("true")),
		gomponents.If(poll, gomponents.Group([]gomponents.Node{
			hx.Get(fmt.Sprintf("%s/status?sid=%s&id=%d", mtRoute, s.ID, e.ID)),
			hx.Trigger(mtPollInterval),
			hx.Swap("outerHTML"),
		})),
		inner,
		gomponents.If(e.Status == mtFailed && e.Message != "",
			html.Div(html.Class("small text-danger"), gomponents.Text(e.Message))),
	)
}

func renderMTDetail(
	s *mtSession,
	e *mtEntry,
	chosen *importfeed.PreviewCandidate,
	plan mtPlan,
	strong, medium float64,
) gomponents.Node {
	p := e.Entry
	idStr := strconv.Itoa(e.ID)

	info := []gomponents.Node{
		html.Strong(gomponents.Text(coalesce(p.File, p.Folder))),
		html.Div(html.Class("small text-muted"), gomponents.Text(fmt.Sprintf(
			"%d files, %s%s%s",
			p.Files, mtDuration(p.RuntimeMS),
			mtIf(p.MBID != "", ", MBID "+p.MBID), mtIf(p.ASIN != "", ", ASIN "+p.ASIN),
		))),
	}

	if len(p.Candidates) == 0 {
		return html.Div(
			html.Class("p-2 border rounded bg-body-tertiary"),
			gomponents.Group(info),
			mtAlert("warning", "No candidates found ("+p.Err+")."),
		)
	}

	candRows := make([]gomponents.Node, 0, len(p.Candidates))
	for _, c := range p.Candidates {
		isChosen := chosen != nil && c.Key == chosen.Key

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
			html.Td(mtDiffText(c.Title, p.Album)),
			html.Td(mtDiffText(c.Artist, p.Artist)),
			html.Td(gomponents.Text(mtIf(c.Year > 0, strconv.Itoa(c.Year)))),
			html.Td(gomponents.Text(fmt.Sprintf("%d / %d", p.Files, c.Tracks))),
			html.Td(gomponents.Text(mtDuration(c.RuntimeMS))),
			html.Td(
				gomponents.Text(mtCandidateSource(c)),
				gomponents.If(c.ExternalID != "",
					html.Div(html.Class("small text-muted"), gomponents.Text(c.ExternalID))),
			),
			html.Td(html.Class("small"), gomponents.Text(mtIssues(c, p))),
		))
	}

	var trackTable gomponents.Node
	if chosen != nil {
		trackTable = renderMTTracks(p, chosen, plan, strong, medium)
	}

	return html.Div(
		html.Class("p-2 border rounded bg-body-tertiary"),
		gomponents.Group(info),
		html.Table(
			html.Class("table table-sm mb-3 mt-2"),
			html.THead(html.Tr(
				html.Th(), html.Th(gomponents.Text("Score")), html.Th(gomponents.Text("Album")),
				html.Th(gomponents.Text("Artist")), html.Th(gomponents.Text("Year")),
				html.Th(gomponents.Text("Tracks")), html.Th(gomponents.Text("Length")),
				html.Th(gomponents.Text("Source")), html.Th(gomponents.Text("Differences")),
			)),
			html.TBody(gomponents.Group(candRows)),
		),
		trackTable,
	)
}

func renderMTTracks(
	p *importfeed.PreviewEntry,
	c *importfeed.PreviewCandidate,
	plan mtPlan,
	strong, medium float64,
) gomponents.Node {
	var planNote gomponents.Node
	switch {
	case c.DBID == 0:
		planNote = html.Div(html.Class("small text-muted mb-1"), gomponents.Text(
			"Not in the database yet - it is created from "+mtCandidateSource(c)+" when processed; the new filenames are computed then."))
	case plan.Err != "":
		planNote = html.Div(html.Class("small text-danger mb-1"), gomponents.Text("Rename plan: "+plan.Err))
	case plan.Dir != "":
		planNote = html.Div(html.Class("small mb-1"), html.Strong(gomponents.Text("Target folder: ")), gomponents.Text(plan.Dir))
	}

	rows := make([]gomponents.Node, 0, len(c.Rows))
	for _, r := range c.Rows {
		var rowClass, mark string

		switch {
		case r.Missing:
			rowClass, mark = "table-warning", "missing"
		case r.Extra:
			rowClass, mark = "table-danger", "extra"
		}

		target := ""
		if r.LocalFile != "" {
			target = plan.Files[r.LocalFile]
		}

		rows = append(rows, html.Tr(
			gomponents.If(rowClass != "", html.Class(rowClass)),
			html.Td(html.Class("small"), gomponents.Text(r.LocalFile), gomponents.If(mark != "",
				html.Span(html.Class("badge bg-secondary ms-1"), gomponents.Text(mark)))),
			html.Td(
				html.Class("small"),
				gomponents.Text(target),
				gomponents.If(r.Extra && target != "", html.Span(
					html.Class("badge bg-warning text-dark ms-1"),
					html.Title("This file matched no track; the forced organize gives it the name of the closest unmatched track."),
					gomponents.Text("fallback"),
				)),
			),
			html.Td(gomponents.Text(mtTrackPos(r.LocalDisc, r.LocalTrack, r.Extra || !r.Missing))),
			html.Td(gomponents.Text(mtTrackPos(r.WantedDisc, r.WantedTrack, !r.Extra))),
			html.Td(gomponents.Text(r.LocalTitle)),
			html.Td(mtDiffText(r.WantedTitle, r.LocalTitle)),
			html.Td(gomponents.Text(r.LocalArtist)),
			html.Td(mtDiffText(r.WantedArtist, r.LocalArtist)),
			html.Td(gomponents.Text(mtIf(!r.Missing, mtDuration(r.LocalRuntimeMS)))),
			html.Td(gomponents.Text(mtIf(!r.Extra, mtDuration(r.WantedRuntimeMS)))),
			html.Td(gomponents.If(!r.Missing && !r.Extra, mtScoreBadge(r.Dist, strong, medium))),
		))
	}

	var leftover gomponents.Node
	if n := p.Files - c.Tracks; n > 0 {
		leftover = html.Div(html.Class("small text-warning-emphasis mb-1"), gomponents.Text(fmt.Sprintf(
			"%d files more than this release has tracks: they are not moved and stay in the source folder.", n)))
	}

	return html.Div(
		html.H6(gomponents.Text("Tracks: "+c.Title)),
		planNote,
		renderMTUpgrade(plan.Upgrade),
		leftover,
		html.Div(
			html.Class("table-responsive"),
			html.Table(
				html.Class("table table-sm table-bordered"),
				html.THead(html.Tr(
					html.Th(gomponents.Text("File")), html.Th(gomponents.Text("New filename")),
					html.Th(gomponents.Text("Have #")), html.Th(gomponents.Text("Wanted #")),
					html.Th(gomponents.Text("Have title")), html.Th(gomponents.Text("Wanted title")),
					html.Th(gomponents.Text("Have artist")), html.Th(gomponents.Text("Wanted artist")),
					html.Th(gomponents.Text("Have length")), html.Th(gomponents.Text("Wanted length")),
					html.Th(gomponents.Text("Score")),
				)),
				html.TBody(gomponents.Group(rows)),
			),
		),
	)
}

// renderMTUpgrade shows the library files of the chosen release and whether
// the organize's upgrade check lets the new files replace them.
func renderMTUpgrade(u *structure.AlbumUpgradeCheck) gomponents.Node {
	if u == nil {
		return nil
	}

	if len(u.Existing) == 0 {
		return html.Div(html.Class("small text-muted mb-1"),
			gomponents.Text("Library: no files of this release yet."))
	}

	files := make([]gomponents.Node, 0, len(u.Existing))
	for _, f := range u.Existing {
		bitrate := "?"
		if f.Bitrate > 0 {
			bitrate = strconv.Itoa(f.Bitrate) + " kbps"
		}

		files = append(files, html.Tr(
			html.Td(html.Class("small"), gomponents.Text(f.Location)),
			html.Td(html.Class("small"), gomponents.Text(f.Format)),
			html.Td(html.Class("small"), gomponents.Text(bitrate)),
		))
	}

	var (
		class   string
		verdict gomponents.Node
	)

	switch {
	case !u.Allowed:
		class = "alert alert-danger py-2 mb-2"
		verdict = gomponents.Group([]gomponents.Node{
			html.Strong(gomponents.Text("Processing will stop: ")),
			gomponents.Text(fmt.Sprintf(
				"the library already has files of equal or better quality for this release "+
					"(quality priority %d for the new files vs %d, %d vs %d files). "+
					"Remove or fix the files below first, or choose another candidate.",
				u.NewPriority, u.OldPriority, u.NewFiles, len(u.Existing))),
		})
	case u.OldPriority == 0:
		class = "alert alert-secondary py-2 mb-2"
		verdict = gomponents.Text("The library already has files of this release. " +
			"No quality priority could be compared, so they do not block processing.")
	default:
		class = "alert alert-info py-2 mb-2"
		verdict = gomponents.Text(fmt.Sprintf(
			"The library already has files of this release; the new files count as an upgrade "+
				"(quality priority %d vs %d, %d vs %d files).",
			u.NewPriority, u.OldPriority, u.NewFiles, len(u.Existing)))
	}

	return html.Div(
		html.Class(class),
		html.Div(verdict),
		html.Table(
			html.Class("table table-sm mb-0 mt-1"),
			html.THead(html.Tr(
				html.Th(gomponents.Text("In the library")), html.Th(gomponents.Text("Format")),
				html.Th(gomponents.Text("Bitrate")),
			)),
			html.TBody(gomponents.Group(files)),
		),
	)
}

// ── Formatting helpers ───────────────────────────────────────────────────────

// mtScoreBadge shows a distance as beets-style similarity, colored by the
// recommendation thresholds.
func mtScoreBadge(dist, strong, medium float64) gomponents.Node {
	class := "bg-danger"

	switch {
	case dist < strong:
		class = "bg-success"
	case dist <= medium:
		class = "bg-warning text-dark"
	}

	sim := math.Max(0, 1-dist) * 100

	return html.Span(
		html.Class("badge "+class),
		html.Title(fmt.Sprintf("distance %.3f (strong < %.2f, medium ≤ %.2f)", dist, strong, medium)),
		gomponents.Text(fmt.Sprintf("%.1f%%", sim)),
	)
}

// mtDiffText shows wanted, highlighted when it differs from have.
func mtDiffText(wanted, have string) gomponents.Node {
	if wanted == "" {
		return gomponents.Text("")
	}

	if strings.EqualFold(strings.TrimSpace(wanted), strings.TrimSpace(have)) {
		return gomponents.Text(wanted)
	}

	return html.Span(html.Class("text-danger fw-semibold"), html.Title("was: "+have), gomponents.Text(wanted))
}

func mtCandidateSource(c *importfeed.PreviewCandidate) string {
	if c.DBID != 0 {
		return "database #" + strconv.FormatUint(uint64(c.DBID), 10) + mtIf(c.Source != importfeed.PreviewSourceDatabase, " ("+c.Source+")")
	}

	return c.Source + " (new)"
}

// mtIssues lists what differs, like beets' "(missing tracks, album, …)".
func mtIssues(c *importfeed.PreviewCandidate, p *importfeed.PreviewEntry) string {
	var issues []string

	if c.Missing > 0 {
		issues = append(issues, fmt.Sprintf("%d missing", c.Missing))
	}

	if c.Extra > 0 {
		issues = append(issues, fmt.Sprintf("%d extra", c.Extra))
	}

	if !strings.EqualFold(c.Title, p.Album) {
		issues = append(issues, "album")
	}

	if !strings.EqualFold(c.Artist, p.Artist) {
		issues = append(issues, "artist")
	}

	if p.Year > 0 && c.Year > 0 && p.Year != c.Year {
		issues = append(issues, "year")
	}

	changed := 0
	for _, r := range c.Rows {
		if !r.Missing && !r.Extra && !strings.EqualFold(r.LocalTitle, r.WantedTitle) {
			changed++
		}
	}

	if changed > 0 {
		issues = append(issues, fmt.Sprintf("%d titles", changed))
	}

	return strings.Join(issues, ", ")
}

func mtTrackPos(disc, track int, show bool) string {
	if !show || track == 0 {
		return ""
	}

	if disc > 1 {
		return fmt.Sprintf("%d-%02d", disc, track)
	}

	return strconv.Itoa(track)
}

func mtDuration(ms int64) string {
	if ms <= 0 {
		return ""
	}

	d := time.Duration(ms) * time.Millisecond
	if d >= time.Hour {
		return fmt.Sprintf("%d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}

	return fmt.Sprintf("%d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

func mtIf(cond bool, s string) string {
	if cond {
		return s
	}

	return ""
}

func coalesce(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

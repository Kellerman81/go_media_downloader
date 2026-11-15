package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// wizardFolder is one folder entered on the Paths step.
type wizardFolder struct {
	group, kind, path string // kind: "library" or "import"
}

// wizardCheckFolders checks the Paths step's folders before anything is
// saved: each must be a writable folder (missing ones are created when
// create is set), and no two may be the same or nested in each other. A
// library inside an import folder would be organized into itself; two
// groups sharing a folder would take each other's files.
func wizardCheckFolders(folders []wizardFolder, create bool) []string {
	var problems []string

	var used []wizardFolder

	for _, f := range folders {
		if f.path == "" {
			continue
		}

		label := f.group + " " + f.kind + " folder " + f.path

		info, err := os.Stat(f.path)

		switch {
		case os.IsNotExist(err) && create:
			if err := os.MkdirAll(f.path, 0o755); err != nil {
				problems = append(problems, label+": can't be created ("+err.Error()+")")
				continue
			}
		case os.IsNotExist(err):
			problems = append(problems, label+" doesn't exist (tick \"Create folders\" or create it)")
			continue
		case err != nil:
			problems = append(problems, label+": "+err.Error())
			continue
		case !info.IsDir():
			problems = append(problems, label+" is a file, not a folder")
			continue
		}

		if !checkWritable(f.path) {
			problems = append(problems, label+" is not writable for this app")
		}

		clean := filepath.Clean(f.path)
		for _, u := range used {
			other := filepath.Clean(u.path)
			switch {
			case other == clean:
				problems = append(problems, fmt.Sprintf("%s is also the %s %s folder", label, u.group, u.kind))
			case wizardInside(clean, other) || wizardInside(other, clean):
				problems = append(problems, fmt.Sprintf("%s and the %s %s folder %s are inside each other", label, u.group, u.kind, u.path))
			}
		}

		used = append(used, f)
	}

	return problems
}

// wizardInside reports whether path is below dir.
func wizardInside(path, dir string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != "." && !strings.HasPrefix(rel, "..")
}

// wizardClientDir asks a download client for the folder it puts finished
// downloads of category in (as the client sees it).
func wizardClientDir(ctx context.Context, dl *config.DownloaderConfig, category string) (string, error) {
	client, err := providers.NewDownloadClient(dl)
	if err != nil {
		return "", err
	}

	d, ok := client.(interface {
		DownloadDir(ctx context.Context, category string) (string, error)
	})
	if !ok {
		return "", fmt.Errorf("%s can't report its download folder; check it in the client", dl.DlType)
	}

	return d.DownloadDir(ctx, category)
}

// wizardDirVerdict compares the client's folder with the group's import
// folder (both empty-able): the organizer only finds downloads that land in
// the import folder. The client may run elsewhere (another container) and
// name the same folder differently; that can't be seen from here, so it's a
// warning, not an error.
func wizardDirVerdict(clientDir, importDir string) (msg, level string) {
	_, err := os.Stat(clientDir)
	visible := err == nil

	if importDir == "" {
		if visible {
			return "This app sees the folder too.", "success"
		}

		return "This app can't see that folder. If the client runs in another container, map its download folder into this one and use the path it has here as a group's Download / Import path.", "warning"
	}

	c, i := filepath.Clean(clientDir), filepath.Clean(importDir)

	switch {
	case c == i || wizardInside(c, i):
		return "Matches the import folder " + importDir + ".", "success"
	case visible:
		return "Downloads land in " + clientDir + ", but the group's import folder is " + importDir + ": the organizer won't find them. Set the client's category folder or the import path so they match.", "danger"
	}

	return "The client names the folder " + clientDir + ", which this app can't see. That's fine when it is the same folder as " + importDir + " mounted under another path (separate containers); otherwise downloads are never organized.", "warning"
}

// HandleWizardCheckDownloadFolders asks the download client where each
// group's downloads land and compares that with the group's import folder
// (/admin/wizard/test/dlfolders, from the Finalize step).
func HandleWizardCheckDownloadFolders(c *gin.Context) {
	_ = c.Request.ParseForm()
	state := readWizardState(c)

	c.Header("Content-Type", "text/html")

	dl := config.GetSettingsDownloader(state.Downloader)
	if dl == nil {
		c.String(http.StatusOK, renderAlert("No download client was set up in this run (step "+fmt.Sprint(wizStepDownloader)+").", "warning"))
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), wizardTestTimeout)
	defer cancel()

	rows := make([]gomponents.Node, 0, len(wizardMediaGroupDefs))

	for _, d := range wizardEnabledGroups(state) {
		g := state.Groups[d.key]
		category := strings.TrimSpace(c.PostForm("wizard_rec_" + d.key + "_dlcat"))

		importDir := ""
		if p := config.GetSettingsPath(g.ImportPath); p != nil {
			importDir = p.Path
		}

		var result gomponents.Node

		dir, err := wizardClientDir(ctx, dl, category)
		if err != nil {
			result = gomponents.Raw(renderAlert(err.Error(), "warning"))
		} else {
			msg, level := wizardDirVerdict(dir, importDir)
			result = html.Div(html.Code(gomponents.Text(dir)), gomponents.Raw(renderAlert(msg, level)))
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(g.Name)), html.Div(html.Class("small text-muted"), gomponents.Text("category: "+category))),
			html.Td(result)))
	}

	c.String(http.StatusOK, renderComponentToString(html.Table(html.Class("table table-sm align-middle"),
		html.THead(html.Tr(html.Th(gomponents.Text("Group")), html.Th(gomponents.Text("Where "+dl.Name+" puts finished downloads")))),
		html.TBody(gomponents.Group(rows)))))
}

// wizardFinishJobs are the jobs the Finish page offers per group.
var wizardFinishJobs = []struct{ job, label, hint string }{
	{"data", "Scan library", "Reads the files already in the library folder"},
	{"feeds", "Import lists", "Adds the wanted entries of the group's lists"},
	{"searchmissinginc", "Search missing", "Searches the indexers for a first batch of missing entries"},
}

// HandleWizardRunJob starts a job from the Finish page
// (/admin/wizard/run): a group job, or the IMDb import.
func HandleWizardRunJob(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	job, cfgName := c.PostForm("job"), c.PostForm("config")

	status := func(msg, level string) {
		c.String(http.StatusOK, renderComponentToString(html.Span(html.Class("small text-"+level), gomponents.Text(msg))))
	}

	if job == "imdb" {
		status(performFillIMDB(c))
		return
	}

	cfgp := config.GetSettingsMedia(cfgName)
	if cfgp == nil || !slices.Contains(jobTypesFor(cfgp), job) {
		status("Unknown job "+job+" for "+cfgName, "warning")
		return
	}

	err := worker.Dispatch(job+"_"+cfgName, func(key uint32, ctx context.Context) error {
		// force: an explicit click runs even while the scheduler is off.
		return utils.SingleJobs(ctx, job, cfgName, "", true, key)
	}, "Data")
	if err != nil {
		status("Failed to start: "+err.Error(), "danger")
		return
	}

	status("✓ Started (Management → Queue)", "success")
}

// renderWizardNextSteps is the Finish page's "first runs" section.
func renderWizardNextSteps(state wizardState, csrfToken string) gomponents.Node {
	button := func(label, job, cfgName, hint string) gomponents.Node {
		return html.Div(html.Class("d-inline-flex align-items-center gap-2 me-3 mb-2"),
			html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-success"), html.Title(hint),
				hx.Post("/api/admin/wizard/run"), hx.Vals(fmt.Sprintf(`{"job": %s, "config": %s}`, jsString(job), jsString(cfgName))),
				hx.Headers(wizardCSRFHeader(csrfToken)), hx.Target("next span"), hx.Swap("innerHTML"),
				html.I(html.Class("fa-solid fa-play me-1")), gomponents.Text(label)),
			html.Span(html.Class("small")))
	}

	var rows []gomponents.Node

	video := false

	for _, d := range wizardEnabledGroups(state) {
		cfgName := wizardMediaConfigName(d, state.Groups[d.key].Name)

		cfgp := config.GetSettingsMedia(cfgName)
		if cfgp == nil {
			continue
		}

		video = video || d.qualityKind == "video"
		jobs := jobTypesFor(cfgp)

		buttons := make([]gomponents.Node, 0, len(wizardFinishJobs))
		for _, j := range wizardFinishJobs {
			if slices.Contains(jobs, j.job) {
				buttons = append(buttons, button(j.label, j.job, cfgName, j.hint))
			}
		}

		rows = append(rows, html.Li(html.Class("list-group-item"),
			html.Div(html.Class("fw-semibold mb-1"), gomponents.Text(state.Groups[d.key].Name)),
			gomponents.Group(buttons)))
	}

	if len(rows) == 0 {
		return nil
	}

	var imdb gomponents.Node
	if video {
		imdb = html.Div(html.Class("mt-2"),
			html.P(html.Class("small text-muted mb-1"), gomponents.Text(
				"Movies and series are matched with a local IMDb database. Building it downloads the IMDb datasets and takes 10 to 30 minutes; the schedule refreshes it later.")),
			button("Build IMDb database", "imdb", "", "Runs init_imdb"))
	}

	return html.Div(
		html.H6(html.Class("mt-3"), gomponents.Text("First runs")),
		html.P(html.Class("small text-muted mb-2"), gomponents.Text(
			"The schedule starts these on its own; run them now to see results right away. Start with Scan library when the library folder already has files.")),
		html.Ul(html.Class("list-group mb-2"), gomponents.Group(rows)),
		imdb)
}

// wizardMediaConfigName is the internal name of a group (movie_EN, ...).
func wizardMediaConfigName(d wizardMediaGroupDef, name string) string {
	return map[string]string{"movies": "movie_", "series": "serie_", "music": "music_", "audiobooks": "audiobook_", "books": "book_"}[d.key] + name
}

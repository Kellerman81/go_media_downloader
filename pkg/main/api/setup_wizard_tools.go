package api

// setup_wizard_tools.go is the wizard's System Tools step: it finds the
// external programs the app calls (the same way the app resolves them), shows
// their version and what they are needed for given the chosen media groups,
// and saves changed paths and the player's hardware acceleration.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// wizardTool is an external program the app runs.
type wizardTool struct {
	key     string // form field suffix
	label   string
	bin     string // program name
	folder  bool   // the setting is the program's folder (ffprobe, mediainfo)
	fileOK  bool   // a folder setting may also be the program itself (ffprobe)
	derived bool   // no setting of its own (ffmpeg: next to ffprobe, else PATH)
	// fixed: no setting, always this path relative to the working folder
	// (init_imdb). Only checked for existence, never run: running it
	// starts the IMDb import.
	fixed string
	args  []string
	pkg   string // Debian/Ubuntu package
	// need says how much the chosen media kinds need it: "required",
	// "recommended", "optional" or "" (not used by them), and why.
	need func(kinds map[string]bool) (string, string)
	get  func(*config.GeneralConfig) string
	set  func(*config.GeneralConfig, string)
}

// wizardMediaKinds are the kinds (video, music, audiobook, book) of the
// chosen groups; all of them when none were chosen.
func wizardMediaKinds(state wizardState) map[string]bool {
	kinds := map[string]bool{}
	for _, d := range wizardEnabledGroups(state) {
		kinds[d.qualityKind] = true
	}

	if len(kinds) == 0 {
		for _, d := range wizardMediaGroupDefs {
			kinds[d.qualityKind] = true
		}
	}

	return kinds
}

func wizardAnyKind(kinds map[string]bool, want ...string) bool {
	for _, w := range want {
		if kinds[w] {
			return true
		}
	}

	return false
}

var wizardTools = []wizardTool{
	{
		key: "ffprobe", label: "ffprobe", bin: "ffprobe", folder: true, fileOK: true, args: []string{"-version"}, pkg: "ffmpeg",
		need: func(k map[string]bool) (string, string) {
			if wizardAnyKind(k, "video", "music", "audiobook") {
				return "required", "Reads runtime, resolution, codecs and languages of downloads; music tags; the player"
			}

			return "", "Not used for books"
		},
		get: func(g *config.GeneralConfig) string { return g.FfprobePath },
		set: func(g *config.GeneralConfig, v string) { g.FfprobePath = v },
	},
	{
		key: "ffmpeg", label: "ffmpeg", bin: "ffmpeg", derived: true, args: []string{"-version"}, pkg: "ffmpeg",
		need: func(k map[string]bool) (string, string) {
			if wizardAnyKind(k, "video", "music", "audiobook") {
				return "recommended", "Converts video and audio for the in-browser player; covers; MP4/M4B tags. Found next to ffprobe, else on the PATH"
			}

			return "", "Not used for books"
		},
	},
	{
		key: "7zip", label: "7-Zip", bin: "7z", args: nil, pkg: "7zip",
		need: func(map[string]bool) (string, string) {
			return "recommended", "Unpacks downloaded archives; handles every format (.rar, .zip, .7z, .tar) when a tool below is missing"
		},
		get: func(g *config.GeneralConfig) string { return g.SevenZipPath },
		set: func(g *config.GeneralConfig, v string) { g.SevenZipPath = v },
	},
	{
		key: "unrar", label: "unrar", bin: "unrar", pkg: "unrar (non-free)",
		need: func(map[string]bool) (string, string) {
			return "optional", "Unpacks .rar archives; 7-Zip is used when it is missing"
		},
		get: func(g *config.GeneralConfig) string { return g.UnrarPath },
		set: func(g *config.GeneralConfig, v string) { g.UnrarPath = v },
	},
	{
		key: "unzip", label: "unzip", bin: "unzip", args: []string{"-v"}, pkg: "unzip",
		need: func(map[string]bool) (string, string) {
			return "optional", "Unpacks .zip archives; 7-Zip is used when it is missing"
		},
		get: func(g *config.GeneralConfig) string { return g.UnzipPath },
		set: func(g *config.GeneralConfig, v string) { g.UnzipPath = v },
	},
	{
		key: "tar", label: "tar", bin: "tar", args: []string{"--version"}, pkg: "tar",
		need: func(map[string]bool) (string, string) {
			return "optional", "Unpacks .tar, .tar.gz, .tar.xz archives; 7-Zip is used when it is missing"
		},
		get: func(g *config.GeneralConfig) string { return g.TarPath },
		set: func(g *config.GeneralConfig, v string) { g.TarPath = v },
	},
	{
		key: "mediainfo", label: "MediaInfo", bin: "mediainfo", folder: true, args: []string{"--Version"}, pkg: "mediainfo",
		need: func(k map[string]bool) (string, string) {
			if wizardAnyKind(k, "video") {
				return "optional", "Alternative to ffprobe for video files (Settings → General → Use MediaInfo)"
			}

			return "", "Only used for video"
		},
		get: func(g *config.GeneralConfig) string { return g.MediainfoPath },
		set: func(g *config.GeneralConfig, v string) { g.MediainfoPath = v },
	},
	{
		key: "initimdb", label: "init_imdb", bin: "init_imdb", fixed: wizardInitImdbPath(), pkg: "init_imdb from the release archive",
		need: func(k map[string]bool) (string, string) {
			if k["video"] {
				return "recommended", "Builds the local IMDb database (titles, years, ratings) the movie and series matching uses; the IMDb job runs it from the program's folder"
			}

			return "", "Only used for movies and series"
		},
	},
	{
		key: "metaflac", label: "metaflac", bin: "metaflac", args: []string{"--version"}, pkg: "flac",
		need: func(k map[string]bool) (string, string) {
			if k["music"] {
				return "optional", "Writes tags into FLAC files (reading works without it)"
			}

			return "", "Only used for music"
		},
		get: func(g *config.GeneralConfig) string { return g.MetaflacPath },
		set: func(g *config.GeneralConfig, v string) { g.MetaflacPath = v },
	},
	{
		key: "fpcalc", label: "fpcalc (Chromaprint)", bin: "fpcalc", args: []string{"-version"}, pkg: "libchromaprint-tools",
		need: func(k map[string]bool) (string, string) {
			if k["music"] {
				return "optional", "Audio fingerprints to identify tracks via AcoustID (needs an AcoustID key in the next steps)"
			}

			return "", "Only used for music"
		},
		get: func(g *config.GeneralConfig) string { return g.FpcalcPath },
		set: func(g *config.GeneralConfig, v string) { g.FpcalcPath = v },
	},
}

// wizardToolResult is what the check found for one tool.
type wizardToolResult struct {
	tool    wizardTool
	setting string // the configured (or entered) value
	path    string // the program found, "" if missing
	version string
	hint    string // a better setting the check found, e.g. 7za for 7z
}

func wizardExe(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}

	return name
}

// wizardFindTool resolves a tool like the app does: a folder setting joined
// with the program name, else the setting itself (a path or a name) or the
// default name on the PATH. ffmpeg is looked for next to ffprobe first.
func wizardFindTool(t wizardTool, setting, ffprobeFolder string) string {
	try := func(p string) string {
		if p == "" {
			return ""
		}

		if found, err := exec.LookPath(p); err == nil {
			return found
		}

		return ""
	}

	switch {
	case t.fixed != "":
		if st, err := os.Stat(t.fixed); err == nil && !st.IsDir() {
			if abs, err := filepath.Abs(t.fixed); err == nil {
				return abs
			}

			return t.fixed
		}

		return ""
	case t.derived:
		if ffprobeFolder != "" {
			if p := try(filepath.Join(ffprobeFolder, wizardExe(t.bin))); p != "" {
				return p
			}
		}

		return try(t.bin)
	case t.folder && setting != "":
		if st, err := os.Stat(setting); t.fileOK && err == nil && !st.IsDir() {
			return try(setting)
		}

		return try(filepath.Join(setting, wizardExe(t.bin)))
	case setting != "":
		return try(setting)
	}

	return try(t.bin)
}

// wizardToolVersion is the first line a program prints about itself.
func wizardToolVersion(ctx context.Context, path string, args []string) string {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Some print their banner with a non-zero exit (unrar without arguments).
	out, _ := exec.CommandContext(ctx, path, args...).CombinedOutput()
	for _, line := range strings.Split(string(out), "\n") {
		if line = strings.TrimSpace(line); line != "" {
			if len(line) > 90 {
				line = line[:90] + "…"
			}

			return line
		}
	}

	return ""
}

// wizardCheckTools checks every tool with the given settings (form values
// over the saved ones).
func wizardCheckTools(ctx context.Context, values map[string]string) []wizardToolResult {
	g := config.GetSettingsGeneral()
	if g == nil {
		g = &config.GeneralConfig{}
	}

	setting := func(t wizardTool) string {
		if v, ok := values[t.key]; ok {
			return strings.TrimSpace(v)
		}

		if t.get != nil {
			return t.get(g)
		}

		return ""
	}

	var ffprobeFolder string

	for _, t := range wizardTools {
		if t.key == "ffprobe" {
			ffprobeFolder = setting(t)
		}
	}

	out := make([]wizardToolResult, 0, len(wizardTools))
	for _, t := range wizardTools {
		r := wizardToolResult{tool: t, setting: setting(t)}

		r.path = wizardFindTool(t, r.setting, ffprobeFolder)
		if r.path != "" && t.fixed == "" {
			r.version = wizardToolVersion(ctx, r.path, t.args)
		}

		// 7-Zip installs as 7za or 7zz on some systems.
		if r.path == "" && t.key == "7zip" && r.setting == "" {
			for _, alt := range []string{"7za", "7zz"} {
				if p, err := exec.LookPath(alt); err == nil {
					r.hint = p
					break
				}
			}
		}

		if r.path == "" && t.folder && !t.fileOK && r.setting != "" {
			if st, err := os.Stat(r.setting); err == nil && !st.IsDir() {
				r.hint = filepath.Dir(r.setting)
			}
		}

		out = append(out, r)
	}

	return out
}

// renderWizardToolTable is the result table; values are the form's entries
// (nil: the saved settings).
func renderWizardToolTable(c context.Context, state wizardState, values map[string]string) gomponents.Node {
	kinds := wizardMediaKinds(state)
	results := wizardCheckTools(c, values)

	var missing []string

	rows := make([]gomponents.Node, 0, len(results))
	for _, r := range results {
		need, why := r.tool.need(kinds)

		badge := map[string]string{"required": "bg-danger", "recommended": "bg-warning text-dark", "optional": "bg-secondary"}[need]
		label := map[string]string{"required": "Required", "recommended": "Recommended", "optional": "Optional", "": "Not needed"}[need]

		var status gomponents.Node

		if r.path != "" {
			status = html.Div(html.Span(html.Class("badge bg-success me-1"), gomponents.Text("Found")),
				html.Code(html.Class("small"), gomponents.Text(r.path)),
				gomponents.If(r.version != "", html.Div(html.Class("small text-muted"), gomponents.Text(r.version))))
		} else {
			if need == "required" || need == "recommended" {
				missing = append(missing, r.tool.label)
			}

			status = html.Div(html.Span(html.Class("badge bg-danger me-1"), gomponents.Text("Missing")),
				html.Span(html.Class("small text-muted"), gomponents.Text("install package "+r.tool.pkg)),
				gomponents.If(r.hint != "", html.Div(html.Class("small"),
					gomponents.Text("Found "), html.Code(gomponents.Text(r.hint)), gomponents.Text(" — enter it in the field to use it."))))
		}

		var field gomponents.Node = html.Span(html.Class("small text-muted"), gomponents.Text("next to ffprobe, else PATH"))
		if r.tool.fixed != "" {
			field = html.Span(html.Class("small text-muted"), gomponents.Text("in the program's folder: "+r.tool.fixed))
		} else if !r.tool.derived {
			placeholder := r.tool.bin + " (on the PATH)"
			switch {
			case r.tool.fileOK:
				placeholder = "folder or path of " + r.tool.bin + " (empty: PATH)"
			case r.tool.folder:
				placeholder = "folder of " + r.tool.bin + " (empty: PATH)"
			}

			field = html.Input(html.Type("text"), html.Class("form-control form-control-sm"), html.Name("wizard_tool_"+r.tool.key),
				html.Value(r.setting), html.Placeholder(placeholder))
		}

		rows = append(rows, html.Tr(
			html.Td(html.Strong(gomponents.Text(r.tool.label)), html.Div(html.Span(html.Class("badge "+badge), gomponents.Text(label)))),
			html.Td(html.Class("small"), gomponents.Text(why)),
			html.Td(status),
			html.Td(html.Style("min-width:14rem"), field),
		))
	}

	summary := renderAlert("All required and recommended tools were found.", "success")
	if len(missing) > 0 {
		summary = renderAlert("Missing: "+strings.Join(missing, ", ")+". Install them (the Docker image has them all) or enter where they are, then Check again.", "warning")
	}

	return html.Div(html.ID("wizardToolTable"),
		gomponents.Raw(summary),
		html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle"),
			html.THead(html.Tr(html.Th(gomponents.Text("Tool")), html.Th(gomponents.Text("Needed for")),
				html.Th(gomponents.Text("Status")), html.Th(gomponents.Text("Path")))),
			html.TBody(gomponents.Group(rows)))))
}

// wizardRenderDevices are the GPU render nodes for the player's hardware
// acceleration (VA-API, Quick Sync).
func wizardRenderDevices() []string {
	devs, _ := filepath.Glob("/dev/dri/renderD*")
	return devs
}

func renderWizardStepTools(csrfToken string, state wizardState, values map[string]string) gomponents.Node {
	g := config.GetSettingsGeneral()
	if g == nil {
		g = &config.GeneralConfig{}
	}

	hw, dev := g.PlayerHWAccel, g.PlayerHWDevice
	if hw == "" {
		hw = "none"
	}

	devs := wizardRenderDevices()
	if dev == "" && len(devs) > 0 {
		dev = devs[0]
	}

	devNote := "No GPU render device (/dev/dri/renderD*) was found: keep \"none\" (CPU), or pass the device into the container."
	if len(devs) > 0 {
		devNote = "GPU render device found: " + strings.Join(devs, ", ") + ". VA-API suits AMD and Intel, Quick Sync Intel only. A failing GPU falls back to the CPU."
	}

	opt := func(v, label string) gomponents.Node {
		return html.Option(html.Value(v), gomponents.If(v == hw, html.Selected()), gomponents.Text(label))
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-toolbox", fmt.Sprintf("Step %d: System Tools", wizStepTools),
			"External programs the downloader runs. Required ones must be there for downloads to be checked and organized; the others add features."),
		html.Form(
			html.Class("config-form"),
			html.ID("wizardStepForm"),
			renderWizardToolTable(context.Background(), state, values),
			gomponents.If(wizardAnyKind(wizardMediaKinds(state), "video"), html.Div(html.Class("row g-2 mt-2"),
				html.H6(html.Class("mb-0"), gomponents.Text("Player: hardware acceleration")),
				html.P(html.Class("small text-muted mb-1"), gomponents.Text(devNote)),
				html.Div(html.Class("col-md-4"),
					html.Label(html.Class("form-label small"), html.For("wizardHW"), gomponents.Text("Video conversion")),
					html.Select(html.Class("form-select form-select-sm"), html.ID("wizardHW"), html.Name("wizard_player_hwaccel"),
						opt("none", "none (CPU)"), opt("vaapi", "VA-API (AMD, Intel)"), opt("qsv", "Quick Sync (Intel)"))),
				html.Div(html.Class("col-md-4"),
					html.Label(html.Class("form-label small"), html.For("wizardHWDev"), gomponents.Text("GPU device")),
					html.Input(html.Type("text"), html.Class("form-control form-control-sm"), html.ID("wizardHWDev"),
						html.Name("wizard_player_device"), html.Value(dev), html.Placeholder("/dev/dri/renderD128"))),
			)),
			html.Div(
				html.Class("form-group submit-group mt-3"),
				html.Button(html.Class("btn btn-outline-info me-2"), html.Type("button"), gomponents.Text("Check again"),
					hx.Post("/api/admin/wizard/test/tools"), hx.Target("#wizardToolTable"), hx.Swap("outerHTML"),
					hx.Include("#wizardStepForm, #wizardCarry"), hx.Headers(wizardCSRFHeader(csrfToken))),
				wizardSaveButton(wizStepTools, csrfToken),
				wizardNextButton(wizStepTools, "Skip"),
			),
		),
	)
}

// wizardToolValues are the tool paths of the form.
func wizardToolValues(c *gin.Context) map[string]string {
	values := map[string]string{}
	for _, t := range wizardTools {
		if t.derived {
			continue
		}

		if v, ok := c.GetPostForm("wizard_tool_" + t.key); ok {
			values[t.key] = strings.TrimSpace(v)
		}
	}

	return values
}

// HandleWizardCheckTools checks the tools with the paths entered, without
// saving (/admin/wizard/test/tools).
func HandleWizardCheckTools(c *gin.Context) {
	_ = c.Request.ParseForm()

	c.Header("Content-Type", "text/html")
	c.String(http.StatusOK, renderComponentToString(renderWizardToolTable(c.Request.Context(), readWizardState(c), wizardToolValues(c))))
}

func wizardSaveToolsStep(c *gin.Context) string {
	g := config.GetSettingsGeneral()
	if g == nil {
		return renderAlert("Configuration not loaded.", "danger")
	}

	updated := *g

	for key, v := range wizardToolValues(c) {
		for _, t := range wizardTools {
			if t.key == key && t.set != nil {
				t.set(&updated, v)
			}
		}
	}

	if hw, ok := c.GetPostForm("wizard_player_hwaccel"); ok {
		if hw != "vaapi" && hw != "qsv" {
			hw = "none"
		}

		updated.PlayerHWAccel = hw
		updated.PlayerHWDevice = strings.TrimSpace(c.PostForm("wizard_player_device"))
	}

	if err := config.UpdateCfgEntryAny(&updated); err != nil {
		return renderAlert("Failed to save the tool settings: "+err.Error(), "danger")
	}

	return renderAlert("Tool settings saved.", "success")
}

// wizardInitImdbPath is where the IMDb job looks for init_imdb (as
// parser.getImdbFilename).
func wizardInitImdbPath() string {
	return map[bool]string{true: "init_imdb.exe", false: "./init_imdb"}[runtime.GOOS == "windows"]
}

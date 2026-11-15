package api

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/goccy/go-json"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"github.com/pelletier/go-toml/v2"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// renderSeriesConfigPage renders the series configuration file editor page.
func renderSeriesConfigPage(ctx *gin.Context) {
	files, err := getSeriesConfigFiles()
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error listing config files: "+err.Error())
		return
	}

	selectedFile := ctx.Query("file")
	if selectedFile == "" && len(files) > 0 {
		selectedFile = files[0].Name
	}

	var seriesConfig *config.MainManualConfig
	if selectedFile != "" {
		seriesConfig, err = loadSeriesConfigFile(selectedFile)
		if err != nil {
			sendJSONError(ctx, http.StatusInternalServerError, "Error loading config file: "+err.Error())
			return
		}
	}

	pageNode := page(
		"Series Configuration",
		true,
		false,
		false,
		renderSeriesConfigContent(files, selectedFile, seriesConfig, getCSRFToken(ctx)),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// seriesConfigFile is a manual config file of the config directory.
type seriesConfigFile struct {
	Name  string
	Kind  string   // series, audiobook, music or book (from the list type)
	Count int      // entries
	Lists []string // lists that import it
}

// manualConfigKinds maps the list types that import a manual config file to
// the kind of entries in it.
var manualConfigKinds = map[string]string{
	"seriesconfig":    "series",
	"audiobookconfig": "audiobook",
	"musicconfig":     "music",
	"bookconfig":      "book",
}

// manualKindMarker is the first line of a file created on the page. It keeps
// the kind of a file that no list imports yet, and lists the file while it
// has no entries.
const manualKindMarker = "# manual config kind: "

var manualKindMarkerRe = regexp.MustCompile(`(?m)^# manual config kind: ([a-z]+)\s*$`)

// markedKind returns the kind a file's marker line names, or "".
func markedKind(raw []byte) string {
	if m := manualKindMarkerRe.FindSubmatch(raw); m != nil {
		if _, ok := manualKindInfo[string(m[1])]; ok {
			return string(m[1])
		}
	}

	return ""
}

// listTypeFor returns the list type that imports files of a kind.
func listTypeFor(kind string) string {
	for listType, k := range manualConfigKinds {
		if k == kind {
			return listType
		}
	}

	return ""
}

// getSeriesConfigFiles returns the manual config files (series, audiobooks,
// music, books): the ones a list imports first, then other *.toml files with
// [[config]] entries or a kind marker (manualKindMarker); without a marker
// they are taken as series files. Files without entries (e.g. copies of
// config.toml) are left out.
func getSeriesConfigFiles() ([]seriesConfigFile, error) {
	used := map[string][]string{}
	kinds := map[string]string{}

	config.RangeSettingsList(func(_ string, l *config.ListsConfig) {
		kind, ok := manualConfigKinds[strings.ToLower(l.ListType)]
		if l.ManualConfigFile == "" || !ok {
			return
		}

		name := filepath.Base(l.ManualConfigFile)
		used[name] = append(used[name], l.Name)
		kinds[name] = kind
	})

	entries, err := os.ReadDir(config.GetConfigDir())
	if err != nil {
		return nil, err
	}

	var files []seriesConfigFile

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(name), ".toml") || strings.EqualFold(name, "config.toml") {
			continue
		}

		path, err := seriesConfigPath(name)
		if err != nil {
			continue
		}

		// Cheap check before decoding: copies of config.toml are large and
		// have no series entries.
		raw, err := os.ReadFile(path)
		marked := markedKind(raw)

		if err != nil || (used[name] == nil && marked == "" && !strings.Contains(string(raw), "[[config]]")) {
			continue
		}

		cfg, err := loadSeriesConfigFile(name)
		if err != nil || (used[name] == nil && marked == "" && len(cfg.Config) == 0) {
			continue
		}

		slices.Sort(used[name])
		files = append(files, seriesConfigFile{
			Name: name, Kind: cmp.Or(kinds[name], marked, "series"), Count: len(cfg.Config), Lists: used[name],
		})
	}

	slices.SortStableFunc(files, func(a, b seriesConfigFile) int {
		if (len(a.Lists) > 0) != (len(b.Lists) > 0) {
			if len(a.Lists) > 0 {
				return -1
			}

			return 1
		}

		return strings.Compare(a.Name, b.Name)
	})

	return files, nil
}

// errInvalidSeriesConfigFile is returned for a series config file name that is
// not a plain *.toml file name inside the config directory.
var errInvalidSeriesConfigFile = errors.New("invalid series config file name")

// seriesConfigPath resolves a user-supplied series config file name to a path
// inside the config directory. Only bare "*.toml" names (no directories, not
// the main config.toml) are accepted.
func seriesConfigPath(filename string) (string, error) {
	if filename == "" || filename != filepath.Base(filename) ||
		strings.ContainsAny(filename, `/\`) ||
		!strings.EqualFold(filepath.Ext(filename), ".toml") ||
		strings.EqualFold(filename, "config.toml") {
		return "", fmt.Errorf("%w: %q", errInvalidSeriesConfigFile, filename)
	}

	configDir := filepath.Clean(config.GetConfigDir())
	filePath := filepath.Join(configDir, filename)

	if filepath.Dir(filePath) != configDir {
		return "", fmt.Errorf("%w: %q", errInvalidSeriesConfigFile, filename)
	}

	return filePath, nil
}

// loadSeriesConfigFile loads a MainSerieConfig from the specified file.
func loadSeriesConfigFile(filename string) (*config.MainManualConfig, error) {
	filePath, err := seriesConfigPath(filename)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	return config.DecodeManualConfig(data)
}

// saveSeriesConfigFile saves a MainSerieConfig to the specified file.
func saveSeriesConfigFile(filename string, cfg *config.MainManualConfig) error {
	filePath, err := seriesConfigPath(filename)
	if err != nil {
		return err
	}

	// Keep other top-level sections and unknown [global] keys as they are;
	// the known [global] keys come from cfg.Global.
	other := map[string]any{}
	marker := ""

	if raw, err := os.ReadFile(filePath); err == nil {
		if kind := markedKind(raw); kind != "" {
			marker = manualKindMarker + kind + "\n"
		}

		if err := toml.Unmarshal(raw, &other); err != nil {
			return fmt.Errorf("read %s: %w", filename, err)
		}

		delete(other, "config")
	}

	global, _ := other["global"].(map[string]any)
	if global == nil {
		global = map[string]any{}
	}

	setOrDelete := func(key string, value any, set bool) {
		if set {
			global[key] = value
		} else {
			delete(global, key)
		}
	}

	setOrDelete("identifiedby", cfg.Global.Identifiedby, cfg.Global.Identifiedby != "")
	setOrDelete("search", ptrValue(cfg.Global.Search), cfg.Global.Search != nil)
	setOrDelete("upgrade", ptrValue(cfg.Global.Upgrade), cfg.Global.Upgrade != nil)

	if len(global) > 0 {
		other["global"] = global
	} else {
		delete(other, "global")
	}

	data, err := marshalSeriesConfig(cfg, other)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, append([]byte(marker), data...), 0o644)
}

// manualConfigFileSuggestions returns the manual config files as paths for
// a list's manual_config_file.
func manualConfigFileSuggestions() []string {
	files, err := getSeriesConfigFiles()
	if err != nil {
		return nil
	}

	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, "./config/"+f.Name)
	}

	return paths
}

// handleSeriesConfigCreate creates an empty manual config file of a kind.
func handleSeriesConfigCreate(ctx *gin.Context) {
	name := strings.TrimSpace(ctx.PostForm("name"))
	if !strings.EqualFold(filepath.Ext(name), ".toml") {
		name += ".toml"
	}

	kind := ctx.PostForm("kind")
	if _, ok := manualKindInfo[kind]; !ok {
		sendBadRequest(ctx, "unknown kind "+kind)
		return
	}

	path, err := seriesConfigPath(name)
	if err != nil {
		sendBadRequest(ctx, "Use a plain file name like series_new.toml (not config.toml, no folders)")
		return
	}

	// O_EXCL: never overwrite an existing file.
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			sendBadRequest(ctx, name+" already exists")
		} else {
			sendJSONError(ctx, http.StatusInternalServerError, "Error creating file: "+err.Error())
		}

		return
	}

	_, err = f.WriteString(manualKindMarker + kind + "\n")
	if cerr := f.Close(); err == nil {
		err = cerr
	}

	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error writing file: "+err.Error())
		return
	}

	ctx.Redirect(http.StatusSeeOther, "/api/admin/seriesconfig?file="+url.QueryEscape(name))
}

// renderSeriesNewFileForm renders the form that creates a manual config file.
func renderSeriesNewFileForm(csrfToken string) gomponents.Node {
	kinds := make([]gomponents.Node, 0, len(manualKindInfo))
	for _, k := range []string{"series", "audiobook", "music", "book"} {
		kinds = append(kinds, html.Option(html.Value(k), gomponents.Text(manualKindInfo[k].Type+" ("+listTypeFor(k)+" list)")))
	}

	return html.Div(
		html.Class("collapse mt-3"), html.ID("scNewFile"),
		html.Form(
			html.Class("row g-2 align-items-end border-top pt-3"),
			html.Method("POST"),
			html.Action("/api/admin/seriesconfig/create"),
			html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
			html.Div(html.Class("col-sm-5"),
				html.Label(html.Class("form-label small mb-1"), html.For("scNewName"), gomponents.Text("File name")),
				html.Div(html.Class("input-group input-group-sm"),
					html.Input(html.Class("form-control"), html.ID("scNewName"), html.Name("name"), html.Required(),
						html.Pattern(`[A-Za-z0-9._\-]+`), html.Placeholder("series_kids")),
					html.Span(html.Class("input-group-text"), gomponents.Text(".toml")),
				),
			),
			html.Div(html.Class("col-sm-4"),
				html.Label(html.Class("form-label small mb-1"), html.For("scNewKind"), gomponents.Text("Contains")),
				html.Select(html.Class("form-select form-select-sm"), html.ID("scNewKind"), html.Name("kind"), gomponents.Group(kinds)),
			),
			html.Div(html.Class("col-sm-3"),
				html.Button(html.Type("submit"), html.Class("btn btn-sm btn-primary w-100"),
					html.I(html.Class("fa-solid fa-file-circle-plus me-1")), gomponents.Text("Create file")),
			),
		),
	)
}

// renderSeriesImportHint tells how to import a file that no list imports yet.
func renderSeriesImportHint(f seriesConfigFile) gomponents.Node {
	return html.Div(
		html.Class("alert alert-info d-flex align-items-start gap-2 p-3 mb-4"),
		html.I(html.Class("fa-solid fa-circle-info mt-1")),
		html.Div(
			html.Strong(gomponents.Text("No list imports this file yet. ")),
			gomponents.Text("Add a list of type "),
			html.Code(gomponents.Text(listTypeFor(f.Kind))),
			gomponents.Text(" with Manual Config File Path "),
			html.Code(gomponents.Text("./config/"+f.Name)),
			gomponents.Text(" (or pick the file there), and add the list to a "+manualKindInfo[f.Kind].Type+" media group. "),
			html.A(html.Href("/api/admin/config/lists"), gomponents.Text("Open the lists config")),
		),
	)
}

func ptrValue(b *bool) bool { return b != nil && *b }

// handleSeriesConfigGlobal saves a file's defaults ([global]).
func handleSeriesConfigGlobal(ctx *gin.Context) {
	filename := ctx.PostForm("file")

	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	onOff := func(v string) *bool {
		switch v {
		case "on":
			return new(true)
		case "off":
			return new(false)
		}

		return nil
	}

	identifiedby := ctx.PostForm("identifiedby")
	if identifiedby != "" && identifiedby != "ep" && identifiedby != "date" && identifiedby != "title" {
		sendBadRequest(ctx, "identifiedby must be ep, date, title or empty")
		return
	}

	// Fields a series doesn't set only hold the old defaults; clear them so
	// the save doesn't write the old default into every series.
	for i := range cfg.Config {
		e := &cfg.Config[i]
		if !e.Explicit.Has("identifiedby") {
			e.Identifiedby = ""
		}

		if !e.Explicit.Has("dont_search") {
			e.DontSearch = false
		}

		if !e.Explicit.Has("dont_upgrade") {
			e.DontUpgrade = false
		}
	}

	cfg.Global = config.ManualGlobalConfig{
		Identifiedby: identifiedby,
		Search:       onOff(ctx.PostForm("search")),
		Upgrade:      onOff(ctx.PostForm("upgrade")),
	}

	if err := saveSeriesConfigFile(filename, cfg); err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error saving config: "+err.Error())
		return
	}

	ctx.Redirect(http.StatusSeeOther, "/api/admin/seriesconfig?file="+url.QueryEscape(filename))
}

// renderSeriesGlobalForm renders the file defaults ([global]) form.
func renderSeriesGlobalForm(filename, kind string, g *config.ManualGlobalConfig, csrfToken string) gomponents.Node {
	selectField := func(name, label, current string, options [][2]string) gomponents.Node {
		opts := []gomponents.Node{html.Option(html.Value(""), gomponents.If(current == "", html.Selected()), gomponents.Text("— not set —"))}
		for _, o := range options {
			opts = append(opts, html.Option(html.Value(o[0]), gomponents.If(current == o[0], html.Selected()), gomponents.Text(o[1])))
		}

		return html.Div(html.Class("col-sm-6 col-lg-3"),
			html.Label(html.Class("form-label small mb-1"), html.For("global-"+name), gomponents.Text(label)),
			html.Select(html.Class("form-select form-select-sm"), html.ID("global-"+name), html.Name(name), gomponents.Group(opts)),
		)
	}

	onOff := func(b *bool) string {
		switch {
		case b == nil:
			return ""
		case *b:
			return "on"
		default:
			return "off"
		}
	}

	return html.Form(
		html.Class("px-3 py-3 border-bottom bg-light"),
		html.Method("POST"),
		html.Action("/api/admin/seriesconfig/global"),
		html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
		html.Input(html.Type("hidden"), html.Name("file"), html.Value(filename)),
		html.Div(html.Class("small fw-semibold mb-2"),
			gomponents.Text("File defaults"),
			html.Span(html.Class("text-muted fw-normal ms-2"),
				gomponents.Text("([global]: used by every entry of this file that doesn't set the value itself)")),
		),
		html.Div(
			html.Class("row g-2 align-items-end"),
			gomponents.If(kind == "series" || g.Identifiedby != "",
				selectField("identifiedby", "Identified by", g.Identifiedby, [][2]string{{"ep", "ep (S01E01)"}, {"date", "date (daily shows)"}, {"title", "title (episode title in the release name)"}})),
			selectField("search", "Search", onOff(g.Search), [][2]string{{"on", "on"}, {"off", "off (dont_search)"}}),
			selectField("upgrade", "Upgrades", onOff(g.Upgrade), [][2]string{{"on", "on"}, {"off", "off (dont_upgrade)"}}),
			html.Div(html.Class("col-sm-6 col-lg-3"),
				html.Button(html.Type("submit"), html.Class("btn btn-sm btn-outline-primary w-100"),
					html.I(html.Class("fa-solid fa-save me-1")), gomponents.Text("Save defaults")),
			),
		),
	)
}

// marshalSeriesConfig writes a series config file compactly: per series only
// the fields that are set, in field order, without comments - the format the
// files are written in by hand. A full struct marshal writes all ~50 fields
// with their help text for every series (16 KB -> 1.3 MB for 248 series).
// ManualConfig holds only scalars and string lists, so each field is one line.
// Fields the entry set in the file (Explicit) are kept even when empty.
func marshalSeriesConfig(cfg *config.MainManualConfig, other map[string]any) ([]byte, error) {
	var b bytes.Buffer

	if len(other) > 0 {
		head, err := toml.Marshal(other)
		if err != nil {
			return nil, err
		}

		b.Write(head)
	}

	for i := range cfg.Config {
		b.WriteString("[[config]]\n")

		v := reflect.ValueOf(cfg.Config[i])
		t := v.Type()

		for f := range t.NumField() {
			field := t.Field(f)

			key, _, _ := strings.Cut(field.Tag.Get("toml"), ",")
			if !field.IsExported() || key == "" || key == "-" {
				continue
			}

			value := v.Field(f)
			empty := value.IsZero() || (value.Kind() == reflect.Slice && value.Len() == 0)

			// Unset fields stay unset, also those DecodeManualConfig filled
			// from [global], so the default keeps applying; explicit ones are
			// kept, also when false or empty.
			if !cfg.Config[i].Explicit.Has(key) {
				if def, ok := cfg.Global.DefaultFor(key); empty || (ok && value.Interface() == def) {
					continue
				}
			}

			line, err := toml.Marshal(map[string]any{key: value.Interface()})
			if err != nil {
				return nil, fmt.Errorf("series %q, %s: %w", cfg.Config[i].Name, key, err)
			}

			b.Write(line)
		}
	}

	return b.Bytes(), nil
}

// manualKindInfo describes a kind of manual config file for the page.
var manualKindInfo = map[string]struct{ Icon, Noun, Type string }{
	"series":    {"fa-tv", "series", "series"},
	"audiobook": {"fa-headphones", "entries", "audiobooks"},
	"music":     {"fa-music", "entries", "music"},
	"book":      {"fa-book", "entries", "books"},
}

// renderSeriesConfigContent renders the file selection and the entries of
// the selected file.
func renderSeriesConfigContent(
	files []seriesConfigFile,
	selectedFile string,
	seriesConfig *config.MainManualConfig,
	csrfToken string,
) gomponents.Node {
	var selected seriesConfigFile

	pills := make([]gomponents.Node, 0, len(files))
	for _, f := range files {
		if f.Name == selectedFile {
			selected = f
		}

		class := "nav-link d-flex flex-column align-items-start py-2"
		if f.Name == selectedFile {
			class += " active"
		}

		usedBy := "not imported by a list"
		if len(f.Lists) > 0 {
			usedBy = "list " + strings.Join(f.Lists, ", ") + " · " + manualKindInfo[f.Kind].Type
		}

		pills = append(pills, html.Li(html.Class("nav-item"),
			html.A(html.Class(class), html.Href("/api/admin/seriesconfig?file="+url.QueryEscape(f.Name)),
				html.Span(html.Class("fw-semibold"), html.I(html.Class("fa-solid "+manualKindInfo[f.Kind].Icon+" me-1")), gomponents.Text(f.Name),
					html.Span(html.Class("badge bg-secondary ms-2"), gomponents.Text(strconv.Itoa(f.Count)))),
				html.Small(html.Class("opacity-75"), gomponents.Text(usedBy)),
			),
		))
	}

	var fileSection gomponents.Node = html.P(html.Class("text-muted mb-0"), gomponents.Text(
		"No config files found. Create one, then import it with a list of type seriesconfig, audiobookconfig, musicconfig or bookconfig (manual_config_file)."))
	if len(pills) > 0 {
		fileSection = html.Ul(html.Class("nav nav-pills gap-2"), gomponents.Group(pills))
	}

	return html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader(
			"fa-solid fa-tv",
			"Series Configuration",
			"The config files your seriesconfig, audiobookconfig, musicconfig and bookconfig lists import: which series, authors, artists and books are tracked and how they are searched and organized.",
		),
		html.Div(
			html.Class("card border-0 shadow-sm mb-4"),
			html.Div(html.Class("card-body"),
				html.Div(html.Class("d-flex align-items-center mb-3"),
					html.H6(html.Class("text-muted text-uppercase small mb-0 me-auto"), gomponents.Text("Configuration file")),
					html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-primary"),
						gomponents.Attr("data-bs-toggle", "collapse"), gomponents.Attr("data-bs-target", "#scNewFile"),
						gomponents.Attr("aria-expanded", "false"), gomponents.Attr("aria-controls", "scNewFile"),
						html.I(html.Class("fa-solid fa-file-circle-plus me-1")), gomponents.Text("New file")),
				),
				fileSection,
				renderSeriesNewFileForm(csrfToken),
			),
		),
		gomponents.If(selected.Name != "" && len(selected.Lists) == 0, renderSeriesImportHint(selected)),
		gomponents.If(selectedFile != "" && seriesConfig != nil,
			renderSeriesConfigEditor(selectedFile, fileKind(files, selectedFile), seriesConfig, csrfToken),
		),
	)
}

// fileKind returns the kind of a listed file; series for an unlisted one.
func fileKind(files []seriesConfigFile, name string) string {
	for _, f := range files {
		if f.Name == name {
			return f.Kind
		}
	}

	return "series"
}

// renderSeriesConfigEditor renders the entries of a file with search and actions.
func renderSeriesConfigEditor(
	filename, kind string,
	cfg *config.MainManualConfig,
	csrfToken string,
) gomponents.Node {
	if cfg == nil {
		return html.Div(html.Class("alert alert-warning"), gomponents.Text("Error: Could not load configuration file."))
	}

	info := manualKindInfo[kind]

	var body gomponents.Node = html.P(html.Class("text-muted text-center py-4 mb-0"), gomponents.Text("This file has no entries yet."))
	if len(cfg.Config) > 0 {
		body = renderSeriesList(filename, kind, cfg.Config, csrfToken)
	}

	addLabel := "Add Series"
	if kind != "series" {
		addLabel = "Add Entry"
	}

	return html.Div(
		html.Class("card border-0 shadow-sm"),
		html.Div(
			html.Class("card-header bg-transparent d-flex flex-wrap align-items-center gap-2"),
			html.H5(html.Class("mb-0 me-auto"),
				html.I(html.Class("fa-solid "+info.Icon+" me-2 text-primary")),
				gomponents.Text(filename),
				html.Span(html.Class("text-muted fw-normal fs-6 ms-2"), gomponents.Textf("%d %s", len(cfg.Config), info.Noun)),
			),
			gomponents.If(len(cfg.Config) > 0, html.Input(
				html.Type("search"), html.Class("form-control form-control-sm"), html.Style("max-width: 260px;"),
				html.ID("scSearch"), html.Placeholder("Filter…"), gomponents.Attr("aria-label", "Filter entries"),
				gomponents.Attr("oninput", `var q=this.value.toLowerCase();document.querySelectorAll('#scTable tbody tr').forEach(function(r){r.classList.toggle('d-none',q&&r.dataset.name.indexOf(q)<0);});`),
			)),
			html.A(
				html.Class("btn btn-primary btn-sm"),
				html.Href("/api/admin/seriesconfig/add?file="+url.QueryEscape(filename)),
				html.I(html.Class("fa-solid fa-plus me-1")),
				gomponents.Text(addLabel),
			),
		),
		html.Div(html.Class("card-body p-0"), renderSeriesGlobalForm(filename, kind, &cfg.Global, csrfToken), body),
	)
}

// renderSeriesList renders the entries of a file as a table.
func renderSeriesList(
	filename, kind string,
	series []config.ManualConfig,
	csrfToken string,
) gomponents.Node {
	return html.Div(
		html.Class("table-responsive"),
		html.Table(
			html.ID("scTable"),
			html.Class("table table-hover align-middle mb-0"),
			html.THead(html.Class("table-light"),
				html.Tr(
					html.Th(html.Class("ps-3"), gomponents.Text(map[string]string{
						"series": "Series", "audiobook": "Name", "music": "Name", "book": "Name",
					}[kind])),
					html.Th(gomponents.Text(map[string]string{
						"series": "TVDB ID", "audiobook": "Author / series", "music": "Artist / album series", "book": "Author / series",
					}[kind])),
					html.Th(gomponents.Text("Source")),
					gomponents.If(kind == "series", html.Th(gomponents.Text("Identified by"))),
					html.Th(gomponents.Text("Search")),
					html.Th(html.Class("text-end pe-3"), gomponents.Text("Actions")),
				),
			),
			html.TBody(renderSeriesRows(filename, kind, series, csrfToken)),
		),
	)
}

// seriesDefault shows a series setting: its own value, else the file default
// ([global], already applied by DecodeManualConfig) or the built-in default,
// marked as such.
func seriesDefault(s *config.ManualConfig, key, value, def string) gomponents.Node {
	if s.Explicit.Has(key) && value != "" {
		return gomponents.Text(value)
	}

	note := "default"
	if value != "" {
		note = "file default"
	}

	return gomponents.Group([]gomponents.Node{
		gomponents.Text(cmp.Or(value, def)),
		html.Small(html.Class("text-muted ms-1"), gomponents.Text(note)),
	})
}

// renderSeriesRows renders table rows for each series.
func renderSeriesRows(
	filename, kind string,
	series []config.ManualConfig,
	csrfToken string,
) gomponents.Node {
	rows := make([]gomponents.Node, 0, len(series))
	for i, s := range series {
		search := html.Span(html.Class("badge bg-success"), gomponents.Text("On"))
		if s.DontSearch {
			search = html.Span(html.Class("badge bg-secondary"), gomponents.Text("Off"))
		}

		tvdb := gomponents.Node(html.Span(html.Class("text-muted"), gomponents.Text("–")))
		if who := cmp.Or(s.AuthorName, s.BookSeriesName); kind == "audiobook" || kind == "book" {
			tvdb = gomponents.Text(cmp.Or(who, "–"))
		} else if who := cmp.Or(s.ArtistName, s.AlbumSeriesName); kind == "music" {
			tvdb = gomponents.Text(cmp.Or(who, "–"))
		} else if s.TvdbID > 0 {
			tvdb = html.A(html.Href(fmt.Sprintf("https://thetvdb.com/dereferrer/series/%d", s.TvdbID)),
				html.Target("_blank"), html.Rel("noopener"), gomponents.Text(strconv.Itoa(s.TvdbID)))
		}

		editURL := fmt.Sprintf("/api/admin/seriesconfig/edit?file=%s&index=%d", url.QueryEscape(filename), i)

		rows = append(rows, html.Tr(
			html.Data("name", strings.ToLower(strings.Join(append([]string{s.Name, s.AuthorName, s.ArtistName,
				s.BookSeriesName, s.AlbumSeriesName}, s.AlternateName...), " "))),
			html.Td(html.Class("ps-3"),
				html.A(html.Class("fw-semibold text-body text-decoration-none"), html.Href(editURL), gomponents.Text(s.Name)),
				gomponents.If(len(s.AlternateName) > 0, html.Div(html.Class("small text-muted text-truncate"), html.Style("max-width: 420px;"),
					gomponents.Text(strings.Join(s.AlternateName, ", ")))),
				gomponents.If(s.DontUpgrade, html.Span(html.Class("badge bg-light text-dark border ms-0 mt-1"), gomponents.Text("no upgrades"))),
			),
			html.Td(tvdb),
			html.Td(seriesDefault(&series[i], "source", s.Source, "tvdb")),
			gomponents.If(kind == "series", html.Td(seriesDefault(&series[i], "identifiedby", s.Identifiedby, "ep"))),
			html.Td(search),
			html.Td(html.Class("text-end pe-3 text-nowrap"),
				html.A(html.Class("btn btn-sm btn-outline-primary me-1"), html.Href(editURL), html.Title("Edit"),
					gomponents.Attr("aria-label", "Edit "+s.Name), html.I(html.Class("fa-solid fa-pen"))),
				html.Form(
					html.Class("d-inline"),
					html.Method("POST"),
					html.Action("/api/admin/seriesconfig/delete"),
					html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
					html.Input(html.Type("hidden"), html.Name("file"), html.Value(filename)),
					html.Input(html.Type("hidden"), html.Name("index"), html.Value(strconv.Itoa(i))),
					html.Button(
						html.Type("button"),
						html.Class("btn btn-sm btn-outline-danger"),
						html.Title("Delete"),
						gomponents.Attr("aria-label", "Delete "+s.Name),
						gomponents.Attr(
							"onclick",
							"var f=this.form; confirmAction('Delete series', 'Remove "+escapeJSString(s.Name)+" from "+escapeJSString(filename)+"? This cannot be undone.', function(){ f.submit(); });",
						),
						html.I(html.Class("fa-solid fa-trash")),
					),
				),
			),
		))
	}

	return gomponents.Group(rows)
}

// handleSeriesConfigDelete handles deletion of a series entry.
func handleSeriesConfigDelete(ctx *gin.Context) {
	filename := ctx.PostForm("file")
	index := ctx.PostForm("index")

	if filename == "" || index == "" {
		sendBadRequest(ctx, "Missing file or index parameter")
		return
	}

	// Parse index
	var idx int
	if _, err := fmt.Sscanf(index, "%d", &idx); err != nil {
		sendBadRequest(ctx, "Invalid index: "+err.Error())
		return
	}

	// Load config
	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// Validate index
	if idx < 0 || idx >= len(cfg.Config) {
		sendBadRequest(ctx, "Index out of range")
		return
	}

	// Remove entry
	cfg.Config = append(cfg.Config[:idx], cfg.Config[idx+1:]...)

	// Save
	if err := saveSeriesConfigFile(filename, cfg); err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error saving config: "+err.Error())
		return
	}

	// Redirect back
	ctx.Redirect(http.StatusSeeOther, "/api/admin/seriesconfig?file="+url.QueryEscape(filename))
}

// handleSeriesConfigEdit handles the edit form page.
func handleSeriesConfigEdit(ctx *gin.Context) {
	filename := ctx.Query("file")
	index := ctx.Query("index")

	if filename == "" || index == "" {
		sendBadRequest(ctx, "Missing file or index parameter")
		return
	}

	// Parse index
	var idx int
	if _, err := fmt.Sscanf(index, "%d", &idx); err != nil {
		sendBadRequest(ctx, "Invalid index: "+err.Error())
		return
	}

	// Load config
	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// Validate index
	if idx < 0 || idx >= len(cfg.Config) {
		sendBadRequest(ctx, "Index out of range")
		return
	}

	csrfToken := getCSRFToken(ctx)

	pageNode := page(
		"Edit Series Configuration",
		true,
		false,
		false,
		renderSeriesConfigForm(filename, &cfg.Config[idx], idx, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// handleSeriesConfigAdd handles the add form page.
func handleSeriesConfigAdd(ctx *gin.Context) {
	filename := ctx.Query("file")

	if filename == "" {
		sendBadRequest(ctx, "Missing file parameter")
		return
	}

	csrfToken := getCSRFToken(ctx)

	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// A new series starts with the file's defaults ([global]).
	emptySerie := &config.ManualConfig{
		Identifiedby: cmp.Or(cfg.Global.Identifiedby, "ep"),
		Source:       "tvdb",
	}
	if cfg.Global.Upgrade != nil {
		emptySerie.DontUpgrade = !*cfg.Global.Upgrade
	}

	if cfg.Global.Search != nil {
		emptySerie.DontSearch = !*cfg.Global.Search
	}

	pageNode := page(
		"Add Series Configuration",
		true,
		false,
		false,
		renderSeriesConfigForm(filename, emptySerie, -1, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// renderSeriesConfigForm renders the add/edit form for a series configuration.
func renderSeriesConfigForm(
	filename string,
	serie *config.ManualConfig,
	index int,
	csrfToken string,
) gomponents.Node {
	isEdit := index >= 0

	title := "Add New Series"
	if isEdit {
		title = "Edit Series: " + serie.Name
	}

	// Get field metadata
	comments, displayNames := getSerieConfigMetadata()
	group := "serie"

	backURL := "/api/admin/seriesconfig?file=" + url.QueryEscape(filename)

	return html.Div(
		html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-tv", title, "Series config file "+filename),
		html.A(html.Class("btn btn-link px-0 mb-3"), html.Href(backURL),
			html.I(html.Class("fa-solid fa-arrow-left me-1")), gomponents.Text("Back to "+filename)),
		html.Form(
			html.Class("config-form"),
			html.Method("POST"),
			html.Action("/api/admin/seriesconfig/update"),
			html.Input(html.Type("hidden"), html.Name("csrf_token"), html.Value(csrfToken)),
			html.Input(html.Type("hidden"), html.Name("file"), html.Value(filename)),
			gomponents.If(isEdit, html.Input(html.Type("hidden"), html.Name("index"), html.Value(strconv.Itoa(index)))),
			renderSeriesConfigSections(serie, group, comments, displayNames),
			html.Div(
				html.Class("form-group submit-group-enhanced mt-4 p-4 rounded-3 d-flex flex-wrap gap-2 justify-content-center"),
				html.Style("background: #f8f9fa; border: 1px solid #dee2e6;"),
				html.Button(
					html.Class("btn btn-primary"),
					html.Type("submit"),
					html.I(html.Class("fa-solid fa-save me-2")),
					gomponents.Text("Save Series"),
				),
				html.A(
					html.Class("btn btn-outline-info"),
					html.Href(fmt.Sprintf("/api/admin/seriesconfig/helper?file=%s&index=%d", url.QueryEscape(filename), index)),
					html.I(html.Class("fa-solid fa-wand-magic-sparkles me-2")),
					gomponents.Text("Scraper Helper"),
				),
				html.A(html.Class("btn btn-outline-secondary"), html.Href(backURL), gomponents.Text("Cancel")),
			),
		),
	)
}

// getSerieConfigMetadata extracts field comments and display names from SerieConfig struct tags.
func getSerieConfigMetadata() (map[string]string, map[string]string) {
	comments := logger.GetFieldComments(config.ManualConfig{})
	displayNames := logger.GetFieldDisplayNames(config.ManualConfig{})
	return comments, displayNames
}

// renderSeriesConfigSections organizes series config fields into logical groups.
func renderSeriesConfigSections(
	serie *config.ManualConfig,
	group string,
	comments map[string]string,
	displayNames map[string]string,
) gomponents.Node {
	sanitizedName := strings.ReplaceAll(strings.ReplaceAll(serie.Name, " ", "-"), "_", "-")
	if sanitizedName == "" {
		sanitizedName = "new-series"
	}

	accordionId := "serieConfigAccordion-" + sanitizedName

	return html.Div(
		html.Class("accordion"),
		html.ID(accordionId),

		// Basic Settings
		renderConfigGroupWithParent("Basic Settings", "basic-serie-"+sanitizedName, true,
			[]FormFieldDefinition{
				{Name: "Name", Type: "text", Value: serie.Name, Options: nil},
				{Name: "TvdbID", Type: "number", Value: serie.TvdbID, Options: nil},
				{Name: "AlternateName", Type: "array", Value: serie.AlternateName, Options: nil},
				{Name: "DisallowedName", Type: "array", Value: serie.DisallowedName, Options: nil},
				// Unset means "ep" and "tvdb" to the importer; show those instead
				// of letting the browser preselect the first option, which a
				// save would then write.
				{
					Name:  "Identifiedby",
					Type:  "select",
					Value: cmp.Or(serie.Identifiedby, "ep"),
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"ep", "date", "title"},
					}),
				},
				{
					Name:  "Source",
					Type:  "select",
					Value: cmp.Or(serie.Source, "tvdb"),
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"tvdb", "scraper", "none"},
					}),
				},
				{Name: "Target", Type: "text", Value: serie.Target, Options: nil},
			}, group, comments, displayNames, accordionId),

		// Search & Upgrade Settings
		renderConfigGroupWithParent(
			"Search & Upgrade Settings",
			"search-serie-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "DontUpgrade", Type: "checkbox", Value: serie.DontUpgrade, Options: nil},
				{Name: "DontSearch", Type: "checkbox", Value: serie.DontSearch, Options: nil},
				{
					Name:    "SearchSpecials",
					Type:    "checkbox",
					Value:   serie.SearchSpecials,
					Options: nil,
				},
				{Name: "IgnoreRuntime", Type: "checkbox", Value: serie.IgnoreRuntime, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Scraper Settings (when source = "scraper")
		renderConfigGroupWithParent(
			"Scraper - Basic Configuration",
			"scraper-basic-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{
					Name:  "ScraperType",
					Type:  "select",
					Value: serie.ScraperType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"project1service", "algolia", "htmlxpath", "csrfapi"},
					}),
				},
				{Name: "StartURL", Type: "text", Value: serie.StartURL, Options: nil},
				{Name: "SiteURL", Type: "text", Value: serie.SiteURL, Options: nil},
				{Name: "SiteID", Type: "number", Value: serie.SiteID, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Project1Service / Algolia Settings
		renderConfigGroupWithParent(
			"Scraper - Filter Settings",
			"scraper-filters-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{
					Name:    "FilterCollectionID",
					Type:    "number",
					Value:   serie.FilterCollectionID,
					Options: nil,
				},
				{Name: "SiteFilterName", Type: "text", Value: serie.SiteFilterName, Options: nil},
				{Name: "SerieFilterName", Type: "text", Value: serie.SerieFilterName, Options: nil},
				{
					Name:    "NetworkFilterName",
					Type:    "text",
					Value:   serie.NetworkFilterName,
					Options: nil,
				},
				{
					Name:    "NetworkSiteFilterName",
					Type:    "text",
					Value:   serie.NetworkSiteFilterName,
					Options: nil,
				},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// HTML/XPath Scraper Settings
		renderConfigGroupWithParent(
			"Scraper - HTML/XPath Settings",
			"scraper-xpath-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "SceneNodeXPath", Type: "text", Value: serie.SceneNodeXPath, Options: nil},
				{Name: "TitleXPath", Type: "text", Value: serie.TitleXPath, Options: nil},
				{Name: "URLXPath", Type: "text", Value: serie.URLXPath, Options: nil},
				{Name: "DateXPath", Type: "text", Value: serie.DateXPath, Options: nil},
				{Name: "ActorsXPath", Type: "text", Value: serie.ActorsXPath, Options: nil},
				{Name: "PlotXPath", Type: "text", Value: serie.PlotXPath, Options: nil},
				{Name: "ScenePlotXPath", Type: "text", Value: serie.ScenePlotXPath, Options: nil},
				{Name: "TitleAttribute", Type: "text", Value: serie.TitleAttribute, Options: nil},
				{Name: "URLAttribute", Type: "text", Value: serie.URLAttribute, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Pagination Settings
		renderConfigGroupWithParent(
			"Scraper - Pagination Settings",
			"scraper-pagination-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{
					Name:  "PaginationType",
					Type:  "select",
					Value: serie.PaginationType,
					Options: convertMapToSelectOptions(map[string][]string{
						"options": {"sequential", "offset"},
					}),
				},
				{Name: "PageIncrement", Type: "number", Value: serie.PageIncrement, Options: nil},
				{Name: "PageURLPattern", Type: "text", Value: serie.PageURLPattern, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// CSRF API Scraper Settings
		renderConfigGroupWithParent(
			"Scraper - CSRF API Settings",
			"scraper-csrfapi-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "CSRFCookieName", Type: "text", Value: serie.CSRFCookieName, Options: nil},
				{Name: "CSRFHeaderName", Type: "text", Value: serie.CSRFHeaderName, Options: nil},
				{Name: "APIURLPattern", Type: "text", Value: serie.APIURLPattern, Options: nil},
				{Name: "PageStartIndex", Type: "number", Value: serie.PageStartIndex, Options: nil},
				{
					Name:    "ResultsArrayPath",
					Type:    "text",
					Value:   serie.ResultsArrayPath,
					Options: nil,
				},
				{Name: "TitleField", Type: "text", Value: serie.TitleField, Options: nil},
				{Name: "DateField", Type: "text", Value: serie.DateField, Options: nil},
				{Name: "URLField", Type: "text", Value: serie.URLField, Options: nil},
				{Name: "ActorsField", Type: "text", Value: serie.ActorsField, Options: nil},
				{Name: "ActorNameField", Type: "text", Value: serie.ActorNameField, Options: nil},
				{Name: "RuntimeField", Type: "text", Value: serie.RuntimeField, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Common Scraper Settings
		renderConfigGroupWithParent(
			"Scraper - Common Settings",
			"scraper-common-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "DateFormat", Type: "text", Value: serie.DateFormat, Options: nil},
				{Name: "WaitSeconds", Type: "number", Value: serie.WaitSeconds, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Audiobook Configuration
		renderConfigGroupWithParent(
			"Audiobook Configuration",
			"audiobook-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "AuthorName", Type: "text", Value: serie.AuthorName, Options: nil},
				{Name: "AuthorID", Type: "text", Value: serie.AuthorID, Options: nil},
				{Name: "BookSeriesName", Type: "text", Value: serie.BookSeriesName, Options: nil},
				{Name: "BookSeriesID", Type: "text", Value: serie.BookSeriesID, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),

		// Music Configuration
		renderConfigGroupWithParent(
			"Music Configuration",
			"music-"+sanitizedName,
			false,
			[]FormFieldDefinition{
				{Name: "ArtistName", Type: "text", Value: serie.ArtistName, Options: nil},
				{Name: "ArtistID", Type: "text", Value: serie.ArtistID, Options: nil},
				{Name: "AlbumSeriesName", Type: "text", Value: serie.AlbumSeriesName, Options: nil},
				{Name: "AlbumSeriesID", Type: "text", Value: serie.AlbumSeriesID, Options: nil},
				{Name: "MBMediaFormats", Type: "array", Value: serie.MBMediaFormats, Options: nil},
			},
			group,
			comments,
			displayNames,
			accordionId,
		),
	)
}

// handleSeriesConfigUpdate handles form submission for add/edit.
func handleSeriesConfigUpdate(ctx *gin.Context) {
	filename := ctx.PostForm("file")
	indexStr := ctx.PostForm("index")

	if filename == "" {
		sendBadRequest(ctx, "Missing file parameter")
		return
	}

	// Load existing config
	cfg, err := loadSeriesConfigFile(filename)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error loading config: "+err.Error())
		return
	}

	// Parse form data into SerieConfig
	serie := parseSerieConfigForm(ctx)

	// Determine if this is an add or edit
	if indexStr != "" {
		// Edit existing entry
		var idx int
		if _, err := fmt.Sscanf(indexStr, "%d", &idx); err != nil {
			sendBadRequest(ctx, "Invalid index: "+err.Error())
			return
		}

		if idx < 0 || idx >= len(cfg.Config) {
			sendBadRequest(ctx, "Index out of range")
			return
		}

		serie.Explicit = cfg.Config[idx].Explicit

		markSeriesOverrides(&serie, &cfg.Global)
		cfg.Config[idx] = serie
	} else {
		// Add new entry
		markSeriesOverrides(&serie, &cfg.Global)
		cfg.Config = append(cfg.Config, serie)
	}

	// Save config
	if err := saveSeriesConfigFile(filename, cfg); err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, "Error saving config: "+err.Error())
		return
	}

	// Redirect back to main page
	ctx.Redirect(http.StatusFound, "/api/admin/seriesconfig?file="+filename)
}

// markSeriesOverrides marks the fields [global] defaults as explicit where the
// series' value differs from that default, so the save keeps the override; a
// value equal to the default stays unset and keeps following [global].
func markSeriesOverrides(serie *config.ManualConfig, global *config.ManualGlobalConfig) {
	values := map[string]any{
		"identifiedby": serie.Identifiedby,
		"dont_upgrade": serie.DontUpgrade,
		"dont_search":  serie.DontSearch,
	}

	for key, value := range values {
		if def, ok := global.DefaultFor(key); ok && def != value {
			serie.Explicit.Set(key)
		}
	}

	// The form shows unset source/identifiedby as their built-in defaults;
	// posting those back unchanged leaves the field unset.
	if !serie.Explicit.Has("source") && serie.Source == "tvdb" {
		serie.Source = ""
	}

	if _, ok := global.DefaultFor("identifiedby"); !ok && !serie.Explicit.Has("identifiedby") && serie.Identifiedby == "ep" {
		serie.Identifiedby = ""
	}
}

// parseSerieConfigForm parses form data into a SerieConfig struct.
func parseSerieConfigForm(ctx *gin.Context) config.ManualConfig {
	serie := config.ManualConfig{}

	// Basic fields
	serie.Name = ctx.PostForm("serie_Name")

	if tvdbID := ctx.PostForm("serie_TvdbID"); tvdbID != "" {
		fmt.Sscanf(tvdbID, "%d", &serie.TvdbID)
	}

	// Parse arrays (comma-separated values)
	if altNames := ctx.PostForm("serie_AlternateName"); altNames != "" {
		serie.AlternateName = parseArrayField(altNames)
	}

	if disallowed := ctx.PostForm("serie_DisallowedName"); disallowed != "" {
		serie.DisallowedName = parseArrayField(disallowed)
	}

	serie.Identifiedby = ctx.PostForm("serie_Identifiedby")
	serie.Source = ctx.PostForm("serie_Source")
	serie.Target = ctx.PostForm("serie_Target")

	// Checkboxes (only present if checked)
	serie.DontUpgrade = ctx.PostForm("serie_DontUpgrade") == "on"
	serie.DontSearch = ctx.PostForm("serie_DontSearch") == "on"
	serie.SearchSpecials = ctx.PostForm("serie_SearchSpecials") == "on"
	serie.IgnoreRuntime = ctx.PostForm("serie_IgnoreRuntime") == "on"

	// Scraper settings
	serie.ScraperType = ctx.PostForm("serie_ScraperType")
	serie.StartURL = ctx.PostForm("serie_StartURL")
	serie.SiteURL = ctx.PostForm("serie_SiteURL")

	if siteID := ctx.PostForm("serie_SiteID"); siteID != "" {
		var id uint
		fmt.Sscanf(siteID, "%d", &id)

		serie.SiteID = id
	}

	if filterID := ctx.PostForm("serie_FilterCollectionID"); filterID != "" {
		fmt.Sscanf(filterID, "%d", &serie.FilterCollectionID)
	}

	serie.SiteFilterName = ctx.PostForm("serie_SiteFilterName")
	serie.SerieFilterName = ctx.PostForm("serie_SerieFilterName")
	serie.NetworkFilterName = ctx.PostForm("serie_NetworkFilterName")
	serie.NetworkSiteFilterName = ctx.PostForm("serie_NetworkSiteFilterName")

	// XPath fields
	serie.SceneNodeXPath = ctx.PostForm("serie_SceneNodeXPath")
	serie.TitleXPath = ctx.PostForm("serie_TitleXPath")
	serie.URLXPath = ctx.PostForm("serie_URLXPath")
	serie.DateXPath = ctx.PostForm("serie_DateXPath")
	serie.ActorsXPath = ctx.PostForm("serie_ActorsXPath")
	serie.PlotXPath = ctx.PostForm("serie_PlotXPath")
	serie.ScenePlotXPath = ctx.PostForm("serie_ScenePlotXPath")
	serie.TitleAttribute = ctx.PostForm("serie_TitleAttribute")
	serie.URLAttribute = ctx.PostForm("serie_URLAttribute")

	// Pagination
	serie.PaginationType = ctx.PostForm("serie_PaginationType")
	if pageInc := ctx.PostForm("serie_PageIncrement"); pageInc != "" {
		fmt.Sscanf(pageInc, "%d", &serie.PageIncrement)
	}

	serie.PageURLPattern = ctx.PostForm("serie_PageURLPattern")

	// CSRF API fields
	serie.CSRFCookieName = ctx.PostForm("serie_CSRFCookieName")
	serie.CSRFHeaderName = ctx.PostForm("serie_CSRFHeaderName")
	serie.APIURLPattern = ctx.PostForm("serie_APIURLPattern")

	if startIdx := ctx.PostForm("serie_PageStartIndex"); startIdx != "" {
		fmt.Sscanf(startIdx, "%d", &serie.PageStartIndex)
	}

	serie.ResultsArrayPath = ctx.PostForm("serie_ResultsArrayPath")
	serie.TitleField = ctx.PostForm("serie_TitleField")
	serie.DateField = ctx.PostForm("serie_DateField")
	serie.URLField = ctx.PostForm("serie_URLField")
	serie.ActorsField = ctx.PostForm("serie_ActorsField")
	serie.ActorNameField = ctx.PostForm("serie_ActorNameField")
	serie.RuntimeField = ctx.PostForm("serie_RuntimeField")

	// Common settings
	serie.DateFormat = ctx.PostForm("serie_DateFormat")
	if waitSec := ctx.PostForm("serie_WaitSeconds"); waitSec != "" {
		fmt.Sscanf(waitSec, "%d", &serie.WaitSeconds)
	}

	// Audiobook fields
	serie.AuthorName = ctx.PostForm("serie_AuthorName")
	serie.AuthorID = ctx.PostForm("serie_AuthorID")
	serie.BookSeriesName = ctx.PostForm("serie_BookSeriesName")
	serie.BookSeriesID = ctx.PostForm("serie_BookSeriesID")

	// Music fields
	serie.ArtistName = ctx.PostForm("serie_ArtistName")
	serie.ArtistID = ctx.PostForm("serie_ArtistID")
	serie.AlbumSeriesName = ctx.PostForm("serie_AlbumSeriesName")

	serie.AlbumSeriesID = ctx.PostForm("serie_AlbumSeriesID")
	if val := ctx.PostFormArray("serie_MBMediaFormats"); len(val) != 0 {
		serie.MBMediaFormats = val
	}

	return serie
}

// parseArrayField parses a comma-separated string into a string slice.
func parseArrayField(value string) []string {
	if value == "" {
		return nil
	}

	// Split by comma and trim whitespace
	parts := strings.Split(value, ",")

	var result []string
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}

// handleScraperHelper handles the scraper helper/discovery page.
func handleScraperHelper(ctx *gin.Context) {
	filename := ctx.Query("file")
	index := ctx.Query("index")
	scraperType := ctx.Query("type")
	startURL := ctx.Query("url")

	if filename == "" || index == "" {
		sendBadRequest(ctx, "Missing file or index parameter")
		return
	}

	// Auto-detect scraper type if URL is provided but type is not
	if startURL != "" && scraperType == "" {
		scraperType = detectScraperType(startURL)
	}

	// Fetch data if URL is provided
	var discoveredData *ScraperDiscoveryData
	if startURL != "" && scraperType != "" {
		discoveredData = discoverScraperConfig(scraperType, startURL)
	}

	csrfToken := getCSRFToken(ctx)

	pageNode := page(
		"Scraper Helper",
		true,
		false,
		false,
		renderScraperHelperPage(filename, index, scraperType, startURL, discoveredData, csrfToken),
	)

	var buf strings.Builder
	pageNode.Render(&buf)

	ctx.Header("Content-Type", "text/html")
	ctx.String(http.StatusOK, buf.String())
}

// ScraperDiscoveryData holds discovered configuration values.
type ScraperDiscoveryData struct {
	Error              string
	AlgoliaAppID       string
	AlgoliaAPIKey      string
	AlgoliaIndexName   string
	AvailableSites     []FacetValue
	AvailableSeries    []FacetValue
	AvailableNetworks0 []FacetValue
	AvailableNetworks1 []FacetValue
	AlgoliaFacets      map[string][]FacetValue
	AlgoliaIndexFields []string
	Collections        []CollectionInfo
	CSRFCookie         string
	CSRFHeader         string
	SampleResponse     string
}

// FacetValue represents a facet value with count.
type FacetValue struct {
	Value string
	Count int
}

// CollectionInfo holds collection information.
type CollectionInfo struct {
	ID   int
	Name string
}

// detectScraperType automatically detects the scraper type by analyzing the URL and page content.
func detectScraperType(startURL string) string {
	client := &http.Client{
		Timeout: 15 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return nil // Allow redirects
		},
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, startURL, nil)
	if err != nil {
		return "" // Can't detect, let user choose
	}

	resp, err := client.Do(req)
	if err != nil {
		return "" // Can't detect, let user choose
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "" // Can't detect, let user choose
	}

	content := string(body)

	// Parse URL for domain-based detection
	parsedURL, _ := url.Parse(startURL)

	domain := ""
	if parsedURL != nil {
		domain = strings.ToLower(parsedURL.Host)
	}

	// Priority 1: Check for Algolia credentials in page
	// Algolia is used by Gamma Entertainment/Adult Time network
	algoliaPattern1 := regexp.MustCompile(`"applicationID":"([^"]+)","apiKey":"([^"]+)"`)

	algoliaPattern2 := regexp.MustCompile(`algoliaApplicationId:"([^"]+)",algoliaApiKey:"([^"]+)"`)
	if algoliaPattern1.MatchString(content) || algoliaPattern2.MatchString(content) {
		return "algolia"
	}

	// Priority 2: Check for Project1Service (Aylo/MindGeek network)
	// Look for instance_token cookie
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "instance_token" {
			return "project1service"
		}
	}

	// Priority 3: Check for CSRF API patterns
	// Look for CSRF cookies and check if page makes API calls
	hasCSRFCookie := false
	for _, cookie := range resp.Cookies() {
		cookieName := strings.ToLower(cookie.Name)
		if strings.Contains(cookieName, "csrf") || strings.Contains(cookieName, "xsrf") {
			hasCSRFCookie = true
			break
		}
	}

	// If has CSRF cookie and contains API endpoints in JavaScript, likely CSRF API
	if hasCSRFCookie {
		apiPatterns := []string{"/api/", "fetch(", "axios", "XMLHttpRequest"}
		for _, pattern := range apiPatterns {
			if strings.Contains(content, pattern) {
				return "csrfapi"
			}
		}
	}

	// Priority 4: Check for known domain patterns
	knownDomains := map[string]string{
		// Algolia-based sites (Gamma/Adult Time network)
		"girlsway.com":       "algolia",
		"puretaboo.com":      "algolia",
		"burningangel.com":   "algolia",
		"devilsfilm.com":     "algolia",
		"fantasymassage.com": "algolia",
		"allgirlmassage.com": "algolia",
		"nurumassage.com":    "algolia",
		"transsensual.com":   "algolia",
		"adulttime.com":      "algolia",

		// Project1Service sites (Aylo/MindGeek)
		"brazzers.com":          "project1service",
		"realitykings.com":      "project1service",
		"mofos.com":             "project1service",
		"twistys.com":           "project1service",
		"babes.com":             "project1service",
		"digitalplayground.com": "project1service",
		"fakehub.com":           "project1service",
	}

	for knownDomain, scraperType := range knownDomains {
		if strings.Contains(domain, knownDomain) {
			return scraperType
		}
	}

	// Priority 5: If none of above, default to htmlxpath as it's most universal
	return "htmlxpath"
}

// discoverScraperConfig discovers configuration values by analyzing the target URL.
func discoverScraperConfig(scraperType, startURL string) *ScraperDiscoveryData {
	data := &ScraperDiscoveryData{}

	switch scraperType {
	case "algolia":
		discoverAlgoliaConfig(startURL, data)
	case "project1service":
		discoverProject1ServiceConfig(startURL, data)
	case "csrfapi":
		discoverCSRFAPIConfig(startURL, data)
	case "htmlxpath":
		// For htmlxpath, we don't auto-discover, just return empty data
		// User needs to manually inspect the page
		data.Error = "" // No error, just show the help guide
	default:
		data.Error = "Scraper type not supported for automatic discovery"
	}

	return data
}

func getAlgoliaCredentials(resp *http.Response) (string, string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read response body: %w", err)
	}

	content := string(body)

	// Try pattern 1: "applicationID":"...","apiKey":"..."
	re1 := regexp.MustCompile(
		`"applicationID":"([a-zA-Z0-9]{1,})","apiKey":"([a-zA-Z0-9=,\.]{1,})"`,
	)
	matches := re1.FindStringSubmatch(content)

	if len(matches) == 3 {
		// s.applicationID = matches[1]
		// s.apiKey = matches[2]
		return matches[1], matches[2], nil
	}

	// Try pattern 2: "apiKey":"...","applicationID":"..."
	re2 := regexp.MustCompile(
		`"apiKey":"([a-zA-Z0-9=,\.]{1,})","applicationID":"([a-zA-Z0-9]{1,})"`,
	)

	matches = re2.FindStringSubmatch(content)

	if len(matches) == 3 {
		// s.apiKey = matches[1]
		// s.applicationID = matches[2]
		return matches[2], matches[1], nil
	}

	return "", "", errors.New("algolia API credentials not found")
}

// discoverAlgoliaConfig discovers Algolia configuration.
func discoverAlgoliaConfig(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, startURL, nil)
	if err != nil {
		data.Error = "Failed to create request: " + err.Error()
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch URL: " + err.Error()
		return
	}
	defer resp.Body.Close()

	data.AlgoliaAppID, data.AlgoliaAPIKey, err = getAlgoliaCredentials(resp)
	if err != nil {
		data.Error = "Failed to read response: " + err.Error()
		return
	}

	if data.AlgoliaAppID == "" || data.AlgoliaAPIKey == "" {
		data.Error = "Could not extract Algolia credentials from page HTML"
		return
	}

	// Decode base64 API key if needed
	if strings.Contains(data.AlgoliaAPIKey, "=") {
		// decoded, err := base64.StdEncoding.DecodeString(data.AlgoliaAPIKey)
		// if err == nil {
		// data.AlgoliaAPIKey = string(decoded)
		//}
	}

	// Now fetch a sample to discover available filter values
	fetchAlgoliaSample(startURL, data)
}

// fetchAlgoliaSample fetches comprehensive data from Algolia using their advanced API.
func fetchAlgoliaSample(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Try common index names used by different sites
	// These match the hardcoded index in scrapers/algolia/scraper.go
	possibleIndexNames := []string{
		"all_scenes_latest_desc", // Gamma/Adult Time common index (PRIMARY)
		"all_scenes",             // Alternative Algolia index
		"scenes_latest_desc",     // Alternative with sorting
		"scenes",                 // Generic scenes index
		"all_videos",             // Video-based sites
		"videos",                 // Alternative video index
		"content",                // Generic content index
	}

	// Use the search API with facets to get comprehensive filter data
	// Algolia DSN is consistent across their network
	apiURL := fmt.Sprintf(
		"https://tsmkfa364q-dsn.algolia.net/1/indexes/*/queries?x-algolia-application-id=%s&x-algolia-api-key=%s",
		url.QueryEscape(data.AlgoliaAppID),
		url.QueryEscape(data.AlgoliaAPIKey),
	)

	// Helper function for min
	min := func(a, b int) int {
		if a < b {
			return a
		}

		return b
	}

	// Try each possible index name
	var (
		successfulIndex string
		result          struct {
			Facets map[string]map[string]int `json:"facets"`
			Hits   []map[string]any          `json:"hits"`
		}
	)

	var lastError string

	var debugInfo string // Store debug information
	for _, indexName := range possibleIndexNames {
		// Request with faceting enabled - get ALL facets with counts
		// This matches what the actual scraper does
		requestBody := fmt.Sprintf(
			`{"requests":[{"indexName":"%s","params":"query=&hitsPerPage=10&page=0&facets=*&maxValuesPerFacet=2000"}]}`,
			indexName,
		)

		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, apiURL, strings.NewReader(requestBody))
		if err != nil {
			lastError = fmt.Sprintf("Failed to create request for index '%s': %v", indexName, err)
			continue // Try next index
		}

		req.Header.Set("Accept", "application/json")
		req.Header.Set("Referer", startURL)
		req.Header.Set("x-algolia-api-key", data.AlgoliaAPIKey)
		req.Header.Set("x-algolia-application-id", data.AlgoliaAppID)

		resp, err := client.Do(req)
		if err != nil {
			lastError = fmt.Sprintf("Failed to fetch index '%s': %v", indexName, err)
			continue // Try next index
		}
		// Drain and close here rather than deferring: this is a loop over
		// candidate indexes, so a deferred Close would keep every response body
		// open until the whole function returns.
		body, readErr := io.ReadAll(resp.Body)
		statusCode := resp.StatusCode

		resp.Body.Close()

		// Check HTTP status
		if statusCode != http.StatusOK {
			// Build debug info for 403 errors
			debugInfo = fmt.Sprintf(
				"\n\nDEBUG INFO:\nAPI URL: %s\nApp ID: %s\nAPI Key: %s\nRequest Body: %s\nReferer: %s\nHTTP Status: %d",
				apiURL,
				data.AlgoliaAppID,
				data.AlgoliaAPIKey,
				requestBody,
				startURL,
				statusCode,
			)
			lastError = fmt.Sprintf("Index '%s' returned HTTP %d", indexName, statusCode)

			continue // Try next index
		}

		if readErr != nil {
			lastError = fmt.Sprintf("Failed to read response for index '%s': %v", indexName, readErr)
			continue // Try next index
		}

		// Parse response with facets
		var algoliaResp struct {
			Results []struct {
				Facets map[string]map[string]int `json:"facets"`
				Hits   []map[string]any          `json:"hits"`
			} `json:"results"`
			Message string `json:"message"` // Algolia error message
		}

		if err := json.Unmarshal(body, &algoliaResp); err != nil {
			lastError = fmt.Sprintf(
				"Failed to parse JSON for index '%s': %v. Response: %s",
				indexName,
				err,
				string(body[:min(200, len(body))]),
			)

			continue // Try next index
		}

		// Check for Algolia error message
		if algoliaResp.Message != "" {
			lastError = fmt.Sprintf(
				"Algolia error for index '%s': %s",
				indexName,
				algoliaResp.Message,
			)

			continue // Try next index
		}

		// Check if we got a valid response with data
		if len(algoliaResp.Results) == 0 {
			lastError = fmt.Sprintf("Index '%s' returned empty results array", indexName)
			continue // Try next index - no results
		}

		// Success if we have at least some data (hits or facets)
		hasData := len(algoliaResp.Results[0].Hits) > 0 || len(algoliaResp.Results[0].Facets) > 0
		if !hasData {
			lastError = fmt.Sprintf("Index '%s' returned no hits or facets (hits: %d, facets: %d)",
				indexName, len(algoliaResp.Results[0].Hits), len(algoliaResp.Results[0].Facets))
			continue // Try next index - no data
		}

		// Success! We found a working index
		successfulIndex = indexName
		result = algoliaResp.Results[0]

		// Store sample response (truncate if too large)
		if len(body) > 5000 {
			data.SampleResponse = string(body[:5000]) + "... (truncated)"
		} else {
			data.SampleResponse = string(body)
		}

		break // Exit loop, we found data
	}

	// Check if we found any working index
	if successfulIndex == "" {
		errorMsg := "Could not find valid Algolia index. Tried: " + strings.Join(
			possibleIndexNames,
			", ",
		)
		if lastError != "" {
			errorMsg += ". Last error: " + lastError
		}

		if debugInfo != "" {
			errorMsg += debugInfo
		}

		data.Error = errorMsg

		return
	}

	data.AlgoliaIndexName = successfulIndex

	// Initialize facets map
	data.AlgoliaFacets = make(map[string][]FacetValue)

	// Extract all facets with their counts
	for facetName, facetValues := range result.Facets {
		var values []FacetValue
		for value, count := range facetValues {
			values = append(values, FacetValue{Value: value, Count: count})
		}

		data.AlgoliaFacets[facetName] = values

		// Populate specific known facets for easy access
		switch facetName {
		case "sitename":
			data.AvailableSites = values
		case "serie_name":
			data.AvailableSeries = values
		case "network.lvl0":
			data.AvailableNetworks0 = values
		case "network.lvl1":
			data.AvailableNetworks1 = values
		}
	}

	// Extract field names from a sample hit
	if len(result.Hits) > 0 {
		for fieldName := range result.Hits[0] {
			data.AlgoliaIndexFields = append(data.AlgoliaIndexFields, fieldName)
		}
	}
}

// discoverProject1ServiceConfig discovers Project1Service configuration.
func discoverProject1ServiceConfig(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	// Fetch the start URL to get instance token
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, startURL, nil)
	if err != nil {
		data.Error = "Failed to create request: " + err.Error()
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch URL: " + err.Error()
		return
	}
	defer resp.Body.Close()

	// Extract instance_token cookie
	var instanceToken string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "instance_token" {
			instanceToken = cookie.Value
			break
		}
	}

	if instanceToken == "" {
		data.Error = "Could not find instance_token cookie - make sure the URL is correct"
		return
	}

	// Parse URL to get site domain and path
	// parsedURL, err := url.Parse(startURL)
	// if err != nil {
	//	data.Error = "Invalid URL: " + err.Error()
	//	return
	//}

	// Fetch collections from API
	apiURL := "https://site-api.project1service.com/v1/collections?limit=100"

	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, apiURL, nil)
	if err != nil {
		data.Error = "Failed to create API request: " + err.Error()
		return
	}

	req.AddCookie(&http.Cookie{Name: "instance_token", Value: instanceToken})
	req.Header.Set("Accept", "application/json")

	resp, err = client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch collections: " + err.Error()
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		data.Error = "Failed to read collections response: " + err.Error()
		return
	}

	// Store sample response
	data.SampleResponse = string(body)

	// Parse collections
	var collections []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	if err := json.Unmarshal(body, &collections); err == nil {
		for _, col := range collections {
			data.Collections = append(data.Collections, CollectionInfo{
				ID:   col.ID,
				Name: col.Name,
			})
		}
	}
}

// discoverCSRFAPIConfig discovers CSRF API configuration.
func discoverCSRFAPIConfig(startURL string, data *ScraperDiscoveryData) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, startURL, nil)
	if err != nil {
		data.Error = "Failed to create request: " + err.Error()
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		data.Error = "Failed to fetch URL: " + err.Error()
		return
	}
	defer resp.Body.Close()

	// Look for CSRF cookies
	for _, cookie := range resp.Cookies() {
		cookieName := strings.ToLower(cookie.Name)
		if strings.Contains(cookieName, "csrf") || strings.Contains(cookieName, "xsrf") {
			data.CSRFCookie = cookie.Name
			break
		}
	}

	if data.CSRFCookie == "" {
		data.Error = "No CSRF cookie found - try checking browser DevTools manually"
	}

	// Suggest common CSRF header names
	data.CSRFHeader = "csrf-token (check Network tab in DevTools for exact header name)"
}

// getScraperTypeDisplayName returns a user-friendly display name for a scraper type.
func getScraperTypeDisplayName(scraperType string) string {
	switch scraperType {
	case "htmlxpath":
		return "HTML/XPath Scraper"
	case "csrfapi":
		return "CSRF API Scraper"
	case "algolia":
		return "Algolia Search"
	case "project1service":
		return "Project1Service (Aylo/MindGeek)"
	default:
		return scraperType
	}
}

// renderScraperHelperPage renders the scraper helper interface.
func renderScraperHelperPage(
	filename, index, scraperType, startURL string,
	discoveredData *ScraperDiscoveryData,
	_ string,
) gomponents.Node {
	// Check if scraper type was auto-detected
	autoDetected := startURL != "" && scraperType != ""

	return html.Div(
		html.Class("container-fluid p-4"),
		html.H1(html.Class("h3 mb-3"), gomponents.Text("Scraper Configuration Helper")),

		html.Div(
			html.Class("card mb-4"),
			html.Div(
				html.Class("card-header"),
				html.H5(html.Class("mb-0"), gomponents.Text("Discover Scraper Values")),
			),
			html.Div(
				html.Class("card-body"),
				html.P(
					gomponents.Text(
						"Enter a URL below and the helper will automatically detect the scraper type and extract available configuration values.",
					),
				),

				// Show auto-detection notice
				gomponents.If(autoDetected,
					html.Div(
						html.Class("alert alert-info mb-3"),
						html.I(html.Class("fa-solid fa-magic me-2")),
						html.Strong(gomponents.Text("Auto-detected scraper type: ")),
						gomponents.Text(getScraperTypeDisplayName(scraperType)),
					),
				),

				html.Form(
					html.Method("GET"),
					html.Action("/api/admin/seriesconfig/helper"),
					html.Input(html.Type("hidden"), html.Name("file"), html.Value(filename)),
					html.Input(html.Type("hidden"), html.Name("index"), html.Value(index)),

					html.Div(
						html.Class("mb-3"),
						html.Label(html.Class("form-label"), gomponents.Text("Start URL")),
						html.Input(
							html.Type("text"),
							html.Class("form-control"),
							html.Name("url"),
							html.ID("start-url"),
							html.Placeholder("https://example.com/videos"),
							html.Value(startURL),
							html.Required(),
						),
						html.Div(
							html.Class("form-text"),
							gomponents.Text(
								"The URL will be analyzed to auto-detect the scraper type",
							),
						),
					),

					html.Div(
						html.Class("mb-3"),
						html.Label(
							html.Class("form-label"),
							gomponents.Text("Scraper Type (optional - auto-detected if blank)"),
						),
						html.Select(
							html.Class("form-select"),
							html.Name("type"),
							html.ID("scraper-type"),
							html.Option(
								html.Value(""),
								gomponents.Text("Auto-detect"),
								gomponents.If(scraperType == "", html.Selected()),
							),
							html.Option(
								html.Value("htmlxpath"),
								gomponents.Text("HTML/XPath"),
								gomponents.If(scraperType == "htmlxpath", html.Selected()),
							),
							html.Option(
								html.Value("csrfapi"),
								gomponents.Text("CSRF API"),
								gomponents.If(scraperType == "csrfapi", html.Selected()),
							),
							html.Option(
								html.Value("algolia"),
								gomponents.Text("Algolia"),
								gomponents.If(scraperType == "algolia", html.Selected()),
							),
							html.Option(
								html.Value("project1service"),
								gomponents.Text("Project1Service"),
								gomponents.If(scraperType == "project1service", html.Selected()),
							),
						),
						html.Div(
							html.Class("form-text"),
							gomponents.Text(
								"Leave as 'Auto-detect' to automatically determine the scraper type",
							),
						),
					),

					html.Button(
						html.Class("btn btn-primary"),
						html.Type("submit"),
						html.I(html.Class("fa-solid fa-magic me-2")),
						gomponents.Text("Analyze URL"),
					),

					html.A(
						html.Class("btn btn-secondary ms-2"),
						html.Href(
							fmt.Sprintf(
								"/api/admin/seriesconfig/edit?file=%s&index=%s",
								url.QueryEscape(filename),
								url.QueryEscape(index),
							),
						),
						gomponents.Text("Back to Edit"),
					),
				),
			),
		),

		gomponents.If(
			discoveredData != nil,
			renderScraperHelperResults(scraperType, startURL, discoveredData),
		),
	)
}

// renderScraperHelperResults renders the analysis results.
func renderScraperHelperResults(
	scraperType, _ string,
	data *ScraperDiscoveryData,
) gomponents.Node {
	// Safety check for nil data
	if data == nil {
		return html.Div(
			html.Class("card"),
			html.Div(
				html.Class("card-header"),
				html.H5(html.Class("mb-0"), gomponents.Text("Analysis Results")),
			),
			html.Div(
				html.Class("card-body"),
				html.Div(
					html.Class("alert alert-warning"),
					gomponents.Text("No data available. Please enter a URL and click Analyze."),
				),
			),
		)
	}

	return html.Div(
		html.Class("card"),
		html.Div(
			html.Class("card-header"),
			html.H5(html.Class("mb-0"), gomponents.Text("Analysis Results")),
		),
		html.Div(
			html.Class("card-body"),
			// Show error if any
			gomponents.If(data.Error != "",
				html.Div(
					html.Class("alert alert-danger"),
					html.I(html.Class("fa-solid fa-exclamation-circle me-2")),
					gomponents.Text(data.Error),
				),
			),

			// Show discovered data based on scraper type
			gomponents.If(
				scraperType == "algolia" && data.Error == "",
				renderAlgoliaDiscoveredData(data),
			),
			gomponents.If(
				scraperType == "project1service" && data.Error == "",
				renderProject1ServiceDiscoveredData(data),
			),
			gomponents.If(
				scraperType == "csrfapi" && data.Error == "",
				renderCSRFAPIDiscoveredData(data),
			),

			// Always show the help guide
			gomponents.If(scraperType == "htmlxpath", renderXPathHelp()),
			gomponents.If(scraperType == "csrfapi" && data.CSRFCookie == "", renderCSRFAPIHelp()),
			gomponents.If(scraperType == "algolia" && data.AlgoliaAppID == "", renderAlgoliaHelp()),
			gomponents.If(
				scraperType == "project1service" && len(data.Collections) == 0,
				renderProject1ServiceHelp(),
			),
		),
	)
}

// renderAlgoliaDiscoveredData renders discovered Algolia configuration.
func renderAlgoliaDiscoveredData(data *ScraperDiscoveryData) gomponents.Node {
	return html.Div(
		html.Div(
			html.Class("alert alert-success"),
			html.I(html.Class("fa-solid fa-check-circle me-2")),
			gomponents.Text("Successfully discovered Algolia configuration!"),
		),

		html.H6(gomponents.Text("Algolia Credentials")),
		html.Div(
			html.Class("mb-3"),
			html.Strong(gomponents.Text("Application ID: ")),
			html.Code(gomponents.Text(data.AlgoliaAppID)),
		),
		html.Div(
			html.Class("mb-3"),
			html.Strong(gomponents.Text("API Key: ")),
			html.Code(gomponents.Text(data.AlgoliaAPIKey)),
		),
		html.Div(
			html.Class("mb-3"),
			html.Strong(gomponents.Text("Index Name: ")),
			html.Code(gomponents.Text(data.AlgoliaIndexName)),
		),

		// Display available index fields
		gomponents.If(len(data.AlgoliaIndexFields) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Available Index Fields")),
				html.Div(
					html.Class("alert alert-secondary"),
					html.Small(
						gomponents.Text(
							"These are the fields available in the Algolia index that you can use for filtering and extraction:",
						),
					),
					html.Div(
						html.Class("mt-2"),
						renderFieldList(data.AlgoliaIndexFields),
					),
				),
			),
		),

		// Display main facets with counts
		gomponents.If(len(data.AvailableSites) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(
					gomponents.Text(
						"Available Sites ("+fmt.Sprintf("%d", len(data.AvailableSites))+")",
					),
				),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("SiteFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact site name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Site Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableSites),
						),
					),
				),
			),
		),

		gomponents.If(len(data.AvailableSeries) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(
					gomponents.Text(
						"Available Series ("+fmt.Sprintf("%d", len(data.AvailableSeries))+")",
					),
				),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("SerieFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact series name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Series Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableSeries),
						),
					),
				),
			),
		),

		gomponents.If(len(data.AvailableNetworks0) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(
					gomponents.Text(
						"Available Networks Lvl0 ("+fmt.Sprintf(
							"%d",
							len(data.AvailableNetworks0),
						)+")",
					),
				),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("NetworkFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact network name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Network Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableNetworks0),
						),
					),
				),
			),
		),

		gomponents.If(len(data.AvailableNetworks1) > 0,
			html.Div(
				html.Class("mt-4"),
				html.H6(
					gomponents.Text(
						"Available Networks Lvl1 ("+fmt.Sprintf(
							"%d",
							len(data.AvailableNetworks1),
						)+")",
					),
				),
				html.Div(
					html.Class("alert alert-info mb-2"),
					html.Small(
						html.Strong(gomponents.Text("Filter name: ")),
						html.Code(gomponents.Text("NetworkFilterName")),
						gomponents.Text(" | "),
						html.Strong(gomponents.Text("Use value like: ")),
						gomponents.Text("Copy the exact network name below"),
					),
				),
				html.Div(
					html.Class("table-responsive"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("Network Name")),
								html.Th(gomponents.Text("Scene Count")),
							),
						),
						html.TBody(
							renderFacetRows(data.AvailableNetworks1),
						),
					),
				),
			),
		),
		// Display all other facets discovered
		gomponents.If(len(data.AlgoliaFacets) > 3,
			html.Div(
				html.Class("mt-4"),
				html.H6(gomponents.Text("Other Available Facets")),
				renderOtherFacets(data.AlgoliaFacets),
			),
		),
	)
}

// renderProject1ServiceDiscoveredData renders discovered Project1Service configuration.
func renderProject1ServiceDiscoveredData(data *ScraperDiscoveryData) gomponents.Node {
	return html.Div(
		html.Div(
			html.Class("alert alert-success"),
			html.I(html.Class("fa-solid fa-check-circle me-2")),
			gomponents.Text("Successfully discovered Project1Service collections!"),
		),

		gomponents.If(len(data.Collections) > 0,
			html.Div(
				html.H6(
					gomponents.Text(
						"Available Collections ("+fmt.Sprintf("%d", len(data.Collections))+")",
					),
				),
				html.Div(
					html.Class("table-responsive mt-3"),
					html.Table(
						html.Class("table table-sm table-striped"),
						html.THead(
							html.Tr(
								html.Th(gomponents.Text("ID")),
								html.Th(gomponents.Text("Collection Name")),
							),
						),
						html.TBody(
							renderCollectionRows(data.Collections),
						),
					),
				),
				html.Div(
					html.Class("alert alert-info mt-3"),
					html.Strong(gomponents.Text("Usage: ")),
					gomponents.Text(
						"Copy the ID number from the table above and use it as FilterCollectionID in your configuration.",
					),
				),
			),
		),
	)
}

// renderCSRFAPIDiscoveredData renders discovered CSRF API configuration.
func renderCSRFAPIDiscoveredData(data *ScraperDiscoveryData) gomponents.Node {
	return html.Div(
		gomponents.If(data.CSRFCookie != "",
			html.Div(
				html.Div(
					html.Class("alert alert-success"),
					html.I(html.Class("fa-solid fa-check-circle me-2")),
					gomponents.Text("Found CSRF cookie!"),
				),
				html.Div(
					html.Class("mb-3"),
					html.Strong(gomponents.Text("CSRF Cookie Name: ")),
					html.Code(gomponents.Text(data.CSRFCookie)),
				),
				html.Div(
					html.Class("mb-3"),
					html.Strong(gomponents.Text("CSRF Header Name: ")),
					gomponents.Text(data.CSRFHeader),
				),
			),
		),
	)
}

// renderFacetRows renders facet values with counts as table rows.
func renderFacetRows(facets []FacetValue) gomponents.Node {
	rows := make([]gomponents.Node, 0, len(facets))
	for _, facet := range facets {
		rows = append(rows, html.Tr(
			html.Td(html.Code(gomponents.Text(facet.Value))),
			html.Td(gomponents.Text(fmt.Sprintf("%d", facet.Count))),
		))
	}

	return gomponents.Group(rows)
}

// renderFieldList renders a list of field names.
func renderFieldList(fields []string) gomponents.Node {
	nodes := make([]gomponents.Node, 0, len(fields))
	for _, field := range fields {
		nodes = append(nodes, html.Span(
			html.Class("badge bg-secondary me-1 mb-1"),
			gomponents.Text(field),
		))
	}

	return gomponents.Group(nodes)
}

// renderOtherFacets renders all facets that aren't the main three.
func renderOtherFacets(allFacets map[string][]FacetValue) gomponents.Node {
	var nodes []gomponents.Node

	// Skip the main three facets we already displayed
	skipFacets := map[string]bool{
		"sitename":     true,
		"serie_name":   true,
		"network_name": true,
	}

	for facetName, values := range allFacets {
		if skipFacets[facetName] || len(values) == 0 {
			continue
		}

		// Limit display to first 20 values for other facets
		displayValues := values
		if len(displayValues) > 20 {
			displayValues = displayValues[:20]
		}

		nodes = append(nodes, html.Div(
			html.Class("mb-3"),
			html.H6(
				html.Class("text-muted"),
				gomponents.Text(facetName),
				html.Small(
					html.Class("ms-2"),
					gomponents.Text(fmt.Sprintf("(%d values)", len(values))),
				),
			),
			html.Div(
				html.Class("table-responsive"),
				html.Table(
					html.Class("table table-sm table-striped"),
					html.THead(
						html.Tr(
							html.Th(gomponents.Text("Value")),
							html.Th(gomponents.Text("Count")),
						),
					),
					html.TBody(
						renderFacetRows(displayValues),
					),
				),
			),
			gomponents.If(len(values) > 20,
				html.Small(html.Class("text-muted"),
					gomponents.Text(fmt.Sprintf("Showing first 20 of %d values", len(values))),
				),
			),
		))
	}

	if len(nodes) == 0 {
		return gomponents.Text("")
	}

	return gomponents.Group(nodes)
}

// renderCollectionRows renders collection table rows.
func renderCollectionRows(collections []CollectionInfo) gomponents.Node {
	rows := make([]gomponents.Node, 0, len(collections))
	for _, col := range collections {
		rows = append(rows, html.Tr(
			html.Td(html.Code(gomponents.Text(fmt.Sprintf("%d", col.ID)))),
			html.Td(gomponents.Text(col.Name)),
		))
	}

	return gomponents.Group(rows)
}

// renderXPathHelp renders XPath configuration help.
func renderXPathHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("HTML/XPath Scraper Configuration Guide")),
		html.P(
			gomponents.Text(
				"To configure an HTML/XPath scraper, you need to inspect the website's HTML structure using browser DevTools (F12):",
			),
		),
		html.Ol(
			html.Li(gomponents.Text("Open the target URL in your browser")),
			html.Li(gomponents.Text("Press F12 to open DevTools, go to Elements/Inspector tab")),
			html.Li(gomponents.Text("Right-click on a video/scene container → Inspect")),
			html.Li(
				gomponents.Text(
					"Find the common container element (e.g., <div class=\"video-thumb\">)",
				),
			),
			html.Li(gomponents.Text("Right-click the element → Copy → Copy XPath")),
		),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("Common XPath Patterns:")),
			html.Ul(
				html.Li(
					html.Code(gomponents.Text("//div[@class=\"video-thumb\"]")),
					gomponents.Text(" - Scene containers"),
				),
				html.Li(
					html.Code(gomponents.Text(".//a[@class=\"title\"]")),
					gomponents.Text(" - Title link (relative to container)"),
				),
				html.Li(
					html.Code(gomponents.Text(".//span[@class=\"date\"]")),
					gomponents.Text(" - Date element"),
				),
				html.Li(
					html.Code(gomponents.Text(".//div[@class=\"models\"]/a")),
					gomponents.Text(" - Actor links"),
				),
			),
		),

		html.Div(
			html.Class("alert alert-warning mt-3"),
			html.Strong(gomponents.Text("Testing XPath: ")),
			gomponents.Text("In browser console, use "),
			html.Code(gomponents.Text("$x('//your/xpath')")),
			gomponents.Text(" to test if it selects the right elements."),
		),
	)
}

// renderCSRFAPIHelp renders CSRF API configuration help.
func renderCSRFAPIHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("CSRF API Scraper Configuration Guide")),
		html.P(
			gomponents.Text(
				"To configure a CSRF API scraper, you need to inspect the website's API calls using browser DevTools:",
			),
		),
		html.Ol(
			html.Li(gomponents.Text("Open the target URL in your browser")),
			html.Li(gomponents.Text("Press F12 → Network tab → Filter to 'Fetch/XHR'")),
			html.Li(gomponents.Text("Reload the page or navigate to see API calls")),
			html.Li(
				gomponents.Text(
					"Look for JSON API requests (usually contain 'api', 'page', 'gallery', etc.)",
				),
			),
			html.Li(gomponents.Text("Click on the API call → Headers tab")),
		),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("What to find:")),
			html.Ul(
				html.Li(
					html.Strong(gomponents.Text("CSRF Cookie: ")),
					gomponents.Text(
						"Application tab → Cookies → look for cookies like '_csrf', 'csrf_token', 'XSRF-TOKEN'",
					),
				),
				html.Li(
					html.Strong(gomponents.Text("CSRF Header: ")),
					gomponents.Text(
						"Request Headers → look for 'csrf-token', 'X-CSRF-Token', 'X-XSRF-TOKEN'",
					),
				),
				html.Li(
					html.Strong(gomponents.Text("API URL: ")),
					gomponents.Text("Copy the Request URL, replace page number with {page}"),
				),
			),
		),

		html.Div(
			html.Class("alert alert-info mt-3"),
			html.H6(gomponents.Text("JSON Response Fields:")),
			gomponents.Text(
				"Click on the API call → Response/Preview tab to see the JSON structure:",
			),
			html.Ul(
				html.Li(
					html.Code(gomponents.Text("galleries")),
					gomponents.Text(", "),
					html.Code(gomponents.Text("scenes")),
					gomponents.Text(", "),
					html.Code(gomponents.Text("data.results")),
					gomponents.Text(" - common array paths"),
				),
				html.Li(
					html.Code(gomponents.Text("name")),
					gomponents.Text(", "),
					html.Code(gomponents.Text("title")),
					gomponents.Text(" - title fields"),
				),
				html.Li(
					html.Code(gomponents.Text("publishedAt")),
					gomponents.Text(", "),
					html.Code(gomponents.Text("release_date")),
					gomponents.Text(" - date fields"),
				),
			),
		),
	)
}

// renderAlgoliaHelp renders Algolia configuration help.
func renderAlgoliaHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("Algolia Scraper Configuration Guide")),
		html.P(
			gomponents.Text(
				"Algolia scrapers are used for Gamma Entertainment/Adult Time network sites:",
			),
		),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("How to find filter values:")),
			html.Ol(
				html.Li(gomponents.Text("Open the target URL in your browser")),
				html.Li(gomponents.Text("Press F12 → Network tab → Filter to 'Fetch/XHR'")),
				html.Li(gomponents.Text("Reload the page")),
				html.Li(gomponents.Text("Look for requests to 'algolia' or 'algolianet'")),
				html.Li(gomponents.Text("Click on the request → Payload tab")),
				html.Li(gomponents.Text("Look at the filters/facetFilters in the request body")),
			),
		),

		html.Div(
			html.Class("alert alert-info mt-3"),
			html.H6(gomponents.Text("Common Filter Values:")),
			html.Ul(
				html.Li(
					html.Strong(gomponents.Text("SiteFilterName: ")),
					html.Code(gomponents.Text("sitename:girlsway")),
					gomponents.Text(" → use 'girlsway'"),
				),
				html.Li(
					html.Strong(gomponents.Text("SerieFilterName: ")),
					gomponents.Text("Usually the series/channel name"),
				),
				html.Li(
					html.Strong(gomponents.Text("NetworkFilterName: ")),
					gomponents.Text("The network name if filtering by network"),
				),
			),
		),
	)
}

// renderProject1ServiceHelp renders Project1Service configuration help.
func renderProject1ServiceHelp() gomponents.Node {
	return html.Div(
		html.H6(gomponents.Text("Project1Service Scraper Configuration Guide")),
		html.P(
			gomponents.Text(
				"Project1Service scrapers are used for Aylo/MindGeek network sites (Brazzers, RealityKings, etc.):",
			),
		),

		html.Div(
			html.Class("alert alert-secondary mt-3"),
			html.H6(gomponents.Text("How to find FilterCollectionID:")),
			html.Ol(
				html.Li(gomponents.Text("Navigate to the site (e.g., twistys.com)")),
				html.Li(gomponents.Text("Browse to the specific series/collection you want")),
				html.Li(gomponents.Text("Look at the URL in your browser")),
				html.Li(gomponents.Text("The collection ID is the numeric value in the URL")),
			),
		),

		html.Div(
			html.Class("alert alert-info mt-3"),
			html.H6(gomponents.Text("Example:")),
			html.P(
				gomponents.Text("If the URL is: "),
				html.Code(
					gomponents.Text("https://www.twistys.com/collection/227/when-girls-play"),
				),
			),
			html.P(
				gomponents.Text("Then "),
				html.Strong(gomponents.Text("FilterCollectionID = 227")),
			),
		),

		html.Div(
			html.Class("alert alert-warning mt-3"),
			html.Strong(gomponents.Text("Note: ")),
			gomponents.Text(
				"You may need to filter by site first to see collections. Set to 0 to disable filtering and get all scenes from the site.",
			),
		),
	)
}

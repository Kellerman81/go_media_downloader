// Package api - path_check.go adds a live read/write check for storage paths,
// used both in the Setup Wizard and the full Settings -> Paths section. The
// check never trusts a saved PathsConfig; it always re-validates whatever
// folder string is currently typed into the form.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"

	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// checkReadable reports whether p's contents can be listed.
func checkReadable(p string) bool {
	f, err := os.Open(p)
	if err != nil {
		return false
	}
	defer f.Close()

	_, err = f.Readdirnames(1)

	return err == nil || errors.Is(err, io.EOF)
}

// checkWritable reports whether a file can be created inside p, by actually
// creating (and immediately removing) a harmless temp file - the only
// reliable way to check write access across platforms/filesystems.
func checkWritable(p string) bool {
	f, err := os.CreateTemp(p, ".gmd_writetest_*")
	if err != nil {
		return false
	}

	name := f.Name()
	f.Close()
	os.Remove(name)

	return true
}

// HandlePathCheck live-validates a folder path and returns a small fragment
// with the folder's existence plus separate read/write status icons. The
// path is posted under a fixed "path" key by renderPathCheckControl's
// hx-vals, independent of whatever other fields the enclosing config form
// also happens to submit.
func HandlePathCheck(c *gin.Context) {
	c.Header("Content-Type", "text/html")

	p := strings.TrimSpace(c.PostForm("path"))
	if p == "" {
		c.String(http.StatusOK, renderAlert("Enter a folder path first.", "warning"))
		return
	}

	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			c.String(
				http.StatusOK,
				renderAlert("Folder does not exist yet - it will need to be created before use.", "warning"),
			)

			return
		}

		c.String(http.StatusOK, renderAlert("Cannot access path: "+err.Error(), "danger"))

		return
	}

	if !info.IsDir() {
		c.String(http.StatusOK, renderAlert("Path exists but is a file, not a folder.", "danger"))
		return
	}

	c.String(
		http.StatusOK,
		renderComponentToString(renderPathCheckResult(checkReadable(p), checkWritable(p))),
	)
}

// pathCheckBadge renders a single "Read"/"Write" pill, colored green with a
// check icon when ok is true and red with an X icon otherwise.
func pathCheckBadge(label string, ok bool) gomponents.Node {
	cssClass := "badge bg-success"
	icon := "fa-solid fa-check"

	if !ok {
		cssClass = "badge bg-danger"
		icon = "fa-solid fa-xmark"
	}

	return html.Span(
		html.Class(cssClass+" d-inline-flex align-items-center gap-1"),
		html.I(html.Class(icon)),
		gomponents.Text(label),
	)
}

// renderPathCheckResult renders the exists+readable+writable status of a
// folder as a row of badges, one per check.
func renderPathCheckResult(readable, writable bool) gomponents.Node {
	return html.Div(
		html.Class("d-inline-flex align-items-center gap-2"),
		html.Span(
			html.Class("badge bg-secondary d-inline-flex align-items-center gap-1"),
			html.I(html.Class("fa-solid fa-folder")),
			gomponents.Text("Exists"),
		),
		pathCheckBadge("Read", readable),
		pathCheckBadge("Write", writable),
	)
}

// pathBrowseMax bounds the folders one listing returns.
const pathBrowseMax = 2000

// pathBrowseResult is a folder's subfolders for the path browser.
type pathBrowseResult struct {
	Path   string   `json:"path"`
	Parent string   `json:"parent"` // "" at the root
	Dirs   []string `json:"dirs"`
	More   bool     `json:"more"`   // more than pathBrowseMax folders
	Places []string `json:"places"` // starting points: the configured paths' top folders
	Note   string   `json:"note,omitempty"`
}

// HandlePathBrowse lists the subfolders of a folder for the path fields'
// browser (/admin/paths/browse?path=). A path that doesn't exist (yet) shows
// its nearest existing parent; nothing is changed on disk.
func HandlePathBrowse(c *gin.Context) {
	p := strings.TrimSpace(c.Query("path"))
	if p == "" {
		p = string(filepath.Separator)
	}

	p = filepath.Clean(p)
	res := pathBrowseResult{Dirs: []string{}, Places: pathBrowsePlaces()}

	// Up to the nearest folder that exists.
	for {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			break
		}

		parent := filepath.Dir(p)
		if parent == p {
			break
		}

		if res.Note == "" {
			res.Note = "The folder doesn't exist (yet): showing the nearest existing one."
		}

		p = parent
	}

	res.Path = p
	if parent := filepath.Dir(p); parent != p {
		res.Parent = parent
	}

	entries, err := os.ReadDir(p)
	if err != nil {
		res.Note = "Can't read this folder: " + err.Error()
	}

	for _, e := range entries {
		if !e.IsDir() && e.Type()&os.ModeSymlink != 0 {
			// A link to a folder counts as one.
			if info, err := os.Stat(filepath.Join(p, e.Name())); err != nil || !info.IsDir() {
				continue
			}
		} else if !e.IsDir() {
			continue
		}

		if len(res.Dirs) == pathBrowseMax {
			res.More = true
			break
		}

		res.Dirs = append(res.Dirs, e.Name())
	}

	slices.SortFunc(res.Dirs, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })

	c.JSON(http.StatusOK, res)
}

// pathBrowsePlaces are the top folders of the configured paths (/data,
// /downloads, ...), the browser's starting points.
func pathBrowsePlaces() []string {
	var places []string

	for _, pc := range config.GetSettingsPathAll() {
		p := filepath.Clean(pc.Path)
		if pc.Path == "" || !filepath.IsAbs(p) {
			continue
		}

		top := p
		for filepath.Dir(top) != filepath.Dir(filepath.Dir(top)) {
			top = filepath.Dir(top)
		}

		if !slices.Contains(places, top) {
			places = append(places, top)
		}
	}

	slices.Sort(places)

	return places
}

// renderPathCheckControl renders a "Check Path" button that live-validates
// whatever value currently sits in the path <input> named fieldName, plus an
// empty result span for the response. fieldName must be the exact "name"
// attribute the caller gave that input (group+"_"+field, per renderFormGroup) -
// looking it up by name instead of DOM position sidesteps two htmx pitfalls:
// the button sits inside the page's one big config <form>, so htmx's default
// behavior also submits every other field in that form regardless of
// hx-include/hx-vals, and htmx's own "previous"/"closest" selector extensions
// are only understood inside attributes like hx-include - the public
// htmx.find() JS API only takes plain CSS selectors, so reusing that syntax
// there silently resolves to nothing.
func renderPathCheckControl(csrfToken, fieldName string) gomponents.Node {
	fieldNameJSON, _ := json.Marshal(fieldName)

	return html.Div(
		html.Class("d-flex align-items-center gap-2 mt-1"),
		html.Button(
			html.Class("btn btn-sm btn-outline-info"),
			html.Type("button"),
			gomponents.Text("Check Path"),
			hx.Target("next span"),
			hx.Swap("innerHTML"),
			hx.Post("/api/admin/paths/check"),
			hx.Vals(
				`js:{path: (document.querySelector('[name=' + CSS.escape(`+string(
					fieldNameJSON,
				)+`) + ']')||{}).value || ""}`,
			),
			hx.Headers(wizardCSRFHeader(csrfToken)),
		),
		// Optional to typing the path: pick it in a folder browser.
		html.Button(
			html.Class("btn btn-sm btn-outline-secondary"),
			html.Type("button"),
			gomponents.Attr("onclick", "gmdPathBrowse("+string(fieldNameJSON)+")"),
			html.I(html.Class("fa-solid fa-folder-tree me-1")),
			gomponents.Text("Browse…"),
		),
		html.Script(html.Src("/static/js/path-browser.js")),
		html.Span(),
	)
}

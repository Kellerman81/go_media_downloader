package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
)

func TestWizardCheckFolders(t *testing.T) {
	dir := t.TempDir()
	lib, imp := filepath.Join(dir, "lib"), filepath.Join(dir, "new", "import")

	if err := os.Mkdir(lib, 0o755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Missing without create: reported, not created.
	if p := wizardCheckFolders([]wizardFolder{{"EN", "import", imp}}, false); len(p) != 1 || !strings.Contains(p[0], "doesn't exist") {
		t.Errorf("missing: %v", p)
	}

	if _, err := os.Stat(imp); err == nil {
		t.Fatal("created without create")
	}

	// With create: made; empty paths skipped.
	if p := wizardCheckFolders([]wizardFolder{{"EN", "library", lib}, {"EN", "import", imp}, {"EN", "import", ""}}, true); len(p) != 0 {
		t.Errorf("valid: %v", p)
	}

	if st, err := os.Stat(imp); err != nil || !st.IsDir() {
		t.Fatal("not created")
	}

	for name, folders := range map[string][]wizardFolder{
		"also the":          {{"EN", "library", lib}, {"DE", "library", lib + "/"}},
		"inside each other": {{"EN", "import", dir}, {"EN", "library", lib}},
		"is a file":         {{"EN", "library", file}},
	} {
		p := wizardCheckFolders(folders, true)
		if len(p) != 1 || !strings.Contains(p[0], name) {
			t.Errorf("%s: %v", name, p)
		}
	}
}

func TestWizardDirVerdict(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "Movies")

	for _, tc := range []struct{ client, imp, level string }{
		{dir, "", "success"},
		{"/no/such/folder", "", "warning"},
		{dir, dir + "/", "success"},             // the same
		{sub, dir, "success"},                   // a category folder below the import folder
		{dir, sub, "danger"},                    // visible, but elsewhere
		{"/client/view/Movies", sub, "warning"}, // another container's name for it
	} {
		if _, level := wizardDirVerdict(tc.client, tc.imp); level != tc.level {
			t.Errorf("%s vs %q: %s, want %s", tc.client, tc.imp, level, tc.level)
		}
	}
}

// TestHandleWizardStepSaveError: a failed save stays on its step; only the
// error is sent, into the step's alert box.
func TestHandleWizardStepSaveError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	form := url.Values{"wizard_ctx_group_movies_enabled": {"1"}, "wizard_ctx_group_movies_name": {"X"},
		"wizard_path_LibraryMovies": {"/no/such/folder/at/all"}}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	c.Params = gin.Params{{Key: "n", Value: "3"}}

	HandleWizardStepSave(c)

	if w.Header().Get("HX-Retarget") != "#wizardStepAlert" || !strings.Contains(w.Body.String(), "/no/such/folder/at/all") ||
		strings.Contains(w.Body.String(), "wizardCarry") {
		t.Fatalf("headers %v body %s", w.Header(), w.Body.String())
	}
}

func TestHandleWizardRunJobUnknown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	useMediaConfig(t, config.MediaConfig{Movies: mediaGroup("EN")})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"job": {"rm -rf"}, "config": {"movie_EN"}}.Encode()))
	c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	HandleWizardRunJob(c)

	if !strings.Contains(w.Body.String(), "Unknown job") {
		t.Fatalf("body: %s", w.Body.String())
	}
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestPathBrowse: the folder browser lists a folder's subfolders (not its
// files), sorted, with the parent; a path that doesn't exist shows its
// nearest existing parent.
func TestPathBrowse(t *testing.T) {
	gin.SetMode(gin.TestMode)

	dir := t.TempDir()
	for _, d := range []string{"beta", "Alpha", "gamma/deep"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	browse := func(p string) pathBrowseResult {
		t.Helper()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/?path="+url.QueryEscape(p), nil)
		HandlePathBrowse(c)

		var res pathBrowseResult
		if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil || w.Code != http.StatusOK {
			t.Fatalf("browse %s: %d %s", p, w.Code, w.Body)
		}

		return res
	}

	res := browse(dir)
	if res.Path != dir || res.Parent != filepath.Dir(dir) || !slices.Equal(res.Dirs, []string{"Alpha", "beta", "gamma"}) || res.Note != "" {
		t.Errorf("folder: %+v", res)
	}

	res = browse(filepath.Join(dir, "gamma", "missing", "deeper"))
	if res.Path != filepath.Join(dir, "gamma") || !slices.Equal(res.Dirs, []string{"deep"}) || !strings.Contains(res.Note, "doesn't exist") {
		t.Errorf("missing folder: %+v", res)
	}

	if res := browse(""); res.Path != string(filepath.Separator) || res.Parent != "" {
		t.Errorf("root: %+v", res)
	}
}

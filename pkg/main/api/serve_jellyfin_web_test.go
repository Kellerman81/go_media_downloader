package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// fakeJellyfinPackage is a server package with a tiny web client and a
// file outside it, which must not be extracted.
func fakeJellyfinPackage(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer

	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	for name, body := range map[string]string{
		"jellyfin/jellyfin-web/index.html":       "<html>web client</html>",
		"jellyfin/jellyfin-web/main.bundle.js":   "console.log(1)",
		"jellyfin/jellyfin-web/../../escape.txt": "x",
		"jellyfin/jellyfin.dll":                  "server",
	} {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}

		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}

	tw.Close()
	gz.Close()

	return buf.Bytes()
}

// TestJellyfinWeb: the web client is extracted from the server package and
// served under /jellyfin/web/.
func TestJellyfinWeb(t *testing.T) {
	f := clientTestData(t)

	config.GetSettingsGeneral().CoverCacheDir = filepath.Join(t.TempDir(), "covers")

	if w := f.do(t, http.MethodGet, "/jellyfin/web/", nil, nil); w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Swiftfin") {
		t.Fatalf("before the download: %d %s", w.Code, w.Body)
	}

	pkg := fakeJellyfinPackage(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(pkg) }))
	defer srv.Close()

	old := jfWebURL
	jfWebURL = srv.URL
	t.Cleanup(func() { jfWebURL = old })

	if err := downloadJellyfinWeb(context.Background()); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(filepath.Dir(jfWebDir()), "escape.txt")); err == nil {
		t.Fatal("a file outside the web client was extracted")
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/", nil, nil); w.Code != http.StatusFound || w.Header().Get("Location") != "/jellyfin/web/" {
		t.Fatalf("root: %d %s", w.Code, w.Header().Get("Location"))
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/web", nil, nil); w.Code != http.StatusMovedPermanently {
		t.Fatalf("without slash: %d", w.Code)
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/web/", nil, nil); w.Code != http.StatusOK || w.Body.String() != "<html>web client</html>" {
		t.Fatalf("index: %d %s", w.Code, w.Body)
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/web/main.bundle.js?abc", nil, nil); w.Code != http.StatusOK || w.Body.String() != "console.log(1)" {
		t.Fatalf("bundle: %d %s", w.Code, w.Body)
	}

	if w := f.do(t, http.MethodGet, "/jellyfin/web/../../jellyfin.dll", nil, nil); w.Code == http.StatusOK && strings.Contains(w.Body.String(), "server") {
		t.Fatal("left the web client's folder")
	}
}

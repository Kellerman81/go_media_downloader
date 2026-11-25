package api

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
)

// Jellyfin's web client (jellyfin-web) under /jellyfin/web: the browser,
// the Jellyfin app for LG TVs and the official iPhone app show it. It isn't
// part of this program (GPL, 60 MB): while the Jellyfin API is on, it is
// loaded once from Jellyfin's server package of the version the API
// answers as, into a folder next to the cover cache.

const jfWebPrefix = "jellyfin/jellyfin-web/"

// jfWebURL is Jellyfin's server package with the web client.
var jfWebURL = "https://repo.jellyfin.org/files/server/portable/stable/v" + jfVersion + "/any/jellyfin_" + jfVersion + ".tar.gz"

// jfWebDir is the web client's folder (next to the cover cache, like the
// HLS conversions).
func jfWebDir() string {
	return filepath.Join(filepath.Dir(filepath.Clean(coverCacheDir())), "jellyfin-web-"+jfVersion)
}

// jfWebReady reports whether the web client is there.
func jfWebReady() bool {
	_, err := os.Stat(filepath.Join(jfWebDir(), "index.html"))
	return err == nil
}

var (
	jfWebOnce    sync.Once
	jfWebLoading atomic.Bool
)

// StartJellyfinWeb loads the web client while the Jellyfin API is on and it
// is missing (checked every 10 minutes, so a failed download is retried).
func StartJellyfinWeb() {
	jfWebOnce.Do(func() {
		go func() {
			for {
				if config.GetSettingsGeneral().ServeJellyfin && !jfWebReady() {
					jfWebLoading.Store(true)

					if err := downloadJellyfinWeb(context.Background()); err != nil {
						logger.Logtype("warn", 0).Err(err).Str("url", jfWebURL).Msg("Jellyfin web client not loaded, retried in 10 minutes")
					} else {
						logger.Logtype("info", 0).Str("dir", jfWebDir()).Msg("Jellyfin web client loaded")
					}

					jfWebLoading.Store(false)
				}

				time.Sleep(10 * time.Minute)
			}
		}()
	})
}

// downloadJellyfinWeb extracts the web client of the server package into a
// new folder, then moves it in place.
func downloadJellyfinWeb(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, jfWebURL, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return errors.New("download: " + resp.Status)
	}

	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}

	dest := jfWebDir()
	tmp := dest + ".tmp"
	_ = os.RemoveAll(tmp)

	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}

	tr := tar.NewReader(gz)
	files := 0

	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			os.RemoveAll(tmp)
			return err
		}

		rel, ok := strings.CutPrefix(h.Name, jfWebPrefix)
		if !ok || rel == "" || h.Typeflag != tar.TypeReg {
			continue
		}

		// Only plain paths inside the folder.
		clean := path.Clean("/" + rel)
		if clean != "/"+rel || strings.Contains(rel, "..") {
			continue
		}

		target := filepath.Join(tmp, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			os.RemoveAll(tmp)
			return err
		}

		f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			os.RemoveAll(tmp)
			return err
		}

		_, err = io.Copy(f, io.LimitReader(tr, 64<<20))
		f.Close()

		if err != nil {
			os.RemoveAll(tmp)
			return err
		}

		files++
	}

	if _, err := os.Stat(filepath.Join(tmp, "index.html")); err != nil || files == 0 {
		os.RemoveAll(tmp)
		return errors.New("no web client in the package")
	}

	_ = os.RemoveAll(dest)

	return os.Rename(tmp, dest)
}

// jfWeb serves the web client (/jellyfin/web/...), or explains why it
// isn't there.
func jfWeb(c *gin.Context, _ map[string]string) {
	if !jfWebReady() {
		jfNoWebClient(c, nil)
		return
	}

	// The client's files are relative to /web/.
	if strings.HasSuffix(strings.ToLower(c.Request.URL.Path), "/web") {
		c.Redirect(http.StatusMovedPermanently, c.Request.URL.Path+"/")
		return
	}

	rel := strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(c.Request.URL.Path, "/jellyfin"), "/emby"), "/")
	rel = strings.TrimPrefix(strings.TrimPrefix(rel, "web"), "/")

	name := path.Clean("/" + rel)
	if name == "/" {
		name = "/index.html"
	}

	file := filepath.Join(jfWebDir(), filepath.FromSlash(name))
	if st, err := os.Stat(file); err != nil || st.IsDir() {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if name == "/index.html" || name == "/config.json" {
		c.Header("Cache-Control", "no-cache")
	} else {
		c.Header("Cache-Control", "public, max-age=604800")
	}

	http.ServeFile(c.Writer, c.Request, file)
}

// jfWebRoot sends the server's root to the web client, like Jellyfin.
func jfWebRoot(c *gin.Context, _ map[string]string) {
	if !jfWebReady() {
		jfNoWebClient(c, nil)
		return
	}

	c.Redirect(http.StatusFound, "/jellyfin/web/")
}

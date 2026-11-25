package api

import (
	"context"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/net/webdav"
)

// WebDAV (/dav) shares the folders of the account's media groups read-only:
// the root has a folder per group, which shows the group's folder (or a
// folder per path when the group stores in several).

const davPrefix = "/dav"

// davMethods are the read-only WebDAV methods; the others are refused.
var davMethods = []string{http.MethodOptions, http.MethodGet, http.MethodHead, "PROPFIND", "LOCK", "UNLOCK"}

// davStarted is the time of the virtual folders.
var davStarted = time.Now()

// davLocks are shared by all requests: a lock lives longer than a request.
var davLocks = webdav.NewMemLS()

// HandleWebDAV answers the WebDAV requests of an account.
func HandleWebDAV(c *gin.Context) {
	if !slices.Contains(davMethods, c.Request.Method) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	// "Depth: infinity" would list the whole library in one answer.
	if c.Request.Method == "PROPFIND" && strings.EqualFold(c.GetHeader("Depth"), "infinity") {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}

	fsys := davFSOf(accessOf(c))

	if (c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead) && davPlaylist(c, fsys) {
		return
	}

	h := &webdav.Handler{Prefix: davPrefix, FileSystem: fsys, LockSystem: davLocks}

	if c.Request.Method == http.MethodGet {
		done, ok := streamGuard(c, "dav", strings.TrimPrefix(c.Request.URL.Path, davPrefix))
		if !ok {
			return
		}
		defer done()
	}

	h.ServeHTTP(c.Writer, c.Request)
}

// davPlaylist answers GET on a folder with an M3U playlist of its
// sub-folders (as playlists again) and media files, and reports whether
// it was a folder. Players without WebDAV support (VLC and mpv on the
// desktop) only GET the URL; WebDAV itself has no answer for a folder.
func davPlaylist(c *gin.Context, fsys *davFS) bool {
	name := strings.TrimPrefix(c.Request.URL.Path, davPrefix)

	fi, err := fsys.Stat(c.Request.Context(), name)
	if err != nil || !fi.IsDir() {
		return false
	}

	dir, err := fsys.OpenFile(c.Request.Context(), name, os.O_RDONLY, 0)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return true
	}

	entries, err := dir.Readdir(-1)
	dir.Close()

	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return true
	}

	entries = slices.DeleteFunc(entries, func(e fs.FileInfo) bool {
		if strings.HasPrefix(e.Name(), ".") {
			return true
		}

		ct := mediaContentType(e.Name())

		return !e.IsDir() && !strings.HasPrefix(ct, "video/") && !strings.HasPrefix(ct, "audio/")
	})
	slices.SortFunc(entries, func(a, b fs.FileInfo) int {
		if a.IsDir() != b.IsDir() {
			if a.IsDir() {
				return -1
			}

			return 1
		}

		return strings.Compare(strings.ToLower(a.Name()), strings.ToLower(b.Name()))
	})

	// Absolute paths: the player resolves them against the playlist's own
	// host, which is right behind a proxy too.
	base := davPrefix
	for seg := range strings.SplitSeq(strings.Trim(path.Clean("/"+name), "/"), "/") {
		if seg != "" {
			base += "/" + url.PathEscape(seg)
		}
	}

	var b strings.Builder

	b.WriteString("#EXTM3U\n")

	for _, e := range entries {
		entry := base + "/" + url.PathEscape(e.Name())
		if e.IsDir() {
			entry += "/"
		}

		b.WriteString("#EXTINF:-1," + strings.NewReplacer("\r", " ", "\n", " ").Replace(e.Name()) + "\n" + entry + "\n")
	}

	c.Header("Content-Disposition", `inline; filename="playlist.m3u"`)
	c.Data(http.StatusOK, "audio/x-mpegurl; charset=utf-8", []byte(b.String()))

	return true
}

// HandleWebDAVOptions answers OPTIONS without a login: that the server
// speaks WebDAV (class 2: locks, which Windows wants) and what it allows.
func HandleWebDAVOptions(c *gin.Context) {
	c.Header("Allow", strings.Join(davMethods, ", "))
	c.Header("DAV", "1, 2")
	c.Header("MS-Author-Via", "DAV")
	c.Header("Content-Length", "0")
	c.Status(http.StatusOK)
}

// davFS is the read-only tree of an account: virtual folders down to the
// group's folders, the real files below.
type davFS struct {
	// dirs are the virtual folders ("/", "/movie_EN" with several paths)
	// with their entries; roots the real folders by virtual path.
	dirs  map[string][]string
	roots map[string]string
}

// davFSOf builds the tree of the account's media groups.
func davFSOf(acc *userAccess) *davFS {
	fsys := &davFS{dirs: map[string][]string{"/": nil}, roots: map[string]string{}}

	for _, g := range groupsOf(acc) {
		media := config.GetSettingsMedia(g.Name)
		if media == nil {
			continue
		}

		var folders []struct{ name, dir string }

		for i := range media.Data {
			if p := media.Data[i].CfgPath; p != nil && p.Path != "" &&
				!slices.ContainsFunc(folders, func(f struct{ name, dir string }) bool { return f.dir == p.Path }) {
				folders = append(folders, struct{ name, dir string }{davName(p.Name), p.Path})
			}
		}

		if len(folders) == 0 {
			continue
		}

		name := "/" + davName(g.Name)
		fsys.dirs["/"] = append(fsys.dirs["/"], path.Base(name))

		if len(folders) == 1 {
			fsys.roots[name] = folders[0].dir
			continue
		}

		for _, f := range folders {
			fsys.dirs[name] = append(fsys.dirs[name], f.name)
			fsys.roots[name+"/"+f.name] = f.dir
		}
	}

	return fsys
}

// davName makes a config name usable as a folder name.
func davName(s string) string {
	return strings.NewReplacer("/", "_", "\\", "_").Replace(s)
}

// resolve returns the real file of a path ("" for a virtual folder) and
// whether the path exists.
func (d *davFS) resolve(name string) (string, bool) {
	name = path.Clean("/" + name)
	if _, ok := d.dirs[name]; ok {
		return "", true
	}

	for root, dir := range d.roots {
		if name == root {
			return dir, true
		}

		if rest, ok := strings.CutPrefix(name, root+"/"); ok {
			return filepath.Join(dir, filepath.FromSlash(rest)), true
		}
	}

	return "", false
}

func (d *davFS) Mkdir(context.Context, string, os.FileMode) error { return os.ErrPermission }
func (d *davFS) RemoveAll(context.Context, string) error          { return os.ErrPermission }
func (d *davFS) Rename(context.Context, string, string) error     { return os.ErrPermission }

func (d *davFS) OpenFile(_ context.Context, name string, flag int, _ os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		return nil, os.ErrPermission
	}

	real, ok := d.resolve(name)
	if !ok {
		return nil, os.ErrNotExist
	}

	if real == "" {
		clean := path.Clean("/" + name)
		return &davDir{name: path.Base(clean), entries: d.dirs[clean]}, nil
	}

	f, err := os.Open(real)
	if err != nil {
		return nil, err
	}

	if clean := path.Clean("/" + name); d.roots[clean] != "" {
		return davNamedFile{File: f, name: path.Base(clean)}, nil
	}

	return f, nil
}

// davNamedFile is a group's folder, opened under the group's name.
type davNamedFile struct {
	*os.File
	name string
}

func (f davNamedFile) Stat() (os.FileInfo, error) {
	fi, err := f.File.Stat()
	if err != nil {
		return nil, err
	}

	return davNamed{FileInfo: fi, name: f.name}, nil
}

func (d *davFS) Stat(_ context.Context, name string) (os.FileInfo, error) {
	real, ok := d.resolve(name)
	if !ok {
		return nil, os.ErrNotExist
	}

	clean := path.Clean("/" + name)
	if real == "" {
		return davDirInfo(path.Base(clean)), nil
	}

	fi, err := os.Stat(real)
	if err != nil {
		return nil, err
	}

	// A group's folder is shown under the group's name, not its own.
	if _, ok := d.roots[clean]; ok {
		return davNamed{FileInfo: fi, name: path.Base(clean)}, nil
	}

	return fi, nil
}

// davNamed is a real folder under another name.
type davNamed struct {
	os.FileInfo
	name string
}

func (n davNamed) Name() string { return n.name }

// davDir is a virtual folder.
type davDir struct {
	name    string
	entries []string
	read    bool
}

func (f *davDir) Close() error                   { return nil }
func (f *davDir) Read([]byte) (int, error)       { return 0, os.ErrInvalid }
func (f *davDir) Seek(int64, int) (int64, error) { return 0, os.ErrInvalid }
func (f *davDir) Write([]byte) (int, error)      { return 0, os.ErrPermission }
func (f *davDir) Stat() (os.FileInfo, error)     { return davDirInfo(f.name), nil }

func (f *davDir) Readdir(count int) ([]fs.FileInfo, error) {
	if f.read && count > 0 {
		return nil, io.EOF
	}

	f.read = true

	out := make([]fs.FileInfo, 0, len(f.entries))
	for _, e := range f.entries {
		out = append(out, davDirInfo(e))
	}

	return out, nil
}

// davDirInfo describes a virtual folder.
type davDirInfo string

func (i davDirInfo) Name() string       { return string(i) }
func (i davDirInfo) Size() int64        { return 0 }
func (i davDirInfo) Mode() fs.FileMode  { return fs.ModeDir | 0o555 }
func (i davDirInfo) ModTime() time.Time { return davStarted }
func (i davDirInfo) IsDir() bool        { return true }
func (i davDirInfo) Sys() any           { return nil }

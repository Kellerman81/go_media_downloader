package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/metadata"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
	"github.com/gin-gonic/gin"
)

// Covers of the Library page are loaded once from the provider and kept in
// the cover cache folder (cover_cache_dir) as <kind>-<id>.jpg. The JPEG
// names its source in a comment (coverSrcTag), so a changed poster is
// loaded again over it.

const (
	defaultCoverCacheDir = "./cache/covers"
	maxCoverBytes        = 8 << 20
)

// coverSources maps a library type to its db* table, cover column and the
// base URL of relative paths.
var coverSources = map[string]struct{ Table, Column, Base string }{
	"movie":     {"dbmovies", "poster", "https://image.tmdb.org/t/p/w342"},
	"series":    {"dbseries", "poster", "https://artworks.thetvdb.com/banners/"},
	"album":     {"dbalbums", "cover_url", ""},
	"audiobook": {"dbaudiobooks", "cover_url", ""},
	"book":      {"dbbooks", "cover_url", ""},
}

var (
	coverClient   = &http.Client{Timeout: 20 * time.Second}
	coverLocks    sync.Map // cache file -> *sync.Mutex, one download per file
	coverFailures sync.Map // cache file -> time of the last failed download
)

// coverRetryAfter is how long a failed download isn't retried, so a dead
// cover URL doesn't slow down every page of the grid.
const coverRetryAfter = time.Hour

// coverCacheDir returns the configured cover cache folder.
func coverCacheDir() string {
	if dir := config.GetSettingsGeneral().CoverCacheDir; dir != "" {
		return dir
	}

	return defaultCoverCacheDir
}

// coverPlaceholder is shown for items without a (loadable) cover.
const coverPlaceholder = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 200 300"><rect width="200" height="300" fill="#dee2e6"/>` +
	`<path d="M70 120h60v60H70z" fill="none" stroke="#868e96" stroke-width="6"/><circle cx="88" cy="140" r="7" fill="#868e96"/>` +
	`<path d="M76 174l20-22 14 14 8-8 12 16z" fill="#868e96"/></svg>`

// HandleLibraryCover serves the cover of a db* row (/admin/library/cover/:type/:id).
func HandleLibraryCover(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if _, ok := coverSources[c.Param("type")]; !ok || err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if k := libKindByType(c.Param("type")); k == nil || !accessOf(c).allowsDB(k, &id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	path, err := libraryCoverFile(c.Param("type"), id)
	if err != nil {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "image/svg+xml", []byte(coverPlaceholder))

		return
	}

	if c.Query("size") != "full" {
		path = coverThumb(path)
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(path)
}

// libraryCoverFile is the cached cover file of a db* row of a library type
// (movie, series, album, audiobook, book); errNoCover without one.
func libraryCoverFile(typ string, id uint64) (string, error) {
	src, ok := coverSources[typ]
	if !ok {
		return "", errNoCover
	}

	stored := database.Getdatarow[string](false,
		"SELECT COALESCE("+src.Column+", '') FROM "+src.Table+" WHERE id = ?", &id)

	sourceURL := calendarImageURL(&stored, src.Base)

	path, err := cachedCover(typ, id, sourceURL, sourceURL)
	if err != nil && sourceURL != "" {
		logger.Logtype("debug", 0).Err(err).Str("url", sourceURL).Msg("Cover not loadable")
	}

	// Without a loadable provider cover: the cover next to or inside the
	// downloaded files, then the user's Plex/Jellyfin.
	if err != nil {
		path, err = localCover(typ, &id)
	}

	if err != nil {
		path, err = mediaServerCover(typ, &id)
	}

	// Last resort for movies and series (e.g. site series without posters):
	// a frame of a downloaded file.
	if err != nil {
		path, err = frameCover(typ, &id)
	}

	if err != nil {
		return "", err
	}

	return path, nil
}

var errNoCover = errors.New("no cover")

// mediaServerCover loads a movie or series poster from the enabled Plex or
// Jellyfin source. The fetch URL holds the server token, so the cache key is
// the server name and only that is logged.
func mediaServerCover(kind string, id *uint64) (string, error) {
	var (
		imdb       string
		tmdb, tvdb int
		msKind     = "movie"
	)

	switch kind {
	case "movie":
		imdb = database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbmovies WHERE id = ?", id)
		tmdb = database.Getdatarow[int](false, "SELECT COALESCE(moviedb_id, 0) FROM dbmovies WHERE id = ?", id)
	case "series":
		msKind = "show"
		imdb = database.Getdatarow[string](false, "SELECT COALESCE(imdb_id, '') FROM dbseries WHERE id = ?", id)
		tvdb = database.Getdatarow[int](false, "SELECT COALESCE(thetvdb_id, 0) FROM dbseries WHERE id = ?", id)
	default:
		return "", errNoCover
	}

	posterURL, server := metadata.MediaServerPoster(msKind, imdb, tmdb, tvdb)
	if posterURL == "" {
		return "", errNoCover
	}

	path, err := cachedCover(kind, *id, server, posterURL)
	if err != nil {
		logger.Logtype("debug", 0).Err(errors.New(strings.ReplaceAll(err.Error(), posterURL, "<"+server+">"))).
			Str("source", server).Msg("Media server cover not loadable")
	}

	return path, err
}

// localCoverFiles are the table and metadata column of the files whose
// folder or tags may hold a cover.
var localCoverFiles = map[string][2]string{
	"album":     {"album_files", "dbalbum_id"},
	"audiobook": {"audiobook_files", "dbaudiobook_id"},
	"book":      {"book_files", "dbbook_id"},
}

// folderCoverNames are image names taken as the cover of a folder, best first.
var folderCoverNames = []string{"cover", "folder", "front", "albumart", "album", "poster"}

// localCover takes the cover of an album, audiobook or book from its first
// downloaded file: an image like cover.jpg in its folder, else the picture
// embedded in its tags.
func localCover(kind string, id *uint64) (string, error) {
	src, ok := localCoverFiles[kind]
	if !ok {
		return "", errNoCover
	}

	order := "id"
	if kind != "book" {
		order = "COALESCE(disc_number, 0), COALESCE(track_number, 0), location"
	}

	loc := database.Getdatarow[string](false,
		"SELECT location FROM "+src[0]+" WHERE "+src[1]+" = ? AND location != '' ORDER BY "+order+" LIMIT 1", id)
	if loc == "" {
		return "", errNoCover
	}

	return cachedCoverWith(kind, *id, "local:"+loc, func() ([]byte, string, error) {
		if img := folderCover(filepath.Dir(loc)); img != "" {
			f, err := os.Open(img)
			if err != nil {
				return nil, "", err
			}
			defer f.Close()

			data, err := readCover(f)

			return data, filepath.Ext(img), err
		}

		data, mimeType := tags.ReadCoverData(loc)
		if len(data) == 0 {
			return nil, "", errNoCover
		}

		return data, mimeType, nil
	})
}

// folderCover returns the cover image of a folder, if it has one.
func folderCover(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}

	best, bestRank := "", len(folderCoverNames)

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}

		stem := strings.ToLower(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		if rank := slices.Index(folderCoverNames, stem); rank >= 0 && rank < bestRank {
			best, bestRank = filepath.Join(dir, e.Name()), rank
		}
	}

	return best
}

// cachedCover returns the cache file of a cover, loading it from fetchURL
// first if needed. key names the source (coverSrcTag).
func cachedCover(kind string, id uint64, key, fetchURL string) (string, error) {
	if !strings.HasPrefix(fetchURL, "https://") && !strings.HasPrefix(fetchURL, "http://") {
		return "", errNoCover
	}

	return cachedCoverWith(kind, id, key, func() ([]byte, string, error) {
		return downloadCover(fetchURL)
	})
}

// cachedCoverWith returns the cache file of an item's cover from the source
// key, storing what load returns (the image and its content type or file
// extension) when the cache has none from that source; one load per item
// at a time, failures wait coverRetryAfter.
func cachedCoverWith(kind string, id uint64, key string, load func() ([]byte, string, error)) (string, error) {
	base := kind + "-" + strconv.FormatUint(id, 10)
	tag := coverSrcTag(key)
	dir := coverCacheDir()

	if path, ok := cachedFrom(dir, base, tag); ok {
		return path, nil
	}

	failKey := base + ":" + tag
	if failed, ok := coverFailures.Load(failKey); ok && time.Since(failed.(time.Time)) < coverRetryAfter {
		return "", errNoCover
	}

	lock, _ := coverLocks.LoadOrStore(base, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	if path, st, ok := cachedFileStat(dir, base); ok && coverSrcOf(path, st) == tag {
		return path, nil
	}

	data, typ, err := load()
	if err == nil {
		var path string
		if path, err = saveCover(dir, base, tag, data, typ); err == nil {
			return path, nil
		}
	}

	coverFailures.Store(failKey, time.Now())

	return "", err
}

// cachedFrom returns an item's cached cover when it is from the source tag
// (or names none: WebP); a large one stored before covers were shrunk is
// shrunk first.
func cachedFrom(dir, base, tag string) (string, bool) {
	path, st, ok := cachedFileStat(dir, base)
	if !ok {
		return "", false
	}

	if src := coverSrcOf(path, st); src != "" && src != tag {
		return "", false
	}

	if st.Size() > coverMaxBytes {
		path = shrinkCachedCover(dir, base, path)
	}

	return path, true
}

// shrinkCachedCover shrinks a large cover of the cache (one at a time per
// item), keeping the source it names.
func shrinkCachedCover(dir, base, path string) string {
	lock, _ := coverLocks.LoadOrStore(base, &sync.Mutex{})
	lock.(*sync.Mutex).Lock()
	defer lock.(*sync.Mutex).Unlock()

	p, st, ok := cachedFileStat(dir, base)
	if !ok || st.Size() <= coverMaxBytes {
		return path
	}

	if _, kept := keptCovers.Load(p); kept {
		return p
	}

	data, err := os.ReadFile(p)
	if err != nil {
		return p
	}

	out, err := saveCover(dir, base, coverSrcOf(p, st), data, filepath.Ext(p))
	if err != nil {
		return p
	}

	if st2, err := os.Stat(out); err == nil && st2.Size() >= st.Size() {
		keptCovers.Store(out, struct{}{})
	}

	return out
}

// readCover reads an image of at most maxCoverBytes.
func readCover(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxCoverBytes+1))
	if err == nil && len(data) > maxCoverBytes {
		err = errors.New("cover too large")
	}

	return data, err
}

// downloadCover loads an image and its content type.
func downloadCover(fetchURL string) ([]byte, string, error) {
	resp, err := coverClient.Get(fetchURL)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if resp.StatusCode != http.StatusOK || !strings.HasPrefix(contentType, "image/") {
		return nil, "", fmt.Errorf("status %d, type %q", resp.StatusCode, contentType)
	}

	data, err := readCover(resp.Body)

	return data, contentType, err
}

// storeCover writes an image into the cache folder as base with the
// extension of its type (a content type or file extension, coverExt).
func storeCover(dir, base, typeOrExt string, r io.Reader) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	tmp, err := os.CreateTemp(dir, base+"-*.tmp")
	if err != nil {
		return "", err
	}

	n, err := io.Copy(tmp, io.LimitReader(r, maxCoverBytes+1))
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}

	if err == nil && n > maxCoverBytes {
		err = errors.New("cover too large")
	}

	if err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	// CreateTemp makes it readable by the owner only: the cache is read
	// over the share too.
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	path := filepath.Join(dir, base+coverExt(typeOrExt))
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}

	return path, nil
}

// frameFiles are the files a movie or series cover frame is taken from,
// by the db* id.
var frameFiles = map[string]string{
	"movie":  "SELECT location FROM movie_files WHERE dbmovie_id = ? AND location != '' ORDER BY id LIMIT 1",
	"series": "SELECT location FROM serie_episode_files WHERE dbserie_id = ? AND location != '' ORDER BY id DESC LIMIT 1",
}

// frameCover takes a movie or series cover from a frame of a downloaded file.
func frameCover(kind string, id *uint64) (string, error) {
	q, ok := frameFiles[kind]
	if !ok {
		return "", errNoCover
	}

	loc := database.Getdatarow[string](false, q, id)
	if loc == "" {
		return "", errNoCover
	}

	return cachedCoverWith(kind, *id, "frame:"+loc, func() ([]byte, string, error) {
		data, err := grabFrame(loc)
		return data, ".jpg", err
	})
}

// HandleLibraryThumb serves a frame of an episode file as its thumbnail
// (/admin/library/thumb/episode/:id, a serie_episode_files ID).
func HandleLibraryThumb(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if c.Param("kind") != "episode" || err != nil || !accessOf(c).allowsFile(libKindByType("series"), &id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	path, err := episodeThumbFile(id)
	if err != nil {
		c.Header("Cache-Control", "public, max-age=3600")
		c.Data(http.StatusOK, "image/svg+xml", []byte(coverPlaceholder))

		return
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(path)
}

// episodeThumbFile is the thumbnail of an episode file (a
// serie_episode_files ID): the episode's image from TheTVDB, else a cached
// frame of the file.
func episodeThumbFile(id uint64) (string, error) {
	de := database.Getdatarow[uint64](false, "SELECT COALESCE(dbserie_episode_id, 0) FROM serie_episode_files WHERE id = ?", &id)
	if p, err := cachedColumnImage("episode", de, "SELECT COALESCE(poster, '') FROM dbserie_episodes WHERE id = ?", tvdbImageBase); err == nil {
		return p, nil
	}

	loc := database.Getdatarow[string](false, "SELECT location FROM serie_episode_files WHERE id = ?", &id)
	if loc == "" {
		return "", errNoCover
	}

	return cachedCoverWith("epthumb", id, "frame:"+loc, func() ([]byte, string, error) {
		data, err := grabFrame(loc)
		return data, ".jpg", err
	})
}

// tvdbImageBase is the base URL of TheTVDB's relative image paths.
const tvdbImageBase = "https://artworks.thetvdb.com/banners/"

// cachedColumnImage loads an image a db* column names (query by db ID),
// relative to base, into the cover cache.
func cachedColumnImage(kind string, db uint64, query, base string) (string, error) {
	if db == 0 {
		return "", errNoCover
	}

	stored := database.Getdatarow[string](false, query, &db)

	u := calendarImageURL(&stored, base)
	if u == "" {
		return "", errNoCover
	}

	return cachedCover(kind, db, u, u)
}

// frameSlots limits the ffmpeg frame grabs running at once: the first visit
// of a grid of site series asks for dozens.
var frameSlots = make(chan struct{}, 2)

// grabFrame takes a frame from 15% into a video (past intros and logos) as
// a JPEG.
func grabFrame(loc string) ([]byte, error) {
	frameSlots <- struct{}{}
	defer func() { <-frameSlots }()

	ffmpeg, err := tags.ResolveFFmpeg()
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	at := 30.0
	if p, err := probeMedia(ctx, loc); err == nil && p.Duration > 0 {
		if p.VCodec == "" {
			return nil, errNoCover
		}

		at = p.Duration * 0.15
	}

	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-loglevel", "error", "-ss", strconv.FormatFloat(at, 'f', 1, 64),
		"-i", loc, "-frames:v", "1", "-vf", "scale=480:-2", "-f", "image2", "-c:v", "mjpeg", "-q:v", "4", "pipe:1").Output()
	if err != nil {
		return nil, fmt.Errorf("frame of %s: %w", loc, err)
	}

	if len(out) == 0 {
		return nil, errNoCover
	}

	return out, nil
}

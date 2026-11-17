package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/gin-gonic/gin"
)

// The file cache cleanup removes cached covers, episode thumbnails and
// converted subtitles nothing refers to any more. Everything it removes is
// rebuilt on demand.

// cacheCleanupJob is the scheduler's name of the cleanup.
const cacheCleanupJob = "CleanupFileCache"

// tempMaxAge: temporary files and conversion folders older than this are
// left over from a crash or restart.
const tempMaxAge = time.Hour

// coverOwners are the tables whose rows a cached cover belongs to, by the
// kind in its file name (<kind>-<id>-<hash>.<ext>): the list entries of a
// db* row (the library, calendar and player only show those), or the
// episode file of a thumbnail.
var coverOwners = map[string]struct{ table, column string }{
	"movie":     {"movies", "dbmovie_id"},
	"series":    {"series", "dbserie_id"},
	"album":     {"albums", "dbalbum_id"},
	"audiobook": {"audiobooks", "dbaudiobook_id"},
	"book":      {"books", "dbbook_id"},
	"epthumb":   {"serie_episode_files", "id"},
}

func init() {
	utils.RegisterGlobalJob(cacheCleanupJob, func(ctx context.Context) error {
		_, err := CleanupFileCache(ctx)
		return err
	})
}

// @Summary      Clean up the file cache
// @Description  Removes cached covers and episode thumbnails no list entry or episode file has any more, converted subtitles of removed or replaced files, and leftover temporary files. Everything removed is rebuilt when needed. The request waits until done.
// @Tags         cache
// @Param        apikey query     string    true  "API key: the webapikey setting from [general] in config.toml"
// @Success      200  {object}  object  "Removed files: covers, subtitles, temp and bytes"
// @Failure      401  {object}  Jsonunauthorized  "Missing or wrong apikey"
// @Failure      500  {object}  Jsonerror  "The cache folder can't be read"
// @Router       /api/cache/cleanupfiles [get].
func apiCacheCleanupFiles(ctx *gin.Context) {
	res, err := CleanupFileCache(ctx)
	if err != nil {
		sendJSONError(ctx, http.StatusInternalServerError, err.Error())
		return
	}

	sendJSONResponse(ctx, http.StatusOK, res)
}

// cacheCleanupResult counts what a cleanup removed.
type cacheCleanupResult struct {
	Covers    int   `json:"covers"`
	Subtitles int   `json:"subtitles"`
	Temp      int   `json:"temp"`
	Bytes     int64 `json:"bytes"`
}

// CleanupFileCache removes orphaned cover and subtitle cache files.
func CleanupFileCache(ctx context.Context) (cacheCleanupResult, error) {
	var res cacheCleanupResult

	if err := cleanupCovers(ctx, &res); err != nil {
		return res, err
	}

	err := cleanupSubs(ctx, &res)

	logger.Logtype("info", 0).Int("covers", res.Covers).Int("subtitles", res.Subtitles).Int("temp", res.Temp).
		Int64("bytes", res.Bytes).Msg("File cache cleanup finished")

	return res, err
}

// removeCached deletes a cache file or folder and counts its size.
func removeCached(path string, info os.FileInfo, res *cacheCleanupResult) bool {
	size := info.Size()
	if info.IsDir() {
		size = 0
		filepath.WalkDir(path, func(_ string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() {
				if fi, err := d.Info(); err == nil {
					size += fi.Size()
				}
			}

			return nil
		})
	}

	if err := os.RemoveAll(path); err != nil {
		logger.Logtype("warn", 1).Err(err).Str("file", path).Msg("File cache cleanup: can't remove")
		return false
	}

	res.Bytes += size

	return true
}

// coverOwnerIDs returns the ids a kind's covers may belong to; ok is false
// when they couldn't be read completely (nothing is removed then).
func coverOwnerIDs(kind string) (map[uint64]struct{}, bool) {
	o := coverOwners[kind]
	query := "SELECT DISTINCT " + o.column + " FROM " + o.table + " WHERE " + o.column + " > 0"

	want := database.Getdatarow[uint](false, "SELECT count() FROM ("+query+")")
	ids := database.GetrowsN[uint64](false, want, query)

	if uint(len(ids)) != want {
		return nil, false
	}

	set := make(map[uint64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}

	return set, true
}

// parseCoverName splits <kind>-<id>-<hash>.<ext> (cachedCoverWith).
func parseCoverName(name string) (string, uint64, bool) {
	parts := strings.SplitN(strings.TrimSuffix(name, filepath.Ext(name)), "-", 3)
	if len(parts) != 3 || filepath.Ext(name) == ".tmp" {
		return "", 0, false
	}

	id, err := strconv.ParseUint(parts[1], 10, 64)

	return parts[0], id, err == nil
}

func cleanupCovers(ctx context.Context, res *cacheCleanupResult) error {
	entries, err := os.ReadDir(coverCacheDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	owners := map[string]map[uint64]struct{}{}
	skip := map[string]bool{}

	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}

		info, err := e.Info()
		if err != nil || e.IsDir() {
			continue
		}

		path := filepath.Join(coverCacheDir(), e.Name())

		// storeCover's partial downloads.
		if filepath.Ext(e.Name()) == ".tmp" {
			if time.Since(info.ModTime()) > tempMaxAge && removeCached(path, info, res) {
				res.Temp++
			}

			continue
		}

		kind, id, ok := parseCoverName(e.Name())
		if _, known := coverOwners[kind]; !ok || !known || skip[kind] {
			continue
		}

		set, loaded := owners[kind]
		if !loaded {
			if set, ok = coverOwnerIDs(kind); !ok {
				logger.Logtype("warn", 0).Str("type", kind).Msg("File cache cleanup: can't read the entries, covers kept")
				skip[kind] = true

				continue
			}

			owners[kind] = set
		}

		if _, used := set[id]; !used && removeCached(path, info, res) {
			coverIndex.remove(path)
			res.Covers++
		}
	}

	return nil
}

// subGroup is the cache of one subtitle source.
type subGroup struct {
	src   string   // from the .src file
	files []string // <key>-<mod ns>-<stream>.json
}

func cleanupSubs(ctx context.Context, res *cacheCleanupResult) error {
	dir := subCacheDir()

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	groups := map[string]*subGroup{}
	group := func(key string) *subGroup {
		if groups[key] == nil {
			groups[key] = &subGroup{}
		}

		return groups[key]
	}

	infos := map[string]os.FileInfo{}

	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}

		name, path := e.Name(), filepath.Join(dir, e.Name())
		infos[name] = info

		switch {
		case e.IsDir():
			// convertSubs' work folders.
			if strings.HasPrefix(name, "convert-") && time.Since(info.ModTime()) > tempMaxAge && removeCached(path, info, res) {
				res.Temp++
			}
		case filepath.Ext(name) == subSourceExt:
			if b, err := os.ReadFile(path); err == nil {
				group(strings.TrimSuffix(name, subSourceExt)).src = string(b)
			}
		case filepath.Ext(name) == ".json":
			key, _, ok := strings.Cut(name, "-")
			if !ok {
				// Named by a hash of everything before 2026-11-15: not
				// traceable to its file.
				if removeCached(path, info, res) {
					res.Subtitles++
				}

				continue
			}

			group(key).files = append(group(key).files, name)
		}
	}

	for key, g := range groups {
		if err := ctx.Err(); err != nil {
			return err
		}

		current := subCurrentPrefix(key, g.src)

		kept := 0

		for _, name := range g.files {
			if current != "" && strings.HasPrefix(name, current) {
				kept++
				continue
			}

			if removeCached(filepath.Join(dir, name), infos[name], res) {
				res.Subtitles++
			}
		}

		// The source note of a source without conversions; a fresh one may
		// belong to a conversion still running.
		srcName := key + subSourceExt
		if info := infos[srcName]; kept == 0 && info != nil && (len(g.files) > 0 || time.Since(info.ModTime()) > tempMaxAge) {
			removeCached(filepath.Join(dir, srcName), info, res)
		}
	}

	return nil
}

// subCurrentPrefix is the name prefix of the still valid conversions of a
// source ("" for none): the file exists unchanged and, unless it is a
// subtitle file next to a video, is a downloaded movie or episode.
func subCurrentPrefix(key, src string) string {
	if src == "" || subSourceKey(src) != key {
		return ""
	}

	st, err := os.Stat(src)
	if err != nil {
		return ""
	}

	if !subFileExts[strings.ToLower(filepath.Ext(src))] && !database.Getdatarow[bool](false,
		"SELECT EXISTS(SELECT 1 FROM movie_files WHERE location = ?) OR EXISTS(SELECT 1 FROM serie_episode_files WHERE location = ?)",
		&src, &src) {
		return ""
	}

	return key + "-" + strconv.FormatInt(st.ModTime().UnixNano(), 10) + "-"
}

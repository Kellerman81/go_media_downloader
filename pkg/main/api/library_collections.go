package api

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/tmdb"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/utils"
	"github.com/gin-gonic/gin"
)

// TMDB collections (film series): the Library's movie dialog shows the
// collection a movie belongs to, adds its missing movies to the movie's
// list and can keep adding new ones (movie_collections, checked daily).

const collectionCheckJob = "CheckMovieCollections"

func init() {
	utils.RegisterGlobalJob(collectionCheckJob, func(ctx context.Context) error {
		checkMonitoredCollections(ctx)

		return nil
	})
}

// collectionPart is a movie of a collection as the dialog shows it.
type collectionPart struct {
	TMDB     int    `json:"tmdb"`
	Title    string `json:"title"`
	Year     string `json:"year"`
	Have     bool   `json:"have"`     // in a list of the movie's media group
	Excluded bool   `json:"excluded"` // deleted with "don't add again"
}

// groupHasTMDB reports whether a list of the media group has the movie.
func groupHasTMDB(cfgp *config.MediaTypeConfig, tmdbID int) bool {
	if len(cfgp.Lists) == 0 {
		return false
	}

	args := []any{tmdbID}
	for i := range cfgp.Lists {
		args = append(args, cfgp.Lists[i].Name)
	}

	return database.Getdatarow[bool](false, "SELECT EXISTS(SELECT 1 FROM movies m INNER JOIN dbmovies d ON d.id = m.dbmovie_id "+
		"WHERE d.moviedb_id = ? AND m.listname COLLATE NOCASE IN (?"+strings.Repeat(", ?", len(cfgp.Lists)-1)+"))", args...)
}

// groupExcludesTMDB reports whether the media group excluded the movie.
func groupExcludesTMDB(cfgp *config.MediaTypeConfig, tmdbID int) bool {
	return database.Getdatarow[bool](false, "SELECT EXISTS(SELECT 1 FROM import_exclusions e INNER JOIN dbmovies d ON d.id = e.db_id "+
		"WHERE e.kind = 'movie' AND e.media_group = ? COLLATE NOCASE AND d.moviedb_id = ?)", cfgp.NamePrefix, tmdbID)
}

// movieCollectionOf returns the TMDB collection of a dbmovies row and the
// media group and list index of its list entry.
func movieCollectionOf(ctx context.Context, dbID *uint64, listname string) (*tmdb.MovieCollection, *config.MediaTypeConfig, int, error) {
	p := providers.GetTMDB()
	cfgp := mediaConfigOfList(listname)

	if p == nil || cfgp == nil {
		return nil, nil, -1, nil
	}

	listid := slices.IndexFunc(cfgp.Lists, func(l config.MediaListsConfig) bool { return strings.EqualFold(l.Name, listname) })

	row := database.StructscanT[struct {
		TMDB int  `db:"moviedb_id"`
		Coll *int `db:"collection_id"` // nil: not looked up yet
	}](false, 1, "SELECT COALESCE(moviedb_id, 0) AS moviedb_id, collection_id FROM dbmovies WHERE id = ?", dbID)
	if len(row) == 0 || row[0].TMDB == 0 {
		return nil, cfgp, listid, nil
	}

	// The import saves the collection; a movie from before that is looked
	// up once here and saved.
	var collID int
	if row[0].Coll != nil {
		collID = *row[0].Coll
	} else {
		id, name, err := p.GetMovieCollectionID(ctx, row[0].TMDB)
		if err != nil {
			return nil, cfgp, listid, err
		}

		collID = id
		database.ExecN("UPDATE dbmovies SET collection_id = ?, collection_name = ? WHERE id = ?", &collID, &name, dbID)
	}

	if collID == 0 {
		return nil, cfgp, listid, nil
	}

	coll, err := p.GetCollection(ctx, collID)

	return coll, cfgp, listid, err
}

// HandleLibraryCollection describes the collection of a movie
// (/admin/library/collection?id=<dbmovies id>&list=), 204 without one.
func HandleLibraryCollection(c *gin.Context) {
	id, err := strconv.ParseUint(c.Query("id"), 10, 64)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	coll, cfgp, _, err := movieCollectionOf(ctx, &id, c.Query("list"))
	if err != nil || coll == nil {
		c.Status(http.StatusNoContent)
		return
	}

	saveCollectionParts(coll)

	parts := make([]collectionPart, 0, len(coll.Parts))
	for _, p := range coll.Parts {
		year, _, _ := strings.Cut(p.ReleaseDate, "-")
		parts = append(parts, collectionPart{TMDB: p.ID, Title: p.Title, Year: year,
			Have: groupHasTMDB(cfgp, p.ID), Excluded: groupExcludesTMDB(cfgp, p.ID)})
	}

	c.JSON(http.StatusOK, gin.H{"id": coll.ID, "name": coll.Name, "parts": parts,
		"monitored": database.CollectionMonitored(coll.ID, c.Query("list"))})
}

// saveCollectionParts saves the collection of its movies in the database
// that aren't looked up yet: one dialog fills in the whole film series.
func saveCollectionParts(coll *tmdb.MovieCollection) {
	if len(coll.Parts) == 0 {
		return
	}

	args := []any{coll.ID, coll.Name}
	for _, p := range coll.Parts {
		args = append(args, p.ID)
	}

	database.ExecN("UPDATE dbmovies SET collection_id = ?, collection_name = ? WHERE collection_id IS NULL AND moviedb_id IN (?"+
		strings.Repeat(", ?", len(coll.Parts)-1)+")", args...)
}

// collectionAdding keeps one background add per collection and list.
var collectionAdding sync.Map

// HandleLibraryCollectionAdd adds the missing movies of a movie's
// collection to its list (add=1, in the background) and switches adding
// new ones on or off (monitor=1/0) (/admin/library/collection/add).
func HandleLibraryCollectionAdd(c *gin.Context) {
	id, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	listname := c.PostForm("list")

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown movie"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()

	coll, cfgp, listid, err := movieCollectionOf(ctx, &id, listname)
	if err != nil || coll == nil || listid < 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "no TMDB collection for this movie"})
		return
	}

	if m := c.PostForm("monitor"); m != "" {
		if err := database.MonitorCollection(coll.ID, coll.Name, cfgp.Lists[listid].Name, m == "1"); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	}

	missing := 0

	if c.PostForm("add") == "1" {
		for _, p := range coll.Parts {
			if !groupHasTMDB(cfgp, p.ID) && !groupExcludesTMDB(cfgp, p.ID) {
				missing++
			}
		}

		if missing > 0 {
			key := strconv.Itoa(coll.ID) + "|" + strings.ToLower(listname)
			if _, running := collectionAdding.LoadOrStore(key, true); !running {
				go func() {
					defer collectionAdding.Delete(key)

					addCollectionParts(context.Background(), coll, cfgp, listid)
				}()
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{"adding": missing})
}

// addCollectionParts adds the collection's movies the media group doesn't
// have (and didn't exclude) to the list; it returns how many it added.
func addCollectionParts(ctx context.Context, coll *tmdb.MovieCollection, cfgp *config.MediaTypeConfig, listid int) int {
	p := providers.GetTMDB()
	if p == nil {
		return 0
	}

	added := 0

	for _, part := range coll.Parts {
		if ctx.Err() != nil || groupHasTMDB(cfgp, part.ID) || groupExcludesTMDB(cfgp, part.ID) {
			continue
		}

		ids, err := p.GetMovieExternalIDs(ctx, part.ID)
		if err != nil || ids == nil || ids.IMDbID == "" {
			logger.Logtype("debug", 0).Int("tmdb", part.ID).Str("title", part.Title).Msg("Collection: movie without IMDb ID skipped")
			continue
		}

		if _, err := importfeed.JobImportMovies(ids.IMDbID, cfgp, listid, true, false); err != nil {
			logger.Logtype("warn", 0).Err(err).Str("imdb", ids.IMDbID).Str("collection", coll.Name).Msg("Collection: movie not added")
			continue
		}

		added++
	}

	logger.Logtype("info", 0).Str("collection", coll.Name).Str("list", cfgp.Lists[listid].Name).Int("added", added).
		Msg("Collection: movies added")

	return added
}

// checkMonitoredCollections adds the new movies of the monitored
// collections.
func checkMonitoredCollections(ctx context.Context) {
	p := providers.GetTMDB()
	if p == nil {
		return
	}

	for _, m := range database.GetMonitoredCollections() {
		if ctx.Err() != nil {
			return
		}

		cfgp := mediaConfigOfList(m.Listname)
		if cfgp == nil {
			continue
		}

		listid := slices.IndexFunc(cfgp.Lists, func(l config.MediaListsConfig) bool { return strings.EqualFold(l.Name, m.Listname) })

		coll, err := p.GetCollection(ctx, m.CollectionID)
		if err != nil || listid < 0 {
			logger.Logtype("warn", 0).Err(err).Str("collection", m.Name).Msg("Collection: check failed")
			continue
		}

		addCollectionParts(ctx, coll, cfgp, listid)
		database.TouchCollectionCheck(&m.ID)
	}
}

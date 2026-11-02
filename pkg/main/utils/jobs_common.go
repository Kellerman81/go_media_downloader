package utils

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/structure"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// SQL query constants to avoid repeated string allocations.
const (
	// COLLATE NOCASE matches querySelectMovieByImdb (jobs_movies.go) - without
	// it, a listname differing only in case from list.Name (config rename,
	// casing drift) makes a just-matched book ID stay 0, so the file is
	// dropped as unmatched even though the DB row was found.
	querySelectBookByDbID = "select id from books where dbbook_id = ? and listname = ? COLLATE NOCASE"
	// querySelectImdbByDbmovie   = "select imdb_id from dbmovies where id = ?".
)

// Error constants for common error conditions.
var (
	errNoDataConfig = errors.New("no data configuration found")
	errListNotFound = errors.New("list template not found")
)

// MediaQualityItem represents a media item with quality info for checkreached operations.
// This is a common struct used by both movies and series.
type MediaQualityItem struct {
	ID             uint   `db:"id"`
	QualityReached bool   `db:"quality_reached"`
	QualityProfile string `db:"quality_profile"`
}

// clearHistory clears the download history for a specific list.
// This is a unified function that works for both movies and series.
func clearHistory(cfgp *config.MediaTypeConfig, listid int) {
	query := mtstrings.GetStringsMap(cfgp.IsType, "ClearHistoryByList")
	if query == "" {
		return
	}

	database.ExecN(query, &cfgp.Lists[listid].Name)
}

// checkreachedflag checks if the quality cutoff has been reached for all media items in the given list config.
// It queries the media table for the list, checks the priority of existing files against the config quality cutoff,
// and updates the quality_reached flag in the database accordingly.
// This is a unified function that works for movies, series, music, audiobooks, and books.
func checkreachedflag(
	rootctx context.Context,
	cfgp *config.MediaTypeConfig,
	listcfg *config.MediaListsConfig,
) error {
	queryCount := mtstrings.GetStringsMap(cfgp.IsType, "QueryMediaCountByList")
	querySelect := mtstrings.GetStringsMap(cfgp.IsType, "QueryMediaByList")
	updateQueryReached := mtstrings.GetStringsMap(cfgp.IsType, "UpdateQualityReached")

	if queryCount == "" || querySelect == "" || updateQueryReached == "" {
		return nil
	}

	count := database.Getdatarow[uint](false, queryCount, &listcfg.Name)
	if count == 0 {
		return nil
	}

	arr := database.StructscanT[MediaQualityItem](false, count, querySelect, &listcfg.Name)

	// Check if this is an audio media type (music, audiobooks, books)
	isAudioType := cfgp.IsType == config.MediaTypeMusic ||
		cfgp.IsType == config.MediaTypeAudiobook ||
		cfgp.IsType == config.MediaTypeBook

	var minPrio int
	for idx := range arr {
		if err := logger.CheckContextEnded(rootctx); err != nil {
			return err
		}

		if !config.CheckGroup("quality_", arr[idx].QualityProfile) {
			logger.Logtype("debug", 1).
				Uint(logger.StrID, arr[idx].ID).
				Str("type", mediatype.GetCategoryName(cfgp.IsType)).
				Msg("Quality not found")

			continue
		}

		qualCfg := config.GetSettingsQuality(arr[idx].QualityProfile)

		// Use audio priority function for audio media types
		if isAudioType {
			minPrio, _ = searcher.GetpriobyfilesAudio(
				cfgp.IsType,
				&arr[idx].ID,
				false,
				-1,
				qualCfg,
				false,
			)
		} else {
			minPrio, _ = searcher.Getpriobyfiles(
				cfgp.IsType,
				&arr[idx].ID,
				false,
				-1,
				qualCfg,
				false,
			)
		}

		reached := minPrio >= qualCfg.CutoffPriority

		// Only update if the state needs to change
		if reached && !arr[idx].QualityReached {
			database.ExecN(updateQueryReached, 1, &arr[idx].ID)
		} else if !reached && arr[idx].QualityReached {
			database.ExecN(updateQueryReached, 0, &arr[idx].ID)
		}
	}

	return nil
}

// jobImportParseCommon handles the common file import/parse logic for both movies and series.
// It parses the video file, calculates priority, inserts file records, updates status flags,
// and handles caching. Type-specific logic is handled via StringsMap queries and the handler.
func jobImportParseCommon(
	ctx context.Context,
	m *database.ParseInfo,
	pathv string,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	addfound bool,
) error {
	if m == nil {
		return logger.ErrNotFound
	}

	if list == nil || list.CfgQuality == nil {
		return logger.ErrListnameEmpty
	}

	if len(cfgp.Data) == 0 {
		return errNoDataConfig
	}

	// Get media ID and validate - movies use MovieID, series use SerieID
	mediaID := mediatype.GetMediaID(cfgp.IsType, m)
	dbMediaID := mediatype.GetDBID(cfgp.IsType, m)

	// For series, also need to get episodes to import
	if cfgp.IsType == config.MediaTypeSeries {
		if m.DbserieID == 0 || m.SerieID == 0 {
			m.TempTitle = pathv
			m.AddUnmatched(cfgp, &list.Name, logger.ErrNotFoundDbserie)
			return logger.ErrNotFoundDbserie
		}
	} else if mediaID == 0 && (cfgp.IsType != config.MediaTypeMovie || !addfound) {
		// Movies with addfound fall through to the add-found import below.
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name, logger.ErrNotFound)
		return logger.ErrNotFoundMovie
	}

	switch cfgp.IsType {
	case config.MediaTypeMovie:
		{
			if m.MovieID == 0 && addfound {
				if m.Imdb != "" {
					if getdbmovieidbyimdb(m, cfgp, list) {
						return logger.ErrNotFoundMovie
					}
				}

				if m.Imdb == "" {
					importfeed.MovieFindImdbIDByTitle(cfgp.Data[0].AddFound, m, cfgp)
				}

				var bl bool
				if m.Imdb != "" {
					m.MovieFindDBIDByImdbParser()

					if m.DbmovieID != 0 {
						bl = true
					}
				}

				if bl && list.Name == cfgp.Data[0].AddFoundList && cfgp.Data[0].AddFound {
					database.Scanrowsdyn(
						false,
						database.QueryMoviesGetIDByDBIDListname,
						&m.MovieID,
						&m.DbmovieID,
						&list.Name,
					)

					if m.MovieID == 0 {
						if m.Imdb == "" {
							if config.GetSettingsGeneral().UseMediaCache {
								m.CacheThreeStringIntIndexFuncGetImdb()
							} else {
								database.Scanrowsdyn(
									false,
									"select imdb_id from dbmovies where id = ?",
									&m.Imdb,
									&m.DbmovieID,
								)
							}
						}

						if m.Imdb != "" {
							if getdbmovieidbyimdb(m, cfgp, list) {
								return logger.ErrNotFoundMovie
							}
						}
					}
				} else if list.Name == cfgp.Data[0].AddFoundList && cfgp.Data[0].AddFound {
					importfeed.MovieFindImdbIDByTitle(cfgp.Data[0].AddFound, m, cfgp)

					var err error
					if m.DbmovieID == 0 {
						m.DbmovieID, err = importfeed.JobImportMovies(
							m.Imdb,
							cfgp,
							cfgp.GetMediaListsEntryListID(list.Name),
							true,
							false,
						)
					}

					if err != nil && m.MovieID == 0 {
						database.Scanrowsdyn(
							false,
							database.QueryMoviesGetIDByDBIDListname,
							&m.MovieID,
							&m.DbmovieID,
							&list.Name,
						)

						if m.MovieID == 0 {
							err = logger.ErrNotFoundMovie
						}
					}

					if err != nil {
						return err
					}
				}
			}

			if m.MovieID == 0 {
				m.TempTitle = pathv
				m.AddUnmatched(cfgp, &list.Name, logger.ErrNotFoundMovie)
				return logger.ErrNotFoundMovie
			}
		}

	case config.MediaTypeSeries:
		{
			err := m.Getepisodestoimport()
			if err != nil || len(m.Episodes) == 0 {
				m.TempTitle = pathv
				m.AddUnmatched(cfgp, &list.Name, err)
				return err
			}
		}
	}

	// Common parsing logic
	parser.GetPriorityMapQual(m, cfgp, list.CfgQuality, true, false)

	m.File = pathv

	// Only parse video file for video media types (movies, series)
	// Books don't have video content; music/audiobooks are audio-only
	if cfgp.IsType == config.MediaTypeMovie || cfgp.IsType == config.MediaTypeSeries {
		if err := parser.ParseVideoFile(ctx, m, list.CfgQuality); err != nil {
			return err
		}
	}

	// Calculate quality reached
	var reached int
	if m.Priority >= list.CfgQuality.CutoffPriority {
		reached = 1
	}

	basefile := filepath.Base(pathv)
	extfile := filepath.Ext(pathv)

	// Get queries from StringsMap
	insertQuery := mtstrings.GetStringsMap(cfgp.IsType, "InsertFile")
	updateMissing := mtstrings.GetStringsMap(cfgp.IsType, "UpdateMissingByID")
	updateReached := mtstrings.GetStringsMap(cfgp.IsType, "UpdateQualityReachedByID")
	deleteUnmatched := mtstrings.GetStringsMap(cfgp.IsType, "DeleteUnmatchedByPath")
	selectRootpath := mtstrings.GetStringsMap(cfgp.IsType, "SelectRootpath")

	// Determine quality profile name to use (series uses list name, others use quality config name)
	qualityProfileName := list.CfgQuality.Name
	if mediatype.UsesListNameAsQualityProfile(cfgp.IsType) {
		qualityProfileName = list.Name
	}

	switch cfgp.IsType {
	case config.MediaTypeMovie:
		{ // Movies: single insert
			m.TempTitle = pathv

			// Update rootpath if empty
			if mediaID != 0 && database.Getdatarow[string](false, selectRootpath, &mediaID) == "" {
				structure.UpdateRootpath(pathv, "movies", &mediaID, cfgp)
			}

			database.ExecN(insertQuery,
				&m.TempTitle, &basefile, &extfile, &qualityProfileName,
				&m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID,
				&m.Proper, &m.Repack, &m.Extended,
				&mediaID, &dbMediaID,
				&m.Height, &m.Width,
			)
			database.ExecN(updateMissing, &mediaID)
			database.ExecN(updateReached, &reached, &mediaID)
		}

	case config.MediaTypeSeries:
		{ // Series: loop over episodes
			countQuery := mtstrings.GetStringsMap(cfgp.IsType, "CountFileByLocationAndID")
			updateProfile := mtstrings.GetStringsMap(cfgp.IsType, "UpdateQualityProfileByID")

			var count uint
			for idx := range m.Episodes {
				database.Scanrowsdyn(false, countQuery, &count, &m.File, &m.Episodes[idx].Num1)

				if count >= 1 {
					continue
				}

				database.ExecN(insertQuery,
					&m.File, &basefile, &extfile, &qualityProfileName,
					&m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID,
					&m.Proper, &m.Repack, &m.Extended,
					&m.SerieID, &m.Episodes[idx].Num1, &m.Episodes[idx].Num2, &m.DbserieID,
					&m.Height, &m.Width,
				)

				database.ExecN(updateMissing, &m.Episodes[idx].Num1)
				database.ExecN(updateReached, &reached, &m.Episodes[idx].Num1)

				if qualityProfileName != "" {
					database.ExecN(updateProfile, &qualityProfileName, &m.Episodes[idx].Num1)
				}
				// deleteUnmatched already runs once, unconditionally, for
				// this path after the switch below - no need to repeat it
				// per episode for a multi-episode file.
			}
		}
	}

	database.ExecN(deleteUnmatched, pathv)

	// Update caches
	if config.GetSettingsGeneral().UseMediaCache || config.GetSettingsGeneral().UseFileCache {
		database.SlicesCacheContainsDelete(mediatype.GetCacheUnmatchedKey(cfgp.IsType), pathv)
		database.AppendCache(mediatype.GetCacheFilesKey(cfgp.IsType), pathv)
	}

	// Update rootpath for series (after loop)
	if cfgp.IsType == config.MediaTypeSeries && m.SerieID != 0 {
		if database.Getdatarow[string](false, selectRootpath, &m.SerieID) == "" {
			structure.UpdateRootpath(pathv, "series", &m.SerieID, cfgp)
		}
	}

	return nil
}

// jobImportParseBook handles book file import without video/audio parsing.
// Books are single-file media that only need filename parsing and database matching.
func jobImportParseBook(
	m *database.ParseInfo,
	pathv string,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	addfound bool,
) error {
	if m == nil {
		return logger.ErrNotFound
	}

	if list == nil || list.CfgQuality == nil {
		return logger.ErrListnameEmpty
	}

	if len(cfgp.Data) == 0 {
		return errNoDataConfig
	}

	// Try to find the book in the database by title
	if m.BookID == 0 {
		m.FindDbbookByTitle()

		if m.DbbookID != 0 {
			// Found in dbbooks, now check if tracked
			database.Scanrowsdyn(
				false,
				querySelectBookByDbID,
				&m.BookID,
				&m.DbbookID,
				&list.Name,
			)
		}
	}

	// If still not found and addfound is enabled, try to import
	if m.BookID == 0 && addfound && m.ISBN != "" {
		// Try to import by ISBN
		dbID, importErr := importfeed.JobImportBooks(
			context.Background(),
			m.ISBN,
			cfgp,
			cfgp.GetMediaListsEntryListID(list.Name),
			true,
		)
		if importErr == nil && dbID != 0 {
			m.DbbookID = dbID
			// Check if now tracked
			database.Scanrowsdyn(
				false,
				querySelectBookByDbID,
				&m.BookID,
				&m.DbbookID,
				&list.Name,
			)
		}
	}

	if m.BookID == 0 {
		m.TempTitle = pathv
		m.AddUnmatched(cfgp, &list.Name, logger.ErrNotFoundBook)
		return logger.ErrNotFoundBook
	}

	// Calculate priority for books (format-based)
	parser.GetPriorityMapQual(m, cfgp, list.CfgQuality, true, false)

	m.File = pathv

	// Calculate quality reached (format-only for books, matching the
	// existing-file score the cutoff is checked against on rescans)
	var reached int
	if parser.ComparablePriority(m, cfgp) >= list.CfgQuality.CutoffPriority {
		reached = 1
	}

	basefile := filepath.Base(pathv)
	extfile := filepath.Ext(pathv)

	// Get queries from StringsMap
	insertQuery := mtstrings.GetStringsMap(cfgp.IsType, "InsertFile")
	updateMissing := mtstrings.GetStringsMap(cfgp.IsType, "UpdateMissingByID")
	updateReached := mtstrings.GetStringsMap(cfgp.IsType, "UpdateQualityReachedByID")
	deleteUnmatched := mtstrings.GetStringsMap(cfgp.IsType, "DeleteUnmatchedByPath")
	selectRootpath := mtstrings.GetStringsMap(cfgp.IsType, "SelectRootpath")

	qualityProfileName := list.CfgQuality.Name

	// Update rootpath if empty
	if m.BookID != 0 && database.Getdatarow[string](false, selectRootpath, &m.BookID) == "" {
		structure.UpdateRootpath(pathv, "books", &m.BookID, cfgp)
	}

	// Insert file record
	database.ExecN(insertQuery,
		&pathv, &basefile, &extfile, &qualityProfileName,
		&m.BookID, &m.DbbookID,
	)
	database.ExecN(updateMissing, &m.BookID)
	database.ExecN(updateReached, &reached, &m.BookID)
	database.ExecN(deleteUnmatched, pathv)

	// Update caches
	if config.GetSettingsGeneral().UseMediaCache || config.GetSettingsGeneral().UseFileCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedBook, pathv)
		database.AppendCache(logger.CacheFilesBook, pathv)
	}

	return nil
}

// ImportFeedsWithURL runs the feeds import for a single list using overrideURL as the
// fetch URL instead of the URL stored in the list config.
// A shallow copy of ListsConfig is made so the shared config is never mutated.
func ImportFeedsWithURL(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	listid int,
	overrideURL string,
) error {
	cfgCopy := *list.CfgList

	cfgCopy.URL = overrideURL

	listCopy := *list

	listCopy.CfgList = &cfgCopy

	return importnewsingle(ctx, cfgp, &listCopy, listid)
}

func importnewsingle(
	ctx context.Context, cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	listid int,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	logger.Logtype("info", 2).
		Str(logger.StrConfig, cfgp.NamePrefix).
		Str(logger.StrListname, list.Name).
		Msg("get feeds for")

	if list.CfgList == nil {
		return errListNotFound
	}

	if !list.Enabled || !list.CfgList.Enabled {
		return logger.ErrDisabled
	}

	feed, err := Feeds(ctx, cfgp, list, false)
	if err != nil {
		feed.requeueIRC(false)
		plfeeds.Put(feed)

		return err
	}
	defer plfeeds.Put(feed)

	// IRC announce lines drained by Feeds are only gone for good once the
	// import ran to completion; otherwise the next run gets them again.
	completed := false

	defer func() {
		if !completed { // panicking
			feed.requeueIRC(true)
		}
	}()

	err = importFeedEntries(ctx, cfgp, list, listid, feed)
	completed = true

	if err == nil && ctx.Err() == nil {
		feed.commitIRC()
	} else {
		feed.requeueIRC(false)
	}

	return err
}

// importFeedEntries imports the entries of a fetched feed into list listid.
func importFeedEntries(
	ctx context.Context, cfgp *config.MediaTypeConfig,
	list *config.MediaListsConfig,
	listid int,
	feed *feedResults,
) error {
	listnamefilter := list.Getlistnamefilterignore()

	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for idx := range list.IgnoreMapLists {
		args.Arr = append(args.Arr, &list.IgnoreMapLists[idx])
	}

	// Aggregate per-entry outcomes so one summary line is logged per run.
	ctx, summary := importfeed.WithImportSummary(ctx)

	pl := worker.WorkerPoolParse().NewGroupContext(ctx)
	// Submitted tasks read feed entries; early returns must not hand the feed
	// back to the pool (deferred in importnewsingle) while they are still running.
	defer func() { _ = pl.Wait() }()

	switch cfgp.IsType {
	case config.MediaTypeMovie:
		{
			if len(feed.Movies) == 0 {
				return nil
			}

			var existing []uint
			if !config.GetSettingsGeneral().UseMediaCache && listnamefilter != "" {
				existing = database.GetrowsNuncached[uint](
					database.Getdatarow[uint](
						false,
						logger.JoinStrings("select count() from movies where "+listnamefilter),
						args.Arr...,
					),
					logger.JoinStrings("select dbmovie_id from movies where "+listnamefilter),
					args.Arr,
				)
			}

			var (
				allowed bool
				movieid uint
				getid   uint
			)

			for idx := range feed.Movies {
				if feed.Movies[idx] == "" {
					continue
				}

				if err := logger.CheckContextEnded(ctx); err != nil {
					return err
				}

				if !logger.HasPrefixI(feed.Movies[idx], "tt") {
					feed.Movies[idx] = logger.AddImdbPrefix(feed.Movies[idx])
				}

				movieid = importfeed.MovieFindDBIDByImdb(&feed.Movies[idx])

				if movieid != 0 {
					if config.GetSettingsGeneral().UseMediaCache {
						if database.CacheOneStringTwoIntIndexFunc(
							logger.CacheMovie,
							func(elem *syncops.DbstaticOneStringTwoInt) bool {
								return elem.Num1 == movieid &&
									(elem.Str == list.Name || strings.EqualFold(elem.Str, list.Name))
							},
						) {
							continue
						}
					} else if database.Scanrowsdyn(false, database.QueryCountMoviesByDBIDList, &getid, &movieid, &list.Name); getid >= 1 {
						continue
					}

					if list.IgnoreMapListsLen >= 1 {
						if config.GetSettingsGeneral().UseMediaCache {
							if database.CacheOneStringTwoIntIndexFunc(
								logger.CacheMovie,
								func(elem *syncops.DbstaticOneStringTwoInt) bool {
									return elem.Num1 == movieid &&
										logger.SlicesContainsI(list.IgnoreMapLists, elem.Str)
								},
							) {
								continue
							}
						} else {
							if slices.Contains(existing, movieid) {
								continue
							}
						}
					}
				}

				allowed, _ = importfeed.AllowMovieImport(&feed.Movies[idx], list.CfgList)
				if allowed {
					pl.Submit(func() {
						defer logger.HandlePanic()

						importfeed.JobImportMoviesByList(
							ctx,
							feed.Movies[idx],
							idx,
							cfgp,
							listid,
							true,
							false,
						)
					})
				} else {
					logger.Logtype("debug", 1).
						Str(logger.StrImdb, feed.Movies[idx]).
						Msg("not allowed movie")
				}
			}
		}

	case config.MediaTypeSeries:
		{
			if len(feed.Series) == 0 {
				return nil
			}

			for idxserie2 := range feed.Series {
				pl.Submit(func() {
					defer logger.HandlePanic()

					importfeed.JobImportDBSeries(
						ctx,
						&feed.Series[idxserie2],
						idxserie2,
						cfgp,
						listid,
					)
				})
			}
		}

	case config.MediaTypeBook:
		{
			if len(feed.Books) == 0 {
				return nil
			}

			for idx := range feed.Books {
				pl.Submit(func() {
					defer logger.HandlePanic()

					importfeed.JobImportDBBook(ctx, &feed.Books[idx], idx, cfgp, listid)
				})
			}
		}

	case config.MediaTypeAudiobook:
		{
			logger.Logtype("debug", 1).
				Int("audiobook_count", len(feed.Audiobooks)).
				Str("config", cfgp.NamePrefix).
				Int("listid", listid).
				Msg("Processing audiobook feed")

			if len(feed.Audiobooks) == 0 {
				logger.Logtype("warn", 1).
					Str("config", cfgp.NamePrefix).
					Msg("No audiobooks found in feed - check config file format")
				return nil
			}

			for idx := range feed.Audiobooks {
				logger.Logtype("debug", 2).
					Int("idx", idx).
					Str("name", feed.Audiobooks[idx].Name).
					Str("author", feed.Audiobooks[idx].AuthorName).
					Msg("Submitting audiobook import job")
				pl.Submit(func() {
					defer logger.HandlePanic()

					importfeed.JobImportDBAudiobook(ctx, &feed.Audiobooks[idx], idx, cfgp, listid)
				})
			}
		}

	case config.MediaTypeMusic:
		{
			if len(feed.Albums) == 0 {
				return nil
			}

			for idx := range feed.Albums {
				pl.Submit(func() {
					defer logger.HandlePanic()

					importfeed.JobImportDBAlbum(ctx, &feed.Albums[idx], idx, cfgp, listid)
				})
			}
		}
	}

	errjobs := pl.Wait()
	if errjobs != nil {
		logger.Logtype("error", 0).
			Err(errjobs).
			Msg("Error importing")
	}

	summary.Log(cfgp.NamePrefix, list.Name)

	return nil
}

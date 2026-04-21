package utils

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

// NumGo is a constant for job processing.
const NumGo = "Num Goroutines"

var pathSep = string(filepath.Separator)

var (
	// v0 and v1 are reusable values to avoid allocations in database calls.
	v0 uint8
	v1 uint8 = 1

	// jobLocks tracks running jobs per configuration name to prevent concurrent execution.
	jobLocks      = make(map[string]*sync.Mutex)
	jobLocksGuard sync.Mutex
)

// getJobLock retrieves or creates a mutex for the given configuration name.
// This ensures only one job per configuration can run at a time.
func getJobLock(cfgName string) *sync.Mutex {
	jobLocksGuard.Lock()
	defer jobLocksGuard.Unlock()

	if lock, exists := jobLocks[cfgName]; exists {
		return lock
	}

	lock := &sync.Mutex{}

	jobLocks[cfgName] = lock

	return lock
}

// tryAcquireJobLock attempts to acquire a lock for the given configuration.
// Returns true if the lock was acquired, false if another job is already running.
func tryAcquireJobLock(cfgName string) bool {
	lock := getJobLock(cfgName)
	return lock.TryLock()
}

// releaseJobLock releases the lock for the given configuration.
func releaseJobLock(cfgName string) {
	lock := getJobLock(cfgName)
	lock.Unlock()
}

// insertjobhistory inserts a new record into the job_histories table to track when a job starts.
// It takes the job type, media config, and current time as parameters.
// It returns the auto-generated id for the inserted row.
func insertjobhistory(jobtype string, cfgp *config.MediaTypeConfig) int64 {
	jobcategory := mediatype.GetCategoryName(cfgp.IsType)

	result, err := database.ExecNid(
		"Insert into job_histories (job_type, job_group, job_category, started) values (?, ?, ?, datetime('now','localtime'))",
		jobtype,
		&cfgp.Name,
		&jobcategory,
	)
	if err == nil {
		return result
	}

	return 0
}

// InitialFillSeries performs the initial database fill for TV series.
func InitialFillSeries() {
	InitialFill(config.MediaTypeSeries)
}

// InitialFillMovies performs the initial database fill for movies.
func InitialFillMovies() {
	InitialFill(config.MediaTypeMovie)
}

// InitialFill performs the initial database fill for a media type.
// It refreshes the unmatched and files caches, inserts job history records,
// imports new items from the configured lists, scans for new files, and clears caches.
func InitialFill(mediaType uint) {
	handler := mediatype.Get(mediaType)
	if handler == nil {
		return
	}

	logger.Logtype("info", 0).
		Str("type", handler.GetCategoryName()).
		Msg("Starting initial DB fill")

	database.Refreshunmatchedcached(mediaType, true)
	database.Refreshfilescached(mediaType, true)

	ctx := context.Background()

	// Import new items from lists
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if media.IsType != mediaType {
			return nil
		}

		dbid := insertjobhistory(logger.StrFeeds, media)
		for idx2 := range media.Lists {
			if idx2 > 127 {
				continue
			}

			err := importnewsingle(ctx, media, &media.Lists[idx2], idx2)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Str("type", handler.GetCategoryName()).
					Msg("Import new failed")
			}
		}

		database.ExecN(database.QueryUpdateHistory, &dbid)

		return nil
	})

	// Scan for new files
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if media.IsType != mediaType {
			return nil
		}

		dbid := insertjobhistory(logger.StrDataFull, media)
		for idxi := range media.Data {
			newfilesloop(ctx, media, &media.Data[idxi])
		}

		database.ExecN(database.QueryUpdateHistory, &dbid)

		return nil
	})

	database.ClearCaches()
}

// FillImdb refreshes the IMDB database by calling the init_imdb executable.
// It inserts a record into the job history table, executes the IMDB update,
// reloads the IMDB database, and updates the job history record when done.
func FillImdb() {
	dbinsert, _ := database.ExecNid(
		"Insert into job_histories (job_type, job_group, job_category, started) values (?, 'RefreshImdb', ?, datetime('now','localtime'))",
		&logger.StrData,
		&logger.StrMovie,
	)

	data, err := parser.ExecCmdString[[]byte]("", logger.StrImdb)
	if err == nil {
		logger.Logtype("info", 1).
			Str("response", data).
			Msg("imdb response")
		database.ExchangeImdbDB()
	}

	if dbinsert != 0 {
		database.ExecN(database.QueryUpdateHistory, dbinsert)
	}
}

// newfilesloop processes a directory of files, checking for new or unmatched files, and importing them into the media database.
// It uses a worker pool to parallelize the file processing, and handles various checks and logic for determining the appropriate
// media list ID and importing the file data.
func newfilesloop(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	if data.CfgPath == nil {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, data.TemplatePath).
			Msg("config not found")
		return errors.New("config not found")
	}

	if cfgp == nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.TemplatePath).
			Err(logger.ErrCfgpNotFound).
			Msg("parse failed cfgp")

		return errors.New("parse failed cfgp")
	}

	// For multi-track media (audiobooks, music), group files by folder first
	if mediatype.UsesGroupedFileProcessing(cfgp.IsType) {
		return newfilesloopGrouped(ctx, cfgp, data)
	}

	// For books, use dedicated book processing (no video/audio parsing)
	if cfgp.IsType == config.MediaTypeBook {
		return newfilesloopBook(ctx, cfgp, data)
	}

	pl := worker.WorkerPoolParse.NewGroupContext(ctx)
	glblistid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	err := filepath.WalkDir(
		data.CfgPath.Path,
		func(fpath string, info fs.DirEntry, errw error) error {
			if errw != nil {
				return errw
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			if config.GetSettingsGeneral().UseFileCache {
				if database.SlicesCacheContains(cfgp.IsType, logger.CacheFiles, fpath) {
					return nil
				}

				if database.SlicesCacheContains(cfgp.IsType, logger.CacheUnmatched, fpath) {
					return nil
				}
			} else {
				if database.Getdatarow[uint](
					false,
					mtstrings.GetStringsMap(cfgp.IsType, logger.DBCountFilesLocation),
					fpath,
				) >= 1 {
					return nil
				}

				if database.Getdatarow[uint](
					false,
					mtstrings.GetStringsMap(cfgp.IsType, logger.DBCountUnmatchedPath),
					fpath,
				) >= 1 {
					return nil
				}
			}

			ok, _ := scanner.CheckExtensions(true, false, data.CfgPath, filepath.Ext(info.Name()))

			// Check IgnoredPaths

			if ok && data.CfgPath.BlockedLen >= 1 {
				if logger.SlicesContainsPart2I(data.CfgPath.Blocked, fpath) {
					return nil
				}
			}

			if !ok {
				return nil
			}

			pl.Submit(func() {
				defer logger.HandlePanic()

				m := parser_v2.ParseFile(fpath, true, true, cfgp, -1)
				if m == nil {
					return // errors.New("parse failed")
				}

				defer m.Close()

				err := parser.GetDBIDs(m, cfgp, true, false)
				if err != nil {
					logger.Logtype("warn", 1).
						Str(logger.StrFile, fpath).
						Err(err).
						Msg(logger.ParseFailedIDs)

					return // err
				}

				listid := glblistid
				if m.ListID != -1 && glblistid == -1 {
					listid = m.ListID
				}

				if h := mediatype.Get(cfgp.IsType); h != nil {
					if h.GetMediaID(m) != 0 && m.ListID == -1 && listid == -1 {
						listid = h.GetListID(cfgp, m)
						m.ListID = listid
					}
				}

				if listid == -1 {
					return // errors.New("listid not found")
				}

				err = jobImportParseCommon(m, fpath, cfgp, &cfgp.Lists[listid], data.AddFound)
				if err != nil {
					logger.Logtype("error", 1).
						Str(logger.StrFile, fpath).
						Err(err).
						Msg("Error Importing")

					return // err
				}
			})

			return nil
		},
	)

	errjobs := pl.Wait()
	if errjobs != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.CfgPath.Path).
			Err(errjobs).
			Msg("Error walking jobs")
	}

	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.CfgPath.Path).
			Err(err).
			Msg("Error walking directory")
	}

	return err
}

// newfilesloopBook handles book files (ebooks).
// Books are single-file media that don't require video or audio parsing.
// This function walks the directory and processes each book file individually.
func newfilesloopBook(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) error {
	pl := worker.WorkerPoolParse.NewGroupContext(ctx)
	glblistid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	err := filepath.WalkDir(
		data.CfgPath.Path,
		func(fpath string, info fs.DirEntry, errw error) error {
			if errw != nil {
				return errw
			}

			if err := ctx.Err(); err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			// Check file cache
			if config.GetSettingsGeneral().UseFileCache {
				if database.SlicesCacheContains(cfgp.IsType, logger.CacheFiles, fpath) {
					return nil
				}

				if database.SlicesCacheContains(cfgp.IsType, logger.CacheUnmatched, fpath) {
					return nil
				}
			} else {
				if database.Getdatarow[uint](
					false,
					mtstrings.GetStringsMap(cfgp.IsType, logger.DBCountFilesLocation),
					fpath,
				) >= 1 {
					return nil
				}

				if database.Getdatarow[uint](
					false,
					mtstrings.GetStringsMap(cfgp.IsType, logger.DBCountUnmatchedPath),
					fpath,
				) >= 1 {
					return nil
				}
			}

			// Check if file has a book extension
			ok, _ := scanner.CheckExtensionsType(
				cfgp.IsType,
				false,
				data.CfgPath,
				filepath.Ext(info.Name()),
			)

			// Check IgnoredPaths
			if ok && data.CfgPath.BlockedLen >= 1 {
				if logger.SlicesContainsPart2I(data.CfgPath.Blocked, fpath) {
					return nil
				}
			}

			if !ok {
				return nil
			}

			pl.Submit(func() {
				defer logger.HandlePanic()

				m := parser_v2.ParseFile(fpath, true, true, cfgp, -1)
				if m == nil {
					return
				}

				defer m.Close()

				err := parser.GetDBIDs(m, cfgp, true, false)
				if err != nil {
					logger.Logtype("warn", 1).
						Str(logger.StrFile, fpath).
						Err(err).
						Msg(logger.ParseFailedIDs)

					return
				}

				listid := glblistid
				if m.ListID != -1 && glblistid == -1 {
					listid = m.ListID
				}

				if h := mediatype.Get(cfgp.IsType); h != nil {
					if h.GetMediaID(m) != 0 && m.ListID == -1 && listid == -1 {
						listid = h.GetListID(cfgp, m)
						m.ListID = listid
					}
				}

				if listid == -1 {
					return
				}

				err = jobImportParseBook(m, fpath, cfgp, &cfgp.Lists[listid], data.AddFound)
				if err != nil {
					logger.Logtype("error", 1).
						Str(logger.StrFile, fpath).
						Err(err).
						Msg("Error Importing Book")

					return
				}
			})

			return nil
		},
	)

	errjobs := pl.Wait()
	if errjobs != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.CfgPath.Path).
			Err(errjobs).
			Msg("Error walking book jobs")
	}

	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, data.CfgPath.Path).
			Err(err).
			Msg("Error walking book directory")
	}

	return err
}

// newfilesloopGrouped handles multi-track media types (audiobooks, music) by:
// 1. Getting all root folders and recursively processing them
// 2. Per folder: determining if it contains files directly or subfolders
// 3. Grouping and validating albums using structure_v2 matching logic
// 4. Importing matched albums to the database if addFound is enabled
// 5. Adding files to the database for matched albums.
func newfilesloopGrouped(
	ctx context.Context,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) error {
	rootPath := data.CfgPath.Path

	// Create a map to track processed directories within this scan
	processedDirs := &sync.Map{}

	// Process the root path recursively
	return processAudioDirectory(ctx, &rootPath, cfgp, data, processedDirs)
}

// processAudioDirectory recursively processes a directory for audio files.
// It determines whether to process the folder as an album or recurse into subfolders.
// The processedDirs map tracks directories already processed in this scan to avoid duplicates.
func processAudioDirectory(
	ctx context.Context,
	dirPath *string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
	processedDirs *sync.Map,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	// Check if this directory was already processed in this scan
	if _, alreadyProcessed := processedDirs.LoadOrStore(*dirPath, true); alreadyProcessed {
		// logger.Logtype("debug", 0).
		// 	Str("folder", dirPath).
		// 	Msg("DEBUG: Directory already processed in this scan - skipping")
		return nil
	}

	// DEBUG: Log entry into directory
	// logger.Logtype("debug", 0).
	// 	Str("folder", dirPath).
	// 	Str("mediaType", cfgp.Name).
	// 	Msg("DEBUG: Entering processAudioDirectory")

	// Check blocked paths
	if data.CfgPath.BlockedLen >= 1 {
		if logger.SlicesContainsPart2I(data.CfgPath.Blocked, *dirPath) {
			logger.Logtype("debug", 0).
				Str("folder", *dirPath).
				Msg("Folder is blocked - skipping")
			return nil
		}
	}

	// Step 2a: Check if this folder is already processed (rootpath exists with files)
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		if database.AudiobookExistsByRootpath(dirPath) {
			// logger.Logtype("debug", 0).
			// 	Str("folder", dirPath).
			// 	Msg("DEBUG: Audiobook folder already in database - skipping")
			return nil
		}

	case config.MediaTypeMusic:
		if database.AlbumExistsByRootpath(dirPath) {
			// logger.Logtype("debug", 0).
			// 	Str("folder", dirPath).
			// 	Msg("DEBUG: Album folder already in database - skipping")
			return nil
		}
	}

	// Read directory contents
	entries, err := os.ReadDir(*dirPath)
	if err != nil {
		logger.Logtype("error", 0).
			Str("folder", *dirPath).
			Err(err).
			Msg("DEBUG: Failed to read directory")

		return err
	}

	// Pre-allocate slices based on entry count for better memory efficiency
	entryCount := len(entries)
	audioFiles := make([]string, 0, entryCount/2+1)
	subDirs := make([]string, 0, entryCount/4+1)
	hasMultiDiscSubfolder := false

	for i := range entries {
		entryPath := logger.JoinStringsSep([]string{*dirPath, entries[i].Name()}, pathSep)

		if entries[i].IsDir() {
			// Check if it's a multi-disc subfolder (CD1, DISC1, Part1, Chapter1, etc.)
			if isMultiDiscSubfolder(entries[i].Name()) {
				hasMultiDiscSubfolder = true

				logger.Logtype("debug", 0).
					Str("folder", *dirPath).
					Str("subfolder", entries[i].Name()).
					Msg("DEBUG: Found multi-disc subfolder")
			}

			subDirs = append(subDirs, entryPath)
		} else {
			// Check if it's an audio file with valid extension
			ext := filepath.Ext(entries[i].Name())
			if ok, _ := scanner.CheckExtensionsType(cfgp.IsType, false, data.CfgPath, ext); ok {
				audioFiles = append(audioFiles, entryPath)
			}
		}
	}

	// logger.Logtype("debug", 0).
	// 	Str("folder", dirPath).
	// 	Int("audioFiles", len(audioFiles)).
	// 	Int("subDirs", len(subDirs)).
	// 	Bool("hasMultiDisc", hasMultiDiscSubfolder).
	// 	Msg("DEBUG: Directory contents analyzed")

	// Step 2b/2c: If we have audio files in root OR multi-disc subfolders, try to match as album
	if len(audioFiles) > 0 || hasMultiDiscSubfolder {
		// logger.Logtype("debug", 0).
		// 	Str("folder", dirPath).
		// 	Msg("DEBUG: Attempting to process as album")

		// Step 4: Try to match this folder as an album
		matched := ProcessAudioFolderAsAlbum(ctx, *dirPath, cfgp, data)
		if matched {
			// logger.Logtype("debug", 0).
			// 	Str("folder", dirPath).
			// 	Msg("DEBUG: Successfully matched and processed as album")
			return nil // Successfully processed
		}

		// logger.Logtype("debug", 0).
		// 	Str("folder", dirPath).
		// 	Msg("DEBUG: No album match found, will process subfolders")
		// If no match found and we have subfolders, fall through to process them
	}

	// Step 2d: Process subfolders recursively
	if len(subDirs) > 0 {
		// logger.Logtype("debug", 0).
		// 	Str("folder", dirPath).
		// 	Int("subfolders", len(subDirs)).
		// 	Msg("DEBUG: Processing subfolders")

		// For large directories (top-level with many authors), process sequentially
		// to avoid overwhelming the system with thousands of parallel tasks
		if len(subDirs) > 50 {
			// logger.Logtype("info", 0).
			// 	Str("folder", dirPath).
			// 	Int("subfolders", len(subDirs)).
			// 	Msg("Processing large directory sequentially")
			for i, subDir := range subDirs {
				if err := ctx.Err(); err != nil {
					return err
				}

				// Progress logging every 100 folders
				if (i+1)%100 == 0 || i == 0 {
					logger.Logtype("info", 0).
						Str("folder", *dirPath).
						Int("progress", i+1).
						Int("total", len(subDirs)).
						Int("percent", ((i+1)*100)/len(subDirs)).
						Msg("Directory scan progress")
				}

				_ = processAudioDirectory(ctx, &subDir, cfgp, data, processedDirs)
			}

			// logger.Logtype("info", 0).
			// 	Str("folder", dirPath).
			// 	Int("total", len(subDirs)).
			// 	Msg("Completed processing large directory")
		} else {
			// For smaller directories, process sequentially too to avoid pool issues
			for i, subDir := range subDirs {
				if err := ctx.Err(); err != nil {
					return err
				}

				// Progress logging every 10 folders for smaller dirs
				if (i+1)%10 == 0 {
					logger.Logtype("debug", 0).
						Str("folder", *dirPath).
						Int("progress", i+1).
						Int("total", len(subDirs)).
						Msg("DEBUG: Subfolder progress")
				}

				_ = processAudioDirectory(ctx, &subDir, cfgp, data, processedDirs)
			}
		}
	}

	return nil
}

// multiDiscPatterns holds common multi-disc folder name patterns.
// Defined at package level to avoid allocation on each call.
var multiDiscPatterns = []string{
	"cd", "disc", "disk", "part", "chapter", "volume", "vol",
	"book", "side", "tape",
}

// isMultiDiscSubfolder checks if a folder name indicates a multi-disc/part structure.
func isMultiDiscSubfolder(name string) bool {
	for _, pattern := range multiDiscPatterns {
		// Check for patterns like "CD1", "CD 1", "CD-1", "Disc 01", etc.
		if !logger.HasPrefixI(name, pattern) {
			continue
		}

		rest := name[len(pattern):]

		rest = strings.TrimLeft(rest, " -_")
		if len(rest) > 0 && rest[0] >= '0' && rest[0] <= '9' {
			return true
		}
	}

	return false
}

// ProcessAudioFolderAsAlbum attempts to process a folder as a complete album/audiobook.
// Routes to appropriate handler based on media type:
// - Audiobooks: Uses ASIN, Audible/Audnex for matching, validates language
// - Music: Uses MusicBrainzID/UPC, MusicBrainz/Discogs/AcoustID for matching
// Returns true if the folder was successfully matched and processed.
// It collects files, matches to API, verifies track count, and organizes the album.
// This is exported so structure.go can call it for organization jobs.
func ProcessAudioFolderAsAlbum(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) bool {
	// logger.Logtype("debug", 0).
	// 	Str("folder", folder).
	// 	Uint("mediaType", cfgp.IsType).
	// 	Msg("DEBUG: processAudioFolderAsAlbum called")

	// Step 1: Collect files only (no tag reading yet)
	files, err := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
	if err != nil {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Err(err).
			Msg("DEBUG: CollectFilesOnly returned error")

		return false
	}

	if len(files) == 0 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Msg("DEBUG: No audio files found")
		return false
	}

	// logger.Logtype("debug", 0).
	// 	Str("folder", folder).
	// 	Int("fileCount", len(files)).
	// 	Msg("DEBUG: Collected audio files")

	// Route to appropriate handler based on media type
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		return processAudiobookFolder(ctx, folder, files, cfgp, data)
	case config.MediaTypeMusic:
		return processMusicFolder(ctx, folder, files, cfgp, data)
	default:
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Uint("mediaType", cfgp.IsType).
			Msg("DEBUG: Unsupported media type for audio folder processing")

		return false
	}
}

// processAudiobookFolder handles audiobook-specific folder processing.
// Uses ASIN for identification and Audible/Audnex for matching.
func processAudiobookFolder(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) bool {
	// Step 2: Parse folder path and first filename for author/album
	artist, albumTitle, year := parser_v2.ParseAudioFolder(folder)
	asin := parser_v2.ParseASINFromPath(folder)

	// Also try parsing first filename for additional info
	firstTrack := parser_v2.ParseAudioFilename(files[0])
	if artist == "" && firstTrack.Artist != "" {
		artist = firstTrack.Artist
	}

	if albumTitle == "" && firstTrack.Album != "" {
		albumTitle = firstTrack.Album
	}

	if asin == "" && firstTrack.ASIN != "" {
		asin = firstTrack.ASIN
	}

	logger.Logtype("debug", 0).
		Str("folder", folder).
		Str("artist", artist).
		Str("album", albumTitle).
		Str("asin", asin).
		Int("year", year).
		Msg("DEBUG: Parsed audiobook metadata from folder/filename")

	// Step 3: Try to find album match in database
	var (
		dbMatch *database.AudiobookSearchResult
		// searchErr error
	)

	// Try by ASIN first (most reliable)

	if asin != "" {
		dbMatch, _ = database.FindAudiobookByASIN(asin)
		// if searchErr == nil && dbMatch != nil {
		// 	logger.Logtype("debug", 0).
		// 		Str("folder", folder).
		// 		Str("asin", asin).
		// 		Uint("dbID", dbMatch.ID).
		// 		Msg("DEBUG: Found audiobook by ASIN")
		// }
	}

	// Try by title/author if ASIN didn't match
	if dbMatch == nil && albumTitle != "" {
		dbMatch, _ = database.FindAudiobookByTitleAuthor(&albumTitle, &artist)
		// if searchErr == nil && dbMatch != nil {
		// 	logger.Logtype("debug", 0).
		// 		Str("folder", folder).
		// 		Str("title", albumTitle).
		// 		Str("artist", artist).
		// 		Uint("dbID", dbMatch.ID).
		// 		Msg("DEBUG: Found audiobook by title/author")
		// }
	}

	// Step 4: If no match and we haven't read tags yet, read tags from first file
	// and retry if the tag data is different
	if dbMatch == nil {
		tagData := parser_v2.ReadTagsForFirstFile(files)
		if tagData != nil {
			tagArtist := coalesceStr(tagData.AlbumArtist, tagData.Artist)
			tagAlbum := tagData.Album
			tagASIN := tagData.ASIN

			// logger.Logtype("debug", 0).
			// 	Str("folder", folder).
			// 	Str("tagArtist", tagArtist).
			// 	Str("tagAlbum", tagAlbum).
			// 	Str("tagASIN", tagASIN).
			// 	Msg("DEBUG: Read tags from first file")

			// Retry search if tag data is different
			if tagASIN != "" && tagASIN != asin {
				dbMatch, _ = database.FindAudiobookByASIN(tagASIN)
				if dbMatch != nil {
					asin = tagASIN
					// logger.Logtype("debug", 0).
					// 	Str("folder", folder).
					// 	Str("asin", tagASIN).
					// 	Msg("DEBUG: Found audiobook by tag ASIN")
				}
			}

			if dbMatch == nil && tagAlbum != "" && (tagAlbum != albumTitle || tagArtist != artist) {
				dbMatch, _ = database.FindAudiobookByTitleAuthor(&tagAlbum, &tagArtist)
				if dbMatch != nil {
					albumTitle = tagAlbum
					artist = tagArtist
					// logger.Logtype("debug", 0).
					// 	Str("folder", folder).
					// 	Str("title", tagAlbum).
					// 	Str("artist", tagArtist).
					// 	Msg("DEBUG: Found audiobook by tag title/author")
				}
			}
		}
	}

	// Build album info structure
	album := &parser_v2.AlbumInfo{
		Title:        albumTitle,
		Artist:       artist,
		Year:         year,
		ASIN:         asin,
		SourceFolder: folder,
		TrackCount:   len(files),
	}

	if dbMatch != nil {
		album.DatabaseID = dbMatch.ID
		album.ExpectedTracks = dbMatch.ChapterCount
	}

	// Step 5: If still no match and addFound is enabled, try to import
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)
	if album.DatabaseID == 0 && data.AddFound && album.ASIN != "" && listid != -1 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("asin", album.ASIN).
			Str("title", album.Title).
			Int("tracks", album.TrackCount).
			Msg("Audiobook not in database - importing via addFound")

		dbID, importErr := importfeed.JobImportAudiobooks(ctx, album.ASIN, cfgp, listid, true)
		if importErr == nil && dbID != 0 {
			album.DatabaseID = dbID
			// Re-fetch to get chapter count
			if imported, _ := database.FindAudiobookByASIN(album.ASIN); imported != nil {
				album.ExpectedTracks = imported.ChapterCount
			}

			logger.Logtype("debug", 0).
				Str("folder", folder).
				Uint("databaseID", dbID).
				Msg("Successfully imported audiobook")
		}
	}

	// If still no match, can't proceed
	if album.DatabaseID == 0 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("title", album.Title).
			Str("artist", album.Artist).
			Str("asin", album.ASIN).
			Msg("Audiobook not found in database - skipping")

		return false
	}

	// Step 6: Verify track count matches expected chapter count
	if album.ExpectedTracks > 0 && album.TrackCount != album.ExpectedTracks {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Int("localTracks", album.TrackCount).
			Int("expectedChapters", album.ExpectedTracks).
			Msg("Track count mismatch - skipping")

		return false
	}

	// Step 7: Now read tags for all files and build track list
	tracks := parser_v2.CollectTracksFromFiles(files)

	tracks = parser_v2.EnrichTracksWithTags(tracks)
	album.Tracks = tracks
	album.TotalRuntime = parser_v2.CalculateTotalRuntime(tracks)

	// Step 8: Sort tracks and verify (try multiple sorting strategies)
	if !verifyAndSortTracks(album) {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Msg("Track verification failed after all sorting attempts")
		return false
	}

	// logger.Logtype("debug", 0).
	// 	Str("folder", folder).
	// 	Uint("databaseID", album.DatabaseID).
	// 	Int("trackCount", album.TrackCount).
	// 	Msg("DEBUG: About to add audiobook files to database")

	// Step 9: Add files to database
	return addAudiobookFilesToDatabase(ctx, folder, album, cfgp, listid)
}

// searchMusicBrainzByMetadata searches MusicBrainz for an album by artist, album title, and track count.
// Returns the MusicBrainz ID if a good match is found (exact track count + runtime match), or empty string if no match.
func searchMusicBrainzByMetadata(
	ctx context.Context,
	artist, album string,
	trackCount int,
	totalRuntimeMS int64,
) string {
	// Get the MusicBrainz provider
	mbProvider := providers.GetMusicBrainz()
	if mbProvider == nil {
		logger.Logtype("error", 1).Msg("MusicBrainz provider not available")
		return ""
	}

	query := string(importfeed.BuildArtistAlbumSearch(album, artist))

	// Search for releases with panic recovery
	var (
		results []apiexternal_v2.ReleaseSearchResult
		err     error
	)

	func() {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("MusicBrainz search panicked: %v", r)
			}
		}()

		results, _, err = mbProvider.SearchReleases(ctx, query, 10, 0)
	}()

	if err != nil {
		logger.Logtype("error", 1).
			Err(err).
			Str("artist", artist).
			Str("album", album).
			Str("query", query).
			Msg("MusicBrainz search failed")

		return ""
	}

	if len(results) == 0 {
		logger.Logtype("debug", 1).
			Str("artist", artist).
			Str("album", album).
			Msg("No MusicBrainz results found")

		return ""
	}

	// Find the best match - MUST have exact track count match
	isVASearch := strings.EqualFold(artist, "various artists")
	for i := range results {
		result := &results[i]

		// Filter by artist client-side (server-side artist: field is unreliable for variant names).
		if !isVASearch && artist != "" {
			artistMatch := false
			for _, a := range result.Artists {
				if strings.EqualFold(a.Name, artist) {
					artistMatch = true
					break
				}
			}

			if !artistMatch {
				continue
			}
		}

		// REQUIRED: Exact track count match
		if result.TrackCount != trackCount {
			logger.Logtype("debug", 2).
				Str("mbid", result.ID).
				Str("title", result.Title).
				Int("expected_tracks", trackCount).
				Int("result_tracks", result.TrackCount).
				Msg("Skipping - track count mismatch")

			continue
		}

		// Fetch full release details to get track durations for runtime validation
		releaseDetails, err := mbProvider.GetReleaseByID(ctx, result.ID)
		if err != nil {
			logger.Logtype("debug", 2).
				Str("mbid", result.ID).
				Err(err).
				Msg("Failed to fetch release details - skipping")

			continue
		}

		// Calculate total runtime from tracks
		var mbTotalRuntimeMS int64
		for i := range releaseDetails.Tracks {
			mbTotalRuntimeMS += int64(releaseDetails.Tracks[i].DurationMs)
		}

		// Check runtime if available
		if totalRuntimeMS > 0 && mbTotalRuntimeMS > 0 {
			// Allow 3% runtime tolerance
			diff := totalRuntimeMS - mbTotalRuntimeMS
			if diff < 0 {
				diff = -diff
			}

			tolerance := max(
				// 3% tolerance
				totalRuntimeMS*3/100,
				// At least 5 seconds
				5000)

			if diff > tolerance {
				logger.Logtype("debug", 2).
					Str("mbid", result.ID).
					Str("title", result.Title).
					Int64("expected_runtime_ms", totalRuntimeMS).
					Int64("result_runtime_ms", mbTotalRuntimeMS).
					Int64("diff_ms", diff).
					Msg("Skipping - runtime mismatch")

				continue
			}
		}

		// Found a good match!
		artistStr := ""
		if len(result.Artists) > 0 {
			artistNames := make([]string, len(result.Artists))
			for i, a := range result.Artists {
				artistNames[i] = a.Name
			}
			artistStr = logger.JoinStringsSep(artistNames, ", ")
		}

		logger.Logtype("info", 1).
			Str("mbid", result.ID).
			Str("title", result.Title).
			Str("artists", artistStr).
			Int("tracks", result.TrackCount).
			Int64("runtime_ms", mbTotalRuntimeMS).
			Msg("Found MusicBrainz match")

		return result.ID
	}

	logger.Logtype("debug", 1).
		Str("artist", artist).
		Str("album", album).
		Int("trackCount", trackCount).
		Msg("No exact MusicBrainz match found (track count or runtime mismatch)")

	return ""
}

// processMusicFolder handles music album-specific folder processing.
// Uses MusicBrainzID/UPC for identification and MusicBrainz/Discogs for matching.
// Does NOT perform language validation (unlike audiobooks).
func processMusicFolder(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataConfig,
) bool {
	// Step 2: Parse folder path and first filename for artist/album
	artist, albumTitle, year := parser_v2.ParseAudioFolder(folder)

	// Also try parsing first filename for additional info
	firstTrack := parser_v2.ParseAudioFilename(files[0])
	if artist == "" && firstTrack.Artist != "" {
		artist = firstTrack.Artist
	}

	if albumTitle == "" && firstTrack.Album != "" {
		albumTitle = firstTrack.Album
	}

	// Try to get MusicBrainzID or UPC from tags
	var musicBrainzID, upc string

	tagData := parser_v2.ReadTagsForFirstFile(files)
	if tagData != nil {
		if artist == "" {
			artist = coalesceStr(tagData.AlbumArtist, tagData.Artist)
		}

		if albumTitle == "" {
			albumTitle = tagData.Album
		}

		musicBrainzID = tagData.MusicBrainzID
		// UPC might be in a custom tag - we'll use whatever is available
	}

	logger.Logtype("debug", 0).
		Str("folder", folder).
		Str("artist", artist).
		Str("album", albumTitle).
		Str("musicBrainzID", musicBrainzID).
		Int("year", year).
		Msg("DEBUG: Parsed music metadata from folder/filename")

	// Step 3: Try to find album match in database
	var (
		dbMatch *database.AlbumSearchResult
		// searchErr error
	)

	// Try by MusicBrainzID first (most reliable)

	if musicBrainzID != "" {
		dbMatch, _ = database.FindAlbumByMusicBrainzID(&musicBrainzID)
		// if searchErr == nil && dbMatch != nil {
		// 	logger.Logtype("debug", 0).
		// 		Str("folder", folder).
		// 		Str("musicBrainzID", musicBrainzID).
		// 		Uint("dbID", dbMatch.ID).
		// 		Msg("DEBUG: Found album by MusicBrainzID")
		// }
	}

	// Try by UPC if MusicBrainzID didn't match
	if dbMatch == nil && upc != "" {
		dbMatch, _ = database.FindAlbumByUPC(&upc)
		// if searchErr == nil && dbMatch != nil {
		// 	logger.Logtype("debug", 0).
		// 		Str("folder", folder).
		// 		Str("upc", upc).
		// 		Uint("dbID", dbMatch.ID).
		// 		Msg("DEBUG: Found album by UPC")
		// }
	}

	// Try by artist/title if identifiers didn't match
	if dbMatch == nil && albumTitle != "" {
		dbMatch, _ = database.FindAlbumByArtistTitle(&artist, &albumTitle)
		// if searchErr == nil && dbMatch != nil {
		// 	logger.Logtype("debug", 0).
		// 		Str("folder", folder).
		// 		Str("artist", artist).
		// 		Str("title", albumTitle).
		// 		Uint("dbID", dbMatch.ID).
		// 		Msg("DEBUG: Found album by artist/title")
		// }
	}

	// Build album info structure
	album := &parser_v2.AlbumInfo{
		Title:        albumTitle,
		Artist:       artist,
		Year:         year,
		SourceFolder: folder,
		TrackCount:   len(files),
	}

	if dbMatch != nil {
		album.DatabaseID = dbMatch.ID
		album.ExpectedTracks = dbMatch.TotalTracks
	}

	// Step 4: If still no match and addFound is enabled, try to import via MusicBrainz
	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

	// Log why MusicBrainz search might be skipped
	// logger.Logtype("debug", 0).
	// 	Str("folder", folder).
	// 	Bool("musicBrainzID_empty", musicBrainzID == "").
	// 	Bool("album_not_in_db", album.DatabaseID == 0).
	// 	Bool("addFound", data.AddFound).
	// 	Bool("artist_present", artist != "").
	// 	Bool("albumTitle_present", albumTitle != "").
	// 	Int("listid", listid).
	// 	Msg("DEBUG: MusicBrainz search conditions check")

	// If no MusicBrainzID in tags, try searching MusicBrainz by artist/album/trackcount
	if musicBrainzID == "" && album.DatabaseID == 0 && data.AddFound && artist != "" &&
		albumTitle != "" &&
		listid != -1 {
		// logger.Logtype("info", 1).
		// 	Str("folder", folder).
		// 	Str("artist", artist).
		// 	Str("album", albumTitle).
		// 	Int("tracks", album.TrackCount).
		// 	Msg("No MusicBrainzID in tags - searching MusicBrainz")

		// Calculate total runtime from all files
		var totalRuntimeMS int64
		for _, file := range files {
			if audioTags := parser_v2.ReadTagsForFirstFile([]string{file}); audioTags != nil {
				totalRuntimeMS += audioTags.RuntimeMS
			}
		}

		// logger.Logtype("debug", 0).
		// 	Str("folder", folder).
		// 	Int64("totalRuntimeMS", totalRuntimeMS).
		// 	Int("trackCount", album.TrackCount).
		// 	Msg("DEBUG: Calling searchMusicBrainzByMetadata")

		searchedMBID := searchMusicBrainzByMetadata(
			ctx,
			artist,
			albumTitle,
			album.TrackCount,
			totalRuntimeMS,
		)
		if searchedMBID != "" {
			musicBrainzID = searchedMBID
			// logger.Logtype("info", 1).
			// 	Str("folder", folder).
			// 	Str("musicBrainzID", musicBrainzID).
			// 	Msg("Found MusicBrainzID via search")
		} else {
			logger.Logtype("info", 1).
				Str("folder", folder).
				Str("artist", artist).
				Str("album", albumTitle).
				Msg("MusicBrainz search found no matches")
		}
	} else {
		// Log which condition(s) failed
		var reasons []string
		if musicBrainzID != "" {
			reasons = append(reasons, "already has MusicBrainzID")
		}

		if album.DatabaseID != 0 {
			reasons = append(reasons, "album already in database")
		}

		if !data.AddFound {
			reasons = append(reasons, "AddFound is disabled")
		}

		if artist == "" {
			reasons = append(reasons, "artist is empty")
		}

		if albumTitle == "" {
			reasons = append(reasons, "albumTitle is empty")
		}

		if listid == -1 {
			reasons = append(reasons, "listid is -1")
		}

		// logger.Logtype("debug", 0).
		// 	Str("folder", folder).
		// 	Strs("skip_reasons", reasons).
		// 	Msg("DEBUG: Skipping MusicBrainz search")
	}

	if album.DatabaseID == 0 && data.AddFound && musicBrainzID != "" && listid != -1 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("musicBrainzID", musicBrainzID).
			Str("title", album.Title).
			Int("tracks", album.TrackCount).
			Msg("Album not in database - importing via addFound")

		dbID, importErr := importfeed.JobImportAlbums(ctx, musicBrainzID, cfgp, listid, true)
		if importErr == nil && dbID != 0 {
			album.DatabaseID = dbID
			// Re-fetch to get track count
			if imported, _ := database.FindAlbumByMusicBrainzID(&musicBrainzID); imported != nil {
				album.ExpectedTracks = imported.TotalTracks
			}

			// Get release group ID from dbalbums table
			var releaseGroupID string
			database.Scanrowsdyn(
				false,
				"SELECT musicbrainz_release_group_id FROM dbalbums WHERE musicbrainz_release_id = ?",
				&releaseGroupID,
				&musicBrainzID,
			)

			logger.Logtype("debug", 0).
				Str("folder", folder).
				Uint("databaseID", dbID).
				Msg("DEBUG: Successfully imported album")

			// Discover and add related albums if AddFound is enabled
			if data.AddFound {
				trackInfos := make([]parser_v2.TrackInfo, 0, len(files))
				for _, f := range files {
					if tags := parser_v2.ReadTagsForFirstFile([]string{f}); tags != nil {
						trackInfos = append(trackInfos, parser_v2.TrackInfo{Artist: tags.Artist})
					}
				}
				isVariousArtists := importfeed.DetectVA(artist, trackInfos)

				if isVariousArtists && releaseGroupID != "" {
					// For Various Artists, try to add albums from the same series
					logger.Logtype("info", 1).
						Str("album", albumTitle).
						Str("release_group_id", releaseGroupID).
						Msg("Various Artists album detected - checking for series")

					albumsAdded := importfeed.DiscoverAndAddSeriesAlbums(
						ctx,
						releaseGroupID,
						albumTitle,
						cfgp,
						listid,
						data.AllowedReleaseTypes,
						data.MBMediaFormats,
					)
					if albumsAdded > 0 {
						logger.Logtype("info", 0).
							Str("album", albumTitle).
							Int("albums_added", albumsAdded).
							Msg("Added albums from series discovery")
					}
				} else if artist != "" {
					// For regular artists, add their other albums
					logger.Logtype("info", 1).
						Str("artist", artist).
						Msg("Discovering other albums by artist")

					albumsAdded := importfeed.DiscoverAndAddArtistAlbums(
						ctx,
						&artist,
						cfgp,
						listid,
						50,
						data.AllowedReleaseTypes,
						data.MBMediaFormats,
					)
					if albumsAdded > 0 {
						logger.Logtype("info", 0).
							Str("artist", artist).
							Int("albums_added", albumsAdded).
							Msg("Added albums from artist discovery")
					}
				}
			}
		}
	}

	// If still no match, can't proceed
	if album.DatabaseID == 0 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("title", album.Title).
			Str("artist", album.Artist).
			Str("musicBrainzID", musicBrainzID).
			Msg("DEBUG: Album not found in database - skipping")

		return false
	}

	// Step 5: Verify track count matches expected (optional for music - allow some variance)
	if album.ExpectedTracks > 0 {
		trackDiff := album.TrackCount - album.ExpectedTracks
		if trackDiff < 0 {
			trackDiff = -trackDiff
		}

		// Allow up to 2 track variance for music (bonus tracks, hidden tracks, etc.)
		if trackDiff > 2 {
			logger.Logtype("debug", 0).
				Str("folder", folder).
				Int("localTracks", album.TrackCount).
				Int("expectedTracks", album.ExpectedTracks).
				Msg("DEBUG: Track count mismatch too large - skipping")

			return false
		}
	}

	// Step 6: Now read tags for all files and build track list
	tracks := parser_v2.CollectTracksFromFiles(files)

	tracks = parser_v2.EnrichTracksWithTags(tracks)
	album.Tracks = tracks
	album.TotalRuntime = parser_v2.CalculateTotalRuntime(tracks)

	// Step 7: Sort tracks and verify (try multiple sorting strategies)
	if !verifyAndSortTracks(album) {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Msg("DEBUG: Track verification failed after all sorting attempts")
		// Continue anyway - we matched by count, sorting issues shouldn't block
	}

	logger.Logtype("debug", 0).
		Str("folder", folder).
		Uint("databaseID", album.DatabaseID).
		Int("trackCount", album.TrackCount).
		Msg("DEBUG: About to add music album files to database")

	// Step 8: Add files to database (no language validation for music)
	return addAlbumFilesToDatabase(ctx, folder, album, cfgp, listid)
}

// verifyAndSortTracks tries different sorting strategies to get a valid track sequence.
// Returns true if tracks are properly sorted and verified.
func verifyAndSortTracks(album *parser_v2.AlbumInfo) bool {
	// Strategy 1: Sort by disc/track number (standard)
	parser_v2.SortTracksByDiscAndTrack(album.Tracks)

	isComplete, _ := parser_v2.ValidateAlbum(album, 0)
	if isComplete {
		return true
	}

	// Strategy 2: Sort by filename
	parser_v2.SortTracksByFilename(album.Tracks)
	// Re-assign track numbers based on position
	for i := range album.Tracks {
		album.Tracks[i].TrackNumber = i + 1
	}

	isComplete, _ = parser_v2.ValidateAlbum(album, 0)
	if isComplete {
		return true
	}

	// Strategy 3: Sort by disc and track again (reset)
	parser_v2.SortTracksByDiscAndTrack(album.Tracks)

	return false
}

// coalesceStr returns the first non-empty string.
func coalesceStr(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}

	return ""
}

// matchAndProcessAlbum attempts to match an album to the database and add its files.
// Returns true if successfully matched and processed.
// func matchAndProcessAlbum(
// 	ctx context.Context,
// 	folder string,
// 	album *structure_v2.AlbumInfo,
// 	cfgp *config.MediaTypeConfig,
// 	data *config.MediaDataConfig,
// ) bool {
// 	listid := cfgp.GetMediaListsEntryListID(data.AddFoundList)

// 	switch cfgp.IsType {
// 	case config.MediaTypeAudiobook:
// 		return matchAndProcessAudiobook(ctx, folder, album, cfgp, data, listid)
// 	case config.MediaTypeMusic:
// 		return matchAndProcessMusicAlbum(ctx, folder, album, cfgp, data, listid)
// 	}

// 	return false
// }

// matchAndProcessAudiobook handles audiobook-specific matching and processing.
// func matchAndProcessAudiobook(
// 	ctx context.Context,
// 	folder string,
// 	album *structure_v2.AlbumInfo,
// 	cfgp *config.MediaTypeConfig,
// 	data *config.MediaDataConfig,
// 	listid int,
// ) bool {
// 	logger.Logtype("debug", 0).
// 		Str("folder", folder).
// 		Str("title", album.Title).
// 		Str("artist", album.Artist).
// 		Str("asin", album.ASIN).
// 		Int("listid", listid).
// 		Bool("addFound", data.AddFound).
// 		Str("addFoundList", data.AddFoundList).
// 		Msg("DEBUG: matchAndProcessAudiobook called")

// 	// Try to find ASIN from folder path if not in tags
// 	if album.ASIN == "" {
// 		album.ASIN = structure_v2.ParseASINFromPath(folder)
// 		logger.Logtype("debug", 0).
// 			Str("folder", folder).
// 			Str("parsedASIN", album.ASIN).
// 			Msg("DEBUG: Parsed ASIN from folder path")
// 	}

// 	// Step 4a: Try to find audiobook in database by title/author
// 	dbResult, err := database.FindAudiobookByTitleAuthor(album.Title, album.Artist)
// 	if err == nil && dbResult != nil {
// 		album.DatabaseID = dbResult.ID
// 		album.ExpectedTracks = dbResult.ChapterCount
// 		logger.Logtype("debug", 0).
// 			Str("folder", folder).
// 			Uint("databaseID", album.DatabaseID).
// 			Msg("DEBUG: Found audiobook by title/author")
// 	} else {
// 		logger.Logtype("debug", 0).
// 			Str("folder", folder).
// 			Str("title", album.Title).
// 			Str("artist", album.Artist).
// 			Err(err).
// 			Msg("DEBUG: FindAudiobookByTitleAuthor failed")
// 	}

// 	// Try by ASIN if title search failed
// 	if album.DatabaseID == 0 && album.ASIN != "" {
// 		dbResult, err = database.FindAudiobookByASIN(album.ASIN)
// 		if err == nil && dbResult != nil {
// 			album.DatabaseID = dbResult.ID
// 			album.ExpectedTracks = dbResult.ChapterCount
// 			logger.Logtype("debug", 0).
// 				Str("folder", folder).
// 				Uint("databaseID", album.DatabaseID).
// 				Msg("DEBUG: Found audiobook by ASIN")
// 		} else {
// 			logger.Logtype("debug", 0).
// 				Str("folder", folder).
// 				Str("asin", album.ASIN).
// 				Err(err).
// 				Msg("DEBUG: FindAudiobookByASIN failed")
// 		}
// 	}

// 	// Step 5a: If not found and addFound is enabled, try to import
// 	if album.DatabaseID == 0 && data.AddFound && album.ASIN != "" && listid != -1 {
// 		logger.Logtype("info", 1).
// 			Str("folder", folder).
// 			Str("asin", album.ASIN).
// 			Str("title", album.Title).
// 			Int("tracks", album.TrackCount).
// 			Msg("Audiobook not in database - importing via addFound")

// 		dbID, importErr := importfeed.JobImportAudiobooks(ctx, album.ASIN, cfgp, listid, true)
// 		if importErr == nil && dbID != 0 {
// 			album.DatabaseID = dbID
// 			logger.Logtype("debug", 0).
// 				Str("folder", folder).
// 				Uint("databaseID", dbID).
// 				Msg("DEBUG: Successfully imported audiobook")
// 		} else {
// 			logger.Logtype("debug", 0).
// 				Str("folder", folder).
// 				Str("asin", album.ASIN).
// 				Err(importErr).
// 				Msg("DEBUG: JobImportAudiobooks failed")
// 		}
// 	} else if album.DatabaseID == 0 {
// 		logger.Logtype("debug", 0).
// 			Str("folder", folder).
// 			Bool("addFound", data.AddFound).
// 			Str("asin", album.ASIN).
// 			Int("listid", listid).
// 			Msg("DEBUG: Skipping import - conditions not met")
// 	}

// 	// Step 5b: If still not found, skip
// 	if album.DatabaseID == 0 {
// 		logger.Logtype("debug", 0).
// 			Str("folder", folder).
// 			Str("title", album.Title).
// 			Str("artist", album.Artist).
// 			Str("asin", album.ASIN).
// 			Msg("DEBUG: Audiobook not found in database - skipping")
// 		return false
// 	}

// 	logger.Logtype("debug", 0).
// 		Str("folder", folder).
// 		Uint("databaseID", album.DatabaseID).
// 		Int("trackCount", album.TrackCount).
// 		Msg("DEBUG: About to add files to database")

// 	// Step 6: Add files to database
// 	return addAudiobookFilesToDatabase(ctx, folder, album, cfgp, listid)
// }

// matchAndProcessMusicAlbum handles music album-specific matching and processing.
// func matchAndProcessMusicAlbum(
// 	ctx context.Context,
// 	folder string,
// 	album *structure_v2.AlbumInfo,
// 	cfgp *config.MediaTypeConfig,
// 	data *config.MediaDataConfig,
// 	listid int,
// ) bool {
// 	// Step 4a: Try to find album in database by artist/title
// 	dbResult, err := database.FindAlbumByArtistTitle(album.Artist, album.Title)
// 	if err == nil && dbResult != nil {
// 		album.DatabaseID = dbResult.ID
// 		album.ExpectedTracks = dbResult.TotalTracks
// 	}

// 	// Get identifier for import (MusicBrainzID or UPC from first track)
// 	var identifier string
// 	if len(album.Tracks) > 0 {
// 		// Try to get MusicBrainzID or UPC from tags
// 		for _, track := range album.Tracks {
// 			if track.MusicBrainzID != "" {
// 				identifier = track.MusicBrainzID
// 				break
// 			}
// 		}
// 	}

// 	// Step 5a: If not found and addFound is enabled, try to import
// 	if album.DatabaseID == 0 && data.AddFound && identifier != "" && listid != -1 {
// 		logger.Logtype("info", 1).
// 			Str("folder", folder).
// 			Str("identifier", identifier).
// 			Str("title", album.Title).
// 			Int("tracks", album.TrackCount).
// 			Msg("Album not in database - importing via addFound")

// 		dbID, importErr := importfeed.JobImportAlbums(ctx, identifier, cfgp, listid, true)
// 		if importErr == nil && dbID != 0 {
// 			album.DatabaseID = dbID
// 		}
// 	}

// 	// Step 5b: If still not found, skip
// 	if album.DatabaseID == 0 {
// 		logger.Logtype("debug", 0).
// 			Str("folder", folder).
// 			Str("title", album.Title).
// 			Str("artist", album.Artist).
// 			Msg("Album not found in database - skipping")
// 		return false
// 	}

// 	// Step 6: Add files to database
// 	return addAlbumFilesToDatabase(ctx, folder, album, cfgp, listid)
// }

// addAudiobookFilesToDatabase adds audiobook files to the database.
func addAudiobookFilesToDatabase(
	ctx context.Context,
	folder string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	listid int,
) bool {
	logger.Logtype("debug", 0).
		Str("folder", folder).
		Uint("databaseID", album.DatabaseID).
		Int("listid", listid).
		Int("trackCount", len(album.Tracks)).
		Msg("DEBUG: addAudiobookFilesToDatabase called")

	if album.DatabaseID == 0 || listid == -1 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Uint("databaseID", album.DatabaseID).
			Int("listid", listid).
			Msg("DEBUG: addAudiobookFilesToDatabase returning false - invalid IDs")

		return false
	}

	// Resolve the correct audiobooks.id (list entry) from dbaudiobook_id + listname
	var listname string
	if listid >= 0 && listid < len(cfgp.Lists) {
		listname = cfgp.Lists[listid].Name
	}

	audiobookListID := database.GetAudiobookListEntryID(album.DatabaseID, listname)
	if audiobookListID == 0 {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Uint("dbaudiobookID", album.DatabaseID).
			Str("listname", listname).
			Msg("Could not find audiobook list entry for dbaudiobook_id")

		return false
	}

	// Step 6a: Update rootpath
	if err := database.UpdateAudiobookRootpath(audiobookListID, folder); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to update audiobook rootpath")
	}

	// Add each file to the database
	filesAdded := 0

	filesSkipped := 0
	for i := range album.Tracks {
		if err := ctx.Err(); err != nil {
			return false
		}

		// Skip files already in database
		if database.AudiobookFileExists(&album.Tracks[i].Filepath) {
			filesSkipped++
			continue
		}

		// Insert file record (audiobookListID = audiobooks.id, album.DatabaseID = dbaudiobook_id)
		if err := database.InsertAudiobookFile(
			audiobookListID,
			album.Tracks[i].Filepath,
			album.Tracks[i].Filename,
			album.Tracks[i].Extension,
			album.Tracks[i].Format,
			album.Tracks[i].QualityProfile,
			album.Tracks[i].FileSize,
			album.Tracks[i].Bitrate,
			album.Tracks[i].RuntimeMS,
			album.Tracks[i].TrackNumber,
			album.Tracks[i].DiscNumber,
			album.DatabaseID,
		); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to insert audiobook file")

			continue
		}

		filesAdded++

		// Step 6b: Tag file if added (add to cache)
		if config.GetSettingsGeneral().UseFileCache {
			database.AppendCacheMap(cfgp.IsType, logger.CacheFiles, album.Tracks[i].Filepath)
		}
	}

	logger.Logtype("debug", 0).
		Str("folder", folder).
		Int("filesAdded", filesAdded).
		Int("filesSkipped", filesSkipped).
		Int("totalTracks", len(album.Tracks)).
		Msg("DEBUG: addAudiobookFilesToDatabase complete")

	if filesAdded > 0 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("title", album.Title).
			Uint("audiobook_id", album.DatabaseID).
			Int("files_added", filesAdded).
			Msg("Added audiobook files to database")

		// Update missing flag to 0 since we now have files
		updateMissing := mtstrings.GetStringsMap(cfgp.IsType, "UpdateMissingByID")
		database.ExecN(updateMissing, &audiobookListID)

		// Update quality_reached based on current file priority vs cutoff
		var reached int
		if listid >= 0 && listid < len(cfgp.Lists) && cfgp.Lists[listid].CfgQuality != nil {
			prio, _ := searcher.GetpriobyfilesAudio(
				cfgp.IsType,
				&audiobookListID,
				false,
				-1,
				cfgp.Lists[listid].CfgQuality,
				false,
			)
			if prio >= cfgp.Lists[listid].CfgQuality.CutoffPriority {
				reached = 1
			}
		}

		updateReached := mtstrings.GetStringsMap(cfgp.IsType, "UpdateQualityReachedByID")
		database.ExecN(updateReached, &reached, &audiobookListID)
	}

	return true
}

// addAlbumFilesToDatabase adds music album files to the database.
func addAlbumFilesToDatabase(
	ctx context.Context,
	folder string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	listid int,
) bool {
	if album.DatabaseID == 0 || listid == -1 {
		return false
	}

	// Resolve the correct albums.id from dbalbum_id + listname
	// album.DatabaseID is dbalbum_id, NOT albums.id
	var listname string
	if listid >= 0 && listid < len(cfgp.Lists) {
		listname = cfgp.Lists[listid].Name
	}

	albumListID := database.GetAlbumListEntryID(album.DatabaseID, listname)
	if albumListID == 0 {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Uint("dbalbumID", album.DatabaseID).
			Str("listname", listname).
			Msg("Could not find album list entry for dbalbum_id")

		return false
	}

	// Step 6a: Update rootpath using the correct albums.id
	if err := database.UpdateAlbumRootpath(albumListID, folder); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to update album rootpath")
	}

	// Add each file to the database
	filesAdded := 0
	for i := range album.Tracks {
		if err := ctx.Err(); err != nil {
			return false
		}

		// Skip files already in database
		if database.AlbumFileExists(&album.Tracks[i].Filepath) {
			continue
		}

		// Look up matching dbtrack_id from dbtracks table
		var dbtrackID uint
		database.Scanrowsdyn(
			false,
			"SELECT id FROM dbtracks WHERE dbalbum_id = ? AND track_number = ? AND disc_number = ?",
			&dbtrackID,
			&album.DatabaseID,
			&album.Tracks[i].TrackNumber,
			&album.Tracks[i].DiscNumber,
		)

		// logger.Logtype("debug", 2).
		// 	Uint("dbalbum_id", album.DatabaseID).
		// 	Int("track_number", track.TrackNumber).
		// 	Int("disc_number", track.DiscNumber).
		// 	Uint("dbtrack_id", dbtrackID).
		// 	Str("file", track.Filepath).
		// 	Msg("Matched album file to track in addAlbumFilesToDatabase")

		// Insert file record — albumListID is albums.id, album.DatabaseID is dbalbum_id
		trackFile := &database.TrackFileInfo{
			Filepath:       album.Tracks[i].Filepath,
			Filename:       album.Tracks[i].Filename,
			Extension:      album.Tracks[i].Extension,
			Format:         album.Tracks[i].Format,
			QualityProfile: album.Tracks[i].QualityProfile,
			FileSize:       album.Tracks[i].FileSize,
			Bitrate:        album.Tracks[i].Bitrate,
			SampleRate:     album.Tracks[i].SampleRate,
			BitDepth:       album.Tracks[i].BitDepth,
			RuntimeMS:      album.Tracks[i].RuntimeMS,
			TrackNumber:    album.Tracks[i].TrackNumber,
			DiscNumber:     album.Tracks[i].DiscNumber,
		}
		if err := database.InsertAlbumFile(
			albumListID,
			trackFile,
			album.Tracks[i].AcoustID,
			album.DatabaseID,
			dbtrackID,
		); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to insert album file")

			continue
		}

		filesAdded++

		// Step 6b: Tag file if added (add to cache)
		if config.GetSettingsGeneral().UseFileCache {
			database.AppendCacheMap(cfgp.IsType, logger.CacheFiles, album.Tracks[i].Filepath)
		}
	}

	if filesAdded > 0 {
		// logger.Logtype("info", 1).
		// 	Str("folder", folder).
		// 	Str("title", album.Title).
		// 	Uint("album_id", albumListID).
		// 	Int("files_added", filesAdded).
		// 	Msg("Added album files to database")

		// Update missing flag to 0 since we have files now — uses albums.id
		updateMissing := mtstrings.GetStringsMap(cfgp.IsType, "UpdateMissingByID")
		database.ExecN(updateMissing, &albumListID)

		// Update quality_reached based on current file priority vs cutoff
		var reached int
		if listid >= 0 && listid < len(cfgp.Lists) && cfgp.Lists[listid].CfgQuality != nil {
			prio, _ := searcher.GetpriobyfilesAudio(
				cfgp.IsType,
				&albumListID,
				false,
				-1,
				cfgp.Lists[listid].CfgQuality,
				false,
			)
			if prio >= cfgp.Lists[listid].CfgQuality.CutoffPriority {
				reached = 1
			}
		}

		updateReached := mtstrings.GetStringsMap(cfgp.IsType, "UpdateQualityReachedByID")
		database.ExecN(updateReached, &reached, &albumListID)

		// logger.Logtype("debug", 0).
		// 	Str("folder", folder).
		// 	Uint("album_id", albumListID).
		// 	Msg("DEBUG: Updated missing flag to 0")
	}

	return true
}

// SingleJobs runs a single maintenance job for the given media config and list.
// It handles running jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the
// job string and dispatched to internal functions. List can be empty to run for all lists.
func SingleJobs(
	rootctx context.Context,
	job, cfgpstr, listname string,
	force bool,
	key uint32,
) (finalErr error) {
	var dbinsert int64

	// Panic recovery to ensure job completion tracking
	defer func() {
		r := recover()
		if r == nil {
			return
		}

		logger.Logtype("error", 0).
			Str("job", job).
			Uint32("key", key).
			Any("panic", logger.Stack()).
			Msg("SingleJobs: Job panicked, ensuring completion tracking")

		// Always update job history on panic
		if dbinsert != 0 {
			database.ExecN(database.QueryUpdateHistory, &dbinsert)
		}

		// Convert panic to error
		if err, ok := r.(error); ok {
			finalErr = err
		} else {
			finalErr = errors.New("job panicked")
		}
	}()

	if job == "" || cfgpstr == "" || (config.GetSettingsGeneral().SchedulerDisabled && !force) {
		logjob("skipped Job", cfgpstr, listname, job)
		return nil
	}

	if err := logger.CheckContextEnded(rootctx); err != nil {
		logjob("Job cancelled - context ended", cfgpstr, listname, job)
		return err
	}

	cfgp := config.GetSettingsMedia(cfgpstr)
	if cfgpstr != "" && cfgp == nil {
		config.LoadCfgDB(true)

		cfgp = config.GetSettingsMedia(cfgpstr)
		if cfgp == nil {
			logjob("config not found", cfgpstr, listname, job)
			return errors.New("config not found")
		}

		if cfgp.IsType != config.MediaTypeSeries &&
			(job == logger.StrRssSeasons || job == logger.StrRssSeasonsAll) {
			return nil
		}
	}

	// Try to acquire job lock for Data, DataFull, and Structure jobs to prevent concurrent execution
	if job == logger.StrData || job == logger.StrDataFull || job == logger.StrStructure {
		if !tryAcquireJobLock(cfgpstr) {
			logger.Logtype("info", 1).
				Str(logger.StrConfig, cfgpstr).
				Str(logger.StrJob, job).
				Str(logger.StrListname, listname).
				Msg("Job skipped - another job is already running for this configuration")

			return nil
		}

		// Ensure lock is released when job completes
		defer releaseJobLock(cfgpstr)
	}

	logjob("Started Job", cfgpstr, listname, job)
	Refreshcache(cfgp.IsType)

	dbinsert = insertjobhistory(job, cfgp)

	idxlist := -2
	if job == logger.StrData || job == logger.StrRss || job == logger.StrReachedFlag ||
		job == logger.StrClearHistory ||
		job == logger.StrFeeds ||
		job == logger.StrCheckMissing ||
		job == logger.StrCheckMissingFlag {
		if job == logger.StrRss {
			for idx := range cfgp.ListsQualities {
				if err := logger.CheckContextEnded(rootctx); err != nil {
					return err
				}

				searcher.NewSearcher(cfgp, config.GetSettingsQuality(cfgp.ListsQualities[idx]), logger.StrRss, nil).
					SearchRSS(rootctx, cfgp, config.GetSettingsQuality(cfgp.ListsQualities[idx]), true, true)
			}
		} else {
			if _, ok := cfgp.ListsMapIdx[listname]; ok {
				idxlist = cfgp.ListsMapIdx[listname]
			}
		}
	} else {
		idxlist = -1
	}

	var err error
	if idxlist != -2 {
		err = runjoblistfunc(rootctx, job, cfgp, idxlist)
		if err != nil {
			logger.Logtype("error", 0).
				Str("job", job).
				Uint32("key", key).
				Err(err).
				Msg("SingleJobs: Error running runjoblistfunc")
		}
	}

	logjob("Ended Job", cfgpstr, listname, job)

	// Always update job history completion, even on error
	if dbinsert != 0 {
		database.ExecN(database.QueryUpdateHistory, &dbinsert)
	}

	return err
}

// logjob logs information about a job, including the action, configuration, list name, and job name.
// It also logs the current number of goroutines.
func logjob(act, cfgp, listname, job string) {
	logger.Logtype("info", 1).
		Str(logger.StrConfig, cfgp).
		Str(logger.StrJob, job).
		Str(logger.StrListname, listname).
		Int(NumGo, runtime.NumGoroutine()).
		Msg(act)
}

// cacheRefreshKeys holds the cache keys to refresh, defined at package level to avoid allocation.
var cacheRefreshKeys = []string{
	logger.CacheMediaTitles,
	logger.CacheFiles,
	logger.CacheUnmatched,
	"CacheHistoryUrl",
	"CacheHistoryTitle",
	logger.CacheMedia,
	logger.CacheDBMedia,
}

// allCacheTypes holds all cache types for full refresh, defined at package level to avoid allocation.
var allCacheTypes = []string{
	logger.CacheMovie,
	logger.CacheSeries,
	logger.CacheDBMovie,
	logger.CacheDBSeries,
	logger.CacheDBSeriesAlt,
	logger.CacheTitlesMovie,
	logger.CacheUnmatchedMovie,
	logger.CacheUnmatchedSeries,
	logger.CacheFilesMovie,
	logger.CacheFilesSeries,
	logger.CacheHistoryURLMovie,
	logger.CacheHistoryTitleMovie,
	logger.CacheHistoryURLSeries,
	logger.CacheHistoryTitleSeries,
	// Book caches
	logger.CacheBook,
	logger.CacheDBBook,
	logger.CacheTitlesBook,
	logger.CacheUnmatchedBook,
	logger.CacheFilesBook,
	logger.CacheHistoryURLBook,
	logger.CacheHistoryTitleBook,
	// Audiobook caches
	logger.CacheAudiobook,
	logger.CacheDBAudiobook,
	logger.CacheTitlesAudiobook,
	logger.CacheUnmatchedAudiobook,
	logger.CacheFilesAudiobook,
	logger.CacheHistoryURLAudiobook,
	logger.CacheHistoryTitleAudiobook,
	// Album/Music caches
	logger.CacheAlbum,
	logger.CacheDBAlbum,
	logger.CacheTitlesAlbum,
	logger.CacheUnmatchedAlbum,
	logger.CacheFilesAlbum,
	logger.CacheHistoryURLAlbum,
	logger.CacheHistoryTitleAlbum,
}

// Refreshcache refreshes various database caches used for performance.
// It refreshes the history cache, media cache, media titles cache,
// unmatched cache, and files cache.
// The isType parameter determines if it should refresh for series
// or movies.
func Refreshcache(isType uint) {
	for _, str := range cacheRefreshKeys {
		database.RefreshCached(mtstrings.GetStringsMap(isType, str), false)
	}
}

// runjoblistfunc executes the specified job for the given media config and list index.
// It handles running various maintenance jobs like getting new files, checking for missing files,
// refreshing data, searching for upgrades, etc. Jobs are determined by the job string and dispatch
// to the appropriate internal functions. List index of -1 runs the job for all lists in the config.
func runjoblistfunc(
	rootctx context.Context,
	job string,
	cfgp *config.MediaTypeConfig,
	listid int,
) error {
	if job == "" || cfgp == nil {
		return errors.New("job or config not found")
	}

	if err := logger.CheckContextEnded(rootctx); err != nil {
		return err
	}

	switch job {
	case logger.StrData, logger.StrDataFull:
		var err error
		for _, data := range cfgp.DataMap {
			if err := logger.CheckContextEnded(rootctx); err != nil {
				return err
			}

			if errsub := newfilesloop(rootctx, cfgp, data); errsub != nil {
				err = errsub
			}
		}

		return err

	case logger.StrCheckMissing:
		return checkmissing(rootctx, cfgp.IsType, &cfgp.Lists[listid])
	case "cleanqueue":
		return worker.Cleanqueue()
	case logger.StrCheckMissingFlag:
		return checkmissingflag(rootctx, cfgp.IsType, &cfgp.Lists[listid])
	case logger.StrReachedFlag:
		return checkreachedflag(rootctx, cfgp, &cfgp.Lists[listid])

	case logger.StrStructure:
		err := structurefolders(rootctx, cfgp)
		return err

	case logger.StrRssSeasons:
		return searcher.SearchSeriesRSSSeasons(rootctx, cfgp)
	case logger.StrRssSeasonsAll:
		return searcher.SearchSeriesRSSSeasonsAll(rootctx, cfgp)
	case logger.StrRssArtists:
		return searcher.SearchArtistMissing(rootctx, cfgp)
	case logger.StrRssArtistsUpgrade:
		return searcher.SearchArtistUpgrade(rootctx, cfgp)
	case logger.StrRssAuthors:
		if cfgp.IsType == config.MediaTypeAudiobook {
			return searcher.SearchAuthorAudiobookMissing(rootctx, cfgp)
		}

		return searcher.SearchAuthorBookMissing(rootctx, cfgp)

	case logger.StrRssAuthorsUpgrade:
		if cfgp.IsType == config.MediaTypeAudiobook {
			return searcher.SearchAuthorAudiobookUpgrade(rootctx, cfgp)
		}

		return searcher.SearchAuthorBookUpgrade(rootctx, cfgp)

	case "refreshinc":
		if h := mediatype.Get(cfgp.IsType); h != nil {
			return h.Refresh(rootctx, cfgp, h.GetRefreshIncData())
		}

	case "refresh":
		if h := mediatype.Get(cfgp.IsType); h != nil {
			return h.Refresh(rootctx, cfgp, h.GetRefreshFullData())
		}

	case logger.StrClearHistory:
		clearHistory(cfgp, listid)
		return nil

	case logger.StrFeeds:
		if cfgp.Lists[listid].CfgList != nil {
			err := importnewsingle(rootctx, cfgp, &cfgp.Lists[listid], listid)

			if err != nil && !errors.Is(err, logger.ErrDisabled) {
				logger.Logtype("error", 1).
					Str(logger.StrListname, cfgp.Lists[listid].Name).
					Err(err).
					Msg("import feeds failed")
			}

			return err
		}

		logger.Logtype("error", 1).
			Str(logger.StrListname, cfgp.Lists[listid].Name).
			Msg("import feeds failed - no cfgp list")

	case logger.StrRss:
		return nil
	case logger.StrSearchMissingFull,
		logger.StrSearchMissingInc,
		logger.StrSearchUpgradeFull,
		logger.StrSearchUpgradeInc,
		logger.StrSearchMissingFullTitle,
		logger.StrSearchMissingIncTitle,
		logger.StrSearchUpgradeFullTitle,
		logger.StrSearchUpgradeIncTitle:
		var (
			searchinterval             uint16
			searchmissing, searchtitle bool
		)

		if strings.Contains(job, "missing") {
			searchmissing = true
		}

		if strings.Contains(job, "title") {
			searchtitle = true
		}

		if strings.Contains(job, "inctitle") {
			searchtitle = true

			searchinterval = cfgp.SearchmissingIncremental
			if searchinterval == 0 {
				searchinterval = 20
			}
		}

		if strings.HasSuffix(job, "inc") {
			searchinterval = cfgp.SearchmissingIncremental
			if searchinterval == 0 {
				searchinterval = 20
			}
		}

		return jobsearchmedia(rootctx, cfgp, searchmissing, searchtitle, searchinterval)

	default:
		logger.Logtype("error", 1).
			Str(logger.StrJob, job).
			Msg("Switch not found")
		return errors.New("switch not found")
	}

	return nil
}

// getSearchQueryForMedia returns the search query string for a media item.
// For music, this is "Title Artist". For audiobooks, this is "Title Author".
// Used to identify duplicate searches for different releases of the same album.
func getSearchQueryForMedia(mediaType uint, mediaid uint) string {
	if mediaid == 0 {
		return ""
	}

	var title, artistAuthor string

	switch mediaType {
	case config.MediaTypeMusic:
		// Get album title and artist name
		database.GetdatarowArgs(
			"SELECT dbalbums.title FROM albums INNER JOIN dbalbums ON dbalbums.id=albums.dbalbum_id WHERE albums.id = ?",
			&mediaid,
			&title,
		)
		database.GetdatarowArgs(
			"SELECT dbartists.name FROM dbalbum_artists INNER JOIN dbartists ON dbartists.id=dbalbum_artists.dbartist_id INNER JOIN albums ON albums.dbalbum_id=dbalbum_artists.dbalbum_id WHERE albums.id = ? ORDER BY dbalbum_artists.position LIMIT 1",
			&mediaid,
			&artistAuthor,
		)

	case config.MediaTypeAudiobook:
		// Get audiobook title and author name
		database.GetdatarowArgs(
			"SELECT dbaudiobooks.title FROM audiobooks INNER JOIN dbaudiobooks ON dbaudiobooks.id=audiobooks.dbaudiobook_id WHERE audiobooks.id = ?",
			&mediaid,
			&title,
		)
		database.GetdatarowArgs(
			"SELECT dbauthors.name FROM dbaudiobook_authors INNER JOIN dbauthors ON dbauthors.id=dbaudiobook_authors.dbauthor_id INNER JOIN audiobooks ON audiobooks.dbaudiobook_id=dbaudiobook_authors.dbaudiobook_id WHERE audiobooks.id = ? ORDER BY dbaudiobook_authors.position LIMIT 1",
			&mediaid,
			&artistAuthor,
		)

	default:
		return ""
	}

	if title == "" {
		return ""
	}

	// Build search query similar to FillSearchVar
	if artistAuthor != "" && artistAuthor != "Various Artists" && artistAuthor != "VA" &&
		artistAuthor != "Various" {
		return title + " " + artistAuthor
	}

	return title
}

// jobsearchmedia searches for media items that need to be searched for new releases
// or missing files based on the provided config, search type, and interval.
// It builds a database query, executes it to get a list of media IDs,
// then calls MediaSearch on each ID to perform the actual search.
func jobsearchmedia(
	ctx context.Context, cfgp *config.MediaTypeConfig,
	searchmissing, searchtitle bool,
	searchinterval uint16,
) error {
	if err := logger.CheckContextEnded(ctx); err != nil {
		return err
	}

	var (
		scaninterval int
		scandatepre  int
	)

	if cfgp.DataLen >= 1 && cfgp.Data[0].CfgPath != nil {
		if searchmissing {
			scaninterval = cfgp.Data[0].CfgPath.MissingScanInterval
			scandatepre = cfgp.Data[0].CfgPath.MissingScanReleaseDatePre
		} else {
			scaninterval = cfgp.Data[0].CfgPath.UpgradeScanInterval
		}
	}

	if cfgp.ListsLen == 0 {
		return errors.New("no lists")
	}

	args := logger.PLArrAny.Get()
	defer logger.PLArrAny.Put(args)

	for _, lst := range cfgp.ListsMap {
		args.Arr = append(args.Arr, &lst.Name)
	}

	if len(args.Arr) == 0 {
		return errors.New("no lists")
	}

	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	bld.WriteStringMap(cfgp.IsType, logger.SearchGenTable)

	if searchmissing {
		bld.WriteStringMap(cfgp.IsType, logger.SearchGenMissing)
		bld.WriteString(cfgp.ListsQu)
		bld.WriteStringMap(cfgp.IsType, logger.SearchGenMissingEnd)
	} else {
		bld.WriteStringMap(cfgp.IsType, logger.SearchGenReached)
		bld.WriteString(cfgp.ListsQu)
		bld.WriteByte(')')
	}

	if scaninterval != 0 {
		bld.WriteStringMap(cfgp.IsType, logger.SearchGenLastScan)

		timeinterval := logger.TimeGetNow().AddDate(0, 0, 0-scaninterval)

		args.Arr = append(args.Arr, &timeinterval)
	}

	if scandatepre != 0 {
		bld.WriteStringMap(cfgp.IsType, logger.SearchGenDate)

		timedatepre := logger.TimeGetNow().AddDate(0, 0, 0+scandatepre)

		args.Arr = append(args.Arr, &timedatepre)
	}

	bld.WriteStringMap(cfgp.IsType, logger.SearchGenOrder)

	if searchinterval != 0 {
		bld.WriteString(" limit ")
		bld.WriteUInt16(searchinterval)
	}

	str := bld.String()

	var err error

	// Track searched queries to avoid duplicate searches for albums with same name
	// This is especially useful for music/audiobooks where multiple releases of the same
	// album may exist with different track counts or editions
	searchedQueries := make(map[string]struct{})

	for _, tbl := range database.GetrowsNuncached[database.DbstaticOneStringOneUInt](database.Getdatarow[uint](false, logger.JoinStrings("select count() ", str), args.Arr...), logger.JoinStrings(mtstrings.GetStringsMap(cfgp.IsType, logger.SearchGenSelect), str), args.Arr) {
		// For music and audiobooks, check if we've already searched for this title+artist
		// to avoid redundant searches for different releases of the same album
		if cfgp.IsType == config.MediaTypeMusic || cfgp.IsType == config.MediaTypeAudiobook {
			searchQuery := getSearchQueryForMedia(cfgp.IsType, tbl.Num)
			if searchQuery != "" {
				queryLower := strings.ToLower(searchQuery)
				if _, exists := searchedQueries[queryLower]; exists {
					logger.Logtype("debug", 2).
						Uint("mediaid", tbl.Num).
						Str("query", searchQuery).
						Msg("Skipping duplicate search - same title already searched")

					continue
				}

				searchedQueries[queryLower] = struct{}{}
			}
		}

		if errsub := searcher.NewSearcher(cfgp, cfgp.GetMediaQualityConfigStr(tbl.Str), "", nil).
			MediaSearch(ctx, cfgp, tbl.Num, searchtitle, true, true); errsub != nil {
			err = errsub
		}
	}

	return err
}

// checkmissing checks for missing files for the given media list.
// It queries for file locations, checks if they exist, and updates
// the database to set missing flags on media items with no files.
// It handles both movies and series based on the isType flag.
func checkmissing(rootctx context.Context, isType uint, listcfg *config.MediaListsConfig) error {
	arrfiles := database.GetrowsN[syncops.DbstaticOneStringTwoInt](
		false,
		database.Getdatarow[uint](
			false,
			mtstrings.GetStringsMap(isType, logger.DBCountFilesByLocation),
		),
		mtstrings.GetStringsMap(isType, logger.DBIDsFilesByLocation),
	)
	arr := database.Getrowssize[string](
		false,
		mtstrings.GetStringsMap(isType, logger.DBCountFilesByList),
		mtstrings.GetStringsMap(isType, logger.DBLocationFilesByList),
		&listcfg.Name,
	)

	var err error
	for idx := range arr {
		if err := logger.CheckContextEnded(rootctx); err != nil {
			return err
		}

		if scanner.CheckFileExist(arr[idx]) {
			continue
		}

		if errsub := checkmissingfiles(isType, &arr[idx], arrfiles); errsub != nil {
			err = errsub
		}
	}

	return err
}

// func Checkruntimes(cfg *config.MediaTypeConfig) {
// 	arr := database.GetrowsN[database.DbstaticOneStringTwoInt](false, database.Getdatarow(false, mtstrings.GetStringsMap(cfg.IsType, logger.DBCountFilesByLocation)), mtstrings.GetStringsMap(cfg.IsType, logger.DBIDsFilesByLocation))
// 	for idx := range arr {
// 		Checkruntimesfiles(cfg, &arr[idx])
// 	}
// }

// checkmissingfiles checks for missing files for a given media item.
// It deletes the file record if missing, and updates the missing flag on the media item if it has no more files.
// It takes the query to count files for the media item, the table to delete from,
// the table to update the missing flag, the query to get the file ID and media item ID,
// and the file location that was found missing.
func checkmissingfiles(
	isType uint,
	row *string,
	arrfiles []syncops.DbstaticOneStringTwoInt,
) error {
	subquerycount := mtstrings.GetStringsMap(isType, logger.DBCountFilesByMediaID)
	deletestmt := logger.JoinStrings(
		"delete from ",
		mtstrings.GetStringsMap(isType, logger.TableFiles),
		" where id = ?",
	)
	updatestmt := logger.JoinStrings(
		"update ",
		mtstrings.GetStringsMap(isType, logger.TableMedia),
		" set missing = 1 where id = ?",
	)

	var errret error
	for idx := range arrfiles {
		if arrfiles[idx].Str != *row {
			continue
		}

		logger.Logtype("info", 1).
			Str(logger.StrFile, *row).
			Msg("File was removed")

		err := database.ExecNErr(deletestmt, &arrfiles[idx].Num1)
		if err != nil {
			errret = err
			continue
		}

		if database.Getdatarow[uint](false, subquerycount, &arrfiles[idx].Num2) == 0 {
			database.ExecN(updatestmt, &arrfiles[idx].Num2)
		}
	}

	return errret
}

// checkmissingflag checks for missing files for the given media list.
// It updates the missing flag in the database based on file count.
func checkmissingflag(
	rootctx context.Context,
	isType uint,
	listcfg *config.MediaListsConfig,
) error {
	queryupdate := mtstrings.GetStringsMap(isType, logger.DBUpdateMissing)
	querycount := mtstrings.GetStringsMap(isType, logger.DBCountFilesByMediaID)

	var counter int

	arr := database.Getrowssize[database.DbstaticOneIntOneBool](
		false,
		mtstrings.GetStringsMap(isType, logger.DBCountMediaByList),
		mtstrings.GetStringsMap(isType, logger.DBIDMissingMediaByList),
		&listcfg.Name,
	)
	for idx := range arr {
		if err := logger.CheckContextEnded(rootctx); err != nil {
			return err
		}

		database.Scanrowsdyn(false, querycount, &counter, &arr[idx].Num)

		if counter >= 1 && arr[idx].Bl {
			database.ExecN(queryupdate, &v0, &arr[idx].Num)
		}

		if counter == 0 && !arr[idx].Bl {
			database.ExecN(queryupdate, &v1, &arr[idx].Num)
		}
	}

	return nil
}

// LoadGlobalSchedulerConfig initializes the global scheduler job functions that are
// not media-specific. These jobs include database maintenance, backup operations,
// and system-wide tasks. The functions are registered in the general settings
// job map for use by the worker scheduler system.
func LoadGlobalSchedulerConfig() {
	config.GetSettingsGeneral().Jobs = map[string]func(uint32, context.Context) error{
		"RefreshImdb": func(key uint32, ctx context.Context) error {
			FillImdb()
			worker.RemoveQueueEntry(key)
			return nil
		},
		"CheckDatabase": func(key uint32, ctx context.Context) error {
			worker.RemoveQueueEntry(key)

			if database.DBIntegrityCheck() != "ok" {
				os.Exit(100)
			}

			return nil
		},
		"BackupDatabase": func(key uint32, ctx context.Context) error {
			if config.GetSettingsGeneral().DatabaseBackupStopTasks {
				worker.StopCronWorker()
				worker.CloseWorkerPools()
			}

			worker.RemoveQueueEntry(key)

			backupto := logger.JoinStrings(
				"./backup/data.db.",
				database.GetVersion(),
				logger.StrDot,
				time.Now().Format("20060102_150405"),
			)

			err := database.Backup(&backupto, config.GetSettingsGeneral().MaxDatabaseBackups)
			if config.GetSettingsGeneral().DatabaseBackupStopTasks {
				worker.InitWorkerPools(
					config.GetSettingsGeneral().WorkerSearch,
					config.GetSettingsGeneral().WorkerFiles,
					config.GetSettingsGeneral().WorkerMetadata,
					config.GetSettingsGeneral().WorkerRSS,
					config.GetSettingsGeneral().WorkerIndexer,
				)
				worker.StartCronWorker()
			}

			return err
		},
		"RefreshCache": func(key uint32, ctx context.Context) error {
			logger.Logtype("info", 0).Msg("Starting scheduled cache refresh")

			for _, cacheType := range allCacheTypes {
				logger.Logtype("debug", 0).Str("type", cacheType).Msg("Refreshing cache")
				database.RefreshCached(cacheType, true)
			}

			logger.Logtype("info", 0).Msg("Completed scheduled cache refresh")
			worker.RemoveQueueEntry(key)

			return nil
		},
	}
}

// LoadSchedulerConfig initializes the media-specific scheduler job functions for each
// configured media type (movies and series). These jobs include search operations,
// data processing, feed imports, and maintenance tasks. The function iterates through
// all media configurations and registers appropriate job functions based on media type.
func LoadSchedulerConfig() {
	config.RangeSettingsMedia(func(_ string, cfgp *config.MediaTypeConfig) error {
		h := mediatype.Get(cfgp.IsType)
		if h == nil {
			return nil
		}

		cfgp.Jobs = make(map[string]func(uint32, context.Context) error)
		for _, jobPair := range h.GetSchedulerJobNames() {
			schedulerName, singleJobName := jobPair[0], jobPair[1]

			cfgp.Jobs[schedulerName] = makeJobFunc(singleJobName, cfgp.NamePrefix)
		}

		return nil
	})
}

// makeJobFunc creates a job function closure for the scheduler.
func makeJobFunc(jobName, namePrefix string) func(uint32, context.Context) error {
	return func(key uint32, ctx context.Context) error {
		defer worker.RemoveQueueEntry(key)
		return SingleJobs(ctx, jobName, namePrefix, "", false, key)
	}
}

// MediaTypeAll is a sentinel value to run jobs for all media types.
const MediaTypeAll uint = ^uint(0)

// AllJobs runs the specified job for all configured media types matching the given type.
// Use MediaTypeAll to run for all types, or pass a specific type
// (e.g., config.MediaTypeMovie, config.MediaTypeSeries) to filter.
func AllJobs(mediaType uint, job string, force bool) {
	if job == "" {
		return
	}

	ctx := context.Background()

	logger.Logtype("debug", 1).
		Str(logger.StrJob, job).
		Msg("Started Job for all")

	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		if mediaType != MediaTypeAll && media.IsType != mediaType {
			return nil
		}

		if mediatype.Get(media.IsType) != nil {
			return SingleJobs(ctx, job, media.NamePrefix, "", force, 0)
		}

		return nil
	})
}

package api

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// getPathsToScan returns all media paths that should be scanned for cleanup.
func getPathsToScan(paths string) []string {
	var scanPaths []string

	if paths != "" {
		// Use user-provided paths
		scanPaths = strings.Split(strings.TrimSpace(paths), "\n")
	} else {
		// Use all configured media paths
		media := config.GetSettingsMediaAll()
		for i := range media.Movies {
			for _, pathCfg := range media.Movies[i].Data {
				if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
					scanPaths = append(scanPaths, pathCfg.CfgPath.Path)
				}
			}
		}

		for i := range media.Series {
			for _, pathCfg := range media.Series[i].Data {
				if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
					scanPaths = append(scanPaths, pathCfg.CfgPath.Path)
				}
			}
		}
	}

	// Remove duplicates and empty paths
	pathMap := make(map[string]bool)

	var uniquePaths []string
	for _, path := range scanPaths {
		cleanPath := strings.TrimSpace(path)
		if cleanPath != "" && !pathMap[cleanPath] {
			pathMap[cleanPath] = true
			uniquePaths = append(uniquePaths, cleanPath)
		}
	}

	return uniquePaths
}

// findOrphanedFiles finds files on disk that are not tracked in the database.
func findOrphanedFiles(scanPaths []string, mediaTypes string, minFileSize int64) []string {
	orphanedFiles := make([]string, 0)

	// Get all files from database
	dbFiles := make(map[string]bool)

	if mediaTypes == "all" || mediaTypes == "movies" {
		movieFiles := database.StructscanT[database.MovieFile](
			false,
			0,
			"SELECT location FROM movie_files",
		)
		for i := range movieFiles {
			dbFiles[movieFiles[i].Location] = true
		}
	}

	if mediaTypes == "all" || mediaTypes == "series" {
		serieFiles := database.StructscanT[database.SerieEpisodeFile](
			false,
			0,
			"SELECT location FROM serie_episode_files",
		)
		for i := range serieFiles {
			dbFiles[serieFiles[i].Location] = true
		}
	}

	// Scan filesystem and check against database
	for _, basePath := range scanPaths {
		if err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			// Check if it's a media file and meets size requirements
			if !isMediaFile(path) {
				return nil
			}

			if minFileSize > 0 {
				if info, statErr := d.Info(); statErr != nil || info.Size() < minFileSize {
					return nil
				}
			}

			// Check if file is tracked in database
			if !dbFiles[path] {
				orphanedFiles = append(orphanedFiles, path)
			}

			return nil
		}); err != nil {
			logger.Logtype("error", 1).
				Str("path", basePath).
				Err(err).
				Msg("Failed to walk directory for orphaned files")
		}
	}

	return orphanedFiles
}

// findDuplicateFiles finds files with identical sizes or checksums.
func findDuplicateFiles(scanPaths []string, minFileSize int64) [][]string {
	sizeMap := make(map[int64][]string)

	var duplicateGroups [][]string

	// Scan all files and group by size
	for _, basePath := range scanPaths {
		if err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !isMediaFile(path) {
				return nil
			}

			info, statErr := d.Info()
			if statErr != nil {
				return nil
			}

			size := info.Size()
			if minFileSize > 0 && size < minFileSize {
				return nil
			}

			sizeMap[size] = append(sizeMap[size], path)
			return nil
		}); err != nil {
			logger.Logtype("error", 1).
				Str("path", basePath).
				Err(err).
				Msg("Failed to walk directory for duplicate files")
		}
	}

	// Group files that have the same size (potential duplicates)
	for _, files := range sizeMap {
		if len(files) > 1 {
			duplicateGroups = append(duplicateGroups, files)
		}
	}

	return duplicateGroups
}

// findBrokenLinks finds database entries pointing to missing files.
func findBrokenLinks(mediaTypes string) []string {
	brokenLinks := make([]string, 0)

	// Check movie files if movies are included
	if mediaTypes == "all" || mediaTypes == "movies" {
		movieFiles := database.StructscanT[database.MovieFile](
			false,
			0,
			"SELECT location FROM movie_files",
		)
		for i := range movieFiles {
			if !checkFileExists(movieFiles[i].Location) {
				brokenLinks = append(brokenLinks, movieFiles[i].Location)
			}
		}
	}

	// Check serie files if series are included
	if mediaTypes == "all" || mediaTypes == "series" {
		serieFiles := database.StructscanT[database.SerieEpisodeFile](
			false,
			0,
			"SELECT location FROM serie_episode_files",
		)
		for i := range serieFiles {
			if !checkFileExists(serieFiles[i].Location) {
				brokenLinks = append(brokenLinks, serieFiles[i].Location)
			}
		}
	}

	return brokenLinks
}

// findEmptyDirectories finds directories with no media files.
func findEmptyDirectories(scanPaths []string, minFileSize int64) []string {
	emptyDirs := make([]string, 0)

	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		if err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || !d.IsDir() || path == basePath {
				return nil
			}

			// Check if directory has any media files
			hasMediaFiles := false
			if err := filepath.WalkDir(path, func(filePath string, fileInfo os.DirEntry, fileErr error) error {
				if fileErr != nil || fileInfo.IsDir() {
					return nil
				}

				// Check if it's a media file and meets size requirements
				if isMediaFile(filePath) {
					if minFileSize == 0 {
						hasMediaFiles = true
						return filepath.SkipAll
					}

					if info, statErr := fileInfo.Info(); statErr == nil && info.Size() >= minFileSize {
						hasMediaFiles = true
						return filepath.SkipAll
					}
				}
				return nil
			}); err != nil {
				logger.Logtype("error", 1).
					Str("path", path).
					Err(err).
					Msg("Failed to walk directory checking for media files")
			}

			if !hasMediaFiles {
				emptyDirs = append(emptyDirs, path)
			}

			return nil
		}); err != nil {
			logger.Logtype("error", 1).
				Str("path", basePath).
				Err(err).
				Msg("Failed to walk directory for empty directories")
		}
	}

	return emptyDirs
}

// performCleanupActions executes cleanup operations based on findings.
func performCleanupActions(
	orphanedFiles []string,
	duplicateGroups [][]string,
	brokenLinks []string,
	emptyDirs []string,
	dryRun bool,
) CleanupResults {
	results := CleanupResults{
		OrphanedFiles:    len(orphanedFiles),
		DuplicateFiles:   0,
		BrokenLinks:      len(brokenLinks),
		EmptyDirectories: len(emptyDirs),
		ActionsPerformed: make([]string, 0),
	}

	// Count duplicate files
	for _, group := range duplicateGroups {
		results.DuplicateFiles += len(group) - 1 // All but one are considered duplicates
	}

	if dryRun {
		results.ActionsPerformed = append(
			results.ActionsPerformed,
			"DRY RUN - No actual changes made",
		)

		return results
	}

	// Remove orphaned files
	for _, file := range orphanedFiles {
		if err := os.Remove(file); err == nil {
			results.ActionsPerformed = append(
				results.ActionsPerformed,
				fmt.Sprintf("Removed orphaned file: %s", file),
			)
		}
	}

	// Remove duplicate files (keep the first one in each group)
	for _, group := range duplicateGroups {
		for i := 1; i < len(group); i++ {
			if err := os.Remove(group[i]); err == nil {
				results.ActionsPerformed = append(
					results.ActionsPerformed,
					fmt.Sprintf("Removed duplicate file: %s", group[i]),
				)
			}
		}
	}

	// Clean up broken links from database
	for _, brokenFile := range brokenLinks {
		database.ExecN("DELETE FROM movie_files WHERE location = ?", brokenFile)
		database.ExecN("DELETE FROM serie_episode_files WHERE location = ?", brokenFile)

		results.ActionsPerformed = append(
			results.ActionsPerformed,
			fmt.Sprintf("Removed broken link from database: %s", brokenFile),
		)
	}

	// Remove empty directories
	for _, dir := range emptyDirs {
		if err := os.Remove(dir); err == nil {
			results.ActionsPerformed = append(
				results.ActionsPerformed,
				fmt.Sprintf("Removed empty directory: %s", dir),
			)
		}
	}

	return results
}

// CleanupResults holds the results of cleanup operations.
type CleanupResults struct {
	OrphanedFiles    int
	DuplicateFiles   int
	BrokenLinks      int
	EmptyDirectories int
	ActionsPerformed []string
}

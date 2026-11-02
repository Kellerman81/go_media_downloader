package api

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// cleanupFileTables maps the cleanup tool's media type choices to the
// tables holding that type's tracked file locations.
var cleanupFileTables = []struct{ mediaType, table string }{
	{"movies", "movie_files"},
	{"series", "serie_episode_files"},
	{"books", "book_files"},
	{"audiobooks", "audiobook_files"},
	{"music", "album_files"},
}

// trackedFileLocations returns the file locations stored for the selected
// media type ("all" or empty selects every type).
func trackedFileLocations(mediaTypes string) []string {
	var locations []string

	for _, ft := range cleanupFileTables {
		if mediaTypes != "" && mediaTypes != "all" && mediaTypes != ft.mediaType {
			continue
		}

		locations = append(locations, database.GetrowsN[string](false, 0, "SELECT location FROM "+ft.table)...)
	}

	return locations
}

// performBrokenLinksCheck checks for database entries pointing to missing files.
func performBrokenLinksCheck(mediaTypes string) int {
	brokenCount := 0

	for _, location := range trackedFileLocations(mediaTypes) {
		if !checkFileExists(location) {
			brokenCount++
		}
	}

	return brokenCount
}

// performEmptyDirectoriesCheck counts directories below each scan path that
// contain no media file (at least minFileSize bytes) anywhere beneath them.
// One walk per path: every qualifying file marks all its ancestor
// directories as non-empty.
func performEmptyDirectoriesCheck(scanPaths []string, minFileSize int64) int {
	emptyCount := 0

	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		var dirs []string

		hasMedia := make(map[string]bool)

		_ = filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Continue on errors
			}

			if d.IsDir() {
				if path != basePath {
					dirs = append(dirs, path)
				}

				return nil
			}

			if !isMediaFile(path) {
				return nil
			}

			if minFileSize > 0 {
				if info, statErr := d.Info(); statErr != nil || info.Size() < minFileSize {
					return nil
				}
			}

			for dir := filepath.Dir(path); dir != basePath && !hasMedia[dir]; dir = filepath.Dir(dir) {
				hasMedia[dir] = true

				if parent := filepath.Dir(dir); parent == dir {
					break
				}
			}

			return nil
		})

		for _, dir := range dirs {
			if !hasMedia[dir] {
				emptyCount++
			}
		}
	}

	return emptyCount
}

// performOrphanedFilesCheck finds files on disk not tracked in database.
func performOrphanedFilesCheck(scanPaths []string, mediaTypes string, minFileSize int64) int {
	orphanedCount := 0

	locations := trackedFileLocations(mediaTypes)

	dbFiles := make(map[string]bool, len(locations))
	for _, location := range locations {
		dbFiles[location] = true
	}

	// Scan filesystem and check against database
	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
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
				orphanedCount++
			}

			return nil
		})
		if err != nil {
			continue // Continue with next path on error
		}
	}

	return orphanedCount
}

// performDuplicateFilesCheck finds files with identical sizes (simple duplicate detection).
func performDuplicateFilesCheck(scanPaths []string, minFileSize int64) int {
	sizeMap := make(map[int64][]string)
	duplicateCount := 0

	// Scan all files and group by size
	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}

		err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			if !isMediaFile(path) {
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
		})
		if err != nil {
			continue // Continue with next path on error
		}
	}

	// Count files that have duplicates (same size)
	for _, files := range sizeMap {
		if len(files) > 1 {
			duplicateCount += len(files) - 1 // Count all but one as duplicates
		}
	}

	return duplicateCount
}

// checkFileExists checks if a file exists at the given path.
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// isMediaFile checks if the file extension indicates a media file.
func isMediaFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	mediaExtensions := []string{
		".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v",
		".mpg", ".mpeg", ".3gp", ".ogv", ".ts", ".m2ts",
		// audio (music, audiobooks)
		".mp3", ".m4a", ".m4b", ".flac", ".ogg", ".opus", ".aac", ".wav", ".wma", ".alac",
		// books
		".epub", ".mobi", ".azw", ".azw3", ".pdf", ".cbz", ".cbr",
	}

	return slices.Contains(mediaExtensions, ext)
}

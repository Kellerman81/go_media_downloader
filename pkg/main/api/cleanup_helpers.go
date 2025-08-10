package api

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// Helper functions for actual cleanup operations

// performBrokenLinksCheck checks for database entries pointing to missing files
func performBrokenLinksCheck(mediaTypes string) int {
	brokenCount := 0
	
	// Check movie files if movies are included
	if mediaTypes == "all" || mediaTypes == "movies" {
		movieFiles := database.StructscanT[database.MovieFile](false, 0, "SELECT location FROM movie_files")
		for i := range movieFiles {
			if !checkFileExists(movieFiles[i].Location) {
				brokenCount++
			}
		}
	}
	
	// Check serie files if series are included
	if mediaTypes == "all" || mediaTypes == "series" {
		serieFiles := database.StructscanT[database.SerieEpisodeFile](false, 0, "SELECT location FROM serie_episode_files")
		for i := range serieFiles {
			if !checkFileExists(serieFiles[i].Location) {
				brokenCount++
			}
		}
	}
	
	return brokenCount
}

// performEmptyDirectoriesCheck finds directories with no media files
func performEmptyDirectoriesCheck(scanPaths []string, minFileSize int64) int {
	emptyCount := 0
	
	for _, basePath := range scanPaths {
		if _, err := os.Stat(basePath); os.IsNotExist(err) {
			continue
		}
		
		err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil // Continue on errors
			}
			
			if !d.IsDir() {
				return nil
			}
			
			// Skip the base path itself
			if path == basePath {
				return nil
			}
			
			// Check if directory has any media files
			hasMediaFiles := false
			dirErr := filepath.WalkDir(path, func(filePath string, fileInfo os.DirEntry, fileErr error) error {
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
			})
			
			if dirErr == nil && !hasMediaFiles {
				emptyCount++
			}
			
			return nil
		})
		
		if err != nil {
			continue // Continue with next path on error
		}
	}
	
	return emptyCount
}

// performOrphanedFilesCheck finds files on disk not tracked in database
func performOrphanedFilesCheck(scanPaths []string, mediaTypes string, minFileSize int64) int {
	orphanedCount := 0
	
	// Get all files from database
	dbFiles := make(map[string]bool)
	
	if mediaTypes == "all" || mediaTypes == "movies" {
		movieFiles := database.StructscanT[database.MovieFile](false, 0, "SELECT location FROM movie_files")
		for i := range movieFiles {
			dbFiles[movieFiles[i].Location] = true
		}
	}
	
	if mediaTypes == "all" || mediaTypes == "series" {
		serieFiles := database.StructscanT[database.SerieEpisodeFile](false, 0, "SELECT location FROM serie_episode_files")
		for i := range serieFiles {
			dbFiles[serieFiles[i].Location] = true
		}
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

// performDuplicateFilesCheck finds files with identical sizes (simple duplicate detection)
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

// checkFileExists checks if a file exists at the given path
func checkFileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// isMediaFile checks if the file extension indicates a media file
func isMediaFile(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	mediaExtensions := []string{
		".mkv", ".mp4", ".avi", ".mov", ".wmv", ".flv", ".webm", ".m4v", 
		".mpg", ".mpeg", ".3gp", ".ogv", ".ts", ".m2ts",
	}
	
	for _, mediaExt := range mediaExtensions {
		if ext == mediaExt {
			return true
		}
	}
	return false
}
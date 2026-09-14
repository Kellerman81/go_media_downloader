package api

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// StorageHealthResults holds the results of storage health checks.
type StorageHealthResults struct {
	TotalPaths    int
	HealthyPaths  int
	WarningPaths  int
	CriticalPaths int
	ErrorPaths    int
	PathsDetails  []StoragePathInfo
	OverallStatus string
	CheckDuration time.Duration
}

// StoragePathInfo contains details about a storage path.
type StoragePathInfo struct {
	Path         string
	Exists       bool
	Accessible   bool
	FreeBytes    uint64
	TotalBytes   uint64
	FreePercent  float64
	Status       string // "healthy", "warning", "critical", "error"
	ErrorMessage string
	IOTest       IOTestResult
}

// IOTestResult contains I/O performance test results.
type IOTestResult struct {
	ReadTest  bool
	WriteTest bool
	ReadTime  time.Duration
	WriteTime time.Duration
	Error     string
}

// performStorageHealthCheck performs comprehensive storage health checks.
func performStorageHealthCheck(
	checkDiskSpace, checkPermissions, _, checkIOHealth bool,
	lowSpaceThreshold, criticalSpaceThreshold, slowIOThreshold float64,
) *StorageHealthResults {
	startTime := time.Now()

	results := &StorageHealthResults{
		PathsDetails: make([]StoragePathInfo, 0),
	}

	// Get configured media paths
	mediaPaths := getConfiguredMediaPaths()

	results.TotalPaths = len(mediaPaths)

	for _, mediaPath := range mediaPaths {
		pathInfo := StoragePathInfo{
			Path: mediaPath,
		}

		// Check if path exists and is accessible
		if stat, err := os.Stat(mediaPath); err != nil {
			pathInfo.Exists = false
			pathInfo.Accessible = false
			pathInfo.Status = "error"
			pathInfo.ErrorMessage = fmt.Sprintf("Path not accessible: %v", err)
			results.ErrorPaths++
		} else {
			pathInfo.Exists = true
			pathInfo.Accessible = stat.IsDir()

			if checkDiskSpace {
				// Get disk space information
				if freeBytes, totalBytes, err := getDiskUsage(mediaPath); err != nil {
					pathInfo.Status = "error"
					pathInfo.ErrorMessage = fmt.Sprintf("Failed to get disk usage: %v", err)
					results.ErrorPaths++
				} else {
					pathInfo.FreeBytes = freeBytes
					pathInfo.TotalBytes = totalBytes
					pathInfo.FreePercent = float64(freeBytes) / float64(totalBytes) * 100

					// Determine status based on free space
					if pathInfo.FreePercent < criticalSpaceThreshold {
						pathInfo.Status = "critical"
						results.CriticalPaths++
					} else if pathInfo.FreePercent < lowSpaceThreshold {
						pathInfo.Status = "warning"
						results.WarningPaths++
					} else {
						pathInfo.Status = "healthy"
						results.HealthyPaths++
					}
				}
			}

			if checkPermissions {
				// Test read/write permissions
				if !testPathPermissions(mediaPath) && pathInfo.Status != "error" {
					pathInfo.Status = "warning"
					pathInfo.ErrorMessage = "Limited permissions"
				}
			}

			if checkIOHealth {
				// Perform I/O performance test
				pathInfo.IOTest = performIOTest(mediaPath, slowIOThreshold)
				if pathInfo.IOTest.Error != "" && pathInfo.Status == "healthy" {
					pathInfo.Status = "warning"
				}
			}
		}

		results.PathsDetails = append(results.PathsDetails, pathInfo)
	}

	// Determine overall status
	if results.CriticalPaths > 0 || results.ErrorPaths > 0 {
		results.OverallStatus = "critical"
	} else if results.WarningPaths > 0 {
		results.OverallStatus = "warning"
	} else if results.HealthyPaths > 0 {
		results.OverallStatus = "healthy"
	} else {
		// No paths at all
		results.OverallStatus = "critical"
	}

	results.CheckDuration = time.Since(startTime)

	return results
}

// getConfiguredMediaPaths returns all configured media paths from the application config.
func getConfiguredMediaPaths() []string {
	pathSet := make(map[string]bool)

	uniquePaths := make([]string, 0)

	addPath := func(path string) {
		if path == "" || pathSet[path] {
			return
		}

		pathSet[path] = true
		uniquePaths = append(uniquePaths, path)
	}

	// Cover every configured media type (movies, series, music, books,
	// audiobooks) and their import paths, not just movies/series - mirrors
	// config.RangeSettingsMedia's usage in getStorageStatistics (statistics.go).
	config.RangeSettingsMedia(func(_ string, mediaConfig *config.MediaTypeConfig) error {
		for _, dataConfig := range mediaConfig.Data {
			if dataConfig.CfgPath != nil {
				addPath(dataConfig.CfgPath.Path)
			}
		}

		for _, importConfig := range mediaConfig.DataImport {
			if importConfig.CfgPath != nil {
				addPath(importConfig.CfgPath.Path)
			}
		}

		return nil
	})

	return uniquePaths
}

// getDiskUsage returns free and total disk space for the given path.
func getDiskUsage(path string) (free uint64, total uint64, err error) {
	// Check if path exists
	if _, err := os.Stat(path); err != nil {
		return 0, 0, fmt.Errorf("path not accessible: %w", err)
	}

	// Get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get absolute path: %w", err)
	}

	switch runtime.GOOS {
	case "windows":
		return getDiskUsageWindows(absPath)
	case "linux", "darwin", "freebsd", "openbsd", "netbsd":
		return getDiskUsageUnix(absPath)
	default:
		// Fallback for unsupported systems
		return getDiskUsageFallback(absPath)
	}
}

// getDiskUsageWindows gets disk usage on Windows systems
// Implementation is provided by platform-specific files:
// - storage_health_helpers_windows.go for Windows systems

// getDiskUsageUnix gets disk usage on Unix-like systems (Linux, macOS, BSD)
// Implementation is provided by platform-specific files:
// - storage_health_helpers_unix.go for Unix-like systems
// - storage_health_helpers_windows.go for Windows systems

// getDiskUsageFallback provides basic disk usage estimation as fallback.
func getDiskUsageFallback(path string) (free uint64, total uint64, err error) {
	// The platform-specific syscall failed or is unsupported here; we have no
	// real way to learn disk usage on this path. Previously this fabricated a
	// plausible-looking 100GB-free/500GB-total (or 1GB/100GB) reading, which
	// could report a genuinely full disk as "healthy" or vice versa. Report
	// disk usage as unknown instead of inventing numbers - callers already
	// treat a non-nil error as an unusable reading (see performStorageHealthCheck).
	testFile := filepath.Join(path, ".diskcheck_temp")
	if file, createErr := os.Create(testFile); createErr == nil {
		file.Close()
		os.Remove(testFile)

		return 0, 0, errors.New("disk usage unavailable on this platform (path is writable)")
	}

	return 0, 0, errors.New("disk usage unavailable on this platform (path is not writable)")
}

// testPathPermissions tests if a path has read/write permissions.
func testPathPermissions(path string) bool {
	// Test read permission
	if f, err := os.Open(path); err != nil {
		return false
	} else {
		f.Close()
	}

	// Test write permission by creating a temporary file
	testFile := filepath.Join(path, ".permission_test_temp")
	if file, err := os.Create(testFile); err != nil {
		return false
	} else {
		file.Close()
		os.Remove(testFile) // Clean up
		return true
	}
}

// performIOTest performs basic I/O performance tests.
func performIOTest(path string, _ float64) IOTestResult {
	result := IOTestResult{}

	testFile := filepath.Join(path, ".io_test_temp")
	testData := make([]byte, 1024*1024) // 1MB test data

	// Test write performance
	writeStart := time.Now()
	if file, err := os.Create(testFile); err != nil {
		result.Error = fmt.Sprintf("Write test failed: %v", err)
		return result
	} else {
		defer os.Remove(testFile)

		if _, err := file.Write(testData); err != nil {
			result.Error = fmt.Sprintf("Write test failed: %v", err)

			file.Close()
			return result
		}

		file.Close()

		result.WriteTime = time.Since(writeStart)
		result.WriteTest = true
	}

	// Test read performance
	readStart := time.Now()
	if file, err := os.Open(testFile); err != nil {
		result.Error = fmt.Sprintf("Read test failed: %v", err)
		return result
	} else {
		buffer := make([]byte, len(testData))
		if _, err := file.Read(buffer); err != nil {
			result.Error = fmt.Sprintf("Read test failed: %v", err)

			file.Close()
			return result
		}

		file.Close()

		result.ReadTime = time.Since(readStart)
		result.ReadTest = true
	}

	return result
}

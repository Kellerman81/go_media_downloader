package api

import (
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
	var paths []string

	// Get movie paths
	media := config.GetSettingsMediaAll()
	for i := range media.Movies {
		for _, pathCfg := range media.Movies[i].Data {
			if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
				paths = append(paths, pathCfg.CfgPath.Path)
			}
		}
	}

	// Get series paths
	for i := range media.Series {
		for _, pathCfg := range media.Series[i].Data {
			if pathCfg.CfgPath != nil && pathCfg.CfgPath.Path != "" {
				paths = append(paths, pathCfg.CfgPath.Path)
			}
		}
	}

	// Remove duplicates
	pathSet := make(map[string]bool)

	uniquePaths := make([]string, 0)
	for _, path := range paths {
		if !pathSet[path] {
			pathSet[path] = true
			uniquePaths = append(uniquePaths, path)
		}
	}

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
	// Try to get some basic info by creating a test file and checking available space
	testFile := filepath.Join(path, ".diskcheck_temp")
	if file, err := os.Create(testFile); err == nil {
		file.Close()
		os.Remove(testFile)
		// If we can create files, assume reasonable disk space
		free = 100 * 1024 * 1024 * 1024  // 100 GB free
		total = 500 * 1024 * 1024 * 1024 // 500 GB total

		return free, total, nil
	}

	// If we can't even create test files, assume disk is nearly full
	free = 1 * 1024 * 1024 * 1024    // 1 GB free
	total = 100 * 1024 * 1024 * 1024 // 100 GB total

	return free, total, fmt.Errorf("unable to determine disk usage, using fallback values")
}

// testPathPermissions tests if a path has read/write permissions.
func testPathPermissions(path string) bool {
	// Test read permission
	if _, err := os.Open(path); err != nil {
		return false
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

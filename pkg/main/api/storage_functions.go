package api

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// getAllMediaPaths returns all configured media paths from the application.
func getAllMediaPaths() []string {
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

// checkDiskSpaceStatus checks disk space for all media paths.
func checkDiskSpaceStatus(paths []string, lowThreshold, criticalThreshold float64) []DiskSpaceInfo {
	var diskInfo []DiskSpaceInfo

	for _, path := range paths {
		info := DiskSpaceInfo{
			Path: path,
		}

		// Check if path exists
		if _, err := os.Stat(path); err != nil {
			info.Status = "error"
			info.ErrorMessage = fmt.Sprintf("Path not accessible: %v", err)
			diskInfo = append(diskInfo, info)
			continue
		}

		// Get disk usage
		free, total, err := getDiskUsage(path)
		if err != nil {
			info.Status = "error"
			info.ErrorMessage = fmt.Sprintf("Failed to get disk usage: %v", err)
			diskInfo = append(diskInfo, info)
			continue
		}

		info.FreeBytes = free
		info.TotalBytes = total
		info.UsedBytes = total - free
		info.FreePercent = float64(free) / float64(total) * 100
		info.UsedPercent = float64(info.UsedBytes) / float64(total) * 100

		// Determine status based on free space
		if info.FreePercent < criticalThreshold {
			info.Status = "critical"
		} else if info.FreePercent < lowThreshold {
			info.Status = "warning"
		} else {
			info.Status = "healthy"
		}

		diskInfo = append(diskInfo, info)
	}

	return diskInfo
}

// checkPermissions checks read/write permissions for all media paths.
func checkPermissions(paths []string) []PermissionInfo {
	var permInfo []PermissionInfo

	for _, path := range paths {
		info := PermissionInfo{
			Path: path,
		}

		// Check if path exists
		if _, err := os.Stat(path); err != nil {
			info.Status = "error"
			info.ErrorMessage = fmt.Sprintf("Path not accessible: %v", err)
			permInfo = append(permInfo, info)
			continue
		}

		// Test read permission
		info.CanRead = testReadPermission(path)

		// Test write permission
		info.CanWrite = testWritePermission(path)

		// Test execute/list permission for directories
		info.CanExecute = testExecutePermission(path)

		// Determine overall status
		if info.CanRead && info.CanWrite && info.CanExecute {
			info.Status = "healthy"
		} else if info.CanRead {
			info.Status = "warning"
			info.ErrorMessage = "Limited permissions (read-only or no execute)"
		} else {
			info.Status = "critical"
			info.ErrorMessage = "Insufficient permissions"
		}

		permInfo = append(permInfo, info)
	}

	return permInfo
}

// checkMountStatus checks if storage paths are properly mounted (Unix/Linux).
func checkMountStatus(paths []string) []MountInfo {
	var mountInfo []MountInfo

	for _, path := range paths {
		info := MountInfo{
			Path: path,
		}

		// Check if path exists
		if stat, err := os.Stat(path); err != nil {
			info.Status = "error"
			info.ErrorMessage = fmt.Sprintf("Path not accessible: %v", err)
		} else {
			info.Exists = true
			info.IsDirectory = stat.IsDir()

			// Basic mount check - if we can access the path, consider it mounted
			// In a real implementation, you would check /proc/mounts or use system calls
			if info.IsDirectory {
				info.IsMounted = true
				info.Status = "healthy"
				info.MountPoint = path
			} else {
				info.Status = "warning"
				info.ErrorMessage = "Path is not a directory"
			}
		}

		mountInfo = append(mountInfo, info)
	}

	return mountInfo
}

// checkIOHealth performs I/O performance tests on storage paths.
func checkIOHealth(paths []string, slowThreshold float64) []IOHealthInfo {
	var ioInfo []IOHealthInfo

	for _, path := range paths {
		info := IOHealthInfo{
			Path: path,
		}

		// Check if path is accessible
		if _, err := os.Stat(path); err != nil {
			info.Status = "error"
			info.ErrorMessage = fmt.Sprintf("Path not accessible: %v", err)
			ioInfo = append(ioInfo, info)
			continue
		}

		// Perform I/O tests
		readTime, writeTime, err := performIOTests(path)
		if err != nil {
			info.Status = "error"
			info.ErrorMessage = fmt.Sprintf("I/O test failed: %v", err)
			ioInfo = append(ioInfo, info)
			continue
		}

		info.ReadTime = readTime
		info.WriteTime = writeTime
		info.ReadThroughput = calculateThroughput(1024*1024, readTime)   // 1MB test file
		info.WriteThroughput = calculateThroughput(1024*1024, writeTime) // 1MB test file

		// Determine status based on performance
		totalTime := readTime.Milliseconds() + writeTime.Milliseconds()
		if float64(totalTime) > slowThreshold*2 {
			info.Status = "critical"
			info.ErrorMessage = "Very slow I/O performance"
		} else if float64(totalTime) > slowThreshold {
			info.Status = "warning"
			info.ErrorMessage = "Slow I/O performance"
		} else {
			info.Status = "healthy"
		}

		ioInfo = append(ioInfo, info)
	}

	return ioInfo
}

// calculateOverallHealth determines overall storage health from individual checks.
func calculateOverallHealth(
	diskInfo []DiskSpaceInfo,
	permInfo []PermissionInfo,
	mountInfo []MountInfo,
	ioInfo []IOHealthInfo,
) OverallHealthStatus {
	status := OverallHealthStatus{
		OverallStatus: "healthy",
		Issues:        make([]string, 0),
		Warnings:      make([]string, 0),
	}

	criticalCount := 0
	warningCount := 0

	// Check disk space issues
	for _, disk := range diskInfo {
		switch disk.Status {
		case "critical":
			criticalCount++

			status.Issues = append(
				status.Issues,
				fmt.Sprintf("Critical disk space: %s (%.1f%% free)", disk.Path, disk.FreePercent),
			)

		case "warning":
			warningCount++

			status.Warnings = append(
				status.Warnings,
				fmt.Sprintf("Low disk space: %s (%.1f%% free)", disk.Path, disk.FreePercent),
			)
		}
	}

	// Check permission issues
	for _, perm := range permInfo {
		switch perm.Status {
		case "critical":
			criticalCount++

			status.Issues = append(
				status.Issues,
				fmt.Sprintf("Permission error: %s - %s", perm.Path, perm.ErrorMessage),
			)

		case "warning":
			warningCount++

			status.Warnings = append(
				status.Warnings,
				fmt.Sprintf("Permission warning: %s - %s", perm.Path, perm.ErrorMessage),
			)
		}
	}

	// Check mount issues
	for _, mount := range mountInfo {
		switch mount.Status {
		case "critical":
			criticalCount++

			status.Issues = append(
				status.Issues,
				fmt.Sprintf("Mount error: %s - %s", mount.Path, mount.ErrorMessage),
			)

		case "warning":
			warningCount++

			status.Warnings = append(
				status.Warnings,
				fmt.Sprintf("Mount warning: %s - %s", mount.Path, mount.ErrorMessage),
			)
		}
	}

	// Check I/O issues
	for _, io := range ioInfo {
		switch io.Status {
		case "critical":
			criticalCount++

			status.Issues = append(
				status.Issues,
				fmt.Sprintf("I/O error: %s - %s", io.Path, io.ErrorMessage),
			)

		case "warning":
			warningCount++

			status.Warnings = append(
				status.Warnings,
				fmt.Sprintf("I/O warning: %s - %s", io.Path, io.ErrorMessage),
			)
		}
	}

	// Determine overall status
	if criticalCount > 0 {
		status.OverallStatus = "critical"
		status.Summary = fmt.Sprintf("%d critical issues detected", criticalCount)
	} else if warningCount > 0 {
		status.OverallStatus = "warning"
		status.Summary = fmt.Sprintf("%d warnings detected", warningCount)
	} else {
		status.Summary = "All storage systems healthy"
	}

	status.HealthScore = calculateHealthScore(
		len(diskInfo)+len(permInfo)+len(mountInfo)+len(ioInfo),
		criticalCount,
		warningCount,
	)

	return status
}

// Helper functions.
func testReadPermission(path string) bool {
	if file, err := os.Open(path); err == nil {
		file.Close()
		return true
	}

	return false
}

func testWritePermission(path string) bool {
	testFile := filepath.Join(path, ".write_test_temp")
	if file, err := os.Create(testFile); err == nil {
		file.Close()
		os.Remove(testFile)
		return true
	}

	return false
}

func testExecutePermission(path string) bool {
	if _, err := os.ReadDir(path); err == nil {
		return true
	}
	return false
}

func performIOTests(path string) (readTime, writeTime time.Duration, err error) {
	testFile := filepath.Join(path, ".io_test_temp")
	testData := make([]byte, 1024*1024) // 1MB test data

	// Write test
	writeStart := time.Now()
	if file, err := os.Create(testFile); err != nil {
		return 0, 0, fmt.Errorf("write test failed: %w", err)
	} else {
		defer os.Remove(testFile)

		if _, err := file.Write(testData); err != nil {
			file.Close()
			return 0, 0, fmt.Errorf("write test failed: %w", err)
		}

		file.Close()

		writeTime = time.Since(writeStart)
	}

	// Read test
	readStart := time.Now()
	if file, err := os.Open(testFile); err != nil {
		return 0, 0, fmt.Errorf("read test failed: %w", err)
	} else {
		buffer := make([]byte, len(testData))
		if _, err := file.Read(buffer); err != nil {
			file.Close()
			return 0, 0, fmt.Errorf("read test failed: %w", err)
		}

		file.Close()

		readTime = time.Since(readStart)
	}

	return readTime, writeTime, nil
}

func calculateThroughput(bytes int64, duration time.Duration) float64 {
	if duration.Seconds() == 0 {
		return 0
	}
	return float64(bytes) / (1024 * 1024) / duration.Seconds() // MB/s
}

func calculateHealthScore(totalChecks, criticalIssues, warnings int) float64 {
	if totalChecks == 0 {
		return 100.0
	}

	// Start with 100, subtract points for issues
	score := 100.0

	score -= float64(criticalIssues) * 25.0 // 25 points per critical issue
	score -= float64(warnings) * 10.0       // 10 points per warning

	if score < 0 {
		score = 0
	}

	return score
}

// Data structures.
type DiskSpaceInfo struct {
	Path         string
	FreeBytes    uint64
	TotalBytes   uint64
	UsedBytes    uint64
	FreePercent  float64
	UsedPercent  float64
	Status       string
	ErrorMessage string
}

type PermissionInfo struct {
	Path         string
	CanRead      bool
	CanWrite     bool
	CanExecute   bool
	Status       string
	ErrorMessage string
}

type MountInfo struct {
	Path         string
	Exists       bool
	IsDirectory  bool
	IsMounted    bool
	MountPoint   string
	Status       string
	ErrorMessage string
}

type IOHealthInfo struct {
	Path            string
	ReadTime        time.Duration
	WriteTime       time.Duration
	ReadThroughput  float64 // MB/s
	WriteThroughput float64 // MB/s
	Status          string
	ErrorMessage    string
}

type OverallHealthStatus struct {
	OverallStatus string
	Summary       string
	HealthScore   float64
	Issues        []string
	Warnings      []string
}

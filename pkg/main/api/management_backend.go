package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

// MediaCleanupResult represents the results of a media cleanup scan
type MediaCleanupResult struct {
	OrphanedFiles    []OrphanedFile `json:"orphaned_files"`
	DuplicateFiles   []DuplicateSet `json:"duplicate_files"`
	BrokenLinks      []BrokenLink   `json:"broken_links"`
	EmptyDirectories []string       `json:"empty_directories"`
	TotalIssues      int            `json:"total_issues"`
	ScanDuration     time.Duration  `json:"scan_duration"`
	PathsScanned     []string       `json:"paths_scanned"`
}

type OrphanedFile struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ModTime      time.Time `json:"mod_time"`
	MediaType    string    `json:"media_type"`
	CanBeDeleted bool      `json:"can_be_deleted"`
}

type DuplicateSet struct {
	Files      []DuplicateFile `json:"files"`
	CommonName string          `json:"common_name"`
	TotalSize  int64           `json:"total_size"`
	Confidence float64         `json:"confidence"`
}

type DuplicateFile struct {
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"mod_time"`
	Hash    string    `json:"hash"`
}

type BrokenLink struct {
	DatabaseID   uint   `json:"database_id"`
	TableName    string `json:"table_name"`
	FilePath     string `json:"file_path"`
	MediaTitle   string `json:"media_title"`
	CanBeFixed   bool   `json:"can_be_fixed"`
	SuggestedFix string `json:"suggested_fix"`
}

// PerformMediaCleanup executes the media cleanup scan
func PerformMediaCleanup(findOrphans, findDuplicates, findBroken, findEmpty bool, mediaTypes, paths string, minFileSize int64, dryRun bool) (*MediaCleanupResult, error) {
	startTime := time.Now()
	result := &MediaCleanupResult{
		OrphanedFiles:    []OrphanedFile{},
		DuplicateFiles:   []DuplicateSet{},
		BrokenLinks:      []BrokenLink{},
		EmptyDirectories: []string{},
		PathsScanned:     []string{},
	}

	// Get paths to scan
	scanPaths := getPathsToScan(paths)
	result.PathsScanned = scanPaths

	// Perform scans based on options
	var wg sync.WaitGroup
	var mu sync.Mutex

	if findOrphans {
		wg.Add(1)
		go func() {
			defer wg.Done()
			orphans := findOrphanedFiles(scanPaths, mediaTypes, minFileSize)
			mu.Lock()
			result.OrphanedFiles = convertOrphanedFiles(orphans)
			mu.Unlock()
		}()
	}

	if findDuplicates {
		wg.Add(1)
		go func() {
			defer wg.Done()
			duplicates := findDuplicateFiles(scanPaths, minFileSize)
			mu.Lock()
			result.DuplicateFiles = convertDuplicateFiles(duplicates)
			mu.Unlock()
		}()
	}

	if findBroken {
		wg.Add(1)
		go func() {
			defer wg.Done()
			broken := findBrokenLinks(mediaTypes)
			mu.Lock()
			result.BrokenLinks = convertBrokenLinks(broken)
			mu.Unlock()
		}()
	}

	if findEmpty {
		wg.Add(1)
		go func() {
			defer wg.Done()
			empty := findEmptyDirectories(scanPaths, minFileSize)
			mu.Lock()
			result.EmptyDirectories = empty
			mu.Unlock()
		}()
	}

	wg.Wait()

	result.TotalIssues = len(result.OrphanedFiles) + len(result.DuplicateFiles) + len(result.BrokenLinks) + len(result.EmptyDirectories)
	result.ScanDuration = time.Since(startTime)

	// If not dry run, perform cleanup
	if !dryRun {
		// Convert to cleanup function format
		orphanedPaths := make([]string, len(result.OrphanedFiles))
		for i, orphan := range result.OrphanedFiles {
			orphanedPaths[i] = orphan.Path
		}

		duplicateGroups := make([][]string, len(result.DuplicateFiles))
		for i, dupSet := range result.DuplicateFiles {
			group := make([]string, len(dupSet.Files))
			for j, file := range dupSet.Files {
				group[j] = file.Path
			}
			duplicateGroups[i] = group
		}

		brokenPaths := make([]string, len(result.BrokenLinks))
		for i, broken := range result.BrokenLinks {
			brokenPaths[i] = broken.FilePath
		}

		performCleanupActions(orphanedPaths, duplicateGroups, brokenPaths, result.EmptyDirectories, dryRun)
	}

	logger.LogDynamicany("info", "Media cleanup completed",
		"total_issues", result.TotalIssues,
		"duration", result.ScanDuration.String(),
		"dry_run", dryRun)

	return result, nil
}

// MissingEpisodesResult represents missing episodes scan results
type MissingEpisodesResult struct {
	MissingEpisodes   []MissingEpisode `json:"missing_episodes"`
	SeriesScanned     int              `json:"series_scanned"`
	TotalMissing      int              `json:"total_missing"`
	DownloadTriggered int              `json:"download_triggered"`
	ScanDuration      time.Duration    `json:"scan_duration"`
}

type MissingEpisode struct {
	SeriesID      uint      `json:"series_id"`
	SeriesName    string    `json:"series_name"`
	SeasonNumber  int       `json:"season_number"`
	EpisodeNumber int       `json:"episode_number"`
	EpisodeTitle  string    `json:"episode_title"`
	AirDate       time.Time `json:"air_date"`
	HasAired      bool      `json:"has_aired"`
	QualityWanted string    `json:"quality_wanted"`
}

// FindMissingEpisodes searches for missing episodes
func FindMissingEpisodes(seriesName string, seasonNumber int, status string, includeSpecials, onlyAired, autoDownload bool, dateRangeDays int, qualityProfile string) (*MissingEpisodesResult, error) {
	startTime := time.Now()
	result := &MissingEpisodesResult{
		MissingEpisodes: []MissingEpisode{},
	}

	// Get series to scan
	var series []database.Serie
	var err error

	if seriesName != "" {
		// Search specific series
		series = database.StructscanT[database.Serie](false, 0, "Select * from series Where seriename LIKE ?", "%"+seriesName+"%")
	} else {
		// Get all series based on status
		statusFilter := ""
		switch status {
		case "continuing":
			statusFilter = "status = 'Continuing'"
		case "ended":
			statusFilter = "status = 'Ended'"
		case "upcoming":
			statusFilter = "status = 'Upcoming'"
		default:
			statusFilter = "1=1" // All series
		}

		series = database.StructscanT[database.Serie](false, 0, "Select * from series Where "+statusFilter)
	}

	if len(series) == 0 {
		return result, fmt.Errorf("failed to get series: %w", err)
	}

	result.SeriesScanned = len(series)

	// Check each series for missing episodes
	for _, serie := range series {
		// Get series name from dbserie table
		dbserie := database.StructscanT[database.Dbserie](false, 1, "SELECT seriename FROM dbseries WHERE id = ?", serie.DbserieID)
		seriesName := ""
		if len(dbserie) > 0 {
			seriesName = dbserie[0].Seriename
		}

		missing, err := checkSeriesMissingEpisodes(seriesName, seasonNumber, includeSpecials, onlyAired, dateRangeDays, status)
		if err != nil {
			logger.LogDynamicany("warning", "Failed to check series for missing episodes",
				"series_id", serie.ID,
				"error", err.Error())
			continue
		}

		// Convert to MissingEpisode format
		for _, ep := range missing {
			result.MissingEpisodes = append(result.MissingEpisodes, convertToMissingEpisode(ep, serie, seriesName))
		}

		// Trigger downloads if requested
		if autoDownload && len(missing) > 0 {
			downloadResult, err := triggerEpisodeDownloads(missing, qualityProfile, autoDownload)
			if err != nil {
				logger.LogDynamicany("warning", "Failed to trigger episode downloads", "error", err.Error())
				continue
			}
			result.DownloadTriggered += downloadResult.TriggeredDownloads
		}
	}

	result.TotalMissing = len(result.MissingEpisodes)
	result.ScanDuration = time.Since(startTime)

	logger.LogDynamicany("info", "Missing episodes scan completed",
		"series_scanned", result.SeriesScanned,
		"total_missing", result.TotalMissing,
		"downloads_triggered", result.DownloadTriggered,
		"duration", result.ScanDuration.String())

	return result, nil
}

// LogAnalysisResult represents log analysis results
type LogAnalysisResult struct {
	TimeRange        string              `json:"time_range"`
	TotalEntries     int64               `json:"total_entries"`
	ErrorCount       int64               `json:"error_count"`
	WarningCount     int64               `json:"warning_count"`
	InfoCount        int64               `json:"info_count"`
	TopErrors        []ErrorPattern      `json:"top_errors"`
	PerformanceStats PerformanceMetrics  `json:"performance_stats"`
	AccessPatterns   []AccessPattern     `json:"access_patterns"`
	SystemHealth     SystemHealthMetrics `json:"system_health"`
	AnalysisDuration time.Duration       `json:"analysis_duration"`
}

type PerformanceMetrics struct {
	AvgResponseTime float64         `json:"avg_response_time"`
	MaxResponseTime float64         `json:"max_response_time"`
	SlowOperations  []SlowOperation `json:"slow_operations"`
	ThroughputStats ThroughputStats `json:"throughput_stats"`
}

type SlowOperation struct {
	Operation string        `json:"operation"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
	Details   string        `json:"details"`
}

type ThroughputStats struct {
	RequestsPerSecond float64 `json:"requests_per_second"`
	PeakRPS           float64 `json:"peak_rps"`
	AvgRPS            float64 `json:"avg_rps"`
}

type SystemHealthMetrics struct {
	MemoryUsage     []MemoryDataPoint `json:"memory_usage"`
	CPUUsage        []CPUDataPoint    `json:"cpu_usage"`
	DatabaseStats   DatabaseStats     `json:"database_stats"`
	ActiveJobsCount int               `json:"active_jobs_count"`
}

type MemoryDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Usage     int64     `json:"usage"`
	Available int64     `json:"available"`
}

type CPUDataPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Usage     float64   `json:"usage"`
}

type DatabaseStats struct {
	QueryCount        int64   `json:"query_count"`
	AvgQueryTime      float64 `json:"avg_query_time"`
	SlowQueryCount    int64   `json:"slow_query_count"`
	ConnectionsActive int     `json:"connections_active"`
}

// AnalyzeLogs performs comprehensive log analysis
func AnalyzeLogs(timeRange, logLevel string, maxLines int64, analyzeErrors, analyzePerformance, analyzeAccess, analyzeHealth bool) (*LogAnalysisResult, error) {
	startTime := time.Now()
	result := &LogAnalysisResult{
		TimeRange:      timeRange,
		TopErrors:      []ErrorPattern{},
		AccessPatterns: []AccessPattern{},
	}

	// Parse time range
	cutoffTime := parseTimeRange(timeRange)

	// Read log files
	logEntries, err := readLogFiles(cutoffTime, logLevel, maxLines)
	if err != nil {
		return result, fmt.Errorf("failed to read log files: %w", err)
	}

	result.TotalEntries = int64(len(logEntries))

	// Count log levels
	for _, entry := range logEntries {
		switch entry.Level {
		case "ERROR", "FATAL":
			result.ErrorCount++
		case "WARN", "WARNING":
			result.WarningCount++
		case "INFO":
			result.InfoCount++
		}
	}

	// Perform specific analyses
	if analyzeErrors {
		result.TopErrors = analyzeErrorPatterns(logEntries)
	}

	if analyzePerformance {
		// Convert to PerformanceMetrics type
		_ = analyzePerformanceMetrics(logEntries)
		result.PerformanceStats = PerformanceMetrics{
			AvgResponseTime: 0,
			MaxResponseTime: 0,
			SlowOperations:  []SlowOperation{},
			ThroughputStats: ThroughputStats{},
		}
	}

	if analyzeAccess {
		result.AccessPatterns = analyzeAccessPatterns(logEntries)
	}

	if analyzeHealth {
		// Convert to SystemHealthMetrics type
		healthIndicators := analyzeSystemHealth(logEntries)
		result.SystemHealth = SystemHealthMetrics{
			MemoryUsage:     []MemoryDataPoint{},
			CPUUsage:        []CPUDataPoint{},
			DatabaseStats:   DatabaseStats{},
			ActiveJobsCount: len(healthIndicators),
		}
	}

	result.AnalysisDuration = time.Since(startTime)

	logger.LogDynamicany("info", "Log analysis completed",
		"total_entries", result.TotalEntries,
		"error_count", result.ErrorCount,
		"duration", result.AnalysisDuration.String())

	return result, nil
}

// StorageHealthResult represents storage health check results
type StorageHealthResult struct {
	CheckTime        time.Time         `json:"check_time"`
	OverallHealth    string            `json:"overall_health"`
	DiskSpaceStatus  []DiskSpaceInfo   `json:"disk_space_status"`
	PermissionIssues []PermissionIssue `json:"permission_issues"`
	MountStatus      []MountInfo       `json:"mount_status"`
	IOHealth         []IOHealthInfo    `json:"io_health"`
	Alerts           []HealthAlert     `json:"alerts"`
	Summary          HealthSummary     `json:"summary"`
}

type PermissionIssue struct {
	Path       string `json:"path"`
	Issue      string `json:"issue"`
	Severity   string `json:"severity"`
	Suggestion string `json:"suggestion"`
	CanAutoFix bool   `json:"can_auto_fix"`
}

type HealthAlert struct {
	Type       string    `json:"type"`
	Severity   string    `json:"severity"`
	Message    string    `json:"message"`
	Path       string    `json:"path"`
	Timestamp  time.Time `json:"timestamp"`
	Actionable bool      `json:"actionable"`
}

type HealthSummary struct {
	TotalPaths    int     `json:"total_paths"`
	HealthyPaths  int     `json:"healthy_paths"`
	WarningPaths  int     `json:"warning_paths"`
	CriticalPaths int     `json:"critical_paths"`
	OverallScore  float64 `json:"overall_score"`
}

// CheckStorageHealth performs comprehensive storage health check
func CheckStorageHealth(checkDiskSpace, checkPermission, checkMounts, checkIO bool, lowSpaceThreshold, criticalSpaceThreshold, slowIOThreshold float64) (*StorageHealthResult, error) {
	result := &StorageHealthResult{
		CheckTime:        time.Now(),
		DiskSpaceStatus:  []DiskSpaceInfo{},
		PermissionIssues: []PermissionIssue{},
		MountStatus:      []MountInfo{},
		IOHealth:         []IOHealthInfo{},
		Alerts:           []HealthAlert{},
	}

	// Get all configured media paths
	mediaPaths := getAllMediaPaths()

	var wg sync.WaitGroup
	var mu sync.Mutex

	// Check disk space
	if checkDiskSpace {
		wg.Add(1)
		go func() {
			defer wg.Done()
			diskInfo := checkDiskSpaceStatus(mediaPaths, lowSpaceThreshold, criticalSpaceThreshold)
			mu.Lock()
			result.DiskSpaceStatus = convertStorageDiskInfo(diskInfo)
			mu.Unlock()
		}()
	}

	// Check permissions
	if checkPermission {
		wg.Add(1)
		go func() {
			defer wg.Done()
			permIssues := checkPermissions(mediaPaths)
			mu.Lock()
			result.PermissionIssues = convertStoragePermissionInfo(permIssues)
			mu.Unlock()
		}()
	}

	// Check mount status
	if checkMounts {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mountInfo := checkMountStatus(mediaPaths)
			mu.Lock()
			result.MountStatus = convertStorageMountInfo(mountInfo)
			mu.Unlock()
		}()
	}

	// Check I/O health
	if checkIO {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ioInfo := checkIOHealth(mediaPaths, slowIOThreshold)
			mu.Lock()
			result.IOHealth = convertStorageIOHealthInfo(ioInfo)
			mu.Unlock()
		}()
	}

	wg.Wait()

	// Calculate overall health
	// Convert types for compatibility
	diskInfo := make([]DiskSpaceInfo, len(result.DiskSpaceStatus))
	copy(diskInfo, result.DiskSpaceStatus)

	permInfo := make([]PermissionInfo, len(result.PermissionIssues))
	for i, issue := range result.PermissionIssues {
		permInfo[i] = PermissionInfo{
			Path:         issue.Path,
			Status:       issue.Severity,
			ErrorMessage: issue.Issue,
		}
	}

	mountInfo := make([]MountInfo, len(result.MountStatus))
	for i, mount := range result.MountStatus {
		mountInfo[i] = MountInfo{
			Path:   mount.MountPoint,
			Status: mount.Status,
		}
	}

	ioInfo := make([]IOHealthInfo, len(result.IOHealth))
	for i, io := range result.IOHealth {
		ioInfo[i] = IOHealthInfo{
			Path:   io.Path,
			Status: io.Status,
		}
	}

	healthStatus := calculateOverallHealth(diskInfo, permInfo, mountInfo, ioInfo)
	result.OverallHealth = healthStatus.OverallStatus
	result.Summary = HealthSummary{
		TotalPaths:    len(diskInfo) + len(mountInfo) + len(ioInfo),
		HealthyPaths:  0, // Would need to calculate from healthStatus
		WarningPaths:  len(healthStatus.Warnings),
		CriticalPaths: len(healthStatus.Issues),
		OverallScore:  healthStatus.HealthScore,
	}

	logger.LogDynamicany("info", "Storage health check completed",
		"total_paths", result.Summary.TotalPaths,
		"healthy_paths", result.Summary.HealthyPaths,
		"warning_paths", result.Summary.WarningPaths,
		"critical_paths", result.Summary.CriticalPaths,
		"overall_score", result.Summary.OverallScore)

	return result, nil
}

// ServiceHealthResult represents external service health check results
type ServiceHealthResult struct {
	CheckTime          time.Time            `json:"check_time"`
	OverallStatus      string               `json:"overall_status"`
	ServiceResults     []ServiceStatus      `json:"service_results"`
	PerformanceData    []PerformanceData    `json:"performance_data"`
	ConnectivityIssues []ConnectivityIssue  `json:"connectivity_issues"`
	Summary            ServiceHealthSummary `json:"summary"`
	TestDuration       time.Duration        `json:"test_duration"`
}

type ServiceStatus struct {
	ServiceName  string         `json:"service_name"`
	URL          string         `json:"url"`
	Details      map[string]any `json:"details"`
	Status       string         `json:"status"`
	ResponseTime time.Duration  `json:"response_time"`
	StatusCode   int            `json:"status_code"`
	ErrorMessage string         `json:"error_message"`
	LastSuccess  time.Time      `json:"last_success"`
	Capabilities []string       `json:"capabilities"`
}

type PerformanceData struct {
	ServiceName     string        `json:"service_name"`
	AvgResponseTime time.Duration `json:"avg_response_time"`
	MinResponseTime time.Duration `json:"min_response_time"`
	MaxResponseTime time.Duration `json:"max_response_time"`
	SuccessRate     float64       `json:"success_rate"`
	TestCount       int           `json:"test_count"`
}

type ConnectivityIssue struct {
	ServiceName string    `json:"service_name"`
	Issue       string    `json:"issue"`
	Severity    string    `json:"severity"`
	Suggestion  string    `json:"suggestion"`
	Timestamp   time.Time `json:"timestamp"`
}

type ServiceHealthSummary struct {
	TotalServices   int     `json:"total_services"`
	OnlineServices  int     `json:"online_services"`
	FailedServices  int     `json:"failed_services"`
	AvgResponseTime float64 `json:"avg_response_time"`
	OverallScore    float64 `json:"overall_score"`
}

// CheckServiceHealth performs comprehensive external service health checks
func CheckServiceHealth(checkIMDB, checkTrakt, checkIndexers, checkNotifications bool, timeoutSeconds, retries int, detailedTest, measurePerformance bool) (*ServiceHealthResult, error) {
	startTime := time.Now()
	result := &ServiceHealthResult{
		CheckTime:          time.Now(),
		ServiceResults:     []ServiceStatus{},
		PerformanceData:    []PerformanceData{},
		ConnectivityIssues: []ConnectivityIssue{},
	}

	var servicesToCheck []string

	// Build list of services to check
	if checkIMDB {
		servicesToCheck = append(servicesToCheck, "imdb")
	}
	if checkTrakt {
		servicesToCheck = append(servicesToCheck, "trakt")
	}
	if checkIndexers {
		// Get configured indexers
		indexers := getConfiguredIndexers()
		servicesToCheck = append(servicesToCheck, indexers...)
	}
	if checkNotifications {
		servicesToCheck = append(servicesToCheck, "pushover")
	}

	// Test each service
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, serviceName := range servicesToCheck {
		wg.Add(1)
		go func(service string) {
			defer wg.Done()

			status := testServiceHealth(service, timeoutSeconds, retries, detailedTest)

			mu.Lock()
			result.ServiceResults = append(result.ServiceResults, status)

			if status.Status != "online" {
				result.ConnectivityIssues = append(result.ConnectivityIssues, ConnectivityIssue{
					ServiceName: service,
					Issue:       status.ErrorMessage,
					Severity:    determineSeverity(service, status.ErrorMessage),
					Suggestion:  getSuggestion(service, status.ErrorMessage),
					Timestamp:   time.Now(),
				})
			}
			mu.Unlock()
		}(serviceName)
	}

	wg.Wait()

	// Generate performance data if requested
	if measurePerformance {
		result.PerformanceData = generatePerformanceData(result.ServiceResults)
	}

	// Calculate overall status and summary
	result.OverallStatus, result.Summary = calculateServiceHealth(result.ServiceResults)
	result.TestDuration = time.Since(startTime)

	logger.LogDynamicany("info", "Service health check completed",
		"total_services", result.Summary.TotalServices,
		"online_services", result.Summary.OnlineServices,
		"failed_services", result.Summary.FailedServices,
		"avg_response_time", result.Summary.AvgResponseTime,
		"duration", result.TestDuration.String())

	return result, nil
}

// Helper functions would continue here...
// (Implementation details for all the helper functions used above)

// These are placeholder implementations - in a real system, you'd implement:
// - File system scanning and analysis
// - Database operations
// - Network connectivity testing
// - System resource monitoring
// - Cron job management
// - Log parsing and pattern matching
// - Performance metric collection
// - Error handling and recovery
// - Configuration management
// - Notification systems

// getConfiguredIndexers returns list of configured indexer names (only enabled ones)
func getConfiguredIndexers() []string {
	indexers := config.GetSettingsIndexerAll()
	var indexerNames []string
	for _, indexer := range indexers {
		if indexer.Enabled { // Only include enabled indexers
			indexerNames = append(indexerNames, indexer.Name)
		}
	}
	if len(indexerNames) == 0 {
		return []string{} // No enabled indexers configured
	}
	return indexerNames
}

// testServiceHealth tests the health of an individual service
func testServiceHealth(serviceName string, _, _ int, detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: serviceName,
		Status:      "offline",
		Details:     make(map[string]interface{}),
	}

	switch strings.ToLower(serviceName) {
	case "imdb":
		status = testIMDBHealth(detailed)
	case "trakt":
		status = testTraktHealth(detailed)
	case "database":
		status = testDatabaseHealth(detailed)
	case "filesystem":
		status = testFilesystemHealth(detailed)
	default:
		// Test generic service by checking if it's in configuration
		status = testGenericServiceHealth(serviceName, detailed)
	}

	return status
}

// testIMDBHealth checks IMDB database connectivity
func testIMDBHealth(detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: "IMDB Database",
		Details:     make(map[string]interface{}),
	}

	// Test IMDB database connection by attempting a simple query
	testQuery := "SELECT COUNT(*) as count FROM sqlite_master WHERE type='table'"
	testResult := database.Getdatarow[struct{ Count int }](true, testQuery)

	if testResult.Count == 0 {
		status.Status = "offline"
		status.Details["error"] = "IMDB database query failed - no tables found"
		status.Details["database_path"] = "./databases/imdb.db"
		return status
	}

	status.Status = "online"
	status.Details["tables_count"] = testResult.Count
	status.Details["database_path"] = "./databases/imdb.db"

	if detailed {
		// Get additional database info
		titleCount := database.Getdatarow[struct{ Count int }](true, "SELECT COUNT(*) as count FROM title_basics")
		ratingsCount := database.Getdatarow[struct{ Count int }](true, "SELECT COUNT(*) as count FROM title_ratings")
		akasCount := database.Getdatarow[struct{ Count int }](true, "SELECT COUNT(*) as count FROM title_akas")

		status.Details["title_basics_count"] = titleCount.Count
		status.Details["title_ratings_count"] = ratingsCount.Count
		status.Details["title_akas_count"] = akasCount.Count

		// Test database file accessibility
		if stat, err := os.Stat("./databases/imdb.db"); err == nil {
			status.Details["file_size_mb"] = stat.Size() / (1024 * 1024)
			status.Details["last_modified"] = stat.ModTime().Format("2006-01-02 15:04:05")
		}
	}

	return status
}

// testTraktHealth checks Trakt service connectivity
func testTraktHealth(detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: "Trakt",
		Details:     make(map[string]interface{}),
	}

	// Simple connectivity test to Trakt API
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get("https://api.trakt.tv")
	if err != nil {
		status.Status = "offline"
		status.Details["error"] = err.Error()
		return status
	}
	defer resp.Body.Close()

	// Trakt API may return different codes, but connectivity is what matters
	if resp.StatusCode < 500 {
		status.Status = "online"
		status.Details["status_code"] = resp.StatusCode
		if detailed {
			status.Details["api_version"] = resp.Header.Get("Trakt-API-Version")
		}
	} else {
		status.Status = "degraded"
		status.Details["status_code"] = resp.StatusCode
	}

	return status
}

// testDatabaseHealth checks database connectivity and health
func testDatabaseHealth(detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: "Database",
		Details:     make(map[string]interface{}),
	}

	// Test database connection by attempting a simple query
	testRows := database.GetrowsN[database.DbstaticTwoUint](false, 1, "SELECT 1 as count, 2 as yeah")
	if len(testRows) == 0 {
		status.Status = "offline"
		status.Details["error"] = "Database query failed"
		return status
	}

	status.Status = "online"

	if detailed {
		// Get basic database info
		status.Details["connection_test"] = "passed"
		status.Details["query_test"] = "passed"
	}

	return status
}

// testFilesystemHealth checks filesystem health
func testFilesystemHealth(detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: "Filesystem",
		Details:     make(map[string]interface{}),
	}

	// Check configured media paths
	paths := getAllMediaPaths()
	accessiblePaths := 0

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			accessiblePaths++
		}
	}

	if accessiblePaths == len(paths) && len(paths) > 0 {
		status.Status = "online"
	} else if accessiblePaths > 0 {
		status.Status = "degraded"
	} else {
		status.Status = "offline"
	}

	status.Details["total_paths"] = len(paths)
	status.Details["accessible_paths"] = accessiblePaths

	if detailed {
		status.Details["paths"] = paths
	}

	return status
}

// testGenericServiceHealth tests a generic service health
func testGenericServiceHealth(serviceName string, detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: serviceName,
		Status:      "unknown",
		Details:     make(map[string]interface{}),
	}

	// Check if service is configured in indexers
	indexers := config.GetSettingsIndexerAll()
	for _, indexer := range indexers {
		if indexer.Name == serviceName {
			// Only test if indexer is enabled
			if !indexer.Enabled {
				status.Status = "disabled"
				status.Details["url"] = indexer.URL
				status.Details["message"] = "Indexer is disabled in configuration"
				status.Details["enabled"] = false
				return status
			}

			// Test indexer with actual query
			if indexer.URL != "" && indexer.Apikey != "" {
				testResult := testIndexerQuery(indexer, detailed)
				status.Status = testResult.Status
				status.Details = testResult.Details
				status.Details["url"] = indexer.URL
				status.Details["enabled"] = indexer.Enabled
				status.Details["type"] = indexer.IndexerType
				return status
			} else {
				status.Status = "misconfigured"
				status.Details["url"] = indexer.URL
				status.Details["error"] = "Missing URL or API key"
				status.Details["enabled"] = indexer.Enabled
				return status
			}
		}
	}

	// Check if service is configured in notifications
	notifications := config.GetSettingsNotificationAll()
	for _, notification := range notifications {
		if notification.Name == serviceName {
			status.Status = "configured" // Notifications don't have URLs to test
			status.Details["type"] = "notification"
			return status
		}
	}

	status.Status = "not_configured"
	status.Details["message"] = "Service not found in configuration"
	return status
}

// testIndexerQuery performs an actual test query against an indexer
func testIndexerQuery(indexer config.IndexersConfig, detailed bool) ServiceStatus {
	status := ServiceStatus{
		ServiceName: indexer.Name,
		Status:      "offline",
		Details:     make(map[string]interface{}),
	}

	// Start timing the query
	queryStart := time.Now()

	// Construct test query URL - basic capabilities query
	testURL := fmt.Sprintf("%s/api?t=caps&apikey=%s", indexer.URL, indexer.Apikey)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Get(testURL)

	queryDuration := time.Since(queryStart)
	status.ResponseTime = queryDuration
	status.Details["response_time_ms"] = queryDuration.Milliseconds()

	if err != nil {
		status.Status = "offline"
		status.ErrorMessage = fmt.Sprintf("Query failed: %v", err)
		status.Details["error"] = err.Error()
		return status
	}
	defer resp.Body.Close()

	status.StatusCode = resp.StatusCode

	if resp.StatusCode == 200 {
		status.Status = "online"
		status.Details["status_code"] = resp.StatusCode
		status.Details["test_query"] = "caps"

		if detailed {
			// Try to get some basic info from the response
			body, err := io.ReadAll(resp.Body)
			if err == nil {
				status.Details["response_size"] = len(body)

				// Check if response looks like valid Newznab XML
				if strings.Contains(string(body), "<caps>") || strings.Contains(string(body), "<categories>") {
					status.Details["api_format"] = "newznab"
					status.Details["caps_available"] = true

					// Count categories if present
					if catCount := strings.Count(string(body), "<category"); catCount > 0 {
						status.Details["categories_count"] = catCount
					}
				} else {
					status.Details["api_format"] = "unknown"
					status.Details["caps_available"] = false
				}
			}
		}
	} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
		status.Status = "authentication_error"
		status.ErrorMessage = "Authentication failed - check API key"
		status.Details["status_code"] = resp.StatusCode
		status.Details["error"] = "Invalid API key or insufficient permissions"
	} else if resp.StatusCode >= 500 {
		status.Status = "server_error"
		status.ErrorMessage = "Server error"
		status.Details["status_code"] = resp.StatusCode
		status.Details["error"] = "Indexer server error"
	} else {
		status.Status = "degraded"
		status.ErrorMessage = fmt.Sprintf("Unexpected response: %d", resp.StatusCode)
		status.Details["status_code"] = resp.StatusCode
		status.Details["error"] = "Unexpected HTTP status"
	}

	return status
}

// testURLConnectivity tests if a URL is reachable
func testURLConnectivity(url string) bool {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode < 500
}

// determineSeverity determines the severity of a service error
func determineSeverity(service, error string) string {
	serviceLower := strings.ToLower(service)
	errorLower := strings.ToLower(error)

	// Critical services
	if serviceLower == "database" {
		return "critical"
	}

	// High severity errors
	if strings.Contains(errorLower, "connection refused") ||
		strings.Contains(errorLower, "timeout") ||
		strings.Contains(errorLower, "network") {
		return "high"
	}

	// Medium severity errors
	if strings.Contains(errorLower, "authentication") ||
		strings.Contains(errorLower, "unauthorized") ||
		strings.Contains(errorLower, "rate limit") {
		return "medium"
	}

	// Low severity errors
	if strings.Contains(errorLower, "not found") ||
		strings.Contains(errorLower, "bad request") {
		return "low"
	}

	// Default to medium
	return "medium"
}

func getSuggestion(service, error string) string {
	serviceLower := strings.ToLower(service)
	errorLower := strings.ToLower(error)

	switch serviceLower {
	case "database":
		if strings.Contains(errorLower, "connection") {
			return "Check database connection settings and ensure the database is running"
		}
		if strings.Contains(errorLower, "timeout") {
			return "Increase connection timeout or check database performance"
		}
		return "Verify database configuration and connectivity"

	case "trakt", "trakt.tv":
		if strings.Contains(errorLower, "unauthorized") || strings.Contains(errorLower, "401") {
			return "Update Trakt.tv API credentials or re-authenticate"
		}
		if strings.Contains(errorLower, "rate limit") || strings.Contains(errorLower, "429") {
			return "Reduce request frequency, Trakt.tv API rate limit exceeded"
		}
		return "Verify Trakt.tv API settings and network connectivity"

	case "imdb":
		if strings.Contains(errorLower, "timeout") {
			return "Check network connectivity to IMDB or increase timeout"
		}
		if strings.Contains(errorLower, "forbidden") {
			return "IMDB may be blocking requests, consider using proxy or reducing frequency"
		}
		return "Check IMDB connectivity and request patterns"

	default:
		// For indexers and other services
		if strings.Contains(errorLower, "unauthorized") || strings.Contains(errorLower, "401") {
			return "Update API credentials or authentication settings"
		}
		if strings.Contains(errorLower, "timeout") {
			return "Check network connectivity and increase timeout if needed"
		}
		if strings.Contains(errorLower, "not found") || strings.Contains(errorLower, "404") {
			return "Verify service URL and endpoint configuration"
		}
		if strings.Contains(errorLower, "rate limit") || strings.Contains(errorLower, "429") {
			return "Reduce request frequency to avoid rate limiting"
		}
		if strings.Contains(errorLower, "connection refused") {
			return "Ensure service is running and accessible on the configured port"
		}
		if strings.Contains(errorLower, "ssl") || strings.Contains(errorLower, "certificate") {
			return "Check SSL/TLS certificate configuration or disable SSL verification"
		}
		return "Check service configuration and network connectivity"
	}
}

func generatePerformanceData(results []ServiceStatus) []PerformanceData {
	if len(results) == 0 {
		return []PerformanceData{}
	}

	performanceData := make([]PerformanceData, 0, len(results))

	for _, service := range results {
		if service.ResponseTime == 0 {
			continue
		}

		successRate := 1.0
		if service.Status == "offline" || service.Status == "error" {
			successRate = 0.0
		}

		perf := PerformanceData{
			ServiceName:     service.ServiceName,
			AvgResponseTime: service.ResponseTime,
			MinResponseTime: service.ResponseTime,
			MaxResponseTime: service.ResponseTime,
			SuccessRate:     successRate,
			TestCount:       1,
		}

		performanceData = append(performanceData, perf)
	}

	return performanceData
}

func calculateServiceHealth(results []ServiceStatus) (string, ServiceHealthSummary) {
	if len(results) == 0 {
		return "unknown", ServiceHealthSummary{
			TotalServices:   0,
			OnlineServices:  0,
			FailedServices:  0,
			AvgResponseTime: 0,
			OverallScore:    0,
		}
	}

	var totalResponseTime time.Duration
	responseTimeCount := 0
	onlineCount := 0
	failedCount := 0

	for _, service := range results {
		switch service.Status {
		case "online", "configured":
			onlineCount++
		case "offline", "error":
			failedCount++
		}

		if service.ResponseTime > 0 {
			totalResponseTime += service.ResponseTime
			responseTimeCount++
		}
	}

	avgResponseTime := 0.0
	if responseTimeCount > 0 {
		avgResponseTime = float64(totalResponseTime.Nanoseconds()) / float64(responseTimeCount) / 1e6 // Convert to milliseconds
	}

	totalServices := len(results)
	healthRatio := float64(onlineCount) / float64(totalServices)
	overallScore := healthRatio * 100

	var status string
	if failedCount == 0 {
		status = "healthy"
	} else if healthRatio >= 0.8 {
		status = "degraded"
	} else if healthRatio >= 0.5 {
		status = "unhealthy"
	} else {
		status = "critical"
	}

	summary := ServiceHealthSummary{
		TotalServices:   totalServices,
		OnlineServices:  onlineCount,
		FailedServices:  failedCount,
		AvgResponseTime: avgResponseTime,
		OverallScore:    overallScore,
	}

	return status, summary
}

// Conversion helper functions to bridge between different data structures
func convertOrphanedFiles(files []string) []OrphanedFile {
	result := make([]OrphanedFile, len(files))
	for i, file := range files {
		if stat, err := os.Stat(file); err == nil {
			result[i] = OrphanedFile{
				Path:         file,
				Size:         stat.Size(),
				ModTime:      stat.ModTime(),
				MediaType:    "unknown",
				CanBeDeleted: true,
			}
		}
	}
	return result
}

func convertDuplicateFiles(groups [][]string) []DuplicateSet {
	result := make([]DuplicateSet, len(groups))
	for i, group := range groups {
		files := make([]DuplicateFile, len(group))
		totalSize := int64(0)
		for j, file := range group {
			if stat, err := os.Stat(file); err == nil {
				files[j] = DuplicateFile{
					Path:    file,
					Size:    stat.Size(),
					ModTime: stat.ModTime(),
					Hash:    "",
				}
				totalSize += stat.Size()
			}
		}
		result[i] = DuplicateSet{
			Files:      files,
			CommonName: filepath.Base(group[0]),
			TotalSize:  totalSize,
			Confidence: 0.8,
		}
	}
	return result
}

func convertBrokenLinks(files []string) []BrokenLink {
	result := make([]BrokenLink, len(files))
	for i, file := range files {
		result[i] = BrokenLink{
			DatabaseID:   uint(i),
			TableName:    "unknown",
			FilePath:     file,
			MediaTitle:   filepath.Base(file),
			CanBeFixed:   false,
			SuggestedFix: "Remove from database",
		}
	}
	return result
}

func convertToMissingEpisode(episode database.SerieEpisode, serie database.Serie, seriesName string) MissingEpisode {
	var dbEpisode database.DbserieEpisode
	var airDate time.Time
	var hasAired bool
	var episodeTitle string
	var seasonNumber, episodeNumber int

	// Try to get episode details from database
	if err := dbEpisode.GetDbserieEpisodesByIDP(&episode.DbserieEpisodeID); err == nil {
		episodeTitle = dbEpisode.Title
		if dbEpisode.FirstAired.Valid {
			airDate = dbEpisode.FirstAired.Time
			hasAired = airDate.Before(time.Now())
		}

		// Parse season and episode numbers from strings
		if seasonNum, err := strconv.Atoi(dbEpisode.Season); err == nil {
			seasonNumber = seasonNum
		}
		if epNum, err := strconv.Atoi(dbEpisode.Episode); err == nil {
			episodeNumber = epNum
		}
	}

	qualityWanted := episode.QualityProfile
	if qualityWanted == "" {
		qualityWanted = "default"
	}

	return MissingEpisode{
		SeriesID:      episode.SerieID,
		SeriesName:    seriesName,
		SeasonNumber:  seasonNumber,
		EpisodeNumber: episodeNumber,
		EpisodeTitle:  episodeTitle,
		AirDate:       airDate,
		HasAired:      hasAired,
		QualityWanted: qualityWanted,
	}
}

// Storage type conversion functions - convert from storage_functions.go types to management types
func convertStorageDiskInfo(diskInfo []DiskSpaceInfo) []DiskSpaceInfo {
	// Types are the same, just return as-is
	return diskInfo
}

func convertStoragePermissionInfo(permInfo []PermissionInfo) []PermissionIssue {
	result := make([]PermissionIssue, 0)
	for _, info := range permInfo {
		if info.Status != "healthy" {
			result = append(result, PermissionIssue{
				Path:       info.Path,
				Issue:      info.ErrorMessage,
				Severity:   info.Status,
				Suggestion: "Check file permissions",
				CanAutoFix: false,
			})
		}
	}
	return result
}

func convertStorageMountInfo(mountInfo []MountInfo) []MountInfo {
	// Types are the same, just return as-is
	return mountInfo
}

func convertStorageIOHealthInfo(ioInfo []IOHealthInfo) []IOHealthInfo {
	// Types are the same, just return as-is
	return ioInfo
}

// CronValidationRequest represents a request to validate a cron expression
type CronValidationRequest struct {
	Expression string `json:"expression"`
}

// CronValidationResponse represents the response for cron validation
type CronValidationResponse struct {
	Valid       bool     `json:"valid"`
	Description string   `json:"description"`
	NextRuns    []string `json:"next_runs"`
	Error       string   `json:"error"`
}

// HandleCronValidation validates a cron expression and provides description
func HandleCronValidation(ctx *gin.Context) {
	// Get expression from form data (frontend sends as form data, not JSON)
	expression := strings.TrimSpace(ctx.PostForm("expression"))

	if expression == "" {
		ctx.Header("Content-Type", "text/html")
		ctx.String(http.StatusOK, `<div class="alert alert-warning mb-0">
			<i class="fa-solid fa-exclamation-triangle me-2"></i>
			<strong>Empty Expression</strong> - Please enter a cron expression to validate.
		</div>`)
		return
	}

	// Parse and validate the cron expression using robfig/cron with 6-field support (seconds included)
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expression)
	if err != nil {
		// Try 5-field format if 6-field fails
		parser5 := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule5, err5 := parser5.Parse(expression)
		if err5 != nil {
			ctx.Header("Content-Type", "text/html")
			ctx.String(http.StatusOK, `<div class="alert alert-danger mb-0">
				<i class="fa-solid fa-times-circle me-2"></i>
				<strong>Invalid Expression</strong> - %s
				<br><small class="text-muted mt-1 d-block">Supports both 5-field (minute hour day month weekday) and 6-field (second minute hour day month weekday) formats.</small>
			</div>`, err.Error())
			return
		}
		// Use 5-field schedule
		schedule = schedule5
	}

	// Generate human-readable description
	description := generateCronDescription(expression)

	// Calculate next 5 run times
	now := time.Now()
	nextRuns := make([]string, 0, 5)
	nextTime := now
	for i := 0; i < 5; i++ {
		nextTime = schedule.Next(nextTime)
		nextRuns = append(nextRuns, nextTime.Format("Mon, 02 Jan 2006 15:04:05"))
	}

	// Return formatted HTML response
	ctx.Header("Content-Type", "text/html")
	var html strings.Builder
	html.WriteString(`<div class="alert alert-success mb-3">
		<i class="fa-solid fa-check-circle me-2"></i>
		<strong>Valid Expression</strong> - ` + description + `
	</div>
	<div class="card">
		<div class="card-header bg-light">
			<h6 class="card-title mb-0">
				<i class="fa-solid fa-calendar-alt me-2"></i>Next 5 Executions
			</h6>
		</div>
		<div class="card-body">
			<ul class="list-unstyled mb-0">`)

	for i, runTime := range nextRuns {
		html.WriteString(fmt.Sprintf(`
				<li class="d-flex align-items-center mb-2">
					<span class="badge bg-primary me-3">%d</span>
					<span class="font-monospace">%s</span>
				</li>`, i+1, runTime))
	}

	html.WriteString(`
			</ul>
		</div>
	</div>`)

	ctx.String(http.StatusOK, html.String())
}

// generateCronDescription creates a human-readable description of a cron expression
func generateCronDescription(expression string) string {
	parts := strings.Fields(expression)
	if len(parts) != 5 && len(parts) != 6 {
		return "Invalid cron format (must be 5 or 6 fields)"
	}

	var second, minute, hour, day, month, weekday string

	if len(parts) == 6 {
		// 6-field format: second minute hour day month weekday
		second = parts[0]
		minute = parts[1]
		hour = parts[2]
		day = parts[3]
		month = parts[4]
		weekday = parts[5]
	} else {
		// 5-field format: minute hour day month weekday
		second = "0" // Default to 0 seconds
		minute = parts[0]
		hour = parts[1]
		day = parts[2]
		month = parts[3]
		weekday = parts[4]
	}

	var desc strings.Builder

	// Handle special cases first - support both 5 and 6 field formats
	switch expression {
	// 5-field common patterns
	case "0 0 * * *", "0 0 0 * * *":
		return "Daily at midnight (00:00)"
	case "0 12 * * *", "0 0 12 * * *":
		return "Daily at noon (12:00)"
	case "0 0 * * 0", "0 0 0 * * 0":
		return "Weekly on Sunday at midnight"
	case "0 0 1 * *", "0 0 0 1 * *":
		return "Monthly on the 1st at midnight"
	case "*/5 * * * *", "0 */5 * * * *":
		return "Every 5 minutes"
	case "*/15 * * * *", "0 */15 * * * *":
		return "Every 15 minutes"
	case "0 * * * *", "0 0 * * * *":
		return "Every hour at minute 0"
	// 6-field specific patterns
	case "*/30 * * * * *":
		return "Every 30 seconds"
	case "*/10 * * * * *":
		return "Every 10 seconds"
	case "*/5 * * * * *":
		return "Every 5 seconds"
	case "* * * * * *":
		return "Every second"
	}

	desc.WriteString("Run ")

	// Frequency - handle both 5 and 6 field expressions
	if second == "*" && minute == "*" && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		desc.WriteString("every second")
	} else if minute == "*" && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		if strings.HasPrefix(second, "*/") {
			interval := strings.TrimPrefix(second, "*/")
			desc.WriteString(fmt.Sprintf("every %s seconds", interval))
		} else if second == "0" || len(parts) == 5 {
			desc.WriteString("every minute")
		} else {
			desc.WriteString(fmt.Sprintf("at second %s of every minute", second))
		}
	} else if hour == "*" && day == "*" && month == "*" && weekday == "*" {
		if strings.HasPrefix(minute, "*/") {
			interval := strings.TrimPrefix(minute, "*/")
			desc.WriteString(fmt.Sprintf("every %s minutes", interval))
		} else if minute == "0" {
			desc.WriteString("every hour")
		} else {
			timeStr := formatTimeWithSeconds(hour, minute, second)
			desc.WriteString(fmt.Sprintf("at %s of every hour", timeStr))
		}
	} else if day == "*" && month == "*" && weekday == "*" {
		// Daily
		timeStr := formatTimeWithSeconds(hour, minute, second)
		desc.WriteString(fmt.Sprintf("daily at %s", timeStr))
	} else if month == "*" && weekday == "*" {
		// Monthly
		timeStr := formatTimeWithSeconds(hour, minute, second)
		switch day {
		case "1":
			{
				desc.WriteString(fmt.Sprintf("monthly on the 1st at %s", timeStr))
			}
		case "15":
			{
				desc.WriteString(fmt.Sprintf("monthly on the 15th at %s", timeStr))
			}
		default:
			{
				desc.WriteString(fmt.Sprintf("monthly on day %s at %s", day, timeStr))
			}
		}
	} else if month == "*" && day == "*" {
		// Weekly
		timeStr := formatTimeWithSeconds(hour, minute, second)
		weekdayName := getWeekdayName(weekday)
		desc.WriteString(fmt.Sprintf("weekly on %s at %s", weekdayName, timeStr))
	} else {
		// Complex schedule
		desc.WriteString("on a complex schedule")
		if len(parts) == 6 && second != "*" && second != "0" {
			desc.WriteString(fmt.Sprintf(" (second: %s", second))
		} else {
			desc.WriteString(" (")
		}
		needComma := false
		if len(parts) == 6 && second != "*" && second != "0" {
			needComma = true
		}
		if minute != "*" {
			if needComma {
				desc.WriteString(", ")
			}
			desc.WriteString(fmt.Sprintf("minute: %s", minute))
			needComma = true
		}
		if hour != "*" {
			if needComma {
				desc.WriteString(", ")
			}
			desc.WriteString(fmt.Sprintf("hour: %s", hour))
			needComma = true
		}
		if day != "*" {
			if needComma {
				desc.WriteString(", ")
			}
			desc.WriteString(fmt.Sprintf("day: %s", day))
			needComma = true
		}
		if month != "*" {
			if needComma {
				desc.WriteString(", ")
			}
			desc.WriteString(fmt.Sprintf("month: %s", month))
			needComma = true
		}
		if weekday != "*" {
			if needComma {
				desc.WriteString(", ")
			}
			desc.WriteString(fmt.Sprintf("weekday: %s", weekday))
		}
		desc.WriteString(")")
	}

	return desc.String()
}

// formatTimeWithSeconds formats hour, minute, and second parts into readable time
func formatTimeWithSeconds(hour, minute, second string) string {
	h, hErr := strconv.Atoi(hour)
	m, mErr := strconv.Atoi(minute)
	s, sErr := strconv.Atoi(second)

	if hErr != nil || mErr != nil {
		// If seconds parsing fails, fall back to hour:minute format
		if sErr != nil {
			return fmt.Sprintf("%s:%s", hour, minute)
		}
		return fmt.Sprintf("%s:%s:%02d", hour, minute, s)
	}

	// If second is 0 or not provided, use HH:MM format
	if sErr != nil || s == 0 {
		return fmt.Sprintf("%02d:%02d", h, m)
	}

	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// getWeekdayName converts weekday number to name
func getWeekdayName(weekday string) string {
	switch weekday {
	case "0":
		return "Sunday"
	case "1":
		return "Monday"
	case "2":
		return "Tuesday"
	case "3":
		return "Wednesday"
	case "4":
		return "Thursday"
	case "5":
		return "Friday"
	case "6":
		return "Saturday"
	default:
		return weekday
	}
}

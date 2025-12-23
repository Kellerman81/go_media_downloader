package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
)

var serverStartTime = time.Now()

// Statistics data structures

// MovieStatistics contains movie-related statistics.
type MovieStatistics struct {
	TotalMovies      int            `json:"total_movies"`
	AvailableMovies  int            `json:"available_movies"`
	MissingMovies    int            `json:"missing_movies"`
	QualityReached   int            `json:"quality_reached"`
	UpgradeAvailable int            `json:"upgrade_available"`
	ByQuality        map[string]int `json:"by_quality"`
	ByList           map[string]int `json:"by_list"`
	TotalSizeBytes   int64          `json:"total_size_bytes"`
}

// SeriesStatistics contains series-related statistics.
type SeriesStatistics struct {
	TotalSeries       int            `json:"total_series"`
	AvailableSeries   int            `json:"available_series"`
	MissingSeries     int            `json:"missing_series"`
	TotalEpisodes     int            `json:"total_episodes"`
	AvailableEpisodes int            `json:"available_episodes"`
	MissingEpisodes   int            `json:"missing_episodes"`
	QualityReached    int            `json:"quality_reached"`
	UpgradeAvailable  int            `json:"upgrade_available"`
	ByQuality         map[string]int `json:"by_quality"`
	ByList            map[string]int `json:"by_list"`
	TotalSizeBytes    int64          `json:"total_size_bytes"`
	TotalSeasons      int            `json:"total_seasons"`
}

// StorageStatistics contains storage-related statistics.
type StorageStatistics struct {
	TotalPaths     int                      `json:"total_paths"`
	ByPath         map[string]PathStatistic `json:"by_path"`
	TotalFileCount int64                    `json:"total_file_count"`
}

// PathStatistic contains statistics for a single storage path.
type PathStatistic struct {
	Path        string `json:"path"`
	FileCount   int64  `json:"file_count"`
	FolderCount int64  `json:"folder_count"`
	TotalSize   int64  `json:"total_size"`
}

// HTTPStatistics contains HTTP client statistics.
type HTTPStatistics struct {
	TotalClients  int                    `json:"total_clients"`
	ClientStats   map[string]ClientStats `json:"client_stats"`
	TotalRequests int64                  `json:"total_requests"`
	SuccessRate   float64                `json:"success_rate"`
}

// ProviderStatistics contains provider statistics (reused for metadata, notification, downloader).
type ProviderStatistics struct {
	TotalProviders int                    `json:"total_providers"`
	ProviderStats  map[string]ClientStats `json:"provider_stats"`
	TotalRequests  int64                  `json:"total_requests"`
	SuccessRate    float64                `json:"success_rate"`
}

// ClientStats contains per-client HTTP statistics.
type ClientStats struct {
	Name                string    `json:"name"`
	Requests1h          int64     `json:"requests_1h"`
	Requests24h         int64     `json:"requests_24h"`
	RequestsTotal       int64     `json:"requests_total"`
	AvgResponseTimeMs   float64   `json:"avg_response_time_ms"`
	LastRequestAt       time.Time `json:"last_request_at"`
	LastErrorAt         time.Time `json:"last_error_at"`
	LastErrorMessage    string    `json:"last_error_message"`
	NextAvailableAt     time.Time `json:"next_available_at"`
	SuccessCount        int64     `json:"success_count"`
	FailureCount        int64     `json:"failure_count"`
	CircuitBreakerState string    `json:"circuit_breaker_state"`
}

// SystemStatistics contains system performance statistics.
type SystemStatistics struct {
	GOOS          string    `json:"goos"`
	GOARCH        string    `json:"goarch"`
	NumCPU        int       `json:"num_cpu"`
	NumGoroutine  int       `json:"num_goroutine"`
	GoRoutines    int       `json:"go_routines"` // Alias for JavaScript compatibility
	UptimeSeconds int64     `json:"uptime_seconds"`
	MemoryAllocMB float64   `json:"memory_alloc_mb"`
	MemorySysMB   float64   `json:"memory_sys_mb"`
	GCCount       uint32    `json:"gc_count"`
	LastGCTime    time.Time `json:"last_gc_time"`
}

// OverallStatistics aggregates all statistics.
type OverallStatistics struct {
	Movies  MovieStatistics   `json:"movies"`
	Series  SeriesStatistics  `json:"series"`
	Storage StorageStatistics `json:"storage"`
	HTTP    HTTPStatistics    `json:"http"`
	Workers worker.Stats      `json:"workers"`
	System  SystemStatistics  `json:"system"`
}

// Statistics collection functions

// getMovieStatistics retrieves movie statistics from the database.
func getMovieStatistics(ctx context.Context) (MovieStatistics, error) {
	stats := MovieStatistics{
		ByQuality: make(map[string]int),
		ByList:    make(map[string]int),
	}

	// Total movies
	stats.TotalMovies = int(database.Getdatarow[uint](false, "SELECT COUNT(*) FROM movies"))

	// Available movies (with files)
	stats.AvailableMovies = int(database.Getdatarow[uint](false,
		`SELECT COUNT(DISTINCT m.id) FROM movies m
		INNER JOIN movie_files mf ON mf.movie_id = m.id`))
	stats.MissingMovies = stats.TotalMovies - stats.AvailableMovies

	// Quality reached
	stats.QualityReached = int(database.Getdatarow[uint](false,
		"SELECT COUNT(*) FROM movies WHERE quality_reached = 1"))

	// Upgrade available
	stats.UpgradeAvailable = int(database.Getdatarow[uint](false,
		"SELECT COUNT(*) FROM movies WHERE quality_reached = 0 AND missing = 0"))

	// Movies by quality profile
	qualityData := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 0,
		`SELECT mf.quality_profile, COUNT(*) as count
		FROM movie_files mf
		WHERE mf.quality_profile IS NOT NULL AND mf.quality_profile != ''
		GROUP BY mf.quality_profile
		ORDER BY count DESC`)
	for _, q := range qualityData {
		if q.Str != "" {
			stats.ByQuality[q.Str] = int(q.Num)
		}
	}

	// Movies by list
	listData := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 0,
		`SELECT listname, COUNT(*) as count
		FROM movies
		WHERE listname IS NOT NULL AND listname != ''
		GROUP BY listname
		ORDER BY count DESC`)
	for _, l := range listData {
		if l.Str != "" {
			stats.ByList[l.Str] = int(l.Num)
		}
	}

	// Total storage size - removed: size column doesn't exist in movie_files table
	stats.TotalSizeBytes = 0

	return stats, nil
}

// getSeriesStatistics retrieves series statistics from the database.
func getSeriesStatistics(ctx context.Context) (SeriesStatistics, error) {
	stats := SeriesStatistics{
		ByQuality: make(map[string]int),
		ByList:    make(map[string]int),
	}

	// Total series
	stats.TotalSeries = int(database.Getdatarow[uint](false, "SELECT COUNT(*) FROM series"))

	// Available series (with at least one episode file)
	stats.AvailableSeries = int(database.Getdatarow[uint](false,
		`SELECT COUNT(DISTINCT s.id) FROM series s
		INNER JOIN serie_episodes se ON se.serie_id = s.id
		INNER JOIN serie_episode_files sef ON sef.serie_episode_id = se.id`))
	stats.MissingSeries = stats.TotalSeries - stats.AvailableSeries

	// Total episodes
	stats.TotalEpisodes = int(
		database.Getdatarow[uint](false, "SELECT COUNT(*) FROM serie_episodes"),
	)

	// Available episodes (with files)
	stats.AvailableEpisodes = int(database.Getdatarow[uint](false,
		`SELECT COUNT(DISTINCT se.id) FROM serie_episodes se
		INNER JOIN serie_episode_files sef ON sef.serie_episode_id = se.id`))
	stats.MissingEpisodes = stats.TotalEpisodes - stats.AvailableEpisodes

	// Quality reached
	stats.QualityReached = int(database.Getdatarow[uint](false,
		"SELECT COUNT(*) FROM serie_episodes WHERE quality_reached = 1"))

	// Upgrade available
	stats.UpgradeAvailable = int(database.Getdatarow[uint](false,
		"SELECT COUNT(*) FROM serie_episodes WHERE quality_reached = 0 AND missing = 0"))

	// Episodes by quality profile
	qualityData := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 0,
		`SELECT sef.quality_profile, COUNT(*) as count
		FROM serie_episode_files sef
		WHERE sef.quality_profile IS NOT NULL AND sef.quality_profile != ''
		GROUP BY sef.quality_profile
		ORDER BY count DESC`)
	for _, q := range qualityData {
		if q.Str != "" {
			stats.ByQuality[q.Str] = int(q.Num)
		}
	}

	// Series by list
	listData := database.GetrowsN[database.DbstaticOneStringOneUInt](false, 0,
		`SELECT listname, COUNT(*) as count
		FROM series
		WHERE listname IS NOT NULL AND listname != ''
		GROUP BY listname
		ORDER BY count DESC`)
	for _, l := range listData {
		if l.Str != "" {
			stats.ByList[l.Str] = int(l.Num)
		}
	}

	// Total storage size - removed: size column doesn't exist in serie_episode_files table
	stats.TotalSizeBytes = 0

	// Total seasons
	stats.TotalSeasons = int(database.Getdatarow[uint](false,
		`SELECT COUNT(DISTINCT identifier) FROM dbserie_episodes
		WHERE identifier != '' AND identifier IS NOT NULL`))

	return stats, nil
}

// getStorageStatistics retrieves storage statistics by walking configured media paths.
func getStorageStatistics(ctx context.Context) StorageStatistics {
	stats := StorageStatistics{
		ByPath: make(map[string]PathStatistic),
	}

	// Track unique paths to avoid duplicates
	processedPaths := make(map[string]bool)

	// Iterate through all media configurations
	config.RangeSettingsMedia(func(mediaName string, mediaConfig *config.MediaTypeConfig) error {
		// Process Data paths
		for _, dataConfig := range mediaConfig.Data {
			if dataConfig.CfgPath == nil || dataConfig.CfgPath.Path == "" {
				continue
			}

			path := dataConfig.CfgPath.Path
			if processedPaths[path] {
				continue
			}

			processedPaths[path] = true

			pathStat := walkPath(ctx, path, mediaName)
			if pathStat.FileCount > 0 || pathStat.FolderCount > 0 {
				stats.ByPath[mediaName+" ("+dataConfig.CfgPath.Name+")"] = pathStat

				stats.TotalFileCount += pathStat.FileCount
				stats.TotalPaths++
			}
		}

		// Process DataImport paths
		for _, importConfig := range mediaConfig.DataImport {
			if importConfig.CfgPath == nil || importConfig.CfgPath.Path == "" {
				continue
			}

			path := importConfig.CfgPath.Path
			if processedPaths[path] {
				continue
			}

			processedPaths[path] = true

			pathStat := walkPath(ctx, path, mediaName+" Import")
			if pathStat.FileCount > 0 || pathStat.FolderCount > 0 {
				stats.ByPath[mediaName+" Import ("+importConfig.CfgPath.Name+")"] = pathStat

				stats.TotalFileCount += pathStat.FileCount
				stats.TotalPaths++
			}
		}

		return nil
	})

	return stats
}

// walkPath walks a filesystem path and returns statistics.
func walkPath(ctx context.Context, rootPath string, mediaType string) PathStatistic {
	stat := PathStatistic{
		Path: rootPath,
	}

	// Check if path exists
	if _, err := os.Stat(rootPath); os.IsNotExist(err) {
		return stat
	}

	// Walk the directory tree
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if err != nil {
			// Log but continue on errors
			logger.Logtype(logger.StatusDebug, 3).
				Str("path", path).
				Err(err).
				Msg("Error accessing path during storage stats")

			return nil
		}

		if d.IsDir() {
			stat.FolderCount++
		} else {
			// Only count video files
			ext := strings.ToLower(filepath.Ext(d.Name()))
			if isVideoFile(ext) {
				stat.FileCount++
				if info, err := d.Info(); err == nil {
					stat.TotalSize += info.Size()
				}
			}
		}

		return nil
	})

	if err != nil && !errors.Is(err, context.Canceled) &&
		!errors.Is(err, context.DeadlineExceeded) {
		logger.Logtype(logger.StatusError, 1).
			Str("path", rootPath).
			Err(err).
			Msg("Failed to walk storage path")
	}

	return stat
}

// isVideoFile checks if the file extension is a video file.
func isVideoFile(ext string) bool {
	videoExtensions := map[string]bool{
		".mkv":  true,
		".mp4":  true,
		".avi":  true,
		".m4v":  true,
		".mov":  true,
		".wmv":  true,
		".flv":  true,
		".webm": true,
		".mpg":  true,
		".mpeg": true,
		".m2ts": true,
		".ts":   true,
		".vob":  true,
		".3gp":  true,
		".divx": true,
		".xvid": true,
	}

	return videoExtensions[ext]
}

// getHTTPStatistics retrieves HTTP client statistics.
func getHTTPStatistics() HTTPStatistics {
	stats := HTTPStatistics{
		ClientStats: make(map[string]ClientStats),
	}

	// Get all indexer clients
	allIndexers := providers.GetAllIndexers()

	stats.TotalClients = len(allIndexers)

	var totalSuccess, totalFailure int64

	for name, provider := range allIndexers {
		clientStats := provider.GetStats()
		cs := ClientStats{
			Name:                name,
			Requests1h:          clientStats.Requests1h,
			Requests24h:         clientStats.Requests24h,
			RequestsTotal:       clientStats.RequestsTotal,
			AvgResponseTimeMs:   float64(clientStats.AvgResponseTimeMs),
			LastRequestAt:       clientStats.LastRequestAt,
			LastErrorAt:         clientStats.LastErrorAt,
			LastErrorMessage:    clientStats.LastErrorMessage,
			NextAvailableAt:     clientStats.NextAvailableAt,
			SuccessCount:        clientStats.SuccessCount,
			FailureCount:        clientStats.FailureCount,
			CircuitBreakerState: clientStats.CircuitBreakerState,
		}

		stats.ClientStats[name] = cs

		stats.TotalRequests += clientStats.RequestsTotal
		totalSuccess += clientStats.SuccessCount
		totalFailure += clientStats.FailureCount
	}

	// Calculate overall success rate
	if totalSuccess+totalFailure > 0 {
		stats.SuccessRate = float64(totalSuccess) / float64(totalSuccess+totalFailure) * 100
	}

	return stats
}

// getMetadataProviderStatistics retrieves metadata provider statistics.
func getMetadataProviderStatistics() ProviderStatistics {
	stats := ProviderStatistics{
		ProviderStats: make(map[string]ClientStats),
	}

	allProviders := providers.GetAllMetadataProviders()

	stats.TotalProviders = len(allProviders)

	var totalSuccess, totalFailure int64

	for name, provider := range allProviders {
		// Each metadata provider has a BaseClient field, use reflection or type assertion
		// to get stats. They embed *base.BaseClient
		var baseStats base.ClientStats

		switch p := provider.(type) {
		case interface{ GetStats() base.ClientStats }:
			baseStats = p.GetStats()
		default:
			continue
		}

		cs := ClientStats{
			Name:                name,
			Requests1h:          baseStats.Requests1h,
			Requests24h:         baseStats.Requests24h,
			RequestsTotal:       baseStats.RequestsTotal,
			AvgResponseTimeMs:   float64(baseStats.AvgResponseTimeMs),
			LastRequestAt:       baseStats.LastRequestAt,
			LastErrorAt:         baseStats.LastErrorAt,
			LastErrorMessage:    baseStats.LastErrorMessage,
			NextAvailableAt:     baseStats.NextAvailableAt,
			SuccessCount:        baseStats.SuccessCount,
			FailureCount:        baseStats.FailureCount,
			CircuitBreakerState: baseStats.CircuitBreakerState,
		}

		stats.ProviderStats[name] = cs

		stats.TotalRequests += baseStats.RequestsTotal
		totalSuccess += baseStats.SuccessCount
		totalFailure += baseStats.FailureCount
	}

	// Calculate overall success rate
	if totalSuccess+totalFailure > 0 {
		stats.SuccessRate = float64(totalSuccess) / float64(totalSuccess+totalFailure) * 100
	}

	return stats
}

// getNotificationProviderStatistics retrieves notification provider statistics.
func getNotificationProviderStatistics() ProviderStatistics {
	stats := ProviderStatistics{
		ProviderStats: make(map[string]ClientStats),
	}

	// Get the global client manager for notification providers
	manager, ok := apiexternal_v2.GetGlobalClientManager()
	if !ok {
		return stats
	}

	allProviders := manager.GetAllNotificationProviders()

	stats.TotalProviders = len(allProviders)

	var totalSuccess, totalFailure int64

	for name, provider := range allProviders {
		// Notification providers embed *base.BaseClient
		var baseStats base.ClientStats

		switch p := provider.(type) {
		case interface{ GetStats() base.ClientStats }:
			baseStats = p.GetStats()
		default:
			continue
		}

		cs := ClientStats{
			Name:                name,
			Requests1h:          baseStats.Requests1h,
			Requests24h:         baseStats.Requests24h,
			RequestsTotal:       baseStats.RequestsTotal,
			AvgResponseTimeMs:   float64(baseStats.AvgResponseTimeMs),
			LastRequestAt:       baseStats.LastRequestAt,
			LastErrorAt:         baseStats.LastErrorAt,
			LastErrorMessage:    baseStats.LastErrorMessage,
			NextAvailableAt:     baseStats.NextAvailableAt,
			SuccessCount:        baseStats.SuccessCount,
			FailureCount:        baseStats.FailureCount,
			CircuitBreakerState: baseStats.CircuitBreakerState,
		}

		stats.ProviderStats[name] = cs

		stats.TotalRequests += baseStats.RequestsTotal
		totalSuccess += baseStats.SuccessCount
		totalFailure += baseStats.FailureCount
	}

	// Calculate overall success rate
	if totalSuccess+totalFailure > 0 {
		stats.SuccessRate = float64(totalSuccess) / float64(totalSuccess+totalFailure) * 100
	}

	return stats
}

// getDownloadProviderStatistics retrieves download provider statistics.
func getDownloadProviderStatistics() ProviderStatistics {
	stats := ProviderStatistics{
		ProviderStats: make(map[string]ClientStats),
	}

	allProviders := providers.GetAllDownloadProviders()

	stats.TotalProviders = len(allProviders)

	var totalSuccess, totalFailure int64

	for name, provider := range allProviders {
		// Download providers embed *base.BaseClient
		var baseStats base.ClientStats

		switch p := provider.(type) {
		case interface{ GetStats() base.ClientStats }:
			baseStats = p.GetStats()
		default:
			continue
		}

		cs := ClientStats{
			Name:                name,
			Requests1h:          baseStats.Requests1h,
			Requests24h:         baseStats.Requests24h,
			RequestsTotal:       baseStats.RequestsTotal,
			AvgResponseTimeMs:   float64(baseStats.AvgResponseTimeMs),
			LastRequestAt:       baseStats.LastRequestAt,
			LastErrorAt:         baseStats.LastErrorAt,
			LastErrorMessage:    baseStats.LastErrorMessage,
			NextAvailableAt:     baseStats.NextAvailableAt,
			SuccessCount:        baseStats.SuccessCount,
			FailureCount:        baseStats.FailureCount,
			CircuitBreakerState: baseStats.CircuitBreakerState,
		}

		stats.ProviderStats[name] = cs

		stats.TotalRequests += baseStats.RequestsTotal
		totalSuccess += baseStats.SuccessCount
		totalFailure += baseStats.FailureCount
	}

	// Calculate overall success rate
	if totalSuccess+totalFailure > 0 {
		stats.SuccessRate = float64(totalSuccess) / float64(totalSuccess+totalFailure) * 100
	}

	return stats
}

// getSystemStatistics retrieves system performance statistics.
func getSystemStatistics() SystemStatistics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	var gc debug.GCStats
	debug.ReadGCStats(&gc)

	numGoroutines := runtime.NumGoroutine()
	uptimeSeconds := int64(time.Since(serverStartTime).Seconds())

	stats := SystemStatistics{
		GOOS:          runtime.GOOS,
		GOARCH:        runtime.GOARCH,
		NumCPU:        runtime.NumCPU(),
		NumGoroutine:  numGoroutines,
		GoRoutines:    numGoroutines,
		UptimeSeconds: uptimeSeconds,
		MemoryAllocMB: float64(mem.Alloc) / 1024 / 1024,
		MemorySysMB:   float64(mem.Sys) / 1024 / 1024,
		GCCount:       mem.NumGC,
	}

	if len(gc.PauseEnd) > 0 {
		stats.LastGCTime = gc.PauseEnd[0]
	}

	return stats
}

// gatherAllStatistics collects all statistics.
func gatherAllStatistics(ctx context.Context) (OverallStatistics, error) {
	var (
		overall OverallStatistics
		err     error
	)

	overall.Movies, err = getMovieStatistics(ctx)
	if err != nil {
		return overall, fmt.Errorf("failed to get movie statistics: %w", err)
	}

	overall.Series, err = getSeriesStatistics(ctx)
	if err != nil {
		return overall, fmt.Errorf("failed to get series statistics: %w", err)
	}

	overall.Storage = getStorageStatistics(ctx)
	overall.HTTP = getHTTPStatistics()
	overall.Workers = worker.GetStats()
	overall.System = getSystemStatistics()

	return overall, nil
}

// API Handlers

// @Summary      Get Statistics
// @Description  Retrieves comprehensive system statistics including movies, series, storage, HTTP, and system performance
// @Tags         statistics
// @Produce      json
// @Param        apikey query string true "API Key"
// @Success      200 {object} OverallStatistics
// @Failure      500 {object} Jsonerror
// @Router       /api/statistics [get].
func apiStatistics(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := gatherAllStatistics(requestCtx)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Failed to gather statistics")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// webStatisticsData returns statistics data for the web admin interface (uses session auth).
func webStatisticsData(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats, err := gatherAllStatistics(requestCtx)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Failed to gather statistics")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// @Summary      Get Movie Statistics
// @Description  Retrieves movie-specific statistics
// @Tags         statistics
// @Produce      json
// @Param        apikey query string true "API Key"
// @Success      200 {object} MovieStatistics
// @Failure      500 {object} Jsonerror
// @Router       /api/statistics/movies [get].
func apiMovieStatistics(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := getMovieStatistics(requestCtx)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Failed to get movie statistics")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// @Summary      Get Series Statistics
// @Description  Retrieves series-specific statistics
// @Tags         statistics
// @Produce      json
// @Param        apikey query string true "API Key"
// @Success      200 {object} SeriesStatistics
// @Failure      500 {object} Jsonerror
// @Router       /api/statistics/series [get].
func apiSeriesStatistics(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := getSeriesStatistics(requestCtx)
	if err != nil {
		logger.Logtype(logger.StatusError, 1).Err(err).Msg("Failed to get series statistics")
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, stats)
}

// @Summary      Get Worker Statistics
// @Description  Retrieves worker pool statistics
// @Tags         statistics
// @Produce      json
// @Param        apikey query string true "API Key"
// @Success      200 {object} worker.Stats
// @Failure      500 {object} Jsonerror
// @Router       /api/statistics/workers [get].
func apiWorkerStatistics(ctx *gin.Context) {
	stats := worker.GetStats()
	ctx.JSON(http.StatusOK, stats)
}

// Web-specific HTMX handlers for individual sections

func webStatisticsMovies(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := getMovieStatistics(requestCtx)
	if err != nil {
		ctx.String(
			http.StatusInternalServerError,
			`<div class="alert alert-danger">Error loading movie statistics: %s</div>`,
			err.Error(),
		)

		return
	}

	// Check if this is a summary card or detail section based on query param or header
	cardType := ctx.Query("type")
	if cardType == "" {
		cardType = ctx.GetHeader("HX-Card-Type")
	}

	var component gomponents.Node
	if cardType == "summary" {
		component = renderMovieCard(stats)
	} else {
		component = renderMovieDetails(stats)
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsSeries(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	stats, err := getSeriesStatistics(requestCtx)
	if err != nil {
		ctx.String(
			http.StatusInternalServerError,
			`<div class="alert alert-danger">Error loading series statistics: %s</div>`,
			err.Error(),
		)

		return
	}

	cardType := ctx.Query("type")
	if cardType == "" {
		cardType = ctx.GetHeader("HX-Card-Type")
	}

	var component gomponents.Node
	if cardType == "summary" {
		component = renderSeriesCard(stats)
	} else {
		component = renderSeriesDetails(stats)
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsStorage(ctx *gin.Context) {
	requestCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stats := getStorageStatistics(requestCtx)

	cardType := ctx.Query("type")
	if cardType == "" {
		cardType = ctx.GetHeader("HX-Card-Type")
	}

	var component gomponents.Node
	if cardType == "summary" {
		component = renderStorageCard(stats)
	} else {
		component = renderStorageDetails(stats)
	}

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsWorkers(ctx *gin.Context) {
	stats := worker.GetStats()
	component := renderWorkerPoolStats(stats)

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsHTTP(ctx *gin.Context) {
	stats := getHTTPStatistics()
	component := renderHTTPClientStats(stats)

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsMetadata(ctx *gin.Context) {
	stats := getMetadataProviderStatistics()
	component := renderProviderStats(stats, "Metadata Providers", "primary")

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsNotification(ctx *gin.Context) {
	stats := getNotificationProviderStatistics()
	component := renderProviderStats(stats, "Notification Providers", "info")

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsDownloader(ctx *gin.Context) {
	stats := getDownloadProviderStatistics()
	component := renderProviderStats(stats, "Downloader Providers", "success")

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

func webStatisticsSystem(ctx *gin.Context) {
	stats := getSystemStatistics()
	component := renderSystemCard(stats)

	ctx.Header("Content-Type", "text/html; charset=utf-8")

	_ = component.Render(ctx.Writer)
}

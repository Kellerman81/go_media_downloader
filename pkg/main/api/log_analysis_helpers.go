package api

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// LogAnalysisResults holds the results of log file analysis.
type LogAnalysisResults struct {
	TotalEntries      int64
	ErrorCount        int64
	WarningCount      int64
	InfoCount         int64
	ErrorPatterns     map[string]int // pattern -> count
	AccessPatterns    map[string]int // endpoint -> count
	PerformanceIssues []string
	TopErrorPattern   string // most common error
	TopAccessPattern  string // most accessed endpoint
	AvgResponseTime   string // calculated average response time
	MaxResponseTime   string // maximum response time found
	SlowestOperation  string // slowest operation found
}

// getLogFilePath returns the path to the application log file.
func getLogFilePath() (string, error) {
	// Try to get log file path from config or use default
	logPaths := []string{
		"logs/downloader.log",
		"../logs/downloader.log",
		"../../logs/downloader.log",
		"/var/log/go_media_downloader.log",
		"./downloader.log",
	}

	for _, logPath := range logPaths {
		if _, err := os.Stat(logPath); err == nil {
			return filepath.Abs(logPath)
		}
	}

	return "", fmt.Errorf("no log file found in standard locations")
}

// performLogAnalysis analyzes the log file and returns results.
func performLogAnalysis(
	logFile, timeRange, logLevel string,
	maxLines int64,
	errorPattern, performanceMetrics, accessPattern, systemHealth, includeStackTraces bool,
) (*LogAnalysisResults, error) {
	file, err := os.Open(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	results := &LogAnalysisResults{
		ErrorPatterns:     make(map[string]int),
		AccessPatterns:    make(map[string]int),
		PerformanceIssues: make([]string, 0),
	}

	// Compile regex patterns for log analysis
	errorRegex := regexp.MustCompile(`(?i)\b(error|failed|exception|panic)\b`)
	warningRegex := regexp.MustCompile(`(?i)\b(warning|warn)\b`)
	infoRegex := regexp.MustCompile(`(?i)\b(info|debug|trace)\b`)
	performanceRegex := regexp.MustCompile(
		`(?i)(slow|timeout|took\s+(\d+(?:\.\d+)?)(\s*m?s)|response.*?time.*?(\d+(?:\.\d+)?)(\s*m?s))`,
	)
	timingRegex := regexp.MustCompile(`(?i)took\s+(\d+(?:\.\d+)?)\s*(ms|s)`)
	operationRegex := regexp.MustCompile(`(?i)(\w+(?:_\w+)*)\s+took\s+(\d+(?:\.\d+)?)\s*(ms|s)`)
	accessRegex := regexp.MustCompile(`\b(GET|POST|PUT|DELETE|PATCH)\s+(/\S*)`)

	// More specific error pattern extraction
	errorPatternRegex := regexp.MustCompile(
		`(?i)(failed to connect|connection refused|timeout|database error|not found|permission denied|out of memory|disk full|network error)`,
	)

	// Variables to track performance metrics
	var (
		totalResponseTimeMs float64
		responseTimeCount   int
		maxResponseTimeMs   float64
		maxResponseTimeStr  string
		slowestOp           string
		slowestOpTime       float64
	)

	// Calculate time range filter
	var (
		timeRangeStart time.Time
		useTimeFilter  bool
	)

	if timeRange != "" && timeRange != "all" {
		useTimeFilter = true

		now := time.Now()
		switch timeRange {
		case "1hour":
			timeRangeStart = now.Add(-1 * time.Hour)
		case "6hours":
			timeRangeStart = now.Add(-6 * time.Hour)
		case "24hours":
			timeRangeStart = now.Add(-24 * time.Hour)
		case "7days":
			timeRangeStart = now.Add(-7 * 24 * time.Hour)
		case "30days":
			timeRangeStart = now.Add(-30 * 24 * time.Hour)
		default:
			useTimeFilter = false
		}
	}

	// Compile log level regex for filtering
	var logLevelRegex *regexp.Regexp
	if logLevel != "" && logLevel != "all" {
		switch strings.ToLower(logLevel) {
		case "error":
			logLevelRegex = regexp.MustCompile(`(?i)\b(error|failed|exception|panic)\b`)
		case "warning":
			logLevelRegex = regexp.MustCompile(`(?i)\b(warning|warn)\b`)
		case "info":
			logLevelRegex = regexp.MustCompile(`(?i)\b(info)\b`)
		case "debug":
			logLevelRegex = regexp.MustCompile(`(?i)\b(debug|trace)\b`)
		}
	}

	// Common log timestamp patterns
	timestampRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2})`)

	scanner := bufio.NewScanner(file)
	lineCount := int64(0)

	for scanner.Scan() && lineCount < maxLines {
		line := scanner.Text()
		lineCount++

		// Apply time range filter if specified
		if useTimeFilter {
			if timestampMatches := timestampRegex.FindStringSubmatch(line); len(
				timestampMatches,
			) > 1 {
				timestampStr := timestampMatches[1]
				// Try multiple timestamp formats
				var (
					logTime time.Time
					err     error
				)

				layouts := []string{
					"2006-01-02T15:04:05",
					"2006-01-02 15:04:05",
					"2006-01-02T15:04:05Z",
					"2006-01-02 15:04:05.000",
				}

				for _, layout := range layouts {
					if logTime, err = time.Parse(layout, timestampStr); err == nil {
						break
					}
				}

				if err == nil && logTime.Before(timeRangeStart) {
					continue // Skip this line as it's outside the time range
				}
			}
		}

		// Apply log level filter if specified
		if logLevelRegex != nil && !logLevelRegex.MatchString(line) {
			continue // Skip this line as it doesn't match the specified log level
		}

		results.TotalEntries++

		// Count log levels
		if errorRegex.MatchString(line) {
			results.ErrorCount++
			if errorPattern {
				// Extract specific error patterns
				matches := errorPatternRegex.FindAllString(line, -1)
				for _, match := range matches {
					results.ErrorPatterns[match]++
				}
			}
		} else if warningRegex.MatchString(line) {
			results.WarningCount++
		} else if infoRegex.MatchString(line) {
			results.InfoCount++
		}

		// Analyze performance metrics
		if performanceMetrics {
			// Extract timing information
			if timingMatches := timingRegex.FindAllStringSubmatch(line, -1); len(
				timingMatches,
			) > 0 {
				for _, match := range timingMatches {
					if len(match) >= 3 {
						timeValue, err := strconv.ParseFloat(match[1], 64)
						if err != nil {
							continue
						}

						timeUnit := strings.ToLower(match[2])

						timeInMs := timeValue
						if timeUnit == "s" {
							timeInMs = timeValue * 1000
						}

						totalResponseTimeMs += timeInMs
						responseTimeCount++

						if timeInMs > maxResponseTimeMs {
							maxResponseTimeMs = timeInMs
							maxResponseTimeStr = fmt.Sprintf("%.2f%s", timeValue, timeUnit)
						}
					}
				}
			}

			// Extract operation timing
			if opMatches := operationRegex.FindAllStringSubmatch(line, -1); len(opMatches) > 0 {
				for _, match := range opMatches {
					if len(match) >= 4 {
						operation := match[1]

						timeValue, err := strconv.ParseFloat(match[2], 64)
						if err != nil {
							continue
						}

						timeUnit := strings.ToLower(match[3])

						timeInMs := timeValue
						if timeUnit == "s" {
							timeInMs = timeValue * 1000
						}

						if timeInMs > slowestOpTime {
							slowestOpTime = timeInMs
							slowestOp = fmt.Sprintf("%s (%.2f%s)", operation, timeValue, timeUnit)
						}
					}
				}
			}

			// Add to performance issues list for backward compatibility
			if performanceRegex.MatchString(line) {
				matches := performanceRegex.FindAllString(line, -1)

				results.PerformanceIssues = append(results.PerformanceIssues, matches...)
			}
		}

		// Analyze access patterns
		if accessPattern && accessRegex.MatchString(line) {
			matches := accessRegex.FindAllStringSubmatch(line, -1)
			for _, match := range matches {
				if len(match) > 2 {
					endpoint := match[2] // The captured endpoint group
					results.AccessPatterns[endpoint]++
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %w", err)
	}

	// Find the top error pattern
	var maxErrorCount int
	for pattern, count := range results.ErrorPatterns {
		if count > maxErrorCount {
			maxErrorCount = count
			results.TopErrorPattern = fmt.Sprintf("%s (%d occurrences)", pattern, count)
		}
	}

	// Find the top access pattern
	var maxAccessCount int
	for endpoint, count := range results.AccessPatterns {
		if count > maxAccessCount {
			maxAccessCount = count
			results.TopAccessPattern = fmt.Sprintf("%s (%d requests)", endpoint, count)
		}
	}

	// Calculate performance metrics
	if responseTimeCount > 0 {
		avgTimeMs := totalResponseTimeMs / float64(responseTimeCount)

		results.AvgResponseTime = fmt.Sprintf("%.2fms", avgTimeMs)
	} else {
		results.AvgResponseTime = "No timing data found"
	}

	if maxResponseTimeStr != "" {
		results.MaxResponseTime = maxResponseTimeStr
	} else {
		results.MaxResponseTime = "No timing data found"
	}

	if slowestOp != "" {
		results.SlowestOperation = slowestOp
	} else {
		results.SlowestOperation = "No operation timing found"
	}

	return results, nil
}

package api

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// readLogFiles reads and parses log files from the specified cutoff time
func readLogFiles(cutoffTime time.Time, logLevel string, maxLines int64) ([]LogEntry, error) {
	var entries []LogEntry

	// Get log file path
	logFile, err := getLogFilePath()
	if err != nil {
		return nil, fmt.Errorf("failed to locate log file: %v", err)
	}

	file, err := os.Open(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %v", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := int64(0)

	// Regex patterns for log parsing
	timestampRegex := regexp.MustCompile(`(\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2})`)
	levelRegex := regexp.MustCompile(`\b(DEBUG|INFO|WARN|WARNING|ERROR|FATAL)\b`)

	for scanner.Scan() && lineCount < maxLines {
		line := scanner.Text()
		lineCount++

		// Parse log entry
		entry := parseLogLine(line, timestampRegex, levelRegex)
		if entry == nil {
			continue
		}

		// Filter by cutoff time
		if entry.Timestamp.Before(cutoffTime) {
			continue
		}

		// Filter by log level if specified
		if logLevel != "" && !strings.EqualFold(entry.Level, logLevel) && logLevel != "all" {
			continue
		}

		entries = append(entries, *entry)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading log file: %v", err)
	}

	// Sort entries by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Timestamp.After(entries[j].Timestamp)
	})

	return entries, nil
}

// parseLogLine parses a single log line into a LogEntry
func parseLogLine(line string, timestampRegex, levelRegex *regexp.Regexp) *LogEntry {
	// Extract timestamp
	timestampMatch := timestampRegex.FindString(line)
	if timestampMatch == "" {
		return nil
	}

	// Parse timestamp
	var timestamp time.Time
	var err error

	// Try different timestamp formats
	formats := []string{
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006/01/02 15:04:05",
		"Jan 02 15:04:05",
	}

	for _, format := range formats {
		timestamp, err = time.Parse(format, timestampMatch)
		if err == nil {
			break
		}
	}

	if err != nil {
		// If we can't parse timestamp, use current time
		timestamp = time.Now()
	}

	// Extract log level
	level := "INFO" // default
	if levelMatch := levelRegex.FindString(line); levelMatch != "" {
		level = strings.ToUpper(levelMatch)
	}

	// Extract message (everything after timestamp and level)
	message := line
	if idx := strings.Index(line, level); idx > 0 {
		message = strings.TrimSpace(line[idx+len(level):])
	}

	return &LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		RawLine:   line,
	}
}

// analyzeErrorPatterns analyzes log entries for error patterns
func analyzeErrorPatterns(entries []LogEntry) []ErrorPattern {
	errorCounts := make(map[string]int)
	errorExamples := make(map[string]string)

	// Regex patterns for common error types
	patterns := map[string]*regexp.Regexp{
		"Connection Error":     regexp.MustCompile(`(?i)(connection.*fail|timeout|refused|unreachable)`),
		"Database Error":       regexp.MustCompile(`(?i)(database|sql|query).*error`),
		"File System Error":    regexp.MustCompile(`(?i)(file.*not.*found|permission.*denied|disk.*full|no.*space)`),
		"Authentication Error": regexp.MustCompile(`(?i)(auth.*fail|unauthorized|forbidden|invalid.*token)`),
		"Network Error":        regexp.MustCompile(`(?i)(network.*error|dns.*error|host.*not.*found)`),
		"Parse Error":          regexp.MustCompile(`(?i)(parse.*error|invalid.*format|malformed)`),
		"Configuration Error":  regexp.MustCompile(`(?i)(config.*error|missing.*setting|invalid.*config)`),
	}

	for _, entry := range entries {
		if entry.Level == "ERROR" || entry.Level == "FATAL" {
			// Check against known patterns
			matched := false
			for patternName, regex := range patterns {
				if regex.MatchString(entry.Message) {
					errorCounts[patternName]++
					if errorExamples[patternName] == "" {
						errorExamples[patternName] = entry.Message
					}
					matched = true
					break
				}
			}

			// If no pattern matched, categorize as "Other Error"
			if !matched {
				errorCounts["Other Error"]++
				if errorExamples["Other Error"] == "" {
					errorExamples["Other Error"] = entry.Message
				}
			}
		}
	}

	// Convert to slice and sort by count
	var patterns_result []ErrorPattern
	for pattern, count := range errorCounts {
		patterns_result = append(patterns_result, ErrorPattern{
			Pattern:  pattern,
			Count:    count,
			Example:  errorExamples[pattern],
			LastSeen: time.Now(), // In real implementation, track actual last seen time
			Severity: determineSeverity(pattern, errorExamples[pattern]),
		})
	}

	sort.Slice(patterns_result, func(i, j int) bool {
		return patterns_result[i].Count > patterns_result[j].Count
	})

	return patterns_result
}

// analyzePerformanceMetrics analyzes log entries for performance data
func analyzePerformanceMetrics(entries []LogEntry) []PerformanceMetric {
	var metrics []PerformanceMetric

	// Regex patterns for performance metrics
	durationRegex := regexp.MustCompile(`(?i)(took|duration|elapsed).*?(\d+(?:\.\d+)?)\s*(ms|milliseconds|s|seconds)`)
	responseTimeRegex := regexp.MustCompile(`(?i)(response.*time|latency).*?(\d+(?:\.\d+)?)\s*(ms|milliseconds|s|seconds)`)

	responseTimes := make([]float64, 0)
	slowQueries := 0

	for _, entry := range entries {
		// Look for duration/timing information
		if matches := durationRegex.FindStringSubmatch(entry.Message); matches != nil {
			if duration, err := strconv.ParseFloat(matches[2], 64); err == nil {
				// Convert to milliseconds
				if strings.Contains(matches[3], "s") && !strings.Contains(matches[3], "ms") {
					duration *= 1000
				}
				responseTimes = append(responseTimes, duration)

				// Flag slow operations (>1000ms)
				if duration > 1000 {
					slowQueries++
				}
			}
		}

		// Look for response time information
		if matches := responseTimeRegex.FindStringSubmatch(entry.Message); matches != nil {
			if responseTime, err := strconv.ParseFloat(matches[2], 64); err == nil {
				// Convert to milliseconds
				if strings.Contains(matches[3], "s") && !strings.Contains(matches[3], "ms") {
					responseTime *= 1000
				}
				responseTimes = append(responseTimes, responseTime)
			}
		}
	}

	// Calculate statistics
	if len(responseTimes) > 0 {
		avg := calculateAverage(responseTimes)
		min, max := calculateMinMax(responseTimes)
		p95 := calculatePercentile(responseTimes, 95)

		metrics = append(metrics, PerformanceMetric{
			Name:        "Average Response Time",
			Value:       avg,
			Unit:        "ms",
			Description: "Average response time across all operations",
		})

		metrics = append(metrics, PerformanceMetric{
			Name:        "Min Response Time",
			Value:       min,
			Unit:        "ms",
			Description: "Fastest response time recorded",
		})

		metrics = append(metrics, PerformanceMetric{
			Name:        "Max Response Time",
			Value:       max,
			Unit:        "ms",
			Description: "Slowest response time recorded",
		})

		metrics = append(metrics, PerformanceMetric{
			Name:        "95th Percentile",
			Value:       p95,
			Unit:        "ms",
			Description: "95% of requests completed within this time",
		})

		metrics = append(metrics, PerformanceMetric{
			Name:        "Slow Operations",
			Value:       float64(slowQueries),
			Unit:        "count",
			Description: "Number of operations taking >1000ms",
		})
	}

	return metrics
}

// analyzeAccessPatterns analyzes log entries for access pattern data
func analyzeAccessPatterns(entries []LogEntry) []AccessPattern {
	var patterns []AccessPattern

	httpMethodCounts := make(map[string]int)
	endpointCounts := make(map[string]int)
	ipCounts := make(map[string]int)

	// Regex patterns for access log parsing
	httpRegex := regexp.MustCompile(`\b(GET|POST|PUT|DELETE|PATCH|HEAD|OPTIONS)\s+([^\s]+)`)
	ipRegex := regexp.MustCompile(`\b(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\b`)

	for _, entry := range entries {
		// Analyze HTTP methods and endpoints
		if matches := httpRegex.FindStringSubmatch(entry.Message); matches != nil {
			method := matches[1]
			endpoint := matches[2]

			httpMethodCounts[method]++
			endpointCounts[endpoint]++
		}

		// Analyze IP addresses
		if matches := ipRegex.FindAllString(entry.Message, -1); matches != nil {
			for _, ip := range matches {
				ipCounts[ip]++
			}
		}
	}

	// Convert to patterns
	for method, count := range httpMethodCounts {
		patterns = append(patterns, AccessPattern{
			Pattern:     fmt.Sprintf("HTTP %s requests", method),
			Count:       count,
			Description: fmt.Sprintf("Number of %s HTTP requests", method),
		})
	}

	// Get top endpoints
	type endpointCount struct {
		endpoint string
		count    int
	}
	var endpoints []endpointCount
	for endpoint, count := range endpointCounts {
		endpoints = append(endpoints, endpointCount{endpoint, count})
	}
	sort.Slice(endpoints, func(i, j int) bool {
		return endpoints[i].count > endpoints[j].count
	})

	// Add top 10 endpoints
	for i, ep := range endpoints {
		if i >= 10 {
			break
		}
		patterns = append(patterns, AccessPattern{
			Pattern:     fmt.Sprintf("Endpoint: %s", ep.endpoint),
			Count:       ep.count,
			Description: fmt.Sprintf("Requests to %s endpoint", ep.endpoint),
		})
	}

	return patterns
}

// analyzeSystemHealth analyzes log entries for system health indicators
func analyzeSystemHealth(entries []LogEntry) []SystemHealthIndicator {
	var indicators []SystemHealthIndicator

	errorCount := 0
	warningCount := 0
	infoCount := 0

	memoryIssues := 0
	diskIssues := 0
	networkIssues := 0

	// Regex patterns for system health
	memoryRegex := regexp.MustCompile(`(?i)(out.*of.*memory|memory.*full|heap.*space)`)
	diskRegex := regexp.MustCompile(`(?i)(disk.*full|no.*space|storage.*full)`)
	networkRegex := regexp.MustCompile(`(?i)(network.*down|connection.*lost|timeout)`)

	for _, entry := range entries {
		// Count log levels
		switch entry.Level {
		case "ERROR", "FATAL":
			errorCount++
		case "WARN", "WARNING":
			warningCount++
		case "INFO", "DEBUG":
			infoCount++
		}

		// Check for specific issues
		if memoryRegex.MatchString(entry.Message) {
			memoryIssues++
		}
		if diskRegex.MatchString(entry.Message) {
			diskIssues++
		}
		if networkRegex.MatchString(entry.Message) {
			networkIssues++
		}
	}

	total := errorCount + warningCount + infoCount

	indicators = append(indicators, SystemHealthIndicator{
		Name:        "Error Rate",
		Value:       float64(errorCount),
		Threshold:   10.0,
		Status:      getHealthStatus(float64(errorCount), 10.0),
		Description: fmt.Sprintf("%d errors out of %d total log entries", errorCount, total),
	})

	indicators = append(indicators, SystemHealthIndicator{
		Name:        "Warning Rate",
		Value:       float64(warningCount),
		Threshold:   20.0,
		Status:      getHealthStatus(float64(warningCount), 20.0),
		Description: fmt.Sprintf("%d warnings out of %d total log entries", warningCount, total),
	})

	if memoryIssues > 0 {
		indicators = append(indicators, SystemHealthIndicator{
			Name:        "Memory Issues",
			Value:       float64(memoryIssues),
			Threshold:   1.0,
			Status:      "critical",
			Description: fmt.Sprintf("%d memory-related issues detected", memoryIssues),
		})
	}

	if diskIssues > 0 {
		indicators = append(indicators, SystemHealthIndicator{
			Name:        "Disk Issues",
			Value:       float64(diskIssues),
			Threshold:   1.0,
			Status:      "critical",
			Description: fmt.Sprintf("%d disk-related issues detected", diskIssues),
		})
	}

	if networkIssues > 0 {
		indicators = append(indicators, SystemHealthIndicator{
			Name:        "Network Issues",
			Value:       float64(networkIssues),
			Threshold:   5.0,
			Status:      getHealthStatus(float64(networkIssues), 5.0),
			Description: fmt.Sprintf("%d network-related issues detected", networkIssues),
		})
	}

	return indicators
}

// Helper functions for statistics
func calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func calculateMinMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}

	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

func calculatePercentile(values []float64, percentile float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sortedValues := make([]float64, len(values))
	copy(sortedValues, values)
	sort.Float64s(sortedValues)

	index := int(float64(len(sortedValues)) * percentile / 100.0)
	if index >= len(sortedValues) {
		index = len(sortedValues) - 1
	}

	return sortedValues[index]
}

func getHealthStatus(value, threshold float64) string {
	if value >= threshold*2 {
		return "critical"
	} else if value >= threshold {
		return "warning"
	}
	return "healthy"
}

// Data structures for log analysis
type LogEntry struct {
	Timestamp time.Time
	Level     string
	Message   string
	RawLine   string
}

type ErrorPattern struct {
	Pattern  string
	Count    int
	Example  string
	LastSeen time.Time
	Severity string
}

type PerformanceMetric struct {
	Name        string
	Value       float64
	Unit        string
	Description string
}

type AccessPattern struct {
	Pattern     string
	Count       int
	Description string
}

type SystemHealthIndicator struct {
	Name        string
	Value       float64
	Threshold   float64
	Status      string
	Description string
}

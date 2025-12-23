package api

import (
	"strconv"
	"strings"
	"time"
)

// parseTimeRange parses time range strings and returns appropriate time.
func parseTimeRange(timeRange string) time.Time {
	switch strings.ToLower(timeRange) {
	case "1h", "1 hour":
		return time.Now().Add(-time.Hour)
	case "6h", "6 hours":
		return time.Now().Add(-6 * time.Hour)
	case "12h", "12 hours":
		return time.Now().Add(-12 * time.Hour)
	case "24h", "1d", "1 day":
		return time.Now().Add(-24 * time.Hour)
	case "7d", "1w", "1 week":
		return time.Now().Add(-7 * 24 * time.Hour)
	case "30d", "1m", "1 month":
		return time.Now().Add(-30 * 24 * time.Hour)
	case "90d", "3m", "3 months":
		return time.Now().Add(-90 * 24 * time.Hour)
	default:
		// Try to parse as number of days
		if days, err := strconv.Atoi(strings.TrimSuffix(timeRange, "d")); err == nil {
			return time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		}
		// Default to 24 hours
		return time.Now().Add(-24 * time.Hour)
	}
}

// ScheduleExecutionResult holds the result of a scheduled task execution.
type ScheduleExecutionResult struct {
	ScheduleName string
	TaskType     string
	StartTime    time.Time
	Duration     time.Duration
	Status       string // "running", "completed", "failed", "skipped"
	Error        string
	Details      []string
}

// ScheduleInfo holds information about a scheduled task.
type ScheduleInfo struct {
	Name           string
	Description    string
	TaskType       string
	Frequency      string
	CronExpression string
	Enabled        bool
	LastRun        *time.Time
	NextRun        *time.Time
	Status         string
}

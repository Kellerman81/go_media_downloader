package api

import (
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
)

type CronValidationRequest struct {
	Expression string `json:"expression"`
}

// CronValidationResponse represents the response for cron validation.
type CronValidationResponse struct {
	Valid       bool     `json:"valid"`
	Description string   `json:"description"`
	NextRuns    []string `json:"next_runs"`
	Error       string   `json:"error"`
}

// HandleCronValidation validates a cron expression and provides description.
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
	parser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow,
	)

	schedule, err := parser.Parse(expression)
	if err != nil {
		// Try 5-field format if 6-field fails
		parser5 := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

		schedule5, err5 := parser5.Parse(expression)
		if err5 != nil {
			// err.Error() embeds robfig/cron's own error text, which can echo
			// back the literal invalid expression fields - escape it before
			// interpolating into HTML that's swapped into the page verbatim
			// via HTMX (management_pages.go), or a cron expression containing
			// e.g. "<img src=x onerror=...>" executes in the admin's browser.
			ctx.Header("Content-Type", "text/html")
			ctx.String(http.StatusOK, `<div class="alert alert-danger mb-0">
				<i class="fa-solid fa-times-circle me-2"></i>
				<strong>Invalid Expression</strong> - %s
				<br><small class="text-muted mt-1 d-block">Supports both 5-field (minute hour day month weekday) and 6-field (second minute hour day month weekday) formats.</small>
			</div>`, html.EscapeString(err.Error()))

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
	for range 5 {
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
		fmt.Fprintf(&html, `
				<li class="d-flex align-items-center mb-2">
					<span class="badge bg-primary me-3">%d</span>
					<span class="font-monospace">%s</span>
				</li>`, i+1, runTime)
	}

	html.WriteString(`
			</ul>
		</div>
	</div>`)

	ctx.String(http.StatusOK, html.String())
}

// generateCronDescription creates a human-readable description of a cron expression.
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
	if second == "*" && minute == "*" && hour == "*" && day == "*" && month == "*" &&
		weekday == "*" {
		desc.WriteString("every second")
	} else if minute == "*" && hour == "*" && day == "*" && month == "*" && weekday == "*" {
		if after, ok := strings.CutPrefix(second, "*/"); ok {
			interval := after
			fmt.Fprintf(&desc, "every %s seconds", interval)
		} else if second == "0" || len(parts) == 5 {
			desc.WriteString("every minute")
		} else {
			fmt.Fprintf(&desc, "at second %s of every minute", second)
		}
	} else if hour == "*" && day == "*" && month == "*" && weekday == "*" {
		if after, ok := strings.CutPrefix(minute, "*/"); ok {
			interval := after
			fmt.Fprintf(&desc, "every %s minutes", interval)
		} else if minute == "0" {
			desc.WriteString("every hour")
		} else {
			timeStr := formatTimeWithSeconds(hour, minute, second)
			fmt.Fprintf(&desc, "at %s of every hour", timeStr)
		}
	} else if day == "*" && month == "*" && weekday == "*" {
		// Daily
		timeStr := formatTimeWithSeconds(hour, minute, second)
		fmt.Fprintf(&desc, "daily at %s", timeStr)
	} else if month == "*" && weekday == "*" {
		// Monthly
		timeStr := formatTimeWithSeconds(hour, minute, second)
		switch day {
		case "1":
			{
				fmt.Fprintf(&desc, "monthly on the 1st at %s", timeStr)
			}

		case "15":
			{
				fmt.Fprintf(&desc, "monthly on the 15th at %s", timeStr)
			}

		default:
			{
				fmt.Fprintf(&desc, "monthly on day %s at %s", day, timeStr)
			}
		}
	} else if month == "*" && day == "*" {
		// Weekly
		timeStr := formatTimeWithSeconds(hour, minute, second)
		weekdayName := getWeekdayName(weekday)
		fmt.Fprintf(&desc, "weekly on %s at %s", weekdayName, timeStr)
	} else {
		// Complex schedule
		desc.WriteString("on a complex schedule")

		if len(parts) == 6 && second != "*" && second != "0" {
			fmt.Fprintf(&desc, " (second: %s", second)
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

			fmt.Fprintf(&desc, "minute: %s", minute)

			needComma = true
		}

		if hour != "*" {
			if needComma {
				desc.WriteString(", ")
			}

			fmt.Fprintf(&desc, "hour: %s", hour)

			needComma = true
		}

		if day != "*" {
			if needComma {
				desc.WriteString(", ")
			}

			fmt.Fprintf(&desc, "day: %s", day)

			needComma = true
		}

		if month != "*" {
			if needComma {
				desc.WriteString(", ")
			}

			fmt.Fprintf(&desc, "month: %s", month)

			needComma = true
		}

		if weekday != "*" {
			if needComma {
				desc.WriteString(", ")
			}

			fmt.Fprintf(&desc, "weekday: %s", weekday)
		}

		desc.WriteString(")")
	}

	return desc.String()
}

// formatTimeWithSeconds formats hour, minute, and second parts into readable time.
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

// getWeekdayName converts weekday number to name.
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

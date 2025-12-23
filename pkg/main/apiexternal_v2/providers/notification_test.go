package providers_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/apprise"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/gotify"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushbullet"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/pushover"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sendmail"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// loadPushoverConfig loads Pushover settings from config.toml
func loadPushoverConfig(t *testing.T) (apiToken, userKey string) {
	// Set config file path if not already set
	if config.Configfile == "" || config.Configfile == "./config/config.toml" {
		config.Configfile = "R:\\golang_ent\\config\\config.toml"
	}

	// Read config file
	cfg, err := config.Readconfigtoml()
	if err != nil {
		t.Fatalf("Failed to read config.toml: %v", err)
	}

	// Find Pushover notification configuration
	for _, notif := range cfg.Notification {
		if notif.NotificationType == "pushover" && notif.Name == "pushover" {
			apiToken = notif.Apikey
			userKey = notif.Recipient
			break
		}
	}

	// Validate that required keys are present
	if apiToken == "" || userKey == "" {
		t.Skip("Pushover credentials not configured in config.toml - skipping Pushover test")
	}

	return
}

//
// Notification Provider Tests
//
// These tests will attempt to actually send notifications to your notification services.
//
// To run a specific test:
//   go test -v -run TestGotifyNotification ./pkg/main/apiexternal_v2/providers
//

// TestGotifyNotification tests Gotify notification sending
func TestGotifyNotification(t *testing.T) {
	//	t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 8070
	token := ""
	useSSL := false
	// ========================================

	config := base.ClientConfig{
		Timeout:                 30 * time.Second,
		CircuitBreakerThreshold: 5,
		CircuitBreakerTimeout:   60 * time.Second,
		EnableStats:             true,
		StatsDBTable:            "api_client_stats",
		MaxRetries:              3,
		RetryBackoff:            2 * time.Second,
	}

	provider := gotify.NewProviderWithConfig(config, host, port, token, useSSL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Sending test notification to Gotify...")
	request := apiexternal_v2.NotificationRequest{
		Title:    "Test Notification",
		Message:  "This is a test notification from the apiexternal_v2 test suite",
		Priority: 5,
	}

	response, err := provider.SendNotification(ctx, request)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	if !response.Success {
		t.Fatalf("Notification failed: %s", response.Error)
	}

	t.Logf("✓ Notification sent successfully")
	t.Logf("  Message ID: %s", response.MessageID)
	t.Logf("  Provider: %s", response.Provider)

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestPushoverConfigLoad verifies Pushover config can be loaded from config.toml
func TestPushoverConfigLoad(t *testing.T) {
	// Load Pushover credentials from config.toml
	apiToken, userKey := loadPushoverConfig(t)

	// Verify credentials were loaded
	if apiToken == "" {
		t.Error("Pushover API token is empty")
	}
	if userKey == "" {
		t.Error("Pushover user key is empty")
	}

	t.Logf("✓ Successfully loaded Pushover configuration")
	t.Logf("  API Token: %s...%s (length: %d)", apiToken[:6], apiToken[len(apiToken)-4:], len(apiToken))
	t.Logf("  User Key: %s...%s (length: %d)", userKey[:6], userKey[len(userKey)-4:], len(userKey))
}

// TestPushoverNotification tests Pushover notification sending
func TestPushoverNotification(t *testing.T) {
	// t.Skip("Manual test - comment out this skip to test actual notification sending")

	// Load Pushover credentials from config.toml
	apiToken, userKey := loadPushoverConfig(t)

	config := base.ClientConfig{
		Name:                      "pushover",
		BaseURL:                   "https://api.pushover.net/1",
		Timeout:                   30 * time.Second,
		AuthType:                  base.AuthNone, // Pushover handles auth via form parameters
		RateLimitCalls:            300,           // Pushover allows ~10,000/month = ~300/hour conservative
		RateLimitSeconds:          3600,          // 1 hour
		CircuitBreakerThreshold:   3,
		CircuitBreakerTimeout:     30 * time.Second,
		CircuitBreakerHalfOpenMax: 1,
		EnableStats:               true,
		StatsDBTable:              "api_client_stats",
		UserAgent:                 "go-media-downloader/2.0",
		MaxRetries:                3,
		RetryBackoff:              2 * time.Second,
	}

	provider := pushover.NewProviderWithConfig(config, apiToken, userKey)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Sending test notification to Pushover...")
	request := apiexternal_v2.NotificationRequest{
		Title:    "Test Notification",
		Message:  "This is a test notification from the apiexternal_v2 test suite",
		Priority: 0,
	}

	response, err := provider.SendNotification(ctx, request)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	if !response.Success {
		t.Fatalf("Notification failed: %s", response.Error)
	}

	t.Logf("✓ Notification sent successfully")
	t.Logf("  Message ID: %s", response.MessageID)
	t.Logf("  Provider: %s", response.Provider)

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestPushbulletNotification tests Pushbullet notification sending
func TestPushbulletNotification(t *testing.T) {
	t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	apiToken := "your-pushbullet-access-token"
	// ========================================

	provider := pushbullet.NewProvider(apiToken)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Sending test notification to Pushbullet...")
	request := apiexternal_v2.NotificationRequest{
		Title:   "Test Notification",
		Message: "This is a test notification from the apiexternal_v2 test suite",
	}

	response, err := provider.SendNotification(ctx, request)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	if !response.Success {
		t.Fatalf("Notification failed: %s", response.Error)
	}

	t.Logf("✓ Notification sent successfully")
	t.Logf("  Message ID: %s", response.MessageID)
	t.Logf("  Provider: %s", response.Provider)

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestAppriseNotification tests Apprise notification sending
func TestAppriseNotification(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 8000
	token := "" // Optional token for Apprise API
	useSSL := false

	// Apprise notification URLs (e.g., "discord://webhook_id/webhook_token")
	// See https://github.com/caronc/apprise for URL formats
	urls := []string{
		"pover://user@key",
		// Add more notification service URLs as needed
	}
	// ========================================

	provider := apprise.NewProvider(host, port, token, urls, useSSL)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Sending test notification via Apprise...")
	request := apiexternal_v2.NotificationRequest{
		Title:   "Test Notification",
		Message: "This is a test notification from the apiexternal_v2 test suite",
	}

	response, err := provider.SendNotification(ctx, request)
	if err != nil {
		t.Fatalf("Failed to send notification: %v", err)
	}

	if !response.Success {
		t.Fatalf("Notification failed: %s", response.Error)
	}

	t.Logf("✓ Notification sent successfully")
	t.Logf("  Provider: %s", response.Provider)

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestSendmailNotification tests email notification sending
func TestSendmailNotification(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	smtpHost := "smtp.gmail.com"
	smtpPort := 587
	from := "@gmail.com"
	to := []string{"@gmail.com"}
	username := "@gmail.com"
	password := "" // For Gmail, use an app-specific password
	// ========================================

	provider := sendmail.NewProvider(smtpHost, smtpPort, from, to, username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Sending test email notification...")
	request := apiexternal_v2.NotificationRequest{
		Title:   "Test Email Notification",
		Message: "This is a test email notification from the apiexternal_v2 test suite.\n\nIf you received this email, the notification system is working correctly.",
	}

	response, err := provider.SendNotification(ctx, request)
	if err != nil {
		t.Fatalf("Failed to send email: %v", err)
	}

	if !response.Success {
		t.Fatalf("Email failed: %s", response.Error)
	}

	t.Logf("✓ Email sent successfully")
	t.Logf("  From: %s", from)
	t.Logf("  To: %v", to)
	t.Logf("  Provider: %s", response.Provider)

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

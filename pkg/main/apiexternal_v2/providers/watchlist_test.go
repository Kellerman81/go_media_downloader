package providers_test

import (
	"context"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/jellyfin"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/plex"
)

//
// Watchlist Provider Tests
//
// These tests will attempt to connect to your media servers and retrieve watchlist data.
//
// To run a specific test:
//   go test -v -run TestPlexWatchlist ./pkg/main/apiexternal_v2/providers
//

// TestPlexWatchlist tests Plex watchlist retrieval
func TestPlexWatchlist(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	serverURL := "https://192.168.1.59:32400" // Your Plex server URL
	token := ""                               // Your Plex authentication token
	username := ""                            // Your Plex username/email
	insecureSkipVerify := true                // Set to true to skip TLS certificate verification (for self-signed certs)
	// ========================================

	// How to get your Plex token:
	// 1. Log into Plex web interface
	// 2. Play any media item
	// 3. Click the ... menu → Get Info
	// 4. Look at the URL - the token is the X-Plex-Token parameter
	// OR
	// 1. Visit: https://support.plex.tv/articles/204059436-finding-an-authentication-token-x-plex-token/

	provider := plex.NewProvider(serverURL, token, insecureSkipVerify)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Connecting to Plex server...")
	t.Logf("  Server URL: %s", serverURL)

	t.Log("Retrieving watchlist...")
	watchlist, err := provider.GetWatchlist(ctx, username)
	if err != nil {
		t.Fatalf("Failed to get watchlist: %v", err)
	}

	t.Logf("✓ Watchlist retrieved successfully")
	t.Logf("  Total items: %d", len(watchlist))

	if len(watchlist) > 0 {
		t.Log("\nWatchlist items (showing first 5):")
		maxItems := 5
		if len(watchlist) < maxItems {
			maxItems = len(watchlist)
		}

		for i := 0; i < maxItems; i++ {
			item := watchlist[i]
			t.Logf("  [%d] %s (%d)", i+1, item.Title, item.Year)
			t.Logf("      Type: %s", item.Type)
			if item.IMDbID != "" {
				t.Logf("      IMDb: %s", item.IMDbID)
			}
			if item.TVDbID != 0 {
				t.Logf("      TVDb: %d", item.TVDbID)
			}
		}

		if len(watchlist) > maxItems {
			t.Logf("  ... and %d more items", len(watchlist)-maxItems)
		}
	} else {
		t.Log("  Watchlist is empty")
	}

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestJellyfinWatchlist tests Jellyfin watchlist retrieval
func TestJellyfinWatchlist(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	serverURL := "http://192.168.1.59:8096" // Your Jellyfin server URL
	token := ""                             // Your Jellyfin API key
	userID := ""                            // Your Jellyfin user ID
	username := ""                          // Your Jellyfin username
	// ========================================

	// How to get your Jellyfin API key:
	// 1. Log into Jellyfin web interface
	// 2. Go to Dashboard → API Keys
	// 3. Create a new API key
	//
	// How to get your User ID:
	// 1. Log into Jellyfin web interface
	// 2. Go to Dashboard → Users
	// 3. Click on your username
	// 4. The URL will show your user ID

	provider := jellyfin.NewProvider(serverURL, token, userID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Log("Connecting to Jellyfin server...")
	t.Logf("  Server URL: %s", serverURL)
	t.Logf("  User ID: %s", userID)

	t.Log("Retrieving watchlist...")
	watchlist, err := provider.GetWatchlist(ctx, username)
	if err != nil {
		t.Fatalf("Failed to get watchlist: %v", err)
	}

	t.Logf("✓ Watchlist retrieved successfully")
	t.Logf("  Total items: %d", len(watchlist))

	if len(watchlist) > 0 {
		t.Log("\nWatchlist items (showing first 5):")
		maxItems := 5
		if len(watchlist) < maxItems {
			maxItems = len(watchlist)
		}

		for i := 0; i < maxItems; i++ {
			item := watchlist[i]
			t.Logf("  [%d] %s (%d)", i+1, item.Title, item.Year)
			t.Logf("      Type: %s", item.Type)
			if item.IMDbID != "" {
				t.Logf("      IMDb: %s", item.IMDbID)
			}
			if item.TVDbID != 0 {
				t.Logf("      TVDb: %d", item.TVDbID)
			}
		}

		if len(watchlist) > maxItems {
			t.Logf("  ... and %d more items", len(watchlist)-maxItems)
		}
	} else {
		t.Log("  Watchlist is empty")
	}

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestPlexConnection tests basic Plex server connectivity
func TestPlexConnection(t *testing.T) {
	t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	serverURL := "http://192.168.1.59:32400"
	token := "your-plex-token"
	insecureSkipVerify := false // Set to true to skip TLS certificate verification
	// ========================================

	provider := plex.NewProvider(serverURL, token, insecureSkipVerify)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Log("Testing Plex server connection...")
	t.Logf("  Server URL: %s", serverURL)

	// Try to get watchlist with empty username to test connection
	_, err := provider.GetWatchlist(ctx, "")
	if err != nil {
		// Connection error or authentication error
		t.Logf("Connection test result: %v", err)
		t.Log("⚠️  If you see an authentication error, your token might be invalid")
		t.Log("⚠️  If you see a connection error, check server URL and network")
	} else {
		t.Log("✓ Successfully connected to Plex server")
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestJellyfinConnection tests basic Jellyfin server connectivity
func TestJellyfinConnection(t *testing.T) {
	t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	serverURL := "http://192.168.1.59:8096"
	token := "your-jellyfin-api-key"
	userID := "your-user-id"
	// ========================================

	provider := jellyfin.NewProvider(serverURL, token, userID)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	t.Log("Testing Jellyfin server connection...")
	t.Logf("  Server URL: %s", serverURL)
	t.Logf("  User ID: %s", userID)

	// Try to get watchlist with empty username to test connection
	_, err := provider.GetWatchlist(ctx, "")
	if err != nil {
		// Connection error or authentication error
		t.Logf("Connection test result: %v", err)
		t.Log("⚠️  If you see an authentication error, your API key might be invalid")
		t.Log("⚠️  If you see a connection error, check server URL and network")
	} else {
		t.Log("✓ Successfully connected to Jellyfin server")
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

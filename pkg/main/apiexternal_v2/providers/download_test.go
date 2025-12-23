package providers

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/deluge"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/qbittorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/rtorrent"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/sabnzbd"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/transmission"
)

//
// Manual Test Functions for Download Clients
//
// IMPORTANT: Edit the configuration parameters in each test function before running!
// These tests will attempt to actually submit downloads to your download clients.
//
// To run a specific test:
//   go test -v -run TestQBittorrentDownload ./pkg/main/apiexternal_v2/providers
//

// TestQBittorrentDownload tests qBittorrent download submission
func TestQBittorrentDownload(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 8080
	username := "admin"
	password := ""
	useSSL := false

	// Test torrent - Ubuntu ISO magnet link (legal, public domain)
	magnetURL := "magnet:?xt=urn:btih:e2467cbf021192c241367b892230dc1e05c0580e&dn=ubuntu-22.04.3-desktop-amd64.iso"
	savePath := "/downloads/test"
	category := "test"
	// ========================================

	provider, err := qbittorrent.NewProvider(host, port, username, password, useSSL)
	if err != nil {
		t.Fatalf("Failed to create qBittorrent provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection first
	t.Log("Testing qBittorrent connection...")
	if err := provider.TestConnection(ctx); err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}
	t.Log("✓ Connection successful")

	// Add torrent
	t.Logf("Adding torrent to qBittorrent...")
	request := apiexternal_v2.TorrentAddRequest{
		URL:      magnetURL,
		SavePath: savePath,
		Category: category,
		Paused:   false,
	}

	response, err := provider.AddTorrent(ctx, request)
	if err != nil {
		t.Fatalf("Failed to add torrent: %v", err)
	}

	t.Logf("✓ Torrent added successfully")
	t.Logf("  Hash: %s", response.Hash)
	t.Logf("  Name: %s", response.Name)
	t.Logf("  Message: %s", response.Message)

	// Get torrent info
	if response.Hash != "" {
		time.Sleep(2 * time.Second) // Wait for torrent to be processed
		t.Logf("Getting torrent info...")
		info, err := provider.GetTorrentInfo(ctx, response.Hash)
		if err != nil {
			t.Logf("Warning: Failed to get torrent info: %v", err)
		} else {
			t.Logf("✓ Torrent info retrieved")
			t.Logf("  Name: %s", info.Name)
			t.Logf("  State: %s", info.State)
			t.Logf("  Size: %d bytes", info.Size)
			t.Logf("  Progress: %.2f%%", info.Progress)
		}

		// Clean up - remove the test torrent
		t.Logf("Cleaning up - removing test torrent...")
		if err := provider.RemoveTorrent(ctx, response.Hash, true); err != nil {
			t.Logf("Warning: Failed to remove torrent: %v", err)
		} else {
			t.Logf("✓ Test torrent removed")
		}
	}

	// Print stats
	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestDelugeDownload tests Deluge download submission
func TestDelugeDownload(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 8112
	password := "deluge"
	useSSL := false

	// Test torrent
	magnetURL := "magnet:?xt=urn:btih:e2467cbf021192c241367b892230dc1e05c0580e&dn=ubuntu-22.04.3-desktop-amd64.iso"
	savePath := "/downloads/test"
	// ========================================

	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s:%d", scheme, host, port)

	provider := deluge.NewProvider(baseURL, "", password)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	t.Log("Testing Deluge connection...")
	if err := provider.TestConnection(ctx); err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}
	t.Log("✓ Connection successful")

	// Add torrent
	t.Logf("Adding torrent to Deluge...")
	request := apiexternal_v2.TorrentAddRequest{
		URL:      magnetURL,
		SavePath: savePath,
		Paused:   false,
	}

	response, err := provider.AddTorrent(ctx, request)
	if err != nil {
		t.Fatalf("Failed to add torrent: %v", err)
	}

	t.Logf("✓ Torrent added successfully")
	t.Logf("  Hash: %s", response.Hash)
	t.Logf("  Message: %s", response.Message)

	// Get torrent info
	if response.Hash != "" {
		time.Sleep(2 * time.Second)
		t.Logf("Getting torrent info...")
		info, err := provider.GetTorrentInfo(ctx, response.Hash)
		if err != nil {
			t.Logf("Warning: Failed to get torrent info: %v", err)
		} else {
			t.Logf("✓ Torrent info retrieved")
			t.Logf("  Name: %s", info.Name)
			t.Logf("  State: %s", info.State)
			t.Logf("  Progress: %.2f%%", info.Progress)
		}

		// Clean up
		t.Logf("Cleaning up - removing test torrent...")
		if err := provider.RemoveTorrent(ctx, response.Hash, true); err != nil {
			t.Logf("Warning: Failed to remove torrent: %v", err)
		} else {
			t.Logf("✓ Test torrent removed")
		}
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestTransmissionDownload tests Transmission download submission
func TestTransmissionDownload(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 9097
	username := "admin"
	password := ""
	useSSL := false

	// Test torrent
	magnetURL := "magnet:?xt=urn:btih:e2467cbf021192c241367b892230dc1e05c0580e&dn=ubuntu-22.04.3-desktop-amd64.iso"
	savePath := "/downloads/test"
	// ========================================

	scheme := "http"
	if useSSL {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s:%d/transmission/rpc", scheme, host, port)

	provider := transmission.NewProvider(baseURL, username, password)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	t.Log("Testing Transmission connection...")
	if err := provider.TestConnection(ctx); err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}
	t.Log("✓ Connection successful")

	// Add torrent
	t.Logf("Adding torrent to Transmission...")
	request := apiexternal_v2.TorrentAddRequest{
		URL:      magnetURL,
		SavePath: savePath,
		Paused:   false,
	}

	response, err := provider.AddTorrent(ctx, request)
	if err != nil {
		t.Fatalf("Failed to add torrent: %v", err)
	}

	t.Logf("✓ Torrent added successfully")
	t.Logf("  Hash: %s", response.Hash)
	t.Logf("  Name: %s", response.Name)
	t.Logf("  Message: %s", response.Message)

	// Get torrent info
	if response.Hash != "" {
		time.Sleep(2 * time.Second)
		t.Logf("Getting torrent info...")
		info, err := provider.GetTorrentInfo(ctx, response.Hash)
		if err != nil {
			t.Logf("Warning: Failed to get torrent info: %v", err)
		} else {
			t.Logf("✓ Torrent info retrieved")
			t.Logf("  Name: %s", info.Name)
			t.Logf("  State: %s", info.State)
			t.Logf("  Progress: %.2f%%", info.Progress)
		}

		// Clean up
		t.Logf("Cleaning up - removing test torrent...")
		// if err := provider.RemoveTorrent(ctx, response.Hash, true); err != nil {
		// 	t.Logf("Warning: Failed to remove torrent: %v", err)
		// } else {
		// 	t.Logf("✓ Test torrent removed")
		// }
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestRTorrentDownload tests rTorrent download submission
func TestRTorrentDownload(t *testing.T) {
	t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "localhost"
	port := 8080
	urlBase := "/RPC2"
	username := ""
	password := ""
	useSSL := false

	// Test torrent
	magnetURL := "magnet:?xt=urn:btih:e2467cbf021192c241367b892230dc1e05c0580e&dn=ubuntu-22.04.3-desktop-amd64.iso"
	savePath := "/downloads/test"
	// ========================================

	provider, err := rtorrent.NewProvider(host, port, username, password, useSSL, urlBase)
	if err != nil {
		t.Fatalf("Failed to create rTorrent provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	t.Log("Testing rTorrent connection...")
	if err := provider.TestConnection(ctx); err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}
	t.Log("✓ Connection successful")

	// Add torrent
	t.Logf("Adding torrent to rTorrent...")
	request := apiexternal_v2.TorrentAddRequest{
		URL:      magnetURL,
		SavePath: savePath,
		Paused:   false,
	}

	response, err := provider.AddTorrent(ctx, request)
	if err != nil {
		t.Fatalf("Failed to add torrent: %v", err)
	}

	t.Logf("✓ Torrent added successfully")
	t.Logf("  Hash: %s", response.Hash)
	t.Logf("  Message: %s", response.Message)

	// Get torrent info
	if response.Hash != "" {
		time.Sleep(2 * time.Second)
		t.Logf("Getting torrent info...")
		info, err := provider.GetTorrentInfo(ctx, response.Hash)
		if err != nil {
			t.Logf("Warning: Failed to get torrent info: %v", err)
		} else {
			t.Logf("✓ Torrent info retrieved")
			t.Logf("  Name: %s", info.Name)
			t.Logf("  State: %s", info.State)
			t.Logf("  Progress: %.2f%%", info.Progress)
		}

		// Clean up
		t.Logf("Cleaning up - removing test torrent...")
		if err := provider.RemoveTorrent(ctx, response.Hash, true); err != nil {
			t.Logf("Warning: Failed to remove torrent: %v", err)
		} else {
			t.Logf("✓ Test torrent removed")
		}
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestSABnzbdDownload tests SABnzbd download submission
func TestSABnzbdDownload(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 8075
	apiKey := ""
	useSSL := false

	// Test NZB URL (you need a valid NZB URL for testing)
	nzbURL := "https://api.nzbplanet.net/api?t=get&id=b28e773c6d8b68714e4e960906b0d2bd&apikey="
	category := "test"
	// ========================================

	provider, err := sabnzbd.NewProvider(host, port, apiKey, useSSL)
	if err != nil {
		t.Fatalf("Failed to create SABnzbd provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	t.Log("Testing SABnzbd connection...")
	if err := provider.TestConnection(ctx); err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}
	t.Log("✓ Connection successful")

	// Get status
	t.Logf("Getting SABnzbd status...")
	status, err := provider.GetStatus(ctx)
	if err != nil {
		t.Logf("Warning: Failed to get status: %v", err)
	} else {
		t.Logf("✓ Status retrieved")
		t.Logf("  Version: %s", status.Version)
		t.Logf("  Free Space: %d bytes", status.FreeSpace)
		t.Logf("  Download Speed: %d bytes/sec", status.TotalDownload)
	}

	// Add NZB
	t.Logf("Adding NZB to SABnzbd...")
	err = provider.AddNZB(ctx, nzbURL, category, 0)
	if err != nil {
		t.Fatalf("Failed to add NZB: %v", err)
	}
	t.Logf("✓ NZB added successfully")

	// List torrents
	time.Sleep(2 * time.Second)
	t.Logf("Listing downloads...")
	list, err := provider.ListTorrents(ctx, "")
	if err != nil {
		t.Logf("Warning: Failed to list downloads: %v", err)
	} else {
		t.Logf("✓ Found %d download(s)", list.Total)
		for i, torrent := range list.Torrents {
			t.Logf("  [%d] %s - %.2f%%", i+1, torrent.Name, torrent.Progress)
		}
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestNZBGetDownload tests NZBGet download submission
func TestNZBGetDownload(t *testing.T) {
	// t.Skip("Manual test - edit parameters below and remove this skip to run")

	// ========================================
	// EDIT THESE PARAMETERS
	// ========================================
	host := "192.168.1.59"
	port := 6789
	username := ""
	password := ""
	useSSL := false

	// Test NZB URL (you need a valid NZB URL for testing)
	// IMPORTANT: This must be a DIRECT download URL that returns NZB XML content
	// Examples:
	//   - NZBPlanet API: https://api.nzbplanet.net/api?t=get&id=XXXXX&apikey=YOUR_API_KEY
	//   - NZBGeek API: https://api.nzbgeek.info/api?t=get&id=XXXXX&apikey=YOUR_API_KEY
	//   - Direct NZB file URL: https://example.com/files/something.nzb
	// DO NOT use web interface URLs - they return HTML, not NZB XML
	nzbURL := "https://api.nzbplanet.net/api?t=get&id=b28e773c6d8b68714e4e960906b0d2bd&apikey="
	category := "test"
	// ========================================

	provider, err := nzbget.NewProvider(host, port, username, password, useSSL)
	if err != nil {
		t.Fatalf("Failed to create NZBGet provider: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Test connection
	t.Log("Testing NZBGet connection...")
	if err := provider.TestConnection(ctx); err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}
	t.Log("✓ Connection successful")

	// Get status
	t.Logf("Getting NZBGet status...")
	status, err := provider.GetStatus(ctx)
	if err != nil {
		t.Logf("Warning: Failed to get status: %v", err)
	} else {
		t.Logf("✓ Status retrieved")
		t.Logf("  Version: %s", status.Version)
		t.Logf("  Free Space: %d bytes", status.FreeSpace)
		t.Logf("  Download Speed: %d bytes/sec", status.TotalDownload)
	}

	// Add NZB
	t.Logf("Adding NZB to NZBGet...")
	err = provider.AddNZB(ctx, nzbURL, category, 0)
	if err != nil {
		t.Fatalf("Failed to add NZB: %v", err)
	}
	t.Logf("✓ NZB added successfully")

	// List downloads
	time.Sleep(2 * time.Second)
	t.Logf("Listing downloads...")
	list, err := provider.ListTorrents(ctx, "")
	if err != nil {
		t.Logf("Warning: Failed to list downloads: %v", err)
	} else {
		t.Logf("✓ Found %d download(s)", list.Total)
		for i, torrent := range list.Torrents {
			t.Logf("  [%d] %s - %.2f%%", i+1, torrent.Name, torrent.Progress)
		}
	}

	stats := provider.GetStats()
	t.Logf("\nStatistics:")
	t.Logf("  Total Requests: %d", stats.RequestsTotal)
	t.Logf("  Success Count: %d", stats.SuccessCount)
	t.Logf("  Failure Count: %d", stats.FailureCount)
}

// TestAllDownloadClients runs all download client tests in sequence
// Only runs tests that are not skipped
func TestAllDownloadClients(t *testing.T) {
	t.Skip("Manual test suite - enable individual tests above to run")

	t.Run("qBittorrent", TestQBittorrentDownload)
	t.Run("Deluge", TestDelugeDownload)
	t.Run("Transmission", TestTransmissionDownload)
	t.Run("rTorrent", TestRTorrentDownload)
	t.Run("SABnzbd", TestSABnzbdDownload)
	t.Run("NZBGet", TestNZBGetDownload)
}

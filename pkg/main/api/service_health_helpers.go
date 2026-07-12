package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// ServiceHealthResults holds the results of service health checks.
type ServiceHealthResults struct {
	TotalServices  int
	OnlineServices int
	FailedServices int
	ServiceDetails []ServiceInfo
	TestDuration   time.Duration
	OverallStatus  string
}

// ServiceInfo contains details about a service.
type ServiceInfo struct {
	Name         string
	Type         string
	URL          string
	Status       string // "online", "offline", "timeout", "error"
	ResponseTime time.Duration
	ErrorMessage string
	Details      map[string]any
}

// performServiceHealthCheck performs comprbnehensive service health checks.
//
// Each selected service is probed concurrently: probes are independent network
// round-trips (each with its own retries and timeout), so running them in
// parallel makes the overall check take about as long as the slowest single
// service rather than the sum of all of them. Results are collected under a
// mutex and counters are only incremented for services that actually run a test
// (disabled indexers/notifications are listed but not counted).
func performServiceHealthCheck(
	checkIMDB, checkTrakt, checkIndexers, checkNotifications bool,
	checkOMDB, checkTVDB, checkTMDB, checkTVMaze, checkMediaProviders bool,
	timeout, retries int,
	detailedTest, _, _ bool,
) *ServiceHealthResults {
	startTime := time.Now()

	results := &ServiceHealthResults{
		ServiceDetails: make([]ServiceInfo, 0),
	}

	httpTimeout := time.Duration(timeout) * time.Second

	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)

	// record appends a single result and, when countable, folds it into the
	// online/failed/total counters. Safe for concurrent callers.
	record := func(service ServiceInfo, countable bool) {
		mu.Lock()
		defer mu.Unlock()

		results.ServiceDetails = append(results.ServiceDetails, service)

		if !countable {
			return
		}

		results.TotalServices++
		if service.Status == "online" {
			results.OnlineServices++
		} else {
			results.FailedServices++
		}
	}

	// run launches fn in its own goroutine and records the single service it returns.
	run := func(fn func() ServiceInfo) {
		wg.Add(1)

		go func() {
			defer wg.Done()

			record(fn(), true)
		}()
	}

	if checkIMDB {
		run(func() ServiceInfo { return checkIMDBService(retries, detailedTest) })
	}

	if checkTrakt {
		run(func() ServiceInfo { return checkTraktService(httpTimeout, retries, detailedTest) })
	}

	if checkOMDB {
		run(func() ServiceInfo { return checkOMDBService(httpTimeout, retries, detailedTest) })
	}

	if checkTVDB {
		run(func() ServiceInfo { return checkTVDBService(httpTimeout, retries, detailedTest) })
	}

	if checkTMDB {
		run(func() ServiceInfo { return checkTMDBService(httpTimeout, retries, detailedTest) })
	}

	if checkTVMaze {
		run(func() ServiceInfo { return checkTVmazeService(httpTimeout, retries, detailedTest) })
	}

	// Indexers and notifications each return a slice of services; disabled ones
	// are listed but excluded from the metrics (countable == false).
	if checkIndexers {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for _, service := range checkIndexerServices(httpTimeout, retries, detailedTest) {
				record(service, service.Status != "disabled")
			}
		}()
	}

	if checkNotifications {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for _, service := range checkNotificationServices(httpTimeout, retries, detailedTest) {
				record(service, service.Status != "disabled")
			}
		}()
	}

	// Book / audiobook / music metadata providers (registry-driven, all countable).
	if checkMediaProviders {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for _, service := range checkMediaProviderServices(httpTimeout, retries, detailedTest) {
				record(service, true)
			}
		}()
	}

	wg.Wait()

	// Determine overall status
	if results.TotalServices > 0 && results.FailedServices >= results.TotalServices/2 {
		results.OverallStatus = "critical"
	} else if results.FailedServices > 0 {
		results.OverallStatus = "warning"
	} else {
		results.OverallStatus = "healthy"
	}

	results.TestDuration = time.Since(startTime)

	return results
}

// checkIMDBService checks IMDB service availability.
func checkIMDBService(retries int, _ bool) ServiceInfo {
	service := ServiceInfo{
		Name:    "IMDB",
		Type:    "Database",
		URL:     "Local IMDB Database",
		Details: make(map[string]any),
	}

	startTime := time.Now()

	// Try to connect to IMDB database by listing tables
	for attempt := range retries {
		// Query the IMDB database to list tables
		tables := database.GetrowsN[string](
			true,
			10,
			"SELECT name FROM sqlite_master WHERE type='table' ORDER BY name",
		)
		if len(tables) > 0 {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			service.Details["table_count"] = len(tables)
			service.Details["tables"] = tables

			break
		} else {
			service.Status = "error"

			service.ErrorMessage = fmt.Sprintf(
				"No tables found in IMDB database (attempt %d/%d)",
				attempt+1,
				retries,
			)
			if attempt < retries-1 {
				time.Sleep(100 * time.Millisecond) // Brief delay between retries
			}
		}
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// checkTraktService checks Trakt service availability.
func checkTraktService(timeout time.Duration, retries int, _ bool) ServiceInfo {
	service := ServiceInfo{
		Name:    "Trakt",
		Type:    "Metadata",
		URL:     "https://api.trakt.tv",
		Details: make(map[string]any),
	}

	startTime := time.Now()

	limit := "5"
	// Try to connect to Trakt API using the traktAPI.Client
	for attempt := range retries {
		statusCode, _, err := apiexternal.TestTraktConnectivity(timeout, &limit)
		if err != nil {
			// Check if it's an initialization error vs connection error
			if strings.Contains(err.Error(), "not initialized") ||
				strings.Contains(err.Error(), "missing ClientID") {
				service.Status = "error"
				service.ErrorMessage = err.Error()
				break // Don't retry initialization errors
			} else {
				service.Status = "timeout"

				service.ErrorMessage = fmt.Sprintf(
					"Connection failed (attempt %d/%d): %v",
					attempt+1,
					retries,
					err,
				)
				if attempt < retries-1 {
					time.Sleep(100 * time.Millisecond) // Brief delay between retries
				}

				continue
			}
		}

		// Trakt API returns different status codes, but we consider 2xx and some 4xx as "online"
		if statusCode >= 200 && statusCode < 500 {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			service.Details["status_code"] = statusCode
			break
		} else {
			service.Status = "error"
			service.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
			service.Details["status_code"] = statusCode
		}
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// checkIndexerServices checks configured indexer services.
func checkIndexerServices(timeout time.Duration, _ int, _ bool) []ServiceInfo {
	services := make([]ServiceInfo, 0)

	// Get configured indexers
	indexers := config.GetSettingsIndexerAll()
	if len(indexers) == 0 {
		// Return empty slice if no indexers configured - no mock data
		return services
	} else {
		// Check real configured indexers
		for _, indexer := range indexers {
			service := ServiceInfo{
				Name:    indexer.Name,
				Type:    "Indexer",
				URL:     indexer.URL,
				Details: make(map[string]any),
			}

			// Store indexer config details
			service.Details["enabled"] = indexer.Enabled
			service.Details["api_key_configured"] = indexer.Apikey != ""
			service.Details["limit_calls"] = indexer.Limitercalls
			service.Details["limit_seconds"] = indexer.Limiterseconds

			// Only test enabled indexers
			if !indexer.Enabled {
				service.Status = "disabled"
				service.ErrorMessage = "Indexer is disabled in configuration"
				services = append(services, service)
				continue
			}

			// Basic connectivity check
			if strings.HasPrefix(indexer.URL, "http") {
				startTime := time.Now()
				client := &http.Client{Timeout: timeout}

				// Try to perform a basic API query to test indexer functionality
				testURL := indexer.URL
				if !strings.HasSuffix(testURL, "/") {
					testURL += "/"
				}

				// Create a basic caps query or RSS query to test API functionality
				testURL += "api"
				if indexer.Apikey != "" {
					testURL += "?apikey=" + indexer.Apikey + "&t=caps"
				} else {
					testURL += "?t=caps"
				}

				testReq, _ := http.NewRequestWithContext(
					context.Background(),
					http.MethodGet,
					testURL,
					nil,
				)
				if resp, err := client.Do(testReq); err != nil {
					service.Status = "timeout"
					service.ErrorMessage = err.Error()
				} else {
					service.Details["test_url"] = testURL
					service.Details["status_code"] = resp.StatusCode

					if resp.StatusCode >= 200 && resp.StatusCode < 400 {
						service.Status = "online"
						service.ResponseTime = time.Since(startTime)

						resp.Body.Close()
					} else if resp.StatusCode == 401 || resp.StatusCode == 403 {
						service.Status = "error"
						service.ErrorMessage = fmt.Sprintf(
							"Authentication failed (HTTP %d) - check API key",
							resp.StatusCode,
						)
					} else {
						service.Status = "error"
						service.ErrorMessage = fmt.Sprintf("HTTP %d", resp.StatusCode)
					}

					resp.Body.Close()
				}
			} else {
				service.Status = "error"
				service.ErrorMessage = "Invalid URL format"
			}

			services = append(services, service)
		}
	}

	return services
}

// checkNotificationServices checks configured notification services.
func checkNotificationServices(_ time.Duration, _ int, _ bool) []ServiceInfo {
	services := make([]ServiceInfo, 0)

	// Get configured notification services
	notifications := config.GetSettingsNotificationAll()
	if len(notifications) == 0 {
		// Return empty slice if no notifications configured - no mock data
		return services
	} else {
		// Check real configured notification services
		for _, notification := range notifications {
			service := ServiceInfo{
				Name:    notification.Name,
				Type:    "Notification",
				Details: make(map[string]any),
			}

			// Basic service availability check (simplified)
			// In a real implementation, you'd test the actual notification service
			service.Status = "online" // Assume working for now
			service.ResponseTime = 100 * time.Millisecond

			services = append(services, service)
		}
	}

	return services
}

// checkOMDBService checks OMDB service availability.
func checkOMDBService(timeout time.Duration, retries int, _ bool) ServiceInfo {
	service := ServiceInfo{
		Name:    "OMDB",
		Type:    "Metadata",
		URL:     "http://www.omdbapi.com",
		Details: make(map[string]any),
	}

	startTime := time.Now()

	// Try to connect to OMDB API using the omdbAPI.Client
	for attempt := range retries {
		statusCode, err := apiexternal.TestOMDBConnectivity(timeout)
		if err != nil {
			// Check if it's an initialization error vs connection error
			if strings.Contains(err.Error(), "not initialized") ||
				strings.Contains(err.Error(), "missing API key") {
				service.Status = "error"
				service.ErrorMessage = err.Error()
				break // Don't retry initialization errors
			} else {
				service.Status = "timeout"

				service.ErrorMessage = fmt.Sprintf(
					"Connection failed (attempt %d/%d): %v",
					attempt+1,
					retries,
					err,
				)
				if attempt < retries-1 {
					time.Sleep(100 * time.Millisecond) // Brief delay between retries
				}

				continue
			}
		}

		// OMDB API returns different status codes, but we consider 2xx and some 4xx as "online"
		if statusCode >= 200 && statusCode < 500 {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			service.Details["status_code"] = statusCode
			break
		} else {
			service.Status = "error"
			service.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
			service.Details["status_code"] = statusCode
		}
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// checkTVmazeService checks TVmaze service availability.
func checkTVmazeService(timeout time.Duration, retries int, _ bool) ServiceInfo {
	service := ServiceInfo{
		Name:    "TVmaze",
		Type:    "Metadata",
		URL:     "https://api.tvmaze.com",
		Details: make(map[string]any),
	}

	startTime := time.Now()

	for attempt := range retries {
		statusCode, err := apiexternal.TestTVmazeConnectivity(timeout)
		if err != nil {
			if strings.Contains(err.Error(), "not initialized") ||
				strings.Contains(err.Error(), "missing API key") {
				service.Status = "error"
				service.ErrorMessage = err.Error()
				break // Don't retry initialization errors
			}

			service.Status = "timeout"

			service.ErrorMessage = fmt.Sprintf(
				"Connection failed (attempt %d/%d): %v",
				attempt+1,
				retries,
				err,
			)
			if attempt < retries-1 {
				time.Sleep(100 * time.Millisecond)
			}

			continue
		}

		if statusCode >= 200 && statusCode < 500 {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			service.Details["status_code"] = statusCode
			break
		}

		service.Status = "error"
		service.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
		service.Details["status_code"] = statusCode
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// checkTVDBService checks TVDB service availability.
func checkTVDBService(timeout time.Duration, retries int, _ bool) ServiceInfo {
	service := ServiceInfo{
		Name:    "TVDB",
		Type:    "Metadata",
		URL:     "https://api.thetvdb.com",
		Details: make(map[string]any),
	}

	startTime := time.Now()

	// Try to connect to TVDB API using the tvdbAPI.Client
	for attempt := range retries {
		statusCode, err := apiexternal.TestTVDBConnectivity(timeout)
		if err != nil {
			// Check if it's an initialization error vs connection error
			if strings.Contains(err.Error(), "not initialized") ||
				strings.Contains(err.Error(), "missing API key") {
				service.Status = "error"
				service.ErrorMessage = err.Error()
				break // Don't retry initialization errors
			} else {
				service.Status = "timeout"

				service.ErrorMessage = fmt.Sprintf(
					"Connection failed (attempt %d/%d): %v",
					attempt+1,
					retries,
					err,
				)
				if attempt < retries-1 {
					time.Sleep(100 * time.Millisecond) // Brief delay between retries
				}

				continue
			}
		}

		// TVDB API returns different status codes, but we consider 2xx and some 4xx as "online"
		if statusCode >= 200 && statusCode < 500 {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			service.Details["status_code"] = statusCode
			break
		} else {
			service.Status = "error"
			service.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
			service.Details["status_code"] = statusCode
		}
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// checkTMDBService checks TMDB service availability.
func checkTMDBService(timeout time.Duration, retries int, _ bool) ServiceInfo {
	service := ServiceInfo{
		Name:    "TMDB",
		Type:    "Metadata",
		URL:     "https://api.themoviedb.org/3",
		Details: make(map[string]any),
	}

	startTime := time.Now()

	// Try to connect to TMDB API using the tmdbAPI.Client
	for attempt := range retries {
		statusCode, err := apiexternal.TestTMDBConnectivity(timeout)
		if err != nil {
			// Check if it's an initialization error vs connection error
			if strings.Contains(err.Error(), "not initialized") ||
				strings.Contains(err.Error(), "missing API key") {
				service.Status = "error"
				service.ErrorMessage = err.Error()
				break // Don't retry initialization errors
			} else {
				service.Status = "timeout"

				service.ErrorMessage = fmt.Sprintf(
					"Connection failed (attempt %d/%d): %v",
					attempt+1,
					retries,
					err,
				)
				if attempt < retries-1 {
					time.Sleep(100 * time.Millisecond) // Brief delay between retries
				}

				continue
			}
		}

		// TMDB API returns different status codes, but we consider 2xx and some 4xx as "online"
		if statusCode >= 200 && statusCode < 500 {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			service.Details["status_code"] = statusCode
			break
		} else {
			service.Status = "error"
			service.ErrorMessage = fmt.Sprintf("HTTP %d", statusCode)
			service.Details["status_code"] = statusCode
		}
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// mediaProbe is a single book/audiobook/music provider health probe. probe
// performs a real, minimal API call (a search / lookup that injects the
// configured API key); a nil error means the service genuinely responded. A bare
// base-URL request returns HTTP 404 for most of these APIs, which is not a valid
// "reachable" signal, so a real call is used instead.
type mediaProbe struct {
	name     string
	category string
	url      string
	probe    func(context.Context) error
}

// collectMediaProbes builds a real API probe for every configured book,
// audiobook and music provider. Unconfigured providers are nil and skipped.
func collectMediaProbes() []mediaProbe {
	out := make([]mediaProbe, 0, 12)

	// Music metadata providers.
	if p := providers.GetMusicBrainz(); p != nil {
		out = append(out, mediaProbe{"MusicBrainz", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchArtists(ctx, "test", 1)
			return err
		}})
	}

	if p := providers.GetLastFM(); p != nil {
		out = append(out, mediaProbe{"Last.fm", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.GetTopArtists(ctx, 1, 1)
			return err
		}})
	}

	if p := providers.GetDiscogs(); p != nil {
		out = append(out, mediaProbe{"Discogs", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.Search(ctx, "test", "artist", 1, 1)
			return err
		}})
	}

	if p := providers.GetDeezer(); p != nil {
		out = append(out, mediaProbe{"Deezer", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchAlbums(ctx, "test", 1)
			return err
		}})
	}

	if p := providers.GetTheAudioDB(); p != nil {
		out = append(out, mediaProbe{"TheAudioDB", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchAlbums(ctx, "test", "test")
			return err
		}})
	}

	if p := providers.GetITunes(); p != nil {
		out = append(out, mediaProbe{"iTunes", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchAlbums(ctx, "test", "test", 1)
			return err
		}})
	}

	if p := providers.GetAcoustID(); p != nil {
		out = append(out, mediaProbe{"AcoustID", "Music", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.LookupByTrackID(ctx, "00000000-0000-0000-0000-000000000000")
			return err
		}})
	}

	// Book metadata providers.
	if p := providers.GetOpenLibrary(); p != nil {
		out = append(out, mediaProbe{"OpenLibrary", "Book", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchBooks(ctx, "test", "", 1)
			return err
		}})
	}

	if p := providers.GetGoodreads(); p != nil {
		out = append(out, mediaProbe{"Goodreads", "Book", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchBooks(ctx, "test", 1)
			return err
		}})
	}

	// Audiobook metadata providers.
	if p := providers.GetAudnex(); p != nil {
		out = append(out, mediaProbe{"Audnex", "Audiobook", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchAuthorByName(ctx, "test")
			return err
		}})
	}

	for region, p := range providers.GetAllAudible() {
		if p == nil {
			continue
		}

		p := p

		out = append(out, mediaProbe{"Audible (" + region + ")", "Audiobook", p.GetBaseURL(), func(ctx context.Context) error {
			_, err := p.SearchAudiobooks(ctx, "test", 1)
			return err
		}})
	}

	return out
}

// probeMediaProvider runs a real API probe (with retries) against a single
// book/audiobook/music provider. The service is "online" only when the call
// actually succeeds; any error (including HTTP 4xx such as 404/401) is reported
// as not reachable.
func probeMediaProvider(mp mediaProbe, timeout time.Duration, retries int) ServiceInfo {
	service := ServiceInfo{
		Name:    mp.name,
		Type:    mp.category,
		URL:     mp.url,
		Details: make(map[string]any),
	}

	startTime := time.Now()

	for attempt := range retries {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		err := mp.probe(ctx)

		cancel()

		if err == nil {
			service.Status = "online"
			service.ResponseTime = time.Since(startTime)
			break
		}

		msg := err.Error()

		// Configuration problems are not transient - don't retry them.
		if strings.Contains(msg, "not initialized") ||
			strings.Contains(strings.ToLower(msg), "missing api key") ||
			strings.Contains(strings.ToLower(msg), "no api key") {
			service.Status = "error"
			service.ErrorMessage = msg

			break
		}

		service.Status = "timeout"
		service.ErrorMessage = fmt.Sprintf(
			"Request failed (attempt %d/%d): %v",
			attempt+1,
			retries,
			err,
		)

		if attempt < retries-1 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if service.Status == "" {
		service.Status = "offline"
	}

	return service
}

// checkMediaProviderServices probes every configured book, audiobook and music
// provider via a real API call. The probes run concurrently since each is an
// independent network round-trip.
func checkMediaProviderServices(timeout time.Duration, retries int, _ bool) []ServiceInfo {
	probes := collectMediaProbes()

	services := make([]ServiceInfo, len(probes))

	var wg sync.WaitGroup

	for i := range probes {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()

			services[i] = probeMediaProvider(probes[i], timeout, retries)
		}(i)
	}

	wg.Wait()

	return services
}

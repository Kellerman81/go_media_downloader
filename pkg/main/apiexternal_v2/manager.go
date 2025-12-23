package apiexternal_v2

import (
	"fmt"
	"sync"
)

//
// ClientManager - Fully Typed Client Management
//

// Global client manager instance.
var (
	globalClientManager *ClientManager
	globalMutex         sync.RWMutex
)

// ClientManager manages all external API clients.
//
// The manager is primarily for dynamic provider lookup by name. For type-safe
// direct access to provider-specific methods, store and use provider instances directly
// instead of retrieving them from the manager.
//
// Example of type-safe direct usage (recommended):
//
//	omdbProvider := omdb.NewProviderWithConfig(config)
//	details, err := omdbProvider.GetDetailsByIMDb(ctx, imdbID) // No type assertion needed!
//
// Example of dynamic lookup (when you need it):
//
//	provider, _ := manager.GetMetadataProvider("omdb")
//	// Now requires type assertion for provider-specific methods
type ClientManager struct {
	mu sync.RWMutex

	// Metadata providers
	// metadataProviders  map[string]MetadataProvider
	watchlistProviders map[string]WatchlistProvider

	// Download clients
	// downloadProviders map[string]DownloadProvider

	// Notification services
	notificationProviders map[string]NotificationProvider

	// Indexer providers
	// indexerProviders map[string]IndexerProvider
}

// NewClientManager creates a new client manager.
func NewClientManager() *ClientManager {
	return &ClientManager{
		// metadataProviders:     make(map[string]MetadataProvider),
		watchlistProviders: make(map[string]WatchlistProvider),
		// downloadProviders:     make(map[string]DownloadProvider),
		notificationProviders: make(map[string]NotificationProvider),
		// indexerProviders:      make(map[string]IndexerProvider),
	}
}

//
// Metadata Provider Methods - Fully Typed
//

// RegisterMetadataProvider registers a metadata provider with the manager.
// func (cm *ClientManager) RegisterMetadataProvider(name string, provider MetadataProvider) {
// 	cm.mu.Lock()
// 	defer cm.mu.Unlock()

// 	cm.metadataProviders[name] = provider
// }

// GetMetadataProvider returns a metadata provider by name
// Returns the provider and true if found, nil and false otherwise.
// func (cm *ClientManager) GetMetadataProvider(name string) (MetadataProvider, bool) {
// 	cm.mu.RLock()
// 	defer cm.mu.RUnlock()

// 	provider, exists := cm.metadataProviders[name]

// 	return provider, exists
// }

// GetMetadataProviderOrError returns a metadata provider or an error if not found.
// func (cm *ClientManager) GetMetadataProviderOrError(name string) (MetadataProvider, error) {
// 	provider, exists := cm.GetMetadataProvider(name)
// 	if !exists {
// 		return nil, fmt.Errorf("metadata provider '%s' not found", name)
// 	}

// 	return provider, nil
// }

// ListMetadataProviders returns all registered metadata provider names.
// func (cm *ClientManager) ListMetadataProviders() []string {
// 	cm.mu.RLock()
// 	defer cm.mu.RUnlock()

// 	names := make([]string, 0, len(cm.metadataProviders))
// 	for name := range cm.metadataProviders {
// 		names = append(names, name)
// 	}

// 	return names
// }

//
// Watchlist Provider Methods - Fully Typed
//

// RegisterWatchlistProvider registers a watchlist provider with the manager.
func (cm *ClientManager) RegisterWatchlistProvider(name string, provider WatchlistProvider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.watchlistProviders[name] = provider
}

// GetWatchlistProvider returns a watchlist provider by name.
func (cm *ClientManager) GetWatchlistProvider(name string) (WatchlistProvider, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	provider, exists := cm.watchlistProviders[name]

	return provider, exists
}

// GetWatchlistProviderOrError returns a watchlist provider or an error if not found.
func (cm *ClientManager) GetWatchlistProviderOrError(name string) (WatchlistProvider, error) {
	provider, exists := cm.GetWatchlistProvider(name)
	if !exists {
		return nil, fmt.Errorf("watchlist provider '%s' not found", name)
	}

	return provider, nil
}

//
// Download Provider Methods - Fully Typed
//

// RegisterDownloadProvider registers a download provider with the manager.
// func (cm *ClientManager) RegisterDownloadProvider(name string, provider DownloadProvider) {
// 	cm.mu.Lock()
// 	defer cm.mu.Unlock()

// 	cm.downloadProviders[name] = provider
// }

// GetDownloadProvider returns a download provider by name.
// func (cm *ClientManager) GetDownloadProvider(name string) (DownloadProvider, bool) {
// 	cm.mu.RLock()
// 	defer cm.mu.RUnlock()

// 	provider, exists := cm.downloadProviders[name]

// 	return provider, exists
// }

// GetDownloadProviderOrError returns a download provider or an error if not found.
// func (cm *ClientManager) GetDownloadProviderOrError(name string) (DownloadProvider, error) {
// 	provider, exists := cm.GetDownloadProvider(name)
// 	if !exists {
// 		return nil, fmt.Errorf("download provider '%s' not found", name)
// 	}

// 	return provider, nil
// }

//
// Notification Provider Methods - Fully Typed
//

// RegisterNotificationProvider registers a notification provider with the manager.
func (cm *ClientManager) RegisterNotificationProvider(name string, provider NotificationProvider) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.notificationProviders[name] = provider
}

// GetNotificationProvider returns a notification provider by name.
func (cm *ClientManager) GetNotificationProvider(name string) (NotificationProvider, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	provider, exists := cm.notificationProviders[name]

	return provider, exists
}

// GetNotificationProviderOrError returns a notification provider or an error if not found.
func (cm *ClientManager) GetNotificationProviderOrError(name string) (NotificationProvider, error) {
	provider, exists := cm.GetNotificationProvider(name)
	if !exists {
		return nil, fmt.Errorf("notification provider '%s' not found", name)
	}

	return provider, nil
}

// GetAllNotificationProviders returns all registered notification providers.
func (cm *ClientManager) GetAllNotificationProviders() map[string]NotificationProvider {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	providers := make(map[string]NotificationProvider, len(cm.notificationProviders))
	for name, provider := range cm.notificationProviders {
		providers[name] = provider
	}

	return providers
}

//
// Indexer Provider Methods - Fully Typed
//

// RegisterIndexerProvider registers an indexer provider with the manager.
// func (cm *ClientManager) RegisterIndexerProvider(name string, provider IndexerProvider) {
// 	cm.mu.Lock()
// 	defer cm.mu.Unlock()

// 	cm.indexerProviders[name] = provider
// }

// GetIndexerProvider returns an indexer provider by name.
// func (cm *ClientManager) GetIndexerProvider(name string) (IndexerProvider, bool) {
// 	cm.mu.RLock()
// 	defer cm.mu.RUnlock()

// 	provider, exists := cm.indexerProviders[name]

// 	return provider, exists
// }

// // GetIndexerProviderOrError returns an indexer provider or an error if not found.
// func (cm *ClientManager) GetIndexerProviderOrError(name string) (IndexerProvider, error) {
// 	provider, exists := cm.GetIndexerProvider(name)
// 	if !exists {
// 		return nil, fmt.Errorf("indexer provider '%s' not found", name)
// 	}

// 	return provider, nil
// }

// ListIndexerProviders returns all registered indexer provider names.
// func (cm *ClientManager) ListIndexerProviders() []string {
// 	cm.mu.RLock()
// 	defer cm.mu.RUnlock()

// 	names := make([]string, 0, len(cm.indexerProviders))
// 	for name := range cm.indexerProviders {
// 		names = append(names, name)
// 	}

// 	return names
// }

//
// Convenience Methods for Common Operations
//

// GetAllWatchlists retrieves watchlists from all registered providers.

// ProviderHealthStatus represents the health status of a single provider.
type ProviderHealthStatus struct {
	Healthy bool   `json:"healthy"`
	Type    string `json:"type"` // metadata, download, notification, indexer, watchlist
	Error   string `json:"error,omitempty"`
}

// GetHealthStatus returns the health status of all registered providers.
func (cm *ClientManager) GetHealthStatus() map[string]ProviderHealthStatus {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	healthStatus := make(map[string]ProviderHealthStatus)

	// Metadata and download providers are now in the direct registry (providers package)
	// This health check method is deprecated - use providers package directly

	// Check notification providers
	for name := range cm.notificationProviders {
		healthStatus[name] = ProviderHealthStatus{
			Healthy: true,
			Type:    "notification",
		}
	}

	// Check watchlist providers
	for name := range cm.watchlistProviders {
		healthStatus[name] = ProviderHealthStatus{
			Healthy: true,
			Type:    "watchlist",
		}
	}

	return healthStatus
}

//
// Global Client Manager Access
//

// SetGlobalClientManager sets the global client manager instance
//
// This allows for singleton-style access to the client manager throughout
// the application while still allowing for dependency injection in tests.
func SetGlobalClientManager(manager *ClientManager) {
	globalMutex.Lock()
	defer globalMutex.Unlock()

	globalClientManager = manager
}

// GetGlobalClientManager returns the global client manager instance
//
// Returns the manager and true if it's been initialized, nil and false otherwise.
func GetGlobalClientManager() (*ClientManager, bool) {
	globalMutex.RLock()
	defer globalMutex.RUnlock()

	if globalClientManager == nil {
		return nil, false
	}

	return globalClientManager, true
}

// GetGlobalClientManagerOrPanic returns the global client manager or panics if not initialized
//
// Use this only in situations where you're certain the manager has been initialized.
// For safer access, use GetGlobalClientManager() instead.
// func getGlobalClientManagerOrPanic() *ClientManager {
// 	manager, exists := GetGlobalClientManager()
// 	if !exists {
// 		panic("global client manager not initialized - call SetGlobalClientManager first")
// 	}

// 	return manager
// }

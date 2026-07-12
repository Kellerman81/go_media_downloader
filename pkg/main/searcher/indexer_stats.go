package searcher

import (
	"cmp"
	"slices"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

// indexerSuccessRate returns the historical success rate for the indexer
// in [0.0, 1.0], based on the request statistics the BaseClient already
// records for every request. Indexers without history default to 1.0 so
// new indexers are tried first rather than penalized.
func indexerSuccessRate(indcfg *config.IndexersConfig) float64 {
	provider := apiexternal.Getnewznabclient(indcfg)
	if provider == nil {
		return 1.0
	}

	stats := provider.GetStats()

	total := stats.SuccessCount + stats.FailureCount
	if total == 0 {
		return 1.0
	}

	return float64(stats.SuccessCount) / float64(total)
}

// sortIndexersByHealth orders indexers by historical success rate, best first,
// so healthy indexers are queried first when the worker pool is saturated.
// The sort is stable so equally healthy indexers keep their configured order.
func sortIndexersByHealth(indexers []*config.IndexersConfig) {
	if len(indexers) < 2 {
		return
	}

	slices.SortStableFunc(indexers, func(a, b *config.IndexersConfig) int {
		return cmp.Compare(indexerSuccessRate(b), indexerSuccessRate(a))
	})
}

package apiexternal_v2

import "github.com/Kellerman81/go_media_downloader/pkg/main/config"

//
// Conversion Functions - Context population helpers
//
// This file provides helper functions to populate Indexer and Quality context
// on search results for downstream validation.
//

// PopulateIndexerContext sets the Indexer and Quality context on all results
// in a slice of Nzbwithprio. This ensures downstream validation (like regex matching)
// has the required context.
func PopulateIndexerContext(
	results []Nzbwithprio,
	indexer *config.IndexersConfig,
	quality *config.QualityConfig,
) {
	for i := range results {
		results[i].NZB.Indexer = indexer
		results[i].NZB.Quality = quality
	}
}

// NzbToIndexerSearchRequest converts legacy search parameters to v2 IndexerSearchRequest.
// This is a helper for searcher compatibility.
// func nzbToIndexerSearchRequest(
// 	query, season, episode string,
// 	categories []string,
// 	limit int,
// ) IndexerSearchRequest {
// 	return IndexerSearchRequest{
// 		Query:      query,
// 		Season:     season,
// 		Episode:    episode,
// 		Categories: categories,
// 		Limit:      limit,
// 	}
// }

package apiexternal_v2

import "github.com/Kellerman81/go_media_downloader/pkg/main/config"

//
// Nzbwithprio Methods - Helper methods for validation and processing
//

// Getregexcfg retrieves the regex configuration for the NZB entry.
// It checks the indexer configuration and quality settings to determine
// which regex patterns should be used for validation.
//
// Returns nil when:
// - No indexer is associated with the NZB
// - No quality config is provided
// - Quality profile has no indexers configured
// - Indexer is not found in quality profile (by TemplateIndexer match)
//
// IMPORTANT: The indexer must be explicitly added to the quality profile's
// indexer list with a matching TemplateIndexer name to get regex validation.
func (s *Nzbwithprio) Getregexcfg(qual *config.QualityConfig) *config.RegexConfig {
	if s.NZB.Indexer == nil {
		return nil
	}

	if qual == nil || len(qual.Indexer) == 0 {
		return nil
	}

	indexerCfg := s.NZB.Indexer

	// Try to find matching indexer in quality profile by TemplateIndexer name
	// This checks qual.Indexer[i].TemplateIndexer == indexerCfg.Name
	indcfg := qual.QualityIndexerByQualityAndTemplateCheckRegex(indexerCfg)
	if indcfg != nil {
		return indcfg
	}

	// No match found - indexer exists but is not configured in this quality profile
	// The indexer must be added to the quality profile's indexer list with:
	// - TemplateIndexer set to the indexer name (e.g., "scenenzbs")
	// - CfgRegex pointing to a regex template for validation
	return nil
}

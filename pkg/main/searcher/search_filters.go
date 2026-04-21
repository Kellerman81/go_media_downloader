package searcher

import (
	"slices"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
)

// filterTestQualityWanted checks if the quality attributes of the
// Nzbwithprio entry match the wanted quality configuration. It returns
// true if any unwanted quality is found to stop further processing of
// the entry.
//
// Optimized: Removed slice allocation by inlining checks directly.
// This avoids creating a temporary slice struct on every call.
func (s *ConfigSearcher) filterTestQualityWanted(
	entry *apiexternal_v2.Nzbwithprio,
	quality *config.QualityConfig,
) bool {
	if quality == nil {
		return false
	}

	// Check Resolution
	if quality.WantedResolutionLen >= 1 && entry.Info.Resolution != "" {
		if !logger.SlicesContainsI(quality.WantedResolution, entry.Info.Resolution) {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "unwanted Resolution").
				Str(logger.StrTitle, entry.NZB.Title).
				Str(logger.StrFound, entry.Info.Resolution).
				Strs(logger.StrWanted, quality.WantedResolution).
				Msg(skippedstr)

			entry.Reason = "unwanted Resolution"
			s.logdenied("", entry)

			return true
		}
	}

	// Check Quality
	if quality.WantedQualityLen >= 1 && entry.Info.Quality != "" {
		if !logger.SlicesContainsI(quality.WantedQuality, entry.Info.Quality) {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "unwanted Quality").
				Str(logger.StrTitle, entry.NZB.Title).
				Str(logger.StrFound, entry.Info.Quality).
				Strs(logger.StrWanted, quality.WantedQuality).
				Msg(skippedstr)

			entry.Reason = "unwanted Quality"
			s.logdenied("", entry)

			return true
		}
	}

	// Check Audio
	if quality.WantedAudioLen >= 1 && entry.Info.Audio != "" {
		if !logger.SlicesContainsI(quality.WantedAudio, entry.Info.Audio) {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "unwanted Audio").
				Str(logger.StrTitle, entry.NZB.Title).
				Str(logger.StrFound, entry.Info.Audio).
				Strs(logger.StrWanted, quality.WantedAudio).
				Msg(skippedstr)

			entry.Reason = "unwanted Audio"
			s.logdenied("", entry)

			return true
		}
	}

	// Check Codec
	if quality.WantedCodecLen >= 1 && entry.Info.Codec != "" {
		if !logger.SlicesContainsI(quality.WantedCodec, entry.Info.Codec) {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "unwanted Codec").
				Str(logger.StrTitle, entry.NZB.Title).
				Str(logger.StrFound, entry.Info.Codec).
				Strs(logger.StrWanted, quality.WantedCodec).
				Msg(skippedstr)

			entry.Reason = "unwanted Codec"
			s.logdenied("", entry)

			return true
		}
	}

	// Check Audio Format (for music/audiobooks)
	if quality.WantedAudioFormatsLen >= 1 && entry.Info.AudioFormat != "" {
		if !logger.SlicesContainsI(quality.WantedAudioFormats, entry.Info.AudioFormat) {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "unwanted AudioFormat").
				Str(logger.StrTitle, entry.NZB.Title).
				Str(logger.StrFound, entry.Info.AudioFormat).
				Strs(logger.StrWanted, quality.WantedAudioFormats).
				Msg(skippedstr)

			entry.Reason = "unwanted AudioFormat"
			s.logdenied("", entry)

			return true
		}
	}

	return false
}

// audioFormatFromCategory infers an audio format string from a Newznab category ID.
// Standard Newznab music categories:
//
//	3010 = MP3
//	3040 = Lossless (typically FLAC)
//
// Returns empty string for unknown/generic categories.
func audioFormatFromCategory(category string) string {
	switch category {
	case "3010":
		return "mp3"
	case "3040":
		return "flac"
	}

	return ""
}

// resolutionFromCategory infers a video resolution string from a Newznab category ID.
// Standard Newznab video categories:
//
//	2030/5030 = SD  → "480p"
//	2040/5040 = HD  → "1080p" only when the indexer also reports UHD categories (2045/5045),
//	                  confirming the release is not UHD
//	2045/5045 = UHD → "2160p"
//
// supportedCategories should be the Provider.SupportedCategories for the source indexer.
// Returns empty string when resolution cannot be reliably determined.
func resolutionFromCategory(category string, supportedCategories []string) string {
	switch category {
	case "2045", "5045":
		return "2160p"
	case "2040", "5040":
		// Only map HD to 1080p when we know the indexer distinguishes UHD;
		// if the indexer has no UHD category, HD could mean anything ≥720p.
		for _, c := range supportedCategories {
			if c == "2045" || c == "5045" {
				return "1080p"
			}
		}

		return ""

	case "2030", "5030":
		return "480p"
	}

	return ""
}

// validateSize checks if an NZB entry meets size-related validation criteria.
// It verifies whether the entry should be skipped based on empty size configuration
// and performs additional size filtering.
//
// Parameters:
//   - entry: The NZB entry to validate
//
// Returns:
//   - true if the entry should be filtered out due to size constraints, false otherwise
func (s *ConfigSearcher) validateSize(entry *apiexternal_v2.Nzbwithprio) bool {
	if entry.NZB.Indexer == nil {
		return false
	}

	skipemptysize := s.Quality.QualityIndexerByQualityAndTemplateSkipEmpty(entry.NZB.Indexer)
	if !skipemptysize {
		if ok := config.TestSettingsList(entry.NZB.Indexer.Name); ok {
			skipemptysize = s.Quality.Indexer[0].SkipEmptySize
		} else if entry.NZB.Indexer.Getlistbyindexer() != nil {
			skipemptysize = s.Quality.Indexer[0].SkipEmptySize
		}
	}

	if skipemptysize && entry.NZB.Size == 0 {
		s.logdenied("no size", entry)
		return true
	}

	return s.filterSizeNzbs(entry)
}

// validateEntry combines multiple validation steps.
func (s *ConfigSearcher) validateEntry(
	e, entry *apiexternal_v2.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	// History check
	if s.checkhistory(entry, qual) {
		return true
	}

	// Episode check for series
	if s.searchActionType != logger.StrRss && s.checkepisode(e, entry) {
		return true
	}

	// Regex filtering
	if s.filterRegexNzbs(entry, qual) {
		return true
	}

	// Priority calculation
	if entry.Info.Priority == 0 {
		parser.GetPriorityMapQual(&entry.Info, s.Cfgp, qual, false, true)

		if entry.Info.Priority == 0 {
			s.logdenied1Str("unknown Prio", entry, logger.StrFound, entry.Info.Title)
			return true
		}
	}

	entry.Info.StripTitlePrefixPostfixGetQual(qual)

	// Quality validation
	if s.filterTestQualityWanted(entry, qual) {
		return true
	}

	// Priority validation
	if s.getminimumpriority(entry, qual) {
		return true
	}

	if entry.MinimumPriority != 0 && entry.MinimumPriority == entry.Info.Priority {
		s.logdenied("same Prio", entry)
		return true
	}

	if entry.MinimumPriority != 0 {
		minDiff := qual.UseForPriorityMinDifference

		threshold := entry.MinimumPriority
		if minDiff != 0 {
			threshold += minDiff
		}

		if entry.Info.Priority <= threshold {
			logger.Logtype("debug", 0).
				Str(logger.StrReason, "lower Prio").
				Str(logger.StrTitle, entry.NZB.Title).
				Int(logger.StrFound, entry.Info.Priority).
				Int(logger.StrWanted, entry.MinimumPriority).
				Msg(skippedstr)

			entry.Reason = "lower Prio"
			s.logdenied("", entry)

			return true
		}
	}

	// Year check for movies
	if s.searchActionType != logger.StrRss && s.checkyear(e, entry, qual) {
		return true
	}

	// Title check
	if s.checktitle(entry, qual) {
		return true
	}

	logger.Logtype("debug", 4).
		Str(logger.StrQuality, qual.Name).
		Str(logger.StrTitle, entry.NZB.Title).
		Int(logger.StrMinPrio, entry.MinimumPriority).
		Int(logger.StrPriority, entry.Info.Priority).
		Msg("Release ok")

	return false
}

// filterSizeNzbs checks if the NZB entry size is within the configured
// minimum and maximum size limits, and returns true if it should be
// rejected based on its size.
func (s *ConfigSearcher) filterSizeNzbs(entry *apiexternal_v2.Nzbwithprio) bool {
	if entry.NZB.Size == 0 {
		return false // Skip size check if no size info
	}

	for _, dataimport := range s.Cfgp.DataImportMap {
		if dataimport == nil || dataimport.CfgPath == nil {
			continue
		}

		if dataimport.CfgPath.MinSize != 0 && entry.NZB.Size < dataimport.CfgPath.MinSizeByte {
			s.logdenied1Int64("too small", entry)
			return true
		}

		if dataimport.CfgPath.MaxSize != 0 && entry.NZB.Size > dataimport.CfgPath.MaxSizeByte {
			s.logdenied1Int64("too big", entry)
			return true
		}
	}

	for _, data := range s.Cfgp.DataMap {
		if data == nil || data.CfgPath == nil {
			continue
		}

		if data.CfgPath.MinSize != 0 && entry.NZB.Size < data.CfgPath.MinSizeByte {
			s.logdenied1Int64("too small", entry)
			return true
		}

		if data.CfgPath.MaxSize != 0 && entry.NZB.Size > data.CfgPath.MaxSizeByte {
			s.logdenied1Int64("too big", entry)
			return true
		}
	}

	return false
}

// filterRegexNzbs checks if the given NZB entry matches the required regexes
// and does not match any rejected regexes from the quality configuration.
// Returns true if the entry fails the regex checks, false if it passes.
func (s *ConfigSearcher) filterRegexNzbs(
	entry *apiexternal_v2.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	regexcfg := entry.Getregexcfg(qual)
	if regexcfg == nil {
		s.logdenied1Str("Denied by Regex", entry, strRegexEmpty, "")
		return true
	}

	if regexcfg.RequiredLen >= 1 {
		var bl bool
		for idx := range regexcfg.Required {
			if database.RegexGetMatchesFind(regexcfg.Required[idx], entry.NZB.Title, 1) {
				bl = true
				break
			}
		}

		if !bl {
			s.logdenied1Str("not matched required", entry, strCheckedFor, regexcfg.Required[0])
			return true
		}
	}

	for idxr := range regexcfg.Rejected {
		if !database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.NZB.Title, 1) {
			continue
		}

		// Check if wanted title matches (allowed)
		if database.RegexGetMatchesFind(regexcfg.Rejected[idxr], entry.WantedTitle, 1) {
			continue
		}

		bl := false
		for idx := range entry.WantedAlternates {
			if entry.WantedTitle != entry.WantedAlternates[idx].Str1 &&
				database.RegexGetMatchesFind(
					regexcfg.Rejected[idxr],
					entry.WantedAlternates[idx].Str1,
					1,
				) {
				bl = true
				break
			}
		}

		if !bl {
			s.logdenied1Str(
				"Denied by Regex",
				entry,
				strRejectedby,
				regexcfg.Rejected[idxr],
			)

			return true
		}
	}

	return false
}

// checkhistory checks if the given entry is already in the history cache
// to avoid duplicate downloads. It checks based on the download URL and title.
// Returns true if a duplicate is found, false otherwise.
func (s *ConfigSearcher) checkhistory(
	entry *apiexternal_v2.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	if entry.NZB.DownloadURL != "" &&
		database.CheckcachedURLHistory(s.Cfgp.IsType, &entry.NZB.DownloadURL) {
		s.logdenied("already downloaded url", entry)
		return true
	}

	if entry.NZB.Indexer == nil ||
		!qual.QualityIndexerByQualityAndTemplateCheckTitle(entry.NZB.Indexer) {
		return false
	}

	if entry.NZB.Title != "" &&
		database.CheckcachedTitleHistory(s.Cfgp.IsType, &entry.NZB.Title) {
		s.logdenied("already downloaded title", entry)
		return true
	}

	return false
}

// checkprocessed checks if the given entry is already processed using O(1) map lookups
// instead of O(n) loops through denied and accepted lists for better performance.
// Returns true if a match is found on the download URL or title.
func (s *ConfigSearcher) checkprocessed(entry *apiexternal_v2.Nzb) bool {
	// O(1) lookup for URL duplicates
	if entry.DownloadURL != "" {
		if _, exists := s.processedURLs[entry.DownloadURL]; exists {
			return true
		}
	}

	// O(1) lookup for title duplicates
	if entry.Title != "" {
		if _, exists := s.processedTitles[entry.Title]; exists {
			return true
		}
	}

	return false
}

// checkcorrectid checks if the entry matches the expected ID based on
// whether it is a movie or series. For movies it checks the IMDB ID,
// trimming any "t0" prefix. For series it checks the TVDB ID. If the
// IDs don't match, it logs a message and returns true to skip the entry.
func (s *ConfigSearcher) checkcorrectid(sourceentry, entry *apiexternal_v2.Nzbwithprio) bool {
	if s.searchActionType == logger.StrRss || sourceentry == nil {
		return false
	}

	handler := mediatype.Get(s.Cfgp.IsType)
	if handler == nil {
		return false
	}

	skip, foundID, wantedID := handler.CheckCorrectID(sourceentry, entry)
	if skip {
		logger.Logtype("debug", 0).
			Str(logger.StrReason, entry.Reason).
			Str(logger.StrTitle, entry.NZB.Title).
			Str(logger.StrFound, foundID).
			Str(logger.StrWanted, wantedID).
			Msg(skippedstr)

		s.logdenied("", entry)
	}

	return skip
}

// checkyear validates the year in the entry title against the year
// configured for the wanted entry. It returns false if a match is found,
// or true to skip the entry if no match is found. This is used during
// search result processing to filter entries by year.
func (s *ConfigSearcher) checkyear(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	// Skip year check for media types that don't require it or for RSS searches
	if !mediatype.RequiresYearCheck(s.Cfgp.IsType) ||
		s.searchActionType == logger.StrRss ||
		sourceentry == nil {
		return false
	}

	if sourceentry.Info.Year == 0 {
		s.logdenied("no year", entry)
		return true
	}

	if qual.CheckYear || qual.CheckYear1 {
		targetYear := sourceentry.Info.Year

		// Check exact year
		if logger.ContainsInt(entry.NZB.Title, targetYear) {
			return false
		}

		// Check year +/- 1 if enabled
		if qual.CheckYear1 {
			if logger.ContainsInt(entry.NZB.Title, targetYear+1) ||
				logger.ContainsInt(entry.NZB.Title, targetYear-1) {
				return false
			}
		}
	}

	s.logdenied1UInt16("unwanted Year", entry, sourceentry.Info.Year)

	return true
}

// checkdownloaded checks if the entry at index idx has already been downloaded
// using O(1) map lookup instead of O(n) slice iteration for better performance.
// It returns true if the entry's movie ID or episode ID is in the downloadedMap.
//
// Optimized: Removed unused 'downloaded' slice parameter since downloadedMap
// provides the same functionality with O(1) lookups.
func (s *ConfigSearcher) checkdownloaded(idx int) bool {
	entry := &s.Accepted[idx]

	// Check movie ID for movies
	if entry.NzbmovieID != 0 {
		if _, exists := s.downloadedMap[entry.NzbmovieID]; exists {
			return true
		}
	}

	// Check episode ID for series
	if entry.NzbepisodeID != 0 {
		if _, exists := s.downloadedMap[entry.NzbepisodeID]; exists {
			return true
		}
	}

	// For new media types (books, audiobooks, music), use the handler's GetNzbID
	if !mediatype.SupportsIDSearch(s.Cfgp.IsType) {
		if h := mediatype.Get(s.Cfgp.IsType); h != nil {
			nzbID := h.GetNzbID(entry)
			if nzbID != 0 {
				if _, exists := s.downloadedMap[nzbID]; exists {
					return true
				}
			}
		}
	}

	return false
}

// checktitle validates the title and alternate titles of the entry against
// the wanted title and quality configuration. It returns false if a match is
// found, or true to skip the entry if no match is found. This is an internal
// function used during search result processing.
func (s *ConfigSearcher) checktitle(
	entry *apiexternal_v2.Nzbwithprio,
	qual *config.QualityConfig,
) bool {
	// Checktitle
	if !qual.CheckTitle {
		return false
	}

	if !qual.CheckTitleOnIDSearch && entry.IDSearched {
		return false
	}

	if !entry.NZB.Indexer.CheckTitleOnIDSearch && entry.IDSearched {
		return false
	}

	entry.Info.StripTitlePrefixPostfixGetQual(qual)

	wantedslug := logger.StringToSlug(entry.WantedTitle)
	if entry.WantedTitle != "" && qual.CheckTitle &&
		database.ChecknzbtitleB(
			entry.WantedTitle,
			wantedslug,
			entry.NZB.Title,
			qual.CheckYear1,
			entry.Info.Year,
		) {
		return false
	}

	var trytitle string
	if entry.WantedTitle != "" && strings.ContainsRune(entry.Info.Title, ']') {
		if idx := strings.LastIndexByte(entry.Info.Title, ']'); idx != -1 &&
			idx < len(entry.Info.Title)-1 {
			trytitle = logger.TrimLeft(entry.Info.Title[idx+1:], '-', '.', ' ')
			if qual.CheckTitle && entry.WantedTitle != "" &&
				database.ChecknzbtitleB(
					entry.WantedTitle,
					wantedslug,
					trytitle,
					qual.CheckYear1,
					entry.Info.Year,
				) {
				return false
			}
		}
	}

	if entry.Dbid != 0 && len(entry.WantedAlternates) == 0 {
		entry.WantedAlternates = database.GetDbstaticTwoStringOneInt(
			database.Getentryalternatetitlesdirect(&entry.Dbid, s.Cfgp.IsType),
			entry.Dbid,
		)
	}

	if entry.Info.Title == "" || len(entry.WantedAlternates) == 0 {
		s.logdenied("unwanted Title", entry)
		return true
	}

	// Optimized: Removed redundant bounds check (idx >= len is never true in range loop)
	// and use pointer to avoid copying struct on each iteration
	for idx := range entry.WantedAlternates {
		alt := &entry.WantedAlternates[idx]
		if alt.Str1 == "" {
			continue
		}

		if database.ChecknzbtitleB(
			alt.Str1,
			alt.Str2,
			entry.NZB.Title,
			qual.CheckYear1,
			entry.Info.Year,
		) {
			return false
		}

		if trytitle == "" || trytitle == alt.Str1 || trytitle == entry.WantedTitle {
			continue
		}

		if database.ChecknzbtitleB(
			alt.Str1,
			alt.Str2,
			trytitle,
			qual.CheckYear1,
			entry.Info.Year,
		) {
			return false
		}
	}

	s.logdenied("unwanted Title and alternate", entry)

	return true
}

// checkepisode validates the episode identifier in the entry against the
// season and episode values. It returns false if the identifier matches the
// expected format, or true to skip the entry if the identifier is invalid.
func (s *ConfigSearcher) checkepisode(sourceentry, entry *apiexternal_v2.Nzbwithprio) bool {
	// Only series need episode validation - other types skip this check entirely
	// (return false = don't skip the entry).
	if !mediatype.SupportsSeasonSearch(s.Cfgp.IsType) {
		return false
	}

	// Series-specific episode validation
	if sourceentry == nil {
		s.logdenied("no sourceentry", entry)
		return true
	}

	if s.searchActionType == logger.StrRss && sourceentry.Info.Identifier == "" {
		return false
	}

	if sourceentry.Info.Identifier == "" {
		s.logdenied("no identifier", entry)
		return true
	}

	if logger.ContainsI(entry.NZB.Title, sourceentry.Info.Identifier) {
		return false
	}

	if s.checkAlternativeFormats(sourceentry, entry) {
		return false
	}

	if s.checkAlternativeIdentifier(sourceentry, entry) {
		return false
	}

	// Final validation for season/episode format
	if sourceentry.NZB.Season == "" || sourceentry.NZB.Episode == "" {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	return s.checkEpisodeFormat(sourceentry, entry, sourceentry.Info.Identifier)
}

// checkEpisodeFormat validates the episode identifier format for a given entry.
// It checks if the identifier matches the expected season and episode format.
// Returns false if the identifier is valid, true if the entry should be skipped.
//
// Optimized: Reduced string allocations by building patterns once and reusing them.
// Combined suffix pattern checks to minimize iterations.
func (s *ConfigSearcher) checkEpisodeFormat(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
	identifier string,
) bool {
	sprefix, eprefix := "s", "e"
	if logger.ContainsI(identifier, "x") {
		sprefix = ""
		eprefix = "x"
	} else if !logger.ContainsI(identifier, "s") && !logger.ContainsI(identifier, "e") {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	if !logger.HasPrefixI(identifier, sprefix+sourceentry.NZB.Season) {
		s.logdenied1StrNo("unwanted Season", entry, &sourceentry.Info)
		return true
	}

	episode := sourceentry.NZB.Episode
	if !logger.ContainsI(identifier, episode) {
		s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)
		return true
	}

	// Pre-build common episode patterns to reduce allocations in loops
	eprefixEpisode := eprefix + episode

	// Check episode suffixes with all prefix combinations
	// episodePrefixes = {"", " ", "0", " 0"}
	for i := range episodePrefixes {
		suffix := eprefix + episodePrefixes[i] + episode
		if logger.HasSuffixI(identifier, suffix) {
			return false
		}
	}

	// Check suffix patterns: eprefix, space, dash
	// Instead of nested loops, check each pattern combination directly
	if logger.ContainsI(identifier, eprefixEpisode+eprefix) ||
		logger.ContainsI(identifier, eprefixEpisode+logger.StrSpace) ||
		logger.ContainsI(identifier, eprefixEpisode+logger.StrDash) {
		return false
	}

	// Check suffix patterns with episode prefixes
	for i := range episodePrefixes {
		base := eprefix + episodePrefixes[i] + episode
		if logger.HasSuffixI(identifier, base+eprefix) ||
			logger.HasSuffixI(identifier, base+logger.StrSpace) ||
			logger.HasSuffixI(identifier, base+logger.StrDash) {
			return false
		}
	}

	s.logdenied1StrNo("unwanted Identifier", entry, &sourceentry.Info)

	return true
}

// checkAlternativeFormats checks for alternative identifier formats when season and episode are not explicitly specified.
// It handles cases where the original identifier uses a hyphen separator, and checks if the entry title
// contains the same identifier with dot or space separators instead.
// Returns true if an alternative format match is found, otherwise false.
func (s *ConfigSearcher) checkAlternativeFormats(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
) bool {
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" &&
		strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		// Check dot separator
		if strings.ContainsRune(entry.NZB.Title, '.') &&
			logger.ContainsI(entry.NZB.Title,
				logger.StringReplaceWith(sourceentry.Info.Identifier, '-', '.')) {
			return true
		}

		// Check space separator
		if strings.ContainsRune(entry.NZB.Title, ' ') &&
			logger.ContainsI(entry.NZB.Title,
				logger.StringReplaceWith(sourceentry.Info.Identifier, '-', ' ')) {
			return true
		}
	}

	return false
}

// checkAlternativeIdentifier checks for alternative identifier formats by converting and transforming the source identifier.
// It handles cases like removing leading 's/S', converting 'E/e' to 'x' format, and checking for alternative separators.
// Returns true if an alternative identifier match is found in the entry title, otherwise false.
func (s *ConfigSearcher) checkAlternativeIdentifier(
	sourceentry, entry *apiexternal_v2.Nzbwithprio,
) bool {
	altIdentifier := logger.TrimLeft(sourceentry.Info.Identifier, 's', 'S', '0')

	// Convert E/e to x format
	if strings.ContainsRune(altIdentifier, 'E') {
		logger.StringReplaceWithP(&altIdentifier, 'E', 'x')
	} else if strings.ContainsRune(altIdentifier, 'e') {
		logger.StringReplaceWithP(&altIdentifier, 'e', 'x')
	}

	if logger.ContainsI(entry.NZB.Title, altIdentifier) {
		return true
	}

	// Check alternative separators for converted identifier
	if sourceentry.NZB.Season == "" && sourceentry.NZB.Episode == "" &&
		strings.ContainsRune(sourceentry.Info.Identifier, '-') {
		if strings.ContainsRune(entry.NZB.Title, '.') &&
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', '.')) {
			return true
		}

		if strings.ContainsRune(entry.NZB.Title, ' ') &&
			logger.ContainsI(entry.NZB.Title, logger.StringReplaceWith(altIdentifier, '-', ' ')) {
			return true
		}
	}

	return false
}

// titleToWordSet converts a title string into a sorted slice of lowercase words.
// A sorted []string is cheaper than map[string]struct{} for the small word counts
// typical in titles: one slice allocation vs. map header + bucket allocations.
func titleToWordSet(title string) []string {
	words := strings.Fields(strings.ToLower(title))
	slices.Sort(words)
	return words
}

// isWordSetRedundant checks if the given word set is a subset or superset
// of any already-searched word set. If so, the search would be redundant.
func isWordSetRedundant(words []string, searchedSets [][]string) bool {
	for _, searched := range searchedSets {
		if isSubsetOrSuperset(words, searched) {
			return true
		}
	}

	return false
}

// isSubsetOrSuperset returns true if a is a subset of b, or b is a subset of a.
// Both slices must be sorted. Uses a merge-scan so no extra allocations are needed.
func isSubsetOrSuperset(a, b []string) bool {
	return sortedIsSubset(a, b) || sortedIsSubset(b, a)
}

// sortedIsSubset returns true if every element of sub appears in super.
// Both must be sorted ascending.
func sortedIsSubset(sub, super []string) bool {
	j := 0
	for _, w := range sub {
		for j < len(super) && super[j] < w {
			j++
		}
		if j >= len(super) || super[j] != w {
			return false
		}
	}
	return true
}

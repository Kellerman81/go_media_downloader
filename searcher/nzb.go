package searcher

import (
	"regexp"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
)

func filter_size_nzbs(configTemplate string, indexer *config.QualityIndexerConfig, title string, size int64) bool {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	for idx := range configEntry.DataImport {

		if indexer.Skip_empty_size && size == 0 {
			logger.Log.Debug("Skipped - Size missing: ", title)
			return true
		}
		if !config.ConfigCheck("path_" + configEntry.DataImport[idx].Template_path) {
			return false
		}
		cfg_path := config.ConfigGet("path_" + configEntry.DataImport[idx].Template_path).Data.(config.PathsConfig)

		if cfg_path.MinSize != 0 {
			if size < int64(cfg_path.MinSize*1024*1024) && size != 0 {
				logger.Log.Debug("Skipped - MinSize not matched: ", title)
				return true
			}
		}

		if cfg_path.MaxSize != 0 {
			if size > int64(cfg_path.MaxSize*1024*1024) {
				logger.Log.Debug("Skipped - MaxSize not matched: ", title)
				return true
			}
		}
	}
	return false
}
func filter_test_quality_wanted(qualityTemplate string, m parser.ParseInfo, title string) bool {
	qualityconfig := config.ConfigGet("quality_" + qualityTemplate).Data.(config.QualityConfig)
	wanted_release_resolution := false
	for idxqual := range qualityconfig.Wanted_resolution {
		if strings.EqualFold(qualityconfig.Wanted_resolution[idxqual], m.Resolution) {
			wanted_release_resolution = true
			break
		}
	}
	if len(qualityconfig.Wanted_resolution) >= 1 && !wanted_release_resolution {
		logger.Log.Debug("Skipped - unwanted resolution: ", title)
		return false
	}
	wanted_release_quality := false
	for idxqual := range qualityconfig.Wanted_quality {
		if !strings.EqualFold(qualityconfig.Wanted_quality[idxqual], m.Quality) {
			wanted_release_quality = true
			break
		}
	}
	if len(qualityconfig.Wanted_quality) >= 1 && !wanted_release_quality {
		logger.Log.Debug("Skipped - unwanted quality: ", title)
		return false
	}
	wanted_release_audio := false
	for idxqual := range qualityconfig.Wanted_audio {
		if strings.EqualFold(qualityconfig.Wanted_audio[idxqual], m.Audio) {
			wanted_release_audio = true
			break
		}
	}
	if len(qualityconfig.Wanted_audio) >= 1 && !wanted_release_audio {
		logger.Log.Debug("Skipped - unwanted audio: ", title)
		return false
	}
	wanted_release_codec := false
	for idxqual := range qualityconfig.Wanted_codec {
		if strings.EqualFold(qualityconfig.Wanted_codec[idxqual], m.Codec) {
			wanted_release_codec = true
			break
		}
	}
	if len(qualityconfig.Wanted_codec) >= 1 && !wanted_release_codec {
		logger.Log.Debug("Skipped - unwanted codec: ", title)
		return false
	}
	return true
}
func findregex(array []config.RegexGroup, find string) regexp.Regexp {
	for idx := range array {
		if array[idx].Name == find {
			return array[idx].Re
		}
	}
	return regexp.Regexp{}
}
func filter_regex_nzbs(regexconfig config.RegexConfig, title string, wantedtitle string, wantedalternates []string) (bool, string) {
	for _, rowtitle := range regexconfig.Rejected {
		if !config.RegexCheck(rowtitle) {
			continue
		}
		rowrejected := config.RegexGet(rowtitle)
		teststrwanted := rowrejected.FindStringSubmatch(wantedtitle)
		if len(teststrwanted) >= 1 {
			//Regex is in title - skip test
			continue
		}
		breakfor := false
		for idx := range wantedalternates {
			teststrwanted := rowrejected.FindStringSubmatch(wantedalternates[idx])
			if len(teststrwanted) >= 1 {
				breakfor = true
				break
			}
		}
		if breakfor {
			//Regex is in alternate title - skip test
			continue
		}
		teststr := rowrejected.FindStringSubmatch(title)
		if len(teststr) >= 1 {
			logger.Log.Debug("Skipped - Regex rejected: ", title, " Regex ", rowtitle)
			return true, rowtitle
		}
	}
	requiredmatched := false
	for _, rowtitle := range regexconfig.Required {
		if !config.RegexCheck(rowtitle) {
			continue
		}
		rowrequired := config.RegexGet(rowtitle)

		teststr := rowrequired.FindStringSubmatch(title)
		if len(teststr) >= 1 {
			logger.Log.Debug("Regex required matched: ", title, " Regex ", rowtitle)
			requiredmatched = true
			break
		}
	}
	if len(regexconfig.Required) >= 1 && !requiredmatched {
		logger.Log.Debug("Skipped - required not matched: ", title)
		return true, ""
	}
	return false, ""
}

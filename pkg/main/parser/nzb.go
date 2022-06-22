package parser

import (
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
)

type Nzbwithprio struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              apiexternal.NZB
	NzbmovieID       uint
	NzbepisodeID     uint
	WantedTitle      string
	WantedAlternates []string
	QualityTemplate  string
	MinimumPriority  int
	Denied           bool
	Reason           string
}
type NzbwithprioJson struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              apiexternal.NZB
	NzbmovieID       uint
	NzbepisodeID     uint
	WantedTitle      string
	WantedAlternates []string
	QualityTemplate  string
	MinimumPriority  int
	Denied           bool
	Reason           string
}

func (entry *Nzbwithprio) Filter_regex_nzbs(template_regex string, title string) (bool, string) {
	regexconfig := config.ConfigGet("regex_" + template_regex).Data.(config.RegexConfig)
	var teststrwanted []string
	defer logger.ClearVar(&teststrwanted)
	for idxregex := range regexconfig.Rejected {
		if !config.RegexCheck(regexconfig.Rejected[idxregex]) {
			continue
		}
		teststrwanted = config.RegexGet(regexconfig.Rejected[idxregex]).FindStringSubmatch(entry.WantedTitle)
		if len(teststrwanted) >= 1 {
			//Regex is in title - skip test
			continue
		}
		breakfor := false
		for idxwanted := range entry.WantedAlternates {
			teststrwanted = config.RegexGet(regexconfig.Rejected[idxregex]).FindStringSubmatch(entry.WantedAlternates[idxwanted])
			if len(teststrwanted) >= 1 {
				breakfor = true
				break
			}
		}
		if breakfor {
			//Regex is in alternate title - skip test
			continue
		}
		teststrwanted = config.RegexGet(regexconfig.Rejected[idxregex]).FindStringSubmatch(title)
		if len(teststrwanted) >= 1 {
			logger.Log.Debug("Skipped - Regex rejected: ", title, " Regex ", regexconfig.Rejected[idxregex])
			return true, regexconfig.Rejected[idxregex]
		}
	}
	requiredmatched := false
	var teststr []string
	defer logger.ClearVar(&teststr)
	for idxregex := range regexconfig.Required {
		if !config.RegexCheck(regexconfig.Required[idxregex]) {
			continue
		}

		teststr = config.RegexGet(regexconfig.Required[idxregex]).FindStringSubmatch(title)
		if len(teststr) >= 1 {
			//logger.Log.Debug("Regex required matched: ", title, " Regex ", regexconfig.Required[idxregex])
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
func (nzb *Nzbwithprio) Getnzbconfig(quality string) (category string, target config.PathsConfig, downloader config.DownloaderConfig) {
	if !config.ConfigCheck("quality_" + quality) {
		return
	}
	cfg_quality := config.ConfigGet("quality_" + quality).Data.(config.QualityConfig)

	var cfg_path config.PathsConfig
	var cfg_downloader config.DownloaderConfig
	for idx := range cfg_quality.Indexer {
		if strings.EqualFold(cfg_quality.Indexer[idx].Template_indexer, nzb.Indexer) {
			if !config.ConfigCheck("path_" + cfg_quality.Indexer[idx].Template_path_nzb) {
				continue
			}
			cfg_path = config.ConfigGet("path_" + cfg_quality.Indexer[idx].Template_path_nzb).Data.(config.PathsConfig)

			if !config.ConfigCheck("downloader_" + cfg_quality.Indexer[idx].Template_downloader) {
				continue
			}
			cfg_downloader = config.ConfigGet("downloader_" + cfg_quality.Indexer[idx].Template_downloader).Data.(config.DownloaderConfig)

			category = cfg_quality.Indexer[idx].Category_dowloader
			target = cfg_path
			downloader = cfg_downloader
			logger.Log.Debug("Downloader nzb config found - category: ", category)
			logger.Log.Debug("Downloader nzb config found - pathconfig: ", cfg_quality.Indexer[idx].Template_path_nzb)
			logger.Log.Debug("Downloader nzb config found - dlconfig: ", cfg_quality.Indexer[idx].Template_downloader)
			logger.Log.Debug("Downloader nzb config found - target: ", cfg_path.Path)
			logger.Log.Debug("Downloader nzb config found - downloader: ", downloader.DlType)
			logger.Log.Debug("Downloader nzb config found - downloader: ", downloader.Name)
			break
		}
	}
	if category == "" {
		logger.Log.Debug("Downloader nzb config NOT found - quality: ", quality)
		category = cfg_quality.Indexer[0].Category_dowloader

		if !config.ConfigCheck("path_" + cfg_quality.Indexer[0].Template_path_nzb) {
			return
		}
		cfg_path := config.ConfigGet("path_" + cfg_quality.Indexer[0].Template_path_nzb).Data.(config.PathsConfig)

		if !config.ConfigCheck("downloader_" + cfg_quality.Indexer[0].Template_downloader) {
			return
		}
		cfg_downloader = config.ConfigGet("downloader_" + cfg_quality.Indexer[0].Template_downloader).Data.(config.DownloaderConfig)

		target = cfg_path
		downloader = cfg_downloader
		logger.Log.Debug("Downloader nzb config NOT found - use first: ", category)
	}
	return
}

func Checknzbtitle(movietitle string, nzbtitle string) bool {
	//logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	if strings.EqualFold(movietitle, nzbtitle) {
		return true
	}
	return strings.EqualFold(logger.StringToSlug(movietitle), logger.StringToSlug(nzbtitle))
}

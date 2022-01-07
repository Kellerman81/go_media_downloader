package parser

import (
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/newznab"
)

type Nzbwithprio struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              newznab.NZB
	Nzbmovie         database.Movie
	Nzbepisode       database.SerieEpisode
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
	NZB              newznab.NZB
	Nzbmovie         database.MovieJson
	Nzbepisode       database.SerieEpisodeJson
	WantedTitle      string
	WantedAlternates []string
	Quality          config.QualityConfig
	Denied           bool
	Reason           string
}

func Getnzbconfig(nzb Nzbwithprio, quality string) (category string, target config.PathsConfig, downloader config.DownloaderConfig) {

	if !config.ConfigCheck("quality_" + quality) {
		return
	}
	cfg_quality := config.ConfigGet("quality_" + quality).Data.(config.QualityConfig)

	for idx := range cfg_quality.Indexer {
		if strings.EqualFold(cfg_quality.Indexer[idx].Template_indexer, nzb.Indexer) {
			if !config.ConfigCheck("path_" + cfg_quality.Indexer[idx].Template_path_nzb) {
				continue
			}
			cfg_path := config.ConfigGet("path_" + cfg_quality.Indexer[idx].Template_path_nzb).Data.(config.PathsConfig)

			if !config.ConfigCheck("downloader_" + cfg_quality.Indexer[idx].Template_downloader) {
				continue
			}
			cfg_downloader := config.ConfigGet("downloader_" + cfg_quality.Indexer[idx].Template_downloader).Data.(config.DownloaderConfig)

			category = cfg_quality.Indexer[idx].Category_dowloader
			target = cfg_path
			downloader = cfg_downloader
			logger.Log.Debug("Downloader nzb config found - category: ", category)
			logger.Log.Debug("Downloader nzb config found - pathconfig: ", cfg_quality.Indexer[idx].Template_path_nzb)
			logger.Log.Debug("Downloader nzb config found - dlconfig: ", cfg_quality.Indexer[idx].Template_downloader)
			logger.Log.Debug("Downloader nzb config found - target: ", cfg_path.Path)
			logger.Log.Debug("Downloader nzb config found - downloader: ", downloader.Type)
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
		cfg_downloader := config.ConfigGet("downloader_" + cfg_quality.Indexer[0].Template_downloader).Data.(config.DownloaderConfig)

		target = cfg_path
		downloader = cfg_downloader
		logger.Log.Debug("Downloader nzb config NOT found - use first: ", category)
	}
	return
}

func Checknzbtitle(movietitle string, nzbtitle string) bool {
	logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	if strings.EqualFold(movietitle, nzbtitle) {
		return true
	}
	return strings.EqualFold(logger.StringToSlug(movietitle), logger.StringToSlug(nzbtitle))
}

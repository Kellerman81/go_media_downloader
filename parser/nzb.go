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
	Quality          config.QualityConfig
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
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+quality, &cfg_quality)

	for idx := range cfg_quality.Indexer {
		if strings.EqualFold(cfg_quality.Indexer[idx].Template_indexer, nzb.Indexer) {
			if !config.ConfigCheck("path_" + cfg_quality.Indexer[idx].Template_path_nzb) {
				continue
			}
			var cfg_path config.PathsConfig
			config.ConfigGet("path_"+cfg_quality.Indexer[idx].Template_path_nzb, &cfg_path)

			if !config.ConfigCheck("downloader_" + cfg_quality.Indexer[idx].Template_downloader) {
				continue
			}
			var cfg_downloader config.DownloaderConfig
			config.ConfigGet("downloader_"+cfg_quality.Indexer[idx].Template_downloader, &cfg_downloader)

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
		var cfg_path config.PathsConfig
		config.ConfigGet("path_"+cfg_quality.Indexer[0].Template_path_nzb, &cfg_path)

		if !config.ConfigCheck("downloader_" + cfg_quality.Indexer[0].Template_downloader) {
			return
		}
		var cfg_downloader config.DownloaderConfig
		config.ConfigGet("downloader_"+cfg_quality.Indexer[0].Template_downloader, &cfg_downloader)

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
	movietitle = logger.StringToSlug(movietitle)
	nzbtitle = logger.StringToSlug(nzbtitle)
	logger.Log.Debug("check ", movietitle, " against ", nzbtitle)
	return strings.EqualFold(movietitle, nzbtitle)
}

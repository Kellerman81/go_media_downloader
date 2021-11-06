package utils

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/nzbget"
)

type Downloader struct {
	ConfigEntry      config.MediaTypeConfig
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss

	Nzb            nzbwithprio
	Movie          database.Movie
	Dbmovie        database.Dbmovie
	Serie          database.Serie
	Dbserie        database.Dbserie
	Serieepisode   database.SerieEpisode
	Dbserieepisode database.DbserieEpisode

	Category   string
	Target     config.PathsConfig
	Downloader config.DownloaderConfig

	Targetfile string
}

func NewDownloader(configEntry config.MediaTypeConfig, searchActionType string) Downloader {
	return Downloader{
		ConfigEntry:      configEntry,
		SearchActionType: searchActionType,
	}
}
func (d *Downloader) SetMovie(movie database.Movie) {
	d.SearchGroupType = "movie"
	dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})

	d.Dbmovie = dbmovie
	d.Movie = movie
	logger.Log.Debug("Downloader movie: ", movie)
	logger.Log.Debug("Downloader movie quality: ", movie.QualityProfile)
	d.Quality = movie.QualityProfile
}
func (d *Downloader) SetSeriesEpisode(seriesepisode database.SerieEpisode) {
	d.SearchGroupType = "series"
	dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{seriesepisode.DbserieID}})
	dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{seriesepisode.DbserieEpisodeID}})
	serie, _ := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{seriesepisode.SerieID}})
	d.Dbserie = dbserie
	d.Serie = serie
	d.Serieepisode = seriesepisode
	d.Quality = seriesepisode.QualityProfile
	d.Dbserieepisode = dbserieepisode
}
func (d *Downloader) DownloadNzb(nzb nzbwithprio) {
	d.Nzb = nzb
	d.Category, d.Target, d.Downloader = getnzbconfig(d.Nzb, d.Quality)

	targetfolder := ""
	if d.SearchGroupType == "movie" {
		if d.Dbmovie.ImdbID != "" {
			targetfolder = Path(d.Nzb.NZB.Title+" ("+d.Dbmovie.ImdbID+")", false)
		} else if d.Nzb.NZB.IMDBID != "" {
			if !strings.Contains(d.Nzb.NZB.IMDBID, "tt") {
				nzb.NZB.IMDBID = "tt" + d.Nzb.NZB.IMDBID
			}
			if nzb.NZB.Title == "" {
				targetfolder = Path(nzb.ParseInfo.Title+"["+nzb.ParseInfo.Resolution+" "+nzb.ParseInfo.Quality+"]"+" ("+nzb.NZB.IMDBID+")", false)
			} else {
				targetfolder = Path(nzb.NZB.Title+" ("+nzb.NZB.IMDBID+")", false)
			}
		} else {
			targetfolder = Path(nzb.NZB.Title, false)
		}
	} else {
		if d.Dbserie.ThetvdbID != 0 {
			if d.Nzb.NZB.Title == "" {
				targetfolder = Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+" (tvdb"+strconv.Itoa(d.Dbserie.ThetvdbID)+")", false)
			} else {
				targetfolder = Path(d.Nzb.NZB.Title+" (tvdb"+strconv.Itoa(d.Dbserie.ThetvdbID)+")", false)
			}
		} else if d.Nzb.NZB.TVDBID != "" {
			if d.Nzb.NZB.Title == "" {
				targetfolder = Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+" (tvdb"+d.Nzb.NZB.TVDBID+")", false)
			} else {
				targetfolder = Path(d.Nzb.NZB.Title+" (tvdb"+d.Nzb.NZB.TVDBID+")", false)
			}
		} else {
			if d.Nzb.NZB.Title == "" {
				targetfolder = Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]", false)
			} else {
				targetfolder = Path(d.Nzb.NZB.Title, false)
			}
		}
	}
	targetfolder = strings.Replace(targetfolder, "[", "", -1)
	targetfolder = strings.Replace(targetfolder, "]", "", -1)
	d.Targetfile = targetfolder

	logger.Log.Debug("Downloader target folder: ", targetfolder)
	logger.Log.Debug("Downloader target type: ", d.Downloader.Type)
	var err error
	switch strings.ToLower(d.Downloader.Type) {
	case "drone":
		err = d.DownloadByDrone()
	case "nzbget":
		err = d.DownloadByNzbget()
	case "deluge":
		err = d.DownloadByDeluge()
	default:
		return
	}
	if err == nil {
		d.Notify()
	}
	d.History()
}
func (d Downloader) DownloadByDrone() error {
	logger.Log.Debug("Download by Drone: ", d.Nzb.NZB.DownloadURL)
	return downloadFile(d.Target.Path, "", d.Targetfile+".nzb", d.Nzb.NZB.DownloadURL)
}
func (d Downloader) DownloadByNzbget() error {
	logger.Log.Debug("Download by Nzbget: ", d.Nzb.NZB.DownloadURL)
	url := "http://" + d.Downloader.Username + ":" + d.Downloader.Password + "@" + d.Downloader.Hostname + "/jsonrpc"
	logger.Log.Debug("Download by Nzbget: ", url)
	nzbcl := nzbget.NewClient(url)
	options := nzbget.NewOptions()
	options.Category = d.Category
	options.AddPaused = d.Downloader.AddPaused
	options.Priority = d.Downloader.Priority
	options.NiceName = d.Targetfile + ".nzb"
	_, err := nzbcl.Add(d.Nzb.NZB.DownloadURL, options)
	if err != nil {
		logger.Log.Error("Download by Nzbget - ERROR: ", err)
	}
	return err
}
func (d Downloader) DownloadByDeluge() error {
	logger.Log.Debug("Download by Deluge: ", d.Nzb.NZB.DownloadURL)

	err := apiexternal.SendToDeluge(
		d.Downloader.Hostname, d.Downloader.Port, d.Downloader.Username, d.Downloader.Password,
		d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, d.Downloader.DelugeMoveAfter, d.Downloader.DelugeMoveTo)

	if err != nil {
		logger.Log.Error("Download by Deluge - ERROR: ", err)
	}
	return err
}

func (d Downloader) Notify() {
	for idxnoti := range d.ConfigEntry.Notification {
		notifier("added_download", d.ConfigEntry.Notification[idxnoti], InputNotifier{
			Targetpath: d.Category + "/" + d.Targetfile + ".nzb",
			SourcePath: d.Nzb.NZB.DownloadURL,
			Title:      d.Nzb.NZB.Title,
			Season:     d.Nzb.NZB.Season,
			Episode:    d.Nzb.NZB.Episode,
			Series:     d.Nzb.ParseInfo.Title,
			Identifier: d.Nzb.ParseInfo.Identifier,
			Tvdb:       d.Nzb.NZB.TVDBID,
			Imdb:       d.Nzb.NZB.IMDBID,
		})
	}
}

func (d Downloader) History() {
	if strings.EqualFold(d.SearchGroupType, "movie") {
		movieID := d.Movie.ID
		if movieID == 0 {
			movieID = d.Nzb.Nzbmovie.ID
		}
		dbmovieID := d.Movie.DbmovieID
		if dbmovieID == 0 {
			dbmovieID = d.Nzb.Nzbmovie.DbmovieID
		}

		database.InsertArray("movie_histories",
			[]string{"title", "url", "target", "indexer", "downloaded_at", "movie_id", "dbmovie_id", "resolution_id", "quality_id", "codec_id", "audio_id"},
			[]interface{}{d.Nzb.NZB.Title, d.Nzb.NZB.DownloadURL, d.Target.Path, d.Nzb.Indexer, time.Now(), movieID, dbmovieID, d.Nzb.ParseInfo.ResolutionID, d.Nzb.ParseInfo.QualityID, d.Nzb.ParseInfo.CodecID, d.Nzb.ParseInfo.AudioID})
	} else {
		serieid := d.Serie.ID
		if serieid == 0 {
			serieid = d.Nzb.Nzbepisode.SerieID
		}
		dbserieid := d.Dbserie.ID
		if dbserieid == 0 {
			dbserieid = d.Nzb.Nzbepisode.DbserieID
		}
		serieepisodeid := d.Serieepisode.ID
		if serieepisodeid == 0 {
			serieepisodeid = d.Nzb.Nzbepisode.ID
		}
		dbserieepisodeid := d.Dbserieepisode.ID
		if dbserieepisodeid == 0 {
			dbserieepisodeid = d.Nzb.Nzbepisode.DbserieEpisodeID
		}

		database.InsertArray("serie_episode_histories",
			[]string{"title", "url", "target", "indexer", "downloaded_at", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id", "resolution_id", "quality_id", "codec_id", "audio_id"},
			[]interface{}{d.Nzb.NZB.Title, d.Nzb.NZB.DownloadURL, d.Target.Path, d.Nzb.Indexer, time.Now(), serieid, serieepisodeid, dbserieepisodeid, dbserieid, d.Nzb.ParseInfo.ResolutionID, d.Nzb.ParseInfo.QualityID, d.Nzb.ParseInfo.CodecID, d.Nzb.ParseInfo.AudioID})
	}
}

func getnzbconfig(nzb nzbwithprio, quality string) (category string, target config.PathsConfig, downloader config.DownloaderConfig) {

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

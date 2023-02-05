package downloader

import (
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/nzbget"
	"go.uber.org/zap"
)

type downloadertype struct {
	Cfgp            *config.MediaTypeConfig
	Quality         string
	SearchGroupType string //series, movies
	//SearchActionType string //missing,upgrade,rss

	Nzb            apiexternal.Nzbwithprio
	Movie          database.Movie
	Dbmovie        database.Dbmovie
	Serie          database.Serie
	Dbserie        database.Dbserie
	Serieepisode   database.SerieEpisode
	Dbserieepisode database.DbserieEpisode

	Category      string
	Target        string
	Downloader    string
	cfgDownloader config.DownloaderConfig
	Targetfile    string
	Time          string
}

const strTvdbid = " (tvdb"

func (d *downloadertype) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if d == nil {
		return
	}
	d.Movie.Close()
	d.Dbmovie.Close()
	d.Serie.Close()
	d.Dbserie.Close()
	d.Serieepisode.Close()
	d.Dbserieepisode.Close()
	d.Nzb.ParseInfo.Close()
	d.Nzb.Close()
	d.cfgDownloader.Close()
	if len(d.Nzb.WantedAlternates) >= 1 {
		d.Nzb.WantedAlternates = nil
	}
	d = nil
}

func DownloadMovie(cfgp *config.MediaTypeConfig, movieid uint, nzb *apiexternal.Nzbwithprio) {
	d := &downloadertype{
		Cfgp:            cfgp,
		SearchGroupType: "movie",
		Nzb:             *nzb,
	}
	if database.GetMovies(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{movieid}}, &d.Movie) != nil {
		d.Close()
		return
	}
	if database.GetDbmovie(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{d.Movie.DbmovieID}}, &d.Dbmovie) != nil {
		d.Close()
		return
	}
	d.Quality = d.Movie.QualityProfile
	d.downloadNzb()
	d.Close()
}
func DownloadSeriesEpisode(cfgp *config.MediaTypeConfig, episodeid uint, nzb *apiexternal.Nzbwithprio) {
	d := &downloadertype{
		Cfgp:            cfgp,
		SearchGroupType: "series",
		Nzb:             *nzb,
	}
	if database.GetSerieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{episodeid}}, &d.Serieepisode) != nil {
		d.Close()
		return
	}
	if database.GetDbserie(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{d.Serieepisode.DbserieID}}, &d.Dbserie) != nil {
		d.Close()
		return
	}
	if database.GetDbserieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{d.Serieepisode.DbserieEpisodeID}}, &d.Dbserieepisode) != nil {
		d.Close()
		return
	}
	if database.GetSeries(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{d.Serieepisode.SerieID}}, &d.Serie) != nil {
		d.Close()
		return
	}
	d.Quality = d.Serieepisode.QualityProfile
	d.downloadNzb()
	d.Close()
}
func (d *downloadertype) downloadNzb() {
	d.Category, d.Target, d.Downloader = d.Nzb.Getnzbconfig(d.Quality)
	d.cfgDownloader = config.Cfg.Downloader[d.Downloader]
	var targetfolder, historytable string
	if d.SearchGroupType == "movie" {
		historytable = "movie_histories"
		if d.Dbmovie.ImdbID != "" {
			targetfolder = logger.Path(d.Nzb.NZB.Title+" ("+d.Dbmovie.ImdbID+")", false)
		} else if d.Nzb.NZB.IMDBID != "" {
			if !strings.Contains(d.Nzb.NZB.IMDBID, "tt") {
				d.Nzb.NZB.IMDBID = "tt" + d.Nzb.NZB.IMDBID
			}
			if d.Nzb.NZB.Title == "" {
				targetfolder = logger.Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+" ("+d.Nzb.NZB.IMDBID+")", false)
			} else {
				targetfolder = logger.Path(d.Nzb.NZB.Title+" ("+d.Nzb.NZB.IMDBID+")", false)
			}
		} else {
			targetfolder = logger.Path(d.Nzb.NZB.Title, false)
		}
	} else {
		historytable = "serie_episode_histories"
		if d.Dbserie.ThetvdbID != 0 {
			if d.Nzb.NZB.Title == "" {
				targetfolder = logger.Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+strTvdbid+logger.IntToString(d.Dbserie.ThetvdbID)+")", false)
			} else {
				targetfolder = logger.Path(d.Nzb.NZB.Title+strTvdbid+logger.IntToString(d.Dbserie.ThetvdbID)+")", false)
			}
		} else if d.Nzb.NZB.TVDBID != 0 {
			if d.Nzb.NZB.Title == "" {
				targetfolder = logger.Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+strTvdbid+logger.IntToString(d.Nzb.NZB.TVDBID)+")", false)
			} else {
				targetfolder = logger.Path(d.Nzb.NZB.Title+strTvdbid+logger.IntToString(d.Nzb.NZB.TVDBID)+")", false)
			}
		} else {
			if d.Nzb.NZB.Title == "" {
				targetfolder = logger.Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]", false)
			} else {
				targetfolder = logger.Path(d.Nzb.NZB.Title, false)
			}
		}
	}
	logger.GlobalCache.Delete(historytable + "_url")
	logger.GlobalCache.Delete(historytable + "_title")
	targetfolder = strings.ReplaceAll(targetfolder, "[", "")
	targetfolder = strings.ReplaceAll(targetfolder, "]", "")
	d.Targetfile = targetfolder

	logger.Log.GlobalLogger.Debug("Downloading", zap.Any("nzb", d.Nzb), zap.String("by", d.cfgDownloader.DlType))

	var err error
	switch d.cfgDownloader.DlType {
	case "drone":
		err = d.downloadByDrone()
	case "nzbget":
		err = d.downloadByNzbget()
	case "sabnzbd":
		err = d.downloadBySabnzbd()
	case "transmission":
		err = d.downloadByTransmission()
	case "rtorrent":
		err = d.downloadByRTorrent()
	case "qbittorrent":
		err = d.downloadByQBittorrent()
	case "deluge":
		err = d.downloadByDeluge()
	default:
		return
	}
	if err == nil {
		d.notify()
	}

	if strings.EqualFold(d.SearchGroupType, "movie") {
		movieID := d.Movie.ID
		moviequality := d.Movie.QualityProfile
		if movieID == 0 {
			movieID = d.Nzb.NzbmovieID
		}
		if movieID != 0 && moviequality == "" {
			database.QueryColumn(&database.Querywithargs{QueryString: "select quality_profile from movies where id = ?", Args: []interface{}{d.Nzb.NzbmovieID}}, &moviequality)
		}
		dbmovieID := d.Movie.DbmovieID
		if dbmovieID == 0 && d.Nzb.NzbmovieID != 0 {
			database.QueryColumn(&database.Querywithargs{QueryString: "select dbmovie_id from movies where id = ?", Args: []interface{}{d.Nzb.NzbmovieID}}, &dbmovieID)
		}

		database.InsertNamed("Insert into movie_histories (title, url, target, indexer, downloaded_at, movie_id, dbmovie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (:title, :url, :target, :indexer, :downloaded_at, :movie_id, :dbmovie_id, :resolution_id, :quality_id, :codec_id, :audio_id, :quality_profile)",
			database.MovieHistory{
				Title:          d.Nzb.NZB.Title,
				URL:            d.Nzb.NZB.DownloadURL,
				Target:         config.Cfg.Paths[d.Target].Path,
				Indexer:        d.Nzb.Indexer,
				DownloadedAt:   logger.TimeGetNow(),
				MovieID:        movieID,
				DbmovieID:      dbmovieID,
				ResolutionID:   d.Nzb.ParseInfo.ResolutionID,
				QualityID:      d.Nzb.ParseInfo.QualityID,
				CodecID:        d.Nzb.ParseInfo.CodecID,
				AudioID:        d.Nzb.ParseInfo.AudioID,
				QualityProfile: moviequality,
			})
		return
	}
	serieid := d.Serie.ID
	if serieid == 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: "select serie_id from serie_episodes where id = ?", Args: []interface{}{d.Nzb.NzbepisodeID}}, &serieid)
	}
	dbserieid := d.Dbserie.ID
	if dbserieid == 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: "select dbserie_id from serie_episodes where id = ?", Args: []interface{}{d.Nzb.NzbepisodeID}}, &dbserieid)
	}
	serieepisodeid := d.Serieepisode.ID
	serieepisodequality := d.Serieepisode.QualityProfile
	if serieepisodeid == 0 {
		serieepisodeid = d.Nzb.NzbepisodeID
	}
	if serieepisodequality == "" {
		database.QueryColumn(&database.Querywithargs{QueryString: "select quality_profile from serie_episodes where id = ?", Args: []interface{}{d.Nzb.NzbepisodeID}}, &serieepisodequality)
	}
	dbserieepisodeid := d.Dbserieepisode.ID
	if dbserieepisodeid == 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: "select dbserie_episode_id from serie_episodes where id = ?", Args: []interface{}{d.Nzb.NzbepisodeID}}, &dbserieepisodeid)
	}

	database.InsertNamed("Insert into serie_episode_histories (title, url, target, indexer, downloaded_at, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (:title, :url, :target, :indexer, :downloaded_at, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :resolution_id, :quality_id, :codec_id, :audio_id, :quality_profile)",
		database.SerieEpisodeHistory{
			Title:            d.Nzb.NZB.Title,
			URL:              d.Nzb.NZB.DownloadURL,
			Target:           config.Cfg.Paths[d.Target].Path,
			Indexer:          d.Nzb.Indexer,
			DownloadedAt:     logger.TimeGetNow(),
			SerieID:          serieid,
			SerieEpisodeID:   serieepisodeid,
			DbserieEpisodeID: dbserieepisodeid,
			DbserieID:        dbserieid,
			ResolutionID:     d.Nzb.ParseInfo.ResolutionID,
			QualityID:        d.Nzb.ParseInfo.QualityID,
			CodecID:          d.Nzb.ParseInfo.CodecID,
			AudioID:          d.Nzb.ParseInfo.AudioID,
			QualityProfile:   serieepisodequality,
		})
}

func (d *downloadertype) notify() {
	d.Time = logger.TimeGetNow().Format(logger.TimeFormat)
	var f *os.File
	var messagetext, messageTitle string
	var err error
	for idx := range d.Cfgp.Notification {
		if !strings.EqualFold(d.Cfgp.Notification[idx].Event, "added_download") {
			continue
		}
		messagetext, err = logger.ParseStringTemplate(d.Cfgp.Notification[idx].Message, d)
		if err != nil {
			continue
		}
		messageTitle, err = logger.ParseStringTemplate(d.Cfgp.Notification[idx].Title, d)
		if err != nil {
			continue
		}

		if !config.Check("notification_" + d.Cfgp.Notification[idx].MapNotification) {
			continue
		}

		if strings.EqualFold(config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].NotificationType, "pushover") {
			if apiexternal.PushoverAPI == nil {
				apiexternal.NewPushOverClient(config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].Apikey)
			}
			if apiexternal.PushoverAPI.APIKey != config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].Apikey {
				apiexternal.NewPushOverClient(config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].Apikey)
			}

			err = apiexternal.PushoverAPI.SendMessage(messagetext, messageTitle, config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].Recipient)
			if err != nil {
				logger.Log.GlobalLogger.Error("Error sending pushover", zap.Error(err))
			} else {
				logger.Log.GlobalLogger.Info("Pushover message sent")
			}
		}
		if strings.EqualFold(config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].NotificationType, "csv") {
			f, err = os.OpenFile(config.Cfg.Notification[d.Cfgp.Notification[idx].MapNotification].Outputto,
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logger.Log.GlobalLogger.Error("Error opening csv to write", zap.Error(err))
				continue
			}
			_, err = io.WriteString(f, messagetext+"\n")
			if err != nil {
				logger.Log.GlobalLogger.Error("Error writing to csv", zap.Error(err))
			} else {
				logger.Log.GlobalLogger.Info("csv written")
			}
			f.Close()
		}
	}
}
func (d *downloadertype) downloadByDrone() error {
	logger.Log.GlobalLogger.Info("Download by Drone", zap.Stringp("title", &d.Nzb.NZB.Title), zap.Stringp("url", &d.Nzb.NZB.DownloadURL))
	filename := d.Targetfile + ".nzb"
	if d.Nzb.NZB.IsTorrent {
		filename = d.Targetfile + ".torrent"
	}
	return logger.DownloadFile(config.Cfg.Paths[d.Target].Path, "", filename, d.Nzb.NZB.DownloadURL)
}
func (d *downloadertype) downloadByNzbget() error {
	url := "http://" + d.cfgDownloader.Username + ":" + d.cfgDownloader.Password + "@" + d.cfgDownloader.Hostname + "/jsonrpc"
	options := nzbget.NewOptions()

	options.Category = d.Category
	options.AddPaused = d.cfgDownloader.AddPaused
	options.Priority = d.cfgDownloader.Priority
	options.NiceName = d.Targetfile + ".nzb"
	_, err := nzbget.NewClient(url).Add(d.Nzb.NZB.DownloadURL, options)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by Nzbget - ERROR", zap.Error(err))
	}
	return err
}
func (d *downloadertype) downloadBySabnzbd() error {
	logger.Log.GlobalLogger.Info("Download by Sabnzbd", zap.Stringp("title", &d.Nzb.NZB.Title), zap.Stringp("url", &d.Nzb.NZB.DownloadURL))
	err := apiexternal.SendToSabnzbd(d.cfgDownloader.Hostname, d.cfgDownloader.Password, d.Nzb.NZB.DownloadURL, d.Category, d.Targetfile, d.cfgDownloader.Priority)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by Sabnzbd - ERROR", zap.Error(err))
	}
	return err
}
func (d *downloadertype) downloadByRTorrent() error {
	logger.Log.GlobalLogger.Info("Download by rTorrent", zap.Stringp("title", &d.Nzb.NZB.Title), zap.Stringp("url", &d.Nzb.NZB.DownloadURL))
	err := apiexternal.SendToRtorrent(d.cfgDownloader.Hostname, false, d.Nzb.NZB.DownloadURL, d.cfgDownloader.DelugeDlTo, d.Targetfile)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by rTorrent - ERROR", zap.Error(err))
	}
	return err
}
func (d *downloadertype) downloadByTransmission() error {
	logger.Log.GlobalLogger.Info("Download by transmission", zap.Stringp("title", &d.Nzb.NZB.Title), zap.Stringp("url", &d.Nzb.NZB.DownloadURL))
	err := apiexternal.SendToTransmission(d.cfgDownloader.Hostname, d.cfgDownloader.Username, d.cfgDownloader.Password, d.Nzb.NZB.DownloadURL, d.cfgDownloader.DelugeDlTo, d.cfgDownloader.AddPaused)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by transmission - ERROR", zap.Error(err))
	}
	return err
}

func (d *downloadertype) downloadByDeluge() error {
	logger.Log.GlobalLogger.Info("Download by Deluge", zap.Stringp("title", &d.Nzb.NZB.Title), zap.Stringp("url", &d.Nzb.NZB.DownloadURL))

	err := apiexternal.SendToDeluge(
		d.cfgDownloader.Hostname, d.cfgDownloader.Port, d.cfgDownloader.Username, d.cfgDownloader.Password,
		d.Nzb.NZB.DownloadURL, d.cfgDownloader.DelugeDlTo, d.cfgDownloader.DelugeMoveAfter, d.cfgDownloader.DelugeMoveTo, d.cfgDownloader.AddPaused)

	if err != nil {
		logger.Log.GlobalLogger.Error("Download by Deluge - ERROR", zap.Error(err))
	}
	return err
}
func (d *downloadertype) downloadByQBittorrent() error {
	logger.Log.GlobalLogger.Info("Download by qBittorrent", zap.Stringp("title", &d.Nzb.NZB.Title), zap.Stringp("url", &d.Nzb.NZB.DownloadURL))

	err := apiexternal.SendToQBittorrent(
		d.cfgDownloader.Hostname, logger.IntToString(d.cfgDownloader.Port), d.cfgDownloader.Username, d.cfgDownloader.Password,
		d.Nzb.NZB.DownloadURL, d.cfgDownloader.DelugeDlTo, strconv.FormatBool(d.cfgDownloader.AddPaused))

	if err != nil {
		logger.Log.GlobalLogger.Error("Download by qBittorrent - ERROR", zap.Error(err))
	}
	return err
}

package downloader

import (
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/nzbget"
	"go.uber.org/zap"
)

type Downloadertype struct {
	Cfg             string
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

	Category   string
	Target     string
	Downloader string

	Targetfile string
	Time       string
}

func (d *Downloadertype) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if d != nil {
		d.Nzb.ParseInfo.Close()
		d.Nzb.Close()
		if len(d.Nzb.WantedAlternates) >= 1 {
			d.Nzb.WantedAlternates = nil
		}
		d = nil
	}
}
func NewDownloader(cfg string) *Downloadertype {
	return &Downloadertype{
		Cfg: cfg,
	}
}
func (d *Downloadertype) SetMovie(movieid uint) {
	d.SearchGroupType = "movie"
	var err error
	d.Movie, err = database.GetMovies(&database.Query{Where: "id = ?"}, movieid)
	if err != nil {
		return
	}
	d.Dbmovie, err = database.GetDbmovie(&database.Query{Where: "id = ?"}, d.Movie.DbmovieID)
	if err != nil {
		return
	}
	logger.Log.GlobalLogger.Debug("Downloader movie quality", zap.String("Profile", d.Movie.QualityProfile))
	d.Quality = d.Movie.QualityProfile
}
func (d *Downloadertype) SetSeriesEpisode(episodeid uint) {
	d.SearchGroupType = "series"
	var err error
	d.Serieepisode, err = database.GetSerieEpisodes(&database.Query{Where: "id = ?"}, episodeid)
	if err != nil {
		return
	}
	d.Dbserie, err = database.GetDbserie(&database.Query{Where: "id = ?"}, d.Serieepisode.DbserieID)
	if err != nil {
		return
	}
	d.Dbserieepisode, err = database.GetDbserieEpisodes(&database.Query{Where: "id = ?"}, d.Serieepisode.DbserieEpisodeID)
	if err != nil {
		return
	}
	d.Serie, err = database.GetSeries(&database.Query{Where: "id = ?"}, d.Serieepisode.SerieID)
	if err != nil {
		return
	}
	d.Quality = d.Serieepisode.QualityProfile
}
func (d *Downloadertype) DownloadNzb() {
	d.Category, d.Target, d.Downloader = d.Nzb.Getnzbconfig(d.Quality)
	targetfolder := ""
	historytable := ""
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
				targetfolder = logger.Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+" (tvdb"+strconv.Itoa(d.Dbserie.ThetvdbID)+")", false)
			} else {
				targetfolder = logger.Path(d.Nzb.NZB.Title+" (tvdb"+strconv.Itoa(d.Dbserie.ThetvdbID)+")", false)
			}
		} else if d.Nzb.NZB.TVDBID != "" {
			if d.Nzb.NZB.Title == "" {
				targetfolder = logger.Path(d.Nzb.ParseInfo.Title+"["+d.Nzb.ParseInfo.Resolution+" "+d.Nzb.ParseInfo.Quality+"]"+" (tvdb"+d.Nzb.NZB.TVDBID+")", false)
			} else {
				targetfolder = logger.Path(d.Nzb.NZB.Title+" (tvdb"+d.Nzb.NZB.TVDBID+")", false)
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
	targetfolder = strings.Replace(targetfolder, "[", "", -1)
	targetfolder = strings.Replace(targetfolder, "]", "", -1)
	d.Targetfile = targetfolder

	logger.Log.GlobalLogger.Debug("Downloader target folder", zap.String("path", targetfolder))
	logger.Log.GlobalLogger.Debug("Downloader target type", zap.String("type", config.Cfg.Downloader[d.Downloader].DlType))
	logger.Log.GlobalLogger.Debug("Downloader debug priority minimum", zap.Int("priority", d.Nzb.MinimumPriority))
	logger.Log.GlobalLogger.Debug("Downloader debug priority found", zap.Int("priority", d.Nzb.ParseInfo.Priority))
	logger.Log.GlobalLogger.Debug("Downloader debug quality", zap.String("Quality", d.Nzb.QualityTemplate))
	var err error
	switch config.Cfg.Downloader[d.Downloader].DlType {
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
	d.history()
}
func (d *Downloadertype) downloadByDrone() error {
	logger.Log.GlobalLogger.Info("Download by Drone", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))
	filename := d.Targetfile + ".nzb"
	if d.Nzb.NZB.IsTorrent {
		filename = d.Targetfile + ".torrent"
	}
	return logger.DownloadFile(config.Cfg.Paths[d.Target].Path, "", filename, d.Nzb.NZB.DownloadURL)
}
func (d *Downloadertype) downloadByNzbget() error {
	logger.Log.GlobalLogger.Info("Download by Nzbget", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))
	url := "http://" + config.Cfg.Downloader[d.Downloader].Username + ":" + config.Cfg.Downloader[d.Downloader].Password + "@" + config.Cfg.Downloader[d.Downloader].Hostname + "/jsonrpc"
	logger.Log.GlobalLogger.Debug("Download by Nzbget", zap.String("url", url))
	nzbcl := nzbget.NewClient(url)
	options := nzbget.NewOptions()

	options.Category = d.Category
	options.AddPaused = config.Cfg.Downloader[d.Downloader].AddPaused
	options.Priority = config.Cfg.Downloader[d.Downloader].Priority
	options.NiceName = d.Targetfile + ".nzb"
	_, err := nzbcl.Add(d.Nzb.NZB.DownloadURL, options)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by Nzbget - ERROR", zap.Error(err))
	}
	return err
}
func (d *Downloadertype) downloadBySabnzbd() error {
	logger.Log.GlobalLogger.Info("Download by Sabnzbd", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))
	err := apiexternal.SendToSabnzbd(config.Cfg.Downloader[d.Downloader].Hostname, config.Cfg.Downloader[d.Downloader].Password, d.Nzb.NZB.DownloadURL, d.Category, d.Targetfile, config.Cfg.Downloader[d.Downloader].Priority)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by Sabnzbd - ERROR", zap.Error(err))
	}
	return err
}
func (d *Downloadertype) downloadByRTorrent() error {
	logger.Log.GlobalLogger.Info("Download by rTorrent", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))
	err := apiexternal.SendToRtorrent(config.Cfg.Downloader[d.Downloader].Hostname, false, d.Nzb.NZB.DownloadURL, config.Cfg.Downloader[d.Downloader].DelugeDlTo, d.Targetfile)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by rTorrent - ERROR", zap.Error(err))
	}
	return err
}
func (d *Downloadertype) downloadByTransmission() error {
	logger.Log.GlobalLogger.Info("Download by transmission", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))
	err := apiexternal.SendToTransmission(config.Cfg.Downloader[d.Downloader].Hostname, config.Cfg.Downloader[d.Downloader].Username, config.Cfg.Downloader[d.Downloader].Password, d.Nzb.NZB.DownloadURL, config.Cfg.Downloader[d.Downloader].DelugeDlTo, config.Cfg.Downloader[d.Downloader].AddPaused)
	if err != nil {
		logger.Log.GlobalLogger.Error("Download by transmission - ERROR", zap.Error(err))
	}
	return err
}

func (d *Downloadertype) downloadByDeluge() error {
	logger.Log.GlobalLogger.Info("Download by Deluge", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))

	err := apiexternal.SendToDeluge(
		config.Cfg.Downloader[d.Downloader].Hostname, config.Cfg.Downloader[d.Downloader].Port, config.Cfg.Downloader[d.Downloader].Username, config.Cfg.Downloader[d.Downloader].Password,
		d.Nzb.NZB.DownloadURL, config.Cfg.Downloader[d.Downloader].DelugeDlTo, config.Cfg.Downloader[d.Downloader].DelugeMoveAfter, config.Cfg.Downloader[d.Downloader].DelugeMoveTo, config.Cfg.Downloader[d.Downloader].AddPaused)

	if err != nil {
		logger.Log.GlobalLogger.Error("Download by Deluge - ERROR", zap.Error(err))
	}
	return err
}
func (d *Downloadertype) downloadByQBittorrent() error {
	logger.Log.GlobalLogger.Info("Download by qBittorrent", zap.String("title", d.Nzb.NZB.Title), zap.String("url", d.Nzb.NZB.DownloadURL))

	err := apiexternal.SendToQBittorrent(
		config.Cfg.Downloader[d.Downloader].Hostname, strconv.Itoa(config.Cfg.Downloader[d.Downloader].Port), config.Cfg.Downloader[d.Downloader].Username, config.Cfg.Downloader[d.Downloader].Password,
		d.Nzb.NZB.DownloadURL, config.Cfg.Downloader[d.Downloader].DelugeDlTo, strconv.FormatBool(config.Cfg.Downloader[d.Downloader].AddPaused))

	if err != nil {
		logger.Log.GlobalLogger.Error("Download by qBittorrent - ERROR", zap.Error(err))
	}
	return err
}

func (d *Downloadertype) sendNotify(event string, noticonfig *config.MediaNotificationConfig) {
	if !strings.EqualFold(noticonfig.Event, event) {
		return
	}
	messagetext, err := logger.ParseStringTemplate(noticonfig.Message, d)
	if err != nil {
		return
	}
	messageTitle, err := logger.ParseStringTemplate(noticonfig.Title, d)
	if err != nil {
		return
	}

	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}

	if strings.EqualFold(config.Cfg.Notification[noticonfig.Map_notification].NotificationType, "pushover") {
		if apiexternal.PushoverApi == nil {
			apiexternal.NewPushOverClient(config.Cfg.Notification[noticonfig.Map_notification].Apikey)
		}
		if apiexternal.PushoverApi.ApiKey != config.Cfg.Notification[noticonfig.Map_notification].Apikey {
			apiexternal.NewPushOverClient(config.Cfg.Notification[noticonfig.Map_notification].Apikey)
		}

		err := apiexternal.PushoverApi.SendMessage(messagetext, messageTitle, config.Cfg.Notification[noticonfig.Map_notification].Recipient)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error sending pushover", zap.Error(err))
		} else {
			logger.Log.GlobalLogger.Info("Pushover message sent")
		}
	}
	if strings.EqualFold(config.Cfg.Notification[noticonfig.Map_notification].NotificationType, "csv") {
		f, errf := os.OpenFile(config.Cfg.Notification[noticonfig.Map_notification].Outputto,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if errf != nil {
			logger.Log.GlobalLogger.Error("Error opening csv to write", zap.Error(errf))
			return
		}
		defer f.Close()
		if errf == nil {
			_, errc := io.WriteString(f, messagetext+"\n")
			if errc != nil {
				logger.Log.GlobalLogger.Error("Error writing to csv", zap.Error(errc))
			} else {
				logger.Log.GlobalLogger.Info("csv written")
			}
		}
	}
}
func (d *Downloadertype) notify() {
	d.Time = time.Now().In(logger.TimeZone).Format(logger.TimeFormat)
	for idx := range config.Cfg.Media[d.Cfg].Notification {
		d.sendNotify("added_download", &config.Cfg.Media[d.Cfg].Notification[idx])
	}
}

func (d *Downloadertype) history() {
	if strings.EqualFold(d.SearchGroupType, "movie") {
		movieID := d.Movie.ID
		moviequality := d.Movie.QualityProfile
		if movieID == 0 {
			movieID = d.Nzb.NzbmovieID
			moviequality, _ = database.QueryColumnString("select quality_profile from movies where id = ?", d.Nzb.NzbmovieID)
		}
		dbmovieID := d.Movie.DbmovieID
		if dbmovieID == 0 {
			dbmovieID, _ = database.QueryColumnUint("select dbmovie_id from movies where id = ?", d.Nzb.NzbmovieID)
		}

		database.InsertNamed("Insert into movie_histories (title, url, target, indexer, downloaded_at, movie_id, dbmovie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (:title, :url, :target, :indexer, :downloaded_at, :movie_id, :dbmovie_id, :resolution_id, :quality_id, :codec_id, :audio_id, :quality_profile)",
			database.MovieHistory{
				Title:          d.Nzb.NZB.Title,
				URL:            d.Nzb.NZB.DownloadURL,
				Target:         config.Cfg.Paths[d.Target].Path,
				Indexer:        d.Nzb.Indexer,
				DownloadedAt:   time.Now().In(logger.TimeZone),
				MovieID:        movieID,
				DbmovieID:      dbmovieID,
				ResolutionID:   d.Nzb.ParseInfo.ResolutionID,
				QualityID:      d.Nzb.ParseInfo.QualityID,
				CodecID:        d.Nzb.ParseInfo.CodecID,
				AudioID:        d.Nzb.ParseInfo.AudioID,
				QualityProfile: moviequality,
			})
	} else {
		serieid := d.Serie.ID
		if serieid == 0 {
			serieid, _ = database.QueryColumnUint("select serie_id from serie_episodes where id = ?", d.Nzb.NzbepisodeID)
		}
		dbserieid := d.Dbserie.ID
		if dbserieid == 0 {
			dbserieid, _ = database.QueryColumnUint("select dbserie_id from serie_episodes where id = ?", d.Nzb.NzbepisodeID)
		}
		serieepisodeid := d.Serieepisode.ID
		serieepisodequality := d.Serieepisode.QualityProfile
		if serieepisodeid == 0 {
			serieepisodeid = d.Nzb.NzbepisodeID
			serieepisodequality, _ = database.QueryColumnString("select quality_profile from serie_episodes where id = ?", d.Nzb.NzbepisodeID)
		}
		dbserieepisodeid := d.Dbserieepisode.ID
		if dbserieepisodeid == 0 {
			dbserieepisodeid, _ = database.QueryColumnUint("select dbserie_episode_id from serie_episodes where id = ?", d.Nzb.NzbepisodeID)
		}

		database.InsertNamed("Insert into serie_episode_histories (title, url, target, indexer, downloaded_at, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (:title, :url, :target, :indexer, :downloaded_at, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :resolution_id, :quality_id, :codec_id, :audio_id, :quality_profile)",
			database.SerieEpisodeHistory{
				Title:            d.Nzb.NZB.Title,
				URL:              d.Nzb.NZB.DownloadURL,
				Target:           config.Cfg.Paths[d.Target].Path,
				Indexer:          d.Nzb.Indexer,
				DownloadedAt:     time.Now().In(logger.TimeZone),
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
}

package utils

import (
	"bytes"
	"html/template"
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
)

type Downloader struct {
	ConfigEntry      config.MediaTypeConfig
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss

	Nzb            Nzbwithprio
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
func (d *Downloader) DownloadNzb(nzb Nzbwithprio) {
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
	case "sabnzbd":
		err = d.DownloadBySabnzbd()
	case "rtorrent":
		err = d.DownloadByRTorrent()
	case "qbittorrent":
		err = d.DownloadByQBittorrent()
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
	filename := d.Targetfile + ".nzb"
	if d.Nzb.NZB.IsTorrent {
		filename = d.Targetfile + ".torrent"
	}
	return downloadFile(d.Target.Path, "", filename, d.Nzb.NZB.DownloadURL)
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
func (d Downloader) DownloadBySabnzbd() error {
	logger.Log.Debug("Download by Sabnzbd: ", d.Nzb.NZB.DownloadURL)
	err := apiexternal.SendToSabnzbd(d.Downloader.Hostname, d.Downloader.Password, d.Nzb.NZB.DownloadURL, d.Category, d.Targetfile, d.Downloader.Priority)
	if err != nil {
		logger.Log.Error("Download by Sabnzbd - ERROR: ", err)
	}
	return err
}
func (d Downloader) DownloadByRTorrent() error {
	logger.Log.Debug("Download by rTorrent: ", d.Nzb.NZB.DownloadURL)
	err := apiexternal.SendToRtorrent(d.Downloader.Hostname, false, d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, d.Targetfile)
	if err != nil {
		logger.Log.Error("Download by rTorrent - ERROR: ", err)
	}
	return err
}
func (d Downloader) DownloadByDeluge() error {
	logger.Log.Debug("Download by Deluge: ", d.Nzb.NZB.DownloadURL)

	err := apiexternal.SendToDeluge(
		d.Downloader.Hostname, d.Downloader.Port, d.Downloader.Username, d.Downloader.Password,
		d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, d.Downloader.DelugeMoveAfter, d.Downloader.DelugeMoveTo, d.Downloader.AddPaused)

	if err != nil {
		logger.Log.Error("Download by Deluge - ERROR: ", err)
	}
	return err
}
func (d Downloader) DownloadByQBittorrent() error {
	logger.Log.Debug("Download by qBittorrent: ", d.Nzb.NZB.DownloadURL)

	err := apiexternal.SendToQBittorrent(
		d.Downloader.Hostname, strconv.Itoa(d.Downloader.Port), d.Downloader.Username, d.Downloader.Password,
		d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, strconv.FormatBool(d.Downloader.AddPaused))

	if err != nil {
		logger.Log.Error("Download by qBittorrent - ERROR: ", err)
	}
	return err
}

func (d Downloader) SendNotify(event string, noticonfig config.MediaNotificationConfig) {
	if !strings.EqualFold(noticonfig.Event, event) {
		return
	}
	//messagetext := noticonfig.Message
	tmplmessage, err := template.New("tmplfile").Parse(noticonfig.Message)
	if err != nil {
		logger.Log.Error(err)
	}
	var docmessage bytes.Buffer
	err = tmplmessage.Execute(&docmessage, d)
	if err != nil {
		logger.Log.Error(err)
	}
	messagetext := docmessage.String()

	tmpltitle, err := template.New("tmplfile").Parse(noticonfig.Title)
	if err != nil {
		logger.Log.Error(err)
	}
	var doctitle bytes.Buffer
	err = tmpltitle.Execute(&doctitle, d)
	if err != nil {
		logger.Log.Error(err)
	}
	MessageTitle := doctitle.String()

	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}
	var cfg_notification config.NotificationConfig
	config.ConfigGet("notification_"+noticonfig.Map_notification, &cfg_notification)

	if strings.EqualFold(cfg_notification.Type, "pushover") {
		if apiexternal.PushoverApi.ApiKey != cfg_notification.Apikey {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}

		err := apiexternal.PushoverApi.SendMessage(messagetext, MessageTitle, cfg_notification.Recipient)
		if err != nil {
			logger.Log.Error("Error sending pushover", err)
		} else {
			logger.Log.Info("Pushover message sent")
		}
	}
	if strings.EqualFold(cfg_notification.Type, "csv") {
		f, errf := os.OpenFile(cfg_notification.Outputto,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if errf != nil {
			logger.Log.Error("Error opening csv to write", errf)
			return
		}
		defer f.Close()
		if errf == nil {
			_, errc := io.WriteString(f, messagetext+"\n")
			if errc != nil {
				logger.Log.Error("Error writing to csv", errc)
			} else {
				logger.Log.Info("csv written")
			}
		}
	}
}
func (d Downloader) Notify() {
	for idxnoti := range d.ConfigEntry.Notification {
		d.SendNotify("added_download", d.ConfigEntry.Notification[idxnoti])
	}
}

func (d Downloader) History() {
	if strings.EqualFold(d.SearchGroupType, "movie") {
		movieID := d.Movie.ID
		moviequality := d.Movie.QualityProfile
		if movieID == 0 {
			movieID = d.Nzb.Nzbmovie.ID
			moviequality = d.Nzb.Nzbmovie.QualityProfile
		}
		dbmovieID := d.Movie.DbmovieID
		if dbmovieID == 0 {
			dbmovieID = d.Nzb.Nzbmovie.DbmovieID
		}

		database.InsertArray("movie_histories",
			[]string{"title", "url", "target", "indexer", "downloaded_at", "movie_id", "dbmovie_id", "resolution_id", "quality_id", "codec_id", "audio_id", "quality_profile"},
			[]interface{}{d.Nzb.NZB.Title, d.Nzb.NZB.DownloadURL, d.Target.Path, d.Nzb.Indexer, time.Now(), movieID, dbmovieID, d.Nzb.ParseInfo.ResolutionID, d.Nzb.ParseInfo.QualityID, d.Nzb.ParseInfo.CodecID, d.Nzb.ParseInfo.AudioID, moviequality})
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
		serieepisodequality := d.Serieepisode.QualityProfile
		if serieepisodeid == 0 {
			serieepisodeid = d.Nzb.Nzbepisode.ID
			serieepisodequality = d.Nzb.Nzbepisode.QualityProfile
		}
		dbserieepisodeid := d.Dbserieepisode.ID
		if dbserieepisodeid == 0 {
			dbserieepisodeid = d.Nzb.Nzbepisode.DbserieEpisodeID
		}

		database.InsertArray("serie_episode_histories",
			[]string{"title", "url", "target", "indexer", "downloaded_at", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id", "resolution_id", "quality_id", "codec_id", "audio_id", "quality_profile"},
			[]interface{}{d.Nzb.NZB.Title, d.Nzb.NZB.DownloadURL, d.Target.Path, d.Nzb.Indexer, time.Now(), serieid, serieepisodeid, dbserieepisodeid, dbserieid, d.Nzb.ParseInfo.ResolutionID, d.Nzb.ParseInfo.QualityID, d.Nzb.ParseInfo.CodecID, d.Nzb.ParseInfo.AudioID, serieepisodequality})
	}
}

func getnzbconfig(nzb Nzbwithprio, quality string) (category string, target config.PathsConfig, downloader config.DownloaderConfig) {

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

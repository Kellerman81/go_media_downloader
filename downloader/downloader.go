package downloader

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
	"github.com/Kellerman81/go_media_downloader/parser"
)

type downloadertype struct {
	ConfigTemplate   string
	Quality          string
	SearchGroupType  string //series, movies
	SearchActionType string //missing,upgrade,rss

	Nzb            parser.Nzbwithprio
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
	Time       string
}

func NewDownloader(configTemplate string, searchActionType string) downloadertype {
	return downloadertype{
		ConfigTemplate:   configTemplate,
		SearchActionType: searchActionType,
	}
}
func (d *downloadertype) SetMovie(movie database.Movie) {
	d.SearchGroupType = "movie"
	dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})

	d.Dbmovie = dbmovie
	d.Movie = movie
	logger.Log.Debug("Downloader movie: ", movie)
	logger.Log.Debug("Downloader movie quality: ", movie.QualityProfile)
	d.Quality = movie.QualityProfile
}
func (d *downloadertype) SetSeriesEpisode(seriesepisode database.SerieEpisode) {
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
func (d *downloadertype) DownloadNzb(nzb parser.Nzbwithprio) {
	d.Nzb = nzb
	d.Category, d.Target, d.Downloader = parser.Getnzbconfig(d.Nzb, d.Quality)

	targetfolder := ""
	if d.SearchGroupType == "movie" {
		if d.Dbmovie.ImdbID != "" {
			targetfolder = logger.Path(d.Nzb.NZB.Title+" ("+d.Dbmovie.ImdbID+")", false)
		} else if d.Nzb.NZB.IMDBID != "" {
			if !strings.Contains(d.Nzb.NZB.IMDBID, "tt") {
				nzb.NZB.IMDBID = "tt" + d.Nzb.NZB.IMDBID
			}
			if nzb.NZB.Title == "" {
				targetfolder = logger.Path(nzb.ParseInfo.Title+"["+nzb.ParseInfo.Resolution+" "+nzb.ParseInfo.Quality+"]"+" ("+nzb.NZB.IMDBID+")", false)
			} else {
				targetfolder = logger.Path(nzb.NZB.Title+" ("+nzb.NZB.IMDBID+")", false)
			}
		} else {
			targetfolder = logger.Path(nzb.NZB.Title, false)
		}
	} else {
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
	targetfolder = strings.Replace(targetfolder, "[", "", -1)
	targetfolder = strings.Replace(targetfolder, "]", "", -1)
	d.Targetfile = targetfolder

	logger.Log.Debug("Downloader target folder: ", targetfolder)
	logger.Log.Debug("Downloader target type: ", d.Downloader.Type)
	var err error
	switch strings.ToLower(d.Downloader.Type) {
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
func (d *downloadertype) downloadByDrone() error {
	logger.Log.Debug("Download by Drone: ", d.Nzb.NZB.DownloadURL)
	filename := d.Targetfile + ".nzb"
	if d.Nzb.NZB.IsTorrent {
		filename = d.Targetfile + ".torrent"
	}
	return logger.DownloadFile(d.Target.Path, "", filename, d.Nzb.NZB.DownloadURL)
}
func (d *downloadertype) downloadByNzbget() error {
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
func (d *downloadertype) downloadBySabnzbd() error {
	logger.Log.Debug("Download by Sabnzbd: ", d.Nzb.NZB.DownloadURL)
	err := apiexternal.SendToSabnzbd(d.Downloader.Hostname, d.Downloader.Password, d.Nzb.NZB.DownloadURL, d.Category, d.Targetfile, d.Downloader.Priority)
	if err != nil {
		logger.Log.Error("Download by Sabnzbd - ERROR: ", err)
	}
	return err
}
func (d *downloadertype) downloadByRTorrent() error {
	logger.Log.Debug("Download by rTorrent: ", d.Nzb.NZB.DownloadURL)
	err := apiexternal.SendToRtorrent(d.Downloader.Hostname, false, d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, d.Targetfile)
	if err != nil {
		logger.Log.Error("Download by rTorrent - ERROR: ", err)
	}
	return err
}
func (d *downloadertype) downloadByTransmission() error {
	logger.Log.Debug("Download by transmission: ", d.Nzb.NZB.DownloadURL)
	err := apiexternal.SendToTransmission(d.Downloader.Hostname, d.Downloader.Username, d.Downloader.Password, d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, d.Downloader.AddPaused)
	if err != nil {
		logger.Log.Error("Download by transmission - ERROR: ", err)
	}
	return err
}

func (d *downloadertype) downloadByDeluge() error {
	logger.Log.Debug("Download by Deluge: ", d.Nzb.NZB.DownloadURL)

	err := apiexternal.SendToDeluge(
		d.Downloader.Hostname, d.Downloader.Port, d.Downloader.Username, d.Downloader.Password,
		d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, d.Downloader.DelugeMoveAfter, d.Downloader.DelugeMoveTo, d.Downloader.AddPaused)

	if err != nil {
		logger.Log.Error("Download by Deluge - ERROR: ", err)
	}
	return err
}
func (d *downloadertype) downloadByQBittorrent() error {
	logger.Log.Debug("Download by qBittorrent: ", d.Nzb.NZB.DownloadURL)

	err := apiexternal.SendToQBittorrent(
		d.Downloader.Hostname, strconv.Itoa(d.Downloader.Port), d.Downloader.Username, d.Downloader.Password,
		d.Nzb.NZB.DownloadURL, d.Downloader.DelugeDlTo, strconv.FormatBool(d.Downloader.AddPaused))

	if err != nil {
		logger.Log.Error("Download by qBittorrent - ERROR: ", err)
	}
	return err
}

func (d *downloadertype) sendNotify(event string, noticonfig config.MediaNotificationConfig) {
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
	cfg_notification := config.ConfigGet("notification_" + noticonfig.Map_notification).Data.(config.NotificationConfig)

	if strings.EqualFold(cfg_notification.Type, "pushover") {
		if apiexternal.PushoverApi == nil {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}
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
func (d *downloadertype) notify() {
	prefix := "serie_"
	if d.SearchGroupType == "movie" {
		prefix = "movie_"
	}
	configEntry := config.ConfigGet(prefix + d.ConfigTemplate).Data.(config.MediaTypeConfig)
	d.Time = time.Now().Format(time.RFC3339)
	for idxnoti := range configEntry.Notification {
		d.sendNotify("added_download", configEntry.Notification[idxnoti])
	}
}

func (d *downloadertype) history() {
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

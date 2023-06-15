package downloader

import (
	"errors"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/nzbget"
	"github.com/Kellerman81/go_media_downloader/scanner"
)

type downloadertype struct {
	Cfgpstr         string
	Quality         string
	SearchGroupType string //series, movies
	//SearchActionType string //missing,upgrade,rss

	Nzb            *apiexternal.Nzbwithprio
	Movie          *database.Movie
	Dbmovie        *database.Dbmovie
	Serie          *database.Serie
	Dbserie        *database.Dbserie
	Serieepisode   *database.SerieEpisode
	Dbserieepisode *database.DbserieEpisode

	Category   string
	Target     string
	Downloader string
	Targetfile string
	Time       string
}

const strTvdbid = " (tvdb"

func (d *downloadertype) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if d == nil {
		return
	}
	logger.ClearVar(d.Dbmovie)
	logger.ClearVar(d.Dbserie)
	logger.ClearVar(d.Dbserieepisode)
	logger.ClearVar(d.Movie)
	logger.ClearVar(d.Serie)
	logger.ClearVar(d.Serieepisode)
	d.Nzb.Close()
	logger.ClearVar(d)
}

func newDownloader(cfgpstr string, nzb *apiexternal.Nzbwithprio, searchGroupType string) *downloadertype {
	return &downloadertype{
		Cfgpstr:         cfgpstr,
		SearchGroupType: searchGroupType,
		Nzb:             nzb,
	}
}
func DownloadMovie(cfgpstr string, movieid uint, nzb *apiexternal.Nzbwithprio) {
	d := newDownloader(cfgpstr, nzb, logger.StrMovie)
	var err error
	d.Movie, err = database.GetMovies(database.Querywithargs{Where: logger.FilterByID}, movieid)
	if err != nil {
		//logger.LogerrorStr(err, "movie not found", logger.IntToString(int(movieid)), "")
		logger.Log.Error().Err(err).Str("movie not found", logger.IntToString(int(movieid))).Msg("not found")
		d.Close()
		return
	}
	d.Dbmovie, err = database.GetDbmovie(logger.FilterByID, d.Movie.DbmovieID)
	if err != nil {
		//logger.LogerrorStr(err, "dbmovie not found", logger.IntToString(int(d.Movie.DbmovieID)), "")
		logger.Log.Error().Err(err).Str("dbmovie not found", logger.IntToString(int(d.Movie.DbmovieID))).Msg("not found")
		d.Close()
		return
	}
	d.Quality = d.Movie.QualityProfile
	d.downloadNzb()
	d.Close()
}
func DownloadSeriesEpisode(cfgpstr string, episodeid uint, nzb *apiexternal.Nzbwithprio) {
	d := newDownloader(cfgpstr, nzb, logger.StrSeries)
	var err error
	d.Serieepisode, err = database.GetSerieEpisodes(database.Querywithargs{Where: logger.FilterByID}, episodeid)
	if err != nil {
		//logger.LogerrorStr(err, "episode not found", logger.IntToString(int(episodeid)), "")
		logger.Log.Error().Err(err).Str("episode not found", logger.IntToString(int(episodeid))).Msg("not found")
		d.Close()
		return
	}
	d.Dbserie, err = database.GetDbserieByID(d.Serieepisode.DbserieID)
	if err != nil {
		//logger.LogerrorStr(err, "dbserie not found", logger.IntToString(int(d.Serieepisode.DbserieID)), "")
		logger.Log.Error().Err(err).Str("dbserie not found", logger.IntToString(int(d.Serieepisode.DbserieID))).Msg("not found")
		d.Close()
		return
	}
	d.Dbserieepisode, err = database.GetDbserieEpisodesByID(d.Serieepisode.DbserieEpisodeID)
	if err != nil {
		//logger.LogerrorStr(err, "dbepispode not found", logger.IntToString(int(d.Serieepisode.DbserieEpisodeID)), "")
		logger.Log.Error().Err(err).Str("dbepisode not found", logger.IntToString(int(d.Serieepisode.DbserieEpisodeID))).Msg("not found")
		d.Close()
		return
	}
	d.Serie, err = database.GetSeries(database.Querywithargs{Where: logger.FilterByID}, d.Serieepisode.SerieID)
	if err != nil {
		//logger.LogerrorStr(err, "serie not found", logger.IntToString(int(d.Serieepisode.SerieID)), "")
		logger.Log.Error().Err(err).Str("serie not found", logger.IntToString(int(d.Serieepisode.SerieID))).Msg("not found")
		d.Close()
		return
	}
	d.Quality = d.Serieepisode.QualityProfile
	d.downloadNzb()
	d.Close()
}

func (d *downloadertype) buildfoldername() (string, string) {
	if d.SearchGroupType == logger.StrMovie {
		if d.Dbmovie.ImdbID != "" {
			return "movie_histories", d.Nzb.NZB.Title + " (" + d.Dbmovie.ImdbID + ")"
		} else if d.Nzb.NZB.IMDBID != "" {
			d.Nzb.NZB.IMDBID = logger.AddImdbPrefix(d.Nzb.NZB.IMDBID)
			if d.Nzb.NZB.Title == "" {
				return "movie_histories", d.Nzb.ParseInfo.M.Title + "[" + d.Nzb.ParseInfo.M.Resolution + " " + d.Nzb.ParseInfo.M.Quality + "] (" + d.Nzb.NZB.IMDBID + ")"
			}
			return "movie_histories", d.Nzb.NZB.Title + " (" + d.Nzb.NZB.IMDBID + ")"
		}
		return "movie_histories", d.Nzb.NZB.Title
	}
	if d.Dbserie.ThetvdbID != 0 {
		if d.Nzb.NZB.Title == "" {
			return "serie_episode_histories", d.Nzb.ParseInfo.M.Title + "[" + d.Nzb.ParseInfo.M.Resolution + " " + d.Nzb.ParseInfo.M.Quality + "]" + strTvdbid + logger.IntToString(d.Dbserie.ThetvdbID) + ")"
		}
		return "serie_episode_histories", d.Nzb.NZB.Title + strTvdbid + logger.IntToString(d.Dbserie.ThetvdbID) + ")"
	} else if d.Nzb.NZB.TVDBID != 0 {
		if d.Nzb.NZB.Title == "" {
			return "serie_episode_histories", d.Nzb.ParseInfo.M.Title + "[" + d.Nzb.ParseInfo.M.Resolution + " " + d.Nzb.ParseInfo.M.Quality + "]" + strTvdbid + logger.IntToString(d.Nzb.NZB.TVDBID) + ")"
		}
		return "serie_episode_histories", d.Nzb.NZB.Title + strTvdbid + logger.IntToString(d.Nzb.NZB.TVDBID) + ")"
	} else {
		if d.Nzb.NZB.Title == "" {
			return "serie_episode_histories", d.Nzb.ParseInfo.M.Title + "[" + d.Nzb.ParseInfo.M.Resolution + " " + d.Nzb.ParseInfo.M.Quality + "]"
		}
		return "serie_episode_histories", d.Nzb.NZB.Title
	}
}
func (d *downloadertype) downloadNzb() {
	var err error
	d.Category, d.Target, d.Downloader, err = d.Nzb.Getnzbconfig(d.Quality)
	if err != nil {
		logger.Logerror(err, "Error get Nzb Config")
		return
	}
	historytable, targetfolder := d.buildfoldername()

	logger.Path(&targetfolder, false)
	if config.SettingsGeneral.UseMediaCache {
		if logger.HasPrefixI(historytable, logger.StrMovie) {
			database.CacheHistoryUrlMovie = append(database.CacheHistoryUrlMovie, d.Nzb.NZB.DownloadURL)
			database.CacheHistoryTitleMovie = append(database.CacheHistoryTitleMovie, d.Nzb.NZB.Title)
		} else {
			database.CacheHistoryUrlSeries = append(database.CacheHistoryUrlSeries, d.Nzb.NZB.DownloadURL)
			database.CacheHistoryTitleSeries = append(database.CacheHistoryTitleSeries, d.Nzb.NZB.Title)
		}
	}
	//cache.Append(logger.GlobalCache, historytable+"_url", d.Nzb.NZB.DownloadURL)
	//cache.Append(logger.GlobalCache, historytable+"_title", d.Nzb.NZB.Title)

	logger.StringDeleteRuneP(&targetfolder, '[')
	logger.StringDeleteRuneP(&targetfolder, ']')
	d.Targetfile = targetfolder

	logger.Log.Debug().Str("nzb", d.Nzb.NZB.Title).Str("by", config.SettingsDownloader["downloader_"+d.Downloader].DlType).Msg("Downloading")

	switch config.SettingsDownloader["downloader_"+d.Downloader].DlType {
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
		logger.Logerror(errors.New("unknown downloader"), "Download")
		return
	}
	if err != nil {
		logger.Logerror(err, "Download")
		return
	}
	d.notify()

	if d.SearchGroupType == logger.StrMovie {
		if d.Movie.ID == 0 {
			d.Movie.ID = d.Nzb.NzbmovieID
		}
		if d.Movie.ID != 0 && d.Movie.QualityProfile == "" {
			database.QueryColumn(database.QueryMoviesGetQualityByID, &d.Movie.QualityProfile, d.Nzb.NzbmovieID)
		}
		if d.Movie.DbmovieID == 0 && d.Nzb.NzbmovieID != 0 {
			database.QueryColumn(database.QueryMoviesGetDBIDByID, &d.Movie.DbmovieID, d.Nzb.NzbmovieID)
		}

		database.InsertStatic("Insert into movie_histories (title, url, target, indexer, downloaded_at, movie_id, dbmovie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			d.Nzb.NZB.Title, d.Nzb.NZB.DownloadURL, config.SettingsPath["path_"+d.Target].Path, d.Nzb.NZB.Indexer, logger.TimeGetNow(), d.Movie.ID, d.Movie.DbmovieID, d.Nzb.ParseInfo.M.ResolutionID, d.Nzb.ParseInfo.M.QualityID, d.Nzb.ParseInfo.M.CodecID, d.Nzb.ParseInfo.M.AudioID, d.Movie.QualityProfile)
		return
	}
	if d.Serie.ID == 0 {
		database.QueryColumn(database.QuerySerieEpisodesGetSerieIDByID, &d.Serie.ID, d.Nzb.NzbepisodeID)
	}
	if d.Dbserie.ID == 0 {
		database.QueryColumn(database.QuerySerieEpisodesGetDBSerieIDByID, &d.Dbserie.ID, d.Nzb.NzbepisodeID)
	}
	if d.Serieepisode.ID == 0 {
		d.Serieepisode.ID = d.Nzb.NzbepisodeID
	}
	if d.Serieepisode.QualityProfile == "" {
		database.QueryColumn(database.QuerySerieEpisodesGetQualityByID, &d.Serieepisode.QualityProfile, d.Nzb.NzbepisodeID)
	}
	if d.Dbserieepisode.ID == 0 {
		database.QueryColumn(database.QuerySerieEpisodesGetDBSerieEpisodeIDByID, &d.Dbserieepisode.ID, d.Nzb.NzbepisodeID)
	}

	database.InsertStatic("Insert into serie_episode_histories (title, url, target, indexer, downloaded_at, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		d.Nzb.NZB.Title, d.Nzb.NZB.DownloadURL, config.SettingsPath["path_"+d.Target].Path, d.Nzb.NZB.Indexer, logger.TimeGetNow(), d.Serie.ID, d.Serieepisode.ID, d.Dbserieepisode.ID, d.Dbserie.ID, d.Nzb.ParseInfo.M.ResolutionID, d.Nzb.ParseInfo.M.QualityID, d.Nzb.ParseInfo.M.CodecID, d.Nzb.ParseInfo.M.AudioID, d.Serieepisode.QualityProfile)
}

func (d *downloadertype) notify() {
	d.Time = logger.TimeGetNow().Format(logger.TimeFormat)
	for idx := range config.SettingsMedia[d.Cfgpstr].Notification {
		if !strings.EqualFold(config.SettingsMedia[d.Cfgpstr].Notification[idx].Event, "added_download") {
			continue
		}
		messagetext, err := logger.ParseStringTemplate(config.SettingsMedia[d.Cfgpstr].Notification[idx].Message, d)
		if err != nil {
			continue
		}
		messageTitle, err := logger.ParseStringTemplate(config.SettingsMedia[d.Cfgpstr].Notification[idx].Title, d)
		if err != nil {
			continue
		}

		if !config.CheckGroup("notification_", config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification) {
			continue
		}

		if strings.EqualFold(config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].NotificationType, "pushover") {
			if apiexternal.PushoverAPI == nil {
				apiexternal.NewPushOverClient(config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].Apikey)
			}
			if apiexternal.PushoverAPI.APIKey != config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].Apikey {
				apiexternal.NewPushOverClient(config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].Apikey)
			}

			err = apiexternal.PushoverAPI.SendMessage(messagetext, messageTitle, config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].Recipient)
			if err != nil {
				logger.Log.Error().Err(err).Msg("Error sending pushover")
				//logger.Logerror(err, "Error sending pushover")
			} else {
				logger.Log.Info().Msg("Pushover message sent")
			}
		}
		if strings.EqualFold(config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].NotificationType, "csv") {
			if scanner.AppendCsv(config.SettingsNotification["notification_"+config.SettingsMedia[d.Cfgpstr].Notification[idx].MapNotification].Outputto, messagetext) != nil {
				continue
			}
		}
	}
}
func (d *downloadertype) downloadByDrone() error {
	filename := d.Targetfile + ".nzb"
	if d.Nzb.NZB.IsTorrent {
		filename = d.Targetfile + ".torrent"
	}
	return scanner.DownloadFile(config.SettingsPath["path_"+d.Target].Path, "", filename, &d.Nzb.NZB.DownloadURL, false)
}
func (d *downloadertype) downloadByNzbget() error {
	urlv := "http://" + config.SettingsDownloader["downloader_"+d.Downloader].Username + ":" + config.SettingsDownloader["downloader_"+d.Downloader].Password + "@" + config.SettingsDownloader["downloader_"+d.Downloader].Hostname + "/jsonrpc"
	options := nzbget.NewOptions()

	options.Category = d.Category
	options.AddPaused = config.SettingsDownloader["downloader_"+d.Downloader].AddPaused
	options.Priority = config.SettingsDownloader["downloader_"+d.Downloader].Priority
	options.NiceName = d.Targetfile + ".nzb"
	_, err := nzbget.NewClient(urlv).Add(d.Nzb.NZB.DownloadURL, options)
	return err
}
func (d *downloadertype) downloadBySabnzbd() error {
	return apiexternal.SendToSabnzbd(config.SettingsDownloader["downloader_"+d.Downloader].Hostname, config.SettingsDownloader["downloader_"+d.Downloader].Password, d.Nzb.NZB.DownloadURL, d.Category, d.Targetfile, config.SettingsDownloader["downloader_"+d.Downloader].Priority)
}
func (d *downloadertype) downloadByRTorrent() error {
	return apiexternal.SendToRtorrent(config.SettingsDownloader["downloader_"+d.Downloader].Hostname, false, d.Nzb.NZB.DownloadURL, config.SettingsDownloader["downloader_"+d.Downloader].DelugeDlTo, d.Targetfile)
}
func (d *downloadertype) downloadByTransmission() error {
	return apiexternal.SendToTransmission(config.SettingsDownloader["downloader_"+d.Downloader].Hostname, config.SettingsDownloader["downloader_"+d.Downloader].Username, config.SettingsDownloader["downloader_"+d.Downloader].Password, d.Nzb.NZB.DownloadURL, config.SettingsDownloader["downloader_"+d.Downloader].DelugeDlTo, config.SettingsDownloader["downloader_"+d.Downloader].AddPaused)
}

func (d *downloadertype) downloadByDeluge() error {
	return apiexternal.SendToDeluge(
		config.SettingsDownloader["downloader_"+d.Downloader].Hostname, config.SettingsDownloader["downloader_"+d.Downloader].Port, config.SettingsDownloader["downloader_"+d.Downloader].Username, config.SettingsDownloader["downloader_"+d.Downloader].Password,
		d.Nzb.NZB.DownloadURL, config.SettingsDownloader["downloader_"+d.Downloader].DelugeDlTo, config.SettingsDownloader["downloader_"+d.Downloader].DelugeMoveAfter, config.SettingsDownloader["downloader_"+d.Downloader].DelugeMoveTo, config.SettingsDownloader["downloader_"+d.Downloader].AddPaused)
}
func (d *downloadertype) downloadByQBittorrent() error {
	return apiexternal.SendToQBittorrent(
		config.SettingsDownloader["downloader_"+d.Downloader].Hostname, logger.IntToString(config.SettingsDownloader["downloader_"+d.Downloader].Port), config.SettingsDownloader["downloader_"+d.Downloader].Username, config.SettingsDownloader["downloader_"+d.Downloader].Password,
		d.Nzb.NZB.DownloadURL, config.SettingsDownloader["downloader_"+d.Downloader].DelugeDlTo, strconv.FormatBool(config.SettingsDownloader["downloader_"+d.Downloader].AddPaused))
}

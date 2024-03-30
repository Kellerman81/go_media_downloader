package downloader

import (
	"errors"
	"html"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/nzbget"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

type downloadertype struct {
	Cfgp *config.MediaTypeConfig
	//Quality         string
	Quality *config.QualityConfig
	//SearchActionType string //missing,upgrade,rss

	Nzb            *apiexternal.Nzbwithprio
	Movie          database.Movie
	Dbmovie        database.Dbmovie
	Serie          database.Serie
	Dbserie        database.Dbserie
	Serieepisode   database.SerieEpisode
	Dbserieepisode database.DbserieEpisode

	Category string
	//Target        string
	IndexerCfg    *config.IndexersConfig
	TargetCfg     *config.PathsConfig
	Downloader    string
	DownloaderCfg *config.DownloaderConfig
	Targetfile    string
	Time          string
}

const strTvdbid = " (tvdb"

// newDownloader initializes a new downloadertype struct.
// It takes in a media type config pointer and an NZB with priority struct pointer.
// It returns a pointer to a downloadertype struct initialized with the passed in config
// and NZB values.
func newDownloader(cfgp *config.MediaTypeConfig, nzb *apiexternal.Nzbwithprio) *downloadertype {
	return &downloadertype{
		Cfgp: cfgp,
		Nzb:  nzb,
	}
}

// Close cleans up the downloader by closing the NZB handle and zeroing out the struct fields.
// This prevents lingering resources being held after the downloader is no longer needed.
// The cleanup is skipped if config.SettingsGeneral.DisableVariableCleanup is true or if d is nil.
func (d *downloadertype) Close() {
	if config.SettingsGeneral.DisableVariableCleanup || d == nil {
		return
	}
	d.Nzb.Close()
	*d = downloadertype{}
}

// DownloadMovie initializes a downloader, gets the movie and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadMovie(cfgp *config.MediaTypeConfig, movieid *uint, nzb *apiexternal.Nzbwithprio) {
	d := newDownloader(cfgp, nzb)
	defer d.Close()
	err := database.GetMoviesByIDP(movieid, &d.Movie)
	if err != nil {
		logger.LogDynamic("error", "not found", logger.NewLogFieldValue(err), logger.NewLogField("movie not found", movieid))
		return
	}
	err = database.GetDbmovieByIDP(&d.Movie.DbmovieID, &d.Dbmovie)
	if err != nil {
		logger.LogDynamic("error", "not found", logger.NewLogFieldValue(err), logger.NewLogField("dbmovie not found", d.Movie.DbmovieID))
		return
	}
	d.Quality = database.GetMediaQualityConfig(cfgp, movieid)
	d.downloadNzb()
}

// DownloadSeriesEpisode initializes a downloader, gets the episode and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadSeriesEpisode(cfgp *config.MediaTypeConfig, episodeid *uint, nzb *apiexternal.Nzbwithprio) {
	d := newDownloader(cfgp, nzb)
	defer d.Close()
	err := database.GetSerieEpisodesByIDP(episodeid, &d.Serieepisode)
	if err != nil {
		logger.LogDynamic("error", "not found", logger.NewLogFieldValue(err), logger.NewLogField("episode not found", episodeid))
		return
	}
	err = database.GetDbserieByIDP(&d.Serieepisode.DbserieID, &d.Dbserie)
	if err != nil {
		logger.LogDynamic("error", "not found", logger.NewLogFieldValue(err), logger.NewLogField("dbserie not found", d.Serieepisode.DbserieID))
		return
	}
	err = database.GetDbserieEpisodesByIDP(&d.Serieepisode.DbserieEpisodeID, &d.Dbserieepisode)
	if err != nil {
		logger.LogDynamic("error", "not found", logger.NewLogFieldValue(err), logger.NewLogField("dbepisode not found", d.Serieepisode.DbserieEpisodeID))
		return
	}
	err = database.GetSerieByIDP(&d.Serieepisode.SerieID, &d.Serie)
	if err != nil {
		logger.LogDynamic("error", "not found", logger.NewLogFieldValue(err), logger.NewLogField("serie not found", d.Serieepisode.SerieID))
		return
	}
	d.Quality = database.GetMediaQualityConfig(cfgp, episodeid)
	d.downloadNzb()
}

// downloadNzb downloads the NZB file using the configured downloader. It gets the
// target download folder, logs the download, calls the specific downloader method,
// handles errors, inserts history rows, and sends notifications.
func (d *downloadertype) downloadNzb() {
	for idx := range d.Quality.Indexer {
		if !strings.EqualFold(d.Quality.Indexer[idx].TemplateIndexer, d.Nzb.NZB.Indexer.Name) {
			continue
		}
		if d.Quality.Indexer[idx].CfgPath == nil {
			continue
		}

		if d.Quality.Indexer[idx].CfgDownloader == nil {
			continue
		}
		if d.Quality.Indexer[idx].CategoryDowloader != "" {
			logger.LogDynamic("debug", "Download", logger.NewLogField("Indexer", d.Quality.Indexer[idx].TemplateIndexer), logger.NewLogField("Downloader", d.Quality.Indexer[idx].TemplateDownloader))
			d.IndexerCfg = d.Quality.Indexer[idx].CfgIndexer
			d.Category = d.Quality.Indexer[idx].CategoryDowloader
			d.TargetCfg = d.Quality.Indexer[idx].CfgPath
			d.Downloader = d.Quality.Indexer[idx].TemplateDownloader
			d.DownloaderCfg = d.Quality.Indexer[idx].CfgDownloader
			break
		}
	}

	if d.Category == "" {
		logger.LogDynamic("debug", "Downloader nzb config NOT found - quality", logger.NewLogField("Quality", d.Quality.Name))

		if d.Quality.Indexer[0].CfgPath == nil {
			logger.LogDynamic("error", "Error get Nzb Config", logger.NewLogFieldValue(errors.New("path template not found")))
			return
		}

		if d.Quality.Indexer[0].CfgDownloader == nil {
			logger.LogDynamic("error", "Error get Nzb Config", logger.NewLogFieldValue(errors.New("downloader template not found")))
			return
		}
		logger.LogDynamic("debug", "Downloader nzb config NOT found - use first", logger.NewLogField("categories", d.Quality.Indexer[0].CategoryDowloader))

		d.IndexerCfg = d.Quality.Indexer[0].CfgIndexer
		d.Category = d.Quality.Indexer[0].CategoryDowloader
		d.TargetCfg = d.Quality.Indexer[0].CfgPath
		d.Downloader = d.Quality.Indexer[0].TemplateDownloader
		d.DownloaderCfg = d.Quality.Indexer[0].CfgDownloader
	}

	targetfolder := d.getdownloadtargetfolder()

	if config.SettingsGeneral.UseHistoryCache {
		database.AppendStringCache(logger.GetStringsMap(d.Cfgp.Useseries, logger.CacheHistoryTitle), d.Nzb.NZB.Title)
		database.AppendStringCache(logger.GetStringsMap(d.Cfgp.Useseries, logger.CacheHistoryUrl), d.Nzb.NZB.DownloadURL)
	}
	d.Targetfile = logger.StringRemoveAllRunesMulti(logger.Path(targetfolder, false), '[', ']')
	//d.Targetfile = logger.StringRemoveAllRunes(logger.StringRemoveAllRunes(logger.Path(targetfolder, false), '['), ']')

	logger.LogDynamic("debug", "Downloading", logger.NewLogField("nzb", d.Nzb.NZB.Title), logger.NewLogField("by", d.DownloaderCfg.DlType))

	var err error
	switch d.DownloaderCfg.DlType {
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
		logger.LogDynamic("error", "Download", logger.NewLogFieldValue(errors.New("unknown downloader")))
		return
	}
	if err != nil {
		logger.LogDynamic("error", "Download", logger.NewLogFieldValue(err))
		return
	}
	d.notify()

	if !d.Cfgp.Useseries {
		if d.Movie.ID == 0 {
			d.Movie.ID = d.Nzb.NzbmovieID
		}
		if d.Movie.ID != 0 && d.Movie.QualityProfile == "" {
			_ = database.ScanrowsNdyn(false, "select quality_profile from movies where id = ?", &d.Movie.QualityProfile, &d.Nzb.NzbmovieID)
		}
		if d.Movie.DbmovieID == 0 && d.Nzb.NzbmovieID != 0 {
			_ = database.ScanrowsNdyn(false, "select dbmovie_id from movies where id = ?", &d.Movie.DbmovieID, &d.Nzb.NzbmovieID)
		}
		now := logger.TimeGetNow()
		database.ExecN("Insert into movie_histories (title, url, target, indexer, downloaded_at, movie_id, dbmovie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&d.Nzb.NZB.Title, &d.Nzb.NZB.DownloadURL, &d.TargetCfg.Path, &d.Nzb.NZB.Indexer.Name, &now, &d.Movie.ID, &d.Movie.DbmovieID, &d.Nzb.Info.M.ResolutionID, &d.Nzb.Info.M.QualityID, &d.Nzb.Info.M.CodecID, &d.Nzb.Info.M.AudioID, &d.Movie.QualityProfile)
		return
	}
	if d.Serie.ID == 0 {
		_ = database.ScanrowsNdyn(false, database.QuerySerieEpisodesGetSerieIDByID, &d.Serie.ID, &d.Nzb.NzbepisodeID)
	}
	if d.Dbserie.ID == 0 {
		_ = database.ScanrowsNdyn(false, database.QuerySerieEpisodesGetDBSerieIDByID, &d.Dbserie.ID, &d.Nzb.NzbepisodeID)
	}
	if d.Serieepisode.ID == 0 {
		d.Serieepisode.ID = d.Nzb.NzbepisodeID
	}
	if d.Serieepisode.QualityProfile == "" {
		_ = database.ScanrowsNdyn(false, "select quality_profile from serie_episodes where id = ?", &d.Serieepisode.QualityProfile, &d.Nzb.NzbepisodeID)
	}
	if d.Dbserieepisode.ID == 0 {
		_ = database.ScanrowsNdyn(false, database.QuerySerieEpisodesGetDBSerieEpisodeIDByID, &d.Dbserieepisode.ID, &d.Nzb.NzbepisodeID)
	}

	now := logger.TimeGetNow()
	database.ExecN("Insert into serie_episode_histories (title, url, target, indexer, downloaded_at, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, resolution_id, quality_id, codec_id, audio_id, quality_profile) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&d.Nzb.NZB.Title, &d.Nzb.NZB.DownloadURL, &d.TargetCfg.Path, &d.Nzb.NZB.Indexer.Name, &now, &d.Serie.ID, &d.Serieepisode.ID, &d.Dbserieepisode.ID, &d.Dbserie.ID, &d.Nzb.Info.M.ResolutionID, &d.Nzb.Info.M.QualityID, &d.Nzb.Info.M.CodecID, &d.Nzb.Info.M.AudioID, &d.Serieepisode.QualityProfile)
}

// getdownloadtargetfolder returns the target download folder path for a download based on whether it is a movie or TV show download.
// For movies it tries to use the movie title and IMDB ID if available.
// For TV shows it tries to use the episode title and TVDB ID if available.
// Falls back to just using the title if IDs are not available.
func (d *downloadertype) getdownloadtargetfolder() string {
	if !d.Cfgp.Useseries {
		if d.Dbmovie.ImdbID != "" {
			return logger.JoinStrings(d.Nzb.NZB.Title, " (", d.Dbmovie.ImdbID, ")")
		} else if d.Nzb.NZB.IMDBID != "" {
			d.Nzb.NZB.IMDBID = logger.AddImdbPrefix(d.Nzb.NZB.IMDBID)
			if d.Nzb.NZB.Title == "" {
				return logger.JoinStrings(d.Nzb.Info.M.Title, "[", d.Nzb.Info.M.Resolution, " ", d.Nzb.Info.M.Quality, "] (", d.Nzb.NZB.IMDBID, ")")
			}
			return logger.JoinStrings(d.Nzb.NZB.Title, " (", d.Nzb.NZB.IMDBID, ")")
		}
		return d.Nzb.NZB.Title
	}
	if d.Dbserie.ThetvdbID != 0 {
		if d.Nzb.NZB.Title == "" {
			return logger.JoinStrings(d.Nzb.Info.M.Title, "[", d.Nzb.Info.M.Resolution, " ", d.Nzb.Info.M.Quality, "]", strTvdbid, strconv.Itoa(d.Dbserie.ThetvdbID), ")")
		}
		return logger.JoinStrings(d.Nzb.NZB.Title, strTvdbid, strconv.Itoa(d.Dbserie.ThetvdbID), ")")
	} else if d.Nzb.NZB.TVDBID != 0 {
		if d.Nzb.NZB.Title == "" {
			return logger.JoinStrings(d.Nzb.Info.M.Title, "[", d.Nzb.Info.M.Resolution, " ", d.Nzb.Info.M.Quality, "]", strTvdbid, strconv.Itoa(d.Nzb.NZB.TVDBID), ")")
		}
		return logger.JoinStrings(d.Nzb.NZB.Title, strTvdbid, strconv.Itoa(d.Nzb.NZB.TVDBID), ")")
	}
	if d.Nzb.NZB.Title == "" {
		return logger.JoinStrings(d.Nzb.Info.M.Title, "[", d.Nzb.Info.M.Resolution, " ", d.Nzb.Info.M.Quality, "]")
	}
	return d.Nzb.NZB.Title
}

// notify sends notifications about the added download based on the
// configured notifications in the downloader. It supports pushover
// and CSV append notifications. It parses notification titles and
// messages as templates using the downloader data.
func (d *downloadertype) notify() {
	d.Time = logger.TimeGetNow().Format(logger.GetTimeFormat())
	for idx := range d.Cfgp.Notification {
		if !strings.EqualFold(d.Cfgp.Notification[idx].Event, "added_download") {
			continue
		}
		bl, messagetext := logger.ParseStringTemplate(d.Cfgp.Notification[idx].Message, d)
		if bl {
			continue
		}
		bl, messageTitle := logger.ParseStringTemplate(d.Cfgp.Notification[idx].Title, d)
		if bl {
			continue
		}

		if d.Cfgp.Notification[idx].CfgNotification == nil {
			continue
		}

		if strings.EqualFold(d.Cfgp.Notification[idx].CfgNotification.NotificationType, "pushover") {
			apiexternal.NewPushOverClient(d.Cfgp.Notification[idx].CfgNotification.Apikey)
			if apiexternal.GetPushOverKey() != d.Cfgp.Notification[idx].CfgNotification.Apikey {
				apiexternal.NewPushOverClient(d.Cfgp.Notification[idx].CfgNotification.Apikey)
			}

			err := apiexternal.SendPushoverMessage(messagetext, messageTitle, d.Cfgp.Notification[idx].CfgNotification.Recipient)
			if err != nil {
				logger.LogDynamic("error", "Error sending pushover", logger.NewLogFieldValue(err))
			} else {
				logger.LogDynamic("info", "Pushover message sent")
			}
		}
		if strings.EqualFold(d.Cfgp.Notification[idx].CfgNotification.NotificationType, "csv") {
			_ = scanner.AppendCsv(d.Cfgp.Notification[idx].CfgNotification.Outputto, messagetext)
		}
	}
}

// downloadByDrone downloads the NZB or torrent file using the Drone downloader.
// It constructs the filename based on Targetfile, downloads the file to the Path
// in TargetCfg using scanner.DownloadFile, and returns any error.
func (d *downloadertype) downloadByDrone() error {
	filename := d.Targetfile + ".nzb"
	if d.Nzb.NZB.IsTorrent {
		filename = d.Targetfile + ".torrent"
	}
	urlv := html.UnescapeString(d.Nzb.NZB.DownloadURL)
	resp, err := apiexternal.GetnewznabclientRlClient(d.IndexerCfg).Getdo(urlv, nil, true)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	fileprefix := ""

	// Create the file
	if filename == "" {
		filename = filepath.Base(urlv)
	}
	if fileprefix != "" && filename != "" {
		filename = fileprefix + filename
	}
	out, err := os.Create(filepath.Join(d.TargetCfg.Path, filename))
	if err != nil {
		return err
	}
	defer out.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}
	// _, err = scanner.DownloadFileRes(resp, d.TargetCfg.Path, "", filename, urlv, true)
	// if err != nil {
	// 	return err
	// }
	return out.Sync()
}

// downloadByNzbget downloads the NZB file using the NZBGet downloader.
// It constructs the NZBGet options based on the downloader configuration,
// downloads the NZB file using the NZBGet API, and returns any error.
func (d *downloadertype) downloadByNzbget() error {
	options := nzbget.NewOptions()

	options.Category = d.Category
	options.AddPaused = d.DownloaderCfg.AddPaused
	options.Priority = d.DownloaderCfg.Priority
	options.NiceName = d.Targetfile + ".nzb"
	_, err := nzbget.NewClient("http://"+d.DownloaderCfg.Username+":"+d.DownloaderCfg.Password+"@"+d.DownloaderCfg.Hostname+"/jsonrpc").Add(html.UnescapeString(d.Nzb.NZB.DownloadURL), options)
	return err
}

// downloadBySabnzbd downloads the NZB file using the Sabnzbd downloader.
// It constructs the Sabnzbd options based on the downloader configuration,
// downloads the NZB file using the Sabnzbd API, and returns any error.
func (d *downloadertype) downloadBySabnzbd() error {
	return apiexternal.SendToSabnzbd(d.DownloaderCfg.Hostname, d.DownloaderCfg.Password, html.UnescapeString(d.Nzb.NZB.DownloadURL), d.Category, d.Targetfile, d.DownloaderCfg.Priority)
}

// downloadByRTorrent downloads the torrent file using the rTorrent downloader.
// It sends the torrent URL to the rTorrent API based on the downloader
// configuration and returns any error.
func (d *downloadertype) downloadByRTorrent() error {
	return apiexternal.SendToRtorrent(d.DownloaderCfg.Hostname, false, html.UnescapeString(d.Nzb.NZB.DownloadURL), d.DownloaderCfg.DelugeDlTo, d.Targetfile)
}

// downloadByTransmission downloads the torrent file using the Transmission
// downloader. It sends the torrent URL to the Transmission API based on
// the downloader configuration and returns any error.
func (d *downloadertype) downloadByTransmission() error {
	return apiexternal.SendToTransmission(d.DownloaderCfg.Hostname, d.DownloaderCfg.Username, d.DownloaderCfg.Password, html.UnescapeString(d.Nzb.NZB.DownloadURL), d.DownloaderCfg.DelugeDlTo, d.DownloaderCfg.AddPaused)
}

// downloadByDeluge downloads the torrent file using the Deluge downloader.
// It sends the torrent URL to the Deluge API based on the downloader
// configuration and returns any error.
func (d *downloadertype) downloadByDeluge() error {
	return apiexternal.SendToDeluge(
		d.DownloaderCfg.Hostname, d.DownloaderCfg.Port, d.DownloaderCfg.Username, d.DownloaderCfg.Password,
		html.UnescapeString(d.Nzb.NZB.DownloadURL), d.DownloaderCfg.DelugeDlTo, d.DownloaderCfg.DelugeMoveAfter, d.DownloaderCfg.DelugeMoveTo, d.DownloaderCfg.AddPaused)
}

// downloadByQBittorrent downloads the torrent file using the qBittorrent
// downloader. It sends the torrent URL to the qBittorrent API based on
// the downloader configuration and returns any error.
func (d *downloadertype) downloadByQBittorrent() error {
	return apiexternal.SendToQBittorrent(
		d.DownloaderCfg.Hostname, strconv.Itoa(d.DownloaderCfg.Port), d.DownloaderCfg.Username, d.DownloaderCfg.Password,
		html.UnescapeString(d.Nzb.NZB.DownloadURL), d.DownloaderCfg.DelugeDlTo, strconv.FormatBool(d.DownloaderCfg.AddPaused))
}

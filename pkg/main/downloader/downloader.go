package downloader

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

var (
	errDownloaderTemplateNotFound = errors.New("downloader template not found")
	errUnknownDownloader          = errors.New("unknown downloader")
	errNoQualityIndexer           = errors.New("quality config missing or has no indexers")
	errNoDownloadClient           = errors.New("download client not registered")
)

type downloadertype struct {
	Dbmovie        database.Dbmovie
	Dbserie        database.Dbserie
	Dbserieepisode database.DbserieEpisode
	Dbbook         database.Dbbook
	Dbaudiobook    database.Dbaudiobook
	Dbalbum        database.Dbalbum
	Movie          database.Movie
	Serie          database.Serie
	Serieepisode   database.SerieEpisode
	Book           database.Book
	Audiobook      database.Audiobook
	Album          database.Album
	Category       string
	Downloader     string
	Targetfile     string
	Time           string
	Cfgp           *config.MediaTypeConfig
	// Quality         string
	Quality *config.QualityConfig
	// SearchActionType string //missing,upgrade,rss
	Nzb *apiexternal_v2.Nzbwithprio
	// Target        string
	IndexerCfg    *config.IndexersConfig
	TargetCfg     *config.PathsConfig
	DownloaderCfg *config.DownloaderConfig
}

// downloadNzb orchestrates the download process for an NZB file using the configured
// downloader client. It performs the following operations:
//   - Matches the NZB indexer with quality configuration settings
//   - Retrieves downloader configuration, target paths, and categories
//   - Falls back to default settings if specific indexer config not found
//   - Validates that required configurations (path, downloader) are available
//   - Prepares download context with proper categorization and target handling
//
// The function sets up all necessary configuration before delegating to the specific
// downloader implementation (SABnzbd, NZBGet, etc.) for the actual download operation.
func (d *downloadertype) downloadNzb() error {
	if d.Quality == nil || len(d.Quality.Indexer) == 0 {
		logger.Logtype("error", 0).
			Err(errNoQualityIndexer).
			Str("nzb", d.Nzb.NZB.Title).
			Msg("Error get Nzb Config")
		return errNoQualityIndexer
	}

	var nzbIndexer string
	if d.Nzb.NZB.Indexer != nil {
		nzbIndexer = d.Nzb.NZB.Indexer.Name
	}

	for idx := range d.Quality.Indexer {
		if nzbIndexer == "" ||
			!strings.EqualFold(d.Quality.Indexer[idx].TemplateIndexer, nzbIndexer) {
			continue
		}

		if d.Quality.Indexer[idx].CfgPath == nil {
			continue
		}

		if d.Quality.Indexer[idx].CfgDownloader == nil {
			continue
		}

		if d.Quality.Indexer[idx].CategoryDownloader == "" {
			continue
		}

		logger.Logtype("debug", 2).
			Str(logger.StrIndexer, d.Quality.Indexer[idx].TemplateIndexer).
			Str("Downloader", d.Quality.Indexer[idx].TemplateDownloader).
			Msg("Download")

		d.IndexerCfg = d.Quality.Indexer[idx].CfgIndexer
		d.Category = d.Quality.Indexer[idx].CategoryDownloader
		d.TargetCfg = d.Quality.Indexer[idx].CfgPath
		d.Downloader = d.Quality.Indexer[idx].TemplateDownloader
		d.DownloaderCfg = d.Quality.Indexer[idx].CfgDownloader

		break
	}

	if d.Category == "" {
		logger.Logtype("debug", 1).
			Str("Quality", d.Quality.Name).
			Msg("Downloader nzb config NOT found - quality")

		if d.Quality.Indexer[0].CfgPath == nil {
			logger.Logtype("error", 0).
				Err(logger.ErrPathTemplateNotFound).
				Msg("Error get Nzb Config")
			return logger.ErrPathTemplateNotFound
		}

		if d.Quality.Indexer[0].CfgDownloader == nil {
			logger.Logtype("error", 0).
				Err(errDownloaderTemplateNotFound).
				Msg("Error get Nzb Config")
			return errDownloaderTemplateNotFound
		}

		logger.Logtype("debug", 1).
			Str("categories", d.Quality.Indexer[0].CategoryDownloader).
			Msg("Downloader nzb config NOT found - use first")

		d.IndexerCfg = d.Quality.Indexer[0].CfgIndexer
		d.Category = d.Quality.Indexer[0].CategoryDownloader
		d.TargetCfg = d.Quality.Indexer[0].CfgPath
		d.Downloader = d.Quality.Indexer[0].TemplateDownloader
		d.DownloaderCfg = d.Quality.Indexer[0].CfgDownloader
	}

	d.Targetfile = d.getdownloadtargetfolder()
	logger.Path(&d.Targetfile, false)
	logger.StringRemoveAllRunesP(&d.Targetfile, '[', ']')

	logger.Logtype("debug", 2).
		Str("nzb", d.Nzb.NZB.Title).
		Str("by", d.DownloaderCfg.DlType).
		Msg("Downloading")

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
		logger.Logtype("error", 0).
			Err(errUnknownDownloader).
			Msg("Download")
		return errUnknownDownloader
	}

	if err != nil {
		logger.Logtype("error", 0).
			Err(err).
			Msg("Download")
		return err
	}

	// Only cache the release once the client accepted it - caching before the
	// add made a failed add skip the release forever.
	if config.GetSettingsGeneral().UseHistoryCache {
		database.AppendCacheMap(d.Cfgp.IsType, logger.CacheHistoryTitle, d.Nzb.NZB.Title)
		database.AppendCacheMap(d.Cfgp.IsType, logger.CacheHistoryURL, d.Nzb.NZB.DownloadURL)
	}

	d.notify()

	d.downloadNzbType(d.Cfgp.IsType, d.savePath())

	return nil
}

// savePath returns where the client actually saves the download: torrent
// clients use DelugeDlTo (when set) rather than the path template.
func (d *downloadertype) savePath() string {
	switch d.DownloaderCfg.DlType {
	case "transmission", "rtorrent", "qbittorrent", "deluge":
		if d.DownloaderCfg.DelugeDlTo != "" {
			return d.DownloaderCfg.DelugeDlTo
		}
	}

	return d.TargetCfg.Path
}

func (d *downloadertype) downloadNzbType(isType uint, savePath string) {
	if handler := mediatype.Get(isType); handler != nil {
		if err := handler.RecordDownloadHistory(d.Nzb, savePath); err != nil {
			logger.Logtype("error", 0).
				Err(err).
				Str("nzb", d.Nzb.NZB.Title).
				Msg("Error recording download history")
		}
	}
}

// getdownloadtargetfolder returns the target download folder path for a download based on whether it is a movie or TV show download.
// For movies it tries to use the movie title and IMDB ID if available.
// For TV shows it tries to use the episode title and TVDB ID if available.
// Falls back to just using the title if IDs are not available.
func (d *downloadertype) getdownloadtargetfolder() string {
	if handler := mediatype.Get(d.Cfgp.IsType); handler != nil {
		// Get the external ID from the appropriate db entity
		var externalID string
		switch d.Cfgp.IsType {
		case config.MediaTypeMovie:
			externalID = d.Dbmovie.ImdbID
		case config.MediaTypeSeries:
			if d.Dbserie.ThetvdbID != 0 {
				externalID = strconv.Itoa(d.Dbserie.ThetvdbID)
			}

		case config.MediaTypeBook:
			if d.Dbbook.GoodreadsID != "" {
				externalID = d.Dbbook.GoodreadsID
			} else if d.Dbbook.ISBN13 != "" {
				externalID = d.Dbbook.ISBN13
			}

		case config.MediaTypeAudiobook:
			if d.Dbaudiobook.AudibleID != "" {
				externalID = d.Dbaudiobook.AudibleID
			} else if d.Dbaudiobook.ASIN != "" {
				externalID = d.Dbaudiobook.ASIN
			}

		case config.MediaTypeMusic:
			if d.Dbalbum.MusicbrainzReleaseGroupID != "" {
				externalID = d.Dbalbum.MusicbrainzReleaseGroupID
			} else if d.Dbalbum.SpotifyID != "" {
				externalID = d.Dbalbum.SpotifyID
			}
		}

		if folder := handler.GetDownloadTargetFolder(d.Nzb, externalID); folder != "" {
			return folder
		}
	}

	// Fallback: return title with quality info
	if d.Nzb.NZB.Title == "" {
		return logger.JoinStrings(
			d.Nzb.Info.Title,
			"[",
			d.Nzb.Info.Resolution,
			logger.StrSpace,
			d.Nzb.Info.Quality,
			"]",
		)
	}

	return d.Nzb.NZB.Title
}

// notify sends notifications about the added download based on the
// configured notifications in the downloader. It supports pushover
// and CSV append notifications. It parses notification titles and
// messages as templates using the downloader data.
func (d *downloadertype) notify() {
	d.Time = logger.TimeGetNow().Format(logger.GetTimeFormat())

	var err error
	for idx := range d.Cfgp.Notification {
		if !strings.EqualFold(d.Cfgp.Notification[idx].Event, "added_download") {
			continue
		}

		if d.Cfgp.Notification[idx].CfgNotification == nil {
			continue
		}

		bl, messagetext, _ := logger.ParseStringTemplate(d.Cfgp.Notification[idx].Message, d)
		if bl {
			continue
		}

		cfgnot := d.Cfgp.Notification[idx].CfgNotification

		switch cfgnot.NotificationType {
		case "csv":
			scanner.AppendCsv(cfgnot.Outputto, messagetext)
		case "pushover", "gotify", "pushbullet", "apprise":
			bl, messageTitle, _ := logger.ParseStringTemplate(d.Cfgp.Notification[idx].Title, d)
			if bl {
				continue
			}

			err = providers.SendNotification(cfgnot, messagetext, messageTitle)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Str("type", cfgnot.NotificationType).
					Msg("Error sending notification")
			} else {
				logger.Logtype("info", 0).
					Str("type", cfgnot.NotificationType).
					Msg("Notification sent")
			}
		}
	}
}

// downloadByDrone downloads the NZB or torrent file using the Drone downloader.
// It constructs the filename based on Targetfile, downloads the file to the Path
// in TargetCfg using scanner.DownloadFile, and returns any error.
func (d *downloadertype) downloadByDrone() error {
	filename := (d.Targetfile + ".nzb")
	if d.Nzb.NZB.IsTorrent {
		filename = (d.Targetfile + ".torrent")
	}

	urlv := logger.Checkhtmlentities(d.Nzb.NZB.DownloadURL)

	return providers.EnsureIndexer(d.IndexerCfg).Download(context.Background(), urlv, d.TargetCfg.Path, filename)
}

// downloadByNzbget downloads the NZB file using the NZBGet downloader.
// It uses the new provider infrastructure from apiexternal_v2 for proper
// stats tracking, rate limiting, and error handling.
func (d *downloadertype) downloadByNzbget() error {
	// Get the NZBGet provider from the registry
	provider := providers.GetNZBGet(d.DownloaderCfg.Name)
	if provider == nil {
		return logger.ErrNotAllowed
	}

	// Use the new provider's AddNZBExtended method which handles:
	// - Downloading the NZB file from the URL
	// - Extracting the filename from headers or URL
	// - Encoding the content to base64
	// - Sending to NZBGet via JSON-RPC
	// All with proper stats tracking, rate limiting, and retry logic
	_, err := provider.AddNZBExtended(
		context.Background(),
		logger.Checkhtmlentities(d.Nzb.NZB.DownloadURL),
		d.Category,
		d.Targetfile,
		d.DownloaderCfg.Priority,
		d.DownloaderCfg.AddPaused,
	)

	return err
}

// downloadBySabnzbd sends the NZB URL to SABnzbd.
func (d *downloadertype) downloadBySabnzbd() error {
	provider := providers.GetSABnzbd(d.DownloaderCfg.Name)
	if provider == nil {
		return errNoDownloadClient
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return provider.AddNZBNamed(
		ctx,
		logger.Checkhtmlentities(d.Nzb.NZB.DownloadURL),
		d.Category,
		d.Targetfile,
		d.DownloaderCfg.Priority,
	)
}

// downloadByRTorrent sends the torrent URL to rTorrent.
func (d *downloadertype) downloadByRTorrent() error {
	return addTorrent(d, providers.GetRTorrent(d.DownloaderCfg.Name), apiexternal_v2.TorrentAddRequest{
		SavePath: d.DownloaderCfg.DelugeDlTo,
	})
}

// downloadByTransmission sends the torrent URL to Transmission.
func (d *downloadertype) downloadByTransmission() error {
	return addTorrent(d, providers.GetTransmission(d.DownloaderCfg.Name), apiexternal_v2.TorrentAddRequest{
		SavePath: d.DownloaderCfg.DelugeDlTo,
		Paused:   d.DownloaderCfg.AddPaused,
		Priority: d.DownloaderCfg.Priority,
	})
}

// downloadByDeluge sends the torrent URL to Deluge. A non-empty category is
// applied as label when Deluge's Label plugin is enabled.
func (d *downloadertype) downloadByDeluge() error {
	options := map[string]string{"move_completed": strconv.FormatBool(d.DownloaderCfg.DelugeMoveAfter)}
	// Like download_location, an empty move_completed_path would replace
	// Deluge's own default with "".
	if d.DownloaderCfg.DelugeMoveTo != "" {
		options["move_completed_path"] = d.DownloaderCfg.DelugeMoveTo
	}

	return addTorrent(d, providers.GetDeluge(d.DownloaderCfg.Name), apiexternal_v2.TorrentAddRequest{
		SavePath: d.DownloaderCfg.DelugeDlTo,
		Paused:   d.DownloaderCfg.AddPaused,
		Options:  options,
		Label:    d.Category,
	})
}

// downloadByQBittorrent sends the torrent URL to qBittorrent.
func (d *downloadertype) downloadByQBittorrent() error {
	return addTorrent(d, providers.GetQBittorrent(d.DownloaderCfg.Name), apiexternal_v2.TorrentAddRequest{
		SavePath: d.DownloaderCfg.DelugeDlTo,
		Category: d.Category,
		Paused:   d.DownloaderCfg.AddPaused,
	})
}

// torrentAdder is the add call the torrent download clients share.
type torrentAdder interface {
	AddTorrent(ctx context.Context, req apiexternal_v2.TorrentAddRequest) (*apiexternal_v2.TorrentAddResponse, error)
}

// addTorrent sends the release's torrent URL with req's settings to client.
func addTorrent[T any, P interface {
	*T
	torrentAdder
}](d *downloadertype, client P, req apiexternal_v2.TorrentAddRequest) error {
	if client == nil {
		return errNoDownloadClient
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req.URL = logger.Checkhtmlentities(d.Nzb.NZB.DownloadURL)
	_, err := client.AddTorrent(ctx, req)

	return err
}

// newDownloader initializes a new downloadertype struct.
// It takes in a media type config pointer and an NZB with priority struct pointer.
// It returns a pointer to a downloadertype struct initialized with the passed in config
// and NZB values.
func newDownloader(cfgp *config.MediaTypeConfig, nzb *apiexternal_v2.Nzbwithprio) *downloadertype {
	return &downloadertype{
		Cfgp: cfgp,
		Nzb:  nzb,
	}
}

// DownloadMovie initializes a downloader, gets the movie and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadMovie(cfgp *config.MediaTypeConfig, nzb *apiexternal_v2.Nzbwithprio) error {
	d := newDownloader(cfgp, nzb)

	err := d.Movie.GetMoviesByIDP(&nzb.NzbmovieID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("movie not found", nzb.NzbmovieID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Dbmovie.GetDbmovieByIDP(&d.Movie.DbmovieID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("dbmovie not found", d.Movie.DbmovieID).
			Err(err).
			Msg("not found")

		return err
	}

	d.Quality = database.GetMediaQualityConfig(cfgp, &nzb.NzbmovieID)
	return d.downloadNzb()
}

// DownloadSeriesEpisode initializes a downloader, gets the episode and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadSeriesEpisode(cfgp *config.MediaTypeConfig, nzb *apiexternal_v2.Nzbwithprio) error {
	d := newDownloader(cfgp, nzb)

	err := d.Serieepisode.GetSerieEpisodesByIDP(&nzb.NzbepisodeID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("episode not found", nzb.NzbepisodeID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Dbserie.GetDbserieByIDP(&d.Serieepisode.DbserieID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("dbserie not found", d.Serieepisode.DbserieID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Dbserieepisode.GetDbserieEpisodesByIDP(&d.Serieepisode.DbserieEpisodeID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("dbepisode not found", d.Serieepisode.DbserieEpisodeID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Serie.GetSerieByIDP(&d.Serieepisode.SerieID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("serie not found", d.Serieepisode.SerieID).
			Err(err).
			Msg("not found")

		return err
	}

	d.Quality = database.GetMediaQualityConfig(cfgp, &nzb.NzbepisodeID)
	return d.downloadNzb()
}

// DownloadBook initializes a downloader, gets the book and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadBook(cfgp *config.MediaTypeConfig, nzb *apiexternal_v2.Nzbwithprio) error {
	d := newDownloader(cfgp, nzb)

	err := d.Book.GetBooksByIDP(&nzb.NzbbookID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("book not found", nzb.NzbbookID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Dbbook.GetDbbookByIDP(&d.Book.DbbookID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("dbbook not found", d.Book.DbbookID).
			Err(err).
			Msg("not found")

		return err
	}

	d.Quality = database.GetMediaQualityConfig(cfgp, &nzb.NzbbookID)
	return d.downloadNzb()
}

// DownloadAudiobook initializes a downloader, gets the audiobook and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadAudiobook(cfgp *config.MediaTypeConfig, nzb *apiexternal_v2.Nzbwithprio) error {
	d := newDownloader(cfgp, nzb)

	err := d.Audiobook.GetAudiobooksByIDP(&nzb.NzbaudiobookID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("audiobook not found", nzb.NzbaudiobookID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Dbaudiobook.GetDbaudiobookByIDP(&d.Audiobook.DbaudiobookID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("dbaudiobook not found", d.Audiobook.DbaudiobookID).
			Err(err).
			Msg("not found")

		return err
	}

	d.Quality = database.GetMediaQualityConfig(cfgp, &nzb.NzbaudiobookID)
	return d.downloadNzb()
}

// DownloadAlbum initializes a downloader, gets the album and related database
// objects by ID, sets the quality config, and calls the download method.
func DownloadAlbum(cfgp *config.MediaTypeConfig, nzb *apiexternal_v2.Nzbwithprio) error {
	d := newDownloader(cfgp, nzb)

	err := d.Album.GetAlbumsByIDP(&nzb.NzbalbumID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("album not found", nzb.NzbalbumID).
			Err(err).
			Msg("not found")

		return err
	}

	err = d.Dbalbum.GetDbalbumByIDP(&d.Album.DbalbumID)
	if err != nil {
		logger.Logtype("error", 1).
			Uint("dbalbum not found", d.Album.DbalbumID).
			Err(err).
			Msg("not found")

		return err
	}

	d.Quality = database.GetMediaQualityConfig(cfgp, &nzb.NzbalbumID)
	return d.downloadNzb()
}

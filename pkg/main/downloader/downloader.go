package downloader

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/base"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloadstate"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
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

	before := d.snapshotDownloads()

	var (
		downloadID string
		err        error
	)

	if d.DownloaderCfg.DlType == "drone" {
		err = d.downloadByDrone()
	} else {
		downloadID, err = d.downloadByClient()
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

	if downloadID == "" && before != nil {
		downloadID = d.findNewDownload(before)
	}

	d.notify()

	d.downloadNzbType(d.Cfgp.IsType, d.savePath(), downloadID)

	return nil
}

// newDownloadWait bounds how long findNewDownload waits for a client to list
// a torrent added by URL (it fetches the .torrent in the background).
const newDownloadWait = 20 * time.Second

// stateLister returns the downloader's client when it can report download
// states, else nil.
func (d *downloadertype) stateLister() apiexternal_v2.DownloadStateLister {
	l, _ := providers.GetDownloadClient(d.DownloaderCfg.Name).(apiexternal_v2.DownloadStateLister)
	return l
}

// snapshotDownloads lists the torrent client's downloads before an add whose
// id cannot be known otherwise: a torrent URL (unlike a magnet) carries no
// info hash, and some clients' add calls return none.
func (d *downloadertype) snapshotDownloads() map[string]apiexternal_v2.DownloadState {
	if !providers.IsTorrentClient(d.DownloaderCfg.DlType) || base.InfoHashFromURL(d.Nzb.NZB.DownloadURL) != "" {
		return nil
	}

	l := d.stateLister()
	if l == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	states, err := l.DownloadStates(ctx)
	if err != nil {
		return nil
	}

	return states
}

// findNewDownload returns the id of the download that appeared since before;
// of several new ones the one named like the release.
func (d *downloadertype) findNewDownload(before map[string]apiexternal_v2.DownloadState) string {
	l := d.stateLister()
	if l == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), newDownloadWait+10*time.Second)
	defer cancel()

	for deadline := time.Now().Add(newDownloadWait); time.Now().Before(deadline); {
		now, err := l.DownloadStates(ctx)
		if err != nil {
			return ""
		}

		var added []string
		for id, st := range now {
			if _, ok := before[id]; !ok {
				if strings.EqualFold(st.Name, d.Nzb.NZB.Title) {
					return id
				}

				added = append(added, id)
			}
		}

		if len(added) == 1 {
			return added[0]
		}

		if len(added) > 1 {
			return ""
		}

		select {
		case <-ctx.Done():
			return ""
		case <-time.After(time.Second):
		}
	}

	return ""
}

// savePath returns where the client actually saves the download: torrent
// clients use DelugeDlTo (when set) rather than the path template.
func (d *downloadertype) savePath() string {
	if d.DownloaderCfg.DelugeDlTo != "" && providers.IsTorrentClient(d.DownloaderCfg.DlType) {
		return d.DownloaderCfg.DelugeDlTo
	}

	return d.TargetCfg.Path
}

// downloadNzbType records the download in the history, with the client
// and its download id for the download state tracking.
func (d *downloadertype) downloadNzbType(isType uint, savePath, downloadID string) {
	if handler := mediatype.Get(isType); handler != nil {
		historyID, err := handler.RecordDownloadHistory(d.Nzb, savePath)
		if err != nil {
			logger.Logtype("error", 0).
				Err(err).
				Str("nzb", d.Nzb.NZB.Title).
				Msg("Error recording download history")

			return
		}

		downloadstate.Started(isType, historyID, d.DownloaderCfg.Name, downloadID)
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

// notify sends the media config's added_download notifications, rendered
// with the downloader data.
func (d *downloadertype) notify() {
	d.Time = logger.TimeGetNow().Format(logger.GetTimeFormat())

	providers.NotifyEvent(d.Cfgp, providers.EventAddedDownload, d, nil)
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

// nzbURLAdder is the add call of the Usenet clients; it returns the job id.
type nzbURLAdder interface {
	AddNZBURL(ctx context.Context, nzbURL, category, name string, priority int, paused bool) (string, error)
}

// downloadByClient sends the release to the downloader's client and returns
// the client's id for it. Every torrent client gets the same settings and
// uses those it supports; the category doubles as label/tag.
func (d *downloadertype) downloadByClient() (string, error) {
	client := providers.GetDownloadClient(d.DownloaderCfg.Name)
	if client == nil {
		if !slices.Contains(providers.DownloadClientTypeNames(), d.DownloaderCfg.DlType) {
			return "", errUnknownDownloader
		}

		return "", errNoDownloadClient
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	switch c := client.(type) {
	case torrentAdder:
		url := logger.Checkhtmlentities(d.Nzb.NZB.DownloadURL)

		resp, err := c.AddTorrent(ctx, apiexternal_v2.TorrentAddRequest{
			URL:      url,
			SavePath: d.DownloaderCfg.DelugeDlTo,
			Category: d.Category,
			Label:    d.Category,
			Paused:   d.DownloaderCfg.AddPaused,
			Priority: d.DownloaderCfg.Priority,
			Options:  d.delugeOptions(),
		})
		if err != nil {
			return "", err
		}

		return addedID(resp, url), nil
	case nzbURLAdder:
		return c.AddNZBURL(ctx, logger.Checkhtmlentities(d.Nzb.NZB.DownloadURL), d.Category, d.Targetfile,
			d.DownloaderCfg.Priority, d.DownloaderCfg.AddPaused)
	}

	return "", errUnknownDownloader
}

// delugeOptions are Deluge's move-after-completion settings; other clients
// read Options differently (qBittorrent passes them on), so only Deluge
// gets them.
func (d *downloadertype) delugeOptions() map[string]string {
	if d.DownloaderCfg.DlType != "deluge" {
		return nil
	}

	options := map[string]string{"move_completed": strconv.FormatBool(d.DownloaderCfg.DelugeMoveAfter)}
	// Like download_location, an empty move_completed_path would replace
	// Deluge's own default with "".
	if d.DownloaderCfg.DelugeMoveTo != "" {
		options["move_completed_path"] = d.DownloaderCfg.DelugeMoveTo
	}

	return options
}

// addedID is the id of an added torrent: the hash the client returned, else
// the info hash of a magnet link. aria2's own job id ("gid ...") wins, as
// its calls take only that.
func addedID(resp *apiexternal_v2.TorrentAddResponse, rawURL string) string {
	if resp != nil {
		if gid, ok := strings.CutPrefix(resp.Message, "gid "); ok {
			return gid
		}

		if resp.Hash != "" {
			return resp.Hash
		}
	}

	return base.InfoHashFromURL(rawURL)
}

// torrentAdder is the add call the torrent download clients share.
type torrentAdder interface {
	AddTorrent(ctx context.Context, req apiexternal_v2.TorrentAddRequest) (*apiexternal_v2.TorrentAddResponse, error)
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

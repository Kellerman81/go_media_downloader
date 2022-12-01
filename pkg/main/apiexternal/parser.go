package apiexternal

import (
	"strings"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"go.uber.org/zap"
)

type ParseInfo struct {
	File             string
	Title            string
	Season           int      `json:"season,omitempty"`
	Episode          int      `json:"episode,omitempty"`
	SeasonStr        string   `json:"seasonstr,omitempty"`
	EpisodeStr       string   `json:"episodestr,omitempty"`
	Year             int      `json:"year,omitempty"`
	Resolution       string   `json:"resolution,omitempty"`
	ResolutionID     uint     `json:"resolutionid,omitempty"`
	Quality          string   `json:"quality,omitempty"`
	QualityID        uint     `json:"qualityid,omitempty"`
	Codec            string   `json:"codec,omitempty"`
	CodecID          uint     `json:"codecid,omitempty"`
	Audio            string   `json:"audio,omitempty"`
	AudioID          uint     `json:"audioid,omitempty"`
	Priority         int      `json:"priority,omitempty"`
	Identifier       string   `json:"identifier,omitempty"`
	Date             string   `json:"date,omitempty"`
	Extended         bool     `json:"extended,omitempty"`
	Proper           bool     `json:"proper,omitempty"`
	Repack           bool     `json:"repack,omitempty"`
	Imdb             string   `json:"imdb,omitempty"`
	Tvdb             string   `json:"tvdb,omitempty"`
	QualitySet       string   `json:"qualityset,omitempty"`
	Languages        []string `json:"languages,omitempty"`
	Runtime          int      `json:"runtime,omitempty"`
	Height           int      `json:"height,omitempty"`
	Width            int      `json:"width,omitempty"`
	DbmovieID        uint     `json:"dbmovieid,omitempty"`
	MovieID          uint     `json:"movieid,omitempty"`
	DbserieID        uint     `json:"dbserieid,omitempty"`
	DbserieEpisodeID uint     `json:"dbserieepisodeid,omitempty"`
	SerieID          uint     `json:"serieid,omitempty"`
	SerieEpisodeID   uint     `json:"serieepisodeid,omitempty"`
	Listname         string   `json:"listname,omitempty"`
	//Group           string   `json:"group,omitempty"`
	//Region          string   `json:"region,omitempty"`
	//Hardcoded       bool     `json:"hardcoded,omitempty"`
	//Container       string   `json:"container,omitempty"`
	//Widescreen      bool     `json:"widescreen,omitempty"`
	//Website         string   `json:"website,omitempty"`
	//Sbs             string   `json:"sbs,omitempty"`
	//Unrated         bool     `json:"unrated,omitempty"`
	//Subs            string   `json:"subs,omitempty"`
	//ThreeD          bool     `json:"3d,omitempty"`
}

func (s *ParseInfo) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		if len(s.Languages) >= 1 {
			s.Languages = nil
		}
		s = nil
	}
}

func before(value string, index int) string {
	if index <= 0 {
		return ""
	}
	return value[index-1 : index]
}

func after(value string, index int) string {
	if index >= len(value) {
		return ""
	}
	return value[index : index+1]
}

func (m *ParseInfo) Parsegroup(tolower string, name string, group []string) {
	var index, lengroup int
	var substr, substrpre, substrpost string

	for idx := range group {
		substr = ""
		if strings.Contains(tolower, group[idx]) {
			lengroup = len(group[idx])
			index = strings.Index(tolower, group[idx])
			//substr = strings.Repeat(tolower[index:index+len(group[idx])], 1)
			substr = tolower[index : index+lengroup]
			substrpre = before(tolower, index)
			substrpost = after(tolower, index+lengroup)
			if substrpost != "" {
				if unicode.IsDigit([]rune(substrpost)[0]) || unicode.IsLetter([]rune(substrpost)[0]) {
					continue
				}
			}
			if substrpre != "" {
				if unicode.IsDigit([]rune(substrpre)[0]) || unicode.IsLetter([]rune(substrpre)[0]) {
					continue
				}
			}
			switch name {
			case "audio":
				m.Audio = substr
			case "codec":
				m.Codec = substr
			case "quality":
				m.Quality = substr
			case "resolution":
				m.Resolution = substr
			case "extended":
				if substr != "" {
					m.Extended = true
				}
			case "proper":
				if substr != "" {
					m.Proper = true
				}
			case "repack":
				if substr != "" {
					m.Repack = true
				}
			}
			break
		}
	}
}

type Nzbwithprio struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              NZB
	NzbmovieID       uint
	NzbepisodeID     uint
	WantedTitle      string
	WantedAlternates []string
	QualityTemplate  string
	MinimumPriority  int
	Denied           bool
	Reason           string
}

func (s *Nzbwithprio) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		s.ParseInfo.Close()
		s.WantedAlternates = nil
		s = nil
	}
}

type NzbwithprioJson struct {
	Prio             int
	Indexer          string
	ParseInfo        *ParseInfo
	NZB              *NZB
	NzbmovieID       uint
	NzbepisodeID     uint
	WantedTitle      string
	WantedAlternates []string
	QualityTemplate  string
	MinimumPriority  int
	Denied           bool
	Reason           string
}

// NZB represents an NZB found on the index
type NZB struct {
	ID    string `json:"id,omitempty"`
	Title string `json:"title,omitempty"`
	//Description string    `json:"description,omitempty"`
	Size int64 `json:"size,omitempty"`
	//AirDate     time.Time `json:"air_date,omitempty"`
	//PubDate time.Time `json:"pub_date,omitempty"`
	//UsenetDate  time.Time `json:"usenet_date,omitempty"`
	//NumGrabs    int       `json:"num_grabs,omitempty"`

	SourceEndpoint string `json:"source_endpoint"`
	//SourceAPIKey   string `json:"source_apikey"`

	//Category []string `json:"category,omitempty"`
	//Info     string   `json:"info,omitempty"`
	//Genre    string   `json:"genre,omitempty"`

	//Resolution string `json:"resolution,omitempty"`
	//Poster     string `json:"poster,omitempty"`
	//Group      string `json:"group,omitempty"`

	// TV Specific stuff
	TVDBID  string `json:"tvdbid,omitempty"`
	Season  string `json:"season,omitempty"`
	Episode string `json:"episode,omitempty"`
	//TVTitle string `json:"tvtitle,omitempty"`
	//Rating  int    `json:"rating,omitempty"`

	// Movie Specific stuff
	IMDBID string `json:"imdb,omitempty"`
	//IMDBTitle string  `json:"imdbtitle,omitempty"`
	//IMDBYear  int     `json:"imdbyear,omitempty"`
	//IMDBScore float32 `json:"imdbscore,omitempty"`
	//CoverURL  string  `json:"coverurl,omitempty"`

	// Torznab specific stuff
	//Seeders     int    `json:"seeders,omitempty"`
	//Peers       int    `json:"peers,omitempty"`
	//InfoHash    string `json:"infohash,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
	IsTorrent   bool   `json:"is_torrent,omitempty"`
}

type NZBArr struct {
	Arr []Nzbwithprio
}

func (s *NZBArr) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		for sr := range s.Arr {
			s.Arr[sr].Close()
		}
		s.Arr = nil
		s = nil
	}
}

//const requirednotmatched string = "Skipped - required not matched"
//const regexrejected string = "Skipped - Regex rejected"

func (entry *Nzbwithprio) FilterRegexNzbs(templateregex string, title string) (bool, string) {
	if templateregex == "" {
		logger.Log.GlobalLogger.Debug("Skipped - regex_template empty", zap.String("regex", title))
		return true, ""
	}
	var breakfor bool

	requiredmatched := false
	for idx := range config.Cfg.Regex[templateregex].Required {
		if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Required[idx], title, 1) {
			requiredmatched = true
			break
		}
	}
	if len(config.Cfg.Regex[templateregex].Required) >= 1 && !requiredmatched {
		//logger.Log.GlobalLogger.Debug(requirednotmatched, zap.String("regex", title))
		return true, "required not matched"
	}
	for idx := range config.Cfg.Regex[templateregex].Rejected {
		if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Rejected[idx], entry.WantedTitle, 1) {
			//Regex is in title - skip test
			continue
		}
		breakfor = false
		for idxwanted := range entry.WantedAlternates {
			if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Rejected[idx], entry.WantedAlternates[idxwanted], 1) {
				breakfor = true
				break
			}
		}
		if breakfor {
			//Regex is in alternate title - skip test
			continue
		}
		if config.RegexGetMatchesFind(config.Cfg.Regex[templateregex].Rejected[idx], title, 1) {
			//logger.Log.GlobalLogger.Debug(regexrejected, zap.String("title", title), zap.String("regex", config.Cfg.Regex[templateregex].Rejected[idx]))
			return true, config.Cfg.Regex[templateregex].Rejected[idx]
		}
	}
	return false, ""
}
func (nzb *Nzbwithprio) Getnzbconfig(quality string) (string, string, string) {
	if !config.ConfigCheck("quality_" + quality) {
		return "", "", ""
	}

	for idx := range config.Cfg.Quality[quality].Indexer {
		if strings.EqualFold(config.Cfg.Quality[quality].Indexer[idx].TemplateIndexer, nzb.Indexer) {
			if !config.ConfigCheck("path_" + config.Cfg.Quality[quality].Indexer[idx].TemplatePathNzb) {
				continue
			}

			if !config.ConfigCheck("downloader_" + config.Cfg.Quality[quality].Indexer[idx].TemplateDownloader) {
				continue
			}
			if config.Cfg.Quality[quality].Indexer[idx].CategoryDowloader != "" {
				logger.Log.GlobalLogger.Debug("Downloader nzb config found - category", zap.String("category", config.Cfg.Quality[quality].Indexer[idx].CategoryDowloader))
				logger.Log.GlobalLogger.Debug("Downloader nzb config found - pathconfig", zap.String("path template", config.Cfg.Quality[quality].Indexer[idx].TemplatePathNzb))
				logger.Log.GlobalLogger.Debug("Downloader nzb config found - dlconfig", zap.String("downloader template", config.Cfg.Quality[quality].Indexer[idx].TemplateDownloader))
				logger.Log.GlobalLogger.Debug("Downloader nzb config found - target", zap.String("path", config.Cfg.Paths[config.Cfg.Quality[quality].Indexer[idx].TemplatePathNzb].Path))
				logger.Log.GlobalLogger.Debug("Downloader nzb config found - downloader", zap.String("downloader type", config.Cfg.Downloader[config.Cfg.Quality[quality].Indexer[idx].TemplateDownloader].DlType))
				logger.Log.GlobalLogger.Debug("Downloader nzb config found - downloader", zap.String("downloader", config.Cfg.Downloader[config.Cfg.Quality[quality].Indexer[idx].TemplateDownloader].Name))
				return config.Cfg.Quality[quality].Indexer[idx].CategoryDowloader, config.Cfg.Quality[quality].Indexer[idx].TemplatePathNzb, config.Cfg.Quality[quality].Indexer[idx].TemplateDownloader
			}
		}
	}
	indexer := config.Cfg.Quality[quality].Indexer[0]
	logger.Log.GlobalLogger.Debug("Downloader nzb config NOT found - quality", zap.String("Quality", quality))
	if !config.ConfigCheck("path_" + indexer.TemplatePathNzb) {
		return "", "", ""
	}

	if !config.ConfigCheck("downloader_" + indexer.TemplateDownloader) {
		return "", "", ""
	}
	logger.Log.GlobalLogger.Debug("Downloader nzb config NOT found - use first", zap.String("categories", indexer.CategoryDowloader))

	return indexer.CategoryDowloader, indexer.TemplatePathNzb, indexer.TemplateDownloader
}

func Checknzbtitle(movietitle string, nzbtitle string) bool {
	if movietitle == nzbtitle {
		return true
	} else if strings.EqualFold(movietitle, nzbtitle) {
		return true
	} else {
		return strings.EqualFold(logger.StringToSlug(movietitle), logger.StringToSlug(nzbtitle))
	}
}

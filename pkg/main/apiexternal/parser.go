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

type Nzbwithprio struct {
	Prio             int
	Indexer          string
	ParseInfo        ParseInfo
	NZB              NZB
	NzbmovieID       uint
	NzbepisodeID     uint
	Dbid             uint
	WantedTitle      string
	WantedAlternates []string
	QualityTemplate  string
	MinimumPriority  int
	Denied           bool
	Reason           string
	AdditionalReason interface{}
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
	TVDBID  int    `json:"tvdbid,omitempty"`
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

func (s *ParseInfo) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	if len(s.Languages) >= 1 {
		s.Languages = nil
	}
	s = nil
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

func (s *ParseInfo) Parsegroup(tolower string, name string, group *logger.InStringArrayStruct) {
	var index, lengroup int
	var substr, substrpre, substrpost string

	for idx := range group.Arr {
		if !strings.Contains(tolower, group.Arr[idx]) {
			continue
		}
		lengroup = len(group.Arr[idx])
		index = strings.Index(tolower, group.Arr[idx])
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
			s.Audio = substr
		case "codec":
			s.Codec = substr
		case "quality":
			s.Quality = substr
		case "resolution":
			s.Resolution = substr
		case "extended":
			if substr != "" {
				s.Extended = true
			}
		case "proper":
			if substr != "" {
				s.Proper = true
			}
		case "repack":
			if substr != "" {
				s.Repack = true
			}
		}
		break
	}
}

func (s *Nzbwithprio) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.ParseInfo.Close()
	s.WantedAlternates = nil
	s = nil
}

func (s *NZBArr) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	for sr := range s.Arr {
		s.Arr[sr].Close()
	}
	s.Arr = nil
	s = nil
}

//const requirednotmatched string = "Skipped - required not matched"
//const regexrejected string = "Skipped - Regex rejected"

func (s *Nzbwithprio) Getnzbconfig(quality string) (string, string, string) {
	if !config.Check("quality_" + quality) {
		return "", "", ""
	}

	cfgqual := config.Cfg.Quality[quality]
	defer cfgqual.Close()
	for idx := range cfgqual.Indexer {
		if strings.EqualFold(cfgqual.Indexer[idx].TemplateIndexer, s.Indexer) {
			if !config.Check("path_" + cfgqual.Indexer[idx].TemplatePathNzb) {
				continue
			}

			if !config.Check("downloader_" + cfgqual.Indexer[idx].TemplateDownloader) {
				continue
			}
			if cfgqual.Indexer[idx].CategoryDowloader != "" {
				logger.Log.Debug("Quality ", cfgqual.Indexer[idx], " Downloader ", config.Cfg.Downloader[cfgqual.Indexer[idx].TemplateDownloader])
				return cfgqual.Indexer[idx].CategoryDowloader, cfgqual.Indexer[idx].TemplatePathNzb, cfgqual.Indexer[idx].TemplateDownloader
			}
		}
	}
	// defer indexer.Close()
	logger.Log.GlobalLogger.Debug("Downloader nzb config NOT found - quality", zap.Stringp("Quality", &quality))
	if !config.Check("path_" + cfgqual.Indexer[0].TemplatePathNzb) {
		return "", "", ""
	}

	if !config.Check("downloader_" + cfgqual.Indexer[0].TemplateDownloader) {
		return "", "", ""
	}
	logger.Log.GlobalLogger.Debug("Downloader nzb config NOT found - use first", zap.Stringp("categories", &cfgqual.Indexer[0].CategoryDowloader))

	return cfgqual.Indexer[0].CategoryDowloader, cfgqual.Indexer[0].TemplatePathNzb, cfgqual.Indexer[0].TemplateDownloader
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

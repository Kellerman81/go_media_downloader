package apiexternal

import (
	"errors"
	"strings"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
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

type FileParser struct {
	Str                string
	M                  ParseInfo
	TypeGroup          string
	IncludeYearInTitle bool
}

func (s *FileParser) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.M.Close()
	logger.ClearVar(s)
}

type Nzbwithprio struct {
	Prio int
	//Indexer          string
	ParseInfo        *FileParser
	NZB              *NZB
	NzbmovieID       uint
	NzbepisodeID     uint
	Dbid             uint
	WantedTitle      string
	WantedAlternates []string
	QualityTemplate  string
	MinimumPriority  int
	Reason           string
	AdditionalReason string
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

	Indexer string `json:"indexer,omitempty"`
	Quality string `json:"quality,omitempty"`
}

func (s *NZB) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.ClearVar(s)
}
func (s *ParseInfo) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.Languages)
	logger.ClearVar(s)
}

func Before(value string, index int) string {
	if index <= 0 {
		return ""
	}
	return value[index-1 : index]
}

func After(value string, index int) string {
	if index >= len(value) {
		return ""
	}
	return value[index : index+1]
}

func CheckDigitLetter(str string) bool {
	if str == "" {
		return true
	}
	//runev := []runestr0]
	if unicode.IsDigit([]rune(str)[0]) || unicode.IsLetter([]rune(str)[0]) {
		return false
	}
	return true
}

func (s *ParseInfo) Parsegroup(tolower string, name string, group *[]string) {
	var index int
	var substr string
	for idx := range *group {
		//if !logger.ContainsI(tolower, &(*group)[idx]) {
		//	continue
		//}
		//lengroup = len((*group)[idx])
		index = logger.IndexI(tolower, (*group)[idx])
		if index == -1 {
			continue
		}
		//substr = strings.Repeat(tolower[index:index+len(group[idx])], 1)

		substr = tolower[index : index+len((*group)[idx])]
		if substr == "" {
			continue
		}
		if !CheckDigitLetter(After(tolower, index+len((*group)[idx]))) {
			continue
		}
		if !CheckDigitLetter(Before(tolower, index)) {
			continue
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
			s.Extended = true
		case "proper":
			s.Proper = true
		case "repack":
			s.Repack = true
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
	s.NZB.Close()
	//s.QualityCfg.Close()
	//logger.ClearVar(&s.NZB)
	s.ParseInfo.Close()
	logger.Clear(&s.WantedAlternates)
	logger.ClearVar(s)
}

//const requirednotmatched string = "Skipped - required not matched"
//const regexrejected string = "Skipped - Regex rejected"

func (s *Nzbwithprio) Getnzbconfig(quality string) (string, string, string, error) {
	if !config.CheckGroup("quality_", quality) {
		return "", "", "", errors.New("quality template not found")
	}

	for idx := range config.SettingsQuality["quality_"+quality].Indexer {
		if !strings.EqualFold(config.SettingsQuality["quality_"+quality].Indexer[idx].TemplateIndexer, s.NZB.Indexer) {
			continue
		}
		if !config.CheckGroup("path_", config.SettingsQuality["quality_"+quality].Indexer[idx].TemplatePathNzb) {
			continue
		}

		if !config.CheckGroup("downloader_", config.SettingsQuality["quality_"+quality].Indexer[idx].TemplateDownloader) {
			continue
		}
		if config.SettingsQuality["quality_"+quality].Indexer[idx].CategoryDowloader != "" {
			logger.Log.Debug().Str(logger.StrIndexer, config.SettingsQuality["quality_"+quality].Indexer[idx].TemplateIndexer).Str("Downloader", config.SettingsQuality["quality_"+quality].Indexer[idx].TemplateDownloader).Msg("Download")
			return config.SettingsQuality["quality_"+quality].Indexer[idx].CategoryDowloader, config.SettingsQuality["quality_"+quality].Indexer[idx].TemplatePathNzb, config.SettingsQuality["quality_"+quality].Indexer[idx].TemplateDownloader, nil
		}
	}

	// defer indexer.Close()
	logger.Log.Debug().Str("Quality", quality).Msg("Downloader nzb config NOT found - quality")

	if !config.CheckGroup("path_", config.SettingsQuality["quality_"+quality].Indexer[0].TemplatePathNzb) {
		return "", "", "", errors.New("path template not found")
	}

	if !config.CheckGroup("downloader_", config.SettingsQuality["quality_"+quality].Indexer[0].TemplateDownloader) {
		return "", "", "", errors.New("downloader template not found")
	}
	logger.Log.Debug().Str("categories", config.SettingsQuality["quality_"+quality].Indexer[0].CategoryDowloader).Msg("Downloader nzb config NOT found - use first")

	return config.SettingsQuality["quality_"+quality].Indexer[0].CategoryDowloader, config.SettingsQuality["quality_"+quality].Indexer[0].TemplatePathNzb, config.SettingsQuality["quality_"+quality].Indexer[0].TemplateDownloader, nil
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

func ChecknzbtitleB(movietitle string, movietitleslug string, nzbtitle string) bool {
	if movietitle == nzbtitle {
		return true
	} else if strings.EqualFold(movietitle, nzbtitle) {
		return true
	} else {
		return strings.EqualFold(movietitleslug, logger.StringToSlug(nzbtitle))
	}
}

func Buildparsedstring(m *ParseInfo) string {
	var bld strings.Builder
	bld.Grow(200)
	// if !logger.DisableVariableCleanup {
	// 	defer bld.Reset()
	// }
	if m.AudioID != 0 {
		bld.WriteString(" Audioid: ")
		bld.WriteString(logger.UintToString(m.AudioID))
	}
	if m.CodecID != 0 {
		bld.WriteString(" Codecid: ")
		bld.WriteString(logger.UintToString(m.CodecID))
	}
	if m.QualityID != 0 {
		bld.WriteString(" Qualityid: ")
		bld.WriteString(logger.UintToString(m.QualityID))
	}
	if m.ResolutionID != 0 {
		bld.WriteString(" Resolutionid: ")
		bld.WriteString(logger.UintToString(m.ResolutionID))
	}
	if m.EpisodeStr != "" {
		bld.WriteString(" Episode: ")
		bld.WriteString(m.EpisodeStr)
	}
	if m.Identifier != "" {
		bld.WriteString(" Identifier: ")
		bld.WriteString(m.Identifier)
	}
	if m.Listname != "" {
		bld.WriteString(" Listname: ")
		bld.WriteString(m.Listname)
	}
	if m.SeasonStr != "" {
		bld.WriteString(" Season: ")
		bld.WriteString(m.SeasonStr)
	}
	if m.Title != "" {
		bld.WriteString(" Title: ")
		bld.WriteString(m.Title)
	}
	if m.Tvdb != "" {
		bld.WriteString(" Tvdb: ")
		bld.WriteString(m.Tvdb)
	}
	if m.Imdb != "" {
		bld.WriteString(" Imdb: ")
		bld.WriteString(m.Imdb)
	}
	if m.Year != 0 {
		bld.WriteString(" Year: ")
		bld.WriteString(logger.IntToString(m.Year))
	}
	return bld.String()
}

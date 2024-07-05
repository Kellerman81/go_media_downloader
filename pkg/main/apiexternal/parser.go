package apiexternal

import (
	"encoding/xml"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

// Nzbwithprio is a struct containing information about an NZB found on the index
// It includes the parsed file name info, the NZB details, IDs, title,
// alternate titles, quality, list name, priority, reasons, and search flags
type Nzbwithprio struct {
	Info             database.ParseInfo                 // The parsed file name information
	NZB              Nzb                                // The NZB details
	NzbmovieID       uint                               // The associated movie ID if this is a movie
	NzbepisodeID     uint                               // The associated episode ID if this is a TV episode
	Dbid             uint                               // The DBMovie or DBEpisode ID
	WantedTitle      string                             // The wanted title for this download
	WantedAlternates []database.DbstaticTwoStringOneInt // Alternate wanted titles
	Quality          string                             // The quality of this NZB
	Listname         string                             // The name of the list this NZB is from
	MinimumPriority  int                                // The minimum priority level
	Reason           string                             // The reason for denying this NZB
	AdditionalReason any                                // Any additional reason details
	DontSearch       bool                               // Whether to avoid searching for this
	DontUpgrade      bool                               // Whether to avoid upgrading this
}

// NZB represents an NZB found on the index
type Nzb struct {
	// ID is the unique identifier for the NZB
	ID string `json:"id,omitempty"`

	// Title is the title of the content
	Title string `json:"title,omitempty"`

	// Size is the size of the NZB in bytes
	Size int64 `json:"size,omitempty"`

	// SourceEndpoint is the endpoint of the NZB source
	SourceEndpoint string `json:"source_endpoint"`

	// TVDBID is the TVDB ID if this NZB is for a TV show
	TVDBID int `json:"tvdbid,omitempty"`

	// Season is the season number if this NZB is for a TV show
	Season string `json:"season,omitempty"`

	// Episode is the episode number if this NZB is for a TV show
	Episode string `json:"episode,omitempty"`

	// IMDBID is the IMDb ID if this NZB is for a movie
	IMDBID string `json:"imdb,omitempty"`

	// DownloadURL is the URL to download the NZB
	DownloadURL string `json:"download_url,omitempty"`

	// IsTorrent indicates if this NZB is a torrent
	IsTorrent bool `json:"is_torrent,omitempty"`

	// Indexer is a pointer to the indexer config for this NZB
	Indexer *config.IndexersConfig

	// Quality is a pointer to the quality config for this NZB
	Quality *config.QualityConfig

	//TVTitle string `json:"tvtitle,omitempty"`
	//Rating  int    `json:"rating,omitempty"`
	//IMDBTitle string  `json:"imdbtitle,omitempty"`
	//IMDBYear  int     `json:"imdbyear,omitempty"`
	//IMDBScore float32 `json:"imdbscore,omitempty"`
	//CoverURL  string  `json:"coverurl,omitempty"`
	//Seeders     int    `json:"seeders,omitempty"`
	//Peers       int    `json:"peers,omitempty"`
	//InfoHash    string `json:"infohash,omitempty"`
	//Description string    `json:"description,omitempty"`
	//AirDate     time.Time `json:"air_date,omitempty"`
	//PubDate time.Time `json:"pub_date,omitempty"`
	//UsenetDate  time.Time `json:"usenet_date,omitempty"`
	//NumGrabs    int       `json:"num_grabs,omitempty"`
	//SourceAPIKey   string `json:"source_apikey"`

	//Category []string `json:"category,omitempty"`
	//Info     string   `json:"info,omitempty"`
	//Genre    string   `json:"genre,omitempty"`

	//Resolution string `json:"resolution,omitempty"`
	//Poster     string `json:"poster,omitempty"`
	//Group      string `json:"group,omitempty"`
}

var PLNzbwithprio = pool.NewPool(100, 10, nil, func(b *Nzbwithprio) {
	b.ClearArr()
	b.AdditionalReason = nil
	*b = Nzbwithprio{}
})

// Close closes the Nzbwithprio by closing the Info field, setting the
// WantedAlternates field to nil if it has a capacity >= 1, and clearing
// the Nzbwithprio with the logger.
func (s *Nzbwithprio) Close() {
	if s == nil {
		return
	}
	PLNzbwithprio.Put(s)
	// s.NZB.Indexer = nil
	// s.NZB.Quality = nil
	// s.ClearArr()
	// *s = Nzbwithprio{}
}

// getregexcfg returns the regex configuration for the given quality config
// that matches the indexer name in the given Nzbwithprio entry. It first checks
// the Indexer list in the quality config, and falls back to the SettingsList
// global config if no match is found. Returns nil if no match is found.
func (s *Nzbwithprio) Getregexcfg(qual *config.QualityConfig) *config.RegexConfig {
	if s.NZB.Indexer != nil {
		indcfg := qual.QualityIndexerByQualityAndTemplate(s.NZB.Indexer)
		if indcfg != nil {
			return indcfg.CfgRegex
		}
		if _, ok := config.SettingsList[s.NZB.Indexer.Name]; ok {
			return qual.Indexer[0].CfgRegex
		}
		if s.NZB.Indexer.Getlistbyindexer() != nil {
			return qual.Indexer[0].CfgRegex
		}
	}
	return nil
}

// ClearArr clears the WantedAlternates slice and calls ClearArr on the Info field.
// This method is used to reset the state of an Nzbwithprio instance.
func (s *Nzbwithprio) ClearArr() {
	if s == nil {
		return
	}
	s.NZB.Indexer = nil
	s.NZB.Quality = nil
	clear(s.WantedAlternates)
	s.WantedAlternates = nil
	s.Info.ClearArr()
}

// saveAttributes populates the fields of the NZB struct from
// the name/value pairs passed in. It handles translating the
// values to the appropriate types for the NZB struct fields.
func (n *Nzb) saveAttributes(name string, value string) {
	switch name {
	case strguid:
		n.ID = value
	case "tvdbid":
		n.TVDBID = logger.StringToInt(value)
	case "season":
		n.Season = value
	case "episode":
		n.Episode = value
	case logger.StrImdb:
		n.IMDBID = value
	case strsize:
		n.Size = logger.StringToInt64(value)
	}
}

// setfield sets the corresponding field in the Nzb struct based on the provided field name and value.
// If the field is already set, it will not be overwritten.
// The supported fields are:
// - Title
// - DownloadURL
// - ID
// - Size
// - IMDBID
// - TVDBID
// - Season
// - Episode
func (n *Nzb) setfield(field string, val any) {
	var value string
	switch tt := val.(type) {
	case string:
		value = tt
	case *string:
		if tt != nil {
			value = *tt
		}
	case []byte:
		value = string(tt)
	case xml.CharData:
		value = string(tt)
	default:
		return
	}
	switch field {
	case strtitle:
		if n.Title == "" {
			n.Title = value
		}
	case strlink, "url":
		if n.DownloadURL == "" {
			n.DownloadURL = value
		}
	case strguid:
		if n.ID == "" {
			n.ID = value
		}
	case strsize, "length":
		if n.Size == 0 {
			n.Size = logger.StringToInt64(value)
		}
	case logger.StrImdb:
		if n.IMDBID == "" {
			n.IMDBID = value
			if value != "" {
				logger.AddImdbPrefixP(&n.IMDBID)
			}
		}
	case "tvdbid":
		if n.TVDBID == 0 {
			n.TVDBID = logger.StringToInt(value)
		}
	case "season":
		if n.Season == "" {
			n.Season = value
		}
	case "episode":
		if n.Episode == "" {
			n.Episode = value
		}
	}
}

// Clear resets the fields of an Nzb struct to their zero values.
// This is useful for reusing an Nzb instance without creating a new one.
func (n *Nzb) Clear(ind *config.IndexersConfig, qual *config.QualityConfig, baseurl string) {
	if n == nil {
		return
	}
	n.Indexer = nil
	n.Quality = nil
	*n = Nzb{}
	n.Indexer = ind
	n.Quality = qual
	n.SourceEndpoint = baseurl
}

// GenerateIdentifierStringFromInt generates a season/episode identifier string
// from the given season and episode integers. It pads each number with leading
// zeros to ensure a consistent format like "S01E02". This is intended to generate
// identifiers for public display/logging.
func generateIdentifierStringFromInt(season int, episode int) string {
	return logger.JoinStrings("S", padNumberWithZero(season), "E", padNumberWithZero(episode)) //JoinStrings
}

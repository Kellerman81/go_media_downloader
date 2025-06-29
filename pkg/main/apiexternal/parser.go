package apiexternal

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Nzbwithprio is a struct containing information about an NZB found on the index
// It includes the parsed file name info, the NZB details, IDs, title,
// alternate titles, quality, list name, priority, reasons, and search flags.
type Nzbwithprio struct {
	Info                database.ParseInfo                 // The parsed file name information
	NZB                 Nzb                                // The NZB details
	WantedAlternates    []database.DbstaticTwoStringOneInt // Alternate wanted titles
	AdditionalReason    any                                // Any additional reason details
	AdditionalReasonStr string                             // Any additional reason details
	WantedTitle         string                             // The wanted title for this download
	Quality             string                             // The quality of this NZB
	Listname            string                             // The name of the list this NZB is from
	Reason              string                             // The reason for denying this NZB
	AdditionalReasonInt int64                              // Any additional reason details
	NzbmovieID          uint                               // The associated movie ID if this is a movie
	NzbepisodeID        uint                               // The associated episode ID if this is a TV episode
	Dbid                uint                               // The DBMovie or DBEpisode ID
	MinimumPriority     int                                // The minimum priority level
	DontSearch          bool                               // Whether to avoid searching for this
	DontUpgrade         bool                               // Whether to avoid upgrading this
	IDSearched          bool                               // Whether this NZB has been searched using the IMDB ID/THETVDB ID
}

// NZB represents an NZB found on the index.
type Nzb struct {
	// ID is the unique identifier for the NZB
	ID string `json:"id,omitempty"`

	// Title is the title of the content
	Title string `json:"title,omitempty"`

	// SourceEndpoint is the endpoint of the NZB source
	SourceEndpoint string `json:"source_endpoint"`

	// Season is the season number if this NZB is for a TV show
	Season string `json:"season,omitempty"`

	// Episode is the episode number if this NZB is for a TV show
	Episode string `json:"episode,omitempty"`

	// IMDBID is the IMDb ID if this NZB is for a movie
	IMDBID string `json:"imdb,omitempty"`

	// DownloadURL is the URL to download the NZB
	DownloadURL string `json:"download_url,omitempty"`

	// Size is the size of the NZB in bytes
	Size int64 `json:"size,omitempty"`

	// TVDBID is the TVDB ID if this NZB is for a TV show
	TVDBID int `json:"tvdbid,omitempty"`

	// IsTorrent indicates if this NZB is a torrent
	IsTorrent bool `json:"is_torrent,omitempty"`

	// Indexer is a pointer to the indexer config for this NZB
	Indexer *config.IndexersConfig

	// Quality is a pointer to the quality config for this NZB
	Quality *config.QualityConfig

	// TVTitle string `json:"tvtitle,omitempty"`
	// Rating  int    `json:"rating,omitempty"`
	// IMDBTitle string  `json:"imdbtitle,omitempty"`
	// IMDBYear  int     `json:"imdbyear,omitempty"`
	// IMDBScore float32 `json:"imdbscore,omitempty"`
	// CoverURL  string  `json:"coverurl,omitempty"`
	// Seeders     int    `json:"seeders,omitempty"`
	// Peers       int    `json:"peers,omitempty"`
	// InfoHash    string `json:"infohash,omitempty"`
	// Description string    `json:"description,omitempty"`
	// AirDate     time.Time `json:"air_date,omitempty"`
	// PubDate time.Time `json:"pub_date,omitempty"`
	// UsenetDate  time.Time `json:"usenet_date,omitempty"`
	// NumGrabs    int       `json:"num_grabs,omitempty"`
	// SourceAPIKey   string `json:"source_apikey"`

	// Category []string `json:"category,omitempty"`
	// Info     string   `json:"info,omitempty"`
	// Genre    string   `json:"genre,omitempty"`

	// Resolution string `json:"resolution,omitempty"`
	// Poster     string `json:"poster,omitempty"`
	// Group      string `json:"group,omitempty"`
}

// getregexcfg returns the regex configuration for the given quality config
// that matches the indexer name in the given Nzbwithprio entry. It first checks
// the Indexer list in the quality config, and falls back to the SettingsList
// global config if no match is found. Returns nil if no match is found.
func (s *Nzbwithprio) Getregexcfg(qual *config.QualityConfig) *config.RegexConfig {
	if s.NZB.Indexer != nil {
		indcfg := qual.QualityIndexerByQualityAndTemplateCheckRegex(s.NZB.Indexer)
		if indcfg != nil {
			return indcfg
		}
		_, ok := config.SettingsList[s.NZB.Indexer.Name]
		if ok {
			return qual.Indexer[0].CfgRegex
		}
		if s.NZB.Indexer.Getlistbyindexer() != nil {
			return qual.Indexer[0].CfgRegex
		}
	}
	return nil
}

// saveAttributes populates the fields of the NZB struct from
// the name/value pairs passed in. It handles translating the
// values to the appropriate types for the NZB struct fields.
func (n *Nzb) saveAttributes(name, value string) {
	switch name {
	case strtitle:
		n.Title = value
	case strlink, "url":
		n.DownloadURL = value
	case strguid:
		n.ID = value
	case "tvdbid":
		n.TVDBID = logger.StringToInt(value)
	case logger.StrImdb:
		n.IMDBID = logger.AddImdbPrefix(value)
	case "season":
		n.Season = value
	case "episode":
		n.Episode = value
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
// - Episode.
func (n *Nzb) setfield(field string, value []byte) {
	switch field {
	case strtitle:
		if n.Title != "" {
			return
		}
	case strlink, "url":
		if n.DownloadURL != "" {
			return
		}
	case strguid:
		if n.ID != "" {
			return
		}
	case strsize, "length":
		if n.Size != 0 {
			return
		}
	case logger.StrImdb:
		if n.IMDBID != "" {
			return
		}
	case "tvdbid":
		if n.TVDBID != 0 {
			return
		}
	case "season":
		if n.Season != "" {
			return
		}
	case "episode":
		if n.Episode != "" {
			return
		}
	default:
		return
	}
	n.saveAttributes(field, string(value)) // BytesToString
}

// setfieldstr sets the specified field of the Nzb struct to the provided value,
// but only if the field is not already set. The supported fields are:
// - Title
// - DownloadURL
// - ID
// - Size
// - IMDBID
// - TVDBID
// - Season
// - Episode.
func (n *Nzb) setfieldstr(field string, value string) {
	switch field {
	case strtitle:
		if n.Title != "" {
			return
		}
	case strlink, "url":
		if n.DownloadURL != "" {
			return
		}
	case strguid:
		if n.ID != "" {
			return
		}
	case strsize, "length":
		if n.Size != 0 {
			return
		}
	case logger.StrImdb:
		if n.IMDBID != "" {
			return
		}
	case "tvdbid":
		if n.TVDBID != 0 {
			return
		}
	case "season":
		if n.Season != "" {
			return
		}
	case "episode":
		if n.Episode != "" {
			return
		}
	default:
		return
	}
	n.saveAttributes(field, value)
}

// generateIdentifierStringFromInt generates a season/episode identifier string
// from the given season and episode integers. It pads each number with leading
// zeros to ensure a consistent format like "S01E02". This is intended to generate
// identifiers for public display/logging.
func generateIdentifierStringFromInt(season, episode *int) string {
	return ("S" + padNumberWithZero(season) + "E" + padNumberWithZero(episode))
}

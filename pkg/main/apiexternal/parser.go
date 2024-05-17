package apiexternal

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
)

// FileParser is a struct for parsing file names
// It contains fields for the file name string, parsed info, config pointer,
// whether to allow search by title, and if it is filled
type FileParser struct {
	Str              string                  // the file name string
	M                database.ParseInfo      // the parsed info
	Cfgp             *config.MediaTypeConfig // pointer to the media type config
	Allowsearchtitle bool                    // whether to allow search by title
	Filled           bool                    // whether the struct is filled
}

var (
	ParserPool = pool.NewPool(100, 0, func(b *FileParser) {}, func(b *FileParser) {
		b.Close()
	})
)

func (s *FileParser) Clear() {
	if s == nil || !s.Filled {
		return
	}
	*s = FileParser{}
}

func (s *FileParser) ClearArr() {
	if s == nil {
		return
	}
	//clear(s.M.Languages)
	s.M.Languages = nil
}

func (s *FileParser) Close() {
	if s == nil {
		return
	}
	s.ClearArr()
	*s = FileParser{}
}

// Nzbwithprio is a struct containing information about an NZB found on the index
// It includes the parsed file name info, the NZB details, IDs, title,
// alternate titles, quality, list name, priority, reasons, and search flags
type Nzbwithprio struct {
	Info             FileParser                         // The parsed file name information
	NZB              nzb                                // The NZB details
	NzbmovieID       uint                               // The associated movie ID if this is a movie
	NzbepisodeID     uint                               // The associated episode ID if this is a TV episode
	Dbid             uint                               // The DBMovie or DBEpisode ID
	WantedTitle      string                             // The wanted title for this download
	WantedAlternates []database.DbstaticTwoStringOneInt // Alternate wanted titles
	Quality          string                             // The quality of this NZB
	Listname         string                             // The name of the list this NZB is from
	MinimumPriority  int                                // The minimum priority level
	Reason           string                             // The reason for denying this NZB
	AdditionalReason string                             // Any additional reason details
	DontSearch       bool                               // Whether to avoid searching for this
	DontUpgrade      bool                               // Whether to avoid upgrading this
}

// NZB represents an NZB found on the index
type nzb struct {
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

// CheckDigitLetter returns true if the given rune is a digit or letter.
func CheckDigitLetter(runev rune) bool {
	if unicode.IsDigit(runev) || unicode.IsLetter(runev) {
		return false
	}
	return true
}

// Parsegroup parses the given name and group strings from s.Str,
// setting the corresponding fields on s.M when matches are found.
// It searches s.Str for each string in group, checks for valid
// surrounding characters, and extracts the matched substring if found.
func (s *FileParser) Parsegroup(name string, group []string) {
	for idx := range group {
		index := logger.IndexI(s.Str, group[idx])
		if index == -1 {
			continue
		}
		indexmax := index + len(group[idx])
		if s.Str[index:indexmax] == "" {
			continue
		}
		if indexmax < len(s.Str) && !CheckDigitLetter(rune(s.Str[indexmax : indexmax+1][0])) {
			continue
		}
		if index > 0 && !CheckDigitLetter(rune(s.Str[index-1 : index][0])) {
			continue
		}
		switch name {
		case "audio":
			s.M.Audio = s.Str[index:indexmax]
		case "codec":
			s.M.Codec = s.Str[index:indexmax]
		case "quality":
			s.M.Quality = s.Str[index:indexmax]
		case "resolution":
			s.M.Resolution = s.Str[index:indexmax]
		case "extended":
			s.M.Extended = true
		case "proper":
			s.M.Proper = true
		case "repack":
			s.M.Repack = true
		}
		break
	}
}

// ParsegroupEntry parses a single group string from s.Str, setting the
// corresponding field on s.M when a match is found. It checks for valid
// surrounding characters before extracting the matched substring.
func (s *FileParser) ParsegroupEntry(name string, group string) {
	index := logger.IndexI(s.Str, group)
	if index == -1 {
		return
	}

	indexmax := index + len(group)
	if indexmax < len(s.Str) && !CheckDigitLetter(rune(s.Str[indexmax : indexmax+1][0])) {
		return
	}
	if index > 0 && !CheckDigitLetter(rune(s.Str[index-1 : index][0])) {
		return
	}

	if s.Str[index:indexmax] == "" {
		return
	}
	switch name {
	case "audio":
		s.M.Audio = s.Str[index:indexmax]
	case "codec":
		s.M.Codec = s.Str[index:indexmax]
	case "quality":
		s.M.Quality = s.Str[index:indexmax]
	case "resolution":
		s.M.Resolution = s.Str[index:indexmax]
	case "extended":
		s.M.Extended = true
	case "proper":
		s.M.Proper = true
	case "repack":
		s.M.Repack = true
	}
}

// Close closes the Nzbwithprio by closing the Info field, setting the
// WantedAlternates field to nil if it has a capacity >= 1, and clearing
// the Nzbwithprio with the logger.
func (s *Nzbwithprio) Close() {
	if s == nil {
		return
	}
	s.ClearArr()
	*s = Nzbwithprio{}
}
func (s *Nzbwithprio) ClearArr() {
	if s == nil {
		return
	}
	//clear(s.WantedAlternates)
	s.WantedAlternates = nil
	s.Info.ClearArr()
}

// ChecknzbtitleB checks if the nzbtitle matches the movietitle and year.
// It compares the movietitle and nzbtitle directly, and also tries
// appending/removing the year, converting to slugs, etc.
// It is used to fuzzy match nzb titles to movie info during parsing.
func ChecknzbtitleB(movietitle string, movietitleslug string, nzbtitle string, allowpm1 bool, yeari int) bool {
	if movietitle == "" {
		return false
	}
	if movietitle == nzbtitle || strings.EqualFold(movietitle, nzbtitle) {
		return true
	}

	year := strconv.Itoa(yeari)
	var yearp, yearm string
	if yeari != 0 {
		checkstr1 := logger.JoinStrings(movietitle, " ", year)
		checkstr2 := logger.JoinStrings(movietitle, " (", year, ")")
		if allowpm1 {
			yearp = strconv.Itoa(yeari + 1)
			yearm = strconv.Itoa(yeari - 1)

			if checkstr1 == nzbtitle ||
				checkstr2 == nzbtitle ||
				strings.EqualFold(checkstr1, nzbtitle) ||
				strings.EqualFold(checkstr2, nzbtitle) {
				return true
			}

			checkstr1 = logger.JoinStrings(movietitle, " ", yearp)
			checkstr2 = logger.JoinStrings(movietitle, " (", yearp, ")")
			if checkstr1 == nzbtitle ||
				checkstr2 == nzbtitle ||
				strings.EqualFold(checkstr1, nzbtitle) ||
				strings.EqualFold(checkstr2, nzbtitle) {
				return true
			}

			checkstr1 = logger.JoinStrings(movietitle, " ", yearm)
			checkstr2 = logger.JoinStrings(movietitle, " (", yearm, ")")
			if checkstr1 == nzbtitle ||
				checkstr2 == nzbtitle ||
				strings.EqualFold(checkstr1, nzbtitle) ||
				strings.EqualFold(checkstr2, nzbtitle) {
				return true
			}
		} else if checkstr1 == nzbtitle ||
			checkstr2 == nzbtitle ||
			strings.EqualFold(checkstr1, nzbtitle) ||
			strings.EqualFold(checkstr2, nzbtitle) {
			return true
		}
	}

	if movietitleslug == "" {
		movietitleslug = logger.StringToSlug(movietitle)
	}
	slugged := logger.StringToSlug(nzbtitle)
	if slugged == "" {
		return false
	}
	if movietitleslug == slugged {
		return true
	}

	movietitleslug = logger.StringRemoveAllRunes(movietitleslug, '-')
	slugged = logger.StringRemoveAllRunes(slugged, '-')
	if movietitleslug == slugged {
		return true
	}

	if yeari != 0 {
		if movietitleslug+year == slugged {
			return true
		}

		if allowpm1 {
			if movietitleslug+yearp == nzbtitle ||
				movietitleslug+yearm == nzbtitle {
				return true
			}
		}
	}

	return false
}

// GenerateIdentifierString generates an identifier string for a movie or episode
// in the format "S{season}E{episode}", where {season} and {episode} are the
// season and episode numbers formatted as strings.
func GenerateIdentifierString(m *database.ParseInfo) string {
	return logger.JoinStrings("S", m.SeasonStr, "E", m.EpisodeStr)
}

// GenerateIdentifierStringFromInt generates a season/episode identifier string
// from the given season and episode integers. It pads each number with leading
// zeros to ensure a consistent format like "S01E02". This is intended to generate
// identifiers for public display/logging.
func GenerateIdentifierStringFromInt(season int, episode int) string {
	return logger.JoinStrings("S", padNumberWithZero(season), "E", padNumberWithZero(episode))
}

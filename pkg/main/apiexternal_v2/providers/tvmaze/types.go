package tvmaze

import (
	"html"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// TVMaze Internal Types - Used for JSON unmarshaling
//

type tvmazeSearchResponse []tvmazeSearchResult

type tvmazeSearchResult struct {
	Score float64    `json:"score"`
	Show  tvmazeShow `json:"show"`
}

type tvmazeShow struct {
	ID             int               `json:"id"`
	URL            string            `json:"url"`
	Name           string            `json:"name"`
	Type           string            `json:"type"`
	Language       string            `json:"language"`
	Genres         []string          `json:"genres"`
	Status         string            `json:"status"`
	Runtime        int               `json:"runtime"`
	AverageRuntime int               `json:"averageRuntime"`
	Premiered      string            `json:"premiered"`
	Ended          string            `json:"ended"`
	OfficialSite   string            `json:"officialSite"`
	Schedule       tvmazeSchedule    `json:"schedule"`
	Rating         tvmazeRating      `json:"rating"`
	Weight         int               `json:"weight"`
	Network        *tvmazeNetwork    `json:"network"`
	WebChannel     *tvmazeWebChannel `json:"webChannel"`
	DVDCountry     *tvmazeCountry    `json:"dvdCountry"`
	Externals      tvmazeExternals   `json:"externals"`
	Image          *tvmazeImage      `json:"image"`
	Summary        string            `json:"summary"`
	Updated        int64             `json:"updated"`
	Links          tvmazeLinks       `json:"_links"`
}

type tvmazeSchedule struct {
	Time string   `json:"time"`
	Days []string `json:"days"`
}

type tvmazeRating struct {
	Average float64 `json:"average"`
}

type tvmazeNetwork struct {
	ID           int           `json:"id"`
	Name         string        `json:"name"`
	Country      tvmazeCountry `json:"country"`
	OfficialSite string        `json:"officialSite"`
}

type tvmazeWebChannel struct {
	ID           int            `json:"id"`
	Name         string         `json:"name"`
	Country      *tvmazeCountry `json:"country"`
	OfficialSite string         `json:"officialSite"`
}

type tvmazeCountry struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Timezone string `json:"timezone"`
}

type tvmazeExternals struct {
	TVRage int    `json:"tvrage"`
	TVDB   int    `json:"thetvdb"`
	IMDB   string `json:"imdb"`
}

type tvmazeImage struct {
	Medium   string `json:"medium"`
	Original string `json:"original"`
}

type tvmazeLinks struct {
	Self            tvmazeLink  `json:"self"`
	PreviousEpisode *tvmazeLink `json:"previousepisode,omitempty"`
	NextEpisode     *tvmazeLink `json:"nextepisode,omitempty"`
}

type tvmazeLink struct {
	Href string `json:"href"`
}

//
// Conversion Functions
//

func convertSearchResults(
	tvmazeResults []tvmazeSearchResult,
	provider string,
) []apiexternal_v2.SeriesSearchResult {
	results := make([]apiexternal_v2.SeriesSearchResult, 0, len(tvmazeResults))

	for i := range tvmazeResults {
		results = append(results, apiexternal_v2.SeriesSearchResult{
			ID:           tvmazeResults[i].Show.ID,
			Name:         tvmazeResults[i].Show.Name,
			FirstAirDate: parseTVMazeDate(tvmazeResults[i].Show.Premiered),
			Overview:     stripHTML(tvmazeResults[i].Show.Summary),
			PosterPath:   getImageURL(tvmazeResults[i].Show.Image),
			VoteAverage:  tvmazeResults[i].Show.Rating.Average,
			ProviderName: provider,
			TVDbID:       tvmazeResults[i].Show.Externals.TVDB,
			IMDbID:       tvmazeResults[i].Show.Externals.IMDB,
		})
	}

	return results
}

func convertShowToDetails(show *tvmazeShow) *apiexternal_v2.SeriesDetails {
	// Convert genres
	genres := make([]apiexternal_v2.Genre, len(show.Genres))
	for i, g := range show.Genres {
		genres[i] = apiexternal_v2.Genre{
			ID:   i + 1,
			Name: g,
		}
	}

	// Convert network
	networks := []apiexternal_v2.Network{}
	if show.Network != nil {
		networks = append(networks, apiexternal_v2.Network{
			ID:            show.Network.ID,
			Name:          show.Network.Name,
			OriginCountry: show.Network.Country.Code,
		})
	} else if show.WebChannel != nil {
		countryCode := ""
		if show.WebChannel.Country != nil {
			countryCode = show.WebChannel.Country.Code
		}

		networks = append(networks, apiexternal_v2.Network{
			ID:            show.WebChannel.ID,
			Name:          show.WebChannel.Name,
			OriginCountry: countryCode,
		})
	}

	return &apiexternal_v2.SeriesDetails{
		ID:           show.ID,
		TVDbID:       show.Externals.TVDB,
		IMDbID:       show.Externals.IMDB,
		Name:         show.Name,
		OriginalName: show.Name,
		Overview:     stripHTML(show.Summary),
		FirstAirDate: parseTVMazeDate(show.Premiered),
		LastAirDate:  parseTVMazeDate(show.Ended),
		Status:       show.Status,
		VoteAverage:  show.Rating.Average,
		PosterPath:   getImageURL(show.Image),
		Genres:       genres,
		Networks:     networks,
		Homepage:     show.OfficialSite,
		ProviderName: "tvmaze",
	}
}

//
// Helper Functions
//

func parseTVMazeDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	layouts := []string{
		"2006-01-02",
		time.RFC3339,
	}

	for i := range layouts {
		if t, err := time.Parse(layouts[i], dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

func stripHTML(s string) string {
	if s == "" {
		return ""
	}

	// Simple HTML tag removal
	var result strings.Builder

	inTag := false
	for _, r := range s {
		if r == '<' {
			inTag = true
			continue
		}

		if r == '>' {
			inTag = false
			continue
		}

		if !inTag {
			result.WriteRune(r)
		}
	}

	return html.UnescapeString(result.String())
}

func getImageURL(image *tvmazeImage) string {
	if image == nil {
		return ""
	}

	if image.Original != "" {
		return image.Original
	}

	return image.Medium
}

// func tvmazeIDToString(id int) string {
// 	if id == 0 {
// 		return ""
// 	}
// 	return strconv.Itoa(id)
// }

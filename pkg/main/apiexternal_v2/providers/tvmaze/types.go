package tvmaze

import (
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

type tvmazeSeasonResponse []tvmazeSeason

type tvmazeSeason struct {
	ID           int               `json:"id"`
	URL          string            `json:"url"`
	Number       int               `json:"number"`
	Name         string            `json:"name"`
	EpisodeOrder int               `json:"episodeOrder"`
	PremiereDate string            `json:"premiereDate"`
	EndDate      string            `json:"endDate"`
	Network      *tvmazeNetwork    `json:"network"`
	WebChannel   *tvmazeWebChannel `json:"webChannel"`
	Image        *tvmazeImage      `json:"image"`
	Summary      string            `json:"summary"`
	Links        tvmazeLinks       `json:"_links"`
}

type tvmazeEpisodeResponse []tvmazeEpisode

type tvmazeEpisode struct {
	ID       int          `json:"id"`
	URL      string       `json:"url"`
	Name     string       `json:"name"`
	Season   int          `json:"season"`
	Number   int          `json:"number"`
	Type     string       `json:"type"`
	Airdate  string       `json:"airdate"`
	Airtime  string       `json:"airtime"`
	Airstamp string       `json:"airstamp"`
	Runtime  int          `json:"runtime"`
	Rating   tvmazeRating `json:"rating"`
	Image    *tvmazeImage `json:"image"`
	Summary  string       `json:"summary"`
	Links    tvmazeLinks  `json:"_links"`
}

type tvmazeCastResponse []tvmazeCastMember

type tvmazeCastMember struct {
	Person    tvmazePerson    `json:"person"`
	Character tvmazeCharacter `json:"character"`
	Self      bool            `json:"self"`
	Voice     bool            `json:"voice"`
}

type tvmazePerson struct {
	ID       int            `json:"id"`
	URL      string         `json:"url"`
	Name     string         `json:"name"`
	Country  *tvmazeCountry `json:"country"`
	Birthday string         `json:"birthday"`
	Deathday string         `json:"deathday"`
	Gender   string         `json:"gender"`
	Image    *tvmazeImage   `json:"image"`
	Updated  int64          `json:"updated"`
	Links    tvmazeLinks    `json:"_links"`
}

type tvmazeCharacter struct {
	ID    int          `json:"id"`
	URL   string       `json:"url"`
	Name  string       `json:"name"`
	Image *tvmazeImage `json:"image"`
	Links tvmazeLinks  `json:"_links"`
}

type tvmazeCrewResponse []tvmazeCrewMember

type tvmazeCrewMember struct {
	Type   string       `json:"type"`
	Person tvmazePerson `json:"person"`
}

//
// Conversion Functions
//

func convertSearchResults(
	tvmazeResults []tvmazeSearchResult,
	provider string,
) []apiexternal_v2.SeriesSearchResult {
	results := make([]apiexternal_v2.SeriesSearchResult, 0, len(tvmazeResults))

	for _, r := range tvmazeResults {
		results = append(results, apiexternal_v2.SeriesSearchResult{
			ID:           r.Show.ID,
			Name:         r.Show.Name,
			FirstAirDate: parseTVMazeDate(r.Show.Premiered),
			Overview:     stripHTML(r.Show.Summary),
			PosterPath:   getImageURL(r.Show.Image),
			VoteAverage:  r.Show.Rating.Average,
			ProviderName: provider,
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

func convertSeasonToDetails(season *tvmazeSeason) *apiexternal_v2.Season {
	return &apiexternal_v2.Season{
		ID:           season.ID,
		SeasonNumber: season.Number,
		Name:         season.Name,
		Overview:     stripHTML(season.Summary),
		AirDate:      parseTVMazeDate(season.PremiereDate),
		PosterPath:   getImageURL(season.Image),
		EpisodeCount: season.EpisodeOrder,
	}
}

func convertEpisodeToDetails(episode *tvmazeEpisode) *apiexternal_v2.Episode {
	return &apiexternal_v2.Episode{
		ID:            episode.ID,
		EpisodeNumber: episode.Number,
		SeasonNumber:  episode.Season,
		Name:          episode.Name,
		Overview:      stripHTML(episode.Summary),
		AirDate:       parseTVMazeDate(episode.Airdate),
		Runtime:       episode.Runtime,
		StillPath:     getImageURL(episode.Image),
		VoteAverage:   episode.Rating.Average,
	}
}

func convertCastToCredits(cast []tvmazeCastMember) *apiexternal_v2.Credits {
	castMembers := make([]apiexternal_v2.CastMember, len(cast))
	for i, c := range cast {
		castMembers[i] = apiexternal_v2.CastMember{
			ID:          c.Person.ID,
			Name:        c.Person.Name,
			Character:   c.Character.Name,
			Order:       i,
			ProfilePath: getImageURL(c.Person.Image),
		}
	}

	return &apiexternal_v2.Credits{
		Cast: castMembers,
		Crew: []apiexternal_v2.CrewMember{}, // Crew handled separately
	}
}

func convertCrewToCredits(crew []tvmazeCrewMember) []apiexternal_v2.CrewMember {
	crewMembers := make([]apiexternal_v2.CrewMember, len(crew))
	for i, c := range crew {
		crewMembers[i] = apiexternal_v2.CrewMember{
			ID:          c.Person.ID,
			Name:        c.Person.Name,
			Job:         c.Type,
			Department:  c.Type,
			ProfilePath: getImageURL(c.Person.Image),
		}
	}

	return crewMembers
}

func convertExternalsToExternalIDs(
	externals tvmazeExternals,
	tvmazeID int,
) *apiexternal_v2.ExternalIDs {
	return &apiexternal_v2.ExternalIDs{
		IMDbID: externals.IMDB,
		TVDbID: externals.TVDB,
	}
}

func convertImagesToCollection(image *tvmazeImage) *apiexternal_v2.ImageCollection {
	images := &apiexternal_v2.ImageCollection{
		Posters:   []apiexternal_v2.Image{},
		Backdrops: []apiexternal_v2.Image{},
		Logos:     []apiexternal_v2.Image{},
	}

	if image != nil {
		img := apiexternal_v2.Image{
			FilePath: image.Original,
		}

		images.Posters = append(images.Posters, img)
	}

	return images
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

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
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
	result := ""

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
			result += string(r)
		}
	}

	return result
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

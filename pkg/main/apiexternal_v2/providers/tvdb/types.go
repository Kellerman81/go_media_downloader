package tvdb

import (
	"strconv"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// TVDB v2 API Types - Used for JSON unmarshaling (actual v2 responses)
//

// tvdbV2SeriesResponse represents the response from /series/:id endpoint.
type tvdbV2SeriesResponse struct {
	Data tvdbV2Series `json:"data"`
}

// tvdbV2Series represents a TV series in TVDB v2 API format.
type tvdbV2Series struct {
	ID              int      `json:"id"`
	SeriesID        string   `json:"seriesId"`
	SeriesName      string   `json:"seriesName"`
	Aliases         []string `json:"aliases"` // v2 uses string array, not object array
	Season          string   `json:"season"`
	Poster          string   `json:"poster"`
	Banner          string   `json:"banner"`
	Fanart          string   `json:"fanart"`
	Status          string   `json:"status"` // v2 uses string, not object
	FirstAired      string   `json:"firstAired"`
	Network         string   `json:"network"`
	NetworkID       string   `json:"networkId"`
	Runtime         string   `json:"runtime"`
	Language        string   `json:"language"`
	Genre           []string `json:"genre"`
	Overview        string   `json:"overview"`
	LastUpdated     int64    `json:"lastUpdated"`
	AirsDayOfWeek   string   `json:"airsDayOfWeek"`
	AirsTime        string   `json:"airsTime"`
	Rating          string   `json:"rating"`
	ImdbID          string   `json:"imdbId"`
	Zap2itID        string   `json:"zap2itId"`
	Added           string   `json:"added"`
	AddedBy         int      `json:"addedBy"`
	SiteRating      float64  `json:"siteRating"`
	SiteRatingCount int      `json:"siteRatingCount"`
	Slug            string   `json:"slug"`
}

// tvdbV2EpisodesResponse represents the response from /series/:id/episodes endpoint.
type tvdbV2EpisodesResponse struct {
	Links struct {
		First int  `json:"first"`
		Last  int  `json:"last"`
		Next  *int `json:"next"`
		Prev  *int `json:"prev"`
	} `json:"links"`
	Data []tvdbV2Episode `json:"data"`
}

// tvdbV2Episode represents an episode in TVDB v2 API format.
type tvdbV2Episode struct {
	ID                 int               `json:"id"`
	AiredSeason        int               `json:"airedSeason"`
	AiredSeasonID      int               `json:"airedSeasonID"`
	AiredEpisodeNumber int               `json:"airedEpisodeNumber"`
	EpisodeName        string            `json:"episodeName"`
	FirstAired         string            `json:"firstAired"`
	GuestStars         []string          `json:"guestStars"`
	Directors          []string          `json:"directors"`
	Writers            []string          `json:"writers"`
	Overview           string            `json:"overview"`
	Language           map[string]string `json:"language"`
	ProductionCode     string            `json:"productionCode"`
	ShowURL            string            `json:"showUrl"`
	LastUpdated        int64             `json:"lastUpdated"`
	DvdDiscID          string            `json:"dvdDiscid"`
	DvdSeason          int               `json:"dvdSeason"`
	DvdEpisodeNumber   int               `json:"dvdEpisodeNumber"`
	DvdChapter         any               `json:"dvdChapter"`
	AbsoluteNumber     int               `json:"absoluteNumber"`
	Filename           string            `json:"filename"`
	SeriesID           int               `json:"seriesId"`
	LastUpdatedBy      int               `json:"lastUpdatedBy"`
	AirsAfterSeason    any               `json:"airsAfterSeason"`
	AirsBeforeSeason   any               `json:"airsBeforeSeason"`
	AirsBeforeEpisode  any               `json:"airsBeforeEpisode"`
	ImdbID             string            `json:"imdbId"`
	ContentRating      string            `json:"contentRating"`
	ThumbAuthor        int               `json:"thumbAuthor"`
	ThumbAdded         string            `json:"thumbAdded"`
	ThumbWidth         string            `json:"thumbWidth"`
	ThumbHeight        string            `json:"thumbHeight"`
	SiteRating         float64           `json:"siteRating"`
	SiteRatingCount    int               `json:"siteRatingCount"`
	IsMovie            int               `json:"isMovie"`
}

// convertV2SeriesToDetails converts TVDB v2 series to common SeriesDetails format.
func convertV2SeriesToDetails(series *tvdbV2Series) *apiexternal_v2.SeriesDetails {
	// Convert genres from string array to Genre structs
	genres := make([]apiexternal_v2.Genre, len(series.Genre))
	for i, g := range series.Genre {
		genres[i] = apiexternal_v2.Genre{
			ID:   i + 1, // v2 doesn't provide genre IDs, use index
			Name: g,
		}
	}

	// Convert networks
	networks := []apiexternal_v2.Network{}
	if series.Network != "" {
		networkID, _ := strconv.Atoi(series.NetworkID)

		networks = append(networks, apiexternal_v2.Network{
			ID:   networkID,
			Name: series.Network,
		})
	}

	return &apiexternal_v2.SeriesDetails{
		ID:           series.ID,
		TVDbID:       series.ID,
		IMDbID:       series.ImdbID,
		Name:         series.SeriesName,
		OriginalName: series.SeriesName,
		Overview:     series.Overview,
		FirstAirDate: parseTVDBDate(series.FirstAired),
		Status:       series.Status,
		VoteAverage:  series.SiteRating,
		PosterPath:   series.Poster,
		BackdropPath: series.Fanart,
		Genres:       genres,
		Networks:     networks,
		ProviderName: "tvdb",
	}
}

// convertV2EpisodeToDetails converts TVDB v2 episode to common Episode format.
func convertV2EpisodeToDetails(episode *tvdbV2Episode) *apiexternal_v2.Episode {
	return &apiexternal_v2.Episode{
		ID:            episode.ID,
		EpisodeNumber: episode.AiredEpisodeNumber,
		SeasonNumber:  episode.AiredSeason,
		Name:          episode.EpisodeName,
		Overview:      episode.Overview,
		AirDate:       parseTVDBDate(episode.FirstAired),
		VoteAverage:   episode.SiteRating,
		StillPath:     episode.Filename,
	}
}

//
// TVDB Internal Types - Used for JSON unmarshaling (TVDB API v4)
//

// type tvdbLoginResponse struct {
// 	Status string                 `json:"status"`
// 	Data   map[string]any `json:"data"`
// 	Token  string                 `json:"token"`
// }

type tvdbSearchResponse struct {
	Status string             `json:"status"`
	Data   []tvdbSearchResult `json:"data"`
}

type tvdbSearchResult struct {
	ObjectID        string   `json:"objectID"`
	Aliases         []string `json:"aliases"`
	Country         string   `json:"country"`
	ID              string   `json:"id"`
	ImageURL        string   `json:"image_url"`
	Name            string   `json:"name"`
	FirstAirTime    string   `json:"first_air_time"`
	Overview        string   `json:"overview"`
	PrimaryLanguage string   `json:"primary_language"`
	PrimaryType     string   `json:"primary_type"`
	Status          string   `json:"status"`
	Type            string   `json:"type"`
	TVDBid          string   `json:"tvdb_id"`
	Year            string   `json:"year"`
}

// type tvdbSeriesResponse struct {
// 	Status string     `json:"status"`
// 	Data   tvdbSeries `json:"data"`
// }

// type tvdbSeries struct {
// 	ID                   int                 `json:"id"`
// 	Name                 string              `json:"name"`
// 	Slug                 string              `json:"slug"`
// 	Image                string              `json:"image"`
// 	NameTranslations     []string            `json:"nameTranslations"`
// 	OverviewTranslations []string            `json:"overviewTranslations"`
// 	Aliases              []tvdbAlias         `json:"aliases"`
// 	FirstAired           string              `json:"firstAired"`
// 	LastAired            string              `json:"lastAired"`
// 	NextAired            string              `json:"nextAired"`
// 	Score                float64             `json:"score"`
// 	Status               tvdbStatus          `json:"status"`
// 	OriginalCountry      string              `json:"originalCountry"`
// 	OriginalLanguage     string              `json:"originalLanguage"`
// 	DefaultSeasonType    int                 `json:"defaultSeasonType"`
// 	IsOrderRandomized    bool                `json:"isOrderRandomized"`
// 	LastUpdated          string              `json:"lastUpdated"`
// 	AverageRuntime       int                 `json:"averageRuntime"`
// 	Episodes             []tvdbEpisode       `json:"episodes"`
// 	Overview             string              `json:"overview"`
// 	Year                 string              `json:"year"`
// 	Artworks             []tvdbArtwork       `json:"artworks"`
// 	Companies            []tvdbCompany       `json:"companies"`
// 	OriginalNetwork      tvdbNetwork         `json:"originalNetwork"`
// 	LatestNetwork        tvdbNetwork         `json:"latestNetwork"`
// 	Genres               []tvdbGenre         `json:"genres"`
// 	Translations         tvdbTranslations    `json:"translations"`
// 	RemoteIDs            []tvdbRemoteID      `json:"remoteIds"`
// 	Characters           []tvdbCharacter     `json:"characters"`
// 	Lists                []tvdbList          `json:"lists"`
// 	ContentRatings       []tvdbContentRating `json:"contentRatings"`
// 	Seasons              []tvdbSeasonType    `json:"seasons"`
// 	Tags                 []tvdbTag           `json:"tags"`
// }

// type tvdbAlias struct {
// 	Language string `json:"language"`
// 	Name     string `json:"name"`
// }

// type tvdbStatus struct {
// 	ID          int    `json:"id"`
// 	Name        string `json:"name"`
// 	RecordType  string `json:"recordType"`
// 	KeepUpdated bool   `json:"keepUpdated"`
// }

// type tvdbEpisode struct {
// 	ID                   int      `json:"id"`
// 	SeriesID             int      `json:"seriesId"`
// 	Name                 string   `json:"name"`
// 	Aired                string   `json:"aired"`
// 	Runtime              int      `json:"runtime"`
// 	NameTranslations     []string `json:"nameTranslations"`
// 	Overview             string   `json:"overview"`
// 	OverviewTranslations []string `json:"overviewTranslations"`
// 	Image                string   `json:"image"`
// 	ImageType            int      `json:"imageType"`
// 	IsMovie              int      `json:"isMovie"`
// 	Seasons              []int    `json:"seasons"`
// 	Number               int      `json:"number"`
// 	SeasonNumber         int      `json:"seasonNumber"`
// 	LastUpdated          string   `json:"lastUpdated"`
// 	FinaleType           string   `json:"finaleType"`
// 	Year                 string   `json:"year"`
// }

// type tvdbEpisodesResponse struct {
// 	Status string           `json:"status"`
// 	Data   tvdbEpisodesData `json:"data"`
// }

// type tvdbEpisodesData struct {
// 	Series   tvdbSeries    `json:"series"`
// 	Episodes []tvdbEpisode `json:"episodes"`
// }

// type tvdbArtwork struct {
// 	ID           int     `json:"id"`
// 	Image        string  `json:"image"`
// 	Thumbnail    string  `json:"thumbnail"`
// 	Language     string  `json:"language"`
// 	Type         int     `json:"type"`
// 	Score        float64 `json:"score"`
// 	Width        int     `json:"width"`
// 	Height       int     `json:"height"`
// 	IncludesText bool    `json:"includesText"`
// }

// type tvdbCompany struct {
// 	ID                   int             `json:"id"`
// 	Name                 string          `json:"name"`
// 	Slug                 string          `json:"slug"`
// 	Country              string          `json:"country"`
// 	CompanyType          tvdbCompanyType `json:"companyType"`
// 	NameTranslations     []string        `json:"nameTranslations"`
// 	OverviewTranslations []string        `json:"overviewTranslations"`
// }

// type tvdbCompanyType struct {
// 	CompanyTypeID   int    `json:"companyTypeId"`
// 	CompanyTypeName string `json:"companyTypeName"`
// }

// type tvdbNetwork struct {
// 	ID      int    `json:"id"`
// 	Name    string `json:"name"`
// 	Slug    string `json:"slug"`
// 	Country string `json:"country"`
// }

// type tvdbGenre struct {
// 	ID   int    `json:"id"`
// 	Name string `json:"name"`
// 	Slug string `json:"slug"`
// }

// type tvdbTranslations struct {
// 	NameTranslations     []tvdbTranslation `json:"nameTranslations"`
// 	OverviewTranslations []tvdbTranslation `json:"overviewTranslations"`
// 	Alias                []string          `json:"alias"`
// }

// type tvdbTranslation struct {
// 	Language  string `json:"language"`
// 	Name      string `json:"name"`
// 	Overview  string `json:"overview"`
// 	TagLine   string `json:"tagLine"`
// 	IsAlias   bool   `json:"isAlias"`
// 	IsPrimary bool   `json:"isPrimary"`
// }

// type tvdbRemoteID struct {
// 	ID         int    `json:"id"`
// 	Type       int    `json:"type"`
// 	SourceName string `json:"sourceName"`
// 	RemoteID   string `json:"remoteId"`
// }

// type tvdbCharacter struct {
// 	ID                   int      `json:"id"`
// 	Name                 string   `json:"name"`
// 	PeopleID             int      `json:"peopleId"`
// 	SeriesID             int      `json:"seriesId"`
// 	Series               string   `json:"series"`
// 	Movie                string   `json:"movie"`
// 	MovieID              int      `json:"movieId"`
// 	EpisodeID            int      `json:"episodeId"`
// 	Type                 int      `json:"type"`
// 	Image                string   `json:"image"`
// 	Sort                 int      `json:"sort"`
// 	IsFeatured           bool     `json:"isFeatured"`
// 	URL                  string   `json:"url"`
// 	NameTranslations     []string `json:"nameTranslations"`
// 	OverviewTranslations []string `json:"overviewTranslations"`
// 	Aliases              []string `json:"aliases"`
// 	PeopleName           string   `json:"peopleName"`
// 	PersonName           string   `json:"personName"`
// 	TagOptions           []string `json:"tagOptions"`
// 	PersonImgURL         string   `json:"personImgURL"`
// }

// type tvdbList struct {
// 	ID                   int      `json:"id"`
// 	Name                 string   `json:"name"`
// 	Overview             string   `json:"overview"`
// 	URL                  string   `json:"url"`
// 	IsOfficial           bool     `json:"isOfficial"`
// 	NameTranslations     []string `json:"nameTranslations"`
// 	OverviewTranslations []string `json:"overviewTranslations"`
// }

// type tvdbContentRating struct {
// 	ID          int    `json:"id"`
// 	Name        string `json:"name"`
// 	Country     string `json:"country"`
// 	Description string `json:"description"`
// 	ContentType string `json:"contentType"`
// 	Order       int    `json:"order"`
// 	FullName    string `json:"fullname"`
// }

// type tvdbSeasonType struct {
// 	ID                   int                    `json:"id"`
// 	SeriesID             int                    `json:"seriesId"`
// 	Type                 tvdbSeasonTypeInfo     `json:"type"`
// 	Number               int                    `json:"number"`
// 	NameTranslations     []string               `json:"nameTranslations"`
// 	OverviewTranslations []string               `json:"overviewTranslations"`
// 	Companies            map[string]any `json:"companies"`
// 	Image                string                 `json:"image"`
// 	ImageType            int                    `json:"imageType"`
// 	LastUpdated          string                 `json:"lastUpdated"`
// 	Name                 string                 `json:"name"`
// }

// type tvdbSeasonTypeInfo struct {
// 	ID            int    `json:"id"`
// 	Name          string `json:"name"`
// 	Type          string `json:"type"`
// 	AlternateName string `json:"alternateName"`
// }

// type tvdbTag struct {
// 	ID       int    `json:"id"`
// 	Tag      int    `json:"tag"`
// 	TagName  string `json:"tagName"`
// 	Name     string `json:"name"`
// 	HelpText string `json:"helpText"`
// }

//
// Conversion Functions
//

func convertSearchResults(
	tvdbResults []tvdbSearchResult,
	provider string,
) []apiexternal_v2.SeriesSearchResult {
	results := make([]apiexternal_v2.SeriesSearchResult, 0, len(tvdbResults))

	for _, r := range tvdbResults {
		id, _ := strconv.Atoi(r.TVDBid)

		results = append(results, apiexternal_v2.SeriesSearchResult{
			ID:           id,
			Name:         r.Name,
			FirstAirDate: parseTVDBDate(r.FirstAirTime),
			PosterPath:   r.ImageURL,
			Overview:     r.Overview,
			ProviderName: provider,
		})
	}

	return results
}

// func convertSeriesToDetails(series *tvdbSeries) *apiexternal_v2.SeriesDetails {
// 	// Convert genres
// 	genres := make([]apiexternal_v2.Genre, len(series.Genres))
// 	for i, g := range series.Genres {
// 		genres[i] = apiexternal_v2.Genre{
// 			ID:   g.ID,
// 			Name: g.Name,
// 		}
// 	}

// 	// Convert networks
// 	networks := []apiexternal_v2.Network{}
// 	if series.OriginalNetwork.ID > 0 {
// 		networks = append(networks, apiexternal_v2.Network{
// 			ID:            series.OriginalNetwork.ID,
// 			Name:          series.OriginalNetwork.Name,
// 			OriginCountry: series.OriginalNetwork.Country,
// 		})
// 	}

// 	// Convert seasons
// 	seasons := make([]apiexternal_v2.Season, len(series.Seasons))
// 	for i, s := range series.Seasons {
// 		seasons[i] = apiexternal_v2.Season{
// 			ID:           s.ID,
// 			SeasonNumber: s.Number,
// 			Name:         s.Name,
// 			PosterPath:   s.Image,
// 		}
// 	}

// 	// Get external IDs
// 	imdbID := ""
// 	for _, rid := range series.RemoteIDs {
// 		if rid.SourceName == "IMDB" {
// 			imdbID = rid.RemoteID
// 			break
// 		}
// 	}

// 	return &apiexternal_v2.SeriesDetails{
// 		ID:               series.ID,
// 		TVDbID:           series.ID,
// 		IMDbID:           imdbID,
// 		Name:             series.Name,
// 		OriginalName:     series.Name,
// 		Overview:         series.Overview,
// 		FirstAirDate:     parseTVDBDate(series.FirstAired),
// 		LastAirDate:      parseTVDBDate(series.LastAired),
// 		Status:           series.Status.Name,
// 		NumberOfSeasons:  len(series.Seasons),
// 		NumberOfEpisodes: len(series.Episodes),
// 		VoteAverage:      series.Score,
// 		PosterPath:       series.Image,
// 		Genres:           genres,
// 		Networks:         networks,
// 		Seasons:          seasons,
// 		ProviderName:     "tvdb",
// 	}
// }

// func convertEpisodeToDetails(episode *tvdbEpisode) *apiexternal_v2.Episode {
// 	return &apiexternal_v2.Episode{
// 		ID:            episode.ID,
// 		EpisodeNumber: episode.Number,
// 		SeasonNumber:  episode.SeasonNumber,
// 		Name:          episode.Name,
// 		Overview:      episode.Overview,
// 		AirDate:       parseTVDBDate(episode.Aired),
// 		Runtime:       episode.Runtime,
// 		StillPath:     episode.Image,
// 	}
// }

// func convertAliasesToAlternativeTitles(aliases []tvdbAlias) []apiexternal_v2.AlternativeTitle {
// 	titles := make([]apiexternal_v2.AlternativeTitle, len(aliases))
// 	for i, alias := range aliases {
// 		titles[i] = apiexternal_v2.AlternativeTitle{
// 			Title:     alias.Name,
// 			ISO3166_1: alias.Language,
// 		}
// 	}
// 	return titles
// }

// func convertCharactersToCredits(characters []tvdbCharacter) *apiexternal_v2.Credits {
// 	cast := make([]apiexternal_v2.CastMember, 0, len(characters))

// 	for _, char := range characters {
// 		if char.PeopleName != "" {
// 			cast = append(cast, apiexternal_v2.CastMember{
// 				ID:          char.ID,
// 				Name:        char.PeopleName,
// 				Character:   char.Name,
// 				Order:       char.Sort,
// 				ProfilePath: char.PersonImgURL,
// 			})
// 		}
// 	}

// 	return &apiexternal_v2.Credits{
// 		Cast: cast,
// 		Crew: []apiexternal_v2.CrewMember{}, // TVDB doesn't provide crew separately
// 	}
// }

// func convertArtworkToImageCollection(artworks []tvdbArtwork) *apiexternal_v2.ImageCollection {
// 	var posters, backdrops, logos []apiexternal_v2.Image

// 	for _, art := range artworks {
// 		img := apiexternal_v2.Image{
// 			FilePath:    art.Image,
// 			Width:       art.Width,
// 			Height:      art.Height,
// 			VoteAverage: art.Score,
// 			ISO639_1:    art.Language,
// 		}

// 		// Type: 1=banner, 2=poster, 3=fanart, 6=background, etc.
// 		switch art.Type {
// 		case 2: // Poster
// 			posters = append(posters, img)
// 		case 3, 6: // Fanart/Background
// 			backdrops = append(backdrops, img)
// 		case 1: // Banner (use as logo)
// 			logos = append(logos, img)
// 		}
// 	}

// 	return &apiexternal_v2.ImageCollection{
// 		Backdrops: backdrops,
// 		Posters:   posters,
// 		Logos:     logos,
// 	}
// }

// func convertRemoteIDsToExternalIDs(remoteIDs []tvdbRemoteID, tvdbID int) *apiexternal_v2.ExternalIDs {
// 	externalIDs := &apiexternal_v2.ExternalIDs{
// 		TVDbID: tvdbID,
// 	}

// 	for _, rid := range remoteIDs {
// 		switch rid.SourceName {
// 		case "IMDB":
// 			externalIDs.IMDbID = rid.RemoteID
// 		case "TheMovieDB.com":
// 			if id, err := strconv.Atoi(rid.RemoteID); err == nil {
// 				externalIDs.TMDbID = id
// 			}
// 		case "Facebook":
// 			externalIDs.FacebookID = rid.RemoteID
// 		case "Instagram":
// 			externalIDs.InstagramID = rid.RemoteID
// 		case "Twitter":
// 			externalIDs.TwitterID = rid.RemoteID
// 		}
// 	}

// 	return externalIDs
// }

//
// Helper Functions
//

func parseTVDBDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	layouts := []string{
		"2006-01-02",
		time.RFC3339,
		"2006-01-02 15:04:05",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

package omdb

import (
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// OMDB Internal Types - Used for JSON unmarshaling
//

type omdbSearchResponse struct {
	Search       []omdbSearchResult `json:"Search"`
	TotalResults string             `json:"totalResults"`
	Response     string             `json:"Response"`
	Error        string             `json:"Error"`
}

// OmdbSearchResult represents a single search result from OMDB API
// Exported to allow direct access to IMDb IDs which are not in the generic interface.
type OmdbSearchResult struct {
	Title  string `json:"Title"`
	Year   string `json:"Year"`
	ImdbID string `json:"imdbID"`
	Type   string `json:"Type"`
	Poster string `json:"Poster"`
}

// Keep lowercase alias for internal use to avoid breaking changes.
type omdbSearchResult = OmdbSearchResult

type omdbDetailsResponse struct {
	Title      string       `json:"Title"`
	Year       string       `json:"Year"`
	Rated      string       `json:"Rated"`
	Released   string       `json:"Released"`
	Runtime    string       `json:"Runtime"`
	Genre      string       `json:"Genre"`
	Director   string       `json:"Director"`
	Writer     string       `json:"Writer"`
	Actors     string       `json:"Actors"`
	Plot       string       `json:"Plot"`
	Language   string       `json:"Language"`
	Country    string       `json:"Country"`
	Awards     string       `json:"Awards"`
	Poster     string       `json:"Poster"`
	Ratings    []omdbRating `json:"Ratings"`
	Metascore  string       `json:"Metascore"`
	ImdbRating string       `json:"imdbRating"`
	ImdbVotes  string       `json:"imdbVotes"`
	ImdbID     string       `json:"imdbID"`
	Type       string       `json:"Type"`
	DVD        string       `json:"DVD"`
	BoxOffice  string       `json:"BoxOffice"`
	Production string       `json:"Production"`
	Website    string       `json:"Website"`
	Response   string       `json:"Response"`
	Error      string       `json:"Error"`

	// Series-specific fields
	TotalSeasons string `json:"totalSeasons"`
	Season       string `json:"Season"`
	Episode      string `json:"Episode"`
	SeriesID     string `json:"seriesID"`
}

type omdbRating struct {
	Source string `json:"Source"`
	Value  string `json:"Value"`
}

//
// Conversion Functions
//

func convertSearchResults(
	omdbResults []omdbSearchResult,
	provider string,
) []apiexternal_v2.MovieSearchResult {
	results := make([]apiexternal_v2.MovieSearchResult, 0, len(omdbResults))

	for _, r := range omdbResults {
		if strings.ToLower(r.Type) == "movie" {
			results = append(results, apiexternal_v2.MovieSearchResult{
				ID:           0, // OMDB doesn't use numeric IDs
				Title:        r.Title,
				Year:         parseYear(r.Year),
				PosterPath:   r.Poster,
				ProviderName: provider,
				IMDbID:       r.ImdbID, // OMDB provides IMDb ID directly in search results
			})
		}
	}

	return results
}

func convertSearchToSeriesResults(
	omdbResults []omdbSearchResult,
	provider string,
) []apiexternal_v2.SeriesSearchResult {
	results := make([]apiexternal_v2.SeriesSearchResult, 0, len(omdbResults))

	for _, r := range omdbResults {
		if strings.ToLower(r.Type) == "series" {
			results = append(results, apiexternal_v2.SeriesSearchResult{
				ID:           0, // OMDB doesn't use numeric IDs
				Name:         r.Title,
				PosterPath:   r.Poster,
				ProviderName: provider,
			})
		}
	}

	return results
}

func convertDetailsToSearchResult(details *omdbDetailsResponse) apiexternal_v2.MovieSearchResult {
	return apiexternal_v2.MovieSearchResult{
		ID:           0, // OMDB uses IMDb ID strings
		Title:        details.Title,
		Year:         parseYear(details.Year),
		ReleaseDate:  parseOMDBDate(details.Released),
		PosterPath:   details.Poster,
		Overview:     details.Plot,
		VoteAverage:  parseRating(details.ImdbRating),
		ProviderName: "omdb",
	}
}

func convertDetailsToSeriesSearchResult(
	details *omdbDetailsResponse,
) apiexternal_v2.SeriesSearchResult {
	return apiexternal_v2.SeriesSearchResult{
		ID:           0,
		Name:         details.Title,
		FirstAirDate: parseOMDBDate(details.Released),
		PosterPath:   details.Poster,
		Overview:     details.Plot,
		VoteAverage:  parseRating(details.ImdbRating),
		ProviderName: "omdb",
	}
}

func convertDetailsToMovieDetails(details *omdbDetailsResponse) *apiexternal_v2.MovieDetails {
	// Parse genres
	genres := []apiexternal_v2.Genre{}
	if details.Genre != "" {
		genreNames := strings.Split(details.Genre, ", ")
		for i, name := range genreNames {
			genres = append(genres, apiexternal_v2.Genre{
				ID:   i + 1,
				Name: name,
			})
		}
	}

	// Parse spoken languages
	languages := []apiexternal_v2.SpokenLanguage{}
	if details.Language != "" {
		langNames := strings.Split(details.Language, ", ")
		for _, name := range langNames {
			languages = append(languages, apiexternal_v2.SpokenLanguage{
				Name:        name,
				EnglishName: name,
			})
		}
	}

	return &apiexternal_v2.MovieDetails{
		ID:              0, // OMDB doesn't use numeric IDs
		IMDbID:          details.ImdbID,
		Title:           details.Title,
		OriginalTitle:   details.Title,
		Overview:        details.Plot,
		Year:            parseYear(details.Year),
		ReleaseDate:     parseOMDBDate(details.Released),
		Runtime:         parseRuntime(details.Runtime),
		VoteAverage:     parseRating(details.ImdbRating),
		PosterPath:      details.Poster,
		Genres:          genres,
		SpokenLanguages: languages,
		ProviderName:    "omdb",
	}
}

// func convertDetailsToSeriesDetails(details *omdbDetailsResponse) *apiexternal_v2.SeriesDetails {
// 	totalSeasons := 0
// 	if details.TotalSeasons != "" {
// 		if seasons, err := strconv.Atoi(details.TotalSeasons); err == nil {
// 			totalSeasons = seasons
// 		}
// 	}

// 	// Parse genres
// 	genres := []apiexternal_v2.Genre{}
// 	if details.Genre != "" {
// 		genreNames := strings.Split(details.Genre, ", ")
// 		for i, name := range genreNames {
// 			genres = append(genres, apiexternal_v2.Genre{
// 				ID:   i + 1,
// 				Name: name,
// 			})
// 		}
// 	}

// 	return &apiexternal_v2.SeriesDetails{
// 		ID:              0,
// 		IMDbID:          details.ImdbID,
// 		Name:            details.Title,
// 		OriginalName:    details.Title,
// 		Overview:        details.Plot,
// 		FirstAirDate:    parseOMDBDate(details.Released),
// 		NumberOfSeasons: totalSeasons,
// 		VoteAverage:     parseRating(details.ImdbRating),
// 		PosterPath:      details.Poster,
// 		Genres:          genres,
// 		ProviderName:    "omdb",
// 	}
// }

func convertDetailsToEpisode(
	details *omdbDetailsResponse,
	seasonNum, episodeNum int,
) *apiexternal_v2.Episode {
	return &apiexternal_v2.Episode{
		ID:            0,
		EpisodeNumber: episodeNum,
		SeasonNumber:  seasonNum,
		Name:          details.Title,
		Overview:      details.Plot,
		AirDate:       parseOMDBDate(details.Released),
		Runtime:       parseRuntime(details.Runtime),
		VoteAverage:   parseRating(details.ImdbRating),
	}
}

func convertDetailsToCredits(details *omdbDetailsResponse) *apiexternal_v2.Credits {
	credits := &apiexternal_v2.Credits{
		Cast: []apiexternal_v2.CastMember{},
		Crew: []apiexternal_v2.CrewMember{},
	}

	// Parse actors (OMDB provides comma-separated list)
	if details.Actors != "" && details.Actors != "N/A" {
		actorNames := strings.Split(details.Actors, ", ")
		for i, name := range actorNames {
			credits.Cast = append(credits.Cast, apiexternal_v2.CastMember{
				ID:        i + 1,
				Name:      name,
				Character: "", // OMDB doesn't provide character names
				Order:     i,
			})
		}
	}

	// Parse director
	if details.Director != "" && details.Director != "N/A" {
		directorNames := strings.Split(details.Director, ", ")
		for i, name := range directorNames {
			credits.Crew = append(credits.Crew, apiexternal_v2.CrewMember{
				ID:         i + 1,
				Name:       name,
				Job:        "Director",
				Department: "Directing",
			})
		}
	}

	// Parse writer
	if details.Writer != "" && details.Writer != "N/A" {
		writerNames := strings.Split(details.Writer, ", ")
		for i, name := range writerNames {
			credits.Crew = append(credits.Crew, apiexternal_v2.CrewMember{
				ID:         len(credits.Crew) + i + 1,
				Name:       name,
				Job:        "Writer",
				Department: "Writing",
			})
		}
	}

	return credits
}

//
// Helper Functions
//

func parseOMDBDate(dateStr string) time.Time {
	if dateStr == "" || dateStr == "N/A" {
		return time.Time{}
	}

	// OMDB format: "02 Jan 2020"
	layouts := []string{
		"02 Jan 2006",
		"2006-01-02",
		"2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

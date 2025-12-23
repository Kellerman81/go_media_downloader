package trakt

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// Trakt Internal Types - Used for JSON unmarshaling (Trakt API v2)
//

type traktSearchResponse []traktSearchResult

type traktSearchResult struct {
	Type  string      `json:"type"`
	Score float64     `json:"score"`
	Movie *TraktMovie `json:"movie,omitempty"`
	Show  *TraktShow  `json:"show,omitempty"`
}

// TraktMovie represents a movie from Trakt API.
type TraktMovie struct {
	Title         string   `json:"title"`
	Year          int      `json:"year"`
	IDs           TraktIDs `json:"ids"`
	Tagline       string   `json:"tagline,omitempty"`
	Overview      string   `json:"overview,omitempty"`
	Released      string   `json:"released,omitempty"`
	Runtime       int      `json:"runtime,omitempty"`
	Trailer       string   `json:"trailer,omitempty"`
	Homepage      string   `json:"homepage,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	Votes         int      `json:"votes,omitempty"`
	Language      string   `json:"language,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Certification string   `json:"certification,omitempty"`
}

// TraktShow represents a TV show from Trakt API.
type TraktShow struct {
	Title         string   `json:"title"`
	Year          int      `json:"year"`
	IDs           TraktIDs `json:"ids"`
	Overview      string   `json:"overview,omitempty"`
	FirstAired    string   `json:"first_aired,omitempty"`
	Runtime       int      `json:"runtime,omitempty"`
	Certification string   `json:"certification,omitempty"`
	Network       string   `json:"network,omitempty"`
	Country       string   `json:"country,omitempty"`
	Trailer       string   `json:"trailer,omitempty"`
	Homepage      string   `json:"homepage,omitempty"`
	Status        string   `json:"status,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	Votes         int      `json:"votes,omitempty"`
	Language      string   `json:"language,omitempty"`
	AiredEpisodes int      `json:"aired_episodes,omitempty"`
	Genres        []string `json:"genres,omitempty"`
}

// TraktIDs represents external IDs for a Trakt item.
type TraktIDs struct {
	Trakt int    `json:"trakt"`
	Slug  string `json:"slug"`
	IMDB  string `json:"imdb,omitempty"`
	TMDB  int    `json:"tmdb,omitempty"`
	TVDB  int    `json:"tvdb,omitempty"`
}

type traktMovieResponse struct {
	Title         string   `json:"title"`
	Year          int      `json:"year"`
	IDs           TraktIDs `json:"ids"`
	Tagline       string   `json:"tagline,omitempty"`
	Overview      string   `json:"overview,omitempty"`
	Released      string   `json:"released,omitempty"`
	Runtime       int      `json:"runtime,omitempty"`
	Trailer       string   `json:"trailer,omitempty"`
	Homepage      string   `json:"homepage,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	Votes         int      `json:"votes,omitempty"`
	Language      string   `json:"language,omitempty"`
	Genres        []string `json:"genres,omitempty"`
	Certification string   `json:"certification,omitempty"`
}

type traktShowResponse struct {
	Title         string   `json:"title"`
	Year          int      `json:"year"`
	IDs           TraktIDs `json:"ids"`
	Overview      string   `json:"overview,omitempty"`
	FirstAired    string   `json:"first_aired,omitempty"`
	Runtime       int      `json:"runtime,omitempty"`
	Certification string   `json:"certification,omitempty"`
	Network       string   `json:"network,omitempty"`
	Country       string   `json:"country,omitempty"`
	Trailer       string   `json:"trailer,omitempty"`
	Homepage      string   `json:"homepage,omitempty"`
	Status        string   `json:"status,omitempty"`
	Rating        float64  `json:"rating,omitempty"`
	Votes         int      `json:"votes,omitempty"`
	Language      string   `json:"language,omitempty"`
	AiredEpisodes int      `json:"aired_episodes,omitempty"`
	Genres        []string `json:"genres,omitempty"`
}

type traktSeasonResponse struct {
	Number        int            `json:"number"`
	IDs           TraktIDs       `json:"ids"`
	Rating        float64        `json:"rating,omitempty"`
	Votes         int            `json:"votes,omitempty"`
	EpisodeCount  int            `json:"episode_count,omitempty"`
	AiredEpisodes int            `json:"aired_episodes,omitempty"`
	Title         string         `json:"title,omitempty"`
	Overview      string         `json:"overview,omitempty"`
	FirstAired    string         `json:"first_aired,omitempty"`
	Network       string         `json:"network,omitempty"`
	Episodes      []traktEpisode `json:"episodes,omitempty"`
}

type traktEpisode struct {
	Season     int      `json:"season"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	IDs        TraktIDs `json:"ids"`
	NumberAbs  int      `json:"number_abs,omitempty"`
	Overview   string   `json:"overview,omitempty"`
	Rating     float64  `json:"rating,omitempty"`
	Votes      int      `json:"votes,omitempty"`
	FirstAired string   `json:"first_aired,omitempty"`
	Runtime    int      `json:"runtime,omitempty"`
}

type traktPopularResponse []traktPopularItem

type traktPopularItem struct {
	WatcherCount   int         `json:"watcher_count,omitempty"`
	PlayCount      int         `json:"play_count,omitempty"`
	CollectedCount int         `json:"collected_count,omitempty"`
	Movie          *TraktMovie `json:"movie,omitempty"`
	Show           *TraktShow  `json:"show,omitempty"`
}

type traktCastMember struct {
	Character string      `json:"character"`
	Person    traktPerson `json:"person"`
}

type traktCrewMember struct {
	Job    string      `json:"job"`
	Person traktPerson `json:"person"`
}

type traktPerson struct {
	Name string   `json:"name"`
	IDs  TraktIDs `json:"ids"`
}

type traktCreditsResponse struct {
	Cast []traktCastMember            `json:"cast"`
	Crew map[string][]traktCrewMember `json:"crew"`
}

// TraktUserListItem represents an item in a Trakt user list.
type TraktUserListItem struct {
	Rank     int         `json:"rank,omitempty"`
	ListedAt string      `json:"listed_at,omitempty"`
	Type     string      `json:"type"`
	Movie    *TraktMovie `json:"movie,omitempty"`
	Show     *TraktShow  `json:"show,omitempty"`
}

// traktAnticipatedItem represents an anticipated item response.
type traktAnticipatedItem struct {
	ListCount int         `json:"list_count"`
	Show      *TraktShow  `json:"show,omitempty"`
	Movie     *TraktMovie `json:"movie,omitempty"`
}

// traktAlias represents an alternative title from Trakt.
type traktAlias struct {
	Title   string `json:"title"`
	Country string `json:"country"`
}

//
// Conversion Functions
//

func convertSearchResults(
	traktResults []traktSearchResult,
	provider string,
) []apiexternal_v2.MovieSearchResult {
	results := make([]apiexternal_v2.MovieSearchResult, 0, len(traktResults))

	for _, r := range traktResults {
		if r.Type == "movie" && r.Movie != nil {
			results = append(results, apiexternal_v2.MovieSearchResult{
				ID:           r.Movie.IDs.Trakt,
				Title:        r.Movie.Title,
				Year:         r.Movie.Year,
				ReleaseDate:  parseTraktDate(r.Movie.Released),
				Overview:     r.Movie.Overview,
				VoteAverage:  r.Movie.Rating,
				IMDbID:       r.Movie.IDs.IMDB,
				ProviderName: provider,
			})
		}
	}

	return results
}

func convertSearchToSeriesResults(
	traktResults []traktSearchResult,
	provider string,
) []apiexternal_v2.SeriesSearchResult {
	results := make([]apiexternal_v2.SeriesSearchResult, 0, len(traktResults))

	for _, r := range traktResults {
		if r.Type == "show" && r.Show != nil {
			results = append(results, apiexternal_v2.SeriesSearchResult{
				ID:           r.Show.IDs.Trakt,
				Name:         r.Show.Title,
				FirstAirDate: parseTraktDate(r.Show.FirstAired),
				Overview:     r.Show.Overview,
				VoteAverage:  r.Show.Rating,
				ProviderName: provider,
			})
		}
	}

	return results
}

func convertMovieToDetails(movie *traktMovieResponse) *apiexternal_v2.MovieDetails {
	// Convert genres
	genres := make([]apiexternal_v2.Genre, len(movie.Genres))
	for i, g := range movie.Genres {
		genres[i] = apiexternal_v2.Genre{
			ID:   i + 1,
			Name: g,
		}
	}

	return &apiexternal_v2.MovieDetails{
		ID:            movie.IDs.Trakt,
		IMDbID:        movie.IDs.IMDB,
		Title:         movie.Title,
		OriginalTitle: movie.Title,
		Overview:      movie.Overview,
		Year:          movie.Year,
		ReleaseDate:   parseTraktDate(movie.Released),
		Runtime:       movie.Runtime,
		VoteAverage:   movie.Rating,
		VoteCount:     movie.Votes,
		Genres:        genres,
		Homepage:      movie.Homepage,
		ProviderName:  "trakt",
	}
}

func convertShowToDetails(show *traktShowResponse) *apiexternal_v2.SeriesDetails {
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
	if show.Network != "" {
		networks = append(networks, apiexternal_v2.Network{
			ID:            1,
			Name:          show.Network,
			OriginCountry: show.Country,
		})
	}

	return &apiexternal_v2.SeriesDetails{
		ID:               show.IDs.Trakt,
		TVDbID:           show.IDs.TVDB,
		IMDbID:           show.IDs.IMDB,
		Name:             show.Title,
		OriginalName:     show.Title,
		Overview:         show.Overview,
		FirstAirDate:     parseTraktDate(show.FirstAired),
		Status:           show.Status,
		NumberOfEpisodes: show.AiredEpisodes,
		VoteAverage:      show.Rating,
		VoteCount:        show.Votes,
		Genres:           genres,
		Networks:         networks,
		Homepage:         show.Homepage,
		ProviderName:     "trakt",
	}
}

func convertSeasonToDetails(season *traktSeasonResponse) *apiexternal_v2.Season {
	return &apiexternal_v2.Season{
		ID:           season.IDs.Trakt,
		SeasonNumber: season.Number,
		Name:         season.Title,
		Overview:     season.Overview,
		AirDate:      parseTraktDate(season.FirstAired),
		EpisodeCount: season.EpisodeCount,
	}
}

func convertEpisodeToDetails(episode *traktEpisode) *apiexternal_v2.Episode {
	return &apiexternal_v2.Episode{
		ID:            episode.IDs.Trakt,
		EpisodeNumber: episode.Number,
		SeasonNumber:  episode.Season,
		Name:          episode.Title,
		Overview:      episode.Overview,
		AirDate:       parseTraktDate(episode.FirstAired),
		Runtime:       episode.Runtime,
		VoteAverage:   episode.Rating,
		VoteCount:     episode.Votes,
	}
}

func convertPopularMovies(
	items []traktPopularItem,
	page int,
) *apiexternal_v2.PopularMoviesResponse {
	results := make([]apiexternal_v2.MovieSearchResult, 0, len(items))

	for _, item := range items {
		if item.Movie != nil {
			results = append(results, apiexternal_v2.MovieSearchResult{
				ID:           item.Movie.IDs.Trakt,
				Title:        item.Movie.Title,
				Year:         item.Movie.Year,
				ReleaseDate:  parseTraktDate(item.Movie.Released),
				Overview:     item.Movie.Overview,
				VoteAverage:  item.Movie.Rating,
				IMDbID:       item.Movie.IDs.IMDB,
				ProviderName: "trakt",
			})
		}
	}

	return &apiexternal_v2.PopularMoviesResponse{
		Page:         page,
		Results:      results,
		TotalPages:   1, // Trakt doesn't provide total pages
		TotalResults: len(results),
	}
}

func convertPopularSeries(
	items []traktPopularItem,
	page int,
) *apiexternal_v2.PopularSeriesResponse {
	results := make([]apiexternal_v2.SeriesSearchResult, 0, len(items))

	for _, item := range items {
		if item.Show != nil {
			results = append(results, apiexternal_v2.SeriesSearchResult{
				ID:           item.Show.IDs.Trakt,
				Name:         item.Show.Title,
				FirstAirDate: parseTraktDate(item.Show.FirstAired),
				Overview:     item.Show.Overview,
				VoteAverage:  item.Show.Rating,
				ProviderName: "trakt",
			})
		}
	}

	return &apiexternal_v2.PopularSeriesResponse{
		Page:         page,
		Results:      results,
		TotalPages:   1, // Trakt doesn't provide total pages
		TotalResults: len(results),
	}
}

func convertCredits(credits *traktCreditsResponse) *apiexternal_v2.Credits {
	cast := make([]apiexternal_v2.CastMember, len(credits.Cast))
	for i, c := range credits.Cast {
		cast[i] = apiexternal_v2.CastMember{
			ID:        c.Person.IDs.Trakt,
			Name:      c.Person.Name,
			Character: c.Character,
			Order:     i,
		}
	}

	crew := []apiexternal_v2.CrewMember{}
	for department, members := range credits.Crew {
		for _, m := range members {
			crew = append(crew, apiexternal_v2.CrewMember{
				ID:         m.Person.IDs.Trakt,
				Name:       m.Person.Name,
				Job:        m.Job,
				Department: department,
			})
		}
	}

	return &apiexternal_v2.Credits{
		Cast: cast,
		Crew: crew,
	}
}

func convertIDsToExternalIDs(ids TraktIDs) *apiexternal_v2.ExternalIDs {
	return &apiexternal_v2.ExternalIDs{
		IMDbID: ids.IMDB,
		TMDbID: ids.TMDB,
		TVDbID: ids.TVDB,
	}
}

//
// Helper Functions
//

func parseTraktDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05.000Z",
		"2006-01-02",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

// func traktIDToString(id int) string {
// 	if id == 0 {
// 		return ""
// 	}
// 	return strconv.Itoa(id)
// }

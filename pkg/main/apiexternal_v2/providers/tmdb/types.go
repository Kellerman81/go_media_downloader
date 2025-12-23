package tmdb

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// TMDB Internal Types - Used only for JSON unmarshaling
// These are internal to the package and converted to apiexternal_v2 types
//

type tmdbMovieSearchResult struct {
	ID            int     `json:"id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	ReleaseDate   string  `json:"release_date"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	Overview      string  `json:"overview"`
	VoteAverage   float64 `json:"vote_average"`
	VoteCount     int     `json:"vote_count"`
	Popularity    float64 `json:"popularity"`
	Adult         bool    `json:"adult"`
}

type tmdbSeriesSearchResult struct {
	ID           int     `json:"id"`
	Name         string  `json:"name"`
	OriginalName string  `json:"original_name"`
	FirstAirDate string  `json:"first_air_date"`
	PosterPath   string  `json:"poster_path"`
	BackdropPath string  `json:"backdrop_path"`
	Overview     string  `json:"overview"`
	VoteAverage  float64 `json:"vote_average"`
	VoteCount    int     `json:"vote_count"`
	Popularity   float64 `json:"popularity"`
}

type tmdbMovieDetails struct {
	ID                  int                        `json:"id"`
	IMDbID              string                     `json:"imdb_id"`
	Title               string                     `json:"title"`
	OriginalTitle       string                     `json:"original_title"`
	OriginalLanguage    string                     `json:"original_language"`
	Tagline             string                     `json:"tagline"`
	Overview            string                     `json:"overview"`
	ReleaseDate         string                     `json:"release_date"`
	Runtime             int                        `json:"runtime"`
	Budget              int64                      `json:"budget"`
	Revenue             int64                      `json:"revenue"`
	VoteAverage         float64                    `json:"vote_average"`
	VoteCount           int                        `json:"vote_count"`
	Popularity          float64                    `json:"popularity"`
	Adult               bool                       `json:"adult"`
	PosterPath          string                     `json:"poster_path"`
	BackdropPath        string                     `json:"backdrop_path"`
	Homepage            string                     `json:"homepage"`
	Status              string                     `json:"status"`
	Genres              []tmdbGenre                `json:"genres"`
	ProductionCompanies []tmdbProductionCompany    `json:"production_companies"`
	SpokenLanguages     []tmdbSpokenLanguage       `json:"spoken_languages"`
	AlternativeTitles   *tmdbAlternativeTitlesResp `json:"alternative_titles"`
	Credits             *tmdbCredits               `json:"credits"`
}

type tmdbSeriesDetails struct {
	ID                  int                        `json:"id"`
	Name                string                     `json:"name"`
	OriginalName        string                     `json:"original_name"`
	OriginalLanguage    string                     `json:"original_language"`
	Overview            string                     `json:"overview"`
	FirstAirDate        string                     `json:"first_air_date"`
	LastAirDate         string                     `json:"last_air_date"`
	Status              string                     `json:"status"`
	Type                string                     `json:"type"`
	NumberOfSeasons     int                        `json:"number_of_seasons"`
	NumberOfEpisodes    int                        `json:"number_of_episodes"`
	EpisodeRunTime      []int                      `json:"episode_run_time"`
	VoteAverage         float64                    `json:"vote_average"`
	VoteCount           int                        `json:"vote_count"`
	Popularity          float64                    `json:"popularity"`
	PosterPath          string                     `json:"poster_path"`
	BackdropPath        string                     `json:"backdrop_path"`
	Homepage            string                     `json:"homepage"`
	Genres              []tmdbGenre                `json:"genres"`
	Networks            []tmdbNetwork              `json:"networks"`
	ProductionCompanies []tmdbProductionCompany    `json:"production_companies"`
	Seasons             []tmdbSeason               `json:"seasons"`
	AlternativeTitles   *tmdbAlternativeTitlesResp `json:"alternative_titles"`
	Credits             *tmdbCredits               `json:"credits"`
	ExternalIDs         *tmdbExternalIDs           `json:"external_ids"`
}

type tmdbGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type tmdbProductionCompany struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type tmdbNetwork struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

type tmdbSpokenLanguage struct {
	ISO639_1    string `json:"iso_639_1"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name"`
}

type tmdbAlternativeTitlesResp struct {
	Titles  []tmdbAlternativeTitle `json:"titles"`
	Results []tmdbAlternativeTitle `json:"results"`
}

type tmdbAlternativeTitle struct {
	Title     string `json:"title"`
	ISO3166_1 string `json:"iso_3166_1"`
	Type      string `json:"type"`
}

type tmdbSeason struct {
	ID           int    `json:"id"`
	SeasonNumber int    `json:"season_number"`
	Name         string `json:"name"`
	Overview     string `json:"overview"`
	AirDate      string `json:"air_date"`
	EpisodeCount int    `json:"episode_count"`
	PosterPath   string `json:"poster_path"`
}

type tmdbEpisode struct {
	ID            int        `json:"id"`
	EpisodeNumber int        `json:"episode_number"`
	SeasonNumber  int        `json:"season_number"`
	Name          string     `json:"name"`
	Overview      string     `json:"overview"`
	AirDate       string     `json:"air_date"`
	Runtime       int        `json:"runtime"`
	VoteAverage   float64    `json:"vote_average"`
	VoteCount     int        `json:"vote_count"`
	StillPath     string     `json:"still_path"`
	Crew          []tmdbCrew `json:"crew"`
	GuestStars    []tmdbCast `json:"guest_stars"`
}

type tmdbCredits struct {
	Cast []tmdbCast `json:"cast"`
	Crew []tmdbCrew `json:"crew"`
}

type tmdbCast struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
	Gender      int    `json:"gender"`
}

type tmdbCrew struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
	Gender      int    `json:"gender"`
}

type tmdbExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	TVDbID      int    `json:"tvdb_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

type tmdbImageCollection struct {
	Backdrops []tmdbImage `json:"backdrops"`
	Posters   []tmdbImage `json:"posters"`
	Logos     []tmdbImage `json:"logos"`
}

type tmdbImage struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	ISO639_1    string  `json:"iso_639_1"`
}

type tmdbVideo struct {
	ID          string `json:"id"`
	Key         string `json:"key"`
	Name        string `json:"name"`
	Site        string `json:"site"`
	Size        int    `json:"size"`
	Type        string `json:"type"`
	Official    bool   `json:"official"`
	PublishedAt string `json:"published_at"`
}

// TMDBListResponse represents a TMDB list response (movies).
type TMDBListResponse struct {
	CreatedBy     string                  `json:"created_by"`
	Description   string                  `json:"description"`
	FavoriteCount int                     `json:"favorite_count"`
	ID            string                  `json:"id"`
	Items         []tmdbMovieSearchResult `json:"items"`
	ItemCount     int                     `json:"item_count"`
	ISO6391       string                  `json:"iso_639_1"`
	Name          string                  `json:"name"`
	PosterPath    string                  `json:"poster_path"`
}

//
// Conversion Functions - Convert TMDB types to apiexternal_v2 types
//

func convertMovieSearchResults(
	tmdbResults []tmdbMovieSearchResult,
) []apiexternal_v2.MovieSearchResult {
	results := make([]apiexternal_v2.MovieSearchResult, len(tmdbResults))
	for i, r := range tmdbResults {
		results[i] = apiexternal_v2.MovieSearchResult{
			ID:            r.ID,
			Title:         r.Title,
			OriginalTitle: r.OriginalTitle,
			Year:          extractYear(r.ReleaseDate),
			ReleaseDate:   parseDate(r.ReleaseDate),
			PosterPath:    r.PosterPath,
			BackdropPath:  r.BackdropPath,
			Overview:      r.Overview,
			VoteAverage:   r.VoteAverage,
			VoteCount:     r.VoteCount,
			Popularity:    r.Popularity,
			Adult:         r.Adult,
			ProviderName:  "tmdb",
		}
	}

	return results
}

func convertSeriesSearchResults(
	tmdbResults []tmdbSeriesSearchResult,
) []apiexternal_v2.SeriesSearchResult {
	results := make([]apiexternal_v2.SeriesSearchResult, len(tmdbResults))
	for i, r := range tmdbResults {
		results[i] = apiexternal_v2.SeriesSearchResult{
			ID:           r.ID,
			Name:         r.Name,
			OriginalName: r.OriginalName,
			FirstAirDate: parseDate(r.FirstAirDate),
			PosterPath:   r.PosterPath,
			BackdropPath: r.BackdropPath,
			Overview:     r.Overview,
			VoteAverage:  r.VoteAverage,
			VoteCount:    r.VoteCount,
			Popularity:   r.Popularity,
			ProviderName: "tmdb",
		}
	}

	return results
}

func convertMovieDetails(tmdb *tmdbMovieDetails) *apiexternal_v2.MovieDetails {
	details := &apiexternal_v2.MovieDetails{
		ID:                  tmdb.ID,
		IMDbID:              tmdb.IMDbID,
		Title:               tmdb.Title,
		OriginalTitle:       tmdb.OriginalTitle,
		OriginalLanguage:    tmdb.OriginalLanguage,
		Tagline:             tmdb.Tagline,
		Overview:            tmdb.Overview,
		Year:                extractYear(tmdb.ReleaseDate),
		ReleaseDate:         parseDate(tmdb.ReleaseDate),
		Runtime:             tmdb.Runtime,
		Budget:              tmdb.Budget,
		Revenue:             tmdb.Revenue,
		VoteAverage:         tmdb.VoteAverage,
		VoteCount:           tmdb.VoteCount,
		Popularity:          tmdb.Popularity,
		Adult:               tmdb.Adult,
		PosterPath:          tmdb.PosterPath,
		BackdropPath:        tmdb.BackdropPath,
		Homepage:            tmdb.Homepage,
		Status:              tmdb.Status,
		Genres:              convertGenres(tmdb.Genres),
		ProductionCompanies: convertProductionCompanies(tmdb.ProductionCompanies),
		SpokenLanguages:     convertSpokenLanguages(tmdb.SpokenLanguages),
		ProviderName:        "tmdb",
	}

	if tmdb.AlternativeTitles != nil {
		details.AlternativeTitles = convertAlternativeTitles(tmdb.AlternativeTitles.Titles)
	}

	if tmdb.Credits != nil {
		details.Credits = convertCredits(tmdb.Credits)
	}

	return details
}

func convertSeriesDetails(tmdb *tmdbSeriesDetails) *apiexternal_v2.SeriesDetails {
	imdbID := ""

	tvdbID := 0
	if tmdb.ExternalIDs != nil {
		imdbID = tmdb.ExternalIDs.IMDbID
		tvdbID = tmdb.ExternalIDs.TVDbID
	}

	details := &apiexternal_v2.SeriesDetails{
		ID:                  tmdb.ID,
		TVDbID:              tvdbID,
		IMDbID:              imdbID,
		Name:                tmdb.Name,
		OriginalName:        tmdb.OriginalName,
		OriginalLanguage:    tmdb.OriginalLanguage,
		Overview:            tmdb.Overview,
		FirstAirDate:        parseDate(tmdb.FirstAirDate),
		LastAirDate:         parseDate(tmdb.LastAirDate),
		Status:              tmdb.Status,
		Type:                tmdb.Type,
		NumberOfSeasons:     tmdb.NumberOfSeasons,
		NumberOfEpisodes:    tmdb.NumberOfEpisodes,
		EpisodeRunTime:      tmdb.EpisodeRunTime,
		VoteAverage:         tmdb.VoteAverage,
		VoteCount:           tmdb.VoteCount,
		Popularity:          tmdb.Popularity,
		PosterPath:          tmdb.PosterPath,
		BackdropPath:        tmdb.BackdropPath,
		Homepage:            tmdb.Homepage,
		Genres:              convertGenres(tmdb.Genres),
		Networks:            convertNetworks(tmdb.Networks),
		ProductionCompanies: convertProductionCompanies(tmdb.ProductionCompanies),
		Seasons:             convertSeasons(tmdb.Seasons),
		ProviderName:        "tmdb",
	}

	if tmdb.AlternativeTitles != nil {
		titles := tmdb.AlternativeTitles.Titles
		if len(titles) == 0 {
			titles = tmdb.AlternativeTitles.Results
		}

		details.AlternativeTitles = convertAlternativeTitles(titles)
	}

	if tmdb.Credits != nil {
		details.Credits = convertCredits(tmdb.Credits)
	}

	return details
}

func convertGenres(tmdbGenres []tmdbGenre) []apiexternal_v2.Genre {
	genres := make([]apiexternal_v2.Genre, len(tmdbGenres))
	for i, g := range tmdbGenres {
		genres[i] = apiexternal_v2.Genre{
			ID:   g.ID,
			Name: g.Name,
		}
	}

	return genres
}

func convertProductionCompanies(
	tmdbCompanies []tmdbProductionCompany,
) []apiexternal_v2.ProductionCompany {
	companies := make([]apiexternal_v2.ProductionCompany, len(tmdbCompanies))
	for i, c := range tmdbCompanies {
		companies[i] = apiexternal_v2.ProductionCompany{
			ID:            c.ID,
			Name:          c.Name,
			LogoPath:      c.LogoPath,
			OriginCountry: c.OriginCountry,
		}
	}

	return companies
}

func convertNetworks(tmdbNetworks []tmdbNetwork) []apiexternal_v2.Network {
	networks := make([]apiexternal_v2.Network, len(tmdbNetworks))
	for i, n := range tmdbNetworks {
		networks[i] = apiexternal_v2.Network{
			ID:            n.ID,
			Name:          n.Name,
			LogoPath:      n.LogoPath,
			OriginCountry: n.OriginCountry,
		}
	}

	return networks
}

func convertSpokenLanguages(tmdbLangs []tmdbSpokenLanguage) []apiexternal_v2.SpokenLanguage {
	langs := make([]apiexternal_v2.SpokenLanguage, len(tmdbLangs))
	for i, l := range tmdbLangs {
		langs[i] = apiexternal_v2.SpokenLanguage{
			ISO639_1:    l.ISO639_1,
			Name:        l.Name,
			EnglishName: l.EnglishName,
		}
	}

	return langs
}

func convertAlternativeTitles(tmdbTitles []tmdbAlternativeTitle) []apiexternal_v2.AlternativeTitle {
	titles := make([]apiexternal_v2.AlternativeTitle, len(tmdbTitles))
	for i, t := range tmdbTitles {
		titles[i] = apiexternal_v2.AlternativeTitle{
			Title:     t.Title,
			ISO3166_1: t.ISO3166_1,
			Type:      t.Type,
		}
	}

	return titles
}

func convertSeasons(tmdbSeasons []tmdbSeason) []apiexternal_v2.Season {
	seasons := make([]apiexternal_v2.Season, len(tmdbSeasons))
	for i, s := range tmdbSeasons {
		seasons[i] = apiexternal_v2.Season{
			ID:           s.ID,
			SeasonNumber: s.SeasonNumber,
			Name:         s.Name,
			Overview:     s.Overview,
			AirDate:      parseDate(s.AirDate),
			EpisodeCount: s.EpisodeCount,
			PosterPath:   s.PosterPath,
		}
	}

	return seasons
}

func convertSeason(tmdbSeason *tmdbSeason) *apiexternal_v2.Season {
	return &apiexternal_v2.Season{
		ID:           tmdbSeason.ID,
		SeasonNumber: tmdbSeason.SeasonNumber,
		Name:         tmdbSeason.Name,
		Overview:     tmdbSeason.Overview,
		AirDate:      parseDate(tmdbSeason.AirDate),
		EpisodeCount: tmdbSeason.EpisodeCount,
		PosterPath:   tmdbSeason.PosterPath,
	}
}

func convertEpisode(tmdbEp *tmdbEpisode) *apiexternal_v2.Episode {
	return &apiexternal_v2.Episode{
		ID:            tmdbEp.ID,
		EpisodeNumber: tmdbEp.EpisodeNumber,
		SeasonNumber:  tmdbEp.SeasonNumber,
		Name:          tmdbEp.Name,
		Overview:      tmdbEp.Overview,
		AirDate:       parseDate(tmdbEp.AirDate),
		Runtime:       tmdbEp.Runtime,
		VoteAverage:   tmdbEp.VoteAverage,
		VoteCount:     tmdbEp.VoteCount,
		StillPath:     tmdbEp.StillPath,
		Crew:          convertCrewMembers(tmdbEp.Crew),
		GuestStars:    convertCastMembers(tmdbEp.GuestStars),
	}
}

func convertCredits(tmdbCredits *tmdbCredits) *apiexternal_v2.Credits {
	return &apiexternal_v2.Credits{
		Cast: convertCastMembers(tmdbCredits.Cast),
		Crew: convertCrewMembers(tmdbCredits.Crew),
	}
}

func convertCastMembers(tmdbCast []tmdbCast) []apiexternal_v2.CastMember {
	cast := make([]apiexternal_v2.CastMember, len(tmdbCast))
	for i, c := range tmdbCast {
		cast[i] = apiexternal_v2.CastMember{
			ID:          c.ID,
			Name:        c.Name,
			Character:   c.Character,
			Order:       c.Order,
			ProfilePath: c.ProfilePath,
			Gender:      c.Gender,
		}
	}

	return cast
}

func convertCrewMembers(tmdbCrew []tmdbCrew) []apiexternal_v2.CrewMember {
	crew := make([]apiexternal_v2.CrewMember, len(tmdbCrew))
	for i, c := range tmdbCrew {
		crew[i] = apiexternal_v2.CrewMember{
			ID:          c.ID,
			Name:        c.Name,
			Job:         c.Job,
			Department:  c.Department,
			ProfilePath: c.ProfilePath,
			Gender:      c.Gender,
		}
	}

	return crew
}

func convertExternalIDs(tmdbIDs *tmdbExternalIDs, tmdbID int) *apiexternal_v2.ExternalIDs {
	return &apiexternal_v2.ExternalIDs{
		IMDbID:      tmdbIDs.IMDbID,
		TVDbID:      tmdbIDs.TVDbID,
		TMDbID:      tmdbID,
		FacebookID:  tmdbIDs.FacebookID,
		InstagramID: tmdbIDs.InstagramID,
		TwitterID:   tmdbIDs.TwitterID,
	}
}

func convertImageCollection(tmdbImages *tmdbImageCollection) *apiexternal_v2.ImageCollection {
	return &apiexternal_v2.ImageCollection{
		Backdrops: convertImages(tmdbImages.Backdrops),
		Posters:   convertImages(tmdbImages.Posters),
		Logos:     convertImages(tmdbImages.Logos),
	}
}

func convertImages(tmdbImages []tmdbImage) []apiexternal_v2.Image {
	images := make([]apiexternal_v2.Image, len(tmdbImages))
	for i, img := range tmdbImages {
		images[i] = apiexternal_v2.Image{
			FilePath:    img.FilePath,
			Width:       img.Width,
			Height:      img.Height,
			AspectRatio: img.AspectRatio,
			VoteAverage: img.VoteAverage,
			VoteCount:   img.VoteCount,
			ISO639_1:    img.ISO639_1,
		}
	}

	return images
}

func convertVideos(tmdbVideos []tmdbVideo) []apiexternal_v2.Video {
	videos := make([]apiexternal_v2.Video, len(tmdbVideos))
	for i, v := range tmdbVideos {
		videos[i] = apiexternal_v2.Video{
			ID:          v.ID,
			Key:         v.Key,
			Name:        v.Name,
			Site:        v.Site,
			Size:        v.Size,
			Type:        v.Type,
			Official:    v.Official,
			PublishedAt: parseDate(v.PublishedAt),
		}
	}

	return videos
}

//
// Helper Functions
//

func parseDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	// Try full datetime format first
	if t, err := time.Parse(time.RFC3339, dateStr); err == nil {
		return t
	}

	// Try date-only format
	if t, err := time.Parse("2006-01-02", dateStr); err == nil {
		return t
	}

	return time.Time{}
}

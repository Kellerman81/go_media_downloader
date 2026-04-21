package apiexternal_v2

import (
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

//
// Core Response Types - All Concrete, No interface{} or any
//

// MovieSearchResult represents a single movie from search results.
type MovieSearchResult struct {
	ID            int       `json:"id"`
	Title         string    `json:"title"`
	OriginalTitle string    `json:"original_title"`
	Year          int       `json:"year"`
	ReleaseDate   time.Time `json:"release_date"`
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path"`
	Overview      string    `json:"overview"`
	VoteAverage   float64   `json:"vote_average"`
	VoteCount     int       `json:"vote_count"`
	Popularity    float64   `json:"popularity"`
	Adult         bool      `json:"adult"`
	ProviderName  string    `json:"provider_name"`     // e.g., "tmdb", "omdb"
	IMDbID        string    `json:"imdb_id,omitempty"` // Optional IMDb ID (OMDB provides this directly in search results)
}

// MovieDetails represents comprehensive movie information.
type MovieDetails struct {
	ID                  int                 `json:"id"`
	IMDbID              string              `json:"imdb_id"`
	Title               string              `json:"title"`
	OriginalTitle       string              `json:"original_title"`
	OriginalLanguage    string              `json:"original_language"`
	Tagline             string              `json:"tagline"`
	Overview            string              `json:"overview"`
	Year                int                 `json:"year"`
	ReleaseDate         time.Time           `json:"release_date"`
	Runtime             int                 `json:"runtime"` // minutes
	Budget              int64               `json:"budget"`
	Revenue             int64               `json:"revenue"`
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	Popularity          float64             `json:"popularity"`
	Adult               bool                `json:"adult"`
	PosterPath          string              `json:"poster_path"`
	BackdropPath        string              `json:"backdrop_path"`
	Homepage            string              `json:"homepage"`
	Status              string              `json:"status"` // Released, Post Production, etc.
	Genres              []Genre             `json:"genres"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	SpokenLanguages     []SpokenLanguage    `json:"spoken_languages"`
	AlternativeTitles   []AlternativeTitle  `json:"alternative_titles"`
	Credits             *Credits            `json:"credits"`
	ProviderName        string              `json:"provider_name"`
}

// SeriesSearchResult represents a single TV series from search results.
type SeriesSearchResult struct {
	ID           int       `json:"id"`
	Name         string    `json:"name"`
	OriginalName string    `json:"original_name"`
	FirstAirDate time.Time `json:"first_air_date"`
	PosterPath   string    `json:"poster_path"`
	BackdropPath string    `json:"backdrop_path"`
	Overview     string    `json:"overview"`
	VoteAverage  float64   `json:"vote_average"`
	VoteCount    int       `json:"vote_count"`
	Popularity   float64   `json:"popularity"`
	ProviderName string    `json:"provider_name"`
}

// SeriesDetails represents comprehensive TV series information.
type SeriesDetails struct {
	ID                  int                 `json:"id"`
	TVDbID              int                 `json:"tvdb_id"`
	IMDbID              string              `json:"imdb_id"`
	Name                string              `json:"name"`
	OriginalName        string              `json:"original_name"`
	OriginalLanguage    string              `json:"original_language"`
	Overview            string              `json:"overview"`
	FirstAirDate        time.Time           `json:"first_air_date"`
	LastAirDate         time.Time           `json:"last_air_date"`
	Status              string              `json:"status"`
	Type                string              `json:"type"` // Scripted, Documentary, etc.
	NumberOfSeasons     int                 `json:"number_of_seasons"`
	NumberOfEpisodes    int                 `json:"number_of_episodes"`
	EpisodeRunTime      []int               `json:"episode_run_time"` // Runtime in minutes (array for varying episode lengths)
	VoteAverage         float64             `json:"vote_average"`
	VoteCount           int                 `json:"vote_count"`
	Popularity          float64             `json:"popularity"`
	PosterPath          string              `json:"poster_path"`
	BackdropPath        string              `json:"backdrop_path"`
	Homepage            string              `json:"homepage"`
	Genres              []Genre             `json:"genres"`
	Networks            []Network           `json:"networks"`
	ProductionCompanies []ProductionCompany `json:"production_companies"`
	Seasons             []Season            `json:"seasons"`
	AlternativeTitles   []AlternativeTitle  `json:"alternative_titles"`
	Credits             *Credits            `json:"credits"`
	ProviderName        string              `json:"provider_name"`
}

// Genre represents a media genre.
type Genre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ProductionCompany represents a production company.
type ProductionCompany struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// Network represents a TV network.
type Network struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	LogoPath      string `json:"logo_path"`
	OriginCountry string `json:"origin_country"`
}

// SpokenLanguage represents a spoken language.
type SpokenLanguage struct {
	ISO639_1    string `json:"iso_639_1"`
	Name        string `json:"name"`
	EnglishName string `json:"english_name"`
}

// AlternativeTitle represents an alternative title for media.
type AlternativeTitle struct {
	Title     string `json:"title"`
	ISO3166_1 string `json:"iso_3166_1"` // Country code
	Type      string `json:"type"`
}

// Season represents a TV series season.
type Season struct {
	ID           int       `json:"id"`
	SeasonNumber int       `json:"season_number"`
	Name         string    `json:"name"`
	Overview     string    `json:"overview"`
	AirDate      time.Time `json:"air_date"`
	EpisodeCount int       `json:"episode_count"`
	PosterPath   string    `json:"poster_path"`
}

// Episode represents a TV series episode.
type Episode struct {
	ID             int          `json:"id"`
	EpisodeNumber  int          `json:"episode_number"`
	SeasonNumber   int          `json:"season_number"`
	AbsoluteNumber int          `json:"absolute_number,omitempty"` // Absolute episode number (for anime and shows with continuous numbering)
	Name           string       `json:"name"`
	Overview       string       `json:"overview"`
	AirDate        time.Time    `json:"air_date"`
	Runtime        int          `json:"runtime"`
	VoteAverage    float64      `json:"vote_average"`
	VoteCount      int          `json:"vote_count"`
	StillPath      string       `json:"still_path"`
	Crew           []CrewMember `json:"crew"`
	GuestStars     []CastMember `json:"guest_stars"`
}

// Credits represents cast and crew information.
type Credits struct {
	Cast []CastMember `json:"cast"`
	Crew []CrewMember `json:"crew"`
}

// CastMember represents a cast member.
type CastMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
	ProfilePath string `json:"profile_path"`
	Gender      int    `json:"gender"` // 1 = female, 2 = male, 0 = not specified
}

// CrewMember represents a crew member.
type CrewMember struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Job         string `json:"job"`
	Department  string `json:"department"`
	ProfilePath string `json:"profile_path"`
	Gender      int    `json:"gender"`
}

// ExternalIDs represents external IDs for cross-referencing.
type ExternalIDs struct {
	IMDbID      string `json:"imdb_id"`
	TVDbID      int    `json:"tvdb_id"`
	TraktID     int    `json:"trakt_id"`
	TMDbID      int    `json:"tmdb_id"`
	FacebookID  string `json:"facebook_id"`
	InstagramID string `json:"instagram_id"`
	TwitterID   string `json:"twitter_id"`
}

// FindByIMDbResult represents the result of finding content by IMDb ID.
type FindByIMDbResult struct {
	MovieResults []MovieSearchResult  `json:"movie_results"`
	TVResults    []SeriesSearchResult `json:"tv_results"`
}

// FindByTraktIDResult represents the result of finding content by Trakt ID.
type FindByTraktIDResult struct {
	MovieResult  *MovieSearchResult  `json:"movie_result,omitempty"`  // Single movie result
	SeriesResult *SeriesSearchResult `json:"series_result,omitempty"` // Single series result
}

// PopularMoviesResponse represents a paginated list of popular movies.
type PopularMoviesResponse struct {
	Page         int                 `json:"page"`
	TotalPages   int                 `json:"total_pages"`
	TotalResults int                 `json:"total_results"`
	Results      []MovieSearchResult `json:"results"`
}

// PopularSeriesResponse represents a paginated list of popular TV series.
type PopularSeriesResponse struct {
	Page         int                  `json:"page"`
	TotalPages   int                  `json:"total_pages"`
	TotalResults int                  `json:"total_results"`
	Results      []SeriesSearchResult `json:"results"`
}

// WatchlistItem represents an item from a user's watchlist.
type WatchlistItem struct {
	ID           int       `json:"id"`
	Type         string    `json:"type"` // "movie" or "tv"
	Title        string    `json:"title"`
	Year         int       `json:"year"`
	AddedAt      time.Time `json:"added_at"`
	IMDbID       string    `json:"imdb_id"`
	TVDbID       int       `json:"tvdb_id"`
	ProviderName string    `json:"provider_name"`
}

// ImageCollection represents a collection of images.
type ImageCollection struct {
	Backdrops []Image `json:"backdrops"`
	Posters   []Image `json:"posters"`
	Logos     []Image `json:"logos"`
}

// Image represents an image resource.
type Image struct {
	FilePath    string  `json:"file_path"`
	Width       int     `json:"width"`
	Height      int     `json:"height"`
	AspectRatio float64 `json:"aspect_ratio"`
	VoteAverage float64 `json:"vote_average"`
	VoteCount   int     `json:"vote_count"`
	ISO639_1    string  `json:"iso_639_1"` // Language code
}

// Video represents a video resource (trailer, clip, etc.)
type Video struct {
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Name        string    `json:"name"`
	Site        string    `json:"site"` // YouTube, Vimeo, etc.
	Size        int       `json:"size"` // Resolution
	Type        string    `json:"type"` // Trailer, Teaser, Clip, etc.
	Official    bool      `json:"official"`
	PublishedAt time.Time `json:"published_at"`
}

// ProviderType represents the type of metadata provider.
type ProviderType string

const (
	ProviderTMDb   ProviderType = "tmdb"
	ProviderOMDb   ProviderType = "omdb"
	ProviderTVDb   ProviderType = "tvdb"
	ProviderTrakt  ProviderType = "trakt"
	ProviderTVMaze ProviderType = "tvmaze"

	// ProviderOpenLibrary and related constants are book metadata providers.
	ProviderOpenLibrary ProviderType = "openlibrary"
	ProviderGoodreads   ProviderType = "goodreads"
	ProviderGoogleBooks ProviderType = "googlebooks"

	// ProviderAudible and related constants are audiobook metadata providers.
	ProviderAudible ProviderType = "audible"
	ProviderAudnex  ProviderType = "audnex"

	// ProviderMusicBrainz and related constants are music metadata providers.
	ProviderMusicBrainz ProviderType = "musicbrainz"
	ProviderDiscogs     ProviderType = "discogs"
	ProviderSpotify     ProviderType = "spotify"
	ProviderAcoustID    ProviderType = "acoustid"
	ProviderLastFM      ProviderType = "lastfm"
	ProviderDeezer      ProviderType = "deezer"
	ProviderTheAudioDB  ProviderType = "theaudiodb"
	ProviderITunes      ProviderType = "itunes"
)

//
// Notification Types
//

// NotificationRequest represents a notification to be sent.
type NotificationRequest struct {
	Title    string               `json:"title"`
	Message  string               `json:"message"`
	Priority NotificationPriority `json:"priority"`
	Options  map[string]string    `json:"options"` // Provider-specific options
}

// NotificationResponse represents the result of sending a notification.
type NotificationResponse struct {
	Success   bool      `json:"success"`
	MessageID string    `json:"message_id"` // Provider-specific message ID
	Timestamp time.Time `json:"timestamp"`
	Provider  string    `json:"provider"`
	Error     string    `json:"error,omitempty"`
}

// NotificationPriority represents notification priority levels.
type NotificationPriority int

const (
	PriorityLowest    NotificationPriority = -2
	PriorityLow       NotificationPriority = -1
	PriorityNormal    NotificationPriority = 0
	PriorityHigh      NotificationPriority = 1
	PriorityEmergency NotificationPriority = 2
)

// NotificationProviderType represents the type of notification provider.
type NotificationProviderType string

const (
	NotificationPushover   NotificationProviderType = "pushover"
	NotificationGotify     NotificationProviderType = "gotify"
	NotificationApprise    NotificationProviderType = "apprise"
	NotificationPushbullet NotificationProviderType = "pushbullet"
	NotificationSendmail   NotificationProviderType = "sendmail"
)

//
// Download Client Types
//

// DownloadProviderType represents the type of download client.
type DownloadProviderType string

const (
	DownloadProviderQBittorrent  DownloadProviderType = "qbittorrent"
	DownloadProviderDeluge       DownloadProviderType = "deluge"
	DownloadProviderTransmission DownloadProviderType = "transmission"
	DownloadProviderRTorrent     DownloadProviderType = "rtorrent"
	DownloadProviderSABnzbd      DownloadProviderType = "sabnzbd"
	DownloadProviderNZBGet       DownloadProviderType = "nzbget"
)

// TorrentAddRequest represents a request to add a torrent.
type TorrentAddRequest struct {
	URL         string            `json:"url"`          // Magnet link or torrent URL
	TorrentData []byte            `json:"torrent_data"` // Raw torrent file data
	SavePath    string            `json:"save_path"`    // Download location
	Category    string            `json:"category"`     // Category/label
	Label       string            `json:"label"`        // Label (alias for Category for some clients)
	Tags        []string          `json:"tags"`         // Tags for organization
	Paused      bool              `json:"paused"`       // Start paused
	Priority    int               `json:"priority"`     // Priority level
	Options     map[string]string `json:"options"`      // Provider-specific options
}

// TorrentAddResponse represents the result of adding a torrent.
type TorrentAddResponse struct {
	Success  bool   `json:"success"`
	Hash     string `json:"hash"`    // Torrent hash (ID)
	Name     string `json:"name"`    // Torrent name
	Message  string `json:"message"` // Success/error message
	Provider string `json:"provider"`
	Error    string `json:"error,omitempty"`
}

// TorrentInfo represents information about a torrent.
type TorrentInfo struct {
	Hash          string    `json:"hash"`
	Name          string    `json:"name"`
	State         string    `json:"state"`          // downloading, seeding, paused, etc.
	Size          int64     `json:"size"`           // Total size in bytes
	Progress      float64   `json:"progress"`       // 0-100%
	DownloadSpeed int64     `json:"download_speed"` // Bytes per second
	UploadSpeed   int64     `json:"upload_speed"`   // Bytes per second
	Downloaded    int64     `json:"downloaded"`     // Bytes downloaded
	Uploaded      int64     `json:"uploaded"`       // Bytes uploaded
	Ratio         float64   `json:"ratio"`          // Upload/download ratio
	ETA           int       `json:"eta"`            // Estimated time remaining (seconds)
	SavePath      string    `json:"save_path"`      // Download location
	Category      string    `json:"category"`       // Category/label
	Label         string    `json:"label"`          // Label (alias for Category for some clients)
	Tags          []string  `json:"tags"`           // Tags
	Priority      int       `json:"priority"`       // Priority level
	AddedOn       time.Time `json:"added_on"`       // When added
	AddedDate     time.Time `json:"added_date"`     // When added (alias)
	CompletionOn  time.Time `json:"completion_on"`  // When completed
	Provider      string    `json:"provider"`
}

// TorrentListResponse represents a list of torrents.
type TorrentListResponse struct {
	Torrents []TorrentInfo `json:"torrents"`
	Total    int           `json:"total"`
}

// DownloadClientStatus represents the status of a download client.
type DownloadClientStatus struct {
	Connected       bool   `json:"connected"`
	Version         string `json:"version"`
	Message         string `json:"message"`        // Status message
	FreeSpace       int64  `json:"free_space"`     // Bytes
	TotalDownload   int64  `json:"total_download"` // Bytes/sec
	TotalUpload     int64  `json:"total_upload"`   // Bytes/sec
	ActiveDownloads int    `json:"active_downloads"`
	ActiveUploads   int    `json:"active_uploads"`
	QueuedDownloads int    `json:"queued_downloads"`
	Provider        string `json:"provider"`
}

//
// Indexer Types (Newznab/Torznab)
//

// IndexerProviderType represents the type of indexer.
type IndexerProviderType string

const (
	IndexerNewznab IndexerProviderType = "newznab"
	IndexerTorznab IndexerProviderType = "torznab"
)

// IndexerSearchRequest represents a search request to an indexer.
type IndexerSearchRequest struct {
	Query      string            `json:"query"`       // Search query
	IMDBID     string            `json:"imdb_id"`     // IMDB ID for movie search
	TVDBID     int               `json:"tvdb_id"`     // TVDB ID for TV search
	Season     string            `json:"season"`      // Season number
	Episode    string            `json:"episode"`     // Episode number
	Categories []string          `json:"categories"`  // Category IDs
	Limit      int               `json:"limit"`       // Max results
	Offset     int               `json:"offset"`      // Pagination offset
	MaxAge     int               `json:"max_age"`     // Max age in days
	SearchType string            `json:"search_type"` // movie, tvsearch, search
	Options    map[string]string `json:"options"`     // Provider-specific options
}

// IndexerCapabilities represents indexer capabilities.
type IndexerCapabilities struct {
	ServerTitle   string            `json:"server_title"`
	ServerVersion string            `json:"server_version"`
	SearchModes   []string          `json:"search_modes"` // movie, tvsearch, search
	Categories    []IndexerCategory `json:"categories"`
	Limits        IndexerLimits     `json:"limits"`
	Provider      string            `json:"provider"`
}

// IndexerCategory represents a category supported by the indexer.
type IndexerCategory struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Subcategories []IndexerCategory `json:"subcategories"`
}

// IndexerLimits represents rate limits and restrictions.
type IndexerLimits struct {
	MaxResults   int `json:"max_results"`
	DefaultLimit int `json:"default_limit"`
	MaxAge       int `json:"max_age"` // Max age in days
}

//
// Searcher/Download Types - Bridge types for compatibility with searcher package
//

// Nzb represents an NZB/Torrent found on an indexer (v2 compatible version).
// This mirrors the apiexternal.Nzb type for backward compatibility with searcher.
type Nzb struct {
	ID             string    `json:"id,omitempty"`
	Title          string    `json:"title,omitempty"`
	SourceEndpoint string    `json:"source_endpoint"`
	Season         string    `json:"season,omitempty"`
	Episode        string    `json:"episode,omitempty"`
	IMDBID         string    `json:"imdb,omitempty"`
	DownloadURL    string    `json:"download_url,omitempty"`
	Size           int64     `json:"size,omitempty"`
	TVDBID         int       `json:"tvdbid,omitempty"`
	IsTorrent      bool      `json:"is_torrent,omitempty"`
	PubDate        time.Time `json:"pub_date"`

	// Category is the Newznab category ID from the indexer (e.g. "3010" for MP3, "3040" for Lossless).
	// Used as a fallback to infer audio format when none is present in the release title.
	Category string `json:"category,omitempty"`

	// Indexer and Quality configs - imported from config package
	// These will be set during conversion
	Indexer *config.IndexersConfig `json:"-"` // *config.IndexersConfig
	Quality *config.QualityConfig  `json:"-"` // *config.QualityConfig
}

// Nzbwithprio represents an NZB with priority and validation information (v2 compatible version).
// This mirrors the apiexternal.Nzbwithprio type for backward compatibility with searcher.
type Nzbwithprio struct {
	NZB                 Nzb                               `json:"nzb"`
	Info                database.ParseInfo                `json:"info"`
	WantedAlternates    []syncops.DbstaticTwoStringOneInt `json:"wanted_alternates"`
	AdditionalReason    any                               `json:"additional_reason"`
	AdditionalReasonStr string                            `json:"additional_reason_str"`
	WantedTitle         string                            `json:"wanted_title"`
	SearchFor           string                            `json:"search_for"` // Complete search query (e.g., "Album Title Artist Name" for music)
	Quality             string                            `json:"quality"`
	Listname            string                            `json:"listname"`
	Reason              string                            `json:"reason"`
	AdditionalReasonInt int64                             `json:"additional_reason_int"`
	NzbmovieID          uint                              `json:"nzb_movie_id"`
	NzbepisodeID        uint                              `json:"nzb_episode_id"`
	NzbbookID           uint                              `json:"nzb_book_id"`
	NzbaudiobookID      uint                              `json:"nzb_audiobook_id"`
	NzbalbumID          uint                              `json:"nzb_album_id"`
	Dbid                uint                              `json:"dbid"`
	MinimumPriority     int                               `json:"minimum_priority"`
	DontSearch          bool                              `json:"dont_search"`
	DontUpgrade         bool                              `json:"dont_upgrade"`
	IDSearched          bool                              `json:"id_searched"`
}

//
// OAuth 2.0 Types
//

// OAuthToken represents an OAuth 2.0 access token with refresh capabilities.
type OAuthToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	Expiry       time.Time `json:"expiry"`
	Scope        string    `json:"scope,omitempty"`
}

// IsValid checks if the token is valid (non-empty access token and not expired).
func (t *OAuthToken) IsValid() bool {
	if t == nil || t.AccessToken == "" {
		return false
	}

	if t.Expiry.IsZero() {
		return true // No expiry means token doesn't expire
	}

	return time.Now().Before(t.Expiry)
}

// NeedsRefresh checks if the token will expire soon (within 5 minutes).
func (t *OAuthToken) NeedsRefresh() bool {
	if t == nil || t.Expiry.IsZero() {
		return false
	}

	return time.Now().Add(5 * time.Minute).After(t.Expiry)
}

//
// Book Types
//

// BookSearchResult represents a book search result.
type BookSearchResult struct {
	ID             string       `json:"id"`
	Title          string       `json:"title"`
	Subtitle       string       `json:"subtitle,omitempty"`
	Authors        []string     `json:"authors"`
	ISBN13         string       `json:"isbn13,omitempty"`
	ISBN10         string       `json:"isbn10,omitempty"`
	PublishDate    time.Time    `json:"publish_date"`
	PublishYear    int          `json:"publish_year,omitempty"`
	CoverURL       string       `json:"cover_url,omitempty"`
	Description    string       `json:"description,omitempty"`
	SeriesName     string       `json:"series_name,omitempty"`
	SeriesPosition string       `json:"series_position,omitempty"`
	ProviderType   ProviderType `json:"provider_type"`
}

// BookDetails represents full book metadata.
type BookDetails struct {
	ID             string    `json:"id"`
	Title          string    `json:"title"`
	Subtitle       string    `json:"subtitle,omitempty"`
	Authors        []string  `json:"authors"`
	ISBN13         string    `json:"isbn13,omitempty"`
	ISBN10         string    `json:"isbn10,omitempty"`
	PublishDate    time.Time `json:"publish_date"`
	PublishYear    int       `json:"publish_year,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	Description    string    `json:"description,omitempty"`
	SeriesName     string    `json:"series_name,omitempty"`
	SeriesPosition string    `json:"series_position,omitempty"`

	// Publishing info
	Publisher string `json:"publisher,omitempty"`
	PageCount int    `json:"page_count,omitempty"`
	Language  string `json:"language,omitempty"`

	// Classification
	Genres   []string `json:"genres,omitempty"`
	Subjects []string `json:"subjects,omitempty"`

	// Ratings
	AverageRating float64 `json:"average_rating,omitempty"`
	RatingsCount  int     `json:"ratings_count,omitempty"`

	// External IDs
	OpenLibraryID   string `json:"openlibrary_id,omitempty"`
	OpenLibraryWork string `json:"openlibrary_work,omitempty"`
	GoodreadsID     string `json:"goodreads_id,omitempty"`
	GoogleBooksID   string `json:"googlebooks_id,omitempty"`
	ASIN            string `json:"asin,omitempty"`
	LCCN            string `json:"lccn,omitempty"`
	OCLC            string `json:"oclc,omitempty"`

	// Additional metadata
	EditionCount    int      `json:"edition_count,omitempty"`
	FirstSentence   string   `json:"first_sentence,omitempty"`
	TableOfContents []string `json:"table_of_contents,omitempty"`

	ProviderType ProviderType `json:"provider_type"`
}

// AuthorSearchResult represents a minimal author result from search.
type AuthorSearchResult struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	ProviderType ProviderType `json:"provider_type"`
}

// AuthorDetails represents author metadata.
type AuthorDetails struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	SortName  string    `json:"sort_name,omitempty"`
	Bio       string    `json:"bio,omitempty"`
	BirthDate time.Time `json:"birth_date"`
	DeathDate time.Time `json:"death_date"`
	ImageURL  string    `json:"image_url,omitempty"`
	Website   string    `json:"website,omitempty"`
	Wikipedia string    `json:"wikipedia,omitempty"`

	// External IDs
	OpenLibraryID string `json:"openlibrary_id,omitempty"`
	GoodreadsID   string `json:"goodreads_id,omitempty"`
	VIAF          string `json:"viaf,omitempty"`
	ISNI          string `json:"isni,omitempty"`

	// Works
	WorkCount    int          `json:"work_count,omitempty"`
	ProviderType ProviderType `json:"provider_type"`
}

// BookSeriesDetails represents book series metadata.
type BookSeriesDetails struct {
	ID           string             `json:"id"`
	Name         string             `json:"name"`
	Description  string             `json:"description,omitempty"`
	TotalBooks   int                `json:"total_books,omitempty"`
	Books        []BookSearchResult `json:"books,omitempty"`
	ProviderType ProviderType       `json:"provider_type"`
}

//
// Audiobook Types
//

// AudiobookSearchResult represents an audiobook search result.
type AudiobookSearchResult struct {
	ID             string        `json:"id"`
	ASIN           string        `json:"asin"`
	Title          string        `json:"title"`
	Subtitle       string        `json:"subtitle,omitempty"`
	Authors        []string      `json:"authors"`
	Narrators      []string      `json:"narrators"`
	ReleaseDate    time.Time     `json:"release_date"`
	ReleaseYear    int           `json:"release_year,omitempty"`
	CoverURL       string        `json:"cover_url,omitempty"`
	Description    string        `json:"description,omitempty"`
	Series         string        `json:"series,omitempty"`
	SeriesName     string        `json:"series_name,omitempty"`
	SeriesPosition string        `json:"series_position,omitempty"`
	Duration       time.Duration `json:"duration,omitempty"`
	RuntimeMinutes int           `json:"runtime_minutes,omitempty"`
	ProviderType   ProviderType  `json:"provider_type"`
}

// AudiobookDetails represents full audiobook metadata.
type AudiobookDetails struct {
	ID             string    `json:"id"`
	ASIN           string    `json:"asin"`
	Title          string    `json:"title"`
	Subtitle       string    `json:"subtitle,omitempty"`
	Authors        []string  `json:"authors"`
	AuthorIDs      []string  `json:"author_ids,omitempty"`
	Narrators      []string  `json:"narrators"`
	NarratorIDs    []string  `json:"narrator_ids,omitempty"`
	ReleaseDate    time.Time `json:"release_date"`
	ReleaseYear    int       `json:"release_year,omitempty"`
	CoverURL       string    `json:"cover_url,omitempty"`
	Description    string    `json:"description,omitempty"`
	Summary        string    `json:"summary,omitempty"`
	Series         string    `json:"series,omitempty"`
	SeriesASIN     string    `json:"series_asin,omitempty"`
	SeriesName     string    `json:"series_name,omitempty"`
	SeriesPosition string    `json:"series_position,omitempty"`

	// Audio info
	Duration       time.Duration `json:"duration,omitempty"`
	RuntimeMinutes int           `json:"runtime_minutes,omitempty"`
	FormatType     string        `json:"format_type,omitempty"`

	// Publisher info
	Publisher     string `json:"publisher,omitempty"`
	PublisherName string `json:"publisher_name,omitempty"`
	Language      string `json:"language,omitempty"`

	// Classification
	Genres     []string `json:"genres,omitempty"`
	Categories []string `json:"categories,omitempty"`

	// Ratings
	Rating        float64 `json:"rating,omitempty"`
	AverageRating float64 `json:"average_rating,omitempty"`
	RatingsCount  int     `json:"ratings_count,omitempty"`
	RatingCount   int     `json:"rating_count,omitempty"`

	// Chapters
	Chapters []AudiobookChapter `json:"chapters,omitempty"`

	// External IDs
	ISBN        string `json:"isbn,omitempty"`
	GoodreadsID string `json:"goodreads_id,omitempty"`

	// Flags
	IsAdult bool `json:"is_adult,omitempty"`

	ProviderType ProviderType `json:"provider_type"`
}

// AudiobookChapter represents a chapter in an audiobook.
type AudiobookChapter struct {
	Number        int           `json:"number"`
	ChapterNumber int           `json:"chapter_number"`
	Title         string        `json:"title"`
	StartOffset   time.Duration `json:"start_offset"`
	StartOffsetMs int64         `json:"start_offset_ms"`
	LengthMs      int64         `json:"length_ms"`
	Duration      time.Duration `json:"duration,omitempty"`
}

//
// Music Types
//

// ArtistSearchResult represents a music artist search result.
type ArtistSearchResult struct {
	ID             string       `json:"id"`
	MusicBrainzID  string       `json:"musicbrainz_id,omitempty"`
	DiscogsID      int          `json:"discogs_id,omitempty"`
	Name           string       `json:"name"`
	SortName       string       `json:"sort_name,omitempty"`
	Disambiguation string       `json:"disambiguation,omitempty"`
	Country        string       `json:"country,omitempty"`
	Area           string       `json:"area,omitempty"`
	Type           string       `json:"type,omitempty"` // Person, Group, Orchestra, etc.
	BeginYear      int          `json:"begin_year,omitempty"`
	EndYear        int          `json:"end_year,omitempty"`
	ImageURL       string       `json:"image_url,omitempty"`
	ProviderType   ProviderType `json:"provider_type"`
}

// ArtistDetails represents full music artist metadata.
type ArtistDetails struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	RealName       string    `json:"real_name,omitempty"`
	SortName       string    `json:"sort_name,omitempty"`
	Disambiguation string    `json:"disambiguation,omitempty"`
	Type           string    `json:"type,omitempty"`
	Country        string    `json:"country,omitempty"`
	BeginDate      time.Time `json:"begin_date"`
	EndDate        time.Time `json:"end_date"`
	Ended          bool      `json:"ended,omitempty"`
	IsEnded        bool      `json:"is_ended,omitempty"`
	Gender         string    `json:"gender,omitempty"`
	Area           string    `json:"area,omitempty"`
	Bio            string    `json:"bio,omitempty"`
	ImageURL       string    `json:"image_url,omitempty"`

	// Related entities
	Aliases []string `json:"aliases,omitempty"`
	Genres  []string `json:"genres,omitempty"`
	Members []string `json:"members,omitempty"`
	Groups  []string `json:"groups,omitempty"`
	Website string   `json:"website,omitempty"`

	// External IDs
	MusicBrainzID string `json:"musicbrainz_id,omitempty"`
	DiscogsID     string `json:"discogs_id,omitempty"`
	WikidataID    string `json:"wikidata_id,omitempty"`

	ProviderType ProviderType `json:"provider_type"`
}

// ArtistRef pairs an artist name with their provider ID so the two can never
// become out of sync when stored in a slice.
type ArtistRef struct {
	Name string `json:"name"`
	ID   string `json:"id,omitempty"`
}

// ReleaseSearchResult represents a music release/album search result.
type ReleaseSearchResult struct {
	ID             string       `json:"id"`
	MusicBrainzID  string       `json:"musicbrainz_id,omitempty"`
	DiscogsID      int          `json:"discogs_id,omitempty"`
	DeezerID       int          `json:"deezer_id,omitempty"`
	TheAudioDBID   string       `json:"theaudiodb_id,omitempty"`
	ITunesID       int          `json:"itunes_id,omitempty"`
	MasterID       int          `json:"master_id,omitempty"`
	ReleaseGroupID string       `json:"release_group_id,omitempty"`
	Title          string       `json:"title"`
	Artists        []ArtistRef  `json:"artists"`
	ReleaseDate    time.Time    `json:"release_date"`
	ReleaseYear    int          `json:"release_year,omitempty"`
	Country        string       `json:"country,omitempty"`
	Status         string       `json:"status,omitempty"` // Official, Bootleg, Promotion
	Type           string       `json:"type,omitempty"`   // Album, Single, EP, Compilation
	CoverURL       string       `json:"cover_url,omitempty"`
	TrackCount     int          `json:"track_count,omitempty"`
	Label          string       `json:"label,omitempty"`
	CatalogNumber  string       `json:"catalog_number,omitempty"`
	Format         string       `json:"format,omitempty"`
	MediaFormats   []string     `json:"media_formats,omitempty"` // Format of every disc (e.g. ["CD","CD"] for 2xCD)
	Genres         []string     `json:"genres,omitempty"`
	Barcode        string       `json:"barcode,omitempty"`
	ProviderType   ProviderType `json:"provider_type"`
}

// ReleaseDetails represents full music release/album metadata.
type ReleaseDetails struct {
	ID          string    `json:"id"`
	Title       string      `json:"title"`
	Artists     []ArtistRef `json:"artists"`
	ReleaseDate time.Time   `json:"release_date"`
	ReleaseYear int       `json:"release_year,omitempty"`
	Country     string    `json:"country,omitempty"`
	Status      string    `json:"status,omitempty"`
	Type        string    `json:"type,omitempty"` // Album, Single, EP, Compilation
	CoverURL    string    `json:"cover_url,omitempty"`
	Barcode     string    `json:"barcode,omitempty"`
	ASIN        string    `json:"asin,omitempty"`
	Notes       string    `json:"notes,omitempty"`
	Language    string    `json:"language,omitempty"`

	// Label info
	Label         string `json:"label,omitempty"`
	LabelID       string `json:"label_id,omitempty"`
	CatalogNum    string `json:"catalog_num,omitempty"`
	CatalogNumber string `json:"catalog_number,omitempty"`

	// Tracks
	TrackCount int     `json:"track_count,omitempty"`
	Tracks     []Track `json:"tracks,omitempty"`

	// Media info
	Format    string `json:"format,omitempty"` // CD, Vinyl, Digital, etc.
	DiscCount int    `json:"disc_count,omitempty"`

	// Genres
	Genres         []string `json:"genres,omitempty"`
	Styles         []string `json:"styles,omitempty"`
	SecondaryTypes []string `json:"secondary_types,omitempty"`

	// Ratings
	Rating      float64 `json:"rating,omitempty"`
	RatingCount int     `json:"rating_count,omitempty"`

	// External IDs
	MusicBrainzID      string `json:"musicbrainz_id,omitempty"`
	MusicBrainzGroupID string `json:"musicbrainz_group_id,omitempty"`
	ReleaseGroupID     string `json:"release_group_id,omitempty"`
	DiscogsID          string `json:"discogs_id,omitempty"`
	DeezerID           string `json:"deezer_id,omitempty"`
	TheAudioDBID       string `json:"theaudiodb_id,omitempty"`
	ITunesID           string `json:"itunes_id,omitempty"`
	SpotifyID          string `json:"spotify_id,omitempty"`
	MasterID           int    `json:"master_id,omitempty"`
	MainReleaseID      int    `json:"main_release_id,omitempty"`

	ProviderType ProviderType `json:"provider_type"`
}

// Track represents a music track/recording.
type Track struct {
	ID           string        `json:"id"`
	Title        string        `json:"title"`
	Position     int           `json:"position"`
	TrackNumber  int           `json:"track_number,omitempty"`
	DiscNumber   int           `json:"disc_number,omitempty"`
	DurationMs   int           `json:"duration_ms,omitempty"`
	Duration     time.Duration `json:"duration,omitempty"`
	ISRC         string        `json:"isrc,omitempty"`
	ArtistCredit string        `json:"artist_credit,omitempty"`
	Artists      []ArtistRef   `json:"artists,omitempty"`
	ReleaseYear  int           `json:"release_year,omitempty"`
	Genres       []string      `json:"genres,omitempty"`

	// External IDs
	MusicBrainzID string `json:"musicbrainz_id,omitempty"`
	AcoustID      string `json:"acoustid,omitempty"`
}

// TrackSearchResult represents a minimal track result from search.
type TrackSearchResult struct {
	ID           string       `json:"id"`
	Title        string       `json:"title"`
	Artists      []string     `json:"artists"`
	Album        string       `json:"album,omitempty"`
	TrackNumber  int          `json:"track_number,omitempty"`
	DiscNumber   int          `json:"disc_number,omitempty"`
	DurationMs   int          `json:"duration_ms,omitempty"`
	ProviderType ProviderType `json:"provider_type"`
}

// TrackDetails represents full track metadata.
type TrackDetails struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Artists     []string  `json:"artists"`
	ArtistIDs   []string  `json:"artist_ids,omitempty"`
	Album       string    `json:"album,omitempty"`
	AlbumID     string    `json:"album_id,omitempty"`
	TrackNumber int       `json:"track_number,omitempty"`
	DiscNumber  int       `json:"disc_number,omitempty"`
	DurationMs  int       `json:"duration_ms,omitempty"`
	ReleaseDate time.Time `json:"release_date"`
	ReleaseYear int       `json:"release_year,omitempty"`
	CoverURL    string    `json:"cover_url,omitempty"`
	Explicit    bool      `json:"explicit,omitempty"`
	ISRC        string    `json:"isrc,omitempty"`
	Genres      []string  `json:"genres,omitempty"`

	// External IDs
	MusicBrainzID string `json:"musicbrainz_id,omitempty"`
	SpotifyID     string `json:"spotify_id,omitempty"`

	ProviderType ProviderType `json:"provider_type"`
}

// RecordingMatch represents a match from audio fingerprinting.
type RecordingMatch struct {
	// Match info
	AcoustID      string       `json:"acoustid,omitempty"`
	Score         float64      `json:"score"`
	Sources       int          `json:"sources,omitempty"`
	MusicBrainzID string       `json:"musicbrainz_id,omitempty"`
	ProviderType  ProviderType `json:"provider_type"`

	// Track info
	RecordingID    string        `json:"recording_id"`
	RecordingTitle string        `json:"recording_title"`
	Title          string        `json:"title,omitempty"`
	Duration       time.Duration `json:"duration,omitempty"`
	DurationMs     int           `json:"duration_ms,omitempty"`
	ISRC           string        `json:"isrc,omitempty"`

	// Artist info
	ArtistName string   `json:"artist_name"`
	ArtistID   string   `json:"artist_id,omitempty"`
	Artists    []string `json:"artists,omitempty"`
	ArtistIDs  []string `json:"artist_ids,omitempty"`

	// Album/Release info
	Album        string    `json:"album,omitempty"`
	AlbumID      string    `json:"album_id,omitempty"`
	AlbumType    string    `json:"album_type,omitempty"`
	AlbumArtists []string  `json:"album_artists,omitempty"`
	ReleaseTitle string    `json:"release_title,omitempty"`
	ReleaseID    string    `json:"release_id,omitempty"`
	ReleaseDate  time.Time `json:"release_date"`
	ReleaseYear  int       `json:"release_year,omitempty"`
	Country      string    `json:"country,omitempty"`

	// Track position
	TrackNumber int `json:"track_number,omitempty"`
	TotalTracks int `json:"total_tracks,omitempty"`
	DiscNumber  int `json:"disc_number,omitempty"`
	TotalDiscs  int `json:"total_discs,omitempty"`
}

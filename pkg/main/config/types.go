//gofumpt:off
package config

import (
	"context"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Series Config

type MainManualConfig struct {
	// Serie is a slice of SerieConfig structs that defines the series configurations
	Config []ManualConfig `comment:"Define Manual configurations for monitoring and downloading shows" displayname:"Manual Configurations" longcomment:"Array of TV series configurations for your media library.\nEach entry in this array defines a complete series configuration with:\n- Primary series name and TVDB ID for identification\n- Alternate names for improved release matching\n- Search and upgrade behavior settings\n- Episode identification format and runtime checking\n- Metadata sources and custom storage paths\nAdd one entry per TV series you want to monitor and download.\nExample: Add 'Breaking Bad', 'Game of Thrones', etc. as separate entries" toml:"config"`
}

// ManualConfig defines the configuration for a TV series.
type ManualConfig struct {
	// Name is the primary name for the series
	Name string `comment:"Primary name for the TV series" displayname:"Primary Series Name" longcomment:"Enter the primary name for this TV series.This should be the most commonly used title for the show.Example: 'Breaking Bad' or 'Game of Thrones'" toml:"name"`

	// TvdbID is the tvdbid for the series for better searches
	TvdbID int `comment:"Numeric TVDB ID for improved search accuracy" displayname:"TVDB Database ID" longcomment:"Enter the numeric TVDB (TheTVDB.com) ID for this series.This improves search accuracy and metadata retrieval.Find the ID by searching for your show on thetvdb.com and copying the number from the URL.Example: For 'Breaking Bad' use 81189" toml:"tvdb_id"`

	// AlternateName specifies alternate names which the series is known for
	// Alternates from tvdb and trakt are added
	AlternateName []string `comment:"Alternate titles for better release matching" displayname:"Alternate Series Names" longcomment:"Enter alternate titles that this series is known by.Include foreign language titles, abbreviations, or common variations.This helps find releases with different naming conventions.Separate multiple names with commas in the array format.Example: ['BB', 'Breaking Bad US', 'Во все тяжкие']Note: Alternates from TVDB and Trakt are automatically added." toml:"alternatename"`

	// DisallowedName specifies names which the series is not allowed to have
	// These are removed from Alternates from tvdb and trakt
	DisallowedName []string `comment:"Enter titles that should NOT be associated with this series" displayname:"Disallowed Series Names" longcomment:"Enter titles that should NOT be associated with this series.This prevents incorrect matches from releases with similar names.Useful when TVDB/Trakt automatically adds confusing alternate names.Separate multiple names with commas in the array format.Example: ['Different Show', 'Breaking Bad Documentary']These names will be excluded from automatic alternates." toml:"disallowedname"`

	// Identifiedby specifies how the media is structured, e.g. ep=S01E01, date=yy-mm-dd
	Identifiedby string `comment:"Specify how episodes are identified and organized.Choose the format that matches how your files are named" displayname:"Episode Identification Format" longcomment:"Specify how episodes are identified and organized.Choose the format that matches how your files are named:- 'ep' for standard episode numbering (S01E01, S02E03, etc.)- 'date' for date-based shows (YYYY-MM-DD format)Most TV series use 'ep', while daily shows use 'date'.Example: 'ep' for most shows, 'date' for news/talk shows" toml:"identifiedby"`

	// DontUpgrade indicates whether to skip searches for better versions of media
	DontUpgrade bool `comment:"Set to true to disable quality upgrade searches for this series.When false, the system will search for upgrades" displayname:"Disable Quality Upgrades" longcomment:"Set to true to disable quality upgrade searches for this series.When false, the system will automatically search for better quality versionsof episodes you already have (e.g., upgrading from 720p to 1080p).Set to true if you're satisfied with current quality and want to save resources.Default: false (upgrades enabled)" toml:"dont_upgrade"`

	// DontSearch indicates whether the series should not be searched
	DontSearch bool `comment:"Set to true to completely disable all searches for this series.This stops both missing episode and upgrade searches" displayname:"Disable All Searches" longcomment:"Set to true to completely disable all searches for this series.This stops both missing episode searches and quality upgrades.Useful for series that are discontinued, completed, or manually managed.When true, the series will remain in your library but won't be searched.Default: false (searches enabled)" toml:"dont_search"`

	// SearchSpecials indicates whether to also search Season 0 (specials)
	SearchSpecials bool `comment:"Set to true to include Season 0 (specials) in searches.Season 0 typically contains extras and specials" displayname:"Search Season Zero Specials" longcomment:"Set to true to include Season 0 (specials) in searches.Season 0 typically contains extras like behind-the-scenes content,deleted scenes, webisodes, or special episodes that don't fit regular seasons.Note: Specials may have inconsistent naming and lower availability.Default: false (specials not searched)" toml:"search_specials"`

	// IgnoreRuntime indicates whether to ignore episode runtime checks
	IgnoreRuntime bool `comment:"Set to true to skip runtime validation for this series.When false, downloaded episodes are checked for completeness" displayname:"Skip Runtime Validation" longcomment:"Set to true to skip runtime validation for this series.When false, downloaded episodes are checked against expected runtimeto ensure they're complete and not fake/incomplete files.Set to true for shows with highly variable episode lengthsor when runtime checking causes issues with legitimate files.Default: false (runtime checking enabled)" toml:"ignore_runtime"`

	// Source specifies the metadata source, e.g. none, tvdb, or scraper
	Source string `comment:"Specify the metadata source for this series information.Available options:- 'tvdb' to use TheTVDB.com for episode data- 'scraper' to use external content scrapers- 'none' to disable metadata" displayname:"Metadata Source Provider" longcomment:"Specify the metadata source for this series information.Available options:- 'tvdb' to use TheTVDB.com for episode information and metadata- 'scraper' to use external content scrapers configured in the scrapers section- 'none' to disable automatic metadata fetchingWhen using 'scraper', the series name must match the serie_name configured in at least one scraper entry.Scrapers automatically fetch and create episodes from external APIs.Default: 'tvdb' (recommended for most series)" toml:"source"`

	// Target defines a specific path to use for the media
	// This path must also be in the media data section
	Target string `comment:"Optionally specify a custom path where this series should be stored.This overrides the default path settings" displayname:"Custom Storage Path" longcomment:"Optionally specify a custom path where this series should be stored.This overrides the default path settings for this specific series.The path must be an absolute path and must also be configuredin the paths section of your configuration.Leave empty to use default path settings.Example: '/media/tv/special-shows/SeriesName'Note: The path must exist and be accessible to the application." toml:"target"`

	// --- Scraper Configuration (when source = "scraper") ---

	// ScraperType is the type of scraper to use (project1service, algolia, htmlxpath, csrfapi)
	ScraperType string `comment:"Type of scraper to use when source='scraper'.Options: 'project1service', 'algolia', 'htmlxpath', 'csrfapi'" displayname:"Scraper Type" longcomment:"Type of scraper to use when source='scraper'.Available options:- 'project1service': Project1Service API scraper- 'algolia': Algolia search API scraper- 'htmlxpath': HTML/XPath scraper for static pages- 'csrfapi': CSRF-protected JSON API scraperOnly used when source='scraper'.Example: 'algolia' or 'htmlxpath'" toml:"scraper_type"`

	// StartURL is the starting URL for the scraper
	StartURL string `comment:"Starting URL for the scraper.Used to obtain authentication tokens" displayname:"Scraper Start URL" longcomment:"Starting URL for the scraper.Used to obtain authentication tokens and as base for API calls.For project1service: URL to fetch instance_token cookie from.For algolia: URL to extract Algolia credentials from HTML.For htmlxpath: First page URL to start scraping.For csrfapi: URL to extract CSRF token from.Example: 'https://example.com/videos/'" toml:"start_url"`

	// SiteURL is the base URL for the site
	SiteURL string `comment:"Base URL for the site.Used for constructing full URLs" displayname:"Site Base URL" longcomment:"Base URL for the site.Used for constructing full URLs from relative paths.Required for some scrapers, optional for others.Example: 'https://example.com'" toml:"site_url"`

	// SiteID is the numeric ID for the site
	SiteID uint `comment:"Numeric ID for the site.Used for logging and reference" displayname:"Site ID" longcomment:"Numeric ID for the site.Optional field used primarily for logging and reference.Can match external site IDs if applicable.Example: 123 or 0" toml:"site_id"`

	// FilterCollectionID is the collection ID filter (Project1Service only)
	FilterCollectionID int `comment:"Filter by collection ID.Only for Project1Service scrapers" displayname:"Collection ID Filter" longcomment:"Filter releases by collection ID.Only for Project1Service scrapers, set to 0 to disable filtering.To find the collection ID: Navigate to the site, filter by your desired site/collection, then copy the numeric ID from the URL (e.g., if URL is '.../collection/123', use 123).Example: 123 or 0" toml:"filter_collection_id"`

	// SiteFilterName is the site name filter (Algolia only)
	SiteFilterName string `comment:"Filter Algolia results by site name.Only for Algolia scrapers" displayname:"Site Name Filter" longcomment:"Filter Algolia results by site name.Only for Algolia scrapers, leave empty to disable filtering.Example: 'examplesite'" toml:"site_filter_name"`

	// SerieFilterName is the series name filter (Algolia only)
	SerieFilterName string `comment:"Filter Algolia results by series name.Only for Algolia scrapers" displayname:"Series Name Filter" longcomment:"Filter Algolia results by series name.Only for Algolia scrapers, leave empty to disable filtering.Example: 'SeriesName'" toml:"serie_filter_name"`

	// NetworkFilterName is the network name filter (Algolia only)
	NetworkFilterName string `comment:"Filter Algolia results by network name.Only for Algolia scrapers" displayname:"Network Name Filter" longcomment:"Filter Algolia results by network name.Only for Algolia scrapers, leave empty to disable filtering.Example: 'Network Name'" toml:"network_filter_name"`

	// NetworkSiteFilterName is the network site filter (Algolia only)
	NetworkSiteFilterName string `comment:"Filter Algolia results by network and site.Format: 'network,site'" displayname:"Network Site Filter" longcomment:"Filter Algolia results by network and site combination.Only for Algolia scrapers, format: 'network,site' (comma-separated).Example: 'Network Name,Site Name'" toml:"network_site_filter_name"`

	// --- HTML/XPath Scraper Settings ---

	// SceneNodeXPath is the XPath to select scene containers (htmlxpath only)
	SceneNodeXPath string `comment:"XPath selector for scene containers. Required for htmlxpath scraper" displayname:"Scene Node XPath" longcomment:"XPath expression to select each scene/video container element. Required for htmlxpath scraper type. Example: '//div[@class=\"photo-thumb video-thumb\"]'" toml:"scene_node_xpath"`

	// TitleXPath is the XPath for title extraction (htmlxpath only)
	TitleXPath string `comment:"XPath selector for title extraction. Required for htmlxpath scraper" displayname:"Title XPath" longcomment:"XPath expression relative to scene node for extracting title. Required for htmlxpath scraper type. Example: './/div[@class=\"caption-header\"]/span/a'" toml:"title_xpath"`

	// URLXPath is the XPath for URL extraction (htmlxpath only)
	URLXPath string `comment:"XPath selector for URL extraction. Required for htmlxpath scraper" displayname:"URL XPath" longcomment:"XPath expression relative to scene node for extracting URL. Required for htmlxpath scraper type. Example: './/a[@class=\"scene-link\"]'" toml:"url_xpath"`

	// DateXPath is the XPath for date extraction (htmlxpath only)
	DateXPath string `comment:"XPath selector for date extraction. Required for htmlxpath scraper" displayname:"Date XPath" longcomment:"XPath expression relative to scene node for extracting date. Required for htmlxpath scraper type. Example: './/span[@class=\"date\"]'" toml:"date_xpath"`

	// ActorsXPath is the XPath for actors extraction (htmlxpath only)
	ActorsXPath string `comment:"XPath selector for actors extraction. Optional for htmlxpath scraper" displayname:"Actors XPath" longcomment:"XPath expression relative to scene node for extracting actors. Optional for htmlxpath scraper type. Example: './/div[@class=\"models\"]/a'" toml:"actors_xpath"`

	// TitleAttribute is the HTML attribute to extract title from (htmlxpath only)
	TitleAttribute string `comment:"HTML attribute to extract title from.Optional, defaults to text content" displayname:"Title Attribute" longcomment:"HTML attribute to extract title from instead of text content.Optional for htmlxpath scraper, leave empty for text extraction.Example: 'title' or 'data-title'" toml:"title_attribute"`

	// URLAttribute is the HTML attribute to extract URL from (htmlxpath only)
	URLAttribute string `comment:"HTML attribute to extract URL from.Optional, defaults to 'href'" displayname:"URL Attribute" longcomment:"HTML attribute to extract URL from.Optional for htmlxpath scraper, defaults to 'href'.Example: 'href' or 'data-url'" toml:"url_attribute"`

	// PaginationType specifies pagination style (htmlxpath only)
	PaginationType string `comment:"Pagination style: 'sequential' or 'offset'.Optional for htmlxpath scraper" displayname:"Pagination Type" longcomment:"Pagination style for htmlxpath scraper.Options:- 'sequential': Pages numbered 1, 2, 3...- 'offset': Pages numbered 0, 12, 24...Default: 'sequential'" toml:"pagination_type"`

	// PageIncrement is the increment for offset pagination (htmlxpath only)
	PageIncrement int `comment:"Increment for offset pagination.Only for htmlxpath with offset pagination" displayname:"Page Increment" longcomment:"Increment for offset pagination.Only used for htmlxpath scraper with pagination_type='offset'.Example: 12 for pages 0, 12, 24..." toml:"page_increment"`

	// PageURLPattern is the URL pattern with {page} placeholder (htmlxpath only)
	PageURLPattern string `comment:"URL pattern with {page} placeholder.Required for htmlxpath multi-page scraping" displayname:"Page URL Pattern" longcomment:"URL pattern with {page} placeholder for pagination.Required for htmlxpath scraper with multiple pages.The {page} placeholder will be replaced with page number/offset.Example: 'https://example.com/videos/{page}'" toml:"page_url_pattern"`

	// --- CSRF API Scraper Settings ---

	// CSRFCookieName is the name of the cookie containing CSRF token (csrfapi only)
	CSRFCookieName string `comment:"Name of cookie containing CSRF token.Required for csrfapi scraper" displayname:"CSRF Cookie Name" longcomment:"Name of the cookie containing CSRF token.Required for csrfapi scraper type.Example: '_csrf' or 'csrf_token'" toml:"csrf_cookie_name"`

	// CSRFHeaderName is the name of the header to send CSRF token in (csrfapi only)
	CSRFHeaderName string `comment:"Name of header for CSRF token.Required for csrfapi scraper" displayname:"CSRF Header Name" longcomment:"Name of the header to send CSRF token in.Required for csrfapi scraper type.Example: 'csrf-token' or 'X-CSRF-Token'" toml:"csrf_header_name"`

	// APIURLPattern is the API URL pattern with {page} placeholder (csrfapi only)
	APIURLPattern string `comment:"API URL pattern with {page} placeholder.Required for csrfapi scraper" displayname:"API URL Pattern" longcomment:"API URL pattern with {page} placeholder.Required for csrfapi scraper type.Example: 'https://example.com/api/scenes?page={page}'" toml:"api_url_pattern"`

	// PageStartIndex is the starting page index (csrfapi only)
	PageStartIndex int `comment:"Starting page index (0 or 1).Optional for csrfapi scraper" displayname:"Page Start Index" longcomment:"Starting page index for pagination.Optional for csrfapi scraper, defaults to 1.Example: 1 for 1-based, 0 for 0-based" toml:"page_start_index"`

	// ResultsArrayPath is the JSON path to results array (csrfapi only)
	ResultsArrayPath string `comment:"JSON path to results array.Required for csrfapi scraper" displayname:"Results Array Path" longcomment:"JSON path to the array of results in API response.Required for csrfapi scraper type.Example: 'galleries' or 'data.scenes'" toml:"results_array_path"`

	// TitleField is the JSON field for title (csrfapi only)
	TitleField string `comment:"JSON field for title.Required for csrfapi scraper" displayname:"Title Field" longcomment:"JSON field name for title in API response.Required for csrfapi scraper type.Example: 'name' or 'title'" toml:"title_field"`

	// DateField is the JSON field for date (csrfapi only)
	DateField string `comment:"JSON field for date.Required for csrfapi scraper" displayname:"Date Field" longcomment:"JSON field name for date in API response.Required for csrfapi scraper type.Example: 'publishedAt' or 'release_date'" toml:"date_field"`

	// URLField is the JSON field for URL (csrfapi only)
	URLField string `comment:"JSON field for URL.Required for csrfapi scraper" displayname:"URL Field" longcomment:"JSON field name for URL in API response.Required for csrfapi scraper type.Example: 'path' or 'url'" toml:"url_field"`

	// ActorsField is the JSON field for actors array (csrfapi only)
	ActorsField string `comment:"JSON field for actors array.Optional for csrfapi scraper" displayname:"Actors Field" longcomment:"JSON field name for actors array in API response.Optional for csrfapi scraper type.Example: 'models' or 'performers'" toml:"actors_field"`

	// ActorNameField is the JSON field for actor name (csrfapi only)
	ActorNameField string `comment:"JSON field for actor name.Optional for csrfapi scraper" displayname:"Actor Name Field" longcomment:"JSON field name for actor name within actors array.Optional for csrfapi scraper type.Example: 'name' or 'performer_name'" toml:"actor_name_field"`

	// RuntimeField is the JSON field for runtime (csrfapi only)
	RuntimeField string `comment:"JSON field for runtime.Optional for csrfapi scraper" displayname:"Runtime Field" longcomment:"JSON field name for runtime in API response.Optional for csrfapi scraper, used for filtering.Example: 'runtime' or 'duration'" toml:"runtime_field"`

	// --- Common Scraper Settings ---

	// DateFormat is the date format string (Go time format)
	DateFormat string `comment:"Date format string (Go time format).Leave empty for ISO 8601" displayname:"Date Format" longcomment:"Date format string in Go time format.Leave empty for automatic ISO 8601 parsing.Examples:- '2006-01-02' for YYYY-MM-DD- 'Jan 2, 2006' for 'May 15, 2024'- '' (empty) for ISO 8601" toml:"date_format"`

	// WaitSeconds is the seconds to wait between requests
	WaitSeconds int `comment:"Seconds to wait between requests.Default: 2" displayname:"Wait Seconds" longcomment:"Seconds to wait between requests to avoid rate limiting.Default: 2 seconds.Example: 2 for normal sites, 15 for rate-limited sites" toml:"wait_seconds"`

	// --- Audiobook Configuration (for audiobook media type) ---

	// AuthorName is the author name to monitor for audiobooks
	AuthorName string `comment:"Author name to monitor for audiobooks.All books by this author will be added" displayname:"Author Name" longcomment:"Author name to monitor for audiobooks.When set, all audiobooks by this author will be automatically added to your library.Leave empty if monitoring a specific book or book series instead.Example: 'Dan Brown' or 'Stephen King'" toml:"author_name"`

	// AuthorID is the external ID for the author (Audible, Goodreads, etc.)
	AuthorID string `comment:"External ID for the author (Audible, Goodreads, etc.)" displayname:"Author ID" longcomment:"External ID for the author from metadata sources.Use the Audible author ID, Goodreads author ID, or similar.Improves accuracy when searching for author's works.Example: 'B000AP9A6K' for Audible or '630' for Goodreads" toml:"author_id"`

	// BookSeriesName is the book series name to monitor
	BookSeriesName string `comment:"Book series name to monitor.All books in this series will be added" displayname:"Book Series Name" longcomment:"Book series name to monitor for audiobooks.When set, all audiobooks in this series will be automatically added.Example: 'Harry Potter' or 'The Lord of the Rings'" toml:"book_series_name"`

	// BookSeriesID is the external ID for the book series
	BookSeriesID string `comment:"External ID for the book series" displayname:"Book Series ID" longcomment:"External ID for the book series from metadata sources.Use Audible series ID, Goodreads series ID, or similar.Example: 'B0182NWM9I' for Audible" toml:"book_series_id"`

	// --- Music Configuration (for music/album media type) ---

	// ArtistName is the artist name to monitor for music
	ArtistName string `comment:"Artist name to monitor for music.All albums by this artist will be added" displayname:"Artist Name" longcomment:"Artist name to monitor for music.When set, all albums by this artist will be automatically added to your library.Leave empty if monitoring a specific album series instead.Example: 'The Beatles' or 'Pink Floyd'" toml:"artist_name"`

	// ArtistID is the external ID for the artist (MusicBrainz, Discogs, Spotify, etc.)
	ArtistID string `comment:"External ID for the artist (MusicBrainz, Discogs, Spotify)" displayname:"Artist ID" longcomment:"External ID for the artist from metadata sources.Use MusicBrainz artist ID, Discogs artist ID, or Spotify ID.Improves accuracy when searching for artist's discography.Example: 'b10bbbfc-cf9e-42e0-be17-e2c3e1d2600d' for MusicBrainz" toml:"artist_id"`

	// AlbumSeriesName is the album/compilation series name to monitor (e.g., "Bravo Hits")
	AlbumSeriesName string `comment:"Album/compilation series name to monitor.All releases in this series will be added" displayname:"Album Series Name" longcomment:"Album or compilation series name to monitor.When set, all releases in this series will be automatically added.Useful for compilation series like 'Bravo Hits', 'Now That's What I Call Music', etc.Example: 'Bravo Hits' or 'Ministry of Sound'" toml:"album_series_name"`

	// AlbumSeriesID is the external ID for the album series
	AlbumSeriesID string `comment:"External ID for the album series" displayname:"Album Series ID" longcomment:"External ID for the album series from metadata sources.Use MusicBrainz series ID or similar identifier.Example: 'a1b2c3d4-e5f6-7890-abcd-ef1234567890'" toml:"album_series_id"`

	// MBMediaFormats restricts imports to releases whose every disc matches one of these formats.
	// Uses MusicBrainz medium format names (e.g., "CD", "Vinyl", "Digital Media").
	// Empty list = accept all formats.
	// Example: ["CD"] accepts 1xCD, 2xCD, 3xCD, etc. but rejects Vinyl or Digital-only releases.
	MBMediaFormats []string `comment:"Restrict to releases where all discs match these formats (e.g. ['CD']).Empty = accept all." displayname:"MB Media Formats" longcomment:"Filter imports to releases where every disc matches one of these MusicBrainz medium format names.\nCommon values: 'CD', 'Vinyl', 'Digital Media', 'Cassette', 'Blu-ray'.\nA 2xCD release passes if all 2 discs are CD format.\nEmpty list accepts all formats.\nExample: ['CD'] - only import CD releases (1xCD, 2xCD, 3xCD, ...)" toml:"mb_media_formats"`

	// AllowAllFormatsWhenStructuring disables the MBMediaFormats filter when structuring
	// (organizing already-downloaded files). Set to true to allow Vinyl, Cassette, or other
	// non-standard formats to be matched and imported during structuring, even when MBMediaFormats
	// restricts discovery to CD / Digital Media only.
	AllowAllFormatsWhenStructuring bool `comment:"Skip the MBMediaFormats filter when structuring already-downloaded files. Allows Vinyl and other formats to be imported during structuring." displayname:"Allow All Formats When Structuring" longcomment:"When true, the MBMediaFormats format filter is bypassed during structuring (organizing already-downloaded files).\nThis allows Vinyl, Cassette, and other non-standard formats to be matched and imported from MusicBrainz when structuring, even if MBMediaFormats restricts discovery to CD / Digital Media only.\nUseful when you have downloaded Vinyl rips that need to be organized." toml:"allow_all_formats_when_structuring"`
}

// MainConfig defines the overall configuration.
// It contains fields for each configuration section.
type MainConfig struct {
	// GeneralConfig contains general configuration settings
	General GeneralConfig `comment:"General application settings including logging, workers, and caching.\nThis section controls core behavior and performance" displayname:"General Application Settings" longcomment:"General application settings including logging, workers, and caching.\nThis section controls core behavior like log levels, worker threads,\nAPI keys, and performance-related cache settings.\nRequired section - must be configured for proper operation." toml:"general"`

	// ImdbConfig contains IMDB specific configuration
	Imdbindexer ImdbConfig `comment:"IMDB database configuration for movie metadata and indexing.\nControls how IMDB data is downloaded and processed" displayname:"IMDB Database Configuration" longcomment:"IMDB database configuration for movie metadata and indexing.\nControls how IMDB data is downloaded, processed, and stored locally.\nIncludes settings for database paths, update intervals, and data filtering.\nOptional section - only needed if using IMDB metadata features." toml:"imdbindexer"`

	// mediaConfig contains media related configuration
	Media MediaConfig `comment:"Media type definitions for movies and TV series.\nDefines how different media types are handled" displayname:"Media Type Configuration" longcomment:"Media type definitions for movies and TV series.\nDefines how different media types are handled, including\ndata sources, quality profiles, notification settings, and search behavior.\nRequired section - must define at least one media type." toml:"media"`

	// DownloaderConfig defines downloader specific configuration
	Downloader []DownloaderConfig `comment:"Download client configurations for handling media downloads.\nDefine connections to SABnzbd, NZBGet, qBittorrent, Transmission, etc." displayname:"Download Client Configurations" longcomment:"Download client configurations for handling media downloads.\nDefine connections to SABnzbd, NZBGet, qBittorrent, Transmission, etc.\nEach entry specifies connection details, categories, and authentication.\nRequired section - must have at least one configured downloader." toml:"downloader"`

	// ListsConfig contains configuration for lists
	Lists []ListsConfig `comment:"External list configurations for automatic media discovery.\nConnect to IMDB lists, Trakt lists, RSS feeds, and other sources" displayname:"External List Configurations" longcomment:"External list configurations for automatic media discovery.\nConnect to IMDB lists, Trakt lists, RSS feeds, and other sources\nto automatically add new media to your wanted lists.\nOptional section - only needed if using automatic list imports." toml:"lists"`

	// IndexersConfig defines configuration for indexers
	Indexers []IndexersConfig `comment:"Search indexer configurations for finding media releases.\nDefine connections to Usenet indexers and torrent trackers" displayname:"Search Indexer Configurations" longcomment:"Search indexer configurations for finding media releases.\nDefine connections to Usenet indexers and torrent trackers.\nEach entry includes API keys, search limits, and connection settings.\nRequired section - must have at least one configured indexer." toml:"indexers"`

	// PathsConfig contains configuration for paths
	Paths []PathsConfig `comment:"File system path configurations for media storage and organization.\nDefine where media files are stored and organized" displayname:"File System Path Configurations" longcomment:"File system path configurations for media storage and organization.\nDefine where media files are stored, file extensions, size limits,\nand file management behaviors like upgrades and cleanup.\nRequired section - must define at least one path configuration." toml:"paths"`

	// NotificationConfig contains configuration for notifications
	Notification []NotificationConfig `comment:"Notification service configurations for download alerts.\nSet up Pushover, email, webhooks, or file-based notifications" displayname:"Notification Service Configurations" longcomment:"Notification service configurations for download alerts.\nSet up Pushover, email, webhooks, or file-based notifications\nto get alerts when media is downloaded or other events occur.\nOptional section - only needed if you want notifications." toml:"notification"`

	// RegexConfig contains configuration for regex
	Regex []RegexConfig `comment:"Regular expression configurations for filtering search results.\nDefine patterns to require or reject specific release characteristics" displayname:"Regular Expression Configurations" longcomment:"Regular expression configurations for filtering search results.\nDefine patterns to require or reject specific release characteristics\nlike group names, file naming conventions, or quality indicators.\nOptional section - only needed for advanced filtering requirements." toml:"regex"`

	// QualityConfig contains configuration for quality
	Quality []QualityConfig `comment:"Quality profile configurations defining preferred media characteristics.\nSet desired video resolutions, audio quality, codecs, and behavior" displayname:"Quality Profile Configurations" longcomment:"Quality profile configurations defining preferred media characteristics.\nSet desired video resolutions, audio quality, codecs, and search behavior.\nEach profile can have different indexer and quality preferences.\nRequired section - must define at least one quality profile." toml:"quality"`

	// SchedulerConfig contains configuration for scheduler
	Scheduler []SchedulerConfig `comment:"Task scheduler configurations for automated operations.\nDefine intervals or cron schedules for searches, scans, backups,\nand other maintenance tasks" displayname:"Task Scheduler Configurations" longcomment:"Task scheduler configurations for automated operations.\nDefine intervals or cron schedules for searches, scans, backups,\nand other maintenance tasks. Controls when and how often tasks run.\nRequired section - must define at least one scheduler configuration." toml:"scheduler"`
}

type GeneralConfig struct {
	// TimeFormat defines the time format to use, options are rfc3339, iso8601, rfc1123, rfc822, rfc850 - default: rfc3339
	TimeFormat string `comment:"Specify the timestamp format used in logs and API responses.\nAvailable options:\n- 'rfc3339': 2023-01-15T14:30:45Z (recommended)" displayname:"Log Timestamp Format" longcomment:"Specify the timestamp format used in logs and API responses.\nAvailable options:\n- 'rfc3339': 2023-01-15T14:30:45Z (recommended, ISO 8601 compatible)\n- 'iso8601': 2023-01-15T14:30:45+00:00\n- 'rfc1123': Sun, 15 Jan 2023 14:30:45 GMT\n- 'rfc822': 15 Jan 23 14:30 GMT\n- 'rfc850': Sunday, 15-Jan-23 14:30:45 GMT\nDefault: 'rfc3339'" toml:"time_format"`
	// TimeZone defines the timezone to use, options are local, utc or one from IANA Time Zone database
	TimeZone string `comment:"Set the timezone for timestamp display and scheduling.\nOptions:\n- 'local': Use system's local timezone\n- 'utc': Use UTC" displayname:"Application Timezone" longcomment:"Set the timezone for timestamp display and scheduling.\nOptions:\n- 'local': Use system's local timezone\n- 'utc': Use Coordinated Universal Time\n- IANA timezone: Use specific timezone (e.g., 'America/New_York', 'Europe/London')\nFind IANA timezones at: https://en.wikipedia.org/wiki/List_of_tz_database_time_zones\nExample: 'America/Los_Angeles' or 'Europe/Berlin'" toml:"time_zone"`
	// UserAgent defines the User-Agent string sent with HTTP requests to external APIs - default: go-media-downloader/2.0
	UserAgent string `comment:"User-Agent string sent with HTTP requests to external APIs.\nSome APIs require a descriptive User-Agent for identification" displayname:"HTTP User-Agent String" longcomment:"User-Agent string sent with HTTP requests to external APIs.\nSome APIs require a descriptive User-Agent for identification and rate limiting.\nUsed by MusicBrainz, Discogs, and other services that track API usage.\nFormat: 'AppName/Version (contact info or URL)'\nExample: 'MyMediaApp/1.0 (https://example.com)'\nDefault: 'go-media-downloader/2.0'" toml:"user_agent"`
	// LogLevel defines the log level to use, options are info or debug - default: info
	LogLevel string `comment:"Set the application logging verbosity level.\nOptions:\n- 'info': Standard logging with important events and errors\n- 'debug': Detailed logging" displayname:"Application Log Level" longcomment:"Set the application logging verbosity level.\nOptions:\n- 'info': Standard logging with important events and errors\n- 'debug': Detailed logging including debug information (verbose)\nUse 'info' for normal operation, 'debug' for troubleshooting.\nWarning: Debug level generates large log files.\nDefault: 'info'" toml:"log_level"`
	// DBLogLevel defines the database log level to use, options are info or debug (not recommended) - default: info
	DBLogLevel string `comment:"Set the database operation logging level.\nOptions:\n- 'info': Log important database operations only\n- 'debug': Log all SQL queries" displayname:"Database Log Level" longcomment:"Set the database operation logging level.\nOptions:\n- 'info': Log important database operations only\n- 'debug': Log all SQL queries and database operations (very verbose)\nWarning: Debug level creates extremely large logs and impacts performance.\nOnly use 'debug' for database troubleshooting.\nDefault: 'info'" toml:"db_log_level"`
	// LogFileSize defines the size in MB for the log files - default: 5
	LogFileSize int `comment:"Maximum size in megabytes for each log file before rotation.\nWhen a log file reaches this size, it will be rotated" displayname:"Log File Size MB" longcomment:"Maximum size in megabytes for each log file before rotation.\nWhen a log file reaches this size, it will be rotated and a new file started.\nLarger files mean less frequent rotation but harder to manage.\nSmaller files rotate more often but are easier to read.\nRecommended range: 5-50 MB\nDefault: 5" toml:"log_file_size"`
	// LogFileCount defines how many log files to keep - default: 1
	LogFileCount uint8 `comment:"Number of rotated log files to retain before deletion.\nWhen log rotation occurs, this many old files will be kept" displayname:"Rotated Log File Count" longcomment:"Number of rotated log files to retain before deletion.\nWhen log rotation occurs, this many old files will be kept.\nHigher values preserve more history but use more disk space.\nSet to 0 to keep only the current log file.\nRecommended range: 1-10\nDefault: 1" toml:"log_file_count"`
	// LogCompress defines whether to compress old log files - default: false
	LogCompress bool `comment:"Enable compression of rotated log files to save disk space.\nWhen true, old log files are compressed using gzip" displayname:"Compress Old Log Files" longcomment:"Enable compression of rotated log files to save disk space.\nWhen true, old log files are compressed using gzip compression.\nThis significantly reduces disk usage but makes logs harder to read directly.\nUse true if disk space is limited and you rarely need to read old logs.\nDefault: false" toml:"log_compress"`
	// LogToFileOnly defines whether to only log to file and not console - default: false
	LogToFileOnly bool `comment:"Disable console output and log only to files.\nWhen true, all log messages go only to log files" displayname:"Log Only To Files" longcomment:"Disable console output and log only to files.\nWhen true, all log messages go only to log files, not the console.\nUseful for background services or when console output causes issues.\nWhen false, logs appear both in files and console output.\nDefault: false (logs to both console and file)" toml:"log_to_file_only"`
	// LogColorize defines whether to use colors in console output - default: false
	LogColorize bool `comment:"Enable colored console output for better log readability.\nWhen true, different log levels are displayed in different colors" displayname:"Enable Colored Console Output" longcomment:"Enable colored console output for better log readability.\nWhen true, different log levels are displayed in different colors\n(errors in red, warnings in yellow, etc.).\nMay not work properly in all terminal environments.\nDisable if colors cause display issues or when redirecting output.\nDefault: false" toml:"log_colorize"`
	// LogZeroValues determines whether to log variables without a value.
	// LogZeroValues bool `toml:"log_zero_values"  displayname:"Log Empty Values"              comment:"Include empty/zero values in log output for debugging.\nWhen true, variables with empty strings and zero numbers are logged"                    longcomment:"Include empty/zero values in log output for debugging.\nWhen true, variables with empty strings, zero numbers, etc. are logged.\nUseful for debugging configuration issues but creates more verbose logs.\nWhen false, only variables with actual values are logged.\nDefault: false"`
	// WorkerMetadata defines how many parallel jobs of list retrievals to run - default: 1
	WorkerMetadata int `comment:"Number of parallel workers for metadata and list retrieval tasks.\nHigher values speed up processing but may overwhelm external APIs" displayname:"Metadata Worker Threads" longcomment:"Number of parallel workers for metadata and list retrieval tasks.\nHigher values speed up processing of IMDB lists, Trakt lists, etc.\nToo many workers may overwhelm external APIs or cause rate limiting.\nRecommended range: 1-5 depending on your system and API limits.\nDefault: 1" toml:"worker_metadata"`
	// WorkerFiles defines how many parallel jobs of file scanning to run - default: 1
	WorkerFiles int `comment:"Number of parallel workers for file system scanning operations.\nHigher values can speed up large library scans" displayname:"File Scanner Worker Threads" longcomment:"Number of parallel workers for file system scanning operations.\nHigher values can speed up large library scans but increase I/O load.\nMore workers may cause issues with network storage or slow drives.\nRecommended: Keep at 1 unless you have very fast local storage.\nDefault: 1" toml:"worker_files"`

	// WorkerParse defines how many parallel parsings to run for list retrievals - default: 1
	WorkerParse int `comment:"Number of parallel workers for parsing list data (RSS, CSV, etc.).\nHigher values speed up processing but increase CPU usage" displayname:"List Parser Worker Threads" longcomment:"Number of parallel workers for parsing list data (RSS, CSV, etc.).\nHigher values speed up processing of large lists and feeds.\nMore workers increase CPU usage but reduce processing time.\nUseful when importing large IMDB lists or processing many RSS feeds.\nRecommended range: 1-4\nDefault: 1" toml:"worker_parse"`
	// WorkerSearch defines how many parallel search jobs to run - default: 1
	WorkerSearch int `comment:"Number of parallel workers for search operations (missing/upgrade scans).\nHigher values speed up searches but increase resource usage" displayname:"Search Worker Threads" longcomment:"Number of parallel workers for search operations (missing/upgrade scans).\nHigher values speed up searches but increase resource usage.\nToo many workers may overwhelm indexers or cause rate limiting.\nBalance between speed and indexer limits/system resources.\nRecommended range: 1-3\nDefault: 1" toml:"worker_search"`
	// WorkerRSS defines how many parallel rss jobs to run - default: 1
	WorkerRSS int `comment:"Number of parallel workers for RSS feed processing.\nHigher values speed up RSS feed checks and parsing" displayname:"RSS Worker Threads" longcomment:"Number of parallel workers for RSS feed processing.\nHigher values speed up RSS feed checks and parsing.\nToo many workers may cause issues with feed servers or rate limits.\nIncrease only if you have many RSS feeds and fast internet.\nRecommended range: 1-3\nDefault: 1" toml:"worker_rss"`
	// WorkerIndexer defines how many indexers to query in parallel for each scan job - default: 1
	WorkerIndexer int `comment:"Number of indexers to query simultaneously during searches.\nHigher values speed up searches by querying multiple indexers" displayname:"Parallel Indexer Workers" longcomment:"Number of indexers to query simultaneously during searches.\nHigher values speed up searches by querying multiple indexers at once.\nToo many may trigger rate limits or overwhelm your connection.\nBalance between search speed and indexer API limits.\nRecommended range: 1-5 depending on indexer count and limits\nDefault: 1" toml:"worker_indexer"`
	// OmdbAPIKey is the API key for OMDB - get one at https://www.omdbapi.com/apikey.aspx
	OmdbAPIKey string `comment:"API key for OMDb (Open Movie Database) service.\nRequired for enhanced movie metadata, ratings, and poster information" displayname:"OMDb API Key" longcomment:"API key for OMDb (Open Movie Database) service.\nRequired for enhanced movie metadata, ratings, and poster information.\nGet a free API key at: https://www.omdbapi.com/apikey.aspx\nLeave empty if you don't want OMDb integration.\nExample: 'a1b2c3d4'" toml:"omdb_apikey"`
	// UseMediaCache defines whether to cache movies and series in RAM for better performance - default: false
	UseMediaCache bool `comment:"Cache movie and TV series information in RAM for faster access.\nWhen enabled, media metadata is kept in memory" displayname:"Cache Media In RAM" longcomment:"Cache movie and TV series information in RAM for faster access.\nWhen enabled, media metadata is kept in memory to speed up searches and UI.\nUses more RAM but significantly improves performance for large libraries.\nRecommended for libraries with 1000+ items and sufficient RAM.\nDefault: false" toml:"use_media_cache"`
	// UseFileCache defines whether to cache all files in RAM - default: false
	UseFileCache bool `comment:"Cache complete file listings in RAM for faster file operations.\nWhen enabled, all file paths and metadata are cached" displayname:"Cache Files In RAM" longcomment:"Cache complete file listings in RAM for faster file operations.\nWhen enabled, all file paths and metadata are kept in memory.\nDramatically speeds up file scans but uses significant RAM.\nRecommended only for large libraries with fast systems and ample RAM.\nDefault: false" toml:"use_file_cache"`
	// UseHistoryCache defines whether to cache downloaded entry history in RAM - default: false
	UseHistoryCache bool `comment:"Cache download history in RAM to prevent duplicate downloads.\nWhen enabled, download history is kept in memory" displayname:"Cache History In RAM" longcomment:"Cache download history in RAM to prevent duplicate downloads.\nWhen enabled, download history is kept in memory for faster duplicate checking.\nImproves performance when processing many search results.\nUses moderate RAM but significantly speeds up duplicate detection.\nDefault: false" toml:"use_history_cache"`
	// CacheDuration defines hours after which cached data will be refreshed - default: 12
	CacheDuration  int `comment:"Number of hours before cached data expires and gets refreshed.\nAfter this time, cached data is considered stale" displayname:"Cache Duration Hours" longcomment:"Number of hours before cached data expires and gets refreshed.\nAfter this time, cached data is considered stale and will be reloaded.\nLower values keep data fresher but increase database load.\nHigher values reduce load but may show outdated information.\nRecommended range: 6-24 hours\nDefault: 12" toml:"cache_duration"`
	CacheDuration2 int `                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         toml:"-"`
	// CacheAutoExtend defines whether cache expiration will be reset on access - default: false
	CacheAutoExtend bool `comment:"Reset cache expiration timer when data is accessed.\nWhen true, frequently accessed data stays cached longer" displayname:"Extend Cache On Access" longcomment:"Reset cache expiration timer when data is accessed.\nWhen true, frequently accessed data stays cached longer.\nPrevents cache expiration for actively used data.\nWhen false, all cached data expires after the set duration regardless of usage.\nDefault: false" toml:"cache_auto_extend"`
	// UseIndexedCache enables O(1) hash-based indexes for cache lookups - default: false
	UseIndexedCache bool `comment:"Enable hash-based indexes for faster cache lookups.\nDramatically improves performance for large media libraries" displayname:"Use Indexed Cache Lookups" longcomment:"Enable hash-based indexes for faster cache lookups.\nBuilds secondary hash indexes on cached data for O(1) lookups instead of O(n) scans.\nBenefits increase with library size (10,000+ items see 1000x+ speedup).\nMemory overhead: ~20% of cache size.\nRecommended for libraries with 1000+ movies/series.\nDefault: false" toml:"use_indexed_cache"`
	// SearcherSize defines initial size of found entries slice - default: 5000
	SearcherSize int `comment:"Initial memory allocation size for search result storage.\nHigher values reduce memory reallocations during large searches" displayname:"Search Result Buffer Size" longcomment:"Initial memory allocation size for search result storage.\nHigher values reduce memory reallocations during large searches.\nCalculate as: (number of indexers) × (max entries per search) × (alternate titles).\nToo low causes frequent reallocations, too high wastes memory.\nRecommended range: 1000-10000\nDefault: 5000" toml:"searcher_size"`
	// MovieMetaSourceImdb defines whether to scan IMDB for movie metadata - default: false
	MovieMetaSourceImdb bool `comment:"Enable IMDB as a metadata source for movies.\nWhen true, movie information like ratings, cast, plot, etc will be fetched" displayname:"Movie IMDB Metadata" longcomment:"Enable IMDB as a metadata source for movies.\nWhen true, movie information like ratings, cast, plot, etc. will be fetched from IMDB.\nRequires local IMDB database setup (see imdbindexer section).\nProvides comprehensive movie data but requires significant setup.\nDefault: false" toml:"movie_meta_source_imdb"`

	// MovieMetaSourceTmdb defines whether to scan TMDb for movie metadata - default: false
	MovieMetaSourceTmdb bool `comment:"Enable The Movie Database (TMDb) as a metadata source for movies.\nWhen true, movie information will be fetched from TMDb" displayname:"Movie TMDb Metadata" longcomment:"Enable The Movie Database (TMDb) as a metadata source for movies.\nWhen true, movie information will be fetched from TMDb API.\nRequires themoviedb_apikey to be configured.\nProvides high-quality movie metadata with posters and backdrops.\nDefault: false" toml:"movie_meta_source_tmdb"`
	// MovieMetaSourceOmdb defines whether to scan OMDB for movie metadata - default: false
	MovieMetaSourceOmdb bool `comment:"Enable Open Movie Database (OMDb) as a metadata source for movies.\nWhen true, movie information will be fetched from OMDb" displayname:"Movie OMDb Metadata" longcomment:"Enable Open Movie Database (OMDb) as a metadata source for movies.\nWhen true, movie information will be fetched from OMDb API.\nRequires omdb_apikey to be configured.\nProvides movie ratings, plot summaries, and basic metadata.\nDefault: false" toml:"movie_meta_source_omdb"`
	// MovieMetaSourceTrakt defines whether to scan Trakt for movie metadata - default: false
	MovieMetaSourceTrakt bool `comment:"Enable Trakt as a metadata source for movies.\nWhen true, movie information will be fetched from Trakt API" displayname:"Movie Trakt Metadata" longcomment:"Enable Trakt as a metadata source for movies.\nWhen true, movie information will be fetched from Trakt API.\nRequires Trakt authentication (client ID/secret) to be configured.\nProvides user ratings, watch statistics, and social features.\nDefault: false" toml:"movie_meta_source_trakt"`
	// MovieAlternateTitleMetaSourceImdb defines whether to scan IMDB for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceImdb bool `comment:"Fetch alternate movie titles from IMDB to improve search results.\nWhen true, foreign language titles and alternate names are retrieved" displayname:"Movie IMDB Alternate Titles" longcomment:"Fetch alternate movie titles from IMDB to improve search results.\nWhen true, foreign language titles and alternate names are retrieved from IMDB.\nHelps find releases with different naming conventions or translations.\nRequires movie_meta_source_imdb to be enabled.\nDefault: false" toml:"movie_alternate_title_meta_source_imdb"`
	// MovieAlternateTitleMetaSourceTmdb defines whether to scan TMDb for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceTmdb bool `comment:"Fetch alternate movie titles from TMDb to improve search results.\nWhen true, international titles and alternate names are retrieved" displayname:"Movie TMDb Alternate Titles" longcomment:"Fetch alternate movie titles from TMDb to improve search results.\nWhen true, international titles and alternate names are retrieved from TMDb.\nImproves matching of releases with regional or translated titles.\nRequires movie_meta_source_tmdb to be enabled.\nDefault: false" toml:"movie_alternate_title_meta_source_tmdb"`
	// MovieAlternateTitleMetaSourceOmdb defines whether to scan OMDB for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceOmdb bool `comment:"Fetch alternate movie titles from OMDb to improve search results.\nWhen true, alternative titles are retrieved from OMDb API" displayname:"Movie OMDb Alternate Titles" longcomment:"Fetch alternate movie titles from OMDb to improve search results.\nWhen true, alternative titles are retrieved from OMDb API.\nProvides additional title variations for better release matching.\nRequires movie_meta_source_omdb to be enabled.\nDefault: false" toml:"movie_alternate_title_meta_source_omdb"`
	// MovieAlternateTitleMetaSourceTrakt defines whether to scan Trakt for alternate movie titles - default: false
	MovieAlternateTitleMetaSourceTrakt bool `comment:"Fetch alternate movie titles from Trakt to improve search results.\nWhen true, alternative titles and aliases are retrieved" displayname:"Movie Trakt Alternate Titles" longcomment:"Fetch alternate movie titles from Trakt to improve search results.\nWhen true, alternative titles and aliases are retrieved from Trakt API.\nProvides community-contributed title variations for better matching.\nRequires movie_meta_source_trakt to be enabled.\nDefault: false" toml:"movie_alternate_title_meta_source_trakt"`
	// SerieAlternateTitleMetaSourceImdb defines whether to scan IMDB for alternate series titles - default: false
	SerieAlternateTitleMetaSourceImdb bool `comment:"Fetch alternate TV series titles from IMDB to improve search results.\nWhen true, foreign language titles and alternate names are retrieved" displayname:"Series IMDB Alternate Titles" longcomment:"Fetch alternate TV series titles from IMDB to improve search results.\nWhen true, foreign language titles and alternate names are retrieved from IMDB.\nHelps find releases with different naming conventions or translations.\nRequires IMDB database setup and appropriate series metadata source.\nDefault: false" toml:"serie_alternate_title_meta_source_imdb"`
	// SerieAlternateTitleMetaSourceTrakt defines whether to scan Trakt for alternate series titles - default: false
	SerieAlternateTitleMetaSourceTrakt bool `comment:"Fetch alternate TV series titles from Trakt to improve search results.\nWhen true, alternative titles and aliases are retrieved" displayname:"Series Trakt Alternate Titles" longcomment:"Fetch alternate TV series titles from Trakt to improve search results.\nWhen true, alternative titles and aliases are retrieved from Trakt API.\nProvides community-contributed title variations for better matching.\nRequires serie_meta_source_trakt to be enabled.\nDefault: false" toml:"serie_alternate_title_meta_source_trakt"`
	// MovieMetaSourcePriority defines priority order to scan metadata providers for movies - overrides individual settings
	MovieMetaSourcePriority []string `comment:"Priority order for movie metadata providers.\nWhen specified, this overrides individual movie_meta_source_* settings" displayname:"Movie Metadata Source Priority" longcomment:"Priority order for movie metadata providers.\nWhen specified, this overrides individual movie_meta_source_* settings.\nList providers in order of preference: first is tried first.\nAvailable options: 'imdb', 'tmdb', 'omdb', 'trakt'\nExample: ['tmdb', 'imdb', 'omdb']\nLeave empty to use individual settings." multiline:"true" toml:"movie_meta_source_priority"`
	// MovieRSSMetaSourcePriority defines priority order to scan metadata providers for movie RSS - overrides individual settings
	MovieRSSMetaSourcePriority []string `comment:"Priority order for movie metadata when processing RSS feeds.\nWhen specified, this overrides individual movie_meta_source_* settings" displayname:"Movie RSS Metadata Priority" longcomment:"Priority order for movie metadata when processing RSS feeds.\nWhen specified, this overrides individual movie_meta_source_* settings for RSS imports.\nList providers in order of preference for RSS-discovered movies.\nAvailable options: 'imdb', 'tmdb', 'omdb', 'trakt'\nExample: ['tmdb', 'imdb']\nLeave empty to use individual settings." multiline:"true" toml:"movie_rss_meta_source_priority"`

	// MovieParseMetaSourcePriority defines priority order to scan metadata providers for movie file parsing - overrides individual settings
	MovieParseMetaSourcePriority []string `comment:"Priority order for movie metadata when parsing files.\nWhen specified, this overrides individual movie_meta_source_* settings for parsing" displayname:"Movie Parse Metadata Priority" longcomment:"Priority order for movie metadata when parsing files.\nWhen specified, this overrides individual movie_meta_source_* settings for file parsing.\nList providers in order of preference for identifying movies from filenames.\nAvailable options: 'imdb', 'tmdb', 'omdb', 'trakt'\nExample: ['imdb', 'tmdb']\nLeave empty to use individual settings." multiline:"true" toml:"movie_parse_meta_source_priority"`
	// SerieMetaSourceTmdb defines whether to scan TMDb for series metadata - default: false
	SerieMetaSourceTmdb bool `comment:"Enable The Movie Database (TMDb) as a metadata source for TV series.\nWhen true, series information" displayname:"Series TMDb Metadata" longcomment:"Enable The Movie Database (TMDb) as a metadata source for TV series.\nWhen true, series information will be fetched from TMDb API.\nRequires themoviedb_apikey to be configured.\nProvides high-quality series metadata with posters and episode information.\nDefault: false" toml:"serie_meta_source_tmdb"`
	// SerieMetaSourceTrakt defines whether to scan Trakt for series metadata - default: false
	SerieMetaSourceTrakt bool `comment:"Enable Trakt as a metadata source for TV series.\nWhen true, series information will be fetched" displayname:"Series Trakt Metadata" longcomment:"Enable Trakt as a metadata source for TV series.\nWhen true, series information will be fetched from Trakt API.\nRequires Trakt authentication (client ID/secret) to be configured.\nProvides user ratings, watch statistics, and social features for series.\nDefault: false" toml:"serie_meta_source_trakt"`
	// MoveBufferSizeKB defines buffer size in KB to use if file buffer copy enabled - default: 1024
	MoveBufferSizeKB int `comment:"File buffer size in kilobytes for file operations.\nLarger buffers can improve file copy/move performance but use more RAM" displayname:"File Buffer Size KB" longcomment:"File buffer size in kilobytes for file operations.\nLarger buffers can improve file copy/move performance but use more RAM.\nUseful when moving large files or working with network storage.\nRecommended range: 64-4096 KB depending on system and storage type.\nDefault: 1024" toml:"move_buffer_size_kb"`
	// WebPort defines port for web interface and API - default: 9090
	WebPort string `comment:"TCP port number for the web interface and API server.\nThe application will listen on this" displayname:"Web Interface Port" longcomment:"TCP port number for the web interface and API server.\nThe application will listen on this port for HTTP connections.\nMake sure the port is not used by other applications.\nCommon alternatives: 8080, 8090, 9091\nExample: '9090' or '8080'\nDefault: '9090'" toml:"webport"`
	// WebAPIKey defines API key for API calls - default: mysecure
	WebAPIKey string `comment:"API key required for authentication with the REST API.\nUsed by third-party applications and scripts to" displayname:"Web API Key" longcomment:"API key required for authentication with the REST API.\nUsed by third-party applications and scripts to access the API.\nAlso serves as the default admin password for the web interface.\nUse a strong, unique key for security.\nExample: 'mySecureApiKey123'\nDefault: 'mysecure' (change this!)" toml:"webapikey"`
	// WebPortalEnabled enables/disables web portal - default: false
	WebPortalEnabled bool `comment:"Enable the web-based administration interface.\nWhen true, you can access the application through a web browser.\nProvides" displayname:"Enable Web Interface" longcomment:"Enable the web-based administration interface.\nWhen true, you can access the application through a web browser.\nProvides a user-friendly interface for configuration and monitoring.\nWhen false, only API access is available.\nDefault: false" toml:"web_portal_enabled"`
	// TheMovieDBApiKey defines API key for TMDb - get from: https://www.themoviedb.org/settings/api
	TheMovieDBApiKey string `comment:"API key for The Movie Database (TMDb) service.\nRequired for TMDb metadata integration (posters, plot, cast," displayname:"TMDb API Key" longcomment:"API key for The Movie Database (TMDb) service.\nRequired for TMDb metadata integration (posters, plot, cast, etc.).\nGet a free API key by registering at: https://www.themoviedb.org/settings/api\nLeave empty if you don't want TMDb integration.\nExample: 'a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6'" toml:"themoviedb_apikey"`
	// TraktClientID defines client ID for Trakt - get from: https://trakt.tv/oauth/applications/new
	TraktClientID string `comment:"Client ID for Trakt API integration.\nRequired for Trakt features like list syncing and user ratings.\nCreate" displayname:"Trakt Client ID" longcomment:"Client ID for Trakt API integration.\nRequired for Trakt features like list syncing and user ratings.\nCreate an application at: https://trakt.tv/oauth/applications/new\nUse 'http://localhost:9090' as the redirect URI.\nLeave empty if you don't want Trakt integration.\nExample: 'a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0'" toml:"trakt_client_id"`
	// TraktClientSecret defines client secret for Trakt application
	TraktClientSecret string `comment:"Client secret for your Trakt application.\nThis is the secret key paired with your Trakt client" displayname:"Trakt Client Secret" longcomment:"Client secret for your Trakt application.\nThis is the secret key paired with your Trakt client ID.\nFound in your application settings on Trakt after creating the application.\nKeep this secret and do not share it publicly.\nRequired if using Trakt integration.\nExample: 'z9y8x7w6v5u4t3s2r1q0p9o8n7m6l5k4j3i2h1g0'" toml:"trakt_client_secret"`

	TraktRedirectUrl string `comment:"Redirect Url for your Trakt application.\nThis is the Redirect Url paired with your Trakt client" displayname:"Trakt Redirect Url" longcomment:"Redirect Url for your Trakt application.\nThis is the Redirect Url paired with your Trakt client ID.\nFound in your application settings on Trakt after creating the application.\nRequired if using Trakt integration.\nExample: 'http://localhost:9090/'" toml:"trakt_redirect_url"`
	// SchedulerDisabled enables/disables scheduler - default false
	SchedulerDisabled bool `comment:"Disable all automated scheduled tasks.\nWhen true, automatic searches, scans, and maintenance tasks are disabled.\nOnly manual" displayname:"Disable All Schedulers" longcomment:"Disable all automated scheduled tasks.\nWhen true, automatic searches, scans, and maintenance tasks are disabled.\nOnly manual operations will be performed.\nUseful for troubleshooting or when running tasks manually.\nDefault: false (scheduler enabled)" toml:"scheduler_disabled"`

	// DisableParserStringMatch defines whether to disable string matching in parsers - default: false
	DisableParserStringMatch bool `comment:"Disable string-based parsing and use only regex for field matching.\nWhen true, only regex patterns are used for file identification" displayname:"Disable String Matching Parser" longcomment:"Disable string-based parsing and use only regex for field matching.\nWhen true, only regex patterns are used to identify release information.\nThis may decrease performance but can increase parsing accuracy.\nUseful when string matching produces too many false positives.\nWhen false, both string matching and regex are used (faster).\nDefault: false (use both string matching and regex)" toml:"disable_parser_string_match"`
	// UseCronInsteadOfInterval defines whether to convert intervals to cron strings - default: false
	UseCronInsteadOfInterval bool `comment:"Convert scheduler intervals to cron expressions for better performance.\nWhen true, simple intervals are internally converted to cron format" displayname:"Use Cron For Intervals" longcomment:"Convert scheduler intervals to cron expressions for better performance.\nWhen true, simple intervals are internally converted to cron formats.\nThis improves scheduler performance and provides more precise timing.\nWhen false, intervals are used as-is (simpler but less efficient).\nRecommended for systems with many scheduled tasks.\nDefault: false" toml:"use_cron_instead_of_interval"`
	// UseFileBufferCopy defines whether to use buffered file copy - default: false
	UseFileBufferCopy bool `comment:"Enable buffered file copying for potentially improved performance.\nWhen true, files are copied using a buffer of configured size" displayname:"Use Buffered File Copy" longcomment:"Enable buffered file copying for potentially improved performance.\nWhen true, files are copied using a buffer of configured size.\nMay improve performance on some systems but not recommended generally.\nCan cause issues with network storage or slow drives.\nBuffer size is controlled by move_buffer_size_kb setting.\nDefault: false (not recommended)" toml:"use_file_buffer_copy"`
	// DisableSwagger defines whether to disable Swagger API docs - default: false
	// DisableSwagger bool `toml:"disable_swagger"              displayname:"Disable API Documentation"      comment:"Disable automatic Swagger API documentation generation.\nWhen true, the Swagger UI and API docs are not generated or served"                  longcomment:"Disable automatic Swagger API documentation generation.\nWhen true, the Swagger UI and API docs are not generated or served.\nThis can slightly improve startup time and reduce memory usage.\nUseful in production environments where API docs are not needed.\nWhen false, API documentation is available at /swagger endpoint.\nDefault: false (Swagger enabled)"`
	// TraktLimiterSeconds defines seconds limit for Trakt API calls - default: 1
	TraktLimiterSeconds uint8 `comment:"Time window in seconds for Trakt API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Trakt Rate Limit Seconds" longcomment:"Time window in seconds for Trakt API rate limiting.\nDefines the time period over which API call limits are applied.\nWorks together with trakt_limiter_calls to prevent API rate limit violations.\nTrakt's API limits change, so adjust based on current Trakt documentation.\nLower values provide more granular rate limiting control.\nDefault: 1 (one second time window)" toml:"trakt_limiter_seconds"`
	// TraktLimiterCalls defines calls limit for Trakt API in defined seconds - default: 1
	TraktLimiterCalls int `comment:"Maximum number of API calls allowed to Trakt within the defined time window.\nWorks with trakt_limiter_seconds" displayname:"Trakt Calls Per Window" longcomment:"Maximum number of API calls allowed to Trakt within the defined time window.\nWorks with trakt_limiter_seconds to enforce rate limiting.\nIf you exceed this limit, requests will be delayed to comply with limits.\nCheck Trakt's current API documentation for their actual rate limits.\nConservative values prevent API key suspension due to rate limit violations.\nDefault: 1 (one call per time window)" toml:"trakt_limiter_calls"`
	// TvdbLimiterSeconds defines seconds limit for TVDB API calls - default: 1
	TvdbLimiterSeconds uint8 `comment:"Time window in seconds for TVDB API rate limiting.\nDefines the time period over which TVDB" displayname:"TVDB Rate Limit Seconds" longcomment:"Time window in seconds for TVDB API rate limiting.\nDefines the time period over which TVDB API call limits are applied.\nWorks together with tvdb_limiter_calls to prevent API rate limit violations.\nTVDB's API has specific rate limits that change over time.\nAdjust based on current TVDB API documentation and your subscription level.\nDefault: 1 (one second time window)" toml:"tvdb_limiter_seconds"`
	// TvdbLimiterCalls defines calls limit for TVDB API in defined seconds - default: 1
	TvdbLimiterCalls int `comment:"Maximum number of API calls allowed to TVDB within the defined time window.\nWorks with tvdb_limiter_seconds" displayname:"TVDB Calls Per Window" longcomment:"Maximum number of API calls allowed to TVDB within the defined time window.\nWorks with tvdb_limiter_seconds to enforce rate limiting.\nTVDB has different rate limits for free vs paid subscriptions.\nExceeding limits may result in temporary API access suspension.\nCheck your TVDB subscription level and current API documentation.\nDefault: 1 (one call per time window)" toml:"tvdb_limiter_calls"`
	// TmdbLimiterSeconds defines seconds limit for TMDb API calls - default: 1
	TmdbLimiterSeconds uint8 `comment:"Time window in seconds for TMDb API rate limiting.\nDefines the time period over which TMDb" displayname:"TMDb Rate Limit Seconds" longcomment:"Time window in seconds for TMDb API rate limiting.\nDefines the time period over which TMDb API call limits are applied.\nWorks together with tmdb_limiter_calls to prevent API rate limit violations.\nTMDb has generous rate limits but they can change based on usage patterns.\nAdjust based on current TMDb API documentation and your usage needs.\nDefault: 1 (one second time window)" toml:"tmdb_limiter_seconds"`
	// TmdbLimiterCalls defines calls limit for TMDb API in defined seconds - default: 1
	TmdbLimiterCalls int `comment:"Maximum number of API calls allowed to TMDb within the defined time window.\nWorks with tmdb_limiter_seconds" displayname:"TMDb Calls Per Window" longcomment:"Maximum number of API calls allowed to TMDb within the defined time window.\nWorks with tmdb_limiter_seconds to enforce rate limiting.\nTMDb typically allows 40 requests per 10 seconds for free accounts.\nExceeding limits may result in temporary API throttling or blocks.\nAdjust based on your TMDb API key limits and usage requirements.\nDefault: 1 (one call per time window)" toml:"tmdb_limiter_calls"`
	// OmdbLimiterSeconds defines seconds limit for OMDb API calls - default: 1
	OmdbLimiterSeconds uint8 `comment:"Time window in seconds for OMDb API rate limiting.\nDefines the time period over which OMDb" displayname:"OMDb Rate Limit Seconds" longcomment:"Time window in seconds for OMDb API rate limiting.\nDefines the time period over which OMDb API call limits are applied.\nWorks together with omdb_limiter_calls to prevent API rate limit violations.\nOMDb has strict rate limits that vary by subscription tier.\nFree tier typically allows 1000 calls per day with rate restrictions.\nDefault: 1 (one second time window)" toml:"omdb_limiter_seconds"`
	// OmdbLimiterCalls defines calls limit for OMDb API in defined seconds - default: 1
	OmdbLimiterCalls int `comment:"Maximum number of API calls allowed to OMDb within the defined time window.\nWorks with omdb_limiter_seconds" displayname:"OMDb Calls Per Window" longcomment:"Maximum number of API calls allowed to OMDb within the defined time window.\nWorks with omdb_limiter_seconds to enforce rate limiting.\nOMDb free tier allows 1000 calls/day, paid tiers have higher limits.\nExceeding daily limits results in API key suspension until reset.\nConservative rate limiting prevents accidental limit violations.\nDefault: 1 (one call per time window)" toml:"omdb_limiter_calls"`
	// TvmazeLimiterSeconds defines seconds limit for TVmaze API calls - default: 1
	TvmazeLimiterSeconds uint8 `comment:"Time window in seconds for TVmaze API rate limiting.\nDefines the time period over which TVmaze" displayname:"TVmaze Rate Limit Seconds" longcomment:"Time window in seconds for TVmaze API rate limiting.\nDefines the time period over which TVmaze API call limits are applied.\nWorks together with tvmaze_limiter_calls to prevent API rate limit violations.\nTVmaze has no API key requirement but enforces reasonable rate limits.\nBe respectful with request frequency to maintain good API citizenship.\nDefault: 1 (one second time window)" toml:"tvmaze_limiter_seconds"`
	// TvmazeLimiterCalls defines calls limit for TVmaze API in defined seconds - default: 1
	TvmazeLimiterCalls int `comment:"Maximum number of API calls allowed to TVmaze within the defined time window.\nWorks with tvmaze_limiter_seconds" displayname:"TVmaze Calls Per Window" longcomment:"Maximum number of API calls allowed to TVmaze within the defined time window.\nWorks with tvmaze_limiter_seconds to enforce rate limiting.\nTVmaze doesn't publish official rate limits but prefers reasonable usage.\nTypical usage should stay well under their informal limits.\nConservative rate limiting ensures continued access to their free API.\nDefault: 1 (one call per time window)" toml:"tvmaze_limiter_calls"`

	// TheMovieDBDisableTLSVerify disables TLS certificate verification for TheMovieDB API requests
	// Setting this to true may increase performance but reduces security
	TheMovieDBDisableTLSVerify bool `comment:"Disable SSL/TLS certificate verification for TMDb API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay" displayname:"TMDb Disable SSL Verification" longcomment:"Disable SSL/TLS certificate verification for TMDb API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nLeaves connections vulnerable to man-in-the-middle attacks.\nDefault: false (secure connections with certificate verification)" toml:"tmdb_disable_tls_verify"`

	// TraktDisableTLSVerify disables TLS certificate verification for Trakt API requests
	// Setting this to true may increase performance but reduces security
	TraktDisableTLSVerify bool `comment:"Disable SSL/TLS certificate verification for Trakt API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay" displayname:"Trakt Disable SSL Verification" longcomment:"Disable SSL/TLS certificate verification for Trakt API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nLeaves OAuth token exchange vulnerable to interception.\nDefault: false (secure connections with certificate verification)" toml:"trakt_disable_tls_verify"`

	// OmdbDisableTLSVerify disables TLS certificate verification for OMDb API requests
	// Setting this to true may increase performance but reduces security
	OmdbDisableTLSVerify bool `comment:"Disable SSL/TLS certificate verification for OMDb API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay" displayname:"OMDb Disable SSL Verification" longcomment:"Disable SSL/TLS certificate verification for OMDb API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nAPI keys could be intercepted through compromised connections.\nDefault: false (secure connections with certificate verification)" toml:"omdb_disable_tls_verify"`

	// TvdbDisableTLSVerify disables TLS certificate verification for TVDB API requests
	// Setting this to true may increase performance but reduces security
	TvdbDisableTLSVerify bool `comment:"Disable SSL/TLS certificate verification for TVDB API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay" displayname:"TVDB Disable SSL Verification" longcomment:"Disable SSL/TLS certificate verification for TVDB API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nAuthentication tokens could be compromised through insecure connections.\nDefault: false (secure connections with certificate verification)" toml:"tvdb_disable_tls_verify"`

	// TvmazeDisableTLSVerify disables TLS certificate verification for TVmaze API requests
	// Setting this to true may increase performance but reduces security
	TvmazeDisableTLSVerify bool `comment:"Disable SSL/TLS certificate verification for TVmaze API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay" displayname:"TVmaze Disable SSL Verification" longcomment:"Disable SSL/TLS certificate verification for TVmaze API requests.\nWhen true, SSL certificates are not validated (INSECURE).\nMay slightly improve performance but significantly reduces security.\nOnly enable if you have certificate issues and understand the risks.\nAPI requests could be intercepted through compromised connections.\nDefault: false (secure connections with certificate verification)" toml:"tvmaze_disable_tls_verify"`

	// FfprobePath specifies the path to the ffprobe executable
	// Used for media analysis
	FfprobePath string `comment:"Absolute path to the ffprobe executable for media file analysis.\nSpecifying the full path improves performance" displayname:"FFprobe Executable Path" longcomment:"Absolute path to the ffprobe executable for media file analysis.\nSpecifying the full path improves performance by avoiding PATH searches.\nFfprobe is part of FFmpeg and is used to extract media file information.\nRequired for video duration, codec, and quality detection.\nDownload FFmpeg from: https://ffmpeg.org/download.html\nExample: '/usr/bin/ffprobe' or 'C:\\FFmpeg\\bin\\ffprobe.exe'\nDefault: './ffprobe' (current directory)" toml:"ffprobe_path"`

	// MediainfoPath specifies the path to the mediainfo executable
	// Used as an alternative to ffprobe for media analysis
	MediainfoPath string `comment:"Absolute path to the MediaInfo executable for media file analysis.\nSpecifying the full path improves performance" displayname:"MediaInfo Executable Path" longcomment:"Absolute path to the MediaInfo executable for media file analysis.\nSpecifying the full path improves performance by avoiding PATH searches.\nMediaInfo is an alternative to ffprobe for extracting media information.\nSome users prefer it for certain file formats or analysis accuracy.\nDownload from: https://mediaarea.net/en/MediaInfo/Download\nExample: '/usr/bin/mediainfo' or 'C:\\MediaInfo\\mediainfo.exe'\nDefault: './mediainfo' (current directory)" toml:"mediainfo_path"`

	// MetaflacPath specifies the path to the metaflac executable
	// Used for writing FLAC audio file tags
	MetaflacPath string `comment:"Absolute path to the metaflac executable for FLAC tag editing.\nSpecifying the full path improves performance" displayname:"Metaflac Executable Path" longcomment:"Absolute path to the metaflac executable for FLAC audio tag editing.\nSpecifying the full path improves performance by avoiding PATH searches.\nMetaflac is part of the FLAC reference implementation and is used to read/write FLAC metadata.\nRequired for writing tags to FLAC audio files (reading works without it).\nDownload FLAC tools from: https://xiph.org/flac/download.html\nExample: '/usr/bin/metaflac' or 'C:\\FLAC\\metaflac.exe'\nDefault: 'metaflac' (searches in PATH)" toml:"metaflac_path"`

	// UseMediainfo specifies whether to use mediainfo instead of ffprobe for media analysis
	UseMediainfo bool `comment:"Use MediaInfo instead of ffprobe as the primary media analysis tool.\nWhen true, MediaInfo is used" displayname:"Use MediaInfo Over FFprobe" longcomment:"Use MediaInfo instead of ffprobe as the primary media analysis tool.\nWhen true, MediaInfo is used for all media file analysis tasks.\nWhen false, ffprobe (FFmpeg) is used for media analysis.\nMediaInfo may provide different information or work better with certain formats.\nRequires mediainfo_path to be properly configured.\nDefault: false (use ffprobe)" toml:"use_mediainfo"`

	// UseMediaFallback specifies whether to use mediainfo as a fallback if ffprobe fails
	UseMediaFallback bool `comment:"Use MediaInfo as a backup when ffprobe fails to analyze media files.\nWhen true, if ffprobe" displayname:"Use MediaInfo As Fallback" longcomment:"Use MediaInfo as a backup when ffprobe fails to analyze media files.\nWhen true, if ffprobe fails, MediaInfo will be tried automatically.\nProvides redundancy for media analysis in case one tool fails.\nUseful when dealing with problematic or unusual file formats.\nRequires both ffprobe_path and mediainfo_path to be configured.\nDefault: false (no fallback)" toml:"use_media_fallback"`

	// FailedIndexerBlockTime specifies how long in minutes an indexer should be blocked after failures
	FailedIndexerBlockTime int `comment:"Duration in minutes to temporarily block an indexer after consecutive failures.\nWhen an indexer fails repeatedly," displayname:"Failed Indexer Block Minutes" longcomment:"Duration in minutes to temporarily block an indexer after consecutive failures.\nWhen an indexer fails repeatedly, it's blocked for this time period.\nPrevents wasting resources on consistently failing indexers.\nAfter the block period, the indexer is retried automatically.\nLonger times reduce load on failing indexers, shorter times retry sooner.\nTypical range: 1-60 minutes\nDefault: 5" toml:"failed_indexer_block_time"`

	// MaxDatabaseBackups defines the maximum number of database backups to retain
	MaxDatabaseBackups int `comment:"Maximum number of database backup files to keep before deleting old ones.\nAutomatic backups are created" displayname:"Maximum Database Backups" longcomment:"Maximum number of database backup files to keep before deleting old ones.\nAutomatic backups are created during maintenance and configuration changes.\nOlder backups beyond this limit are automatically deleted.\nSet to 0 to completely disable database backups (not recommended).\nHigher values preserve more backup history but use more disk space.\nRecommended range: 3-10 backups\nDefault: 0 (backups disabled)" toml:"max_database_backups"`

	// DatabaseBackupStopTasks specifies whether to stop background tasks during database backups
	DatabaseBackupStopTasks bool `comment:"Pause all background tasks and schedulers during database backup operations.\nWhen true, searches, scans, and scheduled" displayname:"Stop Tasks During Backup" longcomment:"Pause all background tasks and schedulers during database backup operations.\nWhen true, searches, scans, and scheduled tasks are suspended during backups.\nThis ensures database consistency but temporarily halts all operations.\nWhen false, tasks continue running during backups (may cause inconsistencies).\nRecommended: true for data integrity, false for minimal downtime.\nDefault: false" toml:"database_backup_stop_tasks"`

	// DisableVariableCleanup specifies whether to disable cleanup of variables after use
	// This may reduce RAM usage but variables will persist
	// Default is false
	// DisableVariableCleanup bool `toml:"disable_variable_cleanup" displayname:"Disable Variable Cleanup"       comment:"Disable automatic cleanup of variables after use to potentially reduce RAM usage.\nWhen true, variables remain" longcomment:"Disable automatic cleanup of variables after use to potentially reduce RAM usage.\nWhen true, variables remain in memory longer, possibly reducing allocations.\nMay actually increase memory usage if variables accumulate over time.\nWhen false, variables are cleaned up promptly after use (recommended).\nThis is an experimental optimization that may or may not help performance.\nDefault: false (enable cleanup)"`
	// OmdbTimeoutSeconds defines the HTTP timeout in seconds for OMDb API calls
	// Default is 10 seconds
	OmdbTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for OMDb API calls.\nMaximum time to wait for OMDb API" displayname:"OMDb Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for OMDb API calls.\nMaximum time to wait for OMDb API responses before timing out.\nOMDb responses are typically fast but may vary based on server load.\nLonger timeouts accommodate network latency and server delays.\nShorter timeouts provide faster error detection but may cause false failures.\nTypical range: 5-30 seconds\nDefault: 10" toml:"omdb_timeout_seconds"`
	// TmdbTimeoutSeconds defines the HTTP timeout in seconds for TMDb API calls
	// Default is 10 seconds
	TmdbTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for TMDb API calls.\nMaximum time to wait for TMDb API" displayname:"TMDb Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for TMDb API calls.\nMaximum time to wait for TMDb API responses before timing out.\nTMDb generally has fast response times but may vary during peak usage.\nImage and artwork requests may take longer than metadata requests.\nBalance between patience for slow responses and quick error detection.\nTypical range: 5-30 seconds\nDefault: 10" toml:"tmdb_timeout_seconds"`
	// TvdbTimeoutSeconds defines the HTTP timeout in seconds for TVDB API calls
	// Default is 10 seconds
	TvdbTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for TVDB API calls.\nMaximum time to wait for TVDB API" displayname:"TVDB Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for TVDB API calls.\nMaximum time to wait for TVDB API responses before timing out.\nLonger timeouts are more forgiving of network issues but slower to fail.\nShorter timeouts fail faster but may miss responses on slow connections.\nBalance between responsiveness and reliability based on your connection.\nTypical range: 5-30 seconds\nDefault: 10" toml:"tvdb_timeout_seconds"`
	// TraktTimeoutSeconds defines the HTTP timeout in seconds for Trakt API calls
	// Default is 10 seconds
	TraktTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for Trakt API calls.\nMaximum time to wait for Trakt API" displayname:"Trakt Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for Trakt API calls.\nMaximum time to wait for Trakt API responses before timing out.\nTrakt OAuth operations may take longer than simple API calls.\nLonger timeouts accommodate OAuth flows and complex requests.\nShorter timeouts provide faster failure detection.\nTypical range: 5-30 seconds\nDefault: 10" toml:"trakt_timeout_seconds"`
	// TvmazeTimeoutSeconds defines the HTTP timeout in seconds for TVmaze API calls
	// Default is 10 seconds
	TvmazeTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for TVmaze API calls.\nMaximum time to wait for TVmaze API" displayname:"TVmaze Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for TVmaze API calls.\nMaximum time to wait for TVmaze API responses before timing out.\nTVmaze generally has fast response times and good reliability.\nLonger timeouts accommodate network latency and peak usage times.\nShorter timeouts provide faster error detection for failing requests.\nTypical range: 5-30 seconds\nDefault: 10" toml:"tvmaze_timeout_seconds"`

	// PlexLimiterSeconds defines seconds limit for Plex API calls - default: 1
	PlexLimiterSeconds uint8 `comment:"Time window in seconds for Plex API rate limiting.\nDefines the time period over which Plex API call limits are applied" displayname:"Plex Rate Limit Seconds" longcomment:"Time window in seconds for Plex API rate limiting.\nDefines the time period over which Plex API call limits are applied.\nWorks together with plex_limiter_calls to prevent API rate limit violations.\nPlex servers typically have generous rate limits but may vary by setup.\nLower values provide more granular rate limiting control.\nDefault: 1 (one second time window)" toml:"plex_limiter_seconds"`
	// PlexLimiterCalls defines calls limit for Plex API in defined seconds - default: 10
	PlexLimiterCalls int `comment:"Maximum number of API calls allowed to Plex within the defined time window.\nWorks with plex_limiter_seconds" displayname:"Plex Calls Per Window" longcomment:"Maximum number of API calls allowed to Plex within the defined time window.\nWorks with plex_limiter_seconds to enforce rate limiting.\nPlex servers typically handle many concurrent requests well.\nAdjust based on your Plex server's performance and network capacity.\nConservative values prevent overwhelming your Plex server.\nDefault: 10 (ten calls per time window)" toml:"plex_limiter_calls"`
	// PlexTimeoutSeconds defines the HTTP timeout in seconds for Plex API calls - default: 30
	PlexTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for Plex API calls.\nMaximum time to wait for Plex API" displayname:"Plex Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for Plex API calls.\nMaximum time to wait for Plex API responses before timing out.\nPlex servers may respond slower on older hardware or with large libraries.\nWatchlist operations typically complete quickly but vary by server load.\nBalance between patience for slow responses and quick error detection.\nTypical range: 10-60 seconds\nDefault: 30" toml:"plex_timeout_seconds"`
	// PlexDisableTLSVerify specifies whether to disable TLS certificate verification for Plex API calls - default: false
	PlexDisableTLSVerify bool `comment:"Disable TLS certificate verification for Plex API calls.\nWhen true, self-signed or invalid certificates are accepted" displayname:"Plex Disable TLS Verification" longcomment:"Disable TLS certificate verification for Plex API calls.\nWhen true, self-signed or invalid certificates are accepted.\nUseful for local Plex servers with self-signed certificates.\nSecurity risk: enables man-in-the-middle attacks.\nOnly enable for trusted local networks or development environments.\nDefault: false (verify certificates)" toml:"plex_disable_tls_verify"`

	// JellyfinLimiterSeconds defines seconds limit for Jellyfin API calls - default: 1
	JellyfinLimiterSeconds uint8 `comment:"Time window in seconds for Jellyfin API rate limiting.\nDefines the time period over which Jellyfin API call limits are applied" displayname:"Jellyfin Rate Limit Seconds" longcomment:"Time window in seconds for Jellyfin API rate limiting.\nDefines the time period over which Jellyfin API call limits are applied.\nWorks together with jellyfin_limiter_calls to prevent API rate limit violations.\nJellyfin servers typically have generous rate limits but may vary by setup.\nLower values provide more granular rate limiting control.\nDefault: 1 (one second time window)" toml:"jellyfin_limiter_seconds"`
	// JellyfinLimiterCalls defines calls limit for Jellyfin API in defined seconds - default: 10
	JellyfinLimiterCalls int `comment:"Maximum number of API calls allowed to Jellyfin within the defined time window.\nWorks with jellyfin_limiter_seconds" displayname:"Jellyfin Calls Per Window" longcomment:"Maximum number of API calls allowed to Jellyfin within the defined time window.\nWorks with jellyfin_limiter_seconds to enforce rate limiting.\nJellyfin servers typically handle many concurrent requests well.\nAdjust based on your Jellyfin server's performance and network capacity.\nConservative values prevent overwhelming your Jellyfin server.\nDefault: 10 (ten calls per time window)" toml:"jellyfin_limiter_calls"`
	// JellyfinTimeoutSeconds defines the HTTP timeout in seconds for Jellyfin API calls - default: 30
	JellyfinTimeoutSeconds uint16 `comment:"HTTP request timeout in seconds for Jellyfin API calls.\nMaximum time to wait for Jellyfin API" displayname:"Jellyfin Request Timeout Seconds" longcomment:"HTTP request timeout in seconds for Jellyfin API calls.\nMaximum time to wait for Jellyfin API responses before timing out.\nJellyfin servers may respond slower on older hardware or with large libraries.\nFavorites/watchlist operations typically complete quickly but vary by server load.\nBalance between patience for slow responses and quick error detection.\nTypical range: 10-60 seconds\nDefault: 30" toml:"jellyfin_timeout_seconds"`
	// JellyfinDisableTLSVerify specifies whether to disable TLS certificate verification for Jellyfin API calls - default: false
	JellyfinDisableTLSVerify bool `comment:"Disable TLS certificate verification for Jellyfin API calls.\nWhen true, self-signed or invalid certificates are accepted" displayname:"Jellyfin Disable TLS Verification" longcomment:"Disable TLS certificate verification for Jellyfin API calls.\nWhen true, self-signed or invalid certificates are accepted.\nUseful for local Jellyfin servers with self-signed certificates.\nSecurity risk: enables man-in-the-middle attacks.\nOnly enable for trusted local networks or development environments.\nDefault: false (verify certificates)" toml:"jellyfin_disable_tls_verify"`

	// Jobs To Run
	Jobs map[string]func(uint32, context.Context) error `json:"-" toml:"-"`
	// UseGoDir                           bool     `toml:"use_godir"`
	// ConcurrentScheduler                int      `toml:"concurrent_scheduler"`
	// EnableFileWatcher specifies whether the file watcher functionality is enabled
	// When set to true, the application will monitor specified directories for file changes
	// Default is false
	EnableFileWatcher bool `comment:"Enable automatic monitoring of configuration file changes.\nWhen true, the application will watch the config file" displayname:"Enable Configuration File Watcher" longcomment:"Enable automatic monitoring of configuration file changes.\nWhen true, the application will watch the config file and reload changes automatically.\nAllows configuration updates without restarting the application.\nUseful for live configuration adjustments during operation.\nMay consume additional system resources for file monitoring.\nDefault: false (manual restart required for config changes)" toml:"enable_file_watcher"`

	// UnrarPath specifies the path to the unrar executable for unpacking RAR archives
	UnrarPath string `comment:"Absolute path to the unrar executable for extracting RAR archives.\nSpecifying the full path improves performance" displayname:"Unrar Executable Path" longcomment:"Absolute path to the unrar executable for extracting RAR archives.\nSpecifying the full path improves performance by avoiding PATH searches.\nRequired for unpacking RAR archives before media file organization.\nDownload from: https://www.rarlab.com/download.htm\nExample: '/usr/bin/unrar' or 'C:\\Program Files\\WinRAR\\unrar.exe'\nDefault: 'unrar' (search in PATH)" toml:"unrar_path"`

	// SevenZipPath specifies the path to the 7z executable for unpacking various archive formats
	SevenZipPath string `comment:"Absolute path to the 7z executable for extracting multiple archive formats.\nSpecifying the full path improves performance" displayname:"7-Zip Executable Path" longcomment:"Absolute path to the 7z executable for extracting multiple archive formats.\nSpecifying the full path improves performance by avoiding PATH searches.\nSupports: ZIP, 7Z, RAR, TAR, GZIP, BZIP2, XZ and many other formats.\nRecommended as primary unpacker due to wide format support.\nDownload from: https://www.7-zip.org/download.html\nExample: '/usr/bin/7z' or 'C:\\Program Files\\7-Zip\\7z.exe'\nDefault: '7z' (search in PATH)" toml:"7zip_path"`

	// UnzipPath specifies the path to the unzip executable for unpacking ZIP archives
	UnzipPath string `comment:"Absolute path to the unzip executable for extracting ZIP archives.\nSpecifying the full path improves performance" displayname:"Unzip Executable Path" longcomment:"Absolute path to the unzip executable for extracting ZIP archives.\nSpecifying the full path improves performance by avoiding PATH searches.\nUsed as fallback for ZIP files if 7-Zip is not available.\nAvailable on most Unix-like systems by default.\nExample: '/usr/bin/unzip' or 'C:\\Windows\\System32\\tar.exe'\nDefault: 'unzip' (search in PATH)" toml:"unzip_path"`

	// TarPath specifies the path to the tar executable for unpacking TAR archives
	TarPath string `comment:"Absolute path to the tar executable for extracting TAR, TAR.GZ, TAR.BZ2 archives.\nSpecifying the full path improves performance" displayname:"Tar Executable Path" longcomment:"Absolute path to the tar executable for extracting TAR, TAR.GZ, TAR.BZ2 archives.\nSpecifying the full path improves performance by avoiding PATH searches.\nUsed for TAR-based archives including compressed variants.\nAvailable on most Unix-like systems and modern Windows.\nExample: '/usr/bin/tar' or 'C:\\Windows\\System32\\tar.exe'\nDefault: 'tar' (search in PATH)" toml:"tar_path"`

	// FpcalcPath specifies the path to the fpcalc executable for audio fingerprinting
	FpcalcPath string `comment:"Absolute path to the fpcalc (chromaprint) executable for audio fingerprinting.\nSpecifying the full path improves performance" displayname:"Fpcalc Executable Path" longcomment:"Absolute path to the fpcalc (chromaprint) executable for audio fingerprinting.\nSpecifying the full path improves performance by avoiding PATH searches.\nRequired for AcoustID audio identification and music fingerprinting.\nDownload chromaprint from: https://acoustid.org/chromaprint\nExample: '/usr/bin/fpcalc' or 'C:\\chromaprint\\fpcalc.exe'\nDefault: 'fpcalc' (search in PATH)" toml:"fpcalc_path"`

	//
	// Book/Audiobook/Music Provider API Keys and Settings
	//

	// GoodreadsAPIKey is the API key for Goodreads (deprecated but still partially functional)
	GoodreadsAPIKey string `comment:"API key for Goodreads book metadata service.\nNote: Goodreads API is deprecated but some endpoints still work" displayname:"Goodreads API Key" longcomment:"API key for Goodreads book metadata service.\nNote: Goodreads API is deprecated but some endpoints still function.\nGet a key at: https://www.goodreads.com/api/keys\nProvides book metadata, ratings, and reviews.\nRequired for Goodreads provider integration.\nLeave empty to disable Goodreads integration." toml:"goodreads_apikey"`

	// DiscogsToken is the personal access token for Discogs API
	DiscogsToken string `comment:"Personal access token for Discogs music database API.\nProvides higher rate limits than unauthenticated access" displayname:"Discogs Personal Access Token" longcomment:"Personal access token for Discogs music database API.\nProvides higher rate limits (60/min vs 25/min) than unauthenticated access.\nGenerate at: https://www.discogs.com/settings/developers\nUsed for music release metadata, artist info, and catalog lookups.\nOptional: Discogs works without authentication but with lower limits." toml:"discogs_token"`

	// SpotifyClientID is the client ID for Spotify Web API
	SpotifyClientID string `comment:"Spotify API credentials for music metadata.\nGet credentials from https://developer.spotify.com/dashboard" displayname:"Spotify Client ID" longcomment:"Client ID for Spotify Web API access.\nObtain by creating an application at: https://developer.spotify.com/dashboard\nUsed for music metadata, track info, and album lookups.\nRequires pairing with spotify_client_secret.\nLeave empty to disable Spotify integration." toml:"spotify_client_id"`

	// SpotifyClientSecret is the client secret for Spotify Web API
	SpotifyClientSecret string `comment:"Spotify API client secret (pairs with spotify_client_id)" displayname:"Spotify Client Secret" longcomment:"Client secret for Spotify Web API access.\nObtained from the same Spotify application as the client ID.\nKeep this value confidential - it grants API access.\nRequired along with spotify_client_id for Spotify features.\nLeave empty to disable Spotify integration." toml:"spotify_client_secret"`

	// SpotifyRegion is the ISO 3166-1 alpha-2 country code for market filtering
	SpotifyRegion string `comment:"Spotify region/market filter (ISO 3166-1 alpha-2 country code, e.g., 'US', 'GB')" displayname:"Spotify Region/Market" longcomment:"Market/region filter for Spotify search results.\nISO 3166-1 alpha-2 country code (e.g., 'US', 'GB', 'DE', 'JP').\nFilters search results to only show albums/tracks available in that market.\nLeave empty to search globally without market restrictions.\nUseful for matching region-specific releases." toml:"spotify_region"`

	// AcoustIDAPIKey is the API key for AcoustID audio fingerprint lookups
	AcoustIDAPIKey string `comment:"API key for AcoustID audio fingerprint identification service.\nRequired for identifying music by audio fingerprint" displayname:"AcoustID API Key" longcomment:"API key for AcoustID audio fingerprint identification service.\nRequired for identifying music files by their audio fingerprint.\nRegister at: https://acoustid.org/api-key\nWorks with chromaprint/fpcalc to match unknown audio files.\nRequired for AcoustID provider integration.\nLeave empty to disable fingerprint-based identification." toml:"acoustid_apikey"`

	// LastFMAPIKey is the API key for Last.fm - get one at https://www.last.fm/api/account/create
	LastFMAPIKey string `comment:"API key for Last.fm music metadata and chart service.\nRequired for chart.getTopArtists, artist.getInfo, album.getInfo, and similar endpoints" displayname:"Last.fm API Key" longcomment:"API key for Last.fm music metadata and chart service.\nRequired for all Last.fm API calls including:\n- chart.getTopArtists / chart.getTopTracks (global charts)\n- geo.getTopArtists (charts by country)\n- tag.getTopAlbums (charts by genre)\n- artist.getInfo / album.getInfo (metadata lookups)\n- artist.search / album.search\nGet a free key at: https://www.last.fm/api/account/create\nLeave empty to disable Last.fm integration." toml:"lastfm_apikey"`

	//
	// Book/Audiobook/Music Provider Rate Limits
	//

	// OpenLibraryLimiterSeconds defines seconds limit for OpenLibrary API calls - default: 1
	OpenLibraryLimiterSeconds uint8 `comment:"Time window in seconds for OpenLibrary API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"OpenLibrary Rate Limit Seconds" longcomment:"Time window in seconds for OpenLibrary API rate limiting.\nDefines the time period over which OpenLibrary API call limits are applied.\nWorks together with openlibrary_limiter_calls to prevent rate limit violations.\nOpenLibrary is a free service - be respectful with request frequency.\nDefault: 1 (one second time window)" toml:"openlibrary_limiter_seconds"`

	// OpenLibraryLimiterCalls defines calls limit for OpenLibrary API in defined seconds - default: 5
	OpenLibraryLimiterCalls int `comment:"Maximum number of API calls allowed to OpenLibrary within the defined time window.\nWorks with openlibrary_limiter_seconds" displayname:"OpenLibrary Calls Per Window" longcomment:"Maximum number of API calls allowed to OpenLibrary within the defined time window.\nWorks with openlibrary_limiter_seconds to enforce rate limiting.\nOpenLibrary requests reasonable usage without hard limits.\nConservative rate limiting ensures continued access.\nDefault: 5 (five calls per time window)" toml:"openlibrary_limiter_calls"`

	// GoodreadsLimiterSeconds defines seconds limit for Goodreads API calls - default: 1
	GoodreadsLimiterSeconds uint8 `comment:"Time window in seconds for Goodreads API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Goodreads Rate Limit Seconds" longcomment:"Time window in seconds for Goodreads API rate limiting.\nDefines the time period over which Goodreads API call limits are applied.\nWorks together with goodreads_limiter_calls to prevent rate limit violations.\nGoodreads API is deprecated - conservative limits recommended.\nDefault: 1 (one second time window)" toml:"goodreads_limiter_seconds"`

	// GoodreadsLimiterCalls defines calls limit for Goodreads API in defined seconds - default: 1
	GoodreadsLimiterCalls int `comment:"Maximum number of API calls allowed to Goodreads within the defined time window.\nWorks with goodreads_limiter_seconds" displayname:"Goodreads Calls Per Window" longcomment:"Maximum number of API calls allowed to Goodreads within the defined time window.\nWorks with goodreads_limiter_seconds to enforce rate limiting.\nGoodreads API is deprecated with 1 request/second limit.\nDefault: 1 (one call per time window)" toml:"goodreads_limiter_calls"`

	// AudibleLimiterSeconds defines seconds limit for Audible API calls - default: 1
	AudibleLimiterSeconds uint8 `comment:"Time window in seconds for Audible API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Audible Rate Limit Seconds" longcomment:"Time window in seconds for Audible API rate limiting.\nDefines the time period over which Audible API call limits are applied.\nWorks together with audible_limiter_calls to prevent blocking.\nAudible may block aggressive scrapers - use conservative limits.\nDefault: 1 (one second time window)" toml:"audible_limiter_seconds"`

	// AudibleLimiterCalls defines calls limit for Audible API in defined seconds - default: 5
	AudibleLimiterCalls int `comment:"Maximum number of API calls allowed to Audible within the defined time window.\nWorks with audible_limiter_seconds" displayname:"Audible Calls Per Window" longcomment:"Maximum number of API calls allowed to Audible within the defined time window.\nWorks with audible_limiter_seconds to enforce rate limiting.\nAudible doesn't publish official limits but may block aggressive usage.\nDefault: 5 (five calls per time window)" toml:"audible_limiter_calls"`

	// AudnexLimiterSeconds defines seconds limit for Audnex API calls - default: 1
	AudnexLimiterSeconds uint8 `comment:"Time window in seconds for Audnex API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Audnex Rate Limit Seconds" longcomment:"Time window in seconds for Audnex API rate limiting.\nDefines the time period over which Audnex API call limits are applied.\nWorks together with audnex_limiter_calls to prevent rate limit violations.\nAudnex provides enhanced audiobook metadata including chapters.\nDefault: 1 (one second time window)" toml:"audnex_limiter_seconds"`

	// AudnexLimiterCalls defines calls limit for Audnex API in defined seconds - default: 10
	AudnexLimiterCalls int `comment:"Maximum number of API calls allowed to Audnex within the defined time window.\nWorks with audnex_limiter_seconds" displayname:"Audnex Calls Per Window" longcomment:"Maximum number of API calls allowed to Audnex within the defined time window.\nWorks with audnex_limiter_seconds to enforce rate limiting.\nAudnex is a community service - use reasonable request frequency.\nDefault: 10 (ten calls per time window)" toml:"audnex_limiter_calls"`

	// MusicBrainzLimiterSeconds defines seconds limit for MusicBrainz API calls - default: 1
	MusicBrainzLimiterSeconds uint8 `comment:"Time window in seconds for MusicBrainz API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"MusicBrainz Rate Limit Seconds" longcomment:"Time window in seconds for MusicBrainz API rate limiting.\nDefines the time period over which MusicBrainz API call limits are applied.\nWorks together with musicbrainz_limiter_calls to prevent rate limit violations.\nMusicBrainz requires 1 request/second with proper User-Agent.\nDefault: 1 (one second time window)" toml:"musicbrainz_limiter_seconds"`

	// MusicBrainzLimiterCalls defines calls limit for MusicBrainz API in defined seconds - default: 1
	MusicBrainzLimiterCalls int `comment:"Maximum number of API calls allowed to MusicBrainz within the defined time window.\nWorks with musicbrainz_limiter_seconds" displayname:"MusicBrainz Calls Per Window" longcomment:"Maximum number of API calls allowed to MusicBrainz within the defined time window.\nWorks with musicbrainz_limiter_seconds to enforce rate limiting.\nMusicBrainz enforces 1 request/second - higher values will be throttled.\nDefault: 1 (one call per time window)" toml:"musicbrainz_limiter_calls"`

	// DiscogsLimiterSeconds defines seconds limit for Discogs API calls - default: 60
	DiscogsLimiterSeconds uint8 `comment:"Time window in seconds for Discogs API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Discogs Rate Limit Seconds" longcomment:"Time window in seconds for Discogs API rate limiting.\nDefines the time period over which Discogs API call limits are applied.\nWorks together with discogs_limiter_calls to prevent rate limit violations.\nDiscogs allows 60 requests/minute (authenticated) or 25/minute (unauthenticated).\nDefault: 60 (sixty second time window)" toml:"discogs_limiter_seconds"`

	// DiscogsLimiterCalls defines calls limit for Discogs API in defined seconds - default: 60
	DiscogsLimiterCalls int `comment:"Maximum number of API calls allowed to Discogs within the defined time window.\nWorks with discogs_limiter_seconds" displayname:"Discogs Calls Per Window" longcomment:"Maximum number of API calls allowed to Discogs within the defined time window.\nWorks with discogs_limiter_seconds to enforce rate limiting.\nDiscogs allows 60/min authenticated, 25/min unauthenticated.\nDefault: 60 (sixty calls per time window when authenticated)" toml:"discogs_limiter_calls"`

	// AcoustIDLimiterSeconds defines seconds limit for AcoustID API calls - default: 1
	AcoustIDLimiterSeconds uint8 `comment:"Time window in seconds for AcoustID API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"AcoustID Rate Limit Seconds" longcomment:"Time window in seconds for AcoustID API rate limiting.\nDefines the time period over which AcoustID API call limits are applied.\nWorks together with acoustid_limiter_calls to prevent rate limit violations.\nAcoustID allows approximately 3 requests/second.\nDefault: 1 (one second time window)" toml:"acoustid_limiter_seconds"`

	// AcoustIDLimiterCalls defines calls limit for AcoustID API in defined seconds - default: 3
	AcoustIDLimiterCalls int `comment:"Maximum number of API calls allowed to AcoustID within the defined time window.\nWorks with acoustid_limiter_seconds" displayname:"AcoustID Calls Per Window" longcomment:"Maximum number of API calls allowed to AcoustID within the defined time window.\nWorks with acoustid_limiter_seconds to enforce rate limiting.\nAcoustID recommends approximately 3 requests/second.\nDefault: 3 (three calls per time window)" toml:"acoustid_limiter_calls"`

	// LastFMLimiterSeconds defines seconds limit for Last.fm API calls - default: 1
	LastFMLimiterSeconds uint8 `comment:"Time window in seconds for Last.fm API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Last.fm Rate Limit Seconds" longcomment:"Time window in seconds for Last.fm API rate limiting.\nDefines the time period over which Last.fm API call limits are applied.\nWorks together with lastfm_limiter_calls to prevent rate limit violations.\nLast.fm allows approximately 5 requests/second for authenticated keys.\nDefault: 1 (one second time window)" toml:"lastfm_limiter_seconds"`

	// LastFMLimiterCalls defines calls limit for Last.fm API in defined seconds - default: 5
	LastFMLimiterCalls int `comment:"Maximum number of API calls allowed to Last.fm within the defined time window.\nWorks with lastfm_limiter_seconds" displayname:"Last.fm Calls Per Window" longcomment:"Maximum number of API calls allowed to Last.fm within the defined time window.\nWorks with lastfm_limiter_seconds to enforce rate limiting.\nLast.fm recommends 5 requests/second per API key.\nDefault: 5 (five calls per time window)" toml:"lastfm_limiter_calls"`

	// ITunesLimiterSeconds defines seconds limit for iTunes API calls - default: 60
	ITunesLimiterSeconds uint8 `comment:"Time window in seconds for iTunes API rate limiting." displayname:"iTunes Rate Limit Seconds" longcomment:"Time window in seconds for iTunes Search API rate limiting.\nDefault: 60 (sixty second time window)" toml:"itunes_limiter_seconds"`

	// ITunesLimiterCalls defines calls limit for iTunes API in defined seconds - default: 20
	ITunesLimiterCalls int `comment:"Maximum number of API calls allowed to iTunes within the defined time window." displayname:"iTunes Calls Per Window" longcomment:"Maximum number of API calls allowed to the iTunes Search API within the defined time window.\nApple does not publish a hard rate limit; 20 requests per minute is a safe conservative default.\nDefault: 20 (twenty calls per time window)" toml:"itunes_limiter_calls"`

	// TheAudioDBAPIKey defines the API key for TheAudioDB - defaults to the free public key "2"
	TheAudioDBAPIKey string `comment:"API key for TheAudioDB. Leave empty to use the free public key (key 123).\nA paid key unlocks higher-resolution artwork and additional endpoints." displayname:"TheAudioDB API Key" toml:"theaudiodb_api_key"`

	// TheAudioDBLimiterSeconds defines seconds limit for TheAudioDB API calls - default: 1
	TheAudioDBLimiterSeconds uint8 `comment:"Time window in seconds for TheAudioDB API rate limiting." displayname:"TheAudioDB Rate Limit Seconds" longcomment:"Time window in seconds for TheAudioDB API rate limiting.\nDefault: 1 (one second time window)" toml:"theaudiodb_limiter_seconds"`

	// TheAudioDBLimiterCalls defines calls limit for TheAudioDB API in defined seconds - default: 2
	TheAudioDBLimiterCalls int `comment:"Maximum number of API calls allowed to TheAudioDB within the defined time window.\nWorks with theaudiodb_limiter_seconds" displayname:"TheAudioDB Calls Per Window" longcomment:"Maximum number of API calls allowed to TheAudioDB within the defined time window.\nThe free tier (key 2) is rate-limited — 2 calls/sec is a safe default.\nDefault: 2 (two calls per time window)" toml:"theaudiodb_limiter_calls"`

	// DeezerLimiterSeconds defines seconds limit for Deezer API calls - default: 5
	DeezerLimiterSeconds uint8 `comment:"Time window in seconds for Deezer API rate limiting.\nDefines the time period over which API call limits are applied" displayname:"Deezer Rate Limit Seconds" longcomment:"Time window in seconds for Deezer API rate limiting.\nDefines the time period over which Deezer API call limits are applied.\nWorks together with deezer_limiter_calls to prevent rate limit violations.\nDeezer allows approximately 50 requests per 5 seconds (unauthenticated).\nDefault: 5 (five second time window)" toml:"deezer_limiter_seconds"`

	// DeezerLimiterCalls defines calls limit for Deezer API in defined seconds - default: 50
	DeezerLimiterCalls int `comment:"Maximum number of API calls allowed to Deezer within the defined time window.\nWorks with deezer_limiter_seconds" displayname:"Deezer Calls Per Window" longcomment:"Maximum number of API calls allowed to Deezer within the defined time window.\nWorks with deezer_limiter_seconds to enforce rate limiting.\nDeezer allows approximately 50 requests per 5 seconds (public API).\nDefault: 50 (fifty calls per time window)" toml:"deezer_limiter_calls"`

	// MusicMetaSourcePriority controls which music metadata providers are active and in what order.
	// An empty list enables all providers in the default order: musicbrainz → acoustid → lastfm → discogs → deezer.
	// To disable a provider, omit it from the list.
	// Note: removing 'musicbrainz' severely limits matching — it is the track-listing source
	// for candidates not yet fully populated in the database.
	MusicMetaSourcePriority []string `comment:"Active music metadata providers in priority order.\nEmpty = all available providers in default order (musicbrainz → acoustid → lastfm → discogs → deezer → theaudiodb → itunes).\nTo disable a provider, omit it from the list." displayname:"Music Metadata Source Priority" longcomment:"Controls which music metadata providers are active and the order they are tried.\nEmpty list (default): all available providers are used in the default order:\n  musicbrainz → acoustid → lastfm → discogs → deezer → theaudiodb → itunes\nTo disable a provider, simply omit it:\n  ['musicbrainz', 'lastfm', 'discogs'] - skip AcoustID fingerprinting\n  ['musicbrainz', 'discogs'] - skip both AcoustID and Last.fm\nNote: 'acoustid' and 'lastfm' also require their API keys to be configured.\nWarning: removing 'musicbrainz' from the list will break track-listing lookups\nfor albums not yet fully populated in the local database." multiline:"true" toml:"music_meta_source_priority"`
}

// ImdbConfig defines the configuration for the IMDb indexer.
type ImdbConfig struct {
	// Indexedtypes is an array of strings specifying the types of IMDb media to import
	// Valid values are 'movie', 'tvMovie', 'tvmovie', 'tvSeries', 'tvseries', 'video'
	// Default is empty array which imports nothing
	Indexedtypes []string `comment:"Specify which types of media to import from the IMDB database.\nThis controls what content types" displayname:"Media Types To Index" longcomment:"Specify which types of media to import from the IMDB database.\nThis controls what content types are downloaded and indexed locally.\nValid options:\n- 'movie': Feature films and theatrical releases\n- 'tvMovie': Made-for-TV movies and TV films\n- 'tvSeries': TV shows, series, and episodic content\n- 'video': Direct-to-video releases, web series, shorts\nExample: ['movie', 'tvSeries'] to import only movies and TV shows\nLeave empty to disable IMDB indexing entirely\nWarning: Importing all types requires significant disk space (10+ GB)" multiline:"true" toml:"indexed_types"`

	// Indexedlanguages is an array of strings specifying the languages to use for titles
	// Examples: "DE", "UK", "US"
	// Include '' or '\N' for global titles
	// Default is empty array which imports all languages
	Indexedlanguages []string `comment:"Filter IMDB titles by language/region to reduce database size.\nSpecify which languages and regions you want" displayname:"Languages To Index" longcomment:"Filter IMDB titles by language/region to reduce database size.\nSpecify which languages and regions you want to include in your local IMDB index.\nUse standard region codes for language variants:\n- 'US': English (United States)\n- 'GB' or 'UK': English (United Kingdom)\n- 'DE': German titles\n- 'FR': French titles\n- 'ES': Spanish titles\n- '': Include titles without specific language designation (original/international)\nExample: ['US', 'GB', ''] for English titles plus international\nLeave empty to import all languages (requires more storage)" multiline:"true" toml:"indexed_languages"`

	// Indexfull is a boolean specifying whether to index all available IMDb data
	// or only the bare minimum
	// Default is false
	Indexfull bool `comment:"Import complete IMDB dataset versus minimal data for basic functionality.\nWhen true, imports comprehensive data including" displayname:"Import Complete Dataset" longcomment:"Import complete IMDB dataset versus minimal data for basic functionality.\nWhen true, imports comprehensive data including cast, crew, ratings, plots, etc.\nWhen false, imports only essential data needed for media matching and identification.\nFull indexing provides richer metadata but requires significantly more:\n- Disk space: 50+ GB vs 2-5 GB for minimal\n- RAM usage: 4+ GB vs 1-2 GB during import\n- Import time: Hours vs minutes\nRecommended: false unless you need comprehensive IMDB metadata\nDefault: false (minimal data only)" toml:"index_full"`

	// ImdbIDSize is an integer specifying the number of expected entries in the IMDb database
	// Default is 12000000
	ImdbIDSize int `comment:"Estimated total number of entries in the IMDB database for memory pre-allocation.\nThis helps optimize memory" displayname:"Expected Database Entry Count" longcomment:"Estimated total number of entries in the IMDB database for memory pre-allocation.\nThis helps optimize memory allocation during the import process.\nShould be set higher than the actual expected number of entries.\nIMDB contains approximately:\n- 10+ million titles (all types)\n- 12+ million entries when including episodes\nIncreasing this value uses more memory but prevents reallocations.\nDecreasing saves memory but may cause performance issues if too low.\nRecommended range: 10,000,000 - 15,000,000\nDefault: 12,000,000" toml:"imdbid_size"`

	// LoopSize is an integer specifying the number of entries to keep in RAM for cached queries
	// Default is 400000
	LoopSize int `comment:"Number of IMDB entries to keep in RAM cache for fast query performance.\nHigher values improve" displayname:"RAM Cache Entry Count" longcomment:"Number of IMDB entries to keep in RAM cache for fast query performance.\nHigher values improve lookup speed but use more memory.\nThis cache stores frequently accessed IMDB data in memory.\nMemory usage scales roughly: (loop_size × 1KB) per entry.\nRecommended values based on system RAM:\n- 2GB RAM: 200,000 - 400,000 entries\n- 4GB RAM: 400,000 - 800,000 entries\n- 8GB+ RAM: 800,000+ entries\nBalance between query performance and available memory\nDefault: 400,000" toml:"loop_size"`

	// UseMemory is a boolean specifying whether to store the IMDb DB in RAM during generation
	// At least 2GB RAM required. Highly recommended.
	// Default is false
	UseMemory bool `comment:"Store the entire IMDB database in RAM during import for dramatically faster processing.\nWhen true, the complete dataset is loaded into memory during import" displayname:"Store Database In RAM" longcomment:"Store the entire IMDB database in RAM during import for dramatically faster processing.\nWhen true, the complete dataset is loaded into memory during import.\nThis provides significant performance improvements but requires substantial RAM.\nMemory requirements:\n- Minimal import: 0,5-2 GB RAM\n- Full import: 1-4 GB RAM\nBenefits: 10-50x faster import times, reduced disk I/O\nDrawbacks: High memory usage, system may become unresponsive if insufficient RAM\nHighly recommended if you have sufficient available memory\nDefault: false (use disk-based processing)" toml:"use_memory"`

	// UseCache is a boolean specifying whether to use caching for SQL queries
	// Might reduce execution time
	// Default is false
	UseCache bool `comment:"Enable SQL query result caching to improve IMDB database query performance.\nWhen true, frequently executed queries" displayname:"Enable SQL Query Caching" longcomment:"Enable SQL query result caching to improve IMDB database query performance.\nWhen true, frequently executed queries are cached in memory.\nReduces database load and improves response times for repeated queries.\nMost beneficial when the same IMDB searches are performed repeatedly.\nCache memory usage grows over time with unique queries.\nRecommended for systems with ample RAM and heavy IMDB usage.\nDefault: false (no query caching)" toml:"use_cache"`
}

// MediaConfig defines the configuration for media types like series and movies.
type MediaConfig struct {
	// Series defines the configuration for all series media types
	Series []MediaTypeConfig `comment:"Configuration definitions for all your TV series and episodic content.\nEach entry defines a separate media" displayname:"TV Series Media Groups" longcomment:"Configuration definitions for all your TV series and episodic content.\nEach entry defines a separate media group with its own settings for:\n- Quality profiles and search preferences\n- Storage paths and file organization\n- Indexers and download clients to use\n- Notification settings for new episodes\n- List integrations and metadata sources\nYou can create multiple series groups for different types:\n- Regular TV shows, anime, documentaries, etc.\n- Different quality requirements (4K vs HD)\n- Separate storage locations or indexers\nExample: Create 'tv-hd' and 'tv-4k' groups with different quality profiles" toml:"series"`
	// Movies defines the configuration for all movies media types
	Movies     []MediaTypeConfig `comment:"Configuration definitions for all your movie collections and film content.\nEach entry defines a separate media" displayname:"Movie Media Groups" longcomment:"Configuration definitions for all your movie collections and film content.\nEach entry defines a separate media group with its own settings for:\n- Quality profiles and search preferences\n- Storage paths and file organization\n- Indexers and download clients to use\n- Notification settings for new releases\n- List integrations and metadata sources\nYou can create multiple movie groups for different purposes:\n- Different genres (action, documentaries, foreign films)\n- Quality tiers (4K, HD, standard definition)\n- Separate storage locations or collection types\nExample: Create 'movies-4k' and 'movies-hd' groups with different storage paths" toml:"movies"`
	Books      []MediaTypeConfig
	AudioBooks []MediaTypeConfig
	Music      []MediaTypeConfig
}

// MediaTypeConfig defines the configuration for a media type like movies or series.
type MediaTypeConfig struct {
	// Name is the name of the media group - keep it unique
	Name string `comment:"Unique identifier name for this media group configuration.\nThis name is used throughout the application to" displayname:"Media Group Name" longcomment:"Unique identifier name for this media group configuration.\nThis name is used throughout the application to reference this specific media setup.\nMust be unique across all media groups (both series and movies).\nChoose descriptive names that indicate the purpose or characteristics:\n- Content type: 'anime', 'documentaries', 'foreign-films'\n- Quality tier: 'movies-4k', 'tv-hd', 'series-sd'\n- Storage location: 'nas-movies', 'local-tv'\nUsed in logs, web interface, and when configuring other sections.\nExample: 'movies-uhd' for a 4K movie collection" toml:"name"`

	// NamePrefix is not set in the TOML config
	NamePrefix string `toml:"-"`

	IsType uint `toml:"-"`

	// DefaultQuality is the default quality to assume if none was found - keep it low
	DefaultQuality string `comment:"Default quality level to assign when release quality cannot be determined.\nUsed as a fallback when" displayname:"Fallback Quality Level" longcomment:"Default quality level to assign when release quality cannot be determined.\nUsed as a fallback when parsing fails to detect the actual quality.\nShould be set to a conservative (lower) quality to avoid false upgrades.\nMust match one of the qualities defined in your quality profile.\nCommon conservative defaults:\n- For movies: 'HDTV' or 'DVD'\n- For TV series: 'HDTV' or 'WEBDL'\nAvoid using high-end qualities like 'BluRay' or '4K' as defaults.\nExample: 'HDTV' (safe, commonly available quality)" toml:"default_quality"`

	// DefaultResolution is the default resolution to assume if none was found - keep it low
	DefaultResolution string `comment:"Default video resolution to assign when release resolution cannot be determined.\nUsed as a fallback when" displayname:"Fallback Video Resolution" longcomment:"Default video resolution to assign when release resolution cannot be determined.\nUsed as a fallback when parsing fails to detect the actual resolution.\nShould be set to a conservative (lower) resolution to avoid false upgrades.\nMust match one of the resolutions defined in your quality profile.\nCommon conservative defaults:\n- For older content: '480p' or '576p'\n- For modern content: '720p'\nAvoid using high resolutions like '1080p' or '2160p' as defaults.\nExample: '720p' (safe, widely available resolution)" toml:"default_resolution"`

	// Naming is the naming scheme for files - see wiki for details
	Naming string `comment:"File and folder naming template for organized media files. \nDefines how downloaded files will be renamed" displayname:"File Naming Template" longcomment:"File and folder naming template for organized media files. \nDefines how downloaded files will be renamed and organized in your library. \nUses template variables that get replaced with actual media information. \nCommon variables include title, year, quality, resolution, codec, etc. \nDifferent templates for movies vs TV series are typical. \nExample movie template: '{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}{{if eq .Source.Extended true}} extended{{end}}] ({{.Source.Imdb}})'\nExample series template: '{{.Dbserie.Seriename}}/Season {{.DbserieEpisode.Season}}/{{.Dbserie.Seriename}} - S{{printf \"%02s\" .DbserieEpisode.Season}}{{range .Episodes}}E{{printf \"%02d\" . }}{{end}} - {{.DbserieEpisode.Title}} [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper true}} proper{{end}}] ({{.Source.Tvdb}})'\nSee documentation for complete variable list and examples:\nhttps://github.com/Kellerman81/go_media_downloader/wiki/Groups" toml:"naming"`

	// TemplateQuality is the name of the quality template to use
	TemplateQuality string `comment:"Name of the quality profile template to use for this media group.\nReferences a quality configuration" displayname:"Quality Profile Reference" longcomment:"Name of the quality profile template to use for this media group.\nReferences a quality configuration defined in the quality section.\nThe quality profile controls:\n- Preferred video resolutions (720p, 1080p, 4K, etc.)\n- Desired source quality (BluRay, WEB-DL, HDTV, etc.)\n- Audio and video codec preferences\n- Search behavior and upgrade criteria\n- Indexer-specific settings\nMust exactly match the 'name' field of a quality configuration.\nExample: 'uhd-quality' to use a 4K-focused quality profile" toml:"template_quality"`

	// CfgQuality is the parsed quality config (not set in TOML)
	CfgQuality *QualityConfig `toml:"-"`

	// TemplateScheduler is the name of the scheduler template to use
	TemplateScheduler string `comment:"Name of the scheduler template to use for this media group's automated tasks.\nReferences a scheduler" displayname:"Scheduler Template Reference" longcomment:"Name of the scheduler template to use for this media group's automated tasks.\nReferences a scheduler configuration defined in the scheduler section.\nThe scheduler controls:\n- How often to search for missing episodes/movies\n- When to perform quality upgrade scans\n- RSS feed check intervals\n- Maintenance task timing\nMust exactly match the 'name' field of a scheduler configuration.\nLeave empty to use default scheduler behavior.\nExample: 'aggressive-schedule' for frequent searches" toml:"template_scheduler"`

	// CfgScheduler is the parsed scheduler config (not set in TOML)
	CfgScheduler *SchedulerConfig `toml:"-"`

	// MetadataLanguage is the default language for metadata
	MetadataLanguage string `comment:"Primary language code for metadata retrieval and display.\nControls the language used when fetching information from" displayname:"Primary Metadata Language" longcomment:"Primary language code for metadata retrieval and display.\nControls the language used when fetching information from TMDB, TVDB, etc.\nAffects plot summaries, descriptions, and other text metadata.\nUse standard ISO 639-1 two-letter language codes:\n- 'en': English\n- 'de': German\n- 'fr': French\n- 'es': Spanish\n- 'ja': Japanese\nMetadata sources may not have all languages available.\nExample: 'en' for English metadata" toml:"metadata_language"`

	// MetadataTitleLanguages are the languages to import titles in
	MetadataTitleLanguages []string `comment:"List of language/region codes for importing alternate titles.\nCollects titles in multiple languages to improve release" displayname:"Alternate Title Languages" longcomment:"List of language/region codes for importing alternate titles.\nCollects titles in multiple languages to improve release matching.\nUseful for finding releases with foreign or alternate names.\nUse ISO country/language codes:\n- 'en': English (generic)\n- 'us': English (United States)\n- 'gb' or 'uk': English (United Kingdom)\n- 'de': German titles\n- 'jp': Japanese titles\nMore languages = better release matching but increased processing time.\nExample: ['en', 'us', 'de'] for English and German titles" multiline:"true" toml:"metadata_title_languages"`

	// MetadataTitleLanguagesLen is the number of title languages (not set in TOML)
	MetadataTitleLanguagesLen int `toml:"-"`

	// Structure indicates whether to structure media after download
	Structure bool `comment:"Enable automatic file organization and renaming after download completion.\nWhen true, downloaded files are automatically:\n- Moved" displayname:"Auto File Organization" longcomment:"Enable automatic file organization and renaming after download completion.\nWhen true, downloaded files are automatically:\n- Moved to proper library locations\n- Renamed according to the naming template\n- Organized into appropriate folder structures\n- Have metadata and artwork added\nWhen false, files remain in download location with original names.\nRequires proper path configuration in the data section.\nRecommended: true for organized media libraries\nDefault: false" toml:"structure"`

	// SearchmissingIncremental is the number of entries to process in incremental missing scans
	SearchmissingIncremental uint16 `comment:"Number of missing items to search for in each incremental scan cycle.\nIncremental scans process a" displayname:"Missing Items Per Scan" longcomment:"Number of missing items to search for in each incremental scan cycle.\nIncremental scans process a limited number of items per run to avoid overwhelming indexers.\nLower values are gentler on indexers but take longer to process large backlogs.\nHigher values process backlogs faster but may trigger rate limits.\nBalance based on your indexer limits and how quickly you want missing content found.\nTypical range: 10-50 items per scan\nExample: 20 (process 20 missing items per scheduled scan)\nDefault: 20" toml:"search_missing_incremental"`

	// SearchupgradeIncremental is the number of entries to process in incremental upgrade scans
	SearchupgradeIncremental uint16 `comment:"Number of existing items to check for quality upgrades in each scan cycle.\nIncremental upgrade scans" displayname:"Upgrade Items Per Scan" longcomment:"Number of existing items to check for quality upgrades in each scan cycle.\nIncremental upgrade scans look for better quality versions of existing media.\nLower values reduce indexer load but slower upgrade discovery.\nHigher values find upgrades faster but consume more indexer API calls.\nUpgrade scans are typically less urgent than missing content searches.\nConsider setting lower than search_missing_incremental.\nTypical range: 5-30 items per scan\nExample: 15 (check 15 items for upgrades per scheduled scan)\nDefault: 20" toml:"search_upgrade_incremental"`

	// Data contains the media data configs
	Data    []MediaDataConfig        `comment:"Storage path configurations for this media group.\nDefines where media files will be stored and how" displayname:"Storage Path Configurations" longcomment:"Storage path configurations for this media group.\nDefines where media files will be stored and how they're organized.\nEach entry specifies:\n- Path template reference (from paths section)\n- Minimum and maximum file sizes\n- Upgrade behavior and file management rules\nMultiple data entries allow different storage tiers or locations.\nExample: Separate entries for different quality levels or storage devices.\nRequired: At least one data configuration must be defined." toml:"data"`
	DataMap map[int]*MediaDataConfig `                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               toml:"-"`

	// DataLen is the number of data configs (not set in TOML)
	DataLen int `toml:"-"`

	// DataImport contains media data import configs
	DataImport    []MediaDataImportConfig        `comment:"Configuration for importing existing media files into the library.\nDefines how to scan and process media" displayname:"Import Path Configurations" longcomment:"Configuration for importing existing media files into the library.\nDefines how to scan and process media files already on your system.\nEach entry specifies:\n- Paths to scan for existing media\n- Import behavior and file handling rules\n- Whether to move, copy, or link existing files\nUseful for migrating from other media managers or adding existing collections.\nOptional: Only needed if importing existing media files." toml:"data_import"`
	DataImportMap map[int]*MediaDataImportConfig `                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    toml:"-"`

	// Lists contains media lists configs
	Lists []MediaListsConfig `comment:"External list integrations for automatic media discovery.\nConnects to various sources to automatically add new media" displayname:"External List Integrations" longcomment:"External list integrations for automatic media discovery.\nConnects to various sources to automatically add new media to your wanted list.\nEach entry can reference:\n- List template configurations (from lists section)\n- IMDB lists, Trakt lists, RSS feeds\n- Custom lists and watchlists\nAllows automatic discovery of new releases based on your preferences.\nOptional: Only needed if using automatic list-based media discovery." toml:"lists"`

	// ListsMap is a map of the lists configs (not set in TOML)
	ListsMap    map[string]*MediaListsConfig `toml:"-"`
	ListsMapIdx map[string]int               `toml:"-"`

	// ListsQu is the quality from the lists config (not set in TOML)
	ListsQu string `toml:"-"`

	// ListsLen is the number of lists configs (not set in TOML)
	ListsLen int `toml:"-"`

	// ListsQualities are the quality strings from lists (not set in TOML)
	ListsQualities []string `toml:"-"`

	// Notification contains notification configs
	Notification []MediaNotificationConfig `comment:"Notification settings for this specific media group.\nDefines how and when you'll be alerted about media" displayname:"Notification Configurations" longcomment:"Notification settings for this specific media group.\nDefines how and when you'll be alerted about media events.\nEach entry can reference:\n- Notification template configurations (from [notification] section)\n- Events to notify about (downloads, upgrades, failures)\n- Specific notification channels (Pushover, email, webhooks)\nAllows different notification preferences per media type.\nOptional: Only needed if you want notifications for this media group." toml:"notification"`

	// AudibleRegion specifies the Audible marketplace region for audiobook metadata
	// Valid values: us, uk, ca, au, de, fr, it, es, in, jp
	AudibleRegion string `comment:"Audible marketplace region for audiobook metadata lookups.\nDetermines which Audible catalog to search" displayname:"Audible Region" longcomment:"Audible marketplace region for audiobook metadata lookups.\nDetermines which Audible catalog to search for audiobook information.\nValid options:\n- 'us': United States (audible.com)\n- 'uk': United Kingdom (audible.co.uk)\n- 'ca': Canada (audible.ca)\n- 'au': Australia (audible.com.au)\n- 'de': Germany (audible.de)\n- 'fr': France (audible.fr)\n- 'it': Italy (audible.it)\n- 'es': Spain (audible.es)\n- 'in': India (audible.in)\n- 'jp': Japan (audible.co.jp)\nUsed for audiobook media types to match regional catalogs.\nDefault: 'us' (United States)" toml:"audible_region"`

	// Jobs To Run
	Jobs map[string]func(uint32, context.Context) error `json:"-" toml:"-"`
}

// MediaDataConfig is a struct that defines configuration for media data.
type MediaDataConfig struct {
	// TemplatePath is the template to use for the path
	TemplatePath string `comment:"Name of the path configuration template to use for media storage.\nReferences a path configuration defined" displayname:"Storage Path Template" longcomment:"Name of the path configuration template to use for media storage.\nReferences a path configuration defined in the paths section.\nThe path template controls:\n- Root storage directory for this media\n- File extensions and size limits\n- File organization and naming rules\n- Upgrade and cleanup behavior\nMust exactly match the 'name' field of a paths configuration.\nDifferent data entries can use different path templates for storage tiers.\nExample: 'movies-4k-storage' for a high-capacity 4K movie path" toml:"template_path"`
	// CfgPath is a pointer to PathsConfig
	CfgPath *PathsConfig `toml:"-"`
	// AddFound indicates if entries not in watched media should be added if found
	// Default is false
	AddFound bool `comment:"Automatically add discovered media to your wanted list when found during scans.\nWhen true, media files" displayname:"Auto Add Discovered Media" longcomment:"Automatically add discovered media to your wanted list when found during scans.\nWhen true, media files found in this storage path are automatically added to tracking.\nUseful for discovering media that was added outside the application.\nWhen false, only manually added or list-imported media is tracked.\nHelps build your library from existing collections or shared storage.\nRequires add_found_list to specify which list to add discoveries to.\nDefault: false (manual management only)" toml:"add_found"`
	// AddFoundList is the list name that found entries should be added to
	AddFoundList string `comment:"Name of the list where automatically discovered media should be added.\nSpecifies which list configuration to" displayname:"Discovery Target List" longcomment:"Name of the list where automatically discovered media should be added.\nSpecifies which list configuration to use when add_found is enabled.\nMust reference a list defined in the lists section of this media group.\nThe list determines:\n- Metadata sources and update behavior\n- Search and upgrade preferences\n- Quality requirements for discovered media\nRequired when add_found is true, ignored when false.\nExample: 'discovered-movies' for a list dedicated to found media" toml:"add_found_list"`
	// AddFoundListCfg is a pointer to ListsConfig
	AddFoundListCfg *ListsConfig `toml:"-"`

	// EnableUnpacking enables automatic unpacking of archives before media file processing
	EnableUnpacking bool `comment:"Enable automatic unpacking of archives (RAR, ZIP, 7Z, TAR, etc.) before media file organization.\nWhen true, archives found" displayname:"Enable Archive Unpacking" longcomment:"Enable automatic unpacking of archives (RAR, ZIP, 7Z, TAR, etc.) before media file organization.\nWhen true, archives found in the source path are automatically extracted before scanning for media files.\nSupported formats: RAR, ZIP, 7Z, TAR, TAR.GZ, TAR.BZ2, TAR.XZ, GZIP, BZIP2, XZ\nRequires appropriate unpacking tools to be configured in the general section.\nExtracted files are placed in a temporary directory within the source folder.\nOriginal archives are preserved after extraction.\nDefault: false (no automatic unpacking)" toml:"enable_unpacking"`

	// WriteRenameLog enables writing a rename log file after media file organization
	WriteRenameLog bool `comment:"Write a _rename_log.txt file into the target folder after organization.\nDocuments original and new filenames" displayname:"Write Rename Log" longcomment:"Write a _rename_log.txt file into the target folder after organization.\nDocuments original and new filenames for all moved files including the main media file and additional files (subtitles, NFOs, etc.).\nUseful for tracking what was renamed during organization.\nThe log includes a timestamp, media title, source and target paths, and a list of old to new filename mappings.\nDefault: false (no rename log)" toml:"write_rename_log"`

	// EmbedArt enables downloading and embedding cover art into audio file tags during organization.
	EmbedArt bool `comment:"Download and embed cover art into audio file tags during organization.\nFetches cover art from the database cover URL and embeds it into MP3, FLAC, and OGG files." displayname:"Embed Cover Art" longcomment:"Download and embed cover art into audio file metadata tags during organization.\nFetches cover art from the database cover URL (from Audible/Amazon CDN) and embeds it\ninto the audio files using ID3v2 APIC frames (MP3), FLAC picture blocks, or OGG METADATA_BLOCK_PICTURE.\nThe cover URL must already be stored in the database from the import step.\nDefault: false" toml:"embed_art"`

	// EmbedLyrics enables fetching and embedding plain-text lyrics into audio file tags during organization.
	// Lyrics are fetched from LyricsOVH then LRCLIB (both free, no API key required).
	// They are written to the USLT frame (MP3) or LYRICS Vorbis comment (FLAC/OGG) and are never stored in the database.
	EmbedLyrics bool `comment:"Fetch and embed lyrics into audio file tags during organization.\nTries LyricsOVH then LRCLIB (both free, no API key needed).\nWritten to USLT (MP3) or LYRICS tag (FLAC/OGG). Not stored in the database." displayname:"Embed Lyrics" longcomment:"Fetch plain-text song lyrics from free public APIs and embed them into audio file tags during organization.\nSources tried in order:\n  1. LyricsOVH (api.lyrics.ovh) — no API key required\n  2. LRCLIB (lrclib.net)        — no API key required\nLyrics are written to:\n  MP3:  ID3v2 USLT (Unsynchronised Lyrics) frame\n  FLAC: LYRICS Vorbis comment\n  OGG:  LYRICS Vorbis comment\nLyrics are NOT stored in the local database — they are re-fetched on each re-tag.\nDefault: false" toml:"embed_lyrics"`

	// SkipSeriesTrackMatch disables chapter/track count matching for audiobooks that belong to a series.
	SkipSeriesTrackMatch bool `comment:"Disable chapter count matching for audiobooks that belong to a series.\nUseful for series like Die drei Fragezeichen where downloads have different file counts than the DB." displayname:"Skip Series Track Match" longcomment:"Disable chapter/track count matching for audiobooks that belong to a series.\nAudiobook series episodes (e.g. Die drei Fragezeichen, Benjamin Blümchen) often have\ndifferent file counts depending on the download source vs. the Audible chapter count.\nWhen enabled, if a matched audiobook has series information, the chapter count check\nis skipped and the first matching candidate is used.\nOnly applies to audiobooks, not music.\nDefault: false" toml:"skip_series_track_match"`

	// AllowAlternativeReleases enables searching MusicBrainz for alternative releases
	// when the DB has a title match but with a different track count/edition.
	// If the existing DB release has no files attached, it will be replaced.
	// If it has files, the alternative release is added as a new entry.
	AllowAlternativeReleases bool `comment:"Search MusicBrainz for alternative releases when the DB edition does not match local files.\nIf no files are attached to the existing release, it is replaced; otherwise a new release is added." displayname:"Allow Alternative Releases" longcomment:"Search MusicBrainz for alternative releases when the database has a title match but with a different track count or edition.\nThis is useful for compilation albums like Kuschelrock where different editions (2CD, 3CD, Gold) have different track counts.\nWhen enabled:\n- If the existing DB release has no files attached, it will be replaced with the matching edition\n- If the existing DB release has files, the alternative release is added as a new entry\nRequires add_found_list to specify which list to add new releases to.\nDefault: false" toml:"allow_alternative_releases"`

	// AllowedReleaseTypes filters which MusicBrainz release types are added during artist/series discovery.
	// Values: Album, Single, EP, Compilation, Broadcast, Other.
	// If empty, all release types are allowed.
	AllowedReleaseTypes []string `comment:"Filter which MusicBrainz release types are added during artist and series discovery.\nCommon values: Album, Single, EP, Compilation, Broadcast, Other." displayname:"Allowed Release Types" longcomment:"Filter which MusicBrainz release types are added during artist and series discovery.\nWhen set, only releases matching one of these types will be added to your library.\nIf empty or not set, all release types are allowed (no filtering).\nCommon MusicBrainz primary types:\n- 'Album': Standard studio albums\n- 'Single': Single releases\n- 'EP': Extended play releases\n- 'Compilation': Compilation/best-of albums\n- 'Broadcast': Radio/TV broadcast recordings\n- 'Other': Other release types\nExample: ['Album', 'Compilation'] to only add albums and compilations, skipping singles and EPs." multiline:"true" toml:"allowed_release_types"`

	// MBMediaFormats restricts artist/series discovery to releases where every disc matches one of these formats.
	// Uses MusicBrainz medium format names (e.g., "CD", "Vinyl", "Digital Media").
	// Empty list = accept all formats.
	// Example: ["CD"] accepts 1xCD, 2xCD, 3xCD, etc. but rejects Vinyl or Digital-only releases.
	MBMediaFormats []string `comment:"Restrict discovery to releases where all discs match these formats (e.g. ['CD']).Empty = accept all." displayname:"MB Media Formats" longcomment:"Filter artist and series discovery to releases where every disc matches one of these MusicBrainz medium format names.\nCommon values: 'CD', 'Vinyl', 'Digital Media', 'Cassette', 'Blu-ray'.\nA 2xCD release passes if all 2 discs are CD format.\nEmpty list accepts all formats.\nExample: ['CD'] - only discover CD releases (1xCD, 2xCD, 3xCD, ...)" toml:"mb_media_formats"`

	// AllowAllFormatsWhenStructuring disables the MBMediaFormats filter when structuring
	// (organizing already-downloaded files). Set to true to allow Vinyl, Cassette, or other
	// non-standard formats to be matched and imported during structuring, even when MBMediaFormats
	// restricts discovery to CD / Digital Media only.
	AllowAllFormatsWhenStructuring bool `comment:"Skip the MBMediaFormats filter when structuring already-downloaded files. Allows Vinyl and other formats to be imported during structuring." displayname:"Allow All Formats When Structuring" longcomment:"When true, the MBMediaFormats format filter is bypassed during structuring (organizing already-downloaded files).\nThis allows Vinyl, Cassette, and other non-standard formats to be matched and imported from MusicBrainz when structuring, even if MBMediaFormats restricts discovery to CD / Digital Media only.\nUseful when you have downloaded Vinyl rips that need to be organized." toml:"allow_all_formats_when_structuring"`

	// PerTrackToleranceSeconds is the grace threshold for per-track runtime matching.
	// Tracks within this tolerance receive no length penalty.
	// If PerTrackToleranceSecondsMax is also set, tracks between the two values receive
	// a graduated penalty; tracks beyond Max are hard-rejected.
	// If only PerTrackToleranceSeconds is set (Max == 0), it acts as a hard cutoff.
	// Default 0 uses beets' built-in 10 s grace / 30 s max windows.
	PerTrackToleranceSeconds int `comment:"Runtime diff below this receives no length penalty. Acts as hard cutoff when PerTrackToleranceSecondsMax is 0. Default 0 = beets defaults (10s grace / 30s max)." displayname:"Per-Track Grace (s)" longcomment:"Per-track runtime grace threshold in seconds.\nDiffs at or below this value receive zero track_length penalty.\nWhen PerTrackToleranceSecondsMax > 0: diffs between the two values get a graduated penalty; diffs above Max are hard-rejected.\nWhen PerTrackToleranceSecondsMax == 0: this value is the hard cutoff — any diff above it causes the track pair to be excluded from matching entirely.\nDefault: 0 (use beets built-in 10 s grace / 30 s max)" toml:"per_track_tolerance_seconds"`

	// PerTrackToleranceSecondsMax is the hard cutoff for per-track runtime matching.
	// Track pairs whose diff exceeds this value are excluded from matching entirely.
	// Only effective when PerTrackToleranceSeconds > 0.
	// Default 0 means PerTrackToleranceSeconds is used as both grace and hard limit.
	PerTrackToleranceSecondsMax int `comment:"Hard runtime cutoff per track. Pairs above this are excluded from matching. 0 = use PerTrackToleranceSeconds as hard limit." displayname:"Per-Track Hard Cutoff (s)" longcomment:"Hard per-track runtime cutoff in seconds.\nTrack pairs whose absolute runtime difference exceeds this value are rejected outright and cannot be matched.\nMust be >= PerTrackToleranceSeconds to be meaningful.\nOnly used when PerTrackToleranceSeconds > 0.\nDefault: 0 (PerTrackToleranceSeconds acts as both grace and hard limit)" toml:"per_track_tolerance_seconds_max"`

	// MaxTotalDifferenceSeconds is the maximum total runtime difference allowed in seconds.
	// Default 0 uses PerTrackToleranceSeconds * track count.
	MaxTotalDifferenceSeconds int `comment:"Maximum total runtime difference allowed in seconds during import matching.\nDefault 0 uses per-track tolerance multiplied by track count." displayname:"Max Total Difference (s)" longcomment:"Maximum total runtime difference allowed in seconds during import matching.\nWhen set, this overrides the calculated total tolerance (per_track * track_count).\nUseful for setting an absolute cap on total runtime mismatch.\nDefault: 0 (use per_track_tolerance_seconds * track_count)" toml:"max_total_difference_seconds"`

	// AllowMissingTracks skips the strict track count check during runtime verification.
	// When true, a local folder with fewer tracks than expected is still accepted.
	AllowMissingTracks bool `comment:"Allow local track count to be fewer than expected from the database.\nDefault false requires exact track count match." displayname:"Allow Missing Tracks" longcomment:"When enabled, the track count check in runtime verification is skipped.\nUseful for partial album imports or when downloads are missing some tracks.\nDefault: false (exact track count required)" toml:"allow_missing_tracks"`

	// ExceedToleranceIfTotalMatch controls the runtime fallback behavior in track sorting.
	// When true, if all per-track runtime strategies fail but the total runtime is within tolerance,
	// the album is accepted using standard sort order.
	// When false (default), standard sort is always used as the final fallback regardless of total runtime.
	ExceedToleranceIfTotalMatch bool `comment:"When per-track runtime matching fails, only accept via standard sort if total runtime is within tolerance.\nDefault false always falls back to standard sort." displayname:"Exceed Tolerance If Total Matches" longcomment:"When enabled, if per-track runtime matching strategies all fail, the standard sort fallback is only accepted if the total runtime difference is within the configured tolerance.\nWhen disabled (default), standard sort is always used as the final fallback.\nDefault: false" toml:"exceed_tolerance_if_total_match"`

	// --- Distance matching overrides (0 = use built-in default) ---

	// StrongRecThresh overrides the strong recommendation threshold (default 0.03).
	// Album candidates with distance <= this are accepted unconditionally.
	// StrongRecThresh float64 `toml:"strong_rec_thresh" displayname:"Strong Rec Threshold" comment:"Album distance threshold for a strong recommendation (default 0.03). Candidates at or below this are accepted unconditionally. 0 = use default." longcomment:"Overrides the built-in beets strong_rec_thresh=0.03.\nAlbum candidates whose weighted distance score is at or below this value are treated as a strong match.\nLower values require a closer match before automatic acceptance.\n0 = use default (0.03)."`

	// MediumRecThresh overrides the medium recommendation threshold / reject cutoff (default 0.30).
	// Album candidates with distance > this are rejected outright.
	// MediumRecThresh float64 `toml:"medium_rec_thresh" displayname:"Medium Rec Threshold" comment:"Album distance reject cutoff (default 0.30). Candidates above this are rejected. 0 = use default." longcomment:"Overrides the built-in beets medium_rec_thresh=0.30.\nAlbum candidates whose weighted distance score exceeds this value are rejected and not passed to track matching.\nRaise to be more permissive, lower to be stricter.\n0 = use default (0.30)."`

	// Track distance weight overrides (0 = use built-in default from beetsWeights / audiobookTrackWeights).
	TrackTitleWeight  float64 `comment:"Weight for track title string distance (default: 3.0 music / 1.0 audiobook). 0 = use default." displayname:"Track Title Weight"  toml:"track_title_weight"`
	TrackIndexWeight  float64 `comment:"Weight for track number equality (default: 1.0 music / 3.0 audiobook). 0 = use default."       displayname:"Track Index Weight"  toml:"track_index_weight"`
	TrackLengthWeight float64 `comment:"Weight for track runtime distance (default: 2.0 music / 3.0 audiobook). 0 = use default."      displayname:"Track Length Weight" toml:"track_length_weight"`
	TrackArtistWeight float64 `comment:"Weight for track artist distance, VA albums only (default: 2.0). 0 = use default."             displayname:"Track Artist Weight" toml:"track_artist_weight"`
	TrackIdWeight     float64 `comment:"Weight for MusicBrainz recording ID match (default: 5.0). 0 = use default."                    displayname:"Track ID Weight"     toml:"track_id_weight"`

	// DiscoverSeriesAlbums enables automatic discovery of other albums in the same release
	// group (series) when a Various Artists album is successfully imported via addFound.
	// When false (default), only artist discovery runs; series discovery is skipped.
	DiscoverSeriesAlbums bool `comment:"Discover and add other albums in the same MusicBrainz release group when a Various Artists album is imported.\nDefault: false" displayname:"Discover Series Albums" longcomment:"When enabled and an addFound import succeeds for a Various Artists album, the application queries MusicBrainz for other releases in the same release group and adds them to the library.\nWhen disabled (default), only per-artist discovery runs; release-group series discovery is skipped.\nDefault: false" toml:"discover_series_albums"`
}

// MediaDataImportConfig defines the configuration for importing media data.
type MediaDataImportConfig struct {
	// TemplatePath is the template to use for the path
	TemplatePath string `comment:"Name of the path configuration template to use for importing existing media files.\nReferences a path" displayname:"Import Path Template" longcomment:"Name of the path configuration template to use for importing existing media files.\nReferences a path configuration defined in the paths section.\nThe import path template controls:\n- Source directories to scan for existing media files\n- File types and extensions to import\n- Size limits and quality filters for import\n- Whether to move, copy, or hardlink imported files\n- File organization and renaming during import\nMust exactly match the 'name' field of a paths configuration.\nTypically uses different settings than regular download paths.\nExample: 'import-movies' for a path optimized for importing existing collections" toml:"template_path"`
	// CfgPath is the PathsConfig reference
	CfgPath *PathsConfig `toml:"-"`

	// EnableUnpacking enables automatic unpacking of archives before media file processing
	EnableUnpacking bool `comment:"Enable automatic unpacking of archives (RAR, ZIP, 7Z, TAR, etc.) before media file organization.\nWhen true, archives found" displayname:"Enable Archive Unpacking" longcomment:"Enable automatic unpacking of archives (RAR, ZIP, 7Z, TAR, etc.) before media file organization.\nWhen true, archives found in the source path are automatically extracted before scanning for media files.\nSupported formats: RAR, ZIP, 7Z, TAR, TAR.GZ, TAR.BZ2, TAR.XZ, GZIP, BZIP2, XZ\nRequires appropriate unpacking tools to be configured in the general section.\nExtracted files are placed in a temporary directory within the source folder.\nOriginal archives are preserved after extraction.\nDefault: false (no automatic unpacking)" toml:"enable_unpacking"`

	// AddFound indicates if entries not in the database should be added when found during import.
	// When true and the media is not in the DB (no_match), it will be imported automatically.
	// Requires add_found_list to be set.
	AddFound bool `comment:"Automatically add discovered media to your wanted list when not found in the database during import.\nRequires add_found_list to be set." displayname:"Auto Add Discovered Media" longcomment:"Automatically add discovered media to your wanted list when the import scan cannot find it in the database.\nWhen true, unrecognized media is imported and added to the specified list.\nRequires add_found_list to reference a valid list configuration.\nDefault: false" toml:"add_found"`

	// AddFoundList is the list name that unrecognized entries should be added to.
	AddFoundList string `comment:"Name of the list where automatically discovered media should be added during import.\nRequired when add_found is true." displayname:"Discovery Target List" longcomment:"Name of the list where automatically discovered unrecognized media should be added.\nSpecifies which list configuration to use when add_found is enabled for the import path.\nMust reference a list defined in the lists section of this media group.\nRequired when add_found is true, ignored when false." toml:"add_found_list"`

	// AllowAlternativeReleases enables searching MusicBrainz for alternative releases
	// when the DB has a title match but with a different track count/edition.
	AllowAlternativeReleases bool `comment:"Search MusicBrainz for alternative releases when the DB edition does not match local files." displayname:"Allow Alternative Releases" longcomment:"Search MusicBrainz for alternative releases when the database has a title match but with a different track count or edition.\nIf the existing DB release has no files, it is replaced; otherwise a new release is added.\nDefault: false" toml:"allow_alternative_releases"`

	// MoveUnprocessed is the path to move unprocessed/unmatched folders to.
	// When set, folders that cannot be organized are moved to <path>/<reason>/<original_folder>/
	MoveUnprocessed string `comment:"Path to move unprocessed folders to when they cannot be matched or organized.\nFolders are placed in reason subfolders (no_match, wrong_runtime, etc.).\nLeave empty to keep folders in place." displayname:"Move Unprocessed Path" longcomment:"Path to move unprocessed folders to when they cannot be matched or organized.\nWhen set, folders that fail processing are moved to:\n<path>/<reason>/<original_folder_name>/\nReason subfolders include: no_match, small_file, no_list, unwanted_title,\nwrong_runtime, wrong_language, organize_failed, naming_failed.\nThis helps separate unprocessable content from pending downloads.\nLeave empty to keep unprocessed folders in place (default behavior)." toml:"move_unprocessed"`

	// PerTrackToleranceSeconds is the grace threshold for per-track runtime matching.
	// Tracks within this tolerance receive no length penalty.
	// If PerTrackToleranceSecondsMax is also set, tracks between the two values receive
	// a graduated penalty; tracks beyond Max are hard-rejected.
	// If only PerTrackToleranceSeconds is set (Max == 0), it acts as a hard cutoff.
	// Default 0 uses beets' built-in 10 s grace / 30 s max windows.
	PerTrackToleranceSeconds int `comment:"Runtime diff below this receives no length penalty. Acts as hard cutoff when PerTrackToleranceSecondsMax is 0. Default 0 = beets defaults (10s grace / 30s max)." displayname:"Per-Track Grace (s)" longcomment:"Per-track runtime grace threshold in seconds.\nDiffs at or below this value receive zero track_length penalty.\nWhen PerTrackToleranceSecondsMax > 0: diffs between the two values get a graduated penalty; diffs above Max are hard-rejected.\nWhen PerTrackToleranceSecondsMax == 0: this value is the hard cutoff — any diff above it causes the track pair to be excluded from matching entirely.\nDefault: 0 (use beets built-in 10 s grace / 30 s max)" toml:"per_track_tolerance_seconds"`

	// PerTrackToleranceSecondsMax is the hard cutoff for per-track runtime matching.
	// Track pairs whose diff exceeds this value are excluded from matching entirely.
	// Only effective when PerTrackToleranceSeconds > 0.
	// Default 0 means PerTrackToleranceSeconds is used as both grace and hard limit.
	PerTrackToleranceSecondsMax int `comment:"Hard runtime cutoff per track. Pairs above this are excluded from matching. 0 = use PerTrackToleranceSeconds as hard limit." displayname:"Per-Track Hard Cutoff (s)" longcomment:"Hard per-track runtime cutoff in seconds.\nTrack pairs whose absolute runtime difference exceeds this value are rejected outright and cannot be matched.\nMust be >= PerTrackToleranceSeconds to be meaningful.\nOnly used when PerTrackToleranceSeconds > 0.\nDefault: 0 (PerTrackToleranceSeconds acts as both grace and hard limit)" toml:"per_track_tolerance_seconds_max"`

	// MaxTotalDifferenceSeconds is the maximum total runtime difference allowed in seconds.
	// Default 0 uses PerTrackToleranceSeconds * track count.
	MaxTotalDifferenceSeconds int `comment:"Maximum total runtime difference allowed in seconds during import matching.\nDefault 0 uses per-track tolerance multiplied by track count." displayname:"Max Total Difference (s)" longcomment:"Maximum total runtime difference allowed in seconds during import matching.\nWhen set, this overrides the calculated total tolerance (per_track * track_count).\nUseful for setting an absolute cap on total runtime mismatch.\nDefault: 0 (use per_track_tolerance_seconds * track_count)" toml:"max_total_difference_seconds"`

	// AllowMissingTracks skips the strict track count check during runtime verification.
	// When true, a local folder with fewer tracks than expected is still accepted.
	AllowMissingTracks bool `comment:"Allow local track count to be fewer than expected from the database.\nDefault false requires exact track count match." displayname:"Allow Missing Tracks" longcomment:"When enabled, the track count check in runtime verification is skipped.\nUseful for partial album imports or when downloads are missing some tracks.\nDefault: false (exact track count required)" toml:"allow_missing_tracks"`

	// AllowAllFormatsWhenStructuring disables the MBMediaFormats filter when structuring
	// (organizing already-downloaded files). Set to true to allow Vinyl, Cassette, or other
	// non-standard formats to be matched and imported during structuring, even when MBMediaFormats
	// restricts discovery to CD / Digital Media only.
	AllowAllFormatsWhenStructuring bool `comment:"Skip the MBMediaFormats filter when structuring already-downloaded files. Allows Vinyl and other formats to be imported during structuring." displayname:"Allow All Formats When Structuring" longcomment:"When true, the MBMediaFormats format filter is bypassed during structuring (organizing already-downloaded files).\nThis allows Vinyl, Cassette, and other non-standard formats to be matched and imported from MusicBrainz when structuring, even if MBMediaFormats restricts discovery to CD / Digital Media only.\nUseful when you have downloaded Vinyl rips that need to be organized." toml:"allow_all_formats_when_structuring"`

	// ExceedToleranceIfTotalMatch controls the runtime fallback behavior in track sorting.
	// When true, if all per-track runtime strategies fail but the total runtime is within tolerance,
	// the album is accepted using standard sort order.
	// When false (default), standard sort is always used as the final fallback regardless of total runtime.
	ExceedToleranceIfTotalMatch bool `comment:"When per-track runtime matching fails, only accept via standard sort if total runtime is within tolerance.\nDefault false always falls back to standard sort." displayname:"Exceed Tolerance If Total Matches" longcomment:"When enabled, if per-track runtime matching strategies all fail, the standard sort fallback is only accepted if the total runtime difference is within the configured tolerance.\nWhen disabled (default), standard sort is always used as the final fallback.\nDefault: false" toml:"exceed_tolerance_if_total_match"`

	// --- Distance matching overrides (0 = use built-in default) ---

	// StrongRecThresh overrides the strong recommendation threshold (default 0.03).
	// Album candidates with distance <= this are accepted unconditionally.
	StrongRecThresh float64 `comment:"Album distance threshold for a strong recommendation (default 0.03). Candidates at or below this are accepted unconditionally. 0 = use default." displayname:"Strong Rec Threshold" longcomment:"Overrides the built-in beets strong_rec_thresh=0.03.\nAlbum candidates whose weighted distance score is at or below this value are treated as a strong match.\nLower values require a closer match before automatic acceptance.\n0 = use default (0.03)." toml:"strong_rec_thresh"`

	// MediumRecThresh overrides the medium recommendation threshold / reject cutoff (default 0.30).
	// Album candidates with distance > this are rejected outright.
	MediumRecThresh float64 `comment:"Album distance reject cutoff (default 0.30). Candidates above this are rejected. 0 = use default." displayname:"Medium Rec Threshold" longcomment:"Overrides the built-in beets medium_rec_thresh=0.30.\nAlbum candidates whose weighted distance score exceeds this value are rejected and not passed to track matching.\nRaise to be more permissive, lower to be stricter.\n0 = use default (0.30)." toml:"medium_rec_thresh"`

	// Track distance weight overrides (0 = use built-in default from beetsWeights / audiobookTrackWeights).
	TrackTitleWeight  float64 `comment:"Weight for track title string distance (default: 3.0 music / 1.0 audiobook). 0 = use default." displayname:"Track Title Weight"  toml:"track_title_weight"`
	TrackIndexWeight  float64 `comment:"Weight for track number equality (default: 1.0 music / 3.0 audiobook). 0 = use default."       displayname:"Track Index Weight"  toml:"track_index_weight"`
	TrackLengthWeight float64 `comment:"Weight for track runtime distance (default: 2.0 music / 3.0 audiobook). 0 = use default."      displayname:"Track Length Weight" toml:"track_length_weight"`
	TrackArtistWeight float64 `comment:"Weight for track artist distance, VA albums only (default: 2.0). 0 = use default."             displayname:"Track Artist Weight" toml:"track_artist_weight"`
	TrackIdWeight     float64 `comment:"Weight for MusicBrainz recording ID match (default: 5.0). 0 = use default."                    displayname:"Track ID Weight"     toml:"track_id_weight"`

	// EmbedArt enables downloading and embedding cover art into audio file tags during organization.
	EmbedArt bool `comment:"Download and embed cover art into audio file tags during organization.\nFetches cover art from the database cover URL and embeds it into MP3, FLAC, and OGG files." displayname:"Embed Cover Art" longcomment:"Download and embed cover art into audio file metadata tags during organization.\nFetches cover art from the database cover URL (from Audible/Amazon CDN) and embeds it\ninto the audio files using ID3v2 APIC frames (MP3), FLAC picture blocks, or OGG METADATA_BLOCK_PICTURE.\nThe cover URL must already be stored in the database from the import step.\nDefault: false" toml:"embed_art"`

	// AllowedReleaseTypes filters which MusicBrainz release types are added during artist/series discovery.
	// Values: Album, Single, EP, Compilation, Broadcast, Other.
	// If empty, all release types are allowed.
	AllowedReleaseTypes []string `comment:"Filter which MusicBrainz release types are added during artist and series discovery.\nCommon values: Album, Single, EP, Compilation, Broadcast, Other." displayname:"Allowed Release Types" longcomment:"Filter which MusicBrainz release types are added during artist and series discovery.\nWhen set, only releases matching one of these types will be added to your library.\nIf empty or not set, all release types are allowed (no filtering).\nCommon MusicBrainz primary types:\n- 'Album': Standard studio albums\n- 'Single': Single releases\n- 'EP': Extended play releases\n- 'Compilation': Compilation/best-of albums\n- 'Broadcast': Radio/TV broadcast recordings\n- 'Other': Other release types\nExample: ['Album', 'Compilation'] to only add albums and compilations, skipping singles and EPs." multiline:"true" toml:"allowed_release_types"`

	// MBMediaFormats restricts artist/series discovery to releases where every disc matches one of these formats.
	// Uses MusicBrainz medium format names (e.g., "CD", "Vinyl", "Digital Media").
	// Empty list = accept all formats.
	// Example: ["CD"] accepts 1xCD, 2xCD, 3xCD, etc. but rejects Vinyl or Digital-only releases.
	MBMediaFormats []string `comment:"Restrict discovery to releases where all discs match these formats (e.g. ['CD']).Empty = accept all." displayname:"MB Media Formats" longcomment:"Filter artist and series discovery to releases where every disc matches one of these MusicBrainz medium format names.\nCommon values: 'CD', 'Vinyl', 'Digital Media', 'Cassette', 'Blu-ray'.\nA 2xCD release passes if all 2 discs are CD format.\nEmpty list accepts all formats.\nExample: ['CD'] - only discover CD releases (1xCD, 2xCD, 3xCD, ...)" toml:"mb_media_formats"`

	// DiscoverSeriesAlbums enables automatic discovery of other albums in the same release
	// group (series) when a Various Artists album is successfully imported via addFound.
	// When false (default), only artist discovery runs; series discovery is skipped.
	DiscoverSeriesAlbums bool `comment:"Discover and add other albums in the same MusicBrainz release group when a Various Artists album is imported.\nDefault: false" displayname:"Discover Series Albums" longcomment:"When enabled and an addFound import succeeds for a Various Artists album, the application queries MusicBrainz for other releases in the same release group and adds them to the library.\nWhen disabled (default), only per-artist discovery runs; release-group series discovery is skipped.\nDefault: false" toml:"discover_series_albums"`
}

// MediaListsConfig defines a media list configuration.
type MediaListsConfig struct {
	// Name is the name of the list - use this name in ignore or replace lists
	Name string `comment:"Unique identifier name for this media list configuration within the media group.\nThis name is used" displayname:"List Configuration Name" longcomment:"Unique identifier name for this media list configuration within the media group.\nThis name is used to reference this list in other configurations and operations.\nMust be unique within this media group but can be reused across different media groups.\nUsed when referencing this list in:\n- ignore_template_lists and replace_template_lists of other lists\n- add_found_list references in data configurations\n- Log messages and web interface displays\nChoose descriptive names that indicate the list's purpose or source.\nExample: 'imdb-watchlist', 'trakt-collection', 'manual-movies'" toml:"name"`
	// TemplateList is the template to use for the list
	TemplateList string `comment:"Name of the list configuration template to use for external list integration.\nReferences a list configuration" displayname:"External List Template" longcomment:"Name of the list configuration template to use for external list integration.\nReferences a list configuration defined in the lists section.\nThe list template controls:\n- External list source (IMDB lists, Trakt lists, RSS feeds, etc.)\n- Update frequency and synchronization behavior\n- Filtering and processing rules for list entries\n- Authentication credentials for accessing external lists\nMust exactly match the 'name' field of a lists configuration.\nLeave empty if this is a manual list without external synchronization.\nExample: 'imdb-top250' for an IMDB top movies list template" toml:"template_list"`
	// CfgList is the pointer to the ListsConfig
	CfgList *ListsConfig `toml:"-"`
	// TemplateQuality is the template to use for the quality
	TemplateQuality string `comment:"Name of the quality profile template to use for media from this list.\nReferences a quality" displayname:"Quality Profile Override" longcomment:"Name of the quality profile template to use for media from this list.\nReferences a quality configuration defined in the quality section.\nOverrides the media group's default quality settings for items from this specific list.\nUseful when different lists require different quality standards:\n- High-quality lists might use 4K/BluRay profiles\n- Bulk import lists might use standard HD profiles\nMust exactly match the 'name' field of a quality configuration.\nLeave empty to use the media group's default quality template.\nExample: 'uhd-quality' for a list requiring 4K content" toml:"template_quality"`
	// CfgQuality is the pointer to the QualityConfig
	CfgQuality *QualityConfig `toml:"-"`
	// TemplateScheduler is the template to use for the scheduler - overrides default of media
	TemplateScheduler string `comment:"Name of the scheduler template to use for automated tasks for this list.\nReferences a scheduler" displayname:"Scheduler Template Override" longcomment:"Name of the scheduler template to use for automated tasks for this list.\nReferences a scheduler configuration defined in the scheduler section.\nOverrides the media group's default scheduler settings for items from this specific list.\nUseful when different lists need different automation behavior:\n- Priority lists might check more frequently\n- Archive lists might check less often\n- New release lists might need immediate processing\nMust exactly match the 'name' field of a scheduler configuration.\nLeave empty to use the media group's default scheduler template.\nExample: 'priority-schedule' for high-priority list items" toml:"template_scheduler"`
	// CfgScheduler is the pointer to the SchedulerConfig
	CfgScheduler *SchedulerConfig `toml:"-"`
	// IgnoreMapLists are the lists to check for ignoring entries
	IgnoreMapLists []string `comment:"List of other list names whose entries should be ignored/skipped.\nWhen processing this list, any entries" displayname:"Lists To Ignore" longcomment:"List of other list names whose entries should be ignored/skipped.\nWhen processing this list, any entries that exist in the specified ignore lists will be skipped.\nUseful for filtering out unwanted content or avoiding duplicates:\n- Skip entries that are in a 'blocked-movies' list\n- Ignore items already in a 'completed-series' list\n- Filter out content from a 'low-priority' list\nReferences other MediaListsConfig names within the same media group.\nProcessed before replace_template_lists during list processing.\nExample: ['completed-movies', 'blocked-content'] to skip those entries" multiline:"true" toml:"ignore_template_lists"`
	// IgnoreMapListsQu is the quality string
	IgnoreMapListsQu string `toml:"-"`
	// IgnoreMapListsLen is the length of IgnoreMapLists
	IgnoreMapListsLen int `toml:"-"`
	// ReplaceMapLists are the lists to check for replacing entries
	ReplaceMapLists []string `comment:"List of other list names whose entries should override/replace entries in this list.\nWhen processing, entries" displayname:"Lists To Override" longcomment:"List of other list names whose entries should override/replace entries in this list.\nWhen processing, entries from replace lists take precedence over this list's entries.\nUseful for priority management and quality upgrades:\n- Let 'high-priority' list override 'standard' list settings\n- Allow 'manual-additions' to override automated list imports\n- Use 'quality-upgrades' list to override original quality requirements\nReferences other MediaListsConfig names within the same media group.\nProcessed after ignore_template_lists during list processing.\nExample: ['manual-overrides', 'priority-queue'] to prioritize those entries" multiline:"true" toml:"replace_template_lists"`
	// ReplaceMapListsLen is the length of ReplaceMapLists
	ReplaceMapListsLen int `toml:"-"`
	// Enabled indicates if this configuration is active
	Enabled bool `comment:"Enable or disable this list configuration.\nWhen true, this list is actively processed and its entries" displayname:"Enable List Processing" longcomment:"Enable or disable this list configuration.\nWhen true, this list is actively processed and its entries are managed.\nWhen false, this list is ignored during all operations:\n- No synchronization with external sources\n- No processing of list entries\n- No searches or downloads triggered by this list\nUseful for temporarily disabling lists without deleting the configuration.\nAlso useful during testing or when lists are under maintenance.\nDefault: false (list disabled)" toml:"enabled"`
	// Addfound indicates if entries not already watched should be added when found
	Addfound bool `comment:"Automatically add discovered media to this list when found during file scans.\nWhen true, media files" displayname:"Auto Add Found Media" longcomment:"Automatically add discovered media to this list when found during file scans.\nWhen true, media files found during library scans are automatically added to this list.\nUseful for building lists from existing media collections:\n- Scan existing movie folders to populate a 'discovered-movies' list\n- Find TV series already on disk and add to 'existing-shows' list\n- Import media from shared storage or external drives\nWhen false, only manually added or externally synchronized entries are in the list.\nWorks in conjunction with the media group's data configuration settings.\nDefault: false (manual/external list management only)" toml:"add_found"`
}

// MediaNotificationConfig defines the configuration for notifications about media events.
type MediaNotificationConfig struct {
	// MapNotification is the template to use for the notification
	MapNotification string `comment:"Name of the notification configuration template to use for this media event.\nReferences a notification configuration" displayname:"Notification Template Reference" longcomment:"Name of the notification configuration template to use for this media event.\nReferences a notification configuration defined in the notification section.\nThe notification template controls:\n- Delivery method (Pushover, email, webhook, CSV file)\n- Authentication credentials and connection settings\n- Message formatting and delivery options\n- Rate limiting and retry behavior\nMust exactly match the 'name' field of a notification configuration.\nDifferent events can use different notification templates for varied delivery.\nExample: 'pushover-downloads' for mobile notifications" toml:"template_notification"`
	// CfgNotification is the NotificationConfig reference
	CfgNotification *NotificationConfig `toml:"-"`
	// Event is the type of event this is for - use added_download, added_data or upgraded_data
	Event string `comment:"Type of media event that triggers this notification.\nSpecifies when this notification configuration should be used.\nSupported" displayname:"Notification Event Type" longcomment:"Type of media event that triggers this notification.\nSpecifies when this notification configuration should be used.\nSupported event types:\n- 'added_download': When media is successfully downloaded and added to library\n- 'added_data': When media files are manually added or imported without replacing existing files\n- 'upgraded_data': When media files are added that replace/upgrade existing files\nEach event type can have different notification settings and messages.\nExample: 'added_download' for successful download notifications, 'upgraded_data' for quality upgrades" toml:"event"`
	// Title is the title of your message (for pushover)
	Title string `comment:"Notification title/subject line for the message.\nUsed as the title for Pushover notifications, email subject lines," displayname:"Notification Title Template" longcomment:"Notification title/subject line for the message.\nUsed as the title for Pushover notifications, email subject lines, etc.\nSupports template variables that are replaced with actual media information.\nKeep concise as some notification services limit title length.\nExample: 'New Movie added in {{.Configuration}}'" toml:"title"`
	// Message is the message body - look at https://github.com/Kellerman81/go_media_downloader/wiki/Groups for format info
	Message string `comment:"Main notification message body content.\nSupports template variables that are replaced with actual media information.\nCan include" displayname:"Notification Message Template" longcomment:"Main notification message body content.\nSupports template variables that are replaced with actual media information.\nCan include multiple lines and detailed information.\nExample: '{{.Title}} - moved from {{.SourcePath}} to {{.Targetpath}}{{if .Replaced }} Replaced: {{ range .Replaced }}{{.}},{{end}}{{end}}'\nExample: '{{.Time}};{{.Title}};{{.Season}};{{.Episode}};{{.Tvdb}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}'\nExample: '{{.Time}};{{.Title}};{{.Year}};{{.Imdb}};{{.SourcePath}};{{.Targetpath}};{{ range .Replaced }}{{.}},{{end}}'\nSee wiki for complete variable list: https://github.com/Kellerman81/go_media_downloader/wiki/Groups" toml:"message"`
	// ReplacedPrefix is text to write in front of the old path if media was replaced
	ReplacedPrefix string `comment:"Text prefix added to notifications when media files are replaced/upgraded.\nWhen existing media is replaced with" displayname:"File Replacement Prefix" longcomment:"Text prefix added to notifications when media files are replaced/upgraded.\nWhen existing media is replaced with better quality, this text appears before the old file path.\nHelps distinguish upgrade notifications from new download notifications.\nUseful for indicating what action was taken with the previous file.\nCommon prefixes:\n- 'Replaced: ' to indicate file replacement\n- 'Upgraded from: ' to show what was upgraded\n- 'Previous: ' to reference the old version\nExample: 'Upgraded from: ' results in 'Upgraded from: /path/to/old/file.mkv'" toml:"replaced_prefix"`
}

// DownloaderConfig is a struct that defines the configuration for a downloader client.
type DownloaderConfig struct {
	// Name is the name of the downloader template
	Name string `comment:"Unique name for this downloader configuration.\nUsed to identify this downloader in quality profiles and logs.\nChoose" displayname:"Downloader Configuration Name" longcomment:"Unique name for this downloader configuration.\nUsed to identify this downloader in quality profiles and logs.\nChoose a descriptive name that identifies the client and purpose.\nExample: 'sabnzbd-main' or 'qbittorrent-movies'" toml:"name"`
	// DlType is the type of downloader, e.g. drone, nzbget, etc.
	DlType string `comment:"Type of download client software.\nSupported options:\n- 'sabnzbd': SABnzbd Usenet client\n- 'nzbget': NZBGet Usenet client\n- 'qbittorrent':" displayname:"Download Client Type" longcomment:"Type of download client software.\nSupported options:\n- 'sabnzbd': SABnzbd Usenet client\n- 'nzbget': NZBGet Usenet client\n- 'qbittorrent': qBittorrent torrent client\n- 'transmission': Transmission torrent client\n- 'rtorrent': rTorrent/ruTorrent client\n- 'deluge': Deluge torrent client\n- 'drone': Drone (Download to filesystem)\nExample: 'sabnzbd' or 'qbittorrent'" toml:"type"`
	// Hostname is the hostname to use if needed
	Hostname string `comment:"IP address or hostname of the download client.\nCan be a local IP (192.168.1.100), hostname (nas.local)," displayname:"Client Hostname Address" longcomment:"IP address or hostname of the download client.\nCan be a local IP (192.168.1.100), hostname (nas.local), or FQDN.\nUse 'localhost' or '127.0.0.1' for local installations.\nDo not include protocol (http://) or port number here.\nExample: '192.168.1.100' or 'localhost'" toml:"hostname"`
	// Port is the port to use if needed
	Port int `comment:"TCP port number where the download client is listening.\nCommon default ports:\n- SABnzbd: 8080\n- NZBGet: 6789\n-" displayname:"Client Port Number" longcomment:"TCP port number where the download client is listening.\nCommon default ports:\n- SABnzbd: 8080\n- NZBGet: 6789\n- qBittorrent: 8080\n- Transmission: 9091\n- Deluge: 8112\nCheck your client's settings for the correct port.\nExample: 8080 or 6789" toml:"port"`
	// Username is the username to use if needed
	Username string `comment:"Username for authentication with the download client.\nRequired if your download client has authentication enabled.\nLeave empty" displayname:"Authentication Username" longcomment:"Username for authentication with the download client.\nRequired if your download client has authentication enabled.\nLeave empty if the client doesn't require authentication.\nSome clients allow guest access or have auth disabled.\nExample: 'admin' or 'myuser'" toml:"username"`
	// Password is the password to use if needed
	Password string `comment:"Password for authentication with the download client.\nRequired if your download client has authentication enabled.\nLeave empty" displayname:"Authentication Password" longcomment:"Password for authentication with the download client.\nRequired if your download client has authentication enabled.\nLeave empty if the client doesn't require authentication.\nFor API key-based clients, this may be the API key instead.\nExample: 'mypassword' or 'api-key-here'" toml:"password"`
	// AddPaused specifies whether to add entries in paused state
	AddPaused bool `comment:"Add downloads in paused state instead of starting immediately.\nWhen true, downloads are queued but not" displayname:"Add Downloads Paused" longcomment:"Add downloads in paused state instead of starting immediately.\nWhen true, downloads are queued but not started automatically.\nUseful for manual review before starting downloads.\nWhen false, downloads start immediately after being added.\nDefault: false (start immediately)" toml:"add_paused"`
	// DelugeDlTo is the Deluge target for downloads
	DelugeDlTo string `comment:"Initial download directory for Deluge client (Deluge-specific setting).\nThis is where files are downloaded before processing.\nMust" displayname:"Deluge Download Directory" longcomment:"Initial download directory for Deluge client (Deluge-specific setting).\nThis is where files are downloaded before processing.\nMust be a path accessible to the Deluge daemon.\nOnly used when type is 'deluge'.\nExample: '/downloads/incomplete'" toml:"deluge_dl_to"`
	// DelugeMoveAfter specifies if downloads should be moved after completion in Deluge
	DelugeMoveAfter bool `comment:"Enable automatic file moving after download completion in Deluge.\nWhen true, completed downloads are moved to" displayname:"Deluge Auto Move Files" longcomment:"Enable automatic file moving after download completion in Deluge.\nWhen true, completed downloads are moved to the path specified in deluge_move_to.\nWhen false, files remain in the download directory.\nOnly used when type is 'deluge'.\nDefault: false" toml:"deluge_move_after"`
	// DelugeMoveTo is the Deluge target for downloads after completion
	DelugeMoveTo string `comment:"Destination directory for completed downloads in Deluge.\nUsed when deluge_move_after is enabled.\nFiles are moved here after" displayname:"Deluge Completion Directory" longcomment:"Destination directory for completed downloads in Deluge.\nUsed when deluge_move_after is enabled.\nFiles are moved here after successful download completion.\nMust be a path accessible to the Deluge daemon.\nOnly used when type is 'deluge' and deluge_move_after is true.\nExample: '/downloads/complete'" toml:"deluge_move_to"`
	// Priority is the priority to set if needed
	Priority int `comment:"Default priority level for downloads added to this client.\nHigher numbers typically mean higher priority (client-dependent).\nCommon" displayname:"Default Download Priority" longcomment:"Default priority level for downloads added to this client.\nHigher numbers typically mean higher priority (client-dependent).\nCommon values: -2 (very low), -1 (low), 0 (normal), 1 (high), 2 (very high)\nCheck your download client's documentation for valid ranges.\nExample: 0 for normal priority" toml:"priority"`
	// Enabled specifies if this template is active
	Enabled bool `comment:"Enable or disable this downloader configuration.\nWhen true, this downloader can be used by quality profiles.\nWhen" displayname:"Enable Downloader Configuration" longcomment:"Enable or disable this downloader configuration.\nWhen true, this downloader can be used by quality profiles.\nWhen false, this downloader is ignored and won't receive downloads.\nUseful for temporarily disabling a downloader without deleting the config.\nDefault: true" toml:"enabled"`
}

// ListsConfig defines the configuration for lists.
type ListsConfig struct {
	// Name is the name of the template
	Name string `comment:"Unique name for this list configuration.\nUsed to identify this list in logs and management interfaces.\nChoose" displayname:"List Configuration Name" longcomment:"Unique name for this list configuration.\nUsed to identify this list in logs and management interfaces.\nChoose a descriptive name that indicates the list source and purpose.\nExample: 'imdb-top250', 'trakt-watchlist', 'popular-movies'" toml:"name"`
	// ListType is the type of the list
	ListType string `comment:"Type of list source to import from.\nAvailable options:\n- 'imdbcsv': IMDB CSV export file\n- 'imdbfile': IMDB" displayname:"List Source Type" longcomment:"Type of list source to import from.\nAvailable options:\n- 'imdbcsv': IMDB CSV export file\n- 'imdbfile': IMDB watchlist file\n- 'seriesconfig': Local TOML series configuration\n- 'moviescraper': Movie scraper (HTML/XPath or CSRF API)\n- 'traktpublicmovielist': Public Trakt movie list\n- 'traktpublicshowlist': Public Trakt TV show list\n- 'traktmoviepopular': Trakt popular movies\n- 'traktmovieanticipated': Trakt anticipated movies\n- 'traktmovietrending': Trakt trending movies\n- 'traktseriepopular': Trakt popular TV series\n- 'traktserieanticipated': Trakt anticipated TV series\n- 'traktserietrending': Trakt trending TV series\n- 'tmdbmoviepopular': TMDB popular movies\n- 'tmdbmovietrending': TMDB trending movies\n- 'tmdbmovieupcoming': TMDB upcoming movies\n- 'tmdbseriepopular': TMDB popular TV series\n- 'tmdbserietrending': TMDB trending TV series\n- 'newznabrss': Newznab RSS feed\n- 'plexwatchlist': Plex user watchlist\n- 'jellyfinwatchlist': Jellyfin user watchlist\nExample: 'imdbcsv' or 'traktmoviepopular' or 'tmdbmoviepopular' or 'moviescraper'" toml:"type"`
	// URL is the url of the list
	URL string `comment:"URL for the list source (when applicable).\nRequired for:\n- Trakt public lists: Full Trakt list URL\n-" displayname:"External List URL" longcomment:"URL for the list source (when applicable).\nRequired for:\n- Trakt public lists: Full Trakt list URL\n- RSS feeds: RSS feed URL\n- IMDB watchlists: IMDB watchlist URL\nNot needed for popular/trending lists or local files.\nExample: 'https://trakt.tv/users/username/lists/listname'\nExample: 'https://rss.example.com/movies.xml'" toml:"url"`
	// Enabled indicates if this template is active
	Enabled     bool   `comment:"Enable or disable this list configuration.\nWhen true, this list will be processed during scheduled imports.\nWhen" displayname:"Enable List Processing" longcomment:"Enable or disable this list configuration.\nWhen true, this list will be processed during scheduled imports.\nWhen false, this list is ignored and won't be imported.\nUseful for temporarily disabling a list without deleting the config.\nDefault: true"                                                            toml:"enabled"`
	IMDBCSVFile string `comment:"Path to IMDB CSV export file (for type 'imdbcsv').\nThis should be a CSV file exported"                             displayname:"IMDB CSV File Path"     longcomment:"Path to IMDB CSV export file (for type 'imdbcsv').\nThis should be a CSV file exported from IMDB containing movie/show data.\nPath can be absolute or relative to the application directory.\nRequired when type is 'imdbcsv', ignored for other types.\nExample: './config/movies.csv' or '/path/to/imdb-export.csv'" toml:"imdb_csv_file"`
	// ManualConfigFile is the path of the toml file for manual media configuration
	// Used for series, audiobooks, and music manual monitoring
	ManualConfigFile string `comment:"Path to TOML manual configuration file.\nUsed for series, audiobooks, and music manual monitoring" displayname:"Manual Config File Path" longcomment:"Path to TOML manual configuration file.\nThis file contains manual media definitions for:\n- TV series with custom settings\n- Audiobooks by author or book series\n- Music by artist or album series\nPath can be absolute or relative to the application directory.\nExample: './config/series.toml', './config/audiobooks.toml', or './config/music.toml'" toml:"manual_config_file"`
	// TraktUsername is the username who owns the trakt list
	TraktUsername string `comment:"Trakt username for public list access (for Trakt list types).\nThis is the username of the" displayname:"Trakt List Owner Username" longcomment:"Trakt username for public list access (for Trakt list types).\nThis is the username of the person who created/owns the Trakt list.\nRequired for public Trakt lists (traktpublicmovielist, traktpublicshowlist).\nNot needed for popular/trending lists as they don't belong to specific users.\nExample: 'moviefan123' or 'tvshowlover'" toml:"trakt_username"`
	// TraktListName is the listname of the trakt list
	TraktListName string `comment:"Name of the Trakt list to import (for public Trakt lists).\nThis is the list name" displayname:"Trakt List Name" longcomment:"Name of the Trakt list to import (for public Trakt lists).\nThis is the list name as it appears in the Trakt URL.\nRequired for public Trakt lists (traktpublicmovielist, traktpublicshowlist).\nNot needed for popular/trending lists.\nExample: 'my-favorites' or 'must-watch-movies'" toml:"trakt_listname"`
	// TraktListType is the listtype of the trakt list
	TraktListType string `comment:"Content type for Trakt lists (for Trakt list types).\nSpecifies whether the list contains movies or" displayname:"Trakt Content Type" longcomment:"Content type for Trakt lists (for Trakt list types).\nSpecifies whether the list contains movies or TV shows.\nAvailable options:\n- 'movie': List contains movies\n- 'show': List contains TV shows/series\nRequired for public Trakt lists to determine content type.\nExample: 'movie' for movie lists, 'show' for TV series lists" toml:"trakt_listtype"`
	// Limit is how many entries should only be processed
	Limit string `comment:"Maximum number of items to import from this list.\nSet to limit large lists to prevent" displayname:"Maximum Items To Import" longcomment:"Maximum number of items to import from this list.\nSet to limit large lists to prevent overwhelming the system.\nUse '0' or leave empty to import all items from the list.\nHigher numbers import more items but take longer to process.\nUseful for testing or limiting popular lists to top items.\nExample: '50' for top 50 items, '0' for all items\nDefault: '0' (all items)" toml:"limit"`
	// MinVotes only import if that number of imdb votes have been reached
	MinVotes int `comment:"Minimum IMDB vote count required for import (filtering criterion).\nOnly items with at least this many" displayname:"Minimum IMDB Vote Count" longcomment:"Minimum IMDB vote count required for import (filtering criterion).\nOnly items with at least this many IMDB votes will be imported.\nHelps filter out obscure or low-quality content.\nSet to 0 to disable vote filtering.\nHigher values result in more popular/mainstream content.\nExample: 1000 for moderately popular, 10000 for very popular\nDefault: 0 (no filtering)" toml:"min_votes"`
	// MinRating only import if that imdb rating has been reached
	MinRating float32 `comment:"Minimum IMDB rating required for import (filtering criterion).\nOnly items with at least this IMDB rating" displayname:"Minimum IMDB Rating" longcomment:"Minimum IMDB rating required for import (filtering criterion).\nOnly items with at least this IMDB rating will be imported.\nHelps filter out low-quality or poorly rated content.\nRating scale is 0.0 to 10.0 (IMDB standard).\nSet to 0.0 to disable rating filtering.\nExample: 6.5 for decent quality, 7.5 for high quality\nDefault: 0.0 (no filtering)" toml:"min_rating"`
	// Excludegenre don't import if it's one of the configured genres
	Excludegenre []string `comment:"List of genres to exclude from import (filtering criterion).\nItems matching any of these genres will" displayname:"Genres To Exclude" longcomment:"List of genres to exclude from import (filtering criterion).\nItems matching any of these genres will be skipped during import.\nUse exact genre names as they appear in IMDB/TMDB/Trakt.\nCommon genres: Action, Comedy, Drama, Horror, Romance, Sci-Fi, Thriller\nLeave empty to disable genre exclusion filtering.\nExample: ['Horror', 'Documentary', 'Reality-TV']" multiline:"true" toml:"exclude_genre"`
	// Includegenre only import if it's one of the configured genres
	Includegenre []string `comment:"List of genres to include in import (filtering criterion).\nOnly items matching at least one of" displayname:"Genres To Include" longcomment:"List of genres to include in import (filtering criterion).\nOnly items matching at least one of these genres will be imported.\nUse exact genre names as they appear in IMDB/TMDB/Trakt.\nCommon genres: Action, Comedy, Drama, Horror, Romance, Sci-Fi, Thriller\nLeave empty to disable genre inclusion filtering (import all genres).\nExample: ['Action', 'Sci-Fi', 'Thriller']" multiline:"true" toml:"include_genre"`
	// ExcludegenreLen is the length of Excludegenre
	ExcludegenreLen int `toml:"-"`
	// IncludegenreLen is the length of Includegenre
	IncludegenreLen int `toml:"-"`
	// URLExtensions for discover
	TmdbDiscover []string `comment:"TMDB Discover API URL parameters for dynamic content discovery.\nUse TMDB Discover API parameters to find" displayname:"TMDB Discover Parameters" longcomment:"TMDB Discover API URL parameters for dynamic content discovery.\nUse TMDB Discover API parameters to find movies/shows matching specific criteria.\nParameters should be in URL query format without the base URL.\nSee TMDB API docs: https://developer.themoviedb.org/reference/discover-movie\nSee TMDB API docs: https://developer.themoviedb.org/reference/discover-tv\nExample: ['sort_by=popularity.desc&vote_average.gte=7']\nExample: ['with_genres=28,12&release_date.gte=2020-01-01']" toml:"tmdb_discover"`
	// List IDs of TMDB Lists
	TmdbList       []int `comment:"List of TMDB list IDs to import from.\nThese are numeric IDs of public lists on"                       displayname:"TMDB List IDs"           longcomment:"List of TMDB list IDs to import from.\nThese are numeric IDs of public lists on The Movie Database.\nFind list IDs in TMDB list URLs: themoviedb.org/list/{ID}\nMultiple list IDs can be specified to import from several lists.\nRequires themoviedb_apikey to be configured in general settings.\nExample: [1, 28, 1000] for list IDs 1, 28, and 1000"      toml:"tmdb_list"`
	RemoveFromList bool  `comment:"Remove items from the list after successful processing.\nWhen true, items are removed from the source" displayname:"Remove After Processing" longcomment:"Remove items from the list after successful processing.\nWhen true, items are removed from the source list after being imported.\nUseful for one-time imports or clearing processed items from lists.\nWhen false, items remain in the list for future processing.\nCaution: This permanently modifies the source list.\nDefault: false (keep items in list)" toml:"remove_from_list"`

	// PlexServerURL is the URL of the Plex Media Server
	PlexServerURL string `comment:"URL of the Plex Media Server for watchlist access.\nShould include protocol and port if needed" displayname:"Plex Server URL" longcomment:"URL of the Plex Media Server for watchlist access.\nShould include protocol (http:// or https://) and port if needed.\nRequired for Plex watchlist integration (type 'plexwatchlist').\nExample: 'https://plex.example.com:32400' or 'http://192.168.1.100:32400'\nLeave empty if not using Plex integration." toml:"plex_server_url"`
	// PlexToken is the authentication token for Plex API access
	PlexToken string `comment:"Plex authentication token for API access.\nRequired to access Plex watchlists and user data" displayname:"Plex Authentication Token" longcomment:"Plex authentication token for API access.\nRequired to access Plex watchlists and user data.\nGet your token from Plex Web App: Account Settings > Privacy > Show Token\nOr use the Plex API to generate a token programmatically.\nRequired for Plex watchlist integration (type 'plexwatchlist').\nExample: 'xxxxxxxxxxxxxxxxxxxx'\nKeep this token secret and secure." toml:"plex_token"`
	// PlexUsername is the Plex username for watchlist access
	PlexUsername string `comment:"Plex username whose watchlist to import from.\nThis should be the username of the Plex account" displayname:"Plex Username" longcomment:"Plex username whose watchlist to import from.\nThis should be the username of the Plex account whose watchlist you want to monitor.\nRequired for Plex watchlist integration (type 'plexwatchlist').\nUsually the same as your Plex account email or display name.\nExample: 'john.doe@example.com' or 'moviefan123'" toml:"plex_username"`

	// JellyfinServerURL is the URL of the Jellyfin Media Server
	JellyfinServerURL string `comment:"URL of the Jellyfin Media Server for watchlist access.\nShould include protocol and port if needed" displayname:"Jellyfin Server URL" longcomment:"URL of the Jellyfin Media Server for watchlist access.\nShould include protocol (http:// or https://) and port if needed.\nRequired for Jellyfin watchlist integration (type 'jellyfinwatchlist').\nExample: 'https://jellyfin.example.com:8096' or 'http://192.168.1.100:8096'\nLeave empty if not using Jellyfin integration." toml:"jellyfin_server_url"`
	// JellyfinToken is the authentication token for Jellyfin API access
	JellyfinToken string `comment:"Jellyfin API key for authentication and access.\nRequired to access Jellyfin watchlists and user data" displayname:"Jellyfin API Key" longcomment:"Jellyfin API key for authentication and access.\nRequired to access Jellyfin watchlists and user data.\nGenerate from Jellyfin Admin Dashboard > API Keys > Add API Key\nRequired for Jellyfin watchlist integration (type 'jellyfinwatchlist').\nExample: 'xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx'\nKeep this API key secret and secure." toml:"jellyfin_token"`
	// JellyfinUsername is the Jellyfin username for watchlist access
	JellyfinUsername string `comment:"Jellyfin username whose watchlist to import from.\nThis should be the username of the Jellyfin user account" displayname:"Jellyfin Username" longcomment:"Jellyfin username whose watchlist to import from.\nThis should be the username of the Jellyfin user account whose watchlist you want to monitor.\nRequired for Jellyfin watchlist integration (type 'jellyfinwatchlist').\nExample: 'john_doe' or 'moviefan123'" toml:"jellyfin_username"`

	// --- Movie Scraper Configuration (when listtype = "moviescraper") ---

	// MovieScraperType is the type of scraper to use for movies
	MovieScraperType string `comment:"Type of scraper for movie import.\nOptions: 'htmlxpath', 'csrfapi'" displayname:"Movie Scraper Type" longcomment:"Type of scraper to use for movie import when type='moviescraper'.\nAvailable options:\n- 'htmlxpath': HTML/XPath scraper for static web pages\n- 'csrfapi': CSRF-protected JSON API scraper\nRequired when type is 'moviescraper', ignored for other types.\nExample: 'htmlxpath' for HTML pages, 'csrfapi' for API endpoints" toml:"movie_scraper_type"`

	// MovieScraperStartURL is the starting URL for the movie scraper
	MovieScraperStartURL string `comment:"Starting URL for the movie scraper.\nUsed to obtain tokens and start scraping" displayname:"Movie Scraper Start URL" longcomment:"Starting URL for the movie scraper.\nFor htmlxpath: First page URL to start scraping movies from.\nFor csrfapi: URL to extract CSRF token from before making API calls.\nRequired when type is 'moviescraper'.\nExample: 'https://example.com/movies/' or 'https://example.com/api/auth'" toml:"movie_scraper_start_url"`

	// MovieScraperSiteURL is the base URL for the movie scraping site
	MovieScraperSiteURL string `comment:"Base URL for the scraping site.\nUsed for constructing full URLs from relative paths" displayname:"Movie Scraper Site URL" longcomment:"Base URL for the movie scraping site.\nUsed for constructing full URLs from relative paths found during scraping.\nOptional but recommended for resolving relative URLs correctly.\nExample: 'https://example.com'" toml:"movie_scraper_site_url"`

	// MovieScraperSiteID is the numeric ID for the scraping site
	MovieScraperSiteID uint `comment:"Numeric ID for the scraping site.\nOptional field for logging and reference" displayname:"Movie Scraper Site ID" longcomment:"Numeric ID for the movie scraping site.\nOptional field used primarily for logging and reference purposes.\nCan match external site IDs if applicable.\nExample: 123 or 0 (default)" toml:"movie_scraper_site_id"`

	// --- HTML/XPath Movie Scraper Settings ---

	// MovieSceneNodeXPath is the XPath to select movie containers
	MovieSceneNodeXPath string `comment:"XPath selector for movie containers.\nRequired for htmlxpath scraper type" displayname:"Movie Node XPath" longcomment:"XPath expression to select each movie/item container element on the page.\nRequired for htmlxpath scraper type.\nEach matched node represents one movie to be imported.\nExample: '//div[@class=\"movie-item\"]' or '//article[contains(@class,\"film\")]'" toml:"movie_scene_node_xpath"`

	// MovieTitleXPath is the XPath for title extraction
	MovieTitleXPath string `comment:"XPath selector for extracting movie title.\nRequired for htmlxpath scraper type" displayname:"Movie Title XPath" longcomment:"XPath expression relative to movie node for extracting the title.\nRequired for htmlxpath scraper type.\nCan extract from element text or attribute (set via movie_title_attribute).\nExample: './/h2[@class=\"title\"]/a' or './/span[@class=\"movie-name\"]'" toml:"movie_title_xpath"`

	// MovieYearXPath is the XPath for year extraction
	MovieYearXPath string `comment:"XPath selector for extracting release year.\nOptional for htmlxpath scraper type" displayname:"Movie Year XPath" longcomment:"XPath expression relative to movie node for extracting the release year.\nOptional for htmlxpath scraper type.\nIf IMDB ID is not found, title+year will be used to search TMDB.\nExample: './/span[@class=\"year\"]' or './/time[@datetime]'" toml:"movie_year_xpath"`

	// MovieImdbIDXPath is the XPath for IMDB ID extraction
	MovieImdbIDXPath string `comment:"XPath selector for extracting IMDB ID.\nOptional for htmlxpath scraper type" displayname:"Movie IMDB ID XPath" longcomment:"XPath expression relative to movie node for extracting IMDB ID.\nOptional for htmlxpath scraper type.\nIf IMDB ID is found directly, movie is imported immediately.\nIf not found, will attempt to search TMDB using title+year.\nExample: './/a[contains(@href,\"imdb.com\")]/@href' or './/span[@data-imdb]'" toml:"movie_imdbid_xpath"`

	// MovieURLXPath is the XPath for URL extraction
	MovieURLXPath string `comment:"XPath selector for extracting movie detail URL.\nOptional for htmlxpath scraper type" displayname:"Movie URL XPath" longcomment:"XPath expression relative to movie node for extracting detail page URL.\nOptional for htmlxpath scraper type.\nUseful for logging and tracking which page a movie came from.\nExample: './/a[@class=\"movie-link\"]/@href' or './/h2/a'" toml:"movie_url_xpath"`

	// MovieRatingXPath is the XPath for rating extraction
	MovieRatingXPath string `comment:"XPath selector for extracting rating/score.\nOptional for htmlxpath scraper type" displayname:"Movie Rating XPath" longcomment:"XPath expression relative to movie node for extracting rating or score.\nOptional for htmlxpath scraper type.\nCan be used for filtering or metadata enhancement.\nExample: './/span[@class=\"rating\"]' or './/div[@class=\"score\"]/text()'" toml:"movie_rating_xpath"`

	// MovieGenreXPath is the XPath for genre extraction
	MovieGenreXPath string `comment:"XPath selector for extracting genre(s).\nOptional for htmlxpath scraper type" displayname:"Movie Genre XPath" longcomment:"XPath expression relative to movie node for extracting genre(s).\nOptional for htmlxpath scraper type.\nCan match multiple genre elements which will be combined.\nExample: './/span[@class=\"genre\"]' or './/a[@class=\"category\"]'" toml:"movie_genre_xpath"`

	// MovieReleaseDateXPath is the XPath for release date extraction
	MovieReleaseDateXPath string `comment:"XPath selector for extracting full release date.\nOptional for htmlxpath scraper type" displayname:"Movie Release Date XPath" longcomment:"XPath expression relative to movie node for extracting full release date.\nOptional for htmlxpath scraper type.\nUse this instead of year_xpath for more precise date information.\nExample: './/time[@class=\"release\"]/@datetime' or './/span[@class=\"date\"]'" toml:"movie_release_date_xpath"`

	// MovieTitleAttribute is the HTML attribute to extract title from
	MovieTitleAttribute string `comment:"HTML attribute to extract title from.\nOptional, defaults to text content" displayname:"Movie Title Attribute" longcomment:"HTML attribute to extract title from instead of element text content.\nOptional for htmlxpath scraper, leave empty to use text extraction.\nExample: 'title' to get <div title=\"Movie Name\">, or 'data-title' or 'aria-label'" toml:"movie_title_attribute"`

	// MovieURLAttribute is the HTML attribute to extract URL from
	MovieURLAttribute string `comment:"HTML attribute to extract URL from.\nOptional, defaults to 'href'" displayname:"Movie URL Attribute" longcomment:"HTML attribute to extract URL from.\nOptional for htmlxpath scraper, defaults to 'href' attribute.\nExample: 'href' for <a href=\"...\">, 'data-url', or 'src'" toml:"movie_url_attribute"`

	// MoviePaginationType specifies pagination style
	MoviePaginationType string `comment:"Pagination style: 'sequential' or 'offset'.\nOptional for htmlxpath scraper" displayname:"Movie Pagination Type" longcomment:"Pagination style for htmlxpath movie scraper.\nOptions:\n- 'sequential': Pages numbered 1, 2, 3, 4...\n- 'offset': Pages use offset values like 0, 12, 24, 36...\nDefault: 'sequential'\nExample: Use 'offset' if API uses skip/offset pagination" toml:"movie_pagination_type"`

	// MoviePageIncrement is the increment for offset pagination
	MoviePageIncrement int `comment:"Increment for offset pagination.\nOnly for htmlxpath with offset pagination" displayname:"Movie Page Increment" longcomment:"Increment value for offset pagination.\nOnly used for htmlxpath scraper when pagination_type='offset'.\nDefines how much to increment the offset for each page.\nExample: 12 for pages 0, 12, 24... or 20 for pages 0, 20, 40, 60..." toml:"movie_page_increment"`

	// MoviePageURLPattern is the URL pattern with {page} placeholder
	MoviePageURLPattern string `comment:"URL pattern with {page} placeholder.\nRequired for htmlxpath multi-page scraping" displayname:"Movie Page URL Pattern" longcomment:"URL pattern with {page} placeholder for pagination.\nRequired for htmlxpath scraper when scraping multiple pages.\nThe {page} placeholder will be replaced with page number or offset value.\nExample: 'https://example.com/movies/page/{page}' or 'https://example.com/films?offset={page}'" toml:"movie_page_url_pattern"`

	// --- CSRF API Movie Scraper Settings ---

	// MovieCSRFCookieName is the name of the cookie containing CSRF token
	MovieCSRFCookieName string `comment:"Name of cookie containing CSRF token.\nRequired for csrfapi scraper type" displayname:"Movie CSRF Cookie Name" longcomment:"Name of the cookie containing CSRF/XSRF token.\nRequired for csrfapi scraper type.\nToken will be extracted from this cookie after visiting start_url.\nExample: '_csrf', 'csrf_token', 'XSRF-TOKEN', or 'csrftoken'" toml:"movie_csrf_cookie_name"`

	// MovieCSRFHeaderName is the name of the header to send CSRF token in
	MovieCSRFHeaderName string `comment:"Name of header for sending CSRF token.\nRequired for csrfapi scraper type" displayname:"Movie CSRF Header Name" longcomment:"Name of the HTTP header to send CSRF token in for API requests.\nRequired for csrfapi scraper type.\nExample: 'csrf-token', 'X-CSRF-Token', 'X-XSRF-TOKEN', or 'X-CSRFToken'" toml:"movie_csrf_header_name"`

	// MovieAPIURLPattern is the API URL pattern with {page} placeholder
	MovieAPIURLPattern string `comment:"API URL pattern with {page} placeholder.\nRequired for csrfapi scraper type" displayname:"Movie API URL Pattern" longcomment:"API URL pattern with {page} placeholder for pagination.\nRequired for csrfapi scraper type.\nThe {page} placeholder will be replaced with page number for each request.\nExample: 'https://example.com/api/movies?page={page}' or 'https://example.com/api/v1/films/{page}'" toml:"movie_api_url_pattern"`

	// MoviePageStartIndex is the starting page index
	MoviePageStartIndex int `comment:"Starting page index (0 or 1).\nOptional for csrfapi scraper" displayname:"Movie Page Start Index" longcomment:"Starting page index for pagination.\nOptional for csrfapi scraper, defaults to 1.\nUse 1 for 1-based pagination (pages 1, 2, 3...).\nUse 0 for 0-based pagination (pages 0, 1, 2...).\nExample: 1 for most APIs, 0 for zero-indexed APIs" toml:"movie_page_start_index"`

	// MovieResultsArrayPath is the JSON path to results array
	MovieResultsArrayPath string `comment:"JSON path to results array.\nRequired for csrfapi scraper type" displayname:"Movie Results Array Path" longcomment:"JSON path to the array of movie results in API response.\nRequired for csrfapi scraper type.\nSupports nested paths using dot notation.\nExample: 'movies', 'data.results', 'response.films', or 'items'" toml:"movie_results_array_path"`

	// MovieTitleField is the JSON field for title
	MovieTitleField string `comment:"JSON field name for movie title.\nRequired for csrfapi scraper type" displayname:"Movie Title Field" longcomment:"JSON field name for movie title in API response objects.\nRequired for csrfapi scraper type.\nExample: 'name', 'title', 'movie_name', or 'originalTitle'" toml:"movie_title_field"`

	// MovieYearField is the JSON field for year
	MovieYearField string `comment:"JSON field name for release year.\nOptional for csrfapi scraper type" displayname:"Movie Year Field" longcomment:"JSON field name for release year in API response objects.\nOptional for csrfapi scraper type.\nIf IMDB ID is not found, title+year will be used to search TMDB.\nExample: 'year', 'release_year', 'releaseYear', or 'releaseDate'" toml:"movie_year_field"`

	// MovieImdbIDField is the JSON field for IMDB ID
	MovieImdbIDField string `comment:"JSON field name for IMDB ID.\nOptional for csrfapi scraper type" displayname:"Movie IMDB ID Field" longcomment:"JSON field name for IMDB ID in API response objects.\nOptional for csrfapi scraper type.\nIf IMDB ID is found, movie is imported immediately.\nIf not found, will search TMDB using title+year.\nExample: 'imdb_id', 'imdbID', 'externalIds.imdb', or 'imdb'" toml:"movie_imdbid_field"`

	// MovieURLField is the JSON field for URL
	MovieURLField string `comment:"JSON field name for movie URL.\nOptional for csrfapi scraper type" displayname:"Movie URL Field" longcomment:"JSON field name for movie detail URL in API response objects.\nOptional for csrfapi scraper type.\nUseful for logging and tracking source URLs.\nExample: 'path', 'url', 'movie_url', or 'link'" toml:"movie_url_field"`

	// MovieRatingField is the JSON field for rating
	MovieRatingField string `comment:"JSON field name for rating/score.\nOptional for csrfapi scraper type" displayname:"Movie Rating Field" longcomment:"JSON field name for rating or score in API response objects.\nOptional for csrfapi scraper type.\nCan be used for filtering or metadata enhancement.\nExample: 'rating', 'score', 'vote_average', 'imdbRating', or 'tmdbRating'" toml:"movie_rating_field"`

	// MovieGenreField is the JSON field for genre
	MovieGenreField string `comment:"JSON field name for genre(s).\nOptional for csrfapi scraper type" displayname:"Movie Genre Field" longcomment:"JSON field name for genre(s) in API response objects.\nOptional for csrfapi scraper type.\nCan be array of strings or comma-separated string.\nExample: 'genres', 'genre', 'category', or 'genreNames'" toml:"movie_genre_field"`

	// MovieReleaseDateField is the JSON field for release date
	MovieReleaseDateField string `comment:"JSON field name for release date.\nOptional for csrfapi scraper type" displayname:"Movie Release Date Field" longcomment:"JSON field name for full release date in API response objects.\nOptional for csrfapi scraper type.\nUse this instead of year_field for more precise date information.\nExample: 'releaseDate', 'release_date', 'premiered', or 'first_air_date'" toml:"movie_release_date_field"`

	// --- Common Movie Scraper Settings ---

	// MovieDateFormat is the date format string (Go time format)
	MovieDateFormat string `comment:"Date format string (Go time format).\nLeave empty for ISO 8601" displayname:"Movie Date Format" longcomment:"Date format string in Go time format for parsing dates.\nLeave empty for automatic ISO 8601 parsing.\nExamples:\n- '2006-01-02' for YYYY-MM-DD format\n- 'Jan 2, 2006' for 'May 15, 2024' format\n- '2006' for just year\n- '' (empty) for automatic ISO 8601 parsing" toml:"movie_date_format"`

	// MovieWaitSeconds is the seconds to wait between requests
	MovieWaitSeconds int `comment:"Seconds to wait between requests.\nDefault: 2" displayname:"Movie Wait Seconds" longcomment:"Seconds to wait between scraper requests to avoid rate limiting.\nDefault: 2 seconds if not specified.\nIncrease for strict rate-limited sites to avoid being blocked.\nExample: 2 for normal sites, 5 for careful scraping, 15 for strict rate limits" toml:"movie_wait_seconds"`

	// --- Chart/Bestseller Scraper Configuration (when listtype = "musiccharts" or "bookbestsellers") ---
	// Use the URL field as the full chart URL (including optional date path segments).
	// offiziellecharts.de date example:  URL + "/for-date-1772209337000"
	// officialcharts.com  date example:  URL + "/20260220/7502/"

	// ChartEntryNodeXPath is the XPath to select each chart/bestseller entry node
	ChartEntryNodeXPath string `comment:"XPath selector for chart/bestseller entries.\nRequired for musiccharts and bookbestsellers list types" displayname:"Chart Entry Node XPath" longcomment:"XPath expression to select each chart or bestseller entry element on the page.\nRequired for musiccharts and bookbestsellers list types.\nExample (offiziellecharts.de): '//div[contains(@class,\"chart-element\")]'\nExample (officialcharts.com):  '//li[contains(@class,\"chart-item\")]'\nExample (bestsellerliste.de): '//div[contains(@class,\"list-entry\")]'" toml:"chart_entry_node_xpath"`

	// ChartTitleXPath is the XPath for album/book title extraction relative to the entry node
	ChartTitleXPath string `comment:"XPath selector for album/book title.\nRequired for musiccharts and bookbestsellers" displayname:"Chart Title XPath" longcomment:"XPath expression relative to the entry node for extracting the album or book title.\nRequired for musiccharts and bookbestsellers list types.\nExample: './/span[@class=\"title\"]' or './/h3'" toml:"chart_title_xpath"`

	// ChartArtistXPath is the XPath for artist/author name extraction relative to the entry node
	ChartArtistXPath string `comment:"XPath selector for artist or author name.\nOptional for musiccharts and bookbestsellers" displayname:"Chart Artist/Author XPath" longcomment:"XPath expression relative to the entry node for extracting the artist or author name.\nOptional — leave empty if the page only lists titles.\nExample: './/span[@class=\"artist\"]' or './/div[@class=\"author\"]'" toml:"chart_artist_xpath"`

	// ChartTitleAttribute is the optional HTML attribute to extract the title from (defaults to inner text)
	ChartTitleAttribute string `comment:"HTML attribute for title.\nOptional, defaults to inner text" displayname:"Chart Title Attribute" longcomment:"HTML attribute to extract the album/book title from instead of the element's inner text.\nLeave empty to use inner text (most common).\nExample: 'title', 'data-title', or 'aria-label'" toml:"chart_title_attribute"`

	// ChartArtistAttribute is the optional HTML attribute to extract the artist/author from (defaults to inner text)
	ChartArtistAttribute string `comment:"HTML attribute for artist/author.\nOptional, defaults to inner text" displayname:"Chart Artist/Author Attribute" longcomment:"HTML attribute to extract the artist/author name from instead of the element's inner text.\nLeave empty to use inner text (most common).\nExample: 'data-artist' or 'aria-label'" toml:"chart_artist_attribute"`

	// ChartDefaultArtist is the fallback artist/author name used when the XPath yields an empty value.
	// Useful for compilation charts where there is no per-entry artist (e.g. set to "Various Artists").
	ChartDefaultArtist string `comment:"Fallback artist or author when XPath returns empty.\nE.g. 'Various Artists' for compilation charts" displayname:"Chart Default Artist/Author" longcomment:"Fallback artist or author name used when chart_artist_xpath is empty or the XPath returns no text.\nTypically set to 'Various Artists' for compilation album charts where no per-entry artist is listed.\nLeave empty to keep artist/author blank when not found.\nExample: 'Various Artists'" toml:"chart_default_artist"`

	// ChartDateURLPattern is the URL template used when importing a chart for a specific date.
	// Use {date} as the placeholder for the formatted date value.
	// Leave empty to disable date-specific imports for this list.
	// Examples:
	//   offiziellecharts.de:  "https://www.offiziellecharts.de/charts/compilation/for-date-{date}"
	//   officialcharts.com:   "https://www.officialcharts.com/charts/albums-chart/{date}/7501/"
	ChartDateURLPattern string `comment:"URL pattern for date-specific chart imports.\nUse {date} as placeholder.\nExample: 'https://www.offiziellecharts.de/charts/compilation/for-date-{date}'" displayname:"Chart Date URL Pattern" longcomment:"URL template used when importing a chart for a specific date via the API.\nThe {date} placeholder is replaced with the date formatted according to chart_date_format.\nLeave empty to disable date-specific imports for this list.\nExamples:\n  offiziellecharts.de: 'https://www.offiziellecharts.de/charts/compilation/for-date-{date}'\n  officialcharts.com:  'https://www.officialcharts.com/charts/albums-chart/{date}/7501/'" toml:"chart_date_url_pattern"`

	// ChartDateFormat controls how the date is formatted into {date} in ChartDateURLPattern.
	// Special values:
	//   "timestamp_ms" (or empty) - Unix millisecond timestamp, e.g. 1704412800000
	//   "timestamp"               - Unix second timestamp, e.g. 1704412800
	// Any other value is treated as a Go time layout string, e.g. "20060102" for YYYYMMDD.
	ChartDateFormat string `comment:"How to format the date in the URL.\n'timestamp_ms' = Unix ms, 'timestamp' = Unix s, or a Go time layout like '20060102'" displayname:"Chart Date Format" longcomment:"Controls how the date value is formatted when building the date-specific URL.\nSpecial values:\n  'timestamp_ms' or empty - Unix millisecond timestamp (e.g. 1704412800000)\n  'timestamp'             - Unix second timestamp   (e.g. 1704412800)\nAny other value is used as a Go time.Format layout string:\n  '20060102' for YYYYMMDD  (officialcharts.com)\n  '2006-01-02' for ISO date\nDefault: 'timestamp_ms' (empty)" toml:"chart_date_format"`
}

// IndexersConfig defines the configuration for indexers.
type IndexersConfig struct {
	// Name is the name of the template
	Name string `comment:"Unique name for this indexer configuration.\nUsed to identify this indexer in quality profiles and logs.\nChoose" displayname:"Indexer Configuration Name" longcomment:"Unique name for this indexer configuration.\nUsed to identify this indexer in quality profiles and logs.\nChoose a descriptive name that identifies the indexer site.\nExample: 'nzbgeek', 'drunkenslug', 'nzbfinder'" toml:"name"`

	// IndexerType is the type of the indexer, currently has to be newznab
	IndexerType string `comment:"Protocol type used by this indexer.\nCurrently only 'newznab' is supported.\nNewznab is the standard API used" displayname:"Indexer Protocol Type" longcomment:"Protocol type used by this indexer.\nCurrently only 'newznab' is supported.\nNewznab is the standard API used by most Usenet indexers.\nTorrent indexers using Newznab-compatible APIs also use 'newznab'.\nExample: 'newznab'" toml:"type"`

	// URL is the main url of the indexer
	URL string `comment:"Base URL of the indexer website.\nThis should be the main domain without any API paths.\nDo" displayname:"Indexer Base URL" longcomment:"Base URL of the indexer website.\nThis should be the main domain without any API paths.\nDo not include '/api' or other paths - they're added automatically.\nMust include protocol (http:// or https://).\nExample: 'https://api.nzbgeek.info' or 'https://drunkenslug.com'" toml:"url"`

	// Apikey is the apikey for the indexer
	Apikey string `comment:"API key for accessing this indexer.\nObtained from your indexer account settings or profile page for authentication" displayname:"Indexer API Key" longcomment:"API key for accessing this indexer.\nObtained from your indexer account settings or profile page.\nRequired for authentication and to track your usage limits.\nKeep this key secret and don't share it publicly.\nSome indexers call this 'API Token' or 'RSS Key'.\nExample: 'a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6q7r8s9t0'" toml:"apikey"`

	// Userid is the userid for rss queries to the indexer if needed
	Userid string `comment:"User ID for RSS feed access (if required by the indexer).\nSome indexers require both API" displayname:"Indexer User ID" longcomment:"User ID for RSS feed access (if required by the indexer).\nSome indexers require both API key and user ID for RSS feeds.\nUsually found in your indexer account settings alongside the API key.\nLeave empty if the indexer doesn't require a user ID.\nExample: '12345' or 'username123'" toml:"userid"`

	// Enabled indicates if this template is active
	Enabled bool `comment:"Enable or disable this indexer configuration.\nWhen true, this indexer will be used for searches.\nWhen false," displayname:"Enable Indexer Configuration" longcomment:"Enable or disable this indexer configuration.\nWhen true, this indexer will be used for searches.\nWhen false, this indexer is ignored and won't be queried.\nUseful for temporarily disabling problematic indexers.\nDefault: true" toml:"enabled"`

	// Rssenabled indicates if this template is active for rss queries
	Rssenabled bool `comment:"Enable RSS feed monitoring for this indexer.\nWhen true, RSS feeds are checked for new releases" displayname:"Enable RSS Monitoring" longcomment:"Enable RSS feed monitoring for this indexer.\nWhen true, RSS feeds are checked for new releases automatically.\nWhen false, only manual searches are performed on this indexer.\nRSS monitoring helps catch new releases quickly.\nDefault: true" toml:"rss_enabled"`

	// Addquotesfortitlequery indicates if quotes should be added to a title query
	Addquotesfortitlequery bool `comment:"Add quotes around title searches for exact matching. \nWhen true, searches like 'Movie Title' become '\"Movie Title\"'. \nImproves search accuracy but may reduce results. \nSome indexers work better with quotes, others without. \nTest with your indexer to see which works better. \nDefault: false" displayname:"Add Quotes Title Search" toml:"add_quotes_for_title_query"`

	// MaxEntries is the maximum number of entries to process, default is 100
	MaxEntries    uint16 `comment:"Maximum number of search results to retrieve per query.\nHigher values get more results but increase" displayname:"Maximum Search Results" longcomment:"Maximum number of search results to retrieve per query.\nHigher values get more results but increase processing time and API usage.\nSome indexers have limits on how many results they return.\nTypical range: 50-500 depending on indexer capabilities.\nDefault: 100" toml:"max_entries"`
	MaxEntriesStr string `                                                                                                                                                                                                                                                                                                                                                                                                                                          toml:"-"`
	// RssEntriesloop is the number of rss calls to make to find last processed release, default is 2
	RssEntriesloop uint8 `comment:"Number of RSS feed pages to check for finding the last processed release.\nHigher values ensure" displayname:"RSS Feed Pages" longcomment:"Number of RSS feed pages to check for finding the last processed release.\nHigher values ensure no releases are missed but increase API usage.\nUsed to determine where to resume RSS processing after downtime.\nIncrease if you frequently miss releases during outages.\nRange: 1-10 depending on indexer activity and downtime frequency.\nDefault: 2" toml:"rss_entries_loop"`

	// OutputAsJSON indicates if the indexer should return json instead of xml
	// Not recommended since the conversion is sometimes different
	OutputAsJSON bool `comment:"Request JSON format instead of XML from the indexer.\nSome indexers support JSON responses which can" displayname:"Request JSON Format" longcomment:"Request JSON format instead of XML from the indexer.\nSome indexers support JSON responses which can be faster to parse.\nNot recommended as JSON conversion may lose data or format differently.\nOnly enable if you experience XML parsing issues with this indexer.\nDefault: false (use XML format)" toml:"output_as_json"`

	// Customapi is used if the indexer needs a different value then 'apikey' for the key
	Customapi string `comment:"Custom API parameter name if the indexer doesn't use 'apikey'.\nMost indexers use 'apikey' but some" displayname:"Custom API Parameter Name" longcomment:"Custom API parameter name if the indexer doesn't use 'apikey'.\nMost indexers use 'apikey' but some use different parameter names.\nCommon alternatives: 'api_key', 'token', 'key', 'apitoken'\nLeave empty if the indexer uses the standard 'apikey' parameter.\nCheck your indexer's API documentation for the correct parameter name.\nExample: 'api_key' or 'token'" toml:"custom_api"`

	// Customurl is used if the indexer needs a different url then url/api/ or url/rss/
	Customurl string `comment:"Custom API URL path if the indexer doesn't use standard paths.\nMost indexers use '/api' for API calls and '/rss' for RSS feeds" displayname:"Custom API Path" longcomment:"Custom API URL path if the indexer doesn't use standard paths.\nMost indexers use '/api' for API calls and '/rss' for RSS feeds.\nSome indexers use different paths like '/newznab/api' or '/api/v1'.\nLeave empty if the indexer uses standard '/api' and '/rss' paths.\nInclude the leading slash in your custom path.\nExample: '/newznab/api' or '/api/v2'" toml:"custom_url"`

	// Customrssurl is used if the indexer uses a custom rss url (not url/rss/)
	Customrssurl string `comment:"Custom RSS URL path if different from the standard '/rss'.\nSome indexers use non-standard RSS paths" displayname:"Custom RSS Path" longcomment:"Custom RSS URL path if different from the standard '/rss'.\nSome indexers use non-standard RSS paths or completely different URLs.\nCan be a relative path (e.g., '/feed') or absolute URL.\nLeave empty if the indexer uses the standard '/rss' path.\nCheck your indexer's RSS feed URL in their documentation.\nExample: '/feed' or 'https://indexer.com/rss.php'" toml:"custom_rss_url"`

	// Customrsscategory is used if the indexer uses something other than &t= for rss categories
	Customrsscategory string `comment:"Custom RSS category parameter if different from the standard '&t='.\nMost Newznab indexers use '&t=' for category filtering in RSS feeds" displayname:"Custom RSS Category Parameter" longcomment:"Custom RSS category parameter if different from the standard '&t='.\nMost Newznab indexers use '&t=' for category filtering in RSS feeds.\nSome indexers use different parameters like '&cat=' or '&category='.\nLeave empty if the indexer uses the standard '&t=' parameter.\nCheck your indexer's RSS URL format for the correct parameter.\nExample: '&cat=' or '&category='" toml:"custom_rss_category"`

	// Limitercalls is the number of calls allowed in Limiterseconds
	Limitercalls int `comment:"Number of API calls allowed within the limiter_seconds timeframe.\nUsed to respect the indexer's rate limiting" displayname:"API Calls Per Window" longcomment:"Number of API calls allowed within the limiter_seconds timeframe.\nUsed to respect the indexer's rate limiting to avoid being banned.\nCheck your indexer's API documentation for their rate limits.\nCommon limits: 1-10 calls per second depending on the indexer.\nSet conservatively to avoid hitting limits during busy periods.\nDefault: 1" toml:"limiter_calls"`

	// Limiterseconds is the number of seconds for Limitercalls calls
	Limiterseconds uint8 `comment:"Time window in seconds for the limiter_calls limit.\nDefines the period over which the call limit" displayname:"Rate Limit Window Seconds" longcomment:"Time window in seconds for the limiter_calls limit.\nDefines the period over which the call limit applies.\nTogether with limiter_calls, controls the rate limiting.\nExample: limiter_calls=5, limiter_seconds=10 = 5 calls per 10 seconds.\nMost indexers use per-second limits, so typically set to 1.\nDefault: 1" toml:"limiter_seconds"`

	// LimitercallsDaily is the number of calls allowed daily, 0 is unlimited
	LimitercallsDaily int `comment:"Maximum number of API calls allowed per day (24-hour period).\nHelps stay within daily API limits" displayname:"Daily API Call Limit" longcomment:"Maximum number of API calls allowed per day (24-hour period).\nHelps stay within daily API limits imposed by some indexers.\nSet to 0 for unlimited daily calls (only rate limiting applies).\nCheck your indexer account for daily API call limits.\nUseful for free accounts with daily restrictions.\nExample: 100 for limited accounts, 0 for unlimited\nDefault: 0 (unlimited)" toml:"limiter_calls_daily"`

	// MaxAge is the maximum age of releases in days
	MaxAge uint16 `comment:"Maximum age of releases to consider during searches (in days).\nReleases older than this age will" displayname:"Maximum Release Age Days" longcomment:"Maximum age of releases to consider during searches (in days).\nReleases older than this age will be ignored.\nHelps focus on recent releases and reduces processing time.\nSet to 0 to disable age filtering (search all releases).\nTypical values: 30-365 days depending on content preferences.\nExample: 90 for 3 months, 365 for 1 year\nDefault: 0 (no age limit)" toml:"max_age"`

	// DisableTLSVerify disables SSL Certificate Checks
	DisableTLSVerify bool `comment:"Disable SSL certificate verification for this indexer.\nOnly enable if the indexer has SSL certificate issues.\nThis" displayname:"Disable SSL Certificate Verification" longcomment:"Disable SSL certificate verification for this indexer.\nOnly enable if the indexer has SSL certificate issues.\nThis reduces security by allowing invalid/expired certificates.\nUse only as a last resort for indexers with certificate problems.\nMost indexers should work with SSL verification enabled.\nDefault: false (SSL verification enabled)" toml:"disable_tls_verify"`

	// DisableCompression disables compression of data
	DisableCompression bool `comment:"Disable HTTP compression for requests to this indexer.\nCompression reduces bandwidth usage and improves performance.\nOnly disable" displayname:"Disable HTTP Compression" longcomment:"Disable HTTP compression for requests to this indexer.\nCompression reduces bandwidth usage and improves performance.\nOnly disable if the indexer has issues with compressed responses.\nMost indexers support compression and it should remain enabled.\nMay be needed for older or misconfigured indexers.\nDefault: false (compression enabled)" toml:"disable_compression"`

	// TimeoutSeconds is the timeout in seconds for queries
	TimeoutSeconds uint16 `comment:"Maximum time to wait for indexer responses (in seconds).\nRequests taking longer than this will be" displayname:"Request Timeout Seconds" longcomment:"Maximum time to wait for indexer responses (in seconds).\nRequests taking longer than this will be cancelled.\nSet higher for slow indexers, lower for fast ones.\nToo low causes timeouts, too high delays error detection.\nTypical range: 30-120 seconds depending on indexer performance.\nExample: 60 for average indexers, 120 for slow ones\nDefault: 60" toml:"timeout_seconds"`

	TrustWithIMDBIDs bool `comment:"trust indexer imdb ids - can be problematic for RSS scans - some indexers tag wrong"                       displayname:"Trust Indexer IMDB IDs" toml:"trust_with_imdb_ids"`
	TrustWithTVDBIDs bool `comment:"Trust TVDB IDs provided by this indexer for TV show identification.\nWhen true, indexer-provided TVDB IDs" displayname:"Trust Indexer TVDB IDs" toml:"trust_with_tvdb_ids" longcomment:"Trust TVDB IDs provided by this indexer for TV show identification.\nWhen true, indexer-provided TVDB IDs are used for matching.\nWhen false, titles are used for matching instead of IDs.\nSome indexers provide incorrect TVDB IDs, especially in RSS feeds.\nDisable if you notice incorrect TV show matches from this indexer.\nDefault: false (don't trust indexer TVDB IDs)"`

	// CheckTitleOnIDSearch is a bool indicating if the title of the release should be checked during an id based search? - default: false
	CheckTitleOnIDSearch bool `comment:"Verify release titles even when searching by IMDB/TVDB ID.\nWhen true, both ID and title must" displayname:"Verify Title On ID Search" longcomment:"Verify release titles even when searching by IMDB/TVDB ID.\nWhen true, both ID and title must match for a release to be accepted.\nWhen false, only the ID needs to match (faster but less accurate).\nUseful when trust_with_imdb_ids or trust_with_tvdb_ids is enabled.\nHelps prevent incorrect matches from indexers with unreliable IDs.\nDefault: false (ID matching only)" toml:"check_title_on_id_search"`
}

type PathsConfig struct {
	// Name is the name of the media template
	Name string `comment:"Unique name for this path configuration.\nUsed to identify this path template in media configurations.\nChoose a" displayname:"Path Configuration Name" longcomment:"Unique name for this path configuration.\nUsed to identify this path template in media configurations.\nChoose a descriptive name that indicates the media type or purpose.\nExample: 'movies-4k', 'tv-shows', 'anime', 'documentaries'" toml:"name"`
	// Path is the path where the media will be stored
	Path string `comment:"Absolute path where media files will be organized and stored. \nThis is the root directory for your media library" displayname:"Media Storage Directory" longcomment:"Absolute path where media files will be organized and stored. \nThis is the root directory for your media library. \nMust be an absolute path accessible to the application. \nEnsure the directory exists and has proper read/write permissions. \nExample: '/media/movies', '/mnt/storage/tv-shows', 'D:\\\\Movies'" toml:"path"`
	// AllowedVideoExtensions lists the allowed video file extensions
	AllowedVideoExtensions []string `comment:"List of video file extensions that will be processed and renamed.\nThese files are considered main" displayname:"Video File Extensions" longcomment:"List of video file extensions that will be processed and renamed.\nThese files are considered main media files and will be renamed according to your naming scheme.\nInclude the dot (.) prefix for each extension.\nCommon video formats: .mkv, .mp4, .avi, .m4v, .wmv, .mov\nExample: ['.mkv', '.mp4', '.avi', '.m4v']" multiline:"true" toml:"allowed_video_extensions"`
	// AllowedVideoExtensionsLen is the number of allowed video extensions
	AllowedVideoExtensionsLen int `toml:"-"`
	// AllowedOtherExtensions lists other allowed file extensions
	AllowedOtherExtensions []string `comment:"List of non-video file extensions that will be copied alongside media files.\nThese are supplementary files" displayname:"Other File Extensions" longcomment:"List of non-video file extensions that will be copied alongside media files.\nThese are supplementary files like subtitles, NFOs, artwork, etc.\nThey will be renamed to match the main media file.\nInclude the dot (.) prefix for each extension.\nCommon formats: .srt, .nfo, .jpg, .png, .txt, .xml\nExample: ['.srt', '.nfo', '.jpg', '.png']" multiline:"true" toml:"allowed_other_extensions"`
	// AllowedOtherExtensionsLen is the number of other allowed extensions
	AllowedOtherExtensionsLen int `toml:"-"`
	// AllowedVideoExtensionsNoRename lists video extensions that should not be renamed
	AllowedVideoExtensionsNoRename []string `comment:"List of video file extensions that will be copied but NOT renamed.\nThese files are preserved" displayname:"Video Extensions No Rename" longcomment:"List of video file extensions that will be copied but NOT renamed.\nThese files are preserved with their original names.\nUseful for samples, trailers, or files you want to keep as-is.\nInclude the dot (.) prefix for each extension.\nExample: ['.sample.mkv', '.trailer.mp4'] for samples and trailers" multiline:"true" toml:"allowed_video_extensions_no_rename"`
	// AllowedVideoExtensionsNoRenameLen is the number of video extensions not to rename
	AllowedVideoExtensionsNoRenameLen int `toml:"-"`
	// AllowedOtherExtensionsNoRename lists other extensions not to rename
	AllowedOtherExtensionsNoRename []string `comment:"List of non-video file extensions that will be copied but NOT renamed.\nThese files preserve their" displayname:"Other Extensions No Rename" longcomment:"List of non-video file extensions that will be copied but NOT renamed.\nThese files preserve their original names and are not matched to media.\nUseful for readme files, original NFOs, or reference materials.\nInclude the dot (.) prefix for each extension.\nExample: ['.txt', '.readme', '.original.nfo']" multiline:"true" toml:"allowed_other_extensions_no_rename"`
	// AllowedOtherExtensionsNoRenameLen is the number of other extensions not to rename
	AllowedOtherExtensionsNoRenameLen int `toml:"-"`
	// AllowedAudioExtensions lists the allowed audio file extensions for music/audiobooks
	AllowedAudioExtensions []string `comment:"List of audio file extensions for music and audiobooks.\nThese files are considered main media files" displayname:"Audio File Extensions" longcomment:"List of audio file extensions for music and audiobooks.\nThese files are considered main media files.\nInclude the dot (.) prefix for each extension.\nCommon audio formats: .mp3, .flac, .m4a, .m4b, .ogg, .opus, .wav, .aac\nExample: ['.mp3', '.flac', '.m4a', '.m4b']" multiline:"true" toml:"allowed_audio_extensions"`
	// AllowedAudioExtensionsLen is the number of allowed audio extensions
	AllowedAudioExtensionsLen int `toml:"-"`
	// AllowedAudioExtensionsNoRename lists audio extensions that should not be renamed
	AllowedAudioExtensionsNoRename []string `comment:"List of audio file extensions that will be copied but NOT renamed.\nThese files are preserved" displayname:"Audio Extensions No Rename" longcomment:"List of audio file extensions that will be copied but NOT renamed.\nThese files are preserved with their original names.\nUseful for samples or files you want to keep as-is.\nInclude the dot (.) prefix for each extension.\nExample: ['.sample.mp3']" multiline:"true" toml:"allowed_audio_extensions_no_rename"`
	// AllowedAudioExtensionsNoRenameLen is the number of audio extensions not to rename
	AllowedAudioExtensionsNoRenameLen int `toml:"-"`
	// AllowedBookExtensions lists the allowed ebook file extensions
	AllowedBookExtensions []string `comment:"List of ebook file extensions.\nThese files are considered main media files for books" displayname:"Book File Extensions" longcomment:"List of ebook file extensions.\nThese files are considered main media files for books.\nInclude the dot (.) prefix for each extension.\nCommon ebook formats: .epub, .pdf, .mobi, .azw, .azw3, .fb2, .djvu\nExample: ['.epub', '.pdf', '.mobi', '.azw3']" multiline:"true" toml:"allowed_book_extensions"`
	// AllowedBookExtensionsLen is the number of allowed book extensions
	AllowedBookExtensionsLen int `toml:"-"`
	// AllowedBookExtensionsNoRename lists book extensions that should not be renamed
	AllowedBookExtensionsNoRename []string `comment:"List of ebook file extensions that will be copied but NOT renamed.\nThese files are preserved" displayname:"Book Extensions No Rename" longcomment:"List of ebook file extensions that will be copied but NOT renamed.\nThese files are preserved with their original names.\nInclude the dot (.) prefix for each extension.\nExample: ['.txt', '.original.pdf']" multiline:"true" toml:"allowed_book_extensions_no_rename"`
	// AllowedBookExtensionsNoRenameLen is the number of book extensions not to rename
	AllowedBookExtensionsNoRenameLen int `toml:"-"`
	// Blocked lists strings that will block processing of files
	Blocked []string `comment:"List of strings that prevent file processing when found in filenames or paths.\nFiles containing any" displayname:"Blocked File Patterns" longcomment:"List of strings that prevent file processing when found in filenames or paths.\nFiles containing any of these strings will be completely ignored.\nUse for blocking unwanted content, file types, or release groups.\nStrings are case-insensitive and can be partial matches.\nExample: ['sample', 'trailer', 'RARBG', 'password']" multiline:"true" toml:"blocked"`
	// BlockedLen is the number of blocked strings
	BlockedLen int `toml:"-"`
	// Upgrade indicates if media should be upgraded
	Upgrade bool `comment:"Enable automatic quality upgrades for media in this path.\nWhen true, the system will search for better quality versions of existing files" displayname:"Enable Quality Upgrades" longcomment:"Enable automatic quality upgrades for media in this path.\nWhen true, the system will search for better quality versions of existing files.\nUpgrades happen based on quality profiles (resolution, codec, etc.).\nWhen false, no upgrade searches are performed for this path.\nDefault: false" toml:"upgrade"`
	// MinSize is the minimum media size in MB for searches
	MinSize int `comment:"Minimum file size in megabytes for search filtering.\nReleases smaller than this size will be rejected" displayname:"Minimum File Size MB" longcomment:"Minimum file size in megabytes for search filtering.\nReleases smaller than this size will be rejected during searches.\nHelps filter out low-quality releases, samples, and fake files.\nSet to 0 to disable minimum size filtering.\nTypical values: 100MB for TV episodes, 1000MB for movies\nExample: 500 for 500MB minimum" toml:"min_size"`
	// MaxSize is the maximum media size in MB for searches
	MaxSize int `comment:"Maximum file size in megabytes for search filtering.\nReleases larger than this size will be rejected" displayname:"Maximum File Size MB" longcomment:"Maximum file size in megabytes for search filtering.\nReleases larger than this size will be rejected during searches.\nHelps avoid extremely large files that may be unwanted or problematic.\nSet to 0 to disable maximum size filtering.\nTypical values: 2000MB for TV episodes, 50000MB for 4K movies\nExample: 10000 for 10GB maximum" toml:"max_size"`
	// MinSizeByte is the minimum size in bytes
	MinSizeByte int64 `toml:"-"`
	// MaxSizeByte is the maximum size in bytes
	MaxSizeByte int64 `toml:"-"`
	// MinVideoSize is the minimum video size in MB for structure
	MinVideoSize int `comment:"Minimum video file size in megabytes for file organization.\nVideo files smaller than this size will" displayname:"Minimum Video Size MB" longcomment:"Minimum video file size in megabytes for file organization.\nVideo files smaller than this size will not be organized/renamed.\nHelps exclude samples, trailers, and low-quality files from organization.\nSet to 0 to organize all video files regardless of size.\nTypical values: 50MB for TV episodes, 200MB for movies\nExample: 100 for 100MB minimum for organization" toml:"min_video_size"`
	// MinVideoSizeByte is the minimum video size in bytes
	MinVideoSizeByte int64 `toml:"-"`
	// CleanupsizeMB is the minimum size in MB to keep a folder, 0 removes all
	CleanupsizeMB int `comment:"Minimum total size in megabytes to keep a folder during cleanup.\nFolders with total content smaller" displayname:"Folder Cleanup Size MB" longcomment:"Minimum total size in megabytes to keep a folder during cleanup.\nFolders with total content smaller than this size will be deleted.\nHelps remove leftover folders with only samples, subtitles, or small files.\nSet to 0 to remove all folders regardless of size (aggressive cleanup).\nTypical values: 50-200MB depending on your minimum file requirements\nExample: 100 to keep folders with at least 100MB of content" toml:"cleanup_size_mb"`
	// AllowedLanguages lists allowed languages for audio streams in videos
	AllowedLanguages []string `comment:"List of allowed audio languages for video files.\nFiles without audio tracks in these languages may" displayname:"Allowed Audio Languages" longcomment:"List of allowed audio languages for video files.\nFiles without audio tracks in these languages may be rejected or flagged.\nUse ISO 639-1 two-letter language codes (en, de, fr, es, etc.).\nSometimes the audio streams can also have 3 char names or full names - so enter all possibilities.\nLeave empty to allow all languages without filtering.\nExample: ['en', 'de'] for English and German audio only" multiline:"true" toml:"allowed_languages"`
	// AllowedLanguagesLen is the number of allowed languages
	AllowedLanguagesLen int `toml:"-"`
	// Replacelower indicates if lower quality video files should be replaced, default false
	Replacelower bool `comment:"Automatically replace existing files with higher quality versions.\nWhen true, better quality releases will replace lower" displayname:"Replace Lower Quality" longcomment:"Automatically replace existing files with higher quality versions.\nWhen true, better quality releases will replace lower quality existing files.\nReplacement is based on quality profiles (resolution, codec, bitrate, etc.).\nWhen false, duplicate files are kept or rejected based on other settings.\nDefault: false" toml:"replace_lower"`
	// Usepresort indicates if a presort folder should be used before media is moved, default false
	Usepresort bool `comment:"Use a temporary presort directory before final organization.\nWhen true, files are placed in presort_folder_path for manual review" displayname:"Enable Presort Directory" longcomment:"Use a temporary presort directory before final organization.\nWhen true, files are placed in presort_folder_path for manual review.\nAllows manual verification before files are moved to final locations.\nWhen false, files are moved directly to their final organized locations.\nUseful for quality control or manual intervention workflows.\nDefault: false" toml:"use_presort"`
	// PresortFolderPath is the path to the presort folder
	PresortFolderPath string `comment:"Absolute path to the presort directory (when use_presort is enabled).\nFiles are temporarily placed here before" displayname:"Presort Directory Path" longcomment:"Absolute path to the presort directory (when use_presort is enabled).\nFiles are temporarily placed here before manual organization.\nMust be an absolute path accessible to the application.\nShould be on the same filesystem as final media path for efficient moves.\nRequired when use_presort is true, ignored otherwise.\nExample: '/tmp/presort', '/downloads/presort'" toml:"presort_folder_path"`
	// UpgradeScanInterval is the number of days to wait after last search before looking for upgrades, 0 means don't wait
	UpgradeScanInterval int `comment:"Minimum days to wait between upgrade searches for the same media.\nPrevents excessive searching by spacing" displayname:"Upgrade Search Wait Days" longcomment:"Minimum days to wait between upgrade searches for the same media.\nPrevents excessive searching by spacing out upgrade attempts.\nSet to 0 to disable waiting (search for upgrades every scan cycle).\nHigher values reduce indexer load but may delay finding upgrades.\nTypical values: 7-30 days depending on how frequently you want upgrades.\nExample: 14 for bi-weekly upgrade searches" toml:"upgrade_scan_interval"`
	// MissingScanInterval is the number of days to wait after last search before looking for missing media, 0 means don't wait
	MissingScanInterval int `comment:"Minimum days to wait between missing media searches for the same item.\nPrevents excessive searching by" displayname:"Missing Search Wait Days" longcomment:"Minimum days to wait between missing media searches for the same item.\nPrevents excessive searching by spacing out search attempts.\nSet to 0 to disable waiting (search for missing media every scan cycle).\nHigher values reduce indexer load but may delay finding new releases.\nTypical values: 1-7 days depending on how actively you want to search.\nExample: 3 for searches every 3 days" toml:"missing_scan_interval"`
	// MissingScanReleaseDatePre is the minimum number of days to wait after media release before scanning, 0 means don't check
	MissingScanReleaseDatePre int `comment:"Days to wait before the official release date before starting searches.\nAllows searching for media before" displayname:"Pre Release Search Days" longcomment:"Days to wait before the official release date before starting searches.\nAllows searching for media before its official release date (for pre-releases).\nPositive values search X days before release, negative values wait X days after.\nSet to 0 to disable release date checking (search immediately when added).\nExample: -7 to wait 7 days after release, 3 to search 3 days before release" toml:"missing_scan_release_date_pre"`
	// Disallowed lists strings that will block processing if found
	Disallowed []string `comment:"List of strings that prevent file organization when found in release names.\nFiles are downloaded but not organized if they contain these strings" displayname:"Disallowed File Patterns" longcomment:"List of strings that prevent file organization when found in release names.\nFiles are downloaded but not organized/renamed if they contain these strings.\nUseful for blocking specific release groups, qualities, or naming patterns.\nStrings are case-insensitive and can be partial matches.\nExample: ['CAM', 'TS', 'HDCAM', 'BadGroup'] to block low-quality releases" multiline:"true" toml:"disallowed"`
	// DisallowedLen is the number of disallowed strings
	DisallowedLen int `toml:"-"`
	// DeleteWrongLanguage indicates if media with wrong language should be deleted, default false
	DeleteWrongLanguage bool `comment:"Automatically delete files with audio languages not in allowed_languages.\nWhen true, files without allowed audio languages" displayname:"Delete Wrong Language Files" longcomment:"Automatically delete files with audio languages not in allowed_languages.\nWhen true, files without allowed audio languages are deleted after download.\nWhen false, files are kept but may not be organized properly.\nOnly works when allowed_languages is configured.\nCaution: This permanently deletes files - use with care.\nDefault: false" toml:"delete_wrong_language"`
	// DeleteDisallowed indicates if media with disallowed strings should be deleted, default false
	DeleteDisallowed bool `comment:"Automatically delete files containing disallowed strings.\nWhen true, files matching disallowed patterns are deleted after download.\nWhen" displayname:"Delete Disallowed Files" longcomment:"Automatically delete files containing disallowed strings.\nWhen true, files matching disallowed patterns are deleted after download.\nWhen false, files are kept but not organized (safer option).\nOnly affects files matching strings in the disallowed list.\nCaution: This permanently deletes files - use with care.\nDefault: false" toml:"delete_disallowed"`
	// CheckRuntime indicates if runtime should be checked before import, default false
	CheckRuntime bool `comment:"Verify video runtime against expected duration before organization.\nWhen true, video files are checked against database" displayname:"Enable Runtime Verification" longcomment:"Verify video runtime against expected duration before organization.\nWhen true, video files are checked against database runtime information.\nHelps detect incomplete, fake, or incorrectly matched files.\nWhen false, runtime verification is skipped (faster processing).\nRequires metadata sources with runtime information to be effective.\nDefault: false" toml:"check_runtime"`
	// MaxRuntimeDifference is the max minutes of difference allowed in runtime checks, 0 means no check
	MaxRuntimeDifference int `comment:"Maximum allowed runtime difference in minutes for runtime verification.\nFiles with runtime differing more than this" displayname:"Max Runtime Difference Minutes" longcomment:"Maximum allowed runtime difference in minutes for runtime verification.\nFiles with runtime differing more than this amount are flagged or rejected.\nAccounts for encoding differences, credits, and metadata inaccuracies.\nSet to 0 to disable runtime checking entirely.\nTypical values: 5-15 minutes depending on content type and tolerance.\nExample: 10 to allow up to 10 minutes difference" toml:"max_runtime_difference"`
	// DeleteWrongRuntime indicates if media with wrong runtime should be deleted, default false
	DeleteWrongRuntime bool `comment:"Automatically delete files that fail runtime verification.\nWhen true, files with runtime outside max_runtime_difference are deleted.\nWhen" displayname:"Delete Wrong Runtime Files" longcomment:"Automatically delete files that fail runtime verification.\nWhen true, files with runtime outside max_runtime_difference are deleted.\nWhen false, files are kept but may not be organized (safer option).\nOnly works when check_runtime is enabled and max_runtime_difference is set.\nCaution: This permanently deletes files - use with care.\nDefault: false" toml:"delete_wrong_runtime"`
	// MoveReplaced indicates if replaced media should be moved to old folder, default false
	MoveReplaced bool `comment:"Move replaced files to a backup directory instead of deleting them.\nWhen true, old files are moved to move_replaced_target_path when upgraded" displayname:"Move Replaced Files" longcomment:"Move replaced files to a backup directory instead of deleting them.\nWhen true, old files are moved to move_replaced_target_path when upgraded.\nWhen false, old files are deleted during replacement (saves space).\nProvides a safety net for undoing upgrades if needed.\nRequires move_replaced_target_path to be configured.\nDefault: false" toml:"move_replaced"`
	// MoveReplacedTargetPath is the path to the folder for replaced media
	MoveReplacedTargetPath string `comment:"Absolute path where replaced/upgraded files are moved for backup.\nUsed when move_replaced is enabled to store" displayname:"Replaced Files Directory" longcomment:"Absolute path where replaced/upgraded files are moved for backup.\nUsed when move_replaced is enabled to store old versions of files.\nMust be an absolute path accessible to the application.\nConsider storage space as this will accumulate replaced files over time.\nRequired when move_replaced is true, ignored otherwise.\nExample: '/backup/replaced-media', '/storage/old-files'" toml:"move_replaced_target_path"`
	// SetChmod is the chmod for files in octal format, default 0777
	SetChmod       string `comment:"File permissions to set on organized media files (Unix/Linux only).\nUse octal format (3-4 digits) to"        displayname:"File Permissions Octal"   longcomment:"File permissions to set on organized media files (Unix/Linux only).\nUse octal format (3-4 digits) to specify read/write/execute permissions.\nApplied to all organized files to ensure consistent access permissions.\nIgnored on Windows systems - only affects Unix-like systems.\nCommon values: '0644' (rw-r--r--), '0664' (rw-rw-r--), '0777' (rwxrwxrwx)\nDefault: '0777'"                                                                               toml:"set_chmod"`
	SetChmodFolder string `comment:"Directory permissions to set on organized media folders (Unix/Linux only).\nUse octal format (3-4 digits) to" displayname:"Folder Permissions Octal" longcomment:"Directory permissions to set on organized media folders (Unix/Linux only).\nUse octal format (3-4 digits) to specify read/write/execute permissions.\nApplied to all created directories to ensure consistent access permissions.\nIgnored on Windows systems - only affects Unix-like systems.\nFolders typically need execute permission for access (x bit set).\nCommon values: '0755' (rwxr-xr-x), '0775' (rwxrwxr-x), '0777' (rwxrwxrwx)\nDefault: '0777'" toml:"set_chmod_folder"`
}

// NotificationConfig defines the configuration for notifications.
type NotificationConfig struct {
	// Name is the name of the notification template
	Name string `comment:"Unique name for this notification configuration.\nUsed to identify this notification method in media configurations.\nChoose a" displayname:"Notification Configuration Name" longcomment:"Unique name for this notification configuration.\nUsed to identify this notification method in media configurations.\nChoose a descriptive name that indicates the notification type and purpose.\nExample: 'pushover-main', 'csv-log', 'gotify-alerts', 'pushbullet-mobile'" toml:"name"`
	// NotificationType is the type of notification - use csv, pushover, gotify, pushbullet, or apprise
	NotificationType string `comment:"Type of notification service to use.\nAvailable options:\n- 'csv': Write notifications to a CSV file" displayname:"Notification Service Type" longcomment:"Type of notification service to use.\nAvailable options:\n- 'csv': Write notifications to a CSV file for logging/tracking\n- 'pushover': Send push notifications via Pushover service\n- 'gotify': Send notifications to self-hosted Gotify server\n- 'pushbullet': Send push notifications via Pushbullet service\n- 'apprise': Send notifications via Apprise API server (supports 80+ services)\nExample: 'pushover' for mobile notifications, 'gotify' for self-hosted" toml:"type"`
	// Apikey is the API key/token for the service
	Apikey string `comment:"API key or token for the notification service.\nRequired for pushover, pushbullet, and gotify.\nLeave empty for" displayname:"API Key/Token" longcomment:"API key or token for the notification service.\nRequired for pushover, pushbullet, and gotify. Leave empty for CSV and Apprise.\nPushover: Get from https://pushover.net/apps/build\nPushbullet: Get from https://www.pushbullet.com/#settings/account\nGotify: Application token from your Gotify server\nExample: 'azGDORePK8gMaC0QOYAMyEEuzJnyUi'" toml:"apikey"`
	// Recipient is the recipient for pushover notifications
	Recipient string `comment:"Pushover user key or group key to receive notifications.\nOnly used when type is 'pushover'" displayname:"Pushover User Key" longcomment:"Pushover user key or group key to receive notifications.\nOnly used when type is 'pushover'. Find your user key in your Pushover account dashboard.\nCan be a user key (for individual) or group key (for groups).\nIgnored for other notification types.\nExample: 'uQiRzpo4DXghDmr9QzzfQu27cmVRsG'" toml:"recipient"`
	// Outputto is the path to output csv notifications
	Outputto string `comment:"File path for CSV notification output (required when type is 'csv').\nIgnored for other notification types" displayname:"CSV Output File Path" longcomment:"File path for CSV notification output (required when type is 'csv').\nNotifications will be appended to this CSV file with timestamps.\nPath can be absolute or relative to the application directory.\nFile will be created if it doesn't exist, appended to if it does.\nIgnored for push notification services.\nExample: './logs/notifications.csv' or '/var/log/media-notifications.csv'" toml:"output_to"`
	// ServerURL is the server URL for self-hosted services
	ServerURL string `comment:"Server URL for self-hosted notification services.\nRequired for gotify and apprise.\nLeave empty for" displayname:"Server URL" longcomment:"Server URL for self-hosted notification services.\nRequired for gotify and apprise. Leave empty for pushover, pushbullet, and CSV.\nGotify: URL to your Gotify server (e.g., 'https://gotify.example.com')\nApprise: URL to your Apprise API server (e.g., 'http://localhost:8000')\nExample: 'https://gotify.mydomain.com'" toml:"server_url"`
	// AppriseURLs contains the notification service URLs for Apprise
	AppriseURLs string `comment:"Comma-separated list of notification service URLs for Apprise.\nOnly used when type is 'apprise'" displayname:"Apprise Service URLs" longcomment:"Comma-separated list of notification service URLs for Apprise.\nOnly used when type is 'apprise'. Each URL represents a different notification service.\nApprise supports 80+ notification services including Discord, Slack, Telegram, etc.\nSee Apprise documentation for URL format for each service.\nExample: 'discord://webhook_id/webhook_token,slack://TokenA/TokenB/TokenC/Channel'" toml:"apprise_urls"`
}

// RegexConfig is a struct that defines a regex template
// It contains fields for the template name, required regexes,
// rejected regexes, and lengths of the regex slices.
type RegexConfig struct {
	// Name is the name of the regex template
	Name string `comment:"Unique name for this regex filter configuration.\nUsed to identify this regex set in quality profiles" displayname:"Regex Filter Name" longcomment:"Unique name for this regex filter configuration.\nUsed to identify this regex set in quality profiles and logs.\nChoose a descriptive name that indicates the filtering purpose.\nExample: 'no-cams', 'preferred-groups', 'block-samples'" toml:"name"`
	// Required is a slice of regex strings that are required (one must match)
	Required []string `comment:"List of regular expressions where at least ONE must match for acceptance.\nReleases must match at" displayname:"Required Pattern Matches" longcomment:"List of regular expressions where at least ONE must match for acceptance.\nReleases must match at least one of these patterns to be considered.\nUse standard regex syntax - patterns are case-insensitive by default.\nUseful for requiring specific release groups, qualities, or naming patterns.\nLeave empty to disable required pattern filtering.\nExample: ['SPARKS', 'FGT', 'DIMENSION'] to require specific groups" multiline:"true" toml:"required"`
	// Rejected is a slice of regex strings that cause rejection if matched
	Rejected []string `comment:"List of regular expressions that cause immediate rejection if ANY match.\nReleases matching any of these" displayname:"Rejected Pattern Matches" longcomment:"List of regular expressions that cause immediate rejection if ANY match.\nReleases matching any of these patterns will be rejected/blocked.\nUse standard regex syntax - patterns are case-insensitive by default.\nUseful for blocking unwanted release groups, qualities, or content types.\nProcessed after required patterns - rejection overrides acceptance.\nExample: ['CAM', 'TS', 'HDCAM', '.*YIFY.*'] to block low-quality releases" multiline:"true" toml:"rejected"`
	// RequiredLen is the length of the Required slice
	RequiredLen int `toml:"-"`
	// RejectedLen is the length of the Rejected slice
	RejectedLen int `toml:"-"`
}

type QualityConfig struct {
	// Name is the name of the template
	Name string `comment:"Unique name for this quality profile configuration.\nUsed to identify this quality profile in media configurations.\nChoose" displayname:"Quality Profile Name" longcomment:"Unique name for this quality profile configuration.\nUsed to identify this quality profile in media configurations.\nChoose a descriptive name that indicates the quality standards.\nExample: 'uhd-4k', 'hd-1080p', 'standard-720p', 'anime-preferred'" toml:"name"`
	// WantedResolution is resolutions which are wanted - others are skipped - empty = allow all
	WantedResolution []string `comment:"List of acceptable video resolutions for this quality profile.\nReleases not matching these resolutions will be" displayname:"Accepted Video Resolutions" longcomment:"List of acceptable video resolutions for this quality profile.\nReleases not matching these resolutions will be rejected.\nLeave empty to accept all resolutions without filtering.\nCommon values: '2160p', '1080p', '720p', '576p', '480p'\nExample: ['2160p', '1080p'] for UHD and Full HD only" multiline:"true" toml:"wanted_resolution"`
	// WantedQuality is qualities which are wanted - others are skipped - empty = allow all
	WantedQuality []string `comment:"List of acceptable video quality levels for this profile.\nReleases not matching these quality standards will" displayname:"Accepted Source Quality Types" longcomment:"List of acceptable video quality levels for this profile.\nReleases not matching these quality standards will be rejected.\nLeave empty to accept all quality levels without filtering.\nCommon values: 'BluRay', 'WEB-DL', 'WEBRip', 'HDTV', 'DVD'\nExample: ['BluRay', 'WEB-DL'] for highest quality sources only" multiline:"true" toml:"wanted_quality"`
	// WantedAudio is audio codecs which are wanted - others are skipped - empty = allow all
	WantedAudio []string `comment:"List of acceptable audio codecs and formats for this profile.\nReleases not matching these audio specifications" displayname:"Accepted Audio Codecs" longcomment:"List of acceptable audio codecs and formats for this profile.\nReleases not matching these audio specifications will be rejected.\nLeave empty to accept all audio formats without filtering.\nCommon values: 'DTS', 'AC3', 'AAC', 'FLAC', 'TrueHD', 'Atmos'\nExample: ['DTS', 'TrueHD', 'Atmos'] for high-quality audio only" multiline:"true" toml:"wanted_audio"`
	// WantedCodec is video codecs which are wanted - others are skipped - empty = allow all
	WantedCodec []string `comment:"List of acceptable video codecs for this profile.\nReleases not matching these video encoding standards will" displayname:"Accepted Video Codecs" longcomment:"List of acceptable video codecs for this profile.\nReleases not matching these video encoding standards will be rejected.\nLeave empty to accept all video codecs without filtering.\nCommon values: 'x264', 'x265', 'H.264', 'H.265', 'HEVC', 'AV1'\nExample: ['x265', 'HEVC'] for modern efficient encoding only" multiline:"true" toml:"wanted_codec"`
	// WantedResolutionLen is the length of the WantedResolution slice
	WantedResolutionLen int `toml:"-"`
	// WantedQualityLen is the length of the WantedQuality slice
	WantedQualityLen int `toml:"-"`
	// WantedAudioLen is the length of the WantedAudio slice
	WantedAudioLen int `toml:"-"`
	// WantedCodecLen is the length of the WantedCodec slice
	WantedCodecLen int `toml:"-"`
	// CutoffResolution is after which resolution should we stop searching for upgrades
	CutoffResolution string `comment:"Resolution at which upgrade searches stop (satisfaction point).\nOnce media reaches this resolution, no further upgrades" displayname:"Upgrade Stop Resolution" longcomment:"Resolution at which upgrade searches stop (satisfaction point).\nOnce media reaches this resolution, no further upgrades are sought.\nMust be one of the resolutions listed in wanted_resolution.\nSet to the highest quality you want to prevent excessive upgrading.\nExample: '2160p' to stop upgrading once 4K is achieved" toml:"cutoff_resolution"`
	// CutoffQuality is after which quality should we stop searching for upgrades
	CutoffQuality string `comment:"Quality level at which upgrade searches stop (satisfaction point).\nOnce media reaches this quality, no further" displayname:"Upgrade Stop Quality" longcomment:"Quality level at which upgrade searches stop (satisfaction point).\nOnce media reaches this quality, no further upgrades are sought.\nMust be one of the qualities listed in wanted_quality.\nSet to the highest quality you want to prevent excessive upgrading.\nExample: 'BluRay' to stop upgrading once Blu-ray quality is achieved" toml:"cutoff_quality"`
	// CutoffPriority is the priority cutoff
	CutoffPriority int `toml:"-"`

	// SearchForTitleIfEmpty is a bool indicating if we should do a title search if the id search didn't return an accepted release
	// - backup_search_for_title needs to be true? - default: false
	SearchForTitleIfEmpty bool `comment:"Enable title-based searching when ID-based search yields no results.\nWhen true, if IMDB/TVDB ID searches fail," displayname:"Fallback Title Search" longcomment:"Enable title-based searching when ID-based search yields no results.\nWhen true, if IMDB/TVDB ID searches fail, fallback to title searches.\nRequires backup_search_for_title to be enabled to function.\nUseful when indexers have limited ID coverage but good title matching.\nIncreases search coverage but may reduce accuracy.\nDefault: false" toml:"search_for_title_if_empty"`

	// BackupSearchForTitle is a bool indicating if we want to search for titles and not only id's - default: false
	BackupSearchForTitle bool `comment:"Enable title-based searches as a backup to ID-based searches.\nWhen true, searches can use media titles" displayname:"Enable Title Search Backup" longcomment:"Enable title-based searches as a backup to ID-based searches.\nWhen true, searches can use media titles when ID searches are insufficient.\nProvides broader search coverage when indexers lack proper ID tagging.\nRequired for search_for_title_if_empty functionality.\nMay increase false positives due to title ambiguity.\nDefault: false" toml:"backup_search_for_title"`

	// SearchForAlternateTitleIfEmpty is a bool indicating if we should do a alternate title search if the id search didn't return an accepted release
	// - backup_search_for_alternate_title needs to be true? - default: false
	SearchForAlternateTitleIfEmpty bool `comment:"Enable alternate title searching when ID-based search yields no results.\nWhen true, if IMDB/TVDB ID searches" displayname:"Fallback Alternate Title Search" longcomment:"Enable alternate title searching when ID-based search yields no results.\nWhen true, if IMDB/TVDB ID searches fail, search using alternate/foreign titles.\nRequires backup_search_for_alternate_title to be enabled to function.\nUseful for finding releases with regional or translated titles.\nFurther increases search coverage but may reduce accuracy.\nDefault: false" toml:"search_for_alternate_title_if_empty"`

	// BackupSearchForAlternateTitle is a bool indicating if we want to search for alternate titles and not only id's - default: false
	BackupSearchForAlternateTitle bool `comment:"Enable alternate title searches as a backup to ID-based searches.\nWhen true, searches can use foreign" displayname:"Enable Alternate Title Backup" longcomment:"Enable alternate title searches as a backup to ID-based searches.\nWhen true, searches can use foreign language titles and aliases.\nHelps find releases using regional names, translations, or alternate titles.\nRequired for search_for_alternate_title_if_empty functionality.\nIncreases search coverage but may have accuracy trade-offs.\nDefault: false" toml:"backup_search_for_alternate_title"`

	// ExcludeYearFromTitleSearch is a bool indicating if the year should not be included in the title search? - default: false
	ExcludeYearFromTitleSearch bool `comment:"Exclude release year from title-based searches.\nWhen true, searches use only the title without the year.\nUseful" displayname:"Exclude Year From Title Search" longcomment:"Exclude release year from title-based searches.\nWhen true, searches use only the title without the year.\nUseful when indexers have inconsistent or missing year information.\nMay increase matches but also increases chance of wrong matches.\nOnly affects title searches, not ID-based searches.\nDefault: false" toml:"exclude_year_from_title_search"`

	// CheckUntilFirstFound is a bool indicating if we should stop searching if we found a release? - default: false
	CheckUntilFirstFound bool `comment:"Stop searching across indexers after finding the first acceptable release.\nWhen true, search stops at first" displayname:"Stop At First Match" longcomment:"Stop searching across indexers after finding the first acceptable release.\nWhen true, search stops at first match that passes quality filters.\nWhen false, all indexers are searched to find the best available release.\nEnabling speeds up searches but may miss better quality releases.\nDisabling finds best quality but increases search time and API usage.\nDefault: false" toml:"check_until_first_found"`

	// CheckTitle is a bool indicating if the title of the release should be checked? - default: false
	CheckTitle bool `comment:"Verify that release titles match the expected media title.\nWhen true, release titles are compared against" displayname:"Verify Release Title" longcomment:"Verify that release titles match the expected media title.\nWhen true, release titles are compared against media titles for accuracy.\nHelps prevent downloading incorrectly named or mismatched releases.\nWhen false, title verification is skipped (faster but less accurate).\nRecommended for reducing false positives and wrong downloads.\nDefault: false" toml:"check_title"`

	// CheckTitleOnIDSearch is a bool indicating if the title of the release should be checked during an id based search? - default: false
	CheckTitleOnIDSearch bool `comment:"Verify release titles even when searching by IMDB/TVDB ID.\nWhen true, both ID and title must" displayname:"Verify Title On ID Search" longcomment:"Verify release titles even when searching by IMDB/TVDB ID.\nWhen true, both ID and title must match for acceptance.\nWhen false, ID matching alone is sufficient (faster).\nUseful when indexers have unreliable ID tagging.\nProvides extra verification at the cost of some performance.\nDefault: false" toml:"check_title_on_id_search"`

	// CheckYear is a bool indicating if the year of the release should be checked? - default: false
	CheckYear bool `comment:"Verify that release years match the expected media release year.\nWhen true, release years must exactly" displayname:"Verify Release Year" longcomment:"Verify that release years match the expected media release year.\nWhen true, release years must exactly match the media's release year.\nHelps prevent downloading releases from wrong years (remakes, etc.).\nWhen false, year verification is skipped.\nUseful for ensuring correct version matching.\nDefault: false" toml:"check_year"`

	// CheckYear1 is a bool indicating if the year of the release should be checked and is +-1 year allowed? - default: false
	CheckYear1 bool `comment:"Verify release years with ±1 year tolerance from expected year.\nWhen true, releases within 1 year" displayname:"Verify Year Plus Minus One" longcomment:"Verify release years with ±1 year tolerance from expected year.\nWhen true, releases within 1 year of the expected year are accepted.\nMore flexible than check_year for handling release date variations.\nAccounts for different regional release dates or metadata discrepancies.\nWhen false, no year tolerance is applied.\nDefault: false" toml:"check_year1"`

	// TitleStripSuffixForSearch is a []string indicating what suffixes should be removed from the title
	TitleStripSuffixForSearch []string `comment:"List of suffixes to remove from titles before searching.\nHelps normalize titles by removing common suffixes" displayname:"Strip Title Suffixes" longcomment:"List of suffixes to remove from titles before searching.\nHelps normalize titles by removing common suffixes that vary between sources.\nProcessed before sending search queries to indexers.\nCommon suffixes include year ranges, edition markers, or format indicators.\nLeave empty to search with original titles." multiline:"true" toml:"title_strip_suffix_for_search"`

	// TitleStripPrefixForSearch is a []string indicating what prefixes should be removed from the title
	TitleStripPrefixForSearch []string `comment:"List of prefixes to remove from titles before searching.\nHelps normalize titles by removing common prefixes" displayname:"Strip Title Prefixes" longcomment:"List of prefixes to remove from titles before searching.\nHelps normalize titles by removing common prefixes that vary between sources.\nProcessed before sending search queries to indexers.\nCommon prefixes include articles, franchise markers, or format indicators.\nLeave empty to search with original titles." multiline:"true" toml:"title_strip_prefix_for_search"`

	// QualityReorder is a []QualityReorderConfig for configs if a quality reordering is needed - for example if 720p releases should be preferred over 1080p
	QualityReorder []QualityReorderConfig `comment:"Custom priority reordering rules for specific quality characteristics.\nAllows overriding default priority calculations for special cases.\nUseful" displayname:"Quality Priority Reorder Rules" longcomment:"Custom priority reordering rules for specific quality characteristics.\nAllows overriding default priority calculations for special cases.\nUseful when you prefer certain resolutions, codecs, or groups over others.\nEach rule specifies what to match and what new priority to assign.\nExample: Prefer 720p over 1080p for bandwidth-limited situations.\nLeave empty to use default priority calculations." toml:"reorder"`

	// Indexer is a []QualityIndexerConfig for configs of the indexers to be used for this quality
	Indexer    []QualityIndexerConfig `comment:"List of indexer configurations specific to this quality profile.\nDefines which indexers to use and their" displayname:"Indexer Configurations" longcomment:"List of indexer configurations specific to this quality profile.\nDefines which indexers to use and their specific settings for this profile.\nEach entry maps to an indexer template and can override default settings.\nAllows different search strategies per quality profile.\nRequired - must specify at least one indexer for searches to work." toml:"indexers"`
	IndexerCfg []*IndexersConfig      `                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             toml:"-"`

	// TitleStripSuffixForSearchLen is a int for the length of the TitleStripSuffixForSearch slice
	TitleStripSuffixForSearchLen int `toml:"-"`

	// TitleStripPrefixForSearchLen is the length of the TitleStripPrefixForSearch slice
	TitleStripPrefixForSearchLen int `toml:"-"`
	// QualityReorderLen is the length of the QualityReorder slice
	QualityReorderLen int `toml:"-"`
	// IndexerLen is the length of the Indexer slice
	IndexerLen int `toml:"-"`
	// UseForPriorityResolution indicates if resolution should be used for priority
	UseForPriorityResolution bool `comment:"Include video resolution in priority calculations for release ranking.\nWhen true, higher resolutions get higher priority" displayname:"Use Resolution For Priority" longcomment:"Include video resolution in priority calculations for release ranking.\nWhen true, higher resolutions get higher priority scores.\nHelps automatically prefer 4K over 1080p, 1080p over 720p, etc.\nRecommended for most users who want the highest available resolution.\nWhen false, resolution doesn't affect priority ranking.\nDefault: false, Recommended: true" toml:"use_for_priority_resolution"`
	// UseForPriorityQuality indicates if quality should be used for priority
	UseForPriorityQuality bool `comment:"Include source quality in priority calculations for release ranking.\nWhen true, higher quality sources get higher" displayname:"Use Quality For Priority" longcomment:"Include source quality in priority calculations for release ranking.\nWhen true, higher quality sources get higher priority scores.\nHelps automatically prefer BluRay over WEB-DL, WEB-DL over HDTV, etc.\nRecommended for most users who want the highest available quality.\nWhen false, source quality doesn't affect priority ranking.\nDefault: false, Recommended: true" toml:"use_for_priority_quality"`
	// UseForPriorityAudio indicates if audio codecs should be used for priority
	UseForPriorityAudio bool `comment:"Include audio codec quality in priority calculations for release ranking.\nWhen true, preferred audio codecs (DTS," displayname:"Use Audio For Priority" longcomment:"Include audio codec quality in priority calculations for release ranking.\nWhen true, preferred audio codecs (DTS, TrueHD, Atmos) get higher priority.\nHelps distinguish between releases with different audio quality levels.\nMay not be necessary if you don't have strong audio preferences.\nWhen false, audio codec doesn't affect priority ranking.\nDefault: false, Recommended: false (unless audio quality is critical)" toml:"use_for_priority_audio"`
	// UseForPriorityCodec indicates if video codecs should be used for priority
	UseForPriorityCodec bool `comment:"Include video codec efficiency in priority calculations for release ranking.\nWhen true, modern codecs (x265, HEVC," displayname:"Use Codec For Priority" longcomment:"Include video codec efficiency in priority calculations for release ranking.\nWhen true, modern codecs (x265, HEVC, AV1) may get priority adjustments.\nHelps distinguish between older and newer encoding technologies.\nMay not be necessary unless you have specific codec preferences.\nWhen false, video codec doesn't affect priority ranking.\nDefault: false, Recommended: false (unless codec efficiency is important)" toml:"use_for_priority_codec"`
	// UseForPriorityOther indicates if other data should be used for priority
	UseForPriorityOther bool `comment:"Include release metadata in priority calculations for ranking.\nWhen true, factors like REPACK, PROPER, EXTENDED, UNCUT" displayname:"Use Metadata For Priority" longcomment:"Include release metadata in priority calculations for ranking.\nWhen true, factors like REPACK, PROPER, EXTENDED, UNCUT affect priority.\nHelps prefer corrected releases and special editions over originals.\nUseful for getting the best available version of releases.\nWhen false, these metadata factors don't affect priority ranking.\nDefault: false, Recommended: false (unless you want enhanced versions)" toml:"use_for_priority_other"`
	// UseForPriorityMinDifference is the min difference to use a release for upgrade
	UseForPriorityMinDifference int `comment:"Minimum priority score difference required to trigger an upgrade.\nOnly releases with priority scores this much" displayname:"Minimum Priority Upgrade Difference" longcomment:"Minimum priority score difference required to trigger an upgrade.\nOnly releases with priority scores this much higher will replace existing files.\nHigher values make upgrades more selective, lower values more aggressive.\nSet to 0 to upgrade for any improvement, however small.\nHelps prevent excessive upgrading for marginal improvements.\nTypical values: 0-50, where 0 = any improvement, 20 = significant improvement only\nDefault: 0 (upgrade for any improvement)" toml:"use_for_priority_min_difference"`

	// UseForPriorityAudioFormat indicates if audio format should be used for priority (music/audiobooks)
	UseForPriorityAudioFormat bool `comment:"Include audio format in priority calculations for music/audiobooks.\nWhen true, lossless formats (FLAC) get higher priority than lossy (MP3)." displayname:"Use Audio Format For Priority" longcomment:"Include audio format in priority calculations for music/audiobooks.\nWhen true, lossless formats (FLAC, ALAC, WAV) get higher priority than lossy (MP3, AAC, OGG).\nHelps automatically prefer lossless audio over compressed formats.\nRecommended for audiophiles who want the highest quality audio.\nDefault: false, Recommended: true for music" toml:"use_for_priority_audio_format"`
	// UseForPriorityAudioBitrate indicates if audio bitrate should be used for priority (music/audiobooks)
	UseForPriorityAudioBitrate bool `comment:"Include audio bitrate in priority calculations for music/audiobooks.\nWhen true, higher bitrates get higher priority." displayname:"Use Audio Bitrate For Priority" longcomment:"Include audio bitrate in priority calculations for music/audiobooks.\nWhen true, higher bitrates (320kbps) get higher priority than lower (128kbps).\nHelps automatically prefer higher quality lossy audio.\nUseful when lossless is not available or file size is a concern.\nDefault: false, Recommended: true for music if not using lossless" toml:"use_for_priority_audio_bitrate"`
	// WantedAudioFormats is a list of wanted audio formats for music/audiobooks (empty = allow all)
	WantedAudioFormats []string `comment:"List of audio formats to accept for music/audiobooks.\nEmpty = allow all formats." displayname:"Wanted Audio Formats" longcomment:"List of audio formats to accept for music/audiobooks.\nFormats: flac, alac, wav, mp3, aac, ogg, opus, m4a, m4b\nEmpty list allows all formats.\nExample: ['flac', 'mp3'] - only accept FLAC or MP3" toml:"wanted_audio_formats"`
	// WantedAudioFormatsLen is the length of the WantedAudioFormats slice
	WantedAudioFormatsLen int `toml:"-"`
	// MinAudioBitrate is the minimum audio bitrate in kbps to accept (0 = no minimum)
	MinAudioBitrate int `comment:"Minimum audio bitrate in kbps to accept (0 = no minimum).\nReleases below this bitrate will be rejected." displayname:"Minimum Audio Bitrate" longcomment:"Minimum audio bitrate in kbps to accept (0 = no minimum).\nReleases below this bitrate will be rejected.\nTypical values: 128, 192, 256, 320 for lossy; 0 for lossless (varies).\nDefault: 0 (no minimum)" toml:"min_audio_bitrate"`
	// PreferLossless indicates if lossless audio formats should be preferred over lossy
	PreferLossless bool `comment:"Prefer lossless audio formats (FLAC, ALAC) over lossy (MP3, AAC).\nLossless releases will get priority bonus." displayname:"Prefer Lossless Audio" longcomment:"Prefer lossless audio formats (FLAC, ALAC, WAV) over lossy (MP3, AAC, OGG).\nLossless releases will get a significant priority bonus.\nUseful for maintaining an audiophile-quality music library.\nDefault: false, Recommended: true for music" toml:"prefer_lossless"`
}

// QualityReorderConfig is a struct for configuring reordering of qualities
// It contains a Name string field for the name of the quality
// A ReorderType string field for the type of reordering
// And a Newpriority int field for the new priority.
type QualityReorderConfig struct {
	// Name is the name of the quality to reorder
	Name string `comment:"Name or pattern of the quality characteristic to reorder.\nSpecifies which quality aspect should have its" displayname:"Quality Pattern To Reorder" longcomment:"Name or pattern of the quality characteristic to reorder.\nSpecifies which quality aspect should have its priority modified.\nExamples based on reorder type:\n- Resolution: '1080p', '2160p', '720p' (single value only)\n- Quality: 'BluRay', 'WEB-DL', 'HDTV' (single value only)\n- Codec: 'x265', 'HEVC', 'x264' (single value only)\n- Audio: 'DTS', 'AC3', 'AAC' (single value only)\n- Position: 'resolution', 'quality', 'codec', or 'audio' (specifies what to multiply by position)\n- Combined: 'resolution,quality' format (e.g. '1080p,BluRay' - exactly one comma required)\nFor all types except combined_res_qual, use single values only.\nMust match the actual values found in release names." toml:"name"`
	// ReorderType is the type of reordering to use
	ReorderType string `comment:"Type of quality characteristic to reorder for priority calculation.\nSpecifies which aspect of releases should have" displayname:"Reorder Type" longcomment:"Type of quality characteristic to reorder for priority calculation.\nSpecifies which aspect of releases should have custom priority scoring.\nSupported reorder types:\n- 'resolution': Video resolution (720p, 1080p, 2160p, etc.)\n- 'quality': Source quality (BluRay, WEB-DL, HDTV, etc.)\n- 'codec': Video codec (x264, x265, HEVC, AV1, etc.)\n- 'audio': Audio codec (DTS, AC3, AAC, FLAC, etc.)\n- 'position': Multiplies priority by position (name field: resolution, quality, codec, or audio)\n- 'combined_res_qual': Combined resolution and quality scoring\nDifferent types affect how new_priority values are applied.\nExample: 'resolution' to customize resolution priority scoring" toml:"type"`
	// Newpriority is the new priority to set for the quality
	Newpriority int `comment:"Custom priority value to assign to the specified quality characteristic.\nHow this value is applied depends" displayname:"New Priority Value" longcomment:"Custom priority value to assign to the specified quality characteristic.\nHow this value is applied depends on the reorder type:\n- 'resolution', 'quality', 'codec', 'audio': Direct priority assignment\n- 'position': Multiplied by position number for ranking\n- 'combined_res_qual': Resolution gets this value, quality set to 0\nHigher numbers = higher priority in search results.\nUseful for preferring specific characteristics:\n- Set 720p to priority 100 to prefer over 1080p (bandwidth saving)\n- Set x265 to priority 150 for codec preference\n- Set BluRay to priority 200 for quality preference\nTypical range: 0-1000, where higher values are preferred.\nExample: 150 to give moderate preference to specified items" toml:"new_priority"`
}

// QualityIndexerConfig defines the configuration for an indexer used for a specific quality.
type QualityIndexerConfig struct {
	// TemplateIndexer is the template to use for the indexer
	TemplateIndexer string `comment:"Name of the indexer configuration template to use for this quality profile.\nReferences an indexer configuration" displayname:"Indexer Template Name" longcomment:"Name of the indexer configuration template to use for this quality profile.\nReferences an indexer configuration defined in the indexers section.\nThe indexer template controls:\n- API connection details and authentication\n- Search capabilities and supported categories\n- Rate limiting and timeout settings\n- Site-specific search parameters\nMust exactly match the 'name' field of an indexers configuration.\nDifferent quality profiles can use different indexers for specialized content.\nExample: 'nzbgeek-hd' for a high-definition focused indexer setup" toml:"template_indexer"`
	// CfgIndexer is a pointer to the IndexersConfig for this indexer
	CfgIndexer *IndexersConfig `toml:"-"`
	// TemplateDownloader is the template to use for the downloader
	TemplateDownloader string `comment:"Name of the downloader configuration template to use for this indexer.\nReferences a downloader configuration defined" displayname:"Downloader Template Name" longcomment:"Name of the downloader configuration template to use for this indexer.\nReferences a downloader configuration defined in the downloader section.\nThe downloader template controls:\n- Download client connection (SABnzbd, NZBGet, qBittorrent, etc.)\n- Authentication credentials and API settings\n- Download categories and priority settings\n- Post-processing behavior\nMust exactly match the 'name' field of a downloader configuration.\nAllows different indexers to use different download clients or settings.\nExample: 'sabnzbd-movies' for movie-specific download handling" toml:"template_downloader"`
	// CfgDownloader is a pointer to the DownloaderConfig for this downloader
	CfgDownloader *DownloaderConfig `toml:"-"`
	// TemplateRegex is the template to use for the regex
	TemplateRegex string `comment:"Name of the regex configuration template to use for filtering releases from this indexer.\nReferences a" displayname:"Regex Template Name" longcomment:"Name of the regex configuration template to use for filtering releases from this indexer.\nReferences a regex configuration defined in the regex section.\nThe regex template controls:\n- Release name patterns to require or reject\n- Group name filtering rules\n- Quality and format validation patterns\n- Size and naming convention filters\nMust exactly match the 'name' field of a regex configuration.\nLeave empty to disable regex filtering for this indexer.\nUseful for indexer-specific filtering needs.\nExample: 'anime-regex' for anime-specific release filtering" toml:"template_regex"`
	// CfgRegex is a pointer to the RegexConfig for this regex
	CfgRegex *RegexConfig `toml:"-"`
	// TemplatePathNzb is the template to use for the nzb path
	TemplatePathNzb string `comment:"Name of the path configuration template for storing NZB/torrent files from this indexer.\nReferences a path" displayname:"NZB Path Template Name" longcomment:"Name of the path configuration template for storing NZB/torrent files from this indexer.\nReferences a path configuration defined in the paths section.\nThe NZB path template controls:\n- Directory where .nzb or .torrent files are saved\n- File naming and organization for download files\n- Cleanup and retention policies for download files\n- Access permissions and file handling\nMust exactly match the 'name' field of a paths configuration.\nUseful for organizing download files by indexer or quality.\nExample: 'nzb-storage' for centralized NZB file storage" toml:"template_path_nzb"`
	// CfgPath is a pointer to the PathsConfig for this path
	CfgPath *PathsConfig `toml:"-"`
	// CategoryDownloader is the category to use for the downloader
	CategoryDownloader string `comment:"Download category to assign to releases from this indexer.\nSpecifies which category the download client should" displayname:"Download Category" longcomment:"Download category to assign to releases from this indexer.\nSpecifies which category the download client should use for organization.\nCategories help organize downloads and can trigger different post-processing.\nCommon categories:\n- 'movies', 'tv', 'anime' for content type organization\n- 'hd', '4k', 'sd' for quality-based organization\n- 'priority', 'bulk' for processing priority\nMust match categories configured in your download client.\nLeave empty to use the download client's default category.\nExample: 'movies-4k' for 4K movie downloads" toml:"category_downloader"`
	// AdditionalQueryParams are additional params to add to the indexer query string
	AdditionalQueryParams string `comment:"Additional URL parameters to append to indexer search queries.\nAllows customization of indexer-specific search options not" displayname:"Additional Query Parameters" longcomment:"Additional URL parameters to append to indexer search queries.\nAllows customization of indexer-specific search options not covered by standard settings.\nParameters are appended to the search URL as-is, so include proper formatting.\nCommon examples:\n- '&extended=1' to enable extended search features\n- '&maxsize=1572864000' to set maximum file size (1.5GB in bytes)\n- '&minage=0&maxage=365' to set age limits in days\n- '&season=complete' for season pack preferences\nFormat: '&param1=value1&param2=value2' (include leading &)\nExample: '&extended=1&maxsize=5368709120' for extended search with 5GB size limit" toml:"additional_query_params"`
	// SkipEmptySize indicates if releases with an empty size are allowed
	SkipEmptySize bool `comment:"Skip releases that don't report a file size from this indexer.\nWhen true, releases without size" displayname:"Skip Empty Size Releases" longcomment:"Skip releases that don't report a file size from this indexer.\nWhen true, releases without size information are ignored.\nWhen false, releases with missing size information are processed normally.\nMissing size info can indicate:\n- Indexer limitations or API issues\n- Fake or problematic releases\n- Freeleech torrents (some trackers)\nRecommended: true for Usenet indexers, false for torrent trackers.\nHelps filter out potentially problematic releases.\nDefault: false" toml:"skip_empty_size"`
	// HistoryCheckTitle indicates if the download history should check the title in addition to the url
	HistoryCheckTitle bool `comment:"Enable title-based duplicate checking in addition to URL-based checking.\nWhen true, both release URLs and titles" displayname:"Check Title In History" longcomment:"Enable title-based duplicate checking in addition to URL-based checking.\nWhen true, both release URLs and titles are checked against download history.\nWhen false, only URLs are checked for duplicates (faster).\nTitle checking helps prevent:\n- Re-downloading same content from different URLs\n- Downloading reposts or mirrors of already grabbed releases\n- Processing renamed versions of already downloaded content\nUseful when indexers frequently change URLs or have multiple mirrors.\nMay increase processing time but improves duplicate detection accuracy.\nDefault: false" toml:"history_check_title"`
	// CategoriesIndexer are the categories to use for the indexer
	CategoriesIndexer string `comment:"Comma-separated list of indexer categories to search (no spaces).\nSpecifies which content categories on the indexer" displayname:"Indexer Categories" longcomment:"Comma-separated list of indexer categories to search (no spaces).\nSpecifies which content categories on the indexer should be searched.\nCategories vary by indexer but commonly include:\n- Movies: 2000, 2010, 2020, 2030, 2040, 2045, 2050, 2060\n- TV: 5000, 5020, 5030, 5040, 5045, 5050, 5060, 5070\n- Anime: 5070 (TV), 2070 (Movies)\nCheck your indexer's category list for specific numbers.\nMore categories = broader search but more API calls and results.\nExample: '2000,2010,2020' for SD/HD/UHD movies\nExample: '5000,5020,5030' for SD/HD/UHD TV shows" toml:"categories_indexer"`
}

type SchedulerConfig struct {
	// Name is the name of the template - see https://github.com/Kellerman81/go_media_downloader/wiki/Scheduler for details
	Name string `comment:"Unique name for this scheduler configuration template.\nUsed to identify this scheduler in media group and" displayname:"Scheduler Template Name" longcomment:"Unique name for this scheduler configuration template.\nUsed to identify this scheduler in media group and list configurations.\nThe scheduler controls automated task timing and frequency:\n- Missing media search intervals\n- Quality upgrade scan schedules\n- RSS feed refresh timing\n- Database maintenance intervals\n- File scanning and import schedules\nChoose descriptive names that indicate the scheduling strategy:\n- 'aggressive' for frequent checks and fast discovery\n- 'conservative' for light resource usage\n- 'priority' for high-value content\n- 'bulk' for large collection management\nSee wiki for detailed scheduling options: https://github.com/Kellerman81/go_media_downloader/wiki/Scheduler\nExample: 'standard-schedule' or 'high-priority'" toml:"name"`

	// IntervalImdb is the interval for imdb scans
	IntervalImdb string `comment:"Time interval between IMDB database updates and metadata refreshes.\nControls how often IMDB data is synchronized" displayname:"IMDB Update Interval" longcomment:"Time interval between IMDB database updates and metadata refreshes.\nControls how often IMDB data is synchronized and movie/series information is updated.\nSupports Go duration format: '1h30m', '2h', '45m', '24h', '2d'\nAlso supports cron-like format: '@daily', '@weekly', '@monthly'\nLonger intervals reduce server load but delay metadata updates.\nShorter intervals keep data fresh but increase resource usage.\nRecommended: '24h' for daily updates, '168h' for weekly\nExample: '24h' for daily IMDB updates" toml:"interval_imdb"`

	// IntervalFeeds is the interval for rss feed scans
	IntervalFeeds string `comment:"Time interval between RSS feed checks for new releases.\nControls how often external RSS feeds are polled for new content" displayname:"RSS Feed Check Interval" longcomment:"Time interval between RSS feed checks for new releases.\nControls how often external RSS feeds are polled for new content.\nSupports Go duration format: '5m', '15m', '1h', '30m', '2d'\nAlso supports cron format: '*/15 * * * *' for every 15 minutes\nShorter intervals catch new releases faster but increase server load.\nLonger intervals reduce load but may miss time-sensitive releases.\nBalance based on feed update frequency and urgency needs.\nRecommended: '15m' to '1h' depending on feed activity\nExample: '30m' for moderate RSS feed monitoring" toml:"interval_feeds"`

	// IntervalFeedsRefreshSeries is the interval for rss feed refreshes for series
	IntervalFeedsRefreshSeries string `comment:"Time interval for refreshing TV series metadata from RSS feeds.\nControls how often series information is" displayname:"Series Metadata Refresh Interval" longcomment:"Time interval for refreshing TV series metadata from RSS feeds.\nControls how often series information is updated from RSS sources.\nThis includes episode lists, season information, and series metadata.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nTV series change less frequently than movies, so longer intervals are typical.\nBalance between keeping episode data current and resource usage.\nRecommended: '12h' to '24h' for active series\nExample: '12h' for twice-daily series metadata refresh" toml:"interval_feeds_refresh_series"`

	// IntervalFeedsRefreshMovies is the interval for rss feed refreshes for movies
	IntervalFeedsRefreshMovies string `comment:"Time interval for refreshing movie metadata from RSS feeds.\nControls how often movie information is updated" displayname:"Movie Metadata Refresh Interval" longcomment:"Time interval for refreshing movie metadata from RSS feeds.\nControls how often movie information is updated from RSS sources.\nThis includes release dates, ratings, and movie metadata updates.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nMovies change less frequently after release, so longer intervals work well.\nBalance between metadata freshness and resource consumption.\nRecommended: '24h' to '48h' for movie metadata\nExample: '24h' for daily movie metadata refresh" toml:"interval_feeds_refresh_movies"`

	// IntervalFeedsRefreshSeriesFull is the interval for full rss feed refreshes for series
	IntervalFeedsRefreshSeriesFull string `comment:"Time interval for complete TV series metadata rebuilds from RSS feeds.\nControls how often ALL series" displayname:"Full Series Metadata Rebuild Interval" longcomment:"Time interval for complete TV series metadata rebuilds from RSS feeds.\nControls how often ALL series data is fully refreshed and rebuilt.\nThis is more comprehensive than regular refresh and rebuilds entire series records.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull refreshes are resource-intensive but ensure data consistency.\nShould be much less frequent than regular refreshes.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly full series refresh" toml:"interval_feeds_refresh_series_full"`

	// IntervalFeedsRefreshMoviesFull is the interval for full rss feed refreshes for movies
	IntervalFeedsRefreshMoviesFull string `comment:"Time interval for complete movie metadata rebuilds from RSS feeds.\nControls how often ALL movie data" displayname:"Full Movie Metadata Rebuild Interval" longcomment:"Time interval for complete movie metadata rebuilds from RSS feeds.\nControls how often ALL movie data is fully refreshed and rebuilt.\nThis is more comprehensive than regular refresh and rebuilds entire movie records.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull refreshes are resource-intensive but ensure data consistency.\nShould be much less frequent than regular refreshes.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '720h' for monthly full movie refresh" toml:"interval_feeds_refresh_movies_full"`

	// IntervalIndexerMissing is the interval for missing media scans
	IntervalIndexerMissing string `comment:"Time interval between incremental searches for missing media.\nControls how often indexers are searched for media" displayname:"Missing Media Search Interval" longcomment:"Time interval between incremental searches for missing media.\nControls how often indexers are searched for media not yet in your library.\nThis is incremental scanning that processes a limited number of items per run.\nSupports Go duration format: '30m', '1h', '2h', '6h', '2d'\nAlso supports cron format for specific timing\nShorter intervals find new content faster but increase indexer API usage.\nLonger intervals reduce load but delay content discovery.\nRecommended: '1h' to '6h' depending on urgency and indexer limits\nExample: '2h' for moderate missing content discovery" toml:"interval_indexer_missing"`

	// IntervalIndexerUpgrade is the interval for upgrade media scans
	IntervalIndexerUpgrade string `comment:"Time interval between incremental searches for media quality upgrades.\nControls how often indexers are searched for better quality versions" displayname:"Quality Upgrade Search Interval" longcomment:"Time interval between incremental searches for media quality upgrades.\nControls how often indexers are searched for better quality versions of existing media.\nThis is incremental scanning that processes a limited number of items per run.\nSupports Go duration format: '2h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nUpgrade scans are typically less urgent than missing content searches.\nBalance between finding upgrades and conserving indexer API calls.\nRecommended: '6h' to '24h' depending on upgrade priority\nExample: '12h' for twice-daily upgrade scanning" toml:"interval_indexer_upgrade"`

	// IntervalIndexerMissingFull is the interval for full missing media scans
	IntervalIndexerMissingFull string `comment:"Time interval between comprehensive searches for ALL missing media.\nControls how often a complete scan for missing content is performed" displayname:"Full Missing Media Scan Interval" longcomment:"Time interval between comprehensive searches for ALL missing media.\nControls how often a complete scan for missing content is performed.\nThis processes the entire library, not just incremental batches.\nSupports Go duration format: '24h', '48h', '168h' (1 week), '8d'\nAlso supports cron format for scheduled timing\nFull scans are resource-intensive and consume many indexer API calls.\nShould be much less frequent than incremental scans.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly comprehensive missing content scan" toml:"interval_indexer_missing_full"`

	// IntervalIndexerUpgradeFull is the interval for full upgrade media scans
	IntervalIndexerUpgradeFull string `comment:"Time interval between comprehensive searches for ALL media upgrades.\nControls how often a complete scan for quality upgrades is performed" displayname:"Full Upgrade Scan Interval" longcomment:"Time interval between comprehensive searches for ALL media upgrades.\nControls how often a complete scan for quality upgrades is performed.\nThis processes the entire library, not just incremental batches.\nSupports Go duration format: '48h', '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull upgrade scans are very resource-intensive and use many API calls.\nShould be infrequent as upgrades are less urgent than missing content.\nRecommended: '720h' (monthly) to '2160h' (quarterly)\nExample: '720h' for monthly comprehensive upgrade scanning" toml:"interval_indexer_upgrade_full"`

	// IntervalIndexerMissingTitle is the interval for missing media scans by title
	IntervalIndexerMissingTitle string `comment:"Time interval between incremental title-based searches for missing media.\nControls how often indexers are searched using" displayname:"Title Missing Search Interval" longcomment:"Time interval between incremental title-based searches for missing media.\nControls how often indexers are searched using media titles instead of IDs.\nUseful when ID-based searches fail or indexers have poor ID coverage.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nTitle searches are less accurate but broader than ID searches.\nShould be less frequent than ID-based searches due to accuracy concerns.\nRecommended: '12h' to '48h' depending on indexer ID coverage\nExample: '24h' for daily title-based missing content search" toml:"interval_indexer_missing_title"`

	// IntervalIndexerUpgradeTitle is the interval for upgrade media scans by title
	IntervalIndexerUpgradeTitle string `comment:"Time interval between incremental title-based searches for media upgrades.\nControls how often indexers are searched for upgrades using media titles" displayname:"Title Upgrade Search Interval" longcomment:"Time interval between incremental title-based searches for media upgrades.\nControls how often indexers are searched for upgrades using media titles instead of IDs.\nUseful when ID-based upgrade searches miss releases with poor tagging.\nSupports Go duration format: '12h', '24h', '48h', '168h', '2d'\nAlso supports cron format for specific timing\nTitle-based upgrade searches can be less precise than ID-based searches.\nShould be infrequent due to potential false positives.\nRecommended: '48h' to '168h' depending on upgrade needs\nExample: '48h' for twice-weekly title-based upgrade search" toml:"interval_indexer_upgrade_title"`

	// IntervalIndexerMissingFullTitle is the interval for full missing media scans
	IntervalIndexerMissingFullTitle string `comment:"Time interval between comprehensive title-based searches for ALL missing media.\nControls how often complete title-based missing" displayname:"Full Title Missing Scan Interval" longcomment:"Time interval between comprehensive title-based searches for ALL missing media.\nControls how often complete title-based missing content scans are performed.\nProcesses entire library using titles when ID searches are insufficient.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for scheduled timing\nFull title scans are very resource-intensive and less accurate than ID scans.\nUse sparingly due to potential false positives and high API usage.\nRecommended: '720h' (monthly) to '2160h' (quarterly)\nExample: '720h' for monthly full title-based missing scan" toml:"interval_indexer_missing_full_title"`
	// IntervalIndexerUpgradeFullTitle is the interval for full upgrade media scans
	IntervalIndexerUpgradeFullTitle string `comment:"Time interval between comprehensive title-based searches for ALL media upgrades.\nControls how often complete title-based upgrade" displayname:"Full Title Upgrade Scan Interval" longcomment:"Time interval between comprehensive title-based searches for ALL media upgrades.\nControls how often complete title-based upgrade scans are performed.\nProcesses entire library using titles when ID-based upgrade searches are insufficient.\nSupports Go duration format: '720h' (1 month), '2160h' (3 months), '8d'\nAlso supports cron format for scheduled timing\nFull title upgrade scans have high false positive risk and massive API usage.\nShould be very infrequent due to accuracy and resource concerns.\nRecommended: '2160h' (quarterly) or disable entirely\nExample: '2160h' for quarterly full title-based upgrade scan" toml:"interval_indexer_upgrade_full_title"`
	// IntervalIndexerRss is the interval for rss feed scans
	IntervalIndexerRss string `comment:"Time interval between indexer-specific RSS feed checks.\nControls how often each indexer's RSS feeds are polled" displayname:"Indexer RSS Feed Interval" longcomment:"Time interval between indexer-specific RSS feed checks.\nControls how often each indexer's RSS feeds are polled for new releases.\nDifferent from general feed scanning, this is indexer-focused RSS monitoring.\nSupports Go duration format: '5m', '15m', '30m', '1h', '2d'\nAlso supports cron format for specific timing\nIndexer RSS feeds often update frequently with new releases.\nShorter intervals catch releases faster but increase API usage.\nRecommended: '15m' to '1h' depending on indexer feed activity\nExample: '30m' for half-hourly indexer RSS monitoring" toml:"interval_indexer_rss"`
	// IntervalScanData is the interval for data scans
	IntervalScanData string `comment:"Time interval between filesystem scans for media file changes.\nControls how often storage paths are scanned" displayname:"Filesystem Scan Interval" longcomment:"Time interval between filesystem scans for media file changes.\nControls how often storage paths are scanned for new, moved, or deleted files.\nThis maintains synchronization between filesystem and database records.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nFrequent scans keep database current but increase I/O load.\nBalance based on how often files are added/moved externally.\nRecommended: '6h' to '24h' depending on library activity\nExample: '12h' for twice-daily filesystem scanning" toml:"interval_scan_data"`
	// IntervalScanDataMissing is the interval for missing data scans
	IntervalScanDataMissing string `comment:"Time interval between scans for media files that should exist but are missing.\nControls how often" displayname:"Missing File Scan Interval" longcomment:"Time interval between scans for media files that should exist but are missing.\nControls how often the system checks for files that are tracked but no longer on disk.\nHelps identify moved, deleted, or corrupted media files.\nSupports Go duration format: '6h', '12h', '24h', '48h', '2d'\nAlso supports cron format for specific timing\nMissing file detection helps maintain library integrity.\nFrequent scans catch issues faster but increase I/O overhead.\nRecommended: '24h' to '48h' for missing file detection\nExample: '24h' for daily missing file verification" toml:"interval_scan_data_missing"`
	// IntervalScanDataFlags is the interval for flagged data scans
	IntervalScanDataFlags string `comment:"Time interval between scans for media files marked with processing flags.\nControls how often files flagged" displayname:"Flagged File Scan Interval" longcomment:"Time interval between scans for media files marked with processing flags.\nControls how often files flagged for reprocessing, upgrading, or fixing are handled.\nFlags indicate files needing attention (corrupt, misnamed, quality issues).\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nFlagged files often need prompt attention to resolve issues.\nShorter intervals resolve problems faster but increase processing load.\nRecommended: '6h' to '12h' for flagged file processing\nExample: '6h' for four-times-daily flagged file handling" toml:"interval_scan_data_flags"`
	// IntervalScanDataimport is the interval for data import scans
	IntervalScanDataimport string `comment:"Time interval between scans for new media files to import from configured import paths.\nControls how" displayname:"Import Directory Scan Interval" longcomment:"Time interval between scans for new media files to import from configured import paths.\nControls how often import directories are scanned for existing media to add to library.\nUseful for gradually importing large existing collections.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nImport scanning processes external media for library integration.\nFrequency depends on how often new files are added to import paths.\nRecommended: '12h' to '24h' for import directory monitoring\nExample: '12h' for twice-daily import scanning" toml:"interval_scan_data_import"`
	// IntervalDatabaseBackup is the interval for database backups
	IntervalDatabaseBackup string `comment:"Time interval between automatic database backup operations.\nControls how often the application database is backed up" displayname:"Database Backup Interval" longcomment:"Time interval between automatic database backup operations.\nControls how often the application database is backed up for safety.\nBackups protect against data loss from corruption or system failures.\nSupports Go duration format: '24h', '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for specific timing (e.g., daily at 3 AM)\nDatabase backups temporarily lock the database during operation.\nBalance between data protection and system performance impact.\nRecommended: '24h' for daily backups, '168h' for weekly\nExample: '24h' for daily database backup at configured time" toml:"interval_database_backup"`
	// IntervalDatabaseCheck is the interval for database checks
	IntervalDatabaseCheck string `comment:"Time interval between database integrity and consistency checks.\nControls how often the database is examined for corruption or inconsistencies" displayname:"Database Check Interval" longcomment:"Time interval between database integrity and consistency checks.\nControls how often the database is examined for corruption or inconsistencies.\nChecks help identify and repair database issues before they cause problems.\nSupports Go duration format: '168h' (1 week), '720h' (1 month), '8d'\nAlso supports cron format for specific timing\nDatabase checks can be resource-intensive and temporarily slow operations.\nInfrequent checks balance integrity monitoring with performance impact.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly database integrity verification" toml:"interval_database_check"`
	// IntervalIndexerRssSeasons is the interval for rss feed season scans
	IntervalIndexerRssSeasons string `comment:"Time interval between RSS feed checks specifically for TV season packs and batches.\nControls how often" displayname:"Season Pack RSS Interval" longcomment:"Time interval between RSS feed checks specifically for TV season packs and batches.\nControls how often indexer RSS feeds are scanned for complete season releases.\nSeason packs provide entire TV seasons in single downloads.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nSeason releases are less frequent but valuable for batch downloading.\nBalance between catching season packs and API usage.\nRecommended: '6h' to '24h' depending on season pack priority\nExample: '12h' for twice-daily season pack monitoring" toml:"interval_indexer_rss_seasons"`
	// IntervalIndexerRssSeasonsAll is the interval for rss feed all season scans
	IntervalIndexerRssSeasonsAll string `comment:"Time interval between comprehensive RSS feed scans for ALL available season packs.\nControls how often complete" displayname:"Full Season RSS Scan Interval" longcomment:"Time interval between comprehensive RSS feed scans for ALL available season packs.\nControls how often complete season pack catalogs are refreshed from indexer feeds.\nThis processes all available seasons, not just new releases.\nSupports Go duration format: '24h', '48h', '168h' (1 week), '2d'\nAlso supports cron format for specific timing\nComprehensive season scanning is resource-intensive.\nShould be much less frequent than regular season monitoring.\nRecommended: '168h' (weekly) to '720h' (monthly)\nExample: '168h' for weekly comprehensive season catalog refresh" toml:"interval_indexer_rss_seasons_all"`
	// CronIndexerRssSeasonsAll is the cron schedule for rss feed all season scans
	CronIndexerRssSeasonsAll string `comment:"Cron schedule for comprehensive RSS season pack scans (alternative to interval).\nUse cron format for precise" displayname:"Full Season RSS Cron Schedule" longcomment:"Cron schedule for comprehensive RSS season pack scans (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 3 * * 1': Every Monday at 3 AM\n- '0 2 */7 * *': Every 7 days at 2 AM\n- '30 1 1 * *': First day of each month at 1:30 AM\nCron scheduling provides better control than intervals for resource management.\nUseful for scheduling intensive scans during low-usage periods.\nExample: '0 2 * * 0' for Sunday at 2 AM weekly scan" toml:"cron_indexer_rss_seasons_all"`
	// CronIndexerRssSeasons is the cron schedule for rss feed season scans
	CronIndexerRssSeasons string `comment:"Cron schedule for RSS season pack monitoring (alternative to interval).\nUse cron format for precise timing" displayname:"Season Pack RSS Cron Schedule" longcomment:"Cron schedule for RSS season pack monitoring (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 8,20 * * *': Daily at 8 AM and 8 PM\n- '15 */4 * * *': Every 4 hours at 15 minutes past\nAllows scheduling season checks at optimal times.\nUseful for avoiding peak indexer usage periods.\nExample: '0 */8 * * *' for every 8 hours season monitoring" toml:"cron_indexer_rss_seasons"`
	// IntervalIndexerRssArtists is the interval for artist-based missing/upgrade searches (music)
	IntervalIndexerRssArtists string `comment:"Time interval between RSS searches for missing albums by artist.\nSearches by artist name to find multiple missing albums per search" displayname:"Artist RSS Search Interval" longcomment:"Time interval between RSS searches for missing albums by artist.\nSearches by artist name to find multiple missing albums per search.\nReduces search volume compared to per-album searching.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing.\nRecommended: '6h' to '24h' depending on library size.\nExample: '12h' for twice-daily artist-based search" toml:"interval_indexer_rss_artists"`
	// IntervalIndexerRssArtistsUpgrade is the interval for artist-based upgrade searches (music)
	IntervalIndexerRssArtistsUpgrade string `comment:"Time interval between RSS searches for album upgrades by artist.\nSearches by artist name to find quality upgrades for existing albums" displayname:"Artist RSS Upgrade Interval" longcomment:"Time interval between RSS searches for album upgrades by artist.\nSearches by artist name to find quality upgrades for existing albums.\nReduces search volume compared to per-album searching.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing.\nRecommended: '12h' to '48h' depending on upgrade priority.\nExample: '24h' for daily artist-based upgrade search" toml:"interval_indexer_rss_artists_upgrade"`
	// CronIndexerRssArtists is the cron schedule for artist-based missing searches (music)
	CronIndexerRssArtists string `comment:"Cron schedule for artist-based missing album searches (alternative to interval).\nUse cron format for precise timing" displayname:"Artist RSS Cron Schedule" longcomment:"Cron schedule for artist-based missing album searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 8,20 * * *': Daily at 8 AM and 8 PM\nUseful for avoiding peak indexer usage periods.\nExample: '0 */8 * * *' for every 8 hours artist search" toml:"cron_indexer_rss_artists"`
	// CronIndexerRssArtistsUpgrade is the cron schedule for artist-based upgrade searches (music)
	CronIndexerRssArtistsUpgrade string `comment:"Cron schedule for artist-based album upgrade searches (alternative to interval).\nUse cron format for precise timing" displayname:"Artist RSS Upgrade Cron Schedule" longcomment:"Cron schedule for artist-based album upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * *': Daily at 4 AM\n- '0 2 * * 0': Weekly on Sunday at 2 AM\nSchedule during low-usage periods.\nExample: '0 3 * * *' for daily artist upgrade search at 3 AM" toml:"cron_indexer_rss_artists_upgrade"`
	// IntervalIndexerRssAuthors is the interval for author-based missing searches (audiobooks/books)
	IntervalIndexerRssAuthors string `comment:"Time interval between RSS searches for missing books/audiobooks by author.\nSearches by author name to find multiple missing items per search" displayname:"Author RSS Search Interval" longcomment:"Time interval between RSS searches for missing books/audiobooks by author.\nSearches by author name to find multiple missing items per search.\nReduces search volume compared to per-title searching.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing.\nRecommended: '6h' to '24h' depending on library size.\nExample: '12h' for twice-daily author-based search" toml:"interval_indexer_rss_authors"`
	// IntervalIndexerRssAuthorsUpgrade is the interval for author-based upgrade searches (audiobooks/books)
	IntervalIndexerRssAuthorsUpgrade string `comment:"Time interval between RSS searches for book/audiobook upgrades by author.\nSearches by author name to find quality upgrades" displayname:"Author RSS Upgrade Interval" longcomment:"Time interval between RSS searches for book/audiobook upgrades by author.\nSearches by author name to find quality upgrades for existing titles.\nReduces search volume compared to per-title searching.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing.\nRecommended: '12h' to '48h' depending on upgrade priority.\nExample: '24h' for daily author-based upgrade search" toml:"interval_indexer_rss_authors_upgrade"`
	// CronIndexerRssAuthors is the cron schedule for author-based missing searches (audiobooks/books)
	CronIndexerRssAuthors string `comment:"Cron schedule for author-based missing item searches (alternative to interval).\nUse cron format for precise timing" displayname:"Author RSS Cron Schedule" longcomment:"Cron schedule for author-based missing item searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 8,20 * * *': Daily at 8 AM and 8 PM\nUseful for avoiding peak indexer usage periods.\nExample: '0 */8 * * *' for every 8 hours author search" toml:"cron_indexer_rss_authors"`
	// CronIndexerRssAuthorsUpgrade is the cron schedule for author-based upgrade searches (audiobooks/books)
	CronIndexerRssAuthorsUpgrade string `comment:"Cron schedule for author-based item upgrade searches (alternative to interval).\nUse cron format for precise timing" displayname:"Author RSS Upgrade Cron Schedule" longcomment:"Cron schedule for author-based item upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * *': Daily at 4 AM\n- '0 2 * * 0': Weekly on Sunday at 2 AM\nSchedule during low-usage periods.\nExample: '0 3 * * *' for daily author upgrade search at 3 AM" toml:"cron_indexer_rss_authors_upgrade"`
	// CronImdb is the cron schedule for imdb scans
	CronImdb string `comment:"Cron schedule for IMDB database updates (alternative to interval).\nUse cron format for precise timing control" displayname:"IMDB Update Cron Schedule" longcomment:"Cron schedule for IMDB database updates (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * *': Daily at 4 AM\n- '0 2 * * 1': Weekly on Monday at 2 AM\n- '0 3 1 * *': Monthly on first day at 3 AM\nIMDB updates are resource-intensive, best scheduled during low-usage periods.\nAllows coordination with IMDB's data release schedule.\nExample: '0 3 * * *' for daily IMDB updates at 3 AM" toml:"cron_imdb"`
	// CronFeeds is the cron schedule for rss feed scans
	CronFeeds string `comment:"Cron schedule for RSS feed monitoring (alternative to interval).\nUse cron format for precise timing control" displayname:"RSS Feed Cron Schedule" longcomment:"Cron schedule for RSS feed monitoring (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '*/15 * * * *': Every 15 minutes\n- '0,30 * * * *': Every 30 minutes (at 0 and 30)\n- '*/10 * * * *': Every 10 minutes\nRSS feeds update frequently, so shorter intervals are common.\nCron allows avoiding specific times when feeds might be busy.\nExample: '*/20 * * * *' for every 20 minutes RSS monitoring" toml:"cron_feeds"`

	// CronFeedsRefreshSeries is the cron schedule for refreshing series RSS feeds
	CronFeedsRefreshSeries string `comment:"Cron schedule for TV series RSS feed metadata refreshes (alternative to interval).\nUse cron format for precise timing control instead of simple intervals" displayname:"Series Metadata Cron Schedule" longcomment:"Cron schedule for TV series RSS feed metadata refreshes (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */12 * * *': Every 12 hours\n- '0 6,18 * * *': Daily at 6 AM and 6 PM\n- '0 8 * * *': Daily at 8 AM\nSeries metadata changes less frequently than new releases.\nSchedule during periods when metadata sources are most current.\nExample: '0 */12 * * *' for twice-daily series metadata refresh" toml:"cron_feeds_refresh_series"`
	// CronFeedsRefreshMovies is the cron schedule for refreshing movie RSS feeds
	CronFeedsRefreshMovies string `comment:"Cron schedule for movie RSS feed metadata refreshes (alternative to interval).\nUse cron format for precise" displayname:"Movie Metadata Cron Schedule" longcomment:"Cron schedule for movie RSS feed metadata refreshes (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * *': Daily at 4 AM\n- '0 2 * * 1,4': Twice weekly on Monday and Thursday at 2 AM\n- '0 6 */2 * *': Every other day at 6 AM\nMovie metadata updates are less frequent than series.\nSchedule when movie databases typically update.\nExample: '0 4 * * *' for daily movie metadata refresh at 4 AM" toml:"cron_feeds_refresh_movies"`
	// CronFeedsRefreshSeriesFull is the cron schedule for full refreshing of series RSS feeds
	CronFeedsRefreshSeriesFull string `comment:"Cron schedule for complete TV series metadata rebuilds (alternative to interval).\nUse cron format for precise" displayname:"Full Series Rebuild Cron Schedule" longcomment:"Cron schedule for complete TV series metadata rebuilds (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 1 * * 0': Weekly on Sunday at 1 AM\n- '0 2 1 * *': Monthly on first day at 2 AM\n- '0 3 */14 * *': Every 14 days at 3 AM\nFull refreshes are resource-intensive and should be infrequent.\nSchedule during lowest usage periods for minimal impact.\nExample: '0 1 * * 0' for weekly full series refresh on Sunday" toml:"cron_feeds_refresh_series_full"`
	// CronFeedsRefreshMoviesFull is the cron schedule for full refreshing of movie RSS feeds
	CronFeedsRefreshMoviesFull string `comment:"Cron schedule for complete movie metadata rebuilds (alternative to interval).\nUse cron format for precise timing" displayname:"Full Movie Rebuild Cron Schedule" longcomment:"Cron schedule for complete movie metadata rebuilds (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 2 * * 0': Weekly on Sunday at 2 AM\n- '0 3 1 * *': Monthly on first day at 3 AM\n- '0 1 1 */3 *': Quarterly on first day at 1 AM\nFull movie refreshes are very resource-intensive.\nSchedule during absolute lowest usage periods.\nExample: '0 2 1 * *' for monthly full movie refresh on first day" toml:"cron_feeds_refresh_movies_full"`
	// CronIndexerMissing is the cron schedule for missing media scans
	CronIndexerMissing string `comment:"Cron schedule for incremental missing media searches (alternative to interval).\nUse cron format for precise timing" displayname:"Missing Media Cron Schedule" longcomment:"Cron schedule for incremental missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */2 * * *': Every 2 hours\n- '0 8,14,20 * * *': Three times daily at 8 AM, 2 PM, 8 PM\n- '*/30 * * * *': Every 30 minutes\nAllows scheduling searches during indexer low-traffic periods.\nAvoid peak hours when indexers may be slow or limited.\nExample: '0 */3 * * *' for every 3 hours missing content search" toml:"cron_indexer_missing"`
	// CronIndexerUpgrade is the cron schedule for upgrade media scans
	CronIndexerUpgrade string `comment:"Cron schedule for incremental media quality upgrade searches (alternative to interval).\nUse cron format for precise" displayname:"Quality Upgrade Cron Schedule" longcomment:"Cron schedule for incremental media quality upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 10,22 * * *': Daily at 10 AM and 10 PM\n- '0 14 * * *': Daily at 2 PM\nUpgrade searches are less urgent than missing content searches.\nSchedule during moderate usage periods when indexers are responsive.\nExample: '0 */8 * * *' for every 8 hours upgrade search" toml:"cron_indexer_upgrade"`
	// CronIndexerMissingFull is the cron schedule for full missing media scans
	CronIndexerMissingFull string `comment:"Cron schedule for comprehensive missing media searches (alternative to interval).\nUse cron format for precise timing" displayname:"Full Missing Scan Cron Schedule" longcomment:"Cron schedule for comprehensive missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 2 * * 0': Weekly on Sunday at 2 AM\n- '0 1 1 * *': Monthly on first day at 1 AM\n- '0 3 */7 * *': Every 7 days at 3 AM\nFull missing scans consume massive indexer API calls.\nSchedule very infrequently during absolute lowest usage periods.\nExample: '0 2 * * 0' for weekly comprehensive missing scan on Sunday" toml:"cron_indexer_missing_full"`
	// CronIndexerUpgradeFull is the cron schedule for full upgrade media scans
	CronIndexerUpgradeFull string `comment:"Cron schedule for comprehensive media upgrade searches (alternative to interval).\nUse cron format for precise timing" displayname:"Full Upgrade Scan Cron Schedule" longcomment:"Cron schedule for comprehensive media upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 3 1 * *': Monthly on first day at 3 AM\n- '0 4 1 */3 *': Quarterly on first day at 4 AM\n- '0 2 15 * *': Monthly on 15th day at 2 AM\nFull upgrade scans are extremely resource-intensive.\nShould be very infrequent due to massive API usage.\nExample: '0 3 1 * *' for monthly comprehensive upgrade scan" toml:"cron_indexer_upgrade_full"`
	// CronIndexerMissingTitle is the cron schedule for missing media scans by title
	CronIndexerMissingTitle string `comment:"Cron schedule for incremental title-based missing media searches (alternative to interval).\nUse cron format for precise" displayname:"Title Missing Cron Schedule" longcomment:"Cron schedule for incremental title-based missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 12 * * *': Daily at noon\n- '0 6 * * 1,4': Twice weekly on Monday and Thursday at 6 AM\n- '0 18 */2 * *': Every other day at 6 PM\nTitle searches are less accurate but broader than ID searches.\nSchedule less frequently than ID searches due to accuracy concerns.\nExample: '0 12 * * *' for daily title-based missing search at noon" toml:"cron_indexer_missing_title"`
	// CronIndexerUpgradeTitle is the cron schedule for upgrade media scans by title
	CronIndexerUpgradeTitle string `comment:"Cron schedule for incremental title-based upgrade searches (alternative to interval).\nUse cron format for precise timing" displayname:"Title Upgrade Cron Schedule" longcomment:"Cron schedule for incremental title-based upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 16 * * *': Daily at 4 PM\n- '0 9 * * 2,5': Twice weekly on Tuesday and Friday at 9 AM\n- '0 20 */3 * *': Every 3 days at 8 PM\nTitle-based upgrade searches can be less precise than ID-based searches.\nSchedule infrequently due to potential false positives.\nExample: '0 16 */2 * *' for every other day title-based upgrade search" toml:"cron_indexer_upgrade_title"`

	// CronIndexerMissingFullTitle is the cron schedule for full missing media scans
	CronIndexerMissingFullTitle string `comment:"Cron schedule for comprehensive title-based missing media searches (alternative to interval).\nUse cron format for precise" displayname:"Full Title Missing Cron Schedule" longcomment:"Cron schedule for comprehensive title-based missing media searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 1 * *': Monthly on first day at 4 AM\n- '0 5 1 */3 *': Quarterly on first day at 5 AM\n- '0 3 15 * *': Monthly on 15th day at 3 AM\nFull title scans are very resource-intensive and less accurate than ID scans.\nUse sparingly due to potential false positives and high API usage.\nExample: '0 4 1 * *' for monthly full title-based missing scan" toml:"cron_indexer_missing_full_title"`
	// CronIndexerUpgradeFullTitle is the cron schedule for full upgrade media scans
	CronIndexerUpgradeFullTitle string `comment:"Cron schedule for comprehensive title-based upgrade searches (alternative to interval).\nUse cron format for precise timing" displayname:"Full Title Upgrade Cron Schedule" longcomment:"Cron schedule for comprehensive title-based upgrade searches (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 5 1 */3 *': Quarterly on first day at 5 AM\n- '0 6 1 */6 *': Twice yearly on first day at 6 AM\n- 'disabled': Consider disabling due to high false positive risk\nFull title upgrade scans have high false positive risk and massive API usage.\nShould be very infrequent or disabled entirely.\nExample: '0 5 1 */6 *' for semi-annual full title upgrade scan (or disable)" toml:"cron_indexer_upgrade_full_title"`
	// CronIndexerRss is the cron schedule for rss feed scans
	CronIndexerRss string `comment:"Cron schedule for indexer-specific RSS feed monitoring (alternative to interval).\nUse cron format for precise timing" displayname:"Indexer RSS Cron Schedule" longcomment:"Cron schedule for indexer-specific RSS feed monitoring (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '*/30 * * * *': Every 30 minutes\n- '*/15 * * * *': Every 15 minutes\n- '0,20,40 * * * *': Every 20 minutes (at 0, 20, 40)\nIndexer RSS feeds often update frequently with new releases.\nBalance between catching releases quickly and API rate limits.\nExample: '*/20 * * * *' for every 20 minutes indexer RSS monitoring" toml:"cron_indexer_rss"`
	// CronScanData is the cron schedule for data scans
	CronScanData string `comment:"Cron schedule for filesystem media file scanning (alternative to interval).\nUse cron format for precise timing" displayname:"Filesystem Scan Cron Schedule" longcomment:"Cron schedule for filesystem media file scanning (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 8,20 * * *': Daily at 8 AM and 8 PM\n- '0 12 * * *': Daily at noon\nFilesystem scans maintain synchronization between disk and database.\nSchedule based on how frequently files are added/moved externally.\nExample: '0 */8 * * *' for every 8 hours filesystem scanning" toml:"cron_scan_data"`
	// CronScanDataMissing is the cron schedule for missing data scans
	CronScanDataMissing string `comment:"Cron schedule for missing media file detection scans (alternative to interval).\nUse cron format for precise" displayname:"Missing File Cron Schedule" longcomment:"Cron schedule for missing media file detection scans (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 6 * * *': Daily at 6 AM\n- '0 4 * * 0': Weekly on Sunday at 4 AM\n- '0 5 */2 * *': Every other day at 5 AM\nMissing file detection helps maintain library integrity.\nSchedule during low-usage periods due to intensive I/O operations.\nExample: '0 6 * * *' for daily missing file verification at 6 AM" toml:"cron_scan_data_missing"`
	// CronScanDataFlags is the cron schedule for flagged data scans
	CronScanDataFlags string `comment:"Cron schedule for flagged media file processing (alternative to interval).\nUse cron format for precise timing" displayname:"Flagged File Cron Schedule" longcomment:"Cron schedule for flagged media file processing (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */4 * * *': Every 4 hours\n- '0 9,15,21 * * *': Three times daily at 9 AM, 3 PM, 9 PM\n- '*/45 * * * *': Every 45 minutes\nFlagged files often need prompt attention to resolve issues.\nSchedule frequently enough to handle problems quickly.\nExample: '0 */6 * * *' for every 6 hours flagged file processing" toml:"cron_scan_data_flags"`
	// CronScanDataimport is the cron schedule for data import scans
	CronScanDataimport string `comment:"Cron schedule for media import directory scanning (alternative to interval).\nUse cron format for precise timing" displayname:"Import Directory Cron Schedule" longcomment:"Cron schedule for media import directory scanning (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */8 * * *': Every 8 hours\n- '0 10,22 * * *': Daily at 10 AM and 10 PM\n- '0 14 * * *': Daily at 2 PM\nImport scanning processes external media for library integration.\nFrequency depends on how often new files are added to import paths.\nExample: '0 */12 * * *' for every 12 hours import directory scanning" toml:"cron_scan_data_import"`
	// CronDatabaseBackup is the cron schedule for database backups
	CronDatabaseBackup string `comment:"Cron schedule for automatic database backup operations (alternative to interval).\nUse cron format for precise timing" displayname:"Database Backup Cron Schedule" longcomment:"Cron schedule for automatic database backup operations (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 3 * * *': Daily at 3 AM\n- '0 2 * * 0': Weekly on Sunday at 2 AM\n- '0 1 1 * *': Monthly on first day at 1 AM\nDatabase backups temporarily lock database during operation.\nSchedule during absolute lowest system usage periods.\nExample: '0 3 * * *' for daily database backup at 3 AM" toml:"cron_database_backup"`
	// CronDatabaseCheck is the cron schedule for database checks
	CronDatabaseCheck string `comment:"Cron schedule for database integrity and consistency checks (alternative to interval).\nUse cron format for precise" displayname:"Database Check Cron Schedule" longcomment:"Cron schedule for database integrity and consistency checks (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 4 * * 0': Weekly on Sunday at 4 AM\n- '0 5 1 * *': Monthly on first day at 5 AM\n- '0 6 1 */3 *': Quarterly on first day at 6 AM\nDatabase checks are resource-intensive and can slow operations.\nSchedule during lowest usage periods for minimal impact.\nExample: '0 4 * * 0' for weekly database integrity check on Sunday" toml:"cron_database_check"`

	// IntervalCacheRefresh is the interval for cache refreshes
	IntervalCacheRefresh string `comment:"Time interval between automatic cache refresh operations.\nControls how often database caches are fully refreshed" displayname:"Cache Refresh Interval" longcomment:"Time interval between automatic cache refresh operations.\nControls how often database caches are fully refreshed to ensure data consistency.\nCache refreshes rebuild in-memory data from the database to prevent stale data.\nSupports Go duration format: '1h', '6h', '12h', '24h', '2d'\nAlso supports cron format for specific timing\nFrequent refreshes keep data current but increase system load.\nBalance between data freshness and performance impact.\nRecommended: '6h' for regular cache refresh, '24h' for light usage\nExample: '6h' for every 6 hours cache refresh" toml:"interval_cache_refresh"`

	// CronCacheRefresh is the cron schedule for cache refreshes
	CronCacheRefresh string `comment:"Cron schedule for automatic cache refresh operations (alternative to interval).\nUse cron format for precise timing" displayname:"Cache Refresh Cron Schedule" longcomment:"Cron schedule for automatic cache refresh operations (alternative to interval).\nUse cron format for precise timing control instead of simple intervals.\nStandard cron format: 'minute hour day month weekday'\nCommon examples:\n- '0 */6 * * *': Every 6 hours\n- '0 2,8,14,20 * * *': 4 times daily at 2 AM, 8 AM, 2 PM, 8 PM\n- '0 3 * * *': Daily at 3 AM\nCache refreshes rebuild in-memory data from database for consistency.\nSchedule during lower usage periods to minimize performance impact.\nExample: '0 */6 * * *' for every 6 hours cache refresh" toml:"cron_cache_refresh"`
}

// Conf is a struct that contains a Name string field and a Data any field.
type Conf struct {
	// Name is a string field
	Name string
	// Data is an any field that can hold any type
	Data any
}

// GetMediaListsEntryListID returns the index position of the list with the given
// name in the MediaTypeConfig. Returns -1 if no match is found.
func (cfgp *MediaTypeConfig) GetMediaListsEntryListID(listname string) int {
	if listname == "" {
		return -1
	}

	if cfgp == nil {
		logger.Logtype("error", 0).Msg("the config couldnt be found")
		return -1
	}

	k, ok := cfgp.ListsMapIdx[listname]
	if ok {
		return k
	}

	for k := range cfgp.Lists {
		if cfgp.Lists[k].Name == listname || strings.EqualFold(cfgp.Lists[k].Name, listname) {
			return k
		}
	}

	return -1
}

// GetMediaQualityConfigStr returns the QualityConfig from cfgp for the
// media with the given ID. It first checks if there is a quality profile
// set for that media in the database. If not, it returns the default
// QualityConfig from cfgp.
func (cfgp *MediaTypeConfig) GetMediaQualityConfigStr(str string) *QualityConfig {
	if cfgp == nil {
		return nil
	}

	return GetSettingsQuality(str)
}

// Getlistnamefilterignore returns a SQL WHERE clause to filter movies
// by list name ignore lists. If the list has ignore lists configured,
// it will generate a clause to exclude movies in those lists.
// Otherwise returns empty string.
func (list *MediaListsConfig) Getlistnamefilterignore() string {
	if list.IgnoreMapListsLen >= 1 {
		return ("listname in (?" + list.IgnoreMapListsQu + ") and ")
	}

	return ""
}

// QualityIndexerByQualityAndTemplate returns the CategoriesIndexer string for the indexer
// in the given QualityConfig that matches the given IndexersConfig by name.
// Returns empty string if no match is found.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplate(ind *IndexersConfig) int {
	if ind == nil {
		return -1
	}

	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return index
		}
	}

	return -1
}

// QualityIndexerByQualityAndTemplateCheckRegex returns the RegexConfig for the indexer
// in the given QualityConfig that matches the given IndexersConfig by name.
// Returns nil if no match is found.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplateCheckRegex(
	ind *IndexersConfig,
) *RegexConfig {
	if ind == nil {
		return nil
	}

	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return quality.Indexer[index].CfgRegex
		}
	}

	return nil
}

// QualityIndexerByQualityAndTemplateCheckTitle checks if the HistoryCheckTitle field of the
// IndexersConfig that matches the given IndexersConfig by name is true. If no match is found,
// it returns false.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplateCheckTitle(
	ind *IndexersConfig,
) bool {
	if ind == nil {
		return false
	}

	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return quality.Indexer[index].HistoryCheckTitle
		}
	}

	return false
}

// QualityIndexerByQualityAndTemplateSkipEmpty checks if the SkipEmptySize field of the
// IndexersConfig that matches the given IndexersConfig by name is true. If no match is found,
// it returns false.
func (quality *QualityConfig) QualityIndexerByQualityAndTemplateSkipEmpty(
	ind *IndexersConfig,
) bool {
	if ind == nil {
		return false
	}

	for index := range quality.Indexer {
		if quality.Indexer[index].TemplateIndexer == ind.Name ||
			strings.EqualFold(quality.Indexer[index].TemplateIndexer, ind.Name) {
			return quality.Indexer[index].SkipEmptySize
		}
	}

	return false
}

// Getlistbyindexer returns the ListsConfig for the list matching the
// given IndexersConfig name. Returns nil if no match is found.
func (ind *IndexersConfig) Getlistbyindexer() *ListsConfig {
	currentSnapshot := getCurrentConfig()
	if currentSnapshot == nil {
		return nil
	}

	for _, listcfg := range currentSnapshot.List {
		if listcfg.Name == ind.Name || strings.EqualFold(listcfg.Name, ind.Name) {
			return listcfg
		}
	}

	return nil
}

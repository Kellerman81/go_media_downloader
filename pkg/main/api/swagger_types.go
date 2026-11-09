package api

import (
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/trakt"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// Response shapes referenced only by the swag annotations (@Success/@Failure)
// on the REST handlers. Handlers build these bodies via sendJSONResponse /
// sendSuccess / sendJSONError; the types exist so `swag init` can resolve them
// and show realistic examples in Swagger UI.

// Jsondata is the generic success body.
type Jsondata struct {
	Data any `json:"data"`
}

// Jsondataerror is a body carrying both data and an error message.
type Jsondataerror struct {
	Data  any    `json:"data"`
	Error string `json:"error" example:""`
}

// Jsondatarows is a paginated list body.
type Jsondatarows struct {
	Data  any `json:"data"`
	Total int `json:"total" example:"1250"`
}

// Jsonok is the body of actions that finished synchronously.
type Jsonok struct {
	Data string `json:"data" example:"ok"`
}

// Jsonstarted is the body of actions queued as a background job.
type Jsonstarted struct {
	Data string `json:"data" example:"started"`
}

// Success bodies whose text names what was done. One type per message so
// Swagger UI can show the real text as the example.

type Jsonjobstarted struct {
	Data string `json:"data" example:"Job searchmissinginc started"`
}

type Jsonsearchstarted struct {
	Data string `json:"data" example:"Search started for book list Books"`
}

type Jsonauthorsearchmissing struct {
	Data string `json:"data" example:"Author search (missing) started for audiobook list Audiobooks"`
}

type Jsonauthorsearchupgrade struct {
	Data string `json:"data" example:"Author search (upgrade) started for audiobook list Audiobooks"`
}

type Jsonartistsearchmissing struct {
	Data string `json:"data" example:"Artist search (missing) started for music list Music"`
}

type Jsonartistsearchupgrade struct {
	Data string `json:"data" example:"Artist search (upgrade) started for music list Music"`
}

type Jsonfeedsdate struct {
	Data string `json:"data" example:"Feeds date import started for NewReleases (2026-11-01)"`
}

type Jsonauthorqueued struct {
	Data string `json:"data" example:"Author B000APZOQA queued for import into audiobook_Audiobooks/Audiobooks"`
}

type Jsonartistqueued struct {
	Data string `json:"data" example:"Artist 0383dadf-2a4e-4d10-a46a-e9e041da8eb3 queued for import into music_Music/Music"`
}

type Jsondiscoverseries struct {
	Data string `json:"data" example:"Discover series albums queued for artist #42 (3 series found)"`
}

type Jsontraktauthurl struct {
	Data string `json:"data" example:"https://trakt.tv/oauth/authorize?client_id=abc123&redirect_uri=urn%3Aietf%3Awg%3Aoauth%3A2.0%3Aoob&response_type=code&state=state"`
}

type Jsonconfigreloaded struct {
	Data string `json:"data" example:"config reloaded"`
}

type Jsoncacheadded struct {
	Data string `json:"data" example:"Cache entry added successfully"`
}

type Jsoncacheremoved struct {
	Data string `json:"data" example:"Cache entry removed successfully"`
}

type Jsoncachecleared struct {
	Data string `json:"data" example:"Cache type CacheMovie cleared successfully"`
}

// Jsoncount is the body of database writes: the new row ID for an insert,
// otherwise the number of affected rows.
type Jsoncount struct {
	Data int64 `json:"data" example:"1"`
}

// Jsonsuccess is the body of the media delete endpoints.
type Jsonsuccess struct {
	Success bool `json:"success" example:"true"`
}

// Jsonsuccessmessage is the body of a canceled queue job.
type Jsonsuccessmessage struct {
	Success bool   `json:"success" example:"true"`
	Message string `json:"message" example:"Queue job canceled successfully"`
}

// Jsonoperation is the body of the admin table edits.
type Jsonoperation struct {
	Success bool   `json:"success" example:"true"`
	Error   string `json:"error" example:""`
}

// Jsonerror is the generic error body.
type Jsonerror struct {
	Error string `json:"error" example:"error message"`
}

// Jsonunauthorized is the 401 body of every apikey-protected endpoint.
type Jsonunauthorized struct {
	Error string `json:"error" example:"unauthorized - wrong apikey in query"`
}

// Jsonbadrequest is a 400 body.
type Jsonbadrequest struct {
	Error string `json:"error" example:"Invalid id"`
}

// Jsonforbidden is the 403 body of failed database operations.
type Jsonforbidden struct {
	Error string `json:"error" example:"database is locked"`
}

// Jsonnotfound is a 404 body.
type Jsonnotfound struct {
	Error string `json:"error" example:"movie configuration EN not found"`
}

// Jsonconflict is the 409 body of a job that is already queued.
type Jsonconflict struct {
	Error string `json:"error" example:"Job already queued: not queued"`
}

// Jsonservererror is a 500 body.
type Jsonservererror struct {
	Error string `json:"error" example:"Job not started: queue closed"`
}

// The types below carry data from other packages. swag resolves a package
// name in an annotation through the imports of the file the annotation is in;
// when that file does not import the package, it falls back to a lookup whose
// result depends on parse order and fails on some runs. Referencing them here,
// with explicit imports, keeps "make swagger" deterministic.

// Jsonqueue is the data of /api/queue.
type Jsonqueue struct {
	Data map[string]syncops.Job `json:"data"`
}

// Jsonschedules is the data of /api/scheduler/list.
type Jsonschedules struct {
	Data map[string]syncops.JobSchedule `json:"data"`
}

// Jsontraktauthtoken is the body of /api/trakt/token/{code}; data is null when Trakt
// is not set up.
type Jsontraktauthtoken struct {
	Data *apiexternal_v2.OAuthToken `json:"data"`
}

// Jsontraktlistitems is the body of /api/trakt/user/{user}/{list}.
type Jsontraktlistitems struct {
	Data  []trakt.TraktUserListItem `json:"data"`
	Error string                    `json:"error" example:""`
}

// Jsoncachelist is the data of /api/cache/list.
type Jsoncachelist struct {
	AvailableTypes []string          `json:"available_types" example:"CacheMovie,CacheSeries,CacheDBMovie"`
	Description    map[string]string `json:"description"`
}

// Jsonconfigall documents /api/config/all. The real object has one key per
// configuration entry - general, imdb and <section>_<name> - so the keys below
// are examples of each kind. Passwords and API keys are redacted.
type Jsonconfigall struct {
	General      config.GeneralConfig      `json:"general"`
	Imdb         config.ImdbConfig         `json:"imdb"`
	Media        config.MediaTypeConfig    `json:"movie_EN"`
	Downloader   config.DownloaderConfig   `json:"downloader_sabnzbd"`
	Indexer      config.IndexersConfig     `json:"indexer_nzbgeek"`
	List         config.ListsConfig        `json:"list_imdb_top250"`
	Notification config.NotificationConfig `json:"notification_pushover"`
	Path         config.PathsConfig        `json:"path_movies"`
	Quality      config.QualityConfig      `json:"quality_SD"`
	Regex        config.RegexConfig        `json:"regex_standard"`
	Scheduler    config.SchedulerConfig    `json:"scheduler_default"`
}

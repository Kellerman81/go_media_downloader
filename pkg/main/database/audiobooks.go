package database

import (
	"database/sql"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Dbnarrator represents a narrator in the database (metadata table).
type Dbnarrator struct {
	Name      string    `comment:"Primary narrator name"       displayname:"Narrator Name"`
	AudibleID string    `comment:"Audible identifier"          displayname:"Audible ID"     db:"audible_id"`
	Bio       string    `comment:"Narrator biography"          displayname:"Biography"`
	ImageURL  string    `comment:"Narrator image URL"          displayname:"Narrator Image" db:"image_url"`
	CreatedAt time.Time `comment:"Record creation timestamp"   displayname:"Date Created"   db:"created_at"`
	UpdatedAt time.Time `comment:"Last modification timestamp" displayname:"Last Updated"   db:"updated_at"`
	ID        uint      `comment:"Unique narrator identifier"  displayname:"Narrator ID"`
}

// Dbaudiobook represents an audiobook in the database (metadata table).
type Dbaudiobook struct {
	Title          string       `comment:"Primary audiobook title"       displayname:"Audiobook Title"`
	ASIN           string       `comment:"Amazon ASIN identifier"        displayname:"Amazon ASIN"`
	AudibleID      string       `comment:"Audible identifier"            displayname:"Audible ID"      db:"audible_id"`
	Description    string       `comment:"Audiobook description/summary" displayname:"Description"`
	Publisher      string       `comment:"Audiobook publisher"           displayname:"Publisher"`
	Language       string       `comment:"Audiobook language"            displayname:"Language"`
	CoverURL       string       `comment:"Audiobook cover image URL"     displayname:"Cover Image"     db:"cover_url"`
	SampleURL      string       `comment:"Audio sample URL"              displayname:"Sample URL"      db:"sample_url"`
	Slug           string       `comment:"URL friendly identifier"       displayname:"URL Slug"`
	SeriesName     string       `comment:"Audiobook series name"         displayname:"Series Name"     db:"series_name"`
	SeriesPosition string       `comment:"Position in series"            displayname:"Series Position" db:"series_position"`
	ReleaseDate    sql.NullTime `comment:"Audiobook release date"        displayname:"Release Date"    db:"release_date"    json:"release_date" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt      time.Time    `comment:"Record creation timestamp"     displayname:"Date Created"    db:"created_at"`
	UpdatedAt      time.Time    `comment:"Last modification timestamp"   displayname:"Last Updated"    db:"updated_at"`
	RuntimeMinutes int          `comment:"Total runtime in minutes"      displayname:"Runtime Minutes" db:"runtime_minutes"`
	ChapterCount   int          `comment:"Number of chapters"            displayname:"Chapter Count"   db:"chapter_count"`
	ID             uint         `comment:"Unique audiobook identifier"   displayname:"Audiobook ID"`
	DbbookID       uint         `comment:"Related print book reference"  displayname:"Book Reference"  db:"dbbook_id"`
	AverageRating  float32      `comment:"Average user rating"           displayname:"Average Rating"  db:"average_rating"`
	RatingsCount   int32        `comment:"Number of ratings"             displayname:"Ratings Count"   db:"ratings_count"`
	Year           uint16       `comment:"Release year"                  displayname:"Release Year"`
	Abridged       bool         `comment:"Abridged version flag"         displayname:"Is Abridged"`
}

// DbaudiobookTitle represents an alternate title for an audiobook.
type DbaudiobookTitle struct {
	Title         string    `comment:"Alternative audiobook title" displayname:"Alternative Title"`
	Slug          string    `comment:"URL friendly identifier"     displayname:"URL Slug"`
	Region        string    `comment:"Title regional variant"      displayname:"Regional Code"`
	CreatedAt     time.Time `comment:"Record creation timestamp"   displayname:"Date Created"      db:"created_at"`
	UpdatedAt     time.Time `comment:"Last modification timestamp" displayname:"Last Updated"      db:"updated_at"`
	ID            uint      `comment:"Unique title identifier"     displayname:"Title ID"`
	DbaudiobookID uint      `comment:"Parent audiobook reference"  displayname:"Audiobook Ref"     db:"dbaudiobook_id"`
}

// DbaudiobookNarrator represents the many-to-many relationship between audiobooks and narrators.
type DbaudiobookNarrator struct {
	CreatedAt     time.Time `comment:"Record creation timestamp"   db:"created_at"     displayname:"Date Created"`
	UpdatedAt     time.Time `comment:"Last modification timestamp" db:"updated_at"     displayname:"Last Updated"`
	ID            uint      `comment:"Unique relation identifier"                      displayname:"Relation ID"`
	DbaudiobookID uint      `comment:"Audiobook reference"         db:"dbaudiobook_id" displayname:"Audiobook Ref"`
	DbnarratorID  uint      `comment:"Narrator reference"          db:"dbnarrator_id"  displayname:"Narrator Ref"`
	Position      uint8     `comment:"Display order position"                          displayname:"Display Order"`
}

// DbaudiobookAuthor represents the many-to-many relationship between audiobooks and authors.
type DbaudiobookAuthor struct {
	Role          string    `comment:"Role (author, co-author)"    db:"role"           displayname:"Author Role"`
	CreatedAt     time.Time `comment:"Record creation timestamp"   db:"created_at"     displayname:"Date Created"`
	UpdatedAt     time.Time `comment:"Last modification timestamp" db:"updated_at"     displayname:"Last Updated"`
	ID            uint      `comment:"Unique relation identifier"                      displayname:"Relation ID"`
	DbaudiobookID uint      `comment:"Audiobook reference"         db:"dbaudiobook_id" displayname:"Audiobook Ref"`
	DbauthorID    uint      `comment:"Author reference"            db:"dbauthor_id"    displayname:"Author Reference"`
	Position      uint8     `comment:"Display order position"                          displayname:"Display Order"`
}

// DbaudiobookChapter represents chapter metadata for an audiobook.
type DbaudiobookChapter struct {
	Title         string    `comment:"Chapter title"               displayname:"Chapter Title"`
	CreatedAt     time.Time `comment:"Record creation timestamp"   displayname:"Date Created"     db:"created_at"`
	UpdatedAt     time.Time `comment:"Last modification timestamp" displayname:"Last Updated"     db:"updated_at"`
	StartTimeMs   int64     `comment:"Chapter start time (ms)"     displayname:"Start Time"       db:"start_time_ms"`
	EndTimeMs     int64     `comment:"Chapter end time (ms)"       displayname:"End Time"         db:"end_time_ms"`
	RuntimeMs     int64     `comment:"Chapter runtime (ms)"        displayname:"Runtime"          db:"runtime_ms"`
	ID            uint      `comment:"Unique chapter identifier"   displayname:"Chapter ID"`
	DbaudiobookID uint      `comment:"Parent audiobook reference"  displayname:"Audiobook Ref"    db:"dbaudiobook_id"`
	ChapterNumber uint16    `comment:"Chapter sequence number"     displayname:"Chapter Number"   db:"chapter_number"`
	Position      uint16    `comment:"Display order position"      displayname:"Display Position"`
}

// Audiobook represents a tracked audiobook (user tracking table).
type Audiobook struct {
	QualityProfile string       `comment:"Audiobook quality settings"   db:"quality_profile" displayname:"Quality Settings"`
	Listname       string       `comment:"Configuration list name"                           displayname:"Configuration List"`
	Rootpath       string       `comment:"Audiobook storage directory"                       displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"                               displayname:"Last Scanned"`
	CreatedAt      time.Time    `comment:"Record creation timestamp"    db:"created_at"      displayname:"Date Created"`
	UpdatedAt      time.Time    `comment:"Last modification timestamp"  db:"updated_at"      displayname:"Last Updated"`
	ID             uint         `comment:"Unique audiobook identifier"                       displayname:"Audiobook ID"`
	DbaudiobookID  uint         `comment:"Database audiobook reference" db:"dbaudiobook_id"  displayname:"Database Reference"`
	AuthorID       uint         `comment:"Tracked author reference"     db:"author_id"       displayname:"Author Reference"`
	BookSeriesID   uint         `comment:"Book series reference"        db:"book_series_id"  displayname:"Series Reference"`
	Blacklisted    bool         `comment:"Audiobook is blacklisted"                          displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"      db:"quality_reached" displayname:"Quality Target Met"`
	Missing        bool         `comment:"Audiobook is missing"                              displayname:"Is Missing"`
	DontUpgrade    bool         `comment:"Disable quality upgrades"     db:"dont_upgrade"    displayname:"Upgrades Disabled"`
	DontSearch     bool         `comment:"Disable new searches"         db:"dont_search"     displayname:"Search Disabled"`
}

// AudiobookFileUnmatched represents an unmatched audiobook file.
type AudiobookFileUnmatched struct {
	Listname    string       `comment:"Configuration list name"     displayname:"Configuration List"`
	Filepath    string       `comment:"Unmatched file location"     displayname:"File Location"`
	ParsedData  string       `comment:"File parsing results"        displayname:"Parse Results"      db:"parsed_data"`
	LastChecked sql.NullTime `comment:"Last check timestamp"        displayname:"Last Check"         db:"last_checked"`
	CreatedAt   time.Time    `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt   time.Time    `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID          uint         `comment:"Unique record identifier"    displayname:"Record ID"`
}

// ResultAudiobooks combines Dbaudiobook metadata with user tracking data.
type ResultAudiobooks struct {
	Dbaudiobook
	Listname       string       `comment:"Configuration list name"     displayname:"Configuration List"`
	QualityProfile string       `comment:"Audiobook quality settings"  displayname:"Quality Settings"   db:"quality_profile"`
	Rootpath       string       `comment:"Audiobook storage directory" displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"         displayname:"Last Scanned"`
	DbaudiobookID  uint         `comment:"Database audiobook ref"      displayname:"Audiobook Ref"      db:"dbaudiobook_id"`
	Blacklisted    bool         `comment:"Audiobook is blacklisted"    displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"     displayname:"Quality Target Met" db:"quality_reached"`
	Missing        bool         `comment:"Audiobook is missing"        displayname:"Is Missing"`
}

// AudiobookFile represents a downloaded audiobook file.
type AudiobookFile struct {
	Location       string    `comment:"File storage path"                   displayname:"File Path"`
	Filename       string    `comment:"File name only"                      displayname:"File Name"`
	Extension      string    `comment:"File extension type"                 displayname:"File Type"`
	Format         string    `comment:"Audio format (m4b, mp3, flac, etc.)" displayname:"Audio Format"`
	QualityProfile string    `comment:"File quality settings"               displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"           displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"         displayname:"Last Updated"     db:"updated_at"`
	AudiobookID    uint      `comment:"Parent audiobook reference"          displayname:"Parent Audiobook" db:"audiobook_id"`
	DbaudiobookID  uint      `comment:"Database audiobook ref"              displayname:"Audiobook Ref"    db:"dbaudiobook_id"`
	ID             uint      `comment:"Unique file identifier"              displayname:"File ID"`
	FileSize       int64     `comment:"File size in bytes"                  displayname:"File Size"        db:"file_size"`
	Bitrate        int       `comment:"Audio bitrate (kbps)"                displayname:"Audio Bitrate"`
	RuntimeMs      int64     `comment:"File runtime (ms)"                   displayname:"Runtime"          db:"runtime_ms"`
	TrackNumber    uint16    `comment:"Track number for multi-file"         displayname:"Track Number"     db:"track_number"`
	DiscNumber     uint16    `comment:"Disc number for multi-file"          displayname:"Disc Number"      db:"disc_number"`
}

// AudiobookHistory represents download history for audiobooks.
type AudiobookHistory struct {
	Title          string    `comment:"Release title name"            displayname:"Release Title"`
	URL            string    `comment:"Download source URL"           displayname:"Download URL"`
	Indexer        string    `comment:"Source indexer name"           displayname:"Source Indexer"`
	HistoryType    string    `comment:"Audiobook category type"       displayname:"Media Type"       db:"type"`
	Target         string    `comment:"Download target path"          displayname:"Target Path"`
	QualityProfile string    `comment:"Quality settings used"         displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"     displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"   displayname:"Last Updated"     db:"updated_at"`
	DownloadedAt   time.Time `comment:"Download completion timestamp" displayname:"Download Date"    db:"downloaded_at"`
	ID             uint      `comment:"Unique history identifier"     displayname:"History ID"`
	AudiobookID    uint      `comment:"Parent audiobook reference"    displayname:"Parent Audiobook" db:"audiobook_id"`
	DbaudiobookID  uint      `comment:"Database audiobook ref"        displayname:"Audiobook Ref"    db:"dbaudiobook_id"`
	Blacklisted    bool      `comment:"Entry is blacklisted"          displayname:"Is Blacklisted"`
}

// GetDbaudiobookByIDP retrieves a Dbaudiobook by ID.
func (audiobook *Dbaudiobook) GetDbaudiobookByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,title,asin,audible_id,runtime_minutes,chapter_count,release_date,publisher,language,abridged,cover_url,sample_url,average_rating,ratings_count,year,slug,dbbook_id,description from dbaudiobooks where id = ?",
		audiobook,
		id,
	)
}

// GetAudiobooksByIDP retrieves an Audiobook by ID.
func (u *Audiobook) GetAudiobooksByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbaudiobook_id,author_id,book_series_id from audiobooks where id = ?",
		u,
		id,
	)
}

// GetDbnarratorByIDP retrieves a Dbnarrator by ID.
func (narrator *Dbnarrator) GetDbnarratorByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,name,audible_id,bio,image_url from dbnarrators where id = ?",
		narrator,
		id,
	)
}

// AudiobookSearchResult holds the result of searching for an audiobook in the database.
type AudiobookSearchResult struct {
	ID           uint
	Title        string
	ASIN         string
	AudibleID    string
	ChapterCount int
	Runtime      int // Runtime in minutes
	Author       string
	Series       string
	SeriesNum    string
}

// FindAudiobookByTitleAuthor searches for an audiobook by title and author.
// Returns the best matching audiobook or an error if not found.
func FindAudiobookByTitleAuthor(title, author *string) (*AudiobookSearchResult, error) {
	if title == nil || *title == "" {
		return nil, logger.ErrNotFoundDbaudiobook
	}

	// First try exact title match
	var result AudiobookSearchResult

	slug := logger.StringToSlug(*title)

	// Query for exact title or slug match
	Scanrowsdyn(false,
		"SELECT id FROM dbaudiobooks WHERE title = ? OR slug = ? LIMIT 1",
		&result.ID, title, &slug,
	)

	if result.ID > 0 {
		// Fill in title info
		var ab Dbaudiobook
		if err := ab.GetDbaudiobookByIDP(&result.ID); err == nil {
			result.Title = ab.Title
			result.ASIN = ab.ASIN
			result.AudibleID = ab.AudibleID
			result.ChapterCount = ab.ChapterCount
			result.Runtime = ab.RuntimeMinutes
		}

		return &result, nil
	}

	// Try with LIKE pattern match on title
	titlePattern := logger.JoinStrings("%", *title, "%")
	slugPattern := logger.JoinStrings("%", slug, "%")

	Scanrowsdyn(false,
		"SELECT id FROM dbaudiobooks WHERE title LIKE ? OR slug LIKE ? LIMIT 1",
		&result.ID, &titlePattern, &slugPattern)

	if result.ID > 0 {
		var ab Dbaudiobook
		if err := ab.GetDbaudiobookByIDP(&result.ID); err == nil {
			result.Title = ab.Title
			result.ASIN = ab.ASIN
			result.AudibleID = ab.AudibleID
			result.ChapterCount = ab.ChapterCount
			result.Runtime = ab.RuntimeMinutes
		}

		return &result, nil
	}

	return nil, logger.ErrNotFoundDbaudiobook
}

// FindAudiobooksByTitleAuthor returns ALL potential audiobook matches for a title/author combination.
// This allows callers to try multiple matches and pick the best one (e.g., by chapter count).
// Returns up to maxResults matches, ordered by best match quality.
func FindAudiobooksByTitleAuthor(
	title, author *string,
	maxResults int,
) ([]*AudiobookSearchResult, error) {
	if title == nil || *title == "" {
		return nil, logger.ErrNotFoundDbaudiobook
	}

	if maxResults <= 0 {
		maxResults = 10
	}

	var results []*AudiobookSearchResult

	seenIDs := make(map[uint]bool)

	addResult := func(id uint) {
		if id > 0 && !seenIDs[id] {
			seenIDs[id] = true
			results = append(results, fillAudiobookResult(&AudiobookSearchResult{ID: id}))
		}
	}

	remaining := func() uint {
		return uint(
			maxResults - len(results),
		)
	}

	slug := logger.StringToSlug(*title)
	// If we have an author, try author-first lookup (more accurate)
	if author != nil && *author != "" {
		authorSlug := logger.StringToSlug(*author)

		// Find author IDs (search by name or slug)
		authorIDs := Getrowssize[uint](
			false,
			"SELECT count() FROM dbauthors WHERE name = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE",
			"SELECT id FROM dbauthors WHERE name = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE",
			author,
			&authorSlug,
		)
		// For each author, look for albums with matching title
		for i := range authorIDs {
			if len(results) >= maxResults {
				break
			}

			// Find audiobooks by this author with matching title
			ids := GetrowsN[uint](
				false,
				remaining(),
				`SELECT ab.id FROM dbaudiobooks ab
				JOIN dbaudiobook_authors aba ON ab.id = aba.dbaudiobook_id
				WHERE aba.dbauthor_id = ? AND (ab.title = ? COLLATE NOCASE OR ab.slug = ? COLLATE NOCASE)`,
				&authorIDs[i], title, &slug)

			for j := range ids {
				addResult(ids[j])
			}
		}

		// Also try LIKE match with author
		if len(results) < maxResults {
			titlePattern := logger.JoinStrings("%", *title, "%")
			slugPattern := logger.JoinStrings("%", slug, "%")

			for i := range authorIDs {
				if len(results) >= maxResults {
					break
				}

				ids := GetrowsN[uint](
					false,
					remaining(),
					`SELECT ab.id FROM dbaudiobooks ab
					JOIN dbaudiobook_authors aba ON ab.id = aba.dbaudiobook_id
					WHERE aba.dbauthor_id = ? AND (ab.title LIKE ? OR ab.slug LIKE ?)`,
					&authorIDs[i], &titlePattern, &slugPattern)

				for j := range ids {
					addResult(ids[j])
				}
			}
		}
	}

	// Try exact title match without author constraint
	if len(results) < maxResults {
		ids := GetrowsN[uint](
			false,
			remaining(),
			"SELECT id FROM dbaudiobooks WHERE title = ? COLLATE NOCASE OR slug = ? COLLATE NOCASE",
			title, slug)

		for i := range ids {
			addResult(ids[i])
		}
	}

	// Try LIKE pattern match on title
	if len(results) < maxResults {
		titlePattern := logger.JoinStrings("%", *title, "%")
		slugPattern := logger.JoinStrings("%", slug, "%")

		ids := GetrowsN[uint](
			false,
			remaining(),
			"SELECT id FROM dbaudiobooks WHERE title LIKE ? OR slug LIKE ?",
			&titlePattern, &slugPattern)

		for i := range ids {
			addResult(ids[i])
		}
	}

	if len(results) == 0 {
		return nil, logger.ErrNotFoundDbaudiobook
	}

	if len(results) > 1 {
		logger.Logtype("debug", 0).
			Str("title", *title).
			Str("author", *author).
			Int("count", len(results)).
			Msg("DEBUG: FindAudiobooksByTitleAuthor - found multiple potential matches")
	}

	return results, nil
}

// fillAudiobookResult fills in the AudiobookSearchResult with full audiobook data.
func fillAudiobookResult(result *AudiobookSearchResult) *AudiobookSearchResult {
	if result.ID == 0 {
		return result
	}

	var ab Dbaudiobook
	if err := ab.GetDbaudiobookByIDP(&result.ID); err == nil {
		result.Title = ab.Title
		result.ASIN = ab.ASIN
		result.AudibleID = ab.AudibleID
		result.ChapterCount = ab.ChapterCount
		result.Runtime = ab.RuntimeMinutes
		result.Series = ab.SeriesName
		result.SeriesNum = ab.SeriesPosition
	}

	return result
}

// FindAudiobookByASIN searches for an audiobook by ASIN.
func FindAudiobookByASIN(asin string) (*AudiobookSearchResult, error) {
	if asin == "" {
		return nil, logger.ErrNotFoundDbaudiobook
	}

	var result AudiobookSearchResult

	// Query for ASIN or audible_id match
	Scanrowsdyn(false,
		"SELECT id FROM dbaudiobooks WHERE asin = ? OR audible_id = ? LIMIT 1",
		&result.ID, asin, asin)

	if result.ID > 0 {
		var ab Dbaudiobook
		if err := ab.GetDbaudiobookByIDP(&result.ID); err == nil {
			result.Title = ab.Title
			result.ASIN = ab.ASIN
			result.AudibleID = ab.AudibleID
			result.ChapterCount = ab.ChapterCount
			result.Runtime = ab.RuntimeMinutes
		}

		return &result, nil
	}

	return nil, logger.ErrNotFoundDbaudiobook
}

// InsertAudiobookFile records an audiobook file in the database.
func InsertAudiobookFile(
	audiobookID uint,
	filepath string,
	filename string,
	extension string,
	format string,
	qualityProfile string,
	fileSize int64,
	bitrate int,
	runtimeMs int64,
	trackNumber int,
	discNumber int,
	dbaudiobookID uint,
) error {
	if audiobookID == 0 {
		return nil // Skip if no database ID
	}

	// Placeholder - implement actual insert when schema is ready
	// use [location],[filename],[extension],[format],[quality_profile],[file_size],[bitrate],[runtime_ms],[track_number],[disc_number],[audiobook_id],[dbaudiobook_id]
	ExecN(
		"insert into audiobook_files (location, filename, extension, format, quality_profile, file_size, bitrate, runtime_ms, track_number, disc_number, audiobook_id, dbaudiobook_id) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&filepath,
		&filename,
		&extension,
		&format,
		&qualityProfile,
		&fileSize,
		&bitrate,
		&runtimeMs,
		&trackNumber,
		&discNumber,
		&audiobookID,
		&dbaudiobookID,
	)

	return nil
}

// AudiobookExistsByRootpath checks if an audiobook with the given rootpath exists
// and has files in the audiobook_files table.
func AudiobookExistsByRootpath(rootpath *string) bool {
	if rootpath == nil || *rootpath == "" {
		return false
	}

	if SlicesCacheContainsIFast(config.MediaTypeAudiobook, logger.CacheRootpath, rootpath) {
		return true
	}

	return Getdatarow[uint](
		false,
		"SELECT a.id FROM audiobooks a WHERE a.rootpath = ? AND EXISTS (SELECT 1 FROM audiobook_files af WHERE af.audiobook_id = a.id) LIMIT 1",
		rootpath,
	) > 0
}

// GetAudiobookByRootpath returns audiobook info if it exists with the given rootpath.
func GetAudiobookByRootpath(rootpath string) (*Audiobook, error) {
	if rootpath == "" {
		return nil, logger.ErrNotFoundAudiobook
	}

	var ab Audiobook
	Scanrowsdyn(
		false,
		"SELECT id, dbaudiobook_id, listname, rootpath, quality_profile FROM audiobooks WHERE rootpath = ? LIMIT 1",
		&ab.ID,
		&ab.DbaudiobookID,
		&ab.Listname,
		&ab.Rootpath,
		&ab.QualityProfile,
		&rootpath,
	)

	if ab.ID == 0 {
		return nil, logger.ErrNotFoundAudiobook
	}

	return &ab, nil
}

// GetAudiobookListEntryID looks up the audiobooks.id (list entry ID)
// for a given dbaudiobook_id and listname. Returns 0 if not found.
func GetAudiobookListEntryID(dbaudiobookID uint, listname string) uint {
	if dbaudiobookID == 0 || listname == "" {
		return 0
	}

	return Getdatarow[uint](
		false,
		"SELECT id FROM audiobooks WHERE dbaudiobook_id = ? AND listname = ?",
		dbaudiobookID,
		listname,
	)
}

// UpdateAudiobookRootpath updates the rootpath for an audiobook.
// audiobookID is the audiobooks.id (list entry), NOT dbaudiobook_id.
func UpdateAudiobookRootpath(audiobookID uint, rootpath string) error {
	if audiobookID == 0 {
		return nil
	}

	ExecN("UPDATE audiobooks SET rootpath = ? WHERE id = ?", &rootpath, &audiobookID)

	return nil
}

// AudiobookFileExists checks if a file already exists in audiobook_files.
func AudiobookFileExists(filepath *string) bool {
	if filepath == nil || *filepath == "" {
		return false
	}

	if SlicesCacheContainsIFast(config.MediaTypeAudiobook, logger.CacheFiles, filepath) {
		return true
	}

	return Getdatarow[uint](false,
		"SELECT id FROM audiobook_files WHERE location = ? LIMIT 1",
		filepath) > 0
}

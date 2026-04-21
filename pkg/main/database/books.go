package database

import (
	"database/sql"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Dbauthor represents an author in the database (metadata table).
type Dbauthor struct {
	Name          string    `comment:"Primary author name"                   displayname:"Author Name"`
	Aliases       string    `comment:"Alternative author names (JSON array)" displayname:"Alternative Names"`
	Bio           string    `comment:"Author biography"                      displayname:"Biography"`
	GoodreadsID   string    `comment:"Goodreads identifier"                  displayname:"Goodreads ID"      db:"goodreads_id"`
	OpenlibraryID string    `comment:"OpenLibrary identifier"                displayname:"OpenLibrary ID"    db:"openlibrary_id"`
	Website       string    `comment:"Author's website URL"                  displayname:"Author Website"`
	ImageURL      string    `comment:"Author image URL"                      displayname:"Author Image"      db:"image_url"`
	BirthDate     string    `comment:"Author birth date"                     displayname:"Birth Date"        db:"birth_date"`
	DeathDate     string    `comment:"Author death date"                     displayname:"Death Date"        db:"death_date"`
	CreatedAt     time.Time `comment:"Record creation timestamp"             displayname:"Date Created"      db:"created_at"`
	UpdatedAt     time.Time `comment:"Last modification timestamp"           displayname:"Last Updated"      db:"updated_at"`
	ID            uint      `comment:"Unique author identifier"              displayname:"Author ID"`
}

// DbbookSeries represents a book series in the database (metadata table).
type DbbookSeries struct {
	Name          string    `comment:"Book series name"            displayname:"Series Name"`
	Description   string    `comment:"Series description"          displayname:"Series Description"`
	GoodreadsID   string    `comment:"Goodreads identifier"        displayname:"Goodreads ID"       db:"goodreads_id"`
	OpenlibraryID string    `comment:"OpenLibrary identifier"      displayname:"OpenLibrary ID"     db:"openlibrary_id"`
	CreatedAt     time.Time `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt     time.Time `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID            uint      `comment:"Unique series identifier"    displayname:"Series ID"`
}

// Dbbook represents a book in the database (metadata table).
type Dbbook struct {
	Title          string       `comment:"Primary book title"                displayname:"Book Title"`
	OriginalTitle  string       `comment:"Original book title"               displayname:"Original Title"   db:"original_title"`
	ISBN13         string       `comment:"ISBN-13 identifier"                displayname:"ISBN-13"          db:"isbn_13"`
	ISBN10         string       `comment:"ISBN-10 identifier"                displayname:"ISBN-10"          db:"isbn_10"`
	ASIN           string       `comment:"Amazon ASIN identifier"            displayname:"Amazon ASIN"`
	OpenlibraryID  string       `comment:"OpenLibrary identifier"            displayname:"OpenLibrary ID"   db:"openlibrary_id"`
	GoodreadsID    string       `comment:"Goodreads identifier"              displayname:"Goodreads ID"     db:"goodreads_id"`
	Description    string       `comment:"Book description/summary"          displayname:"Book Description"`
	Publisher      string       `comment:"Book publisher"                    displayname:"Publisher"`
	Language       string       `comment:"Book language"                     displayname:"Language"`
	Genres         string       `comment:"Book genres (JSON array)"          displayname:"Genres"`
	CoverURL       string       `comment:"Book cover image URL"              displayname:"Cover Image"      db:"cover_url"`
	Slug           string       `comment:"URL friendly identifier"           displayname:"URL Slug"`
	SeriesPosition string       `comment:"Position in series (e.g., 1, 2.5)" displayname:"Series Position"  db:"series_position"`
	PublishDate    sql.NullTime `comment:"Book publication date"             displayname:"Publish Date"     db:"publish_date"     json:"publish_date" time_format:"2006-01-02" time_utc:"1"`
	CreatedAt      time.Time    `comment:"Record creation timestamp"         displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time    `comment:"Last modification timestamp"       displayname:"Last Updated"     db:"updated_at"`
	PageCount      int          `comment:"Number of pages"                   displayname:"Page Count"       db:"page_count"`
	ID             uint         `comment:"Unique book identifier"            displayname:"Book ID"`
	DbauthorID     uint         `comment:"Primary author reference"          displayname:"Author Reference" db:"dbauthor_id"`
	DbbookSeriesID uint         `comment:"Book series reference"             displayname:"Series Reference" db:"dbbook_series_id"`
	AverageRating  float32      `comment:"Average user rating"               displayname:"Average Rating"   db:"average_rating"`
	RatingsCount   int32        `comment:"Number of ratings"                 displayname:"Ratings Count"    db:"ratings_count"`
	Year           uint16       `comment:"Publication year"                  displayname:"Publication Year"`
}

// DbbookTitle represents an alternate title for a book.
type DbbookTitle struct {
	Title     string    `comment:"Alternative book title"      displayname:"Alternative Title"`
	Slug      string    `comment:"URL friendly identifier"     displayname:"URL Slug"`
	Region    string    `comment:"Title regional variant"      displayname:"Regional Code"`
	CreatedAt time.Time `comment:"Record creation timestamp"   displayname:"Date Created"      db:"created_at"`
	UpdatedAt time.Time `comment:"Last modification timestamp" displayname:"Last Updated"      db:"updated_at"`
	ID        uint      `comment:"Unique title identifier"     displayname:"Title ID"`
	DbbookID  uint      `comment:"Parent book reference"       displayname:"Book Reference"    db:"dbbook_id"`
}

// DbbookAuthor represents the many-to-many relationship between books and authors.
type DbbookAuthor struct {
	Role       string    `comment:"Role (author, editor, translator)" db:"role"        displayname:"Author Role"`
	CreatedAt  time.Time `comment:"Record creation timestamp"         db:"created_at"  displayname:"Date Created"`
	UpdatedAt  time.Time `comment:"Last modification timestamp"       db:"updated_at"  displayname:"Last Updated"`
	ID         uint      `comment:"Unique relation identifier"                         displayname:"Relation ID"`
	DbbookID   uint      `comment:"Book reference"                    db:"dbbook_id"   displayname:"Book Reference"`
	DbauthorID uint      `comment:"Author reference"                  db:"dbauthor_id" displayname:"Author Reference"`
	Position   uint8     `comment:"Display order position"                             displayname:"Display Order"`
}

// Author represents a tracked author (user tracking table).
type Author struct {
	Listname   string    `comment:"Configuration list name"                  displayname:"Configuration List"`
	TrackMode  string    `comment:"Tracking mode (all, series_only, manual)" displayname:"Track Mode"         db:"track_mode"`
	CreatedAt  time.Time `comment:"Record creation timestamp"                displayname:"Date Created"       db:"created_at"`
	UpdatedAt  time.Time `comment:"Last modification timestamp"              displayname:"Last Updated"       db:"updated_at"`
	ID         uint      `comment:"Unique author identifier"                 displayname:"Author ID"`
	DbauthorID uint      `comment:"Database author reference"                displayname:"Database Reference" db:"dbauthor_id"`
	DontSearch bool      `comment:"Disable new searches"                     displayname:"Search Disabled"    db:"dont_search"`
}

// BookSeries represents a tracked book series (user tracking table).
type BookSeries struct {
	Listname       string    `comment:"Configuration list name"     displayname:"Configuration List"`
	CreatedAt      time.Time `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID             uint      `comment:"Unique series identifier"    displayname:"Series ID"`
	DbbookSeriesID uint      `comment:"Database series reference"   displayname:"Database Reference" db:"dbbook_series_id"`
	AuthorID       uint      `comment:"Tracked author reference"    displayname:"Author Reference"   db:"author_id"`
	DontSearch     bool      `comment:"Disable new searches"        displayname:"Search Disabled"    db:"dont_search"`
}

// Book represents a tracked book (user tracking table).
type Book struct {
	QualityProfile string       `comment:"Book quality settings"       db:"quality_profile" displayname:"Quality Settings"`
	Listname       string       `comment:"Configuration list name"                          displayname:"Configuration List"`
	Rootpath       string       `comment:"Book storage directory"                           displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"                              displayname:"Last Scanned"`
	CreatedAt      time.Time    `comment:"Record creation timestamp"   db:"created_at"      displayname:"Date Created"`
	UpdatedAt      time.Time    `comment:"Last modification timestamp" db:"updated_at"      displayname:"Last Updated"`
	ID             uint         `comment:"Unique book identifier"                           displayname:"Book ID"`
	DbbookID       uint         `comment:"Database book reference"     db:"dbbook_id"       displayname:"Database Reference"`
	BookSeriesID   uint         `comment:"Book series reference"       db:"book_series_id"  displayname:"Series Reference"`
	AuthorID       uint         `comment:"Tracked author reference"    db:"author_id"       displayname:"Author Reference"`
	Blacklisted    bool         `comment:"Book is blacklisted"                              displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved"     db:"quality_reached" displayname:"Quality Target Met"`
	Missing        bool         `comment:"Book is missing"                                  displayname:"Is Missing"`
	DontUpgrade    bool         `comment:"Disable quality upgrades"    db:"dont_upgrade"    displayname:"Upgrades Disabled"`
	DontSearch     bool         `comment:"Disable new searches"        db:"dont_search"     displayname:"Search Disabled"`
}

// BookFileUnmatched represents an unmatched book file.
type BookFileUnmatched struct {
	Listname    string       `comment:"Configuration list name"     displayname:"Configuration List"`
	Filepath    string       `comment:"Unmatched file location"     displayname:"File Location"`
	ParsedData  string       `comment:"File parsing results"        displayname:"Parse Results"      db:"parsed_data"`
	LastChecked sql.NullTime `comment:"Last check timestamp"        displayname:"Last Check"         db:"last_checked"`
	CreatedAt   time.Time    `comment:"Record creation timestamp"   displayname:"Date Created"       db:"created_at"`
	UpdatedAt   time.Time    `comment:"Last modification timestamp" displayname:"Last Updated"       db:"updated_at"`
	ID          uint         `comment:"Unique record identifier"    displayname:"Record ID"`
}

// ResultBooks combines Dbbook metadata with user tracking data.
type ResultBooks struct {
	Dbbook
	Listname       string       `comment:"Configuration list name" displayname:"Configuration List"`
	QualityProfile string       `comment:"Book quality settings"   displayname:"Quality Settings"   db:"quality_profile"`
	Rootpath       string       `comment:"Book storage directory"  displayname:"Storage Path"`
	Lastscan       sql.NullTime `comment:"Last scan timestamp"     displayname:"Last Scanned"`
	DbbookID       uint         `comment:"Database book reference" displayname:"Book Reference"     db:"dbbook_id"`
	Blacklisted    bool         `comment:"Book is blacklisted"     displayname:"Is Blacklisted"`
	QualityReached bool         `comment:"Target quality achieved" displayname:"Quality Target Met" db:"quality_reached"`
	Missing        bool         `comment:"Book is missing"         displayname:"Is Missing"`
}

// BookFile represents a downloaded book file.
type BookFile struct {
	Location       string    `comment:"File storage path"                   displayname:"File Path"`
	Filename       string    `comment:"File name only"                      displayname:"File Name"`
	Extension      string    `comment:"File extension type"                 displayname:"File Type"`
	Format         string    `comment:"Book format (epub, pdf, mobi, etc.)" displayname:"Book Format"`
	QualityProfile string    `comment:"File quality settings"               displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"           displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"         displayname:"Last Updated"     db:"updated_at"`
	BookID         uint      `comment:"Parent book reference"               displayname:"Parent Book"      db:"book_id"`
	DbbookID       uint      `comment:"Database book reference"             displayname:"Book Reference"   db:"dbbook_id"`
	ID             uint      `comment:"Unique file identifier"              displayname:"File ID"`
	FileSize       int64     `comment:"File size in bytes"                  displayname:"File Size"        db:"file_size"`
}

// BookHistory represents download history for books.
type BookHistory struct {
	Title          string    `comment:"Release title name"            displayname:"Release Title"`
	URL            string    `comment:"Download source URL"           displayname:"Download URL"`
	Indexer        string    `comment:"Source indexer name"           displayname:"Source Indexer"`
	HistoryType    string    `comment:"Book category type"            displayname:"Media Type"       db:"type"`
	Target         string    `comment:"Download target path"          displayname:"Target Path"`
	QualityProfile string    `comment:"Quality settings used"         displayname:"Quality Settings" db:"quality_profile"`
	CreatedAt      time.Time `comment:"Record creation timestamp"     displayname:"Date Created"     db:"created_at"`
	UpdatedAt      time.Time `comment:"Last modification timestamp"   displayname:"Last Updated"     db:"updated_at"`
	DownloadedAt   time.Time `comment:"Download completion timestamp" displayname:"Download Date"    db:"downloaded_at"`
	ID             uint      `comment:"Unique history identifier"     displayname:"History ID"`
	BookID         uint      `comment:"Parent book reference"         displayname:"Parent Book"      db:"book_id"`
	DbbookID       uint      `comment:"Database book reference"       displayname:"Book Reference"   db:"dbbook_id"`
	Blacklisted    bool      `comment:"Entry is blacklisted"          displayname:"Is Blacklisted"`
}

// GetDbbookByIDP retrieves a Dbbook by ID.
func (book *Dbbook) GetDbbookByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,title,original_title,isbn_13,isbn_10,asin,openlibrary_id,goodreads_id,description,publisher,publish_date,page_count,language,genres,cover_url,dbauthor_id,dbbook_series_id,series_position,average_rating,ratings_count,year,slug from dbbooks where id = ?",
		book,
		id,
	)
}

// GetBooksByIDP retrieves a Book by ID.
func (u *Book) GetBooksByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,blacklisted,quality_reached,quality_profile,missing,dont_upgrade,dont_search,listname,rootpath,dbbook_id,book_series_id,author_id from books where id = ?",
		u,
		id,
	)
}

// GetDbauthorByIDP retrieves a Dbauthor by ID.
func (author *Dbauthor) GetDbauthorByIDP(id *uint) error {
	return structscan1(
		"select id,created_at,updated_at,name,aliases,bio,birth_date,death_date,goodreads_id,openlibrary_id,website,image_url from dbauthors where id = ?",
		author,
		id,
	)
}

// FindBook searches for a book by title, author, and/or ISBN.
// Returns a ResultBooks struct if found, error otherwise.
func FindBook(title, author, isbn string) (*ResultBooks, error) {
	var result ResultBooks

	// Try ISBN first (most accurate)
	if isbn != "" {
		// Check ISBN-13
		Scanrowsdyn(
			false,
			"SELECT b.id, b.dbbook_id, b.listname, b.rootpath, db.title FROM books b JOIN dbbooks db ON b.dbbook_id = db.id WHERE db.isbn_13 = ? LIMIT 1",
			&result.ID,
			&isbn,
		)

		if result.ID > 0 {
			fillResultBook(&result)
			return &result, nil
		}

		// Check ISBN-10
		Scanrowsdyn(
			false,
			"SELECT b.id, b.dbbook_id, b.listname, b.rootpath, db.title FROM books b JOIN dbbooks db ON b.dbbook_id = db.id WHERE db.isbn_10 = ? LIMIT 1",
			&result.ID,
			&isbn,
		)

		if result.ID > 0 {
			fillResultBook(&result)
			return &result, nil
		}

		// Check ASIN
		Scanrowsdyn(
			false,
			"SELECT b.id, b.dbbook_id, b.listname, b.rootpath, db.title FROM books b JOIN dbbooks db ON b.dbbook_id = db.id WHERE db.asin = ? LIMIT 1",
			&result.ID,
			&isbn,
		)

		if result.ID > 0 {
			fillResultBook(&result)
			return &result, nil
		}
	}

	// Try title and author match
	if title != "" && author != "" {
		likeTitle := logger.JoinStrings("%", title, "%")
		likeAuthor := logger.JoinStrings("%", author, "%")
		Scanrowsdyn(
			false,
			"SELECT b.id FROM books b JOIN dbbooks db ON b.dbbook_id = db.id JOIN dbauthors da ON db.dbauthor_id = da.id WHERE db.title LIKE ? AND da.name LIKE ? LIMIT 1",
			&result.ID,
			&likeTitle,
			&likeAuthor,
		)

		if result.ID > 0 {
			fillResultBook(&result)
			return &result, nil
		}
	}

	// Try title only
	if title != "" {
		likeTitle := logger.JoinStrings("%", title, "%")
		Scanrowsdyn(
			false,
			"SELECT b.id FROM books b JOIN dbbooks db ON b.dbbook_id = db.id WHERE db.title LIKE ? LIMIT 1",
			&result.ID,
			&likeTitle,
		)

		if result.ID > 0 {
			fillResultBook(&result)
			return &result, nil
		}
	}

	return nil, logger.ErrNotFoundBook
}

// fillResultBook fills in the book details from the database.
func fillResultBook(result *ResultBooks) {
	if result.ID == 0 {
		return
	}

	var book Book
	if err := book.GetBooksByIDP(&result.ID); err == nil {
		result.Listname = book.Listname
		result.Rootpath = book.Rootpath
		result.DbbookID = book.DbbookID
		result.QualityProfile = book.QualityProfile
		result.Blacklisted = book.Blacklisted
		result.QualityReached = book.QualityReached
		result.Missing = book.Missing
	}

	if result.DbbookID == 0 {
		return
	}

	var dbbook Dbbook

	err := dbbook.GetDbbookByIDP(&result.DbbookID)
	if err != nil {
		return
	}

	result.Title = dbbook.Title
	result.ISBN13 = dbbook.ISBN13
	result.ISBN10 = dbbook.ISBN10
	result.ASIN = dbbook.ASIN
}

// InsertBookFile inserts a book file record into the database.
func InsertBookFile(bookID uint, location, format string) error {
	if bookID == 0 {
		return nil
	}

	var (
		dbbookID       uint
		qualityProfile string
	)

	// Get dbbook_id from book

	Scanrowsdyn(false, "SELECT dbbook_id FROM books WHERE id = ?", &dbbookID, &bookID)

	// Extract filename and extension from location
	filename := location
	extension := format

	if idx := lastIndexByte(location, '/'); idx >= 0 {
		filename = location[idx+1:]
	} else if idx := lastIndexByte(location, '\\'); idx >= 0 {
		filename = location[idx+1:]
	}

	// Get file size
	var fileSize int64
	// File size would be obtained via os.Stat in the caller, but for now use 0

	ExecN(
		"insert into book_files (location, filename, extension, format, quality_profile, book_id, dbbook_id, file_size) values (?, ?, ?, ?, ?, ?, ?, ?)",
		&location,
		&filename,
		&extension,
		&format,
		&qualityProfile,
		&bookID,
		&dbbookID,
		&fileSize,
	)

	return nil
}

// lastIndexByte returns the index of the last occurrence of c in s, or -1 if not found.
func lastIndexByte(s string, c byte) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == c {
			return i
		}
	}

	return -1
}

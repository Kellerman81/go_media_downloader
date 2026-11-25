package api

import (
	"encoding/xml"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// OPDS (/opds) offers the books of the account's media groups to reader
// apps: a navigation feed (groups, recently added, authors) and paged
// acquisition feeds with the book files as downloads.

const (
	opdsNav      = "application/atom+xml;profile=opds-catalog;kind=navigation"
	opdsAcq      = "application/atom+xml;profile=opds-catalog;kind=acquisition"
	opdsPageSize = 50
)

type opdsFeed struct {
	XMLName xml.Name    `xml:"feed"`
	Xmlns   string      `xml:"xmlns,attr"`
	DC      string      `xml:"xmlns:dc,attr"`
	OPDS    string      `xml:"xmlns:opds,attr"`
	OS      string      `xml:"xmlns:opensearch,attr"`
	ID      string      `xml:"id"`
	Title   string      `xml:"title"`
	Updated string      `xml:"updated"`
	Author  *opdsAuthor `xml:"author,omitempty"`
	Links   []opdsLink  `xml:"link"`
	Total   int         `xml:"opensearch:totalResults,omitempty"`
	PerPage int         `xml:"opensearch:itemsPerPage,omitempty"`
	Entries []opdsEntry `xml:"entry"`
}

type opdsAuthor struct {
	Name string `xml:"name"`
}

type opdsLink struct {
	Rel   string `xml:"rel,attr,omitempty"`
	Href  string `xml:"href,attr"`
	Type  string `xml:"type,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}

type opdsContent struct {
	Type string `xml:"type,attr"`
	Text string `xml:",chardata"`
}

type opdsEntry struct {
	ID        string       `xml:"id"`
	Title     string       `xml:"title"`
	Updated   string       `xml:"updated"`
	Authors   []opdsAuthor `xml:"author,omitempty"`
	Language  string       `xml:"dc:language,omitempty"`
	Publisher string       `xml:"dc:publisher,omitempty"`
	Issued    string       `xml:"dc:issued,omitempty"`
	Content   *opdsContent `xml:"content,omitempty"`
	Links     []opdsLink   `xml:"link"`
}

// opdsBook is a book entry with files.
type opdsBook struct {
	ID          uint   `db:"id"`
	DbID        uint   `db:"db_id"`
	Title       string `db:"title"`
	Author      string `db:"author"`
	AuthorID    uint   `db:"author_id"`
	Description string `db:"description"`
	Language    string `db:"language"`
	Publisher   string `db:"publisher"`
	Year        int    `db:"year"`
	Updated     string `db:"updated"`
}

type opdsFile struct {
	ID       uint   `db:"id"`
	BookID   uint   `db:"book_id"`
	Location string `db:"location"`
	Size     int64  `db:"file_size"`
}

const opdsBookSelect = `SELECT e.id, d.id AS db_id, COALESCE(d.title, '') AS title, COALESCE(au.name, '') AS author,
	COALESCE(au.id, 0) AS author_id, COALESCE(d.description, '') AS description, COALESCE(d.language, '') AS language,
	COALESCE(d.publisher, '') AS publisher, COALESCE(d.year, 0) AS year, COALESCE(e.updated_at, e.created_at, '') AS updated
	FROM books e INNER JOIN dbbooks d ON d.id = e.dbbook_id LEFT JOIN dbauthors au ON au.id = d.dbauthor_id
	WHERE EXISTS (SELECT 1 FROM book_files f WHERE f.book_id = e.id) AND e.listname`

func newOPDSFeed(id, title, self, kind string) *opdsFeed {
	return &opdsFeed{
		Xmlns: "http://www.w3.org/2005/Atom", DC: "http://purl.org/dc/terms/", OPDS: "http://opds-spec.org/2010/catalog",
		OS: "http://a9.com/-/spec/opensearch/1.1/", ID: "urn:gmd:opds:" + id, Title: title,
		Updated: time.Now().UTC().Format(time.RFC3339), Author: &opdsAuthor{Name: serverName()},
		Links: []opdsLink{
			{Rel: "self", Href: self, Type: kind},
			{Rel: "start", Href: "/opds", Type: opdsNav},
			{Rel: "search", Href: "/opds/search.xml", Type: "application/opensearchdescription+xml"},
		},
	}
}

func writeOPDS(c *gin.Context, f *opdsFeed, kind string) {
	out, err := xml.MarshalIndent(f, "", " ")
	if err != nil {
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

	c.Data(http.StatusOK, kind+";charset=utf-8", append([]byte(xml.Header), out...))
}

// HandleOPDSRoot is the start feed: the book groups, recently added, the
// authors.
func HandleOPDSRoot(c *gin.Context) {
	lang := requestLanguage(c)
	f := newOPDSFeed("root", serverName(), "/opds", opdsNav)
	now := f.Updated

	nav := func(id, title, href, kind, text string) {
		f.Entries = append(f.Entries, opdsEntry{
			ID: "urn:gmd:opds:" + id, Title: title, Updated: now,
			Content: &opdsContent{Type: "text", Text: text},
			Links:   []opdsLink{{Rel: "subsection", Href: href, Type: kind}},
		})
	}

	nav("new", trText(lang, t("Recently added")), "/opds/new", opdsAcq, "")

	groups := groupsOf(accessOf(c), "book")
	for i := range groups {
		nav("group:"+groups[i].Name, groupTitle(&groups[i], lang), "/opds/group/"+url.PathEscape(groups[i].Name), opdsAcq, "")
	}

	nav("authors", trText(lang, t("Authors")), "/opds/authors", opdsNav, "")

	writeOPDS(c, f, opdsNav)
}

// opdsPage is the page of a request (from 1).
func opdsPage(c *gin.Context) int {
	p, err := strconv.Atoi(c.Query("page"))
	if err != nil || p < 1 {
		return 1
	}

	return p
}

// opdsBooks writes an acquisition feed of the books matching cond (with
// args) in the account's groups.
func opdsBooks(c *gin.Context, id, title, self string, groups []libGroup, cond string, args []any, order string) {
	in, largs := listsOf(groups)
	page := opdsPage(c)

	all := append(largs, args...)
	total := database.Getdatarow[int](false, "SELECT count() FROM ("+opdsBookSelect+in+cond+")", all...)
	books := database.StructscanT[opdsBook](false, opdsPageSize,
		opdsBookSelect+in+cond+" ORDER BY "+order+" LIMIT "+strconv.Itoa(opdsPageSize)+" OFFSET "+strconv.Itoa((page-1)*opdsPageSize),
		all...)

	f := newOPDSFeed(id, title, self, opdsAcq)
	f.Total, f.PerPage = total, opdsPageSize

	pageLink := func(p int) string {
		sep := "?"
		if strings.Contains(self, "?") {
			sep = "&"
		}

		return self + sep + "page=" + strconv.Itoa(p)
	}

	if page > 1 {
		f.Links = append(f.Links, opdsLink{Rel: "previous", Href: pageLink(page - 1), Type: opdsAcq})
	}

	if page*opdsPageSize < total {
		f.Links = append(f.Links, opdsLink{Rel: "next", Href: pageLink(page + 1), Type: opdsAcq})
	}

	files := opdsFilesOf(books)
	for i := range books {
		f.Entries = append(f.Entries, opdsBookEntry(&books[i], files[books[i].ID]))
	}

	writeOPDS(c, f, opdsAcq)
}

// opdsFilesOf returns the files of the books by book entry.
func opdsFilesOf(books []opdsBook) map[uint][]opdsFile {
	out := map[uint][]opdsFile{}
	if len(books) == 0 {
		return out
	}

	args := make([]any, len(books))
	for i := range books {
		args[i] = books[i].ID
	}

	rows := database.StructscanT[opdsFile](false, uint(len(books)), //nolint:gosec // a page of books
		"SELECT id, book_id, location, COALESCE(file_size, 0) AS file_size FROM book_files WHERE book_id IN (?"+
			strings.Repeat(", ?", len(books)-1)+") ORDER BY id", args...)

	for _, r := range rows {
		out[r.BookID] = append(out[r.BookID], r)
	}

	return out
}

func opdsBookEntry(b *opdsBook, files []opdsFile) opdsEntry {
	e := opdsEntry{
		ID: "urn:gmd:book:" + strconv.FormatUint(uint64(b.ID), 10), Title: b.Title, Updated: opdsTime(b.Updated),
		Language: b.Language, Publisher: b.Publisher,
	}

	if e.Title == "" && len(files) > 0 {
		e.Title = fileTitle(files[0].Location)
	}

	if b.Author != "" {
		e.Authors = []opdsAuthor{{Name: b.Author}}
	}

	if b.Year > 0 {
		e.Issued = strconv.Itoa(b.Year)
	}

	if b.Description != "" {
		e.Content = &opdsContent{Type: "text", Text: b.Description}
	}

	cover := "/opds/cover/" + strconv.FormatUint(uint64(b.DbID), 10)
	e.Links = append(e.Links,
		opdsLink{Rel: "http://opds-spec.org/image", Href: cover + "?size=full", Type: "image/jpeg"},
		opdsLink{Rel: "http://opds-spec.org/image/thumbnail", Href: cover, Type: "image/jpeg"})

	if b.AuthorID > 0 {
		e.Links = append(e.Links, opdsLink{
			Rel: "related", Href: "/opds/author/" + strconv.FormatUint(uint64(b.AuthorID), 10), Type: opdsAcq, Title: b.Author,
		})
	}

	for _, f := range files {
		ct := mediaContentType(f.Location)
		if ct == "" {
			ct = "application/octet-stream"
		}

		name := f.Location[strings.LastIndexAny(f.Location, `/\`)+1:]
		e.Links = append(e.Links, opdsLink{
			Rel: "http://opds-spec.org/acquisition", Type: ct, Title: name,
			Href: "/opds/file/" + strconv.FormatUint(uint64(f.ID), 10) + "/" + url.PathEscape(name),
		})
	}

	return e
}

// opdsTime turns a database time into RFC 3339 (now when unreadable).
func opdsTime(s string) string {
	for _, layout := range []string{"2006-01-02 15:04:05.999999999-07:00", "2006-01-02 15:04:05", "2006-01-02 15:04", time.RFC3339Nano, "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}

	return time.Now().UTC().Format(time.RFC3339)
}

// HandleOPDSGroup lists the books of a media group (/opds/group/:name).
func HandleOPDSGroup(c *gin.Context) {
	for _, g := range groupsOf(accessOf(c), "book") {
		if g.Name == c.Param("name") {
			opdsBooks(c, "group:"+g.Name, groupTitle(&g, requestLanguage(c)), "/opds/group/"+url.PathEscape(g.Name),
				[]libGroup{g}, "", nil, "title COLLATE NOCASE")

			return
		}
	}

	c.AbortWithStatus(http.StatusNotFound)
}

// HandleOPDSNew lists the books added last.
func HandleOPDSNew(c *gin.Context) {
	opdsBooks(c, "new", trText(requestLanguage(c), t("Recently added")), "/opds/new", groupsOf(accessOf(c), "book"),
		"", nil, "e.id DESC")
}

// HandleOPDSAuthor lists the books of an author (/opds/author/:id).
func HandleOPDSAuthor(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	name := database.Getdatarow[string](false, "SELECT COALESCE(name, '') FROM dbauthors WHERE id = ?", &id)
	opdsBooks(c, "author:"+c.Param("id"), name, "/opds/author/"+c.Param("id"), groupsOf(accessOf(c), "book"),
		" AND d.dbauthor_id = ?", []any{id}, "d.year, title COLLATE NOCASE")
}

// HandleOPDSSearch finds books by title or author (/opds/search?q=).
func HandleOPDSSearch(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	like := "%" + q + "%"

	opdsBooks(c, "search", q, "/opds/search?q="+url.QueryEscape(q), groupsOf(accessOf(c), "book"),
		" AND (d.title LIKE ? OR au.name LIKE ?)", []any{like, like}, "title COLLATE NOCASE")
}

// HandleOPDSAuthors lists the authors with books, by name.
func HandleOPDSAuthors(c *gin.Context) {
	in, args := listsOf(groupsOf(accessOf(c), "book"))

	authors := database.StructscanT[struct {
		ID    uint   `db:"id"`
		Name  string `db:"name"`
		Books int    `db:"books"`
	}](false, 256, "SELECT au.id, au.name, count() AS books FROM books e INNER JOIN dbbooks d ON d.id = e.dbbook_id "+
		"INNER JOIN dbauthors au ON au.id = d.dbauthor_id WHERE EXISTS (SELECT 1 FROM book_files f WHERE f.book_id = e.id) "+
		"AND e.listname"+in+" GROUP BY au.id ORDER BY au.name COLLATE NOCASE", args...)

	lang := requestLanguage(c)
	f := newOPDSFeed("authors", trText(lang, t("Authors")), "/opds/authors", opdsNav)

	for _, a := range authors {
		f.Entries = append(f.Entries, opdsEntry{
			ID: "urn:gmd:author:" + strconv.FormatUint(uint64(a.ID), 10), Title: a.Name, Updated: f.Updated,
			Content: &opdsContent{Type: "text", Text: trText(lang, tf("%d books", a.Books))},
			Links:   []opdsLink{{Rel: "subsection", Href: "/opds/author/" + strconv.FormatUint(uint64(a.ID), 10), Type: opdsAcq}},
		})
	}

	writeOPDS(c, f, opdsNav)
}

// HandleOPDSSearchDescription is the OpenSearch description of the search.
func HandleOPDSSearchDescription(c *gin.Context) {
	c.Data(http.StatusOK, "application/opensearchdescription+xml;charset=utf-8", []byte(xml.Header+
		`<OpenSearchDescription xmlns="http://a9.com/-/spec/opensearch/1.1/"><ShortName>`+xmlEscape(serverName())+
		`</ShortName><Description>Books</Description><InputEncoding>UTF-8</InputEncoding><OutputEncoding>UTF-8</OutputEncoding>`+
		`<Url type="`+opdsAcq+`" template="/opds/search?q={searchTerms}"/></OpenSearchDescription>`))
}

// HandleOPDSCover serves a book cover (/opds/cover/:id, a dbbooks ID).
func HandleOPDSCover(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || !accessOf(c).allowsDB(libKindByType("book"), &id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	serveCover(c, "book", id, c.Query("size") == "full")
}

// serveCover sends the cover of a db* row (thumbnail unless full); 404
// without one, as apps don't show SVG placeholders.
func serveCover(c *gin.Context, typ string, id uint64, full bool) {
	p, err := libraryCoverFile(typ, id)
	if err != nil {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if !full {
		p = coverThumb(p)
	}

	c.Header("Cache-Control", "public, max-age=604800")
	c.File(p)
}

// HandleOPDSFile downloads a book file (/opds/file/:id/:name).
func HandleOPDSFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || !accessOf(c).allowsFile(libKindByType("book"), &id) {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	loc := database.Getdatarow[string](false, "SELECT location FROM book_files WHERE id = ?", &id)
	if loc == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if ct := mediaContentType(loc); ct != "" {
		c.Header("Content-Type", ct)
	}

	c.FileAttachment(loc, loc[strings.LastIndexAny(loc, `/\`)+1:])
}

// xmlEscape escapes a text for XML.
func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))

	return b.String()
}

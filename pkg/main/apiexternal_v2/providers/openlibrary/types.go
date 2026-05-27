package openlibrary

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// OpenLibrary Internal Types - Used for JSON unmarshaling
//

// olSearchResponse represents the search API response.
type olSearchResponse struct {
	NumFound      int           `json:"numFound"`
	Start         int           `json:"start"`
	NumFoundExact bool          `json:"numFoundExact"`
	Docs          []olSearchDoc `json:"docs"`
}

// olSearchDoc represents a single search result document.
type olSearchDoc struct {
	Key                 string   `json:"key"`
	Title               string   `json:"title"`
	Subtitle            string   `json:"subtitle"`
	AuthorName          []string `json:"author_name"`
	AuthorKey           []string `json:"author_key"`
	FirstPublishYear    int      `json:"first_publish_year"`
	PublishYear         []int    `json:"publish_year"`
	ISBN                []string `json:"isbn"`
	CoverI              int      `json:"cover_i"`
	CoverEditionKey     string   `json:"cover_edition_key"`
	EditionCount        int      `json:"edition_count"`
	Subject             []string `json:"subject"`
	Language            []string `json:"language"`
	Publisher           []string `json:"publisher"`
	NumberOfPagesMedian int      `json:"number_of_pages_median"`
	LCCN                []string `json:"lccn"`
	OCLC                []string `json:"oclc"`
	FirstSentence       []string `json:"first_sentence"`
}

// olWorkResponse represents a work details response.
type olWorkResponse struct {
	Key         string   `json:"key"`
	Title       string   `json:"title"`
	Subtitle    string   `json:"subtitle"`
	Description any      `json:"description"` // Can be string or {type, value}
	Subjects    []string `json:"subjects"`
	Authors     []struct {
		Author struct {
			Key string `json:"key"`
		} `json:"author"`
		Type struct {
			Key string `json:"key"`
		} `json:"type"`
	} `json:"authors"`
	Covers        []int    `json:"covers"`
	SubjectPlaces []string `json:"subject_places"`
	SubjectPeople []string `json:"subject_people"`
	SubjectTimes  []string `json:"subject_times"`
	Created       olTime   `json:"created"`
	LastModified  olTime   `json:"last_modified"`
}

// olEditionResponse represents an edition/book details response.
type olEditionResponse struct {
	Key             string   `json:"key"`
	Title           string   `json:"title"`
	Subtitle        string   `json:"subtitle"`
	Description     any      `json:"description"`
	Authors         []olRef  `json:"authors"`
	Works           []olRef  `json:"works"`
	ISBN10          []string `json:"isbn_10"`
	ISBN13          []string `json:"isbn_13"`
	LCCN            []string `json:"lccn"`
	OCLC            []string `json:"oclc_numbers"`
	Publishers      []string `json:"publishers"`
	PublishDate     string   `json:"publish_date"`
	PublishPlaces   []string `json:"publish_places"`
	NumberOfPages   int      `json:"number_of_pages"`
	Covers          []int    `json:"covers"`
	Languages       []olRef  `json:"languages"`
	Subjects        []string `json:"subjects"`
	PhysicalFormat  string   `json:"physical_format"`
	TableOfContents []olTOC  `json:"table_of_contents"`
}

// olAuthorResponse represents an author details response.
type olAuthorResponse struct {
	Key            string      `json:"key"`
	Name           string      `json:"name"`
	PersonalName   string      `json:"personal_name"`
	AlternateNames []string    `json:"alternate_names"`
	Bio            any         `json:"bio"` // Can be string or {type, value}
	BirthDate      string      `json:"birth_date"`
	DeathDate      string      `json:"death_date"`
	Photos         []int       `json:"photos"`
	Links          []olLink    `json:"links"`
	RemoteIDs      olRemoteIDs `json:"remote_ids"`
	Wikipedia      string      `json:"wikipedia"`
}

// olAuthorWorksResponse represents the response from author works endpoint.
type olAuthorWorksResponse struct {
	Links   olLinks       `json:"links"`
	Size    int           `json:"size"`
	Entries []olWorkEntry `json:"entries"`
}

// olWorkEntry represents a single work in author's works list.
type olWorkEntry struct {
	Key      string   `json:"key"`
	Title    string   `json:"title"`
	Subtitle string   `json:"subtitle"`
	Covers   []int    `json:"covers"`
	Authors  []olRef  `json:"authors"`
	Subjects []string `json:"subjects"`
}

// olEditionsResponse represents the response from work editions endpoint.
type olEditionsResponse struct {
	Links   olLinks          `json:"links"`
	Size    int              `json:"size"`
	Entries []olEditionEntry `json:"entries"`
}

// olEditionEntry represents a single edition.
type olEditionEntry struct {
	Key           string   `json:"key"`
	Title         string   `json:"title"`
	Publishers    []string `json:"publishers"`
	PublishDate   string   `json:"publish_date"`
	ISBN10        []string `json:"isbn_10"`
	ISBN13        []string `json:"isbn_13"`
	Covers        []int    `json:"covers"`
	NumberOfPages int      `json:"number_of_pages"`
	Languages     []olRef  `json:"languages"`
}

// Helper types.
type olRef struct {
	Key string `json:"key"`
}

type olLink struct {
	URL   string `json:"url"`
	Title string `json:"title"`
	Type  olRef  `json:"type"`
}

type olLinks struct {
	Self string `json:"self"`
	Next string `json:"next"`
}

type olRemoteIDs struct {
	VIAF      string `json:"viaf"`
	ISNI      string `json:"isni"`
	Wikidata  string `json:"wikidata"`
	Goodreads string `json:"goodreads"`
}

type olTOC struct {
	Level int    `json:"level"`
	Title string `json:"title"`
}

type olTime struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

//
// Conversion Functions
//

func convertSearchResults(docs []olSearchDoc) []apiexternal_v2.BookSearchResult {
	results := make([]apiexternal_v2.BookSearchResult, 0, len(docs))

	for i := range docs {
		result := apiexternal_v2.BookSearchResult{
			ID:           docs[i].Key,
			Title:        docs[i].Title,
			Subtitle:     docs[i].Subtitle,
			Authors:      docs[i].AuthorName,
			PublishYear:  docs[i].FirstPublishYear,
			ProviderType: apiexternal_v2.ProviderOpenLibrary,
		}

		// Get ISBNs
		if len(docs[i].ISBN) > 0 {
			for j := range docs[i].ISBN {
				if len(docs[i].ISBN[j]) == 13 && result.ISBN13 == "" {
					result.ISBN13 = docs[i].ISBN[j]
				} else if len(docs[i].ISBN[j]) == 10 && result.ISBN10 == "" {
					result.ISBN10 = docs[i].ISBN[j]
				}
			}
		}

		// Get cover URL
		if docs[i].CoverI > 0 {
			result.CoverURL = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(
				docs[i].CoverI,
			) + "-M.jpg"
		}

		results = append(results, result)
	}

	return results
}

func convertWorkToBookDetails(work *olWorkResponse) *apiexternal_v2.BookDetails {
	details := &apiexternal_v2.BookDetails{
		ID:              work.Key,
		Title:           work.Title,
		Subtitle:        work.Subtitle,
		Description:     extractTextValue(work.Description),
		Subjects:        work.Subjects,
		OpenLibraryWork: extractOLID(work.Key),
		ProviderType:    apiexternal_v2.ProviderOpenLibrary,
	}

	// Get cover URL from first cover ID
	if len(work.Covers) > 0 && work.Covers[0] > 0 {
		details.CoverURL = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(
			work.Covers[0],
		) + "-L.jpg"
	}

	return details
}

func convertEditionToBookDetails(edition *olEditionResponse) *apiexternal_v2.BookDetails {
	details := &apiexternal_v2.BookDetails{
		ID:            edition.Key,
		Title:         edition.Title,
		Subtitle:      edition.Subtitle,
		Description:   extractTextValue(edition.Description),
		PageCount:     edition.NumberOfPages,
		Subjects:      edition.Subjects,
		OpenLibraryID: extractOLID(edition.Key),
		ProviderType:  apiexternal_v2.ProviderOpenLibrary,
	}

	// ISBNs
	if len(edition.ISBN13) > 0 {
		details.ISBN13 = edition.ISBN13[0]
	}

	if len(edition.ISBN10) > 0 {
		details.ISBN10 = edition.ISBN10[0]
	}

	// LCCNs and OCLCs
	if len(edition.LCCN) > 0 {
		details.LCCN = edition.LCCN[0]
	}

	if len(edition.OCLC) > 0 {
		details.OCLC = edition.OCLC[0]
	}

	// Publisher
	if len(edition.Publishers) > 0 {
		details.Publisher = edition.Publishers[0]
	}

	// Publish date
	if edition.PublishDate != "" {
		details.PublishDate = parseOLDate(edition.PublishDate)
		if year := details.PublishDate.Year(); year > 0 {
			details.PublishYear = year
		}
	}

	// Cover URL
	if len(edition.Covers) > 0 && edition.Covers[0] > 0 {
		details.CoverURL = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(
			edition.Covers[0],
		) + "-L.jpg"
	}

	// Languages
	if len(edition.Languages) > 0 {
		details.Language = extractOLID(edition.Languages[0].Key)
	}

	// Table of contents
	if len(edition.TableOfContents) > 0 {
		toc := make([]string, 0, len(edition.TableOfContents))
		for i := range edition.TableOfContents {
			toc = append(toc, edition.TableOfContents[i].Title)
		}

		details.TableOfContents = toc
	}

	// Work ID
	if len(edition.Works) > 0 {
		details.OpenLibraryWork = extractOLID(edition.Works[0].Key)
	}

	return details
}

func convertToAuthorDetails(author *olAuthorResponse) *apiexternal_v2.AuthorDetails {
	details := &apiexternal_v2.AuthorDetails{
		ID:            author.Key,
		Name:          author.Name,
		Bio:           extractTextValue(author.Bio),
		BirthDate:     parseOLDate(author.BirthDate),
		DeathDate:     parseOLDate(author.DeathDate),
		Wikipedia:     author.Wikipedia,
		OpenLibraryID: extractOLID(author.Key),
		ProviderType:  apiexternal_v2.ProviderOpenLibrary,
	}

	// Remote IDs
	if author.RemoteIDs.VIAF != "" {
		details.VIAF = author.RemoteIDs.VIAF
	}

	if author.RemoteIDs.ISNI != "" {
		details.ISNI = author.RemoteIDs.ISNI
	}

	if author.RemoteIDs.Goodreads != "" {
		details.GoodreadsID = author.RemoteIDs.Goodreads
	}

	// Photo URL
	if len(author.Photos) > 0 && author.Photos[0] > 0 {
		details.ImageURL = "https://covers.openlibrary.org/a/id/" + strconv.Itoa(
			author.Photos[0],
		) + "-L.jpg"
	}

	// Website from links
	for i := range author.Links {
		if logger.ContainsI(author.Links[i].Title, "website") ||
			logger.ContainsI(author.Links[i].Title, "official") {
			details.Website = author.Links[i].URL
			break
		}
	}

	return details
}

func convertAuthorWorksToSearchResults(entries []olWorkEntry) []apiexternal_v2.BookSearchResult {
	results := make([]apiexternal_v2.BookSearchResult, 0, len(entries))

	for i := range entries {
		result := apiexternal_v2.BookSearchResult{
			ID:           entries[i].Key,
			Title:        entries[i].Title,
			Subtitle:     entries[i].Subtitle,
			ProviderType: apiexternal_v2.ProviderOpenLibrary,
		}

		// Cover URL
		if len(entries[i].Covers) > 0 && entries[i].Covers[0] > 0 {
			result.CoverURL = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(
				entries[i].Covers[0],
			) + "-M.jpg"
		}

		results = append(results, result)
	}

	return results
}

func convertEditionsToSearchResults(entries []olEditionEntry) []apiexternal_v2.BookSearchResult {
	results := make([]apiexternal_v2.BookSearchResult, 0, len(entries))

	for i := range entries {
		result := apiexternal_v2.BookSearchResult{
			ID:           entries[i].Key,
			Title:        entries[i].Title,
			ProviderType: apiexternal_v2.ProviderOpenLibrary,
		}

		// ISBNs
		if len(entries[i].ISBN13) > 0 {
			result.ISBN13 = entries[i].ISBN13[0]
		}

		if len(entries[i].ISBN10) > 0 {
			result.ISBN10 = entries[i].ISBN10[0]
		}

		// Cover URL
		if len(entries[i].Covers) > 0 && entries[i].Covers[0] > 0 {
			result.CoverURL = "https://covers.openlibrary.org/b/id/" + strconv.Itoa(
				entries[i].Covers[0],
			) + "-M.jpg"
		}

		// Parse publish date for year
		if entries[i].PublishDate != "" {
			if t := parseOLDate(entries[i].PublishDate); !t.IsZero() {
				result.PublishYear = t.Year()
			}
		}

		results = append(results, result)
	}

	return results
}

//
// Helper Functions
//

// extractTextValue extracts text from OpenLibrary's description format.
// Descriptions can be either a string or {"type": "/type/text", "value": "..."}.
func extractTextValue(v any) string {
	if v == nil {
		return ""
	}

	switch val := v.(type) {
	case string:
		return val
	case map[string]any:
		if value, ok := val["value"].(string); ok {
			return value
		}
	}

	return ""
}

// extractOLID extracts the OpenLibrary ID from a key path.
// e.g., "/works/OL123W" -> "OL123W".
func extractOLID(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return key
}

// parseOLDate attempts to parse various date formats used by OpenLibrary.
func parseOLDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	// Try various formats
	layouts := []string{
		"2006-01-02",
		"January 2, 2006",
		"January 2006",
		"2006",
		"Jan 2, 2006",
		"02 Jan 2006",
	}

	for i := range layouts {
		if t, err := time.Parse(layouts[i], dateStr); err == nil {
			return t
		}
	}

	// Try to extract year only
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil && year > 0 && year < 3000 {
			return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		}
	}

	return time.Time{}
}

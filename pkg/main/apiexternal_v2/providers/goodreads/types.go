package goodreads

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

//
// Goodreads Internal Types - Used for XML unmarshaling
//

// grSearchResponse represents the search API response.
type grSearchResponse struct {
	Search grSearch `xml:"search"`
}

type grSearch struct {
	Query        string    `xml:"query"`
	ResultsStart int       `xml:"results-start"`
	ResultsEnd   int       `xml:"results-end"`
	TotalResults int       `xml:"total-results"`
	Results      grResults `xml:"results"`
}

type grResults struct {
	Work []grWorkResult `xml:"work"`
}

type grWorkResult struct {
	ID                   int        `xml:"id"`
	BooksCount           int        `xml:"books_count"`
	RatingsCount         int        `xml:"ratings_count"`
	TextReviewsCount     int        `xml:"text_reviews_count"`
	OriginalPublishYear  int        `xml:"original_publication_year"`
	OriginalPublishMonth int        `xml:"original_publication_month"`
	OriginalPublishDay   int        `xml:"original_publication_day"`
	AverageRating        float64    `xml:"average_rating"`
	BestBook             grBestBook `xml:"best_book"`
}

type grBestBook struct {
	ID            int           `xml:"id"`
	Title         string        `xml:"title"`
	Author        grAuthorShort `xml:"author"`
	ImageURL      string        `xml:"image_url"`
	SmallImageURL string        `xml:"small_image_url"`
}

type grAuthorShort struct {
	ID   int    `xml:"id"`
	Name string `xml:"name"`
}

// grBookResponse represents the book details API response.
type grBookResponse struct {
	Book grBook `xml:"book"`
}

type grBook struct {
	ID                 int            `xml:"id"`
	Title              string         `xml:"title"`
	TitleWithoutSeries string         `xml:"title_without_series"`
	ISBN               string         `xml:"isbn"`
	ISBN13             string         `xml:"isbn13"`
	ASIN               string         `xml:"asin"`
	ImageURL           string         `xml:"image_url"`
	SmallImageURL      string         `xml:"small_image_url"`
	PublicationYear    int            `xml:"publication_year"`
	PublicationMonth   int            `xml:"publication_month"`
	PublicationDay     int            `xml:"publication_day"`
	Publisher          string         `xml:"publisher"`
	LanguageCode       string         `xml:"language_code"`
	IsEbook            bool           `xml:"is_ebook"`
	Description        string         `xml:"description"`
	WorkID             int            `xml:"work>id"`
	AverageRating      float64        `xml:"average_rating"`
	RatingsCount       int            `xml:"ratings_count"`
	TextReviewsCount   int            `xml:"text_reviews_count"`
	NumPages           int            `xml:"num_pages"`
	Authors            grAuthors      `xml:"authors"`
	SeriesWorks        grSeriesWorks  `xml:"series_works"`
	SimilarBooks       grSimilarBooks `xml:"similar_books"`
}

type grAuthors struct {
	Author []grAuthorInBook `xml:"author"`
}

type grAuthorInBook struct {
	ID               int     `xml:"id"`
	Name             string  `xml:"name"`
	Role             string  `xml:"role"`
	ImageURL         string  `xml:"image_url"`
	SmallImageURL    string  `xml:"small_image_url"`
	Link             string  `xml:"link"`
	AverageRating    float64 `xml:"average_rating"`
	RatingsCount     int     `xml:"ratings_count"`
	TextReviewsCount int     `xml:"text_reviews_count"`
}

type grSeriesWorks struct {
	SeriesWork []grSeriesWork `xml:"series_work"`
}

type grSeriesWork struct {
	ID           int      `xml:"id"`
	UserPosition string   `xml:"user_position"`
	Series       grSeries `xml:"series"`
}

type grSeries struct {
	ID                int    `xml:"id"`
	Title             string `xml:"title"`
	Description       string `xml:"description"`
	Note              string `xml:"note"`
	SeriesWorksCount  int    `xml:"series_works_count"`
	PrimaryWorkCount  int    `xml:"primary_work_count"`
	NumberedBookCount int    `xml:"numbered_book_count"`
}

type grSimilarBooks struct {
	Book []grBookShort `xml:"book"`
}

type grBookShort struct {
	ID            int       `xml:"id"`
	Title         string    `xml:"title"`
	ImageURL      string    `xml:"image_url"`
	SmallImageURL string    `xml:"small_image_url"`
	Authors       grAuthors `xml:"authors"`
}

// grAuthorResponse represents the author details API response.
type grAuthorResponse struct {
	Author grAuthorFull `xml:"author"`
}

type grAuthorFull struct {
	ID              int    `xml:"id"`
	Name            string `xml:"name"`
	Link            string `xml:"link"`
	FansCount       int    `xml:"fans_count"`
	ImageURL        string `xml:"image_url"`
	SmallImageURL   string `xml:"small_image_url"`
	About           string `xml:"about"`
	WorksCount      int    `xml:"works_count"`
	Gender          string `xml:"gender"`
	Hometown        string `xml:"hometown"`
	BornAt          string `xml:"born_at"`
	DiedAt          string `xml:"died_at"`
	GoodreadsAuthor bool   `xml:"goodreads_author"`
}

// grAuthorBooksResponse represents the author books API response.
type grAuthorBooksResponse struct {
	Author grAuthorWithBooks `xml:"author"`
}

type grAuthorWithBooks struct {
	ID    int         `xml:"id"`
	Name  string      `xml:"name"`
	Books grBooksList `xml:"books"`
}

type grBooksList struct {
	Start int          `xml:"start,attr"`
	End   int          `xml:"end,attr"`
	Total int          `xml:"total,attr"`
	Book  []grBookItem `xml:"book"`
}

type grBookItem struct {
	ID                 int     `xml:"id"`
	ISBN               string  `xml:"isbn"`
	ISBN13             string  `xml:"isbn13"`
	Title              string  `xml:"title"`
	TitleWithoutSeries string  `xml:"title_without_series"`
	ImageURL           string  `xml:"image_url"`
	SmallImageURL      string  `xml:"small_image_url"`
	Link               string  `xml:"link"`
	NumPages           int     `xml:"num_pages"`
	Format             string  `xml:"format"`
	PublicationYear    int     `xml:"publication_year"`
	AverageRating      float64 `xml:"average_rating"`
	RatingsCount       int     `xml:"ratings_count"`
}

// grSeriesResponse represents the series API response.
type grSeriesResponse struct {
	Series grSeriesFull `xml:"series"`
}

type grSeriesFull struct {
	ID               int              `xml:"id"`
	Title            string           `xml:"title"`
	Description      string           `xml:"description"`
	Note             string           `xml:"note"`
	SeriesWorksCount int              `xml:"series_works_count"`
	PrimaryWorkCount int              `xml:"primary_work_count"`
	SeriesWorks      grSeriesWorkList `xml:"series_works"`
}

type grSeriesWorkList struct {
	SeriesWork []grSeriesWorkItem `xml:"series_work"`
}

type grSeriesWorkItem struct {
	ID           int    `xml:"id"`
	UserPosition string `xml:"user_position"`
	Work         grWork `xml:"work"`
}

type grWork struct {
	ID       int        `xml:"id"`
	BestBook grBestBook `xml:"best_book"`
}

//
// Conversion Functions
//

func convertSearchResults(works []grWorkResult) []apiexternal_v2.BookSearchResult {
	results := make([]apiexternal_v2.BookSearchResult, 0, len(works))

	for i := range works {
		result := apiexternal_v2.BookSearchResult{
			ID:           strconv.Itoa(works[i].BestBook.ID),
			Title:        works[i].BestBook.Title,
			Authors:      []string{works[i].BestBook.Author.Name},
			CoverURL:     works[i].BestBook.ImageURL,
			PublishYear:  works[i].OriginalPublishYear,
			ProviderType: apiexternal_v2.ProviderGoodreads,
		}

		results = append(results, result)
	}

	return results
}

func convertBookToDetails(book *grBook) *apiexternal_v2.BookDetails {
	details := &apiexternal_v2.BookDetails{
		ID:            strconv.Itoa(book.ID),
		Title:         book.TitleWithoutSeries,
		ISBN10:        book.ISBN,
		ISBN13:        book.ISBN13,
		ASIN:          book.ASIN,
		CoverURL:      book.ImageURL,
		Description:   stripHTMLTags(book.Description),
		Publisher:     book.Publisher,
		PageCount:     book.NumPages,
		Language:      book.LanguageCode,
		AverageRating: book.AverageRating,
		RatingsCount:  book.RatingsCount,
		GoodreadsID:   strconv.Itoa(book.ID),
		ProviderType:  apiexternal_v2.ProviderGoodreads,
	}

	// Full title if different
	if book.Title != "" && book.Title != book.TitleWithoutSeries {
		details.Title = book.Title
	}

	// Authors
	if len(book.Authors.Author) > 0 {
		authors := make([]string, 0, len(book.Authors.Author))
		for i := range book.Authors.Author {
			authors = append(authors, book.Authors.Author[i].Name)
		}

		details.Authors = authors
	}

	// Publication date
	if book.PublicationYear > 0 {
		details.PublishYear = book.PublicationYear

		month := book.PublicationMonth
		if month == 0 {
			month = 1
		}

		day := book.PublicationDay
		if day == 0 {
			day = 1
		}

		details.PublishDate = time.Date(
			book.PublicationYear,
			time.Month(month),
			day,
			0,
			0,
			0,
			0,
			time.UTC,
		)
	}

	// Series info
	if len(book.SeriesWorks.SeriesWork) > 0 {
		sw := book.SeriesWorks.SeriesWork[0]

		details.SeriesName = sw.Series.Title
		details.SeriesPosition = sw.UserPosition
	}

	return details
}

func convertAuthorToDetails(author *grAuthorFull) *apiexternal_v2.AuthorDetails {
	details := &apiexternal_v2.AuthorDetails{
		ID:           strconv.Itoa(author.ID),
		Name:         author.Name,
		Bio:          stripHTMLTags(author.About),
		ImageURL:     author.ImageURL,
		Website:      author.Link,
		WorkCount:    author.WorksCount,
		GoodreadsID:  strconv.Itoa(author.ID),
		ProviderType: apiexternal_v2.ProviderGoodreads,
	}

	// Parse birth date
	if author.BornAt != "" {
		details.BirthDate = parseGRDate(author.BornAt)
	}

	// Parse death date
	if author.DiedAt != "" {
		details.DeathDate = parseGRDate(author.DiedAt)
	}

	return details
}

func convertAuthorBooksToResults(books []grBookItem) []apiexternal_v2.BookSearchResult {
	results := make([]apiexternal_v2.BookSearchResult, 0, len(books))

	for i := range books {
		result := apiexternal_v2.BookSearchResult{
			ID:           strconv.Itoa(books[i].ID),
			Title:        books[i].Title,
			ISBN10:       books[i].ISBN,
			ISBN13:       books[i].ISBN13,
			CoverURL:     books[i].ImageURL,
			PublishYear:  books[i].PublicationYear,
			ProviderType: apiexternal_v2.ProviderGoodreads,
		}

		results = append(results, result)
	}

	return results
}

func convertSeriesToDetails(series *grSeriesFull) *apiexternal_v2.BookSeriesDetails {
	details := &apiexternal_v2.BookSeriesDetails{
		ID:           strconv.Itoa(series.ID),
		Name:         series.Title,
		Description:  stripHTMLTags(series.Description),
		TotalBooks:   series.SeriesWorksCount,
		ProviderType: apiexternal_v2.ProviderGoodreads,
	}

	// Convert series works to book results
	if len(series.SeriesWorks.SeriesWork) > 0 {
		books := make([]apiexternal_v2.BookSearchResult, 0, len(series.SeriesWorks.SeriesWork))
		for i := range series.SeriesWorks.SeriesWork {
			book := apiexternal_v2.BookSearchResult{
				ID:    strconv.Itoa(series.SeriesWorks.SeriesWork[i].Work.BestBook.ID),
				Title: series.SeriesWorks.SeriesWork[i].Work.BestBook.Title,
				Authors: []string{
					series.SeriesWorks.SeriesWork[i].Work.BestBook.Author.Name,
				},
				CoverURL:       series.SeriesWorks.SeriesWork[i].Work.BestBook.ImageURL,
				SeriesPosition: series.SeriesWorks.SeriesWork[i].UserPosition,
				ProviderType:   apiexternal_v2.ProviderGoodreads,
			}

			books = append(books, book)
		}

		details.Books = books
	}

	return details
}

//
// Helper Functions
//

// stripHTMLTags removes HTML tags from a string.
func stripHTMLTags(s string) string {
	// Simple HTML tag removal
	result := s
	for {
		start := strings.Index(result, "<")
		if start == -1 {
			break
		}

		end := strings.Index(result[start:], ">")
		if end == -1 {
			break
		}

		result = result[:start] + result[start+end+1:]
	}

	return strings.TrimSpace(result)
}

// parseGRDate attempts to parse Goodreads date formats.
func parseGRDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	layouts := []string{
		"2006/01/02",
		"2006-01-02",
		"January 2, 2006",
		"January 2006",
		"2006",
	}

	for i := range layouts {
		if t, err := time.Parse(layouts[i], dateStr); err == nil {
			return t
		}
	}

	return time.Time{}
}

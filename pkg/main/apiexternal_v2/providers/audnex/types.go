package audnex

import (
	"strconv"
	"time"

	"github.com/goccy/go-json"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
)

// flexibleFloat handles JSON values that can be either a number or a string.
type flexibleFloat float64

func (f *flexibleFloat) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as float64 first
	var floatVal float64
	if err := json.Unmarshal(data, &floatVal); err == nil {
		*f = flexibleFloat(floatVal)
		return nil
	}

	// Try to unmarshal as string
	var strVal string
	if err := json.Unmarshal(data, &strVal); err == nil {
		if strVal == "" {
			*f = 0
			return nil
		}

		parsed, err := strconv.ParseFloat(strVal, 64)
		if err != nil {
			*f = 0
			return nil // Don't fail on parse errors, just use 0
		}

		*f = flexibleFloat(parsed)

		return nil
	}

	// Default to 0 if we can't parse it
	*f = 0

	return nil
}

//
// Audnex Internal Types - Used for JSON unmarshaling
//

// audnexBookResponse represents the book API response.
type audnexBookResponse struct {
	ASIN           string             `json:"asin"`
	Title          string             `json:"title"`
	Subtitle       string             `json:"subtitle"`
	Description    string             `json:"description"`
	Summary        string             `json:"summary"`
	Image          string             `json:"image"`
	RuntimeMinutes int                `json:"runtimeLengthMin"`
	ReleaseDate    string             `json:"releaseDate"`
	Language       string             `json:"language"`
	Publisher      string             `json:"publisherName"`
	Authors        []audnexPerson     `json:"authors"`
	Narrators      []audnexPerson     `json:"narrators"`
	Series         flexibleSeriesList `json:"seriesPrimary"` // Can be object or array
	Genres         []audnexGenre      `json:"genres"`
	Rating         flexibleFloat      `json:"rating"` // Can be string or number from API
	ISBN           string             `json:"isbn"`
	Region         string             `json:"region"`
}

// audnexChaptersResponse represents the chapters API response.
type audnexChaptersResponse struct {
	ASIN                 string          `json:"asin"`
	BrandIntroDurationMs int             `json:"brandIntroDurationMs"`
	BrandOutroDurationMs int             `json:"brandOutroDurationMs"`
	RuntimeLengthMs      int             `json:"runtimeLengthMs"`
	RuntimeLengthSec     int             `json:"runtimeLengthSec"`
	Chapters             []audnexChapter `json:"chapters"`
}

// audnexChapter represents a single chapter.
type audnexChapter struct {
	LengthMs       int    `json:"lengthMs"`
	StartOffsetMs  int    `json:"startOffsetMs"`
	StartOffsetSec int    `json:"startOffsetSec"`
	Title          string `json:"title"`
}

// audnexPerson represents an author or narrator.
type audnexPerson struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
}

// audnexSeries represents a series.
type audnexSeries struct {
	ASIN     string `json:"asin"`
	Name     string `json:"name"`
	Position string `json:"position"`
}

// flexibleSeriesList handles seriesPrimary which can be either an object or an array.
type flexibleSeriesList []audnexSeries

func (f *flexibleSeriesList) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as array first
	var arr []audnexSeries
	if err := json.Unmarshal(data, &arr); err == nil {
		*f = arr
		return nil
	}

	// Try to unmarshal as single object
	var single audnexSeries
	if err := json.Unmarshal(data, &single); err == nil {
		// Only add if it has meaningful data
		if single.ASIN != "" || single.Name != "" {
			*f = []audnexSeries{single}
		} else {
			*f = []audnexSeries{}
		}

		return nil
	}

	// Default to empty slice
	*f = []audnexSeries{}

	return nil
}

// audnexGenre represents a genre/category.
type audnexGenre struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
	Type string `json:"type"`
}

// audnexAuthorResponse represents the author API response.
type audnexAuthorResponse struct {
	ASIN        string `json:"asin"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Image       string `json:"image"`
	Region      string `json:"region"`
}

// audnexAuthorBooksResponse represents the author books API response.
type audnexAuthorBooksResponse struct {
	ASIN  string           `json:"asin"`
	Name  string           `json:"name"`
	Books []audnexBookItem `json:"books"`
}

// audnexBookItem represents a book in the author's list.
type audnexBookItem struct {
	ASIN           string             `json:"asin"`
	Title          string             `json:"title"`
	Subtitle       string             `json:"subtitle"`
	Image          string             `json:"image"`
	RuntimeMinutes int                `json:"runtimeLengthMin"`
	ReleaseDate    string             `json:"releaseDate"`
	Authors        []audnexPerson     `json:"authors"`
	Narrators      []audnexPerson     `json:"narrators"`
	Series         flexibleSeriesList `json:"seriesPrimary"` // Can be object or array
}

// audnexAuthorSearchResult represents a single author in search results.
type audnexAuthorSearchResult struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
}

//
// Conversion Functions
//

func convertBookToDetails(book *audnexBookResponse) *apiexternal_v2.AudiobookDetails {
	details := &apiexternal_v2.AudiobookDetails{
		ID:           book.ASIN,
		ASIN:         book.ASIN,
		Title:        book.Title,
		Subtitle:     book.Subtitle,
		Description:  book.Description,
		Summary:      book.Summary,
		CoverURL:     book.Image,
		Duration:     time.Duration(book.RuntimeMinutes) * time.Minute,
		Language:     book.Language,
		Publisher:    book.Publisher,
		ISBN:         book.ISBN,
		Rating:       float64(book.Rating),
		ProviderType: apiexternal_v2.ProviderAudnex,
	}

	// Authors
	if len(book.Authors) > 0 {
		authors := make([]string, 0, len(book.Authors))

		authorIDs := make([]string, 0, len(book.Authors))
		for i := range book.Authors {
			authors = append(authors, book.Authors[i].Name)
			if book.Authors[i].ASIN != "" {
				authorIDs = append(authorIDs, book.Authors[i].ASIN)
			}
		}

		details.Authors = authors
		details.AuthorIDs = authorIDs
	}

	// Narrators
	if len(book.Narrators) > 0 {
		narrators := make([]string, 0, len(book.Narrators))

		narratorIDs := make([]string, 0, len(book.Narrators))
		for i := range book.Narrators {
			narrators = append(narrators, book.Narrators[i].Name)
			if book.Narrators[i].ASIN != "" {
				narratorIDs = append(narratorIDs, book.Narrators[i].ASIN)
			}
		}

		details.Narrators = narrators
		details.NarratorIDs = narratorIDs
	}

	// Series
	if len(book.Series) > 0 {
		details.Series = book.Series[0].Name
		details.SeriesASIN = book.Series[0].ASIN
		details.SeriesPosition = book.Series[0].Position
	}

	// Genres
	if len(book.Genres) > 0 {
		genres := make([]string, 0, len(book.Genres))
		for i := range book.Genres {
			genres = append(genres, book.Genres[i].Name)
		}

		details.Genres = genres
	}

	// Release date
	if book.ReleaseDate != "" {
		if t, err := time.Parse("2006-01-02", book.ReleaseDate); err == nil {
			details.ReleaseDate = t
			details.ReleaseYear = t.Year()
		}
	}

	return details
}

func convertChapters(chapters []audnexChapter) []apiexternal_v2.AudiobookChapter {
	result := make([]apiexternal_v2.AudiobookChapter, 0, len(chapters))

	for i := range chapters {
		result = append(result, apiexternal_v2.AudiobookChapter{
			Number:        i + 1,
			ChapterNumber: i + 1,
			Title:         chapters[i].Title,
			StartOffset:   time.Duration(chapters[i].StartOffsetMs) * time.Millisecond,
			StartOffsetMs: int64(chapters[i].StartOffsetMs),
			LengthMs:      int64(chapters[i].LengthMs),
			Duration:      time.Duration(chapters[i].LengthMs) * time.Millisecond,
		})
	}

	return result
}

func convertAuthorToDetails(author *audnexAuthorResponse) *apiexternal_v2.AuthorDetails {
	return &apiexternal_v2.AuthorDetails{
		ID:           author.ASIN,
		Name:         author.Name,
		Bio:          author.Description,
		ImageURL:     author.Image,
		ProviderType: apiexternal_v2.ProviderAudnex,
	}
}

func convertAuthorBooks(books []audnexBookItem) []apiexternal_v2.AudiobookSearchResult {
	results := make([]apiexternal_v2.AudiobookSearchResult, 0, len(books))

	for i := range books {
		result := apiexternal_v2.AudiobookSearchResult{
			ID:           books[i].ASIN,
			Title:        books[i].Title,
			Subtitle:     books[i].Subtitle,
			CoverURL:     books[i].Image,
			Duration:     time.Duration(books[i].RuntimeMinutes) * time.Minute,
			ProviderType: apiexternal_v2.ProviderAudnex,
		}

		// Authors
		if len(books[i].Authors) > 0 {
			authors := make([]string, 0, len(books[i].Authors))
			for j := range books[i].Authors {
				authors = append(authors, books[i].Authors[j].Name)
			}

			result.Authors = authors
		}

		// Narrators
		if len(books[i].Narrators) > 0 {
			narrators := make([]string, 0, len(books[i].Narrators))
			for j := range books[i].Narrators {
				narrators = append(narrators, books[i].Narrators[j].Name)
			}

			result.Narrators = narrators
		}

		// Series
		if len(books[i].Series) > 0 {
			result.Series = books[i].Series[0].Name
			result.SeriesPosition = books[i].Series[0].Position
		}

		// Release year
		if books[i].ReleaseDate != "" {
			if t, err := time.Parse("2006-01-02", books[i].ReleaseDate); err == nil {
				result.ReleaseYear = t.Year()
			}
		}

		results = append(results, result)
	}

	return results
}

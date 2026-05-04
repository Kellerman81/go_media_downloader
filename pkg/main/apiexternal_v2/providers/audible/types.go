package audible

import (
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

//
// Audible Internal Types - Used for JSON unmarshaling
//

// audibleSearchResponse represents the catalog search API response.
type audibleSearchResponse struct {
	Products   []audibleProduct `json:"products"`
	TotalCount int              `json:"total_results"`
}

// audibleProductResponse represents a single product API response.
type audibleProductResponse struct {
	Product audibleProduct `json:"product"`
}

// audibleProduct represents an audiobook product.
type audibleProduct struct {
	ASIN                 string             `json:"asin"`
	Title                string             `json:"title"`
	Subtitle             string             `json:"subtitle"`
	MerchandisingSummary string             `json:"merchandising_summary"`
	PublisherSummary     string             `json:"publisher_summary"`
	RuntimeLengthMin     int                `json:"runtime_length_min"`
	ReleaseDate          string             `json:"release_date"`
	IssueDate            string             `json:"issue_date"`
	Language             string             `json:"language"`
	Publisher            string             `json:"publisher_name"`
	ProductImages        audibleImages      `json:"product_images"`
	Authors              []audiblePerson    `json:"authors"`
	Narrators            []audiblePerson    `json:"narrators"`
	Series               []audibleSeries    `json:"series"`
	Categories           []audibleCategory  `json:"category_ladders"`
	Rating               audibleRating      `json:"rating"`
	Chapters             audibleChapterInfo `json:"chapter_info"`
	FormatType           string             `json:"format_type"`
	IsAdultProduct       bool               `json:"is_adult_product"`
	ISBN                 string             `json:"isbn"`
}

// audiblePerson represents an author or narrator.
type audiblePerson struct {
	ASIN string `json:"asin"`
	Name string `json:"name"`
}

// audibleSeries represents a series entry.
type audibleSeries struct {
	ASIN     string `json:"asin"`
	Title    string `json:"title"`
	Sequence string `json:"sequence"`
}

// audibleImages contains product image URLs.
type audibleImages struct {
	Image500  string `json:"500"`
	Image1024 string `json:"1024"`
}

// audibleCategory represents a category ladder.
type audibleCategory struct {
	Ladder []audibleCategoryItem `json:"ladder"`
}

// audibleCategoryItem represents a single category in the ladder.
type audibleCategoryItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// audibleRating contains rating information.
type audibleRating struct {
	OverallDistribution     audibleDistribution `json:"overall_distribution"`
	PerformanceDistribution audibleDistribution `json:"performance_distribution"`
	StoryDistribution       audibleDistribution `json:"story_distribution"`
}

// audibleDistribution contains rating distribution.
type audibleDistribution struct {
	DisplayStars         float64 `json:"display_stars"`
	DisplayAverageRating float64 `json:"display_average_rating"`
	NumReviews           int     `json:"num_reviews"`
}

// audibleChapterInfo contains chapter information.
type audibleChapterInfo struct {
	BrandIntroDurationMs int              `json:"brand_intro_duration_ms"`
	BrandOutroDurationMs int              `json:"brand_outro_duration_ms"`
	IsAccurate           bool             `json:"is_accurate"`
	RuntimeLengthMs      int              `json:"runtime_length_ms"`
	RuntimeLengthSec     int              `json:"runtime_length_sec"`
	Chapters             []audibleChapter `json:"chapters"`
}

// audibleChapter represents a single chapter.
type audibleChapter struct {
	LengthMs       int    `json:"length_ms"`
	StartOffsetMs  int    `json:"start_offset_ms"`
	StartOffsetSec int    `json:"start_offset_sec"`
	Title          string `json:"title"`
}

//
// Conversion Functions
//

func convertSearchResults(products []audibleProduct) []apiexternal_v2.AudiobookSearchResult {
	results := make([]apiexternal_v2.AudiobookSearchResult, 0, len(products))

	for i := range products {
		result := apiexternal_v2.AudiobookSearchResult{
			ID:             products[i].ASIN,
			ASIN:           products[i].ASIN,
			Title:          products[i].Title,
			Subtitle:       products[i].Subtitle,
			Duration:       time.Duration(products[i].RuntimeLengthMin) * time.Minute,
			RuntimeMinutes: products[i].RuntimeLengthMin,
			ProviderType:   apiexternal_v2.ProviderAudible,
		}

		// Authors
		if len(products[i].Authors) > 0 {
			authors := make([]string, 0, len(products[i].Authors))
			for j := range products[i].Authors {
				authors = append(authors, products[i].Authors[j].Name)
			}

			result.Authors = authors
		}

		// Narrators
		if len(products[i].Narrators) > 0 {
			narrators := make([]string, 0, len(products[i].Narrators))
			for j := range products[i].Narrators {
				narrators = append(narrators, products[i].Narrators[j].Name)
			}

			result.Narrators = narrators
		}

		// Cover URL
		if products[i].ProductImages.Image500 != "" {
			result.CoverURL = products[i].ProductImages.Image500
		}

		// Release year
		if products[i].ReleaseDate != "" {
			if t := parseAudibleDate(products[i].ReleaseDate); !t.IsZero() {
				result.ReleaseYear = t.Year()
			}
		}

		// Series
		if len(products[i].Series) > 0 {
			result.Series = products[i].Series[0].Title
			result.SeriesPosition = products[i].Series[0].Sequence
		}

		results = append(results, result)
	}

	return results
}

func convertProductToDetails(p *audibleProduct) *apiexternal_v2.AudiobookDetails {
	details := &apiexternal_v2.AudiobookDetails{
		ID:             p.ASIN,
		ASIN:           p.ASIN,
		Title:          p.Title,
		Subtitle:       p.Subtitle,
		Description:    cleanDescription(p.PublisherSummary),
		Summary:        cleanDescription(p.MerchandisingSummary),
		Duration:       time.Duration(p.RuntimeLengthMin) * time.Minute,
		RuntimeMinutes: p.RuntimeLengthMin,
		Language:       p.Language,
		Publisher:      p.Publisher,
		ProviderType:   apiexternal_v2.ProviderAudible,
	}

	// Authors with IDs
	if len(p.Authors) > 0 {
		authors := make([]string, 0, len(p.Authors))

		authorIDs := make([]string, 0, len(p.Authors))
		for i := range p.Authors {
			authors = append(authors, p.Authors[i].Name)
			if p.Authors[i].ASIN != "" {
				authorIDs = append(authorIDs, p.Authors[i].ASIN)
			}
		}

		details.Authors = authors
		details.AuthorIDs = authorIDs
	}

	// Narrators with IDs
	if len(p.Narrators) > 0 {
		narrators := make([]string, 0, len(p.Narrators))

		narratorIDs := make([]string, 0, len(p.Narrators))
		for i := range p.Narrators {
			narrators = append(narrators, p.Narrators[i].Name)
			if p.Narrators[i].ASIN != "" {
				narratorIDs = append(narratorIDs, p.Narrators[i].ASIN)
			}
		}

		details.Narrators = narrators
		details.NarratorIDs = narratorIDs
	}

	// Cover URLs
	if p.ProductImages.Image1024 != "" {
		details.CoverURL = p.ProductImages.Image1024
	} else if p.ProductImages.Image500 != "" {
		details.CoverURL = p.ProductImages.Image500
	}

	// Release date
	if p.ReleaseDate != "" {
		details.ReleaseDate = parseAudibleDate(p.ReleaseDate)
		if !details.ReleaseDate.IsZero() {
			details.ReleaseYear = details.ReleaseDate.Year()
		}
	}

	// Series
	if len(p.Series) > 0 {
		details.Series = p.Series[0].Title
		details.SeriesASIN = p.Series[0].ASIN
		details.SeriesPosition = p.Series[0].Sequence
	}

	// Genres from categories
	if len(p.Categories) > 0 {
		genres := make([]string, 0)
		for i := range p.Categories {
			for j := range p.Categories[i].Ladder {
				genres = append(genres, p.Categories[i].Ladder[j].Name)
			}
		}

		details.Genres = genres
	}

	// Rating
	if p.Rating.OverallDistribution.DisplayAverageRating > 0 {
		details.Rating = p.Rating.OverallDistribution.DisplayAverageRating
		details.AverageRating = p.Rating.OverallDistribution.DisplayAverageRating
		details.RatingCount = p.Rating.OverallDistribution.NumReviews
		details.RatingsCount = p.Rating.OverallDistribution.NumReviews
	}

	// Chapters
	if len(p.Chapters.Chapters) > 0 {
		chapters := make([]apiexternal_v2.AudiobookChapter, 0, len(p.Chapters.Chapters))
		for i, ch := range p.Chapters.Chapters {
			chapters = append(chapters, apiexternal_v2.AudiobookChapter{
				Number:      i + 1,
				Title:       ch.Title,
				StartOffset: time.Duration(ch.StartOffsetMs) * time.Millisecond,
				Duration:    time.Duration(ch.LengthMs) * time.Millisecond,
			})
		}

		details.Chapters = chapters
	}

	// ISBN
	if p.ISBN != "" {
		details.ISBN = p.ISBN
	}

	details.IsAdult = p.IsAdultProduct

	return details
}

//
// Helper Functions
//

// parseAudibleDate parses various date formats used by Audible.
func parseAudibleDate(dateStr string) time.Time {
	if dateStr == "" {
		return time.Time{}
	}

	// Try various formats
	layouts := []string{
		"2006-01-02",
		"01-02-2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2006",
	}

	for i := range layouts {
		if t, err := time.Parse(layouts[i], dateStr); err == nil {
			return t
		}
	}

	// Try to extract year only
	if len(dateStr) >= 4 {
		if year, err := strconv.Atoi(dateStr[:4]); err == nil && year > 1900 && year < 2100 {
			return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
		}
	}

	return time.Time{}
}

// cleanDescription removes HTML tags and cleans up the description text.
func cleanDescription(text string) string {
	if text == "" {
		return ""
	}

	// Simple HTML tag removal (for basic tags)
	result := text

	// Remove common HTML tags
	replacements := []string{
		"<p>", "\n",
		"</p>", "\n",
		"<br>", "\n",
		"<br/>", "\n",
		"<br />", "\n",
		"<b>", "",
		"</b>", "",
		"<i>", "",
		"</i>", "",
		"<em>", "",
		"</em>", "",
		"<strong>", "",
		"</strong>", "",
		"&nbsp;", " ",
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", "\"",
		"&#39;", "'",
	}

	for i := 0; i+1 < len(replacements); i += 2 {
		result = strings.ReplaceAll(result, replacements[i], replacements[i+1])
	}

	// Clean up extra whitespace
	lines := strings.Split(result, "\n")

	cleanLines := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	return logger.JoinStringsSep(cleanLines, "\n")
}

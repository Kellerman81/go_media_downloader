package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/audible"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/openlibrary"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	"maragu.dev/gomponents/html"
)

// BookMetadataSearchPage renders the book metadata search page.
func BookMetadataSearchPage(c *gin.Context) {
	bookLists := getMediaListsByType("book")
	csrfToken := getCSRFToken(c)

	content := bookMetadataSearchContent(bookLists, csrfToken)

	pageNode := page(
		"Book Metadata Search",
		false, // activeConfig
		false, // activeDatabase
		true,  // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// AudiobookMetadataSearchPage renders the audiobook metadata search page.
func AudiobookMetadataSearchPage(c *gin.Context) {
	audiobookLists := getMediaListsByType("audiobook")
	csrfToken := getCSRFToken(c)

	content := audiobookMetadataSearchContent(audiobookLists, csrfToken)

	pageNode := page(
		"Audiobook Metadata Search",
		false, // activeConfig
		false, // activeDatabase
		true,  // activeManagement
		content,
	)

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	pageNode.Render(&buf)
	c.String(http.StatusOK, buf.String())
}

func bookMetadataSearchContent(mediaConfigs []string, csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-book header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Book Metadata Search")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search for books across metadata sources and add them to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Search Form Card
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text("Search Books"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("bookSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("book_title"),
										html.Class("form-label"),
										gomponents.Text("Book Title"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("book_title"),
										html.Name("book_title"),
										html.Placeholder("Enter book title to search..."),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("book_author"),
										html.Class("form-label"),
										gomponents.Text("Author (optional)"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("book_author"),
										html.Name("book_author"),
										html.Placeholder("Enter author name..."),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("book_list"),
										html.Class("form-label"),
										gomponents.Text("Add to List"),
									),
									html.Select(
										html.Class("form-select"),
										html.ID("book_list"),
										html.Name("book_list"),
										gomponents.Attr("required", "true"),
										renderSelectOptions(mediaConfigs, ""),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text("Search Books"),
									),
								),
							),
						),
					),
				),

				// Manual Entry Card
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-edit me-2")),
								gomponents.Text("Manual Entry"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("bookManualForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-8 mb-3"),
										html.Label(
											html.For("manualBook_title"),
											html.Class("form-label"),
											gomponents.Text("Book Title *"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualBook_title"),
											html.Name("manualBook_title"),
											html.Placeholder("Enter book title"),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualBook_year"),
											html.Class("form-label"),
											gomponents.Text("Year"),
										),
										html.Input(
											html.Type("number"),
											html.Class("form-control"),
											html.ID("manualBook_year"),
											html.Name("manualBook_year"),
											html.Placeholder("YYYY"),
											gomponents.Attr("min", "1800"),
											gomponents.Attr("max", "2030"),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualBook_author"),
											html.Class("form-label"),
											gomponents.Text("Author"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualBook_author"),
											html.Name("manualBook_author"),
											html.Placeholder("Author name"),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualBook_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List *"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualBook_list"),
											html.Name("manualBook_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualBook_isbn"),
											html.Class("form-label"),
											gomponents.Text("ISBN"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualBook_isbn"),
											html.Name("manualBook_isbn"),
											html.Placeholder("ISBN-10 or ISBN-13"),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualBook_publisher"),
											html.Class("form-label"),
											gomponents.Text("Publisher"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualBook_publisher"),
											html.Name("manualBook_publisher"),
											html.Placeholder("Publisher name"),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-secondary"),
										html.I(html.Class("fas fa-plus me-2")),
										gomponents.Text("Add Book Manually"),
									),
								),
							),
						),
					),
				),
			),

			// Results area
			html.Div(
				html.ID("bookSearchResults"),
				html.Class("mt-4"),
			),

			bookSearchScript(csrfToken),
		),
	)
}

func audiobookMetadataSearchContent(mediaConfigs []string, csrfToken string) gomponents.Node {
	return html.Div(
		html.Class("config-section-enhanced"),

		// Page Header
		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(html.Class("fas fa-headphones header-icon")),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(
						html.Class("header-title"),
						gomponents.Text("Audiobook Metadata Search"),
					),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search for audiobooks on Audible and add them to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),

			// Search Form Card
			html.Div(
				html.Class("row g-4"),
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-search me-2")),
								gomponents.Text("Search Audiobooks"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("audiobookSearchForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("audiobook_title"),
										html.Class("form-label"),
										gomponents.Text("Audiobook Title"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("audiobook_title"),
										html.Name("audiobook_title"),
										html.Placeholder("Enter audiobook title to search..."),
									),
								),

								html.Div(
									html.Class("mb-3"),
									html.Label(
										html.For("audiobook_author"),
										html.Class("form-label"),
										gomponents.Text("Author (optional)"),
									),
									html.Input(
										html.Type("text"),
										html.Class("form-control"),
										html.ID("audiobook_author"),
										html.Name("audiobook_author"),
										html.Placeholder("Enter author name..."),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("audiobook_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("audiobook_list"),
											html.Name("audiobook_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("audiobook_region"),
											html.Class("form-label"),
											gomponents.Text("Audible Region"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("audiobook_region"),
											html.Name("audiobook_region"),
											html.Option(
												html.Value("de"),
												gomponents.Text("Germany (de)"),
											),
											html.Option(
												html.Value("us"),
												gomponents.Text("United States (us)"),
											),
											html.Option(
												html.Value("uk"),
												gomponents.Text("United Kingdom (uk)"),
											),
											html.Option(
												html.Value("fr"),
												gomponents.Text("France (fr)"),
											),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-primary"),
										html.I(html.Class("fas fa-search me-2")),
										gomponents.Text("Search Audiobooks"),
									),
								),
							),
						),
					),
				),

				// Manual Entry Card
				html.Div(
					html.Class("col-lg-6"),
					html.Div(
						html.Class("card shadow-sm"),
						html.Div(
							html.Class("card-header"),
							html.H5(
								html.Class("mb-0"),
								html.I(html.Class("fas fa-edit me-2")),
								gomponents.Text("Manual Entry"),
							),
						),
						html.Div(
							html.Class("card-body"),
							html.Form(
								html.ID("audiobookManualForm"),
								html.Input(
									html.Type("hidden"),
									html.Name("csrf_token"),
									html.Value(csrfToken),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-8 mb-3"),
										html.Label(
											html.For("manualAudiobook_title"),
											html.Class("form-label"),
											gomponents.Text("Audiobook Title *"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualAudiobook_title"),
											html.Name("manualAudiobook_title"),
											html.Placeholder("Enter audiobook title"),
											gomponents.Attr("required", "true"),
										),
									),
									html.Div(
										html.Class("col-md-4 mb-3"),
										html.Label(
											html.For("manualAudiobook_list"),
											html.Class("form-label"),
											gomponents.Text("Add to List *"),
										),
										html.Select(
											html.Class("form-select"),
											html.ID("manualAudiobook_list"),
											html.Name("manualAudiobook_list"),
											gomponents.Attr("required", "true"),
											renderSelectOptions(mediaConfigs, ""),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualAudiobook_author"),
											html.Class("form-label"),
											gomponents.Text("Author"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualAudiobook_author"),
											html.Name("manualAudiobook_author"),
											html.Placeholder("Author name"),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualAudiobook_narrator"),
											html.Class("form-label"),
											gomponents.Text("Narrator"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualAudiobook_narrator"),
											html.Name("manualAudiobook_narrator"),
											html.Placeholder("Narrator name"),
										),
									),
								),

								html.Div(
									html.Class("row"),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualAudiobook_asin"),
											html.Class("form-label"),
											gomponents.Text("ASIN"),
										),
										html.Input(
											html.Type("text"),
											html.Class("form-control"),
											html.ID("manualAudiobook_asin"),
											html.Name("manualAudiobook_asin"),
											html.Placeholder("Amazon ASIN"),
										),
									),
									html.Div(
										html.Class("col-md-6 mb-3"),
										html.Label(
											html.For("manualAudiobook_runtime"),
											html.Class("form-label"),
											gomponents.Text("Runtime (minutes)"),
										),
										html.Input(
											html.Type("number"),
											html.Class("form-control"),
											html.ID("manualAudiobook_runtime"),
											html.Name("manualAudiobook_runtime"),
											html.Placeholder("Duration in minutes"),
											gomponents.Attr("min", "1"),
										),
									),
								),

								html.Div(
									html.Class("d-grid"),
									html.Button(
										html.Type("submit"),
										html.Class("btn btn-secondary"),
										html.I(html.Class("fas fa-plus me-2")),
										gomponents.Text("Add Audiobook Manually"),
									),
								),
							),
						),
					),
				),
			),

			// Results area
			html.Div(
				html.ID("audiobookSearchResults"),
				html.Class("mt-4"),
			),

			audiobookSearchScript(csrfToken),
		),
	)
}

// SearchBookMetadata handles AJAX book search requests.
func SearchBookMetadata(c *gin.Context) {
	title := c.PostForm("book_title")
	author := c.PostForm("book_author")

	if title == "" && author == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title or author is required"})
		return
	}

	provider := openlibrary.NewProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := provider.SearchBooks(ctx, title, author, 20)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search books")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to search books: " + err.Error()},
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No books found for: "+title),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(fmt.Sprintf("Search Results (%d books found)", len(results))),
	))

	for _, book := range results {
		resultNodes = append(resultNodes, createBookResultCard(book))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

// SearchAudiobookMetadata handles AJAX audiobook search requests.
func SearchAudiobookMetadata(c *gin.Context) {
	title := c.PostForm("audiobook_title")
	author := c.PostForm("audiobook_author")
	region := c.PostForm("audiobook_region")

	if title == "" && author == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title or author is required"})
		return
	}

	// Map region to audible region
	audibleRegion := audible.RegionDE
	switch region {
	case "us":
		audibleRegion = audible.RegionUS
	case "uk":
		audibleRegion = audible.RegionUK
	case "fr":
		audibleRegion = audible.RegionFR
	}

	provider := audible.NewProviderWithRegion(audibleRegion)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	query := title
	if author != "" {
		query = author + " " + title
	}

	results, err := provider.SearchAudiobooks(ctx, query, 20)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to search audiobooks")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to search audiobooks: " + err.Error()},
		)

		return
	}

	if len(results) == 0 {
		c.Header("Content-Type", "text/html; charset=utf-8")

		var buf strings.Builder
		html.Div(
			html.Class("alert alert-warning text-center"),
			gomponents.Text("No audiobooks found for: "+query),
		).Render(&buf)
		c.String(http.StatusOK, buf.String())

		return
	}

	resultNodes := make([]gomponents.Node, 0, len(results)+1)

	resultNodes = append(resultNodes, html.H5(
		html.Class("mb-3"),
		gomponents.Text(fmt.Sprintf("Search Results (%d audiobooks found)", len(results))),
	))

	for _, audiobook := range results {
		resultNodes = append(resultNodes, createAudiobookResultCard(audiobook))
	}

	c.Header("Content-Type", "text/html; charset=utf-8")

	var buf strings.Builder
	html.Div(resultNodes...).Render(&buf)
	c.String(http.StatusOK, buf.String())
}

func createBookResultCard(book apiexternal_v2.BookSearchResult) gomponents.Node {
	year := ""
	if book.PublishYear > 0 {
		year = fmt.Sprintf(" (%d)", book.PublishYear)
	}

	authorStr := strings.Join(book.Authors, ", ")

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				html.Div(
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(book.Title+year),
					),
					func() gomponents.Node {
						if authorStr != "" {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text("by "+authorStr),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if book.ISBN13 != "" {
							return html.Span(
								html.Class("badge bg-secondary me-2"),
								gomponents.Text("ISBN: "+book.ISBN13),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Class("btn btn-success add-book-btn"),
					gomponents.Attr("data-openlibrary-id", book.ID),
					gomponents.Attr("data-title", book.Title),
					gomponents.Attr("data-isbn", book.ISBN13),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text("Add Book"),
				),
			),
		),
	)
}

func createAudiobookResultCard(audiobook apiexternal_v2.AudiobookSearchResult) gomponents.Node {
	authorStr := strings.Join(audiobook.Authors, ", ")
	narratorStr := strings.Join(audiobook.Narrators, ", ")

	durationStr := ""
	if audiobook.Duration > 0 {
		hours := int(audiobook.Duration.Hours())

		minutes := int(audiobook.Duration.Minutes()) % 60
		if hours > 0 {
			durationStr = fmt.Sprintf("%dh %dm", hours, minutes)
		} else {
			durationStr = fmt.Sprintf("%dm", minutes)
		}
	}

	return html.Div(
		html.Class("card mb-3"),
		html.Div(
			html.Class("card-body"),
			html.Div(
				html.Class("d-flex justify-content-between align-items-start"),
				html.Div(
					html.H5(
						html.Class("card-title mb-1"),
						gomponents.Text(audiobook.Title),
					),
					func() gomponents.Node {
						if authorStr != "" {
							return html.Small(
								html.Class("text-muted d-block"),
								gomponents.Text("by "+authorStr),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if narratorStr != "" {
							return html.Small(
								html.Class("text-muted d-block mb-2"),
								gomponents.Text("Narrated by: "+narratorStr),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if audiobook.Series != "" {
							return html.Span(
								html.Class("badge bg-info me-2"),
								gomponents.Text(
									"Series: "+audiobook.Series+" #"+audiobook.SeriesPosition,
								),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if durationStr != "" {
							return html.Span(
								html.Class("badge bg-secondary me-2"),
								html.I(html.Class("fas fa-clock me-1")),
								gomponents.Text(durationStr),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Class("btn btn-success add-audiobook-btn"),
					gomponents.Attr("data-asin", audiobook.ID),
					gomponents.Attr("data-title", audiobook.Title),
					html.I(html.Class("fas fa-plus me-1")),
					gomponents.Text("Add Audiobook"),
				),
			),
		),
	)
}

// AddBookToDatabase handles adding a book from metadata sources to the database.
func AddBookToDatabase(c *gin.Context) {
	openlibraryID := c.PostForm("openlibrary_id")
	listName := c.PostForm("book_list")
	isbn := c.PostForm("isbn")

	if listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "List name is required"})
		return
	}

	if openlibraryID == "" && isbn == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "OpenLibrary ID or ISBN is required"})
		return
	}

	provider := openlibrary.NewProvider()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		bookDetails *apiexternal_v2.BookDetails
		err         error
	)

	if openlibraryID != "" {
		bookDetails, err = provider.GetBookByID(ctx, openlibraryID)
	} else if isbn != "" {
		bookDetails, err = provider.SearchByISBN(ctx, isbn)
	}

	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to get book details")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to get book details: " + err.Error()},
		)

		return
	}

	// Check if book already exists in dbbooks
	var bookExists []int
	if bookDetails.ISBN13 != "" {
		bookExists = database.GetrowsN[int](
			false,
			1,
			"SELECT id FROM dbbooks WHERE isbn_13 = ?",
			bookDetails.ISBN13,
		)
	}

	if len(bookExists) == 0 && bookDetails.OpenLibraryID != "" {
		bookExists = database.GetrowsN[int](
			false,
			1,
			"SELECT id FROM dbbooks WHERE openlibrary_id = ?",
			bookDetails.OpenLibraryID,
		)
	}

	var dbBookID int

	nowTime := time.Now()

	if len(bookExists) == 0 {
		newID, err := database.ExecNid(
			"INSERT INTO dbbooks (title, original_title, isbn_13, isbn_10, asin, openlibrary_id, goodreads_id, description, publisher, publish_date, page_count, language, genres, cover_url, dbauthor_id, dbbook_series_id, series_position, average_rating, ratings_count, year, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			bookDetails.Title,
			bookDetails.Title, // Use Title as OriginalTitle since BookDetails doesn't have OriginalTitle
			bookDetails.ISBN13,
			bookDetails.ISBN10,
			"",
			bookDetails.OpenLibraryID,
			"",
			bookDetails.Description,
			bookDetails.Publisher,
			bookDetails.PublishDate,
			bookDetails.PageCount,
			bookDetails.Language,
			strings.Join(bookDetails.Genres, ", "),
			bookDetails.CoverURL,
			0,
			0,
			"",
			0.0,
			0,
			bookDetails.PublishYear,
			"",
			nowTime,
			nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert book")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Failed to add book to database: " + err.Error()},
			)

			return
		}

		dbBookID = int(newID)
	} else {
		dbBookID = bookExists[0]
	}

	// Check if book is already in the specified list
	listExists := database.GetrowsN[int](
		false,
		1,
		"SELECT count() FROM books WHERE dbbook_id = ? AND listname = ?",
		dbBookID,
		listName,
	)
	if len(listExists) > 0 && listExists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "Book already exists in list '" + listName + "'"},
		)

		return
	}

	// Add to books table
	database.ExecN(
		"INSERT INTO books (dbbook_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, '', 0, 0, 0, ?, ?)",
		dbBookID,
		listName,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Book added successfully to " + listName})
}

// AddAudiobookToDatabase handles adding an audiobook from metadata sources to the database.
func AddAudiobookToDatabase(c *gin.Context) {
	asin := c.PostForm("asin")
	listName := c.PostForm("audiobook_list")

	if asin == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ASIN and list name are required"})
		return
	}

	// Check if audiobook already exists in dbaudiobooks
	audiobookExists := database.GetrowsN[int](
		false,
		1,
		"SELECT id FROM dbaudiobooks WHERE asin = ?",
		asin,
	)

	var dbAudiobookID int

	nowTime := time.Now()

	if len(audiobookExists) == 0 {
		// We need to fetch details from Audible
		provider := audible.NewProviderWithRegion(audible.RegionDE)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		details, err := provider.SearchByASIN(ctx, asin)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to get audiobook details")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Failed to get audiobook details: " + err.Error()},
			)

			return
		}

		runtimeMinutes := int(details.Duration.Minutes())

		newID, err := database.ExecNid(
			"INSERT INTO dbaudiobooks (title, asin, audible_id, runtime_minutes, chapter_count, release_date, publisher, language, abridged, cover_url, sample_url, average_rating, ratings_count, year, slug, dbbook_id, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			details.Title,
			asin,
			"",
			runtimeMinutes,
			0,
			details.ReleaseDate,
			details.Publisher,
			details.Language,
			false, // Abridged - not available in AudiobookDetails
			details.CoverURL,
			"",
			0.0,
			0,
			details.ReleaseYear,
			"",
			0,
			details.Description,
			nowTime,
			nowTime,
		)
		if err != nil {
			logger.Logtype("error", 0).Err(err).Msg("Failed to insert audiobook")
			c.JSON(
				http.StatusInternalServerError,
				gin.H{"error": "Failed to add audiobook to database: " + err.Error()},
			)

			return
		}

		dbAudiobookID = int(newID)
	} else {
		dbAudiobookID = audiobookExists[0]
	}

	// Check if audiobook is already in the specified list
	listExists := database.GetrowsN[int](
		false,
		1,
		"SELECT count() FROM audiobooks WHERE dbaudiobook_id = ? AND listname = ?",
		dbAudiobookID,
		listName,
	)
	if len(listExists) > 0 && listExists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "Audiobook already exists in list '" + listName + "'"},
		)

		return
	}

	// Add to audiobooks table
	database.ExecN(
		"INSERT INTO audiobooks (dbaudiobook_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, '', 0, 0, 0, ?, ?)",
		dbAudiobookID,
		listName,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Audiobook added successfully to " + listName})
}

// AddBookManual handles manual book entry.
func AddBookManual(c *gin.Context) {
	title := c.PostForm("manualBook_title")
	yearStr := c.PostForm("manualBook_year")
	listName := c.PostForm("manualBook_list")
	author := c.PostForm("manualBook_author")
	isbn := c.PostForm("manualBook_isbn")
	publisher := c.PostForm("manualBook_publisher")

	if title == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title and list name are required"})
		return
	}

	year := 0
	if yearStr != "" {
		if parsedYear, err := strconv.Atoi(yearStr); err == nil {
			year = parsedYear
		}
	}

	// Check if book with same title already exists
	exists := database.GetrowsN[int](false, 1, "SELECT count() FROM dbbooks WHERE title = ?", title)
	if len(exists) > 0 && exists[0] > 0 {
		c.JSON(
			http.StatusConflict,
			gin.H{"error": "Book with same title already exists in database"},
		)

		return
	}

	nowTime := time.Now()

	newID, err := database.ExecNid(
		"INSERT INTO dbbooks (title, original_title, isbn_13, isbn_10, asin, openlibrary_id, goodreads_id, description, publisher, publish_date, page_count, language, genres, cover_url, dbauthor_id, dbbook_series_id, series_position, average_rating, ratings_count, year, slug, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		title,
		title,
		isbn,
		"",
		"",
		"",
		"",
		"",
		publisher,
		nil,
		0,
		"",
		"",
		"",
		0,
		0,
		"",
		0.0,
		0,
		year,
		"",
		nowTime,
		nowTime,
	)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to insert manual book")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add book to database"})
		return
	}

	// Create author entry if provided
	if author != "" {
		// For now, just log it - proper implementation would create dbauthor entry
		logger.Logtype("info", 0).Str("author", author).Msg("Author provided for manual book entry")
	}

	database.ExecN(
		"INSERT INTO books (dbbook_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, '', 0, 0, 0, ?, ?)",
		int(newID),
		listName,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Book added manually to " + listName})
}

// AddAudiobookManual handles manual audiobook entry.
func AddAudiobookManual(c *gin.Context) {
	title := c.PostForm("manualAudiobook_title")
	listName := c.PostForm("manualAudiobook_list")
	author := c.PostForm("manualAudiobook_author")
	narrator := c.PostForm("manualAudiobook_narrator")
	asin := c.PostForm("manualAudiobook_asin")
	runtimeStr := c.PostForm("manualAudiobook_runtime")

	if title == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Title and list name are required"})
		return
	}

	runtime := 0
	if runtimeStr != "" {
		if parsedRuntime, err := strconv.Atoi(runtimeStr); err == nil {
			runtime = parsedRuntime
		}
	}

	// Check if audiobook already exists
	var exists []int
	if asin != "" {
		exists = database.GetrowsN[int](
			false,
			1,
			"SELECT count() FROM dbaudiobooks WHERE asin = ?",
			asin,
		)
	} else {
		exists = database.GetrowsN[int](
			false,
			1,
			"SELECT count() FROM dbaudiobooks WHERE title = ?",
			title,
		)
	}

	if len(exists) > 0 && exists[0] > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "Audiobook already exists in database"})
		return
	}

	nowTime := time.Now()

	newID, err := database.ExecNid(
		"INSERT INTO dbaudiobooks (title, asin, audible_id, runtime_minutes, chapter_count, release_date, publisher, language, abridged, cover_url, sample_url, average_rating, ratings_count, year, slug, dbbook_id, description, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		title,
		asin,
		"",
		runtime,
		0,
		nil,
		"",
		"",
		false,
		"",
		"",
		0.0,
		0,
		0,
		"",
		0,
		"",
		nowTime,
		nowTime,
	)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("Failed to insert manual audiobook")
		c.JSON(
			http.StatusInternalServerError,
			gin.H{"error": "Failed to add audiobook to database"},
		)

		return
	}

	// Log author/narrator for reference
	if author != "" || narrator != "" {
		logger.Logtype("info", 0).
			Str("author", author).
			Str("narrator", narrator).
			Msg("Author/narrator provided for manual audiobook entry")
	}

	database.ExecN(
		"INSERT INTO audiobooks (dbaudiobook_id, listname, rootpath, missing, quality_reached, quality_profile, blacklisted, dont_upgrade, dont_search, created_at, updated_at) VALUES (?, ?, '', 1, 0, '', 0, 0, 0, ?, ?)",
		int(newID),
		listName,
		nowTime,
		nowTime,
	)

	c.JSON(http.StatusOK, gin.H{"success": "Audiobook added manually to " + listName})
}

func bookSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
document.getElementById('bookSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('book_title').value;
	const author = document.getElementById('book_author').value;
	const list = document.getElementById('book_list').value;

	if (!title && !author) {
		alert('Please enter a title or author');
		return;
	}

	if (!list) {
		alert('Please select a list');
		return;
	}

	const resultsDiv = document.getElementById('bookSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">Loading...</span></div><p class="mt-2">Searching books...</p></div>';

	fetch('/api/admin/search/books', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'book_title=' + encodeURIComponent(title) + '&book_author=' + encodeURIComponent(author)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;

		document.querySelectorAll('.add-book-btn').forEach(btn => {
			btn.addEventListener('click', function() {
				const openlibraryId = this.getAttribute('data-openlibrary-id');
				const bookTitle = this.getAttribute('data-title');
				const isbn = this.getAttribute('data-isbn');
				const selectedList = document.getElementById('book_list').value;

				if (!selectedList) {
					alert('Please select a list');
					return;
				}

				if (confirm('Add "' + bookTitle + '" to list "' + selectedList + '"?')) {
					addBookToDatabase(openlibraryId, isbn, selectedList, this);
				}
			});
		});
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for books. Please try again.</div>';
	});
});

function addBookToDatabase(openlibraryId, isbn, listName, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
	button.disabled = true;

	fetch('/api/admin/add/book', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'openlibrary_id=' + encodeURIComponent(openlibraryId) + '&isbn=' + encodeURIComponent(isbn) + '&book_list=' + encodeURIComponent(listName)
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>Added!';
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add book'));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding book to database');
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('bookManualForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const formData = new FormData(this);
	const data = new URLSearchParams(formData);

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Adding...';
	submitBtn.disabled = true;

	fetch('/api/admin/add/book/manual', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: data
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert('Success: ' + data.success);
			this.reset();
			submitBtn.innerHTML = '<i class="fas fa-check me-2"></i>Added!';
			setTimeout(() => {
				submitBtn.innerHTML = originalText;
				submitBtn.disabled = false;
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add book'));
			submitBtn.innerHTML = originalText;
			submitBtn.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding book manually');
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
`))
}

func audiobookSearchScript(csrfToken string) gomponents.Node {
	return html.Script(gomponents.Raw(`
document.getElementById('audiobookSearchForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const title = document.getElementById('audiobook_title').value;
	const author = document.getElementById('audiobook_author').value;
	const list = document.getElementById('audiobook_list').value;
	const region = document.getElementById('audiobook_region').value;

	if (!title && !author) {
		alert('Please enter a title or author');
		return;
	}

	if (!list) {
		alert('Please select a list');
		return;
	}

	const resultsDiv = document.getElementById('audiobookSearchResults');
	resultsDiv.innerHTML = '<div class="text-center p-4"><div class="spinner-border text-primary" role="status"><span class="visually-hidden">Loading...</span></div><p class="mt-2">Searching audiobooks...</p></div>';

	fetch('/api/admin/search/audiobooks', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'audiobook_title=' + encodeURIComponent(title) + '&audiobook_author=' + encodeURIComponent(author) + '&audiobook_region=' + encodeURIComponent(region)
	})
	.then(response => response.text())
	.then(data => {
		resultsDiv.innerHTML = data;

		document.querySelectorAll('.add-audiobook-btn').forEach(btn => {
			btn.addEventListener('click', function() {
				const asin = this.getAttribute('data-asin');
				const audiobookTitle = this.getAttribute('data-title');
				const selectedList = document.getElementById('audiobook_list').value;

				if (!selectedList) {
					alert('Please select a list');
					return;
				}

				if (confirm('Add "' + audiobookTitle + '" to list "' + selectedList + '"?')) {
					addAudiobookToDatabase(asin, selectedList, this);
				}
			});
		});
	})
	.catch(error => {
		console.error('Error:', error);
		resultsDiv.innerHTML = '<div class="alert alert-danger">Error searching for audiobooks. Please try again.</div>';
	});
});

function addAudiobookToDatabase(asin, listName, button) {
	const originalText = button.innerHTML;
	button.innerHTML = '<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
	button.disabled = true;

	fetch('/api/admin/add/audiobook', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: 'asin=' + encodeURIComponent(asin) + '&audiobook_list=' + encodeURIComponent(listName)
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			button.innerHTML = '<i class="fas fa-check me-1"></i>Added!';
			button.classList.remove('btn-success');
			button.classList.add('btn-outline-success');
			setTimeout(() => {
				button.innerHTML = originalText;
				button.disabled = false;
				button.classList.add('btn-success');
				button.classList.remove('btn-outline-success');
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add audiobook'));
			button.innerHTML = originalText;
			button.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding audiobook to database');
		button.innerHTML = originalText;
		button.disabled = false;
	});
}

document.getElementById('audiobookManualForm').addEventListener('submit', function(e) {
	e.preventDefault();

	const formData = new FormData(this);
	const data = new URLSearchParams(formData);

	const submitBtn = this.querySelector('button[type="submit"]');
	const originalText = submitBtn.innerHTML;
	submitBtn.innerHTML = '<i class="fas fa-spinner fa-spin me-2"></i>Adding...';
	submitBtn.disabled = true;

	fetch('/api/admin/add/audiobook/manual', {
		method: 'POST',
		headers: {
			'Content-Type': 'application/x-www-form-urlencoded',
			'X-CSRF-Token': '` + csrfToken + `',
		},
		body: data
	})
	.then(response => response.json())
	.then(data => {
		if (data.success) {
			alert('Success: ' + data.success);
			this.reset();
			submitBtn.innerHTML = '<i class="fas fa-check me-2"></i>Added!';
			setTimeout(() => {
				submitBtn.innerHTML = originalText;
				submitBtn.disabled = false;
			}, 2000);
		} else {
			alert('Error: ' + (data.error || 'Failed to add audiobook'));
			submitBtn.innerHTML = originalText;
			submitBtn.disabled = false;
		}
	})
	.catch(error => {
		console.error('Error:', error);
		alert('Error adding audiobook manually');
		submitBtn.innerHTML = originalText;
		submitBtn.disabled = false;
	});
});
`))
}

// Helper function to get media lists by type - extend for new types.
func init() {
	// This extends getMediaListsByType in metadata_search.go
}

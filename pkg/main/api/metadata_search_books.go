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
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
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

// Book metadata provider keys.
const (
	provOpenLibrary = "openlibrary"
	provGoodreads   = "goodreads"
)

// optionsSelectCol renders a labelled <select> column with explicit value/label pairs.
func optionsSelectCol(id, label string, values, labels []string, cols int) gomponents.Node {
	opts := make([]gomponents.Node, 0, len(values))
	for i, v := range values {
		lbl := v
		if i < len(labels) {
			lbl = labels[i]
		}

		opts = append(opts, html.Option(html.Value(v), gomponents.Text(lbl)))
	}

	if len(values) == 0 {
		opts = append(opts, html.Option(html.Value(""), gomponents.Text("(none configured)")))
	}

	return html.Div(
		html.Class(fmt.Sprintf("col-md-%d", cols)),
		html.Label(html.For(id), html.Class("form-label"), gomponents.Text(label)),
		html.Select(append([]gomponents.Node{
			html.Class("form-select"), html.ID(id), html.Name(id),
		}, opts...)...),
	)
}

// bookProviderOptions returns the configured book metadata providers (values, labels).
func bookProviderOptions() ([]string, []string) {
	values := []string{provOpenLibrary}
	labels := []string{"OpenLibrary"}

	if providers.GetGoodreads() != nil {
		values = append(values, provGoodreads)
		labels = append(labels, "Goodreads")
	}

	return values, labels
}

// bookMetadataSearchContent renders the 3-mode "Add Books" page.
func bookMetadataSearchContent(mediaConfigs []string, csrfToken string) gomponents.Node {
	provVals, provLabels := bookProviderOptions()

	return html.Div(
		html.Class("config-section-enhanced"),

		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(
						html.Class("fas fa-book header-icon"),
						gomponents.Attr("aria-hidden", "true"),
					),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Add Books")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search book metadata providers and add single titles or an author's catalogue to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),
			html.Input(html.Type("hidden"), html.ID("bk_csrf"), html.Value(csrfToken)),

			html.Ul(
				html.Class("nav nav-tabs mb-3"), html.Role("tablist"),
				musicTab("bk1", "fas fa-book", "Single Book", true),
				musicTab("bk2", "fas fa-user-plus", "Full Author", false),
				musicTab("bk3", "fas fa-list-check", "Selected Books", false),
			),

			html.Div(
				html.Class("tab-content"),

				musicTabPane("bk1", true,
					musicCardWrap(
						"Search for a single book",
						"Enter a title (and optionally an author), pick a provider, then add any match.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							optionsSelectCol("bk1_provider", "Provider", provVals, provLabels, 2),
							musicInputCol("bk1_title", "Title", "Enter book title...", 3, true),
							musicInputCol(
								"bk1_author",
								"Author (optional)",
								"Enter author...",
								3,
								false,
							),
							musicListSelectCol("bk1_list", mediaConfigs, 2),
							musicButtonCol("Search", "fas fa-search", "bookSearch('bk1')"),
						),
					),
					html.Div(html.ID("bk1_results"), html.Class("mt-3")),
				),

				musicTabPane("bk2", false,
					musicCardWrap(
						"Add a complete author",
						"Enter an author name and add their whole catalogue. A background job imports every book found for that author.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							optionsSelectCol("bk2_provider", "Provider", provVals, provLabels, 2),
							musicInputCol(
								"bk2_author",
								"Author Name",
								"Enter author name...",
								4,
								true,
							),
							musicListSelectCol("bk2_list", mediaConfigs, 3),
							html.Div(html.Class("col-md-3 d-grid"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-success"),
									gomponents.Attr("onclick", "bookAddAuthor()"),
									html.I(
										html.Class("fas fa-user-plus me-1"),
										gomponents.Attr("aria-hidden", "true"),
									),
									gomponents.Text("Add Author"),
								),
							),
						),
						html.Div(
							html.Class("mt-2"),
							html.Button(
								html.Type("button"),
								html.Class("btn btn-sm btn-outline-secondary"),
								gomponents.Attr("onclick", "bookSearch('bk2')"),
								html.I(
									html.Class("fas fa-eye me-1"),
									gomponents.Attr("aria-hidden", "true"),
								),
								gomponents.Text("Preview books by this author"),
							),
						),
					),
					html.Div(html.ID("bk2_results"), html.Class("mt-3")),
				),

				musicTabPane("bk3", false,
					musicCardWrap("Add selected books of an author",
						"Search an author's books, then pick exactly which titles to add.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							optionsSelectCol("bk3_provider", "Provider", provVals, provLabels, 2),
							musicInputCol(
								"bk3_author",
								"Author Name",
								"Enter author name...",
								4,
								true,
							),
							musicListSelectCol("bk3_list", mediaConfigs, 3),
							musicButtonCol("Search", "fas fa-search", "bookSearch('bk3')"),
						),
					),
					html.Div(html.ID("bk3_results"), html.Class("mt-3")),
				),
			),

			bookSearchScript(),
		),
	)
}

// audiobookMetadataSearchContent renders the 3-mode "Add Audiobooks" page.
func audiobookMetadataSearchContent(mediaConfigs []string, csrfToken string) gomponents.Node {
	regionVals := []string{"de", "us", "uk", "fr"}
	regionLabels := []string{"Audible.de", "Audible.com", "Audible.co.uk", "Audible.fr"}

	return html.Div(
		html.Class("config-section-enhanced"),

		html.Div(
			html.Class("page-header-enhanced"),
			html.Div(
				html.Class("header-content"),
				html.Div(
					html.Class("header-icon-wrapper"),
					html.I(
						html.Class("fas fa-headphones header-icon"),
						gomponents.Attr("aria-hidden", "true"),
					),
				),
				html.Div(
					html.Class("header-text"),
					html.H2(html.Class("header-title"), gomponents.Text("Add Audiobooks")),
					html.P(
						html.Class("header-subtitle"),
						gomponents.Text(
							"Search Audible and add single audiobooks or an author's catalogue to your library",
						),
					),
				),
			),
		),

		html.Div(
			html.Class("container-fluid"),
			html.Input(html.Type("hidden"), html.ID("ab_csrf"), html.Value(csrfToken)),

			html.Ul(
				html.Class("nav nav-tabs mb-3"), html.Role("tablist"),
				musicTab("ab1", "fas fa-headphones", "Single Audiobook", true),
				musicTab("ab2", "fas fa-user-plus", "Full Author", false),
				musicTab("ab3", "fas fa-list-check", "Selected Audiobooks", false),
			),

			html.Div(
				html.Class("tab-content"),

				musicTabPane("ab1", true,
					musicCardWrap(
						"Search for a single audiobook",
						"Enter a title (and optionally an author), pick an Audible region, then add any match.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							optionsSelectCol("ab1_region", "Region", regionVals, regionLabels, 2),
							musicInputCol(
								"ab1_title",
								"Title",
								"Enter audiobook title...",
								3,
								true,
							),
							musicInputCol(
								"ab1_author",
								"Author (optional)",
								"Enter author...",
								3,
								false,
							),
							musicListSelectCol("ab1_list", mediaConfigs, 2),
							musicButtonCol("Search", "fas fa-search", "audiobookSearch('ab1')"),
						),
					),
					html.Div(html.ID("ab1_results"), html.Class("mt-3")),
				),

				musicTabPane("ab2", false,
					musicCardWrap(
						"Add a complete author",
						"Enter an author name and add their whole catalogue. A background job imports every audiobook found for that author.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							optionsSelectCol("ab2_region", "Region", regionVals, regionLabels, 2),
							musicInputCol(
								"ab2_author",
								"Author Name",
								"Enter author name...",
								4,
								true,
							),
							musicListSelectCol("ab2_list", mediaConfigs, 3),
							html.Div(html.Class("col-md-3 d-grid"),
								html.Button(
									html.Type("button"),
									html.Class("btn btn-success"),
									gomponents.Attr("onclick", "audiobookAddAuthor()"),
									html.I(
										html.Class("fas fa-user-plus me-1"),
										gomponents.Attr("aria-hidden", "true"),
									),
									gomponents.Text("Add Author"),
								),
							),
						),
						html.Div(
							html.Class("mt-2"),
							html.Button(
								html.Type("button"),
								html.Class("btn btn-sm btn-outline-secondary"),
								gomponents.Attr("onclick", "audiobookSearch('ab2')"),
								html.I(
									html.Class("fas fa-eye me-1"),
									gomponents.Attr("aria-hidden", "true"),
								),
								gomponents.Text("Preview audiobooks by this author"),
							),
						),
					),
					html.Div(html.ID("ab2_results"), html.Class("mt-3")),
				),

				musicTabPane("ab3", false,
					musicCardWrap("Add selected audiobooks of an author",
						"Search an author's audiobooks, then pick exactly which titles to add.",
						html.Div(
							html.Class("row g-3 align-items-end"),
							optionsSelectCol("ab3_region", "Region", regionVals, regionLabels, 2),
							musicInputCol(
								"ab3_author",
								"Author Name",
								"Enter author name...",
								4,
								true,
							),
							musicListSelectCol("ab3_list", mediaConfigs, 3),
							musicButtonCol("Search", "fas fa-search", "audiobookSearch('ab3')"),
						),
					),
					html.Div(html.ID("ab3_results"), html.Class("mt-3")),
				),
			),

			audiobookSearchScript(),
		),
	)
}

// ---------------------------------------------------------------------------
// Book search + cards
// ---------------------------------------------------------------------------

// bookSearchDispatch runs a book search on the chosen provider.
func bookSearchDispatch(
	ctx context.Context,
	provider, title, author string,
	limit int,
) ([]apiexternal_v2.BookSearchResult, error) {
	if provider == provGoodreads {
		if p := providers.GetGoodreads(); p != nil {
			query := strings.TrimSpace(strings.TrimSpace(author) + " " + strings.TrimSpace(title))
			return p.SearchBooks(ctx, query, 1)
		}

		return nil, errProviderUnavailable
	}

	p := providers.GetOpenLibrary()
	if p == nil {
		p = openlibrary.NewProvider()
	}

	return p.SearchBooks(ctx, title, author, limit)
}

// SearchBookMetadata handles book search. view=select renders a checklist (mode 3),
// otherwise add-cards (modes 1 & 2 preview).
func SearchBookMetadata(c *gin.Context) {
	provider := c.PostForm("provider")
	title := c.PostForm("book_title")
	author := c.PostForm("book_author")
	selectView := c.PostForm("view") == "select"

	if title == "" && author == "" {
		renderMusicAlert(c, "Please enter a title or author", "warning")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	results, err := bookSearchDispatch(ctx, provider, title, author, 25)
	if err != nil {
		logger.Logtype("error", 0).Err(err).Str("provider", provider).Msg("book search failed")
		renderMusicAlert(c, "Search failed: "+err.Error(), "danger")

		return
	}

	if len(results) == 0 {
		renderMusicAlert(c, "No books found.", "warning")
		return
	}

	if selectView {
		renderMusicHTML(c, bookSelectionList(results))
		return
	}

	nodes := make([]gomponents.Node, 0, len(results)+1)

	nodes = append(
		nodes,
		html.H6(html.Class("mb-2"), gomponents.Textf("%d books found", len(results))),
	)

	for i := range results {
		nodes = append(nodes, createBookResultCard(results[i]))
	}

	renderMusicHTML(c, html.Div(nodes...))
}

// openLibraryWorkID returns the id only when it is an OpenLibrary work id ("OL...W").
// Results from other providers (e.g. Goodreads) use different id schemes, so they fall
// back to ISBN-based adds instead of being mis-treated as OpenLibrary ids.
func openLibraryWorkID(id string) string {
	if strings.HasPrefix(id, "OL") {
		return id
	}

	return ""
}

func createBookResultCard(book apiexternal_v2.BookSearchResult) gomponents.Node {
	title := book.Title
	if book.PublishYear > 0 {
		title = fmt.Sprintf("%s (%d)", book.Title, book.PublishYear)
	}

	authorStr := strings.Join(book.Authors, ", ")

	return html.Div(
		html.Class("card mb-2"),
		html.Div(html.Class("card-body py-2"),
			html.Div(html.Class("d-flex justify-content-between align-items-start"),
				html.Div(html.Class("flex-grow-1"),
					html.H6(html.Class("card-title mb-1"), gomponents.Text(title)),
					func() gomponents.Node {
						if authorStr != "" {
							return html.Small(
								html.Class("text-muted d-block mb-1"),
								gomponents.Text("by "+authorStr),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if book.ISBN13 != "" {
							return html.Span(
								html.Class("badge bg-secondary me-1"),
								gomponents.Text("ISBN: "+book.ISBN13),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Class("btn btn-success btn-sm add-book-btn ms-3"),
					html.Data("openlibrary-id", openLibraryWorkID(book.ID)),
					html.Data("isbn", book.ISBN13),
					html.I(html.Class("fas fa-plus me-1"), gomponents.Attr("aria-hidden", "true")),
					gomponents.Text("Add Book"),
				),
			),
		),
	)
}

// bookSelectionList renders a checklist of books with an Add Selected bar (mode 3).
func bookSelectionList(results []apiexternal_v2.BookSearchResult) gomponents.Node {
	items := make([]gomponents.Node, 0, len(results))
	for i := range results {
		b := results[i]

		title := b.Title
		if b.PublishYear > 0 {
			title = fmt.Sprintf("%s (%d)", b.Title, b.PublishYear)
		}

		items = append(items, html.Label(
			html.Class("form-check d-flex align-items-center gap-2 mb-0"),
			html.Input(
				html.Type("checkbox"), html.Class("form-check-input book-select-check mt-0"),
				html.Data("openlibrary-id", openLibraryWorkID(b.ID)), html.Data("isbn", b.ISBN13),
			),
			html.Span(html.Class("form-check-label"), gomponents.Text(title)),
		))
	}

	return html.Div(
		html.Class("border rounded p-2 mt-2 book-select-list"),
		html.Div(html.Class("d-flex justify-content-between align-items-center mb-2"),
			html.Span(html.Class("fw-semibold"), gomponents.Textf("%d books", len(items))),
			html.Div(
				html.Button(
					html.Type("button"),
					html.Class("btn btn-sm btn-outline-secondary me-2 select-all-books"),
					gomponents.Text("Select all"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-sm btn-success add-selected-books"),
					html.I(
						html.Class("fas fa-plus me-1"),
						gomponents.Attr("aria-hidden", "true"),
					),
					gomponents.Text("Add Selected"),
				),
			),
		),
		html.Div(append([]gomponents.Node{html.Class("d-flex flex-column gap-1")}, items...)...),
	)
}

// AddBooksByAuthor queues a background import of all books by an author.
func AddBooksByAuthor(c *gin.Context) {
	author := c.PostForm("book_author")
	listName := c.PostForm("book_list")

	if author == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Author and list are required"})
		return
	}

	cfgp, listid := findBookCfgpAndListID(listName)
	if cfgp == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "List '" + listName + "' not found in book configuration"},
		)

		return
	}

	authorCopy := author
	worker.Dispatch(
		"add_book_author_"+author+"_"+listName,
		func(_ uint32, ctx context.Context) error {
			err := importfeed.JobImportDBBook(
				ctx,
				&config.ManualConfig{AuthorName: authorCopy},
				0,
				cfgp,
				listid,
			)
			logger.Logtype("info", 0).
				Str("author", authorCopy).
				Str("list", listName).
				Err(err).
				Msg("AddBooksByAuthor: completed")

			return nil
		},
		"Data",
	)

	c.JSON(
		http.StatusOK,
		gin.H{
			"success": fmt.Sprintf(
				"Importing books by \"%s\" into %s in the background.",
				author,
				listName,
			),
		},
	)
}

// ---------------------------------------------------------------------------
// Audiobook search + cards
// ---------------------------------------------------------------------------

// audibleProviderForRegion returns an Audible provider for the given region code.
func audibleProviderForRegion(region string) *audible.Provider {
	switch region {
	case "us":
		return audible.NewProviderWithRegion(audible.RegionUS)
	case "uk":
		return audible.NewProviderWithRegion(audible.RegionUK)
	case "fr":
		return audible.NewProviderWithRegion(audible.RegionFR)
	default:
		return audible.NewProviderWithRegion(audible.RegionDE)
	}
}

// SearchAudiobookMetadata handles audiobook search. byauthor=1 searches by author;
// view=select renders a checklist (mode 3).
func SearchAudiobookMetadata(c *gin.Context) {
	region := c.PostForm("audiobook_region")
	title := c.PostForm("audiobook_title")
	author := c.PostForm("audiobook_author")
	byAuthor := c.PostForm("byauthor") == "1"
	selectView := c.PostForm("view") == "select"

	if title == "" && author == "" {
		renderMusicAlert(c, "Please enter a title or author", "warning")
		return
	}

	provider := audibleProviderForRegion(region)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		results []apiexternal_v2.AudiobookSearchResult
		err     error
	)

	if byAuthor && author != "" {
		results, err = provider.SearchByAuthor(ctx, author, 25)
	} else {
		query := title
		if author != "" {
			query = strings.TrimSpace(author + " " + title)
		}

		results, err = provider.SearchAudiobooks(ctx, query, 25)
	}

	if err != nil {
		logger.Logtype("error", 0).Err(err).Msg("audiobook search failed")
		renderMusicAlert(c, "Search failed: "+err.Error(), "danger")

		return
	}

	if len(results) == 0 {
		renderMusicAlert(c, "No audiobooks found.", "warning")
		return
	}

	if selectView {
		renderMusicHTML(c, audiobookSelectionList(results))
		return
	}

	nodes := make([]gomponents.Node, 0, len(results)+1)

	nodes = append(
		nodes,
		html.H6(html.Class("mb-2"), gomponents.Textf("%d audiobooks found", len(results))),
	)

	for i := range results {
		nodes = append(nodes, createAudiobookResultCard(results[i]))
	}

	renderMusicHTML(c, html.Div(nodes...))
}

func audiobookDurationStr(d time.Duration) string {
	if d <= 0 {
		return ""
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}

	return fmt.Sprintf("%dm", minutes)
}

func createAudiobookResultCard(ab apiexternal_v2.AudiobookSearchResult) gomponents.Node {
	authorStr := strings.Join(ab.Authors, ", ")
	narratorStr := strings.Join(ab.Narrators, ", ")
	durationStr := audiobookDurationStr(ab.Duration)

	return html.Div(
		html.Class("card mb-2"),
		html.Div(html.Class("card-body py-2"),
			html.Div(html.Class("d-flex justify-content-between align-items-start"),
				html.Div(html.Class("flex-grow-1"),
					html.H6(html.Class("card-title mb-1"), gomponents.Text(ab.Title)),
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
								html.Class("text-muted d-block mb-1"),
								gomponents.Text("Narrated by: "+narratorStr),
							)
						}

						return nil
					}(),
					func() gomponents.Node {
						if durationStr != "" {
							return html.Span(
								html.Class("badge bg-secondary me-1"),
								html.I(
									html.Class("fas fa-clock me-1"),
								),
								gomponents.Text(durationStr),
							)
						}

						return nil
					}(),
				),
				html.Button(
					html.Class("btn btn-success btn-sm add-audiobook-btn ms-3"),
					html.Data("asin", ab.ASIN),
					html.I(html.Class("fas fa-plus me-1"), gomponents.Attr("aria-hidden", "true")),
					gomponents.Text("Add Audiobook"),
				),
			),
		),
	)
}

// audiobookSelectionList renders a checklist of audiobooks with an Add Selected bar.
func audiobookSelectionList(results []apiexternal_v2.AudiobookSearchResult) gomponents.Node {
	items := make([]gomponents.Node, 0, len(results))
	for i := range results {
		ab := results[i]

		label := ab.Title
		if len(ab.Authors) > 0 {
			label = ab.Title + " — " + strings.Join(ab.Authors, ", ")
		}

		items = append(items, html.Label(
			html.Class("form-check d-flex align-items-center gap-2 mb-0"),
			html.Input(
				html.Type("checkbox"), html.Class("form-check-input ab-select-check mt-0"),
				html.Data("asin", ab.ASIN),
			),
			html.Span(html.Class("form-check-label"), gomponents.Text(label)),
		))
	}

	return html.Div(
		html.Class("border rounded p-2 mt-2 ab-select-list"),
		html.Div(html.Class("d-flex justify-content-between align-items-center mb-2"),
			html.Span(html.Class("fw-semibold"), gomponents.Textf("%d audiobooks", len(items))),
			html.Div(
				html.Button(
					html.Type("button"),
					html.Class("btn btn-sm btn-outline-secondary me-2 select-all-ab"),
					gomponents.Text("Select all"),
				),
				html.Button(
					html.Type("button"),
					html.Class("btn btn-sm btn-success add-selected-ab"),
					html.I(
						html.Class("fas fa-plus me-1"),
						gomponents.Attr("aria-hidden", "true"),
					),
					gomponents.Text("Add Selected"),
				),
			),
		),
		html.Div(append([]gomponents.Node{html.Class("d-flex flex-column gap-1")}, items...)...),
	)
}

// AddAudiobooksByAuthor queues a background import of all audiobooks by an author.
func AddAudiobooksByAuthor(c *gin.Context) {
	author := c.PostForm("audiobook_author")
	listName := c.PostForm("audiobook_list")

	if author == "" || listName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Author and list are required"})
		return
	}

	cfgp, listid := findAudiobookCfgpAndListID(listName)
	if cfgp == nil {
		c.JSON(
			http.StatusBadRequest,
			gin.H{"error": "List '" + listName + "' not found in audiobook configuration"},
		)

		return
	}

	authorCopy := author
	worker.Dispatch(
		"add_audiobook_author_"+author+"_"+listName,
		func(_ uint32, ctx context.Context) error {
			err := importfeed.JobImportDBAudiobook(
				ctx,
				&config.ManualConfig{AuthorName: authorCopy},
				0,
				cfgp,
				listid,
			)
			logger.Logtype("info", 0).
				Str("author", authorCopy).
				Str("list", listName).
				Err(err).
				Msg("AddAudiobooksByAuthor: completed")

			return nil
		},
		"Data",
	)

	c.JSON(
		http.StatusOK,
		gin.H{
			"success": fmt.Sprintf(
				"Importing audiobooks by \"%s\" into %s in the background.",
				author,
				listName,
			),
		},
	)
}

// findBookCfgpAndListID resolves a book list name to its config and index.
func findBookCfgpAndListID(listName string) (*config.MediaTypeConfig, int) {
	allMedia := config.GetSettingsMediaAll()
	if allMedia == nil {
		return nil, -1
	}

	for i := range allMedia.Books {
		cfgp := config.GetSettingsMedia("book_" + allMedia.Books[i].Name)
		if cfgp == nil {
			continue
		}

		if listid, ok := cfgp.ListsMapIdx[listName]; ok {
			return cfgp, listid
		}
	}

	return nil, -1
}

// findAudiobookCfgpAndListID resolves an audiobook list name to its config and index.
func findAudiobookCfgpAndListID(listName string) (*config.MediaTypeConfig, int) {
	allMedia := config.GetSettingsMediaAll()
	if allMedia == nil {
		return nil, -1
	}

	for i := range allMedia.AudioBooks {
		cfgp := config.GetSettingsMedia("audiobook_" + allMedia.AudioBooks[i].Name)
		if cfgp == nil {
			continue
		}

		if listid, ok := cfgp.ListsMapIdx[listName]; ok {
			return cfgp, listid
		}
	}

	return nil, -1
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

// bookSearchScript returns client logic for the 3 book modes.
func bookSearchScript() gomponents.Node {
	return html.Script(gomponents.Raw(`
function bkCsrf(){ var e=document.getElementById('bk_csrf'); return e?e.value:''; }
function bkPost(url, params){ return fetch(url,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','X-CSRF-Token':bkCsrf()},body:params.toString()}); }
function bkVal(id){ var e=document.getElementById(id); return e?e.value:''; }
function bkSpin(el,l){ el.innerHTML='<div class="text-center p-4"><div class="spinner-border text-primary" role="status"></div><p class="mt-2 mb-0">'+l+'</p></div>'; }

function bookSearch(mode){
	var author=bkVal(mode+'_author');
	var title=bkVal(mode+'_title');
	if(!title && !author){ showToaster('warning','Enter a title or author'); return; }
	var box=document.getElementById(mode+'_results');
	bkSpin(box,'Searching...');
	var p=new URLSearchParams();
	p.append('provider', bkVal(mode+'_provider'));
	p.append('book_title', title);
	p.append('book_author', author);
	if(mode==='bk3'){ p.append('view','select'); }
	bkPost('/api/admin/search/books', p).then(function(r){return r.text();})
		.then(function(h){ box.innerHTML=h; })
		.catch(function(){ box.innerHTML='<div class="alert alert-danger">Search failed.</div>'; });
}

function bookAddAuthor(){
	var author=bkVal('bk2_author'), list=bkVal('bk2_list');
	if(!author){ showToaster('warning','Enter an author name'); return; }
	if(!list){ showToaster('warning','Select a list'); return; }
	confirmAction('Add author','Import all books by "'+author+'" into "'+list+'"? This runs in the background.',function(){
		var p=new URLSearchParams();
		p.append('book_author',author); p.append('book_list',list);
		bkPost('/api/admin/add/book/author',p).then(function(r){return r.json();}).then(function(d){
			if(d.success){ showToaster('success',d.success); } else { showToaster('error',d.error||'Failed'); }
		}).catch(function(){ showToaster('error','Failed to queue'); });
	});
}

document.addEventListener('click', function(e){
	var add=e.target.closest && e.target.closest('.add-book-btn');
	if(add){
		var pane=add.closest('.tab-pane');
		var listEl=pane?pane.querySelector('[id$="_list"]'):null;
		var list=listEl?listEl.value:'';
		if(!list){ showToaster('warning','Select a list first'); return; }
		var orig=add.innerHTML; add.disabled=true; add.innerHTML='<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
		var p=new URLSearchParams();
		p.append('openlibrary_id', add.getAttribute('data-openlibrary-id')||'');
		p.append('isbn', add.getAttribute('data-isbn')||'');
		p.append('book_list', list);
		bkPost('/api/admin/add/book',p).then(function(r){return r.json();}).then(function(d){
			if(d.success){ showToaster('success',d.success); add.innerHTML='<i class="fas fa-check me-1"></i>Added'; add.classList.remove('btn-success'); add.classList.add('btn-outline-success'); }
			else { showToaster('error',d.error||'Failed to add book'); add.innerHTML=orig; add.disabled=false; }
		}).catch(function(){ showToaster('error','Failed to add book'); add.innerHTML=orig; add.disabled=false; });
		return;
	}
	var selAll=e.target.closest && e.target.closest('.select-all-books');
	if(selAll){
		var wrap=selAll.closest('.book-select-list'); var checks=wrap.querySelectorAll('.book-select-check');
		var any=Array.prototype.some.call(checks,function(c){return !c.checked;});
		checks.forEach(function(c){c.checked=any;});
		return;
	}
	var addSel=e.target.closest && e.target.closest('.add-selected-books');
	if(addSel){
		var pane2=addSel.closest('.tab-pane'); var listEl2=pane2?pane2.querySelector('[id$="_list"]'):null;
		var list2=listEl2?listEl2.value:'';
		if(!list2){ showToaster('warning','Select a list first'); return; }
		var wrap2=addSel.closest('.book-select-list');
		var chosen=[]; wrap2.querySelectorAll('.book-select-check:checked').forEach(function(c){ chosen.push(c); });
		if(chosen.length===0){ showToaster('warning','Select at least one book'); return; }
		confirmAction('Add selected','Add '+chosen.length+' selected book(s) to "'+list2+'"?',function(){
			addSel.disabled=true; addSel.innerHTML='<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
			var jobs=chosen.map(function(c){
				var p=new URLSearchParams();
				p.append('openlibrary_id', c.getAttribute('data-openlibrary-id')||'');
				p.append('isbn', c.getAttribute('data-isbn')||'');
				p.append('book_list', list2);
				return bkPost('/api/admin/add/book',p).then(function(r){return r.json();}).then(function(d){return !!d.success;}).catch(function(){return false;});
			});
			Promise.all(jobs).then(function(res){
				var ok=res.filter(Boolean).length;
				showToaster(ok>0?'success':'error', 'Added '+ok+' of '+res.length+' book(s)');
				addSel.innerHTML='<i class="fas fa-check me-1"></i>Done';
			});
		});
		return;
	}
});
`))
}

// audiobookSearchScript returns client logic for the 3 audiobook modes.
func audiobookSearchScript() gomponents.Node {
	return html.Script(gomponents.Raw(`
function abCsrf(){ var e=document.getElementById('ab_csrf'); return e?e.value:''; }
function abPost(url, params){ return fetch(url,{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','X-CSRF-Token':abCsrf()},body:params.toString()}); }
function abVal(id){ var e=document.getElementById(id); return e?e.value:''; }
function abSpin(el,l){ el.innerHTML='<div class="text-center p-4"><div class="spinner-border text-primary" role="status"></div><p class="mt-2 mb-0">'+l+'</p></div>'; }

function audiobookSearch(mode){
	var author=abVal(mode+'_author');
	var title=abVal(mode+'_title');
	if(!title && !author){ showToaster('warning','Enter a title or author'); return; }
	var box=document.getElementById(mode+'_results');
	abSpin(box,'Searching...');
	var p=new URLSearchParams();
	p.append('audiobook_region', abVal(mode+'_region'));
	p.append('audiobook_title', title);
	p.append('audiobook_author', author);
	if(mode==='ab2' || mode==='ab3'){ p.append('byauthor','1'); }
	if(mode==='ab3'){ p.append('view','select'); }
	abPost('/api/admin/search/audiobooks', p).then(function(r){return r.text();})
		.then(function(h){ box.innerHTML=h; })
		.catch(function(){ box.innerHTML='<div class="alert alert-danger">Search failed.</div>'; });
}

function audiobookAddAuthor(){
	var author=abVal('ab2_author'), list=abVal('ab2_list');
	if(!author){ showToaster('warning','Enter an author name'); return; }
	if(!list){ showToaster('warning','Select a list'); return; }
	confirmAction('Add author','Import all audiobooks by "'+author+'" into "'+list+'"? This runs in the background.',function(){
		var p=new URLSearchParams();
		p.append('audiobook_author',author); p.append('audiobook_list',list);
		abPost('/api/admin/add/audiobook/author',p).then(function(r){return r.json();}).then(function(d){
			if(d.success){ showToaster('success',d.success); } else { showToaster('error',d.error||'Failed'); }
		}).catch(function(){ showToaster('error','Failed to queue'); });
	});
}

document.addEventListener('click', function(e){
	var add=e.target.closest && e.target.closest('.add-audiobook-btn');
	if(add){
		var pane=add.closest('.tab-pane');
		var listEl=pane?pane.querySelector('[id$="_list"]'):null;
		var list=listEl?listEl.value:'';
		if(!list){ showToaster('warning','Select a list first'); return; }
		var orig=add.innerHTML; add.disabled=true; add.innerHTML='<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
		var p=new URLSearchParams();
		p.append('asin', add.getAttribute('data-asin')||'');
		p.append('audiobook_list', list);
		abPost('/api/admin/add/audiobook',p).then(function(r){return r.json();}).then(function(d){
			if(d.success){ showToaster('success',d.success); add.innerHTML='<i class="fas fa-check me-1"></i>Added'; add.classList.remove('btn-success'); add.classList.add('btn-outline-success'); }
			else { showToaster('error',d.error||'Failed to add audiobook'); add.innerHTML=orig; add.disabled=false; }
		}).catch(function(){ showToaster('error','Failed to add audiobook'); add.innerHTML=orig; add.disabled=false; });
		return;
	}
	var selAll=e.target.closest && e.target.closest('.select-all-ab');
	if(selAll){
		var wrap=selAll.closest('.ab-select-list'); var checks=wrap.querySelectorAll('.ab-select-check');
		var any=Array.prototype.some.call(checks,function(c){return !c.checked;});
		checks.forEach(function(c){c.checked=any;});
		return;
	}
	var addSel=e.target.closest && e.target.closest('.add-selected-ab');
	if(addSel){
		var pane2=addSel.closest('.tab-pane'); var listEl2=pane2?pane2.querySelector('[id$="_list"]'):null;
		var list2=listEl2?listEl2.value:'';
		if(!list2){ showToaster('warning','Select a list first'); return; }
		var wrap2=addSel.closest('.ab-select-list');
		var chosen=[]; wrap2.querySelectorAll('.ab-select-check:checked').forEach(function(c){ chosen.push(c); });
		if(chosen.length===0){ showToaster('warning','Select at least one audiobook'); return; }
		confirmAction('Add selected','Add '+chosen.length+' selected audiobook(s) to "'+list2+'"?',function(){
			addSel.disabled=true; addSel.innerHTML='<i class="fas fa-spinner fa-spin me-1"></i>Adding...';
			var jobs=chosen.map(function(c){
				var p=new URLSearchParams();
				p.append('asin', c.getAttribute('data-asin')||'');
				p.append('audiobook_list', list2);
				return abPost('/api/admin/add/audiobook',p).then(function(r){return r.json();}).then(function(d){return !!d.success;}).catch(function(){return false;});
			});
			Promise.all(jobs).then(function(res){
				var ok=res.filter(Boolean).length;
				showToaster(ok>0?'success':'error', 'Added '+ok+' of '+res.length+' audiobook(s)');
				addSel.innerHTML='<i class="fas fa-check me-1"></i>Done';
			});
		});
		return;
	}
});
`))
}

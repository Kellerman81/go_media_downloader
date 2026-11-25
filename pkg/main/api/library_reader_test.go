package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestLibraryReader: a book file opens in the reader; the position read
// is kept and from 95% on the book counts as read.
func TestLibraryReader(t *testing.T) {
	clientTestData(t)

	admin := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	asAdmin(c)
	c.Params = gin.Params{{Key: "id", Value: "51"}, {Key: "name", Value: "/novel.epub"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	HandleLibraryRead(c)

	if w.Code != http.StatusOK || w.Body.String() != "data of novel.epub" || w.Header().Get("Content-Type") != "application/epub+zip" {
		t.Fatalf("read: %d %q %s", w.Code, w.Body, w.Header().Get("Content-Type"))
	}

	// pdf.js loads big PDFs in pieces: the file answers range requests.
	w = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(w)
	asAdmin(c)
	c.Params = gin.Params{{Key: "id", Value: "51"}, {Key: "name", Value: "/novel.epub"}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request.Header.Set("Range", "bytes=0-3")
	HandleLibraryRead(c)

	if w.Code != http.StatusPartialContent || w.Body.String() != "data" {
		t.Fatalf("range: %d %q", w.Code, w.Body)
	}

	if w := callAs(admin, HandleLibraryReadProgress, url.Values{"id": {"51"}, "pct": {"0.4"}}); w.Code >= http.StatusMultipleChoices {
		t.Fatalf("progress: %d %s", w.Code, w.Body)
	}

	if c := database.PlayCompletions("admin", "book", "book_files", "book_id", new(uint64(1))); c[51] != 0.4 {
		t.Fatalf("completion: %v", c)
	}

	_, body := libGet(t, HandleLibraryItem, "type=book&id=1")
	if !strings.Contains(body, "libRead(51, ") || !strings.Contains(body, "0.4000") {
		t.Fatalf("dialog: %.600s", body)
	}

	callAs(admin, HandleLibraryReadProgress, url.Values{"id": {"51"}, "pct": {"0.97"}})

	if _, body := libGet(t, HandleLibraryGrid, "group=book_B&filter=watched"); !strings.Contains(body, "Novel") || !strings.Contains(body, "Read") {
		t.Fatalf("read filter: %.600s", body)
	}

	if w := callAs(admin, HandleLibraryWatched, url.Values{"kind": {"book"}, "id": {"51"}, "watched": {"0"}}); w.Code >= http.StatusMultipleChoices {
		t.Fatalf("unmark: %d", w.Code)
	}

	if _, body := libGet(t, HandleLibraryGrid, "group=book_B&filter=unwatched"); !strings.Contains(body, "Novel") {
		t.Fatalf("unread filter: %.600s", body)
	}

	anna := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_EN"}}
	if w := callAs(anna, HandleLibraryReadProgress, url.Values{"id": {"51"}, "pct": {"0.5"}}); w.Code != http.StatusBadRequest {
		t.Fatalf("anna: %d", w.Code)
	}
}

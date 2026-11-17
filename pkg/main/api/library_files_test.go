package api

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

func getAs(s *Session, h gin.HandlerFunc, params ...string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Set("session", s)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	for i := 0; i+1 < len(params); i += 2 {
		c.Params = append(c.Params, gin.Param{Key: params[i], Value: params[i+1]})
	}

	h(c)

	return w
}

// TestLibraryDownload: only with the download feature and only files of
// the account's groups; albums come as one ZIP.
func TestLibraryDownload(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	dir := t.TempDir()
	for i, name := range []string{"01.flac", "02.flac"} {
		loc := filepath.Join(dir, name)
		if err := os.WriteFile(loc, []byte("track "+name), 0o644); err != nil {
			t.Fatal(err)
		}

		if i == 0 {
			if err := database.ExecNErr("update movie_files set location = ? where id = 12", loc); err != nil {
				t.Fatal(err)
			}
		}
	}

	for _, q := range []string{
		"insert into dbalbums (id, title) values (1, 'Album')",
		"insert into albums (id, dbalbum_id, listname) values (1, 1, 'DE List')",
		"insert into album_files (id, album_id, dbalbum_id, location, track_number) values (1, 1, 1, '" + filepath.Join(dir, "01.flac") +
			"', 1), (2, 1, 1, '" + filepath.Join(dir, "02.flac") + "', 2)",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	plain := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	loader := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}, Features: []string{database.FeatureDownload}}

	if w := getAs(plain, HandleLibraryDownload, "type", "movie", "id", "12"); w.Code != http.StatusForbidden {
		t.Errorf("without the feature: %d", w.Code)
	}

	if w := getAs(loader, HandleLibraryDownload, "type", "movie", "id", "11"); w.Code != http.StatusNotFound {
		t.Errorf("file of another group: %d", w.Code)
	}

	if w := getAs(loader, HandleLibraryDownload, "type", "movie", "id", "12"); w.Code != http.StatusOK || w.Body.String() != "track 01.flac" ||
		w.Header().Get("Content-Disposition") != `attachment; filename=01.flac` {
		t.Errorf("download: %d %q %q", w.Code, w.Body, w.Header().Get("Content-Disposition"))
	}

	w := getAs(loader, HandleLibraryDownloadEntry, "type", "album", "id", "1")

	zr, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil || len(zr.File) != 2 || zr.File[0].Name != "01.flac" || zr.File[1].Name != "02.flac" {
		t.Fatalf("album ZIP: %d, %v", w.Code, err)
	}
}

// TestLibraryReport: an account with requestdelete reports a file once; an
// admin deletes it and the entry is searched again.
func TestLibraryReport(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	plain := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}}
	reporter := &Session{UserID: "anna", Role: database.RoleUser, Groups: []string{"movie_DE"}, Features: []string{database.FeatureRequestDelete}}
	form := url.Values{"type": {"movie"}, "id": {"12"}, "reason": {"language"}, "note": {"English audio"}}

	if w := callAs(plain, HandleLibraryReport, form); w.Code != http.StatusForbidden {
		t.Errorf("without the feature: %d", w.Code)
	}

	if w := callAs(reporter, HandleLibraryReport, url.Values{"type": {"movie"}, "id": {"11"}}); w.Code != http.StatusNotFound {
		t.Errorf("file of another group: %d", w.Code)
	}

	if w := callAs(reporter, HandleLibraryReport, form); w.Code != http.StatusOK {
		t.Fatalf("report: %d %s", w.Code, w.Body)
	}

	if w := callAs(reporter, HandleLibraryReport, form); w.Code != http.StatusConflict {
		t.Errorf("second report: %d", w.Code)
	}

	reports := database.GetMediaReports()
	if len(reports) != 1 || reports[0].Reason != "language" || reports[0].Title != "de.mkv" {
		t.Fatalf("reports: %+v", reports)
	}

	admin := &Session{UserID: DefaultUsername, Role: database.RoleAdmin}
	if w := callAs(admin, HandleReportResolve, url.Values{"id": {"1"}, "action": {"research"}}); w.Code != http.StatusOK {
		t.Fatalf("resolve: %d", w.Code)
	}

	if database.CountMediaReports() != 0 || database.Getdatarow[int](false, "select count() from movie_files where id = 12") != 0 ||
		!database.Getdatarow[bool](false, "select missing from movies where id = 2") {
		t.Error("the file wasn't deleted, the report not closed or the movie not missing")
	}
}

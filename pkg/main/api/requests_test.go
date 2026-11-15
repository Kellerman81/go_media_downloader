package api

import (
	"errors"
	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestRequestable: a requesting user's addition is held back from searching
// and recorded; an admin's isn't. Approve lets it be searched, reject
// removes it.
func TestRequestable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// A second list in the DE group, to move a request within its group.
	de := mediaGroup("DE")
	de[0].Lists = append(de[0].Lists, config.MediaListsConfig{Name: "DE Wishes", TemplateList: "listA", TemplateQuality: "HD", Enabled: true})
	accessTestDataWith(t, config.MediaConfig{Movies: append(mediaGroup("EN"), de...), Series: mediaGroup("X")})

	// Stands in for an Add Media handler: creates a list entry.
	add := func(c *gin.Context) {
		database.ExecN("insert into movies (dbmovie_id, listname) values (2, 'DE List')")
		c.JSON(http.StatusOK, gin.H{"success": "Movie added successfully to DE List"})
	}

	call := func(setSession func(*gin.Context)) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		setSession(c)
		c.Request = httptest.NewRequest(http.MethodPost, "/api/admin/add/movie", strings.NewReader("movie_list=DE+List"))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		requestable("movie", add)(c)

		return w.Body.String()
	}

	body := call(func(c *gin.Context) { asUser(c, database.RoleRequestingUser, "movie_DE") })
	if !strings.Contains(body, "Requested: Deutsch") || !strings.Contains(body, "approve") {
		t.Fatalf("request reply: %s", body)
	}

	reqs := database.GetMediaRequests()
	if len(reqs) != 1 || reqs[0].Username != "anna" || reqs[0].Kind != "movie" || reqs[0].Listname != "DE List" {
		t.Fatalf("requests: %+v", reqs)
	}

	entry := reqs[0].EntryID
	if !database.Getdatarow[bool](false, "select dont_search from movies where id = ?", &entry) {
		t.Fatal("requested entry would be searched")
	}

	// An admin's addition is not a request.
	if body := call(asAdmin); !strings.Contains(body, "added successfully") || database.CountMediaRequests() != 1 {
		t.Errorf("admin addition: %s, %d requests", body, database.CountMediaRequests())
	}

	post := func(h gin.HandlerFunc, id uint, extra ...string) string {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		asAdmin(c)

		form := url.Values{"id": {itoa(int(id))}}
		for i := 0; i+1 < len(extra); i += 2 {
			form.Set(extra[i], extra[i+1])
		}

		c.Request = httptest.NewRequest(http.MethodPost, "/", strings.NewReader(form.Encode()))
		c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h(c)

		return w.Body.String()
	}

	// The requests page offers the lists of the request's group only, and
	// the quality profiles.
	if page := renderComponentToString(renderRequests("tok", "")); !strings.Contains(page, `<option value="DE List" selected>`) ||
		!strings.Contains(page, `"DE Wishes"`) || strings.Contains(page, `"EN List"`) || !strings.Contains(page, `name="quality"`) {
		t.Errorf("requests page: %s", page)
	}

	// DE List has this movie already (row 2): not twice.
	if body := post(HandleRequestApprove, reqs[0].ID, "list", "DE List"); !strings.Contains(body, "already has") {
		t.Errorf("duplicate approve: %s", body)
	}

	// Another group's list wouldn't be searched by this group.
	if body := post(HandleRequestApprove, reqs[0].ID, "list", "EN List"); !strings.Contains(body, "not a list of the media group") {
		t.Errorf("other group's list: %s", body)
	}

	if body := post(HandleRequestApprove, reqs[0].ID, "list", "DE Wishes", "quality", "HD"); !strings.Contains(body, "moved to DE Wishes") {
		t.Fatalf("approve: %s", body)
	}

	if database.Getdatarow[bool](false, "select dont_search from movies where id = ?", &entry) || database.CountMediaRequests() != 0 {
		t.Error("approved entry still held back")
	}

	if got := database.Getdatarow[string](false, "select listname || '/' || quality_profile from movies where id = ?", &entry); got != "DE Wishes/HD" {
		t.Errorf("approved entry: %s", got)
	}

	// Reject removes the entry.
	call(func(c *gin.Context) { asUser(c, database.RoleRequestingUser, "movie_DE") })
	reqs = database.GetMediaRequests()

	if body := post(HandleRequestReject, reqs[0].ID); !strings.Contains(body, "rejected") {
		t.Fatalf("reject: %s", body)
	}

	if database.Getdatarow[uint](false, "select count() from movies where id = ?", &reqs[0].EntryID) != 0 || database.CountMediaRequests() != 0 {
		t.Error("rejected entry kept")
	}
}

// TestHeldSeries: a held series holds back its episodes from every
// search: the episode search refuses them, and the missing search and the
// RSS check read the series' flag too.
func TestHeldSeries(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	series := mediatype.Get(config.MediaTypeSeries)

	disabled := func() bool {
		var entry apiexternal_v2.Nzbwithprio
		return errors.Is(series.FillSearchVar(&entry, 5), logger.ErrDisabled)
	}

	r := database.MediaRequest{Username: "anna", Kind: "series", EntryID: 1, Listname: "X List", Title: "Show"}
	if err := database.HoldMediaRequest(&r); err != nil {
		t.Fatal(err)
	}

	if !disabled() {
		t.Error("episode of a held series is searched")
	}

	// The RSS search reads the flags with GetRSSData; its filter rejects
	// releases of entries without files while dont_search is set.
	var dontSearch, dontUpgrade bool

	var list, quality, title string

	database.GetdatarowArgs(mtstrings.GetStringsMap(config.MediaTypeSeries, "GetRSSData"), new(uint(5)),
		&dontSearch, &dontUpgrade, &list, &quality, &title)

	if !dontSearch {
		t.Error("RSS sees the episode of a held series as searchable")
	}

	if n := database.Getdatarow[uint](false, "select count() from serie_episodes where serie_id = 1 and dont_search = 1"); n != 0 {
		t.Errorf("holding the series changed %d episodes", n)
	}

	if err := database.ApproveMediaRequest(&r, "", ""); err != nil {
		t.Fatal(err)
	}

	if disabled() {
		t.Error("episode still held after approval")
	}
}

// TestSearchSelectionSkipsHeld: the selection of the missing search
// (jobsearchmedia) leaves out entries and series held back from searching.
func TestSearchSelectionSkipsHeld(t *testing.T) {
	gin.SetMode(gin.TestMode)
	accessTestData(t)

	database.ExecN("update movies set missing = 1")
	database.ExecN("update dbmovies set year = 2020")

	missing := func(isType uint, list string) []uint {
		q := mtstrings.GetStringsMap(isType, logger.SearchGenSelect) + mtstrings.GetStringsMap(isType, logger.SearchGenTable) +
			mtstrings.GetStringsMap(isType, logger.SearchGenMissing) + mtstrings.GetStringsMap(isType, logger.SearchGenMissingEnd)

		return database.GetrowsN[uint](false, 10, "select id from ("+q+")", &list)
	}

	if got := missing(config.MediaTypeMovie, "DE List"); len(got) != 1 {
		t.Fatalf("missing DE movies: %v", got)
	}

	database.ExecN("update movies set dont_search = 1 where id = 2")

	if got := missing(config.MediaTypeMovie, "DE List"); len(got) != 0 {
		t.Errorf("held movie selected: %v", got)
	}

	if got := missing(config.MediaTypeSeries, "X List"); len(got) != 1 {
		t.Fatalf("missing episodes: %v", got)
	}

	database.ExecN("update series set dont_search = 1 where id = 1")

	if got := missing(config.MediaTypeSeries, "X List"); len(got) != 0 {
		t.Errorf("episode of a held series selected: %v", got)
	}
}

package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestLibraryArtists: the artist view lists the music group's artists; the
// artist filter narrows the album grid to one.
func TestLibraryArtists(t *testing.T) {
	clientTestData(t)

	_, body := libGet(t, HandleLibraryGrid, "group=music_M&view=artists")
	if !strings.Contains(body, "Band") || !strings.Contains(body, "libShowArtist(&#34;1&#34;)") && !strings.Contains(body, `libShowArtist("1")`) {
		t.Fatalf("artists: %.800s", body)
	}

	_, body = libGet(t, HandleLibraryGrid, "group=music_M&artist=1&filter=all")
	if !strings.Contains(body, "Record") || !strings.Contains(body, "libShowArtist(&#39;&#39;)") {
		t.Fatalf("albums of the artist: %.800s", body)
	}

	if _, body = libGet(t, HandleLibraryGrid, "group=music_M&artist=99&filter=all"); strings.Contains(body, "Record") {
		t.Fatalf("albums of another artist: %.800s", body)
	}

	// The image of an artist the account can't see is refused.
	for _, tc := range []struct {
		groups []string
		code   int
	}{{nil, http.StatusOK}, {[]string{"movie_EN"}, http.StatusNotFound}} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		if tc.groups == nil {
			asAdmin(c)
		} else {
			asUser(c, "user", tc.groups...)
		}

		c.Params = gin.Params{{Key: "id", Value: "1"}}
		c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
		HandleLibraryArtistImage(c)

		if w.Code != tc.code {
			t.Errorf("groups %v: %d, want %d", tc.groups, w.Code, tc.code)
		}
	}
}

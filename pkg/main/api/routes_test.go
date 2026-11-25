package api

import (
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// TestAPIEndpointDocsMatchRoutes checks that every endpoint the API testing
// page offers is actually registered.
func TestAPIEndpointDocsMatchRoutes(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)

	r := gin.New()
	g := r.Group("/api")
	AddWebRoutes(g)
	AddGeneralRoutes(g)
	AddAllRoutes(g.Group("/all"))
	AddMoviesRoutes(g.Group("/movies"))
	AddSeriesRoutes(g.Group("/series"))
	AddBooksRoutes(g.Group("/books"))
	AddAudiobooksRoutes(g.Group("/audiobooks"))
	AddMusicRoutes(g.Group("/music"))

	param := regexp.MustCompile(`(:|\*)\w+|\{\w+\}`)

	// gin redirects a missing trailing slash, so "/api/movies" reaches "/api/movies/".
	key := func(method, path string) string {
		return method + " " + strings.TrimSuffix(param.ReplaceAllString(path, "{}"), "/")
	}

	registered := make(map[string]bool)
	for _, rt := range r.Routes() {
		registered[key(rt.Method, rt.Path)] = true
	}

	for _, list := range [][]apiEndpointDoc{
		apiEndpointsAll, apiEndpointsMovies, apiEndpointsSeries, apiEndpointsBooks,
		apiEndpointsAudiobooks, apiEndpointsMusic, apiEndpointsGeneral, apiEndpointsAdmin,
	} {
		for _, ep := range list {
			if !registered[key(ep.Method, ep.Path)] {
				t.Errorf("API testing page lists unregistered endpoint %s %s", ep.Method, ep.Path)
			}
		}
	}
}

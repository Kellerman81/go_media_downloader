package api

import (
	"strings"
	"testing"
)

func TestRenderMetadataResultsReportsLookupFailure(t *testing.T) {
	out := renderMetadataResults(map[string]any{
		"media_type": "movie",
		"title":      "",
		"error":      "No movie data found for the provided IMDB ID",
		"note":       "check the id",
	})

	if !strings.Contains(out, "No movie data found") || strings.Contains(out, "Movie Metadata Retrieved") {
		t.Fatalf("failure rendered as success: %s", out)
	}
}

package api

import (
	"slices"
	"testing"
)

func TestDropNestedPathJobs(t *testing.T) {
	type job struct {
		path string
		typ  uint
	}

	jobs := []job{{"/data/series", 1}, {"/data/series/anime", 1}, {"/data/series/books", 2}, {"/data/movies", 0}}

	got := dropNestedPathJobs(jobs, func(j job) (string, uint) { return j.path, j.typ })
	want := []job{{"/data/series", 1}, {"/data/series/books", 2}, {"/data/movies", 0}}

	if !slices.Equal(got, want) {
		t.Fatalf("dropNestedPathJobs = %v, want %v", got, want)
	}
}

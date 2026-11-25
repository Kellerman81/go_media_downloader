package database

import "testing"

// TestMediaExtras: saving replaces the cast and related titles of one
// title, keeps the order and stops at MaxStoredCast.
func TestMediaExtras(t *testing.T) {
	useUsersTestDB(t)

	cast := make([]MediaCast, MaxStoredCast+5)
	for i := range cast {
		cast[i] = MediaCast{PersonID: i + 1, Name: "Actor " + string(rune('A'+i)), Character: "Role"}
	}

	recs := []MediaRelated{{TMDB: 2, Title: "Two", Year: 2003}, {TMDB: 1, Title: "One", Year: 2001}}
	if err := SaveMediaExtras("movie", 7, cast, recs, []MediaRelated{{TMDB: 3, Title: "Three"}}); err != nil {
		t.Fatal(err)
	}

	if got := MediaCastOf("movie", 7); len(got) != MaxStoredCast || got[0].Name != "Actor A" || got[0].PersonID != 1 {
		t.Fatalf("cast: %d %+v", len(got), got[:1])
	}

	if got := MediaRelatedOf("movie", 7, RelationRecommended); len(got) != 2 || got[0].Title != "Two" || got[1].Year != 2001 {
		t.Fatalf("recommended: %+v", got)
	}

	// A refresh replaces everything; another kind with the same id is apart.
	if err := SaveMediaExtras("movie", 7, cast[:1], nil, nil); err != nil {
		t.Fatal(err)
	}

	if err := SaveMediaExtras("series", 7, cast[1:2], nil, nil); err != nil {
		t.Fatal(err)
	}

	if got := MediaCastOf("movie", 7); len(got) != 1 || len(MediaRelatedOf("movie", 7, RelationSimilar)) != 0 {
		t.Fatalf("replaced: %+v", got)
	}

	if got := MediaCastOf("series", 7); len(got) != 1 || got[0].Name != "Actor B" {
		t.Fatalf("series: %+v", got)
	}

	// Deleting the title removes its rows, the other kind's stay.
	DeleteMediaExtras("movie", "7")

	if len(MediaCastOf("movie", 7)) != 0 || len(MediaCastOf("series", 7)) != 1 {
		t.Fatal("delete")
	}
}

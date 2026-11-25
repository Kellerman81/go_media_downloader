package database

import (
	"maps"
	"testing"
)

// TestTranslations: fetched translations follow the source but leave a
// manual one alone, one equal to the original is not stored, and deleting
// the entry removes its translations.
func TestTranslations(t *testing.T) {
	useUsersTestDB(t)

	if err := ExecNErr("insert into dbmovies (id, title, overview) values (1, 'The Boat', 'A submarine.'), (2, 'Other', '')"); err != nil {
		t.Fatal(err)
	}

	SetTranslation("dbmovie", 1, "title", "de", "Das Boot", "The Boat")
	SetTranslation("dbmovie", 1, "overview", "de", "A submarine.", "A submarine.")
	SetTranslation("dbmovie", 2, "title", "de", "Anderer", "Other")

	got := TranslationsIn("dbmovie", "de", []uint{1, 2, 3})
	if want := map[string]string{"title": "Das Boot"}; !maps.Equal(got[1], want) || got[2]["title"] != "Anderer" || len(got) != 2 {
		t.Fatalf("translations %v", got)
	}

	// Edited by hand: a refresh doesn't change or remove it.
	if err := SetManualTranslation("dbmovie", 1, "title", "de", "Das Boot (1981)"); err != nil {
		t.Fatal(err)
	}

	SetTranslation("dbmovie", 1, "title", "de", "Das Boot", "The Boat")
	SetTranslation("dbmovie", 1, "title", "de", "", "The Boat")

	if tr := GetTranslations("dbmovie", 1); len(tr) != 1 || tr[0].Text != "Das Boot (1981)" || !tr[0].Manual {
		t.Errorf("manual translation %+v", tr)
	}

	// A fetched one the source no longer has goes.
	SetTranslation("dbmovie", 2, "title", "de", "", "Other")

	if tr := GetTranslations("dbmovie", 2); len(tr) != 0 {
		t.Errorf("translation kept after the source dropped it: %+v", tr)
	}

	if err := ExecNErr("delete from dbmovies where id = 1"); err != nil {
		t.Fatal(err)
	}

	if n := Getdatarow[int](false, "select count() from metadata_translations"); n != 0 {
		t.Errorf("%d translations left after deleting the movie", n)
	}

	if TranslationMediaType("dbserie_episodes") != "dbserie_episode" || TranslationMediaType("dbbook_series") != "dbbook_series" ||
		TranslationMediaType("albums") != "" {
		t.Error("TranslationMediaType")
	}
}

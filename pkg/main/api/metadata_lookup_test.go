package api

import (
	"errors"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestIMDbLocalTitle(t *testing.T) {
	restore, err := database.UseMigratedTestIMDB("../schema/imdbdb")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)

	for _, q := range []string{
		`insert into imdb_titles (tconst, title_type, primary_title, original_title, start_year, runtime_minutes, genres)
			values ('tt0959621', 'tvEpisode', 'Pilot', 'Pilot', 2008, 58, 'Crime,Drama,Thriller')`,
		`insert into imdb_ratings (tconst, num_votes, average_rating) values ('tt0959621', 50000, 9.0)`,
		`insert into imdb_akas (tconst, title, region) values ('tt0959621', 'Pilot', ''), ('tt0959621', 'Der Einstieg', 'DE')`,
	} {
		if _, err := database.Getdb(true).Exec(q); err != nil {
			t.Fatal(err)
		}
	}

	got, err := imdbLocalTitle(" tt0959621 ")
	if err != nil {
		t.Fatal(err)
	}

	if got.TitleType != "tvEpisode" || got.PrimaryTitle != "Pilot" || got.StartYear != 2008 || got.Rating != 9.0 ||
		got.Votes != 50000 || !slices.Equal(got.AlternativeTitles, []string{"Der Einstieg [DE]"}) {
		t.Errorf("got %+v", got)
	}

	if _, err := imdbLocalTitle("tt0000001"); !errors.Is(err, errMDNotInIMDb) {
		t.Errorf("unknown title: %v", err)
	}

	if _, err := imdbLocalTitle("81189"); !errors.Is(err, errMDBadID) {
		t.Errorf("non-IMDb ID: %v", err)
	}
}

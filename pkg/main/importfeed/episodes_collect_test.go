package importfeed

import (
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestWriteCollectedEpisodesToDB(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into dbseries (id, seriename) values (1, 'Show')"); err != nil {
		t.Fatal(err)
	}

	if err := database.ExecNErr(
		"insert into dbserie_episodes (dbserie_id, season, episode, identifier, title) values (1, '1', '1', 'S01E01', 'old')",
	); err != nil {
		t.Fatal(err)
	}

	var dbid uint = 1

	aired := time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC)
	writeCollectedEpisodesToDB(map[string]*collectedEpisode{
		episodeKey(1, 1): {Season: 1, Episode: 1, Title: "Pilot", FirstAired: aired},
		episodeKey(1, 2): {Season: 1, Episode: 2, Title: "Second", AbsoluteNumber: 2},
	}, &dbid)

	if got := database.Getdatarow[string](false, "select title from dbserie_episodes where dbserie_id = 1 and season = '1' and episode = '1'"); got != "Pilot" {
		t.Errorf("existing episode title = %q, want updated to Pilot", got)
	}

	if got := database.Getdatarow[string](false, "select identifier from dbserie_episodes where dbserie_id = 1 and episode = '2'"); got != "S01E02" {
		t.Errorf("inserted identifier = %q", got)
	}

	if got := database.Getdatarow[int](false, "select count() from dbserie_episodes where dbserie_id = 1 and episode = '2' and first_aired is null"); got != 1 {
		t.Error("unknown air date not stored as NULL")
	}

	if got := database.Getdatarow[int](false, "select count() from dbserie_episodes where dbserie_id = 1"); got != 2 {
		t.Errorf("episode rows = %d, want 2", got)
	}
}

func TestWriteCollectedEpisodesKeepsStoredValues(t *testing.T) {
	useTestDB(t)

	if err := database.ExecNErr("insert into dbseries (id, seriename) values (1, 'Show')"); err != nil {
		t.Fatal(err)
	}

	if err := database.ExecNErr(
		"insert into dbserie_episodes (dbserie_id, season, episode, identifier, title, overview, poster, first_aired, absolute_episode) " +
			"values (1, '0', '1', 'S00E01', 'Special', 'The stored plot', 'p.jpg', '2022-08-03 00:00:00+00:00', 7)",
	); err != nil {
		t.Fatal(err)
	}

	var dbid uint = 1

	// A refresh without overview, poster, date or absolute number (e.g. no
	// translation in the metadata language) must not blank them.
	writeCollectedEpisodesToDB(map[string]*collectedEpisode{
		episodeKey(0, 1): {Season: 0, Episode: 1, Title: "Special (new)"},
	}, &dbid)

	row := database.Getdatarow[string](false,
		"select title || '|' || overview || '|' || poster || '|' || substr(first_aired, 1, 10) || '|' || absolute_episode from dbserie_episodes where dbserie_id = 1")
	if want := "Special (new)|The stored plot|p.jpg|2022-08-03|7"; row != want {
		t.Errorf("row = %q, want %q", row, want)
	}

	// New values still replace the stored ones.
	writeCollectedEpisodesToDB(map[string]*collectedEpisode{
		episodeKey(0, 1): {Season: 0, Episode: 1, Overview: "New plot"},
	}, &dbid)

	if got := database.Getdatarow[string](false, "select overview || '|' || title from dbserie_episodes where dbserie_id = 1"); got != "New plot|Special (new)" {
		t.Errorf("after second refresh = %q", got)
	}
}

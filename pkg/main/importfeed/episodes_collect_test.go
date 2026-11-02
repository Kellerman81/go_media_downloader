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

	if got := database.Getdatarow[int](false, "select count() from dbserie_episodes where dbserie_id = 1"); got != 2 {
		t.Errorf("episode rows = %d, want 2", got)
	}
}

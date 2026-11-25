package api

import (
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/gin-gonic/gin"
)

// TestSeriesWantedMissing: the missing search of a series takes missing
// episodes that may be searched, specials only with search_specials.
func TestSeriesWantedMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	libraryTestData(t)

	for _, s := range []string{
		"insert into dbserie_episodes (id, dbserie_id, identifier, season, episode) values (2, 1, 'S01E02', '1', '2'), (3, 1, 'S01E03', '1', '3'), (4, 1, 'S00E01', '0', '1')",
		"insert into serie_episodes (id, serie_id, dbserie_id, dbserie_episode_id, missing, dont_search) values (2, 1, 1, 2, 0, 0), (3, 1, 1, 3, 1, 1), (4, 1, 1, 4, 1, 0)",
	} {
		if err := database.ExecNErr(s); err != nil {
			t.Fatal(err)
		}
	}

	ids := func() []uint {
		var id uint = 1
		return database.GetrowsN[uint](false, 10, "select id from "+seriesWantedMissing+" order by id", &id)
	}

	if got := ids(); !slices.Equal(got, []uint{1}) {
		t.Errorf("without specials: %v, want [1]", got)
	}

	if err := database.ExecNErr("update series set search_specials = 1 where id = 1"); err != nil {
		t.Fatal(err)
	}

	if got := ids(); !slices.Equal(got, []uint{1, 4}) {
		t.Errorf("with specials: %v, want [1 4]", got)
	}
}

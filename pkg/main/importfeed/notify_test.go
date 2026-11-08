package importfeed

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

// csvNotifyConfig returns a media config whose event notifications append
// "{{.Event}};{{.Type}};{{.Title}};{{.Year}};{{.Identifier}};{{.List}}" to a
// csv file, and that file.
func csvNotifyConfig(t *testing.T, isType uint, events ...string) (*config.MediaTypeConfig, string) {
	t.Helper()

	file := filepath.Join(t.TempDir(), "events.csv")
	csv := &config.NotificationConfig{Name: "csv", NotificationType: "csv", Outputto: file}

	cfgp := &config.MediaTypeConfig{Name: "EN", IsType: isType}
	for _, e := range events {
		cfgp.Notification = append(cfgp.Notification, config.MediaNotificationConfig{
			Event: e, CfgNotification: csv,
			Message: "{{.Event}};{{.Type}};{{.Title}};{{.Year}};{{.Identifier}};{{.List}}",
		})
	}

	return cfgp, file
}

func readFile(t *testing.T, file string) string {
	t.Helper()

	b, err := os.ReadFile(file)
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}

	return string(b)
}

func TestNotifyAdded(t *testing.T) {
	useTestDB(t)

	for _, q := range []string{
		"insert into dbmovies (id, title, year, imdb_id) values (1, 'Heat', 1995, 'tt0113277')",
		"insert into dbseries (id, seriename, firstaired, thetvdb_id) values (2, 'Dark', '2017-12-01', 334824)",
		"insert into dbbooks (id, title, year, isbn_13) values (3, 'Dune', 1965, '9780441013593')",
		"insert into dbaudiobooks (id, title, year, asin) values (4, 'Mythos', 2017, 'B0000')",
		"insert into dbalbums (id, title, year, musicbrainz_release_id) values (5, 'The Wall', 1979, 'mb-1')",
	} {
		if err := database.ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		isType uint
		dbid   uint
		want   string
	}{
		{config.MediaTypeMovie, 1, "added_to_list;movie;Heat;1995;tt0113277;L\n"},
		{config.MediaTypeSeries, 2, "added_to_list;serie;Dark;2017;334824;L\n"},
		{config.MediaTypeBook, 3, "added_to_list;book;Dune;1965;9780441013593;L\n"},
		{config.MediaTypeAudiobook, 4, "added_to_list;audiobook;Mythos;2017;B0000;L\n"},
		{config.MediaTypeMusic, 5, "added_to_list;album;The Wall;1979;mb-1;L\n"},
	} {
		cfgp, file := csvNotifyConfig(t, tc.isType, providers.EventAddedToList)

		NotifyAdded(cfgp, providers.EventAddedToList, "L", tc.dbid)
		NotifyAdded(cfgp, providers.EventAddedToDB, "", tc.dbid) // not subscribed
		NotifyAdded(cfgp, providers.EventAddedToList, "L", 99)   // no such row

		if got := readFile(t, file); got != tc.want {
			t.Errorf("type %d: csv %q, want %q", tc.isType, got, tc.want)
		}
	}
}

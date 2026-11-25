package database

import "testing"

// TestUserMediaRequests: the requests of one account, open or waiting for
// their media, newest first.
func TestUserMediaRequests(t *testing.T) {
	useUsersTestDB(t)

	ExecN("insert into media_requests (username, kind, entry_id, title, status) values " +
		"('bea', 'movie', 1, 'first', 'approved'), ('cid', 'movie', 2, 'other', 'open'), ('Bea', 'album', 3, 'second', 'open')")

	got := UserMediaRequests("bea")
	if len(got) != 2 || got[0].Title != "second" || got[0].Status != "open" || got[1].Status != "approved" {
		t.Errorf("requests of bea: %+v", got)
	}
}

package database

import "testing"

// TestUserNotifications: messages per account, newest first, dismissed
// one by one or all; an approved request waits for its media, then is
// taken once.
func TestUserNotifications(t *testing.T) {
	useUsersTestDB(t)

	for _, title := range []string{"one", "two", "three"} {
		if err := AddUserNotification("bea", title, "msg", ""); err != nil {
			t.Fatal(err)
		}
	}

	if err := AddUserNotification("cid", "other", "", ""); err != nil {
		t.Fatal(err)
	}

	open := OpenUserNotifications("bea")
	if len(open) != 3 || open[0].Title != "three" {
		t.Fatalf("open: %+v", open)
	}

	if err := DismissUserNotification("bea", &open[0].ID); err != nil || len(OpenUserNotifications("bea")) != 2 {
		t.Errorf("after one dismiss: %v", err)
	}

	// Another account's message can't be dismissed.
	other := OpenUserNotifications("cid")[0].ID
	if err := DismissUserNotification("bea", &other); err != nil || len(OpenUserNotifications("cid")) != 1 {
		t.Errorf("dismissed another account's message: %v", err)
	}

	if err := DismissUserNotification("bea", new(uint(0))); err != nil || len(OpenUserNotifications("bea")) != 0 {
		t.Errorf("after dismiss all: %v", err)
	}

	// Requests: approved ones leave the open list and are taken once.
	for _, q := range []string{
		"insert into dbmovies (id, title) values (1, 'Film')",
		"insert into movies (id, dbmovie_id, listname, dont_search) values (5, 1, 'L', 1)",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	r := MediaRequest{Username: "bea", Kind: "movie", EntryID: 5, Listname: "L", Title: "Film"}
	if err := HoldMediaRequest(&r); err != nil {
		t.Fatal(err)
	}

	if err := ApproveMediaRequest(&r, "", ""); err != nil || CountMediaRequests() != 0 {
		t.Fatalf("approve: %v, %d open", err, CountMediaRequests())
	}

	if got := TakeApprovedRequests("movie", 5); len(got) != 1 || got[0].Username != "bea" {
		t.Errorf("taken: %+v", got)
	}

	if got := TakeApprovedRequests("movie", 5); len(got) != 0 {
		t.Errorf("taken twice: %+v", got)
	}
}

// TestUserNotificationsTitled: dismissed messages count too; only titles
// with one of the prefixes.
func TestUserNotificationsTitled(t *testing.T) {
	useUsersTestDB(t)

	for _, title := range []string{"Request approved: A", "Report handled: B", "Now available: A", "Request declined: C"} {
		if err := AddUserNotification("bea", title, "msg", ""); err != nil {
			t.Fatal(err)
		}
	}

	if err := DismissUserNotification("bea", new(uint)); err != nil {
		t.Fatal(err)
	}

	got := UserNotificationsTitled("bea", "Request ", "Now available: ")
	if len(got) != 3 || got[0].Title != "Request declined: C" || got[2].Title != "Request approved: A" {
		t.Errorf("request messages: %+v", got)
	}

	if got := UserNotificationsTitled("bea"); got != nil {
		t.Errorf("without prefixes: %+v", got)
	}
}

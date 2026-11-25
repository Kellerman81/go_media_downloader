package database

import (
	"errors"
	"testing"
)

// TestPlaylists: names are unique per account, others see only shared
// lists and can't change them, items keep their order without duplicates,
// move and go away with their list.
func TestPlaylists(t *testing.T) {
	useUsersTestDB(t)

	mine := Playlist{Username: "bea", Name: " Evening "}
	if err := SavePlaylist(&mine); err != nil || mine.ID == 0 || mine.Name != "Evening" {
		t.Fatalf("create: %+v, %v", mine, err)
	}

	if err := SavePlaylist(&Playlist{Username: "bea", Name: "evening"}); !errors.Is(err, ErrPlaylistExists) {
		t.Errorf("duplicate name: %v", err)
	}

	if err := SavePlaylist(&Playlist{Username: "bea", Name: "  "}); !errors.Is(err, ErrPlaylistName) {
		t.Errorf("empty name: %v", err)
	}

	items := []PlaylistItem{
		{Kind: "movie", EntryID: 1, Title: "A"},
		{Kind: "episode", EntryID: 2, ItemID: 7, Title: "B"},
		{Kind: "track", EntryID: 3, ItemID: 9, RefID: 4, Title: "C"},
		{Kind: "movie", EntryID: 1, Title: "A again"},
		{Kind: "book", EntryID: 5, Title: "not playable"},
	}

	if n, err := AddPlaylistItems("bea", &mine.ID, items); err != nil || n != 3 {
		t.Fatalf("added %d, %v", n, err)
	}

	if _, err := AddPlaylistItems("cid", &mine.ID, items); !errors.Is(err, ErrPlaylistMissing) {
		t.Errorf("cid added to bea's list: %v", err)
	}

	got := GetPlaylistItems(&mine.ID)
	if len(got) != 3 || got[0].Title != "A" || got[2].RefID != 4 {
		t.Fatalf("items: %+v", got)
	}

	if err := MovePlaylistItem("bea", &mine.ID, &got[2].ID, -1); err != nil {
		t.Fatal(err)
	}

	if got := GetPlaylistItems(&mine.ID); got[1].Title != "C" || got[2].Title != "B" {
		t.Errorf("after moving C up: %+v", got)
	}

	if _, err := GetPlaylist("cid", &mine.ID); err == nil {
		t.Error("cid sees an unshared list")
	}

	mine.Shared = true
	if err := SavePlaylist(&mine); err != nil {
		t.Fatal(err)
	}

	if p, err := GetPlaylist("cid", &mine.ID); err != nil || p.Items != 3 || p.Username != "bea" {
		t.Errorf("shared list for cid: %+v, %v", p, err)
	}

	if lists := GetPlaylists("cid"); len(lists) != 1 {
		t.Errorf("lists of cid: %+v", lists)
	}

	if err := RemovePlaylistItem("bea", &mine.ID, &got[0].ID); err != nil || len(GetPlaylistItems(&mine.ID)) != 2 {
		t.Errorf("remove: %v", err)
	}

	if err := DeletePlaylist("cid", &mine.ID); !errors.Is(err, ErrPlaylistMissing) {
		t.Errorf("cid deleted bea's list: %v", err)
	}

	if err := DeletePlaylist("bea", &mine.ID); err != nil || len(GetPlaylistItems(&mine.ID)) != 0 || len(GetPlaylists("bea")) != 0 {
		t.Errorf("delete: %v", err)
	}
}

package database

import (
	"errors"
	"testing"
)

func useUsersTestDB(t *testing.T) {
	t.Helper()

	restore, err := UseMigratedTestDB("../schema/db")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(restore)
}

// TestUserFeaturesAndSettings: features are kept known and in order, the
// built-in admin's settings row is created on first save, can't log in and
// isn't listed, and its name stays reserved.
func TestUserFeaturesAndSettings(t *testing.T) {
	useUsersTestDB(t)

	u := User{Username: "bea", Role: RoleUser, Features: "download, bogus,requestdelete"}
	if err := InsertUser(&u, "password123"); err != nil {
		t.Fatal(err)
	}

	got, err := GetUserByName(&u.Username)
	if err != nil || got.Features != "download,requestdelete" || got.Settings != "{}" {
		t.Fatalf("stored user %+v, %v", got, err)
	}

	if err := SaveUserSettings("bea", `{"lp-vol":"0.5"}`); err != nil || GetUserSettings("bea") != `{"lp-vol":"0.5"}` {
		t.Fatalf("settings of bea: %q, %v", GetUserSettings("bea"), err)
	}

	if GetUserSettings(SystemAdmin) != "{}" {
		t.Fatalf("admin settings before saving: %q", GetUserSettings(SystemAdmin))
	}

	if err := SaveUserSettings(SystemAdmin, `{"lib-page-size":"50"}`); err != nil || GetUserSettings(SystemAdmin) != `{"lib-page-size":"50"}` {
		t.Fatalf("admin settings: %q, %v", GetUserSettings(SystemAdmin), err)
	}

	admin, _ := GetUserByName(new(SystemAdmin))
	if CheckPassword(admin.PasswordHash, "!") || CheckPassword(admin.PasswordHash, "") {
		t.Error("the admin settings row accepts a password")
	}

	if users := GetUsers(); len(users) != 1 || users[0].Username != "bea" {
		t.Errorf("listed users: %+v", users)
	}

	if err := InsertUser(&User{Username: "Admin", Role: RoleUser}, "password123"); !errors.Is(err, ErrUserReserved) {
		t.Errorf("account named Admin: %v", err)
	}
}

// TestUserActivities: plays, open reports and requests are counted per
// account, whatever case its name was stored in.
func TestUserActivities(t *testing.T) {
	useUsersTestDB(t)

	for _, h := range []PlayHistory{
		{Username: "Bea", Kind: "movie", FileID: 1, Duration: 100, Position: 50},
		{Username: "bea", Kind: "track", FileID: 2, Duration: 100, Position: 50},
		{Username: "cid", Kind: "movie", FileID: 1, Duration: 100, Position: 50},
	} {
		if err := PlayProgress(&h, true); err != nil {
			t.Fatal(err)
		}
	}

	if err := AddMediaReport(&MediaReport{Username: "bea", Kind: "movie", FileID: 1, Reason: "quality"}); err != nil {
		t.Fatal(err)
	}

	ExecN("insert into media_requests (username, kind, entry_id, title) values ('BEA', 'movie', 3, 'x'), ('bea', 'album', 4, 'y')")

	got := UserActivities()
	if got["bea"] != (UserActivity{Plays: 2, Reports: 1, Requests: 2}) || got["cid"] != (UserActivity{Plays: 1}) {
		t.Errorf("activities: %+v", got)
	}
}

// TestSetUserSetting: one value changes, the others stay; the built-in
// admin's row is created on first use; a broken JSON starts over.
func TestSetUserSetting(t *testing.T) {
	useUsersTestDB(t)

	if err := InsertUser(&User{Username: "bea", Role: RoleUser}, "password123"); err != nil {
		t.Fatal(err)
	}

	if err := SaveUserSettings("bea", `{"lp-vol":"0.5"}`); err != nil {
		t.Fatal(err)
	}

	if err := SetUserSetting("bea", "theme", "dark"); err != nil || GetUserSettings("bea") != `{"lp-vol":"0.5","theme":"dark"}` {
		t.Fatalf("settings of bea: %s, %v", GetUserSettings("bea"), err)
	}

	if err := SetUserSetting(SystemAdmin, "theme", "auto"); err != nil || GetUserSettings(SystemAdmin) != `{"theme":"auto"}` {
		t.Fatalf("admin settings: %s, %v", GetUserSettings(SystemAdmin), err)
	}

	ExecN("update users set settings = 'broken' where username = 'bea'")

	if err := SetUserSetting("bea", "theme", "light"); err != nil || GetUserSettings("bea") != `{"theme":"light"}` {
		t.Errorf("after broken settings: %s, %v", GetUserSettings("bea"), err)
	}
}

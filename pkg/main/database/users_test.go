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

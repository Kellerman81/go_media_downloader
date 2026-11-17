package database

import (
	"errors"
	"slices"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

// Web UI roles. Every role may view and play the media groups it is given;
// admins see everything and reach every page.
const (
	RoleAdmin          = "admin"
	RoleUser           = "user"           // view and play
	RoleSearchingUser  = "searchinguser"  // + search and download
	RoleRequestingUser = "requestinguser" // + request new media
)

// Roles in the order the user page offers them.
var Roles = []string{RoleUser, RoleSearchingUser, RoleRequestingUser, RoleAdmin}

// Features an account may have besides viewing and playing (admins have
// all of them).
const (
	FeatureSearch        = "search"        // start searches and downloads
	FeatureRequest       = "request"       // request new media
	FeatureDownload      = "download"      // download media files
	FeatureRequestDelete = "requestdelete" // report files as wrong
)

// Features in the order the user page offers them.
var Features = []string{FeatureSearch, FeatureRequest, FeatureDownload, FeatureRequestDelete}

// RoleFeatures are the features a role pre-sets; each can then be switched
// per account.
var RoleFeatures = map[string][]string{
	RoleUser:           nil,
	RoleSearchingUser:  {FeatureSearch},
	RoleRequestingUser: {FeatureRequest},
	RoleAdmin:          Features,
}

// SystemAdmin is the row holding the built-in admin login's settings: it
// can't log in (no password matches "!") and isn't listed.
const SystemAdmin = "admin"

var (
	ErrUserExists   = errors.New("a user with this name already exists")
	ErrUserInvalid  = errors.New("username and role are required")
	ErrUserPassword = errors.New("the password needs at least 8 characters")
	ErrUserReserved = errors.New(`the name "admin" belongs to the built-in admin login`)
)

// User is a web UI account (the users table). The built-in admin login is
// not one.
type User struct {
	ID           uint   `db:"id"`
	Username     string `db:"username"`
	PasswordHash string `db:"password_hash"`
	Role         string `db:"role"`
	MediaGroups  string `db:"media_groups"`
	Features     string `db:"features"` // comma separated Feature*
	Settings     string `db:"settings"` // JSON object
	CreatedAt    string `db:"created_at"`
	UpdatedAt    string `db:"updated_at"`
	LastLogin    string `db:"last_login"`
}

// Groups are the media configs the user may see.
func (u *User) Groups() []string {
	var out []string

	for g := range strings.SplitSeq(u.MediaGroups, ",") {
		if g = strings.TrimSpace(g); g != "" {
			out = append(out, g)
		}
	}

	return out
}

// FeatureList are the features of the account (Feature*).
func (u *User) FeatureList() []string {
	var out []string

	for f := range strings.SplitSeq(u.Features, ",") {
		if f = strings.TrimSpace(f); slices.Contains(Features, f) {
			out = append(out, f)
		}
	}

	return out
}

const userColumns = "id, username, password_hash, role, media_groups, features, settings, created_at, updated_at, " +
	"coalesce(last_login, '') as last_login"

// GetUsers returns all accounts by name (not the built-in admin's row).
func GetUsers() []User {
	return StructscanT[User](false, 0, "select "+userColumns+" from users where not (username = ? and password_hash = '!') "+
		"order by username collate nocase", SystemAdmin)
}

// GetUserByName returns the account with this name (case-insensitive).
func GetUserByName(name *string) (*User, error) {
	return Structscan[User]("select "+userColumns+" from users where username = ?", false, name)
}

// GetUserByID returns the account with this id.
func GetUserByID(id *uint) (*User, error) {
	return Structscan[User]("select "+userColumns+" from users where id = ?", false, id)
}

// HashPassword returns the bcrypt hash of a password of at least 8
// characters.
func HashPassword(password string) (string, error) {
	if len(password) < 8 {
		return "", ErrUserPassword
	}

	h, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(h), nil
}

// CheckPassword reports whether password belongs to the hash.
func CheckPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func validUser(u *User) error {
	u.Username = strings.TrimSpace(u.Username)
	if u.Username == "" || !slices.Contains(Roles, u.Role) {
		return ErrUserInvalid
	}

	if strings.EqualFold(u.Username, SystemAdmin) {
		return ErrUserReserved
	}

	u.Features = strings.Join(u.FeatureList(), ",")

	return nil
}

// InsertUser creates an account with the given password.
func InsertUser(u *User, password string) error {
	if err := validUser(u); err != nil {
		return err
	}

	if existing, err := GetUserByName(&u.Username); err == nil && existing.ID != 0 {
		return ErrUserExists
	}

	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	id, err := ExecNid("insert into users (username, password_hash, role, media_groups, features) values (?, ?, ?, ?, ?)",
		&u.Username, &hash, &u.Role, &u.MediaGroups, &u.Features)
	if err != nil {
		return err
	}

	u.ID, u.PasswordHash = uint(id), hash //nolint:gosec // row ids are positive

	return nil
}

// UpdateUser saves name, role, media groups and features of an account.
func UpdateUser(u *User) error {
	if err := validUser(u); err != nil {
		return err
	}

	if existing, err := GetUserByName(&u.Username); err == nil && existing.ID != 0 && existing.ID != u.ID {
		return ErrUserExists
	}

	return ExecNErr("update users set username = ?, role = ?, media_groups = ?, features = ?, updated_at = datetime('now','localtime') where id = ?",
		&u.Username, &u.Role, &u.MediaGroups, &u.Features, &u.ID)
}

// SetUserPassword replaces the password of an account.
func SetUserPassword(id *uint, password string) error {
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}

	return ExecNErr("update users set password_hash = ?, updated_at = datetime('now','localtime') where id = ?", &hash, id)
}

// DeleteUser removes an account.
func DeleteUser(id *uint) error {
	return ExecNErr("delete from users where id = ?", id)
}

// TouchUserLogin records a successful login.
func TouchUserLogin(id *uint) {
	ExecN("update users set last_login = datetime('now','localtime') where id = ?", id)
}

// GetUserSettings returns the settings JSON of an account ("{}" without
// any); the built-in admin's are in the SystemAdmin row.
func GetUserSettings(name string) string {
	s := Getdatarow[string](false, "select settings from users where username = ?", name)
	if s == "" {
		return "{}"
	}

	return s
}

// SaveUserSettings stores the settings JSON of an account, creating the
// built-in admin's row on first use.
func SaveUserSettings(name, settings string) error {
	if strings.EqualFold(name, SystemAdmin) {
		if err := ExecNErr("insert or ignore into users (username, password_hash, role) values (?, '!', ?)",
			SystemAdmin, RoleAdmin); err != nil {
			return err
		}
	}

	return ExecNErr("update users set settings = ?, updated_at = datetime('now','localtime') where username = ?", settings, name)
}

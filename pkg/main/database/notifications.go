package database

import "strings"

// UserNotification is a message to an account (user_notifications): its
// request was approved or rejected, the requested media arrived, its report
// was handled.
type UserNotification struct {
	ID        uint   `db:"id"           json:"id"`
	Title     string `db:"title"        json:"title"`
	Message   string `db:"message"      json:"message"`
	Link      string `db:"link"         json:"link"`
	CreatedAt string `db:"created_at"   json:"created"`
}

// AddUserNotification records a message for an account.
func AddUserNotification(username, title, message, link string) error {
	return ExecNErr("insert into user_notifications (username, title, message, link) values (?, ?, ?, ?)",
		username, title, message, link)
}

// OpenUserNotifications are the account's messages it hasn't dismissed,
// newest first.
func OpenUserNotifications(username string) []UserNotification {
	return StructscanT[UserNotification](false, 8, "select id, title, message, link, "+
		"strftime('%Y-%m-%d %H:%M', created_at) as created_at from user_notifications "+
		"where username = ? and dismissed_at is null order by id desc limit 50", username)
}

// UserNotificationsTitled are the account's messages (dismissed or not)
// whose title starts with one of prefixes, newest first: what became of its
// requests and reports after they were handled.
func UserNotificationsTitled(username string, prefixes ...string) []UserNotification {
	if len(prefixes) == 0 {
		return nil
	}

	args := make([]any, 0, len(prefixes)+1)
	args = append(args, username)
	cond := make([]string, len(prefixes))

	for i, p := range prefixes {
		cond[i] = "title like ?"
		args = append(args, p+"%")
	}

	return StructscanT[UserNotification](false, 8, "select id, title, message, link, "+
		"strftime('%Y-%m-%d %H:%M', created_at) as created_at from user_notifications "+
		"where username = ? and ("+strings.Join(cond, " or ")+") order by id desc limit 100", args...)
}

// DismissUserNotification closes one message of the account (id 0: all).
func DismissUserNotification(username string, id *uint) error {
	if *id == 0 {
		return ExecNErr("update user_notifications set dismissed_at = datetime('now','localtime') "+
			"where username = ? and dismissed_at is null", username)
	}

	return ExecNErr("update user_notifications set dismissed_at = datetime('now','localtime') where username = ? and id = ?",
		username, id)
}

// GetUserEmail is the address an account gets update mails at ("" for none).
func GetUserEmail(username string) string {
	return Getdatarow[string](false, "select email from users where username = ?", username)
}

// SetUserEmail stores the address an account gets update mails at.
func SetUserEmail(username, email string) error {
	return ExecNErr("update users set email = ?, updated_at = datetime('now','localtime') where username = ?", email, username)
}

// TakeApprovedRequests returns the approved requests of a list entry and
// closes them: its media arrived.
func TakeApprovedRequests(kind string, entryID uint) []MediaRequest {
	reqs := StructscanT[MediaRequest](false, 2, "select id, username, kind, entry_id, listname, title, created_at "+
		"from media_requests where status = 'approved' and kind = ? and entry_id = ?", kind, entryID)

	for i := range reqs {
		CloseMediaRequest(&reqs[i].ID)
	}

	return reqs
}

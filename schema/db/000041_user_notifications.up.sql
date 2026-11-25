-- email: an account's address for update mails (optional, set on My
-- Account); mails go out through the [[notification]] named in the general
-- setting user_mail_notification.
ALTER TABLE users ADD COLUMN email TEXT NOT NULL DEFAULT '';

-- What an account is told in the web interface (and by mail): a request
-- approved or rejected, a requested title downloaded, a report handled.
-- dismissed_at: the account closed it.
CREATE TABLE IF NOT EXISTS user_notifications (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	username TEXT NOT NULL COLLATE NOCASE,
	title TEXT NOT NULL DEFAULT '',
	message TEXT NOT NULL DEFAULT '',
	link TEXT NOT NULL DEFAULT '',
	created_at DATETIME NOT NULL DEFAULT (datetime('now','localtime')),
	dismissed_at DATETIME
);
CREATE INDEX IF NOT EXISTS idx_user_notifications_open ON user_notifications (username, dismissed_at);

-- open: waits for an admin; approved: searched, the account is told once
-- its media was downloaded (then the row goes).
ALTER TABLE media_requests ADD COLUMN status TEXT NOT NULL DEFAULT 'open';

-- resumed: the play continued an unfinished one (not counted as a new
-- play in the statistics).
ALTER TABLE play_sessions ADD COLUMN resumed INTEGER NOT NULL DEFAULT 0;

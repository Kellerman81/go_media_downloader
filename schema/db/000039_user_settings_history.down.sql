DROP TABLE IF EXISTS media_reports;
DROP TABLE IF EXISTS play_history;
DELETE FROM users WHERE username = 'admin' AND password_hash = '!';
ALTER TABLE users DROP COLUMN features;
ALTER TABLE users DROP COLUMN settings;

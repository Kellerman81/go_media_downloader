ALTER TABLE play_sessions DROP COLUMN resumed;
DELETE FROM media_requests WHERE status <> 'open';
ALTER TABLE media_requests DROP COLUMN status;
DROP TABLE IF EXISTS user_notifications;
ALTER TABLE users DROP COLUMN email;

-- Adds created_at/updated_at to media_cast and media_related (SQLite can't
-- add a column with a current_timestamp default, so the tables are rebuilt).
CREATE TABLE "media_cast_new" (
	"id" integer PRIMARY KEY,
	"created_at" datetime NOT NULL DEFAULT current_timestamp,
	"updated_at" datetime NOT NULL DEFAULT current_timestamp,
	"kind" text NOT NULL,
	"db_id" integer NOT NULL,
	"position" integer NOT NULL,
	"person_id" integer NOT NULL DEFAULT 0,
	"name" text NOT NULL,
	"character" text NOT NULL DEFAULT '',
	"profile" text NOT NULL DEFAULT '',
	UNIQUE ("kind", "db_id", "position")
);
INSERT INTO media_cast_new (id, kind, db_id, position, person_id, name, character, profile) SELECT id, kind, db_id, position, person_id, name, character, profile FROM media_cast;
DROP TABLE media_cast;
ALTER TABLE media_cast_new RENAME TO media_cast;
CREATE INDEX IF NOT EXISTS "idx_media_cast_name" ON "media_cast" ("name" COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS "idx_media_cast_person" ON "media_cast" ("kind", "person_id");
CREATE TRIGGER IF NOT EXISTS tg_media_cast_updated_at AFTER UPDATE ON media_cast FOR EACH ROW BEGIN UPDATE media_cast SET updated_at = current_timestamp WHERE id = old.id; END;

CREATE TABLE "media_related_new" (
	"id" integer PRIMARY KEY,
	"created_at" datetime NOT NULL DEFAULT current_timestamp,
	"updated_at" datetime NOT NULL DEFAULT current_timestamp,
	"kind" text NOT NULL,
	"db_id" integer NOT NULL,
	"relation" text NOT NULL,
	"position" integer NOT NULL,
	"tmdb_id" integer NOT NULL,
	"title" text NOT NULL DEFAULT '',
	"year" integer NOT NULL DEFAULT 0,
	"rating" real NOT NULL DEFAULT 0,
	"poster" text NOT NULL DEFAULT '',
	UNIQUE ("kind", "db_id", "relation", "position")
);
INSERT INTO media_related_new (id, kind, db_id, relation, position, tmdb_id, title, year, rating, poster) SELECT id, kind, db_id, relation, position, tmdb_id, title, year, rating, poster FROM media_related;
DROP TABLE media_related;
ALTER TABLE media_related_new RENAME TO media_related;
CREATE TRIGGER IF NOT EXISTS tg_media_related_updated_at AFTER UPDATE ON media_related FOR EACH ROW BEGIN UPDATE media_related SET updated_at = current_timestamp WHERE id = old.id; END;

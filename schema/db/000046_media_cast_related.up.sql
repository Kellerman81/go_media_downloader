-- Cast and related titles (TMDB recommendations and similar titles) of a
-- movie (kind 'movie', dbmovies id) or series (kind 'series', dbseries id),
-- saved by the metadata import and replaced on every refresh.
CREATE TABLE IF NOT EXISTS "media_cast" (
	"id" integer PRIMARY KEY,
	"kind" text NOT NULL,
	"db_id" integer NOT NULL,
	"position" integer NOT NULL,
	"person_id" integer NOT NULL DEFAULT 0,
	"name" text NOT NULL,
	"character" text NOT NULL DEFAULT '',
	"profile" text NOT NULL DEFAULT '',
	UNIQUE ("kind", "db_id", "position")
);
CREATE INDEX IF NOT EXISTS "idx_media_cast_name" ON "media_cast" ("name" COLLATE NOCASE);
CREATE INDEX IF NOT EXISTS "idx_media_cast_person" ON "media_cast" ("kind", "person_id");

CREATE TABLE IF NOT EXISTS "media_related" (
	"id" integer PRIMARY KEY,
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

CREATE TABLE "imdb_titles" (
	"tconst"	text NOT NULL,
	"title_type"	text DEFAULT "",
	"primary_title"	text DEFAULT "",
	"slug"	text DEFAULT "",
	"original_title"	text DEFAULT "",
	"is_adult"	numeric,
	"start_year"	integer,
	"end_year"	integer,
	"runtime_minutes"	integer,
	"genres"	text DEFAULT ""
);
CREATE TABLE "imdb_ratings" (
	"id"	integer,
	"created_at"	datetime NOT NULL DEFAULT current_timestamp,
	"updated_at"	datetime NOT NULL DEFAULT current_timestamp,
	"tconst"	text DEFAULT "",
	"num_votes"	integer,
	"average_rating"	real,
	PRIMARY KEY("id")
);
CREATE TABLE "imdb_genres" (
	"id"	integer,
	"created_at"	datetime NOT NULL DEFAULT current_timestamp,
	"updated_at"	datetime NOT NULL DEFAULT current_timestamp,
	"tconst"	text DEFAULT "",
	"genre"	text DEFAULT "",
	PRIMARY KEY("id")
);
CREATE TABLE "imdb_akas" (
	"id"	integer,
	"created_at"	datetime NOT NULL DEFAULT current_timestamp,
	"updated_at"	datetime NOT NULL DEFAULT current_timestamp,
	"tconst"	text DEFAULT "",
	"ordering"	integer,
	"title"	text DEFAULT "",
	"slug"	text DEFAULT "",
	"region"	text DEFAULT "",
	"language"	text DEFAULT "",
	"types"	text DEFAULT "",
	"attributes"	text DEFAULT "",
	"is_original_title"	numeric,
	PRIMARY KEY("id")
);
CREATE INDEX "idx_imdb_titles_slug" ON "imdb_titles" (
	"slug"
);
CREATE INDEX "idx_imdb_titles_primary_title" ON "imdb_titles" (
	"primary_title"
);
CREATE UNIQUE INDEX "idx_imdb_titles_tconst" ON "imdb_titles" (
	"tconst"
);
CREATE INDEX "idx_imdb_akas_slug" ON "imdb_akas" (
	"slug"
);
CREATE INDEX "idx_imdb_akas_title" ON "imdb_akas" (
	"title"
);
CREATE TRIGGER tg_imdb_akas_updated_at
AFTER UPDATE
ON imdb_akas FOR EACH ROW
BEGIN
  UPDATE imdb_akas SET updated_at = current_timestamp
    WHERE id = old.id;
END;
CREATE TRIGGER tg_imdb_ratings_updated_at
AFTER UPDATE
ON imdb_ratings FOR EACH ROW
BEGIN
  UPDATE imdb_ratings SET updated_at = current_timestamp
    WHERE id = old.id;
END;
CREATE TRIGGER tg_imdb_genres_updated_at
AFTER UPDATE
ON imdb_genres FOR EACH ROW
BEGIN
  UPDATE imdb_genres SET updated_at = current_timestamp
    WHERE id = old.id;
END;
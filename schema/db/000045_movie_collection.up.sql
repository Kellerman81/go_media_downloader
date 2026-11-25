-- The TMDB collection (film series) of a movie, saved by the metadata
-- import: NULL not looked up yet, 0 in no collection.
ALTER TABLE "dbmovies" ADD COLUMN "collection_id" integer;
ALTER TABLE "dbmovies" ADD COLUMN "collection_name" text NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS "idx_dbmovies_collection_id" ON "dbmovies" ("collection_id") WHERE "collection_id" > 0;

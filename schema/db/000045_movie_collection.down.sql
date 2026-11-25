DROP INDEX IF EXISTS "idx_dbmovies_collection_id";
ALTER TABLE "dbmovies" DROP COLUMN "collection_name";
ALTER TABLE "dbmovies" DROP COLUMN "collection_id";

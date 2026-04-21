ALTER TABLE dbseries ADD COLUMN aliases text DEFAULT "";
UPDATE dbseries SET aliases = COALESCE((SELECT aliases FROM series WHERE series.dbserie_id = dbseries.id LIMIT 1), "");
ALTER TABLE series DROP COLUMN aliases;

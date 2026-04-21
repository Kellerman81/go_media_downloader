ALTER TABLE series ADD COLUMN aliases text DEFAULT "";
UPDATE series SET aliases = COALESCE((SELECT aliases FROM dbseries WHERE dbseries.id = series.dbserie_id), "");
ALTER TABLE dbseries DROP COLUMN aliases;

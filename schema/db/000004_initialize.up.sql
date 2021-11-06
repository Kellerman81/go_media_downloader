ALTER TABLE dbserie_episodes DROP COLUMN first_aired;
ALTER TABLE dbmovies DROP COLUMN release_date;
ALTER TABLE dbserie_episodes ADD COLUMN first_aired datetime;
ALTER TABLE dbmovies ADD COLUMN release_date datetime;
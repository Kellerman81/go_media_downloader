-- Add absolute_episode column to dbserie_episodes table
ALTER TABLE dbserie_episodes ADD COLUMN absolute_episode integer DEFAULT 0;

-- Create index for absolute_episode lookups
CREATE INDEX IF NOT EXISTS idx_dbserie_episodes_absolute ON dbserie_episodes(dbserie_id, absolute_episode);

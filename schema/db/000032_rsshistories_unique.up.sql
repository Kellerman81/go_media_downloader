-- Deduplicate r_sshistories (keep the newest row per config/list/indexer,
-- case-insensitive) so a unique index can be created. The unique index
-- enables atomic upserts and prevents concurrent RSS searches from
-- inserting duplicate history rows.
DELETE FROM r_sshistories
WHERE id NOT IN (
  SELECT MAX(id) FROM r_sshistories
  GROUP BY config COLLATE NOCASE, list COLLATE NOCASE, indexer COLLATE NOCASE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_r_sshistories_config_list_indexer
ON r_sshistories (config COLLATE NOCASE, list COLLATE NOCASE, indexer COLLATE NOCASE);

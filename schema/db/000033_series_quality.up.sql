-- Add quality_profile to the series (per-list) table so a series carries the
-- wanted quality of the list/config it was added to. Episodes inherit this when
-- they are created, instead of re-deriving the quality from whichever config
-- happened to be refreshing (which could pick the wrong, e.g. music, profile).
ALTER TABLE `series` ADD COLUMN `quality_profile` text NOT NULL DEFAULT '';

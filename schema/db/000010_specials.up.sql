ALTER TABLE series ADD COLUMN search_specials numeric default 0;
ALTER TABLE series ADD COLUMN ignore_runtime numeric default 0;
ALTER TABLE serie_episodes ADD COLUMN ignore_runtime numeric default 0;
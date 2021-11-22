ALTER TABLE qualities ADD COLUMN use_regex integer default 0;
UPDATE [qualities] SET Regex='(?i)' || regex  where Regex not like '%(?i)%'
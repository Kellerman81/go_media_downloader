-- Revert everything the up migration added.

DELETE FROM [qualities] WHERE type = 3 AND name IN ('av1', 'vc1', 'mpeg2');
DELETE FROM [qualities] WHERE type = 4 AND name IN ('atmos', 'dtsx');
DELETE FROM [qualities] WHERE type = 2 AND name IN ('vodrip', 'satrip', 'rawhd');
DELETE FROM [qualities] WHERE type = 5 AND name IN ('APE', 'WavPack');

-- Remove appended spellings from existing rows.
UPDATE [qualities] SET Strings = replace(Strings, ',fhd', '')                                              WHERE type = 1 AND name = '1080p';
UPDATE [qualities] SET Strings = replace(Strings, ',848x480,640x480', '')                                  WHERE type = 1 AND name = '480p';
UPDATE [qualities] SET Strings = replace(Strings, ',camrip,newcam,hdcamrip', '')                           WHERE type = 2 AND name = 'cam';
UPDATE [qualities] SET Strings = replace(Strings, ',wp', '')                                               WHERE type = 2 AND name = 'workprint';
UPDATE [qualities] SET Strings = replace(Strings, ',webmux', '')                                           WHERE type = 2 AND name = 'webrip';
UPDATE [qualities] SET Strings = replace(Strings, ',scr,screener', '')                                     WHERE type = 2 AND name = 'dvdscr';
UPDATE [qualities] SET Strings = replace(Strings, ',regional', '')                                         WHERE type = 2 AND name = 'r5';
UPDATE [qualities] SET Strings = replace(Strings, ',bd25,bd50,bd66,bd100,bdiso,bdmux,br-disk,brdisk,uhdbd', '') WHERE type = 2 AND name = 'bluray';
UPDATE [qualities] SET Strings = replace(Strings, ',dvd-r,dvd5,dvd9,dvdr,dvd-5,dvd-9', '')                 WHERE type = 2 AND name = 'dvdrip';

-- Revert 2160p, dd+5.1 and ts to their pre-migration definitions.
UPDATE [qualities] SET Strings = '2160p,2160i' WHERE type = 1 AND name = '2160p';

UPDATE [qualities]
SET Strings = 'dd+5.1',
    Regex   = '(?i)(\b|_)dd[p+][0-9\.]+(\b|_)'
WHERE type = 4 AND name = 'dd+5.1';

UPDATE [qualities]
SET Strings = 'hdts,ts,telesync',
    Regex   = '(?i)(\b|_)(?:hd)?ts|telesync(\b|_)'
WHERE type = 2 AND name = 'ts';

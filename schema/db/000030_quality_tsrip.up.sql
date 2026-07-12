-- Quality table additions.
-- Detection runs off the Strings lists (qualities.use_regex defaults to 0), so
-- every change below targets Strings; existing long lists are appended to with
-- || so their current values do not need to be restated.

-- ts: TSRip / PDVD / HDTSRip / TELESYNCH spellings.
UPDATE [qualities]
SET Strings = 'hdts,ts,telesync,tsrip,pdvd,hdtsrip,telesynch',
    Regex   = '(?i)(\b|_)(?:hd)?ts(?:rip)?|telesync(\b|_)'
WHERE type = 2 AND name = 'ts';

-- dd+5.1 (Dolby Digital Plus / E-AC-3): fold the EAC3 / DDP spellings in.
UPDATE [qualities]
SET Strings = 'dd+5.1,ddp,ddp5.1,ddp7.1,ddp2.0,eac3,e-ac-3,e.ac.3,e ac 3',
    Regex   = '(?i)(\b|_)(dd[p+][0-9\.]+|e[^a-zA-Z0-9]?ac[^a-zA-Z0-9]?3)(\b|_)'
WHERE type = 4 AND name = 'dd+5.1';

-- 2160p: add the bare "4k" and "3840x2160" tags.
UPDATE [qualities] SET Strings = '2160p,2160i,4k,3840x2160' WHERE type = 1 AND name = '2160p';

-- Append additional spellings to existing rows.
UPDATE [qualities] SET Strings = Strings || ',fhd'                                              WHERE type = 1 AND name = '1080p';
UPDATE [qualities] SET Strings = Strings || ',848x480,640x480'                                  WHERE type = 1 AND name = '480p';
UPDATE [qualities] SET Strings = Strings || ',camrip,newcam,hdcamrip'                           WHERE type = 2 AND name = 'cam';
UPDATE [qualities] SET Strings = Strings || ',wp'                                               WHERE type = 2 AND name = 'workprint';
UPDATE [qualities] SET Strings = Strings || ',webmux'                                           WHERE type = 2 AND name = 'webrip';
UPDATE [qualities] SET Strings = Strings || ',scr,screener'                                     WHERE type = 2 AND name = 'dvdscr';
UPDATE [qualities] SET Strings = Strings || ',regional'                                         WHERE type = 2 AND name = 'r5';
UPDATE [qualities] SET Strings = Strings || ',bd25,bd50,bd66,bd100,bdiso,bdmux,br-disk,brdisk,uhdbd' WHERE type = 2 AND name = 'bluray';
UPDATE [qualities] SET Strings = Strings || ',dvd-r,dvd5,dvd9,dvdr,dvd-5,dvd-9'                 WHERE type = 2 AND name = 'dvdrip';

-- New source: RawHD (raw HD capture).
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (2, 'rawhd', 8100, '(?i)(\b|_)raw[^a-zA-Z0-9]?hd(\b|_)', 'rawhd,raw-hd,raw.hd,raw hd,raw_hd');

-- Sources (type 2).
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (2, 'vodrip', 5200, '(?i)(\b|_)vod[^a-zA-Z0-9]?rip(\b|_)', 'vodrip,vod.rip,vod rip,vod-rip,vod_rip');
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (2, 'satrip', 5100, '(?i)(\b|_)sat[^a-zA-Z0-9]?rip(\b|_)', 'satrip,sat.rip,sat rip,sat-rip,sat_rip');

-- Codecs (type 3).
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (3, 'av1',   450, '(?i)(\b|_)av1(\b|_)',                'av1');
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (3, 'vc1',   250, '(?i)(\b|_)vc[^a-zA-Z0-9]?1(\b|_)',   'vc1,vc-1,vc.1,vc 1,vc_1');
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (3, 'mpeg2',  50, '(?i)(\b|_)mpeg[^a-zA-Z0-9]?2(\b|_)', 'mpeg2,mpeg-2,mpeg.2,mpeg 2,mpeg_2');

-- Audio (type 4).
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (4, 'atmos', 100, '(?i)(\b|_)atmos(\b|_)',             'atmos');
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (4, 'dtsx',   95, '(?i)(\b|_)dts[^a-zA-Z0-9]?x(\b|_)', 'dtsx,dts-x,dts.x,dts x,dts_x,dts:x');

-- Music lossless formats (type 5, string-only like the other music formats).
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (5, 'APE',     98, '', 'ape');
INSERT INTO [qualities] ([type], name, priority, regex, strings) VALUES (5, 'WavPack', 92, '', 'wavpack,wv');

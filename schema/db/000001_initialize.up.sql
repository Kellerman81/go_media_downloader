CREATE TABLE `dbmovies` (`id` integer,`created_at` datetime NOT NULL DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`title` text DEFAULT "",`release_date` text DEFAULT "",`year` integer default 0,`adult` numeric default 0,`budget` integer default 0,`genres` text DEFAULT "",`original_language` text DEFAULT "",`original_title` text DEFAULT "",`overview` text DEFAULT "",`popularity` real default 0,`revenue` integer default 0,`runtime` integer default 0,`spoken_languages` text DEFAULT "",`status` text DEFAULT "",`tagline` text DEFAULT "",`vote_average` real default 0,`vote_count` integer default 0,`moviedb_id` integer default 0,`imdb_id` text DEFAULT "",`freebase_m_id` text DEFAULT "",`freebase_id` text DEFAULT "",`facebook_id` text DEFAULT "",`instagram_id` text DEFAULT "",`twitter_id` text DEFAULT "",`url` text DEFAULT "",`backdrop` text DEFAULT "",`poster` text DEFAULT "",`slug` text DEFAULT "", `trakt_id` integer default 0,PRIMARY KEY (`id`));


CREATE TABLE [serie_file_unmatcheds] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [listname] text DEFAULT "", [filepath] text DEFAULT "", [last_checked] datetime, [parsed_data] text DEFAULT "");

CREATE TABLE [r_sshistories] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [config] text DEFAULT "", [list] text DEFAULT "", [indexer] text DEFAULT "", [last_id] text DEFAULT "");

CREATE TABLE [qualities] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [type] integer default 0, [name] text DEFAULT "", [regex] text DEFAULT "", [strings] text DEFAULT "", [priority] integer default 0);

CREATE TABLE [movie_file_unmatcheds] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [listname] text DEFAULT "", [filepath] text DEFAULT "", [last_checked] datetime, [parsed_data] text DEFAULT "");

CREATE TABLE [job_histories] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [job_type] text DEFAULT "", [job_category] text DEFAULT "", [job_group] text DEFAULT "", [started] datetime, [ended] datetime);

CREATE TABLE [indexer_fails] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [indexer] text DEFAULT "", [last_fail] datetime);

CREATE TABLE `dbseries` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`seriename` text DEFAULT "",`aliases` text DEFAULT "",`season` text DEFAULT "",`status` text DEFAULT "",`firstaired` text DEFAULT "",`network` text DEFAULT "",`runtime` text DEFAULT "",`language` text DEFAULT "",`genre` text DEFAULT "",`overview` text DEFAULT "",`rating` text DEFAULT "",`siterating` text DEFAULT "",`siterating_count` text DEFAULT "",`slug` text DEFAULT "",`imdb_id` text DEFAULT "",`thetvdb_id` integer default 0,`freebase_m_id` text DEFAULT "",`freebase_id` text DEFAULT "",`tvrage_id` integer default 0,`facebook` text DEFAULT "",`instagram` text DEFAULT "",`twitter` text DEFAULT "",`banner` text DEFAULT "",`poster` text DEFAULT "",`fanart` text DEFAULT "",`identifiedby` text DEFAULT "", `trakt_id` integer default 0,PRIMARY KEY (`id`));

CREATE TABLE `dbserie_episodes` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`episode` text DEFAULT "",`season` text DEFAULT "",`identifier` text DEFAULT "",`title` text DEFAULT "",`first_aired` text DEFAULT "",`overview` text DEFAULT "",`poster` text DEFAULT "",`dbserie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_dbserie_episodes_dbserie` FOREIGN KEY (`dbserie_id`) REFERENCES `dbseries`(`id`) ON DELETE CASCADE);

CREATE TABLE `dbserie_alternates` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`title` text NOT NULL,`slug` text DEFAULT "",`dbserie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_dbserie_alternates_dbserie` FOREIGN KEY (`dbserie_id`) REFERENCES `dbseries`(`id`) ON DELETE CASCADE);

CREATE TABLE `dbmovie_titles` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`dbmovie_id` integer default 0,`title` text DEFAULT "",`slug` text DEFAULT "",PRIMARY KEY (`id`),CONSTRAINT `fk_dbmovie_titles_dbmovie` FOREIGN KEY (`dbmovie_id`) REFERENCES `dbmovies`(`id`) ON DELETE CASCADE);
CREATE TABLE `series` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`listname` text DEFAULT "",`rootpath` text DEFAULT "",`dbserie_id` integer default 0,`dont_upgrade` numeric default 0,`dont_search` numeric default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_series_dbserie` FOREIGN KEY (`dbserie_id`) REFERENCES `dbseries`(`id`) ON DELETE CASCADE);

CREATE TABLE `serie_episodes` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`lastscan` datetime,`blacklisted` numeric default 0,`quality_reached` numeric default 0,`quality_profile` text DEFAULT "",`missing` numeric default 0,`dont_upgrade` numeric default 0,`dont_search` numeric default 0,`dbserie_episode_id` integer default 0,`serie_id` integer default 0,`dbserie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_serie_episodes_dbserie_episode` FOREIGN KEY (`dbserie_episode_id`) REFERENCES `dbserie_episodes`(`id`),CONSTRAINT `fk_serie_episodes_serie` FOREIGN KEY (`serie_id`) REFERENCES `series`(`id`),CONSTRAINT `fk_serie_episodes_dbserie` FOREIGN KEY (`dbserie_id`) REFERENCES `dbseries`(`id`) ON DELETE CASCADE);

CREATE TABLE [serie_episode_histories] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [title] text DEFAULT "", [url] text DEFAULT "", [indexer] text DEFAULT "", [type] text DEFAULT "", [target] text DEFAULT "", [downloaded_at] datetime, [blacklisted] numeric(53) default 0, [quality_profile] text DEFAULT "", [resolution_id] integer default 0, [quality_id] integer default 0, [codec_id] integer default 0, [audio_id] integer default 0, [serie_id] integer default 0, [serie_episode_id] integer default 0, [dbserie_episode_id] integer default 0, [dbserie_id] integer default 0, 
	FOREIGN KEY ([dbserie_id])
		REFERENCES [dbseries] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE, 
	FOREIGN KEY ([serie_id])
		REFERENCES [series] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE, 
	FOREIGN KEY ([dbserie_episode_id])
		REFERENCES [dbserie_episodes] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE, 
	FOREIGN KEY ([serie_episode_id])
		REFERENCES [serie_episodes] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE);

CREATE TABLE [serie_episode_files] ([id] integer NOT NULL PRIMARY KEY, [created_at] datetime NOT NULL  DEFAULT current_timestamp, [updated_at] datetime NOT NULL DEFAULT current_timestamp, [location] text DEFAULT "", [filename] text DEFAULT "", [extension] text DEFAULT "", [quality_profile] text DEFAULT "", [proper] numeric(53) default 0, [extended] numeric(53) default 0, [repack] numeric(53) default 0, [height] integer default 0, [width] integer default 0, [resolution_id] integer default 0, [quality_id] integer default 0, [codec_id] integer default 0, [audio_id] integer default 0, [serie_id] integer default 0, [serie_episode_id] integer default 0, [dbserie_episode_id] integer default 0, [dbserie_id] integer default 0, 
	FOREIGN KEY ([dbserie_episode_id])
		REFERENCES [dbserie_episodes] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE, 
	FOREIGN KEY ([serie_id])
		REFERENCES [series] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE, 
	FOREIGN KEY ([dbserie_id])
		REFERENCES [dbseries] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE, 
	FOREIGN KEY ([serie_episode_id])
		REFERENCES [serie_episodes] ([id])
		ON UPDATE NO ACTION ON DELETE CASCADE);

CREATE TABLE `movies` (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`lastscan` datetime,`blacklisted` numeric default 0,`quality_reached` numeric default 0,`quality_profile` text DEFAULT "",`missing` numeric default 0,`dont_upgrade` numeric default 0,`dont_search` numeric default 0,`listname` text DEFAULT "",`rootpath` text DEFAULT "",`dbmovie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_movies_dbmovie` FOREIGN KEY (`dbmovie_id`) REFERENCES `dbmovies`(`id`) ON DELETE CASCADE);

CREATE TABLE "movie_histories" (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`title` text NOT NULL,`url` text DEFAULT "",`indexer` text DEFAULT "",`type` text DEFAULT "",`target` text DEFAULT "",`downloaded_at` datetime,`blacklisted` numeric default 0,`quality_profile` text DEFAULT "",`resolution_id` integer default 0,`quality_id` integer default 0,`codec_id` integer default 0,`audio_id` integer default 0,`movie_id` integer default 0,`dbmovie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_movie_histories_movie` FOREIGN KEY (`movie_id`) REFERENCES `movies`(`id`) ON DELETE CASCADE,CONSTRAINT `fk_movie_histories_dbmovie` FOREIGN KEY (`dbmovie_id`) REFERENCES `dbmovies`(`id`) ON DELETE CASCADE);

CREATE TABLE "movie_files" (`id` integer,`created_at` datetime NOT NULL  DEFAULT current_timestamp,`updated_at` datetime NOT NULL DEFAULT current_timestamp,`location` text NOT NULL,`filename` text DEFAULT "",`extension` text DEFAULT "",`quality_profile` text DEFAULT "",`proper` numeric default 0,`extended` numeric default 0,`repack` numeric default 0,`height` integer default 0,`width` integer default 0,`resolution_id` integer default 0,`quality_id` integer default 0,`codec_id` integer default 0,`audio_id` integer default 0,`movie_id` integer default 0,`dbmovie_id` integer default 0,PRIMARY KEY (`id`),CONSTRAINT `fk_movie_files_movie` FOREIGN KEY (`movie_id`) REFERENCES `movies`(`id`) ON DELETE CASCADE,CONSTRAINT `fk_movie_files_dbmovie` FOREIGN KEY (`dbmovie_id`) REFERENCES `dbmovies`(`id`) ON DELETE CASCADE);

CREATE TRIGGER tg_movie_files_updated_at
AFTER UPDATE
ON movie_files FOR EACH ROW
BEGIN
  UPDATE movie_files SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_movie_histories_updated_at
AFTER UPDATE
ON movie_histories FOR EACH ROW
BEGIN
  UPDATE movie_histories SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_movies_updated_at
AFTER UPDATE
ON movies FOR EACH ROW
BEGIN
  UPDATE movies SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_serie_episode_files_updated_at
AFTER UPDATE
ON serie_episode_files FOR EACH ROW
BEGIN
  UPDATE serie_episode_files SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_serie_episode_histories_updated_at
AFTER UPDATE
ON serie_episode_histories FOR EACH ROW
BEGIN
  UPDATE serie_episode_histories SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_serie_episodes_updated_at
AFTER UPDATE
ON serie_episodes FOR EACH ROW
BEGIN
  UPDATE serie_episodes SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_series_updated_at
AFTER UPDATE
ON series FOR EACH ROW
BEGIN
  UPDATE series SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_dbmovie_titles_updated_at
AFTER UPDATE
ON dbmovie_titles FOR EACH ROW
BEGIN
  UPDATE dbmovie_titles SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_dbserie_alternates_updated_at
AFTER UPDATE
ON dbserie_alternates FOR EACH ROW
BEGIN
  UPDATE dbserie_alternates SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_dbserie_episodes_updated_at
AFTER UPDATE
ON dbserie_episodes FOR EACH ROW
BEGIN
  UPDATE dbserie_episodes SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_dbseries_updated_at
AFTER UPDATE
ON dbseries FOR EACH ROW
BEGIN
  UPDATE dbseries SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_indexer_fails_updated_at
AFTER UPDATE
ON indexer_fails FOR EACH ROW
BEGIN
  UPDATE indexer_fails SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_job_histories_updated_at
AFTER UPDATE
ON job_histories FOR EACH ROW
BEGIN
  UPDATE job_histories SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_movie_file_unmatcheds_updated_at
AFTER UPDATE
ON movie_file_unmatcheds FOR EACH ROW
BEGIN
  UPDATE movie_file_unmatcheds SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_qualities_updated_at
AFTER UPDATE
ON qualities FOR EACH ROW
BEGIN
  UPDATE qualities SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_r_sshistories_updated_at
AFTER UPDATE
ON r_sshistories FOR EACH ROW
BEGIN
  UPDATE r_sshistories SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_serie_file_unmatcheds_updated_at
AFTER UPDATE
ON serie_file_unmatcheds FOR EACH ROW
BEGIN
  UPDATE serie_file_unmatcheds SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE TRIGGER tg_dbmovies_updated_at
AFTER UPDATE
ON dbmovies FOR EACH ROW
BEGIN
  UPDATE dbmovies SET updated_at = current_timestamp
    WHERE id = old.id;
END;

CREATE INDEX `idx_dbserie_alternates_title` ON `dbserie_alternates`(`title`);
CREATE INDEX `idx_movie_files_location` ON "movie_files"(`location`);
CREATE INDEX `idx_movie_histories_quality_profile` ON "movie_histories"(`quality_profile`);
CREATE INDEX `idx_movie_histories_title` ON "movie_histories"(`title`);
CREATE INDEX [idx_qualities_id]
	ON [qualities] ([id]);
CREATE INDEX [idx_serie_episode_files_location]
	ON [serie_episode_files] ([location]);
CREATE INDEX [idx_serie_episode_histories_quality_profile]
	ON [serie_episode_histories] ([quality_profile]);
CREATE INDEX [idx_serie_episode_histories_title]
	ON [serie_episode_histories] ([title]);
CREATE UNIQUE INDEX [index_qualities_1]
	ON [qualities] ([name]);
	

INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "360p", 10000, "(\b|_)360p(\b|_)", "360p,360i");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "368p", 20000, "(\b|_)368p(\b|_)", "368p,368i");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "480p", 30000, "(\b|_)480p(\b|_)", "480p,480i");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "576p", 40000, "(\b|_)576p(\b|_)", "576p,576i");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "720p", 50000, "(\b|_)(1280x)?720(i|p)(\b|_)", "720p,720i");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "1080p", 60000, "(\b|_)(1920x)?1080(i|p)(\b|_)", "1080p,1080i");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (1, "2160p", 70000, "(\b|_)((3840x)?2160p|4k)(\b|_)", "2160p,2160i");

INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "workprint", 1000, "(\b|_)workprint(\b|_)", "workprint");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "cam", 1300, "(\b|_)(?:web)?cam(\b|_)", "webcam,cam");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "ts", 2000, "(\b|_)(?:hd)?ts|telesync(\b|_)", "hdts,ts,telesync");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "tc", 2300, "(\b|_)(tc|telecine)(\b|_)", "tc,telecine");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "r5", 3000, "(\b|_)r[2-8c](\b|_)", "r5,r6");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "hdrip", 3300, "(\b|_)hd[^a-zA-Z0-9]?rip(\b|_)", "hdrip,hd.rip,hd rip,hd-rip,hd_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "ppvrip", 4000, "(\b|_)ppv[^a-zA-Z0-9]?rip(\b|_)", "ppvrip,ppv.rip,ppv rip,ppv-rip,ppv_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "preair", 4300, "(\b|_)preair(\b|_)", "preair");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "tvrip", 5000, "(\b|_)tv[^a-zA-Z0-9]?rip(\b|_)", "tvrip,tv.rip,tv rip,tv-rip,tv_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "dsr", 5300, "(\b|_)(dsr|ds)[^a-zA-Z0-9]?rip(\b|_)", "dsrip,ds.rip,ds rip,ds-rip,ds_rip,dsrrip,dsr.rip,dsr rip,dsr-rip,dsr_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "sdtv", 6000, "(\b|_)(?:[sp]dtv|dvb)(?:[^a-zA-Z0-9]?rip)?(\b|_)", "sdtv,pdtv,dvb,sdtvrip,sdtv.rip,sdtv rip,sdtv-rip,sdtv_rip,pdtvrip,pdtv.rip,pdtv rip,pdtv-rip,pdtv_rip,dvbrip,dvb.rip,dvb rip,dvb-rip,dvb_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "dvdscr", 6300, "(\b|_)(?:(?:dvd|web)[^a-zA-Z0-9]?)?scr(?:eener)?(\b|_)", "webscr,webscreener,web.scr,web.screener,web-scr,web-screener,web scr,web screener,web_scr,web_screener,dvdscr,dvdscreener,dvd.scr,dvd.screener,dvd-scr,dvd-screener,dvd scr,dvd screener,dvd_scr,dvd_screener");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "bdscr", 7000, "(\b|_)bdscr(?:eener)?(\b|_)", "bdscr,bdscreener");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "webrip", 7300, "(\b|_)web[^a-zA-Z0-9]?rip(\b|_)", "webrip,web.rip,web rip,web-rip,web_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "hdtv", 8000, "(\b|_)a?hdtv(?:[^a-zA-Z0-9]?rip)?(\b|_)", "hdtv,hdtvrip,hdtv.rip,hdtv rip,hdtv-rip,hdtv_rip");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "webdl", 8300, "(\b|_)web(?:[^a-zA-Z0-9]?(dl|hd))?(\b|_)", "webdl,web dl,web.dl,web-dl,web_dl,webhd,web.hd,web hd,web-hd,web_hd");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "dvdrip", 9000, "(\b|_)(dvd[^a-zA-Z0-9]?rip|hddvd)(\b|_)", "dvdrip,dvd.rip,dvd rip,dvd-rip,dvd_rip,hddvd,dvd");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "remux", 9100, "(\b|_)remux(\b|_)", "remux");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (2, "bluray", 9300, "(\b|_)(?:b[dr][^a-zA-Z0-9]?rip|blu[^a-zA-Z0-9]?ray(?:[^a-zA-Z0-9]?rip)?)(\b|_)", "bdrip,bd.rip,bd rip,bd-rip,bd_rip,brrip,br.rip,br rip,br-rip,br_rip,bluray,blu ray,blu.ray,blu-ray,blu_ray");

INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (3, "divx", 100, "(\b|_)divx(\b|_)", "divx");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (3, "xvid", 200, "(\b|_)xvid(\b|_)", "xvid");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (3, "h264", 300, "(\b|_)(h|x)264(\b|_)", "h264,x264");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (3, "vp9", 400, "(\b|_)vp9(\b|_)", "vp9");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (3, "h265", 500, "(\b|_)((h|x)265|hevc)(\b|_)", "h265,x265,hevc");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (3, "10bit", 600, "(\b|_)(10bit|hi10p)(\b|_)", "10bit,hi10p");

INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "mp3", 10, "(\b|_)mp3(\b|_)", "mp3");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "aac", 20, "(\b|_)aac(s)?(\b|_)", "aac,aacs");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "dd5.1", 30, "(\b|_)dd[0-9\.]+(\b|_)","dd5.1");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "ac3", 40, "(\b|_)ac3(s)?(\b|_)", "ac3,ac3s");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "dd+5.1", 50, "(\b|_)dd[p+][0-9\.]+(\b|_)","dd+5.1");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "flac", 60, "(\b|_)flac(s)?(\b|_)", "flac,flacs");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "dtshd", 70, "(\b|_)dts[^a-zA-Z0-9]?hd(?:[^a-zA-Z0-9]?ma)?(s)?(\b|_)","dtshd");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "dts", 80, "(\b|_)dts(s)?(\b|_)", "dts,dtss");
INSERT INTO [qualities] ([Type], Name, Priority, Regex, Strings) VALUES (4, "truehd", 90, "(\b|_)truehd(s)?(\b|_)", "truehd");
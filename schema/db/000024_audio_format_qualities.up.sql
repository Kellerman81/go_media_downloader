-- Add audio format quality entries (type 5) for music/audiobook priority system
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'FLAC', 100, 'flac', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'ALAC', 95, 'alac', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'WAV', 90, 'wav,aiff', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'DSD', 85, 'dsd,dsf', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'OPUS', 60, 'opus', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'AAC', 50, 'aac,m4a,m4b', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'OGG', 45, 'ogg,vorbis', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'MP3', 40, 'mp3', '');
INSERT INTO [qualities] ([Type], Name, Priority, Strings, Regex) VALUES (5, 'WMA', 20, 'wma', '');

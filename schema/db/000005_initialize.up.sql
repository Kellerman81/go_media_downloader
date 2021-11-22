UPDATE [qualities] SET Strings='480p,480i,480' where [Type]=1 AND Name='480p';
UPDATE [qualities] SET Strings='720p,720i,720,1280x720' where [Type]=1 AND Name='720p';
UPDATE [qualities] SET Strings='1080p,1080i,1080,1920x1080' where [Type]=1 AND Name='1080p';
UPDATE [qualities] SET Regex='(\b|_)(?:web|hd|hq)?cam(\b|_)', Strings='webcam,cam,hdcam,hqcam' where [Type]=2 AND Name='cam';
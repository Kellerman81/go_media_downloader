package utils

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/scanner"
)

type Structure struct {
	configEntry config.MediaTypeConfig
	list        config.MediaListsConfig
	groupType   string //series, movies
	rootpath    string //1st level below input
	sourcepath  config.PathsConfig
	targetpath  config.PathsConfig
}

func NewStructure(configEntry config.MediaTypeConfig, list config.MediaListsConfig, groupType string, rootpath string, sourcepath config.PathsConfig, targetpath config.PathsConfig) (Structure, error) {
	if !configEntry.Structure {
		return Structure{}, errors.New("not allowed")
	}
	return Structure{
		configEntry: configEntry,
		list:        list,
		groupType:   groupType,
		rootpath:    rootpath,
		sourcepath:  sourcepath,
		targetpath:  targetpath,
	}, nil
}

func (s *Structure) CheckDisallowed() bool {
	if s.groupType == "series" {
		return scanner.CheckDisallowed(s.rootpath, s.sourcepath.Disallowed, false)
	} else {
		return scanner.CheckDisallowed(s.rootpath, s.sourcepath.Disallowed, s.sourcepath.DeleteDisallowed)
	}
}

func (s *Structure) GetVideoFiles(folder string, pathconfig config.PathsConfig, removesmallfiles bool) []string {
	videofiles := scanner.GetFilesGoDir(folder, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename, pathconfig.Blocked)
	if removesmallfiles {
		for idx := range videofiles {
			info, err := os.Stat(videofiles[idx])
			if pathconfig.MinVideoSize > 0 && err == nil {
				if info.Size() < int64(pathconfig.MinVideoSize*1024*1024) {
					scanner.RemoveFiles([]string{videofiles[idx]}, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename)
				}
			}
		}
		videofiles = scanner.GetFilesGoDir(folder, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename, pathconfig.Blocked)
	}
	return videofiles
}

func (s *Structure) RemoveSmallVideoFile(file string, pathconfig config.PathsConfig) (removed bool) {
	info, err := os.Stat(file)
	if pathconfig.MinVideoSize > 0 && err == nil {
		if info.Size() < int64(pathconfig.MinVideoSize*1024*1024) {
			scanner.RemoveFiles([]string{file}, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename)
			removed = true
		}
	}
	return
}

//Parses - uses fprobe and checks language
func (s *Structure) ParseFile(videofile string, checkfolder bool, folder string, deletewronglanguage bool) (m *ParseInfo, err error) {
	yearintitle := false
	if s.groupType == "series" {
		yearintitle = true
	}
	m, err = NewFileParser(filepath.Base(videofile), yearintitle, s.groupType)
	if err != nil {
		logger.Log.Debug("Parse failed of ", filepath.Base(videofile))
		return
	}
	if m.Quality == "" && m.Resolution == "" && checkfolder {
		mf, errf := NewFileParser(filepath.Base(folder), yearintitle, s.groupType)
		if errf != nil {
			logger.Log.Debug("Parse failed of folder ", filepath.Base(folder))
			err = errf
			return
		} else {
			m.Quality = mf.Quality
			m.Resolution = mf.Resolution
			m.Title = mf.Title
			if m.Year == 0 {
				m.Year = mf.Year
			}
			if m.Identifier == "" {
				m.Identifier = mf.Identifier
			}
			if m.Audio == "" {
				m.Audio = mf.Audio
			}
			if m.Codec == "" {
				m.Codec = mf.Codec
			}
			if m.Imdb == "" {
				m.Imdb = mf.Imdb
			}
		}
	}

	if !config.ConfigCheck("quality_" + s.list.Template_quality) {
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.list.Template_quality, &cfg_quality)

	m.Title = strings.Trim(m.Title, " ")
	for idx := range cfg_quality.TitleStripSuffixForSearch {
		if strings.HasSuffix(strings.ToLower(m.Title), strings.ToLower(cfg_quality.TitleStripSuffixForSearch[idx])) {
			m.Title = trimStringInclAfterStringInsensitive(m.Title, cfg_quality.TitleStripSuffixForSearch[idx])
			m.Title = strings.Trim(m.Title, " ")
		}
	}
	//logger.Log.Debug("Parsed as: ", m)
	return
}

func (s *Structure) ParseFileAdditional(videofile string, m *ParseInfo, folder string, deletewronglanguage bool) (err error) {
	if !config.ConfigCheck("quality_" + s.list.Template_quality) {
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.list.Template_quality, &cfg_quality)

	m.GetPriority(s.configEntry, cfg_quality)

	err = m.ParseVideoFile(videofile, s.configEntry, cfg_quality)
	if err != nil {
		return
	}
	if len(s.targetpath.Allowed_languages) >= 1 && deletewronglanguage {
		language_ok := false
		for idx := range s.targetpath.Allowed_languages {
			if len(m.Languages) == 0 && s.targetpath.Allowed_languages[idx] == "" {
				language_ok = true
				break
			}
			for langidx := range m.Languages {
				if strings.EqualFold(s.targetpath.Allowed_languages[idx], m.Languages[langidx]) {
					language_ok = true
					break
				}
			}
			if language_ok {
				break
			}
		}
		if !language_ok {
			logger.Log.Debug("Languages not matched - skipping: ", folder, " ", m.Languages)
			if strings.ToLower(s.groupType) == "movie" {
				emptyarr := make([]string, 0, 1)
				filesleft := scanner.GetFilesGoDir(folder, s.sourcepath.AllowedVideoExtensions, s.sourcepath.AllowedVideoExtensionsNoRename, emptyarr)
				scanner.RemoveFiles(filesleft, emptyarr, emptyarr)

				scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
			} else {
				fileext := filepath.Ext(videofile)
				err := scanner.RemoveFile(videofile)
				if err == nil {
					for idxext := range s.sourcepath.AllowedOtherExtensions {
						additionalfile := strings.Replace(videofile, fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
						scanner.RemoveFile(additionalfile)
					}
				}
				scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
			}
		}
		if !language_ok {
			err = errors.New("wrong Language")
		}
	}
	return err
}

func (s *Structure) CheckLowerQualTarget(folder string, videofile string, m ParseInfo, cleanuplowerquality bool, movie database.Movie) ([]string, int, error) {
	moviefiles, _ := database.QueryMovieFiles(database.Query{Select: "location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack", Where: "movie_id = ?", WhereArgs: []interface{}{movie.ID}})
	logger.Log.Debug("Found existing files: ", len(moviefiles))

	if !config.ConfigCheck("quality_" + s.list.Template_quality) {
		return []string{}, 0, errors.New("config not found")
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.list.Template_quality, &cfg_quality)

	oldpriority := getHighestMoviePriorityByFiles(movie, s.configEntry, cfg_quality)
	logger.Log.Debug("Found existing highest prio: ", oldpriority)
	if m.Priority > oldpriority {
		logger.Log.Debug("prio: ", oldpriority, " lower as ", m.Priority)
		oldfiles := make([]string, 0, len(moviefiles)*3)
		if len(moviefiles) >= 1 {
			for idx := range moviefiles {
				logger.Log.Debug("want to remove ", moviefiles[idx])
				oldpath, _ := filepath.Split(moviefiles[idx].Location)
				logger.Log.Debug("want to remove oldpath ", oldpath)
				entry_prio := getMovieDBPriority(moviefiles[idx], s.configEntry, cfg_quality)
				logger.Log.Debug("want to remove oldprio ", entry_prio)
				if m.Priority > entry_prio && s.targetpath.Upgrade {
					oldfiles = append(oldfiles, moviefiles[idx].Location)
					logger.Log.Debug("get all old files ", oldpath)
					emptyarr := make([]string, 0, 1)
					oldfilesadd := scanner.GetFilesGoDir(oldpath, emptyarr, emptyarr, emptyarr)
					logger.Log.Debug("found old files ", len(oldfilesadd))
					oldfiles = append(oldfiles, oldfilesadd...)
				}
			}
		}
		return oldfiles, oldpriority, nil
	} else if len(moviefiles) >= 1 {
		logger.Log.Debug("Skipped import due to lower quality: ", videofile)
		err := errors.New("import file has lower quality")
		if cleanuplowerquality {
			emptyarr := make([]string, 0, 1)
			filesleft := scanner.GetFilesGoDir(folder, s.sourcepath.AllowedVideoExtensions, s.sourcepath.AllowedVideoExtensionsNoRename, emptyarr)
			scanner.RemoveFiles(filesleft, emptyarr, emptyarr)
			scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
		}
		return []string{}, oldpriority, err
	}
	return []string{}, oldpriority, nil
}

type parser struct {
	Dbmovie        database.Dbmovie
	Movie          database.Movie
	Dbserie        database.Dbserie
	DbserieEpisode database.DbserieEpisode
	Source         ParseInfo
	Identifier     string
}

func (s *Structure) GenerateNaming(videofile string, m ParseInfo, movie database.Movie,
	series database.Serie, serietitle string, serieepisode database.SerieEpisode, episodetitle string, episodes []int) (foldername string, filename string) {

	forparser := parser{}
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})

		movietitle := filepath.Base(videofile)
		movietitle = trimStringInclAfterString(movietitle, m.Quality)
		movietitle = trimStringInclAfterString(movietitle, m.Resolution)
		movietitle = trimStringInclAfterString(movietitle, strconv.Itoa(m.Year))
		movietitle = strings.Trim(movietitle, ".")
		movietitle = strings.Replace(movietitle, ".", " ", -1)
		logger.Log.Debug("trimmed title: ", movietitle)

		if dbmovie.Title == "" {
			dbmoviealt, dbmoviealterr := database.GetDbmovieTitle(database.Query{Select: "title", Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.DbmovieID}})
			if dbmoviealterr == nil {
				dbmovie.Title = dbmoviealt.Title
			} else {
				dbmovie.Title = movietitle
			}
		}
		if dbmovie.Year == 0 {
			dbmovie.Year = m.Year
		}

		naming := s.configEntry.Naming
		if s.targetpath.Usepresort {
			fmt.Println("usepresort")
			naming = "!presort/" + naming
			fmt.Println("usepresort: ", naming)
		}
		foldername, filename = path.Split(naming)
		//Naming = '{Title} ({Year})/{Title} ({Year}) German [{Resolution} {Quality} {Codec}]'
		if movie.Rootpath != "" {
			foldername, _ = getrootpath(foldername)
		}

		forparser.Dbmovie = dbmovie
		forparser.Source = m
		//Naming = '{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper 1}} proper{{end}}] ({{.Source.Imdb}})'
		// tmplfolder, err := template.New("tmplfolder").Parse(foldername)
		// if err != nil {
		// 	logger.Log.Error(err)
		// }
		// var doc bytes.Buffer
		// err = tmplfolder.Execute(&doc, parser{Dbmovie: dbmovie, Source: m})
		// if err != nil {
		// 	logger.Log.Error(err)
		// }
		// foldername = doc.String()
		// foldername = strings.Trim(foldername, ".")
		// foldername = StringReplaceDiacritics(foldername)
		// foldername = Path(foldername, true)

		// tmplfile, err := template.New("tmplfile").Parse(filename)
		// if err != nil {
		// 	logger.Log.Error(err)
		// }
		// err = tmplfile.Execute(&doc, parser{Dbmovie: dbmovie, Source: m})
		// if err != nil {
		// 	logger.Log.Error(err)
		// }
		// filename = doc.String()
		// filename = strings.Trim(filename, ".")
		// filename = strings.Replace(filename, "  ", " ", -1)
		// filename = strings.Replace(filename, " ]", "]", -1)
		// filename = strings.Replace(filename, "[ ", "[", -1)
		// filename = strings.Replace(filename, "[ ]", "", -1)
		// filename = strings.Replace(filename, "( )", "", -1)
		// filename = strings.Replace(filename, "[]", "", -1)
		// filename = strings.Replace(filename, "()", "", -1)
		// filename = StringReplaceDiacritics(filename)
		// filename = Path(filename, true)

		foldername = strings.Replace(foldername, "{Title}", dbmovie.Title, -1)
		foldername = strings.Replace(foldername, "{TitleSource}", movietitle, -1)
		foldername = strings.Replace(foldername, "{Year}", strconv.Itoa(dbmovie.Year), -1)
		foldername = strings.Replace(foldername, "{YearSource}", strconv.Itoa(m.Year), -1)
		foldername = strings.Trim(foldername, ".")
		foldername = StringReplaceDiacritics(foldername)
		foldername = Path(foldername, true)

		filename = strings.Replace(filename, "{Title}", dbmovie.Title, -1)
		filename = strings.Replace(filename, "{TitleSource}", movietitle, -1)
		filename = strings.Replace(filename, "{Year}", strconv.Itoa(dbmovie.Year), -1)
		filename = strings.Replace(filename, "{YearSource}", strconv.Itoa(m.Year), -1)
		proper := ""
		if m.Proper {
			proper = "proper"
		}
		filename = strings.Replace(filename, "{Proper}", proper, -1)
		extended := ""
		if m.Extended {
			extended = "extended"
		}
		if m.Imdb == "" {
			m.Imdb = dbmovie.ImdbID
		}
		filename = strings.Replace(filename, "{Extended}", extended, -1)
		filename = strings.Replace(filename, "{Resolution}", m.Resolution, -1)
		filename = strings.Replace(filename, "{Quality}", m.Quality, -1)
		filename = strings.Replace(filename, "{Codec}", m.Codec, -1)
		filename = strings.Replace(filename, "{Audio}", m.Audio, -1)
		if !strings.HasPrefix(m.Imdb, "tt") && len(m.Imdb) >= 1 {
			m.Imdb = "tt" + m.Imdb
		}
		filename = strings.Replace(filename, "{Imdb}", m.Imdb, -1)
		//filename = ReplaceStringObjectFields(filename, dbmovie)
		//filename = ReplaceStringObjectFields(filename, m)
		filename = strings.Replace(filename, "  ", " ", -1)
		filename = strings.Replace(filename, " ]", "]", -1)
		filename = strings.Replace(filename, "[ ", "[", -1)
		filename = strings.Replace(filename, "[ ]", "", -1)
		filename = strings.Replace(filename, "( )", "", -1)
		filename = strings.Replace(filename, "[]", "", -1)
		filename = strings.Replace(filename, "()", "", -1)
		filename = StringReplaceDiacritics(filename)
		filename = Path(filename, false)
	} else {
		dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})

		dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepisode.DbserieEpisodeID}})

		foldername, filename = path.Split(s.configEntry.Naming)
		if dbserie.Seriename == "" {
			dbseriealt, dbseriealterr := database.GetDbserieAlternates(database.Query{Select: "title", Where: "dbserie_id=?", WhereArgs: []interface{}{series.DbserieID}})
			if dbseriealterr == nil {
				dbserie.Seriename = dbseriealt.Title
			} else {
				dbserie.Seriename = serietitle
			}
		}
		if dbserieepisode.Title == "" {
			dbserieepisode.Title = episodetitle
		}
		if series.Rootpath != "" {
			foldername, _ = getrootpath(foldername)
		}

		forparser.Dbserie = dbserie
		forparser.DbserieEpisode = dbserieepisode
		forparser.Source = m

		//Naming = '{Title}/Season {Season}/{Title} - {Identifier} - German [{Resolution} {Quality} {Codec}] (tvdb{tvdb})'
		//Naming = '{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper 1}} proper{{end}}] ({{.Source.Imdb}})'
		// tmplfolder, err := template.New("tmplfolder").Parse(foldername)
		// if err != nil {
		// 	logger.Log.Error(err)
		// }
		// var doc bytes.Buffer
		// err = tmplfolder.Execute(&doc, parser{Dbmovie: dbmovie, Source: m})
		// if err != nil {
		// 	logger.Log.Error(err)
		// }
		// foldername = doc.String()
		// foldername = strings.Trim(foldername, ".")
		// foldername = StringReplaceDiacritics(foldername)
		// foldername = Path(foldername, true)
		//S{0Season}E{0Episode}(E{0Episode})
		foldername = strings.Replace(foldername, "{Title}", dbserie.Seriename, -1)
		foldername = strings.Replace(foldername, "{TitleSource}", serietitle, -1)
		foldername = strings.Replace(foldername, "{Season}", dbserieepisode.Season, -1)
		foldername = strings.Replace(foldername, "{0Season}", fmt.Sprintf("%02s", dbserieepisode.Season), -1)
		foldername = strings.Trim(foldername, ".")
		foldername = StringReplaceDiacritics(foldername)

		filename = strings.Replace(filename, "{Title}", dbserie.Seriename, -1)
		filename = strings.Replace(filename, "{Season}", dbserieepisode.Season, -1)
		filename = strings.Replace(filename, "{0Season}", fmt.Sprintf("%02s", dbserieepisode.Season), -1)
		identifier := s.configEntry.NamingIdentifier
		identifier = strings.Replace(identifier, "{0Season}", fmt.Sprintf("%02s", dbserieepisode.Season), -1)
		identifier = strings.Replace(identifier, "{Season}", dbserieepisode.Season, -1)
		identifier = strings.Replace(identifier, "{Identifier}", dbserieepisode.Identifier, -1)

		r := regexp.MustCompile(`(?i)(\(.*\))`)
		teststr := r.FindStringSubmatch(identifier)
		if len(episodes) == 1 {
			identifier = strings.Replace(identifier, "{0Episode}", fmt.Sprintf("%02d", episodes[0]), 1)
			identifier = strings.Replace(identifier, "{Episode}", strconv.Itoa(episodes[0]), 1)
			if len(teststr) == 2 {
				identifier = strings.Replace(identifier, teststr[1], "", -1)
			}
		} else {
			if len(teststr) == 2 {
				for i := range episodes {
					if i == 0 {
						identifier = strings.Replace(identifier, "{0Episode}", fmt.Sprintf("%02d", episodes[i]), 1)
						identifier = strings.Replace(identifier, "{Episode}", strconv.Itoa(episodes[i]), 1)
						identifier = strings.Replace(identifier, teststr[1], "", -1)
					} else {
						otheridentifier := teststr[1]
						otheridentifier = strings.Trim(otheridentifier, "(")
						otheridentifier = strings.Trim(otheridentifier, ")")
						otheridentifier = strings.Replace(otheridentifier, "{0Episode}", fmt.Sprintf("%02d", episodes[i]), 1)
						otheridentifier = strings.Replace(otheridentifier, "{Episode}", strconv.Itoa(episodes[i]), 1)
						identifier = identifier + otheridentifier
					}
				}
			}
		}
		filename = strings.Replace(filename, "{Identifier}", identifier, -1)
		proper := ""
		if m.Proper {
			proper = "proper"
		}
		filename = strings.Replace(filename, "{Proper}", proper, -1)
		extended := ""
		if m.Extended {
			extended = "extended"
		}
		if m.Tvdb == "" {
			m.Tvdb = strconv.Itoa(dbserie.ThetvdbID)
		}
		filename = strings.Replace(filename, "{Extended}", extended, -1)
		filename = strings.Replace(filename, "{Resolution}", m.Resolution, -1)
		filename = strings.Replace(filename, "{Quality}", m.Quality, -1)
		filename = strings.Replace(filename, "{Codec}", m.Codec, -1)
		filename = strings.Replace(filename, "{Audio}", m.Audio, -1)

		if !strings.HasPrefix(m.Tvdb, "tvdb") && len(m.Tvdb) >= 1 {
			m.Tvdb = "tvdb" + m.Tvdb
		}
		filename = strings.Replace(filename, "{Tvdb}", m.Tvdb, -1)
		filename = strings.Replace(filename, "{EpisodeTitle}", dbserieepisode.Title, -1)
		filename = strings.Replace(filename, "{EpisodeTitleSource}", episodetitle, -1)
		filename = strings.Replace(filename, "{TitleSource}", serietitle, -1)
		//filename = ReplaceStringObjectFields(filename, dbmovie)
		//filename = ReplaceStringObjectFields(filename, m)
		filename = strings.Replace(filename, "  ", " ", -1)
		filename = strings.Replace(filename, " ]", "]", -1)
		filename = strings.Replace(filename, "[ ", "[", -1)
		filename = strings.Replace(filename, "[ ]", "", -1)
		filename = strings.Replace(filename, "( )", "", -1)
		filename = strings.Replace(filename, "[]", "", -1)
		filename = strings.Replace(filename, "()", "", -1)
		filename = StringReplaceDiacritics(filename)

		foldername = Path(foldername, true)
		filename = Path(filename, false)
	}
	return
}

func getrootpath(foldername string) (string, string) {
	folders := make([]string, 0, 10)
	if strings.Contains(foldername, "/") {
		folders = strings.Split(foldername, "/")
	}
	if strings.Contains(foldername, "\\") {
		folders = strings.Split(foldername, "\\")
	}
	if !strings.Contains(foldername, "/") && !strings.Contains(foldername, "\\") {
		folders = []string{foldername}
	}
	foldername = strings.TrimPrefix(foldername, strings.TrimRight(folders[0], "/"))
	foldername = strings.Trim(foldername, "/")
	logger.Log.Debug("Removed ", folders[0], " from ", foldername)
	return foldername, strings.TrimRight(folders[0], "/")
}

func (s *Structure) MoveVideoFile(foldername string, filename string, videofiles []string, rootpath string) (videotarget string, moveok bool, moved int) {
	videotarget = filepath.Join(s.targetpath.Path, foldername)
	logger.Log.Debug("Default target ", videotarget)
	if rootpath != "" {
		videotarget = filepath.Join(rootpath, foldername)
		logger.Log.Debug("Changed target ", videotarget)
	}

	os.MkdirAll(videotarget, os.FileMode(0777))

	moveok, moved = scanner.MoveFiles(videofiles, videotarget, filename, s.sourcepath.AllowedVideoExtensions, s.sourcepath.AllowedVideoExtensionsNoRename)
	return
}

func (s *Structure) UpdateRootpath(videotarget string, foldername string, movie database.Movie, serie database.Serie) {
	rootpath := videotarget

	folders := strings.Split(foldername, "/")
	if len(folders) >= 2 {
		rootpath = trimStringInclAfterString(rootpath, strings.TrimRight(folders[1], "/"))
		rootpath = strings.TrimRight(rootpath, "/")
	}
	if strings.ToLower(s.groupType) == "movie" && movie.Rootpath == "" {
		database.UpdateColumn("movies", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
	} else if strings.ToLower(s.groupType) == "series" && serie.Rootpath == "" {
		database.UpdateColumn("series", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{serie.ID}})
	}
}

func (s *Structure) ReplaceLowerQualityFiles(oldfiles []string, movie database.Movie, series database.Serie) {
	if !s.targetpath.Replacelower || len(oldfiles) == 0 {
		return
	}
	if strings.ToLower(s.groupType) == "movie" {
		logger.Log.Debug("want to remove old files")
		for idx := range oldfiles {
			fileext := filepath.Ext(oldfiles[idx])
			err := scanner.RemoveFile(oldfiles[idx])
			if err == nil {
				logger.Log.Debug("Old File removed: ", oldfiles[idx])
				database.DeleteRow("movie_files", database.Query{Where: "movie_id = ? and location = ?", WhereArgs: []interface{}{movie.ID, oldfiles[idx]}})
				for idxext := range s.sourcepath.AllowedOtherExtensions {
					additionalfile := strings.Replace(oldfiles[idx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
					err := scanner.RemoveFile(additionalfile)
					if err == nil {
						logger.Log.Debug("Additional File removed: ", additionalfile)
					} else {
						logger.Log.Error("Additional File could not be removed: ", additionalfile, " Error: ", err)
					}
				}
			} else {
				logger.Log.Error("Old File could not be removed: ", oldfiles[idx], " Error: ", err)
			}
		}
	} else {
		for idx := range oldfiles {
			fileext := filepath.Ext(oldfiles[idx])
			err := scanner.RemoveFile(oldfiles[idx])
			if err == nil {
				logger.Log.Debug("Old File removed: ", oldfiles[idx])
				database.DeleteRow("serie_episode_files", database.Query{Where: "serie_id = ? and location = ?", WhereArgs: []interface{}{series.ID, oldfiles[idx]}})
				for idxext := range s.sourcepath.AllowedOtherExtensions {
					additionalfile := strings.Replace(oldfiles[idx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
					err := scanner.RemoveFile(additionalfile)
					if err == nil {
						logger.Log.Debug("Additional File removed: ", additionalfile)
					} else {
						logger.Log.Error("Additional File could not be removed: ", additionalfile, " Error: ", err)
					}
				}
			} else {
				logger.Log.Error("Old File could not be removed: ", oldfiles[idx], " Error: ", err)
			}
		}
	}
}

func (s *Structure) MoveAdditionalFiles(folder string, videotarget string, filename string, videofile string, sourcefileext string, videofilecount int) {
	if strings.ToLower(s.groupType) == "movie" || videofilecount == 1 {
		additionalfiles := scanner.GetFilesGoDir(folder, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, s.sourcepath.Blocked)
		if len(additionalfiles) >= 1 {
			scanner.MoveFiles(additionalfiles, videotarget, filename, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename)
		}
	} else {
		for idx := range s.sourcepath.AllowedOtherExtensions {
			additionalfile := strings.Replace(videofile, sourcefileext, s.sourcepath.AllowedOtherExtensions[idx], -1)
			scanner.MoveFiles([]string{additionalfile}, videotarget, filename, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename)
		}
	}
}

func StructureFolders(grouptype string, sourcepath config.PathsConfig, targetpath config.PathsConfig, configEntry config.MediaTypeConfig, list config.MediaListsConfig) {

	jobName := sourcepath.Path + "_" + list.Name
	defer func() {
		database.ReadWriteMu.Lock()
		delete(SeriesStructureJobRunning, jobName)
		database.ReadWriteMu.Unlock()
	}()
	if !configEntry.Structure {
		return
	}
	database.ReadWriteMu.Lock()
	if _, nok := SeriesStructureJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		SeriesStructureJobRunning[jobName] = true
		database.ReadWriteMu.Unlock()
	}

	logger.Log.Debug("Check Source: ", sourcepath.Path)
	folders := getSubFolders(sourcepath.Path)
	logger.Log.Debug("Folders found: ", folders)
	for idx := range folders {
		logger.Log.Debug("Process Folder: ", folders[idx])
		structure, err := NewStructure(configEntry, list, grouptype, folders[idx], sourcepath, targetpath)
		if err != nil {
			continue
		}
		if structure.CheckDisallowed() {
			if structure.sourcepath.DeleteDisallowed {
				emptyarr := make([]string, 0, 1)
				filesleft := scanner.GetFilesGoDir(folders[idx], structure.sourcepath.AllowedVideoExtensions, structure.sourcepath.AllowedVideoExtensionsNoRename, emptyarr)
				scanner.RemoveFiles(filesleft, emptyarr, emptyarr)

				scanner.CleanUpFolder(folders[idx], structure.sourcepath.CleanupsizeMB)
			}
			continue
		}
		removesmallfiles := false
		if strings.ToLower(structure.groupType) == "movie" {
			removesmallfiles = true
		}
		videofiles := structure.GetVideoFiles(folders[idx], sourcepath, removesmallfiles)

		if strings.ToLower(structure.groupType) == "movie" {
			if len(videofiles) >= 2 {
				//skip too many  files
				continue
			}
		}
		for fileidx := range videofiles {
			if filepath.Ext(videofiles[fileidx]) == "" {
				continue
			}
			if strings.ToLower(structure.groupType) == "series" {
				if structure.RemoveSmallVideoFile(videofiles[fileidx], sourcepath) {
					continue
				}
			}

			if strings.Contains(strings.ToLower(videofiles[fileidx]), strings.ToLower("_unpack")) {
				logger.Log.Debug("Unpacking - skipping: ", videofiles[fileidx])
				continue
			}
			m, err := structure.ParseFile(videofiles[fileidx], true, folders[idx], true)
			if err != nil {
				logger.Log.Error("Error parsing: ", videofiles[fileidx], " error: ", err)
				continue
			}
			if strings.ToLower(structure.groupType) == "movie" {
				var entriesfound int
				var movie database.Movie

				//determine list
				if m.Imdb != "" {
					movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ?", WhereArgs: []interface{}{m.Imdb}})
					for idxtmovie := range movies {
						foundinlist := false
						logger.Log.Debug("File found in other list - check list: ", videofiles[fileidx], movies[idxtmovie].Listname)
						for idxlisttest := range configEntry.Lists {
							if configEntry.Lists[idxlisttest].Name == movies[idxtmovie].Listname {
								if entriesfound == 0 && len(m.Imdb) >= 1 {
									movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and movies.listname = ?", WhereArgs: []interface{}{m.Imdb, configEntry.Lists[idxlisttest].Name}})
									if len(movies) == 1 {
										movie = movies[0]
									}
									entriesfound = len(movies)
									if entriesfound >= 1 {
										structure.list = configEntry.Lists[idxlisttest]
										list = configEntry.Lists[idxlisttest]
										foundinlist = true
										break
									}
								}
							}
						}
						if foundinlist {
							break
						}
					}
				}

				moviehistory, moviehistoryerr := database.GetMovieHistory(database.Query{Select: "movie_histories.dbmovie_id", InnerJoin: "movies on movies.id=movie_histories.movie_id", Where: "movie_histories.title = ? and movies.listname = ?", WhereArgs: []interface{}{filepath.Base(folders[idx]), list.Name}, OrderBy: "movie_histories.ID desc"})

				if moviehistoryerr == nil {
					movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.id = ? and movies.listname = ?", WhereArgs: []interface{}{moviehistory.DbmovieID, list.Name}})
					if len(movies) == 1 {
						logger.Log.Debug("Found Movie by history_title")
						movie = movies[0]
					}
					entriesfound = len(movies)
				}

				if !config.ConfigCheck("quality_" + list.Template_quality) {
					logger.Log.Error("Template Quality not found: ", list.Template_quality)
					return
				}
				var cfg_quality config.QualityConfig
				config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

				if entriesfound == 0 && len(m.Imdb) == 0 {
					lists := make([]string, 0, len(configEntry.Lists))
					for idxlisttest := range configEntry.Lists {
						lists = append(lists, configEntry.Lists[idxlisttest].Name)
					}
					logger.Log.Debug("Find Movie using title: ", m.Title, " and year: ", m.Year, " and lists: ", lists)
					list.Name, m.Imdb, _ = movieFindListByTitle(m.Title, strconv.Itoa(m.Year), lists, cfg_quality.CheckYear1, "structure")
				}
				if entriesfound == 0 && len(m.Imdb) >= 1 {
					movies, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and movies.listname = ?", WhereArgs: []interface{}{m.Imdb, list.Name}})
					if len(movies) == 1 {
						logger.Log.Debug("Found Movie by title")
						movie = movies[0]
					}
					entriesfound = len(movies)
				}
				if entriesfound == 0 && len(m.Imdb) >= 1 {
					counter, _ := database.CountRows("movies", database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ?", WhereArgs: []interface{}{m.Imdb, list.Name}})
					if counter >= 1 {
						logger.Log.Debug("Skipped Movie by title")
						continue
					}
				}
				if movie.ID >= 1 && entriesfound >= 1 {
					errpars := structure.ParseFileAdditional(videofiles[fileidx], m, folders[idx], structure.targetpath.DeleteWrongLanguage)
					if errpars != nil {
						logger.Log.Error("Error fprobe video: ", videofiles[fileidx], " error: ", errpars)
						continue
					}
					oldfiles, _, errold := structure.CheckLowerQualTarget(folders[idx], videofiles[fileidx], *m, true, movie)
					if errold != nil {
						logger.Log.Error("Error checking oldfiles: ", videofiles[fileidx], " error: ", errold)
						continue
					}
					foldername, filename := structure.GenerateNaming(videofiles[fileidx], *m, movie, database.Serie{}, "", database.SerieEpisode{}, "", []int{})

					sourcefileext := filepath.Ext(videofiles[fileidx])
					videotarget, moveok, moved := structure.MoveVideoFile(foldername, filename, []string{videofiles[fileidx]}, movie.Rootpath)
					if moveok && moved >= 1 {
						structure.UpdateRootpath(videotarget, foldername, movie, database.Serie{})
						structure.ReplaceLowerQualityFiles(oldfiles, movie, database.Serie{})
						structure.MoveAdditionalFiles(folders[idx], videotarget, filename, videofiles[fileidx], sourcefileext, len(videotarget))

						structure.Notify(videotarget, filename, videofiles[fileidx], movie, database.SerieEpisode{}, oldfiles)
						scanner.CleanUpFolder(folders[idx], sourcepath.CleanupsizeMB)

						//updatemovie
						targetfile := filepath.Join(videotarget, filename)
						database.InsertArray("movie_files",
							[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id"},
							[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie.ID, movie.DbmovieID})

						reached := false

						cutoffPrio := NewCutoffPrio(configEntry, cfg_quality)
						if m.Priority >= cutoffPrio.Priority {
							reached = true
						}
						database.UpdateColumn("movies", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
						database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
					} else {
						logger.Log.Error("Error moving video - unknown reason")
					}
				} else {
					logger.Log.Debug("Movie not matched: ", videofiles[fileidx], " list ", list.Name)
				}
			} else if strings.ToLower(structure.groupType) == "series" {
				//SerieEpisodeHistory, _ := database.QuerySerieEpisodeHistory(database.Query{InnerJoin: "series on series.id=serie_episode_histories.serie_id", Where: "serie_episode_histories.target = ? and series.listname = ?", WhereArgs: []interface{}{filepath.Base(folders[idx]), list.Name}})

				yearstr := strconv.Itoa(m.Year)
				titleyear := m.Title + " (" + yearstr + ")"
				seriestitle := ""
				re, _ := regexp.Compile(`^(.*)(?i)(?:(?:\.| - |-)S?(?:\d+)[ex](?:\d+)(?:[^0-9]|$))`)
				matched := re.FindStringSubmatch(filepath.Base(videofiles[fileidx]))
				if len(matched) >= 2 {
					seriestitle = matched[1]
				}

				if !config.ConfigCheck("quality_" + list.Template_quality) {
					return
				}
				var cfg_quality config.QualityConfig
				config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

				//find dbseries
				series, entriesfound := findSerieByParser(*m, titleyear, seriestitle, list.Name)
				if entriesfound >= 1 {
					errpars := structure.ParseFileAdditional(videofiles[fileidx], m, folders[idx], structure.targetpath.DeleteWrongLanguage)
					if errpars != nil {
						logger.Log.Error("Error fprobe video: ", videofiles[fileidx], " error: ", errpars)
						continue
					}
					oldfiles, episodes, allowimport, serietitle, episodetitle, seriesEpisode := structure.GetSeriesEpisodes(series, videofiles[fileidx], *m, folders[idx])
					if allowimport {
						foldername, filename := structure.GenerateNaming(videofiles[fileidx], *m, database.Movie{}, series, serietitle, seriesEpisode, episodetitle, episodes)
						sourcefileext := filepath.Ext(videofiles[fileidx])
						videotarget, moveok, moved := structure.MoveVideoFile(foldername, filename, []string{videofiles[fileidx]}, series.Rootpath)
						if moveok && moved >= 1 {
							structure.UpdateRootpath(videotarget, foldername, database.Movie{}, series)
							structure.ReplaceLowerQualityFiles(oldfiles, database.Movie{}, series)
							structure.MoveAdditionalFiles(folders[idx], videotarget, filename, videofiles[fileidx], sourcefileext, len(videotarget))
							structure.Notify(videotarget, filename, videofiles[fileidx], database.Movie{}, seriesEpisode, oldfiles)
							scanner.CleanUpFolder(folders[idx], sourcepath.CleanupsizeMB)

							//updateserie
							targetfile := filepath.Join(videotarget, filename)
							database.InsertArray("serie_episode_files",
								[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id"},
								[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, seriesEpisode.SerieID, seriesEpisode.ID, seriesEpisode.DbserieEpisodeID, seriesEpisode.DbserieID})

							reached := false

							cutoffPrio := NewCutoffPrio(configEntry, cfg_quality)
							if m.Priority >= cutoffPrio.Priority {
								reached = true
							}
							database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{seriesEpisode.ID}})
							database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{seriesEpisode.ID}})
						}
					}
				}
			}
		}
	}
}
func (s *Structure) Notify(videotarget string, filename string, videofile string, movie database.Movie, serieepisode database.SerieEpisode, oldfiles []string) {
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, _ := database.GetDbmovie(database.Query{Select: "title, year, imdb_id", Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
		for idx := range s.configEntry.Notification {
			notifier("added_data", s.configEntry.Notification[idx], InputNotifier{
				Targetpath:     filepath.Join(videotarget, filename),
				SourcePath:     videofile,
				Title:          dbmovie.Title,
				Year:           strconv.Itoa(dbmovie.Year),
				Imdb:           dbmovie.ImdbID,
				Replaced:       oldfiles,
				ReplacedPrefix: s.configEntry.Notification[idx].ReplacedPrefix,
				Configuration:  s.list.Name,
			})
		}
	} else {
		dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Select: "season, episode, identifier, dbserie_id", Where: "id=?", WhereArgs: []interface{}{serieepisode.DbserieEpisodeID}})
		dbserie, _ := database.GetDbserie(database.Query{Select: "seriename, firstaired, thetvdb_id", Where: "id=?", WhereArgs: []interface{}{dbserieepisode.DbserieID}})
		for idx := range s.configEntry.Notification {
			notifier("added_data", s.configEntry.Notification[idx], InputNotifier{
				Targetpath:     filepath.Join(videotarget, filename),
				SourcePath:     videofile,
				Title:          dbserie.Seriename,
				Year:           dbserie.Firstaired,
				Season:         dbserieepisode.Season,
				Episode:        dbserieepisode.Episode,
				Series:         dbserie.Seriename,
				Identifier:     dbserieepisode.Identifier,
				Tvdb:           strconv.Itoa(dbserie.ThetvdbID),
				Replaced:       oldfiles,
				ReplacedPrefix: s.configEntry.Notification[idx].ReplacedPrefix,
				Configuration:  s.list.Name,
			})
		}
	}
}

func (s *Structure) GetSeriesEpisodes(series database.Serie, videofile string, m ParseInfo, folder string) (oldfiles []string, episodes []int, allowimport bool, serietitle string, episodetitle string, SeriesEpisode database.SerieEpisode) {
	dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
	r := regexp.MustCompile(`(?i)s?[0-9]{1,4}((?:(?:-?[ex][0-9]{1,3})+))|(\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2})(?:\b|_)`)
	teststr := r.FindStringSubmatch(m.Identifier)
	if len(teststr) == 0 {
		logger.Log.Debug("In Identifier not found: ", videofile, " Identifier: ", m.Identifier)
		return
	}

	episodeArray := getEpisodeArray(dbserie.Identifiedby, teststr[1], teststr[2])

	for idx := range episodeArray {
		epi := episodeArray[idx]
		epi = strings.Trim(epi, "-EX")
		if strings.ToLower(dbserie.Identifiedby) != "date" {
			epi = strings.TrimLeft(epi, "0")
		}
		epinum, _ := strconv.Atoi(epi)
		if epi == "" {
			continue
		}
		episodes = append(episodes, epinum)

		var SeriesEpisodeerr error
		if strings.ToLower(dbserie.Identifiedby) == "date" {
			SeriesEpisode, SeriesEpisodeerr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Identifier = ?", WhereArgs: []interface{}{series.ID, strings.Replace(epi, ".", "-", -1)}})

		} else {
			SeriesEpisode, SeriesEpisodeerr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Season = ? AND DbSerie_episodes.Episode = ?", WhereArgs: []interface{}{series.ID, m.Season, epi}})
		}
		if SeriesEpisodeerr == nil {
			dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.DbserieEpisodeID}})

			reepi, _ := regexp.Compile(`^(.*)(?i)` + m.Identifier + `(?:\.| |-)(.*)$`)
			matched := reepi.FindStringSubmatch(filepath.Base(videofile))
			if len(matched) >= 2 {
				logger.Log.Debug("matched title 1: ", matched[1])
				logger.Log.Debug("matched title 2: ", matched[2])
				episodetitle = matched[2]
				serietitle = matched[1]
				episodetitle = trimStringInclAfterString(episodetitle, "XXX")
				episodetitle = trimStringInclAfterString(episodetitle, m.Quality)
				episodetitle = trimStringInclAfterString(episodetitle, m.Resolution)
				episodetitle = strings.Trim(episodetitle, ".")
				episodetitle = strings.Replace(episodetitle, ".", " ", -1)

				serietitle = strings.Trim(serietitle, ".")
				serietitle = strings.Replace(serietitle, ".", " ", -1)
				logger.Log.Debug("trimmed title: ", episodetitle)
			}
			if len(episodetitle) == 0 && dbserieepisode.Title != "" {
				episodetitle = dbserieepisode.Title
				logger.Log.Debug("use db title: ", episodetitle)
			}

			if !config.ConfigCheck("quality_" + s.list.Template_quality) {
				return
			}
			var cfg_quality config.QualityConfig
			config.ConfigGet("quality_"+s.list.Template_quality, &cfg_quality)

			//episodeids = append(episodeids, SeriesEpisode.ID)
			old_prio := getHighestEpisodePriorityByFiles(SeriesEpisode, s.configEntry, cfg_quality)

			episodefiles, _ := database.QuerySerieEpisodeFiles(database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{SeriesEpisode.ID}})
			if m.Priority > old_prio {
				allowimport = true
				for idx := range episodefiles {
					//_, oldfilename := filepath.Split(episodefile.Location)
					entry_prio := getSerieDBPriority(episodefiles[idx], s.configEntry, cfg_quality)
					if m.Priority > entry_prio {
						oldfiles = append(oldfiles, episodefiles[idx].Location)
					}
				}
			} else if len(episodefiles) == 0 {
				allowimport = true
			} else {
				videoext := filepath.Ext(videofile)
				err := scanner.RemoveFile(videofile)
				if err == nil {
					logger.Log.Debug("Lower Qual Import File removed: ", videofile, " oldprio ", old_prio, " fileprio ", m.Priority)

					for idx := range s.sourcepath.AllowedOtherExtensions {
						additionalfile := strings.Replace(videofile, videoext, s.sourcepath.AllowedOtherExtensions[idx], -1)
						err := scanner.RemoveFile(additionalfile)
						if err == nil {
							logger.Log.Debug("Lower Qual Import Additional File removed: ", additionalfile)
						}
					}
					scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
				}
				continue
			}
			if len(episodefiles) == 0 {
				allowimport = true
			} else {
				if !allowimport {
					logger.Log.Debug("Import Not allowed - no source files found")
				}
			}
		} else {
			logger.Log.Debug("Import Not allowed - episode not matched - file: ", videofile, " - Season: ", m.Season, " - Episode: ", epi)
		}
	}
	return
}

package utils

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
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
	videofiles := scanner.GetFilesDir(folder, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename, pathconfig.Blocked)
	if removesmallfiles {
		for idx := range videofiles {
			info, err := os.Stat(videofiles[idx])
			if pathconfig.MinVideoSize > 0 && err == nil {
				if info.Size() < int64(pathconfig.MinVideoSize*1024*1024) {
					scanner.RemoveFiles([]string{videofiles[idx]}, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename)
				}
			}
		}
		videofiles = scanner.GetFilesDir(folder, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename, pathconfig.Blocked)
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
		logger.Log.Debug("Parse of folder ", filepath.Base(folder), m)

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

func (s Structure) FileCleanup(folder string, videofile string) {
	if strings.ToLower(s.groupType) == "movie" || videofile == "" {
		filesleft := scanner.GetFilesDir(folder, s.sourcepath.AllowedVideoExtensions, s.sourcepath.AllowedVideoExtensionsNoRename, []string{})
		scanner.RemoveFiles(filesleft, []string{}, []string{})

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
func (s *Structure) ParseFileAdditional(videofile string, m *ParseInfo, folder string, deletewronglanguage bool, wantedruntime int) (err error) {
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
	if m.Runtime >= 1 && s.targetpath.CheckRuntime && wantedruntime != 0 && s.targetpath.MaxRuntimeDifference != 0 {
		intruntime := int(m.Runtime / 60)
		if intruntime != wantedruntime {
			maxdifference := s.targetpath.MaxRuntimeDifference
			if m.Extended && strings.ToLower(s.groupType) == "movie" {
				maxdifference += 10
			}
			difference := 0
			if wantedruntime > intruntime {
				difference = wantedruntime - intruntime
			} else {
				difference = intruntime - wantedruntime
			}
			if difference > maxdifference {
				if s.targetpath.DeleteWrongRuntime {
					s.FileCleanup(folder, videofile)
				}
				logger.Log.Error("Wrong runtime: Wanted", wantedruntime, " Having", intruntime, " difference", difference, " for", m.File)
				return errors.New("wrong runtime")
			}
		}
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
			s.FileCleanup(folder, videofile)
		}
		if !language_ok {
			logger.Log.Error("Wrong language: Wanted", s.targetpath.Allowed_languages, " Having", m.Languages, " for", m.File)
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
					oldfilesadd := scanner.GetFilesDir(oldpath, []string{}, []string{}, []string{})
					logger.Log.Debug("found old files ", len(oldfilesadd))
					oldfiles = append(oldfiles, oldfilesadd...)
				}
			}
		}
		return oldfiles, oldpriority, nil
	} else if len(moviefiles) >= 1 {
		logger.Log.Info("Skipped import due to lower quality: ", videofile)
		err := errors.New("import file has lower quality")
		if cleanuplowerquality {
			s.FileCleanup(folder, videofile)
		}
		return []string{}, oldpriority, err
	}
	return []string{}, oldpriority, nil
}

type parser struct {
	Dbmovie            database.Dbmovie
	Movie              database.Movie
	Serie              database.Serie
	Dbserie            database.Dbserie
	DbserieEpisode     database.DbserieEpisode
	Source             ParseInfo
	TitleSource        string
	EpisodeTitleSource string
	Identifier         string
	Episodes           []int
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
		foldername, filename = path.Split(naming)
		//Naming = '{Title} ({Year})/{Title} ({Year}) German [{Resolution} {Quality} {Codec}]'
		if movie.Rootpath != "" {
			foldername, _ = getrootpath(foldername)
		}

		forparser.Dbmovie = dbmovie
		forparser.Source = m
		//Naming = '{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper 1}} proper{{end}}{{if eq .Source.Extended 1}} extended{{end}}] ({{.Source.Imdb}})'
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

func (s *Structure) GenerateNamingTemplate(videofile string, m ParseInfo, movie database.Movie,
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
		forparser.TitleSource = movietitle
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

		foldername, filename = path.Split(naming)
		if movie.Rootpath != "" {
			foldername, _ = getrootpath(foldername)
		}

		forparser.Dbmovie = dbmovie
		if !strings.HasPrefix(m.Imdb, "tt") && len(m.Imdb) >= 1 {
			m.Imdb = "tt" + m.Imdb
		}
		if m.Imdb == "" {
			m.Imdb = dbmovie.ImdbID
		}
		forparser.Source = m

		logger.Log.Debug("Parse folder: " + foldername)
		tmplfolder, err := template.New("tmplfolder").Parse(foldername)
		if err != nil {
			logger.Log.Error(err)
		}
		var doc bytes.Buffer
		err = tmplfolder.Execute(&doc, forparser)
		if err != nil {
			logger.Log.Error(err)
		}
		foldername = doc.String()
		logger.Log.Debug("Folder parsed: " + foldername)
		foldername = strings.Trim(foldername, ".")
		foldername = StringReplaceDiacritics(foldername)
		foldername = Path(foldername, true)

		logger.Log.Debug("Parse file: " + filename)
		tmplfile, err := template.New("tmplfile").Parse(filename)
		if err != nil {
			logger.Log.Error(err)
		}
		var docfile bytes.Buffer
		err = tmplfile.Execute(&docfile, forparser)
		if err != nil {
			logger.Log.Error(err)
		}
		filename = docfile.String()
		logger.Log.Debug("File parsed: " + filename)
		filename = strings.Trim(filename, ".")
		filename = strings.Replace(filename, "  ", " ", -1)
		filename = strings.Replace(filename, " ]", "]", -1)
		filename = strings.Replace(filename, "[ ", "[", -1)
		filename = strings.Replace(filename, "[ ]", "", -1)
		filename = strings.Replace(filename, "( )", "", -1)
		filename = strings.Replace(filename, "[]", "", -1)
		filename = strings.Replace(filename, "()", "", -1)
		filename = strings.Replace(filename, "  ", " ", -1)
		filename = StringReplaceDiacritics(filename)
		filename = Path(filename, true)
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

		forparser.Serie = series
		forparser.TitleSource = serietitle
		forparser.EpisodeTitleSource = episodetitle
		forparser.Dbserie = dbserie
		forparser.DbserieEpisode = dbserieepisode
		forparser.Episodes = episodes
		if m.Tvdb == "" {
			m.Tvdb = strconv.Itoa(dbserie.ThetvdbID)
		}
		if !strings.HasPrefix(m.Tvdb, "tvdb") && len(m.Tvdb) >= 1 {
			m.Tvdb = "tvdb" + m.Tvdb
		}
		forparser.Source = m

		//Naming = '{Title}/Season {Season}/{Title} - {Identifier} - German [{Resolution} {Quality} {Codec}] (tvdb{tvdb})'
		//Naming = '{{.Dbmovie.Title}} ({{.Dbmovie.Year}})/{{.Dbmovie.Title}} ({{.Dbmovie.Year}}) [{{.Source.Resolution}} {{.Source.Quality}} {{.Source.Codec}} {{.Source.Audio}}{{if eq .Source.Proper 1}} proper{{end}}] ({{.Source.Imdb}})'
		logger.Log.Debug("Parse folder: " + foldername)
		tmplfolder, err := template.New("tmplfolder").Parse(foldername)
		if err != nil {
			logger.Log.Error(err)
		}
		var doc bytes.Buffer
		err = tmplfolder.Execute(&doc, forparser)
		if err != nil {
			logger.Log.Error(err)
		}
		foldername = doc.String()
		logger.Log.Debug("Folder parsed: " + foldername)
		foldername = strings.Trim(foldername, ".")
		foldername = StringReplaceDiacritics(foldername)
		foldername = Path(foldername, true)
		//S{0Season}E{0Episode}(E{0Episode})

		logger.Log.Debug("Parse file: " + filename)
		tmplfile, err := template.New("tmplfile").Parse(filename)
		if err != nil {
			logger.Log.Error(err)
		}
		var docfile bytes.Buffer
		err = tmplfile.Execute(&docfile, forparser)
		if err != nil {
			logger.Log.Error(err)
		}
		filename = docfile.String()
		logger.Log.Debug("File parsed: " + filename)
		filename = strings.Trim(filename, ".")
		filename = strings.Replace(filename, "  ", " ", -1)
		filename = strings.Replace(filename, " ]", "]", -1)
		filename = strings.Replace(filename, "[ ", "[", -1)
		filename = strings.Replace(filename, "[ ]", "", -1)
		filename = strings.Replace(filename, "( )", "", -1)
		filename = strings.Replace(filename, "[]", "", -1)
		filename = strings.Replace(filename, "()", "", -1)
		filename = strings.Replace(filename, "  ", " ", -1)
		filename = StringReplaceDiacritics(filename)
		filename = Path(filename, true)
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
	if s.targetpath.Usepresort {
		return
	}
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

func (s *Structure) MoveOldFiles(oldfiles []string, movie database.Movie, series database.Serie) {
	if !s.targetpath.MoveReplaced || len(oldfiles) == 0 || s.targetpath.MoveReplacedTargetPath == "" {
		return
	}
	if strings.ToLower(s.groupType) == "movie" {
		logger.Log.Debug("want to remove old files")
		for idx := range oldfiles {
			fileext := filepath.Ext(oldfiles[idx])
			move_ok, _ := scanner.MoveFiles([]string{oldfiles[idx]}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{})
			if move_ok {
				logger.Log.Debug("Old File moved: ", oldfiles[idx])
				database.DeleteRow("movie_files", database.Query{Where: "movie_id = ? and location = ?", WhereArgs: []interface{}{movie.ID, oldfiles[idx]}})
				for idxext := range s.sourcepath.AllowedOtherExtensions {
					additionalfile := strings.Replace(oldfiles[idx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
					move_sub_ok, _ := scanner.MoveFiles([]string{additionalfile}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{})
					if move_sub_ok {
						logger.Log.Debug("Additional File removed: ", additionalfile)
					} else {
						logger.Log.Error("Additional File could not be removed: ", additionalfile)
					}
				}
			} else {
				logger.Log.Error("Old File could not be removed: ", oldfiles[idx])
			}
		}
	} else {
		for idx := range oldfiles {
			fileext := filepath.Ext(oldfiles[idx])
			move_ok, _ := scanner.MoveFiles([]string{oldfiles[idx]}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{})
			if move_ok {
				logger.Log.Debug("Old File removed: ", oldfiles[idx])
				database.DeleteRow("serie_episode_files", database.Query{Where: "serie_id = ? and location = ?", WhereArgs: []interface{}{series.ID, oldfiles[idx]}})
				for idxext := range s.sourcepath.AllowedOtherExtensions {
					additionalfile := strings.Replace(oldfiles[idx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
					move_sub_ok, _ := scanner.MoveFiles([]string{additionalfile}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{})
					if move_sub_ok {
						logger.Log.Debug("Additional File removed: ", additionalfile)
					} else {
						logger.Log.Error("Additional File could not be removed: ", additionalfile)
					}
				}
			} else {
				logger.Log.Error("Old File could not be removed: ", oldfiles[idx])
			}
		}
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
		additionalfiles := scanner.GetFilesDir(folder, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, s.sourcepath.Blocked)
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

func StructureSeries(structure Structure, folder string, m *ParseInfo, series database.Serie, videofile string, deletewronglanguage bool) {
	if !config.ConfigCheck("quality_" + structure.list.Template_quality) {
		logger.Log.Error("Template Quality not found: ", structure.list.Template_quality)
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+structure.list.Template_quality, &cfg_quality)

	dbseries, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
	runtime, _ := strconv.Atoi(dbseries.Runtime)
	oldfiles, episodes, allowimport, serietitle, episodetitle, seriesEpisode, seriesEpisodes, epiruntime := structure.GetSeriesEpisodes(series, videofile, *m, folder)
	if epiruntime != 0 {
		runtime = epiruntime
	}
	errpars := structure.ParseFileAdditional(videofile, m, folder, deletewronglanguage, runtime*len(episodes))
	if errpars != nil {
		logger.Log.Error("Error fprobe video: ", videofile, " error: ", errpars)
		return
	}
	if allowimport {
		foldername, filename := structure.GenerateNamingTemplate(videofile, *m, database.Movie{}, series, serietitle, seriesEpisode, episodetitle, episodes)
		sourcefileext := filepath.Ext(videofile)
		structure.MoveOldFiles(oldfiles, database.Movie{}, series)
		mediatargetpath := series.Rootpath
		if structure.targetpath.Usepresort && structure.targetpath.PresortFolderPath != "" {
			mediatargetpath = filepath.Join(structure.targetpath.PresortFolderPath, foldername)
		}
		videotarget, moveok, moved := structure.MoveVideoFile(foldername, filename, []string{videofile}, mediatargetpath)
		if moveok && moved >= 1 {
			structure.UpdateRootpath(videotarget, foldername, database.Movie{}, series)
			var oldfiles_remove []string
			for idx := range oldfiles {
				if strings.HasPrefix(strings.ToLower(oldfiles[idx]), strings.ToLower(videotarget)) && strings.Contains(strings.ToLower(oldfiles[idx]), strings.ToLower(filename)) {
					//skip
				} else {
					oldfiles_remove = append(oldfiles_remove, oldfiles[idx])
				}
			}
			structure.ReplaceLowerQualityFiles(oldfiles_remove, database.Movie{}, series)
			structure.MoveAdditionalFiles(folder, videotarget, filename, videofile, sourcefileext, len(videotarget))
			structure.Notify(videotarget, filename, videofile, *m, database.Movie{}, seriesEpisode, oldfiles)
			scanner.CleanUpFolder(folder, structure.sourcepath.CleanupsizeMB)

			//updateserie
			targetfile := filepath.Join(videotarget, filename)

			reached := false

			cutoffPrio := NewCutoffPrio(structure.configEntry, cfg_quality)
			if m.Priority >= cutoffPrio.Priority {
				reached = true
			}
			for idxepi := range seriesEpisodes {
				database.InsertArray("serie_episode_files",
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id"},
					[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), structure.list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, seriesEpisodes[idxepi].SerieID, seriesEpisodes[idxepi].ID, seriesEpisodes[idxepi].DbserieEpisodeID, seriesEpisodes[idxepi].DbserieID})

				database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{seriesEpisodes[idxepi].ID}})
				database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{seriesEpisodes[idxepi].ID}})
			}
		}
	}
}
func StructureMovie(structure Structure, folder string, m *ParseInfo, movie database.Movie, videofile string, deletewronglanguage bool) {
	if !config.ConfigCheck("quality_" + structure.list.Template_quality) {
		logger.Log.Error("Template Quality not found: ", structure.list.Template_quality)
		return
	}
	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+structure.list.Template_quality, &cfg_quality)

	dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
	errpars := structure.ParseFileAdditional(videofile, m, folder, deletewronglanguage, dbmovie.Runtime)
	if errpars != nil {
		logger.Log.Error("Error fprobe video: ", videofile, " error: ", errpars)
		return
	}
	oldfiles, _, errold := structure.CheckLowerQualTarget(folder, videofile, *m, true, movie)
	if errold != nil {
		logger.Log.Error("Error checking oldfiles: ", videofile, " error: ", errold)
		return
	}
	foldername, filename := structure.GenerateNamingTemplate(videofile, *m, movie, database.Serie{}, "", database.SerieEpisode{}, "", []int{})

	sourcefileext := filepath.Ext(videofile)

	structure.MoveOldFiles(oldfiles, movie, database.Serie{})
	mediatargetpath := movie.Rootpath
	if structure.targetpath.Usepresort && structure.targetpath.PresortFolderPath != "" {
		mediatargetpath = filepath.Join(structure.targetpath.PresortFolderPath, foldername)
	}
	videotarget, moveok, moved := structure.MoveVideoFile(foldername, filename, []string{videofile}, mediatargetpath)
	if moveok && moved >= 1 {
		structure.UpdateRootpath(videotarget, foldername, movie, database.Serie{})
		var oldfiles_remove []string
		for idx := range oldfiles {
			if strings.HasPrefix(strings.ToLower(oldfiles[idx]), strings.ToLower(videotarget)) && strings.Contains(strings.ToLower(oldfiles[idx]), strings.ToLower(filename)) {
				//skip
			} else {
				oldfiles_remove = append(oldfiles_remove, oldfiles[idx])
			}
		}
		structure.ReplaceLowerQualityFiles(oldfiles_remove, movie, database.Serie{})
		structure.MoveAdditionalFiles(folder, videotarget, filename, videofile, sourcefileext, len(videotarget))

		structure.Notify(videotarget, filename, videofile, *m, movie, database.SerieEpisode{}, oldfiles)
		scanner.CleanUpFolder(folder, structure.sourcepath.CleanupsizeMB)

		//updatemovie
		targetfile := filepath.Join(videotarget, filename)
		database.InsertArray("movie_files",
			[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id"},
			[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), structure.list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie.ID, movie.DbmovieID})

		reached := false

		cutoffPrio := NewCutoffPrio(structure.configEntry, cfg_quality)
		if m.Priority >= cutoffPrio.Priority {
			reached = true
		}
		database.UpdateColumn("movies", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
		database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
	} else {
		logger.Log.Error("Error moving video - unknown reason")
	}
}
func StructureSingleFolderAs(folder string, id int, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepath config.PathsConfig, targetpath config.PathsConfig, configEntry config.MediaTypeConfig, list config.MediaListsConfig) {
	logger.Log.Debug("Process Folder: ", folder)
	if disableruntimecheck {
		targetpath.CheckRuntime = false
	}
	structure, err := NewStructure(configEntry, list, grouptype, folder, sourcepath, targetpath)
	if err != nil {
		return
	}
	if disabledisallowed {
		structure.sourcepath.Disallowed = []string{}
	}
	if structure.CheckDisallowed() {
		if structure.sourcepath.DeleteDisallowed {
			structure.FileCleanup(folder, "")
		}
		return
	}
	removesmallfiles := false
	if strings.ToLower(structure.groupType) == "movie" {
		removesmallfiles = true
	}
	videofiles := structure.GetVideoFiles(folder, sourcepath, removesmallfiles)

	if strings.ToLower(structure.groupType) == "movie" {
		if len(videofiles) >= 2 {
			//skip too many  files
			return
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
		deletewronglanguage := structure.targetpath.DeleteWrongLanguage
		if disabledeletewronglanguage {
			deletewronglanguage = false
		}
		m, err := structure.ParseFile(videofiles[fileidx], true, folder, deletewronglanguage)
		if err != nil {
			logger.Log.Error("Error parsing: ", videofiles[fileidx], " error: ", err)
			continue
		}
		if strings.ToLower(structure.groupType) == "movie" {
			movie, movierr := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{id}})
			if movierr != nil {
				return
			}

			if movie.ID >= 1 {
				StructureMovie(structure, folder, m, movie, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Debug("Movie not matched: ", videofiles[fileidx])
			}
		} else if strings.ToLower(structure.groupType) == "series" {
			//SerieEpisodeHistory, _ := database.QuerySerieEpisodeHistory(database.Query{InnerJoin: "series on series.id=serie_episode_histories.serie_id", Where: "serie_episode_histories.target = ? and series.listname = ?", WhereArgs: []interface{}{filepath.Base(folders[idx]), list.Name}})
			if !config.ConfigCheck("quality_" + list.Template_quality) {
				return
			}
			var cfg_quality config.QualityConfig
			config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)

			//find dbseries
			series, serierr := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{id}})
			if serierr != nil {
				logger.Log.Debug("Serie not matched: ", videofiles[fileidx])
				return
			}
			StructureSeries(structure, folder, m, series, videofiles[fileidx], deletewronglanguage)
		}
	}
}

func StructureSingleFolder(folder string, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepath config.PathsConfig, targetpath config.PathsConfig, configEntry config.MediaTypeConfig, list config.MediaListsConfig) {
	logger.Log.Debug("Process Folder: ", folder)
	if disableruntimecheck {
		targetpath.CheckRuntime = false
	}
	structure, err := NewStructure(configEntry, list, grouptype, folder, sourcepath, targetpath)
	if err != nil {
		return
	}
	if disabledisallowed {
		structure.sourcepath.Disallowed = []string{}
	}
	if structure.CheckDisallowed() {
		if structure.sourcepath.DeleteDisallowed {
			structure.FileCleanup(folder, "")
		}
		return
	}
	removesmallfiles := false
	if strings.ToLower(structure.groupType) == "movie" {
		removesmallfiles = true
	}
	videofiles := structure.GetVideoFiles(folder, sourcepath, removesmallfiles)

	if strings.ToLower(structure.groupType) == "movie" {
		if len(videofiles) >= 2 {
			//skip too many  files
			return
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
		deletewronglanguage := structure.targetpath.DeleteWrongLanguage
		if disabledeletewronglanguage {
			deletewronglanguage = false
		}
		m, err := structure.ParseFile(videofiles[fileidx], true, folder, deletewronglanguage)
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
								moviesget, _ := database.QueryMovies(database.Query{Select: "movies.*", InnerJoin: "Dbmovies on Dbmovies.id = movies.dbmovie_id", Where: "Dbmovies.imdb_id = ? and movies.listname = ?", WhereArgs: []interface{}{m.Imdb, configEntry.Lists[idxlisttest].Name}})
								if len(moviesget) == 1 {
									movie = moviesget[0]
								}
								entriesfound = len(moviesget)
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

			moviehistory, moviehistoryerr := database.GetMovieHistory(database.Query{Select: "movie_histories.dbmovie_id", InnerJoin: "movies on movies.id=movie_histories.movie_id", Where: "movie_histories.title = ? and movies.listname = ?", WhereArgs: []interface{}{filepath.Base(folder), list.Name}, OrderBy: "movie_histories.ID desc"})

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

			for idxstrip := range cfg_quality.TitleStripSuffixForSearch {
				if strings.HasSuffix(strings.ToLower(m.Title), strings.ToLower(cfg_quality.TitleStripSuffixForSearch[idxstrip])) {
					m.Title = trimStringInclAfterStringInsensitive(m.Title, cfg_quality.TitleStripSuffixForSearch[idxstrip])
					m.Title = strings.Trim(m.Title, " ")
				}
			}
			for idxstrip := range cfg_quality.TitleStripPrefixForSearch {
				if strings.HasPrefix(strings.ToLower(m.Title), strings.ToLower(cfg_quality.TitleStripPrefixForSearch[idxstrip])) {
					m.Title = trimStringPrefixInsensitive(m.Title, cfg_quality.TitleStripPrefixForSearch[idxstrip])
					m.Title = strings.Trim(m.Title, " ")
				}
			}

			if entriesfound == 0 && len(m.Imdb) == 0 {
				lists := make([]string, 0, len(configEntry.Lists))
				for idxlisttest := range configEntry.Lists {
					lists = append(lists, configEntry.Lists[idxlisttest].Name)
				}
				logger.Log.Debug("Find Movie using title: ", m.Title, " and year: ", m.Year, " and lists: ", lists)
				list.Name, m.Imdb, _, _ = movieFindListByTitle(m.Title, strconv.Itoa(m.Year), lists, "structure")
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
				if cfg_quality.CheckTitle {
					titlefound := false
					dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
					if cfg_quality.CheckTitle && checknzbtitle(dbmovie.Title, m.Title) && len(dbmovie.Title) >= 1 {
						titlefound = true
					}
					if !titlefound {
						alttitlefound := false
						dbtitle, _ := database.QueryDbmovieTitle(database.Query{Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.DbmovieID}})

						for idxtitle := range dbtitle {
							if checknzbtitle(dbtitle[idxtitle].Title, m.Title) {
								alttitlefound = true
								break
							}
						}
						if len(dbtitle) >= 1 && !alttitlefound {
							logger.Log.Debug("Skipped - unwanted title and alternate: ", m.Title, " wanted ", dbmovie.Title, " ", dbtitle)
							return
						} else if len(dbtitle) == 0 {
							logger.Log.Debug("Skipped - unwanted title: ", m.Title, " wanted ", dbmovie.Title, " ", dbtitle)
							return
						}
					}
				}
				StructureMovie(structure, folder, m, movie, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Debug("Movie not matched: ", videofiles[fileidx], " list ", list.Name)
			}
		} else if strings.ToLower(structure.groupType) == "series" {
			//SerieEpisodeHistory, _ := database.QuerySerieEpisodeHistory(database.Query{InnerJoin: "series on series.id=serie_episode_histories.serie_id", Where: "serie_episode_histories.target = ? and series.listname = ?", WhereArgs: []interface{}{filepath.Base(folders[idx]), list.Name}})

			yearstr := strconv.Itoa(m.Year)
			titleyear := m.Title + " (" + yearstr + ")"
			seriestitle := ""
			matched := config.RegexSeriesTitle.FindStringSubmatch(filepath.Base(videofiles[fileidx]))
			if len(matched) >= 2 {
				seriestitle = matched[1]
			}

			if !config.ConfigCheck("quality_" + list.Template_quality) {
				return
			}
			var cfg_quality config.QualityConfig
			config.ConfigGet("quality_"+list.Template_quality, &cfg_quality)
			for idxstrip := range cfg_quality.TitleStripSuffixForSearch {
				if strings.HasSuffix(strings.ToLower(m.Title), strings.ToLower(cfg_quality.TitleStripSuffixForSearch[idxstrip])) {
					m.Title = trimStringInclAfterStringInsensitive(m.Title, cfg_quality.TitleStripSuffixForSearch[idxstrip])
					m.Title = strings.Trim(m.Title, " ")
				}
			}
			for idxstrip := range cfg_quality.TitleStripPrefixForSearch {
				if strings.HasPrefix(strings.ToLower(m.Title), strings.ToLower(cfg_quality.TitleStripPrefixForSearch[idxstrip])) {
					m.Title = trimStringPrefixInsensitive(m.Title, cfg_quality.TitleStripPrefixForSearch[idxstrip])
					m.Title = strings.Trim(m.Title, " ")
				}
			}

			//find dbseries
			series, entriesfound := FindSerieByParser(*m, titleyear, seriestitle, list.Name)
			if entriesfound >= 1 {
				if cfg_quality.CheckTitle {
					titlefound := false
					dbseries, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
					if cfg_quality.CheckTitle && checknzbtitle(dbseries.Seriename, m.Title) && len(dbseries.Seriename) >= 1 {
						titlefound = true
					}
					if !titlefound {
						alttitlefound := false
						dbtitle, _ := database.QueryDbserieAlternates(database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{series.DbserieID}})

						for idxtitle := range dbtitle {
							if checknzbtitle(dbtitle[idxtitle].Title, m.Title) {
								alttitlefound = true
								break
							}
						}
						if len(dbtitle) >= 1 && !alttitlefound {
							logger.Log.Debug("Skipped - unwanted title and alternate: ", m.Title, " wanted ", dbseries.Seriename, " ", dbtitle)
							return
						} else if len(dbtitle) == 0 {
							logger.Log.Debug("Skipped - unwanted title: ", m.Title, " wanted ", dbseries.Seriename)
							return
						}
					}
				}
				StructureSeries(structure, folder, m, series, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Errorln("serie not matched", m, titleyear, seriestitle, list.Name)
			}
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
		StructureSingleFolder(folders[idx], false, false, false, grouptype, sourcepath, targetpath, configEntry, list)
	}
}

type forstructurenotify struct {
	Structure
	InputNotifier
}

func StructureSendNotify(event string, noticonfig config.MediaNotificationConfig, notifierdata forstructurenotify) {
	if !strings.EqualFold(noticonfig.Event, event) {
		return
	}
	//messagetext := noticonfig.Message
	tmplmessage, err := template.New("tmplfile").Parse(noticonfig.Message)
	if err != nil {
		logger.Log.Error(err)
	}
	var docmessage bytes.Buffer
	err = tmplmessage.Execute(&docmessage, notifierdata)
	if err != nil {
		logger.Log.Error(err)
	}
	messagetext := docmessage.String()

	tmpltitle, err := template.New("tmplfile").Parse(noticonfig.Title)
	if err != nil {
		logger.Log.Error(err)
	}
	var doctitle bytes.Buffer
	err = tmpltitle.Execute(&doctitle, notifierdata)
	if err != nil {
		logger.Log.Error(err)
	}
	MessageTitle := doctitle.String()

	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}
	var cfg_notification config.NotificationConfig
	config.ConfigGet("notification_"+noticonfig.Map_notification, &cfg_notification)

	if strings.EqualFold(cfg_notification.Type, "pushover") {
		if apiexternal.PushoverApi.ApiKey != cfg_notification.Apikey {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}

		err := apiexternal.PushoverApi.SendMessage(messagetext, MessageTitle, cfg_notification.Recipient)
		if err != nil {
			logger.Log.Error("Error sending pushover", err)
		} else {
			logger.Log.Info("Pushover message sent")
		}
	}
	if strings.EqualFold(cfg_notification.Type, "csv") {
		f, errf := os.OpenFile(cfg_notification.Outputto,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if errf != nil {
			logger.Log.Error("Error opening csv to write", errf)
			return
		}
		defer f.Close()
		if errf == nil {
			_, errc := io.WriteString(f, messagetext+"\n")
			if errc != nil {
				logger.Log.Error("Error writing to csv", errc)
			} else {
				logger.Log.Info("csv written")
			}
		}
	}
}
func (s *Structure) Notify(videotarget string, filename string, videofile string, m ParseInfo, movie database.Movie, serieepisode database.SerieEpisode, oldfiles []string) {
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
		for idx := range s.configEntry.Notification {
			StructureSendNotify("added_data", s.configEntry.Notification[idx], forstructurenotify{*s, InputNotifier{
				Targetpath:     filepath.Join(videotarget, filename),
				SourcePath:     videofile,
				Title:          dbmovie.Title,
				Year:           strconv.Itoa(dbmovie.Year),
				Imdb:           dbmovie.ImdbID,
				Replaced:       oldfiles,
				ReplacedPrefix: s.configEntry.Notification[idx].ReplacedPrefix,
				Configuration:  s.list.Name,
				Dbmovie:        dbmovie,
				Source:         m,
				Time:           time.Now().Format(time.RFC3339),
			}})
		}
	} else {
		dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepisode.DbserieEpisodeID}})
		dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{dbserieepisode.DbserieID}})
		for idx := range s.configEntry.Notification {
			StructureSendNotify("added_data", s.configEntry.Notification[idx], forstructurenotify{*s, InputNotifier{
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
				Dbserie:        dbserie,
				DbserieEpisode: dbserieepisode,
				Source:         m,
				Time:           time.Now().Format(time.RFC3339),
			}})
		}
	}
}

func (s *Structure) GetSeriesEpisodes(series database.Serie, videofile string, m ParseInfo, folder string) (oldfiles []string, episodes []int, allowimport bool, serietitle string, episodetitle string, SeriesEpisode database.SerieEpisode, SeriesEpisodes []database.SerieEpisode, runtime int) {
	dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
	teststr := config.RegexSeriesIdentifier.FindStringSubmatch(m.Identifier)
	if len(teststr) == 0 {
		logger.Log.Debug("In Identifier not found: ", videofile, " Identifier: ", m.Identifier)
		return
	}

	episodeArray := getEpisodeArray(dbserie.Identifiedby, teststr[1], teststr[2])

	var cfg_quality config.QualityConfig
	config.ConfigGet("quality_"+s.list.Template_quality, &cfg_quality)
	m.GetPriority(s.configEntry, cfg_quality)
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
			if SeriesEpisodeerr == nil {
				SeriesEpisodes = append(SeriesEpisodes, SeriesEpisode)
			}
		} else {
			SeriesEpisode, SeriesEpisodeerr = database.GetSerieEpisodes(database.Query{Select: "Serie_episodes.*", InnerJoin: "Dbserie_episodes ON Dbserie_episodes.ID = Serie_episodes.Dbserie_episode_id", Where: "Serie_episodes.serie_id = ? AND DbSerie_episodes.Season = ? AND DbSerie_episodes.Episode = ?", WhereArgs: []interface{}{series.ID, m.Season, epi}})
			if SeriesEpisodeerr == nil {
				SeriesEpisodes = append(SeriesEpisodes, SeriesEpisode)
			}
		}
		if SeriesEpisodeerr == nil {
			dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{SeriesEpisode.DbserieEpisodeID}})
			if runtime == 0 {
				runtime = dbserieepisode.Runtime
			}
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

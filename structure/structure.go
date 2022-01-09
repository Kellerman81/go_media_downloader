package structure

import (
	"bytes"
	"errors"
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
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
)

type structure struct {
	configTemplate string
	listConfig     string
	groupType      string //series, movies
	rootpath       string //1st level below input
	sourcepath     config.PathsConfig
	targetpath     config.PathsConfig
}

func NewStructure(configTemplate string, listConfig string, groupType string, rootpath string, sourcepath config.PathsConfig, targetpath config.PathsConfig) (structure, error) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if !configEntry.Structure {
		return structure{}, errors.New("not allowed")
	}
	return structure{
		configTemplate: configTemplate,
		listConfig:     listConfig,
		groupType:      groupType,
		rootpath:       rootpath,
		sourcepath:     sourcepath,
		targetpath:     targetpath,
	}, nil
}

func (s *structure) checkDisallowed() bool {
	if s.groupType == "series" {
		return scanner.CheckDisallowed(s.rootpath, s.sourcepath.Disallowed, false)
	} else {
		return scanner.CheckDisallowed(s.rootpath, s.sourcepath.Disallowed, s.sourcepath.DeleteDisallowed)
	}
}

func (s *structure) getVideoFiles(folder string, pathconfig config.PathsConfig, removesmallfiles bool) []string {
	videofiles := scanner.GetFilesDir(folder, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename, pathconfig.Blocked)

	if removesmallfiles {
		for idx := range videofiles {
			info, err := os.Stat(videofiles[idx])
			if pathconfig.MinVideoSize > 0 && err == nil {
				if info.Size() < int64(pathconfig.MinVideoSize*1024*1024) {
					scanner.RemoveFiles([]string{videofiles[idx]}, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename)
				}
			}
			info = nil
		}
		videofiles = scanner.GetFilesDir(folder, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename, pathconfig.Blocked)
	}
	return videofiles
}

func (s *structure) removeSmallVideoFile(file string, pathconfig config.PathsConfig) (removed bool) {
	info, err := os.Stat(file)
	if pathconfig.MinVideoSize > 0 && err == nil {
		if info.Size() < int64(pathconfig.MinVideoSize*1024*1024) {
			scanner.RemoveFiles([]string{file}, pathconfig.AllowedVideoExtensions, pathconfig.AllowedVideoExtensionsNoRename)
			removed = true
		}
	}
	info = nil
	return
}

//Parses - uses fprobe and checks language
func (s *structure) ParseFile(videofile string, checkfolder bool, folder string, deletewronglanguage bool) (m parser.ParseInfo, err error) {
	yearintitle := false
	if s.groupType == "series" {
		yearintitle = true
	}
	m, err = parser.NewFileParser(filepath.Base(videofile), yearintitle, s.groupType)
	if err != nil {
		logger.Log.Debug("Parse failed of ", filepath.Base(videofile))
		return
	}
	if m.Quality == "" && m.Resolution == "" && checkfolder {
		logger.Log.Debug("Parse of folder ", filepath.Base(folder), m)

		mf, errf := parser.NewFileParser(filepath.Base(folder), yearintitle, s.groupType)
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

	return
}

func (s *structure) fileCleanup(folder string, videofile string) {
	if strings.ToLower(s.groupType) == "movie" || videofile == "" {
		scanner.RemoveFiles(scanner.GetFilesDir(folder, s.sourcepath.AllowedVideoExtensions, s.sourcepath.AllowedVideoExtensionsNoRename, []string{}), []string{}, []string{})

		scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
	} else {
		fileext := filepath.Ext(videofile)
		err := scanner.RemoveFile(videofile)
		if err == nil {
			for idxext := range s.sourcepath.AllowedOtherExtensions {
				scanner.RemoveFile(strings.Replace(videofile, fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1))
			}
		}
		scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
	}
}
func (s *structure) ParseFileAdditional(videofile string, m parser.ParseInfo, folder string, deletewronglanguage bool, wantedruntime int) (parser.ParseInfo, error) {
	list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)
	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return m, errors.New("no quality")
	}

	m.GetPriority(s.configTemplate, list.Template_quality)
	var err error
	err = m.ParseVideoFile(videofile, s.configTemplate, list.Template_quality)
	if err != nil {
		return m, err
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
					s.fileCleanup(folder, videofile)
				}
				logger.Log.Error("Wrong runtime: Wanted", wantedruntime, " Having", intruntime, " difference", difference, " for", m.File)
				return m, errors.New("wrong runtime")
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
			s.fileCleanup(folder, videofile)
		}
		if !language_ok {
			logger.Log.Error("Wrong language: Wanted", s.targetpath.Allowed_languages, " Having", m.Languages, " for", m.File)
			err = errors.New("wrong Language")
		}
	}
	return m, err
}

func (s *structure) checkLowerQualTarget(folder string, videofile string, m parser.ParseInfo, cleanuplowerquality bool, movie database.Movie) ([]string, int, error) {
	list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)
	moviefiles, _ := database.QueryMovieFiles(database.Query{Select: "location, resolution_id, quality_id, codec_id, audio_id, proper, extended, repack", Where: "movie_id = ?", WhereArgs: []interface{}{movie.ID}})
	logger.Log.Debug("Found existing files: ", len(moviefiles))

	if !config.ConfigCheck("quality_" + list.Template_quality) {
		return []string{}, 0, errors.New("config not found")
	}

	oldpriority := parser.GetHighestMoviePriorityByFiles(movie, s.configTemplate, list.Template_quality)
	logger.Log.Debug("Found existing highest prio: ", oldpriority)
	if m.Priority > oldpriority {
		logger.Log.Debug("prio: ", oldpriority, " lower as ", m.Priority)
		//oldfiles := make([]string, 0, len(moviefiles)*3)
		oldfiles := []string{}
		if len(moviefiles) >= 1 {
			lastprocessed := ""
			for idx := range moviefiles {
				logger.Log.Debug("want to remove ", moviefiles[idx])
				oldpath, _ := filepath.Split(moviefiles[idx].Location)
				logger.Log.Debug("want to remove oldpath ", oldpath)
				entry_prio := parser.GetMovieDBPriority(moviefiles[idx], s.configTemplate, list.Template_quality)
				logger.Log.Debug("want to remove oldprio ", entry_prio)
				if m.Priority > entry_prio && s.targetpath.Upgrade {
					oldfiles = append(oldfiles, moviefiles[idx].Location)
					logger.Log.Debug("get all old files ", oldpath)
					if lastprocessed != oldpath {
						lastprocessed = oldpath

						oldfilesadd := scanner.GetFilesDir(oldpath, []string{}, []string{}, []string{})
						logger.Log.Debug("found old files ", len(oldfilesadd))
						for idxold := range oldfilesadd {
							if oldfilesadd[idxold] != moviefiles[idx].Location {
								oldfiles = append(oldfiles, oldfilesadd[idxold])
							}
						}
					}
				}
			}
		}
		return oldfiles, oldpriority, nil
	} else if len(moviefiles) >= 1 {
		logger.Log.Info("Skipped import due to lower quality: ", videofile)
		err := errors.New("import file has lower quality")
		if cleanuplowerquality {
			s.fileCleanup(folder, videofile)
		}
		return []string{}, oldpriority, err
	}
	return []string{}, oldpriority, nil
}

type parsertype struct {
	Dbmovie            database.Dbmovie
	Movie              database.Movie
	Serie              database.Serie
	Dbserie            database.Dbserie
	DbserieEpisode     database.DbserieEpisode
	Source             parser.ParseInfo
	TitleSource        string
	EpisodeTitleSource string
	Identifier         string
	Episodes           []int
}

func (s *structure) GenerateNamingTemplate(videofile string, m parser.ParseInfo, movie database.Movie,
	series database.Serie, serietitle string, serieepisode database.SerieEpisode, episodetitle string, episodes []int) (foldername string, filename string) {

	forparser := parsertype{}
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})

		movietitle := filepath.Base(videofile)
		movietitle = logger.TrimStringInclAfterString(movietitle, m.Quality)
		movietitle = logger.TrimStringInclAfterString(movietitle, m.Resolution)
		movietitle = logger.TrimStringInclAfterString(movietitle, strconv.Itoa(m.Year))
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

		configEntry := config.ConfigGet(s.configTemplate).Data.(config.MediaTypeConfig)
		naming := configEntry.Naming

		foldername, filename = path.Split(naming)
		if movie.Rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
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
		tmplfolder = nil
		logger.Log.Debug("Folder parsed: " + foldername)
		foldername = strings.Trim(foldername, ".")
		foldername = logger.StringReplaceDiacritics(foldername)
		foldername = logger.Path(foldername, true)

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
		tmplfile = nil
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
		filename = logger.StringReplaceDiacritics(filename)
		filename = logger.Path(filename, true)
	} else {
		dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})

		dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepisode.DbserieEpisodeID}})

		configEntry := config.ConfigGet(s.configTemplate).Data.(config.MediaTypeConfig)
		foldername, filename = path.Split(configEntry.Naming)

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
			foldername, _ = logger.Getrootpath(foldername)
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
		tmplfolder = nil
		logger.Log.Debug("Folder parsed: " + foldername)
		foldername = strings.Trim(foldername, ".")
		foldername = logger.StringReplaceDiacritics(foldername)
		foldername = logger.Path(foldername, true)
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
		tmplfile = nil
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
		filename = logger.StringReplaceDiacritics(filename)
		filename = logger.Path(filename, true)
	}
	return
}

func (s *structure) moveVideoFile(foldername string, filename string, videofiles []string, rootpath string) (videotarget string, moveok bool, moved int) {
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	videotarget = filepath.Join(s.targetpath.Path, foldername)
	logger.Log.Debug("Default target ", videotarget)
	if rootpath != "" {
		videotarget = filepath.Join(rootpath, foldername)
		logger.Log.Debug("Changed target ", videotarget)
	}

	os.MkdirAll(videotarget, os.FileMode(0777))

	moveok, moved = scanner.MoveFiles(videofiles, videotarget, filename, s.sourcepath.AllowedVideoExtensions, s.sourcepath.AllowedVideoExtensionsNoRename, cfg_general.UseFileBufferCopy)
	return
}

func (s *structure) updateRootpath(videotarget string, foldername string, movie database.Movie, serie database.Serie) {
	if s.targetpath.Usepresort {
		return
	}
	rootpath := videotarget

	folders := strings.Split(foldername, "/")
	if len(folders) >= 2 {
		rootpath = logger.TrimStringInclAfterString(rootpath, strings.TrimRight(folders[1], "/"))
		rootpath = strings.TrimRight(rootpath, "/")
	}
	if strings.ToLower(s.groupType) == "movie" && movie.Rootpath == "" {
		database.UpdateColumn("movies", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
	} else if strings.ToLower(s.groupType) == "series" && serie.Rootpath == "" {
		database.UpdateColumn("series", "rootpath", rootpath, database.Query{Where: "id=?", WhereArgs: []interface{}{serie.ID}})
	}
}

func (s *structure) moveOldFiles(oldfiles []string, movie database.Movie, series database.Serie) {
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if !s.targetpath.MoveReplaced || len(oldfiles) == 0 || s.targetpath.MoveReplacedTargetPath == "" {
		return
	}
	if strings.ToLower(s.groupType) == "movie" {
		logger.Log.Debug("want to remove old files")
		for idx := range oldfiles {
			fileext := filepath.Ext(oldfiles[idx])
			move_ok, _ := scanner.MoveFiles([]string{oldfiles[idx]}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{}, cfg_general.UseFileBufferCopy)
			if move_ok {
				logger.Log.Debug("Old File moved: ", oldfiles[idx])
				database.DeleteRow("movie_files", database.Query{Where: "movie_id = ? and location = ?", WhereArgs: []interface{}{movie.ID, oldfiles[idx]}})
				for idxext := range s.sourcepath.AllowedOtherExtensions {
					additionalfile := strings.Replace(oldfiles[idx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
					move_sub_ok, _ := scanner.MoveFiles([]string{additionalfile}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{}, cfg_general.UseFileBufferCopy)
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
			move_ok, _ := scanner.MoveFiles([]string{oldfiles[idx]}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{}, cfg_general.UseFileBufferCopy)
			if move_ok {
				logger.Log.Debug("Old File removed: ", oldfiles[idx])
				database.DeleteRow("serie_episode_files", database.Query{Where: "serie_id = ? and location = ?", WhereArgs: []interface{}{series.ID, oldfiles[idx]}})
				for idxext := range s.sourcepath.AllowedOtherExtensions {
					additionalfile := strings.Replace(oldfiles[idx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)
					move_sub_ok, _ := scanner.MoveFiles([]string{additionalfile}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[idx]))), "", []string{}, []string{}, cfg_general.UseFileBufferCopy)
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

func (s *structure) replaceLowerQualityFiles(oldfiles []string, movie database.Movie, series database.Serie) {
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

func (s *structure) moveAdditionalFiles(folder string, videotarget string, filename string, videofile string, sourcefileext string, videofilecount int) {
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.ToLower(s.groupType) == "movie" || videofilecount == 1 {
		additionalfiles := scanner.GetFilesDir(folder, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, s.sourcepath.Blocked)
		if len(additionalfiles) >= 1 {
			scanner.MoveFiles(additionalfiles, videotarget, filename, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, cfg_general.UseFileBufferCopy)
		}
	} else {
		for idx := range s.sourcepath.AllowedOtherExtensions {
			scanner.MoveFiles([]string{strings.Replace(videofile, sourcefileext, s.sourcepath.AllowedOtherExtensions[idx], -1)}, videotarget, filename, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, cfg_general.UseFileBufferCopy)
		}
	}
}

func (structurevar *structure) structureSeries(folder string, m parser.ParseInfo, series database.Serie, videofile string, deletewronglanguage bool) {

	dbseries, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
	runtime, _ := strconv.Atoi(dbseries.Runtime)
	oldfiles, episodes, allowimport, serietitle, episodetitle, seriesEpisode, seriesEpisodes, epiruntime := structurevar.GetSeriesEpisodes(series, videofile, m, folder)
	if epiruntime != 0 {
		runtime = epiruntime
	}
	var errpars error
	m, errpars = structurevar.ParseFileAdditional(videofile, m, folder, deletewronglanguage, runtime*len(episodes))
	if errpars != nil {
		logger.Log.Error("Error fprobe video: ", videofile, " error: ", errpars)
		return
	}
	if allowimport {
		foldername, filename := structurevar.GenerateNamingTemplate(videofile, m, database.Movie{}, series, serietitle, seriesEpisode, episodetitle, episodes)
		sourcefileext := filepath.Ext(videofile)
		structurevar.moveOldFiles(oldfiles, database.Movie{}, series)
		mediatargetpath := series.Rootpath
		if structurevar.targetpath.Usepresort && structurevar.targetpath.PresortFolderPath != "" {
			mediatargetpath = filepath.Join(structurevar.targetpath.PresortFolderPath, foldername)
		}
		videotarget, moveok, moved := structurevar.moveVideoFile(foldername, filename, []string{videofile}, mediatargetpath)
		if moveok && moved >= 1 {
			structurevar.updateRootpath(videotarget, foldername, database.Movie{}, series)
			var oldfiles_remove []string
			for idx := range oldfiles {
				if strings.HasPrefix(strings.ToLower(oldfiles[idx]), strings.ToLower(videotarget)) && strings.Contains(strings.ToLower(oldfiles[idx]), strings.ToLower(filename)) {
					//skip
				} else {
					oldfiles_remove = append(oldfiles_remove, oldfiles[idx])
				}
			}
			structurevar.replaceLowerQualityFiles(oldfiles_remove, database.Movie{}, series)
			structurevar.moveAdditionalFiles(folder, videotarget, filename, videofile, sourcefileext, len(videotarget))
			structurevar.notify(videotarget, filename, videofile, m, database.Movie{}, seriesEpisode, oldfiles)
			scanner.CleanUpFolder(folder, structurevar.sourcepath.CleanupsizeMB)

			//updateserie
			targetfile := filepath.Join(videotarget, filename)

			reached := false
			list := config.ConfigGetMediaListConfig(structurevar.configTemplate, structurevar.listConfig)
			if !config.ConfigCheck("quality_" + list.Template_quality) {
				logger.Log.Error("Template Quality not found: ", list.Template_quality)
				return
			}
			if m.Priority >= parser.NewCutoffPrio(structurevar.configTemplate, list.Template_quality).Priority {
				reached = true
			}
			for idxepi := range seriesEpisodes {
				database.InsertArray("serie_episode_files",
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id", "height", "width"},
					[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, seriesEpisodes[idxepi].SerieID, seriesEpisodes[idxepi].ID, seriesEpisodes[idxepi].DbserieEpisodeID, seriesEpisodes[idxepi].DbserieID, m.Height, m.Width})

				database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{seriesEpisodes[idxepi].ID}})
				database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{seriesEpisodes[idxepi].ID}})
			}
		}
	}
}
func (structurevar *structure) structureMovie(folder string, m parser.ParseInfo, movie database.Movie, videofile string, deletewronglanguage bool) {
	dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
	var errpars error
	m, errpars = structurevar.ParseFileAdditional(videofile, m, folder, deletewronglanguage, dbmovie.Runtime)
	if errpars != nil {
		logger.Log.Error("Error fprobe video: ", videofile, " error: ", errpars)
		return
	}
	oldfiles, _, errold := structurevar.checkLowerQualTarget(folder, videofile, m, true, movie)
	if errold != nil {
		logger.Log.Error("Error checking oldfiles: ", videofile, " error: ", errold)
		return
	}
	foldername, filename := structurevar.GenerateNamingTemplate(videofile, m, movie, database.Serie{}, "", database.SerieEpisode{}, "", []int{})

	sourcefileext := filepath.Ext(videofile)

	structurevar.moveOldFiles(oldfiles, movie, database.Serie{})
	mediatargetpath := movie.Rootpath
	if structurevar.targetpath.Usepresort && structurevar.targetpath.PresortFolderPath != "" {
		mediatargetpath = filepath.Join(structurevar.targetpath.PresortFolderPath, foldername)
	}
	videotarget, moveok, moved := structurevar.moveVideoFile(foldername, filename, []string{videofile}, mediatargetpath)
	if moveok && moved >= 1 {
		structurevar.updateRootpath(videotarget, foldername, movie, database.Serie{})
		var oldfiles_remove []string
		for idx := range oldfiles {
			if strings.HasPrefix(strings.ToLower(oldfiles[idx]), strings.ToLower(videotarget)) && strings.Contains(strings.ToLower(oldfiles[idx]), strings.ToLower(filename)) {
				//skip
			} else {
				oldfiles_remove = append(oldfiles_remove, oldfiles[idx])
			}
		}
		structurevar.replaceLowerQualityFiles(oldfiles_remove, movie, database.Serie{})
		structurevar.moveAdditionalFiles(folder, videotarget, filename, videofile, sourcefileext, len(videotarget))

		structurevar.notify(videotarget, filename, videofile, m, movie, database.SerieEpisode{}, oldfiles)
		scanner.CleanUpFolder(folder, structurevar.sourcepath.CleanupsizeMB)

		list := config.ConfigGetMediaListConfig(structurevar.configTemplate, structurevar.listConfig)
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			logger.Log.Error("Template Quality not found: ", list.Template_quality)
			return
		}
		//updatemovie
		targetfile := filepath.Join(videotarget, filename+filepath.Ext(videofile))
		database.InsertArray("movie_files",
			[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id", "height", "width"},
			[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), list.Template_quality, m.ResolutionID, m.QualityID, m.CodecID, m.AudioID, m.Proper, m.Repack, m.Extended, movie.ID, movie.DbmovieID, m.Height, m.Width})

		reached := false

		if m.Priority >= parser.NewCutoffPrio(structurevar.configTemplate, list.Template_quality).Priority {
			reached = true
		}
		database.UpdateColumn("movies", "missing", false, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
		database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id=?", WhereArgs: []interface{}{movie.ID}})
	} else {
		logger.Log.Error("Error moving video - unknown reason")
	}
}
func StructureSingleFolderAs(folder string, id int, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepath config.PathsConfig, targetpath config.PathsConfig, configTemplate string) {
	logger.Log.Debug("Process Folder: ", folder)
	if disableruntimecheck {
		targetpath.CheckRuntime = false
	}
	structurevar, err := NewStructure(configTemplate, "", grouptype, folder, sourcepath, targetpath)
	if err != nil {
		return
	}
	if disabledisallowed {
		structurevar.sourcepath.Disallowed = []string{}
	}
	if structurevar.checkDisallowed() {
		if structurevar.sourcepath.DeleteDisallowed {
			structurevar.fileCleanup(folder, "")
		}
		return
	}
	removesmallfiles := false
	if structurevar.groupType == "movie" {
		removesmallfiles = true
	}
	videofiles := structurevar.getVideoFiles(folder, sourcepath, removesmallfiles)

	if structurevar.groupType == "movie" {
		if len(videofiles) >= 2 {
			//skip too many  files
			return
		}
	}
	for fileidx := range videofiles {
		if filepath.Ext(videofiles[fileidx]) == "" {
			continue
		}
		if structurevar.groupType == "series" {
			if structurevar.removeSmallVideoFile(videofiles[fileidx], sourcepath) {
				continue
			}
		}

		if strings.Contains(strings.ToLower(videofiles[fileidx]), strings.ToLower("_unpack")) {
			logger.Log.Debug("Unpacking - skipping: ", videofiles[fileidx])
			continue
		}
		deletewronglanguage := structurevar.targetpath.DeleteWrongLanguage
		if disabledeletewronglanguage {
			deletewronglanguage = false
		}
		n, err := structurevar.ParseFile(videofiles[fileidx], true, folder, deletewronglanguage)
		if err != nil {
			logger.Log.Error("Error parsing: ", videofiles[fileidx], " error: ", err)
			continue
		}
		if structurevar.groupType == "movie" {
			movie, movierr := database.GetMovies(database.Query{Where: "id=?", WhereArgs: []interface{}{id}})
			if movierr != nil {
				return
			}

			structurevar.listConfig = movie.Listname
			if movie.ID >= 1 {
				structurevar.structureMovie(folder, n, movie, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Debug("Movie not matched: ", videofiles[fileidx])
			}
		} else if structurevar.groupType == "series" {
			//SerieEpisodeHistory, _ := database.QuerySerieEpisodeHistory(database.Query{InnerJoin: "series on series.id=serie_episode_histories.serie_id", Where: "serie_episode_histories.target = ? and series.listname = ?", WhereArgs: []interface{}{filepath.Base(folders[idx]), list.Name}})
			//find dbseries
			series, serierr := database.GetSeries(database.Query{Where: "id=?", WhereArgs: []interface{}{id}})
			if serierr != nil {
				logger.Log.Debug("Serie not matched: ", videofiles[fileidx])
				return
			}
			structurevar.listConfig = series.Listname
			structurevar.structureSeries(folder, n, series, videofiles[fileidx], deletewronglanguage)
		}
	}
}

func StructureSingleFolder(folder string, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepath config.PathsConfig, targetpath config.PathsConfig, configTemplate string) {
	logger.Log.Debug("Process Folder: ", folder)
	if disableruntimecheck {
		targetpath.CheckRuntime = false
	}
	structurevar, err := NewStructure(configTemplate, "", grouptype, folder, sourcepath, targetpath)
	if err != nil {
		return
	}
	if disabledisallowed {
		structurevar.sourcepath.Disallowed = []string{}
	}
	if structurevar.checkDisallowed() {
		if structurevar.sourcepath.DeleteDisallowed {
			structurevar.fileCleanup(folder, "")
		}
		return
	}
	removesmallfiles := false
	if structurevar.groupType == "movie" {
		removesmallfiles = true
	}
	videofiles := structurevar.getVideoFiles(folder, sourcepath, removesmallfiles)
	if len(videofiles) == 0 {
		logger.Log.Debug("Folder skipped due to no video files found ", folder)
		//skip too many  files
		return
	}
	if structurevar.groupType == "movie" {
		if len(videofiles) >= 2 {
			logger.Log.Debug("Folder skipped due to too many video files ", folder)
			//skip too many  files
			return
		}
	}
	//list := config.ConfigGetMediaListConfig(structurevar.configTemplate, structurevar.listConfig)
	for fileidx := range videofiles {
		var list config.MediaListsConfig
		if filepath.Ext(videofiles[fileidx]) == "" {
			continue
		}
		if structurevar.groupType == "series" {
			if structurevar.removeSmallVideoFile(videofiles[fileidx], sourcepath) {
				logger.Log.Debug("Folder skipped due to small video files - file was removed ", videofiles[fileidx])
				continue
			}
		}

		if strings.Contains(strings.ToLower(videofiles[fileidx]), strings.ToLower("_unpack")) {
			logger.Log.Debug("Unpacking - skipping: ", videofiles[fileidx])
			continue
		}
		deletewronglanguage := structurevar.targetpath.DeleteWrongLanguage
		if disabledeletewronglanguage {
			deletewronglanguage = false
		}
		n, err := structurevar.ParseFile(videofiles[fileidx], true, folder, deletewronglanguage)
		if err != nil {
			logger.Log.Error("Error parsing: ", videofiles[fileidx], " error: ", err)
			continue
		}
		if structurevar.groupType == "movie" {
			var entriesfound int
			var movie database.Movie

			//determine list
			if n.Imdb != "" {
				configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
				for idxlisttest := range configEntry.Lists {
					movies, movieserr := database.QueryMovies(database.Query{Select: "movies.*", Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname=?", WhereArgs: []interface{}{n.Imdb, configEntry.Lists[idxlisttest].Name}})
					if movieserr == nil {
						if len(movies) == 1 {
							movie = movies[0]
						}
						entriesfound = len(movies)

						if entriesfound >= 1 {
							list = configEntry.Lists[idxlisttest]
							structurevar.listConfig = list.Name

							break
						}
					}
				}
			}

			if entriesfound == 0 && list.Name != "" {
				id, err := database.QueryColumnStatic("Select dbmovie_id from movie_histories where title = ? and movie_id in (Select id from movies where listname = ?)", filepath.Base(folder), list.Name)
				if err == nil {
					movies, _ := database.QueryMovies(database.Query{Where: "Dbmovie_id = ? and listname = ?", WhereArgs: []interface{}{id, list.Name}})
					if len(movies) == 1 {
						logger.Log.Debug("Found Movie by history_title")
						movie = movies[0]
					}
					entriesfound = len(movies)

				}
			}
			if entriesfound == 0 && len(n.Imdb) == 0 {
				configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
				lists := make([]string, 0, len(configEntry.Lists))
				for idxlisttest := range configEntry.Lists {
					lists = append(lists, configEntry.Lists[idxlisttest].Name)
				}
				logger.Log.Debug("Find Movie using title: ", n.Title, " and year: ", n.Year, " and lists: ", lists)
				list.Name, n.Imdb, _, _ = importfeed.MovieFindListByTitle(n.Title, strconv.Itoa(n.Year), lists, "structure")
				if list.Name != "" {
					structurevar.listConfig = list.Name
					for idxlisttest := range configEntry.Lists {
						if configEntry.Lists[idxlisttest].Name == list.Name {
							list = configEntry.Lists[idxlisttest]
							break
						}
					}
				}
			}

			if !config.ConfigCheck("quality_" + list.Template_quality) {
				logger.Log.Error("Template Quality not found: ", list.Template_quality)
				return
			}
			cfg_quality := config.ConfigGet("quality_" + list.Template_quality).Data.(config.QualityConfig)
			n.StripTitlePrefixPostfix(list.Template_quality)

			if entriesfound == 0 && len(n.Imdb) >= 1 && list.Name != "" {
				movies, _ := database.QueryMovies(database.Query{Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname = ?", WhereArgs: []interface{}{n.Imdb, list.Name}})
				if len(movies) == 1 {
					logger.Log.Debug("Found Movie by title")
					movie = movies[0]
				}
				entriesfound = len(movies)

			}
			if entriesfound == 0 && len(n.Imdb) >= 1 {
				configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
				for idxlisttest := range configEntry.Lists {
					counter, _ := database.CountRowsStatic("Select count(id) from movies where dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname=?", n.Imdb, configEntry.Lists[idxlisttest].Name)
					//counter, _ := database.CountRows("movies", database.Query{Where: "dbmovie_id in (Select id from dbmovies where imdb_id = ?) and listname=?", WhereArgs: []interface{}{m.Imdb, configEntry.Lists[idxlisttest].Name}})
					if counter >= 1 {
						logger.Log.Debug("Skipped Movie by title")
						continue
					}
				}
			}
			if movie.ID >= 1 && entriesfound >= 1 {
				if cfg_quality.CheckTitle {
					titlefound := false
					dbmovie, _ := database.QueryStaticColumnsOneString("select title from dbmovies where id=?", "select count(id) from dbmovies where id=?", movie.DbmovieID)
					if len(dbmovie) == 0 {
						continue
					}
					//dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
					if cfg_quality.CheckTitle && parser.Checknzbtitle(dbmovie[0].Str, n.Title) && len(dbmovie[0].Str) >= 1 {
						titlefound = true
					}
					if !titlefound {
						alttitlefound := false
						dbtitle, _ := database.QueryStaticColumnsOneString("select title from dbmovie_titles where dbmovie_id=?", "select count(id) from dbmovie_titles where dbmovie_id=?", movie.DbmovieID)
						//dbtitle, _ := database.QueryDbmovieTitle(database.Query{Where: "dbmovie_id=?", WhereArgs: []interface{}{movie.DbmovieID}})

						for idxtitle := range dbtitle {
							if parser.Checknzbtitle(dbtitle[idxtitle].Str, n.Title) {
								alttitlefound = true
								break
							}
						}
						if len(dbtitle) >= 1 && !alttitlefound {
							logger.Log.Debug("Skipped - unwanted title and alternate: ", n.Title, " wanted ", dbmovie[0].Str, " ", dbtitle)
							return
						} else if len(dbtitle) == 0 {
							logger.Log.Debug("Skipped - unwanted title: ", n.Title, " wanted ", dbmovie[0].Str, " ", dbtitle)
							return
						}
					}
				}
				structurevar.structureMovie(folder, n, movie, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Debug("Movie not matched: ", videofiles[fileidx], " list ", list.Name)
			}

		} else if structurevar.groupType == "series" {
			//SerieEpisodeHistory, _ := database.QuerySerieEpisodeHistory(database.Query{InnerJoin: "series on series.id=serie_episode_histories.serie_id", Where: "serie_episode_histories.target = ? and series.listname = ?", WhereArgs: []interface{}{filepath.Base(folders[idx]), list.Name}})

			configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
			var series database.Serie
			var entriesfound int
			if n.Tvdb != "" {
				logger.Log.Debug("Find Serie by tvdb", n.Tvdb)
				founddbserie, founddbserieerr := database.GetDbserie(database.Query{Select: "id, seriename", Where: "thetvdb_id = ?", WhereArgs: []interface{}{n.Tvdb}})

				if founddbserieerr != nil {
					logger.Log.Debug("Skipped - Not Wanted DB Serie: ", n.Title)
					return
				}
				args := []interface{}{}
				args = append(args, founddbserie.ID)
				for idxlist := range configEntry.Lists {
					args = append(args, configEntry.Lists[idxlist].Name)
				}
				logger.Log.Debug("Find Serie by tvdb in lists ", n.Tvdb)
				foundserie, foundserieerr := database.GetSeries(database.Query{Where: "dbserie_id = ? and listname IN (?" + strings.Repeat(",?", len(configEntry.Lists)-1) + ")", WhereArgs: args})

				if foundserieerr != nil {
					logger.Log.Debug("Skipped - Not Wanted Serie: ", n.Title)
					return
				}
				series = foundserie
				entriesfound = 1
				for idxlist := range configEntry.Lists {
					if configEntry.Lists[idxlist].Name == series.Listname {
						list = configEntry.Lists[idxlist]
						structurevar.listConfig = series.Listname
						n.StripTitlePrefixPostfix(configEntry.Lists[idxlist].Template_quality)
						logger.Log.Debug("Found Serie by tvdb ", n.Tvdb, " in ", list.Name)
					}
				}
			}
			if entriesfound == 0 {
				yearstr := strconv.Itoa(n.Year)
				seriestitle := ""
				matched := config.RegexSeriesTitle.FindStringSubmatch(filepath.Base(videofiles[fileidx]))
				if len(matched) >= 2 {
					seriestitle = matched[1]
				}
				temptitle := n.Title
				for idxlisttest := range configEntry.Lists {
					n.Title = temptitle
					n.StripTitlePrefixPostfix(configEntry.Lists[idxlisttest].Template_quality)
					titleyear := n.Title
					if n.Year != 0 {
						titleyear += " (" + yearstr + ")"
					}
					var getseries database.Serie
					getseries, entriesfound = n.FindSerieByParser(titleyear, seriestitle, configEntry.Lists[idxlisttest].Name)
					if entriesfound >= 1 {
						list = configEntry.Lists[idxlisttest]
						structurevar.listConfig = list.Name
						series = getseries
						break
					}

				}
			}
			if series.ID == 0 {
				logger.Log.Info("Series not matched", n.Title)
				return
			}
			if !config.ConfigCheck("quality_" + list.Template_quality) {
				return
			}
			cfg_quality := config.ConfigGet("quality_" + list.Template_quality).Data.(config.QualityConfig)

			//find dbseries
			if entriesfound >= 1 {
				if cfg_quality.CheckTitle {
					titlefound := false
					logger.Log.Debug(series)
					dbseries, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
					logger.Log.Debug(dbseries)
					if cfg_quality.CheckTitle && parser.Checknzbtitle(dbseries.Seriename, n.Title) && len(dbseries.Seriename) >= 1 {
						titlefound = true
					}
					if !titlefound {
						alttitlefound := false
						dbtitle, _ := database.QueryStaticColumnsOneString("select title from dbserie_alternates where dbserie_id = ?", "select count(id) from dbserie_alternates where dbserie_id = ?", series.DbserieID)
						//dbtitle, _ := database.QueryDbserieAlternates(database.Query{Where: "dbserie_id=?", WhereArgs: []interface{}{series.DbserieID}})

						for idxtitle := range dbtitle {
							if parser.Checknzbtitle(dbtitle[idxtitle].Str, n.Title) {
								alttitlefound = true
								break
							}
						}
						if len(dbtitle) >= 1 && !alttitlefound {
							logger.Log.Debug("Skipped - unwanted title and alternate: ", n.Title, " wanted ", dbseries.Seriename, " ", dbtitle)
							continue
						} else if len(dbtitle) == 0 {
							logger.Log.Debug("Skipped - unwanted title: ", n.Title, " wanted ", dbseries.Seriename)
							continue
						}
					}
				}
				structurevar.structureSeries(folder, n, series, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Errorln("serie not matched", n, list.Name)
			}
		}
	}
}

var StructureJobRunning map[string]bool

func StructureFolders(grouptype string, sourcepath config.PathsConfig, targetpath config.PathsConfig, configTemplate string) {

	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	jobName := sourcepath.Path
	defer func() {
		database.ReadWriteMu.Lock()
		delete(StructureJobRunning, jobName)
		database.ReadWriteMu.Unlock()
	}()
	if !configEntry.Structure {
		logger.Log.Debug("Structure disabled: ", jobName)
		return
	}
	database.ReadWriteMu.Lock()
	if _, nok := StructureJobRunning[jobName]; nok {
		logger.Log.Debug("Job already running: ", jobName)
		database.ReadWriteMu.Unlock()
		return
	} else {
		StructureJobRunning[jobName] = true
		database.ReadWriteMu.Unlock()
	}

	logger.Log.Debug("Check Source: ", sourcepath.Path)
	folders := scanner.GetSubFolders(sourcepath.Path)
	for idx := range folders {
		StructureSingleFolder(folders[idx], false, false, false, grouptype, sourcepath, targetpath, configTemplate)
	}
}

type inputNotifier struct {
	Targetpath     string
	SourcePath     string
	Title          string
	Season         string
	Episode        string
	Identifier     string
	Series         string
	EpisodeTitle   string
	Tvdb           string
	Year           string
	Imdb           string
	Configuration  string
	Replaced       []string
	ReplacedPrefix string
	Dbmovie        database.Dbmovie
	Dbserie        database.Dbserie
	DbserieEpisode database.DbserieEpisode
	Source         parser.ParseInfo
	Time           string
}
type forstructurenotify struct {
	Structure     structure
	InputNotifier inputNotifier
}

func structureSendNotify(event string, noticonfig config.MediaNotificationConfig, notifierdata forstructurenotify) {
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
	tmplmessage = nil
	docmessage = bytes.Buffer{}

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
	tmpltitle = nil
	doctitle = bytes.Buffer{}

	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}
	cfg_notification := config.ConfigGet("notification_" + noticonfig.Map_notification).Data.(config.NotificationConfig)

	if strings.EqualFold(cfg_notification.Type, "pushover") {
		if apiexternal.PushoverApi == nil {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}
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
func (s *structure) notify(videotarget string, filename string, videofile string, m parser.ParseInfo, movie database.Movie, serieepisode database.SerieEpisode, oldfiles []string) {
	configEntry := config.ConfigGet(s.configTemplate).Data.(config.MediaTypeConfig)
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, _ := database.GetDbmovie(database.Query{Where: "id=?", WhereArgs: []interface{}{movie.DbmovieID}})
		for idx := range configEntry.Notification {
			structureSendNotify("added_data", configEntry.Notification[idx], forstructurenotify{Structure: *s, InputNotifier: inputNotifier{
				Targetpath:     filepath.Join(videotarget, filename),
				SourcePath:     videofile,
				Title:          dbmovie.Title,
				Year:           strconv.Itoa(dbmovie.Year),
				Imdb:           dbmovie.ImdbID,
				Replaced:       oldfiles,
				ReplacedPrefix: configEntry.Notification[idx].ReplacedPrefix,
				Configuration:  s.listConfig,
				Dbmovie:        dbmovie,
				Source:         m,
				Time:           time.Now().Format(time.RFC3339),
			}})
		}
	} else {
		dbserieepisode, _ := database.GetDbserieEpisodes(database.Query{Where: "id=?", WhereArgs: []interface{}{serieepisode.DbserieEpisodeID}})
		dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{dbserieepisode.DbserieID}})
		for idx := range configEntry.Notification {
			structureSendNotify("added_data", configEntry.Notification[idx], forstructurenotify{*s, inputNotifier{
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
				ReplacedPrefix: configEntry.Notification[idx].ReplacedPrefix,
				Configuration:  s.listConfig,
				Dbserie:        dbserie,
				DbserieEpisode: dbserieepisode,
				Source:         m,
				Time:           time.Now().Format(time.RFC3339),
			}})
		}
	}
}

func (s *structure) GetSeriesEpisodes(series database.Serie, videofile string, m parser.ParseInfo, folder string) (oldfiles []string, episodes []int, allowimport bool, serietitle string, episodetitle string, SeriesEpisode database.SerieEpisode, SeriesEpisodes []database.SerieEpisode, runtime int) {
	dbserie, _ := database.GetDbserie(database.Query{Where: "id=?", WhereArgs: []interface{}{series.DbserieID}})
	teststr := config.RegexSeriesIdentifier.FindStringSubmatch(m.Identifier)
	if len(teststr) == 0 {
		logger.Log.Debug("In Identifier not found: ", videofile, " Identifier: ", m.Identifier)
		return
	}

	episodeArray := importfeed.GetEpisodeArray(dbserie.Identifiedby, teststr[1], teststr[2])
	list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)

	m.GetPriority(s.configTemplate, list.Template_quality)
	identifiedby := strings.ToLower(dbserie.Identifiedby)
	for idx := range episodeArray {
		epi := episodeArray[idx]
		epi = strings.Trim(epi, "-EX")
		if identifiedby != "date" {
			epi = strings.TrimLeft(epi, "0")
		}
		epinum, _ := strconv.Atoi(epi)
		if epi == "" {
			continue
		}
		episodes = append(episodes, epinum)

		var SeriesEpisodeerr error
		if identifiedby == "date" {
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
				episodetitle = logger.TrimStringInclAfterString(episodetitle, "XXX")
				episodetitle = logger.TrimStringInclAfterString(episodetitle, m.Quality)
				episodetitle = logger.TrimStringInclAfterString(episodetitle, m.Resolution)
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

			list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)
			if !config.ConfigCheck("quality_" + list.Template_quality) {
				return
			}

			episodefiles, _ := database.QuerySerieEpisodeFiles(database.Query{Where: "serie_episode_id = ?", WhereArgs: []interface{}{SeriesEpisode.ID}})
			old_prio := parser.GetHighestEpisodePriorityByFiles(SeriesEpisode, s.configTemplate, list.Template_quality)
			if m.Priority > old_prio {
				allowimport = true
				for idx := range episodefiles {
					//_, oldfilename := filepath.Split(episodefile.Location)
					entry_prio := parser.GetSerieDBPriority(episodefiles[idx], s.configTemplate, list.Template_quality)
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

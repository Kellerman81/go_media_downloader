package structure

import (
	"bytes"
	"errors"
	"html/template"
	"io"
	"io/fs"
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
	n              parser.ParseInfo
}

func NewStructure(configTemplate string, listConfig string, groupType string, rootpath string, sourcepathstr string, targetpathstr string) (structure, error) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	if !configEntry.Structure {
		return structure{}, errors.New("not allowed")
	}
	sourcepath := config.ConfigGet(sourcepathstr).Data.(config.PathsConfig)
	targetpath := config.ConfigGet(targetpathstr).Data.(config.PathsConfig)

	return structure{
		configTemplate: configTemplate,
		listConfig:     listConfig,
		groupType:      groupType,
		rootpath:       rootpath,
		sourcepath:     sourcepath,
		targetpath:     targetpath,
	}, nil
}

func (s *structure) Close() {
	s = nil
}
func (s *structure) checkDisallowed() bool {
	if s.groupType == "series" {
		return s.checkDisallowedtype(s.rootpath, false)
	} else {
		return s.checkDisallowedtype(s.rootpath, s.sourcepath.DeleteDisallowed)
	}
}

func (s *structure) getVideoFiles(folder string, removesmallfiles bool) ([]string, error) {
	videofiles, err := scanner.GetFilesDir(folder, "path_"+s.sourcepath.Name)
	if err != nil {
		return nil, err
	}
	defer logger.ClearVar(&videofiles)
	if removesmallfiles {
		removed := false
		var info fs.FileInfo
		var err error
		for fileidx := range videofiles {
			info, err = os.Stat(videofiles[fileidx])
			if s.sourcepath.MinVideoSize > 0 && err == nil {
				if info.Size() < int64(s.sourcepath.MinVideoSize*1024*1024) {
					scanner.RemoveFiles(videofiles[fileidx], "path_"+s.sourcepath.Name)
					removed = true
				}
			}
		}
		if removed {
			videofiles, err = scanner.GetFilesDir(folder, "path_"+s.sourcepath.Name)
		}
	}
	return videofiles, nil
}

func (s *structure) removeSmallVideoFile(file string) (removed bool) {
	info, err := os.Stat(file)
	defer logger.ClearVar(&info)
	if s.sourcepath.MinVideoSize > 0 && err == nil {
		if info.Size() < int64(s.sourcepath.MinVideoSize*1024*1024) {
			scanner.RemoveFiles(file, "path_"+s.sourcepath.Name)
			removed = true
		}
	}
	return
}

//Parses - uses fprobe and checks language
func (s *structure) ParseFile(videofile string, checkfolder bool, folder string, deletewronglanguage bool) (err error) {
	yearintitle := false
	if s.groupType == "series" {
		yearintitle = true
	}
	s.n, err = parser.NewFileParser(filepath.Base(videofile), yearintitle, s.groupType)
	if err != nil {
		logger.Log.Errorln("Parse failed of ", filepath.Base(videofile))
		return
	}
	if s.n.Quality == "" && s.n.Resolution == "" && checkfolder {
		logger.Log.Debug("Parse of folder ", filepath.Base(folder))

		mf, errf := parser.NewFileParser(filepath.Base(folder), yearintitle, s.groupType)
		if errf != nil {
			logger.Log.Errorln("Parse failed of folder ", filepath.Base(folder))
			err = errf
			return
		} else {
			s.n.Quality = mf.Quality
			s.n.Resolution = mf.Resolution
			s.n.Title = mf.Title
			if s.n.Year == 0 {
				s.n.Year = mf.Year
			}
			if s.n.Identifier == "" {
				s.n.Identifier = mf.Identifier
			}
			if s.n.Audio == "" {
				s.n.Audio = mf.Audio
			}
			if s.n.Codec == "" {
				s.n.Codec = mf.Codec
			}
			if s.n.Imdb == "" {
				s.n.Imdb = mf.Imdb
			}
			mf.Close()
		}
	}

	return
}

func (s *structure) fileCleanup(folder string, videofile string) {
	if strings.ToLower(s.groupType) == "movie" || videofile == "" {
		pathcfg := s.sourcepath
		defer logger.ClearVar(&pathcfg)
		pathcfg.Blocked = []string{}
		toRemove, err := scanner.GetFilesDir(folder, "path_"+pathcfg.Name)
		defer logger.ClearVar(&toRemove)
		if err == nil {
			for idx := range toRemove {
				scanner.RemoveFiles(toRemove[idx], "")
			}
			scanner.CleanUpFolder(folder, s.sourcepath.CleanupsizeMB)
		}
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
func (s *structure) ParseFileAdditional(videofile string, folder string, deletewronglanguage bool, wantedruntime int) error {
	list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)
	if list.Name == "" {
		s.n.Close()
		return errors.New("no list")
	}
	if !config.ConfigCheck("quality_" + list.Template_quality) {
		s.n.Close()
		logger.Log.Error("Quality for List: " + list.Name + " not found")
		return errors.New("no quality")
	}

	s.n.GetPriority(s.configTemplate, list.Template_quality)
	var err error
	err = s.n.ParseVideoFile(videofile, s.configTemplate, list.Template_quality)
	if err != nil {
		s.n.Close()
		return err
	}
	if s.n.Runtime >= 1 && s.targetpath.CheckRuntime && wantedruntime != 0 && s.targetpath.MaxRuntimeDifference != 0 {
		intruntime := int(s.n.Runtime / 60)
		if intruntime != wantedruntime {
			maxdifference := s.targetpath.MaxRuntimeDifference
			if s.n.Extended && strings.ToLower(s.groupType) == "movie" {
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
				logger.Log.Error("Wrong runtime: Wanted", wantedruntime, " Having", intruntime, " difference", difference, " for", s.n.File)
				s.n.Close()
				return errors.New("wrong runtime")
			}
		}
	}
	if len(s.targetpath.Allowed_languages) >= 1 && deletewronglanguage {
		language_ok := false
		for idx := range s.targetpath.Allowed_languages {
			if len(s.n.Languages) == 0 && s.targetpath.Allowed_languages[idx] == "" {
				language_ok = true
				break
			}
			for langidx := range s.n.Languages {
				if strings.EqualFold(s.targetpath.Allowed_languages[idx], s.n.Languages[langidx]) {
					language_ok = true
					break
				}
			}
			if language_ok {
				break
			}
		}
		if !language_ok {
			s.fileCleanup(folder, videofile)
		}
		if !language_ok {
			logger.Log.Error("Wrong language: Wanted", s.targetpath.Allowed_languages, " Having", s.n.Languages, " for", s.n.File)
			s.n.Close()
			err = errors.New("wrong Language")
		}
	}
	return err
}

func (s *structure) checkLowerQualTarget(folder string, videofile string, cleanuplowerquality bool, movieid uint) ([]string, int, error) {
	list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)
	if list.Name == "" {
		return nil, 0, errors.New("list not found")
	}
	moviefiles, _ := database.QueryStaticColumnsOneStringOneInt("Select location, id from movie_files where movie_id = ?", "Select count(id) from movie_files where movie_id = ?", movieid)
	defer logger.ClearVar(&moviefiles)
	logger.Log.Debug("Found existing files: ", len(moviefiles))

	if !config.ConfigCheck("quality_" + list.Template_quality) {
		logger.Log.Error("Quality for List: " + list.Name + " not found")
		return nil, 0, errors.New("config not found")
	}

	oldpriority := parser.GetHighestMoviePriorityByFiles(movieid, s.configTemplate, list.Template_quality)
	logger.Log.Debug("Found existing highest prio: ", oldpriority)
	if s.n.Priority > oldpriority {
		logger.Log.Debug("prio: ", oldpriority, " lower as ", s.n.Priority)
		oldfiles := make([]string, 0, 10)
		defer logger.ClearVar(&oldfiles)
		if len(moviefiles) >= 1 {
			lastprocessed := ""
			var oldpath string
			var entry_prio int
			var oldfilesadd []string
			defer logger.ClearVar(&oldfilesadd)
			var err error
			for idx := range moviefiles {
				logger.Log.Debug("want to remove ", moviefiles[idx])
				oldpath, _ = filepath.Split(moviefiles[idx].Str)
				logger.Log.Debug("want to remove oldpath ", oldpath)
				entry_prio = parser.GetMovieDBPriorityById(uint(moviefiles[idx].Num), s.configTemplate, list.Template_quality)
				logger.Log.Debug("want to remove oldprio ", entry_prio)
				if s.n.Priority > entry_prio && s.targetpath.Upgrade {
					oldfiles = append(oldfiles, moviefiles[idx].Str)
					logger.Log.Debug("get all old files ", oldpath)
					if lastprocessed != oldpath {
						lastprocessed = oldpath

						oldfilesadd, err = scanner.GetFilesDirAll(oldpath)
						if err == nil {
							logger.Log.Debug("found old files ", len(oldfilesadd))
							for oldidx := range oldfilesadd {
								if oldfilesadd[oldidx] != moviefiles[idx].Str {
									oldfiles = append(oldfiles, oldfilesadd[oldidx])
								}
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
		return nil, oldpriority, err
	}
	return nil, oldpriority, nil
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

func (s *structure) GenerateNamingTemplate(videofile string, rootpath string, dbid uint, serietitle string, episodetitle string, episodes []int) (foldername string, filename string) {

	forparser := parsertype{}
	defer logger.ClearVar(&forparser)
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, err := database.GetDbmovie(database.Query{Where: "id = ?", WhereArgs: []interface{}{dbid}})
		if err != nil {
			return
		}
		movietitle := filepath.Base(videofile)
		movietitle = logger.TrimStringInclAfterString(movietitle, s.n.Quality)
		movietitle = logger.TrimStringInclAfterString(movietitle, s.n.Resolution)
		movietitle = logger.TrimStringInclAfterString(movietitle, strconv.Itoa(s.n.Year))
		movietitle = strings.Trim(movietitle, ".")
		movietitle = strings.Replace(movietitle, ".", " ", -1)
		forparser.TitleSource = movietitle
		logger.Log.Debug("trimmed title: ", movietitle)

		if dbmovie.Title == "" {
			dbmovie.Title, _ = database.QueryColumnString("Select title from dbmovie_titles where dbmovie_id = ?", dbid)
			if dbmovie.Title == "" {
				dbmovie.Title = movietitle
			}
		}
		if dbmovie.Year == 0 {
			dbmovie.Year = s.n.Year
		}

		configEntry := config.ConfigGet(s.configTemplate).Data.(config.MediaTypeConfig)
		naming := configEntry.Naming

		foldername, filename = path.Split(naming)
		if rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
		}

		forparser.Dbmovie = dbmovie
		if !strings.HasPrefix(s.n.Imdb, "tt") && len(s.n.Imdb) >= 1 {
			s.n.Imdb = "tt" + s.n.Imdb
		}
		if s.n.Imdb == "" {
			s.n.Imdb = dbmovie.ImdbID
		}
		forparser.Source = s.n

		logger.Log.Debug("Parse folder: " + foldername)
		tmplfolder, err := template.New("tmplfolder").Parse(foldername)
		defer logger.ClearVar(tmplfolder)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		var doc bytes.Buffer
		err = tmplfolder.Execute(&doc, forparser)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		foldername = doc.String()
		doc.Reset()
		logger.Log.Debug("Folder parsed: " + foldername)
		foldername = strings.Trim(foldername, ".")
		foldername = logger.StringReplaceDiacritics(foldername)
		foldername = logger.Path(foldername, true)

		logger.Log.Debug("Parse file: " + filename)
		tmplfile, err := template.New("tmplfile").Parse(filename)
		defer logger.ClearVar(tmplfile)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		var docfile bytes.Buffer
		err = tmplfile.Execute(&docfile, forparser)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		filename = docfile.String()
		docfile.Reset()
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
		epi, err := database.GetSerieEpisodes(database.Query{Select: "dbserie_id, dbserie_episode_id, serie_id", Where: "id = ?", WhereArgs: []interface{}{dbid}})
		series, err := database.GetSeries(database.Query{Where: "id = ?", WhereArgs: []interface{}{epi.SerieID}})
		dbserie, err := database.GetDbserie(database.Query{Where: "id = ?", WhereArgs: []interface{}{epi.DbserieID}})
		if err != nil {
			return
		}
		dbserieepisode, err := database.GetDbserieEpisodes(database.Query{Where: "id = ?", WhereArgs: []interface{}{epi.DbserieEpisodeID}})
		if err != nil {
			return
		}
		configEntry := config.ConfigGet(s.configTemplate).Data.(config.MediaTypeConfig)
		foldername, filename = path.Split(configEntry.Naming)

		if dbserie.Seriename == "" {
			dbseriealt, dbseriealterr := database.QueryColumnString("Select title from dbserie_alternates where dbserie_id = ?", epi.DbserieID)
			if dbseriealterr == nil {
				dbserie.Seriename = dbseriealt
			} else {
				dbserie.Seriename = serietitle
			}
		}
		if dbserieepisode.Title == "" {
			dbserieepisode.Title = episodetitle
		}
		if rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
		}

		forparser.Serie = series
		forparser.TitleSource = serietitle
		forparser.EpisodeTitleSource = episodetitle
		forparser.Dbserie = dbserie
		forparser.DbserieEpisode = dbserieepisode
		forparser.Episodes = episodes
		if s.n.Tvdb == "" {
			s.n.Tvdb = strconv.Itoa(dbserie.ThetvdbID)
		}
		if !strings.HasPrefix(s.n.Tvdb, "tvdb") && len(s.n.Tvdb) >= 1 {
			s.n.Tvdb = "tvdb" + s.n.Tvdb
		}
		forparser.Source = s.n

		logger.Log.Debug("Parse folder: " + foldername)
		tmplfolder, err := template.New("tmplfolder").Parse(foldername)
		defer logger.ClearVar(tmplfolder)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		var doc bytes.Buffer
		err = tmplfolder.Execute(&doc, forparser)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		foldername = doc.String()
		doc.Reset()
		logger.Log.Debug("Folder parsed: " + foldername)
		foldername = strings.Trim(foldername, ".")
		foldername = logger.StringReplaceDiacritics(foldername)
		foldername = logger.Path(foldername, true)
		//S{0Season}E{0Episode}(E{0Episode})

		logger.Log.Debug("Parse file: " + filename)
		tmplfile, err := template.New("tmplfile").Parse(filename)
		defer logger.ClearVar(tmplfile)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		var docfile bytes.Buffer
		err = tmplfile.Execute(&docfile, forparser)
		if err != nil {
			logger.Log.Error(err)
			return
		}
		filename = docfile.String()
		docfile.Reset()
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
	defer logger.ClearVar(&videofiles)
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

func (s *structure) updateRootpath(videotarget string, foldername string, mediarootpath string, id uint) {
	if s.targetpath.Usepresort {
		return
	}
	rootpath := videotarget

	folders := strings.Split(foldername, "/")
	defer logger.ClearVar(&folders)
	if len(folders) >= 2 {
		rootpath = logger.TrimStringInclAfterString(rootpath, strings.TrimRight(folders[1], "/"))
		rootpath = strings.TrimRight(rootpath, "/")
	}
	if strings.ToLower(s.groupType) == "movie" && mediarootpath == "" {
		database.UpdateColumn("movies", "rootpath", rootpath, database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
	} else if strings.ToLower(s.groupType) == "series" && mediarootpath == "" {
		database.UpdateColumn("series", "rootpath", rootpath, database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
	}
}

func (s *structure) moveRemoveOldMedia(oldfiles []string, id uint, usebuffer bool, move bool) {
	defer logger.ClearVar(&oldfiles)
	var fileext string
	var ok bool
	var err error
	var additionalfile string
	for oldidx := range oldfiles {
		fileext = filepath.Ext(oldfiles[oldidx])
		ok = false
		if move {
			ok, _ = scanner.MoveFiles([]string{oldfiles[oldidx]}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[oldidx]))), "", []string{}, []string{}, usebuffer)
		} else {
			err = scanner.RemoveFile(oldfiles[oldidx])
			if err == nil {
				ok = true
			}
		}
		if ok {
			logger.Log.Debug("Old File moved: ", oldfiles[oldidx])
			if strings.ToLower(s.groupType) == "movie" {
				database.DeleteRow("movie_files", database.Query{Where: "movie_id = ? and location = ?", WhereArgs: []interface{}{id, oldfiles[oldidx]}})
			} else {
				database.DeleteRow("serie_episode_files", database.Query{Where: "serie_id = ? and location = ?", WhereArgs: []interface{}{id, oldfiles[oldidx]}})
			}
			for idxext := range s.sourcepath.AllowedOtherExtensions {
				additionalfile = strings.Replace(oldfiles[oldidx], fileext, s.sourcepath.AllowedOtherExtensions[idxext], -1)

				if move {
					ok, _ = scanner.MoveFiles([]string{additionalfile}, filepath.Join(s.targetpath.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfiles[oldidx]))), "", []string{}, []string{}, usebuffer)
				} else {
					err = scanner.RemoveFile(additionalfile)
					if err == nil {
						ok = true
					}
				}
				if ok {
					logger.Log.Debug("Additional File removed: ", additionalfile)
				} else {
					logger.Log.Error("Additional File could not be removed: ", additionalfile)
				}
			}
		} else {
			logger.Log.Error("Old File could not be removed: ", oldfiles[oldidx])
		}
	}
}
func (s *structure) moveOldFiles(oldfiles []string, id uint) {
	defer logger.ClearVar(&oldfiles)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	if oldfiles == nil {
		return
	}
	if !s.targetpath.MoveReplaced || len(oldfiles) == 0 || s.targetpath.MoveReplacedTargetPath == "" {
		return
	}
	s.moveRemoveOldMedia(oldfiles, id, cfg_general.UseFileBufferCopy, true)
}

func (s *structure) replaceLowerQualityFiles(oldfiles []string, id uint) {
	defer logger.ClearVar(&oldfiles)
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)
	if oldfiles == nil {
		return
	}
	if !s.targetpath.Replacelower || len(oldfiles) == 0 {
		return
	}
	if strings.ToLower(s.groupType) == "movie" {
		s.moveRemoveOldMedia(oldfiles, id, cfg_general.UseFileBufferCopy, false)
	} else {
		s.moveRemoveOldMedia(oldfiles, id, cfg_general.UseFileBufferCopy, false)
	}
}

func (s *structure) moveAdditionalFiles(folder string, videotarget string, filename string, videofile string, sourcefileext string, videofilecount int) {
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if strings.ToLower(s.groupType) == "movie" || videofilecount == 1 {
		pathcfg := s.sourcepath
		pathcfg.AllowedVideoExtensions = pathcfg.AllowedOtherExtensions
		pathcfg.AllowedVideoExtensionsNoRename = pathcfg.AllowedOtherExtensionsNoRename
		additionalfiles, err := scanner.GetFilesDir(folder, "path_"+pathcfg.Name)
		defer logger.ClearVar(&additionalfiles)
		defer logger.ClearVar(&pathcfg)
		if err == nil {
			defer logger.ClearVar(&additionalfiles)
			if len(additionalfiles) >= 1 {
				scanner.MoveFiles(additionalfiles, videotarget, filename, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, cfg_general.UseFileBufferCopy)
			}
		}
	} else {
		for idx := range s.sourcepath.AllowedOtherExtensions {
			scanner.MoveFiles([]string{strings.Replace(videofile, sourcefileext, s.sourcepath.AllowedOtherExtensions[idx], -1)}, videotarget, filename, s.sourcepath.AllowedOtherExtensions, s.sourcepath.AllowedOtherExtensionsNoRename, cfg_general.UseFileBufferCopy)
		}
	}
}

func getOldFilesToRemove(oldfiles []string, videotarget string, filename string) ([]string, error) {
	defer logger.ClearVar(&oldfiles)
	var oldfiles_remove []string
	defer logger.ClearVar(&oldfiles_remove)
	for oldidx := range oldfiles {
		if strings.HasPrefix(strings.ToLower(oldfiles[oldidx]), strings.ToLower(videotarget)) && strings.Contains(strings.ToLower(oldfiles[oldidx]), strings.ToLower(filename)) {
			//skip
		} else {
			oldfiles_remove = append(oldfiles_remove, oldfiles[oldidx])
		}
	}
	if len(oldfiles_remove) == 0 {
		return nil, errors.New("no files")
	}
	return oldfiles_remove, nil
}

func (structurevar *structure) structureSeries(folder string, serieid uint, videofile string, deletewronglanguage bool) {
	//series, err := database.GetSeries(database.Query{Where: "id = ?", WhereArgs: []interface{}{serieid}})
	dbserieid, dberr := database.QueryColumnUint("select dbserie_id from series where id = ?", serieid)
	if dberr != nil {
		logger.Log.Error("Error no dbserieid")
		return
	}
	runtimestr, err := database.QueryColumnString("select runtime from dbseries where id = ?", dbserieid)
	if err != nil {
		logger.Log.Error("Error no runtime", err)
		return
	}
	runtime, _ := strconv.Atoi(runtimestr)
	rootpath, dberr := database.QueryColumnString("select rootpath from series where id = ?", serieid)
	if dberr != nil {
		logger.Log.Error("Error no rootpath")
		return
	}
	ignoreRuntime, dberr := database.QueryColumnBool("select ignore_runtime from series where id = ?", serieid)
	if dberr != nil {
		logger.Log.Error("Error no ignoreruntime")
		return
	}

	oldfiles, episodes, allowimport, serietitle, episodetitle, seriesEpisode, seriesEpisodes, epiruntime := structurevar.GetSeriesEpisodes(serieid, dbserieid, videofile, folder)
	defer logger.ClearVar(&oldfiles)
	defer logger.ClearVar(&episodes)
	defer logger.ClearVar(&seriesEpisodes)

	if epiruntime != 0 {
		runtime = epiruntime
	}
	if episodes == nil {
		logger.Log.Error("Error no episodes")
		return
	}
	if allowimport {
		season, err := database.QueryColumnString("select season from dbserie_episodes where id = ?", seriesEpisode.DbserieEpisodeID)
		if err != nil {
			logger.Log.Error("Error no season")
			return
		}
		if epiruntime == 0 && season == "0" {
			seriesEpisode.IgnoreRuntime = true
		}
		totalruntime := int(runtime) * len(episodes)
		if seriesEpisode.IgnoreRuntime {
			totalruntime = 0
		}
		if ignoreRuntime {
			totalruntime = 0
		}
		errpars := structurevar.ParseFileAdditional(videofile, folder, deletewronglanguage, totalruntime)
		if errpars != nil {
			logger.Log.Error("Error fprobe video: ", videofile, " error: ", errpars)
			return
		}
		foldername, filename := structurevar.GenerateNamingTemplate(videofile, rootpath, seriesEpisode.ID, serietitle, episodetitle, episodes)
		if foldername == "" || filename == "" {
			logger.Log.Error("Error generating foldername for: ", videofile)
			return
		}
		structurevar.moveOldFiles(oldfiles, serieid)
		mediatargetpath := rootpath
		if structurevar.targetpath.Usepresort && structurevar.targetpath.PresortFolderPath != "" {
			mediatargetpath = filepath.Join(structurevar.targetpath.PresortFolderPath, foldername)
		}
		videotarget, moveok, moved := structurevar.moveVideoFile(foldername, filename, []string{videofile}, mediatargetpath)
		if moveok && moved >= 1 {
			structurevar.updateRootpath(videotarget, foldername, rootpath, serieid)

			toRemove, err := getOldFilesToRemove(oldfiles, videotarget, filename)
			if err == nil {
				defer logger.ClearVar(&toRemove)
				structurevar.replaceLowerQualityFiles(toRemove, serieid)
			}
			structurevar.moveAdditionalFiles(folder, videotarget, filename, videofile, filepath.Ext(videofile), len(videotarget))
			structurevar.notify(videotarget, filename, videofile, seriesEpisode.DbserieEpisodeID, oldfiles)
			scanner.CleanUpFolder(folder, structurevar.sourcepath.CleanupsizeMB)

			//updateserie
			targetfile := filepath.Join(videotarget, filename+filepath.Ext(videofile))

			reached := false
			list := config.ConfigGetMediaListConfig(structurevar.configTemplate, structurevar.listConfig)
			if list.Name == "" {
				logger.Log.Error("Error no listname")
				return
			}
			if !config.ConfigCheck("quality_" + list.Template_quality) {
				logger.Log.Error("Quality for List: " + list.Name + " not found")
				return
			}
			if structurevar.n.Priority >= parser.NewCutoffPrio(structurevar.configTemplate, list.Template_quality).Priority {
				reached = true
			}
			for idx := range seriesEpisodes {
				database.InsertArray("serie_episode_files",
					[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "serie_id", "serie_episode_id", "dbserie_episode_id", "dbserie_id", "height", "width"},
					[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), list.Template_quality, structurevar.n.ResolutionID, structurevar.n.QualityID, structurevar.n.CodecID, structurevar.n.AudioID, structurevar.n.Proper, structurevar.n.Repack, structurevar.n.Extended, seriesEpisodes[idx].SerieID, seriesEpisodes[idx].ID, seriesEpisodes[idx].DbserieEpisodeID, seriesEpisodes[idx].DbserieID, structurevar.n.Height, structurevar.n.Width})

				database.UpdateColumn("serie_episodes", "missing", false, database.Query{Where: "id = ?", WhereArgs: []interface{}{seriesEpisodes[idx].ID}})
				database.UpdateColumn("serie_episodes", "quality_reached", reached, database.Query{Where: "id = ?", WhereArgs: []interface{}{seriesEpisodes[idx].ID}})
			}
		}
	}
}
func (structurevar *structure) structureMovie(folder string, movieid uint, videofile string, deletewronglanguage bool) {
	dbmovieid, dberr := database.QueryColumnUint("select dbmovie_id from movies where id = ?", movieid)
	if dberr != nil {
		return
	}
	runtime, dberr := database.QueryColumnUint("select runtime from dbmovies where id = ?", dbmovieid)
	if dberr != nil {
		return
	}
	rootpath, dberr := database.QueryColumnString("select rootpath from movies where id = ?", movieid)
	if dberr != nil {
		return
	}
	errpars := structurevar.ParseFileAdditional(videofile, folder, deletewronglanguage, int(runtime))
	if errpars != nil {
		logger.Log.Error("Error fprobe video: ", videofile, " error: ", errpars)
		return
	}
	oldfiles, _, errold := structurevar.checkLowerQualTarget(folder, videofile, true, movieid)
	if errold != nil {
		logger.Log.Error("Error checking oldfiles: ", videofile, " error: ", errold)
		return
	}
	defer logger.ClearVar(&oldfiles)
	foldername, filename := structurevar.GenerateNamingTemplate(videofile, rootpath, dbmovieid, "", "", []int{})
	if foldername == "" || filename == "" {
		logger.Log.Error("Error generating foldername for: ", videofile)
		return
	}
	sourcefileext := filepath.Ext(videofile)

	structurevar.moveOldFiles(oldfiles, movieid)
	mediatargetpath := rootpath
	if structurevar.targetpath.Usepresort && structurevar.targetpath.PresortFolderPath != "" {
		mediatargetpath = filepath.Join(structurevar.targetpath.PresortFolderPath, foldername)
	}
	videotarget, moveok, moved := structurevar.moveVideoFile(foldername, filename, []string{videofile}, mediatargetpath)
	if moveok && moved >= 1 {
		structurevar.updateRootpath(videotarget, foldername, rootpath, movieid)

		toRemove, err := getOldFilesToRemove(oldfiles, videotarget, filename)
		if err == nil {
			defer logger.ClearVar(&toRemove)
			structurevar.replaceLowerQualityFiles(toRemove, movieid)
		}
		structurevar.moveAdditionalFiles(folder, videotarget, filename, videofile, sourcefileext, len(videotarget))

		structurevar.notify(videotarget, filename, videofile, dbmovieid, oldfiles)
		scanner.CleanUpFolder(folder, structurevar.sourcepath.CleanupsizeMB)

		list := config.ConfigGetMediaListConfig(structurevar.configTemplate, structurevar.listConfig)
		if list.Name == "" {
			return
		}
		if !config.ConfigCheck("quality_" + list.Template_quality) {
			logger.Log.Error("Quality for List: " + list.Name + " not found")
			return
		}
		//updatemovie
		targetfile := filepath.Join(videotarget, filename+filepath.Ext(videofile))
		database.InsertArray("movie_files",
			[]string{"location", "filename", "extension", "quality_profile", "resolution_id", "quality_id", "codec_id", "audio_id", "proper", "repack", "extended", "movie_id", "dbmovie_id", "height", "width"},
			[]interface{}{targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), list.Template_quality, structurevar.n.ResolutionID, structurevar.n.QualityID, structurevar.n.CodecID, structurevar.n.AudioID, structurevar.n.Proper, structurevar.n.Repack, structurevar.n.Extended, movieid, dbmovieid, structurevar.n.Height, structurevar.n.Width})

		reached := false

		if structurevar.n.Priority >= parser.NewCutoffPrio(structurevar.configTemplate, list.Template_quality).Priority {
			reached = true
		}
		database.UpdateColumn("movies", "missing", false, database.Query{Where: "id = ?", WhereArgs: []interface{}{movieid}})
		database.UpdateColumn("movies", "quality_reached", reached, database.Query{Where: "id = ?", WhereArgs: []interface{}{movieid}})
	} else {
		logger.Log.Error("Error moving video - unknown reason")
	}
}

func (s *structure) checkDisallowedtype(rootpath string, removefolder bool) bool {
	if !config.ConfigCheck("general") {
		return true
	}

	if scanner.CheckFileExist(rootpath) {
		isok := true
		filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if info.IsDir() {
				return nil
			}
			for idx := range s.sourcepath.Disallowed {
				if s.sourcepath.Disallowed[idx] == "" {
					continue
				}
				if strings.Contains(strings.ToLower(path), strings.ToLower(s.sourcepath.Disallowed[idx])) {
					logger.Log.Warning(path, " is not allowd in the path!")
					isok = false
					return io.EOF
				}
			}
			return nil
		})
		if removefolder && !isok {
			scanner.CleanUpFolder(rootpath, 80000)
		}
		return isok
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return true
}
func StructureSingleFolderAs(folder string, id int, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepathstr string, targetpathstr string, configTemplate string) {
	logger.Log.Infoln("Process Folder: ", folder)
	structurevar, err := NewStructure(configTemplate, "", grouptype, folder, sourcepathstr, targetpathstr)
	if err != nil {
		return
	}
	defer structurevar.Close()
	if disableruntimecheck {
		structurevar.targetpath.CheckRuntime = false
	}
	if disabledisallowed {
		structurevar.sourcepath.Disallowed = []string{}
	}
	if !structurevar.checkDisallowed() {
		if structurevar.sourcepath.DeleteDisallowed {
			structurevar.fileCleanup(folder, "")
		}

		return
	}
	removesmallfiles := false
	if structurevar.groupType == "movie" {
		removesmallfiles = true
	}
	videofiles, err := structurevar.getVideoFiles(folder, removesmallfiles)
	if err != nil {
		return
	}
	defer logger.ClearVar(&videofiles)

	if structurevar.groupType == "movie" {
		if len(videofiles) >= 2 {
			//skip too many  files
			return
		}
	}
	var deletewronglanguage bool
	var movie database.Movie
	var series database.Serie
	for fileidx := range videofiles {
		if filepath.Ext(videofiles[fileidx]) == "" {
			continue
		}
		if structurevar.groupType == "series" {
			if structurevar.removeSmallVideoFile(videofiles[fileidx]) {
				continue
			}
		}

		if strings.Contains(strings.ToLower(videofiles[fileidx]), strings.ToLower("_unpack")) {
			logger.Log.Warningln("Unpacking - skipping: ", videofiles[fileidx])
			continue
		}
		deletewronglanguage = structurevar.targetpath.DeleteWrongLanguage
		if disabledeletewronglanguage {
			deletewronglanguage = false
		}
		err = structurevar.ParseFile(videofiles[fileidx], true, folder, deletewronglanguage)
		if err != nil {

			logger.Log.Error("Error parsing: ", videofiles[fileidx], " error: ", err)
			continue
		}
		if structurevar.groupType == "movie" {
			structurevar.listConfig = movie.Listname
			if movie.ID >= 1 {
				structurevar.structureMovie(folder, uint(id), videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Warningln("Movie not matched: ", videofiles[fileidx])
			}
		} else if structurevar.groupType == "series" {
			//find dbseries
			structurevar.listConfig = series.Listname
			structurevar.structureSeries(folder, uint(id), videofiles[fileidx], deletewronglanguage)
		}

	}

}

func StructureSingleFolder(folder string, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepathstr string, targetpathstr string, configTemplate string) {
	logger.Log.Infoln("Process Folder: ", folder)
	structurevar, err := NewStructure(configTemplate, "", grouptype, folder, sourcepathstr, targetpathstr)
	if err != nil {

		return
	}
	defer structurevar.Close()
	if disableruntimecheck {
		structurevar.targetpath.CheckRuntime = false
	}
	if disabledisallowed {
		structurevar.sourcepath.Disallowed = []string{}
	}
	if !structurevar.checkDisallowed() {
		if structurevar.sourcepath.DeleteDisallowed {
			structurevar.fileCleanup(folder, "")
		}

		return
	}

	removesmallfiles := false
	if structurevar.groupType == "movie" {
		removesmallfiles = true
	}
	videofiles, err := structurevar.getVideoFiles(folder, removesmallfiles)
	if err != nil {

		logger.Log.Debug("Folder skipped due to no video files found ", folder)
		//skip too many  files
		return
	}
	defer logger.ClearVar(&videofiles)

	if len(videofiles) == 0 {
		logger.Log.Debug("Folder skipped due to no video files found ", folder)
		//skip too many  files
		return
	}
	if structurevar.groupType == "movie" {
		if len(videofiles) >= 2 {
			logger.Log.Warningln("Folder skipped due to too many video files ", folder)
			//skip too many  files
			return
		}
	}

	deletewronglanguage := structurevar.targetpath.DeleteWrongLanguage
	if disabledeletewronglanguage {
		deletewronglanguage = false
	}
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)

	list := config.MediaListsConfig{}

	//var movie, getmovie database.Movie
	var movieid, dbmovieid uint
	//var dbmovie database.Dbmovie

	var serieid, dbserieid uint
	//var founddbserie database.Dbserie
	var yearstr, seriestitle, titleyear, seriename string
	var titlefound, alttitlefound, altitleexists bool
	var matched []string
	defer logger.ClearVar(&matched)
	var cfg_quality config.QualityConfig
	var foundmovies []database.Dbstatic_OneStringOneInt
	defer logger.ClearVar(&foundmovies)

	for fileidx := range videofiles {
		if filepath.Ext(videofiles[fileidx]) == "" {
			continue
		}
		if structurevar.groupType == "series" {
			if structurevar.removeSmallVideoFile(videofiles[fileidx]) {
				logger.Log.Debug("Folder skipped due to small video files - file was removed ", videofiles[fileidx])
				continue
			}
		}

		if strings.Contains(strings.ToLower(videofiles[fileidx]), strings.ToLower("_unpack")) {
			logger.Log.Warningln("Unpacking - skipping: ", videofiles[fileidx])
			continue
		}
		err = structurevar.ParseFile(videofiles[fileidx], true, folder, deletewronglanguage)
		if err != nil {
			logger.Log.Error("Error parsing: ", videofiles[fileidx], " error: ", err)
			continue
		}
		list = config.MediaListsConfig{}
		if structurevar.groupType == "movie" {
			dbmovieid = 0
			movieid = 0
			dbmovieid, _ = structurevar.n.FindDbmovieByFile()
			if dbmovieid == 0 {
				dbmovieid, _, _ = importfeed.MovieFindDbIdByTitle(structurevar.n.Title, strconv.Itoa(structurevar.n.Year), "movie", false)
			}
			if dbmovieid != 0 {
				foundmovies, _ = database.QueryStaticColumnsOneStringOneInt("Select listname, id from movies where dbmovie_id = ?", "Select count(id) from movies where dbmovie_id = ?", dbmovieid)
				logger.Log.Debug(foundmovies)
				for listtestidx := range configEntry.Lists {
					logger.Log.Debug("check dbmovieid ", dbmovieid, " in list ", configEntry.Lists[listtestidx].Name)
					for listfoundidx := range foundmovies {
						if configEntry.Lists[listtestidx].Name == foundmovies[listfoundidx].Str {
							list = configEntry.Lists[listtestidx]
							structurevar.listConfig = list.Name
							movieid = uint(foundmovies[listfoundidx].Num)
							break
						}
					}
					if movieid != 0 {
						break
					}
				}
				if movieid != 0 {
					if !config.ConfigCheck("quality_" + list.Template_quality) {

						logger.Log.Error("Quality for List: " + list.Name + " not found - for: " + videofiles[fileidx])
						continue
					}
					cfg_quality = config.ConfigGet("quality_" + list.Template_quality).Data.(config.QualityConfig)
					if cfg_quality.CheckTitle {
						titlefound = false
						titleyear, err = database.QueryColumnString("select title from dbmovies where id = ?", dbmovieid)
						if err != nil {

							continue
						}

						if cfg_quality.CheckTitle && parser.Checknzbtitle(titleyear, structurevar.n.Title) && len(titleyear) >= 1 {
							titlefound = true
						}
						if !titlefound {
							alttitlefound = false
							altitleexists = false

							for _, title := range database.QueryStaticStringArray("select title from dbmovie_titles where dbmovie_id = ?", "select count(id) from dbmovie_titles where dbmovie_id = ?", dbmovieid) {
								if title == "" {
									continue
								}
								altitleexists = true
								if parser.Checknzbtitle(title, structurevar.n.Title) {
									alttitlefound = true
									break
								}
							}
							if !alttitlefound {
								parser.StripTitlePrefixPostfix(&structurevar.n, list.Template_quality)
								if cfg_quality.CheckTitle && parser.Checknzbtitle(titleyear, structurevar.n.Title) && len(titleyear) >= 1 {
									titlefound = true
								}
								if !titlefound {
									for _, title := range database.QueryStaticStringArray("select title from dbmovie_titles where dbmovie_id = ?", "select count(id) from dbmovie_titles where dbmovie_id = ?", dbmovieid) {
										if title == "" {
											continue
										}
										altitleexists = true
										if parser.Checknzbtitle(title, structurevar.n.Title) {
											alttitlefound = true
											break
										}
									}
								}
							}
							if altitleexists && !alttitlefound {

								logger.Log.Warningln("Skipped - unwanted title and alternate: ", structurevar.n.Title, " wanted ", titleyear)
								continue
							} else if !altitleexists {

								logger.Log.Warningln("Skipped - unwanted title: ", structurevar.n.Title, " wanted ", titleyear)
								continue
							}
						}
					}
					structurevar.structureMovie(folder, movieid, videofiles[fileidx], deletewronglanguage)
				} else {
					logger.Log.Debug("Movie not matched: ", videofiles[fileidx], " list ", list.Name)
				}
			} else {
				logger.Log.Debug("DB Movie not matched: ", videofiles[fileidx])
			}
		} else if structurevar.groupType == "series" {
			serieid = 0
			dbserieid = 0
			if structurevar.n.Tvdb != "" {
				logger.Log.Debug("Find Serie by tvdb", structurevar.n.Tvdb)
				dbserieid, err = database.QueryColumnUint("select id from dbseries where thetvdb_id = ?", structurevar.n.Tvdb)
			}
			if dbserieid == 0 {
				yearstr = strconv.Itoa(structurevar.n.Year)
				seriestitle = ""
				matched = config.RegexGet("RegexSeriesTitle").FindStringSubmatch(filepath.Base(videofiles[fileidx]))
				if len(matched) >= 2 {
					seriestitle = matched[1]
				}
				if structurevar.n.Year != 0 {
					titleyear = seriestitle
					if titleyear == "" {
						titleyear = structurevar.n.Title
					}
					titleyear += " (" + yearstr + ")"
					dbserieid, _ = importfeed.FindDbserieByName(titleyear)
				}

				if dbserieid == 0 {
					dbserieid, _ = importfeed.FindDbserieByName(seriestitle)
				}
				if dbserieid == 0 {
					dbserieid, _ = importfeed.FindDbserieByName(structurevar.n.Title)
				}
			}
			if dbserieid != 0 {
				for idxlisttest := range configEntry.Lists {
					logger.Log.Debug("Check dbserieid against list: ", configEntry.Lists[idxlisttest].Name)
					serieid, err = database.QueryColumnUint("Select id from series where dbserie_id = ? AND listname = ? COLLATE NOCASE", dbserieid, configEntry.Lists[idxlisttest].Name)
					if serieid != 0 {
						list = configEntry.Lists[idxlisttest]
						structurevar.listConfig = list.Name
						break
					}
				}

				if serieid == 0 {
					logger.Log.Info("Series not matched: ", structurevar.n.Title, " dbserieid: ", dbserieid)
					continue
				}
				if !config.ConfigCheck("quality_" + list.Template_quality) {

					logger.Log.Error("Quality for List: " + list.Name + " not found - for: " + videofiles[fileidx])
					continue
				}
				cfg_quality = config.ConfigGet("quality_" + list.Template_quality).Data.(config.QualityConfig)
				if cfg_quality.CheckTitle {
					titlefound = false
					seriename, err = database.QueryColumnString("select seriename from dbseries where id = ?", dbserieid)
					if err != nil {
						logger.Log.Error("Seriename not found - for: " + videofiles[fileidx])

						continue
					}
					if cfg_quality.CheckTitle && parser.Checknzbtitle(seriename, structurevar.n.Title) && len(seriename) >= 1 {
						titlefound = true
					}
					if !titlefound {
						alttitlefound = false
						altitleexists = false

						for _, title := range database.QueryStaticStringArray("select title from dbserie_alternates where dbserie_id = ?", "select count(id) from dbserie_alternates where dbserie_id = ?", dbserieid) {
							if title == "" {
								continue
							}
							altitleexists = true
							if parser.Checknzbtitle(title, structurevar.n.Title) {
								alttitlefound = true
								break
							}
						}
						if altitleexists && !alttitlefound {

							logger.Log.Warningln("Skipped - unwanted title and alternate: ", structurevar.n.Title, " wanted ", seriename)
							continue
						} else if !altitleexists {

							logger.Log.Warningln("Skipped - unwanted title: ", structurevar.n.Title, " wanted ", seriename)
							continue
						}
					}
				}
				structurevar.structureSeries(folder, serieid, videofiles[fileidx], deletewronglanguage)
			} else {
				logger.Log.Errorln("serie not matched", structurevar.n, list.Name)
			}
		}
	}
}

var structureJobRunning logger.StringSet

//var structureJobRunning []string

func StructureFolders(grouptype string, sourcepathstr string, targetpathstr string, configTemplate string) {
	configEntry := config.ConfigGet(configTemplate).Data.(config.MediaTypeConfig)
	jobName := sourcepathstr
	if !configEntry.Structure {
		logger.Log.Debug("Structure disabled: ", jobName)
		return
	}
	if structureJobRunning.Contains(jobName) {
		logger.Log.Debug("Job already running: ", jobName)
		return
	} else {
		structureJobRunning.Add(jobName)
		defer structureJobRunning.Remove(jobName)
	}

	logger.Log.Debug("Check Source: ", sourcepathstr)
	sourcepath := config.ConfigGet(sourcepathstr).Data.(config.PathsConfig)
	folders, err := scanner.GetSubFolders(sourcepath.Path)
	defer logger.ClearVar(&folders)
	if err == nil {
		for idx := range folders {
			StructureSingleFolder(folders[idx], false, false, false, grouptype, sourcepathstr, targetpathstr, configTemplate)
		}
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
	tmplmessage, err := template.New("tmplfile").Parse(noticonfig.Message)
	defer logger.ClearVar(tmplmessage)
	if err != nil {
		logger.Log.Error(err)
	}
	var docmessage bytes.Buffer
	err = tmplmessage.Execute(&docmessage, notifierdata)
	if err != nil {
		logger.Log.Error(err)
	}
	messagetext := docmessage.String()
	docmessage = bytes.Buffer{}

	tmpltitle, err := template.New("tmplfile").Parse(noticonfig.Title)
	defer logger.ClearVar(tmpltitle)
	if err != nil {
		logger.Log.Error(err)
	}
	var doctitle bytes.Buffer
	err = tmpltitle.Execute(&doctitle, notifierdata)
	if err != nil {
		logger.Log.Error(err)
	}
	messageTitle := doctitle.String()
	doctitle = bytes.Buffer{}

	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}
	cfg_notification := config.ConfigGet("notification_" + noticonfig.Map_notification).Data.(config.NotificationConfig)

	if strings.EqualFold(cfg_notification.NotificationType, "pushover") {
		if apiexternal.PushoverApi == nil {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}
		if apiexternal.PushoverApi.ApiKey != cfg_notification.Apikey {
			apiexternal.NewPushOverClient(cfg_notification.Apikey)
		}

		err := apiexternal.PushoverApi.SendMessage(messagetext, messageTitle, cfg_notification.Recipient)
		if err != nil {
			logger.Log.Error("Error sending pushover", err)
		} else {
			logger.Log.Info("Pushover message sent")
		}
	}
	if strings.EqualFold(cfg_notification.NotificationType, "csv") {
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
func (s *structure) notify(videotarget string, filename string, videofile string, id uint, oldfiles []string) {
	defer logger.ClearVar(&oldfiles)
	configEntry := config.ConfigGet(s.configTemplate).Data.(config.MediaTypeConfig)
	if strings.ToLower(s.groupType) == "movie" {
		dbmovie, err := database.GetDbmovie(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
		if err != nil {
			return
		}
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
				Source:         s.n,
				Time:           time.Now().Format(time.RFC3339),
			}})
		}
	} else {
		dbserieepisode, err := database.GetDbserieEpisodes(database.Query{Where: "id = ?", WhereArgs: []interface{}{id}})
		if err != nil {
			return
		}
		dbserie, err := database.GetDbserie(database.Query{Where: "id = ?", WhereArgs: []interface{}{dbserieepisode.DbserieID}})
		if err != nil {
			return
		}
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
				Source:         s.n,
				Time:           time.Now().Format(time.RFC3339),
			}})
		}
	}
}

func (s *structure) GetSeriesEpisodes(serieid uint, dbserieid uint, videofile string, folder string) ([]string, []int, bool, string, string, database.SerieEpisode, []database.SerieEpisode, int) {
	identifiedby, dberr := database.QueryColumnString("Select identifiedby from dbseries where id = ?", dbserieid)
	if dberr != nil {
		logger.Log.Error("Error no identified")
		return nil, nil, false, "", "", database.SerieEpisode{}, nil, 0
	}

	episodeArray := importfeed.GetEpisodeArray(identifiedby, s.n.Identifier)
	defer logger.ClearVar(&episodeArray)
	list := config.ConfigGetMediaListConfig(s.configTemplate, s.listConfig)
	if list.Name == "" {
		logger.Log.Error("Error no list")
		return nil, nil, false, "", "", database.SerieEpisode{}, nil, 0
	}

	s.n.GetPriority(s.configTemplate, list.Template_quality)
	identifiedby = strings.ToLower(identifiedby)
	var seriesEpisodeerr, err error
	var runtime, epinum, old_prio, entry_prio int
	var dbserieepisodeid uint
	var allowimport bool
	var serietitle, episodetitle, videoext, additionalfile, loc string
	var oldfiles, matched []string
	defer logger.ClearVar(&oldfiles)
	defer logger.ClearVar(&matched)
	var episodes []int
	defer logger.ClearVar(&episodes)
	var seriesEpisode database.SerieEpisode
	var seriesEpisodes []database.SerieEpisode
	defer logger.ClearVar(&seriesEpisodes)
	var reepi *regexp.Regexp
	defer logger.ClearVar(reepi)
	var episodefiles []database.Dbstatic_OneInt
	//var episodefiles []database.SerieEpisodeFile
	defer logger.ClearVar(&episodefiles)
	if !config.ConfigCheck("quality_" + list.Template_quality) {
		logger.Log.Error("Quality for List: " + list.Name + " not found")
		return nil, nil, false, "", "", database.SerieEpisode{}, nil, 0
	}

	for idx, epi := range episodeArray {
		epi = strings.Trim(epi, "-EX")
		if identifiedby != "date" {
			epi = strings.TrimLeft(epi, "0")
			epinum, err = strconv.Atoi(epi)
			if epi == "" || err != nil {
				continue
			}
			episodes = append(episodes, epinum)
		} else {
			episodes = append(episodes, idx)
		}

		dbserieepisodeid, err = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid, s.n.Identifier, s.n.SeasonStr, epi)
		if dbserieepisodeid != 0 {
			seriesEpisode, seriesEpisodeerr = database.GetSerieEpisodes(database.Query{Where: "dbserie_episode_id = ? and serie_id = ?", WhereArgs: []interface{}{dbserieepisodeid, serieid}})
			if seriesEpisodeerr == nil {
				seriesEpisodes = append(seriesEpisodes, seriesEpisode)
			}
			getruntime, _ := database.QueryColumnUint("Select runtime from dbserie_episodes where id = ?", dbserieepisodeid)

			if runtime == 0 {
				runtime = int(getruntime)
			}

			matched = []string{}
			reepi, err = regexp.Compile(`^(.*)(?i)` + s.n.Identifier + `(?:\.| |-)(.*)$`)
			if err == nil {
				matched = reepi.FindStringSubmatch(filepath.Base(videofile))
			}
			if len(matched) >= 2 {
				logger.Log.Debug("matched title 1: ", matched[1])
				logger.Log.Debug("matched title 2: ", matched[2])
				episodetitle = matched[2]
				serietitle = matched[1]
				episodetitle = logger.TrimStringInclAfterString(episodetitle, "XXX")
				episodetitle = logger.TrimStringInclAfterString(episodetitle, s.n.Quality)
				episodetitle = logger.TrimStringInclAfterString(episodetitle, s.n.Resolution)
				episodetitle = strings.Trim(episodetitle, ".")
				episodetitle = strings.Replace(episodetitle, ".", " ", -1)

				serietitle = strings.Trim(serietitle, ".")
				serietitle = strings.Replace(serietitle, ".", " ", -1)
				logger.Log.Debug("trimmed title: ", episodetitle)
			}
			if len(episodetitle) == 0 {
				episodetitle, _ = database.QueryColumnString("Select title from dbserie_episodes where id = ?", dbserieepisodeid)
				logger.Log.Debug("use db title: ", episodetitle)
			}
			episodefiles, _ := database.QueryStaticColumnsOneInt("Select id from serie_episode_files where serie_episode_id = ?", "Select count(id) from serie_episode_files where serie_episode_id = ?", seriesEpisode.ID)
			old_prio = parser.GetHighestEpisodePriorityByFiles(seriesEpisode.ID, s.configTemplate, list.Template_quality)
			if s.n.Priority > old_prio {
				allowimport = true
				for idxfile := range episodefiles {
					//_, oldfilename := filepath.Split(episodefile.Location)
					entry_prio = parser.GetSerieDBPriorityById(uint(episodefiles[idxfile].Num), s.configTemplate, list.Template_quality)
					if s.n.Priority > entry_prio {
						loc, _ = database.QueryColumnString("Select location from serie_episode_files where id = ?", episodefiles[idxfile].Num)
						oldfiles = append(oldfiles, loc)
					}
				}
			} else if len(episodefiles) == 0 {
				allowimport = true
			} else {
				videoext = filepath.Ext(videofile)
				err = scanner.RemoveFile(videofile)
				if err == nil {
					logger.Log.Debug("Lower Qual Import File removed: ", videofile, " oldprio ", old_prio, " fileprio ", s.n.Priority)

					for idx := range s.sourcepath.AllowedOtherExtensions {
						additionalfile = strings.Replace(videofile, videoext, s.sourcepath.AllowedOtherExtensions[idx], -1)
						err = scanner.RemoveFile(additionalfile)
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
			logger.Log.Warningln("Import Not allowed - episode not matched - file: ", videofile, " - Season: ", s.n.Season, " - Episode: ", epi)
			continue
		}
	}
	return oldfiles, episodes, allowimport, serietitle, episodetitle, seriesEpisode, seriesEpisodes, runtime
}

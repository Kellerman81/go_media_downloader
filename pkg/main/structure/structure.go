package structure

import (
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
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
	"github.com/Kellerman81/go_media_downloader/searcher"
	"go.uber.org/zap"
)

type Structure struct {
	cfg        string
	listConfig string
	groupType  string //series, movies
	rootpath   string //1st level below input
	sourcepath string
	targetpath string
	N          *apiexternal.ParseInfo
}

var errNoQuality error = errors.New("quality not found")
var errNoList error = errors.New("list not found")
var errRuntime error = errors.New("wrong runtime")
var errLanguage error = errors.New("wrong language")
var errNotAllowed error = errors.New("not allowed")
var errLowerQuality error = errors.New("lower quality")

func NewStructure(cfg string, listname string, groupType string, rootpath string, sourcepathstr string, targetpathstr string) (*Structure, error) {
	if !config.Cfg.Media[cfg].Structure {
		return nil, errNotAllowed
	}
	return &Structure{
		cfg:        cfg,
		listConfig: listname,
		groupType:  groupType,
		rootpath:   rootpath,
		sourcepath: sourcepathstr,
		targetpath: targetpathstr,
	}, nil
}

func (s *Structure) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s != nil {
		s.N.Close()
		s = nil
	}
}
func (s *Structure) checkDisallowed() bool {
	check := config.Cfg.Paths[s.sourcepath].DeleteDisallowed
	if s.groupType == "series" {
		check = false
	}
	return s.checkDisallowedtype(s.rootpath, check)
}

func (s *Structure) getVideoFiles(folder string, removesmallfiles bool) (*logger.InStringArrayStruct, error) {
	videofiles, err := scanner.GetFilesDir(folder, config.Cfg.Paths[s.sourcepath].Name, false)
	if err != nil {
		return nil, err
	}
	if removesmallfiles && config.Cfg.Paths[s.sourcepath].MinVideoSize > 0 {
		removed := false
		wantedsize := int64(config.Cfg.Paths[s.sourcepath].MinVideoSize * 1024 * 1024)
		for idx := range videofiles.Arr {
			if scanner.GetFileSize(videofiles.Arr[idx]) < wantedsize {
				scanner.RemoveFiles(videofiles.Arr[idx], config.Cfg.Paths[s.sourcepath].Name)
				removed = true
			}
		}
		if removed {
			return scanner.GetFilesDir(folder, config.Cfg.Paths[s.sourcepath].Name, false)
		}
	}
	return videofiles, nil
}

func (s *Structure) filterVideoFiles(allfiles *logger.InStringArrayStruct, removesmallfiles bool) (*logger.InStringArrayStruct, error) {
	videofiles, err := scanner.FilterFilesDir(allfiles, config.Cfg.Paths[s.sourcepath].Name, false, false)
	if err != nil {
		return nil, err
	}
	if removesmallfiles && config.Cfg.Paths[s.sourcepath].MinVideoSize > 0 {
		removed := false
		videofilesremoved := videofiles.Arr[:0]
		wantedsize := int64(config.Cfg.Paths[s.sourcepath].MinVideoSize * 1024 * 1024)
		for idx := range videofiles.Arr {
			if scanner.GetFileSize(videofiles.Arr[idx]) < wantedsize {
				scanner.RemoveFiles(videofiles.Arr[idx], config.Cfg.Paths[s.sourcepath].Name)
				removed = true
			} else {
				videofilesremoved = append(videofilesremoved, videofiles.Arr[idx])
			}
		}
		if removed {
			videofiles.Arr = videofilesremoved
		}
	}
	return videofiles, nil
}

func (s *Structure) removeSmallVideoFile(file string) (removed bool) {
	if scanner.CheckFileExist(file) {
		if config.Cfg.Paths[s.sourcepath].MinVideoSize > 0 {
			if scanner.GetFileSize(file) < int64(config.Cfg.Paths[s.sourcepath].MinVideoSize*1024*1024) {
				scanner.RemoveFiles(file, config.Cfg.Paths[s.sourcepath].Name)
				removed = true
			}
		}
	}
	return
}

// Parses - uses fprobe and checks language
func (s *Structure) ParseFile(videofile string, checkfolder bool, folder string, deletewronglanguage bool) (err error) {
	yearintitle := false
	if s.groupType == "series" {
		yearintitle = true
	}
	s.N, err = parser.NewFileParser(filepath.Base(videofile), yearintitle, s.groupType)
	if err != nil {
		logger.Log.GlobalLogger.Error("Parse failed of ", zap.String("path", filepath.Base(videofile)))
		return
	}
	if s.N.Quality == "" && s.N.Resolution == "" && checkfolder {
		logger.Log.GlobalLogger.Debug("Parse of folder ", zap.String("path", filepath.Base(folder)))
		mf, errf := parser.NewFileParser(filepath.Base(folder), yearintitle, s.groupType)
		if errf != nil {
			logger.Log.GlobalLogger.Error("Parse failed of folder ", zap.String("path", filepath.Base(folder)))
			err = errf
			return
		} else {
			s.N.Quality = mf.Quality
			s.N.Resolution = mf.Resolution
			s.N.Title = mf.Title
			if s.N.Year == 0 {
				s.N.Year = mf.Year
			}
			if s.N.Identifier == "" {
				s.N.Identifier = mf.Identifier
			}
			if s.N.Audio == "" {
				s.N.Audio = mf.Audio
			}
			if s.N.Codec == "" {
				s.N.Codec = mf.Codec
			}
			if s.N.Imdb == "" {
				s.N.Imdb = mf.Imdb
			}
			mf.Close()
		}
	}

	return
}

func (s *Structure) fileCleanup(folder string, videofile string) {
	if strings.EqualFold(s.groupType, "movie") || videofile == "" {
		err := scanner.RemoveFile(videofile)
		if err == nil {
			toRemove, err := scanner.GetFilesDir(folder, config.Cfg.Paths[s.sourcepath].Name, false)
			if err == nil {
				for idx := range toRemove.Arr {
					scanner.RemoveFile(toRemove.Arr[idx])
				}
				toRemove.Close()
			}
		}
		scanner.CleanUpFolder(folder, config.Cfg.Paths[s.sourcepath].CleanupsizeMB)
	} else {
		fileext := filepath.Ext(videofile)
		err := scanner.RemoveFile(videofile)
		if err == nil {
			for idxext := range config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions {
				scanner.RemoveFile(strings.Replace(videofile, fileext, config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions[idxext], -1))
			}
		}
		scanner.CleanUpFolder(folder, config.Cfg.Paths[s.sourcepath].CleanupsizeMB)
	}
}
func (s *Structure) fileCleanupFilter(folder string, videofile string) {

	if videofile != "" {
		fileext := filepath.Ext(videofile)
		err := scanner.RemoveFile(videofile)
		if err == nil {
			for idxext := range config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions {
				scanner.RemoveFile(strings.Replace(videofile, fileext, config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions[idxext], -1))
			}
		}
	}
	scanner.CleanUpFolder(folder, config.Cfg.Paths[s.sourcepath].CleanupsizeMB)
}
func (s *Structure) ParseFileAdditional(videofile string, folder string, deletewronglanguage bool, wantedruntime int, checkruntime bool) error {
	if s.listConfig == "" {
		return errNoList
	}
	if !config.ConfigCheck("quality_" + config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality) {
		logger.Log.GlobalLogger.Error("Quality for List: " + s.listConfig + " not found")
		return errNoQuality
	}

	parser.GetPriorityMap(s.N, s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality, true)
	err := parser.ParseVideoFile(s.N, videofile, s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality)
	if err != nil {
		return err
	}
	if s.N.Runtime >= 1 && checkruntime && wantedruntime != 0 && config.Cfg.Paths[s.targetpath].MaxRuntimeDifference != 0 {
		if int(s.N.Runtime/60) != wantedruntime {
			maxdifference := config.Cfg.Paths[s.targetpath].MaxRuntimeDifference
			if s.N.Extended && strings.EqualFold(s.groupType, "movie") {
				maxdifference += 10
			}
			difference := 0
			if wantedruntime > int(s.N.Runtime/60) {
				difference = wantedruntime - int(s.N.Runtime/60)
			} else {
				difference = int(s.N.Runtime/60) - wantedruntime
			}
			if difference > maxdifference {
				if config.Cfg.Paths[s.targetpath].DeleteWrongRuntime {
					s.fileCleanup(folder, videofile)
				}
				logger.Log.GlobalLogger.Error("Wrong runtime: Wanted ", zap.Int("wanted", wantedruntime), zap.Int("Having", int(s.N.Runtime/60)), zap.Int("difference", difference), zap.String("path", s.N.File))
				return errRuntime
			}
		}
	}
	if len(config.Cfg.Paths[s.targetpath].Allowed_languages) >= 1 && deletewronglanguage {
		language_ok := false
		allowed := &logger.InStringArrayStruct{Arr: s.N.Languages}
		defer allowed.Close()
		for idx := range config.Cfg.Paths[s.targetpath].Allowed_languages {
			if len(s.N.Languages) == 0 && config.Cfg.Paths[s.targetpath].Allowed_languages[idx] == "" {
				language_ok = true
				break
			}
			if logger.InStringArray(config.Cfg.Paths[s.targetpath].Allowed_languages[idx], allowed) {
				language_ok = true
			}
			if language_ok {
				break
			}
		}
		if !language_ok {
			s.fileCleanup(folder, videofile)
		}
		if !language_ok {
			logger.Log.GlobalLogger.Error("Wrong language: Wanted ", zap.Strings("Allowed", config.Cfg.Paths[s.targetpath].Allowed_languages), zap.Strings("Have", s.N.Languages), zap.String("path", s.N.File))
			err = errLanguage
		}
	}
	return err
}

func (s *Structure) checkLowerQualTarget(folder string, videofile string, cleanuplowerquality bool, movieid uint) ([]string, int, error) {
	if s.listConfig == "" {
		return []string{}, 0, errNoList
	}

	if !config.ConfigCheck("quality_" + config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality) {
		logger.Log.GlobalLogger.Error("Quality for List: " + s.listConfig + " not found")
		return []string{}, 0, errNoQuality
	}
	moviefiles, _ := database.QueryStaticColumnsOneStringOneInt("select location as str, id as num from movie_files where movie_id = ?", false, 0, movieid)
	oldpriority := searcher.GetHighestMoviePriorityByFiles(movieid, s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality)
	logger.Log.GlobalLogger.Debug("Found existing highest prio", zap.Int("old", oldpriority))
	if s.N.Priority > oldpriority {
		if len(moviefiles) >= 1 {
			lastprocessed := ""
			var oldfiles []string = make([]string, 0, len(moviefiles)+1)
			var oldpath string
			var entry_prio int
			var oldfilesadd *logger.InStringArrayStruct
			defer oldfilesadd.Close()
			var err error
			for idx := range moviefiles {
				logger.Log.GlobalLogger.Debug("want to remove ", zap.String("path", moviefiles[idx].Str))
				oldpath, _ = filepath.Split(moviefiles[idx].Str)
				logger.Log.GlobalLogger.Debug("want to remove oldpath ", zap.String("path", oldpath))
				entry_prio = searcher.GetMovieDBPriorityById(uint(moviefiles[idx].Num), s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality)
				logger.Log.GlobalLogger.Debug("want to remove oldprio ", zap.Int("old", entry_prio))
				if s.N.Priority > entry_prio && config.Cfg.Paths[s.targetpath].Upgrade {
					oldfiles = append(oldfiles, moviefiles[idx].Str)
					logger.Log.GlobalLogger.Debug("get all old files ", zap.String("path", oldpath))
					if lastprocessed != oldpath {
						lastprocessed = oldpath
						oldfilesadd, err = scanner.GetFilesDirAll(oldpath)
						if err == nil {
							logger.Log.GlobalLogger.Debug("found old files ", zap.Int("files", len(oldfilesadd.Arr)))
							for oldidx := range oldfilesadd.Arr {
								if oldfilesadd.Arr[oldidx] != moviefiles[idx].Str {
									oldfiles = append(oldfiles, oldfilesadd.Arr[oldidx])
								}
							}
							oldfilesadd.Close()
						}
					}
				}
			}
			moviefiles = nil
			return oldfiles, oldpriority, nil
		}
	} else if len(moviefiles) >= 1 {
		logger.Log.GlobalLogger.Info("Skipped import due to lower quality", zap.String("path", videofile))
		if cleanuplowerquality {
			s.fileCleanup(folder, videofile)
		}
		moviefiles = nil
		return []string{}, oldpriority, errLowerQuality
	}
	return []string{}, oldpriority, nil
}

type parsertype struct {
	Dbmovie            database.Dbmovie
	Dbserie            database.Dbserie
	DbserieEpisode     database.DbserieEpisode
	Source             apiexternal.ParseInfo
	TitleSource        string
	EpisodeTitleSource string
	Identifier         string
	Episodes           []int
}

func (s *parsertype) Close() {
	if s != nil {
		s.Source.Close()
		s.Episodes = nil
		s = nil
	}
}

func (s *Structure) GenerateNamingTemplate(videofile string, rootpath string, dbid uint, serietitle string, episodetitle string, mapepi []database.Dbstatic_TwoInt) (foldername string, filename string) {
	forparser := new(parsertype)
	defer forparser.Close()
	if strings.EqualFold(s.groupType, "movie") {
		var err error
		forparser.Dbmovie, err = database.GetDbmovie(&database.Query{Where: "id = ?"}, dbid)
		if err != nil {
			return
		}
		movietitle := filepath.Base(videofile)
		movietitle = logger.TrimStringInclAfterString(movietitle, s.N.Quality)
		movietitle = logger.TrimStringInclAfterString(movietitle, s.N.Resolution)
		movietitle = logger.TrimStringInclAfterString(movietitle, strconv.Itoa(s.N.Year))
		movietitle = strings.Trim(movietitle, ".")
		movietitle = strings.Replace(movietitle, ".", " ", -1)
		forparser.TitleSource = movietitle
		forparser.TitleSource = strings.Replace(forparser.TitleSource, "/", "", -1)

		if forparser.Dbmovie.Title == "" {
			forparser.Dbmovie.Title, _ = database.QueryColumnString("select title from dbmovie_titles where dbmovie_id = ?", dbid)
			if forparser.Dbmovie.Title == "" {
				forparser.Dbmovie.Title = movietitle
			}
		}
		forparser.Dbmovie.Title = strings.Replace(forparser.Dbmovie.Title, "/", "", -1)
		if forparser.Dbmovie.Year == 0 {
			forparser.Dbmovie.Year = s.N.Year
		}

		foldername, filename = path.Split(config.Cfg.Media[s.cfg].Naming)
		if rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
		}

		if !strings.HasPrefix(s.N.Imdb, "tt") && len(s.N.Imdb) >= 1 {
			s.N.Imdb = "tt" + s.N.Imdb
		}
		if s.N.Imdb == "" {
			s.N.Imdb = forparser.Dbmovie.ImdbID
		}
		forparser.Source = *s.N

		forparser.Source.Title = strings.Replace(forparser.Source.Title, "/", "", -1)

		foldername, err = logger.ParseStringTemplate(foldername, forparser)
		if err != nil {
			return
		}
		filename, err = logger.ParseStringTemplate(filename, forparser)
		if err != nil {
			return
		}
		foldername = strings.Trim(foldername, ".")
		foldername = logger.StringReplaceDiacritics(foldername)
		foldername = logger.Path(foldername, true)

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
		epi, err := database.GetSerieEpisodes(&database.Query{Select: "dbserie_id, dbserie_episode_id, serie_id", Where: "id = ?"}, dbid)
		if err != nil {
			return
		}
		forparser.Dbserie, err = database.GetDbserie(&database.Query{Where: "id = ?"}, epi.DbserieID)
		if err != nil {
			return
		}
		forparser.DbserieEpisode, err = database.GetDbserieEpisodes(&database.Query{Where: "id = ?"}, epi.DbserieEpisodeID)
		if err != nil {
			return
		}
		foldername, filename = path.Split(config.Cfg.Media[s.cfg].Naming)

		if forparser.Dbserie.Seriename == "" {
			forparser.Dbserie.Seriename, err = database.QueryColumnString("select title from dbserie_alternates where dbserie_id = ?", epi.DbserieID)
			if err != nil {
				forparser.Dbserie.Seriename = serietitle
			}
		}
		if forparser.DbserieEpisode.Title == "" {
			forparser.DbserieEpisode.Title = episodetitle
		}
		if rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
		}

		var episodes []int = make([]int, len(mapepi))
		for key := range mapepi {
			epitext, _ := database.QueryColumnString("select episode from dbserie_episodes where id = ?", mapepi[key].Num2)
			epinum, err := strconv.Atoi(epitext)
			if err == nil {
				episodes[key] = epinum
			}
		}
		forparser.TitleSource = serietitle
		forparser.EpisodeTitleSource = episodetitle
		forparser.Episodes = episodes
		if s.N.Tvdb == "" {
			s.N.Tvdb = strconv.Itoa(forparser.Dbserie.ThetvdbID)
		}
		if !strings.HasPrefix(s.N.Tvdb, "tvdb") && len(s.N.Tvdb) >= 1 {
			s.N.Tvdb = "tvdb" + s.N.Tvdb
		}
		forparser.Source = *s.N

		foldername, err = logger.ParseStringTemplate(foldername, forparser)
		if err != nil {
			return
		}
		filename, err = logger.ParseStringTemplate(filename, forparser)
		if err != nil {
			return
		}
		foldername = strings.Trim(foldername, ".")
		foldername = logger.StringReplaceDiacritics(foldername)
		foldername = logger.Path(foldername, true)

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

func (s *Structure) moveVideoFile(foldername string, filename string, videofile string, rootpath string) (string, bool, int) {

	videotarget := filepath.Join(config.Cfg.Paths[s.targetpath].Path, foldername)
	if rootpath != "" {
		videotarget = filepath.Join(rootpath, foldername)
	}

	os.MkdirAll(videotarget, os.FileMode(0777))

	if scanner.MoveFile(videofile, videotarget, filename, &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].AllowedVideoExtensions}, &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].AllowedVideoExtensionsNoRename}, config.Cfg.General.UseFileBufferCopy) {
		return videotarget, true, 1
	}
	return videotarget, false, 0
}

func (s *Structure) updateRootpath(rootpath string, foldername string, mediarootpath string, id uint) {
	if config.Cfg.Paths[s.targetpath].Usepresort {
		return
	}

	folders := strings.Split(foldername, "/")
	if len(folders) >= 2 {
		rootpath = logger.TrimStringInclAfterString(rootpath, strings.TrimRight(folders[1], "/"))
		rootpath = strings.TrimRight(rootpath, "/")
	}
	folders = nil
	if strings.EqualFold(s.groupType, "movie") && mediarootpath == "" {
		database.UpdateColumnStatic("Update movies set rootpath = ? where id = ?", rootpath, id)
	} else if strings.EqualFold(s.groupType, "series") && mediarootpath == "" {
		database.UpdateColumnStatic("Update series set rootpath = ? where id = ?", rootpath, id)
	}
}

func (s *Structure) moveRemoveOldMediaFile(oldfile string, id uint, usebuffer bool, move bool) {

	fileext := filepath.Ext(oldfile)
	ok := false
	if move {
		if scanner.MoveFile(oldfile, filepath.Join(config.Cfg.Paths[s.targetpath].MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", &logger.InStringArrayStruct{Arr: []string{}}, &logger.InStringArrayStruct{Arr: []string{}}, usebuffer) {
			ok = true
		}
	} else {
		err := scanner.RemoveFile(oldfile)
		if err == nil {
			ok = true
		}
	}
	if ok {
		logger.Log.GlobalLogger.Debug("Old File moved", zap.String("path", oldfile))
		if strings.EqualFold(s.groupType, "movie") {
			database.DeleteRowStatic("Delete from movie_files where movie_id = ? and location = ?", id, oldfile)
		} else {
			database.DeleteRowStatic("Delete from serie_episode_files where serie_id = ? and location = ?", id, oldfile)
		}
		var additionalfile string
		var err error
		for idxext := range config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions {
			ok = false
			additionalfile = strings.Replace(oldfile, fileext, config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions[idxext], -1)
			if additionalfile == oldfile {
				continue
			}
			if move {
				ok = scanner.MoveFile(additionalfile, filepath.Join(config.Cfg.Paths[s.targetpath].MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", &logger.InStringArrayStruct{Arr: []string{}}, &logger.InStringArrayStruct{Arr: []string{}}, usebuffer)
			} else {
				err = scanner.RemoveFile(additionalfile)
				if err == nil {
					ok = true
				}
			}
			if ok {
				logger.Log.GlobalLogger.Debug("Additional File removed", zap.String("path", additionalfile))
			} else {
				logger.Log.GlobalLogger.Error("Additional File could not be removed", zap.String("path", additionalfile))
			}
		}
	} else {
		logger.Log.GlobalLogger.Error("Old File could not be removed", zap.String("path", oldfile))
	}
}

func (s *Structure) moveAdditionalFiles(folder string, videotarget string, filename string, videofile string, sourcefileext string, videofilecount int) {
	if strings.EqualFold(s.groupType, "movie") || videofilecount == 1 {
		additionalfiles, err := scanner.GetFilesDir(folder, config.Cfg.Paths[s.sourcepath].Name, true)
		if err == nil {
			if len(additionalfiles.Arr) >= 1 {
				for idx := range additionalfiles.Arr {
					scanner.MoveFile(additionalfiles.Arr[idx], videotarget, filename, &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions}, &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].AllowedOtherExtensionsNoRename}, config.Cfg.General.UseFileBufferCopy)
				}
			}
			additionalfiles.Close()
		}
	} else {
		for idx := range config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions {
			scanner.MoveFile(strings.Replace(videofile, sourcefileext, config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions[idx], -1), videotarget, filename, &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].AllowedVideoExtensions}, &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].AllowedVideoExtensionsNoRename}, config.Cfg.General.UseFileBufferCopy)
		}
	}
}

func (structurevar *Structure) structureSeries(folder string, serieid uint, videofile string, deletewronglanguage bool, checkruntime bool, checkdisallowed []string) {
	dbserieid, err := database.QueryColumnUint("select dbserie_id from series where id = ?", serieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error no dbserieid")
		return
	}
	runtimestr, err := database.QueryColumnString("select runtime from dbseries where id = ?", dbserieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error no runtime ", zap.Error(err))
		return
	}
	runtime, _ := strconv.Atoi(runtimestr)
	listname, _ := database.QueryColumnString("select listname from series where id = ?", serieid)
	rootpath, err := database.QueryColumnString("select rootpath from series where id = ?", serieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error no rootpath")
		return
	}
	structurevar.listConfig = listname

	oldfiles, allowimport, tblepi := structurevar.GetSeriesEpisodes(serieid, dbserieid, videofile, folder)

	if len(tblepi) == 0 {
		logger.Log.GlobalLogger.Error("Error no episodes")
		return
	}
	if allowimport {

		firstdbepiid := uint(tblepi[0].Num2)
		firstepiid := uint(tblepi[0].Num1)
		epiruntime, _ := database.QueryColumnUint("select runtime from dbserie_episodes where id = ?", firstdbepiid)

		if epiruntime != 0 {
			runtime = int(epiruntime)
		}

		season, err := database.QueryColumnString("select season from dbserie_episodes where id = ?", firstdbepiid)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error no season")
			return
		}
		ignoreRuntime, _ := database.QueryColumnBool("select ignore_runtime from serie_episodes where id = ?", firstepiid)
		if runtime == 0 && season == "0" {
			ignoreRuntime = true
		}
		totalruntime := int(runtime) * len(tblepi)
		if ignoreRuntime {
			totalruntime = 0
		}
		if ignoreRuntime {
			totalruntime = 0
		}

		err = structurevar.ParseFileAdditional(videofile, folder, deletewronglanguage, totalruntime, checkruntime)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error fprobe video", zap.String("path", videofile), zap.Error(err))
			structurevar.Close()
			return
		}

		serietitle, episodetitle := structurevar.GetEpisodeTitle(firstdbepiid, videofile)

		foldername, filename := structurevar.GenerateNamingTemplate(videofile, rootpath, firstepiid, serietitle, episodetitle, tblepi)
		if foldername == "" || filename == "" {
			logger.Log.GlobalLogger.Error("Error generating foldername for", zap.String("path", videofile))
			return
		}

		if !config.Cfg.Paths[structurevar.targetpath].MoveReplaced || len(oldfiles) == 0 || config.Cfg.Paths[structurevar.targetpath].MoveReplacedTargetPath == "" {
		} else {
			for idxold := range oldfiles {
				structurevar.moveRemoveOldMediaFile(oldfiles[idxold], serieid, config.Cfg.General.UseFileBufferCopy, true)
			}
		}

		if config.Cfg.Paths[structurevar.targetpath].Usepresort && config.Cfg.Paths[structurevar.targetpath].PresortFolderPath != "" {
			rootpath = filepath.Join(config.Cfg.Paths[structurevar.targetpath].PresortFolderPath, foldername)
		}
		videotarget, moveok, moved := structurevar.moveVideoFile(foldername, filename, videofile, rootpath)
		if moveok && moved >= 1 {
			structurevar.updateRootpath(videotarget, foldername, rootpath, serieid)

			if !config.Cfg.Paths[structurevar.targetpath].Replacelower || len(oldfiles) == 0 {
			} else {
				for oldidx := range oldfiles {
					_, oldfilename := filepath.Split(oldfiles[oldidx])
					if strings.HasPrefix(oldfiles[oldidx], videotarget) && strings.EqualFold(oldfilename, filename) {
						//skip
					} else {
						structurevar.moveRemoveOldMediaFile(oldfiles[oldidx], serieid, config.Cfg.General.UseFileBufferCopy, false)
					}
				}
			}

			structurevar.moveAdditionalFiles(folder, videotarget, filename, videofile, filepath.Ext(videofile), len(videotarget))
			structurevar.notify(videotarget, filename, videofile, firstdbepiid, listname, &oldfiles)
			scanner.CleanUpFolder(folder, config.Cfg.Paths[structurevar.sourcepath].CleanupsizeMB)

			//updateserie

			reached := false

			if listname == "" {
				logger.Log.GlobalLogger.Error("Error no listname")
				return
			}
			if !config.ConfigCheck("quality_" + config.Cfg.Media[structurevar.cfg].ListsMap[structurevar.listConfig].Template_quality) {
				logger.Log.GlobalLogger.Error("Quality for List: " + listname + " not found")
				return
			}
			if structurevar.N.Priority >= parser.NewCutoffPrio(structurevar.cfg, config.Cfg.Media[structurevar.cfg].ListsMap[structurevar.listConfig].Template_quality) {
				reached = true
			}
			targetfile := filepath.Join(videotarget, filename+filepath.Ext(videofile))
			filebase := filepath.Base(targetfile)
			fileext := filepath.Ext(targetfile)

			for key := range tblepi {
				database.InsertNamed("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :serie_id, :serie_episode_id, :dbserie_episode_id, :dbserie_id, :height, :width)",
					database.SerieEpisodeFile{
						Location:         targetfile,
						Filename:         filebase,
						Extension:        fileext,
						QualityProfile:   config.Cfg.Media[structurevar.cfg].ListsMap[structurevar.listConfig].Template_quality,
						ResolutionID:     structurevar.N.ResolutionID,
						QualityID:        structurevar.N.QualityID,
						CodecID:          structurevar.N.CodecID,
						AudioID:          structurevar.N.AudioID,
						Proper:           structurevar.N.Proper,
						Repack:           structurevar.N.Repack,
						Extended:         structurevar.N.Extended,
						SerieID:          structurevar.N.SerieID,
						SerieEpisodeID:   uint(tblepi[key].Num1),
						DbserieEpisodeID: uint(tblepi[key].Num2),
						DbserieID:        structurevar.N.DbserieID,
						Height:           structurevar.N.Height,
						Width:            structurevar.N.Width})

				database.UpdateNamed("Update serie_episodes SET missing = :missing, quality_reached = :quality_reached where id = :id", database.SerieEpisode{ID: uint(tblepi[key].Num1), Missing: false, QualityReached: reached})

			}
		}
	} else {
		logger.Log.GlobalLogger.Warn("Import not allowed ", zap.String("path", folder))
	}
	oldfiles = nil
}
func (structurevar *Structure) structureMovie(folder string, movieid uint, videofile string, deletewronglanguage bool, checkruntime bool, checkdisallowed []string) {
	dbmovieid, err := database.QueryColumnUint("select dbmovie_id from movies where id = ?", movieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Structure failed no dbmovieid ", zap.String("path", folder))
		return
	}
	runtime, err := database.QueryColumnUint("select runtime from dbmovies where id = ?", dbmovieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Structure failed no runtime ", zap.String("path", folder))
		return
	}
	listname, _ := database.QueryColumnString("select listname from movies where id = ?", movieid)
	rootpath, err := database.QueryColumnString("select rootpath from movies where id = ?", movieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Structure failed no rootpath ", zap.String("path", folder))
		return
	}
	structurevar.listConfig = listname
	err = structurevar.ParseFileAdditional(videofile, folder, deletewronglanguage, int(runtime), checkruntime)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error fprobe video", zap.String("path", videofile), zap.Error(err))
		structurevar.Close()
		return
	}
	oldfiles, _, err := structurevar.checkLowerQualTarget(folder, videofile, true, movieid)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error checking oldfiles", zap.String("path", videofile), zap.Error(err))
		return
	}
	foldername, filename := structurevar.GenerateNamingTemplate(videofile, rootpath, dbmovieid, "", "", []database.Dbstatic_TwoInt{})
	if foldername == "" || filename == "" {
		logger.Log.GlobalLogger.Error("Error generating foldername for", zap.String("path", videofile))
		return
	}

	if !config.Cfg.Paths[structurevar.targetpath].MoveReplaced || len(oldfiles) == 0 || config.Cfg.Paths[structurevar.targetpath].MoveReplacedTargetPath == "" {
	} else {
		for idxold := range oldfiles {
			structurevar.moveRemoveOldMediaFile(oldfiles[idxold], movieid, config.Cfg.General.UseFileBufferCopy, true)
		}
	}
	if config.Cfg.Paths[structurevar.targetpath].Usepresort && config.Cfg.Paths[structurevar.targetpath].PresortFolderPath != "" {
		rootpath = filepath.Join(config.Cfg.Paths[structurevar.targetpath].PresortFolderPath, foldername)
	}
	videotarget, moveok, moved := structurevar.moveVideoFile(foldername, filename, videofile, rootpath)
	if moveok && moved >= 1 {
		structurevar.updateRootpath(videotarget, foldername, rootpath, movieid)

		if !config.Cfg.Paths[structurevar.targetpath].Replacelower || len(oldfiles) == 0 {
		} else {
			for oldidx := range oldfiles {
				_, oldfilename := filepath.Split(oldfiles[oldidx])
				if strings.HasPrefix(oldfiles[oldidx], videotarget) && strings.EqualFold(oldfilename, filename) {
					//skip
				} else {
					structurevar.moveRemoveOldMediaFile(oldfiles[oldidx], movieid, config.Cfg.General.UseFileBufferCopy, false)
				}
			}
		}
		structurevar.moveAdditionalFiles(folder, videotarget, filename, videofile, filepath.Ext(videofile), len(videotarget))

		structurevar.notify(videotarget, filename, videofile, dbmovieid, listname, &oldfiles)
		scanner.CleanUpFolder(folder, config.Cfg.Paths[structurevar.sourcepath].CleanupsizeMB)

		if listname == "" {
			logger.Log.GlobalLogger.Error("Structure failed no list ", zap.String("path", folder))
			return
		}
		if !config.ConfigCheck("quality_" + config.Cfg.Media[structurevar.cfg].ListsMap[structurevar.listConfig].Template_quality) {
			logger.Log.GlobalLogger.Error("Quality for List: " + listname + " not found")
			return
		}
		//updatemovie
		targetfile := filepath.Join(videotarget, filename+filepath.Ext(videofile))
		database.InsertNamed("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :movie_id, :dbmovie_id, :height, :width)",
			database.MovieFile{
				Location:       targetfile,
				Filename:       filepath.Base(targetfile),
				Extension:      filepath.Ext(targetfile),
				QualityProfile: config.Cfg.Media[structurevar.cfg].ListsMap[structurevar.listConfig].Template_quality,
				ResolutionID:   structurevar.N.ResolutionID,
				QualityID:      structurevar.N.QualityID,
				CodecID:        structurevar.N.CodecID,
				AudioID:        structurevar.N.AudioID,
				Proper:         structurevar.N.Proper,
				Repack:         structurevar.N.Repack,
				Extended:       structurevar.N.Extended,
				MovieID:        movieid,
				DbmovieID:      dbmovieid,
				Height:         structurevar.N.Height,
				Width:          structurevar.N.Width})

		reached := false

		if structurevar.N.Priority >= parser.NewCutoffPrio(structurevar.cfg, config.Cfg.Media[structurevar.cfg].ListsMap[structurevar.listConfig].Template_quality) {
			reached = true
		}
		database.UpdateNamed("Update movies SET missing = :missing, quality_reached = :quality_reached where id = :id", database.Movie{ID: movieid, Missing: false, QualityReached: reached})
	} else {
		logger.Log.GlobalLogger.Error("Error moving video - unknown reason")
	}
	oldfiles = nil
}

func (s *Structure) checkDisallowedtype(rootpath string, removefolder bool) bool {
	if scanner.CheckFileExist(rootpath) {
		files, err := scanner.GetFilesDirAll(rootpath)
		if err != nil {
			return false
		}
		defer files.Close()
		disallowed := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[s.sourcepath].DisallowedLower}
		defer disallowed.Close()

		for idxfile := range files.Arr {
			if logger.InStringArrayContainsCaseInSensitive(files.Arr[idxfile], disallowed) {
				logger.Log.GlobalLogger.Warn("path not allowed", zap.String("path", files.Arr[idxfile]))

				if removefolder {
					scanner.CleanUpFolder(rootpath, 80000)
				}
				return false
				//}
			}
		}
		return true
	} else {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", rootpath))
	}
	return true
}
func StructureSingleFolderAs(folder string, id int, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepathstr string, targetpathstr string, cfg string) {
	structurevar, err := NewStructure(cfg, "", grouptype, folder, sourcepathstr, targetpathstr)
	if err != nil {
		return
	}
	checkruntime := config.Cfg.Paths[structurevar.sourcepath].CheckRuntime
	if disableruntimecheck {
		checkruntime = false
	}
	checkdisallowed := config.Cfg.Paths[structurevar.sourcepath].Disallowed
	if disabledisallowed {
		checkdisallowed = []string{}
	}
	if !structurevar.checkDisallowed() {
		if config.Cfg.Paths[structurevar.sourcepath].DeleteDisallowed {
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
	defer videofiles.Close()

	if structurevar.groupType == "movie" {
		if len(videofiles.Arr) >= 2 {
			//skip too many  files
			return
		}
	}
	deletewronglanguage := config.Cfg.Paths[structurevar.targetpath].DeleteWrongLanguage
	if disabledeletewronglanguage {
		deletewronglanguage = false
	}
	for fileidx := range videofiles.Arr {
		if filepath.Ext(videofiles.Arr[fileidx]) == "" {
			continue
		}
		if structurevar.groupType == "series" {
			if structurevar.removeSmallVideoFile(videofiles.Arr[fileidx]) {
				continue
			}
		}

		if logger.ContainsIa(videofiles.Arr[fileidx], "_unpack") {
			logger.Log.GlobalLogger.Warn("Unpacking - skipping", zap.String("path", videofiles.Arr[fileidx]))
			continue
		}
		err = structurevar.ParseFile(videofiles.Arr[fileidx], true, folder, deletewronglanguage)
		if err != nil {

			logger.Log.GlobalLogger.Error("Error parsing", zap.String("path", videofiles.Arr[fileidx]), zap.Error(err))
			continue
		}
		if structurevar.groupType == "movie" {
			structurevar.structureMovie(folder, uint(id), videofiles.Arr[fileidx], deletewronglanguage, checkruntime, checkdisallowed)
		} else if structurevar.groupType == "series" {
			//find dbseries
			structurevar.structureSeries(folder, uint(id), videofiles.Arr[fileidx], deletewronglanguage, checkruntime, checkdisallowed)
		}

	}
}

func StructureSingleFolder(folder string, disableruntimecheck bool, disabledisallowed bool, disabledeletewronglanguage bool, grouptype string, sourcepathstr string, targetpathstr string, cfg string) {
	structurevar, err := NewStructure(cfg, "", grouptype, folder, sourcepathstr, targetpathstr)
	if err != nil {
		logger.Log.GlobalLogger.Error("Structure failed ", zap.String("path", folder))

		return
	}
	defer structurevar.Close()

	allfiles, err := scanner.GetFilesDirAll(folder)
	if err != nil {
		return
	}
	defer allfiles.Close()
	disallowed := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[structurevar.sourcepath].DisallowedLower}
	defer disallowed.Close()
	removefolder := config.Cfg.Paths[structurevar.sourcepath].DeleteDisallowed

	removesmallfiles := true
	if structurevar.groupType != "movie" {
		removesmallfiles = false
		removefolder = false
	}

	for idxfile := range allfiles.Arr {
		if logger.InStringArrayContainsCaseInSensitive(allfiles.Arr[idxfile], disallowed) {
			logger.Log.GlobalLogger.Warn("path not allowed", zap.String("path", allfiles.Arr[idxfile]))

			if removefolder {
				allfiles, _ = structurevar.filterVideoFiles(allfiles, removesmallfiles)
				for idxremove := range allfiles.Arr {
					scanner.RemoveFile(allfiles.Arr[idxremove])
				}
				structurevar.fileCleanupFilter(folder, "")
			}
			logger.Log.GlobalLogger.Warn("Structure not allowed ", zap.String("path", folder))
			return
		}
	}

	allfiles, err = structurevar.filterVideoFiles(allfiles, removesmallfiles)
	if err != nil {
		logger.Log.GlobalLogger.Debug("Folder skipped due to no video files found ", zap.String("path", folder))
		//skip files
		return
	}

	if len(allfiles.Arr) == 0 {
		//skip mo  files
		return
	}
	if structurevar.groupType == "movie" {
		if len(allfiles.Arr) >= 2 {
			logger.Log.GlobalLogger.Warn("Folder skipped due to too many video files ", zap.String("path", folder))
			//skip too many  files
			return
		}
	}

	checkruntime := config.Cfg.Paths[structurevar.sourcepath].CheckRuntime
	if disableruntimecheck {
		checkruntime = false
	}
	checkdisallowed := config.Cfg.Paths[structurevar.sourcepath].Disallowed
	if disabledisallowed {
		checkdisallowed = []string{}
	}
	deletewronglanguage := config.Cfg.Paths[structurevar.targetpath].DeleteWrongLanguage
	if disabledeletewronglanguage {
		deletewronglanguage = false
	}

	var titleyear, template_quality string
	for fileidx := range allfiles.Arr {
		template_quality = ""
		if filepath.Ext(allfiles.Arr[fileidx]) == "" {
			continue
		}
		if logger.ContainsIa(allfiles.Arr[fileidx], "_unpack") {
			logger.Log.GlobalLogger.Warn("Unpacking - skipping", zap.String("path", allfiles.Arr[fileidx]))
			continue
		}
		if structurevar.groupType == "series" {
			if structurevar.removeSmallVideoFile(allfiles.Arr[fileidx]) {
				logger.Log.GlobalLogger.Debug("Folder skipped due to small video files - file was removed ", zap.String("path", allfiles.Arr[fileidx]))
				continue
			}
		}

		err = structurevar.ParseFile(allfiles.Arr[fileidx], true, folder, deletewronglanguage)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error parsing", zap.String("path", allfiles.Arr[fileidx]), zap.Error(err))
			continue
		}
		parser.GetDbIDs(structurevar.groupType, structurevar.N, cfg, "", true)
		if structurevar.N.Listname != "" {
			template_quality = config.Cfg.Media[cfg].ListsMap[structurevar.N.Listname].Template_quality
		}
		if template_quality == "" {
			continue
		}
		if !config.ConfigCheck("quality_" + template_quality) {
			logger.Log.GlobalLogger.Error("Quality for List: " + structurevar.N.Listname + " not found - for: " + allfiles.Arr[fileidx])
			continue
		}
		if structurevar.groupType == "movie" {
			if structurevar.N.MovieID != 0 && structurevar.N.DbmovieID != 0 {
				structurevar.listConfig = structurevar.N.Listname
				titleyear, _ = database.QueryColumnString("select title from dbmovies where id = ?", structurevar.N.DbmovieID)

				if searcher.Checktitle(apiexternal.Nzbwithprio{NZB: apiexternal.NZB{}, WantedTitle: titleyear, WantedAlternates: database.QueryStaticStringArray("select title from dbmovie_titles where dbmovie_id = ?", false, 0, structurevar.N.DbmovieID), QualityTemplate: template_quality, ParseInfo: *structurevar.N}, nil) {
					logger.Log.GlobalLogger.Warn("Skipped - unwanted title", zap.String("title", structurevar.N.Title), zap.String("want title", titleyear))
					continue
				}
				structurevar.structureMovie(folder, structurevar.N.MovieID, allfiles.Arr[fileidx], deletewronglanguage, checkruntime, checkdisallowed)
			} else {
				logger.Log.GlobalLogger.Debug("DB Movie not matched", zap.String("path", allfiles.Arr[fileidx]))
			}
		} else if structurevar.groupType == "series" {
			if structurevar.N.DbserieEpisodeID != 0 && structurevar.N.DbserieID != 0 && structurevar.N.SerieEpisodeID != 0 && structurevar.N.SerieID != 0 {
				structurevar.listConfig = structurevar.N.Listname
				titleyear, _ = database.QueryColumnString("select seriename from dbseries where id = ?", structurevar.N.DbserieID)

				if searcher.Checktitle(apiexternal.Nzbwithprio{NZB: apiexternal.NZB{}, WantedTitle: titleyear, WantedAlternates: database.QueryStaticStringArray("select title from dbserie_alternates where dbserie_id = ?", false, 0, structurevar.N.DbserieID), QualityTemplate: template_quality, ParseInfo: *structurevar.N}, nil) {
					logger.Log.GlobalLogger.Warn("Skipped - unwanted title", zap.String("title", structurevar.N.Title), zap.String("want title", titleyear))
					continue
				}
				structurevar.structureSeries(folder, structurevar.N.SerieID, allfiles.Arr[fileidx], deletewronglanguage, checkruntime, checkdisallowed)
			} else {
				logger.Log.GlobalLogger.Error("serie not matched ", zap.String("title", structurevar.N.Title))
			}
		}
	}
}

func StructureFolders(grouptype string, sourcepathstr string, targetpathstr string, cfg string) {
	jobName := sourcepathstr
	if !config.Cfg.Media[cfg].Structure {
		logger.Log.GlobalLogger.Debug("Structure disabled", zap.String("Job", jobName))
		return
	}

	lastStructure, ok := logger.GlobalCache.Get("StructureJobRunning")
	if ok {
		if lastStructure.Value.(string) == jobName {
			logger.Log.GlobalLogger.Debug("Job already running", zap.String("Job", jobName))
			return
		}
	}
	logger.GlobalCache.Set("StructureJobRunning", jobName, 5*time.Minute)

	logger.Log.GlobalLogger.Debug("Check Source", zap.String("path", sourcepathstr))
	folders, err := scanner.GetSubFolders(config.Cfg.Paths[sourcepathstr].Path)
	if err == nil {
		for idx := range folders {
			StructureSingleFolder(folders[idx], false, false, false, grouptype, sourcepathstr, targetpathstr, cfg)
		}
	}
	folders = nil
	logger.Log.GlobalLogger.Debug("Check Source end", zap.String("path", sourcepathstr))
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
	Source         apiexternal.ParseInfo
	Time           string
}

type forstructurenotify struct {
	Config        *Structure
	InputNotifier *inputNotifier
}

func structureSendNotify(event string, noticonfig *config.MediaNotificationConfig, notifierdata *forstructurenotify) {
	if !strings.EqualFold(noticonfig.Event, event) {
		return
	}
	if !config.ConfigCheck("notification_" + noticonfig.Map_notification) {
		return
	}
	messagetext, err := logger.ParseStringTemplate(noticonfig.Message, &notifierdata)
	if err != nil {
		return
	}
	messageTitle, err := logger.ParseStringTemplate(noticonfig.Title, &notifierdata)
	if err != nil {
		return
	}

	switch config.Cfg.Notification[noticonfig.Map_notification].NotificationType {
	case "pushover":
		if apiexternal.PushoverApi == nil {
			apiexternal.NewPushOverClient(config.Cfg.Notification[noticonfig.Map_notification].Apikey)
		}
		if apiexternal.PushoverApi.ApiKey != config.Cfg.Notification[noticonfig.Map_notification].Apikey {
			apiexternal.NewPushOverClient(config.Cfg.Notification[noticonfig.Map_notification].Apikey)
		}

		err = apiexternal.PushoverApi.SendMessage(messagetext, messageTitle, config.Cfg.Notification[noticonfig.Map_notification].Recipient)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error sending pushover ", zap.Error(err))
		} else {
			logger.Log.GlobalLogger.Info("Pushover message sent")
		}
	case "csv":
		f, err := os.OpenFile(config.Cfg.Notification[noticonfig.Map_notification].Outputto,
			os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error opening csv to write ", zap.Error(err))
			return
		}
		defer f.Close()
		if err == nil {
			_, err = io.WriteString(f, messagetext+"\n")
			if err != nil {
				logger.Log.GlobalLogger.Error("Error writing to csv ", zap.Error(err))
			} else {
				logger.Log.GlobalLogger.Info("csv written")
			}
		}
	}
}
func (s *Structure) GetEpisodeTitle(firstdbepiid uint, videofile string) (serietitle string, episodetitle string) {
	serietitle, episodetitle = config.RegexGetMatchesStr1Str2(`^(.*)(?i)`+s.N.Identifier+`(?:\.| |-)(.*)$`, filepath.Base(videofile))
	if serietitle != "" && episodetitle != "" {
		episodetitle = logger.TrimStringInclAfterString(episodetitle, "XXX")
		episodetitle = logger.TrimStringInclAfterString(episodetitle, s.N.Quality)
		episodetitle = logger.TrimStringInclAfterString(episodetitle, s.N.Resolution)
		episodetitle = strings.Trim(episodetitle, ".")
		episodetitle = strings.Replace(episodetitle, ".", " ", -1)

		serietitle = strings.Trim(serietitle, ".")
		serietitle = strings.Replace(serietitle, ".", " ", -1)
	}

	if len(episodetitle) == 0 {
		episodetitle, _ = database.QueryColumnString("select title from dbserie_episodes where id = ?", firstdbepiid)
	}
	return
}
func (s *Structure) notify(videotarget string, filename string, videofile string, id uint, listname string, oldfiles *[]string) {

	var err error
	notify := forstructurenotify{Config: s, InputNotifier: &inputNotifier{
		Targetpath:    filepath.Join(videotarget, filename),
		SourcePath:    videofile,
		Replaced:      *oldfiles,
		Configuration: listname,
		Source:        *s.N,
		Time:          time.Now().In(logger.TimeZone).Format(logger.TimeFormat),
	}}
	if strings.EqualFold(s.groupType, "movie") {
		notify.InputNotifier.Dbmovie, err = database.GetDbmovie(&database.Query{Where: "id = ?"}, id)
		if err != nil {
			return
		}
		notify.InputNotifier.Title = notify.InputNotifier.Dbmovie.Title
		notify.InputNotifier.Year = strconv.Itoa(notify.InputNotifier.Dbmovie.Year)
		notify.InputNotifier.Imdb = notify.InputNotifier.Dbmovie.ImdbID

	} else {
		notify.InputNotifier.DbserieEpisode, err = database.GetDbserieEpisodes(&database.Query{Where: "id = ?"}, id)
		if err != nil {
			return
		}
		notify.InputNotifier.Dbserie, err = database.GetDbserie(&database.Query{Where: "id = ?"}, notify.InputNotifier.DbserieEpisode.DbserieID)
		if err != nil {
			return
		}
		notify.InputNotifier.Title = notify.InputNotifier.Dbserie.Seriename
		notify.InputNotifier.Year = notify.InputNotifier.Dbserie.Firstaired
		notify.InputNotifier.Series = notify.InputNotifier.Dbserie.Seriename
		notify.InputNotifier.Tvdb = strconv.Itoa(notify.InputNotifier.Dbserie.ThetvdbID)
		notify.InputNotifier.Season = notify.InputNotifier.DbserieEpisode.Season
		notify.InputNotifier.Episode = notify.InputNotifier.DbserieEpisode.Episode
		notify.InputNotifier.Identifier = notify.InputNotifier.DbserieEpisode.Identifier
	}
	for idx := range config.Cfg.Media[s.cfg].Notification {
		notify.InputNotifier.ReplacedPrefix = config.Cfg.Media[s.cfg].Notification[idx].ReplacedPrefix
		structureSendNotify("added_data", &config.Cfg.Media[s.cfg].Notification[idx], &notify)
	}
}

func (s *Structure) GetSeriesEpisodes(serieid uint, dbserieid uint, videofile string, folder string) ([]string, bool, []database.Dbstatic_TwoInt) { //, []int, []database.SerieEpisode, , string, string, int
	identifiedby, dberr := database.QueryColumnString("select lower(identifiedby) from dbseries where id = ?", dbserieid)
	if dberr != nil {
		logger.Log.GlobalLogger.Error("Error no identified")
		return []string{}, false, []database.Dbstatic_TwoInt{}
	}

	episodeArray := importfeed.GetEpisodeArray(identifiedby, s.N.Identifier)
	if episodeArray == nil {
		return []string{}, false, []database.Dbstatic_TwoInt{}
	}
	defer episodeArray.Close()
	var err error
	var episodestoimport []database.Dbstatic_TwoInt = make([]database.Dbstatic_TwoInt, 0, len(episodeArray.Arr))
	if len(episodeArray.Arr) == 1 && s.N.DbserieEpisodeID != 0 && s.N.SerieEpisodeID != 0 {
		episodestoimport = append(episodestoimport, database.Dbstatic_TwoInt{Num1: int(s.N.SerieEpisodeID), Num2: int(s.N.DbserieEpisodeID)})
	} else {
		var dbserieepisodeid, serieepisodeid uint
		for idx := range episodeArray.Arr {
			episodeArray.Arr[idx] = strings.Trim(episodeArray.Arr[idx], "-EX")
			if identifiedby != "date" {
				episodeArray.Arr[idx] = strings.TrimLeft(episodeArray.Arr[idx], "0")
				if episodeArray.Arr[idx] == "" || err != nil {
					continue
				}
			}

			dbserieepisodeid, _ = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid, s.N.Identifier, s.N.SeasonStr, episodeArray.Arr[idx])
			if dbserieepisodeid != 0 {
				serieepisodeid, _ = database.QueryColumnUint("select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", dbserieepisodeid, serieid)
				if serieepisodeid != 0 {
					episodestoimport = append(episodestoimport, database.Dbstatic_TwoInt{Num1: int(serieepisodeid), Num2: int(dbserieepisodeid)})
				}
			}
		}
	}

	parser.GetPriorityMap(s.N, s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality, true)

	var allowimport bool
	var oldfiles []string = make([]string, 0, len(episodestoimport))

	var episodefiles []int = make([]int, 0, len(episodestoimport))
	var old_prio, entry_prio int
	var loc, additionalfile string
	var exportepisodestoimport []database.Dbstatic_TwoInt = make([]database.Dbstatic_TwoInt, 0, len(episodestoimport))

	for idx := range episodestoimport {
		episodefiles = database.QueryStaticIntArray("select id as num from serie_episode_files where serie_episode_id = ?", 1, episodestoimport[idx].Num1)
		old_prio = searcher.GetHighestEpisodePriorityByFiles(uint(episodestoimport[idx].Num1), s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality)
		if s.N.Priority > old_prio {
			allowimport = true
			for idxfile := range episodefiles {
				entry_prio = searcher.GetSerieDBPriorityById(uint(episodefiles[idxfile]), s.cfg, config.Cfg.Media[s.cfg].ListsMap[s.listConfig].Template_quality)
				if s.N.Priority > entry_prio {
					loc, _ = database.QueryColumnString("select location from serie_episode_files where id = ?", episodefiles[idxfile])
					oldfiles = append(oldfiles, loc)
				}
			}
		} else if len(episodefiles) == 0 {
			allowimport = true
		} else {
			err = scanner.RemoveFile(videofile)
			if err == nil {
				logger.Log.GlobalLogger.Debug("Lower Qual Import File removed", zap.String("path", videofile), zap.Int("old prio", old_prio), zap.Int("new prio", s.N.Priority))
				for idxext := range config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions {
					additionalfile = strings.Replace(videofile, filepath.Ext(videofile), config.Cfg.Paths[s.sourcepath].AllowedOtherExtensions[idxext], -1)
					err = scanner.RemoveFile(additionalfile)
					if err == nil {
						logger.Log.GlobalLogger.Debug("Lower Qual Import Additional File removed", zap.String("path", additionalfile))
					}
				}
				scanner.CleanUpFolder(folder, config.Cfg.Paths[s.sourcepath].CleanupsizeMB)
			}
			continue
		}
		if len(episodefiles) == 0 {
			allowimport = true
		} else {
			if !allowimport {
				logger.Log.GlobalLogger.Debug("Import Not allowed - no source files found")
			}
		}
		if allowimport {
			exportepisodestoimport = append(exportepisodestoimport, database.Dbstatic_TwoInt{Num1: episodestoimport[idx].Num1, Num2: episodestoimport[idx].Num2})
		} else {
			logger.Log.GlobalLogger.Warn("Import Not allowed - file", zap.String("path", videofile), zap.String("Identifier", s.N.Identifier))
			continue
		}
	}
	episodestoimport = nil
	episodefiles = nil
	return oldfiles, allowimport, exportepisodestoimport //, episodes, seriesEpisodes, serietitle, episodetitle, runtime
}

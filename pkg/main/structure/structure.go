package structure

import (
	"errors"
	"io"
	"io/fs"
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

type organizer struct {
	cfgp          *config.MediaTypeConfig
	listConfig    string
	listcfg       *config.MediaListsConfig
	groupType     string //series, movies
	rootpath      string //1st level below input
	sourcepath    string
	sourcepathcfg *config.PathsConfig
	targetpath    string
	targetpathcfg *config.PathsConfig
	//N             *apiexternal.ParseInfo
}
type parsertype struct {
	Dbmovie            *database.Dbmovie
	Dbserie            *database.Dbserie
	DbserieEpisode     *database.DbserieEpisode
	Source             *apiexternal.ParseInfo
	TitleSource        string
	EpisodeTitleSource string
	Identifier         string
	Episodes           []int
}
type Config struct {
	Disableruntimecheck        bool
	Disabledisallowed          bool
	Disabledeletewronglanguage bool
	Grouptype                  string
	Sourcepathstr              string
	Targetpathstr              string
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
	Dbmovie        *database.Dbmovie
	Dbserie        *database.Dbserie
	DbserieEpisode *database.DbserieEpisode
	Source         *apiexternal.ParseInfo
	Time           string
}
type forstructurenotify struct {
	Config        *organizer
	InputNotifier *inputNotifier
}

const strQualityForList = "Quality for List: "
const strNotFound = " not found"

var errNoQuality = errors.New("quality not found")
var errNoList = errors.New("list not found")
var errRuntime = errors.New("wrong runtime")
var errLanguage = errors.New("wrong language")
var errNotAllowed = errors.New("not allowed")
var errLowerQuality = errors.New("lower quality")
var structureJobRunning string

func NewStructure(cfgp *config.MediaTypeConfig, listname string, groupType string, rootpath string, sourcepathstr string, targetpathstr string) (*organizer, error) {
	if !cfgp.Structure {
		return nil, errNotAllowed
	}
	return &organizer{
		cfgp:          cfgp,
		listConfig:    listname,
		listcfg:       cfgp.GetList(listname),
		groupType:     groupType,
		rootpath:      rootpath,
		sourcepath:    sourcepathstr,
		targetpath:    targetpathstr,
		sourcepathcfg: config.Cfg.GetPath(sourcepathstr),
		targetpathcfg: config.Cfg.GetPath(targetpathstr),
	}, nil
}

func (s *Config) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s = nil
}
func (s *organizer) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.listcfg.Close()
	s.sourcepathcfg.Close()
	s.targetpathcfg.Close()
	s = nil
}
func (s *organizer) checkDisallowed() bool {
	check := s.sourcepathcfg.DeleteDisallowed
	if s.groupType == "series" {
		check = false
	}
	var disallowed bool
	if !scanner.CheckFileExist(s.rootpath) {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", s.rootpath))
		return disallowed
	}

	filepath.WalkDir(s.rootpath, scanner.WalkDisAllowed(check, s.rootpath, s.sourcepathcfg, &disallowed))
	return disallowed
}

func (s *organizer) filterVideoFiles(allfiles *logger.InStringArrayStruct, removesmallfiles bool) error {
	err := scanner.FilterFilesDir(allfiles, s.sourcepathcfg.Name, false, false)
	if err != nil {
		allfiles.Arr = nil
		return err
	}
	if removesmallfiles && s.sourcepathcfg.MinVideoSize > 0 {
		var removed bool
		var allfilesremoved logger.InStringArrayStruct
		for idx := range allfiles.Arr {
			if scanner.GetFileSize(allfiles.Arr[idx], true) < s.sourcepathcfg.MinVideoSizeByte {
				scanner.RemoveFiles(allfiles.Arr[idx], s.sourcepathcfg.Name)
				removed = true
			} else {
				allfilesremoved.Arr = append(allfilesremoved.Arr, allfiles.Arr[idx])
			}
		}
		if removed {
			allfiles = &allfilesremoved
		}
	}
	return nil
}

func (s *organizer) removeSmallVideoFile(file string) bool {
	if !scanner.CheckFileExist(file) {
		return false
	}
	if s.sourcepathcfg.MinVideoSize > 0 && scanner.GetFileSize(file, false) < s.sourcepathcfg.MinVideoSizeByte {
		scanner.RemoveFiles(file, s.sourcepathcfg.Name)
		return true
	}
	return false
}

// Parses - uses fprobe and checks language
func (s *organizer) ParseFile(videofile string, checkfolder bool, folder string, deletewronglanguage bool) *apiexternal.ParseInfo {
	var yearintitle bool
	if s.groupType == "series" {
		yearintitle = true
	}
	m := parser.NewFileParser(filepath.Base(videofile), yearintitle, s.groupType)
	if m.Quality != "" || m.Resolution != "" || !checkfolder {
		return m
	}
	logger.Log.GlobalLogger.Debug("Parse of folder ", zap.Stringp("path", &folder))
	mf := parser.NewFileParser(filepath.Base(folder), yearintitle, s.groupType)
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
	mf.Close()
	return m
}

func (s *organizer) fileCleanup(folder string, videofile string) {
	if strings.EqualFold(s.groupType, "movie") || videofile == "" {
		if scanner.RemoveFile(videofile) == nil {
			toRemove, err := scanner.GetFilesDir(folder, s.sourcepathcfg.Name, false)
			if err == nil {
				for idx := range toRemove.Arr {
					scanner.RemoveFile(toRemove.Arr[idx])
				}
				toRemove.Close()
			}
		}
		scanner.CleanUpFolder(folder, s.sourcepathcfg.CleanupsizeMB)
	} else {
		fileext := filepath.Ext(videofile)
		if scanner.RemoveFile(videofile) == nil {
			for idxext := range s.sourcepathcfg.AllowedOtherExtensions {
				scanner.RemoveFile(strings.ReplaceAll(videofile, fileext, s.sourcepathcfg.AllowedOtherExtensions[idxext]))
			}
		}
		scanner.CleanUpFolder(folder, s.sourcepathcfg.CleanupsizeMB)
	}
}
func (s *organizer) ParseFileAdditional(videofile string, folder string, deletewronglanguage bool, wantedruntime int, checkruntime bool, m *apiexternal.ParseInfo) error {
	if s.listConfig == "" {
		return errNoList
	}
	if !config.Check("quality_" + s.listcfg.TemplateQuality) {
		logger.Log.GlobalLogger.Error(strQualityForList + s.listConfig + strNotFound + " " + s.listcfg.TemplateQuality)
		return errNoQuality
	}

	parser.GetPriorityMap(m, s.cfgp, s.listcfg.TemplateQuality, true, true)
	err := parser.ParseVideoFile(m, videofile, s.listcfg.TemplateQuality)
	if err != nil {
		return err
	}
	if m.Runtime >= 1 && checkruntime && wantedruntime != 0 && s.targetpathcfg.MaxRuntimeDifference != 0 && (m.Runtime/60) != wantedruntime {
		maxdifference := s.targetpathcfg.MaxRuntimeDifference
		if m.Extended && strings.EqualFold(s.groupType, "movie") {
			maxdifference += 10
		}
		var difference int
		if wantedruntime > (m.Runtime / 60) {
			difference = wantedruntime - int(m.Runtime/60)
		} else {
			difference = int(m.Runtime/60) - wantedruntime
		}
		if difference > maxdifference {
			if s.targetpathcfg.DeleteWrongRuntime {
				s.fileCleanup(folder, videofile)
			}
			logger.Log.GlobalLogger.Error("Wrong runtime: Wanted ", zap.Int("wanted", wantedruntime), zap.Int("Having", int(m.Runtime/60)), zap.Int("difference", difference), zap.Stringp("path", &m.File))
			return errRuntime
		}
	}
	if len(s.targetpathcfg.AllowedLanguages) == 0 || !deletewronglanguage {
		return nil
	}
	var languageOk bool
	allowed := logger.InStringArrayStruct{Arr: m.Languages}
	lenlang := len(m.Languages)
	for idx := range s.targetpathcfg.AllowedLanguages {
		if lenlang == 0 && s.targetpathcfg.AllowedLanguages[idx] == "" {
			languageOk = true
			break
		}
		if logger.InStringArray(s.targetpathcfg.AllowedLanguages[idx], &allowed) {
			languageOk = true
			break
		}
	}
	allowed.Close()
	if !languageOk {
		s.fileCleanup(folder, videofile)
		logger.Log.GlobalLogger.Error("Wrong language: Wanted ", zap.Strings("Allowed", s.targetpathcfg.AllowedLanguages), zap.Strings("Have", m.Languages), zap.Stringp("path", &m.File))
		return errLanguage
	}
	return nil
}

func (s *organizer) checkLowerQualTarget(folder string, videofile string, cleanuplowerquality bool, movieid uint, m *apiexternal.ParseInfo) (*logger.InStringArrayStruct, error) {
	if s.listConfig == "" {
		return nil, errNoList
	}

	if !config.Check("quality_" + s.listcfg.TemplateQuality) {
		logger.Log.GlobalLogger.Error(strQualityForList + s.listConfig + strNotFound)
		return nil, errNoQuality
	}
	var moviefiles []database.DbstaticOneStringOneInt
	database.QueryStaticColumnsOneStringOneInt(false, 0, &database.Querywithargs{QueryString: "select location, id from movie_files where movie_id = ?", Args: []interface{}{movieid}}, &moviefiles)

	var oldpriority int
	if len(moviefiles) >= 1 {
		oldpriority = searcher.GetHighestMoviePriorityByFiles(true, true, movieid, s.cfgp, s.listcfg.TemplateQuality)
		logger.Log.GlobalLogger.Debug("Found existing highest prio", zap.Int("old", oldpriority))
	}

	if len(moviefiles) >= 1 && oldpriority != 0 && oldpriority >= m.Priority {
		logger.Log.GlobalLogger.Info("Skipped import due to lower quality", zap.Stringp("path", &videofile))
		if cleanuplowerquality {
			s.fileCleanup(folder, videofile)
		}
		moviefiles = nil
		return nil, errLowerQuality
	}
	if len(moviefiles) == 0 {
		moviefiles = nil
		return nil, nil
	}
	var lastprocessed, oldpath string
	oldfiles := logger.InStringArrayStruct{Arr: make([]string, 0, len(moviefiles)+1)}
	var entryPrio int

	for idx := range moviefiles {
		logger.Log.GlobalLogger.Debug("want to remove ", zap.Stringp("path", &moviefiles[idx].Str))
		oldpath, _ = filepath.Split(moviefiles[idx].Str)
		entryPrio = searcher.GetMovieDBPriorityByID(true, true, uint(moviefiles[idx].Num), s.cfgp, s.listcfg.TemplateQuality)
		logger.Log.GlobalLogger.Debug("want to remove oldprio ", zap.Int("old", entryPrio))
		if entryPrio != 0 && m.Priority > entryPrio && s.targetpathcfg.Upgrade {
			//oldfiles = append(oldfiles, moviefiles[idx].Str)
			if lastprocessed != oldpath {
				lastprocessed = oldpath
				filepath.WalkDir(oldpath, scanner.WalkAll(&oldfiles, true))
			}
		}
	}
	moviefiles = nil
	return &oldfiles, nil
}

func (s *organizer) GenerateNamingTemplate(videofile string, rootpath string, dbid uint, serietitle string, episodetitle string, mapepi *[]database.DbstaticTwoUint, m *apiexternal.ParseInfo) (string, string) {
	var foldername, filename string
	forparser := parsertype{Source: m}
	defer forparser.Close()
	if strings.EqualFold(s.groupType, "movie") {
		forparser.Dbmovie = new(database.Dbmovie)
		if database.GetDbmovie(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{dbid}}, forparser.Dbmovie) != nil {
			return "", ""
		}
		movietitle := filepath.Base(videofile)
		movietitle = logger.TrimStringInclAfterString(movietitle, m.Quality)
		movietitle = logger.TrimStringInclAfterString(movietitle, m.Resolution)
		if m.Year != 0 {
			movietitle = logger.TrimStringInclAfterString(movietitle, logger.IntToString(m.Year))
		}
		movietitle = strings.Trim(movietitle, ".")
		movietitle = strings.ReplaceAll(movietitle, ".", " ")
		forparser.TitleSource = strings.ReplaceAll(movietitle, "/", "")

		if forparser.Dbmovie.Title == "" {
			database.QueryColumn(&database.Querywithargs{QueryString: "select title from dbmovie_titles where dbmovie_id = ?", Args: []interface{}{dbid}}, &forparser.Dbmovie.Title)
			if forparser.Dbmovie.Title == "" {
				forparser.Dbmovie.Title = movietitle
			}
		}
		forparser.Dbmovie.Title = strings.ReplaceAll(forparser.Dbmovie.Title, "/", "")
		if forparser.Dbmovie.Year == 0 {
			forparser.Dbmovie.Year = m.Year
		}

		foldername, filename = path.Split(s.cfgp.Naming)
		if rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
		}

		if !strings.HasPrefix(m.Imdb, "tt") && m.Imdb != "" {
			m.Imdb = "tt" + m.Imdb
		}
		if m.Imdb == "" {
			m.Imdb = forparser.Dbmovie.ImdbID
		}

		forparser.Source.Title = strings.ReplaceAll(forparser.Source.Title, "/", "")

		var err error
		foldername, err = logger.ParseStringTemplate(foldername, forparser)
		if err != nil {
			return "", ""
		}
		filename, err = logger.ParseStringTemplate(filename, forparser)
		if err != nil {
			return "", ""
		}
		foldername = logger.Path(logger.StringReplaceDiacritics(strings.Trim(foldername, ".")), true)

		filename = strings.ReplaceAll(strings.Trim(filename, "."), "  ", " ")
		filename = strings.ReplaceAll(filename, " ]", "]")
		filename = strings.ReplaceAll(filename, "[ ", "[")
		filename = strings.ReplaceAll(filename, "[ ]", "")
		filename = strings.ReplaceAll(filename, "( )", "")
		filename = strings.ReplaceAll(filename, "[]", "")
		filename = strings.ReplaceAll(filename, "()", "")
		filename = strings.ReplaceAll(filename, "  ", " ")
		filename = logger.Path(logger.StringReplaceDiacritics(filename), true)
	} else {
		var epi database.SerieEpisode
		if database.GetSerieEpisodes(&database.Querywithargs{Query: database.Query{Select: "dbserie_id, dbserie_episode_id, serie_id", Where: logger.FilterByID}, Args: []interface{}{dbid}}, &epi) != nil {
			return "", ""
		}
		forparser.Dbserie = new(database.Dbserie)
		if database.GetDbserie(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{epi.DbserieID}}, forparser.Dbserie) != nil {
			return "", ""
		}
		forparser.DbserieEpisode = new(database.DbserieEpisode)
		if database.GetDbserieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{epi.DbserieEpisodeID}}, forparser.DbserieEpisode) != nil {
			return "", ""
		}
		foldername, filename = path.Split(s.cfgp.Naming)

		if forparser.Dbserie.Seriename == "" {
			if database.QueryColumn(&database.Querywithargs{QueryString: "select title from dbserie_alternates where dbserie_id = ?", Args: []interface{}{epi.DbserieID}}, &forparser.Dbserie.Seriename) != nil {
				forparser.Dbserie.Seriename = serietitle
			}
		}
		forparser.Dbserie.Seriename = strings.ReplaceAll(forparser.Dbserie.Seriename, "/", "")
		if forparser.DbserieEpisode.Title == "" {
			forparser.DbserieEpisode.Title = episodetitle
		}
		forparser.DbserieEpisode.Title = strings.ReplaceAll(forparser.DbserieEpisode.Title, "/", "")
		if rootpath != "" {
			foldername, _ = logger.Getrootpath(foldername)
		}

		forparser.Episodes = make([]int, len(*mapepi))
		queryepisode := "select episode from dbserie_episodes where id = ?"
		var epitext string
		var err error
		for key := range *mapepi {
			database.QueryColumn(&database.Querywithargs{QueryString: queryepisode, Args: []interface{}{(*mapepi)[key].Num2}}, &epitext)
			forparser.Episodes[key] = logger.StringToInt(epitext)
		}
		forparser.TitleSource = strings.ReplaceAll(serietitle, "/", "")
		forparser.EpisodeTitleSource = strings.ReplaceAll(episodetitle, "/", "")
		if m.Tvdb == "" {
			m.Tvdb = logger.IntToString(forparser.Dbserie.ThetvdbID)
		}
		if !strings.HasPrefix(m.Tvdb, "tvdb") && m.Tvdb != "" {
			m.Tvdb = "tvdb" + m.Tvdb
		}

		foldername, err = logger.ParseStringTemplate(foldername, forparser)
		if err != nil {
			return "", ""
		}
		filename, err = logger.ParseStringTemplate(filename, forparser)
		if err != nil {
			return "", ""
		}
		foldername = logger.Path(logger.StringReplaceDiacritics(strings.Trim(foldername, ".")), true)

		filename = strings.ReplaceAll(strings.Trim(filename, "."), "  ", " ")
		filename = strings.ReplaceAll(filename, " ]", "]")
		filename = strings.ReplaceAll(filename, "[ ", "[")
		filename = strings.ReplaceAll(filename, "[ ]", "")
		filename = strings.ReplaceAll(filename, "( )", "")
		filename = strings.ReplaceAll(filename, "[]", "")
		filename = strings.ReplaceAll(filename, "()", "")
		filename = strings.ReplaceAll(filename, "  ", " ")
		filename = logger.Path(logger.StringReplaceDiacritics(filename), true)
	}
	return foldername, filename
}

func (s *organizer) moveVideoFile(foldername string, filename string, videofile string, rootpath string) (string, bool, int) {
	videotarget := filepath.Join(s.targetpathcfg.Path, foldername)
	if rootpath != "" {
		videotarget = filepath.Join(rootpath, foldername)
	}

	mode := os.FileMode(0777)
	if s.targetpathcfg.SetChmod != "" && len(s.targetpathcfg.SetChmod) == 4 {
		tempval, _ := strconv.ParseUint(s.targetpathcfg.SetChmod, 0, 32)
		mode = fs.FileMode(uint32(tempval))
	}
	os.MkdirAll(videotarget, mode)

	if scanner.MoveFile(videofile, videotarget, filename, &s.sourcepathcfg.AllowedVideoExtensionsIn, &s.sourcepathcfg.AllowedVideoExtensionsNoRenameIn, config.Cfg.General.UseFileBufferCopy, s.targetpathcfg.SetChmod) {
		return videotarget, true, 1
	}
	return videotarget, false, 0
}

func (s *organizer) updateRootpath(rootpath string, foldername string, mediarootpath string, id uint) {
	if s.targetpathcfg.Usepresort {
		return
	}

	folders := strings.Split(foldername, "/")
	if len(folders) >= 2 {
		rootpath = strings.TrimRight(logger.TrimStringInclAfterString(rootpath, strings.TrimRight(folders[1], "/")), "/")
	}
	folders = nil
	if strings.EqualFold(s.groupType, "movie") && mediarootpath == "" {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies set rootpath = ? where id = ?", Args: []interface{}{rootpath, id}})
	} else if strings.EqualFold(s.groupType, "series") && mediarootpath == "" {
		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update series set rootpath = ? where id = ?", Args: []interface{}{rootpath, id}})
	}
}

func (s *organizer) moveRemoveOldMediaFile(oldfile string, id uint, usebuffer bool, move bool) {

	var ok bool
	if move {
		if scanner.MoveFile(oldfile, filepath.Join(s.targetpathcfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", &logger.InStringArrayStruct{Arr: []string{}}, &logger.InStringArrayStruct{Arr: []string{}}, usebuffer, s.targetpathcfg.SetChmod) {
			ok = true
		}
	} else {
		if scanner.RemoveFile(oldfile) == nil {
			ok = true
		}
	}
	if !ok {
		logger.Log.GlobalLogger.Error("Old File could not be removed", zap.String("path", oldfile))
		return
	}
	fileext := filepath.Ext(oldfile)
	logger.Log.GlobalLogger.Debug("Old File moved", zap.Stringp("path", &oldfile))
	if strings.EqualFold(s.groupType, "movie") {
		database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from movie_files where movie_id = ? and location = ?", Args: []interface{}{id, oldfile}})
	} else {
		database.DeleteRowStatic(&database.Querywithargs{QueryString: "Delete from serie_episode_files where serie_id = ? and location = ?", Args: []interface{}{id, oldfile}})
	}
	var additionalfile string
	var err error
	for idxext := range s.sourcepathcfg.AllowedOtherExtensions {
		ok = false
		additionalfile = strings.ReplaceAll(oldfile, fileext, s.sourcepathcfg.AllowedOtherExtensions[idxext])
		if additionalfile == oldfile {
			continue
		}
		if move {
			ok = scanner.MoveFile(additionalfile, filepath.Join(s.targetpathcfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", &logger.InStringArrayStruct{Arr: []string{}}, &logger.InStringArrayStruct{Arr: []string{}}, usebuffer, s.targetpathcfg.SetChmod)
		} else {
			err = scanner.RemoveFile(additionalfile)
			if err == nil {
				ok = true
			}
		}
		if ok {
			logger.Log.GlobalLogger.Debug("Additional File removed", zap.Stringp("path", &additionalfile))
		} else {
			logger.Log.GlobalLogger.Error("Additional File could not be removed", zap.String("path", additionalfile))
		}
	}

}

func (s *organizer) moveAdditionalFiles(folder string, videotarget string, filename string, videofile string, sourcefileext string, videofilecount int) {
	if strings.EqualFold(s.groupType, "movie") || videofilecount == 1 {
		additionalfiles, err := scanner.GetFilesDir(folder, s.sourcepathcfg.Name, true)
		if err == nil && len(additionalfiles.Arr) >= 1 {
			for idx := range additionalfiles.Arr {
				scanner.MoveFile(additionalfiles.Arr[idx], videotarget, filename, &s.sourcepathcfg.AllowedOtherExtensionsIn, &s.sourcepathcfg.AllowedOtherExtensionsNoRenameIn, config.Cfg.General.UseFileBufferCopy, s.targetpathcfg.SetChmod)
			}
		}
		additionalfiles.Close()
	} else {
		for idx := range s.sourcepathcfg.AllowedOtherExtensions {
			scanner.MoveFile(strings.ReplaceAll(videofile, sourcefileext, s.sourcepathcfg.AllowedOtherExtensions[idx]), videotarget, filename, &s.sourcepathcfg.AllowedVideoExtensionsIn, &s.sourcepathcfg.AllowedVideoExtensionsNoRenameIn, config.Cfg.General.UseFileBufferCopy, s.targetpathcfg.SetChmod)
		}
	}
}

func (s *organizer) organizeSeries(folder string, serieid uint, videofile string, deletewronglanguage bool, checkruntime bool, m *apiexternal.ParseInfo) {
	var dbserieid uint
	if database.QueryColumn(&database.Querywithargs{QueryString: "select dbserie_id from series where id = ?", Args: []interface{}{serieid}}, &dbserieid) != nil {
		logger.Log.GlobalLogger.Error("Error no dbserieid")
		return
	}
	var runtimestr, listname, rootpath string
	if database.QueryColumn(&database.Querywithargs{QueryString: "select runtime from dbseries where id = ?", Args: []interface{}{dbserieid}}, &runtimestr) != nil {
		logger.Log.GlobalLogger.Error("Error no runtime")
		return
	}

	database.QueryColumn(&database.Querywithargs{QueryString: "select listname from series where id = ?", Args: []interface{}{serieid}}, &listname)
	if database.QueryColumn(&database.Querywithargs{QueryString: "select rootpath from series where id = ?", Args: []interface{}{serieid}}, &rootpath) != nil {
		logger.Log.GlobalLogger.Error("Error no rootpath")
		return
	}
	if s.listConfig != listname || s.listcfg == nil {
		s.listConfig = listname
		listcfg := s.cfgp.ListsMap[listname]
		s.listcfg = &listcfg
	}

	oldfiles, allowimport, tblepi := s.GetSeriesEpisodes(serieid, dbserieid, videofile, folder, m)
	defer oldfiles.Close()
	if len(tblepi) == 0 {
		logger.Log.GlobalLogger.Error("Error no episodes")
		tblepi = nil
		return
	}
	if !allowimport {
		logger.Log.GlobalLogger.Warn("Import not allowed ", zap.String("path", folder))
		tblepi = nil
		return
	}

	firstdbepiid := tblepi[0].Num2
	firstepiid := tblepi[0].Num1
	var epiruntime uint
	database.QueryColumn(&database.Querywithargs{QueryString: "select runtime from dbserie_episodes where id = ?", Args: []interface{}{firstdbepiid}}, &epiruntime)

	runtime := logger.StringToInt(runtimestr)
	if epiruntime != 0 {
		runtime = int(epiruntime)
	}

	var season string
	if database.QueryColumn(&database.Querywithargs{QueryString: "select season from dbserie_episodes where id = ?", Args: []interface{}{firstdbepiid}}, &season) != nil {
		logger.Log.GlobalLogger.Error("Error no season")
		tblepi = nil
		return
	}

	var ignoreRuntime bool
	database.QueryColumn(&database.Querywithargs{QueryString: "select ignore_runtime from serie_episodes where id = ?", Args: []interface{}{firstepiid}}, &ignoreRuntime)
	if runtime == 0 && season == "0" {
		ignoreRuntime = true
	}
	totalruntime := runtime * len(tblepi)
	if ignoreRuntime {
		totalruntime = 0
	}

	err := s.ParseFileAdditional(videofile, folder, deletewronglanguage, totalruntime, checkruntime, m)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error fprobe video", zap.Stringp("path", &videofile), zap.Error(err))
		tblepi = nil
		return
	}

	serietitle, episodetitle := s.GetEpisodeTitle(firstdbepiid, videofile, m)

	foldername, filename := s.GenerateNamingTemplate(videofile, rootpath, firstepiid, serietitle, episodetitle, &tblepi, m)
	if foldername == "" || filename == "" {
		logger.Log.GlobalLogger.Error("Error generating foldername for", zap.String("path", videofile))
		tblepi = nil
		return
	}

	if s.targetpathcfg.MoveReplaced && len(oldfiles.Arr) >= 1 && s.targetpathcfg.MoveReplacedTargetPath != "" {
		for idxold := range oldfiles.Arr {
			s.moveRemoveOldMediaFile(oldfiles.Arr[idxold], serieid, config.Cfg.General.UseFileBufferCopy, true)
		}
	}

	if s.targetpathcfg.Usepresort && s.targetpathcfg.PresortFolderPath != "" {
		rootpath = filepath.Join(s.targetpathcfg.PresortFolderPath, foldername)
	}
	videotarget, moveok, moved := s.moveVideoFile(foldername, filename, videofile, rootpath)
	if !moveok || moved == 0 {
		tblepi = nil
		return
	}
	s.updateRootpath(videotarget, foldername, rootpath, serieid)

	if s.targetpathcfg.Replacelower && len(oldfiles.Arr) >= 1 {
		var oldfilename string
		for oldidx := range oldfiles.Arr {
			_, oldfilename = filepath.Split(oldfiles.Arr[oldidx])
			if strings.HasPrefix(oldfiles.Arr[oldidx], videotarget) && strings.EqualFold(oldfilename, filename) {
				//skip
			} else {
				s.moveRemoveOldMediaFile(oldfiles.Arr[oldidx], serieid, config.Cfg.General.UseFileBufferCopy, false)
			}
		}
	}

	s.moveAdditionalFiles(folder, videotarget, filename, videofile, filepath.Ext(videofile), len(videotarget))
	s.notify(videotarget, filename, videofile, firstdbepiid, listname, oldfiles, m)
	scanner.CleanUpFolder(folder, s.sourcepathcfg.CleanupsizeMB)

	//updateserie

	var reached bool

	if listname == "" {
		logger.Log.GlobalLogger.Error("Error no listname")
		oldfiles = nil
		tblepi = nil
		return
	}
	if !config.Check("quality_" + s.listcfg.TemplateQuality) {
		logger.Log.GlobalLogger.Error(strQualityForList + listname + strNotFound)
		oldfiles = nil
		tblepi = nil
		return
	}
	if m.Priority >= parser.NewCutoffPrio(s.cfgp, s.listcfg.TemplateQuality) {
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
				QualityProfile:   s.listcfg.TemplateQuality,
				ResolutionID:     m.ResolutionID,
				QualityID:        m.QualityID,
				CodecID:          m.CodecID,
				AudioID:          m.AudioID,
				Proper:           m.Proper,
				Repack:           m.Repack,
				Extended:         m.Extended,
				SerieID:          m.SerieID,
				SerieEpisodeID:   tblepi[key].Num1,
				DbserieEpisodeID: tblepi[key].Num2,
				DbserieID:        m.DbserieID,
				Height:           m.Height,
				Width:            m.Width})

		database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update serie_episodes SET missing = ?, quality_reached = ? where id = ?", Args: []interface{}{false, reached, tblepi[key].Num1}})

	}
	oldfiles = nil
	tblepi = nil
}
func (s *organizer) organizeMovie(folder string, movieid uint, videofile string, deletewronglanguage bool, checkruntime bool, m *apiexternal.ParseInfo) {
	var dbmovieid uint
	if database.QueryColumn(&database.Querywithargs{QueryString: "select dbmovie_id from movies where id = ?", Args: []interface{}{movieid}}, &dbmovieid) != nil {
		logger.Log.GlobalLogger.Error("Structure failed no dbmovieid ", zap.String("path", folder))
		return
	}
	var runtime uint
	if database.QueryColumn(&database.Querywithargs{QueryString: "select runtime from dbmovies where id = ?", Args: []interface{}{dbmovieid}}, &runtime) != nil {
		logger.Log.GlobalLogger.Error("Structure failed no runtime ", zap.String("path", folder))
		return
	}
	var listname, rootpath string
	database.QueryColumn(&database.Querywithargs{QueryString: "select listname from movies where id = ?", Args: []interface{}{movieid}}, &listname)
	if database.QueryColumn(&database.Querywithargs{QueryString: "select rootpath from movies where id = ?", Args: []interface{}{movieid}}, &rootpath) != nil {
		logger.Log.GlobalLogger.Error("Structure failed no rootpath ", zap.String("path", folder))
		return
	}
	if s.listConfig != listname || s.listcfg == nil {
		s.listConfig = listname
		listcfg := s.cfgp.ListsMap[listname]
		s.listcfg = &listcfg
	}
	err := s.ParseFileAdditional(videofile, folder, deletewronglanguage, int(runtime), checkruntime, m)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error fprobe video", zap.Stringp("path", &videofile), zap.Error(err))
		return
	}
	oldfiles, err := s.checkLowerQualTarget(folder, videofile, true, movieid, m)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error checking oldfiles", zap.Stringp("path", &videofile), zap.Error(err))
		oldfiles.Close()
		return
	}
	defer oldfiles.Close()
	foldername, filename := s.GenerateNamingTemplate(videofile, rootpath, dbmovieid, "", "", &[]database.DbstaticTwoUint{}, m)
	if foldername == "" || filename == "" {
		logger.Log.GlobalLogger.Error("Error generating foldername for", zap.String("path", videofile))
		oldfiles = nil
		return
	}

	if s.targetpathcfg.MoveReplaced && oldfiles != nil && len(oldfiles.Arr) >= 1 && s.targetpathcfg.MoveReplacedTargetPath != "" {
		for idxold := range oldfiles.Arr {
			s.moveRemoveOldMediaFile(oldfiles.Arr[idxold], movieid, config.Cfg.General.UseFileBufferCopy, true)
		}
	}
	if s.targetpathcfg.Usepresort && s.targetpathcfg.PresortFolderPath != "" {
		rootpath = filepath.Join(s.targetpathcfg.PresortFolderPath, foldername)
	}
	videotarget, moveok, moved := s.moveVideoFile(foldername, filename, videofile, rootpath)
	if !moveok || moved == 0 {
		logger.Log.GlobalLogger.Error("Error moving video - unknown reason")
		oldfiles = nil
		return
	}
	s.updateRootpath(videotarget, foldername, rootpath, movieid)

	if s.targetpathcfg.Replacelower && oldfiles != nil && len(oldfiles.Arr) >= 1 {
		var oldfilename string
		for oldidx := range oldfiles.Arr {
			_, oldfilename = filepath.Split(oldfiles.Arr[oldidx])
			if strings.HasPrefix(oldfiles.Arr[oldidx], videotarget) && strings.EqualFold(oldfilename, filename) {
				//skip
			} else {
				s.moveRemoveOldMediaFile(oldfiles.Arr[oldidx], movieid, config.Cfg.General.UseFileBufferCopy, false)
			}
		}
	}
	s.moveAdditionalFiles(folder, videotarget, filename, videofile, filepath.Ext(videofile), len(videotarget))

	s.notify(videotarget, filename, videofile, dbmovieid, listname, oldfiles, m)
	scanner.CleanUpFolder(folder, s.sourcepathcfg.CleanupsizeMB)

	if listname == "" {
		logger.Log.GlobalLogger.Error("Structure failed no list ", zap.String("path", folder))
		oldfiles = nil
		return
	}
	if !config.Check("quality_" + s.listcfg.TemplateQuality) {
		logger.Log.GlobalLogger.Error(strQualityForList + listname + strNotFound)
		oldfiles = nil
		return
	}
	//updatemovie
	targetfile := filepath.Join(videotarget, filename+filepath.Ext(videofile))
	database.InsertNamed("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (:location, :filename, :extension, :quality_profile, :resolution_id, :quality_id, :codec_id, :audio_id, :proper, :repack, :extended, :movie_id, :dbmovie_id, :height, :width)",
		database.MovieFile{
			Location:       targetfile,
			Filename:       filepath.Base(targetfile),
			Extension:      filepath.Ext(targetfile),
			QualityProfile: s.listcfg.TemplateQuality,
			ResolutionID:   m.ResolutionID,
			QualityID:      m.QualityID,
			CodecID:        m.CodecID,
			AudioID:        m.AudioID,
			Proper:         m.Proper,
			Repack:         m.Repack,
			Extended:       m.Extended,
			MovieID:        movieid,
			DbmovieID:      dbmovieid,
			Height:         m.Height,
			Width:          m.Width})

	var reached bool

	if m.Priority >= parser.NewCutoffPrio(s.cfgp, s.listcfg.TemplateQuality) {
		reached = true
	}
	database.UpdateColumnStatic(&database.Querywithargs{QueryString: "Update movies SET missing = ?, quality_reached = ? where id = ?", Args: []interface{}{false, reached, movieid}})
	oldfiles = nil
}
func (s *organizer) GetEpisodeTitle(firstdbepiid uint, videofile string, m *apiexternal.ParseInfo) (string, string) {
	serietitle, episodetitle := config.RegexGetMatchesStr1Str2(`^(.*)(?i)`+m.Identifier+`(?:\.| |-)(.*)$`, filepath.Base(videofile))
	if serietitle != "" && episodetitle != "" {
		episodetitle = logger.TrimStringInclAfterString(episodetitle, "XXX")
		episodetitle = logger.TrimStringInclAfterString(episodetitle, m.Quality)
		episodetitle = logger.TrimStringInclAfterString(episodetitle, m.Resolution)
		episodetitle = strings.ReplaceAll(strings.Trim(episodetitle, "."), ".", " ")

		serietitle = strings.ReplaceAll(strings.Trim(serietitle, "."), ".", " ")
	}

	if episodetitle == "" {
		database.QueryColumn(&database.Querywithargs{QueryString: "select title from dbserie_episodes where id = ?", Args: []interface{}{firstdbepiid}}, &episodetitle)
	}
	return serietitle, episodetitle
}
func (s *organizer) notify(videotarget string, filename string, videofile string, id uint, listname string, oldfiles *logger.InStringArrayStruct, m *apiexternal.ParseInfo) {
	if oldfiles == nil {
		oldfiles = new(logger.InStringArrayStruct)
	}
	notify := forstructurenotify{Config: s, InputNotifier: &inputNotifier{
		Targetpath:    filepath.Join(videotarget, filename),
		SourcePath:    videofile,
		Replaced:      oldfiles.Arr,
		Configuration: listname,
		Source:        m,
		Time:          time.Now().In(logger.TimeZone).Format(logger.TimeFormat),
	}}
	defer notify.close()
	if strings.EqualFold(s.groupType, "movie") {
		notify.InputNotifier.Dbmovie = new(database.Dbmovie)
		if database.GetDbmovie(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{id}}, notify.InputNotifier.Dbmovie) != nil {
			return
		}
		notify.InputNotifier.Title = notify.InputNotifier.Dbmovie.Title
		notify.InputNotifier.Year = logger.IntToString(notify.InputNotifier.Dbmovie.Year)
		notify.InputNotifier.Imdb = notify.InputNotifier.Dbmovie.ImdbID

	} else {
		notify.InputNotifier.DbserieEpisode = new(database.DbserieEpisode)
		if database.GetDbserieEpisodes(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{id}}, notify.InputNotifier.DbserieEpisode) != nil {
			return
		}
		notify.InputNotifier.Dbserie = new(database.Dbserie)
		if database.GetDbserie(&database.Querywithargs{Query: database.QueryFilterByID, Args: []interface{}{notify.InputNotifier.DbserieEpisode.DbserieID}}, notify.InputNotifier.Dbserie) != nil {
			return
		}
		notify.InputNotifier.Title = notify.InputNotifier.Dbserie.Seriename
		notify.InputNotifier.Year = notify.InputNotifier.Dbserie.Firstaired
		notify.InputNotifier.Series = notify.InputNotifier.Dbserie.Seriename
		notify.InputNotifier.Tvdb = logger.IntToString(notify.InputNotifier.Dbserie.ThetvdbID)
		notify.InputNotifier.Season = notify.InputNotifier.DbserieEpisode.Season
		notify.InputNotifier.Episode = notify.InputNotifier.DbserieEpisode.Episode
		notify.InputNotifier.Identifier = notify.InputNotifier.DbserieEpisode.Identifier
	}
	var messagetext, messageTitle string
	var err error
	for idx := range s.cfgp.Notification {
		notify.InputNotifier.ReplacedPrefix = s.cfgp.Notification[idx].ReplacedPrefix

		if !strings.EqualFold(s.cfgp.Notification[idx].Event, "added_data") {
			continue
		}
		if !config.Check("notification_" + s.cfgp.Notification[idx].MapNotification) {
			continue
		}
		messagetext, err = logger.ParseStringTemplate(s.cfgp.Notification[idx].Message, notify)
		if err != nil {
			continue
		}
		messageTitle, err = logger.ParseStringTemplate(s.cfgp.Notification[idx].Title, notify)
		if err != nil {
			continue
		}

		switch config.Cfg.Notification[s.cfgp.Notification[idx].MapNotification].NotificationType {
		case "pushover":
			if apiexternal.PushoverAPI == nil {
				apiexternal.NewPushOverClient(config.Cfg.Notification[s.cfgp.Notification[idx].MapNotification].Apikey)
			}
			if apiexternal.PushoverAPI.APIKey != config.Cfg.Notification[s.cfgp.Notification[idx].MapNotification].Apikey {
				apiexternal.NewPushOverClient(config.Cfg.Notification[s.cfgp.Notification[idx].MapNotification].Apikey)
			}

			err = apiexternal.PushoverAPI.SendMessage(messagetext, messageTitle, config.Cfg.Notification[s.cfgp.Notification[idx].MapNotification].Recipient)
			if err != nil {
				logger.Log.GlobalLogger.Error("Error sending pushover ", zap.Error(err))
			} else {
				logger.Log.GlobalLogger.Info("Pushover message sent")
			}
		case "csv":
			f, err := os.OpenFile(config.Cfg.Notification[s.cfgp.Notification[idx].MapNotification].Outputto,
				os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				logger.Log.GlobalLogger.Error("Error opening csv to write ", zap.Error(err))
				continue
			} else {
				defer f.Close()
				_, err = io.WriteString(f, messagetext+"\n")
				if err != nil {
					logger.Log.GlobalLogger.Error("Error writing to csv ", zap.Error(err))
				} else {
					logger.Log.GlobalLogger.Info("csv written")
				}
			}
		}
	}
}

func (s *organizer) GetSeriesEpisodes(serieid uint, dbserieid uint, videofile string, folder string, m *apiexternal.ParseInfo) (*logger.InStringArrayStruct, bool, []database.DbstaticTwoUint) { //, []int, []database.SerieEpisode, , string, string, int
	var identifiedby string
	if database.QueryColumn(&database.Querywithargs{QueryString: "select lower(identifiedby) from dbseries where id = ?", Args: []interface{}{dbserieid}}, &identifiedby) != nil {
		logger.Log.GlobalLogger.Error("Error no identified")
		return nil, false, []database.DbstaticTwoUint{}
	}

	episodeArray := importfeed.GetEpisodeArray(identifiedby, m.Identifier)
	if episodeArray == nil {
		return nil, false, []database.DbstaticTwoUint{}
	}
	var err error
	var episodestoimport = make([]database.DbstaticTwoUint, 0, len(episodeArray.Arr))
	if len(episodeArray.Arr) == 1 && m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
		episodestoimport = append(episodestoimport, database.DbstaticTwoUint{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID})
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

			dbserieepisodeid, _ = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid, m.Identifier, m.SeasonStr, episodeArray.Arr[idx])
			if dbserieepisodeid != 0 {
				database.QueryColumn(&database.Querywithargs{QueryString: "select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", Args: []interface{}{dbserieepisodeid, serieid}}, &serieepisodeid)
				if serieepisodeid != 0 {
					episodestoimport = append(episodestoimport, database.DbstaticTwoUint{Num1: serieepisodeid, Num2: dbserieepisodeid})
				}
			}
		}
	}
	episodeArray.Close()

	parser.GetPriorityMap(m, s.cfgp, s.listcfg.TemplateQuality, true, true)

	var allowimport bool
	oldfiles := logger.InStringArrayStruct{Arr: make([]string, 0, len(episodestoimport))}

	var episodefiles []uint
	var oldPrio int
	var loc string
	var exportepisodestoimport = make([]database.DbstaticTwoUint, 0, len(episodestoimport))

	for idx := range episodestoimport {
		episodefiles = []uint{}
		database.QueryStaticUintArray(1, &database.Querywithargs{QueryString: "select id from serie_episode_files where serie_episode_id = ?", Args: []interface{}{episodestoimport[idx].Num1}}, &episodefiles)
		oldPrio = searcher.GetHighestEpisodePriorityByFiles(true, true, episodestoimport[idx].Num1, s.cfgp, s.listcfg.TemplateQuality)
		if m.Priority > oldPrio || oldPrio == 0 {
			for idxfile := range episodefiles {
				if m.Priority > searcher.GetSerieDBPriorityByID(true, true, episodefiles[idxfile], s.cfgp, s.listcfg.TemplateQuality) {
					database.QueryColumn(&database.Querywithargs{QueryString: "select location from serie_episode_files where id = ?", Args: []interface{}{episodefiles[idxfile]}}, &loc)
					oldfiles.Arr = append(oldfiles.Arr, loc)
				}
			}
			allowimport = true
			exportepisodestoimport = append(exportepisodestoimport, database.DbstaticTwoUint{Num1: episodestoimport[idx].Num1, Num2: episodestoimport[idx].Num2})
			continue
		} else if len(episodefiles) == 0 {
			exportepisodestoimport = append(exportepisodestoimport, database.DbstaticTwoUint{Num1: episodestoimport[idx].Num1, Num2: episodestoimport[idx].Num2})
			allowimport = true
			continue
		} else {
			if scanner.RemoveFile(videofile) == nil {
				logger.Log.GlobalLogger.Debug("Lower Qual Import File removed", zap.Stringp("path", &videofile), zap.Int("old prio", oldPrio), zap.Int("new prio", m.Priority))
				for idxext := range s.sourcepathcfg.AllowedOtherExtensions {
					scanner.RemoveFile(strings.ReplaceAll(videofile, filepath.Ext(videofile), s.sourcepathcfg.AllowedOtherExtensions[idxext]))
				}
				scanner.CleanUpFolder(folder, s.sourcepathcfg.CleanupsizeMB)
			}
		}
	}
	episodestoimport = nil
	episodefiles = nil
	return &oldfiles, allowimport, exportepisodestoimport //, episodes, seriesEpisodes, serietitle, episodetitle, runtime
}

func OrganizeSingleFolderAs(folder string, id uint, cfgp *config.MediaTypeConfig, inConfig *Config) {
	defer inConfig.close()
	structurevar, err := NewStructure(cfgp, "", inConfig.Grouptype, folder, inConfig.Sourcepathstr, inConfig.Targetpathstr)
	if err != nil {
		return
	}
	defer structurevar.close()
	checkruntime := structurevar.sourcepathcfg.CheckRuntime
	if inConfig.Disableruntimecheck {
		checkruntime = false
	}
	if !structurevar.checkDisallowed() {
		if structurevar.sourcepathcfg.DeleteDisallowed {
			structurevar.fileCleanup(folder, "")
		}

		return
	}
	var removesmallfiles bool
	if structurevar.groupType == "movie" {
		removesmallfiles = true
	}
	videofiles, err := scanner.GetFilesDir(folder, structurevar.sourcepathcfg.Name, false)
	if err != nil {
		return
	}
	defer videofiles.Close()
	if len(videofiles.Arr) == 0 {
		return
	}
	if removesmallfiles && structurevar.sourcepathcfg.MinVideoSize > 0 {
		var removed bool
		for idx := range videofiles.Arr {
			if scanner.GetFileSize(videofiles.Arr[idx], true) < structurevar.sourcepathcfg.MinVideoSizeByte {
				scanner.RemoveFiles(videofiles.Arr[idx], structurevar.sourcepathcfg.Name)
				removed = true
			}
		}
		if removed {
			videofiles, err = scanner.GetFilesDir(folder, structurevar.sourcepathcfg.Name, false)
			if err != nil {
				return
			}
		}
	}

	if structurevar.groupType == "movie" && len(videofiles.Arr) >= 2 {
		//skip too many  files
		return
	}
	deletewronglanguage := structurevar.targetpathcfg.DeleteWrongLanguage
	if inConfig.Disabledeletewronglanguage {
		deletewronglanguage = false
	}
	var m *apiexternal.ParseInfo
	for fileidx := range videofiles.Arr {
		if filepath.Ext(videofiles.Arr[fileidx]) == "" {
			continue
		}
		if structurevar.groupType == "series" && structurevar.removeSmallVideoFile(videofiles.Arr[fileidx]) {
			continue
		}

		if logger.ContainsIa(videofiles.Arr[fileidx], "_unpack") {
			logger.Log.GlobalLogger.Warn("Unpacking - skipping", zap.Stringp("path", &videofiles.Arr[fileidx]))
			continue
		}
		m = structurevar.ParseFile(videofiles.Arr[fileidx], true, folder, deletewronglanguage)
		if m == nil {
			logger.Log.GlobalLogger.Debug("Parse failed", zap.Stringp("path", &videofiles.Arr[fileidx]))
			continue
		}
		if structurevar.groupType == "movie" {
			structurevar.organizeMovie(folder, id, videofiles.Arr[fileidx], deletewronglanguage, checkruntime, m)
		} else if structurevar.groupType == "series" {
			//find dbseries
			structurevar.organizeSeries(folder, id, videofiles.Arr[fileidx], deletewronglanguage, checkruntime, m)
		}
		m.Close()
	}
	m.Close()
}

func OrganizeSingleFolder(folder string, cfgp *config.MediaTypeConfig, inConfig *Config) {
	defer inConfig.close()
	structurevar, err := NewStructure(cfgp, "", inConfig.Grouptype, folder, inConfig.Sourcepathstr, inConfig.Targetpathstr)
	if err != nil {
		logger.Log.GlobalLogger.Error("Structure failed ", zap.Stringp("path", &folder))

		return
	}
	defer structurevar.close()

	allfiles, err := scanner.GetFilesDirAll(folder, false)
	if err != nil {
		logger.Log.GlobalLogger.Error("Structure failed all files ", zap.Stringp("path", &folder))
		return
	}
	defer allfiles.Close()
	if len(allfiles.Arr) == 0 {
		return
	}
	removefolder := structurevar.sourcepathcfg.DeleteDisallowed

	var removesmallfiles bool
	if structurevar.groupType == "movie" {
		removesmallfiles = true
		removefolder = true
	}

	for idxfile := range allfiles.Arr {
		if !logger.InStringArrayContainsCaseInSensitive(allfiles.Arr[idxfile], &structurevar.sourcepathcfg.DisallowedLowerIn) {
			continue
		}
		logger.Log.GlobalLogger.Warn("path not allowed", zap.Stringp("path", &allfiles.Arr[idxfile]))

		if removefolder {
			structurevar.filterVideoFiles(allfiles, removesmallfiles)
			for idxremove := range allfiles.Arr {
				scanner.RemoveFile(allfiles.Arr[idxremove])
			}

			scanner.CleanUpFolder(folder, structurevar.sourcepathcfg.CleanupsizeMB)
			break
		}
		logger.Log.GlobalLogger.Warn("Structure not allowed ", zap.Stringp("path", &folder))
		continue
	}

	err = structurevar.filterVideoFiles(allfiles, removesmallfiles)
	if err != nil {
		logger.Log.GlobalLogger.Debug("Folder skipped due to no video files found ", zap.Stringp("path", &folder))
		//skip files
		return
	}

	if len(allfiles.Arr) == 0 {
		//skip mo  files
		return
	}
	if structurevar.groupType == "movie" && len(allfiles.Arr) >= 2 {
		logger.Log.GlobalLogger.Warn("Folder skipped due to too many video files ", zap.Stringp("path", &folder))
		//skip too many  files
		return
	}

	checkruntime := structurevar.sourcepathcfg.CheckRuntime
	if inConfig.Disableruntimecheck {
		checkruntime = false
	}
	deletewronglanguage := structurevar.targetpathcfg.DeleteWrongLanguage
	if inConfig.Disabledeletewronglanguage {
		deletewronglanguage = false
	}

	for fileidx := range allfiles.Arr {
		structurevar.organizefileinfolder(allfiles.Arr[fileidx], deletewronglanguage, checkruntime)
	}
}

func (s *organizer) organizefileinfolder(path string, deletewronglanguage bool, checkruntime bool) {
	templateQuality := ""
	if filepath.Ext(path) == "" {
		return
	}
	if logger.ContainsIa(path, "_unpack") {
		logger.Log.GlobalLogger.Warn("Unpacking - skipping", zap.Stringp("path", &path))
		return
	}
	if s.groupType == "series" && s.removeSmallVideoFile(path) {
		logger.Log.GlobalLogger.Debug("Folder skipped due to small video files - file was removed ", zap.Stringp("path", &path))
		return
	}

	m := s.ParseFile(path, true, s.rootpath, deletewronglanguage)
	if m == nil {
		logger.Log.GlobalLogger.Debug("Parse failed", zap.Stringp("path", &path))
		return
	}
	defer m.Close()
	parser.GetDbIDs(s.groupType, m, s.cfgp, "", true)
	if m.Listname != "" {
		templateQuality = s.cfgp.ListsMap[m.Listname].TemplateQuality
	}
	if templateQuality == "" {
		//logger.Log.GlobalLogger.Error("Structure quality missing ", zap.String("path", path), zap.String("List",m.Listname), zap.Uint("ID",m.DbmovieID), zap.String("Title",m.Title))
		return
	}
	if !config.Check("quality_" + templateQuality) {
		logger.Log.GlobalLogger.Error(strQualityForList + m.Listname + " not found - for: " + path)
		return
	}
	if s.listConfig != m.Listname || s.listcfg == nil {
		s.listConfig = m.Listname
		listcfg := s.cfgp.ListsMap[m.Listname]
		s.listcfg = &listcfg
	}
	wantedalt := []string{}
	var titleyear string
	if s.groupType == "movie" && m.MovieID != 0 && m.DbmovieID != 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: "select title from dbmovies where id = ?", Args: []interface{}{m.DbmovieID}}, &titleyear)
		database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: "select title from dbmovie_titles where dbmovie_id = ?", Args: []interface{}{m.DbmovieID}}, &wantedalt)
		searchnzb := apiexternal.Nzbwithprio{WantedTitle: titleyear, WantedAlternates: wantedalt, QualityTemplate: templateQuality, ParseInfo: *m}
		if searcher.Checktitle(&searchnzb, "movie", nil) {
			logger.Log.GlobalLogger.Warn("Skipped - unwanted title", zap.Stringp("title", &m.Title), zap.Stringp("want title", &titleyear))
			searchnzb.Close()
			wantedalt = nil
			return
		}
		searchnzb.Close()
		s.organizeMovie(s.rootpath, m.MovieID, path, deletewronglanguage, checkruntime, m)

	} else if s.groupType == "series" && m.DbserieEpisodeID != 0 && m.DbserieID != 0 && m.SerieEpisodeID != 0 && m.SerieID != 0 {
		database.QueryColumn(&database.Querywithargs{QueryString: "select seriename from dbseries where id = ?", Args: []interface{}{m.DbserieID}}, &titleyear)
		database.QueryStaticStringArray(false, 0, &database.Querywithargs{QueryString: "select title from dbserie_alternates where dbserie_id = ?", Args: []interface{}{m.DbserieID}}, &wantedalt)
		searchnzb := apiexternal.Nzbwithprio{WantedTitle: titleyear, WantedAlternates: wantedalt, QualityTemplate: templateQuality, ParseInfo: *m}
		if searcher.Checktitle(&searchnzb, "series", nil) {
			logger.Log.GlobalLogger.Warn("Skipped - unwanted title", zap.Stringp("title", &m.Title), zap.Stringp("want title", &titleyear))
			searchnzb.Close()
			wantedalt = nil
			return
		}
		searchnzb.Close()
		s.organizeSeries(s.rootpath, m.SerieID, path, deletewronglanguage, checkruntime, m)
	} else {
		logger.Log.GlobalLogger.Debug("File not matched", zap.Stringp("path", &path))
	}
	wantedalt = nil
}

func OrganizeFolders(grouptype string, sourcepathstr string, targetpathstr string, cfgp *config.MediaTypeConfig) {
	if !cfgp.Structure {
		//logger.Log.GlobalLogger.Debug("Structure disabled", zap.String("Job", jobName))
		return
	}

	jobName := sourcepathstr
	if structureJobRunning == jobName {
		logger.Log.GlobalLogger.Debug("Job already running", zap.String("Job", jobName))
		return
	}
	structureJobRunning = jobName

	folders, err := scanner.GetSubFolders(config.Cfg.Paths[sourcepathstr].Path)
	if err == nil {
		for idx := range folders {
			OrganizeSingleFolder(folders[idx], cfgp, &Config{Grouptype: grouptype, Sourcepathstr: sourcepathstr, Targetpathstr: targetpathstr})
		}
	}
	folders = nil
}

func (s *parsertype) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.Dbmovie = nil
	s.Dbserie = nil
	s.DbserieEpisode = nil
	s.Source.Close()
	s.Episodes = nil
	s = nil
}
func (s *inputNotifier) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.Replaced = nil
	s.Dbmovie = nil
	s.Dbserie = nil
	s.DbserieEpisode = nil
	s.Source.Close()
	s = nil
}

func (s *forstructurenotify) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.InputNotifier.close()
	s = nil
}

package structure

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/importfeed"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/Kellerman81/go_media_downloader/parser"
	"github.com/Kellerman81/go_media_downloader/scanner"
	"github.com/Kellerman81/go_media_downloader/searcher"
	"github.com/mozillazg/go-unidecode"
)

type Organizer struct {
	cfgpstr         string
	listConfig      string
	templateQuality string
	groupType       string //series, movies
	rootpath        string //1st level below input
	sourcepath      string
	targetpath      string
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
	Config        *Organizer
	InputNotifier inputNotifier
}

func NewStructure(cfgpstr string, listname string, groupType string, rootpath string, sourcepathstr string, targetpathstr string) (*Organizer, error) {

	if len(cfgpstr) == 0 {
		return nil, logger.ErrNotAllowed
	}
	if !config.SettingsMedia[cfgpstr].Structure {
		return nil, logger.ErrNotAllowed
	}
	var templatequality string
	if listname != "" {
		id := config.GetMediaListsEntryIndex(cfgpstr, listname)
		if id != -1 {
			templatequality = config.SettingsMedia[cfgpstr].Lists[id].TemplateQuality
		}
	}
	return &Organizer{
		cfgpstr:         cfgpstr,
		listConfig:      listname,
		templateQuality: templatequality,
		groupType:       groupType,
		rootpath:        rootpath,
		sourcepath:      sourcepathstr,
		targetpath:      targetpathstr,
	}, nil
}
func (s *Organizer) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.ClearVar(s)
}

func (s *Organizer) fileCleanup(folder string, videofile *string) error {
	if s.groupType == logger.StrMovie || *videofile == "" {
		if *videofile != "" {
			ok, err := scanner.RemoveFile(*videofile)
			if err == nil && ok {

				if config.SettingsPath["path_"+s.sourcepath].Name == "" {
					return errors.New("pathtemplate not found")
				}

				if !scanner.CheckFileExist(&folder) {
					return logger.ErrNotFound
				}
				// filepath.WalkDir(folder, func(elem string, info fs.DirEntry, errwalk error) error {
				// 	if errwalk != nil {
				// 		return errwalk
				// 	}
				// 	if info.IsDir() {
				// 		return nil
				// 	}
				// 	if scanner.Filterfile(&elem, false, s.sourcepath) {
				// 		scanner.RemoveFile(elem)
				// 	}
				// 	return nil
				// })
				scanner.WalkdirProcess(folder, true, func(elem *string, _ *fs.DirEntry) error {
					if scanner.Filterfile(elem, false, s.sourcepath) {
						scanner.RemoveFile(*elem)
					}
					return nil
				})
			} else {
				return err
			}

			// for _, removefile := range scanner.GetFilesDir(folder, s.sourcepath, false) {
			// 	_,_ = scanner.RemoveFile(removefile)
			// }
		}
		return scanner.CleanUpFolder(folder, config.SettingsPath["path_"+s.sourcepath].CleanupsizeMB)
	}
	fileext := filepath.Ext(*videofile)
	ok, err := scanner.RemoveFile(*videofile)
	if err == nil && ok {
		for idx := range config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions {
			scanner.RemoveFile(strings.ReplaceAll(*videofile, fileext, config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions[idx]))
		}
		return scanner.CleanUpFolder(folder, config.SettingsPath["path_"+s.sourcepath].CleanupsizeMB)
	}
	return nil
}
func getruntimedifference(wantedruntime int, haveruntime int) int {
	if wantedruntime > haveruntime {
		return wantedruntime - haveruntime
	}
	return haveruntime - wantedruntime
}
func (s *Organizer) ParseFileAdditional(videofile *string, folder string, deletewronglanguage bool, wantedruntime int, checkruntime bool, m *apiexternal.ParseInfo) error {
	if s.listConfig == "" {
		return errors.New("listconfig empty")
	}
	if !config.CheckGroup("quality_", s.templateQuality) {
		return errors.New("quality template not found")
	}

	parser.GetPriorityMapQual(m, s.cfgpstr, s.templateQuality, true, true)
	err := parser.ParseVideoFile(m, videofile, s.templateQuality)
	if err != nil {
		if err == errors.New("no tracks") {
			s.fileCleanup(folder, videofile)
		}
		return err
	}
	if m.Runtime >= 1 && checkruntime && wantedruntime != 0 && config.SettingsPath["path_"+s.targetpath].MaxRuntimeDifference != 0 && (m.Runtime/60) != wantedruntime {
		maxdifference := config.SettingsPath["path_"+s.targetpath].MaxRuntimeDifference
		if m.Extended && s.groupType == logger.StrMovie {
			maxdifference += 10
		}
		difference := getruntimedifference(wantedruntime, m.Runtime/60)

		if difference > maxdifference {
			if config.SettingsPath["path_"+s.targetpath].DeleteWrongRuntime {
				s.fileCleanup(folder, videofile)
			}
			return fmt.Errorf("wrong runtime - wanted %d have %d", wantedruntime, m.Runtime/60)
			//return errors.New("wrong runtime - wanted " + logger.IntToString(wantedruntime) + " have " + logger.IntToString(m.Runtime/60))
		}
	}
	if len(config.SettingsPath["path_"+s.targetpath].AllowedLanguages) == 0 || !deletewronglanguage {
		return nil
	}
	var languageOk bool
	lenlang := len(m.Languages)
	for idx := range config.SettingsPath["path_"+s.targetpath].AllowedLanguages {
		if lenlang == 0 && config.SettingsPath["path_"+s.targetpath].AllowedLanguages[idx] == "" {
			languageOk = true
			break
		}
		for idxlang := range m.Languages {
			if strings.EqualFold(config.SettingsPath["path_"+s.targetpath].AllowedLanguages[idx], m.Languages[idxlang]) {
				languageOk = true
				break
			}
		}
		if languageOk {
			break
		}
	}
	if !languageOk {
		if deletewronglanguage {
			err = s.fileCleanup(folder, videofile)
			if err != nil {
				return errors.New("wrong language - have " + m.Languages[0] + " " + err.Error())
			}
		}
		return errors.New("wrong language - have " + m.Languages[0])
	}
	return nil
}

func (s *Organizer) checkLowerQualTarget(folder string, videofile *string, cleanuplowerquality bool, movieid uint, m *apiexternal.ParseInfo) (*[]string, error) {
	if s.listConfig == "" {
		return nil, errors.New("listconfig empty")
	}

	if !config.CheckGroup("quality_", s.templateQuality) {
		return nil, errors.New("quality template not found")
	}

	moviecount := database.QueryIntColumn("select count() from movie_files where movie_id = ?", movieid)
	var oldpriority int
	if moviecount >= 1 {
		oldpriority = searcher.GetHighestMoviePriorityByFiles(true, true, movieid, s.templateQuality)
	}

	var err error
	if moviecount >= 1 && oldpriority != 0 && oldpriority >= m.Priority {
		if cleanuplowerquality {
			err = s.fileCleanup(folder, videofile)
			if err != nil {
				return nil, errors.New(logger.ErrLowerQuality.Error() + " " + err.Error())
			}
		}
		return nil, logger.ErrLowerQuality
	}
	if moviecount == 0 {
		return nil, nil
	}

	//var lastprocessed string
	tbl := database.QueryStaticColumnsOneStringOneInt(false, 1, "select location, id from movie_files where movie_id = ?", movieid)
	if tbl == nil || len(*tbl) == 0 {
		return nil, nil
	}
	oldfiles := make([]string, 0, len(*tbl))
	var entryPrio int
	for idx := range *tbl {

		entryPrio = parser.Getdbidsfromfiles(true, true, uint((*tbl)[idx].Num), "Select count() from movie_files where id = ?", "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?", s.templateQuality)
		if entryPrio == 0 {
			entryPrio = parser.Getdbidsfromfiles(true, false, uint((*tbl)[idx].Num), "Select count() from movie_files where id = ?", "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from movie_files where id = ?", s.templateQuality)
		}

		if entryPrio != 0 && m.Priority > entryPrio && config.SettingsPath["path_"+s.targetpath].Upgrade {
			oldfiles = append(oldfiles, (*tbl)[idx].Str)
		}
	}
	logger.Clear(tbl)
	return &oldfiles, nil
}

func (s *Organizer) GenerateNamingTemplate(videofile *string, rootpath string, dbid uint, serietitle string, episodetitle string, mapepi *[]database.DbstaticTwoUint, m *apiexternal.ParseInfo) (string, string) {
	forparser := parsertype{Source: m}
	defer forparser.Close()
	if s.groupType == logger.StrMovie {
		return s.generatenamingmovie(videofile, &forparser, rootpath, dbid, m)
	}
	return s.generatenamingserie(rootpath, &forparser, dbid, m, serietitle, episodetitle, mapepi)
	//epi, err := database.GetSerieEpisodes(database.Querywithargs{Select: "dbserie_id, dbserie_episode_id, serie_id", Where: logger.FilterByID, Arg: dbid})

}

func (s *Organizer) generatenamingmovie(videofile *string, forparser *parsertype, rootpath string, dbid uint, m *apiexternal.ParseInfo) (string, string) {
	var err error
	forparser.Dbmovie, err = database.GetDbmovie(logger.FilterByID, dbid)
	if err != nil {
		return "", ""
	}
	forparser.TitleSource = filepath.Base(*videofile)
	logger.TrimStringInclAfterString(&forparser.TitleSource, m.Quality)
	logger.TrimStringInclAfterString(&forparser.TitleSource, m.Resolution)
	if m.Year != 0 {
		logger.TrimStringInclAfterString(&forparser.TitleSource, logger.IntToString(m.Year))
	}
	forparser.TitleSource = strings.Trim(forparser.TitleSource, ".")
	logger.StringReplaceRuneP(&forparser.TitleSource, '.', " ")
	logger.StringDeleteRuneP(&forparser.TitleSource, '/')

	if forparser.Dbmovie.Title == "" {
		forparser.Dbmovie.Title = database.QueryStringColumn(database.QueryDbmovieTitlesGetTitleByIDLmit1, dbid)
		if forparser.Dbmovie.Title == "" {
			forparser.Dbmovie.Title = forparser.TitleSource
		}
	}
	logger.StringDeleteRuneP(&forparser.Dbmovie.Title, '/')
	if forparser.Dbmovie.Year == 0 {
		forparser.Dbmovie.Year = m.Year
	}

	_, splitby := checksplit(config.SettingsMedia[s.cfgpstr].Naming)
	foldername, filename := logger.SplitByLR(config.SettingsMedia[s.cfgpstr].Naming, splitby)

	//foldername, filename := filepath.Split(config.SettingsMedia[s.cfgpstr].Naming)
	if rootpath != "" {
		_, splitbyroot := checksplit(rootpath)
		_, getfoldername := logger.SplitByLR(rootpath, splitbyroot)
		if getfoldername != "" {
			foldername = "" //getfoldername
		}
	}

	if len(forparser.Source.Imdb) == 0 {
		forparser.Source.Imdb = forparser.Dbmovie.ImdbID
	}
	if forparser.Source.Imdb != "" {
		forparser.Source.Imdb = logger.AddImdbPrefix(forparser.Source.Imdb)
	}

	logger.StringDeleteRuneP(&forparser.Source.Title, '/')

	foldername, err = logger.ParseStringTemplate(foldername, forparser)
	if err != nil {
		return "", ""
	}
	filename, err = logger.ParseStringTemplate(filename, forparser)
	if err != nil {
		return "", ""
	}
	foldername = strings.Trim(foldername, ".")
	logger.StringReplaceDiacritics(&foldername)
	logger.Path(&foldername, true)
	foldername = unidecode.Unidecode(foldername)

	filename = strings.ReplaceAll(strings.Trim(filename, "."), "  ", " ")
	filename = strings.ReplaceAll(filename, " ]", "]")
	filename = strings.ReplaceAll(filename, "[ ", "[")
	filename = strings.ReplaceAll(filename, "[ ]", "")
	filename = strings.ReplaceAll(filename, "( )", "")
	filename = strings.ReplaceAll(filename, "[]", "")
	filename = strings.ReplaceAll(filename, "()", "")
	filename = strings.ReplaceAll(filename, "  ", " ")
	logger.StringReplaceDiacritics(&filename)
	logger.Path(&filename, true)
	return foldername, unidecode.Unidecode(filename)
}

func (s *Organizer) generatenamingserie(rootpath string, forparser *parsertype, dbid uint, m *apiexternal.ParseInfo, serietitle string, episodetitle string, mapepi *[]database.DbstaticTwoUint) (string, string) {
	var err error

	dbmainid := database.QueryUintColumn(database.QuerySerieEpisodesGetDBSerieIDByID, dbid)
	dbepisodeid := database.QueryUintColumn(database.QuerySerieEpisodesGetDBSerieEpisodeIDByID, dbid)
	if dbepisodeid == 0 || dbmainid == 0 {
		return "", ""
	}
	forparser.Dbserie, err = database.GetDbserieByID(dbmainid)
	if err != nil {
		return "", ""
	}
	forparser.DbserieEpisode, err = database.GetDbserieEpisodesByID(dbepisodeid)
	if err != nil {
		return "", ""
	}
	_, splitby := checksplit(config.SettingsMedia[s.cfgpstr].Naming)
	foldername, filename := logger.SplitByLR(config.SettingsMedia[s.cfgpstr].Naming, splitby)
	//foldername, filename := filepath.Split(config.SettingsMedia[s.cfgpstr].Naming)

	if forparser.Dbserie.Seriename == "" {
		forparser.Dbserie.Seriename = database.QueryStringColumn(database.QueryDbserieAlternatesGetTitleByDBID, dbmainid)
		if forparser.Dbserie.Seriename == "" {
			forparser.Dbserie.Seriename = serietitle
		}
	}
	logger.StringDeleteRuneP(&forparser.Dbserie.Seriename, '/')
	if forparser.DbserieEpisode.Title == "" {
		forparser.DbserieEpisode.Title = episodetitle
	}
	logger.StringDeleteRuneP(&forparser.DbserieEpisode.Title, '/')
	if rootpath != "" {
		_, splitbyroot := checksplit(rootpath)
		_, getfoldername := logger.SplitByLR(rootpath, splitbyroot)
		if getfoldername != "" {
			_, splitbyget := checksplit(foldername)
			if splitbyget != ' ' {
				_, seasonname := logger.SplitByLR(foldername, splitbyget)
				foldername = seasonname //getfoldername + string(splitbyget) +
			} else {
				foldername = getfoldername
			}
		}
	}

	forparser.Episodes = make([]int, len(*mapepi))
	for key := range *mapepi {
		forparser.Episodes[key] = logger.StringToInt(database.QueryStringColumn(database.QueryDbserieEpisodesGetEpisodeByID, (*mapepi)[key].Num2))
	}
	forparser.TitleSource = serietitle
	logger.StringDeleteRuneP(&forparser.TitleSource, '/')
	forparser.EpisodeTitleSource = episodetitle
	logger.StringDeleteRuneP(&forparser.EpisodeTitleSource, '/')
	if len(forparser.Source.Tvdb) == 0 || forparser.Source.Tvdb == "0" || strings.EqualFold(forparser.Source.Tvdb, "tvdb") {
		forparser.Source.Tvdb = logger.IntToString(forparser.Dbserie.ThetvdbID)
	}
	if forparser.Source.Tvdb != "" {
		forparser.Source.Tvdb = logger.AddTvdbPrefix(forparser.Source.Tvdb)
	}

	foldername, err = logger.ParseStringTemplate(foldername, forparser)
	if err != nil {
		return "", ""
	}
	filename, err = logger.ParseStringTemplate(filename, forparser)
	if err != nil {
		return "", ""
	}
	foldername = strings.Trim(foldername, ".")
	logger.StringReplaceDiacritics(&foldername)
	logger.Path(&foldername, true)
	foldername = unidecode.Unidecode(foldername)

	filename = strings.ReplaceAll(strings.Trim(filename, "."), "  ", " ")
	filename = strings.ReplaceAll(filename, " ]", "]")
	filename = strings.ReplaceAll(filename, "[ ", "[")
	filename = strings.ReplaceAll(filename, "[ ]", "")
	filename = strings.ReplaceAll(filename, "( )", "")
	filename = strings.ReplaceAll(filename, "[]", "")
	filename = strings.ReplaceAll(filename, "()", "")
	filename = strings.ReplaceAll(filename, "  ", " ")
	logger.StringReplaceDiacritics(&filename)
	logger.Path(&filename, true)
	filename = unidecode.Unidecode(filename)
	return foldername, filename
}

func (s *Organizer) moveVideoFile(foldername string, filename string, videofile *string, rootpath string) (string, bool, int, error) {
	videotarget := logger.PathJoin(config.SettingsPath["path_"+s.targetpath].Path, foldername)
	if rootpath != "" {
		videotarget = logger.PathJoin(rootpath, foldername)
	}

	mode := fs.FileMode(uint32(0777))
	if config.SettingsPath["path_"+s.targetpath].SetChmod != "" && len(config.SettingsPath["path_"+s.targetpath].SetChmod) == 4 {
		mode = fs.FileMode(logger.StringToUint32(config.SettingsPath["path_"+s.targetpath].SetChmod))
	}
	err := os.MkdirAll(videotarget, mode)
	if err != nil {
		return videotarget, false, 0, errors.Join(err, errors.New("structure make dir"))
	}
	ok, err := scanner.MoveFile(*videofile, s.sourcepath, videotarget, filename, false, false, config.SettingsGeneral.UseFileBufferCopy, config.SettingsPath["path_"+s.targetpath].SetChmod)
	if err != nil {
		return videotarget, true, 1, errors.Join(err, errors.New("structure move file"))
	}
	if ok {
		return videotarget, true, 1, nil
	}
	return videotarget, false, 0, errors.Join(err, errors.New("structure move file"))
}

func (s *Organizer) moveRemoveOldMediaFile(oldfile *string, id uint, usebuffer bool, move bool) error {

	var ok bool
	var err error
	if move {
		ok, err = scanner.MoveFile(*oldfile, "", logger.PathJoin(config.SettingsPath["path_"+s.targetpath].MoveReplacedTargetPath, filepath.Base(filepath.Dir(*oldfile))), "", false, true, usebuffer, config.SettingsPath["path_"+s.targetpath].SetChmod)
		if err != nil {
			return err
		}
	} else {
		ok, err = scanner.RemoveFile(*oldfile)
		if err != nil {
			return err
		}
	}
	if !ok {
		return logger.ErrNotAllowed
	}
	if s.groupType == logger.StrMovie {
		if config.SettingsGeneral.UseMediaCache {
			ti := logger.IndexFunc(&database.CacheFilesMovie, func(elem string) bool { return elem == *oldfile })
			if ti != -1 {
				logger.Delete(&database.CacheFilesMovie, ti)
			}
		}
		//logger.DeleteFromStringsCache("movie_files_cached", oldfile)
		database.DeleteRowStatic(false, "Delete from movie_files where movie_id = ? and location = ?", id, &oldfile)
	} else {
		if config.SettingsGeneral.UseMediaCache {
			ti := logger.IndexFunc(&database.CacheFilesSeries, func(elem string) bool { return elem == *oldfile })
			if ti != -1 {
				logger.Delete(&database.CacheFilesSeries, ti)
			}
		}
		//logger.DeleteFromStringsCache("serie_episode_files_cached", oldfile)
		database.DeleteRowStatic(false, "Delete from serie_episode_files where serie_id = ? and location = ?", id, &oldfile)
	}
	fileext := filepath.Ext(*oldfile)
	var additionalfile string
	for idxext := range config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions {
		ok = false
		additionalfile = strings.ReplaceAll(*oldfile, fileext, config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions[idxext])
		if !scanner.CheckFileExist(&additionalfile) {
			continue
		}
		if move {
			ok, err = scanner.MoveFile(additionalfile, "", logger.PathJoin(config.SettingsPath["path_"+s.targetpath].MoveReplacedTargetPath, filepath.Base(filepath.Dir(*oldfile))), "", false, true, usebuffer, config.SettingsPath["path_"+s.targetpath].SetChmod)
			if err != nil && err != logger.ErrNotFound {
				logfileerror(err, &additionalfile, "file could not be moved")
				continue
			}
		} else {
			ok, err = scanner.RemoveFile(additionalfile)
			if err != nil {
				logger.Logerror(err, "Delete Files")
				continue
			}
		}
		if ok {
			logger.Log.Debug().Str(logger.StrPath, additionalfile).Msg("Additional File removed")
		}
	}
	return nil
}

func (s *Organizer) organizeSeries(folder string, serieid uint, videofile *string, deletewronglanguage bool, checkruntime bool, m *apiexternal.FileParser) error {
	defer m.Close()
	var dbserieid uint
	var listname, rootpath string
	database.QuerySerieDataSearch(serieid, &dbserieid, &listname, &rootpath)

	//dbserieid := database.QueryUintColumn(database.QuerySeriesGetDBIDByID, serieid)
	if dbserieid == 0 {
		return logger.ErrNotFoundDbserie
	}
	identifiedby := database.QueryStringColumn(database.QueryDbseriesGetIdentifiedByID, dbserieid)
	runtimestr := database.QueryStringColumn(database.QueryDbseriesGetRuntimeByID, dbserieid)
	if runtimestr == "" && identifiedby != "date" {
		return logger.ErrRuntime
	}

	// listname := database.QueryStringColumn(database.QuerySeriesGetListnameByID, serieid)
	// if listname == "" {
	// 	return errors.New("listname empty")
	// }
	// rootpath := database.QueryStringColumn(database.QuerySeriesGetRootpathByID, serieid)
	if s.listConfig != listname || s.templateQuality == "" {
		s.listConfig = listname
		intid := -1
		for idxi := range config.SettingsMedia[s.cfgpstr].Lists {
			if strings.EqualFold(config.SettingsMedia[s.cfgpstr].Lists[idxi].Name, m.M.Listname) {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFuncS(config.SettingsMedia[s.cfgpstr].Lists, func(c config.MediaListsConfig) bool { return strings.EqualFold(c.Name, m.Listname) })
		if intid != -1 {
			s.templateQuality = config.SettingsMedia[s.cfgpstr].Lists[intid].TemplateQuality
		}
	}
	if !config.CheckGroup("quality_", s.templateQuality) {
		return errors.New("quality template not found")
	}

	oldfiles, allowimport, tblepi, err := s.GetSeriesEpisodes(serieid, dbserieid, videofile, folder, &m.M, false)
	if err != nil {
		return err
	}
	if tblepi == nil || len(*tblepi) == 0 {
		logger.Clear(oldfiles)
		return errors.New("episode match not found")
	}
	if !allowimport {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return logger.ErrNotAllowed
	}

	runtime := database.QueryIntColumn("select runtime from dbserie_episodes where id = ?", (*tblepi)[0].Num2)
	if runtime == 0 {
		runtime = logger.StringToInt(runtimestr)
	}

	season := database.QueryStringColumn(database.QueryDbserieEpisodesGetSeasonByID, (*tblepi)[0].Num2)
	if season == "" && identifiedby != "date" {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return errors.New("season not found")
	}

	ignoreRuntime := database.QueryBoolColumn("select ignore_runtime from serie_episodes where id = ?", (*tblepi)[0].Num1)
	if runtime == 0 && (season == "0" || season == "") {
		ignoreRuntime = true
	}
	var totalruntime int
	if !ignoreRuntime {
		totalruntime = runtime * len(*tblepi)
	}

	err = s.ParseFileAdditional(videofile, folder, deletewronglanguage, totalruntime, checkruntime, &m.M)
	if err != nil {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return err
	}

	serietitle, episodetitle := s.GetEpisodeTitle((*tblepi)[0].Num2, m.M.DbserieID, videofile, &m.M)

	foldername, filename := s.GenerateNamingTemplate(videofile, rootpath, (*tblepi)[0].Num1, serietitle, episodetitle, tblepi, &m.M)
	if filename == "" {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return errors.New("generating filename")
	}

	if config.SettingsPath["path_"+s.targetpath].MoveReplaced && oldfiles != nil && len(*oldfiles) >= 1 && config.SettingsPath["path_"+s.targetpath].MoveReplacedTargetPath != "" {
		//Move old files to replaced folder
		var err error
		for idx := range *oldfiles {
			err = s.moveRemoveOldMediaFile(&(*oldfiles)[idx], serieid, config.SettingsGeneral.UseFileBufferCopy, true)
			if err != nil {
				logger.Clear(oldfiles)
				logger.Clear(tblepi)
				return logfileerror(err, &(*oldfiles)[idx], "move old") // errors.Join(err, errors.New("move old file to replaced"))
			}
		}
	}

	if config.SettingsPath["path_"+s.targetpath].Usepresort && config.SettingsPath["path_"+s.targetpath].PresortFolderPath != "" {
		rootpath = logger.PathJoin(config.SettingsPath["path_"+s.targetpath].PresortFolderPath, foldername)
	}
	//Move new files to target folder
	videotarget, moveok, moved, err := s.moveVideoFile(foldername, filename, videofile, rootpath)
	if err != nil {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return err
	}
	if !moveok || moved == 0 {

		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return errors.New("move not ok")
	}
	//s.updateRootpath(videotarget, foldername, rootpath, serieid)
	//Remove old files from target folder
	err = s.moveandcleanup(videotarget, foldername, filename, rootpath, videofile, &m.M, listname, serieid, folder, (*tblepi)[0].Num2, oldfiles)
	if err != nil {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return err
	}
	//updateserie

	var reached bool

	if m.M.Priority >= parser.NewCutoffPrio(s.cfgpstr, s.templateQuality) {
		reached = true
	}
	targetfile := logger.PathJoin(videotarget, filename+filepath.Ext(*videofile))
	filebase := filepath.Base(targetfile)
	fileext := filepath.Ext(targetfile)

	if tblepi == nil || len(*tblepi) == 0 {
		logger.Clear(oldfiles)
		logger.Clear(tblepi)
		return errors.New("episodes not found")
	}
	if config.SettingsGeneral.UseMediaCache {
		database.CacheFilesSeries = append(database.CacheFilesSeries, *videofile)
	}
	//cache.Append(logger.GlobalCache, "serie_episode_files_cached", videofile)
	for key := range *tblepi {
		database.InsertStatic("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			targetfile, filebase, fileext, s.templateQuality, m.M.ResolutionID, m.M.QualityID, m.M.CodecID, m.M.AudioID, m.M.Proper, m.M.Repack, m.M.Extended, m.M.SerieID, (*tblepi)[key].Num1, (*tblepi)[key].Num2, m.M.DbserieID, m.M.Height, m.M.Width)

		database.UpdateColumnStatic("Update serie_episodes SET missing = ?, quality_reached = ? where id = ?", false, reached, (*tblepi)[key].Num1)

	}
	logger.Clear(oldfiles)
	logger.Clear(tblepi)
	return nil
}
func (s *Organizer) organizeMovie(folder string, movieid uint, videofile *string, deletewronglanguage bool, checkruntime bool, m *apiexternal.FileParser) error {
	defer m.Close()
	var dbmovieid uint
	var listname, rootpath string
	database.QueryMovieDataSearch(movieid, &dbmovieid, &listname, &rootpath)
	//dbmovieid := database.QueryUintColumn(database.QueryMoviesGetDBIDByID, movieid)
	if dbmovieid == 0 {
		return logger.ErrNotFoundDbmovie
	}
	runtime := database.QueryUintColumn(database.QueryDbmoviesGetRuntimeByID, dbmovieid)
	if runtime == 0 {
		return logger.ErrRuntime
	}
	// listname := database.QueryStringColumn(database.QueryMoviesGetListnameByID, movieid)
	// if listname == "" {
	// 	return errors.New("listname empty")
	// }
	// rootpath := database.QueryStringColumn(database.QueryMoviesGetRootpathByID, movieid)
	if s.listConfig != listname || s.templateQuality == "" {
		s.listConfig = listname
		intid := -1
		for idxi := range config.SettingsMedia[s.cfgpstr].Lists {
			if strings.EqualFold(config.SettingsMedia[s.cfgpstr].Lists[idxi].Name, m.M.Listname) {
				intid = idxi
				break
			}
		}
		//intid := logger.IndexFuncS(config.SettingsMedia[s.cfgpstr].Lists, func(c config.MediaListsConfig) bool { return strings.EqualFold(c.Name, m.Listname) })
		if intid != -1 {
			s.templateQuality = config.SettingsMedia[s.cfgpstr].Lists[intid].TemplateQuality
		}
	}
	if !config.CheckGroup("quality_", s.templateQuality) {
		return errors.New("quality template not found")
	}
	err := s.ParseFileAdditional(videofile, folder, deletewronglanguage, int(runtime), checkruntime, &m.M)
	if err != nil {
		return errors.Join(err, errors.New("structure parse additional"))
	}
	oldfiles, err := s.checkLowerQualTarget(folder, videofile, true, movieid, &m.M)
	if err != nil {
		return errors.Join(err, errors.New("structure lower qual check"))
	}
	foldername, filename := s.GenerateNamingTemplate(videofile, rootpath, dbmovieid, "", "", nil, &m.M)
	if filename == "" {
		logger.Clear(oldfiles)
		return errors.New("generating filename")
	}

	if config.SettingsPath["path_"+s.targetpath].MoveReplaced && oldfiles != nil && len(*oldfiles) >= 1 && config.SettingsPath["path_"+s.targetpath].MoveReplacedTargetPath != "" {
		//Move old files to replaced folder
		var err error
		for idx := range *oldfiles {
			err = s.moveRemoveOldMediaFile(&(*oldfiles)[idx], movieid, config.SettingsGeneral.UseFileBufferCopy, true)
			if err != nil {
				logger.Clear(oldfiles)
				return errors.Join(err, errors.New("move old file to replaced"))
			}
		}
	}

	if config.SettingsPath["path_"+s.targetpath].Usepresort && config.SettingsPath["path_"+s.targetpath].PresortFolderPath != "" {
		rootpath = logger.PathJoin(config.SettingsPath["path_"+s.targetpath].PresortFolderPath, foldername)
	}
	//Move new files to target folder
	videotarget, moveok, moved, err := s.moveVideoFile(foldername, filename, videofile, rootpath)
	if err != nil {
		logger.Clear(oldfiles)
		return errors.Join(err, errors.New("structure move video"))
	}
	if !moveok || moved == 0 {
		logger.Clear(oldfiles)
		return errors.New("unknown reason")
	}
	//s.updateRootpath(videotarget, foldername, rootpath, movieid)

	//Remove old files from target folder
	err = s.moveandcleanup(videotarget, foldername, filename, rootpath, videofile, &m.M, listname, movieid, folder, dbmovieid, oldfiles)
	if err != nil {
		logger.Clear(oldfiles)
		return errors.Join(err, errors.New("structure move cleanup"))
	}

	if config.SettingsGeneral.UseMediaCache {
		database.CacheFilesMovie = append(database.CacheFilesMovie, *videofile)
	}
	//cache.Append(logger.GlobalCache, "movie_files_cached", videofile)
	//updatemovie
	targetfile := logger.PathJoin(videotarget, filename+filepath.Ext(*videofile))
	database.InsertStatic("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		targetfile, filepath.Base(targetfile), filepath.Ext(targetfile), s.templateQuality, m.M.ResolutionID, m.M.QualityID, m.M.CodecID, m.M.AudioID, m.M.Proper, m.M.Repack, m.M.Extended, movieid, dbmovieid, m.M.Height, m.M.Width)

	database.UpdateColumnStatic("Update movies SET missing = ?, quality_reached = ? where id = ?", false, m.M.Priority >= parser.NewCutoffPrio(s.cfgpstr, s.templateQuality), movieid)
	logger.Clear(oldfiles)
	return nil
}

func logfileerror(err error, file *string, msg string) error {
	logger.Log.Error().Err(err).CallerSkipFrame(1).Str(logger.StrFile, *file).Msg(msg)
	return err
}

func checksplit(foldername string) (bool, rune) {
	if strings.ContainsRune(foldername, '/') {
		return true, '/'
	} else if strings.ContainsRune(foldername, '\\') {
		return true, '\\'
	}
	return false, ' '
}

func (s *Organizer) moveandcleanup(videotarget string, foldername string, filename string, rootpath string, videofile *string, m *apiexternal.ParseInfo, listname string, id uint, folder string, dbid uint, oldfiles *[]string) error {

	//Update Rootpath
	if rootpath == "" {
		if !config.SettingsPath["path_"+s.targetpath].Usepresort {
			dosplit, splitby := checksplit(foldername)
			if dosplit {
				var split string
				idx := strings.IndexRune(foldername, splitby)
				if idx != -1 {
					split = foldername[idx+1:]

					split = logger.SplitBy(split, splitby)
				} else {
					split = foldername
				}
				rootpath = strings.TrimRight(logger.TrimStringInclAfterStringB(videotarget, strings.TrimRight(split, string(splitby))), string(splitby))

				if config.SettingsPath["path_"+s.targetpath].Path != rootpath {
					if s.groupType == logger.StrMovie {
						database.UpdateColumnStatic("Update movies set rootpath = ? where id = ?", rootpath, id)
					} else if s.groupType == logger.StrSeries {
						database.UpdateColumnStatic("Update series set rootpath = ? where id = ?", rootpath, id)
					}
				}
			}
		}
	}
	//Update Rootpath end

	if config.SettingsPath["path_"+s.targetpath].Replacelower && oldfiles != nil && len(*oldfiles) >= 1 {
		var oldfilename string
		var err error
		for idx := range *oldfiles {
			_, oldfilename = filepath.Split((*oldfiles)[idx])
			if !strings.HasPrefix((*oldfiles)[idx], videotarget) || oldfilename != filename {
				err = s.moveRemoveOldMediaFile(&(*oldfiles)[idx], id, config.SettingsGeneral.UseFileBufferCopy, false)
				if err != nil {
					//Continue if old cannot be moved
					logfileerror(err, &(*oldfiles)[idx], "Move old")
				}
			}
		}
	}

	if s.groupType == logger.StrMovie {
		//move other movie

		if config.SettingsPath["path_"+s.sourcepath].Name == "" {
			return errors.New("pathtemplate not found")
		}

		if !scanner.CheckFileExist(&folder) {
			return logger.ErrNotFound
		}
		// filepath.WalkDir(rootpath, func(elem string, info fs.DirEntry, errwalk error) error {
		// 	if errwalk != nil {
		// 		return errwalk
		// 	}
		// 	if info.IsDir() {
		// 		return nil
		// 	}
		// 	if scanner.Filterfile(&elem, true, s.sourcepath) {
		// 		_, errmov := scanner.MoveFile(elem, s.sourcepath, videotarget, filename, true, false, config.SettingsGeneral.UseFileBufferCopy, config.SettingsPath["path_"+s.targetpath].SetChmod)
		// 		if errmov != nil && errmov != logger.ErrNotFound {
		// 			logfileerror(errmov, &elem, "file move")
		// 		}
		// 	}
		// 	return nil
		// })
		scanner.WalkdirProcess(rootpath, true, func(elem *string, _ *fs.DirEntry) error {
			if scanner.Filterfile(elem, true, s.sourcepath) {
				_, errmov := scanner.MoveFile(*elem, s.sourcepath, videotarget, filename, true, false, config.SettingsGeneral.UseFileBufferCopy, config.SettingsPath["path_"+s.targetpath].SetChmod)
				if errmov != nil && errmov != logger.ErrNotFound {
					logfileerror(errmov, elem, "file move")
				}
			}
			return nil
		})
		s.notify(videotarget, filename, videofile, dbid, listname, oldfiles, m)
		scanner.CleanUpFolder(folder, config.SettingsPath["path_"+s.sourcepath].CleanupsizeMB)
		return nil
	}
	//move other serie
	var also string
	var err error
	for idx := range config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions {
		also = strings.ReplaceAll(*videofile, filepath.Ext(*videofile), config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions[idx])
		//if scanner.CheckFileExist(also) {
		_, err = scanner.MoveFile(also, s.sourcepath, videotarget, filename, true, false, config.SettingsGeneral.UseFileBufferCopy, config.SettingsPath["path_"+s.targetpath].SetChmod)
		if err != nil && err != logger.ErrNotFound {
			logfileerror(err, &also, "file move")
		}
	}

	s.notify(videotarget, filename, videofile, dbid, listname, oldfiles, m)
	scanner.CleanUpFolder(folder, config.SettingsPath["path_"+s.sourcepath].CleanupsizeMB)
	return nil
}

func (s *Organizer) GetEpisodeTitle(firstdbepiid uint, dbserieid uint, videofile *string, m *apiexternal.ParseInfo) (string, string) {
	episodetitle := database.QueryStringColumn(database.QueryDbserieEpisodesGetTitleByID, firstdbepiid)
	serietitle := database.QueryStringColumn("select seriename from dbseries where id = ?", dbserieid)
	if serietitle != "" && episodetitle != "" {
		return serietitle, episodetitle
	}
	if m.Identifier != "" {
		regidentifier := `^(.*)(?i)` + m.Identifier + `(?:\.| |-)(.*)$`
		basepath := filepath.Base(*videofile)
		serietitleparse, episodetitleparse := config.RegexGetMatchesStr1Str2(false, &regidentifier, &basepath)
		if serietitle != "" && episodetitleparse != "" {
			logger.StringReplaceRuneP(&episodetitleparse, '.', " ")
			logger.TrimStringInclAfterString(&episodetitleparse, "XXX")
			logger.TrimStringInclAfterString(&episodetitleparse, m.Quality)
			logger.TrimStringInclAfterString(&episodetitleparse, m.Resolution)
			episodetitleparse = strings.Trim(episodetitleparse, ". ")

			serietitleparse = strings.Trim(serietitleparse, ".")
			logger.StringReplaceRuneP(&serietitleparse, '.', " ")
		}

		if episodetitle == "" {
			episodetitle = episodetitleparse
		}
		if serietitle == "" {
			serietitle = serietitleparse
		}
	}
	return serietitle, episodetitle
}
func (s *Organizer) notify(videotarget string, filename string, videofile *string, id uint, listname string, oldfiles *[]string, m *apiexternal.ParseInfo) {
	var err error
	if oldfiles == nil {
		oldfiles = &[]string{}
	}
	notify := forstructurenotify{Config: s, InputNotifier: inputNotifier{
		Targetpath:    logger.PathJoin(videotarget, filename),
		SourcePath:    *videofile,
		Replaced:      *oldfiles,
		Configuration: listname,
		Source:        m,
		Time:          logger.TimeGetNow().Format(logger.TimeFormat),
	}}
	defer notify.close()
	if s.groupType == logger.StrMovie {
		notify.InputNotifier.Dbmovie, err = database.GetDbmovie(logger.FilterByID, id)
		if err != nil {
			return
		}
		notify.InputNotifier.Title = notify.InputNotifier.Dbmovie.Title
		notify.InputNotifier.Year = logger.IntToString(notify.InputNotifier.Dbmovie.Year)
		notify.InputNotifier.Imdb = notify.InputNotifier.Dbmovie.ImdbID

	} else {
		notify.InputNotifier.DbserieEpisode, err = database.GetDbserieEpisodesByID(id)
		if err != nil {
			return
		}
		notify.InputNotifier.Dbserie, err = database.GetDbserieByID(notify.InputNotifier.DbserieEpisode.DbserieID)
		if err != nil {
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
	for idx := range config.SettingsMedia[s.cfgpstr].Notification {
		notify.InputNotifier.ReplacedPrefix = config.SettingsMedia[s.cfgpstr].Notification[idx].ReplacedPrefix

		if !strings.EqualFold(config.SettingsMedia[s.cfgpstr].Notification[idx].Event, "added_data") {
			continue
		}
		if !config.CheckGroup("notification_", config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification) {
			continue
		}
		messagetext, err = logger.ParseStringTemplate(config.SettingsMedia[s.cfgpstr].Notification[idx].Message, notify)
		if err != nil {
			continue
		}
		messageTitle, err = logger.ParseStringTemplate(config.SettingsMedia[s.cfgpstr].Notification[idx].Title, notify)
		if err != nil {
			continue
		}

		switch config.SettingsNotification["notification_"+config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification].NotificationType {
		case "pushover":
			if apiexternal.PushoverAPI == nil {
				apiexternal.NewPushOverClient(config.SettingsNotification["notification_"+config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification].Apikey)
			}
			if apiexternal.PushoverAPI.APIKey != config.SettingsNotification["notification_"+config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification].Apikey {
				apiexternal.NewPushOverClient(config.SettingsNotification["notification_"+config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification].Apikey)
			}

			err = apiexternal.PushoverAPI.SendMessage(messagetext, messageTitle, config.SettingsNotification["notification_"+config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification].Recipient)
			if err != nil {
				logger.Logerror(err, "Error sending pushover")
			} else {
				logger.Log.Info().Msg("Pushover message sent")
			}
		case "csv":
			if scanner.AppendCsv(config.SettingsNotification["notification_"+config.SettingsMedia[s.cfgpstr].Notification[idx].MapNotification].Outputto, messagetext) != nil {
				continue
			}
		}
	}
}
func Getepisodestoimport(m *apiexternal.ParseInfo, serieid uint, dbserieid uint) (*[]database.DbstaticTwoUint, error) {
	identifiedby := database.QueryStringColumn(database.QueryDbseriesGetIdentifiedByID, dbserieid)
	if identifiedby == "" {
		identifiedby = "ep"
	}
	episodeArray, err := importfeed.GetEpisodeArray(identifiedby, &m.Identifier)
	if episodeArray == nil {
		return nil, err
	}
	if len(episodeArray) == 0 {
		return nil, errors.New("no episode array found")
	}
	if len(episodeArray) == 1 && m.DbserieEpisodeID != 0 && m.SerieEpisodeID != 0 {
		logger.Clear(&episodeArray)
		return &[]database.DbstaticTwoUint{{Num1: m.SerieEpisodeID, Num2: m.DbserieEpisodeID}}, nil
	}

	episodestoimport := make([]database.DbstaticTwoUint, 0, len(episodeArray))
	var dbserieepisodeid, serieepisodeid uint
	for idx := range episodeArray {
		episodeArray[idx] = strings.Trim(episodeArray[idx], "-EX")
		if identifiedby != logger.StrDate {
			if episodeArray[idx] != "" && episodeArray[idx][:1] == "0" {
				episodeArray[idx] = strings.TrimLeft(episodeArray[idx], "0")
			}
			if episodeArray[idx] == "" {
				continue
			}
		}

		dbserieepisodeid = importfeed.FindDbserieEpisodeByIdentifierOrSeasonEpisode(dbserieid, &m.Identifier, m.SeasonStr, episodeArray[idx])

		if dbserieepisodeid != 0 {
			serieepisodeid = database.QueryUintColumn("select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", dbserieepisodeid, serieid)
			if serieepisodeid != 0 {
				episodestoimport = append(episodestoimport, database.DbstaticTwoUint{Num1: serieepisodeid, Num2: dbserieepisodeid})
			}
		}
	}
	logger.Clear(&episodeArray)
	if len(episodestoimport) == 0 {
		logger.Clear(&episodestoimport)
		return nil, errors.New("episode not matched")
	}
	return &episodestoimport, nil
}

func (s *Organizer) GetSeriesEpisodes(serieid uint, dbserieid uint, videofile *string, folder string, m *apiexternal.ParseInfo, skipdelete bool) (*[]string, bool, *[]database.DbstaticTwoUint, error) { //, []int, []database.SerieEpisode, , string, string, int

	episodestoimport, err := Getepisodestoimport(m, serieid, dbserieid)
	if err != nil || episodestoimport == nil {
		return nil, false, episodestoimport, err
	}
	if len(*episodestoimport) == 0 {
		return nil, false, episodestoimport, errors.New("no episodes found")
	}
	parser.GetPriorityMapQual(m, s.cfgpstr, s.templateQuality, true, true)

	//oldfiles := make([]string, 0, len(episodestoimport))
	oldfiles := make([]string, 0, len(*episodestoimport))

	//var episodefiles []uint
	var allowimport bool

	var oldPrio, entryPrio int
	var tblepifiles *[]uint
	for _, episoderow := range *episodestoimport {
		oldPrio = searcher.GetHighestEpisodePriorityByFiles(true, true, episoderow.Num1, s.templateQuality)
		if m.Priority > oldPrio || oldPrio == 0 {
			tblepifiles = database.QueryStaticUintArrayNoError(true, 5, database.QuerySerieEpisodeFilesGetIDByEpisodeID, episoderow.Num1)
			for idxepifile := range *tblepifiles {
				entryPrio = parser.Getdbidsfromfiles(true, true, uint((*tblepifiles)[idxepifile]), "Select count() from serie_episode_files where id = ?", "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?", s.templateQuality)
				if entryPrio == 0 {
					entryPrio = parser.Getdbidsfromfiles(true, false, uint((*tblepifiles)[idxepifile]), "Select count() from serie_episode_files where id = ?", "select resolution_id, quality_id, codec_id, audio_id, proper, extended, repack from serie_episode_files where id = ?", s.templateQuality)
				}

				if m.Priority > entryPrio {
					oldfiles = append(oldfiles, database.QueryStringColumn("select location from serie_episode_files where id = ?", (*tblepifiles)[idxepifile]))
				}
			}
			logger.Clear(tblepifiles)
			allowimport = true
		} else if database.QueryIntColumn("select count() from serie_episode_files where serie_episode_id = ?", episoderow.Num1) == 0 {
			allowimport = true
		} else {
			if !skipdelete {
				ok, err := scanner.RemoveFile(*videofile)
				if err == nil && ok {
					logger.Log.Debug().Str(logger.StrPath, *videofile).Int("old prio", oldPrio).Int(logger.StrPriority, m.Priority).Msg("Lower Qual Import File removed")
					for idxall := range config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions {
						//if scanner.CheckFileExist(also) {
						scanner.RemoveFile(strings.ReplaceAll(*videofile, filepath.Ext(*videofile), config.SettingsPath["path_"+s.sourcepath].AllowedOtherExtensions[idxall]))
						//}
					}
					scanner.CleanUpFolder(folder, config.SettingsPath["path_"+s.sourcepath].CleanupsizeMB)
					database.RemoveFromTwoUIntArrayStructV1V2(episodestoimport, episoderow.Num1, episoderow.Num2)
				} else {
					logger.Logerror(err, "Delete Files")
				}
			}
		}
	}
	return &oldfiles, allowimport, episodestoimport, nil //, episodes, seriesEpisodes, serietitle, episodetitle, runtime
}

func CheckDisallowed(folder *string, sourcepathstr string) bool {
	var disallowed bool
	scanner.WalkdirProcess(*folder, false, func(elem *string, _ *fs.DirEntry) error {
		if !disallowed {
			for idx := range config.SettingsPath["path_"+sourcepathstr].Disallowed {
				if logger.ContainsI(*elem, config.SettingsPath["path_"+sourcepathstr].Disallowed[idx]) {
					disallowed = true
					logfileerror(logger.ErrNotAllowed, elem, "file not allowed")
					return fs.SkipAll
				}
			}
		}
		return nil
	})
	return disallowed
}

func CheckUnmatched(grtype string, file *string) bool {
	if logger.HasPrefixI(grtype, logger.StrSerie) {
		if config.SettingsGeneral.UseMediaCache {
			if logger.IndexFunc(&database.CacheUnmatchedSeries, func(elem string) bool { return elem == *file }) != -1 {
				return true
			}
		} else {
			if database.QueryIntColumn("select count() from serie_file_unmatcheds where filepath = ?", file) >= 1 {
				return true
			}
		}
	} else {
		if config.SettingsGeneral.UseMediaCache {
			if logger.IndexFunc(&database.CacheUnmatchedMovie, func(elem string) bool { return elem == *file }) != -1 {
				return true
			}
		} else {
			if database.QueryIntColumn("select count() from movie_file_unmatcheds where filepath = ?", file) >= 1 {
				return true
			}
		}
	}
	return false
}

func CheckFiles(grtype string, file *string) bool {
	if logger.HasPrefixI(grtype, logger.StrSerie) {

		if config.SettingsGeneral.UseMediaCache {
			if logger.IndexFunc(&database.CacheFilesSeries, func(elem string) bool { return elem == *file }) == -1 {
				return true
			}
		} else {
			if database.QueryIntColumn("select count() from serie_episode_files where location = ?", file) >= 1 {
				return true
			}
		}
	} else {
		if config.SettingsGeneral.UseMediaCache {
			if logger.IndexFunc(&database.CacheFilesMovie, func(elem string) bool { return elem == *file }) == -1 {
				return true
			}
		} else {
			if database.QueryIntColumn("select count() from movie_files where location = ?", file) >= 1 {
				return true
			}
		}
	}
	return false
}

func OrganizeSingleFolder(folder string, disableruntimecheck bool,
	disabledeletewronglanguage bool,
	cachedunmatched string, structurevar *Organizer, id ...uint) {
	logger.Clear(&id)
	structurevar.rootpath = folder
	// structurevar, err := NewStructure(cfgpstr, "", grouptype, folder, sourcepathstr, targetpathstr)
	// if err != nil {
	// 	logfileerror(err, &folder, "structure failed")
	// 	return
	// }
	// defer structurevar.Close()
	if config.SettingsPath["path_"+structurevar.sourcepath].Name == "" {
		logfileerror(nil, &folder, "template "+structurevar.sourcepath+" not found")
		return
	}

	checkruntime := config.SettingsPath["path_"+structurevar.sourcepath].CheckRuntime
	if disableruntimecheck {
		checkruntime = false
	}
	deletewronglanguage := config.SettingsPath["path_"+structurevar.sourcepath].DeleteWrongLanguage
	if disabledeletewronglanguage {
		deletewronglanguage = false
	}

	var manualid uint
	if len(id) >= 1 {
		manualid = id[0]
	}

	scanner.WalkdirProcess(folder, true, func(file *string, info *fs.DirEntry) error {
		for idx := range config.SettingsPath["path_"+structurevar.sourcepath].Disallowed {
			if logger.ContainsI(*file, config.SettingsPath["path_"+structurevar.sourcepath].Disallowed[idx]) {
				return fs.SkipDir
			}
		}
		if filepath.Ext(*file) == "" {
			return nil
		}
		if !scanner.Filterfile(file, false, structurevar.sourcepath) {
			return nil
		}
		if logger.ContainsI(*file, "_unpack") {
			logfileerror(nil, file, "skipped - unpacking")
			return fs.SkipDir
		}
		if CheckUnmatched(structurevar.groupType, file) {
			return nil
		}

		if config.SettingsPath["path_"+structurevar.sourcepath].MinVideoSize > 0 {
			if scanner.GetFileSize(file) < config.SettingsPath["path_"+structurevar.sourcepath].MinVideoSizeByte {
				scanner.RemoveFiles(*file, structurevar.sourcepath)

				return logfileerror(logger.ErrNoFiles, file, "skipped - small files")
			}
		}

		if structurevar.groupType != logger.StrSeries {
			if scanner.GetFilesExtCount(file, filepath.Ext(*file)) >= 2 {
				logfileerror(logger.ErrNoFiles, file, "skipped - too many files")
				return nil
			}
			//continue
		}

		if CheckDisallowed(&folder, structurevar.sourcepath) {
			if config.SettingsPath["path_"+structurevar.sourcepath].DeleteDisallowed {
				structurevar.fileCleanup(folder, file)
			}
			return fs.SkipDir
		}

		m := parser.ParseFile(file, true, structurevar.groupType == logger.StrSeries, structurevar.groupType, true)

		if m.M.File == "" {
			m.Close()
			return logfileerror(nil, file, "parse failed")
		}
		err := parser.GetDBIDs(&m.M, structurevar.cfgpstr, "", true)
		if err != nil {
			if structurevar.groupType == logger.StrSeries {
				if config.SettingsGeneral.UseMediaCache {
					database.CacheUnmatchedSeries = append(database.CacheUnmatchedSeries, *file)
				} //cache.Append(logger.GlobalCache, logger.StrSerieFileUnmatched, file)
			} else {
				if config.SettingsGeneral.UseMediaCache {
					database.CacheUnmatchedMovie = append(database.CacheUnmatchedMovie, *file)
				} //cache.Append(logger.GlobalCache, logger.StrMovieFileUnmatched, file)
			}
			m.Close()
			return logfileerror(err, file, "parse failed ids")
		}
		if structurevar.groupType == logger.StrSeries && (m.M.DbserieEpisodeID == 0 || m.M.DbserieID == 0 || m.M.SerieEpisodeID == 0 || m.M.SerieID == 0) {
			m.Close()
			return nil
		}
		if structurevar.groupType == logger.StrMovie && (m.M.MovieID == 0 || m.M.DbmovieID == 0) {
			m.Close()
			return nil
		}

		var templateQuality string
		i := config.GetMediaListsEntryIndex(structurevar.cfgpstr, m.M.Listname)
		if m.M.Listname != "" && i != -1 && config.SettingsMedia[structurevar.cfgpstr].Lists[i].TemplateQuality != "" {
			templateQuality = config.SettingsMedia[structurevar.cfgpstr].Lists[i].TemplateQuality
		}
		if templateQuality == "" || !config.CheckGroup("quality_", templateQuality) {
			m.Close()
			return logfileerror(nil, file, "quality not found")
		}
		if structurevar.groupType == logger.StrSeries {
			if config.SettingsGeneral.UseMediaCache {
				ti := logger.IndexFunc(&database.CacheUnmatchedSeries, func(elem string) bool { return elem == *file })
				if ti != -1 {
					logger.Delete(&database.CacheUnmatchedSeries, ti)
				}
			}
			//logger.DeleteFromStringsCache(logger.StrSerieFileUnmatched, file)
		} else {
			if config.SettingsGeneral.UseMediaCache {
				ti := logger.IndexFunc(&database.CacheUnmatchedMovie, func(elem string) bool { return elem == *file })
				if ti != -1 {
					logger.Delete(&database.CacheUnmatchedMovie, ti)
				}
			}
			//logger.DeleteFromStringsCache(logger.StrMovieFileUnmatched, file)
		}
		if structurevar.listConfig != m.M.Listname || structurevar.templateQuality == "" {
			structurevar.listConfig = m.M.Listname
			for idxi := range config.SettingsMedia[structurevar.cfgpstr].Lists {
				if strings.EqualFold(config.SettingsMedia[structurevar.cfgpstr].Lists[idxi].Name, m.M.Listname) {
					structurevar.templateQuality = config.SettingsMedia[structurevar.cfgpstr].Lists[idxi].TemplateQuality
					break
				}
			}
		}
		if structurevar.groupType == logger.StrSeries && m.M.DbserieEpisodeID != 0 && m.M.DbserieID != 0 && m.M.SerieEpisodeID != 0 && m.M.SerieID != 0 {
			titleyear := database.QueryStringColumn(database.QueryDbseriesGetSerienameByID, m.M.DbserieID)
			if Checktitle(titleyear, database.QueryStaticStringArray(false,
				0, //database.QueryIntColumn(database.QueryDbserieAlternatesCountByDBID, m.DbserieID),
				database.QueryDbserieAlternatesGetTitleByDBID, m.M.DbserieID), templateQuality, &m.M) {
				m.Close()
				return logfileerror(nil, file, "Skipped - unwanted title")
			}
			mediaid := m.M.SerieID
			if manualid != 0 {
				mediaid = manualid
			}
			defer m.Close()
			return logfileerror(structurevar.organizeSeries(structurevar.rootpath, mediaid, file, deletewronglanguage, checkruntime, m), file, "structure")
		}
		if structurevar.groupType == logger.StrMovie && m.M.MovieID != 0 && m.M.DbmovieID != 0 {
			titleyear := database.QueryStringColumn(database.QueryDbmoviesGetTitleByID, m.M.DbmovieID)
			if Checktitle(titleyear, database.QueryStaticStringArray(false,
				0, //database.QueryIntColumn(database.QueryDbmovieTitlesCountByDBID, m.DbmovieID),
				database.QueryDbmovieTitlesGetTitleByID, m.M.DbmovieID), templateQuality, &m.M) {
				logger.Log.Debug().Str(logger.StrTitle, m.M.Title).Str("want title", titleyear).Msg("Skipped - unwanted title")
				m.Close()
				return nil
			}
			mediaid := m.M.MovieID
			if manualid != 0 {
				mediaid = manualid
			}
			defer m.Close()
			return logfileerror(structurevar.organizeMovie(structurevar.rootpath, mediaid, file, deletewronglanguage, checkruntime, m), file, "structure")
		}
		m.Close()
		return logfileerror(nil, file, "File not matched")
	})

}

func Checktitle(wantedTitle string, wantedAlternates *[]string, qualityTemplate string, m *apiexternal.ParseInfo) bool {
	if !config.SettingsQuality["quality_"+qualityTemplate].CheckTitle {
		return false
	}
	err := importfeed.StripTitlePrefixPostfixGetQual(m.Title, qualityTemplate)
	if err != nil {
		logger.Logerror(err, "Strip Failed")
	}

	titlechk := m.Title + " " + logger.IntToString(m.Year)
	//title := dl.GetNzbs[idx].ParseInfo.Title
	if wantedTitle != "" {
		if config.SettingsQuality["quality_"+qualityTemplate].CheckTitle && len(wantedTitle) >= 1 && apiexternal.Checknzbtitle(wantedTitle, m.Title) {
			return false
		}
		if m.Year != 0 && config.SettingsQuality["quality_"+qualityTemplate].CheckTitle && len(wantedTitle) >= 1 && apiexternal.Checknzbtitle(wantedTitle, titlechk) {
			return false
		}
	}
	if wantedAlternates == nil || len(*wantedAlternates) == 0 || m.Title == "" {
		return true
	}
	for idxtitle := range *wantedAlternates {
		if (*wantedAlternates)[idxtitle] == "" {
			continue
		}
		if apiexternal.Checknzbtitle((*wantedAlternates)[idxtitle], m.Title) {
			return false
		}

		if m.Year != 0 && config.SettingsQuality["quality_"+qualityTemplate].CheckTitle && apiexternal.Checknzbtitle((*wantedAlternates)[idxtitle], titlechk) {
			return false
		}
	}
	return true
}

func (s *parsertype) Close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.ClearVar(s.Dbmovie)
	logger.ClearVar(s.Dbserie)
	logger.ClearVar(s.DbserieEpisode)
	s.Source.Close()
	logger.Clear(&s.Episodes)
	logger.ClearVar(s)
}
func (s *inputNotifier) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	logger.Clear(&s.Replaced)
	logger.ClearVar(s.Dbmovie)
	logger.ClearVar(s.Dbserie)
	logger.ClearVar(s.DbserieEpisode)
	s.Source.Close()
	logger.ClearVar(s)
}

func (s *forstructurenotify) close() {
	if logger.DisableVariableCleanup {
		return
	}
	if s == nil {
		return
	}
	s.InputNotifier.close()
	logger.ClearVar(s)
}

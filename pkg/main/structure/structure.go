package structure

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/mozillazg/go-unidecode"
)

// Organizer struct contains configuration and path information for organizing media files
type Organizer struct {
	// Cfgp is a pointer to the MediaTypeConfig
	Cfgp *config.MediaTypeConfig
	// CfgImport is a pointer to the MediaDataImportConfig
	cfgImport *config.MediaDataImportConfig
	// SourcepathCfg is a pointer to the PathsConfig for the source path
	sourcepathCfg *config.PathsConfig
	// TargetpathCfg is a pointer to the PathsConfig for the target path
	targetpathCfg *config.PathsConfig
	// Checkruntime is a boolean indicating whether to check runtime during organization
	checkruntime bool
	// Deletewronglanguage is a boolean indicating whether to delete wrong language files
	deletewronglanguage bool
	// ManualId is a unit containing a manually set ID
	manualId uint
	//orgadata Organizerdata
}

type parsertype struct {
	Dbmovie            database.Dbmovie
	Dbserie            database.Dbserie
	DbserieEpisode     database.DbserieEpisode
	Source             *database.ParseInfo
	TitleSource        string
	EpisodeTitleSource string
	Identifier         string
	Episodes           []int
}

// inputNotifier contains information about the input file being processed
type inputNotifier struct {
	// Targetpath is the target path for the file
	Targetpath string
	// SourcePath is the source path of the file
	SourcePath string
	// Title is the title of the media
	Title string
	// Season is the season number for TV series
	Season string
	// Episode is the episode number for TV series
	Episode string
	// Identifier is the unique identifier for the media
	Identifier string
	// Series is the series name for TV series
	Series string
	// EpisodeTitle is the episode title for TV series
	EpisodeTitle string
	// Tvdb is the TVDB ID for TV series
	Tvdb string
	// Year is the year of release for the media
	Year string
	// Imdb is the IMDb ID for the media
	Imdb string
	// Configuration is the configuration name used for processing
	Configuration string
	// Replaced is a list of replaced strings during processing
	Replaced []string
	// ReplacedPrefix is the prefix used for replaced strings
	ReplacedPrefix string
	// Dbmovie is the Dbmovie struct for movies
	Dbmovie database.Dbmovie
	// Dbserie is the Dbserie struct for TV series
	Dbserie database.Dbserie
	// DbserieEpisode is the DbserieEpisode struct for TV series episodes
	DbserieEpisode database.DbserieEpisode
	// Source is the ParseInfo struct containing parsing info
	Source *database.ParseInfo
	// Time is the timestamp string
	Time string
}

// Organizerdata struct contains data for organizing media files
type Organizerdata struct {
	// videotarget is the video target path
	videotarget string
	// Foldername is the folder name
	Foldername string
	// Filename is the file name
	Filename string
	// Rootpath is the root path
	Rootpath string
	// Videofile is the video file path
	Videofile string
	// Listid is the list ID
	Listid int8
	// Folder is the folder path
	Folder   string
	Oldfiles []string
}

var (
	errRuntime              = errors.New("wrong runtime")
	errLowerQuality         = errors.New("lower quality")
	errSeasonEmpty          = errors.New("season empty")
	errNotFoundPathTemplate = errors.New("path template not found")
	errGeneratingFilename   = errors.New("generating filename")
	errWrongRuntime         = errors.New("wrong runtime")
	errWrongLanguage        = errors.New("wrong language")
	// namingReplacer replaces multiple spaces and brackets in strings
	namingReplacer = strings.NewReplacer("  ", " ", " ]", "]", "[ ", "[", "[]", "", "( )", "", "()", "")
	plStructure    = pool.NewPool[Organizer](20, 3, nil, func(o *Organizer) {
		o.Cfgp = nil
		o.cfgImport = nil
		o.sourcepathCfg = nil
		o.targetpathCfg = nil
		*o = Organizer{}
	})
)

// Clear resets the fields of an Organizerdata struct to their zero values.
// This is useful for clearing the state of an Organizerdata instance.
func (p *Organizerdata) Clear() {
	if p != nil && p.Folder != "" {
		clear(p.Oldfiles)
		*p = Organizerdata{}
	}
}
func (p *Organizerdata) Close() {
	if p != nil {
		clear(p.Oldfiles)
		*p = Organizerdata{}
	}
}

// Close resets the parsertype instance by setting its Episodes and Source fields to nil.
// This function is used to clean up the parsertype instance when it is no longer needed.
func (p *parsertype) close() {
	if p != nil {
		clear(p.Episodes)
		p.Episodes = nil
		p.Source = nil
		*p = parsertype{}
	}
}

// // Sets the Organizerdata for the Organizer struct.
// func (s *Organizer) SetOrga(o Organizerdata) {
// 	s.orgadata = o
// }

// // GetOrgaListID returns the list ID associated with the Organizer's orgadata.
// func (s *Organizer) GetOrgaListID() int8 {
// 	return o.Listid
// }

// // GetOrgaFolderName returns the folder name associated with the Organizer's orgadata.
// func (s *Organizer) GetOrgaFolderName() string {
// 	return o.Foldername
// }

// // GetOrgaFileName returns the filename of the video file associated with the Organizer.
// func (s *Organizer) GetOrgaFileName() string {
// 	return o.Filename
// }

// FileCleanup removes the video file and cleans up the folder for the given Organizerdata.
// It handles both series and non-series files.
func (s *Organizer) fileCleanup(o *Organizerdata) error {
	if !s.Cfgp.Useseries || o.Videofile == "" {
		if o.Videofile != "" {
			bl, err := scanner.RemoveFile(o.Videofile)
			if err != nil {
				return err
			}
			if !bl {
				return nil
			}

			if s.sourcepathCfg.Name == "" {
				return errNotFoundPathTemplate
			}

			if !scanner.CheckFileExist(o.Folder) {
				return logger.ErrNotFound
			}
			s.walkcleanup(o, true)
		}
		return s.cleanUpFolder(o)
	}
	bl, err := scanner.RemoveFile(o.Videofile)
	if err == nil && bl {
		s.removeotherfiles(o)
		return s.cleanUpFolder(o)
	}
	return err
}

// CleanUpFolder walks the given folder path to calculate total size.
// It then compares total size to given cleanup threshold in MB.
// If folder size is less than threshold, folder is deleted.
// Returns any error encountered.
func (s *Organizer) cleanUpFolder(o *Organizerdata) error {
	// if cleanupsizeMB == 0 {
	// 	return nil
	// }
	if !scanner.CheckFileExist(o.Folder) {
		return errors.New("cleanup folder not found")
	}
	var leftsize int64

	cleanupsizeByte := int64(s.sourcepathCfg.CleanupsizeMB) * 1024 * 1024 //MB to Byte
	err := filepath.WalkDir(o.Folder, func(_ string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if cleanupsizeByte <= leftsize {
			return filepath.SkipAll
		}
		fsinfo, err := info.Info()
		if err == nil {
			leftsize += fsinfo.Size()
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.LogDynamicany("debug", "Left size", &logger.StrSize, &leftsize)
	if cleanupsizeByte >= leftsize || leftsize == 0 {
		_ = filepath.WalkDir(o.Folder, setchmodwalk)
		err := os.RemoveAll(o.Folder)
		if err != nil {
			return err
		}
		logger.LogDynamicany("info", "Folder removed", &logger.StrFile, &o.Folder)
	}
	return nil
}

// setchmodwalk changes the file permissions of the given path to 0777 recursively.
// It is used to ensure folders can be deleted properly.
func setchmodwalk(pathv string, _ fs.DirEntry, errw error) error {
	if errw != nil {
		return errw
	}
	_ = os.Chmod(pathv, 0777)
	return nil
}

// ParseFileAdditional performs additional parsing and validation on a video file.
// It checks the runtime, language, and quality against configured values and cleans up the file if needed.
// It is used after initial parsing to enforce business logic around file properties.
func (s *Organizer) ParseFileAdditional(o *Organizerdata, m *database.ParseInfo, deletewronglanguage bool, wantedruntime int, checkruntime bool, cfgQuality *config.QualityConfig) error {
	if o.Listid == -1 {
		return logger.ErrListnameTemplateEmpty
	}

	parser.GetPriorityMapQual(m, s.Cfgp, cfgQuality, true, true)
	err := parser.ParseVideoFile(m, o.Videofile, cfgQuality)
	if err != nil {
		if errors.Is(err, logger.ErrTracksEmpty) {
			_ = s.fileCleanup(o)
		}
		return err
	}
	targetruntime := m.Runtime / 60
	if m.Runtime >= 1 && checkruntime && wantedruntime != 0 && s.targetpathCfg.MaxRuntimeDifference != 0 && targetruntime != wantedruntime {
		maxdifference := s.targetpathCfg.MaxRuntimeDifference
		if m.Extended && !s.Cfgp.Useseries {
			maxdifference += 10
		}
		var difference int
		if wantedruntime > targetruntime {
			difference = wantedruntime - targetruntime
		} else {
			difference = targetruntime - wantedruntime
		}

		if difference > maxdifference {
			if s.targetpathCfg.DeleteWrongRuntime {
				_ = s.fileCleanup(o)
			}
			logger.LogDynamicany("warning", "wrong runtime", &logger.StrWanted, wantedruntime, &logger.StrFound, &targetruntime) //logpointerr
			return errWrongRuntime
		}
	}
	if !deletewronglanguage || s.targetpathCfg.AllowedLanguagesLen == 0 {
		return nil
	}
	var bl bool
	lenlang := len(m.Languages)
	for idx := range s.targetpathCfg.AllowedLanguages {
		if lenlang == 0 && s.targetpathCfg.AllowedLanguages[idx] == "" {
			bl = true
			break
		}
		if logger.SlicesContainsI(m.Languages, s.targetpathCfg.AllowedLanguages[idx]) {
			bl = true
			break
		}
	}
	if !bl {
		if deletewronglanguage {
			err = s.fileCleanup(o)
			if err != nil {
				logger.LogDynamicany("warning", "wrong language", err, &logger.StrWanted, &s.targetpathCfg.AllowedLanguages, &logger.StrFound, &m.Languages[0])
				return errWrongLanguage
			}
		}
		logger.LogDynamicany("warning", "wrong language", &logger.StrWanted, &s.targetpathCfg.AllowedLanguages, &logger.StrFound, &m.Languages[0])
		return errWrongLanguage
	}
	return nil
}

// TrimStringInclAfterString truncates the given string s after the first
// occurrence of the search string. It returns the truncated string.
func trimStringInclAfterString(s string, search string) string {
	if idx := logger.IndexI(s, search); idx != -1 {
		return s[:idx]
	}
	return s
}

// GenerateNamingTemplate generates the folder name and file name for a movie or TV show file
// based on the configured naming template. It looks up metadata from the database and parses
// the naming template to replace placeholders with actual values. It handles movies and shows
// differently based on the UseSeries config option.
func (s *Organizer) GenerateNamingTemplate(o *Organizerdata, m *database.ParseInfo, dbid *uint, tblepi []database.DbstaticTwoUint) {
	//forparser := plparser.Get()
	forparser := parsertype{Source: m}
	defer forparser.close()
	//defer plparser.Put(forparser)
	//forparser := parsertype{Source: &m.M}
	var bl bool
	if !s.Cfgp.Useseries {
		if forparser.Dbmovie.GetDbmovieByIDP(dbid) != nil {
			return
		}
		forparser.Dbmovie.Title = logger.Path(forparser.Dbmovie.Title, false)
		forparser.TitleSource = filepath.Base(o.Videofile)
		forparser.TitleSource = trimStringInclAfterString(forparser.TitleSource, forparser.Source.Quality)
		forparser.TitleSource = trimStringInclAfterString(forparser.TitleSource, forparser.Source.Resolution)
		if forparser.Source.Year != 0 {
			if idx := strings.Index(forparser.TitleSource, logger.IntToString(forparser.Source.Year)); idx != -1 {
				forparser.TitleSource = forparser.TitleSource[:idx]
			}

			//forparser.TitleSource = logger.TrimStringInclAfterInt(forparser.TitleSource, forparser.Source.Year)
		}
		if forparser.TitleSource != "" && (forparser.TitleSource[:1] == logger.StrDot || forparser.TitleSource[len(forparser.TitleSource)-1:] == logger.StrDot) {
			forparser.TitleSource = strings.Trim(forparser.TitleSource, logger.StrDot)
		}
		forparser.TitleSource = logger.Path(forparser.TitleSource, false)

		logger.StringReplaceWithP(&forparser.TitleSource, '.', ' ')

		if forparser.Dbmovie.Title == "" {
			_ = database.Scanrows1dyn(false, database.QueryDbmovieTitlesGetTitleByIDLmit1, &forparser.Dbmovie.Title, dbid)
			if forparser.Dbmovie.Title == "" {
				forparser.Dbmovie.Title = forparser.TitleSource
			}
		}
		if forparser.Dbmovie.Year == 0 {
			forparser.Dbmovie.Year = forparser.Source.Year
		}

		o.Foldername, o.Filename = logger.SplitByLR(s.Cfgp.Naming, checksplit(s.Cfgp.Naming))

		if o.Rootpath != "" {
			_, getfoldername := logger.SplitByLR(o.Rootpath, checksplit(o.Rootpath))
			if getfoldername != "" {
				o.Foldername = "" //getfoldername
			}
		}

		if forparser.Source.Imdb == "" {
			forparser.Source.Imdb = forparser.Dbmovie.ImdbID
		}
		if forparser.Source.Imdb != "" {
			logger.AddImdbPrefixP(&forparser.Source.Imdb)
		}

		forparser.Source.Title = logger.Path(logger.StringRemoveAllRunes(forparser.Source.Title, '/'), false)

		bl, o.Foldername = logger.ParseStringTemplate(o.Foldername, &forparser)
		if bl {
			o.cleanorgafilefolder()
			return
		}
		bl, o.Filename = logger.ParseStringTemplate(o.Filename, &forparser)
		if bl {
			o.cleanorgafilefolder()
			return
		}
		if o.Foldername != "" && (o.Foldername[:1] == logger.StrDot || o.Foldername[len(o.Foldername)-1:] == logger.StrDot) {
			o.Foldername = strings.Trim(o.Foldername, logger.StrDot)
		}
		o.Foldername = logger.DiacriticsReplacer(o.Foldername)
		o.Foldername = logger.Path(o.Foldername, true)
		o.Foldername = unidecode.Unidecode(o.Foldername)

		if o.Filename != "" && (o.Filename[:1] == logger.StrDot || o.Filename[len(o.Filename)-1:] == logger.StrDot) {
			o.Filename = strings.Trim(o.Filename, logger.StrDot)
		}
		o.Filename = namingReplacer.Replace(o.Filename)

		o.Filename = logger.DiacriticsReplacer(o.Filename)
		o.Filename = logger.Path(o.Filename, false)
		o.Filename = unidecode.Unidecode(o.Filename)
		//s.generatenamingmovie(orgadata, &m.M, dbid)
		return
	}

	//Naming Series
	_ = database.Scanrows1dyn(false, database.QuerySerieEpisodesGetDBSerieIDByID, &forparser.Dbserie.ID, dbid)
	_ = database.Scanrows1dyn(false, database.QuerySerieEpisodesGetDBSerieEpisodeIDByID, &forparser.DbserieEpisode.ID, dbid)
	if forparser.DbserieEpisode.ID == 0 || forparser.Dbserie.ID == 0 {
		return
	}
	if forparser.Dbserie.GetDbserieByIDP(&forparser.Dbserie.ID) != nil {
		return
	}
	if forparser.DbserieEpisode.GetDbserieEpisodesByIDP(&forparser.DbserieEpisode.ID) != nil {
		return
	}
	o.Foldername, o.Filename = logger.SplitByLR(s.Cfgp.Naming, checksplit(s.Cfgp.Naming))

	episodetitle := database.Getdatarow1[string](false, "select title from dbserie_episodes where id = ?", &tblepi[0].Num2)
	serietitle := database.Getdatarow1[string](false, "select seriename from dbseries where id = ?", &m.DbserieID)
	if (serietitle == "" || episodetitle == "") && m.Identifier != "" {
		serietitleparse, episodetitleparse := database.RegexGetMatchesStr1Str2(true, logger.JoinStrings(`^(.*)(?i)`, m.Identifier, `(?:\.| |-)(.*)$`), filepath.Base(o.Videofile))
		if serietitle != "" && episodetitleparse != "" {
			logger.StringReplaceWithP(&episodetitleparse, '.', ' ')

			episodetitleparse = trimStringInclAfterString(episodetitleparse, "XXX")
			episodetitleparse = trimStringInclAfterString(episodetitleparse, m.Quality)
			episodetitleparse = trimStringInclAfterString(episodetitleparse, m.Resolution)
			episodetitleparse = strings.Trim(episodetitleparse, ". ")

			if serietitleparse != "" && (serietitleparse[:1] == logger.StrDot || serietitleparse[len(serietitleparse)-1:] == logger.StrDot) {
				serietitleparse = strings.Trim(serietitleparse, logger.StrDot)
			}
			logger.StringReplaceWithP(&serietitleparse, '.', ' ')
		}

		if episodetitle == "" {
			episodetitle = episodetitleparse
		}
		if serietitle == "" {
			serietitle = serietitleparse
		}
	}
	//serietitle, episodetitle := o.GetEpisodeTitle(m, &tblepi[0].Num2)
	if forparser.Dbserie.Seriename == "" {
		_ = database.Scanrows1dyn(false, "select title from dbserie_alternates where dbserie_id = ?", &forparser.Dbserie.Seriename, &forparser.Dbserie.ID)
		if forparser.Dbserie.Seriename == "" {
			forparser.Dbserie.Seriename = serietitle
		}
	}
	logger.StringRemoveAllRunesP(&forparser.Dbserie.Seriename, '/')

	if forparser.DbserieEpisode.Title == "" {
		forparser.DbserieEpisode.Title = episodetitle
	}
	logger.StringRemoveAllRunesP(&forparser.DbserieEpisode.Title, '/')

	forparser.Dbserie.Seriename = logger.Path(forparser.Dbserie.Seriename, false)
	forparser.DbserieEpisode.Title = logger.Path(forparser.DbserieEpisode.Title, false)
	if o.Rootpath != "" {
		_, getfoldername := logger.SplitByLR(o.Rootpath, checksplit(o.Rootpath))
		if getfoldername != "" {
			identifiedby := database.Getdatarow1[string](false, database.QueryDbseriesGetIdentifiedByID, &m.DbserieID)

			if identifiedby == "date" {
				o.Foldername = ""
			} else {
				splitbyget := checksplit(o.Foldername)
				if splitbyget != ' ' {
					_, seasonname := logger.SplitByLR(o.Foldername, splitbyget)
					o.Foldername = seasonname
				} else {
					o.Foldername = getfoldername
				}
			}
		}
	}

	forparser.Episodes = make([]int, len(tblepi))
	for idx := range tblepi {
		_ = database.Scanrows1dyn(false, "select episode from dbserie_episodes where id = ? and episode != ''", &forparser.Episodes[idx], &tblepi[idx].Num2)
	}
	forparser.TitleSource = logger.Path(serietitle, false)
	logger.StringRemoveAllRunesP(&forparser.TitleSource, '/')

	forparser.EpisodeTitleSource = logger.Path(episodetitle, false)
	logger.StringRemoveAllRunesP(&forparser.EpisodeTitleSource, '/')

	if forparser.Source.Tvdb == "0" || forparser.Source.Tvdb == "" || forparser.Source.Tvdb == "tvdb" || strings.EqualFold(forparser.Source.Tvdb, "tvdb") {
		forparser.Source.Tvdb = strconv.Itoa(forparser.Dbserie.ThetvdbID)
	}
	if forparser.Source.Tvdb != "" {
		if len(forparser.Source.Tvdb) >= 1 && !logger.HasPrefixI(forparser.Source.Tvdb, logger.StrTvdb) {
			//return JoinStrings(StrTvdb, str)
			forparser.Source.Tvdb = logger.JoinStrings(logger.StrTvdb, forparser.Source.Tvdb)
		}
		//forparser.Source.Tvdb = logger.AddTvdbPrefix(forparser.Source.Tvdb)
	}
	bl, o.Foldername = logger.ParseStringTemplate(o.Foldername, &forparser)
	if bl {
		o.cleanorgafilefolder()
		//clear(forparser.Episodes)
		return
	}
	bl, o.Filename = logger.ParseStringTemplate(o.Filename, &forparser)
	if bl {
		o.cleanorgafilefolder()
		//clear(forparser.Episodes)
		return
	}
	if o.Foldername != "" && (o.Foldername[:1] == logger.StrDot || o.Foldername[len(o.Foldername)-1:] == logger.StrDot) {
		o.Foldername = strings.Trim(o.Foldername, logger.StrDot)
	}
	o.Foldername = logger.DiacriticsReplacer(o.Foldername)
	o.Foldername = logger.Path(o.Foldername, true)
	o.Foldername = unidecode.Unidecode(o.Foldername)

	if o.Filename != "" && (o.Filename[:1] == logger.StrDot || o.Filename[len(o.Filename)-1:] == logger.StrDot) {
		o.Filename = strings.Trim(o.Filename, logger.StrDot)
	}

	o.Filename = namingReplacer.Replace(o.Filename)
	o.Filename = logger.DiacriticsReplacer(o.Filename)
	o.Filename = logger.Path(o.Filename, false)
	o.Filename = unidecode.Unidecode(o.Filename)
	//clear(forparser.Episodes)
	//s.generatenamingserie(orgadata, m, dbid, tblepi)
}

// cleanorgafilefolder clears the foldername and filename fields of the provided Organizerdata struct to empty strings.
func (o *Organizerdata) cleanorgafilefolder() {
	o.Foldername = ""
	o.Filename = ""
}

// moveVideoFile moves the video file specified in orgadata to the target folder.
// It creates the target folder if needed, setting permissions according to TargetpathCfg.
// The target filename is set in orgadata.Filename.
// Returns a bool indicating if the move was successful, and an error.
func (s *Organizer) moveVideoFile(o *Organizerdata) (string, error) {
	if o.Rootpath != "" {
		o.videotarget = filepath.Join(o.Rootpath, o.Foldername)
	} else {
		o.videotarget = filepath.Join(s.targetpathCfg.Path, o.Foldername)
	}

	mode := fs.FileMode(0777)
	if s.targetpathCfg.SetChmodFolder != "" && len(s.targetpathCfg.SetChmodFolder) == 4 {
		mode = logger.StringToFileMode(s.targetpathCfg.SetChmodFolder)
	}
	err := os.MkdirAll(o.videotarget, mode)
	if err != nil {
		return "", err
	}
	if mode != 0 {
		_ = os.Chmod(o.videotarget, mode)
	}
	return scanner.MoveFile(o.Videofile, s.sourcepathCfg, o.videotarget, o.Filename, false, false, config.SettingsGeneral.UseFileBufferCopy, s.targetpathCfg.SetChmodFolder, s.targetpathCfg.SetChmod)
}

// moveRemoveOldMediaFile moves or deletes an old media file that is being
// replaced. It handles moving/deleting additional files with different
// extensions, and removing database references. This is an internal
// implementation detail not meant to be called externally.
func (s *Organizer) moveRemoveOldMediaFile(oldfile string, id *uint, move bool) error {
	if oldfile == "" {
		return nil
	}
	if move {
		_, err := scanner.MoveFile(oldfile, nil, filepath.Join(s.targetpathCfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", false, true, config.SettingsGeneral.UseFileBufferCopy, s.targetpathCfg.SetChmodFolder, s.targetpathCfg.SetChmod)
		if err != nil {
			if errors.Is(err, logger.ErrNotFound) {
				return nil
			}
			return err
		}
	} else {
		bl, err := scanner.RemoveFile(oldfile)
		if err != nil {
			return err
		}
		if !bl {
			return logger.ErrNotAllowed
		}
	}

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheFiles), oldfile)
	}
	database.ExecNMap(s.Cfgp.Useseries, logger.DBDeleteFileByIDLocation, id, oldfile) //sqlpointerr

	fileext := filepath.Ext(oldfile)
	for idx := range s.sourcepathCfg.AllowedOtherExtensions {
		if fileext == s.sourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		s.moveremovefile(oldfile, fileext, s.sourcepathCfg.AllowedOtherExtensions[idx], move)
	}
	return nil
}

// moveremovefile moves or removes an additional file based on the provided parameters.
//
// oldfile is the path to the original file.
// fileext is the extension of the original file.
// checkext is the extension to check for the additional file.
// move is a boolean indicating whether to move or remove the additional file.
//
// If the additional file exists, it will either be moved to the target path or removed.
// If an error occurs during the move or removal, it will be logged.
func (s *Organizer) moveremovefile(oldfile string, fileext string, checkext string, move bool) {
	additionalfile := strings.ReplaceAll(oldfile, fileext, checkext)
	if !scanner.CheckFileExist(additionalfile) {
		return
	}
	if move {
		_, err := scanner.MoveFile(additionalfile, nil, filepath.Join(s.targetpathCfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", false, true, config.SettingsGeneral.UseFileBufferCopy, s.targetpathCfg.SetChmodFolder, s.targetpathCfg.SetChmod)
		if err != nil {
			if !errors.Is(err, logger.ErrNotFound) {
				logger.LogDynamicany("error", "file could not be moved", err, &logger.StrFile, &additionalfile)
			}
			return
		}
	} else {
		bl, err := scanner.RemoveFile(additionalfile)
		if err != nil {
			logger.LogDynamicany("error", "delete Files", err)
			return
		}
		if !bl {
			return
		}
	}
	logger.LogDynamicany("info", "Additional File removed", &logger.StrFile, &additionalfile)
}

// OrganizeSeries organizes a downloaded series episode file by moving it to the target folder,
// updating the database, removing old lower quality files, and sending notifications.
// It takes organizer data, parsed file info, series ID, quality config, flags to delete
// wrong language and check runtime, and returns any error.
func (s *Organizer) organizeSeries(o *Organizerdata, m *database.ParseInfo, cfgquality *config.QualityConfig, deletewronglanguage bool, checkruntime bool) error {
	var listname string
	database.GetdatarowArgs("select dbserie_id, rootpath, listname from series where id = ?", &m.SerieID, &m.DbserieID, &o.Rootpath, &listname)
	if m.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}

	if o.Listid == -1 {
		o.Listid = s.Cfgp.GetMediaListsEntryListID(listname)
		m.ListID = o.Listid
	}

	err := s.GetSeriesEpisodes(o, m, false, cfgquality)
	if err != nil {
		return err
	}
	if len(m.Episodes) == 0 {
		return logger.ErrNotFoundEpisode
	}

	database.GetdatarowArgs("select runtime, season from dbserie_episodes where id = ?", &m.Episodes[0].Num2, &m.RuntimeStr, &m.SeasonStr)

	identifiedby := database.Getdatarow1[string](false, database.QueryDbseriesGetIdentifiedByID, &m.DbserieID)
	if (m.RuntimeStr == "" || m.RuntimeStr == "0") && identifiedby != "date" {
		_ = database.Scanrows1dyn(false, "select runtime from dbseries where id = ?", &m.RuntimeStr, &m.DbserieID)
		if (m.RuntimeStr == "" || m.RuntimeStr == "0") && checkruntime && identifiedby != "date" {
			return errRuntime
		}
	}
	if m.SeasonStr == "" && identifiedby != "date" {
		return errSeasonEmpty
	}

	var totalruntime int
	if !database.Getdatarow1[bool](false, "select ignore_runtime from serie_episodes where id = ?", &m.Episodes[0].Num1) {
		if identifiedby != "date" && (m.RuntimeStr != "" && m.RuntimeStr != "0") && (m.SeasonStr != "0" && m.SeasonStr != "") {
			m.Runtime, _ = strconv.Atoi(m.RuntimeStr)
			totalruntime = m.Runtime * len(m.Episodes)
		}
	}

	err = s.ParseFileAdditional(o, m, deletewronglanguage, totalruntime, checkruntime, cfgquality)
	if err != nil {
		return err
	}

	s.GenerateNamingTemplate(o, m, &m.Episodes[0].Num1, m.Episodes)
	if o.Filename == "" {
		return errGeneratingFilename
	}

	if s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "" && len(o.Oldfiles) >= 1 {
		//Move old files to replaced folder
		err = s.moveremoveoldfiles(o, false, &m.SerieID, true, o.Oldfiles)
		if err != nil {
			return err
		}
	}

	if s.targetpathCfg.Usepresort && s.targetpathCfg.PresortFolderPath != "" {
		o.Rootpath = filepath.Join(s.targetpathCfg.PresortFolderPath, o.Foldername)
	}
	//Move new files to target folder
	newpath, err := s.moveVideoFile(o)
	if err != nil {
		if errors.Is(err, logger.ErrNotFound) {
			return nil
		}
		return err
	}
	//Remove old files from target folder
	err = s.moveandcleanup(o, newpath, m, &m.SerieID, &m.Episodes[0].Num2, o.Oldfiles)
	if err != nil {
		return err
	}
	//updateserie

	fileext := filepath.Ext(o.Videofile)
	filebase := filepath.Base(newpath)

	var reached int
	if m.Priority >= cfgquality.CutoffPriority {
		reached = 1
	}
	for idx := range m.Episodes {
		database.ExecN("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&newpath, &filebase, &fileext, &cfgquality.Name, &m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID, &m.Proper, &m.Repack, &m.Extended, &m.SerieID, &m.Episodes[idx].Num1, &m.Episodes[idx].Num2, &m.DbserieID, &m.Height, &m.Width)

		database.ExecN("update serie_episodes SET missing = 0, quality_reached = ? where id = ?", &reached, &m.Episodes[idx].Num1)
	}

	if config.SettingsGeneral.UseMediaCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, newpath)
		database.AppendCache(logger.CacheFilesSeries, newpath)
	}
	return nil
}

// OrganizeMovie organizes a downloaded movie file by moving it to the target folder,
// updating the database, removing old lower quality files, and sending notifications.
// It takes organizer data, parsed file info, movie ID, quality config, flags to delete
// wrong language and check runtime, and returns any error.
func (s *Organizer) organizeMovie(o *Organizerdata, m *database.ParseInfo, cfgquality *config.QualityConfig, deletewronglanguage bool, checkruntime bool) error {
	var listname string
	database.GetdatarowArgs("select dbmovie_id, rootpath, listname from movies where id = ?", &m.MovieID, &m.DbmovieID, &o.Rootpath, &listname)
	if m.DbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}
	database.Scanrows1dyn(false, "select runtime from dbmovies where id = ?", &m.RuntimeStr, &m.DbmovieID)
	if (m.RuntimeStr == "" || m.RuntimeStr == "0") && checkruntime {
		return errRuntime
	}
	if o.Listid == -1 {
		o.Listid = s.Cfgp.GetMediaListsEntryListID(listname)
		m.ListID = o.Listid
	}
	if o.Listid == -1 {
		return logger.ErrListnameTemplateEmpty
	}

	m.Runtime, _ = strconv.Atoi(m.RuntimeStr)
	err := s.ParseFileAdditional(o, m, deletewronglanguage, m.Runtime, checkruntime, cfgquality)
	if err != nil {
		return err
	}

	oldpriority, oldfiles := searcher.Getpriobyfiles(false, &m.MovieID, true, m.Priority, cfgquality)
	if oldpriority != 0 && oldpriority >= m.Priority {
		if true {
			err := s.fileCleanup(o)
			if err != nil {
				return err
			}
		}
		return errLowerQuality
	}

	s.GenerateNamingTemplate(o, m, &m.DbmovieID, nil)
	if o.Filename == "" {
		return errGeneratingFilename
	}

	if s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "" && len(oldfiles) >= 1 {
		//Move old files to replaced folder
		err = s.moveremoveoldfiles(o, false, &m.MovieID, true, oldfiles)
		if err != nil {
			return err
		}
	}

	if s.targetpathCfg.Usepresort && s.targetpathCfg.PresortFolderPath != "" {
		o.Rootpath = filepath.Join(s.targetpathCfg.PresortFolderPath, o.Foldername)
	}
	//Move new files to target folder
	newpath, err := s.moveVideoFile(o)
	if err != nil {
		if errors.Is(err, logger.ErrNotFound) {
			return nil
		}
		return err
	}

	//Remove old files from target folder
	err = s.moveandcleanup(o, newpath, m, &m.MovieID, &m.DbmovieID, oldfiles)
	if err != nil {
		return err
	}

	fileext := filepath.Ext(newpath)
	filebase := filepath.Base(newpath)
	//updatemovie
	database.ExecN("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&newpath, &filebase, &fileext, &cfgquality.Name, &m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID, &m.Proper, &m.Repack, &m.Extended, &m.MovieID, &m.DbmovieID, &m.Height, &m.Width)

	var vc int
	if m.Priority >= cfgquality.CutoffPriority {
		vc = 1
	}
	database.ExecN("update movies SET missing = 0, quality_reached = ? where id = ?", &vc, &m.MovieID)

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, newpath)
		database.AppendCache(logger.CacheFilesMovie, newpath)
	}
	return nil
}

// moveandcleanup moves new files to target folder, updates rootpath in database,
// removes old lower quality files from target if enabled, cleans up source folder,
// and sends notifications. It takes organizer data, parsed file info, movie/series ID,
// database movie ID, and list of old files. Returns any error.
func (s *Organizer) moveandcleanup(o *Organizerdata, newfile string, m *database.ParseInfo, id *uint, dbid *uint, oldfiles []string) error {
	//Update rootpath
	if !s.targetpathCfg.Usepresort {
		database.Scanrows1dyn(false, logger.GetStringsMap(s.Cfgp.Useseries, logger.DBRootPathFromMediaID), &m.TempTitle, id)
		if m.TempTitle == "" {
			//if database.Getdatarow1Map[string](false, s.Cfgp.Useseries, logger.DBRootPathFromMediaID, id) == "" {
			if !s.Cfgp.Useseries {
				UpdateRootpath(o.videotarget, "movies", &m.MovieID, s.Cfgp)
			} else {
				UpdateRootpath(o.videotarget, logger.StrSeries, &m.SerieID, s.Cfgp)
			}
		}
	}
	//Update Rootpath end

	if s.targetpathCfg.Replacelower && len(oldfiles) >= 1 {
		newold := oldfiles[:0]
		for idx := range oldfiles {
			if oldfiles[idx] == newfile {
				continue
			}
			newold = append(newold, oldfiles[idx])
		}
		err := s.moveremoveoldfiles(o, true, id, false, newold)
		if err != nil {
			return err
		}
	}

	if !s.Cfgp.Useseries {
		//move other movie

		if s.sourcepathCfg.Name == "" {
			return errNotFoundPathTemplate
		}

		if !scanner.CheckFileExist(o.Folder) {
			return logger.ErrNotFound
		}
		s.walkcleanup(o, false)
		s.notify(o, m, dbid, oldfiles)
		_ = s.cleanUpFolder(o)
		return nil
	}
	//move other serie
	fileext := filepath.Ext(o.Videofile)
	var also string
	for idx := range s.sourcepathCfg.AllowedOtherExtensions {
		if fileext == s.sourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		also = strings.ReplaceAll(o.Videofile, fileext, s.sourcepathCfg.AllowedOtherExtensions[idx])
		_, err := scanner.MoveFile(also, s.sourcepathCfg, o.videotarget, o.Filename, true, false, config.SettingsGeneral.UseFileBufferCopy, s.targetpathCfg.SetChmodFolder, s.targetpathCfg.SetChmod)
		if err != nil && !errors.Is(err, logger.ErrNotFound) {
			logger.LogDynamicany("error", "file move", err, &logger.StrFile, &also)
		}
	}

	s.notify(o, m, dbid, oldfiles)
	_ = s.cleanUpFolder(o)
	return nil
}

// walkcleanup recursively walks the given root path and cleans up files.
// It calls filepath.WalkDir to traverse all files under the root path.
// For each file, it checks if it should be filtered via scanner.Filterfile.
// If so, it will either remove the file or move it to the target folder,
// depending on the useremove parameter.
// Any errors during walking or moving/removing are logged.
func (s *Organizer) walkcleanup(o *Organizerdata, useremove bool) error {
	return filepath.WalkDir(o.Rootpath, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if fpath == o.Rootpath {
			return nil
		}
		ok, _ := scanner.CheckExtensions(false, true, s.sourcepathCfg, filepath.Ext(info.Name()))
		if !ok {
			return nil
		}
		//Check IgnoredPaths

		if s.sourcepathCfg.BlockedLen >= 1 {
			if logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
				ok = false
			}
		}
		if ok {
			if useremove {
				_, _ = scanner.RemoveFile(fpath)
			} else {
				_, err := scanner.MoveFile(fpath, s.sourcepathCfg, o.videotarget, o.Filename, true, false, config.SettingsGeneral.UseFileBufferCopy, s.targetpathCfg.SetChmodFolder, s.targetpathCfg.SetChmod)
				if err != nil && !errors.Is(err, logger.ErrNotFound) {
					logger.LogDynamicany("error", "file move", err, &logger.StrFile, fpath) //logpointerr
				}
			}
		}
		return nil
	})
}

// moveremoveoldfiles moves or removes old media files that are no longer associated with the given ID.
// It loops through the provided list of old file paths, skipping any that match the current filename or target folder.
// For each old file, it calls moveRemoveOldMediaFile to move or remove it.
// If any file cannot be moved/removed, it logs an error but continues the loop.
// Finally it returns any error encountered, or nil if successful.
func (s *Organizer) moveremoveoldfiles(o *Organizerdata, usecompare bool, id *uint, move bool, oldfiles []string) error {
	for idx := range oldfiles {
		if usecompare {
			_, teststr := filepath.Split(oldfiles[idx])
			if teststr == o.Filename || strings.HasPrefix(teststr, o.videotarget) {
				continue
			}
		}
		if !scanner.CheckFileExist(oldfiles[idx]) {
			continue
		}
		err := s.moveRemoveOldMediaFile(oldfiles[idx], id, move)
		if err != nil {
			//Continue if old cannot be moved
			logger.LogDynamicany("error", "Move old", err, &logger.StrFile, &oldfiles[idx])
			return err
		}
	}
	return nil
}

// notify sends notifications when new media is added.
// It populates notification data from the Organizerdata and ParseInfo,
// loops through the configured notifications, renders notification messages,
// and dispatches them based on the notification type.
func (s *Organizer) notify(o *Organizerdata, m *database.ParseInfo, id *uint, oldfiles []string) {
	notify := inputNotifier{
		Targetpath:    filepath.Join(o.videotarget, o.Filename),
		SourcePath:    o.Videofile,
		Configuration: s.Cfgp.Lists[o.Listid].Name,
		Source:        m,
		Time:          logger.TimeGetNow().Format(logger.GetTimeFormat()),
	}
	notify.Replaced = oldfiles

	if !s.Cfgp.Useseries {
		if notify.Dbmovie.GetDbmovieByIDP(id) != nil {
			return
		}
		notify.Title = notify.Dbmovie.Title
		notify.Year = logger.IntToString(notify.Dbmovie.Year)
		notify.Imdb = notify.Dbmovie.ImdbID
	} else {
		if notify.DbserieEpisode.GetDbserieEpisodesByIDP(id) != nil {
			return
		}
		if notify.Dbserie.GetDbserieByIDP(&notify.DbserieEpisode.DbserieID) != nil {
			return
		}
		notify.Title = notify.Dbserie.Seriename
		notify.Year = notify.Dbserie.Firstaired
		notify.Series = notify.Dbserie.Seriename
		notify.Tvdb = strconv.Itoa(notify.Dbserie.ThetvdbID)
		notify.Season = notify.DbserieEpisode.Season
		notify.Episode = notify.DbserieEpisode.Episode
		notify.Identifier = notify.DbserieEpisode.Identifier
	}

	for idx := range s.Cfgp.Notification {
		if s.Cfgp.Notification[idx].CfgNotification == nil || !strings.EqualFold(s.Cfgp.Notification[idx].Event, "added_data") {
			continue
		}
		notify.ReplacedPrefix = s.Cfgp.Notification[idx].ReplacedPrefix
		bl, messagetext := logger.ParseStringTemplate(s.Cfgp.Notification[idx].Message, &notify)
		if bl {
			continue
		}

		switch config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].NotificationType {
		case "pushover":
			bl, messageTitle := logger.ParseStringTemplate(s.Cfgp.Notification[idx].Title, &notify)
			if bl {
				continue
			}
			err := apiexternal.GetPushoverclient(config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Apikey).SendPushoverMessage(messagetext, messageTitle, config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Recipient)
			if err != nil {
				logger.LogDynamicany("error", "Error sending pushover", err)
			} else {
				logger.LogDynamicany("info", "Pushover message sent")
			}
		case "csv":
			if scanner.AppendCsv(config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Outputto, messagetext) != nil {
				continue
			}
		}
	}
	notify.close()
}

// GetSeriesEpisodes checks existing files for a series episode, determines if a new file
// should replace them based on configured quality priorities, deletes lower priority files if enabled,
// and returns the list of episode IDs that are allowed to be imported along with any deleted file paths.
func (s *Organizer) GetSeriesEpisodes(o *Organizerdata, m *database.ParseInfo, skipdelete bool, cfgquality *config.QualityConfig) error {
	err := m.Getepisodestoimport()
	if err != nil {
		return err
	}
	if len(m.Episodes) == 0 {
		return logger.ErrNotFoundEpisode
	}

	parser.GetPriorityMapQual(m, s.Cfgp, cfgquality, true, true)

	o.Oldfiles = make([]string, 0, len(m.Episodes)*2)

	newtbl := m.Episodes[:0]
	var bl bool
	var oldPrio int
	var getoldfiles []string
	for idx := range m.Episodes {
		oldPrio, getoldfiles = searcher.Getpriobyfiles(true, &m.Episodes[idx].Num1, true, m.Priority, cfgquality)
		if m.Priority > oldPrio || oldPrio == 0 {
			o.Oldfiles = append(o.Oldfiles, getoldfiles...)
			//oldfiles = append(oldfiles, getoldfiles...)
			bl = true
			clear(getoldfiles)
		} else {
			clear(getoldfiles)
			database.Scanrows1dyn(false, "select count() from serie_episode_files where serie_episode_id = ?", &m.TempID, &m.Episodes[idx].Num1)
			if m.TempID == 0 {
				//if database.Getdatarow1[int](false, "select count() from serie_episode_files where serie_episode_id = ?", &tblepi[idx].Num1) == 0 {
				bl = true
			} else if !skipdelete {
				if bl, err = scanner.RemoveFile(o.Videofile); err == nil && bl {
					logger.LogDynamicany("info", "Lower Qual Import File removed", &logger.StrPath, &o.Videofile, "old prio", &oldPrio, &logger.StrPriority, &m.Priority)
					s.removeotherfiles(o)
					_ = s.cleanUpFolder(o)
					bl = false
					break
				} else if err != nil {
					logger.LogDynamicany("error", "delete Files", err)
				}
				bl = false
			}
		}
		newtbl = append(newtbl, m.Episodes[idx])
	}

	if !bl {
		clear(m.Episodes)
		m.Episodes = m.Episodes[:0]
		return logger.ErrNotAllowed
	}
	m.Episodes = newtbl
	return nil
}

// removeotherfiles removes any other allowed file extensions
// associated with the video file in orgadata. It loops through
// the configured allowed extensions and calls RemoveFile on the
// same filename with that extension.
func (s *Organizer) removeotherfiles(o *Organizerdata) {
	fileext := filepath.Ext(o.Videofile)
	for idx := range s.sourcepathCfg.AllowedOtherExtensions {
		if fileext == s.sourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		_, _ = scanner.RemoveFile(strings.ReplaceAll(o.Videofile, fileext, s.sourcepathCfg.AllowedOtherExtensions[idx]))
	}
}

// OrganizeSingleFolder walks the given folder to find media files, parses them to get metadata,
// checks that metadata against the database, and moves/renames files based on the config.
// It applies various filters for unsupported files, errors, etc.
// This handles the main logic for processing a single folder.
func OrganizeSingleFolder(folder string, cfgp *config.MediaTypeConfig, data *config.MediaDataImportConfig, defaulttemplate string, checkruntime bool, deleteWrongLanguage bool, manualid uint) {
	//processfolders := make([]string, 0, 500)
	s := NewStructure(cfgp, data, data.TemplatePath, defaulttemplate, checkruntime, deleteWrongLanguage, manualid)
	if s == nil {
		logger.LogDynamicany("error", "structure not found", &logger.StrConfig, &data.TemplatePath)
		return
	}
	defer s.Close()
	_ = filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() || filepath.Ext(info.Name()) == "" {
			return nil
		}

		if logger.ContainsI(fpath, "_unpack") {
			logger.LogDynamicany("warn", "skipped - unpacking", &logger.StrFile, fpath) //logpointerr
			return fs.SkipDir
		}
		if logger.SlicesContainsPart2I(s.sourcepathCfg.Disallowed, fpath) {
			logger.LogDynamicany("warn", "skipped - disallowed", &logger.StrFile, fpath) //logpointerr
			return fs.SkipDir
		}

		//CheckUnmatched
		if config.SettingsGeneral.UseFileCache {
			if database.SlicesCacheContains(cfgp.Useseries, logger.CacheUnmatched, fpath) {
				return nil
			}
		} else {
			if database.Getdatarow1Map[uint](false, cfgp.Useseries, logger.DBCountUnmatchedPath, fpath) >= 1 { //sqlpointerr
				return nil
			}
		}
		ok, _ := scanner.CheckExtensions(true, false, s.sourcepathCfg, filepath.Ext(info.Name()))

		//Check IgnoredPaths

		if ok && s.sourcepathCfg.BlockedLen >= 1 {
			if logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
				return nil
			}
		}
		if !ok {
			return nil
		}

		m := parser.ParseFile(fpath, true, true, cfgp, -1)
		if m == nil {
			logger.LogDynamicany("error", "parse failed", &logger.StrFile, fpath) //logpointerr
			return nil
		}
		defer m.Close()

		err := parser.GetDBIDs(m, cfgp, true)
		if err != nil {
			//m.TempTitle = fpath
			m.LogTempTitle("warn", logger.ParseFailedIDs, err, fpath)
			//m.AddUnmatched(cfgp, &logger.Strstructure)
			return nil
		}

		if !s.hasValidIDs(m) {
			//m.TempTitle = fpath
			m.LogTempTitleNoErr("warn", "skipped - no valid IDs", fpath)
			return nil
		}

		if s.sourcepathCfg.MinVideoSize > 0 {
			if s.checkrecent(fpath) {
				return fs.SkipDir
			}
		}
		if m.ListID == -1 {
			m.ListID = s.getMediaListID(m)

			if m.ListID == -1 {
				//m.TempTitle = fpath
				m.LogTempTitleNoErr("warn", "listcfg not found", fpath)
				return nil
			}
		}
		// if s.Cfgp.Lists[m.ListID].CfgQuality == nil {
		// 	//m.TempTitle = fpath
		// 	m.LogTempTitleNoErr("warn", "quality not found", &logger.StrFile, fpath)
		// 	return nil
		// }
		if config.SettingsGeneral.UseFileCache {
			database.SlicesCacheContainsDelete(logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheUnmatched), fpath)
		}

		if s.Cfgp.Useseries && m.DbserieEpisodeID != 0 && m.DbserieID != 0 && m.SerieEpisodeID != 0 && m.SerieID != 0 {
			if m.Checktitle(true, s.Cfgp.Lists[m.ListID].CfgQuality) {
				//m.TempTitle = fpath
				m.LogTempTitleNoErr("warn", "skipped - unwanted title", fpath)
				return nil
			}
			o := Organizerdata{Folder: folder, Videofile: fpath, Listid: m.ListID}
			defer o.Close()
			if s.checksubfiles(&o) {
				logger.LogDynamicany("error", "check sub files", &logger.StrFile, &o.Videofile)
				return nil
			}
			m.SerieID = getmediaid(m.SerieID, s.manualId)
			err = s.organizeSeries(&o, m, s.Cfgp.Lists[m.ListID].CfgQuality, s.deletewronglanguage, s.checkruntime)
			if err != nil {
				m.LogTempTitle("error", "structure", err, fpath)
			} else {
				s.cleanUpFolder(&o)
			}
		} else if !s.Cfgp.Useseries && m.MovieID != 0 && m.DbmovieID != 0 {
			if m.Checktitle(false, s.Cfgp.Lists[m.ListID].CfgQuality) {
				//m.TempTitle = fpath
				m.LogTempTitleNoErr("warn", "skipped - unwanted title", fpath)
				return nil
			}
			o := Organizerdata{Folder: folder, Videofile: fpath, Listid: m.ListID}
			defer o.Close()
			if s.checksubfiles(&o) {
				logger.LogDynamicany("error", "check sub files", &logger.StrFile, &o.Videofile)
				return nil
			}

			m.MovieID = getmediaid(m.MovieID, s.manualId)
			err = s.organizeMovie(&o, m, s.Cfgp.Lists[m.ListID].CfgQuality, s.deletewronglanguage, s.checkruntime)
			if err != nil {
				m.LogTempTitle("error", "structure", err, fpath)
			} else {
				s.cleanUpFolder(&o)
			}
		}

		return nil
	})
}

// checkrecent checks if a file is too small or has been modified too recently.
// It logs a warning if the file is too small, and optionally removes the file.
// It also logs an error if the file has been modified too recently.
// The function returns true if the file should be skipped, false otherwise.
func (s *Organizer) checkrecent(fpath string) bool {
	info, err := os.Stat(fpath)
	if err == nil {
		if info.Size() < s.sourcepathCfg.MinVideoSizeByte {
			logger.LogDynamicany("warn", "skipped - small files", &logger.StrFile, fpath) //logpointerr
			if s.sourcepathCfg.Name == "" {
				return true
			}
			ok, oknorename := scanner.CheckExtensions(true, false, s.sourcepathCfg, filepath.Ext(fpath))

			if ok || oknorename || (s.sourcepathCfg.AllowedVideoExtensionsLen == 0 && s.sourcepathCfg.AllowedVideoExtensionsNoRenameLen == 0) {
				scanner.SecureRemove(fpath)
			}
			return true
		}
		if info.ModTime().After(logger.TimeGetNow().Add(-2 * time.Minute)) {
			logger.LogDynamicany("error", "file modified too recently", &logger.StrFile, fpath) //logpointerr
			return true
		}
	}
	return false
}

// checksubfiles checks for any disallowed subtitle files in the same
// folder as the video file. It also checks if there are multiple files
// with the same extension, which indicates it may not be a standalone movie.
// It returns an error if disallowed files are found or too many matching files exist.
func (s *Organizer) checksubfiles(o *Organizerdata) bool {
	var disallowed bool
	var count int8
	ext := filepath.Ext(o.Videofile)
	_ = filepath.WalkDir(o.Folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(info.Name()), ext) {
			if count < 2 {
				count++
			}
			if !s.Cfgp.Useseries && count >= 2 {
				return filepath.SkipAll
			}
		}
		if logger.SlicesContainsPart2I(s.sourcepathCfg.Disallowed, fpath) {
			disallowed = true
			return filepath.SkipAll
		}
		return nil
	})
	if disallowed {
		if s.sourcepathCfg.DeleteDisallowed && o.Videofile != s.sourcepathCfg.Path {
			_ = s.fileCleanup(o)
		}
		logger.LogDynamicany("warn", "skipped - disallowed", &logger.StrFile, &o.Videofile)
		return true
	}
	if !s.Cfgp.Useseries && count >= 2 {
		logger.LogDynamicany("warn", "skipped - too many files", &logger.StrFile, &o.Videofile)
		return true
	}
	return false
}

// hasValidIDs checks if the parser has valid IDs for the media item.
// For series it checks episode ID and series ID.
// For movies it checks movie ID.
// It uses the config to determine if this is a series or movie.
func (s *Organizer) hasValidIDs(m *database.ParseInfo) bool {
	if s.Cfgp.Useseries {
		return m.DbserieEpisodeID != 0 && m.DbserieID != 0 && m.SerieEpisodeID != 0 && m.SerieID != 0
	}
	return m.MovieID != 0 && m.DbmovieID != 0
}

// getMediaListID returns the media list ID for the given media item.
// For series, it looks up the list ID based on the series ID.
// For movies, it looks up the list ID based on the movie ID.
// It uses the config to determine if the media type is series or movie.
func (s *Organizer) getMediaListID(m *database.ParseInfo) int8 {
	if m.ListID != -1 {
		return m.ListID
	}
	if s.Cfgp.Useseries {
		return database.GetMediaListIDGetListname(s.Cfgp, &m.SerieID)
	}
	return database.GetMediaListIDGetListname(s.Cfgp, &m.MovieID)
}

// Close closes the Organizer, saving its internal state.
func (s *Organizer) Close() {
	if s == nil {
		return
	}
	// s.cfgImport = nil
	// s.sourcepathCfg = nil
	// s.targetpathCfg = nil
	// *s = Organizer{}
	plStructure.Put(s)
}

// Close closes the Organizer, saving its internal state.
func (s *inputNotifier) close() {
	if s == nil {
		return
	}
	s.Source = nil
	clear(s.Replaced)
	s.Replaced = nil
	*s = inputNotifier{}
}

// NewStructure initializes a new Organizer instance for organizing media
// files based on the provided configuration. It returns nil if structure
// organization is disabled or the config is invalid.
func NewStructure(cfgp *config.MediaTypeConfig, cfgimport *config.MediaDataImportConfig, sourcepathstr, targetpathstr string, checkruntime bool, deletewronglanguage bool, manualid uint) *Organizer {
	if cfgp == nil || !cfgp.Structure {
		logger.LogDynamicany("error", "parse failed cfgp", &logger.ErrCfgpNotFound, &logger.StrFile, sourcepathstr)
		return nil
	}
	if config.SettingsPath[sourcepathstr] == nil {
		logger.LogDynamicany("error", "structure source not found", &logger.StrConfig, sourcepathstr)
		return nil
	}
	if config.SettingsPath[targetpathstr] == nil {
		logger.LogDynamicany("error", "structure target not found", &logger.StrConfig, targetpathstr)
		return nil
	}

	if config.SettingsPath[sourcepathstr].Name == "" {
		logger.LogDynamicany("error", "template "+config.SettingsPath[sourcepathstr].Name+" not found", &logger.StrFile, sourcepathstr)
		return nil
	}
	o := plStructure.Get()
	o.cfgImport = cfgimport
	o.checkruntime = checkruntime
	o.deletewronglanguage = deletewronglanguage
	o.manualId = manualid
	o.sourcepathCfg = config.SettingsPath[sourcepathstr]
	o.targetpathCfg = config.SettingsPath[targetpathstr]
	o.Cfgp = cfgp
	return o
	//return &Organizer{cfgImport: cfgimport, checkruntime: checkruntime, deletewronglanguage: deletewronglanguage, manualId: manualid, sourcepathCfg: config.SettingsPath[sourcepathstr], targetpathCfg: config.SettingsPath[targetpathstr], Cfgp: cfgp}
}

// checksplit checks if the given folder name contains a '/' or '\'
// path separator and returns the detected separator byte.
// It is used to determine the path separator used in a folder name.
func checksplit(foldername string) byte {
	if strings.ContainsRune(foldername, '/') {
		return '/'
	} else if strings.ContainsRune(foldername, '\\') {
		return '\\'
	}
	return ' '
}

// UpdateRootpath updates the rootpath column in the database for the given object type and ID.
// It searches through the provided config to find which path the given file is under.
// It then extracts the first folder from the relative path of the file to that config path.
// It joins that folder with the config path to form the new rootpath value.
// Finally it executes a SQL update statement to update the rootpath for that object ID.
func UpdateRootpath(file string, objtype string, objid *uint, cfgp *config.MediaTypeConfig) {
	for idxdata := range cfgp.Data {
		if !logger.ContainsI(file, cfgp.Data[idxdata].CfgPath.Path) {
			continue
		}
		firstfolder := strings.TrimLeft(strings.ReplaceAll(file, cfgp.Data[idxdata].CfgPath.Path, ""), "/\\")
		if strings.ContainsRune(firstfolder, '/') || strings.ContainsRune(firstfolder, '\\') {
			firstfolder = filepath.Dir(firstfolder)
		}
		firstfolder = filepath.Join(cfgp.Data[idxdata].CfgPath.Path, getrootpath(firstfolder))
		database.ExecN(logger.JoinStrings("update ", objtype, " set rootpath = ? where id = ?"), &firstfolder, objid)
		return
	}
}

// Getrootpath returns the root path of the given folder name by splitting on '/' or '\'
// and trimming any trailing slashes. If no slashes, it just trims trailing slashes.
func getrootpath(foldername string) string {
	if !strings.ContainsRune(foldername, '/') && !strings.ContainsRune(foldername, '\\') {
		return strings.Trim(foldername, "/")
	}
	splitby := '/'
	if !strings.ContainsRune(foldername, '/') {
		splitby = '\\'
	}
	idx := strings.IndexRune(foldername, splitby)
	if idx != -1 {
		foldername = foldername[:idx]
	}
	//foldername = SplitBy(foldername, splitby)
	if foldername != "" && foldername[len(foldername)-1:] == "/" {
		return strings.TrimRight(foldername, "/")
	}

	return foldername
}

// Getmediaid returns the manual media ID if it is non-zero, otherwise returns the regular media ID.
// Used to allow manually overriding the auto-detected media ID.
func getmediaid(id uint, manualid uint) uint {
	if manualid != 0 {
		return manualid
	}
	return id
}

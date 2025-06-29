package structure

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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

// Organizer struct contains configuration and path information for organizing media files.
type Organizer struct {
	// ManualId is a unit containing a manually set ID
	manualID uint
	// Checkruntime is a boolean indicating whether to check runtime during organization
	checkruntime bool
	// Deletewronglanguage is a boolean indicating whether to delete wrong language files
	deletewronglanguage bool
	// Cfgp is a pointer to the MediaTypeConfig
	Cfgp *config.MediaTypeConfig
	// CfgImport is a pointer to the MediaDataImportConfig
	// gcfgImport *config.MediaDataImportConfig
	// SourcepathCfg is a pointer to the PathsConfig for the source path
	sourcepathCfg *config.PathsConfig
	// TargetpathCfg is a pointer to the PathsConfig for the target path
	targetpathCfg *config.PathsConfig
	// orgadata Organizerdata
}

type parsertype struct {
	Dbmovie            database.Dbmovie
	Dbserie            database.Dbserie
	DbserieEpisode     database.DbserieEpisode
	TitleSource        string
	EpisodeTitleSource string
	Identifier         string
	Episodes           []int
	Source             *database.ParseInfo
}

// inputNotifier contains information about the input file being processed.
type inputNotifier struct {
	// Dbmovie is the Dbmovie struct for movies
	Dbmovie database.Dbmovie
	// Dbserie is the Dbserie struct for TV series
	Dbserie database.Dbserie
	// DbserieEpisode is the DbserieEpisode struct for TV series episodes
	DbserieEpisode database.DbserieEpisode
	// Replaced is a list of replaced strings during processing
	Replaced []string
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
	// ReplacedPrefix is the prefix used for replaced strings
	ReplacedPrefix string
	// Time is the timestamp string
	Time string
	// Source is the ParseInfo struct containing parsing info
	Source *database.ParseInfo
}

// Organizerdata struct contains data for organizing media files.
type Organizerdata struct {
	Oldfiles []string
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
	// Folder is the folder path
	Folder string
	// Listid is the list ID
	Listid int
}

var (
	strOldPrio              = "old prio"
	errRuntime              = errors.New("wrong runtime")
	errLowerQuality         = errors.New("lower quality")
	errSeasonEmpty          = errors.New("season empty")
	errNotFoundPathTemplate = errors.New("path template not found")
	errGeneratingFilename   = errors.New("generating filename")
	errWrongRuntime         = errors.New("wrong runtime")
	errWrongLanguage        = errors.New("wrong language")
	// namingReplacer replaces multiple spaces and brackets in strings.
	namingReplacer = strings.NewReplacer(
		"  ",
		" ",
		" ]",
		"]",
		"[ ",
		"[",
		"[]",
		"",
		"( )",
		"",
		"()",
		"",
	)
	plStructure = pool.NewPool(20, 3, nil, func(o *Organizer) bool {
		*o = Organizer{}
		return false
	})
)

// Clear resets the fields of an Organizerdata struct to their zero values.
// This is useful for clearing the state of an Organizerdata instance.
func (p *Organizerdata) Clear() {
	if p != nil && p.Folder != "" {
		p.Oldfiles = p.Oldfiles[:0]
		// clear(p.Oldfiles)
	}
}

// FileCleanup removes the video file and cleans up the folder for the given Organizerdata.
// It handles both series and non-series files.
func (s *Organizer) fileCleanup(folder, videofile, rootpath string) error {
	if videofile == "" {
		return s.cleanUpFolder(folder)
	}
	removed, err := scanner.RemoveFile(videofile)
	if err != nil {
		return err
	}
	if !removed {
		return nil
	}
	if !s.Cfgp.Useseries {
		if s.sourcepathCfg.Name == "" {
			return errNotFoundPathTemplate
		}

		if !scanner.CheckFileExist(folder) {
			return logger.ErrNotFound
		}
		s.walkcleanup(rootpath, "", "", true)
	} else {
		s.removeotherfiles(videofile)
	}
	return s.cleanUpFolder(folder)
}

// cleanUpFolder walks the given folder path to calculate total size.
// It then compares total size to given cleanup threshold in MB.
// If folder size is less than threshold, folder is deleted.
// Returns any error encountered.
func (s *Organizer) cleanUpFolder(folder string) error {
	if !scanner.CheckFileExist(folder) {
		return errors.New("cleanup folder not found")
	}
	var leftsize int64

	cleanupsizeByte := int64(s.sourcepathCfg.CleanupsizeMB) * 1024 * 1024 // MB to Byte

	err := filepath.WalkDir(folder, func(_ string, info fs.DirEntry, errw error) error {
		if errw != nil || info.IsDir() {
			return errw
		}
		if cleanupsizeByte <= leftsize {
			return filepath.SkipAll
		}
		if fsinfo, err := info.Info(); err == nil {
			leftsize += fsinfo.Size()
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.Logtype("debug", 0).Int64(logger.StrSize, leftsize).Msg("Left size")
	if cleanupsizeByte >= leftsize || leftsize == 0 {
		filepath.WalkDir(folder, func(fpath string, _ fs.DirEntry, errw error) error {
			if errw != nil {
				return errw
			}
			os.Chmod(fpath, 0o777)
			return nil
		})
		if err := os.RemoveAll(folder); err != nil {
			return err
		}
		logger.LogDynamicany1String("info", "Folder removed", logger.StrFile, folder)
	}
	return nil
}

// ParseFileAdditional performs additional parsing and validation on a video file.
// It checks the runtime, language, and quality against configured values and cleans up the file if needed.
// It is used after initial parsing to enforce business logic around file properties.
func (s *Organizer) ParseFileAdditional(
	o *Organizerdata,
	m *database.ParseInfo,
	runtime uint,
	deletewronglanguage, checkruntime bool,
	cfgQuality *config.QualityConfig,
) error {
	if o.Listid == -1 {
		return logger.ErrListnameTemplateEmpty
	}

	parser.GetPriorityMapQual(m, s.Cfgp, cfgQuality, true, true)
	m.File = o.Videofile
	if err := parser.ParseVideoFile(m, cfgQuality); err != nil {
		if errors.Is(err, logger.ErrTracksEmpty) {
			s.fileCleanup(o.Folder, o.Videofile, o.Rootpath)
		}
		return err
	}
	// Runtime validation
	if err := s.validateRuntime(m, runtime, checkruntime, o); err != nil {
		return err
	}
	// Language validation
	return s.validateLanguage(m, deletewronglanguage, o)
}

// validateRuntime checks if the runtime matches expected values.
func (s *Organizer) validateRuntime(
	m *database.ParseInfo,
	runtime uint,
	checkruntime bool,
	o *Organizerdata,
) error {
	if m.Runtime < 1 || !checkruntime || runtime == 0 {
		return nil
	}

	wantedruntime := int(runtime)
	if s.Cfgp.Useseries {
		wantedruntime *= len(m.Episodes)
	}

	targetruntime := m.Runtime / 60
	if targetruntime == wantedruntime {
		return nil
	}

	if s.targetpathCfg.MaxRuntimeDifference == 0 {
		logger.LogDynamicany2Int(
			"warning",
			"wrong runtime",
			logger.StrWanted,
			wantedruntime,
			logger.StrFound,
			targetruntime,
		)
		return errWrongRuntime
	}

	maxdifference := s.targetpathCfg.MaxRuntimeDifference
	if m.Extended && !s.Cfgp.Useseries {
		maxdifference += 10
	}

	difference := abs(wantedruntime - targetruntime)
	if difference > maxdifference {
		if s.targetpathCfg.DeleteWrongRuntime {
			s.fileCleanup(o.Folder, o.Videofile, o.Rootpath)
		}
		logger.LogDynamicany2Int(
			"warning",
			"wrong runtime",
			logger.StrWanted,
			wantedruntime,
			logger.StrFound,
			targetruntime,
		)
		return errWrongRuntime
	}
	return nil
}

// Helper function for absolute value.
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// validateLanguage checks if the language is allowed.
func (s *Organizer) validateLanguage(
	m *database.ParseInfo,
	deletewronglanguage bool,
	o *Organizerdata,
) error {
	if !deletewronglanguage || s.targetpathCfg.AllowedLanguagesLen == 0 {
		return nil
	}

	lenlang := len(m.Languages)
	for _, allowedLang := range s.targetpathCfg.AllowedLanguages {
		if (lenlang == 0 && allowedLang == "") || logger.SlicesContainsI(m.Languages, allowedLang) {
			return nil
		}
	}

	// Language not allowed
	if deletewronglanguage {
		if err := s.fileCleanup(o.Folder, o.Videofile, o.Rootpath); err != nil {
			logger.LogDynamicany(
				"warning",
				"wrong language",
				err,
				&logger.StrWanted,
				&s.targetpathCfg.AllowedLanguages[0],
				&logger.StrFound,
				&m.Languages[0],
			)
			return errWrongLanguage
		}
	}

	foundLang := ""
	if len(m.Languages) > 0 {
		foundLang = m.Languages[0]
	}
	logger.LogDynamicany2StrAny(
		"warning",
		"wrong language",
		logger.StrFound,
		foundLang,
		logger.StrWanted,
		&s.targetpathCfg.AllowedLanguages[0],
	)
	return errWrongLanguage
}

// TrimStringInclAfterString truncates the given string s after the first
// occurrence of the search string. It returns the truncated string.
func trimStringInclAfterString(s, search string) string {
	if idx := logger.IndexI(s, search); idx != -1 {
		return s[:idx]
	}
	return s
}

// StringRemoveAllRunes removes all occurrences of the rune r from s.
func stringRemoveAllRunes(s string, r byte) string {
	if s == "" || !strings.ContainsRune(s, rune(r)) {
		return s
	}
	out := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(out)
	for i := 0; i < len(s); i++ {
		if r != s[i] {
			out.WriteByte(s[i])
		}
	}
	return out.String()
}

// GenerateNamingTemplate generates the folder name and file name for a movie or TV show file
// based on the configured naming template. It looks up metadata from the database and parses
// the naming template to replace placeholders with actual values. It handles movies and shows
// differently based on the UseSeries config option.
func (s *Organizer) GenerateNamingTemplate(o *Organizerdata, m *database.ParseInfo, dbid *uint) {
	forparser := parsertype{Source: m}
	var bl bool
	o.Foldername, o.Filename = logger.SplitByLR(s.Cfgp.Naming, checksplit(s.Cfgp.Naming))
	if !s.Cfgp.Useseries {
		if forparser.Dbmovie.GetDbmovieByIDP(dbid) != nil {
			return
		}
		logger.Path(&forparser.Dbmovie.Title, false)
		forparser.TitleSource = filepath.Base(o.Videofile)
		forparser.TitleSource = trimStringInclAfterString(
			forparser.TitleSource,
			forparser.Source.Quality,
		)
		forparser.TitleSource = trimStringInclAfterString(
			forparser.TitleSource,
			forparser.Source.Resolution,
		)
		if forparser.Source.Year != 0 {
			idx := strings.Index(forparser.TitleSource, logger.IntToString(forparser.Source.Year))
			if idx != -1 {
				forparser.TitleSource = forparser.TitleSource[:idx]
			}
		}
		forparser.TitleSource = logger.Trim(forparser.TitleSource, '.')

		logger.Path(&forparser.TitleSource, false)

		logger.StringReplaceWithP(&forparser.TitleSource, '.', ' ')

		if forparser.Dbmovie.Title == "" {
			database.Scanrowsdyn(
				false,
				database.QueryDbmovieTitlesGetTitleByIDLmit1,
				&forparser.Dbmovie.Title,
				dbid,
			)
			if forparser.Dbmovie.Title == "" {
				forparser.Dbmovie.Title = forparser.TitleSource
			}
		}
		if forparser.Dbmovie.Year == 0 {
			forparser.Dbmovie.Year = forparser.Source.Year
		}

		if o.Rootpath != "" {
			_, getfoldername := logger.SplitByLR(o.Rootpath, checksplit(o.Rootpath))
			if getfoldername != "" {
				o.Foldername = "" // getfoldername
			}
		}

		if forparser.Source.Imdb == "" {
			forparser.Source.Imdb = forparser.Dbmovie.ImdbID
		}
		if forparser.Source.Imdb != "" {
			forparser.Source.Imdb = logger.AddImdbPrefix(forparser.Source.Imdb)
		}

		forparser.Source.Title = stringRemoveAllRunes(forparser.Source.Title, '/')
		logger.Path(&forparser.Source.Title, false)
	} else {
		// Naming Series
		database.Scanrowsdyn(false, database.QuerySerieEpisodesGetDBSerieIDByID, &forparser.Dbserie.ID, dbid)
		database.Scanrowsdyn(false, database.QuerySerieEpisodesGetDBSerieEpisodeIDByID, &forparser.DbserieEpisode.ID, dbid)
		if forparser.DbserieEpisode.ID == 0 || forparser.Dbserie.ID == 0 || forparser.Dbserie.GetDbserieByIDP(&forparser.Dbserie.ID) != nil || forparser.DbserieEpisode.GetDbserieEpisodesByIDP(&forparser.DbserieEpisode.ID) != nil {
			return
		}

		episodetitle := database.Getdatarow[string](false, "select title from dbserie_episodes where id = ?", &m.Episodes[0].Num2)
		serietitle := database.Getdatarow[string](false, "select seriename from dbseries where id = ?", &m.DbserieID)
		if (serietitle == "" || episodetitle == "") && m.Identifier != "" {
			serietitleparse, episodetitleparse := database.RegexGetMatchesStr1Str2(false, logger.JoinStrings(`^(.*)(?i)`, m.Identifier, `(?:\.| |-)(.*)$`), filepath.Base(o.Videofile))
			if serietitle != "" && episodetitleparse != "" {
				logger.StringReplaceWithP(&episodetitleparse, '.', ' ')

				episodetitleparse = trimStringInclAfterString(episodetitleparse, "XXX")
				episodetitleparse = trimStringInclAfterString(episodetitleparse, m.Quality)
				episodetitleparse = trimStringInclAfterString(episodetitleparse, m.Resolution)
				episodetitleparse = logger.Trim(episodetitleparse, '.', ' ')

				serietitleparse = logger.Trim(serietitleparse, '.')

				logger.StringReplaceWithP(&serietitleparse, '.', ' ')
			}

			if episodetitle == "" {
				episodetitle = episodetitleparse
			}
			if serietitle == "" {
				serietitle = serietitleparse
			}
		}
		if forparser.Dbserie.Seriename == "" {
			database.Scanrowsdyn(false, "select title from dbserie_alternates where dbserie_id = ?", &forparser.Dbserie.Seriename, &forparser.Dbserie.ID)
			if forparser.Dbserie.Seriename == "" {
				forparser.Dbserie.Seriename = serietitle
			}
		}
		logger.StringRemoveAllRunesP(&forparser.Dbserie.Seriename, '/')

		if forparser.DbserieEpisode.Title == "" {
			forparser.DbserieEpisode.Title = episodetitle
		}
		logger.StringRemoveAllRunesP(&forparser.DbserieEpisode.Title, '/')

		logger.Path(&forparser.Dbserie.Seriename, false)
		logger.Path(&forparser.DbserieEpisode.Title, false)
		if o.Rootpath != "" {
			_, getfoldername := logger.SplitByLR(o.Rootpath, checksplit(o.Rootpath))
			if getfoldername != "" {
				if database.Getdatarow[string](false, database.QueryDbseriesGetIdentifiedByID, &m.DbserieID) == "date" {
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

		forparser.Episodes = make([]int, len(m.Episodes))
		for idx := range m.Episodes {
			database.Scanrowsdyn(false, "select episode from dbserie_episodes where id = ? and episode != ''", &forparser.Episodes[idx], &m.Episodes[idx].Num2)
		}
		forparser.TitleSource = serietitle
		logger.Path(&forparser.TitleSource, false)
		logger.StringRemoveAllRunesP(&forparser.TitleSource, '/')

		forparser.EpisodeTitleSource = episodetitle
		logger.Path(&forparser.EpisodeTitleSource, false)
		logger.StringRemoveAllRunesP(&forparser.EpisodeTitleSource, '/')

		if forparser.Source.Tvdb == "0" || forparser.Source.Tvdb == "" || forparser.Source.Tvdb == "tvdb" || strings.EqualFold(forparser.Source.Tvdb, "tvdb") {
			forparser.Source.Tvdb = strconv.Itoa(forparser.Dbserie.ThetvdbID)
		}
		if forparser.Source.Tvdb != "" && len(forparser.Source.Tvdb) >= 1 && !logger.HasPrefixI(forparser.Source.Tvdb, logger.StrTvdb) {
			forparser.Source.Tvdb = (logger.StrTvdb + forparser.Source.Tvdb)
		}
	}

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
	o.Foldername = logger.Trim(o.Foldername, '.')

	logger.Path(&o.Foldername, true)
	o.Foldername = unidecode.Unidecode(o.Foldername)

	o.Filename = logger.Trim(o.Filename, '.')

	o.Filename = namingReplacer.Replace(o.Filename)
	logger.Path(&o.Filename, false)
	o.Filename = unidecode.Unidecode(o.Filename)
}

// cleanorgafilefolder clears the foldername and filename fields of the provided Organizerdata struct to empty strings.
func (p *Organizerdata) cleanorgafilefolder() {
	p.Foldername = ""
	p.Filename = ""
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

	mode := fs.FileMode(0o777)
	if s.targetpathCfg.SetChmodFolder != "" && len(s.targetpathCfg.SetChmodFolder) == 4 {
		mode = logger.StringToFileMode(s.targetpathCfg.SetChmodFolder)
	}
	err := os.MkdirAll(o.videotarget, mode)
	if err != nil {
		return "", err
	}
	if mode != 0 {
		os.Chmod(o.videotarget, mode)
	}
	return scanner.MoveFile(
		o.Videofile,
		s.sourcepathCfg,
		o.videotarget,
		o.Filename,
		scanner.MoveFileOptions{
			UseBufferCopy: config.SettingsGeneral.UseFileBufferCopy,
			Chmod:         s.targetpathCfg.SetChmod,
			ChmodFolder:   s.targetpathCfg.SetChmodFolder,
		},
	)
}

// moveRemoveOldMediaFile moves or deletes an old media file that is being
// replaced. It handles moving/deleting additional files with different
// extensions, and removing database references. This is an internal
// implementation detail not meant to be called externally.
func (s *Organizer) moveRemoveOldMediaFile(
	oldfile string,
	oldfilep *string,
	id *uint,
	move bool,
) error {
	if oldfile == "" {
		return nil
	}
	if move {
		_, err := scanner.MoveFile(
			oldfile,
			nil,
			filepath.Join(
				s.targetpathCfg.MoveReplacedTargetPath,
				filepath.Base(filepath.Dir(oldfile)),
			),
			"",
			scanner.MoveFileOptions{
				UseBufferCopy: config.SettingsGeneral.UseFileBufferCopy,
				Chmod:         s.targetpathCfg.SetChmod,
				ChmodFolder:   s.targetpathCfg.SetChmodFolder,
				UseNil:        true,
			},
		)
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
		database.SlicesCacheContainsDelete(
			logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheFiles),
			oldfile,
		)
	}
	database.ExecNMap(s.Cfgp.Useseries, logger.DBDeleteFileByIDLocation, id, oldfilep)

	fileext := filepath.Ext(oldfile)
	var err error
	var bl bool
	for idx := range s.sourcepathCfg.AllowedOtherExtensions {
		if fileext == s.sourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		additionalfile := logger.StringReplaceWithStr(
			oldfile,
			fileext,
			s.sourcepathCfg.AllowedOtherExtensions[idx],
		)
		if !scanner.CheckFileExist(additionalfile) {
			continue
		}
		if move {
			_, err = scanner.MoveFile(
				additionalfile,
				nil,
				filepath.Join(
					s.targetpathCfg.MoveReplacedTargetPath,
					filepath.Base(filepath.Dir(oldfile)),
				),
				"",
				scanner.MoveFileOptions{
					UseBufferCopy: config.SettingsGeneral.UseFileBufferCopy,
					Chmod:         s.targetpathCfg.SetChmod,
					ChmodFolder:   s.targetpathCfg.SetChmodFolder,
					UseNil:        true,
				},
			)
			if err != nil {
				if !errors.Is(err, logger.ErrNotFound) {
					logger.LogDynamicany1StringErr(
						"error",
						"file could not be moved",
						err,
						logger.StrFile,
						additionalfile,
					)
				}
				continue
			}
		} else {
			bl, err = scanner.RemoveFile(additionalfile)
			if err != nil {
				logger.LogDynamicanyErr("error", "delete Files", err)
				continue
			}
			if !bl {
				continue
			}
		}
		logger.LogDynamicany1String(
			"info",
			"Additional File removed",
			logger.StrFile,
			additionalfile,
		)
	}
	return nil
}

// OrganizeSeries organizes a downloaded series episode file by moving it to the target folder,
// updating the database, removing old lower quality files, and sending notifications.
// It takes organizer data, parsed file info, series ID, quality config, flags to delete
// wrong language and check runtime, and returns any error.
func (s *Organizer) organizeSeries(
	o *Organizerdata,
	m *database.ParseInfo,
	cfgquality *config.QualityConfig,
	deletewronglanguage, checkruntime bool,
) error {
	if m.DbserieID == 0 {
		return logger.ErrNotFoundDbserie
	}

	err := s.GetSeriesEpisodes(o, m, false, cfgquality)
	if err != nil {
		return err
	}
	if len(m.Episodes) == 0 {
		return logger.ErrNotFoundEpisode
	}

	database.GetdatarowArgs(
		"select runtime, season from dbserie_episodes where id = ?",
		&m.Episodes[0].Num2,
		&m.RuntimeStr,
		&m.SeasonStr,
	)

	identifiedby := database.Getdatarow[string](
		false,
		database.QueryDbseriesGetIdentifiedByID,
		&m.DbserieID,
	)
	if (m.RuntimeStr == "" || m.RuntimeStr == "0") && identifiedby != "date" {
		database.Scanrowsdyn(
			false,
			"select runtime from dbseries where id = ?",
			&m.RuntimeStr,
			&m.DbserieID,
		)
		if (m.RuntimeStr == "" || m.RuntimeStr == "0") && checkruntime && identifiedby != "date" {
			return errRuntime
		}
	}
	if m.SeasonStr == "" && identifiedby != "date" {
		return errSeasonEmpty
	}

	var runtime uint
	if m.RuntimeStr != "" && m.RuntimeStr != "0" {
		getrun, err := strconv.Atoi(m.RuntimeStr)
		if err == nil {
			runtime = uint(getrun)
		}
	}
	if identifiedby == "date" ||
		database.Getdatarow[bool](
			false,
			"select ignore_runtime from serie_episodes where id = ?",
			&m.Episodes[0].Num1,
		) {
		runtime = 0
	}

	err = s.ParseFileAdditional(o, m, runtime, deletewronglanguage, checkruntime, cfgquality)
	if err != nil {
		return err
	}

	s.GenerateNamingTemplate(o, m, &m.Episodes[0].Num1)
	if o.Filename == "" {
		return errGeneratingFilename
	}

	if s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "" &&
		len(o.Oldfiles) >= 1 {
		// Move old files to replaced folder
		err = s.moveremoveoldfiles(o, false, &m.SerieID, true, o.Oldfiles)
		if err != nil {
			return err
		}
	}

	if s.targetpathCfg.Usepresort && s.targetpathCfg.PresortFolderPath != "" {
		o.Rootpath = filepath.Join(s.targetpathCfg.PresortFolderPath, o.Foldername)
	}
	// Move new files to target folder
	newpath, err := s.moveVideoFile(o)
	if err != nil {
		if errors.Is(err, logger.ErrNotFound) {
			return nil
		}
		return err
	}
	// Remove old files from target folder
	err = s.moveandcleanup(o, newpath, m, &m.SerieID, &m.Episodes[0].Num2, o.Oldfiles)
	if err != nil {
		return err
	}
	// updateserie

	fileext := filepath.Ext(o.Videofile)
	filebase := filepath.Base(newpath)

	var reached int
	if m.Priority >= cfgquality.CutoffPriority {
		reached = 1
	}
	for idx := range m.Episodes {
		database.ExecN(
			"insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&newpath,
			&filebase,
			&fileext,
			&cfgquality.Name,
			&m.ResolutionID,
			&m.QualityID,
			&m.CodecID,
			&m.AudioID,
			&m.Proper,
			&m.Repack,
			&m.Extended,
			&m.SerieID,
			&m.Episodes[idx].Num1,
			&m.Episodes[idx].Num2,
			&m.DbserieID,
			&m.Height,
			&m.Width,
		)

		database.ExecN(
			"update serie_episodes SET missing = 0, quality_reached = ? where id = ?",
			reached,
			m.Episodes[idx].Num1,
		)
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
func (s *Organizer) organizeMovie(
	o *Organizerdata,
	m *database.ParseInfo,
	cfgquality *config.QualityConfig,
	deletewronglanguage, checkruntime bool,
) error {
	if m.DbmovieID == 0 {
		return logger.ErrNotFoundDbmovie
	}
	database.Scanrowsdyn(
		false,
		"select runtime from dbmovies where id = ?",
		&m.RuntimeStr,
		&m.DbmovieID,
	)
	// if (m.RuntimeStr == "" || m.RuntimeStr == "0") && checkruntime {
	// 	return errRuntime
	// }
	var runtime uint
	if m.RuntimeStr != "" && m.RuntimeStr != "0" {
		getrun, err := strconv.Atoi(m.RuntimeStr)
		if err == nil {
			runtime = uint(getrun)
		}
	}
	err := s.ParseFileAdditional(o, m, runtime, deletewronglanguage, checkruntime, cfgquality)
	if err != nil {
		return err
	}

	oldpriority, oldfiles := searcher.Getpriobyfiles(
		false,
		&m.MovieID,
		true,
		m.Priority,
		cfgquality,
		true,
	)
	if oldpriority != 0 && oldpriority >= m.Priority {
		if true {
			err := s.fileCleanup(o.Folder, o.Videofile, o.Rootpath)
			if err != nil {
				return err
			}
		}
		return errLowerQuality
	}

	s.GenerateNamingTemplate(o, m, &m.DbmovieID)
	if o.Filename == "" {
		return errGeneratingFilename
	}

	if s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "" &&
		len(oldfiles) >= 1 {
		// Move old files to replaced folder
		err = s.moveremoveoldfiles(o, false, &m.MovieID, true, oldfiles)
		if err != nil {
			return err
		}
	}

	if s.targetpathCfg.Usepresort && s.targetpathCfg.PresortFolderPath != "" {
		o.Rootpath = filepath.Join(s.targetpathCfg.PresortFolderPath, o.Foldername)
	}
	// Move new files to target folder
	newpath, err := s.moveVideoFile(o)
	if err != nil {
		if errors.Is(err, logger.ErrNotFound) {
			return nil
		}
		return err
	}

	// Remove old files from target folder
	err = s.moveandcleanup(o, newpath, m, &m.MovieID, &m.DbmovieID, oldfiles)
	if err != nil {
		return err
	}

	fileext := filepath.Ext(newpath)
	filebase := filepath.Base(newpath)
	// updatemovie
	database.ExecN(
		"insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&newpath,
		&filebase,
		&fileext,
		&cfgquality.Name,
		&m.ResolutionID,
		&m.QualityID,
		&m.CodecID,
		&m.AudioID,
		&m.Proper,
		&m.Repack,
		&m.Extended,
		&m.MovieID,
		&m.DbmovieID,
		&m.Height,
		&m.Width,
	)

	var vc int
	if m.Priority >= cfgquality.CutoffPriority {
		vc = 1
	}
	database.ExecN(
		"update movies SET missing = 0, quality_reached = ? where id = ?",
		&vc,
		&m.MovieID,
	)

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
func (s *Organizer) moveandcleanup(
	o *Organizerdata,
	newfile string,
	m *database.ParseInfo,
	id, dbid *uint,
	oldfiles []string,
) error {
	// Update rootpath
	if !s.targetpathCfg.Usepresort {
		if database.Getdatarow[string](
			false,
			logger.GetStringsMap(s.Cfgp.Useseries, logger.DBRootPathFromMediaID),
			id,
		) == "" {
			// if database.GetdatarowMap[string](false, s.Cfgp.Useseries, logger.DBRootPathFromMediaID, id) == "" {
			if !s.Cfgp.Useseries {
				UpdateRootpath(o.videotarget, "movies", &m.MovieID, s.Cfgp)
			} else {
				UpdateRootpath(o.videotarget, logger.StrSeries, &m.SerieID, s.Cfgp)
			}
		}
	}
	// Update Rootpath end

	if s.targetpathCfg.Replacelower && len(oldfiles) >= 1 {
		err := s.moveremoveoldfiles(
			o,
			true,
			id,
			false,
			slices.DeleteFunc(oldfiles, func(r string) bool {
				return r == newfile
			}),
		)
		if err != nil {
			return err
		}
	}

	if !s.Cfgp.Useseries {
		// move other movie

		if s.sourcepathCfg.Name == "" {
			return errNotFoundPathTemplate
		}

		if !scanner.CheckFileExist(o.Folder) {
			return logger.ErrNotFound
		}
		s.walkcleanup(o.Rootpath, o.videotarget, o.Filename, false)
		s.notify(o, m, dbid, oldfiles)
		s.cleanUpFolder(o.Folder)
		return nil
	}
	// move other serie
	fileext := filepath.Ext(o.Videofile)
	var err error
	for idx := range s.sourcepathCfg.AllowedOtherExtensions {
		if fileext == s.sourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		also := logger.StringReplaceWithStr(
			o.Videofile,
			fileext,
			s.sourcepathCfg.AllowedOtherExtensions[idx],
		)
		_, err = scanner.MoveFile(
			also,
			s.sourcepathCfg,
			o.videotarget,
			o.Filename,
			scanner.MoveFileOptions{
				UseBufferCopy: config.SettingsGeneral.UseFileBufferCopy,
				Chmod:         s.targetpathCfg.SetChmod,
				ChmodFolder:   s.targetpathCfg.SetChmodFolder,
				UseOther:      true,
			},
		)
		if err != nil && !errors.Is(err, logger.ErrNotFound) {
			logger.LogDynamicany1StringErr("error", "file move", err, logger.StrFile, also)
		}
	}

	s.notify(o, m, dbid, oldfiles)
	s.cleanUpFolder(o.Folder)
	return nil
}

// walkcleanup recursively walks the given root path and cleans up files.
// It calls filepath.WalkDir to traverse all files under the root path.
// For each file, it checks if it should be filtered via scanner.Filterfile.
// If so, it will either remove the file or move it to the target folder,
// depending on the useremove parameter.
// Any errors during walking or moving/removing are logged.
func (s *Organizer) walkcleanup(rootpath, videotarget, filename string, useremove bool) error {
	if rootpath == "" {
		return nil
	}
	return filepath.WalkDir(rootpath, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if fpath == rootpath {
			return nil
		}
		ok, _ := scanner.CheckExtensions(false, true, s.sourcepathCfg, filepath.Ext(info.Name()))
		if !ok {
			return nil
		}
		// Check IgnoredPaths

		if s.sourcepathCfg.BlockedLen >= 1 {
			if logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
				ok = false
			}
		}
		if ok {
			if useremove {
				scanner.RemoveFile(fpath)
			} else {
				_, err := scanner.MoveFile(fpath, s.sourcepathCfg, videotarget, filename,
					scanner.MoveFileOptions{
						UseBufferCopy: config.SettingsGeneral.UseFileBufferCopy,
						Chmod:         s.targetpathCfg.SetChmod,
						ChmodFolder:   s.targetpathCfg.SetChmodFolder,
						UseOther:      true,
					})
				if err != nil && !errors.Is(err, logger.ErrNotFound) {
					logger.LogDynamicany1StringErr("error", "file move", err, logger.StrFile, fpath)
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
func (s *Organizer) moveremoveoldfiles(
	o *Organizerdata,
	usecompare bool,
	id *uint,
	move bool,
	oldfiles []string,
) error {
	var err error
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
		err = s.moveRemoveOldMediaFile(oldfiles[idx], &oldfiles[idx], id, move)
		if err != nil {
			// Continue if old cannot be moved
			logger.LogDynamicany1StringErr("error", "Move old", err, logger.StrFile, oldfiles[idx])
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
		Replaced:      oldfiles,
		Time:          logger.TimeGetNow().Format(logger.GetTimeFormat()),
	}
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

	var err error
	for idx := range s.Cfgp.Notification {
		if s.Cfgp.Notification[idx].CfgNotification == nil ||
			!strings.EqualFold(s.Cfgp.Notification[idx].Event, "added_data") {
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
			err = apiexternal.SendPushoverMessage(
				config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Apikey,
				messagetext,
				messageTitle,
				config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Recipient,
			)
			if err != nil {
				logger.LogDynamicanyErr("error", "Error sending pushover", err)
			} else {
				logger.LogDynamicany0("info", "Pushover message sent")
			}
		case "csv":
			scanner.AppendCsv(
				config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Outputto,
				messagetext,
			)
		}
	}
}

// GetSeriesEpisodes checks existing files for a series episode, determines if a new file
// should replace them based on configured quality priorities, deletes lower priority files if enabled,
// and returns the list of episode IDs that are allowed to be imported along with any deleted file paths.
func (s *Organizer) GetSeriesEpisodes(
	o *Organizerdata,
	m *database.ParseInfo,
	skipdelete bool,
	cfgquality *config.QualityConfig,
) error {
	err := m.Getepisodestoimport()
	if err != nil {
		return err
	}
	if len(m.Episodes) == 0 {
		return logger.ErrNotFoundEpisode
	}

	parser.GetPriorityMapQual(m, s.Cfgp, cfgquality, true, true)

	var bl bool
	if len(m.Episodes) == 1 {
		oldPrio, getoldfiles := searcher.Getpriobyfiles(
			true,
			&m.Episodes[0].Num1,
			true,
			m.Priority,
			cfgquality,
			true,
		)
		if m.Priority > oldPrio || oldPrio == 0 {
			o.Oldfiles = getoldfiles
			bl = true
		} else {
			if database.Getdatarow[uint](false, "select count() from serie_episode_files where serie_episode_id = ?", &m.Episodes[0].Num1) == 0 {
				bl = true
			} else if !skipdelete {
				bl, err = scanner.RemoveFile(o.Videofile)
				if err == nil && bl {
					logger.LogDynamicany3StrIntInt("info", "Lower Qual Import File removed", logger.StrPath, o.Videofile, strOldPrio, oldPrio, logger.StrPriority, m.Priority)
					s.removeotherfiles(o.Videofile)
					s.cleanUpFolder(o.Folder)
				} else if err != nil {
					logger.LogDynamicanyErr("error", "delete Files", err)
				}
				bl = false
			}
		}
		if !bl {
			clear(m.Episodes)
			m.Episodes = m.Episodes[:0]
			return logger.ErrNotAllowed
		}
		return nil
	}

	newtbl := m.Episodes[:0]
	for idx := range m.Episodes {
		oldPrio, getoldfiles := searcher.Getpriobyfiles(
			true,
			&m.Episodes[idx].Num1,
			true,
			m.Priority,
			cfgquality,
			true,
		)
		if m.Priority > oldPrio || oldPrio == 0 {
			if len(o.Oldfiles) == 0 {
				o.Oldfiles = getoldfiles
			} else {
				o.Oldfiles = append(o.Oldfiles, getoldfiles...)
			}
			bl = true
		} else {
			if database.Getdatarow[uint](false, "select count() from serie_episode_files where serie_episode_id = ?", &m.Episodes[idx].Num1) == 0 {
				bl = true
			} else if !skipdelete {
				bl, err = scanner.RemoveFile(o.Videofile)
				if err == nil && bl {
					logger.LogDynamicany3StrIntInt("info", "Lower Qual Import File removed", logger.StrPath, o.Videofile, strOldPrio, oldPrio, logger.StrPriority, m.Priority)
					s.removeotherfiles(o.Videofile)
					s.cleanUpFolder(o.Folder)
					bl = false
					break
				}
				if err != nil {
					logger.LogDynamicanyErr("error", "delete Files", err)
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
	if len(newtbl) == len(m.Episodes) {
		return nil
	}
	m.Episodes = newtbl
	return nil
}

// removeotherfiles removes any other allowed file extensions
// associated with the video file in orgadata. It loops through
// the configured allowed extensions and calls RemoveFile on the
// same filename with that extension.
func (s *Organizer) removeotherfiles(videofile string) {
	fileext := filepath.Ext(videofile)
	for idx := range s.sourcepathCfg.AllowedOtherExtensions {
		if fileext == s.sourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		scanner.RemoveFile(
			logger.StringReplaceWithStr(
				videofile,
				fileext,
				s.sourcepathCfg.AllowedOtherExtensions[idx],
			),
		)
	}
}

// OrganizeSingleFolder walks the given folder to find media files, parses them to get metadata,
// checks that metadata against the database, and moves/renames files based on the config.
// It applies various filters for unsupported files, errors, etc.
// This handles the main logic for processing a single folder.
func OrganizeSingleFolder(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	defaulttemplate string,
	checkruntime, deleteWrongLanguage bool,
	manualid uint,
) {
	s := NewStructure(
		cfgp,
		data.TemplatePath,
		defaulttemplate,
		checkruntime,
		deleteWrongLanguage,
		manualid,
	)
	if s == nil {
		logger.LogDynamicany1String(
			"error",
			"structure not found",
			logger.StrConfig,
			data.TemplatePath,
		)
		return
	}
	defer s.Close()

	filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(info.Name()) == "" {
			return nil
		}

		if logger.ContainsI(fpath, "_unpack") {
			logger.LogDynamicany1String(
				"warn",
				"skipped - unpacking",
				logger.StrFile,
				fpath,
			) // logpointerr
			return fs.SkipDir
		}
		if logger.SlicesContainsPart2I(s.sourcepathCfg.Disallowed, fpath) {
			logger.LogDynamicany1String(
				"warn",
				"skipped - disallowed",
				logger.StrFile,
				fpath,
			) // logpointerr
			return fs.SkipDir
		}

		// CheckUnmatched
		if config.SettingsGeneral.UseFileCache {
			if database.SlicesCacheContains(cfgp.Useseries, logger.CacheUnmatched, fpath) {
				return nil
			}
		} else {
			if database.Getdatarow[uint](false, logger.GetStringsMap(cfgp.Useseries, logger.DBCountUnmatchedPath), fpath) >= 1 {
				return nil
			}
		}
		ok, _ := scanner.CheckExtensions(true, false, s.sourcepathCfg, filepath.Ext(info.Name()))

		// Check IgnoredPaths

		if !ok {
			return nil
		}
		if s.sourcepathCfg.BlockedLen >= 1 &&
			logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
			return nil
		}

		return s.walkorganizefolder(fpath, folder, cfgp)
	})
}

// walkorganizefolder is a method of the Organizer struct that processes a file path, parses the file, and organizes the media item based on the configuration settings.
// It performs various checks and validations on the file, such as checking for disallowed subtitle files, minimum video size, and valid IDs. It then updates the media item's metadata and organizes the file accordingly.
// If any errors occur during the process, it logs the errors and adds the file to the unmatched list.
func (s *Organizer) walkorganizefolder(fpath, folder string, cfgp *config.MediaTypeConfig) error {
	m := parser.ParseFile(fpath, true, true, cfgp, -1)
	if m == nil {
		logger.LogDynamicany1String("error", "parse failed", logger.StrFile, fpath) // logpointerr
		return nil
	}
	defer m.Close()
	err := parser.GetDBIDs(m, cfgp, true)
	if err != nil || !s.hasValidIDs(m) {
		logger.LogDynamicany1StringErr("warn", logger.ParseFailedIDs, err, logger.StrFile, fpath)

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, err)
		return nil
	}

	if s.sourcepathCfg.MinVideoSize > 0 {
		info, err := os.Stat(fpath)
		if err == nil {
			if info.Size() < s.sourcepathCfg.MinVideoSizeByte {
				logger.LogDynamicany1String("warn", "skipped - small files", logger.StrFile, fpath)
				if s.sourcepathCfg.Name == "" {
					m.TempTitle = fpath
					m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("small file"))
					return fs.SkipDir
				}
				ok, oknorename := scanner.CheckExtensions(
					true,
					false,
					s.sourcepathCfg,
					filepath.Ext(fpath),
				)

				if ok || oknorename ||
					(s.sourcepathCfg.AllowedVideoExtensionsLen == 0 && s.sourcepathCfg.AllowedVideoExtensionsNoRenameLen == 0) {
					scanner.SecureRemove(fpath)
				}
				m.TempTitle = fpath
				m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("small file"))
				return fs.SkipDir
			}
			if logger.TimeAfter(info.ModTime(), logger.TimeGetNow().Add(-2*time.Minute)) {
				logger.LogDynamicany1String(
					"error",
					"file modified too recently",
					logger.StrFile,
					fpath,
				)
				return fs.SkipDir
			}
		}
	}
	if s.Cfgp.Useseries {
		m.TempID = m.SerieID
	} else {
		m.TempID = m.MovieID
	}
	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(s.Cfgp, &m.TempID)

		if m.ListID == -1 {
			logger.LogDynamicany1String("warn", "listcfg not found", logger.StrFile, fpath)
			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("listcfg not found"))
			return nil
		}
	}
	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(
			logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheUnmatched),
			fpath,
		)
	}

	if s.Cfgp.Useseries &&
		(m.DbserieEpisodeID == 0 || m.DbserieID == 0 || m.SerieEpisodeID == 0 || m.SerieID == 0) {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no valid IDs"))
		return nil
	} else if !s.Cfgp.Useseries && (m.DbmovieID == 0 || m.MovieID == 0) {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no valid IDs"))
		return nil
	}
	if s.checksubfiles(folder, fpath, "") {
		logger.LogDynamicany1String("error", "check sub files", logger.StrFile, fpath)
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("check sub files"))
		return nil
	}
	if s.manualID != 0 {
		if s.Cfgp.Useseries {
			m.SerieID = s.manualID
		} else {
			m.MovieID = s.manualID
		}
	}

	var dbid uint
	var listname string
	var rootpath string
	database.GetdatarowArgs(
		logger.GetStringsMap(s.Cfgp.Useseries, "GetOrganizeData"),
		&m.TempID,
		&dbid,
		&rootpath,
		&listname,
	)
	if dbid == 0 {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no dbid found"))
		return logger.ErrNotFound
	}
	if s.Cfgp.Useseries {
		m.DbserieID = dbid
	} else {
		m.DbmovieID = dbid
	}
	if m.ListID == -1 {
		m.ListID = s.Cfgp.GetMediaListsEntryListID(listname)
	}
	if m.Checktitle(s.Cfgp, s.Cfgp.Lists[m.ListID].CfgQuality, filepath.Base(fpath)) {
		logger.LogDynamicany1String("warn", "skipped - unwanted title", logger.StrFile, fpath)
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("unwanted title"))
		return nil
	}
	if m.ListID == -1 {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no ListID found"))
		return logger.ErrListnameTemplateEmpty
	}

	o := Organizerdata{Folder: folder, Videofile: fpath, Listid: m.ListID, Rootpath: rootpath}
	if s.Cfgp.Useseries {
		err = s.organizeSeries(
			&o,
			m,
			s.Cfgp.Lists[m.ListID].CfgQuality,
			s.deletewronglanguage,
			s.checkruntime,
		)
		if err != nil {
			logger.LogDynamicany1StringErr("error", "structure", err, logger.StrFile, fpath)
			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, err)
			return nil
		}
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, fpath)
	} else {
		err = s.organizeMovie(&o, m, s.Cfgp.Lists[m.ListID].CfgQuality, s.deletewronglanguage, s.checkruntime)
		if err != nil {
			logger.LogDynamicany1StringErr("error", "structure", err, logger.StrFile, fpath)
			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, err)
			return nil
		}
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, fpath)
	}

	s.cleanUpFolder(folder)
	return nil
}

// checksubfiles checks for any disallowed subtitle files in the same
// folder as the video file. It also checks if there are multiple files
// with the same extension, which indicates it may not be a standalone movie.
// It returns an error if disallowed files are found or too many matching files exist.
func (s *Organizer) checksubfiles(folder, videofile, rootpath string) bool {
	if folder == "" {
		return false
	}
	var disallowed bool
	var count int8
	ext := filepath.Ext(videofile)
	filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
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
		if s.sourcepathCfg.DeleteDisallowed && videofile != s.sourcepathCfg.Path {
			s.fileCleanup(folder, videofile, rootpath)
		}
		logger.LogDynamicany1String("warn", "skipped - disallowed", logger.StrFile, videofile)
		return true
	}
	if !s.Cfgp.Useseries && count >= 2 {
		logger.LogDynamicany1String("warn", "skipped - too many files", logger.StrFile, videofile)
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
		return m.DbserieEpisodeID != 0 && m.DbserieID != 0 && m.SerieEpisodeID != 0 &&
			m.SerieID != 0
	}
	return m.MovieID != 0 && m.DbmovieID != 0
}

func (s *Organizer) Close() {
	plStructure.Put(s)
}

// NewStructure initializes a new Organizer instance for organizing media
// files based on the provided configuration. It returns nil if structure
// organization is disabled or the config is invalid.
func NewStructure(
	cfgp *config.MediaTypeConfig,
	sourcepathstr, targetpathstr string,
	checkruntime, deletewronglanguage bool,
	manualid uint,
) *Organizer {
	if cfgp == nil || !cfgp.Structure {
		logger.LogDynamicany1StringErr(
			"error",
			"parse failed cfgp",
			logger.ErrCfgpNotFound,
			logger.StrFile,
			sourcepathstr,
		)
		return nil
	}
	if config.SettingsPath[sourcepathstr] == nil {
		logger.LogDynamicany1String(
			"error",
			"structure source not found",
			logger.StrConfig,
			sourcepathstr,
		)
		return nil
	}
	if config.SettingsPath[targetpathstr] == nil {
		logger.LogDynamicany1String(
			"error",
			"structure target not found",
			logger.StrConfig,
			targetpathstr,
		)
		return nil
	}

	if config.SettingsPath[sourcepathstr].Name == "" {
		logger.LogDynamicany1String(
			"error",
			"template "+config.SettingsPath[sourcepathstr].Name+" not found",
			logger.StrFile,
			sourcepathstr,
		)
		return nil
	}
	o := plStructure.Get()
	o.checkruntime = checkruntime
	o.deletewronglanguage = deletewronglanguage
	o.manualID = manualid
	o.sourcepathCfg = config.SettingsPath[sourcepathstr]
	o.targetpathCfg = config.SettingsPath[targetpathstr]
	o.Cfgp = cfgp
	return o
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
func UpdateRootpath(file, objtype string, objid *uint, cfgp *config.MediaTypeConfig) {
	for _, data := range cfgp.DataMap {
		if !logger.ContainsI(file, data.CfgPath.Path) {
			continue
		}
		firstfolder := logger.TrimLeft(
			logger.StringReplaceWithStr(file, data.CfgPath.Path, ""),
			'/',
			'\\',
		)
		if strings.ContainsRune(firstfolder, '/') || strings.ContainsRune(firstfolder, '\\') {
			firstfolder = filepath.Dir(firstfolder)
		}
		firstfolder = filepath.Join(data.CfgPath.Path, getrootpath(firstfolder))
		database.ExecN(
			logger.JoinStrings("update ", objtype, " set rootpath = ? where id = ?"),
			&firstfolder,
			objid,
		)
		return
	}
}

// Getrootpath returns the root path of the given folder name by splitting on '/' or '\'
// and trimming any trailing slashes. If no slashes, it just trims trailing slashes.
func getrootpath(foldername string) string {
	if !strings.ContainsRune(foldername, '/') && !strings.ContainsRune(foldername, '\\') {
		return logger.Trim(foldername, '/')
	}
	splitby := '/'
	if !strings.ContainsRune(foldername, '/') {
		splitby = '\\'
	}
	idx := strings.IndexRune(foldername, splitby)
	if idx != -1 {
		if foldername[:idx] != "" && foldername[:idx][len(foldername[:idx])-1:] == "/" {
			return logger.TrimRight(foldername[:idx], '/')
		}

		return foldername[:idx]
	}
	if foldername != "" && foldername[len(foldername)-1:] == "/" {
		return logger.TrimRight(foldername, '/')
	}

	return foldername
}

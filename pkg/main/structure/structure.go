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
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
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
	CfgImport *config.MediaDataImportConfig
	// SourcepathCfg is a pointer to the PathsConfig for the source path
	SourcepathCfg *config.PathsConfig
	// TargetpathCfg is a pointer to the PathsConfig for the target path
	TargetpathCfg *config.PathsConfig
	// Checkruntime is a boolean indicating whether to check runtime during organization
	Checkruntime bool
	// Deletewronglanguage is a boolean indicating whether to delete wrong language files
	Deletewronglanguage bool
	// ManualId is a unit containing a manually set ID
	ManualId uint
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

type forstructurenotify struct {
	Config        *Organizer
	InputNotifier inputNotifier
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
	Listid int
	// Folder is the folder path
	Folder string
}

// namingReplacer replaces multiple spaces and brackets in strings
var namingReplacer = strings.NewReplacer("  ", " ", " ]", "]", "[ ", "[", "[]", "", "( )", "", "()", "")

// plorga is a pool for Organizerdata structs
var plorga = pool.NewPool(100, 0, func(b *Organizerdata) {}, func(b *Organizerdata) {
	*b = Organizerdata{}
})

// plstructure is a pool for Organizer structs
var plstructure = pool.NewPool(100, 0, func(b *Organizer) {}, func(b *Organizer) {
	*b = Organizer{}
})

// plparser is a pool for parsertype structs
var plparser = pool.NewPool(100, 0, func(b *parsertype) {}, func(b *parsertype) {
	clear(b.Episodes)
	*b = parsertype{}
})

// NewStructure initializes a new Organizer instance for organizing media
// files based on the provided configuration. It returns nil if structure
// organization is disabled or the config is invalid.
func NewStructure(cfgp *config.MediaTypeConfig, cfgimport *config.MediaDataImportConfig, sourcepathstr, targetpathstr string) *Organizer {
	if cfgp == nil {
		return nil
	}
	if !cfgp.Structure {
		return nil
	}
	s := plstructure.Get()
	s.Cfgp = cfgp
	s.CfgImport = cfgimport
	s.SourcepathCfg = config.SettingsPath[sourcepathstr]
	s.TargetpathCfg = config.SettingsPath[targetpathstr]
	return s
}

// FileCleanup removes the video file and cleans up the folder for the given Organizerdata.
// It handles both series and non-series files.
func (s *Organizer) FileCleanup(orgadata *Organizerdata) error {
	if !s.Cfgp.Useseries || orgadata.Videofile == "" {
		if orgadata.Videofile != "" {
			bl, err := scanner.RemoveFile(orgadata.Videofile)
			if err != nil {
				return err
			}
			if !bl {
				return nil
			}

			if s.SourcepathCfg.Name == "" {
				return errors.New("pathtemplate not found")
			}

			if !scanner.CheckFileExist(orgadata.Folder) {
				return logger.ErrNotFound
			}
			s.walkcleanup(orgadata, true)
		}
		return scanner.CleanUpFolder(orgadata.Folder, s.SourcepathCfg.CleanupsizeMB)
	}
	bl, err := scanner.RemoveFile(orgadata.Videofile)
	if err == nil && bl {
		s.removeotherfiles(orgadata)
		return scanner.CleanUpFolder(orgadata.Folder, s.SourcepathCfg.CleanupsizeMB)
	}
	return err
}

// ParseFileAdditional performs additional parsing and validation on a video file.
// It checks the runtime, language, and quality against configured values and cleans up the file if needed.
// It is used after initial parsing to enforce business logic around file properties.
func (s *Organizer) ParseFileAdditional(orgadata *Organizerdata, m *apiexternal.FileParser, deletewronglanguage bool, wantedruntime int, checkruntime bool, cfgQuality *config.QualityConfig) error {
	if orgadata.Listid == -1 {
		return errors.New("listconfig empty")
	}

	parser.GetPriorityMapQual(&m.M, s.Cfgp, cfgQuality, true, true)
	err := parser.ParseVideoFile(m, orgadata.Videofile, cfgQuality)
	if err != nil {
		if errors.Is(err, logger.ErrTracksEmpty) {
			_ = s.FileCleanup(orgadata)
		}
		return err
	}
	if m.M.Runtime >= 1 && checkruntime && wantedruntime != 0 && s.TargetpathCfg.MaxRuntimeDifference != 0 && (m.M.Runtime/60) != wantedruntime {
		maxdifference := s.TargetpathCfg.MaxRuntimeDifference
		if m.M.Extended && !s.Cfgp.Useseries {
			maxdifference += 10
		}
		var difference int
		if wantedruntime > m.M.Runtime/60 {
			difference = wantedruntime - m.M.Runtime/60
		} else {
			difference = m.M.Runtime/60 - wantedruntime
		}

		if difference > maxdifference {
			if s.TargetpathCfg.DeleteWrongRuntime {
				_ = s.FileCleanup(orgadata)
			}
			return errors.New(logger.JoinStrings("wrong runtime - wanted ", strconv.Itoa(wantedruntime), " have ", strconv.Itoa(m.M.Runtime/60)))
		}
	}
	if !deletewronglanguage || s.TargetpathCfg.AllowedLanguagesLen == 0 {
		return nil
	}
	var bl bool
	lenlang := len(m.M.Languages)
	for idx := range s.TargetpathCfg.AllowedLanguages {
		if lenlang == 0 && s.TargetpathCfg.AllowedLanguages[idx] == "" {
			bl = true
			break
		}
		if logger.SlicesContainsI(m.M.Languages, s.TargetpathCfg.AllowedLanguages[idx]) {
			bl = true
			break
		}
	}
	if !bl {
		if deletewronglanguage {
			err = s.FileCleanup(orgadata)
			if err != nil {
				return errors.New("wrong language - have " + m.M.Languages[0] + " " + err.Error())
			}
		}
		return errors.New("wrong language - have " + m.M.Languages[0])
	}
	return nil
}

// GenerateNamingTemplate generates the folder name and file name for a movie or TV show file
// based on the configured naming template. It looks up metadata from the database and parses
// the naming template to replace placeholders with actual values. It handles movies and shows
// differently based on the UseSeries config option.
func (s *Organizer) GenerateNamingTemplate(orgadata *Organizerdata, m *apiexternal.FileParser, dbid *uint, tblepi []database.DbstaticTwoUint) {
	forparser := plparser.Get()
	forparser.Source = &m.M
	defer plparser.Put(forparser)
	//forparser := parsertype{Source: &m.M}
	var bl bool
	if !s.Cfgp.Useseries {
		if database.GetDbmovieByIDP(dbid, &forparser.Dbmovie) != nil {
			return
		}
		forparser.Dbmovie.Title = logger.Path(forparser.Dbmovie.Title, false)
		forparser.TitleSource = filepath.Base(orgadata.Videofile)
		forparser.TitleSource = logger.TrimStringInclAfterString(forparser.TitleSource, forparser.Source.Quality)
		forparser.TitleSource = logger.TrimStringInclAfterString(forparser.TitleSource, forparser.Source.Resolution)
		if forparser.Source.Year != 0 {
			if idx := strings.Index(forparser.TitleSource, strconv.Itoa(forparser.Source.Year)); idx != -1 {
				forparser.TitleSource = forparser.TitleSource[:idx]
			}

			//forparser.TitleSource = logger.TrimStringInclAfterInt(forparser.TitleSource, forparser.Source.Year)
		}
		if forparser.TitleSource != "" && (forparser.TitleSource[:1] == "." || forparser.TitleSource[len(forparser.TitleSource)-1:] == ".") {
			forparser.TitleSource = strings.Trim(forparser.TitleSource, ".")
		}
		forparser.TitleSource = logger.Path(forparser.TitleSource, false)

		forparser.TitleSource = logger.StringReplaceWith(forparser.TitleSource, '.', ' ')

		if forparser.Dbmovie.Title == "" {
			_ = database.ScanrowsNdyn(false, database.QueryDbmovieTitlesGetTitleByIDLmit1, &forparser.Dbmovie.Title, dbid)
			if forparser.Dbmovie.Title == "" {
				forparser.Dbmovie.Title = forparser.TitleSource
			}
		}
		if forparser.Dbmovie.Year == 0 {
			forparser.Dbmovie.Year = forparser.Source.Year
		}

		orgadata.Foldername, orgadata.Filename = logger.SplitByLR(s.Cfgp.Naming, checksplit(s.Cfgp.Naming))

		if orgadata.Rootpath != "" {
			_, getfoldername := logger.SplitByLR(orgadata.Rootpath, checksplit(orgadata.Rootpath))
			if getfoldername != "" {
				orgadata.Foldername = "" //getfoldername
			}
		}

		if forparser.Source.Imdb == "" {
			forparser.Source.Imdb = forparser.Dbmovie.ImdbID
		}
		if forparser.Source.Imdb != "" {
			forparser.Source.Imdb = logger.AddImdbPrefix(forparser.Source.Imdb)
		}

		forparser.Source.Title = logger.Path(logger.StringRemoveAllRunes(forparser.Source.Title, '/'), false)

		bl, orgadata.Foldername = logger.ParseStringTemplate(orgadata.Foldername, &forparser)
		if bl {
			cleanorgafilefolder(orgadata)
			return
		}
		bl, orgadata.Filename = logger.ParseStringTemplate(orgadata.Filename, &forparser)
		if bl {
			cleanorgafilefolder(orgadata)
			return
		}
		if orgadata.Foldername != "" && (orgadata.Foldername[:1] == "." || orgadata.Foldername[len(orgadata.Foldername)-1:] == ".") {
			orgadata.Foldername = strings.Trim(orgadata.Foldername, ".")
		}
		orgadata.Foldername = logger.DiacriticsReplacer(orgadata.Foldername)
		orgadata.Foldername = logger.Path(orgadata.Foldername, true)
		orgadata.Foldername = unidecode.Unidecode(orgadata.Foldername)

		if orgadata.Filename != "" && (orgadata.Filename[:1] == "." || orgadata.Filename[len(orgadata.Filename)-1:] == ".") {
			orgadata.Filename = strings.Trim(orgadata.Filename, ".")
		}
		orgadata.Filename = namingReplacer.Replace(orgadata.Filename)

		orgadata.Filename = logger.DiacriticsReplacer(orgadata.Filename)
		orgadata.Filename = logger.Path(orgadata.Filename, false)
		orgadata.Filename = unidecode.Unidecode(orgadata.Filename)
		//s.generatenamingmovie(orgadata, &m.M, dbid)
		return
	}

	//Naming Series
	_ = database.ScanrowsNdyn(false, database.QuerySerieEpisodesGetDBSerieIDByID, &forparser.Dbserie.ID, dbid)
	_ = database.ScanrowsNdyn(false, database.QuerySerieEpisodesGetDBSerieEpisodeIDByID, &forparser.DbserieEpisode.ID, dbid)
	if forparser.DbserieEpisode.ID == 0 || forparser.Dbserie.ID == 0 {
		return
	}
	if database.GetDbserieByIDP(&forparser.Dbserie.ID, &forparser.Dbserie) != nil {
		return
	}
	if database.GetDbserieEpisodesByIDP(&forparser.DbserieEpisode.ID, &forparser.DbserieEpisode) != nil {
		return
	}
	orgadata.Foldername, orgadata.Filename = logger.SplitByLR(s.Cfgp.Naming, checksplit(s.Cfgp.Naming))

	episodetitle := database.GetdatarowN[string](false, "select title from dbserie_episodes where id = ?", &tblepi[0].Num2)
	serietitle := database.GetdatarowN[string](false, "select seriename from dbseries where id = ?", &m.M.DbserieID)
	if (serietitle == "" || episodetitle == "") && m.M.Identifier != "" {
		serietitleparse, episodetitleparse := database.RegexGetMatchesStr1Str2(true, `^(.*)(?i)`+m.M.Identifier+`(?:\.| |-)(.*)$`, filepath.Base(orgadata.Videofile))
		if serietitle != "" && episodetitleparse != "" {
			episodetitleparse = logger.StringReplaceWith(episodetitleparse, '.', ' ')

			episodetitleparse = logger.TrimStringInclAfterString(episodetitleparse, "XXX")
			episodetitleparse = logger.TrimStringInclAfterString(episodetitleparse, m.M.Quality)
			episodetitleparse = logger.TrimStringInclAfterString(episodetitleparse, m.M.Resolution)
			episodetitleparse = strings.Trim(episodetitleparse, ". ")

			if serietitleparse != "" && (serietitleparse[:1] == "." || serietitleparse[len(serietitleparse)-1:] == ".") {
				serietitleparse = strings.Trim(serietitleparse, ".")
			}
			serietitleparse = logger.StringReplaceWith(serietitleparse, '.', ' ')
		}

		if episodetitle == "" {
			episodetitle = episodetitleparse
		}
		if serietitle == "" {
			serietitle = serietitleparse
		}
	}
	//serietitle, episodetitle := orgadata.GetEpisodeTitle(m, &tblepi[0].Num2)
	if forparser.Dbserie.Seriename == "" {
		_ = database.ScanrowsNdyn(false, "select title from dbserie_alternates where dbserie_id = ?", &forparser.Dbserie.Seriename, &forparser.Dbserie.ID)
		if forparser.Dbserie.Seriename == "" {
			forparser.Dbserie.Seriename = serietitle
		}
	}
	forparser.Dbserie.Seriename = logger.StringRemoveAllRunes(forparser.Dbserie.Seriename, '/')

	if forparser.DbserieEpisode.Title == "" {
		forparser.DbserieEpisode.Title = episodetitle
	}
	forparser.DbserieEpisode.Title = logger.StringRemoveAllRunes(forparser.DbserieEpisode.Title, '/')

	forparser.Dbserie.Seriename = logger.Path(forparser.Dbserie.Seriename, false)
	forparser.DbserieEpisode.Title = logger.Path(forparser.DbserieEpisode.Title, false)
	if orgadata.Rootpath != "" {
		_, getfoldername := logger.SplitByLR(orgadata.Rootpath, checksplit(orgadata.Rootpath))
		if getfoldername != "" {
			splitbyget := checksplit(orgadata.Foldername)
			identifiedby := database.GetdatarowN[string](false, database.QueryDbseriesGetIdentifiedByID, &m.M.DbserieID)

			if identifiedby == "date" {
				orgadata.Foldername = ""
			} else {
				if splitbyget != ' ' {
					_, seasonname := logger.SplitByLR(orgadata.Foldername, splitbyget)
					orgadata.Foldername = seasonname
				} else {
					orgadata.Foldername = getfoldername
				}
			}
		}
	}

	forparser.Episodes = make([]int, len(tblepi))
	for idx := range tblepi {
		_ = database.ScanrowsNdyn(false, "select episode from dbserie_episodes where id = ? and episode != ''", &forparser.Episodes[idx], &tblepi[idx].Num2)
	}
	forparser.TitleSource = logger.Path(serietitle, false)
	forparser.TitleSource = logger.StringRemoveAllRunes(forparser.TitleSource, '/')

	forparser.EpisodeTitleSource = logger.Path(episodetitle, false)
	forparser.EpisodeTitleSource = logger.StringRemoveAllRunes(forparser.EpisodeTitleSource, '/')

	if forparser.Source.Tvdb == "0" || forparser.Source.Tvdb == "" || strings.EqualFold(forparser.Source.Tvdb, "tvdb") {
		forparser.Source.Tvdb = strconv.Itoa(forparser.Dbserie.ThetvdbID)
	}
	if forparser.Source.Tvdb != "" {
		if !logger.HasPrefixI(forparser.Source.Tvdb, logger.StrTvdb) {
			forparser.Source.Tvdb = logger.StrTvdb + forparser.Source.Tvdb
		}
		//forparser.Source.Tvdb = logger.AddTvdbPrefix(forparser.Source.Tvdb)
	}
	bl, orgadata.Foldername = logger.ParseStringTemplate(orgadata.Foldername, &forparser)
	if bl {
		cleanorgafilefolder(orgadata)
		return
	}
	bl, orgadata.Filename = logger.ParseStringTemplate(orgadata.Filename, &forparser)
	if bl {
		cleanorgafilefolder(orgadata)
		return
	}
	if orgadata.Foldername != "" && (orgadata.Foldername[:1] == "." || orgadata.Foldername[len(orgadata.Foldername)-1:] == ".") {
		orgadata.Foldername = strings.Trim(orgadata.Foldername, ".")
	}
	orgadata.Foldername = logger.DiacriticsReplacer(orgadata.Foldername)
	orgadata.Foldername = logger.Path(orgadata.Foldername, true)
	orgadata.Foldername = unidecode.Unidecode(orgadata.Foldername)

	if orgadata.Filename != "" && (orgadata.Filename[:1] == "." || orgadata.Filename[len(orgadata.Filename)-1:] == ".") {
		orgadata.Filename = strings.Trim(orgadata.Filename, ".")
	}

	orgadata.Filename = namingReplacer.Replace(orgadata.Filename)
	orgadata.Filename = logger.DiacriticsReplacer(orgadata.Filename)
	orgadata.Filename = logger.Path(orgadata.Filename, false)
	orgadata.Filename = unidecode.Unidecode(orgadata.Filename)
	//s.generatenamingserie(orgadata, m, dbid, tblepi)
}

// cleanorgafilefolder clears the foldername and filename fields of the provided Organizerdata struct to empty strings.
func cleanorgafilefolder(orgadata *Organizerdata) {
	orgadata.Foldername = ""
	orgadata.Filename = ""
}

// moveVideoFile moves the video file specified in orgadata to the target folder.
// It creates the target folder if needed, setting permissions according to TargetpathCfg.
// The target filename is set in orgadata.Filename.
// Returns a bool indicating if the move was successful, and an error.
func (s *Organizer) moveVideoFile(orgadata *Organizerdata) scanner.MoveResponse {
	if orgadata.Rootpath != "" {
		orgadata.videotarget = filepath.Join(orgadata.Rootpath, orgadata.Foldername)
	} else {
		orgadata.videotarget = filepath.Join(s.TargetpathCfg.Path, orgadata.Foldername)
	}

	mode := fs.FileMode(0777)
	if s.TargetpathCfg.SetChmodFolder != "" && len(s.TargetpathCfg.SetChmodFolder) == 4 {
		mode = logger.StringToFileMode(s.TargetpathCfg.SetChmodFolder)
	}
	err := os.MkdirAll(orgadata.videotarget, mode)
	if err != nil {
		return scanner.MoveResponse{Err: err}
	}
	if mode != 0 {
		_ = os.Chmod(orgadata.videotarget, mode)
	}
	return scanner.MoveFile(orgadata.Videofile, s.SourcepathCfg, orgadata.videotarget, orgadata.Filename, false, false, config.SettingsGeneral.UseFileBufferCopy, s.TargetpathCfg.SetChmodFolder, s.TargetpathCfg.SetChmod)
}

// moveRemoveOldMediaFile moves or deletes an old media file that is being
// replaced. It handles moving/deleting additional files with different
// extensions, and removing database references. This is an internal
// implementation detail not meant to be called externally.
func (s *Organizer) moveRemoveOldMediaFile(oldfile string, id uint, move bool) error {
	if oldfile == "" {
		return nil
	}
	var err error
	var bl bool
	if move {
		retval := scanner.MoveFile(oldfile, nil, filepath.Join(s.TargetpathCfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", false, true, config.SettingsGeneral.UseFileBufferCopy, s.TargetpathCfg.SetChmodFolder, s.TargetpathCfg.SetChmod)
		if retval.Err != nil {
			return retval.Err
		}
		bl = retval.MoveDone
	} else {
		bl, err = scanner.RemoveFile(oldfile)
		if err != nil {
			return err
		}
	}
	if !bl {
		return logger.ErrNotAllowed
	}

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheFiles), oldfile)
	}
	database.ExecN(logger.GetStringsMap(s.Cfgp.Useseries, logger.DBDeleteFileByIDLocation), &id, &oldfile)

	fileext := filepath.Ext(oldfile)
	for idx := range s.SourcepathCfg.AllowedOtherExtensions {
		if fileext == s.SourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		additionalfile := strings.ReplaceAll(oldfile, fileext, s.SourcepathCfg.AllowedOtherExtensions[idx])
		if !scanner.CheckFileExist(additionalfile) {
			continue
		}
		if move {
			retval := scanner.MoveFile(additionalfile, nil, filepath.Join(s.TargetpathCfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))), "", false, true, config.SettingsGeneral.UseFileBufferCopy, s.TargetpathCfg.SetChmodFolder, s.TargetpathCfg.SetChmod)
			if retval.Err != nil && !errors.Is(retval.Err, logger.ErrNotFound) {
				logger.LogDynamic("error", "file could not be moved", logger.NewLogFieldValue(retval.Err), logger.NewLogField(logger.StrFile, additionalfile))
				continue
			}
			if !retval.MoveDone {
				continue
			}
		} else {
			bl, err = scanner.RemoveFile(additionalfile)
			if err != nil {
				logger.LogDynamic("error", "delete Files", logger.NewLogFieldValue(err))
				continue
			}
			if !bl {
				continue
			}
		}
		logger.LogDynamic("info", "Additional File removed", logger.NewLogField(logger.StrFile, additionalfile))
	}
	return nil
}

// OrganizeSeries organizes a downloaded series episode file by moving it to the target folder,
// updating the database, removing old lower quality files, and sending notifications.
// It takes organizer data, parsed file info, series ID, quality config, flags to delete
// wrong language and check runtime, and returns any error.
func (s *Organizer) OrganizeSeries(orgadata *Organizerdata, m *apiexternal.FileParser, serieid uint, cfgquality *config.QualityConfig, deletewronglanguage bool, checkruntime bool) error {
	var dbserieid uint
	var listname string
	database.GetdatarowArgs("select dbserie_id, rootpath, listname from series where id = ?", &serieid, &dbserieid, &orgadata.Rootpath, &listname)
	if dbserieid == 0 {
		return logger.ErrNotFoundDbserie
	}

	if orgadata.Listid == -1 {
		orgadata.Listid = database.GetMediaListID(s.Cfgp, listname)
		m.M.ListID = orgadata.Listid
	}

	tblepi, oldfiles, err := s.GetSeriesEpisodes(orgadata, m, &serieid, &dbserieid, false, cfgquality)
	if err != nil {
		return err
	}
	if len(tblepi) == 0 {
		return logger.ErrNotFoundEpisode
	}
	defer clear(tblepi)
	defer clear(oldfiles)

	var runtime, season string
	database.GetdatarowArgs("select runtime, season from dbserie_episodes where id = ?", &tblepi[0].Num2, &runtime, &season)

	identifiedby := database.GetdatarowN[string](false, database.QueryDbseriesGetIdentifiedByID, &dbserieid)
	if runtime == "" || runtime == "0" {
		_ = database.ScanrowsNdyn(false, "select runtime from dbseries where id = ?", &runtime, &dbserieid)
		if (runtime == "" || runtime == "0") && checkruntime && identifiedby != "date" {
			return logger.ErrRuntime
		}
	}
	if season == "" && identifiedby != "date" {
		return errors.New("season not found")
	}

	var totalruntime int
	if !database.GetdatarowN[bool](false, "select ignore_runtime from serie_episodes where id = ?", &tblepi[0].Num1) {
		if (runtime != "" && runtime != "0") && (season != "0" && season != "") {
			runtimeint, _ := strconv.Atoi(runtime)
			totalruntime = runtimeint * len(tblepi)
		}
	}

	err = s.ParseFileAdditional(orgadata, m, deletewronglanguage, totalruntime, checkruntime, cfgquality)
	if err != nil {
		return err
	}

	s.GenerateNamingTemplate(orgadata, m, &tblepi[0].Num1, tblepi)
	if orgadata.Filename == "" {
		return errors.New("generating filename")
	}

	if s.TargetpathCfg.MoveReplaced && s.TargetpathCfg.MoveReplacedTargetPath != "" && len(oldfiles) >= 1 {
		//Move old files to replaced folder
		err = s.moveremoveoldfiles(orgadata, false, serieid, true, oldfiles)
		if err != nil {
			return err
		}
	}

	if s.TargetpathCfg.Usepresort && s.TargetpathCfg.PresortFolderPath != "" {
		orgadata.Rootpath = filepath.Join(s.TargetpathCfg.PresortFolderPath, orgadata.Foldername)
	}
	//Move new files to target folder
	retval := s.moveVideoFile(orgadata)
	if retval.Err != nil {
		return retval.Err
	}
	if !retval.MoveDone {
		return errors.New("move not ok")
	}
	//Remove old files from target folder
	err = s.moveandcleanup(orgadata, retval.NewPath, m, serieid, tblepi[0].Num2, oldfiles)
	if err != nil {
		return err
	}
	//updateserie

	fileext := filepath.Ext(orgadata.Videofile)
	filebase := filepath.Base(retval.NewPath)

	var reached int
	if m.M.Priority >= cfgquality.CutoffPriority {
		reached = 1
	}
	for idx := range tblepi {
		database.ExecN("insert into serie_episode_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, serie_id, serie_episode_id, dbserie_episode_id, dbserie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
			&retval.NewPath, &filebase, &fileext, &cfgquality.Name, &m.M.ResolutionID, &m.M.QualityID, &m.M.CodecID, &m.M.AudioID, &m.M.Proper, &m.M.Repack, &m.M.Extended, &m.M.SerieID, &tblepi[idx].Num1, &tblepi[idx].Num2, &m.M.DbserieID, &m.M.Height, &m.M.Width)

		database.ExecN("update serie_episodes SET missing = 0, quality_reached = ? where id = ?", &reached, &tblepi[idx].Num1)
	}

	if config.SettingsGeneral.UseMediaCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedSeries, retval.NewPath)
		database.AppendStringCache(logger.CacheFilesSeries, retval.NewPath)
	}
	return nil
}

// OrganizeMovie organizes a downloaded movie file by moving it to the target folder,
// updating the database, removing old lower quality files, and sending notifications.
// It takes organizer data, parsed file info, movie ID, quality config, flags to delete
// wrong language and check runtime, and returns any error.
func (s *Organizer) OrganizeMovie(orgadata *Organizerdata, m *apiexternal.FileParser, movieid uint, cfgquality *config.QualityConfig, deletewronglanguage bool, checkruntime bool) error {
	var dbmovieid uint
	var listname string
	database.GetdatarowArgs("select dbmovie_id, rootpath, listname from movies where id = ?", &movieid, &dbmovieid, &orgadata.Rootpath, &listname)
	if dbmovieid == 0 {
		return logger.ErrNotFoundDbmovie
	}
	if orgadata.Listid == -1 {
		orgadata.Listid = database.GetMediaListID(s.Cfgp, listname)
		m.M.ListID = orgadata.Listid
	}
	runtime := database.GetdatarowN[string](false, "select runtime from dbmovies where id = ?", &dbmovieid)
	if (runtime == "" || runtime == "0") && checkruntime {
		return logger.ErrRuntime
	}
	parser.GetPriorityMapQual(&m.M, s.Cfgp, cfgquality, true, true)

	if orgadata.Listid == -1 {
		return errors.New("listconfig empty")
	}

	oldpriority, oldfiles := searcher.Getpriobyfiles(false, &movieid, true, m.M.Priority, cfgquality)
	if oldpriority != 0 && oldpriority >= m.M.Priority {
		if true {
			err := s.FileCleanup(orgadata)
			if err != nil {
				clear(oldfiles)
				return err
			}
		}
		clear(oldfiles)
		return logger.ErrLowerQuality
	}
	defer clear(oldfiles)

	runtimeint, _ := strconv.Atoi(runtime)
	err := s.ParseFileAdditional(orgadata, m, deletewronglanguage, runtimeint, checkruntime, cfgquality)
	if err != nil {
		return err
	}
	s.GenerateNamingTemplate(orgadata, m, &dbmovieid, nil)
	if orgadata.Filename == "" {
		return errors.New("generating filename")
	}

	if s.TargetpathCfg.MoveReplaced && s.TargetpathCfg.MoveReplacedTargetPath != "" && len(oldfiles) >= 1 {
		//Move old files to replaced folder
		err = s.moveremoveoldfiles(orgadata, false, movieid, true, oldfiles)
		if err != nil {
			return err
		}
	}

	if s.TargetpathCfg.Usepresort && s.TargetpathCfg.PresortFolderPath != "" {
		orgadata.Rootpath = filepath.Join(s.TargetpathCfg.PresortFolderPath, orgadata.Foldername)
	}
	//Move new files to target folder
	retval := s.moveVideoFile(orgadata)
	if retval.Err != nil {
		return retval.Err
	}
	if !retval.MoveDone {
		return errors.New("move not ok")
	}

	//Remove old files from target folder
	err = s.moveandcleanup(orgadata, retval.NewPath, m, movieid, dbmovieid, oldfiles)
	if err != nil {
		return err
	}

	//updatemovie
	basestr := filepath.Base(retval.NewPath)
	extstr := filepath.Ext(retval.NewPath)
	database.ExecN("insert into movie_files (location, filename, extension, quality_profile, resolution_id, quality_id, codec_id, audio_id, proper, repack, extended, movie_id, dbmovie_id, height, width) values (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)",
		&retval.NewPath, &basestr, &extstr, &cfgquality.Name, &m.M.ResolutionID, &m.M.QualityID, &m.M.CodecID, &m.M.AudioID, &m.M.Proper, &m.M.Repack, &m.M.Extended, &movieid, &dbmovieid, &m.M.Height, &m.M.Width)

	var vc int
	if m.M.Priority >= cfgquality.CutoffPriority {
		vc = 1
	}
	database.ExecN("update movies SET missing = 0, quality_reached = ? where id = ?", &vc, &movieid)

	if config.SettingsGeneral.UseFileCache {
		database.SlicesCacheContainsDelete(logger.CacheUnmatchedMovie, retval.NewPath)
		database.AppendStringCache(logger.CacheFilesMovie, retval.NewPath)
	}
	return nil
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

// moveandcleanup moves new files to target folder, updates rootpath in database,
// removes old lower quality files from target if enabled, cleans up source folder,
// and sends notifications. It takes organizer data, parsed file info, movie/series ID,
// database movie ID, and list of old files. Returns any error.
func (s *Organizer) moveandcleanup(orgadata *Organizerdata, newfile string, m *apiexternal.FileParser, id uint, dbid uint, oldfiles []string) error {
	//Update rootpath
	if !s.TargetpathCfg.Usepresort {
		if database.GetdatarowN[string](false, logger.GetStringsMap(s.Cfgp.Useseries, logger.DBRootPathFromMediaID), &id) != "" {
			if !s.Cfgp.Useseries {
				UpdateRootpath(orgadata.videotarget, "movies", &m.M.MovieID, s.Cfgp)
			} else {
				UpdateRootpath(orgadata.videotarget, "series", &m.M.SerieID, s.Cfgp)
			}
		}
	}
	//Update Rootpath end

	if s.TargetpathCfg.Replacelower && len(oldfiles) >= 1 {
		newold := oldfiles[:0]
		for idx := range oldfiles {
			if oldfiles[idx] == newfile {
				continue
			}
			newold = append(newold, oldfiles[idx])
		}
		err := s.moveremoveoldfiles(orgadata, true, id, false, newold)
		if err != nil {
			return err
		}
	}

	if !s.Cfgp.Useseries {
		//move other movie

		if s.SourcepathCfg.Name == "" {
			return errors.New("pathtemplate not found")
		}

		if !scanner.CheckFileExist(orgadata.Folder) {
			return logger.ErrNotFound
		}
		s.walkcleanup(orgadata, false)
		s.notify(orgadata, &m.M, &dbid, oldfiles)
		_ = scanner.CleanUpFolder(orgadata.Folder, s.SourcepathCfg.CleanupsizeMB)
		return nil
	}
	//move other serie
	fileext := filepath.Ext(orgadata.Videofile)
	for idx := range s.SourcepathCfg.AllowedOtherExtensions {
		if fileext == s.SourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		also := strings.ReplaceAll(orgadata.Videofile, fileext, s.SourcepathCfg.AllowedOtherExtensions[idx])
		retval := scanner.MoveFile(also, s.SourcepathCfg, orgadata.videotarget, orgadata.Filename, true, false, config.SettingsGeneral.UseFileBufferCopy, s.TargetpathCfg.SetChmodFolder, s.TargetpathCfg.SetChmod)
		if retval.Err != nil && !errors.Is(retval.Err, logger.ErrNotFound) {
			logger.LogDynamic("error", "file move", logger.NewLogFieldValue(retval.Err), logger.NewLogField(logger.StrFile, also))
		}
	}

	s.notify(orgadata, &m.M, &dbid, oldfiles)
	_ = scanner.CleanUpFolder(orgadata.Folder, s.SourcepathCfg.CleanupsizeMB)
	return nil
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
		firstfolder = filepath.Join(cfgp.Data[idxdata].CfgPath.Path, logger.Getrootpath(firstfolder))
		database.ExecN(logger.JoinStrings("update ", objtype, " set rootpath = ? where id = ?"), &firstfolder, objid)
		return
	}
}

// walkcleanup recursively walks the given root path and cleans up files.
// It calls filepath.WalkDir to traverse all files under the root path.
// For each file, it checks if it should be filtered via scanner.Filterfile.
// If so, it will either remove the file or move it to the target folder,
// depending on the useremove parameter.
// Any errors during walking or moving/removing are logged.
func (s *Organizer) walkcleanup(orgadata *Organizerdata, useremove bool) error {
	return filepath.WalkDir(orgadata.Rootpath, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if fpath == orgadata.Rootpath {
			return nil
		}
		if scanner.Filterfile(fpath, true, s.SourcepathCfg) {
			if useremove {
				_, _ = scanner.RemoveFile(fpath)
			} else {
				retval := scanner.MoveFile(fpath, s.SourcepathCfg, orgadata.videotarget, orgadata.Filename, true, false, config.SettingsGeneral.UseFileBufferCopy, s.TargetpathCfg.SetChmodFolder, s.TargetpathCfg.SetChmod)
				if retval.Err != nil && !errors.Is(retval.Err, logger.ErrNotFound) {
					logger.LogDynamic("error", "file move", logger.NewLogFieldValue(retval.Err), logger.NewLogField(logger.StrFile, fpath))
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
func (s *Organizer) moveremoveoldfiles(orgadata *Organizerdata, usecompare bool, id uint, move bool, oldfiles []string) error {
	for idx := range oldfiles {
		if usecompare {
			_, teststr := filepath.Split(oldfiles[idx])
			if teststr == orgadata.Filename || strings.HasPrefix(teststr, orgadata.videotarget) {
				continue
			}
		}
		if !scanner.CheckFileExist(oldfiles[idx]) {
			continue
		}
		err := s.moveRemoveOldMediaFile(oldfiles[idx], id, move)
		if err != nil {
			//Continue if old cannot be moved
			logger.LogDynamic("error", "Move old", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, oldfiles[idx]))
			return err
		}
	}
	return nil
}

// notify sends notifications when new media is added.
// It populates notification data from the Organizerdata and ParseInfo,
// loops through the configured notifications, renders notification messages,
// and dispatches them based on the notification type.
func (s *Organizer) notify(orgadata *Organizerdata, m *database.ParseInfo, id *uint, oldfiles []string) {
	notify := forstructurenotify{Config: s, InputNotifier: inputNotifier{
		Targetpath:    filepath.Join(orgadata.videotarget, orgadata.Filename),
		SourcePath:    orgadata.Videofile,
		Configuration: s.Cfgp.Lists[orgadata.Listid].Name,
		Source:        m,
		Time:          logger.TimeGetNow().Format(logger.GetTimeFormat()),
	}}
	defer notify.Close()
	notify.InputNotifier.Replaced = oldfiles

	if !s.Cfgp.Useseries {
		if database.GetDbmovieByIDP(id, &notify.InputNotifier.Dbmovie) != nil {
			return
		}
		notify.InputNotifier.Title = notify.InputNotifier.Dbmovie.Title
		notify.InputNotifier.Year = strconv.Itoa(notify.InputNotifier.Dbmovie.Year)
		notify.InputNotifier.Imdb = notify.InputNotifier.Dbmovie.ImdbID
	} else {
		if database.GetDbserieEpisodesByIDP(id, &notify.InputNotifier.DbserieEpisode) != nil {
			return
		}
		if database.GetDbserieByIDP(&notify.InputNotifier.DbserieEpisode.DbserieID, &notify.InputNotifier.Dbserie) != nil {
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

	for idx := range s.Cfgp.Notification {
		if s.Cfgp.Notification[idx].CfgNotification == nil || !strings.EqualFold(s.Cfgp.Notification[idx].Event, "added_data") {
			continue
		}
		notify.InputNotifier.ReplacedPrefix = s.Cfgp.Notification[idx].ReplacedPrefix
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
			if apiexternal.GetPushOverKey() != config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Apikey {
				apiexternal.NewPushOverClient(config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Apikey)
			}

			err := apiexternal.SendPushoverMessage(messagetext, messageTitle, config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Recipient)
			if err != nil {
				logger.LogDynamic("error", "Error sending pushover", logger.NewLogFieldValue(err))
			} else {
				logger.LogDynamic("info", "Pushover message sent")
			}
		case "csv":
			if scanner.AppendCsv(config.SettingsNotification[s.Cfgp.Notification[idx].MapNotification].Outputto, messagetext) != nil {
				continue
			}
		}
	}
	//notify.close()
}

// Getepisodestoimport parses the episode identifier from the filename
// and returns the matching episode IDs from the database for importing.
// It handles different identifier formats and splitting multi-episode files.
func Getepisodestoimport(serieid *uint, dbserieid *uint, m *apiexternal.FileParser) ([]database.DbstaticTwoUint, error) {
	identifiedby := database.GetdatarowN[string](false, database.QueryDbseriesGetIdentifiedByID, dbserieid)
	if identifiedby == "" {
		identifiedby = "ep"
	}

	episodeArray := getepisodearray(identifiedby, m)

	if len(episodeArray) == 0 {
		return nil, logger.ErrNotFoundEpisode
	}
	defer clear(episodeArray)
	if m.M.DbserieEpisodeID != 0 && m.M.SerieEpisodeID != 0 && len(episodeArray) == 1 {
		return []database.DbstaticTwoUint{{Num1: m.M.SerieEpisodeID, Num2: m.M.DbserieEpisodeID}}, nil
	}

	tblepi := make([]database.DbstaticTwoUint, 0, len(episodeArray))

	for idx := range episodeArray {
		episodeArray[idx] = strings.Trim(episodeArray[idx], "-EX")
		if identifiedby != logger.StrDate {
			episodeArray[idx] = strings.TrimPrefix(episodeArray[idx], "0")
			if episodeArray[idx] == "" {
				continue
			}
		}
		var addentry database.DbstaticTwoUint

		parser.GetDBEpisodeID(m, episodeArray[idx], dbserieid, &addentry.Num2)

		if addentry.Num2 != 0 {
			database.ScanrowsNdyn(false, "select id from serie_episodes where dbserie_episode_id = ? and serie_id = ?", &addentry.Num1, &addentry.Num2, serieid)
			if addentry.Num1 != 0 {
				tblepi = append(tblepi, addentry)
			}
		}
	}

	if len(tblepi) == 0 {
		return nil, logger.ErrNotFoundEpisode
	}
	return tblepi, nil
}

// getepisodearray parses the episode identifier from the filename
// using the identifiedby format. It handles splitting multi-episode files
// and returns a string slice of the individual episode identifiers.
func getepisodearray(identifiedby string, m *apiexternal.FileParser) []string {
	str1, str2 := database.RegexGetMatchesStr1Str2(true, "RegexSeriesIdentifier", m.M.Identifier)
	if str1 == "" && str2 == "" {
		return nil
	}
	if identifiedby == logger.StrDate {
		return []string{string(logger.ByteReplaceWithByte(logger.StringReplaceWithByte(str2, ' ', '-'), '.', '-'))}
	}
	var splitby string
	if strings.ContainsRune(str1, 'E') {
		splitby = "E"
	} else if strings.ContainsRune(str1, 'e') {
		splitby = "e"
	} else if strings.ContainsRune(str1, 'X') {
		splitby = "X"
	} else if strings.ContainsRune(str1, 'x') {
		splitby = "x"
	} else if strings.ContainsRune(str1, '-') {
		splitby = "-"
	}
	if splitby != "" {
		episodeArray := strings.Split(str1, splitby)
		if len(episodeArray) >= 1 {
			if episodeArray[0] == "" {
				episodeArray = episodeArray[1:]
			}
			if splitby != "-" && len(episodeArray) == 1 {
				if strings.ContainsRune(episodeArray[0], '-') {
					episodeArray = strings.Split(episodeArray[0], "-")
				}
			}
			for idx := range episodeArray {
				episodeArray[idx] = strings.Trim(episodeArray[idx], "_-. ")
			}
		}
		return episodeArray
	}
	return nil
}

// GetSeriesEpisodes checks existing files for a series episode, determines if a new file
// should replace them based on configured quality priorities, deletes lower priority files if enabled,
// and returns the list of episode IDs that are allowed to be imported along with any deleted file paths.
func (s *Organizer) GetSeriesEpisodes(orgadata *Organizerdata, m *apiexternal.FileParser, serieid *uint, dbserieid *uint, skipdelete bool, cfgquality *config.QualityConfig) ([]database.DbstaticTwoUint, []string, error) {
	tblepi, err := Getepisodestoimport(serieid, dbserieid, m)
	if err != nil {
		return nil, nil, err
	}
	if len(tblepi) == 0 {
		return nil, nil, logger.ErrNotFoundEpisode
	}

	parser.GetPriorityMapQual(&m.M, s.Cfgp, cfgquality, true, true)

	oldfiles := make([]string, 0, len(tblepi)*3)

	newtbl := tblepi[:0]
	var bl bool
	var oldPrio int
	var getoldfiles []string
	for idx := 0; idx < len(tblepi); idx++ {
		oldPrio, getoldfiles = searcher.Getpriobyfiles(true, &tblepi[idx].Num1, true, m.M.Priority, cfgquality)
		if m.M.Priority > oldPrio || oldPrio == 0 {
			oldfiles = append(oldfiles, getoldfiles...)
			bl = true
		} else {
			if database.GetdatarowN[int](false, "select count() from serie_episode_files where serie_episode_id = ?", &tblepi[idx].Num1) == 0 {
				bl = true
			} else if !skipdelete {
				if bl, err = scanner.RemoveFile(orgadata.Videofile); err == nil && bl {
					logger.LogDynamic("info", "Lower Qual Import File removed", logger.NewLogField(logger.StrPath, orgadata.Videofile), logger.NewLogField("old prio", oldPrio), logger.NewLogField(logger.StrPriority, m.M.Priority))
					s.removeotherfiles(orgadata)
					_ = scanner.CleanUpFolder(orgadata.Folder, s.SourcepathCfg.CleanupsizeMB)
					continue
				} else if err != nil {
					logger.LogDynamic("error", "delete Files", logger.NewLogFieldValue(err))
				}
				bl = false
			}
		}
		clear(getoldfiles)
		newtbl = append(newtbl, tblepi[idx])
	}

	if !bl {
		clear(oldfiles)
		clear(tblepi)
		clear(newtbl)
		return nil, nil, logger.ErrNotAllowed
	}
	return newtbl, oldfiles, nil
}

// removeotherfiles removes any other allowed file extensions
// associated with the video file in orgadata. It loops through
// the configured allowed extensions and calls RemoveFile on the
// same filename with that extension.
func (s *Organizer) removeotherfiles(orgadata *Organizerdata) {
	fileext := filepath.Ext(orgadata.Videofile)
	for idx := range s.SourcepathCfg.AllowedOtherExtensions {
		if fileext == s.SourcepathCfg.AllowedOtherExtensions[idx] {
			continue
		}
		_, _ = scanner.RemoveFile(strings.ReplaceAll(orgadata.Videofile, fileext, s.SourcepathCfg.AllowedOtherExtensions[idx]))
	}
}

// OrganizeSingleFolder walks the given folder to find media files, parses them to get metadata,
// checks that metadata against the database, and moves/renames files based on the config.
// It applies various filters for unsupported files, errors, etc.
// This handles the main logic for processing a single folder.
func (s *Organizer) OrganizeSingleFolder(folder string) {
	if s.Cfgp == nil {
		logger.LogDynamic("error", "parse failed cfgp", logger.NewLogFieldValue(logger.ErrCfgpNotFound), logger.NewLogField(logger.StrFile, &folder))
		return
	}
	if s.SourcepathCfg.Name == "" {
		logger.LogDynamic("error", "template "+s.SourcepathCfg.Name+" not found", logger.NewLogField(logger.StrFile, folder))
		return
	}
	filedata := scanner.NewFileData{Cfgp: s.Cfgp, PathCfg: s.SourcepathCfg, Listid: 0, Checkfiles: false}

	//processfolders := make([]string, 0, 500)
	_ = filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if filepath.Ext(fpath) == "" {
			return nil
		}

		if logger.SlicesContainsPart2I(s.SourcepathCfg.Disallowed, fpath) {
			logger.LogDynamic("warn", "skipped - disallowed", logger.NewLogField(logger.StrFile, fpath))
			return fs.SkipDir
		}
		if logger.ContainsI(fpath, "_unpack") {
			logger.LogDynamic("warn", "skipped - unpacking", logger.NewLogField(logger.StrFile, fpath))
			return fs.SkipDir
		}
		if scanner.Filterfiles(&fpath, &filedata) {
			return nil
		}

		if s.SourcepathCfg.MinVideoSize > 0 {
			info, err := os.Stat(fpath)
			if err == nil {
				if info.Size() < s.SourcepathCfg.MinVideoSizeByte {
					logger.LogDynamic("warn", "skipped - small files", logger.NewLogField(logger.StrFile, fpath))
					if s.SourcepathCfg.Name == "" {
						return fs.SkipDir
					}
					ext := filepath.Ext(fpath)

					ok, oknorename := scanner.CheckExtensions(true, false, s.SourcepathCfg, ext)

					if ok || oknorename || (s.SourcepathCfg.AllowedVideoExtensionsLen == 0 && s.SourcepathCfg.AllowedVideoExtensionsNoRenameLen == 0) {
						_ = os.Chmod(fpath, 0777)
						err := os.Remove(fpath)
						if err != nil {
							logger.LogDynamic("error", "file could not be removed", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, fpath))
						} else {
							logger.LogDynamic("info", "File removed", logger.NewLogField(logger.StrFile, fpath))
						}
					}
					return fs.SkipDir
				}
				bl := info.ModTime().After(logger.TimeGetNow().Add(-2 * time.Minute))
				if bl {
					logger.LogDynamic("error", "file modified too recently", logger.NewLogField(logger.StrFile, fpath))
					return fs.SkipDir
				}
			}
			//if scanner.CheckFileSizeRecent(fpath, s.SourcepathCfg) {
			//	return fs.SkipDir
			//}
		}
		m := parser.ParseFile(fpath, true, true, s.Cfgp, -1)
		if m == nil {
			if config.SettingsGeneral.UseFileCache {
				database.AppendStringCache(logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheUnmatched), fpath)
			}
			return nil
		}

		err := parser.GetDBIDs(m)
		if err != nil {
			logger.LogDynamic("error", "parse failed ids", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, &fpath))
			apiexternal.ParserPool.Put(m)

			if config.SettingsGeneral.UseFileCache {
				database.AppendStringCache(logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheUnmatched), fpath)
			}
			return nil
		}
		defer apiexternal.ParserPool.Put(m)

		if !hasValidIDs(s, m) {
			logger.LogDynamic("warn", "skipped - no valid IDs", logger.NewLogField(logger.StrFile, fpath))
			return nil
		}

		if m.M.ListID == -1 {
			m.M.ListID = getMediaListID(s, m)
		}
		if m.M.ListID == -1 {
			logger.LogDynamic("error", "listcfg not found", logger.NewLogField(logger.StrFile, fpath))
			return nil
		}
		orgadata := plorga.Get()
		defer plorga.Put(orgadata)
		orgadata.Folder = folder
		orgadata.Videofile = fpath
		if s.Cfgp.Lists[m.M.ListID].CfgQuality == nil {
			logger.LogDynamic("error", "quality not found", logger.NewLogField(logger.StrFile, orgadata.Videofile))
			return nil
		}
		if config.SettingsGeneral.UseFileCache {
			database.SlicesCacheContainsDelete(logger.GetStringsMap(s.Cfgp.Useseries, logger.CacheUnmatched), orgadata.Videofile)
		}

		if s.Cfgp.Useseries && m.M.DbserieEpisodeID != 0 && m.M.DbserieID != 0 && m.M.SerieEpisodeID != 0 && m.M.SerieID != 0 {
			if Checktitle(true, s.Cfgp.Lists[m.M.ListID].CfgQuality, &m.M) {
				logger.LogDynamic("warn", "skipped - unwanted title", logger.NewLogField(logger.StrFile, orgadata.Videofile))
				return nil
			}
			err = s.checksubfiles(orgadata)
			if err != nil {
				logger.LogDynamic("error", "check sub files", logger.NewLogField(logger.StrFile, fpath))
				return nil
			}

			err = s.OrganizeSeries(orgadata, m, Getmediaid(m.M.SerieID, s.ManualId), s.Cfgp.Lists[m.M.ListID].CfgQuality, s.Deletewronglanguage, s.Checkruntime)
		} else if !s.Cfgp.Useseries && m.M.MovieID != 0 && m.M.DbmovieID != 0 {
			if Checktitle(false, s.Cfgp.Lists[m.M.ListID].CfgQuality, &m.M) {
				logger.LogDynamic("warn", "skipped - unwanted title", logger.NewLogField(logger.StrFile, orgadata.Videofile))
				return nil
			}
			err = s.checksubfiles(orgadata)
			if err != nil {
				logger.LogDynamic("error", "check sub files", logger.NewLogField(logger.StrFile, fpath))
				return nil
			}

			err = s.OrganizeMovie(orgadata, m, Getmediaid(m.M.MovieID, s.ManualId), s.Cfgp.Lists[m.M.ListID].CfgQuality, s.Deletewronglanguage, s.Checkruntime)
		}

		if err != nil {
			logger.LogDynamic("error", "structure", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, orgadata.Videofile))
		} else {
			scanner.CleanUpFolder(folder, s.SourcepathCfg.CleanupsizeMB)
		}
		return nil
	})
}

// ParseFileIDs parses the file at the given path to extract metadata.
// It uses the provided MediaTypeConfig and list ID.
// It returns a FileParser containing the parsed metadata, or an error if parsing failed.
func ParseFileIDs(pathv string, filedata *scanner.NewFileData) (*apiexternal.FileParser, error) {
	if filedata.Cfgp == nil {
		logger.LogDynamic("error", "parse failed cfgp", logger.NewLogFieldValue(logger.ErrCfgpNotFound), logger.NewLogField(logger.StrFile, &pathv))
		return nil, logger.ErrCfgpNotFound
	}
	m := parser.ParseFile(pathv, true, true, filedata.Cfgp, filedata.Listid)
	if m == nil {
		return nil, logger.ErrNotFound
	}

	err := parser.GetDBIDs(m)
	if err != nil {
		logger.LogDynamic("error", "parse failed ids", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, &pathv))
		apiexternal.ParserPool.Put(m)
		return nil, err
	}
	return m, nil
}

// checksubfiles checks for any disallowed subtitle files in the same
// folder as the video file. It also checks if there are multiple files
// with the same extension, which indicates it may not be a standalone movie.
// It returns an error if disallowed files are found or too many matching files exist.
func (s *Organizer) checksubfiles(orgadata *Organizerdata) error {
	var disallowed bool
	var count int
	ext := filepath.Ext(orgadata.Videofile)
	_ = filepath.WalkDir(orgadata.Folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(fpath), ext) {
			count++
			if !s.Cfgp.Useseries && count >= 2 {
				return filepath.SkipAll
			}
		}
		if logger.SlicesContainsPart2I(s.SourcepathCfg.Disallowed, fpath) {
			disallowed = true
			return filepath.SkipAll
		}
		return nil
	})
	if disallowed {
		if s.SourcepathCfg.DeleteDisallowed && orgadata.Videofile != s.SourcepathCfg.Path {
			_ = s.FileCleanup(orgadata)
		}
		logger.LogDynamic("warn", "skipped - disallowed", logger.NewLogField(logger.StrFile, orgadata.Videofile))
		return fs.SkipDir
	}
	if !s.Cfgp.Useseries && count >= 2 {
		logger.LogDynamic("warn", "skipped - too many files", logger.NewLogField(logger.StrFile, orgadata.Videofile))
		return fs.SkipDir
	}
	return nil
}

// hasValidIDs checks if the parser has valid IDs for the media item.
// For series it checks episode ID and series ID.
// For movies it checks movie ID.
// It uses the config to determine if this is a series or movie.
func hasValidIDs(s *Organizer, m *apiexternal.FileParser) bool {
	if s.Cfgp.Useseries {
		return m.M.DbserieEpisodeID != 0 && m.M.DbserieID != 0 && m.M.SerieEpisodeID != 0 && m.M.SerieID != 0
	}
	return m.M.MovieID != 0 && m.M.DbmovieID != 0
}

// getMediaListID returns the media list ID for the given media item.
// For series, it looks up the list ID based on the series ID.
// For movies, it looks up the list ID based on the movie ID.
// It uses the config to determine if the media type is series or movie.
func getMediaListID(s *Organizer, m *apiexternal.FileParser) int {
	if s.Cfgp.Useseries {
		return database.GetMediaListIDGetListname(s.Cfgp, m.M.SerieID)
	}
	return database.GetMediaListIDGetListname(s.Cfgp, m.M.MovieID)
}

// Getmediaid returns the manual media ID if it is non-zero, otherwise returns the regular media ID.
// Used to allow manually overriding the auto-detected media ID.
func Getmediaid(id uint, manualid uint) uint {
	if manualid != 0 {
		return manualid
	}
	return id
}

// Checktitle checks if the given wanted title and year match the parsed title and year
// from the media file. It compares the wanted title against any alternate titles for the
// media entry from the database. Returns true if the title is unwanted and should be skipped.
func Checktitle(useseries bool, qualcfg *config.QualityConfig, m *database.ParseInfo) bool {
	if qualcfg == nil {
		logger.LogDynamic("debug", "qualcfg empty")
		return true
	}
	if !qualcfg.CheckTitle {
		return false
	}
	var wantedTitle, wantedslug string
	var year int
	var id uint
	if useseries {
		id = m.DbserieID
	} else {
		id = m.DbmovieID
	}
	database.GetdatarowArgs(logger.GetStringsMap(useseries, logger.DBMediaTitlesID), &id, &year, &wantedTitle, &wantedslug)

	if wantedTitle == "" {
		if wantedTitle == "" {
			logger.LogDynamic("debug", "wanttitle empty")
			return true
		}
	}
	if qualcfg.Name != "" {
		importfeed.StripTitlePrefixPostfixGetQual(m, qualcfg)
	}
	if m.Title == "" {
		logger.LogDynamic("debug", "m Title empty")
		return true
	}

	if m.Year != 0 && year != 0 && m.Year != year && (!qualcfg.CheckYear1 || m.Year != year+1 && m.Year != year-1) {
		logger.LogDynamic("debug", "year different", logger.NewLogField("File", m.Year), logger.NewLogField("wanted", year))
		return true
	}
	if wantedTitle != "" {
		if qualcfg.CheckTitle && apiexternal.ChecknzbtitleB(wantedTitle, wantedslug, m.Title, qualcfg.CheckYear1, m.Year) {
			return false
		}
	}

	if config.SettingsGeneral.UseMediaCache {
		b := logger.CacheDBSeriesAlt
		if !useseries {
			b = logger.CacheTitlesMovie
		}
		a := database.GetCachedTypeObjArr[database.DbstaticTwoStringOneInt](b)
		if a != nil {
			intid := int(id)
			for idx := range a {
				if a[idx].Str1 == "" || a[idx].Num != intid {
					continue
				}
				if apiexternal.ChecknzbtitleB(a[idx].Str1, a[idx].Str2, m.Title, qualcfg.CheckYear1, m.Year) {
					return false
				}
			}
		}
	} else {
		arr := database.Getentryalternatetitlesdirect(id, useseries)
		for idx := range arr {
			if arr[idx].Str1 == "" {
				continue
			}
			if apiexternal.ChecknzbtitleB(arr[idx].Str1, arr[idx].Str2, m.Title, qualcfg.CheckYear1, m.Year) {
				clear(arr)
				return false
			}
		}
		clear(arr)
	}

	return true
}

// Close closes the Organizer, saving its internal state.
func (s *Organizer) Close() {
	plstructure.Put(s)
}

// Close closes the Organizer, saving its internal state.
func (s *forstructurenotify) Close() {
	if s == nil {
		return
	}
	*s = forstructurenotify{}
}

package structure

import (
	"context"
	"database/sql"
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
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/mozillazg/go-unidecode"
)

func init() {
	// Organize functions now use unified organize method directly
}

// Organizer struct contains configuration and path information for organizing media files.
type Organizer struct {
	// ManualId is a unit containing a manually set ID
	manualID uint
	// forcedAlbumID is a MBID (music) or ASIN (audiobook) that bypasses normal candidate search.
	forcedAlbumID string
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
	Serie              database.Serie
	DbserieEpisode     database.DbserieEpisode
	Dbaudiobook        database.Dbaudiobook
	DbaudiobookChapter database.DbaudiobookChapter
	Dbbook             database.Dbbook
	Dbalbum            database.Dbalbum
	Dbtrack            database.Dbtrack
	Author             database.Dbauthor
	BookSeries         database.DbbookSeries
	Artist             database.Dbartist
	AlbumArtist        database.Dbartist
	TitleSource        string
	EpisodeTitleSource string
	Title              string
	Track              int
	Disc               int
	TotalDiscs         int
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
	// Serie is the per-list Serie struct for TV series
	Serie database.Serie
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

// RenameEntry records a single file rename during organization.
type RenameEntry struct {
	OldName string
	NewName string
}

type Organizerdata struct {
	Oldfiles []string
	// TargetPath is the target directory path for organized media
	TargetPath string
	// Foldername is the folder name
	Foldername string
	// Filename is the file name (for single-file media like movies/books)
	Filename string
	// Filenames is the list of filenames for multi-file media (music albums, audiobooks)
	Filenames []string
	// Rootpath is the root path
	Rootpath string
	// MediaFile is the primary media file path (video file for movies/series, audio file for music/audiobooks, ebook for books)
	MediaFile string
	// MediaFiles is the list of media files for multi-file media (music albums, audiobooks with multiple chapters)
	MediaFiles []string
	// Folder is the folder path
	Folder string
	// Listid is the list ID
	Listid int
	// RenamedFiles tracks all old->new filename mappings during organization
	RenamedFiles []RenameEntry
}

var (
	strOldPrio             = "old prio"
	errRuntime             = errors.New("wrong runtime")
	errLowerQuality        = errors.New("lower quality")
	errSeasonEmpty         = errors.New("season empty")
	errGeneratingFilename  = errors.New("generating filename")
	errWrongRuntime        = errors.New("wrong runtime")
	errWrongLanguage       = errors.New("wrong language")
	errUnprocessed         = errors.New("unprocessed")
	errTooRecentlyModified = errors.New("file modified too recently")
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
	// archiveExtensions is a pre-allocated map for fast archive extension lookup.
	// Defined at package level to avoid repeated allocations.
	archiveExtensions = map[string]struct{}{
		".rar":     {},
		".zip":     {},
		".7z":      {},
		".tar":     {},
		".tar.gz":  {},
		".tgz":     {},
		".tar.bz2": {},
		".tbz2":    {},
		".tar.xz":  {},
		".txz":     {},
		".gz":      {},
		".bz2":     {},
		".xz":      {},
	}
)

func TestInputnotifier(parsestring string) (string, error) {
	forparser := inputNotifier{
		Dbmovie: database.Dbmovie{
			Title:            "Inception",
			OriginalTitle:    "Inception",
			Year:             2000,
			Overview:         "A thief who steals corporate secrets through the use of dream-sharing technology is given the inverse task of planting an idea into the mind of a C.E.O.",
			Tagline:          "Your mind is the scene of the crime",
			Genres:           "Action, Science Fiction, Adventure",
			Runtime:          148,
			ReleaseDate:      sql.NullTime{Time: time.Now(), Valid: true},
			Status:           "Released",
			OriginalLanguage: "en",
			SpokenLanguages:  "English",
			ImdbID:           "tt1375666",
			MoviedbID:        27205,
			TraktID:          1390,
			FacebookID:       "123",
			InstagramID:      "134",
			TwitterID:        "145",
			VoteAverage:      8.3,
			VoteCount:        4000,
			Popularity:       5.5,
			Budget:           160000000,
			Revenue:          825532764,
			Poster:           "/poster.jpg",
			Backdrop:         "/backdrop.jpg",
		},
		Serie: database.Serie{
			Aliases: "Breaking Bad",
		},
		Dbserie: database.Dbserie{
			Identifiedby:    "ep",
			Firstaired:      "2010-01-01",
			Runtime:         "45:00",
			Language:        "English",
			Rating:          "5.6",
			Siterating:      "5.7",
			SiteratingCount: "500",
			TvrageID:        1,
			Facebook:        "2",
			Instagram:       "3",
			Twitter:         "4",
			Seriename:       "Breaking Bad",
			Overview:        "A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.",
			Status:          "Ended",
			Network:         "AMC",
			Genre:           "Crime, Drama, Thriller",
			ImdbID:          "tt0903747",
			ThetvdbID:       81189,
			TraktID:         1388,
			Poster:          "/poster.jpg",
			Banner:          "/banner.jpg",
			Fanart:          "/fanart.jpg",
		},
		DbserieEpisode: database.DbserieEpisode{
			Title:      "Pilot",
			Season:     "1",
			Episode:    "1",
			Identifier: "S01E01",
			Overview:   "Walter White, a struggling high school chemistry teacher, is diagnosed with inoperable lung cancer.",
			FirstAired: sql.NullTime{Time: time.Now(), Valid: true},
			Runtime:    58,
		},
		Title:          "Breaking Bad",
		Year:           "2008",
		Season:         "1",
		Episode:        "1",
		Series:         "Breaking Bad",
		EpisodeTitle:   "Pilot",
		Configuration:  "series-hd",
		SourcePath:     "/downloads/Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS.mkv",
		Targetpath:     "/media/tv/Breaking Bad/Season 01/Breaking Bad - S01E01 - Pilot.mkv",
		Imdb:           "tt0903747",
		Tvdb:           "81189",
		Time:           "2024-01-15 14:30:00",
		ReplacedPrefix: "Replaced: ",
		Replaced:       []string{"/old/Breaking.Bad.S01E01.720p.mkv"},
		Source: &database.ParseInfo{
			Title:      "Breaking Bad",
			Year:       uint16(2008),
			Season:     1,
			Episode:    1,
			Quality:    "bluray",
			Resolution: "1080p",
			Codec:      "x264",
			Audio:      "AC3",
			File:       "/downloads/Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS.mkv",
			Runtime:    58,
			Height:     1080,
			Width:      1920,
			Imdb:       "tt0903747",
			Tvdb:       "tvdb81189",
			Identifier: "S01E01",
			Proper:     false,
			Extended:   false,
			Repack:     false,
			Languages:  []string{"English"},
		},
	}
	_, outstr, err := logger.ParseStringTemplate(parsestring, &forparser)

	return outstr, err
}

func TestParsertype(parsestring string) (string, error) {
	forparser := parsertype{
		Dbmovie: database.Dbmovie{
			Title:            "Inception",
			OriginalTitle:    "Inception",
			Year:             2000,
			Overview:         "A thief who steals corporate secrets through the use of dream-sharing technology is given the inverse task of planting an idea into the mind of a C.E.O.",
			Tagline:          "Your mind is the scene of the crime",
			Genres:           "Action, Science Fiction, Adventure",
			Runtime:          148,
			ReleaseDate:      sql.NullTime{Time: time.Now(), Valid: true},
			Status:           "Released",
			OriginalLanguage: "en",
			SpokenLanguages:  "English",
			ImdbID:           "tt1375666",
			MoviedbID:        27205,
			TraktID:          1390,
			FacebookID:       "123",
			InstagramID:      "134",
			TwitterID:        "145",
			VoteAverage:      8.3,
			VoteCount:        4000,
			Popularity:       5.5,
			Budget:           160000000,
			Revenue:          825532764,
			Poster:           "/poster.jpg",
			Backdrop:         "/backdrop.jpg",
		},
		Serie: database.Serie{
			Aliases: "Breaking Bad",
		},
		Dbserie: database.Dbserie{
			Identifiedby:    "ep",
			Firstaired:      "2010-01-01",
			Runtime:         "45:00",
			Language:        "English",
			Rating:          "5.6",
			Siterating:      "5.7",
			SiteratingCount: "500",
			TvrageID:        1,
			Facebook:        "2",
			Instagram:       "3",
			Twitter:         "4",
			Seriename:       "Breaking Bad",
			Overview:        "A high school chemistry teacher diagnosed with inoperable lung cancer turns to manufacturing and selling methamphetamine in order to secure his family's future.",
			Status:          "Ended",
			Network:         "AMC",
			Genre:           "Crime, Drama, Thriller",
			ImdbID:          "tt0903747",
			ThetvdbID:       81189,
			TraktID:         1388,
			Poster:          "/poster.jpg",
			Banner:          "/banner.jpg",
			Fanart:          "/fanart.jpg",
		},
		DbserieEpisode: database.DbserieEpisode{
			Title:      "Pilot",
			Season:     "1",
			Episode:    "1",
			Identifier: "S01E01",
			Overview:   "Walter White, a struggling high school chemistry teacher, is diagnosed with inoperable lung cancer.",
			FirstAired: sql.NullTime{Time: time.Now(), Valid: true},
			Runtime:    58,
		},
		Source: &database.ParseInfo{
			Title:      "Breaking Bad",
			Year:       uint16(2008),
			Season:     1,
			Episode:    1,
			Quality:    "bluray",
			Resolution: "1080p",
			Codec:      "x264",
			Audio:      "AC3",
			File:       "/downloads/Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS.mkv",
			Runtime:    58,
			Height:     1080,
			Width:      1920,
			Imdb:       "tt0903747",
			Tvdb:       "tvdb81189",
			Identifier: "S01E01",
			Proper:     false,
			Extended:   false,
			Repack:     false,
			Languages:  []string{"English"},
		},
		TitleSource:        "Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS",
		EpisodeTitleSource: "Pilot",
		Identifier:         "S01E01",
		Episodes:           []int{1},
	}
	_, outstr, err := logger.ParseStringTemplate(parsestring, &forparser)

	return outstr, err
}

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

	if h := mediatype.Get(s.Cfgp.IsType); h != nil {
		err = h.CleanupAfterRemove(
			folder,
			rootpath,
			s.sourcepathCfg.Name,
			func(rp string) { s.walkcleanup(rp, "", "", true, nil) },
			func() { s.removeotherfiles(videofile) },
		)
		if err != nil {
			return err
		}
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

			if err := os.Chmod(fpath, 0o777); err != nil {
				logger.Logtype("error", 1).
					Str(logger.StrFile, fpath).
					Err(err).
					Msg("Failed to change file permissions")
			}

			return nil
		})

		if err := os.RemoveAll(folder); err != nil {
			return err
		}

		logger.Logtype("info", 1).
			Str(logger.StrFile, folder).
			Msg("Folder removed")
	}

	return nil
}

// folderHasRecentFiles returns true if any file in the folder was created or modified within the last minute.
func folderHasRecentFiles(folder string) bool {
	cutoff := logger.TimeGetNow().Add(-1 * time.Minute)
	hasRecent := false

	_ = filepath.WalkDir(folder, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		if logger.TimeAfter(info.ModTime(), cutoff) {
			hasRecent = true
			return fs.SkipAll
		}

		return nil
	})

	return hasRecent
}

// ParseFileAdditional performs additional parsing and validation on a video file.
// It checks the runtime, language, and quality against configured values and cleans up the file if needed.
// It is used after initial parsing to enforce business logic around file properties.
func (s *Organizer) ParseFileAdditional(
	ctx context.Context,
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

	m.File = o.MediaFile

	// Only parse video file for video media types (movies, series)
	if mediatype.SupportsVideoFile(s.Cfgp.IsType) {
		if err := parser.ParseVideoFile(ctx, m, cfgQuality); err != nil {
			if errors.Is(err, logger.ErrTracksEmpty) {
				s.fileCleanup(o.Folder, o.MediaFile, o.Rootpath)
			}

			return err
		}

		// Runtime validation (only for video)
		if err := s.validateRuntime(m, runtime, checkruntime, o); err != nil {
			return err
		}

		// Language validation (only for video)
		return s.validateLanguage(m, deletewronglanguage, o)
	}

	return nil
}

// validateRuntime checks if the parsed video file runtime matches the expected runtime
// from the database. It handles special cases for series (multiplying by episode count)
// and applies tolerance settings for runtime differences. If the runtime differs beyond
// the configured threshold, it can optionally delete the file and return an error.
//
// Parameters:
//   - m: ParseInfo containing parsed video metadata including runtime
//   - runtime: Expected runtime in minutes from database
//   - checkruntime: Flag to enable/disable runtime validation
//   - o: Organizer data containing file paths for cleanup operations
//
// Returns error if runtime validation fails or file cleanup encounters issues.
func (s *Organizer) validateRuntime(
	m *database.ParseInfo,
	runtime uint,
	checkruntime bool,
	o *Organizerdata,
) error {
	if m.Runtime < 1 || !checkruntime || runtime == 0 {
		return nil
	}

	wantedruntime := int(
		runtime,
	) * mediatype.GetRuntimeMultiplier(
		s.Cfgp.IsType,
		m,
	)

	targetruntime := m.Runtime / 60
	if targetruntime == wantedruntime {
		return nil
	}

	if s.targetpathCfg.MaxRuntimeDifference == 0 {
		logger.Logtype("warning", 2).
			Int(logger.StrWanted, wantedruntime).
			Int(logger.StrFound, targetruntime).
			Msg("wrong runtime")

		return errWrongRuntime
	}

	maxdifference := s.targetpathCfg.MaxRuntimeDifference
	if handler := mediatype.Get(s.Cfgp.IsType); handler != nil {
		maxdifference += handler.GetRuntimeBonus(m)
	}

	difference := abs(wantedruntime - targetruntime)
	if difference > maxdifference {
		if s.targetpathCfg.DeleteWrongRuntime {
			s.fileCleanup(o.Folder, o.MediaFile, o.Rootpath)
		}

		logger.Logtype("warning", 2).
			Int(logger.StrWanted, wantedruntime).
			Int(logger.StrFound, targetruntime).
			Msg("wrong runtime")

		return errWrongRuntime
	}

	return nil
}

// abs returns the absolute value of an integer.
// This helper function is used in runtime validation to calculate
// the difference between expected and actual runtimes.
func abs(x int) int {
	if x < 0 {
		return -x
	}

	return x
}

// validateLanguage validates the audio languages of a parsed video file against
// the configured allowed languages list. If the file contains languages not in
// the allowed list and deletion is enabled, it will remove the file and clean up
// the folder. This function helps enforce language preferences for media organization.
//
// Parameters:
//   - m: ParseInfo containing detected audio languages from the video file
//   - deletewronglanguage: Flag to enable deletion of files with wrong languages
//   - o: Organizer data containing file paths for cleanup operations
//
// Returns error if language validation fails or file cleanup encounters issues.
func (s *Organizer) validateLanguage(
	m *database.ParseInfo,
	deletewronglanguage bool,
	o *Organizerdata,
) error {
	if !deletewronglanguage || s.targetpathCfg.AllowedLanguagesLen == 0 {
		return nil
	}

	lenlang := len(m.Languages)
	for i := range s.targetpathCfg.AllowedLanguages {
		if (lenlang == 0 && s.targetpathCfg.AllowedLanguages[i] == "") ||
			logger.SlicesContainsI(m.Languages, s.targetpathCfg.AllowedLanguages[i]) {
			return nil
		}
	}

	// Language not allowed - extract lang info once for logging
	var foundLang, wantedLang string
	if len(m.Languages) > 0 {
		foundLang = m.Languages[0]
	}

	if len(s.targetpathCfg.AllowedLanguages) > 0 {
		wantedLang = s.targetpathCfg.AllowedLanguages[0]
	}

	// deletewronglanguage is already true at this point (checked above)
	if err := s.fileCleanup(o.Folder, o.MediaFile, o.Rootpath); err != nil {
		logger.Logtype("error", 2).
			Str(logger.StrWanted, wantedLang).
			Str(logger.StrFound, foundLang).
			Err(err).
			Msg("failed to cleanup wrong language file")

		return err
	}

	logger.Logtype("warning", 2).
		Str(logger.StrFound, foundLang).
		Str(logger.StrWanted, wantedLang).
		Msg("wrong language")

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

// stringRemoveAllRunes efficiently removes all occurrences of a specific byte
// from a string. It uses a buffer pool for memory efficiency and only allocates
// when the byte is actually found in the string. This function is used to clean
// file paths and names by removing problematic characters like forward slashes.
func stringRemoveAllRunes(s string, r byte) string {
	if s == "" {
		return s
	}

	// Find first occurrence to avoid allocation if not needed
	idx := strings.IndexByte(s, r)
	if idx == -1 {
		return s
	}

	out := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(out)

	// Pre-grow buffer to expected size (at least len(s) - 1)
	out.Grow(len(s))

	// Write bytes before first occurrence
	out.WriteString(s[:idx])

	// Continue from after first occurrence
	for i := idx + 1; i < len(s); i++ {
		if s[i] != r {
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

	logger.Logtype("debug", 0).
		Str("naming", s.Cfgp.Naming).
		Str("foldername_initial", o.Foldername).
		Str("filename_initial", o.Filename).
		Str("rootpath", o.Rootpath).
		Uint("mediatype", s.Cfgp.IsType).
		Msg("DEBUG: GenerateNamingTemplate start")

	handler := mediatype.Get(s.Cfgp.IsType)
	if handler == nil {
		o.Filename = ""

		logger.Logtype("warning", 0).Uint("type", s.Cfgp.IsType).Msg("Handler is nil")
		return
	}

	// Use handler to fill naming data
	namingData := &mediatype.NamingData{}

	clearFolder, ok := handler.FillNamingData(dbid, o.MediaFile, m, namingData)
	if !ok {
		o.Filename = ""

		logger.Logtype("warning", 0).
			Uint("type", s.Cfgp.IsType).
			Uint("dbid", *dbid).
			Msg("FillNamingData failed")

		return
	}

	// Copy naming data to forparser
	forparser.Dbmovie = namingData.Dbmovie
	forparser.Dbserie = namingData.Dbserie
	forparser.Serie = namingData.Serie
	forparser.DbserieEpisode = namingData.DbserieEpisode
	forparser.Dbaudiobook = namingData.Dbaudiobook
	forparser.DbaudiobookChapter = namingData.DbaudiobookChapter
	forparser.Dbbook = namingData.Dbbook
	forparser.Dbalbum = namingData.Dbalbum
	forparser.Dbtrack = namingData.Dbtrack
	forparser.Author = namingData.Author
	forparser.BookSeries = namingData.BookSeries
	forparser.Artist = namingData.Artist
	forparser.AlbumArtist = namingData.AlbumArtist
	forparser.TitleSource = namingData.TitleSource
	forparser.EpisodeTitleSource = namingData.EpisodeTitleSource
	forparser.Title = namingData.Title
	forparser.Track = namingData.Track
	forparser.Episodes = namingData.Episodes

	// Handle folder name based on rootpath
	if o.Rootpath != "" && o.Rootpath != s.targetpathCfg.Path {
		_, getfoldername := logger.SplitByLR(o.Rootpath, checksplit(o.Rootpath))
		if getfoldername != "" {
			if clearFolder {
				// Movies: clear folder name
				o.Foldername = ""
			} else if s.Cfgp.IsType == config.MediaTypeSeries {
				// Series: more complex handling based on identified type
				if database.Getdatarow[string](
					false,
					database.QueryDbseriesGetIdentifiedByID,
					&m.DbserieID,
				) == "date" {
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
			} else if s.Cfgp.IsType == config.MediaTypeMusic || s.Cfgp.IsType == config.MediaTypeAudiobook || s.Cfgp.IsType == config.MediaTypeBook {
				// Rootpath already includes the full album folder path,
				// so clear Foldername to avoid duplication.
				o.Foldername = ""
			}
		}
	}

	var err error

	bl, o.Foldername, err = logger.ParseStringTemplate(o.Foldername, &forparser)

	if bl {
		o.cleanorgafilefolder()
		logger.Logtype("error", 0).
			Uint("type", s.Cfgp.IsType).
			Uint("dbid", *dbid).
			Err(err).
			Msg("Generating foldername")

		return
	}

	bl, o.Filename, err = logger.ParseStringTemplate(o.Filename, &forparser)
	if bl {
		o.cleanorgafilefolder()
		logger.Logtype("error", 0).
			Uint("type", s.Cfgp.IsType).
			Uint("dbid", *dbid).
			Err(err).
			Msg("Generating filename")

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

// moveMediaFile moves the media file(s) specified in orgadata to the target folder.
// For single-file media (movies, series, books): moves MediaFile to target
// For multi-file media (music, audiobooks): moves all files in MediaFiles to target
// It creates the target folder if needed, setting permissions according to TargetpathCfg.
// Returns the path of the moved file (or first file for multi-file), and an error.
func (s *Organizer) moveMediaFile(o *Organizerdata) (string, error) {
	if o.Rootpath != "" && o.Rootpath != s.targetpathCfg.Path {
		// For music/audiobooks, rootpath already contains the full album path
		// Don't append foldername to avoid duplication
		if s.Cfgp.IsType == config.MediaTypeMusic || s.Cfgp.IsType == config.MediaTypeAudiobook {
			o.TargetPath = o.Rootpath
		} else {
			o.TargetPath = filepath.Join(o.Rootpath, o.Foldername)
		}
	} else {
		o.TargetPath = filepath.Join(s.targetpathCfg.Path, o.Foldername)
	}

	mode := fs.FileMode(0o777)
	if len(s.targetpathCfg.SetChmodFolder) == 4 {
		mode = logger.StringToFileMode(s.targetpathCfg.SetChmodFolder)
	}

	if err := os.MkdirAll(o.TargetPath, mode); err != nil {
		return "", err
	}

	if mode != 0 {
		if err := os.Chmod(o.TargetPath, mode); err != nil {
			logger.Logtype("error", 1).
				Str("path", o.TargetPath).
				Str("mode", mode.String()).
				Err(err).
				Msg("Failed to change directory permissions")
		}
	}

	// Cache general settings to avoid repeated lookups
	generalCfg := config.GetSettingsGeneral()
	moveOpts := scanner.MoveFileOptions{
		UseBufferCopy: generalCfg.UseFileBufferCopy,
		Chmod:         s.targetpathCfg.SetChmod,
		ChmodFolder:   s.targetpathCfg.SetChmodFolder,
		MediaType:     s.Cfgp.IsType,
	}

	// Check if this is multi-file media (music albums, audiobooks)
	if len(o.MediaFiles) > 0 {
		// Move all files for multi-file media
		// Verify Filenames array matches MediaFiles array length
		useGeneratedFilenames := len(o.Filenames) == len(o.MediaFiles)

		var firstMovedPath string
		for idx, mediaFile := range o.MediaFiles {
			// Use generated filename if available and lengths match, otherwise use original
			var trackFilename string
			if useGeneratedFilenames && o.Filenames[idx] != "" {
				trackFilename = o.Filenames[idx]
			} else {
				trackFilename = filepath.Base(mediaFile)
			}

			movedPath, err := scanner.MoveFile(
				mediaFile,
				s.sourcepathCfg,
				o.TargetPath,
				trackFilename,
				moveOpts,
			)
			if err != nil {
				logger.Logtype("error", 1).
					Str(logger.StrFile, mediaFile).
					Str("targetFilename", trackFilename).
					Int("trackNumber", idx+1).
					Err(err).
					Msg("Failed to move track file")

				return "", err
			}

			o.RenamedFiles = append(o.RenamedFiles, RenameEntry{
				OldName: filepath.Base(mediaFile),
				NewName: filepath.Base(movedPath),
			})

			if idx == 0 {
				firstMovedPath = movedPath
			}
		}

		return firstMovedPath, nil
	}

	// Single file media (movies, series episodes, books)
	newpath, err := scanner.MoveFile(
		o.MediaFile,
		s.sourcepathCfg,
		o.TargetPath,
		o.Filename,
		moveOpts,
	)
	if err == nil {
		o.RenamedFiles = append(o.RenamedFiles, RenameEntry{
			OldName: filepath.Base(o.MediaFile),
			NewName: filepath.Base(newpath),
		})
	}

	return newpath, err
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

	// Cache general settings once for this function
	generalCfg := config.GetSettingsGeneral()

	// Pre-compute target path for moves
	var moveTargetPath string
	if move {
		moveTargetPath = filepath.Join(
			s.targetpathCfg.MoveReplacedTargetPath,
			filepath.Base(filepath.Dir(oldfile)),
		)
	}

	// Create reusable move options
	moveOpts := scanner.MoveFileOptions{
		UseBufferCopy: generalCfg.UseFileBufferCopy,
		Chmod:         s.targetpathCfg.SetChmod,
		ChmodFolder:   s.targetpathCfg.SetChmodFolder,
		UseNil:        true,
		MediaType:     s.Cfgp.IsType,
	}

	if move {
		_, err := scanner.MoveFile(oldfile, nil, moveTargetPath, "", moveOpts)
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

	if generalCfg.UseFileCache {
		database.SlicesCacheContainsDelete(
			mtstrings.GetStringsMap(s.Cfgp.IsType, logger.CacheFiles),
			oldfile,
		)
	}

	database.ExecNMap(s.Cfgp.IsType, logger.DBDeleteFileByIDLocation, id, oldfilep)

	fileext := filepath.Ext(oldfile)

	var (
		err error
		bl  bool
	)

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
			_, err = scanner.MoveFile(additionalfile, nil, moveTargetPath, "", moveOpts)
			if err != nil {
				if !errors.Is(err, logger.ErrNotFound) {
					logger.Logtype("error", 1).
						Str(logger.StrFile, additionalfile).
						Err(err).
						Msg("file could not be moved")
				}

				continue
			}
		} else {
			bl, err = scanner.RemoveFile(additionalfile)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("delete Files")
				continue
			}

			if !bl {
				continue
			}
		}

		logger.Logtype("info", 1).
			Str(logger.StrFile, additionalfile).
			Msg("Additional File removed")
	}

	return nil
}

// getDataConfig returns the MediaDataConfig whose CfgPath matches the
// organizer's target path config, or nil if none matches.
func (s *Organizer) getDataConfig() *config.MediaDataConfig {
	for idx := range s.Cfgp.Data {
		if s.Cfgp.Data[idx].CfgPath == s.targetpathCfg {
			return &s.Cfgp.Data[idx]
		}
	}

	return nil
}

// Organize organizes a downloaded media file (movie or series) by moving it to the target folder,
// updating the database, removing old lower quality files, and sending notifications.
// It uses switch on s.Cfgp.IsType to handle type-specific logic.
func (s *Organizer) Organize(
	ctx context.Context,
	o *Organizerdata,
	m *database.ParseInfo,
	cfgquality *config.QualityConfig,
	deletewronglanguage, checkruntime bool,
) error {
	var (
		runtime      uint
		oldfiles     []string
		oldpriority  int
		identifiedby string
		mediaID      *uint
		dbMediaID    *uint
		namingID     *uint
	)

	// Type-specific initial validation and setup
	switch s.Cfgp.IsType {
	case config.MediaTypeMovie:
		if m.DbmovieID == 0 {
			return logger.ErrNotFoundDbmovie
		}

		mediaID = &m.MovieID
		dbMediaID = &m.DbmovieID
		namingID = &m.DbmovieID

		// Get runtime for movies
		database.Scanrowsdyn(
			false,
			mtstrings.GetStringsMap(s.Cfgp.IsType, "SelectRuntime"),
			&m.RuntimeStr,
			&m.DbmovieID,
		)

	case config.MediaTypeSeries:
		if m.DbserieID == 0 {
			return logger.ErrNotFoundDbserie
		}

		mediaID = &m.SerieID
		dbMediaID = &m.DbserieID

		// Get series episodes
		err := s.GetSeriesEpisodes(o, m, false, cfgquality)
		if err != nil {
			return err
		}

		if len(m.Episodes) == 0 {
			return logger.ErrNotFoundEpisode
		}

		namingID = &m.Episodes[0].Num1

		// Get episode runtime and season
		if len(m.Episodes) > 0 {
			database.GetdatarowArgs(
				mtstrings.GetStringsMap(s.Cfgp.IsType, "SelectEpisodeRuntime"),
				&m.Episodes[0].Num2,
				&m.RuntimeStr,
				&m.SeasonStr,
			)
		}

		// Get identifiedby for series
		identifiedby = database.Getdatarow[string](
			false,
			database.QueryDbseriesGetIdentifiedByID,
			&m.DbserieID,
		)

		// Try series runtime if episode runtime is empty
		if (m.RuntimeStr == "" || m.RuntimeStr == "0") && identifiedby != "date" {
			database.Scanrowsdyn(
				false,
				mtstrings.GetStringsMap(s.Cfgp.IsType, "SelectRuntime"),
				&m.RuntimeStr,
				&m.DbserieID,
			)

			if (m.RuntimeStr == "" || m.RuntimeStr == "0") && checkruntime &&
				identifiedby != "date" {
				return errRuntime
			}
		}

		if m.SeasonStr == "" && identifiedby != "date" {
			return errSeasonEmpty
		}

	case config.MediaTypeBook:
		if m.DbbookID == 0 {
			return logger.ErrNotFoundDbbook
		}

		mediaID = &m.BookID
		dbMediaID = &m.DbbookID
		namingID = &m.DbbookID

	case config.MediaTypeAudiobook:
		if m.DbaudiobookID == 0 {
			return logger.ErrNotFoundDbaudiobook
		}

		mediaID = &m.AudiobookID
		dbMediaID = &m.DbaudiobookID
		namingID = &m.DbaudiobookID

	case config.MediaTypeMusic:
		if m.DbalbumID == 0 {
			return logger.ErrNotFoundDbalbum
		}

		mediaID = &m.AlbumID
		dbMediaID = &m.DbalbumID
		namingID = &m.DbalbumID
	}

	// Parse runtime string to uint
	if m.RuntimeStr != "" && m.RuntimeStr != "0" {
		if getrun, err := strconv.Atoi(m.RuntimeStr); err == nil {
			runtime = uint(getrun) //nolint:gosec // safe: value within target type range
		}
	}

	switch s.Cfgp.IsType {
	case config.MediaTypeSeries:
		{
			if identifiedby == "date" ||
				(len(m.Episodes) > 0 && database.Getdatarow[bool](
					false,
					mtstrings.GetStringsMap(s.Cfgp.IsType, "SelectIgnoreRuntime"),
					&m.Episodes[0].Num1,
				)) {
				runtime = 0
			}

			oldfiles = o.Oldfiles
		}
	}

	// Parse file additional info
	err := s.ParseFileAdditional(ctx, o, m, runtime, deletewronglanguage, checkruntime, cfgquality)
	if err != nil {
		return err
	}

	// Check old file priority for media types that require it (movies)
	if mediatype.ShouldCheckOldFilePriority(s.Cfgp.IsType) {
		oldpriority, oldfiles = searcher.Getpriobyfiles(
			s.Cfgp.IsType,
			mediaID,
			true,
			m.Priority,
			cfgquality,
			true,
		)
		if oldpriority != 0 && oldpriority >= m.Priority {
			if err := s.fileCleanup(o.Folder, o.MediaFile, o.Rootpath); err != nil {
				return err
			}

			return errLowerQuality
		}
	}

	// Generate naming template
	s.GenerateNamingTemplate(o, m, namingID)

	if o.Filename == "" {
		return errGeneratingFilename
	}

	// Move old files to replaced folder if configured
	if s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "" &&
		len(oldfiles) >= 1 {
		if err := s.moveremoveoldfiles(o, false, mediaID, true, oldfiles); err != nil {
			return err
		}
	}

	// Apply presort path if configured
	if s.targetpathCfg.Usepresort && s.targetpathCfg.PresortFolderPath != "" {
		o.Rootpath = filepath.Join(s.targetpathCfg.PresortFolderPath, o.Foldername)
	}

	// Move new files to target folder
	newpath, err := s.moveMediaFile(o)
	if err != nil {
		if errors.Is(err, logger.ErrNotFound) {
			return nil
		}

		return err
	}

	// Move and cleanup - get the right dbID for cleanup
	var cleanupDbID *uint
	switch s.Cfgp.IsType {
	case config.MediaTypeMovie:
		cleanupDbID = &m.DbmovieID
	case config.MediaTypeSeries:
		cleanupDbID = &m.Episodes[0].Num2
	case config.MediaTypeBook:
		cleanupDbID = &m.DbbookID
	case config.MediaTypeAudiobook:
		cleanupDbID = &m.DbaudiobookID
	case config.MediaTypeMusic:
		cleanupDbID = &m.DbalbumID
	}

	if err := s.moveandcleanup(o, newpath, m, mediaID, cleanupDbID, oldfiles); err != nil {
		return err
	}

	// Write rename log if configured (movies/series/books)
	if s.Cfgp.IsType == config.MediaTypeMovie || s.Cfgp.IsType == config.MediaTypeSeries ||
		s.Cfgp.IsType == config.MediaTypeBook {
		if dataCfg := s.getDataConfig(); dataCfg != nil && dataCfg.WriteRenameLog {
			s.writeRenameLogSingle(o.TargetPath, o.Folder, o, m)
		}
	}

	// Calculate quality reached
	var reached int
	if m.Priority >= cfgquality.CutoffPriority {
		reached = 1
	}

	fileext := filepath.Ext(newpath)
	filebase := filepath.Base(newpath)
	insertQuery := mtstrings.GetStringsMap(s.Cfgp.IsType, "InsertFileOrganize")
	updateQuery := mtstrings.GetStringsMap(s.Cfgp.IsType, "UpdateMissingReached")

	// Insert file records and update status
	switch s.Cfgp.IsType {
	case config.MediaTypeMovie:
		database.ExecN(insertQuery,
			&newpath, &filebase, &fileext, &cfgquality.Name,
			&m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID,
			&m.Proper, &m.Repack, &m.Extended,
			&m.MovieID, &m.DbmovieID,
			&m.Height, &m.Width,
		)
		database.ExecN(updateQuery, &reached, &m.MovieID)

	case config.MediaTypeSeries:
		for idx := range m.Episodes {
			database.ExecN(insertQuery,
				&newpath, &filebase, &fileext, &cfgquality.Name,
				&m.ResolutionID, &m.QualityID, &m.CodecID, &m.AudioID,
				&m.Proper, &m.Repack, &m.Extended,
				&m.SerieID, &m.Episodes[idx].Num1, &m.Episodes[idx].Num2, &m.DbserieID,
				&m.Height, &m.Width,
			)
			database.ExecN(updateQuery, reached, m.Episodes[idx].Num1)
		}

	case config.MediaTypeBook:
		database.ExecN(insertQuery,
			&newpath, &filebase, &fileext, &cfgquality.Name,
			&m.BookID, &m.DbbookID,
		)
		database.ExecN(updateQuery, &reached, &m.BookID)

	case config.MediaTypeAudiobook:
		database.ExecN(insertQuery,
			&newpath, &filebase, &fileext, &cfgquality.Name,
			&m.AudiobookID, &m.DbaudiobookID,
		)
		database.ExecN(updateQuery, &reached, &m.AudiobookID)

	case config.MediaTypeMusic:
		database.ExecN(insertQuery,
			&newpath, &filebase, &fileext, &cfgquality.Name,
			&m.AlbumID, &m.DbalbumID,
		)
		database.ExecN(updateQuery, &reached, &m.AlbumID)
	}

	// Update caches - use helper functions to get cache keys by type
	generalCfg := config.GetSettingsGeneral()
	if generalCfg.UseMediaCache || generalCfg.UseFileCache {
		cacheUnmatched := mtstrings.GetStringsMap(s.Cfgp.IsType, logger.CacheUnmatched)
		cacheFiles := mtstrings.GetStringsMap(s.Cfgp.IsType, logger.CacheFiles)

		database.SlicesCacheContainsDelete(cacheUnmatched, newpath)
		database.AppendCache(cacheFiles, newpath)
	}

	// Suppress unused variable warnings
	_ = dbMediaID

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
	// Get handler once for reuse
	h := mediatype.Get(s.Cfgp.IsType)

	// Update rootpath
	if !s.targetpathCfg.Usepresort {
		if database.Getdatarow[string](
			false,
			mtstrings.GetStringsMap(s.Cfgp.IsType, logger.DBRootPathFromMediaID),
			id,
		) == "" && h != nil {
			mediaID := h.GetMediaID(m)
			UpdateRootpath(o.TargetPath, h.GetTableName(), &mediaID, s.Cfgp)
		}
	}

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

	if h == nil {
		return nil
	}

	// Cache general settings for the closure
	generalCfg := config.GetSettingsGeneral()
	moveOpts := scanner.MoveFileOptions{
		UseBufferCopy: generalCfg.UseFileBufferCopy,
		Chmod:         s.targetpathCfg.SetChmod,
		ChmodFolder:   s.targetpathCfg.SetChmodFolder,
		UseOther:      true,
		MediaType:     s.Cfgp.IsType,
	}

	return h.MoveOtherFilesAfterOrganize(&mediatype.MoveOtherFilesParams{
		Folder:                 o.Folder,
		Rootpath:               o.Rootpath,
		MediaFile:              o.MediaFile,
		TargetPath:             o.TargetPath,
		Filename:               o.Filename,
		PathCfgName:            s.sourcepathCfg.Name,
		AllowedOtherExtensions: s.sourcepathCfg.AllowedOtherExtensions,
		WalkCleanupFn:          func(rp, vt, fn string) { s.walkcleanup(rp, vt, fn, false, &o.RenamedFiles) },
		MoveFileFn: func(source, target, filename string) error {
			newpath, err := scanner.MoveFile(source, s.sourcepathCfg, target, filename, moveOpts)
			if err == nil {
				o.RenamedFiles = append(o.RenamedFiles, RenameEntry{
					OldName: filepath.Base(source),
					NewName: filepath.Base(newpath),
				})
			}

			return err
		},
		NotifyFn:        func() { s.notify(o, m, dbid, oldfiles) },
		CleanupFolderFn: func() { s.cleanUpFolder(o.Folder) },
	})
}

// walkcleanup recursively walks the given root path and cleans up files.
// It calls filepath.WalkDir to traverse all files under the root path.
// For each file, it checks if it should be filtered via scanner.Filterfile.
// If so, it will either remove the file or move it to the target folder,
// depending on the useremove parameter.
// Any errors during walking or moving/removing are logged.
func (s *Organizer) walkcleanup(
	rootpath, videotarget, filename string,
	useremove bool,
	renames *[]RenameEntry,
) error {
	if rootpath == "" {
		return nil
	}

	// Cache settings outside the walk function to avoid repeated lookups
	var moveOpts scanner.MoveFileOptions
	if !useremove {
		moveOpts = scanner.MoveFileOptions{
			UseBufferCopy: config.GetSettingsGeneral().UseFileBufferCopy,
			Chmod:         s.targetpathCfg.SetChmod,
			ChmodFolder:   s.targetpathCfg.SetChmodFolder,
			UseOther:      true,
			MediaType:     s.Cfgp.IsType,
		}
	}

	checkBlocked := s.sourcepathCfg.BlockedLen >= 1

	return filepath.WalkDir(rootpath, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}

		if info.IsDir() || fpath == rootpath {
			return nil
		}

		ok, _ := scanner.CheckExtensionsType(
			s.Cfgp.IsType,
			true,
			s.sourcepathCfg,
			filepath.Ext(info.Name()),
		)
		if !ok {
			return nil
		}

		// Check IgnoredPaths
		if checkBlocked && logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
			return nil
		}

		if useremove {
			scanner.RemoveFile(fpath)
		} else {
			newpath, err := scanner.MoveFile(
				fpath,
				s.sourcepathCfg,
				videotarget,
				filename,
				moveOpts,
			)
			if err != nil && !errors.Is(err, logger.ErrNotFound) {
				logger.Logtype("error", 1).
					Str(logger.StrFile, fpath).
					Err(err).
					Msg("file move")
			} else if err == nil && renames != nil {
				*renames = append(*renames, RenameEntry{
					OldName: filepath.Base(fpath),
					NewName: filepath.Base(newpath),
				})
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
			if teststr == o.Filename || strings.HasPrefix(teststr, o.TargetPath) {
				continue
			}
		}

		if !scanner.CheckFileExist(oldfiles[idx]) {
			continue
		}

		err = s.moveRemoveOldMediaFile(oldfiles[idx], &oldfiles[idx], id, move)
		if err != nil {
			// Continue if old cannot be moved
			logger.Logtype("error", 1).
				Str(logger.StrFile, oldfiles[idx]).
				Err(err).
				Msg("Move old")

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
		Targetpath:    filepath.Join(o.TargetPath, o.Filename),
		SourcePath:    o.MediaFile,
		Configuration: s.Cfgp.Lists[o.Listid].Name,
		Source:        m,
		Replaced:      oldfiles,
		Time:          logger.TimeGetNow().Format(logger.GetTimeFormat()),
	}

	handler := mediatype.Get(s.Cfgp.IsType)
	if handler == nil {
		return
	}

	title, year, externalID, series, season, episode, identifier, ok := handler.FillNotifyData(id)
	if !ok {
		return
	}

	notify.Title = title
	notify.Year = year
	notify.Series = series
	notify.Season = season
	notify.Episode = episode
	notify.Identifier = identifier

	// Set type-specific external ID field
	if s.Cfgp.IsType == config.MediaTypeMovie {
		notify.Imdb = externalID
	} else {
		notify.Tvdb = externalID
	}

	var err error
	for idx := range s.Cfgp.Notification {
		if s.Cfgp.Notification[idx].CfgNotification == nil {
			continue
		}

		// Use "upgraded_data" event if files were replaced, otherwise "added_data"
		expectedEvent := "added_data"
		if len(notify.Replaced) > 0 {
			expectedEvent = "upgraded_data"
		}

		if !strings.EqualFold(s.Cfgp.Notification[idx].Event, expectedEvent) {
			continue
		}

		notify.ReplacedPrefix = s.Cfgp.Notification[idx].ReplacedPrefix

		bl, messagetext, _ := logger.ParseStringTemplate(s.Cfgp.Notification[idx].Message, &notify)
		if bl {
			continue
		}

		cfgnot := config.GetSettingsNotification(s.Cfgp.Notification[idx].MapNotification)

		switch cfgnot.NotificationType {
		case "pushover":
			bl, messageTitle, _ := logger.ParseStringTemplate(
				s.Cfgp.Notification[idx].Title,
				&notify,
			)
			if bl {
				continue
			}

			err = apiexternal.SendPushoverMessage(
				cfgnot.Name,
				cfgnot.Apikey,
				messagetext,
				messageTitle,
				cfgnot.Recipient,
			)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("Error sending pushover")
			} else {
				logger.Logtype("info", 0).
					Msg("Pushover message sent")
			}

		case "gotify":
			bl, messageTitle, _ := logger.ParseStringTemplate(
				s.Cfgp.Notification[idx].Title,
				&notify,
			)
			if bl {
				continue
			}

			err = apiexternal.SendGotifyMessage(
				cfgnot.Name,
				cfgnot.ServerURL,
				cfgnot.Apikey,
				messagetext,
				messageTitle,
			)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("Error sending Gotify notification")
			} else {
				logger.Logtype("info", 0).
					Msg("Gotify message sent")
			}

		case "pushbullet":
			bl, messageTitle, _ := logger.ParseStringTemplate(
				s.Cfgp.Notification[idx].Title,
				&notify,
			)
			if bl {
				continue
			}

			err = apiexternal.SendPushbulletMessage(
				cfgnot.Name,
				cfgnot.Apikey,
				messagetext,
				messageTitle,
			)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("Error sending Pushbullet notification")
			} else {
				logger.Logtype("info", 0).
					Msg("Pushbullet message sent")
			}

		case "apprise":
			bl, messageTitle, _ := logger.ParseStringTemplate(
				s.Cfgp.Notification[idx].Title,
				&notify,
			)
			if bl {
				continue
			}

			err = apiexternal.SendAppriseMessage(
				cfgnot.Name,
				cfgnot.ServerURL,
				messagetext,
				messageTitle,
				cfgnot.AppriseURLs,
			)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("Error sending Apprise notification")
			} else {
				logger.Logtype("info", 0).
					Msg("Apprise message sent")
			}

		case "csv":
			scanner.AppendCsv(
				cfgnot.Outputto,
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
			config.MediaTypeSeries,
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
			if len(m.Episodes) > 0 &&
				database.Getdatarow[uint](
					false,
					"select count() from serie_episode_files where serie_episode_id = ?",
					&m.Episodes[0].Num1,
				) == 0 {
				bl = true
			} else if !skipdelete {
				bl, err = scanner.RemoveFile(o.MediaFile)
				if err == nil && bl {
					logger.Logtype("info", 3).
						Str(logger.StrPath, o.MediaFile).
						Int(strOldPrio, oldPrio).
						Int(logger.StrPriority, m.Priority).
						Msg("Lower Qual Import File removed")
					s.removeotherfiles(o.MediaFile)
					s.cleanUpFolder(o.Folder)

					bl = false
				} else if err != nil {
					logger.Logtype("error", 0).
						Err(err).
						Msg("delete Files")
					clear(m.Episodes)

					m.Episodes = m.Episodes[:0]

					return err
				} else {
					bl = false
				}
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
			config.MediaTypeSeries,
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
			if database.Getdatarow[uint](
				false,
				"select count() from serie_episode_files where serie_episode_id = ?",
				&m.Episodes[idx].Num1,
			) == 0 {
				bl = true
			} else if !skipdelete {
				bl, err = scanner.RemoveFile(o.MediaFile)
				if err == nil && bl {
					logger.Logtype("info", 3).
						Str(logger.StrPath, o.MediaFile).
						Int(strOldPrio, oldPrio).
						Int(logger.StrPriority, m.Priority).
						Msg("Lower Qual Import File removed")
					s.removeotherfiles(o.MediaFile)
					s.cleanUpFolder(o.Folder)

					bl = false

					break
				}

				if err != nil {
					logger.Logtype("error", 0).
						Err(err).
						Msg("delete Files")
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
) error {
	s := NewStructure(
		cfgp,
		data.TemplatePath,
		defaulttemplate,
		checkruntime,
		deleteWrongLanguage,
		manualid,
	)
	if s == nil {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, data.TemplatePath).
			Msg("structure not found")
		return logger.ErrNotFound
	}

	defer s.Close()

	// Unpack archives before processing media files if enabled
	if err := unpackArchivesInFolder(ctx, folder, data); err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrPath, folder).
			Err(err).
			Msg("Failed to unpack archives")
		// Continue processing even if unpacking fails
	}

	// Skip folders where files were modified within the last minute (still being written)
	if folderHasRecentFiles(folder) {
		logger.Logtype("info", 1).
			Str(logger.StrPath, folder).
			Msg("skipped - folder has recently modified files")
		return nil
	}

	// For multi-file media (music, audiobooks), use the album processing workflow
	// This matches files to albums via API, handles upgrades, and moves complete albums
	if cfgp.IsType == config.MediaTypeMusic || cfgp.IsType == config.MediaTypeAudiobook {
		// Skip folders that are still being unpacked
		if logger.ContainsI(folder, "_unpack") {
			logger.Logtype("warn", 1).
				Str(logger.StrPath, folder).
				Msg("skipped - unpacking")
			return nil
		}

		// Skip disallowed folders
		if logger.SlicesContainsPart2I(s.sourcepathCfg.Disallowed, folder) {
			logger.Logtype("warn", 1).
				Str(logger.StrPath, folder).
				Msg("skipped - disallowed")
			return nil
		}

		// Skip blocked folders
		if s.sourcepathCfg.BlockedLen >= 1 &&
			logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, folder) {
			return nil
		}

		reason, matchReport, err := s.organizeAlbumFolderViaAPI(ctx, folder, cfgp, data)
		if err != nil && reason != "" && data.MoveUnprocessed != "" &&
			scanner.CheckFileExist(folder) {
			moveUnprocessedFolder(
				folder,
				data.MoveUnprocessed,
				reason,
				s.targetpathCfg.SetChmod,
				s.targetpathCfg.SetChmodFolder,
				matchReport,
			)
			s.cleanUpFolder(folder)
		}

		if errors.Is(err, errUnprocessed) {
			return nil
		}

		if err == nil {
			s.cleanUpFolder(folder)
		}

		return err
	}

	// For single-file media (movies, series, books), process each file individually
	importAddFound := data != nil && data.AddFound

	var (
		anyOrganized, anySkippedTemporary bool
		lastMoveReason                    string
	)

	walkErr := filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
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
			logger.Logtype("warn", 1).
				Str(logger.StrFile, fpath).
				Msg("skipped - unpacking")
				// logpointerr
			return fs.SkipDir
		}

		if logger.SlicesContainsPart2I(s.sourcepathCfg.Disallowed, fpath) {
			logger.Logtype("warn", 1).
				Str(logger.StrFile, fpath).
				Msg("skipped - disallowed")
				// logpointerr
			return fs.SkipDir
		}

		// CheckUnmatched
		if config.GetSettingsGeneral().UseFileCache {
			if database.SlicesCacheContains(cfgp.IsType, logger.CacheUnmatched, fpath) {
				return nil
			}
		} else {
			if database.Getdatarow[uint](
				false,
				mtstrings.GetStringsMap(cfgp.IsType, logger.DBCountUnmatchedPath),
				fpath,
			) >= 1 {
				return nil
			}
		}

		ok, _ := scanner.CheckExtensionsType(
			cfgp.IsType,
			false,
			s.sourcepathCfg,
			filepath.Ext(info.Name()),
		)

		// Check IgnoredPaths

		if !ok {
			return nil
		}

		if s.sourcepathCfg.BlockedLen >= 1 &&
			logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
			return nil
		}

		organized, moveReason, result := s.walkorganizefolder(
			ctx,
			fpath,
			folder,
			cfgp,
			importAddFound,
		)
		if organized {
			anyOrganized = true
		}

		if moveReason != "" {
			lastMoveReason = moveReason
		}

		if errors.Is(result, errTooRecentlyModified) {
			anySkippedTemporary = true
			return nil
		}

		return result
	})

	if !anyOrganized && !anySkippedTemporary && data.MoveUnprocessed != "" &&
		lastMoveReason != "" && scanner.CheckFileExist(folder) {
		moveUnprocessedFolder(
			folder,
			data.MoveUnprocessed,
			lastMoveReason,
			s.targetpathCfg.SetChmod,
			s.targetpathCfg.SetChmodFolder,
			nil,
		)
		s.cleanUpFolder(folder)
	}

	return walkErr
}

// walkorganizefolder is a method of the Organizer struct that processes a file path, parses the file, and organizes the media item based on the configuration settings.
// It performs various checks and validations on the file, such as checking for disallowed subtitle files, minimum video size, and valid IDs. It then updates the media item's metadata and organizes the file accordingly.
// If any errors occur during the process, it logs the errors and adds the file to the unmatched list.
func (s *Organizer) walkorganizefolder(
	ctx context.Context,
	fpath, folder string,
	cfgp *config.MediaTypeConfig,
	addFound bool,
) (bool, string, error) {
	m := parser_v2.ParseFile(fpath, true, true, cfgp, -1)
	if m == nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, fpath).
			Msg("parse failed")
		return false, "no_match", nil
	}

	defer m.Close()

	err := parser.GetDBIDs(m, cfgp, true, addFound)
	if err != nil || !s.hasValidIDs(m) {
		logger.Logtype("warn", 1).
			Str(logger.StrFile, fpath).
			Err(err).
			Msg(logger.ParseFailedIDs)

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, err)

		return false, "no_match", nil
	}

	if s.sourcepathCfg.MinVideoSize > 0 {
		info, err := os.Stat(fpath)
		if err == nil {
			if info.Size() < s.sourcepathCfg.MinVideoSizeByte {
				logger.Logtype("warn", 1).
					Str(logger.StrFile, fpath).
					Msg("skipped - small files")

				if s.sourcepathCfg.Name == "" {
					m.TempTitle = fpath
					m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("small file"))
					return false, "small_file", fs.SkipDir
				}

				ok, oknorename := scanner.CheckExtensionsType(
					cfgp.IsType,
					false,
					s.sourcepathCfg,
					filepath.Ext(fpath),
				)

				shouldRemove := ok || oknorename ||
					!mediatype.HasConfiguredExtensions(cfgp.IsType, s.sourcepathCfg)

				if shouldRemove {
					scanner.SecureRemove(fpath)
				}

				m.TempTitle = fpath
				m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("small file"))

				// Only trigger move if file was not deleted
				if shouldRemove {
					return false, "", fs.SkipDir
				}

				return false, "small_file", fs.SkipDir
			}

			if logger.TimeAfter(info.ModTime(), logger.TimeGetNow().Add(-2*time.Minute)) {
				logger.Logtype("error", 1).
					Str(logger.StrFile, fpath).
					Msg("file modified too recently")
				return false, "", errTooRecentlyModified
			}
		}
	}

	if h := mediatype.Get(s.Cfgp.IsType); h != nil {
		h.SetTempID(m)
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(s.Cfgp, &m.TempID)

		if m.ListID == -1 {
			logger.Logtype("warn", 1).
				Str(logger.StrFile, fpath).
				Msg("listcfg not found")

			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("listcfg not found"))

			return false, "no_list", nil
		}
	}

	if config.GetSettingsGeneral().UseFileCache {
		database.SlicesCacheContainsDelete(
			mtstrings.GetStringsMap(s.Cfgp.IsType, logger.CacheUnmatched),
			fpath,
		)
	}

	if h := mediatype.Get(s.Cfgp.IsType); h != nil {
		if !h.ValidateIDs(m) {
			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no valid IDs"))
			return false, "no_match", nil
		}
	}

	if s.checksubfiles(folder, fpath, "") {
		logger.Logtype("error", 1).
			Str(logger.StrFile, fpath).
			Msg("check sub files")

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("check sub files"))

		return false, "subfiles_check", nil
	}

	if s.manualID != 0 {
		mediatype.Get(s.Cfgp.IsType).SetMediaID(m, s.manualID)
	}

	var (
		dbid     uint
		listname string
		rootpath string
	)

	database.GetdatarowArgs(
		mtstrings.GetStringsMap(s.Cfgp.IsType, "GetOrganizeData"),
		&m.TempID,
		&dbid,
		&rootpath,
		&listname,
	)

	if dbid == 0 {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no dbid found"))
		return false, "no_match", logger.ErrNotFound
	}

	if h := mediatype.Get(s.Cfgp.IsType); h != nil {
		h.SetDBID(m, dbid)
	}

	if m.ListID == -1 {
		m.ListID = s.Cfgp.GetMediaListsEntryListID(listname)
	}

	if m.Checktitle(s.Cfgp, s.Cfgp.Lists[m.ListID].CfgQuality, filepath.Base(fpath)) {
		logger.Logtype("warn", 1).
			Str(logger.StrFile, fpath).
			Msg("skipped - unwanted title")

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("unwanted title"))

		return false, "unwanted_title", nil
	}

	if m.ListID == -1 {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errors.New("no ListID found"))
		return false, "no_list", logger.ErrListnameTemplateEmpty
	}

	o := Organizerdata{Folder: folder, MediaFile: fpath, Listid: m.ListID, Rootpath: rootpath}
	h := mediatype.Get(s.Cfgp.IsType)

	err = s.Organize(
		ctx,
		&o,
		m,
		s.Cfgp.Lists[m.ListID].CfgQuality,
		s.deletewronglanguage,
		s.checkruntime,
	)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, fpath).
			Err(err).
			Msg("structure")

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, err)

		var moveReason string
		switch {
		case errors.Is(err, errWrongRuntime) || errors.Is(err, errRuntime):
			moveReason = "wrong_runtime"
		case errors.Is(err, errWrongLanguage):
			moveReason = "wrong_language"
		case errors.Is(err, errGeneratingFilename):
			moveReason = "naming_failed"
		default:
			moveReason = "organize_failed"
		}

		return false, moveReason, nil
	}

	h.ClearUnmatchedCache(fpath)

	s.cleanUpFolder(folder)

	return true, "", nil
}

// checksubfiles checks for any disallowed subtitle files in the same
// folder as the video file. It also checks if there are multiple files
// with the same extension, which indicates it may not be a standalone movie.
// It returns an error if disallowed files are found or too many matching files exist.
func (s *Organizer) checksubfiles(folder, videofile, rootpath string) bool {
	if folder == "" {
		return false
	}

	var (
		disallowed bool
		count      int8
	)

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

			if handler := mediatype.Get(s.Cfgp.IsType); handler != nil &&
				handler.SkipMultipleFiles() &&
				count >= 2 {
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

		logger.Logtype("warn", 1).
			Str(logger.StrFile, videofile).
			Msg("skipped - disallowed")

		return true
	}

	if handler := mediatype.Get(s.Cfgp.IsType); handler != nil && handler.SkipMultipleFiles() &&
		count >= 2 {
		logger.Logtype("warn", 1).
			Str(logger.StrFile, videofile).
			Msg("skipped - too many files")
		return true
	}

	return false
}

// hasValidIDs checks if the parser has valid IDs for the media item.
// For series it checks episode ID and series ID.
// For movies it checks movie ID.
// It uses the config to determine if this is a series or movie.
func (s *Organizer) hasValidIDs(m *database.ParseInfo) bool {
	return mediatype.Get(s.Cfgp.IsType).ValidateIDs(m)
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
		logger.Logtype("error", 1).
			Str(logger.StrFile, sourcepathstr).
			Err(logger.ErrCfgpNotFound).
			Msg("parse failed cfgp")

		return nil
	}

	// Get path configs once and reuse
	sourceCfg := config.GetSettingsPath(sourcepathstr)
	if sourceCfg == nil {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, sourcepathstr).
			Msg("structure source not found")
		return nil
	}

	targetCfg := config.GetSettingsPath(targetpathstr)
	if targetCfg == nil {
		logger.Logtype("error", 1).
			Str(logger.StrConfig, targetpathstr).
			Msg("structure target not found")
		return nil
	}

	if sourceCfg.Name == "" {
		logger.Logtype("error", 1).
			Str(logger.StrFile, sourcepathstr).
			Str("name", sourceCfg.Name).
			Msg("template not found")

		return nil
	}

	o := plStructure.Get()

	o.checkruntime = checkruntime
	o.deletewronglanguage = deletewronglanguage
	o.manualID = manualid
	o.sourcepathCfg = sourceCfg
	o.targetpathCfg = targetCfg
	o.Cfgp = cfgp

	return o
}

// checksplit checks if the given folder name contains a '/' or '\'
// path separator and returns the detected separator byte.
// It is used to determine the path separator used in a folder name.
// Optimized to use strings.IndexAny for single-pass detection.
func checksplit(foldername string) byte {
	idx := strings.IndexAny(foldername, "/\\")
	if idx == -1 {
		return ' '
	}

	return foldername[idx]
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
		// Use IndexAny for single-pass check instead of two ContainsRune calls
		if strings.ContainsAny(firstfolder, "/\\") {
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

// getrootpath returns the root path of the given folder name by splitting on '/' or '\'
// and trimming any trailing slashes. If no slashes, it just trims trailing slashes.
// Optimized to use strings.IndexAny for single-pass detection of path separators.
func getrootpath(foldername string) string {
	// Find first path separator (either / or \)
	idx := strings.IndexAny(foldername, "/\\")
	if idx == -1 {
		// No separator found, just trim trailing slashes
		return logger.Trim(foldername, '/')
	}

	// Handle single backslash at start - return the original string
	// This preserves backward compatibility with the original implementation
	if idx == 0 && foldername[0] == '\\' && len(foldername) == 1 {
		return foldername
	}

	// Get the part before the first separator
	root := foldername[:idx]

	// Trim trailing slash if present and root is not empty
	if len(root) > 0 && root[len(root)-1] == '/' {
		return root[:len(root)-1]
	}

	return root
}

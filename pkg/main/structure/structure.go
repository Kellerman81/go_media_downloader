package structure

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/downloadstate"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
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
	// forced bypasses normal candidate search (ForceMatchAlbumFolder run mode).
	forced AlbumForce
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
	// Book / audiobook / music metadata structs, populated from the database so
	// notification templates can reference rich fields for those media types.
	Dbaudiobook        database.Dbaudiobook
	DbaudiobookChapter database.DbaudiobookChapter
	Dbbook             database.Dbbook
	Dbalbum            database.Dbalbum
	Dbtrack            database.Dbtrack
	Author             database.Dbauthor
	BookSeries         database.DbbookSeries
	Artist             database.Dbartist
	AlbumArtist        database.Dbartist
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
	strOldPrio               = "old prio"
	errLowerQuality          = errors.New("lower quality")
	errSeasonEmpty           = errors.New("season empty")
	errGeneratingFilename    = errors.New("generating filename")
	errWrongRuntime          = errors.New("wrong runtime")
	errWrongLanguage         = errors.New("wrong language")
	errUnprocessed           = errors.New("unprocessed")
	errTooRecentlyModified   = errors.New("file modified too recently")
	errCleanupFolderNotFound = errors.New("cleanup folder not found")
	errSmallFile             = errors.New("small file")
	errListcfgNotFound       = errors.New("listcfg not found")
	errNoValidIDs            = errors.New("no valid IDs")
	errCheckSubFiles         = errors.New("check sub files")
	errNoDbidFound           = errors.New("no dbid found")
	errUnwantedTitle         = errors.New("unwanted title")
	errNoListIDFound         = errors.New("no ListID found")
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
		Dbalbum: database.Dbalbum{
			Title:       "The Dark Side of the Moon",
			Year:        1973,
			ReleaseType: "album",
			Format:      "vinyl",
			Label:       "Harvest",
			Genres:      "Progressive Rock",
			TotalTracks: 10,
		},
		Dbtrack:     database.Dbtrack{Title: "Time", TrackNumber: 4, DiscNumber: 1},
		Artist:      database.Dbartist{Name: "Pink Floyd", SortName: "Pink Floyd"},
		AlbumArtist: database.Dbartist{Name: "Pink Floyd"},
		Dbbook: database.Dbbook{
			Title:          "The Hobbit",
			Year:           1937,
			ISBN13:         "9780261103344",
			Publisher:      "George Allen & Unwin",
			Language:       "English",
			SeriesPosition: "1",
		},
		Author:     database.Dbauthor{Name: "J.R.R. Tolkien"},
		BookSeries: database.DbbookSeries{Name: "Middle-earth"},
		Dbaudiobook: database.Dbaudiobook{
			Title:          "The Hobbit",
			Year:           2012,
			ASIN:           "B007978PI2",
			Publisher:      "Recorded Books",
			Language:       "English",
			SeriesName:     "The Lord of the Rings",
			SeriesPosition: "1",
			RuntimeMinutes: 660,
		},
		DbaudiobookChapter: database.DbaudiobookChapter{Title: "An Unexpected Party", ChapterNumber: 1},
		Title:              "Breaking Bad",
		Year:               "2008",
		Season:             "1",
		Episode:            "1",
		Series:             "Breaking Bad",
		EpisodeTitle:       "Pilot",
		Configuration:      "series-hd",
		SourcePath:         "/downloads/Breaking.Bad.S01E01.Pilot.1080p.BluRay.x264-ROVERS.mkv",
		Targetpath:         "/media/tv/Breaking Bad/Season 01/Breaking Bad - S01E01 - Pilot.mkv",
		Imdb:               "tt0903747",
		Tvdb:               "81189",
		Time:               "2024-01-15 14:30:00",
		ReplacedPrefix:     "Replaced: ",
		Replaced:           []string{"/old/Breaking.Bad.S01E01.720p.mkv"},
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
		Dbalbum: database.Dbalbum{
			Title:       "The Dark Side of the Moon",
			Year:        1973,
			ReleaseType: "album",
			Format:      "vinyl",
			Label:       "Harvest",
			Genres:      "Progressive Rock",
			TotalTracks: 10,
		},
		Dbtrack: database.Dbtrack{
			Title:       "Time",
			TrackNumber: 4,
			DiscNumber:  1,
		},
		Artist:      database.Dbartist{Name: "Pink Floyd", SortName: "Pink Floyd"},
		AlbumArtist: database.Dbartist{Name: "Pink Floyd"},
		Dbbook: database.Dbbook{
			Title:          "The Hobbit",
			OriginalTitle:  "The Hobbit",
			Year:           1937,
			ISBN13:         "9780261103344",
			Publisher:      "George Allen & Unwin",
			Language:       "English",
			SeriesPosition: "1",
		},
		Author:     database.Dbauthor{Name: "J.R.R. Tolkien"},
		BookSeries: database.DbbookSeries{Name: "Middle-earth"},
		Dbaudiobook: database.Dbaudiobook{
			Title:          "The Hobbit",
			Year:           2012,
			ASIN:           "B007978PI2",
			Publisher:      "Recorded Books",
			Language:       "English",
			SeriesName:     "The Lord of the Rings",
			SeriesPosition: "1",
			RuntimeMinutes: 660,
		},
		DbaudiobookChapter: database.DbaudiobookChapter{
			Title:         "An Unexpected Party",
			ChapterNumber: 1,
		},
		Title:              "The Hobbit",
		Track:              4,
		Disc:               1,
		TotalDiscs:         1,
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
			// rootpath is the item's library folder: walking it would delete the
			// existing copy's subtitles/NFOs whenever a download is rejected.
			// Clean the rejected download's own folder instead, never the
			// download root itself.
			func(string) {
				if folder != s.sourcepathCfg.Path {
					s.walkcleanup(folder)
				}
			},
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
		return errCleanupFolderNotFound
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

	// "Smaller than" the threshold: a folder holding exactly CleanupsizeMB stays.
	if leftsize < cleanupsizeByte || leftsize == 0 {
		// Try removal directly; only walk the tree to clear read-only bits if the
		// first attempt fails. This avoids a second full walk in the common case.
		if err := os.RemoveAll(folder); err != nil {
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
		}

		logger.Logtype("info", 1).
			Str(logger.StrFile, folder).
			Msg("Folder removed")
	}

	return nil
}

// cleanUpAlbumFolder is cleanUpFolder for album folders: it keeps a folder
// that still holds audio files. An album organize only moves the files that
// matched a track, and the size-based cleanup would delete the rest with the
// folder (a 10-file folder matched to an 8-track release lost 2 files).
func (s *Organizer) cleanUpAlbumFolder(folder string) {
	remaining := false

	filepath.WalkDir(folder, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && parser_v2.HasExtension(path, parser_v2.AudioExtensions) {
			remaining = true
			return filepath.SkipAll
		}

		return nil
	})

	if remaining {
		logger.Logtype("warn", 1).
			Str(logger.StrPath, folder).
			Msg("audio files left in source folder after album organize - folder kept")

		return
	}

	s.cleanUpFolder(folder)
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
	wantedruntime, targetruntime, wrong := s.runtimeMismatch(m, runtime, checkruntime)
	if !wrong {
		return nil
	}

	// Without a tolerance the file is rejected but kept.
	if s.targetpathCfg.MaxRuntimeDifference != 0 && s.targetpathCfg.DeleteWrongRuntime {
		s.fileCleanup(o.Folder, o.MediaFile, o.Rootpath)
	}

	logger.Logtype("warn", 2).
		Int(logger.StrWanted, wantedruntime).
		Int(logger.StrFound, targetruntime).
		Msg("wrong runtime")

	return errWrongRuntime
}

// runtimeMismatch compares the probed runtime of m with the wanted runtime
// in minutes and reports whether validateRuntime rejects the file.
func (s *Organizer) runtimeMismatch(
	m *database.ParseInfo,
	runtime uint,
	checkruntime bool,
) (wanted, found int, wrong bool) {
	if m.Runtime < 1 || !checkruntime || runtime == 0 {
		return 0, 0, false
	}

	wanted = int(runtime) * mediatype.GetRuntimeMultiplier(s.Cfgp.IsType, m)

	found = m.Runtime / 60
	if found == wanted {
		return wanted, found, false
	}

	if s.targetpathCfg.MaxRuntimeDifference == 0 {
		return wanted, found, true
	}

	maxdifference := s.targetpathCfg.MaxRuntimeDifference
	if handler := mediatype.Get(s.Cfgp.IsType); handler != nil {
		maxdifference += handler.GetRuntimeBonus(m)
	}

	return wanted, found, abs(wanted-found) > maxdifference
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
	if !deletewronglanguage || s.languageAllowed(m) {
		return nil
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

	logger.Logtype("warn", 2).
		Str(logger.StrFound, foundLang).
		Str(logger.StrWanted, wantedLang).
		Msg("wrong language")

	return errWrongLanguage
}

// languageAllowed reports whether the audio languages of m satisfy the
// target path's allowed_languages; an empty entry allows files without
// language tags.
func (s *Organizer) languageAllowed(m *database.ParseInfo) bool {
	if s.targetpathCfg.AllowedLanguagesLen == 0 {
		return true
	}

	lenlang := len(m.Languages)
	for i := range s.targetpathCfg.AllowedLanguages {
		if (lenlang == 0 && s.targetpathCfg.AllowedLanguages[i] == "") ||
			logger.SlicesContainsI(m.Languages, s.targetpathCfg.AllowedLanguages[i]) {
			return true
		}
	}

	return false
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
	// Every media type's FillNamingData starts by loading its row by this id,
	// so a zero id can only ever end in the failure branch below. Bail before
	// the template split, the debug event, the handler lookup and the DB
	// round-trip instead of paying for all of them first.
	if dbid == nil || *dbid == 0 {
		o.Filename = ""

		logger.Logtype("warn", 0).
			Uint("type", s.Cfgp.IsType).
			Msg("GenerateNamingTemplate called without a database id")

		return
	}

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

		logger.Logtype("warn", 0).Uint("type", s.Cfgp.IsType).Msg("Handler is nil")
		return
	}

	// Use handler to fill naming data
	namingData := &mediatype.NamingData{}

	clearFolder, ok := handler.FillNamingData(dbid, o.MediaFile, m, namingData)
	if !ok {
		o.Filename = ""

		logger.Logtype("warn", 0).
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

// setTargetPath computes o.TargetPath from o.Rootpath/o.Foldername exactly as
// moveMediaFile moves the files there.
func (s *Organizer) setTargetPath(o *Organizerdata) {
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
}

// applyPresort redirects o.Rootpath to the presort folder when configured.
// setTargetPath appends Foldername to Rootpath for movies/series/books (for
// books Foldername is cleared once a rootpath exists), so those get the bare
// presort root here; music/audiobooks use Rootpath as the full album folder
// and get presort/Foldername.
func (s *Organizer) applyPresort(o *Organizerdata) {
	if !s.targetpathCfg.Usepresort || s.targetpathCfg.PresortFolderPath == "" {
		return
	}

	if s.Cfgp.IsType == config.MediaTypeMusic || s.Cfgp.IsType == config.MediaTypeAudiobook {
		o.Rootpath = filepath.Join(s.targetpathCfg.PresortFolderPath, o.Foldername)
	} else {
		o.Rootpath = s.targetpathCfg.PresortFolderPath
	}
}

// replacedMoveOptions returns the MoveFile options used to move replaced
// files into MoveReplacedTargetPath.
func (s *Organizer) replacedMoveOptions() scanner.MoveFileOptions {
	return scanner.MoveFileOptions{
		UseBufferCopy: config.GetSettingsGeneral().UseFileBufferCopy,
		Chmod:         s.targetpathCfg.SetChmod,
		ChmodFolder:   s.targetpathCfg.SetChmodFolder,
		UseNil:        true,
		MediaType:     s.Cfgp.IsType,
	}
}

// stashReplacedFile moves an old file that occupies the exact path a new file
// is about to be moved to into the replaced folder, so MoveFile does not
// overwrite it. Returns the stashed path for restoreStashedFile.
func (s *Organizer) stashReplacedFile(oldfile string) (string, error) {
	return scanner.MoveFile(
		oldfile,
		nil,
		filepath.Join(s.targetpathCfg.MoveReplacedTargetPath, filepath.Base(filepath.Dir(oldfile))),
		"",
		s.replacedMoveOptions(),
	)
}

// restoreStashedFile moves a stashed old file back to its original path after
// the new file could not be moved in. A failure is logged - the file then
// stays in the replaced folder, which still keeps it.
func (s *Organizer) restoreStashedFile(stashed, original string) {
	if scanner.CheckFileExist(original) {
		logger.Logtype("error", 1).
			Str(logger.StrFile, stashed).
			Str("original", original).
			Msg("original path is occupied - replaced file left in replaced folder")

		return
	}

	_, err := scanner.MoveFile(stashed, nil, filepath.Dir(original), "", s.replacedMoveOptions())
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, stashed).
			Str("original", original).
			Err(err).
			Msg("could not restore replaced file - it stays in the replaced folder")
	}
}

// moveMediaFile moves the media file(s) specified in orgadata to the target folder.
// For single-file media (movies, series, books): moves MediaFile to target
// For multi-file media (music, audiobooks): moves all files in MediaFiles to target
// It creates the target folder if needed, setting permissions according to TargetpathCfg.
// Returns the path of the moved file (or first file for multi-file), and an error.
func (s *Organizer) moveMediaFile(o *Organizerdata) (string, error) {
	s.setTargetPath(o)

	// Same 3-or-4 digit rule as scanner's chmod handling; an unparsable
	// value keeps 0777 instead of creating the folder with mode 0.
	mode := fs.FileMode(0o777)
	if l := len(s.targetpathCfg.SetChmodFolder); l == 3 || l == 4 {
		if m := logger.StringToFileMode(s.targetpathCfg.SetChmodFolder); m != 0 {
			mode = m
		}
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

	// Pre-compute target path for moves
	var moveTargetPath string
	if move {
		moveTargetPath = filepath.Join(
			s.targetpathCfg.MoveReplacedTargetPath,
			filepath.Base(filepath.Dir(oldfile)),
		)
	}

	moveOpts := s.replacedMoveOptions()

	// A file that is already gone has reached the goal state of both the move
	// and the delete path - only its stale DB row / cache entry is left to drop.
	_, lerr := os.Lstat(oldfile)
	missing := errors.Is(lerr, fs.ErrNotExist)

	switch {
	case missing:
		logger.Logtype("info", 1).
			Str(logger.StrFile, oldfile).
			Msg("old file no longer on disk - removing stale entry")
	case lerr != nil:
		return lerr
	case move:
		if _, err := scanner.MoveFile(oldfile, nil, moveTargetPath, "", moveOpts); err != nil {
			return err
		}
	default:
		if _, err := scanner.RemoveFile(oldfile); err != nil {
			return err
		}
	}

	s.dropOldFileRecord(oldfile, oldfilep, id)

	if missing {
		return nil
	}

	var (
		err error
		bl  bool
	)

	// Subtitles etc. named after the old file, language-suffixed ones included.
	for _, additionalfile := range companionsOf(oldfile, s.sourcepathCfg.AllowedOtherExtensions) {
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

// dropOldFileRecord removes the DB row and file-cache entry of a replaced file.
func (s *Organizer) dropOldFileRecord(oldfile string, oldfilep *string, id *uint) {
	if config.GetSettingsGeneral().UseFileCache {
		database.SlicesCacheContainsDelete(
			mtstrings.GetStringsMap(s.Cfgp.IsType, logger.CacheFiles),
			oldfile,
		)
	}

	if id != nil {
		database.ExecNMap(s.Cfgp.IsType, logger.DBDeleteFileByIDLocation, id, oldfilep)
		return
	}

	// Music/audiobook callers pass id == nil since they only have a
	// filesystem path. Passing a nil *uint through the shared ID+location
	// query binds as SQL NULL, and "album_id = NULL" never matches any row -
	// delete by location alone for the media types that can reach this.
	switch s.Cfgp.IsType {
	case config.MediaTypeMusic:
		database.ExecN("DELETE FROM album_files WHERE location = ?", oldfilep)
	case config.MediaTypeAudiobook:
		database.ExecN("DELETE FROM audiobook_files WHERE location = ?", oldfilep)
	}
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
				return errWrongRuntime
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

		// Every existing file of the book is replaced by the new one
		// (honouring replace_lower / move_replaced like movies and series).
		oldfiles = database.GetrowsN[string](
			false,
			0,
			"select location from book_files where book_id = ?",
			&m.BookID,
		)

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

	if s.Cfgp.IsType == config.MediaTypeBook {
		if _, lower := bookLowerQuality(m, s.Cfgp, cfgquality); lower {
			return errLowerQuality
		}
	}

	// Generate naming template
	s.GenerateNamingTemplate(o, m, namingID)

	if o.Filename == "" {
		return errGeneratingFilename
	}

	s.applyPresort(o)
	s.setTargetPath(o)

	moveReplaced := s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "" &&
		len(oldfiles) >= 1

	// Replaced files are only moved away after the new file is in place, so a
	// failed move never leaves the library with neither. The one exception is
	// an old file sitting exactly where the new one goes: MoveFile would
	// overwrite it, so it is stashed in the replaced folder first and put back
	// if the new file cannot be moved in.
	var stashedFrom, stashedTo string
	if moveReplaced && len(o.MediaFiles) == 0 {
		planned := scanner.PlannedMovePath(
			o.MediaFile,
			s.sourcepathCfg,
			o.TargetPath,
			o.Filename,
			scanner.MoveFileOptions{MediaType: s.Cfgp.IsType},
		)
		for _, old := range oldfiles {
			if old != planned || old == o.MediaFile || !scanner.CheckFileExist(old) {
				continue
			}

			stashed, err := s.stashReplacedFile(old)
			if err != nil {
				return fmt.Errorf("move replaced file out of the way: %w", err)
			}

			stashedFrom, stashedTo = old, stashed

			break
		}
	}

	// Move new files to target folder
	newpath, err := s.moveMediaFile(o)
	if err != nil {
		if stashedTo != "" {
			s.restoreStashedFile(stashedTo, stashedFrom)
		}

		return err
	}

	if moveReplaced {
		if stashedFrom != "" {
			// Its file is gone; its row still points at the path now holding
			// the new file and must not survive the insert below.
			s.dropOldFileRecord(stashedFrom, &stashedFrom, mediaID)
		}

		remaining := slices.DeleteFunc(slices.Clone(oldfiles), func(r string) bool {
			return r == newpath || r == stashedFrom
		})
		if err := s.moveremoveoldfiles(o, false, mediaID, true, remaining); err != nil {
			return err
		}
	}

	// FillNotifyData expects the dbid (dbserie_episodes id for series), while
	// FillNamingData expects the same id GenerateNamingTemplate got (namingID:
	// serie_episodes id for series).
	var notifyDbID *uint
	switch s.Cfgp.IsType {
	case config.MediaTypeMovie:
		notifyDbID = &m.DbmovieID
	case config.MediaTypeSeries:
		notifyDbID = &m.Episodes[0].Num2
	case config.MediaTypeBook:
		notifyDbID = &m.DbbookID
	case config.MediaTypeAudiobook:
		notifyDbID = &m.DbaudiobookID
	case config.MediaTypeMusic:
		notifyDbID = &m.DbalbumID
	}

	if err := s.moveandcleanup(o, newpath, m, mediaID, notifyDbID, namingID, oldfiles); err != nil {
		return err
	}

	// The new file overwrote an old one at the same path: drop the old row so the
	// insert below doesn't leave two rows for one location. The file cache entry
	// stays, since the path is still a known file.
	if stashedFrom == "" && mediaID != nil && slices.Contains(oldfiles, newpath) {
		database.ExecNMap(s.Cfgp.IsType, logger.DBDeleteFileByIDLocation, mediaID, &newpath)
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
	if parser.ComparablePriority(m, s.Cfgp) >= cfgquality.CutoffPriority {
		reached = 1
	}

	fileext := filepath.Ext(newpath)
	filebase := filepath.Base(newpath)
	fileSize, bitrate, languages := m.FileDetails(newpath)
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
			&fileSize, &m.Runtime, &bitrate, &languages,
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
				&fileSize, &m.Runtime, &bitrate, &languages,
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
	id, dbid, namingID *uint,
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

	// Clone first: DeleteFunc zeroes the tail of the slice it filters, and the
	// caller's oldfiles must stay intact.
	replaced := slices.DeleteFunc(slices.Clone(oldfiles), func(r string) bool {
		return r == newfile
	})

	if s.targetpathCfg.Replacelower && len(replaced) >= 1 {
		err := s.moveremoveoldfiles(
			o,
			true,
			id,
			false,
			replaced,
		)
		if err != nil {
			return err
		}
	}

	if h == nil {
		return nil
	}

	return h.MoveOtherFilesAfterOrganize(&mediatype.MoveOtherFilesParams{
		Folder:                 o.Folder,
		Rootpath:               o.Rootpath,
		MediaFile:              o.MediaFile,
		TargetPath:             o.TargetPath,
		Filename:               o.Filename,
		PathCfgName:            s.sourcepathCfg.Name,
		AllowedOtherExtensions: s.sourcepathCfg.AllowedOtherExtensions,
		// Companions come from the download, not from the library folder.
		WalkCleanupFn: func(_, _, _ string) { s.moveCompanionFiles(o) },

		NotifyFn:        func() { s.notify(o, m, namingID, dbid, replaced) },
		CleanupFolderFn: func() { s.cleanUpFolder(o.Folder) },
	})
}

// walkcleanup removes every file with an allowed "other" extension under
// rootpath (skipping blocked paths), e.g. the leftovers of a rejected download.
func (s *Organizer) walkcleanup(rootpath string) error {
	if rootpath == "" {
		return nil
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

		if checkBlocked && logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, fpath) {
			return nil
		}

		scanner.RemoveFile(fpath)

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
			// Skip only when this "old" entry is literally the exact file we
			// just wrote as the replacement (defense in depth - the caller,
			// moveandcleanup, already excludes the real new-file path via
			// slices.DeleteFunc before calling here). A directory-prefix or
			// bare-basename check is wrong for libraries that organize files
			// in place (source and destination are the same folder, e.g. a
			// scraper-fed list with no separate download/library split):
			// every pre-existing file there would then always match and
			// never actually get removed, even though it is a genuinely
			// different, lower-priority file for the same episode.
			if oldfiles[idx] == filepath.Join(o.TargetPath, o.Filename) {
				continue
			}
		}

		// Missing files are handled too: moveRemoveOldMediaFile drops their
		// stale DB rows instead of leaving them behind forever.
		err = s.moveRemoveOldMediaFile(oldfiles[idx], &oldfiles[idx], id, move)
		if err != nil {
			// Continue the loop on a single old-file failure rather than
			// aborting the whole batch - callers reach this after the new
			// file has already been moved into place (moveandcleanup), so
			// returning here previously skipped the DB insert/cache update
			// for a file that was already physically in the library.
			logger.Logtype("error", 1).
				Str(logger.StrFile, oldfiles[idx]).
				Err(err).
				Msg("Move old")
		}
	}

	return nil
}

// notify sends notifications when new media is added.
// It populates notification data from the Organizerdata and ParseInfo,
// loops through the configured notifications, renders notification messages,
// and dispatches them based on the notification type.
func (s *Organizer) notify(
	o *Organizerdata,
	m *database.ParseInfo,
	namingID, id *uint,
	oldfiles []string,
) {
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

	// Populate the rich metadata structs (movie/series/book/audiobook/music) so
	// notification templates can reference fields like {{.Dbalbum.Label}} or
	// {{.Dbmovie.Genres}}, not just the scalar fields below.
	namingData := &mediatype.NamingData{}
	if _, ok := handler.FillNamingData(namingID, o.MediaFile, m, namingData); ok {
		notify.Dbmovie = namingData.Dbmovie
		notify.Dbserie = namingData.Dbserie
		notify.Serie = namingData.Serie
		notify.DbserieEpisode = namingData.DbserieEpisode
		notify.Dbaudiobook = namingData.Dbaudiobook
		notify.DbaudiobookChapter = namingData.DbaudiobookChapter
		notify.Dbbook = namingData.Dbbook
		notify.Dbalbum = namingData.Dbalbum
		notify.Dbtrack = namingData.Dbtrack
		notify.Author = namingData.Author
		notify.BookSeries = namingData.BookSeries
		notify.Artist = namingData.Artist
		notify.AlbumArtist = namingData.AlbumArtist
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

	event := providers.EventAddedData
	if len(notify.Replaced) > 0 {
		event = providers.EventUpgradedData
	}

	providers.NotifyEvent(s.Cfgp, event, &notify, func(n *config.MediaNotificationConfig) {
		notify.ReplacedPrefix = n.ReplacedPrefix
	})
}

// bookLowerQuality returns the best score of the book's library files and
// whether it is at least m's on the searcher's format scale. Library files of
// an unrecognised format never block, like unscorable movie files. Unlike
// movies, and like albums, the import file is kept.
func bookLowerQuality(m *database.ParseInfo, cfgp *config.MediaTypeConfig, cfgquality *config.QualityConfig) (int, bool) {
	old, _ := searcher.GetpriobyfilesAudio(config.MediaTypeBook, &m.BookID, true, -1, cfgquality, false)

	return old, old > parser.BookFormatPriority("") && old >= parser.ComparablePriority(m, cfgp)
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
			return errLowerQuality
		}

		return nil
	}

	// Multi-episode file: decide per episode. An episode is wanted when the
	// new file beats its existing files or it has none. The file is imported
	// for the wanted episodes only, and is deleted only when no episode wants
	// it - another episode already having a better file must not throw away
	// the file for the episodes that still need it.
	var oldfiles []string

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

		allowed := m.Priority > oldPrio || oldPrio == 0
		if allowed {
			oldfiles = append(oldfiles, getoldfiles...)
		} else {
			allowed = database.Getdatarow[uint](
				false,
				"select count() from serie_episode_files where serie_episode_id = ?",
				&m.Episodes[idx].Num1,
			) == 0
		}

		if allowed {
			newtbl = append(newtbl, m.Episodes[idx])
		}
	}

	if len(newtbl) == 0 {
		clear(m.Episodes)

		m.Episodes = m.Episodes[:0]

		if !skipdelete {
			removed, err := scanner.RemoveFile(o.MediaFile)
			if err != nil {
				logger.Logtype("error", 0).
					Err(err).
					Msg("delete Files")

				return err
			}

			if removed {
				logger.Logtype("info", 0).
					Str(logger.StrPath, o.MediaFile).
					Int(logger.StrPriority, m.Priority).
					Msg("Lower Qual Import File removed")
				s.removeotherfiles(o.MediaFile)
				s.cleanUpFolder(o.Folder)
			}
		}

		return errLowerQuality
	}

	o.Oldfiles = append(o.Oldfiles, oldfiles...)
	m.Episodes = newtbl

	return nil
}

// removeotherfiles removes any other allowed file extensions
// associated with the video file in orgadata. It loops through
// the configured allowed extensions and calls RemoveFile on the
// same filename with that extension.
func (s *Organizer) removeotherfiles(videofile string) {
	for _, f := range companionsOf(videofile, s.sourcepathCfg.AllowedOtherExtensions) {
		scanner.RemoveFile(f)
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
			s.cleanUpAlbumFolder(folder)
		}

		if errors.Is(err, errUnprocessed) {
			return nil
		}

		if err == nil {
			s.cleanUpAlbumFolder(folder)
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

		switch mediaFileSkipReason(cfgp, s.sourcepathCfg, fpath) {
		case "":
		case skipUnpacking:
			logger.Logtype("warn", 1).
				Str(logger.StrFile, fpath).
				Msg("skipped - unpacking")

			return nil
		case skipDisallowed:
			logger.Logtype("warn", 1).
				Str(logger.StrFile, fpath).
				Msg("skipped - disallowed")

			return nil
		default:
			return nil
		}

		if isUnmatchedFile(cfgp, fpath) {
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
		}

		// A single file's failure must not stop the rest of the walk; it
		// has already been logged and recorded as unmatched.
		return nil
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

	// The handler is constant for this media type; fetch it once and reuse.
	handler := mediatype.Get(s.Cfgp.IsType)

	var err error
	if s.manualID != 0 {
		_, err = forceMediaIDs(m, cfgp, s.manualID)
	} else {
		err = parser.GetDBIDs(m, cfgp, true, addFound)
	}

	if err != nil || !s.hasValidIDs(m) {
		logger.Logtype("warn", 1).
			Str(logger.StrFile, fpath).
			Err(err).
			Msg(logger.ParseFailedIDs)

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, err)

		if err == nil {
			err = errNoValidIDs
		}

		return false, "no_match", err
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
					m.AddUnmatched(cfgp, &logger.StrStructure, errSmallFile)
					return false, "small_file", nil
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
				m.AddUnmatched(cfgp, &logger.StrStructure, errSmallFile)

				// Only trigger move if file was not deleted
				if shouldRemove {
					return false, "", nil
				}

				return false, "small_file", nil
			}

			if logger.TimeAfter(info.ModTime(), logger.TimeGetNow().Add(-2*time.Minute)) {
				logger.Logtype("error", 1).
					Str(logger.StrFile, fpath).
					Msg("file modified too recently")
				return false, "", errTooRecentlyModified
			}
		}
	}

	if handler != nil {
		handler.SetTempID(m)
	}

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(s.Cfgp, &m.TempID)

		if m.ListID == -1 {
			logger.Logtype("warn", 1).
				Str(logger.StrFile, fpath).
				Msg("listcfg not found")

			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, errListcfgNotFound)

			return false, "no_list", nil
		}
	}

	if config.GetSettingsGeneral().UseFileCache {
		database.SlicesCacheContainsDelete(
			mtstrings.GetStringsMap(s.Cfgp.IsType, logger.CacheUnmatched),
			fpath,
		)
	}

	if handler != nil {
		if !handler.ValidateIDs(m) {
			m.TempTitle = fpath
			m.AddUnmatched(cfgp, &logger.StrStructure, errNoValidIDs)
			return false, "no_match", nil
		}
	}

	if s.checksubfiles(folder, fpath, "") {
		logger.Logtype("error", 1).
			Str(logger.StrFile, fpath).
			Msg("check sub files")

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errCheckSubFiles)

		return false, "subfiles_check", nil
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
		m.AddUnmatched(cfgp, &logger.StrStructure, errNoDbidFound)
		return false, "no_match", nil
	}

	if handler != nil {
		handler.SetDBID(m, dbid)
	}

	if m.ListID == -1 {
		m.ListID = s.Cfgp.GetMediaListsEntryListID(listname)
	}

	if m.ListID == -1 {
		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errNoListIDFound)
		return false, "no_list", nil
	}

	// check_title guards automatic matches; a manually chosen entry may carry
	// a different (e.g. translated) title.
	if s.manualID == 0 &&
		m.Checktitle(s.Cfgp, s.Cfgp.Lists[m.ListID].CfgQuality, filepath.Base(fpath)) {
		logger.Logtype("warn", 1).
			Str(logger.StrFile, fpath).
			Msg("skipped - unwanted title")

		m.TempTitle = fpath
		m.AddUnmatched(cfgp, &logger.StrStructure, errUnwantedTitle)

		return false, "unwanted_title", nil
	}

	o := Organizerdata{Folder: folder, MediaFile: fpath, Listid: m.ListID, Rootpath: rootpath}

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

		defer func() { recordOrganized(cfgp.IsType, mediaEntryIDs(cfgp.IsType, m), folder, fpath, moveReason, err) }()

		switch {
		case errors.Is(err, errWrongRuntime):
			moveReason = "wrong_runtime"
		case errors.Is(err, errWrongLanguage):
			moveReason = "wrong_language"
		case errors.Is(err, errGeneratingFilename):
			moveReason = "naming_failed"
		case errors.Is(err, errLowerQuality):
			moveReason = "lower_quality"
		default:
			moveReason = "organize_failed"
		}

		return false, moveReason, err
	}

	if handler != nil {
		handler.ClearUnmatchedCache(fpath)
	}

	recordOrganized(cfgp.IsType, mediaEntryIDs(cfgp.IsType, m), folder, fpath, "", nil)

	s.cleanUpFolder(folder)

	return true, "", nil
}

// mediaEntryIDs returns the list entries an identified file belongs to: the
// movie, book, audiobook or album entry, or the series episodes. A lower
// quality refusal clears m.Episodes, so they are looked up again.
func mediaEntryIDs(isType uint, m *database.ParseInfo) []uint {
	switch isType {
	case config.MediaTypeMovie:
		return []uint{m.MovieID}
	case config.MediaTypeSeries:
		if len(m.Episodes) == 0 && m.SerieID != 0 {
			_ = m.Getepisodestoimport()
		}

		ids := make([]uint, 0, len(m.Episodes))
		for i := range m.Episodes {
			ids = append(ids, m.Episodes[i].Num1)
		}

		return ids
	case config.MediaTypeBook:
		return []uint{m.BookID}
	case config.MediaTypeAudiobook:
		return []uint{m.AudiobookID}
	case config.MediaTypeMusic:
		return []uint{m.AlbumID}
	}

	return nil
}

// recordOrganized hands the structure job's outcome for downloaded files to
// the download state tracking: organized when err is nil, else rejected for
// reason (a move_unprocessed reason; organize_failed when empty).
func recordOrganized(isType uint, ids []uint, folder, file, reason string, err error) {
	if err == nil {
		downloadstate.Organized(isType, ids, folder, file, "", "")
		return
	}

	if reason == "" {
		reason = "organize_failed"
	}

	downloadstate.Organized(isType, ids, folder, file, reason, err.Error())
}

// isForeignUnpackPath reports whether a path component starts with "_unpack"
// (case-insensitive) - the in-progress unpack folder of downloaders like
// SABnzbd ("_UNPACK_<name>"). Our own extraction folders are named
// "<archive>_unpack" and do not match.
func isForeignUnpackPath(fpath string) bool {
	for _, part := range strings.FieldsFunc(fpath, func(r rune) bool {
		return r == '/' || r == '\\'
	}) {
		if logger.HasPrefixI(part, "_unpack") {
			return true
		}
	}

	return false
}

// checksubfiles checks for any disallowed subtitle files in the same
// folder as the video file. It also checks if there are multiple files
// with the same extension, which indicates it may not be a standalone movie.
// It returns an error if disallowed files are found or too many matching files exist.
func (s *Organizer) checksubfiles(folder, videofile, rootpath string) bool {
	disallowed, tooMany := s.subfileProblem(folder, videofile)

	if disallowed {
		if s.sourcepathCfg.DeleteDisallowed && videofile != s.sourcepathCfg.Path {
			s.fileCleanup(folder, videofile, rootpath)
		}

		logger.Logtype("warn", 1).
			Str(logger.StrFile, videofile).
			Msg("skipped - disallowed")

		return true
	}

	if tooMany {
		logger.Logtype("warn", 1).
			Str(logger.StrFile, videofile).
			Msg("skipped - too many files")
		return true
	}

	return false
}

// subfileProblem scans folder for what makes checksubfiles skip videofile:
// a disallowed path, or a second file of its extension for media types that
// take one file per folder.
func (s *Organizer) subfileProblem(folder, videofile string) (disallowed, tooMany bool) {
	if folder == "" {
		return false, false
	}

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
		return true, false
	}

	handler := mediatype.Get(s.Cfgp.IsType)

	return false, handler != nil && handler.SkipMultipleFiles() && count >= 2
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

// computeRootpath searches through the given config to find which configured
// path the given file is under, and returns the folder immediately below
// that path (joined back onto it) as the new rootpath value. For music and
// audiobooks the rootpath is the album folder itself (see audioRootpath), for
// books the folder holding the book (see bookRootpath).
// ok is false if no configured path matches the file.
func computeRootpath(file string, cfgp *config.MediaTypeConfig) (rootpath string, ok bool) {
	// The longest matching path wins: DataMap is a map, so with nested paths
	// ("/data" and "/data/EN_Series") the first hit would be random.
	best := ""

	for _, data := range cfgp.DataMap {
		if data.CfgPath == nil {
			continue
		}

		p := strings.TrimRight(data.CfgPath.Path, "/\\")
		if p == "" || len(p) <= len(best) || !logger.HasPrefixI(file, p) {
			continue
		}

		// Whole path components only: "/data/EN_Series" must not claim
		// "/data/EN_Series_old/...".
		if len(file) > len(p) && file[len(p)] != '/' && file[len(p)] != '\\' {
			continue
		}

		best = p
	}

	if best != "" && (cfgp.IsType == config.MediaTypeMusic || cfgp.IsType == config.MediaTypeAudiobook) {
		return audioRootpath(file, best)
	}

	if best != "" && cfgp.IsType == config.MediaTypeBook {
		return bookRootpath(file, best)
	}

	if best != "" {
		firstfolder := logger.TrimLeft(file[len(best):], '/', '\\')
		// Use IndexAny for single-pass check instead of two ContainsRune calls
		if strings.ContainsAny(firstfolder, "/\\") {
			firstfolder = filepath.Dir(firstfolder)
		}

		return filepath.Join(best, getrootpath(firstfolder)), true
	}

	return "", false
}

// audioRootpath returns the album/audiobook folder for path (a file or the
// album folder itself) below the library root libroot: the folder holding the
// file, or the folder above a disc subfolder ("Album/CD1"). That is the value
// setTargetPath uses as the complete target folder and the album organizer and
// library scan store. ok is false when the folder would be the library root.
func audioRootpath(path, libroot string) (string, bool) {
	dir := filepath.Clean(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	if parent := filepath.Dir(dir); !strings.EqualFold(dir, libroot) &&
		!strings.EqualFold(parent, libroot) &&
		importfeed.IsDiscSubfolder(filepath.Base(dir), filepath.Base(parent)) {
		dir = parent
	}

	if strings.EqualFold(dir, libroot) || len(dir) < len(libroot) {
		return "", false
	}

	return dir, true
}

// bookRootpath returns the folder holding the book file path (or path itself
// when it is a folder, as moveandcleanup passes the target folder). Naming
// clears the folder template once a book has a rootpath and setTargetPath then
// moves into Rootpath as-is, so the rootpath must be the complete book folder,
// not the first folder below the library root ("Author" for
// "Author/Title/book.epub"). ok is false when the folder would be the library
// root.
func bookRootpath(path, libroot string) (string, bool) {
	dir := filepath.Clean(path)
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		dir = filepath.Dir(dir)
	}

	if strings.EqualFold(dir, libroot) || len(dir) < len(libroot) {
		return "", false
	}

	return dir, true
}

// UpdateRootpath updates the rootpath column in the database for the given object type and ID.
// It searches through the provided config to find which path the given file is under.
// It then extracts the first folder from the relative path of the file to that config path.
// It joins that folder with the config path to form the new rootpath value.
// Finally it executes a SQL update statement to update the rootpath for that object ID.
func UpdateRootpath(file, objtype string, objid *uint, cfgp *config.MediaTypeConfig) {
	rootpath, ok := computeRootpath(file, cfgp)
	if !ok {
		return
	}

	database.ExecN(
		logger.JoinStrings("update ", objtype, " set rootpath = ? where id = ?"),
		&rootpath,
		objid,
	)
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

// OrganizePreview describes where a single media file would be organized to,
// computed without moving any file or modifying the database.
type OrganizePreview struct {
	SourceFile   string `json:"source_file"`
	TargetFolder string `json:"target_folder"`
	TargetFile   string `json:"target_file"`
	Title        string `json:"title"`
	Year         int    `json:"year"`
	Season       string `json:"season,omitempty"`
	Episode      string `json:"episode,omitempty"`
}

// PreviewOrganizeFile parses a single-file media item (movie, series episode, or
// book) and computes the folder and filename it would be organized into, without
// touching the filesystem or writing to the database. It is the single-file
// counterpart to ForceMatchAlbumFolder's preview mode and mirrors the read-only
// portion of walkorganizefolder/Organize (no video probe, no deletions).
func PreviewOrganizeFile(
	fpath string,
	cfgp *config.MediaTypeConfig,
	sourceTemplate, targetTemplate string,
	manualID uint,
) (*OrganizePreview, error) {
	s := NewStructure(cfgp, sourceTemplate, targetTemplate, false, false, manualID)
	if s == nil {
		return nil, logger.ErrNotFound
	}

	defer s.Close()

	m := parser_v2.ParseFile(fpath, true, true, cfgp, -1)
	if m == nil {
		return nil, logger.ErrNotFound
	}

	defer m.Close()

	if s.manualID != 0 {
		if _, err := forceMediaIDs(m, cfgp, s.manualID); err != nil {
			return nil, err
		}
	} else if err := parser.GetDBIDs(m, cfgp, true, false); err != nil {
		return nil, err
	}

	handler := mediatype.Get(cfgp.IsType)
	if handler == nil || !handler.ValidateIDs(m) {
		return nil, errNoValidIDs
	}

	handler.SetTempID(m)

	if m.ListID == -1 {
		m.ListID = database.GetMediaListIDGetListname(cfgp, &m.TempID)
	}

	if m.ListID < 0 || m.ListID >= len(cfgp.Lists) {
		return nil, errNoListIDFound
	}

	var (
		dbid               uint
		listname, rootpath string
	)

	database.GetdatarowArgs(
		mtstrings.GetStringsMap(cfgp.IsType, "GetOrganizeData"),
		&m.TempID,
		&dbid,
		&rootpath,
		&listname,
	)

	if dbid == 0 {
		return nil, errNoDbidFound
	}

	handler.SetDBID(m, dbid)

	cfgQuality := cfgp.Lists[m.ListID].CfgQuality
	parser.GetPriorityMapQual(m, cfgp, cfgQuality, true, true)

	o := Organizerdata{
		Folder:    filepath.Dir(fpath),
		MediaFile: fpath,
		Listid:    m.ListID,
		Rootpath:  rootpath,
	}

	// Resolve the naming ID the same way Organize does for each media type.
	var namingID *uint
	switch cfgp.IsType {
	case config.MediaTypeSeries:
		if err := m.Getepisodestoimport(); err != nil {
			return nil, err
		}

		if len(m.Episodes) == 0 {
			return nil, logger.ErrNotFoundEpisode
		}

		namingID = &m.Episodes[0].Num1

	case config.MediaTypeMovie:
		namingID = &m.DbmovieID

	case config.MediaTypeBook:
		namingID = &m.DbbookID

	default:
		namingID = &dbid
	}

	s.GenerateNamingTemplate(&o, m, namingID)

	if o.Filename == "" {
		return nil, errGeneratingFilename
	}

	// Same target computation as Organize/moveMediaFile.
	s.applyPresort(&o)
	s.setTargetPath(&o)

	return &OrganizePreview{
		SourceFile:   fpath,
		TargetFolder: o.TargetPath,
		TargetFile:   o.Filename,
		Title:        m.Title,
		Year:         int(m.Year),
		Season:       m.SeasonStr,
		Episode:      m.EpisodeStr,
	}, nil
}

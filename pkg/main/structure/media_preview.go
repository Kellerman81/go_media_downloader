package structure

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
)

// Why mediaFileSkipReason leaves a file out of the structure walk.
const (
	skipUnpacking  = "unpacking"
	skipDisallowed = "disallowed"
	skipOther      = "other"
)

var (
	errNotOrganized     = errors.New("not organized")
	errPlanNeedsListing = errors.New("the episode's list entry is needed for its file name - it is computed after import")
)

// forceMediaIDs points m at the list entry mediaID of cfgp (movies.id,
// series.id, books.id) instead of what the parser identified: the database
// row, the list and, for an episode file, the episode ids all come from that
// entry. It returns the entry's rootpath.
func forceMediaIDs(m *database.ParseInfo, cfgp *config.MediaTypeConfig, mediaID uint) (string, error) {
	handler := mediatype.Get(cfgp.IsType)
	if handler == nil {
		return "", errNoValidIDs
	}

	var (
		dbid               uint
		rootpath, listname string
	)

	database.GetdatarowArgs(
		mtstrings.GetStringsMap(cfgp.IsType, "GetOrganizeData"),
		&mediaID,
		&dbid,
		&rootpath,
		&listname,
	)

	if dbid == 0 {
		return "", errNoDbidFound
	}

	m.ListID = cfgp.GetMediaListsEntryListID(listname)
	if m.ListID == -1 {
		return "", errNoListIDFound
	}

	handler.SetMediaID(m, mediaID)
	handler.SetDBID(m, dbid)

	if cfgp.IsType != config.MediaTypeSeries {
		return rootpath, nil
	}

	// Date-identified series resolve through the single-episode ids;
	// Getepisodestoimport looks up every episode of a multi-episode file.
	m.DbserieEpisodeID, m.SerieEpisodeID = 0, 0
	m.SetDBEpisodeIDfromM()
	m.SetEpisodeIDfromM()

	if err := m.Getepisodestoimport(); err != nil {
		return rootpath, err
	}

	if len(m.Episodes) == 0 {
		return rootpath, logger.ErrNotFoundEpisode
	}

	m.SerieEpisodeID, m.DbserieEpisodeID = m.Episodes[0].Num1, m.Episodes[0].Num2

	return rootpath, nil
}

// mediaFileSkipReason reports why the structure walk leaves fpath alone, or
// "" when it organizes it.
func mediaFileSkipReason(cfgp *config.MediaTypeConfig, pathCfg *config.PathsConfig, fpath string) string {
	ext := filepath.Ext(fpath)
	if ext == "" {
		return skipOther
	}

	// Our own "<archive>_unpack" folders were fully extracted by
	// unpackArchivesInFolder (failed extractions are removed), so their media
	// is organized like any other file. Only folders another tool is still
	// unpacking into (SABnzbd "_UNPACK_*") are skipped.
	if isForeignUnpackPath(fpath) {
		return skipUnpacking
	}

	if logger.SlicesContainsPart2I(pathCfg.Disallowed, fpath) {
		return skipDisallowed
	}

	if ok, _ := scanner.CheckExtensionsType(cfgp.IsType, false, pathCfg, ext); !ok {
		return skipOther
	}

	if pathCfg.BlockedLen >= 1 && logger.SlicesContainsPart2I(pathCfg.Blocked, fpath) {
		return skipOther
	}

	return ""
}

// isUnmatchedFile reports whether fpath is on the unmatched list; the
// structure walk skips those files.
func isUnmatchedFile(cfgp *config.MediaTypeConfig, fpath string) bool {
	if config.GetSettingsGeneral().UseFileCache {
		return database.SlicesCacheContains(cfgp.IsType, logger.CacheUnmatched, fpath)
	}

	return database.Getdatarow[uint](
		false,
		mtstrings.GetStringsMap(cfgp.IsType, logger.DBCountUnmatchedPath),
		fpath,
	) >= 1
}

// CollectMediaFiles returns the movie, episode or book files below folder
// that the structure walk would organize, unmatched-listed files included.
func CollectMediaFiles(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	pathCfg *config.PathsConfig,
) []string {
	var out []string

	filepath.WalkDir(folder, func(fpath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if ctx.Err() != nil {
			return filepath.SkipAll
		}

		if !d.IsDir() && mediaFileSkipReason(cfgp, pathCfg, fpath) == "" {
			out = append(out, fpath)
		}

		return nil
	})

	return out
}

// MediaFilePreview is the match preview of one movie, episode or book file.
type MediaFilePreview struct {
	File       string
	Folder     string
	Query      importfeed.MediaQuery
	Release    string
	MatcherErr string
	Unmatched  bool
	Set        *importfeed.MediaCandidateSet
	Err        string
}

// PreviewMediaFile parses fpath, runs the structure job's own lookup on it
// and collects the scored candidates, all without writing anything. cache
// shares provider searches between the files of a folder (may be nil).
func PreviewMediaFile(
	ctx context.Context,
	fpath, folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	cache *importfeed.MediaSearchCache,
) *MediaFilePreview {
	p := &MediaFilePreview{File: fpath, Folder: folder, Unmatched: isUnmatchedFile(cfgp, fpath)}

	m := parser_v2.ParseFile(fpath, true, true, cfgp, -1)
	if m == nil {
		p.Err = "parse failed"
		return p
	}

	defer m.Close()

	p.Release = strings.Join(strings.Fields(strings.Join(
		[]string{m.Resolution, m.Quality, m.Codec, m.Audio}, " ")), " ")

	// Same title cleanup as the movie and book lookups.
	if cfgp.IsType != config.MediaTypeSeries {
		for _, lst := range cfgp.ListsMap {
			if lst.TemplateQuality != "" {
				m.StripTitlePrefixPostfixGetQual(lst.CfgQuality)
			}
		}
	}

	p.Query = importfeed.MediaQuery{
		IsType:     cfgp.IsType,
		Title:      logger.TrimSpace(m.Title),
		Year:       int(m.Year),
		Imdb:       m.Imdb,
		Tvdb:       logger.StringToInt(m.Tvdb),
		ISBN:       m.ISBN,
		Author:     m.Artist,
		Identifier: m.Identifier,
	}

	switch cfgp.IsType {
	case config.MediaTypeSeries:
		p.Query.Season, p.Query.Episodes = m.EpisodeNumbers()
	case config.MediaTypeBook:
		if p.Query.Author == "" {
			if author, title, ok := strings.Cut(p.Query.Title, " - "); ok {
				p.Query.Author, p.Query.Title = logger.TrimSpace(author), logger.TrimSpace(title)
			}
		}
	}

	// GetDBIDs rewrites the parsed fields, so it gets its own parse.
	var matcherDBID uint
	if mm := parser_v2.ParseFile(fpath, true, true, cfgp, -1); mm != nil {
		err := parser.GetDBIDs(mm, cfgp, true, data != nil && data.AddFound)
		if handler := mediatype.Get(cfgp.IsType); handler != nil {
			matcherDBID = handler.GetDBID(mm)
		}

		if err != nil {
			p.MatcherErr = err.Error()
		}

		mm.Close()
	}

	p.Set = importfeed.FindMediaCandidates(ctx, &p.Query, cfgp, matcherDBID, cache)
	if len(p.Set.Candidates) == 0 {
		p.Err = "no_match"
	}

	return p
}

// PlanIssue is a reason the structure job would not organize the file as
// planned. Blocks: the file is not organized; Deletes: the import file is
// deleted on the way. Issues are in the order the structure job checks them,
// so the first blocking one decides; later ones are not reached.
type PlanIssue struct {
	Text    string
	Blocks  bool
	Deletes bool
}

// MediaFilePlan is what organizing a file with a chosen entry would do.
type MediaFilePlan struct {
	TargetFolder  string
	TargetFile    string
	Release       string
	Languages     []string
	Priority      int
	OldPriority   int
	OldFiles      []string
	OldFilesFate  string
	RuntimeFound  int
	RuntimeWanted int
	Issues        []PlanIssue
}

func (p *MediaFilePlan) issue(blocks, deletes bool, format string, args ...any) {
	p.Issues = append(p.Issues, PlanIssue{Text: fmt.Sprintf(format, args...), Blocks: blocks, Deletes: deletes})
}

// PlanMediaFile computes, read-only, what organizing fpath with list entry
// mediaID would do: the checks of the structure walk and of Organize, the
// quality comparison with the library's files and the target path. A
// movie or book that is in the database but in none of cfgp's lists can be
// planned by its database row dbID (listid is the list it will go to); an
// episode needs its list entry.
func PlanMediaFile(
	ctx context.Context,
	fpath, folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	targetTemplate string,
	mediaID, dbID uint,
	listid int,
) (*MediaFilePlan, error) {
	s := NewStructure(cfgp, data.TemplatePath, targetTemplate,
		data.CfgPath.CheckRuntime, data.CfgPath.DeleteWrongLanguage, mediaID)
	if s == nil {
		return nil, logger.ErrNotFound
	}

	defer s.Close()

	m := parser_v2.ParseFile(fpath, true, true, cfgp, -1)
	if m == nil {
		return nil, logger.ErrNotFound
	}

	defer m.Close()

	var (
		rootpath string
		err      error
	)

	switch {
	case mediaID != 0:
		rootpath, err = forceMediaIDs(m, cfgp, mediaID)
		if err != nil {
			return nil, err
		}

	case dbID != 0 && cfgp.IsType != config.MediaTypeSeries && listid >= 0 && listid < len(cfgp.Lists):
		mediatype.Get(cfgp.IsType).SetDBID(m, dbID)
		m.ListID = listid

	default:
		return nil, errPlanNeedsListing
	}

	plan := &MediaFilePlan{}
	cfgQuality := cfgp.Lists[m.ListID].CfgQuality

	s.planWalkChecks(plan, fpath, folder)

	var (
		namingID *uint
		runtime  uint
	)

	switch cfgp.IsType {
	case config.MediaTypeMovie:
		namingID = &m.DbmovieID

		database.Scanrowsdyn(false, mtstrings.GetStringsMap(cfgp.IsType, "SelectRuntime"), &m.RuntimeStr, &m.DbmovieID)

	case config.MediaTypeSeries:
		if !s.planSeriesEpisodes(plan, m, cfgQuality) {
			return plan, nil
		}

		namingID = &m.Episodes[0].Num1

	case config.MediaTypeBook:
		namingID = &m.DbbookID
		plan.OldFiles = database.GetrowsN[string](false, 0,
			"select location from book_files where book_id = ?", &m.BookID)
	}

	if m.RuntimeStr != "" && m.RuntimeStr != "0" {
		if v, err := strconv.Atoi(m.RuntimeStr); err == nil && v > 0 {
			runtime = uint(v)
		}
	}

	if cfgp.IsType == config.MediaTypeSeries && len(m.Episodes) > 0 &&
		(database.Getdatarow[string](false, database.QueryDbseriesGetIdentifiedByID, &m.DbserieID) == logger.StrDate ||
			database.Getdatarow[bool](false, mtstrings.GetStringsMap(cfgp.IsType, "SelectIgnoreRuntime"), &m.Episodes[0].Num1)) {
		runtime = 0
	}

	parser.GetPriorityMapQual(m, cfgp, cfgQuality, true, true)

	if mediatype.SupportsVideoFile(cfgp.IsType) {
		s.planVideoChecks(ctx, plan, m, cfgQuality, runtime, fpath)
	}

	plan.Priority = m.Priority
	plan.Release = strings.Join(strings.Fields(strings.Join(
		[]string{m.Resolution, m.Quality, m.Codec, m.Audio}, " ")), " ")
	plan.Languages = slices.Clone(m.Languages)

	if mediatype.ShouldCheckOldFilePriority(cfgp.IsType) {
		mid := mediatype.Get(cfgp.IsType).GetMediaID(m)
		plan.OldPriority, plan.OldFiles = searcher.Getpriobyfiles(cfgp.IsType, &mid, true, m.Priority, cfgQuality, true)

		if plan.OldPriority != 0 && plan.OldPriority >= m.Priority {
			plan.issue(true, true,
				"the library already has a file of priority %d (new file: %d)",
				plan.OldPriority, m.Priority)
		}
	}

	if cfgp.IsType == config.MediaTypeBook {
		var lower bool
		if plan.OldPriority, lower = bookLowerQuality(m, cfgp, cfgQuality); lower {
			plan.issue(true, false,
				"the library already has a file of priority %d (new file: %d, format only)",
				plan.OldPriority, parser.ComparablePriority(m, cfgp))
		}
	}

	o := Organizerdata{Folder: folder, MediaFile: fpath, Listid: m.ListID, Rootpath: rootpath}

	s.GenerateNamingTemplate(&o, m, namingID)

	if o.Filename == "" {
		plan.issue(true, false, "no file name could be generated from the naming template")
		return plan, nil
	}

	s.applyPresort(&o)
	s.setTargetPath(&o)

	planned := scanner.PlannedMovePath(fpath, s.sourcepathCfg, o.TargetPath, o.Filename,
		scanner.MoveFileOptions{MediaType: cfgp.IsType})
	plan.TargetFolder, plan.TargetFile = filepath.Dir(planned), filepath.Base(planned)

	if len(plan.OldFiles) > 0 {
		switch {
		case s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != "":
			plan.OldFilesFate = "moved to " + s.targetpathCfg.MoveReplacedTargetPath
		case s.targetpathCfg.Replacelower:
			plan.OldFilesFate = "deleted"
		default:
			plan.OldFilesFate = "kept (replace_lower is off)"
		}
	}

	return plan, nil
}

// planWalkChecks adds the per-file checks of walkorganizefolder.
func (s *Organizer) planWalkChecks(plan *MediaFilePlan, fpath, folder string) {
	if s.sourcepathCfg.MinVideoSize > 0 {
		if info, err := os.Stat(fpath); err == nil && info.Size() < s.sourcepathCfg.MinVideoSizeByte {
			deletes := false
			if s.sourcepathCfg.Name != "" {
				ok, oknorename := scanner.CheckExtensionsType(s.Cfgp.IsType, false, s.sourcepathCfg, filepath.Ext(fpath))
				deletes = ok || oknorename || !mediatype.HasConfiguredExtensions(s.Cfgp.IsType, s.sourcepathCfg)
			}

			plan.issue(true, deletes, "smaller than min_video_size (%d MB)", s.sourcepathCfg.MinVideoSize)
		}
	}

	disallowed, tooMany := s.subfileProblem(folder, fpath)
	if disallowed {
		plan.issue(true, s.sourcepathCfg.DeleteDisallowed, "the folder contains a disallowed file")
	}

	if tooMany {
		plan.issue(true, false, "the folder holds more than one file of this type")
	}
}

// planSeriesEpisodes mirrors the episode part of Organize: the episodes of
// the file, their runtime and season, and which of them still want the
// file. It reports false when that already stops the organize.
func (s *Organizer) planSeriesEpisodes(plan *MediaFilePlan, m *database.ParseInfo, cfgQuality *config.QualityConfig) bool {
	if err := m.Getepisodestoimport(); err != nil || len(m.Episodes) == 0 {
		plan.issue(true, false, "the file's episodes are not in the series' list entry")
		return false
	}

	database.GetdatarowArgs(
		mtstrings.GetStringsMap(s.Cfgp.IsType, "SelectEpisodeRuntime"),
		&m.Episodes[0].Num2,
		&m.RuntimeStr,
		&m.SeasonStr,
	)

	identifiedby := database.Getdatarow[string](false, database.QueryDbseriesGetIdentifiedByID, &m.DbserieID)

	if (m.RuntimeStr == "" || m.RuntimeStr == "0") && identifiedby != logger.StrDate {
		database.Scanrowsdyn(false, mtstrings.GetStringsMap(s.Cfgp.IsType, "SelectRuntime"), &m.RuntimeStr, &m.DbserieID)

		if (m.RuntimeStr == "" || m.RuntimeStr == "0") && s.checkruntime {
			plan.issue(true, false, "neither the episode nor the series has a runtime (check_runtime is on)")
		}
	}

	if m.SeasonStr == "" && identifiedby != logger.StrDate {
		plan.issue(true, false, "the episode has no season")
	}

	// GetSeriesEpisodes decides on the filename's quality, before the probe.
	parser.GetPriorityMapQual(m, s.Cfgp, cfgQuality, true, true)

	wanted := 0
	for i := range m.Episodes {
		oldPrio, oldfiles := searcher.Getpriobyfiles(config.MediaTypeSeries, &m.Episodes[i].Num1, true, m.Priority, cfgQuality, true)

		if m.Priority > oldPrio || oldPrio == 0 {
			wanted++

			plan.OldFiles = append(plan.OldFiles, oldfiles...)
		} else if database.Getdatarow[uint](false,
			"select count() from serie_episode_files where serie_episode_id = ?", &m.Episodes[i].Num1) == 0 {
			wanted++
		}

		plan.OldPriority = max(plan.OldPriority, oldPrio)
	}

	if wanted == 0 {
		plan.issue(true, true,
			"the library already has files of priority %d or better (new file: %d)",
			plan.OldPriority, m.Priority)
	} else if wanted < len(m.Episodes) {
		plan.issue(false, false, "only %d of %d episodes take the file; the others have better files", wanted, len(m.Episodes))
	}

	return true
}

// planVideoChecks probes the file and adds the runtime and language checks
// of ParseFileAdditional.
func (s *Organizer) planVideoChecks(
	ctx context.Context,
	plan *MediaFilePlan,
	m *database.ParseInfo,
	cfgQuality *config.QualityConfig,
	runtime uint,
	fpath string,
) {
	m.File = fpath

	if err := parser.ParseVideoFile(ctx, m, cfgQuality); err != nil {
		plan.issue(true, errors.Is(err, logger.ErrTracksEmpty), "probing the file failed: %v", err)
		return
	}

	wanted, found, wrong := s.runtimeMismatch(m, runtime, s.checkruntime)
	plan.RuntimeWanted, plan.RuntimeFound = wanted, found

	if plan.RuntimeWanted == 0 && runtime > 0 {
		plan.RuntimeWanted = int(runtime) * mediatype.GetRuntimeMultiplier(s.Cfgp.IsType, m)
		plan.RuntimeFound = m.Runtime / 60
	}

	if wrong {
		plan.issue(true, s.targetpathCfg.MaxRuntimeDifference != 0 && s.targetpathCfg.DeleteWrongRuntime,
			"runtime %d min, wanted %d min (max_runtime_difference %d)", found, wanted, s.targetpathCfg.MaxRuntimeDifference)
	}

	if s.deletewronglanguage && !s.languageAllowed(m) {
		plan.issue(true, true, "audio language %v is not in allowed_languages", m.Languages)
	}
}

// OrganizeMediaFile organizes one movie, episode or book file with the list
// entry mediaID, as the structure job would after identifying it as that
// entry. folder is the import folder the file belongs to.
func OrganizeMediaFile(
	ctx context.Context,
	fpath, folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	targetTemplate string,
	mediaID uint,
) error {
	s := NewStructure(cfgp, data.TemplatePath, targetTemplate,
		data.CfgPath.CheckRuntime, data.CfgPath.DeleteWrongLanguage, mediaID)
	if s == nil {
		return logger.ErrNotFound
	}

	defer s.Close()

	organized, reason, err := s.walkorganizefolder(ctx, fpath, folder, cfgp, data.AddFound)
	if organized {
		return nil
	}

	if reason == "" {
		reason = "file removed"
	}

	if errors.Is(err, errLowerQuality) {
		text := skipReasonText["lower_quality"]
		if !scanner.CheckFileExist(fpath) {
			text += "; the import file was deleted"
		}

		return fmt.Errorf("%w: %s", errNotOrganized, text)
	}

	if err != nil {
		return fmt.Errorf("%w (%s): %w", errNotOrganized, reason, err)
	}

	return fmt.Errorf("%w: %s", errNotOrganized, reason)
}

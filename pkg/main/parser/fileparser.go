package parser

import (
	"context"
	"errors"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/movies" // Register movie handler
	_ "github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/series" // Register series handler
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

type regexpattern struct {
	name string
	// REs need to have 2 sub expressions (groups), the first one is "raw", and
	// the second one for the "clean" value.
	// E.g. Epiode matching on "S01E18" will result in: raw = "E18", clean = "18".
	re       string
	getgroup int
	// Use the last matching pattern. E.g. Year.
	last bool
}

type Prioarr struct {
	QualityGroup  string
	Priority      int
	ResolutionID  uint
	QualityID     uint
	CodecID       uint
	AudioID       uint
	AudioFormatID uint
}

// priokey identifies one quality-priority combination, used only by the
// test-override path. group is the lowercased quality profile name.
type priokey struct {
	group                      string
	reso, qual, codec, aud, af uint
}

// resoQualEntry holds the (possibly reordered) resolution+quality priority and
// the wanted flag for one (resolution, quality) pair within a profile.
type resoQualEntry struct {
	prio   int
	wanted bool
}

// dimVal is an (ID, priority) pair for a single-dimension component (codec,
// audio, audio-format), kept in generation order so the full table can be
// materialized on demand for the admin API.
type dimVal struct {
	id   uint
	prio int
}

// profilePrio holds the compact priority tables for one quality profile. The
// final priority of any combination is additive — resoQual + codec + audio +
// audioFormat — so storing the per-dimension priorities (a few hundred entries)
// replaces the full Cartesian product (millions of entries, ~1 GB).
type profilePrio struct {
	name string // original-case quality group name (for materialized Prioarr)

	resoQual map[[2]uint]resoQualEntry // (resolutionID, qualityID) -> prio + wanted
	codec    map[uint]int
	audio    map[uint]int
	audioFmt map[uint]int

	// Generation order, retained only to materialize the full table on demand.
	resoQualOrder [][2]uint
	codecOrder    []dimVal
	audioOrder    []dimVal
	afOrder       []dimVal
}

// overrideTables holds explicit priority slices installed by setPrioritiesForTest.
type overrideTables struct {
	all, wanted       []Prioarr
	allIdx, wantedIdx map[priokey]int
}

// prioritySnapshot is an immutable view of the generated priority tables.
// Readers load it via one atomic pointer read; GenerateAllQualityPriorities
// builds a fresh snapshot and swaps it in, so hot-path lookups need no locks
// and stay consistent even when a config reload regenerates the tables.
//
// Production lookups compute priorities from the compact per-profile tables.
// The full []Prioarr list is materialized on demand (rare admin/debug API),
// never kept resident. Tests install explicit slices via the override field.
type prioritySnapshot struct {
	byName   map[string]*profilePrio // lowercased group name -> tables
	profiles []*profilePrio          // generation order, for materialization
	override *overrideTables         // non-nil only in tests
}

var (
	prioSnapshot  atomic.Pointer[prioritySnapshot]
	mediainfopath string
	ffprobepath   string
	arrExtended   = [4]string{
		"extended",
		"extended cut",
		"extended.cut",
		"extended-cut",
	}

	// Thread safety: Protect mutable global state.
	prioritiesMu   sync.Mutex   // Serializes priority table generation
	scanpatternsMu sync.RWMutex // Protects scanpatterns slice

	scanpatterns       []regexpattern
	globalscanpatterns = [8]regexpattern{
		{name: "season", last: false, re: `(?i)(s?(\d{1,4}))(?: )?[ex]`, getgroup: 2},
		{
			name:     "episode",
			last:     false,
			re:       `(?i)((?:\d{1,4})(?: )?[ex](?: )?(\d{1,3})(?:\b|_|e|$))`,
			getgroup: 2,
		},
		{
			name:     "identifier",
			last:     false,
			re:       `(?i)((s?\d{1,4}(?:(?:(?: )?-?(?: )?[ex-]\d{1,3})+)|\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`,
			getgroup: 2,
		},
		{
			name:     logger.StrDate,
			last:     false,
			re:       `(?i)(?:\b|_)((\d{2,4}(?:\.|-| |_)\d{1,2}(?:\.|-| |_)\d{1,2}))(?:\b|_)`,
			getgroup: 2,
		},
		{name: "year", last: true, re: `(?:\b|_)(((?:19\d|20\d)\d))(?:\b|_)`, getgroup: 2},
		{
			name:     "audio",
			last:     false,
			re:       `(?i)(?:\b|_)((dd[0-9\\.]+|dd[p+][0-9\\.]+|dts\W?hd(?:\W?ma)?))(?:\b|_)`,
			getgroup: 2,
		},
		{name: "imdb", last: false, re: `(?i)(?:\b|_)((tt[0-9]{4,9}))(?:\b|_)`, getgroup: 2},
		{name: "tvdb", last: false, re: `(?i)(?:\b|_)((tvdb[0-9]{2,9}))(?:\b|_)`, getgroup: 2},
	}
)

// getmatchesroot finds all substring matches in m.Str using the provided regular
// expression pattern. It returns a slice of integer indices indicating the start
// and end positions of the matched substring(s). For regex capture groups, the
// even indices are start positions and odd indices are end positions.
func (pattern *regexpattern) getmatchesroot(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
) (int, int) {
	// For series, use all matches when it's the last year pattern
	// (movies shorten year pattern, series does not)
	useall := pattern.last && pattern.name == "year" &&
		!mediatype.Get(cfgp.IsType).ShortenYearPattern()

	matchest := database.RunRetRegex(
		pattern.re,
		m.Str,
		useall,
	)
	if len(matchest) == 0 {
		return -1, -1
	}

	lensubmatches := len(matchest)
	groupIndex := pattern.getgroup * 2
	// Ensure we have enough groups for the requested index
	if groupIndex+1 >= lensubmatches || matchest[groupIndex+1] == -1 {
		return -1, -1
	}

	// All branches returned the same value, so simplified to direct return
	return matchest[groupIndex], matchest[groupIndex+1]
}

// getImdbFilename returns the path to the init_imdb executable
// based on the current OS. For Windows it returns init_imdb.exe,
// for other OSes it returns ./init_imdb.
func getImdbFilename() string {
	if runtime.GOOS == "windows" {
		return "init_imdb.exe"
	}

	return "./init_imdb"
}

// emptyPrioritySnapshot is returned before the first generation so readers
// never see a nil snapshot.
var emptyPrioritySnapshot = &prioritySnapshot{}

// loadPrioSnapshot returns the current immutable priority snapshot.
func loadPrioSnapshot() *prioritySnapshot {
	if s := prioSnapshot.Load(); s != nil {
		return s
	}

	return emptyPrioritySnapshot
}

// makePriokey builds the lookup key for one combination within a quality group.
func makePriokey(group string, reso, qual, codec, aud, af uint) priokey {
	return priokey{
		group: strings.ToLower(group),
		reso:  reso,
		qual:  qual,
		codec: codec,
		aud:   aud,
		af:    af,
	}
}

// setPrioritiesForTest installs the given tables as the active snapshot via the
// override path. Only used by tests.
func setPrioritiesForTest(all, wanted []Prioarr) {
	ov := &overrideTables{
		all:       all,
		wanted:    wanted,
		allIdx:    make(map[priokey]int, len(all)),
		wantedIdx: make(map[priokey]int, len(wanted)),
	}
	for idx := range all {
		ov.allIdx[makePriokey(all[idx].QualityGroup, all[idx].ResolutionID, all[idx].QualityID, all[idx].CodecID, all[idx].AudioID, all[idx].AudioFormatID)] = idx
	}

	for idx := range wanted {
		ov.wantedIdx[makePriokey(wanted[idx].QualityGroup, wanted[idx].ResolutionID, wanted[idx].QualityID, wanted[idx].CodecID, wanted[idx].AudioID, wanted[idx].AudioFormatID)] = idx
	}

	prioSnapshot.Store(&prioritySnapshot{override: ov})
}

// lookupPriority returns the additive priority for one combination within a
// quality group. inAll is true when the combination exists at all; inWanted is
// true when it also passes the wanted filter. It mirrors the previous
// map-of-Cartesian-product behaviour without materializing it.
func (s *prioritySnapshot) lookupPriority(
	group string,
	reso, qual, codec, aud, af uint,
) (prio int, inAll, inWanted bool) {
	if s.override != nil {
		k := makePriokey(group, reso, qual, codec, aud, af)
		if idx, ok := s.override.wantedIdx[k]; ok {
			return s.override.wanted[idx].Priority, true, true
		}

		if idx, ok := s.override.allIdx[k]; ok {
			return s.override.all[idx].Priority, true, false
		}

		return 0, false, false
	}

	p := s.byName[strings.ToLower(group)]
	if p == nil {
		return 0, false, false
	}

	rq, ok := p.resoQual[[2]uint{reso, qual}]
	if !ok {
		return 0, false, false
	}

	cp, ok := p.codec[codec]
	if !ok {
		return 0, false, false
	}

	ap, ok := p.audio[aud]
	if !ok {
		return 0, false, false
	}

	fp, ok := p.audioFmt[af]
	if !ok {
		return 0, false, false
	}

	return rq.prio + cp + ap + fp, true, rq.wanted
}

// materialize builds the full []Prioarr list on demand. wantedOnly restricts it
// to combinations that pass the wanted filter. Used only by the rare admin/debug
// API endpoints, so the large slice is allocated transiently, never kept resident.
func (s *prioritySnapshot) materialize(wantedOnly bool) []Prioarr {
	if s.override != nil {
		if wantedOnly {
			return slices.Clone(s.override.wanted)
		}

		return slices.Clone(s.override.all)
	}

	var out []Prioarr

	for _, p := range s.profiles {
		for _, rqKey := range p.resoQualOrder {
			rq := p.resoQual[rqKey]
			if wantedOnly && !rq.wanted {
				continue
			}

			for _, c := range p.codecOrder {
				for _, a := range p.audioOrder {
					for _, f := range p.afOrder {
						out = append(out, Prioarr{
							QualityGroup:  p.name,
							ResolutionID:  rqKey[0],
							QualityID:     rqKey[1],
							CodecID:       c.id,
							AudioID:       a.id,
							AudioFormatID: f.id,
							Priority:      rq.prio + c.prio + a.prio + f.prio,
						})
					}
				}
			}
		}
	}

	return out
}

// Getallprios returns all wanted quality priorities. The list is built on
// demand; prefer FindPriorityValue for single lookups. Thread-safe.
func Getallprios() []Prioarr {
	return loadPrioSnapshot().materialize(true)
}

// Getcompleteallprios returns all quality priorities (built on demand).
// Thread-safe. Useful for testing and the admin API.
func Getcompleteallprios() []Prioarr {
	return loadPrioSnapshot().materialize(false)
}

// LoadDBPatterns loads patterns from database if not already loaded.
// Thread-safe for concurrent calls using a write lock.
func LoadDBPatterns() {
	// Fast path: check if already loaded with read lock
	scanpatternsMu.RLock()

	if len(scanpatterns) >= 1 {
		scanpatternsMu.RUnlock()
		return
	}

	scanpatternsMu.RUnlock()

	// Slow path: load patterns with write lock
	scanpatternsMu.Lock()
	defer scanpatternsMu.Unlock()

	// Double-check after acquiring write lock (another goroutine may have loaded)
	if len(scanpatterns) >= 1 {
		return
	}

	capacity := len(globalscanpatterns)
	for _, val := range database.DBConnect.GetaudiosIn {
		if val.UseRegex {
			capacity++
		}
	}

	for _, val := range database.DBConnect.GetcodecsIn {
		if val.UseRegex {
			capacity++
		}
	}

	for _, val := range database.DBConnect.GetqualitiesIn {
		if val.UseRegex {
			capacity++
		}
	}

	for _, val := range database.DBConnect.GetresolutionsIn {
		if val.UseRegex {
			capacity++
		}
	}

	scanpatterns = make([]regexpattern, 0, capacity)
	scanpatterns = append(scanpatterns, globalscanpatterns[:]...)

	for _, pattern := range globalscanpatterns {
		database.SetStaticRegexp(pattern.re)
	}

	addPatterns := func(items []database.Qualities, patternName string) {
		for _, val := range items {
			if val.UseRegex {
				scanpatterns = append(scanpatterns, regexpattern{
					name:     patternName,
					last:     false,
					re:       val.Regex,
					getgroup: val.Regexgroup,
				})
				database.SetStaticRegexp(val.Regex)
			}
		}
	}
	addPatterns(database.DBConnect.GetaudiosIn, "audio")
	addPatterns(database.DBConnect.GetresolutionsIn, "resolution")
	addPatterns(database.DBConnect.GetqualitiesIn, "quality")
	addPatterns(database.DBConnect.GetcodecsIn, "codec")
}

// GenerateCutoffPriorities iterates through the media type and list
// configurations, and sets the CutoffPriority field for any list that
// does not already have it set. It calls NewCutoffPrio to calculate
// the priority value based on the cutoff quality and resolution.
func GenerateCutoffPriorities() {
	config.RangeSettingsMedia(func(_ string, media *config.MediaTypeConfig) error {
		for _, lst := range media.ListsMap {
			if lst.CfgQuality.CutoffPriority != 0 {
				continue
			}

			m := database.ParseInfo{
				Quality:    lst.CfgQuality.CutoffQuality,
				Resolution: lst.CfgQuality.CutoffResolution,
			}
			GetPriorityMapQual(&m, media, lst.CfgQuality, true, false) // newCutoffPrio(media, idxi)

			lst.CfgQuality.CutoffPriority = m.Priority
		}

		return nil
	})
}

// processPatternMatch handles the processing of a matched regex pattern in file parsing.
// It updates the ParseInfo struct with matched information based on the pattern type,
// and manages the start and end indices of the matched substring within the original string.
// The function is used internally during file name parsing to extract metadata like
// IMDb ID, year, season, episode, and other media-related information.
func processPatternMatch(
	m *database.ParseInfo,
	pattern *regexpattern,
	strStart, strEnd int,
	cfgp *config.MediaTypeConfig,
	start, end *int,
) {
	shorten := pattern.name != "year" || mediatype.Get(cfgp.IsType).ShortenYearPattern()
	if shorten {
		if index := strings.Index(m.Str, m.Str[strStart:strEnd]); index == 0 {
			if matchLen := len(m.Str[strStart:strEnd]); matchLen != len(m.Str) && matchLen < *end {
				*start = matchLen
			}
		} else if index < *end && index > *start {
			*end = index
		}
	}

	if m.FirstIDX == 0 || strStart < m.FirstIDX {
		m.FirstIDX = strStart
	}

	// Use a map for better performance on pattern matching
	switch pattern.name {
	case "imdb":
		m.Imdb = m.Str[strStart:strEnd]
	case "tvdb":
		m.Tvdb, _ = strings.CutPrefix(m.Str[strStart:strEnd], logger.StrTvdb)
		if logger.HasPrefixI(m.Tvdb, logger.StrTvdb) {
			m.Tvdb = m.Tvdb[4:]
		}

	case "year":
		m.FirstYearIDX = strStart
		m.Year = logger.StringToUInt16(m.Str[strStart:strEnd])

	case "season":
		m.SeasonStr = m.Str[strStart:strEnd]
		m.Season = logger.StringToInt(m.SeasonStr)

	case "episode":
		m.EpisodeStr = m.Str[strStart:strEnd]
		m.Episode = logger.StringToInt(m.EpisodeStr)

	case "identifier":
		m.Identifier = m.Str[strStart:strEnd]
	case logger.StrDate:
		m.Date = m.Str[strStart:strEnd]
	case "audio":
		m.Audio = m.Str[strStart:strEnd]
	case "resolution":
		m.Resolution = m.Str[strStart:strEnd]
	case "quality":
		m.Quality = m.Str[strStart:strEnd]
	case "codec":
		m.Codec = m.Str[strStart:strEnd]
	}
}

// shouldSkipPattern determines whether a specific regex pattern should be skipped during file parsing.
// It checks various conditions based on the pattern type, media type configuration, and existing parsed information.
// Returns true if the pattern should be skipped, false otherwise.
func shouldSkipPattern(
	pattern *regexpattern,
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	onlyifempty bool,
	conttt, conttvdb bool,
) bool {
	switch pattern.name {
	case "imdb":
		return cfgp.IsType != config.MediaTypeMovie || !conttt || (onlyifempty && m.Imdb != "")
	case "tvdb":
		return cfgp.IsType != config.MediaTypeSeries || !conttvdb || (onlyifempty && m.Tvdb != "")
	case "season":
		return cfgp.IsType != config.MediaTypeSeries || (onlyifempty && m.Season != 0)
	case "identifier":
		return cfgp.IsType != config.MediaTypeSeries || (onlyifempty && m.Identifier != "")
	case "episode":
		return cfgp.IsType != config.MediaTypeSeries || (onlyifempty && m.Episode != 0)
	case "date":
		return cfgp.IsType != config.MediaTypeSeries || (onlyifempty && m.Date != "")
	case "audio":
		return m.Audio != ""
	case "codec":
		return m.Codec != ""
	case "quality":
		return m.Quality != ""
	case "resolution":
		return m.Resolution != ""
	}

	return false
}

// newFileParser reuses a FileParser instance. It sets the filename,
// media config, list ID, and allow title search flag. It runs the main parsing
// logic like splitting the filename on delimiters, running regex matches,
// and cleaning up the parsed title and identifier.
func newFileParser(
	cleanName string,
	onlyifempty bool,
	cfgp *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	m.ListID = listid
	if !onlyifempty || m.File == "" {
		m.File = cleanName
	}

	m.Str = m.File
	logger.StringReplaceWithP(&m.Str, '_', ' ')

	m.Str = logger.Trim(m.Str, '[', ']')

	if !config.GetSettingsGeneral().DisableParserStringMatch {
		m.Parsegroup("audio", onlyifempty, database.DBConnect.AudioStrIn)
		m.Parsegroup("codec", onlyifempty, database.DBConnect.CodecStrIn)
		m.Parsegroup("quality", onlyifempty, database.DBConnect.QualityStrIn)
		m.Parsegroup("resolution", onlyifempty, database.DBConnect.ResolutionStrIn)
	}

	m.Parsegroup("extended", onlyifempty, arrExtended[:])
	m.ParsegroupEntry("proper")
	m.ParsegroupEntry("repack")

	var (
		start, end = 0, len(m.Str)
		conttt     = logger.ContainsI(m.Str, logger.StrTt)
		conttvdb   = logger.ContainsI(m.Str, logger.StrTvdb)
	)

	for i := range scanpatterns {
		pattern := &scanpatterns[i]
		if shouldSkipPattern(pattern, m, cfgp, onlyifempty, conttt, conttvdb) {
			continue
		}

		if strStart, strEnd := pattern.getmatchesroot(m, cfgp); strStart != -1 && strEnd != -1 {
			processPatternMatch(m, pattern, strStart, strEnd, cfgp, &start, &end)
		}
	}

	mediatype.Get(cfgp.IsType).GenerateIdentifier(m, onlyifempty)

	if m.FirstIDX != 0 && m.FirstIDX < m.FirstYearIDX {
		end = m.FirstIDX
	}

	var titleStr string
	if end < start {
		logger.Logtype("debug", 0).
			Str(logger.StrPath, m.File).
			Int("start", start).
			Int("end", end).
			Msg("EndIndex < startindex")

		titleStr = m.File[start:]
	} else {
		titleStr = m.File[start:end]
	}

	if idx := strings.IndexRune(titleStr, '('); idx != -1 {
		titleStr = titleStr[:idx]
	}

	m.Str = titleStr

	if onlyifempty && m.Title != "" {
		return
	}

	m.Title = titleStr
	if strings.ContainsRune(m.Title, '.') && !strings.ContainsRune(m.Title, ' ') {
		logger.StringReplaceWithP(&m.Title, '.', ' ')
	}

	m.Title = logger.TrimSpace(logger.TrimRight(logger.TrimSpace(m.Title), '-', '.', ' '))
}

// SplitByFull splits a string into two parts by the first
// occurrence of the split rune. It returns the part before the split.
// If the split rune is not found, it returns the original string.
func splitByFull(str string, splitby rune) string {
	if idx := strings.IndexRune(str, splitby); idx != -1 {
		return str[:idx]
	}

	return str
}

// ParseFile parses the given video file to extract metadata.
// It accepts a video file path, booleans to indicate whether to use the path and folder
// to extract metadata, a media type config, a list ID, and a FileParser to populate.
// It calls ParseFileP to parse the file and populate the FileParser, which is then returned.
func ParseFile(
	videofile string,
	usepath, usefolder bool,
	cfgp *config.MediaTypeConfig,
	listid int,
) *database.ParseInfo {
	m := database.PLParseInfo.Get()
	ParseFileP(videofile, usepath, usefolder, cfgp, listid, m)
	return m
}

// ParseFileP parses a video file to extract metadata.
// It accepts the video file path, booleans to determine parsing behavior,
// a media type config, list ID, and existing parser to populate.
// It returns the populated parser after attempting to extract metadata.
func ParseFileP(
	videofile string,
	usepath, usefolder bool,
	cfgp *config.MediaTypeConfig,
	listid int,
	m *database.ParseInfo,
) {
	filename := videofile
	if usepath {
		filename = filepath.Base(videofile)
	}

	newFileParser(filename, false, cfgp, listid, m)

	if m.Quality != "" && m.Resolution != "" {
		return
	}

	if usefolder && usepath {
		newFileParser(filepath.Base(filepath.Dir(videofile)), true, cfgp, listid, m)
	}
}

// GetDBIDs retrieves the database IDs needed to locate a movie or TV episode in the database.
// It takes a FileParser struct pointer as input. This contains metadata about the media file.
// It first checks if it is a movie or TV show based on the config.
// For movies:
// It tries to lookup the movie by IMDb ID, trying prefixes if not found
// If still not found, it searches by title
// It gets the movie ID and list ID
// Returns error if not found
// For TV shows:
// Lookup by TVDB ID
// If not found, search by title and year
// Get the episode ID using other metadata
// Get the series ID and list ID
// Returns error if IDs not found
// The goal is to map the metadata from the file to the database IDs needed to locate that movie or episode. This allows further processing on the database data.
// It returns errors if it can't find the expected IDs.
func GetDBIDs(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	allowsearchtitle bool,
	addFound bool,
) error {
	if m == nil {
		return logger.ErrNotFound
	}

	m.ListID = -1

	return mediatype.Get(cfgp.IsType).GetDBIDsFull(m, cfgp, allowsearchtitle, addFound)
}

// ParseVideoFile parses metadata for a video file using ffprobe or MediaInfo.
// It first tries ffprobe, then falls back to MediaInfo if enabled.
// It takes a FileParser, path to the video file, and quality settings.
// It populates the FileParser with metadata parsed from the file.
// Returns an error if both parsing methods fail.
func ParseVideoFile(
	ctx context.Context,
	m *database.ParseInfo,
	quality *config.QualityConfig,
) error {
	if m.File == "" {
		return logger.ErrNotFound
	}

	// For supported containers (MP4/MOV, Matroska/WebM, AVI), parse the header
	// natively first to avoid an ffprobe subprocess per file. It only "wins" when
	// it confidently extracts the video codec and resolution; otherwise (and for
	// other containers like MPEG-PS) fall back to ffprobe/mediainfo below.
	if nativeProbeSupportsExt(m.File) {
		if result, nerr := nativeProbe(m.File); nerr == nil && result != nil {
			if parseffprobe(m, quality, result) == nil {
				return nil
			}
		}
	}

	err := parsemedia(ctx, !config.GetSettingsGeneral().UseMediainfo, m, quality)
	if err == nil {
		return nil
	}

	if !config.GetSettingsGeneral().UseMediaFallback {
		return err
	}

	return parsemedia(ctx, config.GetSettingsGeneral().UseMediainfo, m, quality)
}

// parsemedia attempts to parse the metadata of a video file using either ffprobe or MediaInfo.
// If ffprobe is enabled, it first tries to parse the file using ffprobe. If that fails or ffprobe is
// not enabled, it falls back to using MediaInfo to parse the file.
// It takes a boolean indicating whether to use ffprobe, a pointer to a ParseInfo struct to populate
// with the parsed metadata, and a pointer to a QualityConfig struct.
// Returns an error if both parsing methods fail.
func parsemedia(
	ctx context.Context,
	ffprobe bool,
	m *database.ParseInfo,
	quality *config.QualityConfig,
) error {
	if m.File == "" {
		return logger.ErrNotFound
	}

	if ffprobe {
		if ExecCmdJSON[ffProbeJSON](ctx, m.File, "ffprobe", m, quality) == nil {
			return nil
		}
	}

	return ExecCmdJSON[mediaInfoJSON](ctx, m.File, "mediainfo", m, quality)
}

// parseffprobe parses metadata from the ffprobe JSON output and updates the provided
// ParseInfo with the extracted data. It handles audio and video tracks, extracting
// codec, resolution, and other relevant information. It also determines the priority
// of the media based on the provided QualityConfig.
func parseffprobe(m *database.ParseInfo, quality *config.QualityConfig, result *ffProbeJSON) error {
	if len(result.Streams) == 0 {
		return logger.ErrTracksEmpty
	}

	if result.Error.Code != 0 {
		return errors.New(
			"ffprobe error code " + strconv.Itoa(result.Error.Code) + " " + result.Error.String,
		)
	}

	if duration, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
		m.Runtime = int(math.Round(duration))
	}

	var redetermineprio bool

	var n int
	for i := range result.Streams {
		if result.Streams[i].language() != "" &&
			strings.EqualFold(result.Streams[i].CodecType, "audio") {
			n++
		}
	}

	if n > 1 {
		m.Languages = make([]string, 0, n)
	}

	for i := range result.Streams {
		stream := &result.Streams[i]
		if isAudioStream(stream) {
			if lang := stream.language(); lang != "" {
				m.Languages = append(m.Languages, lang)
			}

			if updateAudio(m, stream) {
				redetermineprio = true
			}
		} else if isVideoStream(stream) {
			if updateVideo(m, stream) {
				redetermineprio = true
			}
		}
	}

	if redetermineprio {
		updatePriority(m, quality)
	}

	return nil
}

// isAudioStream checks if the given stream is an audio stream by comparing its codec type.
// It returns true if the stream's codec type is "audio" (case-insensitive), false otherwise.
func isAudioStream(stream *ffProbeStream) bool {
	return strings.EqualFold(stream.CodecType, "audio")
}

// isVideoStream checks if the given stream is a video stream by comparing its codec type.
// It returns true if the stream's codec type is "video" (case-insensitive), false otherwise.
func isVideoStream(stream *ffProbeStream) bool {
	return strings.EqualFold(stream.CodecType, "video")
}

// normalizeDimensions ensures height is always smaller than width by swapping if necessary.
// This normalizes dimensions for consistent processing.
func normalizeDimensions(m *database.ParseInfo) {
	if m.Height > m.Width {
		m.Height, m.Width = m.Width, m.Height
	}
}

// updateAudio updates the audio metadata in the ParseInfo struct based on the provided stream information.
// It updates the audio codec and sets the corresponding audio ID using the Gettypeids method.
// Returns true if the audio codec has changed, false otherwise.
func updateAudio(m *database.ParseInfo, stream *ffProbeStream) bool {
	if m.Audio == "" || (stream.CodecName != "" && !strings.EqualFold(stream.CodecName, m.Audio)) {
		m.Audio = stream.CodecName
		m.AudioID = m.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
		return true
	}

	return false
}

// updateVideo updates the video metadata in the ParseInfo struct based on the provided stream information.
// It updates the video resolution, codec, and dimensions. If the codec or resolution changes,
// it updates the corresponding IDs using the Gettypeids method. Returns true if either the
// codec or resolution has changed, false otherwise.
func updateVideo(m *database.ParseInfo, stream *ffProbeStream) bool {
	m.Height = stream.Height
	m.Width = stream.Width

	var codecChanged bool

	// Handle special case for MPEG4/XVID
	if strings.EqualFold(stream.CodecName, "mpeg4") &&
		strings.EqualFold(stream.CodecTagString, "xvid") {
		if m.Codec == "" ||
			(stream.CodecTagString != "" && !strings.EqualFold(stream.CodecTagString, m.Codec)) {
			m.Codec = stream.CodecTagString
			codecChanged = true
		}
	} else if m.Codec == "" || (stream.CodecName != "" && !strings.EqualFold(stream.CodecName, m.Codec)) {
		m.Codec = stream.CodecName
		codecChanged = true
	}

	if codecChanged {
		m.CodecID = m.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	// Normalize dimensions
	normalizeDimensions(m)

	var resolutionChanged bool
	if getreso := m.Parseresolution(); getreso != "" &&
		(m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
		m.Resolution = getreso
		m.ResolutionID = m.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
		resolutionChanged = true
	}

	return codecChanged || resolutionChanged
}

// updatePriority determines the priority of a media file based on its resolution, quality, codec, and audio characteristics.
// It uses the provided QualityConfig to find the appropriate priority index and sets the Priority field accordingly.
// If no matching priority is found, the priority remains unchanged.
func updatePriority(m *database.ParseInfo, quality *config.QualityConfig) {
	s := loadPrioSnapshot()

	if prio, _, inWanted := s.lookupPriority(
		quality.Name,
		m.ResolutionID,
		m.QualityID,
		m.CodecID,
		m.AudioID,
		m.AudioFormatID,
	); inWanted {
		m.Priority = prio
	}
}

// parsemediainfo parses media information from a mediaInfoJSON object and updates the
// provided ParseInfo with the extracted data. It handles audio and video tracks,
// extracting codec, resolution, and other relevant information. It also determines
// the priority of the media based on the provided QualityConfig.
func parsemediainfo(
	m *database.ParseInfo,
	quality *config.QualityConfig,
	info *mediaInfoJSON,
) error {
	if len(info.Media.Track) == 0 {
		return logger.ErrTracksEmpty
	}

	var (
		redetermineprio bool
		n               int
	)

	for i := range info.Media.Track {
		if info.Media.Track[i].Type == "Audio" && info.Media.Track[i].Language != "" {
			n++
		}
	}

	if n > 1 {
		m.Languages = make([]string, 0, n)
	}

	for i := range info.Media.Track {
		track := &info.Media.Track[i]
		switch track.Type {
		case "Audio":
			if track.Language != "" {
				m.Languages = append(m.Languages, track.Language)
			}

			if updateAudioFromMediaInfo(m, track) {
				redetermineprio = true
			}

		// MediaInfo emits "@type": "Video" capitalized - the previous
		// lowercase "video" case never matched, so resolution/codec/runtime
		// were silently skipped whenever MediaInfo was the analyzer.
		case "Video":
			if updateVideoFromMediaInfo(m, track) {
				redetermineprio = true
			}
		}
	}

	if redetermineprio {
		updatePriority(m, quality)
	}

	return nil
}

// updateAudioFromMediaInfo updates the ParseInfo with audio track details from MediaInfo.
// It handles audio codec and sets the corresponding audio ID.
// Returns true if the audio codec changes, false otherwise.
func updateAudioFromMediaInfo(m *database.ParseInfo, track *mediaInfoTrack) bool {
	// Compare against Format - that's also what gets assigned (comparing
	// CodecID against m.Audio mixed two different identifier spaces).
	if m.Audio == "" || (track.Format != "" && !strings.EqualFold(track.Format, m.Audio)) {
		m.Audio = track.Format
		m.AudioID = m.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
		return true
	}

	return false
}

// updateVideoFromMediaInfo updates the ParseInfo with video track details from MediaInfo.
// It handles codec, resolution, height, width, and runtime information.
// Returns true if codec or resolution changes, false otherwise.
func updateVideoFromMediaInfo(m *database.ParseInfo, track *mediaInfoTrack) bool {
	var codecChanged bool

	// Handle special case for MPEG4/XVID
	if strings.EqualFold(track.Format, "mpeg4") &&
		strings.EqualFold(track.CodecID, "xvid") {
		if m.Codec == "" || (track.CodecID != "" && !strings.EqualFold(track.CodecID, m.Codec)) {
			m.Codec = track.CodecID
			codecChanged = true
		}
	} else if m.Codec == "" || (track.Format != "" && !strings.EqualFold(track.Format, m.Codec)) {
		m.Codec = track.Format
		codecChanged = true
	}

	if codecChanged {
		m.CodecID = m.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	m.Height = logger.StringToInt(track.Height)
	m.Width = logger.StringToInt(track.Width)
	m.Runtime = logger.StringToInt(splitByFull(track.Duration, '.'))

	// Normalize dimensions
	normalizeDimensions(m)

	var resolutionChanged bool
	if getreso := m.Parseresolution(); getreso != "" &&
		(m.Resolution == "" || !strings.EqualFold(getreso, m.Resolution)) {
		m.Resolution = getreso
		m.ResolutionID = m.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
		resolutionChanged = true
	}

	return codecChanged || resolutionChanged
}

// GetPriorityMapQual calculates priority for a ParseInfo based on its resolution,
// quality, codec, and audio IDs. It looks up missing IDs, applies defaults if configured,
// and maps IDs to names. It then calls getIDPriority to calculate the priority value.
// For music and audiobook media types, it uses audio-specific priority calculation.
func GetPriorityMapQual(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	useall, checkwanted bool,
) {
	// Check if this is an audio media type (music or audiobook)
	if cfgp != nil &&
		(cfgp.IsType == config.MediaTypeMusic || cfgp.IsType == config.MediaTypeAudiobook) {
		GetPriorityMapQualAudio(m, cfgp, quality, useall)
		return
	}

	// Check if this is a book type (ebooks)
	if cfgp != nil && cfgp.IsType == config.MediaTypeBook {
		GetPriorityMapQualBook(m, cfgp, quality, useall)
		return
	}

	if m.ResolutionID == 0 {
		m.ResolutionID = m.Gettypeids(m.Resolution, database.DBConnect.GetresolutionsIn)
	}

	if m.QualityID == 0 {
		m.QualityID = m.Gettypeids(m.Quality, database.DBConnect.GetqualitiesIn)
	}

	if m.CodecID == 0 {
		m.CodecID = m.Gettypeids(m.Codec, database.DBConnect.GetcodecsIn)
	}

	if m.AudioID == 0 {
		m.AudioID = m.Gettypeids(m.Audio, database.DBConnect.GetaudiosIn)
	}

	if m.ResolutionID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetresolutionsIn, cfgp, true)
		if idx != -1 {
			m.ResolutionID = database.DBConnect.GetresolutionsIn[idx].ID
		}
	}

	if m.QualityID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetqualitiesIn, cfgp, false)
		if idx != -1 {
			m.QualityID = database.DBConnect.GetqualitiesIn[idx].ID
		}
	}

	updateNamesFromIDs(m)

	var reso, qual, aud, codec uint

	if quality.UseForPriorityResolution || useall {
		reso = m.ResolutionID
	}

	if quality.UseForPriorityQuality || useall {
		qual = m.QualityID
	}

	// Codec and audio contribute to priority only in the useall path (structure/
	// cutoff table building). In the search/upgrade path (useall=false) for
	// movies/series, only resolution+quality are compared so a different audio
	// track or codec does not look like an upgrade and trigger a re-download.
	if useall {
		if quality.UseForPriorityAudio {
			aud = m.AudioID
		}

		if quality.UseForPriorityCodec {
			codec = m.CodecID
		}
	}

	prio, found := findPriorityValue(reso, qual, codec, aud, 0, quality, checkwanted)
	if !found {
		m.TempTitle = BuildPrioStr(reso, qual, codec, aud, 0)
		logger.Logtype("debug", 2).
			Str("in", quality.Name).
			Str("searched for", m.TempTitle).
			Msg("prio not found")

		m.Priority = 0

		return
	}

	m.Priority = prio

	if quality.UseForPriorityOther || useall {
		applyPriorityModifiers(m)
	}
}

// GetPriorityMapQualAudio calculates priority for music and audiobook releases.
// It looks up AudioFormatID from the qualities table (type 5) and uses the same
// priority system as movies/series via findPriorityIndex. Bitrate, sample rate,
// and bit depth are applied as bonus modifiers on top.
func GetPriorityMapQualAudio(
	m *database.ParseInfo,
	cfgp *config.MediaTypeConfig,
	quality *config.QualityConfig,
	useall bool,
) {
	// Look up audio format from qualities table
	if m.AudioFormatID == 0 {
		m.AudioFormatID = m.Gettypeids(m.AudioFormat, database.DBConnect.GetaudioformatsIn)
	}

	// Fallback to default_quality from MediaTypeConfig if no audio format detected
	if m.AudioFormatID == 0 && cfgp != nil {
		idx := database.Getqualityidxbyname(database.DBConnect.GetaudioformatsIn, cfgp, false)
		if idx != -1 {
			m.AudioFormatID = database.DBConnect.GetaudioformatsIn[idx].ID
		}
	}

	logger.Logtype("debug", 0).
		Str("audioformat_str", m.AudioFormat).
		Uint("audioformat_id", m.AudioFormatID).
		Int("audioformats_count", len(database.DBConnect.GetaudioformatsIn)).
		Str("quality_name", quality.Name).
		Bool("use_for_priority_audio_format", quality.UseForPriorityAudioFormat).
		Str("default_quality", cfgp.DefaultQuality).
		Msg("GetPriorityMapQualAudio debug")

	var audioformat uint
	if quality.UseForPriorityAudioFormat || useall {
		audioformat = m.AudioFormatID
	}

	// Use the same priority lookup as movies (resolution/quality/codec/audio all 0 for audio media)
	prio, found := findPriorityValue(0, 0, 0, 0, audioformat, quality, false)
	if !found {
		logger.Logtype("debug", 0).
			Uint("audioformat", audioformat).
			Str("quality_name", quality.Name).
			Int("profiles", len(loadPrioSnapshot().profiles)).
			Msg("GetPriorityMapQualAudio priority not found")

		m.Priority = 0

		return
	}

	m.Priority = prio

	// Bitrate bonus modifier
	if quality.UseForPriorityAudioBitrate || useall {
		m.Priority += calculateAudioBitratePriority(m.AudioBitrate, m.AudioFormat)
	}

	// Bit depth bonus for hi-res audio (24-bit, 32-bit)
	if m.AudioBitDepth >= 24 {
		m.Priority += (m.AudioBitDepth - 16)
	}

	// Sample rate bonus for hi-res audio (above 44.1kHz)
	if m.AudioSampleRate > 44100 {
		if m.AudioSampleRate >= 96000 {
			m.Priority += 10
		} else if m.AudioSampleRate >= 48000 {
			m.Priority += 5
		}
	}

	// Apply standard modifiers (proper, repack, extended)
	if quality.UseForPriorityOther || useall {
		applyPriorityModifiers(m)
	}
}

// calculateAudioBitratePriority returns priority bonus based on audio bitrate.
// Higher bitrates get higher priority, especially for lossy formats.
func calculateAudioBitratePriority(bitrate int, format string) int {
	// For lossless formats, bitrate isn't as meaningful (it varies with content)
	if parser_v2.IsLosslessFormat(format) {
		if bitrate >= 1000 {
			return 5 // Hi-res lossless
		}

		return 0
	}

	// For lossy formats, bitrate directly correlates with quality
	switch {
	case bitrate >= 320:
		return 30 // 320 kbps - highest quality lossy
	case bitrate >= 256:
		return 25 // 256 kbps - very good quality
	case bitrate >= 192:
		return 20 // 192 kbps - good quality
	case bitrate >= 160:
		return 15 // 160 kbps - acceptable quality
	case bitrate >= 128:
		return 10 // 128 kbps - standard quality
	case bitrate >= 96:
		return 5 // 96 kbps - low quality
	case bitrate > 0:
		return 0 // Very low quality
	}

	return 0 // Unknown bitrate
}

// GetPriorityMapQualBook calculates priority for book/ebook releases based on
// book-specific attributes: format (EPUB, PDF, MOBI, etc.) and source quality.
// This provides appropriate priority ranking for ebook content where video attributes don't apply.
func GetPriorityMapQualBook(
	m *database.ParseInfo,
	_ *config.MediaTypeConfig,
	quality *config.QualityConfig,
	useall bool,
) {
	// Base priority starts at 100
	m.Priority = 100

	// Book format priority
	m.Priority += calculateBookFormatPriority(m.Quality)

	// Source quality bonus (retail > scan)
	if logger.ContainsI(m.Quality, "retail") {
		m.Priority += 20
	}

	// Apply standard modifiers (proper, repack)
	if quality.UseForPriorityOther || useall {
		applyPriorityModifiers(m)
	}
}

// calculateBookFormatPriority returns priority bonus based on ebook format.
// EPUB and AZW3 are generally preferred for reflow capability.
func calculateBookFormatPriority(format string) int {
	switch {
	case logger.ContainsI(format, "epub"):
		return 50 // EPUB - most versatile, best reflow
	case logger.ContainsI(format, "azw3"), logger.ContainsI(format, "azw"):
		return 45 // AZW3/AZW - Kindle format, good quality
	case logger.ContainsI(format, "mobi"):
		return 40 // MOBI - older Kindle format
	case logger.ContainsI(format, "pdf"):
		return 30 // PDF - fixed layout, less flexible
	case logger.ContainsI(format, "cbz"), logger.ContainsI(format, "cbr"):
		return 35 // Comic formats
	case logger.ContainsI(format, "djvu"):
		return 25 // DjVu - good for scans but less compatible
	case logger.ContainsI(format, "txt"), logger.ContainsI(format, "rtf"):
		return 10 // Plain text/RTF - basic
	case logger.ContainsI(format, "doc"), logger.ContainsI(format, "docx"):
		return 15 // Word documents
	}

	return 0 // Unknown format
}

// updateNamesFromIDs populates the name fields of a ParseInfo struct based on its corresponding ID fields.
// It retrieves names for resolution, quality, audio, and codec by matching IDs with predefined database entries.
// If an ID is non-zero, it attempts to find and set the corresponding name from the respective database slice.
func updateNamesFromIDs(m *database.ParseInfo) {
	if m.ResolutionID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetresolutionsIn, 1); idx != -1 {
			m.Resolution = database.DBConnect.GetresolutionsIn[idx].Name
		}
	}

	if m.QualityID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetqualitiesIn, 2); idx != -1 {
			m.Quality = database.DBConnect.GetqualitiesIn[idx].Name
		}
	}

	if m.AudioID != 0 {
		if idx := m.Getqualityidxbyid(database.DBConnect.GetaudiosIn, 3); idx != -1 {
			m.Audio = database.DBConnect.GetaudiosIn[idx].Name
		}
	}

	if m.CodecID == 0 {
		return
	}

	if idx := m.Getqualityidxbyid(database.DBConnect.GetcodecsIn, 4); idx != -1 {
		m.Codec = database.DBConnect.GetcodecsIn[idx].Name
	}
}

// findPriorityValue returns the priority for the given combination, first
// checking the wanted table when checkwanted is true and falling back to the
// full table. Both lookups hit the same immutable snapshot, so the result is
// consistent even when the tables are regenerated concurrently (config reload).
func findPriorityValue(
	reso, qual, codec, aud, audioformat uint,
	quality *config.QualityConfig,
	checkwanted bool,
) (int, bool) {
	s := loadPrioSnapshot()

	prio, inAll, inWanted := s.lookupPriority(quality.Name, reso, qual, codec, aud, audioformat)
	if checkwanted && inWanted {
		return prio, true
	}

	if inAll {
		return prio, true
	}

	return 0, false
}

// FindPriorityValue returns the priority for the given combination, checking
// the wanted table first and falling back to the full table. Both lookups hit
// the same immutable snapshot, making this safe against concurrent table
// regeneration - prefer it over the Findpriorityidx*/Get*ArrPrio pairs.
func FindPriorityValue(
	reso, qual, codec, aud, audioformat uint,
	quality *config.QualityConfig,
) (int, bool) {
	return findPriorityValue(reso, qual, codec, aud, audioformat, quality, true)
}

// applyPriorityModifiers adjusts the priority of a parsed media file based on specific attributes.
// It increases the priority for proper releases, extended versions, and repacks.
// The priority is incremented by 5 for proper releases, 2 for extended versions, and 1 for repacks.
func applyPriorityModifiers(m *database.ParseInfo) {
	if m.Proper {
		m.Priority += 5
	}

	if m.Extended {
		m.Priority += 2
	}

	if m.Repack {
		m.Priority++
	}
}

// GenerateAllQualityPriorities generates all possible quality priority combinations
// by iterating through resolutions, qualities, codecs and audios. It builds up
// a target Prioarr struct containing the ID and name for each, and calculates
// the priority value based on the quality group's reorder rules. The results
// are published as one immutable snapshot, so concurrent readers are never
// affected by a regeneration (e.g. on config reload).
func GenerateAllQualityPriorities() {
	// Serialize concurrent generation; readers are lock-free via the snapshot.
	prioritiesMu.Lock()
	defer prioritiesMu.Unlock()

	regex0 := database.Qualities{Name: "", ID: 0, Priority: 0}
	// Clone before appending the zero sentinel - appending to the global
	// slices directly can write into their shared backing arrays.
	getresolutions := append(slices.Clone(database.DBConnect.GetresolutionsIn), regex0)
	getqualities := append(slices.Clone(database.DBConnect.GetqualitiesIn), regex0)
	getaudios := append(slices.Clone(database.DBConnect.GetaudiosIn), regex0)
	getcodecs := append(slices.Clone(database.DBConnect.GetcodecsIn), regex0)
	getaudioformats := append(slices.Clone(database.DBConnect.GetaudioformatsIn), regex0)

	snap := &prioritySnapshot{
		byName:   make(map[string]*profilePrio, config.GetSettingsQualityLen()),
		profiles: make([]*profilePrio, 0, config.GetSettingsQualityLen()),
	}

	config.RangeSettingsQuality(func(_ string, qual *config.QualityConfig) {
		p := &profilePrio{
			name:     qual.Name,
			resoQual: make(map[[2]uint]resoQualEntry, len(getresolutions)*len(getqualities)),
			codec:    make(map[uint]int, len(getcodecs)),
			audio:    make(map[uint]int, len(getaudios)),
			audioFmt: make(map[uint]int, len(getaudioformats)),
		}

		// Codec / audio / audio-format priorities depend only on the profile and
		// the component, so compute them once per dimension (not per combination).
		for _, codec := range getcodecs {
			prio := codec.Gettypeidprioritysingle("codec", qual)

			p.codec[codec.ID] = prio
			p.codecOrder = append(p.codecOrder, dimVal{id: codec.ID, prio: prio})
		}

		for _, audio := range getaudios {
			prio := audio.Gettypeidprioritysingle("audio", qual)

			p.audio[audio.ID] = prio
			p.audioOrder = append(p.audioOrder, dimVal{id: audio.ID, prio: prio})
		}

		for _, af := range getaudioformats {
			prio := af.Gettypeidprioritysingle("audio_format", qual)

			p.audioFmt[af.ID] = prio
			p.afOrder = append(p.afOrder, dimVal{id: af.ID, prio: prio})
		}

		// Resolution+quality priority (after combined reorder) and wanted-ness
		// depend on the (resolution, quality) pair.
		for _, reso := range getresolutions {
			prioresoorg := reso.Gettypeidprioritysingle("resolution", qual)

			for _, quality := range getqualities {
				prioqualorg := quality.Gettypeidprioritysingle("quality", qual)

				prioreso, prioqual := handleCombinedReorder(
					qual,
					reso.Name,
					quality.Name,
					prioresoorg,
					prioqualorg,
				)

				pairWanted := isWantedCombination(qual, reso.Name, quality.Name)
				if !pairWanted && database.DBLogLevel == logger.StrDebug {
					logger.Logtype("debug", 3).
						Str("quality_group", qual.Name).
						Str("resolution", reso.Name).
						Str("quality", quality.Name).
						Msg("Combination excluded from wanted priorities")
				}

				key := [2]uint{reso.ID, quality.ID}

				p.resoQual[key] = resoQualEntry{prio: prioreso + prioqual, wanted: pairWanted}
				p.resoQualOrder = append(p.resoQualOrder, key)
			}
		}

		snap.profiles = append(snap.profiles, p)
		snap.byName[strings.ToLower(qual.Name)] = p
	})

	prioSnapshot.Store(snap)
}

// handleCombinedReorder processes quality reordering for combined resolution and quality configurations.
// It checks if a specific resolution and quality combination matches a reordering rule and returns
// adjusted priority values. If no matching rule is found, it returns the original priority values.
// The function supports case-insensitive matching and handles combined resolution-quality reordering.
func handleCombinedReorder(
	qual *config.QualityConfig,
	resolutionName, qualityName string,
	prioresoorg, prioqualorg int,
) (int, int) {
	for idx := range qual.QualityReorder {
		reorder := &qual.QualityReorder[idx]
		if reorder.ReorderType != "combined_res_qual" &&
			!strings.EqualFold(reorder.ReorderType, "combined_res_qual") {
			continue
		}

		if !strings.ContainsRune(reorder.Name, ',') {
			continue
		}

		commaIdx := strings.IndexRune(reorder.Name, ',')
		if commaIdx == -1 {
			continue
		}

		reorderRes := reorder.Name[:commaIdx]
		reorderQual := reorder.Name[commaIdx+1:]

		if strings.EqualFold(reorderRes, resolutionName) &&
			strings.EqualFold(reorderQual, qualityName) {
			return reorder.Newpriority, 0
		}
	}

	return prioresoorg, prioqualorg
}

// isWantedCombination checks if a specific resolution and quality combination is
// desired based on the quality configuration. An empty name (the zero/unknown
// sentinel) or an empty wanted list passes - filtering only applies to named
// values against a configured list. Previously this check only ran when debug
// logging was enabled, which made the wanted table contents depend on the log
// level; it is now always applied.
func isWantedCombination(qual *config.QualityConfig, resolutionName, qualityName string) bool {
	if resolutionName != "" && len(qual.WantedResolution) > 0 &&
		!logger.SlicesContainsI(qual.WantedResolution, resolutionName) {
		return false
	}

	if qualityName != "" && len(qual.WantedQuality) > 0 &&
		!logger.SlicesContainsI(qual.WantedQuality, qualityName) {
		return false
	}

	return true
}

// BuildPrioStr builds a priority string from the given resolution, quality, codec, and audio values.
// The priority string is in the format "r_q_c_a" where r is the resolution, q is the quality, c is the codec,
// and a is the audio value. This allows easy comparison of release priority.
func BuildPrioStr(r, q, c, a, af uint) string {
	bld := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(bld)

	bld.WriteUInt(r)
	bld.WriteByte('_')
	bld.WriteUInt(q)
	bld.WriteByte('_')
	bld.WriteUInt(c)
	bld.WriteByte('_')
	bld.WriteUInt(a)
	bld.WriteByte('_')
	bld.WriteUInt(af)

	return bld.String()
}

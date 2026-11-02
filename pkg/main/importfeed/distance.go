package importfeed

import (
	"maps"
	"math"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/pool"
	"golang.org/x/text/unicode/norm"
)

// Recommendation thresholds mirror beets' strong_rec_thresh / medium_rec_thresh /
// rec_gap_thresh from config_default.yaml.
//
//	strong_rec_thresh  0.04  — distance strictly below this → recStrong
//	medium_rec_thresh  0.25  — distance at or below this → at least recMedium
//	rec_gap_thresh     0.25  — gap to 2nd-best at least this → upgrade to recStrong
//
// mediumRecThresh is intentionally set slightly higher (0.30) than beets' default
// to be more permissive in automated (non-interactive) use.
const (
	strongRecThresh = 0.04
	mediumRecThresh = 0.30
	// recGapThresh mirrors beets' rec_gap_thresh. Unlike the strong/medium
	// thresholds it is deliberately not configurable: it only decides whether
	// a clear lead over the runner-up upgrades the recommendation.
	recGapThresh = 0.25
)

// recThresholds returns the strong/medium recommendation thresholds, taking the
// data config's StrongRecThresh / MediumRecThresh overrides (0 = default).
func recThresholds(data *config.MediaDataConfig) (strong, medium float64) {
	strong, medium = strongRecThresh, mediumRecThresh
	if data != nil {
		if data.StrongRecThresh > 0 {
			strong = data.StrongRecThresh
		}

		if data.MediumRecThresh > 0 {
			medium = data.MediumRecThresh
		}
	}

	return strong, medium
}

// albumRecommendation mirrors beets' Recommendation enum.
type albumRecommendation int

const (
	recNone   albumRecommendation = 0
	recLow    albumRecommendation = 1
	recMedium albumRecommendation = 2
	recStrong albumRecommendation = 3
)

// ---------------------------------------------------------------------------
// Match report types — diagnostics for rejected folder moves
// ---------------------------------------------------------------------------

// CandidateTrackReport holds per-track data for one candidate in a match report.
type CandidateTrackReport struct {
	Title          string
	TrackNumber    int
	DiscNumber     int
	DBRuntimeMS    int64
	LocalRuntimeMS int64 // for Unmatched: runtime of the closest local candidate found
	TrackDist      float64
	Unmatched      bool // DB track had no acceptable local match; LocalRuntimeMS/TrackDist hold the closest local candidate
	LocalOnly      bool // local file with no DB equivalent (surplus track)
}

// CandidateReport holds match data for one candidate in a match report.
type CandidateReport struct {
	Title             string
	Artist            string
	MBID              string
	Year              int
	ExpectedTracks    int
	ExpectedRuntimeMS int
	AlbumDist         float64
	FullDist          float64
	Tracks            []CandidateTrackReport
}

// MatchReport captures the top-N candidates evaluated when a folder cannot be matched.
type MatchReport struct {
	DenialReason    string
	ActualTracks    int
	ActualRuntimeMS int64
	Candidates      []CandidateReport
}

// ---------------------------------------------------------------------------
// Beets-faithful distance calculation
// Ported from https://github.com/beetbox/beets/blob/master/beets/autotag/distance.py
// ---------------------------------------------------------------------------

var (
	// ampReplacer replaces & with "and" in one allocation-free scan when & is absent.
	ampReplacer = strings.NewReplacer("&", "and")

	// unicodeReplacer maps accented Latin characters and German umlauts to their ASCII
	// equivalents before Levenshtein comparison, so "Müller"≈"Mueller", "étude"≈"etude".
	unicodeReplacer = strings.NewReplacer(
		// German umlauts and sharp-s
		"ä", "ae", "Ä", "ae",
		"ö", "oe", "Ö", "oe",
		"ü", "ue", "Ü", "ue",
		"ß", "ss", "ẞ", "ss",
		// Ligatures
		"æ", "ae", "Æ", "ae",
		"œ", "oe", "Œ", "oe",
		// a-group
		"à", "a", "á", "a", "â", "a", "ã", "a", "å", "a",
		"À", "a", "Á", "a", "Â", "a", "Ã", "a", "Å", "a",
		// e-group
		"è", "e", "é", "e", "ê", "e", "ë", "e",
		"È", "e", "É", "e", "Ê", "e", "Ë", "e",
		// i-group
		"ì", "i", "í", "i", "î", "i", "ï", "i",
		"Ì", "i", "Í", "i", "Î", "i", "Ï", "i",
		// o-group
		"ò", "o", "ó", "o", "ô", "o", "õ", "o", "ø", "o",
		"Ò", "o", "Ó", "o", "Ô", "o", "Õ", "o", "Ø", "o",
		// u-group
		"ù", "u", "ú", "u", "û", "u",
		"Ù", "u", "Ú", "u", "Û", "u",
		// y, c-cedilla, n-tilde
		"ý", "y", "Ý", "y",
		"ç", "c", "Ç", "c",
		"ñ", "n", "Ñ", "n",
	)

	// numParenRe matches parenthetical groups that start with '#' or a digit,
	// e.g. "(#1)", "(4:46)", "(2:30)". Used to strip track numbers and time codes
	// before the substring-containment check in stringDist.
	numParenRe = regexp.MustCompile(`\([#\d][^)]*\)`)

	// vaArtistTokens lists lowercase tokens that indicate "various artists".
	// Used by ScoreTrackDistance so the local vaArtists map isn't re-allocated per call.
	vaArtistTokens = []string{"", "various artists", "various", "va", "unknown"}

	// beetsWeights mirrors beets' default distance_weights config (music).
	beetsWeights = map[string]float64{
		"artist": 3.0,
		"album":  3.0,
		"year":   1.0,
		// "country": 0.5, // temporarily disabled
		// "label":   0.5, // temporarily disabled
		// "mediums":          1.0, // not used — no disc-format comparison implemented
		// "media":            1.0, // not used — no media-type comparison implemented
		// "catalognum":       0.5, // not used — no catalogue-number comparison implemented
		// "albumdisambig":    0.5, // not used — no disambiguation-comment comparison implemented
		"tracks":           3.0, // per-matched-track-pair distance (Hungarian-assigned)
		"album_id":         5.0,
		"missing_tracks":   0.9,
		"unmatched_tracks": 0.6,
		// track-level defaults
		"track_title":  3.0,
		"track_artist": 2.0,
		"track_index":  1.0,
		"track_length": 3.0,
		"track_id":     5.0,
	}

	// audiobookTrackWeights overrides track-level weights for audiobooks.
	// Audiobooks have no reliable title matching, so track_index and track_length
	// are boosted; track_title is reduced.
	audiobookTrackWeights = map[string]float64{
		"track_title":  1.0,
		"track_artist": 2.0,
		"track_index":  3.0,
		"track_length": 3.0,
		"track_id":     5.0,
	}
	sdEndWordList     = [3]string{"the", "a", "an"}
	sdEndWordSuffixes = [3]string{", the", ", a", ", an"}

	// sdPatterns mirrors beets SD_PATTERNS: (compiled regex, weight).
	// Matched portions are removed and the improvement is credited at the given weight.
	sdPatterns = []struct {
		re     *regexp.Regexp
		weight float64
	}{
		{regexp.MustCompile(`(?i)^the `), 0.1},
		{regexp.MustCompile(`(?i)^an? `), 0.1}, // "A " / "An " article prefix
		{regexp.MustCompile(`(?i)[\[\(]?(ep|single)[\]\)]?`), 0.0},
		{regexp.MustCompile(`(?i)[\[\(]?(featuring|feat|ft)[\. :].+`), 0.1},
		{regexp.MustCompile(`\(.*?\)`), 0.3},
		{regexp.MustCompile(`\[.*?\]`), 0.3},
		{regexp.MustCompile(`(?i)(, )?(pt\.|part) .+`), 0.2},
	}

	// levBufPool recycles the prev+curr buffer across levenshteinInt calls.
	levBufPool = pool.NewPool(200, 0, nil, func(*levBuf) bool { return false })
)

// sdNormalize lowercases and strips everything that is not a letter or digit.
// Letters and digits of every script are kept; Latin diacritics are folded
// (NFD + drop combining marks on Latin letters) so "Dvořák" → "dvorak".
// Fast-path: returns s unchanged (zero allocation) when s is already all
// lowercase ASCII alnum; pure-ASCII input is handled byte-wise without NFD.
func sdNormalize(s string) string {
	i := 0
	for i < len(s) {
		b := s[i]
		if (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') {
			i++
			continue
		}

		break
	}

	if i == len(s) {
		return s // already lowercase alnum — zero allocation
	}

	ascii := true
	for j := i; j < len(s); j++ {
		if s[j] >= utf8.RuneSelf {
			ascii = false
			break
		}
	}

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	buf.Grow(len(s))
	buf.WriteString(s[:i])

	if ascii {
		for ; i < len(s); i++ {
			b := s[i]
			switch {
			case b >= 'a' && b <= 'z', b >= '0' && b <= '9':
				buf.WriteByte(b)
			case b >= 'A' && b <= 'Z':
				buf.WriteByte(b + 32) // ASCII uppercase → lowercase
			}
		}

		return buf.String()
	}

	// Only marks on Latin letters are folded away; in other scripts they are
	// part of the letter (Cyrillic й = и + breve) and get recomposed by NFC.
	foldMarks := true

	for _, r := range norm.NFD.String(s[i:]) {
		switch {
		case unicode.Is(unicode.Mn, r):
			if !foldMarks {
				buf.WriteRune(r)
			}
		case unicode.IsLetter(r), unicode.IsDigit(r):
			buf.WriteRune(unicode.ToLower(r))

			foldMarks = r < utf8.RuneSelf || unicode.Is(unicode.Latin, r)
		default:
			foldMarks = true
		}
	}

	return norm.NFC.String(buf.String())
}

// penaltyEntry accumulates a named penalty: sum of values and count of entries.
// Using sum+count avoids allocating a []float64 per key.
type penaltyEntry struct {
	sum   float64
	count int
}

// beetsDistance mirrors beets' Distance class.
// It accumulates named penalties and normalises them by their combined weight budget.
type beetsDistance struct {
	penalties      map[string]penaltyEntry
	weightOverride map[string]float64 // nil = use beetsWeights global
}

func newBeetsDistance() *beetsDistance {
	return &beetsDistance{penalties: make(map[string]penaltyEntry)}
}

func (d *beetsDistance) weight(key string) float64 {
	if d.weightOverride != nil {
		if w, ok := d.weightOverride[key]; ok {
			return w
		}
	}

	return beetsWeights[key]
}

func (d *beetsDistance) add(key string, dist float64) {
	e := d.penalties[key]

	e.sum += dist
	e.count++

	d.penalties[key] = e
}

func (d *beetsDistance) addString(key, s1, s2 string) {
	d.add(key, stringDist(s1, s2))
}

// addRatio mirrors beets Distance.add_ratio: clamps number1 to [0, number2] then divides.
func (d *beetsDistance) addRatio(key string, number1, number2 float64) {
	n := math.Max(math.Min(number1, number2), 0)

	var dist float64
	if number2 > 0 {
		dist = n / number2
	}

	d.add(key, dist)
}

func (d *beetsDistance) rawDistance() float64 {
	var raw float64
	for key, e := range d.penalties {
		raw += e.sum * d.weight(key)
	}

	return raw
}

func (d *beetsDistance) maxDistance() float64 {
	var total float64
	for key, e := range d.penalties {
		total += float64(e.count) * d.weight(key)
	}

	return total
}

func (d *beetsDistance) distance() float64 {
	maxDist := d.maxDistance()
	if maxDist == 0 {
		return 0
	}

	return d.rawDistance() / maxDist
}

// levBuf wraps the two-row buffer used by levenshteinInt so it can be pooled.
type levBuf struct {
	data []int
}

// levenshteinInt returns the raw Levenshtein edit distance between two strings,
// counted in runes. Pure-ASCII input stays on the allocation-free byte path.
func levenshteinInt(a, b string) int {
	if !isASCII(a) || !isASCII(b) {
		return levenshteinRunes([]rune(a), []rune(b))
	}

	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}

	if lb == 0 {
		return la
	}

	// Allocate prev and curr as one contiguous buffer and reuse across calls.
	need := 2 * (lb + 1)

	buf := levBufPool.Get()
	if cap(buf.data) < need {
		buf.data = make([]int, need)
	} else {
		buf.data = buf.data[:need]
	}

	defer levBufPool.Put(buf)

	prev := buf.data[:lb+1]
	curr := buf.data[lb+1 : need]

	for j := range prev {
		prev[j] = j
	}

	for i := range la {
		ca := a[i]
		curr[0] = i + 1
		for j := range lb {
			cb := b[j]
			del := prev[j+1] + 1
			ins := curr[j] + 1

			sub := prev[j]
			if ca != cb {
				sub++
			}

			curr[j+1] = min3(del, ins, sub)
		}

		prev, curr = curr, prev
	}

	return prev[lb]
}

func isASCII(s string) bool {
	for i := range len(s) {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}

	return true
}

func levenshteinRunes(a, b []rune) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}

	if lb == 0 {
		return la
	}

	prev := make([]int, lb+1)
	curr := make([]int, lb+1)

	for j := range prev {
		prev[j] = j
	}

	for i := range la {
		curr[0] = i + 1
		for j := range lb {
			sub := prev[j]
			if a[i] != b[j] {
				sub++
			}

			curr[j+1] = min3(prev[j+1]+1, curr[j]+1, sub)
		}

		prev, curr = curr, prev
	}

	return prev[lb]
}

// stringDistBasic mirrors beets _string_dist_basic:
// strips non-alphanumeric chars, lowercases, then computes normalised Levenshtein.
// When nothing alphanumeric survives on both sides there is no evidence of a
// match, so only literally equal inputs count as identical.
func stringDistBasic(str1, str2 string) float64 {
	raw1, raw2 := str1, str2
	str1 = sdNormalize(str1)

	str2 = sdNormalize(str2)
	if str1 == "" && str2 == "" {
		if strings.EqualFold(raw1, raw2) {
			return 0.0
		}

		return 1.0
	}

	maxLen := max(utf8.RuneCountInString(str2), utf8.RuneCountInString(str1))

	return float64(levenshteinInt(str1, str2)) / float64(maxLen)
}

// importMatchMaxDist is the largest stringDist at which a search hit's
// name/title is accepted as the item that was asked for.
const importMatchMaxDist = 0.3

// namesMatch reports whether two artist/author names or titles refer to the
// same thing: equal ignoring case, or within importMatchMaxDist.
func namesMatch(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	return strings.EqualFold(a, b) || stringDist(a, b) <= importMatchMaxDist
}

// namesContain reports whether any of names matches want (see namesMatch).
func namesContain(names []string, want string) bool {
	for i := range names {
		if namesMatch(names[i], want) {
			return true
		}
	}

	return false
}

// bestNameMatch returns the index of the candidate whose name (get(i)) is
// closest to want, or -1 when none is within importMatchMaxDist.
// accept, when non-nil, filters candidates before they are scored.
func bestNameMatch(want string, n int, get func(int) string, accept func(int) bool) int {
	if want == "" {
		return -1
	}

	best, bestDist := -1, importMatchMaxDist

	for i := range n {
		if accept != nil && !accept(i) {
			continue
		}

		name := get(i)
		if name == "" {
			continue
		}

		d := 0.0
		if !strings.EqualFold(name, want) {
			d = stringDist(name, want)
		}

		if d < bestDist || (best == -1 && d <= bestDist) {
			best, bestDist = i, d
		}
	}

	return best
}

// stringDist mirrors beets string_dist:
// handles "X, the"→"the X" normalisation, & replacement, and SD_PATTERNS weighting.
func stringDist(str1, str2 string) float64 {
	if str1 == "" && str2 == "" {
		return 0.0
	}

	if str1 == "" || str2 == "" {
		return 1.0
	}

	// Pre-normalize: map accented chars and German umlauts to ASCII equivalents so that
	// "Müller"≡"Mueller", "étude"≡"etude", etc. contribute zero Levenshtein distance.
	str1 = unicodeReplacer.Replace(str1)
	str2 = unicodeReplacer.Replace(str2)

	// SD_END_WORDS: "something, the" → "the something"
	for i, suffix := range sdEndWordSuffixes {
		if trimmed := logger.TrimSuffixI(str1, suffix); trimmed != str1 {
			str1 = logger.JoinStrings(sdEndWordList[i], " ", trimmed)
		}

		if trimmed := logger.TrimSuffixI(str2, suffix); trimmed != str2 {
			str2 = logger.JoinStrings(sdEndWordList[i], " ", trimmed)
		}
	}

	// SD_REPLACE: & → and
	str1 = ampReplacer.Replace(str1)
	str2 = ampReplacer.Replace(str2)

	// SD_PATTERNS: each match is credited at its weight rather than full distance.
	baseDist := stringDistBasic(str1, str2)

	penalty := 0.0
	for i := range sdPatterns {
		match1 := sdPatterns[i].re.MatchString(str1)

		match2 := sdPatterns[i].re.MatchString(str2)
		if !match1 && !match2 {
			continue
		}

		case1, case2 := str1, str2
		if match1 {
			case1 = sdPatterns[i].re.ReplaceAllLiteralString(str1, "")
		}

		if match2 {
			case2 = sdPatterns[i].re.ReplaceAllLiteralString(str2, "")
		}

		caseDist := stringDistBasic(case1, case2)

		caseDelta := math.Max(0.0, baseDist-caseDist)
		if caseDelta == 0.0 {
			continue
		}

		str1, str2 = case1, case2
		baseDist = caseDist

		penalty += sdPatterns[i].weight * caseDelta
	}

	// Optional: subtitle/movement containment check.
	// If one title (with track-number and time-code parentheticals stripped) is a
	// prefix or suffix of the other, cap the distance at 0.3 × extra-content fraction.
	// Example: "Preludio (Largo)" is a suffix of "Sonata in E minor - Preludio (Largo)".
	if reg := baseDist + penalty; reg > 0 {
		s1 := sdNormalize(numParenRe.ReplaceAllLiteralString(str1, ""))
		s2 := sdNormalize(numParenRe.ReplaceAllLiteralString(str2, ""))

		shortS, longS := s1, s2
		if len(shortS) > len(longS) {
			shortS, longS = longS, shortS
		}

		if len(shortS) >= 4 &&
			(strings.HasPrefix(longS, shortS) || strings.HasSuffix(longS, shortS)) {
			if sub := 0.3 * float64(len(longS)-len(shortS)) / float64(len(longS)); sub < reg {
				return sub
			}
		}
	}

	return baseDist + penalty
}

// ---------------------------------------------------------------------------
// Beets-faithful per-track distance and bipartite track matching
// ---------------------------------------------------------------------------

// trackLengthGraceMs and trackLengthMaxMs mirror beets' track_length_grace /
// track_length_max config defaults (10 s and 30 s respectively).
const (
	trackLengthGraceMs = int64(10_000)
	trackLengthMaxMs   = int64(30_000)
)

// trackDistance mirrors beets track_distance().
// It scores how well a local file matches a database track using:
//
//	track_length  (weight 3.0)
//	track_title   (weight 3.0 music / 1.0 audiobook)
//	track_artist  (weight 2.0) — only for VA albums
//	track_index   (weight 1.0 music / 3.0 audiobook)
//	track_id      (weight 5.0) — skipped if local has no MB recording ID
//
// Returns [0..1]; lower is better.
func trackDistance(
	local *parser_v2.TrackInfo,
	db *database.DbtrackWithArtist,
	isVA, isAudiobook bool,
	data *config.MediaDataConfig,
) float64 {
	// Derive runtime bounds from config; fall back to beets defaults.
	// When PerTrackToleranceSeconds > 0:
	//   graceMs = PerTrackToleranceSeconds * 1000  (diff ≤ grace → no penalty)
	//   hardMs  = PerTrackToleranceSecondsMax * 1000, if > 0 (diff > hard → hard reject)
	//           = graceMs, if PerTrackToleranceSecondsMax == 0 (grace is the hard limit)
	graceMs := trackLengthGraceMs
	hardMs := trackLengthMaxMs

	strictMode := data != nil && data.PerTrackToleranceSeconds > 0
	if strictMode {
		graceMs = int64(data.PerTrackToleranceSeconds) * 1000
		if data.PerTrackToleranceSecondsMax > 0 &&
			data.PerTrackToleranceSecondsMax > data.PerTrackToleranceSeconds {
			hardMs = int64(data.PerTrackToleranceSecondsMax) * 1000
		} else {
			hardMs = graceMs // grace == hard → any excess is a hard reject
		}
	}

	// Build weight override map: start from audiobook defaults (if applicable),
	// then apply any per-config field overrides on top.
	var weightOverride map[string]float64
	if isAudiobook {
		weightOverride = make(map[string]float64, len(audiobookTrackWeights))
		maps.Copy(weightOverride, audiobookTrackWeights)
	}

	if data != nil {
		apply := func(key string, v float64) {
			if v == 0 {
				return
			}

			if weightOverride == nil {
				weightOverride = make(map[string]float64)
			}

			weightOverride[key] = v
		}
		apply("track_title", data.TrackTitleWeight)
		apply("track_index", data.TrackIndexWeight)
		apply("track_length", data.TrackLengthWeight)
		apply("track_artist", data.TrackArtistWeight)
		apply("track_id", data.TrackIdWeight)
	}

	dist := &beetsDistance{
		penalties:      make(map[string]penaltyEntry),
		weightOverride: weightOverride,
	}

	// track_length
	if db.RuntimeMs > 0 {
		diff := local.RuntimeMS - db.RuntimeMs
		if diff < 0 {
			diff = -diff
		}

		if strictMode {
			// Strict two-tier mode:
			//   diff ≤ graceMs → no penalty
			//   graceMs < diff ≤ hardMs → graduated penalty (diff−grace)/(hard−grace)
			//   diff > hardMs → hard reject (excluded from matching entirely)
			if diff > hardMs {
				return 1.0
			}

			window := hardMs - graceMs
			if window <= 0 || diff <= graceMs {
				dist.add("track_length", 0.0)
			} else {
				dist.addRatio("track_length", float64(diff-graceMs), float64(window))
			}
		} else {
			// Beets default: 10 s grace window, 30 s max penalty window.
			penaltyMs := float64(diff - graceMs)
			dist.addRatio("track_length", penaltyMs, float64(hardMs))
		}
	}

	// track_title
	dist.addString("track_title", local.Title, db.Title)

	// track_artist — beets only adds this for VA releases when the DB track has
	// an artist and the local artist is not one of the "various artists" tokens.
	if isVA && db.Artist != "" &&
		!logger.SlicesContainsI(vaArtistTokens, strings.TrimSpace(local.Artist)) {
		dist.addString("track_artist", local.Artist, db.Artist)
	}

	// track_index — graduated penalty mirroring beets: abs(diff) / max(local#, db#).
	// Binary 0/1 made off-by-one mismatches (e.g. editions with an extra bonus track)
	// cost as much as a completely wrong position, causing the algorithm to prefer
	// track-number-matching assignments even when runtimes were clearly worse.
	if db.TrackNumber > 0 && local.TrackNumber > 0 {
		dbN := int(db.TrackNumber)
		localN := local.TrackNumber

		diff := dbN - localN
		if diff < 0 {
			diff = -diff
		}

		denom := max(localN, dbN)
		dist.add("track_index", float64(diff)/float64(denom))
	}

	// track_id — only when local file carries a MusicBrainz recording ID
	if local.MusicBrainzRecordingID != "" && db.MusicbrainzRecordingID != "" {
		if local.MusicBrainzRecordingID == db.MusicbrainzRecordingID {
			dist.add("track_id", 0.0)
		} else {
			dist.add("track_id", 1.0)
		}
	}

	return dist.distance()
}

// singleFileTotalMatch reports whether one local file holding a whole book
// matches a listing of chapters by total runtime. The allowed difference is
// MaxTotalDifferenceSeconds when set, otherwise the per-track hard limit (as in
// trackDistance) times the chapter count — the slack the chapters would have
// had as separate files. A listing without runtimes never matches.
func singleFileTotalMatch(
	localMs, chaptersMs int64,
	chapters int,
	data *config.MediaDataConfig,
) (ok bool, diffMs, toleranceMs int64) {
	if chaptersMs <= 0 || localMs <= 0 || chapters <= 0 {
		return false, 0, 0
	}

	perTrackMs := trackLengthMaxMs
	if data != nil && data.PerTrackToleranceSeconds > 0 {
		perTrackMs = int64(data.PerTrackToleranceSeconds) * 1000
		if data.PerTrackToleranceSecondsMax > data.PerTrackToleranceSeconds {
			perTrackMs = int64(data.PerTrackToleranceSecondsMax) * 1000
		}
	}

	toleranceMs = int64(chapters) * perTrackMs
	if data != nil && data.MaxTotalDifferenceSeconds > 0 {
		toleranceMs = int64(data.MaxTotalDifferenceSeconds) * 1000
	}

	diffMs = localMs - chaptersMs
	if diffMs < 0 {
		diffMs = -diffMs
	}

	return diffMs <= toleranceMs, diffMs, toleranceMs
}

// globaliseDiscTrackNumbers returns a copy of dbTracks where per-disc track
// numbers are replaced by global sequential positions (1, 2, 3, …) when the
// DB release spans multiple discs but the local tracks carry no disc info.
//
// This eliminates the spurious track_index penalty that arises when the DB
// stores, say, disc-2 track 1 and the local file is sequentially numbered 12.
// Without this the LAP still assigns the pair correctly (runtime+title win), but
// the large index delta inflates the full distance and can push the candidate
// above the acceptance threshold.
//
// Condition for normalisation:
//   - at least one DB track has DiscNumber > 1 (multi-disc release in DB), AND
//   - every local track has DiscNumber ≤ 1 (flat sequential file numbering).
//
// When the condition is not met, the original slice is returned as-is.
func globaliseDiscTrackNumbers(
	dbTracks []database.DbtrackWithArtist,
	localTracks []parser_v2.TrackInfo,
) []database.DbtrackWithArtist {
	multiDisc := false
	for i := range dbTracks {
		if dbTracks[i].DiscNumber > 1 {
			multiDisc = true
			break
		}
	}

	if !multiDisc {
		return dbTracks
	}

	for i := range localTracks {
		if localTracks[i].DiscNumber > 1 {
			return dbTracks // local also has disc info — no normalisation needed
		}
	}

	// DB tracks are ordered by (disc_number, track_number) so position i+1 is
	// the correct global sequential index.
	out := make([]database.DbtrackWithArtist, len(dbTracks))
	copy(out, dbTracks)

	for i := range out {
		out[i].TrackNumber = uint16(i + 1)
	}

	return out
}

// matchTracksByDistance assigns local tracks to database track positions using
// beets-style track_distance scoring and Kuhn's augmenting-path bipartite matching.
//
// Unlike matchTracksByRuntimeProgressive (which uses only runtime within a tolerance),
// this function uses the full track_distance — combining runtime, title, track index,
// and optionally MB recording ID — so it degrades gracefully when one signal is absent.
//
// Candidate edges are built for every local×db pair whose distance is below
// maxTrackDist (default 0.9), sorted ascending so Kuhn's algorithm naturally
// prefers the closest match.
//
// Returns:
//   - result[i]    — local TrackInfo assigned to db position i (zero value if unmatched)
//   - matched[i]   — true when db position i was filled
//   - used[j]      — true when local track j was consumed
func matchTracksByDistance(
	tracks []parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	isVA, isAudiobook bool,
	data *config.MediaDataConfig,
) ([]parser_v2.TrackInfo, []bool, []bool) {
	const maxTrackDist = 0.9

	// Normalise multi-disc DB track numbers to global sequential positions when
	// local files use flat sequential numbering (no disc info). This keeps
	// track_index meaningful: disc-2 track 1 becomes global position 12 and
	// compares correctly against local track 12, rather than producing a
	// spurious diff of 11.
	dbTracks = globaliseDiscTrackNumbers(dbTracks, tracks)

	n := len(dbTracks)
	m := len(tracks)
	result := make([]parser_v2.TrackInfo, n)
	matched := make([]bool, n)
	used := make([]bool, m)

	if n == 0 || m == 0 {
		return result, matched, used
	}

	// Build padded square cost matrix for Hungarian assignment.
	// Rows = db tracks (0..n-1), cols = local tracks (0..m-1).
	// Padding entries use dummyCost so the algorithm treats them as unmatched.
	size := max(m, n)

	const dummyCost = 1.0

	// Single contiguous allocation; row headers slice into it — 2 allocs instead of size+1.
	backing := make([]float64, size*size)

	cost := make([][]float64, size)
	for i := range cost {
		row := backing[i*size : (i+1)*size]

		cost[i] = row
		if i < n {
			for j := range size {
				if j < m {
					row[j] = trackDistance(&tracks[j], &dbTracks[i], isVA, isAudiobook, data)
				} else {
					row[j] = dummyCost
				}
			}
		} else {
			for j := range row {
				row[j] = dummyCost
			}
		}
	}

	// Audiobook files that already carry a playback order keep it: the
	// assignment may skip chapters or files, but never reorders them. Chapters
	// of near-identical length would otherwise be swapped by the unconstrained
	// assignment, and the rename then scrambles the book.
	var rowAssign []int
	if order := audiobookLocalOrder(tracks, isAudiobook); order != nil {
		rowAssign = alignOrdered(cost, n, order, maxTrackDist)
	} else {
		// Hungarian algorithm: minimum-cost assignment (O(n³)).
		rowAssign = lapAssign(cost)
	}

	// Convert assignment to result/matched/used; ignore virtual (padded) pairs.
	for i := range n {
		j := rowAssign[i]
		if j >= 0 && j < m && cost[i][j] < maxTrackDist {
			matched[i] = true
			used[j] = true
			result[i] = tracks[j]
		}
	}

	// Post-match quality check — wrong-edition detection (music albums only).
	//
	// Problem: minimum-cost assignment can still produce a fully-assigned but wrong-
	// edition result when runtimes are very similar across tracks (e.g. Sanctuary
	// 196.3 s vs Running Free 197.1 s). The algorithm minimises total cost globally,
	// which can shuffle tracks into plausible-runtime slots with completely wrong titles,
	// suppressing searchAndImportAlternativeRelease.
	//
	// Fix: after matching, count pairs where both sides carry a non-empty title but
	// stringDist > 0.7. Even one such pair signals a wrong-edition shuffle; those
	// pairs are unset so the caller sees unmatchedDB > 0 and triggers the alternative
	// release search. stringDist > 0.7 is already conservative (similar strings score
	// well below that), so a single mismatch is sufficient evidence.
	//
	// Why both-non-empty guard matters:
	//   - Empty local title: suppresses the check, so untagged files never falsely
	//     trigger wrong-edition detection.
	//   - All files tagged with album title: every DB track shows dist > 0.7 vs the
	//     repeated album title → alternative search triggered (correct for badly-tagged files).
	//
	// Why !isAudiobook: audiobook chapter titles in local tags are unreliable — files
	// are often tagged with the full book title rather than the chapter name, so every
	// pair would appear as a title mismatch. Audiobook matching relies on runtime +
	// track_index instead.
	if n > 4 && !isAudiobook {
		for i := range n {
			j := rowAssign[i]
			if !(j < m && cost[i][j] < maxTrackDist &&
				dbTracks[i].Title != "" && tracks[j].Title != "" &&
				stringDist(tracks[j].Title, dbTracks[i].Title) > 0.7) {
				continue
			}

			// Title mismatch detected. Distinguish a genuine wrong-edition shuffle
			// from a within-album tag-swap (local file has the correct audio but the
			// track title tag was mislabelled).
			//
			// A tag-swap looks like: local runtime ≈ DB runtime for the current
			// assignment, but the local title matches a DIFFERENT DB track whose
			// runtime is much farther away.  In that case the LAP picked the right
			// file by runtime — keep the match.
			//
			// A wrong-edition shuffle looks like: the DB track whose title matches
			// the local tag has an equally good (or better) runtime match, meaning
			// LAP plausibly assigned the wrong-edition track here.
			isTagSwap := false
			localRt := tracks[j].RuntimeMS

			var currentDiff int64
			if d := localRt - dbTracks[i].RuntimeMs; d < 0 {
				currentDiff = -d
			} else {
				currentDiff = d
			}

			for i2 := range n {
				if i2 == i {
					continue
				}

				if stringDist(tracks[j].Title, dbTracks[i2].Title) >= 0.3 {
					continue
				}

				// tracks[j].Title closely matches dbTracks[i2].Title.
				// If the current assignment beats i2 by runtime, it's a tag-swap.
				var altDiff int64
				if d := localRt - dbTracks[i2].RuntimeMs; d < 0 {
					altDiff = -d
				} else {
					altDiff = d
				}

				if currentDiff < altDiff {
					isTagSwap = true
					break
				}
			}

			if !isTagSwap {
				matched[i] = false
				used[j] = false
				result[i] = parser_v2.TrackInfo{}
			}
		}
	}

	return result, matched, used
}

// audiobookLocalOrder returns the local track indices in playback order —
// sorted by (disc, track) — when isAudiobook is set and every track carries a
// distinct track number. Returns nil when the order is unknown, leaving the
// unconstrained assignment to decide.
func audiobookLocalOrder(tracks []parser_v2.TrackInfo, isAudiobook bool) []int {
	if !isAudiobook || len(tracks) < 2 {
		return nil
	}

	seen := make(map[[2]int]struct{}, len(tracks))
	for i := range tracks {
		if tracks[i].TrackNumber <= 0 {
			return nil
		}

		key := [2]int{tracks[i].DiscNumber, tracks[i].TrackNumber}
		if _, dup := seen[key]; dup {
			return nil
		}

		seen[key] = struct{}{}
	}

	order := make([]int, len(tracks))
	for i := range order {
		order[i] = i
	}

	sort.SliceStable(order, func(a, b int) bool {
		ta, tb := &tracks[order[a]], &tracks[order[b]]
		if ta.DiscNumber != tb.DiscNumber {
			return ta.DiscNumber < tb.DiscNumber
		}

		return ta.TrackNumber < tb.TrackNumber
	})

	return order
}

// alignOrdered is the order-preserving counterpart of lapAssign: a minimum-cost
// monotone alignment of DB rows 0..n-1 against the local tracks visited in
// order. A pair can only be matched when its cost is below maxDist; every
// skipped row or track costs 1.0, the same as a dummy pairing in lapAssign.
// cost is indexed [dbRow][localIndex]. Returns rowAssign with -1 for rows left
// unmatched.
func alignOrdered(cost [][]float64, n int, order []int, maxDist float64) []int {
	const (
		skipCost = 1.0
		stepDiag = 0
		stepDB   = 1
		stepLoc  = 2
	)

	m := len(order)
	w := m + 1

	dp := make([]float64, (n+1)*w)
	step := make([]uint8, (n+1)*w)

	for i := 1; i <= n; i++ {
		dp[i*w] = float64(i) * skipCost
		step[i*w] = stepDB
	}

	for k := 1; k <= m; k++ {
		dp[k] = float64(k) * skipCost
		step[k] = stepLoc
	}

	for i := 1; i <= n; i++ {
		for k := 1; k <= m; k++ {
			best, how := dp[(i-1)*w+k]+skipCost, uint8(stepDB)
			if v := dp[i*w+k-1] + skipCost; v < best {
				best, how = v, stepLoc
			}

			if c := cost[i-1][order[k-1]]; c < maxDist {
				if v := dp[(i-1)*w+k-1] + c; v <= best {
					best, how = v, stepDiag
				}
			}

			dp[i*w+k] = best
			step[i*w+k] = how
		}
	}

	rowAssign := make([]int, n)
	for i := range rowAssign {
		rowAssign[i] = -1
	}

	for i, k := n, m; i > 0 || k > 0; {
		switch step[i*w+k] {
		case stepDiag:
			rowAssign[i-1] = order[k-1]
			i--
			k--
		case stepDB:
			i--
		default:
			k--
		}
	}

	return rowAssign
}

func min3(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}

		return c
	}

	if b < c {
		return b
	}

	return c
}

// lapAssign solves the minimum-cost linear assignment problem for an n×n cost matrix
// using the O(n³) Hungarian algorithm (potential-based / Jonker-Volgenant variant).
// Returns rowAssign where rowAssign[i] = j means row i is assigned to column j.
// The matrix must be square; pad with a dummy cost before calling for rectangular inputs.
func lapAssign(cost [][]float64) []int {
	n := len(cost)
	if n == 0 {
		return nil
	}

	const inf = math.MaxFloat64 / 2
	// u[i], v[j] are row/column potentials (1-indexed; index 0 = sentinel).
	u := make([]float64, n+1)
	v := make([]float64, n+1)
	// p[j] = row assigned to column j (1-indexed; 0 = unassigned).
	p := make([]int, n+1)
	// way[j] = previous column in the augmentation path leading to j.
	way := make([]int, n+1)
	// minv and used are hoisted out of the per-row loop to avoid O(n) re-allocs.
	minv := make([]float64, n+1)

	used := make([]bool, n+1)
	for i := 1; i <= n; i++ {
		p[0] = i

		j0 := 0

		for j := range minv {
			minv[j] = inf
		}

		clear(used)

		for {
			used[j0] = true

			i0 := p[j0]
			delta := inf

			j1 := 0
			for j := 1; j <= n; j++ {
				if !used[j] {
					cur := cost[i0-1][j-1] - u[i0] - v[j]
					if cur < minv[j] {
						minv[j] = cur
						way[j] = j0
					}

					if minv[j] < delta {
						delta = minv[j]
						j1 = j
					}
				}
			}

			for j := 0; j <= n; j++ {
				if used[j] {
					u[p[j]] += delta

					v[j] -= delta
				} else {
					minv[j] -= delta
				}
			}

			j0 = j1
			if p[j0] == 0 {
				break
			}
		}

		for j0 != 0 {
			p[j0] = p[way[j0]]
			j0 = way[j0]
		}
	}

	rowAssign := make([]int, n)
	for j := 1; j <= n; j++ {
		if p[j] > 0 {
			rowAssign[p[j]-1] = j - 1
		}
	}

	return rowAssign
}

// VariousArtistsName is the canonical artist name used for VA compilations.
// Centralised here so it can be made configurable in the future.
const VariousArtistsName = "Various Artists"

// IsVariousArtists returns true when the artist name string alone indicates a
// Various Artists release. It covers common abbreviations and spellings.
func IsVariousArtists(artist string) bool {
	artist = strings.TrimSpace(artist)
	return strings.EqualFold(artist, "various artists") || strings.EqualFold(artist, "various") ||
		strings.EqualFold(artist, "va") || strings.EqualFold(artist, "v.a.") ||
		logger.ContainsI(artist, "various artists")
}

// DetectVA returns true when the artist name or local track tags indicate a
// Various Artists release.  It combines a string check on the album artist with
// a consensus check across all local track artist tags: if there are 3+ distinct
// artists and none holds more than 60 % of the tracks, the release is VA.
func DetectVA(artist string, tracks []parser_v2.TrackInfo) bool {
	if IsVariousArtists(artist) {
		return true
	}

	// Consensus check: count distinct track-level artists from file metadata.
	counts := make(map[string]int, len(tracks))
	for i := range tracks {
		a := strings.ToLower(strings.TrimSpace(tracks[i].Artist))
		if a != "" {
			counts[a]++
		}
	}

	if len(counts) >= 3 {
		// One artist holding > 60 % of tracks → not a VA release.
		for i := range counts {
			if float64(counts[i])/float64(len(tracks)) > 0.6 {
				return false
			}
		}

		return true
	}

	return false
}

// albumMatchDistance computes a beets-faithful weighted distance score [0..1]
// for an album candidate against local parsed metadata.
//
// Mirrors beets autotag/distance.py distance() using the same Distance
// accumulator, string_dist(), and penalty weights. Only components where at
// least one side has a non-empty value are added to the distance (so the
// normalisation denominator only counts what was actually compared).
//
// Weights follow beets distance_weights defaults:
//
//	artist: 3.0  album: 3.0  year: 1.0  album_id: 5.0
//	label: 0.5   country: 0.5
//	missing_tracks: 0.9 (per missing)  unmatched_tracks: 0.6 (per extra)
//
// Per-track title/length distance is omitted here because we score candidates
// before reading all local track tags; missing/unmatched counts cover the
// track-count dimension instead.
//
// Lower score = better match. 0 = perfect match.
func albumMatchDistance(
	c *database.AlbumSearchResult,
	artist, album, mbReleaseID, label, country string,
	year, trackCount int,
	data *config.MediaDataConfig,
) float64 {
	dist := newBeetsDistance()

	// Artist (weight 3.0) — always compared; returns 1.0 if either side is empty.
	// Normalise VA abbreviations so "VA"/"V.A." score 0 against "Various Artists".
	localArtist := artist
	if IsVariousArtists(localArtist) {
		localArtist = VariousArtistsName
	}

	dbArtist := c.Artist
	if IsVariousArtists(dbArtist) {
		dbArtist = VariousArtistsName
	}

	dist.addString("artist", dbArtist, localArtist)

	// Album title (weight 3.0).
	dist.addString("album", c.Title, album)

	// Album MusicBrainz release ID (weight 5.0).
	// Only scored when both sides have an MBID — avoids penalising candidates
	// that simply lack the field in the database.
	if mbReleaseID != "" && c.MusicBrainzReleaseID != "" {
		if c.MusicBrainzReleaseID == mbReleaseID {
			dist.add("album_id", 0.0)
		} else {
			dist.add("album_id", 1.0)
		}
	}

	// Year (weight 1.0) — mirrors beets' "no original_year" branch:
	// exact match → 0.0, mismatch → ratio(diff, current_year − 1889).
	if year > 0 && c.Year > 0 {
		if year == c.Year {
			dist.add("year", 0.0)
		} else {
			diff := math.Abs(float64(year - c.Year))
			diffMax := float64(time.Now().Year() - 1889)
			dist.addRatio("year", diff, diffMax)
		}
	}

	// Label (weight 0.5) — temporarily disabled; keep parameter for future re-enablement.
	_ = label

	// Country (weight 0.5) — temporarily disabled; keep parameter for future re-enablement.
	_ = country

	allowMissing := data != nil && data.AllowMissingTracks

	// Missing tracks: in the album catalogue but absent locally (weight 0.9 each).
	// Skipped when AllowMissingTracks is set — the user explicitly accepts partial albums,
	// so penalising for missing tracks would incorrectly filter out valid candidates here.
	if !allowMissing && c.TotalTracks > trackCount {
		for range c.TotalTracks - trackCount {
			dist.add("missing_tracks", 1.0)
		}
	}

	// Unmatched tracks: local files with no counterpart in the album (weight 0.6 each).
	if trackCount > c.TotalTracks {
		for range trackCount - c.TotalTracks {
			dist.add("unmatched_tracks", 1.0)
		}
	}

	return dist.distance()
}

// audiobookMatchDistance computes a beets-inspired weighted distance score [0..1]
// for an audiobook candidate. Chapter-count differences contribute via
// missing_tracks / unmatched_tracks; runtime verification does the rest.
//
// Weights: title: 3.0  author: 2.0  year: 0.5.
func audiobookMatchDistance(
	c *database.AudiobookSearchResult,
	title, author string,
	fileCount int,
) float64 {
	dist := newBeetsDistance()

	// Title (weight 3.0).
	dist.addString("album", c.Title, title)

	// Author (weight 2.0 — mapped to "artist").
	// When either side lacks an author we cannot verify it, so use a fixed
	// uncertainty penalty (0.5) rather than 0 (falsely confident) or 1 (unfairly harsh).
	if c.Author == "" || author == "" {
		dist.add("artist", 0.5)
	} else {
		dist.addString("artist", c.Author, author)
	}

	// Missing chapters: in the catalogue but absent locally (weight 0.9 each).
	// A single file holds the whole book; its total runtime is verified instead.
	if fileCount > 1 && c.ChapterCount > fileCount {
		for range c.ChapterCount - fileCount {
			dist.add("missing_tracks", 1.0)
		}
	}

	// Unmatched files: local files with no counterpart in the catalogue (weight 0.6 each).
	if fileCount > 0 && fileCount > c.ChapterCount && c.ChapterCount > 0 {
		for range fileCount - c.ChapterCount {
			dist.add("unmatched_tracks", 1.0)
		}
	}

	return dist.distance()
}

// albumDistanceWithTracks computes a beets-faithful full distance score [0..1]
// combining album-level metadata penalties with per-track distance penalties.
// It builds the track cost matrix internally and runs the Hungarian assignment,
// so it accounts for both "how well does the metadata match" and "how well do
// the individual tracks match" — the same approach beets uses for its final
// candidate ranking step.
//
// Per-track distances are accumulated under the "tracks" key (weight 3.0).
// DB tracks left unassigned contribute to "missing_tracks" (weight 0.9).
// Extra local tracks contribute to "unmatched_tracks" (weight 0.6).
func albumDistanceWithTracks(
	c *database.AlbumSearchResult,
	artist, album, mbReleaseID string,
	year int,
	localTracks []parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	isVA bool,
	data *config.MediaDataConfig,
) float64 {
	dist := newBeetsDistance()

	// Album metadata (same components as albumMatchDistance, without track counts).
	// Normalise VA abbreviations so "VA"/"V.A." score 0 against "Various Artists".
	localArtist := artist
	if IsVariousArtists(localArtist) {
		localArtist = VariousArtistsName
	}

	dbArtist := c.Artist
	if IsVariousArtists(dbArtist) {
		dbArtist = VariousArtistsName
	}

	dist.addString("artist", dbArtist, localArtist)
	dist.addString("album", c.Title, album)

	if mbReleaseID != "" && c.MusicBrainzReleaseID != "" {
		if c.MusicBrainzReleaseID == mbReleaseID {
			dist.add("album_id", 0.0)
		} else {
			dist.add("album_id", 1.0)
		}
	}

	if year > 0 && c.Year > 0 {
		if year == c.Year {
			dist.add("year", 0.0)
		} else {
			diff := math.Abs(float64(year - c.Year))
			diffMax := float64(time.Now().Year() - 1889)
			dist.addRatio("year", diff, diffMax)
		}
	}

	// Same normalisation used in matchTracksByDistance so the two functions
	// agree on track positions when computing the full album distance.
	dbTracks = globaliseDiscTrackNumbers(dbTracks, localTracks)

	sourcePenalty := config.GetMusicSourcePenalty(c.Source)

	nDB := len(dbTracks)

	nLocal := len(localTracks)
	if nDB == 0 || nLocal == 0 {
		// No track data — fall back to track-count delta penalties.
		for range nDB - min(nDB, nLocal) {
			dist.add("missing_tracks", 1.0)
		}

		for range nLocal - min(nDB, nLocal) {
			dist.add("unmatched_tracks", 1.0)
		}

		return dist.distance() + sourcePenalty
	}

	// Build padded square cost matrix and run Hungarian assignment.
	size := max(nLocal, nDB)

	cost := make([][]float64, size)
	for i := range cost {
		cost[i] = make([]float64, size)
		for j := range cost[i] {
			if i < nDB && j < nLocal {
				cost[i][j] = trackDistance(&localTracks[j], &dbTracks[i], isVA, false, data)
			} else {
				cost[i][j] = 1.0
			}
		}
	}

	rowAssign := lapAssign(cost)

	// Accumulate per-track penalties. A pair rejected for exceeding maxTD leaves
	// both sides unmatched, exactly as matchTracksByDistance reports it.
	const maxTD = 0.9
	for i := range nDB {
		j := rowAssign[i]
		if j < nLocal && cost[i][j] < maxTD {
			dist.add("tracks", cost[i][j])
		} else {
			dist.add("missing_tracks", 1.0)

			if j < nLocal {
				dist.add("unmatched_tracks", 1.0)
			}
		}
	}

	// Unmatched local tracks are assigned to dummy DB rows (i >= nDB).
	for i := nDB; i < size; i++ {
		if rowAssign[i] < nLocal {
			dist.add("unmatched_tracks", 1.0)
		}
	}

	return dist.distance() + sourcePenalty
}

// audiobookDistanceWithTracks computes the full beets-style distance for an audiobook
// candidate, combining title/author metadata with per-chapter runtime distances.
// Mirrors albumDistanceWithTracks but uses AudiobookSearchResult and isAudiobook=true
// for trackDistance (titles are unreliable for audiobook chapters).
func audiobookDistanceWithTracks(
	c *database.AudiobookSearchResult,
	title, author string,
	localTracks []parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	data *config.MediaDataConfig,
) float64 {
	dist := newBeetsDistance()

	// Title (weight 3.0).
	dist.addString("album", c.Title, title)

	// Author (weight 2.0 — mapped to "artist").
	// Fixed uncertainty penalty when either side lacks an author.
	if c.Author == "" || author == "" {
		dist.add("artist", 0.5)
	} else {
		dist.addString("artist", c.Author, author)
	}

	nDB := len(dbTracks)

	nLocal := len(localTracks)
	if nDB == 0 || nLocal == 0 {
		for range nDB - min(nDB, nLocal) {
			dist.add("missing_tracks", 1.0)
		}

		for range nLocal - min(nDB, nLocal) {
			dist.add("unmatched_tracks", 1.0)
		}

		return dist.distance()
	}

	if nLocal == 1 && nDB > 1 {
		var chaptersMs int64
		for i := range dbTracks {
			chaptersMs += dbTracks[i].RuntimeMs
		}

		ok, diffMs, toleranceMs := singleFileTotalMatch(
			localTracks[0].RuntimeMS, chaptersMs, nDB, data,
		)
		if ok {
			dist.add("tracks", float64(diffMs)/float64(toleranceMs))
		} else {
			dist.add("missing_tracks", 1.0)
			dist.add("unmatched_tracks", 1.0)
		}

		return dist.distance()
	}

	// Build padded square cost matrix and run Hungarian assignment (audiobook mode).
	size := max(nLocal, nDB)

	cost := make([][]float64, size)
	for i := range cost {
		cost[i] = make([]float64, size)
		for j := range cost[i] {
			if i < nDB && j < nLocal {
				cost[i][j] = trackDistance(&localTracks[j], &dbTracks[i], false, true, data)
			} else {
				cost[i][j] = 1.0
			}
		}
	}

	// Same assignment as matchTracksByDistance, so the score describes the
	// pairing that would actually be applied.
	const maxTD = 0.9

	var rowAssign []int
	if order := audiobookLocalOrder(localTracks, true); order != nil {
		rowAssign = alignOrdered(cost, nDB, order, maxTD)
	} else {
		rowAssign = lapAssign(cost)
	}

	// Accumulate per-chapter distances as a single averaged "tracks" component,
	// mirroring beets' approach. Adding one entry per chapter would give chapters
	// weight 3.0×N vs metadata weight ~5, completely overwhelming title/author for
	// audiobooks with many chapters. A single averaged entry keeps the weight fixed
	// at 3.0 regardless of chapter count.
	var (
		trackDistSum   float64
		trackDistCount int
	)

	for i := range nDB {
		j := rowAssign[i]
		if j >= 0 && j < nLocal && cost[i][j] < maxTD {
			trackDistSum += cost[i][j]
			trackDistCount++
		} else {
			dist.add("missing_tracks", 1.0)
		}
	}

	if trackDistCount > 0 {
		dist.add("tracks", trackDistSum/float64(trackDistCount))
	}

	// Every local track not paired with a chapter is unmatched, whether the
	// assignment left it on a rejected pair or a padding row.
	for range nLocal - trackDistCount {
		dist.add("unmatched_tracks", 1.0)
	}

	return dist.distance()
}

// recommendation returns the match quality level for a set of full-distance scores.
// sortedDists must be sorted ascending (best-first).
//
// Faithfully mirrors beets' _recommendation() from autotag/match.py:
//
//	dist < strongRecThresh (0.04)               → recStrong
//	dist ≤ mediumRecThresh (0.30)               → recMedium
//	  … but if only one candidate or 2nd-best gap ≥ recGapThresh (0.25) → recStrong
//	dist > mediumRecThresh, single or big gap   → recLow
//	otherwise                                   → recNone
//
// recLow means the best candidate is weak; callers that only accept ≥ recMedium
// will fall through to the alternative-release search path. The strong/medium
// thresholds come from data (see recThresholds; nil = built-in defaults).
func recommendation(sortedDists []float64, data *config.MediaDataConfig) albumRecommendation {
	if len(sortedDists) == 0 {
		return recNone
	}

	strongThresh, mediumThresh := recThresholds(data)

	best := sortedDists[0]
	single := len(sortedDists) < 2
	hasGap := !single && sortedDists[1]-best >= recGapThresh

	// Zone 1: very close match → always strong.
	if best < strongThresh {
		return recStrong
	}

	// Zone 2: acceptable match.
	if best <= mediumThresh {
		// Upgrade to strong when no competition (single candidate or clear gap).
		if single || hasGap {
			return recStrong
		}

		return recMedium
	}

	// Zone 3: distance above medium threshold — only accept if there is no competition.
	// This mirrors beets' recLow for dist > medium_rec_thresh with single/gap.
	if single || hasGap {
		return recLow
	}

	return recNone
}

// selectBestAlbumMatches scores all candidates using beets-style weighted distance
// and returns those within the medium threshold (recThresholds), sorted best-first.
// Exact track-count matches are always kept regardless of threshold.
// This replaces the previous "exact track count only" approach so that alternative
// editions (bonus tracks, different pressings) are still passed to runtime verification.
func selectBestAlbumMatches(
	matches []*database.AlbumSearchResult,
	fileCount int,
	artist, album, mbReleaseID, label, country string,
	year int,
	data *config.MediaDataConfig,
) []*database.AlbumSearchResult {
	if len(matches) == 0 {
		return nil
	}

	type scored struct {
		m    *database.AlbumSearchResult
		dist float64
	}

	// exactTrackHardCap is the maximum allowed distance for exact-track-count candidates.
	// Without this cap, a release with 0 matching artist/title but the same number of tracks
	// would always pass (d ≈ 1.0), causing completely wrong albums to become candidates.
	// 0.75 allows same-artist alternative editions (different pressings, bonus discs) whose
	// album title or year differs slightly, while rejecting unrelated artists/albums.
	const exactTrackHardCap = 0.75

	_, mediumThresh := recThresholds(data)

	var candidates []scored
	for _, m := range matches {
		d := albumMatchDistance(
			m,
			artist,
			album,
			mbReleaseID,
			label,
			country,
			year,
			fileCount,
			data,
		)
		// Exact track-count matches get a more lenient threshold (exactTrackHardCap) so that
		// alternative editions of the same album are included for runtime verification.
		// The hard cap prevents unrelated artists/albums from bypassing the distance check
		// just because their track count happens to match.
		// When AllowMissingTracks is set, also apply the lenient cap to albums with more
		// tracks than local files (m.TotalTracks >= fileCount), since the user accepts partial albums.
		allowMissing := data != nil && data.AllowMissingTracks

		isExactOrAllowed := m.TotalTracks == fileCount ||
			(allowMissing && m.TotalTracks >= fileCount)
		if (isExactOrAllowed && d <= exactTrackHardCap) || d <= mediumThresh {
			candidates = append(candidates, scored{m, d})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	result := make([]*database.AlbumSearchResult, len(candidates))
	for i, c := range candidates {
		result[i] = c.m
	}

	return result
}

// selectBestAudiobookMatches scores all candidates using beets-style weighted distance
// (title, author, and chapter-count mismatch penalties).
// Returns candidates within the medium threshold (recThresholds) sorted best-first.
// Exact chapter-count matches are always included regardless of threshold.
// When AllowMissingTracks is enabled, candidates with ChapterCount >= fileCount
// whose distance score passes are also included for runtime verification.
func selectBestAudiobookMatches(
	matches []*database.AudiobookSearchResult,
	fileCount int,
	data *config.MediaDataConfig,
	title, author string,
) []*database.AudiobookSearchResult {
	if len(matches) == 0 {
		return nil
	}

	type scored struct {
		m    *database.AudiobookSearchResult
		dist float64
	}

	// Mirrors selectBestAlbumMatches's exactTrackHardCap: without a ceiling
	// here, an audiobook with 0 matching author/title but the same chapter
	// count would always pass (d ≈ 1.0), letting a completely unrelated book
	// become a candidate - and bestCandidates[0] downstream (album_processor.go)
	// is read before the later distance/recommendation re-gate runs.
	const exactChapterHardCap = 0.75

	_, mediumThresh := recThresholds(data)

	var candidates []scored
	for _, m := range matches {
		d := audiobookMatchDistance(m, title, author, fileCount)
		isExact := m.ChapterCount == fileCount

		isAllowedMissing := data != nil && data.AllowMissingTracks && m.ChapterCount >= fileCount
		if ((isExact || isAllowedMissing) && d <= exactChapterHardCap) || d <= mediumThresh {
			candidates = append(candidates, scored{m, d})
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].dist < candidates[j].dist
	})

	result := make([]*database.AudiobookSearchResult, len(candidates))
	for i, c := range candidates {
		result[i] = c.m
	}

	return result
}

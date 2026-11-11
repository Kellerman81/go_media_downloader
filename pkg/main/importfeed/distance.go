package importfeed

import (
	"math"
	"regexp"
	"slices"
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
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
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

	// unicodeFold maps accented Latin characters and German umlauts to their ASCII
	// equivalents before Levenshtein comparison, so "Müller"≈"Mueller", "étude"≈"etude".
	// Applied by foldUnicode.
	unicodeFold = foldMap(
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

// foldMap builds unicodeFold from rune/replacement pairs.
func foldMap(pairs ...string) map[rune]string {
	m := make(map[rune]string, len(pairs)/2)
	for i := 0; i+1 < len(pairs); i += 2 {
		r, _ := utf8.DecodeRuneInString(pairs[i])
		m[r] = pairs[i+1]
	}

	return m
}

// foldUnicode replaces the characters in unicodeFold. It returns s itself,
// without allocating, when there is nothing to replace - always for ASCII.
// (strings.Replacer with multi-byte keys allocates on every call; stringDist
// runs for every local file x release track pair.)
func foldUnicode(s string) string {
	first := -1

	for i, r := range s {
		if r < utf8.RuneSelf {
			continue
		}

		if _, ok := unicodeFold[r]; ok {
			first = i
			break
		}
	}

	if first < 0 {
		return s
	}

	var b strings.Builder
	b.Grow(len(s) + 8)
	b.WriteString(s[:first])

	for _, r := range s[first:] {
		if rep, ok := unicodeFold[r]; ok {
			b.WriteString(rep)
		} else {
			b.WriteRune(r)
		}
	}

	return b.String()
}

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
	return normalizedDist(sdNormalize(str1), sdNormalize(str2), str1, str2)
}

// normalizedDist is stringDistBasic on the sdNormalize forms n1, n2 of raw1,
// raw2.
func normalizedDist(n1, n2, raw1, raw2 string) float64 {
	if n1 == "" && n2 == "" {
		if strings.EqualFold(raw1, raw2) {
			return 0.0
		}

		return 1.0
	}

	maxLen := max(utf8.RuneCountInString(n2), utf8.RuneCountInString(n1))

	return float64(levenshteinInt(n1, n2)) / float64(maxLen)
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
	t1, t2 := prepareSD(str1), prepareSD(str2)

	return stringDistPrepared(&t1, &t2)
}

// sdCut is a string with one of sdPatterns removed, and its sdNormalize form.
type sdCut struct {
	s, n string
}

// sdText is a string prepared for stringDistPrepared: the steps of stringDist
// that depend on one side only. Track matching compares every local title
// with every release title, so it prepares each title once (see trackTexts).
type sdText struct {
	raw string
	// s is raw folded (foldUnicode), with a trailing ", the" moved to the
	// front and & replaced; n is sdNormalize(s).
	s, n string
	// mask has bit i set when sdPatterns[i] matches s; cuts[i] is s with that
	// pattern removed, filled on first use (cutDone).
	mask, cutDone uint8
	cuts          []sdCut
	// paren is n with numParenRe groups removed first, filled on first use.
	paren     string
	parenDone bool
}

// prepareSD prepares raw for stringDistPrepared.
func prepareSD(raw string) sdText {
	t := sdText{raw: raw}
	if raw == "" {
		return t
	}

	// Pre-normalize: map accented chars and German umlauts to ASCII equivalents so that
	// "Müller"≡"Mueller", "étude"≡"etude", etc. contribute zero Levenshtein distance.
	str := foldUnicode(raw)

	// SD_END_WORDS: "something, the" → "the something"
	for i, suffix := range sdEndWordSuffixes {
		if trimmed := logger.TrimSuffixI(str, suffix); trimmed != str {
			str = logger.JoinStrings(sdEndWordList[i], " ", trimmed)
		}
	}

	// SD_REPLACE: & → and
	t.s = ampReplacer.Replace(str)
	t.n = sdNormalize(t.s)

	for i := range sdPatterns {
		if sdPatterns[i].re.MatchString(t.s) {
			t.mask |= 1 << i
		}
	}

	return t
}

// sdPatternCut applies sdPatterns[i] to str (n is its sdNormalize form): from
// t (computing and keeping the cut on first use) while str is still t.s,
// otherwise directly.
func sdPatternCut(t *sdText, i int, str, n string, prepared bool) (matched bool, cut, cutN string) {
	if prepared {
		if t.mask&(1<<i) == 0 {
			return false, str, n
		}

		if t.cutDone&(1<<i) == 0 {
			if t.cuts == nil {
				t.cuts = make([]sdCut, len(sdPatterns))
			}

			cut = sdPatterns[i].re.ReplaceAllLiteralString(t.s, "")
			t.cuts[i] = sdCut{cut, sdNormalize(cut)}
			t.cutDone |= 1 << i
		}

		return true, t.cuts[i].s, t.cuts[i].n
	}

	if !sdPatterns[i].re.MatchString(str) {
		return false, str, n
	}

	cut = sdPatterns[i].re.ReplaceAllLiteralString(str, "")

	return true, cut, sdNormalize(cut)
}

// stringDistPrepared is stringDist on prepared strings. It fills the lazy
// parts of t1 and t2, so a prepared text must not be shared between
// goroutines.
func stringDistPrepared(t1, t2 *sdText) float64 {
	if t1.raw == "" && t2.raw == "" {
		return 0.0
	}

	if t1.raw == "" || t2.raw == "" {
		return 1.0
	}

	str1, str2 := t1.s, t2.s
	n1, n2 := t1.n, t2.n
	// prep1/prep2: str1/str2 are still the prepared t1.s/t2.s.
	prep1, prep2 := true, true

	// SD_PATTERNS: each match is credited at its weight rather than full distance.
	baseDist := normalizedDist(n1, n2, str1, str2)

	penalty := 0.0
	for i := range sdPatterns {
		match1, case1, c1 := sdPatternCut(t1, i, str1, n1, prep1)
		match2, case2, c2 := sdPatternCut(t2, i, str2, n2, prep2)

		if !match1 && !match2 {
			continue
		}

		caseDist := normalizedDist(c1, c2, case1, case2)

		caseDelta := math.Max(0.0, baseDist-caseDist)
		if caseDelta == 0.0 {
			continue
		}

		prep1 = prep1 && !match1
		prep2 = prep2 && !match2
		str1, str2 = case1, case2
		n1, n2 = c1, c2
		baseDist = caseDist

		penalty += sdPatterns[i].weight * caseDelta
	}

	// Optional: subtitle/movement containment check.
	// If one title (with track-number and time-code parentheticals stripped) is a
	// prefix or suffix of the other, cap the distance at 0.3 × extra-content fraction.
	// Example: "Preludio (Largo)" is a suffix of "Sonata in E minor - Preludio (Largo)".
	if reg := baseDist + penalty; reg > 0 {
		s1 := parenStripped(t1, str1, n1, prep1)
		s2 := parenStripped(t2, str2, n2, prep2)

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

// parenStripped is sdNormalize of str without numParenRe groups.
func parenStripped(t *sdText, str, n string, prepared bool) string {
	switch {
	case prepared:
		if !t.parenDone {
			t.paren = parenStripped(t, str, n, false)
			t.parenDone = true
		}

		return t.paren
	case strings.IndexByte(str, '(') < 0:
		return n
	default:
		return sdNormalize(numParenRe.ReplaceAllLiteralString(str, ""))
	}
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

// trackDistance mirrors beets track_distance(): how well a local file matches
// a release track, from
//
//	track_length  (weight 3.0)
//	track_title   (weight 3.0 music / 1.0 audiobook)
//	track_artist  (weight 2.0) — only for VA albums
//	track_index   (weight 1.0 music / 3.0 audiobook)
//	track_id      (weight 5.0) — only when both carry a MusicBrainz recording ID
//
// overridable per config. For many pairs use one trackScorer with localTexts /
// dbTexts, so the config and every title are prepared once.
func trackDistance(
	local *parser_v2.TrackInfo,
	db *database.DbtrackWithArtist,
	isVA, isAudiobook bool,
	data *config.MediaDataConfig,
) float64 {
	sc := newTrackScorer(isVA, isAudiobook, data)

	var lt, dt trackText

	return sc.dist(local, db, &lt, &dt)
}

// trackScorer holds what trackDistance derives from the config: runtime
// bounds and the track weights.
type trackScorer struct {
	isVA                                     bool
	strictMode                               bool
	graceMs, hardMs                          int64
	wLength, wTitle, wArtist, wIndex, wTrkID float64
}

// trackText is a track's title (and artist, for VA releases) prepared for
// stringDistPrepared. It is prepared on first use: pairs rejected on runtime
// never compare titles.
type trackText struct {
	title, artist sdText
	ready         bool
}

func newTrackScorer(isVA, isAudiobook bool, data *config.MediaDataConfig) trackScorer {
	sc := trackScorer{
		isVA:    isVA,
		graceMs: trackLengthGraceMs,
		hardMs:  trackLengthMaxMs,
		wLength: beetsWeights["track_length"],
		wTitle:  beetsWeights["track_title"],
		wArtist: beetsWeights["track_artist"],
		wIndex:  beetsWeights["track_index"],
		wTrkID:  beetsWeights["track_id"],
	}

	// Derive runtime bounds from config; fall back to beets defaults.
	// When PerTrackToleranceSeconds > 0:
	//   graceMs = PerTrackToleranceSeconds * 1000  (diff ≤ grace → no penalty)
	//   hardMs  = PerTrackToleranceSecondsMax * 1000, if > 0 (diff > hard → hard reject)
	//           = graceMs, if PerTrackToleranceSecondsMax == 0 (grace is the hard limit)
	sc.strictMode = data != nil && data.PerTrackToleranceSeconds > 0
	if sc.strictMode {
		sc.graceMs = int64(data.PerTrackToleranceSeconds) * 1000
		if data.PerTrackToleranceSecondsMax > 0 &&
			data.PerTrackToleranceSecondsMax > data.PerTrackToleranceSeconds {
			sc.hardMs = int64(data.PerTrackToleranceSecondsMax) * 1000
		} else {
			sc.hardMs = sc.graceMs // grace == hard → any excess is a hard reject
		}
	}

	// Weights: audiobook defaults (if applicable), then per-config overrides.
	if isAudiobook {
		sc.wLength = audiobookTrackWeights["track_length"]
		sc.wTitle = audiobookTrackWeights["track_title"]
		sc.wArtist = audiobookTrackWeights["track_artist"]
		sc.wIndex = audiobookTrackWeights["track_index"]
		sc.wTrkID = audiobookTrackWeights["track_id"]
	}

	if data != nil {
		override := func(w *float64, v float64) {
			if v != 0 {
				*w = v
			}
		}
		override(&sc.wTitle, data.TrackTitleWeight)
		override(&sc.wIndex, data.TrackIndexWeight)
		override(&sc.wLength, data.TrackLengthWeight)
		override(&sc.wArtist, data.TrackArtistWeight)
		override(&sc.wTrkID, data.TrackIdWeight)
	}

	return sc
}

// prepare fills t from title and artist unless already done.
func (sc *trackScorer) prepare(t *trackText, title, artist string) {
	if t.ready {
		return
	}

	t.title = prepareSD(title)
	if sc.isVA {
		t.artist = prepareSD(artist)
	}

	t.ready = true
}

// localTexts returns unprepared texts for tracks, filled by dist on first use.
func (*trackScorer) localTexts(tracks []parser_v2.TrackInfo) []trackText {
	return make([]trackText, len(tracks))
}

// dbTexts returns unprepared texts for dbTracks, filled by dist on first use.
func (*trackScorer) dbTexts(dbTracks []database.DbtrackWithArtist) []trackText {
	return make([]trackText, len(dbTracks))
}

// dist mirrors beets' track_distance: each component adds value×weight to
// the raw distance and weight to the maximum; the result is raw/max.
func (sc *trackScorer) dist(
	local *parser_v2.TrackInfo,
	db *database.DbtrackWithArtist,
	lt, dt *trackText,
) float64 {
	var raw, maxDist float64

	add := func(value, weight float64) {
		raw += value * weight
		maxDist += weight
	}

	// track_length
	if db.RuntimeMs > 0 {
		diff := local.RuntimeMS - db.RuntimeMs
		if diff < 0 {
			diff = -diff
		}

		if sc.strictMode {
			// Strict two-tier mode:
			//   diff ≤ graceMs → no penalty
			//   graceMs < diff ≤ hardMs → graduated penalty (diff−grace)/(hard−grace)
			//   diff > hardMs → hard reject (excluded from matching entirely)
			if diff > sc.hardMs {
				return 1.0
			}

			window := sc.hardMs - sc.graceMs
			if window <= 0 || diff <= sc.graceMs {
				add(0, sc.wLength)
			} else {
				add(clampRatio(float64(diff-sc.graceMs), float64(window)), sc.wLength)
			}
		} else {
			// Beets default: 10 s grace window, 30 s max penalty window.
			add(clampRatio(float64(diff-sc.graceMs), float64(sc.hardMs)), sc.wLength)
		}
	}

	sc.prepare(lt, local.Title, local.Artist)
	sc.prepare(dt, db.Title, db.Artist)

	// track_title
	add(stringDistPrepared(&lt.title, &dt.title), sc.wTitle)

	// track_artist — beets only adds this for VA releases when the DB track has
	// an artist and the local artist is not one of the "various artists" tokens.
	if sc.isVA && db.Artist != "" &&
		!logger.SlicesContainsI(vaArtistTokens, strings.TrimSpace(local.Artist)) {
		add(stringDistPrepared(&lt.artist, &dt.artist), sc.wArtist)
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

		add(float64(diff)/float64(max(localN, dbN)), sc.wIndex)
	}

	// track_id — only when local file carries a MusicBrainz recording ID
	if local.MusicBrainzRecordingID != "" && db.MusicbrainzRecordingID != "" {
		if local.MusicBrainzRecordingID == db.MusicbrainzRecordingID {
			add(0, sc.wTrkID)
		} else {
			add(1, sc.wTrkID)
		}
	}

	if maxDist == 0 {
		return 0
	}

	return raw / maxDist
}

// clampRatio mirrors beets Distance.add_ratio: number1 clamped to [0, number2],
// divided by number2.
func clampRatio(number1, number2 float64) float64 {
	if number2 <= 0 {
		return 0
	}

	return math.Max(math.Min(number1, number2), 0) / number2
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

// repackedTotalMatch reports whether local holds the whole book although it
// is cut into more than one file and not into the book's chapters (e.g. 152
// parts of a 60-chapter book): every file has a runtime and their total
// matches chaptersMs. The tolerance is per_track_tolerance_seconds (default
// 10 s) per chapter, or max_total_difference_seconds - stricter than for a
// single file, because the file count says nothing about completeness and a
// release missing a chapter must not pass.
func repackedTotalMatch(
	local []parser_v2.TrackInfo,
	chaptersMs int64,
	chapters int,
	data *config.MediaDataConfig,
) (ok bool, diffMs, toleranceMs int64) {
	if len(local) < 2 || len(local) == chapters || chapters <= 0 || chaptersMs <= 0 {
		return false, 0, 0
	}

	var localMs int64
	for i := range local {
		if local[i].RuntimeMS <= 0 {
			return false, 0, 0
		}

		localMs += local[i].RuntimeMS
	}

	toleranceMs = repackToleranceMs(chapters, data)

	diffMs = localMs - chaptersMs
	if diffMs < 0 {
		diffMs = -diffMs
	}

	return diffMs <= toleranceMs, diffMs, toleranceMs
}

// repackToleranceMs is the total-runtime tolerance of repackedTotalMatch for a
// book of chapters chapters.
func repackToleranceMs(chapters int, data *config.MediaDataConfig) int64 {
	perChapterMs := trackLengthGraceMs
	if data != nil && data.PerTrackToleranceSeconds > 0 {
		perChapterMs = int64(data.PerTrackToleranceSeconds) * 1000
	}

	if data != nil && data.MaxTotalDifferenceSeconds > 0 {
		return int64(data.MaxTotalDifferenceSeconds) * 1000
	}

	return int64(chapters) * perChapterMs
}

// audiobookMetaDistance is audiobookMatchDistance without the chapter-count
// penalties, for candidates whose total runtime is verified instead.
func audiobookMetaDistance(c *database.AudiobookSearchResult, title, author string) float64 {
	return audiobookMatchDistance(c, title, author, 1)
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

	// Each title is prepared once, not once per pair.
	sc := newTrackScorer(isVA, isAudiobook, data)
	lts, dts := sc.localTexts(tracks), sc.dbTexts(dbTracks)

	// Single contiguous allocation; row headers slice into it — 2 allocs instead of size+1.
	backing := make([]float64, size*size)

	cost := make([][]float64, size)
	for i := range cost {
		row := backing[i*size : (i+1)*size]

		cost[i] = row
		if i < n {
			for j := range size {
				if j < m {
					row[j] = sc.dist(&tracks[j], &dbTracks[i], &lts[j], &dts[i])
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
	// Skipped when AllowExtraTracks is set, like missing tracks above.
	if (data == nil || !data.AllowExtraTracks) && trackCount > c.TotalTracks {
		for range trackCount - c.TotalTracks {
			dist.add("unmatched_tracks", 1.0)
		}
	}

	return dist.distance()
}

// albumMetaDistance is albumMatchDistance without the track-count penalties:
// how well the release matches the tags alone.
func albumMetaDistance(
	c *database.AlbumSearchResult,
	artist, album, mbReleaseID string,
	year int,
	data *config.MediaDataConfig,
) float64 {
	return albumMatchDistance(c, artist, album, mbReleaseID, "", "", year, c.TotalTracks, data)
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

	sc := newTrackScorer(isVA, false, data)
	lts, dts := sc.localTexts(localTracks), sc.dbTexts(dbTracks)

	cost := make([][]float64, size)
	for i := range cost {
		cost[i] = make([]float64, size)
		for j := range cost[i] {
			if i < nDB && j < nLocal {
				cost[i][j] = sc.dist(&localTracks[j], &dbTracks[i], &lts[j], &dts[i])
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

	var chaptersMs int64
	for i := range dbTracks {
		chaptersMs += dbTracks[i].RuntimeMs
	}

	if nLocal == 1 && nDB > 1 {
		ok, diffMs, toleranceMs := singleFileTotalMatch(
			localTracks[0].RuntimeMS, chaptersMs, nDB, data,
		)
		if !ok {
			dist.add("missing_tracks", 1.0)
			dist.add("unmatched_tracks", 1.0)

			return dist.distance()
		}

		// A file whose embedded chapters are the candidate's chapters is
		// scored chapter by chapter, which tells editions of the same length
		// apart. Its total already matches, so a chapter that does not pair
		// (a boundary moved) is no sign of missing content: then the total
		// decides, as for a file without chapters. Embedded chapter titles are
		// the release's own ("001 - Kapitel 1 & Kapitel 2.1 - <book>"), so
		// only lengths and order count.
		if sum, ok := embeddedChapterPairing(&localTracks[0], dbTracks, data); ok {
			dist.add("tracks", sum/float64(nDB))

			return dist.distance()
		}

		dist.add("tracks", float64(diffMs)/float64(toleranceMs))

		return dist.distance()
	}

	if ok, diffMs, toleranceMs := repackedTotalMatch(localTracks, chaptersMs, nDB, data); ok {
		dist.add("tracks", float64(diffMs)/float64(toleranceMs))

		return dist.distance()
	}

	// Accumulate per-chapter distances as a single averaged "tracks" component,
	// mirroring beets' approach. Adding one entry per chapter would give chapters
	// weight 3.0×N vs metadata weight ~5, completely overwhelming title/author for
	// audiobooks with many chapters. A single averaged entry keeps the weight fixed
	// at 3.0 regardless of chapter count.
	sum, pairs := audiobookChapterPairing(localTracks, dbTracks, data, true)
	paired := pairedCount(pairs)

	for range nDB - paired {
		dist.add("missing_tracks", 1.0)
	}

	if paired > 0 {
		dist.add("tracks", sum/float64(paired))
	}

	// Every local track not paired with a chapter is unmatched, whether the
	// assignment left it on a rejected pair or a padding row.
	for range nLocal - paired {
		dist.add("unmatched_tracks", 1.0)
	}

	return dist.distance()
}

// embeddedChapterPairing pairs the chapters embedded in a single file with
// the candidate's chapters on length and order. Titles are left out: they are
// the release's own ("001 - Kapitel 1 & Kapitel 2.1 - <book>"). ok only when
// the counts are equal and every chapter pairs; sum is the summed distance.
func embeddedChapterPairing(
	local *parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	data *config.MediaDataConfig,
) (sum float64, ok bool) {
	if len(dbTracks) < 2 || len(local.Chapters) != len(dbTracks) {
		return 0, false
	}

	sum, pairs := audiobookChapterPairing(embeddedChapterTracks(local), dbTracks, data, false)

	return sum, pairedCount(pairs) == len(dbTracks)
}

// pairedCount counts the paired entries of an audiobookChapterPairing result.
func pairedCount(pairs []int) int {
	n := 0

	for _, j := range pairs {
		if j >= 0 {
			n++
		}
	}

	return n
}

// audiobookChapterPairing pairs local tracks with the candidate's chapters the
// way matchTracksByDistance does. pairs[i] is the local track paired with
// chapter i, or -1 when its distance is not below the track-distance limit;
// sum is the summed distance of the pairs. Without titles, only lengths and
// order are compared.
func audiobookChapterPairing(
	localTracks []parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	data *config.MediaDataConfig,
	titles bool,
) (sum float64, pairs []int) {
	nLocal, nDB := len(localTracks), len(dbTracks)

	// Build padded square cost matrix and run Hungarian assignment (audiobook mode).
	size := max(nLocal, nDB)

	sc := newTrackScorer(false, true, data)
	if !titles {
		sc.wTitle = 0
	}
	lts, dts := sc.localTexts(localTracks), sc.dbTexts(dbTracks)

	cost := make([][]float64, size)
	for i := range cost {
		cost[i] = make([]float64, size)
		for j := range cost[i] {
			if i < nDB && j < nLocal {
				cost[i][j] = sc.dist(&localTracks[j], &dbTracks[i], &lts[j], &dts[i])
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

	pairs = make([]int, nDB)

	for i := range nDB {
		pairs[i] = -1
		if j := rowAssign[i]; j >= 0 && j < nLocal && cost[i][j] < maxTD {
			sum += cost[i][j]
			pairs[i] = j
		}
	}

	return sum, pairs
}

// releaseChapterList returns the chapter list to write into local, one file
// holding the whole book, from the candidate's chapters dbTracks:
//   - the file's chapters pair 1:1 with the candidate's (embeddedChapterPairing):
//     the file's chapters under the candidate's titles;
//   - otherwise, with replace_embedded_chapters: the candidate's chapters,
//     their starts scaled to the file's runtime (also for a file without
//     chapters);
//   - otherwise: the file's chapters, each under the title of the candidate's
//     chapter it starts in, the way files cut differently than the chapters
//     are named (structure.assignAudiobookChapters).
//
// nil when the candidate's chapters lack a title or runtime, the file has
// nothing to rename, or the list would not change.
func releaseChapterList(
	local *parser_v2.TrackInfo,
	dbTracks []database.DbtrackWithArtist,
	data *config.MediaDataConfig,
) []tags.Chapter {
	if len(dbTracks) < 2 || local.RuntimeMS <= 0 ||
		slices.ContainsFunc(dbTracks, func(t database.DbtrackWithArtist) bool {
			return t.Title == "" || t.RuntimeMs <= 0
		}) {
		return nil
	}

	var out []tags.Chapter

	if _, ok := embeddedChapterPairing(local, dbTracks, data); ok {
		_, pairs := audiobookChapterPairing(embeddedChapterTracks(local), dbTracks, data, false)

		out = slices.Clone(local.Chapters)
		for i, j := range pairs {
			out[j].Title = dbTracks[i].Title
		}
	} else if data != nil && data.ReplaceEmbeddedChapters {
		out = scaledReleaseChapters(dbTracks, local.RuntimeMS)
	} else if len(local.Chapters) > 0 {
		out = chaptersTitledByStart(local.Chapters, dbTracks, local.RuntimeMS)
	}

	if out == nil || slices.Equal(out, local.Chapters) {
		return nil
	}

	return out
}

// releaseChapterStarts returns the start of each of the candidate's chapters
// on its own timeline, and its total runtime.
func releaseChapterStarts(dbTracks []database.DbtrackWithArtist) (starts []int64, totalMs int64) {
	starts = make([]int64, len(dbTracks))
	for i := range dbTracks {
		starts[i] = totalMs
		totalMs += dbTracks[i].RuntimeMs
	}

	return starts, totalMs
}

// scaledReleaseChapters returns the candidate's chapters with their starts
// scaled to a file of fileMs; the last one runs to the end of the file.
func scaledReleaseChapters(dbTracks []database.DbtrackWithArtist, fileMs int64) []tags.Chapter {
	starts, totalMs := releaseChapterStarts(dbTracks)

	out := make([]tags.Chapter, len(dbTracks))
	for i := range dbTracks {
		out[i] = tags.Chapter{Title: dbTracks[i].Title, StartMS: starts[i] * fileMs / totalMs}
	}

	for i := range out {
		end := fileMs
		if i+1 < len(out) {
			end = out[i+1].StartMS
		}

		out[i].RuntimeMS = end - out[i].StartMS
	}

	return out
}

// chapterStartSlackMs lets a chapter that starts just before a release
// chapter boundary (rounding, a little silence) take that chapter's title.
// Same value as structure.chapterStartSlackMs for files.
const chapterStartSlackMs = 2_000

// chaptersTitledByStart returns the file's chapters, each titled after the
// candidate's chapter its start falls into, positions scaled from the file's
// runtime fileMs to the candidate's total.
func chaptersTitledByStart(
	chapters []tags.Chapter,
	dbTracks []database.DbtrackWithArtist,
	fileMs int64,
) []tags.Chapter {
	starts, totalMs := releaseChapterStarts(dbTracks)

	out := slices.Clone(chapters)
	for i := range out {
		pos := out[i].StartMS*totalMs/fileMs + chapterStartSlackMs

		// The last release chapter starting at or before pos.
		ch, _ := slices.BinarySearch(starts, pos+1)
		out[i].Title = dbTracks[max(ch-1, 0)].Title
	}

	return out
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
		// tracks than local files (m.TotalTracks >= fileCount), since the user accepts partial
		// albums; AllowExtraTracks does the same for albums with fewer tracks.
		allowMissing := data != nil && data.AllowMissingTracks
		allowExtra := data != nil && data.AllowExtraTracks

		isExactOrAllowed := m.TotalTracks == fileCount ||
			(allowMissing && m.TotalTracks >= fileCount) ||
			(allowExtra && m.TotalTracks <= fileCount)
		if (isExactOrAllowed && d <= exactTrackHardCap) || d <= mediumThresh {
			candidates = append(candidates, scored{m, d})
			continue
		}

		// allow_count_mismatch_if_total_match: the tags must match on their own;
		// the candidate pass checks the total runtime.
		if data != nil && data.AllowCountMismatchIfTotalMatch {
			if md := albumMetaDistance(m, artist, album, mbReleaseID, year, data); md <= mediumThresh {
				candidates = append(candidates, scored{m, md})
			}
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
	localTotalMs int64,
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
		isAllowedExtra := data != nil && data.AllowExtraTracks && m.ChapterCount <= fileCount
		if ((isExact || isAllowedMissing || isAllowedExtra) && d <= exactChapterHardCap) || d <= mediumThresh {
			candidates = append(candidates, scored{m, d})
			continue
		}

		// Files cut differently than the chapters: title and author must match
		// on their own, and a stored runtime must fit. The candidate pass then
		// checks the total runtime against the chapters.
		if fileCount > 1 && m.ChapterCount > 0 && !isExact {
			md := audiobookMetaDistance(m, title, author)
			if md > mediumThresh {
				continue
			}

			// The stored runtime is in whole minutes; the candidate pass checks
			// the exact chapter runtimes.
			if m.Runtime > 0 {
				diff := localTotalMs - int64(m.Runtime)*60_000
				if diff < 0 {
					diff = -diff
				}

				if localTotalMs <= 0 || diff > repackToleranceMs(m.ChapterCount, data)+60_000 {
					continue
				}
			}

			candidates = append(candidates, scored{m, md})
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

package database

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

// Series identified by "title" have releases that name an episode only by its
// title ("Terra.X.Der.grosse.Bluff.2018.GERMAN.DOKU..."). MatchEpisodeByTitle
// finds such a series by its name at the start of the release name and the
// episode by the words that follow.

// titleMatchMin is the share of the release's words an episode title must
// contain.
const titleMatchMin = 0.75

var (
	// titleCutWord ends the episode part of a release name: language,
	// source, resolution and codec tags.
	titleCutWord = regexp.MustCompile(
		`^(german|deutsch|english|multi|dubbed|doku|hdtv|hdtvrip|webrip|webdl|web|bdrip|bluray|dvdrip|hdrip|uhd|x264|x265|h264|h265|hevc|avc|xvid|ws|repack|proper|internal|ac3.*|\d{3,4}[pi])$`,
	)
	// titleNumberWord is an episode number the parser couldn't use
	// (S2026E39 that matches no episode, E01).
	titleNumberWord = regexp.MustCompile(`^(s\d+e\d+|s\d+|e\d+|\d+x\d+)$`)
	// titleAbsoluteWord is an absolute episode number: "E1322", "E0705",
	// "S01E1311" or a bare "1151" ("Tatort - 1151 - Der feine Geist"). A
	// three-digit SxxEyyy ("S01E133") is a season's episode, not absolute.
	titleAbsoluteWord = regexp.MustCompile(`^(?:s\d+e(\d{4})|e?(\d{3,4}))$`)
	titleYearWord     = regexp.MustCompile(`^(19|20)\d\d$`)
	// titleFillerWords don't tell episodes apart.
	titleFillerWords = []string{
		"der", "die", "das", "und", "ein", "eine", "einer", "im", "in", "am", "an", "auf",
		"zum", "zur", "mit", "von", "vom", "des", "dem", "den", "um", "aus",
		"the", "a", "of", "and", "to", "for", "on", "at", "is",
	}
)

type titleSerie struct {
	id    uint
	names [][]string
}

var titleSeriesCache struct {
	sync.Mutex
	at   time.Time
	list []titleSerie
}

// titleSeries returns the series identified by title with the words of their
// names and alternate names, cached for five minutes: it is asked for every
// release no series name matches.
func titleSeries() []titleSerie {
	titleSeriesCache.Lock()
	defer titleSeriesCache.Unlock()

	if time.Since(titleSeriesCache.at) < 5*time.Minute {
		return titleSeriesCache.list
	}

	rows := GetrowsN[DbstaticOneStringOneUInt](false, 10,
		`select seriename as str, id as num from dbseries where lower(identifiedby) = ?
		 union all
		 select a.title, a.dbserie_id from dbserie_alternates a inner join dbseries s on s.id = a.dbserie_id where lower(s.identifiedby) = ?`,
		logger.StrByTitle, logger.StrByTitle)

	var list []titleSerie

	for _, r := range rows {
		words := titleWords(r.Str)
		if len(words) == 0 {
			continue
		}

		i := slices.IndexFunc(list, func(s titleSerie) bool { return s.id == r.Num })
		if i < 0 {
			list = append(list, titleSerie{id: r.Num})
			i = len(list) - 1
		}

		list[i].names = append(list[i].names, words)
	}

	titleSeriesCache.list, titleSeriesCache.at = list, time.Now()

	return list
}

// titleWords are the lower-case words of s, umlauts spelled out (ü = ue).
func titleWords(s string) []string {
	return strings.FieldsFunc(logger.StringToSlug(s), func(r rune) bool { return r == '-' })
}

// releaseWords splits a release name into its words up to the first tag, and
// the year and absolute episode number found among them.
func releaseWords(name string) (words []string, hint releaseHint) {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}

	if ext := filepath.Ext(name); len(ext) >= 3 && len(ext) <= 5 && !strings.ContainsAny(ext, " -") {
		name = strings.TrimSuffix(name, ext)
	}

	for i, w := range titleWords(name) {
		switch {
		case i > 0 && titleCutWord.MatchString(w):
			return words, hint
		case i > 0 && titleYearWord.MatchString(w):
			hint.year = w
		case i > 0 && titleAbsoluteWord.MatchString(w):
			sub := titleAbsoluteWord.FindStringSubmatch(w)
			hint.absolute = logger.StringToInt(sub[1] + sub[2])
		case titleNumberWord.MatchString(w):
		default:
			words = append(words, w)
		}
	}

	return words, hint
}

// cutSeriesName returns the words after the longest series name the release
// starts with (none left when it names only a number), and the length of that
// name (0: none).
func cutSeriesName(words []string, names [][]string) ([]string, int) {
	best := 0

	for _, n := range names {
		if len(n) > best && len(n) <= len(words) && slices.Equal(words[:len(n)], n) {
			best = len(n)
		}
	}

	return words[best:], best
}

type titleEpisode struct {
	ID         uint   `db:"id"`
	Title      string `db:"title"`
	Season     string `db:"season"`
	Episode    string `db:"episode"`
	Identifier string `db:"identifier"`
	FirstAired string `db:"first_aired"`
	Absolute   int    `db:"absolute_episode"`
}

// titleEp is an episode with the words of its title, split once.
type titleEp struct {
	titleEpisode
	words []string
	own   int // title words without filler words (all when that leaves none)
}

func newTitleEp(e titleEpisode) titleEp {
	ep := titleEp{titleEpisode: e, words: titleWords(e.Title)}
	for _, w := range ep.words {
		if !slices.Contains(titleFillerWords, w) {
			ep.own++
		}
	}

	if ep.own == 0 {
		ep.own = len(ep.words)
	}

	return ep
}

const titleEpisodesSQL = "select id, title, season, episode, identifier, first_aired, absolute_episode from dbserie_episodes"

// titleEpisodesCache holds the episodes of the series identified by title
// (a few series, but Tatort alone has over 1300 episodes): every release of
// them is matched against all episodes.
var titleEpisodesCache struct {
	sync.Mutex
	series map[uint]*titleEpisodeSet
}

type titleEpisodeSet struct {
	at         time.Time
	count, max uint
	eps        []titleEp
}

// titleEpisodes returns the episodes of serie, read again when episodes were
// added or removed, and at least every five minutes for changed titles. The
// slice is shared: callers must not change it.
func titleEpisodes(serie uint) []titleEp {
	var state DbstaticTwoUint
	if rows := GetrowsN[DbstaticTwoUint](false, 1,
		"select count(*) as num1, coalesce(max(id), 0) as num2 from dbserie_episodes where dbserie_id = ?", &serie); len(rows) == 1 {
		state = rows[0]
	}

	titleEpisodesCache.Lock()
	defer titleEpisodesCache.Unlock()

	if set := titleEpisodesCache.series[serie]; set != nil && set.count == state.Num1 && set.max == state.Num2 &&
		time.Since(set.at) < 5*time.Minute {
		return set.eps
	}

	rows := GetrowsN[titleEpisode](false, state.Num1, titleEpisodesSQL+" where dbserie_id = ?", &serie)

	eps := make([]titleEp, len(rows))
	for i := range rows {
		eps[i] = newTitleEp(rows[i])
	}

	if titleEpisodesCache.series == nil {
		titleEpisodesCache.series = make(map[uint]*titleEpisodeSet)
	}

	titleEpisodesCache.series[serie] = &titleEpisodeSet{at: time.Now(), count: state.Num1, max: state.Num2, eps: eps}

	return eps
}

// titleScore is how well an episode title fits the release words: the share
// of the release words it contains, whether the year and the season fit and
// the share of its own words the release names.
type titleScore struct {
	cover, own   float64
	hits         int
	year, season bool
}

func (a titleScore) better(b titleScore) bool {
	if a.cover != b.cover {
		return a.cover > b.cover
	}

	if a.year != b.year {
		return a.year
	}

	if a.season != b.season {
		return a.season
	}

	return a.own > b.own
}

// releaseHint is what else the release says: its year, the season of an
// episode number that named no episode (or the wrong one) and an absolute
// episode number.
type releaseHint struct {
	year, season string
	absolute     int
}

func scoreEpisodeTitle(words []string, hint releaseHint, ep *titleEp) titleScore {
	title := ep.words
	hits := 0

	for _, w := range words {
		if slices.Contains(title, w) || joinsTwo(title, w) {
			hits++
		}
	}

	return titleScore{
		cover: float64(hits) / float64(len(words)),
		hits:  hits,
		own:   float64(min(hits, ep.own)) / float64(max(ep.own, 1)),
		year: hint.year != "" && (ep.Season == hint.year ||
			(len(ep.FirstAired) >= 4 && ep.FirstAired[:4] == hint.year)),
		season: hint.season != "" && strings.TrimLeft(ep.Season, "0") == strings.TrimLeft(hint.season, "0"),
	}
}

// significantWords drops the filler words, unless that leaves none.
func significantWords(words []string) []string {
	sig := slices.DeleteFunc(slices.Clone(words), func(w string) bool { return slices.Contains(titleFillerWords, w) })
	if len(sig) == 0 {
		return words
	}

	return sig
}

// joinsTwo reports whether w is two neighbouring title words written as one
// ("Eiszeitraetsel" for "Eiszeit-Rätsel").
func joinsTwo(title []string, w string) bool {
	for i := 0; i+1 < len(title); i++ {
		if a, b := title[i], title[i+1]; len(a)+len(b) == len(w) && w[:len(a)] == a && w[len(a):] == b {
			return true
		}
	}

	return false
}

// bestEpisodeByTitle picks the episode whose title fits the words best. It
// fails when none covers titleMatchMin of them, when one shared word is all
// (unless it is the whole title) or when two fit equally well. An absolute
// episode number that names exactly one episode decides, unless the words
// contradict it ("Tatort.E0705.Liebeswirren": "Lannert - 05 - Liebeswirren").
func bestEpisodeByTitle(words []string, hint releaseHint, eps []titleEp) (*titleEp, bool) {
	sig := significantWords(words)

	if hint.absolute > 0 {
		var hit *titleEp

		n := 0

		for i := range eps {
			if eps[i].Absolute == hint.absolute {
				hit = &eps[i]
				n++
			}
		}

		if n == 1 && (len(sig) == 0 || scoreEpisodeTitle(sig, hint, hit).cover >= 0.5) {
			return hit, true
		}
	}

	if len(sig) == 0 {
		return nil, false
	}

	var (
		best        *titleEp
		first, next titleScore
	)

	for i := range eps {
		if strings.TrimSpace(eps[i].Title) == "" {
			continue
		}

		sc := scoreEpisodeTitle(sig, hint, &eps[i])
		switch {
		case best == nil || sc.better(first):
			best, first, next = &eps[i], sc, first
		case sc.better(next):
			next = sc
		}
	}

	if best == nil || first.cover < titleMatchMin || (first.hits < 2 && first.own < 1) || !first.better(next) {
		return nil, false
	}

	return best, true
}

// titleRelease finds the series identified by title the release belongs to
// (DbserieID, or the one whose name it starts with) and returns the words
// after the series name; serie is 0 when there is none.
func (m *ParseInfo) titleRelease() (rest []string, serie uint, hint releaseHint) {
	name := m.File
	if name == "" {
		name = m.Title
	}

	words, hint := releaseWords(name)
	if m.Year != 0 && hint.year == "" {
		hint.year = logger.IntToString(m.Year)
	}

	hint.season = m.SeasonStr

	length := 0

	for _, s := range titleSeries() {
		if m.DbserieID != 0 && s.id != m.DbserieID {
			continue
		}

		if r, n := cutSeriesName(words, s.names); n > length {
			rest, serie, length = r, s.id, n
		}
	}

	return rest, serie, hint
}

// EpisodeTitleFits reports whether the episode an episode number found
// (DbserieEpisodeID) is the one the release names. It is false only for a
// series identified by title whose release names an episode title that
// doesn't fit that episode: release and database numbering differ
// ("Rosamunde.Pilcher.S01E139.Geerbtes.Glueck" vs S01E139 "Schutzengel").
func (m *ParseInfo) EpisodeTitleFits() bool {
	if m.DbserieEpisodeID == 0 {
		return true
	}

	rest, serie, hint := m.titleRelease()
	if serie == 0 || len(rest) == 0 {
		return true
	}

	eps := titleEpisodes(serie)

	i := slices.IndexFunc(eps, func(e titleEp) bool { return e.ID == m.DbserieEpisodeID })
	if i < 0 {
		// An episode of another series than the one the name says.
		rows := GetrowsN[titleEpisode](false, 1, titleEpisodesSQL+" where id = ?", &m.DbserieEpisodeID)
		if len(rows) == 0 {
			return true
		}

		ep := newTitleEp(rows[0])

		return scoreEpisodeTitle(significantWords(rest), hint, &ep).cover >= titleMatchMin
	}

	return scoreEpisodeTitle(significantWords(rest), hint, &eps[i]).cover >= titleMatchMin
}

// MatchEpisodeByTitle matches a release without a usable episode number to
// an episode of a series identified by title. Without DbserieID the series is
// the one whose name the release starts with. On a match it sets the episode
// and its identifier (S2026E39), so the rest goes on as for an S01E01 release,
// and TitleMatched.
func (m *ParseInfo) MatchEpisodeByTitle() bool {
	rest, serie, hint := m.titleRelease()
	if serie == 0 {
		return false
	}

	ep, ok := bestEpisodeByTitle(rest, hint, titleEpisodes(serie))
	if !ok {
		return false
	}

	m.DbserieID, m.DbserieEpisodeID = serie, ep.ID
	m.Identifier, m.SeasonStr, m.EpisodeStr = ep.Identifier, ep.Season, ep.Episode
	m.Season, m.Episode = logger.StringToInt(ep.Season), logger.StringToInt(ep.Episode)
	m.TitleMatched = true

	return true
}

package api

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
	"github.com/gin-gonic/gin"
)

// Audio tracks and subtitles of videos. An audio track other than the
// first is mapped by ffmpeg (a=N, the file is repackaged or converted).
// Text subtitles, in the file or in files next to it, go to the browser as
// cues (sub endpoint) and are shown in the video's text track, shifted to
// where a converted stream starts. Picture subtitles (Blu-ray, DVD) can't be
// shown as text: they are burned into a converted stream (burn=N).

// playTrack is an audio or subtitle stream of a file; Index counts per
// stream type (ffmpeg's 0:a:N / 0:s:N).
type playTrack struct {
	Index    int
	Lang     string // two-letter code when known, else as tagged
	Title    string
	Codec    string
	Channels int
	Default  bool
	Forced   bool
}

// imageSubCodecs can only be burned in.
var imageSubCodecs = map[string]bool{"hdmv_pgs_subtitle": true, "dvd_subtitle": true, "dvb_subtitle": true, "xsub": true}

// textSubCodecs convert to cues.
var textSubCodecs = map[string]bool{"subrip": true, "srt": true, "ass": true, "ssa": true, "mov_text": true, "webvtt": true, "text": true}

var subFileExts = map[string]bool{".srt": true, ".vtt": true, ".ass": true, ".ssa": true}

var videoFileExts = map[string]bool{
	".mkv": true, ".mp4": true, ".m4v": true, ".avi": true, ".webm": true, ".mov": true, ".wmv": true,
	".ts": true, ".m2ts": true, ".mpg": true, ".mpeg": true, ".divx": true, ".flv": true,
}

var langNames = map[string]string{
	"en": "English", "de": "German", "fr": "French", "es": "Spanish", "it": "Italian", "ja": "Japanese", "nl": "Dutch",
	"pt": "Portuguese", "ru": "Russian", "pl": "Polish", "sv": "Swedish", "da": "Danish", "no": "Norwegian",
	"fi": "Finnish", "tr": "Turkish", "ko": "Korean", "zh": "Chinese", "cs": "Czech", "hu": "Hungarian", "el": "Greek",
	"ar": "Arabic", "he": "Hebrew", "hi": "Hindi",
}

// langCode normalises a language tag ("eng", "English", "en") to two
// letters; "" for undetermined.
func langCode(s string) string {
	return config.LanguageCode(s)
}

// langLabel is a language name (langNames) for the page; trackLabel's
// parameter t hides t().
func langLabel(name string) string {
	return t(name)
}

// trackNumberLabel is "Track 3" for a track without a language.
func trackNumberLabel(n int) string {
	return tf("Track %d", n)
}

// trackLabel is "English · AC3 5.1 · Director's comments".
func trackLabel(t playTrack, n int, audio bool) string {
	parts := []string{}

	switch name := langNames[t.Lang]; {
	case name != "":
		parts = append(parts, langLabel(name))
	case t.Lang != "":
		parts = append(parts, t.Lang)
	default:
		parts = append(parts, trackNumberLabel(n+1))
	}

	if audio && t.Codec != "" {
		codec := strings.ToUpper(t.Codec)
		if t.Channels > 0 {
			codec += " " + map[int]string{1: "mono", 2: "stereo", 6: "5.1", 8: "7.1"}[t.Channels]
		}

		parts = append(parts, strings.TrimSpace(codec))
	}

	if t.Title != "" && !strings.EqualFold(t.Title, langNames[t.Lang]) {
		parts = append(parts, t.Title)
	}

	return strings.Join(parts, " · ")
}

// playAudioInfo and playSubInfo are what the player lists.
type playAudioInfo struct {
	Index   int    `json:"index"`
	Label   string `json:"label"`
	Lang    string `json:"lang"`
	Default bool   `json:"default"`
}

type playSubInfo struct {
	ID     string `json:"id"` // sN: stream N of the file, xN: file N next to it
	Label  string `json:"label"`
	Lang   string `json:"lang"`
	Forced bool   `json:"forced"`
	Image  bool   `json:"image"` // burned in (the stream is converted)
}

func audioInfos(p playProbe) []playAudioInfo {
	out := make([]playAudioInfo, 0, len(p.AudioTracks))
	for i, t := range p.AudioTracks {
		out = append(out, playAudioInfo{Index: t.Index, Label: trackLabel(t, i, true), Lang: t.Lang, Default: t.Default})
	}

	return out
}

func subInfos(path string, p playProbe) []playSubInfo {
	out := []playSubInfo{}

	for i, t := range p.SubTracks {
		if !imageSubCodecs[t.Codec] && !textSubCodecs[t.Codec] {
			continue
		}

		label := trackLabel(t, i, false)
		if t.Forced {
			label += " (forced)"
		}

		if imageSubCodecs[t.Codec] {
			label += " – burned in"
		}

		out = append(out, playSubInfo{ID: "s" + strconv.Itoa(t.Index), Label: label, Lang: t.Lang, Forced: t.Forced, Image: imageSubCodecs[t.Codec]})
	}

	for i, f := range externalSubs(path) {
		label := f.Label
		if name := langNames[f.Lang]; name != "" {
			label = langLabel(name) + " (" + f.Label + ")"
		}

		out = append(out, playSubInfo{ID: "x" + strconv.Itoa(i), Label: label + t(" – file"), Lang: f.Lang, Forced: f.Forced})
	}

	return out
}

// extSub is a subtitle file next to a video.
type extSub struct {
	Path   string
	Label  string // the part of the name after the video's, e.g. "en.forced"
	Lang   string
	Forced bool
}

// externalSubs finds subtitle files of a video: named like it ("Movie.en.srt")
// in its folder or a Subs folder, all files in Subs/<video name>/, and any
// subtitle file when the folder holds only this video.
func externalSubs(video string) []extSub {
	dir := filepath.Dir(video)
	base := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))
	lbase := strings.ToLower(base)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	videos := 0

	var subsDir string

	for _, e := range entries {
		switch {
		case e.IsDir() && strings.EqualFold(e.Name(), "subs"):
			subsDir = filepath.Join(dir, e.Name())
		case !e.IsDir() && videoFileExts[strings.ToLower(filepath.Ext(e.Name()))]:
			videos++
		}
	}

	var out []extSub

	add := func(folder, name string, any bool) {
		ext := strings.ToLower(filepath.Ext(name))
		if !subFileExts[ext] {
			return
		}

		stem := strings.TrimSuffix(name, filepath.Ext(name))

		rest := stem
		if strings.HasPrefix(strings.ToLower(stem), lbase) {
			rest = strings.Trim(stem[len(base):], ". _-")
		} else if !any {
			return
		}

		s := extSub{Path: filepath.Join(folder, name), Label: rest}
		if s.Label == "" {
			s.Label = strings.TrimPrefix(ext, ".")
		}

		for _, tok := range strings.FieldsFunc(strings.ToLower(rest), func(r rune) bool { return r == '.' || r == '_' || r == '-' || r == ' ' }) {
			switch {
			case tok == "forced":
				s.Forced = true
			case s.Lang == "" && (config.IsLanguageName(tok) || langNames[tok] != ""):
				s.Lang = langCode(tok)
			}
		}

		out = append(out, s)
	}

	for _, e := range entries {
		if !e.IsDir() {
			add(dir, e.Name(), videos <= 1)
		}
	}

	if subsDir != "" {
		if sub, err := os.ReadDir(subsDir); err == nil {
			for _, e := range sub {
				if e.IsDir() && strings.EqualFold(e.Name(), base) {
					if inner, err := os.ReadDir(filepath.Join(subsDir, e.Name())); err == nil {
						for _, f := range inner {
							if !f.IsDir() {
								add(filepath.Join(subsDir, e.Name()), f.Name(), true)
							}
						}
					}

					continue
				}

				if !e.IsDir() {
					add(subsDir, e.Name(), videos <= 1)
				}
			}
		}
	}

	slices.SortStableFunc(out, func(a, b extSub) int { return strings.Compare(a.Path, b.Path) })

	return out
}

// playSel is the audio track and burned-in subtitle of a stream.
type playSel struct {
	Audio int // 0:a:N
	Burn  int // 0:s:N burned in, -1 none
}

// defaultSel plays the first audio track without burned-in subtitles.
var defaultSel = playSel{Burn: -1}

// isDefault: the file can be sent as it is.
func (s playSel) isDefault() bool { return s.Audio == 0 && s.Burn < 0 }

// selFromQuery reads a= and burn= and checks them against the file.
func selFromQuery(c *gin.Context, p playProbe) (playSel, error) {
	s := defaultSel

	if v := c.Query("a"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= max(len(p.AudioTracks), 1) {
			return s, fmt.Errorf("unknown audio track %q", v)
		}

		s.Audio = n
	}

	if v := c.Query("burn"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 || n >= len(p.SubTracks) || !imageSubCodecs[p.SubTracks[n].Codec] {
			return s, fmt.Errorf("unknown picture subtitle %q", v)
		}

		s.Burn = n
	}

	return s, nil
}

// playCue is one subtitle line, in seconds of the file.
type playCue struct {
	Start float64 `json:"s"`
	End   float64 `json:"e"`
	Text  string  `json:"t"`
}

// subCacheDir holds converted subtitles, next to the cover cache.
func subCacheDir() string {
	return filepath.Join(filepath.Dir(filepath.Clean(coverCacheDir())), "subs")
}

// HandlePlayerSub returns the cues of a text subtitle
// (/admin/library/play/:kind/:id/sub/:sid).
func HandlePlayerSub(c *gin.Context) {
	path, pk, ok := playFileFor(c)
	if !ok || pk.Audio {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	sid := c.Param("sid")

	n, err := strconv.Atoi(sid[min(1, len(sid)):])
	if err != nil || n < 0 || len(sid) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown subtitle"})
		return
	}

	// src is the file ffmpeg reads; mapArg picks the stream, all are the
	// streams converted with it.
	var (
		src, mapArg string
		all         []string
	)

	switch sid[0] {
	case 's':
		p, err := probeMedia(c.Request.Context(), path)
		if err != nil || n >= len(p.SubTracks) || !textSubCodecs[p.SubTracks[n].Codec] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown subtitle"})
			return
		}

		src, mapArg = path, "0:s:"+strconv.Itoa(n)

		for _, t := range p.SubTracks {
			if textSubCodecs[t.Codec] {
				all = append(all, "0:s:"+strconv.Itoa(t.Index))
			}
		}
	case 'x':
		subs := externalSubs(path)
		if n >= len(subs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unknown subtitle"})
			return
		}

		src, mapArg = subs[n].Path, "0:s:0"
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown subtitle"})
		return
	}

	cues, err := subtitleCues(c.Request.Context(), src, mapArg, all)
	if err != nil {
		logger.Logtype("warn", 0).Err(err).Str("file", src).Msg("Player: subtitle conversion failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": tf("The subtitle can't be read: %s", err.Error())})

		return
	}

	c.JSON(http.StatusOK, gin.H{"cues": cues})
}

// subSourceExt is the file next to a source's cached subtitles that holds
// the source path.
const subSourceExt = ".src"

// subSourceKey names the cache files of one source file.
func subSourceKey(src string) string {
	sum := sha1.Sum([]byte(src))
	return hex.EncodeToString(sum[:8])
}

// subCacheName is the cache file of stream mapArg of src as modified at
// mod: <source key>-<mod time ns>-<stream hash>.json, so the cleanup finds
// the conversions of removed and replaced files.
func subCacheName(src string, mod time.Time, mapArg string) string {
	sum := sha1.Sum([]byte(mapArg))
	return subSourceKey(src) + "-" + strconv.FormatInt(mod.UnixNano(), 10) + "-" + hex.EncodeToString(sum[:6]) + ".json"
}

// subLocks serialises the conversions of one file, so a second request
// waits for the first and then reads the cache.
var subLocks sync.Map // source path -> *sync.Mutex

// subtitleCues returns the cues of stream want of src, converted with
// ffmpeg and cached by file, modification time and stream. Reading a
// stream of a large file means reading all of it, so the streams in all
// are converted in the same pass.
func subtitleCues(ctx context.Context, src, want string, all []string) ([]playCue, error) {
	st, err := os.Stat(src)
	if err != nil {
		return nil, err
	}

	cacheFile := func(mapArg string) string {
		return filepath.Join(subCacheDir(), subCacheName(src, st.ModTime(), mapArg))
	}

	cached := func(mapArg string) ([]playCue, bool) {
		var cues []playCue

		b, err := os.ReadFile(cacheFile(mapArg))

		return cues, err == nil && json.Unmarshal(b, &cues) == nil
	}

	if cues, ok := cached(want); ok {
		return cues, nil
	}

	mu, _ := subLocks.LoadOrStore(src, &sync.Mutex{})
	mu.(*sync.Mutex).Lock()
	defer mu.(*sync.Mutex).Unlock()

	if cues, ok := cached(want); ok {
		return cues, nil
	}

	missing := []string{want}

	for _, m := range all {
		if _, ok := cached(m); !ok && m != want {
			missing = append(missing, m)
		}
	}

	if err := os.MkdirAll(subCacheDir(), 0o755); err != nil {
		return nil, err
	}

	// The source path, for the cache cleanup (the names only hold its hash).
	if err := os.WriteFile(filepath.Join(subCacheDir(), subSourceKey(src)+subSourceExt), []byte(src), 0o644); err != nil {
		return nil, err
	}

	err = convertSubs(ctx, src, missing, cacheFile)
	if err != nil && len(missing) > 1 {
		// One broken stream shouldn't keep the wanted one from working.
		err = convertSubs(ctx, src, missing[:1], cacheFile)
	}

	if err != nil {
		return nil, err
	}

	cues, ok := cached(want)
	if !ok {
		return nil, fmt.Errorf("no subtitle written")
	}

	return cues, nil
}

// convertSubs converts streams of src to SRT in one ffmpeg run and stores
// their cues in cacheFile(stream).
func convertSubs(ctx context.Context, src string, streams []string, cacheFile func(string) string) error {
	ffmpeg, err := tags.ResolveFFmpeg()
	if err != nil {
		return err
	}

	tmp, err := os.MkdirTemp(subCacheDir(), "convert-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	args := []string{"-hide_banner", "-loglevel", "error", "-nostdin", "-y"}

	// Subtitle files are often not UTF-8 (Windows-1252 is the usual
	// European one); ffmpeg assumes UTF-8.
	if subFileExts[strings.ToLower(filepath.Ext(src))] {
		if b, err := os.ReadFile(src); err == nil && !utf8.Valid(b) {
			args = append(args, "-sub_charenc", "CP1252")
		}
	}

	args = append(args, "-i", src)
	for i, m := range streams {
		args = append(args, "-map", m, "-f", "srt", filepath.Join(tmp, strconv.Itoa(i)+".srt"))
	}

	// Not bound to the request: a closed player keeps the result for next time.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Minute)
	defer cancel()

	if out, err := exec.CommandContext(ctx, ffmpeg, args...).CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}

	for i, m := range streams {
		b, err := os.ReadFile(filepath.Join(tmp, strconv.Itoa(i)+".srt"))
		if err != nil {
			return err
		}

		j, err := json.Marshal(parseSRT(string(b)))
		if err != nil {
			return err
		}

		if err := os.WriteFile(cacheFile(m), j, 0o644); err != nil {
			return err
		}
	}

	return nil
}

var (
	srtTimeRe = regexp.MustCompile(`(\d+):(\d{2}):(\d{2})[,.](\d{3})\s*-->\s*(\d+):(\d{2}):(\d{2})[,.](\d{3})`)
	cueTagRe  = regexp.MustCompile(`</?([a-zA-Z]+)[^>]*>`)
	assTagRe  = regexp.MustCompile(`\{\\[^}]*\}`)
)

func srtSeconds(h, m, s, ms string) float64 {
	hv, _ := strconv.Atoi(h)
	mv, _ := strconv.Atoi(m)
	sv, _ := strconv.Atoi(s)
	msv, _ := strconv.Atoi(ms)

	return float64(hv*3600+mv*60+sv) + float64(msv)/1000
}

// parseSRT reads ffmpeg's SRT output into cues.
func parseSRT(s string) []playCue {
	cues := []playCue{}
	sc := bufio.NewScanner(strings.NewReader(strings.ReplaceAll(s, "\r\n", "\n")))
	sc.Buffer(make([]byte, 64<<10), 1<<20)

	var (
		cur   *playCue
		lines []string
	)

	flush := func() {
		if cur != nil {
			if t := cueText(strings.Join(lines, "\n")); t != "" {
				cur.Text = t
				cues = append(cues, *cur)
			}
		}

		cur, lines = nil, nil
	}

	for sc.Scan() {
		line := sc.Text()

		if m := srtTimeRe.FindStringSubmatch(line); m != nil {
			// A number line ahead of the times belongs to the next cue.
			if cur != nil && len(lines) > 0 {
				if _, err := strconv.Atoi(strings.TrimSpace(lines[len(lines)-1])); err == nil {
					lines = lines[:len(lines)-1]
				}
			}

			flush()

			cur = &playCue{Start: srtSeconds(m[1], m[2], m[3], m[4]), End: srtSeconds(m[5], m[6], m[7], m[8])}

			continue
		}

		if cur == nil {
			continue
		}

		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}

		lines = append(lines, line)
	}

	flush()

	return cues
}

var cueEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// cueText keeps <i>, <b> and <u> and escapes everything else, as WebVTT
// cue text expects.
func cueText(s string) string {
	s = assTagRe.ReplaceAllString(s, "")

	var b strings.Builder

	last := 0

	for _, m := range cueTagRe.FindAllStringSubmatchIndex(s, -1) {
		b.WriteString(cueEscaper.Replace(s[last:m[0]]))

		switch tag := strings.ToLower(s[m[2]:m[3]]); tag {
		case "i", "b", "u":
			if strings.HasPrefix(s[m[0]:], "</") {
				b.WriteString("</" + tag + ">")
			} else {
				b.WriteString("<" + tag + ">")
			}
		}

		last = m[1]
	}

	b.WriteString(cueEscaper.Replace(s[last:]))

	return strings.TrimSpace(b.String())
}

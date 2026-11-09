package parser

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/whisper"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
)

const (
	langClipSeconds = 30
	// langMinConfidence drops clips Whisper is unsure about; music or credits
	// typically come back below 0.5 (a German film's English end-credits song
	// scored 0.44 as English in testing), speech above 0.8.
	langMinConfidence = 0.7
	// langMinVotes is how many confident clips must agree on a language.
	langMinVotes = 2
)

// langClipPositions are where clips are cut, as a share of the runtime.
// Openings and end credits are avoided: they are often music only.
var langClipPositions = []float64{0.25, 0.5, 0.7}

// langDetector is what the detection needs from the Whisper client.
type langDetector interface {
	DetectLanguage(ctx context.Context, wav []byte) (*whisper.Detection, error)
}

// getLangDetector returns the configured Whisper client, or nil. Tests
// replace it.
var getLangDetector = func() langDetector {
	if p := providers.GetWhisper(); p != nil {
		return p
	}

	return nil
}

// languageAliases maps ISO 639-1 codes to the other names a language appears
// under in allowed_languages and in audio track tags: ISO 639-2/B, 639-2/T,
// English name and native name.
var languageAliases = map[string][]string{
	"ar": {"ara", "Arabic"},
	"cs": {"cze", "ces", "Czech", "Čeština"},
	"da": {"dan", "Danish", "Dansk"},
	"de": {"ger", "deu", "German", "Deutsch"},
	"el": {"gre", "ell", "Greek"},
	"en": {"eng", "English"},
	"es": {"spa", "Spanish", "Español"},
	"fi": {"fin", "Finnish", "Suomi"},
	"fr": {"fre", "fra", "French", "Français"},
	"he": {"heb", "Hebrew"},
	"hi": {"hin", "Hindi"},
	"hu": {"hun", "Hungarian", "Magyar"},
	"it": {"ita", "Italian", "Italiano"},
	"ja": {"jpn", "Japanese"},
	"ko": {"kor", "Korean"},
	"nl": {"dut", "nld", "Dutch", "Nederlands"},
	"no": {"nor", "Norwegian", "Norsk"},
	"pl": {"pol", "Polish", "Polski"},
	"pt": {"por", "Portuguese", "Português"},
	"ro": {"rum", "ron", "Romanian", "Română"},
	"ru": {"rus", "Russian"},
	"sv": {"swe", "Swedish", "Svenska"},
	"th": {"tha", "Thai"},
	"tr": {"tur", "Turkish", "Türkçe"},
	"uk": {"ukr", "Ukrainian"},
	"zh": {"chi", "zho", "Chinese"},
}

// languageNames returns code with all its aliases; for a code without an
// entry, code and the English name Whisper reported.
func languageNames(code, name string) []string {
	if a, ok := languageAliases[code]; ok {
		return append([]string{code}, a...)
	}

	if name == "" {
		return []string{code}
	}

	return []string{code, name}
}

// trackLanguageTag returns how a detected language is recorded as the track's
// language: the ISO 639-2/B code most files are tagged with (e.g. "ger"), or
// the ISO 639-1 code for a language without an entry.
func trackLanguageTag(code string) string {
	if a, ok := languageAliases[code]; ok {
		return a[0]
	}

	return code
}

// untaggedLanguage reports whether an audio track tag says nothing about the
// language.
func untaggedLanguage(tag string) bool {
	return tag == "" || strings.EqualFold(tag, "und")
}

// DetectAudioLanguages identifies the audio tracks of file that have no
// language tag (or "und") by listening, through the Whisper service from
// whisper_url. languages is the file's track language list with each
// identified track tagged (see trackLanguageTag); names are all names of the
// identified languages (see languageAliases), for matching allowed_languages
// in whatever form it lists them. Both are nil when no track was identified:
// no untagged tracks, no whisper_url, or no confident result.
func DetectAudioLanguages(ctx context.Context, file string) (languages, names []string, err error) {
	client := getLangDetector()
	if client == nil {
		return nil, nil, nil
	}

	duration, tags, err := probeAudioTags(ctx, file)
	if err != nil {
		return nil, nil, err
	}

	for track, tag := range tags {
		if !untaggedLanguage(tag) {
			languages = append(languages, tag)
			continue
		}

		code, name, err := detectTrackLanguage(ctx, client, file, track, duration)
		if err != nil {
			return nil, nil, err
		}

		if code == "" {
			if tag != "" {
				languages = append(languages, tag)
			}

			continue
		}

		logger.Logtype("info", 1).
			Str(logger.StrFile, file).
			Int("track", track).
			Str("language", code).
			Msg("Detected audio language")

		languages = append(languages, trackLanguageTag(code))
		names = append(names, languageNames(code, name)...)
	}

	if len(names) == 0 {
		return nil, nil, nil
	}

	return languages, names, nil
}

// probeAudioTags returns the runtime in seconds and the language tag of each
// audio track, in stream order.
func probeAudioTags(ctx context.Context, file string) (float64, []string, error) {
	out, err := exec.CommandContext(ctx, getFFProbeFilename(), "-v", "error",
		"-select_streams", "a", "-show_entries", "stream_tags=language:format=duration",
		"-of", "json", file).Output()
	if err != nil {
		return 0, nil, fmt.Errorf("ffprobe audio tracks: %w", err)
	}

	var res ffProbeJSON
	if err := json.Unmarshal(out, &res); err != nil {
		return 0, nil, fmt.Errorf("ffprobe audio tracks: %w", err)
	}

	duration, _ := strconv.ParseFloat(res.Format.Duration, 64)

	tags := make([]string, len(res.Streams))
	for i := range res.Streams {
		tags[i] = res.Streams[i].language()
	}

	return duration, tags, nil
}

// detectTrackLanguage cuts clips from audio track track and returns the
// language at least langMinVotes confident clips agree on, or "" when there
// is none.
func detectTrackLanguage(
	ctx context.Context,
	client langDetector,
	file string,
	track int,
	duration float64,
) (code, name string, err error) {
	votes := make(map[string]int, len(langClipPositions))
	names := make(map[string]string, len(langClipPositions))

	for _, pos := range clipStarts(duration) {
		wav, err := extractClip(ctx, file, track, pos)
		if err != nil {
			return "", "", err
		}

		det, err := client.DetectLanguage(ctx, wav)
		if err != nil {
			return "", "", fmt.Errorf("whisper: %w", err)
		}

		if det.Confidence < langMinConfidence {
			continue
		}

		votes[det.Code]++
		names[det.Code] = det.Name
	}

	code = pickLanguage(votes)

	return code, names[code], nil
}

// pickLanguage returns the language with the most votes if it has at least
// langMinVotes and no other language has as many.
func pickLanguage(votes map[string]int) string {
	var best string

	most, tie := 0, false

	for code, n := range votes {
		switch {
		case n > most:
			best, most, tie = code, n, false
		case n == most:
			tie = true
		}
	}

	if most < langMinVotes || tie {
		return ""
	}

	return best
}

// clipStarts returns the start offsets in seconds of the clips to cut from a
// file of duration seconds. Files too short for separate clips get one clip
// from the start.
func clipStarts(duration float64) []float64 {
	if duration < float64(len(langClipPositions)*langClipSeconds*2) {
		return []float64{0}
	}

	starts := make([]float64, len(langClipPositions))
	for i, p := range langClipPositions {
		starts[i] = duration * p
	}

	return starts
}

// extractClip returns langClipSeconds of audio track track starting at start
// seconds as 16 kHz mono WAV, the format Whisper works on.
func extractClip(ctx context.Context, file string, track int, start float64) ([]byte, error) {
	var stderr bytes.Buffer

	cmd := exec.CommandContext(ctx, ffmpegFilename(), "-nostdin", "-v", "error",
		"-ss", strconv.FormatFloat(start, 'f', 0, 64), "-t", strconv.Itoa(langClipSeconds),
		"-i", file, "-map", "0:a:"+strconv.Itoa(track), "-ac", "1", "-ar", "16000", "-f", "wav", "pipe:1")
	cmd.Stderr = &stderr

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg clip at %.0fs of track %d: %w: %s", start, track, err, strings.TrimSpace(stderr.String()))
	}

	return out, nil
}

// ffmpegFilename returns ffmpeg from the folder of the configured ffprobe,
// which is just "ffmpeg" (looked up in PATH) when ffprobe_path is empty.
func ffmpegFilename() string {
	executable := "ffmpeg"
	if runtime.GOOS == "windows" {
		executable += ".exe"
	}

	return filepath.Join(filepath.Dir(getFFProbeFilename()), executable)
}

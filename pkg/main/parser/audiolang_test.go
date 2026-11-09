package parser

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/whisper"
)

func TestPickLanguage(t *testing.T) {
	tests := []struct {
		votes map[string]int
		want  string
	}{
		{map[string]int{"de": 3}, "de"},
		{map[string]int{"de": 2, "en": 1}, "de"},
		{map[string]int{"de": 1}, ""},          // one confident clip is not enough
		{map[string]int{"de": 1, "en": 1}, ""}, // tie
		{map[string]int{}, ""},
	}
	for _, tt := range tests {
		if got := pickLanguage(tt.votes); got != tt.want {
			t.Errorf("pickLanguage(%v) = %q, want %q", tt.votes, got, tt.want)
		}
	}
}

func TestClipStarts(t *testing.T) {
	if got := clipStarts(6000); !slices.Equal(got, []float64{1500, 3000, 4200}) {
		t.Errorf("clipStarts(6000) = %v", got)
	}

	if got := clipStarts(120); !slices.Equal(got, []float64{0}) {
		t.Errorf("clipStarts(120) = %v", got)
	}
}

func TestLanguageNames(t *testing.T) {
	if got := languageNames("de", "german"); !slices.Equal(got, []string{"de", "ger", "deu", "German", "Deutsch"}) {
		t.Errorf("languageNames(de) = %v", got)
	}

	if got := languageNames("sw", "swahili"); !slices.Equal(got, []string{"sw", "swahili"}) {
		t.Errorf("languageNames(sw) = %v", got)
	}

	if got := trackLanguageTag("de"); got != "ger" {
		t.Errorf("trackLanguageTag(de) = %q", got)
	}

	if got := trackLanguageTag("sw"); got != "sw" {
		t.Errorf("trackLanguageTag(sw) = %q", got)
	}
}

// fakeDetector answers every clip with the next detection in order.
type fakeDetector struct {
	answers []whisper.Detection
	clips   int
}

func (f *fakeDetector) DetectLanguage(_ context.Context, wav []byte) (*whisper.Detection, error) {
	if len(wav) < 1000 {
		return nil, errors.New("empty clip")
	}

	d := f.answers[f.clips%len(f.answers)]
	f.clips++

	return &d, nil
}

func useDetector(t *testing.T, d langDetector) {
	t.Helper()

	saved := getLangDetector
	getLangDetector = func() langDetector { return d }

	t.Cleanup(func() { getLangDetector = saved })
}

// testVideo writes a 200 s file with a German-tagged and an untagged audio
// track (sine tones; the detector is faked).
func testVideo(t *testing.T) string {
	t.Helper()

	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}

	if _, err := exec.LookPath(getFFProbeFilename()); err != nil {
		t.Skip("ffprobe not installed")
	}

	file := filepath.Join(t.TempDir(), "movie.mkv")

	out, err := exec.Command("ffmpeg", "-v", "error",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=200",
		"-f", "lavfi", "-i", "sine=frequency=880:duration=200",
		"-map", "0:a", "-map", "1:a", "-c:a", "aac",
		"-metadata:s:a:0", "language=ger", file).CombinedOutput()
	if err != nil {
		t.Fatalf("creating test file: %v: %s", err, out)
	}

	return file
}

func TestDetectAudioLanguages(t *testing.T) {
	file := testVideo(t)

	det := &fakeDetector{answers: []whisper.Detection{{Name: "english", Code: "en", Confidence: 0.95}}}
	useDetector(t, det)

	languages, names, err := DetectAudioLanguages(context.Background(), file)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(languages, []string{"ger", "eng"}) {
		t.Errorf("languages = %v, want the German tag kept and the untagged track as eng", languages)
	}

	if !slices.Equal(names, []string{"en", "eng", "English"}) {
		t.Errorf("names = %v", names)
	}

	if det.clips != len(langClipPositions) {
		t.Errorf("%d clips sent, want %d (the tagged track must not be checked)", det.clips, len(langClipPositions))
	}
}

func TestDetectAudioLanguagesUnsure(t *testing.T) {
	file := testVideo(t)

	// One confident clip, two below langMinConfidence (music, credits).
	useDetector(t, &fakeDetector{answers: []whisper.Detection{
		{Code: "en", Confidence: 0.95},
		{Code: "en", Confidence: 0.44},
		{Code: "de", Confidence: 0.5},
	}})

	languages, names, err := DetectAudioLanguages(context.Background(), file)
	if err != nil || languages != nil || names != nil {
		t.Errorf("unsure detection changed the result: %v, %v, %v", languages, names, err)
	}
}

func TestDetectAudioLanguagesDisabled(t *testing.T) {
	useDetector(t, nil)

	languages, names, err := DetectAudioLanguages(context.Background(), "/does/not/exist.mkv")
	if err != nil || languages != nil || names != nil {
		t.Errorf("without whisper_url: %v, %v, %v", languages, names, err)
	}
}

package tags

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeDurationDispatch(t *testing.T) {
	for _, s := range []string{testMP3Sample, testOGGSample} {
		if _, err := os.Stat(s); err != nil {
			t.Logf("skip %s (absent)", s)
			continue
		}

		d, ok := NativeDuration(s)
		if !ok || d <= 0 {
			t.Errorf("NativeDuration(%s) = %s, ok=%v; want positive duration", s, d, ok)
		} else {
			t.Logf("NativeDuration(%s) = %s", s, d)
		}
	}

	// Unhandled extension must report false so the caller falls back to ffprobe.
	if _, ok := NativeDuration("nope.wav"); ok {
		t.Error("NativeDuration(.wav) should return false")
	}
}

// synthMP3 returns n MPEG1 Layer III 128 kbps 44.1 kHz frames (417 bytes each).
func synthMP3(n int) []byte {
	frame := make([]byte, 417)
	copy(frame, []byte{0xFF, 0xFB, 0x90, 0x00})

	return bytes.Repeat(frame, n)
}

func writeTemp(t *testing.T, data []byte) string {
	t.Helper()

	p := filepath.Join(t.TempDir(), "x.mp3")
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}

	return p
}

// Junk inside the stream stops the frame walk; reporting the partial count as
// the duration would make a long file look short, so it must fail instead and
// let the caller fall back to ffprobe.
func TestMP3DurationWalkStoppedEarly(t *testing.T) {
	data := append(synthMP3(100), make([]byte, 512*1024)...)
	data = append(data, synthMP3(5000)...)

	if d, err := mp3Duration(writeTemp(t, data)); err == nil {
		t.Errorf("mp3Duration = %v, nil; want an error for a truncated frame walk", d)
	}

	if d, ok := NativeDuration(writeTemp(t, data)); ok {
		t.Errorf("NativeDuration = %v, true; want ok=false", d)
	}

}

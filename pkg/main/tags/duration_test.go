package tags

import (
	"io"
	"os"
	"testing"
	"time"
)

const testMP3VBRSample = "testdata/sample-vbr.mp3"

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}

	return d
}

// frameWalkDuration computes duration purely by counting MPEG frames (ignoring
// any Xing/Info header), to validate the VBR frame-walk path against ground truth.
func frameWalkDuration(tb testing.TB, path string) (time.Duration, int64) {
	tb.Helper()

	f, err := os.Open(path)
	if err != nil {
		tb.Fatalf("open: %v", err)
	}
	defer f.Close()

	var hdr [10]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		tb.Fatalf("read header: %v", err)
	}

	var audioStart int64
	if string(hdr[0:3]) == "ID3" {
		size := int64(hdr[6]&0x7f)<<21 | int64(hdr[7]&0x7f)<<14 |
			int64(hdr[8]&0x7f)<<7 | int64(hdr[9]&0x7f)
		if hdr[5]&0x10 != 0 {
			size += 10
		}

		audioStart = 10 + size
	}

	if _, err := f.Seek(audioStart, io.SeekStart); err != nil {
		tb.Fatalf("seek: %v", err)
	}

	buf := make([]byte, 8192)

	n, _ := io.ReadFull(f, buf)
	buf = buf[:n]

	off := -1

	var fh mp3FrameHeader

	for i := 0; i+4 <= len(buf); i++ {
		if buf[i] == 0xFF && buf[i+1]&0xE0 == 0xE0 {
			if h, ok := parseMP3FrameHeader(buf[i : i+4]); ok {
				fh = h
				off = i

				break
			}
		}
	}

	if off < 0 {
		tb.Fatal("no MPEG frame found")
	}

	frames := mp3CountFrames(f, audioStart+int64(off))

	return framesToDuration(frames, fh.samplesPerFrame(), fh.sampleRate), frames
}

// TestMP3VBRDuration checks the header (Xing) path on a real VBR file.
// ffprobe reports exactly 60.000000s for this sample.
func TestMP3VBRDuration(t *testing.T) {
	if _, err := os.Stat(testMP3VBRSample); err != nil {
		t.Skipf("sample %s not available - skipping", testMP3VBRSample)
	}

	d, err := mp3Duration(testMP3VBRSample)
	if err != nil {
		t.Fatalf("mp3Duration: %v", err)
	}

	t.Logf("VBR duration (Xing header): %s", d)

	if absDur(d-60*time.Second) > 500*time.Millisecond {
		t.Errorf("duration %s, want ~60s (ffprobe ground truth)", d)
	}
}

// TestMP3VBRFrameWalk forces the frame-counting path on real VBR audio and
// confirms it agrees with both the Xing header and ffprobe — the case where a
// first-frame-bitrate estimate would be wrong.
func TestMP3VBRFrameWalk(t *testing.T) {
	if _, err := os.Stat(testMP3VBRSample); err != nil {
		t.Skipf("sample %s not available - skipping", testMP3VBRSample)
	}

	header, err := mp3Duration(testMP3VBRSample)
	if err != nil {
		t.Fatalf("mp3Duration: %v", err)
	}

	walk, frames := frameWalkDuration(t, testMP3VBRSample)
	t.Logf("VBR header=%s frameWalk=%s (%d frames)", header, walk, frames)

	if absDur(walk-header) > time.Second {
		t.Errorf("frame-walk %s differs from Xing header %s by %s", walk, header, absDur(walk-header))
	}

	if absDur(walk-60*time.Second) > time.Second {
		t.Errorf("frame-walk %s differs from ffprobe 60s", walk)
	}
}

func TestMP3NativeDuration(t *testing.T) {
	if _, err := os.Stat(testMP3Sample); err != nil {
		t.Skipf("sample %s not available - skipping", testMP3Sample)
	}

	d, err := mp3Duration(testMP3Sample)
	if err != nil {
		t.Fatalf("mp3Duration: %v", err)
	}

	t.Logf("MP3 native duration: %s", d)

	if d < 30*time.Second || d > 30*time.Minute {
		t.Errorf("duration %s outside sane range for a music track", d)
	}
}

func TestOGGNativeDuration(t *testing.T) {
	if _, err := os.Stat(testOGGSample); err != nil {
		t.Skipf("sample %s not available - skipping", testOGGSample)
	}

	d, err := (&OGGHandler{}).GetDuration(testOGGSample)
	if err != nil {
		t.Fatalf("GetDuration: %v", err)
	}

	t.Logf("OGG native duration: %s", d)

	if d <= 0 {
		t.Errorf("expected positive duration, got %s", d)
	}
}

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

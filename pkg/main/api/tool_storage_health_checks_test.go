package api

import (
	"strings"
	"testing"
)

func TestPerformIOTestSlowThreshold(t *testing.T) {
	dir := t.TempDir()

	if r := performIOTest(dir, 0); r.Error != "" || !r.WriteTest || !r.ReadTest {
		t.Fatalf("threshold 0: %+v", r)
	}

	// No disk does 1e12 MB/s, so this must be reported as slow.
	if r := performIOTest(dir, 1e12); !strings.Contains(r.Error, "MB/s") {
		t.Fatalf("huge threshold not reported as slow: %+v", r)
	}
}

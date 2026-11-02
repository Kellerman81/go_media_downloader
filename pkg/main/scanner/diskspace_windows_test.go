package scanner

import (
	"testing"
)

// TestFreeDiskSpace sanity-checks the platform implementation.
func TestFreeDiskSpace(t *testing.T) {
	free := freeDiskSpace(t.TempDir())
	if free <= 0 {
		t.Fatalf("freeDiskSpace = %d; want > 0 for the temp directory", free)
	}
}

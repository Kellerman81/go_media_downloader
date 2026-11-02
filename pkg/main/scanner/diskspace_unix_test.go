package scanner

import (
	"testing"
)

func TestDiskUsage(t *testing.T) {
	free, total, err := DiskUsage(t.TempDir())
	if err != nil {
		t.Skipf("statfs unavailable: %v", err)
	}

	if total == 0 || free > total {
		t.Fatalf("DiskUsage = free %d, total %d", free, total)
	}

	if got := freeDiskSpace(t.TempDir()); got >= 0 && uint64(got) > total {
		t.Fatalf("freeDiskSpace %d exceeds DiskUsage total %d", got, total)
	}
}

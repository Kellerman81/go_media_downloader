package api

import (
	"strings"
	"testing"
)

func TestSuccessRateBadge(t *testing.T) {
	tests := []struct {
		success, failure int64
		class, text      string
	}{
		{0, 0, "secondary", "-"},
		{19, 1, "success", "95.0%"},
		{9, 1, "warning", "90.0%"},
		{1, 1, "danger", "50.0%"},
	}

	for _, tt := range tests {
		class, text := successRateBadge(tt.success, tt.failure)
		if class != tt.class || text != tt.text {
			t.Errorf("successRateBadge(%d, %d) = %q, %q; want %q, %q",
				tt.success, tt.failure, class, text, tt.class, tt.text)
		}
	}
}

func TestTruncateStringRuneSafe(t *testing.T) {
	got := truncateString(strings.Repeat("ä", 40), 10)
	if got != strings.Repeat("ä", 7)+"..." {
		t.Fatalf("truncateString = %q", got)
	}

	if got := truncateString("short", 10); got != "short" {
		t.Fatalf("truncateString(short) = %q", got)
	}
}

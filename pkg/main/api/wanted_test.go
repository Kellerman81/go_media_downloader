package api

import (
	"testing"
)

func TestEscapeLikePattern(t *testing.T) {
	for in, want := range map[string]string{
		"plain":  "plain",
		"100%":   `100\%`,
		"a_b":    `a\_b`,
		`back\s`: `back\\s`,
	} {
		if got := escapeLikePattern(in); got != want {
			t.Errorf("escapeLikePattern(%q) = %q, want %q", in, got, want)
		}
	}
}

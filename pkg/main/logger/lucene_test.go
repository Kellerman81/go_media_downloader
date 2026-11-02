package logger

import "testing"

func TestLuceneEscape(t *testing.T) {
	tests := map[string]string{
		"":                  "",
		"plain words":       "plain words",
		"AC/DC":             `AC\/DC`,
		"a+b-c":             `a\+b\-c`,
		"x && y || !z":      `x \&\& y \|\| \!z`,
		`(a)[b]{c}^"d"~*?:`: `\(a\)\[b\]\{c\}\^\"d\"\~\*\?\:`,
		`back\slash`:        `back\\slash`,
		"Motörhead":         "Motörhead",
	}

	for in, want := range tests {
		if got := LuceneEscape(in); got != want {
			t.Errorf("LuceneEscape(%q) = %q, want %q", in, got, want)
		}
	}
}

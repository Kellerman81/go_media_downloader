package parser_v2

import (
	"testing"
)

func TestRegression_MatchStringAllOccurrences(t *testing.T) {
	ps := &DBPatternStore{
		Audios: []DBPattern{{Name: "DD", Strings: []string{"dd"}}},
	}

	if name, idx, _ := ps.MatchString("Eddie.Murphy.DD.5.1", PatternTypeAudio); name != "DD" || idx != 13 {
		t.Errorf("MatchString = %q at %d, want DD at 13", name, idx)
	}

	if name, _, _ := ps.MatchString("Eddie.Murphy", PatternTypeAudio); name != "" {
		t.Errorf("MatchString = %q, want no match", name)
	}
}

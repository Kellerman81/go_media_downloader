package tvmaze

import (
	"testing"
)

func TestStripHTMLDecodesEntities(t *testing.T) {
	got := stripHTML("<p>Tom &amp; Jerry&#39;s <b>caf&eacute;</b></p>")
	if want := "Tom & Jerry's café"; got != want {
		t.Errorf("stripHTML = %q, want %q", got, want)
	}
}

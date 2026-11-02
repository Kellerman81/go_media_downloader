package api

import (
	"slices"
	"testing"
)

func TestSelectOptionsWithCurrent(t *testing.T) {
	opts := []string{"info", "debug"}

	if got := selectOptionsWithCurrent(opts, "Info"); !slices.Equal(got, []string{"info", "debug", "Info"}) {
		t.Errorf("unknown value not added: %v", got)
	}

	if got := selectOptionsWithCurrent(opts, "debug"); !slices.Equal(got, opts) {
		t.Errorf("known value duplicated: %v", got)
	}

	if got := selectOptionsWithCurrent(opts, ""); !slices.Equal(got, opts) {
		t.Errorf("empty value added: %v", got)
	}
}

package utils

import (
	"context"
	"errors"
	"testing"
)

func TestTmdbFetchPages(t *testing.T) {
	page := func(p int) []int {
		out := make([]int, tmdbPageSize)
		for i := range out {
			out[i] = (p-1)*tmdbPageSize + i + 1
		}

		return out
	}

	var calls []int

	got, err := tmdbFetchPages(45, func(p int) ([]int, int, error) {
		calls = append(calls, p)
		return page(p), 10, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got) != 45 || got[0] != 1 || got[44] != 45 {
		t.Errorf("got %d items, first %d last %d", len(got), got[0], got[len(got)-1])
	}

	if len(calls) != 3 {
		t.Errorf("fetched pages %v, want 1..3", calls)
	}

	got, err = tmdbFetchPages(100, func(p int) ([]int, int, error) {
		if p == 3 {
			return nil, 0, errors.New("boom")
		}

		return page(p), 0, nil
	})
	if err != nil || len(got) != 40 {
		t.Errorf("partial failure: got %d items, err %v", len(got), err)
	}

	if _, err := tmdbFetchPages(20, func(int) ([]int, int, error) {
		return nil, 0, errors.New("boom")
	}); err == nil {
		t.Error("first page failure must be returned")
	}
}

func TestTraktLimitAndPaging(t *testing.T) {
	for _, tt := range []struct {
		in           string
		limit, pages int
		size         int
	}{
		{"", 20, 1, 20},
		{"0", 20, 1, 20},
		{"x", 20, 1, 20},
		{"50", 50, 1, 50},
		{"250", 250, 3, 100},
	} {
		l := listLimit(tt.in, 20)
		p, s := traktPaging(l)

		if l != tt.limit || p != tt.pages || s != tt.size {
			t.Errorf("%q: limit %d pages %d size %d", tt.in, l, p, s)
		}
	}
}

func TestTraktSeqStopsAtLimitAndShortPage(t *testing.T) {
	var pages []int

	fetch := func(_ context.Context, page, size int) ([]int, int, error) {
		pages = append(pages, page)
		if page == 2 {
			return []int{1}, size - 1, nil // short page ends the list
		}

		return make([]int, size), size, nil
	}

	count := 0
	for range traktSeq("test", "250", fetch) {
		count++
	}

	if count != 101 || len(pages) != 2 {
		t.Errorf("got %d items from pages %v, want 101 from [1 2]", count, pages)
	}

	count = 0
	for range traktSeq("test", "30", fetch) {
		count++
	}

	if count != 30 {
		t.Errorf("limit 30 yielded %d", count)
	}
}

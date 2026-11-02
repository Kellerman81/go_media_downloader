package api

import (
	"testing"
)

func TestBoundPagination(t *testing.T) {
	tests := []struct {
		name string
		in   PaginationParams
		want PaginationParams
	}{
		{"default", PaginationParams{}, PaginationParams{Limit: defaultListLimit}},
		{"within", PaginationParams{Limit: 50, Offset: 100, Page: 3}, PaginationParams{Limit: 50, Offset: 100, Page: 3}},
		{"capped", PaginationParams{Limit: 5000}, PaginationParams{Limit: maxListLimit}},
		{
			"capped page",
			PaginationParams{Limit: 5000, Offset: 10000, Page: 3},
			PaginationParams{Limit: maxListLimit, Offset: 2 * maxListLimit, Page: 3},
		},
	}

	for _, tt := range tests {
		got := tt.in
		boundPagination(&got)

		if got != tt.want {
			t.Errorf("%s: boundPagination(%+v) = %+v, want %+v", tt.name, tt.in, got, tt.want)
		}
	}
}

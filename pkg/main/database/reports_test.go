package database

import (
	"errors"
	"testing"
)

// TestMediaReports: one open report per account and file, unknown reasons
// become "other", and handling a file closes every report of it.
func TestMediaReports(t *testing.T) {
	useUsersTestDB(t)

	r := MediaReport{Username: "bea", Kind: "movie", FileID: 3, Title: "x.mkv", Reason: "nonsense"}
	if err := AddMediaReport(&r); err != nil || r.Reason != "other" {
		t.Fatalf("report: %+v, %v", r, err)
	}

	if err := AddMediaReport(&MediaReport{Username: "bea", Kind: "movie", FileID: 3}); !errors.Is(err, ErrReportDuplicate) {
		t.Errorf("second report: %v", err)
	}

	if err := AddMediaReport(&MediaReport{Username: "cid", Kind: "movie", FileID: 3, Reason: "language"}); err != nil {
		t.Fatal(err)
	}

	if CountMediaReports() != 2 {
		t.Fatalf("open reports: %d", CountMediaReports())
	}

	if err := CloseMediaReports("movie", new(uint(3))); err != nil || CountMediaReports() != 0 {
		t.Errorf("after closing the file's reports: %d, %v", CountMediaReports(), err)
	}
}

// TestUserMediaReports: the open reports of one account, newest first.
func TestUserMediaReports(t *testing.T) {
	useUsersTestDB(t)

	for _, r := range []MediaReport{
		{Username: "bea", Kind: "movie", FileID: 1, Title: "first", Reason: "quality"},
		{Username: "cid", Kind: "movie", FileID: 1, Title: "other", Reason: "quality"},
		{Username: "bea", Kind: "album", FileID: 2, Title: "second", Reason: "language"},
	} {
		if err := AddMediaReport(&r); err != nil {
			t.Fatal(err)
		}
	}

	got := UserMediaReports("Bea")
	if len(got) != 2 || got[0].Title != "second" || got[1].Title != "first" || got[0].CreatedAt == "" {
		t.Errorf("reports of bea: %+v", got)
	}
}

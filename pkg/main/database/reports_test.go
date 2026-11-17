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

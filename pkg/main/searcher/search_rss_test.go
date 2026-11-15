package searcher

import (
	"testing"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

// TestPickRSSSeasonSeries: every series gets its turn before one is searched
// again, per media config.
func TestPickRSSSeasonSeries(t *testing.T) {
	rows := func() []database.DbstaticTwoUint {
		out := make([]database.DbstaticTwoUint, 55)
		for i := range out {
			out[i] = database.DbstaticTwoUint{Num1: uint(i + 1), Num2: uint(i + 100)}
		}

		return out
	}

	seen := map[uint]int{}

	for run := range 3 {
		got := pickRSSSeasonSeries("serie_test", rows(), 20)
		if len(got) != 20 {
			t.Fatalf("run %d: %d series", run, len(got))
		}

		for _, r := range got {
			if r.Num2 != r.Num1+99 {
				t.Fatalf("pair broken: %+v", r)
			}

			seen[r.Num1]++
		}

		time.Sleep(time.Millisecond)
	}

	// 3 runs of 20 over 55 series: all searched, 5 of them twice.
	twice := 0

	for id := uint(1); id <= 55; id++ {
		switch seen[id] {
		case 0:
			t.Errorf("series %d never searched", id)
		case 2:
			twice++
		}
	}

	if twice != 5 {
		t.Errorf("%d series searched twice, want 5", twice)
	}

	// Another config has its own rotation.
	if got := pickRSSSeasonSeries("serie_other", rows()[:3], 20); len(got) != 3 {
		t.Errorf("other config: %d series", len(got))
	}
}

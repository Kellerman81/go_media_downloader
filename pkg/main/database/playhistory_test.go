package database

import "testing"

// TestPlayProgress: one row per account and file; a new play counts and
// restarts, completion keeps the furthest point, and resume skips files
// that were (nearly) finished.
func TestPlayProgress(t *testing.T) {
	useUsersTestDB(t)

	h := PlayHistory{Username: "bea", Kind: "movie", FileID: 7, Title: "Film", Position: 0, Duration: 1000}
	for _, step := range []struct {
		pos     float64
		started bool
	}{{0, true}, {600, false}, {300, false}} {
		h.Position = step.pos
		if err := PlayProgress(&h, step.started); err != nil {
			t.Fatal(err)
		}
	}

	rows := RecentPlays("bea", 10)
	if len(rows) != 1 || rows[0].Plays != 1 || rows[0].Position != 300 || rows[0].Completion != 0.6 {
		t.Fatalf("history after one play: %+v", rows)
	}

	if got := PlayResume("bea", "movie", new(uint(7))); got != 300 {
		t.Errorf("resume at %v, want 300", got)
	}

	h.Position = 990
	if err := PlayProgress(&h, false); err != nil {
		t.Fatal(err)
	}

	if got := PlayResume("bea", "movie", new(uint(7))); got != 0 {
		t.Errorf("resume of a finished file at %v", got)
	}

	h.Position = 0
	if err := PlayProgress(&h, true); err != nil {
		t.Fatal(err)
	}

	if rows := RecentPlays("", 10); len(rows) != 1 || rows[0].Plays != 2 || rows[0].Completion != 0 {
		t.Errorf("history after the second play: %+v", rows)
	}

	if stats := PlayStatsBy("username", 30); len(stats) != 1 || stats[0].Key != "bea" || stats[0].Plays != 2 {
		t.Errorf("stats by account: %+v", stats)
	}
}

// TestWatchedAndContinue: watched marks, the completions of an entry's
// files, and the Continue list (started, unfinished, in the given lists).
func TestWatchedAndContinue(t *testing.T) {
	useUsersTestDB(t)

	for _, q := range []string{
		"insert into dbmovies (id, title, year) values (1, 'A', 2020), (2, 'B', 2021), (3, 'C', 2022)",
		"insert into movies (id, dbmovie_id, listname) values (1, 1, 'EN List'), (2, 2, 'EN List'), (3, 3, 'DE List')",
		"insert into movie_files (id, movie_id, dbmovie_id, location) values (11, 1, 1, '/a'), (12, 2, 2, '/b'), (13, 3, 3, '/c')",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	for _, h := range []PlayHistory{
		{Username: "bea", Kind: "movie", FileID: 11, Title: "A", Position: 1200, Duration: 6000},
		{Username: "bea", Kind: "movie", FileID: 12, Title: "B", Position: 5900, Duration: 6000},
		{Username: "bea", Kind: "movie", FileID: 13, Title: "C", Position: 900, Duration: 6000},
	} {
		if err := PlayProgress(&h, true); err != nil {
			t.Fatal(err)
		}
	}

	if items := ContinuePlaying("bea", "movie", []string{"EN List"}, 10); len(items) != 1 || items[0].FileID != 11 ||
		items[0].Title != "A" || items[0].Sub != "2020" || items[0].DbID != 1 {
		t.Fatalf("continue: %+v", items)
	}

	if err := SetWatched("bea", "movie", new(uint(11)), true); err != nil {
		t.Fatal(err)
	}

	if items := ContinuePlaying("bea", "movie", []string{"EN List", "DE List"}, 10); len(items) != 1 || items[0].FileID != 13 {
		t.Errorf("continue after watching A: %+v", items)
	}

	if c := PlayCompletions("bea", "movie", "movie_files", "movie_id", new(uint64(1))); c[11] != 1 {
		t.Errorf("completions of A: %v", c)
	}

	if err := SetWatched("bea", "movie", new(uint(11)), false); err != nil {
		t.Fatal(err)
	}

	if c := PlayCompletions("bea", "movie", "movie_files", "movie_id", new(uint64(1))); len(c) != 0 {
		t.Errorf("completions after unwatching: %v", c)
	}
}

// TestPlaySessions: a play collects its time; the stats read it back.
func TestPlaySessions(t *testing.T) {
	useUsersTestDB(t)

	id := StartPlaySession("bea", "movie", 11, "A", "direct", "original", false)
	if id == 0 {
		t.Fatal("no session row")
	}

	AddPlaySeconds(id, 10, "")
	AddPlaySeconds(id, 5, "transcode")

	s := PlaySessionsSince(1)
	if len(s) != 1 || s[0].Seconds != 15 || s[0].Mode != "transcode" || s[0].Title != "A" {
		t.Errorf("sessions: %+v", s)
	}
}

// TestLastPlayedFile: the part of an entry the account played last; marks
// set by hand don't count as playing.
func TestLastPlayedFile(t *testing.T) {
	useUsersTestDB(t)

	for _, q := range []string{
		"insert into dbaudiobooks (id, title) values (1, 'Book')",
		"insert into audiobooks (id, dbaudiobook_id, listname) values (1, 1, 'L')",
		"insert into audiobook_files (id, audiobook_id, dbaudiobook_id, location) values (1, 1, 1, '/p1'), (2, 1, 1, '/p2'), (3, 1, 1, '/p3')",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	last := func() uint {
		return LastPlayedFile("bea", "audiobook", "audiobook_files", "audiobook_id", new(uint64(1)))
	}
	if last() != 0 {
		t.Fatal("played something before playing")
	}

	for _, id := range []uint{1, 2} {
		if err := PlayProgress(&PlayHistory{Username: "bea", Kind: "audiobook", FileID: id, Position: 60, Duration: 3600}, true); err != nil {
			t.Fatal(err)
		}

		if err := ExecNErr("update play_history set updated_at = datetime('now', '-' || ? || ' minutes') where file_id = ?", 10-id, id); err != nil {
			t.Fatal(err)
		}
	}

	if err := SetWatched("bea", "audiobook", new(uint(3)), true); err != nil {
		t.Fatal(err)
	}

	if got := last(); got != 2 {
		t.Errorf("last played %d, want 2 (part 3 was only marked)", got)
	}
}

// TestFinishAudiobook: finishing the last part marks every part heard;
// a middle part doesn't finish the book.
func TestFinishAudiobook(t *testing.T) {
	useUsersTestDB(t)

	for _, q := range []string{
		"insert into dbaudiobooks (id, title) values (1, 'Book')",
		"insert into audiobooks (id, dbaudiobook_id, listname) values (1, 1, 'L')",
		"insert into audiobook_files (id, audiobook_id, dbaudiobook_id, location, track_number) values (1, 1, 1, '/p1', 1), (2, 1, 1, '/p2', 2), (3, 1, 1, '/p3', 3)",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	if FinishAudiobook("bea", new(uint(2))) {
		t.Error("a middle part finished the book")
	}

	if !FinishAudiobook("bea", new(uint(3))) {
		t.Fatal("the last part didn't finish the book")
	}

	if c := PlayCompletions("bea", "audiobook", "audiobook_files", "audiobook_id", new(uint64(1))); len(c) != 3 || c[1] != 1 || c[3] != 1 {
		t.Errorf("completions %v", c)
	}
}

// TestContinueAudiobook: a book with two started parts is listed once, at
// the part played last, with the time left of the whole book.
func TestContinueAudiobook(t *testing.T) {
	useUsersTestDB(t)

	for _, q := range []string{
		"insert into dbaudiobooks (id, title) values (1, 'Book')",
		"insert into audiobooks (id, dbaudiobook_id, listname) values (1, 1, 'L')",
		"insert into audiobook_files (id, audiobook_id, dbaudiobook_id, location, track_number, runtime_ms) values " +
			"(1, 1, 1, '/p1', 1, 3600000), (2, 1, 1, '/p2', 2, 3600000), (3, 1, 1, '/p3', 3, 1800000)",
	} {
		if err := ExecNErr(q); err != nil {
			t.Fatal(err)
		}
	}

	for i, h := range []PlayHistory{
		{Username: "bea", Kind: "audiobook", FileID: 1, Position: 600, Duration: 3600},
		{Username: "bea", Kind: "audiobook", FileID: 2, Position: 1800, Duration: 3600},
	} {
		if err := PlayProgress(&h, true); err != nil {
			t.Fatal(err)
		}

		if err := ExecNErr("update play_history set updated_at = datetime('now', '-' || ? || ' minutes') where file_id = ?", 10-i, h.FileID); err != nil {
			t.Fatal(err)
		}
	}

	items := ContinuePlaying("bea", "audiobook", []string{"L"}, 10)
	if len(items) != 1 {
		t.Fatalf("items: %+v", items)
	}

	// Part 2 at 30 of 60 minutes: 1.5 h of 2.5 h done, 1 h left.
	if it := items[0]; it.FileID != 2 || it.Part != 2 || it.Parts != 3 || it.Left != 3600 || it.Progress != 0.6 {
		t.Errorf("item %+v", it)
	}
}

// TestUserPlayTotals: files, plays, hours and finished files of one
// account only.
func TestUserPlayTotals(t *testing.T) {
	useUsersTestDB(t)

	if got := UserPlayTotals("bea"); got != (PlayTotals{}) {
		t.Fatalf("totals without plays: %+v", got)
	}

	for _, h := range []PlayHistory{
		{Username: "bea", Kind: "movie", FileID: 1, Duration: 3600, Position: 3600},
		{Username: "bea", Kind: "episode", FileID: 2, Duration: 3600, Position: 1800},
		{Username: "cid", Kind: "movie", FileID: 1, Duration: 3600, Position: 3600},
	} {
		if err := PlayProgress(&h, true); err != nil {
			t.Fatal(err)
		}
	}

	got := UserPlayTotals("BEA")
	if got.Titles != 2 || got.Plays != 2 || got.Hours != 1.5 || got.Finished != 1 || got.Last == "" {
		t.Errorf("totals: %+v", got)
	}
}

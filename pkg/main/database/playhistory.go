package database

import "strings"

// PlayHistory is what an account played of a file (play_history).
type PlayHistory struct {
	ID         uint    `db:"id"`
	Username   string  `db:"username"`
	Kind       string  `db:"kind"` // movie, episode, track, audiobook
	FileID     uint    `db:"file_id"`
	Title      string  `db:"title"`
	Position   float64 `db:"position"`
	Duration   float64 `db:"duration"`
	Completion float64 `db:"completion"`
	Plays      int     `db:"plays"`
	StartedAt  string  `db:"started_at"`
	UpdatedAt  string  `db:"updated_at"`
}

// PlayProgress records the position an account reached in a file; started
// counts a new play.
func PlayProgress(h *PlayHistory, started bool) error {
	completion := 0.0
	if h.Duration > 0 {
		completion = min(max(h.Position/h.Duration, 0), 1)
	}

	plays := 0
	if started {
		plays = 1
	}

	return ExecNErr(`insert into play_history (username, kind, file_id, title, position, duration, completion, plays)
		values (?, ?, ?, ?, ?, ?, ?, 1)
		on conflict (username, kind, file_id) do update set
			title = case when excluded.title <> '' then excluded.title else title end,
			position = excluded.position,
			duration = case when excluded.duration > 0 then excluded.duration else duration end,
			completion = case when ? = 1 then excluded.completion else max(completion, excluded.completion) end,
			plays = plays + ?,
			started_at = case when ? = 1 then datetime('now','localtime') else started_at end,
			updated_at = datetime('now','localtime')`,
		&h.Username, &h.Kind, &h.FileID, &h.Title, &h.Position, &h.Duration, &completion, &plays, &plays, &plays)
}

// PlayResume is the position to resume a file at for an account (0: from
// the start).
func PlayResume(username, kind string, fileID *uint) float64 {
	return Getdatarow[float64](false,
		"select position from play_history where username = ? and kind = ? and file_id = ? and completion < 0.95",
		username, kind, fileID)
}

// RecentPlays returns the latest plays, newest first (all accounts when
// username is empty); files only marked watched aren't plays.
func RecentPlays(username string, limit int) []PlayHistory {
	return StructscanT[PlayHistory](false, uint(max(limit, 0)), //nolint:gosec // limit is not negative
		`select id, username, kind, file_id, title, position, duration, completion, plays, started_at, updated_at
		from play_history where plays > 0 and (? = '' or username = ?) order by updated_at desc limit ?`, username, username, limit)
}

// PlayStat is a count of plays by a key (account, kind or day).
type PlayStat struct {
	Key   string  `db:"key"`
	Plays int     `db:"plays"`
	Hours float64 `db:"hours"`
}

// PlayStatsBy sums plays and played hours by column (username, kind) or
// by day of the last days (column "day").
func PlayStatsBy(column string, days int) []PlayStat {
	key := "username"

	switch column {
	case "kind":
		key = "kind"
	case "day":
		key = "date(updated_at)"
	}

	order := "2 desc"
	if column == "day" {
		order = "1"
	}

	return StructscanT[PlayStat](false, 32, "select "+key+" as key, sum(plays) as plays, "+
		"round(sum(completion * duration) / 3600.0, 1) as hours from play_history "+
		"where updated_at >= datetime('now','localtime', '-' || ? || ' days') group by 1 order by "+order, days)
}

// StartPlaySession records a play (play_sessions) and returns its row.
func StartPlaySession(username, kind string, fileID uint, title, mode, quality string) uint {
	id, err := ExecNid("insert into play_sessions (username, kind, file_id, title, mode, quality) values (?, ?, ?, ?, ?, ?)",
		username, kind, fileID, title, mode, quality)
	if err != nil {
		return 0
	}

	return uint(id) //nolint:gosec // row ids are positive
}

// AddPlaySeconds adds played time to a play and moves its end to now.
func AddPlaySeconds(id uint, seconds float64, mode string) {
	ExecN("update play_sessions set seconds = seconds + ?, ended_at = datetime('now','localtime'), "+
		"mode = case when ? <> '' then ? else mode end where id = ?", seconds, mode, mode, id)
}

// PlaySession is a play for the viewing statistics.
type PlaySession struct {
	Username  string  `db:"username"`
	Kind      string  `db:"kind"`
	Title     string  `db:"title"`
	Mode      string  `db:"mode"`
	StartedAt string  `db:"started_at"`
	EndedAt   string  `db:"ended_at"`
	Seconds   float64 `db:"seconds"`
}

// PlaySessionsSince returns the plays of the last days, oldest first.
func PlaySessionsSince(days int) []PlaySession {
	return StructscanT[PlaySession](false, 256, "select username, kind, title, mode, "+
		"strftime('%Y-%m-%d %H:%M:%S', started_at) as started_at, strftime('%Y-%m-%d %H:%M:%S', ended_at) as ended_at, seconds "+
		"from play_sessions where started_at >= datetime('now','localtime', '-' || ? || ' days') order by started_at", days)
}

// Watched is the completion from which a file counts as watched (or
// heard): 95%, so stopping during the credits still counts.
const Watched = 0.95

// PlayCompletions returns the account's completion of the files of a list
// entry (files table and its column referencing the entry), by file id.
func PlayCompletions(username, kind, filesTable, entryColumn string, entryID *uint64) map[uint]float64 {
	rows := StructscanT[struct {
		FileID     uint    `db:"file_id"`
		Completion float64 `db:"completion"`
	}](false, 32, "select h.file_id, h.completion from play_history h inner join "+filesTable+" f on f.id = h.file_id "+
		"where h.username = ? and h.kind = ? and f."+entryColumn+" = ?", username, kind, entryID)

	out := make(map[uint]float64, len(rows))
	for _, r := range rows {
		out[r.FileID] = r.Completion
	}

	return out
}

// SetWatched marks a file watched (completion 1, played from the start
// next time) or unwatched (its history removed) for an account.
func SetWatched(username, kind string, fileID *uint, watched bool) error {
	if !watched {
		return ExecNErr("delete from play_history where username = ? and kind = ? and file_id = ?", username, kind, fileID)
	}

	return ExecNErr(`insert into play_history (username, kind, file_id, completion, plays) values (?, ?, ?, 1, 0)
		on conflict (username, kind, file_id) do update set completion = 1, position = 0, updated_at = datetime('now','localtime')`,
		username, kind, fileID)
}

// ContinueItem is a file the account started and didn't finish.
type ContinueItem struct {
	FileID     uint    `db:"file_id"`
	EntryID    uint    `db:"entry_id"`
	DbID       uint    `db:"db_id"`
	Title      string  `db:"title"`
	Sub        string  `db:"sub"`
	Position   float64 `db:"position"`
	Duration   float64 `db:"duration"`
	Completion float64 `db:"completion"`

	// Over the whole entry (an audiobook's parts): time left in seconds and
	// how far it got; for one file the file's.
	Left     float64 `db:"-"`
	Progress float64 `db:"-"`
	Part     int     `db:"-"` // audiobooks: the part (1-based) of Parts
	Parts    int     `db:"-"`
}

// continueQueries select the started files of a kind with their entry, its
// metadata row (for the cover) and what to show.
var continueQueries = map[string]string{
	"movie": `select h.file_id, e.id as entry_id, e.dbmovie_id as db_id, coalesce(d.title, '') as title,
		coalesce(cast(d.year as text), '') as sub, h.position, h.duration, h.completion
		from play_history h inner join movie_files f on f.id = h.file_id inner join movies e on e.id = f.movie_id
		inner join dbmovies d on d.id = e.dbmovie_id`,
	"episode": `select h.file_id, e.id as entry_id, e.dbserie_id as db_id, coalesce(d.seriename, '') as title,
		trim(coalesce(de.identifier, '') || ' ' || coalesce(de.title, '')) as sub, h.position, h.duration, h.completion
		from play_history h inner join serie_episode_files f on f.id = h.file_id inner join series e on e.id = f.serie_id
		inner join dbseries d on d.id = e.dbserie_id left join dbserie_episodes de on de.id = f.dbserie_episode_id`,
	"audiobook": `select h.file_id, e.id as entry_id, e.dbaudiobook_id as db_id, coalesce(d.title, '') as title,
		'' as sub, h.position, h.duration, h.completion
		from play_history h inner join audiobook_files f on f.id = h.file_id inner join audiobooks e on e.id = f.audiobook_id
		inner join dbaudiobooks d on d.id = e.dbaudiobook_id`,
}

// ContinuePlaying returns the account's started, unfinished files of kind
// in the lists, most recent first.
func ContinuePlaying(username, kind string, lists []string, limit int) []ContinueItem {
	query, ok := continueQueries[kind]
	if !ok || len(lists) == 0 {
		return nil
	}

	args := []any{username, kind, Watched}
	for _, l := range lists {
		args = append(args, l)
	}

	// Several started files of one entry (parts, episodes): the latest only.
	args = append(args, limit*8)

	rows := StructscanT[ContinueItem](false, uint(max(limit, 0)), query+ //nolint:gosec // limit is not negative
		" where h.username = ? and h.kind = ? and h.completion > 0.02 and h.completion < ? and h.position > 30"+
		" and e.listname collate nocase in (?"+strings.Repeat(", ?", len(lists)-1)+") order by h.updated_at desc, h.id desc limit ?", args...)

	seen := map[uint]bool{}
	out := rows[:0]

	for _, r := range rows {
		if seen[r.EntryID] || len(out) >= limit {
			continue
		}

		seen[r.EntryID] = true
		r.Left, r.Progress = max(r.Duration-r.Position, 0), r.Completion

		if kind == "audiobook" {
			bookProgress(&r)
		}

		out = append(out, r)
	}

	return out
}

// bookProgress sets an audiobook item's part, the time left of the whole
// book and its progress, from the parts' running times.
func bookProgress(r *ContinueItem) {
	parts := StructscanT[struct {
		ID        uint    `db:"id"`
		RuntimeMS float64 `db:"runtime_ms"`
	}](false, 16, "select id, coalesce(runtime_ms, 0) as runtime_ms from audiobook_files where audiobook_id = ? "+
		"order by coalesce(disc_number, 0), coalesce(track_number, 0), location", &r.EntryID)
	if len(parts) < 2 {
		return
	}

	total, done := 0.0, 0.0

	for i, p := range parts {
		length := p.RuntimeMS / 1000

		switch {
		case p.ID == r.FileID:
			r.Part = i + 1
			length = max(length, r.Duration)
			done += min(r.Position, length)
		case r.Part == 0:
			done += length // parts before the current one count as heard
		}

		total += length
	}

	r.Parts = len(parts)
	if r.Part == 0 || total <= 0 {
		r.Part, r.Parts = 0, 0
		return
	}

	r.Left, r.Progress = max(total-done, 0), done/total
}

// LastPlayedFile is the file of a list entry (files table and its column
// referencing the entry) the account played last (0: none).
func LastPlayedFile(username, kind, filesTable, entryColumn string, entryID *uint64) uint {
	return Getdatarow[uint](false, "select h.file_id from play_history h inner join "+filesTable+" f on f.id = h.file_id "+
		"where h.username = ? and h.kind = ? and h.plays > 0 and f."+entryColumn+" = ? order by h.updated_at desc, h.id desc limit 1",
		username, kind, entryID)
}

// FinishAudiobook marks every part of an audiobook heard when the account
// finished its last part (in playing order); true when it did.
func FinishAudiobook(username string, fileID *uint) bool {
	last := Getdatarow[uint](false, `select f2.id from audiobook_files f inner join audiobook_files f2 on f2.audiobook_id = f.audiobook_id
		where f.id = ? order by coalesce(f2.disc_number, 0) desc, coalesce(f2.track_number, 0) desc, f2.location desc limit 1`, fileID)
	if last == 0 || last != *fileID {
		return false
	}

	return ExecNErr(`insert into play_history (username, kind, file_id, completion, plays)
		select ?, 'audiobook', f2.id, 1, 0 from audiobook_files f inner join audiobook_files f2 on f2.audiobook_id = f.audiobook_id where f.id = ?
		on conflict (username, kind, file_id) do update set completion = 1, position = 0`, username, fileID) == nil
}

// SetEntryWatched marks every file of a list entry (files table and its
// column referencing the entry) watched, or removes their history.
func SetEntryWatched(username, kind, filesTable, entryColumn string, entryID *uint64, watched bool) error {
	if !watched {
		return ExecNErr("delete from play_history where username = ? and kind = ? and file_id in (select id from "+filesTable+
			" where "+entryColumn+" = ?)", username, kind, entryID)
	}

	return ExecNErr("insert into play_history (username, kind, file_id, completion, plays) select ?, ?, id, 1, 0 from "+filesTable+
		" where "+entryColumn+" = ? on conflict (username, kind, file_id) do update set completion = 1, position = 0, "+
		"updated_at = datetime('now','localtime')", username, kind, entryID)
}

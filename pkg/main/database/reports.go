package database

import "errors"

// MediaReport is a file an account reported as wrong (media_reports).
type MediaReport struct {
	ID        uint   `db:"id"`
	Username  string `db:"username"`
	Kind      string `db:"kind"` // library type: movie, series, album, audiobook, book
	FileID    uint   `db:"file_id"`
	Title     string `db:"title"`
	Reason    string `db:"reason"`
	Note      string `db:"note"`
	CreatedAt string `db:"created_at"`
}

// ReportReasons are the reasons a file can be reported for, with their
// labels.
var ReportReasons = map[string]string{
	"quality":  "Bad quality",
	"language": "Wrong language",
	"content":  "Wrong content",
	"other":    "Something else",
}

// ErrReportDuplicate: the account reported the file already.
var ErrReportDuplicate = errors.New("you reported this file already")

const reportColumns = "id, username, kind, file_id, title, reason, note, created_at"

// AddMediaReport records a report; one open report per account and file.
func AddMediaReport(r *MediaReport) error {
	if _, ok := ReportReasons[r.Reason]; !ok {
		r.Reason = "other"
	}

	if Getdatarow[bool](false, "select exists(select 1 from media_reports where username = ? and kind = ? and file_id = ?)",
		&r.Username, &r.Kind, &r.FileID) {
		return ErrReportDuplicate
	}

	id, err := ExecNid("insert into media_reports (username, kind, file_id, title, reason, note) values (?, ?, ?, ?, ?, ?)",
		&r.Username, &r.Kind, &r.FileID, &r.Title, &r.Reason, &r.Note)
	if err == nil {
		r.ID = uint(id) //nolint:gosec // row ids are positive
	}

	return err
}

// GetMediaReports returns the open reports, oldest first.
func GetMediaReports() []MediaReport {
	return StructscanT[MediaReport](false, 0, "select "+reportColumns+" from media_reports order by id")
}

// GetMediaReport returns one report.
func GetMediaReport(id *uint) (*MediaReport, error) {
	return Structscan[MediaReport]("select "+reportColumns+" from media_reports where id = ?", false, id)
}

// CountMediaReports is the number of open reports.
func CountMediaReports() uint {
	return Getdatarow[uint](false, "select count() from media_reports")
}

// CloseMediaReports removes the reports of a file (all accounts): it was
// handled.
func CloseMediaReports(kind string, fileID *uint) error {
	return ExecNErr("delete from media_reports where kind = ? and file_id = ?", kind, fileID)
}

// CloseMediaReport removes one report (dismissed).
func CloseMediaReport(id *uint) error {
	return ExecNErr("delete from media_reports where id = ?", id)
}

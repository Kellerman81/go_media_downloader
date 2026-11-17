package api

import (
	"archive/zip"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/gin-gonic/gin"
	"maragu.dev/gomponents"
	hx "maragu.dev/gomponents-htmx"
	"maragu.dev/gomponents/html"
)

// Downloading files (the download feature) and reporting them as wrong
// (requestdelete): buttons in the Library's detail dialog, the reports
// page for admins.

// libFileButtons are a file's download and report buttons (hidden by the
// Library page for accounts without the feature).
func libFileButtons(k *libKind, fileID uint, label string) gomponents.Node {
	id := strconv.FormatUint(uint64(fileID), 10)

	return gomponents.Group([]gomponents.Node{
		html.A(html.Class("btn btn-sm btn-outline-secondary py-0 me-1 lib-download"), html.Title("Download"),
			html.Href("/api/admin/library/download/"+k.Type+"/"+id), gomponents.Attr("download"),
			html.I(html.Class("fa-solid fa-download"))),
		html.Button(html.Type("button"), html.Class("btn btn-sm btn-outline-warning py-0 me-1 lib-report"),
			html.Title("Report this file as wrong (quality, language, content)"),
			gomponents.Attr("onclick", "libReport("+jsString(k.Type)+", "+id+", "+jsString(label)+")"),
			html.I(html.Class("fa-solid fa-flag"))),
	})
}

// libEntryDownload is the "Download all" button of an album or audiobook.
func libEntryDownload(k *libKind, entryID uint, files int) gomponents.Node {
	if files < 2 || (k.Type != "album" && k.Type != "audiobook") {
		return nil
	}

	return html.A(html.Class("btn btn-sm btn-outline-secondary lib-download"), html.Title("All files as one ZIP"),
		html.Href("/api/admin/library/download-entry/"+k.Type+"/"+strconv.FormatUint(uint64(entryID), 10)),
		gomponents.Attr("download"), html.I(html.Class("fa-solid fa-file-zipper me-1")), gomponents.Text("Download all"))
}

// libFileOf returns the kind, id and location of the request's :type/:id
// file when the account may see it.
func libFileOf(c *gin.Context, typ, idStr string) (*libKind, uint, string, bool) {
	k := libKindByType(typ)
	id, err := strconv.ParseUint(idStr, 10, 64)

	if k == nil || err != nil || !accessOf(c).allowsFile(k, &id) {
		return nil, 0, "", false
	}

	loc := database.Getdatarow[string](false, "SELECT location FROM "+k.Files+" WHERE id = ?", &id)

	return k, uint(id), loc, loc != ""
}

// attachment is a Content-Disposition for a download named name.
func attachment(name string) string {
	return mime.FormatMediaType("attachment", map[string]string{"filename": name})
}

// HandleLibraryDownload sends one file (/admin/library/download/:type/:id).
func HandleLibraryDownload(c *gin.Context) {
	if !accessOf(c).CanDownload() {
		c.String(http.StatusForbidden, "downloads are not enabled for this account")
		return
	}

	k, id, loc, ok := libFileOf(c, c.Param("type"), c.Param("id"))
	if !ok {
		c.String(http.StatusNotFound, "file not found")
		return
	}

	if st, err := os.Stat(loc); err != nil || st.IsDir() {
		c.String(http.StatusNotFound, "the file is not on disk")
		return
	}

	done, ok := streamGuard(c, "download", k.Type+":"+strconv.FormatUint(uint64(id), 10))
	if !ok {
		return
	}
	defer done()

	c.Header("Content-Disposition", attachment(filepath.Base(loc)))
	c.File(loc)
}

// HandleLibraryDownloadEntry sends all files of an album or audiobook as
// one ZIP, stored without compression (media files don't shrink)
// (/admin/library/download-entry/:type/:id).
func HandleLibraryDownloadEntry(c *gin.Context) {
	acc := accessOf(c)
	k := libKindByType(c.Param("type"))
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)

	if !acc.CanDownload() || k == nil || err != nil || (k.Type != "album" && k.Type != "audiobook") || !acc.allowsEntry(k, &id) {
		c.String(http.StatusNotFound, "not found")
		return
	}

	locs := database.GetrowsN[string](false, 32, "SELECT location FROM "+k.Files+" WHERE "+k.FilesFK+
		" = ? AND location != '' ORDER BY "+libFileOrder[k.Type], &id)
	if len(locs) == 0 {
		c.String(http.StatusNotFound, "no files")
		return
	}

	done, ok := streamGuard(c, "download", k.Type+"-all:"+c.Param("id"))
	if !ok {
		return
	}
	defer done()

	base := filepath.Dir(locs[0])

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", attachment(filepath.Base(base)+".zip"))
	c.Status(http.StatusOK)

	zw := zip.NewWriter(c.Writer)

	for _, loc := range locs {
		name, err := filepath.Rel(base, loc)
		if err != nil || strings.HasPrefix(name, "..") {
			name = filepath.Base(loc)
		}

		if err := zipFile(zw, loc, filepath.ToSlash(name)); err != nil {
			logger.Logtype("warn", 0).Err(err).Str("file", loc).Msg("Library: download as ZIP")
			return
		}
	}

	zw.Close()
}

func zipFile(zw *zip.Writer, loc, name string) error {
	f, err := os.Open(loc)
	if err != nil {
		return err
	}
	defer f.Close()

	st, err := f.Stat()
	if err != nil {
		return err
	}

	hdr, err := zip.FileInfoHeader(st)
	if err != nil {
		return err
	}

	hdr.Name, hdr.Method = name, zip.Store

	w, err := zw.CreateHeader(hdr)
	if err != nil {
		return err
	}

	_, err = io.Copy(w, f)

	return err
}

// HandleLibraryReport records a report of a file (/admin/library/report:
// type, id, reason, note).
func HandleLibraryReport(c *gin.Context) {
	acc := accessOf(c)
	if !acc.CanReport() {
		c.JSON(http.StatusForbidden, gin.H{"error": "reports are not enabled for this account"})
		return
	}

	k, id, loc, ok := libFileOf(c, c.PostForm("type"), c.PostForm("id"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}

	note := strings.TrimSpace(c.PostForm("note"))
	r := database.MediaReport{Username: accName(acc), Kind: k.Type, FileID: id, Title: filepath.Base(loc),
		Reason: c.PostForm("reason"), Note: note[:min(len(note), 500)]}

	if err := database.AddMediaReport(&r); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, database.ErrReportDuplicate) {
			status = http.StatusConflict
		}

		c.JSON(status, gin.H{"error": err.Error()})

		return
	}

	logger.Logtype("info", 0).Str("user", r.Username).Str("type", k.Type).Uint("file", id).Str("reason", r.Reason).
		Msg("Library: file reported")

	listname, title := libFileEntry(k, &id)
	notifyAccountEvent(c, providers.EventReported, "/api/admin/reports", &accountEvent{List: listname, User: r.Username, Type: k.Type,
		Title: title, Reason: database.ReportReasons[r.Reason], Note: r.Note, File: r.Title})
	c.JSON(http.StatusOK, gin.H{"reported": true})
}

// ReportsPage lists the reported files (/admin/reports, admins).
func ReportsPage(c *gin.Context) {
	renderPageIn(c, "Reports", true, html.Div(html.Class("config-section-enhanced"),
		renderEnhancedPageHeader("fa-solid fa-flag", "Reported Files",
			"Files accounts reported as wrong. Delete a file to have it searched again, or dismiss the report."),
		html.Div(html.ID("reportsContent"), renderReports(getCSRFToken(c), ""))))
}

func renderReports(csrf, alert string) gomponents.Node {
	reports := database.GetMediaReports()
	rows := make([]gomponents.Node, 0, len(reports))

	for i := range reports {
		r := &reports[i]
		id := strconv.FormatUint(uint64(r.ID), 10)

		var loc string
		if k := libKindByType(r.Kind); k != nil {
			loc = database.Getdatarow[string](false, "SELECT location FROM "+k.Files+" WHERE id = ?", &r.FileID)
		}

		action := func(act, label, class, confirm string) gomponents.Node {
			return html.Button(html.Type("button"), html.Class("btn btn-sm "+class+" me-1 mb-1"),
				hx.Post("/api/admin/reports/resolve"), hx.Vals(`{"id": "`+id+`", "action": "`+act+`"}`), hx.Headers(wizardCSRFHeader(csrf)),
				hx.Target("#reportsContent"), hx.Swap("innerHTML"), gomponents.If(confirm != "", hx.Confirm(confirm)), gomponents.Text(label))
		}

		rows = append(rows, html.Tr(
			html.Td(html.Class("small text-nowrap"), gomponents.Text(shortTime(r.CreatedAt))),
			html.Td(html.Strong(gomponents.Text(r.Username))),
			html.Td(html.Span(html.Class("badge text-bg-warning"), gomponents.Text(database.ReportReasons[r.Reason])),
				gomponents.If(r.Note != "", html.Div(html.Class("small"), gomponents.Text(r.Note)))),
			html.Td(html.Div(gomponents.Text(r.Title)),
				html.Div(html.Class("small text-muted font-monospace text-break"), gomponents.Text(orDash(loc)))),
			html.Td(html.Class("text-end"),
				gomponents.If(loc != "", action("research", "Delete & search again", "btn-danger",
					"Delete this file from disk and search the entry again?")),
				gomponents.If(loc != "", action("delete", "Delete file", "btn-outline-danger", "Delete this file from disk?")),
				action("dismiss", "Dismiss", "btn-outline-secondary", "")),
		))
	}

	list := gomponents.Node(html.P(html.Class("text-muted mb-0"), gomponents.Text("No reported files.")))
	if len(rows) > 0 {
		list = html.Div(html.Class("table-responsive"), html.Table(html.Class("table table-sm align-middle mb-0"),
			html.THead(html.Tr(html.Th(gomponents.Text("Reported")), html.Th(gomponents.Text("By")), html.Th(gomponents.Text("Reason")),
				html.Th(gomponents.Text("File")), html.Th())),
			html.TBody(gomponents.Group(rows))))
	}

	return html.Div(gomponents.Raw(alert), html.Div(html.Class("card"), html.Div(html.Class("card-body"), list)))
}

// HandleReportResolve deletes a reported file (research: and marks its
// entry missing, so the searches get it again) or dismisses the report
// (/admin/reports/resolve: id, action).
func HandleReportResolve(c *gin.Context) {
	reply := func(msg, level string) {
		c.Header("Content-Type", "text/html")
		c.String(http.StatusOK, renderComponentToString(renderReports(getCSRFToken(c), renderAlert(msg, level))))
	}

	rid64, err := strconv.ParseUint(c.PostForm("id"), 10, 64)
	rid := uint(rid64)

	r, err2 := database.GetMediaReport(&rid)
	if err != nil || err2 != nil || r.ID == 0 {
		reply("The report no longer exists.", "warning")
		return
	}

	action := c.PostForm("action")
	if action == "dismiss" {
		database.CloseMediaReport(&rid)
		reply("Report dismissed.", "success")

		return
	}

	k := libKindByType(r.Kind)
	if k == nil || (action != "delete" && action != "research") {
		reply("Unknown action.", "danger")
		return
	}

	loc := database.Getdatarow[string](false, "SELECT location FROM "+k.Files+" WHERE id = ?", &r.FileID)
	if loc != "" {
		if _, err := scanner.RemoveFile(loc); err != nil {
			logger.Logtype("error", 0).Err(err).Str("file", loc).Msg("Reports: delete file")
			reply("The file could not be deleted: "+err.Error(), "danger")

			return
		}
	}

	if action == "research" {
		markFileMissing(k, &r.FileID)
	}

	database.DeleteRow(k.Files, "id = ?", &r.FileID)
	database.CloseMediaReports(r.Kind, &r.FileID)

	logger.Logtype("info", 0).Str("type", k.Type).Str("file", loc).Str("action", action).Msg("Reports: file deleted")

	msg := "Deleted " + filepath.Base(loc) + "."
	if action == "research" {
		msg += " The entry is missing again and is searched by the next search runs (or use its Search button in the Library)."
	}

	reply(msg, "success")
}

// libFileEntry is the list and title of the entry a file belongs to.
func libFileEntry(k *libKind, fileID *uint) (string, string) {
	entryID := database.Getdatarow[uint64](false, "SELECT "+k.FilesFK+" FROM "+k.Files+" WHERE id = ?", fileID)
	if it := loadLibraryItem(k, &entryID); it != nil {
		return it.Listname, it.Title
	}

	return "", ""
}

// markFileMissing marks the list entry (episode) of a file missing and not
// at its cutoff, so the searches look for it again.
func markFileMissing(k *libKind, fileID *uint) {
	if k.Type == "series" {
		database.ExecN("UPDATE serie_episodes SET missing = 1, quality_reached = 0 WHERE id = "+
			"(SELECT serie_episode_id FROM serie_episode_files WHERE id = ?)", fileID)

		return
	}

	database.ExecN("UPDATE "+k.entryTable()+" SET missing = 1, quality_reached = 0 WHERE id = "+
		"(SELECT "+k.FilesFK+" FROM "+k.Files+" WHERE id = ?)", fileID)
}

// reportsBadge is the number of open reports, when there are any.
func reportsBadge() gomponents.Node {
	n := database.CountMediaReports()
	if n == 0 {
		return nil
	}

	return html.Span(html.Class("badge bg-warning text-dark ms-2"), gomponents.Text(strconv.FormatUint(uint64(n), 10)))
}

package structure

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/apiexternal_v2/providers/lyrics"
	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/importfeed"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype/mtstrings"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/providers"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
	"github.com/Kellerman81/go_media_downloader/pkg/main/searcher"
	"github.com/Kellerman81/go_media_downloader/pkg/main/tags"
)

// moveUnprocessedFolder moves a folder to the MoveUnprocessed target path under a reason subfolder.
// The resulting path is: moveUnprocessedPath/reason/basename(folder)/
// If moveUnprocessedPath is empty, this is a no-op.
// chmod and chmodFolder apply file/folder permissions after the move (like performMove does).
func moveUnprocessedFolder(
	folder, moveUnprocessedPath, reason, chmod, chmodFolder string,
	report *importfeed.MatchReport,
) {
	if moveUnprocessedPath == "" || reason == "" {
		return
	}

	var folderMode, fileMode fs.FileMode
	if len(chmodFolder) == 4 {
		folderMode = logger.StringToFileMode(chmodFolder)
	}

	if len(chmod) == 4 {
		fileMode = logger.StringToFileMode(chmod)
	}

	reasonDir := filepath.Join(moveUnprocessedPath, reason)
	if err := os.MkdirAll(reasonDir, 0o777); err != nil {
		logger.Logtype("error", 1).
			Str("path", reasonDir).
			Err(err).
			Msg("Failed to create unprocessed reason directory")

		return
	}

	if folderMode != 0 {
		os.Chmod(reasonDir, folderMode)
	}

	target := filepath.Join(reasonDir, filepath.Base(folder))
	if err := scanner.MoveFolder(
		folder,
		target,
		scanner.MoveFileOptions{Chmod: chmod, ChmodFolder: chmodFolder},
	); err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, folder).
			Str("target", target).
			Str("reason", reason).
			Err(err).
			Msg("Failed to move unprocessed folder")

		return
	}

	logger.Logtype("info", 1).
		Str(logger.StrFile, folder).
		Str("target", target).
		Str("reason", reason).
		Msg("Moved unprocessed folder")

	if report != nil {
		writeMatchReportFile(target, report)
	}

	// Apply chmod to moved files and folders
	if folderMode != 0 || fileMode != 0 {
		filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}

			if d.IsDir() {
				if folderMode != 0 {
					os.Chmod(path, folderMode)
				}
			} else {
				if fileMode != 0 {
					os.Chmod(path, fileMode)
				}
			}

			return nil
		})
	}
}

// writeMatchReportFile writes a human-readable _match_report.txt into targetFolder.
// rptColInt writes n left-aligned in a field of width bytes followed by a space separator.
func rptColInt(buf *logger.AddBuffer, n, width int) {
	s := strconv.Itoa(n)
	buf.WriteString(s)

	for i := len(s); i < width; i++ {
		buf.WriteByte(' ')
	}

	buf.WriteByte(' ')
}

// rptColStr writes s left-aligned in a field of width bytes followed by a space separator.
func rptColStr(buf *logger.AddBuffer, s string, width int) {
	s = truncateUTF8(s, width)
	n := len(s)
	buf.WriteString(s)

	for i := n; i < width; i++ {
		buf.WriteByte(' ')
	}

	buf.WriteByte(' ')
}

func writeMatchReportFile(targetFolder string, report *importfeed.MatchReport) {
	reportPath := filepath.Join(targetFolder, "_match_report.txt")

	f, err := os.Create(reportPath)
	if err != nil {
		logger.Logtype("error", 1).
			Str("path", reportPath).
			Err(err).
			Msg("Failed to write match report")

		return
	}
	defer f.Close()

	buf := logger.PlAddBuffer.Get()
	defer logger.PlAddBuffer.Put(buf)

	buf.WriteString("Match Report\n============\n")
	buf.WriteString("Denial:      ")
	buf.WriteString(report.DenialReason)
	buf.WriteString("\nLocal:       ")
	buf.WriteInt(report.ActualTracks)
	buf.WriteString(" tracks, ")
	buf.WriteString(matchReportDuration(report.ActualRuntimeMS))
	buf.WriteString(" runtime\n\n")

	for i := range report.Candidates {
		buf.WriteString("Candidate ")
		buf.WriteInt(i + 1)
		buf.WriteString("  (album dist: ")
		buf.WriteString(strconv.FormatFloat(report.Candidates[i].AlbumDist, 'f', 3, 64))

		if report.Candidates[i].FullDist > 0 {
			buf.WriteString(", full dist: ")
			buf.WriteString(strconv.FormatFloat(report.Candidates[i].FullDist, 'f', 3, 64))
		}

		buf.WriteString(")\n  Title:    ")
		buf.WriteString(report.Candidates[i].Title)
		buf.WriteString("\n  Artist:   ")
		buf.WriteString(report.Candidates[i].Artist)
		buf.WriteByte('\n')

		if report.Candidates[i].MBID != "" {
			buf.WriteString("  ID:       ")
			buf.WriteString(report.Candidates[i].MBID)
			buf.WriteByte('\n')
		}

		if report.Candidates[i].Year > 0 {
			buf.WriteString("  Year:     ")
			buf.WriteInt(report.Candidates[i].Year)
			buf.WriteByte('\n')
		}

		buf.WriteString("  Expected: ")
		buf.WriteInt(report.Candidates[i].ExpectedTracks)
		buf.WriteString(" tracks, ")
		buf.WriteString(matchReportDuration(int64(report.Candidates[i].ExpectedRuntimeMS)))
		buf.WriteString(" runtime\n")

		if len(report.Candidates[i].Tracks) > 0 {
			buf.WriteString("  ")
			rptColStr(buf, "Track", 5)
			rptColStr(buf, "Disc", 4)
			rptColStr(buf, "Title", 32)
			rptColStr(buf, "DB Runtime", 11)
			rptColStr(buf, "Local", 11)
			buf.WriteString("Dist\n")

			for j := range report.Candidates[i].Tracks {
				title := report.Candidates[i].Tracks[j].Title
				if len(title) > 32 {
					title = truncateUTF8(title, 29) + "..."
				}

				buf.WriteString("  ")
				rptColInt(buf, report.Candidates[i].Tracks[j].TrackNumber, 5)
				rptColInt(buf, report.Candidates[i].Tracks[j].DiscNumber, 4)
				rptColStr(buf, title, 32)

				switch {
				case report.Candidates[i].Tracks[j].LocalOnly:
					rptColStr(buf, "---", 11)
					rptColStr(
						buf,
						matchReportDuration(report.Candidates[i].Tracks[j].LocalRuntimeMS),
						11,
					)
					buf.WriteString("(local only)")

				case report.Candidates[i].Tracks[j].Unmatched:
					rptColStr(
						buf,
						matchReportDuration(report.Candidates[i].Tracks[j].DBRuntimeMS),
						11,
					)
					rptColStr(
						buf,
						matchReportDuration(report.Candidates[i].Tracks[j].LocalRuntimeMS),
						11,
					)
					buf.WriteString(
						strconv.FormatFloat(report.Candidates[i].Tracks[j].TrackDist, 'f', 3, 64),
					)
					buf.WriteString(" (unmatched)")

				default:
					rptColStr(
						buf,
						matchReportDuration(report.Candidates[i].Tracks[j].DBRuntimeMS),
						11,
					)
					rptColStr(
						buf,
						matchReportDuration(report.Candidates[i].Tracks[j].LocalRuntimeMS),
						11,
					)
					buf.WriteString(
						strconv.FormatFloat(report.Candidates[i].Tracks[j].TrackDist, 'f', 3, 64),
					)
				}

				buf.WriteByte('\n')
			}
		}

		buf.WriteByte('\n')
	}

	if _, err := f.Write(buf.Bytes()); err != nil {
		logger.Logtype("error", 1).
			Str("path", reportPath).
			Err(err).
			Msg("Failed to write match report")
	}
}

// albumLocks serializes imports of the same album (keyed by media type and
// db id); see organizeAlbumFolderViaAPI.
var albumLocks sync.Map

// lockAlbum takes the import lock of one album and returns its release.
func lockAlbum(mediaType, dbid uint) func() {
	mu, _ := albumLocks.LoadOrStore([2]uint{mediaType, dbid}, &sync.Mutex{})
	m := mu.(*sync.Mutex)
	m.Lock()

	return m.Unlock
}

// truncateUTF8 cuts s to at most n bytes without splitting a UTF-8 sequence.
func truncateUTF8(s string, n int) string {
	if len(s) <= n {
		return s
	}

	for n > 0 && !utf8.RuneStart(s[n]) {
		n--
	}

	return s[:n]
}

// matchReportDuration formats milliseconds as M:SS or H:MM:SS.
func matchReportDuration(ms int64) string {
	if ms <= 0 {
		return "0:00"
	}

	s := ms / 1000
	h := s / 3600
	m := (s % 3600) / 60

	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, m, sec)
	}

	return fmt.Sprintf("%d:%02d", m, sec)
}

// buildAlbumDataConfig converts a MediaDataImportConfig into the MediaDataConfig
// expected by importfeed, then merges any unset matching parameters from the media
// type's data configs (cfgp.Data). Settings from data_import take precedence; the
// per-data merge fills gaps for track weights, tolerances, release types, and the
// add_found / alternative-release behaviour. Centralising this prevents the match
// configuration from drifting between the normal organize path and force-match.
func buildAlbumDataConfig(
	data *config.MediaDataImportConfig,
	cfgp *config.MediaTypeConfig,
) *config.MediaDataConfig {
	dataCfg := &config.MediaDataConfig{}
	if data != nil {
		dataCfg.EnableUnpacking = data.EnableUnpacking
		dataCfg.CfgPath = data.CfgPath
		dataCfg.PerTrackToleranceSeconds = data.PerTrackToleranceSeconds
		dataCfg.MaxTotalDifferenceSeconds = data.MaxTotalDifferenceSeconds
		dataCfg.AllowMissingTracks = data.AllowMissingTracks
		dataCfg.AllowAllFormatsWhenStructuring = data.AllowAllFormatsWhenStructuring
		dataCfg.AllowAlternativeReleases = data.AllowAlternativeReleases
		dataCfg.TrackArtistWeight = data.TrackArtistWeight
		dataCfg.TrackTitleWeight = data.TrackTitleWeight
		dataCfg.TrackLengthWeight = data.TrackLengthWeight
		dataCfg.TrackIdWeight = data.TrackIdWeight
		dataCfg.TrackIndexWeight = data.TrackIndexWeight
		dataCfg.PerTrackToleranceSecondsMax = data.PerTrackToleranceSecondsMax
		dataCfg.MBMediaFormats = data.MBMediaFormats
		dataCfg.AllowedReleaseTypes = data.AllowedReleaseTypes
		dataCfg.EmbedArt = data.EmbedArt
		dataCfg.ExceedToleranceIfTotalMatch = data.ExceedToleranceIfTotalMatch
		dataCfg.DiscoverSeriesAlbums = data.DiscoverSeriesAlbums
		dataCfg.StrongRecThresh = data.StrongRecThresh
		dataCfg.MediumRecThresh = data.MediumRecThresh
		// Use add_found settings directly from the import config if set.
		if data.AddFound {
			dataCfg.AddFound = true
			dataCfg.AddFoundList = data.AddFoundList
		}
	}

	// Copy AddFound and AllowAlternativeReleases settings from the media type's data configs so that
	// searchAndImportAlternativeRelease can find matching releases.
	// data_import.add_found (set above) takes precedence when present.
	for idx := range cfgp.Data {
		if cfgp.Data[idx].AddFound && !dataCfg.AddFound {
			dataCfg.AddFound = true
			dataCfg.AddFoundList = cfgp.Data[idx].AddFoundList
		}

		if !dataCfg.WriteRenameLog && cfgp.Data[idx].WriteRenameLog {
			dataCfg.WriteRenameLog = true
		}

		if !dataCfg.EmbedArt && cfgp.Data[idx].EmbedArt {
			dataCfg.EmbedArt = true
		}

		if !dataCfg.AllowAllFormatsWhenStructuring &&
			cfgp.Data[idx].AllowAllFormatsWhenStructuring {
			dataCfg.AllowAllFormatsWhenStructuring = true
		}

		if !dataCfg.AllowMissingTracks && cfgp.Data[idx].AllowMissingTracks {
			dataCfg.AllowMissingTracks = true
		}

		if len(dataCfg.MBMediaFormats) == 0 && len(cfgp.Data[idx].MBMediaFormats) > 0 {
			dataCfg.MBMediaFormats = cfgp.Data[idx].MBMediaFormats
		}

		if len(dataCfg.AllowedReleaseTypes) == 0 && len(cfgp.Data[idx].AllowedReleaseTypes) > 0 {
			dataCfg.AllowedReleaseTypes = cfgp.Data[idx].AllowedReleaseTypes
		}

		if dataCfg.StrongRecThresh == 0 && cfgp.Data[idx].StrongRecThresh > 0 {
			dataCfg.StrongRecThresh = cfgp.Data[idx].StrongRecThresh
		}

		if dataCfg.MediumRecThresh == 0 && cfgp.Data[idx].MediumRecThresh > 0 {
			dataCfg.MediumRecThresh = cfgp.Data[idx].MediumRecThresh
		}

		if dataCfg.TrackArtistWeight == 0 && cfgp.Data[idx].TrackArtistWeight > 0 {
			dataCfg.TrackArtistWeight = cfgp.Data[idx].TrackArtistWeight
		}

		if dataCfg.TrackTitleWeight == 0 && cfgp.Data[idx].TrackTitleWeight > 0 {
			dataCfg.TrackTitleWeight = cfgp.Data[idx].TrackTitleWeight
		}

		if dataCfg.TrackLengthWeight == 0 && cfgp.Data[idx].TrackLengthWeight > 0 {
			dataCfg.TrackLengthWeight = cfgp.Data[idx].TrackLengthWeight
		}

		if dataCfg.TrackIdWeight == 0 && cfgp.Data[idx].TrackIdWeight > 0 {
			dataCfg.TrackIdWeight = cfgp.Data[idx].TrackIdWeight
		}

		if dataCfg.TrackIndexWeight == 0 && cfgp.Data[idx].TrackIndexWeight > 0 {
			dataCfg.TrackIndexWeight = cfgp.Data[idx].TrackIndexWeight
		}

		if dataCfg.PerTrackToleranceSecondsMax == 0 &&
			cfgp.Data[idx].PerTrackToleranceSecondsMax > 0 {
			dataCfg.PerTrackToleranceSecondsMax = cfgp.Data[idx].PerTrackToleranceSecondsMax
		}

		if dataCfg.PerTrackToleranceSeconds == 0 &&
			cfgp.Data[idx].PerTrackToleranceSeconds > 0 {
			dataCfg.PerTrackToleranceSeconds = cfgp.Data[idx].PerTrackToleranceSeconds
		}

		if dataCfg.MaxTotalDifferenceSeconds == 0 &&
			cfgp.Data[idx].MaxTotalDifferenceSeconds > 0 {
			dataCfg.MaxTotalDifferenceSeconds = cfgp.Data[idx].MaxTotalDifferenceSeconds
		}

		if !cfgp.Data[idx].AllowAlternativeReleases {
			continue
		}

		dataCfg.AllowAlternativeReleases = true
		// Ensure we have a list to import alternative releases into
		if dataCfg.AddFoundList == "" {
			dataCfg.AddFoundList = cfgp.Data[idx].AddFoundList
		}
	}

	return dataCfg
}

// organizeAlbumFolderViaAPI processes multi-file media (music, audiobooks) using API matching.
// It uses importfeed.MatchAudioFolderAsAlbum to match the album, then organizes the matched files.
// Workflow: Match via API → Upgrade check against old files → Tag and rename → Move →
// Retire old files → Add to DB.
// Returns the rejection reason from matching (empty on success) and any error.
func (s *Organizer) organizeAlbumFolderViaAPI(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
) (string, *importfeed.MatchReport, error) {
	// Convert MediaDataImportConfig to MediaDataConfig for importfeed.
	dataCfg := buildAlbumDataConfig(data, cfgp)

	// Check for multi-episode audiobook folder (each file is a separate episode)
	if cfgp.IsType == config.MediaTypeAudiobook {
		files, filesErr := parser_v2.CollectFilesOnly(folder, parser_v2.AudioExtensions)
		if filesErr == nil && len(files) > 1 &&
			parser_v2.IsMultiEpisodeAudiobookFolder(folder, files) {
			r, e := s.organizeMultiEpisodeFolder(ctx, folder, files, cfgp, data, dataCfg)
			return r, nil, e
		}
	}

	// Step 1: Match album via API (without adding to database yet).
	// Use the forced ID when set (supplied by ForceMatchAlbumFolder run mode).
	album, matchReason, matchReport := importfeed.MatchAudioFolderAsAlbumForced(
		ctx,
		folder,
		cfgp,
		dataCfg,
		false,
		&s.forcedAlbumID,
	)
	// A zero DatabaseID is a failed match however the matcher labelled it:
	// everything below this point looks up or writes rows keyed by that id, so
	// accepting one burns the old-file queries, the
	// listname/rootpath lookups, the Organizerdata allocation and the whole
	// naming pass before dying in GenerateNamingTemplate.
	if matchReason != "" || album == nil || album.DatabaseID == 0 {
		logger.Logtype("debug", 0).
			Str("folder", folder).
			Str("reason", matchReason).
			Msg("Failed to match album via API - skipping")

		if matchReason == "" {
			matchReason = "no_match"
		}

		um := database.ParseInfo{TempTitle: folder}
		um.AddUnmatched(cfgp, &logger.StrStructure, errors.New(matchReason))

		return matchReason, matchReport, errUnprocessed
	}

	logger.Logtype("info", 1).
		Str("folder", folder).
		Str("title", album.Title).
		Str("artist", album.Artist).
		Uint("album_id", album.DatabaseID).
		Int("tracks", len(album.Tracks)).
		Msg("Matched album via API")

	// One import per album at a time: disc folders of one set (or the same
	// folder submitted twice) resolve to the same album and files, and
	// running them in parallel moved and retired each other's tracks.
	unlock := lockAlbum(cfgp.IsType, album.DatabaseID)
	defer unlock()

	for idx := range album.Tracks {
		if !scanner.CheckFileExist(album.Tracks[idx].Filepath) {
			logger.Logtype("info", 1).
				Str("folder", folder).
				Str(logger.StrFile, album.Tracks[idx].Filepath).
				Msg("album files were organized by another run meanwhile - skipping")

			return "already_organized", matchReport, nil
		}
	}

	// Step 2: Create ParseInfo for template generation and resolve the list
	// entry (albums.id / audiobooks.id) of the list this import targets.
	// album.DatabaseID is the dbalbums/dbaudiobooks id; everything keyed by
	// the list entry (files' album_id, missing/quality_reached) needs the
	// list entry id instead.
	m := database.ParseInfo{}

	m.Title = album.Title
	m.Year = uint16(album.Year) //nolint:gosec // safe: value within target type range

	// Get list ID and rootpath for this album
	var listname, rootpath string
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		database.GetdatarowArgs(
			"SELECT listname, rootpath FROM audiobooks WHERE dbaudiobook_id = ? LIMIT 1",
			album.DatabaseID,
			&listname, &rootpath)

	case config.MediaTypeMusic:
		database.GetdatarowArgs(
			"SELECT listname, rootpath FROM albums WHERE dbalbum_id = ? LIMIT 1",
			album.DatabaseID,
			&listname, &rootpath)
	}

	listid := cfgp.GetMediaListsEntryListID(listname)
	listEntryID := albumListEntryID(cfgp, listid, &album.DatabaseID)

	if listEntryID == 0 {
		listid, listEntryID = unlistedEntry(cfgp, dataCfg, &album.DatabaseID)
	}

	// Set the database ID based on media type
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		m.DbaudiobookID = album.DatabaseID
		m.AudiobookID = listEntryID

	case config.MediaTypeMusic:
		m.DbalbumID = album.DatabaseID
		m.AlbumID = listEntryID
	}

	// Create Organizerdata structure
	o := &Organizerdata{
		Folder:     folder,
		MediaFiles: make([]string, len(album.Tracks)),
		Listid:     listid,
		Rootpath:   rootpath,
	}

	// Populate MediaFiles array with track filepaths
	for idx, track := range album.Tracks {
		o.MediaFiles[idx] = track.Filepath
	}

	// Set first file as MediaFile for compatibility
	if len(o.MediaFiles) > 0 {
		o.MediaFile = o.MediaFiles[0]
	}

	// Step 3: Only replace what is already in the library when the new files
	// are an upgrade. Nothing is touched yet - tagging comes later, and old
	// files are only retired after the new ones are in place.
	oldFiles := oldAlbumFiles(cfgp.IsType, album.DatabaseID)
	if !albumUpgradeAllowed(cfgp, listid, album, oldFiles) {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Uint("album_id", album.DatabaseID).
			Msg("Existing album files are of equal or better quality - skipping import")

		m.TempTitle = folder
		m.AddUnmatched(cfgp, &logger.StrStructure, errLowerQuality)

		return "lower_quality", nil, errUnprocessed
	}

	// Step 4: Generate naming template for folder and filename
	s.GenerateNamingTemplate(o, &m, &album.DatabaseID)

	if o.Foldername == "" && (o.Rootpath == "" || o.Rootpath == s.targetpathCfg.Path) {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Msg("Failed to generate folder name")

		m.TempTitle = folder
		m.AddUnmatched(cfgp, &logger.StrStructure, errGeneratingFilename)

		return "naming_failed", nil, errGeneratingFilename
	}

	if o.Filename == "" {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Msg("Failed to generate file name")

		m.TempTitle = folder
		m.AddUnmatched(cfgp, &logger.StrStructure, errGeneratingFilename)

		return "naming_failed", nil, errGeneratingFilename
	}

	// Step 4b: Generate filenames for each track
	o.Filenames = s.generateTrackFilenames(o, &m, album, cfgp)

	// Step 5: Tag files with proper metadata
	if err := s.tagAlbumFiles(ctx, album); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to tag album files")
	}

	// Apply presort path if configured — redirect target to the staging folder.
	s.applyPresort(o)
	s.setTargetPath(o)

	// Step 6: Move files to target location, then retire the old files
	newpath, err := s.moveAlbumIntoPlace(ctx, folder, o, oldFiles, true)
	if err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to move album files")

		m.TempTitle = folder
		m.AddUnmatched(cfgp, &logger.StrStructure, err)

		return "move_failed", nil, err
	}

	// Step 7: Update album struct with new file paths after move
	targetPath := s.updateAlbumTrackPaths(o, album)

	logger.Logtype("info", 1).
		Str("folder", folder).
		Str("newpath", newpath).
		Str("target", targetPath).
		Int("tracks", len(album.Tracks)).
		Msg("Moved album files")

	// Write rename log immediately after the file move so it is always persisted,
	// even if the job context is cancelled before the database operations below.
	if dataCfg.WriteRenameLog {
		s.writeRenameLog(ctx, targetPath, folder, album, o, cfgp)
	}

	// Exit early if the job was cancelled — files and rename log are already written.
	if err := logger.CheckContextEnded(ctx); err != nil {
		return "", nil, err
	}

	// Step 8: Add new files to database and mark the list entry found
	s.recordImportedAlbum(ctx, targetPath, album, cfgp, listid, listEntryID)

	s.cleanUpFolder(folder)

	return "", nil, nil
}

// organizeMultiEpisodeFolder processes a folder containing multiple distinct
// audiobook episodes (each file is a separate audiobook). Each file is matched,
// named, and moved individually.
func (s *Organizer) organizeMultiEpisodeFolder(
	ctx context.Context,
	folder string,
	files []string,
	cfgp *config.MediaTypeConfig,
	_ *config.MediaDataImportConfig,
	dataCfg *config.MediaDataConfig,
) (string, error) {
	logger.Logtype("info", 1).
		Str("folder", folder).
		Int("fileCount", len(files)).
		Msg("Processing multi-episode audiobook folder - each file as individual audiobook")

	var matchedCount, unmatchedCount int

	for i := range files {
		if err := ctx.Err(); err != nil {
			return "cancelled", err
		}

		// Match this single file as its own audiobook
		album, matchReason := importfeed.MatchSingleAudiobookFile(
			ctx,
			files[i],
			cfgp,
			dataCfg,
			false,
		)
		if matchReason != "" || album == nil {
			logger.Logtype("debug", 0).
				Str("file", files[i]).
				Str("reason", matchReason).
				Msg("Multi-episode: file not matched - skipping")

			unmatchedCount++

			um := database.ParseInfo{TempTitle: files[i]}
			um.AddUnmatched(cfgp, &logger.StrStructure, errors.New("multi_episode_"+matchReason))

			continue
		}

		logger.Logtype("info", 1).
			Str("file", files[i]).
			Str("title", album.Title).
			Str("artist", album.Artist).
			Uint("dbID", album.DatabaseID).
			Msg("Multi-episode: matched audiobook")

		if len(album.Tracks) == 0 {
			logger.Logtype("warn", 1).
				Str("file", files[i]).
				Msg("Multi-episode: matched audiobook has no tracks - skipping")

			unmatchedCount++

			continue
		}

		// Create ParseInfo for template generation
		m := database.ParseInfo{}

		m.Title = album.Title
		m.Year = uint16(album.Year) //nolint:gosec // safe: value within target type range

		// Get list ID and rootpath for this audiobook
		var listname, rootpath string
		database.GetdatarowArgs(
			"SELECT listname, rootpath FROM audiobooks WHERE dbaudiobook_id = ? LIMIT 1",
			album.DatabaseID,
			&listname, &rootpath)

		listid := cfgp.GetMediaListsEntryListID(listname)
		listEntryID := albumListEntryID(cfgp, listid, &album.DatabaseID)

		if listEntryID == 0 {
			listid, listEntryID = unlistedEntry(cfgp, dataCfg, &album.DatabaseID)
		}

		m.DbaudiobookID = album.DatabaseID
		m.AudiobookID = listEntryID

		// Create Organizerdata for this single file
		o := &Organizerdata{
			Folder:     folder,
			MediaFiles: []string{album.Tracks[0].Filepath},
			MediaFile:  album.Tracks[0].Filepath,
			Listid:     listid,
			Rootpath:   rootpath,
		}

		// Only replace existing library files with an upgrade.
		oldFiles := oldAlbumFiles(cfgp.IsType, album.DatabaseID)
		if !albumUpgradeAllowed(cfgp, listid, album, oldFiles) {
			logger.Logtype("info", 1).
				Str("file", files[i]).
				Uint("dbID", album.DatabaseID).
				Msg("Multi-episode: existing files are of equal or better quality - skipping")

			unmatchedCount++

			continue
		}

		// Generate naming template
		s.GenerateNamingTemplate(o, &m, &album.DatabaseID)

		if o.Foldername == "" && (o.Rootpath == "" || o.Rootpath == s.targetpathCfg.Path) {
			logger.Logtype("error", 1).
				Str("file", files[i]).
				Msg("Multi-episode: failed to generate folder name")

			unmatchedCount++

			continue
		}

		if o.Filename == "" {
			logger.Logtype("error", 1).
				Str("file", files[i]).
				Msg("Multi-episode: failed to generate file name")

			unmatchedCount++

			continue
		}

		// Generate filename for the single track
		o.Filenames = s.generateTrackFilenames(o, &m, album, cfgp)

		// Tag the file
		if err := s.tagAlbumFiles(ctx, album); err != nil {
			logger.Logtype("error", 1).
				Str("file", files[i]).
				Err(err).
				Msg("Multi-episode: failed to tag file")
		}

		s.applyPresort(o)
		s.setTargetPath(o)

		// Move the file, then retire this audiobook's old files. Episodes can
		// share a target folder, so no sweep of other audio files there.
		newpath, err := s.moveAlbumIntoPlace(ctx, folder, o, oldFiles, false)
		if err != nil {
			logger.Logtype("error", 1).
				Str("file", files[i]).
				Err(err).
				Msg("Multi-episode: failed to move file")

			unmatchedCount++

			continue
		}

		// Update track paths after move
		targetPath := s.updateAlbumTrackPaths(o, album)

		logger.Logtype("info", 1).
			Str("file", files[i]).
			Str("newpath", newpath).
			Str("title", album.Title).
			Msg("Multi-episode: organized audiobook")

		// Write rename log if configured
		if dataCfg.WriteRenameLog {
			s.writeRenameLog(ctx, targetPath, folder, album, o, cfgp)
		}

		// Add file to database, clear missing and score what was just imported.
		s.recordImportedAlbum(ctx, targetPath, album, cfgp, listid, listEntryID)

		matchedCount++
	}

	logger.Logtype("info", 1).
		Str("folder", folder).
		Int("matched", matchedCount).
		Int("unmatched", unmatchedCount).
		Int("total", len(files)).
		Msg("Multi-episode audiobook folder processing complete")

	// Clean up the source folder only if all files were processed: the size
	// based cleanup would otherwise delete the unmatched episodes with it.
	if matchedCount > 0 && unmatchedCount == 0 {
		s.cleanUpFolder(folder)
	}

	if matchedCount == 0 {
		return "no_match", errUnprocessed
	}

	return "", nil
}

// writeRenameLog writes a small text file into the target folder documenting
// the original filenames and what they were renamed to during organization.
func (*Organizer) writeRenameLog(
	ctx context.Context,
	targetPath, sourceFolder string,
	album *parser_v2.AlbumInfo,
	o *Organizerdata,
	cfgp *config.MediaTypeConfig,
) {
	logPath := filepath.Join(targetPath, "_rename_log.txt")

	var b strings.Builder
	b.WriteString("Organized: ")
	b.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	b.WriteByte('\n')

	b.WriteString("Album: ")
	b.WriteString(album.Artist)
	b.WriteString(" - ")
	b.WriteString(album.Title)

	if album.Year > 0 {
		b.WriteString(" (")
		b.WriteString(strconv.Itoa(album.Year))
		b.WriteByte(')')
	}

	b.WriteByte('\n')

	b.WriteString("Source: ")
	b.WriteString(sourceFolder)
	b.WriteByte('\n')
	b.WriteString("Target: ")
	b.WriteString(targetPath)
	b.WriteByte('\n')

	if cfgp.IsType == config.MediaTypeMusic && album.DatabaseID > 0 {
		var mbid string
		database.Scanrowsdyn(
			false,
			"SELECT musicbrainz_release_id FROM dbalbums WHERE id = ?",
			&mbid,
			&album.DatabaseID,
		)

		if mbid != "" {
			b.WriteString("MusicBrainz: ")
			b.WriteString(mbid)
			b.WriteByte('\n')
		}
	} else if album.ASIN != "" {
		b.WriteString("ASIN: ")
		b.WriteString(album.ASIN)
		b.WriteByte('\n')
	}

	b.WriteString("Tracks: actual=")
	b.WriteString(strconv.Itoa(album.TrackCount))

	if album.ExpectedTracks > 0 {
		b.WriteString(" expected=")
		b.WriteString(strconv.Itoa(album.ExpectedTracks))
	}

	b.WriteString("\n\nRenamed files:\n")

	// Build a position→runtime map from Last.fm if any track is missing ExpectedRuntimeMS.
	// Last.fm positions run across the whole album, so they only line up with
	// per-disc track numbers on a single-disc album.
	var lfmRuntimeByPos map[int]int64
	if cfgp.IsType == config.MediaTypeMusic && album.Artist != "" && album.Title != "" {
		needsLFM := false
		for i := range album.Tracks {
			if album.Tracks[i].DiscNumber > 1 {
				needsLFM = false
				break
			}

			if album.Tracks[i].ExpectedRuntimeMS == 0 {
				needsLFM = true
			}
		}

		if needsLFM {
			if lfm := providers.GetLastFM(); lfm != nil {
				if rel, err := lfm.GetAlbumInfo(
					ctx,
					album.Artist,
					album.Title,
					"",
				); err == nil &&
					rel != nil {
					lfmRuntimeByPos = make(map[int]int64, len(rel.Tracks))
					for i := range rel.Tracks {
						t := &rel.Tracks[i]

						pos := t.Position
						if pos == 0 {
							pos = i + 1
						}

						lfmRuntimeByPos[pos] = t.Duration.Milliseconds()
					}
				}
			}
		}
	}

	for idx, mediaFile := range o.MediaFiles {
		oldName := filepath.Base(mediaFile)

		var newName string
		if idx < len(album.Tracks) && album.Tracks[idx].GeneratedFilename != "" {
			newName = album.Tracks[idx].GeneratedFilename
		} else {
			newName = oldName
		}

		b.WriteString("  ")
		b.WriteString(oldName)

		if oldName == newName {
			b.WriteString(" (unchanged)")
		} else {
			b.WriteString(" -> ")
			b.WriteString(newName)
		}

		if idx < len(album.Tracks) {
			track := &album.Tracks[idx]

			expectedMs := track.ExpectedRuntimeMS
			if expectedMs == 0 && lfmRuntimeByPos != nil {
				expectedMs = lfmRuntimeByPos[track.TrackNumber]
			}

			b.WriteString("  [disc=")
			b.WriteString(strconv.Itoa(track.DiscNumber))
			b.WriteString(" track=")
			b.WriteString(strconv.Itoa(track.TrackNumber))
			b.WriteString(" actual=")
			b.WriteString(strconv.FormatFloat(float64(track.RuntimeMS)/1000, 'f', 1, 64))
			b.WriteString("s")

			if expectedMs > 0 {
				b.WriteString(" expected=")
				b.WriteString(
					strconv.FormatFloat(float64(expectedMs)/1000, 'f', 1, 64),
				)
				b.WriteString("s diff=")

				diff := track.RuntimeMS - expectedMs
				if diff < 0 {
					diff = -diff
				}

				b.WriteString(strconv.FormatFloat(float64(diff)/1000, 'f', 1, 64))
				b.WriteString("s")
			}

			b.WriteByte(']')
		}

		b.WriteByte('\n')
	}

	appendRenameLog(logPath, b.String())
}

// appendRenameLog appends one entry to the folder's rename log. Appending
// keeps earlier entries when several imports share a folder (season folders,
// multi-episode audiobooks) instead of each overwriting the last.
func appendRenameLog(logPath, entry string) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err == nil {
		if fi, serr := f.Stat(); serr == nil && fi.Size() > 0 {
			entry = "\n" + entry
		}

		_, err = f.WriteString(entry)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
	}

	if err != nil {
		logger.Logtype("error", 1).
			Str("path", logPath).
			Err(err).
			Msg("Failed to write rename log")
	}
}

// writeRenameLogSingle writes a rename log for single-file media (movies, series).
// It uses the collected RenamedFiles entries from Organizerdata to document
// all old->new filename mappings including additional files (subtitles, NFOs, etc.).
func (*Organizer) writeRenameLogSingle(
	targetPath, sourceFolder string,
	o *Organizerdata,
	m *database.ParseInfo,
) {
	logPath := filepath.Join(targetPath, "_rename_log.txt")

	var b strings.Builder
	b.WriteString("Organized: ")
	b.WriteString(time.Now().Format("2006-01-02 15:04:05"))
	b.WriteByte('\n')

	b.WriteString("Title: ")
	b.WriteString(m.Title)

	if m.Year > 0 {
		b.WriteString(" (")
		b.WriteString(strconv.Itoa(int(m.Year)))
		b.WriteByte(')')
	}

	if m.SeasonStr != "" || m.EpisodeStr != "" {
		b.WriteString(" - S")
		b.WriteString(m.SeasonStr)
		b.WriteByte('E')
		b.WriteString(m.EpisodeStr)
	}

	b.WriteByte('\n')

	b.WriteString("Source: ")
	b.WriteString(sourceFolder)
	b.WriteByte('\n')
	b.WriteString("Target: ")
	b.WriteString(targetPath)
	b.WriteString("\n\nRenamed files:\n")

	for idx := range o.RenamedFiles {
		b.WriteString("  ")
		b.WriteString(o.RenamedFiles[idx].OldName)

		if o.RenamedFiles[idx].OldName == o.RenamedFiles[idx].NewName {
			b.WriteString(" (unchanged)")
		} else {
			b.WriteString(" -> ")
			b.WriteString(o.RenamedFiles[idx].NewName)
		}

		b.WriteByte('\n')
	}

	appendRenameLog(logPath, b.String())
}

// unlistedEntry adds a matched album or audiobook that is in no list - for
// music typically a synthetic album the fallback providers built from a
// non-MusicBrainz source - to the add_found list, so the moved files can be
// recorded for it. It returns that list and the new entry; 0 entry when
// add_found is off.
func unlistedEntry(
	cfgp *config.MediaTypeConfig,
	dataCfg *config.MediaDataConfig,
	dbid *uint,
) (listid int, listEntryID uint) {
	listid = -1
	if dataCfg != nil && dataCfg.AddFound {
		listid = cfgp.GetMediaListsEntryListID(dataCfg.AddFoundList)
	}

	table := "albums"

	switch cfgp.IsType {
	case config.MediaTypeMusic:
		listEntryID = importfeed.EnsureAlbumListEntry(dbid, cfgp, listid)
	case config.MediaTypeAudiobook:
		table = "audiobooks"
		listEntryID = importfeed.EnsureAudiobookListEntry(dbid, cfgp, listid)
	default:
		return -1, 0
	}

	if listEntryID > 0 && listid == -1 {
		listid = cfgp.GetMediaListsEntryListID(database.Getdatarow[string](
			false, "SELECT listname FROM "+table+" WHERE id = ?", &listEntryID))
	}

	return listid, listEntryID
}

// albumListEntryID resolves the albums.id / audiobooks.id list entry of dbid
// in the list the import targets (0 when there is none).
func albumListEntryID(cfgp *config.MediaTypeConfig, listid int, dbid *uint) uint {
	if listid < 0 || listid >= len(cfgp.Lists) {
		return 0
	}

	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		return database.GetAudiobookListEntryID(dbid, &cfgp.Lists[listid].Name)
	case config.MediaTypeMusic:
		return database.GetAlbumListEntryID(dbid, &cfgp.Lists[listid].Name)
	}

	return 0
}

// oldAlbumFiles returns the files the database already holds for the album /
// audiobook (dbalbum_id / dbaudiobook_id, across all lists), with the columns
// the audio priority scorer needs.
func oldAlbumFiles(mediaType, dbid uint) []database.AudioFilePrio {
	if dbid == 0 {
		return nil
	}

	switch mediaType {
	case config.MediaTypeMusic:
		return database.Getrowssize[database.AudioFilePrio](
			false,
			"SELECT count() FROM album_files WHERE dbalbum_id = ?",
			"SELECT location, album_id, id, format, bitrate, sample_rate, bit_depth FROM album_files WHERE dbalbum_id = ?",
			dbid,
		)
	case config.MediaTypeAudiobook:
		return database.Getrowssize[database.AudioFilePrio](
			false,
			"SELECT count() FROM audiobook_files WHERE dbaudiobook_id = ?",
			"SELECT location, audiobook_id, id, format, bitrate, 0, 0 FROM audiobook_files WHERE dbaudiobook_id = ?",
			dbid,
		)
	}

	return nil
}

// albumUpgradeAllowed reports whether the incoming album may replace the files
// already in the library, mirroring the movie/series errLowerQuality rule: the
// new files must score higher than the existing ones. Equal quality is still
// accepted when it brings more tracks (completing an incomplete album). Old
// rows that are the very files being imported (in-place reorganize) or whose
// files are gone are not "existing" files. Without a quality config there is nothing to compare, so
// the import proceeds as before.
func albumUpgradeAllowed(
	cfgp *config.MediaTypeConfig,
	listid int,
	album *parser_v2.AlbumInfo,
	old []database.AudioFilePrio,
) bool {
	sources := make(map[string]struct{}, len(album.Tracks))
	for idx := range album.Tracks {
		sources[album.Tracks[idx].Filepath] = struct{}{}
	}

	// Stale rows of files no longer on disk must not block a re-download.
	existing := make([]database.AudioFilePrio, 0, len(old))
	for idx := range old {
		if _, ok := sources[old[idx].Location]; ok {
			continue
		}

		if _, err := os.Lstat(old[idx].Location); errors.Is(err, fs.ErrNotExist) {
			continue
		}

		existing = append(existing, old[idx])
	}

	if len(existing) == 0 || listid < 0 || listid >= len(cfgp.Lists) ||
		cfgp.Lists[listid].CfgQuality == nil {
		return true
	}

	qual := cfgp.Lists[listid].CfgQuality

	incoming := make([]database.AudioFilePrio, len(album.Tracks))
	for idx := range album.Tracks {
		incoming[idx] = database.AudioFilePrio{
			Location: album.Tracks[idx].Filepath,
			Format:   album.Tracks[idx].Format,
			Bitrate:  album.Tracks[idx].Bitrate,
		}
		// audiobook_files has no sample rate / bit depth, so score the new
		// files without them as well.
		if cfgp.IsType == config.MediaTypeMusic {
			incoming[idx].SampleRate = album.Tracks[idx].SampleRate
			incoming[idx].BitDepth = album.Tracks[idx].BitDepth
		}
	}

	oldPrio := searcher.AudioFilesPriority(existing, qual)
	newPrio := searcher.AudioFilesPriority(incoming, qual)

	logger.Logtype("debug", 1).
		Int("old_priority", oldPrio).
		Int("new_priority", newPrio).
		Int("old_files", len(existing)).
		Int("new_files", len(incoming)).
		Msg("album upgrade check")

	if oldPrio == 0 || newPrio > oldPrio {
		return true
	}

	return newPrio == oldPrio && len(incoming) > len(existing)
}

// audioFilesAtTarget lists the audio files currently under o.TargetPath (never
// the library root itself).
func (s *Organizer) audioFilesAtTarget(o *Organizerdata) []string {
	if o.TargetPath == "" || o.TargetPath == s.targetpathCfg.Path ||
		!scanner.CheckFileExist(o.TargetPath) {
		return nil
	}

	var files []string

	err := filepath.WalkDir(o.TargetPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() && parser_v2.HasExtension(path, parser_v2.AudioExtensions) {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		logger.Logtype("error", 1).
			Str("path", o.TargetPath).
			Err(err).
			Msg("Failed to walk target directory")
	}

	return files
}

// moveAlbumIntoPlace moves the album's files to o.TargetPath (already set)
// and only afterwards retires the files they replace: the album's old DB
// files and, when sweepTarget is set and the import is not in place, other
// audio files already in the target folder. Retiring honours MoveReplaced
// (move to the replaced folder) and Replacelower (delete); with neither set
// old files stay on disk. Files that are part of this import (source or new
// path) are never removed - only their stale DB rows are dropped. An old file
// at the exact path a new file takes is stashed in the replaced folder first
// when MoveReplaced is set, and restored if the move fails.
func (s *Organizer) moveAlbumIntoPlace(
	ctx context.Context,
	folder string,
	o *Organizerdata,
	old []database.AudioFilePrio,
	sweepTarget bool,
) (string, error) {
	candidates := make([]string, 0, len(old))
	for idx := range old {
		candidates = append(candidates, old[idx].Location)
	}

	if sweepTarget && filepath.Clean(folder) != filepath.Clean(o.TargetPath) {
		candidates = append(candidates, s.audioFilesAtTarget(o)...)
	}

	keep := make(map[string]struct{}, 2*len(o.MediaFiles))
	opts := scanner.MoveFileOptions{MediaType: s.Cfgp.IsType}
	useGenerated := len(o.Filenames) == len(o.MediaFiles)

	for idx, mediaFile := range o.MediaFiles {
		name := filepath.Base(mediaFile)
		if useGenerated && o.Filenames[idx] != "" {
			name = o.Filenames[idx]
		}

		keep[mediaFile] = struct{}{}
		keep[scanner.PlannedMovePath(mediaFile, s.sourcepathCfg, o.TargetPath, name, opts)] = struct{}{}
	}

	moveReplaced := s.targetpathCfg.MoveReplaced && s.targetpathCfg.MoveReplacedTargetPath != ""

	type stash struct{ from, to string }

	var stashed []stash

	restore := func() {
		for idx := range stashed {
			s.restoreStashedFile(stashed[idx].to, stashed[idx].from)
		}
	}

	if moveReplaced {
		sources := make(map[string]struct{}, len(o.MediaFiles))
		for _, mediaFile := range o.MediaFiles {
			sources[mediaFile] = struct{}{}
		}

		seen := make(map[string]struct{}, len(candidates))
		for _, c := range candidates {
			if _, ok := seen[c]; ok {
				continue
			}

			seen[c] = struct{}{}

			_, isTarget := keep[c]
			_, isSource := sources[c]

			if !isTarget || isSource || !scanner.CheckFileExist(c) {
				continue
			}

			to, err := s.stashReplacedFile(c)
			if err != nil {
				restore()
				return "", fmt.Errorf("move replaced file out of the way: %w", err)
			}

			stashed = append(stashed, stash{from: c, to: to})
		}
	}

	newpath, err := s.moveMediaFile(o)
	if err != nil {
		restore()
		return "", err
	}

	retired := make([]string, 0, len(candidates))

	seen := make(map[string]struct{}, len(candidates))
	for _, c := range candidates {
		if ctx.Err() != nil {
			break
		}

		if _, ok := seen[c]; ok {
			continue
		}

		seen[c] = struct{}{}

		if _, ok := keep[c]; ok {
			// The path holds a file of this import now (or held its source):
			// only the old row pointing at it goes.
			s.dropOldFileRecord(c, &c, nil)
			continue
		}

		var rerr error

		switch {
		case moveReplaced:
			rerr = s.moveRemoveOldMediaFile(c, &c, nil, true)
		case s.targetpathCfg.Replacelower:
			rerr = s.moveRemoveOldMediaFile(c, &c, nil, false)
		default:
			// Old files are kept; only rows of files already gone are dropped.
			if _, lerr := os.Lstat(c); errors.Is(lerr, fs.ErrNotExist) {
				s.dropOldFileRecord(c, &c, nil)
			}
		}

		if rerr != nil {
			logger.Logtype("error", 1).
				Str("file", c).
				Err(rerr).
				Msg("Failed to retire replaced album file")

			continue
		}

		retired = append(retired, c)
	}

	// The whole old album goes: leftovers and emptied folders as well.
	if (moveReplaced || s.targetpathCfg.Replacelower) && ctx.Err() == nil {
		s.retireOldAlbumFolders(o, retired, moveReplaced)
	}

	return newpath, nil
}

// updateAlbumTrackPaths points album.Tracks at the files moveMediaFile wrote
// and returns the target folder. The names MoveFile actually used win over the
// generated ones: it keeps the original name for no-rename extensions, and
// recording the planned name would leave the DB and rename log pointing at a
// file that does not exist.
func (*Organizer) updateAlbumTrackPaths(o *Organizerdata, album *parser_v2.AlbumInfo) string {
	targetPath := o.TargetPath

	useMovedNames := len(o.RenamedFiles) == len(album.Tracks)
	useGeneratedFilenames := len(o.Filenames) == len(album.Tracks)
	for idx := range album.Tracks {
		var trackFilename string
		switch {
		case useMovedNames && o.RenamedFiles[idx].NewName != "":
			trackFilename = o.RenamedFiles[idx].NewName
		case useGeneratedFilenames && o.Filenames[idx] != "":
			trackFilename = o.Filenames[idx]
		default:
			trackFilename = filepath.Base(album.Tracks[idx].Filepath)
		}

		album.Tracks[idx].Filepath = filepath.Join(targetPath, trackFilename)
		album.Tracks[idx].Filename = trackFilename
		album.Tracks[idx].GeneratedFilename = trackFilename
	}

	album.SourceFolder = targetPath

	return targetPath
}

// recordImportedAlbum inserts the moved files for the list entry and updates
// its missing / quality_reached flags, scoring the files just added (m is a
// naming carrier, its Priority is always 0).
func (*Organizer) recordImportedAlbum(
	ctx context.Context,
	targetPath string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	listid int,
	listEntryID uint,
) {
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		addAudiobookFilesToDatabase(ctx, targetPath, album, cfgp, listEntryID)
	case config.MediaTypeMusic:
		addAlbumFilesToDatabase(ctx, targetPath, album, cfgp, listEntryID)
	}

	if listEntryID == 0 {
		return
	}

	var reached int

	if listid >= 0 && listid < len(cfgp.Lists) {
		if qual := cfgp.Lists[listid].CfgQuality; qual != nil {
			prio, _ := searcher.GetpriobyfilesAudio(cfgp.IsType, &listEntryID, false, -1, qual, false)
			if prio >= qual.CutoffPriority {
				reached = 1
			}
		}
	}

	database.ExecN(
		mtstrings.GetStringsMap(cfgp.IsType, "UpdateMissingReached"),
		&reached,
		&listEntryID,
	)
}

// generateTrackFilenames generates filenames for each track using the naming template.
func (s *Organizer) generateTrackFilenames(
	_ *Organizerdata,
	m *database.ParseInfo,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
) []string {
	if len(album.Tracks) == 0 {
		return nil
	}

	// Get the filename template from the naming config
	_, filenameTemplate := logger.SplitByLR(s.Cfgp.Naming, checksplit(s.Cfgp.Naming))
	if filenameTemplate == "" {
		// Fallback to original filenames
		filenames := make([]string, len(album.Tracks))
		for idx, track := range album.Tracks {
			filenames[idx] = filepath.Base(track.Filepath)
		}

		return filenames
	}

	handler := mediatype.Get(cfgp.IsType)
	if handler == nil {
		return nil
	}

	// Fetch track/chapter titles from authoritative sources
	var (
		dbTrackTitles []string
		dbtracks      []database.Dbtrack
	)

	// dbtrackByNum: (disc_number*10000 + track_number) → Dbtrack, populated for
	// music so title/track lookup is by track number rather than by position.
	// Positional indexing breaks when unmatched DB tracks are skipped — every
	// subsequent idx maps to the wrong dbtracks entry (one off per skipped track).
	var dbtrackByNum map[int64]database.Dbtrack

	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		// Fetch chapter titles from Audnex
		audnexProvider := providers.GetAudnex()
		if audnexProvider == nil {
			break
		}

		var dbaudiobook database.Dbaudiobook
		if dbaudiobook.GetDbaudiobookByIDP(&album.DatabaseID) != nil || dbaudiobook.ASIN == "" {
			break
		}

		chaptersT, errT := audnexProvider.GetChaptersByASIN(
			context.Background(), dbaudiobook.ASIN, cfgp.AudibleRegion,
		)
		if errT != nil {
			break
		}

		dbTrackTitles = make([]string, len(chaptersT))
		for i, ch := range chaptersT {
			dbTrackTitles[i] = ch.Title
		}

	case config.MediaTypeMusic:
		// Fetch track titles from dbtracks (MusicBrainz/Discogs data)
		dbtracks = database.Getrowssize[database.Dbtrack](
			false,
			"SELECT count() FROM dbtracks WHERE dbalbum_id = ?",
			"SELECT id, title, track_number, disc_number, dbalbum_id, runtime_ms FROM dbtracks WHERE dbalbum_id = ? ORDER BY disc_number, track_number",
			&album.DatabaseID,
		)
		if len(dbtracks) == 0 {
			break
		}

		dbtrackByNum = make(map[int64]database.Dbtrack, len(dbtracks))
		for i := range dbtracks {
			disc := int64(dbtracks[i].DiscNumber)
			if disc == 0 {
				disc = 1
			}

			dbtrackByNum[disc*10000+int64(dbtracks[i].TrackNumber)] = dbtracks[i]
		}
	}

	// Compute total disc count once — it is the same for every track in the album.
	// Priority: album.DiscCount → count distinct non-zero DiscNumber values across tracks.
	totalDiscs := album.DiscCount
	if totalDiscs == 0 {
		seen := make(map[int]struct{}, 4)
		for i := range album.Tracks {
			if album.Tracks[i].DiscNumber > 0 {
				seen[album.Tracks[i].DiscNumber] = struct{}{}
			}
		}

		totalDiscs = len(seen)
	}

	// Hoist per-track temporaries outside the loop so they escape to heap once
	// (at function entry) rather than once per track.
	var (
		trackM     database.ParseInfo
		forparser  parsertype
		namingData mediatype.NamingData
	)

	filenames := make([]string, len(album.Tracks))
	for idx, track := range album.Tracks {
		trackM = *m
		trackM.Episode = track.TrackNumber

		forparser = parsertype{Source: &trackM}
		namingData = mediatype.NamingData{}

		_, ok := handler.FillNamingData(&album.DatabaseID, track.Filepath, &trackM, &namingData)
		if !ok {
			// Fallback to original filename
			filenames[idx] = filepath.Base(track.Filepath)
			continue
		}

		// Copy naming data to forparser
		forparser.Dbalbum = namingData.Dbalbum
		forparser.Dbtrack = namingData.Dbtrack
		forparser.Artist = namingData.Artist
		forparser.AlbumArtist = namingData.AlbumArtist

		forparser.Title = track.Title
		if cfgp.IsType == config.MediaTypeAudiobook {
			forparser.Track = idx + 1
		} else {
			forparser.Track = track.TrackNumber
		}

		forparser.Disc = track.DiscNumber
		// Prefer the DB-queried disc count (stored in trackM.Season by FillNamingData);
		// fall back to the pre-computed totalDiscs (album.DiscCount or distinct-disc count).
		if trackM.Season > 0 {
			forparser.TotalDiscs = trackM.Season
		} else {
			forparser.TotalDiscs = totalDiscs
		}

		forparser.Dbaudiobook = namingData.Dbaudiobook
		forparser.DbaudiobookChapter = namingData.DbaudiobookChapter
		forparser.Author = namingData.Author

		// Use database track data / title.
		// Music: look up by (disc, track) number — album.Tracks is the matched
		// subset and may have gaps where DB tracks were unmatched, so positional
		// indexing would assign the wrong title to every track after a gap.
		// Audiobook: use positional chapter titles (sequential from Audnex).
		if dbtrackByNum != nil {
			disc := int64(track.DiscNumber)
			if disc == 0 {
				disc = 1
			}

			key := disc*10000 + int64(track.TrackNumber)
			if dt, ok := dbtrackByNum[key]; ok {
				forparser.Dbtrack = dt
				if dt.Title != "" {
					forparser.Title = dt.Title
				}
			}
		} else if idx < len(dbTrackTitles) && dbTrackTitles[idx] != "" {
			forparser.Title = dbTrackTitles[idx]
		}

		// Parse the filename template
		_, filename, err := logger.ParseStringTemplate(filenameTemplate, &forparser)
		if err != nil || filename == "" {
			// Fallback to original filename
			filenames[idx] = filepath.Base(track.Filepath)
			album.Tracks[idx].GeneratedFilename = filenames[idx]
			continue
		}

		// Add extension from original file
		ext := filepath.Ext(track.Filepath)

		logger.Path(&filename, false)

		filenames[idx] = filename + ext
		album.Tracks[idx].GeneratedFilename = filenames[idx]
	}

	return filenames
}

// addAudiobookFilesToDatabase is a local wrapper that calls importfeed's add function.
func addAudiobookFilesToDatabase(
	ctx context.Context,
	folder string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	audiobookListID uint,
) {
	if album.DatabaseID == 0 {
		return
	}

	// audiobookListID is the audiobooks.id list entry (albumListEntryID)
	if audiobookListID == 0 {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Uint("dbaudiobookID", album.DatabaseID).
			Msg("Could not find audiobook list entry for dbaudiobook_id")

		return
	}

	// Update rootpath
	if err := database.UpdateAudiobookRootpath(audiobookListID, folder); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to update audiobook rootpath")
	}

	// Add each file to the database
	filesAdded := 0
	for i := range album.Tracks {
		if err := ctx.Err(); err != nil {
			return
		}

		// Skip files already in database
		if database.AudiobookFileExists(&album.Tracks[i].Filepath) {
			continue
		}

		// Insert file record (audiobookListID = audiobooks.id, album.DatabaseID = dbaudiobook_id)
		if err := database.InsertAudiobookFile(
			audiobookListID,
			album.Tracks[i].Filepath,
			album.Tracks[i].Filename,
			album.Tracks[i].Extension,
			album.Tracks[i].Format,
			album.Tracks[i].QualityProfile,
			album.Tracks[i].FileSize,
			album.Tracks[i].Bitrate,
			album.Tracks[i].RuntimeMS,
			album.Tracks[i].TrackNumber,
			album.Tracks[i].DiscNumber,
			album.DatabaseID,
		); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to insert audiobook file")

			continue
		}

		filesAdded++

		// Tag file if added (add to cache)
		if config.GetSettingsGeneral().UseFileCache {
			database.AppendCacheMap(cfgp.IsType, logger.CacheFiles, album.Tracks[i].Filepath)
		}
	}

	if filesAdded > 0 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("title", album.Title).
			Uint("audiobook_id", album.DatabaseID).
			Int("files_added", filesAdded).
			Msg("Added audiobook files to database")
	}
}

// addAlbumFilesToDatabase is a local wrapper that calls importfeed's add function.
func addAlbumFilesToDatabase(
	ctx context.Context,
	folder string,
	album *parser_v2.AlbumInfo,
	cfgp *config.MediaTypeConfig,
	albumListID uint,
) {
	// This would ideally call importfeed.AddAlbumFilesToDatabase
	// but since that function is not exported, we duplicate the logic here
	if album.DatabaseID == 0 {
		return
	}

	// albumListID is the albums.id list entry (albumListEntryID); album.DatabaseID
	// is dbalbum_id, NOT albums.id
	if albumListID == 0 {
		logger.Logtype("warn", 1).
			Str("folder", folder).
			Uint("dbalbumID", album.DatabaseID).
			Msg("Could not find album list entry for dbalbum_id")

		return
	}

	// Update rootpath using the correct albums.id
	if err := database.UpdateAlbumRootpath(albumListID, folder); err != nil {
		logger.Logtype("error", 1).
			Str("folder", folder).
			Err(err).
			Msg("Failed to update album rootpath")
	}

	// Add each file to the database
	filesAdded := 0
	for i := range album.Tracks {
		if err := ctx.Err(); err != nil {
			return
		}

		// Skip files already in database
		if database.AlbumFileExists(&album.Tracks[i].Filepath) {
			continue
		}

		// Insert file record — albumListID is albums.id, album.DatabaseID is dbalbum_id
		trackFile := &database.TrackFileInfo{
			Filepath:       album.Tracks[i].Filepath,
			Filename:       album.Tracks[i].Filename,
			Extension:      album.Tracks[i].Extension,
			Format:         album.Tracks[i].Format,
			QualityProfile: album.Tracks[i].QualityProfile,
			FileSize:       album.Tracks[i].FileSize,
			Bitrate:        album.Tracks[i].Bitrate,
			SampleRate:     album.Tracks[i].SampleRate,
			BitDepth:       album.Tracks[i].BitDepth,
			RuntimeMS:      album.Tracks[i].RuntimeMS,
			TrackNumber:    album.Tracks[i].TrackNumber,
			DiscNumber:     album.Tracks[i].DiscNumber,
		}
		if err := database.InsertAlbumFile(
			albumListID,
			trackFile,
			album.Tracks[i].AcoustID,
			album.DatabaseID,
			0,
		); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to insert album file")

			continue
		}

		filesAdded++

		// Tag file if added (add to cache)
		if config.GetSettingsGeneral().UseFileCache {
			database.AppendCacheMap(cfgp.IsType, logger.CacheFiles, album.Tracks[i].Filepath)
		}
	}

	if filesAdded > 0 {
		logger.Logtype("info", 1).
			Str("folder", folder).
			Str("title", album.Title).
			Uint("album_id", album.DatabaseID).
			Int("files_added", filesAdded).
			Msg("Added album files to database")
	}
}

// fetchCoverArt downloads cover art from a URL and returns the image data and MIME type.
func fetchCoverArt(coverURL string) ([]byte, string) {
	if coverURL == "" {
		return nil, ""
	}

	req, err := http.NewRequestWithContext(
		context.Background(),
		http.MethodGet,
		coverURL,
		nil,
	)
	if err != nil {
		logger.Logtype("warn", 1).
			Str("url", coverURL).
			Err(err).
			Msg("Failed to create cover art request")

		return nil, ""
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		logger.Logtype("warn", 1).Str("url", coverURL).Err(err).Msg("Failed to fetch cover art")
		return nil, ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ""
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20)) // 10MB max
	if err != nil {
		return nil, ""
	}

	mime := resp.Header.Get("Content-Type")
	if mime == "" {
		mime = http.DetectContentType(data)
	}

	return data, mime
}

// tagAlbumFiles writes proper metadata tags to all tracks in an album.
// It uses the tags package to write tags based on the album and track information.
func (s *Organizer) tagAlbumFiles(ctx context.Context, album *parser_v2.AlbumInfo) error {
	embedArt := false

	embedLyrics := false
	for idx := range s.Cfgp.Data {
		if s.Cfgp.Data[idx].EmbedArt {
			embedArt = true
		}

		if s.Cfgp.Data[idx].EmbedLyrics {
			embedLyrics = true
		}
	}

	return TagAlbumFiles(ctx, s.Cfgp.IsType, embedArt, embedLyrics, album)
}

// TagAlbumFiles writes metadata tags to all audio files in an album.
// It looks up artist/author, album metadata, and cover art from the database,
// then writes tags to each track file. This is exported so it can be called
// from API endpoints for re-tagging existing files.
func TagAlbumFiles(
	ctx context.Context,
	mediaType uint,
	embedArt, embedLyrics bool,
	album *parser_v2.AlbumInfo,
) error {
	// Determine album artist from DB first (more reliable than file tags),
	// then fall back to file tags.
	var (
		albumArtist                                                 string
		mbReleaseID, mbReleaseGroupID, mbArtistID, dbLabel, dbGenre string
	)

	if album.DatabaseID > 0 {
		switch mediaType {
		case config.MediaTypeAudiobook:
			// For audiobooks, query dbaudiobook_authors/dbauthors
			database.Scanrowsdyn(false,
				"SELECT au.name FROM dbauthors au "+
					"JOIN dbaudiobook_authors aa ON au.id = aa.dbauthor_id "+
					"WHERE aa.dbaudiobook_id = ? LIMIT 1",
				&albumArtist, &album.DatabaseID)

		case config.MediaTypeMusic:
			// For music, query dbalbum_artists/dbartists
			var artistCount int
			database.Scanrowsdyn(false,
				"SELECT count() FROM dbalbum_artists WHERE dbalbum_id = ?",
				&artistCount, &album.DatabaseID)

			if artistCount > 3 {
				albumArtist = "Various Artists"
			} else {
				database.Scanrowsdyn(false,
					"SELECT ar.name FROM dbartists ar "+
						"JOIN dbalbum_artists aa ON ar.id = aa.dbartist_id "+
						"WHERE aa.dbalbum_id = ? ORDER BY aa.position LIMIT 1",
					&albumArtist, &album.DatabaseID)
			}

			// Look up album-level metadata
			var dbAlbum database.Dbalbum
			if err := dbAlbum.GetDbalbumByIDP(&album.DatabaseID); err == nil {
				mbReleaseID = dbAlbum.MusicbrainzReleaseID
				mbReleaseGroupID = dbAlbum.MusicbrainzReleaseGroupID
				dbLabel = dbAlbum.Label
				dbGenre = dbAlbum.Genres
			}

			// Get primary artist MusicBrainz ID via album-artist relationship
			database.Scanrowsdyn(false,
				"SELECT ar.musicbrainz_id FROM dbartists ar "+
					"JOIN dbalbum_artists aa ON ar.id = aa.dbartist_id "+
					"WHERE aa.dbalbum_id = ? ORDER BY aa.position LIMIT 1",
				&mbArtistID, &album.DatabaseID)
		}
	}

	// Fall back to file tags if DB lookup didn't produce a result
	if albumArtist == "" {
		albumArtist = album.AlbumArtist
	}

	if albumArtist == "" {
		albumArtist = album.Artist
	}

	// Resolve genre: prefer album info from file tags, fall back to database
	genre := album.Genre
	if genre == "" {
		genre = dbGenre
	}

	// Resolve label: prefer database (more reliable), fall back to AlbumInfo
	label := dbLabel
	if label == "" {
		label = album.Label
	}

	// Fetch and embed cover art if enabled
	var (
		coverData []byte
		coverMIME string
	)

	if embedArt && album.DatabaseID > 0 {
		var coverURL string
		switch mediaType {
		case config.MediaTypeAudiobook:
			database.Scanrowsdyn(false,
				"SELECT cover_url FROM dbaudiobooks WHERE id = ?",
				&coverURL, &album.DatabaseID)

		case config.MediaTypeMusic:
			database.Scanrowsdyn(false,
				"SELECT cover_url FROM dbalbums WHERE id = ?",
				&coverURL, &album.DatabaseID)
		}

		// Fall back to Cover Art Archive for MusicBrainz releases without a stored cover URL
		if coverURL == "" && mbReleaseID != "" {
			coverURL = "https://coverartarchive.org/release/" + url.PathEscape(mbReleaseID) + "/front"
		}

		if coverURL != "" {
			coverData, coverMIME = fetchCoverArt(coverURL)
			if len(coverData) > 0 {
				logger.Logtype("debug", 0).
					Str("url", coverURL).
					Int("size", len(coverData)).
					Str("mime", coverMIME).
					Msg("Fetched cover art for embedding")
			}
		}
	}

	for i := range album.Tracks {
		// Create AudioTags struct from track info
		audioTags := &tags.AudioTags{
			Title:            album.Tracks[i].Title,
			Artist:           album.Tracks[i].Artist,
			Album:            album.Title,
			AlbumArtist:      albumArtist,
			Genre:            genre,
			Year:             album.Year,
			TrackNumber:      album.Tracks[i].TrackNumber,
			TotalTracks:      album.TrackCount,
			DiscNumber:       album.Tracks[i].DiscNumber,
			TotalDiscs:       album.DiscCount,
			Label:            label,
			CatalogNum:       album.CatalogNumber,
			MBReleaseID:      mbReleaseID,
			MBReleaseGroupID: mbReleaseGroupID,
			MBArtistID:       mbArtistID,
			MBAlbumArtistID:  mbArtistID,
		}

		// Add track-level MusicBrainz IDs if available
		if album.Tracks[i].MusicBrainzID != "" {
			audioTags.MBRecordingID = album.Tracks[i].MusicBrainzID
		}

		if album.Tracks[i].AcoustID != "" {
			audioTags.AcoustID = album.Tracks[i].AcoustID
		}

		if album.Tracks[i].ISRC != "" {
			audioTags.ISRC = album.Tracks[i].ISRC
		}

		// Add audiobook-specific fields
		if album.Tracks[i].Narrator != "" {
			audioTags.Composer = album.Tracks[i].Narrator // Store narrator in composer field
		}

		if album.Tracks[i].ASIN != "" {
			audioTags.CatalogNum = album.Tracks[i].ASIN // Store ASIN in catalog number field
		}

		// Embed cover art if available
		if len(coverData) > 0 {
			audioTags.CoverData = coverData
			audioTags.CoverMIME = coverMIME
		} else {
			// No new album-wide cover was fetched - carry forward whatever
			// cover is already embedded in this specific file. Without this,
			// WriteTags below would silently strip any existing embedded
			// cover art (confirmed for MP3 via the id3v2 library's ParseFrames
			// behavior, which discards unlisted frames like APIC entirely
			// rather than round-tripping them - see tags.ReadCoverData).
			if existingCover, existingMIME := tags.ReadCoverData(album.Tracks[i].Filepath); len(existingCover) > 0 {
				audioTags.CoverData = existingCover
				audioTags.CoverMIME = existingMIME
			}
		}

		// Fetch and embed lyrics when enabled.
		if embedLyrics && audioTags.Title != "" {
			lyrCtx, lyrCancel := context.WithTimeout(context.Background(), 15*time.Second)
			lyr := lyrics.Fetch(lyrCtx, audioTags.Artist, audioTags.Title, audioTags.Album)

			lyrCancel()

			if lyr != "" {
				audioTags.Lyrics = lyr
				logger.Logtype("debug", 0).
					Str("file", album.Tracks[i].Filepath).
					Str("title", audioTags.Title).
					Msg("Fetched lyrics for embedding")
			}
		}

		// Write tags to file
		if err := tags.WriteTags(ctx, album.Tracks[i].Filepath, audioTags); err != nil {
			logger.Logtype("error", 1).
				Str("file", album.Tracks[i].Filepath).
				Err(err).
				Msg("Failed to write tags to file")
			// Continue with other files even if one fails
			continue
		}

		logger.Logtype("debug", 0).
			Str("file", album.Tracks[i].Filepath).
			Str("title", album.Tracks[i].Title).
			Int("track", album.Tracks[i].TrackNumber).
			Msg("Successfully wrote tags to file")
	}

	return nil
}

// ---------------------------------------------------------------------------
// ForceMatchAlbumFolder — force-match a folder against a known MBID / ASIN
// ---------------------------------------------------------------------------

// AlbumForceMatchTrack holds the per-track rename plan returned in preview mode.
type AlbumForceMatchTrack struct {
	Source      string `json:"source"` // current file path (basename)
	Target      string `json:"target"` // target filename after rename
	TrackNumber int    `json:"track_number"`
	DiscNumber  int    `json:"disc_number"`
	Title       string `json:"title"`
	DBRuntimeMS int64  `json:"db_runtime_ms"`
}

// AlbumForceMatchPreview is the response body for preview mode.
type AlbumForceMatchPreview struct {
	FolderTarget string                 `json:"folder_target"` // computed target folder path
	Title        string                 `json:"title"`
	Artist       string                 `json:"artist"`
	Year         int                    `json:"year"`
	Tracks       []AlbumForceMatchTrack `json:"tracks"`
}

// ForceMatchAlbumFolder matches folder against forcedID (MBID for music, ASIN for
// audiobooks) and either previews the rename plan or executes the full organize
// workflow.
//
// When preview is true the function returns a filled *AlbumForceMatchPreview and a
// nil error without touching any files.  When preview is false it runs the full
// organize pipeline (tag → move → DB insert) and returns nil, nil on success.
func (s *Organizer) ForceMatchAlbumFolder(
	ctx context.Context,
	folder string,
	cfgp *config.MediaTypeConfig,
	data *config.MediaDataImportConfig,
	forcedID *string,
	preview bool,
) (*AlbumForceMatchPreview, error) {
	// Build the MediaDataConfig the same way organizeAlbumFolderViaAPI does so that
	// force-match uses identical weights, tolerances, and release-type filters.
	dataCfg := buildAlbumDataConfig(data, cfgp)

	// Match — always skip adding to DB at this stage so preview can inspect first.
	album, reason, _ := importfeed.MatchAudioFolderAsAlbumForced(
		ctx, folder, cfgp, dataCfg, false, forcedID,
	)
	if reason != "" || album == nil {
		return nil, fmt.Errorf("match failed: %s", reason)
	}

	// Build the ParseInfo / Organizerdata needed for naming.
	m := database.ParseInfo{}

	m.Title = album.Title

	m.Year = uint16(album.Year)

	var listname, rootpath string
	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		database.GetdatarowArgs(
			"SELECT listname, rootpath FROM audiobooks WHERE dbaudiobook_id = ? LIMIT 1",
			album.DatabaseID, &listname, &rootpath)

	case config.MediaTypeMusic:
		database.GetdatarowArgs(
			"SELECT listname, rootpath FROM albums WHERE dbalbum_id = ? LIMIT 1",
			album.DatabaseID, &listname, &rootpath)
	}

	listid := cfgp.GetMediaListsEntryListID(listname)
	listEntryID := albumListEntryID(cfgp, listid, &album.DatabaseID)

	if listEntryID == 0 && !preview {
		listid, listEntryID = unlistedEntry(cfgp, dataCfg, &album.DatabaseID)
	}

	switch cfgp.IsType {
	case config.MediaTypeAudiobook:
		m.DbaudiobookID = album.DatabaseID
		m.AudiobookID = listEntryID

	case config.MediaTypeMusic:
		m.DbalbumID = album.DatabaseID
		m.AlbumID = listEntryID
	}

	o := &Organizerdata{
		Folder:     folder,
		MediaFiles: make([]string, len(album.Tracks)),
		Listid:     listid,
		Rootpath:   rootpath,
	}
	for idx, track := range album.Tracks {
		o.MediaFiles[idx] = track.Filepath
	}

	if len(o.MediaFiles) > 0 {
		o.MediaFile = o.MediaFiles[0]
	}

	s.GenerateNamingTemplate(o, &m, &album.DatabaseID)

	// GenerateNamingTemplate clears Foldername on purpose when the album
	// already has a rootpath (that is the full album folder), so only an
	// empty Foldername without a rootpath is a naming failure.
	if o.Foldername == "" && (o.Rootpath == "" || o.Rootpath == s.targetpathCfg.Path) {
		return nil, errors.New("failed to generate folder name")
	}

	o.Filenames = s.generateTrackFilenames(o, &m, album, cfgp)

	// Compute the target folder exactly as the organize run will.
	s.applyPresort(o)
	s.setTargetPath(o)

	targetFolder := o.TargetPath

	if preview {
		result := &AlbumForceMatchPreview{
			FolderTarget: targetFolder,
			Title:        album.Title,
			Artist:       album.Artist,
			Year:         album.Year,
			Tracks:       make([]AlbumForceMatchTrack, 0, len(album.Tracks)),
		}
		for idx, track := range album.Tracks {
			targetName := ""
			if idx < len(o.Filenames) {
				targetName = o.Filenames[idx]
			}

			result.Tracks = append(result.Tracks, AlbumForceMatchTrack{
				Source:      filepath.Base(track.Filepath),
				Target:      targetName,
				TrackNumber: track.TrackNumber,
				DiscNumber:  track.DiscNumber,
				Title:       track.Title,
				DBRuntimeMS: track.ExpectedRuntimeMS,
			})
		}

		return result, nil
	}

	// Run mode — hand off to the normal organize pipeline via organizeAlbumFolderViaAPI.
	// Store the forced ID on the Organizer so organizeAlbumFolderViaAPI uses it.
	s.forcedAlbumID = *forcedID

	matchReason, _, err := s.organizeAlbumFolderViaAPI(ctx, folder, cfgp, data)

	s.forcedAlbumID = "" // reset after use

	if err != nil {
		return nil, fmt.Errorf("organize failed: %s: %w", matchReason, err)
	}

	return nil, nil
}

// Package scanner provides file system operations for media file handling,
// including moving, copying, and validating files based on media type extensions.
package scanner

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
)

// MoveFileOptions configures behavior for the MoveFile function.
type MoveFileOptions struct {
	UseOther bool // Check "other" extensions (subtitles, NFOs, etc.) instead of primary
	UseNil   bool // Skip extension validation entirely
	// UseBufferCopy is kept for compatibility; cross-device moves always use a
	// buffered, size-verified copy with the configured MoveBufferSizeKB now.
	UseBufferCopy bool
	ChmodFolder   string // Folder permissions in octal format (e.g., "0755" or "755")
	Chmod         string // File permissions in octal format (e.g., "0644" or "644")
	MediaType     uint   // Media type constant for extension checking
}

// tmpMoveSuffix marks the staging files written next to a final destination
// before they are swapped in. uniqueTmpPath appends "-<pid>-<random>" so
// concurrent moves never share a staging file. The suffix leaves the file with
// an extension no media/other-extension list contains, so library scans never
// import a staging file left behind by a crash.
const tmpMoveSuffix = ".tmp-move"

var (
	strto               = "to"
	errSizeMismatch     = errors.New("copied size does not match source size")
	errDstInsideSrc     = errors.New("destination folder is inside the source folder")
	errFolderIncomplete = errors.New("folder move incomplete - source kept")
)

// uniqueTmpPath returns a staging path next to path that no other move
// (concurrent or earlier) uses.
func uniqueTmpPath(path string) string {
	var b [6]byte

	rand.Read(b[:])

	return path + tmpMoveSuffix + "-" + strconv.Itoa(os.Getpid()) + "-" + hex.EncodeToString(b[:])
}

// chmodLogged applies mode to path and logs (but does not fail on) errors.
func chmodLogged(path string, mode fs.FileMode) {
	if err := os.Chmod(path, mode); err != nil {
		logger.Logtype("warn", 2).
			Str(logger.StrFile, path).
			Str("mode", mode.String()).
			Err(err).
			Msg("chmod failed")
	}
}

// mkdirAllMode creates dir and any missing parents. MkdirAll applies the umask,
// so every directory level it creates (not just the last one) is chmod'ed to
// mode afterwards. mode 0 creates with 0o777 (umask applies) and skips chmod.
func mkdirAllMode(dir string, mode fs.FileMode) error {
	var created []string

	for p := filepath.Clean(dir); ; {
		if _, err := os.Lstat(p); err == nil || !errors.Is(err, fs.ErrNotExist) {
			break
		}

		created = append(created, p)

		parent := filepath.Dir(p)
		if parent == p {
			break
		}

		p = parent
	}

	if len(created) == 0 {
		return nil
	}

	mkmode := mode
	if mkmode == 0 {
		mkmode = 0o777
	}

	if err := os.MkdirAll(dir, mkmode); err != nil {
		return err
	}

	if mode != 0 {
		for _, p := range created {
			chmodLogged(p, mode)
		}
	}

	return nil
}

// MoveFile moves a file from one path to another. It handles checking
// extensions, renaming, and setting permissions.
//
// The move is staged: the content is first placed at a unique ".tmp-move-…"
// path next to the destination (instant rename on the same filesystem,
// verified buffered copy across devices) and only then swapped in over any existing
// destination file. At every instant either the old or the complete new file
// exists, and a failed move leaves the source at its original path.
//
// Returns the new file path on success, or an error if the move failed.
func MoveFile(
	file string,
	setpathcfg *config.PathsConfig,
	target, newname string,
	opts MoveFileOptions,
) (string, error) {
	if !CheckFileExist(file) {
		return "", logger.ErrNotFound
	}

	ext := filepath.Ext(file)

	ok, oknorename := checkExtensionPermissions(
		opts.MediaType,
		setpathcfg,
		ext,
		opts.UseOther,
		opts.UseNil,
	)
	if !ok {
		return "", logger.ErrNotAllowed
	}

	newfilename := determineNewFilename(file, newname, ext, oknorename)
	logger.Path(&newfilename, false)

	newpath := filepath.Join(target, newfilename)

	// Moving a file onto itself must be a no-op - the previous flow deleted
	// the destination first, which would have destroyed the file.
	srcAbs, _ := filepath.Abs(file)

	dstAbs, _ := filepath.Abs(newpath)
	if srcAbs == dstAbs {
		return newpath, nil
	}

	logger.Logtype("info", 0).Str(logger.StrFile, file).Str(strto, newpath).Msg("File move start")

	err := moveToTempThenSwap(
		file,
		newpath,
		parseFileMode(opts.ChmodFolder),
		parseFileMode(opts.Chmod),
	)
	if err != nil {
		return "", err
	}

	logger.Logtype("info", 0).Str(logger.StrFile, file).Str(strto, newpath).Msg("File moved from")

	return newpath, nil
}

// PlannedMovePath returns the path MoveFile would move file to with the same
// arguments, without touching the filesystem.
func PlannedMovePath(
	file string,
	setpathcfg *config.PathsConfig,
	target, newname string,
	opts MoveFileOptions,
) string {
	ext := filepath.Ext(file)
	_, oknorename := checkExtensionPermissions(
		opts.MediaType,
		setpathcfg,
		ext,
		opts.UseOther,
		opts.UseNil,
	)

	newfilename := determineNewFilename(file, newname, ext, oknorename)
	logger.Path(&newfilename, false)

	return filepath.Join(target, newfilename)
}

// moveToTempThenSwap stages the file at a unique temp path next to newpath and
// only then swaps it in over any existing destination (delete + same-directory
// rename). Failures leave the source untouched (copy path) or restore it to
// its original path (rename path), so the move can always be retried.
func moveToTempThenSwap(file, newpath string, folderMode, fileMode fs.FileMode) error {
	if folderMode == 0 {
		folderMode = 0o777
	}

	if err := mkdirAllMode(filepath.Dir(newpath), folderMode); err != nil {
		return err
	}

	tmppath := uniqueTmpPath(newpath)

	// Fast path: same-filesystem rename is atomic and instant. Only a
	// cross-device error justifies the copy fallback - any other rename
	// failure (permissions, missing source, ...) would fail the same way or
	// leave a duplicate behind.
	renamed := false
	if err := os.Rename(file, tmppath); err == nil {
		renamed = true
	} else if !isCrossDevice(err) {
		return err
	} else if err := copyFileVerified(file, tmppath); err != nil {
		// Cross-device: stage via a verified buffered copy. The source stays
		// untouched until the swap has succeeded.
		return err
	}

	if fileMode != 0 {
		chmodLogged(tmppath, fileMode)
	}

	if err := swapInto(tmppath, newpath); err != nil {
		if renamed {
			// Restore the source so a retry finds it at the original path.
			if errrb := os.Rename(tmppath, file); errrb != nil {
				logger.Logtype("error", 2).
					Str(logger.StrFile, tmppath).
					Str(strto, file).
					Err(errrb).
					Msg("Move rollback failed - file left at staging path")
			}
		} else {
			os.Remove(tmppath)
		}

		return err
	}

	if !renamed {
		// Copy path: the verified new file is in place - now the source can go.
		// A failed source removal does not fail the move itself.
		if _, err := SecureRemove(file); err != nil {
			logger.Logtype("warn", 2).
				Str(logger.StrFile, file).
				Err(err).
				Msg("Move succeeded but source file could not be removed")
		}
	}

	return nil
}

// swapInto replaces newpath with the staged file at tmppath. The existing
// destination is removed only now, when the complete new content already
// sits next to it, shrinking the data-loss window to a same-directory rename.
func swapInto(tmppath, newpath string) error {
	if CheckFileExist(newpath) {
		logger.Logtype("info", 0).Str(strto, newpath).Msg("File remove start")

		if _, err := SecureRemove(newpath); err != nil {
			return err
		}
	}

	return os.Rename(tmppath, newpath)
}

// MoveFolder moves an entire folder from src to dst (full target path including folder name).
// It first attempts os.Rename (fast, same-drive). If that fails (cross-drive,
// or dst already exists), it falls back to copying every regular file to a
// temp name next to its destination and renaming it into place once the copy
// is verified, then removes the source. Pre-existing destination files are
// only ever replaced by a complete copy, never truncated or removed on
// failure. Symlinks and other non-regular entries are skipped with a warning;
// if anything was skipped or failed the source is kept.
// opts.ChmodFolder/opts.Chmod are applied to created directories and files.
func MoveFolder(src, dst string, opts MoveFileOptions) error {
	if !CheckFileExist(src) {
		return logger.ErrNotFound
	}

	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	if srcAbs == dstAbs {
		return nil
	}

	// Copying a folder into its own subtree would recurse into the copy and
	// RemoveAll(src) would then delete the copy too.
	if rel, err := filepath.Rel(srcAbs, dstAbs); err == nil && rel != ".." &&
		!strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return errDstInsideSrc
	}

	folderMode := parseFileMode(opts.ChmodFolder)
	fileMode := parseFileMode(opts.Chmod)

	if err := mkdirAllMode(filepath.Dir(dstAbs), folderMode); err != nil {
		return err
	}

	// Try rename first (same drive, fast)
	if err := os.Rename(srcAbs, dstAbs); err == nil {
		logger.Logtype("info", 1).
			Str(logger.StrFile, src).
			Str(strto, dst).
			Msg("Folder moved")

		return nil
	}

	// Fallback: walk, copy each file, then remove source
	var skipped int

	err = filepath.WalkDir(srcAbs, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, relErr := filepath.Rel(srcAbs, path)
		if relErr != nil {
			return relErr
		}

		targetPath := filepath.Join(dstAbs, relPath)

		if d.IsDir() {
			return mkdirAllMode(targetPath, folderMode)
		}

		if !d.Type().IsRegular() {
			skipped++

			logger.Logtype("warn", 1).
				Str(logger.StrFile, path).
				Str("type", d.Type().String()).
				Msg("Folder move: skipping symlink/non-regular file")

			return nil
		}

		return copyFileAtomic(path, targetPath, fileMode)
	})
	if err != nil {
		return err
	}

	if skipped > 0 {
		return fmt.Errorf("%w: %d entries skipped", errFolderIncomplete, skipped)
	}

	logger.Logtype("info", 1).
		Str(logger.StrFile, src).
		Str(strto, dst).
		Msg("Folder moved (cross-drive)")

	return os.RemoveAll(srcAbs)
}

// checkExtensionPermissions validates if a file extension is allowed for processing
// based on the path configuration and processing flags.
//
// Parameters:
//   - mediaType: Media type for extension checking (0=movie, 1=series, 2=book, 3=audiobook, 4=music)
//   - setpathcfg: Path configuration containing allowed extensions
//   - ext: File extension to check (should include dot, e.g., ".mp4")
//   - useother: Whether to check against "other" extensions list
//   - usenil: Whether to skip validation entirely (returns true)
//
// Returns:
//   - bool: True if extension is allowed for processing
//   - bool: True if renaming is allowed for this extension
func checkExtensionPermissions(
	mediaType uint,
	setpathcfg *config.PathsConfig,
	ext string,
	useother, usenil bool,
) (bool, bool) {
	if usenil || setpathcfg == nil {
		return true, true
	}

	return CheckExtensionsType(mediaType, useother, setpathcfg, ext)
}

// determineNewFilename generates a new filename for a file during a move operation.
// Uses original filename if no new name specified or renaming is skipped (oknorename).
// Ensures the filename includes the original file extension (case-insensitive check).
func determineNewFilename(file, newname, ext string, oknorename bool) string {
	if newname == "" || oknorename {
		return capFileName(filepath.Base(file))
	}

	// Check if extension already present (case-insensitive)
	if logger.HasSuffixI(newname, ext) {
		return capFileName(newname)
	}

	return capFileName(newname + ext)
}

// maxFileNameBytes keeps a file name, plus the staging suffix MoveFile adds
// while copying (tmpMoveSuffix + "-<pid>-<random>"), below the 255-byte name
// limit of common filesystems. Very long track titles otherwise fail with
// "file name too long".
const maxFileNameBytes = 200

// capFileName shortens name to maxFileNameBytes, cutting the part before the
// extension at a UTF-8 boundary and keeping the extension.
func capFileName(name string) string {
	if len(name) <= maxFileNameBytes {
		return name
	}

	ext := filepath.Ext(name)
	if len(ext) > 16 {
		ext = ""
	}

	stem := name[:len(name)-len(ext)]

	n := maxFileNameBytes - len(ext)
	for n > 0 && !utf8.RuneStart(stem[n]) {
		n--
	}

	return strings.TrimRight(stem[:n], " .-_") + ext
}

// parseFileMode parses an octal permission string ("755" or "0755").
// Returns 0 for anything else, which callers treat as "not configured".
func parseFileMode(s string) fs.FileMode {
	if len(s) == 3 || len(s) == 4 {
		return logger.StringToFileMode(s)
	}

	return 0
}

// CheckExtensions checks if the given file extension is allowed based on the
// checkvideo and checkother flags. If both are true, video is checked first
// and falls back to other extensions if video doesn't match.
func CheckExtensions(
	checkvideo, checkother bool,
	pathcfg *config.PathsConfig,
	ext string,
) (bool, bool) {
	// Check video extensions first if requested
	if checkvideo {
		if ok, norename := mediatype.CheckVideoExtensions(pathcfg, ext); ok {
			return ok, norename
		}

		// If not checking other extensions, return the video result
		if !checkother {
			return false, false
		}
	}

	// Check other extensions if requested
	if checkother {
		return mediatype.CheckOtherExtensions(pathcfg, ext)
	}

	return false, false
}

// CheckExtensionsType checks if the given file extension is allowed for the
// specified media type. It delegates to the mediatype package which routes
// to the appropriate handler based on media type.
//
// Parameters:
//   - mediaType: Media type constant (config.MediaTypeMovie, config.MediaTypeSeries, etc.)
//   - checkother: If true, checks "other" extensions (subtitles, NFOs, etc.)
//   - pathcfg: Path configuration containing allowed extensions
//   - ext: File extension to check (should include dot, e.g., ".mp4")
//
// Returns:
//   - bool: True if extension is allowed for processing
//   - bool: True if renaming should be skipped for this extension
func CheckExtensionsType(
	mediaType uint,
	checkother bool,
	pathcfg *config.PathsConfig,
	ext string,
) (bool, bool) {
	return mediatype.CheckExtensions(mediaType, checkother, pathcfg, ext)
}

// RemoveFile removes the file at the given path if it exists.
// It uses Lstat, so a dangling symlink is removed too.
// Returns (true, nil) when the file was removed and (false, nil) when it did
// not exist - callers whose goal is "file gone" should treat both as success.
func RemoveFile(file string) (bool, error) {
	if _, err := os.Lstat(file); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return SecureRemove(file)
}

// SecureRemove attempts to remove the file at the given path. If the file is successfully removed,
// it logs an informational message. If the file cannot be removed, it logs an error message and returns the error.
// Unlink permission lives on the parent directory, so a permission error is
// returned as-is rather than loosening the file's own mode.
func SecureRemove(file string) (bool, error) {
	err := os.Remove(file)
	if err == nil {
		logger.Logtype("info", 1).
			Str(logger.StrFile, file).
			Msg("File removed")
		return true, nil
	}

	logger.Logtype("error", 1).
		Str(logger.StrFile, file).
		Err(err).
		Msg("File not removed")

	return false, err
}

// checkFile performs file validation checks based on the provided flags.
// It can check for file existence and whether the file is a regular file (not a directory or special file).
//
// Parameters:
//   - fpath: Path to the file to check
//   - checkexists: Whether to verify the file exists
//   - checkregular: Whether to verify the file is a regular file
//
// Returns:
//   - bool: True if all requested checks pass, false otherwise
//
// Any Stat error (not only "not exist" - also permission denied, a non-directory
// path component, symlink loops) counts as a failed check, so callers never go
// on to operate on a path they cannot actually access.
func checkFile(fpath string, checkexists, checkregular bool) bool {
	sfi, err := os.Stat(fpath)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			logger.Logtype("warn", 2).
				Str(logger.StrFile, fpath).
				Err(err).
				Msg("file check failed")
		}

		return false
	}

	if checkregular {
		return sfi.Mode().IsRegular()
	}

	return checkexists
}

// CheckFileExist checks if the file exists at the given file path.
// It returns true only if the path can be stat'ed successfully.
func CheckFileExist(fpath string) bool {
	return checkFile(fpath, true, false)
}

// copyFileAtomic copies src to a unique temp file next to dst and renames it
// into place only after the copy is verified. A pre-existing dst is therefore
// either replaced by the complete new content or left untouched; on failure
// only the temp file this call created is removed. dst's directory must exist.
func copyFileAtomic(src, dst string, fileMode fs.FileMode) error {
	tmp := uniqueTmpPath(dst)
	if err := copyFileVerified(src, tmp); err != nil {
		return err
	}

	if fileMode != 0 {
		chmodLogged(tmp, fileMode)
	}

	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return err
	}

	return nil
}

// copyFileVerified copies src to dst with the configured buffer size, checks
// the available disk space first, verifies the copied byte count against the
// source size, fsyncs, and preserves the source modification time. On any
// failure the partial destination file is removed so a retry is never blocked
// by leftovers. dst must not exist yet (it is created exclusively, so an
// existing file is never truncated or removed). The destination directory
// must already exist.
func copyFileVerified(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	// Cannot copy non-regular files (directories, symlinks, devices, ...)
	if !srcInfo.Mode().IsRegular() {
		return errors.New("CopyFile: non-regular source file " + src)
	}

	// Fail fast with a clear error instead of at the end of a long copy.
	// freeDiskSpace returns -1 when it cannot be determined (fail open).
	if free := freeDiskSpace(filepath.Dir(dst)); free >= 0 && free < srcInfo.Size() {
		return errors.New(
			"insufficient disk space: need " + strconv.FormatInt(srcInfo.Size(), 10) +
				" bytes, have " + strconv.FormatInt(free, 10),
		)
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return err
	}

	bufferKB := 1024
	if s := config.GetSettingsGeneral(); s != nil && s.MoveBufferSizeKB > 0 {
		bufferKB = s.MoveBufferSizeKB
	}

	written, err := io.CopyBuffer(dstFile, srcFile, make([]byte, bufferKB*1024))
	if err == nil {
		err = dstFile.Sync()
	}

	if errc := dstFile.Close(); err == nil {
		err = errc
	}

	// Verify the copy is complete before anyone deletes the source.
	if err == nil && written != srcInfo.Size() {
		err = errSizeMismatch
	}

	if err != nil {
		// Remove the partial destination so a retry is not blocked.
		os.Remove(dst)
		return err
	}

	// Preserve the source modification time (best effort) - rename-based
	// moves keep it, so copy-based moves should too.
	os.Chtimes(dst, time.Now(), srcInfo.ModTime())

	return nil
}

// AppendCsv appends a line to the CSV file at fpath.
// It opens the file for appending, creating it if needed, with permissions 0777.
// It writes the line to the file with a newline separator.
// It handles logging and returning any errors.
func AppendCsv(fpath, line string) {
	f, err := os.OpenFile(fpath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o777)
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, fpath).
			Err(err).
			Msg("Error opening csv to write")

		return
	}
	defer f.Close()

	_, err = f.WriteString(logger.JoinStrings(line, "\n"))
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, fpath).
			Err(err).
			Msg("Error writing to csv")
	} else {
		logger.Logtype("debug", 0).
			Msg("csv written")
	}

	f.Sync()
}

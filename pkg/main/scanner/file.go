// Package scanner provides file system operations for media file handling,
// including moving, copying, and validating files based on media type extensions.
package scanner

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

// tmpMoveSuffix marks the staging file MoveFile writes next to the final
// destination before swapping it in.
const tmpMoveSuffix = ".tmp-move"

var (
	strto           = "to"
	errSameFile     = errors.New("same file")
	errSizeMismatch = errors.New("copied size does not match source size")
)

// MoveFile moves a file from one path to another. It handles checking
// extensions, renaming, and setting permissions.
//
// The move is staged: the content is first placed at the destination path
// with a ".tmp-move" suffix (instant rename on the same filesystem, verified
// buffered copy across devices) and only then swapped in over any existing
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

// moveToTempThenSwap stages the file at newpath+".tmp-move" and only then
// swaps it in over any existing destination (delete + same-directory rename).
// Failures leave the source untouched (copy path) or restore it to its
// original path (rename path), so the move can always be retried.
func moveToTempThenSwap(file, newpath string, folderMode, fileMode fs.FileMode) error {
	if folderMode == 0 {
		folderMode = 0o777
	}

	targetDir := filepath.Dir(newpath)
	if !CheckFileExist(targetDir) {
		if err := os.MkdirAll(targetDir, folderMode); err != nil {
			return err
		}

		// Ensure permissions are set (MkdirAll may apply umask)
		os.Chmod(targetDir, folderMode)
	}

	tmppath := newpath + tmpMoveSuffix
	// Remove a stale staging file from a previously interrupted move so the
	// retry isn't blocked.
	if CheckFileExist(tmppath) {
		if _, err := SecureRemove(tmppath); err != nil {
			return err
		}
	}

	// Fast path: same-filesystem rename is atomic and instant.
	renamed := os.Rename(file, tmppath) == nil
	if !renamed {
		// Cross-device: stage via a verified buffered copy. The source stays
		// untouched until the swap has succeeded.
		if err := copyFileVerified(file, tmppath); err != nil {
			return err
		}
	}

	if fileMode != 0 {
		os.Chmod(tmppath, fileMode)
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
// It first attempts os.Rename (fast, same-drive). If that fails (cross-drive),
// it falls back to copying all files individually and removing the source.
func MoveFolder(src, dst string) error {
	if !CheckFileExist(src) {
		return logger.ErrNotFound
	}

	srcAbs, _ := filepath.Abs(src)

	dstAbs, _ := filepath.Abs(dst)
	if srcAbs == dstAbs {
		return nil
	}

	// Try rename first (same drive, fast)
	if err := os.Rename(src, dst); err == nil {
		logger.Logtype("info", 1).
			Str(logger.StrFile, src).
			Str(strto, dst).
			Msg("Folder moved")

		return nil
	}

	// Cross-drive fallback: walk, copy each file, then remove source
	if err := os.MkdirAll(dst, 0o777); err != nil {
		return err
	}

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return relErr
		}

		targetPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			return os.MkdirAll(targetPath, 0o777)
		}

		return copyFile(path, targetPath, 0o777)
	})
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Str(logger.StrFile, src).
		Str(strto, dst).
		Msg("Folder moved (cross-drive)")

	return os.RemoveAll(src)
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
		return filepath.Base(file)
	}

	// Check if extension already present (case-insensitive)
	if logger.HasSuffixI(newname, ext) {
		return newname
	}

	return newname + ext
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
// It checks if the file exists first before removing.
// Returns a bool indicating if the file was removed, and an error if one occurred.
func RemoveFile(file string) (bool, error) {
	if !CheckFileExist(file) {
		return false, nil
	}

	return SecureRemove(file)
}

// SecureRemove attempts to remove the file at the given path. If the file cannot be removed due to a permissions error,
// it first attempts to change the file permissions to 0777 and then remove the file. If the file is successfully removed,
// it logs an informational message. If the file cannot be removed, it logs an error message and returns the error.
func SecureRemove(file string) (bool, error) {
	err := os.Remove(file)
	if errors.Is(err, os.ErrPermission) {
		os.Chmod(file, 0o777)

		err = os.Remove(file)
	}

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
func checkFile(fpath string, checkexists, checkregular bool) bool {
	sfi, err := os.Stat(fpath)
	if checkexists {
		return !errors.Is(err, os.ErrNotExist)
	}

	if checkregular {
		if err != nil {
			return false
		}

		return sfi.Mode().IsRegular()
	}

	return false
}

// CheckFileExist checks if the file exists at the given file path.
// It returns true if the file exists, and false if there is an error
// indicating the file does not exist.
func CheckFileExist(fpath string) bool {
	return checkFile(fpath, true, false)
}

// copyFile copies the contents of the file named src to the file named
// dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the
// contents of the source file. This function handles creating any missing
// directories in the destination path and setting permissions.
func copyFile(src, dst string, chmodfolder fs.FileMode) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return err
	}

	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return err
	}

	if srcAbs == dstAbs {
		return errSameFile
	}

	if !checkFile(filepath.Dir(dst), true, false) {
		if chmodfolder == 0 {
			chmodfolder = 0o777
		}

		if err = os.MkdirAll(filepath.Dir(dst), chmodfolder); err != nil {
			return err
		}

		// Ensure permissions are set (MkdirAll may apply umask)
		os.Chmod(filepath.Dir(dst), chmodfolder)
	}

	return copyFileVerified(srcAbs, dstAbs)
}

// copyFileVerified copies src to dst with the configured buffer size, checks
// the available disk space first, verifies the copied byte count against the
// source size, fsyncs, and preserves the source modification time. On any
// failure the partial destination file is removed so a retry is never blocked
// by leftovers. The destination directory must already exist.
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

	dstFile, err := os.Create(dst)
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

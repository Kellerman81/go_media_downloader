// Package scanner provides file system operations for media file handling,
// including moving, copying, and validating files based on media type extensions.
package scanner

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/mediatype"
)

// MoveFileOptions configures behavior for the MoveFile function.
type MoveFileOptions struct {
	UseOther      bool   // Check "other" extensions (subtitles, NFOs, etc.) instead of primary
	UseNil        bool   // Skip extension validation entirely
	UseBufferCopy bool   // Use buffered copy instead of rename (for cross-drive moves)
	ChmodFolder   string // Folder permissions in octal format (e.g., "0755")
	Chmod         string // File permissions in octal format (e.g., "0644")
	MediaType     uint   // Media type constant for extension checking
}

// strto is a reusable string constant for log messages.
var (
	strto       = "to"
	errSameFile = errors.New("same file")
)

// MoveFile moves a file from one path to another. It handles checking extensions,
// renaming, and setting permissions.
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

	renamepath := filepath.Join(filepath.Dir(file), newfilename)
	newpath := filepath.Join(target, newfilename)

	if err := prepareMove(file, newpath, renamepath); err != nil {
		return "", err
	}

	if err := performMove(renamepath, newpath, opts); err != nil {
		return "", err
	}

	logger.Logtype("info", 0).Str(logger.StrFile, file).Str("to", newpath).Msg("File moved from")

	return newpath, nil
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
			Str("to", dst).
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

		return copyFile(path, targetPath, false, 0o777)
	})
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Str(logger.StrFile, src).
		Str("to", dst).
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

// prepareMove prepares a file for moving by logging the move operation,
// removing the destination file if it already exists, and renaming the source file
// to an intermediate path before the final move.
func prepareMove(file, newpath, renamepath string) error {
	logger.Logtype("info", 0).Str(logger.StrFile, file).Str("to", newpath).Msg("File move start")

	if CheckFileExist(newpath) {
		logger.Logtype("info", 0).Str("to", newpath).Msg("File remove start")
		RemoveFile(newpath)
	}

	return os.Rename(file, renamepath)
}

// performMove executes the actual file move operation from the renamed path to the final destination.
// It handles different move strategies based on whether the source and destination are on the same drive,
// and applies appropriate permissions after the move.
//
// Parameters:
//   - renamepath: Intermediate path where the file was temporarily renamed
//   - newpath: Final destination path for the file
//   - opts: Move options containing chmod settings and buffer copy preferences
//
// Returns:
//   - error: Any error encountered during the move operation
func performMove(renamepath, newpath string, opts MoveFileOptions) error {
	logger.Logtype("info", 0).
		Str(logger.StrFile, renamepath).
		Str("to", newpath).
		Msg("File move start move")

	var fileMode fs.FileMode
	if len(opts.Chmod) == 4 {
		fileMode = logger.StringToFileMode(opts.Chmod)
	}

	// Fast path: same-filesystem rename is atomic and instant.
	if err := os.Rename(renamepath, newpath); err == nil {
		if fileMode != 0 {
			os.Chmod(newpath, fileMode)
		}

		return nil
	}

	// Slow path: cross-device — copy then delete.
	if opts.UseBufferCopy {
		if fileMode != 0 {
			setchmod(renamepath, fileMode)
		}

		return moveFileDriveBufferRemove(renamepath, newpath, fileMode)
	}

	var folderMode fs.FileMode
	if len(opts.ChmodFolder) == 4 {
		folderMode = logger.StringToFileMode(opts.ChmodFolder)
	}

	return moveFileDrive(renamepath, newpath, folderMode, fileMode)
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

// setchmod sets file permissions on the specified file.
// It silently ignores errors as this is a best-effort operation.
func setchmod(file string, chmod fs.FileMode) {
	if chmod != 0 {
		os.Chmod(file, chmod)
	}
}

// RemoveFile removes the file at the given path if it exists.
// It checks if the file exists first before removing.
// Returns a bool indicating if the file was removed, and an error if one occurred.
func RemoveFile(file string) (bool, error) {
	// logger.Logtype("info", 1).Str("File", file).Msg("Pre RemoveFile1")
	if !CheckFileExist(file) {
		return false, nil
	}

	return SecureRemove(file)
}

// SecureRemove attempts to remove the file at the given path. If the file cannot be removed due to a permissions error,
// it first attempts to change the file permissions to 0777 and then remove the file. If the file is successfully removed,
// it logs an informational message. If the file cannot be removed, it logs an error message and returns the error.
func SecureRemove(file string) (bool, error) {
	// logger.Logtype("info", 1).Str("File", file).Msg("Pre RemoveFile1-1")
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

// moveFileDriveBuffer moves a file from sourcePath to destPath using a buffer
// to avoid memory issues with large files. It checks that the source file exists
// and is a regular file, and that the destination does not already exist.
// It copies the file in chunks using a buffer size determined by the
// MoveBufferSizeKB setting, or 1024 KB by default.
func moveFileDriveBuffer(sourcePath, destPath string) error {
	if !checkFile(sourcePath, false, true) {
		return errors.New(sourcePath + " is not a regular file")
	}

	if CheckFileExist(destPath) {
		return errors.New(destPath + " already exists")
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	// Determine buffer size (allocate once outside loop)
	bufferKB := config.GetSettingsGeneral().MoveBufferSizeKB
	if bufferKB == 0 {
		bufferKB = 1024
	}

	buf := make([]byte, bufferKB*1024)

	for {
		n, err := source.Read(buf)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}

		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
			return err
		}
	}

	return destination.Sync()
}

// moveFileDriveBufferRemove moves the file at sourcePath to destPath using a buffer,
// sets the file permissions to chmod, and deletes the original source file
// after a successful copy. Returns any error.
func moveFileDriveBufferRemove(sourcePath, destPath string, chmod fs.FileMode) error {
	if err := moveFileDriveBuffer(sourcePath, destPath); err != nil {
		return err
	}

	if _, err := SecureRemove(sourcePath); err != nil {
		return errors.New("failed removing original file: " + err.Error())
	}

	if chmod != 0 {
		os.Chmod(destPath, chmod)
	}

	return nil
}

// moveFileDrive copies the file at sourcePath to destPath, setting the folder
// permissions to chmodfolder and file permissions to chmod after the copy.
// It handles deleting the original source file after a successful copy.
// Returns any errors from the copy or file deletions.
func moveFileDrive(sourcePath, destPath string, chmodfolder, chmod fs.FileMode) error {
	logger.Logtype("info", 0).
		Str(logger.StrFile, sourcePath).
		Str(strto, destPath).
		Msg("File move begin")

	if err := copyFile(sourcePath, destPath, false, chmodfolder); err != nil {
		logger.Logtype("error", 0).
			Str(logger.StrFile, sourcePath).
			Str(strto, destPath).
			Err(err).
			Msg("Error copying source")

		return err
	}

	logger.Logtype("info", 0).
		Str(logger.StrFile, sourcePath).
		Str(strto, destPath).
		Msg("File move end")

	if chmod != 0 {
		if err := os.Chmod(destPath, chmod); err != nil {
			return err
		}
	}

	_, err := SecureRemove(sourcePath)

	return err
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
func copyFile(src, dst string, allowFileLink bool, chmodfolder fs.FileMode) error {
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

	// open source file
	if !checkFile(srcAbs, false, true) {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return errors.New("CopyFile: non-regular source file " + srcAbs)
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

	// The destination file doesn't exist yet, so we don't need to check if it's regular
	// open dest file

	if allowFileLink {
		if os.Link(src, dst) == nil {
			return nil
		}
	}

	return performFileCopy(src, dst)
}

// performFileCopy copies the contents of the source file to the destination file.
// It opens the source file for reading and creates the destination file,
// then uses io.Copy to transfer the contents. It ensures the destination
// file is synchronized to disk after copying. Returns any error encountered
// during the file copy process.
func performFileCopy(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return dstFile.Sync()
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

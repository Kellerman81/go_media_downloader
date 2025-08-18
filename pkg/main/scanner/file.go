package scanner

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
)

type MoveFileOptions struct {
	UseOther      bool
	UseNil        bool
	UseBufferCopy bool
	ChmodFolder   string
	Chmod         string
}

var strto = "to"

// MoveFile moves a file from one path to another. It handles checking extensions, renaming, setting permissions etc.
// It returns a bool indicating if the move was successful, and an error.
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
	ok, oknorename := checkExtensionPermissions(setpathcfg, ext, opts.UseOther, opts.UseNil)
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

// checkExtensionPermissions validates if a file extension is allowed for processing
// based on the path configuration and processing flags.
//
// Parameters:
//   - setpathcfg: Path configuration containing allowed extensions
//   - ext: File extension to check (should include dot, e.g., ".mp4")
//   - useother: Whether to check against "other" extensions list
//   - usenil: Whether to skip validation entirely (returns true)
//
// Returns:
//   - bool: True if extension is allowed for processing
//   - bool: True if renaming is allowed for this extension
func checkExtensionPermissions(
	setpathcfg *config.PathsConfig,
	ext string,
	useother, usenil bool,
) (bool, bool) {
	if usenil || setpathcfg == nil {
		return true, true
	}
	return CheckExtensions(!useother, useother, setpathcfg, ext)
}

// determineNewFilename generates a new filename for a file during a move operation.
// It handles filename generation based on provided parameters:
// - If no new name is specified or renaming is allowed, it uses the original filename
// - If a new name is provided, it uses that name
// - Ensures the filename includes the original file extension
func determineNewFilename(file, newname, ext string, oknorename bool) string {
	var newfilename string
	if newname == "" || oknorename {
		newfilename = filepath.Base(file)
	} else {
		newfilename = newname
	}

	if !strings.HasSuffix(newfilename, ext) {
		newfilename += ext
	}
	return newfilename
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

// CheckExtensions checks if the given file extension is allowed for the
// provided checkvideo and checkother booleans. It returns a bool for if the
// extension is allowed, and a bool for if renaming should be skipped.
func CheckExtensions(
	checkvideo, checkother bool,
	pathcfg *config.PathsConfig,
	ext string,
) (bool, bool) {
	if checkvideo {
		if ok, oknorename := checkVideoExtensions(pathcfg, ext); !checkother {
			return ok, oknorename
		} else if ok {
			return ok, oknorename
		}
	}
	if checkother {
		return checkOtherExtensions(pathcfg, ext)
	}

	return false, false
}

// checkVideoExtensions checks if the given file extension is allowed for video file types.
// It returns two booleans: the first indicates if the extension is allowed,
// and the second indicates whether renaming should be skipped.
// If no video extensions are configured, it returns (true, true).
// If the extension is in the allowed list, it returns (true, false).
// If the extension is in the no-rename list, it returns (true, true).
// Otherwise, it returns (false, false).
func checkVideoExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	if pathcfg.AllowedVideoExtensionsLen == 0 {
		return true, true
	}

	if logger.SlicesContainsI(pathcfg.AllowedVideoExtensions, ext) {
		return true, false
	}

	if pathcfg.AllowedVideoExtensionsNoRenameLen > 0 &&
		logger.SlicesContainsI(pathcfg.AllowedVideoExtensionsNoRename, ext) {
		return true, true
	}

	return false, false
}

// checkOtherExtensions checks if the given file extension is allowed for other file types.
// It returns two booleans: the first indicates if the extension is allowed,
// and the second indicates whether renaming should be skipped.
// If no other extensions are configured, it returns (true, true).
// If the extension is in the allowed list, it returns (true, false).
// If the extension is in the no-rename list, it returns (true, true).
// Otherwise, it returns (false, false).
func checkOtherExtensions(pathcfg *config.PathsConfig, ext string) (bool, bool) {
	if pathcfg.AllowedOtherExtensionsLen == 0 {
		return true, true
	}

	if logger.SlicesContainsI(pathcfg.AllowedOtherExtensions, ext) {
		return true, false
	}

	if pathcfg.AllowedOtherExtensionsNoRenameLen > 0 &&
		logger.SlicesContainsI(pathcfg.AllowedOtherExtensionsNoRename, ext) {
		return true, true
	}

	return false, false
}

// setchmod sets file permissions on the specified file.
// It logs any errors encountered during the chmod operation but does not return them.
// This function is used to apply consistent permissions after file operations.
//
// Parameters:
//   - file: Path to the file whose permissions should be changed
//   - chmod: File mode/permissions to apply to the file
func setchmod(file string, chmod fs.FileMode) {
	if chmod == 0 {
		return
	}
	if f, err := os.Open(file); err == nil {
		defer f.Close()
		f.Chmod(chmod)
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
	bufferkb := 1024

	buffersize := config.GetSettingsGeneral().MoveBufferSizeKB
	if buffersize != 0 {
		bufferkb = buffersize
	}

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

	for {
		buf := make([]byte, int64(bufferkb)*1024)
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

// MoveFileDriveBuffer moves the file at sourcePath to destPath using a buffer,
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
		return errors.New("same file")
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
		err = os.MkdirAll(filepath.Dir(dst), chmodfolder)
		if err != nil {
			return err
		}
		if chmodfolder != 0 {
			os.Chmod(filepath.Dir(dst), chmodfolder)
		}
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

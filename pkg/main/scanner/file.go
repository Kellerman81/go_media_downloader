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

var strto = "to"

// MoveFile moves a file from one path to another. It handles checking extensions, renaming, setting permissions etc.
// It returns a bool indicating if the move was successful, and an error.
func MoveFile(file string, setpathcfg *config.PathsConfig, target, newname string, useother, usenil, usebuffercopy bool, chmodfolder, chmod string) (string, error) {
	if !CheckFileExist(file) {
		return "", logger.ErrNotFound
	}
	ext := filepath.Ext(file)
	var ok, oknorename bool
	if usenil || setpathcfg == nil {
		ok = true
		oknorename = true
	} else {
		ok, oknorename = CheckExtensions(!useother, useother, setpathcfg, ext)
	}

	if !ok {
		return "", logger.ErrNotAllowed
	}
	var newfilename string
	if newname == "" || oknorename {
		newfilename = filepath.Base(file)
	} else {
		newfilename = newname
	}
	if !strings.HasSuffix(newfilename, ext) {
		newfilename = (newfilename + ext)
	}
	logger.Path(&newfilename, false)
	renamepath := filepath.Join(filepath.Dir(file), newfilename)
	newpath := filepath.Join(target, newfilename)

	logger.Logtype("info", 0).Str(logger.StrFile, file).Str(strto, newpath).Msg("File move start")
	if target != newpath && CheckFileExist(newpath) {
		// Remove Target to suppress error

		logger.Logtype("info", 0).Str(strto, newpath).Msg("File remove start")
		RemoveFile(newpath)
	}
	err := os.Rename(file, renamepath)
	if err != nil {
		return "", err
	}

	logger.Logtype("info", 0).Str(logger.StrFile, file).Str(strto, newpath).Msg("File move start move")
	var uchmod fs.FileMode
	if chmod != "" && len(chmod) == 4 {
		uchmod = logger.StringToFileMode(chmod)
	}
	if usebuffercopy {
		if chmod != "" && len(chmod) == 4 {
			setchmod(renamepath, uchmod)
		}
		err = moveFileDriveBufferRemove(renamepath, newpath, uchmod)
	} else {
		var uchmodfolder fs.FileMode
		if chmodfolder != "" && len(chmodfolder) == 4 {
			uchmodfolder = logger.StringToFileMode(chmodfolder)
		}
		err = moveFileDrive(renamepath, newpath, uchmodfolder, uchmod)
	}
	if err != nil {
		return "", err
	}
	logger.Logtype("info", 0).Str(logger.StrFile, file).Str(strto, newpath).Msg("File moved from")
	return newpath, nil
}

// CheckExtensions checks if the given file extension is allowed for the
// provided checkvideo and checkother booleans. It returns a bool for if the
// extension is allowed, and a bool for if renaming should be skipped.
func CheckExtensions(checkvideo, checkother bool, pathcfg *config.PathsConfig, ext string) (bool, bool) {
	var ok, oknorename bool
	if checkvideo {
		if pathcfg.AllowedVideoExtensionsLen == 0 {
			ok = true
			oknorename = true
		} else {
			ok = logger.SlicesContainsI(pathcfg.AllowedVideoExtensions, ext)
		}

		if !ok && pathcfg.AllowedVideoExtensionsNoRenameLen >= 1 {
			ok = logger.SlicesContainsI(pathcfg.AllowedVideoExtensionsNoRename, ext)
			if ok {
				oknorename = ok
			}
		}
	}
	if !checkother {
		return ok, oknorename
	}
	if pathcfg.AllowedOtherExtensionsLen == 0 {
		ok = true
		if ok {
			oknorename = ok
		}
	} else {
		ok = logger.SlicesContainsI(pathcfg.AllowedOtherExtensions, ext)
	}

	if !ok && pathcfg.AllowedOtherExtensionsNoRenameLen >= 1 {
		ok = logger.SlicesContainsI(pathcfg.AllowedOtherExtensionsNoRename, ext)
		if ok {
			oknorename = ok
		}
	}
	return ok, oknorename
}

// Setchmod sets the file mode permissions for the given file.
// If chmod is 0, no change is made. Otherwise, it opens the file,
// calls Chmod() to set the permissions, and closes the file.
// Returns without error handling if the file failed to open.
func setchmod(file string, chmod fs.FileMode) {
	if chmod == 0 {
		return
	}
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close()
	f.Chmod(chmod)
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
	if err != nil && errors.Is(err, os.ErrPermission) {
		os.Chmod(file, 0o777)
		err = os.Remove(file)
	}
	if err == nil {
		logger.LogDynamicany1String("info", "File removed", logger.StrFile, file)
		return true, nil
	}
	logger.LogDynamicany1StringErr("error", "File not removed", err, logger.StrFile, file)
	return false, err
}

// moveFileDriveBuffer moves a file from sourcePath to destPath using a buffer
// to avoid memory issues with large files. It checks that the source file exists
// and is a regular file, and that the destination does not already exist.
// It copies the file in chunks using a buffer size determined by the
// MoveBufferSizeKB setting, or 1024 KB by default.
func moveFileDriveBuffer(sourcePath, destPath string) error {
	bufferkb := 1024

	buffersize := config.SettingsGeneral.MoveBufferSizeKB
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

		_, err = destination.Write(buf[:n])
		if err != nil {
			return err
		}
	}
	return destination.Sync()
}

// MoveFileDriveBuffer moves the file at sourcePath to destPath using a buffer,
// sets the file permissions to chmod, and deletes the original source file
// after a successful copy. Returns any error.
func moveFileDriveBufferRemove(sourcePath, destPath string, chmod fs.FileMode) error {
	err := moveFileDriveBuffer(sourcePath, destPath)
	if err != nil {
		return err
	}
	_, err = SecureRemove(sourcePath)
	if err != nil {
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
	logger.Logtype("info", 0).Str(logger.StrFile, sourcePath).Str(strto, destPath).Msg("File move begin")
	err := copyFile(sourcePath, destPath, false, chmodfolder)
	if err != nil {
		logger.Logtype("error", 0).Str(logger.StrFile, sourcePath).Str(strto, destPath).Err(err).Msg("Error copiing source")
		return err
	}
	logger.Logtype("info", 0).Str(logger.StrFile, sourcePath).Str(strto, destPath).Msg("File move end")

	if chmod != 0 {
		err = os.Chmod(destPath, chmod)
		if err != nil {
			return err
		}
	}
	_, err = SecureRemove(sourcePath)
	return err
}

// checkFile checks if the file at fpath exists and/or is a regular file,
// based on the provided checkexists and checkregular flags.
func checkFile(fpath string, checkexists, checkregular bool) bool {
	sfi, err := os.Stat(fpath)
	if checkexists {
		return !errors.Is(err, fs.ErrNotExist)
	}
	if checkregular {
		if err != nil {
			return true
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
	if !checkFile(dstAbs, false, true) {
		return errors.New("CopyFile: non-regular destination file " + dstAbs)
	}
	// open dest file

	if allowFileLink {
		if os.Link(src, dst) == nil {
			return nil
		}
	}

	// Open the source file for reading
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	// Open the destination file for writing
	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()
	// Return any errors that result from closing the destination file
	// Will return nil if no errors occurred

	// Copy the contents of the source file into the destination files
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}
	// Sync and set permissions if needed
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
		logger.LogDynamicanyErr("error", "Error opening csv to write", err)
		return
	}
	defer f.Close()
	_, err = f.WriteString(logger.JoinStrings(line, "\n"))
	if err != nil {
		logger.LogDynamicanyErr("error", "Error writing to csv", err)
	} else {
		logger.LogDynamicany0("info", "csv written")
	}
	f.Sync()
}

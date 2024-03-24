package scanner

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
)

// MoveResponse represents the response from moving a file
// It contains the new path of the moved file, any error that occurred,
// and a bool indicating if the move was successful
type MoveResponse struct {
	// NewPath is the new path of the moved file
	NewPath string
	// Err contains any error that occurred during the move
	Err error
	// MoveDone indicates if the move was successful
	MoveDone bool
}

type NewFileData struct {
	Cfgp       *config.MediaTypeConfig
	PathCfg    *config.PathsConfig
	Listid     int
	Checkfiles bool
	AddFound   bool
}

// Filterfiles checks if the given path should be filtered based on the
// provided media type config and paths config. It checks the useseries lists
// in the config to see if the path is in the allowed files or unmatched files lists.
// It also calls Filterfile to check the ignore/filter rules if checkfiles is true.
// Returns true if the path should be filtered/ignored.
func Filterfiles(pathv *string, filedata *NewFileData) bool {
	if filedata.Checkfiles {
		if config.SettingsGeneral.UseFileCache {
			if database.SlicesCacheContains(logger.GetStringsMap(filedata.Cfgp.Useseries, logger.CacheFiles), *pathv) {
				return true
			}
		} else if database.GetdatarowN[uint](false, logger.GetStringsMap(filedata.Cfgp.Useseries, logger.DBCountFilesLocation), pathv) >= 1 {
			return true
		}
	}
	//CheckUnmatched
	if config.SettingsGeneral.UseFileCache {
		if database.SlicesCacheContains(logger.GetStringsMap(filedata.Cfgp.Useseries, logger.CacheUnmatched), *pathv) {
			return true
		}
	} else if database.GetdatarowN[uint](false, logger.GetStringsMap(filedata.Cfgp.Useseries, logger.DBCountUnmatchedPath), pathv) >= 1 {
		return true
	}
	ok, _ := CheckExtensions(true, false, filedata.PathCfg, filepath.Ext(*pathv))

	//Check IgnoredPaths

	if ok && filedata.PathCfg.BlockedLen >= 1 {
		if logger.SlicesContainsPart2I(filedata.PathCfg.Blocked, *pathv) {
			return true
		}
	}
	return !ok
}

// MoveFile moves a file from one path to another. It handles checking extensions, renaming, setting permissions etc.
// It returns a bool indicating if the move was successful, and an error.
func MoveFile(file string, setpathcfg *config.PathsConfig, target, newname string, useother, usenil, usebuffercopy bool, chmodfolder, chmod string) MoveResponse {
	var retval MoveResponse
	if !CheckFileExist(file) {
		return retval
	}
	var ok, oknorename bool
	if usenil || setpathcfg == nil {
		ok = true
		oknorename = true
	} else {
		ext := filepath.Ext(file)
		ok, oknorename = CheckExtensions(!useother, useother, setpathcfg, ext)
	}

	if !ok {
		retval.Err = logger.ErrNotAllowed
		return retval
	}
	if newname == "" || oknorename {
		newname = filepath.Base(file)
	}
	if !strings.HasSuffix(newname, filepath.Ext(file)) {
		newname += filepath.Ext(file)
	}
	renamepath := filepath.Join(filepath.Dir(file), newname)
	newpath := filepath.Join(target, newname)
	if target != newpath && CheckFileExist(newpath) {
		//Remove Target to supress error
		_, _ = RemoveFile(newpath)
	}
	err := os.Rename(file, renamepath)
	if err != nil {
		retval.Err = err
		return retval
	}

	var uchmod, uchmodfolder fs.FileMode
	if chmod != "" && len(chmod) == 4 {
		uchmod = logger.StringToFileMode(chmod)
	}
	if chmodfolder != "" && len(chmodfolder) == 4 {
		uchmodfolder = logger.StringToFileMode(chmodfolder)
	}
	if usebuffercopy {
		if chmod != "" && len(chmod) == 4 {
			setchmod(renamepath, logger.StringToFileMode(chmod))
		}
		err = MoveFileDriveBuffer(renamepath, newpath, uchmod)
	} else {
		err = moveFileDrive(renamepath, newpath, uchmodfolder, uchmod)
	}
	if err != nil {
		retval.Err = err
		return retval
	}
	logger.LogDynamic("info", "File moved from", logger.NewLogField(logger.StrFile, file), logger.NewLogField("to", newpath))
	retval.MoveDone = true
	retval.NewPath = newpath
	return retval
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
		} else if pathcfg.AllowedVideoExtensionsLen >= 1 {
			ok = logger.SlicesContainsI(pathcfg.AllowedVideoExtensions, ext)
		}

		if !ok && pathcfg.AllowedVideoExtensionsNoRenameLen >= 1 {
			ok = logger.SlicesContainsI(pathcfg.AllowedVideoExtensionsNoRename, ext)
			if ok {
				oknorename = ok
			}
		}
	}
	if checkother {
		if pathcfg.AllowedOtherExtensionsLen == 0 {
			ok = true
			if ok {
				oknorename = ok
			}
		} else if pathcfg.AllowedOtherExtensionsLen >= 1 {
			ok = logger.SlicesContainsI(pathcfg.AllowedOtherExtensions, ext)
		}

		if !ok && pathcfg.AllowedOtherExtensionsNoRenameLen >= 1 {
			ok = logger.SlicesContainsI(pathcfg.AllowedOtherExtensionsNoRename, ext)
			if ok {
				oknorename = ok
			}
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
	if chmod != 0 {
		_ = f.Chmod(chmod)
	}
	f.Close()
}

// RemoveFile removes the file at the given path if it exists.
// It checks if the file exists first before removing.
// Returns a bool indicating if the file was removed, and an error if one occurred.
func RemoveFile(file string) (bool, error) {
	if !CheckFileExist(file) {
		return false, nil
	}
	_ = os.Chmod(file, 0777)
	err := os.Remove(file)
	if err != nil {
		return false, err
	}
	logger.LogDynamic("info", "File removed", logger.NewLogField(logger.StrFile, file))
	return true, nil
}

// CleanUpFolder walks the given folder path to calculate total size.
// It then compares total size to given cleanup threshold in MB.
// If folder size is less than threshold, folder is deleted.
// Returns any error encountered.
func CleanUpFolder(folder string, cleanupsizeMB int) error {
	// if cleanupsizeMB == 0 {
	// 	return nil
	// }
	if !CheckFileExist(folder) {
		return errors.New("cleanup folder not found")
	}
	var leftsize int64

	cleanupsizeByte := int64(cleanupsizeMB) * 1024 * 1024 //MB to Byte
	err := filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}
		if info.IsDir() {
			return nil
		}
		if cleanupsizeByte <= leftsize {
			return filepath.SkipAll
		}
		fsinfo, err := info.Info()
		if err == nil {
			leftsize += fsinfo.Size()
		}
		return nil
	})
	if err != nil {
		return err
	}

	logger.LogDynamic("debug", "Left size", logger.NewLogField("Size", leftsize))
	if cleanupsizeByte >= leftsize || leftsize == 0 {
		_ = filepath.WalkDir(folder, setchmodwalk)
		err := os.RemoveAll(folder)
		if err != nil {
			return err
		}
		logger.LogDynamic("info", "Folder removed", logger.NewLogField(logger.StrFile, folder))
	}
	return nil
}

// setchmodwalk changes the file permissions of the given path to 0777 recursively.
// It is used to ensure folders can be deleted properly.
func setchmodwalk(pathv string, _ fs.DirEntry, errw error) error {
	if errw != nil {
		return errw
	}
	_ = os.Chmod(pathv, 0777)
	return nil
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

		if _, err = destination.Write(buf[:n]); err != nil {
			return err
		}
	}
	_ = destination.Sync()
	// The copy was successful, so now delete the original file

	return nil
}

// MoveFileDriveBuffer moves the file at sourcePath to destPath using a buffer,
// sets the file permissions to chmod, and deletes the original source file
// after a successful copy. Returns any error.
func MoveFileDriveBuffer(sourcePath, destPath string, chmod fs.FileMode) error {
	err := moveFileDriveBuffer(sourcePath, destPath)
	if err != nil {
		return err
	}

	err = os.Remove(sourcePath)
	if err != nil {
		_ = os.Chmod(sourcePath, 0777)
		err = os.Remove(sourcePath)
		if err != nil {
			return errors.New("failed removing original file: " + err.Error())
		}
	}
	if chmod != 0 {
		_ = os.Chmod(destPath, chmod)
	}
	return nil
}

// moveFileDrive copies the file at sourcePath to destPath, setting the folder
// permissions to chmodfolder and file permissions to chmod after the copy.
// It handles deleting the original source file after a successful copy.
// Returns any errors from the copy or file deletions.
func moveFileDrive(sourcePath, destPath string, chmodfolder, chmod fs.FileMode) error {
	err := copyFile(sourcePath, destPath, false, chmodfolder, chmod)
	if err != nil {
		logger.LogDynamic("error", "Error copiing source", logger.NewLogField("sourcepath", sourcePath), logger.NewLogField("targetpath", destPath), logger.NewLogFieldValue(err))
		return err
	}

	if chmod != 0 {
		_ = os.Chmod(destPath, chmod)
	}
	// The copy was successful, so now delete the original file
	if os.Remove(sourcePath) != nil {
		_ = os.Chmod(sourcePath, 0777)
		err = os.Remove(sourcePath)
		if err != nil {
			logger.LogDynamic("error", "file could not be removed", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, sourcePath))
			return err
		}
	}
	return nil
}

// checkFile checks if the file at fpath exists and/or is a regular file,
// based on the provided checkexists and checkregular flags.
func checkFile(fpath string, checkexists bool, checkregular bool) bool {
	sfi, err := os.Stat(fpath)
	if checkexists {
		return !errors.Is(err, fs.ErrNotExist)
	}
	if checkregular {
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

// CheckFileSizeRecent checks if the file at fpath is recent and meets the
// minimum size requirements from the PathsConfig. It returns true if the file
// is too small or older than 2 minutes, which indicates it should be skipped.
func CheckFileSizeRecent(fpath string, sourcepathCfg *config.PathsConfig) bool {
	info, err := os.Stat(fpath)
	if err != nil {
		return false
	}
	if info.Size() < sourcepathCfg.MinVideoSizeByte {
		logger.LogDynamic("warn", "skipped - small files", logger.NewLogField(logger.StrFile, fpath))
		if sourcepathCfg.Name == "" {
			return true
		}
		ext := filepath.Ext(fpath)

		ok, oknorename := CheckExtensions(true, false, sourcepathCfg, ext)

		if ok || oknorename || (sourcepathCfg.AllowedVideoExtensionsLen == 0 && sourcepathCfg.AllowedVideoExtensionsNoRenameLen == 0) {
			_ = os.Chmod(fpath, 0777)
			err := os.Remove(fpath)
			if err != nil {
				logger.LogDynamic("error", "file could not be removed", logger.NewLogFieldValue(err), logger.NewLogField(logger.StrFile, fpath))
			} else {
				logger.LogDynamic("info", "File removed", logger.NewLogField(logger.StrFile, fpath))
			}
		}
		return true
	}
	bl := info.ModTime().After(logger.TimeGetNow().Add(-2 * time.Minute))
	if bl {
		logger.LogDynamic("error", "file modified too recently", logger.NewLogField(logger.StrFile, fpath))
	}
	return bl
}

// copyFile copies the contents of the file named src to the file named
// dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the
// contents of the source file. This function handles creating any missing
// directories in the destination path and setting permissions.
func copyFile(src, dst string, allowFileLink bool, chmodfolder, chmod fs.FileMode) error {
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

	dfi, err := os.Stat(dstAbs)
	if errors.Is(err, fs.ErrNotExist) {
		if chmodfolder == 0 {
			chmodfolder = 0777
		}
		err = os.MkdirAll(filepath.Dir(dst), chmodfolder)
		if err != nil {
			return err
		}
		if chmodfolder != 0 {
			_ = os.Chmod(filepath.Dir(dst), chmodfolder)
		}
	} else if err == nil {
		if !dfi.Mode().IsRegular() {
			return errors.New("CopyFile: non-regular destination file " + dfi.Name() + " - " + dfi.Mode().String())
		}
		//if os.SameFile(sfi, dfi) {
		//	return errors.New("same file")
		//}
	}
	// open dest file

	// if allowFileLink {
	// 	if err = os.Link(src, dst); err == nil {
	// 		return nil
	// 	}
	// }

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
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	// Sync and set permissions if needed
	if err = dstFile.Sync(); err != nil {
		return err
	}
	if chmod != 0 {
		if err = os.Chmod(dstAbs, chmod); err != nil {
			return err
		}
	}
	return nil
}

// Filterfile checks if a file path matches the allowed extensions and is not in the blocked paths.
// It takes the file path, a bool indicating if it is an "other" non-video file,
// and a PathsConfig struct containing the allowed/blocked extensions and paths.
// It returns a bool indicating if the path is allowed.
func Filterfile(pathv string, useother bool, setpathcfg *config.PathsConfig) bool {
	ok, _ := CheckExtensions(!useother, useother, setpathcfg, filepath.Ext(pathv))

	//Check IgnoredPaths

	if ok && setpathcfg.BlockedLen >= 1 {
		if logger.SlicesContainsPart2I(setpathcfg.Blocked, pathv) {
			return false
		}
	}
	return ok
}

// AppendCsv appends a line to the CSV file at fpath.
// It opens the file for appending, creating it if needed, with permissions 0777.
// It writes the line to the file with a newline separator.
// It handles logging and returning any errors.
func AppendCsv(fpath, line string) error {
	f, err := os.OpenFile(fpath,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0777)
	if err != nil {
		logger.LogDynamic("error", "Error opening csv to write", logger.NewLogFieldValue(err))
		return err
	}
	_, err = io.WriteString(f, line+"\n")
	_ = f.Sync()
	f.Close()
	if err != nil {
		logger.LogDynamic("error", "Error writing to csv", logger.NewLogFieldValue(err))
	} else {
		logger.LogDynamic("info", "csv written")
	}
	return nil
}

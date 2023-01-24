package scanner

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/karrick/godirwalk"
	"go.uber.org/zap"
)

const pathnotfound = "Path not found"

var errNoGeneral = errors.New("no general")
var errNoFiles = errors.New("no files")
var errNoPath = errors.New("no path")
var errNotFound = errors.New("not found")

func GetFilesDir(rootpath string, pathcfgstr string, useother bool) (*logger.InStringArrayStruct, error) {

	if pathcfgstr == "" {
		return nil, errNoGeneral
	}

	// if config.Cfg.General.UseGoDir {
	// 	return GetFilesGoDir(rootpath, pathcfgstr)
	// }

	if !CheckFileExist(rootpath) {
		logger.Log.GlobalLogger.Error(pathnotfound, zap.String("path", rootpath))
		return nil, errNoFiles
	}

	pathcfg := config.Cfg.Paths[pathcfgstr]
	defer pathcfg.Close()
	var allfiles logger.InStringArrayStruct

	var extlower string
	var lennorename, lenfiles int
	if useother {
		lennorename = len(pathcfg.AllowedOtherExtensionsIn.Arr)
		lenfiles = len(pathcfg.AllowedOtherExtensionsNoRenameIn.Arr)
	} else {
		lennorename = len(pathcfg.AllowedVideoExtensionsIn.Arr)
		lenfiles = len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr)
	}
	lenblock := len(pathcfg.BlockedLowerIn.Arr)
	//var pathdir string
	var ok bool

	var walkfunc = func(path string, info fs.DirEntry, errwalk error) error {
		if errwalk != nil {
			return errwalk
		}
		if info.IsDir() {
			return nil
		}
		extlower = filepath.Ext(path)
		if useother {
			ok = logger.InStringArray(extlower, &pathcfg.AllowedOtherExtensionsIn)
		} else {
			ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsIn)
		}

		if lennorename >= 1 && !ok {
			if useother {
				ok = logger.InStringArray(extlower, &pathcfg.AllowedOtherExtensionsNoRenameIn)
			} else {
				ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsNoRenameIn)
			}
		}

		if lennorename == 0 && lenfiles == 0 && !ok {
			ok = true
		}

		//Check IgnoredPaths
		if lenblock >= 1 && ok && logger.InStringArrayContainsCaseInSensitive(path, &pathcfg.BlockedLowerIn) {
			return nil
		}

		if ok {
			allfiles.Arr = append(allfiles.Arr, path)
		}
		return nil
	}

	return &allfiles, filepath.WalkDir(rootpath, walkfunc)
}

func FilterFilesDir(allfiles *logger.InStringArrayStruct, pathcfgstr string, useother bool, checkisdir bool) error {

	if pathcfgstr == "" {
		return errNoGeneral
	}
	pathcfg := config.Cfg.Paths[pathcfgstr]
	defer pathcfg.Close()

	filterfiles := logger.InStringArrayStruct{Arr: allfiles.Arr[:0]}
	var ok bool
	var extlower string
	var lennorename, lenfiles int
	if useother {
		lennorename = len(pathcfg.AllowedOtherExtensionsIn.Arr)
		lenfiles = len(pathcfg.AllowedOtherExtensionsNoRenameIn.Arr)
	} else {
		lennorename = len(pathcfg.AllowedVideoExtensionsIn.Arr)
		lenfiles = len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr)
	}
	lenblock := len(pathcfg.BlockedLowerIn.Arr)
	for idx := range allfiles.Arr {
		if checkisdir && getFileInfo(allfiles.Arr[idx]).IsDir() {
			continue
		}
		extlower = filepath.Ext(allfiles.Arr[idx])
		if useother {
			ok = logger.InStringArray(extlower, &pathcfg.AllowedOtherExtensionsIn)
		} else {
			ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsIn)
		}

		if lennorename >= 1 && !ok {
			if useother {
				ok = logger.InStringArray(extlower, &pathcfg.AllowedOtherExtensionsNoRenameIn)
			} else {
				ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsNoRenameIn)
			}
		}

		if lennorename == 0 && lenfiles == 0 && !ok {
			ok = true
		}

		//Check IgnoredPaths
		if lenblock >= 1 && ok && logger.InStringArrayContainsCaseInSensitive(allfiles.Arr[idx], &pathcfg.BlockedLowerIn) {
			continue
		}

		if ok {
			filterfiles.Arr = append(filterfiles.Arr, allfiles.Arr[idx])
		}
	}
	allfiles.Arr = filterfiles.Arr
	filterfiles.Close()
	return nil
}

func GetFilesDirAll(rootpath string, cachecount bool) (*logger.InStringArrayStruct, error) {
	if !CheckFileExist(rootpath) {
		logger.Log.GlobalLogger.Error(pathnotfound, zap.String("path", rootpath))
		return nil, errNoFiles
	}
	cnt, ok := logger.GlobalCounter[rootpath]

	var list logger.InStringArrayStruct
	if ok {
		list.Arr = make([]string, 0, cnt)
	}
	var walkfunc = func(path string, info fs.DirEntry, errwalk error) error {
		if errwalk != nil {
			return errwalk
		}
		if info.IsDir() {
			return nil
		}
		list.Arr = append(list.Arr, path)
		return nil
	}
	errwalk := filepath.WalkDir(rootpath, walkfunc)
	if errwalk != nil {
		logger.Log.GlobalLogger.Error("", zap.Error(errwalk))
	}
	if cachecount {
		logger.GlobalMu.Lock()
		logger.GlobalCounter[rootpath] = len(list.Arr)
		logger.GlobalMu.Unlock()
	}
	return &list, nil
}

func GetFilesWithDirAll(rootpath string) (*logger.InStringArrayStruct, error) {
	if !CheckFileExist(rootpath) {
		logger.Log.GlobalLogger.Error(pathnotfound, zap.String("path", rootpath))
		return nil, errNoFiles
	}
	var list logger.InStringArrayStruct
	var walkfunc = func(path string, info fs.DirEntry, errwalk error) error {
		if errwalk != nil {
			return errwalk
		}
		list.Arr = append(list.Arr, path)
		return nil
	}
	errwalk := filepath.WalkDir(rootpath, walkfunc)
	if errwalk != nil {
		logger.Log.GlobalLogger.Error("", zap.Error(errwalk))
	}
	return &list, nil
}

func GetFilesGoDir(rootpath string, pathcfgstr string) ([]string, error) {

	if pathcfgstr == "" {
		return []string{}, errNoPath
	}

	if !CheckFileExist(rootpath) {
		logger.Log.GlobalLogger.Error(pathnotfound, zap.String("path", rootpath))
		return []string{}, errNoPath
	}
	pathcfg := config.Cfg.Paths[pathcfgstr]
	defer pathcfg.Close()

	var list []string
	err := godirwalk.Walk(rootpath, &godirwalk.Options{
		Callback: func(osPathname string, de *godirwalk.Dirent) error {
			if de.IsDir() {
				return nil
			}
			extlower := filepath.Ext(osPathname)

			//Check Extension
			ok := logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsIn)

			if len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr) >= 1 && !ok {
				ok = logger.InStringArray(extlower, &pathcfg.AllowedVideoExtensionsNoRenameIn)
			}

			if len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr) == 0 && len(pathcfg.AllowedVideoExtensionsIn.Arr) == 0 && !ok {
				ok = true
			}

			//Check IgnoredPaths
			if len(pathcfg.BlockedLowerIn.Arr) >= 1 && ok {
				if logger.InStringArrayContainsCaseInSensitive(osPathname, &pathcfg.BlockedLowerIn) {
					ok = false
				}
			}

			if ok {
				list = append(list, osPathname)
			}

			return nil
		},
		ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
			return godirwalk.SkipNode
		},
		Unsorted: true, // set true for faster yet non-deterministic enumeration (see godoc)
	})
	if err != nil {
		logger.Log.GlobalLogger.Error("", zap.Error(err))
	}
	return list, nil
}

func getFolderSize(rootpath string) int64 {

	if CheckFileExist(rootpath) {
		logger.Log.GlobalLogger.Error(pathnotfound, zap.String("path", rootpath))
		return 0
	}
	var size int64
	if config.Cfg.General.UseGoDir {
		err := godirwalk.Walk(rootpath, &godirwalk.Options{
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				if de.IsDir() {
					return nil
				}
				size += GetFileSize(osPathname, false)
				return nil
			},
			ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
				return godirwalk.SkipNode
			},
			Unsorted: true, // set true for faster yet non-deterministic enumeration (see godoc)
		})
		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
		}
	} else {
		var walkfunc = func(path string, info fs.DirEntry, errwalk error) error {
			if info.IsDir() {
				return nil
			}
			size += GetFileSizeDirEntry(info)
			return nil
		}
		errwalk := filepath.WalkDir(rootpath, walkfunc)
		if errwalk != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(errwalk))
		}
	}
	return size
}

func MoveFile(file string, target string, newname string, filetypes *logger.InStringArrayStruct, filetypesNoRename *logger.InStringArrayStruct, usebuffercopy bool, chmod string) bool {
	if !CheckFileExist(file) {
		filetypes.Close()
		filetypesNoRename.Close()
		return false
	}
	var ok, oknorename bool
	if len(filetypes.Arr) == 0 {
		ok = true
		oknorename = true
	} else if len(filetypes.Arr) >= 1 && logger.InStringArray(filepath.Ext(file), filetypes) {
		ok = true
	}
	if !ok && len(filetypesNoRename.Arr) >= 1 && logger.InStringArray(filepath.Ext(file), filetypesNoRename) {
		ok = true
		oknorename = true
	}
	if !ok {
		filetypes.Close()
		filetypesNoRename.Close()
		return false
	}
	if newname == "" || oknorename {
		newname = filepath.Base(file)
	}
	newpath := filepath.Join(target, newname+filepath.Ext(file))
	if CheckFileExist(newpath) && target != newpath {
		//Remove Target to supress error
		RemoveFile(newpath)
	}
	if chmod != "" && len(chmod) == 4 {
		tempval, _ := strconv.ParseUint(chmod, 0, 32)
		Setchmod(file, fs.FileMode(uint32(tempval)))
	}
	if os.Rename(file, newpath) == nil {
		logger.Log.GlobalLogger.Debug("File moved from ", zap.Stringp("file", &file), zap.Stringp("to", &newpath))
		filetypes.Close()
		filetypesNoRename.Close()
		return true
	}

	var err error
	if usebuffercopy {
		err = moveFileDriveBuffer(file, newpath)
	} else {
		err = moveFileDrive(file, newpath)
	}
	if err == nil {
		logger.Log.GlobalLogger.Debug("File moved from ", zap.Stringp("file", &file), zap.Stringp("to", &newpath))
		filetypes.Close()
		filetypesNoRename.Close()
		return true
	}
	logger.Log.GlobalLogger.Error("File could not be moved", zap.Stringp("file", &file), zap.Error(err))
	filetypes.Close()
	filetypesNoRename.Close()
	return false
}

func Setchmod(file string, chmod fs.FileMode) {
	f, err := os.Open(file)
	if err != nil {
		f.Close()
		return
	}
	f.Chmod(chmod)
	f.Close()
}
func RemoveFiles(val string, pathcfgstr string) {

	if pathcfgstr == "" {
		return
	}
	if !CheckFileExist(val) {
		return
	}
	pathcfg := config.Cfg.Paths[pathcfgstr]
	defer pathcfg.Close()

	var ok, oknorename bool
	if len(pathcfg.AllowedVideoExtensionsIn.Arr) >= 1 {
		ok = logger.InStringArray(filepath.Ext(val), &pathcfg.AllowedVideoExtensionsIn)
	}
	if len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr) >= 1 && !ok {
		ok = logger.InStringArray(filepath.Ext(val), &pathcfg.AllowedVideoExtensionsNoRenameIn)
		if ok {
			oknorename = true
		}
	}
	if ok || oknorename || (len(pathcfg.AllowedVideoExtensionsIn.Arr) == 0 && len(pathcfg.AllowedVideoExtensionsNoRenameIn.Arr) == 0) {
		err := os.Remove(val)
		if err != nil {
			logger.Log.GlobalLogger.Error("File could not be removed", zap.Stringp("file", &val), zap.Error(err))
		} else {
			logger.Log.GlobalLogger.Debug("File removed", zap.Stringp("file", &val))
		}
	}
}

func RemoveFile(file string) error {
	if CheckFileExist(file) {
		err := os.Remove(file)
		if err != nil {
			logger.Log.GlobalLogger.Error("File could not be removed", zap.Stringp("file", &file), zap.Error(err))
		} else {
			logger.Log.GlobalLogger.Debug("File removed", zap.Stringp("file", &file))
		}
		return err
	}
	return errNotFound
}

func CleanUpFolder(folder string, CleanupsizeMB int) {
	if !CheckFileExist(folder) {
		return
	}
	if CleanupsizeMB == 0 {
		return
	}
	leftsize := getFolderSize(folder)
	logger.Log.GlobalLogger.Debug("Left size", zap.Int("Size", int(leftsize/1024/1024)))
	if CleanupsizeMB >= int(leftsize/1024/1024) {
		err := os.RemoveAll(folder)
		if err == nil {
			logger.Log.GlobalLogger.Debug("Folder removed", zap.Stringp("folder", &folder))
		} else {
			logger.Log.GlobalLogger.Error("Folder could not be removed", zap.Stringp("folder", &folder), zap.Error(err))
		}
	}
}

func checkRegular(path string) bool {
	return getFileInfo(path).Mode().IsRegular()
}
func moveFileDriveBuffer(sourcePath, destPath string) error {
	bufferkb := 1024

	buffersize := config.Cfg.General.MoveBufferSizeKB
	if buffersize != 0 {
		bufferkb = buffersize
	}

	if !checkRegular(sourcePath) {
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

	buf := make([]byte, int64(bufferkb)*1024)
	var n int
	for {
		n, err = source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err = destination.Write(buf[:n]); err != nil {
			return err
		}
	}
	destination.Sync()
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	buf = nil
	if err != nil {
		return errors.New("failed removing original file: " + err.Error())
	}
	return nil
}

func MoveFileDriveBuffer(sourcePath, destPath string) error {
	return moveFileDriveBuffer(sourcePath, destPath)
}

func moveFileDrive(sourcePath, destPath string) error {
	if !CheckFileExist(sourcePath) {
		return errNotFound
	}
	err := copyFile(sourcePath, destPath, false)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error copiing source", zap.Stringp("sourcepath", &sourcePath), zap.Stringp("targetpath", &destPath), zap.Error(err))
		return err
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		logger.Log.GlobalLogger.Error("Error removing source", zap.Stringp("path", &sourcePath), zap.Error(err))
		return err
	}
	return nil
}

func MoveFileDrive(sourcePath, destPath string) error {
	return moveFileDrive(sourcePath, destPath)
}

// AbsolutePath converts a path (relative or absolute) into an absolute one.
// Supports '~' notation for $HOME directory of the current user.
func absolutePath(path string) (string, error) {
	return filepath.Abs(path)
}

func CheckFileExist(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func getFileInfo(path string) fs.FileInfo {
	info, _ := os.Stat(path)
	return info
}

func GetFileSize(path string, checkpath bool) int64 {
	if checkpath {
		if !CheckFileExist(path) {
			return 0
		}
	}
	return getFileInfo(path).Size()
}
func GetFileSizeDirEntry(info fs.DirEntry) int64 {
	fsinfo, errinfo := info.Info()
	if errinfo == nil {
		return fsinfo.Size()
	}
	return 0
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, attempt to create a hard link
// between the two files. If that fails, copy the file contents from src to dst.
// Creates any missing directories. Supports '~' notation for $HOME directory of the current user.
func copyFile(src, dst string, allowFileLink bool) error {
	var srcAbs string
	var err error
	srcAbs, err = absolutePath(src)
	if err != nil {
		return err
	}
	var dstAbs string
	dstAbs, err = absolutePath(dst)
	if err != nil {
		return err
	}

	// open source file
	var sfi fs.FileInfo
	sfi, err = os.Stat(srcAbs)

	if err != nil {
		return err
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return errors.New("CopyFile: non-regular source file " + sfi.Name() + " (" + sfi.Mode().String() + ")")
	}

	// open dest file
	var dfi fs.FileInfo
	dfi, err = os.Stat(dstAbs)

	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		// file doesn't exist
		err = os.MkdirAll(filepath.Dir(dst), 0755)
		if err != nil {
			return err
		}

	} else {
		if !dfi.Mode().IsRegular() {
			return errors.New("CopyFile: non-regular destination file " + dfi.Name() + " (" + dfi.Mode().String() + ")")
		}
		if os.SameFile(sfi, dfi) {
			return errors.New("same file")
		}
	}
	if allowFileLink {
		if err = os.Link(src, dst); err == nil {
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
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return err
	}
	dstFile.Sync()
	return nil
}

func GetSubFolders(sourcepath string) ([]string, error) {
	files, err := os.ReadDir(sourcepath)
	if err != nil {
		return []string{}, errNotFound
	}
	var folders []string
	// cnt, ok := logger.GlobalCounter[sourcepath]
	// if ok {
	// 	folders = logger.GrowSliceBy(folders, cnt)
	// }
	for idxfile := range files {
		if files[idxfile].IsDir() {
			folders = append(folders, filepath.Join(sourcepath, files[idxfile].Name()))
		}
	}
	files = nil
	//logger.GlobalCounter[sourcepath] = len(folders)
	return folders, nil
}

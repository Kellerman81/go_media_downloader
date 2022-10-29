package scanner

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/karrick/godirwalk"
	"go.uber.org/zap"
)

var errNoGeneral error = errors.New("no general")
var errNoFiles error = errors.New("no files")
var errNoPath error = errors.New("no path")
var errNotFound error = errors.New("not found")

func GetFilesDir(rootpath string, pathcfgstr string, useother bool) (*logger.InStringArrayStruct, error) {

	if pathcfgstr == "" {
		return nil, errNoGeneral
	}

	// if config.Cfg.General.UseGoDir {
	// 	return GetFilesGoDir(rootpath, pathcfgstr)
	// }

	if CheckFileExist(rootpath) {
		files, err := GetFilesDirAll(rootpath)
		if err != nil {
			return nil, errNoFiles
		}
		defer files.Close()
		return FilterFilesDir(files, pathcfgstr, useother, false)
	} else {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", rootpath))
	}
	return nil, errNoFiles
}

func FilterFilesDir(allfiles *logger.InStringArrayStruct, pathcfgstr string, useother bool, checkisdir bool) (*logger.InStringArrayStruct, error) {

	if pathcfgstr == "" {
		return nil, errNoGeneral
	}

	allowedVideoExtensions := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedVideoExtensions}
	defer allowedVideoExtensions.Close()

	allowedVideoExtensionsNoRename := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedVideoExtensionsNoRename}
	defer allowedVideoExtensionsNoRename.Close()

	if useother {
		allowedVideoExtensions = &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedOtherExtensions}
		allowedVideoExtensionsNoRename = &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedOtherExtensionsNoRename}
	}

	blocked := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].BlockedLower}
	defer blocked.Close()

	filterfiles := &logger.InStringArrayStruct{Arr: allfiles.Arr[:0]}
	for idx := range allfiles.Arr {
		if checkisdir {
			if GetFileInfo(allfiles.Arr[idx]).IsDir() {
				continue
			}
		}
		extlower := filepath.Ext(allfiles.Arr[idx])
		ok := logger.InStringArray(extlower, allowedVideoExtensions)
		if len(allowedVideoExtensionsNoRename.Arr) >= 1 && !ok {
			ok = logger.InStringArray(extlower, allowedVideoExtensionsNoRename)
		}

		if len(allowedVideoExtensionsNoRename.Arr) == 0 && len(allowedVideoExtensions.Arr) == 0 && !ok {
			ok = true
		}

		//Check IgnoredPaths
		if len(blocked.Arr) >= 1 && ok {
			pathdir, _ := filepath.Split(allfiles.Arr[idx])
			if logger.InStringArrayContainsCaseInSensitive(pathdir, blocked) {
				ok = false
			}
		}

		if ok {
			filterfiles.Arr = append(filterfiles.Arr, allfiles.Arr[idx])
		}
	}
	return filterfiles, nil
}

func GetFilesDirAll(rootpath string) (*logger.InStringArrayStruct, error) {
	if CheckFileExist(rootpath) {
		cnt, ok := logger.GlobalCounter[rootpath]

		list := &logger.InStringArrayStruct{} // = make([]string, 0, 20)
		if ok {
			list.Arr = make([]string, 0, cnt)
		}
		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			list.Arr = append(list.Arr, path)
			return nil
		})
		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
		}
		logger.GlobalCounter[rootpath] = len(list.Arr)
		return list, nil
	} else {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", rootpath))
	}
	return nil, errNoFiles
}

func GetFilesWithDirAll(rootpath string) (*logger.InStringArrayStruct, error) {
	if CheckFileExist(rootpath) {
		list := &logger.InStringArrayStruct{} // = make([]string, 0, 20)
		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			list.Arr = append(list.Arr, path)
			return nil
		})
		if err != nil {
			logger.Log.GlobalLogger.Error("", zap.Error(err))
		}
		return list, nil
	} else {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", rootpath))
	}
	return nil, errNoFiles
}

func GetFilesGoDir(rootpath string, pathcfgstr string) ([]string, error) {

	if pathcfgstr == "" {
		return []string{}, errNoPath
	}

	if CheckFileExist(rootpath) {
		allowedVideoExtensions := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedVideoExtensions}
		defer allowedVideoExtensions.Close()

		allowedVideoExtensionsNoRename := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedVideoExtensionsNoRename}
		defer allowedVideoExtensionsNoRename.Close()

		blocked := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].BlockedLower}
		defer blocked.Close()

		var list []string //= make([]string, 0, 20)
		err := godirwalk.Walk(rootpath, &godirwalk.Options{
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				if de.IsDir() {
					return nil
				}
				extlower := filepath.Ext(osPathname)

				//Check Extension
				ok := logger.InStringArray(extlower, allowedVideoExtensions)

				if len(allowedVideoExtensionsNoRename.Arr) >= 1 && !ok {
					ok = logger.InStringArray(extlower, allowedVideoExtensionsNoRename)
				}

				if len(allowedVideoExtensionsNoRename.Arr) == 0 && len(allowedVideoExtensions.Arr) == 0 && !ok {
					ok = true
				}

				//Check IgnoredPaths
				if len(blocked.Arr) >= 1 && ok {
					pathdir, _ := filepath.Split(osPathname)
					if logger.InStringArrayContainsCaseInSensitive(pathdir, blocked) {
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
	} else {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", rootpath))
		return []string{}, errNoPath
	}
}

func getFolderSize(rootpath string) int64 {
	var size int64

	if CheckFileExist(rootpath) {
		if config.Cfg.General.UseGoDir {
			err := godirwalk.Walk(rootpath, &godirwalk.Options{
				Callback: func(osPathname string, de *godirwalk.Dirent) error {
					if de.IsDir() {
						return nil
					}
					size += GetFileSize(osPathname)
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
			var fsinfo fs.FileInfo

			var errinfo error
			err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
				if info.IsDir() {
					return nil
				}
				fsinfo, errinfo = info.Info()
				if errinfo == nil {
					size += fsinfo.Size()
				}
				return nil
			})
			if err != nil {
				logger.Log.GlobalLogger.Error("", zap.Error(err))
			}
		}
	} else {
		logger.Log.GlobalLogger.Error("Path not found", zap.String("path", rootpath))
	}
	return size
}

func MoveFile(file string, target string, newname string, filetypes *logger.InStringArrayStruct, filetypesNoRename *logger.InStringArrayStruct, usebuffercopy bool) bool {
	defer filetypes.Close()
	defer filetypesNoRename.Close()
	if CheckFileExist(file) {
		ok := false
		oknorename := false
		if len(filetypes.Arr) == 0 {
			ok = true
			oknorename = true
		} else {
			if len(filetypes.Arr) >= 1 {
				if logger.InStringArray(filepath.Ext(file), filetypes) {
					ok = true
				}
			}
		}
		if !ok {
			if len(filetypesNoRename.Arr) >= 1 {
				if logger.InStringArray(filepath.Ext(file), filetypesNoRename) {
					ok = true
				}
				if ok {
					oknorename = true
				}
			}
		}
		if ok {
			if newname == "" || oknorename {
				newname = filepath.Base(file)
			}
			newpath := filepath.Join(target, newname+filepath.Ext(file))
			if CheckFileExist(newpath) {
				if target != newpath {
					//Remove Target to supress error
					RemoveFile(newpath)
				}
			}
			err := os.Rename(file, newpath)
			if err != nil {
				if usebuffercopy {
					err = moveFileDriveBuffer(file, newpath)
				} else {
					err = moveFileDrive(file, newpath)
				}
				if err != nil {
					logger.Log.GlobalLogger.Error("File could not be moved", zap.String("file", file), zap.Error(err))
				} else {
					logger.Log.GlobalLogger.Debug("File moved from ", zap.String("file", file), zap.String("to", newpath))
					return true
				}
			} else {
				logger.Log.GlobalLogger.Debug("File moved from ", zap.String("file", file), zap.String("to", newpath))
				return true
			}

		}
	}
	return false
}

func RemoveFiles(val string, pathcfgstr string) {

	if pathcfgstr == "" {
		return
	}
	allowedVideoExtensions := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedVideoExtensions}
	defer allowedVideoExtensions.Close()

	allowedVideoExtensionsNoRename := &logger.InStringArrayStruct{Arr: config.Cfg.Paths[pathcfgstr].AllowedVideoExtensionsNoRename}
	defer allowedVideoExtensionsNoRename.Close()

	ok := false
	oknorename := false
	if len(allowedVideoExtensions.Arr) >= 1 {
		ok = logger.InStringArray(filepath.Ext(val), allowedVideoExtensions)
	}
	if len(allowedVideoExtensionsNoRename.Arr) >= 1 && !ok {
		ok = logger.InStringArray(filepath.Ext(val), allowedVideoExtensionsNoRename)
		if ok {
			oknorename = true
		}
	}
	if ok || oknorename || (len(allowedVideoExtensions.Arr) == 0 && len(allowedVideoExtensionsNoRename.Arr) == 0) {
		if CheckFileExist(val) {
			err := os.Remove(val)
			if err != nil {
				logger.Log.GlobalLogger.Error("File could not be removed", zap.String("file", val), zap.Error(err))
			} else {
				logger.Log.GlobalLogger.Debug("File removed", zap.String("file", val))
			}
		}
	}
}

func RemoveFile(file string) error {
	if CheckFileExist(file) {
		err := os.Remove(file)
		if err != nil {
			logger.Log.GlobalLogger.Error("File could not be removed", zap.String("file", file), zap.Error(err))
		} else {
			logger.Log.GlobalLogger.Debug("File removed", zap.String("file", file))
		}
		return err
	} else {
		return errNotFound
	}
}

func CleanUpFolder(folder string, CleanupsizeMB int) {
	if CheckFileExist(folder) {
		filesleft, err := GetFilesDirAll(folder)
		if err == nil {
			defer filesleft.Close()
			logger.Log.GlobalLogger.Debug("Left files", zap.Strings("files", filesleft.Arr))
			if CleanupsizeMB >= 1 {
				leftsize := getFolderSize(folder)
				logger.Log.GlobalLogger.Debug("Left size", zap.Int("Size", int(leftsize/1024/1024)))
				if CleanupsizeMB >= int(leftsize/1024/1024) {
					err := os.RemoveAll(folder)
					if err == nil {
						logger.Log.GlobalLogger.Debug("Folder removed", zap.String("folder", folder))
					} else {
						logger.Log.GlobalLogger.Error("Folder could not be removed", zap.String("folder", folder), zap.Error(err))
					}
				}
			}
		}
	}
}

func CheckRegular(path string) bool {
	return GetFileInfo(path).Mode().IsRegular()
}
func moveFileDriveBuffer(sourcePath, destPath string) error {
	bufferkb := 1024

	buffersize := config.Cfg.General.MoveBufferSizeKB
	if buffersize != 0 {
		bufferkb = buffersize
	}

	if !CheckRegular(sourcePath) {
		return errors.New(sourcePath + " is not a regular file")
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	if CheckFileExist(destPath) {
		return errors.New(destPath + " already exists")
	}

	destination, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	buf := make([]byte, int64(bufferkb)*1024)
	defer logger.ClearVar(&buf)
	for {
		n, err := source.Read(buf)
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
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return errors.New("failed removing original file: " + err.Error())
	}
	return nil
}

func MoveFileDriveBuffer(sourcePath, destPath string) error {
	bufferkb := 1024

	buffersize := config.Cfg.General.MoveBufferSizeKB
	if buffersize != 0 {
		bufferkb = buffersize
	}

	if !CheckRegular(sourcePath) {
		return errors.New(sourcePath + " is not a regular file")
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	if CheckFileExist(destPath) {
		return errors.New(destPath + " already exists")
	}

	destination, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	buf := make([]byte, int64(bufferkb)*1024)
	defer logger.ClearVar(&buf)
	for {
		n, err := source.Read(buf)
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
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return errors.New("failed removing original file: " + err.Error())
	}
	return nil
}

func moveFileDrive(sourcePath, destPath string) error {
	if CheckFileExist(sourcePath) {
		err := copyFile(sourcePath, destPath, false)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error copiing source", zap.String("sourcepath", sourcePath), zap.String("targetpath", destPath), zap.Error(err))
			return err
		}
	} else {
		return errNotFound
	}
	if CheckFileExist(sourcePath) {
		// The copy was successful, so now delete the original file
		err := os.Remove(sourcePath)
		if err != nil {
			logger.Log.GlobalLogger.Error("Error removing source", zap.String("path", sourcePath), zap.Error(err))
			return err
		}
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

func GetFileInfo(path string) fs.FileInfo {
	info, _ := os.Stat(path)
	return info
}

func GetFileSize(path string) int64 {
	return GetFileInfo(path).Size()
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, attempt to create a hard link
// between the two files. If that fails, copy the file contents from src to dst.
// Creates any missing directories. Supports '~' notation for $HOME directory of the current user.
func copyFile(src, dst string, allowFileLink bool) (err error) {
	srcAbs, err := absolutePath(src)
	if err != nil {
		return err
	}
	dstAbs, err := absolutePath(dst)
	if err != nil {
		return err
	}

	// open source file
	sfi, err := os.Stat(srcAbs)

	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return errors.New("CopyFile: non-regular source file " + sfi.Name() + " (" + sfi.Mode().String() + ")")
	}

	// open dest file
	dfi, err := os.Stat(dstAbs)

	if err != nil {
		if !os.IsNotExist(err) {
			return
		}
		// file doesn't exist
		err := os.MkdirAll(filepath.Dir(dst), 0755)
		if err != nil {
			return err
		}

	} else {
		if !(dfi.Mode().IsRegular()) {
			return errors.New("CopyFile: non-regular destination file " + dfi.Name() + " (" + dfi.Mode().String() + ")")
		}
		if os.SameFile(sfi, dfi) {
			return
		}
	}
	if allowFileLink {
		if err = os.Link(src, dst); err == nil {
			return
		}
	}
	// Open the source file for reading
	srcFile, err := os.Open(src)
	if err != nil {
		return
	}
	defer srcFile.Close()

	// Open the destination file for writing
	dstFile, err := os.Create(dst)
	if err != nil {
		return
	}
	defer dstFile.Close()
	// Return any errors that result from closing the destination file
	// Will return nil if no errors occurred

	// Copy the contents of the source file into the destination files
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return
	}
	dstFile.Sync()
	return
}

func GetSubFolders(sourcepath string) ([]string, error) {
	files, err := os.ReadDir(sourcepath)
	if err == nil {
		var folders []string //= make([]string, 0, 20)
		cnt, ok := logger.GlobalCounter[sourcepath]
		if ok {
			folders = make([]string, 0, cnt)
		}
		for idxfile := range files {
			if files[idxfile].IsDir() {
				folders = append(folders, filepath.Join(sourcepath, files[idxfile].Name()))
			}
		}
		logger.GlobalCounter[sourcepath] = len(folders)
		return folders, nil
	}
	return []string{}, errNotFound
}

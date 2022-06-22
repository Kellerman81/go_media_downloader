package scanner

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/karrick/godirwalk"
)

func GetFilesDirCounter(rootpath string, pathcfgstr string) ([]string, error) {
	pathcfg := config.PathsConfig{}
	if len(pathcfgstr) > 1 {
		pathcfg = config.ConfigGet(pathcfgstr).Data.(config.PathsConfig)
	}

	if !config.ConfigCheck("general") {
		return nil, errors.New("no general")
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.UseGoDir {
		return GetFilesGoDir(rootpath, pathcfgstr)
	}

	if CheckFileExist(rootpath) {
		counter := 0
		filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if info.IsDir() {
				return nil
			}
			counter += 1

			return nil
		})
		list := make([]string, 0, counter)
		defer logger.ClearVar(&list)

		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if info.IsDir() {
				return nil
			}

			//Check Extension
			ok := false
			for idx := range pathcfg.AllowedVideoExtensions {
				if strings.EqualFold(pathcfg.AllowedVideoExtensions[idx], filepath.Ext(path)) {
					ok = true
					break
				}
			}

			if pathcfg.AllowedVideoExtensionsNoRename != nil {
				if len(pathcfg.AllowedVideoExtensionsNoRename) >= 1 && !ok {
					for idx := range pathcfg.AllowedVideoExtensionsNoRename {
						if strings.EqualFold(pathcfg.AllowedVideoExtensionsNoRename[idx], filepath.Ext(path)) {
							ok = true
							break
						}
					}
				}
			}

			if pathcfg.AllowedVideoExtensionsNoRename != nil && pathcfg.AllowedVideoExtensions != nil {
				if len(pathcfg.AllowedVideoExtensionsNoRename) == 0 && len(pathcfg.AllowedVideoExtensions) == 0 {
					ok = true
				}
			}

			//Check IgnoredPaths
			if pathcfg.Blocked != nil {
				if len(pathcfg.Blocked) >= 1 && ok {
					pathdir, _ := filepath.Split(path)
					for idx := range pathcfg.Blocked {
						if strings.Contains(strings.ToLower(pathdir), strings.ToLower(pathcfg.Blocked[idx])) {
							ok = false
							break
						}
					}
				}
			}

			if ok {
				list = append(list, path)
			}

			return nil

		})
		if err != nil {
			logger.Log.Error(err)
		}
		return list, nil
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return nil, errors.New("no files")
}

func GetFilesDir(rootpath string, pathcfgstr string) ([]string, error) {
	pathcfg := config.PathsConfig{}
	if len(pathcfgstr) > 1 {
		pathcfg = config.ConfigGet(pathcfgstr).Data.(config.PathsConfig)
	}
	if !config.ConfigCheck("general") {
		return nil, errors.New("no general")
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.UseGoDir {
		return GetFilesGoDir(rootpath, pathcfgstr)
	}

	if CheckFileExist(rootpath) {
		var list []string
		defer logger.ClearVar(&list)
		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			//Check Extension
			ok := false
			for idx := range pathcfg.AllowedVideoExtensions {
				if strings.EqualFold(pathcfg.AllowedVideoExtensions[idx], filepath.Ext(path)) {
					ok = true
					break
				}
			}

			if pathcfg.AllowedVideoExtensionsNoRename != nil && !ok {
				if len(pathcfg.AllowedVideoExtensionsNoRename) >= 1 && !ok {
					for idx := range pathcfg.AllowedVideoExtensionsNoRename {
						if strings.EqualFold(pathcfg.AllowedVideoExtensionsNoRename[idx], filepath.Ext(path)) {
							ok = true
							break
						}
					}
				}
			}

			if pathcfg.AllowedVideoExtensionsNoRename != nil && pathcfg.AllowedVideoExtensions != nil && !ok {
				if len(pathcfg.AllowedVideoExtensionsNoRename) == 0 && len(pathcfg.AllowedVideoExtensions) == 0 && !ok {
					ok = true
				}
			}

			//Check IgnoredPaths
			if pathcfg.Blocked != nil && ok {
				if len(pathcfg.Blocked) >= 1 {
					pathdir, _ := filepath.Split(path)
					for idx := range pathcfg.Blocked {
						if strings.Contains(strings.ToLower(pathdir), strings.ToLower(pathcfg.Blocked[idx])) {
							ok = false
							break
						}
					}
				}
			}

			if ok {
				list = append(list, path)
			}

			return nil

		})
		if err != nil {
			logger.Log.Error(err)
		}
		return list, nil
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return nil, errors.New("no files")
}

func GetFilesDirNewOld(configTemplate string, rootpath string, pathcfgstr string) (filesWanted logger.StringSet, filesFound logger.StringSet, err error) {
	pathcfg := config.PathsConfig{}
	if len(pathcfgstr) > 1 {
		pathcfg = config.ConfigGet(pathcfgstr).Data.(config.PathsConfig)
	}
	if !config.ConfigCheck("general") {
		return logger.StringSet{}, logger.StringSet{}, errors.New("no general")
	}

	rootpathlike := rootpath + "%"
	var filesHave logger.StringSet
	defer filesHave.Clear()
	if strings.HasPrefix(configTemplate, "movie") {
		filesHave = logger.NewStringSetExactSize(database.CountRowsStaticNoError("Select count(id) FROM movie_files where location like ?", rootpathlike))
		for _, file := range database.QueryStaticStringArray("Select DISTINCT location FROM movie_files where location like ?", "Select count(id) FROM movie_files where location like ?", rootpathlike) {
			if file == "" {
				continue
			}
			filesHave.Add(file)
		}
	} else {
		filesHave = logger.NewStringSetExactSize(database.CountRowsStaticNoError("Select count(id) FROM serie_episode_files where location like ?", rootpathlike))
		for _, file := range database.QueryStaticStringArray("Select DISTINCT location FROM serie_episode_files where location like ?", "Select count(id) FROM serie_episode_files where location like ?", rootpathlike) {
			if file == "" {
				continue
			}
			filesHave.Add(file)
		}
	}

	mapfiletypes := logger.NewStringSetExactSize(len(pathcfg.AllowedVideoExtensions))
	defer mapfiletypes.Clear()
	for idx := range pathcfg.AllowedVideoExtensions {
		mapfiletypes.Add(strings.ToLower(pathcfg.AllowedVideoExtensions[idx]))
	}

	mapfiletypesNoRename := logger.NewStringSetExactSize(len(pathcfg.AllowedVideoExtensionsNoRename))
	defer mapfiletypesNoRename.Clear()
	for idx := range pathcfg.AllowedVideoExtensionsNoRename {
		mapfiletypesNoRename.Add(strings.ToLower(pathcfg.AllowedVideoExtensionsNoRename[idx]))
	}

	filesWanted = logger.NewStringSet()
	defer filesWanted.Clear()

	filesFound = logger.NewStringSet()
	defer filesFound.Clear()

	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.UseGoDir {
		//return GetFilesGoDir(rootpath, filetypes, filetypesNoRename, ignoredpaths)
	}

	if CheckFileExist(rootpath) {
		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			//Check Extension
			ok := false
			if len(pathcfg.AllowedVideoExtensions) >= 1 {
				if mapfiletypes.Contains(strings.ToLower(filepath.Ext(path))) {
					ok = true
				}
			}

			if pathcfg.AllowedVideoExtensionsNoRename != nil && !ok {
				if len(pathcfg.AllowedVideoExtensionsNoRename) >= 1 && !ok {
					if mapfiletypesNoRename.Contains(strings.ToLower(filepath.Ext(path))) {
						ok = true
					}
				}
			}

			if pathcfg.AllowedVideoExtensionsNoRename != nil && pathcfg.AllowedVideoExtensions != nil && !ok {
				if len(pathcfg.AllowedVideoExtensionsNoRename) == 0 && len(pathcfg.AllowedVideoExtensions) == 0 && !ok {
					ok = true
				}
			}

			//Check IgnoredPaths
			if pathcfg.Blocked != nil && ok {
				if len(pathcfg.Blocked) >= 1 {
					pathdir, _ := filepath.Split(path)
					for idx := range pathcfg.Blocked {
						if strings.Contains(strings.ToLower(pathdir), strings.ToLower(pathcfg.Blocked[idx])) {
							ok = false
							break
						}
					}
				}
			}

			if ok {
				if filesHave.Contains(path) {
					filesFound.Add(path)
				} else {
					filesWanted.Add(path)
				}
			}

			return nil

		})
		if err != nil {
			logger.Log.Error(err)
		}

		return filesFound, filesWanted, nil
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return logger.StringSet{}, logger.StringSet{}, errors.New("no files")
}

func GetFilesDirAll(rootpath string) ([]string, error) {

	if !config.ConfigCheck("general") {
		return nil, errors.New("no general")
	}

	if CheckFileExist(rootpath) {
		var list []string
		defer logger.ClearVar(&list)
		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			list = append(list, path)
			return nil
		})
		if err != nil {
			logger.Log.Error(err)
		}
		return list, nil
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return nil, errors.New("no files")
}

func GetFilesGoDir(rootpath string, pathcfgstr string) ([]string, error) {
	pathcfg := config.PathsConfig{}
	if len(pathcfgstr) > 1 {
		pathcfg = config.ConfigGet(pathcfgstr).Data.(config.PathsConfig)
	}
	var list []string
	defer logger.ClearVar(&list)

	if CheckFileExist(rootpath) {
		err := godirwalk.Walk(rootpath, &godirwalk.Options{
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				if de.IsDir() {
					return nil
				}

				//Check Extension
				ok := false
				for idx2 := range pathcfg.AllowedVideoExtensions {
					if strings.EqualFold(pathcfg.AllowedVideoExtensions[idx2], filepath.Ext(osPathname)) {
						ok = true
						break
					}
				}

				if pathcfg.AllowedVideoExtensionsNoRename != nil && !ok {
					if len(pathcfg.AllowedVideoExtensionsNoRename) >= 1 && !ok {
						for idx2 := range pathcfg.AllowedVideoExtensionsNoRename {
							if strings.EqualFold(pathcfg.AllowedVideoExtensionsNoRename[idx2], filepath.Ext(osPathname)) {
								ok = true
								break
							}
						}
					}
				}

				if pathcfg.AllowedVideoExtensionsNoRename != nil && pathcfg.AllowedVideoExtensions != nil && !ok {
					if len(pathcfg.AllowedVideoExtensionsNoRename) == 0 && len(pathcfg.AllowedVideoExtensions) == 0 && !ok {
						ok = true
					}
				}

				//Check IgnoredPaths
				if pathcfg.Blocked != nil && ok {
					if len(pathcfg.Blocked) >= 1 && ok {
						pathdir, _ := filepath.Split(osPathname)
						for idx2 := range pathcfg.Blocked {
							if strings.Contains(strings.ToLower(pathdir), strings.ToLower(pathcfg.Blocked[idx2])) {
								ok = false
								break
							}
						}
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
			logger.Log.Error(err)
		}
	} else {
		logger.Log.Error("Path not found: ", rootpath)
		return nil, errors.New("no path")
	}
	return list, nil
}

func getFolderSize(rootpath string) int64 {
	var size int64

	if !config.ConfigCheck("general") {
		return 0
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if CheckFileExist(rootpath) {
		if cfg_general.UseGoDir {
			var info fs.FileInfo
			defer logger.ClearVar(&info)
			var errinfo error
			err := godirwalk.Walk(rootpath, &godirwalk.Options{
				Callback: func(osPathname string, de *godirwalk.Dirent) error {
					if de.IsDir() {
						return nil
					}
					info, errinfo = os.Stat(osPathname)
					if errinfo == nil {
						size += info.Size()
					}
					return nil
				},
				ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
					return godirwalk.SkipNode
				},
				Unsorted: true, // set true for faster yet non-deterministic enumeration (see godoc)
			})
			if err != nil {
				logger.Log.Error(err)
			}
		} else {
			var fsinfo fs.FileInfo
			defer logger.ClearVar(&fsinfo)
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
				logger.Log.Error(err)
			}
		}
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return size
}

func getFileSize(file string) int64 {
	var size int64
	if CheckFileExist(file) {
		info, err := os.Stat(file)
		defer logger.ClearVar(&info)
		if err == nil {
			size += info.Size()
		}
	} else {
		logger.Log.Error("File not found: ", file)
	}
	return size
}

func createFolderWithSubfolders(path string, security uint32) error {
	if security == 0 {
		security = 0777
	}
	err := os.MkdirAll(path, os.FileMode(security))
	return err
}

func MoveFiles(files []string, target string, newname string, filetypes []string, filetypesNoRename []string, usebuffercopy bool) (bool, int) {
	defer logger.ClearVar(&files)
	defer logger.ClearVar(&filetypes)
	defer logger.ClearVar(&filetypesNoRename)
	moved := 0
	moveok := false
	if files == nil {
		return moveok, moved
	}
	for idx := range files {
		if CheckFileExist(files[idx]) {
			ok := false
			oknorename := false
			if filetypes == nil {
				ok = true
				oknorename = true
			} else {
				if len(filetypes) == 0 {
					ok = true
					oknorename = true
				} else {
					if len(filetypes) >= 1 {
						for idx2 := range filetypes {
							if strings.EqualFold(filetypes[idx2], filepath.Ext(files[idx])) {
								ok = true
								break
							}
						}
					}
				}
			}
			if !ok {
				if filetypesNoRename != nil {
					if len(filetypesNoRename) >= 1 {
						for idx2 := range filetypesNoRename {
							if strings.EqualFold(filetypesNoRename[idx2], filepath.Ext(files[idx])) {
								ok = true
								break
							}
						}
						if ok {
							oknorename = true
						}
					}
				}
			}
			if ok {
				if newname == "" || oknorename {
					newname = filepath.Base(files[idx])
				}
				newpath := filepath.Join(target, newname+filepath.Ext(files[idx]))
				if CheckFileExist(newpath) {
					if target != newpath {
						//Remove Target to supress error
						RemoveFile(newpath)
					}
				}
				err := os.Rename(files[idx], newpath)
				if err != nil {
					if usebuffercopy {
						err = moveFileDriveBuffer(files[idx], newpath)
					} else {
						err = moveFileDrive(files[idx], newpath)
					}
					if err != nil {
						logger.Log.Error("File could not be moved: ", files[idx], " Error: ", err)
					} else {
						logger.Log.Debug("File moved from ", files[idx], " to ", newpath)
						moved = moved + 1
					}
				} else {
					logger.Log.Debug("File moved from ", files[idx], " to ", newpath)
					moved = moved + 1
				}

			}
		} else {
			logger.Log.Error("File not found: ", files[idx])
		}
	}
	if len(files) == moved {
		moveok = true
	}
	return moveok, moved
}

func RemoveFiles(val string, pathcfgstr string) {
	pathcfg := config.PathsConfig{}
	if len(pathcfgstr) > 1 {
		pathcfg = config.ConfigGet(pathcfgstr).Data.(config.PathsConfig)
	}
	ok := false
	oknorename := false
	if pathcfg.AllowedVideoExtensions != nil {
		if len(pathcfg.AllowedVideoExtensions) >= 1 {
			for idxpath := range pathcfg.AllowedVideoExtensions {
				if strings.EqualFold(pathcfg.AllowedVideoExtensions[idxpath], filepath.Ext(val)) {
					ok = true
					return
				}
			}
		}
	}
	if pathcfg.AllowedVideoExtensionsNoRename != nil {
		if len(pathcfg.AllowedVideoExtensionsNoRename) >= 1 && !ok {
			for idxpath := range pathcfg.AllowedVideoExtensionsNoRename {
				if strings.EqualFold(pathcfg.AllowedVideoExtensionsNoRename[idxpath], filepath.Ext(val)) {
					ok = true
					break
				}
			}
			if ok {
				oknorename = true
			}
		}
	}
	if ok || oknorename || len(pathcfg.AllowedVideoExtensions) == 0 {
		if CheckFileExist(val) {
			err := os.Remove(val)
			if err != nil {
				logger.Log.Error("File could not be removed: ", val, " Error: ", err)
			} else {
				logger.Log.Debug("File removed: ", val)
			}
		} else {
			logger.Log.Error("File not found: ", val)
		}
	}
}

func RemoveFile(file string) error {
	var err error
	if CheckFileExist(file) {
		err := os.Remove(file)
		if err != nil {
			logger.Log.Error("File could not be removed: ", file, " Error: ", err)
		} else {
			logger.Log.Debug("File removed: ", file)
		}
	} else {
		logger.Log.Error("File not found: ", file)
	}
	return err
}

func CleanUpFolder(folder string, CleanupsizeMB int) {
	if CheckFileExist(folder) {
		filesleft, err := GetFilesDir(folder, "")
		if err == nil {
			defer logger.ClearVar(&filesleft)
			logger.Log.Debug("Left files: ", filesleft)
			if CleanupsizeMB >= 1 {
				leftsize := getFolderSize(folder)
				logger.Log.Debug("Left size: ", int(leftsize/1024/1024))
				if CleanupsizeMB >= int(leftsize/1024/1024) {
					err := os.RemoveAll(folder)
					if err == nil {
						logger.Log.Debug("Folder removed: ", folder)
					} else {
						logger.Log.Error("Folder could not be removed: ", folder, " Error: ", err)
					}
				}
			}
		}
	}
}

func moveFileDriveReadWrite(sourcePath, destPath string) error {

	//High Ram Usage !!!
	input, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		logger.Log.Error(err)
		return err
	}
	defer logger.ClearVar(&input)

	err = ioutil.WriteFile(destPath, input, 0644)
	if err != nil {
		logger.Log.Error("Error creating", destPath)
		logger.Log.Error(err)
		return err
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		logger.Log.Error("Error removing source", sourcePath)
		return err
	}
	return nil
}

func moveFileDriveBuffer(sourcePath, destPath string) error {
	if !config.ConfigCheck("general") {
		return errors.New("missing config")
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	bufferkb := 1024
	if cfg_general.MoveBufferSizeKB != 0 {
		bufferkb = cfg_general.MoveBufferSizeKB
	}

	sourceFileStat, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	defer logger.ClearVar(&sourceFileStat)

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", sourcePath)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	_, err = os.Stat(destPath)
	if err == nil {
		return fmt.Errorf("file %s already exists", destPath)
	}

	destination, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	buf := make([]byte, int64(bufferkb)*1024)
	defer logger.ClearVar(&buf)
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
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("failed removing original file: %s", err)
	}
	return nil
}

func MoveFileDriveBuffer(sourcePath, destPath string) error {
	if !config.ConfigCheck("general") {
		return errors.New("missing config")
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	bufferkb := 1024
	if cfg_general.MoveBufferSizeKB != 0 {
		bufferkb = cfg_general.MoveBufferSizeKB
	}

	sourceFileStat, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	defer logger.ClearVar(&sourceFileStat)

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", sourcePath)
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer source.Close()

	_, err = os.Stat(destPath)
	if err == nil {
		return fmt.Errorf("file %s already exists", destPath)
	}

	destination, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destination.Close()

	buf := make([]byte, int64(bufferkb)*1024)
	defer logger.ClearVar(&buf)
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
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return fmt.Errorf("failed removing original file: %s", err)
	}
	return nil
}

func moveFileDrive(sourcePath, destPath string) error {
	if CheckFileExist(sourcePath) {
		err := copyFile(sourcePath, destPath, false)
		if err != nil {
			logger.Log.Error("Error copiing source", sourcePath, destPath, err)
			return err
		}
	} else {
		return errors.New("source doesnt exist")
	}
	if CheckFileExist(sourcePath) {
		// The copy was successful, so now delete the original file
		err := os.Remove(sourcePath)
		if err != nil {
			logger.Log.Error("Error removing source", sourcePath, err)
			return err
		}
	}
	return nil
}

func MoveFileDrive(sourcePath, destPath string) error {
	if CheckFileExist(sourcePath) {
		err := copyFile(sourcePath, destPath, false)
		if err != nil {
			logger.Log.Error("Error copiing source", sourcePath, destPath, err)
			return err
		}
	} else {
		return errors.New("source doesnt exist")
	}
	if CheckFileExist(sourcePath) {
		// The copy was successful, so now delete the original file
		err := os.Remove(sourcePath)
		if err != nil {
			logger.Log.Error("Error removing source", sourcePath, err)
			return err
		}
	}
	return nil
}

// AbsolutePath converts a path (relative or absolute) into an absolute one.
// Supports '~' notation for $HOME directory of the current user.
func absolutePath(path string) (string, error) {
	homeReplaced := path
	return filepath.Abs(homeReplaced)
}

func CheckFileExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
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
	defer logger.ClearVar(&sfi)

	if err != nil {
		return
	}
	if !sfi.Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
	}

	// open dest file
	dfi, err := os.Stat(dstAbs)
	defer logger.ClearVar(&dfi)

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
			return fmt.Errorf("CopyFile: non-regular destination file %s (%q)", dfi.Name(), dfi.Mode().String())
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
	defer logger.ClearVar(&files)
	if err == nil {
		var folders []string
		defer logger.ClearVar(&folders)
		for idxfile := range files {
			if files[idxfile].IsDir() {
				folders = append(folders, filepath.Join(sourcepath, files[idxfile].Name()))
			}
		}
		return folders, nil
	}
	return nil, errors.New("empty")
}

func getSubFiles(sourcepath string) ([]string, error) {
	files, err := os.ReadDir(sourcepath)
	defer logger.ClearVar(&files)
	if err == nil {
		folders := make([]string, 0, len(files))
		defer logger.ClearVar(&folders)
		for idxfile := range files {
			if !files[idxfile].IsDir() {
				folders = append(folders, filepath.Join(sourcepath, files[idxfile].Name()))
			}
		}
		return folders, nil
	}
	return nil, errors.New("empty")
}

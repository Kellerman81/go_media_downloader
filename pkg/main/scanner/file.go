package scanner

import (
	"bufio"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/apiexternal"
	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/logger"
)

func GetFilesExtCount(rootpath *string, ext string) int {
	i := 0
	// filepath.WalkDir(filepath.Dir(*rootpath), func(file string, _ fs.DirEntry, err error) error {
	// 	if err != nil {
	// 		return err
	// 	}
	// 	if strings.EqualFold(filepath.Ext(file), ext) {
	// 		i++
	// 	}
	// 	return nil
	// })
	_ = WalkdirProcess(*rootpath, true, func(file *string, _ *fs.DirEntry) error {
		if strings.EqualFold(filepath.Ext(*file), ext) {
			i++
		}
		return nil
	})
	return i
}

func getFolderSize(rootpath string) (int64, error) {
	var size int64
	err := filepath.Walk(rootpath, func(_ string, info fs.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		size += info.Size()
		return nil
	})
	return size, err
}

func WalkdirProcess(rootpath string, usedir bool, fun func(path *string, info *fs.DirEntry) error) error {
	return filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if usedir && info.IsDir() {
			return nil
		}
		return fun(&path, &info)
	})
}

func MoveFile(file string, pathcfg string, target string, newname string, useother bool, usenil bool, usebuffercopy bool, chmod string) (bool, error) {
	if !CheckFileExist(&file) {
		return false, nil
	}
	var ok, oknorename bool
	if usenil {
		ok = true
		oknorename = true
	} else {
		ext := filepath.Ext(file)
		if !useother {
			if len(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions) == 0 {
				ok = true
				oknorename = true
			} else if len(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions) >= 1 {
				for idx := range config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions {
					if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions[idx], ext) {
						ok = true
						break
					}
				}
			}

			if !ok && len(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename) >= 1 {
				for idx := range config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename {
					if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename[idx], ext) {
						ok = true
						oknorename = true
						break
					}
				}
			}
		} else {
			if len(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions) == 0 {
				ok = true
				oknorename = true
			} else if len(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions) >= 1 {
				for idx := range config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions {
					if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions[idx], ext) {
						ok = true
						break
					}
				}
			}

			if !ok && len(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename) >= 1 {
				for idx := range config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename {
					if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename[idx], ext) {
						ok = true
						oknorename = true
						break
					}
				}
			}
		}
	}

	if !ok {
		return false, logger.ErrNotAllowed
	}
	if newname == "" || oknorename {
		newname = filepath.Base(file)
	}
	if !strings.HasSuffix(newname, filepath.Ext(file)) {
		newname = newname + filepath.Ext(file)
	}
	renamepath := logger.PathJoin(filepath.Dir(file), newname)
	newpath := logger.PathJoin(target, newname)
	if CheckFileExist(&newpath) && target != newpath {
		//Remove Target to supress error
		RemoveFile(newpath)
	}
	if chmod != "" && len(chmod) == 4 {
		Setchmod(file, fs.FileMode(logger.StringToUint32(chmod)))
	}
	err := os.Rename(file, renamepath)
	if err != nil {
		return false, errors.Join(err, errors.New("scanner move file rename"))
	}

	if usebuffercopy {
		err = moveFileDriveBuffer(renamepath, newpath)
	} else {
		err = moveFileDrive(renamepath, newpath)
	}
	if err != nil {
		return false, errors.Join(err, errors.New("scanner move file run"))
	}
	logger.Log.Debug().Str(logger.StrFile, file).Str("to", newpath).Msg("File moved from")
	return true, nil
}

func Setchmod(file string, chmod fs.FileMode) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	f.Chmod(chmod)
	f.Close()
}
func RemoveFiles(val string, pathtemplate string) {

	if config.SettingsPath["path_"+pathtemplate].Name == "" {
		return
	}
	if !CheckFileExist(&val) {
		return
	}
	ext := filepath.Ext(val)
	var ok, oknorename bool
	if len(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensions) >= 1 {
		for idxi := range config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensions {
			if strings.EqualFold(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensions[idxi], ext) {
				ok = true
				break
			}
		}
		//ok = logger.ContainsStringsIS(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensions, ext)
	}
	if len(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensionsNoRename) >= 1 && !ok {
		for idxi := range config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensionsNoRename {
			if strings.EqualFold(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensionsNoRename[idxi], ext) {
				ok = true
				break
			}
		}
		//ok = logger.ContainsStringsIS(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensionsNoRename, ext)
		if ok {
			oknorename = true
		}
	}
	if ok || oknorename || (len(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensions) == 0 && len(config.SettingsPath["path_"+pathtemplate].AllowedVideoExtensionsNoRename) == 0) {
		err := os.Remove(val)
		if err != nil {
			logger.Log.Error().Err(err).Str(logger.StrPath, val).Msg("file could not be removed")
		} else {
			logger.Log.Debug().Str(logger.StrFile, val).Msg("File removed")
		}
	}
}

func RemoveFile(file string) (bool, error) {
	if !CheckFileExist(&file) {
		return false, nil
	}
	err := os.Remove(file)
	if err != nil {
		return false, err
	}
	logger.Log.Debug().Str(logger.StrFile, file).Msg("File removed")
	return true, nil
}

func RenameFileSimple(file string, filenew string) error {
	err := os.Rename(file, filenew)
	if err == nil {
		logger.Log.Debug().Str(logger.StrFile, file).Msg("File renamed")
	}
	return err
}

func CleanUpFolder(folder string, cleanupsizeMB int) error {
	if cleanupsizeMB == 0 {
		return nil
	}
	if !CheckFileExist(&folder) {
		return errors.New("cleanup folder not found")
	}
	leftsize, err := getFolderSize(folder)
	if err != nil {
		return err
	}
	leftmb := int(leftsize / 1024 / 1024)
	logger.Log.Debug().Int("Size", leftmb).Msg("Left size")
	if cleanupsizeMB >= leftmb || leftmb == 0 {
		err := os.RemoveAll(folder)
		if err == nil {
			logger.Log.Debug().Str(logger.StrPath, folder).Msg("Folder removed")
		}
		return err
	}
	return nil
}

func checkRegular(path *string) bool {
	info, _ := filestat(path, false)
	defer logger.ClearVar(info)
	return (info).Mode().IsRegular()
}
func moveFileDriveBuffer(sourcePath, destPath string) error {
	bufferkb := 1024

	buffersize := config.SettingsGeneral.MoveBufferSizeKB
	if buffersize != 0 {
		bufferkb = buffersize
	}

	if !checkRegular(&sourcePath) {
		return errors.New(sourcePath + " is not a regular file")
	}

	if CheckFileExist(&destPath) {
		return errors.New(destPath + " already exists")
	}

	source, err := os.Open(sourcePath)
	if err != nil {
		return errors.Join(err, errors.New("move source open"))
	}
	defer source.Close()

	destination, err := os.Create(destPath)
	if err != nil {
		return errors.Join(err, errors.New("move destionation create"))
	}
	defer destination.Close()

	for {
		buf := make([]byte, int64(bufferkb)*1024)
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err = destination.Write(buf[:n]); err != nil {
			return errors.Join(err, errors.New("move destination write"))
		}
	}
	destination.Sync()
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		return errors.New("failed removing original file: " + err.Error())
	}
	return nil
}

func MoveFileDriveBuffer(sourcePath, destPath string) error {
	return moveFileDriveBuffer(sourcePath, destPath)
}

func moveFileDrive(sourcePath, destPath string) error {
	err := copyFile(sourcePath, destPath, false)
	if err != nil {
		logger.Log.Error().Str("sourcepath", sourcePath).Str("targetpath", destPath).Err(err).Msg("Error copiing source")
		return err
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		logger.Log.Error().Err(err).Str(logger.StrPath, sourcePath).Msg("file could not be removed")
		return err
	}
	return nil
}

func MoveFileDrive(sourcePath, destPath string) error {
	return moveFileDrive(sourcePath, destPath)
}

func filestat(path *string, onlyerr bool) (fs.FileInfo, error) {
	a, e := os.Stat(*path)
	if onlyerr {
		a = nil
		return nil, e
	}
	return a, e
}
func CheckFileExist(path *string) bool {
	_, err := filestat(path, true)
	return !errors.Is(err, fs.ErrNotExist)
}

func GetFileSize(path *string) int64 {
	info, err := filestat(path, false)
	if err != nil {
		return 0
	}
	defer logger.ClearVar(info)
	return (info).Size()
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, attempt to create a hard link
// between the two files. If that fails, copy the file contents from src to dst.
// Creates any missing directories. Supports '~' notation for $HOME directory of the current user.
func copyFile(src, dst string, allowFileLink bool) error {
	srcAbs, err := filepath.Abs(src)
	if err != nil {
		return errors.Join(err, errors.New("move file abs src"))
	}
	dstAbs, err := filepath.Abs(dst)
	if err != nil {
		return errors.Join(err, errors.New("move file abs dst"))
	}

	// Open the source file for reading
	srcFile, err := os.Open(src)
	if err != nil {
		return errors.Join(err, errors.New("move file open"))
	}
	defer srcFile.Close()

	// open source file
	sfi, err := filestat(&srcAbs, false)

	if err != nil {
		return errors.Join(err, errors.New("move file stat src"))
	}
	if !(sfi).Mode().IsRegular() {
		// cannot copy non-regular files (e.g., directories,
		// symlinks, devices, etc.)
		return errors.New("CopyFile: non-regular source file " + (sfi).Name() + " (" + (sfi).Mode().String() + ")")
	}

	dfi, err := filestat(&dstAbs, false)
	if errors.Is(err, fs.ErrNotExist) {
		err = os.MkdirAll(filepath.Dir(dst), (sfi).Mode())
		if err != nil {
			return errors.Join(err, errors.New("move file dir"))
		}
		dfi, err = filestat(&dstAbs, false)
	}
	// open dest file

	if err == nil {
		if !(dfi).Mode().IsRegular() {
			return errors.New("CopyFile: non-regular destination file " + (dfi).Name() + " (" + (dfi).Mode().String() + ")")
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

	// Open the destination file for writing
	dstFile, err := os.Create(dst)
	if err != nil {
		return errors.Join(err, errors.New("move file create"))
	}
	defer dstFile.Close()
	// Return any errors that result from closing the destination file
	// Will return nil if no errors occurred

	// Copy the contents of the source file into the destination files
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return errors.Join(err, errors.New("move file copy"))
	}
	dstFile.Sync()
	return os.Chmod(dst, (sfi).Mode())
}

func Filterfile(path *string, useother bool, pathcfg string) bool {
	var ok bool
	extlower := filepath.Ext(*path)
	if useother {
		for idx := range config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions {
			if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions[idx], extlower) {
				ok = true
				break
			}
		}
	} else {
		for idx := range config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions {
			if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions[idx], extlower) {
				ok = true
				break
			}
		}
	}

	if !ok {
		if useother && len(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename) >= 1 {

			for idx := range config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename {
				if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename[idx], extlower) {
					ok = true
					break
				}
			}
		} else if !useother && len(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename) >= 1 {

			for idx := range config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename {
				if strings.EqualFold(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename[idx], extlower) {
					ok = true
					break
				}
			}
		}
	}
	if !ok {
		if useother {
			if len(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensionsNoRename) == 0 && len(config.SettingsPath["path_"+pathcfg].AllowedOtherExtensions) == 0 && !ok {
				ok = true
			}
		} else {
			if len(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensionsNoRename) == 0 && len(config.SettingsPath["path_"+pathcfg].AllowedVideoExtensions) == 0 && !ok {
				ok = true
			}
		}
	}

	//Check IgnoredPaths

	if len(config.SettingsPath["path_"+pathcfg].Blocked) >= 1 && ok {
		for idx := range config.SettingsPath["path_"+pathcfg].Blocked {
			if logger.ContainsI(*path, config.SettingsPath["path_"+pathcfg].Blocked[idx]) {
				return false
			}
		}
	}
	return ok
}

func AppendCsv(path string, line string) error {
	f, err := os.OpenFile(path,
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Logerror(err, "Error opening csv to write")
		return err
	}
	_, err = io.WriteString(f, line+"\n")
	f.Sync()
	f.Close()
	if err != nil {
		logger.Logerror(err, "Error writing to csv")
	} else {
		logger.Log.Info().Msg("csv written")
	}
	return nil
}

// DownloadFile will download a url to a local file. It's efficient because it will
// write as it downloads and not load the whole file into memory.
func DownloadFile(saveIn string, fileprefix string, filename string, url *string, allowhtml bool) error {
	resp, err := apiexternal.WebClient.Do(logger.HTTPGetRequest(url))
	if err != nil || resp == nil {
		return err
	}
	defer resp.Body.Close()

	// Create the file
	if filename == "" {
		filename = path.Base(resp.Request.URL.String())
	}
	if fileprefix != "" {
		filename = fileprefix + filename
	}
	filepath := path.Join(saveIn, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	out.Sync()
	out.Close()
	if err != nil {
		return err
	}

	if !allowhtml {
		tst, err := os.Open(filepath)
		if err != nil {
			return err
		}
		defer tst.Close()
		scanner := bufio.NewScanner(tst)
		if scanner.Scan() {
			str := strings.TrimSpace(scanner.Text())
			if len(str) >= 5 {
				if strings.EqualFold(str[:5], "<html") {
					RemoveFile(filepath)
					return errors.New("contained html")
				}
			}
			if scanner.Scan() {
				str = strings.TrimSpace(scanner.Text())
				if len(str) >= 5 {
					if strings.EqualFold(str[:5], "<html") {
						RemoveFile(filepath)
						return errors.New("contained html")
					}
				}
			}
		}
	}
	return err
}

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
	"time"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/karrick/godirwalk"
)

func GetFilesDir(rootpath string, filetypes []string, filetypesNoRename []string, ignoredpaths []string) []string {

	if !config.ConfigCheck("general") {
		return []string{}
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if cfg_general.UseGoDir {
		return GetFilesGoDir(rootpath, filetypes, filetypesNoRename, ignoredpaths)
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
		err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
			if info.IsDir() {
				return nil
			}

			//Check Extension
			ok := false
			if len(filetypes) >= 1 {
				for idx := range filetypes {
					if strings.EqualFold(filetypes[idx], filepath.Ext(path)) {
						ok = true
						break
					}
				}
			}

			if len(filetypesNoRename) >= 1 && !ok {
				for idx := range filetypesNoRename {
					if strings.EqualFold(filetypesNoRename[idx], filepath.Ext(path)) {
						ok = true
						break
					}
				}
			}

			if len(filetypesNoRename) == 0 && len(filetypes) == 0 {
				ok = true
			}

			//Check IgnoredPaths
			if len(ignoredpaths) >= 1 && ok {
				pathdir, _ := filepath.Split(path)
				for idxignore := range ignoredpaths {
					if strings.Contains(strings.ToLower(pathdir), strings.ToLower(ignoredpaths[idxignore])) {
						ok = false
						break
					}
				}
			}

			if ok {
				list = append(list, path)
			}

			return nil

		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
		return list
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return []string{}
}

func GetFilesGoDir(rootpath string, filetypes []string, filetypesNoRename []string, ignoredpaths []string) []string {
	var list []string

	if CheckFileExist(rootpath) {
		err := godirwalk.Walk(rootpath, &godirwalk.Options{
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				if de.IsDir() {
					return nil
				}

				//Check Extension
				ok := false
				if len(filetypes) >= 1 {
					for idx := range filetypes {
						if strings.EqualFold(filetypes[idx], filepath.Ext(osPathname)) {
							ok = true
							break
						}
					}
				}

				if len(filetypesNoRename) >= 1 && !ok {
					for idx := range filetypesNoRename {
						if strings.EqualFold(filetypesNoRename[idx], filepath.Ext(osPathname)) {
							ok = true
							break
						}
					}
				}

				if len(filetypesNoRename) == 0 && len(filetypes) == 0 {
					ok = true
				}

				//Check IgnoredPaths
				if len(ignoredpaths) >= 1 && ok {
					path, _ := filepath.Split(osPathname)
					for idxignore := range ignoredpaths {
						if strings.Contains(strings.ToLower(path), strings.ToLower(ignoredpaths[idxignore])) {
							ok = false
							break
						}
					}
				}

				if ok {
					list = append(list, osPathname)
				}

				return nil
			},
			ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
				//fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
				return godirwalk.SkipNode
			},
			Unsorted: true, // set true for faster yet non-deterministic enumeration (see godoc)
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", err)
		}
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return list
}

func getFolderSize(rootpath string) int64 {
	var size int64

	if !config.ConfigCheck("general") {
		return 0
	}
	cfg_general := config.ConfigGet("general").Data.(config.GeneralConfig)

	if CheckFileExist(rootpath) {
		if cfg_general.UseGoDir {
			err := godirwalk.Walk(rootpath, &godirwalk.Options{
				Callback: func(osPathname string, de *godirwalk.Dirent) error {
					if de.IsDir() {
						return nil
					}
					info, errinfo := os.Stat(osPathname)
					if errinfo == nil {
						size += info.Size()
					}
					info = nil
					return nil
				},
				ErrorCallback: func(osPathname string, err error) godirwalk.ErrorAction {
					//fmt.Fprintf(os.Stderr, "%s: %s\n", progname, err)
					return godirwalk.SkipNode
				},
				Unsorted: true, // set true for faster yet non-deterministic enumeration (see godoc)
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
			}
		} else {
			err := filepath.WalkDir(rootpath, func(path string, info fs.DirEntry, err error) error {
				if info.IsDir() {
					return nil
				}
				fsinfo, errinfo := info.Info()
				if errinfo == nil {
					size += fsinfo.Size()
				}
				return nil
			})
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s\n", err)
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
		if err == nil {
			size += info.Size()
		}
		info = nil
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
	moved := 0
	moveok := false

	for idxfile := range files {
		if CheckFileExist(files[idxfile]) {
			var ok bool
			var oknorename bool
			if len(filetypes) == 0 {
				ok = true
				oknorename = true
			} else {
				if len(filetypes) >= 1 {
					for idx := range filetypes {
						if strings.EqualFold(filetypes[idx], filepath.Ext(files[idxfile])) {
							ok = true
							break
						}
					}
				}

				if len(filetypesNoRename) >= 1 && !ok {
					for idx := range filetypesNoRename {
						if strings.EqualFold(filetypesNoRename[idx], filepath.Ext(files[idxfile])) {
							ok = true
							oknorename = true
							break
						}
					}
				}
			}
			if ok {
				if newname == "" || oknorename {
					newname = filepath.Base(files[idxfile])
				}
				newpath := filepath.Join(target, newname+filepath.Ext(files[idxfile]))
				if CheckFileExist(newpath) {
					if target != newpath {
						//Remove Target to supress error
						RemoveFile(newpath)
					}
				}
				err1 := os.Rename(files[idxfile], newpath)
				if err1 != nil {
					var err error
					if usebuffercopy {
						err = moveFileDriveBuffer(files[idxfile], newpath)
					} else {
						err = moveFileDrive(files[idxfile], newpath)
					}
					if err != nil {
						logger.Log.Error("File could not be moved: ", files[idxfile], " Error: ", err)
					} else {
						logger.Log.Debug("File moved from ", files[idxfile], " to ", newpath)
						moved = moved + 1
					}
				} else {
					logger.Log.Debug("File moved from ", files[idxfile], " to ", newpath)
					moved = moved + 1
				}

			}
		} else {
			logger.Log.Error("File not found: ", files[idxfile])
		}
	}
	if len(files) == moved {
		moveok = true
	}
	return moveok, moved
}

func RemoveFiles(files []string, filetypes []string, filetypesNoRename []string) {

	for idxfile := range files {
		var ok bool
		var oknorename bool
		if len(filetypes) >= 1 {
			for idx := range filetypes {
				if strings.EqualFold(filetypes[idx], filepath.Ext(files[idxfile])) {
					ok = true
					break
				}
			}
		}

		if len(filetypesNoRename) >= 1 && !ok {
			for idx := range filetypesNoRename {
				if strings.EqualFold(filetypesNoRename[idx], filepath.Ext(files[idxfile])) {
					ok = true
					oknorename = true
					break
				}
			}
		}
		if ok || oknorename || len(filetypes) == 0 {
			if CheckFileExist(files[idxfile]) {
				err := os.Remove(files[idxfile])
				if err != nil {
					logger.Log.Error("File could not be removed: ", files[idxfile], " Error: ", err)
				} else {
					logger.Log.Debug("File removed: ", files[idxfile])
				}
			} else {
				logger.Log.Error("File not found: ", files[idxfile])
			}
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

func CheckDisallowed(folder string, disallowed []string, removefolder bool) bool {
	emptyarr := []string{}
	var disallow bool
	if len(disallowed) == 0 {
		disallow = false
		return disallow
	}
	logger.Log.Debug("Check disallowed")
	if CheckFileExist(folder) {
		filesleft := GetFilesDir(folder, emptyarr, emptyarr, emptyarr)
		for idxfile := range filesleft {
			for idxdisallow := range disallowed {
				if disallowed[idxdisallow] == "" {
					continue
				}
				if strings.Contains(strings.ToLower(filesleft[idxfile]), strings.ToLower(disallowed[idxdisallow])) {
					logger.Log.Warning(filesleft[idxfile], " is not allowd in the path!")
					disallow = true
					if removefolder {
						CleanUpFolder(folder, 80000)
					}
					return disallow
				}
			}
		}
	}
	return disallow
}
func CleanUpFolder(folder string, CleanupsizeMB int) {
	emptyarr := []string{}
	if CheckFileExist(folder) {
		filesleft := GetFilesDir(folder, emptyarr, emptyarr, emptyarr)
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

func checkfilespathlist(array []database.Dbfiles, typeof string, find string, configTemplate string, listname string, listConfig string) bool {
	for idx := range array {
		if array[idx].Location == find {
			var getlist interface{}
			var errget error
			if typeof != "movies" {
				counter, err := database.CountRowsStatic("select count(id) FROM serie_file_unmatcheds WHERE filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", find, listname, time.Now().Add(time.Hour*-12))
				if err == nil {
					if counter >= 1 {
						return true
					}
				}
				getlist, errget = database.QueryColumnStatic("select listname FROM series WHERE id = ?", array[idx].ID)
				if errget != nil {
					continue
				}
			} else {
				counter, err := database.CountRowsStatic("select count(id) FROM movie_file_unmatcheds WHERE filepath = ? and listname = ? and (last_checked > ? or last_checked is null)", find, listname, time.Now().Add(time.Hour*-12))
				if err == nil {
					if counter >= 1 {
						return true
					}
				}
				getlist, errget = database.QueryColumnStatic("select listname FROM movies WHERE id = ?", array[idx].ID)
				if errget != nil {
					continue
				}
			}
			if getlist == nil {
				continue
			}
			var getlistname string = getlist.(string)
			if getlistname == "" {
				continue
			}
			if strings.EqualFold(getlistname, listname) {
				return true
			}
			list := config.ConfigGetMediaListConfig(configTemplate, listConfig)
			for idxignore := range list.Replace_map_lists {
				if strings.EqualFold(getlistname, list.Replace_map_lists[idxignore]) {
					return true
				}
			}
			for idxignore := range list.Ignore_map_lists {
				if strings.EqualFold(getlistname, list.Ignore_map_lists[idxignore]) {
					return true
				}
			}
		}
	}
	return false
}

func GetFilesAdded(files []string, listname string, configTemplate string, listConfig string) []string {
	listentries := files[:0]

	filesdb, fileerr := database.QueryDbfiles("Select location, movie_id as id from movie_files", "Select count(id) from movie_files")
	if len(filesdb) == 0 {
		logger.Log.Error("File Struct error", fileerr)
		return listentries
	}
	for idxfile := range files {
		if !checkfilespathlist(filesdb, "movies", files[idxfile], configTemplate, listname, listConfig) {
			logger.Log.Debug("File added to list - not found", files[idxfile], " ", listname)
			listentries = append(listentries, files[idxfile])
		}
	}
	return listentries
}
func GetFilesSeriesAdded(files []string, configTemplate string, listname string, listConfig string) []string {
	listentries := files[:0]

	filesdb, fileerr := database.QueryDbfiles("Select location, serie_id as id from serie_episode_files", "Select count(id) from serie_episode_files")
	if len(filesdb) == 0 {
		logger.Log.Error("File Struct error", fileerr)
		return listentries
	}
	for idxfile := range files {
		if !checkfilespathlist(filesdb, "series", files[idxfile], configTemplate, listname, listConfig) {
			logger.Log.Debug("File added to list - not found", files[idxfile], " ", listname)
			listentries = append(listentries, files[idxfile])
		}
	}
	return listentries
}

func GetFilesRemoved(listname string) []string {
	moviefile, _ := database.QueryStaticColumnsOneString("Select location from movie_files where movie_id in (Select id from movies where listname=?)", "Select count(id) from movie_files where movie_id in (Select id from movies where listname=?)", listname)
	var listentries []string
	for idxmovie := range moviefile {
		if !CheckFileExist(moviefile[idxmovie].Str) {
			listentries = append(listentries, moviefile[idxmovie].Str)
		}
	}
	return listentries
}

func GetFilesSeriesRemoved(listname string) []string {
	seriefile, _ := database.QueryStaticColumnsOneString("Select location from serie_episode_files where serie_id in (Select id from series where listname=?)", "Select count(id) from serie_episode_files where serie_id in (Select id from series where listname=?)", listname)
	var listentries []string
	for idxserie := range seriefile {
		if !CheckFileExist(seriefile[idxserie].Str) {
			listentries = append(listentries, seriefile[idxserie].Str)
		}
	}
	return listentries
}

func moveFileDriveReadWrite(sourcePath, destPath string) error {

	//High Ram Usage !!!
	input, err := ioutil.ReadFile(sourcePath)
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = ioutil.WriteFile(destPath, input, 0644)
	if err != nil {
		fmt.Println("Error creating", destPath)
		fmt.Println(err)
		return err
	}
	// The copy was successful, so now delete the original file
	err = os.Remove(sourcePath)
	if err != nil {
		fmt.Println("Error removing source", sourcePath)
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

	var BUFFERSIZE int64 = int64(bufferkb) * 1024 //have to convert to bytes

	sourceFileStat, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", sourcePath)
	}
	sourceFileStat = nil

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

	if err != nil {
		return err
	}

	buf := make([]byte, BUFFERSIZE)
	for {
		n, err := source.Read(buf)
		if err != nil && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}

		if _, err := destination.Write(buf[:n]); err != nil {
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
			fmt.Println("Error copiing source", sourcePath, destPath, err)
			return err
		}
	} else {
		return errors.New("source doesnt exist")
	}
	if CheckFileExist(sourcePath) {
		// The copy was successful, so now delete the original file
		err := os.Remove(sourcePath)
		if err != nil {
			fmt.Println("Error removing source", sourcePath, err)
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
	defer func() {
		sfi = nil
	}()
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
	defer func() {
		dfi = nil
	}()
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
	// Return any errors that result from closing the destination file
	// Will return nil if no errors occurred

	// Copy the contents of the source file into the destination files
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		dstFile.Close()
		return
	}
	dstFile.Sync()
	cerr := dstFile.Close()
	if err == nil {
		err = cerr
	}
	return
}

func GetSubFolders(sourcepath string) []string {
	files, err := ioutil.ReadDir(sourcepath)
	if err == nil {
		folders := make([]string, 0, len(files))
		for idxfile := range files {
			if files[idxfile].IsDir() {
				folders = append(folders, filepath.Join(sourcepath, files[idxfile].Name()))
			}
		}
		return folders
	}
	return []string{}
}

func getSubFiles(sourcepath string) []string {
	files, err := ioutil.ReadDir(sourcepath)
	if err == nil {
		folders := make([]string, 0, len(files))
		for idxfile := range files {
			if !files[idxfile].IsDir() {
				folders = append(folders, filepath.Join(sourcepath, files[idxfile].Name()))
			}
		}
		return folders
	}
	return []string{}
}

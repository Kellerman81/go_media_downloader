package scanner

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/config"
	"github.com/Kellerman81/go_media_downloader/database"
	"github.com/Kellerman81/go_media_downloader/logger"
	"github.com/karrick/godirwalk"
)

func GetFilesDir(rootpath string, filetypes []string, filetypesNoRename []string, ignoredpaths []string) []string {

	if !config.ConfigCheck("general") {
		return []string{}
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)

	if cfg_general.UseGoDir {
		return GetFilesGoDir(rootpath, filetypes, filetypesNoRename, ignoredpaths)
	}
	var list []string

	if _, err := os.Stat(rootpath); !os.IsNotExist(err) {
		err := filepath.Walk(rootpath, func(path string, info os.FileInfo, err error) error {
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
				path, _ := filepath.Split(path)
				for idxignore := range ignoredpaths {
					if strings.Contains(strings.ToLower(path), strings.ToLower(ignoredpaths[idxignore])) {
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
	} else {
		logger.Log.Error("Path not found: ", rootpath)
	}
	return list
}

func GetFilesGoDir(rootpath string, filetypes []string, filetypesNoRename []string, ignoredpaths []string) []string {
	var list []string

	if _, err := os.Stat(rootpath); !os.IsNotExist(err) {
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

func GetFolderSize(rootpath string) int64 {
	var size int64
	if _, err := os.Stat(rootpath); !os.IsNotExist(err) {
		err := godirwalk.Walk(rootpath, &godirwalk.Options{
			Callback: func(osPathname string, de *godirwalk.Dirent) error {
				if de.IsDir() {
					return nil
				}
				info, errinfo := os.Stat(osPathname)
				if errinfo == nil {
					size += info.Size()
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
	return size
}

func GetFileSize(file string) int64 {
	var size int64
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		info, err := os.Stat(file)
		if err == nil {
			size += info.Size()
		}
	} else {
		logger.Log.Error("File not found: ", file)
	}
	return size
}

func CreateFolderWithSubfolders(path string, security uint32) error {
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
		if _, err := os.Stat(files[idxfile]); !os.IsNotExist(err) {
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
				if _, err := os.Stat(newpath); !os.IsNotExist(err) {
					if target != newpath {
						//Remove Target to supress error
						RemoveFile(newpath)
					}
				}
				err1 := os.Rename(files[idxfile], newpath)
				if err1 != nil {
					var err error
					if usebuffercopy {
						err = MoveFileDriveBuffer(files[idxfile], newpath)
					} else {
						err = MoveFileDrive(files[idxfile], newpath)
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
			if _, err := os.Stat(files[idxfile]); !os.IsNotExist(err) {
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
	if _, err := os.Stat(file); !os.IsNotExist(err) {
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
	emptyarr := make([]string, 0, 1)
	var disallow bool
	if len(disallowed) == 0 {
		disallow = false
		return disallow
	}
	logger.Log.Debug("Check disallowed")
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
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
	emptyarr := make([]string, 0, 1)
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		filesleft := GetFilesDir(folder, emptyarr, emptyarr, emptyarr)
		logger.Log.Debug("Left files: ", filesleft)
		if CleanupsizeMB >= 1 {
			leftsize := GetFolderSize(folder)
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

func checkmoviefilespath(array []database.MovieFile, find string) bool {
	for idx := range array {
		if strings.EqualFold(array[idx].Location, find) {
			return true
		}
	}
	return false
}
func checkmoviefilespathlist(array []database.MovieFile, find string, listname string) bool {
	for idx := range array {
		if strings.EqualFold(array[idx].Location, find) && strings.EqualFold(array[idx].Filename, listname) {
			return true
		}
	}
	return false
}
func GetFilesAdded(files []string, listname string) []string {
	list := make([]string, 0, len(files))
	filesdb, _ := database.QueryMovieFiles(database.Query{Select: "movie_files.location, movies.listname AS filename", InnerJoin: "movies on movie_files.movie_id = movies.id"})
	for idxfile := range files {
		if !checkmoviefilespathlist(filesdb, files[idxfile], listname) {
			if !checkmoviefilespath(filesdb, files[idxfile]) {
				logger.Log.Debug("File added to list - not found", files[idxfile], " ", listname)
				list = append(list, files[idxfile])
			}
		}
	}
	return list
}
func checkseriesfilespath(array []database.SerieEpisodeFile, find string) bool {
	for idx := range array {
		if strings.EqualFold(array[idx].Location, find) {
			return true
		}
	}
	return false
}
func checkseriesfilespathlist(array []database.SerieEpisodeFile, find string, listname string) bool {
	for idx := range array {
		if strings.EqualFold(array[idx].Location, find) && strings.EqualFold(array[idx].Filename, listname) {
			return true
		}
	}
	return false
}
func GetFilesSeriesAdded(files []string, listname string) []string {
	list := make([]string, 0, len(files))
	filesdb, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "serie_episode_files.location, series.listname AS filename", InnerJoin: "series on serie_episode_files.serie_id = series.id"})
	for idxfile := range files {
		if !checkseriesfilespathlist(filesdb, files[idxfile], listname) {
			if !checkseriesfilespath(filesdb, files[idxfile]) {
				logger.Log.Debug("File added to list - not found", files[idxfile], " ", listname)
				list = append(list, files[idxfile])
			}
		}
	}
	return list
}

func GetFilesRemoved(listname string) []string {
	moviefile, _ := database.QueryMovieFiles(database.Query{Select: "Movie_files.location", InnerJoin: "Movies on Movies.ID=movie_files.movie_id", Where: "Movies.listname = ?", WhereArgs: []interface{}{listname}})
	list := make([]string, 0, len(moviefile))
	for idxmovie := range moviefile {
		if _, err := os.Stat(moviefile[idxmovie].Location); os.IsNotExist(err) {
			list = append(list, moviefile[idxmovie].Location)
		}
	}
	return list
}

func GetFilesSeriesRemoved(listname string) []string {
	seriefile, _ := database.QuerySerieEpisodeFiles(database.Query{Select: "Serie_episode_files.location", InnerJoin: "Serie_episodes ON Serie_episodes.ID = Serie_episode_files.serie_episode_id INNER JOIN Series ON series.ID = Serie_episodes.serie_id", Where: "Series.listname = ?", WhereArgs: []interface{}{listname}})
	list := make([]string, 0, len(seriefile))
	for idxserie := range seriefile {
		if _, err := os.Stat(seriefile[idxserie].Location); os.IsNotExist(err) {
			list = append(list, seriefile[idxserie].Location)
		}
	}
	return list
}

func MoveFileDriveReadWrite(sourcePath, destPath string) error {

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

func MoveFileDriveBuffer(sourcePath, destPath string) error {
	if !config.ConfigCheck("general") {
		return errors.New("missing config")
	}
	var cfg_general config.GeneralConfig
	config.ConfigGet("general", &cfg_general)
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

func MoveFileDrive(sourcePath, destPath string) error {
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		err := CopyFile(sourcePath, destPath, false)
		if err != nil {
			fmt.Println("Error copiing source", sourcePath, destPath, err)
			return err
		}
	}
	if _, err := os.Stat(sourcePath); !os.IsNotExist(err) {
		// The copy was successful, so now delete the original file
		err = os.Remove(sourcePath)
		if err != nil {
			fmt.Println("Error removing source", sourcePath, err)
			return err
		}
	}
	return nil
}

// AbsolutePath converts a path (relative or absolute) into an absolute one.
// Supports '~' notation for $HOME directory of the current user.
func AbsolutePath(path string) (string, error) {
	homeReplaced := path
	return filepath.Abs(homeReplaced)
}

// CopyFile copies a file from src to dst. If src and dst files exist, and are
// the same, then return success. Otherwise, attempt to create a hard link
// between the two files. If that fails, copy the file contents from src to dst.
// Creates any missing directories. Supports '~' notation for $HOME directory of the current user.
func CopyFile(src, dst string, allowFileLink bool) (err error) {
	srcAbs, err := AbsolutePath(src)
	if err != nil {
		return err
	}
	dstAbs, err := AbsolutePath(dst)
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
		return fmt.Errorf("CopyFile: non-regular source file %s (%q)", sfi.Name(), sfi.Mode().String())
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
	return copyFileContents(src, dst)
}

// copyFileContents copies the contents of the file named src to the file named
// by dst. The file will be created if it does not already exist. If the
// destination file exists, all it's contents will be replaced by the contents
// of the source file.
func copyFileContents(src, dst string) (err error) {
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
	defer func() {
		cerr := dstFile.Close()
		if err == nil {
			err = cerr
		}
	}()

	// Copy the contents of the source file into the destination files
	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return
	}
	err = dstFile.Sync()
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

func GetSubFiles(sourcepath string) []string {
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

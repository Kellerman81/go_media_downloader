package structure

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

// fileStem returns the file name without directory and extension.
func fileStem(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// stemSuffix returns what follows mediaStem in stem ("" or ".en") when stem
// belongs to the media file: identical, or continued after one of seps.
func stemSuffix(stem, mediaStem, seps string) (string, bool) {
	if len(stem) < len(mediaStem) || !strings.EqualFold(stem[:len(mediaStem)], mediaStem) {
		return "", false
	}

	rest := stem[len(mediaStem):]
	if rest != "" && !strings.ContainsRune(seps, rune(rest[0])) {
		return "", false
	}

	return rest, true
}

// companionsOf lists the files next to mediafile that belong to it: an
// extension from exts and the same name, optionally followed by ".something"
// ("Movie.srt", "Movie.en.srt", "Movie.forced.idx").
func companionsOf(mediafile string, exts []string) []string {
	entries, err := os.ReadDir(filepath.Dir(mediafile))
	if err != nil {
		return nil
	}

	stem := fileStem(mediafile)

	var out []string

	for _, e := range entries {
		if e.IsDir() || !logger.SlicesContainsI(exts, filepath.Ext(e.Name())) {
			continue
		}

		path := filepath.Join(filepath.Dir(mediafile), e.Name())
		if path == mediafile {
			continue
		}

		if _, ok := stemSuffix(fileStem(e.Name()), stem, "."); ok {
			out = append(out, path)
		}
	}

	return out
}

// moveCompanionFiles moves the download's subtitle/other files (the source
// path's allowed_other_extensions) next to the imported media file and names
// them after it. A part after the original media name is kept
// ("heat.1080p.en.srt" -> "Heat (1995).en.srt"); a file with another name keeps
// its own name as suffix only when several files share its extension, so a
// lone "Subs/English.srt" or an idx/sub pair still matches the media name.
//
// Movies and books take every such file of their release folder. Series only
// take the episode's own companions, since a season pack holds other episodes,
// and so does a file lying directly in the download root.
func (s *Organizer) moveCompanionFiles(o *Organizerdata) {
	exts := s.sourcepathCfg.AllowedOtherExtensions
	if len(exts) == 0 || o.MediaFile == "" || o.TargetPath == "" || o.Filename == "" {
		return
	}

	mediaStem := fileStem(o.MediaFile)
	mediaDir := filepath.Dir(o.MediaFile)
	wholeFolder := s.Cfgp.IsType != config.MediaTypeSeries && o.Folder != "" &&
		filepath.Clean(o.Folder) != filepath.Clean(s.sourcepathCfg.Path) &&
		strings.HasPrefix(filepath.Clean(mediaDir)+string(filepath.Separator), filepath.Clean(o.Folder)+string(filepath.Separator))

	var files []string

	if wholeFolder {
		_ = filepath.WalkDir(o.Folder, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || path == o.MediaFile {
				return nil
			}

			if !logger.SlicesContainsI(exts, filepath.Ext(path)) {
				return nil
			}

			if s.sourcepathCfg.BlockedLen >= 1 && logger.SlicesContainsPart2I(s.sourcepathCfg.Blocked, path) {
				return nil
			}

			files = append(files, path)

			return nil
		})
	} else {
		entries, _ := os.ReadDir(mediaDir)
		for _, e := range entries {
			path := filepath.Join(mediaDir, e.Name())
			if e.IsDir() || path == o.MediaFile || !logger.SlicesContainsI(exts, filepath.Ext(e.Name())) {
				continue
			}

			if _, ok := stemSuffix(fileStem(e.Name()), mediaStem, "._- "); ok {
				files = append(files, path)
			}
		}
	}

	perExt := make(map[string]int, len(files))
	for _, f := range files {
		perExt[strings.ToLower(filepath.Ext(f))]++
	}

	opts := scanner.MoveFileOptions{
		UseBufferCopy: config.GetSettingsGeneral().UseFileBufferCopy,
		Chmod:         s.targetpathCfg.SetChmod,
		ChmodFolder:   s.targetpathCfg.SetChmodFolder,
		UseOther:      true,
		MediaType:     s.Cfgp.IsType,
	}

	for _, f := range files {
		stem := fileStem(f)

		suffix, ok := stemSuffix(stem, mediaStem, "._- ")
		switch {
		case ok && suffix != "":
			suffix = "." + strings.TrimLeft(suffix, "._- ")
		case ok:
		case perExt[strings.ToLower(filepath.Ext(f))] > 1:
			suffix = "." + stem
		}

		newpath, err := scanner.MoveFile(f, s.sourcepathCfg, o.TargetPath, o.Filename+suffix, opts)
		if err != nil {
			if !errors.Is(err, logger.ErrNotFound) {
				logger.Logtype("error", 1).Str(logger.StrFile, f).Err(err).Msg("companion file move")
			}

			continue
		}

		o.RenamedFiles = append(o.RenamedFiles, RenameEntry{OldName: filepath.Base(f), NewName: filepath.Base(newpath)})
	}
}

// retireOldAlbumFolders finishes replacing an album: once the folders that
// held the retired files contain no audio anymore, their leftovers (cover,
// nfo, cue, subtitles, ...) are moved to the replaced folder (move) or
// deleted, and the emptied folders are removed - disc subfolders and their
// album folder included. Library roots, the new target folder (and its
// parents) and the download folder are never touched.
func (s *Organizer) retireOldAlbumFolders(o *Organizerdata, retired []string, move bool) {
	protected := []string{o.TargetPath, s.targetpathCfg.Path, s.sourcepathCfg.Path, o.Folder}
	if s.targetpathCfg.MoveReplacedTargetPath != "" {
		protected = append(protected, s.targetpathCfg.MoveReplacedTargetPath)
	}

	for _, d := range s.Cfgp.DataMap {
		if d != nil && d.CfgPath != nil {
			protected = append(protected, d.CfgPath.Path)
		}
	}

	sep := string(filepath.Separator)
	isProtected := func(dir string) bool {
		dir = filepath.Clean(dir)
		if dir == "." || dir == sep ||
			(o.TargetPath != "" && strings.HasPrefix(dir, filepath.Clean(o.TargetPath)+sep)) {
			return true
		}

		for _, p := range protected {
			if p == "" {
				continue
			}

			// The folder itself, or a parent of it (the album folder above a
			// disc folder of the new target, the artist folder, ...).
			p = filepath.Clean(p)
			if dir == p || strings.HasPrefix(p, dir+sep) {
				return true
			}
		}

		return false
	}

	dirs := make(map[string]struct{})

	for _, f := range retired {
		dir := filepath.Dir(f)
		// A disc folder's album folder is retired with it.
		if isDiscFolderName(filepath.Base(dir)) {
			dir = filepath.Dir(dir)
		}

		dirs[dir] = struct{}{}
	}

	opts := s.replacedMoveOptions()

	for dir := range dirs {
		if isProtected(dir) || !scanner.CheckFileExist(dir) {
			continue
		}

		var leftovers []string

		hasAudio := false

		_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}

			if parser_v2.HasExtension(path, parser_v2.AudioExtensions) {
				hasAudio = true
				return fs.SkipAll
			}

			leftovers = append(leftovers, path)

			return nil
		})

		if hasAudio {
			logger.Logtype("info", 1).Str(logger.StrPath, dir).Msg("old album folder still holds audio files - left in place")
			continue
		}

		for _, f := range leftovers {
			var err error
			if move {
				rel, _ := filepath.Rel(filepath.Dir(dir), filepath.Dir(f))
				_, err = scanner.MoveFile(f, nil, filepath.Join(s.targetpathCfg.MoveReplacedTargetPath, rel), "", opts)
			} else {
				_, err = scanner.RemoveFile(f)
			}

			if err != nil {
				logger.Logtype("error", 1).Str(logger.StrFile, f).Err(err).Msg("old album leftover")
			}
		}

		removeEmptyDirs(dir)

		if parent := filepath.Dir(dir); !isProtected(parent) {
			_ = os.Remove(parent) // only succeeds when the artist folder is now empty
		}
	}
}

// removeEmptyDirs removes dir and its subfolders bottom-up as long as they are
// empty; folders that still hold files are kept.
func removeEmptyDirs(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, e := range entries {
		if e.IsDir() {
			removeEmptyDirs(filepath.Join(dir, e.Name()))
		}
	}

	_ = os.Remove(dir)
}

// isDiscFolderName reports whether name is a disc/side subfolder ("CD1",
// "Disc 2", "Disk 03").
func isDiscFolderName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, p := range []string{"cd", "disc", "disk"} {
		if rest, ok := strings.CutPrefix(n, p); ok {
			rest = strings.TrimLeft(rest, " ._-")
			return rest != "" && strings.Trim(rest, "0123456789") == ""
		}
	}

	return false
}

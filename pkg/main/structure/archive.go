package structure

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

// IsArchiveFile checks if a file is a supported archive format.
func IsArchiveFile(filename string) bool {
	// Check for compound extensions like .tar.gz
	for ext := range archiveExtensions {
		if logger.HasSuffixI(filename, ext) {
			return true
		}
	}

	return false
}

// IsMainArchiveFile checks if this is the main archive file to extract for multipart archives.
func IsMainArchiveFile(filename string) bool {
	// For RAR multipart archives, look for .part1.rar, .part01.rar, or just .rar (single file)
	if logger.ContainsI(filename, ".part") && logger.HasSuffixI(filename, ".rar") {
		// Only extract .part1.rar or .part01.rar
		return logger.ContainsI(filename, ".part1.rar") ||
			logger.ContainsI(filename, ".part01.rar")
	}

	// For ZIP multipart archives, look for .z01, .z02, etc. - only extract .zip
	if logger.HasSuffixI(filename, ".zip") && !logger.ContainsI(filename, ".z0") {
		return true
	}

	// For 7z multipart archives, look for .7z.001, .7z.002, etc. - only extract .7z
	if logger.ContainsI(filename, ".7z.") && !logger.HasSuffixI(filename, ".7z") {
		return false
	}

	// For all other formats, extract normally
	return IsArchiveFile(filename)
}

// getUnpackCommand determines the appropriate unpacking command for a given archive file.
func getUnpackCommand(archivePath, extractPath string) (string, []string, error) {
	generalCfg := config.GetSettingsGeneral()

	switch {
	case logger.HasSuffixI(archivePath, ".rar"):
		if generalCfg.UnrarPath != "" {
			return generalCfg.UnrarPath, []string{"x", "-o+", archivePath, extractPath}, nil
		}

		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "unrar", []string{"x", "-o+", archivePath, extractPath}, nil

	case logger.HasSuffixI(archivePath, ".zip"):
		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		if generalCfg.UnzipPath != "" {
			return generalCfg.UnzipPath, []string{"-o", archivePath, "-d", extractPath}, nil
		}

		return "unzip", []string{"-o", archivePath, "-d", extractPath}, nil

	case logger.HasSuffixI(archivePath, ".7z"):
		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "7z", []string{"x", "-o" + extractPath, "-y", archivePath}, nil

	case logger.HasSuffixI(archivePath, ".tar.gz") || logger.HasSuffixI(archivePath, ".tgz"):
		if generalCfg.TarPath != "" {
			return generalCfg.TarPath, []string{"-xzf", archivePath, "-C", extractPath}, nil
		}

		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "tar", []string{"-xzf", archivePath, "-C", extractPath}, nil

	case logger.HasSuffixI(archivePath, ".tar.bz2") || logger.HasSuffixI(archivePath, ".tbz2"):
		if generalCfg.TarPath != "" {
			return generalCfg.TarPath, []string{"-xjf", archivePath, "-C", extractPath}, nil
		}

		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "tar", []string{"-xjf", archivePath, "-C", extractPath}, nil

	case logger.HasSuffixI(archivePath, ".tar.xz") || logger.HasSuffixI(archivePath, ".txz"):
		if generalCfg.TarPath != "" {
			return generalCfg.TarPath, []string{"-xJf", archivePath, "-C", extractPath}, nil
		}

		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "tar", []string{"-xJf", archivePath, "-C", extractPath}, nil

	case logger.HasSuffixI(archivePath, ".tar"):
		if generalCfg.TarPath != "" {
			return generalCfg.TarPath, []string{"-xf", archivePath, "-C", extractPath}, nil
		}

		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "tar", []string{"-xf", archivePath, "-C", extractPath}, nil

	case logger.HasSuffixI(archivePath, ".gz"):
		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "gzip", []string{"-d", "-c", archivePath}, nil

	case logger.HasSuffixI(archivePath, ".bz2"):
		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "bzip2", []string{"-d", "-c", archivePath}, nil

	case logger.HasSuffixI(archivePath, ".xz"):
		if generalCfg.SevenZipPath != "" {
			return generalCfg.SevenZipPath, []string{
				"x",
				"-o" + extractPath,
				"-y",
				archivePath,
			}, nil
		}

		return "xz", []string{"-d", "-c", archivePath}, nil
	}

	return "", nil, errors.New("unsupported archive format")
}

// extractArchive extracts a single archive file to the specified directory.
func extractArchive(ctx context.Context, archivePath, extractPath string) error {
	// Create extraction directory if it doesn't exist
	if err := os.MkdirAll(extractPath, 0o755); err != nil {
		return err
	}

	command, args, err := getUnpackCommand(archivePath, extractPath)
	if err != nil {
		return err
	}

	logger.Logtype("info", 1).
		Str(logger.StrFile, archivePath).
		Msg("Extracting archive")

	cmd := exec.CommandContext(ctx, command, args...)

	cmd.Dir = filepath.Dir(archivePath)

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Logtype("error", 1).
			Str(logger.StrFile, archivePath).
			Err(err).
			Msg("Archive extraction failed")

		if len(output) > 0 {
			logger.Logtype("debug", 1).
				Str("output", string(output)).
				Msg("Extraction output")
		}

		return err
	}

	logger.Logtype("info", 1).
		Str(logger.StrFile, archivePath).
		Msg("Archive extracted successfully")

	return nil
}

// unpackArchivesInFolder scans a folder for archive files and extracts them.
func unpackArchivesInFolder(
	ctx context.Context,
	folder string,
	data *config.MediaDataImportConfig,
) error {
	if !data.EnableUnpacking {
		return nil
	}

	logger.Logtype("info", 1).
		Str(logger.StrPath, folder).
		Msg("Scanning for archives to unpack")

	var archiveFiles []string

	// Find all archive files in the folder (only main archives for multipart)
	err := filepath.WalkDir(folder, func(fpath string, info fs.DirEntry, errw error) error {
		if errw != nil {
			return errw
		}

		if err := ctx.Err(); err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		// Skip files in _unpack directories to avoid recursive unpacking
		if logger.ContainsI(fpath, "_unpack") {
			return nil
		}

		// Only extract main archive files (handles multipart archives correctly)
		if IsMainArchiveFile(info.Name()) {
			archiveFiles = append(archiveFiles, fpath)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if len(archiveFiles) == 0 {
		return nil
	}

	logger.Logtype("info", 1).
		Str(logger.StrPath, folder).
		Msg("Found archives to extract")

	// Extract each archive
	for i := range archiveFiles {
		if err := ctx.Err(); err != nil {
			return err
		}

		// Create extraction path: <archive_dir>/<archive_name>_unpack/
		archiveDir := filepath.Dir(archiveFiles[i])
		archiveName := strings.TrimSuffix(
			filepath.Base(archiveFiles[i]),
			filepath.Ext(archiveFiles[i]),
		)

		// Handle compound extensions like .tar.gz and multipart names
		for ext := range archiveExtensions {
			suffix := strings.TrimSuffix(ext, filepath.Ext(ext))

			if trimmed := logger.TrimSuffixI(archiveName, suffix); trimmed != archiveName {
				archiveName = trimmed
				break
			}
		}

		// Remove multipart suffix from archive name for extraction directory
		archiveName = logger.TrimSuffixI(archiveName, ".part1")
		archiveName = logger.TrimSuffixI(archiveName, ".part01")

		extractPath := filepath.Join(archiveDir, archiveName+"_unpack")

		// Skip if already extracted
		if scanner.CheckFileExist(extractPath) {
			logger.Logtype("info", 1).
				Str(logger.StrFile, archiveFiles[i]).
				Msg("Archive already extracted, skipping")
			continue
		}

		if err := extractArchive(ctx, archiveFiles[i], extractPath); err != nil {
			logger.Logtype("error", 1).
				Str(logger.StrFile, archiveFiles[i]).
				Err(err).
				Msg("Failed to extract archive")

			continue
		}
	}

	return nil
}

package structure

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/scanner"
)

var (
	errUnsupportedArchiveFormat = errors.New("unsupported archive format")
	errExtractSizeExceeded      = errors.New("extracted size exceeds limit")

	// extractTimeout bounds a single archive extraction so a hung or malicious
	// archive cannot block the import pipeline indefinitely. Generous enough for
	// legitimate large archives over slow storage.
	extractTimeout = 60 * time.Minute

	// maxStreamExtractBytes caps the output of streaming decompressors
	// (gzip/bzip2/xz) to defend against decompression bombs. Far above any
	// legitimate single decompressed media stream, so no false positives.
	maxStreamExtractBytes int64 = 50 << 30 // 50 GiB
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
	// For RAR multipart archives, look for .part1.rar, .part01.rar, .part001.rar,
	// or just .rar (single file). Only the first volume is extracted.
	if logger.ContainsI(filename, ".part") && logger.HasSuffixI(filename, ".rar") {
		return logger.ContainsI(filename, ".part1.rar") ||
			logger.ContainsI(filename, ".part01.rar") ||
			logger.ContainsI(filename, ".part001.rar")
	}

	// For ZIP multipart archives, look for .z01, .z02, etc. - only extract .zip
	if logger.HasSuffixI(filename, ".zip") && !logger.ContainsI(filename, ".z0") {
		return true
	}

	// For 7z multipart archives (name.7z.001, name.7z.002, ...) only extract the
	// first volume; there is no bare .7z file in a split set.
	if logger.ContainsI(filename, ".7z.") && !logger.HasSuffixI(filename, ".7z") {
		return logger.HasSuffixI(filename, ".7z.001")
	}

	// For generic raw split archives (name.001, name.002, ...) only extract the
	// first part. 7-Zip reads the remaining volumes automatically.
	if ext := filepath.Ext(filename); len(ext) == 4 && isAllDigits(ext[1:]) {
		return logger.HasSuffixI(filename, ".001")
	}

	// For all other formats, extract normally
	return IsArchiveFile(filename)
}

// isAllDigits reports whether s is non-empty and contains only ASCII digits.
func isAllDigits(s string) bool {
	if s == "" {
		return false
	}

	for i := range len(s) {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}

	return true
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

	case logger.HasSuffixI(archivePath, ".001"):
		// Split archive (name.7z.001 or generic name.001). 7-Zip opens the
		// first volume and reads the remaining parts automatically.
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

	return "", nil, errUnsupportedArchiveFormat
}

// limitedWriter wraps an io.Writer and returns errExtractSizeExceeded once more
// than limit bytes have been written. Used to abort decompression bombs mid-stream.
type limitedWriter struct {
	w       io.Writer
	limit   int64
	written int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	if lw.written+int64(len(p)) > lw.limit {
		return 0, errExtractSizeExceeded
	}

	n, err := lw.w.Write(p)

	lw.written += int64(n)

	return n, err
}

// streamsToStdout reports whether the unpack command emits decompressed data on
// stdout (gzip/bzip2/xz invoked with -c) rather than writing files itself.
func streamsToStdout(args []string) bool {
	for i := range args {
		if args[i] == "-c" {
			return true
		}
	}

	return false
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

	// Bound the extraction so a hung or malicious archive cannot stall the pipeline.
	ctx, cancel := context.WithTimeout(ctx, extractTimeout)
	defer cancel()

	logger.Logtype("info", 1).
		Str(logger.StrFile, archivePath).
		Msg("Extracting archive")

	output, err := runUnpack(ctx, archivePath, extractPath, command, args)

	// Fall back to 7-Zip when the chosen extractor isn't installed (e.g. unzip/unrar
	// missing from PATH). 7-Zip handles zip/rar/7z/tar/gz/bz2/xz, so one fallback
	// covers every archive type. Skipped when the primary already was 7-Zip.
	if err != nil && errors.Is(err, exec.ErrNotFound) {
		if szCmd, szArgs := sevenZipUnpackCommand(archivePath, extractPath); szCmd != command {
			logger.Logtype("warning", 1).
				Str(logger.StrFile, archivePath).
				Str("missing", command).
				Str("fallback", szCmd).
				Msg("Extractor not found in PATH, retrying with 7-Zip")

			output, err = runUnpack(ctx, archivePath, extractPath, szCmd, szArgs)
		}
	}

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

// sevenZipUnpackCommand returns the 7-Zip command and args to extract any archive
// into extractPath. It uses the configured SevenZipPath, falling back to "7z" on
// PATH. 7-Zip understands every archive format this package handles.
func sevenZipUnpackCommand(archivePath, extractPath string) (string, []string) {
	szCmd := config.GetSettingsGeneral().SevenZipPath
	if szCmd == "" {
		szCmd = "7z"
	}

	return szCmd, []string{"x", "-o" + extractPath, "-y", archivePath}
}

// runUnpack executes one extraction command, handling both directory extractors
// (unzip/unrar/7z/tar) and streaming decompressors (gzip/bzip2/xz) that write the
// payload to stdout. Returns the captured output and the command error.
func runUnpack(
	ctx context.Context,
	archivePath, extractPath, command string,
	args []string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = filepath.Dir(archivePath)

	if !streamsToStdout(args) {
		return cmd.CombinedOutput()
	}

	// Streaming decompressors write the decompressed payload to stdout; capture
	// it to a file in extractPath (otherwise nothing is extracted), bounded by
	// maxStreamExtractBytes to defend against decompression bombs.
	outName := strings.TrimSuffix(filepath.Base(archivePath), filepath.Ext(archivePath))
	if outName == "" || outName == filepath.Base(archivePath) {
		outName = filepath.Base(archivePath) + ".out"
	}

	outPath := filepath.Join(extractPath, outName)

	outFile, ferr := os.Create(outPath)
	if ferr != nil {
		return nil, ferr
	}

	var stderr bytes.Buffer

	cmd.Stdout = &limitedWriter{w: outFile, limit: maxStreamExtractBytes}
	cmd.Stderr = &stderr

	err := cmd.Run()

	outFile.Close()

	// Remove the partial output if extraction failed (including bomb abort).
	if err != nil {
		os.Remove(outPath)
	}

	return stderr.Bytes(), err
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

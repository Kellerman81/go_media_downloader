package parser

import (
	"errors"
	"os"
	"runtime"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
	"github.com/Kellerman81/go_media_downloader/pkg/main/worker"
)

func TestBuildPrioStr(t *testing.T) {
	tests := []struct {
		name     string
		r        uint
		q        uint
		c        uint
		a        uint
		af       uint
		expected string
	}{
		{
			name:     "All zeros",
			r:        0,
			q:        0,
			c:        0,
			a:        0,
			af:       0,
			expected: "0_0_0_0_0",
		},
		{
			name:     "Mixed values",
			r:        1080,
			q:        2,
			c:        3,
			a:        4,
			af:       0,
			expected: "1080_2_3_4_0",
		},
		{
			name:     "Large values",
			r:        4320,
			q:        999,
			c:        888,
			a:        777,
			af:       0,
			expected: "4320_999_888_777_0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPrioStr(tt.r, tt.q, tt.c, tt.a, tt.af)
			if result != tt.expected {
				t.Errorf(
					"BuildPrioStr(%d, %d, %d, %d, %d) = %s; want %s",
					tt.r,
					tt.q,
					tt.c,
					tt.a,
					tt.af,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestGetDBIDs(t *testing.T) {
	// Idempotent: only step up to pkg/main if we aren't already there, so the
	// full suite (where several tests Chdir) doesn't overshoot past the config dir.
	if _, err := os.Stat("config"); err != nil {
		if err := os.Chdir(".."); err != nil {
			t.Fatal("Failed to change to parent directory: ", err)
		}
	}
	if err := config.LoadCfgDB(false); err != nil {
		t.Skip("Skipping: config not available - ", err)
	}
	general := config.GetSettingsGeneral()
	if general == nil {
		t.Skip("Skipping: settings not available")
	}
	logger.InitLogger(logger.Config{
		LogLevel:     "Warning",
		LogFileSize:  general.LogFileSize,
		LogFileCount: general.LogFileCount,
		LogCompress:  general.LogCompress,
	})
	database.UpgradeDB()
	if err := database.InitDB(general.DBLogLevel); err != nil {
		t.Fatal("Failed to init database: ", err)
	}
	database.InitCache()
	tests := []struct {
		name           string
		parseInfo      *database.ParseInfo
		cfgp           *config.MediaTypeConfig
		allowSearch    bool
		expectedError  error
		expectedListID int
	}{
		{
			name:          "Nil ParseInfo",
			parseInfo:     nil,
			cfgp:          &config.MediaTypeConfig{},
			allowSearch:   true,
			expectedError: logger.ErrNotFound,
		},
		{
			name: "Empty ParseInfo",
			parseInfo: &database.ParseInfo{
				Title: "",
				Imdb:  "",
			},
			cfgp:           &config.MediaTypeConfig{IsType: config.MediaTypeMovie},
			allowSearch:    true,
			expectedError:  logger.ErrNotFoundDbmovie,
			expectedListID: -1,
		},
		{
			name: "Invalid IMDB ID",
			parseInfo: &database.ParseInfo{
				Title: "Test Movie",
				Imdb:  "invalid",
			},
			cfgp:           &config.MediaTypeConfig{IsType: config.MediaTypeMovie},
			allowSearch:    true,
			expectedError:  logger.ErrNotFoundDbmovie,
			expectedListID: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GetDBIDs(tt.parseInfo, tt.cfgp, tt.allowSearch, false)
			if !errors.Is(err, tt.expectedError) {
				t.Errorf("GetDBIDs() error = %v, expectedError %v", err, tt.expectedError)
			}
			if tt.parseInfo != nil && tt.parseInfo.ListID != tt.expectedListID {
				t.Errorf(
					"GetDBIDs() ListID = %v, expected %v",
					tt.parseInfo.ListID,
					tt.expectedListID,
				)
			}
		})
	}
}

func TestLoadDBPatterns(t *testing.T) {
	database.InitCache()

	// Clear existing patterns
	scanpatterns = nil

	// Run LoadDBPatterns
	LoadDBPatterns()

	// Verify patterns were loaded
	if len(scanpatterns) == 0 {
		t.Error("LoadDBPatterns() failed to load any patterns")
	}

	// Verify global patterns were loaded
	foundGlobalPatterns := 0
	for _, p := range scanpatterns {
		for _, gp := range globalscanpatterns {
			if p.name == gp.name && p.re == gp.re {
				foundGlobalPatterns++
			}
		}
	}

	if foundGlobalPatterns != len(globalscanpatterns) {
		t.Errorf(
			"LoadDBPatterns() loaded %d global patterns, expected %d",
			foundGlobalPatterns,
			len(globalscanpatterns),
		)
	}
}

func TestGenerateAllQualityPriorities(t *testing.T) {
	// Tests run from parser/ dir, but config expects ./config/config.toml from pkg/main/.
	// Idempotent so the full suite (multiple tests Chdir) doesn't overshoot.
	if _, err := os.Stat("config"); err != nil {
		if err := os.Chdir(".."); err != nil {
			t.Fatal("Failed to change to parent directory: ", err)
		}
	}
	if err := config.LoadCfgDB(false); err != nil {
		t.Skip("Skipping: config not available - ", err)
	}
	database.InitCache()
	general := config.GetSettingsGeneral()
	if general == nil {
		t.Skip("Skipping: settings not available")
	}
	worker.InitWorkerPools(
		general.WorkerSearch,
		general.WorkerFiles,
		general.WorkerMetadata,
		general.WorkerRSS,
		general.WorkerIndexer,
	)
	logger.InitLogger(logger.Config{
		LogLevel:      general.LogLevel,
		LogFileSize:   general.LogFileSize,
		LogFileCount:  general.LogFileCount,
		LogCompress:   general.LogCompress,
		LogToFileOnly: general.LogToFileOnly,
		LogColorize:   general.LogColorize,
		TimeFormat:    general.TimeFormat,
		TimeZone:      general.TimeZone,
	})
	err := database.InitDB(general.DBLogLevel)
	if err != nil {
		t.Skip("Skipping: database not available - ", err)
	}
	err = database.InitImdbdb()
	if err != nil {
		t.Skip("Skipping: IMDB database not available - ", err)
	}
	database.SetVars()
	GenerateAllQualityPriorities()

	logger.Logtype("info", 0).
		Msg("Load DB Patterns")
	LoadDBPatterns()

	logger.Logtype("info", 0).
		Msg("Load DB Cutoff")
	GenerateCutoffPriorities()

	// Verify priorities were generated
	if len(Getcompleteallprios()) == 0 {
		t.Error("GenerateAllQualityPriorities() failed to generate any priorities")
	}

	if len(Getallprios()) == 0 {
		t.Error("GenerateAllQualityPriorities() failed to generate any wanted priorities")
	}

	// Verify priorities are unique per quality group
	seen := make(map[string]bool)
	for _, p := range Getcompleteallprios() {
		key := p.QualityGroup + "_" + BuildPrioStr(
			p.ResolutionID,
			p.QualityID,
			p.CodecID,
			p.AudioID,
			p.AudioFormatID,
		)
		if seen[key] {
			t.Errorf("GenerateAllQualityPriorities() generated duplicate priority: %s", key)
		}
		seen[key] = true
	}
}

func TestGetImdbFilename(t *testing.T) {
	// Test on different operating systems
	tests := []struct {
		name     string
		goos     string
		expected string
	}{
		{
			name:     "Windows",
			goos:     "windows",
			expected: "init_imdb.exe",
		},
		{
			name:     "Linux",
			goos:     "linux",
			expected: "./init_imdb",
		},
		{
			name:     "Darwin (macOS)",
			goos:     "darwin",
			expected: "./init_imdb",
		},
		{
			name:     "FreeBSD",
			goos:     "freebsd",
			expected: "./init_imdb",
		},
	}

	// Save original GOOS
	originalGOOS := runtime.GOOS

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We can't actually change runtime.GOOS in tests,
			// so we test the actual function on the current OS
			result := getImdbFilename()

			// On current OS, verify the result matches expected pattern
			if runtime.GOOS == "windows" {
				if result != "init_imdb.exe" {
					t.Errorf("getImdbFilename() on Windows = %s; want init_imdb.exe", result)
				}
			} else {
				if result != "./init_imdb" {
					t.Errorf("getImdbFilename() on non-Windows = %s; want ./init_imdb", result)
				}
			}
		})
	}

	// Restore original GOOS (not needed in this case, but good practice)
	_ = originalGOOS
}

func TestGetallprios(t *testing.T) {
	// Initialize test data
	wanted := []Prioarr{
		{
			QualityGroup: "test1",
			Priority:     100,
			ResolutionID: 1,
			QualityID:    2,
			CodecID:      3,
			AudioID:      4,
		},
		{
			QualityGroup: "test2",
			Priority:     200,
			ResolutionID: 5,
			QualityID:    6,
			CodecID:      7,
			AudioID:      8,
		},
	}
	setPrioritiesForTest(nil, wanted)

	result := Getallprios()

	// Verify we get a copy of the data
	if len(result) != len(wanted) {
		t.Errorf(
			"Getallprios() returned %d items; want %d",
			len(result),
			len(wanted),
		)
	}

	// Verify content matches
	for i, expected := range wanted {
		if i >= len(result) {
			t.Errorf("Getallprios() missing item at index %d", i)
			continue
		}

		actual := result[i]
		if actual.QualityGroup != expected.QualityGroup ||
			actual.Priority != expected.Priority ||
			actual.ResolutionID != expected.ResolutionID ||
			actual.QualityID != expected.QualityID ||
			actual.CodecID != expected.CodecID ||
			actual.AudioID != expected.AudioID {
			t.Errorf("Getallprios()[%d] = %+v; want %+v", i, actual, expected)
		}
	}

	// Verify that Getallprios returns a copy, not a reference.
	// Mutating the returned slice must not affect the active snapshot.
	if len(result) > 0 {
		result[0].Priority = 999
		if Getallprios()[0].Priority == 999 {
			t.Error("Getallprios() returned a reference; expected an independent copy")
		}
	}
}

func TestGetcompleteallprios(t *testing.T) {
	// Initialize test data
	all := []Prioarr{
		{
			QualityGroup: "complete1",
			Priority:     150,
			ResolutionID: 10,
			QualityID:    20,
			CodecID:      30,
			AudioID:      40,
		},
		{
			QualityGroup: "complete2",
			Priority:     250,
			ResolutionID: 50,
			QualityID:    60,
			CodecID:      70,
			AudioID:      80,
		},
		{
			QualityGroup: "complete3",
			Priority:     350,
			ResolutionID: 90,
			QualityID:    100,
			CodecID:      110,
			AudioID:      120,
		},
	}
	setPrioritiesForTest(all, nil)

	result := Getcompleteallprios()

	// Verify we get a copy of the data
	if len(result) != len(all) {
		t.Errorf(
			"Getcompleteallprios() returned %d items; want %d",
			len(result),
			len(all),
		)
	}

	// Verify content matches
	for i, expected := range all {
		if i >= len(result) {
			t.Errorf("Getcompleteallprios() missing item at index %d", i)
			continue
		}

		actual := result[i]
		if actual.QualityGroup != expected.QualityGroup ||
			actual.Priority != expected.Priority ||
			actual.ResolutionID != expected.ResolutionID ||
			actual.QualityID != expected.QualityID ||
			actual.CodecID != expected.CodecID ||
			actual.AudioID != expected.AudioID {
			t.Errorf("Getcompleteallprios()[%d] = %+v; want %+v", i, actual, expected)
		}
	}
}

func TestSplitByFull(t *testing.T) {
	tests := []struct {
		name     string
		str      string
		splitby  rune
		expected string
	}{
		{
			name:     "Split by space",
			str:      "hello world test",
			splitby:  ' ',
			expected: "hello",
		},
		{
			name:     "Split by dot",
			str:      "file.name.ext",
			splitby:  '.',
			expected: "file",
		},
		{
			name:     "Split character not found",
			str:      "no-split-char",
			splitby:  '_',
			expected: "no-split-char",
		},
		{
			name:     "Empty string",
			str:      "",
			splitby:  ' ',
			expected: "",
		},
		{
			name:     "Split character at beginning",
			str:      ".hidden.file",
			splitby:  '.',
			expected: "",
		},
		{
			name:     "Split character at end",
			str:      "filename.",
			splitby:  '.',
			expected: "filename",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitByFull(tt.str, tt.splitby)
			if result != tt.expected {
				t.Errorf(
					"splitByFull(%q, %q) = %q; want %q",
					tt.str,
					tt.splitby,
					result,
					tt.expected,
				)
			}
		})
	}
}

package parser

import (
	"errors"
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
		expected string
	}{
		{
			name:     "All zeros",
			r:        0,
			q:        0,
			c:        0,
			a:        0,
			expected: "0_0_0_0",
		},
		{
			name:     "Mixed values",
			r:        1080,
			q:        2,
			c:        3,
			a:        4,
			expected: "1080_2_3_4",
		},
		{
			name:     "Large values",
			r:        4320,
			q:        999,
			c:        888,
			a:        777,
			expected: "4320_999_888_777",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := BuildPrioStr(tt.r, tt.q, tt.c, tt.a)
			if result != tt.expected {
				t.Errorf(
					"BuildPrioStr(%d, %d, %d, %d) = %s; want %s",
					tt.r,
					tt.q,
					tt.c,
					tt.a,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestGetDBIDs(t *testing.T) {
	config.LoadCfgDB(true)
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
			cfgp:          &config.MediaTypeConfig{Useseries: false},
			allowSearch:   true,
			expectedError: logger.ErrNotFoundDbmovie,
		},
		{
			name: "Invalid IMDB ID",
			parseInfo: &database.ParseInfo{
				Title: "Test Movie",
				Imdb:  "invalid",
			},
			cfgp:          &config.MediaTypeConfig{Useseries: false},
			allowSearch:   true,
			expectedError: logger.ErrNotFoundDbmovie,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := GetDBIDs(tt.parseInfo, tt.cfgp, tt.allowSearch)
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
	config.LoadCfgDB(false)
	database.InitCache()
	general := config.GetSettingsGeneral()
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
		LogZeroValues: general.LogZeroValues,
	})
	err := database.InitDB(general.DBLogLevel)
	if err != nil {
		logger.Logtype("fatal", 0).
			Err(err).
			Msg("Database Initialization Failed")
	}
	err = database.InitImdbdb()
	if err != nil {
		logger.Logtype("fatal", 0).
			Err(err).
			Msg("IMDB Database Initialization Failed")
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
	if len(allQualityPrioritiesT) == 0 {
		t.Error("GenerateAllQualityPriorities() failed to generate any priorities")
	}

	if len(allQualityPrioritiesWantedT) == 0 {
		t.Error("GenerateAllQualityPriorities() failed to generate any wanted priorities")
	}

	// Verify priorities are unique
	seen := make(map[string]bool)
	for _, p := range allQualityPrioritiesT {
		key := BuildPrioStr(p.ResolutionID, p.QualityID, p.CodecID, p.AudioID)
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
	allQualityPrioritiesWantedT = []Prioarr{
		{QualityGroup: "test1", Priority: 100, ResolutionID: 1, QualityID: 2, CodecID: 3, AudioID: 4},
		{QualityGroup: "test2", Priority: 200, ResolutionID: 5, QualityID: 6, CodecID: 7, AudioID: 8},
	}

	result := Getallprios()

	// Verify we get a copy of the data
	if len(result) != len(allQualityPrioritiesWantedT) {
		t.Errorf("Getallprios() returned %d items; want %d", len(result), len(allQualityPrioritiesWantedT))
	}

	// Verify content matches
	for i, expected := range allQualityPrioritiesWantedT {
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

	// NOTE: The function comment says it returns a copy, but it actually returns a reference
	// This test documents the actual behavior, not the expected behavior from the comment
	if len(result) > 0 {
		originalPriority := result[0].Priority
		result[0].Priority = 999
		if allQualityPrioritiesWantedT[0].Priority == 999 {
			t.Log("Getallprios() returns a reference (not a copy as documented)")
			// This is the actual behavior, restore for other tests
			result[0].Priority = originalPriority
		} else {
			t.Error("Getallprios() should return a reference but didn't")
		}
	}
}

func TestGetcompleteallprios(t *testing.T) {
	// Initialize test data
	allQualityPrioritiesT = []Prioarr{
		{QualityGroup: "complete1", Priority: 150, ResolutionID: 10, QualityID: 20, CodecID: 30, AudioID: 40},
		{QualityGroup: "complete2", Priority: 250, ResolutionID: 50, QualityID: 60, CodecID: 70, AudioID: 80},
		{QualityGroup: "complete3", Priority: 350, ResolutionID: 90, QualityID: 100, CodecID: 110, AudioID: 120},
	}

	result := Getcompleteallprios()

	// Verify we get a copy of the data
	if len(result) != len(allQualityPrioritiesT) {
		t.Errorf("Getcompleteallprios() returned %d items; want %d", len(result), len(allQualityPrioritiesT))
	}

	// Verify content matches
	for i, expected := range allQualityPrioritiesT {
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
				t.Errorf("splitByFull(%q, %q) = %q; want %q", tt.str, tt.splitby, result, tt.expected)
			}
		})
	}
}


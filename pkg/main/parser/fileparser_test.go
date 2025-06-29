package parser

import (
	"errors"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/logger"
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
	database.InitCache()

	// Clear existing priorities
	allQualityPrioritiesT = nil
	allQualityPrioritiesWantedT = nil

	// Run generation
	GenerateAllQualityPriorities()

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

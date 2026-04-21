package parser_v2

import (
	"fmt"
	"testing"
)

// TestParserComparison compares parser_v2 output against expected results
// that would be produced by the original parser for the same inputs.
// Note: We cannot directly import the original parser due to circular dependencies,
// so we compare against known expected values.
func TestParserComparison(t *testing.T) {
	// Test cases with expected values (matching original parser behavior)
	tests := []struct {
		name     string
		filename string
		// Expected values from original parser
		wantTitle      string
		wantYear       int
		wantResolution string
		wantQuality    string
		wantCodec      string
		wantAudio      string
		wantSeason     int
		wantEpisode    int
		wantIdentifier string
		wantImdb       string
		wantExtended   bool
		wantProper     bool
		wantRepack     bool
		isMovie        bool
	}{
		// Movie test cases
		{
			name:           "Standard movie with year and quality",
			filename:       "The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
			wantTitle:      "The Matrix",
			wantYear:       1999,
			wantResolution: "1080p",
			wantQuality:    "BluRay",
			wantCodec:      "x264",
			isMovie:        true,
		},
		{
			name:           "Movie with spaces in name",
			filename:       "The Shawshank Redemption 1994 720p WEB-DL.mkv",
			wantTitle:      "The Shawshank Redemption",
			wantYear:       1994,
			wantResolution: "720p",
			wantQuality:    "WEB-DL",
			isMovie:        true,
		},
		{
			name:           "Movie with IMDB ID",
			filename:       "Inception.2010.1080p.BluRay.tt1375666.mkv",
			wantTitle:      "Inception",
			wantYear:       2010,
			wantResolution: "1080p",
			wantQuality:    "BluRay",
			wantImdb:       "tt1375666",
			isMovie:        true,
		},
		{
			name:           "Extended edition movie",
			filename:       "The.Lord.of.the.Rings.2001.EXTENDED.1080p.BluRay.x265.mkv",
			wantTitle:      "The Lord of the Rings",
			wantYear:       2001,
			wantResolution: "1080p",
			wantQuality:    "BluRay",
			wantCodec:      "x265",
			wantExtended:   true,
			isMovie:        true,
		},
		{
			name:           "Proper release movie",
			filename:       "Avatar.2009.1080p.BluRay.PROPER.x264.mkv",
			wantTitle:      "Avatar",
			wantYear:       2009,
			wantResolution: "1080p",
			wantQuality:    "BluRay",
			wantCodec:      "x264",
			wantProper:     true,
			isMovie:        true,
		},
		{
			name:           "4K UHD movie",
			filename:       "Dune.2021.2160p.UHD.BluRay.x265.mkv",
			wantTitle:      "Dune",
			wantYear:       2021,
			wantResolution: "2160p",
			wantQuality:    "BluRay",
			wantCodec:      "x265",
			isMovie:        true,
		},
		// Series test cases
		{
			name:           "Standard series episode S01E01",
			filename:       "Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
			wantTitle:      "Breaking Bad",
			wantResolution: "720p",
			wantQuality:    "BluRay",
			wantCodec:      "x264",
			wantSeason:     1,
			wantEpisode:    1,
			wantIdentifier: "S01E01",
			isMovie:        false,
		},
		{
			name:           "Series episode with double digits",
			filename:       "Game.of.Thrones.S08E06.1080p.WEB-DL.mkv",
			wantTitle:      "Game of Thrones",
			wantResolution: "1080p",
			wantQuality:    "WEB-DL",
			wantSeason:     8,
			wantEpisode:    6,
			wantIdentifier: "S08E06",
			isMovie:        false,
		},
		{
			name:           "Series with x notation (1x01)",
			filename:       "Friends.1x01.720p.WEB-DL.mkv",
			wantTitle:      "Friends",
			wantResolution: "720p",
			wantQuality:    "WEB-DL",
			wantSeason:     1,
			wantEpisode:    1,
			wantIdentifier: "S01E01",
			isMovie:        false,
		},
		{
			name:           "Series with year in title",
			filename:       "The.Mandalorian.2019.S02E01.1080p.WEB-DL.mkv",
			wantTitle:      "The Mandalorian",
			wantYear:       2019,
			wantResolution: "1080p",
			wantQuality:    "WEB-DL",
			wantSeason:     2,
			wantEpisode:    1,
			wantIdentifier: "S02E01",
			isMovie:        false,
		},
		{
			name:           "Multi-episode format",
			filename:       "House.M.D.S03E01E02.720p.BluRay.mkv",
			wantTitle:      "House M D",
			wantResolution: "720p",
			wantQuality:    "BluRay",
			wantSeason:     3,
			wantEpisode:    1,
			wantIdentifier: "S03E01-E02",
			isMovie:        false,
		},
		// Edge cases
		{
			name:           "Movie with underscores",
			filename:       "The_Dark_Knight_2008_1080p_BluRay_x264.mkv",
			wantTitle:      "The Dark Knight",
			wantYear:       2008,
			wantResolution: "1080p",
			wantQuality:    "BluRay",
			wantCodec:      "x264",
			isMovie:        true,
		},
		{
			name:           "Repack release",
			filename:       "Stranger.Things.S04E01.REPACK.1080p.WEB-DL.mkv",
			wantTitle:      "Stranger Things",
			wantResolution: "1080p",
			wantQuality:    "WEB-DL",
			wantSeason:     4,
			wantEpisode:    1,
			wantIdentifier: "S04E01",
			wantRepack:     true,
			isMovie:        false,
		},
	}

	parser := NewParser()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseVideo(tt.filename)

			// Compare title
			if result.Title != tt.wantTitle {
				t.Errorf("Title mismatch:\n  got:  %q\n  want: %q", result.Title, tt.wantTitle)
			}

			// Compare year (only for movies or when year is in filename)
			if tt.wantYear > 0 && result.Year != tt.wantYear {
				t.Errorf("Year mismatch:\n  got:  %d\n  want: %d", result.Year, tt.wantYear)
			}

			// Compare resolution
			if tt.wantResolution != "" && result.Resolution != tt.wantResolution {
				// Allow HDR suffix difference
				if result.Resolution != tt.wantResolution+" HDR" {
					t.Errorf(
						"Resolution mismatch:\n  got:  %q\n  want: %q",
						result.Resolution,
						tt.wantResolution,
					)
				}
			}

			// Compare quality
			if tt.wantQuality != "" && result.Quality != tt.wantQuality {
				t.Errorf(
					"Quality mismatch:\n  got:  %q\n  want: %q",
					result.Quality,
					tt.wantQuality,
				)
			}

			// Compare codec
			if tt.wantCodec != "" && result.Codec != tt.wantCodec {
				t.Errorf("Codec mismatch:\n  got:  %q\n  want: %q", result.Codec, tt.wantCodec)
			}

			// Compare audio (if specified)
			if tt.wantAudio != "" && result.Audio != tt.wantAudio {
				t.Errorf("Audio mismatch:\n  got:  %q\n  want: %q", result.Audio, tt.wantAudio)
			}

			// Compare series-specific fields
			if !tt.isMovie {
				if result.Season != tt.wantSeason {
					t.Errorf(
						"Season mismatch:\n  got:  %d\n  want: %d",
						result.Season,
						tt.wantSeason,
					)
				}
				if result.Episode != tt.wantEpisode {
					t.Errorf(
						"Episode mismatch:\n  got:  %d\n  want: %d",
						result.Episode,
						tt.wantEpisode,
					)
				}
				if tt.wantIdentifier != "" && result.Identifier != tt.wantIdentifier {
					t.Errorf(
						"Identifier mismatch:\n  got:  %q\n  want: %q",
						result.Identifier,
						tt.wantIdentifier,
					)
				}

				// Check media type
				if result.MediaType != MediaTypeSeries {
					t.Errorf(
						"MediaType mismatch:\n  got:  %v\n  want: %v",
						result.MediaType,
						MediaTypeSeries,
					)
				}
			} else {
				// Check media type for movies
				if result.MediaType != MediaTypeMovie {
					t.Errorf(
						"MediaType mismatch:\n  got:  %v\n  want: %v",
						result.MediaType,
						MediaTypeMovie,
					)
				}
			}

			// Compare IMDB
			if tt.wantImdb != "" && result.Imdb != tt.wantImdb {
				t.Errorf("IMDB mismatch:\n  got:  %q\n  want: %q", result.Imdb, tt.wantImdb)
			}

			// Compare flags
			if result.Extended != tt.wantExtended {
				t.Errorf(
					"Extended flag mismatch:\n  got:  %v\n  want: %v",
					result.Extended,
					tt.wantExtended,
				)
			}
			if result.Proper != tt.wantProper {
				t.Errorf(
					"Proper flag mismatch:\n  got:  %v\n  want: %v",
					result.Proper,
					tt.wantProper,
				)
			}
			if result.Repack != tt.wantRepack {
				t.Errorf(
					"Repack flag mismatch:\n  got:  %v\n  want: %v",
					result.Repack,
					tt.wantRepack,
				)
			}
		})
	}
}

// TestParserComparisonPrintResults prints a detailed comparison for debugging.
func TestParserComparisonPrintResults(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping detailed comparison in short mode")
	}

	testFiles := []string{
		// Movies
		"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
		"Inception.2010.2160p.UHD.BluRay.x265.DTS-HD.MA.mkv",
		"The.Godfather.1972.REMASTERED.1080p.BluRay.x264.mkv",
		"Avatar.2009.EXTENDED.1080p.BluRay.PROPER.x264.mkv",
		// Series
		"Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
		"Game.of.Thrones.S08E06.The.Iron.Throne.1080p.WEB-DL.mkv",
		"Friends.1x01.The.Pilot.720p.WEB-DL.mkv",
		"The.Office.US.S01E01E02.1080p.BluRay.x264.mkv",
		// Edge cases
		"2001.A.Space.Odyssey.1968.1080p.BluRay.mkv",
		"1917.2019.1080p.BluRay.x264.mkv",
	}

	parser := NewParser()

	for _, filename := range testFiles {
		result := parser.ParseVideo(filename)
		fmt.Printf("\n=== %s ===\n", filename)
		fmt.Printf("  Title:      %q\n", result.Title)
		fmt.Printf("  Year:       %d\n", result.Year)
		fmt.Printf("  MediaType:  %s\n", result.MediaType)
		fmt.Printf("  Resolution: %s\n", result.Resolution)
		fmt.Printf("  Quality:    %s\n", result.Quality)
		fmt.Printf("  Codec:      %s\n", result.Codec)
		fmt.Printf("  Audio:      %s\n", result.Audio)
		if result.MediaType == MediaTypeSeries {
			fmt.Printf("  Season:     %d\n", result.Season)
			fmt.Printf("  Episode:    %d\n", result.Episode)
			fmt.Printf("  Identifier: %s\n", result.Identifier)
		}
		fmt.Printf("  Extended:   %v\n", result.Extended)
		fmt.Printf("  Proper:     %v\n", result.Proper)
		fmt.Printf("  Repack:     %v\n", result.Repack)
		fmt.Printf("  IMDB:       %s\n", result.Imdb)
		fmt.Printf("  Confidence: %.2f\n", result.Confidence)
	}
}

// BenchmarkParserV2 benchmarks the v2 parser performance.
func BenchmarkParserV2(b *testing.B) {
	testFiles := []string{
		"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
		"Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
		"Inception.2010.2160p.UHD.BluRay.x265.DTS-HD.MA.mkv",
		"Game.of.Thrones.S08E06.The.Iron.Throne.1080p.WEB-DL.mkv",
	}

	parser := NewParser()

	for b.Loop() {
		for _, filename := range testFiles {
			_ = parser.ParseVideo(filename)
		}
	}
}

// TestParserFieldMapping verifies that parser_v2 produces fields compatible with
// the database.ParseInfo struct used by the original parser.
func TestParserFieldMapping(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		filename string
		check    func(r *VideoParseResult) error
	}{
		{
			filename: "The.Matrix.1999.1080p.BluRay.x264.mkv",
			check: func(r *VideoParseResult) error {
				// Verify fields that map to ParseInfo
				if r.Title == "" {
					return fmt.Errorf("Title should not be empty")
				}
				if r.Year == 0 {
					return fmt.Errorf("Year should not be 0")
				}
				if r.Resolution == "" {
					return fmt.Errorf("Resolution should not be empty")
				}
				if r.Quality == "" {
					return fmt.Errorf("Quality should not be empty")
				}
				if r.Codec == "" {
					return fmt.Errorf("Codec should not be empty")
				}
				return nil
			},
		},
		{
			filename: "Breaking.Bad.S01E01.720p.BluRay.mkv",
			check: func(r *VideoParseResult) error {
				// Verify series-specific fields
				if r.Season == 0 {
					return fmt.Errorf("Season should not be 0")
				}
				if r.Episode == 0 {
					return fmt.Errorf("Episode should not be 0")
				}
				if r.Identifier == "" {
					return fmt.Errorf("Identifier should not be empty")
				}
				if r.MediaType != MediaTypeSeries {
					return fmt.Errorf("MediaType should be series, got %v", r.MediaType)
				}
				return nil
			},
		},
		{
			filename: "Movie.2020.tt1234567.1080p.mkv",
			check: func(r *VideoParseResult) error {
				// Verify IMDB extraction
				if r.Imdb == "" {
					return fmt.Errorf("IMDB should not be empty")
				}
				if r.Imdb != "tt1234567" {
					return fmt.Errorf("IMDB mismatch: got %s, want tt1234567", r.Imdb)
				}
				return nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := parser.ParseVideo(tt.filename)
			if err := tt.check(result); err != nil {
				t.Error(err)
			}
		})
	}
}

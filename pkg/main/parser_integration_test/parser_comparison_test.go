package parser_integration_test

import (
	"fmt"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser"
	"github.com/Kellerman81/go_media_downloader/pkg/main/parser_v2"
)

// TestParserV1vsV2Comparison compares the original parser against parser_v2
// using the same input filenames to verify compatible output.
func TestParserV1vsV2Comparison(t *testing.T) {
	// Skip if database is not available (patterns not loaded)
	if len(database.DBConnect.ResolutionStrIn) == 0 {
		t.Skip("Database patterns not loaded - skipping comparison test")
	}

	// Load database patterns for parser_v2
	parser_v2.LoadDBPatterns()

	// Ensure original parser patterns are loaded
	parser.LoadDBPatterns()

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

	parserV2 := parser_v2.NewParser()

	for _, filename := range testFiles {
		t.Run(filename, func(t *testing.T) {
			// Parse with original parser (usepath=false, usefolder=false means just parse the filename)
			v1Result := parser.ParseFile(filename, false, false, nil, -1)
			defer database.PLParseInfo.Put(v1Result)

			// Parse with v2 parser
			v2Result := parserV2.ParseVideo(filename)

			// Compare results
			if v1Result.Title != v2Result.Title {
				t.Logf("Title differs: v1=%q, v2=%q", v1Result.Title, v2Result.Title)
			}

			if int(v1Result.Year) != v2Result.Year {
				t.Logf("Year differs: v1=%d, v2=%d", v1Result.Year, v2Result.Year)
			}

			if v1Result.Resolution != v2Result.Resolution {
				t.Logf("Resolution differs: v1=%q, v2=%q", v1Result.Resolution, v2Result.Resolution)
			}

			if v1Result.Quality != v2Result.Quality {
				t.Logf("Quality differs: v1=%q, v2=%q", v1Result.Quality, v2Result.Quality)
			}

			if v1Result.Codec != v2Result.Codec {
				t.Logf("Codec differs: v1=%q, v2=%q", v1Result.Codec, v2Result.Codec)
			}

			if v1Result.Audio != v2Result.Audio {
				t.Logf("Audio differs: v1=%q, v2=%q", v1Result.Audio, v2Result.Audio)
			}

			if v1Result.Season != v2Result.Season {
				t.Logf("Season differs: v1=%d, v2=%d", v1Result.Season, v2Result.Season)
			}

			if v1Result.Episode != v2Result.Episode {
				t.Logf("Episode differs: v1=%d, v2=%d", v1Result.Episode, v2Result.Episode)
			}

			if v1Result.Identifier != v2Result.Identifier {
				t.Logf("Identifier differs: v1=%q, v2=%q", v1Result.Identifier, v2Result.Identifier)
			}

			if v1Result.Imdb != v2Result.Imdb {
				t.Logf("IMDB differs: v1=%q, v2=%q", v1Result.Imdb, v2Result.Imdb)
			}

			if v1Result.Extended != v2Result.Extended {
				t.Logf("Extended differs: v1=%v, v2=%v", v1Result.Extended, v2Result.Extended)
			}

			if v1Result.Proper != v2Result.Proper {
				t.Logf("Proper differs: v1=%v, v2=%v", v1Result.Proper, v2Result.Proper)
			}

			if v1Result.Repack != v2Result.Repack {
				t.Logf("Repack differs: v1=%v, v2=%v", v1Result.Repack, v2Result.Repack)
			}
		})
	}
}

// TestParserV1vsV2PrintComparison prints side-by-side comparison for debugging.
func TestParserV1vsV2PrintComparison(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping detailed comparison in short mode")
	}

	// Skip if database is not available
	if len(database.DBConnect.ResolutionStrIn) == 0 {
		t.Skip("Database patterns not loaded - skipping comparison test")
	}

	parser_v2.LoadDBPatterns()
	parser.LoadDBPatterns()

	testFiles := []string{
		"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
		"Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
		"Inception.2010.2160p.UHD.BluRay.x265.DTS-HD.MA.mkv",
		"Game.of.Thrones.S08E06.The.Iron.Throne.1080p.WEB-DL.mkv",
	}

	parserV2 := parser_v2.NewParser()

	for _, filename := range testFiles {
		v1Result := parser.ParseFile(filename, false, false, nil, -1)
		defer database.PLParseInfo.Put(v1Result)
		v2Result := parserV2.ParseVideo(filename)

		fmt.Printf("\n=== %s ===\n", filename)
		fmt.Printf("%-15s %-30s %-30s\n", "Field", "V1 (original)", "V2 (new)")
		fmt.Printf("%-15s %-30s %-30s\n", "-----", "-------------", "--------")
		fmt.Printf("%-15s %-30q %-30q\n", "Title", v1Result.Title, v2Result.Title)
		fmt.Printf("%-15s %-30d %-30d\n", "Year", v1Result.Year, v2Result.Year)
		fmt.Printf("%-15s %-30s %-30s\n", "Resolution", v1Result.Resolution, v2Result.Resolution)
		fmt.Printf("%-15s %-30s %-30s\n", "Quality", v1Result.Quality, v2Result.Quality)
		fmt.Printf("%-15s %-30s %-30s\n", "Codec", v1Result.Codec, v2Result.Codec)
		fmt.Printf("%-15s %-30s %-30s\n", "Audio", v1Result.Audio, v2Result.Audio)
		fmt.Printf("%-15s %-30d %-30d\n", "Season", v1Result.Season, v2Result.Season)
		fmt.Printf("%-15s %-30d %-30d\n", "Episode", v1Result.Episode, v2Result.Episode)
		fmt.Printf("%-15s %-30s %-30s\n", "Identifier", v1Result.Identifier, v2Result.Identifier)
		fmt.Printf("%-15s %-30s %-30s\n", "IMDB", v1Result.Imdb, v2Result.Imdb)
		fmt.Printf("%-15s %-30v %-30v\n", "Extended", v1Result.Extended, v2Result.Extended)
		fmt.Printf("%-15s %-30v %-30v\n", "Proper", v1Result.Proper, v2Result.Proper)
		fmt.Printf("%-15s %-30v %-30v\n", "Repack", v1Result.Repack, v2Result.Repack)
	}
}

// BenchmarkParserV1 benchmarks the original parser performance.
func BenchmarkParserV1(b *testing.B) {
	if len(database.DBConnect.ResolutionStrIn) == 0 {
		b.Skip("Database patterns not loaded")
	}

	parser.LoadDBPatterns()

	testFiles := []string{
		"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
		"Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
		"Inception.2010.2160p.UHD.BluRay.x265.DTS-HD.MA.mkv",
		"Game.of.Thrones.S08E06.The.Iron.Throne.1080p.WEB-DL.mkv",
	}

	for b.Loop() {
		for _, filename := range testFiles {
			result := parser.ParseFile(filename, false, false, nil, -1)
			database.PLParseInfo.Put(result)
		}
	}
}

// BenchmarkParserV2 benchmarks the v2 parser performance.
func BenchmarkParserV2(b *testing.B) {
	parser_v2.LoadDBPatterns()

	testFiles := []string{
		"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
		"Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
		"Inception.2010.2160p.UHD.BluRay.x265.DTS-HD.MA.mkv",
		"Game.of.Thrones.S08E06.The.Iron.Throne.1080p.WEB-DL.mkv",
	}

	p := parser_v2.NewParser()

	for b.Loop() {
		for _, filename := range testFiles {
			_ = p.ParseVideo(filename)
		}
	}
}

// BenchmarkParserV2WithDBPatterns benchmarks v2 parser with database patterns.
func BenchmarkParserV2WithDBPatterns(b *testing.B) {
	if len(database.DBConnect.ResolutionStrIn) == 0 {
		b.Skip("Database patterns not loaded")
	}

	parser_v2.LoadDBPatterns()
	ps := parser_v2.GetPatternStore()
	vp := parser_v2.NewVideoParserWithPatternStore(ps)

	testFiles := []string{
		"The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv",
		"Breaking.Bad.S01E01.720p.BluRay.x264.mkv",
		"Inception.2010.2160p.UHD.BluRay.x265.DTS-HD.MA.mkv",
		"Game.of.Thrones.S08E06.The.Iron.Throne.1080p.WEB-DL.mkv",
	}

	for b.Loop() {
		for _, filename := range testFiles {
			_ = vp.Parse(filename)
		}
	}
}

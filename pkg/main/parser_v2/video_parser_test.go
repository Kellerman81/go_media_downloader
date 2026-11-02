package parser_v2

import (
	"regexp"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/database"
)

func TestExtractASINFromPath(t *testing.T) {
	tests := map[string]string{
		"/data/Audiobooks/Stephen Fry/Mythos [B0759MNMD2]": "B0759MNMD2",
		`D:\Audiobooks\Mythos [B0759MNMD2]\CD1`:            "B0759MNMD2",
		"/data/Audiobooks/Stephen Fry/Mythos":              "",
		"/data/B0759MNMD2/Mythos [B07ZZZZZZZ]":             "B07ZZZZZZZ",
	}

	for in, want := range tests {
		if got := extractASINFromPath(in); got != want {
			t.Errorf("extractASINFromPath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestVideoParser_Parse_Movies(t *testing.T) {
	vp := NewVideoParser()

	tests := []struct {
		name     string
		filename string
		want     struct {
			title      string
			year       int
			resolution string
			quality    string
			codec      string
			audio      string
			imdb       string
			extended   bool
		}
	}{
		{
			name:     "Standard movie format",
			filename: "The Matrix.1999.1080p.BluRay.x264.DTS-GROUP.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "The Matrix",
				year:       1999,
				resolution: "1080p",
				quality:    "BluRay",
				codec:      "x264",
				audio:      "DTS",
			},
		},
		{
			name:     "Language before date movie format",
			filename: "The Matrix.German.1999.1080p.BluRay.x264.DTS-GROUP.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "The Matrix",
				year:       1999,
				resolution: "1080p",
				quality:    "BluRay",
				codec:      "x264",
				audio:      "DTS",
			},
		},
		{
			name:     "4K UHD movie",
			filename: "Dune.2021.2160p.UHD.BluRay.x265.HDR.Atmos-GROUP.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "Dune",
				year:       2021,
				resolution: "2160p HDR", // HDR is appended to resolution
				quality:    "BluRay",
				codec:      "x265",
				audio:      "Atmos",
			},
		},
		{
			name:     "Movie with IMDB",
			filename: "Inception.2010.1080p.BluRay.tt1375666.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "Inception",
				year:       2010,
				resolution: "1080p",
				quality:    "BluRay",
				imdb:       "tt1375666",
			},
		},
		{
			name:     "Movie with Language in Name and IMDB",
			filename: "The.Italian.Job.2003.1080p.BluRay.tt1375666.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "The Italian Job",
				year:       2003,
				resolution: "1080p",
				quality:    "BluRay",
				imdb:       "tt1375666",
			},
		},
		{
			name:     "Extended edition",
			filename: "The Lord of the Rings.2001.Extended.1080p.BluRay.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "The Lord of the Rings",
				year:       2001,
				resolution: "1080p",
				quality:    "BluRay",
				extended:   true,
			},
		},
		{
			name:     "WEB-DL movie",
			filename: "Spider-Man.No.Way.Home.2021.1080p.AMZN.WEB-DL.DDP5.1.H264.mkv",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "Spider-Man No Way Home",
				year:       2021,
				resolution: "1080p",
				quality:    "AMZN",
			},
		},
		{
			// Regression: a source word ("Amazon") that is part of the title must
			// not truncate the title. The year is the correct boundary here.
			name:     "Source word in title",
			filename: "Love on the Amazon 2026 1080p WEB-DL HEVC x265 BONE",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "Love on the Amazon",
				year:       2026,
				resolution: "1080p",
				quality:    "AMZN",
				codec:      "x265",
			},
		},
		{
			// Regression: a short source tag ("nf") inside a title word must not
			// match as a quality token ("U[nf]orgettable") and truncate the title.
			name:     "Source tag inside title word",
			filename: "Unforgettable Christmas 2024 1080p WEB-DL AAC2.0 x264",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "Unforgettable Christmas",
				year:       2024,
				resolution: "1080p",
				quality:    "WEB-DL",
				codec:      "x264",
			},
		},
		{
			name:     "No Valid Extension",
			filename: "The.Christmas.Spark.[2025].1080p.WEBRip-LAMA",
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				codec      string
				audio      string
				imdb       string
				extended   bool
			}{
				title:      "The Christmas Spark",
				year:       2025,
				resolution: "1080p",
				quality:    "WEBRip",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vp.Parse(tt.filename)

			if result.Title != tt.want.title {
				t.Errorf("Title = %s, want %s", result.Title, tt.want.title)
			}
			if result.MediaType != MediaTypeMovie {
				t.Errorf("MediaType = %v, want MediaTypeMovie", result.MediaType)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if tt.want.resolution != "" && result.Resolution != tt.want.resolution {
				t.Errorf("Resolution = %q, want %q", result.Resolution, tt.want.resolution)
			}
			if tt.want.quality != "" && result.Quality != tt.want.quality {
				t.Errorf("Quality = %q, want %q", result.Quality, tt.want.quality)
			}
			if tt.want.codec != "" && result.Codec != tt.want.codec {
				t.Errorf("Codec = %q, want %q", result.Codec, tt.want.codec)
			}
			if tt.want.audio != "" && result.Audio != tt.want.audio {
				t.Errorf("Audio = %q, want %q", result.Audio, tt.want.audio)
			}
			if tt.want.imdb != "" && result.Imdb != tt.want.imdb {
				t.Errorf("Imdb = %q, want %q", result.Imdb, tt.want.imdb)
			}
			if tt.want.extended && !result.Extended {
				t.Errorf("Extended = %v, want %v", result.Extended, tt.want.extended)
			}
		})
	}
}

func TestVideoParser_Parse_Series(t *testing.T) {
	vp := NewVideoParser()

	tests := []struct {
		name     string
		filename string
		want     struct {
			title      string
			season     int
			episode    int
			identifier string
			resolution string
			quality    string
		}
	}{
		{
			name:     "Standard episode format",
			filename: "Breaking.Bad.S05E16.1080p.BluRay.x264.mkv",
			want: struct {
				title      string
				season     int
				episode    int
				identifier string
				resolution string
				quality    string
			}{
				title:      "Breaking Bad",
				season:     5,
				episode:    16,
				identifier: "S05E16",
				resolution: "1080p",
			},
		},
		{
			name:     "Multi-episode",
			filename: "Game.of.Thrones.S01E01-E02.1080p.BluRay.mkv",
			want: struct {
				title      string
				season     int
				episode    int
				identifier string
				resolution string
				quality    string
			}{
				title:      "Game of Thrones",
				season:     1,
				episode:    1,
				identifier: "S01E01-E02",
				resolution: "1080p",
			},
		},
		{
			name:     "Alternative format (x notation)",
			filename: "Friends.1x01.720p.BluRay.mkv",
			want: struct {
				title      string
				season     int
				episode    int
				identifier string
				resolution string
				quality    string
			}{
				title:      "Friends",
				season:     1,
				episode:    1,
				identifier: "S01E01",
				resolution: "720p",
			},
		},
		{
			name:     "Date-based episode",
			filename: "Last.Week.Tonight.2024.01.15.720p.WEB.h264.mkv",
			want: struct {
				title      string
				season     int
				episode    int
				identifier string
				resolution string
				quality    string
			}{
				title:      "Last Week Tonight",
				identifier: "2024-01-15",
				resolution: "720p",
				quality:    "WEB",
			},
		},
		{
			name:     "Date-based episode short date",
			filename: "Last.Week.Tonight.24.01.15.720p.WEB.h264.mkv",
			want: struct {
				title      string
				season     int
				episode    int
				identifier string
				resolution string
				quality    string
			}{
				title:      "Last Week Tonight",
				identifier: "24-01-15",
				resolution: "720p",
				quality:    "WEB",
			},
		},
		{
			name:     "Path-based episode",
			filename: "/data/EN_Series/Alias/Season 1/Alias - S01E01 - Truth Be Told - 480P DVDRIP XVID - proper.avi",
			want: struct {
				title      string
				season     int
				episode    int
				identifier string
				resolution string
				quality    string
			}{
				title:      "Alias",
				identifier: "S01E01",
				resolution: "480p",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := vp.Parse(tt.filename)

			if result.MediaType != MediaTypeSeries {
				t.Errorf("MediaType = %v, want MediaTypeSeries", result.MediaType)
			}
			if tt.want.season != 0 && result.Season != tt.want.season {
				t.Errorf("Season = %d, want %d", result.Season, tt.want.season)
			}
			if tt.want.episode != 0 && result.Episode != tt.want.episode {
				t.Errorf("Episode = %d, want %d", result.Episode, tt.want.episode)
			}
			if tt.want.identifier != "" && result.Identifier != tt.want.identifier {
				t.Errorf("Identifier = %q, want %q", result.Identifier, tt.want.identifier)
			}
			if tt.want.resolution != "" && result.Resolution != tt.want.resolution {
				t.Errorf("Resolution = %q, want %q", result.Resolution, tt.want.resolution)
			}
			if tt.want.quality != "" && result.Quality != tt.want.quality {
				t.Errorf("Quality = %q, want %q", result.Quality, tt.want.quality)
			}
		})
	}
}

func BenchmarkVideoParser_Parse(b *testing.B) {
	vp := NewVideoParser()
	filename := "The.Matrix.Reloaded.2003.REMASTERED.2160p.UHD.BluRay.x265.10bit.HDR.DTS-HD.MA.7.1-GROUP.mkv"

	for b.Loop() {
		vp.Parse(filename)
	}
}

func TestVideoParser_Parse_NZBStyle(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name  string
		input string
		want  struct {
			title      string
			year       int
			resolution string
			quality    string
			proper     bool
		}
	}{
		{
			name:  "Simple quoted NZB filename",
			input: `[001/279] "The.Matrix.Revolutions.2003.PROPER.1080p.UHD.BluRay.DD.7.1.x264-LoRD.mkv"`,
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				proper     bool
			}{
				title:      "The Matrix Revolutions",
				year:       2003,
				resolution: "1080p",
				quality:    "BluRay",
				proper:     true,
			},
		},
		{
			name:  "Complex Usenet subject line",
			input: `[22569]-[FULL]-[#a.b.hdtv.x264@EFNet]-[ The.Matrix.1999.1080p.BluRay.x264-CtrlHD ]-[136/143] - "The.Matrix.1999.1080p.BluRay.x264-CtrlHD.vol000+01.par2" yEnc`,
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				proper     bool
			}{
				title:      "The Matrix",
				year:       1999,
				resolution: "1080p",
				quality:    "BluRay",
			},
		},
		{
			name:  "Quality prefix format",
			input: `(TR-1080p)[01/80] - "The.Matrix.1999.1080p.DUAL.BluRay.x264.DTS.ShareKiosk.mkv" yEnc`,
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				proper     bool
			}{
				title:      "The Matrix",
				year:       1999,
				resolution: "1080p",
				quality:    "BluRay",
			},
		},
		{
			name:  "Site tag format",
			input: `<kere.ws> - Filme - 1325657022 - The.Matrix.German.1999.DVDRIP.AC3.XviD.iNTERNAL-TU - [01/58] - "tu-tma.par2" yEnc`,
			want: struct {
				title      string
				year       int
				resolution string
				quality    string
				proper     bool
			}{
				title: "The Matrix",
				year:  1999,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.ParseVideo(tt.input)

			if result.Title != tt.want.title {
				t.Errorf("Title = %q, want %q", result.Title, tt.want.title)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if tt.want.resolution != "" && result.Resolution != tt.want.resolution {
				t.Errorf("Resolution = %q, want %q", result.Resolution, tt.want.resolution)
			}
			if tt.want.quality != "" && result.Quality != tt.want.quality {
				t.Errorf("Quality = %q, want %q", result.Quality, tt.want.quality)
			}
			if tt.want.proper && !result.Proper {
				t.Errorf("Proper = %v, want %v", result.Proper, tt.want.proper)
			}
		})
	}
}

func TestRegression_VideoParseFileP(t *testing.T) {
	tests := []struct {
		input      string
		title      string
		year       uint16
		identifier string
		episode    int
		resolution string
		quality    string
	}{
		// Resolution tokens need word boundaries ("sd" in Wednesday, "hd" in Ashdown).
		{"Wednesday.S01E01.1080p.WEB.x264-GRP.mkv", "Wednesday", 0, "S01E01", 1, "1080p", ""},
		{"Wisdom.of.the.Crowd.2019.1080p.mkv", "Wisdom of the Crowd", 2019, "", 0, "1080p", ""},
		{"Ashdown.2019.720p.mkv", "Ashdown", 2019, "", 0, "720p", ""},
		{"Highdive.2020.1080p.mkv", "Highdive", 2020, "", 0, "1080p", ""},
		{"Movie [1080p] (2019).mkv", "Movie", 2019, "", 0, "1080p", ""},
		// Year selection: last/bracketed year before the first non-title anchor.
		{"Blade Runner 2049 (2017) 1080p.mkv", "Blade Runner 2049", 2017, "", 0, "1080p", ""},
		{"Movie 1917 2019 1080p.mkv", "Movie 1917", 2019, "", 0, "1080p", ""},
		{"1917.2019.1080p.mkv", "1917", 2019, "", 0, "1080p", ""},
		{"2001.A.Space.Odyssey.1968.1080p.mkv", "2001 A Space Odyssey", 1968, "", 0, "1080p", ""},
		{"Doctor.Who.2005.S01E01.720p.mkv", "Doctor Who", 2005, "S01E01", 1, "720p", ""},
		// Multi-episode identifiers.
		{"Show.S01E01E02.720p.mkv", "Show", 0, "S01E01E02", 1, "720p", ""},
		{"Show.S01E01E02E03.720p.mkv", "Show", 0, "S01E01E02E03", 1, "720p", ""},
		{"Show.S01E01-E03.720p.mkv", "Show", 0, "S01E01-E03", 1, "720p", ""},
		{"Show.S01E02-720p-GRP.mkv", "Show", 0, "S01E02", 2, "720p", ""},
		// Episode-only marker needs a boundary.
		{"Se7en.1995.1080p.mkv", "Se7en", 1995, "", 0, "1080p", ""},
		// Episode-only marker ends the title.
		{"Show.E05.720p.mkv", "Show", 0, "", 5, "720p", ""},
		{"Show.Ep12.HDTV.mkv", "Show", 0, "", 12, "", "HDTV"},
		// 3-digit episode numbers.
		{"Show.S01E100.720p.mkv", "Show", 0, "S01E100", 100, "720p", ""},
		{"Show.S01E99E100.720p.mkv", "Show", 0, "S01E99E100", 99, "720p", ""},
		// Hyphenated source variants.
		{"Movie.2019.web-rip.mkv", "Movie", 2019, "", 0, "", "WEBRip"},
		{"Movie.2019.dvd-rip.mkv", "Movie", 2019, "", 0, "", "DVDRip"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			m := &database.ParseInfo{}
			ParseFileP(tt.input, false, false, nil, 0, m)

			if m.Title != tt.title {
				t.Errorf("Title = %q, want %q", m.Title, tt.title)
			}
			if m.Year != tt.year {
				t.Errorf("Year = %d, want %d", m.Year, tt.year)
			}
			if m.Identifier != tt.identifier {
				t.Errorf("Identifier = %q, want %q", m.Identifier, tt.identifier)
			}
			if m.Episode != tt.episode {
				t.Errorf("Episode = %d, want %d", m.Episode, tt.episode)
			}
			if tt.resolution != "" && m.Resolution != tt.resolution {
				t.Errorf("Resolution = %q, want %q", m.Resolution, tt.resolution)
			}
			if tt.quality != "" && m.Quality != tt.quality {
				t.Errorf("Quality = %q, want %q", m.Quality, tt.quality)
			}
		})
	}
}

func TestRegression_PadInt(t *testing.T) {
	for n, want := range map[int]string{0: "00", 7: "07", 42: "42", 100: "100", 1234: "1234"} {
		if got := padInt(n); got != want {
			t.Errorf("padInt(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestRegression_NormalizeQuality(t *testing.T) {
	for in, want := range map[string]string{
		"web-rip": "WEBRip", "WEB RIP": "WEBRip", "webrip": "WEBRip",
		"dvd-rip": "DVDRip", "dvdrip": "DVDRip",
	} {
		if got := normalizeQuality(in); got != want {
			t.Errorf("normalizeQuality(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRegression_BareDashEpisodeRange(t *testing.T) {
	cases := map[string]string{
		"Eureka.S04E04-05.720p.HDTV.x264-DIMENSION": "S04E04-05",
		"Show.S01E02-720p-GRP":                      "S01E02",
		"Show.S01E02-2019.720p":                     "S01E02",
		"Show.S01E09-03.720p":                       "S01E09",
		"Show.S01E01-E03.720p":                      "S01E01-E03",
	}
	for in, want := range cases {
		m := ParseFile(in, false, false, nil, -1)
		if m == nil {
			t.Fatalf("%s: nil", in)
		}
		if m.Identifier != want {
			t.Errorf("%s: identifier %q, want %q", in, m.Identifier, want)
		}
	}
}

func TestVideoParseWithPathUsesParentDir(t *testing.T) {
	vp := NewVideoParser()

	r := vp.ParseWithPath("/movies/Heat (1995)/heat.mkv")
	if r.Year != 1995 || r.SourcePath != "/movies/Heat (1995)/heat.mkv" {
		t.Errorf("year from parent = %d, SourcePath = %q", r.Year, r.SourcePath)
	}

	s := vp.ParseWithPath("/tv/Breaking.Bad.2008/S01E01.mkv")
	if s.MediaType != MediaTypeSeries || s.Title != "Breaking Bad" {
		t.Errorf("series title from parent = %q (%v)", s.Title, s.MediaType)
	}
}

func testPatternStore() *DBPatternStore {
	return &DBPatternStore{
		Resolutions: []DBPattern{
			{Name: "1080p", Strings: []string{"1080p"}, ID: 1},
			{Name: "2160p", Strings: []string{"2160p", "4k"}, ID: 2},
			{Name: "720p", UseRegex: true, Regex: regexp.MustCompile(`(?i)(720[pi])`), ID: 3},
		},
		Qualities: []DBPattern{{Name: "BluRay", Strings: []string{"bluray", "bdrip"}, ID: 10}},
		Codecs:    []DBPattern{{Name: "x265", Strings: []string{"x265", "hevc"}, ID: 20}},
		Audios:    []DBPattern{{Name: "DTS", Strings: []string{"dts"}, ID: 30}},
	}
}

func TestExtractQualityInfoFromDB(t *testing.T) {
	vp := NewVideoParserWithPatternStore(testPatternStore())

	var r VideoParseResult
	vp.extractQualityInfoFromDB("Movie.2020.4K.BDRip.HEVC.DTS-GRP", &r)

	if r.Resolution != "2160p" || r.ResolutionID != 2 || r.Quality != "BluRay" || r.QualityID != 10 ||
		r.Codec != "x265" || r.CodecID != 20 || r.Audio != "DTS" || r.AudioID != 30 {
		t.Errorf("string matches = %+v", r)
	}

	r = VideoParseResult{}
	vp.extractQualityInfoFromDB("Movie.2020.720i.REMUX", &r)

	if r.Resolution != "720p" || r.ResolutionID != 3 || r.Quality != "REMUX" {
		t.Errorf("regex/remux = %q (%d) %q", r.Resolution, r.ResolutionID, r.Quality)
	}

	// Pattern strings inside a word do not match ("dts" in "Edtsx").
	r = VideoParseResult{}
	vp.extractQualityInfoFromDB("Edtsx.Movie", &r)

	if r.Audio != "" {
		t.Errorf("in-word match: %q", r.Audio)
	}
}

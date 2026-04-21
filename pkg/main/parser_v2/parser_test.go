package parser_v2

import (
	"testing"
)

func TestBookParser_Parse(t *testing.T) {
	bp := NewBookParser()

	tests := []struct {
		name     string
		filename string
		want     struct {
			title    string
			author   string
			isbn13   string
			isbn10   string
			asin     string
			series   string
			position string
			year     int
			format   string
			retail   bool
		}
	}{
		{
			name:     "Simple book with author",
			filename: "Stephen King - The Shining.epub",
			want: struct {
				title    string
				author   string
				isbn13   string
				isbn10   string
				asin     string
				series   string
				position string
				year     int
				format   string
				retail   bool
			}{
				title:  "The Shining",
				author: "Stephen King",
				format: "epub",
			},
		},
		{
			name:     "Book with ISBN-13",
			filename: "Clean Code (ISBN 978-0-13-468599-1).pdf",
			want: struct {
				title    string
				author   string
				isbn13   string
				isbn10   string
				asin     string
				series   string
				position string
				year     int
				format   string
				retail   bool
			}{
				isbn13: "9780134685991",
				format: "pdf",
			},
		},
		{
			name:     "Book with ASIN",
			filename: "B0CHXKQC2M - The Great Novel.mobi",
			want: struct {
				title    string
				author   string
				isbn13   string
				isbn10   string
				asin     string
				series   string
				position string
				year     int
				format   string
				retail   bool
			}{
				asin:   "B0CHXKQC2M",
				format: "mobi",
			},
		},
		{
			name:     "Book with series",
			filename: "Brandon Sanderson - The Final Empire (Mistborn Book 1).epub",
			want: struct {
				title    string
				author   string
				isbn13   string
				isbn10   string
				asin     string
				series   string
				position string
				year     int
				format   string
				retail   bool
			}{
				author:   "Brandon Sanderson",
				series:   "Mistborn",
				position: "1",
				format:   "epub",
			},
		},
		{
			name:     "Book with year and retail",
			filename: "Robert C. Martin - Clean Architecture (2017) [Retail].epub",
			want: struct {
				title    string
				author   string
				isbn13   string
				isbn10   string
				asin     string
				series   string
				position string
				year     int
				format   string
				retail   bool
			}{
				author: "Robert C. Martin",
				year:   2017,
				retail: true,
				format: "epub",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := bp.Parse(tt.filename)

			if tt.want.author != "" && result.Author != tt.want.author {
				t.Errorf("Author = %q, want %q", result.Author, tt.want.author)
			}
			if tt.want.isbn13 != "" && result.ISBN13 != tt.want.isbn13 {
				t.Errorf("ISBN13 = %q, want %q", result.ISBN13, tt.want.isbn13)
			}
			if tt.want.asin != "" && result.ASIN != tt.want.asin {
				t.Errorf("ASIN = %q, want %q", result.ASIN, tt.want.asin)
			}
			if tt.want.series != "" && result.Series != tt.want.series {
				t.Errorf("Series = %q, want %q", result.Series, tt.want.series)
			}
			if tt.want.position != "" && result.SeriesPosition != tt.want.position {
				t.Errorf("SeriesPosition = %q, want %q", result.SeriesPosition, tt.want.position)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if result.Format != tt.want.format {
				t.Errorf("Format = %q, want %q", result.Format, tt.want.format)
			}
			if tt.want.retail && !result.IsRetail {
				t.Errorf("IsRetail = %v, want %v", result.IsRetail, tt.want.retail)
			}
		})
	}
}

func TestAudiobookParser_Parse(t *testing.T) {
	ap := NewAudiobookParser()

	tests := []struct {
		name     string
		filename string
		want     struct {
			title    string
			author   string
			narrator string
			asin     string
			abridged bool
			format   string
		}
	}{
		{
			name:     "Simple audiobook",
			filename: "Stephen King - The Stand - Read by Grover Gardner.m4b",
			want: struct {
				title    string
				author   string
				narrator string
				asin     string
				abridged bool
				format   string
			}{
				author:   "Stephen King",
				narrator: "Grover Gardner",
				format:   "m4b",
			},
		},
		{
			name:     "Audiobook with ASIN",
			filename: "B0BX7F2P6G - Project Hail Mary.m4b",
			want: struct {
				title    string
				author   string
				narrator string
				asin     string
				abridged bool
				format   string
			}{
				asin:   "B0BX7F2P6G",
				format: "m4b",
			},
		},
		{
			name:     "Unabridged audiobook",
			filename: "Andy Weir - The Martian [Unabridged].mp3",
			want: struct {
				title    string
				author   string
				narrator string
				asin     string
				abridged bool
				format   string
			}{
				author:   "Andy Weir",
				abridged: false,
				format:   "mp3",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ap.Parse(tt.filename)

			if tt.want.author != "" && result.Author != tt.want.author {
				t.Errorf("Author = %q, want %q", result.Author, tt.want.author)
			}
			if tt.want.narrator != "" && result.Narrator != tt.want.narrator {
				t.Errorf("Narrator = %q, want %q", result.Narrator, tt.want.narrator)
			}
			if tt.want.asin != "" && result.ASIN != tt.want.asin {
				t.Errorf("ASIN = %q, want %q", result.ASIN, tt.want.asin)
			}
			if result.Format != tt.want.format {
				t.Errorf("Format = %q, want %q", result.Format, tt.want.format)
			}
		})
	}
}

func TestMusicParser_ParseAlbum(t *testing.T) {
	mp := NewMusicParser()

	tests := []struct {
		name    string
		dirName string
		want    struct {
			artist      string
			album       string
			year        int
			releaseType string
		}
	}{
		{
			name:    "Standard album format",
			dirName: "Pink Floyd - The Dark Side of the Moon (1973) [FLAC]",
			want: struct {
				artist      string
				album       string
				year        int
				releaseType string
			}{
				artist: "Pink Floyd",
				album:  "The Dark Side of the Moon",
				year:   1973,
			},
		},
		{
			name:    "Album with release type",
			dirName: "Radiohead - OK Computer [Album] (1997)",
			want: struct {
				artist      string
				album       string
				year        int
				releaseType string
			}{
				artist:      "Radiohead",
				year:        1997,
				releaseType: "album",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &MusicParseResult{}
			mp.parseAlbumName(tt.dirName, result)

			if tt.want.artist != "" && result.Artist != tt.want.artist {
				t.Errorf("Artist = %q, want %q", result.Artist, tt.want.artist)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if tt.want.releaseType != "" && result.ReleaseType != tt.want.releaseType {
				t.Errorf("ReleaseType = %q, want %q", result.ReleaseType, tt.want.releaseType)
			}
		})
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

func TestRuntimeMatcher_MatchRuntime(t *testing.T) {
	rm := DefaultRuntimeMatcher()

	tests := []struct {
		name     string
		expected int64
		actual   int64
		want     bool
	}{
		{
			name:     "Exact match",
			expected: 3600000, // 1 hour
			actual:   3600000,
			want:     true,
		},
		{
			name:     "Within 3% tolerance",
			expected: 3600000, // 1 hour
			actual:   3708000, // 1 hour + 1.8 minutes (3%)
			want:     true,
		},
		{
			name:     "Outside tolerance",
			expected: 3600000, // 1 hour
			actual:   4000000, // ~11% over
			want:     false,
		},
		{
			name:     "Minimum tolerance applies",
			expected: 60000, // 1 minute (3% would be 1.8s, but min is 5s)
			actual:   65000, // 5 seconds over
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rm.MatchRuntime(tt.expected, tt.actual)
			if got != tt.want {
				t.Errorf("MatchRuntime(%d, %d) = %v, want %v", tt.expected, tt.actual, got, tt.want)
			}
		})
	}
}

func TestISBN_Validation(t *testing.T) {
	tests := []struct {
		name     string
		isbn     string
		valid    bool
		isISBN13 bool
	}{
		{
			name:     "Valid ISBN-13",
			isbn:     "978-0-13-468599-1",
			valid:    true,
			isISBN13: true,
		},
		{
			name:     "Valid ISBN-10",
			isbn:     "0-306-40615-2", // A valid ISBN-10 with correct checksum
			valid:    true,
			isISBN13: false,
		},
		{
			name:     "Invalid ISBN-13 checksum",
			isbn:     "978-0-13-468599-0",
			valid:    false,
			isISBN13: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got bool
			if tt.isISBN13 {
				got = ValidateISBN13(tt.isbn)
			} else {
				got = ValidateISBN10(tt.isbn)
			}
			if got != tt.valid {
				t.Errorf("Validate ISBN = %v, want %v", got, tt.valid)
			}
		})
	}
}

func TestUnifiedParser_Parse(t *testing.T) {
	p := NewParser()

	tests := []struct {
		name     string
		filename string
		wantType MediaType
	}{
		{
			name:     "Video file",
			filename: "Movie.2023.1080p.BluRay.mkv",
			wantType: MediaTypeMovie,
		},
		{
			name:     "Ebook file",
			filename: "Author - Book Title.epub",
			wantType: MediaTypeBook,
		},
		{
			name:     "Audiobook file",
			filename: "Audiobook Title.m4b",
			wantType: MediaTypeAudiobook,
		},
		{
			name:     "Music file",
			filename: "01 - Track Name.flac",
			wantType: MediaTypeMusic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.Parse(tt.filename)
			if result.MediaType != tt.wantType {
				t.Errorf("MediaType = %v, want %v", result.MediaType, tt.wantType)
			}
		})
	}
}

func TestMediaType_String(t *testing.T) {
	tests := []struct {
		mt   MediaType
		want string
	}{
		{MediaTypeMovie, "movie"},
		{MediaTypeSeries, "series"},
		{MediaTypeBook, "book"},
		{MediaTypeAudiobook, "audiobook"},
		{MediaTypeMusic, "music"},
		{MediaTypeUnknown, "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.mt.String(); got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsExtension(t *testing.T) {
	tests := []struct {
		name string
		ext  string
		fn   func(string) bool
		want bool
	}{
		{"mkv is video", ".mkv", IsVideoExtension, true},
		{"mp4 is video", ".MP4", IsVideoExtension, true},
		{"epub is book", ".epub", IsBookExtension, true},
		{"pdf is book", ".PDF", IsBookExtension, true},
		{"m4b is audiobook", ".m4b", IsAudiobookExtension, true},
		{"mp3 is audio", ".mp3", IsAudioExtension, true},
		{"flac is lossless", ".flac", IsLosslessAudioExtension, true},
		{"mp3 is not lossless", ".mp3", IsLosslessAudioExtension, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.ext); got != tt.want {
				t.Errorf("%s = %v, want %v", tt.name, got, tt.want)
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

func BenchmarkBookParser_Parse(b *testing.B) {
	bp := NewBookParser()
	filename := "Brandon Sanderson - Mistborn Book 1 - The Final Empire (2006) [978-0-7653-1178-8] [Retail].epub"

	for b.Loop() {
		bp.Parse(filename)
	}
}

func BenchmarkMusicParser_Parse(b *testing.B) {
	mp := NewMusicParser()
	filename := "01 - Comfortably Numb.flac"

	for b.Loop() {
		mp.Parse(filename)
	}
}

func TestNZBPreprocessor_Clean(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple quoted filename with part number",
			input:    `[001/279] "The.Matrix.Revolutions.2003.PROPER.1080p.UHD.BluRay.DD.7.1.x264-LoRD.mkv"`,
			expected: "The.Matrix.Revolutions.2003.PROPER.1080p.UHD.BluRay.DD.7.1.x264-LoRD.mkv",
		},
		{
			name:     "Complex Usenet subject with channel and metadata",
			input:    `[22569]-[FULL]-[#a.b.hdtv.x264@EFNet]-[ The.Matrix.1999.1080p.BluRay.x264-CtrlHD ]-[136/143] - "The.Matrix.1999.1080p.BluRay.x264-CtrlHD.vol000+01.par2" yEnc`,
			expected: "The.Matrix.1999.1080p.BluRay.x264-CtrlHD",
		},
		{
			name:     "Quality prefix with part number",
			input:    `(TR-1080p)[01/80] - "The.Matrix.1999.1080p.DUAL.BluRay.x264.DTS.ShareKiosk.mkv" yEnc`,
			expected: "The.Matrix.1999.1080p.DUAL.BluRay.x264.DTS.ShareKiosk.mkv",
		},
		{
			name:     "Site tag with metadata",
			input:    `<kere.ws> - Filme - 1325657022 - The.Matrix.German.1999.DVDRIP.AC3.XviD.iNTERNAL-TU - [01/58] - "tu-tma.par2" yEnc`,
			expected: "The.Matrix.German.1999.DVDRIP.AC3.XviD.iNTERNAL-TU",
		},
		{
			name:     "Par2 file extraction",
			input:    `[001/279] "The.Matrix.Revolutions.2003.PROPER.1080p.UHD.BluRay.DD.7.1.x264-LoRD.par2"`,
			expected: "The.Matrix.Revolutions.2003.PROPER.1080p.UHD.BluRay.DD.7.1.x264-LoRD",
		},
		{
			name:     "Simple filename without NZB formatting",
			input:    "The.Matrix.1999.1080p.BluRay.x264.mkv",
			expected: "The.Matrix.1999.1080p.BluRay.x264.mkv",
		},
		{
			name:     "Part number prefix only",
			input:    `[05/50] The.Matrix.1999.1080p.BluRay.mkv`,
			expected: "The.Matrix.1999.1080p.BluRay.mkv",
		},
		{
			name:     "RAR file in NZB format",
			input:    `[001/100] "Movie.2020.1080p.BluRay.x264.rar"`,
			expected: "Movie.2020.1080p.BluRay.x264.rar",
		},
		{
			name:     "Name at end in NZB format",
			input:    `The.Matrix.Revolutions.2003.720p.BluRay.x264-CtrlHD -[02/96] - "The.Matrix.Revolutions.2003.720p.BluRay.x264-CtrlHD.md5" yEnc`,
			expected: "The.Matrix.Revolutions.2003.720p.BluRay.x264-CtrlHD.md5",
		},
	}

	nzb := NewNZBPreprocessor()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := nzb.Clean(tt.input)
			if result != tt.expected {
				t.Errorf("Clean() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestNZBPreprocessor_IsNZBFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{`[001/279] "file.mkv"`, true},
		{`[22569]-[FULL]-[#channel] - "file.mkv" yEnc`, true},
		{`(TR-1080p)[01/80] - "file.mkv"`, true},
		{`<site.ws> - content - "file.mkv"`, true},
		{`Movie.2020.1080p.BluRay.mkv`, false},
		{`Breaking.Bad.S01E01.mkv`, false},
	}

	nzb := NewNZBPreprocessor()
	for _, tt := range tests {
		t.Run(tt.input[:min(30, len(tt.input))], func(t *testing.T) {
			result := nzb.IsNZBFormat(tt.input)
			if result != tt.expected {
				t.Errorf("IsNZBFormat(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
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

func TestMusicParser_ParseAlbumTitle(t *testing.T) {
	mp := NewMusicParser()

	tests := []struct {
		name  string
		title string
		want  struct {
			artist string
			album  string
			year   int
			format string
		}
	}{
		{
			name:  "Standard format with spaces",
			title: "Alabama Shakes - At The Loveless Barn (2014) FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Alabama Shakes",
				album:  "At The Loveless Barn",
				year:   2014,
			},
		},
		{
			name:  "Scene release format",
			title: "Alan Menken And Howard Ashman-Little Shop Of Horrors-OST-CD-FLAC-1986-KINDA-AUD",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Alan Menken And Howard Ashman",
				album:  "Little Shop Of Horrors",
				year:   1986,
			},
		},
		{
			name:  "Scene release with WEB source",
			title: "Howard Ashman and Alan Menken-Little Shop Of Horrors-The New Off-Broadway Cast Album-OST-16BIT-WEB-FLAC-2019-KINDA",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				// Note: When multiple artists are detected (via "and"), Artist is set to the first one
				// The full artist string is stored in AlbumArtist
				artist: "Howard Ashman",
				album:  "Little Shop Of Horrors The New Off Broadway Cast Album",
				year:   2019,
			},
		},
		{
			name:  "Multiple artists with ampersand",
			title: "Daft Punk & Pharrell Williams - Get Lucky (2013) MP3",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Daft Punk",
				album:  "Get Lucky",
				year:   2013,
			},
		},
		{
			name:  "Album with year in brackets",
			title: "Radiohead - OK Computer [1997] FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Radiohead",
				album:  "OK Computer",
				year:   1997,
			},
		},
		{
			name:  "Scene release with underscore",
			title: "Pink_Floyd-The_Dark_Side_of_the_Moon-REMASTERED-CD-FLAC-2011-GROUP",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Pink Floyd",
				album:  "The Dark Side of the Moon",
				year:   2011,
			},
		},
		{
			name:  "Simple format no year",
			title: "The Beatles - Abbey Road FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "The Beatles",
				album:  "Abbey Road",
				year:   0,
			},
		},
		{
			name:  "24-bit hi-res release",
			title: "Miles Davis - Kind of Blue (1959) 24BIT-96KHZ FLAC",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Miles Davis",
				album:  "Kind of Blue",
				year:   1959,
			},
		},
		{
			name:  "Scene format with dots as separators",
			title: "Alan.Silvestri-Predator.2-OST-1990-EOS",
			want: struct {
				artist string
				album  string
				year   int
				format string
			}{
				artist: "Alan Silvestri",
				album:  "Predator 2",
				year:   1990,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mp.ParseAlbumTitle(tt.title)

			if result.Artist != tt.want.artist {
				t.Errorf("Artist = %q, want %q", result.Artist, tt.want.artist)
			}
			if result.Album != tt.want.album {
				t.Errorf("Album = %q, want %q", result.Album, tt.want.album)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
		})
	}
}

func TestAudiobookParser_Parse_NZBTitles(t *testing.T) {
	ap := NewAudiobookParser()

	tests := []struct {
		name  string
		title string
		want  struct {
			author   string
			title    string
			year     int
			narrator string
			series   string
			asin     string
		}
	}{
		{
			name:  "Standard format with spaces",
			title: "Stephen King - The Shining (1977)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Stephen King",
				title:  "The Shining",
				year:   1977,
			},
		},
		{
			name:  "With narrator",
			title: "Brandon Sanderson - Mistborn Read by Michael Kramer (2006)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author:   "Brandon Sanderson",
				title:    "Mistborn",
				year:     2006,
				narrator: "Michael Kramer",
			},
		},
		{
			name:  "With ASIN",
			title: "Terry Pratchett - Guards Guards [B00354ZSS2]",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Terry Pratchett",
				title:  "Guards Guards",
				asin:   "B00354ZSS2",
			},
		},
		{
			name:  "With series info",
			title: "Jim Butcher - Storm Front (Dresden Files Book 1) (2000)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Jim Butcher",
				title:  "Storm Front",
				year:   2000,
				series: "Dresden Files",
			},
		},
		{
			name:  "Scene format with dashes",
			title: "Neil Gaiman-American Gods-Audiobook-MP3-64kbps-2001",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Neil Gaiman",
				title:  "American Gods",
				year:   2001,
			},
		},
		{
			name:  "Lowercase author name",
			title: "stephen king - it (1986)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "stephen king",
				title:  "it",
				year:   1986,
			},
		},
		{
			name:  "Multiple authors",
			title: "Douglas Adams & Eoin Colfer - And Another Thing (2009)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Douglas Adams",
				title:  "And Another Thing",
				year:   2009,
			},
		},
		{
			name:  "Unabridged indicator",
			title: "Frank Herbert - Dune [Unabridged] (1965)",
			want: struct {
				author   string
				title    string
				year     int
				narrator string
				series   string
				asin     string
			}{
				author: "Frank Herbert",
				title:  "Dune",
				year:   1965,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ap.Parse(tt.title)

			if result.Author != tt.want.author {
				t.Errorf("Author = %q, want %q", result.Author, tt.want.author)
			}
			if result.Title != tt.want.title {
				t.Errorf("Title = %q, want %q", result.Title, tt.want.title)
			}
			if tt.want.year != 0 && result.Year != tt.want.year {
				t.Errorf("Year = %d, want %d", result.Year, tt.want.year)
			}
			if tt.want.narrator != "" && result.Narrator != tt.want.narrator {
				t.Errorf("Narrator = %q, want %q", result.Narrator, tt.want.narrator)
			}
			if tt.want.series != "" && result.Series != tt.want.series {
				t.Errorf("Series = %q, want %q", result.Series, tt.want.series)
			}
			if tt.want.asin != "" && result.ASIN != tt.want.asin {
				t.Errorf("ASIN = %q, want %q", result.ASIN, tt.want.asin)
			}
		})
	}
}

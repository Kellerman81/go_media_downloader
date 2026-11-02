package parser_v2

import (
	"testing"
)

func TestMatchTotalRuntime(t *testing.T) {
	rm := DefaultRuntimeMatcher()
	tracks := []TrackInfo{{RuntimeMS: 600_000}, {RuntimeMS: 400_000}}

	if ok, conf := rm.MatchTotalRuntime(1_000_000, tracks); !ok || conf != 1 {
		t.Errorf("exact match = %t %v", ok, conf)
	}

	// 3% of 1,000,000 ms is 30 s; 15 s off gives half confidence.
	if ok, conf := rm.MatchTotalRuntime(1_015_000, tracks); !ok || conf < 0.49 || conf > 0.51 {
		t.Errorf("half tolerance = %t %v", ok, conf)
	}

	if ok, conf := rm.MatchTotalRuntime(1_031_000, tracks); ok || conf != 0 {
		t.Errorf("outside tolerance = %t %v", ok, conf)
	}

	// Short runtimes get the 5 s minimum, long ones the 5 min maximum.
	short := []TrackInfo{{RuntimeMS: 60_000}}
	if ok, _ := rm.MatchTotalRuntime(64_000, short); !ok {
		t.Error("minimum tolerance not applied")
	}

	long := []TrackInfo{{RuntimeMS: 36_000_000}}
	if ok, _ := rm.MatchTotalRuntime(36_400_000, long); ok {
		t.Error("maximum tolerance not applied")
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

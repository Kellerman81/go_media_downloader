package parser_v2

import (
	"testing"
)

func TestNZBCleanBracketPrefixes(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		// The release name in a padded bracket is content, not metadata.
		{
			`[22569]-[FULL]-[#a.b.hdtv.x264@EFNet]-[ The.Matrix.1999 ]-[136/143] - "file.par2" yEnc`,
			"The.Matrix.1999",
		},
		// Post id chained with a dash is still stripped even when it looks like a year.
		{`[2019]-[FULL]-[#a.b.hdtv@EFNet]-[ Movie.2019.1080p ]-[01/10] - "x.par2" yEnc`, "Movie.2019.1080p"},
		// Group tags are stripped.
		{`[Sub-Group] Title S01E01 1080p.mkv`, "Title S01E01 1080p.mkv"},
	}

	n := NewNZBPreprocessor()
	for _, tt := range tests {
		if got := n.Clean(tt.input); got != tt.want {
			t.Errorf("Clean(%q) = %q; want %q", tt.input, got, tt.want)
		}
	}
}

func TestNZBBracketTitleNotTreatedAsUsenet(t *testing.T) {
	n := NewNZBPreprocessor()
	for _, in := range []string{
		"[2019] Some Title 1080p.mkv",
		"[Kinder] Die Biene Maja 1975 German.mkv",
		"Title [Director's Cut] 2019.mkv",
	} {
		if n.IsNZBFormat(in) {
			t.Errorf("IsNZBFormat(%q) = true; the bracket is part of the name", in)
		}
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

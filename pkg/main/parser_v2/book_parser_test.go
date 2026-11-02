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

func BenchmarkBookParser_Parse(b *testing.B) {
	bp := NewBookParser()
	filename := "Brandon Sanderson - Mistborn Book 1 - The Final Empire (2006) [978-0-7653-1178-8] [Retail].epub"

	for b.Loop() {
		bp.Parse(filename)
	}
}

func TestRegression_Book(t *testing.T) {
	bp := NewBookParser()
	tests := []struct {
		input    string
		author   string
		title    string
		isbn13   string
		isbn10   string
		series   string
		position string
	}{
		{"J.K. Rowling - Harry Potter [B012345678]", "J.K. Rowling", "Harry Potter", "", "", "", ""},
		{"Title 9781234567890 2020", "", "Title 9781234567890 2020", "", "", "", ""},
		{"Title 9780306406157 2020.epub", "", "Title 2020", "9780306406157", "", "", ""},
		{"Title 978-0-306-40615-7.epub", "", "Title", "9780306406157", "", "", ""},
		{"Title 0306406152.epub", "", "Title", "", "0306406152", "", ""},
		{"Title 0306406153.epub", "", "Title 0306406153", "", "", "", ""},
		{"Author - Casino 2.epub", "Author", "Casino 2", "", "", "", ""},
		{"Author - Piano 3.epub", "Author", "Piano 3", "", "", "", ""},
		{"Author - Volcano 3.epub", "Author", "Volcano 3", "", "", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			r := bp.Parse(tt.input)
			if r.Author != tt.author || r.Title != tt.title || r.ISBN13 != tt.isbn13 ||
				r.ISBN10 != tt.isbn10 || r.Series != tt.series || r.SeriesPosition != tt.position {
				t.Errorf("got author=%q title=%q isbn13=%q isbn10=%q series=%q pos=%q",
					r.Author, r.Title, r.ISBN13, r.ISBN10, r.Series, r.SeriesPosition)
			}
		})
	}
}

func TestBookParseWithPathAuthorFromDirs(t *testing.T) {
	bp := NewBookParser()

	r := bp.ParseWithPath("/books/Brandon Sanderson/The Way of Kings.epub")
	if r.Author != "Brandon Sanderson" || r.SourcePath == "" {
		t.Errorf("author from parent = %q", r.Author)
	}

	// Any capitalised folder passes as an author, so the grandparent is only
	// used when the parent does not look like a name.
	r = bp.ParseWithPath("/books/Brandon Sanderson/stormlight/The Way of Kings.epub")
	if r.Author != "Brandon Sanderson" || r.Series == "" {
		t.Errorf("author/series from grandparent = %q / %q", r.Author, r.Series)
	}
}

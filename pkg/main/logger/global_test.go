package logger

import (
	"context"
	"testing"
	"time"
)

func TestStringToSlug(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Simple string", "hello world", "hello-world"},
		{"Special characters", "Hello & World!", "hello-and-world"},
		{"Multiple spaces", "hello   world", "hello-world"},
		{"German umlauts", "München Straße", "muenchen-strasse"},
		{"HTML entities", "Black & White", "black-and-white"},
		{"Multiple hyphens", "hello---world", "hello-world"},
		{"Leading/trailing hyphens", "-hello world-", "hello-world"},
		{"Unicode characters", "café", "cafe"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToSlug(tt.input)
			if result != tt.expected {
				t.Errorf("StringToSlug(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestTimeAfter(t *testing.T) {
	tests := []struct {
		name     string
		time1    time.Time
		time2    time.Time
		expected bool
	}{
		{
			name:     "Same time",
			time1:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			time2:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "First time after",
			time1:    time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			time2:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
		{
			name:     "First time before",
			time1:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			time2:    time.Date(2023, 1, 2, 0, 0, 0, 0, time.UTC),
			expected: false,
		},
		{
			name:     "Nanosecond difference",
			time1:    time.Date(2023, 1, 1, 0, 0, 0, 1, time.UTC),
			time2:    time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TimeAfter(tt.time1, tt.time2)
			if result != tt.expected {
				t.Errorf(
					"TimeAfter(%v, %v) = %v, expected %v",
					tt.time1,
					tt.time2,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestCheckContextEnded(t *testing.T) {
	tests := []struct {
		name        string
		setupCtx    func() context.Context
		expectError bool
	}{
		{
			name: "Active context",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expectError: false,
		},
		{
			name: "Canceled context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx
			},
			expectError: true,
		},
		{
			name: "Timeout context",
			setupCtx: func() context.Context {
				ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
				defer cancel()
				time.Sleep(time.Millisecond * 2)
				return ctx
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			err := CheckContextEnded(ctx)
			if (err != nil) != tt.expectError {
				t.Errorf("CheckContextEnded() error = %v, expectError %v", err, tt.expectError)
			}
		})
	}
}

func TestStringToInt(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"Empty string", "", 0},
		{"Zero string", "0", 0},
		{"Simple integer", "123", 123},
		{"Negative integer", "-123", -123},
		{"Float string", "123.45", 123},
		{"Invalid string", "abc", 0},
		{"Mixed string", "123abc", 0},
		{"Comma decimal", "123,45", 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToInt(tt.input)
			if result != tt.expected {
				t.Errorf("StringToInt(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestJoinStrings(t *testing.T) {
	tests := []struct {
		name     string
		inputs   []string
		expected string
	}{
		{"No strings", []string{}, ""},
		{"Single string", []string{"hello"}, "hello"},
		{"Two strings", []string{"hello", "world"}, "helloworld"},
		{"Empty strings", []string{"", "", ""}, ""},
		{"Mixed empty strings", []string{"hello", "", "world"}, "helloworld"},
		{"Multiple strings", []string{"hello", "beautiful", "world"}, "hellobeautifulworld"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := JoinStrings(tt.inputs...)
			if result != tt.expected {
				t.Errorf("JoinStrings(%v) = %q, expected %q", tt.inputs, result, tt.expected)
			}
		})
	}
}

func TestIntToString(t *testing.T) {
	tests := []struct {
		name     string
		input    uint16
		expected string
	}{
		{"Zero value", 0, "0"},
		{"Small number", 42, "42"},
		{"Max uint16", 65535, "65535"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IntToString(tt.input)
			if result != tt.expected {
				t.Errorf("IntToString(%d) = %s; want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAddBuffer_WriteInt(t *testing.T) {
	tests := []struct {
		name     string
		input    int
		expected string
	}{
		{"Negative number", -42, "-42"},
		{"Zero", 0, "0"},
		{"Positive number", 12345, "12345"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := PlAddBuffer.Get()
			b.WriteInt(tt.input)
			if b.String() != tt.expected {
				t.Errorf("WriteInt(%d) = %s; want %s", tt.input, b.String(), tt.expected)
			}
			PlAddBuffer.Put(b)
		})
	}
}

func TestAddBuffer_WriteUInt16(t *testing.T) {
	tests := []struct {
		name     string
		input    uint16
		expected string
	}{
		{"Zero", 0, "0"},
		{"Small number", 123, "123"},
		{"Max uint16", 65535, "65535"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := PlAddBuffer.Get()
			b.WriteUInt16(tt.input)
			if b.String() != tt.expected {
				t.Errorf("WriteUInt16(%d) = %s; want %s", tt.input, b.String(), tt.expected)
			}
			PlAddBuffer.Put(b)
		})
	}
}

func TestAddBuffer_WriteUInt(t *testing.T) {
	tests := []struct {
		name     string
		input    uint
		expected string
	}{
		{"Zero", 0, "0"},
		{"Small number", 123, "123"},
		{"Large number", 4294967295, "4294967295"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := PlAddBuffer.Get()
			b.WriteUInt(tt.input)
			if b.String() != tt.expected {
				t.Errorf("WriteUInt(%d) = %s; want %s", tt.input, b.String(), tt.expected)
			}
			PlAddBuffer.Put(b)
		})
	}
}

func TestAddBuffer_WriteStringMap(t *testing.T) {
	tests := []struct {
		name      string
		useseries bool
		typestr   string
		expected  string
	}{
		{"Series map", true, "test", ""},
		{"Movies map", false, "test", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := PlAddBuffer.Get()
			b.WriteStringMap(tt.useseries, tt.typestr)
			result := b.String()
			if result != GetStringsMap(tt.useseries, tt.typestr) {
				t.Errorf(
					"WriteStringMap(%v, %s) wrote %s; want %s",
					tt.useseries,
					tt.typestr,
					result,
					tt.expected,
				)
			}
			PlAddBuffer.Put(b)
		})
	}
}

func TestAddBuffer_WriteURL(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Simple string", "hello world", "hello+world"},
		{"Special characters", "test?&=", "test%3F%26%3D"},
		{"Unicode characters", "テスト", "%E3%83%86%E3%82%B9%E3%83%88"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := PlAddBuffer.Get()
			b.WriteURL(tt.input)
			if b.String() != tt.expected {
				t.Errorf("WriteURL(%s) = %s; want %s", tt.input, b.String(), tt.expected)
			}
			PlAddBuffer.Put(b)
		})
	}
}

func TestParseStringTemplate(t *testing.T) {
	tests := []struct {
		name        string
		message     string
		messageData any
		wantErr     bool
		want        string
	}{
		{
			name:        "Empty message",
			message:     "",
			messageData: nil,
			wantErr:     false,
			want:        "",
		},
		{
			name:        "Simple template",
			message:     "Hello {{.Name}}",
			messageData: struct{ Name string }{"World"},
			wantErr:     false,
			want:        "Hello World",
		},
		{
			name:        "Invalid template syntax",
			message:     "Hello {{.Name",
			messageData: struct{ Name string }{"World"},
			wantErr:     true,
			want:        "",
		},
		{
			name:        "Template with missing field",
			message:     "Hello {{.Missing}}",
			messageData: struct{ Name string }{"World"},
			wantErr:     true,
			want:        "",
		},
		{
			name:        "Complex template",
			message:     "{{.First}} {{.Second}} {{.Third}}",
			messageData: struct{ First, Second, Third string }{"One", "Two", "Three"},
			wantErr:     false,
			want:        "One Two Three",
		},
		{
			name:        "Template with numbers",
			message:     "Count: {{.Count}}",
			messageData: struct{ Count int }{42},
			wantErr:     false,
			want:        "Count: 42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotErr, gotResult := ParseStringTemplate(tt.message, tt.messageData)
			if gotErr != tt.wantErr {
				t.Errorf("ParseStringTemplate() error = %v, wantErr %v", gotErr, tt.wantErr)
			}
			if gotResult != tt.want {
				t.Errorf("ParseStringTemplate() = %v, want %v", gotResult, tt.want)
			}
		})
	}
}

func TestCheckhtmlentities(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "No HTML entities",
			input:    "Hello world",
			expected: "Hello world",
		},
		{
			name:     "Ampersand without entity",
			input:    "Hello & world",
			expected: "Hello & world",
		},
		{
			name:     "HTML entity with semicolon",
			input:    "Hello & world",
			expected: "Hello & world",
		},
		{
			name:     "Multiple HTML entities",
			input:    "Copyright © 2023 & all rights reserved",
			expected: "Copyright © 2023 & all rights reserved",
		},
		{
			name:     "Partial HTML entity",
			input:    "Hello &am world",
			expected: "Hello &am world",
		},
		{
			name:     "HTML entity without semicolon",
			input:    "Hello & world",
			expected: "Hello & world",
		},
		{
			name:     "Special characters entity",
			input:    "<div>",
			expected: "<div>",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only ampersand",
			input:    "&",
			expected: "&",
		},
		{
			name:     "Only semicolon",
			input:    ";",
			expected: ";",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Checkhtmlentities(tt.input)
			if result != tt.expected {
				t.Errorf("Checkhtmlentities(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestAddImdbPrefixP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Empty string", "", ""},
		{"Already has tt prefix", "tt1234567", "tt1234567"},
		{"Already has TT prefix", "TT1234567", "TT1234567"},
		{"No prefix", "1234567", "tt1234567"},
		{"Single character", "1", "tt1"},
		{"Invalid IMDb ID", "abc", "ttabc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AddImdbPrefix(tt.input)
			if result != tt.expected {
				t.Errorf("AddImdbPrefixP(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestPath(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		allowslash bool
		expected   string
	}{
		{"Empty string", "", true, ""},
		{"Simple path", "test/path", true, "test/path"},
		{"Path with backslashes", "test\\path", true, "testpath"},
		{"Path without slashes", "test/path", false, "test/path"},
		{"Path with diacritics", "tést/päth", true, "tést/paeth"},
		{"Path with special chars", "test$path", true, "test$path"},
		{"Double slashes", "test//path", true, "test//path"},
		{"Dots in path", "../test/./path", true, "../test/./path"},
		{"Mixed slashes", "test\\//path", true, "test/path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			Path(&input, tt.allowslash)
			if input != tt.expected {
				t.Errorf(
					"Path(%q, %v) = %q, expected %q",
					tt.input,
					tt.allowslash,
					input,
					tt.expected,
				)
			}
		})
	}
}

func TestTrim(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cutset   []rune
		expected string
	}{
		{"Empty string", "", []rune{' '}, ""},
		{"Empty cutset", "test", []rune{}, "test"},
		{"Multiple cutset chars", "---test---", []rune{'-'}, "test"},
		{"Mixed cutset chars", "-_-test-_-", []rune{'-', '_'}, "test"},
		{"No matching cutset", "test", []rune{'x'}, "test"},
		{"Single char string", "x", []rune{'x'}, "x"},
		{"All cutset chars", "---", []rune{'-'}, "---"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Trim(tt.input, tt.cutset...)
			if result != tt.expected {
				t.Errorf("Trim(%q, %v) = %q, expected %q", tt.input, tt.cutset, result, tt.expected)
			}
		})
	}
}

func TestTrimLeft(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cutset   []rune
		expected string
	}{
		{"Empty string", "", []rune{' '}, ""},
		{"Empty cutset", "test", []rune{}, "test"},
		{"Leading chars only", "---test", []rune{'-'}, "test"},
		{"Mixed leading chars", "-_-test", []rune{'-', '_'}, "test"},
		{"No leading chars", "test---", []rune{'-'}, "test---"},
		{"Single char string", "x", []rune{'x'}, "x"},
		{"All cutset chars", "---", []rune{'-'}, "---"},
		{"Multiple different chars", "--_test", []rune{'-', '_'}, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimLeft(tt.input, tt.cutset...)
			if result != tt.expected {
				t.Errorf(
					"TrimLeft(%q, %v) = %q, expected %q",
					tt.input,
					tt.cutset,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestTrimRight(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		cutset   []rune
		expected string
	}{
		{"Empty string", "", []rune{' '}, ""},
		{"Empty cutset", "test", []rune{}, "test"},
		{"Trailing chars only", "test---", []rune{'-'}, "test"},
		{"Mixed trailing chars", "test-_-", []rune{'-', '_'}, "test"},
		{"No trailing chars", "---test", []rune{'-'}, "---test"},
		{"Single char string", "x", []rune{'x'}, "x"},
		{"All cutset chars", "---", []rune{'-'}, "---"},
		{"Multiple different chars", "test--_", []rune{'-', '_'}, "test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := TrimRight(tt.input, tt.cutset...)
			if result != tt.expected {
				t.Errorf(
					"TrimRight(%q, %v) = %q, expected %q",
					tt.input,
					tt.cutset,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestContainsI(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected bool
	}{
		{"Empty strings", "", "", true},
		{"Empty search string", "hello", "", true},
		{"Empty source string", "", "hello", false},
		{"Exact match", "hello world", "hello", true},
		{"Case insensitive match", "Hello World", "hello", true},
		{"Mixed case match", "HeLLo WoRLD", "HELLO", true},
		{"No match", "hello world", "goodbye", false},
		{"Partial match", "hello world", "lo wo", true},
		{"Unicode match", "café", "CAFÉ", true},
		{"Special chars match", "Hello! World?", "HELLO!", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsI(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ContainsI(%q, %q) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestContainsByteI(t *testing.T) {
	tests := []struct {
		name     string
		a        []byte
		b        []byte
		expected bool
	}{
		{"Empty slices", []byte{}, []byte{}, true},
		{"Empty search slice", []byte("hello"), []byte{}, true},
		{"Empty source slice", []byte{}, []byte("hello"), false},
		{"Exact match", []byte("hello world"), []byte("hello"), true},
		{"Case insensitive match", []byte("Hello World"), []byte("hello"), true},
		{"Mixed case match", []byte("HeLLo WoRLD"), []byte("HELLO"), true},
		{"No match", []byte("hello world"), []byte("goodbye"), false},
		{"Partial match", []byte("hello world"), []byte("lo wo"), true},
		{"Special chars match", []byte("Hello! World?"), []byte("HELLO!"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsByteI(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ContainsByteI(%q, %q) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestContainsInt(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        uint16
		expected bool
	}{
		{"Empty string", "", 123, false},
		{"Single digit match", "test5test", 5, true},
		{"Multiple digit match", "test12345test", 12345, true},
		{"No match", "test123test", 456, false},
		{"Partial match", "test12345test", 123, true},
		{"Max uint16", "test65535test", 65535, true},
		{"Zero value", "test0test", 0, true},
		{"Multiple occurrences", "123123123", 123, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ContainsInt(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("ContainsInt(%q, %d) = %v, expected %v", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestHasPrefixI(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefix   string
		expected bool
	}{
		{"Empty strings", "", "", true},
		{"Empty prefix", "hello", "", true},
		{"Empty string", "", "hello", false},
		{"Exact match", "hello world", "hello", true},
		{"Case insensitive match", "Hello World", "hello", true},
		{"Mixed case match", "HeLLo WoRLD", "HELLO", true},
		{"No match", "hello world", "goodbye", false},
		{"Partial match not at start", "world hello", "hello", false},
		{"Unicode match", "CAFÉ latte", "café", true},
		{"Special chars match", "!Hello World", "!hello", true},
		{"Longer prefix than string", "hi", "hello", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPrefixI(tt.s, tt.prefix)
			if result != tt.expected {
				t.Errorf(
					"HasPrefixI(%q, %q) = %v, expected %v",
					tt.s,
					tt.prefix,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestHasSuffixI(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		suffix   string
		expected bool
	}{
		{"Empty strings", "", "", true},
		{"Empty suffix", "hello", "", true},
		{"Empty string", "", "hello", false},
		{"Exact match", "world hello", "hello", true},
		{"Case insensitive match", "World HELLO", "hello", true},
		{"Mixed case match", "WoRLD HeLLo", "HELLO", true},
		{"No match", "hello world", "goodbye", false},
		{"Partial match not at end", "hello world", "hell", false},
		{"Unicode match", "latté CAFÉ", "café", true},
		{"Special chars match", "Hello World!", "world!", true},
		{"Longer suffix than string", "hi", "hello", false},
		{"Single character match", "a", "A", true},
		{"Multiple occurrences, match at end", "hellohellohello", "hello", true},
		{"Numbers in suffix", "Test123", "123", true},
		{"Whitespace suffix", "hello ", " ", true},
		{"Special characters case match", "Hello???", "???", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasSuffixI(tt.s, tt.suffix)
			if result != tt.expected {
				t.Errorf(
					"HasSuffixI(%q, %q) = %v, expected %v",
					tt.s,
					tt.suffix,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestIndexI(t *testing.T) {
	tests := []struct {
		name     string
		a        string
		b        string
		expected int
	}{
		{"Empty strings", "", "", 0},
		{"Empty search string", "hello", "", 0},
		{"Empty source string", "", "hello", -1},
		{"ASCII match with diacritics", "München", "münchen", 0},
		{"Mixed case with special chars", "HéLLo WöRLD", "héllo wörld", 0},
		{"Non-ASCII characters", "こんにちは", "こん", 0},
		{"Partial match with diacritics", "Café au lait", "café", 0},
		{"No match with special chars", "Hello World", "xyz", -1},
		{"Match with German umlauts", "Über allen Gipfeln", "über", 0},
		{"Multiple possible matches", "hello hello HELLO", "hello", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IndexI(tt.a, tt.b)
			if result != tt.expected {
				t.Errorf("IndexI(%q, %q) = %d, expected %d", tt.a, tt.b, result, tt.expected)
			}
		})
	}
}

func TestInt64ToUint(t *testing.T) {
	tests := []struct {
		name     string
		input    int64
		expected uint
	}{
		{"Zero value", 0, 0},
		{"Positive value", 42, 42},
		{"Max int32", int64(^uint32(0) >> 1), uint(^uint32(0) >> 1)},
		{"Negative value", -42, 0},
		{"Large positive value", int64(1) << 40, uint(1) << 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Int64ToUint(tt.input)
			if result != tt.expected {
				t.Errorf("Int64ToUint(%d) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringToDuration(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected time.Duration
	}{
		{"Empty string", "", 0},
		{"Zero string", "0", 0},
		{"Integer duration", "1000", time.Duration(1000)},
		{"Float duration", "1.5", time.Duration(1)},
		{"Invalid string", "abc", 0},
		{"Negative duration", "-1000", time.Duration(-1000)},
		{"Float with comma", "1,5", time.Duration(1)},
		{"Large duration", "9223372036854775807", time.Duration(9223372036854775807)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := time.Duration(StringToDuration(tt.input))
			if result != tt.expected {
				t.Errorf("StringToDuration(%q) = %v, expected %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringToInt32(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int32
	}{
		{"Empty string", "", 0},
		{"Zero string", "0", 0},
		{"Positive integer", "2147483647", 2147483647},
		{"Negative integer", "-2147483648", -2147483648},
		{"Float string", "123.45", 123},
		{"Invalid string", "abc", 0},
		{"Float with comma", "123,45", 123},
		{"Overflow value", "2147483648", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToInt32(tt.input)
			if result != tt.expected {
				t.Errorf("StringToInt32(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringToUInt16(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint16
	}{
		{"Empty string", "", 0},
		{"Zero string", "0", 0},
		{"Valid value", "65534", 65534},
		{"Invalid string", "abc", 0},
		{"Negative value", "-1", 0},
		{"Overflow value", "65536", 0},
		{"Small value", "256", 256},
		{"Leading zeros", "00123", 123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToUInt16(tt.input)
			if result != tt.expected {
				t.Errorf("StringToUInt16(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStringToInt64(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"Empty string", "", 0},
		{"Zero string", "0", 0},
		{"Max int64", "9223372036854775807", 9223372036854775807},
		{"Min int64", "-9223372036854775808", -9223372036854775808},
		{"Invalid string", "abc", 0},
		{"Overflow string", "9223372036854775808", 0},
		{"Underflow string", "-9223372036854775809", 0},
		{"Valid positive", "123456789", 123456789},
		{"Valid negative", "-123456789", -123456789},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringToInt64(tt.input)
			if result != tt.expected {
				t.Errorf("StringToInt64(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSlicesContainsI(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		value    string
		expected bool
	}{
		{
			name:     "Empty slice",
			slice:    []string{},
			value:    "test",
			expected: false,
		},
		{
			name:     "Exact match",
			slice:    []string{"test", "example", "sample"},
			value:    "test",
			expected: true,
		},
		{
			name:     "Case insensitive match",
			slice:    []string{"TEST", "Example", "Sample"},
			value:    "test",
			expected: true,
		},
		{
			name:     "Mixed case match",
			slice:    []string{"TeSt", "ExAmPlE", "SaMpLe"},
			value:    "TEST",
			expected: true,
		},
		{
			name:     "No match",
			slice:    []string{"test", "example", "sample"},
			value:    "missing",
			expected: false,
		},
		{
			name:     "Single element slice",
			slice:    []string{"test"},
			value:    "TEST",
			expected: true,
		},
		{
			name:     "Empty string value",
			slice:    []string{"test", "example", ""},
			value:    "",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SlicesContainsI(tt.slice, tt.value)
			if result != tt.expected {
				t.Errorf(
					"SlicesContainsI(%v, %q) = %v, expected %v",
					tt.slice,
					tt.value,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestSlicesContainsPart2I(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		value    string
		expected bool
	}{
		{
			name:     "Empty slice",
			slice:    []string{},
			value:    "test",
			expected: false,
		},
		{
			name:     "Substring match",
			slice:    []string{"test", "example"},
			value:    "this is a test case",
			expected: true,
		},
		{
			name:     "Case insensitive substring",
			slice:    []string{"TEST", "Example"},
			value:    "this is a test case",
			expected: true,
		},
		{
			name:     "No substring match",
			slice:    []string{"missing", "notfound"},
			value:    "this is a test case",
			expected: false,
		},
		{
			name:     "Empty string in slice",
			slice:    []string{""},
			value:    "test",
			expected: true,
		},
		{
			name:     "Empty value string",
			slice:    []string{"test", "example"},
			value:    "",
			expected: false,
		},
		{
			name:     "Partial word match",
			slice:    []string{"est"},
			value:    "test",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SlicesContainsPart2I(tt.slice, tt.value)
			if result != tt.expected {
				t.Errorf(
					"SlicesContainsPart2I(%v, %q) = %v, expected %v",
					tt.slice,
					tt.value,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestStringRemoveAllRunesP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		runes    []byte
		expected string
	}{
		{
			name:     "Nil string pointer",
			input:    "",
			runes:    []byte{'a'},
			expected: "",
		},
		{
			name:     "Empty string",
			input:    "",
			runes:    []byte{'a', 'b'},
			expected: "",
		},
		{
			name:     "Empty runes",
			input:    "test",
			runes:    []byte{},
			expected: "test",
		},
		{
			name:     "Single rune not present",
			input:    "test",
			runes:    []byte{'x'},
			expected: "test",
		},
		{
			name:     "Single rune present",
			input:    "test",
			runes:    []byte{'t'},
			expected: "es",
		},
		{
			name:     "Multiple runes present",
			input:    "test string",
			runes:    []byte{'t', 's'},
			expected: "e ring",
		},
		{
			name:     "All characters to be removed",
			input:    "aaa",
			runes:    []byte{'a'},
			expected: "",
		},
		{
			name:     "Mixed case characters",
			input:    "Test String",
			runes:    []byte{'t', 'S'},
			expected: "Tes ring",
		},
		{
			name:     "Special characters",
			input:    "test!@#string",
			runes:    []byte{'!', '@', '#'},
			expected: "teststring",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			StringRemoveAllRunesP(&input, tt.runes...)
			if input != tt.expected {
				t.Errorf(
					"StringRemoveAllRunesP(%q, %v) = %q, expected %q",
					tt.input,
					tt.runes,
					input,
					tt.expected,
				)
			}
		})
	}
}

func TestStringReplaceWith(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		r        byte
		t        byte
		expected string
	}{
		{
			name:     "Empty string",
			input:    "",
			r:        'a',
			t:        'b',
			expected: "",
		},
		{
			name:     "No matches",
			input:    "hello",
			r:        'x',
			t:        'y',
			expected: "hello",
		},
		{
			name:     "Single replacement",
			input:    "hello",
			r:        'l',
			t:        'w',
			expected: "hewwo",
		},
		{
			name:     "Multiple replacements",
			input:    "mississippi",
			r:        'i',
			t:        'e',
			expected: "messesseppe",
		},
		{
			name:     "Replace with same character",
			input:    "test",
			r:        't',
			t:        't',
			expected: "test",
		},
		{
			name:     "Replace special characters",
			input:    "test\ntest",
			r:        '\n',
			t:        ' ',
			expected: "test test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringReplaceWith(tt.input, tt.r, tt.t)
			if result != tt.expected {
				t.Errorf(
					"StringReplaceWith(%q, %q, %q) = %q, expected %q",
					tt.input,
					tt.r,
					tt.t,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestStringReplaceWithP(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		r        byte
		t        byte
		expected string
	}{
		{
			name:     "Nil string pointer",
			input:    "",
			r:        'a',
			t:        'b',
			expected: "",
		},
		{
			name:     "Replace all occurrences",
			input:    "aaa",
			r:        'a',
			t:        'b',
			expected: "bbb",
		},
		{
			name:     "Mixed content",
			input:    "a1a2a3",
			r:        'a',
			t:        'x',
			expected: "x1x2x3",
		},
		{
			name:     "Replace with space",
			input:    "hello_world",
			r:        '_',
			t:        ' ',
			expected: "hello world",
		},
		{
			name:     "Replace numbers",
			input:    "12321",
			r:        '2',
			t:        '0',
			expected: "10301",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := tt.input
			StringReplaceWithP(&input, tt.r, tt.t)
			if input != tt.expected {
				t.Errorf(
					"StringReplaceWithP(%q, %q, %q) = %q, expected %q",
					tt.input,
					tt.r,
					tt.t,
					input,
					tt.expected,
				)
			}
		})
	}
}

func TestStringReplaceWithStr(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		r        string
		t        string
		expected string
	}{
		{
			name:     "Empty strings",
			s:        "",
			r:        "old",
			t:        "new",
			expected: "",
		},
		{
			name:     "Replace empty string",
			s:        "abc",
			r:        "",
			t:        "x",
			expected: "abc",
		},
		{
			name:     "Replace with empty string",
			s:        "hello world",
			r:        "o",
			t:        "",
			expected: "hell wrld",
		},
		{
			name:     "Multiple word replacement",
			s:        "the quick brown fox",
			r:        "quick",
			t:        "slow",
			expected: "the slow brown fox",
		},
		{
			name:     "Overlapping patterns",
			s:        "aaaaa",
			r:        "aa",
			t:        "b",
			expected: "bba",
		},
		{
			name:     "Unicode replacement",
			s:        "hello 世界",
			r:        "世界",
			t:        "world",
			expected: "hello world",
		},
		{
			name:     "Replace longer with shorter",
			s:        "hello world",
			r:        "world",
			t:        "all",
			expected: "hello all",
		},
		{
			name:     "Replace shorter with longer",
			s:        "hi world",
			r:        "hi",
			t:        "hello",
			expected: "hello world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StringReplaceWithStr(tt.s, tt.r, tt.t)
			if result != tt.expected {
				t.Errorf(
					"StringReplaceWithStr(%q, %q, %q) = %q, expected %q",
					tt.s,
					tt.r,
					tt.t,
					result,
					tt.expected,
				)
			}
		})
	}
}

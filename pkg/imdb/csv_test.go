package main

import (
	"html"
	"strconv"
	"strings"
	"testing"
)

// Original functions (current implementation)
func csvgetuint32arr_original(record string) uint32 {
	if record == "" || record == "0" || record == "\\N" {
		return 0
	}
	getint, err := strconv.ParseUint(strings.TrimLeft(record, "t"), 10, 0)
	if err != nil {
		return 0
	}
	return uint32(getint)
}

func csvgetintarr_original(record string) int {
	if record == "" || record == "0" || record == "\\N" {
		return 0
	}
	getint, err := strconv.Atoi(record)
	if err != nil {
		return 0
	}
	return getint
}

func csvgetfloatarr_original(record string) float32 {
	if record == "" || record == "0" || record == "0.0" || record == "\\N" {
		return 0
	}
	flo, err := strconv.ParseFloat(record, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

// Optimized versions (attempt 1 - length-based null check)
func csvgetuint32arr_optimized1(record string) uint32 {
	if len(record) == 0 || record == "0" || (len(record) == 2 && record == "\\N") {
		return 0
	}
	getint, err := strconv.ParseUint(strings.TrimLeft(record, "t"), 10, 0)
	if err != nil {
		return 0
	}
	return uint32(getint)
}

func csvgetintarr_optimized1(record string) int {
	if len(record) == 0 || record == "0" || (len(record) == 2 && record == "\\N") {
		return 0
	}
	getint, err := strconv.Atoi(record)
	if err != nil {
		return 0
	}
	return getint
}

func csvgetfloatarr_optimized1(record string) float32 {
	if len(record) == 0 || record == "0" || record == "0.0" || (len(record) == 2 && record == "\\N") {
		return 0
	}
	flo, err := strconv.ParseFloat(record, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

// Optimized versions (attempt 2 - switch-based)
func csvgetuint32arr_optimized2(record string) uint32 {
	switch record {
	case "", "0", "\\N":
		return 0
	}
	getint, err := strconv.ParseUint(strings.TrimLeft(record, "t"), 10, 0)
	if err != nil {
		return 0
	}
	return uint32(getint)
}

func csvgetintarr_optimized2(record string) int {
	switch record {
	case "", "0", "\\N":
		return 0
	}
	getint, err := strconv.Atoi(record)
	if err != nil {
		return 0
	}
	return getint
}

func csvgetfloatarr_optimized2(record string) float32 {
	switch record {
	case "", "0", "0.0", "\\N":
		return 0
	}
	flo, err := strconv.ParseFloat(record, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

// Optimized versions (attempt 3 - byte comparison)
func csvgetuint32arr_optimized3(record string) uint32 {
	if len(record) == 0 || record == "0" {
		return 0
	}
	// Check for \N with byte comparison
	if len(record) == 2 && record[0] == '\\' && record[1] == 'N' {
		return 0
	}
	getint, err := strconv.ParseUint(strings.TrimLeft(record, "t"), 10, 0)
	if err != nil {
		return 0
	}
	return uint32(getint)
}

func csvgetintarr_optimized3(record string) int {
	if len(record) == 0 || record == "0" {
		return 0
	}
	// Check for \N with byte comparison
	if len(record) == 2 && record[0] == '\\' && record[1] == 'N' {
		return 0
	}
	getint, err := strconv.Atoi(record)
	if err != nil {
		return 0
	}
	return getint
}

func csvgetfloatarr_optimized3(record string) float32 {
	if len(record) == 0 || record == "0" || record == "0.0" {
		return 0
	}
	// Check for \N with byte comparison
	if len(record) == 2 && record[0] == '\\' && record[1] == 'N' {
		return 0
	}
	flo, err := strconv.ParseFloat(record, 32)
	if err != nil {
		return 0
	}
	return float32(flo)
}

// Tests for correctness
func TestCsvgetuint32arr_Correctness(t *testing.T) {
	tests := []struct {
		input    string
		expected uint32
	}{
		{"123", 123},
		{"t456", 456},
		{"", 0},
		{"0", 0},
		{"\\N", 0},
		{"4294967295", 4294967295},
		{"abc", 0},
		{"-123", 0},
	}

	for _, test := range tests {
		t.Run("original_"+test.input, func(t *testing.T) {
			result := csvgetuint32arr_original(test.input)
			if result != test.expected {
				t.Errorf("original: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})

		t.Run("optimized1_"+test.input, func(t *testing.T) {
			result := csvgetuint32arr_optimized1(test.input)
			if result != test.expected {
				t.Errorf("optimized1: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})

		t.Run("optimized2_"+test.input, func(t *testing.T) {
			result := csvgetuint32arr_optimized2(test.input)
			if result != test.expected {
				t.Errorf("optimized2: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})

		t.Run("optimized3_"+test.input, func(t *testing.T) {
			result := csvgetuint32arr_optimized3(test.input)
			if result != test.expected {
				t.Errorf("optimized3: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})
	}
}

func TestCsvgetintarr_Correctness(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"-456", -456},
		{"", 0},
		{"0", 0},
		{"\\N", 0},
		{"2147483647", 2147483647},
		{"abc", 0},
	}

	for _, test := range tests {
		t.Run("original_"+test.input, func(t *testing.T) {
			result := csvgetintarr_original(test.input)
			if result != test.expected {
				t.Errorf("original: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})

		t.Run("optimized1_"+test.input, func(t *testing.T) {
			result := csvgetintarr_optimized1(test.input)
			if result != test.expected {
				t.Errorf("optimized1: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})

		t.Run("optimized2_"+test.input, func(t *testing.T) {
			result := csvgetintarr_optimized2(test.input)
			if result != test.expected {
				t.Errorf("optimized2: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})

		t.Run("optimized3_"+test.input, func(t *testing.T) {
			result := csvgetintarr_optimized3(test.input)
			if result != test.expected {
				t.Errorf("optimized3: input %q expected %d, got %d", test.input, test.expected, result)
			}
		})
	}
}

func TestCsvgetfloatarr_Correctness(t *testing.T) {
	tests := []struct {
		input    string
		expected float32
	}{
		{"3.14", 3.14},
		{"123", 123.0},
		{"", 0},
		{"0", 0},
		{"0.0", 0},
		{"\\N", 0},
		{"abc", 0},
		{"-3.14", -3.14},
	}

	for _, test := range tests {
		t.Run("original_"+test.input, func(t *testing.T) {
			result := csvgetfloatarr_original(test.input)
			if result != test.expected {
				t.Errorf("original: input %q expected %f, got %f", test.input, test.expected, result)
			}
		})

		t.Run("optimized1_"+test.input, func(t *testing.T) {
			result := csvgetfloatarr_optimized1(test.input)
			if result != test.expected {
				t.Errorf("optimized1: input %q expected %f, got %f", test.input, test.expected, result)
			}
		})

		t.Run("optimized2_"+test.input, func(t *testing.T) {
			result := csvgetfloatarr_optimized2(test.input)
			if result != test.expected {
				t.Errorf("optimized2: input %q expected %f, got %f", test.input, test.expected, result)
			}
		})

		t.Run("optimized3_"+test.input, func(t *testing.T) {
			result := csvgetfloatarr_optimized3(test.input)
			if result != test.expected {
				t.Errorf("optimized3: input %q expected %f, got %f", test.input, test.expected, result)
			}
		})
	}
}

// Benchmark data - realistic CSV data distribution
var benchmarkInputs = []string{
	"123456",     // Regular number
	"t789012",    // tconst with prefix
	"",           // Empty string
	"\\N",        // Null value
	"0",          // Zero
	"3.14159",    // Float
	"0.0",        // Zero float
	"2147483647", // Large number
	"abc123",     // Invalid number
	"12.34",      // Regular float
	// Add more realistic distribution
	"1", "2", "3", "4", "5", "6", "7", "8", "9", "10",
	"\\N", "\\N", "\\N", "\\N", "\\N", // More nulls as they're common
}

// Benchmarks for csvgetuint32arr
func BenchmarkCsvgetuint32arr_Original(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetuint32arr_original(input)
		}
	}
}

func BenchmarkCsvgetuint32arr_Optimized1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetuint32arr_optimized1(input)
		}
	}
}

func BenchmarkCsvgetuint32arr_Optimized2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetuint32arr_optimized2(input)
		}
	}
}

func BenchmarkCsvgetuint32arr_Optimized3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetuint32arr_optimized3(input)
		}
	}
}

// Benchmarks for csvgetintarr
func BenchmarkCsvgetintarr_Original(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetintarr_original(input)
		}
	}
}

func BenchmarkCsvgetintarr_Optimized1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetintarr_optimized1(input)
		}
	}
}

func BenchmarkCsvgetintarr_Optimized2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetintarr_optimized2(input)
		}
	}
}

func BenchmarkCsvgetintarr_Optimized3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetintarr_optimized3(input)
		}
	}
}

// Benchmarks for csvgetfloatarr
func BenchmarkCsvgetfloatarr_Original(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetfloatarr_original(input)
		}
	}
}

func BenchmarkCsvgetfloatarr_Optimized1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetfloatarr_optimized1(input)
		}
	}
}

func BenchmarkCsvgetfloatarr_Optimized2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetfloatarr_optimized2(input)
		}
	}
}

func BenchmarkCsvgetfloatarr_Optimized3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range benchmarkInputs {
			csvgetfloatarr_optimized3(input)
		}
	}
}

// Edge case specific benchmarks
func BenchmarkNullValues_Original(b *testing.B) {
	nulls := []string{"", "\\N", "0", "0.0"}
	for i := 0; i < b.N; i++ {
		for _, null := range nulls {
			csvgetuint32arr_original(null)
			csvgetintarr_original(null)
			csvgetfloatarr_original(null)
		}
	}
}

func BenchmarkNullValues_Optimized2(b *testing.B) {
	nulls := []string{"", "\\N", "0", "0.0"}
	for i := 0; i < b.N; i++ {
		for _, null := range nulls {
			csvgetuint32arr_optimized2(null)
			csvgetintarr_optimized2(null)
			csvgetfloatarr_optimized2(null)
		}
	}
}

func BenchmarkValidNumbers_Original(b *testing.B) {
	numbers := []string{"123", "456789", "t987654", "3.14159", "2.71828"}
	for i := 0; i < b.N; i++ {
		for _, num := range numbers {
			csvgetuint32arr_original(num)
			csvgetintarr_original(num)
			csvgetfloatarr_original(num)
		}
	}
}

func BenchmarkValidNumbers_Optimized2(b *testing.B) {
	numbers := []string{"123", "456789", "t987654", "3.14159", "2.71828"}
	for i := 0; i < b.N; i++ {
		for _, num := range numbers {
			csvgetuint32arr_optimized2(num)
			csvgetintarr_optimized2(num)
			csvgetfloatarr_optimized2(num)
		}
	}
}

// Memory allocation benchmarks
func BenchmarkCsvgetuint32arr_Original_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		csvgetuint32arr_original("t123456")
	}
}

func BenchmarkCsvgetuint32arr_Optimized2_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		csvgetuint32arr_optimized2("t123456")
	}
}

// UnescapeString optimization variations
func unescapeString_original(record string) string {
	// Fast empty check
	if len(record) == 0 {
		return ""
	}

	// Fast \\N check (literal backslash followed by N = 2 characters)
	if len(record) == 2 && record == "\\N" {
		return ""
	}

	// Fast path: check for '&' using indexing instead of ContainsRune
	// This avoids UTF-8 decoding overhead since '&' is ASCII
	for i := 0; i < len(record); i++ {
		if record[i] == '&' {
			return html.UnescapeString(record)
		}
	}

	return record
}

func unescapeString_optimized1(record string) string {
	// Fast empty check
	if len(record) == 0 {
		return ""
	}

	// Fast \\N check with byte comparison (2 characters: \ and N)
	if len(record) == 2 && record[0] == '\\' && record[1] == 'N' {
		return ""
	}

	// Fast path: check for '&' using byte indexing
	for i := 0; i < len(record); i++ {
		if record[i] == '&' {
			return html.UnescapeString(record)
		}
	}

	return record
}

func unescapeString_optimized2(record string) string {
	// Fast empty check
	if len(record) == 0 {
		return ""
	}

	// Fast \\N check with switch
	if record == "\\N" {
		return ""
	}

	// Use strings.ContainsRune instead of manual loop
	if strings.ContainsRune(record, '&') {
		return html.UnescapeString(record)
	}

	return record
}

func unescapeString_optimized3(record string) string {
	// Combined length and content checks
	switch len(record) {
	case 0:
		return ""
	case 2:
		if record == "\\N" {
			return ""
		}
	}

	// Fast path: check for '&' using byte indexing
	for i := 0; i < len(record); i++ {
		if record[i] == '&' {
			return html.UnescapeString(record)
		}
	}

	return record
}

// Tests for unescapeString correctness
func TestUnescapeString_Correctness(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"\\N", ""},
		{"Test &amp; More", "Test & More"},
		{"Test &#39;quoted&#39;", "Test 'quoted'"},
		{"Regular string", "Regular string"},
		{"No entities here", "No entities here"},
		{"&lt;html&gt;", "<html>"},
		{"Mixed &amp; content &#39;here&#39;", "Mixed & content 'here'"},
	}

	for _, test := range tests {
		t.Run("original_"+test.input, func(t *testing.T) {
			result := unescapeString_original(test.input)
			if result != test.expected {
				t.Errorf("original: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})

		t.Run("optimized1_"+test.input, func(t *testing.T) {
			result := unescapeString_optimized1(test.input)
			if result != test.expected {
				t.Errorf("optimized1: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})

		t.Run("optimized2_"+test.input, func(t *testing.T) {
			result := unescapeString_optimized2(test.input)
			if result != test.expected {
				t.Errorf("optimized2: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})

		t.Run("optimized3_"+test.input, func(t *testing.T) {
			result := unescapeString_optimized3(test.input)
			if result != test.expected {
				t.Errorf("optimized3: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})
	}
}

// Benchmark data for unescapeString - realistic distribution
var unescapeBenchmarkInputs = []string{
	"Regular title without entities",
	"Title with &amp; ampersand",
	"Title with &#39;quotes&#39;",
	"",    // Empty string
	"\\N", // Null value
	"No entities at all here",
	"&lt;The Matrix&gt;",
	"Film &amp; Television &#39;Special&#39;",
	"Long title without any HTML entities that need unescaping",
	"Short",
	"A",
	"Multiple &amp; entities &lt;here&gt; and &#39;quotes&#39;",
	// Add more nulls as they're common
	"\\N", "\\N", "\\N", "\\N", "\\N",
	// Add more regular strings as they're most common
	"Action", "Drama", "Comedy", "Thriller", "Horror",
}

// Benchmarks for unescapeString
func BenchmarkUnescapeString_Original(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range unescapeBenchmarkInputs {
			unescapeString_original(input)
		}
	}
}

func BenchmarkUnescapeString_Optimized1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range unescapeBenchmarkInputs {
			unescapeString_optimized1(input)
		}
	}
}

func BenchmarkUnescapeString_Optimized2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range unescapeBenchmarkInputs {
			unescapeString_optimized2(input)
		}
	}
}

func BenchmarkUnescapeString_Optimized3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range unescapeBenchmarkInputs {
			unescapeString_optimized3(input)
		}
	}
}

// Specific benchmarks for different input types
func BenchmarkUnescapeString_NoEntities_Original(b *testing.B) {
	inputs := []string{"Regular title", "Another title", "No entities here"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			unescapeString_original(input)
		}
	}
}

func BenchmarkUnescapeString_NoEntities_Optimized1(b *testing.B) {
	inputs := []string{"Regular title", "Another title", "No entities here"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			unescapeString_optimized1(input)
		}
	}
}

func BenchmarkUnescapeString_WithEntities_Original(b *testing.B) {
	inputs := []string{"Title &amp; More", "Film &#39;Quote&#39;", "&lt;HTML&gt;"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			unescapeString_original(input)
		}
	}
}

func BenchmarkUnescapeString_WithEntities_Optimized1(b *testing.B) {
	inputs := []string{"Title &amp; More", "Film &#39;Quote&#39;", "&lt;HTML&gt;"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			unescapeString_optimized1(input)
		}
	}
}

func BenchmarkUnescapeString_NullValues_Original(b *testing.B) {
	inputs := []string{"", "\\N"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			unescapeString_original(input)
		}
	}
}

func BenchmarkUnescapeString_NullValues_Optimized1(b *testing.B) {
	inputs := []string{"", "\\N"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			unescapeString_optimized1(input)
		}
	}
}

// Memory allocation benchmarks for unescapeString
func BenchmarkUnescapeString_Original_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		unescapeString_original("Title &amp; More")
	}
}

func BenchmarkUnescapeString_Optimized1_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		unescapeString_optimized1("Title &amp; More")
	}
}

// StringToSlug optimization variations
func stringToSlug_original(instr string) string {
	// Fast empty check
	if len(instr) == 0 {
		return ""
	}

	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}

	// Optimize trimming by finding actual start/end positions
	start := 0
	end := len(inbyte)

	// Find start position (skip leading hyphens and spaces)
	for start < end && (inbyte[start] == '-' || inbyte[start] == ' ') {
		start++
	}

	// Find end position (skip trailing hyphens and spaces)
	for end > start && (inbyte[end-1] == '-' || inbyte[end-1] == ' ') {
		end--
	}

	if start >= end {
		return ""
	}

	return string(inbyte[start:end])
}

func stringToSlug_optimized1(instr string) string {
	// Fast empty check
	if len(instr) == 0 {
		return ""
	}

	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}

	// Optimize trimming with early bounds check
	if len(inbyte) == 1 {
		if inbyte[0] == '-' || inbyte[0] == ' ' {
			return ""
		}
		return string(inbyte)
	}

	// Find start position (skip leading hyphens and spaces)
	start := 0
	for start < len(inbyte) && (inbyte[start] == '-' || inbyte[start] == ' ') {
		start++
	}

	// Early exit if all characters are separators
	if start >= len(inbyte) {
		return ""
	}

	// Find end position (skip trailing hyphens and spaces)
	end := len(inbyte)
	for end > start && (inbyte[end-1] == '-' || inbyte[end-1] == ' ') {
		end--
	}

	return string(inbyte[start:end])
}

func stringToSlug_optimized2(instr string) string {
	// Combined fast checks
	switch len(instr) {
	case 0:
		return ""
	case 1:
		if instr[0] == ' ' || instr[0] == '-' {
			return ""
		}
		// For single character, avoid unidecode2 if it's already valid ASCII
		if instr[0] >= 'a' && instr[0] <= 'z' || instr[0] >= '0' && instr[0] <= '9' {
			return instr
		}
	}

	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}

	// Use bytes.TrimSpace equivalent for our specific characters
	start := 0
	end := len(inbyte)

	// Trim leading
	for start < end {
		if inbyte[start] != '-' && inbyte[start] != ' ' {
			break
		}
		start++
	}

	// Trim trailing
	for end > start {
		if inbyte[end-1] != '-' && inbyte[end-1] != ' ' {
			break
		}
		end--
	}

	if start >= end {
		return ""
	}

	return string(inbyte[start:end])
}

func stringToSlug_optimized3(instr string) string {
	// Fast empty check
	if len(instr) == 0 {
		return ""
	}

	// Always use unidecode2 for consistency with original
	// The complexity of detecting "clean" strings isn't worth the optimization
	inbyte := unidecode2(instr)
	if len(inbyte) == 0 {
		return ""
	}

	// Trim efficiently
	start := 0
	end := len(inbyte)

	for start < end && (inbyte[start] == '-' || inbyte[start] == ' ') {
		start++
	}

	for end > start && (inbyte[end-1] == '-' || inbyte[end-1] == ' ') {
		end--
	}

	if start >= end {
		return ""
	}

	return string(inbyte[start:end])
}

// Tests for stringToSlug correctness
func TestStringToSlug_Correctness(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"\\N", "n"}, // Current behavior: \\N becomes "n"
		{"  -test-  ", "test"},
		{"test  --  example", "test-example"},
		{"Action", "action"},
		{"The Matrix", "the-matrix"},
		{"Film & TV", "film-and-tv"},
		{"José", "jose"},
		{"Café", "cafe"},
		{"   ", ""},
		{"---", ""},
		{"a", "a"},
		{"-a-", "a"},
		{"Test's Movie", "tests-movie"}, // Current behavior: apostrophe gets removed
		{"Movie (2023)", "movie-2023"},
		{"  Leading spaces", "leading-spaces"},
		{"Trailing spaces  ", "trailing-spaces"},
		{"Multiple   spaces", "multiple-spaces"},
	}

	for _, test := range tests {
		t.Run("original_"+test.input, func(t *testing.T) {
			result := stringToSlug_original(test.input)
			if result != test.expected {
				t.Errorf("original: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})

		t.Run("optimized1_"+test.input, func(t *testing.T) {
			result := stringToSlug_optimized1(test.input)
			if result != test.expected {
				t.Errorf("optimized1: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})

		t.Run("optimized2_"+test.input, func(t *testing.T) {
			result := stringToSlug_optimized2(test.input)
			if result != test.expected {
				t.Errorf("optimized2: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})

		t.Run("optimized3_"+test.input, func(t *testing.T) {
			result := stringToSlug_optimized3(test.input)
			if result != test.expected {
				t.Errorf("optimized3: input %q expected %q, got %q", test.input, test.expected, result)
			}
		})
	}
}

// Benchmark data for stringToSlug - realistic IMDB title distribution
var slugBenchmarkInputs = []string{
	"The Matrix",        // Common movie title
	"Action",            // Single word genre
	"Film & Television", // With ampersand
	"José's Movie",      // With accent and apostrophe
	"",                  // Empty string
	"\\N",               // Null value
	"Movie (2023)",      // With parentheses and year
	"The Lord of the Rings: The Fellowship of the Ring", // Long title
	"a",                  // Single character
	"TV Show",            // Short title
	"Horror",             // Another genre
	"Café",               // With accent
	"Test's Adventure",   // With apostrophe
	"   Leading Spaces",  // With leading spaces
	"Trailing Spaces   ", // With trailing spaces
	"Multiple   Spaces",  // With multiple spaces
	"- Hyphenated -",     // With hyphens
	// Add more common patterns
	"Comedy", "Drama", "Thriller", "Romance", "Adventure",
	// More nulls as they're common
	"\\N", "\\N", "\\N",
}

// Benchmarks for stringToSlug
func BenchmarkStringToSlug_Original(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range slugBenchmarkInputs {
			stringToSlug_original(input)
		}
	}
}

func BenchmarkStringToSlug_Optimized1(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range slugBenchmarkInputs {
			stringToSlug_optimized1(input)
		}
	}
}

func BenchmarkStringToSlug_Optimized2(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range slugBenchmarkInputs {
			stringToSlug_optimized2(input)
		}
	}
}

func BenchmarkStringToSlug_Optimized3(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, input := range slugBenchmarkInputs {
			stringToSlug_optimized3(input)
		}
	}
}

// Specific benchmarks for different input types
func BenchmarkStringToSlug_SimpleASCII_Original(b *testing.B) {
	inputs := []string{"Action", "Drama", "Comedy", "The Matrix"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			stringToSlug_original(input)
		}
	}
}

func BenchmarkStringToSlug_SimpleASCII_Optimized1(b *testing.B) {
	inputs := []string{"Action", "Drama", "Comedy", "The Matrix"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			stringToSlug_optimized1(input)
		}
	}
}

func BenchmarkStringToSlug_WithUnicode_Original(b *testing.B) {
	inputs := []string{"José", "Café", "Naïve", "Résumé"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			stringToSlug_original(input)
		}
	}
}

func BenchmarkStringToSlug_WithUnicode_Optimized1(b *testing.B) {
	inputs := []string{"José", "Café", "Naïve", "Résumé"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			stringToSlug_optimized1(input)
		}
	}
}

func BenchmarkStringToSlug_NullValues_Original(b *testing.B) {
	inputs := []string{"", "\\N", "   ", "---"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			stringToSlug_original(input)
		}
	}
}

func BenchmarkStringToSlug_NullValues_Optimized1(b *testing.B) {
	inputs := []string{"", "\\N", "   ", "---"}
	for i := 0; i < b.N; i++ {
		for _, input := range inputs {
			stringToSlug_optimized1(input)
		}
	}
}

// Memory allocation benchmarks for stringToSlug
func BenchmarkStringToSlug_Original_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		stringToSlug_original("The Matrix")
	}
}

func BenchmarkStringToSlug_Optimized1_Allocs(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		stringToSlug_optimized1("The Matrix")
	}
}

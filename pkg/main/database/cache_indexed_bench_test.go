package database

import (
	"fmt"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/Kellerman81/go_media_downloader/pkg/main/syncops"
)

// normalizeKeyOriginal is the original implementation for comparison.
func normalizeKeyOriginal(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	bufPtr := normKeyPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]
	for _, r := range s {
		if r <= unicode.MaxASCII {
			if r >= 'A' && r <= 'Z' {
				buf = append(buf, byte(r+32))
			} else {
				buf = append(buf, byte(r))
			}
		} else {
			for _, b := range string(unicode.ToLower(r)) {
				buf = append(buf, byte(b))
			}
		}
	}
	result := string(buf)
	*bufPtr = buf
	normKeyPool.Put(bufPtr)
	return result
}

// BenchmarkNormalizeKey covers three representative inputs:
//   - already-lowercase slug  (fast path candidate)
//   - mixed-case title        (needs lowercase conversion)
//   - string with non-ASCII   (exercises utf8 branch)
func BenchmarkNormalizeKey_Original(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = normalizeKeyOriginal("series-title-1234")         // already lower
		_ = normalizeKeyOriginal("Game of Thrones")           // needs lower
		_ = normalizeKeyOriginal("Séries Très Intéressantes") // non-ASCII
	}
}

func BenchmarkNormalizeKey_Current(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = normalizeKey("series-title-1234")
		_ = normalizeKey("Game of Thrones")
		_ = normalizeKey("Séries Très Intéressantes")
	}
}

// normalizeKeyOpt is the optimised candidate to benchmark against the current.
func normalizeKeyOpt(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	// Fast path: already all-lowercase ASCII — return as-is, no alloc.
	needsConversion := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' || c >= 0x80 {
			needsConversion = true
			break
		}
	}
	if !needsConversion {
		return s
	}

	// Slow path: build lowercased copy via pool buffer.
	bufPtr := normKeyPool.Get().(*[]byte)
	buf := (*bufPtr)[:0]

	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 0x80 {
			if c >= 'A' && c <= 'Z' {
				buf = append(buf, c+32)
			} else {
				buf = append(buf, c)
			}
		} else {
			r, size := utf8.DecodeRuneInString(s[i:])
			var tmp [utf8.UTFMax]byte
			n := utf8.EncodeRune(tmp[:], unicode.ToLower(r))
			buf = append(buf, tmp[:n]...)
			i += size - 1
		}
	}

	result := string(buf)
	*bufPtr = buf
	normKeyPool.Put(bufPtr)

	return result
}

func BenchmarkNormalizeKey_Opt(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = normalizeKeyOpt("series-title-1234")
		_ = normalizeKeyOpt("Game of Thrones")
		_ = normalizeKeyOpt("Séries Très Intéressantes")
	}
}

// BenchmarkNormalizeKey_Sizes exercises each variant at the per-call level
// split by input class.
func BenchmarkNormalizeKey_Sizes(b *testing.B) {
	cases := []struct {
		name  string
		input string
	}{
		{"AlreadyLower", "series-title-1234"},
		{"MixedCase", "Game of Thrones"},
		{"NonASCII", "Séries Très Intéressantes"},
	}
	for _, tc := range cases {
		b.Run("Original_"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = normalizeKeyOriginal(tc.input)
			}
		})
		b.Run("Opt_"+tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = normalizeKeyOpt(tc.input)
			}
		})
	}
}

// generateTestArr produces n rows with realistic title/slug pairs.
// Every 10th entry shares a Str1 with a previous entry to simulate
// duplicate-title collisions (multiple entries per key).
func generateTestArr(n int) []syncops.DbstaticTwoStringOneInt {
	arr := make([]syncops.DbstaticTwoStringOneInt, n)
	for i := range arr {
		base := i
		if i%10 == 0 && i > 0 {
			base = i - 10 // collide with an earlier entry's title
		}
		arr[i] = syncops.DbstaticTwoStringOneInt{
			Str1: fmt.Sprintf("Series Title %d", base),
			Str2: fmt.Sprintf("series-title-%d", i), // slugs are always unique
			Num:  uint(i + 1),
		}
	}
	return arr
}

// buildIndexTwoStringByStr1OneMake is the original single-pass implementation.
func buildIndexTwoStringByStr1OneMake(
	arr []syncops.DbstaticTwoStringOneInt,
) map[string][]*syncops.DbstaticTwoStringOneInt {
	index := make(map[string][]*syncops.DbstaticTwoStringOneInt, len(arr)*2)

	for i := range arr {
		if arr[i].Str1 != "" {
			key := normalizeKey(arr[i].Str1)
			index[key] = append(index[key], &arr[i])
		}
		if arr[i].Str2 != "" {
			key := normalizeKey(arr[i].Str2)
			index[key] = append(index[key], &arr[i])
		}
	}

	return index
}

// buildIndexTwoStringByStr1TwoMake is the two-pass build kept for comparison only.
func buildIndexTwoStringByStr1TwoMake(
	arr []syncops.DbstaticTwoStringOneInt,
) map[string][]*syncops.DbstaticTwoStringOneInt {
	counts := make(map[string]int, len(arr))
	for i := range arr {
		if arr[i].Str1 != "" {
			counts[normalizeKey(arr[i].Str1)]++
		}
		if arr[i].Str2 != "" {
			counts[normalizeKey(arr[i].Str2)]++
		}
	}
	index := make(map[string][]*syncops.DbstaticTwoStringOneInt, len(counts))
	for k, cnt := range counts {
		index[k] = make([]*syncops.DbstaticTwoStringOneInt, 0, cnt)
	}
	for i := range arr {
		if arr[i].Str1 != "" {
			key := normalizeKey(arr[i].Str1)
			index[key] = append(index[key], &arr[i])
		}
		if arr[i].Str2 != "" {
			key := normalizeKey(arr[i].Str2)
			index[key] = append(index[key], &arr[i])
		}
	}
	return index
}

// BenchmarkBuildIndex_OneMake measures the original hint (len(arr)*2).
func BenchmarkBuildIndex_OneMake(b *testing.B) {
	arr := generateTestArr(5000)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = buildIndexTwoStringByStr1OneMake(arr)
	}
}

// BenchmarkBuildIndex_TwoMake measures the two-pass build (kept for reference).
func BenchmarkBuildIndex_TwoMake(b *testing.B) {
	arr := generateTestArr(5000)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = buildIndexTwoStringByStr1TwoMake(arr)
	}
}

// BenchmarkBuildIndex_OneMakeLenArr measures the current implementation (hint len(arr)).
func BenchmarkBuildIndex_OneMakeLenArr(b *testing.B) {
	arr := generateTestArr(5000)
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		_ = buildIndexTwoStringByStr1(arr)
	}
}

// Sub-benchmarks at different sizes to see how each scales.
func BenchmarkBuildIndex_Sizes(b *testing.B) {
	for _, n := range []int{1000, 5000, 10000, 25000} {
		arr := generateTestArr(n)

		b.Run(fmt.Sprintf("OneMake2x_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = buildIndexTwoStringByStr1OneMake(arr)
			}
		})

		b.Run(fmt.Sprintf("OneMake1x_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = buildIndexTwoStringByStr1(arr)
			}
		})

		b.Run(fmt.Sprintf("TwoMake_%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				_ = buildIndexTwoStringByStr1TwoMake(arr)
			}
		})
	}
}

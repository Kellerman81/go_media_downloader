package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestNewPool(t *testing.T) {
	t.Run("creates empty pool", func(t *testing.T) {
		pool := newPool[int](5, 0, nil, nil)
		if cap(pool.objs) != 5 {
			t.Errorf("expected capacity 5, got %d", cap(pool.objs))
		}
		if len(pool.objs) != 0 {
			t.Errorf("expected length 0, got %d", len(pool.objs))
		}
	})

	t.Run("initializes with objects", func(t *testing.T) {
		constructorCalled := 0
		constructor := func(i *int) {
			*i = 42
			constructorCalled++
		}
		pool := newPool(5, 3, constructor, nil)

		if len(pool.objs) != 3 {
			t.Errorf("expected 3 objects, got %d", len(pool.objs))
		}
		if constructorCalled != 3 {
			t.Errorf("constructor should be called 3 times, got %d", constructorCalled)
		}
	})
}

func TestPoolGet(t *testing.T) {
	t.Run("gets existing object", func(t *testing.T) {
		pool := newPool(5, 1, func(i *int) { *i = 42 }, nil)
		obj := pool.Get()
		if *obj != 42 {
			t.Errorf("expected 42, got %d", *obj)
		}
		if len(pool.objs) != 0 {
			t.Errorf("pool should be empty after get")
		}
	})

	t.Run("creates new object when empty", func(t *testing.T) {
		constructorCalled := 0
		pool := newPool(5, 0, func(i *int) {
			*i = 99
			constructorCalled++
		}, nil)

		obj := pool.Get()
		if *obj != 99 {
			t.Errorf("expected 99, got %d", *obj)
		}
		if constructorCalled != 1 {
			t.Errorf("constructor should be called once, got %d", constructorCalled)
		}
	})
}

func TestPoolPut(t *testing.T) {
	t.Run("puts object back in pool", func(t *testing.T) {
		destructorCalled := 0
		pool := newPool(5, 0, nil, func(i *int) {
			*i = 0
			destructorCalled++
		})

		obj := new(int)
		*obj = 42
		pool.Put(obj)

		if len(pool.objs) != 1 {
			t.Errorf("expected 1 object in pool, got %d", len(pool.objs))
		}
		if destructorCalled != 1 {
			t.Errorf("destructor should be called once, got %d", destructorCalled)
		}
	})

	t.Run("ignores nil objects", func(t *testing.T) {
		pool := newPool[int](5, 0, nil, nil)
		pool.Put(nil)
		if len(pool.objs) != 0 {
			t.Errorf("pool should remain empty after putting nil")
		}
	})

	t.Run("respects capacity limit", func(t *testing.T) {
		pool := newPool[int](2, 2, nil, nil)
		extra := new(int)
		pool.Put(extra)
		if len(pool.objs) != 2 {
			t.Errorf("pool should not exceed capacity")
		}
	})
}

func TestCsvgetintarr(t *testing.T) {
	t.Run("converts valid integer string", func(t *testing.T) {
		result := csvgetintarr("42")
		if result != 42 {
			t.Errorf("expected 42, got %d", result)
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		result := csvgetintarr("")
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("handles \\N", func(t *testing.T) {
		result := csvgetintarr("\\N")
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})

	t.Run("handles invalid integer", func(t *testing.T) {
		result := csvgetintarr("abc")
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})
}

func TestCsvgetuint32arr(t *testing.T) {
	t.Run("converts valid uint32 string", func(t *testing.T) {
		result := csvgetuint32arr("123")
		if result != 123 {
			t.Errorf("expected 123, got %d", result)
		}
	})

	t.Run("handles t prefix", func(t *testing.T) {
		result := csvgetuint32arr("t456")
		if result != 456 {
			t.Errorf("expected 456, got %d", result)
		}
	})

	t.Run("handles invalid uint32", func(t *testing.T) {
		result := csvgetuint32arr("-123")
		if result != 0 {
			t.Errorf("expected 0, got %d", result)
		}
	})
}

func TestCsvgetfloatarr(t *testing.T) {
	t.Run("converts valid float string", func(t *testing.T) {
		result := csvgetfloatarr("3.14")
		if result != 3.14 {
			t.Errorf("expected 3.14, got %f", result)
		}
	})

	t.Run("handles 0.0", func(t *testing.T) {
		result := csvgetfloatarr("0.0")
		if result != 0 {
			t.Errorf("expected 0, got %f", result)
		}
	})

	t.Run("handles invalid float", func(t *testing.T) {
		result := csvgetfloatarr("abc.def")
		if result != 0 {
			t.Errorf("expected 0, got %f", result)
		}
	})
}

func TestCsvgetboolarr(t *testing.T) {
	t.Run("handles various true values", func(t *testing.T) {
		trueValues := []string{"1", "t", "T", "true", "TRUE", "True"}
		for _, v := range trueValues {
			result := csvgetboolarr(v)
			if !result {
				t.Errorf("expected true for %s, got false", v)
			}
		}
	})

	t.Run("handles various false values", func(t *testing.T) {
		falseValues := []string{"0", "f", "F", "false", "FALSE", "False"}
		for _, v := range falseValues {
			result := csvgetboolarr(v)
			if result {
				t.Errorf("expected false for %s, got true", v)
			}
		}
	})

	t.Run("handles \\N value", func(t *testing.T) {
		result := csvgetboolarr("\\N")
		if result {
			t.Errorf("expected false for \\N, got true")
		}
	})
}

func TestUnescapeString(t *testing.T) {
	t.Run("handles null value", func(t *testing.T) {
		result := unescapeString("\\N")
		if result != "" {
			t.Errorf("expected empty string for \\N, got %q", result)
		}
	})

	t.Run("unescapes HTML entities", func(t *testing.T) {
		result := unescapeString("Test & More")
		if result != "Test & More" {
			t.Errorf("expected 'Test & More', got %q", result)
		}
	})

	t.Run("returns original string when no entities", func(t *testing.T) {
		input := "Regular string"
		result := unescapeString(input)
		if result != input {
			t.Errorf("expected %q, got %q", input, result)
		}
	})
}

func TestStringToSlug(t *testing.T) {
	t.Run("handles empty string", func(t *testing.T) {
		result := stringToSlug("")
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})

	t.Run("trims hyphens and spaces", func(t *testing.T) {
		result := stringToSlug("  -test-  ")
		if result != "test" {
			t.Errorf("expected 'test', got %q", result)
		}
	})

	t.Run("handles multiple spaces and hyphens", func(t *testing.T) {
		result := stringToSlug("test  --  example")
		if result != "test-example" {
			t.Errorf("expected 'test-example', got %q", result)
		}
	})
}

func TestUnidecode2(t *testing.T) {
	t.Run("converts unicode to ASCII", func(t *testing.T) {
		result := string(unidecode2("héllo"))
		if result != "hello" {
			t.Errorf("expected 'hello', got %q", result)
		}
	})

	t.Run("handles HTML entities", func(t *testing.T) {
		result := string(unidecode2("Test & More"))
		if result != "test-and-more" {
			t.Errorf("expected 'test-and-more', got %q", result)
		}
	})

	t.Run("collapses multiple hyphens", func(t *testing.T) {
		result := string(unidecode2("test---example"))
		if result != "test-example" {
			t.Errorf("expected 'test-example', got %q", result)
		}
	})

	t.Run("converts uppercase to lowercase", func(t *testing.T) {
		result := string(unidecode2("TEST"))
		if result != "test" {
			t.Errorf("expected 'test', got %q", result)
		}
	})

	t.Run("handles special unicode characters", func(t *testing.T) {
		result := string(unidecode2("café"))
		if result != "cafe" {
			t.Errorf("expected 'cafe', got %q", result)
		}
	})

	t.Run("ignores characters above 0xeffff", func(t *testing.T) {
		result := string(unidecode2(string([]rune{0xf0000})))
		if result != "" {
			t.Errorf("expected empty string, got %q", result)
		}
	})
}

func TestCSVParse(t *testing.T) {
	t.Run("parses valid CSV string", func(t *testing.T) {
		filetitle, err := os.Open("./title.basics.tsv")
		if err != nil {
			fmt.Println(fmt.Errorf("an error occurred while opening titles.. %v", err))
			return
		}
		defer filetitle.Close()
		parsertitle := csv.NewReader(filetitle)
		parsertitle.Comma = '\t'
		parsertitle.LazyQuotes = true
		parsertitle.ReuseRecord = true
		parsertitle.TrimLeadingSpace = true
		_, _ = parsertitle.Read() // skip header
		for {
			record, err := parsertitle.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				fmt.Println(fmt.Errorf("an error occurred while parsing title.. %v", err))
				continue
			}
			if record[1] == "" {
				continue
			}
			if record[0] != "tt1320347" {
				continue
			}
			t.Log(record)
			if record[7] != "\\N" && record[7] != "" && record[7] != "0" {
				t.Log(csvgetintarr(record[7]))
			}
		}
	})
}

func newTestProcessor(t *testing.T, cfg imdbConfig) *imdbProcessor {
	t.Helper()
	p, err := newimdbProcessor(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", "file:"+t.Name()+"?mode=memory&cache=shared&_fk=1")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	p.db = db
	t.Cleanup(func() { _ = p.Close() })
	if err := p.createInMemorySchema(); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestTSVReader(t *testing.T) {
	input := "tconst\ttitleType\tprimaryTitle\n" +
		"tt0090555\tmovie\t\"Crocodile\" Dundee\n" +
		"tt0000002\t\tNext Line\r\n" +
		"tt0000003\tshort\n" +
		"tt0000004\tmovie\tLast"
	r := newTSVReader(strings.NewReader(input))
	if err := r.readHeader(); err != nil {
		t.Fatal(err)
	}
	want := [][]string{
		{"tt0090555", "movie", "\"Crocodile\" Dundee"},
		{"tt0000002", "", "Next Line"},
		{"tt0000003", "short"},
		{"tt0000004", "movie", "Last"},
	}
	for i, w := range want {
		got, err := r.Read()
		if err != nil {
			t.Fatalf("row %d: %v", i, err)
		}
		if strings.Join(got, "|") != strings.Join(w, "|") {
			t.Errorf("row %d: got %q, want %q", i, got, w)
		}
	}
	if _, err := r.Read(); err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}

	long := newTSVReader(strings.NewReader("h\n" + strings.Repeat("x", tsvMaxLineSize+1) + "\n"))
	_ = long.readHeader()
	if _, err := long.Read(); err == nil || err == io.EOF {
		t.Errorf("expected read error for oversized line, got %v", err)
	}
	if err := newTSVReader(strings.NewReader("")).readHeader(); err == nil {
		t.Error("expected error for empty file")
	}
}

func TestProcessTitlesAndAkas(t *testing.T) {
	p := newTestProcessor(t, imdbConfig{Indexedtypes: []string{"movie"}, Indexfull: true})
	p.titleMap = map[string]struct{}{"movie": {}}

	titles := "tconst\ttitleType\tprimaryTitle\toriginalTitle\tisAdult\tstartYear\tendYear\truntimeMinutes\tgenres\n" +
		"tt0090555\tmovie\t\"Crocodile\" Dundee\t\"Crocodile\" Dundee\t0\t1986\t\\N\t97\tAdventure,Comedy\n" +
		"tt0000002\tmovie\tSecond\tSecond\t0\t2000\t\\N\t\tDrama\r\n" +
		"tt0000003\tmovie\tmalformed\n" +
		"tt0000004\tshort\tNot Indexed\tNot Indexed\t0\t2001\t\\N\t5\tShort\n"
	if err := p.processTitlesCSV(context.Background(), strings.NewReader(titles)); err != nil {
		t.Fatal(err)
	}
	if err := p.commitBatch(false, true); err != nil {
		t.Fatal(err)
	}
	if err := p.commitBatch(true, true); err != nil {
		t.Fatal(err)
	}

	var n int
	if err := p.db.QueryRow("select count(*) from imdb_titles").Scan(&n); err != nil || n != 2 {
		t.Fatalf("titles: got %d (%v), want 2", n, err)
	}
	var title string
	var runtime sql.NullInt64
	if err := p.db.QueryRow("select primary_title, runtime_minutes from imdb_titles where tconst='tt0090555'").Scan(&title, &runtime); err != nil {
		t.Fatal(err)
	}
	if title != "\"Crocodile\" Dundee" || runtime.Int64 != 97 {
		t.Errorf("got title %q runtime %d", title, runtime.Int64)
	}

	// Empty indexed_languages imports all regions, including \N.
	p.initAkaFilter()
	akas := "titleId\tordering\ttitle\tregion\tlanguage\ttypes\tattributes\tisOriginalTitle\n" +
		"tt0090555\t1\t\"Crocodile\" Dundee\tUS\ten\timdbDisplay\t\\N\t0\n" +
		"tt0090555\t2\tCrocodile Dundee\t\\N\t\\N\toriginal\t\\N\t1\n" +
		"tt0000002\t1\tZweiter\tDE\tde\t\\N\t\\N\t0\n"
	if err := p.processAkasCSV(context.Background(), strings.NewReader(akas)); err != nil {
		t.Fatal(err)
	}
	if err := p.commitBatch(false, true); err != nil {
		t.Fatal(err)
	}
	if err := p.db.QueryRow("select count(*) from imdb_akas").Scan(&n); err != nil || n != 3 {
		t.Fatalf("akas: got %d (%v), want 3", n, err)
	}
	var ordering, isOrig int
	var lang, types string
	if err := p.db.QueryRow("select ordering, language, types, is_original_title from imdb_akas where tconst='tt0090555' and ordering=2").Scan(&ordering, &lang, &types, &isOrig); err != nil {
		t.Fatal(err)
	}
	if ordering != 2 || types != "original" || isOrig != 1 {
		t.Errorf("full aka columns not populated: ordering=%d language=%q types=%q is_original=%d", ordering, lang, types, isOrig)
	}

	for _, idx := range []string{
		"idx_imdb_titles_primary_original_slug", "idx_imdb_akas_title_slug",
		"idx_imdb_titles_primary_title", "idx_imdb_titles_original_title",
		"idx_imdb_titles_slug", "idx_imdb_titles_start_year",
		"idx_imdb_akas_slug", "idx_imdb_akas_title", "idx_imdb_akas_tconst",
	} {
		if err := p.db.QueryRow("select count(*) from sqlite_master where type='index' and name=?", idx).Scan(&n); err != nil || n != 1 {
			t.Errorf("index %s missing", idx)
		}
	}
}

func TestShouldIndexLanguage(t *testing.T) {
	tests := []struct {
		langs  []string
		region string
		want   bool
	}{
		{nil, "US", true},
		{nil, "\\N", true},
		{[]string{"DE"}, "DE", true},
		{[]string{"DE"}, "US", false},
		{[]string{"DE"}, "\\N", false},
		{[]string{"DE", ""}, "\\N", true},
		{[]string{"DE", ""}, "", true},
		{[]string{"DE", "\\N"}, "\\N", true},
	}
	for _, tt := range tests {
		p := &imdbProcessor{config: imdbConfig{Indexedlanguages: tt.langs}}
		p.initAkaFilter()
		if got := p.shouldIndexLanguage(tt.region); got != tt.want {
			t.Errorf("langs %q region %q: got %v, want %v", tt.langs, tt.region, got, tt.want)
		}
	}
}

func TestCommitBatchResetsAfterFailure(t *testing.T) {
	p := newTestProcessor(t, imdbConfig{Indexedtypes: []string{"movie"}})
	p.buildSQLBatch("INSERT INTO no_such_table (a) VALUES ", "(?)", false)
	p.valueArgs = append(p.valueArgs, 1)
	if err := p.commitBatch(false, true); err == nil {
		t.Fatal("expected error")
	}
	if p.sqlBuild.Len() != 0 || len(p.valueArgs) != 0 || p.batchRows != 0 {
		t.Fatal("failed batch was not reset")
	}
	p.buildSQLBatch("INSERT INTO imdb_titles (tconst) VALUES ", "(?)", false)
	p.valueArgs = append(p.valueArgs, "tt1")
	if err := p.commitBatch(false, true); err != nil {
		t.Fatalf("next batch failed: %v", err)
	}
}

func TestGunzipTruncated(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, _ = zw.Write(bytes.Repeat([]byte("tt0000001\tmovie\tTitle\n"), 5000))
	_ = zw.Close()

	good := filepath.Join(dir, "good.tsv.gz")
	if err := os.WriteFile(good, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := gunzip(good, filepath.Join(dir, "good.tsv")); err != nil {
		t.Fatalf("valid archive: %v", err)
	}

	bad := filepath.Join(dir, "bad.tsv.gz")
	if err := os.WriteFile(bad, buf.Bytes()[:buf.Len()/2], 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "bad.tsv")
	if err := gunzip(bad, target); err == nil {
		t.Fatal("expected error for truncated archive")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("partial target file left behind")
	}
	if err := gunzip(filepath.Join(dir, "missing.gz"), target); err == nil {
		t.Error("expected error for missing source")
	}
}

func TestDownloadFileHTTPStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			_, _ = w.Write([]byte("data"))
			return
		}
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	dir := t.TempDir()
	if err := downloadFile(context.Background(), dir, "", "bad.gz", srv.URL+"/bad"); err == nil {
		t.Error("expected error for HTTP 503")
	}
	if err := downloadFile(context.Background(), dir, "", "ok.gz", srv.URL+"/ok"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

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

// TestStringToSlugMatchesMain pins the outputs that must equal the main
// module's logger.StringToSlug, since imdb slugs are compared against it.
func TestStringToSlugMatchesMain(t *testing.T) {
	tests := map[string]string{
		"日本語":          "ri-ben-yu",
		"Гарри Поттер": "garri-potter",
		"Österreich":   "oesterreich",
		"Élan Vital":   "elan-vital",
		"Björk":        "bjoerk",
		"The Matrix":   "the-matrix",
		"Ελληνικά":     "ellenika",
	}
	for in, want := range tests {
		if got := stringToSlug(in); got != want {
			t.Errorf("stringToSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

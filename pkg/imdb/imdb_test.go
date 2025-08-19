package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
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

package api

import (
	"net/url"
	"slices"
	"testing"
)

func TestFormBool(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"on", true},
		{"ON", true},
		{"true", true},
		{"True", true},
		{"1", true},
		{"yes", true},
		{" on ", true},
		{"", false},
		{"off", false},
		{"false", false},
		{"0", false},
		{"no", false},
		{"garbage", false},
	}

	for _, tt := range tests {
		if got := formBool(tt.in); got != tt.want {
			t.Errorf("formBool(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestFormRowIndexes(t *testing.T) {
	form := url.Values{
		"downloader_10_Name":         {"c"},
		"downloader_2_Name":          {"b"},
		"downloader_0_Name":          {"a"},
		"downloader_0_Hostname":      {"h"},
		"downloader_x_y_Name":        {"not numeric"},
		"indexers_1_Name":            {"other prefix"},
		"downloader_3_Name__present": {"1"},
	}

	got := formRowIndexes(form, "downloader_", "_Name")
	if want := []string{"0", "2", "10"}; !slices.Equal(got, want) {
		t.Fatalf("formRowIndexes = %v, want %v", got, want)
	}

	// Nested rows sharing the prefix must not be picked up as rows.
	nested := url.Values{
		"media_movies_0_data_0_TemplatePath":       {"p"},
		"media_movies_0_data_1_TemplatePath":       {"p"},
		"media_movies_0_dataimport_0_TemplatePath": {"p"},
	}

	got = formRowIndexes(nested, "media_movies_0_data_", "_TemplatePath")
	if want := []string{"0", "1"}; !slices.Equal(got, want) {
		t.Fatalf("nested formRowIndexes = %v, want %v", got, want)
	}

	if n := nextFormRowIndex(form, "downloader_", "_Name"); n != 11 {
		t.Fatalf("nextFormRowIndex = %d, want 11", n)
	}

	if n := nextFormRowIndex(url.Values{}, "downloader_", "_Name"); n != 0 {
		t.Fatalf("nextFormRowIndex on empty form = %d, want 0", n)
	}
}

func TestConfigBuilderSetters(t *testing.T) {
	c := newFormContext(t, url.Values{
		"g_Cleared":                      {""},
		"g_Ids":                          {"1", "2,3", ""},
		"g_List__present":                {"1"},
		"g_Float":                        {"1.5"},
		"g_BadFloat":                     {"x"},
		"g_Pen_key":                      {"a", ""},
		"g_Pen_value":                    {"0.5", ""},
		"g_Pen_key" + arrayPresentSuffix: {"1"},
		"g_Small":                        {"300"},
	})

	b := NewConfigBuilder(c, "g", "")

	cleared, absent := "old", "old"
	b.SetString(&cleared, "Cleared").SetString(&absent, "Absent")

	if cleared != "" || absent != "old" {
		t.Errorf("SetString: cleared=%q absent=%q", cleared, absent)
	}

	var ids []int
	b.SetIntArray(&ids, "Ids")

	if !slices.Equal(ids, []int{1, 2, 3}) {
		t.Errorf("SetIntArray = %v", ids)
	}

	list := []string{"old"}
	b.SetStringArrayFromForm(&list, "List")

	if len(list) != 0 {
		t.Errorf("emptied array must clear, got %v", list)
	}

	var f float64
	b.SetFloat64(&f, "Float")

	if f != 1.5 {
		t.Errorf("SetFloat64 = %v", f)
	}

	pen := map[string]float64{"old": 1}
	b.SetStringFloatMap(&pen, "Pen")

	if len(pen) != 1 || pen["a"] != 0.5 {
		t.Errorf("SetStringFloatMap = %v", pen)
	}

	if err := b.Err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bad float64

	var small uint8 = 7

	b.SetFloat64(&bad, "BadFloat").SetUint8(&small, "Small")

	if b.Err() == nil || small != 7 {
		t.Errorf("invalid input must error and keep the value, small=%d err=%v", small, b.Err())
	}
}

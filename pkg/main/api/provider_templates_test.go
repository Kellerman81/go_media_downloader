package api

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Kellerman81/go_media_downloader/pkg/main/config"
)

func TestIndexerTemplatesUseSearchableType(t *testing.T) {
	for _, tpl := range indexerTemplates {
		if typ := tpl.Fields["IndexerType"]; typ != "newznab" {
			t.Errorf("template %q uses IndexerType %q; searcher only queries newznab", tpl.Label, typ)
		}
	}
}

// TestListTemplatesFields: every list template sets fields the list form has
// and a list type the form offers.
func TestListTemplatesFields(t *testing.T) {
	fields := map[string]bool{}

	lt := reflect.TypeFor[config.ListsConfig]()
	for i := range lt.NumField() {
		fields[lt.Field(i).Name] = true
	}

	form := renderComponentToString(renderListsConfig([]config.ListsConfig{{Name: "x"}}, "tok"))
	seen := map[string]bool{}

	for _, tpl := range listTemplates {
		if seen[tpl.Label] {
			t.Errorf("label %q twice", tpl.Label)
		}

		seen[tpl.Label] = true

		for name := range tpl.Fields {
			if !fields[name] {
				t.Errorf("%s: no list field %s", tpl.Label, name)
			}
		}

		for _, name := range tpl.Required {
			if !fields[name] {
				t.Errorf("%s: no list field %s (required)", tpl.Label, name)
			}
		}

		if lt := tpl.Fields["ListType"]; !strings.Contains(form, `value="`+lt+`"`) {
			t.Errorf("%s: list type %q not offered", tpl.Label, lt)
		}
	}
}

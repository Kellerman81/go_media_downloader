package api

import (
	"testing"
)

func TestIndexerTemplatesUseSearchableType(t *testing.T) {
	for _, tpl := range indexerTemplates {
		if typ := tpl.Fields["IndexerType"]; typ != "newznab" {
			t.Errorf("template %q uses IndexerType %q; searcher only queries newznab", tpl.Label, typ)
		}
	}
}

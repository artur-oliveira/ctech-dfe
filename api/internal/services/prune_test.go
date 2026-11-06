package services

import (
	"reflect"
	"testing"
)

func TestPruneEmpty(t *testing.T) {
	in := map[string]any{
		"xNome": "A", "xFant": "", "@Id": "",
		"emit":     map[string]any{"IE": "", "xLgr": ""},
		"marcador": map[string]any{},
		"det":      []any{map[string]any{"cProd": "1", "cEAN": ""}},
	}
	want := map[string]any{
		"xNome":    "A",
		"marcador": map[string]any{},
		"det":      []any{map[string]any{"cProd": "1"}},
	}
	if got := PruneEmpty(in); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v", got)
	}
}

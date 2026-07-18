package services

import (
	"reflect"
	"sort"
	"testing"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func TestDiff_UpdateOnlyChangedFields(t *testing.T) {
	before := map[string]any{"pk": "P", "sk": "S", "description": "old", "code": "SKU1", "updated_at": "t1"}
	after := map[string]any{"description": "new", "code": "SKU1"}

	got := Diff(before, after)
	sort.Slice(got, func(i, j int) bool { return got[i].Name < got[j].Name })

	want := []repositories.Modification{{Name: "description", Before: "old", After: "new"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Diff() = %+v, want %+v", got, want)
	}
}

func TestDiff_Create(t *testing.T) {
	after := map[string]any{"description": "new", "code": "SKU1"}
	got := Diff(nil, after)
	if len(got) != 2 {
		t.Fatalf("len(got) = %d, want 2", len(got))
	}
	for _, m := range got {
		if m.Before != nil {
			t.Errorf("Before = %v, want nil for CREATE", m.Before)
		}
	}
}

func TestDiff_Delete(t *testing.T) {
	before := map[string]any{"pk": "P", "description": "old"}
	got := Diff(before, nil)
	if len(got) != 1 || got[0].Name != "description" || got[0].After != nil {
		t.Errorf("Diff() = %+v, want one modification {description, old, nil}", got)
	}
}

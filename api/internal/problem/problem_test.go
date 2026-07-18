package problem

import (
	"encoding/json"
	"testing"
)

// TestProblem_JSONShape verifies that embedding commonproblem.Problem still
// inlines its fields into the top-level JSON object (rather than nesting under
// a "Problem" key), since the response shape is a public API contract.
func TestProblem_JSONShape(t *testing.T) {
	p := BadRequest("bad input")
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, field := range []string{"type", "title", "status", "detail"} {
		if _, ok := m[field]; !ok {
			t.Errorf("expected top-level field %q in JSON, got %s", field, b)
		}
	}
	if _, ok := m["Problem"]; ok {
		t.Errorf("embedded Problem must not appear as a nested key, got %s", b)
	}
}

// TestProblem_ErrorSemantics verifies Error() preserves dfe's original
// semantics (Detail alone when present, else Title) rather than the shared
// library's "Title: Detail" format.
func TestProblem_ErrorSemantics(t *testing.T) {
	withDetail := BadRequest("bad input")
	if got := withDetail.Error(); got != "bad input" {
		t.Errorf("Error() = %q, want %q", got, "bad input")
	}

	noDetail := Validation(nil)
	noDetail.Detail = ""
	if got := noDetail.Error(); got != noDetail.Title {
		t.Errorf("Error() = %q, want title %q", got, noDetail.Title)
	}
}

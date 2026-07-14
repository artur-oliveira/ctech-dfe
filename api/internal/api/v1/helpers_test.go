package v1

import (
	"testing"
)

func TestExtractCrt_ReadsNestedPersonField(t *testing.T) {
	item := map[string]any{
		"person": map[string]any{
			"crt": float64(3),
		},
	}
	crt := extractCrt(item)
	if crt == nil || *crt != 3 {
		t.Fatalf("crt = %v, want 3", crt)
	}
}

func TestExtractCrt_MissingPersonReturnsNil(t *testing.T) {
	if crt := extractCrt(map[string]any{}); crt != nil {
		t.Errorf("crt = %v, want nil", crt)
	}
}

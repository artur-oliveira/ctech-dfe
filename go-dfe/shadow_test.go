package dfe

import "testing"

func TestJSONEqual(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want bool
	}{
		{"identical", `{"a":1,"b":2}`, `{"a":1,"b":2}`, true},
		{"different key order", `{"a":1,"b":2}`, `{"b":2,"a":1}`, true},
		{"different values", `{"a":1}`, `{"a":2}`, false},
		{"invalid a", `not json`, `{"a":1}`, false},
		{"invalid b", `{"a":1}`, `not json`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := jsonEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("jsonEqual(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestShadowCompare_SkipsWhenNotImplemented(t *testing.T) {
	// NFeAutorizacao is not in the implemented set — ShadowCompare must not
	// call Call() (which would fail loudly for a missing certificate) and
	// must simply return without side effects.
	ShadowCompare(nil, Request{DocType: "nfe", Service: "NFeAutorizacao"}, 200, "{}") //nolint:staticcheck // nil ctx fine: this path returns before ctx is ever used.
}

package nfse

import "testing"

// intOf/strSlice must handle both JSON-decoded body values (float64, []any —
// arrives via the worker's SQS/py-dfe-shaped path) and native Go values
// (int64, []string — arrives via in-process callers like
// worker/internal/service/distribution_nfse.go and
// api/internal/services/nfses/municipal.go, which build the body map
// directly with no JSON round trip). Regression test for the int64 NSU cursor
// and []string param_args being silently read as zero/empty on that
// in-process path.
func TestIntOf(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want int
	}{
		{"float64", float64(42), 42},
		{"int", 42, 42},
		{"int64", int64(42), 42},
		{"missing", nil, 0},
	}
	for _, c := range cases {
		body := map[string]any{}
		if c.v != nil {
			body["k"] = c.v
		}
		if got := intOf(body, "k"); got != c.want {
			t.Errorf("%s: intOf() = %d, want %d", c.name, got, c.want)
		}
	}
}

func TestStrSlice(t *testing.T) {
	cases := []struct {
		name string
		v    any
		want []string
	}{
		{"native []string", []string{"a", "b"}, []string{"a", "b"}},
		{"json []any", []any{"a", "b"}, []string{"a", "b"}},
		{"missing", nil, nil},
	}
	for _, c := range cases {
		body := map[string]any{}
		if c.v != nil {
			body["k"] = c.v
		}
		got := strSlice(body, "k")
		if len(got) != len(c.want) {
			t.Fatalf("%s: strSlice() = %v, want %v", c.name, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: strSlice()[%d] = %q, want %q", c.name, i, got[i], c.want[i])
			}
		}
	}
}

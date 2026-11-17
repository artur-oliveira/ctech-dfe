package v1

import "testing"

func TestEmitVariant(t *testing.T) {
	cases := map[string]string{
		`{"variant":"tm","modal":"ferroviario"}`: "tm",
		`{"variant":"normal"}`:                   "normal",
		`{"modal":"rodoviario"}`:                 "",
		`não é json`:                             "",
		``:                                       "",
	}
	for body, want := range cases {
		if got := emitVariant([]byte(body)); got != want {
			t.Errorf("emitVariant(%q) = %q, quer %q", body, got, want)
		}
	}
}

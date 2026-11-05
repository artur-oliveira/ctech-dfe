package nfes

import (
	"testing"
	"time"
)

// NT 2023.002 v1.02: a inutilização só deixa de existir em AC, BA e SP, a partir de 11/11/2026.
func TestInutilizationDisabled(t *testing.T) {
	before := time.Date(2026, time.November, 10, 23, 59, 59, 0, time.UTC)
	from := time.Date(2026, time.November, 11, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		uf   string
		now  time.Time
		want bool
	}{
		{"SP", before, false},
		{"SP", from, true},
		{"ba", from.AddDate(0, 1, 0), true},
		{" AC ", from, true},
		{"PI", from.AddDate(1, 0, 0), false},
		{"", from, false},
	}
	for _, c := range cases {
		if got := InutilizationDisabled(c.uf, c.now); got != c.want {
			t.Errorf("InutilizationDisabled(%q, %s) = %v, want %v", c.uf, c.now.Format("2006-01-02"), got, c.want)
		}
	}
}

package uftz

import (
	"testing"
	"time"
)

func TestOffsets(t *testing.T) {
	ref := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	for uf, want := range map[string]int{"SP": -3, "PI": -3, "AM": -4, "MT": -4, "MS": -4, "RO": -4, "RR": -4, "AC": -5, "": -3, "XX": -3} {
		if _, off := ref.In(Location(uf)).Zone(); off != want*3600 {
			t.Errorf("%q: offset %d, want %d", uf, off/3600, want)
		}
	}
	for uf := range zones {
		if _, err := time.LoadLocation(zones[uf]); err != nil {
			t.Errorf("%s: %v", uf, err)
		}
	}
}

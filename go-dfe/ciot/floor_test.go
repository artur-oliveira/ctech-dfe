package ciot

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Synthetic coefficients (NOT ANTT values): CCD 2.00/km, CC 100.00; axles 5 is blank, 6 has a value.
const syntheticFloor = `{"version":"synthetic","valid_from":"2026-07-17","source":"test","verified":false,"rows":[
 {"composition":true,"high_performance":false,"cargo_type":1,"axles":{"4":{"ccd":"2.00","cc":"100.00"},"6":{"ccd":"3.00","cc":"200.00"}}}]}`

func TestDefaultFloorTableIsVerifiedAndComplete(t *testing.T) {
	d := DefaultFloorTable()
	if !d.Verified || d.Version != "2026-07-17" || d.ValidFrom != "2026-07-17" || len(d.Rows) != 48 {
		t.Fatalf("embedded table: verified=%v version=%s rows=%d", d.Verified, d.Version, len(d.Rows))
	}
	seen := map[[3]int]bool{}
	for _, r := range d.Rows {
		k := [3]int{b2i(r.Composition), b2i(r.HighPerformance), r.CargoType}
		if seen[k] || r.CargoType < 1 || r.CargoType > 12 || len(r.Axles) == 0 {
			t.Fatalf("bad or duplicate row %v", k)
		}
		seen[k] = true
		for ax := range r.Axles {
			if ax < 2 || ax > 9 || ax == 8 {
				t.Fatalf("axle column %d is not an Anexo II column", ax)
			}
		}
	}
}

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

// Spot checks against the reference tables (ciot-api/internal/freight/tables.go): corners and blank cells of A to D.
func TestEmbeddedTableSpotChecks(t *testing.T) {
	d := DefaultFloorTable()
	cases := []struct {
		comp, hp     bool
		cargo, axles int
		ccd, cc      string // "" = blank cell
	}{
		{true, false, 1, 2, "4.0144", "460.59"}, {true, false, 1, 9, "9.2231", "908.91"},
		{true, false, 4, 2, "", ""}, {true, false, 12, 3, "", ""}, {true, false, 12, 7, "", ""}, {true, false, 12, 9, "9.7444", "1052.26"},
		{false, false, 1, 2, "", ""}, {false, false, 1, 3, "", ""}, {false, false, 1, 4, "5.2180", "533.40"}, {false, false, 12, 4, "", ""},
		{true, true, 5, 2, "3.3850", "172.50"}, {true, true, 3, 9, "9.1377", "351.60"}, {true, true, 12, 5, "5.7939", "249.68"},
		{false, true, 7, 6, "6.2960", "273.05"}, {false, true, 12, 9, "6.6136", "263.35"}, {false, true, 9, 2, "", ""},
	}
	for _, c := range cases {
		var got *FloorCoeffs
		for _, r := range d.Rows {
			if r.Composition == c.comp && r.HighPerformance == c.hp && r.CargoType == c.cargo {
				if v, ok := r.Axles[c.axles]; ok {
					got = &v
				}
			}
		}
		switch {
		case c.ccd == "" && got != nil:
			t.Errorf("%+v: expected blank, got %+v", c, *got)
		case c.ccd != "" && (got == nil || got.CCD != c.ccd || got.CC != c.cc):
			t.Errorf("%+v: got %v", c, got)
		}
	}
}

func TestFloorWorkedExampleTableC(t *testing.T) {
	// General cargo, 2 axles, 100 km, composition yes, high performance yes, empty return yes: Table C.
	r, err := DefaultFloorTable().Calculate(FloorInput{Composition: true, HighPerformance: true, CargoType: 5, Axles: 2, DistanceKm: 100, EmptyReturn: true})
	if err != nil || r.OutboundCents != 51100 || r.EmptyReturnCents != 31142 || r.Cents != 82242 || r.AxlesUsed != 2 ||
		r.TableVersion != "2026-07-17" || r.ValidFrom != "2026-07-17" {
		t.Fatalf("%+v %v", r, err)
	}
	if FormatMoney(r.Cents) != "822.42" {
		t.Fatalf("formatted %q", FormatMoney(r.Cents))
	}
}

func TestFloorNoEmptyReturn(t *testing.T) {
	r, err := DefaultFloorTable().Calculate(FloorInput{CargoType: 5, Axles: 4, DistanceKm: 100})
	if err != nil || r.EmptyReturnCents != 0 || r.Cents != r.OutboundCents { // Table B: 100 x 5.2180 + 533.40
		t.Fatalf("%+v %v", r, err)
	}
	if r.OutboundCents != 105520 {
		t.Fatalf("outbound %d", r.OutboundCents)
	}
}

// Anexo II Nota 1: the nearest column below the requested count first, above only if none below; the same across
// blank cells; above 9 resolves to 9 and 1 to 2.
func TestFloorAxleResolution(t *testing.T) {
	d := DefaultFloorTable()
	cases := []struct {
		name         string
		comp         bool
		cargo, axles int
		want         int
	}{
		{"exact", true, 1, 5, 5},
		{"count 1 resolves to 2", true, 1, 1, 2},
		{"8 resolves down to 7", true, 1, 8, 7},
		{"above 9 resolves to 9", true, 1, 15, 9},
		{"blank 2 in A cargo 4: nothing below, up to 3", true, 4, 2, 3},
		{"count 1 with blank 2 goes up to 3", true, 4, 1, 3},
		{"blank 7 in A cargo 12 goes below to 6", true, 12, 7, 6},
		{"8 on blank 7 passes through to 6", true, 12, 8, 6},
		{"blank 4 in A cargo 12: nothing below, up to 5", true, 12, 4, 5},
		{"B has no 2 or 3: 2 goes up to 4", false, 1, 2, 4},
		{"B 3 goes up to 4", false, 1, 3, 4},
	}
	for _, c := range cases {
		r, err := d.Calculate(FloorInput{Composition: c.comp, CargoType: c.cargo, Axles: c.axles, DistanceKm: 10})
		if err != nil || r.AxlesUsed != c.want {
			t.Errorf("%s: used %d err %v, want %d", c.name, r.AxlesUsed, err, c.want)
		}
	}
	if _, err := d.Calculate(FloorInput{CargoType: 1, Axles: 0, DistanceKm: 10}); !errors.Is(err, ErrFloorNoCoefficient) {
		t.Errorf("0 axles: %v", err)
	}
	if _, err := d.Calculate(FloorInput{CargoType: 13, Axles: 2, DistanceKm: 10}); !errors.Is(err, ErrFloorNoCoefficient) {
		t.Errorf("cargo 13: %v", err)
	}
}

func TestFloorValidFromGate(t *testing.T) {
	tab := DefaultFloorTable()
	before := time.Date(2026, 7, 16, 0, 0, 0, 0, time.UTC)
	if _, err := tab.Calculate(FloorInput{CargoType: 5, Axles: 2, DistanceKm: 1, At: before}); err == nil {
		t.Fatal("date before valid_from accepted")
	}
}

func TestUnverifiedTableIsRefused(t *testing.T) {
	tab, _ := LoadFloorTable(strings.NewReader(syntheticFloor))
	if _, err := tab.Calculate(FloorInput{}); !errors.Is(err, ErrFloorUnverified) {
		t.Fatalf("err %v", err)
	}
}

func TestFloorFormulaWithSyntheticCoefficients(t *testing.T) {
	tab, err := LoadFloorTable(strings.NewReader(syntheticFloor))
	if err != nil {
		t.Fatal(err)
	}
	in := FloorInput{Composition: true, CargoType: 1, Axles: 4, DistanceKm: 100}
	r, err := tab.calculate(in)
	if err != nil || r.Cents != 30000 { // 100*2.00 + 100.00
		t.Fatalf("%+v %v", r, err)
	}
	in.EmptyReturn = true
	if r, _ = tab.calculate(in); r.Cents != 30000+18400 { // + 0.92 * 100 * 2.00
		t.Fatalf("empty return: %d", r.Cents)
	}
	in.EmptyReturn, in.Axles = false, 5 // blank cell: nearest below (4)
	if r, _ = tab.calculate(in); r.Cents != 30000 {
		t.Fatalf("below axle: %d", r.Cents)
	}
	in.Axles = 7 // beyond the last column: nearest below (6)
	if r, _ = tab.calculate(in); r.Cents != 50000 {
		t.Fatalf("beyond: %d", r.Cents)
	}
	in.CargoType = 2
	if _, err = tab.calculate(in); !errors.Is(err, ErrFloorNoCoefficient) {
		t.Fatalf("err %v", err)
	}
}

func TestFloorRejectsBadTable(t *testing.T) {
	if _, err := LoadFloorTable(strings.NewReader(`{"version":"v","valid_from":"x"}`)); err == nil {
		t.Fatal("bad date accepted")
	}
	if _, err := LoadFloorTable(strings.NewReader(`{"version":"v","valid_from":"2026-01-01","rows":[{"axles":{"2":{"ccd":"abc","cc":"1"}}}]}`)); err == nil {
		t.Fatal("bad coefficient accepted")
	}
}

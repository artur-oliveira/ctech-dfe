package ciot

import (
	"errors"
	"strings"
	"testing"
)

// Synthetic coefficients (NOT ANTT values): CCD 2.00/km, CC 100.00; axles 5 is blank, 6 has a value.
const syntheticFloor = `{"version":"synthetic","valid_from":"2026-07-17","source":"test","verified":false,"rows":[
 {"composition":"conj","high_performance":"","cargo_type":1,"axles":{"4":{"ccd":"2.00","cc":"100.00"},"6":{"ccd":"3.00","cc":"200.00"}}}]}`

func TestDefaultFloorTableIsUnverifiedPlaceholder(t *testing.T) {
	d := DefaultFloorTable()
	if d.Verified || len(d.Rows) != 0 {
		t.Fatal("embedded table must stay an unverified placeholder")
	}
	if _, err := d.Calculate(FloorInput{}); !errors.Is(err, ErrFloorUnverified) {
		t.Fatalf("err %v", err)
	}
}

func TestFloorFormulaWithSyntheticCoefficients(t *testing.T) {
	tab, err := LoadFloorTable(strings.NewReader(syntheticFloor))
	if err != nil {
		t.Fatal(err)
	}
	in := FloorInput{Composition: "conj", CargoType: 1, Axles: 4, DistanceKm: 100}
	r, err := tab.calculate(in)
	if err != nil || r.Cents != 30000 { // 100*2.00 + 100.00
		t.Fatalf("%+v %v", r, err)
	}
	in.EmptyReturn = true
	if r, _ = tab.calculate(in); r.Cents != 30000+18400 { // + 0.92 * 100 * 2.00
		t.Fatalf("empty return: %d", r.Cents)
	}
	in.EmptyReturn, in.Axles = false, 5 // blank cell: next axle with value (6)
	if r, _ = tab.calculate(in); r.Cents != 50000 {
		t.Fatalf("next axle: %d", r.Cents)
	}
	in.Axles = 7
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

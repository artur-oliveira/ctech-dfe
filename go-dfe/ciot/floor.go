package ciot

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"time"
)

// Freight floor (spec section 7): piso = distance x CCD + CC; with empty return, plus 0.92 x distance x CCD.
// Coefficients live in a versioned JSON table loaded at runtime; the embedded default is an UNVERIFIED empty
// placeholder, so nothing is ever computed from values typed from memory.

// EmptyReturnFactor is the 0.92 empty-return surcharge on distance x CCD.
const EmptyReturnFactor = "0.92"

// maxAxlesInTable bounds the "next axle with value" search.
const maxAxlesInTable = 9

// ErrFloorUnverified: the table is not marked verified against the official publication.
var ErrFloorUnverified = errors.New("ciot: freight floor table is not verified against the official ANTT publication")

// ErrFloorNoCoefficient: no coefficient for the combination, not even on a higher axle count.
var ErrFloorNoCoefficient = errors.New("ciot: no freight floor coefficient for the combination")

//go:embed floor_table.json
var defaultFloorJSON []byte

// FloorRow holds the coefficients of one (composition, high performance, cargo type); Axles maps the axle count to
// the coefficients. A missing axle count is a blank cell. CCD and CC are decimal text (exact, no float).
type FloorRow struct {
	Composition     string              `json:"composition"`
	HighPerformance string              `json:"high_performance"` // "", "A", "B", "C" or "D"
	CargoType       int                 `json:"cargo_type"`
	Axles           map[int]FloorCoeffs `json:"axles"`
}

// FloorCoeffs: CCD is per km, CC is a fixed amount in BRL.
type FloorCoeffs struct {
	CCD string `json:"ccd"`
	CC  string `json:"cc"`
}

// FloorTable is the versioned table.
type FloorTable struct {
	Version   string     `json:"version"`
	ValidFrom string     `json:"valid_from"` // yyyy-MM-dd
	Source    string     `json:"source"`
	Verified  bool       `json:"verified"`
	Rows      []FloorRow `json:"rows"`
}

// LoadFloorTable decodes and validates a table.
func LoadFloorTable(r io.Reader) (*FloorTable, error) {
	var t FloorTable
	if err := json.NewDecoder(r).Decode(&t); err != nil {
		return nil, fmt.Errorf("ciot: floor table: %w", err)
	}
	if _, err := ParseDate(t.ValidFrom); err != nil || t.Version == "" {
		return nil, errors.New("ciot: floor table needs version and valid_from")
	}
	for _, row := range t.Rows {
		for ax, c := range row.Axles {
			for _, v := range []string{c.CCD, c.CC} {
				if _, ok := new(big.Rat).SetString(v); !ok {
					return nil, fmt.Errorf("ciot: floor table: bad coefficient %q (axles %d)", v, ax)
				}
			}
		}
	}
	return &t, nil
}

// DefaultFloorTable is the embedded table: today an UNVERIFIED empty placeholder.
func DefaultFloorTable() *FloorTable {
	t, err := LoadFloorTable(bytesReader(defaultFloorJSON))
	if err != nil {
		panic(err) // the embedded file is covered by a test
	}
	return t
}

// FloorInput is one calculation.
type FloorInput struct {
	Composition     string
	HighPerformance string
	CargoType       int
	Axles           int
	DistanceKm      int
	EmptyReturn     bool
	At              time.Time // validity check; zero skips it
}

// FloorResult carries the minimum in cents and the table version used.
type FloorResult struct {
	Cents        int64
	TableVersion string
}

// Calculate returns the floor in integer cents (rounded half up). It refuses an unverified table.
func (t *FloorTable) Calculate(in FloorInput) (FloorResult, error) {
	if !t.Verified {
		return FloorResult{}, ErrFloorUnverified
	}
	return t.calculate(in)
}

// calculate is Calculate without the verification gate (synthetic-table tests).
func (t *FloorTable) calculate(in FloorInput) (FloorResult, error) {
	if !in.At.IsZero() {
		if from, _ := ParseDate(t.ValidFrom); in.At.Before(from) {
			return FloorResult{}, fmt.Errorf("ciot: floor table %s is valid from %s", t.Version, t.ValidFrom)
		}
	}
	var coeffs *FloorCoeffs
	for _, row := range t.Rows {
		if row.Composition != in.Composition || row.HighPerformance != in.HighPerformance || row.CargoType != in.CargoType {
			continue
		}
		for ax := in.Axles; ax <= maxAxlesInTable; ax++ { // blank cell: next axle count with a value
			if c, ok := row.Axles[ax]; ok {
				coeffs = &c
				break
			}
		}
	}
	if coeffs == nil {
		return FloorResult{}, ErrFloorNoCoefficient
	}
	ccd, _ := new(big.Rat).SetString(coeffs.CCD)
	cc, _ := new(big.Rat).SetString(coeffs.CC)
	dist := new(big.Rat).SetInt64(int64(in.DistanceKm))
	perKm := new(big.Rat).Mul(dist, ccd)
	total := new(big.Rat).Add(perKm, cc)
	if in.EmptyReturn {
		f, _ := new(big.Rat).SetString(EmptyReturnFactor)
		total.Add(total, new(big.Rat).Mul(f, perKm))
	}
	total.Mul(total, big.NewRat(100, 1))
	// round half up to cents
	total.Add(total, big.NewRat(1, 2))
	cents := new(big.Int).Quo(total.Num(), total.Denom())
	return FloorResult{Cents: cents.Int64(), TableVersion: t.Version}, nil
}

func bytesReader(b []byte) io.Reader { return bytes.NewReader(b) }

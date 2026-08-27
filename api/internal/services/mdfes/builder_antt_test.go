package mdfes

import "testing"

func TestCategCombVeic(t *testing.T) {
	for trailers, want := range map[int]string{0: "02", 1: "04", 2: "06", 3: "07", 4: "07"} {
		if got := categCombVeic(trailers); got != want {
			t.Fatalf("%d reboques: want %q, got %q", trailers, want, got)
		}
	}
}

func TestBuildRodoIncluiCIntECapM3(t *testing.T) {
	p := buildParams{vehicle: resolvedVehicle{
		Placa: "ABC1D23", Tara: "5000", TpRod: "06", TpCar: "02", UF: "PI",
		CInt: "T-01", CapM3: "90",
	}}
	veic := p.buildRodo()["veicTracao"].(map[string]any)
	if veic["cInt"] != "T-01" || veic["capM3"] != "90" {
		t.Fatalf("cInt/capM3 ausentes: %v", veic)
	}
}

func TestBuildRodoReboqueIncluiCIntECapM3(t *testing.T) {
	p := buildParams{
		vehicle:  resolvedVehicle{Placa: "ABC1D23", Tara: "5000", TpRod: "06", TpCar: "02", UF: "PI"},
		trailers: []resolvedVehicle{{Placa: "XYZ9Z99", Tara: "3000", TpCar: "02", CInt: "R-01", CapM3: "60"}},
	}
	reb := p.buildRodo()["veicReboque"].([]map[string]any)[0]
	if reb["cInt"] != "R-01" || reb["capM3"] != "60" {
		t.Fatalf("cInt/capM3 do reboque ausentes: %v", reb)
	}
}

func TestBuildEnderMDFeIncluiXCpl(t *testing.T) {
	person := map[string]any{"addresses": []any{map[string]any{
		"street": "Rua A", "number": "10", "complement": "Sala 2",
		"neighborhood": "Centro", "city": "Teresina", "city_ibge_code": "2211001",
		"state_federation": "PI", "postal_code": "64000-000",
	}}}
	if got := buildEnderMDFe(person); got["xCpl"] != "Sala 2" {
		t.Fatalf("xCpl ausente: %v", got)
	}
}

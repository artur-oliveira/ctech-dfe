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

func TestBuildValePedComDispECategoria(t *testing.T) {
	p := buildParams{
		trailers: []resolvedVehicle{{Placa: "R1"}},
		tolls: []resolvedToll{
			{CNPJForn: "11111111111111", CNPJPg: "22222222222222", NCompra: "C-1", VValePed: "150.00", TpValePed: "01"},
		},
	}
	got := p.buildValePed()
	disp := got["disp"].([]map[string]any)
	if len(disp) != 1 || disp[0]["CNPJForn"] != "11111111111111" || disp[0]["vValePed"] != "150.00" {
		t.Fatalf("disp errado: %v", disp)
	}
	if got["categCombVeic"] != "04" {
		t.Fatalf("categoria derivada errada: %v", got["categCombVeic"])
	}
}

func TestBuildValePedSemValeDevolveNil(t *testing.T) {
	if (buildParams{}).buildValePed() != nil {
		t.Fatal("valePed sem vale tem que ser omitido")
	}
}

// CNPJPg e CPFPg são um choice: informados os dois no cadastro, só o CNPJ sai.
func TestBuildValePedPagadorEChoice(t *testing.T) {
	p := buildParams{tolls: []resolvedToll{{
		CNPJForn: "1", CNPJPg: "22222222222222", CPFPg: "33333333333",
		NCompra: "C-1", VValePed: "10.00",
	}}}
	disp := p.buildValePed()["disp"].([]map[string]any)[0]
	if _, ok := disp["CPFPg"]; ok {
		t.Fatalf("CPFPg não pode coexistir com CNPJPg: %v", disp)
	}
}

func TestBuildInfANTTIncluiValePed(t *testing.T) {
	p := buildParams{tolls: []resolvedToll{{CNPJForn: "1", NCompra: "C-1", VValePed: "10.00"}}}
	if _, ok := p.buildInfANTT()["valePed"]; !ok {
		t.Fatalf("valePed ausente em infANTT: %v", p.buildInfANTT())
	}
}

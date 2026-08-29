package nfes

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

// updateGolden rewrites the golden files instead of comparing against them.
var updateGolden = flag.Bool("update", false, "rewrite golden files")

// assertGolden compares got against the file at path, or rewrites it when the
// -update flag is passed.
func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if *updateGolden {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden ausente (rode com -update): %v", err)
	}
	if !bytes.Equal(want, got) {
		t.Fatalf("árvore mudou.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func goldenOrg() map[string]any {
	return map[string]any{
		"name": "Emit Ltda",
		"person": map[string]any{
			"crt":          float64(3),
			"fantasy_name": "Emit",
			"emails":       []any{map[string]any{"email": "emit@example.com"}},
			"phones":       []any{map[string]any{"phone": "8630000000"}},
			"state_registrations": []any{
				map[string]any{"uf": "PI", "state_registration": "194000000"},
			},
			"addresses": []any{
				map[string]any{
					"street": "Rua Emit", "number": "10", "neighborhood": "Centro",
					"city": "Teresina", "city_ibge_code": "2211001",
					"state_federation": "PI", "postal_code": "64000-000",
				},
			},
		},
	}
}

func goldenReceiver() map[string]any {
	return map[string]any{
		"sk":   "CNPJ_11222333000181",
		"name": "Dest Ltda",
		"person": map[string]any{
			"emails": []any{map[string]any{"email": "dest@example.com"}},
			"state_registrations": []any{
				map[string]any{"uf": "SP", "state_registration": "111222333444"},
			},
			"addresses": []any{
				map[string]any{
					"street": "Rua Dest", "number": "20", "neighborhood": "Centro",
					"city": "Sao Paulo", "city_ibge_code": "3550308",
					"state_federation": "SP", "postal_code": "01000-000",
				},
			},
		},
	}
}

func goldenItems() []map[string]any {
	return []map[string]any{
		{
			"product_code": "P1",
			"description":  "Produto 1",
			"ncm":          "84713012",
			"cfop":         "6102",
			"unit":         "UN",
			"quantity":     "2",
			"unit_value":   "50.00",
			"discount":     "0.00",
			"origin":       "0",
			"net_weight":   "1.500",
			"gross_weight": "2.000",
			"cfop_config": []any{
				map[string]any{
					"cfop": "6102", "icms": "00", "icms_aliq": "18.00",
					"pis": "01", "pis_aliq": "1.65",
					"cofins": "01", "cofins_aliq": "7.60",
					"ipi_cst": "50", "ipi_aliq": "5.00",
					"ibs_cbs_cst": "000", "ibs_cbs_class_trib": "000001",
					"ibs_uf_aliq": "0.1000", "ibs_mun_aliq": "0.0500", "cbs_aliq": "0.9000",
				},
			},
		},
	}
}

func goldenPayments() []map[string]any {
	return []map[string]any{
		{"payment_type": "01", "value": "100.00", "ind_pag": "0"},
	}
}

// TestBuildEnviNFeGolden congela a árvore produzida hoje. O split da Task 1 não
// pode mudar um byte dela; qualquer tarefa posterior que mude a saída tem que
// atualizar o golden no mesmo commit, de propósito e à vista.
func TestBuildEnviNFeGolden(t *testing.T) {
	got := BuildEnviNFe(
		goldenOrg(), goldenReceiver(), "CNPJ_11647612000197",
		goldenItems(), goldenPayments(),
		1, 1, 2,
		"22260811647612000197550010000000011100000015",
		decimal.RequireFromString("100.00"), decimal.Zero,
		nil, time.Date(2026, 8, 27, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		nil, "1", "1", "1", "1",
		nil, nil, nil, nil,
		TechData{CNPJ: "11647612000197", Name: "Ctech", Email: "t@t.com", Phone: "8630000000", Version: "1.0"},
		nfModel55, nil, nil, nil, NormalEmission(nfModel55),
		docExtras{},
	)
	b, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	assertGolden(t, "testdata/envinfe_golden.json", b)
}

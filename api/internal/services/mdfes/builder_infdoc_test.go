package mdfes

import "testing"

const testNFeKey = "22260811647612000197550010000000011100000015"

func infNFeNodes(t *testing.T, p buildParams) []map[string]any {
	t.Helper()
	mun := p.buildInfDoc()["infMunDescarga"].([]map[string]any)[0]
	return mun["infNFe"].([]map[string]any)
}

func TestBuildInfDocSegCodBarra(t *testing.T) {
	p := buildParams{
		cargo: &resolvedCargo{descarga: []descargaGroup{{
			mun:     MdfeMun{IBGECode: "2211001", City: "Teresina"},
			nfeKeys: []string{testNFeKey},
		}}},
		redelivery: map[string]bool{testNFeKey: true},
	}
	nfe := infNFeNodes(t, p)[0]
	if nfe["SegCodBarra"] != testNFeKey {
		t.Fatalf("SegCodBarra ausente: %v", nfe)
	}
	if nfe["indReentrega"] != indReentregaSim {
		t.Fatalf("indReentrega ausente: %v", nfe)
	}
}

// Sem reentrega marcada o campo não sai: indReentrega só admite "1".
func TestBuildInfDocSemReentrega(t *testing.T) {
	p := buildParams{cargo: &resolvedCargo{descarga: []descargaGroup{{
		mun:     MdfeMun{IBGECode: "2211001", City: "Teresina"},
		nfeKeys: []string{testNFeKey},
	}}}}
	if _, ok := infNFeNodes(t, p)[0]["indReentrega"]; ok {
		t.Fatal("indReentrega não devia existir")
	}
}

// O CT-e tem o mesmo par de campos que a NF-e no leiaute.
func TestBuildInfDocCTeSegCodBarra(t *testing.T) {
	key := "22260811647612000197570010000000011100000015"
	p := buildParams{
		cargo: &resolvedCargo{descarga: []descargaGroup{{
			mun:     MdfeMun{IBGECode: "2211001", City: "Teresina"},
			cteKeys: []string{key},
		}}},
		redelivery: map[string]bool{key: true},
	}
	cte := p.buildInfDoc()["infMunDescarga"].([]map[string]any)[0]["infCTe"].([]map[string]any)[0]
	if cte["SegCodBarra"] != key || cte["indReentrega"] != indReentregaSim {
		t.Fatalf("CT-e sem SegCodBarra/indReentrega: %v", cte)
	}
}

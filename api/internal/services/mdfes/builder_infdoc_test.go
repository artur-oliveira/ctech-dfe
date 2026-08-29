package mdfes

import "testing"

const (
	testNFeKey  = "22260811647612000197550010000000011100000015"
	testCTeKey  = "22260811647612000197570010000000011100000015"
	testMDFeKey = "22260811647612000197580010000000011100000015"
)

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
	key := testCTeKey
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

func TestBuildTotQMDFe(t *testing.T) {
	p := baseParams(nil)
	p.transportedMdfes = []MdfeTransportedBody{
		{AccessKey: testNFeKey, Unloading: MdfeMun{IBGECode: "2211001", City: "Teresina"}},
		{AccessKey: testCTeKey, Unloading: MdfeMun{IBGECode: "2211001", City: "Teresina"}},
	}
	if got := p.buildTot()["qMDFe"]; got != "2" {
		t.Fatalf("qMDFe é contagem derivada: got %v", got)
	}
}

// Sem MDF-e transportado o campo não sai — zero não é uma quantidade a declarar.
func TestBuildTotSemQMDFe(t *testing.T) {
	if _, ok := baseParams(nil).buildTot()["qMDFe"]; ok {
		t.Fatal("qMDFe não devia existir")
	}
}

func TestBuildInfDocEntregaParcial(t *testing.T) {
	p := buildParams{
		cargo: &resolvedCargo{descarga: []descargaGroup{{
			mun:     MdfeMun{IBGECode: "2211001", City: "Teresina"},
			cteKeys: []string{testCTeKey},
		}}},
		partial: map[string]partialDelivery{testCTeKey: {
			QtdTotal: "10.0000", QtdParcial: "4.0000", NFeKeys: []string{testNFeKey},
		}},
	}
	cte := p.buildInfDoc()["infMunDescarga"].([]map[string]any)[0]["infCTe"].([]map[string]any)[0]
	ep := cte["infEntregaParcial"].(map[string]any)
	if ep["qtdTotal"] != "10.0000" || ep["qtdParcial"] != "4.0000" {
		t.Fatalf("infEntregaParcial errado: %v", ep)
	}
	if cte["indPrestacaoParcial"] != indPrestacaoParcialSim {
		t.Fatalf("indPrestacaoParcial ausente: %v", cte)
	}
	if len(cte["infNFePrestParcial"].([]map[string]any)) != 1 {
		t.Fatalf("infNFePrestParcial ausente: %v", cte)
	}
}

// O XSD só prevê entrega parcial no CT-e: uma chave de NF-e marcada é ignorada.
func TestBuildInfDocEntregaParcialSoNoCTe(t *testing.T) {
	p := buildParams{
		cargo: &resolvedCargo{descarga: []descargaGroup{{
			mun:     MdfeMun{IBGECode: "2211001", City: "Teresina"},
			nfeKeys: []string{testNFeKey},
		}}},
		partial: map[string]partialDelivery{testNFeKey: {QtdTotal: "10.0000", QtdParcial: "4.0000"}},
	}
	if _, ok := infNFeNodes(t, p)[0]["infEntregaParcial"]; ok {
		t.Fatal("infNFe não tem entrega parcial no leiaute")
	}
}

func TestBuildInfDocMDFeTranspEmMunicipioNovo(t *testing.T) {
	p := buildParams{
		cargo: &resolvedCargo{descarga: []descargaGroup{{
			mun:     MdfeMun{IBGECode: "2211001", City: "Teresina"},
			nfeKeys: []string{testNFeKey},
		}}},
		transportedMdfes: []MdfeTransportedBody{
			{AccessKey: testMDFeKey, Unloading: MdfeMun{IBGECode: "2304400", City: "Fortaleza"}},
		},
	}
	muns := p.buildInfDoc()["infMunDescarga"].([]map[string]any)
	if len(muns) != 2 {
		t.Fatalf("município só do MDF-e transportado tem que virar grupo: %v", muns)
	}
	transp := muns[1]["infMDFeTransp"].([]map[string]any)[0]
	if transp["chMDFe"] != testMDFeKey {
		t.Fatalf("chMDFe errado: %v", transp)
	}
}

// Mesmo município já existente recebe o infMDFeTransp no grupo que já existe.
func TestBuildInfDocMDFeTranspNoMesmoMunicipio(t *testing.T) {
	mun := MdfeMun{IBGECode: "2211001", City: "Teresina"}
	p := buildParams{
		cargo:            &resolvedCargo{descarga: []descargaGroup{{mun: mun, nfeKeys: []string{testNFeKey}}}},
		transportedMdfes: []MdfeTransportedBody{{AccessKey: testMDFeKey, Unloading: mun}},
	}
	muns := p.buildInfDoc()["infMunDescarga"].([]map[string]any)
	if len(muns) != 1 || len(muns[0]["infMDFeTransp"].([]map[string]any)) != 1 {
		t.Fatalf("infMDFeTransp devia entrar no grupo existente: %v", muns)
	}
}

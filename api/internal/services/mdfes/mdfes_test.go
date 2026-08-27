package mdfes

import (
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const sampleNFeXML = `<?xml version="1.0" encoding="UTF-8"?>
<nfeProc xmlns="http://www.portalfiscal.inf.br/nfe" versao="4.00">
  <NFe>
    <infNFe versao="4.00" Id="NFe35260612345678000190550010000000011000000017">
      <emit>
        <CNPJ>12345678000190</CNPJ>
        <xNome>EMITENTE SP</xNome>
        <enderEmit>
          <cMun>3550308</cMun>
          <xMun>Sao Paulo</xMun>
          <UF>SP</UF>
        </enderEmit>
      </emit>
      <dest>
        <CNPJ>98765432000110</CNPJ>
        <xNome>DEST RJ</xNome>
        <enderDest>
          <cMun>3304557</cMun>
          <xMun>Rio de Janeiro</xMun>
          <UF>RJ</UF>
        </enderDest>
      </dest>
      <det nItem="1">
        <prod>
          <xProd>PARAFUSO</xProd>
          <NCM>73181500</NCM>
          <vProd>100.00</vProd>
        </prod>
      </det>
      <det nItem="2">
        <prod>
          <xProd>MOTOR ELETRICO</xProd>
          <NCM>85011019</NCM>
          <vProd>900.00</vProd>
        </prod>
      </det>
      <transp>
        <vol>
          <pesoL>40.000</pesoL>
          <pesoB>50.500</pesoB>
        </vol>
      </transp>
    </infNFe>
  </NFe>
</nfeProc>`

func TestExtractCargoNFe(t *testing.T) {
	c, err := extractCargo("35260612345678000190550010000000011000000017", docTypeNFe, []byte(sampleNFeXML))
	if err != nil {
		t.Fatalf("extractCargo: %v", err)
	}
	if c.emit.cMun != "3550308" || c.emit.uf != "SP" {
		t.Errorf("emit municipality = %s/%s, want 3550308/SP", c.emit.cMun, c.emit.uf)
	}
	if c.dest.cMun != "3304557" || c.dest.uf != "RJ" {
		t.Errorf("dest municipality = %s/%s, want 3304557/RJ", c.dest.cMun, c.dest.uf)
	}
	if !c.weightKG.Equal(decimal.RequireFromString("50.500")) {
		t.Errorf("weight = %s, want 50.500 (pesoB preferred over pesoL)", c.weightKG)
	}
	if !c.totalValue.Equal(decimal.RequireFromString("1000.00")) {
		t.Errorf("totalValue = %s, want 1000.00", c.totalValue)
	}
	if c.predNCM != "85011019" || c.predProd != "MOTOR ELETRICO" {
		t.Errorf("predominant = %s/%s, want 85011019/MOTOR ELETRICO (highest vProd)", c.predNCM, c.predProd)
	}
}

func TestValidateSingleDocType(t *testing.T) {
	key := "35260612345678000190550010000000011000000017"
	if _, err := validateSingleDocType([]MdfeDocRef{{Type: docTypeNFe, AccessKey: key}, {Type: docTypeCTe, AccessKey: key}}); err == nil {
		t.Error("expected error mixing NF-e and CT-e")
	}
	if _, err := validateSingleDocType([]MdfeDocRef{{Type: docTypeNFe, AccessKey: "123"}}); err == nil {
		t.Error("expected error for invalid 44-digit key")
	}
	dt, err := validateSingleDocType([]MdfeDocRef{{Type: docTypeNFe, AccessKey: key}})
	if err != nil || dt != docTypeNFe {
		t.Errorf("got (%q,%v), want (nfe,nil)", dt, err)
	}
}

// buildTestCargo returns a resolved cargo with two NF-e docs from different
// emitter municipalities, both destined to the same unloading municipality.
func buildTestCargo() *resolvedCargo {
	return &resolvedCargo{
		docs: []*docCargo{
			{accessKey: "111", docType: docTypeNFe, emit: party{cMun: "3550308", xMun: "Sao Paulo", uf: "SP"}, dest: party{cMun: "3304557", xMun: "Rio de Janeiro", uf: "RJ"}},
			{accessKey: "222", docType: docTypeNFe, emit: party{cMun: "3509502", xMun: "Campinas", uf: "SP"}, dest: party{cMun: "3304557", xMun: "Rio de Janeiro", uf: "RJ"}},
		},
		carrega: []MdfeMun{{IBGECode: "3550308", City: "Sao Paulo"}, {IBGECode: "3509502", City: "Campinas"}},
		descarga: []descargaGroup{
			{mun: MdfeMun{IBGECode: "3304557", City: "Rio de Janeiro"}, nfeKeys: []string{"111", "222"}},
		},
		totalWeight: decimal.RequireFromString("101.000"),
		totalValue:  decimal.RequireFromString("2000.00"),
		prodPred:    MdfeProdPred{TpCarga: defaultTpCarga, XProd: "MOTOR ELETRICO", NCM: "85011019"},
		ufIni:       "SP",
		ufFim:       "RJ",
	}
}

func ptrStr(s string) *string { return &s }

func testOrg() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"name": &types.AttributeValueMemberS{Value: "CTECH"},
		"person": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"fantasy_name": &types.AttributeValueMemberS{Value: "CTECH"},
			"addresses": &types.AttributeValueMemberL{Value: []types.AttributeValue{
				&types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
					"street":           &types.AttributeValueMemberS{Value: "Rua Teste"},
					"number":           &types.AttributeValueMemberS{Value: "100"},
					"neighborhood":     &types.AttributeValueMemberS{Value: "Centro"},
					"city":             &types.AttributeValueMemberS{Value: "Sao Paulo"},
					"city_ibge_code":   &types.AttributeValueMemberS{Value: "3550308"},
					"postal_code":      &types.AttributeValueMemberS{Value: "01000-000"},
					"state_federation": &types.AttributeValueMemberS{Value: "SP"},
				}},
			}},
		}},
	}
}

func TestBuildEnviMDFe_Structure(t *testing.T) {
	now := time.Date(2026, 6, 12, 10, 0, 0, 0, time.UTC)
	accessKey := services.GenerateAccessKey("SP", "12345678000190", services.ModelMDFe, 1, 1, now, services.TpEmisNormal)
	if len(accessKey) != 44 {
		t.Fatalf("access key len = %d, want 44", len(accessKey))
	}

	out := BuildMDFe(buildParams{
		org:         testOrg(),
		orgPK:       "CNPJ_12345678000190",
		accessKey:   accessKey,
		serie:       1,
		number:      1,
		environment: 2,
		now:         now,
		cargo:       buildTestCargo(),
		vehicle:     resolvedVehicle{Placa: "ABC1D23", Tara: "10000", UF: "SP", TpRod: "01", TpCar: "00", RNTRC: "12345678"},
		drivers:     []MdfeDriver{{Name: "MOTORISTA", CPF: "111.222.333-44"}},
		tripStart:   new("2026-06-12T08:00:00-03:00"),
	})

	mdfe := out["MDFe"].(map[string]any)
	if mdfe["@xmlns"] != mdfeXMLNS {
		t.Errorf("MDFe @xmlns = %v, want %s", mdfe["@xmlns"], mdfeXMLNS)
	}
	infMDFe := mdfe["infMDFe"].(map[string]any)

	ide := infMDFe["ide"].(map[string]any)
	if ide["UFIni"] != "SP" || ide["UFFim"] != "RJ" {
		t.Errorf("UFIni/UFFim = %v/%v, want SP/RJ", ide["UFIni"], ide["UFFim"])
	}
	if ide["mod"] != services.ModelMDFe || ide["modal"] != modalCodeRodoviario {
		t.Errorf("mod/modal = %v/%v, want 58/01", ide["mod"], ide["modal"])
	}
	munCarrega := ide["infMunCarrega"].([]map[string]any)
	if len(munCarrega) != 2 {
		t.Errorf("infMunCarrega len = %d, want 2", len(munCarrega))
	}
	if ide["dhIniViagem"] != "2026-06-12T08:00:00-03:00" {
		t.Errorf("dhIniViagem = %v, want trip start", ide["dhIniViagem"])
	}

	infDoc := infMDFe["infDoc"].(map[string]any)["infMunDescarga"].([]map[string]any)
	if len(infDoc) != 1 {
		t.Fatalf("infMunDescarga len = %d, want 1 (both docs same dest)", len(infDoc))
	}
	nfes := infDoc[0]["infNFe"].([]map[string]any)
	if len(nfes) != 2 {
		t.Errorf("infNFe len = %d, want 2", len(nfes))
	}

	tot := infMDFe["tot"].(map[string]any)
	if tot["qNFe"] != "2" || tot["cUnid"] != cUnidKG {
		t.Errorf("tot qNFe/cUnid = %v/%v, want 2/%s", tot["qNFe"], tot["cUnid"], cUnidKG)
	}
	if tot["vCarga"] != "2000.00" || tot["qCarga"] != "101.0000" {
		t.Errorf("tot vCarga/qCarga = %v/%v, want 2000.00/101.0000", tot["vCarga"], tot["qCarga"])
	}

	// CPF must be stripped of formatting in the condutor node.
	rodo := infMDFe["infModal"].(map[string]any)["rodo"].(map[string]any)
	cond := rodo["veicTracao"].(map[string]any)["condutor"].([]map[string]any)
	if cond[0]["CPF"] != "11122233344" {
		t.Errorf("condutor CPF = %v, want 11122233344 (digits only)", cond[0]["CPF"])
	}
}

// eventDetEvento drills into an envEventoMDFe body to return the detEvento map.
func eventDetEvento(body map[string]any, part string) map[string]any {
	ev, _ := body["eventoMDFe"].(map[string]any)
	inf, _ := ev["infEvento"].(map[string]any)
	det, _ := inf["detEvento"].(map[string]any)
	specificevent, _ := det[part].(map[string]any)
	return specificevent
}

func TestBuildEventEnvelope_Encerramento(t *testing.T) {
	s := &MdfeService{}
	ec := &eventContext{environment: 2, cnpj: "12345678000190", docTag: services.TagCNPJ}
	key := "35260612345678000190580010000000011000000017"
	body := s.buildEventEnvelope(ec, key, TpEventoEncerramento, 1, map[string]any{
		"evEncMDFe": map[string]any{
			"descEvento": "Encerramento", "nProt": "135250000000001", "cUF": "35", "cMun": "3550308",
		},
	})
	det := eventDetEvento(body, "evEncMDFe")
	if det == nil || det["descEvento"] != "Encerramento" {
		t.Fatalf("detEvento not built correctly: %+v", det)
	}
	inf := body["eventoMDFe"].(map[string]any)["infEvento"].(map[string]any)
	if inf["tpEvento"] != TpEventoEncerramento {
		t.Errorf("tpEvento = %v, want %s", inf["tpEvento"], TpEventoEncerramento)
	}
	if inf["CNPJ"] != "12345678000190" {
		t.Errorf("CNPJ = %v, want 12345678000190", inf["CNPJ"])
	}
}

// Regression: MDF-e event dispatch (cancel/encerrar) must send the 2-letter UF
// abbreviation to py-dfe, not the numeric IBGE cUF code embedded in the access
// key. Previously emitUF was accessKey[0:2] ("22"), causing KeyError in py-dfe's
// endpoint resolver.
func TestEmitUFFromAccessKey(t *testing.T) {
	cases := []struct {
		name      string
		accessKey string
		want      string
	}{
		{"PI", "22260612345678000190580010000000011000000017", "PI"},
		{"SP", "35260612345678000190580010000000011000000017", "SP"},
		{"RS", "43260612345678000190580010000000011000000017", "RS"},
		{"too short", "2", ""},
		{"unknown code", "99260612345678000190580010000000011000000017", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := emitUFFromAccessKey(tc.accessKey); got != tc.want {
				t.Errorf("emitUFFromAccessKey(%q) = %q, want %q", tc.accessKey, got, tc.want)
			}
		})
	}
}

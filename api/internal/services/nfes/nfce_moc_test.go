package nfes

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/go-dfe/uftz"
)

// Regressões da auditoria MOC NFC-e (docs/specs/2026-11-09-auditoria-moc-nfce.md).

func nfceImposto(t *testing.T, model string, cfg map[string]any) map[string]any {
	t.Helper()
	items := goldenItems()
	base := map[string]any{"cfop": "6102", "icms": "00", "icms_aliq": "18.00", "ibs_cbs_cst": "000", "ibs_cbs_class_trib": "000001"}
	for k, v := range cfg {
		base[k] = v
	}
	items[0]["cfop_config"] = []any{base}
	got := BuildEnviNFe(goldenOrg(), goldenReceiver(), "CNPJ_11647612000197", items, goldenPayments(),
		1, 1, 2, "22260811647612000197650010000000011100000015", decimal.RequireFromString("100.00"), decimal.Zero,
		nil, time.Date(2026, 8, 27, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)), nil, "1", "1", "1", "1",
		nil, nil, nil, nil, TechData{CNPJ: "11647612000197", Name: "Ctech", Email: "t@t.com", Phone: "8630000000", Version: "1.0"},
		model, nil, nil, nil, NormalEmission(model), docExtras{})
	inf := got["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)
	return inf["det"].([]map[string]any)[0]["imposto"].(map[string]any)
}

// M7: o perfil fiscal do produto traz IPI de uma NF-e; a NFC-e não pode sair com o grupo (O01-10, rej. 742).
func TestNFCeBuilderOmitsForbiddenGroups(t *testing.T) {
	cfg := map[string]any{"ipi_cst": "50", "ipi_aliq": "5.00"}
	if _, ok := nfceImposto(t, nfModel55, cfg)["IPI"]; !ok {
		t.Fatal("controle: a NF-e deveria emitir IPI")
	}
	for _, g := range []string{"IPI", "II", "PISST", "COFINSST", "ICMSUFDest"} {
		if _, ok := nfceImposto(t, nfModel65, cfg)[g]; ok {
			t.Errorf("NFC-e emitiu o grupo vedado %s", g)
		}
	}
}

// M6: YA02-10 e YA02-40.
func TestNFCeRejectsTPag14And90(t *testing.T) {
	for _, tp := range []string{"14", "90"} {
		if checkNfcePayments([]NfePaymentItem{{PaymentType: tp}}) == nil {
			t.Errorf("tPag %s aceito na NFC-e", tp)
		}
	}
	if err := checkNfcePayments([]NfePaymentItem{{PaymentType: "01"}, {PaymentType: "23"}, {PaymentType: "91"}}); err != nil {
		t.Errorf("tPag válido recusado: %v", err)
	}
}

// M2: 23h30 em Brasília é dia seguinte em UTC; dhEmi, AAMM e dia do QR saem do fuso da UF.
func TestNFCeDhEmiInUFZone(t *testing.T) {
	utc := time.Date(2026, 9, 1, 2, 30, 0, 0, time.UTC) // 31/08 23:30 em SP, 22:30 em AM
	sp, am := utc.In(uftz.Location("SP")), utc.In(uftz.Location("AM"))
	if got := fmtDhEmi(sp); got != "2026-08-31T23:30:00-03:00" {
		t.Errorf("SP dhEmi = %s", got)
	}
	if got := fmtDhEmi(am); got != "2026-08-31T22:30:00-04:00" {
		t.Errorf("AM dhEmi = %s", got)
	}
	if got := sp.Format("0601"); got != "2608" {
		t.Errorf("AAMM = %s, want 2608 (mês local)", got)
	}
}

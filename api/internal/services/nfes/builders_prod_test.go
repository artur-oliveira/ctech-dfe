package nfes

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestBuildDIDerivaNumeroDaAdicao(t *testing.T) {
	di := map[string]any{
		"n_di": "2026/0000001", "d_di": "2026-01-15", "x_loc_desemb": "PORTO DE ITAQUI",
		"uf_desemb": "MA", "d_desemb": "2026-01-20", "tp_via_transp": "01",
		"v_afrmm": "150.00", "tp_intermedio": "1", "c_exportador": "EXP-1",
		"additions": []any{
			map[string]any{"n_adicao": "1", "c_fabricante": "F1", "v_desc_di": "0.00"},
			map[string]any{"n_adicao": "2", "c_fabricante": "F2", "v_desc_di": "5.00"},
		},
	}
	got := buildDI(di, 2, 1, "")
	if got["nDI"] != "2026/0000001" || got["UFDesemb"] != "MA" || got["vAFRMM"] != "150.00" {
		t.Fatalf("cabeçalho da DI errado: %v", got)
	}
	adi := got["adi"].([]map[string]any)
	if len(adi) != 1 || adi[0]["nAdicao"] != "2" || adi[0]["nSeqAdic"] != "1" || adi[0]["cFabricante"] != "F2" {
		t.Fatalf("adição derivada errada: %v", adi)
	}
}

// O nDraw do embarque vence o cadastrado na adição.
func TestBuildDINDrawDaEmissaoVence(t *testing.T) {
	di := map[string]any{"additions": []any{map[string]any{"n_adicao": "1", "n_draw": "CADASTRO"}}}
	if got := buildDI(di, 1, 1, "EMBARQUE")["adi"].([]map[string]any)[0]; got["nDraw"] != "EMBARQUE" {
		t.Fatalf("nDraw errado: %v", got)
	}
	if got := buildDI(di, 1, 1, "")["adi"].([]map[string]any)[0]; got["nDraw"] != "CADASTRO" {
		t.Fatalf("nDraw do cadastro perdido: %v", got)
	}
}

// Índice fora da lista não inventa adição — o nó sai sem adi e a SEFAZ recusa,
// que é melhor que emitir uma adição errada.
func TestBuildDIAdicaoInexistente(t *testing.T) {
	if _, ok := buildDI(map[string]any{"additions": []any{}}, 3, 1, "")["adi"]; ok {
		t.Fatal("adição inexistente não pode virar nó")
	}
}

func TestBuildProdNVEnFCIeCodigosDeBarra(t *testing.T) {
	item := map[string]any{
		"product_code": "P1", "nve": []any{"AA0001", "BB0002"},
		"n_fci":   "0A1B2C3D-4E5F-6789-ABCD-EF0123456789",
		"c_barra": "7891234567890", "c_barra_trib": "7899999999999",
	}
	prod := buildProd(item, prodParams{
		Description: "Produto", Unit: "UN", TaxableUnit: "UN",
		QTrib: "1.0000", VUnTrib: "10.00", VProd: "10.00",
		Disc: decimal.Zero, VFrete: decimal.Zero, VSeg: decimal.Zero, VOutro: decimal.Zero,
	})
	nve := prod["NVE"].([]string)
	if len(nve) != 2 || nve[0] != "AA0001" {
		t.Fatalf("NVE errado: %v", prod["NVE"])
	}
	if prod["nFCI"] != "0A1B2C3D-4E5F-6789-ABCD-EF0123456789" {
		t.Fatalf("nFCI ausente: %v", prod)
	}
	if prod["cBarra"] != "7891234567890" || prod["cBarraTrib"] != "7899999999999" {
		t.Fatalf("códigos de barra ausentes: %v", prod)
	}
}

func TestBuildExportaUsaLocalDeRetiradaSalvo(t *testing.T) {
	op := map[string]any{"export_uf_saida_pais": "PI", "export_loc_despacho_index": 0}
	pickups := []any{map[string]any{"x_lgr": "Porto de Luís Correia", "x_mun": "Luis Correia"}}
	got := buildExporta(op, pickups)
	if got["UFSaidaPais"] != "PI" || got["xLocDespacho"] != "Porto de Luís Correia" {
		t.Fatalf("exporta errado: %v", got)
	}
	if got["xLocExporta"] != "Luis Correia" {
		t.Fatalf("município de saída ausente: %v", got)
	}
}

// Índice fora da lista não inventa local: só a UF sai.
func TestBuildExportaIndiceInvalido(t *testing.T) {
	got := buildExporta(map[string]any{"export_uf_saida_pais": "PI", "export_loc_despacho_index": 3}, nil)
	if len(got) != 1 || got["UFSaidaPais"] != "PI" {
		t.Fatalf("want só UFSaidaPais: %v", got)
	}
	if buildExporta(map[string]any{}, nil) != nil {
		t.Fatal("operação sem UF de saída não é exportação")
	}
}

func TestBuildDetExportExportIndTudoOuNada(t *testing.T) {
	got := buildDetExport([]map[string]any{
		{"n_draw": "D1", "n_re": "123456789012", "ch_nfe": testDetExportKey, "q_export": "10.0000"},
		{"n_draw": "D2", "n_re": "123456789012"},
	})
	if len(got) != 2 {
		t.Fatalf("want 2 nós: %v", got)
	}
	if got[0]["exportInd"].(map[string]any)["nRE"] != "123456789012" {
		t.Fatalf("exportInd errado: %v", got[0])
	}
	if _, ok := got[1]["exportInd"]; ok {
		t.Fatalf("exportInd incompleto não pode sair: %v", got[1])
	}
	if buildDetExport(nil) != nil {
		t.Fatal("sem exportação não há detExport")
	}
}

const testDetExportKey = "22260811647612000197550010000000011100000015"

func TestBuildIISoComValoresDeclarados(t *testing.T) {
	got := buildII(map[string]any{"ii_v_ii": "50.00", "ii_v_desp_adu": "10.00"},
		decimal.RequireFromString("100.00"))
	if got["vBC"] != "100.00" || got["vII"] != "50.00" || got["vDespAdu"] != "10.00" || got["vIOF"] != "0.00" {
		t.Fatalf("II errado: %v", got)
	}
	if buildII(map[string]any{}, decimal.RequireFromString("100.00")) != nil {
		t.Fatal("item sem II declarado não gera o grupo")
	}
}

// ── Task 39: veicProd e med sem default inventado ────────────────────────────

// veicProdCompleto é o item mínimo que satisfaz as 24 tags obrigatórias do
// grupo. Os testes de campo faltante partem dele e removem uma chave.
func veicProdCompleto() map[string]any {
	return map[string]any{
		"veic_tp_op": "1", "veic_chassi": "9BWZZZ377VT004251", "veic_c_cor": "0013",
		"veic_x_cor": "PRATA", "veic_pot": "85", "veic_cilin": "1600",
		"net_weight": "950.0000", "gross_weight": "1100.0000", "veic_n_serie": "123456",
		"veic_tp_comb": "16", "veic_n_motor": "MTR0001", "veic_cmt": "1.5000",
		"veic_dist": "2600", "veic_ano_mod": "2026", "veic_ano_fab": "2025",
		"veic_tp_pint": "P", "veic_tp_veic": "06", "veic_esp_veic": "1",
		"veic_vin": "N", "veic_cond_veic": "1", "veic_c_mod": "023459",
		"veic_c_cor_denatran": "10", "veic_lota": "5", "veic_tp_rest": "0",
	}
}

func TestBuildVeicProdCompleto(t *testing.T) {
	got, err := buildVeicProd(veicProdCompleto())
	if err != nil {
		t.Fatalf("item completo não pode falhar: %v", err)
	}
	for _, tag := range veicProdTagOrder {
		if v, ok := got[tag]; !ok || v == "" {
			t.Fatalf("tag %s ausente: %v", tag, got)
		}
	}
	if len(got) != len(veicProdTagOrder) {
		t.Fatalf("nó com tag a mais: %v", got)
	}
	if got["cMod"] != "023459" || got["tpVeic"] != "06" || got["pesoL"] != "950.0000" {
		t.Fatalf("valores do cadastro perdidos: %v", got)
	}
}

// Nenhum campo do veículo tem default: cada ausência nomeia a própria tag.
func TestBuildVeicProdCampoFaltanteNomeiaATag(t *testing.T) {
	for itemKey, tag := range veicProdFields {
		// A ausência do chassi não é campo faltante: é o que diz que o item não
		// é veículo novo (TestBuildVeicProdAusenteNaoEhErro).
		if itemKey == "veic_chassi" {
			continue
		}
		item := veicProdCompleto()
		delete(item, itemKey)
		_, err := buildVeicProd(item)
		if err == nil {
			t.Fatalf("veicProd sem %s (%s) tinha que falhar", itemKey, tag)
		}
		if !strings.Contains(err.Error(), tag) {
			t.Fatalf("erro de %s não nomeia a tag %s: %v", itemKey, tag, err)
		}
	}
}

// Item que não é veículo não produz o grupo nem erro.
func TestBuildVeicProdAusenteNaoEhErro(t *testing.T) {
	got, err := buildVeicProd(map[string]any{"product_code": "P1"})
	if got != nil || err != nil {
		t.Fatalf("item sem chassi não é veículo: %v %v", got, err)
	}
}

// A cor informada na emissão vence a do cadastro do modelo.
func TestBuildVeicProdCorDaEmissaoVence(t *testing.T) {
	item := veicProdCompleto()
	item["veic_c_cor_override"] = "0099"
	item["veic_x_cor_override"] = "VERMELHO FANTASIA"
	got, err := buildVeicProd(item)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["cCor"] != "0099" || got["xCor"] != "VERMELHO FANTASIA" {
		t.Fatalf("override de cor ignorado: %v", got)
	}
}

func TestBuildMedRegistrado(t *testing.T) {
	got, err := buildMed(map[string]any{
		"med_c_prod_anvisa": "1234567890123", "med_v_pmc": "49.90",
	})
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["cProdANVISA"] != "1234567890123" || got["vPMC"] != "49.90" {
		t.Fatalf("med errado: %v", got)
	}
	if _, ok := got["xMotivoIsencao"]; ok {
		t.Fatalf("medicamento registrado não leva xMotivoIsencao: %v", got)
	}
}

// vPMC não tem default: 0.00 inventado é rejeição adiada.
func TestBuildMedSemPMCFalha(t *testing.T) {
	_, err := buildMed(map[string]any{"med_c_prod_anvisa": "1234567890123"})
	if err == nil || !strings.Contains(err.Error(), "vPMC") {
		t.Fatalf("want erro nomeando vPMC, got %v", err)
	}
}

// ISENTO exige o motivo da isenção; registrado proíbe.
func TestBuildMedIsentoExigeMotivo(t *testing.T) {
	_, err := buildMed(map[string]any{"med_c_prod_anvisa": "ISENTO", "med_v_pmc": "10.00"})
	if err == nil || !strings.Contains(err.Error(), "xMotivoIsencao") {
		t.Fatalf("want erro nomeando xMotivoIsencao, got %v", err)
	}
	got, err := buildMed(map[string]any{
		"med_c_prod_anvisa": "ISENTO", "med_v_pmc": "10.00",
		"med_x_motivo_isencao": "RDC 123/2025",
	})
	if err != nil || got["xMotivoIsencao"] != "RDC 123/2025" {
		t.Fatalf("isento com motivo errado: %v %v", got, err)
	}
	if _, err := buildMed(map[string]any{
		"med_c_prod_anvisa": "1234567890123", "med_v_pmc": "10.00",
		"med_x_motivo_isencao": "RDC 123/2025",
	}); err == nil {
		t.Fatal("medicamento registrado com motivo de isenção tinha que falhar")
	}
}

func TestBuildMedAusenteNaoEhErro(t *testing.T) {
	got, err := buildMed(map[string]any{"product_code": "P1"})
	if got != nil || err != nil {
		t.Fatalf("item sem registro ANVISA não é medicamento: %v %v", got, err)
	}
}

// ── Task 41: xPed, nItemPed e nRECOPI no prod ────────────────────────────────

func TestBuildProdPedidoDoClienteENRecopi(t *testing.T) {
	item := map[string]any{
		"product_code": "P1", "x_ped": "PC-2026-0099", "n_item_ped": "7",
		"n_recopi": "12345678901234567890",
	}
	prod := buildProd(item, prodParams{
		Description: "Papel", Unit: "UN", TaxableUnit: "UN",
		QTrib: "1.0000", VUnTrib: "10.00", VProd: "10.00",
	})
	if prod["xPed"] != "PC-2026-0099" || prod["nItemPed"] != "7" {
		t.Fatalf("pedido do cliente ausente: %v", prod)
	}
	if prod["nRECOPI"] != "12345678901234567890" {
		t.Fatalf("nRECOPI ausente: %v", prod)
	}
}

// nRECOPI é choice com comb/med/veicProd/arma no XSD: com combustível no item,
// o RECOPI não sai — emitir os dois é rejeição.
func TestBuildProdNRecopiNaoConviveComComb(t *testing.T) {
	item := map[string]any{
		"product_code": "P1", "n_recopi": "12345678901234567890",
		"comb_c_prod_anp": "320102001", "quantity": "1",
	}
	prod := buildProd(item, prodParams{Description: "X", Unit: "UN", TaxableUnit: "UN"})
	if _, ok := prod["nRECOPI"]; ok {
		t.Fatalf("nRECOPI e comb são alternativos no XSD: %v", prod)
	}
	if _, ok := prod["comb"]; !ok {
		t.Fatalf("comb tinha que continuar: %v", prod)
	}
}

// ── Task 43: gCred, tpCredPresIBSZFM e indBemMovelUsado ─────────────────────

// vCredPresumido é derivado: é o percentual sobre o valor do item, nunca
// digitado — dois campos que têm que fechar entre si são um campo.
func TestBuildGCredDerivaValor(t *testing.T) {
	creds := []any{
		map[string]any{"c_cred_presumido": "PR000001", "p_cred_presumido": "10.0000"},
		map[string]any{"c_cred_presumido": "PR00000002", "p_cred_presumido": "2.5000"},
	}
	got := buildGCred(creds, decimal.RequireFromString("1000.00"))
	if len(got) != 2 {
		t.Fatalf("want 2 créditos, got %v", got)
	}
	if got[0]["cCredPresumido"] != "PR000001" || got[0]["vCredPresumido"] != "100.00" {
		t.Fatalf("primeiro crédito errado: %v", got[0])
	}
	if got[1]["vCredPresumido"] != "25.00" {
		t.Fatalf("segundo crédito errado: %v", got[1])
	}
}

// O XSD limita a 4 ocorrências; o excedente é cortado com o resto ignorado, não
// emitido — mas o cadastro é que deveria impedir, então aqui só o corte.
func TestBuildGCredRespeitaOLimiteDoLeiaute(t *testing.T) {
	creds := make([]any, 0, 6)
	for i := 0; i < 6; i++ {
		creds = append(creds, map[string]any{"c_cred_presumido": "PR00000X", "p_cred_presumido": "1.0000"})
	}
	if got := buildGCred(creds, decimal.NewFromInt(100)); len(got) != maxGCred {
		t.Fatalf("want %d, got %d", maxGCred, len(got))
	}
}

func TestBuildGCredAusente(t *testing.T) {
	if buildGCred(nil, decimal.NewFromInt(100)) != nil {
		t.Fatal("item sem crédito presumido não leva gCred")
	}
	// Entrada sem código nem percentual não vira nó de valor zero.
	if got := buildGCred([]any{map[string]any{}}, decimal.NewFromInt(100)); got != nil {
		t.Fatalf("crédito em branco não vira nó: %v", got)
	}
}

func TestBuildProdCamposDaReforma(t *testing.T) {
	item := map[string]any{
		"product_code": "P1", "quantity": "2",
		"gcred":                []any{map[string]any{"c_cred_presumido": "PR000001", "p_cred_presumido": "10.0000"}},
		"tp_cred_pres_ibs_zfm": "1",
		"ind_bem_movel_usado":  "1",
	}
	prod := buildProd(item, prodParams{
		Description: "Bem usado", Unit: "UN", TaxableUnit: "UN",
		QTrib: "2.0000", VUnTrib: "500.00", VProd: "1000.00",
	})
	if len(prod["gCred"].([]map[string]any)) != 1 {
		t.Fatalf("gCred ausente: %v", prod)
	}
	if prod["tpCredPresIBSZFM"] != "1" || prod["indBemMovelUsado"] != "1" {
		t.Fatalf("campos da reforma ausentes: %v", prod)
	}
}

// indBemMovelUsado só tem o valor 1 no XSD: qualquer outra coisa é omitida em
// vez de virar rejeição.
func TestBuildProdIndBemMovelUsadoSoAceita1(t *testing.T) {
	for _, v := range []string{"0", "2", "S", ""} {
		item := map[string]any{"product_code": "P1", "ind_bem_movel_usado": v}
		prod := buildProd(item, prodParams{Description: "X", Unit: "UN", TaxableUnit: "UN"})
		if _, ok := prod["indBemMovelUsado"]; ok {
			t.Fatalf("indBemMovelUsado=%q não pode ser emitido: %v", v, prod)
		}
	}
}

package nfes

import (
	"strings"
	"testing"

	"github.com/shopspring/decimal"
)

func TestBuildInfAdicTodosOsDestinos(t *testing.T) {
	got := buildInfAdic(
		"Beneficio fiscal 123", "Obrigado pela preferencia",
		[]map[string]any{{"@xCampo": "Pedido", "xTexto": "42"}},
		[]map[string]any{{"@xCampo": "Regime", "xTexto": "Especial"}},
		[]map[string]any{{"nProc": "0001/2026", "indProc": "0"}},
	)
	if got["infAdFisco"] != "Beneficio fiscal 123" || got["infCpl"] != "Obrigado pela preferencia" {
		t.Fatalf("textos errados: %v", got)
	}
	if len(got["obsCont"].([]map[string]any)) != 1 || len(got["procRef"].([]map[string]any)) != 1 {
		t.Fatalf("listas ausentes: %v", got)
	}
}

func TestBuildInfAdicVazioDevolveNil(t *testing.T) {
	if buildInfAdic("", "", nil, nil, nil) != nil {
		t.Fatal("infAdic vazio tem que ser omitido, não presente e vazio")
	}
}

// ── Task 40: compra, cana e agropecuario ─────────────────────────────────────

func strPtr(v string) *string { return &v }

func TestBuildCompraJuntaEmpenhoDaOperacaoEPedidoDaNota(t *testing.T) {
	op := map[string]any{"compra_x_n_emp": "2026NE000123"}
	got := buildCompra(op, "PED-4455", "CT-2026/09")
	if got["xNEmp"] != "2026NE000123" || got["xPed"] != "PED-4455" || got["xCont"] != "CT-2026/09" {
		t.Fatalf("compra errada: %v", got)
	}
}

// Sem nenhum dos três, o grupo não existe — nó vazio é rejeição.
func TestBuildCompraVazioDevolveNil(t *testing.T) {
	if buildCompra(map[string]any{}, "", "") != nil {
		t.Fatal("compra sem empenho, pedido nem contrato tem que ser omitida")
	}
	got := buildCompra(map[string]any{}, "PED-1", "")
	if len(got) != 1 || got["xPed"] != "PED-1" {
		t.Fatalf("want só xPed: %v", got)
	}
}

func canaDeliveries() []NfeCanaDeliveryBody {
	return []NfeCanaDeliveryBody{
		{Dia: "1", Qtde: "1000.0000"},
		{Dia: "2", Qtde: "500.5000"},
		{Dia: "15", Qtde: "250.0000"},
	}
}

// qTotMes é a soma dos lançamentos diários e qTotGer é ela mais o acumulado
// anterior: nenhum dos dois é digitado.
func TestBuildCanaTotaisSaoDerivados(t *testing.T) {
	op := map[string]any{"cana_safra": "2025/2026"}
	req := &NfeCanaBody{Ref: "09/2026", Deliveries: canaDeliveries(), QTotAnt: strPtr("3000.0000")}
	got, err := buildCana(op, req, decimal.RequireFromString("18750.00"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["safra"] != "2025/2026" || got["ref"] != "09/2026" {
		t.Fatalf("cabeçalho errado: %v", got)
	}
	if got["qTotMes"] != "1750.5000" {
		t.Fatalf("qTotMes não é a soma dos dias: %v", got["qTotMes"])
	}
	if got["qTotAnt"] != "3000.0000" || got["qTotGer"] != "4750.5000" {
		t.Fatalf("acumulados errados: %v", got)
	}
	dias := got["forDia"].([]map[string]any)
	if len(dias) != 3 || dias[0]["@dia"] != "1" || dias[0]["qtde"] != "1000.0000" {
		t.Fatalf("forDia errado: %v", dias)
	}
	// Sem dedução: vTotDed é zero e vLiqFor é o fornecimento inteiro.
	if got["vFor"] != "18750.00" || got["vTotDed"] != "0.00" || got["vLiqFor"] != "18750.00" {
		t.Fatalf("valores errados: %v", got)
	}
	if _, ok := got["deduc"]; ok {
		t.Fatalf("deduc vazia não pode virar nó: %v", got)
	}
}

// vTotDed é a soma das deduções e vLiqFor é vFor menos ela.
func TestBuildCanaDeducoesSomamEAbatem(t *testing.T) {
	req := &NfeCanaBody{
		Ref: "09/2026", Deliveries: canaDeliveries(),
		Deducoes: []NfeCanaDeducBody{{XDed: "CONSECANA", VDed: "1000.00"}, {XDed: "FRETE", VDed: "250.50"}},
	}
	got, err := buildCana(map[string]any{"cana_safra": "2025/2026"}, req, decimal.RequireFromString("18750.00"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["vTotDed"] != "1250.50" || got["vLiqFor"] != "17499.50" {
		t.Fatalf("dedução não abateu: %v", got)
	}
	if len(got["deduc"].([]map[string]any)) != 2 {
		t.Fatalf("deduc errada: %v", got["deduc"])
	}
}

// Sem acumulado anterior informado, qTotAnt é zero — e qTotGer é só o mês.
func TestBuildCanaSemAcumuladoAnterior(t *testing.T) {
	req := &NfeCanaBody{Ref: "09/2026", Deliveries: []NfeCanaDeliveryBody{{Dia: "3", Qtde: "10.0000"}}}
	got, err := buildCana(map[string]any{"cana_safra": "2025/2026"}, req, decimal.RequireFromString("100.00"))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if got["qTotAnt"] != "0.0000" || got["qTotGer"] != "10.0000" {
		t.Fatalf("acumulado errado: %v", got)
	}
}

// Safra é da operação: sem ela cadastrada, o grupo é recusado nomeando o campo.
func TestBuildCanaSemSafraFalha(t *testing.T) {
	req := &NfeCanaBody{Ref: "09/2026", Deliveries: canaDeliveries()}
	if _, err := buildCana(map[string]any{}, req, decimal.Zero); err == nil ||
		!strings.Contains(err.Error(), "safra") {
		t.Fatalf("want erro nomeando safra, got %v", err)
	}
}

// Dois lançamentos no mesmo dia violam a chave única @dia do XSD.
func TestBuildCanaDiaRepetidoFalha(t *testing.T) {
	req := &NfeCanaBody{Ref: "09/2026", Deliveries: []NfeCanaDeliveryBody{
		{Dia: "7", Qtde: "1.0000"}, {Dia: "7", Qtde: "2.0000"},
	}}
	if _, err := buildCana(map[string]any{"cana_safra": "2025/2026"}, req, decimal.Zero); err == nil ||
		!strings.Contains(err.Error(), "7") {
		t.Fatalf("want erro nomeando o dia repetido, got %v", err)
	}
}

func TestBuildCanaAusenteDevolveNil(t *testing.T) {
	got, err := buildCana(map[string]any{"cana_safra": "2025/2026"}, nil, decimal.Zero)
	if got != nil || err != nil {
		t.Fatalf("nota que não é de cana não leva o grupo: %v %v", got, err)
	}
}

// O CPF do responsável técnico é do emitente (nível 1); o receituário é da nota.
func TestBuildAgropecuarioDefensivo(t *testing.T) {
	org := map[string]any{"technical_manager_cpf": "11144477735"}
	req := &NfeAgroBody{Receituarios: []string{"REC-1", "REC-2"}}
	got, err := buildAgropecuario(org, req)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	def := got["defensivo"].([]map[string]any)
	if len(def) != 2 || def[0]["nReceituario"] != "REC-1" || def[1]["CPFRespTec"] != "11144477735" {
		t.Fatalf("defensivo errado: %v", def)
	}
	if _, ok := got["guiaTransito"]; ok {
		t.Fatalf("choice violado: %v", got)
	}
}

func TestBuildAgropecuarioGuiaDeTransito(t *testing.T) {
	req := &NfeAgroBody{Guia: &NfeAgroGuiaBody{
		TpGuia: "1", UFGuia: "PI", SerieGuia: strPtr("A"), NGuia: "123456",
	}}
	got, err := buildAgropecuario(map[string]any{}, req)
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	guia := got["guiaTransito"].(map[string]any)
	if guia["tpGuia"] != "1" || guia["UFGuia"] != "PI" || guia["serieGuia"] != "A" || guia["nGuia"] != "123456" {
		t.Fatalf("guia errada: %v", guia)
	}
	if _, ok := got["defensivo"]; ok {
		t.Fatalf("choice violado: %v", got)
	}
}

// Receituário sem CPF do responsável técnico no cadastro do emitente é 400 —
// o XSD exige os dois juntos.
func TestBuildAgropecuarioReceituarioSemCPFFalha(t *testing.T) {
	req := &NfeAgroBody{Receituarios: []string{"REC-1"}}
	if _, err := buildAgropecuario(map[string]any{}, req); err == nil ||
		!strings.Contains(err.Error(), "CPFRespTec") {
		t.Fatalf("want erro nomeando CPFRespTec, got %v", err)
	}
}

// O XSD é um choice: receituário e guia juntos são recusados.
func TestBuildAgropecuarioChoiceExclusivo(t *testing.T) {
	req := &NfeAgroBody{
		Receituarios: []string{"REC-1"},
		Guia:         &NfeAgroGuiaBody{TpGuia: "1", UFGuia: "PI", NGuia: "1"},
	}
	org := map[string]any{"technical_manager_cpf": "11144477735"}
	if _, err := buildAgropecuario(org, req); err == nil {
		t.Fatal("defensivo e guiaTransito juntos violam o choice do XSD")
	}
}

func TestBuildAgropecuarioAusenteDevolveNil(t *testing.T) {
	got, err := buildAgropecuario(map[string]any{"technical_manager_cpf": "11144477735"}, nil)
	if got != nil || err != nil {
		t.Fatalf("nota comum não leva agropecuario: %v %v", got, err)
	}
	got, err = buildAgropecuario(map[string]any{}, &NfeAgroBody{})
	if got != nil || err != nil {
		t.Fatalf("grupo sem conteúdo é omitido, não vazio: %v %v", got, err)
	}
}

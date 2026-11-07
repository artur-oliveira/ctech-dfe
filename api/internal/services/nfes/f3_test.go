package nfes

import (
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"gopkg.aoctech.app/api-commons/problem"
)

// Testes da fase F3 (MOC NF-e/NFC-e): decimal estrito, cadastro incompleto, exterior, isento, locais completos,
// notas de crédito/débito, pagamento x total e tamanhos do leiaute.

type f3Doc struct {
	model, finNFe, tpNF string
	env                 int
	receiver            map[string]any
	items, payments     []map[string]any
	retirada, entrega   *NfeLocalBody
	extra               docExtras
}

func (c f3Doc) build() map[string]any {
	if c.model == "" {
		c.model = nfModel55
	}
	if c.finNFe == "" {
		c.finNFe = "1"
	}
	if c.tpNF == "" {
		c.tpNF = "1"
	}
	if c.env == 0 {
		c.env = 2
	}
	if c.receiver == nil && c.model == nfModel55 {
		c.receiver = goldenReceiver()
	}
	if c.items == nil {
		c.items = goldenItems()
	}
	if c.payments == nil {
		c.payments = []map[string]any{{"payment_type": "01", "value": "105.00"}}
	}
	return BuildEnviNFe(goldenOrg(), c.receiver, "CNPJ_11647612000197", c.items, c.payments,
		1, 1, c.env, "22261111647612000197550010000000011100000015",
		decimal.RequireFromString("100.00"), decimal.Zero,
		nil, time.Date(2026, 11, 6, 10, 0, 0, 0, time.FixedZone("BRT", -3*3600)),
		nil, c.finNFe, "1", "1", c.tpNF, nil, nil, nil, nil,
		TechData{CNPJ: "11647612000197", Name: "Ctech", Email: "t@t.com", Phone: "8630000000", Version: "1.0"},
		c.model, nil, c.retirada, c.entrega, NormalEmission(c.model), c.extra)
}

func f3Inf(body map[string]any) map[string]any {
	return body["enviNFe"].(map[string]any)["NFe"].(map[string]any)["infNFe"].(map[string]any)
}

func f3Problem(t *testing.T, err error) *problem.Problem {
	t.Helper()
	var p *problem.Problem
	if !errors.As(err, &p) {
		t.Fatalf("esperado *problem.Problem, veio %v", err)
	}
	return p
}

// itemsWithCFOP troca o CFOP do item de exemplo (o perfil é por CFOP).
func itemsWithCFOP(cfop string) []map[string]any {
	items := goldenItems()
	items[0]["cfop"] = cfop
	items[0]["cfop_config"].([]any)[0].(map[string]any)["cfop"] = cfop
	return items
}

func exteriorReceiver() map[string]any {
	return map[string]any{
		"sk":   "IDEST_AB12345",
		"name": "Cliente Exterior",
		"person": map[string]any{
			"state_registrations": []any{map[string]any{"uf": "EX", "state_registration": "123456"}},
			"addresses": []any{map[string]any{
				"street": "Calle Florida", "number": "100", "neighborhood": "Centro", "city": "Buenos Aires",
				"city_ibge_code": "9999999", "state_federation": "EX", "postal_code": "",
				"country_code": "0639", "country_name": "Argentina",
			}},
		},
	}
}

// --- 1. decimal malformado ---

func TestDecimalMalformadoVira422ComCampo(t *testing.T) {
	if !d("").IsZero() {
		t.Fatal("vazio deve ser zero")
	}
	run := func(f func()) (err error) {
		defer recoverDecimal(&err)
		f()
		return nil
	}
	err := run(func() { dField(map[string]any{"quantity": "1,5"}, "quantity") })
	p := f3Problem(t, err)
	if p.Status != http.StatusUnprocessableEntity || p.Code != CodeInvalidDecimal {
		t.Fatalf("status/code = %d/%s", p.Status, p.Code)
	}
	if len(p.Errors) != 1 || p.Errors[0].Field != "quantity" || p.Errors[0].Code != CodeInvalidDecimal {
		t.Fatalf("errors = %+v", p.Errors)
	}
	if strings.Contains(p.Detail, "1,5") || strings.Contains(p.Errors[0].Message, "1,5") {
		t.Error("o valor malformado não pode voltar na resposta")
	}
	if err := run(func() { d("abc") }); f3Problem(t, err).Errors[0].Field != "valor" {
		t.Error("d sem campo deve nomear 'valor'")
	}
	// Outro pânico não é engolido.
	defer func() {
		if recover() == nil {
			t.Error("pânico que não é de decimal deve seguir adiante")
		}
	}()
	_ = run(func() { panic("outro") })
}

// --- 2. município ausente ---

func TestRequireAddress(t *testing.T) {
	person := func(addr map[string]any) map[string]any {
		return map[string]any{"addresses": []any{addr}}
	}
	if err := requireAddress(entityEmitente, person(map[string]any{"state_federation": "SP", "city_ibge_code": "3550308"})); err != nil {
		t.Fatalf("endereço completo recusado: %v", err)
	}
	cases := []struct {
		addr  map[string]any
		field string
	}{
		{map[string]any{"state_federation": "SP"}, "person.addresses[0].city_ibge_code"},
		{map[string]any{"city_ibge_code": "3550308"}, "person.addresses[0].state_federation"},
		{map[string]any{"state_federation": "EX", "country_name": "Argentina"}, "person.addresses[0].country_code"},
	}
	for _, c := range cases {
		p := f3Problem(t, requireAddress(entityDestinatario, person(c.addr)))
		if p.Code != CodeRegistrationIncomplete || p.Errors[0].Field != c.field || p.Errors[0].Params["entity"] != entityDestinatario {
			t.Errorf("%v: %+v", c.addr, p)
		}
	}
}

func TestSemFallbackDeMunicipio(t *testing.T) {
	recv := goldenReceiver()
	delete(recv["person"].(map[string]any)["addresses"].([]any)[0].(map[string]any), "city_ibge_code")
	ender := f3Inf(f3Doc{receiver: recv}.build())["dest"].(map[string]any)["enderDest"].(map[string]any)
	if v, ok := ender["cMun"]; ok {
		t.Fatalf("cMun inventado: %q", v)
	}
}

func TestRequireISSQNMunFG(t *testing.T) {
	items := goldenItems()
	cfg := items[0]["cfop_config"].([]any)[0].(map[string]any)
	cfg["issqn_ind_iss"] = "1"
	p := f3Problem(t, requireISSQNMunFG(items))
	if p.Code != CodeRegistrationIncomplete || p.Errors[0].Field != "cfop_config.issqn_c_mun_fg" {
		t.Fatalf("%+v", p)
	}
	cfg["issqn_c_mun_fg"] = "2211001"
	if err := requireISSQNMunFG(items); err != nil {
		t.Fatal(err)
	}
}

// --- 3. destinatário no exterior e isento ---

func TestDestinatarioNoExterior(t *testing.T) {
	body := f3Doc{receiver: exteriorReceiver(), items: itemsWithCFOP("7102")}.build()
	inf := f3Inf(body)
	if got := inf["ide"].(map[string]any)["idDest"]; got != idDestExterior {
		t.Errorf("idDest = %v, want 3", got)
	}
	dest := inf["dest"].(map[string]any)
	if dest["idEstrangeiro"] != "AB12345" || dest["indIEDest"] != indIEDestNaoContrib || dest["IE"] != nil {
		t.Errorf("dest = %+v", dest)
	}
	ender := dest["enderDest"].(map[string]any)
	want := map[string]string{"UF": "EX", "cMun": "9999999", "cPais": "0639", "xPais": "Argentina"}
	for k, v := range want {
		if ender[k] != v {
			t.Errorf("enderDest/%s = %q, want %q", k, ender[k], v)
		}
	}
	if _, ok := ender["CEP"]; ok {
		t.Error("endereço no exterior não leva CEP")
	}
	if err := gateEnviNFe(body, nfModel55); err != nil {
		t.Fatalf("nota de exportação coerente recusada: %+v", f3Problem(t, err).Errors)
	}
}

// CFOP interno para destinatário no exterior: I08-30 (731) recusa antes da SEFAZ.
func TestExteriorComCFOPInternoRecusado(t *testing.T) {
	recv := exteriorReceiver()
	recv["sk"] = "CPF_52998224725" // e sem idEstrangeiro: E03a-10 (720)
	body := f3Doc{receiver: recv, items: itemsWithCFOP("6102")}.build()
	p := f3Problem(t, gateEnviNFe(body, nfModel55))
	rules := map[string]bool{}
	for _, e := range p.Errors {
		rules[e.Field] = true
	}
	for _, id := range []string{"I08-40", "E03a-10"} {
		if !rules[id] {
			t.Errorf("regra %s não recusou; erros: %+v", id, p.Errors)
		}
	}
	if p.Code != CodeFiscalPrevalidation {
		t.Errorf("code = %s", p.Code)
	}
}

func TestDestinatarioIsento(t *testing.T) {
	recv := goldenReceiver()
	recv["person"].(map[string]any)["state_registrations"] = []any{map[string]any{"uf": "SP", "state_registration": "isento"}}
	body := f3Doc{receiver: recv}.build()
	dest := f3Inf(body)["dest"].(map[string]any)
	if dest["indIEDest"] != indIEDestIsento || dest["IE"] != nil {
		t.Fatalf("dest = %+v", dest)
	}
	// SP não admite contribuinte isento em operação tributada: E16a-30 (805).
	p := f3Problem(t, gateEnviNFe(body, nfModel55))
	if p.Errors[0].Field != "E16a-30" {
		t.Errorf("erros = %+v", p.Errors)
	}
}

// --- 4. retirada/entrega completas ---

func TestLocalCompletoEExterior(t *testing.T) {
	br := buildLocal(&NfeLocalBody{XLgr: "Rua A", Nro: "1", XBairro: "Centro", CMun: "3550308", XMun: "Sao Paulo", UF: "SP",
		CEP: new("01000000"), IE: new("111222333444")})
	if br["CEP"] != "01000000" || br["IE"] != "111222333444" || br["cPais"] != cPaisBrasil {
		t.Errorf("local no Brasil = %+v", br)
	}
	ex := buildLocal(&NfeLocalBody{XLgr: "Calle B", Nro: "2", XBairro: "Centro", CMun: "3550308", XMun: "Montevideo", UF: "EX",
		CEP: new("01000000"), CPais: new("8451"), XPais: new("Uruguai")})
	if ex["cMun"] != cMunExterior || ex["cPais"] != "8451" || ex["xPais"] != "Uruguai" || ex["CEP"] != nil {
		t.Errorf("local no exterior = %+v", ex)
	}
	if err := requireLocalCountry(entityEntrega, &NfeLocalBody{UF: "EX"}); f3Problem(t, err).Errors[0].Field != "entrega.c_pais" {
		t.Errorf("local no exterior sem país deveria ser recusado: %v", err)
	}
}

// --- 5. nota de crédito e de débito ---

func TestNotaDeCreditoSoLevaIBSCBS(t *testing.T) {
	body := f3Doc{finNFe: "5", tpNF: "0", items: itemsWithCFOP("2949"),
		extra: docExtras{TpNFCredito: "05"}}.build()
	inf := f3Inf(body)
	imp := inf["det"].([]map[string]any)[0]["imposto"].(map[string]any)
	for _, g := range []string{"ICMS", "PIS", "COFINS", "IPI"} {
		if _, ok := imp[g]; ok {
			t.Errorf("nota de crédito tipo 05 com o grupo %s (B25-80)", g)
		}
	}
	if _, ok := imp["IBSCBS"]; !ok {
		t.Error("nota de crédito sem IBSCBS")
	}
	tot := inf["total"].(map[string]any)["ICMSTot"].(map[string]any)
	if tot["vICMS"] != "0.00" || tot["vIPI"] != "0.00" || tot["vPIS"] != "0.00" {
		t.Errorf("totais dos grupos retirados não zerados: %+v", tot)
	}
	for _, e := range gateErrs(body, nfModel55) {
		if strings.HasPrefix(e.Field, "B25") {
			t.Errorf("nota de crédito coerente recusada: %+v", e)
		}
	}
	// Exceção 1: crédito 04 (redução de valores) mantém os grupos.
	body = f3Doc{finNFe: "5", tpNF: "0", items: itemsWithCFOP("2949"), extra: docExtras{TpNFCredito: "04"}}.build()
	if _, ok := f3Inf(body)["det"].([]map[string]any)[0]["imposto"].(map[string]any)["ICMS"]; !ok {
		t.Error("crédito 04 deveria manter o ICMS")
	}
}

func TestNotaDeDebitoSemTipoRecusada(t *testing.T) {
	errs := gateErrs(f3Doc{finNFe: "6"}.build(), nfModel55)
	if !hasField(errs, "B25.1-20") {
		t.Errorf("finNFe 6 sem tpNFDebito deveria recusar B25.1-20: %+v", errs)
	}
	errs = gateErrs(f3Doc{extra: docExtras{TpNFCredito: "01"}}.build(), nfModel55)
	if !hasField(errs, "B25.2-10") {
		t.Errorf("tpNFCredito em nota normal deveria recusar B25.2-10: %+v", errs)
	}
}

// --- 6. pagamento x total ---

func TestPagamentoMaiorQueTotalSemTroco(t *testing.T) {
	pay := []map[string]any{{"payment_type": "01", "value": "200.00"}}
	body := f3Doc{model: nfModel65, items: itemsWithCFOP("5102"), payments: pay}.build()
	if !hasField(gateErrs(body, nfModel65), "YA03-20") {
		t.Fatal("pago a mais sem troco deveria recusar YA03-20 (866)")
	}
	vNF := setTroco(body)
	pag := f3Inf(body)["pag"].(map[string]any)
	if want := q2(decimal.RequireFromString("200.00").Sub(vNF)); pag["vTroco"] != want {
		t.Fatalf("vTroco = %v, want %s (vNF %s)", pag["vTroco"], want, vNF)
	}
	for _, id := range []string{"YA03-20", "YA09-10"} {
		if hasField(gateErrs(body, nfModel65), id) {
			t.Errorf("com troco correto %s não deveria recusar", id)
		}
	}
}

func TestPagamentoMenorQueTotal(t *testing.T) {
	pay := []map[string]any{{"payment_type": "01", "value": "10.00"}}
	if !hasField(gateErrs(f3Doc{model: nfModel65, items: itemsWithCFOP("5102"), payments: pay}.build(), nfModel65), "YA03-10") {
		t.Error("NFC-e paga a menos deveria recusar YA03-10 (865)")
	}
	// Modelo 55: "implementação futura" no MOC, fica só no shadow.
	if hasField(gateErrs(f3Doc{payments: pay}.build(), nfModel55), "YA03-10") {
		t.Error("NF-e paga a menos não deveria recusar YA03-10")
	}
}

// --- 7. tamanhos ---

func TestTamanhoForaDoLeiauteNomeiaEntidade(t *testing.T) {
	recv := goldenReceiver()
	recv["name"] = strings.Repeat("N", 70)
	items := goldenItems()
	items[0]["description"] = strings.Repeat("D", 130)
	errs := gateErrs(f3Doc{env: 1, receiver: recv, items: items}.build(), nfModel55)
	var name, desc bool
	for _, e := range errs {
		if e.Field == "name" && e.Params["entity"] == entityDestinatario && e.Code == fieldCodeTooLong && e.Params["limit"] == 60 {
			name = true
		}
		if e.Field == "description" && e.Params["entity"] == entityProduto && e.Params["item"] == 1 {
			desc = true
		}
	}
	if !name || !desc {
		t.Fatalf("erros = %+v", errs)
	}
}

func TestLengthTarget(t *testing.T) {
	cases := map[string][3]any{
		"NFe/infNFe/dest/xNome":              {entityDestinatario, "name", 0},
		"NFe/infNFe/dest/enderDest/xLgr":     {entityDestinatario, "person.addresses[0].street", 0},
		"NFe/infNFe/emit/enderEmit/xBairro":  {entityEmitente, "person.addresses[0].neighborhood", 0},
		"NFe/infNFe/det[2]/prod/xProd":       {entityProduto, "description", 3},
		"NFe/infNFe/det/prod/xProd":          {entityProduto, "description", 1},
		"NFe/infNFe/entrega/xLgr":            {"entrega", "entrega.x_lgr", 0},
		"NFe/infNFe/transp/transporta/xNome": {entityTransporta, "name", 0},
		"NFe/infNFe/infAdic/infCpl":          {entityDocumento, "infNFe/infAdic/infCpl", 0},
	}
	for path, want := range cases {
		e, f, i := lengthTarget(path)
		if e != want[0] || f != want[1] || i != want[2] {
			t.Errorf("%s = %s %s %d, want %v", path, e, f, i, want)
		}
	}
}

func gateErrs(body map[string]any, model string) []problem.FieldError {
	err := gateEnviNFe(body, model)
	if err == nil {
		return nil
	}
	var p *problem.Problem
	if errors.As(err, &p) {
		return p.Errors
	}
	return nil
}

func hasField(errs []problem.FieldError, field string) bool {
	for _, e := range errs {
		if e.Field == field {
			return true
		}
	}
	return false
}

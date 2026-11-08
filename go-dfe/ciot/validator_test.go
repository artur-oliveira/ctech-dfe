package ciot

import (
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/internal/taxid"
)

// Synthetic documents with valid check digits (not real people or companies).
const (
	cpfA     = "52998224725"
	cnpjA    = "11222333000181"
	cnpjAlfa = "12ABC34501DE35" // alphanumeric, check digits computed by the helper below
)

func alfaCNPJ(base string) string {
	w1 := []int{5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	w2 := []int{6, 5, 4, 3, 2, 9, 8, 7, 6, 5, 4, 3, 2}
	dv := func(s string, w []int) string {
		sum := 0
		for i, x := range w {
			sum += int(s[i]-'0') * x
		}
		r := sum % 11
		if r < 2 {
			return "0"
		}
		return string(rune('0' + 11 - r))
	}
	d1 := dv(base, w1)
	return base + d1 + dv(base+d1, w2)
}

var testNow = time.Date(2026, 11, 10, 12, 0, 0, 0, Brasilia)

func validDecl() *Declaracao {
	d := &Declaracao{
		IdOperacaoTransporte: "ABCDEF123456", TipoOperacao: TipoLotacao,
		CpfCnpjContratado: cpfA, RNTRCContratado: "12345678", CpfCnpjContratante: cnpjA, CpfCnpjDestinatario: cnpjA,
		ValorFrete: "5000.00", DataDeclaracao: FormatDateTime(testNow),
		DataInicioViagem: "2026-11-11", DataFimViagem: "2026-11-20",
		Veiculos:                   []Veiculo{{Placa: "ABC1D23", NumeroEixos: 3, Automotor: true, CavaloTrator: true}, {Placa: "DEF4567", NumeroEixos: 2}},
		OrigemDestino:              []OrigemDestino{{OrigemIBGE: "3550308", DestinoIBGE: "3304557", DistanciaPercorrida: 430}},
		DadosCarga:                 &DadosCarga{NaturezaCarga: "1001", PesoCarga: "25000.00", TipoCarga: 5},
		InfPagamento:               InfPagamento{TipoPagamento: PagPix, ChavePix: "k", IdentificadorPix: "i", CpfCnpjCreditado: cpfA},
		InfIndicadoresOperacionais: &Indicadores{},
	}
	d.Normalize()
	return d
}

func rulesOf(v []Violation) map[string]bool {
	m := map[string]bool{}
	for _, x := range v {
		m[x.Rule] = true
	}
	return m
}

func TestValidDeclarationHasNoViolations(t *testing.T) {
	if v := Validate(validDecl(), Context{Now: testNow}); len(v) != 0 {
		t.Fatalf("unexpected: %+v", v)
	}
}

func TestAlphanumericCNPJ(t *testing.T) {
	c := alfaCNPJ("12ABC34501DE")
	if !taxid.CNPJ(c) {
		t.Fatalf("%s should be valid", c)
	}
	d := validDecl()
	d.CpfCnpjContratante = c
	if v := Validate(d, Context{Now: testNow}); len(v) != 0 {
		t.Fatalf("%+v", v)
	}
	d.CpfCnpjContratante = c[:13] + "0"
	if !rulesOf(Validate(d, Context{Now: testNow}))["B4"] {
		t.Fatal("bad check digit not caught")
	}
}

func TestEachRuleFires(t *testing.T) {
	yes := true
	cases := []struct {
		rule string
		mut  func(d *Declaracao)
		ctx  func(c *Context)
	}{
		{"B1", func(d *Declaracao) { d.ValorFrete = "" }, nil},
		{"B4", func(d *Declaracao) { d.Veiculos[1].Placa = "XX" }, nil},
		{"B5", func(d *Declaracao) { d.Veiculos[1].Placa = d.Veiculos[0].Placa }, nil},
		{"B6", func(d *Declaracao) { d.DataInicioViagem = "2026-11-01" }, nil},
		{"B11", func(d *Declaracao) { d.DataDeclaracao = FormatDateTime(testNow.Add(-time.Hour)) }, nil},
		{"B12", func(d *Declaracao) { d.DataInicioViagem = "2026-11-09"; d.DataDeclaracao = FormatDateTime(testNow) }, nil},
		{"B13", func(d *Declaracao) { d.DataFimViagem = "2026-11-10" }, nil},
		{"B18", func(d *Declaracao) { d.DadosCarga.PesoCarga = "0.00" }, nil},
		{"B19", func(d *Declaracao) {
			d.IndContingencia, d.JustificativaContingencia = true, "x"
			d.DataDeclaracao = FormatDateTime(testNow.AddDate(0, 0, -31))
		}, nil},
		{"B21", func(d *Declaracao) { d.DataFimViagem = "2027-03-01" }, nil},
		{"B22", func(d *Declaracao) { d.DataInicioViagem = "2026-12-20"; d.DataFimViagem = "2026-12-25" }, nil},
		{"B30", func(d *Declaracao) {
			d.TipoOperacao, d.DataInicioViagem, d.CpfCnpjDestinatario, d.DataFimViagem = TipoAgregado, "", "", "2026-12-20"
		}, nil},
		{"B49", func(d *Declaracao) { d.Veiculos = append(d.Veiculos, make([]Veiculo, 4)...) }, nil},
		{"B56", func(d *Declaracao) { d.TipoOperacao, d.CpfCnpjDestinatario = TipoAgregado, "" }, nil},
		{"B62", func(d *Declaracao) { d.TipoOperacao, d.DataInicioViagem = TipoAgregado, "" }, nil},
		{"B92", func(d *Declaracao) {
			d.InfPagamento = InfPagamento{TipoPagamento: PagContaCorrente, CpfCnpjCreditado: cpfA}
		}, nil},
		{"B96", func(d *Declaracao) { d.DataDeclaracao = FormatDateTime(testNow.AddDate(0, 0, -8)) }, nil},
		{"B99", func(d *Declaracao) { d.InfPagamento.Conta = "1" }, nil},
		{"B100", func(d *Declaracao) { d.InfPagamento = InfPagamento{TipoPagamento: PagOutros, CpfCnpjCreditado: cpfA} }, func(c *Context) { c.ContratadoIsTAC = &yes }},
		{"B101", func(d *Declaracao) { d.Veiculos[0].NumeroEixos = 5 }, nil},
		{"B103", func(d *Declaracao) { d.IndContingencia = true }, nil},
		{"B104", func(d *Declaracao) { d.JustificativaContingencia = "x" }, nil},
		{"B105", func(d *Declaracao) { d.InfPagamento.IndPagamento = 1 }, nil},
		{"B106", func(d *Declaracao) { d.InfPagamento.Parcelas = []Parcela{{NumeroParcela: 1}} }, nil},
		{"B109", func(d *Declaracao) { d.OrigemDestino[0] = OrigemDestino{DistanciaPercorrida: 1} }, nil},
		{"B110", func(d *Declaracao) { d.OrigemDestino[0].OrigemLatitude = "-23.5" }, nil},
		{"B111", func(d *Declaracao) { d.OrigemDestino[0].DestinoIBGE, d.OrigemDestino[0].DestinoCEP = "", "20000000" }, nil},
		{"B117", func(d *Declaracao) { d.Veiculos = d.Veiculos[:1] }, nil},
		{"B119", func(d *Declaracao) {
			d.TipoOperacao, d.DadosCarga.ContratantesCargaFrac = TipoFracionada, []string{d.CpfCnpjContratante}
		}, nil},
		{"B120", func(d *Declaracao) { d.ValorFrete = "0.00" }, nil},
	}
	for _, tc := range cases {
		d := validDecl()
		tc.mut(d)
		c := Context{Now: testNow}
		if tc.ctx != nil {
			tc.ctx(&c)
		}
		if !rulesOf(Validate(d, c))[tc.rule] {
			t.Errorf("%s did not fire: %+v", tc.rule, Validate(d, c))
		}
	}
}

func TestViolationCarriesStableInternalCode(t *testing.T) {
	d := validDecl()
	d.ValorFrete = "0.00"
	v := Validate(d, Context{Now: testNow})
	if len(v) != 1 || v[0].Internal != "CIOT_FLOOR_319" || v[0].ANTTCode != 319 {
		t.Fatalf("%+v", v)
	}
	d = validDecl()
	d.Veiculos[1].Placa = d.Veiculos[0].Placa
	if v = Validate(d, Context{Now: testNow}); v[0].Internal != "CIOT_LOCAL_B5" {
		t.Fatalf("%+v", v)
	}
}

func TestLifecycle(t *testing.T) {
	start := testNow
	cases := []struct {
		rule string
		c    LifecycleContext
	}{
		{"B35", LifecycleContext{Op: OpCancel, Start: start, Now: start.Add(25 * time.Hour)}},
		{"B36", LifecycleContext{Op: OpCancel, Start: start, Now: start, Inspected: true}},
		{"B40", LifecycleContext{Op: OpRectify, Tipo: TipoFracionada, Now: start, Inspected: true}},
		{"B121", LifecycleContext{Op: OpRectify, Tipo: TipoLotacao, Now: start}},
		{"B39", LifecycleContext{Op: OpRectify, Tipo: TipoAgregado, End: start, Now: start.AddDate(0, 0, 31)}},
		{"B116", LifecycleContext{Op: OpClose, Tipo: TipoAgregado, Start: start, Now: start.AddDate(0, 0, 11), Viagens: 1}},
		{"B46", LifecycleContext{Op: OpClose, Tipo: TipoFracionada, Start: start, Now: start, CloseDate: start.AddDate(0, 0, -1)}},
		{"B65", LifecycleContext{Op: OpClose, Tipo: TipoLotacao, Start: start, Now: start}},
		{"B43", LifecycleContext{Op: OpClose, Tipo: TipoAgregado, Start: start, Now: start}},
	}
	for _, tc := range cases {
		if !rulesOf(ValidateLifecycle(tc.c))[tc.rule] {
			t.Errorf("%s did not fire", tc.rule)
		}
	}
	if v := ValidateLifecycle(LifecycleContext{Op: OpCancel, Start: start, Now: start.Add(time.Hour)}); len(v) != 0 {
		t.Fatalf("%+v", v)
	}
}

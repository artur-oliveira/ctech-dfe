package ciot

import (
	"strconv"
	"strings"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/internal/taxid"
)

// Limits and windows of the declaration rules (spec section 5 and 6, [PDF]).
const (
	maxVehicles                = 5
	maxTripDays                = 90
	maxTripDaysAgregado        = 30
	maxStartAheadDays          = 30
	declPastTolerance          = 15 * time.Minute
	declFutureTolerance        = 5 * time.Minute
	declMaxAge                 = 168 * time.Hour
	contingencyMaxAge          = 30 * 24 * time.Hour
	maxWeight                  = 9999999.99
	minAutomotorAxles          = 2
	maxAxles                   = 4
	minImplementoAxles         = 1
	minCargoType, maxCargoType = 1, 12
	ibgeLen, cepLen            = 7, 8
	rntrcDigits                = 9
	natureLen                  = 4
	maxJustification           = 255
)

// Context carries what the pure rules need besides the declaration: the clock and facts from the pre-flight
// queries (nil means unknown, and the rule that needs it is skipped).
type Context struct {
	Now             time.Time
	ContratadoIsTAC *bool // B100, from ConsultarSituacaoTransportador
}

// Violation is a failed rule: the PDF rule id, the ANTT code it maps to, the field and the stable internal code.
type Violation struct {
	Rule     string `json:"rule"`
	ANTTCode int    `json:"antt_code,omitempty"`
	Field    string `json:"field"`
	Internal string `json:"internal_code"`
	Message  string `json:"message"`
}

// Rule is one catalog entry: id, ANTT code (0 when the spec gives none), field, the condition in words, and the
// predicate that returns true when the declaration VIOLATES the rule.
type Rule struct {
	ID        string
	ANTT      int
	Field     string
	Condition string
	Violated  func(d *Declaracao, c Context) bool
}

// Internal returns the stable internal code: the rejection-table code when the ANTT code is known, else a local one.
func (r Rule) Internal() string {
	if r.ANTT != 0 {
		return LookupRejection(r.ANTT).Internal
	}
	return internalPrefix + "LOCAL_" + r.ID
}

// Rules is the declaration catalog. Rules in the spec's list that the spec does not define (B57, B58, B63 to B67,
// B69) are NOT implemented: the PDF text is not in the spec.
var Rules = buildRules()

// Validate runs the catalog (after Normalize) and returns every violation.
func Validate(d *Declaracao, c Context) []Violation {
	if c.Now.IsZero() {
		c.Now = time.Now()
	}
	return runRules(Rules, d, c)
}

func runRules(rules []Rule, d *Declaracao, c Context) []Violation {
	var out []Violation
	for _, r := range rules {
		if r.Violated(d, c) {
			out = append(out, Violation{r.ID, r.ANTT, r.Field, r.Internal(), r.Condition})
		}
	}
	return out
}

func tipo(d *Declaracao, ts ...int) bool {
	for _, t := range ts {
		if d.TipoOperacao == t {
			return true
		}
	}
	return false
}

func parseOK(f func(string) (time.Time, error), s string) (time.Time, bool) {
	t, err := f(s)
	return t, err == nil && s != ""
}

func day(t time.Time) time.Time {
	y, m, dd := t.In(Brasilia).Date()
	return time.Date(y, m, dd, 0, 0, 0, 0, Brasilia)
}

func positiveMoney(s string) bool { v, err := ParseMoney(s); return err == nil && v > 0 }

func validMoneyText(s string) bool {
	_, err := ParseMoney(s)
	return err == nil && !strings.Contains(s, "-")
}

func bankFieldsPresent(p InfPagamento) bool {
	return p.CodigoInstituicaoFinanceira != "" || p.Agencia != "" || p.Conta != ""
}

// required lists the B1 mandatory fields as data: field, applies, present.
var required = []struct {
	field   string
	applies func(*Declaracao) bool
	present func(*Declaracao) bool
}{
	{"IdOperacaoTransporte", nil, func(d *Declaracao) bool { return d.IdOperacaoTransporte != "" }},
	{"TipoOperacao", nil, func(d *Declaracao) bool { return d.TipoOperacao != 0 }},
	{"CpfCnpjContratado", nil, func(d *Declaracao) bool { return d.CpfCnpjContratado != "" }},
	{"RNTRCContratado", nil, func(d *Declaracao) bool { return d.RNTRCContratado != "" }},
	{"CpfCnpjContratante", nil, func(d *Declaracao) bool { return d.CpfCnpjContratante != "" }},
	{"ValorFrete", nil, func(d *Declaracao) bool { return d.ValorFrete != "" }},
	{"DataDeclaracao", nil, func(d *Declaracao) bool { return d.DataDeclaracao != "" }},
	{"DataFimViagem", nil, func(d *Declaracao) bool { return d.DataFimViagem != "" }},
	{"Veiculos", nil, func(d *Declaracao) bool { return len(d.Veiculos) > 0 }},
	{"InfPagamento.TipoPagamento", nil, func(d *Declaracao) bool { return d.InfPagamento.TipoPagamento != 0 }},
	{"InfPagamento.CpfCnpjCreditado", nil, func(d *Declaracao) bool { return d.InfPagamento.CpfCnpjCreditado != "" }},
	{"CpfCnpjDestinatario", func(d *Declaracao) bool { return tipo(d, TipoLotacao, TipoFracionada) }, func(d *Declaracao) bool { return d.CpfCnpjDestinatario != "" }},
	{"DataInicioViagem", func(d *Declaracao) bool { return tipo(d, TipoLotacao, TipoFracionada) }, func(d *Declaracao) bool { return d.DataInicioViagem != "" }},
	{"OrigemDestino", func(d *Declaracao) bool { return tipo(d, TipoLotacao, TipoFracionada) }, func(d *Declaracao) bool { return len(d.OrigemDestino) > 0 }},
	{"DadosCarga", func(d *Declaracao) bool { return tipo(d, TipoLotacao, TipoFracionada) }, func(d *Declaracao) bool { return d.DadosCarga != nil }},
	{"InfIndicadoresOperacionais", func(d *Declaracao) bool { return tipo(d, TipoLotacao) }, func(d *Declaracao) bool { return d.InfIndicadoresOperacionais != nil }},
}

func buildRules() []Rule {
	var rs []Rule
	for _, f := range required {
		f := f
		rs = append(rs, Rule{"B1", 229, f.field, "mandatory field " + f.field + " missing", func(d *Declaracao, _ Context) bool {
			return (f.applies == nil || f.applies(d)) && !f.present(d)
		}})
	}
	rs = append(rs, catalogFormat()...)
	rs = append(rs, catalogDates()...)
	rs = append(rs, catalogStructure()...)
	rs = append(rs, catalogPayment()...)
	return rs
}

func catalogFormat() []Rule {
	b4 := func(field, cond string, v func(*Declaracao) bool) Rule {
		return Rule{"B4", 205, field, cond, func(d *Declaracao, _ Context) bool { return v(d) }}
	}
	docBad := func(s string) bool { return s != "" && !taxid.Doc(s) }
	return []Rule{
		b4("IdOperacaoTransporte", "must have 12 characters", func(d *Declaracao) bool { return d.IdOperacaoTransporte != "" && len(d.IdOperacaoTransporte) != IDLen }),
		b4("TipoOperacao", "must be 1, 2 or 3", func(d *Declaracao) bool { return d.TipoOperacao != 0 && !tipo(d, 1, 2, 3) }),
		b4("CpfCnpjContratado", "invalid CPF/CNPJ check digits", func(d *Declaracao) bool { return docBad(d.CpfCnpjContratado) }),
		b4("CpfCnpjContratante", "invalid CPF/CNPJ check digits", func(d *Declaracao) bool { return docBad(d.CpfCnpjContratante) }),
		b4("CpfCnpjDestinatario", "invalid CPF/CNPJ check digits", func(d *Declaracao) bool { return docBad(d.CpfCnpjDestinatario) }),
		b4("InfPagamento.CpfCnpjCreditado", "invalid CPF/CNPJ check digits", func(d *Declaracao) bool { return docBad(d.InfPagamento.CpfCnpjCreditado) }),
		b4("RNTRCContratado", "RNTRC must have 9 digits", func(d *Declaracao) bool {
			return d.RNTRCContratado != "" && len(d.RNTRCContratado) != rntrcDigits
		}),
		b4("ValorFrete", "money must be text with at most 2 decimals", func(d *Declaracao) bool { return d.ValorFrete != "" && !validMoneyText(d.ValorFrete) }),
		b4("Veiculos.Placa", "plate must be AAA9999 or AAA9A99", func(d *Declaracao) bool {
			for _, v := range d.Veiculos {
				if !ValidPlate(v.Placa) {
					return true
				}
			}
			return false
		}),
		b4("DadosCarga.TipoCarga", "cargo type must be 1 to 12", func(d *Declaracao) bool {
			return d.DadosCarga != nil && (d.DadosCarga.TipoCarga < minCargoType || d.DadosCarga.TipoCarga > maxCargoType)
		}),
		b4("DadosCarga.NaturezaCarga", "cargo nature must be 4 digits", func(d *Declaracao) bool {
			if d.DadosCarga == nil {
				return false
			}
			n := d.DadosCarga.NaturezaCarga
			_, err := strconv.Atoi(n)
			return len(n) != natureLen || err != nil
		}),
		b4("InfPagamento.TipoPagamento", "must be 1 to 6", func(d *Declaracao) bool {
			return d.InfPagamento.TipoPagamento != 0 && (d.InfPagamento.TipoPagamento < PagCartaoPrePago || d.InfPagamento.TipoPagamento > PagPix)
		}),
		b4("InfPagamento.ChavePix", "Pix payment needs ChavePix and IdentificadorPix", func(d *Declaracao) bool {
			p := d.InfPagamento
			return p.TipoPagamento == PagPix && (p.ChavePix == "" || p.IdentificadorPix == "")
		}),
		{"B120", 319, "ValorFrete", "freight value must be greater than zero", func(d *Declaracao, _ Context) bool {
			return d.ValorFrete != "" && validMoneyText(d.ValorFrete) && !positiveMoney(d.ValorFrete)
		}},
		{"B18", 226, "DadosCarga.PesoCarga", "weight must be greater than zero and below 9999999.99", func(d *Declaracao, _ Context) bool {
			if d.DadosCarga == nil || d.DadosCarga.PesoCarga == "" {
				return false
			}
			cents, err := ParseMoney(d.DadosCarga.PesoCarga)
			return err != nil || cents <= 0 || float64(cents)/100 >= maxWeight
		}},
	}
}

func catalogDates() []Rule {
	date := func(id string, code int, field, cond string, v func(d *Declaracao, c Context, decl, start, end time.Time) bool) Rule {
		return Rule{id, code, field, cond, func(d *Declaracao, c Context) bool {
			decl, ok1 := parseOK(ParseDateTime, d.DataDeclaracao)
			start, _ := parseOK(ParseDate, d.DataInicioViagem)
			end, ok3 := parseOK(ParseDate, d.DataFimViagem)
			if !ok1 || !ok3 {
				return false // missing/malformed is B1/B4
			}
			if start.IsZero() { // type 3 omits it; the ANTT uses the declaration date (B56)
				start = day(decl)
			}
			return v(d, c, decl, start, end)
		}}
	}
	return []Rule{
		date("B11", 269, "DataDeclaracao", "declaration must be from 15 min before to 5 min after Brasilia time", func(d *Declaracao, c Context, decl, _, _ time.Time) bool {
			return !d.IndContingencia && (decl.Before(c.Now.Add(-declPastTolerance)) || decl.After(c.Now.Add(declFutureTolerance)))
		}),
		date("B96", 214, "DataDeclaracao", "declaration must not be older than 168 h", func(d *Declaracao, c Context, decl, _, _ time.Time) bool {
			return !d.IndContingencia && decl.Before(c.Now.Add(-declMaxAge))
		}),
		date("B19", 227, "DataDeclaracao", "contingency declaration at most 30 days in the past", func(d *Declaracao, c Context, decl, _, _ time.Time) bool {
			return d.IndContingencia && decl.Before(c.Now.Add(-contingencyMaxAge))
		}),
		date("B6", 208, "DataInicioViagem", "trip start must not be before today", func(d *Declaracao, c Context, _, start, _ time.Time) bool {
			return tipo(d, TipoLotacao, TipoFracionada) && start.Before(day(c.Now))
		}),
		date("B22", 271, "DataInicioViagem", "trip start at most 30 days ahead", func(d *Declaracao, c Context, _, start, _ time.Time) bool {
			return start.After(day(c.Now).AddDate(0, 0, maxStartAheadDays))
		}),
		date("B12", 225, "DataInicioViagem", "trip start must not be before the declaration date", func(d *Declaracao, _ Context, decl, start, _ time.Time) bool {
			return tipo(d, TipoLotacao, TipoFracionada) && start.Before(day(decl))
		}),
		date("B13", 270, "DataFimViagem", "trip end must not be before the start", func(_ *Declaracao, _ Context, _, start, end time.Time) bool { return end.Before(start) }),
		date("B21", 234, "DataFimViagem", "trip duration at most 90 days", func(d *Declaracao, _ Context, _, start, end time.Time) bool {
			return !tipo(d, TipoAgregado) && end.Sub(start) > maxTripDays*24*time.Hour
		}),
		date("B30", 250, "DataFimViagem", "TAC-Agregado trip duration at most 30 days", func(d *Declaracao, _ Context, _, start, end time.Time) bool {
			return tipo(d, TipoAgregado) && end.Sub(start) > maxTripDaysAgregado*24*time.Hour
		}),
		{"B56", 0, "DataInicioViagem", "TAC-Agregado must omit the trip start", func(d *Declaracao, _ Context) bool {
			return tipo(d, TipoAgregado) && d.DataInicioViagem != ""
		}},
	}
}

func catalogStructure() []Rule {
	loc := func(od OrigemDestino) (ibge, cep, coord [2]bool) {
		return [2]bool{od.OrigemIBGE != "", od.DestinoIBGE != ""}, [2]bool{od.OrigemCEP != "", od.DestinoCEP != ""},
			[2]bool{od.OrigemLatitude != "" || od.OrigemLongitude != "", od.DestinoLatitude != "" || od.DestinoLongitude != ""}
	}
	anyOD := func(d *Declaracao, f func(OrigemDestino) bool) bool {
		for _, od := range d.OrigemDestino {
			if f(od) {
				return true
			}
		}
		return false
	}
	return []Rule{
		{"B49", 0, "Veiculos", "at most 5 vehicles", func(d *Declaracao, _ Context) bool { return len(d.Veiculos) > maxVehicles }},
		{"B5", 0, "Veiculos.Placa", "repeated plate", func(d *Declaracao, _ Context) bool {
			seen := map[string]bool{}
			for _, v := range d.Veiculos {
				if seen[v.Placa] {
					return true
				}
				seen[v.Placa] = true
			}
			return false
		}},
		{"B101", 0, "Veiculos.NumeroEixos", "axles: automotor 2 to 4, implemento 1 to 4", func(d *Declaracao, _ Context) bool {
			for _, v := range d.Veiculos {
				min := minImplementoAxles
				if v.Automotor {
					min = minAutomotorAxles
				}
				if v.NumeroEixos < min || v.NumeroEixos > maxAxles {
					return true
				}
			}
			return false
		}},
		{"B117", 316, "Veiculos", "a tractor unit requires an implement", func(d *Declaracao, _ Context) bool {
			tractor, impl := false, false
			for _, v := range d.Veiculos {
				tractor = tractor || v.CavaloTrator
				impl = impl || !v.Automotor
			}
			return tractor && !impl
		}},
		{"B62", 0, "CpfCnpjDestinatario", "TAC-Agregado must not carry a consignee", func(d *Declaracao, _ Context) bool {
			return tipo(d, TipoAgregado) && d.CpfCnpjDestinatario != ""
		}},
		{"B109", 308, "OrigemDestino", "origin/destination needs at least one valid location form", func(d *Declaracao, _ Context) bool {
			return anyOD(d, func(od OrigemDestino) bool {
				i, c, g := loc(od)
				return (!i[0] || !i[1]) && (!c[0] || !c[1]) && (!g[0] || !g[1])
			})
		}},
		{"B110", 309, "OrigemDestino", "coordinates must come in latitude/longitude pairs", func(d *Declaracao, _ Context) bool {
			return anyOD(d, func(od OrigemDestino) bool {
				return (od.OrigemLatitude == "") != (od.OrigemLongitude == "") || (od.DestinoLatitude == "") != (od.DestinoLongitude == "")
			})
		}},
		{"B111", 310, "OrigemDestino", "origin and destination must be of the same kind", func(d *Declaracao, _ Context) bool {
			return anyOD(d, func(od OrigemDestino) bool {
				i, c, g := loc(od)
				return i[0] != i[1] || c[0] != c[1] || g[0] != g[1]
			})
		}},
		{"B119", 318, "DadosCarga.ContratantesCargaFrac", "fractional shippers only in type 2 and never the main contractor", func(d *Declaracao, _ Context) bool {
			if d.DadosCarga == nil || len(d.DadosCarga.ContratantesCargaFrac) == 0 {
				return false
			}
			if !tipo(d, TipoFracionada) {
				return true
			}
			for _, c := range d.DadosCarga.ContratantesCargaFrac {
				if c == d.CpfCnpjContratante {
					return true
				}
			}
			return false
		}},
	}
}

func catalogPayment() []Rule {
	bank := func(p InfPagamento) bool {
		return p.TipoPagamento >= PagCartaoPrePago && p.TipoPagamento <= PagContaPagamento
	}
	return []Rule{
		{"B92", 233, "InfPagamento", "bank payment (types 1 to 4) needs institution, account and, for 2 to 4, agency", func(d *Declaracao, _ Context) bool {
			p := d.InfPagamento
			if !bank(p) {
				return false
			}
			return p.CodigoInstituicaoFinanceira == "" || p.Conta == "" || (p.TipoPagamento != PagCartaoPrePago && p.Agencia == "")
		}},
		{"B99", 224, "InfPagamento", "bank fields are forbidden for payment types 5 and 6", func(d *Declaracao, _ Context) bool {
			p := d.InfPagamento
			return (p.TipoPagamento == PagOutros || p.TipoPagamento == PagPix) && bankFieldsPresent(p)
		}},
		{"B100", 204, "InfPagamento.TipoPagamento", "TAC or equivalent must not use payment type 5", func(d *Declaracao, c Context) bool {
			return c.ContratadoIsTAC != nil && *c.ContratadoIsTAC && d.InfPagamento.TipoPagamento == PagOutros
		}},
		{"B103", 252, "JustificativaContingencia", "contingency requires a justification of at most 255 characters", func(d *Declaracao, _ Context) bool {
			return d.IndContingencia && (d.JustificativaContingencia == "" || len(d.JustificativaContingencia) > maxJustification)
		}},
		{"B104", 252, "JustificativaContingencia", "justification is forbidden without contingency", func(d *Declaracao, _ Context) bool {
			return !d.IndContingencia && d.JustificativaContingencia != ""
		}},
		{"B105", 215, "InfPagamento.Parcelas", "installment payment needs installments", func(d *Declaracao, _ Context) bool {
			return d.InfPagamento.IndPagamento == 1 && len(d.InfPagamento.Parcelas) == 0
		}},
		{"B106", 215, "InfPagamento.Parcelas", "installments are forbidden for payment in cash", func(d *Declaracao, _ Context) bool {
			return d.InfPagamento.IndPagamento == 0 && len(d.InfPagamento.Parcelas) > 0
		}},
	}
}

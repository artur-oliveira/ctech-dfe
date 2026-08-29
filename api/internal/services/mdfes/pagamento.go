package mdfes

// pagamento.go resolve o pagamento ao transportador autônomo na emissão
// (infANTT/infPag). O nó em si é montado por buildInfPag (payment_events.go),
// que continua sendo o único construtor do grupo: aqui só se traduz o cadastro
// e o prazo da viagem para a estrutura que ela já recebe.

import (
	"context"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// dueDateLayout é o formato de dVenc (infPrazo).
const dueDateLayout = "2006-01-02"

// personBankFields são as chaves do grupo `bank` no cadastro da pessoa.
const (
	bankFieldPix      = "pix_key"
	bankFieldCode     = "bank_code"
	bankFieldBranch   = "branch_code"
	bankFieldCNPJIPEF = "cnpj_ipef"
)

// resolveMdfePayments cruza cada pagamento da viagem com o cadastro do
// beneficiário — nome, documento e recebimento (PIX/banco/IPEF) são invariantes
// da pessoa e nunca são redigitados na emissão — e deriva as parcelas do prazo,
// como a condição de pagamento da NF-e faz. Devolve o grupo infPag pronto.
func (s *MdfeService) resolveMdfePayments(
	ctx context.Context, orgPK string, payments []MdfePaymentBody, issued time.Time,
) ([]map[string]any, error) {
	if len(payments) == 0 {
		return nil, nil
	}
	resolved := make([]MdfePayment, 0, len(payments))
	for _, pay := range payments {
		sk, err := services.BuildPersonSK(pay.PersonDoc)
		if err != nil {
			return nil, err
		}
		person, err := s.personRepo.Get(ctx, orgPK, sk)
		if err != nil {
			return nil, err
		}
		if person == nil {
			return nil, problem.NotFound("beneficiário do pagamento não encontrado no cadastro: " + pay.PersonDoc)
		}

		name := strAttr(person, "name")
		out := MdfePayment{
			Name:            &name,
			Components:      pay.Components,
			ContractValue:   pay.ContractValue,
			PaymentType:     pay.PaymentType,
			AdvanceValue:    pay.AdvanceValue,
			AdvanceRequest:  pay.AdvanceRequest,
			AdvanceKind:     pay.AdvanceKind,
			HighPerformance: pay.HighPerformance,
			Bank:            personBank(personMap(person)),
		}
		cpf, cnpj, foreign := personDocChoice(sk)
		out.CPF, out.CNPJ, out.ForeignID = optional(cpf), optional(cnpj), optional(foreign)

		if pay.PaymentType == paymentIndicatorTerm {
			out.Installments = expandMdfeInstallments(pay, issued)
		}
		resolved = append(resolved, out)
	}
	return buildInfPag(resolved)
}

// expandMdfeInstallments deriva infPrazo do prazo escolhido: o que se parcela é
// o saldo, porque o adiantamento já foi pago. É o mesmo algoritmo da cobrança
// da NF-e (services.ExpandInstallments) — um só parcelamento no repositório.
func expandMdfeInstallments(pay MdfePaymentBody, issued time.Time) []MdfePaymentInstallment {
	count := pay.Installments
	if count < 1 {
		// Um pagamento a prazo sem número de parcelas é um pagamento em
		// parcela única, não um pagamento sem infPrazo (que o XSD recusa).
		count = 1
	}
	rest := parseDec(pay.ContractValue)
	if pay.AdvanceValue != nil {
		rest = rest.Sub(parseDec(*pay.AdvanceValue))
	}
	out := make([]MdfePaymentInstallment, 0, count)
	for _, inst := range services.ExpandInstallments(rest, count, pay.IntervalDays, pay.FirstDueDays, issued) {
		out = append(out, MdfePaymentInstallment{
			Number:  inst.Number,
			DueDate: inst.DueDate.Format(dueDateLayout),
			Value:   inst.Value.StringFixed(2),
		})
	}
	return out
}

// personBank lê o grupo `bank` do cadastro da pessoa para o choice infBanc.
func personBank(person map[string]any) MdfePaymentBank {
	bank, ok := person["bank"].(map[string]any)
	if !ok {
		return MdfePaymentBank{}
	}
	str := func(key string) *string {
		v, _ := bank[key].(string)
		return optional(v)
	}
	return MdfePaymentBank{
		PIX:        str(bankFieldPix),
		BankCode:   str(bankFieldCode),
		AgencyCode: str(bankFieldBranch),
		IPEFCNPJ:   str(bankFieldCNPJIPEF),
	}
}

// optional devolve nil para string vazia — os campos de choice do leiaute são
// ponteiros justamente para distinguir "ausente" de "vazio".
func optional(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// valueOr desreferencia um campo opcional do request.
func valueOr(v *string, def string) string {
	if v == nil || *v == "" {
		return def
	}
	return *v
}

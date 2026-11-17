package bpes

import (
	"fmt"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/api-commons/problem"
)

const (
	minJustificationLen = 15
	maxJustificationLen = 256
	legConnection       = "3"
	paymentCardCredit   = "03"
	paymentCardDebit    = "04"
)

// validateEmitBody confere o que os tags de validação do DTO não alcançam: números em texto, datas e as combinações que o
// leiaute e o MOC proíbem. Os erros são 400 em texto claro (o portão fiscal, depois do builder, cuida do schema e das
// regras do MOC).
func validateEmitBody(b BpeEmitBody) error {
	if b.Buyer != nil {
		ids := 0
		for _, v := range []*string{b.Buyer.CNPJ, b.Buyer.CPF, b.Buyer.ForeignID} {
			if v != nil && *v != "" {
				ids++
			}
		}
		if ids != 1 {
			return problem.BadRequest("informe exatamente um documento do comprador: CNPJ, CPF ou identificador estrangeiro")
		}
	}
	boarding, err := dateTime(b.Passage.Boarding)
	if err != nil {
		return problem.BadRequest("data-hora do embarque inválida (use AAAA-MM-DDThh:mm:ss±hh:mm)")
	}
	validity, err := dateTime(b.Passage.Validity)
	if err != nil {
		return problem.BadRequest("data-hora da validade inválida (use AAAA-MM-DDThh:mm:ss±hh:mm)")
	}
	if validity.Before(boarding) {
		return problem.BadRequest("a validade do bilhete não pode ser anterior ao embarque")
	}
	for i, t := range b.Trips {
		if _, err := dateTime(t.Departure); err != nil {
			return problem.BadRequest(fmt.Sprintf("trecho %d: data-hora da viagem inválida", i+1))
		}
		hasConn := t.Connection != nil && *t.Connection != ""
		if hasConn {
			if _, err := dateTime(*t.Connection); err != nil {
				return problem.BadRequest(fmt.Sprintf("trecho %d: data-hora da conexão inválida", i+1))
			}
		}
		if (t.LegType == legConnection) != hasConn {
			return problem.BadRequest(fmt.Sprintf("trecho %d: a data-hora da conexão vale só no trecho de conexão e é obrigatória nele", i+1))
		}
	}
	total := decimal.Zero
	for i, c := range b.Components {
		v, err := nonNegative(c.Value)
		if err != nil {
			return problem.BadRequest(fmt.Sprintf("componente %d: valor inválido", i+1))
		}
		total = total.Add(v)
	}
	discount := decimal.Zero
	if b.Discount != nil && *b.Discount != "" {
		if discount, err = nonNegative(*b.Discount); err != nil {
			return problem.BadRequest("desconto inválido")
		}
	}
	hasType := b.DiscountType != nil && *b.DiscountType != ""
	if discount.Sign() > 0 && !hasType {
		return problem.BadRequest("informe o tipo do desconto")
	}
	if hasType {
		if discount.Sign() <= 0 {
			return problem.BadRequest("com tipo de desconto o valor do desconto deve ser maior que zero")
		}
		if b.DiscountDesc == nil || *b.DiscountDesc == "" {
			return problem.BadRequest("informe a descrição do desconto")
		}
	}
	if discount.GreaterThan(total) {
		return problem.BadRequest("o desconto não pode superar o valor do bilhete")
	}
	paid := decimal.Zero
	for i, p := range b.Payments {
		v, err := nonNegative(p.Value)
		if err != nil {
			return problem.BadRequest(fmt.Sprintf("pagamento %d: valor inválido", i+1))
		}
		paid = paid.Add(v)
		if (p.Type == paymentCardCredit || p.Type == paymentCardDebit) && p.Card == nil {
			return problem.BadRequest(fmt.Sprintf("pagamento %d: informe os dados do cartão", i+1))
		}
	}
	if paid.LessThan(total.Sub(discount)) {
		return problem.BadRequest("a soma dos pagamentos é menor que o valor a pagar")
	}
	if b.ICMS.CST == "00" || b.ICMS.CST == "90" {
		if b.ICMS.Rate == nil {
			return problem.BadRequest("informe a alíquota do ICMS")
		}
		if _, err := nonNegative(*b.ICMS.Rate); err != nil {
			return problem.BadRequest("alíquota do ICMS inválida")
		}
	}
	return nil
}

func dateTime(s string) (time.Time, error) { return time.Parse(time.RFC3339, s) }

func nonNegative(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil || d.Sign() < 0 {
		return decimal.Zero, fmt.Errorf("invalid")
	}
	return d, nil
}

// resolveContingencyReason devolve a justificativa (xJust) da contingência offline: a do corpo, ou a que o dono
// registrou ao trocar a forma de emissão. De 15 a 256 caracteres (MOC); sem ela a emissão offline é recusada.
func resolveContingencyReason(fromBody, fromConfig *string) (string, error) {
	reason := ""
	switch {
	case fromBody != nil && *fromBody != "":
		reason = *fromBody
	case fromConfig != nil:
		reason = *fromConfig
	}
	if n := len([]rune(reason)); n < minJustificationLen || n > maxJustificationLen {
		return "", problem.BadRequest("a contingência offline exige a justificativa (xJust), de 15 a 256 caracteres")
	}
	return reason, nil
}

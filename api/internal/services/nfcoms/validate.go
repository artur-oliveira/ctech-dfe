package nfcoms

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/api-commons/problem"
)

// validateEmitBody confere o que os tags de validação do DTO não alcançam: escolhas entre campos, números em texto
// e as combinações que o leiaute e o MOC proíbem. Os erros são 400 em texto claro (o portão fiscal, depois do builder,
// cuida do schema e das regras do MOC).
func validateEmitBody(b NfcomEmitBody) error {
	c := b.Customer
	ids := 0
	for _, v := range []*string{c.CNPJ, c.CPF, c.OtherID} {
		if v != nil && *v != "" {
			ids++
		}
	}
	if ids != 1 {
		return problem.BadRequest("informe exatamente um documento do cliente: CNPJ, CPF ou outro identificador")
	}
	hasIE := c.IE != nil && *c.IE != ""
	switch c.IEIndicator {
	case nfcomIEContribuinte:
		if !hasIE {
			return problem.BadRequest("informe a IE do cliente contribuinte")
		}
	case nfcomIEIsento:
		if !hasIE || !strings.EqualFold(*c.IE, "ISENTO") {
			return problem.BadRequest("a IE do cliente isento deve ser ISENTO")
		}
	default:
		if hasIE {
			return problem.BadRequest("cliente não contribuinte não tem IE")
		}
	}
	if b.Invoice == nil && !b.Prepaid {
		return problem.BadRequest("informe a fatura (competência, vencimento e código de barras), salvo em serviço pré-pago")
	}
	if s := b.Subscriber; s.ContractStart != nil && s.ContractEnd != nil && *s.ContractEnd < *s.ContractStart {
		return problem.BadRequest("o fim do contrato não pode ser anterior ao início")
	}
	for i, it := range b.Items {
		if err := validateItem(i+1, it); err != nil {
			return err
		}
	}
	return nil
}

func validateItem(n int, it NfcomItem) error {
	qty, err := positive(it.Quantity)
	if err != nil {
		return problem.BadRequest(fmt.Sprintf("item %d: quantidade inválida", n))
	}
	unit, err := nonNegative(it.UnitValue)
	if err != nil {
		return problem.BadRequest(fmt.Sprintf("item %d: valor unitário inválido", n))
	}
	desc, other := decimal.Zero, decimal.Zero
	if it.Discount != nil {
		if desc, err = nonNegative(*it.Discount); err != nil {
			return problem.BadRequest(fmt.Sprintf("item %d: desconto inválido", n))
		}
	}
	if it.OtherCharges != nil {
		if other, err = nonNegative(*it.OtherCharges); err != nil {
			return problem.BadRequest(fmt.Sprintf("item %d: outras despesas inválidas", n))
		}
	}
	if qty.Mul(unit).Sub(desc).Add(other).Sign() < 0 {
		return problem.BadRequest(fmt.Sprintf("item %d: o desconto não pode superar o valor do item", n))
	}
	semCST := it.ICMS.CST == icmsSemCST
	hasCFOP := it.CFOP != nil && *it.CFOP != ""
	if semCST == hasCFOP {
		if semCST {
			return problem.BadRequest(fmt.Sprintf("item %d: item sem CST não leva CFOP", n))
		}
		return problem.BadRequest(fmt.Sprintf("item %d: informe o CFOP do item", n))
	}
	if it.ICMS.CST == "00" || it.ICMS.CST == "90" {
		if it.ICMS.Rate == nil {
			return problem.BadRequest(fmt.Sprintf("item %d: informe a alíquota do ICMS", n))
		}
		if _, err := nonNegative(*it.ICMS.Rate); err != nil {
			return problem.BadRequest(fmt.Sprintf("item %d: alíquota do ICMS inválida", n))
		}
	}
	for label, rate := range map[string]*string{"PIS": rateOf(it.PIS), "COFINS": rateOfCofins(it.COFINS)} {
		if rate == nil {
			continue
		}
		if _, err := nonNegative(*rate); err != nil {
			return problem.BadRequest(fmt.Sprintf("item %d: alíquota do %s inválida", n, label))
		}
	}
	return nil
}

func rateOf(p *NfcomPIS) *string {
	if p == nil {
		return nil
	}
	return &p.Rate
}

func rateOfCofins(c *NfcomCOFINS) *string {
	if c == nil {
		return nil
	}
	return &c.Rate
}

func nonNegative(s string) (decimal.Decimal, error) {
	d, err := decimal.NewFromString(strings.TrimSpace(s))
	if err != nil || d.Sign() < 0 {
		return decimal.Zero, fmt.Errorf("invalid")
	}
	return d, nil
}

func positive(s string) (decimal.Decimal, error) {
	d, err := nonNegative(s)
	if err != nil || d.Sign() == 0 {
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
	if n := len([]rune(reason)); n < minJustification || n > 256 {
		return "", problem.BadRequest("a contingência offline exige a justificativa (xJust), de 15 a 256 caracteres")
	}
	return reason, nil
}

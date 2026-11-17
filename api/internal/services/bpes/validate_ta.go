package bpes

import "gopkg.aoctech.app/api-commons/problem"

// validateTABody confere o que as tags do TA não alcançam, reaproveitando a validação do bilhete comum (datas, conexão,
// valores, desconto e pagamentos) sobre o corpo reorganizado por BpeTAEmitBody.common. As regras do tipo de compra são as
// F50 a F54 da NT 2025.002: normal e vinculado levam passageiro, o múltiplo leva comprador e não leva passageiro, normal e
// múltiplo levam valores e imposto, e o vinculado leva só a chave do múltiplo.
func validateTABody(b BpeTAEmitBody, year int) error {
	if len(b.Trips) == 0 {
		return problem.BadRequest("informe ao menos um trecho de voo")
	}
	hasPassenger := b.Passage.Passenger != nil
	switch b.PurchaseType {
	case PurchaseMultiple:
		if hasPassenger {
			return problem.BadRequest("o BP-e múltiplo não leva dados do passageiro")
		}
		if b.Buyer == nil {
			return problem.BadRequest("o BP-e múltiplo exige o comprador")
		}
	case PurchaseLinked:
		if !hasPassenger {
			return problem.BadRequest("o BP-e vinculado exige os dados do passageiro")
		}
		if b.MultipleKey == nil || *b.MultipleKey == "" {
			return problem.BadRequest("o BP-e vinculado exige a chave de acesso do BP-e múltiplo")
		}
		if len(b.Components) > 0 || len(b.Payments) > 0 || b.Discount != nil || b.Tax.ClassTrib != "" {
			return problem.BadRequest("o BP-e vinculado não leva valores, imposto nem pagamento")
		}
	default: // normal
		if !hasPassenger {
			return problem.BadRequest("informe os dados do passageiro")
		}
	}
	if b.PurchaseType != PurchaseLinked {
		if b.MultipleKey != nil {
			return problem.BadRequest("multiple_key só vale no BP-e vinculado")
		}
		if len(b.Components) == 0 || len(b.Payments) == 0 {
			return problem.BadRequest("informe os componentes do valor e os pagamentos")
		}
		if b.Tax.ClassTrib == "" {
			return problem.BadRequest("informe a classificação tributária do IBS/CBS")
		}
		if err := validateTATax(b.Tax, year); err != nil {
			return err
		}
	}
	return validateEmitBody(b.common())
}

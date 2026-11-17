package bpes

import "gopkg.aoctech.app/api-commons/problem"

// validateTABody confere o que as tags do TA não alcançam, reaproveitando a validação do bilhete comum (datas, conexão,
// valores, desconto e pagamentos) sobre o corpo reorganizado por BpeTAEmitBody.common.
func validateTABody(b BpeTAEmitBody, year int) error {
	if len(b.Trips) == 0 {
		return problem.BadRequest("informe ao menos um trecho de voo")
	}
	if err := validateTATax(b.Tax, year); err != nil {
		return err
	}
	return validateEmitBody(b.common())
}

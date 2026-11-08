package ciots

import (
	"errors"

	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

// FloorResult is the public freight-floor answer.
type FloorResult struct {
	Cents        int64  `json:"cents"`
	Value        string `json:"value"`
	TableVersion string `json:"table_version"`
}

// FreightFloor computes the minimum freight. While the embedded table is unverified it answers 503 with a clear
// problem and never a number (spec section 7; the table is updated in phase C7).
func FreightFloor(in ciot.FloorInput) (*FloorResult, error) {
	res, err := ciot.DefaultFloorTable().Calculate(in)
	switch {
	case errors.Is(err, ciot.ErrFloorUnverified):
		return nil, problem.ServiceUnavailable("a tabela do piso mínimo de frete ainda não foi verificada contra a publicação oficial da ANTT").WithCode(CodeFloor)
	case errors.Is(err, ciot.ErrFloorNoCoefficient):
		return nil, problem.UnprocessableEntity("não há coeficiente do piso para esta combinação").WithCode(CodeValidation)
	case err != nil:
		return nil, problem.UnprocessableEntity(err.Error()).WithCode(CodeValidation)
	}
	return &FloorResult{Cents: res.Cents, Value: ciot.FormatMoney(res.Cents), TableVersion: res.TableVersion}, nil
}

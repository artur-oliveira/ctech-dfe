package ciots

import (
	"errors"

	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/go-dfe/ciot"
)

// FloorResult is the public freight-floor answer: the minimum, its two legs, the axle column applied and the
// table version and validity it came from.
type FloorResult struct {
	Cents            int64  `json:"cents"`
	Value            string `json:"value"`
	OutboundCents    int64  `json:"outbound_cents"`
	OutboundValue    string `json:"outbound_value"`
	EmptyReturnCents int64  `json:"empty_return_cents"`
	EmptyReturnValue string `json:"empty_return_value"`
	AxlesUsed        int    `json:"axles_used"`
	TableVersion     string `json:"table_version"`
	ValidFrom        string `json:"valid_from"`
}

// FreightFloor computes the minimum freight from the embedded table. An unverified table answers 503 with a clear
// problem and never a number (spec section 7); the declaration only warns about the floor, ANTT decides.
func FreightFloor(in ciot.FloorInput) (*FloorResult, error) {
	return freightFloor(ciot.DefaultFloorTable(), in)
}

func freightFloor(tab *ciot.FloorTable, in ciot.FloorInput) (*FloorResult, error) {
	if in.CargoType < 1 || in.CargoType > 12 || in.Axles < 1 || in.DistanceKm < 1 {
		return nil, problem.UnprocessableEntity("informe cargo_type (1 a 12), axles e distance_km maiores que zero").WithCode(CodeValidation)
	}
	res, err := tab.Calculate(in)
	switch {
	case errors.Is(err, ciot.ErrFloorUnverified):
		return nil, problem.ServiceUnavailable("a tabela do piso mínimo de frete ainda não foi verificada contra a publicação oficial da ANTT").WithCode(CodeFloor)
	case errors.Is(err, ciot.ErrFloorNoCoefficient):
		return nil, problem.UnprocessableEntity("não há coeficiente do piso para esta combinação").WithCode(CodeValidation)
	case err != nil:
		return nil, problem.UnprocessableEntity(err.Error()).WithCode(CodeValidation)
	}
	return &FloorResult{
		Cents: res.Cents, Value: ciot.FormatMoney(res.Cents),
		OutboundCents: res.OutboundCents, OutboundValue: ciot.FormatMoney(res.OutboundCents),
		EmptyReturnCents: res.EmptyReturnCents, EmptyReturnValue: ciot.FormatMoney(res.EmptyReturnCents),
		AxlesUsed: res.AxlesUsed, TableVersion: res.TableVersion, ValidFrom: res.ValidFrom,
	}, nil
}

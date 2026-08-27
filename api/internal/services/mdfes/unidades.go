package mdfes

// unidades.go resolve as unidades de transporte e de carga (infUnidTransp /
// infUnidCarga) do cadastro e as associa aos documentos que elas levam. O
// rateio é calculado em rateio.go — nunca digitado.

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/shopspring/decimal"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Kinds do cadastro: separam infUnidTransp de infUnidCarga.
const (
	cargoUnitKindTransport = "transport"
	cargoUnitKindCargo     = "cargo"
)

// Campos do cadastro organization_cargo_units.
const (
	cargoUnitFieldKind   = "kind"
	cargoUnitFieldTpUnid = "tp_unid"
	cargoUnitFieldIdUnid = "id_unid"
	cargoUnitFieldSeals  = "seals"
)

// cargoUnit é uma unidade do cadastro já decodificada.
type cargoUnit struct {
	Kind   string
	TpUnid string
	IdUnid string
	Seals  []string
}

// resolveTransportUnits monta, por chave de documento, os nós infUnidTransp com
// o rateio calculado a partir dos pesos dos documentos que cada unidade leva.
func (s *MdfeService) resolveTransportUnits(
	ctx context.Context, orgPK string, units []MdfeTransportUnitBody, weights map[string]decimal.Decimal,
) (map[string][]map[string]any, error) {
	if len(units) == 0 {
		return nil, nil
	}
	out := map[string][]map[string]any{}
	for _, u := range units {
		unit, err := s.loadCargoUnit(ctx, orgPK, u.CargoUnitID, cargoUnitKindTransport)
		if err != nil {
			return nil, err
		}
		nested := make([]cargoUnit, 0, len(u.CargoUnitIDs))
		for _, id := range u.CargoUnitIDs {
			c, err := s.loadCargoUnit(ctx, orgPK, id, cargoUnitKindCargo)
			if err != nil {
				return nil, err
			}
			nested = append(nested, *c)
		}
		// Rateio da unidade de transporte: quanto de cada documento vai nela.
		rat := rateCargo(weights, u.DocumentKeys)
		for _, key := range u.DocumentKeys {
			node := map[string]any{
				"tpUnidTransp": unit.TpUnid,
				"idUnidTransp": unit.IdUnid,
			}
			if lac := services.SealNodes(unit.Seals); lac != nil {
				node["lacUnidTransp"] = lac
			}
			if cargas := buildUnidCarga(nested); cargas != nil {
				node["infUnidCarga"] = cargas
			}
			if v, ok := rat[key]; ok {
				node["qtdRat"] = v
			}
			out[key] = append(out[key], node)
		}
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// buildUnidCarga monta as unidades de carga aninhadas. O qtdRat de cada uma é a
// fatia do documento que vai nela: sem informação de peso por contêiner, a
// divisão é igual entre elas, e a última absorve o resíduo (rateCargo).
func buildUnidCarga(units []cargoUnit) []map[string]any {
	if len(units) == 0 {
		return nil
	}
	weights := make(map[string]decimal.Decimal, len(units))
	keys := make([]string, 0, len(units))
	one := decimal.NewFromInt(1)
	for _, u := range units {
		weights[u.IdUnid] = one
		keys = append(keys, u.IdUnid)
	}
	rat := rateCargo(weights, keys)

	out := make([]map[string]any, 0, len(units))
	for _, u := range units {
		node := map[string]any{
			"tpUnidCarga": u.TpUnid,
			"idUnidCarga": u.IdUnid,
			"qtdRat":      rat[u.IdUnid],
		}
		if lac := services.SealNodes(u.Seals); lac != nil {
			node["lacUnidCarga"] = lac
		}
		out = append(out, node)
	}
	return out
}

// loadCargoUnit lê a unidade do cadastro e confere o tipo: usar um contêiner
// onde o leiaute quer uma carreta produziria um XML que só a SEFAZ recusaria.
func (s *MdfeService) loadCargoUnit(ctx context.Context, orgPK, id, wantKind string) (*cargoUnit, error) {
	item, err := s.cargoUnitRepo.Get(ctx, orgPK, id)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("unidade não encontrada no cadastro: " + id)
	}
	var attrs struct {
		Kind   string   `dynamodbav:"kind"`
		TpUnid string   `dynamodbav:"tp_unid"`
		IdUnid string   `dynamodbav:"id_unid"`
		Seals  []string `dynamodbav:"seals"`
	}
	if err := attributevalue.UnmarshalMap(item, &attrs); err != nil {
		return nil, err
	}
	if attrs.Kind != wantKind {
		return nil, problem.BadRequest("unidade " + id + " é do tipo " + attrs.Kind + ", esperado " + wantKind)
	}
	return &cargoUnit{Kind: attrs.Kind, TpUnid: attrs.TpUnid, IdUnid: attrs.IdUnid, Seals: attrs.Seals}, nil
}

package mdfes

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/problem"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// Campos da composição veicular lidos na emissão.
const (
	vsFieldTractorSK  = "tractor_sk"
	vsFieldTrailerSKs = "trailer_sks"
	vsFieldDriverDocs = "driver_docs"
	vsFieldRNTRC      = "rntrc"
	vsFieldCIOT       = "ciot"
)

// loadVehicleSet carrega a composição veicular referenciada na emissão.
func loadVehicleSet(
	ctx context.Context, repo *repositories.VehicleSetRepository, orgPK string, setID *string,
) (map[string]any, error) {
	if setID == nil || *setID == "" {
		return nil, nil
	}
	item, err := repo.Get(ctx, orgPK, *setID)
	if err != nil {
		return nil, err
	}
	if item == nil {
		return nil, problem.NotFound("composição veicular não encontrada: " + *setID)
	}
	var set map[string]any
	if err := attributevalue.UnmarshalMap(item, &set); err != nil {
		return nil, problem.InternalServer("failed to decode vehicle set")
	}
	return set, nil
}

// applyVehicleSet preenche veículo, reboques, condutores, RNTRC e CIOT do
// request a partir da composição.
//
// **Cada campo expandido continua sobrescrevível individualmente no mesmo
// request** — trocar o motorista de um dia não pode exigir criar outro
// conjunto. Por isso a composição só preenche o que veio vazio.
func applyVehicleSet(
	ctx context.Context, personRepo *repositories.PersonRepository, orgPK string,
	req *MdfeEmitBody, set map[string]any,
) error {
	if set == nil {
		return nil
	}

	if req.Vehicle.SK == nil && req.Vehicle.Placa == "" {
		if sk, ok := set[vsFieldTractorSK].(string); ok && sk != "" {
			req.Vehicle.SK = &sk
		}
	}
	if len(req.Trailers) == 0 {
		for _, sk := range stringList(set[vsFieldTrailerSKs]) {
			req.Trailers = append(req.Trailers, MdfeTrailer{SK: sk})
		}
	}
	if len(req.Drivers) == 0 {
		drivers, err := resolveSetDrivers(ctx, personRepo, orgPK, stringList(set[vsFieldDriverDocs]))
		if err != nil {
			return err
		}
		req.Drivers = drivers
	}
	if req.RNTRC == nil {
		if v, ok := set[vsFieldRNTRC].(string); ok && v != "" {
			req.RNTRC = &v
		}
	}
	if req.CIOT == nil {
		if v, ok := set[vsFieldCIOT].(string); ok && v != "" {
			req.CIOT = &v
		}
	}
	return nil
}

// resolveSetDrivers traduz os CPFs da composição em condutores com nome, lendo
// o cadastro de pessoas. Um CPF que não existe mais no cadastro é erro: emitir
// sem o condutor produziria um MDF-e incompleto que só a SEFAZ recusaria.
func resolveSetDrivers(
	ctx context.Context, personRepo *repositories.PersonRepository, orgPK string, docs []string,
) ([]MdfeDriver, error) {
	drivers := make([]MdfeDriver, 0, len(docs))
	for _, doc := range docs {
		sk, err := services.BuildPersonSK(doc)
		if err != nil {
			return nil, err
		}
		person, err := personRepo.Get(ctx, orgPK, sk)
		if err != nil {
			return nil, err
		}
		if person == nil {
			return nil, problem.BadRequest(fmt.Sprintf(
				"condutor %s da composição veicular não está mais no cadastro", doc))
		}
		drivers = append(drivers, MdfeDriver{
			Name: strAttr(person, "name"),
			CPF:  services.StripPKPrefix(sk),
		})
	}
	return drivers, nil
}

// stringList lê uma lista de strings vinda do DynamoDB.
func stringList(v any) []string {
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// ── Proprietário do veículo ──────────────────────────────────────────────────

// Campos de VehicleOwnerBody no item do veículo.
const (
	ownerFieldCpfCnpj = "cpf_cnpj"
	ownerFieldRNTRC   = "rntrc"
	ownerFieldName    = "name"
	ownerFieldType    = "type"

	// ownerTypeTAC é o único tipo do cadastro que corresponde a pessoa física
	// (transportador autônomo de cargas).
	ownerTypeTAC = "TAC"
	// tpPropTACIndependente é o tpProp usado quando o cadastro só diz "TAC" —
	// agregado vs. independente não é distinguível a partir do cadastro atual,
	// e independente é o caso comum. Quem precisa do outro informa MdfeOwner
	// explicitamente no request, que continua vencendo.
	// tpPropOutros (ETC/CTC, pessoa jurídica) vive em builder.go.
	tpPropTACIndependente = "1"
)

// ownerFromRegistry traduz o proprietário cadastrado no veículo para a forma
// que a emissão usa. Devolve nil quando o cadastro está incompleto — um `prop`
// pela metade seria rejeitado pela SEFAZ.
func ownerFromRegistry(owner map[string]types.AttributeValue) *MdfeOwner {
	doc := onlyDigits(avStr(owner, ownerFieldCpfCnpj))
	rntrc := avStr(owner, ownerFieldRNTRC)
	name := avStr(owner, ownerFieldName)
	if doc == "" || rntrc == "" || name == "" {
		return nil
	}

	out := &MdfeOwner{Name: name, RNTRC: rntrc}
	if len(doc) == 11 {
		out.CPF = doc
	} else {
		out.CNPJ = doc
	}
	if avStr(owner, ownerFieldType) == ownerTypeTAC {
		out.TpProp = tpPropTACIndependente
	} else {
		out.TpProp = tpPropOutros
	}
	return out
}

// firstOwner escolhe o proprietário da emissão: o informado no request vence
// sempre; na ausência dele vale o do cadastro do veículo.
//
// Um proprietário cadastrado que **é o próprio emitente** significa frota
// própria, não terceiro: devolver nil aí é o que mantém `ide/tpTransp` idêntico
// ao comportamento de hoje (regras SEFAZ F18/F19/F25). Pelo caminho do request
// esse caso é erro, porque informá-lo explicitamente é contradição; pelo
// cadastro é apenas o estado normal de quem tem frota própria.
// firstOwner takes the issuer's DOCUMENT, not its partition key: the check
// below is a comparison against a CPF/CNPJ, and a company id would never match
// one — which would not fail, it would quietly stop refusing an owner who is
// the issuer.
func firstOwner(fromRequest, fromRegistry *MdfeOwner, emitterDoc string) *MdfeOwner {
	if fromRequest != nil {
		return fromRequest
	}
	if fromRegistry == nil {
		return nil
	}
	if emitterDoc != "" && (fromRegistry.CPF == emitterDoc || fromRegistry.CNPJ == emitterDoc) {
		return nil
	}
	return fromRegistry
}

// avStr lê um atributo string do item.
func avStr(m map[string]types.AttributeValue, key string) string {
	if v, ok := m[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}

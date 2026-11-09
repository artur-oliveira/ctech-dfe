package ctes

// complement.go: CT-e complementar de valores (tpCTe 1). Spec: docs/specs/2026-11-05-cte-emissao-rodoviaria.md
// (seção Complemento e substituto). O XSD 4.00 só pede infCteComp/chCTe (1 a 10): os participantes, o percurso, o
// tipo de serviço e o tomador repetem o original (G168 a G176) e o valor complementar vai em vPrest e imp.

import (
	"context"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/go-dfe/uftz"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// MaxComplementedKeys é o máximo de CT-e complementados por um complementar (XSD: infCteComp maxOccurs 10).
const MaxComplementedKeys = 10

// CteComplementBody é o corpo de POST /ctes/complement.
type CteComplementBody struct {
	// OriginalKeys: CT-e autorizados da organização que este complementar complementa (1 a 10, sem repetição: G177).
	OriginalKeys []string `json:"original_keys" validate:"required,min=1,max=10,unique,dive,dfe_access_key=57"`
	CteChargeBody
}

// Complement valida, monta e grava o CT-e complementar (pending) com o outbox na mesma transação da numeração.
func (s *CteService) Complement(ctx context.Context, orgPK string, req CteComplementBody, userID, userName string) (map[string]types.AttributeValue, error) {
	cfg, environment, err := s.config(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	emit, emitDoc, crt, err := s.issuer(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	pk := services.EnvToPrefix(environment) + "#" + orgPK
	now := uftz.Now(emit.Addr.UF)

	origs := make([]*original, 0, len(req.OriginalKeys))
	for _, k := range req.OriginalKeys {
		if err := checkKeyAge(k, now); err != nil {
			return nil, err
		}
		o, err := s.loadOriginal(ctx, pk, emitDoc, emit.IE, k)
		if err != nil {
			return nil, err
		}
		if len(origs) > 0 {
			if what := sameShape(origs[0].cte, o.cte); what != "" {
				return nil, unprocessable(CodeOriginalsMismatch, "os CT-e "+origs[0].key+" e "+k+" têm "+what+" diferente; um complementar só complementa CT-e com os mesmos participantes e percurso (G168 a G176)")
			}
		}
		origs = append(origs, o)
	}

	p, err := complementParams(origs, emit, crt)
	if err != nil {
		return nil, err
	}
	toma := p.takerActor()
	ch, err := resolveCharge(req.CteChargeBody, crt, docsOf(emit, origs[0].cte, &toma))
	if err != nil {
		return nil, err
	}
	ch.apply(&p, req.CteChargeBody)
	return s.persist(ctx, orgPK, cfg, environment, emitDoc, p, ch.cst, userID, userName)
}

// complementParams monta os parâmetros do complementar a partir dos originais (já conferidos entre si).
func complementParams(origs []*original, emit actor, crt int) (buildParams, error) {
	first := origs[0].cte
	p := buildParams{emit: emit, crt: crt, tpCTe: tpCTeComplement}
	for _, o := range origs {
		p.complementKeys = append(p.complementKeys, o.key)
	}
	if first.Rem == nil || first.Dest == nil {
		return p, unprocessable(CodeOriginalUnsupported, "o CT-e original "+origs[0].key+" não tem remetente e destinatário")
	}
	p.setActors(first)
	role, toma := takerOf(first)
	if toma == nil {
		return p, unprocessable(CodeTakerRoleMissing, "o tomador do CT-e original "+origs[0].key+" não consta no XML")
	}
	p.takerRole = role
	if role == RoleOutro {
		p.toma4 = toma
	}
	return p, nil
}

// complementRecord acrescenta ao registro as marcas do complementar.
func complementRecord(rec map[string]any, p buildParams) {
	rec["cte_type"] = cteTypeComplement
	rec["complemented_keys"] = p.complementKeys
	rec["nfe_keys"] = []string{}
}

const (
	cteTypeComplement = "complement"
	cteTypeSubstitute = "substitute"
)

package ctes

// substitute.go: CT-e substituto (tpCTe 3, infCTeNorm/infCteSub). Spec: docs/specs/2026-11-05-cte-emissao-rodoviaria.md
// (seção Complemento e substituto). O substituto repete o original (participantes, percurso, carga e as mesmas NF-e:
// G086 a G095) e corrige valor, tributos e, opcionalmente, o tomador (indAlteraToma, G097 a G101). O que é provado
// só com o original sai como 422 local; o que depende do estado na SEFAZ fica com ela (ver CONDUCT.md).

import (
	"context"
	"time"

	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/nfexml"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/shopspring/decimal"
)

// CteSubstituteBody é o corpo de POST /ctes/substitute.
type CteSubstituteBody struct {
	// OriginalKey: o CT-e autorizado da organização que este substitui.
	OriginalKey string `json:"original_key" validate:"required,dfe_access_key=57"`
	// ChangeTaker liga o indAlteraToma: o substituto troca o tomador. Sem ele o tomador é o do original.
	ChangeTaker bool `json:"change_taker"`
	// Taker só com change_taker: remetente, destinatario, expedidor ou recebedor (toma3, papel que existe no
	// original e diferente do atual) ou outro com person_id (toma4).
	Taker *CteTaker `json:"taker" validate:"omitempty"`
	CteChargeBody
}

// Substitute valida, monta e grava o CT-e substituto (pending).
func (s *CteService) Substitute(ctx context.Context, orgPK string, req CteSubstituteBody, userID, userName string) (map[string]types.AttributeValue, error) {
	cfg, environment, err := s.config(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if !req.ChangeTaker && req.Taker != nil {
		return nil, unprocessable(CodeTakerNotRequested, "taker só vale com change_taker; sem troca de tomador o substituto repete o do original")
	}
	emit, emitDoc, crt, err := s.issuer(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	pk := services.EnvToPrefix(environment) + "#" + orgPK
	o, err := s.loadOriginal(ctx, pk, emitDoc, emit.IE, req.OriginalKey)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	if err := checkSubstitutionWindow(o, now); err != nil {
		return nil, err
	}

	p, err := substituteParams(o, emit, crt)
	if err != nil {
		return nil, err
	}
	if req.ChangeTaker {
		if err := s.changeTaker(ctx, orgPK, o.cte, req.Taker, &p); err != nil {
			return nil, err
		}
	}

	mdfeCfg, err := s.mdfeConfigRepo.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	if p.rntrc = strAttr(mdfeCfg, rntrcField); p.rntrc == "" {
		return nil, unprocessable(CodeRNTRCRequired, "cadastre o RNTRC da transportadora em Configuração Fiscal, aba MDF-e, antes de emitir CT-e")
	}
	toma := p.takerActor()
	ch, err := resolveCharge(req.CteChargeBody, crt, docsOf(emit, o.cte, &toma))
	if err != nil {
		return nil, err
	}
	ch.apply(&p, req.CteChargeBody)
	return s.persist(ctx, orgPK, cfg, environment, emitDoc, p, ch.cst, userID, userName)
}

// checkSubstitutionWindow: a SEFAZ exige a substituição em até 60 dias da autorização do original (G096, 563).
func checkSubstitutionWindow(o *original, now time.Time) error {
	emi, err := time.Parse(layoutDateTimeTZ, strAttr(o.item, "dh_emi"))
	if err != nil {
		return nil
	}
	if now.Sub(emi) > substitutionWindow+authorizationSlack {
		return unprocessable(CodeSubstitutionWindowEnded, "o CT-e "+o.key+" foi emitido há mais de 60 dias; o prazo de substituição acabou (G096)")
	}
	return nil
}

// substituteParams copia do original o que o substituto repete: participantes, percurso, tipo de serviço, tomador,
// carga e as mesmas NF-e (G094).
func substituteParams(o *original, emit actor, crt int) (buildParams, error) {
	c := o.cte
	p := buildParams{emit: emit, crt: crt, tpCTe: tpCTeSubstitute, sub: &subGroup{chCte: o.key}}
	if c.Rem == nil || c.Dest == nil {
		return p, unprocessable(CodeOriginalUnsupported, "o CT-e original "+o.key+" não tem remetente e destinatário")
	}
	if c.OtherDocs || len(c.NFeKeys) == 0 || c.ProPred == "" {
		return p, unprocessable(CodeOriginalUnsupported, "só o CT-e com NF-e transportadas pode ser substituído por esta API")
	}
	p.setActors(c)
	role, toma := takerOf(c)
	if toma == nil {
		return p, unprocessable(CodeTakerRoleMissing, "o tomador do CT-e original "+o.key+" não consta no XML")
	}
	p.takerRole = role
	if role == RoleOutro {
		p.toma4 = toma
	}
	p.vCarga, p.proPred, p.nfeKeys = nfexml.Dec(c.VCarga), c.ProPred, c.NFeKeys
	for _, q := range c.InfQ {
		switch {
		case q.CUnid == cUnidKG && q.TpMed == tpMedPeso:
			p.weight = nfexml.Dec(q.QCarga)
		case q.CUnid == cUnidUnidade && q.TpMed == tpMedVolumes:
			p.volumes = nfexml.Dec(q.QCarga).IntPart()
		default:
			return p, unprocessable(CodeOriginalUnsupported, "a carga do CT-e original "+o.key+" tem medida fora do escopo (peso bruto e volumes)")
		}
	}
	if !p.weight.GreaterThan(decimal.Zero) {
		return p, unprocessable(CodeOriginalUnsupported, "o CT-e original "+o.key+" não traz o peso bruto da carga")
	}
	return p, nil
}

// changeTaker aplica indAlteraToma: G099 (CNPJ-base entre os papéis do original), G100 (mesma UF do tomador do
// original) para toma4, e G101 (tomador diferente do original) para toma3.
func (s *CteService) changeTaker(ctx context.Context, orgPK string, orig *nfexml.CTe, t *CteTaker, p *buildParams) error {
	if t == nil {
		return unprocessable(CodeTakerRequired, "informe o novo tomador (taker) para a troca de tomador")
	}
	p.sub.alterTakerFlag = true
	oldRole, oldTaker := takerOf(orig)
	if t.Role != RoleOutro {
		if (t.Role == RoleExpedidor && orig.Exped == nil) || (t.Role == RoleRecebedor && orig.Receb == nil) {
			return unprocessable(CodeTakerRoleMissing, "o CT-e original não tem "+t.Role+"; escolha um papel que exista nele")
		}
		if oldRole == t.Role {
			return unprocessable(CodeTakerUnchanged, "o novo tomador é o mesmo do CT-e original; a troca de tomador exige um tomador diferente (G101)")
		}
		p.takerRole, p.toma4 = t.Role, nil
		return nil
	}
	if t.PersonID == nil || *t.PersonID == "" {
		return unprocessable(CodeTakerPersonRequired, "escolha no cadastro a pessoa que passa a pagar o frete")
	}
	a, err := s.personActorByID(ctx, orgPK, *t.PersonID)
	if err != nil {
		return err
	}
	if err := checkNewTaker(orig, a, oldTaker); err != nil {
		return err
	}
	p.takerRole, p.toma4 = RoleOutro, &a
	return nil
}

// checkNewTaker confere o toma4 novo contra o original: CNPJ-base entre os papéis (G099) e UF do tomador (G100).
func checkNewTaker(orig *nfexml.CTe, newTaker actor, oldTaker *actor) error {
	found := false
	for _, r := range []*nfexml.CTeActor{orig.Rem, orig.Dest, orig.Exped, orig.Receb, orig.Toma4} {
		if r != nil && docBase(r.Doc()) == docBase(newTaker.doc()) {
			found = true
		}
	}
	if !found {
		return unprocessable(CodeTakerBaseNotInOriginal, "o CNPJ-base do novo tomador precisa constar nos papéis do CT-e original (G099)")
	}
	if oldTaker != nil && oldTaker.Addr.UF != newTaker.Addr.UF {
		return unprocessable(CodeTakerUFMismatch, "o novo tomador deve ser da mesma UF do tomador do CT-e original (G100)")
	}
	return nil
}

// substituteRecord acrescenta ao registro as marcas do substituto.
func substituteRecord(rec map[string]any, p buildParams) {
	rec["cte_type"] = cteTypeSubstitute
	rec["substituted_key"] = p.sub.chCte
}

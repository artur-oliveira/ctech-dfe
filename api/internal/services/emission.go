package services

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

// Emission justification bounds (ide/xJust, XSD: 15 to 256 characters).
const (
	EmissionReasonMin = 15
	EmissionReasonMax = 256

	// AttrEmissionOptions is the read-only list of valid types in the config GET.
	AttrEmissionOptions = "emission_options"

	// CodeEmissionTypeInvalid: the emission type does not exist for the document.
	CodeEmissionTypeInvalid = "emission_type_invalid"
	// CodeEmissionReasonRequired: leaving normal emission needs the justification.
	CodeEmissionReasonRequired = "emission_reason_required"
	// CodeAutoContingencyTypeRequired: auto_contingency on needs the contingency to switch to.
	CodeAutoContingencyTypeRequired = "auto_contingency_type_required"
	// CodeAutoContingencyTypeInvalid: the chosen automatic contingency is not a contingency of the document.
	CodeAutoContingencyTypeInvalid = "auto_contingency_type_invalid"
)

// emissionOptions lists, per model, every emission type valid for the document
// (docs/specs/2026-11-08-contingencia.md section 2). The first is the default.
//   - NF-e: SVC-AN/SVC-RS by the issuer's UF (tpEmis 6/7) and EPEC (4): sign-only, event 110140 to the Ambiente
//     Nacional, retransmission armed by the worker once the event is registered (nfes/epec_emit.go).
//   - NFC-e: offline (9) in every UF. No EPEC (the MOC does not accept the NFC-e in the EPEC event).
//   - CT-e: offline (2); SVC-RS/SVC-SP by the issuer's UF (7/8). No EPEC (revoked).
//   - MDF-e: offline (2). No SVC and no EPEC.
var emissionOptions = map[string][]string{
	ModelNFe:  {repositories.EmissionNormal, repositories.EmissionSVC, repositories.EmissionEPEC},
	ModelNFCe: {repositories.EmissionNormal, repositories.EmissionOffline},
	ModelCTe:  {repositories.EmissionNormal, repositories.EmissionOffline, repositories.EmissionSVC},
	ModelMDFe: {repositories.EmissionNormal, repositories.EmissionOffline},
}

// EmissionOptions is the emission types valid for a model (nil if none).
func EmissionOptions(model string) []string { return emissionOptions[model] }

// EmissionUpdate is the body of the emission endpoints.
type EmissionUpdate struct {
	Type   string
	Auto   bool
	Reason string
	// AutoType is the contingency the automatic switch uses (emission_options minus normal); required with Auto,
	// ignored (and removed from the row) without it.
	AutoType string
}

// SetEmission validates and stores the emission type of the document and the
// automatic-contingency flag. contingency_since is the moment the row left
// normal emission (kept while the row stays in contingency, so switching
// SVC to EPEC does not move dhCont) and is removed on return to normal.
func (s *fiscalConfigService) SetEmission(ctx context.Context, orgPK string, u EmissionUpdate) (map[string]types.AttributeValue, error) {
	if !slices.Contains(emissionOptions[s.emissionModel], u.Type) {
		return nil, problem.UnprocessableEntity("tipo de emissão inválido para este documento: " + u.Type).WithCode(CodeEmissionTypeInvalid)
	}
	if u.Auto {
		if u.AutoType == "" {
			return nil, problem.UnprocessableEntity("escolha a contingência usada na troca automática").WithCode(CodeAutoContingencyTypeRequired)
		}
		if u.AutoType == repositories.EmissionNormal || !slices.Contains(emissionOptions[s.emissionModel], u.AutoType) {
			return nil, problem.UnprocessableEntity("contingência automática inválida para este documento: " + u.AutoType).WithCode(CodeAutoContingencyTypeInvalid)
		}
	}
	cur, err := s.Get(ctx, orgPK)
	if err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(u.Reason)
	since := time.Now()
	if u.Type != repositories.EmissionNormal {
		if prev := attrStrAV(cur, repositories.AttrContingencySince); prev != "" && attrStrAV(cur, repositories.AttrEmissionType) != repositories.EmissionNormal {
			if t, err := time.Parse(time.RFC3339, prev); err == nil {
				since = t
			}
		}
		if reason == "" {
			reason = attrStrAV(cur, repositories.AttrContingencyReason)
		}
		if n := len([]rune(reason)); n < EmissionReasonMin || n > EmissionReasonMax {
			return nil, problem.UnprocessableEntity("justificativa de 15 a 256 caracteres é obrigatória para sair da emissão normal").WithCode(CodeEmissionReasonRequired)
		}
	}
	if err := s.repo.SetEmission(ctx, orgPK, u.Type, u.Auto, u.AutoType, since, reason); err != nil {
		return nil, err
	}
	slog.InfoContext(ctx, "emission type changed", "org_pk", orgPK, "model", s.emissionModel, "emission_type", u.Type, "auto", u.Auto)
	return s.Get(ctx, orgPK)
}

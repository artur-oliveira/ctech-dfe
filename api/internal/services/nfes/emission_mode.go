package nfes

// emission_mode.go carries the form of emission through the NF-e/NFC-e builder.
//
// Até aqui toda emissão era `tpEmis=1` com `tpImp` fixo por modelo, o que torna
// contingência impossível: se o autorizador da UF cai, o cliente para. O grupo
// `ide/dhCont` + `ide/xJust` é exigido pelo XSD "apenas para tpEmis diferente de
// 1", então ele é pré-requisito de qualquer modo de contingência.
//
// Esta é a fase C0 do plano de contingência: a emissão passa a *aceitar* uma
// forma de emissão. Quem *decide* qual usar (detecção de indisponibilidade,
// máquina de estados por organização) é a fase C2.

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	dfe "gopkg.aoctech.app/dfe/go-dfe"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

const (
	accessKeyLen       = 44
	accessKeyTpEmisPos = 34 // tpEmis é o dígito 35 da chave
)

// EmissionMode is the form of emission of one document.
type EmissionMode struct {
	// TpEmis is ide/tpEmis. It is also embedded in the access key (position 35),
	// so it must be resolved before the key is generated.
	TpEmis string
	// TpImp is ide/tpImp — the DANFE layout, which changes in contingency.
	TpImp string
	// ContingencyAt / Justification fill ide/dhCont + ide/xJust. Required by the
	// XSD whenever TpEmis != "1", ignored otherwise.
	ContingencyAt time.Time
	Justification string
}

// NormalEmission is the online, non-contingency mode for a model — what every
// emission used before contingency existed.
func NormalEmission(model string) EmissionMode {
	tpImp := tpImpDANFERetrato
	if model == nfModel65 {
		tpImp = tpImpDANFENFCe
	}
	return EmissionMode{TpEmis: tpEmisNormal, TpImp: tpImp}
}

// IsContingency reports whether this mode requires the dhCont/xJust group.
func (m EmissionMode) IsContingency() bool {
	return m.TpEmis != "" && m.TpEmis != tpEmisNormal
}

// Validate rejects a mode the SEFAZ layout would reject anyway. A contingency
// mode without its timestamp and justification is the most likely mistake, and
// it is silent until SEFAZ rejects the document.
func (m EmissionMode) Validate() error {
	if m.TpEmis == "" || m.TpImp == "" {
		return problem.InternalServer("forma de emissão não resolvida (tpEmis/tpImp)")
	}
	if !m.IsContingency() {
		return nil
	}
	if m.ContingencyAt.IsZero() {
		return problem.BadRequest("contingência exige a data/hora de entrada em contingência (dhCont)")
	}
	if len([]rune(m.Justification)) < contJustificationMin {
		return problem.BadRequest("contingência exige justificativa com ao menos 15 caracteres (xJust)")
	}
	return nil
}

// ── Contingência SVC (NF-e; docs/specs/2026-11-08-contingencia.md) ───────────

// svcAuthorizerByTpEmis maps the SVC tpEmis of an access key to the egress
// authorizer that issued it. A cancellation must go to the SVC that authorized
// the note (MOC 1.1.3.4), which the tpEmis in the key tells.
var svcAuthorizerByTpEmis = map[string]string{
	TpEmisSVCAN: dfe.ContingencySVCAN,
	TpEmisSVCRS: dfe.ContingencySVCRS,
}

// svcAuthorizerForKey returns the SVC authorizer of a note authorized on the
// SVC, or "" for any other note (including malformed keys).
func svcAuthorizerForKey(accessKey string) string {
	if len(accessKey) != accessKeyLen {
		return ""
	}
	return svcAuthorizerByTpEmis[accessKey[accessKeyTpEmisPos:accessKeyTpEmisPos+1]]
}

// resolveNFeEmission picks the form of emission of an NF-e from the
// organization's contingency window: normal unless the window is open, SVC-AN
// or SVC-RS by the issuer's UF otherwise. The second result is the egress
// authorizer ("" for normal).
func resolveNFeEmission(config map[string]types.AttributeValue, uf string) (EmissionMode, string, error) {
	mode := NormalEmission(nfModel55)
	if strAttr(config, repositories.AttrContingencyMode) != repositories.ContingencyModeSVC {
		return mode, "", nil
	}
	svc, ok := dfe.SVCFor(uf)
	if !ok {
		return mode, "", problem.BadRequest("UF do emitente sem SVC designada: " + uf)
	}
	since, err := time.Parse(time.RFC3339, strAttr(config, repositories.AttrContingencySince))
	if err != nil {
		return mode, "", problem.InternalServer("janela de contingência sem data de início válida")
	}
	mode.TpEmis = TpEmisSVCAN
	if svc == dfe.ContingencySVCRS {
		mode.TpEmis = TpEmisSVCRS
	}
	mode.ContingencyAt = since
	mode.Justification = strAttr(config, repositories.AttrContingencyReason)
	return mode, svc, nil
}

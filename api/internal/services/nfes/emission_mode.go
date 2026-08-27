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

	"gopkg.aoctech.app/dfe/api/internal/problem"
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

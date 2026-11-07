package mdfes

// later_loading.go: carregamento posterior (ide/indCarregaPosterior=1, MOC 2.4, regras F21 a F27). O manifesto
// sai sem documentos e eles entram depois pelo evento de inclusão de DF-e (110115).

import (
	"fmt"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"

	"github.com/shopspring/decimal"
)

// defaultXProd é a descrição do produto predominante quando nem o documento nem o pedido a trazem.
const defaultXProd = "CARGA GERAL"

// resolveLaterLoading decide o carregamento posterior: o pedido manda; sem ele vale a configuração do MDF-e,
// e só para pedido sem documentos (F27: com documentos o manifesto nunca é de carregamento posterior).
func resolveLaterLoading(requested *bool, config bool, docs int) bool {
	if requested != nil {
		return *requested
	}
	return config && docs == 0
}

// laterLoadingCargo monta a carga do carregamento posterior a partir do pedido e recusa, com a regra do MOC, o
// que a SEFAZ rejeitaria: documentos informados (F27), modal não rodoviário (F23), tipo de emitente diferente de
// carga própria (F24), municípios de carregamento e descarregamento que não são um só e o mesmo (F21) e UF de
// início diferente da de fim ou exterior (F22).
func laterLoadingCargo(modal string, req MdfeEmitBody) (*resolvedCargo, error) {
	if len(req.Documents) > 0 || len(req.TransportedMdfes) > 0 {
		return nil, ruleProblem("documents", "F27", 706, "carregamento posterior não admite documentos; inclua-os depois pelo evento de inclusão de DF-e")
	}
	if modal != ModalRodoviario {
		return nil, ruleProblem("modal", "F23", 705, "carregamento posterior só vale para o modal rodoviário")
	}
	if req.EmitterType != nil && *req.EmitterType != "" && *req.EmitterType != tpEmitCargaPropria {
		return nil, ruleProblem("emitter_type", "F24", 707, "carregamento posterior exige tipo de emitente 2 (transportador de carga própria)")
	}
	if len(req.Loadings) != 1 || len(req.Unloadings) != 1 || req.Loadings[0].IBGECode != req.Unloadings[0].IBGECode {
		return nil, ruleProblem("loadings", "F21", 703, "carregamento posterior exige um único município de carregamento igual ao único de descarregamento")
	}
	mun := req.Loadings[0]
	uf := services.UFFromCode[mun.IBGECode[:2]]
	for _, o := range []string{req.UFStart, req.UFEnd} {
		if o != "" && o != uf {
			return nil, ruleProblem("uf_start", "F22", 704, "carregamento posterior exige UF de carregamento igual à de descarregamento e fora do exterior")
		}
	}
	if uf == "" {
		return nil, ruleProblem("loadings", "F22", 704, "município de carregamento sem UF conhecida")
	}
	prod := MdfeProdPred{}
	if req.Predominant != nil {
		prod = *req.Predominant
	}
	return &resolvedCargo{
		carrega:     []MdfeMun{mun},
		descarga:    []descargaGroup{{mun: req.Unloadings[0]}},
		totalValue:  decOrZero(req.CargoValue),
		totalWeight: decOrZero(req.CargoWeight),
		valueByNCM:  map[string]decimal.Decimal{},
		prodPred:    withProdPredDefaults(prod),
		ufIni:       uf,
		ufFim:       uf,
	}, nil
}

// withProdPredDefaults completa o produto predominante com o tipo de carga e a descrição padrão.
func withProdPredDefaults(p MdfeProdPred) MdfeProdPred {
	if p.TpCarga == "" {
		p.TpCarga = defaultTpCarga
	}
	if p.XProd == "" {
		p.XProd = defaultXProd
	}
	return p
}

func decOrZero(s *string) decimal.Decimal {
	if s == nil || *s == "" {
		return decimal.Zero
	}
	return parseDec(*s)
}

// ruleProblem é o 422 de uma regra do MOC conferida antes de montar o documento, no mesmo formato do portão
// (gateMDFe): problem.code fiscal_prevalidation, errors[].code fiscal_rule, params rule e cstat.
func ruleProblem(field, rule string, cstat int, msg string) *problem.Problem {
	return problem.UnprocessableEntityFields(msg,
		problem.Field(field, dfecheck.FieldCodeFiscalRule, fmt.Sprintf("regra %s do MOC (rejeição %d): %s", rule, cstat, msg),
			"rule", rule, "cstat", cstat),
	).WithCode(dfecheck.CodeFiscalPrevalidation)
}

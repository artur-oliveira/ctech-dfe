package mdfes

import "gopkg.aoctech.app/api-commons/problem"

// emitter_type.go: tipo do emitente do MDF-e (ide/tpEmit, TEmit) e as regras F14 a F17 e F113
// do MOC que dependem dele (NT 2026.001 para o CIOT).
const (
	// tpEmitPrestador: prestador de serviço de transporte; carrega CT-e.
	tpEmitPrestador = "1"
	// tpEmitGlobalizado: transportador que emitirá CT-e globalizado; carrega NF-e e só opera
	// dentro de uma UF.
	tpEmitGlobalizado = "3"

	// ufExterior é o código de UF de exterior em UFIni/UFFim.
	ufExterior = "EX"
)

// resolveTpEmit decide o tpEmit. Sem pedido explícito vale o do documento: CT-e é do
// prestador de serviço (1), NF-e é de carga própria (2). Um pedido explícito precisa
// respeitar o MOC: tpEmit 1 só aceita CT-e (F14, 638), 2 só NF-e (F15, 639), 3 só NF-e
// (F16, 540) e só operação intraestadual (F17, 541).
func resolveTpEmit(docType string, requested *string, ufIni, ufFim string) (string, error) {
	tp := tpEmitCargaPropria
	if docType == docTypeCTe {
		tp = tpEmitPrestador
	}
	if requested != nil && *requested != "" {
		tp = *requested
	}
	switch tp {
	case tpEmitPrestador:
		if docType != docTypeCTe {
			return "", problem.BadRequest("tipo de emitente Prestador de serviço de transporte só aceita CT-e, não NF-e (regra F14)")
		}
	case tpEmitCargaPropria:
		if docType != docTypeNFe {
			return "", problem.BadRequest("tipo de emitente Transportador de carga própria só aceita NF-e, não CT-e (regra F15)")
		}
	case tpEmitGlobalizado:
		if docType != docTypeNFe {
			return "", problem.BadRequest("tipo de emitente CT-e globalizado só aceita NF-e, não CT-e (regra F16)")
		}
		if ufIni != ufFim || ufIni == ufExterior || ufFim == ufExterior {
			return "", problem.BadRequest("tipo de emitente CT-e globalizado só vale para operação dentro de uma UF (regra F17)")
		}
	default:
		return "", problem.BadRequest("tipo de emitente inválido: " + tp)
	}
	return tp, nil
}

// requireCIOT aplica a F113 (684) no que dá para saber aqui: modal rodoviário, tpEmit 1 ou 3, RNTRC
// informado e UF de carregamento e descarregamento diferentes de exterior exigem ao menos um CIOT.
// A exigência por categoria do RNTRC (TAC, equiparado) depende do cadastro da ANTT; só a SEFAZ sabe.
func requireCIOT(modal, tpEmit, rntrc, ufIni, ufFim string, hasCIOT bool) error {
	if modal != ModalRodoviario || (tpEmit != tpEmitPrestador && tpEmit != tpEmitGlobalizado) {
		return nil
	}
	if rntrc == "" || ufIni == ufExterior || ufFim == ufExterior || hasCIOT {
		return nil
	}
	return problem.BadRequest("informe o CIOT: o MDF-e rodoviário de prestador de serviço de transporte exige CIOT (regra F113)")
}

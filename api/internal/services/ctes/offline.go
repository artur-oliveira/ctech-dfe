package ctes

// offline.go: forma de emissão do CT-e em contingência (docs/specs/2026-11-08-contingencia.md), MOC CT-e
// Contingencia e Regras de Validação:
//   - off-line (tpEmis 2): dhCont e xJust obrigatórios (G009, 587), dhCont <= dhEmi (G010, 588); o QR Code leva
//     o parâmetro sign (G215, 853); o XML assinado fica guardado e é transmitido em até 168 horas (cStat 150
//     fora do prazo, G103).
//   - SVC-RS (7) e SVC-SP (8): dhCont e xJust NÃO podem ser informados (G008, 586), sign NÃO vai no QR Code
//     (G216, 854), só tpCTe normal (G012, 517). Sem estado pendente: a SVC autoriza.
//   - EPEC foi revogado e não é emitido.

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/api-commons/problem"
	dfe "gopkg.aoctech.app/dfe/go-dfe"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

const (
	tpEmisOffline = "2"
	tpEmisSVCRS   = "7"
	tpEmisSVCSP   = "8"

	accessKeyLen       = 44
	accessKeyTpEmisPos = 34 // tpEmis é o dígito 35 da chave

	// xJust of dhCont/xJust: 15 to 256 characters (Leiaute ide).
	contJustificationMin = 15
	contJustificationMax = 256
)

// svcAuthorizerByTpEmis maps the SVC tpEmis of an access key to the egress authorizer that issued it, so a
// cancellation goes to the same SVC (N09: the SVC accepts only the cancellation event).
var svcAuthorizerByTpEmis = map[string]string{
	tpEmisSVCRS: dfe.ContingencySVCRS,
	tpEmisSVCSP: dfe.ContingencySVCSP,
}

// svcAuthorizerForKey returns the SVC authorizer of a CT-e authorized on the SVC, or "".
func svcAuthorizerForKey(accessKey string) string {
	if len(accessKey) != accessKeyLen {
		return ""
	}
	return svcAuthorizerByTpEmis[accessKey[accessKeyTpEmisPos:accessKeyTpEmisPos+1]]
}

// emission is the form of emission of one CT-e.
type emission struct {
	tpEmis  string
	offline bool
	// svc is the egress authorizer for tpEmis 7/8, "" otherwise.
	svc string
	// since / reason fill dhCont / xJust (off-line only).
	since  time.Time
	reason string
}

// resolveCTeEmission picks the form of emission from the CT-e fiscal configuration and the issuer's UF.
func resolveCTeEmission(cfg map[string]types.AttributeValue, uf string) (emission, error) {
	switch repositories.EmissionTypeOf(cfg) {
	case repositories.EmissionOffline:
		since, err := time.Parse(time.RFC3339, strAttr(cfg, repositories.AttrContingencySince))
		if err != nil {
			return emission{}, problem.InternalServer("emissão offline sem data de início válida (contingency_since)")
		}
		reason := strAttr(cfg, repositories.AttrContingencyReason)
		if n := len([]rune(reason)); n < contJustificationMin || n > contJustificationMax {
			return emission{}, problem.BadRequest("contingência exige justificativa de 15 a 256 caracteres (xJust)")
		}
		return emission{tpEmis: tpEmisOffline, offline: true, since: since, reason: reason}, nil
	case repositories.EmissionSVC:
		svc, ok := dfe.SVCForCTe(uf)
		if !ok {
			return emission{}, problem.BadRequest("UF do emitente sem SVC designada: " + uf)
		}
		tp := tpEmisSVCRS
		if svc == dfe.ContingencySVCSP {
			tp = tpEmisSVCSP
		}
		return emission{tpEmis: tp, svc: svc}, nil
	}
	return emission{tpEmis: services.TpEmisNormal}, nil
}

// markOfflinePending turns the CT-e record into a pending contingency document (168 h window).
func markOfflinePending(item map[string]types.AttributeValue, msg services.WorkerMessage, signedXML string, now time.Time) error {
	return services.MarkContingencyPending(item, msg, signedXML, now, services.OfflineWindowCTe)
}

package mdfes

// offline.go: MDF-e em contingência off-line (tpEmis 2; docs/specs/2026-11-08-contingencia.md).
// O layout do MDF-e não tem dhCont/xJust (MOC MDF-e, Leiaute ide); o que muda é tpEmis (também na
// chave), o parâmetro sign do QR Code e o XML assinado guardado para a retransmissão em até 168 horas.

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// resolveMDFeEmission picks tpEmis from the MDF-e fiscal configuration: off-line (2) when the owner
// switched the document to it, normal otherwise.
func resolveMDFeEmission(config map[string]types.AttributeValue) (tpEmis string, offline bool) {
	if repositories.EmissionTypeOf(config) == repositories.EmissionOffline {
		return TpEmisContingencia, true
	}
	return tpEmisNormal, false
}

// markOfflinePending turns the MDF-e record into a pending contingency document (168 h window).
func markOfflinePending(item map[string]types.AttributeValue, msg services.WorkerMessage, signedXML string, now time.Time) error {
	return services.MarkContingencyPending(item, msg, signedXML, now, services.OfflineWindowMDFe)
}

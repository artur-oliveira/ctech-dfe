package bpes

// offline.go: BP-e em contingência offline (tpEmis 2; MOC do BP-e, Contingência). dhCont e xJust vão no leiaute, a chave
// leva tpEmis 2, o QR Code leva o parâmetro sign (assinatura da chave) e o XML assinado fica guardado para a
// retransmissão com a mesma chave até o fim do primeiro dia útil seguinte.

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// OfflineWindowBPe is the retransmission window the sweep uses: the MOC says "até o final do primeiro dia útil
// subsequente"; 72 hours covers a weekend and the next business day without a holiday calendar (a longer window only
// keeps the sweep trying a little longer, it never transmits late on its own).
const OfflineWindowBPe = 72 * time.Hour

// resolveBPeEmission picks tpEmis from the fiscal configuration: offline (2) when the owner switched the document to
// it, normal otherwise. The reason (xJust) of the switch is kept on the configuration row.
func resolveBPeEmission(config map[string]types.AttributeValue) (tpEmis string, offline bool) {
	if repositories.EmissionTypeOf(config) == repositories.EmissionOffline {
		return tpEmisOffline, true
	}
	return tpEmisNormal, false
}

// markOfflinePending turns the BP-e record into a pending contingency document.
func markOfflinePending(item map[string]types.AttributeValue, msg services.WorkerMessage, signedXML string, now time.Time) error {
	return services.MarkContingencyPending(item, msg, signedXML, now, OfflineWindowBPe)
}

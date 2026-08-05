package nfse

import "context"

// Provider é o contrato que nacional (F2) e abrasf204 (F5) implementam.
type Provider interface {
	Emit(ctx context.Context, doc Document) (Result, error)
	Event(ctx context.Context, ev EventRequest) (Result, error)
	QueryByKey(ctx context.Context, key string) (Result, error)
	QueryByDPSID(ctx context.Context, idDPS string) (Result, error)
	QueryEvents(ctx context.Context, f EventFilter) (Result, error)
}

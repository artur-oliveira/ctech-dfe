package services

import (
	"context"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

// shadowCallGoDfeFromMap adapts invokeSefazLambda's map[string]any
// payload/response shape to godfe.ShadowCompare — see
// docs/plans/2026-07-17-go-dfe-migration.md, "Gate de validação por fase".
// A payload that doesn't match the expected shape is simply skipped;
// this path only ever affects diagnostics, never the caller's response.
//
// Runs synchronously (not a goroutine): api/CLAUDE.md forbids goroutines
// inside request handlers (Fiber handles concurrency), so this adds latency
// to every implemented operation during the shadow window — an accepted,
// temporary tradeoff, same as worker's equivalent
// (worker/internal/service/godfe_shadow.go).
func shadowCallGoDfeFromMap(ctx context.Context, payload map[string]any, statusCode int, body string) {
	req, ok := mapToDfeRequest(payload)
	if !ok {
		return
	}
	godfe.ShadowCompare(ctx, req, statusCode, body)
}

// mapToDfeRequest builds a godfe.Request from the map[string]any payload
// shape invokeSefazLambda's callers build (currently LookupOrganization).
// Returns ok=false if the payload is missing a field this path relies on.
func mapToDfeRequest(payload map[string]any) (godfe.Request, bool) {
	cnpj, _ := payload["cnpj"].(string)
	certB64, _ := payload["certificate_b64"].(string)
	certPassword, _ := payload["certificate_password"].(string)
	uf, _ := payload["uf"].(string)
	environment, _ := payload["environment"].(string)
	docType, _ := payload["doc_type"].(string)
	service, _ := payload["service"].(string)
	body, ok := payload["body"].(map[string]any)
	if !ok || docType == "" || service == "" || uf == "" {
		return godfe.Request{}, false
	}
	return godfe.Request{
		CNPJ: cnpj, CertificateB64: certB64, CertificatePassword: certPassword,
		UF: uf, Environment: environment,
		DocType: docType, Service: service, Body: body,
	}, true
}

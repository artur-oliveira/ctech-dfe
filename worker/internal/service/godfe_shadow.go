package service

import (
	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

// godfeImplements/godfeCall wrap godfe.Implements/godfe.Call as package vars
// (not called directly) so tests can stub them out — e.g. distribution_test.go
// forces godfeImplements to always return false, restoring the pre-cutover
// fake-Lambda-response test setup for distribution logic tests that have
// nothing to do with the SEFAZ call itself. Production code never
// reassigns these; they always resolve to the real go-dfe implementation.
var (
	godfeImplements = godfe.Implements
	godfeCall       = godfe.Call
)

// normalizeSefazEnvironment converts worker's environment strings
// (sefazEnvProd="producao"/sefazEnvHom="homologacao") to the "prod"/"hom"
// form go-dfe's Request.Environment expects. py-dfe normalizes this itself
// (Pydantic validator in py-dfe/py_dfe/models/request.py); go-dfe does not
// replicate that, so callers normalize before calling godfe.Call — see
// go-dfe/request.go's doc comment.
func normalizeSefazEnvironment(env string) string {
	switch env {
	case sefazEnvProd:
		return envProd
	case sefazEnvHom:
		return envHom
	default:
		return env
	}
}

// mapToDfeRequest builds a godfe.Request from distribution.go's
// map[string]any payload shape (DistributionService.buildPayload). Returns
// ok=false if the payload is missing a field this path relies on.
func mapToDfeRequest(payload map[string]any) (godfe.Request, bool) {
	cnpj, _ := payload["cnpj"].(string)
	certB64, _ := payload["certificate_b64"].(string)
	certPassword, _ := payload["certificate_password"].(string)
	uf, _ := payload["uf"].(string)
	environment, _ := payload["environment"].(string)
	docType, _ := payload["doc_type"].(string)
	service, _ := payload["service"].(string)
	body, ok := payload["body"].(map[string]any)
	// UF vazia só é válida em NFS-e (competência municipal, sem UF autorizadora);
	// nos demais docTypes ela endereça o webservice, e sem ela a chamada é inútil.
	if !ok || docType == "" || service == "" || (uf == "" && !isNfse(docType)) {
		return godfe.Request{}, false
	}
	return godfe.Request{
		CNPJ: cnpj, CertificateB64: certB64, CertificatePassword: certPassword,
		UF: uf, Environment: normalizeSefazEnvironment(environment),
		DocType: docType, Service: service, Body: body,
	}, true
}

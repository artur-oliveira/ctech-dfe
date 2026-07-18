// Package dfe is the entrypoint of go-dfe: in-process SEFAZ (Brazilian tax
// authority) SOAP communication for NF-e/NFC-e/CT-e/MDF-e, called directly by
// worker/api instead of invoking the py-dfe Lambda. Request/Response/Problem
// mirror py-dfe's LambdaRequest/LambdaResponse/Problem (py-dfe/py_dfe/models/request.py,
// py-dfe/py_dfe/exceptions.py) so worker/api's existing lambdaPayload/lambdaResponse
// marshaling can be swapped for an in-process Call with no wire-format change.
package dfe

// Request mirrors py-dfe's LambdaRequest (py-dfe/py_dfe/models/request.py).
// Environment must already be normalized to "prod"/"hom" (py-dfe accepts
// "producao"/"homologacao" too and normalizes on the way in; go-dfe callers
// are internal to this monorepo and always send the normalized form).
type Request struct {
	CNPJ                string         `json:"cnpj"`
	CertificateB64      string         `json:"certificate_b64,omitempty"`
	CertificatePassword string         `json:"certificate_password,omitempty"`
	UF                  string         `json:"uf"`
	Environment         string         `json:"environment"`
	DocType             string         `json:"doc_type"`
	Service             string         `json:"service"`
	Body                map[string]any `json:"body"`
	ValidateSchema      bool           `json:"validate_schema,omitempty"`
	MaxRetries          int            `json:"max_retries,omitempty"`
}

// Response mirrors py-dfe's LambdaResponse: Body is a JSON-encoded string,
// not a nested object, matching the existing Lambda Invoke contract so
// worker/api's response-parsing code needs no changes when switching from
// invokePyDfe to dfe.Call.
type Response struct {
	StatusCode int               `json:"statusCode"`
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

// Problem mirrors py-dfe's RFC7807-shaped Problem (py-dfe/py_dfe/exceptions.py).
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

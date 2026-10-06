// Package dfe is the entrypoint of go-dfe: SEFAZ (Brazilian tax authority)
// SOAP and NFS-e REST communication for NF-e/NFC-e/CT-e/MDF-e/NFS-e. It is
// executed by the go-dfe-egress Lambda (sa-east-1), which receives a Request
// as JSON and returns a Response. Request/Response/Problem are that Lambda's
// wire contract; worker/api marshal them without any further translation.
package dfe

// Request is the input of Call. Environment must already be normalized to
// "prod"/"hom"; the go-dfe-egress handler converts the long forms
// ("producao"/"homologacao") that worker/api send.
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

// Response is the output of Call. Body is a JSON-encoded string, not a nested
// object, so callers keep one response-parsing path for every doc type.
type Response struct {
	StatusCode int               `json:"statusCode"`
	Body       string            `json:"body"`
	Headers    map[string]string `json:"headers"`
}

// Problem is the RFC 7807-shaped error body carried in Response.Body.
type Problem struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

// Package contract is the single source of the wire contracts shared across
// the ctech-dfe modules (B17). It is resolved through the repo-root go.work —
// api and worker alias these types instead of redeclaring them.
//
// The other cross-module contract, the py-dfe/go-dfe SEFAZ invoke payload, is
// godfe.Request/Response (gopkg.aoctech.app/dfe/go-dfe/request.go); the
// map-literal payloads built in api/internal/services/{external,distributions}.go
// mirror its JSON keys.
package contract

// WorkerMessage is the SNS command payload: published by the API
// (services.WorkerService.PublishWorkerEvent), consumed by the SQS document
// workers. A field change here changes the wire format for both sides at once.
type WorkerMessage struct {
	DocPK            string         `json:"doc_pk"`
	AccessKey        string         `json:"access_key"`
	TableName        string         `json:"table_name"`
	S3Prefix         string         `json:"s3_prefix"`
	ExpectedFileName string         `json:"expected_file_name"`
	CNPJ             string         `json:"cnpj"`
	UF               string         `json:"uf"`
	SefazEnvironment string         `json:"sefaz_environment"`
	CertS3Key        string         `json:"cert_s3_key"`
	CertPassword     string         `json:"cert_password"` // may be certcrypt "kms1:" ciphertext (B4)
	DocType          string         `json:"doc_type"`
	SefazService     string         `json:"sefaz_service"`
	Body             map[string]any `json:"body"`
	// Event fields — present only for SEFAZ event operations (cancellation, CC-e, …)
	EventsTableName *string `json:"events_table_name,omitempty"`
	EventType       *string `json:"event_type,omitempty"`
	SequenceNumber  *int    `json:"sequence_number,omitempty"`
	EventSK         *string `json:"event_sk,omitempty"`
}

package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Webhook event types of the v1 (docs/specs/2026-11-15-webhooks.md): the
// terminal states a document reaches, the same ones the ResultsConsumer receives.
const (
	WebhookEventAuthorized = "document.authorized"
	WebhookEventRejected   = "document.rejected"
	WebhookEventFailed     = "document.failed"
)

// WebhookEventTypes is every type an endpoint may subscribe to ("ping" is not
// one of them: nobody subscribes to the console's test).
var WebhookEventTypes = []string{WebhookEventAuthorized, WebhookEventRejected, WebhookEventFailed}

// WebhookHeaderPrefix names the delivery headers: X-Ctech-Event-Id, -Event-Type,
// -Timestamp and -Signature.
const WebhookHeaderPrefix = "X-Ctech-"

// WebhookUserAgent identifies the deliveries and the registration challenge.
const WebhookUserAgent = "ctech-dfe-webhooks/1"

// WebhookDocument is what an event says about a document: the minimum a receiver
// needs to know something happened. Anything else it reads from the API with its
// own credential, so a URL that is wrong leaks an id, never a fiscal document.
type WebhookDocument struct {
	ID, AccessKey, Status string
	// DocumentType says which API to read the document from: nfe, nfce, cte, mdfe, nfcom, bpe or nfse.
	DocumentType string
	// RejectionCode and RejectionMessage are set only for a rejection.
	RejectionCode, RejectionMessage string
}

type webhookRejection struct {
	Code    string `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

type webhookData struct {
	ID        string `json:"id,omitempty"`
	AccessKey string `json:"access_key,omitempty"`
	Status    string `json:"status,omitempty"`
	// DocumentType is the one field beyond the minimum: an id alone does not say
	// which of the five documents it names.
	DocumentType string            `json:"document_type,omitempty"`
	Rejection    *webhookRejection `json:"rejection,omitempty"`
}

type webhookBody struct {
	ID         string      `json:"id"`
	Type       string      `json:"type"`
	OccurredAt string      `json:"occurred_at"`
	Data       webhookData `json:"data"`
}

// BuildWebhookBody renders the exact payload sent for an event.
func BuildWebhookBody(eventID, eventType string, occurredAt time.Time, d WebhookDocument) ([]byte, error) {
	b := webhookBody{
		ID: eventID, Type: eventType, OccurredAt: occurredAt.UTC().Format(time.RFC3339),
		Data: webhookData{ID: d.ID, AccessKey: d.AccessKey, Status: d.Status, DocumentType: d.DocumentType},
	}
	if d.RejectionCode != "" || d.RejectionMessage != "" {
		b.Data.Rejection = &webhookRejection{Code: d.RejectionCode, Message: d.RejectionMessage}
	}
	return json.Marshal(b)
}

// ResultDedupeKey names a document result (company, document and status) independently of the queue message that
// reported it: two messages for one result carry two event ids but one key. Webhooks and push both dedupe by it.
func ResultDedupeKey(companyPK, documentKey, status string) string {
	sum := sha256.Sum256([]byte(companyPK + "|" + documentKey + "|" + status))
	return hex.EncodeToString(sum[:16])
}

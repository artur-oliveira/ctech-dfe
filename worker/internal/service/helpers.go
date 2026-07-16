package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	StatusAuthorized = "authorized"
	StatusRejected   = "rejected"
	StatusFailed     = "failed"
	StatusCancelled  = "cancelled"
	StatusClosed     = "closed"

	EventStatusSuccess = "success"
	EventStatusError   = "error"

	// resultKindDocument / resultKindEvent discriminate the SNS result
	// notification: a document result carries the document status, an event
	// result carries the SEFAZ event outcome (cancellation, encerramento, …)
	// so the frontend does not mistake a reverted document status for the
	// event outcome.
	resultKindDocument = "document"
	resultKindEvent    = "event"

	// Result notification payload keys (worker SNS → API → websocket).
	notifyKeyResultKind    = "result_kind"
	notifyKeyAccessKey     = "access_key"
	notifyKeyDocPK         = "doc_pk"
	notifyKeyTableName     = "table_name"
	notifyKeyStatus        = "status"
	notifyKeySefazStatus   = "sefaz_status"
	notifyKeySefazMotive   = "sefaz_motive"
	notifyKeySefazProtocol = "sefaz_protocol"
	notifyKeyXMLS3Key      = "xml_s3_key"
	notifyKeyEventType     = "event_type"
	notifyKeyEventSK       = "event_sk"

	// docTypeMDFe is the doc_type carried on the worker message for MDF-e.
	docTypeMDFe = "mdfe"
	docTypeNFCe = "nfce"

	cancellationEvent = "110111"
	// cancellationSubstEvent — NFC-e "Cancelamento por Substituição"; like a
	// regular cancellation, a successful event transitions the doc to cancelled.
	// Note: the SAME code 110112 means "Encerramento" for MDF-e — disambiguated
	// by doc_type in isCancellationEvent / isCloseEvent.
	cancellationSubstEvent = "110112"
	// mdfeEncerramentoEvent — MDF-e "Encerramento" (closes the manifest).
	mdfeEncerramentoEvent = "110112"

	DuplicatedEventError = "573"
)

// isCancellationEvent reports whether the event type cancels the document.
// 110111 cancels any document; 110112 is "cancelamento por substituição" only
// for NF-e/NFC-e — for MDF-e 110112 is encerramento (see isCloseEvent).
func isCancellationEvent(docType string, eventType *string) bool {
	if eventType == nil {
		return false
	}
	if *eventType == cancellationEvent {
		return true
	}
	return *eventType == cancellationSubstEvent && docType == docTypeNFCe
}

// isCloseEvent reports whether the event encerra (closes) an MDF-e (110112).
func isCloseEvent(docType string, eventType *string) bool {
	return eventType != nil && docType == docTypeMDFe && *eventType == mdfeEncerramentoEvent
}

var authorizedStats = map[string]bool{
	"100": true, "135": true, "136": true, "150": true, "155": true,
}

// batchStats are envelope-level cStat codes that wrap per-document results
// inside infProt or infEvento.
var batchStats = map[string]bool{
	"104": true, "128": true,
}

// updateParts holds the components of a DynamoDB SET expression.
type updateParts struct {
	expression string
	attrNames  map[string]string
	attrValues map[string]types.AttributeValue
}

// updateAttrs holds optional fields for a status update.
// Nil fields are excluded from the SET expression.
type updateAttrs struct {
	SefazStatus   *string
	SefazMotive   *string
	SefazProtocol *string
	XMLS3Key      *string
}

// buildUpdateExpression builds DynamoDB SET expression components for a status
// update. Only non-nil optional attrs are included.
func buildUpdateExpression(status string, attrs updateAttrs) updateParts {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	attrValues := map[string]types.AttributeValue{
		":status":  &types.AttributeValueMemberS{Value: status},
		":updated": &types.AttributeValueMemberS{Value: now},
	}
	setParts := []string{"#status = :status", "updated_at = :updated"}
	attrNames := map[string]string{"#status": "status"}

	optionals := []struct {
		name string
		val  *string
	}{
		{"sefaz_status", attrs.SefazStatus},
		{"sefaz_motive", attrs.SefazMotive},
		{"sefaz_protocol", attrs.SefazProtocol},
		{"xml_s3_key", attrs.XMLS3Key},
	}
	for _, o := range optionals {
		if o.val != nil {
			ph := ":" + o.name
			attrValues[ph] = &types.AttributeValueMemberS{Value: *o.val}
			setParts = append(setParts, o.name+" = "+ph)
		}
	}
	return updateParts{
		expression: "SET " + strings.Join(setParts, ", "),
		attrNames:  attrNames,
		attrValues: attrValues,
	}
}

// findValue recursively searches data for the first occurrence of key and
// returns its value as a string. Returns nil if not found.
func findValue(data any, key string) *string {
	switch v := data.(type) {
	case map[string]any:
		if val, ok := v[key]; ok {
			return new(fmt.Sprintf("%v", val))
		}
		for _, child := range v {
			if result := findValue(child, key); result != nil {
				return result
			}
		}
	case []any:
		for _, item := range v {
			if result := findValue(item, key); result != nil {
				return result
			}
		}
	}
	return nil
}

// findDict recursively searches data for the first occurrence of key whose
// value is a map. Returns nil if not found or value is not a map.
func findDict(data any, key string) map[string]any {
	switch v := data.(type) {
	case map[string]any:
		if val, ok := v[key]; ok {
			if m, ok := val.(map[string]any); ok {
				return m
			}
		}
		for _, child := range v {
			if result := findDict(child, key); result != nil {
				return result
			}
		}
	case []any:
		for _, item := range v {
			if result := findDict(item, key); result != nil {
				return result
			}
		}
	}
	return nil
}

// strVal dereferences a *string, returning "" if nil.
func strVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

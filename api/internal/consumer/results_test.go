package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/ws"
)

// fakeConn records every payload written to it, letting a test assert a
// broadcast reached a specific org_pk key without needing a real WebSocket.
type fakeConn struct {
	messages [][]byte
}

func (f *fakeConn) WriteMessage(_ int, data []byte) error {
	f.messages = append(f.messages, data)
	return nil
}

func sqsMessageWithBody(body []byte) sqstypes.Message {
	return sqstypes.Message{Body: aws.String(string(body))}
}

func TestDispatch_BroadcastsWithOrgPKOnly(t *testing.T) {
	t.Parallel()
	reg := ws.NewMemoryRegistry()
	conn := &fakeConn{}
	reg.Register("CNPJ_12345678000195", "conn-1", conn)
	c := &ResultsConsumer{registry: reg, cache: cache.NewMemoryBackend(16)}

	event := map[string]any{
		"type":       "new_distribution_nfe",
		"org_pk":     "CNPJ_12345678000195",
		"access_key": "35250512345678000195550010000000011000000015",
	}
	body, _ := json.Marshal(event)
	c.dispatch(context.Background(), sqsMessageWithBody(body))

	if len(conn.messages) != 1 {
		t.Fatalf("broadcasts = %d, want 1 (org_pk without doc_pk must still dispatch)", len(conn.messages))
	}

	var got map[string]any
	if err := json.Unmarshal(conn.messages[0], &got); err != nil {
		t.Fatalf("unmarshal broadcast payload: %v", err)
	}
	if got["type"] != "new_distribution_nfe" {
		t.Errorf("type = %v, want %q (must not be clobbered to dfe_result)", got["type"], "new_distribution_nfe")
	}
}

func TestDispatch_DocPKMessageDefaultsTypeToDfeResult(t *testing.T) {
	t.Parallel()
	reg := ws.NewMemoryRegistry()
	conn := &fakeConn{}
	reg.Register("CNPJ_12345678000195", "conn-1", conn)
	c := &ResultsConsumer{registry: reg, cache: cache.NewMemoryBackend(16)}

	event := map[string]any{
		"doc_pk":     "prod#CNPJ_12345678000195",
		"access_key": "35250512345678000195550010000000011000000015",
	}
	body, _ := json.Marshal(event)
	c.dispatch(context.Background(), sqsMessageWithBody(body))

	if len(conn.messages) != 1 {
		t.Fatalf("broadcasts = %d, want 1", len(conn.messages))
	}
	var got map[string]any
	if err := json.Unmarshal(conn.messages[0], &got); err != nil {
		t.Fatalf("unmarshal broadcast payload: %v", err)
	}
	if got["type"] != "dfe_result" {
		t.Errorf("type = %v, want %q", got["type"], "dfe_result")
	}
}

func TestDispatch_MissingBothPKsIsDropped(t *testing.T) {
	t.Parallel()
	reg := ws.NewMemoryRegistry()
	conn := &fakeConn{}
	reg.Register("CNPJ_12345678000195", "conn-1", conn)
	c := &ResultsConsumer{registry: reg, cache: cache.NewMemoryBackend(16)}

	event := map[string]any{"access_key": "35250512345678000195550010000000011000000015"}
	body, _ := json.Marshal(event)
	c.dispatch(context.Background(), sqsMessageWithBody(body))

	if len(conn.messages) != 0 {
		t.Fatalf("broadcasts = %d, want 0 (no doc_pk and no org_pk)", len(conn.messages))
	}
}

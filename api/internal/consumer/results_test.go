package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"gopkg.aoctech.app/dfe/api/internal/services"
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

// Duas abas logadas na mesma organização são duas conexões com connID próprio
// sob a mesma chave: as duas recebem o resultado, e a aba de outra organização não.
func TestDispatch_BroadcastReachesEveryTabOfTheOrg(t *testing.T) {
	t.Parallel()
	reg := ws.NewMemoryRegistry()
	tabA, tabB, otherOrg := &fakeConn{}, &fakeConn{}, &fakeConn{}
	reg.Register("CNPJ_12345678000195", "conn-tab-a", tabA)
	reg.Register("CNPJ_12345678000195", "conn-tab-b", tabB)
	reg.Register("CNPJ_99999999000191", "conn-other", otherOrg)
	c := &ResultsConsumer{registry: reg, cache: cache.NewMemoryBackend(16)}

	body, _ := json.Marshal(map[string]any{
		"doc_pk":     "prod#CNPJ_12345678000195",
		"table_name": "nfses",
		"status":     "authorized",
		"access_key": "DPS1",
	})
	c.dispatch(context.Background(), sqsMessageWithBody(body))

	if len(tabA.messages) != 1 || len(tabB.messages) != 1 {
		t.Fatalf("abas da mesma organização receberam %d e %d mensagens, quer 1 e 1", len(tabA.messages), len(tabB.messages))
	}
	if len(otherOrg.messages) != 0 {
		t.Errorf("aba de outra organização recebeu %d mensagens", len(otherOrg.messages))
	}
}

type fakeWebhooks struct {
	got []services.WebhookResult
	err error
}

func (f *fakeWebhooks) Emit(_ context.Context, r services.WebhookResult) error {
	f.got = append(f.got, r)
	return f.err
}

func resultMessage(event map[string]any, sentAt string) sqstypes.Message {
	body, _ := json.Marshal(event)
	m := sqsMessageWithBody(body)
	if sentAt != "" {
		m.Attributes = map[string]string{"SentTimestamp": sentAt}
	}
	return m
}

func documentResult(status string) map[string]any {
	return map[string]any{
		"result_kind": "document", "table_name": "nfes", "status": status,
		"doc_pk": "prod#COMP1", "access_key": "35250512345678000195550010000000011000000015",
		"sefaz_status": "225", "sefaz_motive": "Falha no Schema XML",
		"billing_organization_id": "acct-org-1",
	}
}

func TestDispatch_TerminalDocumentResultEmitsAWebhookEvent(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"authorized", "rejected", "failed"} {
		hooks := &fakeWebhooks{}
		c := (&ResultsConsumer{registry: ws.NewMemoryRegistry(), cache: cache.NewMemoryBackend(16)}).WithWebhooks(hooks)
		if err := c.dispatch(context.Background(), resultMessage(documentResult(status), "1800000000123")); err != nil {
			t.Fatalf("%s: %v", status, err)
		}
		if len(hooks.got) != 1 {
			t.Fatalf("%s: emitted %d events, want 1", status, len(hooks.got))
		}
		r := hooks.got[0]
		if r.CompanyPK != "COMP1" || r.OrganizationID != "acct-org-1" || r.Table != "nfes" || r.Status != status ||
			r.DocumentKey != "35250512345678000195550010000000011000000015" || r.SentAt.UnixMilli() != 1800000000123 {
			t.Fatalf("%s: result = %+v", status, r)
		}
		if r.SefazStatus != "225" || r.SefazMotive != "Falha no Schema XML" {
			t.Fatalf("%s: rejection fields lost: %+v", status, r)
		}
	}
}

func TestDispatch_OnlyTerminalDocumentResultsEmit(t *testing.T) {
	t.Parallel()
	for name, event := range map[string]map[string]any{
		"in flight":    documentResult("retryable_failed"),
		"processing":   documentResult("processing"),
		"event result": {"result_kind": "event", "table_name": "nfes", "status": "authorized", "doc_pk": "prod#C", "access_key": "k"},
		"distribution": {"type": "new_distribution_nfe", "org_pk": "C", "access_key": "k"},
		"ciot":         {"result_kind": "document", "table_name": "ciots", "status": "authorized", "doc_pk": "prod#C", "access_key": "k"},
	} {
		hooks := &fakeWebhooks{}
		c := (&ResultsConsumer{registry: ws.NewMemoryRegistry(), cache: cache.NewMemoryBackend(16)}).WithWebhooks(hooks)
		if err := c.dispatch(context.Background(), resultMessage(event, "")); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(hooks.got) != 0 {
			t.Errorf("%s emitted %+v", name, hooks.got)
		}
	}
}

func TestDispatch_AStoreFailureLeavesTheMessageOnTheQueue(t *testing.T) {
	t.Parallel()
	hooks := &fakeWebhooks{err: errors.New("dynamodb down")}
	c := (&ResultsConsumer{registry: ws.NewMemoryRegistry(), cache: cache.NewMemoryBackend(16)}).WithWebhooks(hooks)
	if err := c.dispatch(context.Background(), resultMessage(documentResult("authorized"), "")); err == nil {
		t.Fatal("a failed webhook write was swallowed; the message would be deleted and the event lost")
	}
}

func TestDispatch_NoWebhooksWiredStillDispatches(t *testing.T) {
	t.Parallel()
	c := &ResultsConsumer{registry: ws.NewMemoryRegistry(), cache: cache.NewMemoryBackend(16)}
	if err := c.dispatch(context.Background(), resultMessage(documentResult("authorized"), "")); err != nil {
		t.Fatal(err)
	}
}

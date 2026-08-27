package service

import (
	"context"
	"testing"
)

// inutMsg mirrors what api/internal/services/nfes/inutilization.go publishes:
// an events-table row with a synthetic access key (inutilização has no chave de
// acesso) and no document to update.
var inutMsg = WorkerMessage{
	DocPK:            "hom#CNPJ_11647612000197",
	AccessKey:        "INUT#hom#CNPJ_11647612000197",
	ExpectedFileName: "inut_2026_000_000000004_000000005",
	TableName:        "nfces",
	S3Prefix:         "nfce",
	CNPJ:             "11647612000197",
	UF:               "PI",
	SefazEnvironment: "homologacao",
	CertS3Key:        "certs/org-123/cert.pfx",
	CertPassword:     "senha123",
	DocType:          "nfce",
	SefazService:     "NfeInutilizacao",
	Body:             map[string]any{"inutNFe": map[string]any{}},
	EventsTableName:  new("nfce_events"),
	EventType:        new("INUT"),
	SequenceNumber:   new(1),
	EventSK:          new("01a040ad-28b4-7d1c-93d0-ac9377391f6b"),
}

// cStat 102 ("Inutilizacao de numero homologado") is the success code of the
// NfeInutilizacao service. Treating it as a rejection marked a homologated
// range as rejected in production (2026-08-27).
func TestProcess_CStat102_MarksInutilizationSuccess(t *testing.T) {
	s3m := certS3()
	dynm := &mockDynamo{}
	svc := newSvc(s3m, &mockLambda{
		payload: invokeResp("102", "Inutilizacao de numero homologado", "322260000058123"),
	}, dynm)

	if err := svc.Process(context.Background(), inutMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(s3m.putCalls) != 1 {
		t.Errorf("expected the response to be stored in S3, got %d puts", len(s3m.putCalls))
	}
	// Only the event row is touched — an inutilização has no document.
	if len(dynm.updates) != 1 {
		t.Fatalf("expected 1 DynamoDB update, got %d: %+v", len(dynm.updates), dynm.updates)
	}
	got := dynm.updates[0]
	if got.status != EventStatusSuccess {
		t.Errorf("status = %q, want %q", got.status, EventStatusSuccess)
	}
	if got.table != testCfg.TablePrefix+"_nfce_events" {
		t.Errorf("table = %q", got.table)
	}
}

func TestProcess_InutilizationRejected_MarksRejected(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{
		payload: invokeResp("563", "Ja existe NF-e autorizada para a faixa informada", ""),
	}, dynm)

	if err := svc.Process(context.Background(), inutMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updates) != 1 {
		t.Fatalf("expected 1 DynamoDB update, got %d", len(dynm.updates))
	}
	if dynm.updates[0].status != StatusRejected {
		t.Errorf("status = %q, want %q", dynm.updates[0].status, StatusRejected)
	}
}

// A SEFAZ business rejection of an event is final. It must be recognised as
// terminal on redelivery, otherwise the message errors until it hits the DLQ.
func TestProcess_RejectedEvent_IsTerminalOnRedelivery(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("102", "Inutilizacao de numero homologado", "")}
	dynm := &mockDynamo{getItemOutput: statusItem(StatusRejected)}
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), inutMsg); err != nil {
		t.Fatalf("redelivery of a rejected event must be a no-op, got: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("expected SEFAZ NOT to be called again, got %d calls", lamm.calls)
	}
	if len(dynm.updates) != 0 {
		t.Errorf("expected no further updates, got %+v", dynm.updates)
	}
}

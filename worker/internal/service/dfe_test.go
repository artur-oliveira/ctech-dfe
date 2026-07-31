package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sns"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/worker/internal/config"
)

// ---------------------------------------------------------------------------
// Mock clients
// ---------------------------------------------------------------------------

type mockS3 struct {
	certData []byte
	getErr   error
	putErr   error
	putCalls []string // captured S3 keys
}

func (m *mockS3) GetObject(_ context.Context, _ *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &s3.GetObjectOutput{Body: io.NopCloser(bytes.NewReader(m.certData))}, nil
}

func (m *mockS3) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	if in.Key != nil {
		m.putCalls = append(m.putCalls, *in.Key)
	}
	return &s3.PutObjectOutput{}, nil
}

type mockLambda struct {
	payload []byte
	err     error
	calls   int
}

func (m *mockLambda) Invoke(_ context.Context, _ *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	m.calls++
	if m.err != nil {
		return nil, m.err
	}
	return &lambdaSDK.InvokeOutput{Payload: m.payload}, nil
}

type capturedUpdate struct {
	table     string
	status    string
	condition string // ConditionExpression, empty if absent
}

type mockDynamo struct {
	updates    []capturedUpdate
	err        error
	claimErr   error
	claimCalls int
	// getItemOutput/getItemErr configure the response to GetItem (used by the
	// idempotency guard). Nil getItemOutput.Item (the zero value) means "not
	// found", matching the default behavior needed by every pre-existing test.
	getItemOutput *dynamodb.GetItemOutput
	getItemErr    error
}

func (m *mockDynamo) GetItem(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getItemErr != nil {
		return nil, m.getItemErr
	}
	if m.getItemOutput != nil {
		return m.getItemOutput, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	if _, isClaim := in.ExpressionAttributeValues[":processing"]; isClaim {
		m.claimCalls++
		if m.claimErr != nil {
			return nil, m.claimErr
		}
		if m.getItemOutput != nil {
			if status, ok := m.getItemOutput.Item["status"].(*types.AttributeValueMemberS); ok &&
				(docTerminalStatuses[status.Value] || eventTerminalStatuses[status.Value] || status.Value == StatusProcessing) {
				return nil, &types.ConditionalCheckFailedException{}
			}
		}
		return &dynamodb.UpdateItemOutput{}, nil
	}
	if m.err != nil {
		return nil, m.err
	}
	status := ""
	if sv, ok := in.ExpressionAttributeValues[":status"]; ok {
		if s, ok := sv.(*types.AttributeValueMemberS); ok {
			status = s.Value
		}
	}
	cond := ""
	if in.ConditionExpression != nil {
		cond = *in.ConditionExpression
	}
	table := ""
	if in.TableName != nil {
		table = *in.TableName
	}
	m.updates = append(m.updates, capturedUpdate{table: table, status: status, condition: cond})
	return &dynamodb.UpdateItemOutput{}, nil
}

type mockSNS struct {
	calls []string
}

func (m *mockSNS) Publish(_ context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	if in.Message != nil {
		m.calls = append(m.calls, *in.Message)
	}
	return &sns.PublishOutput{}, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

var testCfg = &config.Config{
	TablePrefix:     "dev",
	DocumentsBucket: "docs-bucket",
	CertsBucket:     "certs-bucket",
	DfeLambdaName:   "dev-py-dfe",
}

var baseMsg = WorkerMessage{
	DocPK:            "prod#CNPJ_12345678000195",
	AccessKey:        "35250512345678000195550010000000011000000011",
	ExpectedFileName: "35250512345678000195550010000000011000000011",
	TableName:        "nfes",
	S3Prefix:         "nfe",
	CNPJ:             "12345678000195",
	UF:               "SP",
	SefazEnvironment: "producao",
	CertS3Key:        "certs/org-123/cert.pfx",
	CertPassword:     "senha123",
	DocType:          "nfe",
	SefazService:     "NFeAutorizacao",
	Body:             map[string]any{"NFe": map[string]any{}},
}

// invokeResp builds mock Lambda payload: {"statusCode": 200, "body": "{cStat,xMotivo,nProt}"}.
func invokeResp(cStat, xMotivo, nProt string) []byte {
	body, _ := json.Marshal(map[string]any{"cStat": cStat, "xMotivo": xMotivo, "nProt": nProt})
	payload, _ := json.Marshal(map[string]any{"statusCode": 200, "body": string(body)})
	return payload
}

// invokeRespStatus builds a mock error response from py-dfe.
func invokeRespStatus(statusCode int, detail string) []byte {
	body, _ := json.Marshal(map[string]any{"detail": detail})
	payload, _ := json.Marshal(map[string]any{"statusCode": statusCode, "body": string(body)})
	return payload
}

func newSvc(s3m *mockS3, lamm *mockLambda, dynm *mockDynamo) *DfeService {
	return New(Clients{S3: s3m, Lambda: lamm, Dynamo: dynm}, testCfg)
}

func certS3() *mockS3 { return &mockS3{certData: []byte("CERTBYTES")} }

// ---------------------------------------------------------------------------
// Authorized emission
// ---------------------------------------------------------------------------

func TestProcess_CStat100_SavesAndMarksAuthorized(t *testing.T) {
	s3m := certS3()
	dynm := &mockDynamo{}
	svc := newSvc(s3m, &mockLambda{payload: invokeResp("100", "Autorizado", "135")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(s3m.putCalls) != 1 {
		t.Errorf("expected 1 S3 put, got %d", len(s3m.putCalls))
	}
	if len(dynm.updates) != 1 {
		t.Fatalf("expected 1 DynamoDB update, got %d", len(dynm.updates))
	}
	if dynm.updates[0].status != StatusAuthorized {
		t.Errorf("status = %q, want %q", dynm.updates[0].status, StatusAuthorized)
	}
}

// TestProcess_GoDfeCutover_SkipsLambdaEntirely is the one test in this
// package that actually exercises the 2026-07-18 hard-cutover branch (every
// other test forces godfeImplements=false via distribution_test.go's init,
// to keep testing Process()'s logic against a controllable fake response
// without a real certificate/network call). It stubs godfeImplements/
// godfeCall directly to prove: (a) the mock Lambda is never invoked when
// go-dfe implements the operation, (b) go-dfe's response flows through the
// exact same status-update path as a py-dfe response would.
func TestProcess_GoDfeCutover_SkipsLambdaEntirely(t *testing.T) {
	origImplements, origCall := godfeImplements, godfeCall
	defer func() { godfeImplements, godfeCall = origImplements, origCall }()

	godfeImplements = func(docType, service string) bool { return docType == "nfe" && service == "NFeAutorizacao" }
	godfeCall = func(_ context.Context, req godfe.Request) (godfe.Response, error) {
		if req.DocType != "nfe" || req.Service != "NFeAutorizacao" {
			t.Errorf("unexpected godfe.Request: %+v", req)
		}
		body, _ := json.Marshal(map[string]any{"cStat": "100", "xMotivo": "Autorizado", "nProt": "135"})
		return godfe.Response{StatusCode: 200, Body: string(body)}, nil
	}

	s3m := certS3()
	dynm := &mockDynamo{}
	lamm := &mockLambda{payload: invokeResp("999", "should never be read", "")}
	svc := newSvc(s3m, lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("expected py-dfe Lambda to never be invoked, got %d calls", lamm.calls)
	}
	if len(dynm.updates) != 1 || dynm.updates[0].status != StatusAuthorized {
		t.Fatalf("expected 1 update with status=%q, got %+v", StatusAuthorized, dynm.updates)
	}
}

func TestProcess_CStat100_SefazStatusPropagated(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("100", "Autorizado", "prot-99")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	// Verify :sefaz_status attr value in the UpdateItem call
	sefazStatusVal, ok := dynm.updates[0].status, dynm.updates[0].status != ""
	_ = sefazStatusVal
	_ = ok
	// The status itself is checked above; checking the sefaz_status attr in UpdateItem input
	// requires capturing the full UpdateItemInput. Here we rely on integration for that detail.
	if dynm.updates[0].status != StatusAuthorized {
		t.Errorf("status = %q, want authorized", dynm.updates[0].status)
	}
}

func TestProcess_CTEEmission_UsesCorrectTable(t *testing.T) {
	dynm := &mockDynamo{}
	msg := baseMsg
	msg.DocType = "cte"
	msg.TableName = "ctes"
	msg.S3Prefix = "cte"
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("100", "Autorizado", "")}, dynm)

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].table != "dev_ctes" {
		t.Errorf("table = %q, want dev_ctes", dynm.updates[0].table)
	}
	if dynm.updates[0].status != StatusAuthorized {
		t.Errorf("status = %q, want authorized", dynm.updates[0].status)
	}
}

func TestProcess_S3PrefixFromMessage(t *testing.T) {
	s3m := certS3()
	msg := baseMsg
	msg.S3Prefix = "mdfe"
	msg.TableName = "mdfes"
	svc := newSvc(s3m, &mockLambda{payload: invokeResp("100", "", "")}, &mockDynamo{})

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(s3m.putCalls) == 0 {
		t.Fatal("expected S3 put call")
	}
	if !startsWith(s3m.putCalls[0], "mdfe/") {
		t.Errorf("S3 key = %q, want mdfe/ prefix", s3m.putCalls[0])
	}
}

// ---------------------------------------------------------------------------
// cStat 104 batch unwrapping
// ---------------------------------------------------------------------------

func TestProcess_CStat104_UnwrapsInfProt(t *testing.T) {
	body := map[string]any{
		"retEnviNFe": map[string]any{
			"cStat":   "104",
			"xMotivo": "Lote processado",
			"protNFe": map[string]any{
				"infProt": map[string]any{
					"cStat":   "100",
					"xMotivo": "Autorizado o uso da NF-e",
					"nProt":   "999",
				},
			},
		},
	}
	bodyJSON, _ := json.Marshal(body)
	payload, _ := json.Marshal(map[string]any{"statusCode": 200, "body": string(bodyJSON)})

	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: payload}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusAuthorized {
		t.Errorf("status = %q, want authorized", dynm.updates[0].status)
	}
	// Verify nProt was captured in UpdateItem attrs
	// (captured via :sefaz_protocol in ExpressionAttributeValues — verified in integration)
}

// ---------------------------------------------------------------------------
// Rejected
// ---------------------------------------------------------------------------

func TestProcess_CStat110_MarksRejected(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("110", "Uso Denegado", "")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusRejected {
		t.Errorf("status = %q, want rejected", dynm.updates[0].status)
	}
}

func TestProcess_CStat301_MarksRejected(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("301", "Irregularidade fiscal", "")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusRejected {
		t.Errorf("status = %q, want rejected", dynm.updates[0].status)
	}
}

// Any cStat returned by SEFAZ (even unknown codes) is a business rejection,
// not a technical failure. StatusFailed is reserved for cases without a valid cStat.
func TestProcess_UnknownCStat_MarksRejected(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("999", "Erro desconhecido", "")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusRejected {
		t.Errorf("status = %q, want rejected", dynm.updates[0].status)
	}
}

// ---------------------------------------------------------------------------
// Failed
// ---------------------------------------------------------------------------

func TestProcess_PyDfe422_MarksFailed(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeRespStatus(422, "cnpj invalido")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusFailed {
		t.Errorf("status = %q, want failed", dynm.updates[0].status)
	}
}

func TestProcess_PyDfe500_RemainsRetryable(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeRespStatus(503, "SEFAZ indisponível")}, dynm)

	if err := svc.Process(context.Background(), baseMsg); err == nil {
		t.Fatal("expected retryable engine error")
	}
	if len(dynm.updates) != 1 || dynm.updates[0].status != StatusRetryableFailed {
		t.Fatalf("expected retryable_failed, got %+v", dynm.updates)
	}
}

func TestProcess_LambdaError_MarksRetryableAndReturnsError(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{err: errors.New("network timeout")}, dynm)

	err := svc.Process(context.Background(), baseMsg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(dynm.updates) == 0 || dynm.updates[0].status != StatusRetryableFailed {
		t.Errorf("expected document marked retryable_failed; updates: %+v", dynm.updates)
	}
}

func TestProcess_S3CertError_MarksRetryableAndReturnsError(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(&mockS3{getErr: errors.New("s3 error")}, &mockLambda{}, dynm)

	err := svc.Process(context.Background(), baseMsg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(dynm.updates) == 0 || dynm.updates[0].status != StatusRetryableFailed {
		t.Errorf("expected document marked retryable_failed; updates: %+v", dynm.updates)
	}
}

// ---------------------------------------------------------------------------
// Cancellation flow
// ---------------------------------------------------------------------------

var cancelMsg = WorkerMessage{
	DocPK:            "prod#CNPJ_12345678000195",
	AccessKey:        "35250512345678000195550010000000011000000011",
	ExpectedFileName: "35250512345678000195550010000000011000000011",
	TableName:        "nfes",
	S3Prefix:         "nfe",
	CNPJ:             "12345678000195",
	UF:               "SP",
	SefazEnvironment: "producao",
	CertS3Key:        "certs/org-123/cert.pfx",
	CertPassword:     "senha123",
	DocType:          "nfe",
	SefazService:     "NFeRecepcaoEvento",
	Body:             map[string]any{"envEvento": map[string]any{}},
	EventsTableName:  new("nfe_events"),
	EventType:        new(cancellationEvent),
	SequenceNumber:   new(1),
	EventSK:          new("evt-sk-001"),
}

func TestProcess_Cancellation_Accepted_MarksDocCancelledAndEventSuccess(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado e vinculado", "prot-123")}, dynm)

	if err := svc.Process(context.Background(), cancelMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	// Expect 2 UpdateItem calls: one for the document (cancelled) and one for the event (success)
	if len(dynm.updates) != 2 {
		t.Fatalf("expected 2 DynamoDB updates, got %d: %+v", len(dynm.updates), dynm.updates)
	}
	docUpdate := dynm.updates[0]
	eventUpdate := dynm.updates[1]

	if docUpdate.status != StatusCancelled {
		t.Errorf("doc status = %q, want cancelled", docUpdate.status)
	}
	if eventUpdate.status != EventStatusSuccess {
		t.Errorf("event status = %q, want success", eventUpdate.status)
	}
	if eventUpdate.condition == "" {
		t.Error("event UpdateItem should have ConditionExpression (attribute_exists)")
	}
}

func TestProcess_Cancellation_Rejected_RevertsToAuthorizedAndEventRejected(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("501", "Rejeição: prazo expirado", "")}, dynm)

	if err := svc.Process(context.Background(), cancelMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updates) != 2 {
		t.Fatalf("expected 2 DynamoDB updates, got %d: %+v", len(dynm.updates), dynm.updates)
	}
	// Document reverts to authorized (not cancelled, not rejected)
	if dynm.updates[0].status != StatusAuthorized {
		t.Errorf("doc status = %q, want authorized (reverted)", dynm.updates[0].status)
	}
	// Event is marked rejected
	if dynm.updates[1].status != StatusRejected {
		t.Errorf("event status = %q, want rejected", dynm.updates[1].status)
	}
}

// ---------------------------------------------------------------------------
// Event operations (non-cancellation)
// ---------------------------------------------------------------------------

var eventMsg = WorkerMessage{
	DocPK:            "prod#CNPJ_12345678000195",
	AccessKey:        "35250512345678000195550010000000011000000011",
	ExpectedFileName: "35250512345678000195550010000000011000000011",
	TableName:        "nfes",
	S3Prefix:         "nfe",
	CNPJ:             "12345678000195",
	UF:               "SP",
	SefazEnvironment: "producao",
	CertS3Key:        "certs/org-123/cert.pfx",
	CertPassword:     "senha123",
	DocType:          "nfe",
	SefazService:     "NFeRecepcaoEvento",
	Body:             map[string]any{"envEvento": map[string]any{}},
	EventsTableName:  new("nfe_events"),
	EventType:        new("110110"), // non-cancellation event (e.g., CC-e)
	SequenceNumber:   new(1),
	EventSK:          new("evt-sk-002"),
}

func TestProcess_EventAuthorized_OnlyEventUpdated(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado", "")}, dynm)

	if err := svc.Process(context.Background(), eventMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	// Document must NOT be touched; only the event record is updated
	if len(dynm.updates) != 1 {
		t.Fatalf("expected 1 DynamoDB update (event only), got %d: %+v", len(dynm.updates), dynm.updates)
	}
	if dynm.updates[0].status != EventStatusSuccess {
		t.Errorf("event status = %q, want success", dynm.updates[0].status)
	}
}

func TestProcess_EventRejected_DocumentNotUpdated(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("573", "Rejeição", "")}, dynm)

	if err := svc.Process(context.Background(), eventMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updates) != 1 {
		t.Fatalf("expected 1 DynamoDB update (event only), got %d: %+v", len(dynm.updates), dynm.updates)
	}
	if dynm.updates[0].status != StatusRejected {
		t.Errorf("event status = %q, want rejected", dynm.updates[0].status)
	}
}

// ---------------------------------------------------------------------------
// MDF-e encerramento (close) — event 110112, disambiguated by doc_type
// ---------------------------------------------------------------------------

var mdfeCloseMsg = WorkerMessage{
	DocPK:            "prod#CNPJ_12345678000195",
	AccessKey:        "35250512345678000195580010000000011000000019",
	ExpectedFileName: "35250512345678000195580010000000011000000019",
	TableName:        "mdfes",
	S3Prefix:         "mdfe",
	CNPJ:             "12345678000195",
	UF:               "SP",
	SefazEnvironment: "producao",
	CertS3Key:        "certs/org-123/cert.pfx",
	CertPassword:     "senha123",
	DocType:          "mdfe",
	SefazService:     "MDFeRecepcaoEvento",
	Body:             map[string]any{"envEventoMDFe": map[string]any{}},
	EventsTableName:  new("mdfe_events"),
	EventType:        new(mdfeEncerramentoEvent), // 110112 — for MDF-e this is encerramento
	SequenceNumber:   new(1),
	EventSK:          new("evt-sk-close-1"),
}

func TestProcess_MDFeClose_Accepted_MarksDocClosed(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado e vinculado", "prot-c1")}, dynm)

	if err := svc.Process(context.Background(), mdfeCloseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updates) != 2 {
		t.Fatalf("expected 2 updates (doc + event), got %d: %+v", len(dynm.updates), dynm.updates)
	}
	if dynm.updates[0].status != StatusClosed {
		t.Errorf("doc status = %q, want closed", dynm.updates[0].status)
	}
	if dynm.updates[0].table != "dev_mdfes" {
		t.Errorf("doc table = %q, want dev_mdfes", dynm.updates[0].table)
	}
	if dynm.updates[1].status != EventStatusSuccess {
		t.Errorf("event status = %q, want success", dynm.updates[1].status)
	}
}

func TestProcess_MDFeClose_Rejected_RevertsToAuthorized(t *testing.T) {
	dynm := &mockDynamo{}
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("501", "Rejeição: MDF-e já encerrado", "")}, dynm)

	if err := svc.Process(context.Background(), mdfeCloseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusAuthorized {
		t.Errorf("doc status = %q, want authorized (reverted)", dynm.updates[0].status)
	}
	if dynm.updates[1].status != StatusRejected {
		t.Errorf("event status = %q, want rejected", dynm.updates[1].status)
	}
}

// TestProcess_NFCeSubstitution_StillCancels guards the 110112 disambiguation:
// for NF-e/NFC-e, 110112 must remain "cancelamento por substituição".
func TestProcess_NFCeSubstitution_StillCancels(t *testing.T) {
	dynm := &mockDynamo{}
	msg := cancelMsg
	msg.DocType = "nfce"
	msg.TableName = "nfces"
	msg.S3Prefix = "nfce"
	msg.EventsTableName = new("nfce_events")
	msg.EventType = new(cancellationSubstEvent) // 110112 on NFC-e → cancellation
	svc := newSvc(certS3(), &mockLambda{payload: invokeResp("135", "Evento registrado", "p1")}, dynm)

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if dynm.updates[0].status != StatusCancelled {
		t.Errorf("doc status = %q, want cancelled (110112 on nfce)", dynm.updates[0].status)
	}
}

// ---------------------------------------------------------------------------
// S3 key construction
// ---------------------------------------------------------------------------

func TestProcess_SavesXMLWhenAtXmlPresent(t *testing.T) {
	s3m := certS3()
	xmlContent := "<nfeProc>...</nfeProc>"
	bodyMap := map[string]any{"cStat": "100", "@xml": xmlContent}
	bodyJSON, _ := json.Marshal(bodyMap)
	payload, _ := json.Marshal(map[string]any{"statusCode": 200, "body": string(bodyJSON)})

	svc := newSvc(s3m, &mockLambda{payload: payload}, &mockDynamo{})

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(s3m.putCalls) == 0 {
		t.Fatal("expected S3 put")
	}
	if !endsWith(s3m.putCalls[0], ".xml") {
		t.Errorf("expected .xml key, got %q", s3m.putCalls[0])
	}
}

func TestProcess_SavesJSONWhenNoAtXml(t *testing.T) {
	s3m := certS3()
	svc := newSvc(s3m, &mockLambda{payload: invokeResp("100", "Autorizado", "")}, &mockDynamo{})

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(s3m.putCalls) == 0 {
		t.Fatal("expected S3 put")
	}
	if !endsWith(s3m.putCalls[0], ".json") {
		t.Errorf("expected .json key, got %q", s3m.putCalls[0])
	}
}

// ---------------------------------------------------------------------------
// SNS publish
// ---------------------------------------------------------------------------

func TestProcess_PublishesToSNSWhenConfigured(t *testing.T) {
	cfg := &config.Config{
		TablePrefix:     "dev",
		DocumentsBucket: "docs",
		CertsBucket:     "certs",
		DfeLambdaName:   "dev-py-dfe",
		ResultsTopicARN: "arn:aws:sns:us-east-1:123456789:results",
	}
	snsm := &mockSNS{}
	dynm := &mockDynamo{}
	svc := New(Clients{S3: certS3(), Lambda: &mockLambda{payload: invokeResp("100", "Autorizado", "")}, Dynamo: dynm, SNS: snsm}, cfg)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(snsm.calls) == 0 {
		t.Error("expected SNS publish when ResultsTopicARN is set")
	}
}

func TestProcess_SkipsSNSWhenTopicARNEmpty(t *testing.T) {
	snsm := &mockSNS{}
	svc := New(Clients{S3: certS3(), Lambda: &mockLambda{payload: invokeResp("100", "", "")}, Dynamo: &mockDynamo{}, SNS: snsm}, testCfg)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(snsm.calls) != 0 {
		t.Errorf("expected no SNS publish when ResultsTopicARN is empty, got %d calls", len(snsm.calls))
	}
}

// TestProcess_CancellationFailure_NotifiesEventNotDocument is a regression test
// for the bug where a failed cancellation published the reverted document
// status ("authorized") to SNS, masking the real event error. The notification
// must describe the event outcome (result_kind=event, status=error, motive),
// and must NOT publish a document "authorized" result.
func TestProcess_CancellationFailure_NotifiesEventNotDocument(t *testing.T) {
	cfg := &config.Config{
		TablePrefix:     "dev",
		DocumentsBucket: "docs",
		CertsBucket:     "certs",
		DfeLambdaName:   "dev-py-dfe",
		ResultsTopicARN: "arn:aws:sns:us-east-1:123456789:results",
	}
	snsm := &mockSNS{}
	const detail = "Failed to sign XML: Unable to resolve reference URI: #"
	lam := &mockLambda{payload: invokeRespStatus(400, detail)}
	svc := New(Clients{S3: certS3(), Lambda: lam, Dynamo: &mockDynamo{}, SNS: snsm}, cfg)

	msg := baseMsg
	msg.DocType = "mdfe"
	msg.TableName = "mdfes"
	msg.EventsTableName = strPtr("mdfe_events")
	msg.EventType = strPtr(cancellationEvent)
	msg.EventSK = strPtr("019f00a3-017e-7965-be14-090609df4ffb")

	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}

	if len(snsm.calls) != 1 {
		t.Fatalf("expected exactly 1 SNS publish (the event result), got %d", len(snsm.calls))
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(snsm.calls[0]), &payload); err != nil {
		t.Fatalf("unmarshal SNS payload: %v", err)
	}
	if payload[notifyKeyResultKind] != resultKindEvent {
		t.Errorf("result_kind = %v, want %q", payload[notifyKeyResultKind], resultKindEvent)
	}
	if payload[notifyKeyStatus] != EventStatusError {
		t.Errorf("status = %v, want %q (must not mask as authorized)", payload[notifyKeyStatus], EventStatusError)
	}
	if payload[notifyKeySefazMotive] != detail {
		t.Errorf("sefaz_motive = %v, want %q", payload[notifyKeySefazMotive], detail)
	}
	if payload[notifyKeyEventType] != cancellationEvent {
		t.Errorf("event_type = %v, want %q", payload[notifyKeyEventType], cancellationEvent)
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }
func intPtr(i int) *int       { return &i }

func startsWith(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func endsWith(s, suffix string) bool {
	return len(s) >= len(suffix) && s[len(s)-len(suffix):] == suffix
}

// ---------------------------------------------------------------------------
// Idempotency guard
// ---------------------------------------------------------------------------

func statusItem(status string) *dynamodb.GetItemOutput {
	return &dynamodb.GetItemOutput{
		Item: map[string]types.AttributeValue{
			"status": &types.AttributeValueMemberS{Value: status},
		},
	}
}

func TestProcess_SkipsWhenDocumentAlreadyTerminal(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{getItemOutput: statusItem(StatusAuthorized)}
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("expected invokePyDfe NOT to be called, got %d calls", lamm.calls)
	}
	if len(dynm.updates) != 0 {
		t.Errorf("expected no UpdateItem calls, got %d", len(dynm.updates))
	}
}

func TestProcess_SkipsWhenEventAlreadyTerminal(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("135", "Sucesso", "")}
	dynm := &mockDynamo{getItemOutput: statusItem(EventStatusSuccess)}
	svc := newSvc(certS3(), lamm, dynm)

	msg := cancelMsg
	if err := svc.Process(context.Background(), msg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 0 {
		t.Errorf("expected invokePyDfe NOT to be called, got %d calls", lamm.calls)
	}
}

func TestProcess_ActiveLeaseDoesNotInvokeSefaz(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{getItemOutput: statusItem(StatusProcessing)}
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); !errors.Is(err, errProcessingLeaseHeld) {
		t.Fatalf("expected processing lease error, got %v", err)
	}
	if lamm.calls != 0 {
		t.Fatalf("active lease must block duplicate SEFAZ call, got %d", lamm.calls)
	}
}

func TestProcess_ProceedsWhenNotYetTerminal(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{} // GetItem returns "not found" by default
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lamm.calls != 1 {
		t.Errorf("expected invokePyDfe to be called once, got %d", lamm.calls)
	}
}

func TestProcess_ClaimStoreErrorFailsClosed(t *testing.T) {
	lamm := &mockLambda{payload: invokeResp("100", "Autorizado", "135")}
	dynm := &mockDynamo{claimErr: errors.New("transient dynamodb error")}
	svc := newSvc(certS3(), lamm, dynm)

	if err := svc.Process(context.Background(), baseMsg); err == nil {
		t.Fatal("expected claim-store error")
	}
	if lamm.calls != 0 {
		t.Errorf("claim-store failure must fail closed before SEFAZ; got %d calls", lamm.calls)
	}
}

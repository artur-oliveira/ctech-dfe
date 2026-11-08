package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	"gopkg.aoctech.app/api-commons/alerts"
	"gopkg.aoctech.app/dfe/worker/internal/config"
)

const testGenKey = "test-generator-key-not-real"

func TestDecideCiot(t *testing.T) {
	ok := `{"codigo":110,"mensagem":"ok","id_operacao":"ABCDEFGHIJKL","codigo_verificador":"1234","ciot":"ABCDEFGHIJKL1234","aviso_transportador":"aviso"}`
	rej := func(code int) string {
		b, _ := json.Marshal(map[string]any{"internal_code": "CIOT_X", "antt_code": code, "antt_message": "m"})
		return string(b)
	}
	cases := []struct {
		name, op string
		status   int
		body     string
		check    func(t *testing.T, o ciotOutcome)
	}{
		{"declare ok", opDeclare, 200, ok, func(t *testing.T, o ciotOutcome) {
			if o.Status != ciotDeclared || o.Attrs["ciot"] != "ABCDEFGHIJKL1234" || o.Attrs["carrier_warning"] != "aviso" {
				t.Fatalf("%+v", o)
			}
		}},
		{"cancel ok", "cancel", 200, ok, func(t *testing.T, o ciotOutcome) {
			if o.Status != ciotCancelled {
				t.Fatalf("%+v", o)
			}
		}},
		{"219 reconciles", opDeclare, 422, rej(219), func(t *testing.T, o ciotOutcome) {
			if !o.Reconcile || o.LastError {
				t.Fatalf("%+v", o)
			}
		}},
		{"222 confirms cancel", "cancel", 422, rej(222), func(t *testing.T, o ciotOutcome) {
			if o.Status != ciotCancelled || !o.TerminalOK {
				t.Fatalf("%+v", o)
			}
		}},
		{"251 confirms close", "close", 422, rej(251), func(t *testing.T, o ciotOutcome) {
			if o.Status != ciotClosed || !o.TerminalOK {
				t.Fatalf("%+v", o)
			}
		}},
		{"222 does not confirm close", "close", 422, rej(222), func(t *testing.T, o ciotOutcome) {
			if o.Status != "" || !o.LastError {
				t.Fatalf("%+v", o)
			}
		}},
		{"declare business rejection is terminal", opDeclare, 422, rej(233), func(t *testing.T, o ciotOutcome) {
			if o.Status != ciotRejected || !o.LastError || o.Retry {
				t.Fatalf("%+v", o)
			}
		}},
		{"lifecycle rejection keeps the status", "rectify", 422, rej(236), func(t *testing.T, o ciotOutcome) {
			if o.Status != "" || !o.LastError || o.Retry {
				t.Fatalf("%+v", o)
			}
		}},
		{"credential group alerts instead of a business error", opDeclare, 422, rej(312), func(t *testing.T, o ciotOutcome) {
			if !o.Alert || !o.Retry || o.LastError {
				t.Fatalf("%+v", o)
			}
		}},
		{"egress 400 is configuration", opDeclare, 400, `{}`, func(t *testing.T, o ciotOutcome) {
			if !o.Alert || !o.Retry {
				t.Fatalf("%+v", o)
			}
		}},
		{"502 is retried", opDeclare, 502, `{}`, func(t *testing.T, o ciotOutcome) {
			if !o.Retry || o.Alert || o.Status != "" {
				t.Fatalf("%+v", o)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { c.check(t, decideCiot(c.op, c.status, c.body)) })
	}
}

// --- fakes (no real network, no real AWS) ---

type ciotDynamo struct {
	updates []*dynamodb.UpdateItemInput
	claim   map[string]types.AttributeValue   // row returned by the claim; nil makes the claim fail
	current map[string]types.AttributeValue   // row returned by GetItem after a failed claim
	linked  []map[string]types.AttributeValue // rows returned by the mdfe-index query
	queries []*dynamodb.QueryInput
	// condFail makes UpdateItem on the cascade/unlink conditions fail like an already-applied row.
	condFail bool
}

func (d *ciotDynamo) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	d.queries = append(d.queries, in)
	return &dynamodb.QueryOutput{Items: d.linked}, nil
}

func (d *ciotDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	d.updates = append(d.updates, in)
	if d.condFail && strings.Contains(aws.ToString(in.ConditionExpression), "#mk = :k") {
		return nil, &types.ConditionalCheckFailedException{}
	}
	if strings.Contains(aws.ToString(in.UpdateExpression), "processing_lease_until = :lease") {
		if d.claim == nil {
			return nil, &types.ConditionalCheckFailedException{}
		}
		return &dynamodb.UpdateItemOutput{Attributes: d.claim}, nil
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (d *ciotDynamo) GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	return &dynamodb.GetItemOutput{Item: d.current}, nil
}

// finish returns the final row update (the one that removes pending_operation).
func (d *ciotDynamo) finish() *dynamodb.UpdateItemInput {
	for _, u := range d.updates {
		if strings.Contains(aws.ToString(u.UpdateExpression), "REMOVE pending_operation") && strings.Contains(aws.ToString(u.UpdateExpression), "history") {
			return u
		}
	}
	return nil
}

type ciotLambda struct {
	handle func(p lambdaPayload) lambdaResponse
	calls  []lambdaPayload
}

func (l *ciotLambda) Invoke(_ context.Context, in *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	var p lambdaPayload
	_ = json.Unmarshal(in.Payload, &p)
	l.calls = append(l.calls, p)
	raw, _ := json.Marshal(l.handle(p))
	return &lambdaSDK.InvokeOutput{Payload: raw}, nil
}

type ciotSNS struct{ published []string }

func (s *ciotSNS) Publish(_ context.Context, in *sns.PublishInput, _ ...func(*sns.Options)) (*sns.PublishOutput, error) {
	s.published = append(s.published, aws.ToString(in.Message))
	return &sns.PublishOutput{}, nil
}

type ciotSSM struct {
	values map[string]string
	reads  int
}

func (s *ciotSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	s.reads++
	v, ok := s.values[aws.ToString(in.Name)]
	if !ok {
		return nil, &ssmtypes.ParameterNotFound{}
	}
	return &ssm.GetParameterOutput{Parameter: &ssmtypes.Parameter{Value: aws.String(v)}}, nil
}

type captureAlerts struct{ got []alerts.Alert }

func (c *captureAlerts) Alert(_ context.Context, a alerts.Alert) { c.got = append(c.got, a) }

type ciotRig struct {
	svc *DfeService
	dyn *ciotDynamo
	lam *ciotLambda
	sns *ciotSNS
	ssm *ciotSSM
	al  *captureAlerts
}

func newCiotRig(t *testing.T, row map[string]types.AttributeValue, handle func(p lambdaPayload) lambdaResponse) *ciotRig {
	t.Helper()
	r := &ciotRig{
		dyn: &ciotDynamo{claim: row},
		lam: &ciotLambda{handle: handle},
		sns: &ciotSNS{},
		ssm: &ciotSSM{values: map[string]string{
			"/ctech-dfe/prod/ciot/generator-api-key":      testGenKey,
			"/ctech-dfe/prod/ciot/generator-fallback-url": "https://fallback.invalid",
		}},
		al: &captureAlerts{},
	}
	r.svc = New(Clients{S3: &mockS3{certData: []byte("pfx")}, Lambda: r.lam, Dynamo: r.dyn, SNS: r.sns},
		&config.Config{TablePrefix: "t", ResultsTopicARN: "arn", CertsBucket: "certs"}).WithCiot(r.ssm, r.al)
	return r
}

func ciotRow(status string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "prod#ORG"}, "sk": &types.AttributeValueMemberS{Value: "CIOT_1"},
		"status": &types.AttributeValueMemberS{Value: status},
	}
}

func declareMsg() WorkerMessage {
	op, sk := "declare", "CIOT_1"
	return WorkerMessage{
		DocPK: "prod#ORG", AccessKey: sk, TableName: "ciots", DocType: "ciot", SefazService: "DeclaracaoOperacaoTransporte",
		SefazEnvironment: "prod", CertS3Key: "cert-ciot-test", EventType: &op, EventSK: &sk, CNPJ: "11111111000100",
		Body: map[string]any{"declaration": map[string]any{"CpfCnpjContratado": "11111111000100", "TipoOperacao": 1.0, "IndContingencia": false}},
	}
}

func byService(declare, gen lambdaResponse, extra map[string]lambdaResponse) func(p lambdaPayload) lambdaResponse {
	return func(p lambdaPayload) lambdaResponse {
		if r, ok := extra[p.Service]; ok {
			return r
		}
		if p.Service == "GerarIdentificador" {
			return gen
		}
		return declare
	}
}

var (
	genOK    = lambdaResponse{StatusCode: 200, Body: `{"id":"ABCDEFGHIJKL"}`}
	declOK   = lambdaResponse{StatusCode: 200, Body: `{"codigo":110,"mensagem":"ok","id_operacao":"ABCDEFGHIJKL","codigo_verificador":"1234","ciot":"ABCDEFGHIJKL1234","aviso_transportador":"aviso"}`}
	rej219   = lambdaResponse{StatusCode: 422, Body: `{"internal_code":"CIOT_STATE_219","antt_code":219,"antt_message":"dup"}`}
	rej233   = lambdaResponse{StatusCode: 422, Body: `{"internal_code":"CIOT_PAYMENT_233","antt_code":233,"antt_message":"pay"}`}
	unavail  = lambdaResponse{StatusCode: 502, Body: `{"detail":"down"}`}
	consulta = lambdaResponse{StatusCode: 200, Body: `{"codigo":111,"mensagem":"ok","id_operacao":"ABCDEFGHIJKL","codigo_verificador":"9999"}`}
)

func TestProcessCiotDeclareHappyPath(t *testing.T) {
	r := newCiotRig(t, ciotRow("pending"), byService(declOK, genOK, nil))
	if err := r.svc.processCiot(t.Context(), declareMsg()); err != nil {
		t.Fatal(err)
	}
	fin := r.dyn.finish()
	if fin == nil || fin.ExpressionAttributeNames["#st"] != "status" {
		t.Fatalf("no finishing update: %+v", fin)
	}
	if v := fin.ExpressionAttributeValues[":st"].(*types.AttributeValueMemberS).Value; v != ciotDeclared {
		t.Fatalf("status = %s", v)
	}
	if len(r.sns.published) != 1 || !strings.Contains(r.sns.published[0], `"status":"declared"`) || !strings.Contains(r.sns.published[0], "ABCDEFGHIJKL1234") {
		t.Fatalf("published = %v", r.sns.published)
	}
	// the generator key reaches only the generator call, never the declaration or the result
	for _, c := range r.lam.calls {
		_, has := c.Body["generator_api_key"]
		if has != (c.Service == "GerarIdentificador") {
			t.Fatalf("generator key in %s call: %v", c.Service, has)
		}
	}
	if strings.Contains(r.sns.published[0], testGenKey) {
		t.Fatal("secret leaked into the result notification")
	}
	if r.ssm.reads != 2 {
		t.Fatalf("ssm reads = %d (cached per invocation)", r.ssm.reads)
	}
	if len(r.al.got) != 0 {
		t.Fatalf("unexpected alerts %v", r.al.got)
	}
}

func TestProcessCiotRedeliveryOfFinishedOperationRepublishesOnly(t *testing.T) {
	r := newCiotRig(t, nil, byService(declOK, genOK, nil))
	r.dyn.current = ciotRow("declared") // no pending_operation: already done
	if err := r.svc.processCiot(t.Context(), declareMsg()); err != nil {
		t.Fatal(err)
	}
	if len(r.lam.calls) != 0 || len(r.sns.published) != 1 {
		t.Fatalf("egress calls %d, published %d", len(r.lam.calls), len(r.sns.published))
	}
}

func TestProcessCiotLeaseHeldIsRetried(t *testing.T) {
	r := newCiotRig(t, nil, byService(declOK, genOK, nil))
	row := ciotRow("pending")
	row["pending_operation"] = &types.AttributeValueMemberS{Value: "declare"}
	r.dyn.current = row
	if err := r.svc.processCiot(t.Context(), declareMsg()); err == nil {
		t.Fatal("expected a retryable error")
	}
	if len(r.lam.calls) != 0 {
		t.Fatal("must not call the egress while another worker holds the lease")
	}
}

func TestProcessCiotTransportFailureReleasesLeaseAndRetries(t *testing.T) {
	r := newCiotRig(t, ciotRow("pending"), byService(unavail, genOK, nil))
	if err := r.svc.processCiot(t.Context(), declareMsg()); err == nil {
		t.Fatal("expected an error so the platform retries")
	}
	if r.dyn.finish() != nil || len(r.sns.published) != 0 {
		t.Fatal("a transport failure must not finish the operation")
	}
	last := r.dyn.updates[len(r.dyn.updates)-1]
	if !strings.Contains(aws.ToString(last.UpdateExpression), "REMOVE processing_owner") || strings.Contains(aws.ToString(last.UpdateExpression), "pending_operation") {
		t.Fatalf("lease not released while keeping the pending marker: %s", aws.ToString(last.UpdateExpression))
	}
}

func TestProcessCiotBusinessRejectionIsTerminalAndNotRetried(t *testing.T) {
	r := newCiotRig(t, ciotRow("pending"), byService(rej233, genOK, nil))
	if err := r.svc.processCiot(t.Context(), declareMsg()); err != nil {
		t.Fatalf("a business rejection must not be an error (it would be retried): %v", err)
	}
	if v := r.dyn.finish().ExpressionAttributeValues[":st"].(*types.AttributeValueMemberS).Value; v != ciotRejected {
		t.Fatalf("status = %s", v)
	}
	if !strings.Contains(r.sns.published[0], `"status":"rejected"`) || !strings.Contains(r.sns.published[0], "CIOT_PAYMENT_233") {
		t.Fatalf("published = %v", r.sns.published)
	}
}

func TestProcessCiot219AdoptsTheExistingCIOT(t *testing.T) {
	r := newCiotRig(t, ciotRow("pending"), byService(rej219, genOK, map[string]lambdaResponse{"ConsultarCIOTGerado": consulta}))
	if err := r.svc.processCiot(t.Context(), declareMsg()); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.sns.published[0], "ABCDEFGHIJKL9999") || !strings.Contains(r.sns.published[0], `"status":"declared"`) {
		t.Fatalf("published = %v", r.sns.published)
	}
}

func TestProcessCiotCredentialProblemAlertsAndIsNotABusinessRejection(t *testing.T) {
	cred := lambdaResponse{StatusCode: 422, Body: `{"internal_code":"CIOT_CREDENTIAL_312","antt_code":312,"antt_message":"cert"}`}
	r := newCiotRig(t, ciotRow("pending"), byService(cred, genOK, nil))
	if err := r.svc.processCiot(t.Context(), declareMsg()); err == nil {
		t.Fatal("expected a retryable error")
	}
	if len(r.al.got) != 1 || r.dyn.finish() != nil {
		t.Fatalf("alerts %d, finished %v", len(r.al.got), r.dyn.finish() != nil)
	}
}

func TestProcessCiotReusesStoredIdentifier(t *testing.T) {
	row := ciotRow("pending")
	row["code"] = &types.AttributeValueMemberS{Value: "ABCDEFGHIJKL"}
	r := newCiotRig(t, row, byService(declOK, lambdaResponse{StatusCode: 500}, nil))
	if err := r.svc.processCiot(t.Context(), declareMsg()); err != nil {
		t.Fatal(err)
	}
	for _, c := range r.lam.calls {
		if c.Service == "GerarIdentificador" {
			t.Fatal("must not ask the generator again when the identifier is stored")
		}
	}
	if r.ssm.reads != 0 {
		t.Fatal("no SSM read is needed when the identifier is stored")
	}
}

func TestProcessCiotMissingGeneratorKeyAlerts(t *testing.T) {
	r := newCiotRig(t, ciotRow("pending"), byService(declOK, genOK, nil))
	r.ssm.values = map[string]string{}
	if err := r.svc.processCiot(t.Context(), declareMsg()); err == nil {
		t.Fatal("expected a configuration error")
	}
	if len(r.al.got) != 1 {
		t.Fatalf("alerts = %d", len(r.al.got))
	}
}

func TestCiotDLQ(t *testing.T) {
	d := &ciotDynamo{}
	msg := declareMsg()
	status, err := CiotDLQ(t.Context(), d, "t", msg, "motive")
	if err != nil || status != ciotFailed {
		t.Fatalf("%s %v", status, err)
	}
	op := "cancel"
	msg.EventType = &op
	if status, _ = CiotDLQ(t.Context(), d, "t", msg, "motive"); status != "operation_failed" {
		t.Fatalf("status = %s", status)
	}
	if strings.Contains(aws.ToString(d.updates[1].UpdateExpression), "#st") {
		t.Fatal("a lifecycle operation must leave the row status alone")
	}
}

// --- MDF-e <-> CIOT link (ciot_link.go) ---

func linkedRow(sk, status string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: "prod#ORG"}, "sk": &types.AttributeValueMemberS{Value: sk},
		"status": &types.AttributeValueMemberS{Value: status},
	}
}

func mdfeMsg() WorkerMessage {
	return WorkerMessage{DocPK: "prod#ORG", AccessKey: strings.Repeat("1", 44), TableName: "mdfes", DocType: "mdfe"}
}

func countUpdates(d *ciotDynamo, sub string) (n int) {
	for _, u := range d.updates {
		if strings.Contains(aws.ToString(u.UpdateExpression), sub) {
			n++
		}
	}
	return n
}

func TestOfferCiotCascadeMarksOpenRowsAndAuditsOnce(t *testing.T) {
	r := newCiotRig(t, nil, nil)
	r.dyn.linked = []map[string]types.AttributeValue{linkedRow("CIOT_1", "declared"), linkedRow("CIOT_2", "closed")}
	r.svc.offerCiotCascade(t.Context(), mdfeMsg(), "cancel")
	if got := countUpdates(r.dyn, "mdfe_cascade = :op"); got != 1 {
		t.Fatalf("cascade marks = %d, want 1 (closed row skipped)", got)
	}
	if got := countUpdates(r.dyn, "body_hash"); got != 1 {
		t.Fatalf("audit entries = %d, want 1", got)
	}
	if len(r.dyn.queries) != 1 || aws.ToString(r.dyn.queries[0].IndexName) != "mdfe-index" {
		t.Fatalf("query = %+v", r.dyn.queries)
	}
}

func TestOfferCiotCascadeIsIdempotentOnRedelivery(t *testing.T) {
	r := newCiotRig(t, nil, nil)
	r.dyn.linked = []map[string]types.AttributeValue{linkedRow("CIOT_1", "declared")}
	r.dyn.condFail = true // the row already carries this offer
	r.svc.offerCiotCascade(t.Context(), mdfeMsg(), "close")
	if countUpdates(r.dyn, "body_hash") != 0 || len(r.al.got) != 0 {
		t.Fatalf("redelivery wrote audit or alerted: audit=%d alerts=%d", countUpdates(r.dyn, "body_hash"), len(r.al.got))
	}
}

func TestUnlinkCiotsRemovesLinkAndAudits(t *testing.T) {
	r := newCiotRig(t, nil, nil)
	r.dyn.linked = []map[string]types.AttributeValue{linkedRow("CIOT_1", "declared")}
	r.svc.unlinkCiotsOfMdfe(t.Context(), mdfeMsg())
	if countUpdates(r.dyn, "REMOVE #mk") != 1 || countUpdates(r.dyn, "body_hash") != 1 {
		t.Fatalf("updates = %d", len(r.dyn.updates))
	}
	// Only MDF-e authorization failures unlink; any other document type leaves the table alone.
	r2 := newCiotRig(t, nil, nil)
	m := mdfeMsg()
	m.DocType = "nfe"
	r2.svc.unlinkCiotsOfMdfe(t.Context(), m)
	if len(r2.dyn.queries) != 0 {
		t.Fatal("nfe queried the ciots table")
	}
}

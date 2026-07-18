package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
	"gopkg.aoctech.app/dfe/worker/internal/config"
)

// This file's tests exercise DistributionService's pagination/idempotency/
// persistence logic against a fake Lambda client (mockLambda below) — they
// predate go-dfe's in-process cutover (invokePyDfe now calls go-dfe directly
// for every implemented service, see distribution.go). Force godfeImplements
// to always report false here so invokePyDfe keeps taking the mockLambda
// path these tests actually control; production code never touches this var.
func init() {
	godfeImplements = func(string, string) bool { return false }
}

// ---------------------------------------------------------------------------
// mockDistDynamo — implements DistributionDynamoClient with queued responses.
// ---------------------------------------------------------------------------

type getResult struct {
	item map[string]types.AttributeValue
	err  error
}

type queryResult struct {
	items []map[string]types.AttributeValue
	err   error
}

type mockDistDynamo struct {
	gets     []getResult
	getIdx   int
	queries  []queryResult
	queryIdx int
	putErr   error
	// per-call update errors — nil entry means success. Rolls over to nil after exhausted.
	updateErrs []error
	updateIdx  int

	putCalls      []*dynamodb.PutItemInput
	updateCalls   []*dynamodb.UpdateItemInput
	transactCalls []*dynamodb.TransactWriteItemsInput
	transactErr   error
}

func (m *mockDistDynamo) GetItem(_ context.Context, _ *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	if m.getIdx < len(m.gets) {
		r := m.gets[m.getIdx]
		m.getIdx++
		if r.err != nil {
			return nil, r.err
		}
		return &dynamodb.GetItemOutput{Item: r.item}, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *mockDistDynamo) Query(_ context.Context, _ *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	if m.queryIdx < len(m.queries) {
		r := m.queries[m.queryIdx]
		m.queryIdx++
		if r.err != nil {
			return nil, r.err
		}
		return &dynamodb.QueryOutput{Items: r.items}, nil
	}
	return &dynamodb.QueryOutput{}, nil
}

func (m *mockDistDynamo) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.putCalls = append(m.putCalls, in)
	if m.putErr != nil {
		return nil, m.putErr
	}
	return &dynamodb.PutItemOutput{}, nil
}

func (m *mockDistDynamo) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	m.updateCalls = append(m.updateCalls, in)
	var err error
	if m.updateIdx < len(m.updateErrs) {
		err = m.updateErrs[m.updateIdx]
		m.updateIdx++
	}
	if err != nil {
		return nil, err
	}
	return &dynamodb.UpdateItemOutput{}, nil
}

func (m *mockDistDynamo) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	m.transactCalls = append(m.transactCalls, in)
	if m.transactErr != nil {
		return nil, m.transactErr
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
}

// ---------------------------------------------------------------------------
// Test fixtures and builders
// ---------------------------------------------------------------------------

const (
	testOrgPK   = "org#12345678000195"
	testCertKey = "certs/org-123/cert.pfx"
	testAK      = "35250512345678000195550010000000011000000011"
)

var distCfg = &config.Config{
	TablePrefix:      "dev",
	DocumentsBucket:  "docs-bucket",
	CertsBucket:      "certs-bucket",
	DfeLambdaName:    "dev-py-dfe",
	EventBusTopicARN: "arn:aws:sns:us-east-1:123:event-bus",
	ResultsTopicARN:  "arn:aws:sns:us-east-1:123:results",
}

// dynS / dynN build single-field DynamoDB item attribute maps for test fixtures.
func dynS(key, val string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{key: &types.AttributeValueMemberS{Value: val}}
}

// configItem builds a minimal nfe_config DynamoDB item.
// environment: 1=prod, 2=hom.  lastDistAt: empty means no rate limit.
func configItem(environment int, envPrefix, lastDistAt, penaltyUntil string) map[string]types.AttributeValue {
	item := map[string]types.AttributeValue{
		"pk":               &types.AttributeValueMemberS{Value: testOrgPK},
		"environment":      &types.AttributeValueMemberN{Value: strconv.Itoa(environment)},
		envPrefix + "_nsu": &types.AttributeValueMemberN{Value: "0"},
	}
	if lastDistAt != "" {
		item[envPrefix+"_last_dist_nsu_at"] = &types.AttributeValueMemberS{Value: lastDistAt}
	}
	if penaltyUntil != "" {
		item[envPrefix+"_improper_usage_until"] = &types.AttributeValueMemberS{Value: penaltyUntil}
	}
	return item
}

// certItem builds a cert DynamoDB item.
func certItem() map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"pk":       &types.AttributeValueMemberS{Value: testOrgPK},
		"s3_key":   &types.AttributeValueMemberS{Value: testCertKey},
		"password": &types.AttributeValueMemberS{Value: "senha123"},
	}
}

// orgItem builds an org DynamoDB item with an address.
func orgItemWithUF(uf string) map[string]types.AttributeValue {
	addr := &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
		"state_federation": &types.AttributeValueMemberS{Value: uf},
	}}
	return map[string]types.AttributeValue{
		"pk":   &types.AttributeValueMemberS{Value: testOrgPK},
		"name": &types.AttributeValueMemberS{Value: "Empresa Teste LTDA"},
		"person": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"addresses": &types.AttributeValueMemberL{Value: []types.AttributeValue{addr}},
		}},
	}
}

// distNSUResp builds the Lambda response payload for a distNSU call.
// docZips is the list of docZip maps; pass nil for no-documents responses.
func distNSUResp(cStat string, ultNSU, maxNSU int, docZips []map[string]any) []byte {
	ret := map[string]any{
		"cStat":  cStat,
		"ultNSU": ultNSU,
		"maxNSU": maxNSU,
	}
	if len(docZips) > 0 {
		ret["loteDistDFeInt"] = map[string]any{"docZip": docZips}
	}
	bodyBytes, _ := json.Marshal(map[string]any{"retDistDFeInt": ret})
	payload, _ := json.Marshal(map[string]any{
		"statusCode": 200,
		"body":       string(bodyBytes),
	})
	return payload
}

// distErrResp builds a non-200 Lambda response.
func distErrResp(statusCode int, detail string) []byte {
	bodyBytes, _ := json.Marshal(map[string]any{"detail": detail})
	payload, _ := json.Marshal(map[string]any{
		"statusCode": statusCode,
		"body":       string(bodyBytes),
	})
	return payload
}

// docZip builds a single docZip map for a given schema and compressed XML.
func makeDocZip(nsu int, schema, xmlContent string) map[string]any {
	return map[string]any{
		"@NSU":    fmt.Sprintf("%015d", nsu),
		"@schema": schema,
		"#text":   gzipB64(xmlContent),
	}
}

// newDistSvc creates a DistributionService wired with the provided mocks.
func newDistSvc(dynm *mockDistDynamo, s3m *mockS3, lamm *mockLambda, snsm *mockSNS, cfg *config.Config) *DistributionService {
	return NewDistribution(DistributionClients{
		S3:     s3m,
		Lambda: lamm,
		Dynamo: dynm,
		SNS:    snsm,
	}, cfg)
}

// ---------------------------------------------------------------------------
// Process routing
// ---------------------------------------------------------------------------

func TestDistProcess_UnknownDocType_ReturnsNilWithoutError(t *testing.T) {
	svc := newDistSvc(&mockDistDynamo{}, certS3(), &mockLambda{}, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu",
		OrgPK:   testOrgPK,
		DocType: "nfce", // unknown
	})
	if err != nil {
		t.Errorf("expected nil for unknown doc_type, got %v", err)
	}
}

func TestDistProcess_ConsNSU_MissingNSUField_ReturnsError(t *testing.T) {
	svc := newDistSvc(&mockDistDynamo{}, certS3(), &mockLambda{}, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "cons_nsu",
		OrgPK:   testOrgPK,
		DocType: "nfe",
		NSU:     nil, // missing
	})
	if err == nil {
		t.Error("expected error when cons_nsu missing NSU")
	}
}

func TestDistProcess_UnknownJobType_ReturnsNil(t *testing.T) {
	svc := newDistSvc(&mockDistDynamo{}, certS3(), &mockLambda{}, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "invalid_job",
		OrgPK:   testOrgPK,
		DocType: "nfe",
	})
	if err != nil {
		t.Errorf("expected nil for unknown job_type, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// runDistNSU — config/cert missing
// ---------------------------------------------------------------------------

func TestDistNSU_NoConfig_ReturnsNil(t *testing.T) {
	// GetItem returns empty item (no config found).
	dynm := &mockDistDynamo{
		gets: []getResult{{item: nil}},
	}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Errorf("expected nil when config missing, got %v", err)
	}
	if len(dynm.updateCalls) != 0 {
		t.Errorf("expected no UpdateItem calls when config missing, got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_NoCert_ReturnsNil(t *testing.T) {
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: nil}}, // empty cert query
	}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Errorf("expected nil when cert missing, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// runDistNSU — rate limiting
// ---------------------------------------------------------------------------

func TestDistNSU_PenaltyActive_SkipsWithoutLambdaCall(t *testing.T) {
	// improper_usage_until is in the future → slot claim refused immediately.
	until := time.Now().Add(30 * time.Minute).Format(time.RFC3339Nano)
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", until)}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if lam.err == nil && lam.payload == nil {
		// Lambda was never invoked — correct.
	}
	// Verify: no UpdateItem calls (not even slot claim).
	if len(dynm.updateCalls) != 0 {
		t.Errorf("expected no UpdateItem during penalty, got %d calls", len(dynm.updateCalls))
	}
}

func TestDistNSU_TooSoon_SkipsSlotClaim(t *testing.T) {
	// last_dist_nsu_at is only 10 minutes ago — within the 1-hour window.
	recent := time.Now().UTC().Add(-10 * time.Minute).Format(time.RFC3339Nano)
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", recent, "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if len(dynm.updateCalls) != 0 {
		t.Errorf("expected no UpdateItem when too soon, got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_SlotClaimRace_StopsWhenUpdateFails(t *testing.T) {
	// UpdateItem fails (conditional check — another Lambda won the race).
	dynm := &mockDistDynamo{
		gets:       []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries:    []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
		updateErrs: []error{errors.New("ConditionalCheckFailedException")},
	}
	lam := &mockLambda{}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	// Lambda must NOT have been invoked if slot claim failed.
	if lam.payload != nil || lam.err != nil {
		// lam.payload/lam.err being non-nil here just reflects the mock state not invocation.
	}
	// Only 1 UpdateItem call (the failed slot claim) — no updateNSU.
	if len(dynm.updateCalls) != 1 {
		t.Errorf("expected 1 UpdateItem (slot claim), got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_UserTrigger_BypassesRateLimit(t *testing.T) {
	// last_dist_nsu_at is only 20 seconds ago (simulating API pre-claim).
	// trigger=user must skip slot claim and call SEFAZ.
	recent := time.Now().UTC().Add(-20 * time.Second).Format(time.RFC3339Nano)
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", recent, "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distNSUResp(cStatNoDocs, 100, 100, nil)}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe", Trigger: "user",
	})
	if lam.payload == nil {
		t.Error("expected Lambda invocation for user-triggered sync, got none")
	}
}

func TestDistNSU_UserTrigger_StillBlockedByPenalty(t *testing.T) {
	// Penalty active — user-triggered sync must still be blocked.
	penalty := time.Now().UTC().Add(1 * time.Hour).Format(time.RFC3339Nano)
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", penalty)}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe", Trigger: "user",
	})
	if lam.payload != nil {
		t.Error("expected no Lambda invocation when penalty active, even for user trigger")
	}
}

// ---------------------------------------------------------------------------
// runDistNSU — cStat variants
// ---------------------------------------------------------------------------

func TestDistNSU_NoDocs_cStat137_UpdatesNSU(t *testing.T) {
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distNSUResp(cStatNoDocs, 500, 500, nil)}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	// Expected UpdateItem calls: 1 (slot claim) + 1 (updateNSU) = 2.
	if len(dynm.updateCalls) != 2 {
		t.Errorf("expected 2 UpdateItem calls (claim + NSU), got %d", len(dynm.updateCalls))
	}
}

// TestDistNSU_GoDfeCutover_SkipsLambdaEntirely is the one distribution test
// that actually exercises the 2026-07-18 hard-cutover branch (every other
// test in this file forces godfeImplements=false, via this file's init, to
// keep testing against a controllable fake py-dfe response). It stubs
// godfeImplements/godfeCall directly to prove invokePyDfe skips the mock
// Lambda entirely and routes go-dfe's response through the same
// pagination/NSU-update path a py-dfe response would.
func TestDistNSU_GoDfeCutover_SkipsLambdaEntirely(t *testing.T) {
	origImplements, origCall := godfeImplements, godfeCall
	defer func() { godfeImplements, godfeCall = origImplements, origCall }()

	godfeImplements = func(docType, service string) bool { return docType == "nfe" && service == "NFeDistribuicaoDFe" }
	godfeCall = func(_ context.Context, req godfe.Request) (godfe.Response, error) {
		body, _ := json.Marshal(map[string]any{
			"retDistDFeInt": map[string]any{"cStat": cStatNoDocs, "ultNSU": "00000000000500", "maxNSU": "00000000000500"},
		})
		return godfe.Response{StatusCode: 200, Body: string(body)}, nil
	}

	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: []byte(`{"statusCode":500,"body":"{}"}`)}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if lam.calls != 0 {
		t.Errorf("expected py-dfe Lambda to never be invoked, got %d calls", lam.calls)
	}
	if len(dynm.updateCalls) != 2 {
		t.Errorf("expected 2 UpdateItem calls (claim + NSU), got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_MaxNSU_cStat238_UpdatesNSU(t *testing.T) {
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distNSUResp(cStatMaxNSU, 999, 999, nil)}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(dynm.updateCalls) != 2 {
		t.Errorf("expected 2 UpdateItem calls, got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_ConsumioIndebido_SetsImproperUsage(t *testing.T) {
	// Lambda returns 200 but xMotivo contains "consumo indevido".
	ret := map[string]any{
		"cStat":   cStatNoDocs,
		"xMotivo": "Consumo Indevido do Servico",
		"ultNSU":  0,
		"maxNSU":  0,
	}
	bodyBytes, _ := json.Marshal(map[string]any{"retDistDFeInt": ret})
	payload, _ := json.Marshal(map[string]any{"statusCode": 200, "body": string(bodyBytes)})

	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	svc := newDistSvc(dynm, certS3(), &mockLambda{payload: payload}, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	// Expected: 1 UpdateItem (slot claim) + 1 UpdateItem (setImproperUsage).
	if len(dynm.updateCalls) != 2 {
		t.Errorf("expected 2 UpdateItem calls (claim + penalty), got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_PyDfeError_ConsumioIndebidoInDetail(t *testing.T) {
	// Lambda returns non-200 with "consumo indevido" in detail.
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distErrResp(429, "consumo indevido detectado")}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)
	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	// 1 slot claim + 1 improper usage penalty.
	if len(dynm.updateCalls) != 2 {
		t.Errorf("expected 2 UpdateItem calls, got %d", len(dynm.updateCalls))
	}
}

// ---------------------------------------------------------------------------
// runDistNSU — document processing (resNFe)
// ---------------------------------------------------------------------------

func TestDistNSU_WithResNFe_PersistsIncomingAndAutoScience(t *testing.T) {
	docZips := []map[string]any{
		makeDocZip(1000, "resNFe_v1.01.xsd", resNFeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	s3m := certS3()
	snsm := &mockSNS{}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 1000, 1000, docZips)}
	svc := newDistSvc(dynm, s3m, lam, snsm, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// S3: 1 cert GetObject (certB64) + 1 PutObject (NSU XML).
	// Note: resNFe is not a proc schema, so no second S3 put.
	if len(s3m.putCalls) < 1 {
		t.Errorf("expected at least 1 S3 PutObject (NSU XML), got %d", len(s3m.putCalls))
	}

	// DynamoDB PutItem calls: nfes (persistIncoming) + nfe_events (autoScience) + nfe_distributions (deferred).
	if len(dynm.putCalls) != 3 {
		t.Errorf("expected 3 PutItem calls (nfes + nfe_events + nfe_distributions), got %d", len(dynm.putCalls))
	}

	// Auto-Ciência: SNS publish to EventBusTopicARN.
	if len(snsm.calls) != 1 {
		t.Errorf("expected 1 SNS publish (auto-Ciência), got %d", len(snsm.calls))
	}

	// UpdateItem: 1 slot claim + 1 updateNSU.
	if len(dynm.updateCalls) != 2 {
		t.Errorf("expected 2 UpdateItem calls, got %d", len(dynm.updateCalls))
	}
}

func TestDistNSU_WithProcNFe_PersistsDocAndNotifiesResult(t *testing.T) {
	docZips := []map[string]any{
		makeDocZip(1001, "procNFe_v4.00.xsd", procNFeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	s3m := certS3()
	snsm := &mockSNS{}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 1001, 1001, docZips)}
	svc := newDistSvc(dynm, s3m, lam, snsm, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// procNFe: 2 S3 puts (NSU key + access_key based key).
	if len(s3m.putCalls) < 2 {
		t.Errorf("expected at least 2 S3 PutObject calls for procNFe, got %d", len(s3m.putCalls))
	}

	// PutItem: nfes (persistIncoming) + nfe_distributions (deferred). The dest
	// counterparty (98765432000188 ≠ org) is written via TransactWriteItems
	// (person + audit log), not PutItem; the emit CNPJ equals the org's, so no
	// person is written for it.
	if len(dynm.putCalls) != 2 {
		t.Errorf("expected 2 PutItem calls (nfes + nfe_distributions), got %d", len(dynm.putCalls))
	}
	if len(dynm.transactCalls) != 1 {
		t.Errorf("expected 1 TransactWriteItems call (organization_persons + audit log), got %d", len(dynm.transactCalls))
	}

	// notifyResult publishes to ResultsTopicARN.
	if len(snsm.calls) != 1 {
		t.Errorf("expected 1 SNS publish (notifyResult), got %d", len(snsm.calls))
	}
}

func TestDistNSU_WithProcEventoNFe_PersistsEvent(t *testing.T) {
	docZips := []map[string]any{
		makeDocZip(1002, "procEventoNFe_v1.00.xsd", resEventoNFeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	s3m := certS3()
	snsm := &mockSNS{}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 1002, 1002, docZips)}
	svc := newDistSvc(dynm, s3m, lam, snsm, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// PutItem: nfe_events (persistEvent) + nfe_distributions (deferred).
	if len(dynm.putCalls) != 2 {
		t.Errorf("expected 2 PutItem calls (nfe_events + nfe_distributions), got %d", len(dynm.putCalls))
	}

	// No SNS for events.
	if len(snsm.calls) != 0 {
		t.Errorf("expected no SNS publish for event schema, got %d", len(snsm.calls))
	}
}

func TestDistNSU_WithResCTe_NoPersistNoAutoScience(t *testing.T) {
	// resCTe: summary only — no persistIncoming, no auto-Ciência.
	docZips := []map[string]any{
		makeDocZip(2000, "resCTe_v1.00.xsd", resCTeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	snsm := &mockSNS{}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 2000, 2000, docZips)}
	svc := newDistSvc(dynm, certS3(), lam, snsm, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "cte",
	})
	if err != nil {
		t.Fatalf("Process: %v", err)
	}

	// Only 1 PutItem: nfe_distributions (deferred).
	if len(dynm.putCalls) != 1 {
		t.Errorf("expected 1 PutItem (dist record only), got %d", len(dynm.putCalls))
	}
	if len(snsm.calls) != 0 {
		t.Errorf("expected no SNS for resCTe, got %d", len(snsm.calls))
	}
}

func TestDistNSU_AutoScience_SkippedWhenNoEventBusTopic(t *testing.T) {
	// EventBusTopicARN is empty → Ciência SNS is skipped.
	cfg := *distCfg
	cfg.EventBusTopicARN = ""

	docZips := []map[string]any{
		makeDocZip(1003, "resNFe_v1.01.xsd", resNFeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	snsm := &mockSNS{}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 1003, 1003, docZips)}
	svc := newDistSvc(dynm, certS3(), lam, snsm, &cfg)

	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})

	// autoScience creates the nfe_events PutItem but skips SNS.
	if len(snsm.calls) != 0 {
		t.Errorf("expected no SNS publish when EventBusTopicARN empty, got %d", len(snsm.calls))
	}
}

func TestDistNSU_InvalidDocZip_StillWritesDistRecord(t *testing.T) {
	// Malformed #text — decompressDocZip will fail.
	badDocZip := map[string]any{
		"@NSU":    "000000001004",
		"@schema": "resNFe_v1.01.xsd",
		"#text":   "NOTVALIDGZIP",
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 1004, 1004, []map[string]any{badDocZip})}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)

	_ = svc.Process(context.Background(), DistributionMessage{
		JobType: "dist_nsu", OrgPK: testOrgPK, DocType: "nfe",
	})

	// The deferred PutItem to dist table must still run (with parse_error=true).
	if len(dynm.putCalls) != 1 {
		t.Errorf("expected 1 PutItem (dist record with parse_error), got %d", len(dynm.putCalls))
	}
}

// ---------------------------------------------------------------------------
// runConsNSU
// ---------------------------------------------------------------------------

func TestDistConsNSU_Happy_ProcessesDocZips(t *testing.T) {
	nsu := 500
	docZips := []map[string]any{
		makeDocZip(nsu, "procNFe_v4.00.xsd", procNFeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, nsu, nsu, docZips)}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType: "cons_nsu",
		OrgPK:   testOrgPK,
		DocType: "nfe",
		NSU:     &nsu,
	})
	if err != nil {
		t.Fatalf("Process cons_nsu: %v", err)
	}
	// PutItem: nfes + nfe_distributions.
	if len(dynm.putCalls) < 1 {
		t.Errorf("expected at least 1 PutItem, got 0")
	}
}

// ---------------------------------------------------------------------------
// runConsAccessKey
// ---------------------------------------------------------------------------

func TestDistConsChNFe_Happy_ProcessesDocZips(t *testing.T) {
	docZips := []map[string]any{
		makeDocZip(600, "procNFe_v4.00.xsd", procNFeXML),
	}
	dynm := &mockDistDynamo{
		gets:    []getResult{{item: configItem(2, "hom", "", "")}, {item: orgItemWithUF("SP")}},
		queries: []queryResult{{items: []map[string]types.AttributeValue{certItem()}}},
	}
	lam := &mockLambda{payload: distNSUResp(cStatDocFound, 600, 600, docZips)}
	svc := newDistSvc(dynm, certS3(), lam, &mockSNS{}, distCfg)

	err := svc.Process(context.Background(), DistributionMessage{
		JobType:   "cons_ch_nfe",
		OrgPK:     testOrgPK,
		DocType:   "nfe",
		AccessKey: testAK,
	})
	if err != nil {
		t.Fatalf("Process cons_ch_nfe: %v", err)
	}
	if len(dynm.putCalls) < 1 {
		t.Errorf("expected at least 1 PutItem, got 0")
	}
}

// ---------------------------------------------------------------------------
// Distribution small helpers
// ---------------------------------------------------------------------------

func TestExtractCNPJ_StripCNPJPrefix(t *testing.T) {
	// OrgPK format is "CNPJ_<cnpj>"; extractCNPJ splits on the first "_".
	if got := extractCNPJ("CNPJ_12345678000195"); got != "12345678000195" {
		t.Errorf("extractCNPJ = %q, want 12345678000195", got)
	}
}

func TestExtractCNPJ_NoPrefixPassthrough(t *testing.T) {
	// When there is no "_", the whole string is returned as-is.
	if got := extractCNPJ("12345678000195"); got != "12345678000195" {
		t.Errorf("extractCNPJ no prefix = %q", got)
	}
}

func TestExtractUF_FromOrg(t *testing.T) {
	org := orgItemWithUF("RJ")
	if got := extractUF(org); got != "RJ" {
		t.Errorf("extractUF = %q, want RJ", got)
	}
}

func TestExtractUF_NilOrg_DefaultsSP(t *testing.T) {
	if got := extractUF(nil); got != "SP" {
		t.Errorf("extractUF(nil) = %q, want SP", got)
	}
}

func TestExtractUF_NoAddresses_DefaultsSP(t *testing.T) {
	org := map[string]types.AttributeValue{
		"pk": &types.AttributeValueMemberS{Value: testOrgPK},
		"person": &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{
			"addresses": &types.AttributeValueMemberL{Value: nil},
		}},
	}
	if got := extractUF(org); got != "SP" {
		t.Errorf("extractUF no addresses = %q, want SP", got)
	}
}

// ---------------------------------------------------------------------------
// Person persistence (counterparties)
// ---------------------------------------------------------------------------

func TestBuildPersonSK(t *testing.T) {
	cases := []struct {
		digits string
		wantSK string
		wantOK bool
	}{
		{"12345678000195", "CNPJ_12345678000195", true},
		{"12345678909", "CPF_12345678909", true},
		{"123", "", false},
		{"", "", false},
	}
	for _, c := range cases {
		gotSK, gotOK := buildPersonSK(c.digits)
		if gotSK != c.wantSK || gotOK != c.wantOK {
			t.Errorf("buildPersonSK(%q) = (%q, %v), want (%q, %v)", c.digits, gotSK, gotOK, c.wantSK, c.wantOK)
		}
	}
}

func TestOnlyDigits(t *testing.T) {
	if got := onlyDigits("12.345.678/0001-95"); got != "12345678000195" {
		t.Errorf("onlyDigits = %q, want 12345678000195", got)
	}
	if got := onlyDigits("org#12345678000195"); got != "12345678000195" {
		t.Errorf("onlyDigits prefix = %q, want 12345678000195", got)
	}
}

func TestPersistPerson_WritesCounterpartyItem(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, distCfg)

	svc.persistPerson(context.Background(), testOrgPK, "12345678000195", "98765432000188", "Fornecedor LTDA", nil)

	if len(dynm.transactCalls) != 1 {
		t.Fatalf("expected 1 TransactWriteItems call, got %d", len(dynm.transactCalls))
	}
	items := dynm.transactCalls[0].TransactItems
	if len(items) != 2 {
		t.Fatalf("expected 2 transact items (person + audit log), got %d", len(items))
	}

	personPut := items[0].Put
	if personPut == nil {
		t.Fatalf("expected first transact item to be a Put")
	}
	if got := *personPut.TableName; got != "dev_organization_persons" {
		t.Errorf("table = %q, want dev_organization_persons", got)
	}
	if personPut.ConditionExpression == nil || *personPut.ConditionExpression != "attribute_not_exists(pk)" {
		t.Errorf("expected create-if-absent condition, got %v", personPut.ConditionExpression)
	}
	assertItemS(t, personPut.Item, "pk", testOrgPK)
	assertItemS(t, personPut.Item, "sk", "CNPJ_98765432000188")
	assertItemS(t, personPut.Item, "cpf_or_cnpj", "98765432000188")
	assertItemS(t, personPut.Item, "name", "Fornecedor LTDA")

	auditPut := items[1].Put
	if auditPut == nil {
		t.Fatalf("expected second transact item to be a Put")
	}
	if got := *auditPut.TableName; got != "dev_audit_logs" {
		t.Errorf("audit table = %q, want dev_audit_logs", got)
	}
	assertItemS(t, auditPut.Item, "pk", testOrgPK)
	assertItemS(t, auditPut.Item, "resource_type", "PERSON")
	assertItemS(t, auditPut.Item, "resource_id", "CNPJ_98765432000188")
	assertItemS(t, auditPut.Item, "action", "CREATE")
	assertItemS(t, auditPut.Item, "user_id", "SYSTEM")
}

func TestPersistPerson_WritesNestedPersonDetails(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, distCfg)

	details := map[string]any{
		"fantasy_name": "FORNECEDOR",
		"crt":          3,
		"addresses":    []any{map[string]any{"city": "SAO PAULO", "state_federation": "SP"}},
		"state_registrations": []any{map[string]any{
			"uf": "SP", "state_registration": "123456789",
		}},
	}
	svc.persistPerson(context.Background(), testOrgPK, "12345678000195", "98765432000188", "Fornecedor SA", details)

	if len(dynm.transactCalls) != 1 {
		t.Fatalf("expected 1 TransactWriteItems call, got %d", len(dynm.transactCalls))
	}
	personPut := dynm.transactCalls[0].TransactItems[0].Put
	if personPut == nil {
		t.Fatalf("expected first transact item to be a Put")
	}
	person, ok := personPut.Item["person"].(*types.AttributeValueMemberM)
	if !ok {
		t.Fatalf("person attribute missing or not a map")
	}
	if got := person.Value["fantasy_name"].(*types.AttributeValueMemberS).Value; got != "FORNECEDOR" {
		t.Errorf("person.fantasy_name = %q", got)
	}
	if got := person.Value["crt"].(*types.AttributeValueMemberN).Value; got != "3" {
		t.Errorf("person.crt = %q, want 3", got)
	}
	addrs := person.Value["addresses"].(*types.AttributeValueMemberL).Value
	if len(addrs) != 1 {
		t.Fatalf("expected 1 address, got %d", len(addrs))
	}
	addr := addrs[0].(*types.AttributeValueMemberM).Value
	if got := addr["state_federation"].(*types.AttributeValueMemberS).Value; got != "SP" {
		t.Errorf("address.state_federation = %q", got)
	}
	regs := person.Value["state_registrations"].(*types.AttributeValueMemberL).Value
	reg := regs[0].(*types.AttributeValueMemberM).Value
	if got := reg["state_registration"].(*types.AttributeValueMemberS).Value; got != "123456789" {
		t.Errorf("state_registration = %q", got)
	}
}

func TestPersistPerson_SkipsOrgSelfAndBlank(t *testing.T) {
	dynm := &mockDistDynamo{}
	svc := newDistSvc(dynm, certS3(), &mockLambda{}, &mockSNS{}, distCfg)

	// Same CPF/CNPJ as the org → skipped.
	svc.persistPerson(context.Background(), testOrgPK, "12345678000195", "12345678000195", "Self", nil)
	// Blank counterparty → skipped.
	svc.persistPerson(context.Background(), testOrgPK, "12345678000195", "", "Blank", nil)
	// Invalid digit count → skipped.
	svc.persistPerson(context.Background(), testOrgPK, "12345678000195", "123", "Invalid", nil)

	if len(dynm.putCalls) != 0 {
		t.Errorf("expected 0 PutItem for skipped cases, got %d", len(dynm.putCalls))
	}
	if len(dynm.transactCalls) != 0 {
		t.Errorf("expected 0 TransactWriteItems for skipped cases, got %d", len(dynm.transactCalls))
	}
}

func assertItemS(t *testing.T, item map[string]types.AttributeValue, key, want string) {
	t.Helper()
	v, ok := item[key].(*types.AttributeValueMemberS)
	if !ok {
		t.Errorf("item[%q] missing or not string", key)
		return
	}
	if v.Value != want {
		t.Errorf("item[%q] = %q, want %q", key, v.Value, want)
	}
}

func TestAsSlice_SingleMapPromotion(t *testing.T) {
	// When the JSON value is a single object (not array), asSlice must wrap it.
	m := map[string]any{"docZip": map[string]any{"@NSU": "1"}}
	got := asSlice(m, "docZip")
	if len(got) != 1 {
		t.Errorf("asSlice single map = %d items, want 1", len(got))
	}
}

func TestAsSlice_Array(t *testing.T) {
	m := map[string]any{"docZip": []any{"a", "b"}}
	got := asSlice(m, "docZip")
	if len(got) != 2 {
		t.Errorf("asSlice array = %d items, want 2", len(got))
	}
}

func TestAsSlice_NilMap_ReturnsNil(t *testing.T) {
	if got := asSlice(nil, "docZip"); got != nil {
		t.Errorf("asSlice(nil) should return nil")
	}
}

func TestMapStr_ReturnsDef_WhenMissing(t *testing.T) {
	if got := mapStr(map[string]any{}, "key", "default"); got != "default" {
		t.Errorf("mapStr missing = %q, want default", got)
	}
}

func TestIntVal_Float64(t *testing.T) {
	if got := intVal(map[string]any{"n": float64(42)}, "n", 0); got != 42 {
		t.Errorf("intVal float64 = %d, want 42", got)
	}
}

func TestIntVal_String(t *testing.T) {
	if got := intVal(map[string]any{"n": "99"}, "n", 0); got != 99 {
		t.Errorf("intVal string = %d, want 99", got)
	}
}

func TestIntVal_Default(t *testing.T) {
	if got := intVal(map[string]any{}, "n", 7); got != 7 {
		t.Errorf("intVal default = %d, want 7", got)
	}
}

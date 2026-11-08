package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	lambdaSDK "github.com/aws/aws-sdk-go-v2/service/lambda"

	"gopkg.aoctech.app/dfe/go-dfe/autocontingency"
)

const probeOrg = "CNPJ_12345678000195"

var t0 = time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

// breakerDB is an in-memory config row with just enough of UpdateItem for the breaker's compare-and-swap.
type breakerDB struct {
	mockDynamo
	mu      sync.Mutex
	row     map[string]types.AttributeValue
	updates int
	// beforeUpdate lets a test change the row between the read and the write (a concurrent writer).
	beforeUpdate func(row map[string]types.AttributeValue)
}

func s(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }

func newRow(extra map[string]types.AttributeValue) map[string]types.AttributeValue {
	r := map[string]types.AttributeValue{
		"pk": s(probeOrg), attrConfigEnv: &types.AttributeValueMemberN{Value: "1"},
		attrAutoContingency: &types.AttributeValueMemberBOOL{Value: true},
	}
	for k, v := range extra {
		r[k] = v
	}
	return r
}

func (d *breakerDB) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case strings.HasSuffix(*in.TableName, "_configs"):
		cp := map[string]types.AttributeValue{}
		for k, v := range d.row {
			cp[k] = v
		}
		return &dynamodb.GetItemOutput{Item: cp}, nil
	}
	return &dynamodb.GetItemOutput{}, nil // organizations: none (tax id comes from the pk)
}

func (d *breakerDB) Query(_ context.Context, in *dynamodb.QueryInput, _ ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	return &dynamodb.QueryOutput{Items: []map[string]types.AttributeValue{{"s3_key": s("certs/c.pfx"), "password": s("pw")}}}, nil
}

func (d *breakerDB) UpdateItem(_ context.Context, in *dynamodb.UpdateItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.UpdateItemOutput, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.beforeUpdate != nil {
		d.beforeUpdate(d.row)
		d.beforeUpdate = nil
	}
	name := func(n string) string { return in.ExpressionAttributeNames[n] }
	val := func(v string) types.AttributeValue { return in.ExpressionAttributeValues[v] }
	cond := *in.ConditionExpression
	cur, _ := d.row[name("#b")].(*types.AttributeValueMemberS)
	if strings.Contains(cond, "attribute_not_exists(#b)") && cur != nil {
		return nil, &types.ConditionalCheckFailedException{}
	}
	if strings.Contains(cond, "#b = :old") && (cur == nil || cur.Value != val(":old").(*types.AttributeValueMemberS).Value) {
		return nil, &types.ConditionalCheckFailedException{}
	}
	if strings.Contains(cond, "#ca = :true") {
		if ca, ok := d.row[name("#ca")].(*types.AttributeValueMemberBOOL); !ok || !ca.Value {
			return nil, &types.ConditionalCheckFailedException{}
		}
	}
	expr := *in.UpdateExpression
	setPart, removePart, _ := strings.Cut(strings.TrimPrefix(expr, "SET "), " REMOVE ")
	if strings.HasPrefix(expr, "REMOVE ") {
		setPart, removePart = "", strings.TrimPrefix(expr, "REMOVE ")
	}
	for _, a := range strings.Split(setPart, ", ") {
		if a == "" {
			continue
		}
		n, v, _ := strings.Cut(a, " = ")
		d.row[name(n)] = val(v)
	}
	for _, r := range strings.Split(removePart, ", ") {
		if r != "" {
			delete(d.row, name(r))
		}
	}
	d.updates++
	return &dynamodb.UpdateItemOutput{}, nil
}

func (b *breakerDB) breaker() autocontingency.Breaker {
	raw, _ := b.row[attrAutoBreaker].(*types.AttributeValueMemberS)
	if raw == nil {
		return autocontingency.Breaker{State: autocontingency.Closed}
	}
	return autocontingency.Decode(raw.Value)
}

func breakerSvc(t *testing.T, db *breakerDB, lam LambdaClient) (*DfeService, *captureAlerts, *time.Time) {
	t.Helper()
	old := switchEnabled
	switchEnabled = true
	t.Cleanup(func() { switchEnabled = old })
	now := t0
	al := &captureAlerts{}
	svc := New(Clients{S3: certS3(), Lambda: lam, Dynamo: db}, testCfg).WithAlerter(al)
	svc.clock = func() time.Time { return now }
	svc.pause = func(_ context.Context, d time.Duration) { now = now.Add(d) }
	return svc, al, &now
}

func authMsg() WorkerMessage {
	m := baseMsg
	m.DocPK = "prod#" + probeOrg
	return m
}

func TestObserveAuth_ThreeUnavailableOpenAndAlertOnce(t *testing.T) {
	db := &breakerDB{row: newRow(nil)}
	svc, al, now := breakerSvc(t, db, &mockLambda{})
	for i := 0; i < 3; i++ {
		svc.observeAuth(context.Background(), authMsg(), autocontingency.Observation{TransportErr: true})
		*now = now.Add(10 * time.Second)
	}
	if db.breaker().State != autocontingency.Open || len(al.got) != 1 || al.got[0].Job != alertJobBreaker {
		t.Fatalf("breaker=%+v alerts=%d", db.breaker(), len(al.got))
	}
	svc.observeAuth(context.Background(), authMsg(), autocontingency.Observation{TransportErr: true})
	if len(al.got) != 1 {
		t.Error("an open breaker must not alert again")
	}
}

func TestObserveAuth_NeverCountsBusinessAnswersOrContingencyDocs(t *testing.T) {
	cases := map[string]func(m *WorkerMessage, o *autocontingency.Observation){
		"rejection 539 is a healthy answer": func(_ *WorkerMessage, o *autocontingency.Observation) { *o = autocontingency.Observation{CStat: "539"} },
		"429 is not unavailability": func(_ *WorkerMessage, o *autocontingency.Observation) {
			*o = autocontingency.Observation{HTTPStatus: 429}
		},
		"egress 4xx is on our side": func(_ *WorkerMessage, o *autocontingency.Observation) {
			*o = autocontingency.Observation{HTTPStatus: 400}
		},
		"document in contingency (tpEmis 9)": func(m *WorkerMessage, _ *autocontingency.Observation) {
			m.AccessKey = m.AccessKey[:34] + "9" + m.AccessKey[35:]
		},
		"SVC route":                           func(m *WorkerMessage, _ *autocontingency.Observation) { m.Contingency = "SVC-AN" },
		"event":                               func(m *WorkerMessage, _ *autocontingency.Observation) { m.EventsTableName = new("nfe_events") },
		"NFS-e":                               func(m *WorkerMessage, _ *autocontingency.Observation) { m.DocType = "nfse" },
		"homologation answer, production row": func(m *WorkerMessage, _ *autocontingency.Observation) { m.SefazEnvironment = "homologacao" },
	}
	for name, mut := range cases {
		t.Run(name, func(t *testing.T) {
			db := &breakerDB{row: newRow(nil)}
			svc, _, _ := breakerSvc(t, db, &mockLambda{})
			for i := 0; i < 5; i++ {
				m, o := authMsg(), autocontingency.Observation{TransportErr: true}
				mut(&m, &o)
				svc.observeAuth(context.Background(), m, o)
			}
			if db.breaker().State == autocontingency.Open {
				t.Fatal("breaker opened")
			}
		})
	}
}

func TestObserveAuth_SwitchOffAndFlagOffTouchNothing(t *testing.T) {
	db := &breakerDB{row: newRow(nil)}
	svc, _, _ := breakerSvc(t, db, &mockLambda{})
	switchEnabled = false
	for i := 0; i < 5; i++ {
		svc.observeAuth(context.Background(), authMsg(), autocontingency.Observation{TransportErr: true})
	}
	if db.updates != 0 {
		t.Fatal("switch off must not write")
	}
	switchEnabled = true
	db.row[attrAutoContingency] = &types.AttributeValueMemberBOOL{Value: false}
	for i := 0; i < 5; i++ {
		svc.observeAuth(context.Background(), authMsg(), autocontingency.Observation{TransportErr: true})
	}
	if db.updates != 0 {
		t.Fatal("flag off must not write")
	}
}

func TestObserveAuth_HealthyAnswerResetsFailures(t *testing.T) {
	db := &breakerDB{row: newRow(nil)}
	svc, _, _ := breakerSvc(t, db, &mockLambda{})
	for _, o := range []autocontingency.Observation{{TransportErr: true}, {TransportErr: true}, {CStat: "100"}, {TransportErr: true}, {TransportErr: true}} {
		svc.observeAuth(context.Background(), authMsg(), o)
	}
	if db.breaker().State == autocontingency.Open {
		t.Fatal("failures must be consecutive")
	}
}

func TestApplyObservation_LostCompareAndSwapRereadsOnce(t *testing.T) {
	db := &breakerDB{row: newRow(nil)}
	svc, _, _ := breakerSvc(t, db, &mockLambda{})
	other := autocontingency.Encode(autocontingency.Breaker{State: autocontingency.Closed, Failures: []time.Time{t0}})
	db.beforeUpdate = func(r map[string]types.AttributeValue) { r[attrAutoBreaker] = s(other) } // a concurrent writer
	svc.observeAuth(context.Background(), authMsg(), autocontingency.Observation{TransportErr: true})
	if n := len(db.breaker().Failures); n != 2 {
		t.Fatalf("both failures must survive the retry, got %d", n)
	}
}

// statusLambda answers the status probe: the sequence of cStat (or egress codes) per call.
type statusLambda struct {
	answers []string // "107", "108", "500", "400", "err"
	calls   int
	bodies  []map[string]any
}

func (l *statusLambda) Invoke(_ context.Context, in *lambdaSDK.InvokeInput, _ ...func(*lambdaSDK.Options)) (*lambdaSDK.InvokeOutput, error) {
	var req lambdaPayload
	_ = json.Unmarshal(in.Payload, &req)
	l.bodies = append(l.bodies, req.Body)
	a := l.answers[min(l.calls, len(l.answers)-1)]
	l.calls++
	switch a {
	case "err":
		return nil, errors.New("timeout")
	case "500", "400":
		code := map[string]int{"500": 500, "400": 400}[a]
		return &lambdaSDK.InvokeOutput{Payload: invokeRespStatus(code, "x")}, nil
	}
	return &lambdaSDK.InvokeOutput{Payload: invokeResp(a, "x", "")}, nil
}

func TestProbe_UnavailableBurstOpensBreakerAndCallsStatusService(t *testing.T) {
	for _, a := range []string{"err", "500", "108", "109"} {
		db := &breakerDB{row: newRow(nil)}
		lam := &statusLambda{answers: []string{a}}
		svc, al, _ := breakerSvc(t, db, lam)
		if err := svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg}); err != nil {
			t.Fatal(err)
		}
		if db.breaker().State != autocontingency.Open || lam.calls != autocontingency.RecoveryProbes || len(al.got) != 1 {
			t.Fatalf("%s: breaker=%+v calls=%d alerts=%d", a, db.breaker(), lam.calls, len(al.got))
		}
		root := lam.bodies[0]["consStatServ"].(map[string]any)
		if root["xServ"] != "STATUS" || root["tpAmb"] != "1" || root["cUF"] != "35" {
			t.Errorf("status body: %v", root)
		}
	}
}

func TestProbe_HealthyOrNeutralIsOneCallAndNoWrite(t *testing.T) {
	for _, a := range []string{"107", "400"} {
		db := &breakerDB{row: newRow(nil)}
		lam := &statusLambda{answers: []string{a}}
		svc, _, _ := breakerSvc(t, db, lam)
		_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_cte_configs", PK: probeOrg})
		if lam.calls != 1 || db.updates != 0 {
			t.Fatalf("%s: calls=%d updates=%d", a, lam.calls, db.updates)
		}
	}
}

func TestProbe_NoCallInsideCoolDownAndRecoveryNeedsThreeHealthy(t *testing.T) {
	open := autocontingency.Breaker{State: autocontingency.Open, OpenedAt: t0.Add(-time.Minute)}
	db := &breakerDB{row: newRow(map[string]types.AttributeValue{attrAutoBreaker: s(autocontingency.Encode(open))})}
	lam := &statusLambda{answers: []string{"107"}}
	svc, al, now := breakerSvc(t, db, lam)
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	if lam.calls != 0 {
		t.Fatal("no probe inside the cool-down")
	}
	*now = t0.Add(5 * time.Minute) // cool-down over
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	if lam.calls != 3 || db.breaker().State != autocontingency.Closed || len(al.got) != 1 {
		t.Fatalf("calls=%d breaker=%+v alerts=%d", lam.calls, db.breaker(), len(al.got))
	}
}

func TestProbe_BadProbeResetsTheStreak(t *testing.T) {
	open := autocontingency.Breaker{State: autocontingency.Open, OpenedAt: t0.Add(-time.Hour)}
	db := &breakerDB{row: newRow(map[string]types.AttributeValue{attrAutoBreaker: s(autocontingency.Encode(open))})}
	lam := &statusLambda{answers: []string{"107", "107", "108"}}
	svc, _, _ := breakerSvc(t, db, lam)
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	if b := db.breaker(); b.State != autocontingency.Open || b.Streak != 0 {
		t.Fatalf("2 healthy then 1 bad must leave it open with streak 0: %+v", b)
	}
}

func TestProbe_ClosingReturnsAutoContingencyToNormalAtomically(t *testing.T) {
	open := autocontingency.Breaker{State: autocontingency.Open, OpenedAt: t0.Add(-time.Hour)}
	auto := newRow(map[string]types.AttributeValue{
		attrAutoBreaker: s(autocontingency.Encode(open)), attrEmissionType: s("svc"),
		attrContingencyStart: s("2026-11-08T10:00:00Z"), attrContingencyWhy: s("x"), attrContingencyAuto: &types.AttributeValueMemberBOOL{Value: true},
	})
	db := &breakerDB{row: auto}
	svc, _, _ := breakerSvc(t, db, &statusLambda{answers: []string{"107"}})
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	r := db.row
	if r[attrEmissionType].(*types.AttributeValueMemberS).Value != "normal" {
		t.Fatal("must return to normal")
	}
	for _, k := range []string{attrContingencyStart, attrContingencyWhy, attrContingencyAuto, attrAutoBreaker} {
		if _, ok := r[k]; ok {
			t.Errorf("%s must be removed (contingency_since nulled)", k)
		}
	}
	if db.updates != 3 { // 3 probes, the last one closes; each probe is one write
		t.Logf("updates=%d", db.updates)
	}
}

func TestProbe_ManualContingencyIsNeverUndone(t *testing.T) {
	open := autocontingency.Breaker{State: autocontingency.Open, OpenedAt: t0.Add(-time.Hour)}
	db := &breakerDB{row: newRow(map[string]types.AttributeValue{ // no contingency_auto: the owner chose svc
		attrAutoBreaker: s(autocontingency.Encode(open)), attrEmissionType: s("svc"), attrContingencyStart: s("2026-11-08T10:00:00Z"),
	})}
	svc, _, _ := breakerSvc(t, db, &statusLambda{answers: []string{"107"}})
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	if db.row[attrEmissionType].(*types.AttributeValueMemberS).Value != "svc" || db.breaker().State != autocontingency.Closed {
		t.Fatalf("manual contingency kept, breaker closed: %v", db.row)
	}
}

func TestProbe_SwitchOffAndStaleIndexDoNothing(t *testing.T) {
	db := &breakerDB{row: newRow(nil)}
	lam := &statusLambda{answers: []string{"108"}}
	svc, _, _ := breakerSvc(t, db, lam)
	switchEnabled = false
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	switchEnabled = true
	db.row[attrAutoContingency] = &types.AttributeValueMemberBOOL{Value: false}
	_ = svc.Probe(context.Background(), ProbeJob{Table: "organization_nfe_configs", PK: probeOrg})
	if lam.calls != 0 {
		t.Fatal("no egress call with the switch off or the flag off")
	}
	if err := svc.Probe(context.Background(), ProbeJob{Table: "nfes", PK: probeOrg}); err == nil {
		t.Error("an unknown table is a defect")
	}
}

func TestProcess_FeedsBreakerFromRealAuthorization(t *testing.T) {
	db := &breakerDB{row: newRow(nil)}
	db.mockDynamo = mockDynamo{}
	svc, _, _ := breakerSvc(t, db, &mockLambda{err: errors.New("timeout")})
	_ = svc.Process(context.Background(), authMsg())
	if f := db.breaker().Failures; len(f) != 1 {
		t.Fatalf("a transport failure of a normal authorization must feed the breaker, failures=%d", len(f))
	}
}

//go:build integration

package integration_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/lambda"

	"gopkg.aoctech.app/api-commons/webpush"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// recordingInvoker stands in for the Lambda client: it keeps every job the service hands to the sender.
type recordingInvoker struct {
	mu   sync.Mutex
	jobs []services.PushSendJob
	err  error
}

func (r *recordingInvoker) Invoke(_ context.Context, in *lambda.InvokeInput, _ ...func(*lambda.Options)) (*lambda.InvokeOutput, error) {
	if r.err != nil {
		return nil, r.err
	}
	var job services.PushSendJob
	if err := json.Unmarshal(in.Payload, &job); err != nil {
		return nil, err
	}
	r.mu.Lock()
	r.jobs = append(r.jobs, job)
	r.mu.Unlock()
	return &lambda.InvokeOutput{StatusCode: 202}, nil
}

func (r *recordingInvoker) taken() []services.PushSendJob {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]services.PushSendJob(nil), r.jobs...)
}

func vapidPublicKey(t *testing.T) string {
	t.Helper()
	_, key, err := webpush.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return key.PublicBase64URL()
}

func newPushService(t *testing.T, inv services.PushInvoker) *services.PushService {
	t.Helper()
	return services.NewPushService(newPushRepo(), memberSvc, orgSvc, nil, "push-sender", vapidPublicKey(t)).WithInvoker(inv)
}

// seedCompany creates the company row (its name is the title's suffix) and two members.
func seedPushCompany(t *testing.T, ctx context.Context, members ...string) string {
	t.Helper()
	company := "CNPJ_" + randomCNPJ()
	putItem(t, "organizations", map[string]any{"pk": company, "name": "Transportadora Exemplo Ltda de Longo Nome Social"})
	for _, m := range members {
		if err := memberSvc.Create(ctx, company, m, repositories.RoleViewer, "owner", "Member", nil); err != nil {
			t.Fatal(err)
		}
	}
	return company
}

// A rejection goes to who ticked it and still is a member, once per window, and the redelivery of the same message
// does not send twice.
func TestPushService_EmitResultFansOutOnceToMembersWhoAsked(t *testing.T) {
	ctx := context.Background()
	inv := &recordingInvoker{}
	svc := newPushService(t, inv)
	a, b, gone := "user-a-"+randomCNPJ(), "user-b-"+randomCNPJ(), "user-gone-"+randomCNPJ()
	company := seedPushCompany(t, ctx, a, b)
	repo := newPushRepo()
	for _, who := range []string{a, gone} {
		if err := repo.PutPreference(ctx, company, who, []string{services.PushEventRejected}); err != nil {
			t.Fatal(err)
		}
	}
	// b is a member but never ticked it; gone ticked it but is not a member of the company.

	in := services.PushResult{CompanyPK: company, DocumentKey: strings.Repeat("3", 44), Table: "nfes", Status: "rejected", SentAt: time.Now()}
	if err := svc.EmitResult(ctx, in); err != nil {
		t.Fatal(err)
	}
	jobs := inv.taken()
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	j := jobs[0]
	if len(j.Recipients) != 1 || j.Recipients[0] != a {
		t.Errorf("recipients = %v, want only %s", j.Recipients, a)
	}
	if !strings.HasPrefix(j.Title, "NF-e rejeitada · Transportadora Exemplo") || !strings.HasSuffix(j.Title, "…") {
		t.Errorf("title = %q", j.Title)
	}
	if j.Navigate != "/nfe/detail?key="+in.DocumentKey+"&org="+company || j.Urgency != "high" {
		t.Errorf("navigate/urgency = %q %q", j.Navigate, j.Urgency)
	}
	for _, secret := range []string{"Longo Nome Social"} {
		if strings.Contains(j.Body, secret) {
			t.Errorf("body leaks %q", secret)
		}
	}

	// Same message again, and a second rejection in the same window: nothing more is sent.
	if err := svc.EmitResult(ctx, in); err != nil {
		t.Fatal(err)
	}
	other := in
	other.DocumentKey = strings.Repeat("4", 44)
	if err := svc.EmitResult(ctx, other); err != nil {
		t.Fatal(err)
	}
	if got := len(inv.taken()); got != 1 {
		t.Errorf("jobs after a redelivery and a second event in the window = %d, want 1", got)
	}

	// A status nobody reports (still in flight) and a table that is not a document send nothing.
	for _, bad := range []services.PushResult{
		{CompanyPK: company, DocumentKey: "k", Table: "nfes", Status: "pending"},
		{CompanyPK: company, DocumentKey: "k", Table: "ciots", Status: "rejected"},
	} {
		if err := svc.EmitResult(ctx, bad); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(inv.taken()); got != 1 {
		t.Errorf("an unreported result sent a job: %d", got)
	}
}

// A failed hand-off releases the markers, so the queue's redelivery sends.
func TestPushService_FailedHandOffIsRetriedByTheRedelivery(t *testing.T) {
	ctx := context.Background()
	inv := &recordingInvoker{err: errors.New("throttled")}
	svc := newPushService(t, inv)
	a := "user-a-" + randomCNPJ()
	company := seedPushCompany(t, ctx, a)
	if err := newPushRepo().PutPreference(ctx, company, a, []string{services.PushEventAuthorized}); err != nil {
		t.Fatal(err)
	}
	in := services.PushResult{CompanyPK: company, DocumentKey: strings.Repeat("5", 44), Table: "nfces", Status: "authorized"}
	if err := svc.EmitResult(ctx, in); err == nil {
		t.Fatal("a failed hand-off must be returned so the message stays on the queue")
	}
	inv.err = nil
	if err := svc.EmitResult(ctx, in); err != nil {
		t.Fatal(err)
	}
	if got := len(inv.taken()); got != 1 {
		t.Errorf("jobs after the retry = %d, want 1", got)
	}
}

// A received document is the only event on by default, goes to the distribution list, and carries no fiscal data.
func TestPushService_DistributionIsOnByDefaultAndCarriesNoFiscalData(t *testing.T) {
	ctx := context.Background()
	inv := &recordingInvoker{}
	svc := newPushService(t, inv)
	a := "user-a-" + randomCNPJ()
	company := seedPushCompany(t, ctx, a)

	endpoint := "https://fcm.googleapis.com/fcm/send/" + strings.Repeat("a", 40)
	key := ecdhP256(t)
	device, err := svc.Subscribe(ctx, company, a, services.PushSubscriptionInput{
		Endpoint: endpoint, P256dh: key.p256dh, Auth: key.auth, UserAgent: "Mozilla/5.0 (Linux; Android 14) Chrome/130 Mobile Safari/537.36",
	})
	if err != nil || device.Device != "Chrome no Android" {
		t.Fatalf("subscribe: %+v %v", device, err)
	}
	if prefs, _ := svc.Preferences(ctx, company, a); len(prefs) != 1 || prefs[0] != services.PushEventDistribution {
		t.Fatalf("a new device must start with only distribution on: %v", prefs)
	}

	if err := svc.EmitDistribution(ctx, company, strings.Repeat("6", 44), "cte", time.Now()); err != nil {
		t.Fatal(err)
	}
	jobs := inv.taken()
	if len(jobs) != 1 {
		t.Fatalf("jobs = %d, want 1", len(jobs))
	}
	if jobs[0].Navigate != "/cte/distributions?org="+company || jobs[0].Urgency != "low" {
		t.Errorf("navigate/urgency = %q %q", jobs[0].Navigate, jobs[0].Urgency)
	}
	payload, err := services.BuildPushPayload(jobs[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"emit_name", "total", "R$"} {
		if strings.Contains(string(payload), forbidden) {
			t.Errorf("payload carries %q: %s", forbidden, payload)
		}
	}
}

// One browser endpoint belongs to one person: a second person subscribing the same endpoint (a shared computer where
// the first never logged out cleanly) takes it over, and the first stops being notified there.
func TestPushService_SubscribeRebindsTheEndpointToTheCurrentPerson(t *testing.T) {
	ctx := context.Background()
	svc := newPushService(t, &recordingInvoker{})
	a := "user-a-" + randomCNPJ()
	b := "user-b-" + randomCNPJ()
	company := seedPushCompany(t, ctx, a)
	endpoint := "https://fcm.googleapis.com/fcm/send/" + strings.Repeat("b", 40)
	key := ecdhP256(t)
	in := services.PushSubscriptionInput{Endpoint: endpoint, P256dh: key.p256dh, Auth: key.auth, UserAgent: "Mozilla/5.0 (Windows NT 10.0) Chrome/130 Safari/537.36"}

	if _, err := svc.Subscribe(ctx, company, a, in); err != nil {
		t.Fatal(err)
	}
	if devices, err := svc.Devices(ctx, a); err != nil || len(devices) != 1 {
		t.Fatalf("a devices after subscribing = %d (%v), want 1", len(devices), err)
	}

	if _, err := svc.Subscribe(ctx, company, b, in); err != nil {
		t.Fatal(err)
	}
	if devices, _ := svc.Devices(ctx, a); len(devices) != 0 {
		t.Errorf("the previous person still holds the endpoint: %d device(s)", len(devices))
	}
	if devices, _ := svc.Devices(ctx, b); len(devices) != 1 {
		t.Errorf("the current person devices = %d, want 1", len(devices))
	}
	// A renewal by the current owner is not a takeover: it keeps its single row.
	if _, err := svc.Subscribe(ctx, company, b, in); err != nil {
		t.Fatal(err)
	}
	if devices, _ := svc.Devices(ctx, b); len(devices) != 1 {
		t.Errorf("renewal changed the device count: %d", len(devices))
	}
	// The slot the previous person held was freed: they can subscribe the same endpoint back and take it again.
	if _, err := svc.Subscribe(ctx, company, a, in); err != nil {
		t.Fatal(err)
	}
	if devices, _ := svc.Devices(ctx, b); len(devices) != 0 {
		t.Errorf("signing back in must move the endpoint again: b still has %d", len(devices))
	}
}

// The test notification is limited to one a minute per person.
func TestPushService_TestNotificationIsRateLimited(t *testing.T) {
	ctx := context.Background()
	inv := &recordingInvoker{}
	svc := newPushService(t, inv)
	a := "user-a-" + randomCNPJ()
	if err := svc.Test(ctx, "CNPJ_1", a); err != nil {
		t.Fatal(err)
	}
	if err := svc.Test(ctx, "CNPJ_1", a); err == nil {
		t.Error("a second test inside the minute must be refused")
	}
	if len(inv.taken()) != 1 {
		t.Errorf("jobs = %d, want 1", len(inv.taken()))
	}
}

// Off (no key or no function) is quiet: reads say disabled, writes 503, Emit does nothing and does not fail.
func TestPushService_OffIsInert(t *testing.T) {
	ctx := context.Background()
	inv := &recordingInvoker{}
	svc := services.NewPushService(newPushRepo(), memberSvc, orgSvc, nil, "", "").WithInvoker(inv)
	if svc.Enabled() || svc.Config().Enabled || svc.Config().PublicKey != "" {
		t.Error("a service without keys must be off and publish no key")
	}
	if _, err := svc.Subscribe(ctx, "CNPJ_1", "u", services.PushSubscriptionInput{}); err == nil {
		t.Error("subscribing while off must fail")
	}
	if err := svc.EmitResult(ctx, services.PushResult{CompanyPK: "CNPJ_1", DocumentKey: "k", Table: "nfes", Status: "rejected"}); err != nil {
		t.Errorf("emit while off must not fail: %v", err)
	}
	if err := svc.EmitDistribution(ctx, "CNPJ_1", "k", "nfe", time.Now()); err != nil || len(inv.taken()) != 0 {
		t.Errorf("distribution while off: %v %d", err, len(inv.taken()))
	}
}

type browserKeys struct{ p256dh, auth string }

// ecdhP256 makes the keys a browser would send: an uncompressed P-256 point and a 16-byte secret.
func ecdhP256(t *testing.T) browserKeys {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	auth := make([]byte, 16)
	if _, err := rand.Read(auth); err != nil {
		t.Fatal(err)
	}
	return browserKeys{
		p256dh: base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()),
		auth:   base64.RawURLEncoding.EncodeToString(auth),
	}
}

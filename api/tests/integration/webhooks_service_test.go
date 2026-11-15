//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gopkg.aoctech.app/api-commons/webhooks"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

type fakeVerifier struct {
	err   error
	calls []string
}

func (f *fakeVerifier) Verify(_ context.Context, url string) error {
	f.calls = append(f.calls, url)
	return f.err
}

type webhookEnv struct {
	svc      *services.WebhookService
	repo     *repositories.WebhookRepository
	verifier *fakeVerifier
	org      string
	company  string
}

func newWebhookEnv(t *testing.T, deliver *webhooks.Deliverer) *webhookEnv {
	t.Helper()
	repo := repositories.NewWebhookRepository(db, cfg)
	billing := services.NewBillingService(repositories.NewAccountBillingRepository(db, cfg), nil, nil, orgSvc, memCache).
		WithWorkspaceRoles(fakeRoles{roles: map[string]string{"usr-admin": services.AccountRoleAdmin, "usr-member": "member"}})
	v := &fakeVerifier{}
	org := "org-hook-" + newCompanyPK(t)
	return &webhookEnv{
		svc: services.NewWebhookService(repo, billing, v, deliver), repo: repo, verifier: v, org: org,
		company: seedCompany(t, org, "usr-owner", "11222333000181", "Hook Ltda"),
	}
}

func TestWebhookRegisterRefusesBadURLsWithoutAnyWrite(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	for _, url := range []string{"http://hooks.example.com/x", "https://127.0.0.1/x", "https://10.0.0.5/x", "https://intranet/x", "not a url"} {
		_, err := e.svc.Register(ctx, e.company, "usr-admin", services.WebhookRegistration{URL: url})
		if !isStatus(err, http.StatusBadRequest) {
			t.Errorf("%s: %v, want 400", url, err)
		}
	}
	if len(e.verifier.calls) != 0 {
		t.Fatalf("the challenge was sent for a refused URL: %v", e.verifier.calls)
	}
	if list, _ := e.repo.ListEndpoints(ctx, e.org); len(list) != 0 {
		t.Fatalf("stored %d endpoints", len(list))
	}
}

func TestWebhookRegisterStoresOnlyAfterTheChallengeAndShowsTheSecretOnce(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	e.verifier.err = errors.New("the endpoint did not return the hub.challenge value")
	_, err := e.svc.Register(ctx, e.company, "usr-admin", services.WebhookRegistration{URL: "https://hooks.example.com/dfe"})
	if !isStatus(err, http.StatusUnprocessableEntity) || !strings.Contains(err.Error(), "hub.challenge") {
		t.Fatalf("failed challenge: %v, want 422 carrying the reason", err)
	}
	if list, _ := e.repo.ListEndpoints(ctx, e.org); len(list) != 0 {
		t.Fatal("an endpoint that failed the challenge was stored")
	}

	e.verifier.err = nil
	created, err := e.svc.Register(ctx, e.company, "usr-admin", services.WebhookRegistration{
		URL: "https://hooks.example.com/dfe", Events: []string{"document.rejected"}, CompanyPKs: []string{e.company},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(created.Secret, "whsec_") || created.Status != repositories.WebhookActive {
		t.Fatalf("created = %+v", created)
	}
	list, err := e.svc.List(ctx, e.company, "usr-admin")
	if err != nil || len(list) != 1 {
		t.Fatalf("list = %v %v", list, err)
	}
	raw, _ := json.Marshal(list)
	if strings.Contains(string(raw), "whsec_") || strings.Contains(string(raw), "secret") {
		t.Fatalf("the list carries the secret: %s", raw)
	}
}

func TestWebhookRegisterValidatesEventsAndCompanies(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	other := seedCompany(t, "org-other-"+newCompanyPK(t), "usr-x", "44555666000105", "Outra Ltda")
	for name, in := range map[string]services.WebhookRegistration{
		"unknown event":          {URL: "https://hooks.example.com/a", Events: []string{"document.exploded"}},
		"company of another org": {URL: "https://hooks.example.com/a", CompanyPKs: []string{other}},
	} {
		if _, err := e.svc.Register(ctx, e.company, "usr-admin", in); !isStatus(err, http.StatusBadRequest) {
			t.Errorf("%s: %v, want 400", name, err)
		}
	}
	if len(e.verifier.calls) != 0 {
		t.Fatal("the challenge was sent for a refused registration")
	}
}

func TestWebhookOnlyOwnersAndAdminsManage(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	ep, err := e.svc.Register(ctx, e.company, "usr-admin", services.WebhookRegistration{URL: "https://hooks.example.com/dfe"})
	if err != nil {
		t.Fatal(err)
	}
	m := "usr-member"
	checks := map[string]error{}
	_, checks["register"] = e.svc.Register(ctx, e.company, m, services.WebhookRegistration{URL: "https://hooks.example.com/b"})
	_, checks["list"] = e.svc.List(ctx, e.company, m)
	checks["delete"] = e.svc.Delete(ctx, e.company, m, ep.ID)
	_, checks["rotate"] = e.svc.RotateSecret(ctx, e.company, m, ep.ID)
	checks["ping"] = e.svc.Ping(ctx, e.company, m, ep.ID)
	checks["replay"] = e.svc.Replay(ctx, e.company, m, ep.ID, "ev")
	_, _, checks["deliveries"] = e.svc.Deliveries(ctx, e.company, m, ep.ID, nil, 10)
	for name, err := range checks {
		if !isStatus(err, http.StatusForbidden) {
			t.Errorf("member %s: %v, want 403", name, err)
		}
	}
}

func TestWebhookRegisterRefusesWhenNoSecretKeyIsConfigured(t *testing.T) {
	ctx := context.Background()
	noKey := *cfg
	noKey.WebhookSecretKey = ""
	repo := repositories.NewWebhookRepository(db, &noKey)
	billing := services.NewBillingService(repositories.NewAccountBillingRepository(db, cfg), nil, nil, orgSvc, memCache).
		WithWorkspaceRoles(fakeRoles{roles: map[string]string{"usr-admin": services.AccountRoleAdmin}})
	org := "org-nokey-" + newCompanyPK(t)
	company := seedCompany(t, org, "usr-owner", "11222333000181", "Sem Chave Ltda")
	v := &fakeVerifier{}
	_, err := services.NewWebhookService(repo, billing, v, nil).Register(ctx, company, "usr-admin", services.WebhookRegistration{URL: "https://hooks.example.com/a"})
	if !isStatus(err, http.StatusNotImplemented) {
		t.Fatalf("err = %v, want 501", err)
	}
	if len(v.calls) != 0 {
		t.Fatal("the challenge was sent with no way to store the secret")
	}
}

// receiver is a TLS endpoint that records what it was sent.
type receiver struct {
	mu   sync.Mutex
	hits []http.Header
	body [][]byte
}

func (r *receiver) server(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		buf := make([]byte, 4096)
		n, _ := req.Body.Read(buf)
		r.mu.Lock()
		r.hits = append(r.hits, req.Header.Clone())
		r.body = append(r.body, buf[:n])
		r.mu.Unlock()
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestWebhookPingIsSignedAndDeliveredAtOnce(t *testing.T) {
	ctx := context.Background()
	rcv := &receiver{}
	srv := rcv.server(t)
	repo := repositories.NewWebhookRepository(db, cfg)
	store := repositories.NewWebhookStore(repo, nil)
	engine := webhooks.NewDeliverer(store, webhooks.Config{HeaderPrefix: services.WebhookHeaderPrefix, UserAgent: "test/1", Client: srv.Client()})
	e := newWebhookEnv(t, engine)
	ep := &repositories.WebhookEndpoint{ID: repositories.GenerateID(), OrganizationID: e.org, URL: srv.URL, Secret: webhooks.NewSecret(), Status: repositories.WebhookActive}
	if err := e.repo.CreateEndpoint(ctx, ep, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Ping(ctx, e.company, "usr-admin", ep.ID); err != nil {
		t.Fatal(err)
	}
	if len(rcv.hits) != 1 {
		t.Fatalf("receiver got %d requests, want 1", len(rcv.hits))
	}
	h := rcv.hits[0]
	if err := webhooks.Verify(ep.Secret, h.Get("X-Ctech-Timestamp"), rcv.body[0], h.Get("X-Ctech-Signature"), time.Now(), time.Minute); err != nil {
		t.Fatalf("signature: %v", err)
	}
	if h.Get("X-Ctech-Event-Type") != "ping" {
		t.Fatalf("event type = %q", h.Get("X-Ctech-Event-Type"))
	}
	page, _, _ := e.svc.Deliveries(ctx, e.company, "usr-admin", ep.ID, nil, 10)
	if len(page) != 1 || page[0].Status != repositories.DeliveryDelivered {
		t.Fatalf("deliveries = %+v", page)
	}
}

func TestWebhookReplayRefusesADisabledEndpointAndSendsNothing(t *testing.T) {
	ctx := context.Background()
	rcv := &receiver{}
	srv := rcv.server(t)
	repo := repositories.NewWebhookRepository(db, cfg)
	engine := webhooks.NewDeliverer(repositories.NewWebhookStore(repo, nil), webhooks.Config{HeaderPrefix: services.WebhookHeaderPrefix, Client: srv.Client()})
	e := newWebhookEnv(t, engine)
	ep := &repositories.WebhookEndpoint{ID: repositories.GenerateID(), OrganizationID: e.org, URL: srv.URL, Secret: webhooks.NewSecret(), Status: repositories.WebhookDisabled}
	if err := e.repo.CreateEndpoint(ctx, ep, time.Now()); err != nil {
		t.Fatal(err)
	}
	evID := repositories.GenerateID()
	list, _ := e.repo.ListEndpoints(ctx, e.org)
	active := list[0]
	active.Status = repositories.WebhookActive
	if _, err := e.repo.WriteDeliveries(ctx, e.org, repositories.WebhookEvent{ID: evID, Type: "document.failed", Body: []byte("{}")}, []repositories.WebhookEndpoint{active}, time.Now()); err != nil {
		t.Fatal(err)
	}
	err := e.svc.Replay(ctx, e.company, "usr-admin", ep.ID, evID)
	if !isStatus(err, http.StatusConflict) {
		t.Fatalf("err = %v, want 409", err)
	}
	if len(rcv.hits) != 0 {
		t.Fatal("a replay reached a disabled endpoint")
	}
	if err := e.svc.Replay(ctx, e.company, "usr-admin", ep.ID, "no-such-event"); !isStatus(err, http.StatusNotFound) {
		t.Fatalf("unknown delivery: %v, want 404", err)
	}
}

func TestWebhookRotateReturnsANewSecretThatSigns(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	ep, err := e.svc.Register(ctx, e.company, "usr-admin", services.WebhookRegistration{URL: "https://hooks.example.com/dfe"})
	if err != nil {
		t.Fatal(err)
	}
	next, err := e.svc.RotateSecret(ctx, e.company, "usr-admin", ep.ID)
	if err != nil || next == ep.Secret || !strings.HasPrefix(next, "whsec_") {
		t.Fatalf("rotated = %q %v", next, err)
	}
	got, _ := e.repo.GetEndpoint(ctx, e.org, ep.ID)
	if got.Secret != next {
		t.Fatal("the stored secret is not the rotated one")
	}
	if err := e.svc.Delete(ctx, e.company, "usr-admin", ep.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.Delete(ctx, e.company, "usr-admin", ep.ID); !isStatus(err, http.StatusNotFound) {
		t.Fatalf("second delete: %v, want 404", err)
	}
}

func TestWebhookEmitWritesOnlyForWantingEndpointsAndIsIdempotent(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	mk := func(events, companies []string) *repositories.WebhookEndpoint {
		ep := &repositories.WebhookEndpoint{ID: repositories.GenerateID(), OrganizationID: e.org, URL: "https://hooks.invalid/x",
			Secret: webhooks.NewSecret(), Status: repositories.WebhookActive, Events: events, CompanyPKs: companies}
		if err := e.repo.CreateEndpoint(ctx, ep, time.Now()); err != nil {
			t.Fatal(err)
		}
		return ep
	}
	all := mk(nil, nil)
	onlyRejected := mk([]string{services.WebhookEventRejected}, nil)
	otherCompany := mk(nil, []string{"another-company"})

	in := services.WebhookResult{
		CompanyPK: e.company, OrganizationID: e.org, DocumentKey: "35250512345678000195550010000000011000000015",
		Table: "nfes", Status: "rejected", SefazStatus: "225", SefazMotive: "Falha no Schema XML", SentAt: time.UnixMilli(1800000000123),
	}
	for i := 0; i < 2; i++ { // a redelivered SQS message repeats the call
		if err := e.svc.Emit(ctx, in); err != nil {
			t.Fatal(err)
		}
	}
	count := func(ep *repositories.WebhookEndpoint) int {
		page, _, _ := e.repo.ListDeliveries(ctx, e.org, ep.ID, nil, 50)
		return len(page)
	}
	if count(all) != 1 || count(onlyRejected) != 1 || count(otherCompany) != 0 {
		t.Fatalf("deliveries all=%d onlyRejected=%d otherCompany=%d, want 1 1 0 (the repeat must not add one)",
			count(all), count(onlyRejected), count(otherCompany))
	}

	// An authorization is not for the endpoint that only wants rejections.
	in.Status, in.SefazStatus, in.SefazMotive = "authorized", "100", "Autorizado o uso da NF-e"
	if err := e.svc.Emit(ctx, in); err != nil {
		t.Fatal(err)
	}
	if count(all) != 2 || count(onlyRejected) != 1 {
		t.Fatalf("after authorization all=%d onlyRejected=%d, want 2 1", count(all), count(onlyRejected))
	}

	// The body is the minimum and carries the rejection only when there is one.
	page, _, _ := e.repo.ListDeliveries(ctx, e.org, onlyRejected.ID, nil, 50)
	due := repositories.DueOf(page[0])
	var body map[string]any
	if err := json.Unmarshal(due.Event.Body, &body); err != nil {
		t.Fatal(err)
	}
	data := body["data"].(map[string]any)
	if body["type"] != "document.rejected" || data["id"] != in.DocumentKey || data["access_key"] != in.DocumentKey ||
		data["status"] != "rejected" || data["document_type"] != "nfe" {
		t.Fatalf("body = %v", body)
	}
	if rej := data["rejection"].(map[string]any); rej["code"] != "225" || rej["message"] != "Falha no Schema XML" {
		t.Fatalf("rejection = %v", rej)
	}
	if len(data) != 5 {
		t.Fatalf("data has %d fields, want exactly id, access_key, status, document_type, rejection: %v", len(data), data)
	}
}

func TestWebhookEmitSkipsWhenNoEndpointOrNoKey(t *testing.T) {
	ctx := context.Background()
	e := newWebhookEnv(t, nil)
	in := services.WebhookResult{CompanyPK: e.company, OrganizationID: e.org, DocumentKey: "k", Table: "nfes", Status: "authorized", SentAt: time.Now()}
	if err := e.svc.Emit(ctx, in); err != nil { // nobody subscribed: nothing to write, nothing to fail
		t.Fatal(err)
	}
	noKey := *cfg
	noKey.WebhookSecretKey = ""
	svc := services.NewWebhookService(repositories.NewWebhookRepository(db, &noKey), nil, e.verifier, nil)
	if err := svc.Emit(ctx, in); err != nil {
		t.Fatalf("emit with webhooks off: %v", err)
	}
}

func TestWebhookEventIDIsStableAndSortsByTime(t *testing.T) {
	a := repositories.WebhookEventID(time.UnixMilli(1800000000000), "doc|authorized")
	b := repositories.WebhookEventID(time.UnixMilli(1800000000000), "doc|authorized")
	later := repositories.WebhookEventID(time.UnixMilli(1800000005000), "doc|authorized")
	other := repositories.WebhookEventID(time.UnixMilli(1800000000000), "doc|rejected")
	if a != b || a == other || !(a < later) {
		t.Fatalf("a=%s b=%s later=%s other=%s", a, b, later, other)
	}
}

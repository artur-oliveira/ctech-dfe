//go:build integration

package integration_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.aoctech.app/api-commons/webhooks"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
)

// The whole path in one test: a terminal document result becomes an event, the
// delivery engine sends it signed, a receiver that keeps failing is disabled after
// twelve failures in a row, and from then on nothing more is sent to it.
func TestWebhookFlowFromResultToDisabledEndpoint(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	var hits []http.Header
	var bodies [][]byte
	var failing atomic.Bool
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		mu.Lock()
		hits = append(hits, r.Header.Clone())
		bodies = append(bodies, buf[:n])
		mu.Unlock()
		if failing.Load() {
			http.Error(w, "down", http.StatusInternalServerError)
		}
	}))
	t.Cleanup(srv.Close)
	count := func() int { mu.Lock(); defer mu.Unlock(); return len(hits) }

	repo := repositories.NewWebhookRepository(db, cfg)
	var disabled atomic.Int32
	store := repositories.NewWebhookStore(repo, func(repositories.WebhookEndpoint) { disabled.Add(1) })
	engine := webhooks.NewDeliverer(store, webhooks.Config{HeaderPrefix: services.WebhookHeaderPrefix, UserAgent: "test/1", Client: srv.Client(), Workers: 2})

	e := newWebhookEnv(t, engine)
	ep := &repositories.WebhookEndpoint{ID: repositories.GenerateID(), OrganizationID: e.org, URL: srv.URL, Secret: webhooks.NewSecret(), Status: repositories.WebhookActive}
	if err := e.repo.CreateEndpoint(ctx, ep, time.Now()); err != nil {
		t.Fatal(err)
	}

	// A pass runs at the real time (the Lambda calls time.Now()); `later` moves it past a backoff.
	later := func(d time.Duration) time.Time { return time.Now().Add(d) }
	result := func(key, status string) services.WebhookResult {
		return services.WebhookResult{CompanyPK: e.company, OrganizationID: e.org, DocumentKey: key, Table: "nfes", Status: status,
			SefazStatus: "225", SefazMotive: "Falha no Schema XML", SentAt: time.Now()}
	}

	// 1. A rejection becomes one delivery, sent signed with the minimum body.
	if err := e.svc.Emit(ctx, result("35250512345678000195550010000000011000000015", "rejected")); err != nil {
		t.Fatal(err)
	}
	if res := engine.Deliver(ctx, 100, later(time.Second)); res.Handled != 1 || res.Failed != 0 {
		t.Fatalf("first pass = %+v", res)
	}
	if count() != 1 {
		t.Fatalf("receiver got %d requests, want 1", count())
	}
	h := hits[0]
	if err := webhooks.Verify(ep.Secret, h.Get("X-Ctech-Timestamp"), bodies[0], h.Get("X-Ctech-Signature"), time.Now(), time.Minute); err != nil {
		t.Fatalf("signature: %v", err)
	}
	var body map[string]any
	_ = json.Unmarshal(bodies[0], &body)
	if body["type"] != "document.rejected" || body["id"] != h.Get("X-Ctech-Event-Id") {
		t.Fatalf("body = %v, headers = %v", body, h)
	}

	// 2. The receiver goes down. One failure per pass (the rest of its queue waits), so twelve passes
	//    spaced past the backoff add up to twelve in a row.
	failing.Store(true)
	for i := 0; i < 5; i++ {
		if err := e.svc.Emit(ctx, result("key-"+string(rune('a'+i)), "authorized")); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= webhooks.MaxFailureRun; i++ {
		engine.Deliver(ctx, 100, later(time.Duration(i)*13*time.Hour))
	}
	got, err := e.repo.GetEndpoint(ctx, e.org, ep.ID)
	if err != nil || got.Status != repositories.WebhookDisabled || got.FailureRun != webhooks.MaxFailureRun {
		t.Fatalf("endpoint = %+v %v, want disabled after %d failures", got, err, webhooks.MaxFailureRun)
	}
	if disabled.Load() != 1 {
		t.Fatalf("operator alert fired %d times, want once", disabled.Load())
	}

	// 3. Disabled: replays are refused and nothing is sent, not even what was still queued.
	before := count()
	page, _, _ := e.repo.ListDeliveries(ctx, e.org, ep.ID, nil, 50)
	if err := e.svc.Replay(ctx, e.company, "usr-admin", ep.ID, page[0].EventID); !isStatus(err, http.StatusConflict) {
		t.Fatalf("replay = %v, want 409", err)
	}
	engine.Deliver(ctx, 100, later(30*24*time.Hour))
	if count() != before {
		t.Fatalf("a disabled endpoint was still sent %d requests", count()-before)
	}
}

// I1: a tenant with a deep backlog and a receiver that answers 200 must not fill the whole batch, run
// after run: one pass gives an endpoint a bounded number of attempts and moves the rest down the queue.
func TestWebhookEngineCapsAttemptsPerEndpointPerPass(t *testing.T) {
	ctx := context.Background()
	var hits atomic.Int32
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	t.Cleanup(srv.Close)
	repo := repositories.NewWebhookRepository(db, cfg)
	engine := services.NewWebhookEngine(repo, nil, 1, webhooks.DeliveryTimeout, func(c *webhooks.Config) { c.Client = srv.Client() })

	e := newWebhookEnv(t, nil)
	ep := &repositories.WebhookEndpoint{ID: repositories.GenerateID(), OrganizationID: e.org, URL: srv.URL, Secret: webhooks.NewSecret(), Status: repositories.WebhookActive}
	if err := e.repo.CreateEndpoint(ctx, ep, time.Now()); err != nil {
		t.Fatal(err)
	}
	list, _ := e.repo.ListEndpoints(ctx, e.org)
	total := services.WebhookMaxPerEndpointPerPass + 10
	for i := 0; i < total; i++ {
		ev := repositories.WebhookEvent{ID: repositories.GenerateID(), Type: services.WebhookEventAuthorized, Body: []byte("{}")}
		if _, err := e.repo.WriteDeliveries(ctx, e.org, ev, list, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	res := engine.Deliver(ctx, 1000, time.Now().Add(time.Second))
	if int(hits.Load()) != services.WebhookMaxPerEndpointPerPass || res.Deferred < 10 {
		t.Fatalf("hits=%d res=%+v, want %d attempts and the rest deferred", hits.Load(), res, services.WebhookMaxPerEndpointPerPass)
	}
}

// I2: ping, replay and the registration challenge run inside the request, whose deadline (REQUEST_TIMEOUT, 8 s by
// default) is shorter than the Lambda's attempt timeout. A slower receiver would leave the writes after the attempt
// with an expired context: the attempt unlogged, the endpoint's health uncounted, a generic 500 for the person.
func TestWebhookAPIAttemptTimeoutFitsInsideTheRequestDeadline(t *testing.T) {
	const defaultRequestTimeout = 8 * time.Second
	if services.WebhookAPIAttemptTimeout+2*time.Second > defaultRequestTimeout {
		t.Fatalf("API attempt timeout %v leaves under 2 s of the %v request deadline for recording the attempt",
			services.WebhookAPIAttemptTimeout, defaultRequestTimeout)
	}
}

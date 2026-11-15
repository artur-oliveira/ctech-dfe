//go:build integration

package integration_test

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"gopkg.aoctech.app/api-commons/webhooks"
	"gopkg.aoctech.app/dfe/api/internal/repositories"
)

func newWebhookRepo() (*repositories.WebhookRepository, webhooks.Store) {
	repo := repositories.NewWebhookRepository(db, cfg)
	return repo, repositories.NewWebhookStore(repo, nil)
}

func webhookEndpoint(org string) *repositories.WebhookEndpoint {
	return &repositories.WebhookEndpoint{
		ID: repositories.GenerateID(), OrganizationID: org, URL: "https://hooks.invalid/dfe",
		Secret: webhooks.NewSecret(), Status: repositories.WebhookActive, CreatedBy: "usr_1",
	}
}

func rawWebhookItem(t *testing.T, ctx context.Context, org, sk string) map[string]types.AttributeValue {
	t.Helper()
	out, err := db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tablePrefix + "_" + repositories.TableWebhooks),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: repositories.WebhookPK(org)},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil || out.Item == nil {
		t.Fatalf("raw get: %v %v", out, err)
	}
	return out.Item
}

func putRawWebhookSecret(t *testing.T, ctx context.Context, org, sk string, secret types.AttributeValue) {
	t.Helper()
	_, err := db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tablePrefix + "_" + repositories.TableWebhooks),
		Key: map[string]types.AttributeValue{
			"pk": &types.AttributeValueMemberS{Value: repositories.WebhookPK(org)},
			"sk": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression:          aws.String("SET secret = :s"),
		ExpressionAttributeValues: map[string]types.AttributeValue{":s": secret},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func orgID() string { return "org_" + repositories.GenerateID() }

func TestWebhookEndpointSecretIsSealedAtRestAndBoundToItsRow(t *testing.T) {
	ctx := context.Background()
	repo, _ := newWebhookRepo()
	org := orgID()
	a, b := webhookEndpoint(org), webhookEndpoint(org)
	for _, e := range []*repositories.WebhookEndpoint{a, b} {
		if err := repo.CreateEndpoint(ctx, e, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	raw := rawWebhookItem(t, ctx, org, repositories.WebhookEndpointSK(a.ID))
	if strings.Contains(raw["secret"].(*types.AttributeValueMemberS).Value, a.Secret) {
		t.Fatal("the signing secret is stored in clear")
	}
	got, err := repo.GetEndpoint(ctx, org, a.ID)
	if err != nil || got.Secret != a.Secret {
		t.Fatalf("round trip: %v %q", err, got.Secret)
	}
	// The sealed value of A copied onto B must not open there.
	putRawWebhookSecret(t, ctx, org, repositories.WebhookEndpointSK(b.ID), raw["secret"])
	if _, err := repo.GetEndpoint(ctx, org, b.ID); err == nil {
		t.Fatal("a secret sealed for another endpoint opened")
	}
}

func TestWebhookEndpointLimitHoldsUnderConcurrency(t *testing.T) {
	ctx := context.Background()
	repo, _ := newWebhookRepo()
	org := orgID()
	var wg sync.WaitGroup
	var mu sync.Mutex
	created := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.CreateEndpoint(ctx, webhookEndpoint(org), time.Now()); err == nil {
				mu.Lock()
				created++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	list, err := repo.ListEndpoints(ctx, org)
	if err != nil || created != repositories.MaxWebhookEndpoints || len(list) != created {
		t.Fatalf("created=%d listed=%d err=%v, want %d", created, len(list), err, repositories.MaxWebhookEndpoints)
	}
	// Deleting one frees a slot.
	if err := repo.DeleteEndpoint(ctx, org, list[0].ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateEndpoint(ctx, webhookEndpoint(org), time.Now()); err != nil {
		t.Fatalf("after delete: %v", err)
	}
}

func TestWebhookEventFansOutOnlyToWantingEndpoints(t *testing.T) {
	ctx := context.Background()
	repo, store := newWebhookRepo()
	org := orgID()
	all, onlyRejected, otherCompany, disabled := webhookEndpoint(org), webhookEndpoint(org), webhookEndpoint(org), webhookEndpoint(org)
	onlyRejected.Events = []string{"document.rejected"}
	otherCompany.CompanyPKs = []string{"ORG#other"}
	disabled.Status = repositories.WebhookDisabled
	eps := []*repositories.WebhookEndpoint{all, onlyRejected, otherCompany, disabled}
	for _, e := range eps {
		if err := repo.CreateEndpoint(ctx, e, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := repo.ListEndpoints(ctx, org)
	n, err := repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{
		ID: repositories.GenerateID(), Type: "document.authorized", CompanyPK: "ORG#mine", Body: []byte(`{"id":"e"}`),
	}, list, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("authorized wrote %d (%v), want only the unfiltered active endpoint", n, err)
	}
	due, _ := store.DueDeliveries(ctx, 100)
	found := 0
	for _, d := range due {
		if d.Delivery.EndpointID == all.ID {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("due deliveries for the wanting endpoint = %d", found)
	}
}

func TestWebhookEventWithNoWantingEndpointWritesNothing(t *testing.T) {
	ctx := context.Background()
	repo, _ := newWebhookRepo()
	org := orgID()
	n, err := repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{ID: repositories.GenerateID(), Type: "document.failed"}, nil, time.Now())
	if err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	page, _, err := repo.ListDeliveries(ctx, org, "any", nil, 10)
	if err != nil || len(page) != 0 {
		t.Fatalf("deliveries = %v %v", page, err)
	}
}

func TestWebhookPingGoesOnlyToItsEndpoint(t *testing.T) {
	ctx := context.Background()
	repo, _ := newWebhookRepo()
	org := orgID()
	a, b := webhookEndpoint(org), webhookEndpoint(org)
	a.Events = []string{"document.rejected"} // a ping ignores the filter
	for _, e := range []*repositories.WebhookEndpoint{a, b} {
		if err := repo.CreateEndpoint(ctx, e, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	list, _ := repo.ListEndpoints(ctx, org)
	n, err := repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{ID: repositories.GenerateID(), Type: repositories.WebhookPing, EndpointID: a.ID, Body: []byte("{}")}, list, time.Now())
	if err != nil || n != 1 {
		t.Fatalf("ping wrote %d (%v)", n, err)
	}
	pa, _, _ := repo.ListDeliveries(ctx, org, a.ID, nil, 10)
	pb, _, _ := repo.ListDeliveries(ctx, org, b.ID, nil, 10)
	if len(pa) != 1 || len(pb) != 0 {
		t.Fatalf("a=%d b=%d", len(pa), len(pb))
	}
}

func TestWebhookHealthDisablesAtTwelveAndSuccessResets(t *testing.T) {
	ctx := context.Background()
	repo, store := newWebhookRepo()
	org := orgID()
	e := webhookEndpoint(org)
	if err := repo.CreateEndpoint(ctx, e, time.Now()); err != nil {
		t.Fatal(err)
	}
	d := webhooks.Delivery{EndpointID: e.ID, Ref: repositories.WebhookDelivery{OrganizationID: org, EndpointID: e.ID}}
	fail := func() {
		ep, err := store.GetEndpoint(ctx, d)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.RecordHealth(ctx, *ep, false, "HTTP 500", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < webhooks.MaxFailureRun-1; i++ {
		fail()
	}
	ep, _ := store.GetEndpoint(ctx, d)
	if !ep.Active {
		t.Fatal("disabled before the 12th consecutive failure")
	}
	if err := store.RecordHealth(ctx, *ep, true, "", time.Now()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < webhooks.MaxFailureRun-1; i++ {
		fail()
	}
	if ep, _ = store.GetEndpoint(ctx, d); !ep.Active {
		t.Fatal("a success did not reset the failure run")
	}
	fail()
	if ep, _ = store.GetEndpoint(ctx, d); ep.Active {
		t.Fatal("still active after 12 consecutive failures")
	}
}

func TestWebhookDeliveryLifecycleLeavesTheDueIndex(t *testing.T) {
	ctx := context.Background()
	repo, store := newWebhookRepo()
	org := orgID()
	e := webhookEndpoint(org)
	if err := repo.CreateEndpoint(ctx, e, time.Now()); err != nil {
		t.Fatal(err)
	}
	list, _ := repo.ListEndpoints(ctx, org)
	evID := repositories.GenerateID()
	if _, err := repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{ID: evID, Type: "document.authorized", Body: []byte(`{"id":"x"}`)}, list, time.Now()); err != nil {
		t.Fatal(err)
	}
	mine := func() *webhooks.Due {
		due, _ := store.DueDeliveries(ctx, 1000)
		for i := range due {
			if due[i].Delivery.EventID == evID {
				return &due[i]
			}
		}
		return nil
	}
	d := mine()
	if d == nil || string(d.Event.Body) != `{"id":"x"}` || d.Event.Type != "document.authorized" {
		t.Fatalf("due = %+v", d)
	}
	now := time.Now()
	// A failure reschedules with backoff and counts the attempt.
	if err := store.MarkAttemptFailed(ctx, d.Delivery, 500, "boom", now); err != nil {
		t.Fatal(err)
	}
	d2 := mine()
	if d2 == nil || d2.Delivery.Attempts != 1 || d2.Delivery.NextAttemptAt <= now.UTC().Format(time.RFC3339Nano) {
		t.Fatalf("after failure = %+v", d2)
	}
	// Delivered leaves the index for good.
	if err := store.MarkDelivered(ctx, d2.Delivery, 200, now); err != nil {
		t.Fatal(err)
	}
	if mine() != nil {
		t.Fatal("a delivered delivery is still in the due index")
	}
	// Replay puts it back, keeping its attempt count.
	if err := store.Requeue(ctx, d2.Delivery, now); err != nil {
		t.Fatal(err)
	}
	if d3 := mine(); d3 == nil || d3.Delivery.Attempts != 2 {
		t.Fatalf("after requeue = %+v", d3)
	}
}

func TestWebhookDeliveryGivesUpAfterMaxAttempts(t *testing.T) {
	ctx := context.Background()
	repo, store := newWebhookRepo()
	org := orgID()
	e := webhookEndpoint(org)
	_ = repo.CreateEndpoint(ctx, e, time.Now())
	list, _ := repo.ListEndpoints(ctx, org)
	evID := repositories.GenerateID()
	_, _ = repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{ID: evID, Type: "document.failed", Body: []byte("{}")}, list, time.Now())
	due, _ := store.DueDeliveries(ctx, 1000)
	var d webhooks.Delivery
	for _, x := range due {
		if x.Delivery.EventID == evID {
			d = x.Delivery
		}
	}
	for i := 0; i < webhooks.MaxDeliveryAttempts; i++ {
		d.Attempts = i
		if err := store.MarkAttemptFailed(ctx, d, 500, "boom", time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	page, _, err := repo.ListDeliveries(ctx, org, e.ID, nil, 10)
	if err != nil || len(page) != 1 || page[0].Status != repositories.DeliveryFailed || page[0].Attempts != webhooks.MaxDeliveryAttempts {
		t.Fatalf("delivery = %+v %v", page, err)
	}
}

func TestWebhookDeliveriesListNewestFirstWithACursor(t *testing.T) {
	ctx := context.Background()
	repo, _ := newWebhookRepo()
	org := orgID()
	e := webhookEndpoint(org)
	_ = repo.CreateEndpoint(ctx, e, time.Now())
	list, _ := repo.ListEndpoints(ctx, org)
	var ids []string
	for i := 0; i < 5; i++ {
		id := repositories.GenerateID()
		ids = append(ids, id)
		if _, err := repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{ID: id, Type: "document.authorized", Body: []byte("{}")}, list, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	p1, cur, err := repo.ListDeliveries(ctx, org, e.ID, nil, 3)
	if err != nil || len(p1) != 3 || cur == nil || p1[0].EventID != ids[4] {
		t.Fatalf("page 1 = %v cursor=%v err=%v", p1, cur, err)
	}
	p2, cur2, err := repo.ListDeliveries(ctx, org, e.ID, cur, 3)
	if err != nil || len(p2) != 2 || cur2 != nil || p2[1].EventID != ids[0] {
		t.Fatalf("page 2 = %v cursor=%v err=%v", p2, cur2, err)
	}
}

func TestWebhookAttemptsAreKeptAndIdempotentOnTheNumber(t *testing.T) {
	ctx := context.Background()
	repo, store := newWebhookRepo()
	org := orgID()
	e := webhookEndpoint(org)
	_ = repo.CreateEndpoint(ctx, e, time.Now())
	list, _ := repo.ListEndpoints(ctx, org)
	evID := repositories.GenerateID()
	_, _ = repo.WriteDeliveries(ctx, org, repositories.WebhookEvent{ID: evID, Type: "document.authorized", Body: []byte("{}")}, list, time.Now())
	due, _ := store.DueDeliveries(ctx, 1000)
	var d webhooks.Delivery
	for _, x := range due {
		if x.Delivery.EventID == evID {
			d = x.Delivery
		}
	}
	for _, a := range []webhooks.Attempt{{Number: 1, StatusCode: 502, Response: "gateway", Error: "endpoint answered 502"}, {Number: 1, StatusCode: 502}, {Number: 2, StatusCode: 200}} {
		if err := store.RecordAttempt(ctx, d, a, time.Now()); err != nil {
			t.Fatal(err)
		}
	}
	got, err := repo.Attempts(ctx, org, e.ID, evID)
	if err != nil || len(got) != 2 || got[0].Number != 1 || got[0].Response != "gateway" || got[1].Number != 2 {
		t.Fatalf("attempts = %+v %v", got, err)
	}
}

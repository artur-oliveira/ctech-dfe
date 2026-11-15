package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"gopkg.aoctech.app/api-commons/webhooks"
)

var now = time.Unix(1_800_000_000, 0)

type store struct {
	mu        sync.Mutex
	due       []webhooks.Due
	err       error
	delivered []string
	endpoint  webhooks.Endpoint
}

func (s *store) DueDeliveries(context.Context, int) ([]webhooks.Due, error) { return s.due, s.err }
func (s *store) GetEndpoint(context.Context, webhooks.Delivery) (*webhooks.Endpoint, error) {
	e := s.endpoint
	return &e, nil
}
func (s *store) RecordAttempt(context.Context, webhooks.Delivery, webhooks.Attempt, time.Time) error {
	return nil
}
func (s *store) MarkDelivered(_ context.Context, d webhooks.Delivery, _ int, _ time.Time) error {
	s.mu.Lock()
	s.delivered = append(s.delivered, d.EventID)
	s.mu.Unlock()
	return nil
}
func (s *store) MarkAttemptFailed(context.Context, webhooks.Delivery, int, string, time.Time) error {
	return nil
}
func (s *store) Defer(context.Context, webhooks.Delivery, time.Time, time.Time) error { return nil }
func (s *store) Close(context.Context, webhooks.Delivery, string, time.Time) error    { return nil }
func (s *store) RecordHealth(context.Context, webhooks.Endpoint, bool, string, time.Time) error {
	return nil
}
func (s *store) Requeue(context.Context, webhooks.Delivery, time.Time) error { return nil }

func TestRunDeliversWhatIsDue(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	t.Cleanup(srv.Close)
	s := &store{
		due:      []webhooks.Due{{Delivery: webhooks.Delivery{EventID: "e1", EndpointID: "w1", Group: "g"}, Event: webhooks.Event{ID: "e1", Type: "document.authorized", Body: []byte("{}")}}},
		endpoint: webhooks.Endpoint{ID: "w1", URL: srv.URL, Secret: "s", Active: true},
	}
	engine := webhooks.NewDeliverer(s, webhooks.Config{HeaderPrefix: "X-Ctech-", Client: srv.Client()})
	if err := run(context.Background(), engine, 10, now); err != nil {
		t.Fatal(err)
	}
	if len(s.delivered) != 1 || s.delivered[0] != "e1" {
		t.Fatalf("delivered = %v", s.delivered)
	}
}

func TestRunFailsWhenTheQueueCannotBeRead(t *testing.T) {
	s := &store{err: errors.New("dynamodb down")}
	engine := webhooks.NewDeliverer(s, webhooks.Config{})
	if err := run(context.Background(), engine, 10, now); err == nil {
		t.Fatal("a pass that could not read the queue reported success")
	}
}

func TestRunWithNothingDueIsQuiet(t *testing.T) {
	engine := webhooks.NewDeliverer(&store{}, webhooks.Config{})
	if err := run(context.Background(), engine, 10, now); err != nil {
		t.Fatal(err)
	}
}

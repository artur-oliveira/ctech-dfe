package billingclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"gopkg.aoctech.app/api-commons/cache"
)

func TestReportLevelPostsTheWholeCount(t *testing.T) {
	var got map[string]any
	var key string
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/usage/levels", func(w http.ResponseWriter, r *http.Request) {
		key = r.Header.Get("Idempotency-Key")
		_ = json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusCreated)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	c := New(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/v1.0/token", ClientID: "dfe", ClientSecret: "s", Cache: cache.NewMemoryBackend(4)})

	err := c.ReportLevel(context.Background(), LevelReport{
		CustomerRef: "ORG_org_1", Meter: "dfe_companies", Value: 3,
		OccurredAt: "2026-10-10T14:03:00Z", IdempotencyKey: "dfe_companies:org_1:v7",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got["customer_ref"] != "ORG_org_1" || got["meter"] != "dfe_companies" || got["value"] != float64(3) ||
		got["occurred_at"] != "2026-10-10T14:03:00Z" || got["idempotency_key"] != "dfe_companies:org_1:v7" || key != "dfe_companies:org_1:v7" {
		t.Fatalf("body = %v, header key = %q", got, key)
	}
}

func levelClientAnswering(t *testing.T, status int, body string) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/v1.0/token", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
	})
	mux.HandleFunc("/v1.0/usage/levels", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/v1.0/token", ClientID: "dfe", ClientSecret: "s", Cache: cache.NewMemoryBackend(4)})
}

// Billing answers 409 idempotency_key_reused when the key already recorded a
// level (with another body): that version was delivered.
func TestReportLevelKeyReusedIsAlreadyRecorded(t *testing.T) {
	c := levelClientAnswering(t, http.StatusConflict, `{"status":409,"title":"Idempotency Conflict","code":"idempotency_key_reused"}`)
	err := c.ReportLevel(context.Background(), LevelReport{CustomerRef: "ORG_o", Meter: "dfe_companies", Value: 1, IdempotencyKey: "k"})
	if !errors.Is(err, ErrLevelAlreadyRecorded) {
		t.Fatalf("err = %v, want ErrLevelAlreadyRecorded", err)
	}
}

// Billing answers 409 concurrent_update when nothing was recorded: it must be
// retried, so it is not "already recorded".
func TestReportLevelConcurrentUpdateIsRetryable(t *testing.T) {
	c := levelClientAnswering(t, http.StatusConflict, `{"status":409,"title":"Concurrent Update","code":"concurrent_update"}`)
	err := c.ReportLevel(context.Background(), LevelReport{CustomerRef: "ORG_o", Meter: "dfe_companies", Value: 1, IdempotencyKey: "k"})
	if err == nil || errors.Is(err, ErrLevelAlreadyRecorded) {
		t.Fatalf("err = %v, want a retryable error", err)
	}
}

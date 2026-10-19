package accountclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"gopkg.aoctech.app/api-commons/resilient"
)

func serving(t *testing.T, status int, body string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return &Client{http: srv.Client(), baseURL: srv.URL, tokens: nil}
}

// An unconfigured client refuses and says so. It must never read as permission:
// billing's nil client answers ErrNotConfigured and callers treat that as
// "unlimited", which is right for a quota and an open door for an authorization
// check.
func TestANilClientRefuses(t *testing.T) {
	var c *Client
	if c.Enabled() {
		t.Fatal("a nil client reported itself enabled")
	}
	_, ok, err := c.Reach(context.Background(), "cmp_1", "usr_1")
	if ok {
		t.Fatal("an unconfigured client granted reach")
	}
	if err == nil {
		t.Fatal("an unconfigured client answered as a plain refusal")
	}
}

// A 403 is an error, not a refusal. It means this client's own credential is
// wrong — an operational fault rather than a statement about the user — and
// reading it as a refusal would hide a broken deployment behind thousands of
// denied requests.
func TestA403IsAnErrorAndNotARefusal(t *testing.T) {
	c := serving(t, http.StatusForbidden, `{"detail":"Missing required scope."}`)
	c.tokens = nil
	_, _, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
	if err == nil {
		t.Fatal("a 403 answered as a refusal; a broken credential would look like a denied user")
	}
	if !strings.Contains(err.Error(), "403") {
		t.Errorf("the error does not name the status: %v", err)
	}
}

// A refusal is a refusal: 200 with may_act false, no error.
func TestARefusalIsNotAnError(t *testing.T) {
	c := serving(t, http.StatusOK, `{"may_act":false}`)
	orgID, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
	if err != nil || ok || orgID != "" {
		t.Fatalf("got %q %v %v, want empty false nil", orgID, ok, err)
	}
}

// A grant carries the organization the caller did not know.
func TestAGrantCarriesTheOrganization(t *testing.T) {
	c := serving(t, http.StatusOK, `{"may_act":true,"organization_id":"org_1"}`)
	orgID, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1")
	if err != nil || !ok || orgID != "org_1" {
		t.Fatalf("got %q %v %v", orgID, ok, err)
	}
}

// A malformed body is an error, never a grant. Half a response is not permission.
func TestAMalformedBodyIsAnError(t *testing.T) {
	c := serving(t, http.StatusOK, `<html>`)
	if _, ok, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1"); ok || err == nil {
		t.Fatalf("ok=%v err=%v, want a refusal with an error", ok, err)
	}
}

// A 500 is an error, and the caller refuses. The alternative — treating an
// outage as a refusal — is what makes an incident indistinguishable from a
// permissions change.
func TestAnUpstreamErrorIsAnError(t *testing.T) {
	c := serving(t, http.StatusInternalServerError, ``)
	if _, _, err := c.reachWithToken(context.Background(), "tok", "cmp_1", "usr_1"); err == nil {
		t.Fatal("a 500 answered without an error")
	}
}

// While ctech-account is down five failures open the circuit and the next call is
// refused at once. It is still an ERROR, never an answer: an authorization check that
// reads "the account did not say" as anything but a failure is an open door.
func TestADownAccountOpensTheCircuitAndStaysAnErrorNotAPermission(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			_, _ = w.Write([]byte(`{"access_token":"tok","expires_in":3600}`))
			return
		}
		hits.Add(1)
		http.Error(w, "down", http.StatusServiceUnavailable)
	}))
	t.Cleanup(srv.Close)
	c := New(Config{BaseURL: srv.URL, TokenURL: srv.URL + "/token", ClientID: "id", ClientSecret: "secret"})

	for i := 0; i < 5; i++ {
		if _, _, err := c.Reach(context.Background(), "cmp_1", "usr_1"); err == nil {
			t.Fatalf("call %d against a down account answered without an error", i)
		}
	}
	before := hits.Load()
	_, allowed, err := c.Reach(context.Background(), "cmp_1", "usr_1")
	if !errors.Is(err, resilient.ErrCircuitOpen) || allowed {
		t.Fatalf("allowed=%v err=%v, want a refused call with the open-circuit error and no permission", allowed, err)
	}
	if hits.Load() != before {
		t.Fatal("the refused call still reached the account")
	}
}

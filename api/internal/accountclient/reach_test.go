package accountclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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

package accountclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSubjectCPF(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth = r.URL.Path, r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"cpf":"12345678909"}`))
	}))
	t.Cleanup(srv.Close)
	c := &SubjectClient{http: srv.Client(), baseURL: srv.URL}

	cpf, err := c.cpfWithToken(context.Background(), "tok", "req-1")
	if err != nil || cpf != "12345678909" {
		t.Fatalf("cpf %q, err %v", cpf, err)
	}
	if gotPath != "/v1.0/internal/erasure/req-1/subject" || gotAuth != "Bearer tok" {
		t.Fatalf("path %q auth %q", gotPath, gotAuth)
	}
}

// Outside PURGING ctech-account answers 404. That is an error (the message is
// redelivered), and no error text may carry a response body.
func TestSubjectCPF_NotPurgingIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"12345678909"}`))
	}))
	t.Cleanup(srv.Close)
	c := &SubjectClient{http: srv.Client(), baseURL: srv.URL}
	_, err := c.cpfWithToken(context.Background(), "tok", "req-1")
	if err == nil || strings.Contains(err.Error(), "12345678909") {
		t.Fatalf("err = %v; want an error that carries no body", err)
	}
}

func TestSubjectNilClientRefuses(t *testing.T) {
	var c *SubjectClient
	if _, err := c.CPF(context.Background(), "req-1"); err == nil {
		t.Fatal("an unconfigured subject client must fail, not answer an empty CPF")
	}
	if NewSubject(Config{BaseURL: "https://a", TokenURL: "https://a/t", ClientID: "dfe"}) != nil {
		t.Fatal("built without a secret")
	}
}

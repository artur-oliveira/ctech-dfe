package services

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSingleRootElement(t *testing.T) {
	tag, body, err := singleRootElement(map[string]any{"consStatServ": map[string]any{"tpAmb": "2"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tag != "consStatServ" {
		t.Errorf("tag = %q, want consStatServ", tag)
	}
	if body["tpAmb"] != "2" {
		t.Errorf("body = %v", body)
	}

	if _, _, err := singleRootElement(map[string]any{"a": map[string]any{}, "b": map[string]any{}}); err == nil {
		t.Error("expected error for multi-key payload, got nil")
	}
	if _, _, err := singleRootElement(map[string]any{}); err == nil {
		t.Error("expected error for empty payload, got nil")
	}
	if _, _, err := singleRootElement(map[string]any{"a": "not an object"}); err == nil {
		t.Error("expected error for non-object root value, got nil")
	}
}

func TestPostWithRetry_SucceedsAfterRetryableStatus(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), maxRetries: 3}
	body, err := c.postWithRetryNoSleep(context.Background(), srv.URL, []byte("<x/>"), "text/xml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want ok", body)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestPostWithRetry_NeverRetriesBusinessRejection(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("rejected"))
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), maxRetries: 3}
	_, err := c.postWithRetryNoSleep(context.Background(), srv.URL, []byte("<x/>"), "text/xml")
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1 (no retry on 4xx)", calls)
	}
}

func TestPostWithRetry_ExhaustsRetriesOnPersistentFailure(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := &Client{httpClient: srv.Client(), maxRetries: 2}
	_, err := c.postWithRetryNoSleep(context.Background(), srv.URL, []byte("<x/>"), "text/xml")
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if calls != 3 { // initial attempt + 2 retries
		t.Errorf("calls = %d, want 3", calls)
	}
}

// postWithRetryNoSleep is postWithRetry with backoff removed, so retry tests
// run in milliseconds instead of seconds.
func (c *Client) postWithRetryNoSleep(ctx context.Context, url string, body []byte, contentType string) ([]byte, error) {
	orig := sleepFn
	sleepFn = func(int) {}
	defer func() { sleepFn = orig }()
	return c.postWithRetry(ctx, url, body, contentType)
}

// redirectingTransport rewrites every outgoing request's scheme/host to
// target's, regardless of what URL the caller built (i.e. whatever
// endpoints.Resolve produced) — lets a test point Client.Call at an
// httptest.Server without needing an endpoint-override hook on Client
// itself.
type redirectingTransport struct{ target *url.URL }

func (t redirectingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = t.target.Scheme
	req.URL.Host = t.target.Host
	req.Host = t.target.Host
	return http.DefaultTransport.RoundTrip(req)
}

// buildTestRSACert generates a self-signed RSA cert + key for signing tests
// (mirrors go-dfe/internal/certificate/manager_test.go's buildTestPFX, but
// returns the parsed cert/key directly since Client takes them pre-decoded).
func buildTestRSACert(t *testing.T) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "TEST COMPANY:12345678000195"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	certDER, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate: %v", err)
	}
	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	return cert, key
}

// TestCall_SignedService_EndToEnd proves the full pipeline for a *signed*
// service through Client.Call — build XML, sign it (internal/xmlops.Sign),
// wrap in a SOAP envelope, POST, parse the response — composes correctly.
// This is NOT the plan's byte-identical py-dfe gate (see
// docs/plans/2026-07-17-go-dfe-migration.md and go-dfe/CLAUDE.md); it only
// proves the Go-side machinery works end-to-end, independent of whether
// NFeAutorizacao is ever promoted into dfe.Implements().
func TestCall_SignedService_EndToEnd(t *testing.T) {
	cert, key := buildTestRSACert(t)

	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, _ := io.ReadAll(r.Body)
		capturedBody = string(buf)

		w.Header().Set("Content-Type", "application/soap+xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<soap12:Envelope xmlns:soap12="http://www.w3.org/2003/05/soap-envelope">
  <soap12:Body>
    <nfeResultMsg xmlns="http://www.portalfiscal.inf.br/wsdl/NFeAutorizacao4">
      <retEnviNFe xmlns="http://www.portalfiscal.inf.br/nfe">
        <protNFe>
          <infProt>
            <cStat>100</cStat>
            <xMotivo>Autorizado o uso da NF-e</xMotivo>
          </infProt>
        </protNFe>
      </retEnviNFe>
    </nfeResultMsg>
  </soap12:Body>
</soap12:Envelope>`))
	}))
	defer srv.Close()

	target, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	httpClient := &http.Client{Transport: redirectingTransport{target: target}}

	client, err := NewClient("nfe", "SP", "hom", httpClient, cert, key, false, 0)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	payload := map[string]any{
		"enviNFe": map[string]any{
			"@xmlns": "http://www.portalfiscal.inf.br/nfe",
			"idLote": "1",
			"NFe": map[string]any{
				"infNFe": map[string]any{
					"@Id": "NFe35200714200166000187550010000000046550000046",
					"ide": map[string]any{"cUF": "35"},
				},
			},
		},
	}

	result, err := client.Call(context.Background(), "NFeAutorizacao", payload)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}

	if !strings.Contains(capturedBody, "<Signature") || !strings.Contains(capturedBody, "<SignatureValue>") {
		t.Errorf("SOAP request sent to SEFAZ was not signed: %s", capturedBody)
	}
	if !strings.Contains(capturedBody, "infNFe") {
		t.Errorf("signed body missing infNFe: %s", capturedBody)
	}

	retEnviNFe, _ := result["retEnviNFe"].(map[string]any)
	// ensureList("retEnviNFe/protNFe") normalizes the single protNFe
	// occurrence into a one-element list (NFeAutorizacao's
	// nfeNfceEnsureListPaths entry, response.go) — matching py-dfe's
	// _ensure_list, exercised here rather than assumed.
	protNFeList, ok := retEnviNFe["protNFe"].([]any)
	if !ok || len(protNFeList) != 1 {
		t.Fatalf("retEnviNFe.protNFe = %#v, want a 1-element list (ensureList should have normalized it)", retEnviNFe["protNFe"])
	}
	protNFe, _ := protNFeList[0].(map[string]any)
	infProt, _ := protNFe["infProt"].(map[string]any)
	if cStat, _ := infProt["cStat"].(string); cStat != "100" {
		t.Errorf("cStat = %v, want 100 (result: %+v)", infProt["cStat"], result)
	}

	xmlField, _ := result["@xml"].(string)
	if !strings.Contains(xmlField, "<nfeProc") || !strings.Contains(xmlField, "<protNFe>") {
		t.Errorf("result[@xml] missing expected processed document: %q", xmlField)
	}
}

package nacional

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

func TestGzipB64_RoundTrip(t *testing.T) {
	raw := []byte("<DPS><infDPS/></DPS>")
	enc, err := GzipB64(raw)
	if err != nil {
		t.Fatalf("GzipB64: %v", err)
	}
	got, err := UngzipB64(enc)
	if err != nil {
		t.Fatalf("UngzipB64: %v", err)
	}
	if string(got) != string(raw) {
		t.Errorf("round-trip = %q, esperado %q", got, raw)
	}
}

func TestWithUTF8DeclarationDoesNotDuplicateHeader(t *testing.T) {
	raw := []byte(xml.Header + "<DPS/>")
	if got := withUTF8Declaration(raw); string(got) != string(raw) {
		t.Errorf("withUTF8Declaration() = %q, esperado %q", got, raw)
	}
}

func TestHTTPDo_FiscalErrorPreservesCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got != nfseRESTUserAgent {
			t.Errorf("User-Agent = %q, esperado %q", got, nfseRESTUserAgent)
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"erros": []map[string]string{
				{"codigo": "E0001", "descricao": "cTribNac inválido", "complemento": "linha 1"},
			},
		})
	}))
	defer srv.Close()

	var out map[string]any
	_, err := httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, &out, 0)
	var fe *nfse.FiscalError
	if !errors.As(err, &fe) {
		t.Fatalf("esperado *nfse.FiscalError, veio %v", err)
	}
	if len(fe.Messages) != 1 || fe.Messages[0].Codigo != "E0001" {
		t.Errorf("mensagem do fisco perdida: %+v", fe.Messages)
	}
	if fe.Status != http.StatusBadRequest {
		t.Errorf("Status = %d, esperado 400", fe.Status)
	}
}

func TestHTTPDo_FiscalErrorAcceptsMunicipalMensagem(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"tipoAmbiente":"2","versaoAplicativo":"1.01","idDps":"DPS221100126278744900010700000000000000000001","erros":[{"codigo":"L0017","mensagem":"O código de tributação municipal não foi informado."}]}`))
	}))
	defer srv.Close()

	_, err := httpDo(context.Background(), srv.Client(), http.MethodPost, srv.URL, nil, nil, 0)
	var fe *nfse.FiscalError
	if !errors.As(err, &fe) {
		t.Fatalf("esperado *nfse.FiscalError, veio %v", err)
	}
	if got, want := fe.Error(), "nfse: HTTP 400: L0017 - O código de tributação municipal não foi informado."; got != want {
		t.Errorf("Error() = %q, esperado %q", got, want)
	}
}

func TestHTTPDo_RetriesOn5xxNotOn4xx(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"chaveAcesso": "abc"})
	}))
	defer srv.Close()

	var out map[string]any
	if _, err := httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, &out, 3); err != nil {
		t.Fatalf("httpDo: %v", err)
	}
	if calls != 3 {
		t.Errorf("chamadas = %d, esperado 3 (dois 502 + sucesso)", calls)
	}
	if out["chaveAcesso"] != "abc" {
		t.Errorf("resposta não decodificada: %+v", out)
	}

	calls = 0
	srv4 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"erro": map[string]string{"codigo": "X"}})
	}))
	defer srv4.Close()
	_, _ = httpDo(context.Background(), srv4.Client(), http.MethodGet, srv4.URL, nil, &out, 3)
	if calls != 1 {
		t.Errorf("4xx foi repetido %d vezes; rejeição de negócio nunca se repete", calls)
	}
}

func TestHTTPDoAttemptTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // never answers
	}))
	defer srv.Close()

	orig := attemptTimeout
	attemptTimeout = 50 * time.Millisecond
	defer func() { attemptTimeout = orig }()

	start := time.Now()
	_, err := httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, nil, 0)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("took %v: per-attempt timeout not applied", elapsed)
	}
}

// A POST that timed out may already have been accepted by the authority:
// re-sending the same DPS would turn an authorized NFS-e into a duplicate
// rejection. A timed-out POST must not be retried.
func TestHTTPDoDoesNotRetryTimedOutPOST(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.Copy(io.Discard, r.Body) // lets the server notice the client hanging up
		<-r.Context().Done()
	}))
	defer srv.Close()

	orig := attemptTimeout
	attemptTimeout = 50 * time.Millisecond
	defer func() { attemptTimeout = orig }()

	_, err := httpDo(context.Background(), srv.Client(), http.MethodPost, srv.URL, map[string]any{"a": 1}, nil, 1)
	if err == nil {
		t.Fatal("expected an error")
	}
	if got := hits.Load(); got != 1 {
		t.Fatalf("server saw %d POSTs, want exactly 1 (timed-out POST must not be retried)", got)
	}
}

func TestHTTPDoRetriesTimedOutGET(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		<-r.Context().Done()
	}))
	defer srv.Close()

	orig := attemptTimeout
	attemptTimeout = 30 * time.Millisecond
	defer func() { attemptTimeout = orig }()

	_, _ = httpDo(context.Background(), srv.Client(), http.MethodGet, srv.URL, nil, nil, 1)
	if got := hits.Load(); got != 2 {
		t.Fatalf("server saw %d GETs, want 2 (reads are idempotent and retried)", got)
	}
}

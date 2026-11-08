package ciot

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func noSleep(context.Context, int) error { return nil }

func newTestClient(t *testing.T, h http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c, err := NewClient(EnvHom, srv.Client(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c.Sleep = noSleep
	return c
}

func TestParseResultAcceptsListsAndBothIdNames(t *testing.T) {
	r, err := ParseResult([]byte(`{"Codigo":["110"],"Mensagem":["ok","x"],"Protocolo":"P1","CodigoIdentificacaoOperacao":"ABCDEF123456","CodigoVerificador":"1234"}`))
	if err != nil || !r.Success() || r.Mensagem != "ok" || r.CIOT != "ABCDEF1234561234" {
		t.Fatalf("got %+v err %v", r, err)
	}
	r, _ = ParseResult([]byte(`{"Codigo":111,"Mensagem":"m","IdOperacaoTransporte":"ABCDEF123456"}`))
	if !r.Success() || r.IdOperacao != "ABCDEF123456" {
		t.Fatalf("got %+v", r)
	}
}

func TestDeclarePathRejectionAndNoRetryOnBusiness(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.URL.Path != "/api/DeclaracaoOperacaoTransporte" || r.Method != http.MethodPost {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		var d Declaracao
		_ = json.Unmarshal(b, &d)
		if d.RNTRCContratado != "012345678" {
			t.Errorf("RNTRC not normalized: %q", d.RNTRCContratado)
		}
		_, _ = w.Write([]byte(`{"Codigo":[219],"Mensagem":["duplicado"],"Protocolo":"p"}`))
	}))
	_, err := c.Declarar(context.Background(), Declaracao{IdOperacaoTransporte: "ABCDEF123456", RNTRCContratado: "12345678"})
	rj, ok := err.(*Rejection)
	if !ok || rj.Codigo != CodeDuplicateID || rj.Internal != "CIOT_STATE_219" || calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestDeclareSuccessBuildsCIOT16(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"Codigo":110,"Mensagem":"ok","CodigoVerificador":"9876","AvisoTransportador":"aviso"}`))
	}))
	res, err := c.Declarar(context.Background(), Declaracao{IdOperacaoTransporte: "abcdef123456"})
	if err != nil || res.CIOT != "ABCDEF1234569876" || res.AvisoTransportador != "aviso" || len(res.CIOT) != CIOTLen {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestQueryRetriesTransportFailureOnly(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			hj, _ := w.(http.Hijacker)
			conn, _, _ := hj.Hijack()
			_ = conn.Close() // transport failure
			return
		}
		if r.Method != http.MethodGet || r.URL.Query().Get("CPFCNPJTransportador") != "12345678909" {
			t.Errorf("bad GET %s %s", r.Method, r.URL)
		}
		_, _ = w.Write([]byte(`{"Codigo":111,"Mensagem":"ok"}`))
	}))
	defer srv.Close()
	c, _ := NewClient(EnvHom, srv.Client(), srv.URL)
	c.Sleep = noSleep
	if _, err := c.Call(context.Background(), ServiceExcecao, map[string]string{"CPFCNPJTransportador": "12345678909"}); err != nil || calls.Load() != 3 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestServerErrorNotRetried(t *testing.T) {
	var calls atomic.Int32
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	_, err := c.Call(context.Background(), ServiceSituacaoTransportador, ConsultaTransportador{})
	if _, ok := err.(*ErrServer); !ok || calls.Load() != 1 {
		t.Fatalf("err %v calls %d", err, calls.Load())
	}
}

func TestTimeoutIsTransportFailure(t *testing.T) {
	release := make(chan struct{})
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	t.Cleanup(func() { close(release) }) // runs before srv.Close (LIFO)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Call(ctx, ServiceCIOTGerado, ConsultaCIOTGerado{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestHostForNeverMixes(t *testing.T) {
	if h, _ := realHostFor(EnvHom); h != HostHomologacao {
		t.Fatal(h)
	}
	if h, _ := realHostFor(EnvProd); h != HostProducao {
		t.Fatal(h)
	}
	if _, err := realHostFor("staging"); err == nil {
		t.Fatal("expected error")
	}
}

func TestNormalizeAndMoney(t *testing.T) {
	if NormalizeRNTRC("12345678") != "012345678" || NormalizeRNTRC("012345678") != "012345678" {
		t.Fatal("rntrc")
	}
	if !ValidPlate("ABC1234") || !ValidPlate("ABC1D23") || ValidPlate("AB12345") {
		t.Fatal("plate")
	}
	if FormatMoney(123456) != "1234.56" || FormatMoney(5) != "0.05" {
		t.Fatal("money")
	}
	if c, err := ParseMoney("12.3"); err != nil || c != 1230 {
		t.Fatal(c, err)
	}
	if _, err := ParseMoney("1.234"); err == nil {
		t.Fatal("3 decimals must fail")
	}
	ts := time.Date(2026, 11, 4, 2, 30, 0, 0, time.UTC)
	if FormatDateTime(ts) != "2026-11-03T23:30:00" || FormatDate(ts) != "2026-11-03" {
		t.Fatal(FormatDateTime(ts))
	}
}

func TestRejectionTableStable(t *testing.T) {
	if LookupRejection(291).Internal != "CIOT_FLOOR_291" || LookupRejection(9999).Internal != "CIOT_UNMAPPED" {
		t.Fatal("table")
	}
	if !LookupRejection(311).IsCredentialProblem() {
		t.Fatal("credential")
	}
}

// Nenhum teste pode alcançar o host real da ANTT: sem host falso fixado, HostFor falha em vez de devolver o real.
func TestHostForRefusesRealHostUnderTest(t *testing.T) {
	restore := SetTestHosts(map[string]string{})
	defer restore()
	for _, env := range []string{EnvProd, EnvHom} {
		if h, err := HostFor(env); err == nil {
			t.Fatalf("%s: devolveu %q em teste; o host real deveria ser recusado", env, h)
		}
	}
}

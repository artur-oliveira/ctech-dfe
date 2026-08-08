package nacional

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gopkg.aoctech.app/dfe/go-dfe/nfse"
)

// newTestProvider aponta o provider para um httptest.Server substituindo as
// bases resolvidas. baseOverride existe só para teste.
func newTestProvider(t *testing.T, srvURL string) *Nacional {
	t.Helper()
	p, err := New(Config{Environment: "hom", HTTPClient: http.DefaultClient, MaxRetries: 0})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	p.baseOverride = map[string]string{
		SystemSefin: srvURL, SystemADN: srvURL,
		SystemDANFSE: srvURL, SystemParametros: srvURL,
	}
	return p
}

func TestNacional_Emit(t *testing.T) {
	nfseXML := "<NFSe><infNFSe/></NFSe>"
	encoded, err := GzipB64([]byte(nfseXML))
	if err != nil {
		t.Fatalf("GzipB64: %v", err)
	}

	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != PathNFSe || r.Method != http.MethodPost {
			t.Errorf("rota inesperada: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente": 2, "versaoAplicativo": "1.0",
			"idDps":          "DPS" + strings.Repeat("1", 42),
			"chaveAcesso":    strings.Repeat("9", 50),
			"nfseXmlGZipB64": encoded,
		})
	}))
	defer srv.Close()

	// Sem certificado o provider não assina; o teste cobre a montagem e o
	// transporte. A assinatura tem cobertura própria em xmlops/signer_test.go.
	p := newTestProvider(t, srv.URL)
	res, err := p.Emit(context.Background(), minimalDoc())
	if err != nil {
		t.Fatalf("Emit: %v", err)
	}
	if received["dpsXmlGZipB64"] == "" {
		t.Error("dpsXmlGZipB64 não foi enviado")
	}
	sentDPS, err := UngzipB64(received["dpsXmlGZipB64"])
	if err != nil {
		t.Fatalf("descompactar DPS enviada: %v", err)
	}
	if !strings.HasPrefix(string(sentDPS), xml.Header) {
		t.Errorf("DPS enviada não começa com declaração UTF-8: %q", sentDPS[:min(len(sentDPS), len(xml.Header))])
	}
	if strings.Count(string(sentDPS), xml.Header) != 1 {
		t.Errorf("DPS enviada deve conter uma declaração XML, veio %d", strings.Count(string(sentDPS), xml.Header))
	}
	if res.ChaveAcesso != strings.Repeat("9", 50) {
		t.Errorf("ChaveAcesso = %q", res.ChaveAcesso)
	}
	if res.NFSeXML != nfseXML {
		t.Errorf("NFSeXML = %q, esperado %q", res.NFSeXML, nfseXML)
	}
	if !strings.Contains(res.DPSXML, "<infDPS") {
		t.Error("DPSXML enviado não foi devolvido no Result")
	}
}

func TestNacional_QueryByDPSID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/dps/") {
			t.Errorf("rota inesperada: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"idDps": "DPS123", "chaveAcesso": strings.Repeat("7", 50),
		})
	}))
	defer srv.Close()

	res, err := newTestProvider(t, srv.URL).QueryByDPSID(context.Background(), "DPS123")
	if err != nil {
		t.Fatalf("QueryByDPSID: %v", err)
	}
	if res.ChaveAcesso != strings.Repeat("7", 50) {
		t.Errorf("ChaveAcesso = %q", res.ChaveAcesso)
	}
}

func TestNacional_Event(t *testing.T) {
	eventoXML := "<evento><infEvento/></evento>"
	encoded, _ := GzipB64([]byte(eventoXML))
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/eventos") || r.Method != http.MethodPost {
			t.Errorf("rota inesperada: %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"eventoXmlGZipB64": encoded})
	}))
	defer srv.Close()

	ev := baseEvent(nfse.EventCancelamento)
	ev.Motivo = &nfse.EventMotivo{Codigo: "1", Descricao: "Erro na emissão"}
	res, err := newTestProvider(t, srv.URL).Event(context.Background(), ev)
	if err != nil {
		t.Fatalf("Event: %v", err)
	}
	if res.EventoXML != eventoXML {
		t.Errorf("EventoXML = %q", res.EventoXML)
	}
	sentEvent, err := UngzipB64(received["pedidoRegistroEventoXmlGZipB64"])
	if err != nil {
		t.Fatalf("descompactar evento enviado: %v", err)
	}
	if !strings.HasPrefix(string(sentEvent), xml.Header) {
		t.Errorf("evento enviado não começa com declaração UTF-8: %q", sentEvent[:min(len(sentEvent), len(xml.Header))])
	}
	if strings.Count(string(sentEvent), xml.Header) != 1 {
		t.Errorf("evento enviado deve conter uma declaração XML, veio %d", strings.Count(string(sentEvent), xml.Header))
	}
}

func TestNacional_EmitPropagatesFiscalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"erros": []map[string]string{{"codigo": "E1", "descricao": "rejeitado"}},
		})
	}))
	defer srv.Close()

	_, err := newTestProvider(t, srv.URL).Emit(context.Background(), minimalDoc())
	if err == nil || !strings.Contains(err.Error(), "rejeitado") {
		t.Fatalf("rejeição do fisco não propagada: %v", err)
	}
}

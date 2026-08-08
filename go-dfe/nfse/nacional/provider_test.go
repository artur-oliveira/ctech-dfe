package nacional

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"log/slog"
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
		t.Error("DPS enviada deve declarar UTF-8")
	}
	if strings.Contains(string(sentDPS), "Análise") || !strings.Contains(string(sentDPS), "Analise") {
		t.Errorf("DPS não removeu diacríticos antes da assinatura: %s", sentDPS)
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
		t.Error("evento enviado deve declarar UTF-8")
	}
	if strings.Contains(string(sentEvent), "emissão") || !strings.Contains(string(sentEvent), "emissao") {
		t.Errorf("evento não removeu diacríticos antes da assinatura: %s", sentEvent)
	}
}

func TestNacional_EmitPropagatesFiscalError(t *testing.T) {
	var received map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&received)
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"erros": []map[string]string{{"codigo": "E1", "descricao": "rejeitado"}},
		})
	}))
	defer srv.Close()
	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&logs, nil)))
	defer slog.SetDefault(previousLogger)

	_, err := newTestProvider(t, srv.URL).Emit(context.Background(), minimalDoc())
	if err == nil || !strings.Contains(err.Error(), "rejeitado") {
		t.Fatalf("rejeição do fisco não propagada: %v", err)
	}
	packed := received[fieldDpsXMLGZipB64]
	if packed == "" {
		t.Fatal("payload gzip+base64 não foi recebido pelo servidor")
	}
	if !strings.Contains(logs.String(), packed) {
		t.Errorf("log da rejeição não contém %s", fieldDpsXMLGZipB64)
	}
	sentDPS, err := UngzipB64(packed)
	if err != nil {
		t.Fatalf("descompactar DPS rejeitada: %v", err)
	}
	if !strings.HasPrefix(string(sentDPS), xml.Header) {
		t.Error("DPS rejeitada logada deve declarar UTF-8")
	}
}

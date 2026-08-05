package nacional

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNacional_Distribute(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/DFe/10" {
			t.Errorf("path = %q, esperado /DFe/10", r.URL.Path)
		}
		if r.URL.Query().Get("cnpjConsulta") != "11222333000181" {
			t.Errorf("cnpjConsulta ausente: %q", r.URL.RawQuery)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"StatusProcessamento": "DOCUMENTOS_LOCALIZADOS",
			"LoteDFe": []map[string]any{{
				"NSU": 11, "ChaveAcesso": "abc", "TipoDocumento": "NFSE",
				"ArquivoXml": "<NFSe/>", "DataHoraGeracao": "2026-08-04T12:00:00Z",
			}},
		})
	}))
	defer srv.Close()

	res, err := newTestProvider(t, srv.URL).Distribute(context.Background(), 10, "11222333000181", true)
	if err != nil {
		t.Fatalf("Distribute: %v", err)
	}
	if res.StatusDistribuicao != "DOCUMENTOS_LOCALIZADOS" {
		t.Errorf("StatusDistribuicao = %q", res.StatusDistribuicao)
	}
	if len(res.Distribuicao) != 1 || res.Distribuicao[0].NSU != 11 {
		t.Fatalf("lote não parseado: %+v", res.Distribuicao)
	}
	if res.Distribuicao[0].XML != "<NFSe/>" {
		t.Errorf("ArquivoXml perdido: %q", res.Distribuicao[0].XML)
	}
}

func TestNacional_DANFSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4 fake"))
	}))
	defer srv.Close()

	pdf, err := newTestProvider(t, srv.URL).DANFSE(context.Background(), "chave")
	if err != nil {
		t.Fatalf("DANFSE: %v", err)
	}
	if string(pdf[:4]) != "%PDF" {
		t.Errorf("resposta não é PDF: %q", pdf)
	}
}

func TestNacional_MunicipalParameters_Convenio(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/2211001/convenio" {
			t.Errorf("path = %q", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"aderenteAmbienteNacional": true})
	}))
	defer srv.Close()

	res, err := newTestProvider(t, srv.URL).MunicipalParameters(context.Background(), ParamConvenio, "2211001")
	if err != nil {
		t.Fatalf("MunicipalParameters: %v", err)
	}
	if res.Parametros["aderenteAmbienteNacional"] != true {
		t.Errorf("parâmetros não parseados: %+v", res.Parametros)
	}
}

func TestNacional_MunicipalParameters_WrongArity(t *testing.T) {
	if _, err := newTestProvider(t, "http://x").MunicipalParameters(context.Background(), ParamAliquota, "2211001"); err == nil {
		t.Fatal("esperado erro: aliquota exige município, serviço e competência")
	}
}

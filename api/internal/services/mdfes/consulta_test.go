package mdfes

import (
	"context"
	"encoding/json"
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

// fakeEgress simula o go-dfe-egress: guarda o pedido e devolve a resposta montada no teste.
type fakeEgress struct {
	got  godfe.Request
	resp godfe.Response
}

func (f *fakeEgress) CertificateB64(context.Context, string) (string, string, error) {
	return "Y2VydA==", "senha", nil
}

func (f *fakeEgress) CallDfe(_ context.Context, req godfe.Request) (godfe.Response, error) {
	f.got = req
	return f.resp, nil
}

func egressOK(t *testing.T, body map[string]any) godfe.Response {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	return godfe.Response{StatusCode: 200, Body: string(b)}
}

const consultaKey = "35250512345678000195580010000000011000000015"

func TestConsultaSituacaoViaEgressSimulado(t *testing.T) {
	f := &fakeEgress{resp: egressOK(t, map[string]any{"retConsSitMDFe": map[string]any{
		"cStat": "132", "xMotivo": "Encerramento de MDF-e homologado",
		"protMDFe": map[string]any{"infProt": map[string]any{"nProt": "935250000000001", "dhRecbto": "2025-05-10T10:00:00-03:00", "cStat": "100", "xMotivo": "Autorizado"}},
		// um evento só chega como objeto, não lista
		"procEventoMDFe": map[string]any{
			"eventoMDFe":    map[string]any{"infEvento": map[string]any{"tpEvento": "110112", "nSeqEvento": "1"}},
			"retEventoMDFe": map[string]any{"infEvento": map[string]any{"cStat": "135", "nProt": "935250000000002", "dhRegEvento": "2025-05-11T10:00:00-03:00"}},
		},
	}})}
	s := &MdfeService{egress: f}
	body := map[string]any{"consSitMDFe": map[string]any{"chMDFe": consultaKey}}
	ret, err := s.callConsulta(context.Background(), "org", "12345678000195", "SP", EnvHom, sefazServiceConsulta, body, "retConsSitMDFe")
	if err != nil {
		t.Fatal(err)
	}
	if f.got.Service != sefazServiceConsulta || f.got.DocType != s3PrefixMdfe || f.got.UF != "SP" || f.got.Environment != EnvHom || f.got.CertificateB64 == "" {
		t.Fatalf("pedido ao egress = %+v", f.got)
	}
	sit, err := parseSituation(consultaKey, ret)
	if err != nil {
		t.Fatal(err)
	}
	if sit.Status != StatusClosed || sit.Protocol == nil || sit.Protocol.NProt != "935250000000001" {
		t.Fatalf("situação = %+v", sit)
	}
	if len(sit.Events) != 1 || sit.Events[0].TpEvento != "110112" || sit.Events[0].CStat != "135" {
		t.Fatalf("eventos = %+v", sit.Events)
	}
}

func TestConsultaSituacaoRejeicaoEEgressComErro(t *testing.T) {
	_, err := parseSituation(consultaKey, map[string]any{"cStat": "460", "xMotivo": "Consulta a uma Chave de Acesso muito antiga"})
	if p, ok := err.(*problem.Problem); !ok || p.Type != dfeproblem.TypeSefazRejection {
		t.Fatalf("460 deve ser sefaz-rejection, got %#v", err)
	}
	if sit, err := parseSituation(consultaKey, map[string]any{"cStat": "217", "xMotivo": "nao consta"}); err != nil || sit.Status != StatusNotFound {
		t.Fatalf("217 é resposta, não erro: %+v %v", sit, err)
	}
	f := &fakeEgress{resp: godfe.Response{StatusCode: 502, Body: `{"title":"Bad Gateway","detail":"SEFAZ indisponivel","status":502}`}}
	s := &MdfeService{egress: f}
	_, err = s.callConsulta(context.Background(), "org", "1", "SP", EnvProd, sefazServiceConsulta, nil, "retConsSitMDFe")
	if p, ok := err.(*problem.Problem); !ok || p.Status != 502 || p.Detail != "SEFAZ indisponivel" {
		t.Fatalf("erro do egress deve preservar status e detalhe: %#v", err)
	}
}

func TestNaoEncerradosViaEgressSimulado(t *testing.T) {
	two := map[string]any{"cStat": "111", "xMotivo": "Consulta não encerrados localizou MDF-e", "infMDFe": []any{
		map[string]any{"chMDFe": consultaKey, "nProt": "935250000000001"},
		map[string]any{"chMDFe": consultaKey[:43] + "6", "nProt": "935250000000003"},
	}}
	f := &fakeEgress{resp: egressOK(t, map[string]any{"retConsMDFeNaoEnc": two})}
	s := &MdfeService{egress: f}
	ret, err := s.callConsulta(context.Background(), "org", "12345678000195", "SP", EnvProd, sefazServiceNaoEnc,
		map[string]any{"consMDFeNaoEnc": map[string]any{}}, "retConsMDFeNaoEnc")
	if err != nil {
		t.Fatal(err)
	}
	got, err := parseUnclosed(ret)
	if err != nil || len(got.Items) != 2 || got.Items[1].NProt != "935250000000003" {
		t.Fatalf("não encerrados = %+v %v", got, err)
	}
	none, err := parseUnclosed(map[string]any{"cStat": "112", "xMotivo": "nenhum"})
	if err != nil || none.Items == nil || len(none.Items) != 0 {
		t.Fatalf("112 é lista vazia: %+v %v", none, err)
	}
	if _, err := parseUnclosed(map[string]any{"cStat": "213", "xMotivo": "CNPJ-Base difere"}); err == nil {
		t.Fatal("213 deve ser rejeição")
	}
	if tpAmbFor(EnvProd) != tpAmbProd || tpAmbFor(EnvHom) != tpAmbHom {
		t.Fatal("tpAmb por ambiente")
	}
}

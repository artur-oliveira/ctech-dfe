package ctes

import (
	"context"
	"encoding/json"
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	"gopkg.aoctech.app/dfe/api/internal/dfeproblem"
	"gopkg.aoctech.app/dfe/api/internal/services"

	godfe "gopkg.aoctech.app/dfe/go-dfe"
)

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

const consultaKey = "35250512345678000195570010000000011000000015"

func TestConsultaSituacaoViaEgressSimulado(t *testing.T) {
	b, _ := json.Marshal(map[string]any{"retConsSitCTe": map[string]any{
		"cStat": "101", "xMotivo": "Cancelamento de CT-e homologado",
		"protCTe": map[string]any{"infProt": map[string]any{"nProt": "135250000000001", "dhRecbto": "2025-05-10T10:00:00-03:00", "cStat": "100", "xMotivo": "Autorizado"}},
		"procEventoCTe": map[string]any{
			"eventoCTe":    map[string]any{"infEvento": map[string]any{"tpEvento": "110111", "nSeqEvento": "1"}},
			"retEventoCTe": map[string]any{"infEvento": map[string]any{"cStat": "135", "nProt": "135250000000002", "dhRegEvento": "2025-05-11T10:00:00-03:00"}},
		},
	}})
	f := &fakeEgress{resp: godfe.Response{StatusCode: 200, Body: string(b)}}
	s := &CteService{egress: f}
	ret, err := s.callConsulta(context.Background(), "org", "12345678000195", "SP", services.EnvHom, sefazServiceConsulta,
		map[string]any{"consSitCTe": map[string]any{"chCTe": consultaKey}}, "retConsSitCTe")
	if err != nil {
		t.Fatal(err)
	}
	if f.got.Service != "CTeConsulta" || f.got.DocType != "cte" || f.got.UF != "SP" || f.got.CertificateB64 == "" {
		t.Fatalf("pedido ao egress = %+v", f.got)
	}
	sit, err := parseSituation(consultaKey, ret)
	if err != nil || sit.Status != StatusCancelled || sit.Protocol.NProt != "135250000000001" {
		t.Fatalf("situação = %+v %v", sit, err)
	}
	if len(sit.Events) != 1 || sit.Events[0].TpEvento != "110111" || sit.Events[0].CStat != "135" {
		t.Fatalf("eventos = %+v", sit.Events)
	}
}

func TestConsultaRejeicaoENaoConsta(t *testing.T) {
	_, err := parseSituation(consultaKey, map[string]any{"cStat": "731", "xMotivo": "Consulta a uma Chave de Acesso muito antiga"})
	if p, ok := err.(*problem.Problem); !ok || p.Type != dfeproblem.TypeSefazRejection {
		t.Fatalf("731 deve ser sefaz-rejection, got %#v", err)
	}
	if sit, err := parseSituation(consultaKey, map[string]any{"cStat": "217"}); err != nil || sit.Status != StatusNotFound {
		t.Fatalf("217 é resposta: %+v %v", sit, err)
	}
	if tpAmbFor(services.EnvProd) != "1" || tpAmbFor(services.EnvHom) != "2" || ufFromAccessKey(consultaKey) != "SP" {
		t.Fatal("tpAmb/UF")
	}
}

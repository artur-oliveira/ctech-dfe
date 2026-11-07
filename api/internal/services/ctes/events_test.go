package ctes

import (
	"context"
	"strings"
	"testing"

	"gopkg.aoctech.app/api-commons/problem"
	dfe "gopkg.aoctech.app/dfe/go-dfe"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const eventKey = "35251112345678000195570010000000071000000015"

func evCtx() *eventContext {
	return &eventContext{environment: 2, cnpj: "12345678000195", docTag: "CNPJ"}
}

func eventXML(t *testing.T, tp string, seq int, root string, det map[string]any) string {
	t.Helper()
	body := buildEventEnvelope(evCtx(), eventKey, tp, seq, root, det)
	raw, err := dfe.BuildXMLFragment(body["eventoCTe"].(map[string]any), "eventoCTe", cteXMLNS)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func inOrder(t *testing.T, xml string, tags ...string) {
	t.Helper()
	last := -1
	for _, tag := range tags {
		i := strings.Index(xml[last+1:], tag)
		if i < 0 {
			t.Fatalf("%s fora de ordem ou ausente em\n%s", tag, xml)
		}
		last += i + 1
	}
}

func TestEventEnvelopesFollowXSDOrder(t *testing.T) {
	x := eventXML(t, TpEventoCancelamento, 1, "evCancCTe", map[string]any{"descEvento": descCancelamento, "nProt": "135250000000001", "xJust": "justificativa de teste"})
	inOrder(t, x, "<cOrgao>", "<tpAmb>", "<CNPJ>", "<chCTe>", "<dhEvento>", "<tpEvento>", "<nSeqEvento>", "<detEvento", "<evCancCTe>", "<descEvento>", "<nProt>", "<xJust>")
	if !strings.Contains(x, `Id="ID110111`+eventKey+`01"`) {
		t.Fatalf("Id deve ser ID+tpEvento+chCTe+nSeq(2): %s", x)
	}

	x = eventXML(t, TpEventoCCe, 2, "evCCeCTe", map[string]any{
		"descEvento": descCCe, "xCondUso": xCondUsoCCe,
		"infCorrecao": []map[string]any{{"valorAlterado": "X", "campoAlterado": "xNome", "grupoAlterado": "rem", "nroItemAlterado": "1"}},
	})
	inOrder(t, x, "<evCCeCTe>", "<descEvento>", "<infCorrecao>", "<grupoAlterado>", "<campoAlterado>", "<valorAlterado>", "<nroItemAlterado>", "</infCorrecao>", "<xCondUso>")

	x = eventXML(t, TpEventoDesacordo, 1, "evPrestDesacordo", map[string]any{"xObs": "observacao do tomador", "indDesacordoOper": "1", "descEvento": descDesacordo})
	inOrder(t, x, "<evPrestDesacordo>", "<descEvento>", "<indDesacordoOper>", "<xObs>")

	x = eventXML(t, TpEventoCancDesacordo, 1, "evCancPrestDesacordo", map[string]any{"nProtEvPrestDes": "135250000000009", "descEvento": descCancDesacordo})
	inOrder(t, x, "<evCancPrestDesacordo>", "<descEvento>", "<nProtEvPrestDes>")
}

func TestEventInputsRefusedLocally(t *testing.T) {
	s := &CteService{}
	ctx := context.Background()
	check := func(name string, err error) {
		t.Helper()
		if p, ok := err.(*problem.Problem); !ok || p.Status != 422 || p.Code != CodeEventInvalid {
			t.Errorf("%s: want 422 %s, got %#v", name, CodeEventInvalid, err)
		}
	}
	_, err := s.Cancel(ctx, "org", eventKey, "curta", "u", "U")
	check("justificativa curta", err)
	_, err = s.CorrectionLetter(ctx, "org", eventKey, nil, 0, "u", "U")
	check("sem correções", err)
	_, err = s.CorrectionLetter(ctx, "org", eventKey, []Correction{{Group: "rem", Field: "xNome", Value: "A", ItemNumber: 100}}, 0, "u", "U")
	check("item acima de 99", err)
	_, err = s.ServiceDisagreement(ctx, "org", eventKey, "curto", 0, "u", "U")
	check("xObs curto", err)
	_, err = s.nextSequence(ctx, eventKey, TpEventoCCe, MaxEventSequence+1)
	check("seq acima de 20", err)
}

func TestEventAuthorChecks(t *testing.T) {
	str := func(v string) types.AttributeValue { return &types.AttributeValueMemberS{Value: v} }
	ec := &eventContext{cnpj: "12345678000195", cte: map[string]types.AttributeValue{
		"emit_cpf_cnpj": str("99999999000191"),
		"taker":         &types.AttributeValueMemberM{Value: map[string]types.AttributeValue{"doc": str("99999999000191")}},
	}}
	if p, _ := ec.requireIssuer().(*problem.Problem); p == nil || p.Code != CodeNotIssuer {
		t.Fatal("outro emitente deve ser recusado")
	}
	if p, _ := ec.requireTaker().(*problem.Problem); p == nil || p.Code != CodeNotTaker {
		t.Fatal("outro tomador deve ser recusado")
	}
	ec.cte["emit_cpf_cnpj"], ec.cte["taker"] = str("12345678000195"), nil
	if ec.requireIssuer() != nil || ec.requireTaker() != nil {
		t.Fatal("emitente e registro sem tomador seguem para a SEFAZ")
	}
}

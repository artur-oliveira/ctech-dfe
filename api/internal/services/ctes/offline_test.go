package ctes

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	godfe "gopkg.aoctech.app/dfe/go-dfe"

	"gopkg.aoctech.app/dfe/api/internal/repositories"
	"gopkg.aoctech.app/dfe/api/internal/services"
	"gopkg.aoctech.app/dfe/api/internal/services/dfecheck"
	"gopkg.aoctech.app/dfe/go-dfe/validate"
)

type offEgress struct{ calls []godfe.Request }

func (f *offEgress) CertificateB64(context.Context, string) (string, string, error) {
	return "pfx", "pw", nil
}
func (f *offEgress) CallDfe(_ context.Context, r godfe.Request) (godfe.Response, error) {
	f.calls = append(f.calls, r)
	body := map[string]string{"signature": "QUJD+/=="}
	if r.Service == "SignOnly" {
		body = map[string]string{"xml": "<CTe>signed</CTe>"}
	}
	b, _ := json.Marshal(body)
	return godfe.Response{StatusCode: 200, Body: string(b)}, nil
}

func cfgWith(kv map[string]string) map[string]types.AttributeValue {
	m := map[string]types.AttributeValue{}
	for k, v := range kv {
		m[k] = &types.AttributeValueMemberS{Value: v}
	}
	return m
}

func TestResolveCTeEmission(t *testing.T) {
	em, err := resolveCTeEmission(cfgWith(nil), "SP")
	if err != nil || em.tpEmis != "1" || em.offline || em.svc != "" {
		t.Fatalf("normal: %+v %v", em, err)
	}
	off := cfgWith(map[string]string{
		repositories.AttrEmissionType:      repositories.EmissionOffline,
		repositories.AttrContingencySince:  "2026-11-08T09:00:00Z",
		repositories.AttrContingencyReason: "falha de comunicação com a SEFAZ autorizadora",
	})
	if em, err = resolveCTeEmission(off, "SP"); err != nil || em.tpEmis != "2" || !em.offline || em.reason == "" {
		t.Fatalf("offline: %+v %v", em, err)
	}
	off[repositories.AttrContingencyReason] = &types.AttributeValueMemberS{Value: "curta"}
	if _, err = resolveCTeEmission(off, "SP"); err == nil {
		t.Error("short xJust must be refused")
	}
	svc := cfgWith(map[string]string{repositories.AttrEmissionType: repositories.EmissionSVC})
	if em, err = resolveCTeEmission(svc, "SP"); err != nil || em.tpEmis != "7" || em.svc != "SVC-RS" || em.offline {
		t.Fatalf("svc SP: %+v %v", em, err)
	}
	if em, err = resolveCTeEmission(svc, "BA"); err != nil || em.tpEmis != "8" || em.svc != "SVC-SP" {
		t.Fatalf("svc BA: %+v %v", em, err)
	}
	if _, err = resolveCTeEmission(svc, "XX"); err == nil {
		t.Error("unknown UF must be refused")
	}
}

// MOC CT-e: offline carries dhCont/xJust (G009) and sign in the QR Code (G215); SVC carries neither
// (G008, G216); the tpEmis digit of the key is position 35.
func TestBuildCTeContingency(t *testing.T) {
	p := baseParams(t, RoleRemetente)
	p.tpEmis = tpEmisOffline
	p.accessKey = services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 7, p.now, p.tpEmis)
	if p.accessKey[34] != '2' {
		t.Fatalf("key: %s", p.accessKey)
	}
	p.contingencyAt = time.Date(2026, 11, 5, 9, 0, 0, 0, time.UTC)
	p.justification = "falha de comunicação com a SEFAZ autorizadora"
	p.qrSign = "QUJD+/=="
	cte := BuildCTe(p)["CTe"].(map[string]any)
	ide := cte["infCte"].(map[string]any)["ide"].(map[string]any)
	if ide["tpEmis"] != "2" || ide["xJust"] != p.justification || ide["dhCont"] == nil {
		t.Errorf("offline ide: %v", ide)
	}
	if qr := cte["infCTeSupl"].(map[string]any)["qrCodCTe"].(string); !strings.HasSuffix(qr, "&tpAmb=2&sign=QUJD+/==") {
		t.Errorf("qr = %s", qr)
	}

	q := baseParams(t, RoleRemetente)
	q.tpEmis = tpEmisSVCSP
	q.accessKey = services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 7, q.now, q.tpEmis)
	cte = BuildCTe(q)["CTe"].(map[string]any)
	ide = cte["infCte"].(map[string]any)["ide"].(map[string]any)
	if _, ok := ide["dhCont"]; ok {
		t.Error("SVC must not carry dhCont")
	}
	if _, ok := ide["xJust"]; ok {
		t.Error("SVC must not carry xJust")
	}
	if qr := cte["infCTeSupl"].(map[string]any)["qrCodCTe"].(string); strings.Contains(qr, "sign") {
		t.Errorf("SVC QR must not carry sign: %s", qr)
	}
}

func TestSVCCancellationRouting(t *testing.T) {
	for tp, want := range map[string]string{"7": "SVC-RS", "8": "SVC-SP", "1": "", "2": ""} {
		key := services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 7, time.Now(), tp)
		if got := eventContingency(TpEventoCancelamento, key); got != want {
			t.Errorf("tpEmis %s: %q, want %q", tp, got, want)
		}
	}
	key := services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 7, time.Now(), "7")
	if eventContingency("110110", key) != "" {
		t.Error("only the cancellation goes to the SVC")
	}
}

func TestOfflinePending(t *testing.T) {
	f := &offEgress{}
	xml, err := services.SignOnlyViaEgress(context.Background(), f, "org", cnpjA, "SP", "hom", "cte", map[string]any{"CTe": map[string]any{}})
	if err != nil || xml != "<CTe>signed</CTe>" || f.calls[0].DocType != "cte" {
		t.Fatalf("%q %v", xml, err)
	}
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	item := map[string]types.AttributeValue{}
	if err := markOfflinePending(item, services.WorkerMessage{SefazService: sefazServiceAutorizacao}, xml, now); err != nil {
		t.Fatal(err)
	}
	if got := item["contingency_deadline"].(*types.AttributeValueMemberS).Value; got != "2026-11-15T10:00:00Z" {
		t.Errorf("deadline (168 h) = %s", got)
	}
	if services.ContingencySignedXML(item) != xml {
		t.Error("signed xml")
	}
}

// The offline ide (dhCont, xJust) passes the XSD parity and the MOC rules.
func TestBuildCTeOfflineParidadeComXSD(t *testing.T) {
	p := baseParams(t, RoleRemetente)
	p.tpEmis = tpEmisOffline
	p.accessKey = services.GenerateAccessKey("SP", cnpjA, services.ModelCTe, 1, 7, p.now, p.tpEmis)
	p.contingencyAt = p.now.Add(-time.Hour)
	p.justification = "falha de comunicação com a SEFAZ autorizadora"
	p.qrSign = "QUJD+/=="
	cte := BuildCTe(p)["CTe"].(map[string]any)
	for _, v := range dfecheck.Shadow(dfecheck.SpecCTe, "cte", p.accessKey, cte) {
		if v.Code == "rule" && v.Severity != validate.SevError {
			continue
		}
		t.Errorf("%s %s: %s %s", v.Path, v.Code, v.Rule, v.Message)
	}
}

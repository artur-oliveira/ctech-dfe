package mdfes

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
)

type offlineEgress struct{ calls []godfe.Request }

func (f *offlineEgress) CertificateB64(context.Context, string) (string, string, error) {
	return "pfx", "pw", nil
}
func (f *offlineEgress) CallDfe(_ context.Context, r godfe.Request) (godfe.Response, error) {
	f.calls = append(f.calls, r)
	body := map[string]string{"signature": "QUJD+/=="}
	if r.Service == "SignOnly" {
		body = map[string]string{"xml": "<MDFe>signed</MDFe>"}
	}
	b, _ := json.Marshal(body)
	return godfe.Response{StatusCode: 200, Body: string(b)}, nil
}

func TestResolveMDFeEmission(t *testing.T) {
	cfg := map[string]types.AttributeValue{}
	if tp, off := resolveMDFeEmission(cfg); tp != "1" || off {
		t.Fatalf("normal: %s %v", tp, off)
	}
	cfg[repositories.AttrEmissionType] = &types.AttributeValueMemberS{Value: repositories.EmissionOffline}
	if tp, off := resolveMDFeEmission(cfg); tp != "2" || !off {
		t.Fatalf("offline: %s %v", tp, off)
	}
}

// MOC MDF-e: tpEmis 2 goes in ide and in the key (position 35), the layout has no dhCont/xJust, and
// the QR Code carries sign (F117/F118) only in contingency.
func TestBuildMDFeOffline(t *testing.T) {
	p := baseParams(nil)
	p.tpEmis = TpEmisContingencia
	p.accessKey = services.GenerateAccessKey("SP", "12345678000190", services.ModelMDFe, 1, 1, p.now, p.tpEmis)
	if p.accessKey[34] != '2' {
		t.Fatalf("tpEmis digit of the key: %s", p.accessKey)
	}
	p.qrSign = "QUJD+/=="
	body := BuildMDFe(p)["MDFe"].(map[string]any)
	inf := body["infMDFe"].(map[string]any)
	ide := inf["ide"].(map[string]any)
	if ide["tpEmis"] != "2" {
		t.Errorf("tpEmis = %v", ide["tpEmis"])
	}
	for _, k := range []string{"dhCont", "xJust"} {
		if _, ok := ide[k]; ok {
			t.Errorf("MDF-e layout has no %s", k)
		}
	}
	qr := body["infMDFeSupl"].(map[string]any)["qrCodMDFe"].(string)
	if want := "?chMDFe=" + p.accessKey + "&tpAmb=2&sign=QUJD+/=="; !strings.HasSuffix(qr, want) {
		t.Errorf("qr = %s", qr)
	}
	p.qrSign = ""
	qr = BuildMDFe(p)["MDFe"].(map[string]any)["infMDFeSupl"].(map[string]any)["qrCodMDFe"].(string)
	if strings.Contains(qr, "sign") {
		t.Errorf("normal QR must not carry sign: %s", qr)
	}
}

func TestOfflineSignAndPending(t *testing.T) {
	f := &offlineEgress{}
	sig, err := services.SignTextViaEgress(context.Background(), f, "org", "12345678000190", "mdfe", "chave")
	if err != nil || sig != "QUJD+/==" || f.calls[0].DocType != "mdfe" || f.calls[0].Service != "SignText" {
		t.Fatalf("sign text: %q %v %+v", sig, err, f.calls)
	}
	xml, err := services.SignOnlyViaEgress(context.Background(), f, "org", "12345678000190", "SP", "hom", "mdfe", map[string]any{"MDFe": map[string]any{}})
	if err != nil || xml != "<MDFe>signed</MDFe>" || f.calls[1].Service != "SignOnly" {
		t.Fatalf("sign only: %q %v", xml, err)
	}
	now := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	item := map[string]types.AttributeValue{}
	msg := services.WorkerMessage{AccessKey: "k", DocType: "mdfe", SefazService: sefazServiceAutorizacao}
	if err := markOfflinePending(item, msg, xml, now); err != nil {
		t.Fatal(err)
	}
	if item["contingency_state"].(*types.AttributeValueMemberS).Value != "pending" ||
		item["contingency_sweep"].(*types.AttributeValueMemberS).Value != "1" {
		t.Errorf("index attributes: %v", item)
	}
	// MOC: transmit within 168 hours of the offline emission.
	if got := item["contingency_deadline"].(*types.AttributeValueMemberS).Value; got != "2026-11-15T10:00:00Z" {
		t.Errorf("deadline = %s", got)
	}
	if services.ContingencySignedXML(item) != xml {
		t.Error("stored signed xml")
	}
	var stored services.WorkerMessage
	_ = json.Unmarshal([]byte(item["contingency_msg"].(*types.AttributeValueMemberS).Value), &stored)
	if stored.SefazService != sefazServiceAutorizacao || stored.Body.(map[string]any)["@signed_xml"] != xml {
		t.Errorf("stored message: %+v", stored)
	}
}

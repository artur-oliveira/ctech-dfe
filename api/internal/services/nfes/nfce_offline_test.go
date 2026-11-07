package nfes

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

type fakeEgress struct {
	got  godfe.Request
	resp godfe.Response
}

func (f *fakeEgress) CertificateB64(context.Context, string) (string, string, error) {
	return "pfx", "pw", nil
}
func (f *fakeEgress) CallDfe(_ context.Context, r godfe.Request) (godfe.Response, error) {
	f.got = r
	return f.resp, nil
}

const offlineKey = "35200514200166000187650010000000011000000018"

// Manual DANFE NFC-e 4.4.2: chave|3|tpAmb|dia|vNF|tp_idDest|idDest|assinatura.
func TestQRV3OfflineParams(t *testing.T) {
	dh := "2026-11-08T10:30:00-03:00"
	cases := []struct {
		name, doc, want string
	}{
		{"cpf", "12345678909", offlineKey + "|3|2|08|125.50|2|12345678909"},
		{"cnpj", "11222333000181", offlineKey + "|3|2|08|125.50|1|11222333000181"},
		// recipient not identified: parameters 6 and 7 hold only the separator
		{"none", "", offlineKey + "|3|2|08|125.50||"},
	}
	for _, c := range cases {
		if got := qrV3OfflineParams(offlineKey, 2, dh, "125.50", c.doc); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
	signed := qrV3OfflineParams(offlineKey, 1, dh, "1234.50", "")
	if want := "https://x/qrcode?p=" + signed + "|SIG="; buildQRCodeV3Offline("https://x/qrcode", signed, "SIG=") != want {
		t.Errorf("url = %s", buildQRCodeV3Offline("https://x/qrcode", signed, "SIG="))
	}
	if signed != offlineKey+"|3|1|08|1234.50||" {
		t.Errorf("vNF must have a dot and no thousands separator: %s", signed)
	}
}

func TestSignTextViaEgress(t *testing.T) {
	body, _ := json.Marshal(map[string]string{"signature": "QUJD"})
	f := &fakeEgress{resp: godfe.Response{StatusCode: 200, Body: string(body)}}
	s := (&NfceService{}).WithEgress(f)
	sig, err := s.signText(context.Background(), "org", "11222333000181", "texto")
	if err != nil || sig != "QUJD" {
		t.Fatalf("sig=%q err=%v", sig, err)
	}
	if f.got.Service != "SignText" || f.got.DocType != "nfce" || f.got.Body["text"] != "texto" || f.got.CertificateB64 != "pfx" {
		t.Errorf("request = %+v", f.got)
	}
	f.resp = godfe.Response{StatusCode: 400, Body: `{"detail":"x","status":400}`}
	if _, err := s.signText(context.Background(), "org", "d", "t"); err == nil {
		t.Error("a non-200 answer must be an error")
	}
}

// Manual DANFE NFC-e 4.3.6.2: the v2 offline hash example.
func TestQRV2OfflineManualExample(t *testing.T) {
	params := qrV2OfflineParams("28170800156225000131650110000151349562040824", 1, "2017-08-02T10:00:00-03:00", "60.90", "yzGYhUx1/XYYzksWB+fPR3Qc50c=", "1")
	want := "28170800156225000131650110000151349562040824|2|1|02|60.90|797a4759685578312f5859597a6b7357422b6650523351633530633d|1"
	if params != want {
		t.Fatalf("params = %s", params)
	}
	got := buildQRCodeV2Offline("https://x/qrcode?", params, "SEU-CODIGO-CSC-CONTRIBUINTE-36-CARACTERES")
	if !strings.HasSuffix(got, "|1|4615A93BB0D7C4E780F8D30EE77EDD5BA55C7D66") {
		t.Errorf("hash mismatch: %s", got)
	}
}

func TestResolveNFCeEmission(t *testing.T) {
	cfg := map[string]types.AttributeValue{}
	if _, off, err := resolveNFCeEmission(cfg); off || err != nil {
		t.Fatalf("normal: off=%v err=%v", off, err)
	}
	cfg[repositories.AttrEmissionType] = &types.AttributeValueMemberS{Value: repositories.EmissionOffline}
	if _, _, err := resolveNFCeEmission(cfg); err == nil {
		t.Error("offline without contingency_since must fail")
	}
	cfg[repositories.AttrContingencySince] = &types.AttributeValueMemberS{Value: "2026-11-08T10:00:00-03:00"}
	cfg[repositories.AttrContingencyReason] = &types.AttributeValueMemberS{Value: "SEFAZ fora do ar ha horas"}
	m, off, err := resolveNFCeEmission(cfg)
	if err != nil || !off || m.TpEmis != "9" || m.Validate() != nil {
		t.Fatalf("offline mode = %+v off=%v err=%v", m, off, err)
	}
}

func TestSignOfflineXMLAndPending(t *testing.T) {
	signed := `<enviNFe><NFe><infNFe/><infNFeSupl><qrCode>` + qrPlaceholder + `</qrCode></infNFeSupl><Signature><DigestValue>yzGYhUx1/XYYzksWB+fPR3Qc50c=</DigestValue></Signature></NFe></enviNFe>`
	body, _ := json.Marshal(map[string]string{"xml": signed})
	f := &fakeEgress{resp: godfe.Response{StatusCode: 200, Body: string(body)}}
	s := (&NfceService{}).WithEgress(f)
	q := &offlineQR{orgPK: "o", issuerDoc: "11222333000181", uf: "SP", environment: 1, version: 2,
		dhEmi: "2026-11-08T10:00:00-03:00", vNF: "10.00", accessKey: offlineKey, csc: "CSC", cscID: "1"}
	out, err := s.signOfflineXML(context.Background(), q, map[string]any{"enviNFe": map[string]any{}}, "producao")
	if err != nil || strings.Contains(out, qrPlaceholder) || !strings.Contains(out, "https://www.nfce.fazenda.sp.gov.br/qrcode?p="+offlineKey+"|2|1|08|10.00|") {
		t.Fatalf("out=%s err=%v", out, err)
	}
	if f.got.Service != "SignOnly" || f.got.DocType != "nfce" || f.got.Environment != "producao" {
		t.Errorf("request = %+v", f.got)
	}

	item := map[string]types.AttributeValue{}
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	if err := markOfflinePending(item, services.WorkerMessage{AccessKey: offlineKey}, out, now); err != nil {
		t.Fatal(err)
	}
	if item["contingency_sweep"].(*types.AttributeValueMemberS).Value != "1" ||
		item["contingency_state"].(*types.AttributeValueMemberS).Value != "pending" ||
		item["contingency_deadline"].(*types.AttributeValueMemberS).Value != "2026-11-11T12:00:00Z" {
		t.Errorf("index attributes = %v", item)
	}
	var msg map[string]any
	_ = json.Unmarshal([]byte(item["contingency_msg"].(*types.AttributeValueMemberS).Value), &msg)
	if msg["sefaz_service"] != "NFeAutorizacao" || msg["body"].(map[string]any)["@signed_xml"] != out {
		t.Errorf("msg = %v", msg)
	}
}

func TestOfflineSignedXML(t *testing.T) {
	item := map[string]types.AttributeValue{}
	if offlineSignedXML(item) != "" {
		t.Error("no message, no xml")
	}
	_ = markOfflinePending(item, services.WorkerMessage{}, "<enviNFe/>", time.Now())
	if offlineSignedXML(item) != "<enviNFe/>" {
		t.Error("stored xml not recovered")
	}
}

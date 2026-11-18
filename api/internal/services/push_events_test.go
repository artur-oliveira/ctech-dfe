package services

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.aoctech.app/api-commons/webpush"
)

// testdata/push-notification-v1.json is byte for byte go-common/webpush/testdata/notification-v1.json, the contract
// the push-client service worker is tested against. A change there is a contract change (bump PushPayloadVersion).
func TestBuildPushPayloadMatchesTheGoldenContract(t *testing.T) {
	raw, err := os.ReadFile("testdata/push-notification-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildPushPayload(PushSendJob{
		EventID: "01JEXEMPLOEVENTID0000000000", Type: PushEventRejected, Title: "NF-e rejeitada · Empresa Exemplo",
		Body: "Toque para ver o motivo.", Tag: "document.rejected:a1b2c3d4",
		Navigate: "/nfe/detail?key=35260112345678000190550010000000011000000011&org=ORG_EXEMPLO",
	})
	if err != nil {
		t.Fatal(err)
	}
	var want, have map[string]any
	if err := json.Unmarshal(raw, &want); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(got, &have); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(want, have) {
		t.Errorf("payload drifted from the golden contract:\n got  %s\n want %s", got, raw)
	}
}

func TestBuildPushPayloadRefusesWhatTheClientWouldNotOpen(t *testing.T) {
	ok := PushSendJob{EventID: "e", Type: PushEventFailed, Title: "t", Body: "b", Tag: "g", Navigate: "/nfe"}
	for _, nav := range []string{"https://evil.example/", "//evil.example", `/\evil.example`, "nfe/detail", ""} {
		j := ok
		j.Navigate = nav
		if _, err := BuildPushPayload(j); err == nil {
			t.Errorf("navigate %q must be refused", nav)
		}
	}
	j := ok
	j.Body = strings.Repeat("x", PushPayloadBudget)
	if _, err := BuildPushPayload(j); err == nil {
		t.Error("a payload over the budget must be refused")
	}
	if _, err := BuildPushPayload(ok); err != nil {
		t.Errorf("a plain job must build: %v", err)
	}
}

func TestPushTitleGenderAndTruncation(t *testing.T) {
	cases := []struct{ event, doc, company, want string }{
		{PushEventAuthorized, "nfe", "Acme", "NF-e autorizada · Acme"},
		{PushEventAuthorized, "cte", "Acme", "CT-e autorizado · Acme"},
		{PushEventRejected, "bpe", "Acme", "BP-e rejeitado · Acme"},
		{PushEventRejected, "nfcom", "Acme", "NFCom rejeitada · Acme"},
		{PushEventFailed, "mdfe", "Acme", "Falha ao emitir MDF-e · Acme"},
		{PushEventDistribution, "", "Acme", "Novos documentos recebidos · Acme"},
		{PushEventRejected, "nfe", "", "NF-e rejeitada"},
		{PushEventRejected, "nfe", "  Acme   Transportes  ", "NF-e rejeitada · Acme Transportes"},
		{PushEventRejected, "nfe", "Transportadora Exemplo Ltda de Longo Nome", "NF-e rejeitada · Transportadora Exemplo Ltda d…"},
	}
	for _, c := range cases {
		if got := pushTitle(c.event, c.doc, c.company); got != c.want {
			t.Errorf("pushTitle(%q, %q, %q) = %q, want %q", c.event, c.doc, c.company, got, c.want)
		}
	}
	if name := pushCompanyName(strings.Repeat("é", 80)); len([]rune(name)) != PushCompanyNameMax {
		t.Errorf("a cut name must be %d characters with the ellipsis, got %d", PushCompanyNameMax, len([]rune(name)))
	}
}

func TestPushNavigate(t *testing.T) {
	key := strings.Repeat("1", 44)
	cases := []struct{ event, doc, key, want string }{
		{PushEventRejected, "nfe", key, "/nfe/detail?key=" + key + "&org=CNPJ_1"},
		{PushEventAuthorized, "nfse", "DPS1", "/nfse/detail?id=DPS1&org=CNPJ_1"},
		{PushEventFailed, "bpe", key, "/bpe/detail?key=" + key + "&org=CNPJ_1"},
		{PushEventDistribution, "nfe", key, "/nfe?tab=distribuicao&org=CNPJ_1"},
		{PushEventDistribution, "cte", key, "/cte/distributions?org=CNPJ_1"},
		{PushEventDistribution, "mdfe", key, "/mdfe/distributions?org=CNPJ_1"},
		{PushEventDistribution, "unknown", key, "/nfe?tab=distribuicao&org=CNPJ_1"},
		{PushEventRejected, "unknown", key, "/?org=CNPJ_1"},
	}
	for _, c := range cases {
		if got := pushNavigate(c.event, c.doc, c.key, "CNPJ_1"); got != c.want {
			t.Errorf("pushNavigate(%q, %q) = %q, want %q", c.event, c.doc, got, c.want)
		}
	}
	// A key with reserved characters never escapes its parameter.
	if got := pushNavigate(PushEventRejected, "nfe", "a&b=c", "CNPJ_1"); !strings.Contains(got, "key=a%26b%3Dc&") {
		t.Errorf("key not escaped: %s", got)
	}
}

func TestPushTagAndTopicAreStablePerCompanyAndType(t *testing.T) {
	tag, topic := pushTag(PushEventRejected, "CNPJ_1")
	tag2, topic2 := pushTag(PushEventRejected, "CNPJ_1")
	if tag != tag2 || topic != topic2 {
		t.Error("tag and topic must be deterministic")
	}
	if otherTag, otherTopic := pushTag(PushEventRejected, "CNPJ_2"); otherTag == tag || otherTopic == topic {
		t.Error("another company must not share a tag or a topic")
	}
	if len(topic) > 32 || strings.ContainsAny(topic, "+/=") {
		t.Errorf("topic %q must be at most 32 URL-safe characters", topic)
	}
	if !strings.HasPrefix(tag, PushEventRejected+":") {
		t.Errorf("tag = %q", tag)
	}
}

func TestPushDeviceLabel(t *testing.T) {
	cases := map[string]string{
		"Mozilla/5.0 (Linux; Android 14) AppleWebKit Chrome/130 Mobile Safari/537.36":                 "Chrome no Android",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/130 Safari/537.36 Edg/130":                  "Edge no Windows",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit Version/17 Safari/605.1.15":      "Safari no macOS",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_4 like Mac OS X) AppleWebKit Version/18 Mobile Safari": "Safari no iOS",
		"Mozilla/5.0 (X11; Linux x86_64; rv:130.0) Gecko/20100101 Firefox/130.0":                      "Firefox no Linux",
		"": "Navegador",
	}
	for ua, want := range cases {
		if got := pushDeviceLabel(ua); got != want {
			t.Errorf("pushDeviceLabel(%q) = %q, want %q", ua, got, want)
		}
	}
}

// The API holds only the public key; the kid it publishes must be the one the sender derives from the private key.
func TestVapidKIDMatchesTheSendersKey(t *testing.T) {
	_, key, err := webpush.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	kid, ok := vapidKID(key.PublicBase64URL())
	if !ok || kid != key.KID {
		t.Fatalf("vapidKID = %q %v, want %q", kid, ok, key.KID)
	}
	for _, bad := range []string{"", "not base64!", "AAAA", key.PublicBase64URL()[:40]} {
		if _, ok := vapidKID(bad); ok {
			t.Errorf("vapidKID(%q) must be refused", bad)
		}
	}
}

func TestPushEventTypesAreTheOnesWeCanSend(t *testing.T) {
	want := []string{PushEventDistribution, PushEventAuthorized, PushEventRejected, PushEventFailed}
	if !reflect.DeepEqual(PushEventTypes, want) {
		t.Errorf("PushEventTypes = %v", PushEventTypes)
	}
	if !reflect.DeepEqual(PushDefaultEvents, []string{PushEventDistribution}) {
		t.Errorf("only the received documents start on: %v", PushDefaultEvents)
	}
	for _, e := range PushEventTypes {
		ttl, urgency := pushDelivery(e)
		if ttl <= 0 || urgency == "" || pushWindow(e) <= 0 || pushBody(e) == "" {
			t.Errorf("event %q is missing delivery, window or body", e)
		}
	}
}

package dfeproblem

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// A retired route answers 410 and says where the thing moved.
//
// Not a 404: 404 says "there is nothing here", which sends somebody looking for
// a typo in their own integration. 410 says "this existed and is gone", which is
// the true thing and the one that makes them read the message.
func TestRetiredSaysWhereItWent(t *testing.T) {
	p := Retired("os convites agora são gerenciados na sua conta CTech")
	if p.Status != http.StatusGone {
		t.Fatalf("status = %d, want 410", p.Status)
	}
	if !strings.Contains(p.Detail, "conta CTech") {
		t.Errorf("the detail does not say where it moved: %q", p.Detail)
	}
	// A machine-readable type, so a client can branch on "retired" rather than
	// matching prose that will be rewritten.
	if p.Type == "" || !strings.Contains(p.Type, "retired") {
		t.Errorf("type = %q, want something a client can branch on", p.Type)
	}
}

// The 402 body is a public contract: the UI reads reason, meter, plan and the
// quota numbers by these exact keys, and a zero quota number stays omitted.
func TestQuotaExceededWireShape(t *testing.T) {
	b, err := QuotaExceeded("nfe", "free", 3, 3, "limite atingido").MarshalBody()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"type": "/problems/payment-required", "status": float64(http.StatusPaymentRequired),
		"reason": ReasonQuotaExceeded, "meter": "nfe", "plan": "free", "quota_limit": float64(3), "quota_used": float64(3),
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}
	b, _ = QuotaExceeded("nfe", "free", 0, 0, "x").MarshalBody()
	m = map[string]any{}
	_ = json.Unmarshal(b, &m)
	if _, ok := m["quota_limit"]; ok {
		t.Errorf("zero quota_limit must be omitted: %s", b)
	}
	b, _ = PaymentRequired(ReasonSubscriptionPastDue, "x").MarshalBody()
	_ = json.Unmarshal(b, &m)
	if m["reason"] != ReasonSubscriptionPastDue {
		t.Errorf("reason = %v: %s", m["reason"], b)
	}
}

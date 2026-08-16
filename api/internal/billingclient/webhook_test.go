package billingclient

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"testing"
	"time"
)

// sign is ctech-billing's `services.Sign`, written out here rather than
// imported — the two services share no module, so this test is what proves the
// two implementations agree. If billing ever changes the material it signs, this
// is the thing that must be updated first and deliberately.
func sign(secret, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

const testSecret = "a-secret-both-sides-hold"

func TestVerifyWebhookAcceptsWhatBillingSigns(t *testing.T) {
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"id":"evt_1","type":"subscription.activated"}`)

	if err := VerifyWebhook(testSecret, ts, "v1="+sign(testSecret, ts, body), body, now); err != nil {
		t.Fatalf("a genuine delivery must verify: %v", err)
	}
}

// TestVerifyWebhookRejects walks every way a delivery can fail to be genuine.
//
// They are one table and one error on purpose: a response that told a prober
// which half of the scheme they had right would be a hint worth having.
func TestVerifyWebhookRejects(t *testing.T) {
	now := time.Now()
	ts := strconv.FormatInt(now.Unix(), 10)
	body := []byte(`{"id":"evt_1","type":"subscription.activated"}`)
	good := "v1=" + sign(testSecret, ts, body)

	stale := strconv.FormatInt(now.Add(-MaxClockSkew-time.Minute).Unix(), 10)
	future := strconv.FormatInt(now.Add(MaxClockSkew+time.Minute).Unix(), 10)

	for _, tc := range []struct {
		name      string
		secret    string
		timestamp string
		signature string
		body      []byte
	}{
		{"wrong secret", "not-the-secret", ts, good, body},
		{"no secret configured", "", ts, good, body},
		{"missing signature", testSecret, ts, "", body},
		{"missing timestamp", testSecret, "", good, body},
		{"unknown scheme version", testSecret, ts, "v2=" + sign(testSecret, ts, body), body},
		{"signature is not hex", testSecret, ts, "v1=zzzz", body},
		{"timestamp is not a number", testSecret, "ontem", good, body},
		// The body was altered after signing — the case the whole scheme exists
		// for. A consumer that parsed and re-encoded before verifying would let
		// this through, because key order and whitespace are part of the material.
		{"tampered body", testSecret, ts, good, []byte(`{"id":"evt_1","type":"subscription.canceled"}`)},
		// The timestamp is inside the signed material, so a captured delivery is
		// perfectly signed forever. This is what bounds the replay window.
		{"replay from before the window", testSecret, stale, "v1=" + sign(testSecret, stale, body), body},
		{"timestamp from the future", testSecret, future, "v1=" + sign(testSecret, future, body), body},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := VerifyWebhook(tc.secret, tc.timestamp, tc.signature, tc.body, now); err == nil {
				t.Fatal("this delivery must not verify")
			}
		})
	}
}

package v1

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"gopkg.aoctech.app/dfe/api/internal/billingclient"
)

// The webhook accepts subscription state changes from outside this service, and
// its only door is the HMAC. These tests exercise the mounted route rather than
// the verifier alone, because two things can go wrong above the crypto: the
// route can be mounted when it should not be, and a rejection can happen after
// the handler has already acted.
//
// The service is nil throughout. A genuine delivery would reach it and panic —
// which is exactly why every case here is one that must be refused *before* any
// work happens. A test that got further would be testing the wrong thing.

const webhookPath = "/v1/internal/webhooks/billing"

func appWithWebhook(t *testing.T, secret string) *fiber.App {
	t.Helper()
	app := fiber.New()
	registerBillingWebhook(app, nil, secret)
	return app
}

func post(t *testing.T, app *fiber.App, body, timestamp, signature, eventID string) int {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, webhookPath, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if timestamp != "" {
		req.Header.Set(billingclient.HeaderTimestamp, timestamp)
	}
	if signature != "" {
		req.Header.Set(billingclient.HeaderSignature, signature)
	}
	if eventID != "" {
		req.Header.Set(billingclient.HeaderEventID, eventID)
	}
	resp, err := app.Test(req, fiber.TestConfig{Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func hmacOf(secret, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write([]byte(body))
	return "v1=" + hex.EncodeToString(mac.Sum(nil))
}

// TestWebhookIsNotMountedWithoutASecret is the important one.
//
// A signature check that cannot run is not a signature check. Without the shared
// secret the route must not exist at all, so a deployment that lost it answers
// 404 — something an operator notices — rather than accepting whatever arrives.
func TestWebhookIsNotMountedWithoutASecret(t *testing.T) {
	app := appWithWebhook(t, "")
	if status := post(t, app, `{"id":"evt_1"}`, "1", "v1=deadbeef", "evt_1"); status != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — the route must not exist without a secret", status)
	}
}

// TestWebhookRejectsAForgedDelivery: the body is signed, so altering it after
// signing must fail even though the signature itself is well formed.
func TestWebhookRejectsAForgedDelivery(t *testing.T) {
	const secret = "shared-with-billing"
	app := appWithWebhook(t, secret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	signed := `{"id":"evt_1","type":"subscription.activated","data":{"object":"subscription","id":"sub_1"}}`
	forged := `{"id":"evt_1","type":"subscription.canceled","data":{"object":"subscription","id":"sub_1"}}`

	for _, tc := range []struct{ name, body, ts, sig string }{
		{"body altered after signing", forged, ts, hmacOf(secret, ts, signed)},
		{"signed with another secret", signed, ts, hmacOf("guessed", ts, signed)},
		{"no signature at all", signed, ts, ""},
		{"no timestamp", signed, "", hmacOf(secret, ts, signed)},
		{"replayed from an hour ago", signed,
			strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10),
			hmacOf(secret, strconv.FormatInt(time.Now().Add(-time.Hour).Unix(), 10), signed)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if status := post(t, app, tc.body, tc.ts, tc.sig, "evt_1"); status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", status)
			}
		})
	}
}

// TestWebhookRejectsADeliveryWithNoEventID: without an id there is nothing to
// deduplicate on, and billing delivers at least once. Accepting one would mean
// acting twice on the same change the first time a retry arrived.
//
// It is refused *after* the signature check, so an unsigned request never
// learns whether the id mattered.
func TestWebhookRejectsADeliveryWithNoEventID(t *testing.T) {
	const secret = "shared-with-billing"
	app := appWithWebhook(t, secret)
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	body := `{"type":"subscription.activated"}`

	if status := post(t, app, body, ts, hmacOf(secret, ts, body), ""); status != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", status)
	}
}

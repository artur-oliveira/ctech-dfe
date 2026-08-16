package billingclient

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// Verifying ctech-billing's outbound webhooks.
//
// This mirrors `services.Sign` in ctech-billing, and the mirroring is the point:
// one signature scheme with two implementations is how a rollout discovers that
// trailing whitespace matters. What is copied is the algorithm, not the code —
// the two services do not share a module — so any change to it must land on both
// sides before it lands on either.

// Webhook header names, as billing sets them
// (ctech-billing/api/internal/services/delivering.go).
const (
	HeaderEventID   = "X-Billing-Event-Id"
	HeaderEventType = "X-Billing-Event-Type"
	HeaderTimestamp = "X-Billing-Timestamp"
	HeaderSignature = "X-Billing-Signature"
)

// signaturePrefix versions the scheme. Billing sends `v1=<hex>`, and the prefix
// is what lets a future scheme be introduced without every consumer breaking on
// the first delivery of it.
const signaturePrefix = "v1="

// MaxClockSkew is how far a delivery's timestamp may be from now.
//
// Five minutes each way. The timestamp is inside the signed material, so this is
// what actually bounds a replay: without it a captured delivery stays valid
// forever, and with it an attacker has five minutes to use one — during which
// the event id has already been recorded as processed.
const MaxClockSkew = 5 * time.Minute

// ErrBadSignature reports a delivery this service will not act on: wrong
// signature, missing headers, or a timestamp outside the tolerance.
//
// One error for all three on purpose. A response that distinguished "bad
// signature" from "stale timestamp" would tell whoever is probing which half of
// the scheme they have got right.
var ErrBadSignature = errors.New("billing: the delivery signature did not verify")

// WebhookEvent is what a verified delivery says.
//
// It carries an id and a type and nothing else worth acting on. The body names
// what changed; it never says what is now true, and this service re-reads
// billing to find that out (see BillingService.SyncBySubscription).
type WebhookEvent struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Livemode   bool   `json:"livemode"`
	OccurredAt string `json:"occurred_at"`
	Data       struct {
		Object         string `json:"object"`
		ID             string `json:"id"`
		SubscriptionID string `json:"subscription_id"`
	} `json:"data"`
}

// VerifyWebhook checks a delivery against the shared secret.
//
// `body` must be the exact bytes received. Re-encoding a parsed payload and
// signing that would verify a different document than the one billing signed —
// key order and whitespace are part of the material.
func VerifyWebhook(secret, timestamp, signature string, body []byte, now time.Time) error {
	if secret == "" || timestamp == "" || signature == "" {
		return ErrBadSignature
	}
	if !strings.HasPrefix(signature, signaturePrefix) {
		return ErrBadSignature
	}
	sent, err := hex.DecodeString(strings.TrimPrefix(signature, signaturePrefix))
	if err != nil {
		return ErrBadSignature
	}

	epoch, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return ErrBadSignature
	}
	if drift := now.Sub(time.Unix(epoch, 0)); drift > MaxClockSkew || drift < -MaxClockSkew {
		return ErrBadSignature
	}

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp))
	mac.Write([]byte("."))
	mac.Write(body)
	// Constant-time, because a byte-by-byte comparison leaks the correct prefix
	// to anyone willing to time the responses.
	if !hmac.Equal(sent, mac.Sum(nil)) {
		return ErrBadSignature
	}
	return nil
}

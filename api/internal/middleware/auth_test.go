package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/middleware"

	"github.com/golang-jwt/jwt/v5"
)

const (
	testIssuer   = "https://accounts.example"
	testAudience = "https://dfe-api.example"
)

// jwksServer serves a JWKS whose key set can be swapped at runtime, simulating
// a key rotation at the identity provider.
type jwksServer struct {
	srv    *httptest.Server
	keys   atomic.Value // []map[string]any
	hits   atomic.Int64
	status atomic.Int64
}

func newJWKSServer(t *testing.T) *jwksServer {
	t.Helper()
	js := &jwksServer{}
	js.status.Store(http.StatusOK)
	js.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		js.hits.Add(1)
		code := int(js.status.Load())
		if code != http.StatusOK {
			w.WriteHeader(code)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		keys, _ := js.keys.Load().([]map[string]any)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"keys": keys})
	}))
	t.Cleanup(js.srv.Close)
	return js
}

func (js *jwksServer) publish(pub *rsa.PublicKey, kid string) {
	js.keys.Store([]map[string]any{{
		"kty": "RSA",
		"kid": kid,
		"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
	}})
}

func signToken(t *testing.T, key *rsa.PrivateKey, kid, issuer, audience string) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub": "user-1",
		"iss": issuer,
		"aud": []string{audience},
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signed
}

func newVerifier(js *jwksServer) *middleware.Verifier {
	return middleware.NewVerifier(js.srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16))
}

func TestVerify_ValidToken(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&key.PublicKey, "kid-1")

	sub, err := newVerifier(js).Verify(context.Background(), signToken(t, key, "kid-1", testIssuer, testAudience))
	if err != nil {
		t.Fatalf("expected valid token, got %v", err)
	}
	if sub != "user-1" {
		t.Errorf("expected sub user-1, got %q", sub)
	}
}

// A token from an unexpected issuer must be rejected even though the signature
// verifies against a published key.
func TestVerify_WrongIssuer(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&key.PublicKey, "kid-1")

	_, err := newVerifier(js).Verify(context.Background(),
		signToken(t, key, "kid-1", "https://evil.example", testAudience))
	if err == nil {
		t.Fatal("expected token with wrong iss to be rejected")
	}
}

func TestVerify_WrongAudience(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&key.PublicKey, "kid-1")

	_, err := newVerifier(js).Verify(context.Background(),
		signToken(t, key, "kid-1", testIssuer, "https://other.example"))
	if err == nil {
		t.Fatal("expected token with wrong aud to be rejected")
	}
}

// An unknown kid must be rejected outright — never verified against keys[0].
// Here the JWKS holds a DIFFERENT key than the one that signed the token, so a
// keys[0] fallback would have failed the signature check anyway; the point is
// that we never even try, and we don't accept a token signed by the wrong key.
func TestVerify_UnknownKID_Rejected(t *testing.T) {
	signing, _ := rsa.GenerateKey(rand.Reader, 2048)
	published, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&published.PublicKey, "kid-published")

	_, err := newVerifier(js).Verify(context.Background(),
		signToken(t, signing, "kid-unknown", testIssuer, testAudience))
	if err == nil {
		t.Fatal("expected unknown kid to be rejected")
	}
}

// The dangerous case: the JWKS' first key is the one the attacker's token claims
// a different kid for. With a keys[0] fallback a token signed by the published
// key but carrying a bogus kid would be accepted. It must not be.
func TestVerify_BogusKID_NoFallbackToFirstKey(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&key.PublicKey, "kid-real")

	_, err := newVerifier(js).Verify(context.Background(),
		signToken(t, key, "kid-does-not-exist", testIssuer, testAudience))
	if err == nil {
		t.Fatal("token with unlisted kid must be rejected, not verified against keys[0]")
	}
}

// After the provider rotates its key, a token with the new kid must succeed
// without waiting for the 1h cache TTL to expire.
func TestVerify_RefetchesJWKSOnKeyRotation(t *testing.T) {
	oldKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&oldKey.PublicKey, "kid-old")
	v := newVerifier(js)

	// Warm the cache with the old key set.
	if _, err := v.Verify(context.Background(), signToken(t, oldKey, "kid-old", testIssuer, testAudience)); err != nil {
		t.Fatalf("old key should verify: %v", err)
	}
	hitsBefore := js.hits.Load()

	// Provider rotates to a new key.
	newKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	js.publish(&newKey.PublicKey, "kid-new")

	sub, err := v.Verify(context.Background(), signToken(t, newKey, "kid-new", testIssuer, testAudience))
	if err != nil {
		t.Fatalf("expected refetch to pick up rotated key, got %v", err)
	}
	if sub != "user-1" {
		t.Errorf("expected sub user-1, got %q", sub)
	}
	if js.hits.Load() <= hitsBefore {
		t.Error("expected a forced JWKS refetch on unknown kid")
	}
}

// A non-200 JWKS response must not be cached, or every request fails for the
// whole TTL after one blip.
func TestFetchJWKS_ErrorResponseNotCached(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	js := newJWKSServer(t)
	js.publish(&key.PublicKey, "kid-1")
	js.status.Store(http.StatusInternalServerError)

	v := newVerifier(js)
	token := signToken(t, key, "kid-1", testIssuer, testAudience)

	if _, err := v.Verify(context.Background(), token); err == nil {
		t.Fatal("expected failure while JWKS endpoint is down")
	}

	// Endpoint recovers — the next call must succeed, proving nothing bad was cached.
	js.status.Store(http.StatusOK)
	if _, err := v.Verify(context.Background(), token); err != nil {
		t.Fatalf("expected recovery after JWKS endpoint returns, got %v", err)
	}
}

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
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/dfe/api/internal/middleware"
)

const (
	testIssuer   = "https://accounts.example"
	testAudience = "https://dfe-api.example"
)

// newJWKSServer returns an RSA key and an httptest server serving its public JWKS.
// JWKS-fetch/rotation/kid-rejection mechanics are covered by
// gopkg.aoctech.app/api-commons/jwtverify's own tests; this file only checks
// that dfe's Verifier wrapper wires the shared verifier correctly.
func newJWKSServer(t *testing.T) (*rsa.PrivateKey, *httptest.Server) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("genkey: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"keys": []map[string]any{{
		"kty": "RSA",
		"kid": "kid-1",
		"n":   base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return key, srv
}

func signToken(t *testing.T, key *rsa.PrivateKey, claims jwt.MapClaims) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = "kid-1"
	s, err := tok.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return s
}

func TestVerify_DelegatesToSharedVerifier(t *testing.T) {
	key, srv := newJWKSServer(t)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16))

	now := time.Now().Unix()
	token := signToken(t, key, jwt.MapClaims{
		"sub":   "user-1",
		"scope": "dfe:nfes:read dfe:nfes:write",
		"iss":   testIssuer,
		"aud":   []string{testAudience},
		"iat":   now,
		"exp":   now + 900,
	})

	sub, scopes, err := v.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if sub != "user-1" {
		t.Errorf("expected sub user-1, got %q", sub)
	}
	if len(scopes) != 2 || scopes[0] != "dfe:nfes:read" {
		t.Errorf("expected parsed scopes, got %v", scopes)
	}
}

func TestVerify_RejectsInvalidToken(t *testing.T) {
	_, srv := newJWKSServer(t)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16))

	if _, _, err := v.Verify(context.Background(), "not-a-jwt"); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}
}

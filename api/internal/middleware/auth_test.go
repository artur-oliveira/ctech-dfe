package middleware_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/jwtverify"
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
		"sub":       "user-1",
		"scope":     "dfe:nfes:read dfe:nfes:write",
		"token_use": "access",
		"iss":       testIssuer,
		"aud":       []string{testAudience},
		"iat":       now,
		"exp":       now + 900,
	})

	claims, err := v.VerifyClaims(context.Background(), token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Sub != "user-1" {
		t.Errorf("expected sub user-1, got %q", claims.Sub)
	}
	if len(claims.Scopes()) != 2 || claims.Scopes()[0] != "dfe:nfes:read" {
		t.Errorf("expected parsed scopes, got %v", claims.Scopes())
	}
}

func TestVerify_RejectsInvalidToken(t *testing.T) {
	_, srv := newJWKSServer(t)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16))

	if _, err := v.VerifyClaims(context.Background(), "not-a-jwt"); err == nil {
		t.Fatal("expected malformed token to be rejected")
	}
}

func TestMiddleware_RefusesWritesOfLockedUser(t *testing.T) {
	key, srv := newJWKSServer(t)
	locked := map[string]bool{"user-locked": true}
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16)).
		WithLockCheck(func(_ context.Context, sub string) (bool, error) { return locked[sub], nil })
	app := fiber.New()
	app.Use(v.Middleware())
	app.All("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })

	do := func(method, sub string) int {
		now := time.Now().Unix()
		tok := signToken(t, key, jwt.MapClaims{
			"sub": sub, "iss": testIssuer, "aud": []string{testAudience}, "iat": now, "exp": now + 900, "token_use": "access",
		})
		req := httptest.NewRequest(method, "/x", nil)
		req.Header.Set("Authorization", "Bearer "+tok)
		resp, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		return resp.StatusCode
	}
	if got := do(fiber.MethodPost, "user-locked"); got != fiber.StatusForbidden {
		t.Errorf("POST by a locked user = %d, want 403", got)
	}
	if got := do(fiber.MethodGet, "user-locked"); got != fiber.StatusNoContent {
		t.Errorf("GET by a locked user = %d, want 204 (reads are not state-changing)", got)
	}
	if got := do(fiber.MethodDelete, "user-free"); got != fiber.StatusNoContent {
		t.Errorf("DELETE by an active user = %d, want 204", got)
	}
}

func TestMiddleware_LockCheckFailureFailsClosed(t *testing.T) {
	key, srv := newJWKSServer(t)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, cache.NewMemoryBackend(16)).
		WithLockCheck(func(context.Context, string) (bool, error) { return false, errors.New("dynamodb down") })
	app := fiber.New()
	app.Use(v.Middleware())
	app.Post("/x", func(c fiber.Ctx) error { return c.SendStatus(fiber.StatusNoContent) })
	now := time.Now().Unix()
	tok := signToken(t, key, jwt.MapClaims{"sub": "u", "iss": testIssuer, "aud": []string{testAudience}, "iat": now, "exp": now + 900, "token_use": "access"})
	req := httptest.NewRequest(fiber.MethodPost, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != fiber.StatusInternalServerError {
		t.Fatalf("status = %d, want 500 (an unknown lock state must not allow a write)", resp.StatusCode)
	}
}

func TestVerify_RejectsRevokedToken(t *testing.T) {
	key, srv := newJWKSServer(t)
	mem := cache.NewMemoryBackend(16)
	v := middleware.NewVerifier(srv.URL, testAudience, testIssuer, mem)
	now := time.Now()
	tok := signToken(t, key, jwt.MapClaims{
		"sub": "user-1", "iss": testIssuer, "aud": []string{testAudience},
		"iat": now.Add(-time.Minute).Unix(), "exp": now.Add(14 * time.Minute).Unix(), "token_use": "access",
	})
	if err := jwtverify.Revoke(context.Background(), mem, "user-1", now, jwtverify.RevocationTTL); err != nil {
		t.Fatal(err)
	}
	if _, err := v.VerifyClaims(context.Background(), tok); !errors.Is(err, jwtverify.ErrTokenRevoked) {
		t.Fatalf("err = %v, want ErrTokenRevoked", err)
	}
}

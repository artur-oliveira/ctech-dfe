// Package middleware provides Fiber middleware for JWT RS256 auth, tenant resolution,
// and request recovery — mirroring api/app/dependencies/auth.py and security.py.
package middleware

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/artur-oliveira/ctech-dfe/api/internal/cache"
	"github.com/artur-oliveira/ctech-dfe/api/internal/problem"

	"github.com/gofiber/fiber/v3"
	"github.com/golang-jwt/jwt/v5"
)

const (
	jwksCacheKey = "ctech:jwks"
	jwksTTL      = 3600 // 1 hour — matches Python _JWKS_TTL
	UserIDKey    = "user_id"

	// minJWKSRefetchInterval throttles forced JWKS refreshes triggered by an unknown
	// kid, so a flood of bogus tokens cannot hammer the identity provider.
	minJWKSRefetchInterval = 60 * time.Second
)

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

// Verifier validates RS256 access tokens issued by ctech-account against its JWKS.
// It bundles the settings every call site needs, so they are configured once.
type Verifier struct {
	jwksURL  string
	audience string // expected aud claim; empty disables the audience check
	issuer   string // expected iss claim; empty disables the issuer check
	cache    cache.Backend

	mu          sync.Mutex
	lastRefetch time.Time
}

func NewVerifier(jwksURL, audience, issuer string, cacheBackend cache.Backend) *Verifier {
	return &Verifier{jwksURL: jwksURL, audience: audience, issuer: issuer, cache: cacheBackend}
}

// Middleware returns Fiber middleware that validates the Bearer token on each request.
func (v *Verifier) Middleware() fiber.Handler {
	return func(c fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(problem.Unauthorized("missing bearer token"))
		}
		tokenStr := strings.TrimPrefix(authHeader, "Bearer ")

		sub, err := v.Verify(c.Context(), tokenStr)
		if err != nil || sub == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(problem.Unauthorized("invalid credentials"))
		}

		c.Locals(UserIDKey, sub)
		return c.Next()
	}
}

// GetUserID returns the authenticated user's subject from Fiber locals.
func GetUserID(c fiber.Ctx) string {
	v, _ := c.Locals(UserIDKey).(string)
	return v
}

// Verify validates a raw JWT string and returns the subject claim. Used directly by
// the WebSocket handler, where auth arrives as a query param rather than a header.
func (v *Verifier) Verify(ctx context.Context, tokenStr string) (string, error) {
	kid, err := tokenKID(tokenStr)
	if err != nil {
		return "", err
	}

	pubKey, err := v.keyForKID(ctx, kid)
	if err != nil {
		return "", err
	}

	parseOpts := []jwt.ParserOption{}
	if v.audience != "" {
		parseOpts = append(parseOpts, jwt.WithAudience(v.audience))
	}
	if v.issuer != "" {
		parseOpts = append(parseOpts, jwt.WithIssuer(v.issuer))
	}
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return pubKey, nil
	}, parseOpts...)
	if err != nil {
		return "", err
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return "", fmt.Errorf("invalid claims")
	}

	sub, _ := claims["sub"].(string)
	return sub, nil
}

// keyForKID resolves the signing key for kid. On a cache miss it forces one JWKS
// refresh (throttled) so a key rotation at the identity provider takes effect
// immediately instead of after the cache TTL. An unresolvable kid is rejected —
// never silently verified against some other key.
func (v *Verifier) keyForKID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	keys, err := v.fetchJWKS(ctx, false)
	if err != nil {
		return nil, fmt.Errorf("jwks unavailable: %w", err)
	}
	if k := findKID(keys, kid); k != nil {
		return jwkToRSA(k)
	}

	// Unknown kid: the provider may have rotated keys since we cached them.
	keys, err = v.fetchJWKS(ctx, true)
	if err != nil {
		return nil, fmt.Errorf("jwks refresh failed: %w", err)
	}
	if k := findKID(keys, kid); k != nil {
		return jwkToRSA(k)
	}
	return nil, fmt.Errorf("no signing key for kid %q", kid)
}

func findKID(keys []jwk, kid string) *jwk {
	for i := range keys {
		if keys[i].Kid == kid {
			return &keys[i]
		}
	}
	return nil
}

// tokenKID extracts the kid from a JWT header without verifying the signature.
func tokenKID(tokenStr string) (string, error) {
	parts := strings.Split(tokenStr, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("malformed token")
	}
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", err
	}
	var header struct {
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return "", err
	}
	return header.Kid, nil
}

// fetchJWKS returns the provider's signing keys. When force is true the cache is
// bypassed and refreshed, subject to minJWKSRefetchInterval.
func (v *Verifier) fetchJWKS(ctx context.Context, force bool) ([]jwk, error) {
	if !force {
		if data, ok, _ := v.cache.Get(ctx, jwksCacheKey); ok {
			var keys []jwk
			if err := json.Unmarshal(data, &keys); err == nil && len(keys) > 0 {
				return keys, nil
			}
		}
	} else {
		v.mu.Lock()
		if time.Since(v.lastRefetch) < minJWKSRefetchInterval {
			v.mu.Unlock()
			return nil, fmt.Errorf("jwks refresh throttled")
		}
		v.lastRefetch = time.Now()
		v.mu.Unlock()
	}

	keys, err := fetchJWKSFromURL(ctx, v.jwksURL)
	if err != nil {
		return nil, err
	}

	// Only ever cache a usable key set — caching an empty or failed response would
	// break every request for the whole TTL.
	if len(keys) > 0 {
		if data, err := json.Marshal(keys); err == nil {
			_ = v.cache.Set(ctx, jwksCacheKey, data, jwksTTL)
		}
	}
	return keys, nil
}

func fetchJWKSFromURL(ctx context.Context, jwksURL string) ([]jwk, error) {
	httpCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(httpCtx, http.MethodGet, jwksURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// A 4xx/5xx body is not a key set; decoding it would yield zero keys and,
	// before this check, poison the cache for an hour.
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jwks endpoint returned status %d", resp.StatusCode)
	}

	var jwks jwksResponse
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}
	if len(jwks.Keys) == 0 {
		return nil, fmt.Errorf("jwks endpoint returned no keys")
	}
	return jwks.Keys, nil
}

func jwkToRSA(k *jwk) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
	if err != nil {
		return nil, fmt.Errorf("jwk: decode N: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
	if err != nil {
		return nil, fmt.Errorf("jwk: decode E: %w", err)
	}
	n := new(big.Int).SetBytes(nBytes)
	e := int(new(big.Int).SetBytes(eBytes).Int64())
	return &rsa.PublicKey{N: n, E: e}, nil
}

package ciot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ErrProviderUnavailable: the generator failed and no fallback applies (spec section 4); the declaration fails
// closed, since the identifier is never fabricated locally.
var ErrProviderUnavailable = errors.New("ciot: PROVIDER_UNAVAILABLE: identifier generator unavailable")

// SecretProvider resolves the generator secrets (SSM /ctech-dfe/{env}/ciot/..., see ParamName). The library never
// reads AWS; the worker supplies an SSM-backed implementation, the egress receives the values in the request.
type SecretProvider interface {
	Get(ctx context.Context, name string) (string, error)
}

// StaticSecrets is a SecretProvider over a map keyed by ParamAPIKey / ParamFallbackURL.
type StaticSecrets map[string]string

// Get implements SecretProvider.
func (s StaticSecrets) Get(_ context.Context, name string) (string, error) { return s[name], nil }

type token struct {
	value   string
	expires time.Time
}

var (
	tokenMu    sync.Mutex
	tokenCache = map[string]token{}
)

// ResetTokenCache clears the process-wide token cache (tests).
func ResetTokenCache() {
	tokenMu.Lock()
	tokenCache = map[string]token{}
	tokenMu.Unlock()
}

// Generator gets 12-character identifiers from the ANTT generator (principal over mTLS, fallback without).
type Generator struct {
	env         string
	apiKey      string
	primary     *Client
	fallbackURL string
	fallback    *Client
	now         func() time.Time
}

// NewGenerator validates the configuration eagerly: an empty or CHANGE_ME API key is a configuration failure
// here, not on first use. primary is the mTLS client of the webservice host; plainHTTP is the client used for the
// fallback gateway (real TLS verification). Fallback is used only in production and only when configured.
func NewGenerator(ctx context.Context, env string, primary *Client, plainHTTP *http.Client, secrets SecretProvider) (*Generator, error) {
	if env != EnvProd && env != EnvHom {
		return nil, fmt.Errorf("ciot: unknown environment %q", env)
	}
	if primary == nil {
		return nil, errors.New("ciot: generator needs a primary client")
	}
	key, err := secrets.Get(ctx, ParamAPIKey)
	if err != nil {
		return nil, fmt.Errorf("ciot: read generator api key: %w", err)
	}
	key = strings.TrimSpace(key)
	if key == "" || key == PlaceholderAPIKey {
		return nil, errors.New("ciot: generator API key is empty or the CHANGE_ME placeholder (configuration failure)")
	}
	g := &Generator{env: env, apiKey: key, primary: primary, now: time.Now}
	if env == EnvProd {
		u, err := secrets.Get(ctx, ParamFallbackURL)
		if err != nil {
			return nil, fmt.Errorf("ciot: read generator fallback url: %w", err)
		}
		if u = strings.TrimRight(strings.TrimSpace(u), "/"); u != "" {
			if plainHTTP == nil {
				plainHTTP = &http.Client{}
			}
			g.fallbackURL, g.fallback = u, &Client{Host: u, HTTP: plainHTTP, Sleep: primary.Sleep}
		}
	}
	return g, nil
}

// Generate returns a new 12-character identifier for cpfCnpj (the contratado).
func (g *Generator) Generate(ctx context.Context, cpfCnpj string) (string, error) {
	id, err := g.generateVia(ctx, g.primary, "primary", cpfCnpj)
	if err == nil {
		return id, nil
	}
	var se *ErrServer
	if !errors.As(err, &se) || se.Status != http.StatusInternalServerError {
		return "", err
	}
	if g.env != EnvProd || g.fallback == nil {
		return "", fmt.Errorf("%w: %v", ErrProviderUnavailable, err)
	}
	id, ferr := g.generateVia(ctx, g.fallback, "fallback", cpfCnpj)
	if ferr != nil {
		return "", fmt.Errorf("%w: %v", ErrProviderUnavailable, ferr)
	}
	return id, nil
}

func (g *Generator) generateVia(ctx context.Context, c *Client, kind, cpfCnpj string) (string, error) {
	tok, err := g.token(ctx, c, kind)
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(map[string]string{"cpfCnpj": NormalizeDoc(cpfCnpj)})
	status, body, err := c.do(ctx, http.MethodPost, c.Host+pathGerar, payload, GeneratorTimeout,
		map[string]string{"Authorization": "Bearer " + tok})
	if err != nil {
		return "", err
	}
	if status == http.StatusUnauthorized { // token revoked early: drop it, caller retries
		g.dropToken(c, kind)
	}
	if status >= 500 {
		return "", &ErrServer{Status: status}
	}
	var out struct {
		Sucesso bool `json:"sucesso"`
		Dados   struct {
			CIOT string `json:"ciot"`
		} `json:"dados"`
	}
	if err := json.Unmarshal(body, &out); err != nil || status >= 400 || !out.Sucesso || len(out.Dados.CIOT) != IDLen {
		return "", fmt.Errorf("ciot: generator refused (HTTP %d)", status)
	}
	return out.Dados.CIOT, nil
}

func (g *Generator) cacheKey(c *Client, kind string) string { return g.env + "|" + kind + "|" + c.Host }

func (g *Generator) dropToken(c *Client, kind string) {
	tokenMu.Lock()
	delete(tokenCache, g.cacheKey(c, kind))
	tokenMu.Unlock()
}

// token returns a cached JWT, renewing it TokenRenewMargin before its TokenLifetime ends. The cache is per
// environment and per generator; the API key is never part of a log line or an error.
func (g *Generator) token(ctx context.Context, c *Client, kind string) (string, error) {
	key := g.cacheKey(c, kind)
	tokenMu.Lock()
	defer tokenMu.Unlock()
	if t, ok := tokenCache[key]; ok && g.now().Before(t.expires.Add(-TokenRenewMargin)) {
		return t.value, nil
	}
	status, body, err := c.do(ctx, http.MethodPost, c.Host+pathToken, []byte("{}"), GeneratorTimeout,
		map[string]string{headerChave: g.apiKey})
	if err != nil {
		return "", err
	}
	if status >= 500 {
		return "", &ErrServer{Status: status}
	}
	var out struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(body, &out); err != nil || status >= 400 || out.Token == "" {
		return "", fmt.Errorf("ciot: generator token refused (HTTP %d)", status)
	}
	tokenCache[key] = token{value: out.Token, expires: g.now().Add(TokenLifetime)}
	return out.Token, nil
}

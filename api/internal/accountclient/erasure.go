package accountclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"gopkg.aoctech.app/api-commons/oauth2client"
)

const (
	// ScopeErasureSubject reads the CPF of a user being purged, to find their
	// e-CPF certificates (saga protocol §4.4). Granted to dfe only.
	ScopeErasureSubject = "internal:account:erasure-subject"
	// ScopeErasureAck acks a finished user.erase (saga protocol §3).
	ScopeErasureAck = "internal:account:erasure-ack"
	// ScopeExportAck acks an uploaded part of the personal-data export
	// (ctech-account docs/specs/2026-10-17-data-export.md); an operator grants it to the same client.
	ScopeExportAck = "internal:account:export-ack"
)

// SubjectClient asks ctech-account for the CPF of a purging request. Its own
// token manager, so the token carries this one scope (plan ruling R11).
type SubjectClient struct {
	http    *http.Client
	tokens  *oauth2client.TokenManager
	baseURL string
}

// NewSubject builds the client, or nil when the credential is incomplete.
func NewSubject(cfg Config) *SubjectClient {
	if cfg.BaseURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	httpClient := &http.Client{Timeout: requestTimeout}
	return &SubjectClient{
		http:    httpClient,
		tokens:  oauth2client.New(httpClient, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, ScopeErasureSubject),
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

// CPF returns the CPF of the request's user ("" when the account has none).
// The value is personal data: callers keep it in memory and never log it.
func (c *SubjectClient) CPF(ctx context.Context, requestID string) (string, error) {
	if c == nil {
		return "", fmt.Errorf("ctech-account erasure subject client is not configured")
	}
	token, err := c.tokens.Get(ctx)
	if err != nil {
		return "", fmt.Errorf("minting a service token: %w", err)
	}
	return c.cpfWithToken(ctx, token, requestID)
}

func (c *SubjectClient) cpfWithToken(ctx context.Context, token, requestID string) (string, error) {
	u := fmt.Sprintf("%s/v1.0/internal/erasure/%s/subject", c.baseURL, url.PathEscape(requestID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", fmt.Errorf("building the subject request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("calling ctech-account: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		// Never the body: it may echo personal data into a log line.
		return "", fmt.Errorf("ctech-account answered %d for erasure subject %s", resp.StatusCode, requestID)
	}
	var out struct {
		CPF string `json:"cpf"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding the erasure subject")
	}
	return out.CPF, nil
}

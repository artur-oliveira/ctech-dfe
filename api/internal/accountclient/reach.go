// Package accountclient calls ctech-account for the facts it owns.
//
// Today that is one fact: whether a person may act for a company
// (ctech-billing ADR 0023). It is its own package rather than a method on the
// billing client because the two answer different questions with different
// failure rules — billing degrades open on an outage, reach must not — and
// sharing a type is how one rule ends up applied to the other.
package accountclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"gopkg.aoctech.app/api-commons/cache"
	"gopkg.aoctech.app/api-commons/oauth2client"
)

// Scope is what the reach route requires. One scope, and nothing else: this
// client has no business holding a credential that can do more than ask.
// ErrCompanyNotFound is a company ctech-account does not have under the named
// organization. Its own error because the caller acts on it: a handoff naming a
// company that is not there is a link to refuse, not an outage to retry.
var ErrCompanyNotFound = errors.New("company not found in ctech-account")

const Scope = "internal:account:company-actor"

const (
	requestTimeout = 6 * time.Second
	// maxBody caps what is read from a service we do not control in-process.
	maxBody = 8 << 10
)

// Config is what the client needs to reach ctech-account.
type Config struct {
	BaseURL      string
	TokenURL     string
	ClientID     string
	ClientSecret string
	Cache        cache.Backend
}

// Client asks ctech-account about reach.
type Client struct {
	http    *http.Client
	tokens  *oauth2client.TokenManager
	baseURL string
}

// New builds a client, or nil when ctech-account's service credential is not
// configured.
//
// Nil is deliberately NOT usable here, unlike the billing client's nil. Billing
// answers ErrNotConfigured and the caller reads that as "unlimited", which is
// right for a quota and catastrophic for an authorization check: an
// unconfigured reach client that answered "allowed" would be a deployment
// mistake turning into an open door. Callers must treat a nil client as a
// refusal, and Reach on nil says so.
func New(cfg Config) *Client {
	if cfg.BaseURL == "" || cfg.TokenURL == "" || cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil
	}
	httpClient := &http.Client{Timeout: requestTimeout}
	return &Client{
		http:    httpClient,
		tokens:  oauth2client.New(httpClient, cfg.Cache, cfg.TokenURL, cfg.ClientID, cfg.ClientSecret, Scope),
		baseURL: strings.TrimSuffix(cfg.BaseURL, "/"),
	}
}

// Enabled reports a client that can actually ask.
func (c *Client) Enabled() bool { return c != nil }

type reachResponse struct {
	MayAct         bool   `json:"may_act"`
	OrganizationID string `json:"organization_id"`
}

// Reach answers whether userID may act for companyID, and which organization
// the company belongs to.
//
// A refusal is ("", false, nil) — "not permitted" is an answer. Anything that
// stops us from knowing is an error, and the caller refuses: this is an
// authorization check, and an unconfigured or unreachable dependency must never
// read as permission.
func (c *Client) Reach(ctx context.Context, companyID, userID string) (string, bool, error) {
	if c == nil {
		return "", false, fmt.Errorf("ctech-account reach client is not configured")
	}
	token, err := c.tokens.Get(ctx)
	if err != nil {
		return "", false, fmt.Errorf("minting a service token: %w", err)
	}
	return c.reachWithToken(ctx, token, companyID, userID)
}

// reachWithToken is the request itself, split from the token so the HTTP
// behaviour — which status means what — is testable without a token endpoint.
func (c *Client) reachWithToken(ctx context.Context, token, companyID, userID string) (string, bool, error) {
	path := fmt.Sprintf("%s/v1.0/internal/companies/%s/actors/%s",
		c.baseURL, url.PathEscape(companyID), url.PathEscape(userID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return "", false, fmt.Errorf("building the reach request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", false, fmt.Errorf("calling ctech-account: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		// Every non-200 is an error, including 403. A 403 here means this
		// client's own credential is wrong, which is an operational fault
		// rather than a statement about the user — reading it as a refusal
		// would hide a broken deployment behind thousands of denied requests.
		return "", false, fmt.Errorf("ctech-account answered %d", resp.StatusCode)
	}
	var out reachResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
		return "", false, fmt.Errorf("decoding the reach answer: %w", err)
	}
	return out.OrganizationID, out.MayAct, nil
}

// Identity is who a company is, as ctech-account records it.
type Identity struct {
	OrganizationID string `json:"organization_id"`
	TaxID          string `json:"tax_id"`
	TaxIDKind      string `json:"tax_id_kind"`
	LegalName      string `json:"legal_name"`
	TradeName      string `json:"trade_name"`
}

// Company reads a company's identity.
//
// Asked once, when this product first links a company handed to it by the
// handoff — not on the issuance path, which reads the local record. Both ids
// because the handoff hands both, and a mismatched pair must not resolve to
// somebody else's company.
func (c *Client) Company(ctx context.Context, organizationID, companyID string) (*Identity, error) {
	if c == nil {
		return nil, fmt.Errorf("ctech-account client is not configured")
	}
	token, err := c.tokens.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("minting a service token: %w", err)
	}

	path := fmt.Sprintf("%s/v1.0/internal/organizations/%s/companies/%s",
		c.baseURL, url.PathEscape(organizationID), url.PathEscape(companyID))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, fmt.Errorf("building the identity request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling ctech-account: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrCompanyNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ctech-account answered %d", resp.StatusCode)
	}
	var out Identity
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&out); err != nil {
		return nil, fmt.Errorf("decoding the identity: %w", err)
	}
	if out.TaxID == "" {
		// A company with no tax id cannot be issued for, and linking it would
		// produce a local record that fails at the first emission with a much
		// worse message than this one.
		return nil, fmt.Errorf("ctech-account returned a company with no tax id")
	}
	return &out, nil
}
